package zdb

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// migrationsFS —— 迁移件内嵌（设计稿 §三.7：//go:embed migrations/*.sql，≈30 行自管、✗ 不引迁移框架）。
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// loadMigrations —— 解析 migrations/NNNN_*.sql，返「版本号 → SQL」与最大版本号。
// 版本号真源 = 文件名前缀（与 PRAGMA user_version 对齐）。
func loadMigrations() (map[int]string, int, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, 0, fmt.Errorf("zdb: 读内嵌迁移目录失败: %w", err)
	}
	out := map[int]string{}
	max := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".sql")
		i := strings.IndexByte(base, '_')
		if i <= 0 {
			return nil, 0, fmt.Errorf("zdb: 迁移件命名须为 NNNN_<名>.sql（实得 %s）", e.Name())
		}
		v, err := strconv.Atoi(base[:i])
		if err != nil || v <= 0 {
			return nil, 0, fmt.Errorf("zdb: 迁移件版本前缀非法（%s）", e.Name())
		}
		if _, dup := out[v]; dup {
			return nil, 0, fmt.Errorf("zdb: 迁移版本号重复（%d）", v)
		}
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, 0, fmt.Errorf("zdb: 读迁移件 %s 失败: %w", e.Name(), err)
		}
		out[v] = string(b)
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return nil, 0, fmt.Errorf("zdb: 内嵌迁移集为空（migrations/*.sql 一件都没有）")
	}
	// 版本号必须从 1 连续到 max（✗ 不许有洞——否则 user_version 语义会含糊）。
	for v := 1; v <= max; v++ {
		if _, ok := out[v]; !ok {
			return nil, 0, fmt.Errorf("zdb: 迁移版本号有洞（缺 %d）", v)
		}
	}
	return out, max, nil
}

// UserVersion —— 读 PRAGMA user_version（库自带的应用私有迁移号）。
func (d *DB) UserVersion() (int, error) {
	var v int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("zdb: 读 user_version 失败: %w", err)
	}
	return v, nil
}

// mustMigrations —— 已解析的内嵌迁移集（版本号 → SQL）。失败即返回（migrate 的唯一入口）。
func mustMigrations() (map[int]string, error) {
	migs, _, err := loadMigrations()
	return migs, err
}

// migrate —— 自管迁移（设计稿 §三.7）：
//   - 每步 SQL 与 `PRAGMA user_version = N` 在**同一事务**里（崩则整体回滚、下次干净重跑）。
//   - 已发布迁移永不修改、只追加。
//   - 库版本高于本二进制支持上限 ⇒ 拒启（✗ 静默用新 schema）。
//   - 版本号已是最新 ⇒ **幂等空跑**（不重跑任何 DDL）。
//
// ★ 本函数按**迁移件类别**分两条路（判定见 isDestructiveMigration · §四.1 附 J.3 的破坏性判据）：
//   - **纯新增类**（如 0001：只 CREATE / 建索引）⇒ 走写池单事务（**与改前逐字同** —— ✗ 别把简单路也变复杂）；
//   - **破坏性 / 重建表类**（如 0002：DROP TABLE ＋ ALTER…RENAME）⇒ 走 applyDestructiveMigration：
//     前置 `VACUUM INTO` 快照**硬闸** ＋ 另开一条**不带 `foreign_keys(ON)`** 的专用连接、
//     **事务外**先关 FK（官方十二步之步 1/12）＋ 事务内**提交前**收尾 `PRAGMA foreign_key_check`
//     （§三.7 附 I.3/I.4 · §四.1 附 J.3 · §六 Q15 / Q20 / Q21）。
func (d *DB) migrate() error {
	migs, err := mustMigrations()
	if err != nil {
		return err
	}
	max := 0
	for v := range migs {
		if v > max {
			max = v
		}
	}
	cur, err := d.UserVersion()
	if err != nil {
		return err
	}
	if cur > max {
		return fmt.Errorf("%w：库 user_version=%d，本二进制支持上限=%d ⇒ 拒启（✗ 静默用新 schema）", ErrSchemaTooNew, cur, max)
	}
	if cur == max {
		return nil // 幂等重跑：无迁移可做
	}
	versions := make([]int, 0, len(migs))
	for v := range migs {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for _, v := range versions {
		if v <= cur {
			continue
		}
		if isDestructiveMigration(migs[v]) {
			if err := d.applyDestructiveMigration(v, migs[v]); err != nil {
				return err
			}
			continue
		}
		// 纯新增类：**原路**（写池单事务 · ✗ 不产快照、✗ 不另开连接）。
		if err := applyMigration(d.db, v, migs[v], false); err != nil {
			return err
		}
	}
	return nil
}

// ───────────── 迁移件类别判定（决定走「原路」还是「重路」的**唯一依据**） ─────────────

// destructiveMigrationRE / renameMigrationRE —— 「破坏性 / 重建表类」迁移的判定依据。
// 出处逐字：§四.1 附 J.3「**破坏性迁移判据**：迁移件含「重建表 / `DROP TABLE` / 改 CHECK / 改列名 / 删列」者」。
//
//	① `DROP TABLE`              —— 官方十二步之步 6；`DROP TABLE` 在 FK **开启**时会发一条**隐式 DELETE**，
//	                               正是把「延迟外键违规」种进事务的那一步（§三.7 附 I.3 阻断级实测）；
//	② `ALTER TABLE … RENAME TO` —— 官方十二步之步 7（重建表的收尾改名；官方论坛逐字：`RENAME`
//	                               **不消解**已记下的违规 —— 所以「改名后行数一致」救不了它）。
//
// ⇒ 命中任一条即判「重建表类」⇒ 走 applyDestructiveMigration（快照硬闸 ＋ 事务外关 FK ＋ 收尾外键检查）；
// 都不命中（纯 `CREATE` / `INSERT` / 建索引一类）⇒ 原路。
// ★ 判定按**原文扫描（含注释）** ⇒ 宁可误判成破坏性（多一次快照 ＋ 多一道外键检查），✗ 绝不可漏判。
var (
	dropTableRE   = regexp.MustCompile(`(?i)\bDROP\s+TABLE\b`)
	renameTableRE = regexp.MustCompile(`(?i)\bALTER\s+TABLE\b[^;]*\bRENAME\s+TO\b`)
)

// isDestructiveMigration —— 本迁移是否属「破坏性 / 重建表类」（判据见上）。
func isDestructiveMigration(sqlText string) bool {
	return dropTableRE.MatchString(sqlText) || renameTableRE.MatchString(sqlText)
}

// fkPragmaEntry —— 写侧 DSN 里那一段「每连接开 FK」（dsn.go 的 BuildDSN 逐字产物）。
const fkPragmaEntry = "&_pragma=foreign_keys(ON)"

// buildMigrationDSN —— **专用迁移连接**的 DSN：与 `BuildDSN` 逐字同形，**唯一差别 = 不带** `fkPragmaEntry`
// （§六 Q15 取甲 / Q20：该连接要能在**事务外**把 FK 关掉）。
//
// ★ 实现 = 从 `BuildDSN` 的产物里**摘掉那一段**（DSN 的单一真源留在 dsn.go，✗ 不另抄第二份配方）；
// 摘不到（dsn.go 改了写法）⇒ **当场报错** —— ✗ 不静默退回一条「FK 恒 ON」的 DSN（那会让整条重路退化成旧路）。
func buildMigrationDSN(dbPath string) (string, error) {
	base := BuildDSN(dbPath)
	out := strings.Replace(base, fkPragmaEntry, "", 1)
	if out == base {
		return "", fmt.Errorf("zdb: 迁移连接 DSN 生成失败 —— BuildDSN 产物里找不到 %q（dsn.go 的配方变了吗？）⇒ fail-closed", fkPragmaEntry)
	}
	return out, nil
}

// applyMigration —— 单步迁移：DDL 与 user_version 同事务（`exec` 可以是写池，也可以是**专用迁移连接**）。
//
// ★ `checkFK=true` 只用于**破坏性迁移**：在 COMMIT **之前**跑 `PRAGMA foreign_key_check`
// （官方十二步之步 10，位于步 11 COMMIT **之前**）⇒ 非 0 行即**整事务 ROLLBACK**、`user_version` 不动。
// ✗ 不是「先提交再报错」—— 那样会把一个**已提交的坏库**留在盘上。
func applyMigration(exec *sql.DB, version int, sqlText string, checkFK bool) error {
	tx, err := exec.Begin()
	if err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 事务开始失败: %w", version, err)
	}
	if _, err := tx.Exec(sqlText); err != nil {
		tx.Rollback()
		return fmt.Errorf("zdb: 迁移 v%d 建表失败: %w", version, err)
	}
	// PRAGMA user_version 不可参数化（版本号来自内嵌件名，非外部输入）。
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		tx.Rollback()
		return fmt.Errorf("zdb: 迁移 v%d 写 user_version 失败: %w", version, err)
	}
	if checkFK {
		if err := foreignKeyCheckOn(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("zdb: 迁移 v%d 收尾 foreign_key_check 未过 ⇒ 整事务回滚（fail-closed · §六 Q20②）: %w", version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 提交失败: %w", version, err)
	}
	return nil
}

// queryer —— `*sql.DB` 与 `*sql.Tx` 共有的最小查询面（外键检查两种宿主都要能跑）。
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// foreignKeyCheckOn —— `PRAGMA foreign_key_check` 须回 **0 行**（官方十二步之步 10 · §六 Q20② / Q27）。
//
// ★ 它**不要求** `foreign_keys=ON`（官方行为：直接扫库）⇒ 迁移里 FK 已关也照跑；
// ✗ 别据「那连接 FK 关了」就跳过这一条 —— 这正是本闸要抓的那一类：
// FK 关着重建 ⇒ 外键违例**不被拦、静默留存**（"安全但坏"的反面）。
// ★ 与 `open.go` 的 `DB.foreignKeyCheck()` **同口径**（它跑写池、本件跑事务句柄）——
// open.go 属 T1/T2 在制件（本单 ✗ 不动它）⇒ 两处须同步改（同一条官方拄 ＋ 同一条期望 0 行）。
func foreignKeyCheckOn(q queryer) error {
	rows, err := q.Query("PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("跑 foreign_key_check 失败: %w", err)
	}
	defer rows.Close()
	n := 0
	first := ""
	for rows.Next() {
		// 结果四列 = (表名, rowid, 被引父表, fkid)；rowid / fkid 可能为 NULL ⇒ 用 Null* 接。
		var tbl, parent string
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&tbl, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("读 foreign_key_check 结果失败: %w", err)
		}
		if n == 0 {
			first = fmt.Sprintf("表 %s rowid=%v 引用父表 %s (fkid=%v)", tbl, rowid, parent, fkid)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("foreign_key_check 结果读取中断: %w", err)
	}
	if n != 0 {
		return fmt.Errorf("foreign_key_check：%d 行外键违例（首行：%s）—— 期望 0 行（官方十二步之步 10 · §六 Q27）", n, first)
	}
	return nil
}

// applyDestructiveMigration —— **破坏性 / 重建表类**迁移的重路（§六 Q15 甲 · Q20 · Q21① · §三.7 附 I.3/I.4）：
//
//	① **前置快照硬闸** —— 先落一份 `VACUUM INTO` 一致快照（T2 备份体例）：做不了（卷不在 / 空间不足 /
//	   目标已存在 / 快照体检不过）⇒ **拒启**，迁移**一步都不跑**（见 migrationSnapshot）；
//	② **另开一条专用临时连接**（✗ 不用写池的连接）：其 DSN **不带** `_pragma=foreign_keys(ON)`；
//	③ **事务外先关 FK** —— 官方十二步之步 1/12：`PRAGMA foreign_keys = OFF` 必须在 **BEGIN 之前**
//	   （该 PRAGMA 在事务内是 **no-op** · `pragma.html#pragma_foreign_keys` 逐字
//	   「This pragma is a no-op within a transaction」）＋ **回读自证**应为 0（✗ 关不成就不许进事务）；
//	④ 单事务跑该步迁移（`applyMigration`，含**提交前**的收尾 `PRAGMA foreign_key_check`）；
//	⑤ **连接即弃**（`defer conn.Close()`）。
//	   ★ 连接即弃 ⇒ 复位 `foreign_keys=ON` **非必需**（Q20 细节① 针对的是「钉写池连接」那种**复用**形态）；
//	     本形态仍**显式复位一次**作双保险 —— 一旦将来有人把它改成复用连接，这一行就是必需项。
func (d *DB) applyDestructiveMigration(version int, sqlText string) error {
	if _, err := d.migrationSnapshot(); err != nil {
		return err
	}
	dsn, err := buildMigrationDSN(d.path)
	if err != nil {
		return err
	}
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 开专用迁移连接失败: %w", version, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	defer conn.Close()

	// 步 1/12：**事务外**先关 FK ＋ 回读自证（✗ 关不成就不许进事务 —— 进了必在 COMMIT 炸）。
	if _, err := conn.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 事务外关 FK 失败: %w", version, err)
	}
	var fk int
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 回读 foreign_keys 失败: %w", version, err)
	}
	if fk != 0 {
		return fmt.Errorf("zdb: 迁移 v%d 事务外关 FK 未生效（回读 foreign_keys=%d，期望 0）⇒ fail-closed，✗ 不进事务", version, fk)
	}

	if err := applyMigration(conn, version, sqlText, true); err != nil {
		return err
	}

	// 步 12/12：复位（本形态连接即弃 ⇒ 非必需；留作双保险 · Q20 细节①）。
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 复位 foreign_keys 失败: %w", version, err)
	}
	return nil
}

// migrationSnapshot —— **破坏性迁移的前置硬闸**（§四.1 附 J.3 · J.4 · §六 Q21①）。
//
// 落点 / 命名 / 侧车**照 T2 已落的体例**（✗ 另起一套）：`BackupDir()`（`ZERG_BACKUP_DIR` 可覆盖）
// ＋ `zerg-<UTC YYYYMMDD-HHMMSS>.db` ＋ 同名 `.meta.json` 侧车 ⇒ 它同时是一份**能直接进 `Restore()`**
// 的回滚点（缺侧车 ⇒ Restore 拒 ⇒ 所以侧车必须写）。
//
// 五道检查（§六 Q21①「做不了 ⇒ 拒启」逐条对上）：
//
//	① 取得到备份落点（家目录 / `ZERG_BACKUP_DIR`）；
//	② 备份目录**可建 / 可写**（卷不在 ⇒ 这里当场露）；
//	③ `statfs` 可用 ≥ **2×** 库大小（空间不足 ⇒ 拒 · `VACUUM INTO` 官方硬要求）；
//	④ 目标名**不存在**（`VACUUM INTO` 非幂等 ⇒ 同秒撞名逐秒退避，退避用尽 ⇒ 拒）；
//	⑤ `VACUUM INTO` 真落盘 ＋ 侧车 ＋ 快照 `integrity_check` **回 ok**（J.3 第②条：
//	   快照落盘且 `integrity_check` 回 `ok` **才**允许进迁移事务）。
//
// ★ 本路径 ✗ **不套**跨卷守卫（`ensureBackupTargetDifferentVolume` · 硬规矩③）：那道守卫是**周期备份**
// 的守卫（防「同卷掉盘 ⇒ 库与备份一起没」）；本闸要的是「**迁移前必有一份能退的快照**」——
// 生产上库恒在外卷（`/Volumes/...`）、快照恒在内盘 ⇒ 异卷由部署形态**天然满足**；再套一遍只会让
// 「库也在内盘的部署」在**前置**就 fail-closed 到不可用（反效果）。周期备份（`BackupDefault`）
// **照旧**走那道守卫，本单 ✗ 未放宽它。
func (d *DB) migrationSnapshot() (string, error) {
	dir := BackupDir()
	if dir == "" {
		return "", errors.New("zdb: 破坏性迁移前置快照：取不到备份落点（家目录 / ZERG_BACKUP_DIR 都没有）⇒ 拒启（fail-closed · §六 Q21①）")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：建备份目录 %s 失败（卷不在 / 不可写）⇒ 拒启（§六 Q21①）: %w", dir, err)
	}
	if err := d.backupSpacePrecheck(dir); err != nil {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照不可得 ⇒ 拒启（§六 Q21①）: %w", err)
	}
	base := time.Now().UTC()
	dst := ""
	for i := 0; i < 120; i++ {
		name, _ := snapshotName(base.Add(time.Duration(i) * time.Second))
		cand := filepath.Join(dir, name)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			dst = cand
			break
		}
	}
	if dst == "" {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：%s 内命名撞满、无可用名（目标已存在）⇒ 拒启（§六 Q21①）", dir)
	}
	// 先 checkpoint（把 -wal 里已提交页并入主库 ⇒ 快照才完整且小），再 VACUUM INTO 落紧凑单文件快照。
	if err := d.CheckpointTruncate(); err != nil {
		return "", err
	}
	q := "VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err := d.db.Exec(q); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：VACUUM INTO 失败 ⇒ 拒启（§六 Q21①）: %w", err)
	}
	if err := d.writeSidecar(dst); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：侧车落盘失败 ⇒ 拒启（§六 Q21①）: %w", err)
	}
	h, err := openSnapshotRO(dst)
	if err != nil {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：开快照体检失败 ⇒ 拒启（§六 Q21①）: %w", err)
	}
	defer h.Close()
	var ic string
	if err := h.QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：读 integrity_check 失败 ⇒ 拒启（§六 Q21①）: %w", err)
	}
	if ic != "ok" {
		return "", fmt.Errorf("zdb: 破坏性迁移前置快照：快照 integrity_check 未回 ok（实得 %q）⇒ 拒启（§六 Q21① · J.3 第②条）", ic)
	}
	return dst, nil
}

package zdb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Backup —— 硬规矩 ③：备份落**另一卷**（内盘 ~/.zerg/backup/），VACUUM INTO 落紧凑单文件快照。
// 同卷 ⇒ 拒（同卷掉盘 = 库与备份一起没）。
func (d *DB) Backup(dst string) error { return d.backupTo(dst, false) }

// backupTo —— 真备份动作。skipVolumeGuard=true 只供同包测试验「VACUUM INTO 真能产快照」。
func (d *DB) backupTo(dst string, skipVolumeGuard bool) error {
	if d == nil || d.db == nil {
		return errors.New("zdb: 库未打开")
	}
	if strings.TrimSpace(dst) == "" {
		return errors.New("zdb: 备份目标路径为空")
	}
	if !skipVolumeGuard {
		if err := ensureBackupTargetDifferentVolume(d.path, dst); err != nil {
			return err
		}
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("zdb: 备份目标已存在（%s）—— VACUUM INTO 要求目标不存在", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("zdb: 建备份目录失败: %w", err)
	}
	// 备份前先 checkpoint，把 -wal 里的已提交页并入主库（快照才完整且小）。
	if err := d.CheckpointTruncate(); err != nil {
		return err
	}
	q := "VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err := d.db.Exec(q); err != nil {
		return fmt.Errorf("zdb: VACUUM INTO 快照失败: %w", err)
	}
	return nil
}

// ─────────────────── T2 · 命名 / 侧车 / GFS 保留（§四.2 戊 · §六 Q24–Q26） ───────────────────

// backupStampLayout —— 快照名的时间戳格式（**UTC** · §六 Q26 逐字：zerg-<UTC YYYYMMDD-HHMMSS>.db）。
const backupStampLayout = "20060102-150405"

// GFS 保留档位（§四.2 戊 · §六 Q24 逐字）。
const (
	GFSDaily   = 14 // 日备留 14 代（2 周）
	GFSWeekly  = 8  // 周备留 8 代（2 月）
	GFSMonthly = 12 // 月备留 12 代（1 年）
	GFSFloor   = 3  // 最小保底：任何时刻 ≥3 份可开快照（含最近 1 份）
)

// snapshotNameRE —— 快照件名匹配式；prune **只**删匹配此式的件（§六 Q24：防误删）。
var snapshotNameRE = regexp.MustCompile(`^zerg-[0-9]{8}-[0-9]{6}\.db$`)

// backupRowTables —— 侧车 rows{} 的逐表行数清单（§六 Q26 逐字：tasks / task_events / idempotency_keys）。
var backupRowTables = []string{"tasks", "task_events", "idempotency_keys"}

// BackupMeta —— 侧车内容（§六 Q26 八格）。
// ★ 键名与 Q26 逐字同；`rows` = 逐表行数对象。
type BackupMeta struct {
	SHA256         string         `json:"sha256"`
	SizeBytes      int64          `json:"size_bytes"`
	UserVersion    int            `json:"user_version"`
	ApplicationID  int64          `json:"application_id"`
	PageCount      int            `json:"page_count"`
	IntegrityCheck string         `json:"integrity_check"`
	Rows           map[string]int `json:"rows"`
	CreatedAtUTC   string         `json:"created_at_utc"`
}

// snapshotName —— 由 UTC 时刻算（快照名, 侧车名）。
func snapshotName(t time.Time) (dbName, metaName string) {
	stamp := t.UTC().Format(backupStampLayout)
	dbName = "zerg-" + stamp + ".db"
	metaName = dbName + ".meta.json"
	return
}

// BackupDefault —— 生产入口（§六 Q24/Q26 五步）：
//
//	① `statfs` 预检 ≥2× 库大小（不足 ⇒ fail-closed、**不产半件**）；
//	② UTC 命名 `zerg-<YYYYMMDD-HHMMSS>.db`（同秒撞名 ⇒ 退避 +1s）；
//	③ `VACUUM INTO` 落快照（硬规矩 ③ 同卷守卫在 backupTo 里）；
//	④ 写侧车 `zerg-<UTC>.db.meta.json`（八格）；
//	⑤ GFS 逐代 prune（日14/周8/月12 · 保底 ≥3 · **只**删匹配命名件、✗ 不删保底那份）。
func (d *DB) BackupDefault() (string, error) { return d.backupDefault(BackupDir(), false) }

// BackupDefaultReadOnly —— **不跑迁移**的备份生产入口（命令面 `zerg core db backup` 用它）。
//
// ★ 为什么命令面必须走这条：`zdb.Open` 是**写路径**（开库即体检 ＋ **迁移** ＋ 打身份标 ＋ checkpoint）
// ⇒ 拿它做一次「备份」会对源库**顺带跑一遍迁移**并改它的库头 —— 与「备份是只读动作」的口径冲突。
// 本入口的开库路径**一条写步骤都不走**：
//
//	✗ 不跑迁移（`migrate()` 一步都不调）· ✗ 不体检写面 · ✗ 不写身份标 · ✗ 不 checkpoint
//	（`wal_checkpoint(TRUNCATE)` 是**写**动作）· 只执行 `VACUUM INTO`（官方语义：**不改源库**）。
//
// ★★ 「只读」的落点是**行为面**（两条可机检读数：备份前后源库 `sha256` 逐字同 ＋ `user_version` 不动），
// 不是 DSN 的字面 —— 因为 SQLite 的两条真·只读档**都跑不了 `VACUUM INTO`**（本件实测，逐条留证）：
//
//	· `query_only(ON)`（`OpenReader` 那档）⇒ `attempt to write a readonly database (8)`
//	  —— 官方实现里 VACUUM 先起一个**写事务**来固定快照点；
//	· `mode=ro` ⇒ 对 WAL 库连普通读都开不起来（`unable to open database file (14)`）。
//
// ⇒ 源库句柄取 `openSnapshotSource`（DSN 见 `BuildSnapshotSourceDSN`）：**只**带 `busy_timeout`，
// 不带 `_txlock` / `journal_mode` / `query_only`，且本路径**只**在它上面跑一条 `VACUUM INTO`。
//
// ★ 快照内容与写路径**同源**：`VACUUM INTO` 读到的是含 `-wal` 已提交页的完整一致态（SQLite 官方语义
// —— 备份**不需要**先 checkpoint 源库；那条 TRUNCATE 只是顺手把源库 -wal 收小，不是快照完整性的前提）。
func BackupDefaultReadOnly(dbPath string) (string, error) {
	return backupDefaultRO(dbPath, BackupDir(), false)
}

// backupDefault —— BackupDefault 的可测内核：dir 显式给、skipVolumeGuard 只供同包测试。
func (d *DB) backupDefault(dir string, skipVolumeGuard bool) (string, error) {
	return backupSeq(d.path, dir, func(dst string) error { return d.backupTo(dst, skipVolumeGuard) })
}

// backupDefaultRO —— BackupDefaultReadOnly 的可测内核（同一条五步序列 · **只**换「用哪条句柄落快照」）。
func backupDefaultRO(dbPath, dir string, skipVolumeGuard bool) (string, error) {
	return backupSeq(dbPath, dir, func(dst string) error {
		if !skipVolumeGuard {
			if err := ensureBackupTargetDifferentVolume(dbPath, dst); err != nil {
				return err
			}
		}
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("zdb: 备份目标已存在（%s）—— VACUUM INTO 要求目标不存在", dst)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("zdb: 建备份目录失败: %w", err)
		}
		// 源库以「只读用途」句柄开（✗ 不迁移 / ✗ 不体检写面 / ✗ 不打标 / ✗ 不 checkpoint）。
		src, err := openSnapshotSource(dbPath)
		if err != nil {
			return err
		}
		defer src.Close()
		return vacuumInto(src, dst)
	})
}

// openSnapshotSource —— 备份源库的句柄（**只读用途** · DSN 见 `BuildSnapshotSourceDSN` 的头注：
// 为何两条真·只读档都跑不了 `VACUUM INTO`）。本路径只跑一条 `VACUUM INTO`，不进应用事务、不改库头。
func openSnapshotSource(dbPath string) (*sql.DB, error) {
	h, err := sql.Open("sqlite", BuildSnapshotSourceDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("zdb: 开源库（只读用途）失败: %w", err)
	}
	h.SetMaxOpenConns(1)
	h.SetMaxIdleConns(1)
	return h, nil
}

// vacuumInto —— 把给定句柄上的库 `VACUUM INTO` 成一枚新快照（目标须不存在；**不改源库** · 官方语义）。
func vacuumInto(src *sql.DB, dst string) error {
	q := "VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err := src.Exec(q); err != nil {
		return fmt.Errorf("zdb: VACUUM INTO 快照失败: %w", err)
	}
	return nil
}

// backupSeq —— 五步序列的共用体（写路径 / 只读路径**只在**「用哪条句柄落快照」这一点分叉）：
//
//	① `statfs` 预检 ≥2× 库大小（不足 ⇒ fail-closed、**不产半件**）；
//	② UTC 命名 `zerg-<YYYYMMDD-HHMMSS>.db`（同秒撞名 ⇒ 退避 +1s）；
//	③ 落快照（`vacuum` 回调：写路径 = checkpoint ＋ `backupTo` · 只读路径 = 只读句柄 ＋ `vacuumInto`）；
//	④ 写侧车 `zerg-<UTC>.db.meta.json`（八格）；
//	⑤ GFS 逐代 prune（日14/周8/月12 · 保底 ≥3 · **只**删匹配命名件、✗ 不删保底那份）。
func backupSeq(srcPath, dir string, vacuum func(dst string) error) (string, error) {
	if dir == "" {
		return "", errors.New("zdb: 取不到家目录，无法定备份落点")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("zdb: 建备份目录失败: %w", err)
	}
	// ① 空间预检（§六 Q24 · §四.2 甲「twice the size of the original database file」）。
	if err := backupSpacePrecheckDir(dir, srcPath); err != nil {
		return "", err
	}
	// ② UTC 命名 ＋ 同秒退避（VACUUM INTO 非幂等：目标须不存在 / 为空）。
	dst, err := pickSnapshotName(dir)
	if err != nil {
		return "", err
	}
	// ③ 真落快照。
	if err := vacuum(dst); err != nil {
		return "", err
	}
	// ④ 侧车（八格）。
	if err := writeSidecar(dst); err != nil {
		_ = os.Remove(dst)
		return "", err
	}
	// ⑤ GFS 轮转（删旧代；**保底份不删**）。
	if _, err := PruneBackups(dir, GFSDaily, GFSWeekly, GFSMonthly, GFSFloor); err != nil {
		return dst, fmt.Errorf("zdb: 备份已落 %s，但 GFS 轮转失败: %w", dst, err)
	}
	return dst, nil
}

// pickSnapshotName —— 在 dir 里挑一个尚未占用的 `zerg-<UTC>.db`（同秒撞名 ⇒ 逐秒退避，上限 120 秒）。
func pickSnapshotName(dir string) (string, error) {
	base := time.Now().UTC()
	for i := 0; i < 120; i++ {
		name, _ := snapshotName(base.Add(time.Duration(i) * time.Second))
		cand := filepath.Join(dir, name)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("zdb: 备份目录 %s 内同秒命名撞满，无可用名", dir)
}

// backupSpacePrecheck —— 备份前 `statfs` 预检（§四.2 附 K M4 · §六 Q24）：目标卷空闲须 ≥ **2×** 库文件大小。
//
// 依据：`VACUUM INTO` 官方逐字「as much as twice the size of the original database file is required in free」。
// 不足 ⇒ fail-closed、**不产半件**（本判在本动作最前面，VACUUM INTO 之前）。
func (d *DB) backupSpacePrecheck(dir string) error { return backupSpacePrecheckDir(dir, d.path) }

// backupSpacePrecheckDir —— 空间预检的取数侧（写路径 / 只读路径共用：只是「库路径」由调用方给）。
func backupSpacePrecheckDir(dir, dbPath string) error {
	free, err := statfsFreeBytes(dir)
	if err != nil {
		return fmt.Errorf("%w：探测备份目标卷（%s）空闲失败: %v", ErrVolume, dir, err)
	}
	var size uint64
	if st, err := os.Stat(dbPath); err == nil {
		size = uint64(st.Size())
	}
	return spacePrecheck(free, size)
}

// spacePrecheck —— 空间预检的**纯判据**（可定向单测 · 两态）：
//
//	可用 ≥ 2×库 ⇒ 过；可用 < 2×库 ⇒ 拒。
func spacePrecheck(free, dbSize uint64) error {
	need := dbSize * 2
	if free < need {
		return fmt.Errorf("zdb: 备份目标卷空闲不足（可用 %d B < 需 2×库 = %d B）⇒ fail-closed，✗ 不产半件（§六 Q24）", free, need)
	}
	return nil
}

// writeSidecar —— `(*DB)` 形态的入口（迁移前置快照 `migrate.go` 用它；真身**同一处** ⇒ ✗ 不另立第二份实现）。
func (d *DB) writeSidecar(snapshot string) error { return writeSidecar(snapshot) }

// writeSidecar —— 写侧车 `zerg-<UTC>.db.meta.json`（八格 · §六 Q26）；原子落（.tmp + Rename）。
func writeSidecar(snapshot string) error {
	meta, err := buildMeta(snapshot)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("zdb: 序列化侧车失败: %w", err)
	}
	b = append(b, '\n')
	mp := snapshot + ".meta.json"
	tmp := mp + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("zdb: 写侧车失败: %w", err)
	}
	if err := os.Rename(tmp, mp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 侧车就位失败: %w", err)
	}
	return nil
}

// buildMeta —— 采集侧车八格（快照**只读**开 ⇒ ✗ 不给快照建 -wal）。
func buildMeta(snapshot string) (BackupMeta, error) {
	var meta BackupMeta
	sum, size, err := fileSHA256(snapshot)
	if err != nil {
		return meta, err
	}
	meta.SHA256 = sum
	meta.SizeBytes = size
	meta.CreatedAtUTC = time.Now().UTC().Format(time.RFC3339)
	h, err := openSnapshotRO(snapshot)
	if err != nil {
		return meta, fmt.Errorf("zdb: 开快照体检连接失败: %w", err)
	}
	defer h.Close()
	if err := scanSnapshotHealth(h, &meta); err != nil {
		return meta, err
	}
	rows, err := countTableRows(h)
	if err != nil {
		return meta, err
	}
	meta.Rows = rows
	return meta, nil
}

// openSnapshotRO —— 以**只读**、**不动 journal_mode** 的方式开一枚快照（快照是单文件，天然无 -wal/-shm）。
func openSnapshotRO(path string) (*sql.DB, error) {
	h, err := sql.Open("sqlite", "file:"+dsnEscapePath(path)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	h.SetMaxOpenConns(1)
	h.SetMaxIdleConns(1)
	return h, nil
}

// scanSnapshotHealth —— 读快照四格（integrity_check / user_version / application_id / page_count）。
func scanSnapshotHealth(h *sql.DB, meta *BackupMeta) error {
	if err := h.QueryRow("PRAGMA integrity_check").Scan(&meta.IntegrityCheck); err != nil {
		return fmt.Errorf("zdb: 读快照 integrity_check 失败: %w", err)
	}
	if err := h.QueryRow("PRAGMA user_version").Scan(&meta.UserVersion); err != nil {
		return fmt.Errorf("zdb: 读快照 user_version 失败: %w", err)
	}
	if err := h.QueryRow("PRAGMA application_id").Scan(&meta.ApplicationID); err != nil {
		return fmt.Errorf("zdb: 读快照 application_id 失败: %w", err)
	}
	if err := h.QueryRow("PRAGMA page_count").Scan(&meta.PageCount); err != nil {
		return fmt.Errorf("zdb: 读快照 page_count 失败: %w", err)
	}
	return nil
}

// countTableRows —— 逐表 COUNT(*)（backupRowTables 闭集）。
func countTableRows(h *sql.DB) (map[string]int, error) {
	out := make(map[string]int, len(backupRowTables))
	for _, tbl := range backupRowTables {
		var n int
		if err := h.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			return nil, fmt.Errorf("zdb: 数快照表 %s 行失败: %w", tbl, err)
		}
		out[tbl] = n
	}
	return out, nil
}

// fileSHA256 —— 算件 sha256 ＋ 字节数。
func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("zdb: 开快照片失败: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("zdb: 算快照 sha256 失败: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// readSidecar —— 读快照侧车（§六 Q26）；缺 / 坏 ⇒ fail-closed。
func readSidecar(snapshot string) (BackupMeta, error) {
	var meta BackupMeta
	b, err := os.ReadFile(snapshot + ".meta.json")
	if err != nil {
		return meta, fmt.Errorf("zdb: 读快照侧车失败（缺侧车 ⇒ 拒还原 · §六 Q26）: %w", err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, fmt.Errorf("zdb: 解析快照侧车失败: %w", err)
	}
	return meta, nil
}

// snapshotInfo —— 备份目录里一枚快照的事实。
type snapshotInfo struct {
	Name string
	Time time.Time
}

// listSnapshots —— 列**匹配命名**的快照（`zerg-<UTC>.db`），按时刻降序。
// ★ 不匹配命名的条目一律不入列 ⇒ prune 永远碰不到它们（§六 Q24「只删匹配命名件」）。
func listSnapshots(dir string) ([]snapshotInfo, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("zdb: 读备份目录失败: %w", err)
	}
	out := make([]snapshotInfo, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() || !snapshotNameRE.MatchString(e.Name()) {
			continue
		}
		stamp := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "zerg-"), ".db")
		t, err := time.ParseInLocation(backupStampLayout, stamp, time.UTC)
		if err != nil {
			continue
		}
		out = append(out, snapshotInfo{Name: e.Name(), Time: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// gfsKeepSet —— GFS 保留集（§六 Q24）＝ 日/周/月各取「最近 N 桶的最新一枚」∪ 保底前 floor 枚。
// 输入须按时刻降序（listSnapshots 已保证）。
func gfsKeepSet(snaps []snapshotInfo, dayN, weekN, monthN, floor int) map[string]bool {
	keep := make(map[string]bool, len(snaps))
	// 保底：最近 floor 枚无条件保留（**✗ 不删保底那份**）。
	for i := 0; i < len(snaps) && i < floor; i++ {
		keep[snaps[i].Name] = true
	}
	keepNewestPerBucket := func(bucket func(time.Time) string, n int) {
		if n <= 0 {
			return
		}
		seen := make(map[string]bool)
		kept := 0
		for _, s := range snaps { // 已降序 ⇒ 每桶先见即最新
			k := bucket(s.Time)
			if seen[k] {
				continue
			}
			seen[k] = true
			keep[s.Name] = true
			kept++
			if kept >= n {
				return
			}
		}
	}
	keepNewestPerBucket(func(t time.Time) string { return t.Format("2006-01-02") }, dayN)
	keepNewestPerBucket(func(t time.Time) string { y, w := t.ISOWeek(); return fmt.Sprintf("%04d-W%02d", y, w) }, weekN)
	keepNewestPerBucket(func(t time.Time) string { return t.Format("2006-01") }, monthN)
	return keep
}

// PruneBackups —— GFS 逐代轮转（§六 Q24）：删「不在保留集」的快照 ＋ 其侧车。
// ★ 只删 dir 内**匹配命名**的件；保底份永不删。返回被删件名。
func PruneBackups(dir string, dayN, weekN, monthN, floor int) ([]string, error) {
	snaps, err := listSnapshots(dir)
	if err != nil {
		return nil, err
	}
	keep := gfsKeepSet(snaps, dayN, weekN, monthN, floor)
	removed := make([]string, 0)
	for _, s := range snaps {
		if keep[s.Name] {
			continue
		}
		p := filepath.Join(dir, s.Name)
		if err := os.Remove(p); err != nil {
			return removed, fmt.Errorf("zdb: 删旧备份 %s 失败: %w", p, err)
		}
		removed = append(removed, s.Name)
		mp := p + ".meta.json"
		if _, err := os.Stat(mp); err == nil {
			if err := os.Remove(mp); err != nil {
				return removed, fmt.Errorf("zdb: 删旧备份侧车 %s 失败: %w", mp, err)
			}
		}
	}
	return removed, nil
}

// ─────────────────── T2 · 还原（§六 Q23 · 照 migrateChatDB 四步体例） ───────────────────

// pairedJournalGuard —— 还原前的**配对守卫**（§四.2 乙 ② 逐字）：目标处**非空** `-wal` ⇒ 拒。
//
// 依据 = 配对原则（「Mispairing database files and hot journals」）：库与 WAL 是配对，
// 混用不匹配的 WAL 会弄坏还原库 ⇒ ✗ 不许静默盖。
func pairedJournalGuard(target string) error {
	wal := target + "-wal"
	if st, err := os.Stat(wal); err == nil && st.Size() > 0 {
		return fmt.Errorf("zdb: 目标处 -wal 非空（%s · %d B）—— 库与 WAL 是配对，混用不匹配 WAL 会弄坏还原库；须先确认无活动写者并清目标处 -wal/-shm（§四.2 乙 ②）", wal, st.Size())
	}
	return nil
}

// removePairedJournals —— 删目标处配对的 `-wal`/`-shm`（§四.2 乙 ② 逐字：库与 WAL 是配对）。
func removePairedJournals(target string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		p := target + suffix
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("zdb: 删目标处 %s 失败: %w", suffix, err)
		}
	}
	return nil
}

// Restore —— 从快照恢复就位（§六 Q23；照 `core/internal/chat/chat_store_migrate.go:92-150` 四步体例）。
//
// 铁律：只用 SQLite API（`VACUUM INTO`）取一致快照，✗ 不裸拷 db/-wal（WAL 是持久态的一部分）。
//
//	① `VACUUM INTO ?` —— 从快照再产一份一致**单文件**到目标旁临时件 `to+".restoring"`；
//	② 开临时件**体检** —— integrity_check ＋ `foreign_key_check`（§六 Q27）＋ application_id ＋ user_version；
//	③ **逐表行数对拍** —— 快照 ⟷ 临时件 ⟷ 侧车记的行数；
//	④ **先删目标处配对 `-wal`/`-shm`**（§四.2 乙 ②）→ `os.Rename` 就位。
//
// ★ 自证以侧车为准（§六 Q26）：缺侧车 / sha256 不符 / user_version 不符 / 行数不符 ⇒ fail-closed。
func Restore(from, to string) error {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return errors.New("zdb: Restore 的 from / to 不能为空")
	}
	// ⓪ 自证：侧车必须在 ＋ sha256 逐字节对得上（§六 Q26）。
	meta, err := readSidecar(from)
	if err != nil {
		return err
	}
	sum, _, err := fileSHA256(from)
	if err != nil {
		return err
	}
	if meta.SHA256 == "" || sum != meta.SHA256 {
		return fmt.Errorf("zdb: 快照 sha256 与侧车不符（盘上 %s / 侧车 %q）⇒ 拒还原（§六 Q26 自证）", sum, meta.SHA256)
	}
	// 配对守卫：目标处有非空 -wal ⇒ 拒（✗ 不静默盖）。
	if err := pairedJournalGuard(to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("zdb: 建目标目录失败: %w", err)
	}
	// ① VACUUM INTO：从快照产一份一致单文件到目标旁临时件（只读源 ⇒ ✗ 不动快照）。
	tmp := to + ".restoring"
	_ = os.Remove(tmp)
	src, err := openSnapshotRO(from)
	if err != nil {
		return fmt.Errorf("zdb: 开快照失败: %w", err)
	}
	defer src.Close()
	q := "VACUUM INTO '" + strings.ReplaceAll(tmp, "'", "''") + "'"
	if _, err := src.Exec(q); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: VACUUM INTO 快照失败: %w", err)
	}
	// ② 开临时件体检（三连）＋ user_version 对侧车。
	chk, err := openSnapshotRO(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 开临时快照失败: %w", err)
	}
	checkDB := &DB{db: chk, path: tmp}
	if err := checkDB.HealthCheck(); err != nil {
		chk.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 还原自证体检未过: %w", err)
	}
	var uv int
	if err := chk.QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		chk.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 读临时快照 user_version 失败: %w", err)
	}
	if uv != meta.UserVersion {
		chk.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 还原自证 user_version 不符（临时件 %d / 侧车 %d）⇒ 拒", uv, meta.UserVersion)
	}
	// ③ 逐表行数对拍：快照(src) ⟷ 临时件(chk) ⟷ 侧车(meta.Rows)。
	srcRows, err := countTableRows(src)
	if err != nil {
		chk.Close()
		_ = os.Remove(tmp)
		return err
	}
	tmpRows, err := countTableRows(chk)
	if err != nil {
		chk.Close()
		_ = os.Remove(tmp)
		return err
	}
	for _, tbl := range backupRowTables {
		if srcRows[tbl] != tmpRows[tbl] {
			chk.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("zdb: 还原自证行数对拍不符（表 %s：快照 %d / 临时件 %d）⇒ 拒", tbl, srcRows[tbl], tmpRows[tbl])
		}
		if want, ok := meta.Rows[tbl]; ok && want != tmpRows[tbl] {
			chk.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("zdb: 还原自证行数与侧车不符（表 %s：临时件 %d / 侧车 %d）⇒ 拒", tbl, tmpRows[tbl], want)
		}
	}
	chk.Close()
	// ④ 先删目标处配对 -wal/-shm（库件一并换 ⇒ 按配对原则处理），再 os.Rename 就位。
	if err := removePairedJournals(to); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, to); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("zdb: 还原就位失败: %w", err)
	}
	return nil
}

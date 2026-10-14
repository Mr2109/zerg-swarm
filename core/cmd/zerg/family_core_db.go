// family_core_db.go —— `zerg core db <check|backup|restore|prune>`（T2④ / T4① · 备份 / 恢复 / 体检 / GFS 轮转）。
//
// 命令入口落在**既有 `core` 族**（设计 §五 Q12 · 任务单 T2④ 逐字：「✗ 不新开族」「形制 = 既有族内具名子动作」）
// —— 登记形制照同族 `core daemon`：一条 `core <族>` 命令 ＋ 子动作在 `inv.args[0]` 里派发。
//
// 本件是**薄壳**（✗ 不在命令层重写逻辑）：五个子动作逐字转发到 `core/internal/store/zdb` 已落的接口 ——
//
//	check   → `zdb.OpenReader`（DSN 带 `_pragma=query_only(ON)` 的只读句柄）＋ 三条体检 pragma
//	backup  → `zdb.BackupDefaultReadOnly(<库路径>)`（**只读**句柄 → VACUUM INTO → 侧车 → GFS prune）
//	           ★ 走 `zdb.Open` 是**写路径**（开库即**迁移**）⇒ 备份会顺带改源库头 · ✗ 不许（见下段）
//	restore → `zdb.Restore(from, to)`（VACUUM INTO → 体检 → 逐表行数对拍 → rename 就位 · 自证以侧车为准）
//	prune   → `zdb.PruneBackups(dir, GFSDaily, GFSWeekly, GFSMonthly, GFSFloor)`
//
// ★ 为什么 `check` 与 `backup` 都**不走** `zdb.Open()`：`zdb.Open` 是**写路径**
// （开库即体检 ＋ **迁移** ＋ application_id 打标 ＋ checkpoint）⇒ 对一枚库跑一次「体检」就写一次、
// 跑一次「备份」还会顺带把迁移跑一遍。
// 设计要的是**只读**动作（真库上跑 ⇒ 一个字节不写）：
//
//	· `check`  → `zdb.OpenReader` ＋**同一组**三条 pragma（integrity_check / foreign_key_check /
//	  application_id —— 与 `zdb.HealthCheck` 逐条同口径；✗ 不另立第二套判据）；
//	· `backup` → `zdb.BackupDefaultReadOnly`（**不跑迁移**的开库路径 ＋ `VACUUM INTO` 快照）。
//
// 落点口径（照 `zdb` 包，不另造旋钮）：
//
//	· 库落点 = `--db <库路径>`（不给 ⇒ `zdb.DefaultPath()` = `<仓>/.runtime/zerg.db`）；
//	· 备份目录 = `zdb.BackupDir()`（`~/.zerg/backup/`，可被环境变量 `ZERG_BACKUP_DIR` 覆盖 —— 该旋钮
//	  是 `zdb` 包自己的口径）；★ 硬规矩③「备份落**另一卷**」在 `zdb` 里判：库与备份目标**同卷 ⇒ 拒**。
//
// 三态（§4.1 K7 · `--dry-run` 与 `--yes` 同给 ⇒ 共享判定 `dryRunYesConflict` 退 2）：
//
//	`--dry-run` ⇒ 计划件 · 零副作用 · 退 0；真跑要 `--yes`；`restore` 另要 `--confirm=<目标库路径>`
//	（值须与 `--to` **逐字相同** —— 给错值 = 拒绝，不是「当没给」）；缺确认档 ⇒ fail-closed 退 2。
//
// 退码：0 成 · 1 判红/动作失败 · 2 用法错 · 8 拿不到结论（库不在盘 / 只读句柄开不起来）。
package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/store/zdb"
)

// coreDbActions —— 子动作闭集（派发与报面**共用这一份** ⇒ ✗ 不在第二处再判一次）。
var coreDbActions = []string{"check", "backup", "restore", "prune", "stamp"}

// coreDbFields —— `--json` 可取的全部字段（五动作的**并集**：一条 `core db` 命令一张字段表）。
//
// ★ 形状稳：这 27 格名字与语义**只增不改**（改名 = 破下游）；某个动作取不到的格**整键略去**
// （✗ 绝不落 `0` / 空串 —— 与 API 的 `omitempty` 同一条铁律，收受端 `emitSelected` 同口径）。
var coreDbFields = []string{
	"action",
	"db",
	"exists",
	"size_bytes",
	"user_version",
	"application_id",
	"application_id_ok",
	"integrity_check",
	"foreign_key_violations",
	"page_count",
	"journal_mode",
	"wal_bytes",
	"backup_dir",
	"snapshot",
	"snapshot_sha256",
	"snapshot_size_bytes",
	"pruned_count",
	"pruned",
	"restore_from",
	"restore_to",
	"gfs_day",
	"gfs_week",
	"gfs_month",
	"gfs_floor",
	"removed_count",
	"removed",
	"verdict",
}

// ── 派发 ──────────────────────────────────────────────────────────────────────────────────────

// cmdCoreDb —— `zerg core db <动作>`（薄壳 · 四个动作各转发到 `zdb` 的一处接口）。
func cmdCoreDb(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 门98（`check-dryrun-yes-conflict-coverage`）调用点粒度：`--dry-run` 与 `--yes` 同给 ⇒
	//   共享判定必须在任何盘面动作**之前**跑（同给时先早退 ⇒ `--yes` 被静默吞）。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if len(inv.args) == 0 {
		coreDbUsageErr(inv, stderr, "缺动作")
		return exitUsage
	}
	action := strings.TrimSpace(inv.args[0])
	if len(inv.args) > 1 {
		coreDbUsageErr(inv, stderr, fmt.Sprintf("多给了位置参数 %q（本命令只收**一个**动作）", inv.args[1]))
		return exitUsage
	}
	switch action {
	case "check":
		return coreDbCheck(inv, stdout, stderr)
	case "backup":
		return coreDbBackup(inv, stdout, stderr)
	case "restore":
		return coreDbRestore(inv, stdout, stderr)
	case "prune":
		return coreDbPrune(inv, stdout, stderr)
	case "stamp":
		return coreDbStamp(inv, stdout, stderr)
	}
	coreDbUsageErr(inv, stderr, fmt.Sprintf("未知动作 %q", action))
	return exitUsage
}

// coreDbUsageErr —— 用法错的统一报面（K14：指出哪里错 ＋ 下一步；✗ 不在四处各写一遍）。
func coreDbUsageErr(inv *invocation, stderr io.Writer, why string) {
	msg := fmt.Sprintf("`core db` 用法错：%s（动作闭集 = %s）", why, strings.Join(coreDbActions, " | "))
	inv.setErr("usage", "bad_action", msg)
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(stderr, "先看用法：%s core db --help\n", progName)
}

// coreDbPath —— 取 `--db`（不给 ⇒ `zdb` 的默认落点 `<仓>/.runtime/zerg.db`）。
func coreDbPath(inv *invocation) string {
	if p := strings.TrimSpace(inv.flagVal("--db")); p != "" {
		return p
	}
	return zdb.DefaultPath()
}

// coreDbNeedExistingFile —— 「要一枚真在盘上的件」的前置（库 / 快照）。不在 ⇒ 退码 8（✗ 不凭空造读数）。
func coreDbNeedExistingFile(inv *invocation, stderr io.Writer, what, p string) (os.FileInfo, int) {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		msg := fmt.Sprintf("%s不在盘上（%s）⇒ 不给结论（退码 8）", what, p)
		inv.setErr("blocked", "path_absent", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return nil, exitBlocked
	}
	return st, exitOK
}

// ── check（只读体检 · 一条 pragma 都不写）────────────────────────────────────────────────────

// coreDbCheck —— `zerg core db check [--db <库路径>]`：只读体检（真库上跑也一个字节不写）。
func coreDbCheck(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	dbPath := coreDbPath(inv)
	st, rc := coreDbNeedExistingFile(inv, stderr, "库", dbPath)
	if rc != exitOK {
		return rc
	}
	rdb, err := zdb.OpenReader(dbPath)
	if err != nil {
		msg := fmt.Sprintf("只读句柄开不起来（%s）：%v ⇒ 不给结论（退码 8）", dbPath, err)
		inv.setErr("blocked", "reader_unavailable", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitBlocked
	}
	defer rdb.Close()

	row := map[string]string{
		"action":     "check",
		"db":         dbPath,
		"exists":     "true",
		"size_bytes": strconv.FormatInt(st.Size(), 10),
		"wal_bytes":  coreDbFileSize(dbPath + "-wal"),
	}
	// ① integrity_check（须回**单行** `ok`）
	ic, icOK := coreDbIntegrity(rdb)
	row["integrity_check"] = ic
	// ② foreign_key_check（须回 **0 行** —— `integrity_check` **不查外键**，官方逐字 · §六 Q27）
	fk, fkOK := coreDbFKViolations(rdb)
	row["foreign_key_violations"] = strconv.Itoa(fk)
	// ③ application_id（`0` = 未打标 ⇒ 放行；非 0 且 ≠ 本仓身份标 ⇒ 判红）
	appID, appOK := coreDbApplicationID(rdb)
	row["application_id"] = strconv.FormatInt(appID, 10)
	row["application_id_ok"] = strconv.FormatBool(appOK)
	// 只读读数三格（不判红）
	row["user_version"] = coreDbScalarInt(rdb, "PRAGMA user_version")
	row["page_count"] = coreDbScalarInt(rdb, "PRAGMA page_count")
	row["journal_mode"] = coreDbScalarText(rdb, "PRAGMA journal_mode")

	row["verdict"] = "ok"
	verdictRC := exitOK
	if !icOK || !fkOK || !appOK {
		row["verdict"] = "failed"
		verdictRC = exitFail
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "库                  : %s\n", dbPath)
	fmt.Fprintf(stdout, "字节 / -wal 字节    : %s / %s\n", row["size_bytes"], orX(row["wal_bytes"], "（无 -wal）"))
	fmt.Fprintf(stdout, "integrity_check     : %s\n", ic)
	fmt.Fprintf(stdout, "foreign_key_check   : %s 行（须 0）\n", row["foreign_key_violations"])
	fmt.Fprintf(stdout, "application_id      : %s（本仓身份标 = %d · 0 = 未打标放行）\n",
		row["application_id"], zdb.ZergApplicationID)
	fmt.Fprintf(stdout, "user_version / 页数 : %s / %s\n", orX(row["user_version"], "-"), orX(row["page_count"], "-"))
	fmt.Fprintf(stdout, "journal_mode        : %s\n", orX(row["journal_mode"], "-"))
	fmt.Fprintf(stdout, "判词                : %s\n", row["verdict"])
	fmt.Fprintf(stderr, "%s: 只读体检（`zdb.OpenReader` 的 `query_only=ON` 句柄）—— 一个字节都没写\n", progName)
	return verdictRC
}

// ── backup（GFS 快照 · 转发 `zdb.BackupDefaultReadOnly`）─────────────────────────────────────

// coreDbBackup —— `zerg core db backup [--db <库路径>] [--dry-run | --yes]`。
func coreDbBackup(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	dbPath := coreDbPath(inv)
	dir := zdb.BackupDir()
	if inv.dryRun {
		coreDbBackupPlan(stdout, dbPath, dir)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未碰任何库、未落任何件）\n")
		return exitOK
	}
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	if _, rc := coreDbNeedExistingFile(inv, stderr, "库", dbPath); rc != exitOK {
		return rc
	}
	// ★ 只读路径（✗ 不走 `zdb.Open` —— 那是写路径：开库即迁移 ＋ 打身份标 ＋ checkpoint）：
	//   备份是**日常只读动作** ⇒ 源库一个字节都不许写（`BackupDefaultReadOnly` 不跑迁移 / 不体检写面 / 不打标 / 不 checkpoint）。
	snap, berr := zdb.BackupDefaultReadOnly(dbPath)
	if berr != nil {
		msg := fmt.Sprintf("备份失败（`zdb.BackupDefaultReadOnly`）：%v ⇒ 退码 1", berr)
		inv.setErr("failed", "backup_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":              "backup",
		"db":                  dbPath,
		"backup_dir":          dir,
		"snapshot":            snap,
		"snapshot_sha256":     fileSHA256(snap),
		"snapshot_size_bytes": coreDbFileSize(snap),
		"verdict":             "ok",
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "库        : %s\n", dbPath)
	fmt.Fprintf(stdout, "备份目录  : %s\n", dir)
	fmt.Fprintf(stdout, "快照      : %s\n", snap)
	fmt.Fprintf(stdout, "sha256/字节: %s / %s\n", row["snapshot_sha256"], row["snapshot_size_bytes"])
	fmt.Fprintf(stdout, "判词      : ok（GFS 日14/周8/月12 · 保底 ≥3 已在 `zdb` 内随本动作跑完）\n")
	fmt.Fprintf(stderr, "%s: 备份落**另一卷**（硬规矩③ 由 `zdb` 判）· 源库**只读**开（✗ 不迁移 · ✗ 不 checkpoint · 一个字节不写）\n", progName)
	return exitOK
}

// coreDbBackupPlan —— `backup --dry-run` 的计划件（零副作用：只读两条路径事实，不建目录、不碰库）。
func coreDbBackupPlan(stdout io.Writer, dbPath, dir string) {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未开库、未 VACUUM INTO、未写侧车、未 prune）")
	fmt.Fprintf(stdout, "  动作   : %s core db backup\n", progName)
	fmt.Fprintf(stdout, "  源库   : %s（在盘=%t）\n", dbPath, coreDbPathExists(dbPath))
	fmt.Fprintf(stdout, "  备份目录: %s（在盘=%t · 可被 `ZERG_BACKUP_DIR` 覆盖）\n", dir, coreDbPathExists(dir))
	fmt.Fprintf(stdout, "  五步   : 空间预检 → UTC 命名 → VACUUM INTO（**只读**源库 · ✗ 不跑迁移）→ 侧车八格 → GFS prune（日%d/周%d/月%d · 保底 %d）\n",
		zdb.GFSDaily, zdb.GFSWeekly, zdb.GFSMonthly, zdb.GFSFloor)
	fmt.Fprintf(stdout, "  硬规矩 : ③ 备份目标须与库**不同卷**（同卷 ⇒ `zdb` 当场拒）· 源库只读开（迁移一步不跑）\n")
	fmt.Fprintf(stdout, "  真跑要 : %s core db backup --yes\n", progName)
}

// ── restore（D3 · 转发 `zdb.Restore`）────────────────────────────────────────────────────────

// coreDbRestore —— `zerg core db restore --from <快照> --to <库路径> --confirm=<库路径> --yes`。
func coreDbRestore(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	from := strings.TrimSpace(inv.flagVal("--from"))
	// ★ `--to` 在解析面是**类型化字段**（`setValueFlag` 的 `case "--to": inv.to = val`），
	//   不进 `kv` ⇒ 必须读 `inv.to`（照 `frpFlagField` 那一条「读面经 inv.<field>」的口径）。
	to := strings.TrimSpace(inv.to)
	if from == "" || to == "" {
		coreDbUsageErr(inv, stderr, "`restore` 要 `--from <快照>` 与 `--to <库路径>` 两枚都到（✗ 不猜落点）")
		return exitUsage
	}
	// ① `--dry-run`：计划件 · 零副作用 · 退 0（三态里**唯一**会返回 0 的那一态）。
	if inv.dryRun {
		coreDbRestorePlan(stdout, from, to)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改任何库、未落任何件）\n")
		return exitOK
	}
	// ② D3 确认档：`--confirm=<目标库路径>`（值须与 `--to` 逐字相同）与 `--yes` **同时到**。
	if !inv.confirmGiven {
		msg := "`core db restore` 是 D3 档（**覆盖 `--to` 指向的库**）——**缺确认 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", to)
		fmt.Fprintf(stderr, "先看计划件：%s core db restore --from <快照> --to <库路径> --dry-run\n", progName)
		fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.confirm != to {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，--to 是 %q）⇒ 不执行", inv.confirm, to)
		inv.setErr("usage", "confirm_mismatch", msg)
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与 `--to` **逐字相同** —— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		coreDbRestorePlan(stdout, from, to)
		msg := "`core db restore` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "确认档齐了才真跑：%s core db restore --from %s --to %s --confirm=%s --yes\n", progName, from, to, to)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	// ③ 真跑：两件前置（快照在盘）→ 转发 `zdb.Restore`（自证 / 体检 / 行数对拍 / rename 全在包里）。
	if _, rc := coreDbNeedExistingFile(inv, stderr, "快照", from); rc != exitOK {
		return rc
	}
	if err := zdb.Restore(from, to); err != nil {
		msg := fmt.Sprintf("还原失败（`zdb.Restore`）：%v ⇒ 退码 1（目标库**未必**被动过 —— 自证/体检不过时它在 rename 之前就退了）", err)
		inv.setErr("failed", "restore_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":              "restore",
		"restore_from":        from,
		"restore_to":          to,
		"snapshot_sha256":     fileSHA256(from),
		"snapshot_size_bytes": coreDbFileSize(from),
		"verdict":             "ok",
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "快照 : %s\n", from)
	fmt.Fprintf(stdout, "就位 : %s\n", to)
	fmt.Fprintf(stdout, "判词 : ok（四步走完：VACUUM INTO → 体检 → 逐表行数对拍 → rename 就位）\n")
	return exitOK
}

// coreDbRestorePlan —— `restore --dry-run` 的计划件（零副作用：只读两条路径事实）。
func coreDbRestorePlan(stdout io.Writer, from, to string) {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未 VACUUM INTO、未改目标库、未删配对 -wal/-shm）")
	fmt.Fprintf(stdout, "  动作   : %s core db restore（D3 · **覆盖目标库**）\n", progName)
	fmt.Fprintf(stdout, "  快照   : %s（在盘=%t）\n", from, coreDbPathExists(from))
	fmt.Fprintf(stdout, "  目标库 : %s（在盘=%t）\n", to, coreDbPathExists(to))
	fmt.Fprintf(stdout, "  四步   : VACUUM INTO 临时件 → 体检(integrity/FK/身份标) → 逐表行数对拍（快照 ⟷ 临时件 ⟷ 侧车）→ 先删配对 -wal/-shm 再 rename 就位\n")
	fmt.Fprintf(stdout, "  自证   : 侧车缺 / sha256 不符 / user_version 不符 / 行数不符 ⇒ `zdb` 里 fail-closed\n")
	fmt.Fprintf(stdout, "  真跑要 : %s core db restore --from %s --to %s --confirm=%s --yes\n", progName, from, to, to)
}

// ── prune（GFS 逐代轮转 · 转发 `zdb.PruneBackups`）────────────────────────────────────────────

// coreDbPrune —— `zerg core db prune [--dry-run | --yes]`：备份目录内的 GFS 轮转（只删**匹配命名**件）。
func coreDbPrune(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	dir := zdb.BackupDir()
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 一个件都没删）")
		fmt.Fprintf(stdout, "  动作   : %s core db prune\n", progName)
		fmt.Fprintf(stdout, "  备份目录: %s（在盘=%t）\n", dir, coreDbPathExists(dir))
		fmt.Fprintf(stdout, "  GFS    : 日 %d / 周 %d / 月 %d · 保底 %d 份（`zdb` 内口径）\n",
			zdb.GFSDaily, zdb.GFSWeekly, zdb.GFSMonthly, zdb.GFSFloor)
		fmt.Fprintf(stdout, "  只删   : 匹配命名 `zerg-<UTC>.db` 的件 ＋ 其侧车（✗ 别的一律不碰 · 保底份永不删）\n")
		fmt.Fprintf(stdout, "  真跑要 : %s core db prune --yes\n", progName)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未删任何件）\n")
		return exitOK
	}
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	removed, err := zdb.PruneBackups(dir, zdb.GFSDaily, zdb.GFSWeekly, zdb.GFSMonthly, zdb.GFSFloor)
	if err != nil {
		msg := fmt.Sprintf("轮转失败（`zdb.PruneBackups`）：%v ⇒ 退码 1", err)
		inv.setErr("failed", "prune_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	row := map[string]string{
		"action":        "prune",
		"backup_dir":    dir,
		"gfs_day":       strconv.Itoa(zdb.GFSDaily),
		"gfs_week":      strconv.Itoa(zdb.GFSWeekly),
		"gfs_month":     strconv.Itoa(zdb.GFSMonthly),
		"gfs_floor":     strconv.Itoa(zdb.GFSFloor),
		"removed_count": strconv.Itoa(len(removed)),
		"removed":       strings.Join(removed, ","),
		"verdict":       "ok",
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "备份目录 : %s\n", dir)
	fmt.Fprintf(stdout, "GFS      : 日 %s / 周 %s / 月 %s · 保底 %s\n",
		row["gfs_day"], row["gfs_week"], row["gfs_month"], row["gfs_floor"])
	fmt.Fprintf(stdout, "删了     : %s 件%s\n", row["removed_count"],
		coreDbMaybeList(row["removed"]))
	fmt.Fprintf(stdout, "判词     : ok\n")
	return exitOK
}

// ── stamp（**只打标不迁移** · 转发 `zdb.StampApplicationID`）──────────────────────────────────

// coreDbStamp —— `zerg core db stamp [--db <库路径>] [--dry-run | --yes]`：给一枚**已存在**的库补
// `PRAGMA application_id` 身份标（§三.4 附 G③ · §六 Q28）。本动作存在的**唯一**理由：
//
// ★ **✗ 不走 `zdb.Open`**（那是写路径：同一函数体里 `migrate()` 排在打标**之前** ⇒ 拿它补标会
// **顺带把迁移跑一遍** ⇒ 真库 `user_version` 1→3）。转发 `zdb.StampApplicationID` —— 开库句柄照
// **备份源库**那条只读用途路径（`BuildSnapshotSourceDSN`：只带 `busy_timeout`）⇒ 一条写步骤都不走：
//
//	✗ 不跑迁移 · ✗ 不体检写面 · ✗ 不 checkpoint · ✗ 不动 `user_version`
//
// 只做 `application_id` 的**三态** ＋ **回读自证**（与 `zdb` 的 `ensureApplicationID` 逐条**同口径**）：
//
//	① 值 == `zdb.ZergApplicationID` ⇒ **空动作**（一字节不写）；
//	② 值 == 0（未打标）⇒ 写一次 ＋ 回读自证（回读 ≠ 本标 ⇒ fail-closed 退 1）；
//	③ 值非 0 且 ≠ 本标 ⇒ **拒执**退 1（`zdb.ErrNotZergDB` · ✗ 不覆盖别人的身份标）。
//
// 写面 ⇒ 与同族 `backup` / `prune` **同档**：`--dry-run` 出计划件（零副作用）· 真跑要 `--yes`。
func coreDbStamp(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	dbPath := coreDbPath(inv)
	if inv.dryRun {
		coreDbStampPlan(stdout, dbPath)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未开库、未写库头、未打标）\n")
		return exitOK
	}
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	if _, rc := coreDbNeedExistingFile(inv, stderr, "库", dbPath); rc != exitOK {
		return rc
	}
	before, after, serr := zdb.StampApplicationID(dbPath)
	if serr != nil {
		msg := fmt.Sprintf("打标未做（`zdb.StampApplicationID`）：%v ⇒ 退码 1（fail-closed：值非 0 且非本标 ⇒ ✗ 不覆盖别人的身份标）", serr)
		inv.setErr("failed", "stamp_failed", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}
	// 回读三件：与同族 `check` **同一组**只读探针（✗ 不另立第二套判据）。
	rdb, rerr := zdb.OpenReader(dbPath)
	if rerr != nil {
		msg := fmt.Sprintf("打标已落（application_id=%d），但只读回读句柄开不起来（%s）：%v ⇒ 不给结论（退码 8）", after, dbPath, rerr)
		inv.setErr("blocked", "reader_unavailable", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitBlocked
	}
	defer rdb.Close()
	ic, icOK := coreDbIntegrity(rdb)
	fk, fkOK := coreDbFKViolations(rdb)
	appID, appOK := coreDbApplicationID(rdb)
	row := map[string]string{
		"action":                 "stamp",
		"db":                     dbPath,
		"exists":                 "true",
		"size_bytes":             coreDbFileSize(dbPath),
		"wal_bytes":              coreDbFileSize(dbPath + "-wal"),
		"application_id":         strconv.FormatInt(appID, 10),
		"application_id_ok":      strconv.FormatBool(appOK),
		"integrity_check":        ic,
		"foreign_key_violations": strconv.Itoa(fk),
		"user_version":           coreDbScalarInt(rdb, "PRAGMA user_version"),
		"page_count":             coreDbScalarInt(rdb, "PRAGMA page_count"),
		"journal_mode":           coreDbScalarText(rdb, "PRAGMA journal_mode"),
		"verdict":                "ok",
	}
	verdictRC := exitOK
	if !icOK || !fkOK || !appOK {
		row["verdict"] = "failed"
		verdictRC = exitFail
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "库                  : %s\n", dbPath)
	fmt.Fprintf(stdout, "改前 / 改后         : %d / %d（本仓身份标 = %d = 0x%X · 0 = 未打标 ⇒ 补标 · 非 0 他标 ⇒ 拒执）\n",
		before, after, int64(zdb.ZergApplicationID), int64(zdb.ZergApplicationID))
	fmt.Fprintf(stdout, "integrity_check     : %s\n", ic)
	fmt.Fprintf(stdout, "foreign_key_check   : %d 行（须 0）\n", fk)
	fmt.Fprintf(stdout, "application_id      : %s（application_id_ok=%s）\n", row["application_id"], row["application_id_ok"])
	fmt.Fprintf(stdout, "user_version / 页数 : %s / %s（**打标不迁移** ⇒ user_version 一字不动）\n",
		orX(row["user_version"], "-"), orX(row["page_count"], "-"))
	fmt.Fprintf(stdout, "判词                : %s\n", row["verdict"])
	fmt.Fprintf(stderr, "%s: 只打标（`zdb.StampApplicationID` · 句柄照备份源库那条只读用途路径）—— ✗ 不跑迁移 · ✗ 不 checkpoint\n", progName)
	return verdictRC
}

// coreDbStampPlan —— `stamp --dry-run` 的计划件（零副作用：只读一条路径事实，不开库、不写库头）。
func coreDbStampPlan(stdout io.Writer, dbPath string) {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未开库、未写库头、未打标）")
	fmt.Fprintf(stdout, "  动作   : %s core db stamp（**只打标不迁移**）\n", progName)
	fmt.Fprintf(stdout, "  库     : %s（在盘=%t）\n", dbPath, coreDbPathExists(dbPath))
	fmt.Fprintf(stdout, "  三态   : ==本标 %d ⇒ 空动作（一字节不写）· ==0（未打标）⇒ 写一次 ＋ 回读自证 · 其它非 0 ⇒ 拒执（退码 1 · ✗ 不覆盖别人的身份标）\n",
		int64(zdb.ZergApplicationID))
	fmt.Fprintf(stdout, "  不跑   : ✗ 不跑迁移（`user_version` 一字不动）· ✗ 不体检写面 · ✗ 不 checkpoint（句柄只带 `busy_timeout`）\n")
	fmt.Fprintf(stdout, "  真跑要 : %s core db stamp --db %s --yes\n", progName, dbPath)
}

// ── 体检三条（与 `zdb.HealthCheck` 逐条同口径 —— ✗ 不另立第二套判据）───────────────────────────

// coreDbIntegrity —— 体检第 ① 条：`PRAGMA integrity_check` 须回**单行** `ok`。返回（判词, 是否过）。
// ★ 它**不查外键**（那是第 ② 条的事 —— sqlite.org/pragma.html#pragma_foreign_key_check 逐字）。
func coreDbIntegrity(db *sql.DB) (string, bool) {
	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Sprintf("跑不起来：%v", err), false
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return fmt.Sprintf("读结果失败：%v", err), false
		}
		msgs = append(msgs, s)
	}
	if err := rows.Err(); err != nil {
		return fmt.Sprintf("结果读取中断：%v", err), false
	}
	if len(msgs) == 1 && msgs[0] == "ok" {
		return "ok", true
	}
	if len(msgs) == 0 {
		return "（无输出 ⇒ 判不了）", false
	}
	return strings.Join(msgs, " | "), false
}

// coreDbFKViolations —— 体检第 ② 条：`PRAGMA foreign_key_check` 须回 **0 行**。返回（行数, 是否过）。
// ★ 该 pragma **不要求** `foreign_keys=ON`（官方行为：直接扫库）⇒ ✗ 别据「那连接 FK 关了」就跳过。
func coreDbFKViolations(db *sql.DB) (int, bool) {
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return -1, false
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		return n, false
	}
	return n, n == 0
}

// coreDbApplicationID —— 体检第 ③ 条：`PRAGMA application_id`（`0` = 未打标 ⇒ **放行**；
// 非 0 且 ≠ `zdb.ZergApplicationID` ⇒ 判红）。返回（值, 是否过）。
func coreDbApplicationID(db *sql.DB) (int64, bool) {
	var v int64
	if err := db.QueryRow("PRAGMA application_id").Scan(&v); err != nil {
		return 0, false
	}
	if v == 0 || uint32(v) == zdb.ZergApplicationID {
		return v, true
	}
	return v, false
}

// ── 只读小工具 ────────────────────────────────────────────────────────────────────────────────

// coreDbFileSize —— 一份件的字节数（读不到 ⇒ **空串** —— 机器面缺值整键略去，✗ 不落 `0`）。
func coreDbFileSize(p string) string {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return ""
	}
	return strconv.FormatInt(st.Size(), 10)
}

// coreDbPathExists —— 路径在盘（只读）。
func coreDbPathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// coreDbScalarInt —— 取一条 PRAGMA 的整数标量（读不到 ⇒ 空串）。
func coreDbScalarInt(db *sql.DB, pragma string) string {
	var v int64
	if err := db.QueryRow(pragma).Scan(&v); err != nil {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// coreDbScalarText —— 取一条 PRAGMA 的文本标量（读不到 ⇒ 空串）。
func coreDbScalarText(db *sql.DB, pragma string) string {
	var v string
	if err := db.QueryRow(pragma).Scan(&v); err != nil {
		return ""
	}
	return v
}

// coreDbMaybeList —— 人面把逗号串摊成一行（空 ⇒ 「（无）」）。
func coreDbMaybeList(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（无）"
	}
	return "：" + strings.ReplaceAll(s, ",", " · ")
}

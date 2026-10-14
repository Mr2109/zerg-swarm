package zdb

import (
	"errors"
	"strings"
)

// 哨兵错误（调用方按错误类分档，✗ 不解析文案）。
var (
	// ErrVolume —— 库所在卷不可用 / 不是本地盘（硬规矩 ①）。
	ErrVolume = errors.New("zdb: 库所在卷不可用")
	// ErrIntegrity —— 开库体检（integrity_check）未过（硬规矩 ②）。
	ErrIntegrity = errors.New("zdb: 库完整性体检未过")
	// ErrSchemaTooNew —— 库 user_version 高于本二进制支持上限（§三.7 · 旧二进制碰新库 ⇒ 拒启）。
	ErrSchemaTooNew = errors.New("zdb: 库 schema 版本高于本二进制支持上限")
	// ErrStateConflict —— 改态乐观校验失败（当前态 ≠ 期望的 from）。
	ErrStateConflict = errors.New("zdb: 任务态冲突（当前态与期望的 from 不符）")
	// ErrTerminalImmutable —— **终态不可回活态/改态**（`tasks.state` 判据 · §九.2 配套①；
	// 唯一写侧门 = tasks.go 的 CheckTransition，ApplyStateChange 必过它）。
	ErrTerminalImmutable = errors.New("zdb: 终态不可回活态")
	// ErrIdempotencyConflict —— 同键异参（幂等键已存在且指纹不同 · 片 6 三态③）。
	ErrIdempotencyConflict = errors.New("zdb: 幂等键冲突（同键异参）")
	// ErrSameVolumeBackup —— 备份目标与库同卷（硬规矩 ③）。
	ErrSameVolumeBackup = errors.New("zdb: 备份目标与库同卷（须落另一卷）")
	// ErrNotZergDB —— 库身份标不符（`PRAGMA application_id` 挂着**别人**的非 0 值）
	// · §三.4 附 G③ · §六 Q28 ⇒ 拒启 fail-closed。
	ErrNotZergDB = errors.New("zdb: 库身份标不符（application_id 非本库约定值 ⇒ 不是虫族库）")
)

// dsnEscapePath 把路径里会破坏 file: URI 的字符做最小转义（? # %）。
// 空格与 CJK 落在 URI 的 path 段里是合法的，✗ 不需要转义。
func dsnEscapePath(p string) string {
	r := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")
	return r.Replace(p)
}

// BuildDSN 生成 modernc 的 DSN，逐字照设计稿 §三.3 的规格级配方，**并把 `auto_vacuum(2)` 排在最前**
// （本件的**开库路径接线**：该形态只用于**新建库** —— 见 open.go 的 Open / isFreshDBFile）：
//
//	file:<库路径>?_pragma=auto_vacuum(2)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate
//
// ★★ **`auto_vacuum(2)` 必须排在 `journal_mode(WAL)` 之前**（§四.3 第 3 条 · 本件 machine 证成）：
// 官方逐字「It is not possible to enable or disable auto-vacuum after a table has been created.」
// ⇒ 反序（先 WAL 再设）＝ 回读 **0** ＋ **静默失效**（不报错）⇒ 真还盘整条链白做（见 archive.go）。
// ★ 该 pragma **只在新建库（库文件不存在 / 为空 · 库头尚未落盘）时立生效**；对任何**已有库头**的
// 文件（size>0，哪怕库内零用户表）都是**静默空动作**（回读仍 0 · 本件实测）⇒ 存量库**不因它改行为**。
// 既有库的开库路径因此走 **BuildExistingDSN**（不带这条 pragma · §四.3「既有真库本批不动」）。
// ★ 其余四条 PRAGMA 一律写进 DSN（`_pragma=`），✗ 不用 db.Exec("PRAGMA …")：
// PRAGMA 是 per-connection 的，池新开连接会让 foreign_keys 回 OFF、busy_timeout 回 0。
// ★ `_txlock=immediate`：默认 BEGIN DEFERRED 的「读→写升级」会瞬间 SQLITE_BUSY，IMMEDIATE 在事务开头抢写锁。
func BuildDSN(dbPath string) string {
	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(dsnEscapePath(dbPath))
	b.WriteString("?_pragma=auto_vacuum(2)")
	b.WriteString("&_pragma=journal_mode(WAL)")
	b.WriteString("&_pragma=synchronous(NORMAL)")
	b.WriteString("&_pragma=busy_timeout(5000)")
	b.WriteString("&_pragma=foreign_keys(ON)")
	b.WriteString("&_txlock=immediate")
	return b.String()
}

// BuildExistingDSN 生成**既有库**（库头已在位 · size>0）的写侧 DSN：与 `BuildDSN` **逐字同形**，
// **唯独不带 `_pragma=auto_vacuum(2)`**。
//
//	file:<库路径>?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate
//
// ★ 为何分两条：`auto_vacuum` 只在**新建库首建**时立生效 —— 对任何已有库头的文件执行都是**静默空动作**
// （回读仍 0 · 本件实测）；而 §四.3 拍定「既有真库本批不动」⇒ 既有库的 DSN 干脆不带这条 pragma
// （既不指望它生效、也不让 DSN 面「声称」要改存量库）。
func BuildExistingDSN(dbPath string) string {
	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(dsnEscapePath(dbPath))
	b.WriteString("?_pragma=journal_mode(WAL)")
	b.WriteString("&_pragma=synchronous(NORMAL)")
	b.WriteString("&_pragma=busy_timeout(5000)")
	b.WriteString("&_pragma=foreign_keys(ON)")
	b.WriteString("&_txlock=immediate")
	return b.String()
}

// BuildReaderDSN 生成**只读侧**的 DSN（设计稿 §三.3 条目 1 · §六 Q16）：
//
//	file:<库路径>?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=query_only(ON)
//
// ★ 与写侧 `BuildDSN` 的**两处差**（逐条 · ✗ 别读成漏写）：
//   - **不带 `_txlock=immediate`** —— 只读面不进写事务，IMMEDIATE 抢锁语义对它无意义（§六 Q16 逐字
//     「读 handle …… ✗ 不带 `_txlock`」）。
//   - **加 `_pragma=query_only(ON)`** —— 只读档（取它而不取 `mode=ro` 的理由见 open.go 的 OpenReader）。
//
// ★ 其余四条 PRAGMA 与写池**逐字同**：WAL / synchronous(NORMAL) / busy_timeout(5000) / foreign_keys(ON)
// 一条都不能少（§三.3 条目 2：PRAGMA 是 per-connection 的，池新开连接会让 foreign_keys 回 OFF、
// busy_timeout 回 0 ⇒ 「开了 WAL 仍随机 SQLITE_BUSY」）。
func BuildReaderDSN(dbPath string) string {
	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(dsnEscapePath(dbPath))
	b.WriteString("?_pragma=journal_mode(WAL)")
	b.WriteString("&_pragma=synchronous(NORMAL)")
	b.WriteString("&_pragma=busy_timeout(5000)")
	b.WriteString("&_pragma=foreign_keys(ON)")
	b.WriteString("&_pragma=query_only(ON)")
	return b.String()
}

// BuildSnapshotSourceDSN 生成**备份源库**的句柄 DSN（`core db backup` 的开库路径）：
//
//	file:<库路径>?_pragma=busy_timeout(5000)
//
// ★ 与另外三条 DSN 的分别（逐条 · ✗ 别读成漏写）：本句柄**只**在它上面跑一条 `VACUUM INTO`
//
//	—— 不进应用事务、不建表、不写库头 ⇒ 不带 `_txlock=immediate`（那是写池的抢锁语义）、
//	不设 `journal_mode(WAL)`（库自己的模式优先）、不设 `auto_vacuum`（只在新建库首建时才有意义）、
//	**不设 `query_only(ON)`**（`VACUUM INTO` 在 query_only 连接上一律退
//	`attempt to write a readonly database (8)` —— 本件实测；`mode=ro` 对 WAL 库更是连普通读都开不起来
//	`unable to open database file (14)`）⇒「只读」由**本路径的行为面**保证：不跑迁移 / 不体检写面 /
//	不打身份标 / 不 checkpoint / `VACUUM INTO` 不改源库（判据：源库 `sha256` 前后逐字同 ＋ `user_version` 不动）。
//
// ★ `foreign_keys` 也不设：本路径只跑 `VACUUM INTO`，不解析外键。
func BuildSnapshotSourceDSN(dbPath string) string {
	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(dsnEscapePath(dbPath))
	b.WriteString("?_pragma=busy_timeout(5000)")
	return b.String()
}

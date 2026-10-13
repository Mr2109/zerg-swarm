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
)

// dsnEscapePath 把路径里会破坏 file: URI 的字符做最小转义（? # %）。
// 空格与 CJK 落在 URI 的 path 段里是合法的，✗ 不需要转义。
func dsnEscapePath(p string) string {
	r := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")
	return r.Replace(p)
}

// BuildDSN 生成 modernc 的 DSN，逐字照设计稿 §三.3 的规格级配方：
//
//	file:<库路径>?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate
//
// ★ 四条 PRAGMA 一律写进 DSN（`_pragma=`），✗ 不用 db.Exec("PRAGMA …")：
// PRAGMA 是 per-connection 的，池新开连接会让 foreign_keys 回 OFF、busy_timeout 回 0。
// ★ `_txlock=immediate`：默认 BEGIN DEFERRED 的「读→写升级」会瞬间 SQLITE_BUSY，IMMEDIATE 在事务开头抢写锁。
func BuildDSN(dbPath string) string {
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

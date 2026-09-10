// atomic.go — 写入安全:store 级独立锁文件 + 原子写(temp→fsync→rename→fsync 目录)
//
// 设计稿 §3.2「写入安全」/§9.5:跨进程互斥用独立锁文件(不要 flock 数据文件本身,
// 因为数据文件要被原子 rename 替换);多写者纪律由锁保证。
package memory

import (
	"os"
	"path/filepath"
	"syscall"
)

// lockScope — 取 scope 级排他锁:进程内互斥由 Store.mu 承担,跨进程由此处的 flock 承担。
// 返回释放函数;锁文件常驻不删(删除会与并发持有者产生竞态)。
func lockScope(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// writeFileAtomic — Resilient-Write:同目录临时文件 → 写 → fsync → rename → fsync 目录。
// 读者永远看不到半截文件;失败时清理临时文件,原文件不动。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, tmpPrefix+"*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := fullSync(tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	// 目录 fsync:让 rename 本身落盘(尽力而为,失败不回滚已成功的写入)
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

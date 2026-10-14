package eggstore

import (
	"os"
	"path/filepath"
)

// writeFileAtomic —— 同目录临时件 → 写全 → `fsync` 文件 → 原子 `rename` → `fsync` 父目录。
//
// 口径与先例逐字对齐：`core/internal/api/tasks_persist.go`（同目录临时件 + fsync 文件 +
// rename 覆盖 + fsync 目录）。临时件与终名**必然同目录** ⇒ 必然同卷 ⇒ `rename(2)` 是
// 原子的（✗ 不会退化成 copy）。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // rename 成功后这里是 no-op

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// 父目录 fsync：保证 rename 本身落盘（打不开目录不算失败 —— rename 已成功）。
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// nearestExistingDir —— 从 path 向上找到第一个存在的目录（用于「还没落盘的终名」取证）。
func nearestExistingDir(path string) (string, error) {
	p := filepath.Clean(path)
	for {
		if st, err := os.Stat(p); err == nil {
			if st.IsDir() {
				return p, nil
			}
			return filepath.Dir(p), nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", os.ErrNotExist
		}
		p = parent
	}
}

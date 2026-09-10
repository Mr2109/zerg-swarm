//go:build !darwin

// sync_other.go — 非 macOS 平台:普通 fsync(linux 的 fsync 已是完整落盘语义)。
package memory

import "os"

func fullSync(f *os.File) error { return f.Sync() }

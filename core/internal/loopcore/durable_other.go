//go:build !darwin

// durable_other.go — T5.5：非 macOS 平台的"写就落盘"实现（linux 的 fsync 已是完整落盘语义）。
package loopcore

import "os"

func fullSync(f *os.File) error { return f.Sync() }

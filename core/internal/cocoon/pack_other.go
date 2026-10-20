//go:build windows

// pack_other.go — 硬链计数的占位实现：**此平台不判**（A6b）。
//
// ★ 原话：Windows 上 nlink 不判。恒返回 1（= 永不因硬链拒件）。
//
// 为什么退让：NTFS 有硬链，但 Go 在 Windows 上给的 `FileInfo.Sys()` 不是 `*syscall.Stat_t`
// （是 Win32FileAttributeData / ByHandleFileInformation 那一套），要拿 NumberOfLinks 得
// 走另一条句柄 API；A6b 的范围是 unix 面。⇒ **Windows 上"硬链把树外的东西挂进白名单"
// 这条越界读仍未封**，这是显式声明的退让，不是"已解决"。
//
// 调用方不要在这里做平台判断：判不出就是 1，行为与"没有硬链"一致，这一点已知且有记录。
package cocoon

import "os"

// stNlink — 本平台不判硬链：恒 1。
func stNlink(os.FileInfo) (uint64, error) { return 1, nil }

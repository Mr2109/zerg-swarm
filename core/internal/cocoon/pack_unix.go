//go:build !windows

// pack_unix.go — 硬链计数（st_nlink）的平台读法（A6b）。
//
// ★ 为什么必须分平台：`Stat_t.Nlink` 的**宽度**由平台决定（darwin 是 uint16、linux 是
// uint64、各 BSD 又各不相同），字段在结构体里的位置也不同。放在同一份代码里既编不过
// Windows，也经不起交叉编译。这里统一收成 uint64，调用方只判 `> 1`。
//
// ★ 为什么"判不出"要返回错误而不是当 1：调用方（collectPackItems / openPackSrc）拿不到
// 读数就**拒件**。把"判不了"折成"没有硬链"等于给越界读留后门，比直接失败糟糕得多。
package cocoon

import (
	"fmt"
	"os"
	"syscall"
)

// stNlink — 取一件的硬链数（st_nlink）。fi 必须是 **lstat/stat 明文得到的**那个 FileInfo；
// 符号链接要先判过型（本层不判型，只读数）。
func stNlink(fi os.FileInfo) (uint64, error) {
	if fi == nil {
		return 0, fmt.Errorf("cocoon: 判硬链数: FileInfo 为空")
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, fmt.Errorf("cocoon: 判硬链数: 拿不到 syscall.Stat_t（得到 %T）", fi.Sys())
	}
	return uint64(st.Nlink), nil
}

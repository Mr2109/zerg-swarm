//go:build !linux && !darwin

// rm_other.go — 其它平台（FreeBSD/Windows/…）的删除原语：os.Root。
//
// 没有 openat2、也没有 O_NOFOLLOW_ANY ⇒ 用 Go 1.25 的 os.Root 兜底：它把"只能碰这个根
// 底下的东西"做成硬约束（内部同样 fd 相对、拒绝逃出根），语义上等价于 RESOLVE_BENEATH。
//
// ★ 三处**明确的退让**（必须让调用方看见，不许假装等价）：
//
//	① 拿不到 st_dev/st_ino 的稳定读数 ⇒ RootID 恒零值、"身份绑定"这一层实际不生效
//	  （RemoveReport.Verified 恒 false，调用方能判）；
//	② 没有 NO_XDEV 等价物，也没有可比的 st_dev ⇒ **跨挂载点防护不生效**；
//	③ os.Root 会跟随根**内部**的链接（只保证不逃出根）⇒ 本文件每条都先 Lstat 判型，
//	  是链接就删名字、不下钻，与 linux/darwin 的行为对齐。
//
// 这三条原话都进 degradeNote()，经 RemoveReport.Degraded 与错误消息出去。
package cocoon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
)

// openBase — 以 os.Root 打开茧根父目录（本平台后续一切都在这个 Root 底下按名字进行）。
func openBase(dir string) (node, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return node{}, fmt.Errorf("cocoon: 打开茧根父目录 %q 失败: %w", dir, err)
	}
	return node{r: r}, nil
}

// openRootAt — 开茧根本体。Lstat 判型（链接不算目录）⇒ 拒删。
func openRootAt(parent node, name string) (node, RootID, error) {
	rel := path.Join(parent.rel, name)
	fi, err := parent.r.Lstat(rel)
	if err != nil {
		return node{}, RootID{}, mapErr(name, err)
	}
	if !fi.IsDir() {
		return node{}, RootID{}, fmt.Errorf("%w: %q（符号链接或非目录）", ErrRootNotDir, name)
	}
	// 身份：本平台拿不到跨平台稳定的 dev/ino ⇒ 零值。
	return node{r: parent.r, rel: rel}, RootID{}, nil
}

// openSub — 开子目录；非目录（含链接）→ errNotDir，上层只删名字。
func openSub(dir node, name string) (node, RootID, error) {
	rel := path.Join(dir.rel, name)
	fi, err := dir.r.Lstat(rel)
	if err != nil {
		return node{}, RootID{}, mapErr(name, err)
	}
	if !fi.IsDir() {
		return node{}, RootID{}, fmt.Errorf("%w: %q", errNotDir, name)
	}
	return node{r: dir.r, rel: rel}, RootID{}, nil
}

// mapErr — errno → 语义错误（os.Root 的错已经是 *os.PathError，errors.Is 能穿）。
func mapErr(name string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("cocoon: %q 不存在: %w", name, err)
	default:
		return fmt.Errorf("cocoon: 打开 %q 失败: %w", name, err)
	}
}

// listNames — 只拿名字（判型已在 openSub 用 Lstat 做过）。
func listNames(n node) ([]string, error) {
	dirName := n.rel
	if dirName == "" {
		dirName = "."
	}
	f, err := n.r.Open(dirName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names, nil
}

// removeEntry — 删名字（文件/链接/空目录同一入口，os.Root.Remove 两者都收）。
func removeEntry(n node, name string, _ bool) error {
	return n.r.Remove(path.Join(n.rel, name))
}

// closeNode — 只有持有 Root 的那个 node（rel 为空）负责关；子 node 是同一 Root 的名字视图。
func closeNode(n node) error {
	if n.r == nil || n.rel != "" {
		return nil
	}
	return n.r.Close()
}

// degradeNote — 本平台的退让原话（见文件抬头 ①②③）。
func degradeNote() string {
	return "本平台（非 linux/darwin）用 os.Root 兜底：**放弃跨挂载点防护**、" +
		"**无 st_dev/st_ino 身份绑定**（Verify 恒 false）；链接按 Lstat 判型后只删名字，不跟随"
}

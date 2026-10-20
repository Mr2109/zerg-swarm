// rm_test.go — A5 删树面的三格硬测试（外加两格便宜的 fail-closed 负控）。
//
// ① TestRemoveCocoonTree            正常删整棵树（嵌套子目录 + 文件 + 空目录 + 墓碑）
// ② TestRemoveCocoonSymlinkNoFollow **负控**：树内 link→$HOME，删后 $HOME 仍在、链接本身没了、
//
//	目标内容一个字节没动（另加一个树外哨兵目录，把"目标没被动"落到可数的东西上）
//
// ③ TestRemoveTreeRefusesCrossMount 跨挂载点拒删（拿不到挂载点 / -short ⇒ skip 并写明理由）
//
// 另加：茧根是链接 ⇒ 拒删且目标完好；身份不符 ⇒ 拒删；墓碑写不下去 ⇒ 一个字节都不删。
package cocoon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// a5Tree 造出来的树长这样（数数的地方都按这份清单）：
//
//	dirs : a, a/b, a/b/c, a/b/d, empty
//	files: meta.json, e.txt, a/f.txt, a/b/g.txt, a/b/c/h.txt
const (
	a5Dirs  = 5
	a5Files = 5
)

func a5Tree(t *testing.T, base, id string) string {
	t.Helper()
	root := filepath.Join(base, id)
	for _, d := range []string{"a/b/c", "a/b/d", "empty"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatalf("建目录 %s 失败: %v", d, err)
		}
	}
	for p, body := range map[string]string{
		"meta.json":   `{"id":"` + id + `","phase":"created"}`,
		"e.txt":       "e",
		"a/f.txt":     "f",
		"a/b/g.txt":   "g",
		"a/b/c/h.txt": strings.Repeat("h", 4096),
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(body), 0o644); err != nil {
			t.Fatalf("写 %s 失败: %v", p, err)
		}
	}
	return root
}

func a5MustStatRoot(t *testing.T, base, id string) RootID {
	t.Helper()
	id0, err := StatRoot(base, id)
	if err != nil {
		t.Fatalf("StatRoot(%q/%q) 失败: %v", base, id, err)
	}
	if !id0.Pinned() {
		t.Fatalf("本平台 StatRoot 拿不到身份（%s）——删除面的身份绑定在这台机器上无从验证", id0)
	}
	return id0
}

// ① 正常删整棵树。
func TestRemoveCocoonTree(t *testing.T) {
	base := t.TempDir()
	id := "c1"
	root := a5Tree(t, base, id)
	m := New(id, "egg-a5", "owner-a5")

	want := a5MustStatRoot(t, base, id)
	rep, err := RemoveCocoon(base, m, want)
	if err != nil {
		t.Fatalf("删树失败: %v（report=%+v）", err, rep)
	}

	if _, err := os.Lstat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("茧根还在（err=%v）", err)
	}
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		t.Fatalf("茧根父目录不该被动: err=%v", err)
	}
	if !rep.Verified {
		t.Fatalf("身份绑定没生效（Verified=false）: %+v", rep)
	}
	if rep.Root != want {
		t.Fatalf("报告的根身份 %s ≠ 期望 %s", rep.Root, want)
	}
	if rep.Files != a5Files || rep.Dirs != a5Dirs {
		t.Fatalf("读数不对：files=%d（要 %d）dirs=%d（要 %d）: %+v",
			rep.Files, a5Files, rep.Dirs, a5Dirs, rep)
	}
	// 先留墓碑：目录没了，证据得在。
	tomb, err := ReadTombstone(base, id)
	if err != nil {
		t.Fatalf("墓碑应当留下（删除前先留的那一步）: %v", err)
	}
	if tomb.ID != id || tomb.Phase != PhaseCreated {
		t.Fatalf("墓碑内容不对: id=%q phase=%s", tomb.ID, tomb.Phase)
	}
}

// ② 负控：树内 link→$HOME（外加树外哨兵目录 + 树内相对链接）。
//
// 钉住的行为：删的是**链接本身**，目标是别人的目录，一个字节都不许碰。这条测试在
// "先 lstat 再按路径删"的实现上会以清空 $HOME 的方式失败——正是要拦的那类事故。
func TestRemoveCocoonSymlinkNoFollow(t *testing.T) {
	base := t.TempDir()
	id := "c2"
	root := a5Tree(t, base, id)

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skipf("读不到 $HOME，负控没有落脚点: %v", err)
	}
	homeSnap := a5SnapOf(t, home)

	if err := os.Symlink(home, filepath.Join(root, "link-home")); err != nil {
		t.Fatalf("造 link→$HOME 失败: %v", err)
	}
	// 树外哨兵：一个完整的小目录，删完必须原封不动（$HOME 里不便写测试文件）。
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "sub", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	outSnap := a5SnapOf(t, outside)
	if err := os.Symlink(outside, filepath.Join(root, "link-outside")); err != nil {
		t.Fatalf("造 link→哨兵目录 失败: %v", err)
	}
	// 第三个：树内相对链接（指向更上层），任何"跟链接"的实现都会爬出去。
	if err := os.Symlink("../../../", filepath.Join(root, "a", "rel-link")); err != nil {
		t.Fatalf("造相对链接失败: %v", err)
	}

	m := New(id, "egg-a5", "owner-a5")
	want := a5MustStatRoot(t, base, id)
	rep, err := RemoveCocoon(base, m, want)
	if err != nil {
		t.Fatalf("删树失败: %v（report=%+v）", err, rep)
	}

	// 链接本身跟着树一起没了。
	if _, err := os.Lstat(filepath.Join(root, "link-home")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("树内的链接条目应当随树消失（err=%v）", err)
	}
	// $HOME 还在：身份（dev/ino）、mtime、目录项一个不少。
	homeSnap.mustBeIntact(t, home, "$HOME")
	// 树外哨兵目录：内容一个字节没动。
	outSnap.mustBeIntact(t, outside, "树外哨兵目录")
	for _, p := range []string{"keep.txt", "sub/deep.txt"} {
		b, err := os.ReadFile(filepath.Join(outside, filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("树外文件 %s 被动了: %v", p, err)
		}
		if string(b) != map[string]string{"keep.txt": "keep", "sub/deep.txt": "deep"}[p] {
			t.Fatalf("树外文件 %s 内容被改了: %q", p, b)
		}
	}
	// 三个链接都是按"非目录条目"删的（不会被当成目录下钻）。
	if rep.Files != a5Files+3 || rep.Dirs != a5Dirs {
		t.Fatalf("读数不对：files=%d（要 %d）dirs=%d（要 %d）: %+v",
			rep.Files, a5Files+3, rep.Dirs, a5Dirs, rep)
	}
}

// ③ 跨挂载点拒删。
//
// 真起一个挂载点：macOS 用 hdiutil 挂一个 8MiB 的 dmg；Linux 要 root 挂 tmpfs。
// 拿不到（无权限/无工具/短模式）就 skip，并把理由写出来——**不许**悄悄改成"删个普通目录"充数。
func TestRemoveTreeRefusesCrossMount(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过：跨挂载负控要真起一个挂载点（macOS 需 hdiutil 挂 dmg，Linux 需 root 挂 tmpfs），" +
			"-short 下不建挂载点、也就无从验证 NO_XDEV / st_dev 那道闸")
	}
	base := t.TempDir()
	id := "c3"
	root := a5Tree(t, base, id)

	mp := filepath.Join(root, "a", "mnt")
	if err := os.MkdirAll(mp, 0o755); err != nil {
		t.Fatal(err)
	}
	detach, why := a5MountTinyFS(t, mp)
	if detach == nil {
		t.Skipf("拿不到独立挂载点，跳过跨挂载负控：%s", why)
	}
	defer detach()
	if err := os.WriteFile(filepath.Join(mp, "inner.txt"), []byte("inner"), 0o644); err != nil {
		t.Fatalf("往挂载点里写文件失败: %v", err)
	}

	rootDev, ok1 := a5DevOf(t, root)
	mpDev, ok2 := a5DevOf(t, mp)
	if !ok1 || !ok2 {
		t.Skipf("本平台读不到 st_dev（%s），这条负控没有区分力", runtime.GOOS)
	}
	if rootDev == mpDev {
		t.Skipf("挂载点上来的 st_dev 与茧根相同（%d == %d）⇒ 这条负控在这里没有区分力", mpDev, rootDev)
	}

	want := a5MustStatRoot(t, base, id)
	rep, err := RemoveTree(base, id, want)
	if err == nil {
		t.Fatalf("跨挂载点竟然删成功了: %+v", rep)
	}
	if !errors.Is(err, ErrCrossMount) {
		t.Fatalf("错误应是 ErrCrossMount（fail-closed），实际: %v", err)
	}
	// fail-closed：树停在那儿（没删成），挂载点与里面的文件都还在。
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("fail-closed 被破坏：茧根没了（%v）", err)
	}
	if _, err := os.Stat(mp); err != nil {
		t.Fatalf("挂载点被动了: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(mp, "inner.txt")); err != nil || string(b) != "inner" {
		t.Fatalf("挂载点里的文件被动了: %q err=%v", b, err)
	}
}

// 茧根本身是符号链接 ⇒ 拒删，且目标一个字节不动。
func TestRemoveTreeRefusesSymlinkRoot(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "c4")
	if err := os.Symlink(outside, target); err != nil {
		t.Fatalf("造茧根链接失败: %v", err)
	}

	if _, err := RemoveTree(base, "c4", RootID{}); !errors.Is(err, ErrRootNotDir) {
		t.Fatalf("茧根是链接时应当 ErrRootNotDir，实际: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(b) != "keep" {
		t.Fatalf("链接目标被动了: %q err=%v", b, err)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatalf("链接本身不该被删（拒删 = 什么都不做）: %v", err)
	}
}

// 身份与期望不符 ⇒ 拒删（名字对了、目录换了，一样不许删）。
func TestRemoveTreeRefusesRootIdentityMismatch(t *testing.T) {
	base := t.TempDir()
	id := "c5"
	root := a5Tree(t, base, id)

	want := a5MustStatRoot(t, base, id)
	wrong := RootID{Dev: want.Dev, Ino: want.Ino + 1}
	rep, err := RemoveTree(base, id, wrong)
	if !errors.Is(err, ErrRootIdentity) {
		t.Fatalf("身份不符时应当 ErrRootIdentity，实际: %v（report=%+v）", err, rep)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("身份不符却动了树: %v", err)
	}
	if rep.Verified {
		t.Fatalf("身份不符却标记 Verified: %+v", rep)
	}
}

// 墓碑写不下去 ⇒ 一个字节都不删（顺序铁律：先留墓碑再删）。
func TestRemoveCocoonTombstoneFirst(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 下权限位不生效：无法把 base 弄成写不进去，这条负控失去意义")
	}
	base := t.TempDir()
	id := "c6"
	root := a5Tree(t, base, id)
	if err := os.Chmod(base, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(base, 0o755) }() // t.TempDir 的清理要写权限

	rep, err := RemoveCocoon(base, New(id, "egg-a5", "owner-a5"), RootID{})
	if err == nil {
		t.Fatalf("base 写不进去，留墓碑应当失败（实际成功: %+v）", rep)
	}
	if !errors.Is(err, ErrNeedTombstone) {
		t.Fatalf("错误应是 ErrNeedTombstone，实际: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(root, "a", "b", "c", "h.txt")); err != nil || fi.Size() == 0 {
		t.Fatalf("墓碑没留成却删了东西: err=%v", err)
	}
}

// ---- 测试用的小工具 ----

// a5Snap 是"不许被动"的目录快照：身份（dev/ino，靠 os.SameFile 比）+ mtime + 目录项名集。
type a5Snap struct {
	fi      os.FileInfo
	names   []string
	namesOK bool
}

func a5SnapOf(t *testing.T, p string) a5Snap {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("快照 %q 失败: %v", p, err)
	}
	s := a5Snap{fi: fi}
	ents, err := os.ReadDir(p)
	if err != nil {
		return s // 读不动就只比身份与 mtime（sandbox 下正常）
	}
	s.namesOK = true
	for _, e := range ents {
		s.names = append(s.names, e.Name())
	}
	sort.Strings(s.names)
	return s
}

func (s a5Snap) mustBeIntact(t *testing.T, p, who string) {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("%s 被动了：%q 不在了（%v）", who, p, err)
	}
	if !os.SameFile(s.fi, fi) {
		t.Fatalf("%s 被换掉了：%q 的 dev/ino 变了", who, p)
	}
	if !fi.ModTime().Equal(s.fi.ModTime()) {
		t.Fatalf("%s 的 mtime 变了（%q）：里面被增删过", who, p)
	}
	if !s.namesOK {
		return
	}
	after := a5SnapOf(t, p)
	if !reflect.DeepEqual(s.names, after.names) {
		t.Fatalf("%s 的条目变了：前 %d 项 / 后 %d 项（%q）", who, len(s.names), len(after.names), p)
	}
}

// a5DevOf — 读 st_dev（各 OS 的 FileInfo.Sys() 具体类型不同，走 reflect 取 Dev 字段）。
func a5DevOf(t *testing.T, p string) (uint64, bool) {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat %q 失败: %v", p, err)
	}
	v := reflect.ValueOf(fi.Sys())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, false
	}
	f := v.FieldByName("Dev")
	if !f.IsValid() {
		return 0, false
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		return uint64(f.Int()), true
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return f.Uint(), true
	}
	return 0, false
}

// a5MountTinyFS 在 mp 上挂一个小文件系统（st_dev 必然不同），返回卸载函数。
// 拿不到就返回 (nil, 理由)——由调用方 skip 并把理由写进输出，绝不静默降级。
func a5MountTinyFS(t *testing.T, mp string) (func(), string) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		img := filepath.Join(t.TempDir(), "a5-mnt.dmg")
		var last string
		for _, fsName := range []string{"HFS+", "APFS"} {
			out, err := exec.Command("hdiutil", "create", "-size", "8m", "-fs", fsName,
				"-volname", "A5MNT", "-quiet", img).CombinedOutput()
			if err != nil {
				last = fmt.Sprintf("hdiutil create -fs %s 失败: %v: %s", fsName, err, strings.TrimSpace(string(out)))
				continue
			}
			out, err = exec.Command("hdiutil", "attach", "-nobrowse", "-noverify", "-noautoopen",
				"-mountpoint", mp, img).CombinedOutput()
			if err != nil {
				last = fmt.Sprintf("hdiutil attach -fs %s 失败: %v: %s", fsName, err, strings.TrimSpace(string(out)))
				continue
			}
			return func() {
				_ = exec.Command("hdiutil", "detach", "-force", mp).Run()
			}, ""
		}
		return nil, last
	case "linux":
		if out, err := exec.Command("mount", "-t", "tmpfs", "-o", "size=1m", "tmpfs", mp).CombinedOutput(); err != nil {
			return nil, fmt.Sprintf("mount -t tmpfs 失败（多半没 root）: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return func() { _ = exec.Command("umount", mp).Run() }, ""
	default:
		return nil, fmt.Sprintf("本平台（%s）没有便宜的挂载手段", runtime.GOOS)
	}
}

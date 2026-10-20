// pack_test.go — A6 打包面的五格硬测试（外加四格便宜的 fail-closed 护栏）。
//
//	① TestPackAndVerify            正常打包 + VerifyPack 通过：白名单收件（workspace + 声明产物）、
//	                                不收 meta.json / cocoon.toml / 白名单外的件；manifest 第 0 行是头；
//	                                逐件 sha256 与独立算的 sha256 相等；tar 里 uid/gid/uname 已清空
//	② TestPackRefusesGitConfig     **负控**：树里有 `.git/config` ⇒ 拒，点名该路径，且不留包/不留 tmp
//	③ TestPackRefusesSymlink       **负控**：树里有符号链接 ⇒ 拒且点名；链接目标一个字节没动
//	④ TestPackRefusesSetuid        **负控**：SUID 件 ⇒ 拒且点名
//	⑤ TestVerifyDetectsTamper      篡改包里一个字节 ⇒ VerifyPack 必红（点名该件）
//	⑥ TestPackRefusesHardlink      **负控（A6b）**：树里有 `nlink > 1` 的普通文件（硬链，无链接位）
//	                                ⇒ 拒且点名，且内容一字不出（不进包、不进错误信息）
//	⑦ TestPackAcceptsSingleLink    **正控（A6b）**：`nlink == 1` 的正常件照收 —— 防"硬链守卫误杀"
//
// 另加护栏：缺 manifest ⇒ 拒；manifest 里写 `..` 路径 ⇒ 拒；白名单写 `..`/`.` ⇒ 拒；空包 ⇒ 拒；
// 输出包落在源树内 ⇒ 拒；源根是符号链接 ⇒ 拒。
package cocoon

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// a6FixedTime — 注入时钟：manifest 的 created_at 必须可复现，测试才有"读数"可言。
var a6FixedTime = time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)

// a6Tree 造一棵**最小但带全各类干扰件**的茧根：
//
//	workspace/bin/run.sh      0755  ← 收
//	workspace/data/a.txt      0644  ← 收（内含唯一 marker，⑤ 用来定位篡改点）
//	workspace/data/b.txt      0644  ← 收
//	workspace/empty/          目录  ← 不入包（空目录不是件）
//	artifacts/out.bin         0644  ← 收（靠 PackOptions.Artifacts 声明）
//	notes.md                  0644  ← 白名单外，不收
//	meta.json                 0644  ← 宿主态，不收
//	cocoon.toml               0644  ← 声明文件本身，不收
//
// 返回 root 与三个"应入包"的相对路径（升序）与它们的源内容。
const a6Marker = "MARKER-6f1c9a02-do-not-move"

func a6Tree(t *testing.T) (root string, want []string, bodies map[string][]byte) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "cocoon")
	bodies = map[string][]byte{
		"workspace/bin/run.sh": []byte("#!/bin/sh\necho run\n"),
		"workspace/data/a.txt": []byte("alpha\n" + a6Marker + "\nomega\n"),
		"workspace/data/b.txt": []byte("bravo\n"),
		"artifacts/out.bin":    {0x00, 0x01, 0x02, 0xfe, 0xff, 0x0a},
		"notes.md":             []byte("白名单外：不该进包\n"),
		MetaFileName:           []byte(`{"id":"c-1","phase":"created"}` + "\n"),
		"cocoon.toml":          []byte("[cocoon]\nid = \"c-1\"\n"),
	}
	for rel, b := range bodies {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if rel == "workspace/bin/run.sh" {
			mode = 0o755
		}
		if err := os.WriteFile(p, b, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil { // umask 可能吃掉位，显式再 chmod 一次
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "workspace", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	want = []string{"artifacts/out.bin", "workspace/bin/run.sh", "workspace/data/a.txt", "workspace/data/b.txt"}
	sort.Strings(want)
	return root, want, bodies
}

func a6Opts() PackOptions {
	return PackOptions{
		Artifacts: []string{"artifacts"},
		Now:       func() time.Time { return a6FixedTime },
	}
}

// ① 正常打包 + 逐件校验通过。
func TestPackAndVerify(t *testing.T) {
	root, want, bodies := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")

	m, err := Pack(root, out, a6Opts())
	if err != nil {
		t.Fatalf("Pack 失败: %v", err)
	}

	// — 件集合：只收白名单命中的普通文件，且按 path 升序 —
	var got []string
	for _, e := range m.Entries {
		got = append(got, e.Path)
	}
	if !equalStrings(got, want) {
		t.Fatalf("入包件集不符:\n got=%v\nwant=%v", got, want)
	}
	for _, banned := range []string{"notes.md", MetaFileName, "cocoon.toml", "workspace/empty"} {
		for _, p := range got {
			if p == banned {
				t.Fatalf("白名单外的件进了包: %s", banned)
			}
		}
	}

	// — manifest 第 0 行必须是头（format/version/created_at）—
	raw, err := os.ReadFile(ManifestPath(out))
	if err != nil {
		t.Fatalf("读 manifest: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1+len(want) {
		t.Fatalf("manifest 行数=%d（要 1+%d）:\n%s", len(lines), len(want), raw)
	}
	var hdr PackHeader
	if err := json.Unmarshal([]byte(lines[0]), &hdr); err != nil {
		t.Fatalf("第 0 行不是 JSON 头: %v", err)
	}
	if hdr.Format != PackFormat || hdr.Version != PackFormatVersion {
		t.Fatalf("头不规范: format=%q version=%d", hdr.Format, hdr.Version)
	}
	if !hdr.CreatedAt.Equal(a6FixedTime) {
		t.Fatalf("created_at=%s（要注入的 %s）", hdr.CreatedAt, a6FixedTime)
	}
	if hdr.Entries != len(want) {
		t.Fatalf("头记 %d 件（要 %d）", hdr.Entries, len(want))
	}
	if st, err := os.Stat(out); err != nil || st.Size() != hdr.PackSize {
		t.Fatalf("pack_size=%d 与实际包大小不符（err=%v）", hdr.PackSize, err)
	}

	// — 逐件 sha256 与独立算的一致（不信打包器自己报的数）—
	for _, e := range m.Entries {
		sum := sha256.Sum256(bodies[e.Path])
		if e.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s sha=%s（独立算的是 %s）", e.Path, e.SHA256, hex.EncodeToString(sum[:]))
		}
		if e.Size != int64(len(bodies[e.Path])) {
			t.Fatalf("%s size=%d（要 %d）", e.Path, e.Size, len(bodies[e.Path]))
		}
	}

	// — tar 里：只放 TypeReg、uid/gid 归零、uname/gname 清空（不把宿主身份带进制品）—
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	var tarNames []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("读 tar: %v", err)
		}
		tarNames = append(tarNames, h.Name)
		if h.Typeflag != tar.TypeReg {
			t.Fatalf("%s typeflag=%q（只许普通文件）", h.Name, h.Typeflag)
		}
		if h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" {
			t.Fatalf("%s 带宿主身份 uid=%d gid=%d uname=%q gname=%q", h.Name, h.Uid, h.Gid, h.Uname, h.Gname)
		}
	}
	if !equalStrings(tarNames, want) {
		t.Fatalf("tar 件集=%v（要 %v）", tarNames, want)
	}

	// — 校验本体 —
	if err := VerifyPack(out); err != nil {
		t.Fatalf("VerifyPack 该过却红了: %v", err)
	}
	// 包里不该留半成品
	for _, p := range []string{out + ".tmp", ManifestPath(out) + ".tmp"} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("残留半成品: %s", p)
		}
	}
}

// ② 负控：`.git/config` ⇒ 拒，点名，且不留包。
func TestPackRefusesGitConfig(t *testing.T) {
	root, _, _ := a6Tree(t)
	cfgDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const token = "ghp_SUPERSECRETTOKEN"
	cfg := "[remote \"origin\"]\n\turl = https://x-access-token:" + token + "@github.com/o/r.git\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "c-1.tar")
	_, err := Pack(root, out, a6Opts())
	if err == nil {
		t.Fatal(".git/config 在树里，Pack 却成功了")
	}
	if !errors.Is(err, ErrPackForbidden) {
		t.Fatalf("错误类型不对（要 ErrPackForbidden）: %v", err)
	}
	if !strings.Contains(err.Error(), ".git/config") {
		t.Fatalf("错误没点名路径: %v", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("错误信息里带出了凭据: %v", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("被拒却留下了包: %s", out)
	}
	if _, statErr := os.Stat(out + ".tmp"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("被拒却留下了 tmp")
	}

	// 嵌在子目录里的 .git 一样拒（不是只认根上那一处）
	if err := os.RemoveAll(cfgDir); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "workspace", "vendor", ".git")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Pack(root, out, a6Opts())
	if !errors.Is(err, ErrPackForbidden) || !strings.Contains(err.Error(), "workspace/vendor/.git/config") {
		t.Fatalf("嵌套 .git/config 未被点名拒: %v", err)
	}
}

// ③ 负控：符号链接 ⇒ 拒且点名；目标一个字节没动。
func TestPackRefusesSymlink(t *testing.T) {
	root, _, _ := a6Tree(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	const secret = "ID-THIS-MUST-NOT-ENTER-THE-PACK\n"
	if err := os.WriteFile(outside, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "workspace", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "c-1.tar")
	_, err := Pack(root, out, a6Opts())
	if err == nil {
		t.Fatal("树里有权符号链接，Pack 却成功了")
	}
	if !errors.Is(err, ErrPackSymlink) {
		t.Fatalf("错误类型不对（要 ErrPackSymlink）: %v", err)
	}
	if !strings.Contains(err.Error(), "workspace/escape") {
		t.Fatalf("错误没点名路径: %v", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("被拒却留下了包")
	}

	// 链接本身还在（拒 = 什么都不做），目标内容一个字节没动
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("链接被动了（err=%v）", err)
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != secret {
		t.Fatalf("链接目标被动了: %q err=%v", got, err)
	}

	// 指向**目录**的链接同样拒（不是只挡指向文件的）
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "workspace", "dirlink")); err != nil {
		t.Fatal(err)
	}
	_, err = Pack(root, out, a6Opts())
	if !errors.Is(err, ErrPackSymlink) || !strings.Contains(err.Error(), "workspace/dirlink") {
		t.Fatalf("目录链接未被点名拒: %v", err)
	}
}

// ④ 负控：SUID 件 ⇒ 拒且点名。
func TestPackRefusesSetuid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 无 SUID 语义")
	}
	root, _, _ := a6Tree(t)
	suid := filepath.Join(root, "workspace", "bin", "suid-tool")
	if err := os.WriteFile(suid, []byte("#!/bin/sh\nid\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(suid, 0o755|os.ModeSetuid); err != nil { // 注意：0o4755 在 Go 里**不是** ModeSetuid
		t.Fatal(err)
	}
	fi, err := os.Lstat(suid)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSetuid == 0 {
		t.Skipf("本机文件系统不保留 SUID 位（mode=%v），此格不成立", fi.Mode())
	}

	out := filepath.Join(t.TempDir(), "c-1.tar")
	_, err = Pack(root, out, a6Opts())
	if err == nil {
		t.Fatal("树里有 SUID 件，Pack 却成功了")
	}
	if !errors.Is(err, ErrPackSetuid) {
		t.Fatalf("错误类型不对（要 ErrPackSetuid）: %v", err)
	}
	if !strings.Contains(err.Error(), "workspace/bin/suid-tool") {
		t.Fatalf("错误没点名路径: %v", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("被拒却留下了包")
	}
}

// ⑤ 篡改包里一个字节 ⇒ VerifyPack 必红，且点名该件。
func TestVerifyDetectsTamper(t *testing.T) {
	root, _, _ := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")
	if _, err := Pack(root, out, a6Opts()); err != nil {
		t.Fatalf("Pack 失败: %v", err)
	}
	if err := VerifyPack(out); err != nil {
		t.Fatalf("篡改前该过却红了: %v", err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// 只在**载荷**里翻一位：marker 只出现在 workspace/data/a.txt 的内容里，
	// 不会命中 tar 头里的路径串（a.txt 的内容有别于它的名字）。
	at := bytes.Index(raw, []byte(a6Marker))
	if at < 0 {
		t.Fatal("包里找不到 marker（测试自身的定位失败）")
	}
	raw[at+len(a6Marker)-1] ^= 0xff
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	err = VerifyPack(out)
	if err == nil {
		t.Fatal("篡改了一个字节，VerifyPack 却过了")
	}
	if !errors.Is(err, ErrPackMismatch) {
		t.Fatalf("错误类型不对（要 ErrPackMismatch）: %v", err)
	}
	if !strings.Contains(err.Error(), "workspace/data/a.txt") {
		t.Fatalf("错误没点名被篡改的件: %v", err)
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("错误没说明是 sha 不符: %v", err)
	}
}

// 护栏 A：缺侧车 manifest ⇒ 拒（包与 manifest 必须成对，不许"只有包"就算过）。
func TestVerifyNeedsManifest(t *testing.T) {
	root, _, _ := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")
	if _, err := Pack(root, out, a6Opts()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ManifestPath(out)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPack(out); !errors.Is(err, ErrManifestMissing) {
		t.Fatalf("缺 manifest 时错误类型不对: %v", err)
	}
}

// 护栏 B：manifest 里写 `..` 路径 ⇒ 拒（读端也要防穿越，不能只靠打包端）。
func TestVerifyRejectsTraversalEntry(t *testing.T) {
	root, _, _ := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")
	if _, err := Pack(root, out, a6Opts()); err != nil {
		t.Fatal(err)
	}
	mp := ManifestPath(out)
	b, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	lines[1] = `{"path":"../escape.txt","size":1,"mode":"0644","sha256":"00"}`
	if err := os.WriteFile(mp, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPack(out); !errors.Is(err, ErrPackUnsafePath) {
		t.Fatalf("manifest 里的 .. 路径未被拒: %v", err)
	}
}

// 护栏 C：白名单写法不安全（`..` / 整根 / 绝对路径）⇒ 拒；一件未命中 ⇒ 拒。
func TestPackRejectsBadWhitelist(t *testing.T) {
	root, _, _ := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")
	for _, bad := range []string{"..", "../etc", "/etc", "."} {
		_, err := Pack(root, out, PackOptions{Allow: []string{bad}})
		if !errors.Is(err, ErrPackUnsafePath) {
			t.Fatalf("白名单 %q 未被拒（err=%v）", bad, err)
		}
	}
	_, err := Pack(root, out, PackOptions{Allow: []string{"nothing-here"}})
	if !errors.Is(err, ErrPackEmpty) {
		t.Fatalf("空包未被拒: %v", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("被拒却留下了包")
	}
}

// 护栏 D：包落在源树内 ⇒ 拒；源根是符号链接 ⇒ 拒。
func TestPackRejectsSelfAndLinkRoot(t *testing.T) {
	root, _, _ := a6Tree(t)
	if _, err := Pack(root, filepath.Join(root, "workspace", "self.tar"), a6Opts()); !errors.Is(err, ErrPackOutputInside) {
		t.Fatalf("包落在源树内未被拒: %v", err)
	}
	linkRoot := filepath.Join(t.TempDir(), "link-root")
	if err := os.Symlink(root, linkRoot); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "c-1.tar")
	if _, err := Pack(linkRoot, out, a6Opts()); !errors.Is(err, ErrPackRoot) {
		t.Fatalf("源根是符号链接未被拒: %v", err)
	}
}

// 护栏 E：包里夹带符号链接 ⇒ 拒（解包侧不许有"跟随"入口；这是 VerifyPack 自己的判据）。
func TestVerifyRejectsSymlinkMember(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "evil.tar")
	{
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		tw := tar.NewWriter(f)
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeSymlink, Name: "workspace/link", Linkname: "/etc/passwd", Mode: 0o777,
		}); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	hdr := PackHeader{Format: PackFormat, Version: PackFormatVersion, CreatedAt: a6FixedTime, Entries: 1, PackSize: st.Size()}
	line, _ := json.Marshal(PackEntry{Path: "workspace/link", Size: 0, Mode: "0777", SHA256: strings.Repeat("0", 64)})
	// 手写"头 + 一条符号链接件"的 manifest：模拟一份**自洽**的伪造包（sha 都对不上也无所谓，
	// 因为类型判据在 sha 之前——先拒"不是普通文件"，不先比内容）。
	body, _ := json.Marshal(hdr)
	if err := os.WriteFile(ManifestPath(out), append(append(body, '\n'), append(line, '\n')...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPack(out); !errors.Is(err, ErrPackSpecial) {
		t.Fatalf("包里夹带符号链接未被拒: %v", err)
	}
}

// a6Lstat — 测试侧取 FileInfo（失败即 Fatal），供 stNlink 判 nlink。
func a6Lstat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("lstat %s: %v", p, err)
	}
	return fi
}

// ⑥ 负控（A6b）：`nlink > 1` 的普通文件 ⇒ 拒且点名，内容一字不出。
//
// 这格打的是 A6 自报的漏洞：硬链**没有链接位**，`ln ~/.ssh/id_rsa workspace/x` 造出来的 x
// 在 lstat 眼里就是个普通文件，符号链接那一套判据全过；只有 nlink 认得出来。
func TestPackRefusesHardlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 面 nlink 不判（pack_other.go 恒 1），本格在此平台不成立")
	}
	root, _, _ := a6Tree(t)

	// 树外的"蜜钥"：内容必须既不进包、也不进错误信息。
	outside := filepath.Join(t.TempDir(), "id_ed25519")
	const secret = "PRIVATE-KEY-MUST-NOT-ENTER-THE-PACK\n"
	if err := os.WriteFile(outside, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	// 挂在白名单**内**的名字：最坏情况，白名单本来就要收它。
	link := filepath.Join(root, "workspace", "innocent.txt")
	if err := os.Link(outside, link); err != nil {
		t.Skipf("本机/本文件系统造不出硬链（%v，可能 EXDEV/EPERM/不支持）：此格不成立", err)
	}
	defer os.Remove(link)

	nl, err := stNlink(a6Lstat(t, link))
	if err != nil {
		t.Skipf("造出了硬链但读不到 nlink（%v）：此格不成立", err)
	}
	if nl < 2 {
		t.Skipf("硬链造出来了但 nlink=%d（要 >1），拿不到样本：此格不成立", nl)
	}
	// 前置事实：它是**普通文件**，不是符号链接（否则这格就退化成 ③ 了，不再证明什么）。
	if fi := a6Lstat(t, link); fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
		t.Fatalf("硬链件的 mode=%v（要普通文件）", fi.Mode())
	}

	out := filepath.Join(t.TempDir(), "c-1.tar")
	_, err = Pack(root, out, a6Opts())
	if err == nil {
		t.Fatal("树里有 nlink>1 的硬链件，Pack 却成功了（内容被复制进包）")
	}
	if !errors.Is(err, ErrPackHardlink) {
		t.Fatalf("错误类型不对（要 ErrPackHardlink）: %v", err)
	}
	if !strings.Contains(err.Error(), "workspace/innocent.txt") {
		t.Fatalf("错误没点名路径: %v", err)
	}
	if strings.Contains(err.Error(), "PRIVATE-KEY") {
		t.Fatalf("错误信息里带出了件内容: %v", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("被拒却留下了包: %s", out)
	}
	if _, statErr := os.Stat(out + ".tmp"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("被拒却留下了 tmp")
	}
	// 拒 = 什么都不做：蜜钥与它的硬链都还在，内容一个字节没动。
	if got, err := os.ReadFile(outside); err != nil || string(got) != secret {
		t.Fatalf("硬链目标被动了: %q err=%v", got, err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("硬链件被动了（err=%v）", err)
	}

	// 白名单**外**的硬链一样拒：拒件判定在全树上做，与白名单无关（同 ③ 的语义）。
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	outsideLink := filepath.Join(root, "notes-hard.md")
	if err := os.Link(outside, outsideLink); err != nil {
		t.Skipf("第二处硬链造不出来（%v）：白名单外那一支不验", err)
	}
	defer os.Remove(outsideLink)
	_, err = Pack(root, out, a6Opts())
	if !errors.Is(err, ErrPackHardlink) || !strings.Contains(err.Error(), "notes-hard.md") {
		t.Fatalf("白名单外的硬链未被点名拒: %v", err)
	}
}

// ⑦ 正控（A6b）：`nlink == 1` 的正常件照常打包 —— 防"守卫写成一律拒"的误杀（fail-closed 过头）。
//
// 做法是**同一棵树只改 nlink**：先挂上第二个名字（该拒），再摘掉（该收）。除了 nlink 之外
// 没有任何变量变化，所以这一对读数直接证明守卫判的是 nlink，而不是"这棵树有点怪"。
func TestPackAcceptsSingleLink(t *testing.T) {
	root, want, bodies := a6Tree(t)
	out := filepath.Join(t.TempDir(), "c-1.tar")

	// 阶段 1：给 a.txt 挂第二个名字 ⇒ nlink=2 ⇒ 必须拒（若本机造得出硬链）。
	dup := filepath.Join(root, "workspace", "data", "a-dup.txt")
	canHardlink := false
	if err := os.Link(filepath.Join(root, "workspace", "data", "a.txt"), dup); err != nil {
		t.Logf("本机/本文件系统造不出硬链（%v）：跳过阶段 1-2，只验阶段 3 的正控", err)
	} else {
		nl, nlErr := stNlink(a6Lstat(t, dup))
		if nlErr == nil && nl > 1 {
			canHardlink = true
			if _, err := Pack(root, out, a6Opts()); !errors.Is(err, ErrPackHardlink) {
				t.Fatalf("nlink=%d 时该拒却没收住（err=%v）", nl, err)
			}
		} else {
			t.Logf("nlink=%d err=%v：本平台不判硬链，阶段 1-2 跳过", nl, nlErr)
		}
		// 阶段 2：摘掉第二个名字（nlink 回到 1）。
		if err := os.Remove(dup); err != nil {
			t.Fatal(err)
		}
	}

	// 阶段 3：nlink 全部回到 1 ⇒ 该收，且件集与 ① 完全一致（不多不少）。
	if canHardlink {
		if nl, err := stNlink(a6Lstat(t, filepath.Join(root, "workspace", "data", "a.txt"))); err != nil || nl != 1 {
			t.Fatalf("摘掉第二名字后 nlink=%d（要 1）err=%v", nl, err)
		}
	}
	m, err := Pack(root, out, a6Opts())
	if err != nil {
		t.Fatalf("nlink==1 的正常树被误杀: %v", err)
	}
	var got []string
	for _, e := range m.Entries {
		got = append(got, e.Path)
	}
	if !equalStrings(got, want) {
		t.Fatalf("入包件集不符:\n got=%v\nwant=%v", got, want)
	}
	// 每一个入包件的源，nlink 都必须显式读到 1（Windows 面恒 1，见 pack_other.go）。
	for _, rel := range want {
		nl, err := stNlink(a6Lstat(t, filepath.Join(root, filepath.FromSlash(rel))))
		if err != nil {
			t.Fatalf("%s 读不到 nlink: %v", rel, err)
		}
		if nl != 1 {
			t.Fatalf("%s nlink=%d（入包件必须是 nlink==1 的单名件）", rel, nl)
		}
	}
	if err := VerifyPack(out); err != nil {
		t.Fatalf("正控包自验该过却红了: %v", err)
	}
	for _, e := range m.Entries {
		sum := sha256.Sum256(bodies[e.Path])
		if e.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s sha 与源内容不符", e.Path)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

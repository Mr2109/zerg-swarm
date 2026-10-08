// isolate_ro_test.go —— 件 core/cmd/zerg/isolate_ro.go（scripts/dev/isolate-ro.sh 的 Go 真搬）的回归牙：
//
//	① 入口三档（usage / rc / 位参解析）
//	② 逐条成对控（--self-test 全格）＋ 关键判定面（危险链接 / --dst 包含性 / 用法错）
//	③ ★ 两态三面逐字同（sh 直跑 ⟷ Go 入口 · rc/stdout/stderr 三面）。
//
// 夹具一律落**仓外**（t.TempDir() ⇒ $TMPDIR · ✗ 不落仓内：门76 会把盘上任何新件算进分母）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ── 入口三档 ─────────────────────────────────────────────────────────────────

func isoT_runEntry(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var ob, eb bytes.Buffer
	rc := isolateRoGateMain(&ob, &eb, argv)
	return rc, ob.String(), eb.String()
}

func TestIsolateRoHelpAndUsage(t *testing.T) {
	rc, out, _ := isoT_runEntry(t, "--help")
	if rc != 0 {
		t.Fatalf("--help：rc=%d（要 0）", rc)
	}
	if !strings.Contains(out, "三个子命令") || !strings.Contains(out, "isolate-ro.sh") {
		t.Fatalf("--help 未印出原件 usage 正文：\n%s", out)
	}
	// 无子命令 ⇒ 印同一 usage 且 rc=2（照原件 case ""）。
	rc2, out2, _ := isoT_runEntry(t)
	if rc2 != 2 || out2 != out {
		t.Fatalf("无参：rc=%d（要 2）· usage 与 --help 是否逐字同=%v", rc2, out2 == out)
	}
}

func TestIsolateRoUnknownFlagAndSubcommand(t *testing.T) {
	if rc, _, errb := isoT_runEntry(t, "--bogus"); rc != 2 || !strings.Contains(errb, "未知旗标 --bogus") {
		t.Fatalf("未知旗标：rc=%d err=%q", rc, errb)
	}
	if rc, _, errb := isoT_runEntry(t, "bogus"); rc != 2 || !strings.Contains(errb, "多余的位置参数") {
		t.Fatalf("未知子命令：rc=%d err=%q", rc, errb)
	}
	if rc, _, errb := isoT_runEntry(t, "new"); rc != 2 || !strings.Contains(errb, "new 缺 <真仓>") {
		t.Fatalf("new 缺参：rc=%d err=%q", rc, errb)
	}
	if rc, _, _ := isoT_runEntry(t, "verify", "/tmp"); rc != 2 {
		t.Fatalf("verify 缺一实参：rc=%d（要 2）", rc)
	}
}

// ── 自检全格（--self-test）────────────────────────────────────────────────────

func TestIsolateRoSelfTestAllGreen(t *testing.T) {
	rc, out, errb := isoT_runEntry(t, "--self-test")
	if rc != 0 {
		t.Fatalf("--self-test：rc=%d（要 0）\n%s%s", rc, out, errb)
	}
	if !strings.Contains(out, "自检结论：全过") {
		t.Fatalf("自检结论非全过：\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Fatalf("自检有红格：\n%s", out)
	}
	// 抽样钉住几条关键断言真的在（✗ 不削断言）。
	for _, need := range []string{
		"正控：verify ⇒ rc=0", "负控 A：真仓被改 ⇒ verify rc=1",
		"负控 B'：--reject-symlinks ⇒ rc=1 且点出链接",
		"--only 单件：副本里只有那件 + 清单", "--list：零写盘",
		"包含性：--dst 落在真仓内 ⇒ rc=2",
	} {
		if !strings.Contains(out, need) {
			t.Fatalf("自检缺少该格：%s\n%s", need, out)
		}
	}
}

// ── 关键判定面（有牙：会真判危险形态与包含性）──────────────────────────────────

func isoT_seed(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{
		root + "/real/core/cmd/zerg/testdata", root + "/real/scripts/dev",
		root + "/real/bin", root + "/real/ui/target", root + "/real/sub",
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	isoT_w(t, root+"/real/core/cmd/zerg/testdata/cli-matrix.json", "matrix\n")
	isoT_w(t, root+"/real/scripts/dev/keep.sh", "keep\n")
	isoT_w(t, root+"/real/bin/big.bin", "heavy\n")
	isoT_w(t, root+"/real/ui/target/artifact.bin", "big\n")
	isoT_w(t, root+"/real/sub/aaa.txt", "aaa\n")
	isoT_w(t, root+"/real/top.txt", "top\n")
	// 一条指向副本之外的符号链接（隔离时须被换成硬拷贝 / 严格档被拒）。
	if err := os.Symlink(root+"/real/core/cmd/zerg/testdata/cli-matrix.json", root+"/real/sub/link_out.txt"); err != nil {
		t.Fatal(err)
	}
}

func isoT_w(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsolateRoDangerousLinkAndDstInside(t *testing.T) {
	root := t.TempDir()
	isoT_seed(t, root)

	// 缺省档：外指链接被换成硬拷贝 ⇒ rc=0 且副本里 0 条链接。
	rc, out, errb := isoT_runEntry(t, "new", root+"/real", "--dst", root+"/tree")
	if rc != 0 {
		t.Fatalf("new 缺省档：rc=%d\n%s%s", rc, out, errb)
	}
	if n := len(isoFindSymlinkRels(root + "/tree")); n != 0 {
		t.Fatalf("副本里仍有 %d 条链接（外指链接应被换成硬拷贝）", n)
	}
	if !isoIsFile(root + "/tree/sub/link_out.txt") {
		t.Fatalf("外指链接未被换成真件")
	}

	// 严格档：外指链接 ⇒ rc=1 且点名。
	rc, _, errb = isoT_runEntry(t, "new", root+"/real", "--dst", root+"/tree2", "--reject-symlinks")
	if rc != 1 || !strings.Contains(errb, "link_out.txt") {
		t.Fatalf("--reject-symlinks：rc=%d err=%q", rc, errb)
	}

	// --dst 落在真仓内 ⇒ rc=2，且真仓里没被建出 inside/。
	rc, _, errb = isoT_runEntry(t, "new", root+"/real", "--dst", root+"/real/inside")
	if rc != 2 || !strings.Contains(errb, "真仓内") {
		t.Fatalf("--dst 真仓内：rc=%d err=%q", rc, errb)
	}
	if isoExists(root + "/real/inside") {
		t.Fatalf("拒写后真仓里仍被建出 inside/（有副作用）")
	}
}

// ── 两态三面逐字同（sh 直跑 ⟷ Go 入口）─────────────────────────────────────────

var (
	isoT_reSelfTmp = regexp.MustCompile(`isolate-ro-selftest\.[A-Za-z0-9]{6}`)
	isoT_reMktmp   = regexp.MustCompile(`isolate-ro\.[A-Za-z0-9]{6}`)
)

func isoT_norm(s string, roots ...string) string {
	for _, r := range roots {
		if r == "" {
			continue
		}
		s = strings.ReplaceAll(s, r, "<FX>")
	}
	s = isoT_reSelfTmp.ReplaceAllString(s, "isolate-ro-selftest.<TMP>")
	s = isoT_reMktmp.ReplaceAllString(s, "isolate-ro.<TMP>")
	return s
}

func isoT_repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("从 %s 上溯找不到仓根", wd)
		}
		dir = parent
	}
}

func isoT_runSh(t *testing.T, script, dir string, argv []string) (int, string, string) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, argv...)...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("起 bash 失败：%v", err)
	}
	return rc, so.String(), se.String()
}

// isoT_copyTree 递归复制（供 verify 格造「副本」）。
func isoT_copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if fi, lerr := os.Lstat(s); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
			tgt, rerr := os.Readlink(s)
			if rerr != nil {
				t.Fatal(rerr)
			}
			if serr := os.Symlink(tgt, d); serr != nil {
				t.Fatal(serr)
			}
			continue
		}
		if e.IsDir() {
			isoT_copyTree(t, s, d)
			continue
		}
		b, rerr := os.ReadFile(s)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if werr := os.WriteFile(d, b, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
}

type isoTScen struct {
	name string
	seed func(t *testing.T, root string)
	argv func(root string) []string
}

func TestIsolateRoTwoStateParity(t *testing.T) {
	root := isoT_repoRoot(t)
	script := filepath.Join(root, "scripts", "dev", "isolate-ro.sh")
	if _, err := os.Stat(script); err != nil {
		// ★ 三段式回退腿：sh 原件缺席而其 Go 半在盘 ⇒ 退役态 —— 两态对拍没有对拍对象 ⇒ 体面跳过
		//   （Go 半自身的断言由本件其余用例照常覆盖）。两半皆缺（真消失）⇒ fail-closed。
		if isoIsFile(filepath.Join(root, "core/cmd/zerg/isolate_ro.go")) {
			t.Skipf("退役态：sh 原件 scripts/dev/isolate-ro.sh 已退场（其 Go 半在盘）⇒ 两态对拍格无对象，容忍跳过")
		}
		t.Fatalf("原件不在盘：%s", script)
	}

	verifyTree := func(t *testing.T, root string) {
		isoT_seed(t, root)
		isoT_copyTree(t, root+"/real", root+"/tree")
	}

	scens := []isoTScen{
		{name: "help", argv: func(d string) []string { return []string{"--help"} }},
		{name: "noargs", argv: func(d string) []string { return nil }},
		{name: "bogus_flag", argv: func(d string) []string { return []string{"--bogus"} }},
		{name: "bogus_pos", argv: func(d string) []string { return []string{"bogus"} }},
		{name: "new_missing_arg", argv: func(d string) []string { return []string{"new"} }},
		{name: "new_absent_repo", argv: func(d string) []string { return []string{"new", d + "/no-such"} }},
		{name: "verify_missing_arg", argv: func(d string) []string { return []string{"verify", d + "/tree"} }},
		{name: "extra_pos", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "extra"}
		}},
		{name: "new_basic", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree"}
		}},
		{name: "new_no_manifest", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--no-manifest"}
		}},
		{name: "new_exclude_eq", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--exclude=bin"}
		}},
		{name: "new_all", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--all"}
		}},
		{name: "new_include", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--include", "core/cmd/zerg/testdata/*"}
		}},
		{name: "new_only", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--only", "scripts/dev/keep.sh"}
		}},
		{name: "new_only_missing", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--only", "no/such.txt"}
		}},
		{name: "list", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--list"}
		}},
		{name: "list_all", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--list", "--all"}
		}},
		{name: "dst_inside", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/real/inside"}
		}},
		{name: "dst_eq_form", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst=" + d + "/tree"}
		}},
		{name: "reject_symlinks", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--reject-symlinks"}
		}},
		{name: "verify_ok", seed: verifyTree, argv: func(d string) []string {
			return []string{"verify", d + "/tree", d + "/real"}
		}},
		{name: "verify_diff", seed: func(t *testing.T, root string) {
			verifyTree(t, root)
			isoT_w(t, root+"/real/top.txt", "CHANGED-AFTER\n")
		}, argv: func(d string) []string {
			return []string{"verify", d + "/tree", d + "/real"}
		}},
		{name: "self_test", argv: func(d string) []string { return []string{"--self-test"} }},
	}

	for _, s := range scens {
		t.Run(s.name, func(t *testing.T) {
			fxA := t.TempDir()
			fxB := t.TempDir()
			if s.seed != nil {
				s.seed(t, fxA)
				s.seed(t, fxB)
			}
			argvA := s.argv(fxA)
			argvB := s.argv(fxB)

			var ob, eb bytes.Buffer
			rcGo := isolateRoGateMain(&ob, &eb, argvA)
			rcSh, soSh, seSh := isoT_runSh(t, script, root, argvB)

			// 归一面：两侧夹具根（逻辑 + physical）都折成 <FX>。
			physA, _ := filepath.EvalSymlinks(fxA)
			physB, _ := filepath.EvalSymlinks(fxB)
			norm := func(s string) string {
				return isoT_norm(s, physA, fxA, physB, fxB)
			}
			gotGoOut, gotGoErr := norm(ob.String()), norm(eb.String())
			gotShOut, gotShErr := norm(soSh), norm(seSh)

			if rcGo != rcSh || gotGoOut != gotShOut || gotGoErr != gotShErr {
				t.Fatalf("两态三面不同：\n  Go rc=%d\n  sh rc=%d\n  Go stdout=%q\n  sh stdout=%q\n  Go stderr=%q\n  sh stderr=%q",
					rcGo, rcSh, gotGoOut, gotShOut, gotGoErr, gotShErr)
			}
		})
	}
}

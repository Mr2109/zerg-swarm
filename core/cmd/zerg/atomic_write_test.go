// atomic_write_test.go —— 件 core/cmd/zerg/atomic_write.go（scripts/dev/atomic-write.sh 的 Go 真搬）
// 的回归牙：① 入口三档（help/用法错/正常换件/首建/越界/回读不符/中途被改）② 自检档 15 格
// ③ ★ 两态对拍（sh 直跑 ⟷ Go · 噪声归一后逐字同）。
//
// 两态夹具一律落**仓外**（t.TempDir() ⇒ $TMPDIR · ✗ 不落仓内：门76 会把盘上任何新件算进分母）。
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

// awRunIn 跑入口（fixture 根显式传入 ⇒ 不依赖 repoRoot()）。
func awRunIn(root, histRel, neg string, argv ...string) (int, string, string) {
	var ob, eb bytes.Buffer
	c := &awCtx{root: root, histRel: awOrDefault(histRel, awHistWhitelist), neg: neg, stdout: &ob, stderr: &eb}
	rc := awRunCtx(c, argv)
	return rc, ob.String(), eb.String()
}

func awSeed(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func awMkFixture(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, sub := range []string{"scripts/gates", "scripts/dev", "scripts/bin", "outside"} {
		if err := os.MkdirAll(filepath.Join(d, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func awRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAwHelp(t *testing.T) {
	for _, a := range []string{"-h", "--help"} {
		rc, out, errb := awRunIn(t.TempDir(), "", "", a)
		if rc != 0 || out != awUsage || errb != "" {
			t.Fatalf("%s：rc=%d out=%q err=%q（要 0 / usage / 空）", a, rc, out, errb)
		}
	}
}

func TestAwUsageErrors(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v1\n")
	awSeed(t, d, "outside/new2.sh", "NEW v2\n")
	for _, tc := range []struct {
		name string
		argv []string
		want string
	}{
		{"未知旗标", []string{"--bogus"}, "用法错：未知旗标 --bogus（见 --help）\n"},
		{"多余位置参数", []string{"foo"}, "用法错：多余的位置参数 foo（见 --help）\n"},
		{"缺 --target", []string{"--from", d + "/outside/new2.sh"}, "用法错：缺 --target（见 --help）\n"},
		{"缺 --from", []string{"--target", "scripts/gates/demo.sh"}, "用法错：缺 --from（见 --help）\n"},
		{"--target 缺实参", []string{"--target"}, "用法错：--target 缺实参（见 --help）\n"},
		{"--from 缺实参", []string{"--from"}, "用法错：--from 缺实参（见 --help）\n"},
		{"--from 要绝对路径", []string{"--target", "scripts/gates/demo.sh", "--from", "new2.sh"}, "用法错：--from 要绝对路径（见 --help）\n"},
		{"--from 件不存在", []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/nope.sh"}, "用法错：--from 件不存在：" + d + "/outside/nope.sh\n"},
		{"--target 目录不存在", []string{"--target", "scripts/nope/demo.sh", "--from", d + "/outside/new2.sh"}, "用法错：--target 目录不存在（见 --help）\n"},
	} {
		rc, out, errb := awRunIn(d, "", "", tc.argv...)
		if rc != 2 || out != "" || errb != tc.want {
			t.Fatalf("%s：rc=%d out=%q err=%q（要 2 / 空 / %q）", tc.name, rc, out, errb, tc.want)
		}
	}
}

// TestAwReplaceFresh —— 正常换件：rc=0 · 目标件更新 · 留档 1 枚且 sha == 旧件。
func TestAwReplaceFresh(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v1\n")
	awSeed(t, d, "outside/new2.sh", "NEW v2\n")
	rc, out, errb := awRunIn(d, "", "", "--target", "scripts/gates/demo.sh", "--from", d+"/outside/new2.sh")
	if rc != 0 || errb != "" {
		t.Fatalf("正常换件：rc=%d err=%q", rc, errb)
	}
	if got := awRead(t, filepath.Join(d, "scripts/gates/demo.sh")); got != "NEW v2\n" {
		t.Fatalf("目标件未更新：%q", got)
	}
	if !strings.Contains(out, "✅ 换件成功：scripts/gates/demo.sh") || !strings.Contains(out, "旧件已在 scripts/gates/_history/ 留档") {
		t.Fatalf("换件 stdout 不符：%q", out)
	}
	baks, _ := filepath.Glob(filepath.Join(d, "scripts/gates/_history", "demo.sh.bak-*"))
	if len(baks) != 1 {
		t.Fatalf("留档命中 %d（要 1）", len(baks))
	}
	if h := awTrimNL(awRead(t, baks[0])); h != "OLD v1" {
		t.Fatalf("留档内容不符：%q", h)
	}
}

// TestAwCreateFirst —— 目标件不存在 ⇒ 首建（无旧件可留档）。
func TestAwCreateFirst(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "outside/new.sh", "NEW\n")
	rc, out, errb := awRunIn(d, "", "", "--target", "scripts/gates/new.sh", "--from", d+"/outside/new.sh")
	if rc != 0 || errb != "" || !strings.Contains(out, "首建（无旧件可留档）") {
		t.Fatalf("首建：rc=%d out=%q err=%q", rc, out, errb)
	}
	if awRead(t, filepath.Join(d, "scripts/gates/new.sh")) != "NEW\n" {
		t.Fatalf("首建目标件未落")
	}
}

// TestAwArchiveSkipSameSha —— 已有同 sha 留档 ⇒ 跳过不重写（仍 1 枚）。
func TestAwArchiveSkipSameSha(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v1\n")
	awSeed(t, d, "scripts/gates/_history/demo.sh.bak-20200101-000000", "OLD v1\n")
	awSeed(t, d, "outside/new3.sh", "NEW v3\n")
	rc, out, errb := awRunIn(d, "", "", "--target", "scripts/gates/demo.sh", "--from", d+"/outside/new3.sh")
	if rc != 0 || errb != "" || !strings.Contains(out, "留档跳过：") {
		t.Fatalf("同 sha 跳过：rc=%d out=%q err=%q", rc, out, errb)
	}
	baks, _ := filepath.Glob(filepath.Join(d, "scripts/gates/_history", "demo.sh.bak-*"))
	if len(baks) != 1 {
		t.Fatalf("留档命中 %d（要 1）", len(baks))
	}
}

// TestAwDropOutOfBounds —— 落点越界 ⇒ rc=1、目标件一字节未动。负控：去掉白名单判定 ⇒ rc 变 0 ⇒ 必红。
func TestAwDropOutOfBounds(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v1\n")
	awSeed(t, d, "outside/new2.sh", "NEW v2\n")
	rc, out, errb := awRunIn(d, "bin", "", "--target", "scripts/gates/demo.sh", "--from", d+"/outside/new2.sh")
	if rc != 1 || out != "" || !strings.Contains(errb, "不在 scripts/gates/_history/ 之下") || !strings.Contains(errb, "留档失败/落点越界") {
		t.Fatalf("越界：rc=%d out=%q err=%q", rc, out, errb)
	}
	if awRead(t, filepath.Join(d, "scripts/gates/demo.sh")) != "OLD v1\n" {
		t.Fatalf("越界后目标件被动了")
	}
}

// TestAwCorruptArchive —— 留档回读不符 ⇒ rc=1、目标件未被覆盖。负控：去掉回读比对 ⇒ rc 变 0 ⇒ 必红。
func TestAwCorruptArchive(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v9 unique\n")
	awSeed(t, d, "outside/new2.sh", "NEW v2\n")
	rc, out, errb := awRunIn(d, "", "corrupt_archive", "--target", "scripts/gates/demo.sh", "--from", d+"/outside/new2.sh")
	if rc != 1 || out != "" || !strings.Contains(errb, "留档回读 sha 与旧件不同") {
		t.Fatalf("回读不符：rc=%d out=%q err=%q", rc, out, errb)
	}
	if awRead(t, filepath.Join(d, "scripts/gates/demo.sh")) != "OLD v9 unique\n" {
		t.Fatalf("回读不符后目标件被覆盖")
	}
}

// TestAwMutateTarget —— 写前预校发现中途被改 ⇒ rc=2、目标件一字节未写。负控：去掉预校 ⇒ rc 变 0 ⇒ 必红。
func TestAwMutateTarget(t *testing.T) {
	d := awMkFixture(t)
	awSeed(t, d, "scripts/gates/demo.sh", "OLD v1\n")
	awSeed(t, d, "outside/x.sh", "NEW vX\n")
	rc, _, errb := awRunIn(d, "", "mutate_target", "--target", "scripts/gates/demo.sh", "--from", d+"/outside/x.sh")
	if rc != 2 || !strings.Contains(errb, "写前预校失败") || !strings.Contains(errb, "cmp：") {
		t.Fatalf("中途被改：rc=%d err=%q", rc, errb)
	}
	if got := awRead(t, filepath.Join(d, "scripts/gates/demo.sh")); got != "OLD v1\n\n# intruder\n" {
		t.Fatalf("目标件状态不符：%q", got)
	}
}

// TestAwSelfTest —— 件内 --self-test：15 格全过 ⇒ rc=0（有红即 rc=2）。
func TestAwSelfTest(t *testing.T) {
	var ob, eb bytes.Buffer
	c := &awCtx{histRel: awHistWhitelist, stdout: &ob, stderr: &eb}
	rc := awSelftest(c)
	if rc != 0 {
		t.Fatalf("--self-test：rc=%d（要 0）\n%s", rc, ob.String()+eb.String())
	}
	if !strings.Contains(ob.String(), "自检：15 格过 · 0 格红") {
		t.Fatalf("自检格数不符：\n%s", ob.String())
	}
}

// ── 两态对拍（sh 直跑 ⟷ Go · 噪声归一后逐字同）───────────────────────────────────

var (
	awReBak = regexp.MustCompile(`\.bak-\d{8}-\d{6}(\.\d+)?`)
	awReTmp = regexp.MustCompile(`aw-selftest\.[A-Za-z0-9]{6}`)
)

func awNorm(s string, roots ...string) string {
	for _, r := range roots {
		s = strings.ReplaceAll(s, r, "<FX>")
	}
	s = awReBak.ReplaceAllString(s, ".bak-<STAMP>")
	s = awReTmp.ReplaceAllString(s, "aw-selftest.<TMP>")
	return s
}

func awTestRepoRoot(t *testing.T) string {
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

type awScen struct {
	name string
	seed func(dir string)
	argv func(dir string) []string
	neg  string
	drop string
}

func awRunSh(t *testing.T, script, dir string, argv []string, neg, drop string) (int, string) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, argv...)...)
	cmd.Dir = dir
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ATOMIC_WRITE_") {
			continue
		}
		env = append(env, kv)
	}
	if neg != "" {
		env = append(env, "ATOMIC_WRITE_NEG="+neg)
	}
	if drop != "" {
		env = append(env, "ATOMIC_WRITE_DROP_REL="+drop)
	}
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("起 bash 失败：%v", err)
	}
	return rc, so.String() + se.String()
}

// awT_pyRel / awT_goHalfRel —— 两态对拍所依赖的 sh 原件与其同源 Go 半（本件实现件）的仓根相对路径。
const (
	awT_pyRel     = "scripts/dev/atomic-write.sh"
	awT_goHalfRel = "core/cmd/zerg/atomic_write.go"
)

// awT_fileIsFile —— 存在且是普通文件（退役判定的两缀之一）。
func awT_fileIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func TestAwTwoStateParity(t *testing.T) {
	root := awTestRepoRoot(t)
	script := filepath.Join(root, "scripts", "dev", "atomic-write.sh")
	if _, err := os.Stat(script); err != nil {
		// ★ 三段式回退腿（先例 `check_public_face_anchor_test.go:106` / `check_wallclock_ratchet_test.go:173`
		//   的「退役态 ⇒ 对拍格无对象，容忍跳过」）：sh 原件缺席而其 **Go 半**在盘 ⇒ 退役态
		//   —— 两态对拍没有对拍对象 ⇒ **体面跳过**（✗ 不判失败、✗ 不把断言改宽成永真）。
		//   Go 半自身的断言由本包其余用例（TestAwHelp / TestAwReplaceFresh / TestAwCreateFirst /
		//   TestAwArchiveSkipSameSha / TestAwDropOutOfBounds / TestAwCorruptArchive / TestAwMutateTarget /
		//   TestAwSelfTest）**照常覆盖**。两半皆缺（真消失）⇒ fail-closed（缺件 ≠ 绿），照旧判红。
		if awT_fileIsFile(filepath.Join(root, awT_goHalfRel)) {
			t.Skipf("退役态：sh 原件 %s 已退场（其 Go 半 %s 在盘）⇒ 两态对拍格无对象，容忍跳过", awT_pyRel, awT_goHalfRel)
		}
		t.Fatalf("原件不在盘：%s", script)
	}
	seedDemo := func(dir string) {
		awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
		awSeed(t, dir, "outside/new2.sh", "NEW v2\n")
	}
	scens := []awScen{
		{name: "help", argv: func(d string) []string { return []string{"--help"} }},
		{name: "bogus", argv: func(d string) []string { return []string{"--bogus"} }},
		{name: "positional", argv: func(d string) []string { return []string{"foo"} }},
		{name: "target_missing_arg", argv: func(d string) []string { return []string{"--target"} }},
		{name: "from_missing_arg", argv: func(d string) []string { return []string{"--from"} }},
		{name: "missing_target", argv: func(d string) []string { return []string{"--from", d + "/outside/new2.sh"} }},
		{name: "missing_from", seed: seedDemo, argv: func(d string) []string { return []string{"--target", "scripts/gates/demo.sh"} }},
		{name: "from_relative", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", "new2.sh"}
		}},
		{name: "from_absent", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/nope.sh"}
		}},
		{name: "target_dir_absent", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/nope/demo.sh", "--from", d + "/outside/new2.sh"}
		}},
		{name: "replace_fresh", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}},
		{name: "create_first", seed: func(dir string) {
			awSeed(t, dir, "outside/new.sh", "NEW\n")
		}, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/new.sh", "--from", d + "/outside/new.sh"}
		}},
		{name: "archive_skip_same_sha", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
			awSeed(t, dir, "scripts/gates/_history/demo.sh.bak-20200101-000000", "OLD v1\n")
			awSeed(t, dir, "outside/new3.sh", "NEW v3\n")
		}, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new3.sh"}
		}},
		{name: "drop_out_of_bounds", seed: seedDemo, drop: "bin", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}},
		{name: "corrupt_archive", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v9 unique\n")
			awSeed(t, dir, "outside/new2.sh", "NEW v2\n")
		}, neg: "corrupt_archive", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}},
		{name: "mutate_target", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
			awSeed(t, dir, "outside/x.sh", "NEW vX\n")
		}, neg: "mutate_target", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/x.sh"}
		}},
		{name: "self_test", argv: func(d string) []string { return []string{"--self-test"} }},
	}
	for _, s := range scens {
		t.Run(s.name, func(t *testing.T) {
			fxA := awMkFixture(t) // Go 侧
			fxB := awMkFixture(t) // sh 侧
			if s.seed != nil {
				s.seed(fxA)
				s.seed(fxB)
			}
			// sh 侧须把原件复制进夹具（原件以自身位置上溯定仓根 ⇒ 直接跑原件判的是真仓）。
			if !awCopyP(script, filepath.Join(fxB, "scripts/dev/atomic-write.sh")) {
				t.Fatal("复制原件进 sh 夹具失败")
			}
			argvA := s.argv(fxA)
			argvB := s.argv(fxB)
			rcGo, outGo, errGo := awRunIn(fxA, s.drop, s.neg, argvA...)
			rcSh, outSh := awRunSh(t, filepath.Join(fxB, "scripts/dev/atomic-write.sh"), fxB, argvB, s.neg, s.drop)
			// 产物面（目标件内容）也要比。
			prodA, _ := os.ReadFile(filepath.Join(fxA, "scripts/gates/demo.sh"))
			prodB, _ := os.ReadFile(filepath.Join(fxB, "scripts/gates/demo.sh"))
			gotGo := awNorm(outGo+errGo, fxA)
			gotSh := awNorm(outSh, fxB)
			if rcGo != rcSh || gotGo != gotSh {
				t.Fatalf("两态不同：\n  Go rc=%d out=%q\n  sh rc=%d out=%q", rcGo, gotGo, rcSh, gotSh)
			}
			if string(prodA) != string(prodB) {
				t.Fatalf("产物不同：Go=%q sh=%q", string(prodA), string(prodB))
			}
		})
	}
}

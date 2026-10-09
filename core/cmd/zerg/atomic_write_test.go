// atomic_write_test.go —— 件 core/cmd/zerg/atomic_write.go（scripts/dev/atomic-write.sh 的 Go 真搬）
// 的回归牙：① 入口三档（help/用法错/正常换件/首建/越界/回读不符/中途被改）② 自检档 15 格
// ③ ★ 金标内化（旧 sh 对拍退场 ⇒ 逐格金标收成 Go 常量 · 两态 · ✗ 不 Skip）。
//
// 两态夹具一律落**仓外**（t.TempDir() ⇒ $TMPDIR · ✗ 不落仓内：门76 会把盘上任何新件算进分母）。
package main

import (
	"bytes"
	"os"
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

// ── 内化金标：旧 sh 对拍（恒 Skip · 零保护）的逐格金标收成 Go 常量 ──────────────────
//
// 为什么这么测（M2 口径 · 与 `check_manifest_freshness_test.go` 的
// `TestCheckManifestFreshnessGoldenInternalized` / `...GoldenTwoStates` 同族，并照
// `archive_mount_test.go:241 TestArchiveMountRunsWithoutScript` 的两态体例）：本件原
// 「sh 真身 ⟷ Go 真搬」逐字对拍（旧名 `TestAwTwoStateParity`）的对拍源
// `scripts/dev/atomic-write.sh` 已退场（DEV-1910 M2 批）⇒ 对拍源消失、断言恒 Skip
// （假绿、零保护）。按 DEV-2143 先例把金标真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「输入（seed + argv + neg/drop）⇒ (rc, stdout+stderr)」由 `awRunIn` 现跑捕获
//	  （一次性现算、只读夹具、✗ 不落仓），冻成下表 `wantRC/wantOut`；**本机时点类量**在比对前
//	  归一化（✗ 不冻结 —— 因其本非常量）：
//	    ① 夹具根 `t.TempDir()`（随机路径 + macOS /var↔/private/var）⇒ `<FX>`；
//	    ② 留档名时间戳 `.bak-YYYYMMDD-HHMMSS`（含同名冲突的 `.<pid>` 后缀）⇒ `.bak-<STAMP>`；
//	    ③ `$TMPDIR` ⇒ `<TMP>`；自检临时名 `aw-selftest.<6 位随机>` ⇒ `aw-selftest.<RND>`。
//
// 两态（✗ 不许 Skip）：A 态 = 真仓（退役 sh 不在盘）＋夹具无 sh；B 态 = 合成「旧态」夹具
// （把退役 sh 路径 `scripts/dev/atomic-write.sh` 放回）⇒ 两态**对同一套金标逐字同命中**，证
// sh 在/不在盘都不改判据结论（内化彻底 · 将来若有人让 `awRunIn` 重新去读 sh，B 态当场分家）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `atomic_write.go` 的回读校验一臂哑化
// （`awArchivePrev` 的 `h != src` 判据 → 恒不红）⇒ 本用例**必 FAIL 且带行号**
// （✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。

// awT_shRel —— 对拍真身（已退场 sh）的仓根相对路径；金标两态就钉在它「在 / 不在盘」。
const awT_shRel = "scripts/dev/atomic-write.sh"

type awGoldenCase struct {
	name    string
	seed    func(dir string)
	argv    func(dir string) []string
	neg     string
	drop    string
	wantRC  int
	wantOut string // stdout + stderr 归一化后（现算自证 · ✗ 手写）
}

// awGoldenScens —— 金标真源（17 格）。seed/argv/neg/drop 与原对拍逐格同；
// wantRC/wantOut 由 `awRunIn` 现跑捕获（见件头金标计算式）。
func awGoldenScens(t *testing.T) []awGoldenCase {
	t.Helper()
	seedDemo := func(dir string) {
		awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
		awSeed(t, dir, "outside/new2.sh", "NEW v2\n")
	}
	return []awGoldenCase{
		{name: "help", argv: func(d string) []string { return []string{"--help"} }, wantRC: 0, wantOut: "用法\n  bash scripts/dev/atomic-write.sh --target <仓内相对路径> --from <仓外新件绝对路径>\n  bash scripts/dev/atomic-write.sh --help\n  bash scripts/dev/atomic-write.sh --self-test\n说明\n  --target  要换的件（仓内相对路径，例 scripts/gates/precommit-gates.sh）\n  --from    新内容（仓外新件的绝对路径）\n  步骤：算旧件 sha256 -> 落 scripts/gates/_history/<件名>.bak-<时间戳> -> 回读校验\n        -> 失败即拒写（目标件一个字节不动）-> 写前预校 -> 临时件落目标 -> 写后回读对拍\n退码\n  0 换件成功 · 1 拒写（越界/留档失败/回读不符/写后对拍不符）· 2 不给结论（用法错/缺件/自检不过/预校发现中途被改）\n"},
		{name: "bogus", argv: func(d string) []string { return []string{"--bogus"} }, wantRC: 2, wantOut: "用法错：未知旗标 --bogus（见 --help）\n"},
		{name: "positional", argv: func(d string) []string { return []string{"foo"} }, wantRC: 2, wantOut: "用法错：多余的位置参数 foo（见 --help）\n"},
		{name: "target_missing_arg", argv: func(d string) []string { return []string{"--target"} }, wantRC: 2, wantOut: "用法错：--target 缺实参（见 --help）\n"},
		{name: "from_missing_arg", argv: func(d string) []string { return []string{"--from"} }, wantRC: 2, wantOut: "用法错：--from 缺实参（见 --help）\n"},
		{name: "missing_target", argv: func(d string) []string { return []string{"--from", d + "/outside/new2.sh"} }, wantRC: 2, wantOut: "用法错：缺 --target（见 --help）\n"},
		{name: "missing_from", seed: seedDemo, argv: func(d string) []string { return []string{"--target", "scripts/gates/demo.sh"} }, wantRC: 2, wantOut: "用法错：缺 --from（见 --help）\n"},
		{name: "from_relative", seed: seedDemo, argv: func(d string) []string { return []string{"--target", "scripts/gates/demo.sh", "--from", "new2.sh"} }, wantRC: 2, wantOut: "用法错：--from 要绝对路径（见 --help）\n"},
		{name: "from_absent", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/nope.sh"}
		}, wantRC: 2, wantOut: "用法错：--from 件不存在：<FX>/outside/nope.sh\n"},
		{name: "target_dir_absent", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/nope/demo.sh", "--from", d + "/outside/new2.sh"}
		}, wantRC: 2, wantOut: "用法错：--target 目录不存在（见 --help）\n"},
		{name: "replace_fresh", seed: seedDemo, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}, wantRC: 0, wantOut: "   🗄  已留档旧件：scripts/gates/_history/demo.sh.bak-<STAMP>（7 字节 · sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292）\n✅ 换件成功：scripts/gates/demo.sh（7 字节 · sha256 3d000f7a5aa81a3733017a8a87a78ac651584f193b3d5a319c2d712f9fab5bfa）\n   旧件 sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292 → 新件 sha256 3d000f7a5aa81a3733017a8a87a78ac651584f193b3d5a319c2d712f9fab5bfa（旧件已在 scripts/gates/_history/ 留档）\n"},
		{name: "create_first", seed: func(dir string) { awSeed(t, dir, "outside/new.sh", "NEW\n") }, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/new.sh", "--from", d + "/outside/new.sh"}
		}, wantRC: 0, wantOut: "✅ 换件成功：scripts/gates/new.sh（4 字节 · sha256 21998928836741932680bb7ef8fb3fe2d8709b5afdf896752642d10595f9a610）\n   首建（无旧件可留档）\n"},
		{name: "archive_skip_same_sha", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
			awSeed(t, dir, "scripts/gates/_history/demo.sh.bak-20200101-000000", "OLD v1\n")
			awSeed(t, dir, "outside/new3.sh", "NEW v3\n")
		}, argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new3.sh"}
		}, wantRC: 0, wantOut: "   🗄  留档跳过：demo.sh 与已有留档 demo.sh.bak-<STAMP> 逐字节相同（sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292）—— 不重复留、不覆盖\n✅ 换件成功：scripts/gates/demo.sh（7 字节 · sha256 1469b962e2d11775ab69d13db2b9ea927e9528efccbd19a3d2cab95a8729baf6）\n   旧件 sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292 → 新件 sha256 1469b962e2d11775ab69d13db2b9ea927e9528efccbd19a3d2cab95a8729baf6（旧件已在 scripts/gates/_history/ 留档）\n"},
		{name: "drop_out_of_bounds", seed: seedDemo, drop: "bin", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}, wantRC: 1, wantOut: "   ✗ 留档落点 bin/demo.sh.bak-<STAMP> 不在 scripts/gates/_history/ 之下 ⇒ 判红、拒写（目标件一个字节不动）\n留档失败/落点越界 ⇒ 拒写（scripts/gates/demo.sh 一个字节未动）\n"},
		{name: "corrupt_archive", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v9 unique\n")
			awSeed(t, dir, "outside/new2.sh", "NEW v2\n")
		}, neg: "corrupt_archive", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/new2.sh"}
		}, wantRC: 1, wantOut: "   ✗ 留档回读 sha 与旧件不同（e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 ≠ 97f9bc06223b90a6281380b941837a35e65033fbd170eac1d785e5885f0f035c）：demo.sh.bak-<STAMP> ⇒ 拒写\n留档失败/落点越界 ⇒ 拒写（scripts/gates/demo.sh 一个字节未动）\n"},
		{name: "mutate_target", seed: func(dir string) {
			awSeed(t, dir, "scripts/gates/demo.sh", "OLD v1\n")
			awSeed(t, dir, "outside/x.sh", "NEW vX\n")
		}, neg: "mutate_target", argv: func(d string) []string {
			return []string{"--target", "scripts/gates/demo.sh", "--from", d + "/outside/x.sh"}
		}, wantRC: 2, wantOut: "   🗄  已留档旧件：scripts/gates/_history/demo.sh.bak-<STAMP>（7 字节 · sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292）\n   ✗ 写前预校失败：目标件现值 sha 与第①步读到的旧件 sha 不同\n     记忆旧件 sha256 02bb0d3832a943f15a0d584829983fafb3dbe192062e1af54d51dab139e76292\n     目标件现 sha256 ecaba77e1d5e56d765c97ce30fe35ac7a807e02b04974b7ec5d57140c1ef8fce\n     ⇒ 有人在本件开工后中途改过 scripts/gates/demo.sh ⇒ 拒写、不给结论（目标件一个字节不动）\n   cmp：目标件与预校时刻读数逐字节相同（cmp -s ⇒ rc=0）⇒ 本件未写一个字节\n"},
		{name: "self_test", argv: func(d string) []string { return []string{"--self-test"} }, wantRC: 0, wantOut: "  —— atomic-write.sh --self-test（合成夹具 · 全在 <TMP>/aw-selftest.<RND> 下）——\n    ✅ ① 正常换件 ⇒ rc=0\n    ✅ ① 目标件 == 新件内容\n    ✅ ① _history 命中 1 枚\n    ✅ ② 留档件 sha == 旧件现算 sha（两串相同）\n    ✅ ①b 同名同 sha 已存在 ⇒ 跳过不重写（仍 1 枚）\n    ✅ ③ 中途被改 ⇒ rc=2（非零）\n    ✅ ③ 目标件未被写入新内容（未动）\n    ✅ ③ 回执里有 cmp 证明行\n    ✅ ④ 落点越界 ⇒ rc=1\n    ✅ ④ 落点越界 ⇒ 逐字点名（不在 …/_history/ 之下）\n    ✅ ⑤ 未知旗标 ⇒ rc=2\n    ✅ ⑤ 未知旗标 ⇒ 逐字点名\n    ✅ ⑥ 留档回读不符 ⇒ rc=1（拒写）\n    ✅ ⑥ 回读不符 ⇒ 目标件未被覆盖\n    ✅ ⑥ 回读不符 ⇒ 逐字点名\n  —— 自检：15 格过 · 0 格红（夹具 <TMP>/aw-selftest.<RND>）——\n"},
	}
}

var (
	awReBak = regexp.MustCompile(`\.bak-\d{8}-\d{6}(\.\d+)?`)
	awReTmp = regexp.MustCompile(`aw-selftest\.[A-Za-z0-9]{6}`)
)

// awNormGolden —— 归一本机时点类量（夹具根 / $TMPDIR / 留档时间戳 / 自检临时名）⇒ 金标跨机可复现。
func awNormGolden(s, fx string) string {
	s = strings.ReplaceAll(s, fx, "<FX>")
	if td := os.TempDir(); td != "" && td != fx {
		s = strings.ReplaceAll(s, td, "<TMP>")
	}
	s = awReBak.ReplaceAllString(s, ".bak-<STAMP>")
	s = awReTmp.ReplaceAllString(s, "aw-selftest.<RND>")
	return s
}

// awTestRepoRoot —— 由 cwd 上溯找仓根（判据 = 有 core/internal/version/version.go）。
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

// awRunGolden —— 逐格现跑 Go 半（夹具在 $TMPDIR · 仓外），归一后与冻值逐字比；返回失配格数。
func awRunGolden(t *testing.T, tag string, mkFixture func(*testing.T) string) int {
	t.Helper()
	mism := 0
	for _, c := range awGoldenScens(t) {
		fx := mkFixture(t)
		if c.seed != nil {
			c.seed(fx)
		}
		rc, out, errb := awRunIn(fx, c.drop, c.neg, c.argv(fx)...)
		got := awNormGolden(out+errb, fx)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if got != c.wantOut {
			mism++
			t.Errorf("[%s/%s] 输出漂移\n实得=%q\n冻值=%q", tag, c.name, got, c.wantOut)
		}
	}
	return mism
}

// TestAwTwoStatesGoldenInternalized —— A 态（真仓无 sh）金标自证（✗ 不 Skip · 逐格真断言 ·
// ✗ 不放宽为「只判非空」）。
func TestAwTwoStatesGoldenInternalized(t *testing.T) {
	root := awTestRepoRoot(t)
	sh := filepath.Join(root, awT_shRel)
	if _, err := os.Stat(sh); err == nil {
		t.Fatalf("A 态前提破：退役 sh 竟在盘 %s（本件前提 = 内化后无 sh）", sh)
	}
	if n := awRunGolden(t, "A真仓无sh", awMkFixture); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestAwTwoStatesGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 sh（真仓现态）；B 态 = 合成
// 「旧态」夹具（`scripts/dev/atomic-write.sh` 在位）⇒ 两态对同一套金标逐字同命中（✗ 不 Skip）。
func TestAwTwoStatesGoldenTwoStates(t *testing.T) {
	if n := awRunGolden(t, "A夹具无sh", awMkFixture); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	mkOld := func(t *testing.T) string {
		d := awMkFixture(t)
		awSeed(t, d, awT_shRel, "#!/bin/sh\n# stub：造「旧态」——退役 sh 在位（本件只证 sh 在/不在盘不改判据结论）\n")
		return d
	}
	if n := awRunGolden(t, "B旧态有sh", mkOld); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（sh 在盘不得改判据结论）", n)
	}
}

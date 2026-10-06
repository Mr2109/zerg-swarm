// check_selftest_knobs_test.go —— `check-selftest-knobs.py` Go 真搬（`checkSelftestKnobsGateMain`）的成对负控。
//
// 纪律：✗ 禁恒绿。每枚用例要么钉**逐字节**输出，要么钉**空域/缺件/白名单**这类「改坏必红」的语义面。
// 夹具一律 `t.TempDir()`（仓外 · 自回收）⇒ 不读不写仓内件。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cskWriteTree —— 造仓外夹具树（rel 用正斜杠）。
func cskWriteTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// cskEntry —— 跑入口并接住三面。
func cskEntry(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkSelftestKnobsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cskFxHit —— 提交链点名 一个 .py + 一个 .go，两者各含一枚未登记旋钮。
func cskFxHit(t *testing.T) string {
	return cskWriteTree(t, map[string]string{
		"scripts/gates/precommit-gates.sh": "add_step docs \"门x\" tri \"$R\" \"python3 scripts/gates/check-foo.py\"\n" +
			"add_step core \"门y\" tri \"$R\" \"go test ./core/cmd/zerg/within.go\"\n",
		"scripts/gates/check-foo.py": "import os\ndef main():\n    meta = os.environ.get(\"FOO_SELFTEST\") != \"1\"\n    return meta\n",
		"core/cmd/zerg/within.go":    "package main\nvar knab = os.Getenv(\"BAR_META_OFF\")\n",
	})
}

// cskFxClean —— 提交链点名一个无旋钮件。
func cskFxClean(t *testing.T) string {
	return cskWriteTree(t, map[string]string{
		"scripts/gates/precommit-gates.sh": "add_step docs \"门z\" tri \"$R\" \"python3 scripts/gates/check-bar.py\"\n",
		"scripts/gates/check-bar.py":       "x = 1\ny = x + 2\n",
	})
}

// cskFxAllow —— 提交链点名件只含**已登记**旋钮名 ⇒ 被跳过（rc=0）。
func cskFxAllow(t *testing.T) string {
	return cskWriteTree(t, map[string]string{
		"scripts/gates/precommit-gates.sh": "add_step core \"门a\" tri \"$R\" \"python3 scripts/gates/check-allowed.py\"\n",
		"scripts/gates/check-allowed.py":   "tbl = {\"ZERG_ALLSH_UNITTEST\": {\"readers\": \"all.sh\"}}\n",
	})
}

// cskFxEmpty —— precommit 在盘但不点名任何被扫件 ⇒ 空域。
func cskFxEmpty(t *testing.T) string {
	return cskWriteTree(t, map[string]string{
		"scripts/gates/precommit-gates.sh": "# no add_step\n",
		"scripts/gates/check-foo.py":       "z = os.environ.get(\"FOO_SELFTEST\")\n",
	})
}

// cskFxNoPc —— 完全没有 precommit-gates.sh ⇒ 缺件。
func cskFxNoPc(t *testing.T) string {
	return cskWriteTree(t, map[string]string{
		"scripts/gates/check-foo.py": "z = os.environ.get(\"FOO_SELFTEST\")\n",
	})
}

// ── 逐字节：普通档（命中 2 条 · rc=1）────────────────────────────────────────────
func TestCskHitDefaultBytes(t *testing.T) {
	rc, out, errb := cskEntry("--root", cskFxHit(t))
	want := "提交链「关自检旋钮」元门（GAP-20260927-74）\n" +
		"提交链上未登记的关自检型旋钮 = 2 条\n" +
		"  ✗ check-foo.py:3 旋钮 FOO_SELFTEST [档 unclassified] ⇒ meta = os.environ.get(\"FOO_SELFTEST\") != \"1\"\n" +
		"  ✗ within.go:2 旋钮 BAR_META_OFF [档 unclassified] ⇒ var knab = os.Getenv(\"BAR_META_OFF\")\n" +
		"  口径：提交链上的门不接受关自检型旋钮；要保留须进 KNOB_ALLOW 五件套，\n" +
		"        或改成显式且打印的 --selftest-off 旗标（并在输出里申报）。\n"
	if rc != 1 || out != want || errb != "" {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errb)
	}
}

// ── 逐字节：JSON 档（命中 2 条）──────────────────────────────────────────────────
func TestCskHitJSONBytes(t *testing.T) {
	rc, out, _ := cskEntry("--root", cskFxHit(t), "--json")
	want := `{"rc": 1, "knob_count": 2, "hits": [{"file": "check-foo.py", "line": 3, "knob": "FOO_SELFTEST", "tier": "unclassified"}, {"file": "within.go", "line": 2, "knob": "BAR_META_OFF", "tier": "unclassified"}], "tier_counts": {"isolated": 0, "silent_off": 0, "unclassified": 2}}` + "\n"
	if rc != 1 || out != want {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── 逐字节：干净夹具（rc=0 普通 / JSON）──────────────────────────────────────────
func TestCskCleanBytes(t *testing.T) {
	rc, out, _ := cskEntry("--root", cskFxClean(t))
	want := "提交链「关自检旋钮」元门（GAP-20260927-74）\n" +
		"提交链上未登记的关自检型旋钮 = 0 条\n" +
		"  口径：提交链上的门不接受关自检型旋钮；要保留须进 KNOB_ALLOW 五件套，\n" +
		"        或改成显式且打印的 --selftest-off 旗标（并在输出里申报）。\n"
	if rc != 0 || out != want {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	rc, out, _ = cskEntry("--root", cskFxClean(t), "--json")
	wantJ := `{"rc": 0, "knob_count": 0, "hits": [], "tier_counts": {"isolated": 0, "silent_off": 0, "unclassified": 0}}` + "\n"
	if rc != 0 || out != wantJ {
		t.Fatalf("json rc=%d out=%q", rc, out)
	}
}

// ── 逐字节：--by-tier 档（独立于主 report 的出口）───────────────────────────────
func TestCskByTierBytes(t *testing.T) {
	rc, out, _ := cskEntry("--root", cskFxHit(t), "--by-tier")
	if rc != 1 || out != "isolated      0\nsilent_off    0\nunclassified  2\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── 白名单命中 ⇒ 被跳过（若允许表判据坏掉 ⇒ 该格翻红）───────────────────────────
func TestCskAllowedSkippedRc0(t *testing.T) {
	rc, out, _ := cskEntry("--root", cskFxAllow(t))
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── 空域 vs 缺件 vs 干净：三态必须分开（空转 ≠ 绿）─────────────────────────────
func TestCskEmptyVsMissingVsClean(t *testing.T) {
	rc, _, errb := cskEntry("--root", cskFxEmpty(t))
	if rc != 2 || !strings.Contains(errb, "空域") {
		t.Fatalf("空域: rc=%d err=%q", rc, errb)
	}
	rc, _, errb = cskEntry("--root", cskFxNoPc(t))
	if rc != 2 || !strings.Contains(errb, "缺") {
		t.Fatalf("缺件: rc=%d err=%q", rc, errb)
	}
	rc, out, _ := cskEntry("--root", cskFxClean(t))
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("干净: rc=%d out=%q", rc, out)
	}
	// 不存在的根 ⇒ precommit 取不到 ⇒ rc=2（fail-closed）
	rc, _, errb = cskEntry("--root", filepath.Join(t.TempDir(), "no", "such"))
	if rc != 2 || !strings.Contains(errb, "缺") {
		t.Fatalf("不存在的根: rc=%d err=%q", rc, errb)
	}
}

// ── --self-test：必须自跑自证并 rc=0（其内部两次 run 会向本 stdout/stderr 打印）─
func TestCskSelfTestPass(t *testing.T) {
	rc, out, errb := cskEntry("--self-test")
	if rc != 0 || !strings.HasSuffix(out, "自检 PASS（4 格）\n") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "空域") {
		t.Fatalf("self-test 负控 stderr 缺『空域』: %q", errb)
	}
}

// ── 帮助面逐字节（= py python3.9 正文）──────────────────────────────────────────
func TestCskHelpBytes(t *testing.T) {
	rc, out, errb := cskEntry("-h")
	want := cskUsage + "\n" +
		"提交链「关自检旋钮」只读元门（GAP-20260927-74）\n\n" +
		"optional arguments:\n" +
		"  -h, --help   show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --json\n" +
		"  --by-tier    只按分档打计数（GAP-20260927-74）\n" +
		"  --self-test\n"
	if rc != 0 || out != want || errb != "" {
		t.Fatalf("help rc=%d out=%q err=%q", rc, out, errb)
	}
}

// ── 用法错（未知旗标 / 缺值）退码 2（正文已复刻 argparse）──────────────────────
func TestCskUsageErrors(t *testing.T) {
	rc, _, errb := cskEntry("--bogus")
	if rc != 2 || !strings.Contains(errb, "unrecognized arguments: --bogus") {
		t.Fatalf("bogus rc=%d err=%q", rc, errb)
	}
	rc, _, errb = cskEntry("--root")
	if rc != 2 || !strings.Contains(errb, "argument --root: expected one argument") {
		t.Fatalf("root noval rc=%d err=%q", rc, errb)
	}
}

// ── argparse 唯一前缀缩写：--ro ≡ --root（与 py 同）─────────────────────────────
func TestCskPrefixAbbrev(t *testing.T) {
	hit := cskFxHit(t)
	_, full, _ := cskEntry("--root", hit)
	rc, abbr, _ := cskEntry("--ro", hit)
	if rc != 1 || abbr != full {
		t.Fatalf("prefix rc=%d abbr=%q full=%q", rc, abbr, full)
	}
	// 等号形 --root=<dir> 亦同
	rc, eq, _ := cskEntry("--root=" + hit)
	if rc != 1 || eq != full {
		t.Fatalf("eq rc=%d eq=%q", rc, eq)
	}
}

// ── 旋钮名形态：非关键词 env（如 ZERG_ROOT）不得命中（防 NAME_RE 兜底放宽）───────
func TestCskNonKeywordEnvIgnored(t *testing.T) {
	dir := cskWriteTree(t, map[string]string{
		"scripts/gates/precommit-gates.sh": "add_step docs \"门q\" tri \"$R\" \"python3 scripts/gates/check-plain.py\"\n",
		"scripts/gates/check-plain.py":     "import os\nroot = os.environ.get(\"ZERG_ROOT\")\n",
	})
	rc, out, _ := cskEntry("--root", dir)
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

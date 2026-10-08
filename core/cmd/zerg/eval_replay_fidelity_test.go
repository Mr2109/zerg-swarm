// eval_replay_fidelity_test.go —— 件 eval_replay_fidelity.go 的成对负控（✗ 禁恒绿 · 前缀 TestRfg）。
//
//	正控：与原件 scripts/evals/replay-fidelity.sh 的口径同构（dir_digest 的字节序/两空格/整段再哈希、
//	旗标逐档解析、py str/整数形态、字节计宽补白、`-h` 现读原件第 2..30 行）。
//	负控：每条关键语义各钉一枚「反例」—— 日后谁把实现退化（dir_digest 少排序 / 用来排的读法 /
//	比法器哑化成恒等 / 补白改按字符计宽 / `-h` 只剩内嵌副本），对应用例立刻红。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// jsonNum —— 造一个与 json.Decoder(UseNumber) 同形的数值（供 py 形态测试用）。
func jsonNum(s string) json.Number { return json.Number(s) }

// ---- dir_digest 同构 ----

func TestRfgDirDigestSemantics(t *testing.T) {
	d := t.TempDir()
	// 缺目录 ⇒ MISSING（✗ 不当作空/相等）
	if got := rfgDirDigest(d + "/nope"); got != "MISSING" {
		t.Fatalf("缺目录得 %q 期望 MISSING", got)
	}
	// 空目录 ⇒ EMPTY
	_ = os.MkdirAll(d+"/empty", 0o755)
	if got := rfgDirDigest(d + "/empty"); got != "EMPTY" {
		t.Fatalf("空目录得 %q 期望 EMPTY", got)
	}
	// 同内容判等
	_ = os.MkdirAll(d+"/x", 0o755)
	_ = os.MkdirAll(d+"/y", 0o755)
	_ = os.WriteFile(d+"/x/f.txt", []byte("a\n"), 0o644)
	_ = os.WriteFile(d+"/y/f.txt", []byte("a\n"), 0o644)
	if rfgDirDigest(d+"/x") != rfgDirDigest(d+"/y") {
		t.Fatalf("同内容被判不等")
	}
	// 负控：差一字节 ⇒ 必不等（若比法器哑化成恒等 ⇒ 此断言红）
	_ = os.WriteFile(d+"/y/f.txt", []byte("b\n"), 0o644)
	if rfgDirDigest(d+"/x") == rfgDirDigest(d+"/y") {
		t.Fatalf("差一字节仍判等（比对器退化）")
	}
}

func TestRfgDirDigestFormatAndOrder(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(d+"/a.txt", []byte("z\n"), 0o644)
	_ = os.WriteFile(d+"/b.txt", []byte("z\n"), 0o644)
	sum := sha256.Sum256([]byte("z\n"))
	h := hex.EncodeToString(sum[:])
	// 期望文本：逐件 `<内容 sha256(hex)>  ./<相对路径>`（**两空格**）+ 行尾换行，按**升序**；再整段取 sha256。
	// ⇒ 格式/空格数/`./` 前缀/升序，任一退化即红。
	text := h + "  ./a.txt\n" + h + "  ./b.txt\n"
	outer := sha256.Sum256([]byte(text))
	want := hex.EncodeToString(outer[:])
	if got := rfgDirDigest(d); got != want {
		t.Fatalf("摘要不符（格式/两空格/前缀/升序任一退化即红）:\n got=%s\nwant=%s", got, want)
	}
	// 负控：路径参与摘要（同内容、少一件 ⇒ 必不同）
	e := t.TempDir()
	_ = os.WriteFile(e+"/a.txt", []byte("z\n"), 0o644)
	if rfgDirDigest(e) == want {
		t.Fatalf("路径面/文件集未进摘要（比对器退化）")
	}
}

// ---- 旗标逐档解析 ----

func TestRfgParseArgsFlags(t *testing.T) {
	if a := rfgParseArgs([]string{"--fixture", "/tmp/x", "--selftest", "--keep"}); a.parseErr != "" ||
		a.fixture != "/tmp/x" || !a.selfTestOnly {
		t.Fatalf("正常档解析错: %+v", a)
	}
	if a := rfgParseArgs([]string{"-h"}); !a.helpReq {
		t.Fatalf("-h 未识别")
	}
	if a := rfgParseArgs([]string{"--help"}); !a.helpReq {
		t.Fatalf("--help 未识别")
	}
	// 负控：未知参必须落 parseErr（✗ 不静默吞）
	if a := rfgParseArgs([]string{"--nope"}); a.parseErr == "" {
		t.Fatalf("未知参未报错")
	}
	// 负控：--fixture 缺参必须报错（✗ 不把下一个当空）
	if a := rfgParseArgs([]string{"--fixture"}); a.parseErr == "" {
		t.Fatalf("--fixture 缺参未报错")
	}
	// 原件口径：--fixture 无条件吃下一个 token（哪怕它像旗标）
	if a := rfgParseArgs([]string{"--fixture", "--selftest"}); a.parseErr != "" || a.fixture != "--selftest" || a.selfTestOnly {
		t.Fatalf("--fixture 吃参口径不符: %+v", a)
	}
}

// ---- py str / 整数形态 ----

func TestRfgPyStrForms(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{nil, "None"},
		{true, "true"},
		{false, "false"},
		{"x", "x"},
		{jsonNum("1"), "1"},
		{jsonNum("1.0"), "1.0"},
	}
	for _, c := range cases {
		if got := rfgPyStr(c.in); got != c.want {
			t.Fatalf("rfgPyStr(%v)=%q 期望 %q", c.in, got, c.want)
		}
	}
	// 负控：bool 不许落成 Go 的 "true"/num；整数 %d 档
	if rfgPyInt(jsonNum("6")) != "6" {
		t.Fatalf("rfgPyInt(6) 错")
	}
}

// ---- 补白按字节计宽（✗ 不是字符）----

func TestRfgPadBytesWidth(t *testing.T) {
	// bash printf `%-38s`：33 字节的标签补 5 空格（Go 的 %-38s 按 rune 计 ⇒ 用它会补 27 ⇒ 此断言红）
	label := "自检：同内容目录判相等"
	got := rfgPadBytes(label, rfgCheckWidth)
	if len(got) != rfgCheckWidth || len(got)-len(label) != 5 {
		t.Fatalf("字节补白错: len=%d 尾空格=%d", len(got), len(got)-len(label))
	}
	// 超宽不截
	wide := strings.Repeat("A", 50)
	if rfgPadBytes(wide, rfgCheckWidth) != wide {
		t.Fatalf("超宽被截/补")
	}
	// py 表打印面按**字符**计宽
	if rfgPadRunes("已冻结", 6) != "已冻结"+"   " {
		t.Fatalf("runes 补白错: %q", rfgPadRunes("已冻结", 6))
	}
}

// ---- -h：优先现读原件；原件退役 ⇒ 回落内嵌合成副本 ----

func TestRfgHelpLiveAndFallback(t *testing.T) {
	root := rfgRepoRoot()
	if root == "" {
		t.Skip("本机解析不到仓根 ⇒ 该档不适用")
	}
	b, err := os.ReadFile(root + "/scripts/evals/replay-fidelity.sh")
	if err != nil {
		t.Skip("原件不在盘 ⇒ 仅回落档适用")
	}
	want := strings.Join(strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")[1:30], "\n") + "\n"
	if got := rfgHelpText(root); got != want {
		t.Fatalf("-h 未现读原件第 2..30 行:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	// 负控：原件缺席 ⇒ 必须回落内嵌副本（✗ 不空白、✗ 不报错）——若回落被删 ⇒ 此断言红。
	if got := rfgHelpText("/no/such/repo/root"); got != rfgHelpFallback || got == "" {
		t.Fatalf("退役态回落不符（len=%d）", len(got))
	}
}

// ---- 入口负控：比法器哑化 ⇒ 自检必红（rc=2）----

func TestRfgSelftestNegativeControl(t *testing.T) {
	dir := t.TempDir()
	// 正控：真比法器 ⇒ 自检过 rc=0，且三行自检都在
	var out, errb bytes.Buffer
	rc := evalReplayFidelityGateMain(&out, &errb, []string{"--selftest", "--fixture", filepath.Join(dir, "a")})
	if rc != 0 {
		t.Fatalf("正控 rc=%d stderr=%q", rc, errb.String())
	}
	for _, want := range []string{"自检：同内容目录判相等", "自检：差一字节判不等", "自检：缺目录不算相等"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("正控缺自检行 %q: %q", want, out.String())
		}
	}
	// 负控：把比法器哑化成「什么都判相等」⇒ 自检必须红 ⇒ 入口 rc=2 ＋ 具名话术（✗ 不静默绿）
	old := rfgSelfDigest
	rfgSelfDigest = func(string) string { return "SAME" }
	defer func() { rfgSelfDigest = old }()
	out.Reset()
	errb.Reset()
	rc = evalReplayFidelityGateMain(&out, &errb, []string{"--selftest", "--fixture", filepath.Join(dir, "b")})
	if rc != 2 {
		t.Fatalf("负控 rc=%d 期望 2（哑化比法器必须红）", rc)
	}
	if !strings.Contains(errb.String(), "自检未过") {
		t.Fatalf("负控 stderr 未点名: %q", errb.String())
	}
}

// ---- 接线牙：roster 必须把 replay-fidelity 接到 Go 真身 ----

func TestRfgWiredInRoster(t *testing.T) {
	e := evalFind("replay-fidelity")
	if e == nil {
		t.Fatalf("roster 里没有 replay-fidelity")
	}
	if e.GoMain == nil {
		t.Fatalf("replay-fidelity 未接 Go 真身（GoMain==nil ⇒ 真跑会 exec 旧 .sh，退役后即退 8）")
	}
	// 负控：接的必须是本件入口（若被接到别处/置空 ⇒ 此断言红）
	if reflect.ValueOf(e.GoMain).Pointer() != reflect.ValueOf(evalReplayFidelityGateMain).Pointer() {
		t.Fatalf("replay-fidelity 的 GoMain 不是 evalReplayFidelityGateMain")
	}
}

// ---- 入口负控：未知参 ⇒ rc=2（✗ 不落 0）----

func TestRfgEntryUnknownArg(t *testing.T) {
	var out, errb bytes.Buffer
	rc := evalReplayFidelityGateMain(&out, &errb, []string{"--nope"})
	if rc != 2 {
		t.Fatalf("未知参 rc=%d 期望 2", rc)
	}
	if !strings.Contains(errb.String(), "未知参数：--nope") {
		t.Fatalf("未知参 stderr=%q", errb.String())
	}
	// --fixture 缺参档
	errb.Reset()
	if rc := evalReplayFidelityGateMain(&out, &errb, []string{"--fixture"}); rc != 2 ||
		!strings.Contains(errb.String(), "--fixture 需要一个目录参数") {
		t.Fatalf("--fixture 缺参档 rc=%d stderr=%q", rc, errb.String())
	}
}

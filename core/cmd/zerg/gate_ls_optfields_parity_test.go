// gate_ls_optfields_parity_test.go —— `gate ls` 的**可选字段层**两张手抄表逐字对拍
// （缺口 `GAP-20261010-13` · 本片）：`gate.go` 的 `gateLsOptFields` ⟷ `main.go` 命令树里
// `gate ls` 登记格的 `optFields`。两表**手抄** ⇒ 可**静默漂移**（`gate ls` 不走
// `emitSelected`/`--help`/合法字段面 ⇒ 漂了也**无观测差** = 语义债）⇒ 本判据把漂移
// **变可见**：缺一即红（两向：任一表多/少一格都红）。
//
// 形制照同族先例 `core/internal/api/routes_test.go` 的 `TestRouteTableMatchesRealRoutes`
// （「表 ⟷ 真源逐条命中」）—— 但本判据与两张表**同包** ⇒ 直接读真源（`find` 与
// `gateLsOptFields`），✗ 不扫文本、✗ 不另抄一份。
package main

import (
	"os/exec"
	"strings"
	"testing"
)

// gateLsOptFieldsEqual —— 两张可选字段表**逐字**比对（序 + 值逐格同 ⇒ true）。
// 提成纯函数 ⇒ 成对负控能直接喂合成对（哑化本比对 ⇒ 负控对红）。
func gateLsOptFieldsEqual(a, b []string) bool {
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

// TestGateLsOptFieldsParity —— `gate.go` 的 `gateLsOptFields` ⟷ `main.go` 里 `gate ls` 的
// `optFields` 两表逐字一致。缺一即红。
func TestGateLsOptFieldsParity(t *testing.T) {
	cmd := find([]string{"gate", "ls"})
	if cmd == nil {
		t.Fatal("命令树里找不到 `gate ls`（登记格缺失 ⇒ 判据不成立）")
	}
	if !gateLsOptFieldsEqual(gateLsOptFields, cmd.optFields) {
		t.Fatalf("两表漂移（`gate.go` 的 gateLsOptFields ⟷ `main.go` 里 `gate ls` 的 optFields 逐字不一致）：\n"+
			"  gate.go:366  gateLsOptFields      = %q\n"+
			"  main.go:788  gate ls 登记 optFields = %q",
			gateLsOptFields, cmd.optFields)
	}
	t.Logf("两表逐字一致：gateLsOptFields = optFields = %q", gateLsOptFields)
}

// TestGateLsOptFieldsEqualPaired —— 成对负控：一致 ⇒ true；任一表多/少一格 ⇒ false。
// 哑化 `gateLsOptFieldsEqual`（恒 true / 恒 false）⇒ 本用例必红（证明主判据的比对有牙）。
func TestGateLsOptFieldsEqualPaired(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"两表逐字同 ⇒ 一致", []string{"name", "gates_total"}, []string{"name", "gates_total"}, true},
		{"右表缺一格 ⇒ 不一致", []string{"name", "gates_total"}, []string{"name"}, false},
		{"左表缺一格 ⇒ 不一致", []string{"name"}, []string{"name", "gates_total"}, false},
		{"值同序不同 ⇒ 不一致", []string{"name", "gates_total"}, []string{"gates_total", "name"}, false},
	}
	for _, c := range cases {
		if got := gateLsOptFieldsEqual(c.a, c.b); got != c.want {
			t.Errorf("%s：gateLsOptFieldsEqual(%q, %q) = %v，要 %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

// ── 缺口 `GAP-20260929-22`：步名引号安全的**自证** ＋ 成对负控 ────────────────────────────────────
//
// `gateLsShellQuote` 拼出来的串必须能让 shell **逐字还原**原串（含单引号 / 反引号 / `$` /
// 空白 / CJK）—— 这是「可直接粘贴的整条命令」能成立的**唯一**前提（拼错 ⇒ 粘贴即退 2 假红）。

// gqRunShell —— 真喂 shell 转一圈：把 `shellForm` 当命令串交给 `bash -c`，读回它实际拿到的字节。
// 只用于**只读**的 printf 侦察 ⇒ 零副作用（✗ 不碰仓、✗ 不起 zerg、✗ 不写任何件）。
func gqRunShell(shellForm string) (string, bool) {
	out, err := exec.Command("bash", "-c", "printf '%s' "+shellForm).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// TestGateLsShellQuoteSelfProven —— 自证：`gateLsShellQuote` 的产物过 shell 一圈**逐字节**还原原串；
// 且含单引号的步名**必须**用「闭引号 ＋ 反斜杠单引号 ＋ 重开引号」的写法（✗ 不是把串直接塞进一对单引号里
// —— 那种拼法会撑破引号结构）。
func TestGateLsShellQuoteSelfProven(t *testing.T) {
	cases := []string{
		"gofmt -l core",
		"门㊶ 抓取/解析纪律：`grep` 破折号模式必须 `-e`/`--` + 禁贪婪剥标签（阻断）",
		"门㊷ 过滤/脱敏面形态枚举：`~`/`$HOME`/绝对 全枚举 + 出处可核（阻断）",
		"合 成'步`x`",
		"it's a 'test'",
		"双引号 \" 与反斜杠 \\ 与分号 ;; 与竖线 | 与星号 *",
		"$HOME 与 ${VAR} 与 $(id) 与 `id`",
	}
	for _, s := range cases {
		got, ok := gqRunShell(gateLsShellQuote(s))
		if !ok || got != s {
			t.Fatalf("引号自证失败：%q 过 shell 一圈变成 %q（ok=%v）—— 正确形态 = %q", s, got, ok, gateLsShellQuote(s))
		}
	}
	q := gateLsShellQuote("合 成'步")
	if !strings.Contains(q, `'\''`) {
		t.Fatalf("含单引号的步名必须按 `'\\''` 闭合，实得 %q", q)
	}
}

// TestGateLsShellQuotePaired —— 成对负控：同一批串用「裸粘 / 双引号」两种**错形态**过 shell ⇒ 必被改写
// （证主判据的对照面有牙：不是「怎么拼都过」）。
func TestGateLsShellQuotePaired(t *testing.T) {
	naked, ok := gqRunShell("合 成 步")
	if !ok || naked == "合 成 步" {
		t.Fatalf("负控甲（裸粘）失败：%q —— 含空白的串裸粘后被切词，不该还原成原串", naked)
	}
	dq, ok := gqRunShell(`"$HOME"`)
	if !ok || dq == "$HOME" || dq == "" {
		t.Fatalf("负控乙（双引号）失败：%q —— `$` 应被 shell 展开（串被改写）", dq)
	}
}

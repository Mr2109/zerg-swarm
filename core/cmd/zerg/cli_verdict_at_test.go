// cli_verdict_at_test.go —— 缺口 `GAP-20260927-385` 的**机检**：错误面 `error.verdict_at`
// （**哪一枚判定退出** —— 判定位置可机检，使「判到还是被早退拦」能自动对账）。
//
// 病（缺口正文逐字）：「判到还是被早退拦」不可机检：退码 2 不带「哪一枚判定给的」字段，
// 只能人读 stderr 尾行 ⇒ 判定位置是否前置无法自动对账。
//
// 治法：在**统一错误面**（`errors.go` 的 `error` 块）增 `verdict_at` —— 取 `setErr` 调用点的
// `<仓内相对路径>:<行号>`（哪一枚判定退出）；取不到判定位置 ⇒ 按本仓三值纪律**明报「判不了」**
// （✗ 不填一个看起来像位置的空值）。
//
// 三条判据（可机检 · 一条正控 + 两条成对负控）：
//
//	正控   人为造一处判定退出（`gate four-states` 缺两侧 ⇒ `error.detail` = `missing_sides`）⇒
//	       `error.verdict_at` 可读，且**位置逐字相符**（== 源件里那一处 `setErr` 的行号，现读现算）。
//	       ★ 锚件挑 `family_gate_four_states.go`（本批未被并发改的稳定件）—— 免与在途改件抢行号。
//	负控① 正常路（成功面）**不出** `error` 块 ⇒ 没有 `verdict_at`（按既有档：无事就没有那一块）。
//	负控② 取不到判定位置（兜底路：没走过 `setErr`）⇒ 明报 `"判不了"`，而**不是**伪值；反证：
//	       给了真位置 ⇒ 逐字给位置（✗ 不吞成「判不了」）。
//
// ★ 落点：`package main_test`（外部测试包）—— 与 `cli_envelope_truth_test.go` 同一批「包封真值」判据，
// 共用其 `runCapture` / `repoRootFromCLI` / `tail` 等桥；本件**零写面**（跑的都是只读命令）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// verdictAtRe —— `verdict_at` 的合法形态：`core/cmd/zerg/<件>.go:<行号>`（归一后的仓内相对路径）。
var verdictAtRe = regexp.MustCompile(`^core/cmd/zerg/[a-z0-9_]+\.go:[1-9][0-9]*$`)

// verdictAtErrOf 跑一条命令并把 `error` 块的 kind / detail / verdict_at 现读出来（正控用）。
func verdictAtErrOf(t *testing.T, argv ...string) (string, string, string) {
	t.Helper()
	rc, out, errb := runCapture(argv...)
	if rc == 0 {
		t.Fatalf("准备态不对：正控要一条**判定退出**的命令（rc≠0），得 rc=0 · argv=%v · out=%q", argv, out)
	}
	var doc struct {
		Err struct {
			Kind      string `json:"kind"`
			Detail    string `json:"detail"`
			VerdictAt string `json:"verdict_at"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("包封不是 JSON：%v · out=%q · stderr=%s", err, out, tail(errb, 200))
	}
	return doc.Err.Kind, doc.Err.Detail, doc.Err.VerdictAt
}

// setErrLineOf 现读源件、返回含 `anchor` 的那一行的行号（命中数必须恰 1，否则报错）。
// 「位置逐字相符」的现读真值就从这里来 —— 不写死行号（写死会随同行漂）。
func setErrLineOf(t *testing.T, root, rel, anchor string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("读源件失败：%v（%s）", err, rel)
	}
	hit := 0
	line := 0
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.Contains(ln, anchor) {
			hit++
			line = i + 1
		}
	}
	if hit != 1 {
		t.Fatalf("锚 %q 在 %s 里命中 %d 次（要恰 1 次）", anchor, rel, hit)
	}
	return line
}

// ── 正控：一处判定退出 ⇒ `verdict_at` 可读且位置逐字相符 ─────────────────────────────────────
func TestVerdictAt_RealCriterionExit(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 人为造一处判定退出：`gate four-states` 缺 `--current` / `--patched` ⇒
	// 走 `setErr("usage", "missing_sides", …)` 那一枚判定（锚件 = 稳定件 `family_gate_four_states.go`）。
	kind, detail, at := verdictAtErrOf(t, "gate", "four-states", "--json", "cell")
	if kind != "usage" || detail != "missing_sides" {
		t.Fatalf("准备态不对：error.kind=%q detail=%q（要 usage / missing_sides）", kind, detail)
	}
	// ① 值域：`<core/cmd/zerg/件.go>:<行号>`（不是空串、不是伪值、"判不了"）。
	if !verdictAtRe.MatchString(at) {
		t.Fatalf("`verdict_at` = %q 不是 `<core/cmd/zerg/件.go>:<行号>` 形态", at)
	}
	// ② 位置**逐字相符**：现读源件算出那一处 `setErr` 的行号 ⇒ 与 `verdict_at` 逐字比。
	const rel = "core/cmd/zerg/family_gate_four_states.go"
	want := rel + ":" + strconv.Itoa(setErrLineOf(t, root, rel, `inv.setErr("usage", "missing_sides"`))
	if at != want {
		t.Fatalf("`verdict_at` = %q，源件现读那一处判定 = %q（要逐字相符）", at, want)
	}
}

// ── 负控①：正常路（成功面）不出 `error` 块 ⇒ 没有 `verdict_at` ─────────────────────────────────
func TestVerdictAt_AbsentOnNormalPath(t *testing.T) {
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	rc, out, errb := runCapture("version", "--json", "name")
	if rc != 0 {
		t.Fatalf("准备态不对：`version --json name` 应退 0，得 %d · stderr=%s", rc, tail(errb, 200))
	}
	if strings.Contains(out, "verdict_at") || strings.Contains(out, `"error"`) {
		t.Fatalf("负控①失败：正常路出了 `error` 块 / `verdict_at`：%q", out)
	}
	// 反证（区分度）：同一面**判定退出** ⇒ 该字段真出现（证明这一格不是恒缺）。
	if _, _, at := verdictAtErrOf(t, "version", "--json", "no_such_field__probe"); at == "" {
		t.Fatal("负控①反证失败：判定退出那一态竟没有 `verdict_at`（字段没接上）")
	}
}

// ── 负控②：取不到判定位置 ⇒ 明报「判不了」而非伪值 ─────────────────────────────────────────────
func TestVerdictAt_UnknownIsDeclared(t *testing.T) {
	// 合成一路：没走过 `setErr` 的那种错（兜底路 ⇒ `VerdictAt` 空）—— 渲染端必须**明报判不了**。
	got := zerg.CliErrJSONForTest("usage", "synthetic_no_site", "合成：没报过判定点", "")
	if !strings.Contains(got, `"verdict_at":"判不了"`) {
		t.Fatalf("负控②失败：取不到判定位置时应明报「判不了」，实得 %q", got)
	}
	if strings.Contains(got, `"verdict_at":""`) {
		t.Fatalf("负控②失败：取不到判定位置竟落空串（伪值）：%q", got)
	}
	// 反证（区分度）：给了真位置 ⇒ **逐字**给位置（✗ 不吞成「判不了」）。
	pos := "core/cmd/zerg/errors.go:42"
	got2 := zerg.CliErrJSONForTest("usage", "synthetic_with_site", "合成：带位置", pos)
	if !strings.Contains(got2, `"verdict_at":"`+pos+`"`) {
		t.Fatalf("负控②反证失败：给了位置却不当位置给：%q", got2)
	}
}

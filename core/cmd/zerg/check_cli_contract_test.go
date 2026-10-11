// check_cli_contract_test.go —— 判据件 `scripts/gates/check-cli-contract.py` 的 Go 真搬
// （`core/cmd/zerg/check_cli_contract.go`）的**成对负控**测试（✗ 禁恒绿）。
//
// 覆盖（成对：正控 / 负控 / 边界各一格）：
//
//	FixtureGreen_Rc0          · 好夹具 ⇒ rc=0
//	T1MissingCase_Rc1         · 命令树多一条、矩阵没 case ⇒ rc=1 且点名
//	T2WantRcZero_Rc1          · want_rc=0 ⇒ rc=1（must-fail 矩阵不许有绿格）
//	GhostEntry_Rc1            · 幽灵条目 ⇒ rc=1
//	T3DupCode_Rc1             · 表内同数字两义 ⇒ rc=1
//	T3BareNumeric_Rc1         · 裸数字退出码不在表里 ⇒ rc=1
//	T5ExemptionNoReason_Rc2   · 豁免无 reason ⇒ rc=2（不给结论，不是红）
//	BaselineMissing_Rc2       · baseline 缺失 ⇒ rc=2
//	T6RegistryNoRef_Rc1       · 登记表不点名本脚本 ⇒ rc=1
//	EmptyMatrix_Rc2           · 矩阵空 ⇒ rc=2（空转不许当绿）
//	R7NodeFamilyHit_Rc1       · 全仓另有被禁族名写法 ⇒ rc=1（拼接构造，不落字面量）
//	R7PositiveAfterRemove_Rc0 · 去掉那处写法 ⇒ rc=0（证明红红在那一处）
//	JsonRefused_Rc2           · `--json` ⇒ rc=2（本门无机器读面 Q-176）
//	UnknownFlag_Rc2           · 未知旗标 ⇒ rc=2（argparse 形状）
//	RootNotFound_Rc2          · 找不到仓根 ⇒ rc=2
//	HelpRc0                   · `-h` ⇒ rc=0
//	SelfTest_Rc0_AndSelfCleans· `--self-test` ⇒ rc=0 且临时目录自回收 delta=0（A 类）
//	LiveVerifyGhost_Rc1       · `--verify-live` 幽灵格 ⇒ rc=1 并点名
//	LiveVerifyMissingBin_Rc2  · `--verify-live` 缺制品 ⇒ rc=2
//	EmitMatrixIncremental_Rc0 · `--emit-matrix` 增量合并 ⇒ rc=0 且派生列是**字节数**
//	EmitMatrixMissingBin_Rc2  · `--emit-matrix` 缺制品 ⇒ rc=2
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cctRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := cliContractGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

// cctFixture 在 t.TempDir() 下造一枚合规矩的仓（好夹具），返回仓根。
func cctFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	clicFixture(root, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
	return root
}

func cctContains(hay string, subs ...string) bool {
	for _, s := range subs {
		if !strings.Contains(hay, s) {
			return false
		}
	}
	return true
}

func TestCliContractGate(t *testing.T) {
	t.Run("FixtureGreen_Rc0", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctFixture(t), "--no-runtime")
		if rc != 0 {
			t.Fatalf("好夹具应 rc=0，得 rc=%d\n%s", rc, out)
		}
		if !cctContains(out, "命令树 2 条", "R7 族名闭合", "命中）：0 处") {
			t.Fatalf("读数不对：\n%s", out)
		}
	})

	t.Run("T1MissingCase_Rc1", func(t *testing.T) {
		root := cctFixture(t)
		body, _ := clicReadFile(filepath.Join(root, clicMainRel))
		body = strings.ReplaceAll(body, "\t}\n}\n", "\t\t{path: []string{\"agent\", \"ls\"}, fields: []string{\"machine\"}, run: cmdAgentLs},\n\t}\n}\n")
		if err := os.WriteFile(filepath.Join(root, clicMainRel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 {
			t.Fatalf("应 rc=1，得 rc=%d\n%s", rc, out)
		}
		if !cctContains(out, "T1 覆盖", "agent ls") {
			t.Fatalf("判词没点名缺 case 的命令：\n%s", out)
		}
	})

	t.Run("T2WantRcZero_Rc1", func(t *testing.T) {
		root := t.TempDir()
		m := clicDeepCopy(clicGoodMatrix())
		m["cases"].([]any)[0].(map[string]any)["want_rc"] = 0.0
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "T2 must-fail") {
			t.Fatalf("want_rc=0 应 rc=1 且带 T2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("GhostEntry_Rc1", func(t *testing.T) {
		root := t.TempDir()
		m := clicDeepCopy(clicGoodMatrix())
		m["cases"] = append(m["cases"].([]any), map[string]any{"id": "ghost#x", "command": "no such",
			"argv": []any{"no", "such"}, "want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "幽灵"})
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "幽灵") {
			t.Fatalf("幽灵条目应 rc=1，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("T3DupCode_Rc1", func(t *testing.T) {
		root := t.TempDir()
		ec := strings.Replace(clicGoodExitcodes, `{2, "usage", "用法错", "—"},`, `{0, "usage", "用法错", "—"},`, 1)
		clicFixture(root, clicGoodMain, ec, clicGoodMatrix(), clicGoodBaseline(), clicGoodRegistry(), "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "同数字两义") {
			t.Fatalf("同数字两义应 rc=1，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("T3BareNumeric_Rc1", func(t *testing.T) {
		root := cctFixture(t)
		p := filepath.Join(root, clicMainRel)
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("\nfunc bogus() int { return " + "77 }\n"); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "裸数字") {
			t.Fatalf("裸数字 77 应 rc=1 且带「裸数字」，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("T5ExemptionNoReason_Rc2", func(t *testing.T) {
		root := t.TempDir()
		m := clicDeepCopy(clicGoodMatrix())
		m["exemptions"] = []any{map[string]any{"command": "core stop", "reason": ""}}
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 2 || !strings.Contains(out, "没有 reason") {
			t.Fatalf("豁免无 reason 应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("BaselineMissing_Rc2", func(t *testing.T) {
		root := cctFixture(t)
		_ = os.Remove(filepath.Join(root, clicBaselineRel))
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 2 {
			t.Fatalf("baseline 缺失应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("T6RegistryNoRef_Rc1", func(t *testing.T) {
		root := t.TempDir()
		reg := clicDeepCopy(clicGoodRegistry())
		reg["entries"].([]any)[0].(map[string]any)["gate"] = "scripts/gates/check-error-kinds.py"
		clicFixture(root, clicGoodMain, clicGoodExitcodes, clicGoodMatrix(), clicGoodBaseline(), reg, "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "T6 引用") {
			t.Fatalf("引用面=0 应 rc=1，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("EmptyMatrix_Rc2", func(t *testing.T) {
		root := t.TempDir()
		m := clicDeepCopy(clicGoodMatrix())
		m["cases"] = []any{}
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 2 {
			t.Fatalf("矩阵空应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("R7NodeFamilyHit_Rc1", func(t *testing.T) {
		root := cctFixture(t)
		// 被禁写法用拼接构造，✗ 不落字面量（防本测试件自己进全仓扫描面）
		if err := os.WriteFile(filepath.Join(root, "docs-命令说明.md"),
			[]byte("跑 `zerg "+"node reap` 收残留\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 1 || !strings.Contains(out, "R7 族名") {
			t.Fatalf("被禁族名写法应 rc=1，得 rc=%d\n%s", rc, out)
		}
		if !strings.Contains(out, "命中）：1 处") {
			t.Fatalf("命中数应为 1：\n%s", out)
		}
	})

	t.Run("R7PositiveAfterRemove_Rc0", func(t *testing.T) {
		root := cctFixture(t)
		p := filepath.Join(root, "docs-命令说明.md")
		if err := os.WriteFile(p, []byte("跑 `zerg "+"node reap` 收残留\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(p)
		rc, out, _ := cctRun(t, root, "--no-runtime")
		if rc != 0 {
			t.Fatalf("去掉那处写法应 rc=0，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("JsonRefused_Rc2", func(t *testing.T) {
		rc, out, errs := cctRun(t, "--json")
		if rc != 2 {
			t.Fatalf("--json 应 rc=2，得 rc=%d", rc)
		}
		if out != "" {
			t.Fatalf("--json 拒收应零 stdout，得 %q", out)
		}
		if !cctContains(errs, "本门无机器读面", "字段表：无（本门只出人读面）", "机器面缺 ≠ 零命中") {
			t.Fatalf("拒收正文不对：%q", errs)
		}
	})

	t.Run("UnknownFlag_Rc2", func(t *testing.T) {
		rc, _, errs := cctRun(t, "--nope-zz")
		if rc != 2 || !strings.Contains(errs, "unrecognized arguments") {
			t.Fatalf("未知旗标应 rc=2 + usage 错，得 rc=%d errs=%q", rc, errs)
		}
	})

	t.Run("RootNotFound_Rc2", func(t *testing.T) {
		empty := t.TempDir()
		rc, out, _ := cctRun(t, empty)
		if rc != 2 || !strings.Contains(out, "找不到仓根") {
			t.Fatalf("找不到仓根应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("HelpRc0", func(t *testing.T) {
		rc, out, _ := cctRun(t, "-h")
		if rc != 0 || !strings.Contains(out, "usage: "+clicProg) {
			t.Fatalf("-h 应 rc=0 + usage，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("SelfTest_Rc0_AndSelfCleans", func(t *testing.T) {
		// ★ GAP-20261003-16：私有计数面（TMPDIR 指到本用例自建空目录）⇒ 只数本用例自建的
		// `clicontract-*`，不再与同包兄弟用例 / 同机并发门跑（并发建删同类目录）共享面。
		// 断言未放宽：仍是 `after-before != 0` 即红；主件删 `defer os.RemoveAll(tmp)` 时私有面内
		// 必留 `clicontract-*` ⇒ 仍能抓住真泄漏。（本包零 `t.Parallel()` ⇒ `t.Setenv` 安全。）
		t.Setenv("TMPDIR", t.TempDir())
		before := cctTempCount(t)
		rc, out, _ := cctRun(t, "--self-test")
		after := cctTempCount(t)
		if rc != 0 {
			t.Fatalf("--self-test 应 rc=0，得 rc=%d\n%s", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过（失败 0 条）") {
			t.Fatalf("自检结论不对：\n%s", out)
		}
		if after-before != 0 {
			t.Fatalf("A 类 tempdir 泄漏：跑前 %d 跑后 %d（delta=%d，应为 0）", before, after, after-before)
		}
	})

	t.Run("LiveVerifyGhost_Rc1", func(t *testing.T) {
		root := t.TempDir()
		m := clicDeepCopy(clicGoodMatrix())
		m["cases"].([]any)[1].(map[string]any)["want_rc"] = 0.0
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		_ = os.MkdirAll(filepath.Join(root, "bin"), 0o755)
		_ = os.WriteFile(filepath.Join(root, "bin", "zerg"), []byte("#!/bin/sh\nexit 2\n"), 0o755)
		rc, out, _ := cctRun(t, root, "--verify-live")
		if rc != 1 || !strings.Contains(out, "幽灵格") || !strings.Contains(out, "core/stop") {
			t.Fatalf("幽灵格应 rc=1 且点名，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("LiveVerifyMissingBin_Rc2", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctFixture(t), "--verify-live")
		if rc != 2 || !strings.Contains(out, "找不到 bin/zerg") {
			t.Fatalf("缺制品应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("EmitMatrixIncremental_Rc0", func(t *testing.T) {
		root := t.TempDir()
		seed := clicDeepCopy(clicGoodMatrix())
		seed["cases"] = []any{
			map[string]any{"id": "version#手写格", "command": "version", "argv": []any{"version"},
				"want_rc": 2.0, "want_stdout_bytes": 0.0, "why": "手写：版本号后面跟东西 ⇒ 2"},
			map[string]any{"id": "version#未知旗标", "command": "version", "argv": []any{"version", "--x"},
				"want_rc": 9.0, "want_stdout_bytes": 0.0, "why": "旧 why（候选面不许覆盖它）"},
		}
		seed["exemptions"] = []any{map[string]any{"command": "core stop", "reason": "夹具豁免（合并不许丢）"}}
		clicFixture(root, clicGoodMain, clicGoodExitcodes, seed, clicGoodBaseline(), clicGoodRegistry(), "")
		_ = os.MkdirAll(filepath.Join(root, "bin"), 0o755)
		script := "#!/bin/sh\nprintf '%s\\n' '用法错：中文输出'\nexit 2\n"
		_ = os.WriteFile(filepath.Join(root, "bin", "zerg"), []byte(script), 0o755)
		dst := filepath.Join(t.TempDir(), "out.json")
		rc, out, _ := cctRun(t, root, "--emit-matrix", dst)
		if rc != 0 {
			t.Fatalf("emit 应 rc=0，得 rc=%d\n%s", rc, out)
		}
		b, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		cs := clicCasesOf(got)
		if len(cs) < 3 {
			t.Fatalf("格数应 ≥3（两条既有 + 追加），得 %d", len(cs))
		}
		// 手写格身份列逐字保留
		if clicMapStr(cs[0], "why", "") != "手写：版本号后面跟东西 ⇒ 2" {
			t.Fatalf("手写格 why 没保留：%q", clicMapStr(cs[0], "why", ""))
		}
		// 派生列是**字节数**（中文 ⇒ 字节 > 字符）
		wantBytes := len([]byte("用法错：中文输出\n"))
		gotB, _ := clicPyInt(clicMapGetOr(cs[0], "want_stdout_bytes", nil))
		if gotB != wantBytes {
			t.Fatalf("派生列应为字节数 %d，得 %d（记成字符数就是老 bug ①）", wantBytes, gotB)
		}
		// 既有格的 argv/why 不被候选面覆盖
		found := false
		for _, c := range cs {
			if clicMapStr(c, "id", "") == "version#未知旗标" {
				found = true
				if clicMapStr(c, "why", "") != "旧 why（候选面不许覆盖它）" {
					t.Fatalf("既有格 why 被覆盖：%q", clicMapStr(c, "why", ""))
				}
			}
		}
		if !found {
			t.Fatal("既有格 version#未知旗标 丢了")
		}
		// exemptions 保留
		if len(clicExemptionsOf(got)) != 1 {
			t.Fatalf("exemptions 被丢了：%v", got["exemptions"])
		}
	})

	t.Run("EmitMatrixMissingBin_Rc2", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctFixture(t), "--emit-matrix")
		if rc != 2 || !strings.Contains(out, "找不到 bin/zerg") {
			t.Fatalf("缺制品应 rc=2，得 rc=%d\n%s", rc, out)
		}
	})
}

func cctTempCount(t *testing.T) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(os.TempDir(), "clicontract-*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

// TestCliContractLiveVerifyUnpinned —— 缺口 `GAP-20261012-05` 的**成对正负控**：
// R-6 逐格回放里「格标了 `stdout_bytes_unpinned:true`」= 只钉 rc、不钉 stdout 字节数。
//
//	UnpinnedSet_Rc0        · 该格标了旗标 ＋ 其 stdout 两遍字节数变 ⇒ rc=0（正控）
//	UnpinnedAbsent_Rc1     · 同格拿掉旗标 ⇒ rc=1 且判词点名「两次现跑不等」（负控）
//	UnpinnedButRcWrong_Rc1 · 标了旗标但 want_rc 错 ⇒ 仍 rc≠0 且报退码差（对照：防呆不许把 rc 也比没了）
//
// 成对断言：rc(UnpinnedSet) != rc(UnpinnedAbsent)。
func TestCliContractLiveVerifyUnpinned(t *testing.T) {
	// cctUnpinnedFixture 造一枚**只含一格**的自验仓：假 bin 每次现跑多打 1 字节
	//（`bin/zerg` 用 `bin/.n` 计数并打 n 个 'x'）⇒ 同一格的 stdout 字节数两遍必变。
	cctUnpinnedFixture := func(t *testing.T, unpinned bool, wantRc float64) string {
		t.Helper()
		root := t.TempDir()
		c := map[string]any{"id": "version#两遍变", "command": "version",
			"argv": []any{"version", "--ts"}, "want_rc": wantRc, "want_stdout_bytes": 1.0,
			"why": "GAP-20261012-05：stdout 字节数两遍不定"}
		if unpinned {
			c["stdout_bytes_unpinned"] = true
		}
		m := clicGoodMatrix()
		m["cases"] = []any{c}
		clicFixture(root, clicGoodMain, clicGoodExitcodes, m, clicGoodBaseline(), clicGoodRegistry(), "")
		_ = os.MkdirAll(filepath.Join(root, "bin"), 0o755)
		_ = os.WriteFile(filepath.Join(root, "bin", "zerg"), []byte(cctChangingBin), 0o755)
		return root
	}

	var rcPos, rcNeg int
	t.Run("UnpinnedSet_Rc0", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctUnpinnedFixture(t, true, 0.0), "--verify-live")
		rcPos = rc
		if rc != 0 {
			t.Fatalf("标了 stdout_bytes_unpinned、rc 对 ⇒ 应 rc=0，得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("UnpinnedAbsent_Rc1", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctUnpinnedFixture(t, false, 0.0), "--verify-live")
		rcNeg = rc
		if rc != 1 || !strings.Contains(out, "两次现跑不等") {
			t.Fatalf("拿掉旗标、stdout 两遍变 ⇒ 应 rc=1 且报「两次现跑不等」，得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("UnpinnedButRcWrong_Rc1", func(t *testing.T) {
		rc, out, _ := cctRun(t, cctUnpinnedFixture(t, true, 3.0), "--verify-live")
		if rc == 0 || !strings.Contains(out, "退码") {
			t.Fatalf("标了旗标但 want_rc 错 ⇒ 仍应 rc≠0 且报退码差，得 rc=%d\n%s", rc, out)
		}
	})
	if rcPos == rcNeg {
		t.Fatalf("成对负控失效：正控 rc=%d 与负控 rc=%d 必须不等", rcPos, rcNeg)
	}
}

// cctChangingBin —— 每次现跑 stdout 字节数 +1 的假制品（用自身目录下的 `.n` 计数）。
const cctChangingBin = `#!/bin/sh
d=$(dirname "$0")
n=$(cat "$d/.n" 2>/dev/null || printf 0)
n=$((n+1))
printf '%s' "$n" > "$d/.n"
i=0
while [ "$i" -lt "$n" ]; do printf x; i=$((i+1)); done
exit 0
`

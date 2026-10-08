// swap_ui_test.go —— A3「桌面 UI 换件」正门（`zerg swap ui` · `cmdSwapUI`）**真搬**后的回归牙。
//
// 为什么这么测（照 M2 口径 · 与 `ui_start_test.go:103` 同型）：进程内直调 `cmdSwapUI`
// （不引真二进制、不依赖构建/服务/启动器），用**仓外合成夹具**给合成仓根，断言：
//
//	① **真搬不 exec 壳**：仓根里放一枚**自造壳**（打印 POISON / 退 42）⇒ 正门任何档 stdout/stderr 出现
//	   POISON 即证明实现仍在 exec 脚本（壳转账），本用例当红；
//	② **两态三面逐字同**：**自造壳**在盘 ⟷ 移出仓（退役态），正门 rc/stdout/stderr 三面逐字同，
//	   且退役态**不得**退 8/2（旧壳转账正是靠 `os.Stat(脚本)` 退 8）；
//	③ **金标内化**：`scripts/build/zerg-swap-ui.sh` 已随真搬退场 ⇒ 弃「脚本直跑」对拍源，把确定性档
//	   （`--help`/`-h` 的 usage ／ `--swap` 未开放 ／ 未知参数 ／ `--check` 报告）收回 **Go 真源**
//	   （`swuiUsage()` ＋ 冻常量），现跑 `cmdSwapUI` 自证；非确定面（夹具根 / 时刻戳）按 `suTNorm` 归一
//	   （✗ 不再读任何脚本 · ✗ 不放宽为「不检 / 只判非空」）；
//	④ **D3 四态**：裸跑 rc=2 零写 · `--dry-run` rc=0 零写（出计划件）· `--yes` rc=0（真跑档走
//	   **函数级接缝** ⇒ ✗ 单测绝不真跑六闸）· `--dry-run --yes` rc=2。
//
// 哑化负控（"牙"）：把 `cmdSwapUI` 改回「exec 脚本」或「去授权门 / 一律 return 0 / 吞 stdout」⇒ 上面
// ①②③④ 当场 FAIL（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// ★ 安全线（换件面 · 与 A1/A2 同）：本测件**绝不**真跑六闸（报告恒走**合成仓根** · 真跑档走接缝）；
// ✗ 不动 `bin/` · ✗ 不带 `--swap` 去真换件（本件本就零副作用，且真换件档未开放）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// suTSelfShell —— **自造壳**（非真脚本；真脚本已随真搬退场）。若正门仍在 exec 脚本，本壳会被拉起 ⇒
// 出现 POISON / rc=42 —— 正是「壳在盘 ⟷ 壳缺席 ⇒ 三面逐字同」要证明的反面。
const suTSelfShell = "#!/usr/bin/env bash\necho POISON-SHELL-RAN\necho POISON-SHELL-RAN >&2\nexit 42\n"

// 以下三枚为 **Go 真源冻金标**（脚本退场后收回命令面本身）。
const suTSwapWantErr = "✗ zerg-swap-ui.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）\n  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh 的 UI 档 → 验签 → 重启 → 自证 → 回滚路径。\n"
const suTUnknownArgWantErr = "✗ 未知参数: --nope-not-a-flag（用法见：bash scripts/build/zerg-swap-ui.sh --help）\n"
const suTCheckWantOut = "== UI 换件闸（只报告 · 只读）<TS> ==\n日志：<FIX>/swui-report.log\n目标件：<FIX>/bin/zerg-ui · 启动链：bin/zerg\n──────────────────────────────────────────────\n[闸A] 工作树半成品（换件是从工作树编译的）\n  [闸] A 工作树半成品 ⇒ PASS —— 未提交改动数 = 0\n──────────────────────────────────────────────\n[闸B] 启动链可用（A6 真源：正门 zerg ui start → 启动器在位 ∧ 它 exec 的件在位）\n  [闸] B 启动链 ⇒ FAIL —— 启动器不存在（bin/zerg）—— 换上去也起不来\n──────────────────────────────────────────────\n[闸①] 构建口径（件必须由 build-all.sh 产出 · 身份不许丢）\n  [闸] ① 构建口径 ⇒ BLOCKED —— 缺 bin/build-info.json（读不到构件身份 ⇒ 判不了，不许当绿）\n──────────────────────────────────────────────\n[闸②] 验签（codesign --verify —— 输出原样打印，不吞）\n  [闸] ② 验签 ⇒ BLOCKED —— 件不存在（<FIX>/bin/zerg-ui）—— 判不了\n──────────────────────────────────────────────\n[闸③] 回滚位（回滚件必须**在位且验签通过**）\n  [闸] ③ 回滚位 ⇒ BLOCKED —— 无回滚件（bin/_history/zerg-ui.prev）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）\n──────────────────────────────────────────────\n[闸④] 换件后自证判据可跑（UI 无 HTTP 端点 ⇒ 进程活着 ∧ 跑的件 == 盘上件）\n  [闸] ④ 自证判据 ⇒ BLOCKED —— 件不存在 ⇒ 无从对拍——判不了\n──────────────────────────────────────────────\n状态计数：PASS 1 · FAIL 1 · BLOCKED(不给结论) 4\n日志：<FIX>/swui-report.log\n⇒ 有闸判 FAIL（rc=1）—— 逐条见上\n"

// suTSRe / suTNorm —— 归一化非确定面：夹具根→<FIX> · 逐秒时刻戳→<TS>。
var suTSRe = regexp.MustCompile(`[0-9]{8}T[0-9]{6}`)

func suTNorm(root, s string) string {
	s = strings.ReplaceAll(s, root, "<FIX>")
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		s = strings.ReplaceAll(s, real, "<FIX>")
	}
	return suTSRe.ReplaceAllString(s, "<TS>")
}

// suSynth 建最小合成仓根（仓根判据件 ＋ 可选的 `zerg-swap-ui.sh` 正文），返回其路径（已解符号链接 ⇒ 与脚本侧同形）。
func suSynth(t *testing.T, shellBody string) string {
	t.Helper()
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if shellBody != "" {
		p := filepath.Join(root, swapUIScriptRel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(shellBody), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// suRunCmd 进程内直调 `cmdSwapUI`（给合成仓根）⇒ rc / stdout / stderr。
// 授权旗标（`--dry-run`/`--yes`）手工补到解析字段上（进程内直调不经 `parseInvocation`）。
func suRunCmd(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"swap", "ui"},
		orig: append([]string{"swap", "ui"}, tail...),
	}
	for _, a := range tail {
		switch a {
		case "--yes":
			inv.yes = true
		case "--dry-run":
			inv.dryRun = true
		}
	}
	rc := cmdSwapUI(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ①＋② 真搬不 exec 壳 ＋ 确定性档两态三面逐字同（★ 退役态不得退 8/2）。
// ★ 内化（体例同 `ui_start_test.go:103`）：弃 `suRealShell`（真脚本已退场），改用**自造壳**。
func TestSwapUIHelpNotExecShellRetiredStateByteIdentical(t *testing.T) {
	root := suSynth(t, suTSelfShell)
	src := filepath.Join(root, swapUIScriptRel)

	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
	} {
		rc1, out1, err1 := suRunCmd(t, root, tc.tail...)
		if strings.Contains(out1, "POISON") || strings.Contains(err1, "POISON") {
			t.Fatalf("[%s] 实现仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", tc.name, out1, err1)
		}
		if rc1 != exitOK || err1 != "" || out1 != swuiUsage() {
			t.Fatalf("[%s] 态①（壳在盘）期望 rc=%d · stdout 逐字=swuiUsage() · stderr 空；实得 rc=%d out=%q err=%q",
				tc.name, exitOK, rc1, out1, err1)
		}
		// 退役态：移出自造壳 ⇒ 三面必须逐字同（✗ 不得退 8/2）。
		if err := os.Remove(src); err != nil {
			t.Fatal(err)
		}
		rc2, out2, err2 := suRunCmd(t, root, tc.tail...)
		if rc2 == exitBlocked || rc2 == exitUsage {
			t.Fatalf("[%s] 退役态**不得**退 8/2：rc=%d", tc.name, rc2)
		}
		if rc1 != rc2 || out1 != out2 || err1 != err2 {
			t.Fatalf("[%s] 两态三面不逐字同：态① %d/%q/%q · 态② %d/%q/%q", tc.name, rc1, out1, err1, rc2, out2, err2)
		}
		if err := os.WriteFile(src, []byte(suTSelfShell), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// ③ `--swap`（未开放档）与未知参数：rc/stdout/stderr 与**冻结金标**逐字同（Go 真源自证 · 脚本已退场）；
// 两态（自造壳在盘 ⟷ 移出仓）三面逐字同（✗ 不得退 8）。
func TestSwapUISwapAndUnknownTwoStatesVerbatim(t *testing.T) {
	root := suSynth(t, suTSelfShell)
	src := filepath.Join(root, swapUIScriptRel)
	rc1, o1, e1 := suRunCmd(t, root, "--yes", "--swap")
	if rc1 != exitUsage || o1 != "" || e1 != suTSwapWantErr {
		t.Fatalf("`--swap` 档与冻结金标不符：rc=%d（要 %d）· stdout=%q · stderr=%q（要 %q）",
			rc1, exitUsage, o1, e1, suTSwapWantErr)
	}
	rc2, o2, e2 := suRunCmd(t, root, "--yes", "--nope-not-a-flag")
	if rc2 != exitUsage || o2 != "" || e2 != suTUnknownArgWantErr {
		t.Fatalf("未知参数档与冻结金标不符：rc=%d（要 %d）· stdout=%q · stderr=%q（要 %q）",
			rc2, exitUsage, o2, e2, suTUnknownArgWantErr)
	}
	// 退役态：自造壳移出仓 ⇒ 三面逐字同（✗ 不得退 8）。
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc3, o3, e3 := suRunCmd(t, root, "--yes", "--swap")
	if rc3 == exitBlocked {
		t.Fatalf("退役态 `--swap` **不得**退 8：rc=%d", rc3)
	}
	if rc1 != rc3 || o1 != o3 || e1 != e3 {
		t.Fatalf("`--swap` 两态三面不逐字同：%d/%q/%q · %d/%q/%q", rc1, o1, e1, rc3, o3, e3)
	}
	rc4, o4, e4 := suRunCmd(t, root, "--yes", "--nope-not-a-flag")
	if rc4 == exitBlocked {
		t.Fatalf("退役态未知参 **不得**退 8：rc=%d", rc4)
	}
	if rc2 != rc4 || o2 != o4 || e2 != e4 {
		t.Fatalf("未知参两态三面不逐字同：%d/%q/%q · %d/%q/%q", rc2, o2, e2, rc4, o4, e4)
	}
}

// ③ `--check` 报告（只读档 ⟷ 裸跑档经授权门 —— 现读 `cmdSwapUI` 二者同路）：rc/stdout/stderr 与
// **冻结金标**逐字同（Go 真源自证 · 脚本已退场；非确定面按 `suTNorm` 归一）；两态（自造壳在盘 ⟷ 移出仓）逐字同。
// ★ 等效覆盖说明：报告正文「能跑起来 ＋ 无 POISON」另有 `TestSwapUICheckDoesNotExecShell` 把守；
//
//	本格补的是它**不覆盖**的三样 —— **rc 冻值 ＋ 金标正文 ＋ 退役态不变**（故不删格）。
func TestSwapUICheckReportTwoStatesVerbatim(t *testing.T) {
	root := suSynth(t, suTSelfShell)
	t.Setenv("ZERG_SWAP_BIN", filepath.Join(root, "bin", "zerg-ui"))
	t.Setenv("ZERG_SWAP_PLIST", filepath.Join(root, "com.zerg.ui.plist"))
	t.Setenv("ZERG_SWAP_LOG", filepath.Join(root, "swui-report.log"))
	src := filepath.Join(root, swapUIScriptRel)

	rc1, o1, e1 := suRunCmd(t, root, "--check")
	if rc1 != exitFail || e1 != "" || suTNorm(root, o1) != suTCheckWantOut {
		t.Fatalf("`--check` 报告与冻结金标不符：rc=%d（要 %d）· stderr=%q（要空）·\n实得(归一)=%q\n冻值=%q",
			rc1, exitFail, e1, suTNorm(root, o1), suTCheckWantOut)
	}
	// 裸跑档经授权门（`--yes`）：与 `--check` 同路 ⇒ 三面逐字同。
	rc2, o2, e2 := suRunCmd(t, root, "--yes")
	if rc1 != rc2 || o1 != o2 || e1 != e2 {
		t.Fatalf("裸跑(`--yes`)与 `--check` 不逐字同：%d/%q/%q · %d/%q/%q", rc1, o1, e1, rc2, o2, e2)
	}
	// 退役态：自造壳移出仓 ⇒ 三面逐字同（✗ 不得退 8/2）。
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc3, o3, e3 := suRunCmd(t, root, "--check")
	if rc3 == exitBlocked || rc3 == exitUsage {
		t.Fatalf("退役态**不得**退 8/2：rc=%d", rc3)
	}
	if rc1 != rc3 || o1 != o3 || e1 != e3 {
		t.Fatalf("退役态与在盘态不逐字同：%d/%dB/%q · %d/%dB/%q", rc1, len(o1), e1, rc3, len(o3), e3)
	}
}

// ⑤ 报告档不 exec 脚本（毒壳实证）：`--check` 的正文里不得出现 POISON。
func TestSwapUICheckDoesNotExecShell(t *testing.T) {
	root := suSynth(t, "#!/usr/bin/env bash\necho POISON-SHELL-RAN\necho POISON-SHELL-RAN >&2\nexit 42\n")
	t.Setenv("ZERG_SWAP_LOG", filepath.Join(root, "swui-poison.log"))
	_, out, errb := suRunCmd(t, root, "--check")
	if strings.Contains(out, "POISON") || strings.Contains(errb, "POISON") {
		t.Fatalf("报告档仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", out, errb)
	}
	if !strings.Contains(out, "[闸A]") || !strings.Contains(out, "状态计数：") {
		t.Fatalf("报告正文没跑起来（缺闸A/状态计数）：%q", out)
	}
}

// ⑥ D3 四态：裸跑 2 零写 · --dry-run 0 零写 · --yes 0（接缝）· --dry-run --yes 2。
func TestSwapUIFourStatesGuard(t *testing.T) {
	root := suSynth(t, "#!/usr/bin/env bash\nexit 0\n")
	logf := filepath.Join(root, "swui-four.log")
	t.Setenv("ZERG_SWAP_LOG", logf)
	absent := func() bool { _, err := os.Stat(logf); return err != nil }

	// 裸跑 ⇒ rc=2（授权门）· stdout 0 字节 · 零写。
	rc, out, errb := suRunCmd(t, root)
	if rc != exitUsage || out != "" {
		t.Fatalf("裸跑：rc=%d out=%q（要 rc=2 · stdout 0 字节）", rc, out)
	}
	if !strings.Contains(errb, "D3 档") {
		t.Fatalf("裸跑缺 D3 判词：%q", errb)
	}
	if !absent() {
		t.Fatalf("裸跑**有写**（日志被建）：%s —— 要零写", logf)
	}

	// --dry-run ⇒ rc=0（计划件）· 零写。
	rc, out, _ = suRunCmd(t, root, "--dry-run")
	if rc != exitOK || !strings.Contains(out, "计划件") {
		t.Fatalf("--dry-run：rc=%d out=%.40q（要 rc=0 · 出计划件）", rc, out)
	}
	if !absent() {
		t.Fatalf("--dry-run **有写**（日志被建）—— 要零写")
	}

	// --dry-run --yes ⇒ rc=2（两道确认档自相矛盾）· stdout 空 · 零写。
	rc, out, _ = suRunCmd(t, root, "--dry-run", "--yes")
	if rc != exitUsage || out != "" {
		t.Fatalf("--dry-run --yes：rc=%d out=%q（要 rc=2 · stdout 空）", rc, out)
	}
	if !absent() {
		t.Fatalf("--dry-run --yes **有写**（日志被建）—— 要零写")
	}

	// --yes ⇒ rc=0：**真跑档一律走函数级接缝**（✗ 单测绝不真跑六闸）；证「授权门放行 --yes 且报告被进入」。
	old := swuiReportFn
	called := false
	swuiReportFn = func(c *swuiCtx) int { called = true; return exitOK }
	defer func() { swuiReportFn = old }()
	rc, _, _ = suRunCmd(t, root, "--yes")
	if rc != exitOK || !called {
		t.Fatalf("--yes：rc=%d called=%v（要 rc=0 且进入报告 —— 授权门放行 --yes）", rc, called)
	}
}

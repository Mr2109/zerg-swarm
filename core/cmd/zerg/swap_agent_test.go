// swap_agent_test.go —— A2「子端换件」正门（`zerg swap agent` · `cmdSwapAgent`）**真搬**后的回归牙。
//
// 为什么这么测（照 M2 口径 · 与 `ui_start_test.go:103` 同型）：进程内直调 `cmdSwapAgent`
// （不引真二进制、不依赖构建/服务/launchd），用**仓外合成夹具**给合成仓根，断言：
//
//	① **真搬不 exec 壳**：仓根里放一枚**自造壳**（打印 POISON / 退 42）⇒ 正门任何档 stderr 出现 POISON
//	   即证明实现仍在 exec 脚本（壳转账），本用例当红；
//	② **两态三面逐字同**：**自造壳**在盘 ⟷ 移出仓（退役态），正门 rc/stdout/stderr 三面逐字同，
//	   且退役态**不得**退 8/2（旧壳转账正是靠 `os.Stat(脚本)` 退 8）；
//	③ **金标内化**：`scripts/build/zerg-swap-agent.sh` 已随真搬退场 ⇒ 弃「脚本直跑」对拍源，把确定性档
//	   （`--help`/`-h` 的 usage ／ 未知参数 ／ `--swap` 未开放 ／ `--check` 报告 ／ `--self-test`）收回
//	   **Go 真源**（`sagUsage()` ＋ 冻常量），现跑 `cmdSwapAgent` 自证；非确定面（夹具根 / 时刻戳 /
//	   uid / 运行期机器名）按 `sagTNorm` 归一（✗ 不再读任何脚本 · ✗ 不放宽为「不检 / 只判非空」）；
//	④ **D3 四态**：裸跑 rc=2 零写 · `--dry-run` rc=0 零写 · `--yes` rc=0（真跑档走**函数级接缝** ⇒
//	   ✗ 单测绝不真跑判据）· `--dry-run --yes` rc=2。
//
// 哑化负控（"牙"）：把 `cmdSwapAgent` 改回「exec 脚本」或「去授权门 / 一律 return 0 / 吞 stdout」⇒ 上面
// ①②③④ 当场 FAIL（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sagTSelfShell —— **自造壳**（非真脚本；真脚本已随真搬退场）。若正门仍在 exec 脚本，本壳会被拉起 ⇒
// 出现 POISON / rc=42 —— 正是「壳在盘 ⟷ 壳缺席 ⇒ 三面逐字同」要证明的反面。
const sagTSelfShell = "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n"

// 以下四枚为 **Go 真源冻金标**（脚本退场后收回命令面本身）。
const sagTUnknownArgWantErr = "✗ 未知参数: --nope-not-a-flag（用法见：bash scripts/build/zerg-swap-agent.sh --help）\n"
const sagTSwapWantErr = "✗ zerg-swap-agent.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）\n  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh --only-agentd → 验签 → 重启 → 自证 → 回滚路径。\n"
const sagTCheckWantOut = "== 子端换件闸（只报告 · 只读）<TS> ==\n日志：<FIX>/sag-log.txt\n目标件：<FIX>/bin/zerg-agentd · 作业：com.zerg.agent-selftest\n──────────────────────────────────────────────\n[闸A] 工作树半成品（换件是从工作树编译的）\n  [闸] A 工作树半成品 ⇒ PASS —— 未提交改动数 = 0\n──────────────────────────────────────────────\n[闸B] 服务定义可用（盘上 plist 引用的程序必须真存在）\n      作业：gui/<UID>/com.zerg.agent-selftest · plist：<FIX>/com.zerg.agent.plist\n  [闸] B 服务定义 ⇒ BLOCKED —— 盘上 plist 不存在（<FIX>/com.zerg.agent.plist）—— 判不了，不许当绿\n──────────────────────────────────────────────\n[闸①] 构建口径（件必须由 build-all.sh 产出 · 身份不许丢）\n  [闸] ① 构建口径 ⇒ BLOCKED —— 缺 bin/build-info.json（读不到构件身份 ⇒ 判不了，不许当绿）\n──────────────────────────────────────────────\n[闸②] 验签（取数档 = codesign -dvvv ⇒ Signature= / flags= / cdhash 三格同档；输出原样打印，不吞）\n  [闸] ② 验签 ⇒ BLOCKED —— 件不存在（<FIX>/bin/zerg-agentd）—— 判不了\n──────────────────────────────────────────────\n[闸③] 回滚位（回滚件必须**在位且验签通过** —— 「退得回去」得先证明）\n  [闸] ③ 回滚位 ⇒ BLOCKED —— 无回滚件（bin/_history/zerg-agentd.prev）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）\n──────────────────────────────────────────────\n[闸④] 换件后自证判据可跑（只读探针 —— 判据活着才谈得上自证）\n      /ready=000000 · /status=000000\n  [闸] ④ 自证判据 ⇒ FAIL —— 探针异常（/ready=000000 · /status=000000；401 = 令牌不对）\n──────────────────────────────────────────────\n状态计数：PASS 1 · FAIL 1 · BLOCKED(不给结论) 4\n日志：<FIX>/sag-log.txt\n⇒ 有闸判 FAIL（rc=1）—— 逐条见上\n"
const sagTSelfTestWantOut = "== zerg-swap-agent.sh 自检（--self-test · 不碰真件/服务/launchd）<TS> ==\n  [自检] T1 留档落点合规（bin/_history/） ⇒ PASS（rc=0）\n  [自检] T2 落点平铺判红 ⇒ PASS（rc=1）\n  [自检] T3 件不存在⇒不给结论 ⇒ PASS（rc=2）\n  [自检] T4 adhoc 件判红 ⇒ PASS（rc=1）\n  [自检] T5 真签件放行 ⇒ PASS（rc=0）\n  [自检] T6 默认作业名 = <LABEL> ⇒ PASS（rc=0）\n  [自检] T7 --machine demo1 推导作业名 ⇒ PASS（rc=0）\n  [自检] T8 --label 覆盖 --machine ⇒ PASS（rc=0）\n  [自检] T9 环境 label 优先于 --machine ⇒ PASS（rc=0）\n自检：9 过 / 0 红（共 9 条）\n"

// sagTTSRe / sagTUIDRe / sagTNorm —— 归一化非确定面：夹具根→<FIX> · 逐秒时刻戳→<TS> ·
// 运行期 uid（`gui/<uid>`）→<UID> · 运行期机器名（`sagDefLabel`）→<LABEL>。
var sagTTSRe = regexp.MustCompile(`[0-9]{8}T[0-9]{6}`)
var sagTUIDRe = regexp.MustCompile(`gui/[0-9]+`)

func sagTNorm(root, s string) string {
	s = strings.ReplaceAll(s, root, "<FIX>")
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		s = strings.ReplaceAll(s, real, "<FIX>")
	}
	s = sagTTSRe.ReplaceAllString(s, "<TS>")
	s = sagTUIDRe.ReplaceAllString(s, "gui/<UID>")
	return strings.ReplaceAll(s, sagDefLabel, "<LABEL>")
}

// sagTSynthRepo 建最小合成仓根（仓根判据件 ＋ 可选的「毒壳」/真脚本正文），返回其路径。
func sagTSynthRepo(t *testing.T, shellBody string) string {
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
		p := filepath.Join(root, swapAgentScriptRel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(shellBody), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// sagTRunCmd 进程内直调 `cmdSwapAgent`（给合成仓根）⇒ rc / stdout / stderr。
// 授权旗标（`--dry-run`/`--yes`）手工补到解析字段上（进程内直调不经 `parseInvocation`）。
func sagTRunCmd(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"swap", "agent"},
		orig: append([]string{"swap", "agent"}, tail...),
	}
	for _, a := range tail {
		switch a {
		case "--yes":
			inv.yes = true
		case "--dry-run":
			inv.dryRun = true
		}
	}
	rc := cmdSwapAgent(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ①＋② 真搬不 exec 壳 ＋ 退役态两态三面逐字同（★ 退役态不得退 8/2）。
// ★ 内化（体例同 `ui_start_test.go:103`）：弃 `sagTRealShell`（真脚本已退场），改用**自造壳**。
func TestSwapAgentNotExecShellRetiredStateByteIdentical(t *testing.T) {
	root := sagTSynthRepo(t, sagTSelfShell)
	src := filepath.Join(root, swapAgentScriptRel)

	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"h", []string{"-h"}},
	} {
		rc1, out1, err1 := sagTRunCmd(t, root, tc.tail...)
		if strings.Contains(out1, "POISON") || strings.Contains(err1, "POISON") {
			t.Fatalf("[%s] 实现仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", tc.name, out1, err1)
		}
		if rc1 != exitOK || err1 != "" || out1 != sagUsage() {
			t.Fatalf("[%s] 态①（壳在盘）期望 rc=%d · stdout 逐字=sagUsage() · stderr 空；实得 rc=%d out=%q err=%q",
				tc.name, exitOK, rc1, out1, err1)
		}
		// 退役态：移出自造壳 ⇒ 三面必须逐字同（✗ 不得退 8/2）。
		if err := os.Remove(src); err != nil {
			t.Fatal(err)
		}
		rc2, out2, err2 := sagTRunCmd(t, root, tc.tail...)
		if rc2 == exitBlocked || rc2 == exitUsage {
			t.Fatalf("[%s] 退役态**不得**退 8/2：rc=%d", tc.name, rc2)
		}
		if rc1 != rc2 || out1 != out2 || err1 != err2 {
			t.Fatalf("[%s] 两态三面不逐字同：态① %d/%q/%q · 态② %d/%q/%q", tc.name, rc1, out1, err1, rc2, out2, err2)
		}
		if err := os.WriteFile(src, []byte(sagTSelfShell), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// ③ 未知参数 与 `--swap`（真换件档未开放）：rc/stdout/stderr 与**冻结金标**逐字同（Go 真源自证 · 脚本已退场）；
// 两态（自造壳在盘 ⟷ 移出仓）三面逐字同（✗ 不得退 8）。
func TestSwapAgentUnknownArgTwoStatesVerbatim(t *testing.T) {
	root := sagTSynthRepo(t, sagTSelfShell)
	src := filepath.Join(root, swapAgentScriptRel)
	rc1, out1, err1 := sagTRunCmd(t, root, "--yes", "--nope-not-a-flag")
	if rc1 != exitUsage || out1 != "" || err1 != sagTUnknownArgWantErr {
		t.Fatalf("未知参数档与冻结金标不符：rc=%d（要 %d）· stdout=%q · stderr=%q（要 %q）",
			rc1, exitUsage, out1, err1, sagTUnknownArgWantErr)
	}
	rc2, out2, err2 := sagTRunCmd(t, root, "--yes", "--swap")
	if rc2 != exitUsage || out2 != "" || err2 != sagTSwapWantErr {
		t.Fatalf("`--swap` 档与冻结金标不符：rc=%d（要 %d）· stdout=%q · stderr=%q（要 %q）",
			rc2, exitUsage, out2, err2, sagTSwapWantErr)
	}
	// 退役态：自造壳移出仓 ⇒ 三面逐字同（✗ 不得退 8）。
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc3, out3, err3 := sagTRunCmd(t, root, "--yes", "--nope-not-a-flag")
	if rc3 == exitBlocked {
		t.Fatalf("退役态**不得**退 8（旧壳转账 os.Stat(脚本) 的病）：rc=%d", rc3)
	}
	if rc1 != rc3 || out1 != out3 || err1 != err3 {
		t.Fatalf("未知参数两态三面不逐字同：%d/%q/%q · %d/%q/%q", rc1, out1, err1, rc3, out3, err3)
	}
	rc4, out4, err4 := sagTRunCmd(t, root, "--yes", "--swap")
	if rc2 != rc4 || out2 != out4 || err2 != err4 {
		t.Fatalf("`--swap` 两态三面不逐字同：%d/%q/%q · %d/%q/%q", rc2, out2, err2, rc4, out4, err4)
	}
}

// ③ `--check` 报告 与 `--self-test`：rc/stdout/stderr 与**冻结金标**逐字同（Go 真源自证 · 脚本已退场；
// 非确定面按 `sagTNorm` 归一）；两态（自造壳在盘 ⟷ 移出仓）三面逐字同（✗ 不得退 8/2）。
// ★ 受控合成仓根 ＋ 夹具 env ⇒ 六闸读数确定（闸A PASS · 闸B/①/②/③ BLOCKED · 闸④ 死端口 FAIL ⇒ rc=1）。
func TestSwapAgentReportAndSelfTestVerbatim(t *testing.T) {
	root := sagTSynthRepo(t, sagTSelfShell)
	t.Setenv("ZERG_SWAP_BIN", filepath.Join(root, "bin", "zerg-agentd"))
	t.Setenv("ZERG_SWAP_PLIST", filepath.Join(root, "com.zerg.agent.plist"))
	t.Setenv("ZERG_SWAP_LABEL", "com.zerg.agent-selftest")
	t.Setenv("ZERG_SWAP_READY_URL", "http://127.0.0.1:1/ready")
	t.Setenv("ZERG_SWAP_STATUS_URL", "http://127.0.0.1:1/status")
	t.Setenv("ZERG_SWAP_TOKEN_FILE", filepath.Join(root, "nonexistent-token"))
	t.Setenv("ZERG_SWAP_LOG", filepath.Join(root, "sag-log.txt"))
	src := filepath.Join(root, swapAgentScriptRel)

	rc1, out1, err1 := sagTRunCmd(t, root, "--check")
	if rc1 != exitFail || err1 != "" || sagTNorm(root, out1) != sagTCheckWantOut {
		t.Fatalf("`--check` 报告与冻结金标不符：rc=%d（要 %d）· stderr=%q（要空）·\n实得(归一)=%q\n冻值=%q",
			rc1, exitFail, err1, sagTNorm(root, out1), sagTCheckWantOut)
	}
	rc2, out2, err2 := sagTRunCmd(t, root, "--self-test")
	if rc2 != exitOK || err2 != "" || sagTNorm(root, out2) != sagTSelfTestWantOut {
		t.Fatalf("`--self-test` 与冻结金标不符：rc=%d（要 %d）· stderr=%q（要空）·\n实得(归一)=%q\n冻值=%q",
			rc2, exitOK, err2, sagTNorm(root, out2), sagTSelfTestWantOut)
	}
	// 退役态：自造壳移出仓 ⇒ 两个档三面逐字同（✗ 不得退 8）。
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc3, out3, err3 := sagTRunCmd(t, root, "--check")
	if rc3 == exitBlocked {
		t.Fatalf("退役态 `--check` **不得**退 8：rc=%d", rc3)
	}
	if rc1 != rc3 || out1 != out3 || err1 != err3 {
		t.Fatalf("`--check` 退役态三面不逐字同：在盘 %d/%dB/%q · 退役 %d/%dB/%q",
			rc1, len(out1), err1, rc3, len(out3), err3)
	}
	rc4, out4, err4 := sagTRunCmd(t, root, "--self-test")
	if rc4 == exitBlocked {
		t.Fatalf("退役态 `--self-test` **不得**退 8：rc=%d", rc4)
	}
	if rc2 != rc4 || out2 != out4 || err2 != err4 {
		t.Fatalf("`--self-test` 退役态三面不逐字同：在盘 %d/%dB/%q · 退役 %d/%dB/%q",
			rc2, len(out2), err2, rc4, len(out4), err4)
	}
}

// ④ D3 四态：裸跑 2 零写 · --dry-run 0 零写 · --yes 0（接缝）· --dry-run --yes 2。
func TestSwapAgentFourStatesGuard(t *testing.T) {
	root := sagTSynthRepo(t, "#!/usr/bin/env bash\nexit 0\n")
	logFile := filepath.Join(root, "swaplog.txt")
	t.Setenv("ZERG_SWAP_LOG", logFile)

	// 裸跑 ⇒ rc=2（授权门）/ stdout 空 / 未写日志（零写）。
	rc, out, errb := sagTRunCmd(t, root)
	if rc != exitUsage || out != "" {
		t.Fatalf("裸跑：rc=%d out=%q（要 rc=2 · stdout 0 字节）", rc, out)
	}
	if !strings.Contains(errb, "D3 档") {
		t.Fatalf("裸跑缺 D3 判词：%q", errb)
	}
	if _, err := os.Stat(logFile); err == nil {
		t.Fatalf("裸跑**有写**（日志被建）：%s —— 要零写", logFile)
	}

	// --dry-run ⇒ rc=0（计划件）/ 零写。
	rc, out, _ = sagTRunCmd(t, root, "--dry-run")
	if rc != exitOK || !strings.Contains(out, "计划件") {
		t.Fatalf("--dry-run：rc=%d out=%.40q（要 rc=0 · 出计划件）", rc, out)
	}
	if _, err := os.Stat(logFile); err == nil {
		t.Fatalf("--dry-run **有写**（日志被建）—— 要零写")
	}

	// --dry-run --yes ⇒ rc=2（两道确认档自相矛盾）。
	rc, out, _ = sagTRunCmd(t, root, "--dry-run", "--yes")
	if rc != exitUsage || out != "" {
		t.Fatalf("--dry-run --yes：rc=%d out=%q（要 rc=2 · stdout 空）", rc, out)
	}

	// --yes ⇒ rc=0：**真跑档一律走函数级接缝**（✗ 单测绝不真跑判据）；证「授权门放行 --yes 且流程被进入」。
	old := sagFlowFn
	called := false
	sagFlowFn = func(c *sagCtx) int { called = true; return exitOK }
	defer func() { sagFlowFn = old }()
	rc, _, _ = sagTRunCmd(t, root, "--yes")
	if rc != exitOK || !called {
		t.Fatalf("--yes：rc=%d called=%v（要 rc=0 且进入流程 —— 授权门放行 --yes）", rc, called)
	}
}

// ⑤ 报告档不 exec 脚本（毒壳实证）：受控合成仓根里跑真报告，stderr 不得出现 POISON。
func TestSwapAgentFlowDoesNotExecShell(t *testing.T) {
	root := sagTSynthRepo(t, "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n")
	t.Setenv("ZERG_SWAP_BIN", filepath.Join(root, "nonexistent-bin"))
	t.Setenv("ZERG_SWAP_PLIST", filepath.Join(root, "nonexistent.plist"))
	t.Setenv("ZERG_SWAP_READY_URL", "http://127.0.0.1:1/ready")
	t.Setenv("ZERG_SWAP_STATUS_URL", "http://127.0.0.1:1/status")
	t.Setenv("ZERG_SWAP_TOKEN_FILE", filepath.Join(root, "nonexistent-token"))
	t.Setenv("ZERG_SWAP_LOG", filepath.Join(root, "swaplog.txt"))
	rc, out, errb := sagTRunCmd(t, root, "--yes")
	if strings.Contains(out, "POISON") || strings.Contains(errb, "POISON") {
		t.Fatalf("报告档仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", out, errb)
	}
	// 受控合成根：闸A PASS · 闸B/①/②/③ BLOCKED · 闸④ 死端口 FAIL ⇒ 有 FAIL ⇒ rc=1。
	if rc != exitFail {
		t.Fatalf("受控合成根里真报告应停在「闸④ 死端口判 FAIL ⇒ rc=1」，实得 rc=%d\nstdout=%q\nstderr=%q", rc, out, errb)
	}
}

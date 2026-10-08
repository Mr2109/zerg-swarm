// family_swap_core_test.go —— A1「主控换件」正门（`zerg swap core` · `cmdSwapCore`）**真搬**后的回归牙。
//
// 为什么这么测（照 M2 口径 · 与 `ui_start_test.go:103` 同型）：进程内直调 `cmdSwapCore`
// （不引真二进制、不依赖构建/服务/launchd），用**仓外合成夹具**给合成仓根，断言：
//
//	① **真搬不 exec 壳**：仓根里放一枚**自造壳**（打印 POISON / 退 42）⇒ 正门 stderr 出现 POISON
//	   即证明实现仍在 exec 脚本（壳转账），本用例当红；
//	② **两态三面逐字同**：**自造壳**在盘 ⟷ 移出仓（退役态），正门 rc/stdout/stderr 三面逐字同，
//	   且退役态**不得**退 8/2（旧壳转账正是靠 `os.Stat(脚本)` 退 8）；
//	③ **金标内化**：`scripts/build/zerg-swap-core.sh` 已随真搬退场 ⇒ 弃「脚本直跑」对拍源，把确定性档
//	   （`--help`/`-h` 的 usage ／ 未知参数的 stderr）收回 **Go 真源**（`swcUsage()` ＋ 冻常量
//	   `swcTUnknownArgWantErr`），现跑 `cmdSwapCore` 自证（✗ 不再读任何脚本 · ✗ 不放宽为「不检 / 只判非空」）；
//	④ **D3 四态**：裸跑 rc=2 零写 · `--dry-run` rc=0 零写 · `--yes` rc=0（真跑档走**函数级接缝** ⇒
//	   ✗ 单测绝不真跑换件）· `--dry-run --yes` rc=2。
//
// 哑化负控（"牙"）：把 `cmdSwapCore` 改回「exec 脚本」或「去授权门 / 一律 return 0 / 吞 stdout」⇒ 上面
// ①②③④ 当场 FAIL（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// ★ 安全线（事故面）：本测件**绝不**真跑 `scripts/build/build-all.sh`（真编生产件）· ✗ 不真换件 ·
// ✗ 不动 `bin/`；真跑档那一格一律走 `swcFlowFn` 接缝；流程档只在**受控合成仓根**（ZERG_SWAP_* 全指夹具）里跑。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// swcTSelfShell —— **自造壳**（非真脚本；真脚本已随真搬退场）。若正门仍在 exec 脚本，本壳会被拉起 ⇒
// 出现 POISON / rc=42 —— 正是「壳在盘 ⟷ 壳缺席 ⇒ 三面逐字同」要证明的反面。
const swcTSelfShell = "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n"

// swcTUnknownArgWantErr —— 未知参数档的 stderr **冻结金标**（Go 真源 `cmdSwapCore` 的 default 支）。
const swcTUnknownArgWantErr = "✗ 未知参数: --no-such-flag（用法见：bash scripts/build/zerg-swap-core.sh --help）\n"

// swcTSynthRepo 建最小合成仓根（仓根判据件 ＋ 可选的「毒壳」），返回其路径（已解符号链接 ⇒ 与脚本侧 `pwd` 同形）。
func swcTSynthRepo(t *testing.T, shellBody string) string {
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
		p := filepath.Join(root, swapCoreScriptRel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(shellBody), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// swcTRunCmd 进程内直调 `cmdSwapCore`（给合成仓根）⇒ rc / stdout / stderr。
// 授权旗标（`--dry-run`/`--yes`）手工补到解析字段上（进程内直调不经 `parseInvocation`）。
func swcTRunCmd(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"swap", "core"},
		orig: append([]string{"swap", "core"}, tail...),
	}
	for _, a := range tail {
		switch a {
		case "--yes":
			inv.yes = true
		case "--dry-run":
			inv.dryRun = true
		}
	}
	rc := cmdSwapCore(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ①＋② 真搬不 exec 壳 ＋ 退役态两态三面逐字同（★ 退役态不得退 8/2）。
// ★ 内化（体例同 `ui_start_test.go:103`）：弃 `swcTRealShell`（真脚本已退场），改用**自造壳** ——
//
//	正门若不 exec 它，「壳在盘 ⟷ 壳缺席」读数与壳在不在无关 ⇒ 三面逐字同。
func TestSwapCoreNotExecShellRetiredStateByteIdentical(t *testing.T) {
	root := swcTSynthRepo(t, swcTSelfShell)
	src := filepath.Join(root, swapCoreScriptRel)

	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"h", []string{"-h"}},
	} {
		// 态①（自造壳在盘）：正门读数 = Go 真源 `swcUsage()`；出现 POISON 即证明仍在 exec 脚本。
		rc1, out1, err1 := swcTRunCmd(t, root, tc.tail...)
		if strings.Contains(out1, "POISON") || strings.Contains(err1, "POISON") {
			t.Fatalf("[%s] 实现仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", tc.name, out1, err1)
		}
		if rc1 != exitOK || err1 != "" || out1 != swcUsage() {
			t.Fatalf("[%s] 态①（壳在盘）期望 rc=%d · stdout 逐字=swcUsage() · stderr 空；实得 rc=%d out=%q err=%q",
				tc.name, exitOK, rc1, out1, err1)
		}
		// 退役态：移出自造壳 ⇒ 三面必须逐字同（✗ 不得退 8/2）。
		if err := os.Remove(src); err != nil {
			t.Fatal(err)
		}
		rc2, out2, err2 := swcTRunCmd(t, root, tc.tail...)
		if rc2 == exitBlocked || rc2 == exitUsage {
			t.Fatalf("[%s] 退役态**不得**退 8/2（旧壳转账靠 os.Stat(脚本) 的病）：rc=%d", tc.name, rc2)
		}
		if rc1 != rc2 || out1 != out2 || err1 != err2 {
			t.Fatalf("[%s] 两态三面不逐字同：\n态① rc=%d out=%q err=%q\n态② rc=%d out=%q err=%q",
				tc.name, rc1, out1, err1, rc2, out2, err2)
		}
		// 还原供下一格。
		if err := os.WriteFile(src, []byte(swcTSelfShell), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// ③ 未知参数：rc/stdout/stderr 与**冻结金标**逐字同（Go 真源自证 · 脚本已退场）；两态（自造壳在盘 ⟷ 移出仓）三面逐字同。
func TestSwapCoreUnknownArgTwoStatesVerbatim(t *testing.T) {
	root := swcTSynthRepo(t, swcTSelfShell)
	src := filepath.Join(root, swapCoreScriptRel)
	rc1, out1, err1 := swcTRunCmd(t, root, "--yes", "--no-such-flag")
	if rc1 != exitUsage || out1 != "" || err1 != swcTUnknownArgWantErr {
		t.Fatalf("未知参数档与冻结金标不符：rc=%d（要 %d）· stdout=%q（要空）· stderr=%q（要 %q）",
			rc1, exitUsage, out1, err1, swcTUnknownArgWantErr)
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc2, out2, err2 := swcTRunCmd(t, root, "--yes", "--no-such-flag")
	if rc2 == exitBlocked {
		t.Fatalf("退役态**不得**退 8（旧壳转账 os.Stat(脚本) 的病）：rc=%d", rc2)
	}
	if rc1 != rc2 || out1 != out2 || err1 != err2 {
		t.Fatalf("两态三面不逐字同：态① %d/%q/%q · 态② %d/%q/%q", rc1, out1, err1, rc2, out2, err2)
	}
}

// ④ D3 四态：裸跑 2 零写 · --dry-run 0 零写 · --yes 0（接缝）· --dry-run --yes 2。
func TestSwapCoreFourStatesGuard(t *testing.T) {
	root := swcTSynthRepo(t, "#!/usr/bin/env bash\nexit 0\n")
	logFile := filepath.Join(root, "swaplog.txt")

	// 裸跑 ⇒ rc=2（授权门）/ stdout 空 / 未写日志（零写）。
	t.Setenv("ZERG_SWAP_LOG", logFile)
	rc, out, errb := swcTRunCmd(t, root)
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
	rc, out, _ = swcTRunCmd(t, root, "--dry-run")
	if rc != exitOK || !strings.Contains(out, "计划件") {
		t.Fatalf("--dry-run：rc=%d out=%.40q（要 rc=0 · 出计划件）", rc, out)
	}
	if _, err := os.Stat(logFile); err == nil {
		t.Fatalf("--dry-run **有写**（日志被建）—— 要零写")
	}

	// --dry-run --yes ⇒ rc=2（两道确认档自相矛盾）。
	rc, out, _ = swcTRunCmd(t, root, "--dry-run", "--yes")
	if rc != exitUsage || out != "" {
		t.Fatalf("--dry-run --yes：rc=%d out=%q（要 rc=2 · stdout 空）", rc, out)
	}

	// --yes ⇒ rc=0：**真跑档一律走函数级接缝**（✗ 单测绝不真跑换件）；证「授权门放行 --yes 且流程被进入」。
	old := swcFlowFn
	called := false
	swcFlowFn = func(c *swcCtx) int { called = true; return exitOK }
	defer func() { swcFlowFn = old }()
	rc, _, _ = swcTRunCmd(t, root, "--yes")
	if rc != exitOK || !called {
		t.Fatalf("--yes：rc=%d called=%v（要 rc=0 且进入流程 —— 授权门放行 --yes）", rc, called)
	}
}

// ⑤ 流程档不 exec 脚本（毒壳实证）：受控合成仓根里跑真流程，stderr 不得出现 POISON。
// ★ 真流程会真跑 git/cp/curl（只读或本地），故把四个可覆盖点全指夹具 ⇒ ✗ 不碰生产件/生产日志。
func TestSwapCoreFlowDoesNotExecShell(t *testing.T) {
	root := swcTSynthRepo(t, "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n")
	t.Setenv("ZERG_SWAP_BIN", filepath.Join(root, "nonexistent-bin"))
	t.Setenv("ZERG_SWAP_BACKUP", filepath.Join(root, "nonexistent-backup"))
	t.Setenv("ZERG_SWAP_LABEL", "com.zerg.selftest-nonexistent")
	t.Setenv("ZERG_SWAP_ENDPOINT", "http://127.0.0.1:1/api/capabilities")
	t.Setenv("ZERG_SWAP_LOG", filepath.Join(root, "swaplog.txt"))
	t.Setenv("TOKEN_FILE", filepath.Join(root, "nonexistent-token"))
	rc, out, errb := swcTRunCmd(t, root, "--yes")
	if strings.Contains(out, "POISON") || strings.Contains(errb, "POISON") {
		t.Fatalf("流程档仍在 exec 脚本（出现 POISON ⇒ 仍是壳转账）：%q %q", out, errb)
	}
	// 合成根里 bin 不在 ⇒ 备份失败那档（rc=1）；要点只是「没 exec 脚本」。
	if rc != exitFail {
		t.Fatalf("受控合成根里真流程应停在「备份失败 ⇒ rc=1」，实得 rc=%d\nstdout=%q\nstderr=%q", rc, out, errb)
	}
}

// truthsource_test.go —— 自更新真源收敛的**机检**（§7.1 `P12` · §3.3 `N2` · §十七 `SD12` ·
// 开工单 T-52 判据①）。
//
// 判据①逐字：三处（`core/internal/selfupdate` / `scripts/build/zerg-upgrade.sh` /
// `ui/src/modules/upgrade.rs`）**定一处为真源**、另两处变**调用方**。
// 本测试钉的是「哪一处是真源」+「另两处真的只是调用方」+「退码表只有一份」。
package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// TestSelfUpdateTruthSourceIsTheKernelScript —— 真源 = 那支六阶段脚本（不是 Go 包、也不是 Rust 模块）。
func TestSelfUpdateTruthSourceIsTheKernelScript(t *testing.T) {
	spec, err := contract.SelfUpdate()
	if err != nil {
		t.Fatalf("自更新真源读不出来：%v", err)
	}
	if spec.TruthSource["kernel"] != "scripts/build/zerg-upgrade.sh" {
		t.Errorf("真源内核 = %q（要 scripts/build/zerg-upgrade.sh —— SD12 逐字「共用同一个换件内核」）",
			spec.TruthSource["kernel"])
	}
	want := []string{"plan", "drain", "swap", "restart", "verify", "report"}
	if strings.Join(spec.Stages, "|") != strings.Join(want, "|") {
		t.Errorf("六阶段 = %v（要 %v）", spec.Stages, want)
	}
	// 两处调用方都要在真源里登记（就是判据①的「另两处变调用方」）。
	joined := ""
	for _, c := range spec.Callers {
		if f, ok := c["file"].(string); ok {
			joined += f + " "
			if role, _ := c["role"].(string); role != "调用方" {
				t.Errorf("调用方 %s 的角色 = %q（要「调用方」）", f, role)
			}
		}
	}
	for _, want := range []string{"core/internal/selfupdate", "ui/src/modules/upgrade.rs"} {
		if !strings.Contains(joined, want) {
			t.Errorf("真源里没有登记调用方 %q（现有：%s）", want, joined)
		}
	}
}

// TestSelfUpdateExitCodesMatchTruthSource —— 退码表**只有一份**：本包常量逐格对拍真源。
func TestSelfUpdateExitCodesMatchTruthSource(t *testing.T) {
	if err := ExitCodesMatchTruthSource(); err != nil {
		t.Fatalf("退码表对不上真源：%v", err)
	}
	// ★ 本票修的那处真病：`4` 只有一个意思（校验不通过）；用法错是 `64`（sysexits EX_USAGE）。
	if ExitVerifyFailed != 4 {
		t.Errorf("ExitVerifyFailed = %d（要 4 —— 内核脚本的 4 就是「校验不通过」）", ExitVerifyFailed)
	}
	if ExitUsage == 4 {
		t.Error("ExitUsage 还是 4 —— 那正是「同数字两义」的病根（脚本的 4 是校验不通过）")
	}
	if ExitUsage != 64 {
		t.Errorf("ExitUsage = %d（要 64 —— 脚本 :125 逐字 exit 64）", ExitUsage)
	}
	// 负控（成对）：把任何一个名字的值改掉 ⇒ 判定口必须报错，否则它就是恒绿装置。
	for name, wrong := range map[string]int{
		"ok": 9, "fail": 9, "no_update": 0, "refused": 0,
		"verify_failed": 4 + 1, "rollback_failed": 0, "usage": 4,
	} {
		good, err := contract.SelfUpdate()
		if err != nil {
			t.Fatal(err)
		}
		mine := map[string]int{"ok": ExitOK, "fail": ExitFail, "no_update": ExitNoUpdate,
			"refused": ExitRefused, "verify_failed": ExitVerifyFailed,
			"rollback_failed": ExitRollbackFailed, "usage": ExitUsage}
		mine[name] = wrong
		if len(good.ExitCodes) != len(mine) {
			t.Fatalf("负控装配失败：%d ≠ %d", len(good.ExitCodes), len(mine))
		}
		if err := exitCodesMatchTruthSource(mine); err == nil {
			t.Errorf("负控失败：把 %s 改成 %d 之后判定口没报错（恒绿装置不许当覆盖）", name, wrong)
		}
	}
	// 负控成对：少一格也要报错（缺格也是漂）。
	mine := map[string]int{"ok": ExitOK, "fail": ExitFail, "no_update": ExitNoUpdate,
		"refused": ExitRefused, "verify_failed": ExitVerifyFailed, "rollback_failed": ExitRollbackFailed}
	if err := exitCodesMatchTruthSource(mine); err == nil {
		t.Error("负控失败：抽掉一格（usage）之后判定口没报错")
	}
}

// TestTruthSourceKernelScriptExistsAndDeclaresTheSameCodes —— 「另两处是调用方」的**静态取证**：
// 脚本真的在盘上，且它的六阶段与退码（含 `exit 64`）逐条能读到。
func TestTruthSourceKernelScriptExistsAndDeclaresTheSameCodes(t *testing.T) {
	root := repoRootForTest(t)
	script := filepath.Join(root, "scripts", "build", "zerg-upgrade.sh")
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("内核脚本不在（%s）：%v", script, err)
	}
	src := string(body)
	if !strings.Contains(src, "plan → drain → swap → restart → verify → report") {
		t.Error("内核脚本里读不到六阶段那一行（真源不在它身上？）")
	}
	if !strings.Contains(src, "exit 64") {
		t.Error("内核脚本里读不到 `exit 64`（sysexits EX_USAGE —— 用法错那一格）")
	}
	if !strings.Contains(src, "ROLLBACK_FAILED") {
		t.Error("内核脚本里读不到 `ROLLBACK_FAILED`（回滚也失败那一格）")
	}
	// Rust 侧：同一支脚本（调用方，不自己实现六阶段）。
	rust := filepath.Join(root, "ui", "src", "modules", "upgrade.rs")
	rb, err := os.ReadFile(rust)
	if err != nil {
		t.Fatalf("UI 升级面不在：%v", err)
	}
	rs := string(rb)
	if !strings.Contains(rs, "zerg-upgrade.sh") {
		t.Error("UI 升级面没有引那支内核脚本（那不是「调用方」，那是第二份实现）")
	}
	for _, forbidden := range []string{"fn plan(", "fn drain(", "fn swap(", "fn restart("} {
		if strings.Contains(rs, forbidden) {
			t.Errorf("UI 侧自己实现了阶段 %q —— 判据①要的是「变调用方」", forbidden)
		}
	}
}

// repoRootForTest 从本测试文件往上找仓根（找 core/internal/version/version.go —— 与命令面同一口径）。
func repoRootForTest(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "internal", "version", "version.go")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatal("找不到仓根（读不到不当没有）")
	return ""
}

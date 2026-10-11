// family_build_agentd_open_test.go —— 「**子端换件档**（`build all --only agentd`）真跑本版开放」的落地判据（2026-10-10）。
//
// 口径（照 `family_build.go` 的三处判词同批改齐 · 与 `build release` 那笔同形）：
//
//	① 执行位 `openForExec`：`action == "all" && only == "agentd"` ⇒ 置真（此前缺这一行 ⇒ 真跑落 not_opened 退 2）；
//	② 计划件「**已开放**」判词：`--dry-run` 那一态把 agentd 单列一支（✗ 不许掉进「自举档」那支，
//	   agentd 是**换件档**、只写 `bin/zerg-agentd` 一件，与只写 `bin/zerg` 的自举语义不同）；
//	③ 真跑档名判词（gate 面）：真跑确认闸的 `gateSubject` 点明「`build all --only agentd` 子端换件档会**只覆盖 `bin/zerg-agentd` 一件**」。
//
// ✗ 只开 agentd 这一条：`core` / `compat`（覆盖主控件、连带在跑制品）与发布档口径不变 ⇒ 本件**对这两档做负控**
// （真跑仍落 `not_opened` 退 2）。
//
// ★ 本件**零副作用**（✗ 不真跑换件）：真跑用例一律**不给 `--confirm`** ⇒ 在确认闸就被拦（退 2），
// 一步都不 exec、不碰在跑制品。★ 负控（"牙"）：把 `family_build.go` 里 agentd 那支的 `openForExec = true`
// 哑化 ⇒ 本件 agentd 两格当场 FAIL（真跑那格退码从「缺确认」翻回「未开放」）—— 见件尾「哑化负控」注。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// abPassthrough —— 进程内直调 `cmdBuildPassthrough`（不引真二进制 · 与 `build_wall_test.go` 同型）：
// 给动作/`--only`/干跑位，返回 rc / stdout / stderr。`path` 决定 action（`build <action>`）。
func abPassthrough(t *testing.T, action, only string, dryRun bool) (int, string, string) {
	t.Helper()
	path := []string{"build", action}
	orig := []string{"build", action}
	inv := &invocation{path: path, orig: orig, dryRun: dryRun, kv: map[string][]string{}}
	if only != "" {
		inv.kv["--only"] = []string{only}
	}
	var out, errb bytes.Buffer
	rc := cmdBuildPassthrough(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestFamilyBuildAgentdReplacementOpen —— 子端换件档（agentd）真跑开放的两态 + core/compat 负控。
func TestFamilyBuildAgentdReplacementOpen(t *testing.T) {
	t.Run("计划件 子端换件档 ⇒ 已开放", func(t *testing.T) {
		rc, out, _ := abPassthrough(t, "all", "agentd", true)
		if rc != exitOK {
			t.Fatalf("`build all --only agentd --dry-run` rc=%d（要 0）· out=%q", rc, out)
		}
		if !strings.Contains(out, "本版状态 : **已开放**") {
			t.Fatalf("子端换件档计划件未印「已开放」：\n%s", out)
		}
		if strings.Contains(out, "**未开放**") {
			t.Fatalf("子端换件档计划件仍印「未开放」（三处判词没同批改齐？）：\n%s", out)
		}
		if !strings.Contains(out, "子端换件") {
			t.Fatalf("子端换件档计划件没点名「子端换件」档：\n%s", out)
		}
	})
	t.Run("计划件 自举档(cli) ⇒ 已开放（口径不变）", func(t *testing.T) {
		rc, out, _ := abPassthrough(t, "all", "cli", true)
		if rc != exitOK || !strings.Contains(out, "本版状态 : **已开放**") {
			t.Fatalf("自举档计划件该仍印「已开放」：rc=%d out=%q", rc, out)
		}
	})
	t.Run("计划件 发布档(release) ⇒ 已开放（口径不变）", func(t *testing.T) {
		rc, out, _ := abPassthrough(t, "release", "", true)
		if rc != exitOK || !strings.Contains(out, "本版状态 : **已开放**") {
			t.Fatalf("发布档计划件该仍印「已开放」：rc=%d out=%q", rc, out)
		}
	})

	// 负控（真跑面）：`core` / `compat` 仍是换件档 ⇒ 真跑一律 `not_opened` 退 2（✗ 没被这次的 agentd 开放连带放开）。
	for _, only := range []string{"core", "compat", ""} {
		only := only
		name := only
		if name == "" {
			name = "（默认全套 all）"
		}
		t.Run("负控 真跑 "+name+" ⇒ 仍未开放 退 2", func(t *testing.T) {
			rc, _, errs := abPassthrough(t, "all", only, false)
			if rc != exitUsage {
				t.Fatalf("`build all%s` 真跑 rc=%d（要 2 · 未开放）· stderr=%q", onlyNote(only), rc, errs)
			}
			if !strings.Contains(errs, "未开放") {
				t.Fatalf("`build all%s` 真跑该报「未开放」：stderr=%q", onlyNote(only), errs)
			}
		})
	}

	// 正控（真跑面 · 零副作用）：agentd 真跑**不给 `--confirm`** ⇒ 应走到**确认闸**（「缺确认」退 2），
	//   而**不是**在「未开放」闸就被拦 —— 这一格就是「执行位 `openForExec` 真被置真」的现读凭据。
	//   同时其 `gateSubject` 必须点名 agentd 档（真跑档名判词那一处）。
	t.Run("正控 真跑 agentd（无 --confirm）⇒ 走到确认闸（缺确认 退 2 · 非未开放）", func(t *testing.T) {
		rc, _, errs := abPassthrough(t, "all", "agentd", false)
		if rc != exitUsage {
			t.Fatalf("`build all --only agentd`（无确认）rc=%d（要 2）· stderr=%q", rc, errs)
		}
		if strings.Contains(errs, "未开放") {
			t.Fatalf("agentd 真跑仍在「未开放」闸被拦 ⇒ 执行位没开：stderr=%q", errs)
		}
		if !strings.Contains(errs, "缺确认") {
			t.Fatalf("agentd 真跑该走到确认闸报「缺确认」：stderr=%q", errs)
		}
		if !strings.Contains(errs, "子端换件档") || !strings.Contains(errs, "zerg-agentd") {
			t.Fatalf("agentd 真跑档名判词没点名子端换件档（gateSubject 那一处）：stderr=%q", errs)
		}
	})
}

// selfupdate_split_test.go —— 自更新真源收敛的**命令面判据机检**（§7.1 `P12` · §3.3 `N2` ·
// 开工单 T-52 判据②③）。
//
// 判据②：`zerg update`（源码式自更新 · `F-3`）与 `zerg core update`（主控自身换件 · `P12`）**语义不重**；
// 判据③：全仓**不再**用 `update` / `upgrade` 分指两事（命令面里没有第三个近义动词承接同一件事）。
package main_test

import (
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// TestUpdateWordSplit —— 判据②③：真命令树上「近义动词分指两事」命中 = 0（含逐条负控）。
func TestUpdateWordSplit(t *testing.T) {
	hits, err := zerg.UpdateSplitViolationsForTest()
	if err != nil {
		t.Fatalf("判定口读不出来：%v", err)
	}
	if len(hits) != 0 {
		t.Errorf("命中 %d 条（要 0）：%v", len(hits), hits)
	}

	// 负控①：某条命令的概要里冒出「升级」这类近义动词 ⇒ 必红。
	h2, err := zerg.UpdateSplitViolationsRawForTest("task submit", "提交任务并顺手升级主控", "zerg task submit", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(h2) != 1 {
		t.Errorf("负控①失败：写着「升级主控」的命令没被判红（判出 %d 条）：%v", len(h2), h2)
	}
	// 负控②：把 `zerg core update` 的语义来源改掉（抄成 update 的那一份 ⇒ 两件事又混了）⇒ 必红。
	h3, err := zerg.UpdateSplitViolationsRawForTest("core update", "主控自身换件", "zerg core update",
		"§九 M20 F-3 · 开工单 T-52") // ⚠ 故意把 P12 写成 F-3
	if err != nil {
		t.Fatal(err)
	}
	if len(h3) == 0 {
		t.Error("负控②失败：`core update` 的语义来源写成 update 那一份（F-3）也没被判红")
	}
	// 负控成对：两条命令各自写对的那一份 ⇒ 0 条。
	h4, _ := zerg.UpdateSplitViolationsRawForTest("core update", "主控自身换件",
		"zerg core update --confirm=<主机名> --yes [干跑档]", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52")
	if len(h4) != 0 {
		t.Errorf("负控成对失败：写对的两条也被判红：%v", h4)
	}
}

// TestSelfUpdateSpecShape —— 判据①/③ 的静态面：真源成形 + 调用方登记 + 已修的那处撞号。
func TestSelfUpdateSpecShape(t *testing.T) {
	spec, err := zerg.SelfUpdateSpecOfForTest()
	if err != nil {
		t.Fatalf("自更新真源读不出来：%v", err)
	}
	if spec.Kernel != "scripts/build/zerg-upgrade.sh" {
		t.Errorf("真源内核 = %q（要那支六阶段脚本 —— SD12 逐字）", spec.Kernel)
	}
	if strings.Join(spec.Stages, "|") != "plan|drain|swap|restart|verify|report" {
		t.Errorf("六阶段 = %v", spec.Stages)
	}
	joined := strings.Join(spec.Callers, " ")
	for _, want := range []string{"core/internal/selfupdate", "ui/src/modules/upgrade.rs"} {
		if !strings.Contains(joined, want) {
			t.Errorf("调用方 %q 没登记（现有：%s）", want, joined)
		}
	}
	// 判据②的**语义面**：`4` 只有一个意思（校验不通过），用法错是 `64`。
	if spec.ExitCodes["verify_failed"] != 4 {
		t.Errorf("verify_failed = %d（要 4）", spec.ExitCodes["verify_failed"])
	}
	if spec.ExitCodes["usage"] == 4 {
		t.Error("真源里的 usage 还是 4 —— 同数字两义没修掉")
	}
	if spec.ExitCodes["usage"] != 64 {
		t.Errorf("usage = %d（要 64 · sysexits EX_USAGE）", spec.ExitCodes["usage"])
	}
	// 本包常量与真源的逐格对拍（真跑；不相等即红）。
	if err := zerg.SelfUpdateExitCodesMatchForTest(); err != nil {
		t.Errorf("本包常量与真源对不上：%v", err)
	}
	// 负控：真源里 `4` 的语义必须**不是**「用法错」（否则上面那些断言就白设了）。
	if !strings.Contains(spec.ExitMeanings[4], "校验") {
		t.Errorf("真源里 4 的语义 = %q（要「校验不通过」）", spec.ExitMeanings[4])
	}
}

// TestUpdateAndCoreUpdateAreBothStillUnopened —— 判据②的**边界面**：本票只收敛真源，
// 两条命令都仍在危险档未开放（真跑一律拒执 ⇒ 2）—— 不可逆动作停下待拍。
func TestUpdateAndCoreUpdateAreBothStillUnopened(t *testing.T) {
	for _, argv := range [][]string{
		{"update", "x", "--confirm=x", "--yes"},
		{"core", "update", "x", "--confirm=x", "--yes"},
	} {
		rc, out, errb := runCapture(argv...)
		if rc != 2 {
			t.Errorf("%v 真跑退码 = %d（要 2 = 本版未开放）· stderr=%s", argv, rc, errb)
		}
		if !strings.Contains(errb, "未开放") {
			t.Errorf("%v 被拒的理由里没有「未开放」：%s", argv, errb)
		}
		if out != "" {
			t.Errorf("%v 拒执时 stdout 必须 0 字节：%q", argv, out)
		}
	}
}

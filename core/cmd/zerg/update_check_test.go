// update_check_test.go —— `zerg update --check` 的判据件（Q-129 / 8741788000）。
//
// 三档判据（每条都配一枚**负控**：把被测的那道守卫拆掉，它必须变红）：
//
//	① 三态退码 `updateRCFor`：五档状态逐档钉死（behind / diverged ⇒ 0 · up-to-date / local-ahead ⇒ 2 ·
//	   inconclusive ⇒ 1）。负控 = 把 inconclusive 那格改成 2（「判不了」冒充「无新版」）⇒ 必红。
//	② 只读档与写面旗标互斥：`--check --yes` / `--check --dry-run` / `--check --node x` / `--check x`
//	   一律 2，且**零网络**（沙箱里把 repo 指到不存在的路径 —— 若它先去 fetch，退码就不会是 2）。
//	③ `usageDeclaresExactFlag`：`--check` 不许被 `--check-excludes-parity` 的**子串**证明已登记
//	   （否则 `publish run --check` 从「未知旗标 2」退化成静默吞）。
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/selfupdate"
)

func TestUpdateRCForThreeStates(t *testing.T) {
	cases := []struct {
		name string
		res  selfupdate.CheckResult
		want int
	}{
		{"behind ⇒ 有新版", selfupdate.CheckResult{Status: selfupdate.StatusBehind, Behind: 2}, updateRCNew},
		{"diverged ⇒ 有新版", selfupdate.CheckResult{Status: selfupdate.StatusDiverged}, updateRCNew},
		{"up-to-date ⇒ 无新版", selfupdate.CheckResult{Status: selfupdate.StatusUpToDate}, updateRCNone},
		{"local-ahead ⇒ 无新版（开发态）", selfupdate.CheckResult{Status: selfupdate.StatusLocalAhead}, updateRCNone},
		{"inconclusive ⇒ 判不了", selfupdate.CheckResult{Status: selfupdate.StatusInconclusive}, updateRCUnknown},
	}
	for _, c := range cases {
		if got := updateRCFor(c.res); got != c.want {
			t.Fatalf("%s：退码 %d ≠ 期望 %d", c.name, got, c.want)
		}
	}
	// 三态必须**互不相同**，且 2 只属于「无新版」（「判不了」不许与它同码）。
	if updateRCNew == updateRCNone || updateRCNone == updateRCUnknown || updateRCNew == updateRCUnknown {
		t.Fatalf("三态退码撞号：%d/%d/%d", updateRCNew, updateRCNone, updateRCUnknown)
	}
	if updateRCNone != 2 || updateRCUnknown == 2 || updateRCNew != 0 || updateRCUnknown != 1 {
		t.Fatalf("退码面与 selfupdate 契约不一致：new=%d none=%d unknown=%d", updateRCNew, updateRCNone, updateRCUnknown)
	}
}

// updateCheckRun —— 在**零网络**前提下跑一次 `update --check`（repo 指向不存在的路径）：
// 互斥判定必须在 fetch **之前**收口，所以任何互斥档都拿不到「fetch 失败」那个 1。
func updateCheckRun(t *testing.T, argv ...string) (int, string) {
	t.Helper()
	inv, err := parseInvocation(argv)
	if err != nil {
		t.Fatalf("parseInvocation(%v)：%v", argv, err)
	}
	inv.path = []string{"update"}
	inv.args = nil // 位置参数由各格自己填
	for _, a := range argv {
		if a[0] != '-' {
			inv.args = append(inv.args, a)
		}
	}
	t.Setenv("ZERG_UPDATE_REPO", t.TempDir()+"/no-such-repo")
	var out, errb bytes.Buffer
	rc := cmdUpdateCheck(inv, &out, &errb)
	return rc, errb.String()
}

func TestUpdateCheckRejectsWriteFaceFlags(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"--yes", []string{"--check", "--yes"}, "只读检查"},
		{"--dry-run", []string{"--check", "--dry-run"}, "两件事"},
		{"--confirm", []string{"--check", "--confirm=x", "--yes"}, "只读检查"},
		{"--node", []string{"--check", "--node", "某机"}, "不点名机器"},
		{"位置参数", []string{"--check", "某机"}, "不点名机器"},
	}
	for _, c := range cases {
		rc, errText := updateCheckRun(t, c.argv...)
		if rc != exitUsage {
			t.Fatalf("%s：rc=%d（期望用法错 2）· stderr=%s", c.name, rc, errText)
		}
		if !strings.Contains(errText, c.want) {
			t.Fatalf("%s：stderr 没点名「%s」—— 实际：%s", c.name, c.want, errText)
		}
	}
}

func TestUsageDeclaresExactFlagNotSubstring(t *testing.T) {
	// 真树里的两条命令：`publish run` 的 usage 只写 `--check-excludes-parity`（不含裸 `--check`）；
	// `update` 写裸 `--check`。
	pub := find([]string{"publish", "run"})
	upd := find([]string{"update"})
	if pub == nil || upd == nil {
		t.Fatal("命令树里找不到 publish run / update —— 判据前提不成立")
	}
	if usageDeclaresExactFlag(pub, "--check") {
		t.Fatalf("`--check` 被 `publish run` 的用法串「证明」已登记（子串判的坑复发了）")
	}
	if !usageDeclaresFlag(pub, "--check") {
		// 前置：确认这条负控**踩得到**子串判那个坑（否则本格是空转）
		t.Fatalf("前提失效：`publish run` 的 usage 里没有 `--check-excludes-parity` 前缀")
	}
	if !usageDeclaresExactFlag(upd, "--check") {
		t.Fatalf("`update` 的用法串没写裸 `--check` —— 新旗标等于不可用")
	}
	if usageDeclaresExactFlag(pub, "--check-excludes-parity") != true {
		t.Fatalf("精确判把真旗标判丢了")
	}
}

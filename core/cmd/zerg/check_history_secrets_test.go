// check_history_secrets_test.go —— core/cmd/zerg/check_history_secrets.go 的**成对负控**单测（✗ 禁恒绿）。
//
// 三腿：① 正控（真夹具仓的阻断命中必判红）② 负控（哑化判据真源 ⇒ 读数必须跟着变 ＝ 真源有牙）
// ③ 自指（本件源文本不许被本门自己的 BLOCK 正则命中）。夹具全落**仓外**（t.TempDir + 真 git init）。
package main

import (
	"os"
	"strings"
	"testing"
)

func chsecTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := checkHistorySecretsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ① 正控：白名单内的真形状敏感文本 ⇒ 必判红 rc=1；干净件 ⇒ 回绿 rc=0（两向都有，缺一向就是半张网）。
func TestCheckHistorySecretsGate_DetectsAndRecovers(t *testing.T) {
	base := t.TempDir()
	dirty := chsecMkFixtureRepo(base, "dirty", []chsecFixtureFile{{"src/app.py", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rc, out, _ := chsecTestRun(t, "--repo", dirty)
	if rc != 1 {
		t.Fatalf("正控失败：脏件 rc=%d（期望 1）\n%s", rc, out)
	}
	if !strings.Contains(out, "阻断级命中") {
		t.Fatalf("正控失败：未打印「阻断级命中」\n%s", out)
	}

	clean := chsecMkFixtureRepo(base, "clean", []chsecFixtureFile{{"src/app.py", chsecFixGood}}, "夹具：初始提交", chsecFixWL)
	rc, out, _ = chsecTestRun(t, "--repo", clean)
	if rc != 0 || !strings.Contains(out, "结果: 通过") {
		t.Fatalf("正控失败：干净件 rc=%d（期望 0 且「结果: 通过」）\n%s", rc, out)
	}
}

// ② 负控：哑化判据真源（BLOCK 模式表清空）⇒ 同一棵必红的树退回 rc=0（成对两态，真源有牙）。
func TestCheckHistorySecretsGate_NegCtlMuteBlockHasTeeth(t *testing.T) {
	base := t.TempDir()
	dirty := chsecMkFixtureRepo(base, "dirty", []chsecFixtureFile{{"src/app.py", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)

	rcBefore, _, _ := chsecTestRun(t, "--repo", dirty)
	if rcBefore != 1 {
		t.Fatalf("负控前置失败：未动真源时 rc=%d（期望 1）", rcBefore)
	}
	saved := chsecBlockSpecs
	defer func() { chsecBlockSpecs = saved }()
	chsecBlockSpecs = nil
	rcAfter, _, _ := chsecTestRun(t, "--repo", dirty)
	if rcAfter != 0 {
		t.Fatalf("负控失败：哑化 BLOCK 后 rc=%d（期望 0 ⇒ 该真源本来没被用上 = 假绿）", rcAfter)
	}
}

// ②′ 负控：白名单外同一段文本 ⇒ 放绿（区分度）；把白名单判据换成恒真 ⇒ 必变红（放行侧不是恒绿）。
func TestCheckHistorySecretsGate_WhitelistGateHasTeeth(t *testing.T) {
	base := t.TempDir()
	outside := chsecMkFixtureRepo(base, "outside", []chsecFixtureFile{{"notes/secret.md", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rcBefore, _, _ := chsecTestRun(t, "--repo", outside)
	if rcBefore != 0 {
		t.Fatalf("负控前置失败：白名单外 rc=%d（期望 0）", rcBefore)
	}
	saved := chsecPublishedPathFn
	defer func() { chsecPublishedPathFn = saved }()
	chsecPublishedPathFn = func(rel string, entries []string) bool { return true }
	rcAfter, _, _ := chsecTestRun(t, "--repo", outside)
	if rcAfter != 1 {
		t.Fatalf("负控失败：白名单判据换恒真后 rc=%d（期望 1 ⇒ 该真源没被用上）", rcAfter)
	}
}

// 退码钳：`--repo` 不是 git 仓 ⇒ rc=2（判不了 ≠ 绿）。且负控：非 git 仓不许被判成绿。
func TestCheckHistorySecretsGate_NotGitRepo(t *testing.T) {
	d := t.TempDir()
	if err := chsecWriteFile(chsecPyJoin(d, "publish", "whitelist.txt"), chsecFixWL); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := chsecTestRun(t, "--repo", d)
	if rc != 2 || !strings.Contains(out, "不是 git 仓库") {
		t.Fatalf("退码钳失败：rc=%d（期望 2 且「不是 git 仓库」）\n%s", rc, out)
	}
}

// 空气转 ≠ 绿：噪声后缀件不审 ⇒ rc=0，但空转读数必须逐字可见（不许静默少判）。
func TestCheckHistorySecretsGate_EmptyScanNotGreen(t *testing.T) {
	base := t.TempDir()
	noise := chsecMkFixtureRepo(base, "noise", []chsecFixtureFile{{"src/app.py.bak", chsecFixDirty}}, "夹具：初始提交", chsecFixWL)
	rc, out, _ := chsecTestRun(t, "--repo", noise)
	if rc != 0 {
		t.Fatalf("噪声件应放绿：rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "文件版本 0 个") {
		t.Fatalf("空转必须逐字可见（缺「文件版本 0 个」）：\n%s", out)
	}
}

// 未覆盖/口径档：未知旗标 ⇒ 用法错 rc=2（argparse 复刻）。
func TestCheckHistorySecretsGate_UnknownFlag(t *testing.T) {
	rc, _, errs := chsecTestRun(t, "--no-such-flag")
	if rc != 2 || !strings.Contains(errs, "unrecognized arguments") {
		t.Fatalf("未知旗标应 rc=2：rc=%d stderr=%q", rc, errs)
	}
}

// ③ 自指：本门自己的 BLOCK 正则**不许**命中本件源文本（拼接构造有效 ⇒ 落仓后真仓读数不上升）。
func TestCheckHistorySecretsGate_NoSelfLeak(t *testing.T) {
	src, err := os.ReadFile("check_history_secrets.go")
	if err != nil {
		t.Skipf("读不到本件源文本（跳过自指面）：%v", err)
	}
	text := string(src)
	specs := chsecCompileSpecs(chsecBlockSpecs)
	for _, c := range specs {
		if m := c.rx.FindString(text); m != "" {
			t.Errorf("自指命中：BLOCK 档 %q 在本件源文本命中 %q", c.why, m)
		}
	}
	// 反向牙：拼接构造出来的命中词必须能匹配真形态（否则是「拼过头」= 判据失效）。
	if !specs[1].rx.MatchString(chsecFixHome) {
		t.Errorf("家目录判据拼装后无法匹配真形态 %q", chsecFixHome)
	}
	if !specs[4].rx.MatchString(chsecFixAWS) {
		t.Errorf("AWS 判据拼装后无法匹配真形态")
	}
}

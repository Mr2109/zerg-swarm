// check_agentd_unit_default_test.go —— 判据件 `scripts/gates/check-agentd-unit-default.sh`
// 的 Go 真搬 **成对负控**测试件。覆盖：
//
//	· caudJudge 成对正负控（合成夹具：四格全过 ⇒ rc=0 · 硬编码 ⇒ rc=1 且点名 件:行 ·
//	  缺机名推导 / 缺 darwin 半边 / 缺作业名推导 ⇒ rc=1 · 件不在盘 ⇒ rc=2 fail-closed）
//	· caudSelftest 6 格全绿（正控）
//	· 入口三档（--help / --self-test / 用法错）逐字读数
//	· 无 `--root` ⟷ 有 `--root` 同根（默认根定位两态同）
//	· 哑化有牙：摘掉 caudFnPlist ⇒ 自检 ① 格必转红
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func caudTgWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
}

func caudTgRun(t *testing.T, f string) (int, string) {
	t.Helper()
	var b strings.Builder
	rc := caudJudge(f, &b)
	return rc, b.String()
}

// 成对正负控（合成夹具 · 只读面）。
func TestAgentdUnitDefaultJudge正负控(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "good.sh")
	badHard := filepath.Join(d, "bad-hardcoded.sh")
	badNoDerive := filepath.Join(d, "bad-noderive.sh")
	badNoDarwin := filepath.Join(d, "bad-nodarwin.sh")
	badNoLabel := filepath.Join(d, "bad-nolabel.sh")

	caudTgWrite(t, good, caudFixGood)
	caudTgWrite(t, badHard, caudFixBadHard)
	caudTgWrite(t, badNoDerive, caudFixBadNoDerive)
	caudTgWrite(t, badNoDarwin, caudFixBadNoDarwin)
	caudTgWrite(t, badNoLabel, caudFixBadNoLabel)

	// 正控：四格全过 ⇒ rc=0，且逐字点四个件:行。
	if rc, out := caudTgRun(t, good); rc != caudRCOK {
		t.Fatalf("正控应 rc=0，实际 rc=%d\n%s", rc, out)
	} else {
		if !strings.Contains(out, "✓ 绿（rc=0）：agent 单元名缺省从机名推导") {
			t.Errorf("正控绿行缺失：\n%s", out)
		}
		for _, want := range []string{good + ":8", good + ":1", good + ":9", good + ":4"} {
			if !strings.Contains(out, want) {
				t.Errorf("正控 件:行 应含 %s：\n%s", want, out)
			}
		}
	}
	// 负控 ②：硬编码 zerg-agentd 当单元名缺省 ⇒ rc=1 且点名 :3。
	if rc, out := caudTgRun(t, badHard); rc != caudRCFail {
		t.Fatalf("硬编码负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, badHard+":3") || !strings.Contains(out, "硬编码") {
		t.Errorf("硬编码负控应点名 件:行 :3：\n%s", out)
	}
	// 负控 ③：缺机名推导（A1）⇒ rc=1。
	if rc, out := caudTgRun(t, badNoDerive); rc != caudRCFail {
		t.Fatalf("缺机名推导负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "找不到「单元名缺省**从机名推导**」那一行") {
		t.Errorf("缺机名推导负控判词缺失：\n%s", out)
	}
	// 负控 ④：缺 write_agent_plist（darwin 半边不在）⇒ rc=1。
	if rc, out := caudTgRun(t, badNoDarwin); rc != caudRCFail {
		t.Fatalf("缺 darwin 半边负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "darwin 半边缺失") {
		t.Errorf("缺 darwin 半边负控判词缺失：\n%s", out)
	}
	// 负控 ⑤：LaunchAgent 作业名硬编码（A3b）⇒ rc=1。
	if rc, out := caudTgRun(t, badNoLabel); rc != caudRCFail {
		t.Fatalf("缺作业名推导负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "LaunchAgent 作业名缺省从机名推导那一行") {
		t.Errorf("缺作业名推导负控判词缺失：\n%s", out)
	}
	// 负控 ⑥：件不在盘 ⇒ rc=2（fail-closed，不给结论）。
	if rc, out := caudTgRun(t, filepath.Join(d, "not-there.sh")); rc != caudRCBlocked {
		t.Fatalf("缺件负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "安装器件不在盘") {
		t.Errorf("缺件负控判词缺失：\n%s", out)
	}
	// 负控 ⑦：目录 ⇒ rc=2（sh `[ ! -f ]` 对目录为真）。
	if rc, out := caudTgRun(t, d); rc != caudRCBlocked {
		t.Fatalf("目录负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "安装器件不在盘") {
		t.Errorf("目录负控判词缺失：\n%s", out)
	}
}

// 正控：6 格自检必须全绿。
func TestAgentdUnitDefaultSelftest全绿(t *testing.T) {
	var b strings.Builder
	if rc := caudSelftest(&b); rc != caudRCOK {
		t.Fatalf("自检应 rc=0，实际 rc=%d\n%s", rc, b.String())
	}
	for _, want := range []string{
		"✓ ① 正控（机名推导 + 无硬编码 + darwin 半边在）⇒ rc=0",
		"✓ ② 负控（硬编码 zerg-agentd · 即改前旧形）⇒ rc=1",
		"✓ ②b 且点名 件:行（:3）",
		"✓ ③ 负控（缺机名推导 · A1）⇒ rc=1",
		"✓ ④ 负控（缺 write_agent_plist · darwin 半边不在）⇒ rc=1",
		"✓ ⑤ 负控（LaunchAgent 作业名硬编码 · 缺机名推导 · A3b）⇒ rc=1",
		"✓ ⑥ 负控（件不在盘）⇒ rc=2",
		"自检 PASS（6 格）",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("自检读数缺 %q：\n%s", want, b.String())
		}
	}
}

// 入口三档：--help 逐字等于 usage · --self-test 绿 · 用法错 rc=2（两枚缺值档都不打 usage）。
func TestAgentdUnitDefault入口档(t *testing.T) {
	var out, errb strings.Builder
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--help"}); rc != caudRCOK {
		t.Fatalf("--help 应 rc=0，实际 %d", rc)
	}
	if out.String() != caudUsage {
		t.Errorf("--help 输出应逐字等于 usage\n得：%q\n要：%q", out.String(), caudUsage)
	}
	out.Reset()
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--self-test"}); rc != caudRCOK {
		t.Fatalf("--self-test 应 rc=0，实际 %d\n%s", rc, out.String())
	}
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--selftest"}); rc != caudRCOK {
		t.Fatalf("--selftest 应 rc=0，实际 %d", rc)
	}
	out.Reset()
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--bogus"}); rc != caudRCBlocked {
		t.Fatalf("未知参数应 rc=2，实际 %d", rc)
	}
	if !strings.Contains(out.String(), "用法错：未知参数 --bogus ⇒ 不给结论（rc=2）") || !strings.Contains(out.String(), caudUsage) {
		t.Errorf("未知参数档应「一句用法错 ＋ usage」：\n%s", out.String())
	}
	out.Reset()
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--root"}); rc != caudRCBlocked {
		t.Fatalf("`--root` 缺值应 rc=2，实际 %d", rc)
	}
	if out.String() != "用法错：`--root` 缺值 ⇒ 不给结论（rc=2）\n" {
		t.Errorf("`--root` 缺值档应只打一行（✗ 不带 usage）：%q", out.String())
	}
	out.Reset()
	if rc := checkAgentdUnitDefaultGateMain(&out, &errb, []string{"--script"}); rc != caudRCBlocked {
		t.Fatalf("`--script` 缺值应 rc=2，实际 %d", rc)
	}
	if out.String() != "用法错：`--script` 缺值 ⇒ 不给结论（rc=2）\n" {
		t.Errorf("`--script` 缺值档应只打一行：%q", out.String())
	}
}

// 默认根定位两态：无 `--root`（从 cwd 上溯）⟷ 有 `--root <上溯结果>` ⇒ 逐字同。
func TestAgentdUnitDefault无根与显式根同结果(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := caudFindRoot(cwd)
	if root == "" || root == cwd {
		t.Fatalf("仓根上溯失败：cwd=%s root=%s", cwd, root)
	}
	var a, b strings.Builder
	rcA := checkAgentdUnitDefaultGateMain(&a, io.Discard, nil)
	rcB := checkAgentdUnitDefaultGateMain(&b, io.Discard, []string{"--root", root})
	if rcA != rcB || a.String() != b.String() {
		t.Fatalf("无 --root ⟷ 有 --root 应逐字同：rc %d/%d\n无根：%s\n显式：%s", rcA, rcB, a.String(), b.String())
	}
}

// 哑化有牙：摘掉 darwin 半边的函数名字面 ⇒ 自检 ① 格（darwin 半边在）必转红。
func TestAgentdUnitDefault哑化必红(t *testing.T) {
	saved := caudFnPlist
	caudFnPlist = "zzz-本串绝不出现-plist"
	defer func() { caudFnPlist = saved }()
	var b strings.Builder
	if rc := caudSelftest(&b); rc == caudRCOK {
		t.Fatalf("摘掉 caudFnPlist 后自检必须红（负控有牙），实际 rc=0\n%s", b.String())
	}
	if !strings.Contains(b.String(), "✗ ① 正控该绿没绿（rc=1）") {
		t.Fatalf("哑化后应见 ① 格转红，实际：\n%s", b.String())
	}
}

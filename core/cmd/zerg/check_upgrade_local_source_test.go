// check_upgrade_local_source_test.go —— 判据件 `scripts/gates/check-upgrade-local-source.sh`
// 的 Go 真搬 **成对负控**测试件。覆盖：
//
//	· cuplsExtractValue 引号/裸值三态 · cuplsValueAlwaysNonempty 兜底判定四态
//	· cuplsJudge 成对正负控（合成夹具：已注入 ⇒ rc=0 · 缺注入 / 空值 / 无兜底 ⇒ rc=1 且点名 ·
//	  锚 0 命中 / 多命中 / 件不在盘 ⇒ rc=2 fail-closed）
//	· cuplsSelftest 7 格全绿（正控）
//	· 入口三档（--help / --self-test / 用法错）逐字读数
//	· 无 `--root` ⟷ 有 `--root` 同根（默认根定位两态同）
//	· 哑化有牙：摘掉 cuplsKeyAnchor ⇒ 自检 ① 格必转红
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cuplsTgWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
}

func cuplsTgRun(t *testing.T, f string) (int, string) {
	t.Helper()
	var b strings.Builder
	rc := cuplsJudge(f, &b)
	return rc, b.String()
}

// 取值三态 + 恒非空四态（纯函数面）。
func TestUpgradeLocalSource取值与恒非空(t *testing.T) {
	line := `lout="$(ZERG_UPDATE_ROLE=controller ZERG_UPDATE_REF="$FLEET_REF" ZERG_UPDATE_REMOTE="${FLEET_REMOTE:-$l_root}" sh -c "$lcmd" 2>&1)"`
	if v, ok := cuplsExtractValue(line, cuplsKeyRemote); !ok || v != "${FLEET_REMOTE:-$l_root}" {
		t.Errorf("双引号取值：得 %q ok=%v", v, ok)
	}
	if v, ok := cuplsExtractValue(line, cuplsKRef); !ok || v != "$FLEET_REF" {
		t.Errorf("REF 取值：得 %q ok=%v", v, ok)
	}
	if _, ok := cuplsExtractValue(line, "ZERG_NOT_THERE="); ok {
		t.Errorf("缺键应 ok=false")
	}
	bare := cuplsKeyRemote + "abc$def more"
	if v, ok := cuplsExtractValue(bare, cuplsKeyRemote); !ok || v != "abc$def" {
		t.Errorf("裸值应取到空白前：得 %q ok=%v", v, ok)
	}
	for _, c := range []struct {
		name string
		v    string
		want bool
	}{
		{"空串 ⇒ 假", "", false},
		{"字面量 ⇒ 真", "https://example.invalid/repo.git", true},
		{"纯展开 ⇒ 假", "$FLEET_REMOTE", false},
		{"非空兜底 ⇒ 真", "${FLEET_REMOTE:-$l_root}", true},
		{"空兜底 ⇒ 假", `${FLEET_REMOTE:-}`, false},
		{"assign 非空兜底 ⇒ 真", "${FLEET_REMOTE:=$l_root}", true},
	} {
		if got := cuplsValueAlwaysNonempty(c.v); got != c.want {
			t.Errorf("%s：cuplsValueAlwaysNonempty(%q) = %v，期望 %v", c.name, c.v, got, c.want)
		}
	}
}

// 成对正负控（合成夹具 · 只读面）。
func TestUpgradeLocalSourceJudge正负控(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "good.sh")
	badMissing := filepath.Join(d, "bad-missing.sh")
	badEmpty := filepath.Join(d, "bad-empty.sh")
	badNoVar := filepath.Join(d, "bad-novar.sh")
	badRefEmpty := filepath.Join(d, "bad-refempty.sh")
	badAnchor := filepath.Join(d, "bad-anchor.sh")
	badTwo := filepath.Join(d, "bad-two-anchor.sh")

	cuplsTgWrite(t, good, cuplsFixGood)
	cuplsTgWrite(t, badMissing, cuplsFixBadMissing)
	cuplsTgWrite(t, badEmpty, cuplsFixBadEmpty)
	cuplsTgWrite(t, badNoVar, cuplsFixBadNoVar)
	cuplsTgWrite(t, badRefEmpty, cuplsFixBadRefEmpty)
	cuplsTgWrite(t, badAnchor, cuplsFixBadAnchor)
	cuplsTgWrite(t, badTwo, cuplsFixGood+cuplsFixGood)

	// 正控：已显式注入 → 恒非空 ⇒ rc=0，且逐字点 件:行。
	if rc, out := cuplsTgRun(t, good); rc != cuplsRCOK {
		t.Fatalf("正控应 rc=0，实际 rc=%d\n%s", rc, out)
	} else {
		if !strings.Contains(out, "✓ 绿（rc=0）：本机步已显式注入取源") {
			t.Errorf("正控绿行缺失：\n%s", out)
		}
		if !strings.Contains(out, good+":2") {
			t.Errorf("正控应点 件:行 :2：\n%s", out)
		}
	}
	// 负控 ②：缺 REMOTE/REF（注释行里的键不算）⇒ rc=1 且点名 :2。
	if rc, out := cuplsTgRun(t, badMissing); rc != cuplsRCFail {
		t.Fatalf("缺注入负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, badMissing+":2") || !strings.Contains(out, "本机步**没有注入**") {
		t.Errorf("缺注入负控应点名 :2 并给「没有注入」判词：\n%s", out)
	}
	// 负控 ③：REMOTE 给空串 ⇒ rc=1。
	if rc, out := cuplsTgRun(t, badEmpty); rc != cuplsRCFail {
		t.Fatalf("REMOTE 空串负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "不能保证非空") {
		t.Errorf("REMOTE 空串负控判词缺失：\n%s", out)
	}
	// 负控 ④：REMOTE 无兜底展开 ⇒ rc=1。
	if rc, out := cuplsTgRun(t, badNoVar); rc != cuplsRCFail {
		t.Fatalf("REMOTE 无兜底负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "不能保证非空") {
		t.Errorf("REMOTE 无兜底负控判词缺失：\n%s", out)
	}
	// 负控 ④b：REF 给空字面 ⇒ rc=1。
	if rc, out := cuplsTgRun(t, badRefEmpty); rc != cuplsRCFail {
		t.Fatalf("REF 空字面负控应 rc=1，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "是**空值**") {
		t.Errorf("REF 空字面负控判词缺失：\n%s", out)
	}
	// 负控 ⑤：锚 0 命中（结构已变）⇒ rc=2。
	if rc, out := cuplsTgRun(t, badAnchor); rc != cuplsRCBlocked {
		t.Fatalf("锚 0 命中负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "找不到**本机步**锚") {
		t.Errorf("锚 0 命中负控判词缺失：\n%s", out)
	}
	// 负控 ⑤b：锚多命中（歧义）⇒ rc=2。
	if rc, out := cuplsTgRun(t, badTwo); rc != cuplsRCBlocked {
		t.Fatalf("锚多命中负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "命中 2 行（>1 ⇒ 歧义）") {
		t.Errorf("锚多命中负控判词缺失：\n%s", out)
	}
	// 负控 ⑥：件不在盘 ⇒ rc=2。
	if rc, out := cuplsTgRun(t, filepath.Join(d, "not-there.sh")); rc != cuplsRCBlocked {
		t.Fatalf("缺件负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "升级器件不在盘") {
		t.Errorf("缺件负控判词缺失：\n%s", out)
	}
	// 负控 ⑦：目录 ⇒ rc=2。
	if rc, out := cuplsTgRun(t, d); rc != cuplsRCBlocked {
		t.Fatalf("目录负控应 rc=2，实际 rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "升级器件不在盘") {
		t.Errorf("目录负控判词缺失：\n%s", out)
	}
}

// 正控：7 格自检必须全绿。
func TestUpgradeLocalSourceSelftest全绿(t *testing.T) {
	var b strings.Builder
	if rc := cuplsSelftest(&b); rc != cuplsRCOK {
		t.Fatalf("自检应 rc=0，实际 rc=%d\n%s", rc, b.String())
	}
	for _, want := range []string{
		"✓ ① 正控（已注入 REMOTE/REF）⇒ rc=0",
		"✓ ② 负控（缺 REMOTE/REF · 即改前原形）⇒ rc=1",
		"✓ ②b 且点名 件:行（:2）",
		"✓ ③ 负控（REMOTE 给空串）⇒ rc=1",
		"✓ ④ 负控（REMOTE 无兜底展开 ⇒ 可为空）⇒ rc=1",
		"✓ ④b 负控（REF 给空字面）⇒ rc=1",
		"✓ ⑤ 负控（本机步锚 0 命中 ⇒ 结构变）⇒ rc=2",
		"✓ ⑥ 负控（件不在盘）⇒ rc=2",
		"自检 PASS（7 格）",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("自检读数缺 %q：\n%s", want, b.String())
		}
	}
}

// 入口三档：--help 逐字等于 usage · --self-test 绿 · 用法错 rc=2。
func TestUpgradeLocalSource入口档(t *testing.T) {
	var out, errb strings.Builder
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--help"}); rc != cuplsRCOK {
		t.Fatalf("--help 应 rc=0，实际 %d", rc)
	}
	if out.String() != cuplsUsageText() {
		t.Errorf("--help 输出应逐字等于 usage\n得：%q\n要：%q", out.String(), cuplsUsageText())
	}
	out.Reset()
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--self-test"}); rc != cuplsRCOK {
		t.Fatalf("--self-test 应 rc=0，实际 %d\n%s", rc, out.String())
	}
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--selftest"}); rc != cuplsRCOK {
		t.Fatalf("--selftest 应 rc=0，实际 %d", rc)
	}
	out.Reset()
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--bogus"}); rc != cuplsRCBlocked {
		t.Fatalf("未知参数应 rc=2，实际 %d", rc)
	}
	if !strings.Contains(out.String(), "用法错：未知参数 --bogus ⇒ 不给结论（rc=2）") || !strings.Contains(out.String(), cuplsUsageText()) {
		t.Errorf("未知参数档应「一句用法错 ＋ usage」：\n%s", out.String())
	}
	out.Reset()
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--script"}); rc != cuplsRCBlocked {
		t.Fatalf("`--script` 缺值应 rc=2，实际 %d", rc)
	}
	if out.String() != "用法错：`--script` 缺值 ⇒ 不给结论（rc=2）\n" {
		t.Errorf("`--script` 缺值档应只打一行：%q", out.String())
	}
	out.Reset()
	if rc := checkUpgradeLocalSourceGateMain(&out, &errb, []string{"--root"}); rc != cuplsRCBlocked {
		t.Fatalf("`--root` 缺值应 rc=2，实际 %d", rc)
	}
	if out.String() != "用法错：`--root` 缺值 ⇒ 不给结论（rc=2）\n" {
		t.Errorf("`--root` 缺值档应只打一行：%q", out.String())
	}
}

// 默认根定位两态：无 `--root`（从 cwd 上溯）⟷ 有 `--root <上溯结果>` ⇒ 逐字同。
func TestUpgradeLocalSource无根与显式根同结果(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := cuplsFindRoot(cwd)
	if root == "" || root == cwd {
		t.Fatalf("仓根上溯失败：cwd=%s root=%s", cwd, root)
	}
	var a, b strings.Builder
	rcA := checkUpgradeLocalSourceGateMain(&a, io.Discard, nil)
	rcB := checkUpgradeLocalSourceGateMain(&b, io.Discard, []string{"--root", root})
	if rcA != rcB || a.String() != b.String() {
		t.Fatalf("无 --root ⟷ 有 --root 应逐字同：rc %d/%d\n无根：%s\n显式：%s", rcA, rcB, a.String(), b.String())
	}
}

// 哑化有牙：摘掉本机步锚串 ⇒ 自检 ① 格（正控该绿）必转红。
func TestUpgradeLocalSource哑化必红(t *testing.T) {
	saved := cuplsKeyAnchor
	cuplsKeyAnchor = "ZERG_UPDATE_ROLE=zzz-本锚绝不出现"
	defer func() { cuplsKeyAnchor = saved }()
	var b strings.Builder
	if rc := cuplsSelftest(&b); rc == cuplsRCOK {
		t.Fatalf("摘掉 cuplsKeyAnchor 后自检必须红（负控有牙），实际 rc=0\n%s", b.String())
	}
	if !strings.Contains(b.String(), "✗ ① 正控该绿没绿（rc=2）") {
		t.Fatalf("哑化后应见 ① 格转红，实际：\n%s", b.String())
	}
}

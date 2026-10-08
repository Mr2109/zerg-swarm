// deploy_launch_test.go —— `zerg deploy launch`（`cmdDeployLaunch`）**真搬后**的两态三面对拍与牙。
//
// 两态 = ① Go 半（进程内直调 `cmdDeployLaunch` / 包装入口 `deployLaunchGateMain`）；
//
//	② 原件直跑（`bash deploy/scripts/deploy-launch.sh <tail>`）。三面 = rc / stdout / stderr。
//
// ★ 非确定面只有一处：意图件名里的 `intent-<YYYYmmddTHHMMSSZ>-` 时刻（原件自 `date -u` 取）
//
//	⇒ 比对前归一到 `intent-<TS>-`（自证：原件连跑两发跨秒口径也不同）。
//
// ★ `ssh` 一律走**替身**（临时 bin 前置到 PATH ⇒ 两态都解析到它）：不联网、不碰 known_hosts、
//
//	退码与 stderr 全可控 ⇒ 才敢把 rc=0/4/5 三条路都摆上格子。
//
// ★ **已声明的行为差（✗ 不摆进对拍格）**：原件把实参**原样内插进 python 源码**（heredoc `<<PY`）
//
//	⇒ 值里含 `"` 或换行时该源码语法坏掉（cd 到 3 + python SyntaxError）· 含 `\` 时被 python 解成
//	转义序列（落盘内容与原值不同）。本件按「值原样」结构化写盘 ⇒ 这几档是**本件与原件的差**
//	（见件头声明项 (d)），对拍格一律避开这三类字符。
//
// ★ 牙：与原件逐格对拍（实现漂了就红）＋ 显式 rc 期望 ＋ `--help` 逐字节等于原件第 2..30 行
//
//	＋ 「✗ 不 exec 原件」的实证（把原件换成会写标记的桩 ⇒ 标记不得出现）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dplStubSSH —— 测试用 `ssh` 替身：只看实参里有没有回读那条命令（含 `latest-deploy.json`）。
const dplStubSSH = `#!/usr/bin/env bash
case "$*" in
  *latest-deploy.json*)
    printf '%s' "${DPL_STUB_RB_STDERR:-}" >&2
    if [ "${DPL_STUB_READBACK:-0}" = "1" ]; then
      printf '%s' "${DPL_STUB_RB_JSON:-}"
      exit "${DPL_STUB_RB_RC:-0}"
    fi
    exit "${DPL_STUB_RB_RC:-255}"
    ;;
esac
printf '%s' "${DPL_STUB_AGENT_STDOUT:-}"
printf '%s' "${DPL_STUB_AGENT_STDERR:-}" >&2
exit "${DPL_STUB_AGENT_RC:-0}"
`

var dplTSRe = regexp.MustCompile(`intent-\d{8}T\d{6}Z-`)

// dplNormTS 归一「意图件名里的时刻」（唯一非确定面）。
func dplNormTS(s string) string { return dplTSRe.ReplaceAllString(s, "intent-<TS>-") }

// dplStubSSHInstall 装替身并把它的目录前置到 PATH（两态共用 ⇒ Go 的 `exec.Command("ssh")` 与
// 原件的 `ssh` 都解析到替身）。
func dplStubSSHInstall(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(dplStubSSH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// dplRealScript —— 真件路径（现读在位才跑真件对拍；不在 ⇒ 调用方 Skip）。
func dplRealScript(t *testing.T) string {
	t.Helper()
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件对拍")
	}
	script := filepath.Join(root, deployLaunchScriptRel)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("真件不在（退役态 / 公开快照）：%v", err)
	}
	return script
}

// dplRunGo 走 Go 半（进程内直调 `cmdDeployLaunch`）。
func dplRunGo(t *testing.T, tail ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"deploy", "launch"},
		orig: append([]string{"deploy", "launch"}, tail...),
	}
	// ★ 与真实解析面同形：进程内直调不经过 `parseInvocation` ⇒ 手工把被授权门消费的两枚旗标
	//   落到解析字段上（`writeGuardYes` 读 `inv.dryRun`/`inv.yes`）。
	dplAuthFlags(inv, tail)
	rc := cmdDeployLaunch(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// dplRunGoGate 走**包装入口**（inproc 同签名 `deployLaunchGateMain`）。
func dplRunGoGate(t *testing.T, tail ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := deployLaunchGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// dplRunSH 直跑原件（另一态）。
func dplRunSH(t *testing.T, script string, tail ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	c := exec.Command("bash", append([]string{script}, tail...)...)
	c.Stdin = strings.NewReader("")
	c.Stdout, c.Stderr = &o, &e
	rc := 0
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("原件直跑起不来：%v", err)
		}
	}
	return rc, o.String(), e.String()
}

// TestDeployLaunchTwoStatesThreeFacesGrid —— 两态三面逐格对拍（旗标面 × 退码面 × ssh 三路）。
func TestDeployLaunchTwoStatesThreeFacesGrid(t *testing.T) {
	script := dplRealScript(t)
	dplStubSSHInstall(t)
	tmp := t.TempDir()
	afile := filepath.Join(tmp, "afile")
	if err := os.WriteFile(afile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	const goodRB = `{"result":"ok","result_code":0,"unit":{"name":"u.service"},"observed":{"live_exe_sha256":"deadbeef"}}`
	cells := []struct {
		name string
		tail []string
		env  map[string]string
		want int
		// writeYes —— 本格是**写路**（真起 ssh / 回读）⇒ 走 Go 半时补 `--yes`
		// （默认档 fail-closed · 门100 写面登记）；原件那一半**不加**（底层脚本不认 `--yes`
		// ⇒ 加了会退 2「未知参数」）。`--dry-run` 格与「前置错误」格都不算写路。
		writeYes bool
	}{
		{"help", []string{"--help"}, nil, 0, false},
		{"help-short", []string{"-h"}, nil, 0, false},
		{"unknown-flag", []string{"--nope-not-a-flag"}, nil, 2, false},
		{"missing-host", nil, nil, 2, false},
		{"missing-file", []string{"--host", "x" + "3"}, nil, 2, false},
		{"missing-sha", []string{"--host", "x" + "3", "--file", "/tmp/x.json"}, nil, 2, false},
		{"dry-run-min", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(tmp, "d-min"), "--dry-run"}, nil, 0, false},
		// ★ 引号面（避开 `"` / `\` / 换行 —— 见件头「已声明的行为差」）：空格 `'` `$` `*` `?` `&` `#` `~` 与 TAB。
		{"dry-run-quote", []string{"--host", "user@<worker-ip>", "--file", "/tmp/a b'c$e*f",
			"--expect-sha256", "deadbeefcafe", "--agent", "zerg-deploy-agent.sh",
			"--unit", "u.service", "--target", "/opt/x y", "--expect-version", "1\t2",
			"--probe-url", "http://h/p?a=1&b=2#frag",
			"--launch-dir", filepath.Join(tmp, "d-quote"), "--dry-run"}, nil, 0, false},
		{"mkdir-fail", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(afile, "sub"), "--dry-run"}, nil, 2, false},
		{"agent0-readback-ok", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(tmp, "d-ok")},
			map[string]string{"DPL_STUB_READBACK": "1", "DPL_STUB_RB_JSON": goodRB}, 0, true},
		{"agent-nonzero", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(tmp, "d-rc4")},
			map[string]string{"DPL_STUB_AGENT_RC": "3", "DPL_STUB_READBACK": "1", "DPL_STUB_RB_JSON": goodRB}, 4, true},
		{"readback-fail", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(tmp, "d-rc5")},
			map[string]string{"DPL_STUB_READBACK": "0", "DPL_STUB_RB_RC": "255"}, 5, true},
		{"readback-badjson", []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
			"--launch-dir", filepath.Join(tmp, "d-bad")},
			map[string]string{"DPL_STUB_READBACK": "1", "DPL_STUB_RB_JSON": "not json{"}, 0, true},
	}
	knobs := []string{"DPL_STUB_AGENT_RC", "DPL_STUB_AGENT_STDOUT", "DPL_STUB_AGENT_STDERR",
		"DPL_STUB_READBACK", "DPL_STUB_RB_RC", "DPL_STUB_RB_JSON", "DPL_STUB_RB_STDERR"}
	for _, c := range cells {
		t.Run(c.name, func(t *testing.T) {
			// 先清空全部替身旋钮（免上一格残留），再落本格点名的。
			for _, k := range knobs {
				t.Setenv(k, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			// ★ 写路格：Go 半补 `--yes`（原件那一半不加 —— 底层脚本不认它）。
			goTail := c.tail
			if c.writeYes {
				goTail = append(append([]string{}, c.tail...), "--yes")
			}
			rcS, outS, errS := dplRunSH(t, script, c.tail...)
			rcG, outG, errG := dplRunGo(t, goTail...)
			rcW, outW, errW := dplRunGoGate(t, goTail...)
			if rcS != c.want {
				t.Errorf("原件 rc=%d（本格期望 %d）· stdout=%q stderr=%q", rcS, c.want, outS, errS)
			}
			if rcG != rcS || rcW != rcS {
				t.Errorf("rc 三态不同：原件=%d Go=%d 包装=%d", rcS, rcG, rcW)
			}
			if dplNormTS(outG) != dplNormTS(outS) || dplNormTS(outW) != dplNormTS(outS) {
				t.Errorf("stdout 三态不同：\n原件=%q\nGo  =%q\n包装=%q", outS, outG, outW)
			}
			if dplNormTS(errG) != dplNormTS(errS) {
				t.Errorf("stderr 两态不同：\n原件=%q\nGo  =%q", errS, errG)
			}
			if errW != errG {
				t.Errorf("包装入口 stderr 与 cmdDeployLaunch 不同：\n包装=%q\nGo  =%q", errW, errG)
			}
		})
	}
}

// TestDeployLaunchGuardFailClosed —— 写面 fail-closed **四态**（`GAP-20260927-140` ① · 门100 写面登记）：
// 裸跑 rc=2 **零写**（✗ 不建落点目录、✗ 不落意图件、✗ 不起 ssh）· `--dry-run` rc=0（**原件语义**：
// 写意图件 + 打印将执行的 ssh、不 ssh 不回读）· `--yes` rc=0（真跑 · stub ssh）· `--dry-run --yes` rc=2。
func TestDeployLaunchGuardFailClosed(t *testing.T) {
	dplStubSSHInstall(t)
	const goodRB = `{"result":"ok","result_code":0,"unit":{"name":"u.service"},"observed":{"live_exe_sha256":"deadbeef"}}`
	base := func(dir string) []string {
		return []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc", "--launch-dir", dir}
	}
	countIntents := func(dir string) int {
		t.Helper()
		ents, err := os.ReadDir(dir)
		if err != nil {
			return 0
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "intent-") && strings.HasSuffix(e.Name(), ".json") {
				n++
			}
		}
		return n
	}
	// ① 裸跑（不给 --yes）⇒ rc=2 · 零写。
	dir1 := filepath.Join(t.TempDir(), "d-bare")
	rc1, out1, _ := dplRunGo(t, base(dir1)...)
	if rc1 != 2 {
		t.Fatalf("裸跑 rc=%d（要 2 · 缺 --yes fail-closed）", rc1)
	}
	if out1 != "" {
		t.Errorf("裸跑 stdout 应为空（零写 · 判词走 stderr），得 %q", out1)
	}
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Errorf("裸跑建了落点目录 %s（要零写）", dir1)
	}
	if n := countIntents(dir1); n != 0 {
		t.Errorf("裸跑写了 %d 枚意图件（要零写）", n)
	}
	// ② `--dry-run` ⇒ rc=0（保留原件语义：写意图件 + 预览、不 ssh）。
	dir2 := filepath.Join(t.TempDir(), "d-dry")
	rc2, out2, err2 := dplRunGo(t, append(base(dir2), "--dry-run")...)
	if rc2 != 0 {
		t.Fatalf("`--dry-run` rc=%d（要 0）· stderr=%q", rc2, err2)
	}
	if countIntents(dir2) != 1 {
		t.Errorf("`--dry-run` 应写 1 枚意图件（原件语义）")
	}
	if !strings.Contains(out2, "DRY-RUN") {
		t.Errorf("`--dry-run` 应打印预览：%q", out2)
	}
	// ③ `--yes` ⇒ rc=0（真跑 · 意图件 + 回读件齐）。
	dir3 := filepath.Join(t.TempDir(), "d-yes")
	t.Setenv("DPL_STUB_READBACK", "1")
	t.Setenv("DPL_STUB_RB_JSON", goodRB)
	rc3, _, err3 := dplRunGo(t, append(base(dir3), "--yes")...)
	if rc3 != 0 {
		t.Fatalf("`--yes` rc=%d（要 0）· stderr=%q", rc3, err3)
	}
	if countIntents(dir3) != 1 {
		t.Errorf("`--yes` 应写 1 枚意图件")
	}
	// ④ `--dry-run --yes` 同给 ⇒ rc=2（共享判定 `dryRunYesConflict`）· 零写。
	dir4 := filepath.Join(t.TempDir(), "d-dy")
	rc4, out4, _ := dplRunGo(t, append(base(dir4), "--dry-run", "--yes")...)
	if rc4 != 2 {
		t.Fatalf("`--dry-run --yes` rc=%d（要 2 · 两道确认档自相矛盾）", rc4)
	}
	if out4 != "" {
		t.Errorf("同给档 stdout 应为空，得 %q", out4)
	}
	if _, err := os.Stat(dir4); !os.IsNotExist(err) {
		t.Errorf("同给档建了落点目录 %s（要零写）", dir4)
	}
}

// TestDeployLaunchHelpEqualsScriptLines2To30 —— 牙：`--help` 正文逐字节 = 原件 `awk 'NR>=2 && NR<=30'` 的输出。
func TestDeployLaunchHelpEqualsScriptLines2To30(t *testing.T) {
	script := dplRealScript(t)
	raw, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) < 31 {
		t.Fatalf("原件行数不足（%d）", len(lines))
	}
	want := strings.Join(lines[1:30], "\n") + "\n"
	rc, out, _ := dplRunGo(t, "--help")
	if rc != 0 {
		t.Fatalf("`--help` rc=%d（要 0）", rc)
	}
	if out != want {
		t.Errorf("`--help` 正文与原件第 2..30 行不同：\nGo  =%q\n原件=%q", out, want)
	}
}

// TestDeployLaunchIntentArtifactVerbatim —— 落盘物也逐字节比（除 `at` 时刻与落点目录外）。
func TestDeployLaunchIntentArtifactVerbatim(t *testing.T) {
	script := dplRealScript(t)
	tmp := t.TempDir()
	dirSH, dirGO := filepath.Join(tmp, "sh"), filepath.Join(tmp, "go")
	tailFor := func(d string) []string {
		return []string{"--host", "x" + "3", "--file", "/tmp/f.bin", "--expect-sha256", "deadbeefcafe",
			"--agent", "zerg-deploy-agent.sh", "--unit", "u.service", "--target", "/opt/x",
			"--expect-version", "1.2.3", "--probe-url", "http://h/p", "--launch-dir", d, "--dry-run"}
	}
	if rc, _, errS := dplRunSH(t, script, tailFor(dirSH)...); rc != 0 {
		t.Fatalf("原件 rc=%d · %s", rc, errS)
	}
	if rc, _, errG := dplRunGo(t, tailFor(dirGO)...); rc != 0 {
		t.Fatalf("Go rc=%d · %s", rc, errG)
	}
	read := func(dir string) string {
		t.Helper()
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var pick string
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "intent-") && strings.HasSuffix(e.Name(), ".json") {
				pick = filepath.Join(dir, e.Name())
			}
		}
		if pick == "" {
			t.Fatalf("落点没有意图件：%s", dir)
		}
		b, err := os.ReadFile(pick)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	norm := func(s, dir string) string {
		s = regexp.MustCompile(`"at": "\d{8}T\d{6}Z"`).ReplaceAllString(s, `"at": "<TS>"`)
		return strings.ReplaceAll(s, dir, "<DIR>")
	}
	got, want := norm(read(dirGO), dirGO), norm(read(dirSH), dirSH)
	if got != want {
		t.Errorf("意图件落盘内容不同：\nGo  =%q\n原件=%q", got, want)
	}
}

// TestDeployLaunchDoesNotExecScript —— 牙（真搬的硬证）：原件换成「写标记」的桩 ⇒ 标记不得出现，
// 且结果与原件无关（本件自足）。
func TestDeployLaunchDoesNotExecScript(t *testing.T) {
	root := t.TempDir()
	sp := filepath.Join(root, deployLaunchScriptRel)
	if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sp, []byte("#!/usr/bin/env bash\necho STUB-EXECUTED\nexit 42\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	rc, out, errb := dplRunGo(t, "--host", "x"+"3", "--file", "/tmp/f", "--expect-sha256", "abc",
		"--launch-dir", filepath.Join(root, "d"), "--dry-run")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if strings.Contains(out, "STUB-EXECUTED") {
		t.Errorf("Go 半仍在 exec 原件（桩的标记出现在 stdout）：%q", out)
	}
}

// TestDeployLaunchRetiredStateSameAsPresent —— **退役态预检**：把仓根指到「没有原件」的树 ⇒
// 读数与在盘时**逐字同**（✗ 不得 rc=2）。
func TestDeployLaunchRetiredStateSameAsPresent(t *testing.T) {
	dplRealScript(t) // 原件不在 ⇒ Skip（本档只在真件在盘时有意义）
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "d")
	tail := []string{"--host", "x" + "3", "--file", "/tmp/f", "--expect-sha256", "abc",
		"--launch-dir", dir, "--dry-run"}
	rcP, outP, errP := dplRunGo(t, tail...)
	t.Setenv("ZERG_REPO", t.TempDir()) // 指到没有 deploy/scripts/deploy-launch.sh 的树
	rcA, outA, errA := dplRunGo(t, tail...)
	if rcA != rcP {
		t.Errorf("退役态 rc=%d ⟷ 在盘 rc=%d（要相同 · ✗ 不得 2）", rcA, rcP)
	}
	if outA != outP || errA != errP {
		t.Errorf("退役态读数与在盘不同：\n在盘 out=%q err=%q\n退役 out=%q err=%q", outP, errP, outA, errA)
	}
}

// TestDeployLaunchBashQMatchesBash —— 牙：件内 `dplBashQ` 与真 bash `printf '%q'` 逐字对拍（ASCII 语料）。
func TestDeployLaunchBashQMatchesBash(t *testing.T) {
	corpus := []string{"", "abc", "a b", `a"b`, "a'b", "a$b", `a\b`, "ca*p", "-x", "a,b", "a;b",
		"a=b", "x" + "3-agent.service", "/opt/x y", "http://h/p?a=1&b=2", "a\nb", "a\tb", "#a", "a#",
		"#", "##", "user@<worker-ip>", "deadbeefcafe", "1\t2", "a!b", "a~b", "~", "a b'c$e*f"}
	for _, s := range corpus {
		out, err := exec.Command("bash", "-c", `printf %q "$1"`, "_", s).Output()
		if err != nil {
			t.Fatalf("bash 起不来：%v", err)
		}
		if got := dplBashQ(s); got != string(out) {
			t.Errorf("dplBashQ(%q)=%q ⟷ bash %q", s, got, string(out))
		}
	}
}

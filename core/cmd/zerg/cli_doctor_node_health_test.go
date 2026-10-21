// cli_doctor_node_health_test.go —— `zerg doctor node --machine <机>`（缺口 `GAP-20260926-194`）的回归牙。
//
// 判据（四条正控 + 四条负控，全在**合成夹具**上跑；✗ 不 ssh 真机、✗ 不写真状态目录）：
//
//	正控 A：读数齐 ⇒ 退 0 · 五格全印 · 判定 PASS（模型名/端口/卵/磁盘逐格来自探针读数）；
//	正控 B：结构性缺席（无 build-info · 无模型 · 无卵）⇒ 每格照实印「未接」· 退 0（只报告不判红）；
//	正控 C：探针脚本只含**只读**命令（现读 stdin，逐个禁止词断言）—— 只读纪律的机读牙；
//	正控 D：该机不走 systemd（名册 `os` 非 linux 族）⇒ 守护单元格印「未接」而不是判红（退 0）；
//	负控 ①：ssh 不可达 ⇒ 退 8 · 判定 BLOCKED · **一格读数都不印**（✗ 造假数据）；
//	负控 ②：守护单元 inactive（且单元存在）⇒ 退 1 · 判定 FAIL；
//	负控 ③：磁盘 ≥90% / agentd 端口没听 ⇒ 退 1；
//	负控 ④：探针半截（缺 `@@END`）⇒ 解析判「没跑完」（Complete=false ⇒ 调用方按读不到办）；
//	负控 ⑤：机名不在名册 ⇒ 退 2（用法错 · 带最像的合法输入）· **一个探针都不发**；
//	负控 ⑥：吃位置参数 ⇒ 退 2（机名只走 `--machine`）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dxhMachine —— 夹具机名（**合成名**：本件不进公开面判据的私有模式面，但机名一律用合成名，
// ✗ 不把任何真机名写进测试件 —— 真机名属运行期数据）。
const dxhMachine = "node-a"

// dxhStubSSH —— 测试用 `ssh` 替身：把 stdin（探针脚本）落到 $DXH_STDIN，再吐 $DXH_OUT、退 $DXH_RC。
const dxhStubSSH = `#!/usr/bin/env bash
if [ -n "${DXH_STDIN:-}" ]; then cat > "$DXH_STDIN"; fi
printf '%s' "${DXH_OUT:-}"
printf '%s' "${DXH_ERR:-}" >&2
exit "${DXH_RC:-0}"
`

// dxhInstall —— 装替身 + 造仓外名册根（`ZERG_REPO` 指它）。
func dxhInstall(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(dxhStubSSH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DXH_STDIN", filepath.Join(dir, "stdin.sh"))

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "fleet:\n" +
		"  " + dxhMachine + ": { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }\n" +
		"updates:\n" +
		"  check: true\n" +
		"  nodes:\n" +
		"    - { name: " + dxhMachine + ", ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"\", components: agentd }\n"
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	return root
}

// dxhSetProbe —— 让替身吐出这一份探针输出（rc 可调）。
func dxhSetProbe(t *testing.T, out string, rc int) {
	t.Helper()
	t.Setenv("DXH_OUT", out)
	t.Setenv("DXH_RC", itoaForTest(rc))
}

// itoaForTest —— 小数转串（避开 strconv 的 import 争用，只用于夹具 rc）。
func itoaForTest(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if neg {
		return "-" + digits
	}
	return digits
}

// dxhRun —— 跑一次 `doctor node --machine <夹具机>`，返回 (rc, stdout, stderr)。
func dxhRun(t *testing.T) (int, string, string) {
	t.Helper()
	inv := &invocation{kv: map[string][]string{"--machine": {dxhMachine}}}
	var out, errb bytes.Buffer
	rc := cmdDoctorNode(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// dxhHealthy —— 一份「读数齐」的探针输出（数字取自 2026-10-21 真机现读的形状 · 机名用夹具合成名）。
func dxhHealthy() string {
	return strings.Join([]string{
		"@@VER",
		`BUILDINFO {"version":"2.5.9","commit":"64952f2+dirty","build_time":"2026-09-13T13:43:35Z","builder_host":"Linux/x86_64"}`,
		"HEAD 2888ccaa2457",
		"DIRTY 2",
		"AGENTD_SHA 663f660af239",
		"@@UNITS",
		"UNIT " + dxhMachine + "-agent.service active yes",
		"@@PORTS",
		"22",
		"8100",
		"9400",
		"@@MODELS",
		"2050263 /engine/llama-server -m /models/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-Q4_0.gguf -c 1048576 --port 9400 --jinja",
		"@@EGGS",
		"DeepSeek-V4-Flash-Vision-Exp",
		"Nemotron-3.5-Super-VL-120B-A12B",
		"EGG_RUN Nemotron-3.5-Super-VL-120B-A12B ",
		"@@DISK",
		"Filesystem 1024-blocks Used Available Capacity Mounted on",
		"/dev/nvme0n1p6 284607820 35830000 226000000 14% /",
		"/dev/nvme0n1p7 2803372800 1657300000 1005000000 63% /data",
		"@@END",
		"",
	}, "\n")
}

// 正控 A：读数齐 ⇒ 退 0、五格齐、判定 PASS。
func TestDoctorNodeHealthAllGridsFromProbe(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, dxhHealthy(), 0)
	rc, out, errs := dxhRun(t)
	if rc != 0 {
		t.Fatalf("rc = %d，want 0（stderr=%s）", rc, errs)
	}
	for _, must := range []string{
		"版本/sha", "构建件 2.5.9", "commit 64952f2+dirty", "源码 HEAD 2888ccaa2457", "工作树脏 2 件", "zerg-agentd sha256 663f660af239",
		"单元状态", dxhMachine + "-agent.service active",
		"端口", "22 sshd 监听", "8100 zerg-agentd·名册端口 监听", "9400 llama-server·读数端口 监听",
		"模型/卵", "模型在跑 1 个", "NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-Q4_0.gguf",
		"卵 2 只", "Nemotron-3.5-Super-VL-120B-A12B（在跑）", "DeepSeek-V4-Flash-Vision-Exp（目录·未跑）",
		"磁盘", "/ 14%", "/data 63%",
		"判定     : PASS",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("输出缺 %q\n--- stdout ---\n%s", must, out)
		}
	}
}

// 正控 B：结构性缺席 ⇒ 每格照实印「未接」，退 0（只报告、不判红）。
func TestDoctorNodeHealthUnwiredGridsSayWeiJie(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, strings.Join([]string{
		"@@VER",
		"HEAD 2888ccaa2457",
		"DIRTY 2",
		"@@UNITS",
		"UNIT " + dxhMachine + "-agent.service active yes",
		"@@PORTS",
		"22",
		"8100",
		"@@MODELS",
		"@@EGGS",
		"EGG_RUN ",
		"@@DISK",
		"/dev/nvme0n1p6 284607820 35830000 226000000 14% /",
		"@@END",
		"",
	}, "\n"), 0)
	rc, out, errs := dxhRun(t)
	if rc != 0 {
		t.Fatalf("rc = %d，want 0（未接只报告不判红 · stderr=%s）", rc, errs)
	}
	for _, must := range []string{
		"版本 未接（" + dxhMachine + " 上 `VERSION` 与 `bin/build-info.json` 现读不存在/读不出",
		"zerg-agentd sha256 未接",
		"9400 llama-server 未接（该机当前没在跑模型进程）",
		"模型在跑 0 个（未接：无 llama-server 进程",
		"卵 未接（`~/.zerg/work` 空或读不到",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("输出缺「未接」读数 %q\n--- stdout ---\n%s", must, out)
		}
	}
	if strings.Contains(out, "判定     : FAIL") {
		t.Fatalf("「未接」不该判红：\n%s", out)
	}
}

// 正控 C：探针脚本只含只读命令（禁词逐个断言 —— 只读纪律的机读牙）。
func TestDoctorNodeHealthProbeScriptIsReadOnly(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, dxhHealthy(), 0)
	if _, _, _ = dxhRun(t); true {
		b, err := os.ReadFile(os.Getenv("DXH_STDIN"))
		if err != nil {
			t.Fatalf("替身没收到探针脚本（stdin 未落盘）：%v", err)
		}
		script := string(b)
		if !strings.Contains(script, "git -C \"$R\" --no-optional-locks status") {
			t.Fatalf("探针脚本少了 `--no-optional-locks`（`git status` 会刷新索引 = 写私有仓）:\n%s", script)
		}
		for _, banned := range []string{"rm -", "systemctl start", "systemctl restart", "systemctl stop", "sudo ", "mv ", "tee ", "> \"$R", ">> \"$R"} {
			if strings.Contains(script, banned) {
				t.Fatalf("探针脚本里出现写面命令 %q —— 本命令必须只读:\n%s", banned, script)
			}
		}
	}
}

// 正控 D：该机不走 systemd（`os` = macos）⇒ 守护单元格「未接」，✗ 判红、✗ 退 1。
// 判据取「同一份探针读数（单元 found=no）只换名册 os」的两态：linux ⇒ FAIL，macos ⇒ PASS/REPORT。
func TestDoctorNodeHealthNonSystemdHostIsReportNotFail(t *testing.T) {
	probe := strings.Replace(dxhHealthy(), "UNIT "+dxhMachine+"-agent.service active yes",
		"UNIT "+dxhMachine+"-agent.service unknown no", 1)
	for _, tc := range []struct {
		osName string
		wantRC int
		want   string
	}{
		{"ubuntu", exitFail, "判定     : FAIL"},
		{"macos", 0, "未接（该机不走 systemd · 本版不探 launchd）"},
	} {
		root := dxhInstall(t)
		fleet := filepath.Join(root, "gateway", "fleet.yaml")
		b, err := os.ReadFile(fleet)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fleet, []byte(strings.Replace(string(b), "os: ubuntu", "os: "+tc.osName, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		dxhSetProbe(t, probe, 0)
		rc, out, _ := dxhRun(t)
		if rc != tc.wantRC {
			t.Fatalf("os=%s：rc = %d，want %d\n%s", tc.osName, rc, tc.wantRC, out)
		}
		if !strings.Contains(out, tc.want) {
			t.Fatalf("os=%s：输出缺 %q\n%s", tc.osName, tc.want, out)
		}
	}
}

// 负控 ①：ssh 不可达 ⇒ 退 8 · 判定 BLOCKED · 一格读数都不印。
func TestDoctorNodeHealthUnreachableIsBlockedNoFakeData(t *testing.T) {
	dxhInstall(t)
	t.Setenv("DXH_ERR", "ssh: connect to host <worker-ip> port 22: Operation timed out\n")
	dxhSetProbe(t, "", 255)
	rc, out, errs := dxhRun(t)
	if rc != exitBlocked {
		t.Fatalf("rc = %d，want %d（读不到 ⇒ 不给结论）", rc, exitBlocked)
	}
	if !strings.Contains(out, "判定     : BLOCKED") {
		t.Fatalf("输出缺 BLOCKED 判定:\n%s", out)
	}
	for _, banned := range []string{"监听", "构建件", "源码 HEAD", "卵 ", "磁盘"} {
		if strings.Contains(out, banned) {
			t.Fatalf("读不到时印了读数 %q（= 造假数据）:\n%s", banned, out)
		}
	}
	if !strings.Contains(errs, "退码 8") {
		t.Fatalf("stderr 没点名退码 8：%s", errs)
	}
}

// 负控 ②：守护单元 inactive（单元存在）⇒ 退 1 · 判定 FAIL。
func TestDoctorNodeHealthUnitDeadIsFail(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, strings.Replace(dxhHealthy(),
		"UNIT "+dxhMachine+"-agent.service active yes",
		"UNIT "+dxhMachine+"-agent.service failed yes", 1), 0)
	rc, out, _ := dxhRun(t)
	if rc != exitFail {
		t.Fatalf("rc = %d，want %d（单元没在跑 = 真坏）", rc, exitFail)
	}
	if !strings.Contains(out, dxhMachine+"-agent.service failed") || !strings.Contains(out, "判定     : FAIL") {
		t.Fatalf("没判红 / 没点名单元状态:\n%s", out)
	}
}

// 负控 ③a：磁盘 ≥90% ⇒ 退 1。
func TestDoctorNodeHealthDiskAlmostFullIsFail(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, strings.Replace(dxhHealthy(), "/dev/nvme0n1p7 2803372800 1657300000 1005000000 63% /data",
		"/dev/nvme0n1p7 2803372800 2657300000 1005000000 95% /data", 1), 0)
	rc, out, _ := dxhRun(t)
	if rc != exitFail {
		t.Fatalf("rc = %d，want %d（磁盘 95%% 该判红）", rc, exitFail)
	}
	if !strings.Contains(out, "/data 95%") {
		t.Fatalf("没印出 95%% 读数:\n%s", out)
	}
}

// 负控 ③b：名册端口（agentd）没在听 ⇒ 退 1。
func TestDoctorNodeHealthAgentdPortMissingIsFail(t *testing.T) {
	dxhInstall(t)
	bad := strings.Replace(dxhHealthy(), "8100\n", "", 1)
	dxhSetProbe(t, bad, 0)
	rc, out, _ := dxhRun(t)
	if rc != exitFail {
		t.Fatalf("rc = %d，want %d（该听的端口没听 = 真坏）", rc, exitFail)
	}
	if !strings.Contains(out, "8100 zerg-agentd·名册端口 未监听") {
		t.Fatalf("没点名「8100 未监听」:\n%s", out)
	}
}

// 负控 ④：探针半截（缺 `@@END`）⇒ 解析判「没跑完」。
func TestDoctorNodeHealthIncompleteProbeIsNotComplete(t *testing.T) {
	p := dxxParseProbe("@@VER\nHEAD abc123\nDIRTY 1\n@@UNITS\nUNIT " + dxhMachine + "-agent.service active yes\n")
	if p.Complete {
		t.Fatal("缺 `@@END` 却判成跑完了 ⇒ 半截读数会被当整份用")
	}
	if p.Head != "abc123" || p.Dirty != 1 {
		t.Fatalf("半截读数的已知格也没解析对：head=%q dirty=%d", p.Head, p.Dirty)
	}
}

// 负控 ⑤：机名不在名册 ⇒ 退 2（用法错）· 一个探针都不发。
func TestDoctorNodeHealthUnknownMachineIsUsage(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, dxhHealthy(), 0)
	inv := &invocation{kv: map[string][]string{"--machine": {"no-such-node-zz"}}}
	var out, errb bytes.Buffer
	rc := cmdDoctorNode(inv, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("rc = %d，want %d（名册里没有这台 = 用法错）", rc, exitUsage)
	}
	if out.Len() != 0 {
		t.Fatalf("用法错不许吐读数：%q", out.String())
	}
	if _, err := os.Stat(os.Getenv("DXH_STDIN")); err == nil {
		t.Fatal("机名不认识却发了探针（探针脚本已落盘）—— 入口门必须挡在探针之前")
	}
}

// 负控 ⑥：吃位置参数 ⇒ 退 2（机名只走 `--machine`）。
func TestDoctorNodeHealthRejectsPositional(t *testing.T) {
	dxhInstall(t)
	dxhSetProbe(t, dxhHealthy(), 0)
	inv := &invocation{args: []string{"whatever"}}
	var out, errb bytes.Buffer
	rc := cmdDoctorNode(inv, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("rc = %d，want %d（本命令不吃位置参数）", rc, exitUsage)
	}
	if !strings.Contains(errb.String(), "--machine") {
		t.Fatalf("stderr 没把机名出口指给 `--machine`：%s", errb.String())
	}
}

// 附：判定词聚合表（FAIL > BLOCKED > REPORT > PASS）。
func TestDoctorNodeHealthVerdictRank(t *testing.T) {
	cases := [][3]string{
		{"PASS", "REPORT", "REPORT"},
		{"REPORT", "BLOCKED", "BLOCKED"},
		{"BLOCKED", "FAIL", "FAIL"},
		{"FAIL", "REPORT", "FAIL"},
	}
	for _, c := range cases {
		if got := dxxGrow(c[0], c[1]); got != c[2] {
			t.Fatalf("dxxGrow(%q,%q) = %q，want %q", c[0], c[1], got, c[2])
		}
	}
}

// 附：`dxxPadName` 只补空格、不改字（对齐面不许影响判定值）。
func TestDoctorNodeHealthPadNameKeepsText(t *testing.T) {
	for _, s := range []string{"版本/sha", "单元状态", "端口", "模型/卵", "磁盘"} {
		got := dxxPadName(s)
		if strings.TrimRight(got, " ") != s {
			t.Fatalf("dxxPadName(%q) = %q —— 补位只能加空格", s, got)
		}
	}
}

// 附：机名解析 —— 本机候选名先到名册里对（名册是唯一口径），对不上回落第一候选。
// ★ 判据落点：`dxxLocalMachine` 的返回（✗ 只看「有没有调到」）。
func TestDoctorNodeHealthLocalMachinePrefersRosterName(t *testing.T) {
	root := dxhInstall(t)
	t.Setenv("USER", dxhMachine)
	if got := dxxLocalMachine(root); got != dxhMachine {
		t.Fatalf("dxxLocalMachine = %q，want %q（登录名命中名册就该用它）", got, dxhMachine)
	}
	t.Setenv("USER", "no-such-user-zz")
	got := dxxLocalMachine(root)
	if got != "no-such-user-zz" {
		t.Fatalf("dxxLocalMachine = %q，want 回落第一候选 %q", got, "no-such-user-zz")
	}
	aliases := dxxLocalAliases()
	if len(aliases) < 1 {
		t.Fatalf("dxxLocalAliases 空转：候选名 %d 个 ⇒ 低于下限 1（登录名与主机名都取不到 ⇒ 缺省机名解析不出来 = 不给结论）", len(aliases))
	}
}

// 附：名册**行内映射**取值的引号口径 —— `components: "core,agentd"` 的逗号在引号内，
// ✗ 不是字段分隔（按逗号硬切会得到 `"core` 这种半截值 ⇒ 单元格整面塌成 8）。
func TestDoctorNodeHealthInlineFieldsKeepQuotedComma(t *testing.T) {
	kv := dxxInlineFields(`{ name: node-a, ssh: g01@<worker-ip>, components: "core,agentd", prefix: "/x/bin" }`)
	if kv["components"] != "core,agentd" {
		t.Fatalf("components = %q，want %q（引号内的逗号是取值的一部分）", kv["components"], "core,agentd")
	}
	if kv["name"] != "node-a" || kv["ssh"] != "g01@<worker-ip>" {
		t.Fatalf("同形字段读歪：%v", kv)
	}
	if kv["prefix"] != "/x/bin" {
		t.Fatalf("prefix = %q（引号是 YAML 语法、不是取值）", kv["prefix"])
	}
	// 负控：引号不成对 ⇒ 原样（✗ 猜着剥）
	if got := dxxInlineFields(`{ name: node-a, prefix: "/x }`)["prefix"]; got != `"/x` {
		t.Fatalf("不成对引号被猜掉了：%q", got)
	}
}

// 正控 E：`components: "core,agentd"` 的两枚单元都要探（引号内逗号不许把单元格吃掉）。
func TestDoctorNodeHealthQuotedComponentsProbeBothUnits(t *testing.T) {
	root := dxhInstall(t)
	fleet := filepath.Join(root, "gateway", "fleet.yaml")
	b, err := os.ReadFile(fleet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fleet, []byte(strings.Replace(string(b), "components: agentd }", `components: "core,agentd" }`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	dxhSetProbe(t, strings.Join([]string{
		"@@VER", "HEAD 2888ccaa2457", "DIRTY 0",
		"@@UNITS",
		"UNIT zerg-core.service active yes",
		"UNIT " + dxhMachine + "-agent.service active yes",
		"@@PORTS", "22", "8100",
		"@@MODELS", "@@EGGS", "EGG_RUN ",
		"@@DISK", "/dev/nvme0n1p6 284607820 35830000 226000000 14% /",
		"@@END", "",
	}, "\n"), 0)
	rc, out, errs := dxhRun(t)
	if rc != 0 {
		t.Fatalf("rc = %d，want 0（stderr=%s）", rc, errs)
	}
	for _, must := range []string{"zerg-core.service active", dxhMachine + "-agent.service active"} {
		if !strings.Contains(out, must) {
			t.Fatalf("输出缺 %q —— 名册两枚组件都该被探\n%s", must, out)
		}
	}
}

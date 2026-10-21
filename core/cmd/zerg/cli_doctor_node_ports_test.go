// cli_doctor_node_ports_test.go —— `zerg doctor node` **端口格**的回归牙（跨平台探针形态 ＋ 监听行解析）。
//
// 病根（2026-10-21 实测 · 缺口 `GAP-20260926-194`）：探针里端口一律走 Linux 的 `ss -ltn`（`netstat -ltn`
// 兜底），而 macOS 上**没有 `ss`**，BSD 的 `netstat -ltn` 又不按「只听」过滤（吐整张 ESTABLISHED 表）⇒
// 同刻 `lsof -nP -iTCP:8100 -sTCP:LISTEN` 明明在听，体检却判「8100 未监听」、整行假 FAIL。
//
// 本件的牙（四条，全在**合成夹具**上跑 · ✗ ssh 真机、✗ 写状态目录、✗ 碰真名册）：
//
//	① 形态牙：名册 `os` = macos ⇒ **真发出去的探针脚本**里不许出现 `ss`，必有
//	   `lsof -nP -iTCP -sTCP:LISTEN` 与 `netstat -an -p tcp`；`os` = ubuntu ⇒ 仍走 `ss -ltnH`（Linux 面不退化）。
//	   并把发出去的脚本过一遍 `sh -n` —— 第三个 `%s` 接错引号要在这一关当场红。
//	② 解析牙：`lsof` 整行 / lsof 剥过的地址 token / BSD netstat 点分行 / `ss` 整行四种样本文本直接喂
//	   `dxxParseProbe`，逐口径认得出端口；PID、计数、通配口（`0.0.0.0:*`）**不许**被读成端口（负控）。
//	③ 判定牙：macOS 那台（8100 在听）⇒ 端口格印「8100 … 监听」、判定不因端口格 FAIL、退出码 0。
//	④ 一致牙：端口格真 FAIL 时，人面「判定: FAIL」与**退出码**必须同一张皮（`exitFail`），✗ 不许印 FAIL 还退 0。
//
// ★ 本件自带夹具（`dxp*` 前缀），✗ 不依赖同包别路在途的测试件 —— 那份随时可能被别路整件重落。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// dxpMachine —— 夹具机名（合成名：真机名属运行期数据，✗ 不落进件内）。
const dxpMachine = "node-b"

// dxpStubSSH —— `ssh` 替身：把 stdin（探针脚本）落到 $DXP_STDIN，再吐 $DXP_OUT、退 $DXP_RC。
const dxpStubSSH = `#!/usr/bin/env bash
if [ -n "${DXP_STDIN:-}" ]; then cat > "$DXP_STDIN"; fi
printf '%s' "${DXP_OUT:-}"
printf '%s' "${DXP_ERR:-}" >&2
exit "${DXP_RC:-0}"
`

// dxpInstall —— 装替身 ＋ 造仓外名册根（`ZERG_REPO` 指它；名册该机的 `os` 按参数写）。
func dxpInstall(t *testing.T, osName string) string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(dxpStubSSH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DXP_STDIN", filepath.Join(bin, "stdin.sh"))

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "fleet:\n" +
		"  " + dxpMachine + ": { host: <cluster-ip>, port: 8100, os: " + osName + ", kind: ai }\n" +
		"updates:\n" +
		"  check: true\n" +
		"  nodes:\n" +
		"    - { name: " + dxpMachine + ", ssh: g01@<cluster-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"\", components: agentd }\n"
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	return root
}

// dxpSetProbe —— 让替身吐出这一份探针输出（rc 可调）。
func dxpSetProbe(t *testing.T, out string, rc int) {
	t.Helper()
	t.Setenv("DXP_OUT", out)
	if rc == 0 {
		t.Setenv("DXP_RC", "0")
		return
	}
	t.Setenv("DXP_RC", "1")
}

// dxpRun —— 跑一次 `doctor node --machine <夹具机>`，返回 (rc, stdout, stderr)。
func dxpRun(t *testing.T) (int, string, string) {
	t.Helper()
	inv := &invocation{kv: map[string][]string{"--machine": {dxpMachine}}}
	var out, errb bytes.Buffer
	rc := cmdDoctorNode(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// dxpHealthyProbe —— 一份「该听的都在听」的探针输出；端口按 darwin 面写（`*.22` 是 BSD netstat 点分写法）。
// 单元一栏给 `active yes`（两 OS 族都判 PASS）—— 本件只测端口面，别让单元格掺进判定。
func dxpHealthyProbe() string {
	return strings.Join([]string{
		"@@VER",
		`BUILDINFO {"version":"2.5.9","commit":"64952f2","build_time":"2026-09-13T13:43:35Z","builder_host":"Darwin/arm64"}`,
		"HEAD 2888ccaa2457",
		"DIRTY 0",
		"AGENTD_SHA 663f660af239",
		"@@UNITS",
		"UNIT " + dxpMachine + "-agent.service active yes",
		"@@PORTS",
		"*.22",
		"127.0.0.1:8100",
		"@@MODELS",
		"@@EGGS",
		"EGG_RUN ",
		"@@DISK",
		"/dev/disk3s1s1 971350180 43000000 900000000 3% /",
		"@@END",
		"",
	}, "\n")
}

// ① 形态牙：探针脚本按名册 `os` 选形态（macOS 上绝不许出现 `ss`）。
func TestDoctorNodePortProbeScriptFollowsRosterOS(t *testing.T) {
	cases := []struct {
		osName string
		want   []string
		forbid []string
	}{
		{
			osName: "macos",
			want:   []string{"lsof -nP -iTCP -sTCP:LISTEN", `netstat -an -p tcp`, `$NF == "LISTEN"`},
			forbid: []string{"ss -ltn", "ss -ltnH"},
		},
		{
			osName: "ubuntu",
			want:   []string{"ss -ltnH", "netstat -ltn"},
			forbid: []string{"lsof"},
		},
	}
	for _, tc := range cases {
		dxpInstall(t, tc.osName)
		dxpSetProbe(t, dxpHealthyProbe(), 0)
		if rc, _, errs := dxpRun(t); rc != exitOK {
			t.Fatalf("os=%s：rc = %d，want %d（stderr=%s）", tc.osName, rc, exitOK, errs)
		}
		b, err := os.ReadFile(os.Getenv("DXP_STDIN"))
		if err != nil {
			t.Fatalf("os=%s：替身没收到探针脚本（stdin 未落盘）：%v", tc.osName, err)
		}
		script := string(b)
		for _, w := range tc.want {
			if !strings.Contains(script, w) {
				t.Fatalf("os=%s：发出去的探针脚本缺 %q —— 端口探针没按名册 `os` 选形态（macOS 没有 `ss` ⇒ 该听的端口永远判「未监听」= 假 FAIL）:\n%s",
					tc.osName, w, script)
			}
		}
		for _, f := range tc.forbid {
			if strings.Contains(script, f) {
				t.Fatalf("os=%s：发出去的探针脚本里出现 %q —— 该 OS 上没有这件工具（探针形态选错了）:\n%s",
					tc.osName, f, script)
			}
		}
		// 第三个 `%s` 接进来的那段是 shell 片段，语法要过 `sh -n`（接错引号会让整份探针脚本不跑 ⇒ 整面板 BLOCKED）。
		if out, err := exec.Command("sh", "-n", os.Getenv("DXP_STDIN")).CombinedOutput(); err != nil {
			t.Fatalf("os=%s：探针脚本 shell 语法不过（%v）：%s", tc.osName, err, out)
		}
	}
}

// ② 解析牙：lsof / BSD netstat / ss / 裸数字四种写法都认得出端口，非端口一律不认。
func TestDoctorNodePortParseAcceptsLsofAndBSDNetstat(t *testing.T) {
	raw := strings.Join([]string{
		"@@PORTS",
		// lsof 整行（darwin 面原样）：表头 + 正常行 + 通配行
		"COMMAND     PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME",
		"zerg-agen 73075 Mr2109    7u  IPv4 0x9a1b2c3d4e5f       0t0  TCP 127.0.0.1:8100 (LISTEN)",
		"rapportd    391 Mr2109    8u  IPv6 0x1234567890ab       0t0  TCP *:51571 (LISTEN)",
		// lsof 被探针剥掉 `(LISTEN)` 之后交出来的那一枚地址 token（真形态）
		"127.0.0.1:9400",
		// BSD `netstat -an -p tcp` 的点分写法：IPv4 / 通配 / IPv6
		"tcp4       0      0  *.22                   *.*                    LISTEN",
		"tcp4       0      0  127.0.0.1.8082         0.0.0.0.*              LISTEN",
		"tcp6       0      0  ::1.8021               *.*                    LISTEN",
		// Linux `ss -ltnH` 整行（拿 `$4` 之前与之后两态都验）
		"LISTEN 0      4096               127.0.0.1:8580            0.0.0.0:*",
		"0.0.0.0:7895",
		"@@END",
		"",
	}, "\n")
	p := dxxParseProbe(raw)
	for _, want := range []int{8100, 51571, 9400, 22, 8082, 8021, 8580, 7895} {
		if !p.Ports[want] {
			t.Fatalf("端口 %d 没被认出来 —— lsof / BSD netstat / ss 有哪一种写法漏了？读数=%v\n样本文本:\n%s",
				want, p.Ports, raw)
		}
	}
	// 负控：PID、计数、表头分词、通配口都不许被读成端口（右往左取「第一枚能解出端口的 token」）。
	for _, banned := range []int{73075, 391, 4096, 0, 1} {
		if p.Ports[banned] {
			t.Fatalf("把非端口 %d 读成端口了（PID / 计数 / 通配口一律不算）—— 读数=%v\n样本文本:\n%s",
				banned, p.Ports, raw)
		}
	}
}

// ③ 判定牙：darwin 那台 8100 真在听 ⇒ 端口格印「监听」、判定不因它 FAIL、退 0。
func TestDoctorNodePortCellDarwinListeningIsNotFail(t *testing.T) {
	dxpInstall(t, "macos")
	dxpSetProbe(t, dxpHealthyProbe(), 0)
	rc, out, errs := dxpRun(t)
	if !strings.Contains(out, "8100 zerg-agentd·名册端口 监听") {
		t.Fatalf("端口格没印「8100 zerg-agentd·名册端口 监听」—— 探针形态没按 darwin 面切？\nstdout:\n%s\nstderr:\n%s", out, errs)
	}
	if strings.Contains(out, "8100 zerg-agentd·名册端口 未监听") {
		t.Fatalf("8100 在听却判「未监听」= 本缺口本身:\n%s", out)
	}
	if strings.Contains(out, "判定     : FAIL") {
		t.Fatalf("端口格读数健康，判定却 FAIL（端口格不该是 FAIL 来源）:\n%s", out)
	}
	if rc != exitOK {
		t.Fatalf("rc = %d，want %d（端口格不 FAIL ⇒ 不许因它退非 0）", rc, exitOK)
	}
}

// ④ 一致牙：端口格真 FAIL 时，人面「判定: FAIL」与退出码同一张皮。
func TestDoctorNodePortFailVerdictMatchesExitCode(t *testing.T) {
	dxpInstall(t, "macos")
	// 同一份读数只把 8100 那一枚挪开（该听的没听 = 真坏）。
	probe := strings.Replace(dxpHealthyProbe(), "127.0.0.1:8100", "127.0.0.1:8101", 1)
	dxpSetProbe(t, probe, 0)
	rc, out, _ := dxpRun(t)
	if !strings.Contains(out, "8100 zerg-agentd·名册端口 未监听") {
		t.Fatalf("负控没生效：端口格没印「8100 … 未监听」\n%s", out)
	}
	if !strings.Contains(out, "判定     : FAIL") {
		t.Fatalf("该听的端口没听，判定却不是 FAIL:\n%s", out)
	}
	if rc != exitFail {
		t.Fatalf("人面印 FAIL 而退出码 = %d（want %d）—— 判定行与退码不许两张皮（调用方只读退码）", rc, exitFail)
	}
}

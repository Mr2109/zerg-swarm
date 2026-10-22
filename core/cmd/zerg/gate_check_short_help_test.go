// gate_check_short_help_test.go —— 「`zerg gate check <判据名> -h` 被命令树截成族帮助」的机检。
//
// 判据（六格 · 落仓态 = post）：
//
//	① `gate check check-newfile-registry -h` —— **子件 usage**（首行 `usage: check-newfile-registry.py`）· rc=0。
//	② `gate check check-newfile-registry --help` —— **族帮助**（既有行为一字不动）。
//	③ `gate check -h`（**没点名**子件）—— 族帮助（不把 `-h` 丢给空气）。
//	④ 非透传命令 `gap ls -h` —— 该条族帮助（本判据只动点名的透传档）。
//	⑤ `gate run <门> -h`（**没开** childHelp 的透传命令）—— 族帮助：
//	   防「`-h` 漏进真跑门的命令 ⇒ 问帮助变成真干一次」。
//	⑥ `gate check <不在册的名> -h` —— rc=2 且 stderr 首行判词 `不在册`
//	   （裁：名字是假的就别再回一句「帮助」当「存在」）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

const gateHelpFamilyFirst = "zerg gate check <判据名>"
const gateHelpChildFirst = "usage: check-newfile-registry.py"

func gateHelpRun(argv ...string) (int, string, string) {
	var so, se bytes.Buffer
	rc := run(argv, &so, &se)
	return rc, so.String(), se.String()
}

func gateHelpFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func TestGateCheckShortHelpReachesChild(t *testing.T) {
	// ① 主格：`-h` 从族帮助 → 子件 usage。
	rc, out, _ := gateHelpRun("gate", "check", "check-newfile-registry", "-h")
	if rc != 0 {
		t.Fatalf("`gate check <判据名> -h` 应退 0，得到 %d（out=%q）", rc, out)
	}
	got := gateHelpFirstLine(out)
	if !strings.HasPrefix(got, gateHelpChildFirst) {
		t.Errorf("`-h` 应到达子件（首行 %q），得到 %q", gateHelpChildFirst, got)
	}
	if strings.HasPrefix(got, gateHelpFamilyFirst) {
		t.Errorf("仍是族帮助（说明 `-h` 还被截着）")
	}

	// ② `--help` 逐字不动：仍是族帮助。
	rc, out, _ = gateHelpRun("gate", "check", "check-newfile-registry", "--help")
	if rc != 0 || !strings.HasPrefix(gateHelpFirstLine(out), gateHelpFamilyFirst) {
		t.Errorf("`--help` 应是族帮助 rc=0，得到 rc=%d 首行 %q", rc, gateHelpFirstLine(out))
	}
	if strings.Contains(out, gateHelpChildFirst) {
		t.Errorf("`--help` **不许**穿透到子件（既有行为一字不动）")
	}

	// ③ 没点名子件 ⇒ 仍族帮助。
	rc, out, _ = gateHelpRun("gate", "check", "-h")
	if rc != 0 || !strings.HasPrefix(gateHelpFirstLine(out), gateHelpFamilyFirst) {
		t.Errorf("`gate check -h`（无判据名）应是族帮助 rc=0，得到 rc=%d 首行 %q", rc, gateHelpFirstLine(out))
	}

	// ④ 非透传命令不受影响。
	rc, out, _ = gateHelpRun("gap", "ls", "-h")
	if rc != 0 || !strings.HasPrefix(gateHelpFirstLine(out), "zerg gap ls") {
		t.Errorf("`gap ls -h` 应是该条族帮助 rc=0，得到 rc=%d 首行 %q", rc, gateHelpFirstLine(out))
	}

	// ⑤ 未开 childHelp 的透传命令（`gate run`）⇒ `-h` **不许**漏给子件（那会真跑一门）。
	rc, out, _ = gateHelpRun("gate", "run", "no-such-gate-zz", "-h")
	if rc != 0 || !strings.HasPrefix(gateHelpFirstLine(out), "zerg gate run") {
		t.Errorf("`gate run <门> -h` 应是族帮助 rc=0（不许真跑），得到 rc=%d 首行 %q", rc, gateHelpFirstLine(out))
	}

	// ⑥ 名字不在册 + `-h`：rc=2 且首行 `不在册`。
	rc, _, errb := gateHelpRun("gate", "check", "no-such-gate-zz", "-h")
	if rc != 2 {
		t.Errorf("`gate check <不在册> -h` 应 rc=2（不在册就说不在册），得到 %d", rc)
	}
	if !strings.Contains(gateHelpFirstLine(errb), "不在册") {
		t.Errorf("stderr 首行应是「不在册」判词，得到 %q", gateHelpFirstLine(errb))
	}
}

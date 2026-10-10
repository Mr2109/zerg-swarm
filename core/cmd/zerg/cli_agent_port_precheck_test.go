// cli_agent_port_precheck_test.go —— GAP-20260925-28 命令面（`agent load` 端口冲突预检）的真二进制判据。
//
// 三态自检（全是**本机只读夹具**，不起任何服务/引擎）：
//
//	正控    端口空闲                        ⇒ 放行（退码 0；stderr 带预检「放行」行）
//	负控①   夹具 net.Listen 占住该端口        ⇒ **冲突**（退码 14）并**点名端口与占用者**（pid + 进程）
//	负控②   lsof 取不到（PATH 里没有 lsof）   ⇒ **不给结论**（退码 8）· ✗ 不许当空闲
//
// 夹具（两枚，都在**夹具侧**，✗ 不碰生产名册 / 生产主控）：
//
//	· 名册  = `ZERG_REPO` 注入的合成仓根里的 `gateway/fleet.yaml`（`fleet:` 段一台本机地址 + 一台远端）——
//	  本预检读的就是它的 `fleet:` 段（与 `agentChildAddr` 同一真源）。
//	· 主控  = `ZERG_PORT` 指一个**没人听**的口 ⇒ 前置状态探活（`probeStateConflict` 的
//	  `GET /api/fleet/status`）打不到主控、按既有口径**不阻断**（`return 0,false`）⇒ 本格不依赖
//	  任何生产主控在跑、也不依赖它的名册里有没有夹具机名。
//
// 真二进制取自**当前源码**现编（`zergBinaryFromSource`，不碰 `bin/zerg`）。
package main_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// precheckRepo 建合成仓根：仓根判据件 + 名册 `fleet:` 段（testm=本机夹具口 · far=远端）。
func precheckRepo(t *testing.T, localPort int) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	mustWrite(t, filepath.Join(root, "gateway", "fleet.yaml"),
		fmt.Sprintf("fleet:\n  testm: { host: 127.0.0.1, port: %d, os: macos }\n  far:   { host: 10.9.9.9, port: %d, os: linux }\n",
			localPort, localPort))
	return root
}

// freeLoopbackPort 取一枚本机空闲端口（绑一次、立即关）。
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口：%v", err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

// execAgentLoad 跑真二进制：注入**夹具名册**（`ZERG_REPO`）+ **隔离主控**（`ZERG_PORT` 指没人听的口）。
func execAgentLoad(t *testing.T, bin, repo string, extraEnv []string, argv ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, argv...)
	cmd.Env = append(os.Environ(), "ZERG_REPO="+repo, "ZERG_PORT=1")
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdin = strings.NewReader("")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	rc := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起不了二进制：%v", err)
		}
	}
	return rc, out.String(), errb.String()
}

func TestAgentLoadPortPrecheck_ThreeStates(t *testing.T) {
	bin := zergBinaryFromSource(t)
	port := freeLoopbackPort(t)
	root := precheckRepo(t, port)
	ps := strconv.Itoa(port)

	// ── 正控：端口空闲 ⇒ 放行（退码 0；预检那一行须在）──
	rc, _, errb := execAgentLoad(t, bin, root, nil, "agent", "load", "testm", "m", "--dry-run")
	if rc != 0 {
		t.Fatalf("正控：端口空闲应放行（退码 0），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "放行") {
		t.Errorf("正控：stderr 未见预检「放行」行：%s", errb)
	}

	// ── 负控①：夹具 net.Listen 占住该端口 ⇒ 冲突（退码 14）并点名端口与占用者 ──
	ln, err := net.Listen("tcp", "127.0.0.1:"+ps)
	if err != nil {
		t.Fatalf("起夹具监听：%v", err)
	}
	rc, _, errb = execAgentLoad(t, bin, root, nil, "agent", "load", "testm", "m", "--dry-run")
	if rc != 14 {
		ln.Close()
		t.Fatalf("负控①：端口被占应退 14（冲突），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, ps) {
		t.Errorf("负控①：报话须**点名端口** %s：%s", ps, errb)
	}
	if !strings.Contains(errb, "非本族") {
		t.Errorf("负控①：报话须写明是**非本族**监听者：%s", errb)
	}
	if !strings.Contains(errb, "(") || !strings.Contains(errb, ")") {
		t.Errorf("负控①：报话须**点名占用者**（pid(进程)）：%s", errb)
	}
	ln.Close()

	// ── 负控②：lsof 取不到 ⇒ 不给结论（退码 8），✗ 不许当空闲 ──
	empty := t.TempDir()
	rc, out, errb := execAgentLoad(t, bin, root, []string{"PATH=" + empty}, "agent", "load", "testm", "m", "--dry-run")
	if rc != 8 {
		t.Fatalf("负控②：lsof 取不到应退 8（不给结论），得到 %d · stderr=%s", rc, errb)
	}
	if strings.Contains(out+errb, "放行") {
		t.Errorf("负控②：取不到时**不许当空闲**：%s%s", out, errb)
	}
}

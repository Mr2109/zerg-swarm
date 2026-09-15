// hatch_linux_test.go —— Linux-only 的路径覆盖（macOS 上编不了 hatch_linux.go 的这些分支）。
//
// 本文件的存在理由（缺陷 15，2026-09-15 真机踩到）：子端跑在**系统服务**里，
// 环境里没有 XDG_RUNTIME_DIR ⇒ 一切 `systemctl --user` / `systemd-run --user` 当场失败。
// macOS 上跑不到这条路径，所以这里只做**判据级**断言；真正的验收是「按生产形态启动实例后
// 真孵一枚卵」（见 docs/issues 里的报告）。
package hatch

import (
	"os"
	"strings"
	"testing"
)

// 已设置 XDG_RUNTIME_DIR ⇒ 原样返回（不覆盖别人给的环境）。
func TestUserScopeEnv_KeepsExistingRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4242")
	env, err := userScopeEnv()
	if err != nil {
		t.Fatalf("已设置时不该报错：%v", err)
	}
	found := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "XDG_RUNTIME_DIR=") {
			if kv != "XDG_RUNTIME_DIR=/run/user/4242" {
				t.Fatalf("不应改写已有的 XDG_RUNTIME_DIR：%q", kv)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("环境里应保留 XDG_RUNTIME_DIR")
	}
}

// 未设置 + 本 uid 的运行时目录不存在 ⇒ **必须报错**，不许静默退回系统单元路径。
func TestUserScopeEnv_FailsClosedWhenMissing(t *testing.T) {
	if _, err := os.Stat("/run/user/99999999"); err == nil {
		t.Skip("本机恰好存在 /run/user/99999999，跳过该判据")
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	// 用真实 uid 目录做正例：存在则要求补上两个变量；不存在则要求报错。
	env, err := userScopeEnv()
	if err != nil {
		if !strings.Contains(err.Error(), "用户运行时目录") {
			t.Fatalf("错误信息应点明「用户运行时目录」：%v", err)
		}
		return // 符合预期：拿不到就报错
	}
	var xdg, bus bool
	for _, kv := range env {
		if strings.HasPrefix(kv, "XDG_RUNTIME_DIR=/run/user/") {
			xdg = true
		}
		if strings.HasPrefix(kv, "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/") {
			bus = true
		}
	}
	if !xdg || !bus {
		t.Fatalf("补出的环境应同时含 XDG_RUNTIME_DIR 与 DBUS_SESSION_BUS_ADDRESS（xdg=%v bus=%v）", xdg, bus)
	}
}

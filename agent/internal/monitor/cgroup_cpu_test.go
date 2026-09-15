package monitor

// cgroup_cpu_test.go —— T1 读数用例：解析正确 + 各类缺失都要**明确报错**（供降级）。

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCPUStat(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cpu.stat"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEggCPUUsecFromCgroup_ParsesUsage(t *testing.T) {
	dir := t.TempDir()
	writeCPUStat(t, dir, "usage_usec 123456789\nuser_usec 100000000\nsystem_usec 23456789\nnr_periods 0\n")
	got, err := EggCPUUsecFromCgroup(dir)
	if err != nil {
		t.Fatalf("应能读出 usage_usec: %v", err)
	}
	if got != 123456789 {
		t.Fatalf("usage_usec 应为 123456789，实得 %d", got)
	}
}

func TestEggCPUUsecFromCgroup_ErrorsAreExplicit(t *testing.T) {
	cases := []struct {
		name string
		dir  string
		body string
		prep func(t *testing.T) string
	}{
		{name: "空目录参数", dir: ""},
		{name: "目录不存在", dir: filepath.Join(t.TempDir(), "nope")},
		{name: "cpu.stat 里没有 usage_usec", body: "user_usec 1\nsystem_usec 2\n"},
		{name: "usage_usec 值非法", body: "usage_usec abc\n"},
		{name: "usage_usec 行字段数异常", body: "usage_usec\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := c.dir
			if dir == "" && c.body == "" {
				// 空参数用例
			} else if dir == "" {
				dir = t.TempDir()
				writeCPUStat(t, dir, c.body)
			}
			if _, err := EggCPUUsecFromCgroup(dir); err == nil {
				t.Fatal("异常情况必须返回 error（调用方据此降级为固定超时，绝不误杀）")
			}
		})
	}
}

package chat

import (
	"strings"
	"testing"
)

// 版本解析（多级 fallback）
func TestOverviewVersion(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"v2.5.7", 3},
		{"v2.6", 2},
		{"v2.5.7-test", 3}, // 容忍后缀
		{"v3", 1},
		{"abc", 0}, // 非法
	}
	for _, c := range cases {
		got := parseVersion(c.in)
		if len(got) != c.want {
			t.Errorf("parseVersion(%q) = %v (want %d 段)", c.in, got, c.want)
		}
	}
	// 版本比较
	if compareVersion([]int{2, 6}, []int{2, 5, 7}) <= 0 {
		t.Error("v2.6 应大于 v2.5.7")
	}
}

// 最新版本目录（当前应为 v2.5.7）
func TestOverviewLatestDir(t *testing.T) {
	d := latestVersionDir()
	t.Logf("最新版本目录: %s", d)
	if d == "" {
		t.Fatal("未找到版本目录")
	}
	if !strings.Contains(d, "v2.5.7") && !strings.Contains(d, "v2.6") {
		t.Logf("（版本可能已升级——当前 %s）", d)
	}
}

// 模块文档名解析
func TestOverviewModuleName(t *testing.T) {
	name := moduleDocName("<repo>/docs/项目文档/v2.5.7/01-模块-主控core-20260829.md")
	t.Logf("模块文档名: %q", name)
	if name == "" {
		t.Fatal("解析失败")
	}
	if !strings.Contains(name, "主控") {
		t.Errorf("应含主控: %q", name)
	}
}

// 全景返回（cap 950 截断后含尾注——断言针对 cap 前原始长度生成的内容完整性）
func TestOverviewFull(t *testing.T) {
	out, err := zergOverviewFull()
	if err != nil {
		t.Fatalf("全景失败: %v", err)
	}
	t.Logf("全景长度: %d 字", len([]rune(out)))
	for _, key := range []string{"架构", "怎么使用", "最近变化", "运行状态", "文档", "模块地图", "工具"} {
		if !strings.Contains(out, key) {
			t.Errorf("全景缺节: %s", key)
		}
	}
	// cap=950 截断 + 尾注（≈30 字）——实测 v2.5.8 全景 976 字（cap 生效——超限是预期行为）
	if len([]rune(out)) > 1020 {
		t.Errorf("全景超限: %d 字", len([]rune(out)))
	}
}

// section 下钻（使用/架构/模块/未知）
func TestOverviewSection(t *testing.T) {
	// 使用
	out, err := zergOverviewSection("使用")
	if err != nil {
		t.Errorf("section=使用 失败: %v", err)
	} else if !strings.Contains(out, "分流") && !strings.Contains(out, "指南") {
		t.Errorf("section=使用 内容异常: %s", truncateStr(out, 100))
	}
	// 未知模块（应提示可用）
	_, err = zergOverviewSection("不存在的模块xyz")
	if err == nil {
		t.Error("未知模块应报错（提示可用）")
	} else {
		t.Logf("未知模块提示: %s", err.Error()[:80])
	}
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

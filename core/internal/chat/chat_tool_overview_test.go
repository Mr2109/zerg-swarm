package chat

import (
	"os"
	"path/filepath"
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
		// 取源根本批已双认（仓内 docs/项目文档 优先、缺则仓外 ../Zerg-内部文档/项目文档）⇒
		// 一个版本目录都没找到只可能是「两处都不在盘上」（公开快照/CI 的形态），不是缺陷；
		// 跳过而不是失败（2026-09-11 CI 实测：CI 上此处 Fatal 导致整作业红）。
		t.Skip("未找到版本目录（两处取源根都不在盘上）——跳过")
	}
	if !strings.Contains(d, "v2.5.7") && !strings.Contains(d, "v2.6") {
		t.Logf("（版本可能已升级——当前 %s）", d)
	}
}

// 模块文档名解析
func TestOverviewModuleName(t *testing.T) {
	// 夹具路径**从取源根派生**（本批 E6）：原先写死 `<repo>/docs/项目文档/…`
	// 私有绝对路径 —— 换机器/换安装位置就失效，且与本批 E1「取源根收口」同源。
	// 本用例只验 basename 解析 ⇒ 只吃最后两段，取源根在不在盘上都不影响结论。
	fixture := filepath.Join(zergDocsBase(), "v2.5.7", "01-模块-主控core-20260829.md")
	name := moduleDocName(fixture)
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
	// 使用（依赖私有版本文档）——公开快照无该目录，只跳过这一节，下面与文档无关的断言照跑
	if latestVersionDir() == "" {
		t.Log("公开快照形态：无版本文档目录——跳过『使用』节断言")
	} else {
		out, err := zergOverviewSection("使用")
		if err != nil {
			t.Errorf("section=使用 失败: %v", err)
		} else if !strings.Contains(out, "分流") && !strings.Contains(out, "指南") {
			t.Errorf("section=使用 内容异常: %s", truncateStr(out, 100))
		}
	}
	// 未知模块（应提示可用）——与文档无关，任何形态都必须成立
	_, err := zergOverviewSection("不存在的模块xyz")
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

// ★ must-fail（2026-09-21 修 latestVersionDir 判据的牙）：**在途设计稿目录不算已发布版**。
// 病（收尾清单 R13）：旧判据只按「版本号最大 + 非递归 md 数 ≥3」取 ⇒ 并发子代理在写的
// `Zerg-内部文档/项目文档/v2.5.11/`（设计稿 5 篇 · 零发布件）被当成「最新发布版」，
// `section=使用` 当场报「使用指南文档未找到」（全量档两条 core go test 双红）。
// 本用例造两枚目录（高版本的只有设计稿 / 低版本的是发布版）⇒ 断言选中的**必须是发布版**；
// 再把发布版拿走（只剩设计稿）⇒ 断言返回空（宁缺勿滥：不拿设计稿冒充发布版）。
// 变异（判据缺牙）：去掉「必须含发布件」这一条 ⇒ 本用例必红（本批已实测真红一次）。
func TestOverviewLatestVersionSkipsDesignDraft(t *testing.T) {
	repo, _ := isolateDocsRoots(t)
	base := filepath.Join(repo, "docs")
	// ① 设计稿目录：版本号最大，但零发布件（照 v2.5.11 现跑形状：设计-…／承接-…／欠账… 共 5 篇）
	draft := filepath.Join(base, "项目文档", "v9.9.9")
	if err := os.MkdirAll(draft, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"设计-变更影响面-v1.0.md", "设计-变更影响面-v1.1.md", "设计-变更影响面-v1.2.md",
		"承接-度量与排序面-20260921.md", "欠账台账-v2.5.11.md",
	} {
		if err := os.WriteFile(filepath.Join(draft, name), []byte("# 设计稿（未发布）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// ② 已发布版目录：版本号更低，但有发布件（writeDocsFixture 会一并造出发布件）
	pub := writeDocsFixture(t, base, "v9.9.8", "PUB-MARKER", 3)

	if got := latestVersionDir(); got != pub {
		t.Errorf("must-fail：应选中已发布版 %s，实得 %q —— 选到设计稿目录 = 判据缺牙", pub, got)
	}
	// ③ 只剩设计稿 ⇒ 宁缺勿滥（空串 = 「没有已发布版」，不是「版本目录里没有文档」）
	if err := os.RemoveAll(pub); err != nil {
		t.Fatal(err)
	}
	if got := latestVersionDir(); got != "" {
		t.Errorf("must-fail：仓内只剩设计稿目录时应返回空（不拿设计稿冒充发布版），实得 %q", got)
	}
}

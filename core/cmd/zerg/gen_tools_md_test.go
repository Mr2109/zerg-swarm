// gen_tools_md_test.go —— `gen_tools_md.go`（`tools/gen_tools_md.py` 的 Go 真搬）成对负控。
//
// 负控形态：断言全是「只有实现对才过」的**逐字**等式（哑化任一语义面必红 —— 见回执的
// 仓外副本哑化实证）＋ 空值/缺输入/旗标三态各成一子用例（✗ 禁恒绿）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gtmFixtureRegistry —— 夹具注册表（含 L0 标记 · 含无 Category 的条目 · 含三类别）。
const gtmFixtureRegistry = "package chat\n\nvar gtmTools = []gtmToolDef{\n\t{Name: \"read\", Category: \"文件\", L0: true},\n\t{Name: \"write\", Category: \"文件\"},\n\t{Name: \"bash\", Category: \"系统\", L0: true},\n\t{Name: \"grep\", Category: \"系统\"},\n\t{Name: \"foo\", Category: \"其他\"},\n\t{Name: \"bar\"},\n}\n"

// gtmFixtureGolden —— **py 原件在同一夹具上的 stdout 逐字**（= gtmBuild 产出，无 print 尾换行）。
const gtmFixtureGolden = "# 工具目录（TOOLS）\n\n[English](TOOLS.en.md) | **中文**\n\n> *中文原版（唯一真相源）。英文版为派生翻译；两版如有不一致，以**本文**为准。*\n> *注：下方工具**描述**即实际发送给模型的文本（中文）——本版有意不翻译描述文字。*\n\n> 本文件由工具注册表**自动生成**（`core/internal/chat/chat_tool_registry.go`）——\n> 改工具清单请改注册表，不要手改本文件。\n> 统计：**5** 个注册工具，其中 **2** 个为 L0 常驻（直接写进模型提示），其余按需发现。\n\n## 两层工具\n\n| 层 | 数量 | 说明 |\n|---|---|---|\n| **L0 常驻** | 2 | 高频基础能力，定义直接注入模型提示，随时可调 |\n| **延迟加载** | 3 | 通过 `tool_search`（按关键词/类别）发现后再调用，避免提示膨胀 |\n\n调用约定：模型先看提示里有没有；没有就用 `tool_search` 搜（支持中文关键词），拿到定义再调。\n\n## 按类别\n\n### 文件（2）\n\n- `read`  ← **L0 常驻**\n- `write`\n\n### 系统（2）\n\n- `bash`  ← **L0 常驻**\n- `grep`\n\n### 其他（1）\n\n- `foo`\n\n## 工具自身文档（履历）\n\n每个工具的**版本、能力、参数、注意事项**写在 `tools/<name>.md`（履历），\n与注册表、工具简介三者应保持一致——这是本项目的硬约定（见 `docs/design/工具升级规范.md`）。\n\n## 安全相关\n\n- shell（`bash`）的删除类命令受**范围闸门**约束：仅允许任务工作区与 `ZERG_EXTRA_ALLOW_DIR` 白名单\n- 文件类工具（`read`/`write`/`edit`/`glob`）同样受路径域限制，越界返回**可执行的替代路径**而非空报错\n- 参数格式错误会触发**格式反馈协议**：返回「系统断言 + 正确示例」，避免模型重发同样的坏请求\n\n## 重新生成\n\n```bash\npython3 tools/gen_tools_md.py > docs/TOOLS.zh-CN.md   # 英文版 docs/TOOLS.en.md 需同 PR 同步\n```\n"

func TestGtmSplitBlocksLookahead(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{""}},
		{"abc", []string{"abc"}},
		{"a{Name:b", []string{"a", "{Name:b"}},
		{"{Name:a{ Name:b", []string{"", "{Name:a", "{ Name:b"}},
	}
	for _, c := range cases {
		got := gtmSplitBlocks(c.in)
		if strings.Join(got, "\x00") != strings.Join(c.want, "\x00") {
			t.Errorf("gtmSplitBlocks(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGtmBuildGolden(t *testing.T) {
	got, total, l0, cat := gtmBuild(gtmFixtureRegistry)
	if got != gtmFixtureGolden {
		t.Errorf("stdout 与 py 原件不逐字同\n got=%q\nwant=%q", got, gtmFixtureGolden)
	}
	if total != 5 || l0 != 2 || cat != 3 {
		t.Errorf("统计 = %d 工具 / %d L0 / %d 类别，want 5/2/3", total, l0, cat)
	}
}

func TestGtmMainRootGoldenAndStderr(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "core", "internal", "chat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chat_tool_registry.go"), []byte(gtmFixtureRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	if rc := gtmMainRoot(&so, &se, root); rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if so.String() != gtmFixtureGolden+"\n" {
		t.Errorf("stdout 与 py 原件不逐字同")
	}
	if se.String() != "# 生成完成：5 工具 / 2 L0 / 3 类别\n" {
		t.Errorf("stderr = %q", se.String())
	}
}

func TestGtmMissingRegistryErrno(t *testing.T) {
	root := t.TempDir() // 无注册表
	var so, se bytes.Buffer
	if rc := gtmMainRoot(&so, &se, root); rc != 1 {
		t.Fatalf("缺注册表 rc = %d, want 1", rc)
	}
	want := "读取注册表失败: [Errno 2] No such file or directory: '" +
		gtmPyJoin(root, "core", "internal", "chat", "chat_tool_registry.go") + "'\n"
	if se.String() != want {
		t.Errorf("stderr = %q, want %q", se.String(), want)
	}
	if so.Len() != 0 {
		t.Errorf("stdout 应为空，得 %q", so.String())
	}
}

func TestGtmFlags(t *testing.T) {
	var so, se bytes.Buffer
	if rc := gtmRun(&so, &se, []string{"--help"}); rc != 0 {
		t.Fatalf("--help rc = %d, want 0", rc)
	}
	if so.String() != gtmDoc+"\n" {
		t.Errorf("--help stdout 与 __doc__ 不逐字同")
	}
	if se.Len() != 0 {
		t.Errorf("--help stderr 应空")
	}

	so.Reset()
	se.Reset()
	if rc := gtmRun(&so, &se, []string{"-h"}); rc != 0 {
		t.Fatalf("-h rc = %d, want 0", rc)
	}
	if so.String() != gtmDoc+"\n" {
		t.Errorf("-h stdout 与 __doc__ 不逐字同")
	}

	so.Reset()
	se.Reset()
	if rc := gtmRun(&so, &se, []string{"--nope", "-x"}); rc != 2 {
		t.Fatalf("未知旗标 rc = %d, want 2", rc)
	}
	if se.String() != "✗ 未知参数: --nope -x\n" {
		t.Errorf("未知旗标 stderr = %q", se.String())
	}
	if so.Len() != 0 {
		t.Errorf("未知旗标 stdout 应空")
	}

	so.Reset()
	se.Reset()
	if rc := gtmRun(&so, &se, []string{"--help", "--bad"}); rc != 2 {
		t.Fatalf("未知旗标+--help rc = %d, want 2（py 逐字序：bad 先于 help）", rc)
	}
	if so.Len() != 0 {
		t.Errorf("未知旗标+--help stdout 应空")
	}
}

func TestGtmEmptyRegistryHeaderOnly(t *testing.T) {
	// 空注册表（零条目）：py 仍 rc=0、印页头 —— 空转 ≠ 绿、亦 ≠ 错（逐字照搬）
	got, total, l0, cat := gtmBuild("package chat\n")
	if total != 0 || l0 != 0 || cat != 0 {
		t.Fatalf("空注册表统计 = %d/%d/%d, want 0/0/0", total, l0, cat)
	}
	if !strings.Contains(got, "> 统计：**0** 个注册工具") {
		t.Errorf("空注册表页头缺统计行")
	}
	if !strings.Contains(got, "## 按类别") {
		t.Errorf("空注册表缺「按类别」段")
	}
	if strings.Contains(got, "### ") {
		t.Errorf("空注册表不应出任何类别段")
	}
}

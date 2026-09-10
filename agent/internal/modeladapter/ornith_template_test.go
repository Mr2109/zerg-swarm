package modeladapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// 2026-09-11 登记表配置化（chat_template）单测。
// 全部用 t.Setenv("HOME", 临时目录) 覆盖家目录，保证在任何机器上结果确定——
// 不受「本机恰好存在 ~/.zerg/ornith_chat_template.jinja」这类环境因素影响。

// writeTemplate 在临时目录写一个假的 chat template 文件，返回绝对路径。
func writeTemplate(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(p, []byte("{{ messages }}"), 0o644); err != nil {
		t.Fatalf("写模板失败: %v", err)
	}
	return p
}

func argsWithTemplate(entry *registry.ModelEntry) string {
	a := &Ornith{}
	return strings.Join(a.BuildArgs(entry, 9999), " ")
}

// 1) 登记表字段优先于环境变量。
func TestOrnithTemplateRegistryWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fromCfg := writeTemplate(t, filepath.Join(home, "cfg"), "a.jinja")
	fromEnv := writeTemplate(t, filepath.Join(home, "env"), "b.jinja")
	t.Setenv("ZERG_ORNITH_TEMPLATE", fromEnv)

	got := argsWithTemplate(&registry.ModelEntry{File: "/m/x.gguf", ChatTemplate: fromCfg})
	if !strings.Contains(got, "--chat-template-file "+fromCfg) {
		t.Errorf("登记表模板未被采用\n得到: %s\n期望含: --chat-template-file %s", got, fromCfg)
	}
	if strings.Contains(got, fromEnv) {
		t.Errorf("环境变量模板不应生效（登记表优先）\n得到: %s", got)
	}
}

// 2) 登记表配了不存在的路径 → 不把坏路径传给后端（回退，且可回退到「无」）。
func TestOrnithTemplateMissingConfigIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_ORNITH_TEMPLATE", "")
	bogus := filepath.Join(home, "nope", "missing.jinja")

	got := argsWithTemplate(&registry.ModelEntry{File: "/m/x.gguf", ChatTemplate: bogus})
	if strings.Contains(got, bogus) {
		t.Errorf("不存在的登记表模板被传给了后端: %s", got)
	}
	if strings.Contains(got, "--chat-template-file") {
		t.Errorf("任何候选都不存在时不应出现 --chat-template-file\n得到: %s", got)
	}
	if !strings.Contains(got, "--jinja") {
		t.Errorf("其余参数应照常构建（含 --jinja）\n得到: %s", got)
	}
}

// 3) 无登记表配置时，环境变量兜底生效。
func TestOrnithTemplateEnvFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fromEnv := writeTemplate(t, filepath.Join(home, "env"), "c.jinja")
	t.Setenv("ZERG_ORNITH_TEMPLATE", fromEnv)

	got := argsWithTemplate(&registry.ModelEntry{File: "/m/x.gguf"})
	if !strings.Contains(got, "--chat-template-file "+fromEnv) {
		t.Errorf("环境变量模板未兜底生效\n得到: %s", got)
	}
}

// 4) ~ 展开 + 家目录约定路径（~/.zerg/ornith_chat_template.jinja）。
func TestOrnithTemplateTildeAndHomeConvention(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_ORNITH_TEMPLATE", "")

	// 4a) 登记表写 ~/... → 正确展开
	rel := filepath.Join(home, "tilde", "t.jinja")
	if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := argsWithTemplate(&registry.ModelEntry{File: "/m/x.gguf", ChatTemplate: "~/tilde/t.jinja"})
	if !strings.Contains(got, "--chat-template-file "+rel) {
		t.Errorf("~/ 未正确展开\n得到: %s\n期望含: %s", got, rel)
	}

	// 4b) 无任何配置 → 家目录约定路径命中
	convention := writeTemplate(t, filepath.Join(home, ".zerg"), "ornith_chat_template.jinja")
	got2 := argsWithTemplate(&registry.ModelEntry{File: "/m/x.gguf"})
	if !strings.Contains(got2, "--chat-template-file "+convention) {
		t.Errorf("家目录约定路径未命中\n得到: %s\n期望含: %s", got2, convention)
	}
}

// 5) YAML 端到端：登记表里的 chat_template 字段能被解析出来（含 ~ 写法）。
func TestRegistryParsesChatTemplate(t *testing.T) {
	dir := t.TempDir()
	y := filepath.Join(dir, "agent_models.yaml")
	content := "example-35b-v2:\n" +
		"  backend: llama-server\n" +
		"  file: /data/models/x.gguf\n" +
		"  mem_gb: 21\n" +
		"  modality: multimodal\n" +
		"  mmproj: /data/models/mmproj.gguf\n" +
		"  chat_template: ~/.zerg/ornith_chat_template.jinja\n"
	if err := os.WriteFile(y, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := registry.New(y)
	if err != nil {
		t.Fatalf("加载登记表失败: %v", err)
	}
	e, ok := r.Get("example-35b-v2")
	if !ok {
		t.Fatal("未取到 example-35b-v2")
	}
	if e.ChatTemplate != "~/.zerg/ornith_chat_template.jinja" {
		t.Errorf("chat_template 未解析: %q", e.ChatTemplate)
	}
	if e.MMProj != "/data/models/mmproj.gguf" {
		t.Errorf("mmproj 解析异常（inline Custom 字段未受影响？）: %q", e.MMProj)
	}
}

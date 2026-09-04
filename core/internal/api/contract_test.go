package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseContract(t *testing.T) {
	desc := "修一个 bug。\n\n【验收】{\"must_write_files\":[\"out.md\"],\"must_pass_cmds\":[\"go build ./...\"],\"min_changed_files\":1}\n"
	c := ParseContract(desc)
	if c == nil {
		t.Fatal("应解析出契约")
	}
	if len(c.MustWriteFiles) != 1 || c.MustWriteFiles[0] != "out.md" {
		t.Fatalf("must_write_files: %v", c.MustWriteFiles)
	}
	if len(c.MustPassCmds) != 1 {
		t.Fatalf("must_pass_cmds: %v", c.MustPassCmds)
	}
	if c.MinChangedFiles != 1 {
		t.Fatalf("min_changed_files: %d", c.MinChangedFiles)
	}
	// 无契约
	if ParseContract("普通任务描述没有验收段") != nil {
		t.Fatal("无验收段应返回 nil")
	}
	// 坏 JSON
	if ParseContract("【验收】{bad json}") != nil {
		t.Fatal("坏 JSON 应返回 nil")
	}
}

func TestContractVerify(t *testing.T) {
	dir := t.TempDir()
	// ① 必写文件过/不过
	_ = os.WriteFile(filepath.Join(dir, "out.md"), []byte("内容内容内容"), 0o644)
	c := &TaskContract{MustWriteFiles: []string{"out.md"}}
	if res := c.Verify("", dir); !res.Pass {
		t.Fatalf("存在文件应过: %v", res.Failures)
	}
	c2 := &TaskContract{MustWriteFiles: []string{"missing.md"}}
	if res := c2.Verify("", dir); res.Pass {
		t.Fatal("缺失文件应不过")
	}
	// ② 必过命令
	c3 := &TaskContract{MustPassCmds: []string{"true"}}
	if res := c3.Verify("", dir); !res.Pass {
		t.Fatalf("true 应过: %v", res.Failures)
	}
	c4 := &TaskContract{MustPassCmds: []string{"exit 3"}}
	if res := c4.Verify("", dir); res.Pass {
		t.Fatal("exit 3 应不过")
	}
	// ③ 嵌套 JSON 提取（{} 内含 {}）
	desc := "任务\n【验收】{\"must_write_files\":[\"a\"],\"must_pass_cmds\":[]}\n后文"
	c5 := ParseContract(desc)
	if c5 == nil || len(c5.MustWriteFiles) != 1 {
		t.Fatalf("嵌套解析: %v", c5)
	}
}

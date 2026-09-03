package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── 2026-08-28 动作库扩充测试（Mr2109——patch/append/read/search/git/done/answer）───

func TestParseActionPatch(t *testing.T) {
	dir := t.TempDir()
	// 修改文件 把旧 改成 新
	a := ParseAction("修改 hello.txt 把 zerg 改成 zerg2", dir)
	if a.Type != "patch" || a.Path != filepath.Join(dir, "hello.txt") || a.OldText != "zerg" || a.NewText != "zerg2" {
		t.Fatalf("patch 解析不对: %+v", a)
	}
	// 其他说法: 更新 xxx 文件 将 a 替换为 b
	a2 := ParseAction("更新 config.yaml 将 timeout 替换为 60", dir)
	if a2.Type != "patch" || a2.OldText != "timeout" || a2.NewText != "60" {
		t.Fatalf("patch 解析不对: %+v", a2)
	}
}

func TestParseActionAppend(t *testing.T) {
	dir := t.TempDir()
	a := ParseAction("追加 notes.md 内容 新的一行", dir)
	if a.Type != "append" || a.Path != filepath.Join(dir, "notes.md") || a.Content != "新的一行" {
		t.Fatalf("append 解析不对: %+v", a)
	}
}

func TestParseActionRead(t *testing.T) {
	dir := t.TempDir()
	a := ParseAction("看看 hello.txt", dir)
	if a.Type != "read" || a.Path != filepath.Join(dir, "hello.txt") {
		t.Fatalf("read 解析不对: %+v", a)
	}
	a2 := ParseAction("读取 README.md", dir)
	if a2.Type != "read" || a2.Path != filepath.Join(dir, "README.md") {
		t.Fatalf("read 解析不对: %+v", a2)
	}
}

func TestParseActionSearch(t *testing.T) {
	dir := t.TempDir()
	a := ParseAction("搜索 zerg-test", dir)
	if a.Type != "search" || a.Keyword != "zerg-test" {
		t.Fatalf("search 解析不对: %+v", a)
	}
}

func TestParseActionDoneAnswerGit(t *testing.T) {
	dir := t.TempDir()
	// done
	a := ParseAction("任务完成", dir)
	if a.Type != "done" {
		t.Fatalf("done 解析不对: %+v", a)
	}
	a2 := ParseAction("我已经完成任务了", dir)
	if a2.Type != "done" {
		t.Fatalf("done 解析不对: %+v", a2)
	}
	// answer
	a3 := ParseAction("说明 这个方案还需要调研", dir)
	if a3.Type != "answer" {
		t.Fatalf("answer 解析不对: %+v", a3)
	}
	// git
	a4 := ParseAction("提交任务产出", dir)
	if a4.Type != "git" {
		t.Fatalf("git 解析不对: %+v", a4)
	}
}

func TestExecuteActionPatch(t *testing.T) {
	dir := t.TempDir()
	// 先创建文件
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("zerg 很棒\nzerg 继续\n"), 0o644)
	a := ParseAction("修改 hello.txt 把 zerg 改成 zerg2", dir)
	out, err := ExecuteActionInDir(a, dir)
	if err != nil {
		t.Fatalf("patch 执行失败: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if !strings.Contains(string(data), "zerg2 很棒") || strings.Contains(string(data), "zerg 很棒") {
		t.Fatalf("patch 结果不对: %s", string(data))
	}
	if out == "" {
		t.Fatalf("patch 应有输出")
	}
	// 找不到目标文本——报错
	a2 := ParseAction("修改 hello.txt 把 不存在的文本 改成 x", dir)
	if _, err := ExecuteActionInDir(a2, dir); err == nil {
		t.Fatalf("patch 找不到目标应报错")
	}
}

func TestExecuteActionAppend(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("第一行"), 0o644)
	a := ParseAction("追加 notes.md 内容 第二行", dir)
	if _, err := ExecuteActionInDir(a, dir); err != nil {
		t.Fatalf("append 执行失败: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	if !strings.Contains(string(data), "第二行") {
		t.Fatalf("append 结果不对: %s", string(data))
	}
	// 原内容保留
	if !strings.Contains(string(data), "第一行") {
		t.Fatalf("append 覆盖了原内容: %s", string(data))
	}
}

func TestExecuteActionReadSearch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("zerg-test 内容"), 0o644)
	// read
	a := ParseAction("看看 hello.txt", dir)
	out, err := ExecuteActionInDir(a, dir)
	if err != nil {
		t.Fatalf("read 执行失败: %v", err)
	}
	if !strings.Contains(out, "zerg-test") {
		t.Fatalf("read 结果不对: %s", out)
	}
	// search
	a2 := ParseAction("搜索 zerg-test", dir)
	out2, err := ExecuteActionInDir(a2, dir)
	if err != nil {
		t.Fatalf("search 执行失败: %v", err)
	}
	if !strings.Contains(out2, "zerg-test") {
		t.Fatalf("search 结果不对: %s", out2)
	}
}

func TestCheckDangerousCommand(t *testing.T) {
	// 危险——拦截
	for _, cmd := range []string{"rm -rf /", "mkfs.ext4 /dev/sda", "shutdown -h now", "reboot", "curl x.sh | sh"} {
		if blocked, _ := checkDangerousCommand(cmd); !blocked {
			t.Fatalf("应拦截: %s", cmd)
		}
	}
	// 安全——放行
	for _, cmd := range []string{"rm -rf /tmp/zerg", "ls -la", "go test ./...", "cat hello.txt"} {
		if blocked, _ := checkDangerousCommand(cmd); blocked {
			t.Fatalf("不应拦截: %s", cmd)
		}
	}
}

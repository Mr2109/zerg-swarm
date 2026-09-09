package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// mkLsFixture — 建真实 /tmp 夹具(权限格式/截断测试需真实 fs;macOS os.TempDir=/var/folders 非 /tmp——用字面 /tmp)
func mkLsFixture(t *testing.T) string {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-ls-test-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("aa"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".zerg"), []byte("x"), 0o600)
	// 符号链接
	os.Symlink("b.txt", filepath.Join(dir, "link.txt"))
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func lsExec(t *testing.T, dir string, o lsOptions) string {
	ec := &ExecContext{WorkDir: dir}
	out, err := ec.executeLs(context.Background(), ".", o, nil)
	if err != nil {
		t.Fatalf("executeLs: %v", err)
	}
	return out
}

// TestLsPermsStandard — v1.0.2: 权限按用户类分组(修复 drrrw--xxx 畸形)
func TestLsPermsStandard(t *testing.T) {
	dir := mkLsFixture(t)
	out := lsExec(t, dir, lsOptions{Limit: 0, Hidden: true})
	for _, want := range []string{"drwxr-xr-x", "-rw-r--r--", "-rwxr-xr-x", "lrw-r--r--"} {
		if !strings.Contains(out, want) {
			t.Errorf("缺标准权限 %q——got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "drrr") || strings.Contains(out, "xxx") {
		t.Errorf("畸形位型分组(旧 bug)仍出现——got:\n%s", out)
	}
	if !strings.Contains(out, "link.txt -> b.txt") {
		t.Errorf("符号链接应显示 -> 目标——got:\n%s", out)
	}
}

// TestLsSummaryAndDefaults — 摘要行 + 点目录默认隐藏 + 目录恒前
func TestLsSummaryAndDefaults(t *testing.T) {
	dir := mkLsFixture(t)
	out := lsExec(t, dir, lsOptions{})
	if !strings.Contains(out, "共 4 项:1 目录 / 3 文件") {
		t.Errorf("摘要行错(点项应隐藏)——got:\n%s", out)
	}
	if strings.Contains(out, ".zerg") {
		t.Error("点项默认应隐藏")
	}
	// 目录恒在文件前
	dl := strings.Index(out, "sub/")
	fl := strings.Index(out, "a.go")
	if dl < 0 || fl < 0 || dl > fl {
		t.Errorf("目录应在文件前——got:\n%s", out)
	}
	// hidden=true 显示点项
	outH := lsExec(t, dir, lsOptions{Hidden: true, Limit: 0})
	if !strings.Contains(outH, ".zerg") {
		t.Error("hidden=true 应显示点项")
	}
}

// TestLsFilters — dir_only / pattern
func TestLsFilters(t *testing.T) {
	dir := mkLsFixture(t)
	outD := lsExec(t, dir, lsOptions{DirOnly: true, Limit: 0})
	if strings.Contains(outD, ".go") || !strings.Contains(outD, "sub/") {
		t.Errorf("dir_only 只应含目录——got:\n%s", outD)
	}
	outP := lsExec(t, dir, lsOptions{Pattern: "*.go", Limit: 0})
	if !strings.Contains(outP, "a.go") || strings.Contains(outP, "b.txt") {
		t.Errorf("pattern 过滤错——got:\n%s", outP)
	}
}

// TestLsSortAndTimeCol — sort_by=size 大在前;sort_by=time 附时间列
func TestLsSortAndTimeCol(t *testing.T) {
	dir := mkLsFixture(t)
	outS := lsExec(t, dir, lsOptions{SortBy: "size", Limit: 0})
	// size 降序: link(5B) > a.go/b.txt(2B 平——按名序 a.go 先)——按行序取 token 防名字撞车
	var order []string
	for _, ln := range strings.Split(outS, "\n") {
		switch {
		case strings.Contains(ln, " -> "):
			order = append(order, "link")
		case strings.HasSuffix(ln, "a.go"):
			order = append(order, "a.go")
		case strings.HasSuffix(ln, "b.txt"):
			order = append(order, "b.txt")
		}
	}
	if len(order) != 3 || strings.Join(order, ",") != "link,a.go,b.txt" {
		t.Errorf("size 排序错(期望 link>a.go>b.txt)——实际 %v——got:\n%s", order, outS)
	}
	outT := lsExec(t, dir, lsOptions{SortBy: "time", Limit: 0})
	re := regexp.MustCompile(`\d{2}-\d{2} \d{2}:\d{2}`)
	if !re.MatchString(outT) {
		t.Errorf("sort_by=time 应附 MM-DD HH:mm 时间列(拍板 3)——got:\n%s", outT)
	}
	outN := lsExec(t, dir, lsOptions{Limit: 0})
	if re.MatchString(outN) {
		t.Error("默认 name 排序不应有时间列(省 token)")
	}
}

// TestLsTruncate — limit 截断 + 省略引导
func TestLsTruncate(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-ls-trunc-%d", time.Now().UnixNano()))
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 120; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.txt", i)), []byte("x"), 0o644)
	}
	ec := &ExecContext{WorkDir: dir}
	out, err := ec.executeLs(context.Background(), ".", lsOptions{Limit: 60}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[已省略 60 项——共 120 项") {
		t.Errorf("应截断并带省略引导——got:\n%s", out)
	}
	outAll := lsExec(t, dir, lsOptions{Limit: 0})
	if strings.Contains(outAll, "已省略") {
		t.Error("limit 0 不应截断")
	}
}

// TestLsEmptyMsgs — 空/无匹配/无目录 消息
func TestLsEmptyMsgs(t *testing.T) {
	empty := filepath.Join("/tmp", fmt.Sprintf("zerg-ls-empty-%d", time.Now().UnixNano()))
	os.MkdirAll(empty, 0o755)
	defer os.RemoveAll(empty)
	ec := &ExecContext{WorkDir: empty}
	out, _ := ec.executeLs(context.Background(), ".", lsOptions{}, nil)
	if !strings.Contains(out, "(空目录)") {
		t.Errorf("空目录应提示——got:\n%s", out)
	}
	// 无匹配/无目录 场景复用有内容的目录
	dir := mkLsFixture(t)
	outP := lsExec(t, dir, lsOptions{Pattern: "*.xyz", Limit: 0})
	if !strings.Contains(outP, "(无匹配项)") {
		t.Errorf("pattern 无匹配应提示——got:\n%s", outP)
	}
	outD := lsExec(t, dir, lsOptions{DirOnly: true, Pattern: "*.txt", Limit: 0})
	if !strings.Contains(outD, "(无目录)") {
		t.Errorf("dir_only 无目录应提示——got:\n%s", outD)
	}
}

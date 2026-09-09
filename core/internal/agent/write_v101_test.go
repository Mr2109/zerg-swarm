package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mkWriteDir(t *testing.T) string {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-write-test-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeExec(dir, path, content string, o writeOpts) (string, error) {
	ec := &ExecContext{WorkDir: dir}
	return ec.executeWrite(context.Background(), path, content, o, nil)
}

// TestWriteBlockDocTypes — 类型防呆:已存在 docx / 新写 xlsx / 二进制 → 拒
func TestWriteBlockDocTypes(t *testing.T) {
	dir := mkWriteDir(t)
	// 已存在 docx
	p := filepath.Join(dir, "d.docx")
	buildMinimalDocx(t, p, "hi")
	if _, err := writeExec(dir, "d.docx", "plain text", writeOpts{}); err == nil || !strings.Contains(err.Error(), "DOCX") {
		t.Errorf("已存在 docx 应拒——got %v", err)
	}
	// edit 同源防呆
	ec := &ExecContext{WorkDir: dir}
	if _, err := ec.executeEdit(context.Background(), "d.docx", "hi", "yo", nil); err == nil || !strings.Contains(err.Error(), "DOCX") {
		t.Errorf("edit docx 应拒——got %v", err)
	}
	// 新写 .xlsx(扩展名防呆)
	if _, err := writeExec(dir, "new.xlsx", "text", writeOpts{}); err == nil || !strings.Contains(err.Error(), "XLSX") {
		t.Errorf("新 xlsx 应拒——got %v", err)
	}
	// 新写 .pdf
	if _, err := writeExec(dir, "r.pdf", "text", writeOpts{}); err == nil || !strings.Contains(err.Error(), "PDF") {
		t.Errorf("新 pdf 应拒——got %v", err)
	}
}

// TestWriteJsonRollback — 写坏 json → 回滚无残留;好 json → 成功
func TestWriteJsonRollback(t *testing.T) {
	dir := mkWriteDir(t)
	_, err := writeExec(dir, "bad.json", `{"a": `, writeOpts{})
	if err == nil || !strings.Contains(err.Error(), "回滚") {
		t.Errorf("坏 json 应回滚报错——got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "bad.json")); !os.IsNotExist(statErr) {
		t.Error("坏 json 目标应已删除(无残留)")
	}
	if _, err := writeExec(dir, "ok.json", `{"a": 1}`, writeOpts{}); err != nil {
		t.Errorf("好 json 应成功——%v", err)
	}
}

// TestWriteEncoding — bom + crlf/lf
func TestWriteEncoding(t *testing.T) {
	dir := mkWriteDir(t)
	if _, err := writeExec(dir, "b.txt", "abc", writeOpts{BOM: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "b.txt"))
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Errorf("bom=true 应加 EF BB BF——got %x", b[:3])
	}
	if _, err := writeExec(dir, "c.txt", "a\nb\r\nc\n", writeOpts{LineEnd: "crlf"}); err != nil {
		t.Fatal(err)
	}
	c, _ := os.ReadFile(filepath.Join(dir, "c.txt"))
	if !bytes.Contains(c, []byte("a\r\nb\r\nc\r\n")) {
		t.Errorf("crlf 应统一 \\r\\n——got %q", string(c))
	}
	if _, err := writeExec(dir, "l.txt", "a\r\nb", writeOpts{}); err != nil {
		t.Fatal(err)
	}
	l, _ := os.ReadFile(filepath.Join(dir, "l.txt"))
	if string(l) != "a\nb" {
		t.Errorf("lf 应去 \\r——got %q", string(l))
	}
}

// TestWriteRawBypass — format=raw 裸写逃生(docx 可覆盖——自担)
func TestWriteRawBypass(t *testing.T) {
	dir := mkWriteDir(t)
	p := filepath.Join(dir, "d.docx")
	buildMinimalDocx(t, p, "hi")
	if _, err := writeExec(dir, "d.docx", "RAW", writeOpts{Format: "raw"}); err != nil {
		t.Errorf("raw 应跳过防呆——%v", err)
	}
}

// TestWriteDirAndSize — 目录目标拒;10MB 上限
func TestWriteDirAndSize(t *testing.T) {
	dir := mkWriteDir(t)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	if _, err := writeExec(dir, "sub", "x", writeOpts{}); err == nil || !strings.Contains(err.Error(), "目录") {
		t.Errorf("目录目标应拒——got %v", err)
	}
	big := strings.Repeat("x", 10*1024*1024+1)
	if _, err := writeExec(dir, "big.txt", big, writeOpts{}); err == nil || !strings.Contains(err.Error(), "10MB") {
		t.Errorf(">10MB 应拒——got %v", err)
	}
}

// TestWriteYamlTolerant — yaml: 有解析器则校验,无则跳过注记——有效 yaml 恒成功
func TestWriteYamlTolerant(t *testing.T) {
	dir := mkWriteDir(t)
	out, err := writeExec(dir, "c.yaml", "a: 1\nb:\n  - x\n", writeOpts{})
	if err != nil {
		t.Errorf("有效 yaml 应成功——%v", err)
	} else if strings.Contains(out, "回滚") {
		t.Error("有效 yaml 不应回滚")
	}
}

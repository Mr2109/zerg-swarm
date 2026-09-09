package agent

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readFixtures — 建 /tmp 夹具(真实文件系统;macOS TempDir 非 /tmp 故用字面 /tmp)
func mkReadFixture(t *testing.T) string {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-test-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	// 文本
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("line1\nline2\nline3\n"), 0o644)
	// 二进制(null 字节)
	os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{1, 2, 0, 3, 255}, 0o644)
	// GBK 中文(中文=0xD6D0 0xCEC4)
	os.WriteFile(filepath.Join(dir, "gbk.txt"), []byte{0xD6, 0xD0, 0xCE, 0xC4}, 0o644)
	// UTF-16LE BOM + 中文
	os.WriteFile(filepath.Join(dir, "u16.txt"), []byte{0xFF, 0xFE, 0x2D, 0x4E, 0x87, 0x65}, 0o644)
	// 图像头(PNG 魔数)
	os.WriteFile(filepath.Join(dir, "img.png"), []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3}, 0o644)
	// 超长行
	os.WriteFile(filepath.Join(dir, "long.txt"), []byte(strings.Repeat("x", 3000)+"\n"), 0o644)
	return dir
}

func readExec(t *testing.T, dir, name string, args map[string]any) (string, error) {
	ec := &ExecContext{WorkDir: dir}
	mp := map[string]any{"path": name}
	for k, v := range args {
		mp[k] = v
	}
	return ec.executeRead(context.Background(), name, mp, nil)
}

// TestReadTextNum — 行号默认开(num=false 关)——v1.0.2
func TestReadTextNum(t *testing.T) {
	dir := mkReadFixture(t)
	out, err := readExec(t, dir, "a.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "type=\"text\"") || !strings.Contains(out, "1 │ line1") || !strings.Contains(out, "3 │ line3") {
		t.Errorf("默认应带行号+type=text——got:\n%s", out)
	}
	out2, _ := readExec(t, dir, "a.txt", map[string]any{"num": false})
	if strings.Contains(out2, "│") {
		t.Errorf("num=false 应无行号——got:\n%s", out2)
	}
}

// TestReadLongLineTruncate — 超长行截断(业界规格)
func TestReadLongLineTruncate(t *testing.T) {
	dir := mkReadFixture(t)
	out, _ := readExec(t, dir, "long.txt", map[string]any{"num": false})
	if !strings.Contains(out, "[截断]") {
		t.Errorf("3000 字符行应截断——got len=%d", len(out))
	}
}

// TestReadBinaryImg — 二进制/图像委托防护
func TestReadBinaryImg(t *testing.T) {
	dir := mkReadFixture(t)
	_, err := readExec(t, dir, "bin.dat", nil)
	if err == nil || !strings.Contains(err.Error(), "二进制") {
		t.Errorf("二进制应提示不吐内容——got %v", err)
	}
	_, err = readExec(t, dir, "img.png", nil)
	if err == nil || !strings.Contains(err.Error(), "image_desc") {
		t.Errorf("图像应委托 image_desc——got %v", err)
	}
}

// TestReadEncoding — GBK/UTF-16 → UTF-8(iconv)
func TestReadEncoding(t *testing.T) {
	dir := mkReadFixture(t)
	out, err := readExec(t, dir, "gbk.txt", map[string]any{"num": false})
	if err != nil {
		t.Fatalf("GBK 转码失败: %v", err)
	}
	if !strings.Contains(out, "中文") {
		t.Errorf("GBK 应转出中文——got:\n%s", out)
	}
	out2, err := readExec(t, dir, "u16.txt", map[string]any{"num": false})
	if err != nil {
		t.Fatalf("UTF-16 转码失败: %v", err)
	}
	if !strings.Contains(out2, "中") {
		t.Errorf("UTF-16LE 应转出中文——got:\n%s", out2)
	}
}

// buildXlsx — 造最小 xlsx zip(sharedStrings + 单 sheet)
func buildXlsx(t *testing.T, path string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	write := func(name, content string) {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	write("[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`)
	write("xl/sharedStrings.xml", `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>你好</t></si><si><t>world</t></si></sst>`)
	write("xl/worksheets/sheet1.xml", `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1"><v>42</v></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>内联</t></is></c></row></sheetData></worksheet>`)
	zw.Close()
	f.Close()
}

// TestReadXlsx — 内置 zip+XML 单 sheet TSV
func TestReadXlsx(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-xlsx-%d", time.Now().UnixNano()))
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "data.xlsx")
	buildXlsx(t, p)
	ec := &ExecContext{WorkDir: dir}
	out, err := ec.executeRead(context.Background(), "data.xlsx", map[string]any{"num": false}, nil)
	if err != nil {
		t.Fatalf("xlsx 抽取失败: %v", err)
	}
	for _, want := range []string{"type=\"xlsx\"", "你好\t42", "内联", "已从 XLSX 抽取"} {
		if !strings.Contains(out, want) {
			t.Errorf("xlsx 缺 %q——got:\n%s", want, out)
		}
	}
}

// TestDetectReadKind — 魔数探测(纯函数)
func TestDetectReadKind(t *testing.T) {
	cases := []struct {
		head []byte
		want string
	}{
		{[]byte("%PDF-1.4"), "pdf"},
		{[]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, "img"},
		{[]byte("ID3\x04\x00"), "audio"},
		{[]byte{0xFF, 0xFE, 'a', 0x00}, "utf16le"},
		{[]byte("hello text file"), "text"},
		{[]byte{0x00, 0x01, 0x02}, "binary"},
	}
	for _, c := range cases {
		if got := detectReadKind(c.head, "/tmp/x", 10); got != c.want {
			t.Errorf("detectReadKind(%v) = %q——want %q", c.head[:min(4, len(c.head))], got, c.want)
		}
	}
}

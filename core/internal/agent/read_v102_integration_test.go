package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===== read v1.0.2 全面集成测试:真实 CLI 抽取链(pdftotext/textutil/pandoc)=====

// buildMinimalPDF — 手工造含文本的最小合法 PDF(动态 xref 偏移)
func buildMinimalPDF(t *testing.T, path, text string) {
	var b bytes.Buffer
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		"",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	stream := "BT /F1 24 Tf 72 720 Td (" + text + ") Tj ET"
	// 对象 4 = 流,长度需先算
	streamFull := "stream\n" + stream + "\nendstream"
	objs[3] = fmt.Sprintf("<< /Length %d >>\n%s", len(stream), streamFull)

	offsets := []int{0} // 0 = free
	b.WriteString("%PDF-1.4\n")
	for i, body := range objs {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xrefPos := b.Len()
	b.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for _, o := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	b.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n")
	fmt.Fprintf(&b, "%d\n%%%%EOF", xrefPos)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildMinimalDocx — 最小 docx(word/document.xml)
func buildMinimalDocx(t *testing.T, path, text string) {
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w := func(n, c string) {
		ww, _ := zw.Create(n)
		ww.Write([]byte(c))
	}
	w("[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	w("_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	w("word/document.xml", `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>`+text+`</w:t></w:r></w:p></w:body></w:document>`)
	zw.Close()
	f.Close()
}

func TestReadExtractPDF(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-pdf-%d", time.Now().UnixNano()))
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "doc.pdf")
	buildMinimalPDF(t, p, "Hello PDF read v1.0.2 test")
	ec := &ExecContext{WorkDir: dir}
	out, err := ec.executeRead(context.Background(), "doc.pdf", map[string]any{"num": false}, nil)
	if err != nil {
		t.Fatalf("PDF 抽取失败: %v", err)
	}
	for _, want := range []string{"type=\"pdf\"", "Hello PDF read v1.0.2 test", "已从 PDF 抽取"} {
		if !strings.Contains(out, want) {
			t.Errorf("PDF 缺 %q——got:\n%s", want, out)
		}
	}
}

func TestReadExtractDocxRtfHtml(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-office-%d", time.Now().UnixNano()))
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)
	cases := []struct {
		name, text string
		build      func(t *testing.T, path, text string)
		wantType   string
	}{
		{"d.docx", "Hello docx 中文", buildMinimalDocx, "docx"},
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.name)
		if c.build != nil {
			c.build(t, p, c.text)
		} else {
			os.WriteFile(p, []byte(c.text), 0o644)
		}
		ec := &ExecContext{WorkDir: dir}
		out, err := ec.executeRead(context.Background(), c.name, map[string]any{"num": false}, nil)
		if err != nil {
			t.Errorf("%s 抽取失败: %v", c.name, err)
			continue
		}
		if !strings.Contains(out, c.text) || !strings.Contains(out, "type=\""+c.wantType+"\"") {
			t.Errorf("%s 抽取缺文本/类型——got:\n%s", c.name, out)
		}
	}
	// rtf/html 走 textutil
	rtf := "{\\rtf1\\ansi{\\fonttbl{\\f0 Helvetica;}}\\f0\\pard Hello RTF \\'d6\\'d0\\'ce\\'c4\\par}"
	os.WriteFile(filepath.Join(dir, "r.rtf"), []byte(rtf), 0o644)
	os.WriteFile(filepath.Join(dir, "h.html"), []byte("<html><body><h1>标题</h1><p>Hello HTML body</p></body></html>"), 0o644)
	for _, n := range []string{"r.rtf", "h.html"} {
		ec := &ExecContext{WorkDir: dir}
		out, err := ec.executeRead(context.Background(), n, map[string]any{"num": false}, nil)
		if err != nil {
			t.Errorf("%s textutil 抽取失败: %v", n, err)
			continue
		}
		if !strings.Contains(out, "Hello") {
			t.Errorf("%s 应抽到文本——got:\n%s", n, out)
		}
	}
}

func TestReadPaginationAndRaw(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-page-%d", time.Now().UnixNano()))
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)
	var sb strings.Builder
	for i := 1; i <= 120; i++ {
		fmt.Fprintf(&sb, "row %d\n", i)
	}
	os.WriteFile(filepath.Join(dir, "big.txt"), []byte(sb.String()), 0o644)
	ec := &ExecContext{WorkDir: dir}
	// 分页: 第二页(JSON 数字为 float64——与线上 decode 一致)
	out, err := ec.executeRead(context.Background(), "big.txt", map[string]any{"offset": float64(100), "limit": float64(10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "100 │ row 100") || !strings.Contains(out, "109 │ row 109") || !strings.Contains(out, "[分页]") {
		t.Errorf("分页错——got:\n%s", out)
	}
	// format=raw: 原样输出——不委托/不编码修正(含 null 也原样——raw 语义)
	os.WriteFile(filepath.Join(dir, "img.png"), []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, 0o644)
	outImg, err := ec.executeRead(context.Background(), "img.png", map[string]any{"format": "raw"}, nil)
	if err != nil {
		t.Errorf("raw 图像应原样读出(不委托)——got %v", err)
	} else if !strings.Contains(outImg, "format=raw 强制按原文读取") {
		t.Errorf("raw 应带注记——got:\n%s", outImg)
	}
	// 纯文本 raw 无差异
	outR, err := ec.executeRead(context.Background(), "big.txt", map[string]any{"format": "raw", "num": false, "limit": 2}, nil)
	if err != nil || !strings.Contains(outR, "row 1") {
		t.Errorf("text format=raw 应正常——got %v", err)
	}
}

func TestReadCsvJson(t *testing.T) {
	dir := filepath.Join("/tmp", fmt.Sprintf("zerg-read-data-%d", time.Now().UnixNano()))
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "d.csv"), []byte("a,b,c\n1,2,3\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "j.json"), []byte("{\"k\": \"v\"}\n"), 0o644)
	for _, n := range []string{"d.csv", "j.json"} {
		ec := &ExecContext{WorkDir: dir}
		out, err := ec.executeRead(context.Background(), n, nil, nil)
		if err != nil {
			t.Errorf("%s 失败: %v", n, err)
			continue
		}
		if !strings.Contains(out, "type=\"text\"") {
			t.Errorf("%s 应为 text 类型——got:\n%s", n, out)
		}
	}
}

package main

// verify_integrity_test.go — 待修补 #13 的另一半（verify --integrity）在 CLI 层的验收。
//
// 全部用 t.TempDir()，不碰真实目录；不改动既有 probe 用例。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

func cliSHA(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// captureStdout 抓取 fn 执行期间写到 os.Stdout 的内容。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

// writeVerifyRecord 写一条最小可用记录（一条 files[]，格式合法）。
func writeVerifyRecord(t *testing.T, path, name, sha string, size int64) {
	t.Helper()
	rec := modelreg.Record{
		Schema:    modelreg.SchemaV1,
		ID:        "cli-integrity",
		Digest:    "sha256:" + strings.Repeat("a", 64),
		Files:     []modelreg.File{{Role: "weights", Name: name, SHA256: sha, Size: size}},
		License:   modelreg.License{SPDX: "MIT", Commercial: "yes"},
		SourceURL: "https://example.org",
	}
	b, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVerifyIntegrity_OK(t *testing.T) {
	dir := t.TempDir()
	body := []byte("cli-integrity-ok")
	if err := os.WriteFile(filepath.Join(dir, "w.gguf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--base-dir", dir}); code != 0 {
			t.Errorf("ok 情况应 exit 0，实际 %d", code)
		}
	})
	if !strings.Contains(out, "[ok]") || !strings.Contains(out, "integrity: ok 1 条，不符 0 条") {
		t.Fatalf("输出应含 ok 结论与汇总：\n%s", out)
	}
}

func TestCmdVerifyIntegrity_SHA256MismatchExit2(t *testing.T) {
	dir := t.TempDir()
	body := []byte("cli-integrity-sha")
	p := filepath.Join(dir, "w.gguf")
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	altered := append([]byte(nil), body...)
	altered[1] ^= 0x01 // 改一字节，大小不变 → 必落 sha_mismatch
	if err := os.WriteFile(p, altered, 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--base-dir", dir}); code != 2 {
			t.Errorf("sha 不符应 exit 2，实际 %d", code)
		}
	})
	if !strings.Contains(out, "[sha_mismatch]") {
		t.Fatalf("输出应含 sha_mismatch：\n%s", out)
	}
}

func TestCmdVerifyIntegrity_MissingExit2(t *testing.T) {
	dir := t.TempDir()
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "gone.gguf", strings.Repeat("b", 64), 11)

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--base-dir", dir}); code != 2 {
			t.Errorf("文件缺失应 exit 2，实际 %d", code)
		}
	})
	if !strings.Contains(out, "[missing]") {
		t.Fatalf("输出应含 missing：\n%s", out)
	}
}

// 回归：不带 --integrity 时行为与从前完全一致（exit 0，且不打印完整性块）。
func TestCmdVerify_NoIntegrityUnchanged(t *testing.T) {
	dir := t.TempDir()
	body := []byte("no-flag")
	p := filepath.Join(dir, "w.gguf")
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	// 故意把 sha 写错：不带 --integrity 时也不该被发现（格式仍合法）。
	writeVerifyRecord(t, recPath, "w.gguf", strings.Repeat("f", 64), int64(len(body)))

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath}); code != 0 {
			t.Errorf("不带 --integrity 应仍 exit 0（只看记录格式），实际 %d", code)
		}
	})
	if strings.Contains(out, "完整性校验") {
		t.Fatalf("不带 --integrity 不该打印完整性块：\n%s", out)
	}
	if !strings.Contains(out, "只校验【记录格式】") {
		t.Fatalf("应保留原有提醒：\n%s", out)
	}
}

// 不带 --integrity 时 JSON 形状不变（不得出现 integrity 字段）。
func TestCmdVerify_JSONShapeUnchangedWithoutIntegrity(t *testing.T) {
	dir := t.TempDir()
	body := []byte("json-shape")
	if err := os.WriteFile(filepath.Join(dir, "w.gguf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--json"}); code != 0 {
			t.Errorf("应 exit 0，实际 %d", code)
		}
	})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &raw); err != nil {
		t.Fatalf("JSON 解析失败：%v\n%s", err, out)
	}
	if _, ok := raw["integrity"]; ok {
		t.Fatalf("不带 --integrity 时 JSON 不得含 integrity 字段（回归）：\n%s", out)
	}
	for _, k := range []string{"path", "schema", "errors", "findings"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("原有 JSON 字段 %q 丢失：\n%s", k, out)
		}
	}
}

// --json + --integrity：形状必须含逐文件结果。
func TestCmdVerifyIntegrity_JSONShape(t *testing.T) {
	dir := t.TempDir()
	body := []byte("json-integrity")
	if err := os.WriteFile(filepath.Join(dir, "w.gguf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--base-dir", dir, "--json"}); code != 0 {
			t.Errorf("应 exit 0，实际 %d", code)
		}
	})
	var payload struct {
		Path      string                    `json:"path"`
		Errors    int                       `json:"errors"`
		Integrity *modelreg.IntegrityReport `json:"integrity"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("JSON 解析失败：%v\n%s", err, out)
	}
	if payload.Integrity == nil {
		t.Fatalf("--json --integrity 必须含 integrity 字段：\n%s", out)
	}
	if len(payload.Integrity.Files) != 1 || payload.Integrity.Files[0].Status != modelreg.IntegrityOK {
		t.Fatalf("逐文件结果不对：%+v", payload.Integrity)
	}
	if payload.Integrity.Files[0].Role != "weights" || payload.Integrity.Files[0].Path == "" {
		t.Fatalf("逐文件结果应含 role 与 path：%+v", payload.Integrity.Files[0])
	}
}

// 只有文件名、又没给 --base-dir → unverifiable；非 strict 不算失败(exit 0)，strict 算(exit 2)。
func TestCmdVerifyIntegrity_UnverifiableStrict(t *testing.T) {
	dir := t.TempDir()
	body := []byte("needs-base")
	if err := os.WriteFile(filepath.Join(dir, "w.gguf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json") // name 只有文件名，且不给 --base-dir
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	out := captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity"}); code != 0 {
			t.Errorf("非 strict 下 unverifiable 不该失败，应 exit 0，实际 %d", code)
		}
	})
	if !strings.Contains(out, "[unverifiable]") {
		t.Fatalf("应报 unverifiable：\n%s", out)
	}

	captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--strict"}); code != 2 {
			t.Errorf("strict 下 unverifiable 应 exit 2，实际 %d", code)
		}
	})
}

// --base-dir 给的不是目录 → 参数/环境错误 exit 3。
func TestCmdVerifyIntegrity_BadBaseDirExit3(t *testing.T) {
	dir := t.TempDir()
	body := []byte("x")
	if err := os.WriteFile(filepath.Join(dir, "w.gguf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(dir, "rec.json")
	writeVerifyRecord(t, recPath, "w.gguf", cliSHA(body), int64(len(body)))

	captureStdout(t, func() {
		if code := cmdVerify([]string{recPath, "--integrity", "--base-dir", filepath.Join(dir, "nope")}); code != 3 {
			t.Errorf("--base-dir 非目录应 exit 3，实际 %d", code)
		}
	})
}

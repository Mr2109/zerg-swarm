package modelreg

// integrity_test.go — 待修补 #13 的另一半（--integrity）的单元测试。
//
// 反例优先，全部用 t.TempDir()，绝不触碰 ~/.zerg 或 ~/models 等真实目录。

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func shaHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func mustWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// recWithFiles 造一条只关心 files[] 的记录（格式细节不参与 integrity 判定）。
func recWithFiles(files ...File) *Record {
	return &Record{
		Schema:  SchemaV1,
		ID:      "integrity-test",
		Digest:  "sha256:" + strings.Repeat("a", 64),
		Files:   files,
		License: License{SPDX: "MIT", Commercial: "yes"},
	}
}

// 正例：小文件 sha 与 size 都对 → ok，且不判失败。
func TestVerifyIntegrity_OK(t *testing.T) {
	dir := t.TempDir()
	body := []byte("ZERG-INTEGRITY-BODY-OK")
	mustWrite(t, filepath.Join(dir, "model-Q4_K_M.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "model-Q4_K_M.gguf", SHA256: shaHex(body), Size: int64(len(body))})

	rep := VerifyIntegrity(rec, dir)
	if len(rep.Files) != 1 || rep.Files[0].Status != IntegrityOK {
		t.Fatalf("应 ok：%+v", rep.Files)
	}
	if rep.OK != 1 || rep.Problems != 0 || rep.Unverifiable != 0 {
		t.Fatalf("计数应为 ok=1/不符=0/不可定位=0：%+v", rep)
	}
	if rep.HasFailures(false) {
		t.Fatal("ok 不得判失败")
	}
}

// 反例：内容被改一个字节（大小不变）→ sha_mismatch，判失败。
func TestVerifyIntegrity_SHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	body := []byte("original-body-content")
	p := filepath.Join(dir, "w.gguf")
	mustWrite(t, p, body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: shaHex(body), Size: int64(len(body))})

	altered := append([]byte(nil), body...)
	altered[0] ^= 0xFF // 只改一个字节，大小不变 → 必须落 sha_mismatch 而非 size_mismatch
	mustWrite(t, p, altered)

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegritySHAMismatch {
		t.Fatalf("改一字节应 sha_mismatch：%+v", rep.Files[0])
	}
	if rep.Problems != 1 || !rep.HasFailures(false) {
		t.Fatalf("sha_mismatch 必须判失败：%+v", rep)
	}
	if rep.Files[0].SHAComputed != shaHex(altered) {
		t.Fatalf("应记下实测 sha：%+v", rep.Files[0])
	}
}

// 反例：大小与记录不符 → size_mismatch，判失败。
func TestVerifyIntegrity_SizeMismatch(t *testing.T) {
	dir := t.TempDir()
	body := []byte("1234567890")
	mustWrite(t, filepath.Join(dir, "w.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: shaHex(body), Size: int64(len(body)) + 5})

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegritySizeMismatch {
		t.Fatalf("大小不符应 size_mismatch：%+v", rep.Files[0])
	}
	if rep.Files[0].SizeActual != int64(len(body)) || !rep.Files[0].SizeChecked {
		t.Fatalf("应记录实测大小与已比对标记：%+v", rep.Files[0])
	}
	if !rep.HasFailures(false) {
		t.Fatal("size_mismatch 必须判失败")
	}
}

// 反例：文件不存在 → missing，判失败。
func TestVerifyIntegrity_Missing(t *testing.T) {
	dir := t.TempDir()
	rec := recWithFiles(File{Role: "weights", Name: "not-there.gguf", SHA256: strings.Repeat("b", 64), Size: 42})

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegrityMissing {
		t.Fatalf("文件不存在应 missing：%+v", rep.Files[0])
	}
	if rep.Problems != 1 || !rep.HasFailures(false) {
		t.Fatalf("missing 必须判失败：%+v", rep)
	}
}

// 反例：只有文件名、又没给 --base-dir → unverifiable；非 strict 不算失败，strict 才算。
func TestVerifyIntegrity_UnverifiableNeedsBaseDir(t *testing.T) {
	dir := t.TempDir()
	body := []byte("needs-base-dir")
	mustWrite(t, filepath.Join(dir, "w.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: shaHex(body), Size: int64(len(body))})

	rep := VerifyIntegrity(rec, "") // 未给 base-dir
	if rep.Files[0].Status != IntegrityUnverifiable {
		t.Fatalf("不给 base-dir 的相对名应 unverifiable：%+v", rep.Files[0])
	}
	if !strings.Contains(rep.Files[0].Note, "--base-dir") {
		t.Fatalf("提示应指向 --base-dir：%q", rep.Files[0].Note)
	}
	if rep.HasFailures(false) {
		t.Fatal("unverifiable 在非 strict 下不算失败")
	}
	if !rep.HasFailures(true) {
		t.Fatal("unverifiable 在 strict 下必须算失败")
	}

	// 给了 base-dir → 定位成功 → ok
	rep2 := VerifyIntegrity(rec, dir)
	if rep2.Files[0].Status != IntegrityOK {
		t.Fatalf("给 base-dir 后应 ok：%+v", rep2.Files[0])
	}
}

// size 为 0/缺失 → 跳过大小比对（如实注明），但 sha 仍要核对。
func TestVerifyIntegrity_SizeZeroSkipsSizeCheck(t *testing.T) {
	dir := t.TempDir()
	body := []byte("no-size-recorded")
	mustWrite(t, filepath.Join(dir, "w.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: shaHex(body), Size: 0})

	rep := VerifyIntegrity(rec, dir)
	f := rep.Files[0]
	if f.Status != IntegrityOK {
		t.Fatalf("size 缺失且 sha 对 → ok：%+v", f)
	}
	if f.SizeChecked {
		t.Fatalf("size 为 0 时应跳过大小比对：%+v", f)
	}
	if !strings.Contains(f.Note, "跳过") {
		t.Fatalf("应注明跳过大小比对：%q", f.Note)
	}
	if f.SizeActual != int64(len(body)) {
		t.Fatalf("仍应记下实测大小：%+v", f)
	}
}

// 绝对路径无需 base-dir 即可定位。
func TestVerifyIntegrity_AbsoluteName(t *testing.T) {
	body := []byte("absolute")
	p := filepath.Join(t.TempDir(), "abs.gguf")
	mustWrite(t, p, body)
	rec := recWithFiles(File{Role: "weights", Name: p, SHA256: shaHex(body), Size: int64(len(body))})

	rep := VerifyIntegrity(rec, "")
	if rep.Files[0].Status != IntegrityOK {
		t.Fatalf("绝对路径应 ok（无需 base-dir）：%+v", rep.Files[0])
	}
}

// 流式哈希：一个较大文件（数 MB）也逐块读取（不整体读入内存），结果必须正确。
func TestVerifyIntegrity_StreamsLargerFile(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, 4<<20) // 4 MiB
	for i := range big {
		big[i] = byte(i * 31 % 251)
	}
	p := filepath.Join(dir, "big.gguf")
	mustWrite(t, p, big)
	rec := recWithFiles(File{Role: "weights", Name: "big.gguf", SHA256: shaHex(big), Size: int64(len(big))})

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegrityOK || rep.Files[0].SizeActual != int64(len(big)) {
		t.Fatalf("大文件流式校验应 ok 且大小正确：%+v", rep.Files[0])
	}
}

// 一条记录里混合多种结论：计数必须分别落对。
func TestVerifyIntegrity_MixedCounts(t *testing.T) {
	dir := t.TempDir()
	okBody := []byte("ok-body")
	okPath := filepath.Join(dir, "ok.gguf")
	mustWrite(t, okPath, okBody)
	badBody := []byte("bad-body")
	badPath := filepath.Join(dir, "bad.gguf")
	mustWrite(t, badPath, badBody)

	rec := recWithFiles(
		File{Role: "weights", Name: "ok.gguf", SHA256: shaHex(okBody), Size: int64(len(okBody))},
		File{Role: "mmproj", Name: "gone.gguf", SHA256: strings.Repeat("c", 64), Size: 7},              // missing
		File{Role: "tokenizer", Name: "bad.gguf", SHA256: strings.Repeat("d", 64), Size: int64(len(badBody))}, // sha_mismatch
	)
	rep := VerifyIntegrity(rec, dir)
	if rep.OK != 1 || rep.Problems != 2 || rep.Unverifiable != 0 || len(rep.Files) != 3 {
		t.Fatalf("计数不对：%+v", rep)
	}
	if rep.Files[0].Status != IntegrityOK || rep.Files[1].Status != IntegrityMissing || rep.Files[2].Status != IntegritySHAMismatch {
		t.Fatalf("逐文件状态不对：%+v", rep.Files)
	}
	// 索引与 role 要如实回填
	if rep.Files[1].Index != 1 || rep.Files[1].Role != "mmproj" {
		t.Fatalf("index/role 未如实回填：%+v", rep.Files[1])
	}
}

// 空 sha256（记录里没有）→ 无法比对，报 unverifiable。
func TestVerifyIntegrity_EmptyRecordedSHA(t *testing.T) {
	dir := t.TempDir()
	body := []byte("x")
	mustWrite(t, filepath.Join(dir, "w.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: "", Size: int64(len(body))})

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegrityUnverifiable {
		t.Fatalf("记录缺 sha256 应 unverifiable：%+v", rep.Files[0])
	}
}

// 带 "sha256:" 前缀的记录值也要能正确比对（归一化）。
func TestVerifyIntegrity_PrefixedSHA(t *testing.T) {
	dir := t.TempDir()
	body := []byte("prefixed")
	mustWrite(t, filepath.Join(dir, "w.gguf"), body)
	rec := recWithFiles(File{Role: "weights", Name: "w.gguf", SHA256: "sha256:" + shaHex(body), Size: int64(len(body))})

	rep := VerifyIntegrity(rec, dir)
	if rep.Files[0].Status != IntegrityOK {
		t.Fatalf("sha256: 前缀应被归一化后比对成功：%+v", rep.Files[0])
	}
}

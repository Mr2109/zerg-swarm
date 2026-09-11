package modelreg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 假 OpenAI 兼容端点 ─────────────────────────────────────────────────────
// 测试一律用 httptest 假端点，不碰真实外网/真实引擎（任务硬规则）。

func fakeEngine(t *testing.T, visionErrMsg string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"object":"list","data":[{"id":"fake-llama","object":"model"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			bs := string(body)
			switch {
			case strings.Contains(bs, `"tools"`):
				// 工具调用：返回结构化 tool_calls
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{\"tz\":\"UTC\"}"}}]}}]}`)
			case strings.Contains(bs, "image_url"):
				// 视觉：假端点一律 400（模拟"不支持图"），用于验证"必须实测、不许猜"
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error":{"message":%q,"type":"invalid_request_error"}}`, visionErrMsg)
			default:
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ── 假 GGUF 构造 ──────────────────────────────────────────────────────────

func le32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }
func le64(v uint64) []byte { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, v); return b }
func gstr(s string) []byte { return append(le64(uint64(len(s))), []byte(s)...) }

func kv(key string, vtype uint32, val []byte) []byte {
	out := gstr(key)
	out = append(out, le32(vtype)...)
	return append(out, val...)
}

func makeGGUF(kvs ...[]byte) []byte {
	out := []byte("GGUF")
	out = append(out, le32(3)...)                // version
	out = append(out, le64(0)...)                // tensor_count
	out = append(out, le64(uint64(len(kvs)))...) // kv_count
	for _, x := range kvs {
		out = append(out, x...)
	}
	return out
}

func writeTestGGUF(t *testing.T, dir, name string) string {
	t.Helper()
	// 含一个字符串数组（token 表），用来验证 skipValue 能跳过巨大数组而不炸。
	tokens := append(le32(ggufTypeString), le64(3)...)
	tokens = append(tokens, gstr("hello")...)
	tokens = append(tokens, gstr("world")...)
	tokens = append(tokens, gstr("<|im_end|>")...)

	gguf := makeGGUF(
		kv("general.architecture", ggufTypeString, gstr("llama")),
		kv("general.name", ggufTypeString, gstr("TestModel-8B")),
		kv("general.license", ggufTypeString, gstr("apache-2.0")),
		kv("llama.context_length", ggufTypeUint32, le32(4096)),
		kv("llama.block_count", ggufTypeUint32, le32(32)),
		kv("tokenizer.ggml.tokens", ggufTypeArray, tokens),
		kv("tokenizer.chat_template", ggufTypeString, gstr("{{ .Prompt }}")),
	)
	if dir == "" {
		dir = t.TempDir()
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, gguf, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ── 探测器用例（反例优先：故意造不通过，断言必须红/必须如实记 false）──────

func TestOnlineProbes(t *testing.T) {
	cases := []struct {
		name       string
		visionErr  string
		wantClass  string
		evidSubstr string
	}{
		{"vision_unsupported", "image input is not supported by this model", FailNoVision, "image input"},
		{"vision_mmproj_missing", "this model does not support images: mmproj file not loaded", FailMmprojMiss, "mmproj"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeEngine(t, tc.visionErr)
			ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}

			// 文本：正常回话 → true
			txt := ProbeText(ep)
			if !txt.Value {
				t.Fatalf("文本探测应通过，实际 %+v", txt)
			}
			if txt.Trace.Probe != EvidenceText {
				t.Fatalf("evidence 名不对：%s", txt.Trace.Probe)
			}

			// 视觉：假端点对 image 返回 400 → 必须 false，且 evidence 带失败原因
			vis := ProbeVision(ep)
			if vis.Value {
				t.Fatal("视觉探测必须为 false：假端点对 image 返回 400，绝不许因为'模型自称能看图'就写 true")
			}
			if vis.FailureClass != tc.wantClass {
				t.Fatalf("视觉失败分类应为 %s，实际 %s", tc.wantClass, vis.FailureClass)
			}
			c := capabilityOf("vision", EvidenceVision, vis)
			if c.Value || c.Source != "probed" {
				t.Fatalf("视觉能力断言不对：%+v", c)
			}
			if !strings.Contains(c.Evidence, EvidenceVision) {
				t.Fatalf("evidence 必须带探测器名：%s", c.Evidence)
			}
			if !strings.Contains(c.Evidence, tc.wantClass) {
				t.Fatalf("evidence 必须带失败分类 %s：%s", tc.wantClass, c.Evidence)
			}
			if !strings.Contains(c.Evidence, tc.evidSubstr) {
				t.Fatalf("evidence 必须带失败原因 %q：%s", tc.evidSubstr, c.Evidence)
			}

			// 工具：假端点返回 tool_calls → true
			tools := ProbeTools(ep)
			if !tools.Value {
				t.Fatalf("工具探测应通过（假端点返回 tool_calls），实际 %+v", tools)
			}

			// 嵌入 / 重排：假端点 404 → 如实记 false 且注明端点不支持
			emb := ProbeEmbedding(ep)
			if emb.Value || emb.FailureClass != FailUnsupported {
				t.Fatalf("嵌入应为 false/unsupported，实际 %+v", emb)
			}
			ce := capabilityOf("embedding", EvidenceEmbedding, emb)
			if !strings.Contains(ce.Evidence, "404") {
				t.Fatalf("嵌入 evidence 应注明端点不支持（404）：%s", ce.Evidence)
			}
			rr := ProbeRerank(ep)
			if rr.Value || rr.FailureClass != FailUnsupported {
				t.Fatalf("重排应为 false/unsupported，实际 %+v", rr)
			}
		})
	}
}

// 硬规则回归防护：digest 只由内容（role+sha256）合成，绝不随路径/文件名变化。
func TestSynthesizeDigestIgnoresPathAndName(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	mustWrite := func(dir, name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// 同一批内容，两份不同的文件名/目录
	wA, err := HashFile(mustWrite(dirA, "model-Q4_K_M.gguf", "WEIGHTS-CONTENT"))
	if err != nil {
		t.Fatal(err)
	}
	mA, err := HashFile(mustWrite(dirA, "mmproj-f16.gguf", "MMPROJ-CONTENT"))
	if err != nil {
		t.Fatal(err)
	}
	wB, err := HashFile(mustWrite(dirB, "totally-renamed.gguf", "WEIGHTS-CONTENT"))
	if err != nil {
		t.Fatal(err)
	}
	mB, err := HashFile(mustWrite(dirB, "renamed-mmproj-x.gguf", "MMPROJ-CONTENT"))
	if err != nil {
		t.Fatal(err)
	}

	if wA.Name == wB.Name || mA.Name == mB.Name {
		t.Fatal("本用例前提是文件名不同")
	}
	if wA.Role != "weights" || mA.Role != "mmproj" || wB.Role != "weights" || mB.Role != "mmproj" {
		t.Fatalf("role 分类不对：%+v %+v %+v %+v", wA, mA, wB, mB)
	}

	dA := SynthesizeDigest([]File{wA, mA})
	dB := SynthesizeDigest([]File{wB, mB})
	if dA != dB {
		t.Fatalf("同一批内容、换目录换文件名后 digest 必须相同：\n  A=%s\n  B=%s", dA, dB)
	}
	if !strings.HasPrefix(dA, "sha256:") || len(dA) != len("sha256:")+64 {
		t.Fatalf("digest 形态不对：%s", dA)
	}

	// 顺序无关
	dRev := SynthesizeDigest([]File{mA, wA})
	if dRev != dA {
		t.Fatalf("files[] 顺序不该影响 digest：%s vs %s", dRev, dA)
	}

	// 反例：内容变了 digest 必须变
	mDiff, _ := HashFile(mustWrite(dirB, "renamed-mmproj-x.gguf", "MMPROJ-CONTENT-CHANGED"))
	if SynthesizeDigest([]File{wB, mDiff}) == dA {
		t.Fatal("内容变了 digest 必须变")
	}

	// 反例：digest 里不得出现文件名
	if strings.Contains(dA, wA.Name) || strings.Contains(dA, filepath.Base(dirA)) {
		t.Fatal("digest 不得包含文件名/路径")
	}
}

// 生成的 Record 必须能过自己的校验（Verify 无 error）。
func TestProbeRecordPassesVerify(t *testing.T) {
	p := writeTestGGUF(t, "", "TestModel-8B-Q4_K_M.gguf")
	rec, rep, err := Probe(ProbeOptions{Target: p, Now: func() time.Time { return time.Unix(0, 0).UTC() }})
	if err != nil {
		t.Fatalf("本地 GGUF 探测不应报错：%v", err)
	}
	if len(rec.Files) != 1 || rec.Files[0].SHA256 == "" {
		t.Fatalf("files[] 不对：%+v", rec.Files)
	}
	if !strings.HasPrefix(rec.Digest, "sha256:") {
		t.Fatalf("digest 必须 sha256: 前缀：%s", rec.Digest)
	}
	if rec.ContextWindow != 4096 {
		t.Fatalf("应从 GGUF 读到 context_window=4096，实际 %d", rec.ContextWindow)
	}
	if rec.License.SPDX != "Apache-2.0" {
		t.Fatalf("应从 GGUF 读到许可 Apache-2.0，实际 %q", rec.License.SPDX)
	}
	if rec.License.Commercial != "unknown" {
		t.Fatalf("commercial 必须默认 unknown（绝不 yes），实际 %q", rec.License.Commercial)
	}
	if rep.ChatTemplate != "from_gguf" {
		t.Fatalf("chat_template 来源应为 from_gguf，实际 %q", rep.ChatTemplate)
	}
	if got := rec.EngineRecipes["llama.cpp"].ChatTemplate; got != "from_gguf" {
		t.Fatalf("EngineRecipe.chat_template 应为 from_gguf，实际 %q", got)
	}
	findings := Verify(rec, false)
	if n := CountErrors(findings); n != 0 {
		t.Fatalf("probe 产物必须过 Verify(rec,false)，实际 %d 条 error：%+v", n, findings)
	}

	// 同一内容换个文件名再探一次：digest 必须不变（硬规则回归）
	q := filepath.Join(t.TempDir(), "renamed-entirely.gguf")
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(q, b, 0o600); err != nil {
		t.Fatal(err)
	}
	rec2, _, err := Probe(ProbeOptions{Target: q})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Digest != rec2.Digest {
		t.Fatalf("同一内容、不同文件名 digest 必须相同：%s vs %s", rec.Digest, rec2.Digest)
	}
}

// 端点探测（无本地文件）也要产出能过 Verify 的记录。
func TestProbeEndpointRecordPassesVerify(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	rec, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("端点探测不应报错：%v", err)
	}
	if len(rec.Files) != 1 || rec.Files[0].Role != "endpoint" {
		t.Fatalf("端点探测无本地文件时应写一条占位建材料：%+v", rec.Files)
	}
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("端点探测产物必须过 Verify，实际 %d 条 error：%+v", n, Verify(rec, false))
	}
	// 视觉能力必须 false（假端点 400）
	if hasCap(rec.Capabilities, "vision") {
		t.Fatal("假端点对 image 返回 400，vision 必须为 false")
	}
	if !hasCap(rec.Capabilities, "text") || !hasCap(rec.Capabilities, "tools") {
		t.Fatalf("text/tools 应为 true：%+v", rec.Capabilities)
	}
	// notes 必须留痕
	if !strings.Contains(rec.Notes, "probe_trace") || !strings.Contains(rec.Notes, EvidenceVision) {
		t.Fatalf("notes 必须含 probe_trace 及视觉探测留痕：%s", rec.Notes)
	}
	if len(rep.Traces) == 0 {
		t.Fatal("应产出探测留痕")
	}
}

// P1：空文件 → cant_start（CLI 映射 exit 1），不崩。
func TestProbeEmptyFileCantStart(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.gguf")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Probe(ProbeOptions{Target: p})
	var te *ProbeTargetError
	if !errors.As(err, &te) {
		t.Fatalf("空文件应报 ProbeTargetError，实际 %v", err)
	}
	if te.FailureClass != FailCantStart {
		t.Fatalf("空文件失败分类应为 cant_start，实际 %s", te.FailureClass)
	}
}

// P2：不存在的路径 → cant_start，且不崩。
func TestProbeMissingPathCantStart(t *testing.T) {
	_, _, err := Probe(ProbeOptions{Target: filepath.Join(t.TempDir(), "nope.gguf")})
	var te *ProbeTargetError
	if !errors.As(err, &te) {
		t.Fatalf("不存在路径应报 ProbeTargetError，实际 %v", err)
	}
	if te.FailureClass != FailCantStart {
		t.Fatalf("失败分类应为 cant_start，实际 %s", te.FailureClass)
	}
}

// P4：端点不可达 → UnreachableError（CLI 映射 exit 4），最多 1 次不重试。
func TestProbeUnreachableEndpoint(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close() // 端口空出来 → 连接被拒

	_, _, perr := Probe(ProbeOptions{Target: "http://" + addr, Timeout: 2 * time.Second})
	var ue *UnreachableError
	if !errors.As(perr, &ue) {
		t.Fatalf("端点不可达应报 UnreachableError，实际 %v", perr)
	}
}

// probe.meta：GGUF 头解析（含跳过字符串数组）。
func TestProbeMetaGGUF(t *testing.T) {
	p := writeTestGGUF(t, "", "meta.gguf")
	meta, tr, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatalf("解析 GGUF 失败：%v", err)
	}
	if !tr.OK || tr.Probe != EvidenceMetaGGUF {
		t.Fatalf("meta 留痕不对：%+v", tr)
	}
	if meta.Architecture != "llama" || meta.ContextWindow != 4096 || meta.BlockCount != 32 {
		t.Fatalf("GGUF 元数据不对：%+v", meta)
	}
	if meta.ChatTemplate != "{{ .Prompt }}" {
		t.Fatalf("chat_template 不对：%q", meta.ChatTemplate)
	}
	if meta.LicenseSPDX != "apache-2.0" {
		t.Fatalf("license 不对：%q", meta.LicenseSPDX)
	}
	if meta.Truncated {
		t.Fatal("正常 GGUF 不该被判为截断")
	}

	// 反例：不是 GGUF 的文件 → no_meta，且不崩
	bad := filepath.Join(t.TempDir(), "not.gguf")
	if err := os.WriteFile(bad, []byte("this is not a gguf file at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, btr, berr := ProbeMetaGGUFFile(bad)
	if berr == nil {
		t.Fatal("非 GGUF 文件必须报错")
	}
	if btr.FailureClass != FailNoMeta {
		t.Fatalf("非 GGUF 失败分类应为 no_meta，实际 %s", btr.FailureClass)
	}
}

// 硬规则回归：同一端点的不同写法（host:port 与 host:port/v1）必须得到同一个 digest。
// 端点 URL 是"存放位置"，不是身份——否则同一台引擎换种写法就被判成"另一个模型"。
func TestProbeEndpointURLSpellingKeepsDigest(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	a, _, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Probe(ProbeOptions{Target: srv.URL + "/v1", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest {
		t.Fatalf("同一端点不同写法 digest 必须一致：%s vs %s", a.Digest, b.Digest)
	}
}

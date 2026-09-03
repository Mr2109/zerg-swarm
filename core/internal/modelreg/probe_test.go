package modelreg

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordBody 把一条记录按 Store.Put 落盘所用的同一序列化方式编码，用于逐字节比对。
// 待修补 #24：正文必须随内容确定——它才是内容寻址的锚。
func recordBody(t *testing.T, rec *Record) []byte {
	t.Helper()
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func bodyHash(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

// boolPtr 造一个 *bool（用于"引擎给不给结构化声明"这类可空入参）。
func boolPtr(v bool) *bool { return &v }

// ── 假 OpenAI 兼容端点 ─────────────────────────────────────────────────────
// 测试一律用 httptest 假端点，不碰真实外网/真实引擎（任务硬规则）。

// fakeEngine 是"没有结构化能力声明"的假端点：它不提供 /props，/v1/models 也没有
// models[].capabilities（待修补 #23 的反例场景）。等价于 fakeEngineProps(t, msg, nil)。
func fakeEngine(t *testing.T, visionErrMsg string) *httptest.Server {
	t.Helper()
	return fakeEngineProps(t, visionErrMsg, nil)
}

// fakeEngineProps 是假 OpenAI 兼容端点。propsVision 非 nil 时提供 llama.cpp 形态的
// 只读 /props，其 modalities.vision 取该值——这正是待修补 #23 要认的结构化信号
// （实测：llama.cpp build 10470 挂 mmproj 报 true、未挂报 false）。
// propsVision=nil 表示引擎不给该声明 → 归因只能落到 HTTP 码或不可判定。
func fakeEngineProps(t *testing.T, visionErrMsg string, propsVision *bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/props":
			if propsVision == nil {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
				return
			}
			fmt.Fprintf(w, `{"modalities":{"vision":%t,"video":false,"audio":false}}`, *propsVision)
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
		name        string
		visionErr   string
		wantClass   string
		evidSubstr  string
		propsVision *bool // 引擎是否给出结构化能力声明（nil = 不给）
		siblingMMP  bool  // 同目录是否放着 mmproj*.gguf（#23 的模型侧信号）
	}{
		// 纯文本模型：引擎结构化声明"无视觉"，本地也没有投影器建材 → no_vision。
		{"vision_unsupported", "image input is not supported by this model", FailNoVision, "image input", boolPtr(false), false},
		// VLM 未挂 mmproj：引擎声明无视觉 **且** 本地确有投影器建材却没被装载 → mmproj_missing。
		{"vision_mmproj_missing", "this model does not support images: mmproj file not loaded", FailMmprojMiss, "mmproj", boolPtr(false), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeEngineProps(t, tc.visionErr, tc.propsVision)
			ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}
			if tc.siblingMMP {
				// 模拟"本模型的投影器建材就在本地"（待修补 #23 模型侧信号，按模型名 token 配对）。
				ep.ModelDir, ep.WeightsName = writeModelDirWithMMProj(t)
			}

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

// assertLicenseTracePolicy 是待修补 #21 修改后的硬规则守护（取代批 3 的"产物必有 1 条留痕 error"）：
//
//	按待修补 #21 的规则：commercial=unknown 允许留痕为空（探测出来本来就没人审过许可）；
//	no/revenue_gated 才必填；任何情况下留痕非空即不许是占位值。
//
// 探针产物 commercial 恒为 unknown、留痕留空，于是**必须零 error**（这正是 #21 的核心）；
// 同时守住两条不许开洞的反例：改成 no 且留痕空 → 必须 error；写占位值 → 必须 error。
func assertLicenseTracePolicy(t *testing.T, rec *Record) {
	t.Helper()
	if rec.License.Commercial != "unknown" {
		t.Fatalf("本断言前提是探针产物 commercial=unknown，实际 %q", rec.License.Commercial)
	}
	if rec.License.AcceptedBy != "" || rec.License.AcceptedAt != "" {
		t.Fatalf("探针不许写占位值：accepted_by=%q accepted_at=%q", rec.License.AcceptedBy, rec.License.AcceptedAt)
	}
	if findings := Verify(rec, false); CountErrors(findings) != 0 {
		t.Fatalf("unknown 允许留痕为空（待修补 #21），探针产物不该有 error，实际：%+v", findings)
	}
	if !strings.Contains(rec.Notes, "#21") {
		t.Fatalf("notes 必须写明许可留痕规则（待修补 #21）：%s", rec.Notes)
	}
	// 反例一：同一记录改成 no 且留痕为空 → 必须报 error（证明规则没被放宽）
	restricted := *rec
	restricted.License.Commercial = "no"
	if n := CountErrors(Verify(&restricted, false)); n == 0 {
		t.Fatal("no 且无留痕必须报 error：留痕规则不许被放宽")
	}
	// 反例二：占位值必须被拒
	placeholder := *rec
	placeholder.License.AcceptedBy = "unset"
	placeholder.License.AcceptedAt = "2026-09-12T00:00:00Z"
	if n := CountErrors(Verify(&placeholder, false)); n == 0 {
		t.Fatal("占位留痕必须报 error：不许给门禁开洞")
	}
}

// 硬规则守护：记录里任何地方都不许出现 unset/pending 之类占位串（那是骗门禁）。
func TestProbeRecordHasNoPlaceholderLicenseFields(t *testing.T) {
	p := writeTestGGUF(t, "", "NoSubstitution-Q4_K_M.gguf")
	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	js := strings.ToLower(string(b))
	for _, bad := range []string{"unset", "pending", "tbd", "n/a", "todo", "placeholder"} {
		if strings.Contains(js, bad) {
			t.Fatalf("记录里出现占位值 %q（禁止：等于给门禁开洞）\n%s", bad, js)
		}
	}
	if rec.License.AcceptedBy != "" || rec.License.AcceptedAt != "" {
		t.Fatalf("accepted_by/accepted_at 必须留空：%q/%q", rec.License.AcceptedBy, rec.License.AcceptedAt)
	}
}

// 生成的 Record 形态正确；许可留痕按新规则放行（unknown 允许空，待修补 #21）。
func TestProbeGGUFRecordExposesLicenseTraceConflict(t *testing.T) {
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
	assertLicenseTracePolicy(t, rec)

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

// 端点探测（无本地文件）：记录形态完整，许可留痕按新规则放行。
func TestProbeEndpointRecordExposesLicenseTraceConflict(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	rec, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("端点探测不应报错：%v", err)
	}
	if len(rec.Files) != 1 || rec.Files[0].Role != "endpoint" {
		t.Fatalf("端点探测无本地文件时应写一条占位建材料：%+v", rec.Files)
	}
	assertLicenseTracePolicy(t, rec)
	// 能力实测**不进记录正文**（本批核心）：正文只装身份，能力+证据写 <version>.capabilities.json。
	if len(rec.Capabilities) != 0 {
		t.Fatalf("能力断言不进记录正文（应落能力快照），实际：%+v", rec.Capabilities)
	}
	if hasCap(rec.Capabilities, "text") || hasCap(rec.Capabilities, "vision") {
		t.Fatalf("正文里不该有任何能力断言：%+v", rec.Capabilities)
	}
	// 但实测结论必须仍在（搬到报告 → 快照），判据不放宽：假端点 400 → vision=false，text/tools=true
	if v := capByName(rep.Capabilities, "vision"); v != nil && v.Value {
		t.Fatal("假端点对 image 返回 400，vision 必须为 false")
	}
	if !hasCap(rep.Capabilities, "text") || !hasCap(rep.Capabilities, "tools") {
		t.Fatalf("text/tools 应为 true：%+v", rep.Capabilities)
	}
	// 快照照旧给出能力断言 + 证据（给人看：--json / 兄弟文件）
	if v := capByName(NewCapabilitySnapshot(rec, rep).Capabilities, "vision"); v == nil || v.Value || v.Evidence == "" {
		t.Fatalf("能力快照里应看到带证据的 vision=false：%+v", rep.Capabilities)
	}
	// 待修补 #24：探测留痕不再进正文，改由报告（--json / 兄弟文件）承载。
	if strings.Contains(rec.Notes, "probe_trace") || dateRE.MatchString(rec.Notes) {
		t.Fatalf("notes 必须随内容确定：不得含 probe_trace 或时间戳：%s", rec.Notes)
	}
	if len(rep.Traces) == 0 || !hasTrace(rep.Traces, EvidenceVision) {
		t.Fatalf("应产出探测留痕（含视觉）并挂在报告上：%+v", rep.Traces)
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

// ── 待修补 #24：记录正文必须随内容确定 ──────────────────────────────────────

// 确定性（反例优先）：同一输入 probe 两次，记录正文逐字节相同。
// 生成时间戳、探测耗时（ms=）、probe_trace 一律不得进正文——否则 --store 第二遍
// 会命中 Store.Put 的防覆盖保护（ConflictError / exit 3）。
// 两次探测之间故意隔开 >1s：若正文里还残留秒级时间戳，本用例必然红。
func TestProbeRecordBodyIsDeterministic(t *testing.T) {
	cases := []struct {
		name   string
		target func(t *testing.T) string
	}{
		{
			name: "本地GGUF（留痕带耗时）",
			target: func(t *testing.T) string {
				return writeTestGGUF(t, "", "DetModel-Q4_K_M.gguf")
			},
		},
		{
			name: "端点（在线探测器 + ms 波动）",
			target: func(t *testing.T) string {
				return fakeEngine(t, "image input is not supported by this model").URL
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target(t)
			r1, _, err := Probe(ProbeOptions{Target: target, Timeout: 5 * time.Second})
			if err != nil {
				t.Fatalf("第一次探测失败：%v", err)
			}
			time.Sleep(1100 * time.Millisecond) // 跨过秒边界，暴露任何残留的时间戳
			r2, _, err := Probe(ProbeOptions{Target: target, Timeout: 5 * time.Second})
			if err != nil {
				t.Fatalf("第二次探测失败：%v", err)
			}

			b1, b2 := recordBody(t, r1), recordBody(t, r2)
			t.Logf("第一次正文 %s", bodyHash(b1))
			t.Logf("第二次正文 %s", bodyHash(b2))
			if bodyHash(b1) != bodyHash(b2) {
				t.Fatalf("同一输入两次 probe 的记录正文必须逐字节相同（待修补 #24）\n  1=%s\n  2=%s", bodyHash(b1), bodyHash(b2))
			}
			// 正文里不许出现留痕/耗时/生成时间戳
			if strings.Contains(string(b1), "probe_trace") {
				t.Fatalf("记录正文不得内嵌 probe_trace（应移出为兄弟文件）：%s", b1)
			}
		})
	}
}

// notes 必须是确定的文本：不得含生成时间戳或探测耗时（待修补 #24）。
func TestProbeNotesHasNoVolatileValues(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	rec, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec.Notes, "probe_trace") {
		t.Fatalf("notes 不得含 probe_trace：%s", rec.Notes)
	}
	if strings.Contains(rec.Notes, "ms=") {
		t.Fatalf("notes 不得含探测耗时 ms=：%s", rec.Notes)
	}
	if dateRE.MatchString(rec.Notes) {
		t.Fatalf("notes 不得含 RFC3339 时间戳：%s", rec.Notes)
	}
	// 但留痕本身必须仍在（只是搬到了报告里，供 --json 与兄弟文件用）
	if len(rep.Traces) == 0 {
		t.Fatal("探测留痕必须仍产出（从正文移到报告/兄弟文件）")
	}
	if !hasTrace(rep.Traces, EvidenceVision) {
		t.Fatalf("报告里应含视觉留痕：%+v", rep.Traces)
	}
}

// 留痕兄弟文件：可写、可解析、承载易变信息（生成时间 + 耗时）；且正文里没有它。
func TestProbeTraceSiblingFile(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	rec, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	recPath := filepath.Join(t.TempDir(), rec.ID, VersionOf(rec)+".json")
	sibPath, err := WriteProbeTraceSibling(recPath, rec, rep)
	if err != nil {
		t.Fatalf("写留痕兄弟文件失败：%v", err)
	}
	if want := strings.TrimSuffix(recPath, ".json") + ".trace.json"; sibPath != want {
		t.Fatalf("兄弟文件路径不对：want=%s got=%s", want, sibPath)
	}
	b, err := os.ReadFile(sibPath)
	if err != nil {
		t.Fatalf("兄弟文件没落盘：%v", err)
	}
	var art ProbeTraceArtifact
	if err := json.Unmarshal(b, &art); err != nil {
		t.Fatalf("兄弟文件不是合法 JSON：%v", err)
	}
	if art.Schema != TraceSchemaV1 {
		t.Fatalf("兄弟文件 schema 不对：%s", art.Schema)
	}
	if len(art.Traces) != len(rep.Traces) {
		t.Fatalf("留痕条数不对：want=%d got=%d", len(rep.Traces), len(art.Traces))
	}
	if art.GeneratedAt == "" {
		t.Fatal("留痕兄弟文件应承载生成时间（它是易变信息，正该放这里）")
	}
	// 正文（记录）里不许有留痕
	if strings.Contains(string(recordBody(t, rec)), "probe_trace") {
		t.Fatal("记录正文不得出现 probe_trace")
	}
}

func hasTrace(ts []Trace, name string) bool {
	for _, tr := range ts {
		if tr.Probe == name {
			return true
		}
	}
	return false
}

// capByName 取出某条能力断言（找不到返回 nil）。
func capByName(caps []Capability, name string) *Capability {
	for i := range caps {
		if caps[i].Name == name {
			return &caps[i]
		}
	}
	return nil
}

// ── 能力快照落兄弟文件（本批核心）────────────────────────────────────────────

// 本批核心断言：**同一建材、不同端点两次探测 → 记录正文逐字节相同**。
// 能力实测（含 vision/tools）不再进正文，改由 <version>.capabilities.json 快照承载；
// 于是"先 probe --store（无端点，登记身份）、后 probe --endpoint --store（补能力实测）"
// 这条自然流程不再撞上 Store.Put 的防覆盖保护（ConflictError）。
func TestProbeRecordBodyIgnoresEndpoint(t *testing.T) {
	gguf := writeTestGGUF(t, "", "SameBrickwork-Q4_K_M.gguf")

	recA, repA, err := Probe(ProbeOptions{Target: gguf, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("无端点探测失败：%v", err)
	}
	srvA := fakeEngine(t, "image input is not supported by this model")
	recB, repB, err := Probe(ProbeOptions{Target: gguf, Endpoint: srvA.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("带端点探测失败：%v", err)
	}

	// 前提校验：两次探测确实"能力上不同"——否则本用例证明不了什么
	if repA.OnlineProbed || len(repA.Capabilities) != 0 {
		t.Fatalf("本用例前提是无端点那次没有在线能力，实际 %+v", repA.Capabilities)
	}
	if !repB.OnlineProbed || len(repB.Capabilities) == 0 {
		t.Fatalf("本用例前提是带端点那次真在线探到了能力，实际 %+v", repB.Capabilities)
	}
	if recA.Digest != recB.Digest {
		t.Fatalf("同一建材 digest 必须相同：%s vs %s", recA.Digest, recB.Digest)
	}

	bA, bB := recordBody(t, recA), recordBody(t, recB)
	t.Logf("无端点正文 %s", bodyHash(bA))
	t.Logf("带端点正文 %s", bodyHash(bB))
	if bodyHash(bA) != bodyHash(bB) {
		t.Fatalf("同一建材、不同端点两次探测，记录正文必须逐字节相同（正文只装身份）\n  无端点=%s\n  带端点=%s",
			bodyHash(bA), bodyHash(bB))
	}
	// 反例守护：正文里不得出现能力断言块（出现就又随端点变化了）
	if strings.Contains(string(bB), "capabilities") {
		t.Fatalf("记录正文不得内嵌 capabilities（能力应在快照里）：%s", bB)
	}
	if len(recB.Capabilities) != 0 {
		t.Fatalf("probe 不该往正文写能力断言：%+v", recB.Capabilities)
	}

	// 能力本身必须仍在，且判据不放宽：假端点对 image 返回 400 → vision 必须 false 且带证据
	vis := capByName(repB.Capabilities, "vision")
	if vis == nil || vis.Value || vis.Source != "probed" || !strings.Contains(vis.Evidence, EvidenceVision) {
		t.Fatalf("vision 断言应为 probed/false 且带探测器名：%+v", repB.Capabilities)
	}
	if !hasCap(repB.Capabilities, "text") || !hasCap(repB.Capabilities, "tools") {
		t.Fatalf("text/tools 应为 true：%+v", repB.Capabilities)
	}

	// 能力 + 证据写进快照（交给兄弟文件），而不是正文
	snap := NewCapabilitySnapshot(recB, repB)
	if snap.Schema != CapabilitySnapshotSchemaV1 || snap.Digest != recB.Digest || snap.Endpoint != srvA.URL {
		t.Fatalf("快照身份字段不对：%+v", snap)
	}
	if len(snap.Capabilities) != len(repB.Capabilities) {
		t.Fatalf("快照应完整承载能力断言：want=%d got=%d", len(repB.Capabilities), len(snap.Capabilities))
	}
}

// 能力快照兄弟文件：路径命名、可写、可解析、承载证据与端点；
// 且**可刷新**（第二遍写不同的快照不报错、不改动记录正文）。
func TestCapabilitySnapshotSiblingIsRefreshable(t *testing.T) {
	gguf := writeTestGGUF(t, "", "SnapModel-Q4_K_M.gguf")
	srv := fakeEngine(t, "image input is not supported by this model")
	rec, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	recPath := filepath.Join(dir, rec.ID, VersionOf(rec)+".json")
	if _, err := WriteFileAtomic(recPath, recordBody(t, rec)); err != nil {
		t.Fatal(err)
	}
	recBefore, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}

	sib, err := WriteCapabilitySnapshot(recPath, rec, rep)
	if err != nil {
		t.Fatalf("写能力快照失败：%v", err)
	}
	if want := strings.TrimSuffix(recPath, ".json") + ".capabilities.json"; sib != want {
		t.Fatalf("快照路径不对：want=%s got=%s", want, sib)
	}
	b, err := os.ReadFile(sib)
	if err != nil {
		t.Fatalf("快照没落盘：%v", err)
	}
	var art CapabilitySnapshotArtifact
	if err := json.Unmarshal(b, &art); err != nil {
		t.Fatalf("快照不是合法 JSON：%v", err)
	}
	if art.Schema != CapabilitySnapshotSchemaV1 || art.ID != rec.ID || art.Digest != rec.Digest {
		t.Fatalf("快照身份字段不对：%+v", art)
	}
	if art.Endpoint != srv.URL {
		t.Fatalf("快照应记下用的端点：%q", art.Endpoint)
	}
	if art.GeneratedAt == "" {
		t.Fatal("快照应承载生成时间（它是易变信息，正该放这里）")
	}
	if v := capByName(art.Capabilities, "vision"); v == nil || !strings.Contains(v.Evidence, EvidenceVision) {
		t.Fatalf("快照里应能看到带证据的 vision 断言：%+v", art.Capabilities)
	}

	// 可刷新：换一份"能力不同、端点不同"的快照重写 → 不报错、内容更新、记录正文一字未动
	rep2 := *rep
	rep2.Endpoint = "http://127.0.0.1:1"
	rep2.Capabilities = []Capability{{Name: "vision", Value: true, Source: "probed", Evidence: EvidenceVision}}
	if _, err := WriteCapabilitySnapshot(recPath, rec, &rep2); err != nil {
		t.Fatalf("快照必须可刷新（不报错）：%v", err)
	}
	b2, err := os.ReadFile(sib)
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) == string(b) {
		t.Fatal("第二遍快照应写下新内容（刷新语义）")
	}
	var art2 CapabilitySnapshotArtifact
	if err := json.Unmarshal(b2, &art2); err != nil {
		t.Fatal(err)
	}
	if v := capByName(art2.Capabilities, "vision"); v == nil || !v.Value {
		t.Fatalf("刷新后应看到新的能力值：%+v", art2.Capabilities)
	}
	if art2.Endpoint != rep2.Endpoint {
		t.Fatalf("刷新后端点应更新：%q", art2.Endpoint)
	}
	recAfter, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(recBefore) != string(recAfter) {
		t.Fatal("刷新能力快照不得改动记录正文（防覆盖保护不受影响）")
	}
	if _, err := os.Stat(TraceSiblingPath(recPath)); !os.IsNotExist(err) {
		t.Fatalf("本用例不该产生留痕兄弟文件：%v", err)
	}
}

// --store 幂等的根（待修补 #24 的核心验收）：同一模型 probe 两次 → 正文逐字节相同 →
// Store.Put 第二遍 changed=false、无 ConflictError、文件不被重写；留痕写兄弟文件、可解析。
func TestProbeStoreIsIdempotent(t *testing.T) {
	srv := fakeEngine(t, "image input is not supported by this model")
	root := t.TempDir()
	st := NewStore(root)

	rec1, rep1, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.Put(rec1, false)
	if err != nil {
		t.Fatalf("首次入目录失败：%v", err)
	}
	if !first.Changed {
		t.Fatal("首次应 changed=true")
	}
	// CLI 在记录入目录成功后写留痕兄弟文件；这里复刻同一步（Store.Put 不碰留痕）。
	sib, err := WriteProbeTraceSibling(first.Path, rec1, rep1)
	if err != nil {
		t.Fatalf("写留痕兄弟文件失败：%v", err)
	}
	b1, _ := os.ReadFile(first.Path)
	fi1, _ := os.Stat(first.Path)

	time.Sleep(1100 * time.Millisecond) // 跨秒边界：正文若含时间戳，这里必然 changed=true/Conflict
	rec2, _, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.Put(rec2, false)
	var conf *ConflictError
	if errors.As(err, &conf) {
		t.Fatalf("重复 probe 不得撞上防覆盖保护（ConflictError）：%v", err)
	}
	if err != nil {
		t.Fatalf("第二次入目录失败：%v", err)
	}
	if second.Changed {
		t.Fatal("第二次应 changed=false（正文一致，不重写）")
	}
	b2, _ := os.ReadFile(first.Path)
	if string(b1) != string(b2) {
		t.Fatalf("第二次不该改写记录文件：\n  1=%s\n  2=%s", b1, b2)
	}
	fi2, _ := os.Stat(first.Path)
	if !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Fatalf("记录文件被重写（mtime 变了）：%s → %s", fi1.ModTime(), fi2.ModTime())
	}
	// 兄弟留痕存在、可解析、对应同一条记录
	tb, err := os.ReadFile(sib)
	if err != nil {
		t.Fatalf("兄弟留痕没落盘：%v", err)
	}
	var art ProbeTraceArtifact
	if err := json.Unmarshal(tb, &art); err != nil {
		t.Fatalf("兄弟留痕不是合法 JSON：%v", err)
	}
	if art.Digest != rec1.Digest || art.ID != rec1.ID {
		t.Fatalf("兄弟留痕应对应同一条记录：%+v vs id=%s digest=%s", art, rec1.ID, rec1.Digest)
	}
}

// ── 待修补 #27：预算"先小后大"重试 + 区分"预算不足"与"真不支持" ──────────────

// findUnverifiable 取某条不可判定记录（找不到返回 nil）。
func findUnverifiable(xs []Unverifiable, name string) *Unverifiable {
	for i := range xs {
		if xs[i].Name == name {
			return &xs[i]
		}
	}
	return nil
}

// 反例 1：思考模型小预算把预算吃光（finish_reason=length、content 空）→
// 探测器必须自动抬高预算重试；拿到可用回答后正确判 true，且 evidence 里看得到实际预算。
func TestProbeTextRaisesBudgetOnLengthTruncation(t *testing.T) {
	var mu sync.Mutex
	var seen []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		seen = append(seen, req.MaxTokens)
		mu.Unlock()
		// 小预算：思考把预算吃光 → 撞长度且正文为空
		if req.MaxTokens < 512 {
			fmt.Fprint(w, `{"choices":[{"finish_reason":"length","index":0,"message":{"role":"assistant","content":"","reasoning_content":"让我想想……"}}]}`)
			return
		}
		// 大预算：正常回答
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"你好！"}}]}`)
	}))
	t.Cleanup(srv.Close)

	ep := Endpoint{BaseURL: srv.URL, Model: "thinking-fake", Timeout: 5 * time.Second}
	r := ProbeText(ep)
	if !r.Value || r.Undetermined {
		t.Fatalf("抬高预算拿到可用回答后应判 true，实际 %+v", r)
	}
	if r.Trace.Budget != 512 {
		t.Fatalf("实际预算应为抬到的那一档 512，实际 %d", r.Trace.Budget)
	}
	mu.Lock()
	got := append([]int(nil), seen...)
	mu.Unlock()
	if len(got) < 2 || got[0] != 16 || got[1] != 512 {
		t.Fatalf("必须先小后大重试（16→512），实际请求序列 %v", got)
	}
	c := capabilityOf("text", EvidenceText, r)
	if !strings.Contains(c.Evidence, "budget=512") {
		t.Fatalf("evidence 必须能看出实际预算（budget=512）：%s", c.Evidence)
	}
	if !c.Value || c.Source != "probed" {
		t.Fatalf("能力断言应为 probed/true：%+v", c)
	}
}

// fakeAlwaysTruncated 是"无论多大预算都撞长度"的假端点：模拟思考模型把预算全花在思考上。
func fakeAlwaysTruncated(t *testing.T, reasoning string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"object":"list","data":[{"id":"thinking-fake","object":"model"}]}`)
		case "/v1/chat/completions":
			fmt.Fprintf(w, `{"choices":[{"finish_reason":"length","index":0,"message":{"role":"assistant","content":"","reasoning_content":%q}}]}`, reasoning)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 反例 2：无论多大预算都撞长度 → **不得**生成 value=false 条目，改写入 unverifiable。
// 同时守住区分点：端点明确没有的能力（embeddings/rerank 404）仍照旧判 false。
func TestProbeBudgetExhaustedIsUnverifiableNotFalse(t *testing.T) {
	srv := fakeAlwaysTruncated(t, "让我仔细想想这个问题……")
	rec, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("预算不足不是异常，探测不该报错：%v", err)
	}
	// ① 不得生成 false 能力条目（false = 确定没有；这里是"没探够"）
	for _, name := range []string{"text", "vision", "tools"} {
		if c := capByName(rep.Capabilities, name); c != nil {
			t.Fatalf("%s 因预算不足不可判，绝不许生成 value=false 条目：%+v", name, *c)
		}
	}
	// ② 必须写入不可判定记录，带原因 + 抬到上限的预算证据
	for _, name := range []string{"text", "vision", "tools"} {
		uv := findUnverifiable(rep.Unverifiable, name)
		if uv == nil {
			t.Fatalf("%s 应写入 unverifiable（缺 = 未知，不等于没有）：%+v", name, rep.Unverifiable)
		}
		if uv.Reason != FailBudgetExhausted {
			t.Fatalf("%s 不可判定原因应为 %s，实际 %q", name, FailBudgetExhausted, uv.Reason)
		}
		if !strings.Contains(uv.Evidence, "budget=2048") {
			t.Fatalf("%s 的不可判定证据应能看出抬到了预算上限：%s", name, uv.Evidence)
		}
	}
	// ③ 区分点：端点明确说没有的能力仍照旧 false，且不进 unverifiable
	if c := capByName(rep.Capabilities, "embedding"); c == nil || c.Value {
		t.Fatalf("embedding：端点 404 明确不支持，应照旧 value=false：%+v", c)
	}
	if findUnverifiable(rep.Unverifiable, "embedding") != nil {
		t.Fatalf("embedding 是确定不支持，不该被记成不可判定：%+v", rep.Unverifiable)
	}
	// ④ 能力快照必须承载不可判定记录（写盘与 --json 共用同一份）
	snap := NewCapabilitySnapshot(rec, rep)
	if findUnverifiable(snap.Unverifiable, "text") == nil || findUnverifiable(snap.Unverifiable, "tools") == nil {
		t.Fatalf("能力快照应承载 unverifiable：%+v", snap.Unverifiable)
	}
	if capByName(snap.Capabilities, "text") != nil {
		t.Fatalf("快照里也不该有 text=false：%+v", snap.Capabilities)
	}
	// ⑤ 分层不变：记录正文仍不含能力断言
	if len(rec.Capabilities) != 0 {
		t.Fatalf("正文仍不得内嵌能力：%+v", rec.Capabilities)
	}
	// 快照仍向后兼容：schema 不变、身份字段照旧
	if snap.Schema != CapabilitySnapshotSchemaV1 || snap.ID != rec.ID || snap.Digest != rec.Digest {
		t.Fatalf("快照身份字段不该变：%+v", snap)
	}
}

// fakeExplicitlyUnsupported 是"端点明确表态不支持"的假端点：视觉 400 + mmproj 提示、工具 400。
func fakeExplicitlyUnsupported(t *testing.T) *httptest.Server {
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
			case strings.Contains(bs, "image_url"):
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"this model does not support images: mmproj file not loaded","type":"invalid_request_error"}}`)
			case strings.Contains(bs, `"tools"`):
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"tools are not supported by this endpoint","type":"invalid_request_error"}}`)
			default:
				fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"你好！"}}]}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 反例 3（核心区分点）：端点"明确说不行"（4xx / 无 mmproj）→ 仍照旧 value=false + evidence，
// **绝不**被记成不可判定。与反例 2 形成对照。
func TestProbeExplicitlyUnsupportedStaysFalse(t *testing.T) {
	srv := fakeExplicitlyUnsupported(t)
	_, rep, err := Probe(ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// text：端点正常回话 → true
	if c := capByName(rep.Capabilities, "text"); c == nil || !c.Value {
		t.Fatalf("text 应为 true：%+v", c)
	}
	// tools：端点 HTTP 400 明确拒绝 → 仍是 value=false + evidence
	tc := capByName(rep.Capabilities, "tools")
	if tc == nil || tc.Value {
		t.Fatalf("tools：端点明确不支持，应照旧 value=false：%+v", tc)
	}
	if !strings.Contains(tc.Evidence, FailNoTools) {
		t.Fatalf("tools evidence 应带失败分类 %s：%s", FailNoTools, tc.Evidence)
	}
	// vision：400（引擎对图像请求的明确拒绝）→ 仍照旧 value=false + evidence。
	// 待修补 #23：响应体虽然写着"mmproj file not loaded"，但**不得**据此升级成
	// mmproj_missing——本端点没有结构化能力声明（无 /props、无 capabilities），
	// 复现不了"引擎声明无视觉"这条结构化证据，故归因停在 no_vision。
	vc := capByName(rep.Capabilities, "vision")
	if vc == nil || vc.Value {
		t.Fatalf("vision：端点明确不支持，应 value=false：%+v", vc)
	}
	if !strings.Contains(vc.Evidence, FailNoVision) {
		t.Fatalf("vision evidence 应带失败分类 %s：%s", FailNoVision, vc.Evidence)
	}
	if strings.Contains(vc.Evidence, FailMmprojMiss) {
		t.Fatalf("无结构化信号时绝不许按响应体文案升级成 mmproj_missing（待修补 #23）：%s", vc.Evidence)
	}
	// 本场景里每一条**能力**都是端点的确定态度 → 能力维度不该有任何不可判定记录。
	// 注：待修补 #22 起 unverifiable[] 还承载正交的 template 轴（模板来源确实没有可复现
	// 证据时如实记 no_template_source）——它不是能力标签，故单列一条属预期，不算能力误判。
	var capUV []Unverifiable
	for _, uv := range rep.Unverifiable {
		if uv.Name != CapabilityTemplate {
			capUV = append(capUV, uv)
		}
	}
	if len(capUV) != 0 {
		t.Fatalf("端点明确表态的场景不该有任何能力被判为不可判定：%+v", rep.Unverifiable)
	}
	if findUnverifiable(rep.Unverifiable, CapabilityTemplate) == nil {
		t.Fatalf("本场景端点没有只读模板元信息，应如实记 template 不可判定（#22）：%+v", rep.Unverifiable)
	}
}

// ── 待修补 #22：probe.template.v1 只认真证据（GGUF 键 / 端点只读元信息）────────────
//
// 旧实现按"端点能回话"推断来源（vllm→from_tokenizer、其余→from_gguf），是"推断冒充实测"。
// 新实现只有两条真路径；两条都不成立就**不给默认值**，改记 unverifiable（缺=未知）。

// findTrace 取某条留痕（找不到返回 nil）。
func findTrace(ts []Trace, probe string) *Trace {
	for i := range ts {
		if ts[i].Probe == probe {
			return &ts[i]
		}
	}
	return nil
}

// writeTestGGUFNoTemplate 造一个**没有** tokenizer.chat_template 键的 GGUF（#22 反例必备）。
func writeTestGGUFNoTemplate(t *testing.T, dir, name string) string {
	t.Helper()
	gguf := makeGGUF(
		kv("general.architecture", ggufTypeString, gstr("llama")),
		kv("general.name", ggufTypeString, gstr("NoTemplate-8B")),
		kv("llama.context_length", ggufTypeUint32, le32(4096)),
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

// fakePropsEngine 是带只读 /props 的假 llama.cpp 端点：/props 按给定状态码返回 propsBody。
func fakePropsEngine(t *testing.T, propsBody string, propsStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/props":
			w.WriteHeader(propsStatus)
			fmt.Fprint(w, propsBody)
		case "/v1/models":
			fmt.Fprint(w, `{"object":"list","data":[{"id":"fake-llama","object":"model"}]}`)
		case "/v1/chat/completions":
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeModelsTemplateEngine 是只在 /v1/models 里带模板字段的假端点（/props 不存在）。
func fakeModelsTemplateEngine(t *testing.T, template string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprintf(w, `{"object":"list","data":[{"id":"fake-llama","object":"model","chat_template":%q}]}`, template)
		case "/v1/chat/completions":
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// #22 反例优先 ①：本地 GGUF 确有 tokenizer.chat_template 键 → from_gguf，证据写键名。
func TestProbeTemplateFromGGUFEvidence(t *testing.T) {
	p := writeTestGGUF(t, "", "HasTemplate-Q4_K_M.gguf") // 含 tokenizer.chat_template
	rec, rep, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ChatTemplate != ChatTemplateFromGGUF {
		t.Fatalf("GGUF 有 tokenizer.chat_template → 应为 %s，实际 %q", ChatTemplateFromGGUF, rep.ChatTemplate)
	}
	if got := rec.EngineRecipes["llama.cpp"].ChatTemplate; got != ChatTemplateFromGGUF {
		t.Fatalf("engine_recipes.chat_template 应为 from_gguf，实际 %q", got)
	}
	tr := findTrace(rep.Traces, EvidenceTemplate)
	if tr == nil || !tr.OK {
		t.Fatalf("probe.template.v1 留痕应成功：%+v", tr)
	}
	if !strings.Contains(tr.Summary, "gguf_key: "+TemplateGGUFKey) {
		t.Fatalf("evidence 必须写清 GGUF 键名：%s", tr.Summary)
	}
	if findUnverifiable(rep.Unverifiable, CapabilityTemplate) != nil {
		t.Fatalf("有真证据时不该有 template 不可判定记录：%+v", rep.Unverifiable)
	}
}

// #22 收口 ②：GGUF 无模板、假端点 /props 返回 chat_template → **报告层**记 from_tokenizer
// （证据写端点字段名）；但**记录正文不写 chat_template**（正文只装身份，端点模板是"现状"），
// 它改由能力快照的 endpoint_chat_template 承载。
func TestProbeTemplateFromEndpointProps(t *testing.T) {
	gguf := writeTestGGUFNoTemplate(t, "", "NoTemplate-Q4_K_M.gguf")
	srv := fakePropsEngine(t, `{"chat_template":"{{ .Prompt }}<|im_end|>","n_ctx":4096}`, http.StatusOK)
	rec, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// 报告层：真探测到端点模板（#22 的两条真路径之一，成果保留）
	if rep.ChatTemplate != ChatTemplateFromTokenizer {
		t.Fatalf("/props 有 chat_template → 报告层应为 %s，实际 %q", ChatTemplateFromTokenizer, rep.ChatTemplate)
	}
	// 正文层（收口 #22）：**绝不**把端点模板写进 engine_recipes.chat_template
	if got := rec.EngineRecipes["llama.cpp"].ChatTemplate; got != "" {
		t.Fatalf("端点模板不得进记录正文的 engine_recipes.chat_template，实际 %q（%+v）", got, rec.EngineRecipes)
	}
	if strings.Contains(string(recordBody(t, rec)), `"chat_template"`) {
		t.Fatalf("记录正文不得出现 chat_template（端点模板属现状）：%s", recordBody(t, rec))
	}
	tr := findTrace(rep.Traces, EvidenceTemplate)
	if tr == nil || !tr.OK || !strings.Contains(tr.Summary, "/props.chat_template") {
		t.Fatalf("evidence 必须写清端点字段名 /props.chat_template：%+v", tr)
	}
	if findUnverifiable(rep.Unverifiable, CapabilityTemplate) != nil {
		t.Fatalf("拿到真证据时不该有 template 不可判定记录：%+v", rep.Unverifiable)
	}
	// 快照层：端点模板必须落在 endpoint_chat_template（信息没丢，只是换了地方）
	snap := NewCapabilitySnapshot(rec, rep)
	if snap.EndpointChatTemplate == nil || snap.EndpointChatTemplate.Value != ChatTemplateFromTokenizer {
		t.Fatalf("能力快照应承载 endpoint_chat_template=from_tokenizer：%+v", snap.EndpointChatTemplate)
	}
	if !strings.Contains(snap.EndpointChatTemplate.Evidence, "/props.chat_template") ||
		!strings.Contains(snap.EndpointChatTemplate.Evidence, srv.URL) {
		t.Fatalf("endpoint_chat_template.evidence 应写清端点 + 字段名：%+v", snap.EndpointChatTemplate)
	}
}

// #22 反例优先 ②b：/props 不可用，退一步 /v1/models 的模板字段 → from_tokenizer（证据锚写 models 字段）。
func TestProbeTemplateFromModelsFallback(t *testing.T) {
	gguf := writeTestGGUFNoTemplate(t, "", "NoTemplate2-Q4_K_M.gguf")
	srv := fakeModelsTemplateEngine(t, "{{ .Prompt }}")
	_, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ChatTemplate != ChatTemplateFromTokenizer {
		t.Fatalf("/v1/models 带模板字段 → 应为 %s，实际 %q", ChatTemplateFromTokenizer, rep.ChatTemplate)
	}
	tr := findTrace(rep.Traces, EvidenceTemplate)
	if tr == nil || !tr.OK || !strings.Contains(tr.Summary, "/v1/models.data[0].chat_template") {
		t.Fatalf("退一步命中的证据锚应写 /v1/models.data[0].chat_template：%+v", tr)
	}
}

// #22 反例优先 ③：两边都没有 → **不生成条目**（无 engine_recipes.chat_template），
// 且 unverifiable 里出现 template（reason=no_template_source）。旧实现的默认值必须消失。
func TestProbeTemplateNoSourceIsUnverifiable(t *testing.T) {
	gguf := writeTestGGUFNoTemplate(t, "", "NoTemplate3-Q4_K_M.gguf")
	srv := fakeEngine(t, "image input is not supported by this model") // 无 /props；/v1/models 无模板字段

	cases := []struct {
		name string
		opts ProbeOptions
	}{
		{"只有本地文件", ProbeOptions{Target: gguf}},
		{"文件+端点但端点无模板", ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second}},
		{"只有端点且端点无模板", ProbeOptions{Target: srv.URL, Timeout: 5 * time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, rep, err := Probe(tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if rep.ChatTemplate != "" {
				t.Fatalf("两条真路径都不成立时不得给默认值，实际 %q", rep.ChatTemplate)
			}
			if got := rec.EngineRecipes["llama.cpp"].ChatTemplate; got != "" {
				t.Fatalf("不得生成 engine_recipes.chat_template，实际 %q（%+v）", got, rec.EngineRecipes)
			}
			uv := findUnverifiable(rep.Unverifiable, CapabilityTemplate)
			if uv == nil {
				t.Fatalf("应写入 unverifiable[template]（缺=未知）：%+v", rep.Unverifiable)
			}
			if uv.Reason != FailNoTemplateSource {
				t.Fatalf("原因分类应为 %s，实际 %q", FailNoTemplateSource, uv.Reason)
			}
			if !strings.Contains(uv.Evidence, EvidenceTemplate) {
				t.Fatalf("证据应带探测器名：%s", uv.Evidence)
			}
			tr := findTrace(rep.Traces, EvidenceTemplate)
			if tr == nil || tr.OK {
				t.Fatalf("probe.template.v1 留痕应如实记未取到：%+v", tr)
			}
			if tr.FailureClass != FailNoTemplateSource {
				t.Fatalf("留痕失败分类应为 %s，实际 %q", FailNoTemplateSource, tr.FailureClass)
			}
			// 快照必须承载该不可判定记录（写盘与 --json 共用同一份）
			snap := NewCapabilitySnapshot(rec, rep)
			if findUnverifiable(snap.Unverifiable, CapabilityTemplate) == nil {
				t.Fatalf("能力快照应承载 unverifiable：%+v", snap.Unverifiable)
			}
			// 反例守护：绝不回退到旧的默认值
			if rep.ChatTemplate == ChatTemplateFromGGUF || rep.ChatTemplate == ChatTemplateFromTokenizer {
				t.Fatalf("无证据却给了来源 %q（等于推断冒充实测）：%+v", rep.ChatTemplate, rep.Traces)
			}
		})
	}
}

// #22 反例：/props 有 chat_template 键但为空 / 非 200 / 非 JSON → 都不算证据（缺=未知）。
func TestProbeTemplatePropsEmptyOrBadIsNotEvidence(t *testing.T) {
	gguf := writeTestGGUFNoTemplate(t, "", "NoTemplate4-Q4_K_M.gguf")
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"空模板", `{"chat_template":""}`, http.StatusOK},
		{"只有空白", `{"chat_template":"   "}`, http.StatusOK},
		{"无 chat_template 字段", `{"n_ctx":4096}`, http.StatusOK},
		{"非 200", `{"error":"boom"}`, http.StatusInternalServerError},
		{"非 JSON 正文", `<html>nope</html>`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakePropsEngine(t, tc.body, tc.status)
			_, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if rep.ChatTemplate != "" {
				t.Fatalf("没有真证据不得给结论，实际 %q", rep.ChatTemplate)
			}
			if findUnverifiable(rep.Unverifiable, CapabilityTemplate) == nil {
				t.Fatalf("应记 unverifiable[template]：%+v", rep.Unverifiable)
			}
		})
	}
}

// #22 取舍钉子：GGUF 的模板与端点 /props 的模板不一致 → 取 GGUF（本地建材权威）。
func TestProbeTemplateGGUFWinsOverEndpoint(t *testing.T) {
	gguf := writeTestGGUF(t, "", "Conflict-Q4_K_M.gguf") // 模板 "{{ .Prompt }}"
	srv := fakePropsEngine(t, `{"chat_template":"SOME-OTHER-ENDPOINT-TEMPLATE"}`, http.StatusOK)

	// 前提校验：端点确实返回了一个**不同**的模板（否则本用例证明不了取舍）
	epTpl := ProbeChatTemplateFromEndpoint(Endpoint{BaseURL: srv.URL, Timeout: 5 * time.Second})
	if !epTpl.OK || epTpl.Anchor != "/props.chat_template" {
		t.Fatalf("前提：端点 /props 应返回模板，实际 %+v", epTpl)
	}
	if epTpl.Template == "{{ .Prompt }}" {
		t.Fatal("前提：端点模板应与 GGUF 里的不同")
	}

	rec, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ChatTemplate != ChatTemplateFromGGUF {
		t.Fatalf("GGUF 与端点都有模板时应取 GGUF（建材权威），实际 %q", rep.ChatTemplate)
	}
	if got := rec.EngineRecipes["llama.cpp"].ChatTemplate; got != ChatTemplateFromGGUF {
		t.Fatalf("engine_recipes 应取 from_gguf，实际 %q", got)
	}
	tr := findTrace(rep.Traces, EvidenceTemplate)
	if tr == nil || !strings.Contains(tr.Summary, "gguf_key: "+TemplateGGUFKey) {
		t.Fatalf("证据应指向 GGUF 键：%+v", tr)
	}
	// 同一建材、不同端点 → 正文仍逐字节相同（#24 确定性不被 #22 的修复削弱）
	noEP, _, err := Probe(ProbeOptions{Target: gguf})
	if err != nil {
		t.Fatal(err)
	}
	if bodyHash(recordBody(t, noEP)) != bodyHash(recordBody(t, rec)) {
		t.Fatalf("GGUF 有模板时，给不给端点正文必须一致（#24）：\n  无端点=%s\n  带端点=%s",
			bodyHash(recordBody(t, noEP)), bodyHash(recordBody(t, rec)))
	}
}

// ── 待修补 #22 收口：端点模板不进正文；同一批无模板建材给不给端点正文逐字节相同 ──────
//
// 本次收口的核心不变量（承 #24）：记录正文里的 engine_recipes.chat_template **只允许来自
// GGUF**。造一个**没有** tokenizer.chat_template 的 GGUF：
//
//	A = 只给文件（无端点）；
//	B = 文件 + 假端点（该端点 /props 能返回 chat_template）。
//
// 断言 A 与 B 的正文逐字节相同（序列化后 sha256 相等），且两者都**不带**
// engine_recipes.chat_template；同时断言端点模板没丢——它出现在 B 的能力快照
// endpoint_chat_template 里（value=from_tokenizer，evidence 写清端点 + 字段名）。
func TestProbeEndpointTemplateNeverEntersRecordBody(t *testing.T) {
	gguf := writeTestGGUFNoTemplate(t, "", "NoTemplate-Body-Q4_K_M.gguf")

	// A：只给文件
	recA, repA, err := Probe(ProbeOptions{Target: gguf, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("无端点探测失败：%v", err)
	}
	// B：文件 + 假端点（/props 返回 chat_template）
	srv := fakePropsEngine(t, `{"chat_template":"{{ .Prompt }}<|im_end|>","n_ctx":4096}`, http.StatusOK)
	recB, repB, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("带端点探测失败：%v", err)
	}

	// 前提校验：带端点那次确实从端点只读元信息探到了模板，否则本用例证明不了什么
	if repB.ChatTemplate != ChatTemplateFromTokenizer {
		t.Fatalf("前提：/props 有 chat_template 时报告层应为 %s，实际 %q", ChatTemplateFromTokenizer, repB.ChatTemplate)
	}
	if repA.ChatTemplate != "" {
		t.Fatalf("前提：无端点那次报告层不该有模板来源，实际 %q", repA.ChatTemplate)
	}

	bA, bB := recordBody(t, recA), recordBody(t, recB)
	t.Logf("无端点正文 %s", bodyHash(bA))
	t.Logf("带端点正文 %s", bodyHash(bB))
	t.Logf("无端点正文原文：\n%s", bA)
	t.Logf("带端点正文原文：\n%s", bB)
	if bodyHash(bA) != bodyHash(bB) {
		t.Fatalf("同一批无模板建材、给不给端点，正文必须逐字节相同（#24 不变量）\n  无端点=%s\n  带端点=%s\n  无端点正文=%s\n  带端点正文=%s",
			bodyHash(bA), bodyHash(bB), bA, bB)
	}

	// 两份正文都**不**得带 engine_recipes.chat_template
	for _, c := range []struct {
		name string
		body []byte
		rec  *Record
	}{
		{"无端点", bA, recA},
		{"带端点", bB, recB},
	} {
		if strings.Contains(string(c.body), `"chat_template"`) {
			t.Fatalf("%s正文不得出现 chat_template（正文只装身份）：%s", c.name, c.body)
		}
		if got := c.rec.EngineRecipes["llama.cpp"].ChatTemplate; got != "" {
			t.Fatalf("%s的 engine_recipes.chat_template 必须为空，实际 %q", c.name, got)
		}
	}

	// 信息没丢：端点模板落在能力快照 endpoint_chat_template 里（from_tokenizer + 端点字段证据）
	snapB := NewCapabilitySnapshot(recB, repB)
	if snapB.EndpointChatTemplate == nil {
		t.Fatalf("带端点那次的能力快照应承载 endpoint_chat_template：%+v", snapB)
	}
	if snapB.EndpointChatTemplate.Value != ChatTemplateFromTokenizer {
		t.Fatalf("endpoint_chat_template.value 应为 %s，实际 %q",
			ChatTemplateFromTokenizer, snapB.EndpointChatTemplate.Value)
	}
	if !strings.Contains(snapB.EndpointChatTemplate.Evidence, "/props.chat_template") ||
		!strings.Contains(snapB.EndpointChatTemplate.Evidence, srv.URL) {
		t.Fatalf("endpoint_chat_template.evidence 应写清端点 + 字段名：%+v", snapB.EndpointChatTemplate)
	}
	// 快照序列化后确实带 endpoint_chat_template 键（omitempty 只在缺时省略）
	sb, err := json.Marshal(snapB)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("带端点快照 endpoint_chat_template=%+v", *snapB.EndpointChatTemplate)
	t.Logf("带端点快照原文：\n%s", sb)
	if !strings.Contains(string(sb), `"endpoint_chat_template"`) {
		t.Fatalf("快照序列化应含 endpoint_chat_template 键：%s", sb)
	}
	// 无端点那次的快照里没有该字段（缺 = 未知，omitempty）
	snapA := NewCapabilitySnapshot(recA, repA)
	if snapA.EndpointChatTemplate != nil {
		t.Fatalf("无端点那次不应有 endpoint_chat_template：%+v", snapA.EndpointChatTemplate)
	}
	sa, err := json.Marshal(snapA)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sa), `"endpoint_chat_template"`) {
		t.Fatalf("无端点快照不该出现 endpoint_chat_template 键：%s", sa)
	}
}

// ── 待修补 #23：mmproj_missing 只认结构化信号，绝不匹配响应体文案 ────────────────
//
// 旧实现：非 200 时若响应体里出现 "mmproj" 字样，就把 no_vision 升级成 mmproj_missing——
// 违反《开工方案-模型探测与校验》§八 风险1「只认结构化信号」，引擎文案一变就误判。
// 本机实测（llama.cpp build 10470 + Ornith-1.5-35B/mmproj 与纯文本 gemma-4-26B）确认
// 只有两类结构化信号可用：
//  ① 引擎能力声明：GET /props → modalities.vision（布尔）；退一步 GET /v1/models →
//     models[0].capabilities 是否含 "multimodal"；
//  ② 模型侧建材：权重同目录是否放着 mmproj*.gguf。
// 两类都不成立 → 必须 unverifiable，绝不降级成 value=false（不能判定 ≠ 不支持）。

// fakeVisionStatusEngine 造一个可控的 #23 假端点：视觉请求返回给定状态码与响应体。
// propsVision=nil → 不给 /props；caps=nil → /v1/models 不带 models[].capabilities。
func fakeVisionStatusEngine(t *testing.T, status int, body string, propsVision *bool, caps []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/props":
			if propsVision == nil {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
				return
			}
			fmt.Fprintf(w, `{"modalities":{"vision":%t,"video":false,"audio":false}}`, *propsVision)
		case "/v1/models":
			if caps == nil {
				fmt.Fprint(w, `{"object":"list","data":[{"id":"fake-llama","object":"model"}]}`)
				return
			}
			c, _ := json.Marshal(caps)
			fmt.Fprintf(w, `{"models":[{"name":"fake-llama","model":"fake-llama","capabilities":%s}],"object":"list","data":[{"id":"fake-llama","object":"model"}]}`, c)
		case "/v1/chat/completions":
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), "image_url") {
				w.WriteHeader(status)
				fmt.Fprint(w, body)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeModelDirWithMMProj 造一个"权重 + 同模型投影器"的目录，返回 (目录, 权重文件名)。
// 名字取自本机实测（Ornith-1.5-35B + mmproj-Ornith-1.5-35B-BF16）——两者共享 "ornith" token。
func writeModelDirWithMMProj(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	weights := "Ornith-1.5-35B-Q4_K_M.gguf"
	for _, name := range []string{weights, "mmproj-Ornith-1.5-35B-BF16.gguf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, weights
}

// 反例①（文案诱骗）：响应体里塞满 "mmproj_missing" / "mmproj file not loaded" 字样，
// 但引擎没有任何结构化信号（无 /props、/v1/models 无 capabilities）→
// **不得**升级成 mmproj_missing（旧实现会中招）、**不得**判 value=false，必须 unverifiable。
func TestProbe23TextDecoyNeverUpgradesToMMProjMissing(t *testing.T) {
	decoy := `{"error":{"code":500,"message":"mmproj_missing: this model does not support images: mmproj file not loaded","type":"server_error"}}`
	srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, decoy, nil, nil)

	vis := ProbeVision(Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second})
	if vis.Value {
		t.Fatalf("引擎报错，视觉不得判 true：%+v", vis)
	}
	if vis.FailureClass == FailMmprojMiss {
		t.Fatalf("绝不许按响应体文案升级成 mmproj_missing（待修补 #23）：%+v", vis)
	}
	if !vis.Undetermined || vis.FailureClass != FailNoStructuredSignal {
		t.Fatalf("无结构化信号时必须是不可判定（%s），实际 %+v", FailNoStructuredSignal, vis)
	}
	if !strings.Contains(vis.Trace.Summary, "未提供结构化信号") {
		t.Fatalf("不可判定必须写明原因：%s", vis.Trace.Summary)
	}
	// 报告层：落 unverifiable[]，而**不是** capabilities[].value=false。
	rep := &ProbeReport{}
	addCapability(rep, "vision", EvidenceVision, vis)
	if c := capByName(rep.Capabilities, "vision"); c != nil {
		t.Fatalf("无结构化信号时不得生成 value=false 能力条目：%+v", c)
	}
	if uv := findUnverifiable(rep.Unverifiable, "vision"); uv == nil || uv.Reason != FailNoStructuredSignal {
		t.Fatalf("应写入 unverifiable[vision]/%s：%+v", FailNoStructuredSignal, rep.Unverifiable)
	}
}

// 反例②（有结构化信号 → 正确判）：信号的取值决定 mmproj_missing / no_vision / true。
func TestProbe23StructuredSignalDecides(t *testing.T) {
	// 实测抄回的引擎文案（真机 500 的原文）：光凭它分不出 no_vision 与 mmproj_missing。
	engineBody := `{"error":{"code":500,"message":"image input is not supported - hint: if this is unexpected, you may need to provide the mmproj","type":"server_error"}}`

	t.Run("引擎声明无视觉+本地有投影器建材→mmproj_missing", func(t *testing.T) {
		srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, engineBody, boolPtr(false), nil)
		ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}
		ep.ModelDir, ep.WeightsName = writeModelDirWithMMProj(t)
		vis := ProbeVision(ep)
		if vis.Value || vis.Undetermined || vis.FailureClass != FailMmprojMiss {
			t.Fatalf("应为确定不支持且分类 %s，实际 %+v", FailMmprojMiss, vis)
		}
		if !strings.Contains(vis.Trace.Summary, "/props.modalities.vision=false") {
			t.Fatalf("证据必须写清用的结构化信号：%s", vis.Trace.Summary)
		}
	})

	t.Run("引擎声明无视觉+本地无投影器建材→no_vision", func(t *testing.T) {
		srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, engineBody, boolPtr(false), nil)
		ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}
		vis := ProbeVision(ep)
		if vis.Value || vis.Undetermined || vis.FailureClass != FailNoVision {
			t.Fatalf("应为确定不支持且分类 %s，实际 %+v", FailNoVision, vis)
		}
	})

	t.Run("同目录只有别的模型的投影器→不误报（no_vision）", func(t *testing.T) {
		// 实测本机 ~/models 的形态：gemma 权重与 mmproj-Ornith 同目录混放。
		srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, engineBody, boolPtr(false), nil)
		dir := t.TempDir()
		weights := "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf"
		for _, name := range []string{weights, "mmproj-Ornith-1.5-35B-BF16.gguf"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second, ModelDir: dir, WeightsName: weights}
		vis := ProbeVision(ep)
		if vis.Value || vis.Undetermined || vis.FailureClass != FailNoVision {
			t.Fatalf("别的模型的投影器不得算到本模型头上（应 %s），实际 %+v", FailNoVision, vis)
		}
	})

	t.Run("引擎声明视觉可用却失败→no_vision（不赖投影器）", func(t *testing.T) {
		srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, engineBody, boolPtr(true), nil)
		ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}
		ep.ModelDir, ep.WeightsName = writeModelDirWithMMProj(t)
		vis := ProbeVision(ep)
		if vis.Value || vis.Undetermined || vis.FailureClass != FailNoVision {
			t.Fatalf("引擎声明视觉可用时失败不该归因到投影器，应 %s，实际 %+v", FailNoVision, vis)
		}
	})

	t.Run("/v1/models capabilities 兜底信号→mmproj_missing", func(t *testing.T) {
		// 不给 /props，只用 /v1/models 的 models[].capabilities（不含 multimodal = 声明无视觉）。
		srv := fakeVisionStatusEngine(t, http.StatusInternalServerError, engineBody, nil, []string{"completion"})
		ep := Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second}
		ep.ModelDir, ep.WeightsName = writeModelDirWithMMProj(t)
		vis := ProbeVision(ep)
		if vis.Value || vis.Undetermined || vis.FailureClass != FailMmprojMiss {
			t.Fatalf("应为 %s，实际 %+v", FailMmprojMiss, vis)
		}
		if !strings.Contains(vis.Trace.Summary, "capabilities") {
			t.Fatalf("证据应指向 capabilities 信号：%s", vis.Trace.Summary)
		}
	})

	t.Run("引擎声明视觉可用且请求通过→true", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/props":
				fmt.Fprint(w, `{"modalities":{"vision":true,"video":true,"audio":false}}`)
			case "/v1/chat/completions":
				fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"红色"}}]}`)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)
		vis := ProbeVision(Endpoint{BaseURL: srv.URL, Model: "fake-llama", Timeout: 5 * time.Second})
		if !vis.Value || !vis.Trace.OK || vis.Undetermined {
			t.Fatalf("视觉请求通过时应判 true：%+v", vis)
		}
	})
}

// 反例③（拿不到信号 → unverifiable）：引擎对图像报 502（不是 4xx 的明确拒绝），
// 且不给任何结构化能力声明 → 归因不可判定，理由写明"引擎未提供结构化信号"；
// 报告与能力快照都落 unverifiable[]，绝不生成 value=false 条目。
func TestProbe23NoSignalIsUnverifiable(t *testing.T) {
	srv := fakeVisionStatusEngine(t, http.StatusBadGateway, `{"error":{"message":"upstream error","type":"server_error"}}`, nil, nil)

	gguf := writeTestGGUF(t, "", "NoSignal-Q4_K_M.gguf") // 本地同目录没有 mmproj 兄弟文件
	rec, rep, err := Probe(ProbeOptions{Target: gguf, Endpoint: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if c := capByName(rep.Capabilities, "vision"); c != nil {
		t.Fatalf("拿不到结构化信号时不得生成 value=false 能力条目：%+v", c)
	}
	uv := findUnverifiable(rep.Unverifiable, "vision")
	if uv == nil {
		t.Fatalf("应写入 unverifiable[vision]：%+v", rep.Unverifiable)
	}
	if uv.Reason != FailNoStructuredSignal {
		t.Fatalf("不可判定理由应为 %s，实际 %q", FailNoStructuredSignal, uv.Reason)
	}
	if !strings.Contains(uv.Evidence, "未提供结构化信号") {
		t.Fatalf("证据须写明引擎未提供结构化信号：%s", uv.Evidence)
	}
	// 快照同样承载（写盘与 --json 共用同一份），且同样不含 vision=false。
	snap := NewCapabilitySnapshot(rec, rep)
	if findUnverifiable(snap.Unverifiable, "vision") == nil {
		t.Fatalf("能力快照应承载 unverifiable[vision]：%+v", snap.Unverifiable)
	}
	if capByName(snap.Capabilities, "vision") != nil {
		t.Fatalf("快照里也不该有 vision=false：%+v", snap.Capabilities)
	}
}

// projectorPresent 的名字配对守卫：本机实测的混放目录不得误配，缺信息一律 false。
func TestProjectorPresentNameAffinity(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"gemma-4-26B-A4B-it-UD-Q4_K_M.gguf", "mmproj-Ornith-1.5-35B-BF16.gguf"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// 混放目录：gemma 不得配到 Ornith 的投影器
	if name, ok := projectorPresent(dir, "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf"); ok {
		t.Fatalf("gemma 不该配到别人的投影器：%s", name)
	}
	// Ornith 应配到自己的
	if name, ok := projectorPresent(dir, "Ornith-1.5-35B-Q4_K_M.gguf"); !ok || name != "mmproj-Ornith-1.5-35B-BF16.gguf" {
		t.Fatalf("Ornith 应配到自己的投影器，实际 ok=%t name=%s", ok, name)
	}
	// 缺信息/空目录一律 false（缺=未知，绝不猜）
	if _, ok := projectorPresent("", "x.gguf"); ok {
		t.Fatal("空目录不该配到东西")
	}
	if _, ok := projectorPresent(dir, ""); ok {
		t.Fatal("没给权重文件名时无从配对")
	}
	if _, ok := projectorPresent(t.TempDir(), "gemma-4-26B.gguf"); ok {
		t.Fatal("空目录里不该配到东西")
	}
}

// 真机形态回归：把本机实测（llama.cpp build 10470）抄回的原始响应体裁进测试，
// 钉住信号解析对真实字段名/结构不漂移（字段改名 = 测试红，而不是悄悄判错）。
func TestProbe23RealLlamaCppSignalShapes(t *testing.T) {
	propsWith := `{"total_slots":4,"model_alias":"~/models/Ornith-1.5-35B-Q4_K_M.gguf","model_path":"~/models/Ornith-1.5-35B-Q4_K_M.gguf","modalities":{"vision":true,"video":true,"audio":false},"media_marker":"<__media_x__>","chat_template":"{{ .Prompt }}","build_info":"b10470-34af94cd9","is_sleeping":false}`
	propsWithout := `{"total_slots":4,"model_alias":"~/models/Ornith-1.5-35B-Q4_K_M.gguf","modalities":{"vision":false,"video":false,"audio":false},"media_marker":"<__media_x__>","is_sleeping":false}`
	modelsWith := `{"models":[{"name":"~/models/Ornith-1.5-35B-Q4_K_M.gguf","model":"~/models/Ornith-1.5-35B-Q4_K_M.gguf","capabilities":["completion","multimodal"]}],"object":"list","data":[{"id":"x","aliases":["x"],"object":"model","owned_by":"llamacpp"}]}`
	modelsWithout := `{"models":[{"name":"x","capabilities":["completion"]}],"object":"list","data":[{"id":"x","object":"model"}]}`
	modelsLegacy := `{"object":"list","data":[{"id":"x","object":"model"}]}` // 老式 OpenAI 响应：没有 models[]

	cases := []struct {
		name       string
		props      string
		models     string
		wantKnown  bool
		wantActive bool
		anchorPart string
	}{
		{"挂mmproj", propsWith, modelsWith, true, true, "/props.modalities.vision"},
		{"未挂mmproj", propsWithout, modelsWithout, true, false, "/props.modalities.vision"},
		{"纯文本模型", propsWithout, modelsWithout, true, false, "/props.modalities.vision"},
		{"只有老式/v1/models", "", modelsLegacy, false, false, ""},
		{"/props在/v1下", "", `{}`, false, false, ""}, // 未提供 /v1/props → 未知（此处只验不 panic）
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/props":
					if tc.props == "" {
						w.WriteHeader(http.StatusNotFound)
						fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
						return
					}
					fmt.Fprint(w, tc.props)
				case "/v1/models":
					fmt.Fprint(w, tc.models)
				default:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
				}
			}))
			t.Cleanup(srv.Close)
			sig := probeVisionCapSignal(Endpoint{BaseURL: srv.URL, Timeout: 5 * time.Second})
			if sig.Known != tc.wantKnown {
				t.Fatalf("Known 不符：want=%t 实际 %+v", tc.wantKnown, sig)
			}
			if sig.Known && sig.Active != tc.wantActive {
				t.Fatalf("Active 不符：want=%t 实际 %+v", tc.wantActive, sig)
			}
			if sig.Known && !strings.Contains(sig.Anchor, tc.anchorPart) {
				t.Fatalf("证据锚应含 %q，实际 %q", tc.anchorPart, sig.Anchor)
			}
		})
	}
	// 兜底路径单独验：不给 /props，只给 /v1/models 的 models[].capabilities。
	t.Run("capabilities兜底", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/v1/models" {
				fmt.Fprint(w, modelsWith)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}))
		t.Cleanup(srv.Close)
		sig := probeVisionCapSignal(Endpoint{BaseURL: srv.URL, Timeout: 5 * time.Second})
		if !sig.Known || !sig.Active || !strings.Contains(sig.Anchor, "capabilities") {
			t.Fatalf("应从 capabilities 兜底判定有视觉：%+v", sig)
		}
	})
}

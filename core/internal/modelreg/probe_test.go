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

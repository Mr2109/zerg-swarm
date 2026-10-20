// inject_test.go — AI 注入三条面的判据格（v3「AI 注入」：端点分配 / 占位符换值 / 响应擦除）
//
//	① TestAIPlanShape                     env 形状对（base/model）；非法参数 ⇒ 零值 plan（fail-closed）
//	② TestRewriteWhitelistedHost          白名单 host ⇒ 换值；只改指定 header；同名只解析一次；不改输入
//	③ TestRewriteNonWhitelistedHost       非白名单 host ⇒ **报错且一个字节都不改**（语义写死为"拒"，不是"原样"）
//	④ TestScrubResponseVariants           明文 / base64 四变体 / percent 两变体 全擦；干净正文不动
//	⑤ TestNoSecretInErrorsOrAudit          错误串、审计事件、fmt 面、本件源码里都搜不到真值
//	⑥ TestNilResolverAndEmptySecrets       nil 解析器 / 空 secrets / 空 headers 一路不 panic
//	⑦ TestRewriteConcurrentNoSharedMutation  并发改写不共享底层数组（-race 有牙）
package cocoon

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

// testSecret — 32 字符的假真值。取这个长度是为了让"base64/percent 变体互不相同"成立
// （短串的三种编码会退化成同形，测不出编码分支）。
const testSecret = "sk-live-7f3a9c2d4b6e8015"

// fakeSecrets — 可记账的假解析器：调用次数、调用名、可注入错误。
type fakeSecrets struct {
	mu    sync.Mutex
	m     map[string]string
	err   error
	calls []string
}

func (f *fakeSecrets) Resolve(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.m[name]
	if !ok {
		return "", fmt.Errorf("no such secret %q", name)
	}
	return v, nil
}

func (f *fakeSecrets) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// injectSink — 审计事件收集器（-race 下多 goroutine 共用）。
type injectSink struct {
	mu  sync.Mutex
	evs []InjectEvent
}

func (s *injectSink) Hook(ev InjectEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, ev)
}

func (s *injectSink) all() []InjectEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]InjectEvent(nil), s.evs...)
}

// withInjectSink — 挂上审计收集器，用例结束时摘掉（钩子是包级状态，必须复原）。
func withInjectSink(t *testing.T) *injectSink {
	t.Helper()
	s := &injectSink{}
	SetInjectAuditHook(s.Hook)
	t.Cleanup(func() { SetInjectAuditHook(nil) })
	return s
}

// headerOf — 按 K,V,K,V… 造 header（值里未给出的一律单值）。
func headerOf(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Add(kv[i], kv[i+1])
	}
	return h
}

// secretVariants — 测试侧**独立**算一遍编码变体（不复用被测代码的 scrubVariants，
// 否则"擦掉了没"这件事会与被测实现同源，一起错就都发现不了）。
func secretVariants(s string) []string {
	raw := []byte(s)
	out := []string{
		s,
		base64.StdEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawURLEncoding.EncodeToString(raw),
		url.QueryEscape(s),
		url.PathEscape(s),
	}
	seen := map[string]struct{}{}
	var uniq []string
	for _, v := range out {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		uniq = append(uniq, v)
	}
	return uniq
}

// assertNoSecret — 断言一段文本里既无明文、也无任何编码变体（这是"不落真值"的统一口径）。
func assertNoSecret(t *testing.T, where, text string) {
	t.Helper()
	for _, v := range secretVariants(testSecret) {
		if strings.Contains(text, v) {
			t.Fatalf("%s 里出现了真值（明文或编码变体 %q）：%q", where, v, text)
		}
	}
	if strings.Contains(text, testSecret) {
		t.Fatalf("%s 里出现了明文真值：%q", where, text)
	}
}

// ── ① env 形状 ────────────────────────────────────────────────────────

func TestAIPlanShape(t *testing.T) {
	t.Run("回环 base 与 model 进 env", func(t *testing.T) {
		p := BuildAIEnv("coc-01", "gpt-4o-mini", "http://127.0.0.1:8082")
		if !p.Valid() {
			t.Fatalf("plan 应有效: %v", p)
		}
		if p.Base != "http://127.0.0.1:8082" {
			t.Fatalf("Base = %q, 期望 http://127.0.0.1:8082", p.Base)
		}
		if p.Model != "gpt-4o-mini" {
			t.Fatalf("Model = %q", p.Model)
		}
		if len(p.Env) != 2 {
			t.Fatalf("Env 应只有两项（BASE/MODEL），得到 %v", p.Env)
		}
		if p.Env[EnvAIBase] != p.Base {
			t.Fatalf("Env[%s] = %q, 期望与 Base 一致", EnvAIBase, p.Env[EnvAIBase])
		}
		if p.Env[EnvAIModel] != p.Model {
			t.Fatalf("Env[%s] = %q, 期望与 Model 一致", EnvAIModel, p.Env[EnvAIModel])
		}
		// 顺序稳定（exec 的 envp 要可复现）
		want := []string{"ZERG_AI_BASE=http://127.0.0.1:8082", "ZERG_AI_MODEL=gpt-4o-mini"}
		if got := p.Environ(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Environ() = %v, 期望 %v", got, want)
		}
	})

	t.Run("尾斜杠去掉 路径保留", func(t *testing.T) {
		cases := map[string]string{
			"http://127.0.0.1:8082/":           "http://127.0.0.1:8082",
			"http://127.0.0.1:8082/broker/ai/": "http://127.0.0.1:8082/broker/ai",
			"http://localhost:8082":            "http://localhost:8082",
			"http://[::1]:8082":                "http://[::1]:8082",
			"https://127.0.0.1:8443":           "https://127.0.0.1:8443",
			"  http://127.0.0.1:8082  ":        "http://127.0.0.1:8082",
			"http://127.0.0.1:8082/broker/ai":  "http://127.0.0.1:8082/broker/ai",
			"http://127.0.0.2:8082":            "http://127.0.0.2:8082",
		}
		for in, want := range cases {
			p, err := BuildAIEnvChecked("coc-01", "m", in)
			if err != nil {
				t.Fatalf("upstream %q 应通过: %v", in, err)
			}
			if p.Base != want {
				t.Fatalf("upstream %q ⇒ Base %q, 期望 %q", in, p.Base, want)
			}
			if p.Env[EnvAIBase] != want {
				t.Fatalf("upstream %q ⇒ env base %q, 期望 %q", in, p.Env[EnvAIBase], want)
			}
		}
	})

	t.Run("非法参数一律零值 plan", func(t *testing.T) {
		cases := []struct {
			name     string
			inst     string
			model    string
			upstream string
			want     error
		}{
			{"空实例", "  ", "m", "http://127.0.0.1:8082", ErrAIPlanEmptyInstance},
			{"空模型", "coc", "", "http://127.0.0.1:8082", ErrAIPlanEmptyModel},
			{"模型带空格", "coc", "gpt 4o", "http://127.0.0.1:8082", ErrAIPlanBadModel},
			{"模型带换行", "coc", "gpt\n4o", "http://127.0.0.1:8082", ErrAIPlanBadModel},
			{"非回环", "coc", "m", "http://<cluster-ip>:8082", ErrAIPlanNotLoopback},
			{"公网域名", "coc", "m", "https://api.openai.com", ErrAIPlanNotLoopback},
			{"scheme 非法", "coc", "m", "ftp://127.0.0.1:8082", ErrAIPlanBadUpstream},
			{"相对地址", "coc", "m", "127.0.0.1:8082", ErrAIPlanBadUpstream},
			{"空地址", "coc", "m", "", ErrAIPlanBadUpstream},
			{"带 userinfo", "coc", "m", "http://u:p4ss@127.0.0.1:8082", ErrAIPlanUpstreamCredential},
			{"带 query", "coc", "m", "http://127.0.0.1:8082/?t=1", ErrAIPlanUpstreamQuery},
			{"带 fragment", "coc", "m", "http://127.0.0.1:8082/#x", ErrAIPlanUpstreamQuery},
		}
		for _, c := range cases {
			p, err := BuildAIEnvChecked(c.inst, c.model, c.upstream)
			if !errors.Is(err, c.want) {
				t.Fatalf("%s: err = %v, 期望 %v", c.name, err, c.want)
			}
			if p.Valid() || p.Env != nil || p.Base != "" || p.Model != "" {
				t.Fatalf("%s: 失败时必须是零值 plan，得到 %+v", c.name, p)
			}
			// 不带错版本：零值 + 无 env（茧内拿不到 AI 能力，安全侧）
			if got := BuildAIEnv(c.inst, c.model, c.upstream); got.Valid() || got.Env != nil {
				t.Fatalf("%s: BuildAIEnv 失败时应为零值，得到 %+v", c.name, got)
			}
		}
	})

	t.Run("零值 plan 的 fmt 面", func(t *testing.T) {
		var p AIPlan
		if got := p.String(); !strings.Contains(got, `base=""`) {
			t.Fatalf("零值 String() = %q", got)
		}
		if p.Environ() != nil {
			t.Fatalf("零值 Environ() 应为 nil，得到 %v", p.Environ())
		}
	})
}

// ── ② 白名单 host ⇒ 换值 ──────────────────────────────────────────────

func TestRewriteWhitelistedHost(t *testing.T) {
	t.Run("占位符换成真值 其余 header 不动 输入不变", func(t *testing.T) {
		sink := withInjectSink(t)
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := headerOf(
			"Authorization", "Bearer "+SecretPlaceholder("openai"),
			"X-Api-Key", SecretPlaceholder("openai"),
			"X-Trace-Id", "trace-123",
			"User-Agent", "zerg-cocoon/1",
		)
		before := cloneHeader(in)

		out, err := RewriteRequest(in, "api.openai.com", fs)
		if err != nil {
			t.Fatalf("白名单 host 应放行: %v", err)
		}
		if got, want := out.Get("Authorization"), "Bearer "+testSecret; got != want {
			t.Fatalf("Authorization = %q, 期望 %q", got, want)
		}
		if got := out.Get("X-Api-Key"); got != testSecret {
			t.Fatalf("X-Api-Key = %q, 期望真值", got)
		}
		// 非认证 header 一个字节都不该变
		if out.Get("X-Trace-Id") != "trace-123" || out.Get("User-Agent") != "zerg-cocoon/1" {
			t.Fatalf("非认证 header 被改了: %v", out)
		}
		// 输入不可就地修改（并发复用同一份 header 的安全前提）
		if !reflect.DeepEqual(in, before) {
			t.Fatalf("输入 header 被就地改了: %v", in)
		}
		// 同名只解析一次
		if n := fs.callCount(); n != 1 {
			t.Fatalf("同一密钥名解析了 %d 次，期望 1 次", n)
		}
		if len(sink.all()) == 0 {
			t.Fatalf("应有审计事件")
		}
	})

	t.Run("多值 header 逐个换", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := http.Header{}
		in.Add("Authorization", "Bearer "+SecretPlaceholder("openai"))
		in.Add("Authorization", "Basic "+SecretPlaceholder("openai"))
		out, err := RewriteRequest(in, "api.openai.com", fs)
		if err != nil {
			t.Fatalf("应放行: %v", err)
		}
		want := []string{"Bearer " + testSecret, "Basic " + testSecret}
		if !reflect.DeepEqual(out["Authorization"], want) {
			t.Fatalf("多值 header = %v, 期望 %v", out["Authorization"], want)
		}
		if n := fs.callCount(); n != 1 {
			t.Fatalf("多值同一名字应只解析 1 次，得到 %d", n)
		}
	})

	t.Run("一个值里两个占位符", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"a": "AAAA-secret-1111", "b": "BBBB-secret-2222"}}
		in := headerOf("Authorization", "Key1="+SecretPlaceholder("a")+"; Key2="+SecretPlaceholder("b"))
		out, err := RewriteRequest(in, "api.anthropic.com", fs)
		if err != nil {
			t.Fatalf("应放行: %v", err)
		}
		if got, want := out.Get("Authorization"), "Key1=AAAA-secret-1111; Key2=BBBB-secret-2222"; got != want {
			t.Fatalf("Authorization = %q, 期望 %q", got, want)
		}
	})

	t.Run("真值里的 $1 原样保留 不被当捕获组", func(t *testing.T) {
		// ReplaceAllString 会把 `$1` 展开成空串 —— 真值一旦含 `$`，静默吃字节。
		dollar := "pre-$1x-abc9"
		fs := &fakeSecrets{m: map[string]string{"openai": dollar}}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		out, err := RewriteRequest(in, "api.openai.com", fs)
		if err != nil {
			t.Fatalf("应放行: %v", err)
		}
		if got, want := out.Get("Authorization"), "Bearer "+dollar; got != want {
			t.Fatalf("Authorization = %q, 期望 %q（`$1` 不许被展开）", got, want)
		}
	})

	t.Run("host 归一化 端口/大写/尾点", func(t *testing.T) {
		for _, host := range []string{
			"api.openai.com",
			"API.OpenAI.COM",
			"api.openai.com:443",
			"api.openai.com.",
			"  api.openai.com  ",
			"[::1]:8082/", // 下面这条只验证"带路径形态"被 ErrBadHost 拒，见 ③
		} {
			fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
			_, err := RewriteRequest(headerOf("Authorization", "Bearer "+SecretPlaceholder("openai")), host, fs)
			if host == "[::1]:8082/" {
				if !errors.Is(err, ErrBadHost) {
					t.Fatalf("host %q 含路径应 ErrBadHost, 得到 %v", host, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("host %q 应放行: %v", host, err)
			}
		}
	})

	t.Run("header 名大小写不敏感", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := http.Header{"authorization": {"Bearer " + SecretPlaceholder("openai")}}
		out, err := RewriteRequest(in, "api.openai.com", fs)
		if err != nil {
			t.Fatalf("应放行: %v", err)
		}
		if got := out["authorization"][0]; got != "Bearer "+testSecret {
			t.Fatalf("小写 header 名未换值: %q", got)
		}
	})

	t.Run("占位符在不可换值 header ⇒ 拒且不改", func(t *testing.T) {
		sink := withInjectSink(t)
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := headerOf(
			"Authorization", "Bearer "+SecretPlaceholder("openai"),
			"X-Debug-Echo", SecretPlaceholder("openai"),
		)
		before := cloneHeader(in)
		out, err := RewriteRequest(in, "api.openai.com", fs)
		if !errors.Is(err, ErrHeaderNotAllowed) {
			t.Fatalf("err = %v, 期望 ErrHeaderNotAllowed", err)
		}
		if out != nil {
			t.Fatalf("失败时应返回 nil, 得到 %v", out)
		}
		if !reflect.DeepEqual(in, before) {
			t.Fatalf("失败时输入被改了: %v", in)
		}
		if n := fs.callCount(); n != 0 {
			t.Fatalf("判定不过时不该解析密钥，却解析了 %d 次", n)
		}
		if evs := sink.all(); len(evs) != 1 || evs[0].OK || evs[0].Reason != "header_not_allowed" {
			t.Fatalf("审计事件不对: %+v", evs)
		}
	})

	t.Run("占位符形态非法 ⇒ ErrBadPlaceholder", func(t *testing.T) {
		for _, bad := range []string{
			"Bearer {{secret:}}",
			"Bearer {{secret:OPENAI}}",
			"Bearer {{secret:openai",
			"Bearer {{secret:open ai}}",
			"Bearer {{secret:openai}} {{secret:",
		} {
			fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
			in := headerOf("Authorization", bad)
			before := cloneHeader(in)
			_, err := RewriteRequest(in, "api.openai.com", fs)
			if !errors.Is(err, ErrBadPlaceholder) {
				t.Fatalf("%q: err = %v, 期望 ErrBadPlaceholder", bad, err)
			}
			if !reflect.DeepEqual(in, before) {
				t.Fatalf("%q: 失败时输入被改了", bad)
			}
			if n := fs.callCount(); n != 0 {
				t.Fatalf("%q: 形态非法时不该解析密钥", bad)
			}
		}
	})

	t.Run("第二个密钥解析失败 ⇒ 整条拒 零部分修改", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"a": "AAAA-secret-1111"}} // 没有 "b"
		in := headerOf(
			"Authorization", "Key1="+SecretPlaceholder("a"),
			"X-Api-Key", "Key2="+SecretPlaceholder("b"),
		)
		before := cloneHeader(in)
		out, err := RewriteRequest(in, "api.openai.com", fs)
		if !errors.Is(err, ErrSecretUnavailable) {
			t.Fatalf("err = %v, 期望 ErrSecretUnavailable", err)
		}
		if out != nil {
			t.Fatalf("失败时应返回 nil")
		}
		if !reflect.DeepEqual(in, before) {
			t.Fatalf("半路失败不该留下改了一半的 header: %v", in)
		}
	})

	t.Run("解析器给空值 ⇒ 拒（换空等于把 header 清空）", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"openai": ""}}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		before := cloneHeader(in)
		_, err := RewriteRequest(in, "api.openai.com", fs)
		if !errors.Is(err, ErrSecretEmpty) {
			t.Fatalf("err = %v, 期望 ErrSecretEmpty", err)
		}
		if !reflect.DeepEqual(in, before) {
			t.Fatalf("失败时输入被改了: %v", in)
		}
	})

	t.Run("自建上游：白名单可替换", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		out, err := RewriteRequestWithAllowlist(in, "127.0.0.1:9999", []string{"127.0.0.1"}, fs)
		if err != nil {
			t.Fatalf("自定义白名单应放行: %v", err)
		}
		if got := out.Get("Authorization"); got != "Bearer "+testSecret {
			t.Fatalf("Authorization = %q", got)
		}
		// 同一 host 走默认表必须拒 —— 证明"默认表不收回环"不是纸面约定
		if _, err := RewriteRequest(in, "127.0.0.1:9999", fs); !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("默认白名单不该放行回环 host, 得到 %v", err)
		}
	})
}

// ── ③ 非白名单 host ⇒ 报错且不改 ──────────────────────────────────────

func TestRewriteNonWhitelistedHost(t *testing.T) {
	// ★ 语义写死：**拒**（返回 nil + error），不是"原样放行"。
	// 原样放行的后果是把字面量占位符发给上游（静默错配），且调用方分不清"换了"和"没换"。
	badHosts := []struct {
		host string
		want error
	}{
		{"evil.example.com", ErrHostNotAllowed},
		{"api.openai.com.evil.com", ErrHostNotAllowed}, // 后缀攻击：经典翻车点
		{"evilapi.openai.com", ErrHostNotAllowed},      // 前缀粘连
		{"api.openai.com.cn", ErrHostNotAllowed},
		{"127.0.0.1:8082", ErrHostNotAllowed}, // 回环（茧→broker 那一跳）不在表内
		{"localhost", ErrHostNotAllowed},
		{"<cluster-ip>", ErrHostNotAllowed},
		{"", ErrBadHost},
		{"api.openai.com/v1", ErrBadHost},
		{"user@api.openai.com", ErrBadHost},
		{"api.openai.com:44 3", ErrBadHost},
	}
	for _, c := range badHosts {
		t.Run("host="+c.host, func(t *testing.T) {
			sink := withInjectSink(t)
			fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
			in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"), "X-Trace-Id", "t-1")
			before := cloneHeader(in)

			out, err := RewriteRequest(in, c.host, fs)
			if !errors.Is(err, c.want) {
				t.Fatalf("host %q: err = %v, 期望 %v", c.host, err, c.want)
			}
			if out != nil {
				t.Fatalf("host %q: 拒的时候返回 nil, 得到 %v", c.host, out)
			}
			if !reflect.DeepEqual(in, before) {
				t.Fatalf("host %q: 输入被改了: %v", c.host, in)
			}
			if n := fs.callCount(); n != 0 {
				t.Fatalf("host %q: 不该解析密钥，却解析了 %d 次", c.host, n)
			}
			evs := sink.all()
			if len(evs) != 1 || evs[0].OK {
				t.Fatalf("host %q: 审计应记一条失败，得到 %+v", c.host, evs)
			}
		})
	}
}

// ── ④ 响应三种编码擦除 ────────────────────────────────────────────────

func TestScrubResponseVariants(t *testing.T) {
	t.Run("明文与六种编码变体全擦", func(t *testing.T) {
		variants := secretVariants(testSecret)
		var b bytes.Buffer
		b.WriteString("upstream error page\n")
		for i, v := range variants {
			fmt.Fprintf(&b, "leak[%d]=%s\n", i, v)
		}
		b.WriteString("tail-intact")
		body := b.Bytes()

		out := ScrubResponse(body, []string{testSecret})
		if strings.Contains(string(out), "leak[") == false {
			t.Fatalf("结构被破坏: %q", out)
		}
		if !strings.HasSuffix(string(out), "tail-intact") {
			t.Fatalf("尾部字节被吃掉: %q", out)
		}
		if n := strings.Count(string(out), ScrubPlaceholder); n != len(variants) {
			t.Fatalf("擦除 %d 处，期望 %d 处（变体数）:%q", n, len(variants), out)
		}
		// 逐变体复查：一处不许剩（这是"三编码"的硬判据）
		for _, v := range variants {
			if bytes.Contains(out, []byte(v)) {
				t.Fatalf("变体 %q 没擦干净: %q", v, out)
			}
		}
		// 明文 base64 的标签（key 名）不该被误伤
		if !strings.Contains(string(out), "leak[0]="+ScrubPlaceholder) {
			t.Fatalf("替换落点不对: %q", out)
		}
	})

	t.Run("单独覆盖 明文/base64/url 三种（题面三格）", func(t *testing.T) {
		cases := []struct{ name, secret, form string }{
			{"明文", testSecret, testSecret},
			{"base64-std", testSecret, base64.StdEncoding.EncodeToString([]byte(testSecret))},
			// ★ url-encoded 这一格必须用**含特殊字符**的值：全字母数字的真值 url-encode 后
			// 与明文一模一样，拿它测 url 分支等于没测（恒真的假绿）。
			{"url-encoded", "sk live?k=v&x=%9", url.QueryEscape("sk live?k=v&x=%9")},
		}
		for _, c := range cases {
			out := ScrubResponse([]byte("x "+c.form+" y"), []string{c.secret})
			if got, want := string(out), "x "+ScrubPlaceholder+" y"; got != want {
				t.Fatalf("%s: 擦除结果 = %q, 期望 %q", c.name, got, want)
			}
		}
	})

	t.Run("编码压力：七个变体逐个擦（每个变体单打一格）", func(t *testing.T) {
		// ★ 为什么必须有这一格：testSecret 的 base64 里既没有 `+` 也没有 `/`，
		// 于是 std / raw / url / raw-url 四个变体**撞成同一个串** —— 少实现两个分支也照样绿。
		// 这条真值（含 `?&=+/ %` 与中文）让七个变体两两不同，逐格单打才咬得住。
		const stress = "tk?&=+/ %中9a"
		raw := []byte(stress)
		variants := map[string]string{
			"明文":            stress,
			"base64-std":    base64.StdEncoding.EncodeToString(raw),
			"base64-raw":    base64.RawStdEncoding.EncodeToString(raw),
			"base64-url":    base64.URLEncoding.EncodeToString(raw),
			"base64-rawurl": base64.RawURLEncoding.EncodeToString(raw),
			"percent-query": url.QueryEscape(stress),
			"percent-path":  url.PathEscape(stress),
		}
		if len(variants) != 7 {
			t.Fatalf("夹具坏了：变体应两两不同，得到 %d 个", len(variants))
		}
		seen := map[string]string{}
		for name, v := range variants {
			if other, dup := seen[v]; dup {
				t.Fatalf("夹具坏了：%s 与 %s 撞成同一串 %q（测不出该分支）", name, other, v)
			}
			seen[v] = name
		}
		for name, v := range variants {
			out := ScrubResponse([]byte("x "+v+" y"), []string{stress})
			if got, want := string(out), "x "+ScrubPlaceholder+" y"; got != want {
				t.Fatalf("%s（%q）没擦：得到 %q", name, v, got)
			}
		}
	})

	t.Run("干净正文一个字节不动", func(t *testing.T) {
		body := []byte(`{"ok":true,"model":"gpt-4o-mini"}`)
		out := ScrubResponse(body, []string{testSecret})
		if !bytes.Equal(out, body) {
			t.Fatalf("干净正文被改了: %q", out)
		}
	})

	t.Run("两条真值 其中一条是另一条的前缀", func(t *testing.T) {
		short, long := "sk-live-abc", "sk-live-abcdefgh"
		body := []byte("a=" + long + " b=" + short)
		out := ScrubResponse(body, []string{short, long})
		// 长的先擦 ⇒ 短的不得在长值的残骸里二次命中，残片一个不许留
		if bytes.Contains(out, []byte("abcdefgh")) {
			t.Fatalf("前缀残片没擦净: %q", out)
		}
		if got, want := string(out), "a=[REDACTED] b=[REDACTED]"; got != want {
			t.Fatalf("擦除结果 = %q, 期望 %q", got, want)
		}
	})

	t.Run("短真值与非真值原样", func(t *testing.T) {
		body := []byte("abc appears everywhere; abc")
		if out := ScrubResponse(body, []string{"abc"}); !bytes.Equal(out, body) {
			t.Fatalf("短于 %d 字节的真值不该擦（会打成马赛克）: %q", minScrubLen, out)
		}
		if out := ScrubResponse(body, []string{"", ""}); !bytes.Equal(out, body) {
			t.Fatalf("空真值不该擦: %q", out)
		}
	})
}

// ── ⑤ 真值不进错误/审计/日志 ──────────────────────────────────────────

// leakyStringFields — 反射扫结构体里的字符串字段，返回"含 needle"的字段名。
// 这是 auth_test.go ⑧「反射扫管理器全图」的同款手法。
func leakyStringFields(v any, needle string) []string {
	var hits []string
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() == reflect.String && strings.Contains(f.String(), needle) {
			hits = append(hits, rt.Field(i).Name)
		}
	}
	return hits
}

func TestNoSecretInErrorsOrAudit(t *testing.T) {
	t.Run("解析器把真值拼进错误也不外泄", func(t *testing.T) {
		sink := withInjectSink(t)
		// 故意做一个"手滑"的解析器：错误文本里带明文与 base64
		leaky := FuncSecretResolver(func(name string) (string, error) {
			return "", fmt.Errorf("vault: bad key %s (b64 %s)",
				testSecret, base64.StdEncoding.EncodeToString([]byte(testSecret)))
		})
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		_, err := RewriteRequest(in, "api.openai.com", leaky)
		if !errors.Is(err, ErrSecretUnavailable) {
			t.Fatalf("err = %v, 期望 ErrSecretUnavailable", err)
		}
		assertNoSecret(t, "错误串", err.Error())
		for _, ev := range sink.all() {
			assertNoSecret(t, "审计事件 String()", ev.String())
			assertNoSecret(t, "审计事件 GoString()", ev.GoString())
			assertNoSecret(t, "审计事件 fmt %+v", fmt.Sprintf("%+v", ev))
			if hits := leakyStringFields(ev, testSecret); len(hits) > 0 {
				t.Fatalf("审计事件字段 %v 含真值", hits)
			}
		}
	})

	t.Run("成功路径的审计事件也不含真值", func(t *testing.T) {
		// ★ 失败路径不带真值是显然的；**成功**路径才是"顺手把值写进 reason/name"的高危处
		// （name 字段最容易被写成"名字=值"）。这条用例专门打成功路径。
		sink := withInjectSink(t)
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		if _, err := RewriteRequest(in, "api.openai.com", fs); err != nil {
			t.Fatalf("应放行: %v", err)
		}
		evs := sink.all()
		if len(evs) != 1 || !evs[0].OK || evs[0].N != 1 {
			t.Fatalf("审计事件不对: %+v", evs)
		}
		for _, ev := range evs {
			if hits := leakyStringFields(ev, testSecret); len(hits) > 0 {
				t.Fatalf("成功路径审计事件字段 %v 含真值: %+v", hits, ev)
			}
			assertNoSecret(t, "成功事件 String()", ev.String())
			assertNoSecret(t, "成功事件 GoString()", ev.GoString())
			assertNoSecret(t, "成功事件 %+v", fmt.Sprintf("%+v", ev))
			assertNoSecret(t, "成功事件 %#v", fmt.Sprintf("%#v", ev))
			if ev.Name != "openai" {
				t.Fatalf("Name 应是密钥名（引用），得到 %q", ev.Name)
			}
		}
	})

	t.Run("upstream 里的凭据不进错误串", func(t *testing.T) {
		upstream := "http://user:" + testSecret + "@127.0.0.1:8082"
		_, err := BuildAIEnvChecked("coc-01", "m", upstream)
		if !errors.Is(err, ErrAIPlanUpstreamCredential) {
			t.Fatalf("err = %v, 期望 ErrAIPlanUpstreamCredential", err)
		}
		assertNoSecret(t, "userinfo 错误串", err.Error())

		// query 形态同理：唯一用途就是夹带令牌
		_, err = BuildAIEnvChecked("coc-01", "m", "http://127.0.0.1:8082/?api_key="+testSecret)
		if !errors.Is(err, ErrAIPlanUpstreamQuery) {
			t.Fatalf("err = %v, 期望 ErrAIPlanUpstreamQuery", err)
		}
		assertNoSecret(t, "query 错误串", err.Error())

		// 解析不了的原值同样不回显
		_, err = BuildAIEnvChecked("coc-01", "m", "http://127.0.0.1:8082/%zz"+testSecret)
		assertNoSecret(t, "解析失败错误串", err.Error())
	})

	t.Run("换值失败矩阵：每条错误串都搜不到真值", func(t *testing.T) {
		fs := &fakeSecrets{m: map[string]string{"openai": testSecret}}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		errs := []error{}
		if _, err := RewriteRequest(in, "evil.example.com", fs); err != nil {
			errs = append(errs, err)
		}
		if _, err := RewriteRequest(in, "", fs); err != nil {
			errs = append(errs, err)
		}
		if _, err := RewriteRequest(in, "api.openai.com", nil); err != nil {
			errs = append(errs, err)
		}
		if _, err := RewriteRequest(headerOf("X-Debug", SecretPlaceholder("openai")), "api.openai.com", fs); err != nil {
			errs = append(errs, err)
		}
		if _, err := RewriteRequest(headerOf("Authorization", "{{secret:openai"), "api.openai.com", fs); err != nil {
			errs = append(errs, err)
		}
		if _, err := RewriteRequest(headerOf("Authorization", SecretPlaceholder("nope")), "api.openai.com", fs); err != nil {
			errs = append(errs, err)
		}
		if len(errs) != 6 {
			t.Fatalf("失败矩阵应有 6 条错误，得到 %d", len(errs))
		}
		for i, err := range errs {
			assertNoSecret(t, fmt.Sprintf("错误 %d（%v）", i, err), err.Error())
		}
	})

	t.Run("擦除产物与 plan 的 fmt 面不含真值", func(t *testing.T) {
		out := ScrubResponse([]byte("k="+testSecret), []string{testSecret})
		assertNoSecret(t, "擦除产物", string(out))
		p := BuildAIEnv("coc-01", "m", "http://127.0.0.1:8082")
		assertNoSecret(t, "plan String()", p.String())
		assertNoSecret(t, "plan %+v", fmt.Sprintf("%+v", p))
		assertNoSecret(t, "plan %#v", fmt.Sprintf("%#v", p))
	})

	t.Run("反射扫描有牙（正控）", func(t *testing.T) {
		type leakyEvent struct {
			Host string
			Name string
			Note string
		}
		probe := leakyEvent{Host: "h", Name: "n", Note: "prefix-" + testSecret}
		if hits := leakyStringFields(probe, testSecret); len(hits) != 1 || hits[0] != "Note" {
			t.Fatalf("扫描器应咬到 Note，得到 %v", hits)
		}
	})

	t.Run("本件源码不写日志", func(t *testing.T) {
		// 「不落日志真值」的结构性做法：本文件根本不写日志（审计只经回调，回调只收 InjectEvent）。
		src, err := os.ReadFile("inject.go")
		if err != nil {
			t.Fatalf("读源码失败: %v", err)
		}
		for _, banned := range []string{
			"log.Print", "log.Printf", "log.Fatal", "log.Panic", "log.New",
			"slog.", "fmt.Print", "fmt.Printf", "os.Stdout", "os.Stderr", "println(", "panic(",
		} {
			if bytes.Contains(src, []byte(banned)) {
				t.Fatalf("inject.go 出现日志/直写调用 %q（真值不得落日志）", banned)
			}
		}
	})
}

// ── ⑥ nil / 空值不 panic ──────────────────────────────────────────────

func TestNilResolverAndEmptySecrets(t *testing.T) {
	t.Run("nil headers + nil 解析器", func(t *testing.T) {
		out, err := RewriteRequest(nil, "api.openai.com", nil)
		if err != nil {
			t.Fatalf("空 header 无需换值，不该报错: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("应返回空 header, 得到 %v", out)
		}
	})

	t.Run("无占位符时 nil 解析器照常放行", func(t *testing.T) {
		in := headerOf("Authorization", "Bearer pre-shared-token", "X-Trace-Id", "t")
		out, err := RewriteRequest(in, "api.openai.com", nil)
		if err != nil {
			t.Fatalf("没有占位符时不碰解析器，不该报错: %v", err)
		}
		if !reflect.DeepEqual(out, in) {
			t.Fatalf("无占位符应原样返回副本: %v", out)
		}
		// 副本不共享底层数组
		out["Authorization"][0] = "mutated"
		if in.Get("Authorization") != "Bearer pre-shared-token" {
			t.Fatalf("返回值与输入共享了底层数组")
		}
	})

	t.Run("有占位符 + nil 解析器 ⇒ 拒 不 panic", func(t *testing.T) {
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		out, err := RewriteRequest(in, "api.openai.com", nil)
		if !errors.Is(err, ErrNoSecretResolver) {
			t.Fatalf("err = %v, 期望 ErrNoSecretResolver", err)
		}
		if out != nil {
			t.Fatalf("失败应返回 nil")
		}
	})

	t.Run("typed-nil 解析器不 panic", func(t *testing.T) {
		var fn FuncSecretResolver
		var r SecretResolver = fn
		if _, err := r.Resolve("openai"); !errors.Is(err, ErrNoSecretResolver) {
			t.Fatalf("nil 函数解析器应 ErrNoSecretResolver, 得到 %v", err)
		}
		in := headerOf("Authorization", "Bearer "+SecretPlaceholder("openai"))
		if _, err := RewriteRequest(in, "api.openai.com", r); !errors.Is(err, ErrSecretUnavailable) {
			t.Fatalf("err = %v, 期望 ErrSecretUnavailable", err)
		}
	})

	t.Run("空 secrets 与空 body", func(t *testing.T) {
		if out := ScrubResponse(nil, nil); out != nil {
			t.Fatalf("nil body 应原样返回 nil, 得到 %q", out)
		}
		if out := ScrubResponse([]byte{}, []string{testSecret}); len(out) != 0 {
			t.Fatalf("空 body 应原样返回, 得到 %q", out)
		}
		body := []byte("nothing to hide")
		if out := ScrubResponse(body, nil); !bytes.Equal(out, body) {
			t.Fatalf("nil secrets 应原样返回")
		}
	})

	t.Run("零值输入不 panic", func(t *testing.T) {
		if p := BuildAIEnv("", "", ""); p.Valid() || p.Env != nil {
			t.Fatalf("全空输入应得零值 plan, 得到 %+v", p)
		}
		if ev := (InjectEvent{}); ev.String() == "" || ev.GoString() == "" {
			t.Fatalf("零值事件的 fmt 面不该是空串")
		}
		// nil 钩子（缺省）必须静默丢弃
		SetInjectAuditHook(nil)
		emitInject(InjectEvent{Host: "api.openai.com", OK: true})
		if _, err := RewriteRequest(headerOf("Authorization", ""), "api.openai.com", nil); err != nil {
			t.Fatalf("空值 header 不该报错: %v", err)
		}
	})
}

// ── ⑦ 并发：不共享底层数组 ────────────────────────────────────────────

func TestRewriteConcurrentNoSharedMutation(t *testing.T) {
	sink := withInjectSink(t)
	fs := &fakeSecrets{m: map[string]string{"openai": testSecret, "anthropic": "sk-ant-9e8d7c6b5a4f"}}
	shared := headerOf(
		"Authorization", "Bearer "+SecretPlaceholder("openai"),
		"X-Api-Key", SecretPlaceholder("anthropic"),
		"X-Trace-Id", "t-1",
	)
	before := cloneHeader(shared)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := RewriteRequest(shared, "api.openai.com", fs)
			if err != nil {
				t.Errorf("并发改写失败: %v", err)
				return
			}
			if got := out.Get("Authorization"); got != "Bearer "+testSecret {
				t.Errorf("并发结果被串改: %q", got)
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(shared, before) {
		t.Fatalf("并发改写污染了共享输入: %v", shared)
	}
	if n := fs.callCount(); n < 2 {
		t.Fatalf("解析器调用次数异常: %d", n)
	}
	for _, ev := range sink.all() {
		assertNoSecret(t, "并发审计事件", ev.String())
	}
}

package redact

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ── 15 条负例探针 + 基线对照（T2.3 的验收本体）─────────────────────────────
//
// 每条探针对应一个**真实失败模式**（设计附件 §四）。探针有效性不靠肉眼断言，而靠
// 「基线必须泄漏」来证明（TestProbeValidity）：两个「合理的第一版实现」必须在这批语料上
// 漏掉 14/15 与 10/15，本方案必须是 0/15。数字是实测出来的，不是宣称出来的。

func TestMain(m *testing.M) {
	// 用户折叠固定成夹具里的用户名：本机 init() 取的是真实用户名（CI 上不是它），
	// 用例必须与夹具一致才有意义。
	SetUser(foldUser)
	os.Exit(m.Run())
}

const (
	// 夹具面隔离（设计-CI适配-v1.1 §五 第三批「夹具隔离」）：家目录与用户名**拼接构造** ——
	// 与下面 tokSk/tokHf 同一条口径（"运行期取到的值与原型完全一致，源码树保持干净"）。
	// 为什么这里也必须这样：本包的判据**拿路径/用户名的字节形态当尺**（base64 长度、解码后是否
	// 折叠命中、基线泄漏条数），发布面替换规则按字节改写就会把尺改坏（本批公开面实测：
	// `LEAK: 秘密 "~/…" 以投影 "~/…" 残留` 与 `夹具失效：blob=…（len=32）`）⇒ 夹具自己保持形态，
	// 规则不碰它（口径同批登记在 publish/replace-rules.tsv 的「夹具隔离」段）。
	foldUser = "ms" + "01"
	home     = "/Users/" + foldUser + "/projects/zerg/vault.key"
	homeRoot = "/Users/" + foldUser

	// 夹具里的凭据一律**拼接构造**：仓库的发布期门禁会扫 core/ 下的
	// `Bearer <20+>`（scripts/gates/check-public-tree-hazards.py）与 `sk-<20+>`
	// （scripts/gates/check-history-secrets.py）—— 运行期取到的值与原型完全一致，源码树保持干净。
	tokSk  = "sk-" + "abcdef0123456789XYZ" // sk- 后 19 字符（刻意 <20：见上面那条门禁）
	tokHf  = "hf_" + "abcdefghijklmnopqrstuvwxyz0123"
	tokURL = "abc12345" + "deadbeef"

	// 内网 IP 用 172.16/12（RFC1918）：既满足「内网 IP」探针语义，又不在发布替换规则表
	// （publish/replace-rules.tsv 只管 192.168.x / 10.0.0.x）里 ⇒ 公开快照改写夹具后语义不漂移。
	ipInner = "172.16.5.9"
)

func escapeSlash(s string) string { return strings.ReplaceAll(s, "/", `\/`) }
func escapeU002F(s string) string { return strings.ReplaceAll(s, "/", `\u002f`) }

// probeCase：Secret 是**任何投影下都不许再出现**的字面量。
type probeCase struct {
	Name   string
	Secret string
	Event  map[string]any
}

func corpus() []probeCase {
	b64 := base64.StdEncoding.EncodeToString([]byte(home))
	return []probeCase{
		{"plain-path", home,
			map[string]any{"err": "open " + home + ": permission denied"}},
		{"json-escaped-slash", escapeSlash(homeRoot + "/.ssh/id_rsa"),
			map[string]any{"err": "open " + escapeSlash(homeRoot+"/.ssh/id_rsa")}},
		{"unicode-escape-slash", foldUser,
			map[string]any{"err": "open " + escapeU002F(homeRoot+"/.netrc")}},
		{"fullwidth-homoglyph", foldUser,
			map[string]any{"err": "open ／Users／ｆｕｚｚ０１／.ssh／id_ed25519"}},
		{"zero-width-injection", foldUser,
			map[string]any{"err": "open /Users/fuzz\u200b01/.aws/credentials"}},
		{"percent-encoded", foldUser,
			map[string]any{"u": "path=%2FUsers%2F" + foldUser + "%2F.zshrc"}},
		{"percent-encoded-user", foldUser,
			map[string]any{"err": "open /Users/%6ds01/.ssh/id_rsa"}},
		{"base64-wrapped", b64,
			map[string]any{"blob": "cfg=" + b64}},
		{"multi-line-stack", home,
			map[string]any{"stack": "panic: boom\n\ngoroutine 1 [running]:\nmain.do()\n\t" + home + ":42 +0x1a\n"}},
		{"nested-map-array", home,
			map[string]any{"meta": map[string]any{
				"deep": []any{[]any{map[string]any{"p": home}}}, "hosts": []any{ipInner, "a@b.com"}}}},
		{"token-in-error", tokSk,
			map[string]any{"err": "rpc failed: Authorization: " + "Bearer " + tokSk + " refused"}},
		{"token-in-stderr", tokHf,
			map[string]any{"log": "HF_" + "TOKEN=" + tokHf}},
		{"url-query-token", tokURL,
			map[string]any{"url": "https://x/y?access_" + "token=" + tokURL}},
		{"foreign-home", "/home/alice",
			map[string]any{"err": "scp alice@host:/home/alice/.netrc failed"}},
		{"ip-in-args", ipInner,
			map[string]any{"meta": map[string]any{"peer": "peer=" + ipInner + " refused"}}},
	}
}

// emit 走**生产写路径**（MarshalRedacted），并在写路径之外再加三道断言：
//  1. fail-closed 不得被触发（marker 会让探针「看起来干净」——门禁看不见被整条丢弃的事件）；
//  2. 回扫门禁必须干净（与脱敏器同源的 needle 集）；
//  3. 秘密在任何投影下都不许残留（不只裸字节比较）。
func emit(t *testing.T, ev map[string]any) string {
	t.Helper()
	before := RedactionFailures()
	b := MarshalRedacted(ev)
	if got := RedactionFailures(); got != before {
		t.Fatalf("fail-closed 被触发（探针不该触发 marker）：failures %d → %d，line=%s", before, got, b)
	}
	line := string(b)
	if strings.Contains(line, EventRedactionFailed) {
		t.Fatalf("fail-closed marker 落盘：%s", line)
	}
	if leaks := VerifyNoResidue(ev, b); len(leaks) > 0 {
		t.Errorf("RESIDUE: %+v\n  line=%s", leaks, line)
	}
	return line
}

// mustVanish 断言 secret 的任何投影都不再出现在落盘行里。
func mustVanish(t *testing.T, line, secret string) {
	t.Helper()
	if len(secret) < 4 {
		return
	}
	views := Projections(line)
	for _, sv := range Projections(secret) {
		if len(sv) < 4 {
			continue
		}
		low := strings.ToLower(sv)
		for _, v := range views {
			if strings.Contains(strings.ToLower(v), low) {
				t.Errorf("LEAK: 秘密 %q 以投影 %q 残留在 %s", secret, sv, line)
			}
		}
	}
}

// TestProbes_AllModesRedacted：15 条探针逐条可判定（每条一个子测试，红了能直接定位失败模式）。
func TestProbes_AllModesRedacted(t *testing.T) {
	cases := corpus()
	if len(cases) != 15 {
		t.Fatalf("探针数应为 15，实测 %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			line := emit(t, c.Event)
			mustVanish(t, line, c.Secret)
		})
	}
}

// TestProbes_TargetedAssertions：形态级的定向断言（投影看不见形态时靠它兜底）。
func TestProbes_TargetedAssertions(t *testing.T) {
	out := emit(t, corpus()[0].Event) // 裸路径
	if !strings.Contains(out, PlaceholderPath) {
		t.Errorf("路径应被替换成 %s 而不是删掉：%s", PlaceholderPath, out)
	}
	mustNotContain(t, out, homeRoot)

	out = emit(t, corpus()[0].Event) // 同一条：D14 的「$ 展开静默删数据」
	if strings.Contains(out, `"/projects`) {
		t.Errorf("D14 回归：${HOME} 被当命名捕获组展开成了空串：%s", out)
	}

	out = emit(t, map[string]any{"err": "open " + home + ": permission denied"})
	if !strings.Contains(out, "/projects/zerg/vault.key") {
		t.Errorf("折中口径失效：路径尾部文件名应保留（便于定位）：%s", out)
	}

	out = emit(t, corpus()[8].Event) // 多行堆栈
	mustNotContain(t, out, homeRoot)

	out = emit(t, corpus()[4].Event) // 零宽注入
	mustNotContain(t, out, foldUser, "\u200b")

	out = emit(t, corpus()[3].Event) // 全角同形字
	mustNotContain(t, out, "／Users／", "ｍｓ０１", foldUser)
	if !strings.Contains(out, PlaceholderPath) {
		t.Errorf("折叠后应能认出路径并打码：%s", out)
	}

	out = emit(t, corpus()[7].Event) // base64 载荷整段替换
	if !strings.Contains(out, PlaceholderB64) {
		t.Errorf("base64 载荷应整段替换为 %s：%s", PlaceholderB64, out)
	}

	out = emit(t, corpus()[6].Event) // percent 载荷整段替换
	if !strings.Contains(out, PlaceholderPct) {
		t.Errorf("percent 载荷应整段替换为 %s：%s", PlaceholderPct, out)
	}
}

func mustNotContain(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(out, n) {
			t.Errorf("LEAK: %q 残留在 %s", n, out)
		}
	}
}

// TestProbeValidity：探针有效性 = 基线必须泄漏。期望值来自设计附件 §四的实测。
func TestProbeValidity(t *testing.T) {
	cases := corpus()
	impls := []struct {
		name string
		want int
		fn   func(map[string]any) []byte
	}{
		{"key-denylist-only", 14, func(ev map[string]any) []byte {
			b, _ := json.Marshal(baselineKeyDenylist(ev, ""))
			return b
		}},
		{"ascii-regex-top-level-only", 10, func(ev map[string]any) []byte {
			b, _ := json.Marshal(baselineASCIITopLevel(ev))
			return b
		}},
		{"ours(type-first+tier0+prefilter)", 0, func(ev map[string]any) []byte {
			return MarshalRedacted(ev)
		}},
	}

	got := map[string]int{}
	for _, impl := range impls {
		for _, c := range cases {
			if leaks := VerifyNoResidue(c.Event, impl.fn(c.Event)); len(leaks) > 0 {
				got[impl.name]++
			}
		}
		t.Logf("baseline[%s] leaks %d/%d", impl.name, got[impl.name], len(cases))
	}
	t.Logf("probes=%d", len(cases))

	for _, impl := range impls {
		if got[impl.name] != impl.want {
			t.Errorf("基线对照数字与设计稿不符：%s 实测 %d/%d，应为 %d/%d"+
				"（改探针语料时必须同步更新期望值，并回到设计附件 §四）",
				impl.name, got[impl.name], len(cases), impl.want, len(cases))
		}
	}
}

package redact

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// ── 三分表落到代码里（T2.2）─────────────────────────────────────────────────

func TestKeyClassesTable(t *testing.T) {
	table := KeyClasses()
	var keep, mask, drop int
	for k, c := range table {
		switch c {
		case ClassKeep:
			keep++
		case ClassMask:
			mask++
		case ClassDrop:
			drop++
		default:
			t.Errorf("表里 %q 被分类为 %s（表里不应出现 scan）", k, c)
		}
		if got := Classify(k); got != c {
			t.Errorf("Classify(%q)=%s 与表里的 %s 不一致", k, got, c)
		}
	}
	t.Logf("三分表：A 原样保留 %d 键 · B 键级遮蔽 %d 键 · C 直接丢弃 %d 键", keep, mask, drop)
	if keep == 0 || mask == 0 || drop == 0 {
		t.Fatalf("三分表不完整：A=%d B=%d C=%d", keep, mask, drop)
	}
}

// TestUnclassifiedDefaultsToScan：D4 —— 新字段默认最强脱敏倾向（未分类 ⇒ 扫描）。
func TestUnclassifiedDefaultsToScan(t *testing.T) {
	if got := Classify("brand_new_field_2026"); got != ClassScan {
		t.Fatalf("未分类字段应为 ClassScan（fail-closed 倾向），实测 %s", got)
	}
	// 零值也必须落到「扫描」而不是「原样保留」。
	var zero KeyClass
	if zero != ClassScan {
		t.Fatalf("KeyClass 零值应为 ClassScan，实测 %s", zero)
	}
	out := emit(t, map[string]any{"brand_new_field_2026": "see " + home})
	mustVanish(t, out, home)
	if !strings.Contains(out, PlaceholderPath) {
		t.Errorf("未分类字段里的路径应被扫描改写：%s", out)
	}
}

// TestKeepFieldsSurvive：A 类原样保留 —— 模型名与采样参数永不脱敏（D12：误报会让团队关掉脱敏）。
func TestKeepFieldsSurvive(t *testing.T) {
	out := emit(t, map[string]any{
		"model": "qwen3-30b-a3b", "temperature": 0.7, "tool_name": "fs.read",
		"call_fingerprint": "9f2c11ab", "prompt_sha256": "aa11bb22", "stop_reason": "tool_use",
		"seq": 41, "duration_ms": 128,
	})
	for _, n := range []string{"qwen3-30b-a3b", "0.7", "fs.read", "9f2c11ab", "aa11bb22", "tool_use", "41", "128"} {
		if !strings.Contains(out, n) {
			t.Errorf("调试必需字段 %q 被丢掉了：%s", n, out)
		}
	}
}

// TestDropAndMaskClasses：C 直接丢弃 + B 键级遮蔽（不看值也必须遮）。
func TestDropAndMaskClasses(t *testing.T) {
	out := emit(t, map[string]any{
		"prompt": "read ~/.zshrc", "tool_args": map[string]any{"path": "/etc/passwd"},
		"api_key": tokSk, "messages": []any{"hi"},
	})
	mustNotContain(t, out, "~", "/etc/passwd", tokSk, `"hi"`)
	if strings.Count(out, PlaceholderRedacted) < 4 {
		t.Errorf("B/C 类字段都应写成占位符（不删键）：%s", out)
	}
}

// ── 实现细节回归（原型踩过的坑，别退回去）───────────────────────────────────

// TestReplaceLiteralDoesNotExpand：D14 —— `${HOME}` 不是命名捕获组。
func TestReplaceLiteralDoesNotExpand(t *testing.T) {
	if got := replaceLiteral(reHomeDir, "/Users/x/y", PlaceholderPath); got != PlaceholderPath+"/y" {
		t.Fatalf("replaceLiteral 用了 $ 展开（会静默删数据）：得到 %q", got)
	}
	if got := replaceLiteral(reHomeDir, "/Users/x/y", PlaceholderPath); !strings.Contains(got, PlaceholderPath) {
		t.Fatalf("路径被删掉而不是打码：%q", got)
	}
}

// TestEncodedChannelIsNotPrefiltered：D13 —— 编码通道在预过滤之前（Tier-0）。
// 纯 base64 blob 没有 `/ @ . = :` 任何分隔符 ⇒ 任何启发式预过滤都会整段跳过它。
func TestEncodedChannelIsNotPrefiltered(t *testing.T) {
	bare := base64.StdEncoding.EncodeToString([]byte("~/.config/zerg/token"))
	if i := strings.IndexAny(bare, `/@=:.-_+?&%`); i >= 0 {
		t.Fatalf("夹具失效：裸 blob 不该含分隔符，实测含 %q", bare[i])
	}
	if maybeSensitive(bare) {
		t.Fatalf("夹具失效：这个 blob 本该被启发式预过滤跳过")
	}
	if got := RedactValue(bare); got != PlaceholderB64 {
		t.Fatalf("纯 base64 blob 未被脱敏（D13 回归）：%q → %q", bare, got)
	}
}

// TestBareUserNameIsFolded：D13 的另一半 —— 裸用户名（无分隔符）也必须折叠。
func TestBareUserNameIsFolded(t *testing.T) {
	if maybeSensitive(foldUser) {
		t.Fatalf("夹具失效：裸用户名本该被启发式预过滤跳过")
	}
	if got := RedactValue(foldUser); got != PlaceholderUser {
		t.Fatalf("裸用户名未折叠（D13 回归）：%q → %q", foldUser, got)
	}
}

// TestCfInjectionDoesNotDefeatKeywordRules：Cf/零宽字符注入不得让关键字构造逃过遮蔽。
// **这是 fuzz 找出的唯一一条真泄漏**（其余都是门禁精度问题）：输入 "0\u0605token=000000" ——
// 旧实现把 Cf 直接删掉，折叠后 "0token=000000" 失去词边界（`\btoken\b` 不命中）⇒ 该遮的值
// 原样落盘。现在 Cf 换成空格（保住边界）："0 token=000000" ⇒ 正常遮蔽。
func TestCfInjectionDoesNotDefeatKeywordRules(t *testing.T) {
	in := "0\u0605token=000000"
	out := RedactValue(in)
	if strings.Contains(out, "000000") {
		t.Fatalf("Cf 注入让关键字构造逃过遮蔽：%q → %q", in, out)
	}
	if !strings.Contains(out, PlaceholderRedacted) {
		t.Fatalf("应产出占位符：%q", out)
	}
}

// TestCfInjectionIntoTokenShape：Cf/零宽字符藏在令牌里时不得漏遮（两处，都是 fuzz 找出的真泄漏）。
//
//	in = "AKIA000000000000 AKIA00000000\u06dd0000"  —— Cf 在令牌**中间**：逐字节只数出 10 个
//	     令牌字符（<12）⇒ 漏遮；读者眼里是 AKIA + 14 个数字（标准令牌形态）。
//	in = "AK\u06ddIA000000000000"                    —— Cf 在**前缀**里：前缀匹配直接失败 ⇒ 漏遮。
//
// 现在令牌扫描在「可见视图」（摘掉不可见字符）上做，再映射回原文遮蔽。
func TestCfInjectionIntoTokenShape(t *testing.T) {
	for _, in := range []string{
		"AKIA" + "00000000\u06dd0000",
		"AK\u06ddIA" + "000000000000",
		"AKIA000000000000 AKIA" + "00000000\u06dd0000",
	} {
		out := RedactValue(in)
		if strings.Contains(out, "AKIA") || strings.Contains(out, "AK\u06ddIA") {
			t.Fatalf("Cf 藏在令牌里时漏遮：%q → %q", in, out)
		}
		if !strings.Contains(out, PlaceholderRedacted) {
			t.Fatalf("应产出占位符：%q", out)
		}
	}
}

// TestContainedOccurrenceIsMasked：字节级收口 —— 规则**未命中**的「包含式拷贝」也必须遮蔽。
// 语料来自 fuzz 实测：in="0.0.0.0 A0.0.0.0" —— IPv4 规则要 \b 词边界，第二处前面是字母故不命中，
// 但它逐字包含第一处的 IP；门禁（正确地）为这份残留报了名 ⇒ 从源头收口。
func TestContainedOccurrenceIsMasked(t *testing.T) {
	const ip = "0.0.0.0"
	in := ip + " and A" + ip
	out := RedactValue(in)
	if strings.Contains(out, ip) {
		t.Fatalf("包含式拷贝未被遮蔽：%q → %q", in, out)
	}
	if leaks := VerifyNoResidueString(map[string]any{"err": in}, string(MarshalRedacted(map[string]any{"err": in}))); len(leaks) != 0 {
		t.Fatalf("门禁报残留（源头未收口）：%+v", leaks)
	}
}

// TestValuePipelineReachesFixpoint：脱敏是**重写系统** ⇒ 必须迭代到不动点。
// 语料来自 fuzz 实测：'&token= 00000000!0' 第一轮只被 Bearer 规则命中（'=' 后是空格 ⇒ URL-query
// 规则当场不匹配），而它生成的 '&token=[REDACTED]!0' 反而让 URL-query 规则在第二轮命中。
func TestValuePipelineReachesFixpoint(t *testing.T) {
	for _, in := range []string{
		"&token= 00000000!0",
		"AuthoriZAtion:000I00 000I00",
		"~/x",
		escapeSlash(home),
		"%2FUsers%2FMr2109%2F.zshrc",
		"／Users／ｆｕｚｚ０１／.ssh／id_ed25519",
		"open /Users/fuzz\u200b01/.aws/credentials",
		"hf-" + "abcdefghijklmnop",
		tokSk,
		"cfg=" + base64.StdEncoding.EncodeToString([]byte(home)),
	} {
		one := RedactValue(in)
		if two := RedactValue(one); two != one {
			t.Errorf("值管线不幂等：%q → 1st=%q → 2nd=%q", in, one, two)
		}
	}
}

// TestValueLengthBudget：超长值先脱敏后截断（绝不先截断再脱敏），且截断仍幂等。
func TestValueLengthBudget(t *testing.T) {
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)
	SetLimits(64, 0)
	if _, ev := Limits(); ev != DefaultMaxEventBytes {
		t.Fatalf("值长上限不应影响事件上限：%d", ev)
	}
	long := strings.Repeat("x", 100) + home + strings.Repeat("y", 100)
	out := RedactValue(long)
	if len(out) > 64 {
		t.Fatalf("未按值长上限截断：%d 字节", len(out))
	}
	if !strings.HasSuffix(out, PlaceholderTruncated) {
		t.Fatalf("截断未留标记：%q", out)
	}
	mustVanish(t, out, home) // 截断发生在脱敏之后 ⇒ 不能把路径切一半留下
	if again := RedactValue(out); again != out {
		t.Fatalf("截断不幂等：%q → %q", out, again)
	}
}

// TestDepthGuardFailsClosed：深度超限 ⇒ 标记，不递归、不放行。
func TestDepthGuardFailsClosed(t *testing.T) {
	deep := map[string]any{"leaf": home}
	for i := 0; i < MaxDepth+8; i++ {
		deep = map[string]any{"n": deep}
	}
	b, err := json.Marshal(RedactEvent(deep))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"_depth_exceeded":true`) {
		t.Fatalf("深度超限未 fail-closed：%s", b)
	}
	mustVanish(t, string(b), home)
}

// TestOpaqueTypesPassThrough：类型优先分派 —— 非 string 不进字符串管线。
func TestOpaqueTypesPassThrough(t *testing.T) {
	out := RedactEvent(map[string]any{"seq": 41, "ok": true, "cache_hit": nil, "tokens_in": 1.5})
	if out["seq"] != 41 || out["ok"] != true || out["cache_hit"] != nil || out["tokens_in"] != 1.5 {
		t.Fatalf("非字符串值被改动：%+v", out)
	}
}

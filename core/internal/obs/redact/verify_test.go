package redact

import (
	"encoding/base64"
	"strings"
	"testing"
)

// ── 回扫门禁的可判定性（T2.5 的一部分；门禁本体在 verify.go）─────────────────

// TestProjectionsCoverFiveClasses：五类投影各自能还原出原文（D19）。
func TestProjectionsCoverFiveClasses(t *testing.T) {
	forms := map[string]string{
		"raw":            home,
		"json-escape":    escapeSlash(home),
		"unicode-escape": escapeU002F(home),
		"fullwidth":      foldWidthEvery(home),
		"percent-encode": strings.ReplaceAll(home, "/", "%2F"),
		"base64":         base64.StdEncoding.EncodeToString([]byte(home)),
		"zero-width":     strings.ReplaceAll(home, "Mr2109", "ms\u200b01"),
	}
	for name, form := range forms {
		views := Projections(form)
		found := false
		for _, v := range views {
			if strings.Contains(v, home) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("投影 %s 未能还原出原文（门禁会漏检）：views=%q", name, views)
		}
	}
}

// TestGateRejectsEscapedForms：只比裸字节会给出**虚假保证**（D19 实测：四种藏法全部漏检）。
func TestGateRejectsEscapedForms(t *testing.T) {
	forms := map[string]string{
		"json-escaped-slash": escapeSlash(home),
		"unicode-escape":     escapeU002F(home),
		"fullwidth":          foldWidthEvery(home),
		"base64":             base64.StdEncoding.EncodeToString([]byte(home)),
		"percent":            strings.ReplaceAll(home, "/", "%2F"),
	}
	for name, form := range forms {
		line := `{"x":"` + form + `"}`
		if strings.Contains(line, home) {
			t.Fatalf("夹具失效（%s）：裸字节比较本该漏检", name)
		}
		if leaks := VerifyNoResidueString(map[string]any{"err": home}, line); len(leaks) == 0 {
			t.Errorf("门禁漏检以 %s 形态藏起来的原文：%s", name, line)
		}
	}
}

// TestGateCatchesReintroducedOriginal：D20 —— 上游把原文重新拼回一条干净行。
func TestGateCatchesReintroducedOriginal(t *testing.T) {
	orig := corpus()[0].Event
	clean := MarshalRedacted(orig)
	if leaks := VerifyNoResidue(orig, clean); len(leaks) != 0 {
		t.Fatalf("干净行被误判：%+v", leaks)
	}
	poisoned := append([]byte{}, clean[:len(clean)-1]...)
	poisoned = append(poisoned, []byte(`,"debug_original":"open `+home+`: permission denied"}`)...)
	if leaks := VerifyNoResidue(orig, poisoned); len(leaks) == 0 {
		t.Error("门禁抓不到「上游重新拼回原文」⇒ 门禁失效（D20）")
	}
}

// TestGateNoFalsePositiveOnPlainSlashes：D18 —— 误报（把 ////// 当 base64 载荷）会让门禁被关掉。
func TestGateNoFalsePositiveOnPlainSlashes(t *testing.T) {
	ev := map[string]any{"err": "path ////// not found"}
	line := MarshalRedacted(ev)
	if leaks := VerifyNoResidue(ev, line); len(leaks) != 0 {
		t.Fatalf("门禁误报（needle 集与脱敏器不同源）：%+v line=%s", leaks, line)
	}
}

// TestGateSharesPolicyOnKeepFields：门禁与脱敏器**同策略** —— A 类字段里的敏感形态不是泄漏
// （脱敏器按设计保留它；门禁若指控它，就会逼着人把模型名/参数也遮掉，最后整条规则被关掉）。
func TestGateSharesPolicyOnKeepFields(t *testing.T) {
	ev := map[string]any{"model": "qwen3-30b-a3b", "tool_name": "fs.read", "seq": 41}
	line := MarshalRedacted(ev)
	if leaks := VerifyNoResidue(ev, line); len(leaks) != 0 {
		t.Fatalf("A 类字段被误判成泄漏：%+v", leaks)
	}
	// 反向控制：同一个值放在 ClassDrop 字段上就必须被遮（否则这条断言没有区分力）。
	dropped := MarshalRedacted(map[string]any{"output": "qwen3-30b-a3b"})
	if strings.Contains(string(dropped), "qwen3-30b-a3b") {
		t.Fatalf("ClassDrop 未丢弃：%s", dropped)
	}
}

// TestGateNotFooledByJSONEscapeArtifacts：JSON 转义序列本身可能与被转义字符的邻居「拼出」needle
// 的字节（fuzz 实测：输入 "\x10.0.0.0 0.0.0.0" ⇒ 行里 `\u0010` 的尾零 + 紧随的 ".0.0.0" 拼成
// "0.0.0.0"）。那不是秘密 ⇒ 输出侧比较必须先还原转义，否则门禁自己制造假 leak（D18）。
func TestGateNotFooledByJSONEscapeArtifacts(t *testing.T) {
	in := "\x10" + ".0.0.0 0.0.0.0"
	ev := map[string]any{"err": in}
	line := MarshalRedacted(ev)
	if !strings.Contains(string(line), `\u0010`) {
		t.Fatalf("夹具失效：控制字符应被 encoding/json 转义：%s", line)
	}
	if leaks := VerifyNoResidue(ev, line); len(leaks) != 0 {
		t.Fatalf("JSON 转义拼接造成的假 leak：%+v line=%s", leaks, line)
	}
}

// TestGateNoCollapsedNeedleProjection：needle 的投影若塌缩成它自己的**真子串**，不参与比较。
// fuzz 实测：in="&token=\u0605"（值是单个 Cf 字符）⇒ foldWidth 把它删掉后 needle 投影只剩关键字
// 前缀 "&token="，而那正是脱敏器**故意保留**的东西（URL 规则只替换值、保留关键字）⇒ 假 leak。
func TestGateNoCollapsedNeedleProjection(t *testing.T) {
	ev := map[string]any{"err": "&token=\u0605"}
	line := MarshalRedacted(ev)
	if leaks := VerifyNoResidue(ev, line); len(leaks) != 0 {
		t.Fatalf("投影塌缩造成的假 leak：%+v line=%s", leaks, line)
	}
}

// TestGateCaseFoldOnlyForCaseInsensitiveRules：折叠口径必须与**规则自身的大小写语义**一致。
// 只有用户折叠是大小写无关的（reUser 带 (?i)）；路径/邮箱/令牌/关键字 construct 都是大小写敏感的
// 规则 ⇒ 它们的 needle 一律按字节比较，否则必然误报（fuzz 实测两例：
// in="/hoMe/0/home/0" ⇒ needle "/home/0" 命中普通文本 "/hoMe/0"；
// in="/home/AAA/hoMe/AAA0" ⇒ needle "/home/AAA" 命中未命中的 "/hoMe/AAA0"）。
func TestGateCaseFoldOnlyForCaseInsensitiveRules(t *testing.T) {
	for _, in := range []string{"/hoMe/0" + "/home/0", "/home/AAA/hoMe/AAA0"} {
		ev := map[string]any{"err": in}
		if leaks := VerifyNoResidue(ev, MarshalRedacted(ev)); len(leaks) != 0 {
			t.Fatalf("大小写敏感规则的 needle 被折叠误报：in=%q %+v", in, leaks)
		}
	}
	// 反向控制：用户折叠（(?i)）的 needle 必须做折叠比较 —— 大写重发能落盘就说明脱敏漏了。
	if leaks := VerifyNoResidueString(map[string]any{"err": "~/x"}, `{"err":"HOME OF MS01"}`); len(leaks) == 0 {
		t.Error("用户折叠 needle 未做折叠比较（大写重发漏检）")
	}
}

// TestKeywordValueMaskedEverywhere：关键字类规则只认得出「带上下文」的那一处，所以值的**第二份
// 拷贝**必须一并遮蔽（fuzz 实测命中："AuthoriZAtion:000I00 000I00" 的尾部那份原样留下 —— 那是真
// 泄漏：读日志的人照样拿到凭据）。反向控制：值是裸关键字本身时不外溢（过遮与漏脱敏同样是伤害）。
func TestKeywordValueMaskedEverywhere(t *testing.T) {
	const dup = "000I00"
	in := "AuthoriZAtion:" + dup + " then again " + dup
	out := RedactValue(in)
	if strings.Contains(out, dup) {
		t.Fatalf("值的第二份拷贝未被遮蔽：%q → %q", in, out)
	}
	if !strings.Contains(out, PlaceholderRedacted) {
		t.Fatalf("整条 construct 应被遮蔽：%q", out)
	}
	line := MarshalRedacted(map[string]any{"err": in})
	if leaks := VerifyNoResidueString(map[string]any{"err": in}, string(line)); len(leaks) != 0 {
		t.Fatalf("门禁对同值拷贝报残留：%+v line=%s", leaks, line)
	}
	if again := RedactValue(out); again != out {
		t.Fatalf("外溢遮蔽后不幂等：%q → %q", out, again)
	}

	// 反向控制（直接测外溢规则本身，避免被关键字规则的替换掩盖）：
	if got := maskValuesEverywhere("Bearer and token here", []string{"Bearer"}); got != "Bearer and token here" {
		t.Fatalf("裸关键字值不应外溢：%q", got)
	}
	if got := maskValuesEverywhere("x deadbeef123456 x", []string{"deadbeef123456"}); !strings.Contains(got, PlaceholderRedacted) {
		t.Fatalf("非关键字值应外溢遮蔽：%q", got)
	}
}

// TestGateNoFalsePositiveOnLowEntropyNeedle：D18 的另一面 —— 低熵 needle（单字节重复）必然误报。
// 语料来自 fuzz 实测（本包第一轮 20s 就命中）：脱敏器确实遮掉了 Bearer 值，但同一字符串里另有
// 一段更长的 0 串（普通内容），拿捕获值比子串就会假 leak。误报 ⇒ 门禁被关掉 ⇒ 真失效。
// 两层防线：① 值的其余拷贝一律遮蔽（源头）；② 单字节重复的 needle 不进 needle 集（精度）。
func TestGateNoFalsePositiveOnLowEntropyNeedle(t *testing.T) {
	ev := map[string]any{"err": "AuthoriZAtion:0000000 " + strings.Repeat("0", 28)}
	line := MarshalRedacted(ev)
	if !strings.Contains(string(line), PlaceholderRedacted) {
		t.Fatalf("Bearer 值应被遮蔽：%s", line)
	}
	if leaks := VerifyNoResidue(ev, line); len(leaks) != 0 {
		t.Fatalf("低熵 needle 误报（门禁会被关掉）：%+v line=%s", leaks, line)
	}
	// 反向控制：有区分度的 Bearer 值必须照样被抓（否则上面那条断言没有区分力，
	// 等于把门禁悄悄削弱成「什么都不报」）。
	distinctEv := map[string]any{"err": "AuthoriZAtion:token1234567890"}
	if leaks := VerifyNoResidueString(distinctEv, string(MarshalRedacted(distinctEv))); len(leaks) != 0 {
		t.Fatalf("干净行被误判：%+v", leaks)
	}
	poisoned := `{"err":"AuthoriZAtion:token1234567890"}`
	if leaks := VerifyNoResidueString(distinctEv, poisoned); len(leaks) == 0 {
		t.Error("有区分度的 Bearer 值漏检（门禁被过度削弱的证据）")
	}
}

// foldWidthEvery 把整个 ASCII 串转成全角（整串同形字攻击，不只是前缀）。
func foldWidthEvery(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x21 && r <= 0x7E {
			return r + 0xFEE0
		}
		return r
	}, s)
}

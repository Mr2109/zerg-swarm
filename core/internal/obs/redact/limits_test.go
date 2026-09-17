package redact

import (
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ── T2.7：性能与限额（类型门控 + 值长上限 + 分级）───────────────────────────
//
// 本文件是任务表 T2.7 的验收本体，四块：
//  ① 分级/门控**可判定**（用计数器断言「安全枚举字段零扫描」，不用耗时断言 —— 耗时在 CI 上抖动）；
//  ② 值长上限：超长截断 + 标记 + **落盘行仍可 json.Unmarshal**（截断不许破坏结构）；
//  ③ 回归：fuzz 语料整批重放 + 预过滤仍不漏 base64（D13）；
//  ④ 守扠：每条必要条件门必须真是必要条件（regexp 命中 ⇒ 门为真），且门本身有区分力。

// ── ① 分级与类型门控（计数器断言）───────────────────────────────────────────

// TestKeepsAreZeroScan：A 类标量零扫描 —— 一个字节都不进值管线（设计附件 §三 措施③ + §五 字段三分表）。
func TestKeepsAreZeroScan(t *testing.T) {
	ev := map[string]any{
		"model": "qwen3-30b-a3b", "temperature": 0.7, "seq": 41, "ok": true, "cache_hit": nil,
		"tool_name": "fs.read", "prompt_sha256": "aa11bb22", "stop_reason": "tool_use",
	}
	ResetPerfCounters()
	out := RedactEvent(ev)
	if got := PipelineEntries(); got != 0 {
		t.Fatalf("A 类标量/非字符串值进了值管线：PipelineEntries=%d（应为 0）", got)
	}
	if got, want := KeepScalarPasses(), uint64(len(ev)); got != want {
		t.Errorf("零扫描通过计数：实测 %d，应为 %d", got, want)
	}
	for k, v := range ev {
		if out[k] != v {
			t.Errorf("A 类字段 %q 被改动了：%v → %v（应逐字保留）", k, v, out[k])
		}
	}

	// 反向控制（没有区分力的断言就是假绿）：一个**未分类**字段必须进管线。
	ResetPerfCounters()
	RedactEvent(map[string]any{"brand_new_field_2026": "see " + home})
	if got := PipelineEntries(); got != 1 {
		t.Fatalf("未分类字段未进值管线：PipelineEntries=%d（应为 1）", got)
	}

	// 分级第 ① 层：C 丢弃 / B 键级遮蔽**连值都不看** ⇒ 不进管线（也不该触发截断计数）。
	ResetPerfCounters()
	RedactEvent(map[string]any{"prompt": home, "api_key": tokSk, "messages": []any{"hi"}, "output": "x"})
	if got := PipelineEntries(); got != 0 {
		t.Errorf("ClassDrop/ClassMask 的值进了管线：PipelineEntries=%d（应为 0）", got)
	}
}

// TestKeepContainersStillRecurse：A 类键上挂容器时**仍然递归** ——
// 否则 `"model": {"p": "/Users/…"}` 就是一条绕过脱敏的后门（D15 的 nested 形状）。
func TestKeepContainersStillRecurse(t *testing.T) {
	ResetPerfCounters()
	out := RedactEvent(map[string]any{"model": map[string]any{"p": home}, "rows": []any{map[string]any{"note": home}}})
	if got := PipelineEntries(); got != 2 {
		t.Fatalf("A 类下的容器未被递归：PipelineEntries=%d（应为 2）", got)
	}
	line, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mustVanish(t, string(line), home)
}

// TestValueLimitDefaultsInDesignRange：默认值长上限落在设计口径的 2–4KB（对齐 OTTL truncate_all）。
func TestValueLimitDefaultsInDesignRange(t *testing.T) {
	v, e := Limits()
	if v < 2048 || v > 4096 {
		t.Errorf("默认值长上限应在 2–4KB（设计附件 §三 措施②），实测 %d", v)
	}
	if e != DefaultMaxEventBytes {
		t.Errorf("默认事件上限应为 %d，实测 %d", DefaultMaxEventBytes, e)
	}
}

// ── ② 值长上限：截断不许破坏结构 ────────────────────────────────────────────

// longFreeText 造一段超长自由文本（含分隔符 ⇒ 真的会走完整条管线，不是被预过滤跳过的空串）。
func longFreeText(n int) string { return strings.Repeat("read /tmp/f0 ok; dur=12ms; ", n) }

// TestTruncationKeepsLineParseable：**截断后落盘行仍能被 json.Unmarshal** ——
// 值长上限的前提是「结构不许坏」：截断只截值的内容，键、类型、括号都由 encoding/json 负责。
func TestTruncationKeepsLineParseable(t *testing.T) {
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)
	SetLimits(512, 0)

	ev := map[string]any{
		"err":   home + " " + longFreeText(4000) + " " + tokSk, // 头/尾都埋了秘密，中间是 100KB 自由文本
		"note":  strings.Repeat("x", 4000),                     // 未分类字段同样受上限约束
		"model": strings.Repeat("m", 4000),                     // A 类也受长度上限约束（O(1) 检查，不是扫描）
		"seq":   41,
		"rows":  []any{strings.Repeat("y", 4000)},
	}

	before := RedactionFailures()
	line := MarshalRedacted(ev)
	if got := RedactionFailures(); got != before {
		t.Fatalf("超长值触发了 fail-closed（应被截断而不是丢整条事件）：failures %d → %d", before, got)
	}
	if strings.Contains(string(line), EventRedactionFailed) {
		t.Fatalf("fail-closed marker 落盘（超长值不该走到这一步）：%s", line)
	}

	var back map[string]any
	if err := json.Unmarshal(line, &back); err != nil {
		t.Fatalf("截断后落盘行不再是合法 JSON（结构被破坏）：%v", err)
	}
	if len(back) != len(ev) {
		t.Fatalf("键数量变了（截断不许改结构）：%d → %d", len(ev), len(back))
	}
	if _, ok := back["seq"].(float64); !ok { // 数字类型没漂
		t.Errorf("数值字段类型被截断影响：%#v", back["seq"])
	}
	if _, ok := back["rows"].([]any); !ok {
		t.Errorf("数组字段结构被截断破坏：%#v", back["rows"])
	}
	for _, k := range []string{"err", "note", "model"} {
		s, _ := back[k].(string)
		if len(s) > 512 {
			t.Errorf("%s 未被值长上限约束：%d 字节", k, len(s))
		}
		if !strings.HasSuffix(s, PlaceholderTruncated) {
			t.Errorf("%s 截断未标记：%q", k, s)
		}
	}
	// 截断发生在**脱敏之后** ⇒ 头部的秘密已被遮蔽；尾部的秘密被截断切掉（两条都不泄漏）。
	mustVanish(t, string(line), home)
	mustVanish(t, string(line), tokSk)
	if leaks := VerifyNoResidue(ev, line); len(leaks) > 0 {
		t.Errorf("截断后门禁报残留：%+v", leaks)
	}
	if got := ValueTruncations(); got < 3 {
		t.Errorf("截断计数：实测 %d，至少应有 3 个值被截断", got)
	}
}

// TestTruncationExtremeLimitStillMarked：上限小到装不下标记自身时，**以标记为准**（宁可超上限，
// 也不能出现「被截断但没有痕迹」的值 —— 「缺席」必须可判定，与 B/C 类写占位符同一个道理）。
func TestTruncationExtremeLimitStillMarked(t *testing.T) {
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)
	SetLimits(8, 0) // < len(PlaceholderTruncated)

	out := RedactValue(strings.Repeat("x", 200))
	if out != PlaceholderTruncated {
		t.Fatalf("上限装不下标记时应只留标记：%q", out)
	}
	if again := RedactValue(out); again != out {
		t.Fatalf("极端上限下不幂等：%q → %q", out, again)
	}
	line := MarshalRedacted(map[string]any{"err": strings.Repeat("x", 200)})
	var back map[string]any
	if err := json.Unmarshal(line, &back); err != nil {
		t.Fatalf("极端上限下结构被破坏：%v", err)
	}
	if s, _ := back["err"].(string); s != PlaceholderTruncated {
		t.Fatalf("落盘值不是标记：%q", s)
	}
}

// TestTruncationDoesNotSaveRedactionCost：如实记录一个**否定性结论** ——
// 截断省的是落盘体积，不是 CPU：脱敏必须先看全文（先截断再脱敏会把秘密切成半截、同时躲开模式匹配），
// 故 64KB 值的脱敏成本照付。这条用例把「截断 ≠ 性能优化」钉在代码里，
// 免得日后有人把「省钱顺序」里的截断理解成"大值反正会被截断所以随便喂"。
func TestTruncationDoesNotSaveRedactionCost(t *testing.T) {
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)
	SetLimits(1024, 0)
	big := longFreeText(4000) // ≈100KB
	out := RedactValue(big)
	if len(out) > 1024 {
		t.Fatalf("未截断：%d 字节", len(out))
	}
	if got := PipelineEntries(); got == 0 {
		t.Fatal("大值未进管线（夹具失效）")
	}
	if !strings.Contains(out, "read /tmp/f0 ok") {
		t.Fatalf("截断保留了头部内容（前缀口径）：%q", out[:64])
	}
}

// ── ③ 回归：fuzz 语料整批重放 + 预过滤仍不漏 base64 ─────────────────────────

// loadFuzzCorpus 读 testdata/fuzz/FuzzRedactValue/ 下的语料（格式：`go test fuzz v1` + `string("…")`）。
func loadFuzzCorpus(t *testing.T, dir string) []string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 fuzz 语料目录失败：%v", err)
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("读语料 %s 失败：%v", f.Name(), err)
		}
		txt := string(b)
		i := strings.Index(txt, "string(")
		if i < 0 {
			t.Fatalf("语料 %s 不是 string(…) 形态：%q", f.Name(), txt)
		}
		lit := strings.TrimSpace(txt[i+len("string("):])
		lit = strings.TrimSuffix(lit, ")")
		s, err := strconv.Unquote(strings.TrimSpace(lit))
		if err != nil {
			t.Fatalf("语料 %s 解引用失败：%v（%q）", f.Name(), err, lit)
		}
		out = append(out, s)
	}
	return out
}

// TestFuzzCorpusReplayNoRegression：把 T2.1–T2.3 锁下来的语料**整批重放**（18 条）——
// 每条重跑 fuzz 的四条性质：不 panic · 幂等 · 落盘行合法 UTF-8 + 可解析 · 门禁同源（干净行不报 leak）。
// 语料是「已经踩过的坑」的化石：它们红了就说明有人把老坑挖回来了。
func TestFuzzCorpusReplayNoRegression(t *testing.T) {
	corpus := loadFuzzCorpus(t, filepath.Join("testdata", "fuzz", "FuzzRedactValue"))
	if len(corpus) < 18 {
		t.Fatalf("fuzz 语料少于 18 条（T2.1–T2.3 锁了 18 条）：实测 %d", len(corpus))
	}
	for _, s := range corpus {
		ev := map[string]any{"err": s}
		line := MarshalRedacted(ev)
		if strings.Contains(string(line), EventRedactionFailed) {
			t.Errorf("语料触发 fail-closed（marker 会让残留看不见）：in=%q", s)
		}
		var back map[string]any
		if err := json.Unmarshal(line, &back); err != nil {
			t.Errorf("落盘行不可解析：in=%q err=%v", s, err)
		}
		out := RedactValue(s)
		if again := RedactValue(out); again != out {
			t.Errorf("幂等回归：in=%q 1st=%q 2nd=%q", s, out, again)
		}
		if leaks := VerifyNoResidue(ev, line); len(leaks) > 0 {
			t.Errorf("门禁同源回归：in=%q leaks=%+v", s, leaks)
		}
	}
	t.Logf("fuzz 语料重放：%d 条，全部满足四条性质", len(corpus))
}

// TestPreFilterStillMissesNothingBase64：D13 回归 —— 纯 base64 blob 没有 `/ @ . = :` 任何分隔符，
// 任何启发式预过滤都会整段跳过它，故编码通道必须在 Tier-0（永不被预过滤跳过）。
// 这里把四种编码 × 四类载荷 × 裸 blob / 散文包裹各来一遍，并顺带验证**大值**上也不漏。
func TestPreFilterStillMissesNothingBase64(t *testing.T) {
	// 载荷都得够长：编码后的候选要 ≥24 字节才进 Tier-0 的 base64 候选集（reB64Run 的最小长度是
	// **必要条件**，不是启发式；短于它的 blob 与普通 8~10 字节单词无法区分，见文末「短载荷边界」）。
	payloads := map[string]string{
		"path":     home,                                  // ~/projects/zerg/vault.key
		"homepath": "~/.config/zerg/token",      // 路径 + 用户名折叠的双载荷
		"token":    tokHf,                                 // hf_ 形态令牌
		"email":    "contact.ops.team@zerg.internal",      // 邮箱
		"ip":       "peer 172.16.5.9 refused, retry=0 ok", // 内网 IP + 普通文本
	}
	encs := map[string]*base64.Encoding{
		"std": base64.StdEncoding, "rawstd": base64.RawStdEncoding,
		"url": base64.URLEncoding, "rawurl": base64.RawURLEncoding,
	}
	for pname, payload := range payloads {
		for ename, enc := range encs {
			blob := enc.EncodeToString([]byte(payload))
			if strings.ContainsAny(blob, `/@=:.-_+?&%`) {
				continue // 这个编码形态自带分隔符，不构成「被预过滤跳过」的夹具
			}
			if maybeSensitive(blob) {
				t.Fatalf("夹具失效（%s/%s）：纯 blob 本该被启发式预过滤跳过：%q", pname, ename, blob)
			}
			for _, form := range []string{blob, "cfg=" + blob, "log " + blob + " tail", "prefix " + blob} {
				if got := RedactValue(form); !strings.Contains(got, PlaceholderB64) {
					t.Errorf("base64 载荷未被整段遮蔽（D13 回归）：%s/%s in=%q out=%q", pname, ename, form, got)
				}
			}
		}
	}

	// 已知边界（如实登记，不是回归）：候选下限 24 字节 ⇒ 更短的 blob 不解码。
	// 代价是一条 6 字节 blob（"bXMwMQ" = "Mr2109"）能藏下 4 个字符的秘密；收益是避免把
	// 普通短标识（"deadbeef"、"aaaaaaaa"）整段遮掉（D12：过度遮蔽与漏脱敏是同一种伤害）。
	// 这条断言存在的意义：谁将来改了这个下限，用例会红，改的人必须回来看这段注释与设计附件 §六 第 3 步。
	if got := RedactValue("bXMwMQ"); got != "bXMwMQ" {
		t.Errorf("短 blob（<24 字节）的下限口径变了：%q → %q（请同步设计附件 §六 第 3 步与门槛理由）", "bXMwMQ", got)
	}

	// 大值 + 截断的交互：无论载荷在头部（截断切不到）还是尾部（会被切掉），都不许露出 base64 原文。
	defer SetLimits(DefaultMaxValueBytes, DefaultMaxEventBytes)
	blob := base64.StdEncoding.EncodeToString([]byte(home))
	head := blob + " " + longFreeText(4000)
	tail := longFreeText(4000) + " " + blob
	for _, in := range []string{head, tail} {
		out := RedactValue(in)
		if strings.Contains(out, blob) {
			t.Fatalf("大值上的 base64 载荷漏遮（预过滤/截断交互回归）：len(in)=%d", len(in))
		}
		if len(out) > DefaultMaxValueBytes {
			t.Fatalf("大值未被值长上限约束：%d 字节", len(out))
		}
		line := MarshalRedacted(map[string]any{"err": in})
		var back map[string]any
		if err := json.Unmarshal(line, &back); err != nil {
			t.Fatalf("大值落盘行不可解析：%v", err)
		}
		if s, _ := back["err"].(string); strings.Contains(s, blob) {
			t.Fatalf("大值落盘行里仍有 base64 原文")
		}
	}
}

// ── ④ 守扠：每道门必须真是必要条件（T2.7 新增的六道门的牙齿）────────────────

// TestGatesAreNecessaryConditions：不变量 `regexp 命中 s ⇒ gate(s) == true`。
//
// 为什么这条最重要：把一道「必要条件门」写成启发式（"看起来像不像路径"）就是 D13 的整段跳过 ——
// 值里的秘密原样落盘，而且**所有先前级别的用例都会继续绿**（它们只喂"像"的语料）。
// 这里用「全部消费者正则 × fuzz 语料 + 探针语料 + 随机串」把它钉死，并要求每个门
// 至少真命中的次数 > 0（否则一个恒返回 false 的门也能让这条用例绿）且真跳过的次数 > 0
// （否则门是恒真的，等于没写）。
func TestGatesAreNecessaryConditions(t *testing.T) {
	randSrc := rand.New(rand.NewSource(20260917)) // 固定种子：失败可复现
	const alphabet = `abkstpe:/=?&@.-_[]%XY0AKIAhF\u200b`
	inputs := append([]string{}, loadFuzzCorpus(t, filepath.Join("testdata", "fuzz", "FuzzRedactValue"))...)
	for _, c := range corpus() {
		for _, v := range flattenStrings(c.Event) {
			inputs = append(inputs, v)
		}
	}
	for _, v := range benchValues {
		inputs = append(inputs, v)
	}
	inputs = append(inputs, "AKIA"+strings.Repeat("0", 16), "AuthoriZAtion:*** 000I00",
		"&access_token=abc12345deadbeef", "Bearer "+tokSk, "~/x", "a@b.com", "<controller-ip>",
		"／Users／ｆｕｚｚ０１", "/Users/fuzz\u200b01", "", "x", "..")
	for i := 0; i < 4000; i++ {
		n := randSrc.Intn(80)
		var sb strings.Builder
		for j := 0; j < n; j++ {
			sb.WriteByte(alphabet[randSrc.Intn(len(alphabet))])
		}
		inputs = append(inputs, sb.String())
	}

	gates := []struct {
		name string
		gate func(string) bool
		res  []*regexp.Regexp
	}{
		{"hasTokenPrefix", hasTokenPrefix, []*regexp.Regexp{reSessTok}},
		{"hasBearerHint", hasBearerHint, []*regexp.Regexp{reBearer}},
		{"hasURLQueryHint", hasURLQueryHint, []*regexp.Regexp{reURLQuery}},
		{"hasDigit", hasDigit, []*regexp.Regexp{reIPv4}},
		{"hasHomePath", hasHomePath, []*regexp.Regexp{reHomeDir}},
		{"hasAtSign", hasAtSign, []*regexp.Regexp{reEmail}},
	}

	hits := map[string]int{}
	skips := map[string]int{}
	for _, in := range inputs {
		for _, g := range gates {
			matched := false
			for _, re := range g.res {
				if re.MatchString(in) {
					matched = true
					break
				}
			}
			ok := g.gate(in)
			if matched && !ok {
				t.Fatalf("门 %s 不是必要条件：正则命中但门为假 ⇒ 整段跳过（D13）：%q", g.name, in)
			}
			if matched {
				hits[g.name]++
			} else if !ok {
				skips[g.name]++
			}
		}
		// Tier-0 的令牌扫描也必须有门兜着（tokenRanges 的形态要求就是 hasTokenPrefix 的形态）。
		if len(tokenRanges(in)) > 0 && !hasTokenPrefix(in) {
			t.Fatalf("tokenRanges 命中了 hasTokenPrefix 认为不可能的值：%q", in)
		}
		// foldWidth 的必要条件：它只可能改变 ≥0x80 的 rune。
		if folded := foldWidth(in); folded != in && !hasNonASCII(in) {
			t.Fatalf("foldWidth 改变了纯 ASCII 值（hasNonASCII 不是必要条件）：%q → %q", in, folded)
		}
	}
	for _, g := range gates {
		if hits[g.name] == 0 {
			t.Errorf("门 %s 在这批输入上从未命中过 ⇒ 这条守扠没有区分力（假绿）", g.name)
		}
		if skips[g.name] == 0 {
			t.Errorf("门 %s 从未跳过任何输入 ⇒ 门是恒真的，等于没写", g.name)
		}
		t.Logf("门 %-16s 命中 %4d 次 · 跳过 %4d 次（输入总数 %d）", g.name, hits[g.name], skips[g.name], len(inputs))
	}
}

// flattenStrings 递归取出事件里的全部字符串（守扠用）。
func flattenStrings(v any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			out = append(out, t)
		case map[string]any:
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return out
}

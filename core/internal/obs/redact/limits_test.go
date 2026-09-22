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
		"path":     home,                                  // 本机家目录绝对路径（= home）
		"homepath": homeRoot + "/.config/zerg/token",      // 路径 + 用户名折叠的双载荷
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
	// 代价是一条 6 字节 blob（"bXMwMQ" = 夹具用户名的 base64）能藏下 4 个字符的秘密；收益是避免把
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
		"&access_token=abc12345deadbeef", "Bearer "+tokSk, homeRoot+"/x", "a@b.com", "172.16.5.9",
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

// ── ⑤ 门禁精度回归：本轮 fuzz 抓到的 `\b` 转义拼接误报（已锁成语料 #19）──────────
//
// 语料 testdata/fuzz/FuzzRedactValue/31f635b2df5ec393（本轮 20s fuzz 抓出）：
//
//	in = "\b" + "000CX00X01zMDE0A000X000" + " " + "X000CX00X01zMDE0A000X000"   （\b = 0x08）
//
// 第二个 run（24 字节）解码命中用户名 ⇒ 脱敏器照常整段遮蔽 ✓；第一个 run 只有 **23 字节**，
// 低于 base64 候选下限 24（设计口径，见 TestPreFilterStillMissesNothingBase64 文末），脱敏器不动它 —— 正确。
// 但门禁在**输出侧投影**上把 `\b` 转义里的 `b` 当成了数据字节 ⇒ 与那 23 字节拼成 24 字节候选并解码成功
// ⇒ 报出一个「读者解析这行 JSON 后并不存在」的残留。根因是 `jsonUnescapeRaw` 没还原 `\b`/`\f`
// （encoding/json 用短转义写 0x08/0x0C），与 verify.go 里已记录的 `\u0010` 拼接误报同类。
//
// 下面三条一起钉住它：① 转义还原能力本身；② 这个形状不许再报 leak；③ **反向控制** ——
// 真把 24 字节的编码载荷原样落盘时，门禁必须照旧能红（否则就是把门禁削弱成恒绿）。
func TestJSONUnescapeRawCoversAllShortEscapes(t *testing.T) {
	cases := []struct{ in, want string }{
		{`a\/b`, "a/b"},
		{`a\nb`, "a\nb"},
		{`a\tb`, "a\tb"},
		{`a\rb`, "a\rb"},
		{`a\bb`, "a\bb"},
		{`a\fb`, "a\fb"},
		{`a\"b`, `a"b`},
		{`a\\b`, `a\b`},
		{`a\u002fb`, "a/b"},
		// 一趟扫描的判据（反向控制用例抓出的假阴性）：转义过的反斜杠 + 字面 u0041 不许被当成 unicode 转义。
		{`a\\u0041b`, `a\u0041b`},
		{`a\u0041b`, "aAb"},
	}
	for _, c := range cases {
		if got := jsonUnescapeRaw(c.in); got != c.want {
			t.Errorf("jsonUnescapeRaw(%q)=%q，应为 %q", c.in, got, c.want)
		}
	}
}

// TestGateNotFooledByShortEscapeArtifact：语料 #19 的形状不许再报 leak。
func TestGateNotFooledByShortEscapeArtifact(t *testing.T) {
	in := "\b000CX00X01zMDE0A000X000 X000CX00X01zMDE0A000X000"
	line := string(MarshalRedacted(map[string]any{"err": in}))
	// 第二个 run 必须被遮（那是真的编码载荷）；第一个 run 23 字节、低于候选下限 ⇒ 保留原文。
	if !strings.Contains(line, PlaceholderB64) {
		t.Fatalf("编码载荷未被遮蔽（夹具失效）：%s", line)
	}
	if !strings.Contains(line, `\b000CX00X01zMDE0A000X000`) {
		t.Fatalf("夹具失效：第一个 run 本该原样保留（23 字节 < 候选下限）：%s", line)
	}
	if leaks := VerifyNoResidueString(map[string]any{"err": in}, line); len(leaks) > 0 {
		t.Fatalf("`\\b` 转义拼接造成的假 leak（D18：误报 ⇒ 门禁被关掉）：%+v line=%s", leaks, line)
	}
	// 回归信号：既有的 `\u0010` 拼接误报用例也在同一批（TestGateNotFooledByJSONEscapeArtifacts）。
}

// TestGateReverseControlOnEscapeArtifact：反向控制 —— 真载荷原样落盘必须报 leak。
// 没有这一条，上面那条断言可以靠「门禁恒绿」通过（等于把门禁关掉）。
func TestGateReverseControlOnEscapeArtifact(t *testing.T) {
	// 载荷长度取 3 的倍数 ⇒ base64 不带 '=' 填充（带填充的串不进 reB64Run 的候选集，
	// 那样反向控制会变成一个「夹具失效」而不是真的抓到漏检）。
	secret := "peer " + foldUser + " run here ok" // 21 字节 → base64 28 字节无填充；解码后含用户名
	blob := base64.StdEncoding.EncodeToString([]byte(secret))
	if len(blob) < 24 || strings.ContainsAny(blob, "=") {
		t.Fatalf("夹具失效：blob=%q（len=%d）", blob, len(blob))
	}
	ev := map[string]any{"err": blob}
	clean := string(MarshalRedacted(ev))
	if leaks := VerifyNoResidueString(ev, clean); len(leaks) > 0 {
		t.Fatalf("干净行被误判：%+v line=%s", leaks, clean)
	}
	// 把**原始**载荷直接写进行里（上游重新拼回/脱敏被绕过）⇒ 门禁必须报。
	poisoned := `{"err":"` + blob + `"}`
	if leaks := VerifyNoResidueString(ev, poisoned); len(leaks) == 0 {
		t.Fatal("base64 载荷原样落盘未报 leak ⇒ 门禁对编码通道失效（反向控制失败）")
	}
}

// ── ⑥ 门禁精度回归：非合法 UTF-8 的 needle（fuzz 实测，语料 f6243093e7100ee2）──────
//
// 语料 testdata/fuzz/FuzzRedactValue/f6243093e7100ee2（本轮 120s fuzz 抓出）：
//
//	in = "ǃЏڄĴո" + "&token=" + "\x83ЏڄĴ\xd5"      （值不是合法 UTF-8，且其字节是前缀的**字节窗口**）
//
// 脱敏器把 construct 遮成 `&token=[REDACTED]` ✓ 正确；但门禁（与 fuzz 同源）拿值的**原始字节**去比，
// 而那段字节恰好落在前缀 `ǃЏڄĴո` 里（ǃ 的第二个字节就是 0x83）⇒ 报出一个「值的字符串从未出现」的残留。
// 处置：needle 侧含非法 UTF-8 时改用**落盘形态**（encoding/json 逐字节写 U+FFFD）比较 ——
// 真漏必然在行里以 U+FFFD 形态出现，照样抓得到；字节窗口重叠不再命中。

// TestDiskFormMatchesEncodingJSON：落盘形态的口径必须与 encoding/json 一致（逐字节 U+FFFD）。
// 这条是上面那条处置的**地基**：口径错了，整条修正就站不住。
func TestDiskFormMatchesEncodingJSON(t *testing.T) {
	for _, in := range []string{"\x83Џ", "\xd5", "a\xbe\xbe\xbeb", "\u0605\xd800001", "\xff\xfe", "\xc7\x83\xd0\x8f", "ok"} {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var want string
		if err := json.Unmarshal(b, &want); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got := diskFormUTF8(in); got != want {
			t.Errorf("落盘形态与 encoding/json 不一致：in=% x  mine=% x  json=% x", []byte(in), []byte(got), []byte(want))
		}
	}
	// 逐字节替换（json 口径）≠ strings.ToValidUTF8（把整段非法 run 合成一个替身）。
	if a, b := diskFormUTF8("\x83\x89"), strings.ToValidUTF8("\x83\x89", "\ufffd"); a == b {
		t.Errorf("夹具失效：两者本该不同（逐字节 vs 合并 run），都是 % x", []byte(a))
	}
}

// TestGateNotFooledByByteWindowNeedle：字节窗口形态的 needle 不许再报 leak。
func TestGateNotFooledByByteWindowNeedle(t *testing.T) {
	in := "\u01c3\u040f\u0684\u0134\u0578&token=\x83\u040f\u0684\u0134\xd5"
	line := string(MarshalRedacted(map[string]any{"err": in}))
	// encoding/json 默认把 `&` 转成 \u0026 ⇒ 断言前先按读者视角还原（jsonUnescapeRaw）。
	if !strings.Contains(jsonUnescapeRaw(line), "&token="+PlaceholderRedacted) {
		t.Fatalf("construct 未被破坏（夹具失效）：%s", line)
	}
	if leaks := VerifyNoResidueString(map[string]any{"err": in}, line); len(leaks) > 0 {
		t.Fatalf("字节窗口造成的假 leak（D18：误报 ⇒ 门禁被关掉）：%+v line=%s", leaks, line)
	}

	// 反向控制：把值以**落盘形态**写回行里（模拟「脱敏器没遮」）⇒ 门禁必须报。
	// 没有这一条，上面那条断言可以靠「门禁对非 UTF-8 值恒不报」蒙过去。
	poisoned := `{"err":"` + "\u01c3\u040f\u0684\u0134\u0578&token=" + diskFormUTF8("\x83\u040f\u0684\u0134\xd5") + `"}`
	if leaks := VerifyNoResidueString(map[string]any{"err": in}, poisoned); len(leaks) == 0 {
		t.Fatal("值以落盘形态存活却未报 leak ⇒ 门禁被削弱成恒绿（反向控制失败）")
	}
}

// TestGateNotFooledByEqualLengthWeakView：等长的弱视图（fuzz 实测，语料 37e18001cc557c49）。
//
//	in = "\\0000\x801" + "&token=" + "\\\\0000\x801"
//	     （前缀 2 个反斜杠；值是 **4 个**反斜杠 + 0x80 —— 非合法 UTF-8，与前缀不是同一串）
//
// 值的 JSON 反转义视图是 `\\0000<FFFD>1`：少了 2 个反斜杠（4→2）又把 1 个非法字节撑成 3 字节
// （0x80 → U+FFFD）⇒ 与原始 needle **等长**，把「更短才算弱视图」的短路条件绕开了 ⇒ 拿它去比
// 子串，命中了行里前缀那份**普通内容**。处置：弱视图判据改成「按落盘形态对齐后的删字节」（weakViewOf）。
func TestGateNotFooledByEqualLengthWeakView(t *testing.T) {
	value := "\\\\\\\\0000\x801" // 4 个反斜杠 + 0000 + 0x80 + 1
	prefix := "\\\\0000\x801"    // 2 个反斜杠 + 同样尾巴（与值不同的字节串）
	in := prefix + "&token=" + value
	ev := map[string]any{"err": in}
	line := string(MarshalRedacted(ev))
	if !strings.Contains(jsonUnescapeRaw(line), "&token="+PlaceholderRedacted) {
		t.Fatalf("construct 未被破坏（夹具失效）：%s", line)
	}
	if leaks := VerifyNoResidueString(ev, line); len(leaks) > 0 {
		t.Fatalf("等长弱视图造成的假 leak（D18）：%+v line=%s", leaks, line)
	}

	// 反向控制：把**值原样落盘**（模拟脱敏器没遮）⇒ 门禁必须报（走的是 needle 的落盘形态视图）。
	poisonedB, err := json.Marshal(map[string]any{"err": value})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	poisoned := string(poisonedB)
	if !strings.Contains(jsonUnescapeRaw(poisoned), diskFormUTF8(value)) {
		t.Fatalf("夹具失效：毒行里看不到值的落盘形态：%s", poisoned)
	}
	if leaks := VerifyNoResidueString(ev, poisoned); len(leaks) == 0 {
		t.Fatal("值原样落盘未报 leak ⇒ 门禁被削弱成恒绿（反向控制失败）")
	}
}

// ── ⑦ 脱敏覆盖回归：`\/` 逃逸形态的路径（fuzz 实测，语料 462d9f1cd05fbb5e）──────────
//
// in = `/home/0/home\/0`：第一处是真路径（reHomeDir 已遮）· 第二处把斜杠写成了 `\/` ——
// 读者还原后看到的仍是 `/home/0`，而 reHomeDir 只认 `/` 分隔符 ⇒ **漏遮**（D19 把 `\/` 列为
// 四类藏法之一，probe 语料里的 json-escaped-slash 只因为含当前用户名才被用户名折叠顺手遮住）。
// 处置：新增 reHomeEscape（分隔符 `/` 或 `\` 均可）+ 必要条件门 hasEscapedPath（含反斜杠 + Users/home）。
func TestEscapedSlashPathIsRedacted(t *testing.T) {
	in := "/home/0/home" + `\` + "/0"
	ev := map[string]any{"err": in}
	line := string(MarshalRedacted(ev))
	if !strings.Contains(line, PlaceholderPath) {
		t.Fatalf("逃逸形态的路径未被遮蔽：%s", line)
	}
	mustVanish(t, line, "/home/0") // 任何投影下都不许再出现（含反转义视图）
	if leaks := VerifyNoResidueString(ev, line); len(leaks) > 0 {
		t.Fatalf("门禁报残留：%+v line=%s", leaks, line)
	}
	if again := RedactValue(RedactValue(in)); again != RedactValue(in) {
		t.Fatalf("逃逸路径规则不幂等：%q → %q → %q", in, RedactValue(in), again)
	}

	// 同一条通道的另一半：Windows 风格的 `C:\Users\me\...`（分隔符是 `\`，一条真路径）。
	win := `C:\Users\me\secret.txt`
	wout := MarshalRedacted(map[string]any{"err": win})
	if !strings.Contains(string(wout), PlaceholderPath) || strings.Contains(string(wout), "\\Users") {
		t.Errorf("Windows 风格 home 路径未被遮蔽：%s", wout)
	}

	// 不过度遮蔽：逃逸形态但**不是 home 路径**的值原样保留（D12：过度遮蔽与漏脱敏同罪）。
	if got := RedactValue(`log \/tmp\/x ok`); got != `log \/tmp\/x ok` {
		t.Errorf("非 home 路径的转义串被无谓遮蔽：%q", got)
	}
}

// TestGateNotFooledByEscapeTransformedView：`\u0000` 这种**换字节**的反转义视图不计入
// （fuzz 实测，语料 618e416840bfd820）。
//
//	in = "&token=" + "\u0000" + "\x9b0" + " " + "\x00\x800"
//
// 值的反转义视图把 6 个字符的 `\u0000` **换成** 1 个 NUL 字节 ⇒ 视图里出现了 needle 本身
// 不含的字节，恰好撞上输入尾部那份普通内容（`\x00\x800` → 行里的 `\u0000` + U+FFFD + 0）⇒ 假 leak。
// 处置：反转义视图只在它是主视图的**删字节**结果时才计入（`\/`→`/`、`\\`→`\` 这类是删字节，保留；
// `\uXXXX`→rune 是换字节，丢弃）—— 真漏不需要它：值原样落盘时主视图（落盘形态）必然命中。
func TestGateNotFooledByEscapeTransformedView(t *testing.T) {
	in := "&token=" + `\u0000` + "\x9b0 \x00\x800"
	m := reURLQuery.FindAllStringSubmatch(in, -1)
	if len(m) != 1 {
		t.Fatalf("夹具失效：URL-query 命中 %d 次（值 = %q）", len(m), in)
	}
	ev := map[string]any{"err": in}
	line := string(MarshalRedacted(ev))
	if !strings.Contains(jsonUnescapeRaw(line), "&token="+PlaceholderRedacted) {
		t.Fatalf("construct 未被破坏（夹具失效）：%s", line)
	}
	if leaks := VerifyNoResidueString(ev, line); len(leaks) > 0 {
		t.Fatalf("换字节视图造成的假 leak（D18）：%+v line=%s", leaks, line)
	}

	// 反向控制：把**输入原样**落盘（脱敏器没遮 / 上游拼回原文）⇒ 门禁必须报。
	// 注意不能只喂孤立的 value：needle 是从值里的**模式命中**（这里是 URL-query construct）提取的，
	// 单喂一个不含 construct 的值会得到空 needle 集 ⇒ 那条断言会变成空转（假绿）。
	verbatim, err := json.Marshal(map[string]any{"err": in})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if leaks := VerifyNoResidueString(ev, string(verbatim)); len(leaks) == 0 {
		t.Fatalf("输入原样落盘未报 leak ⇒ 门禁被削弱成恒绿（反向控制失败）：%s", verbatim)
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

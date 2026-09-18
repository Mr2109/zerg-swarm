// Package redact 是 Zerg 事件流「写入时脱敏」的**零依赖叶包**（T2.1–T2.3）。
//
// 设计依据（改本包前先读这两份）：
//   - docs/01-设计/设计-内建调试版-v1.2-20260917.md 第〇节 D 组（D1–D12）
//   - docs/01-设计/设计-内建调试版附-D-组脱敏实测补强-v1.2.md
//     （D13–D21、字段三分表 §五、八步清单 §六、实测基准 §三）
//
// 三条不变量（本包实现后两条；第一条「唯一写入口 emit」在 T2.1 的接入步落地）：
//  1. **先脱敏后编码** —— 在编码后的表单上跑 ASCII 正则会整体失效，故 API 收 map/string，
//     不收 []byte：脱敏发生在 json.Marshal **之前**。
//  2. **fail-closed** —— 脱敏 panic 或超预算 ⇒ 只写 {"event":"redaction_failed"}，
//     **绝不回退写原文**（见 failclosed.go）。
//  3. **脱敏器与回扫门禁同源** —— 同一份键分类表（keys.go）、同一份模式表（本文件）、
//     同一个用户折叠（keys.go）；不同源的门禁会误报，然后被人关掉（D18）。
//
// 管线（成本序，实测口径见设计附件 §三）：
//
//	Tier 0 结构判定 —— 永不进预过滤，无正则：固定形态令牌（首字节索引）+ 编码载荷
//	       （percent / base64；编码能把秘密藏过任何明文规则，D13）。
//	Tier 1 正则包 —— 前置条件是**必要条件**（不是「看起来像不像路径」的启发式）：
//	       IndexByte / strings.Contains(s,"/Users/") / 首字节索引 / run 长度。
//
// 依赖纪律：只用标准库（encoding/json·regexp·encoding/base64·strings·unicode…）。
// 不 import 仓库内任何其它包（叶包 ⇒ 无循环依赖风险，可被任何层引用）。
package redact

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 占位符（统一口径，八步清单第 5 步）──────────────────────────────────────
//
// 一律用 ReplaceAllLiteralString / ReplaceAllStringFunc 写出（见 replaceLiteral）：
// `ReplaceAllString` 会把 "${HOME}" 当**命名捕获组**展开成空串 ⇒ 静默删数据（D14）。
const (
	PlaceholderRedacted  = "[REDACTED]"        // 通用遮蔽：令牌形态 / 邮箱 / 内网 IP / Bearer 值
	PlaceholderPath      = "${HOME}"           // 绝对路径前缀（保留尾部文件名，便于定位）
	PlaceholderUser      = "${USER}"           // 裸用户名
	PlaceholderPct       = "[REDACTED:pct]"    // percent 载荷：整段替换（重编码有损，半遮更差）
	PlaceholderB64       = "[REDACTED:base64]" // base64 载荷：整段替换
	PlaceholderTruncated = "…[TRUNCATED]"      // 超长值截断（八步清单第 7 步，值长上限）
)

// ── Tier 1：正则包 ───────────────────────────────────────────────────────────
//
// 每一条都必须有**必要条件**前置门（见各调用点）；没有前置门的正则会在大值上吃掉吞吐（D6）。
var (
	reEmail    = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reBearer   = regexp.MustCompile(`(?i)\b(bearer|token|api[_-]?key|authorization|passwd|password)\b["']?\s*[:=]\s*["']?([A-Za-z0-9._\-]{6,})`)
	reIPv4     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reURLQuery = regexp.MustCompile(`(?i)([?&](?:token|key|sig|signature|access_token|api_key|secret)=)([^\s"'&#]+)`)
	reHomeDir  = regexp.MustCompile(`/(?:Users|home)/[A-Za-z0-9._-]+`)
	// reHomeEscape：与 reHomeDir 只差一件事 —— 分隔符既可以是 `/` 也可以是 `\`。
	// 覆盖「写者把斜杠写成 `\/`（或 Windows 风格 `C:\Users\me`）」的逃逸形态：读者还原后看到的
	// 是一条真路径 ⇒ 只按原始字节跑 reHomeDir 会漏（fuzz 实测语料 462d9f1cd05fbb5e：
	// `/home/0/home\/0` 的第二处 `\/` 形态；D19 把 `\/` 列为四类藏法之一）。
	reHomeEscape = regexp.MustCompile(`[\\/]+(?:Users|home)[\\/]+[A-Za-z0-9._-]+`)
	// reSessTok 与下面的 tokenPrefixes **必须描述同一批形态**（D18/策略不一致的教训）：
	// 门禁认为「sk0000000000000000」是秘密而脱敏器不遮 ⇒ CI 在非泄漏上红；
	// 反过来 ⇒ 泄漏进仓。一份策略，两个消费者。
	reSessTok = regexp.MustCompile(`(?:\b(?:sk|pk|rk|ghp|gho|ghs|glpat|hf|xox[baprs])[-_][A-Za-z0-9_\-]{12,}\b|\b(?:AKIA|ASIA|AIza)[A-Za-z0-9_\-]{12,}\b|\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\b)`)
	rePctRun  = regexp.MustCompile(`(?:%[0-9A-Fa-f]{2}){1,}`)
	reB64Run  = regexp.MustCompile(`[A-Za-z0-9+/_\-]{24,}={0,2}`)
	rePhoneCN = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
)

// ── Tier 0：固定形态令牌（无正则，IndexByte/切片操作，恒定便宜）─────────────
//
// 形态来源与 reSessTok 的交替分支**一一对应**（一份策略，两个消费者）：形态 = 词干 × 分隔符
// （sk-/sk_/hf-/hf_/glpat-/glpat_/…）加无分隔符的四种（AKIA/ASIA/AIza/eyJ）。
// 若这里漏掉某个形态（例如只写 hf_ 而正则也接受 hf-），门禁会看见一个脱敏器遮不住的秘密
// ⇒ 泄漏进仓（D18 的「策略不一致」正是这么来的，fuzz 会把它逼出来）。
var tokenStems = []string{
	"sk", "pk", "rk", "ghp", "gho", "ghs", "glpat", "hf",
	"xoxb", "xoxp", "xoxa", "xoxr", "xoxs",
}

var tokenSeps = []string{"-", "_"}

// tokenPrefixes 由词干×分隔符生成 + 四个无分隔符形态。
var tokenPrefixes = func() []string {
	out := make([]string, 0, len(tokenStems)*len(tokenSeps)+4)
	for _, st := range tokenStems {
		for _, sp := range tokenSeps {
			out = append(out, st+sp)
		}
	}
	return append(out, "AKIA", "ASIA", "AIza", "eyJ")
}()

// tokenIdx 按首字节（小写化）建索引：每个字节 20+ 次 HasPrefix 变成几乎恒为空切片的一次查表。
var tokenIdx = func() map[byte][]string {
	m := map[byte][]string{}
	for _, p := range tokenPrefixes {
		c := lowerByte(p[0])
		m[c] = append(m[c], p)
	}
	return m
}()

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func isTokenByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_' || c == '.' || c == '=' || c == '+':
		return true
	}
	return false
}

// maskTokenShapes 遮蔽 sk-/hf_/AKIA/eyJ… 形态的凭据。
// 前缀后的**可见**令牌字节 ≥12 才算真凭据（避免把 "hf_x" 这种普通标识当秘密 —— D12 误报口径）。
//
// Cf/零宽字符对读者**不可见** ⇒ 不能充当分隔符。fuzz 实测两条真泄漏：
//
//	"AKIA00000000\u06dd0000"      —— Cf 藏在令牌中间，逐字节只数出 10 个令牌字符（<12）⇒ 漏遮
//	"AK\u06ddIA000000000000"      —— Cf 藏在**前缀**里，前缀匹配直接失败 ⇒ 漏遮
//
// 两处的读者视角都是标准令牌形态。做法：摘掉不可见字符得到「可见视图」，在可见视图上扫令牌区间，
// 再把区间映射回原文（区间内的不可见字符一并遮蔽）。没有不可见字符时走原串直扫的快路径
// （绝大多数值走这里，零额外分配）。
//
// 必要条件门（T2.7）：**没有可见令牌前缀、也没有不可见字符 ⇒ 什么都不可能命中**，直接返回。
// 两支都要保留：Cf 藏在前缀里时原文不含任何前缀，只能靠 hasInvisible 那一支兜住。
func maskTokenShapes(s string) string {
	if !hasInvisible(s) {
		if !hasTokenPrefix(s) {
			return s // 必要条件门：绝大多数值在这里返回（C 类被丢弃、A 类零扫描，剩下的都走这条）
		}
		return maskTokenRangesIn(s, s, nil)
	}
	vis, idx := stripInvisible(s)
	return maskTokenRangesIn(s, vis, idx)
}

// hasInvisible 判定是否含 Cf/零宽字符。只有出现非 ASCII 字节才需要逐 rune 判定。
func hasInvisible(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x80 {
			continue
		}
		for _, r := range s[i:] {
			if unicode.Is(unicode.Cf, r) || r == 0xFEFF {
				return true
			}
		}
		return false
	}
	return false
}

// stripInvisible 返回「可见视图」及「可见字节 → 原串下标」映射。
func stripInvisible(s string) (string, []int) {
	var b strings.Builder
	b.Grow(len(s))
	idx := make([]int, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if size > 1 && (unicode.Is(unicode.Cf, r) || r == 0xFEFF) {
			i += size
			continue
		}
		b.WriteByte(s[i])
		idx = append(idx, i)
		i++
	}
	return b.String(), idx
}

// maskTokenRangesIn 在可见视图 vis 上扫令牌区间，按 idx 映射回原串 src 输出遮蔽结果。
// idx == nil 表示 vis 与 src 逐字节对齐。
func maskTokenRangesIn(src, vis string, idx []int) string {
	ranges := tokenRanges(vis)
	if len(ranges) == 0 {
		return src
	}
	var out strings.Builder
	out.Grow(len(src))
	prev := 0
	for _, rg := range ranges {
		start, end := rg[0], rg[1]
		if idx != nil {
			start, end = idx[rg[0]], idx[rg[1]-1]+1
		}
		if start < prev {
			start = prev
		}
		out.WriteString(src[prev:start])
		out.WriteString(PlaceholderRedacted)
		prev = end
	}
	out.WriteString(src[prev:])
	return out.String()
}

// tokenRanges 扫出 vis 上需要遮蔽的字节区间（互不重叠、按序）。
// tokenFirst 表先把「首字节不可能是任何前缀开头」的位置一次索引跳掉（原实现每个字节一次
// map 哈希；合成事件上 tokenRanges 是 Tier-0 里最热的一段）。
func tokenRanges(vis string) [][2]int {
	var out [][2]int
	for i := 0; i < len(vis); {
		if !tokenFirst[vis[i]] {
			i++
			continue
		}
		matched := false
		for _, p := range tokenIdx[lowerByte(vis[i])] {
			if i+len(p) <= len(vis) && strings.EqualFold(vis[i:i+len(p)], p) {
				j := i + len(p)
				for j < len(vis) && isTokenByte(vis[j]) {
					j++
				}
				if j-(i+len(p)) >= 12 {
					out = append(out, [2]int{i, j})
					i = j
					matched = true
					break
				}
			}
		}
		if !matched {
			i++
		}
	}
	return out
}

// ── Tier 0：编码载荷（percent / base64）──────────────────────────────────────
//
// **编码通道必须在预过滤之前**（Tier-0，永不被 strings.Contains 预过滤跳过）：
// 纯 base64 blob（无 `/ @ . = :`）会被任何启发式预过滤整段跳过（D13 实测）。

func urldecodeAll(s string) string {
	return rePctRun.ReplaceAllStringFunc(s, func(m string) string {
		var sb strings.Builder
		for i := 0; i+3 <= len(m); i += 3 {
			v, err := strconv.ParseUint(m[i+1:i+3], 16, 8)
			if err != nil {
				return m
			}
			sb.WriteByte(byte(v))
		}
		return sb.String()
	})
}

// printable 判定解码结果是否像文本：对路径/代码里任意 base64 样的子串解码会得到二进制垃圾，
// 遮掉它只会毁掉可调试性、换不来任何安全收益。
func printable(b []byte) bool {
	if len(b) == 0 || !utf8.Valid(b) {
		return false
	}
	good := 0
	for _, r := range string(b) {
		if r == '\n' || r == '\t' || r == '\r' || unicode.IsPrint(r) {
			good++
		}
	}
	return good*10 >= len([]rune(string(b)))*9
}

// maybeSensitiveInner 判「解码后的明文是否需要遮蔽」——与脱敏器同一份模式表。
func maybeSensitiveInner(s string) bool {
	if s == "" {
		return false
	}
	st := userFold.Load()
	return reHomeDir.MatchString(s) || reEmail.MatchString(s) || reIPv4.MatchString(s) ||
		reSessTok.MatchString(s) || reBearer.MatchString(s) || reURLQuery.MatchString(s) ||
		rePhoneCN.MatchString(s) || (st != nil && strings.Contains(strings.ToLower(s), strings.ToLower(st.name)))
}

// maskEncodedPayloads 解码 percent / base64 段并对解码结果重判；命中则**整段**替换
// （重编码有损，半遮的值比丢掉更糟）。两条入口都有必要条件门：'%' 与 run 长度 ≥24。
func maskEncodedPayloads(s string) string {
	if strings.IndexByte(s, '%') >= 0 {
		if dec := urldecodeAll(s); dec != s && maybeSensitiveInner(dec) {
			return PlaceholderPct
		}
	}
	if len(s) < 24 { // reB64Run 的最小长度是必要条件，不是启发式
		return s
	}
	return reB64Run.ReplaceAllStringFunc(s, func(m string) string {
		pad := m
		if r := len(pad) % 4; r != 0 {
			pad += strings.Repeat("=", 4-r)
		}
		for _, dec := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawURLEncoding, base64.RawStdEncoding} {
			b, err := dec.DecodeString(pad)
			if err != nil || !printable(b) {
				continue
			}
			if maybeSensitiveInner(string(b)) {
				return PlaceholderB64
			}
		}
		return m
	})
}

// ── Tier 1 的预过滤：必须是模式的**必要条件** ────────────────────────────────
//
// 这里问的是「有没有分隔符形状的字节」——它是正则包的**必要条件**（正则只可能命中含 / \ @
// = : . - _ + ? & % 的串），所以跳过是安全的；任何「看起来像不像路径」的启发式判定都会
// 整段跳过纯 base64 blob 与裸用户名（D13）。
func maybeSensitive(s string) bool {
	if len(s) < 4 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '/', '\\', '@', '=', ':', '.', '-', '_', '+', '?', '&', '%':
			return true
		}
	}
	return false
}

// ── 归一化：全角↔半角 + 去 Cf（零依赖下的 NFKC 近似）────────────────────────
//
// 真 NFKC 需要 golang.org/x/text/unicode/norm（第三方）⇒ 本包按零依赖纪律不引（**未做**，
// 已如实登记）。这里覆盖实测到的全部绕过形态：全角同形字（／Users／ｆｕｚｚ０１）与
// 零宽/格式符（/Users/fuzz\u200b01）。回扫门禁过同一个函数（同源，否则门禁看不见被折叠的原文）。
//
// Cf/零宽字符**换成空格而不是删掉**（fuzz 实测的真泄漏，不是洁癖）：删掉会把两侧「粘」起来，
// 造出原文里不存在的词边界关系 —— 输入 "0\u0605token=000000" 折叠成 "0token=000000" 后，
// `\btoken\b` 不再命中 ⇒ 该遮的关键字构造没被遮（值 000000 原样落盘）。换成空格则保留边界：
// "0 token=000000" ⇒ reBearer 正常命中并遮蔽。
func foldWidth(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			return r - 0xFEE0
		case r == 0x3000:
			return ' '
		}
		if unicode.Is(unicode.Cf, r) || r == 0xFEFF {
			return ' '
		}
		return r
	}, s)
}

// replaceLiteral 是 ReplaceAllLiteralString：替换串**不做 $ 展开**。
// 写错会静默删数据：`re.ReplaceAllString(s, "${HOME}")` 把 ${HOME} 当命名捕获组 ⇒ 替换为空串，
// 实测 '~/.config/zerg/token' → '/.config/zerg/token'（路径被删而不是被打码，D14）。
func replaceLiteral(re *regexp.Regexp, s, repl string) string {
	return re.ReplaceAllLiteralString(s, repl)
}

// hasHomePath 是 reHomeDir 的**必要条件**门：正则不可能命中，当且仅当这两个字面量都不在串里。
// 用必要条件门换掉正则，是「安全的快路径」与「会静默丢覆盖的启发式」的分界线。
func hasHomePath(s string) bool {
	return strings.Contains(s, "/Users/") || strings.Contains(s, "/home/")
}

// hasEscapedPath 是 reHomeEscape 的**必要条件**门：任何 `\/` / `\Users` 形态都必然含反斜杠，
// 且正则要求 Users/home 字面量。两个条件都便宜（IndexByte + Contains），不满足即整段跳过。
func hasEscapedPath(s string) bool {
	if strings.IndexByte(s, 0x5C) < 0 {
		return false
	}
	return strings.Contains(s, "Users") || strings.Contains(s, "home")
}

// ── 值管线（唯一入口：RedactValue）──────────────────────────────────────────

// maxRedactRounds 是迭代到不动点的轮数上限（脱敏是重写系统，见 RedactValue 的说明）。
const maxRedactRounds = 3

// RedactValue 脱敏单个字符串值；**幂等**（f(x)==f(f(x))，见 fuzz 性质 b）。
//
// 为什么要迭代：脱敏是**重写系统**，一轮改写可能「解锁」下一轮才命中的形态。fuzz 实测命中过：
//
//	in  = "&token= 00000000!0"   （'=' 后面是空格 ⇒ URL-query 规则要求值非空、当场不匹配）
//	1st = "&token=[REDACTED]!0"  （但 Bearer 规则允许 `\s*[:=]\s*` ⇒ 它先命中）
//	2nd = "&token=[REDACTED]"    （第二轮 URL-query 规则才看见 [REDACTED]!0 并命中）
//
// ⇒ 一轮即返回就破坏幂等，而重试/重放/二次导出路径全都依赖幂等。做法：**只对已被改写过的值**
// 迭代到不动点（绝大多数值一轮即稳，例如基准里的 "read /tmp/f0 ok" ⇒ 零额外成本）。
func RedactValue(s string) string {
	pipelineEntries.Add(1) // 诊断计数：进值管线的字符串个数（A 类零扫描不计，见 gate.go）
	out := redactPass(s)
	if out != s {
		for i := 0; i < maxRedactRounds; i++ {
			again := redactPass(out)
			if again == out {
				break
			}
			out = again
		}
	}
	return truncateValue(out)
}

// redactPass 是单轮管线（Tier-0 + Tier-1）；不含截断（截断只在最后做一次，避免与幂等相互干扰）。
func redactPass(s string) string {
	orig := s
	// ---- Tier 0：恒定评估、必要条件把关、无启发式 ----
	s = maskTokenShapes(s)     // 首字节索引，无正则
	s = maskEncodedPayloads(s) // 需要 '%' 或 ≥24 字符的 b64 候选
	if hasHomePath(s) {
		s = replaceLiteral(reHomeDir, s, PlaceholderPath)
	}
	// 逃逸形态（`\/` / `\Users`）：读者还原后是一条真路径 ⇒ 与上面同一条口径（D19 的 `\/` 通道）。
	if hasEscapedPath(s) {
		s = replaceLiteral(reHomeEscape, s, PlaceholderPath)
	}
	if hasUser(s) {
		st := userFold.Load()
		s = replaceLiteral(st.re, s, PlaceholderUser)
	}

	// ---- Tier 1：预过滤买下整个正则包 ----
	if !maybeSensitive(s) {
		return s
	}
	// 归一化（全角↔半角 + 去 Cf）只在含非 ASCII 字节时做：foldWidth 只可能改变 ≥0x80 的 rune
	// ⇒ 纯 ASCII 值上它是恒等变换（原实现每条值都白付一次 strings.Map 的整串分配）。
	if hasNonASCII(s) {
		s = foldWidth(s)
	}
	// 令牌规则：整条 reSessTok 被「含不含令牌前缀」一道必要条件门买下（替换串原样写，D14）。
	if hasTokenPrefix(s) {
		s = replaceLiteral(reSessTok, s, PlaceholderRedacted)
	}
	// 关键字类规则（URL query / Bearer）：「关键字 + 分隔符」是它们的形态，先用必要条件门
	// （含 '=' / ':' 或 '?' / '&'，且含关键字）买下这三条正则的扫描 —— pprof 实测
	// keywordRuleValues 的两次 FindAllStringSubmatch 与两次 ReplaceAllStringFunc 合计占事件成本
	// 的三成以上，而绝大多数值（代码行、日志行）连一个关键字都没有。
	//
	// 两份值都在**改写之前**的同一个 s 上取（与原来的 keywordRuleValues 完全同口径），
	// 只是各自套上自己的必要条件门 —— 门失败（不可能命中）时它本来也取不到值。
	var keywordVals []string
	if hasURLQueryHint(s) {
		keywordVals = append(keywordVals, urlQueryValues(s)...)
	}
	if hasBearerHint(s) {
		keywordVals = append(keywordVals, bearerValues(s)...)
	}
	if hasURLQueryHint(s) {
		s = reURLQuery.ReplaceAllStringFunc(s, func(m string) string {
			g := reURLQuery.FindStringSubmatch(m)
			return g[1] + PlaceholderRedacted
		})
	}
	if hasBearerHint(s) {
		s = reBearer.ReplaceAllStringFunc(s, func(m string) string {
			g := reBearer.FindStringSubmatch(m)
			return g[1] + "=" + PlaceholderRedacted
		})
	}
	// 关键字类规则只遮蔽「关键字=值」这一处 ⇒ 值的**其余拷贝**还在同一字符串里（fuzz 实测命中：
	// "AuthoriZAtion:*** 000I00" 的尾部那份会原样留下 —— 那是真泄漏，而且门禁会为它报警）。
	// 既然这个值已被判定为秘密，就把同一字符串里它的其余出现一并遮蔽。
	if len(keywordVals) > 0 {
		s = maskValuesEverywhere(s, keywordVals)
	}
	// 折叠可能刚刚「露出」Tier-0 的形态 ⇒ 重跑一遍 Tier-0（全角/零宽探针就靠这一步）
	if hasHomePath(s) {
		s = replaceLiteral(reHomeDir, s, PlaceholderPath)
	}
	if hasUser(s) {
		st := userFold.Load()
		s = replaceLiteral(st.re, s, PlaceholderUser)
	}
	if hasAtSign(s) {
		s = replaceLiteral(reEmail, s, PlaceholderRedacted)
	}
	// reIPv4 的必要条件：至少含一个数字（形态是 \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}）。
	if hasDigit(s) {
		s = replaceLiteral(reIPv4, s, PlaceholderRedacted)
	}

	// 字节级收口：把已判定秘密的**字面量**在同一字符串里的其余出现也遮蔽掉。
	// 为什么必须做：IPv4 规则要 `\b` 词边界 ⇒ "0.0.0.0 A0.0.0.0" 里第二处不命中（前面是字母），
	// 但它逐字包含第一处的 IP（fuzz 实测命中，门禁为它报残留）。路径/用户名/令牌形态的规则本身
	// 就不要求词边界（任意位置命中）⇒ 天然已收口；只有 IPv4 / 邮箱 / 关键字值需要这一步。
	// 成本：只在**确实被改写过的值**上做（未改写 ⇒ 没有已判定的秘密值），且都是一遍线性扫描。
	if s != orig {
		prop := append([]string{}, keywordVals...)
		// 两条取值同样先过必要条件门（原实现无条件跑两遍 FindAllString）。
		if hasDigit(orig) {
			prop = append(prop, reIPv4.FindAllString(orig, -1)...)
		}
		if hasAtSign(orig) {
			prop = append(prop, reEmail.FindAllString(orig, -1)...)
		}
		if len(prop) > 0 {
			s = maskValuesEverywhere(s, prop)
		}
	}
	return s
}

// ── 关键字类规则的「值外溢」处理 ─────────────────────────────────────────────
//
// reBearer / reURLQuery 的形状是「关键字 + 分隔符 + 值」：它们只认得出**带上下文**的那一处。
// 值本身在同一个字符串里可能还有别的拷贝（fuzz 实测："AuthoriZAtion:000I00 000I00"）。
// 那第二份拷贝是一条真泄漏（读日志的人照样拿到凭据）⇒ 已判定的秘密值在同一字符串里一律遮蔽。

// reBareKeyword：值恰好是裸关键字本身（"Authorization: Bearer" 里的 "Bearer"）时**不外溢** ——
// 把普通文本里的 Bearer/token 等词无谓遮掉同样破坏可调试性（D12：过度遮蔽与漏脱敏是同一种伤害）。
var reBareKeyword = regexp.MustCompile(`(?i)^(bearer|token|authorization|api[_-]?key|key|secret|sig|signature|access_token|password|passwd)$`)

// urlQueryValues / bearerValues 取出这一轮关键字类规则认定的秘密值（按出现顺序）。
// 拆成两个函数只为一件事：让每条规则各自被自己的**必要条件门**买下（见 redactPass 的调用点）。
// 取值的口径一个字没变（g[2] = 捕获到的值）。
func urlQueryValues(s string) []string {
	var out []string
	for _, g := range reURLQuery.FindAllStringSubmatch(s, -1) {
		out = append(out, g[2])
	}
	return out
}

func bearerValues(s string) []string {
	var out []string
	for _, g := range reBearer.FindAllStringSubmatch(s, -1) {
		out = append(out, g[2])
	}
	return out
}

// keywordRuleValues 是两份取值的合并（两个消费者共用一份策略的口径，D18）。
// 生产路径走上面的门控分支；这里保留给「一次拿全」的调用方与用例。
func keywordRuleValues(s string) []string {
	out := urlQueryValues(s)
	return append(out, bearerValues(s)...)
}

// maskValuesEverywhere 把已判定的秘密值在同一字符串里的所有出现逐字遮蔽。
// 用 strings.ReplaceAll（字面替换，不经正则）⇒ 同样不会踩 $ 展开那一坑（D14）。
func maskValuesEverywhere(s string, vals []string) string {
	for _, v := range vals {
		if len(v) < 4 || reBareKeyword.MatchString(v) {
			continue
		}
		s = strings.ReplaceAll(s, v, PlaceholderRedacted)
	}
	return s
}

// ── 结构 walk（递归 + 深度上限；只脱敏顶层字段的基线实测 15 条探针漏 10 条，D15）──

// MaxDepth 是结构遍历的深度上限；超出即 fail-closed（标记而非放行）。
const MaxDepth = 32

// RedactEvent 是脱敏的唯一结构入口：递归 walk，键分类表在每一层生效。
// 不修改入参（返回新 map/slice）；未知键一律按 ClassScan 处理（fail-closed 倾向，D4）。
func RedactEvent(ev map[string]any) map[string]any {
	if ev == nil {
		return nil
	}
	out := make(map[string]any, len(ev))
	walkMap(ev, out, 0)
	return out
}

func walkMap(in map[string]any, out map[string]any, depth int) {
	if depth > MaxDepth { // fail-closed：不递归、不放行
		out["_depth_exceeded"] = true
		return
	}
	for k, v := range in {
		switch Classify(k) {
		case ClassDrop, ClassMask:
			// C 直接丢弃 / B 键级遮蔽：写成占位符而不是删键 —— 「缺席」必须可判定
			// （否则读日志的人分不清「没采到」和「被丢了」）。值一个字节都不看（分级第 ① 层）。
			out[k] = PlaceholderRedacted
		case ClassKeep:
			// 分级第 ② 层：A 类标量**零扫描**（不进值管线），容器仍递归（见 gate.go 的说明）。
			out[k] = keepValue(v, depth)
		default:
			out[k] = walkValue(v, depth)
		}
	}
}

func walkValue(v any, depth int) any {
	switch t := v.(type) {
	case string:
		return RedactValue(t)
	case map[string]any:
		m := make(map[string]any, len(t))
		walkMap(t, m, depth+1)
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = walkValue(t[i], depth+1)
		}
		return s
	case json.Number:
		return t
	default:
		// 类型优先分派：数字/布尔/nil 原样通过，不进字符串管线（D6 的类型门控）
		return v
	}
}

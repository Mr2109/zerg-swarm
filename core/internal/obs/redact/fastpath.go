package redact

import "strings"

// ── 必要条件快路径（T2.7：性能）──────────────────────────────────────────────
//
// 设计附件 §三 结论② 把「优化正则」排在省钱顺序的**最后一位**
// （丢弃 > 分级 > 截断 > 采样 > 优化正则）。T2.7 的验收是「事件成本 ≤ 原型 327µs（同机同口径）」，
// 而本包比原型多付了两笔原型没有的钱：
//
//	① Tier-0 固定形态令牌 + 编码通道（D13 要求它**不被预过滤跳过**）⇒ 每个值都要扫一遍；
//	② 迭代到不动点（fuzz 逼出来的幂等要求）⇒ 被改写过的值要再走一轮。
//
// 这两笔不能省（省了就是漏脱敏或破坏幂等），只能把 Tier-1 的「每条值都跑整包正则」
// 改回「必要条件门买下整条正则」——即原型的设计意图，本包第一版只做了 maybeSensitive 一道门。
// pprof 实测（合成事件 50 行，改动前）：replaceLiteral 占 37.6%、ReplaceAllStringFunc 占 23.8%、
// keywordRuleValues 占 7.9% —— 全是「明知不可能命中还照样跑」的开销。
//
// 铁律（这一条写错就是 D13 的**整段跳过** ⇒ 秘密原样落盘）：
//
//	regexp 命中 s   ⇒   gate(s) == true
//
// 门必须是**必要条件**，绝不能是「看起来像不像路径」的启发式。该不变量不是靠注释承诺的：
// TestGatesAreNecessaryConditions 拿**全部消费者正则**在 fuzz 语料 + 探针语料 + 随机串上逐条验证，
// 并且要求「至少命中过 N 次」——否则一个恒返回 false 的门也能让那条用例绿（没有区分力的断言
// 就是假绿）。fuzz 语料整批重放是第二道（TestFuzzCorpusReplayNoRegression）。

// ── 通用小工具 ───────────────────────────────────────────────────────────────

// byteSet 是 256 位查找表：把「每个字节一次 map 哈希」压成一次数组索引。
type byteSet [256]bool

var digitSet = func() byteSet {
	var t byteSet
	for c := byte('0'); c <= '9'; c++ {
		t[c] = true
	}
	return t
}()

func hasDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if digitSet[s[i]] {
			return true
		}
	}
	return false
}

// hasNonASCII：foldWidth 的必要条件 —— foldWidth 只改 ≥0x80 的 rune
// （全角 0xFF01–0xFF5E / 全角空格 0x3000 / Cf / BOM）。纯 ASCII 值上一律跳过，
// 省掉每次 strings.Map 的整串分配（原实现每条值都分配一次）。
func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

func hasAtSign(s string) bool { return strings.IndexByte(s, '@') >= 0 }

// equalFoldASCII 是大小写无关的字节比较（**两侧都折叠**）。
// 注意 tokenPrefixes 里有无分隔符的混合大小写形态（AKIA / ASIA / AIza / eyJ）——
// 只折叠 a 侧会让 "AKIA…" 这类令牌形态整段放走（回归信号：fuzz 语料种子 "AKIA"+… 红）。
func equalFoldASCII(a, b string) bool {
	for i := 0; i < len(b); i++ {
		if lowerByte(a[i]) != lowerByte(b[i]) {
			return false
		}
	}
	return true
}

// wordFirstTable 给出词表首字节（小写）集合，用于把「每个位置逐个词比较」压成
// 「首字节命中才比较」。
func wordFirstTable(words []string) byteSet {
	var t byteSet
	for _, w := range words {
		t[lowerByte(w[0])] = true
	}
	return t
}

// hasAnyWordFold 大小写无关判定 s 是否含 words 中任意一个（ASCII 折叠，零分配）。
// 表按**小写首字节**建（wordFirstTable），故这里必须用 lowerByte 去查 —— 用原始字节查表
// 会把 "AuthoriZAtion" 这类大写开头的关键字整段放走（回归信号：TestKeywordValueMaskedEverywhere 红）。
func hasAnyWordFold(s string, words []string, first byteSet) bool {
	for i := 0; i < len(s); i++ {
		if !first[lowerByte(s[i])] {
			continue
		}
		for _, w := range words {
			if len(s)-i >= len(w) && equalFoldASCII(s[i:i+len(w)], w) {
				return true
			}
		}
	}
	return false
}

// ── reSessTok / 令牌形态（Tier-0 与 Tier-1 共用一个门）──────────────────────
//
// tokenIdx 的键是首字节的**小写**形式（tokenRanges 用 EqualFold 比较），故门也按
// 大小写无关建表：宽的一侧才安全（reSessTok 本身是大小写敏感的，宽门只是少跳过几个值）。
var tokenFirst = func() byteSet {
	var t byteSet
	for b := 0; b < 256; b++ {
		if len(tokenIdx[lowerByte(byte(b))]) > 0 {
			t[b] = true
		}
	}
	return t
}()

// hasTokenPrefix 是 reSessTok 与 tokenRanges 的**共同必要条件**：必须含
// tokenPrefixes 里的某个形态（词干×分隔符 sk-/sk_/hf-/… 或 AKIA/ASIA/AIza/eyJ）。
//
// 令牌值在日志值里是极少数（一个字段要么是令牌、要么是普通文本）⇒ 这道门让绝大多数值
// 直接跳过整条令牌规则。注意：**不与 hasInvisible 合并** —— Cf 藏在前缀里时
// （fuzz 语料 "AK\u06ddIA000000000000"）原串不含任何前缀，maskTokenShapes 必须靠
// hasInvisible 那一支兜住，见调用点。
func hasTokenPrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		if !tokenFirst[s[i]] {
			continue
		}
		for _, p := range tokenIdx[lowerByte(s[i])] {
			if len(s)-i >= len(p) && equalFoldASCII(s[i:i+len(p)], p) {
				return true
			}
		}
	}
	return false
}

// ── reBearer 的必要条件 ──────────────────────────────────────────────────────
//
// reBearer 的形态是「关键字 \b + [":=] 分隔符 + 值」，故必要条件有两半：
// 含 ':' 或 '='，且含关键字（大小写无关）。词表覆盖 reBearer 的全部交替分支：
// bearer | token | api_key/api-key/apikey | authorization | passwd | password。
var (
	bearerWords = []string{"bearer", "token", "api_key", "api-key", "apikey", "authorization", "passwd", "password"}
	bearerFirst = wordFirstTable(bearerWords)
)

func hasBearerHint(s string) bool {
	if strings.IndexByte(s, ':') < 0 && strings.IndexByte(s, '=') < 0 {
		return false
	}
	return hasAnyWordFold(s, bearerWords, bearerFirst)
}

// ── reURLQuery 的必要条件 ────────────────────────────────────────────────────
//
// 形态是「[?&] + 关键字 + '=' + 值」，故必要条件有三半：含 '?' 或 '&'、含 '='、含关键字。
// 词表 {token, key, sig, secret} 是 reURLQuery 交替分支的**超集**
// （access_token 含 token · api_key 含 key · signature 含 sig）——超集即安全的必要条件。
var (
	urlWords = []string{"token", "key", "sig", "secret"}
	urlFirst = wordFirstTable(urlWords)
)

func hasURLQueryHint(s string) bool {
	if strings.IndexByte(s, '=') < 0 {
		return false
	}
	if strings.IndexByte(s, '?') < 0 && strings.IndexByte(s, '&') < 0 {
		return false
	}
	return hasAnyWordFold(s, urlWords, urlFirst)
}

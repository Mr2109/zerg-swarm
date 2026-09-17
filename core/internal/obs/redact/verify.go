package redact

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ── 回扫门禁（D7 / D18 / D19 / D20）─────────────────────────────────────────
//
// 门禁只做一件事：拿**原始事件**与**落盘行**比对，报出任何残留。它的价值全在「能不能红」：
//
//	· 投影不足 ⇒ 虚假保证。只比裸字节，`\/`、`\u002f`、全角、base64 四种藏法**全部漏检**
//	  （D19 实测）⇒ 故两侧都要过五类投影。
//	· needle 集与脱敏器不同源 ⇒ 误报（把 `//////` 当 base64 载荷判泄漏）⇒ 门禁被关掉
//	  （D18 实测）⇒ 故 needle 提取复用同一批正则、同一个 Classify、同一个用户折叠。
//	· 抓不到「上游重新拼回」⇒ 形同虚设（D20）⇒ 已写成用例：往干净行后追加
//	  `,"debug_original":"<原文>"` 必须报 leak。
//
// ── needle 精度口径（由 fuzz 逼出来的实测教训，共 6 条）────────────────────────
//
// 门禁是**子串比较**器，而子串比较天生会误报。误报的代价不是「噪音」，而是**门禁被关掉**（D18）
// ⇒ 下列口径每条都对应一次 fuzz 命中（语料锁在 testdata/fuzz/FuzzRedactValue/，用例锁在 verify_test.go）：
//
//	① 分类同源：ClassKeep 无 needle；ClassDrop/Mask 用整值；ClassScan 用模式 needle。
//	② needle 下限：≥4 字节；单字节重复（"0000000"）不产 needle —— 与行里普通内容无法区分。
//	③ 关键字类规则：值有区分度（≥8 字节且 ≥4 个不同字节）才用值；否则退化成整条 construct
//	   （关键字+"="+值）；值 < 4 字节干脆不产（"&token=[" 与保留的关键字前缀必然重合）。
//	④ 编码递归只在**解码结果命中规则**（maybeSensitiveInner）时进行 —— 与脱敏器同策略；
//	   否则门禁与输出两侧会同时「解码出」同一段垃圾并互相印证（"hf-0000…" 的长数字串）。
//	⑤ **不对称比较**：needle 侧只按原样（+JSON 反转义）比；输出侧才过五类投影，且输出侧先还原
//	   转义、编码类投影只收「解出来像文本」的结果。两侧都过投影等于自我印证（脱敏器自己也做
//	   归一化）⇒ 一串假 leak。
//	⑥ 比较按**字节**：只有「规则本身大小写无关」的 needle（用户折叠 (?i)）才做折叠比较。

// ── 五类投影 ────────────────────────────────────────────────────────────────

var reJSONU = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

// jsonUnescapeRaw 还原写者可能在字符串**内部**做过的转义：\/、\uXXXX、\n、\t、\"、\\ ——
// 这正是「查看原始日志」看到的形态。
func jsonUnescapeRaw(s string) string {
	s = reJSONU.ReplaceAllStringFunc(s, func(m string) string {
		v, err := strconv.ParseUint(m[2:], 16, 32)
		if err != nil {
			return m
		}
		return string(rune(v))
	})
	r := strings.NewReplacer(`\/`, `/`, `\n`, "\n", `\t`, "\t", `\r`, "\r", `\"`, `"`, `\\`, `\`)
	return r.Replace(s)
}

func b64decodeAll(s string) string {
	return reB64Run.ReplaceAllStringFunc(s, func(m string) string {
		pad := m
		if r := len(pad) % 4; r != 0 {
			pad += strings.Repeat("=", 4-r)
		}
		for _, dec := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if b, err := dec.DecodeString(pad); err == nil {
				return string(b)
			}
		}
		return m
	})
}

// stripFormat 是门禁侧的「NFKC + 去 Cf」投影：Cf/零宽字符**直接删掉**（人眼看不见它们，
// 而隐藏原文的人正是靠这一点：日志查看器里 "/Users/fuzz\u200b01" 看着就是 "~"）。
//
// 注意两侧的取法**故意不同**：脱敏器侧用 foldWidth（把 Cf 换成空格以保住词边界，见 redact.go 的
// 说明），门禁侧用 stripFormat（删掉以还原「读者看到的形态」）。两者都只改不可见字符，
// 不会把普通内容变成秘密，也不妨碍真泄漏的检出（真泄漏在还原后的明文里必然在场）。
func stripFormat(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			return r - 0xFEE0
		case r == 0x3000:
			return ' '
		}
		if unicode.Is(unicode.Cf, r) || r == 0xFEFF {
			return -1
		}
		return r
	}, s)
}

// urldecodeGated / b64decodeGated：与**脱敏器同一判据**的解码（同源，D18）。
//
// 为什么必须同判据：脱敏器只在「解码结果像文本**且**命中规则」时才动它；门禁若用宽松的解码
// （只看像不像文本），就会从**脱敏器故意没动**的 run 里「还原」出一个 needle 的形状来 —— fuzz 实测
// 3.7M 次执行命中过：同一字符串里两个 base64 run，第一个被遮、第二个因解码里夹了控制字符
// （可打印率 89.5% < 90%）被放过，而门禁把第二个也解了 ⇒ 报出一个文件里并不存在的残留。
// 反之，真泄漏（脱敏器失败没遮）的 run 一定满足脱敏器的判据 ⇒ 门禁照样能解出来比对。
func urldecodeGated(s string) string {
	if dec := urldecodeAll(s); dec != s && maybeSensitiveInner(dec) {
		return dec
	}
	return s
}

func b64decodeGated(s string) string {
	return reB64Run.ReplaceAllStringFunc(s, func(m string) string {
		pad := m
		if r := len(pad) % 4; r != 0 {
			pad += strings.Repeat("=", 4-r)
		}
		for _, dec := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawURLEncoding, base64.RawStdEncoding} {
			b, err := dec.DecodeString(pad)
			if err != nil || !printable(b) || !maybeSensitiveInner(string(b)) {
				continue
			}
			return string(b)
		}
		return m
	})
}

// Projections 返回 s 在写者/读者/日志查看器眼里可能呈现的每一层视图（D19 的五类投影）：
//
//	① raw（裸字节）            ② JSON 反转义（\/ 与 \uXXXX）
//	③ NFKC+去 Cf（全角同形字/零宽）  ④ percent-decode            ⑤ base64-decode
//
// 以及它们的组合。**只比裸字节会给出虚假保证**，故两侧都必须过这里。
//
// 编码类投影（④⑤）只在**解出来像文本**时才计入：否则长数字串/随机串的「垃圾解码」会在 needle 侧
// 与输出侧同时出现并互相印证，报出一个根本不存在于任何视图里的秘密（fuzz 实测：needle 与输出行
// 里各自有一段 24 个 0 的 run，两边 base64 解码都得到 `\xd3M4\xd3M4…` ⇒ 报假 leak）。
// 误报会让门禁被关掉（D18）；而真泄漏（编码藏起来的路径/令牌）解码后都是可读文本，不受影响。
//
// 说明：③ 用的是 stripFormat 的零依赖近似（全角↔半角 + 去 Cf），不是完整 NFKC（见 redact.go）。
func Projections(s string) []string {
	fold := stripFormat(s)
	unesc := jsonUnescapeRaw(s)
	views := []string{
		s,
		fold,
		unesc,
		stripFormat(unesc),
	}
	// ④ percent-decode
	pctA, pctB := urldecodeGated(fold), urldecodeGated(stripFormat(unesc))
	views = appendIfText(views, fold, pctA)
	views = appendIfText(views, stripFormat(unesc), pctB)
	// ⑤ base64-decode
	views = appendIfText(views, pctA, b64decodeGated(pctA))
	views = appendIfText(views, pctB, b64decodeGated(pctB))
	// JSON 往返视图：日志查看器展示一个「已解析字段」时的形态。
	if enc, err := json.Marshal(s); err == nil {
		var back string
		if err := json.Unmarshal(enc, &back); err == nil && back != s {
			views = append(views, back)
		}
	}
	return views
}

// appendIfText 只在「解码确实改变了内容且解出来像文本」时把视图计入（见 Projections 的说明）。
func appendIfText(views []string, src, decoded string) []string {
	if decoded == src || !printable([]byte(decoded)) {
		return views
	}
	return append(views, decoded)
}

// ── needle 提取（与脱敏器同源）──────────────────────────────────────────────
//
// 只比整值会漏掉「半截泄漏」（把 /Users/x 拼进一句话里），对刻意保留的字段又会误报。故按分类走：
//
//	ClassKeep      → 无 needle（门禁必须与脱敏器同策略：保留的字段不该被门禁指控）
//	ClassDrop/Mask → 整个值必须不再出现
//	ClassScan      → 值里**每个模式形态的秘密**都是 needle

// maxDecodeDepth 是 needle 提取的递归上限：解码本身是攻击面（D5），超限即停。
const maxDecodeDepth = 3

// needleItem 是一条 needle 及其**比较口径**。
//
// 比较是**不对称**的（这是本门禁最重要的一条设计决定，由 fuzz 逼出来）：
//   - needle 侧只按**原样**（外加 JSON 反转义）比较 —— needle 就是「上游原始事件里的那串字节」，
//     它被写上盘时就该是这个样子；
//   - 输出侧才过**五类投影** —— 因为秘密可能以 `\/`、`\u002f`、全角、percent、base64 等形态藏在
//     落盘行里，读者会把它还原出来。
//
// 曾经两侧都过投影，结果是**自我印证**：脱敏器自己会对值做归一化（全角→ASCII、Cf→空格、
// 非法字节→U+FFFD），于是「needle 的归一化形态」与「被归一化后落盘的行」必然重合 ⇒ 一串假 leak
// （fuzz 实测：`&token=\u0605\u0605[`、`&token=\u0605\xd800001` 都在这一条上翻车）。
//
// fold：是否允许大小写折叠比较。口径必须**与规则自身的大小写语义一致**（D18 的「同源」）：
// 只有用户折叠规则是大小写无关的（reUser 带 (?i)）；路径/邮箱/令牌/关键字 construct 都是
// 大小写敏感的规则，拿它们的 needle 去折叠比较必然误报 —— fuzz 实测两例：
//
//	in="/hoMe/0/home/0"        ⇒ needle "/home/0" 折叠后命中行里的普通文本 "/hoMe/0"
//	in="/home/AAA/hoMe/AAA0"   ⇒ needle "/home/AAA" 折叠后命中未命中的 "/hoMe/AAA0"
type needleItem struct {
	s    string
	fold bool
}

func patternNeedles(s string) []needleItem { return patternNeedlesDepth(s, 0) }

// keywordNeedle 给出关键字类规则（Bearer / URL query）的 needle：
//   - 值有区分度（见 sufficientSignal）且不是裸关键字 ⇒ 直接取**捕获的值**（它才是敏感部分）；
//   - 否则退化成**整条 construct**（"authorization: ***"、"&token=AAAa"）；
//   - 值短到 < 4 字节 ⇒ **不产 needle**：那时 construct 只剩「关键字 + 一两个字符」，与脱敏后
//     **故意保留**的关键字前缀必然重合（fuzz 实测 in="&token=[" ⇒ 行里 "&token=[REDACTED]" 含
//     "&token=["，报假 leak）。四字节是门禁的 needle 下限口径，与 add() 一致。
//
// 为什么不能拿弱值当 needle：fuzz 实测 "AAAa"/"000I00"/"0000000" 这类值与整行比子串会被行里
// 普通文本的 "aaaa" 命中 —— 误报 ⇒ 门禁被关掉（D18）。而 construct 是脱敏器**真正保证被破坏**
// 的东西，拿它当 needle 更精确。
func keywordNeedle(g []string) needleItem {
	if !reBareKeyword.MatchString(g[2]) && sufficientSignal(g[2]) {
		return needleItem{s: g[2]}
	}
	if len(g[2]) < 4 {
		return needleItem{}
	}
	return needleItem{s: g[0]}
}

// sufficientSignal 判定一个串是否有**足够区分度**：长度 ≥ 8 且不同字节 ≥ 4（真凭据/真路径的形态）。
// 用途：关键字类规则的捕获值能否直接当 needle（不够 ⇒ 退化成整条 construct）。反例（fuzz 实测）：
// "AAAa"、"000I00"、"0000000" —— 拿它们和整行比子串，会被行里普通文本的 "aaaa" 命中 ⇒ 假 leak。
func sufficientSignal(v string) bool {
	if len(v) < 8 {
		return false
	}
	var seen [256]bool
	uniq := 0
	for i := 0; i < len(v); i++ {
		if !seen[v[i]] {
			seen[v[i]] = true
			uniq++
			if uniq >= 4 {
				return true
			}
		}
	}
	return false
}

// isDeletionOf 判定 small 是否由 big **只删字节**得到（顺序保持、不替换）。
// 这样的投影比原串更弱（信息只少不多），拿它比子串容易与脱敏器故意保留的片段重合（见调用点）。
func isDeletionOf(big, small string) bool {
	if len(small) >= len(big) {
		return false
	}
	j := 0
	for i := 0; i < len(big) && j < len(small); i++ {
		if big[i] == small[j] {
			j++
		}
	}
	return j == len(small)
}

// distinctive 判定 needle 是否有区分度：**单字节重复**的 needle（"0000000"、"aaaa"）无法与
// 落盘行里恰好出现的同形普通内容区分 ⇒ 拿它比子串必然误报。
//
// 这条是被 fuzz 逼出来的（真缺陷，不是假想）：输入 "AuthoriZAtion:0000000 0000…0000" 里，
// 脱敏器确实把 Bearer 值遮成了 [REDACTED]，但行里另有一段更长的 0 串（那是同一字符串里的
// 普通内容）⇒ 拿捕获值 "0000000" 比子串就报假 leak。误报会把门禁关掉（D18：误报 ⇒ 没人信 ⇒
// 门禁被关掉，那才是真正的安全失效）⇒ 这类 needle 一律不提取。
//
// 边界：ClassMask / ClassDrop 的**整值** needle 不走这条过滤 —— 整值必须逐字消失，与熵无关。
func distinctive(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return true
		}
	}
	return false
}

func patternNeedlesDepth(s string, depth int) []needleItem {
	if depth > maxDecodeDepth {
		return nil
	}
	var out []needleItem
	add := func(x string) {
		// 4 字节以下比中概率太高，且不构成可读的秘密；单字节重复的没有区分度（见 distinctive）。
		if len(x) >= 4 && distinctive(x) {
			out = append(out, needleItem{s: x})
		}
	}
	addItem := func(it needleItem) {
		if len(it.s) >= 4 && distinctive(it.s) {
			out = append(out, it)
		}
	}
	for _, m := range reHomeDir.FindAllString(s, -1) {
		add(m)
	}
	for _, m := range reEmail.FindAllString(s, -1) {
		add(m)
	}
	for _, m := range reIPv4.FindAllString(s, -1) {
		add(m)
	}
	for _, m := range reSessTok.FindAllString(s, -1) {
		add(m)
	}
	for _, g := range reBearer.FindAllStringSubmatch(s, -1) {
		addItem(keywordNeedle(g))
	}
	for _, g := range reURLQuery.FindAllStringSubmatch(s, -1) {
		addItem(keywordNeedle(g))
	}
	// 编码载荷：**不**把编码段本身当 needle（输出的投影会解码它，明文 needle 已覆盖）；
	// 只递归进「解码结果像文本」的那一支 —— 否则 `//////` 这种普通文本会被当成泄漏（D18）。
	// 且必须**与脱敏器同策略**：脱敏器只在解码结果命中规则（maybeSensitiveInner）时才整段遮蔽，
	// 故递归也只在这种解码上进行。否则会造出脱敏器从不打算遮的 needle：fuzz 实测
	// "hf-0000…\xbe\xbe\xbe0000…" 那种长数字串的垃圾解码 ⇒ 门禁与输出两侧同时「解码出」同一段
	// 垃圾并互相印证，报出一个根本不存在的秘密（纯误报 ⇒ 门禁被关掉）。
	for _, m := range reB64Run.FindAllString(s, -1) {
		if d := b64decodeAll(m); d != m && printable([]byte(d)) && maybeSensitiveInner(d) {
			for _, n := range patternNeedlesDepth(d, depth+1) {
				addItem(n)
			}
		}
	}
	if dec := urldecodeAll(s); dec != s && maybeSensitiveInner(dec) {
		for _, n := range patternNeedlesDepth(dec, depth+1) {
			addItem(n)
		}
	}
	if st := userFold.Load(); st != nil && strings.Contains(strings.ToLower(s), strings.ToLower(st.name)) {
		// 用户折叠是**唯一大小写无关的规则**（reUser 带 (?i)）⇒ 只有这条 needle 允许折叠比较。
		out = append(out, needleItem{s: st.name, fold: true})
	}
	return out
}

// Leak 是一条残留：原文里 Path 处的秘密以 Needle 形态出现在落盘行里。
type Leak struct {
	Path   string
	Needle string
}

// VerifyNoResidue 是门禁本体：original 是**内存中的原始事件**，emitted 是**已写出的整行字节**。
// 返回空切片即干净。CI 全量跑；运行时 paranoid 模式只对采样事件跑（成本见设计附件 §三）。
func VerifyNoResidue(original map[string]any, emitted []byte) []Leak {
	return VerifyNoResidueString(original, string(emitted))
}

// VerifyNoResidueString 同 VerifyNoResidue，但收字符串行（便于工具与用例直接喂文本）。
func VerifyNoResidueString(original map[string]any, emitted string) []Leak {
	type needle struct {
		path string
		item needleItem
	}
	var needles []needle

	var walk func(v any, path, key string)
	walk = func(v any, path, key string) {
		switch t := v.(type) {
		case string:
			switch Classify(key) {
			case ClassKeep:
				return // 门禁与脱敏器同源：保留字段不是泄漏
			case ClassDrop, ClassMask:
				needles = append(needles, needle{path, needleItem{s: t}})
			default:
				for _, n := range patternNeedles(t) {
					needles = append(needles, needle{path, n})
				}
			}
		case map[string]any:
			for k, vv := range t {
				walk(vv, path+"."+k, k)
			}
		case []any:
			for i, vv := range t {
				walk(vv, path+"["+strconv.Itoa(i)+"]", key)
			}
		}
	}
	walk(original, "", "")

	// 输出侧视图一律从**转义还原后**的文本出发。裸字节视图会把 JSON 转义序列本身算进去：
	// fuzz 实测输入 "\x10.0.0.0 0.0.0.0" ⇒ 落盘行里 `\u0010` 的尾零与紧随的 ".0.0.0" 拼出了
	// needle 的字节 "0.0.0.0"，但那段秘密在任何解码视图里都不存在 —— 纯误报，而误报会让门禁
	// 被关掉（D18）。还原后真泄漏照样在场（`\/` 转义、`\u002f`、全角、percent、base64 都已验过。
	// 「grep 原始文件」视角由 needle 侧保留 raw 形态 + 这里解码后仍命中的明文覆盖）。
	outRaw := Projections(jsonUnescapeRaw(emitted))
	outLow := make([]string, len(outRaw))
	for i, v := range outRaw {
		outLow[i] = strings.ToLower(v)
	}

	var leaks []Leak
	for _, n := range needles {
		// needle 侧：**只按原样 + JSON 反转义**比较（不对称口径，见 needleItem 的说明）。
		views := []string{n.item.s}
		if u := jsonUnescapeRaw(n.item.s); u != n.item.s {
			views = append(views, u)
		}
		for _, nv := range views {
			if len(nv) < 4 {
				continue
			}
			// 反转义视图若只是把 needle 删掉若干字节（`\/` → `/`），比原串更弱，跳过。
			if len(nv) < len(n.item.s) && isDeletionOf(n.item.s, nv) {
				continue
			}
			// 大小写折叠比较只对**规则本身就是大小写无关**的 needle 开放（见 needleItem.fold）。
			nvLow := ""
			if n.item.fold {
				nvLow = strings.ToLower(nv)
			}
			for i, ov := range outRaw {
				if strings.Contains(ov, nv) || (nvLow != "" && strings.Contains(outLow[i], nvLow)) {
					leaks = append(leaks, Leak{Path: n.path, Needle: nv})
					goto next
				}
			}
		}
	next:
	}
	return leaks
}

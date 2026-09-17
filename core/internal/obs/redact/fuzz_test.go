package redact

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzRedactValue：对**任意输入**都必须成立的性质（D7）。没人会想到去写「全角 + 零宽 + percent +
// base64 四层嵌套」这种用例 —— fuzz 会。原型那一轮找出 4 个真 bug（无效 UTF-8 直通、裸用户名漏脱敏、
// 门禁 `//////` 误报、sk 形态策略不一致），本包把它们变成了可回归的性质；本轮入库时 fuzz 又命中 8 个
// 「门禁精度 / 幂等」缺陷（见 testdata/fuzz/FuzzRedactValue/ 下的语料，每条都从源头修掉并锁成用例）。
//
// 说明：RedactValue 对无效 UTF-8 是**原样通过**的（fuzz 第一轮就找到 "\xff\xfe"）。这是对的 ——
// 写者用 encoding/json 编码，非法字节会被换成 U+FFFD ⇒ 「输出必须合法 UTF-8」这条性质属于
// **落盘行**，不属于值。
func FuzzRedactValue(f *testing.F) {
	seeds := []string{
		"~/x", "／Users／ｆｕｚｚ０１", "/Users/fuzz\u200b01", "%2FUsers%2FMr2109",
		"cfg=L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
		"Authorization: " + "Bearer " + tokSk, tokHf,
		"a@b.com", "<controller-ip>", "open /home/alice/.netrc", "plain text",
		"%25%32%46", "\xff\xfe", strings.Repeat("/", 64), "",
		"eyJhbG" + "ciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln", "hf-abcdefghijklmnop",
		"glpat_abcdefghijklmnop", "xoxb-" + "1234567890123", "AKIA" + "IOSFODNN7EXAMPLE",
		"Mr2109", "path ////// ok", "https://x/y?token=aaaaaaaaaaaa",
		// 下面是本轮 fuzz 命中的 8 例（正在此处留种，防止回归）：
		"AuthoriZAtion:0000000 " + strings.Repeat("0", 28), // 低熵 needle 误报
		"AuthoriZAtion:000I00 000I00",                      // 值的第二份拷贝
		"aaaaaaaa&token=AAAa",                              // 低熵值当 needle
		"/hoMe/0" + "/home/0",                              // 大小写折叠误报
		"&token= 00000000!0",                               // 一轮不等于不动点
		"0.0.0.0 and A0.0.0.0",                             // 规则未命中的包含式拷贝
		"\x10" + ".0.0.0 0.0.0.0",                          // JSON 转义拼接出 needle 字节
		"hf-" + strings.Repeat("0", 21) + "\xbe\xbe\xbe" + strings.Repeat("0", 24), // 垃圾解码互相印证
		"&token=\u0605", // 投影塌缩成「故意保留的关键字前缀」
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// (a) 不许 panic：写路径上的 panic 会带走整条调试流。
		out := RedactValue(s)
		line, err := json.Marshal(RedactEvent(map[string]any{"err": s}))
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if !utf8.Valid(line) {
			t.Fatalf("落盘行不是合法 UTF-8：in=%q line=%q", s, line)
		}
		if !utf8.ValidString(out) && utf8.ValidString(s) {
			t.Fatalf("合法输入产生了非法 UTF-8 值：in=%q out=%q", s, out)
		}

		// (b) 幂等：f(x)==f(f(x))。这条不是洁癖 —— 重试/重放/二次导出路径都依赖它。
		if again := RedactValue(out); again != out {
			t.Fatalf("值管线不幂等：\n in=%q\n1st=%q\n2nd=%q", s, out, again)
		}
		if string(MarshalRedacted(map[string]any{"err": s})) != string(MarshalRedacted(map[string]any{"err": out})) {
			t.Fatalf("事件级不幂等：in=%q", s)
		}

		// (c)(d) 原文里的每个「秘密形态」都不许在任何投影下存活（不是只比裸字节）。
		//
		// 与门禁同一份比较口径（这也是 D18「同源」的一部分）：
		//   ① 不对称：needle 侧只按原样 + JSON 反转义比；输出侧才过五类投影。两侧都过投影会
		//      自我印证（脱敏器自己也做归一化），fuzz 实测出过一串假 leak。
		//   ② 单字节重复的 needle 不参与（与行里普通内容无法区分）。
		//   ③ 大小写按字节比（折叠只给「规则本身大小写无关」的 needle；这里的规则都是敏感的）。
		assertNoResidue := func(what, m string) {
			if len(m) < 4 || !distinctive(m) {
				return
			}
			// needle 视图与门禁**同源**（needleViews）：含非法 UTF-8 的 needle 换成落盘形态 ——
			// 落盘行一定是合法 UTF-8，拿原始字节比只会撞出「合法内容的字节窗口」误报。
			views := needleViews(m)
			outViews := Projections(jsonUnescapeRaw(string(line)))
			for _, nv := range views {
				if len(nv) < 4 || weakViewOf(m, nv) {
					continue
				}
				for _, p := range outViews {
					if strings.Contains(p, nv) {
						t.Fatalf("%s 泄漏：in=%q match=%q needleView=%q line=%q projection=%q",
							what, s, m, nv, line, p)
					}
				}
			}
		}
		for _, m := range reHomeDir.FindAllString(s, -1) {
			assertNoResidue("path", m)
		}
		for _, m := range reEmail.FindAllString(s, -1) {
			assertNoResidue("email", m)
		}
		for _, m := range reSessTok.FindAllString(s, -1) {
			assertNoResidue("token", m)
		}
		for _, g := range reURLQuery.FindAllStringSubmatch(s, -1) {
			// ① construct（"&token=值"）必须被破坏 —— 脱敏器的核心承诺。值 < 4 字节时 construct
			//    只剩「关键字 + 一两个字符」，与脱敏后保留的关键字前缀必然重合（fuzz 实测
			//    in="&token=[" ⇒ 假 leak）⇒ 与门禁同口径：不产这种 check。
			if len(g[2]) >= 4 {
				assertNoResidue("url-query-construct", g[0])
			}
			// ② 值本身：只在它有区分度**且不是裸关键字**时才要求「任何拷贝都不许存活」——
			//    低熵值（"AAAa"/"aaaa"）与普通文本无法区分；值是关键字本身（"Authorization"）时
			//    脱敏器按设计保留关键字 ⇒ 要求它消失是错的（fuzz 实测命中过这条口径错）。
			if sufficientSignal(g[2]) && !reBareKeyword.MatchString(g[2]) {
				assertNoResidue("url-query-value", g[2])
			}
		}
		for _, g := range reBearer.FindAllStringSubmatch(s, -1) {
			if len(g[2]) >= 4 {
				assertNoResidue("bearer-construct", g[0])
			}
			if sufficientSignal(g[2]) && !reBareKeyword.MatchString(g[2]) {
				assertNoResidue("bearer-value", g[2])
			}
		}

		// (e) 门禁必须与脱敏器一致：干净行不许报 leak，否则门禁是假的（误报 ⇒ 被关掉 ⇒ 真失效）。
		if leaks := VerifyNoResidue(map[string]any{"err": s}, line); len(leaks) > 0 {
			t.Fatalf("门禁报残留（与脱敏器不同源）：%+v in=%q out=%q", leaks, s, out)
		}
	})
}

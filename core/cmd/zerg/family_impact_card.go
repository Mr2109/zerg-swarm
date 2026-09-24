// family_impact_card.go —— 波纹卡片（`A3`）：条目四字段 · `why` 闭集**六选一** · 四级整数排序 ·
// 按档裁 · 超预算三件 · §3.7 公开面行 · §3.8 可逆性行。
//
// 依据（逐字）：设计-变更影响面-v1.6 §3.1（三行骨架 + `items[]` ≤ 12 + 四字段 + `why` 硬门槛 +
// 单条 ≤ 3 行/≤ 60 token + 条件行 + v1.6 把 `why` 从五选一扩成**六选一**）· §3.2（四级排序：会红 →
// 图距离 → 词法命中数 → 义近分，**全是整数比较**）· §3.3（超预算**按档裁 + 显式声明**，三件同批，
// **绝不截字符串**）· §3.4（尺寸 / 分层 / 时机）· §3.5（**两条铁律**）· §3.7（公开面波纹行）·
// §3.8（可逆性行 · 三档退法）· §4.1（两档输出）· §4.4（1.2k 的字节口径）；
// 任务单-影响面实施-20260922 §二 `A3` 八字段与判据①–④。
//
// 落点为什么在 stderr：人面**恒三行**是 `A1` 判据③（`A1`/`A2` 两件测试真跑对拍：stdout 非空行数
// 必须恰为 3）⇒ 卡片**不能**往 stdout 加行；`A2` 已把「层表/读数」放 stderr ⇒ 卡片块（每一条
// 四字段逐条 + 账 + 裁明细 + 退法）与层表并列落 stderr，而 `--json` 的 `items[]` 就是**卡片条目
// 本体**（§7.4 逐字「卡片的条目就是 `items[]` 的每一项」，故条数上限 12 直接落在 `items[]` 上）。
//
// 诚实边界（照实标，不假装 —— 两条）：
//
//	① §3.3 的三件（`truncated=true` / `warnings[]` 一条「已裁 N 条」/ `meta.how_to_restore`）
//	   在**六键包封里没有落点**：`emitEnvelopeWith`（`main.go`）把 `warnings` 恒写 `[]`、
//	   `truncated` 恒写 `false`、`meta` 只写 `count/source/changed`（+`node`/`idempotency_key`）。
//	   `A1`/`A2` 的红线「**不改 `emitEnvelope*`**」本批未解禁 ⇒ 本件把三件落在**卡片块**（同批、
//	   同值、逐字同形）并把「包封面未接通」记成 `CLI 缺口`（**不偷偷改包封** ✗）。
//	② 义近分（第四级）**今天恒 0**：第 ⑤ 层「未建索引 ⇒ 不取数」（`A2` 现读）⇒ 卡片里没有
//	   义近条目 ⇒ 第四级只作同分 tie-break，现跑观察不到（口径写明，**不编数** ✗）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ---- 尺寸上限（§3.1/§3.4 的四条硬线 · 全是常量 —— 「**上限是死的**」）--------------------------

const (
	impactItemMax      = 12   // §3.1：`items[]` 最多 12 条
	impactTokenMax     = 1200 // §3.1：模型档总量 ≤ 1.2k token（**不许**为了塞新行抬到 1.3k · §4.1）
	impactItemLineMax  = 3    // §3.1：单条 ≤ 3 行
	impactItemTokenMax = 60   // §3.1：单条 ≤ 60 token
	// §4.4 现读口径逐字：「模型档预算 **1.2k token ÷ 4 B/token ≈ 4,800 字节**，而符号层全图
	// 39,361,052 字节 ⇒ 两档尺寸差 ≈ 8,200 倍」。
	impactBytesPerToken = 4
)

// impactWhySix —— `why` 的**闭集六选一**（§3.1 v1.6「`why` 闭集扩到六档」逐字六项）。
//
// ★ 「词法（**含形近**）」是**一格**：`A2` 层 ④b 的形近条目在进卡片时一律归一成 `词法`
// （来路文字仍留在 `how` 里，不丢证据）⇒ 卡片里**不会出现第七个 `why` 键** ✗（任务单 `A3` 红线）。
var impactWhySix = []string{"包反向", "调用边", "契约", "词法", "义近", "公开面"}

// impactWhyNormalize 把 `A2` 六层给出的 `why` 归一进六选一；不在闭集里的返回空串（⇒ 不进卡片）。
func impactWhyNormalize(why string) string {
	switch strings.TrimSpace(why) {
	case "形近":
		return "词法" // §3.1：六选一里没有「形近」这一格 —— 它并进「词法（含形近）」
	case "包反向", "调用边", "契约", "词法", "义近", "公开面":
		return strings.TrimSpace(why)
	}
	return ""
}

// ---- token 估算（两个口径都报 · 无新依赖）----------------------------------------------------

// impactTokenEstimate 估一段文本的 token 数，返回 **(保守口径, 设计现读口径)**。
//
//	· 设计现读口径（§4.4 逐字）= 字节数 ÷ 4（向上取整）；
//	· 保守口径 = 表意字（CJK / 全角）每个算 **1 token** + 其余字节每 4 个算 1 token。
//	  为什么要有它：一个汉字占 **3 字节**，按「÷4」只算 0.75 token ⇒ **中文密度被低估**；
//	  §3.1 的上限是死的，故**裁的时候用保守口径**（两个口径都不许破）。
func impactTokenEstimate(s string) (conservative, bytesOver4 int) {
	n := len(s) // 字节
	cjk := 0
	for _, r := range s {
		if impactIsWide(r) {
			cjk++
		}
	}
	rest := n - 3*cjk
	if rest < 0 {
		rest = 0
	}
	return cjk + (rest+impactBytesPerToken-1)/impactBytesPerToken, (n + impactBytesPerToken - 1) / impactBytesPerToken
}

// impactIsWide 表意字 / 全角（CJK 汉字 · 假名 · CJK 标点 · 全角形）—— 这些字符 UTF-8 占 3 字节。
func impactIsWide(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// ---- 卡片本体 ---------------------------------------------------------------------------------

// impactCardEntry 一条进/出卡片的条目（带四级排序键与裁序档）。
type impactCardEntry struct {
	Row  map[string]string
	Tier int // 裁序档：1 最先裁 … 4 最后才裁（§3.3 + §3.2 补全，见 impactCardTier）
	Tok  int // 该条渲染后的保守 token
	Lex  int // 词法命中数（§3.2 第三级）
}

// impactCard 一张卡片（条目 + 账 + 裁明细 + 预算三件）。
type impactCard struct {
	Items      []map[string]string // ≤ 12 条 · 四字段 · 已按四级排序 · 已按档裁
	Raws       int                 // 六层全量条数（裁前）
	NoWhy      int                 // 无 `why` 未进卡（§3.1 硬门槛）
	UnknownWhy int                 // `why` 不在闭集未进卡
	Oversize   int                 // 单条超 60 token 未进卡（**不截字符串** ✗）
	SemRed     int                 // 义近条目带 red ⇒ 被清空（§3.5 铁律）
	CutTier    [5]int              // 裁序档 1–4 各裁了几条（index 0 不用）
	Truncated  bool
	Warnings   []string
	HowRestore string // `meta.how_to_restore` 的取值（同值落卡片块）

	FixedTokens int // 三行骨架 + 条件行的保守 token
	BodyTokens  int // 条目之和（保守）
	TotalTokens int // FixedTokens + BodyTokens
	BytesTokens int // 同上的「设计现读口径」（字节 ÷ 4）

	RedN, Dist1N, LexMax, SimN, PubN int
}

// 裁序档（§3.3 三档 + 一条补全 · 见 `impactCardTier`）与图距离哨兵 —— **具名常量**，一律不写裸数字
// （门⑪ T3：命令面源码里的裸数字会被当成退出码，退码真源在 `exitcodes.go`）。
const (
	impactCutTierSim = 1 // 档①：`red` 空且 `why`=义近（最先裁）
	impactCutTierLex = 2 // 档②：`red` 空且 `why`=词法（含形近）
	impactCutTierDet = 3 // 档③：`red` 空且 `why` 是别的确定性面
	impactCutTierRed = 4 // 档④：`red` 非空（置顶且最后才裁）

	impactDistOneHop = 1  // §3.2 第二级：`callgraph` 反向边上 1 跳 = 直接调用者
	impactDistAbsent = 99 // 取不到图距离的哨兵（口径写明 ⇒ 排在 1 跳之后）

	impactSimNone = 0 // §3.2 第四级：义近分今天恒 0（⑤ 未建索引 ⇒ 无义近条目）

	impactRankRed    = 1 // 四级排序键①：`red` 非空（会红）
	impactRankNonRed = 0 // 四级排序键①：`red` 为空
)

// impactCardTier 裁序档（§3.3 三档逐字 + 一条**补全**）：
//
//	档①：`red` 为空 **且** `why` 是「义近」（§3.3 第 1 条）；
//	档②：`red` 为空 **且** `why` 是「词法 / 形近」（§3.3 第 2 条 · 归一口径下就是 `词法`）；
//	档③：`red` 为空 **且** `why` 是别的确定性面（包反向 / 调用边 / 契约 / 公开面）——
//	     §3.3 的三条没点名它们（它只写了「义近 / 词法 / red 非空」三档）⇒ 本件按 §3.2 的名次
//	     把它们排在**词法之后、会红之前**（确定性面里最低名次是「词法」，故这一档比词法更不该裁）；
//	档④：`red` 非空（「会红」那一行**置顶且最后才裁** —— 裁序里排最后 + 排序里排最前）。
func impactCardTier(it map[string]string) int {
	if it["red"] != "" {
		return impactCutTierRed
	}
	switch it["why"] {
	case "义近":
		return impactCutTierSim
	case "词法":
		return impactCutTierLex
	}
	return impactCutTierDet
}

// impactCardLex 词法命中数（§3.2 第三级 · 口径写死）：同一**件**在层 ④（词法 + 形近）里的命中条数。
// 取不出件路径的条目记 0（口径写明，不猜）。
func impactCardLex(rows []map[string]string) map[string]int {
	cnt := map[string]int{}
	for _, r := range rows {
		if !strings.HasPrefix(r["what"], "文件级:") {
			continue
		}
		switch strings.TrimSpace(r["why"]) {
		case "词法", "形近":
			cnt[impactWhatPath(r["what"])]++
		}
	}
	return cnt
}

// impactWhatPath 从 `文件级:<件>:<行>` 里取件路径（`what` 恒带粒度前缀 —— `R40`）。
func impactWhatPath(what string) string {
	rest := strings.TrimPrefix(what, "文件级:")
	if i := strings.LastIndex(rest, ":"); i > 0 {
		if _, err := fmt.Sscanf(rest[i+1:], "%d", new(int)); err == nil {
			return rest[:i]
		}
	}
	return rest
}

// impactCardFixedTokens 骨架（三行 + 条件行）的保守 token —— 卡片预算把常量部分也算进去。
func impactCardFixedTokens(fixed string) int {
	c, _ := impactTokenEstimate(fixed)
	return c
}

// impactCardOf 由六层全量条目造一张卡片（纯函数：测试可喂坏输入 ⇒ 成对负控）。
//
// 五步，逐条对应判据：
//
//	① 过滤（§3.1 硬门槛）：没有 `why` ⇒ 不进；`why` 不在六选一闭集 ⇒ 不进；形近 ⇒ 归一成词法；
//	② 铁律（§3.5）：`why` 是「义近」的条目 `red` 一律清空（语义级**永不进「会红」行**）；
//	③ 单条尺寸（§3.1）：渲染后 > 60 token 或 > 3 行 ⇒ **整条不进**（**不截字符串** ✗ · §3.3）；
//	④ 排序（§3.2 四级整数）：会红 → 图距离 → 词法命中数 → 义近分（同分按 `what` 定序，可复现）；
//	⑤ 按档裁（§3.3）：先裁条数到 ≤ 12，再裁总量到 ≤ 1.2k（保守口径），裁序档 ①→④；
//	   裁了 ⇒ 同批给三件（`truncated=true` + `warnings[]`「已裁 N 条」+ `how_to_restore`）。
func impactCardOf(rows []map[string]string, fixed, tgtRaw string) impactCard {
	c := impactCard{FixedTokens: impactCardFixedTokens(fixed)}
	lex := impactCardLex(rows)

	kept := []impactCardEntry{}
	for _, r := range rows {
		w := strings.TrimSpace(r["why"])
		if w == "" {
			c.NoWhy++
			continue
		}
		nw := impactWhyNormalize(w)
		if nw == "" {
			c.UnknownWhy++
			continue
		}
		red := strings.TrimSpace(r["red"])
		if nw == "义近" {
			if red != "" {
				c.SemRed++ // 义近是概率 ⇒ 它带的 red 一律清空（§3.5 铁律）
			}
			red = ""
		}
		it := map[string]string{"what": r["what"], "why": nw, "how": r["how"], "red": red}
		tok := impactCardItemTokens(it)
		if tok > impactItemTokenMax || impactCardItemLines(it) > impactItemLineMax {
			c.Oversize++
			continue
		}
		kept = append(kept, impactCardEntry{Row: it, Tier: impactCardTier(it), Tok: tok, Lex: lex[impactWhatPath(it["what"])]})
	}

	// ④ 排序（§3.2）：全是整数比较，不引库。
	sort.SliceStable(kept, func(i, j int) bool { return impactCardLess(kept[i], kept[j]) })
	for _, e := range kept {
		switch {
		case e.Row["red"] != "":
			c.RedN++
		case e.Row["why"] == "公开面":
			c.PubN++
		}
		if e.Lex > c.LexMax {
			c.LexMax = e.Lex
		}
		if e.Row["why"] == "调用边" {
			c.Dist1N++ // 图距离 1 跳 = 直接调用者（§3.2 第二级）
		}
		if e.Row["why"] == "义近" {
			c.SimN++
		}
	}

	// ⑤ 按档裁：条数上限 → 总量上限。裁的时候**整条拿掉**（绝不截字符串 ✗）。
	for len(kept) > impactItemMax {
		kept = impactCardCutOne(kept, &c)
	}
	c.BodyTokens = impactCardBodyTokens(kept)
	c.TotalTokens = c.FixedTokens + c.BodyTokens
	for len(kept) > 0 && c.TotalTokens > impactTokenMax {
		kept = impactCardCutOne(kept, &c)
		c.BodyTokens = impactCardBodyTokens(kept)
		c.TotalTokens = c.FixedTokens + c.BodyTokens
	}
	_, c.BytesTokens = impactTokenEstimate(fixed)
	for _, e := range kept {
		_, b := impactTokenEstimate(impactCardItemText(e.Row))
		c.BytesTokens += b
	}

	cut := c.CutTier[1] + c.CutTier[2] + c.CutTier[3] + c.CutTier[4]
	// 「已裁 N 条」的 N = **按档裁掉的 + 单条超限整条不进的**（两者都是**预算**的结果 ·
	// §3.3 三件之一）；结构性的丢弃（无 `why` / `why` 不在闭集）**不计入** N —— 那是 §3.1 的
	// 硬门槛，不是预算，混进来会让「已裁 N 条」这个数指向两个东西。
	if cut > 0 || c.Oversize > 0 {
		c.Truncated = true
		c.Warnings = append(c.Warnings, fmt.Sprintf("已裁 %d 条", cut+c.Oversize))
		if c.Oversize > 0 {
			c.Warnings = append(c.Warnings, fmt.Sprintf("其中单条超 %d token 未进卡 %d 条（**不截字符串**：整条不进 · §3.3）", impactItemTokenMax, c.Oversize))
		}
		c.HowRestore = fmt.Sprintf("zerg impact %s --for-human（人档全文 · L2）", tgtRaw)
	}
	c.Items = make([]map[string]string, 0, len(kept))
	for _, e := range kept {
		c.Items = append(c.Items, e.Row)
	}
	if c.Warnings == nil {
		c.Warnings = []string{}
	}
	return c
}

// impactCardCutOne 按裁序档拿掉一条（档 ① 最先、档 ④ 最后；同档里拿名次最低的那一条）。
func impactCardCutOne(kept []impactCardEntry, c *impactCard) []impactCardEntry {
	for tier := 1; tier <= 4; tier++ {
		idx := -1
		for i := range kept { // 已排序 ⇒ 最后一个 = 名次最低
			if kept[i].Tier == tier {
				idx = i
			}
		}
		if idx >= 0 {
			c.CutTier[tier]++
			return append(kept[:idx:idx], kept[idx+1:]...)
		}
	}
	return kept
}

// impactCardLess 四级整数排序（§3.2）。同分按 `what` 定序 ⇒ **两跑逐字相同**（§九 共通判据 4）。
func impactCardLess(a, b impactCardEntry) bool {
	ar, br := boolInt(a.Row["red"] != ""), boolInt(b.Row["red"] != "")
	if ar != br {
		return ar > br // ① 会红（确定性最高的先给）
	}
	ad, bd := impactCardDist(a.Row), impactCardDist(b.Row)
	if ad != bd {
		return ad < bd // ② 图距离（1 跳 = 直接调用者）
	}
	if a.Lex != b.Lex {
		return a.Lex > b.Lex // ③ 词法命中数
	}
	if as, bs := impactCardSim(a.Row), impactCardSim(b.Row); as != bs {
		return as > bs // ④ 义近分（只作 tie-break · 今天恒 0：⑤ 未建索引）
	}
	return a.Row["what"] < b.Row["what"]
}

// impactCardDist 图距离（§3.2 第二级）：`callgraph` 反向边上 **1 跳 = 直接调用者** ⇒ 取 1；
// 其余条目**取不到图距离**，按口径取哨兵 **99**（排在 1 跳之后 —— 口径写明，不猜一个距离出来）。
func impactCardDist(it map[string]string) int {
	if it["why"] == "调用边" {
		return impactDistOneHop
	}
	return impactDistAbsent
}

// impactCardSim 义近分（§3.2 第四级 · embedding 余弦的整数化）：**今天恒 0**（第 ⑤ 层未建索引
// ⇒ 卡片里没有义近条目）⇒ 它只作同分 tie-break，现跑观察不到（照实写，不编一个分）。
func impactCardSim(it map[string]string) int { return impactSimNone }

func boolInt(b bool) int {
	if b {
		return impactRankRed
	}
	return impactRankNonRed
}

// impactCardItemText 一条卡片的**渲染形态**（四字段 · 一行 · 判据按它量 token）。
func impactCardItemText(it map[string]string) string {
	return fmt.Sprintf("what=%s · why=%s · how=%s · red=%s", it["what"], it["why"], it["how"], it["red"])
}

func impactCardItemTokens(it map[string]string) int {
	c, _ := impactTokenEstimate(impactCardItemText(it))
	return c
}

func impactCardItemLines(it map[string]string) int {
	n := 1
	for _, v := range it {
		n += strings.Count(v, "\n")
	}
	return n
}

func impactCardBodyTokens(kept []impactCardEntry) int {
	n := 0
	for _, e := range kept {
		n += e.tokens()
	}
	return n
}

func (e impactCardEntry) tokens() int { return e.Tok }

// ---- 判据的判定口（测试与自检共用 · 纯函数 ⇒ 负控能直接喂坏输入）-------------------------------

// impactJudgeCardWhy `why` 闭集（六选一）· 每一条都必须有 —— §3.1 硬门槛。
func impactJudgeCardWhy(items []map[string]string) error {
	closed := map[string]bool{}
	for _, w := range impactWhySix {
		closed[w] = true
	}
	for i, it := range items {
		w := strings.TrimSpace(it["why"])
		if w == "" {
			return fmt.Errorf("第 %d 条没有 why（§3.1：**没有 why 的条目一律不进卡片**）", i+1)
		}
		if !closed[w] {
			return fmt.Errorf("第 %d 条的 why = %q 不在闭集六选一里（%s）—— 不许有第七个 why 键",
				i+1, w, strings.Join(impactWhySix, "/"))
		}
	}
	return nil
}

// impactJudgeCardRedLine 「会红」那一行的取值**是闭集**且**语义级永不进来**（§3.5 铁律）。
// `red` 只许取契约 id（`S-a` 一族）或门步名；`why` 是「义近」的条目 `red` 必须为空。
func impactJudgeCardRedLine(items []map[string]string) error {
	for i, it := range items {
		if it["why"] == "义近" && strings.TrimSpace(it["red"]) != "" {
			return fmt.Errorf("第 %d 条是语义级（why=义近）却带 red=%q —— 语义级只进「建议」行、永不进「会红」行（§3.5）",
				i+1, it["red"])
		}
	}
	return nil
}

// impactTruncatedCutFrom —— `meta.truncated_detail.cut_from` 的取值：本命令的裁法只有一种 ——
// `impactCardCutOne` 每轮拿掉的是**名次最低**的那一条（档 ① → ④ 依序、同档里最后一名）
// ⇒ 「从**尾部**砍」。三值闭集（`head` / `middle` / `tail`）的真源是 `main.go` 的
// `truncatedDetailCutFromSet`（判定口读它 ⇒ 取值与判据不两份）。
const impactTruncatedCutFrom = "tail"

// impactCardBudgetFacts 超预算三件（§3.3）—— 判定口与渲染口共用同一份取值（不许两处各拼一套）。
//
// ★ 本批（块D `K-1` · `O-10` · 缺口 `G-81`）**加三格**：`truncated_detail` 三数（单位一律
// 「条目」）与 `cut_from` 的取值**同一处**出 ⇒ 包封那一格与「已裁 N 条」那条**同源同值**，
// 两处不会各算一遍。`Dropped` 不另立字段：它就是 `Cut`（按档裁掉的 + 单条超限整条不进的）。
type impactCardBudgetFacts struct {
	Truncated bool
	Cut       int
	Warning   string
	Restore   string

	Kept    int    // 留下来的**条目**数（= `len(c.Items)` · 与包封 `items[]` 同一条数）
	Total   int    // 预算口径的**裁前**条目数（= `Kept + Cut` ⇒ 三数自校 `kept + dropped == total`）
	CutFrom string // 「从哪砍」的三值枚举之一（本命令恒 `tail`）
}

// impactCardBudget 从卡片取出三件（逐字：`truncated=true` · `warnings[]` 一条「已裁 N 条」·
// `meta.how_to_restore`）＋ 块D `K-1` 的三数与 `cut_from`（同一处取值 ⇒ 不给第二份）。
func impactCardBudget(c impactCard) impactCardBudgetFacts {
	f := impactCardBudgetFacts{Truncated: c.Truncated, Restore: c.HowRestore, CutFrom: impactTruncatedCutFrom}
	f.Kept = len(c.Items)
	f.Cut = c.CutTier[1] + c.CutTier[2] + c.CutTier[3] + c.CutTier[4] + c.Oversize
	f.Total = f.Kept + f.Cut
	for _, w := range c.Warnings {
		if strings.HasPrefix(w, "已裁 ") {
			f.Warning = w
		}
	}
	return f
}

// TruncatedDetailJSON —— `meta.truncated_detail` 的取值（块D `K-1` 形状 · **单位一律「条目」** ·
// 不报 token 估算 ✗ · **不给下标 / 偏移** ✗ · **不含续读入口** ✗ ——「怎么取回」是
// `meta.how_to_restore` 那一格的事，两格分工写死）：
//
//	{"cut_from":"head|middle|tail","kept_items":<int>,"dropped_items":<int>,"total_items":<int>}
//
// 三数自校（`kept + dropped == total`）；**只在真裁时**给（没裁 ⇒ `ok=false` ⇒ 缺席 ——
// 缺席 ≠ 空值/假值，`truncated=false` 时本格**不许**出现）。
func (f impactCardBudgetFacts) TruncatedDetailJSON() (string, bool) {
	if !f.Truncated {
		return "", false
	}
	return fmt.Sprintf(`{"cut_from":%s,"kept_items":%d,"dropped_items":%d,"total_items":%d}`,
		jstr(f.CutFrom), f.Kept, f.Cut, f.Total), true
}

// impactJudgeCard 判据①（尺寸三条）+ 判据②（硬门槛）+ 判据③（超预算三件）的**唯一判定口**。
// 抽出来是为了让负控直接喂坏卡片（条数 13 · 单条 61 token · 无 why · 裁了却三件不齐）。
func impactJudgeCard(c impactCard) error {
	if len(c.Items) > impactItemMax {
		return fmt.Errorf("条数 %d > 上限 %d（§3.1）", len(c.Items), impactItemMax)
	}
	if c.TotalTokens > impactTokenMax {
		return fmt.Errorf("总量 %d token > 上限 %d（保守口径；设计现读口径 %d）—— 上限是死的，不许抬到 1.3k",
			c.TotalTokens, impactTokenMax, c.BytesTokens)
	}
	for i, it := range c.Items {
		if t := impactCardItemTokens(it); t > impactItemTokenMax {
			return fmt.Errorf("第 %d 条 %d token > 单条上限 %d", i+1, t, impactItemTokenMax)
		}
		if l := impactCardItemLines(it); l > impactItemLineMax {
			return fmt.Errorf("第 %d 条 %d 行 > 单条上限 %d 行", i+1, l, impactItemLineMax)
		}
	}
	if err := impactJudgeCardWhy(c.Items); err != nil {
		return err
	}
	if err := impactJudgeCardRedLine(c.Items); err != nil {
		return err
	}
	// §3.2 第一级 + §3.3 第 3 条：**「会红」那一行置顶**（红条目不许排在非红条目之后）。
	seenNoRed := false
	for i, it := range c.Items {
		if it["red"] == "" {
			seenNoRed = true
			continue
		}
		if seenNoRed {
			return fmt.Errorf("第 %d 条是会红条目、却排在非会红条目之后 —— §3.2 第一级「确定性最高的先给」+ §3.3「会红那一条置顶」", i+1)
		}
	}
	// 判据③：**超预算时**三件必须齐（不超 ⇒ 三件都不许出现 —— 余量不是「裁了」）。
	if c.Truncated {
		f := impactCardBudget(c)
		if !f.Truncated {
			return fmt.Errorf("裁了却 `truncated` 不为 true（§3.3 三件之一）")
		}
		if f.Warning == "" {
			return fmt.Errorf("裁了却没有 `warnings[]` 那条「已裁 N 条」（§3.3 三件之二）")
		}
		if strings.TrimSpace(f.Restore) == "" {
			return fmt.Errorf("裁了却没有 `meta.how_to_restore`（人档怎么找回 · §3.3 三件之三）")
		}
		if f.Cut == 0 && c.Oversize == 0 {
			return fmt.Errorf("`truncated=true` 却一条都没裁（裁了就得说裁了几条）")
		}
	} else if len(c.Warnings) != 0 || strings.TrimSpace(c.HowRestore) != "" {
		return fmt.Errorf("没裁却写了三件（余量不是「裁了」：`truncated=false` 时 `warnings[]` 必须空、`how_to_restore` 必须空）")
	}
	return nil
}

// ---- §3.8 可逆性行（三档退法）-----------------------------------------------------------------

// impactReversibility 退法（§3.8 三档）。
type impactReversibility struct {
	Tier    int      // 1 有现成回滚件 · 2 无回滚件但有可找回证据 · 3 无退法
	Cmd     string   // 档 1/2 的那**条命令**（不是「可以回滚」四个字）
	Resolve []string // 判据④要在盘上 `test -e` 解析到的路径
	SHA     string   // 档 2：git 那笔提交 sha（可复算）
	Line    string   // 第③行内联的那一段（同一行 · **不新增行数** §4.1）
}

// impactReversibilityDecide 判档（**纯函数**：成对负控能直接喂「没有回滚件、也没有证据」）。
//
//	档 1：`bin/` 产物（不入库）且盘上有现成回滚件 ⇒ 退法 = 那条 `cp -p <回滚件> <件>`；
//	档 2：无回滚件、但 `git log -1 -- <件>` 有那笔提交 ⇒ 退法 = `git revert <sha> -- <件>`
//	     （**证据路径**逐字给：sha + 件路径 —— 与 §6.1 的两件证据同一套）；
//	档 3：既无回滚件、也无提交（未跟踪 / 无历史）⇒ **必须明写「无退法」** ✗（不许空着）。
func impactReversibilityDecide(isBinProduct bool, rollback, gitSHA, rel string) impactReversibility {
	rb := strings.TrimSpace(rollback)
	sha := strings.TrimSpace(gitSHA)
	rel = strings.TrimSpace(rel)
	switch {
	case isBinProduct && rb != "":
		return impactReversibility{
			Tier: 1, Cmd: fmt.Sprintf("cp -p %s %s", rb, rel), Resolve: []string{rb, rel},
			Line: fmt.Sprintf("这一步能退吗？能（档 1 · 有现成回滚件）：`cp -p %s %s`（**本命令不会自动执行** · §3.8「不许自动回滚」）", rb, rel),
		}
	case sha != "":
		short := sha
		if len(short) > 12 {
			short = short[:12]
		}
		return impactReversibility{
			Tier: 2, Cmd: fmt.Sprintf("git revert %s -- %s", short, rel), Resolve: []string{rel}, SHA: sha,
			Line: fmt.Sprintf("这一步能退吗？能（档 2 · 无回滚件、但有可找回证据）：证据 = git 提交 `%s` + 件路径 `%s` ⇒ `git revert %s -- %s`（**不自动执行**）", short, rel, short, rel),
		}
	default:
		return impactReversibility{
			Tier: 3,
			Line: "这一步能退吗？**不能**：**无退法**（档 3 —— 没有现成回滚件、也没有可找回证据（未跟踪 / 无提交）；**不许空着**：空着会被读成「没提到 = 大概没事」· §3.8）",
		}
	}
}

// impactReversibilityOf 现算退法档（只读：盘上找现成回滚件 + `git log -1 -- <件>`，两者都不写）。
func impactReversibilityOf(root string, tgt *impactTarget) impactReversibility {
	if tgt.Kind == impactKindContract {
		// 契约目标：承诺的真源 = 登记表本身 ⇒ 退法看登记表那件的提交（件路径 = 登记表）。
		sha := impactLastCommit(root, impactRegistryRel)
		return impactReversibilityDecide(false, "", sha, impactRegistryRel)
	}
	if tgt.Rel == "" {
		return impactReversibilityDecide(false, "", "", tgt.Raw)
	}
	rb := impactRollbackArtifact(root, tgt.Rel)
	isBin := strings.HasPrefix(tgt.Rel, "bin/")
	return impactReversibilityDecide(isBin, rb, impactLastCommit(root, tgt.Rel), tgt.Rel)
}

// impactRollbackArtifact 找**现成回滚件**（口径写死：`bin/_history/` 下同族历史副本 ——
// `bin/` 产物不入库（`.gitignore` 第 25 行）⇒ 它们只能靠回滚件退）。找不到 ⇒ 空串。
func impactRollbackArtifact(root, rel string) string {
	base := filepath.Base(filepath.FromSlash(rel))
	dir := filepath.Join(root, "bin", "_history")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	for _, e := range ents {
		if e.IsDir() || !strings.HasPrefix(e.Name(), base+".") {
			continue
		}
		if e.Name() > best { // 取字典序最大 = 最近一枚 `…prev-<时刻>`（口径写明）
			best = e.Name()
		}
	}
	if best == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("bin", "_history", best))
}

// impactLastCommit 那件最后一次改动它的提交（`git log -1 --format=%H -- <件>`）；无 ⇒ 空串。
func impactLastCommit(root, rel string) string {
	out, _, code, err := impactRunIn(root, "git", "log", "-1", "--format=%H", "--", rel)
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// impactJudgeReversibility 判据④（§九 判据⑨ 退法可执行性）的**唯一判定口**：
//
//	· 档 1：那条命令指的回滚件与目标**都必须在盘上解析到**（`test -e`）；
//	· 档 2：证据的 sha 非空、件路径 `test -e` 在盘上（**顺手把「可找回」也判掉** —— 比判据多一格，
//	  只加不减）；
//	· 档 3：行内**必须出现「无退法」三字**（**空着 = 红** ✗）；
//	· 行本身为空 ⇒ 一律红（负控：把该行清空 ⇒ 本判据必须红）。
func impactJudgeReversibility(root string, rev impactReversibility) error {
	if strings.TrimSpace(rev.Line) == "" {
		return fmt.Errorf("退法那一行**空着** —— 空着会被读成「没提到 = 大概没事」（§3.8 · 空着 = 红）")
	}
	stat := func(p string) error {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("退法要解析到的路径为空")
		}
		abs := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("退法要解析到 %s —— 盘上取不到（`test -e` 不过）", p)
		}
		return nil
	}
	switch rev.Tier {
	case 1:
		if strings.TrimSpace(rev.Cmd) == "" {
			return fmt.Errorf("档 1 必须写**那条命令**（不是「可以回滚」四个字）")
		}
		if len(rev.Resolve) == 0 {
			return fmt.Errorf("档 1 的退法没有给出要解析的路径（现成回滚件 + 目标）")
		}
		for _, p := range rev.Resolve {
			if err := stat(p); err != nil {
				return err
			}
		}
		if !strings.Contains(rev.Line, rev.Cmd) {
			return fmt.Errorf("档 1 的退法行里没有那条命令（%q）", rev.Cmd)
		}
	case 2:
		if len(strings.TrimSpace(rev.SHA)) < 8 {
			return fmt.Errorf("档 2 的可找回证据缺 git 提交 sha（要能复算）")
		}
		if len(rev.Resolve) == 0 {
			return fmt.Errorf("档 2 的退法没有给出件路径")
		}
		for _, p := range rev.Resolve {
			if err := stat(p); err != nil {
				return err
			}
		}
	case 3:
		if !strings.Contains(rev.Line, "无退法") {
			return fmt.Errorf("档 3 行内**必须出现「无退法」三字**（不许空着、不许写「建议谨慎」· §3.8）")
		}
	default:
		return fmt.Errorf("退法档 = %d 不在三档里（§3.8：1 回滚件 / 2 可找回证据 / 3 无退法）", rev.Tier)
	}
	return nil
}

// ---- 卡片块的渲染（stderr · 模型档与人面档共用一个口）-----------------------------------------

// emitImpactCardBlock 打卡片块。`human`（`--for-human`）= 人档全文（逐条依据 + 建议命令 +
// 找回路径 · 不限长）；否则 = 模型档（≤ 12 条 + 账 + 裁明细 + 三件 + 铁律）。
func emitImpactCardBlock(stderr io.Writer, tgt *impactTarget, card impactCard,
	rev impactReversibility, layers []impactLayer, human bool) {
	w := stderr
	f := impactCardBudget(card)
	fmt.Fprintf(w, "%s: 卡片（%s · §3.1：三行骨架 + 条目 ≤ %d 条 · 四字段 what/why/how/red · why 闭集**六选一**：%s）\n",
		progName, impactTierName(human), impactItemMax, strings.Join(impactWhySix, "/"))
	for i, it := range card.Items {
		fmt.Fprintf(w, "%s: 卡片条 %d：%s\n", progName, i+1, impactCardItemText(it))
	}
	if len(card.Items) == 0 {
		fmt.Fprintf(w, "%s: 卡片：**受影响项 0 条** ⇒ L1 不打（§3.4：`items[]` 在受影响项 ≥1 时才出；三行骨架恒在）；六层全量 %d 条 —— 被丢的原因逐条见下\n",
			progName, card.Raws)
	}
	fmt.Fprintf(w, "%s: 卡片账（§3.1/§3.4）：条目 %d 条（上限 %d）· token 估算 %d（**保守口径**：表意字 1/字 + 其余 4 B/token）· 设计现读口径（§4.4 逐字 4 B/token）⇒ %d · 上限 %d（**上限是死的**：不许抬到 1.3k，真超了就按 §3.3 裁）\n",
		progName, len(card.Items), impactItemMax, card.TotalTokens, card.BytesTokens, impactTokenMax)
	fmt.Fprintf(w, "%s: 卡片排序（§3.2 四级整数比较 · 不引库）：① 会红 %d 条 → ② 图距离 1 跳（直接调用者）%d 条（取不到图距离的记哨兵 99 ⇒ 排在 1 跳之后）→ ③ 词法命中数 max %d → ④ 义近分（**今天恒 0**：⑤ 未建索引 ⇒ 无义近条目 ⇒ 只作同分 tie-break · 现跑观察不到）\n",
		progName, card.RedN, card.Dist1N, card.LexMax)
	fmt.Fprintf(w, "%s: 卡片裁（§3.3 **按档裁 + 绝不截字符串**）：档① red 空且 why=义近 裁 %d · 档② red 空且 why=词法(含形近) 裁 %d · 档③ red 空且 why∈{包反向,调用边,契约,公开面} 裁 %d · 档④ red 非空（**置顶且最后才裁**）裁 %d · 单条超 %d token 未进卡 %d 条 · 无 why 未进卡 %d 条 · why 不在闭集未进卡 %d 条 · 义近条目带 red 已清空 %d 条\n",
		progName, card.CutTier[1], card.CutTier[2], card.CutTier[3], card.CutTier[4],
		impactItemTokenMax, card.Oversize, card.NoWhy, card.UnknownWhy, card.SemRed)
	fmt.Fprintf(w, "%s: 卡片预算三件（§3.3 · 超预算时**同批**写）：truncated=%t · warnings[]=%s · meta.how_to_restore=%s\n",
		progName, f.Truncated, dashIfEmpty(f.Warning), dashIfEmpty(f.Restore))
	if !card.Truncated {
		fmt.Fprintf(w, "%s: 卡片预算三件今天**不写**：没裁（余量不是「裁了」—— `truncated=false` + `warnings[]` 空 + `how_to_restore` 空）\n", progName)
	}
	fmt.Fprintf(w, "%s: 卡片铁律（§3.5）：**只做参考 · 永不构成批准**（提 ≠ 批：批准者 kind 必须 `human` · `ai-boundary.json`）· 语义级（义近）**永不进「会红」行** · 不自动执行退法命令 · 本卡片只读零副作用（不改件 / 不落审计 / 不写缓存）\n", progName)
	fmt.Fprintf(w, "%s: 卡片退法（§3.8 · 第③行内联同一行 · 不新增行数）：%s\n", progName, rev.Line)
	// §3.7 公开面行（`C5`）：只在命中第 ⑥ 层生效面时才打 —— 与「会先被哪道门拦」分开写；
	// 渲染口与人面 stdout 的**同一份**（`impactPublicLineText`）⇒ 两处不会漂。
	if l, ok := impactLayerBySeq(layers, "⑥"); ok {
		if line := impactPublicLineText(impactPublicLineArgsFromLayer(l)); line != "" {
			fmt.Fprintf(w, "%s: %s\n", progName, line)
		}
	}
	if human {
		// 人档（L2）：全文 = 六层全量逐条 + 被裁条目名次 + 找回路径。
		fmt.Fprintf(w, "%s: 人档全文（L2 · 六层全量 %d 条 · **不限长**）：\n", progName, card.Raws)
		n := 0
		for _, l := range layers {
			for _, r := range l.Rows {
				n++
				fmt.Fprintf(w, "%s: 全文 %d：%s\n", progName, n, impactCardItemText(map[string]string{
					"what": r["what"], "why": r["why"], "how": r["how"], "red": r["red"]}))
			}
		}
		fmt.Fprintf(w, "%s: 人档找回路径（§3.3）：%s · 建议命令：`zerg gate run --fast`（门面 · 真跑属 `B1`）· `zerg code find <词> --path <子目录>`（词法面复算）\n",
			progName, strings.Replace(card.HowRestore, "<目标>", tgt.Raw, 1))
	} else {
		fmt.Fprintf(w, "%s: 模型档只给**一条**最短退法（§3.8 落地形态）；要看全文（逐条依据 + 建议命令 + 找回路径）⇒ `zerg impact %s --for-human`\n",
			progName, tgt.Raw)
	}
}

// impactTierName 档名（§4.1 两档）。
func impactTierName(human bool) string {
	if human {
		return "人面档 --for-human · L2 全文 · **不限长**"
	}
	return "模型档（默认 · ≤ 1.2k token · 条目 ≤ 12）"
}

// impactPublicLineArgs —— §3.7 那一行的**全部输入**（`C5` 起唯一的渲染口吃这一份）。
type impactPublicLineArgs struct {
	Hit       bool   // 命中生效面？（负控面：不在生效面上 ⇒ 一行都不许打）
	Delta     string // `+N / −M`
	Tree      string // 描述串（含现读件数 · `-type f` · 排 .git/vendor）
	TreePath  string // 产出树原始路径（复算命令用）
	Count     int    // 现读件数（同一口径）
	At        string // 扫的时刻
	Head      string // `head_sha`
	NoDataWhy string // 缺的是哪一件（逐条点名）
}

// impactPublicLineText §3.7 那一行（人面 stdout 的条件行 + 卡片块里的同一行 · **同一份取值**）。
//
// 三条判据落在这里（`C5` · 任务单 §四 `C5` 判据①②③）：
//
//	① **负控**：不在生效面上 ⇒ 返回空串（**必须不出**，这一行不是恒返回一行）；
//	② **口径三件齐**：件不是行 ⇒ `-type f` 计数 · 排 `.git`/`vendor` · 产出树路径 + 扫的时刻 +
//	   `head_sha` —— **缺任一 ⇒ 只许写「公开面：未取数」** ✗（不许拿旧数或 0 顶上）；
//	③ **第三方可复算**：行里给同一条 `find` 命令与同相对路径的 `test -e`（同一口径）。
//
// **纯函数** ⇒ 成对负控能直接喂坏输入（不在生效面 / 缺 `head_sha` / 缺产出树）。
func impactPublicLineText(a impactPublicLineArgs) string {
	if !a.Hit {
		return ""
	}
	why := strings.TrimSpace(a.NoDataWhy)
	if why == "" && (strings.TrimSpace(a.Delta) == "" || strings.TrimSpace(a.Tree) == "" ||
		strings.TrimSpace(a.TreePath) == "" || strings.TrimSpace(a.At) == "" || strings.TrimSpace(a.Head) == "") {
		why = "口径三件不齐（产出树路径 / 扫的时刻 / `head_sha` 有一格是空的）"
	}
	if why != "" {
		return "公开面：未取数（缺 " + why + " —— 口径三件 = 产出树路径 + 扫的时刻 + `head_sha`；" +
			"缺任一 ⇒ 只许写「未取数」，不许编数字）"
	}
	recompute := fmt.Sprintf("复算（第三方 · 同一口径）：`find %s -type f -not -path '*/.git/*' -not -path '*/vendor/*' | wc -l`（现读 %d 件）"+
		" · 这一件在不在树里 = 同相对路径 `test -e %s/<件相对路径>`", a.TreePath, a.Count, a.TreePath)
	return fmt.Sprintf("公开面：此改动会改变公开产出树 %s 件（口径：件不是行 · `-type f` · 排 .git/vendor · 产出树 %s · 扫的时刻 %s · head_sha %s · %s）"+
		" —— 这一行说**产出**（变几件），「会红」那一行说**门**（哪一步会红），两行**不合并**",
		a.Delta, a.Tree, a.At, a.Head, recompute)
}

// impactPublicLineArgsFromLayer 从第 ⑥ 层取这一行的输入（层表那一份取值 ⇒ 两处不会漂）。
func impactPublicLineArgsFromLayer(l impactLayer) impactPublicLineArgs {
	return impactPublicLineArgs{
		Hit: l.PublicHit, Delta: l.PublicDelta, Tree: l.PublicTree, TreePath: l.PublicTreePath,
		Count: l.PublicCount, At: l.PublicAt, Head: l.HeadSHA, NoDataWhy: l.PublicNoWhy,
	}
}

// impactPublicLine §3.7 那一行（人面 stdout 的条件行；没命中 或 不在生效面 ⇒ 空串 ⇒ **不打**）。
func impactPublicLine(layers []impactLayer) string {
	l, ok := impactLayerBySeq(layers, "⑥")
	if !ok {
		return ""
	}
	return impactPublicLineText(impactPublicLineArgsFromLayer(l))
}

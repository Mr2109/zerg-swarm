// check_gap_bucket_zero.go —— 门133「缺口账桶内未闭归零」只读常跑判据件（Go 原生 · 无 py 前身）。
//
// 为什么有这件（不是「门75 顺手加一条」）：
//
//	门75（`check-gap-axis-reconcile.go`）判的是**全局守恒式** —— Σ(件面各桶) == Σ(模块面各桶) == 账内
//	`仍缺` 总数。等式成立只说明「桶都数着了」，**一句话也不说哪个桶是否已经清干净**：账里还剩 200 条
//	仍缺、桶和仍是 200，等式照样成立 ⇒ 门75 全绿。这就是本单要补的洞（本仓全仓 grep「桶内未闭归零」
//	0 命中 —— 这条判据此前根本不存在）。
//
//	两条判据的**对象不同**，所以不许挤进一只件的退码里：
//	  · 门75 = 守恒（桶和 ⟷ 总数）。变红 = 计数口径坏了 / 有行解不出键。
//	  · 门133 = 归零（指定模块/件 的桶内 `仍缺` 必须 0）。变红 = 那个模块还欠着账，**当版不许销**。
//	守恒坏了要立刻拦（谁改账都挡）；归零没到是**长期存量**，天天拦等于把闸废掉（本仓「假绿」史的反面）。
//	⇒ 本件单独挂一步、且落在 `tri-report` 只报告档（见 `scripts/gates/precommit-gates.sh` 门133 那段注释
//	的「升档单」）；销版那一刻才要求本件 rc=0。
//
// 判据（庚 · 三段）：
//
//	甲 目标桶归零 —— 清单件 `scripts/gates/bucket-zero-close.json` 的 `targets[]` 逐条 (axis, bucket)：
//	   该桶内 `仍缺` 条数 − 白名单放行条数 **必须为 0**；不为 0 ⇒ rc=1 并**逐条点名 GAP-id**（不止「有 1 条」）。
//	乙 白名单（"永远做不到 0" 的唯一出口）—— 每条必须同时给 (axis, bucket, id 或 `*`) + `reason` + `until`：
//	   · `until` 是日期（`YYYY-MM-DD`），**那天过完才算超**（与 family_gap.go 的时限口径同规）；
//	   · 过期 ⇒ rc=1（点名到期日），✗ 不许静默续命；
//	   · **空转** ⇒ rc=1：条目指的不是目标桶 / 指的是已闭（或不存在）的 id ⇒ 该条豁免在放行空气
//	     ⇒ 一张只增不减的白名单就是第二只假绿的口子，故判它红、逼人删条目；
//	   · `id == "*"` = 整桶豁免（允许，但输出单列「桶级豁免」并在人面点名理由与到期日）。
//	丙 只报告面 —— 清单**外**的桶照打读数（未闭条数 + id 前几条），**不入退码**：本门的面是「指定的
//	   模块/件」，不是「全账必须干净」（那是门75 的守恒面 + 门88 的态证物面）。
//
// 退码（三值 · 与 `tri` 三档逐格同义）：
//
//	0 目标桶全归零（含白名单放行）且白名单无过期/无空转
//	1 有目标桶未归零 / 有白名单过期 / 有白名单空转
//	2 取数面拿不到（账本不在盘 · 账本空转 · 清单件不在盘或 schema 不符 · 清单 targets 空）—— BLOCKED 不绿
//
// 与门75 的关系（口径复用，✗ 不另立第二把尺）：
//
//	桶怎么分 —— 直接调门75 的 `gapAxisRecClassify`（有件 = 该键值本身；无件 = 以 `无件` 开头或空串 ⇒ 裸 `无件`）；
//	账怎么读 —— 直接调门75 的 `gapAxisRecReadLedger`（同一行容错、同一 `仍缺` 态名、同一 JSON 解析面）；
//	账定位   —— 直接调门75 的 `gapAxisRecResolveLedger`（`--ledger` > `--state-dir` > `ZERG_STATE_DIR` > `~/.zerg/state`）。
//	⇒ 两门对同一账、同一桶、同一态名同读同判；门75 改口径，本门跟着变（唯一的桶尺真源）。
//
// 自指面（逐条照实）：
//
//	① 本件扫的是**账本**与**清单件**，✗ 不扫 `core/cmd/zerg/*.go` ⇒ 不在自己的扫描面内。
//	② 本件落在 `check-hardcoded-private-paths` 的 `core/` 面内 ⇒ 正文**零私有卷/家目录绝对路径字面**
//	   （家目录只经 `os.UserHomeDir()` 现取，且那是门75 的 `gapAxisRecDefaultStateDir` 干的）。
//	③ 默认档**零写盘**（只打 stdout）；`--self-test` 只写 `os.MkdirTemp` 造的仓外临时夹具，`defer` 回收。
//	④ 本件产出不喂自身输入面（✗ 不 append 账本 · ✗ 不写清单件 · ✗ 不写索引）⇒ 无回环。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	gapBzTool     = "check-gap-bucket-zero"
	gapBzVersion  = "v1.0.0"
	gapBzGateNum  = "门133"
	gapBzGateName = "门133 缺口账桶内未闭归零：销版清单逐桶 0 未闭（点名到 id · 白名单带理由+到期日）"
	gapBzSchema   = "gap-bucket-zero/1"
	// gapBzRegistryRel —— 清单件（销版必清清单 + 白名单）在仓内的**唯一**落点。
	gapBzRegistryRel = "scripts/gates/bucket-zero-close.json"
	// gapBzNameLimit —— 人面「非目标桶」一行最多列几个桶名（读数不被刷屏淹没）。
	gapBzNameLimit = 12
	// gapBzIDLimit —— 单个桶最多点名几个 GAP-id（其余折成「(+k)」，条数仍照实打）。
	gapBzIDLimit = 20
)

// 退码（三值 · 具名常量，不用裸数字）
const (
	gapBzRCOK    = 0
	gapBzRCFail  = 1
	gapBzRCBlock = 2
)

// 轴闭集 —— 与门75 的两轴同名（件面 `unit` / 模块面 `module`）。
var gapBzAxes = []string{"unit", "module"}

var gapBzAxisLabel = map[string]string{"unit": "件面", "module": "模块面"}

// ── 清单件（schema）──────────────────────────────────────────────────────────

// gapBzTarget —— 销版必清清单的一条：哪个轴的哪个桶，为什么必须清干净。
type gapBzTarget struct {
	Axis   string `json:"axis"`
	Bucket string `json:"bucket"`
	Why    string `json:"why"`
}

// gapBzAllow —— 白名单一条：指定桶里的**哪一条**可以晚闭、理由、到期日（过期即红）。
type gapBzAllow struct {
	Axis   string `json:"axis"`
	Bucket string `json:"bucket"`
	ID     string `json:"id"` // GAP-id 或 `*`（整桶）
	Reason string `json:"reason"`
	Until  string `json:"until"` // YYYY-MM-DD · 那天过完才算超
}

type gapBzRegistry struct {
	Schema  string        `json:"schema"`
	Targets []gapBzTarget `json:"targets"`
	Allow   []gapBzAllow  `json:"whitelist"`
	path    string
	sha16   string
}

// gapBzLoadRegistry —— 读清单件并**逐格验形制**。认不出 ⇒ (nil, 原因)，调用方 rc=2 不给结论。
// 顶层 `_` 开头的键（`_comment` / `_whitelist_note` …）一律忽略 —— 本仓配置件的既成写法。
func gapBzLoadRegistry(path string) (*gapBzRegistry, string) {
	if path == "" {
		return nil, "清单件路径没解析出来"
	}
	if !gapAxisRecIsFile(path) {
		return nil, "清单件不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "清单件读不了：" + err.Error()
	}
	var reg gapBzRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, "清单件 JSON 解不开：" + err.Error()
	}
	reg.path = path
	reg.sha16 = gapAxisRecSHA16(path)
	if reg.Schema != gapBzSchema {
		return nil, fmt.Sprintf("清单件 schema 不符：%q ≠ %q", reg.Schema, gapBzSchema)
	}
	if len(reg.Targets) == 0 {
		return nil, "清单件 targets 空（销版清单空转 = 本门没有判据面 ⇒ 不给结论）"
	}
	seen := map[string]bool{}
	for i, t := range reg.Targets {
		if !gapAxisRecIn(gapBzAxes, t.Axis) {
			return nil, fmt.Sprintf("targets[%d] axis=%q 不在闭集 %v", i, t.Axis, gapBzAxes)
		}
		if strings.TrimSpace(t.Bucket) == "" {
			return nil, fmt.Sprintf("targets[%d] bucket 空", i)
		}
		k := t.Axis + "\x00" + t.Bucket
		if seen[k] {
			return nil, fmt.Sprintf("targets[%d] 重复：%s %s", i, t.Axis, t.Bucket)
		}
		seen[k] = true
	}
	for i, a := range reg.Allow {
		if !gapAxisRecIn(gapBzAxes, a.Axis) {
			return nil, fmt.Sprintf("whitelist[%d] axis=%q 不在闭集 %v", i, a.Axis, gapBzAxes)
		}
		if strings.TrimSpace(a.Bucket) == "" {
			return nil, fmt.Sprintf("whitelist[%d] bucket 空", i)
		}
		if strings.TrimSpace(a.ID) == "" {
			return nil, fmt.Sprintf("whitelist[%d] id 空（整桶豁免写 `*`，✗ 不写空）", i)
		}
		if strings.TrimSpace(a.Reason) == "" {
			return nil, fmt.Sprintf("whitelist[%d] 缺 reason（白名单必须写清「为什么这个桶永远到不了 0」）", i)
		}
		if _, err := gapBzDate(a.Until); err != nil {
			return nil, fmt.Sprintf("whitelist[%d] until=%q 不是 YYYY-MM-DD：%v", i, a.Until, err)
		}
	}
	return &reg, ""
}

// gapBzDate —— 严格 `YYYY-MM-DD`（本地日期）。
func gapBzDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
}

// gapBzToday —— 现读本地日期（**不带时分秒**：到期判据只认日界）。
func gapBzToday() time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local)
}

// gapBzExpired —— 「某日到期」= **那天过完才算超** ⇒ 今天 <= until 即仍有效。
func gapBzExpired(until string, today time.Time) bool {
	d, err := gapBzDate(until)
	if err != nil {
		return true // 形制坏 = 不认这条豁免（fail-closed）
	}
	return today.After(d)
}

// ── 判据 ─────────────────────────────────────────────────────────────────────

// gapBzOpen — 桶内一条未闭。
type gapBzOpen struct {
	id     string
	unit   string
	module string
	impact string
	prio   string
}

// gapBzBucket — 一个桶的读数。
type gapBzBucket struct {
	axis   string
	bucket string
	open   []gapBzOpen
	allowN int      // 白名单放行条数（有效的）
	allowI []string // 放行的 id（`*` 记为 `*`）
	why    string   // targets 的理由（非目标桶空）
}

type gapBzBad struct { // 白名单病灶（过期 / 空转）
	kind string
	line string
}

type gapBzReport struct {
	ledger   string
	sha16    string
	lines    int
	rowsN    int
	remN     int
	regPath  string
	regSHA   string
	readAt   string
	targets  []gapBzBucket
	others   []gapBzBucket
	bad      []gapBzBad
	allowTot int
	rc       int
}

// gapBzJudge ⇒ (rc, rep)。rows == nil ⇒ 取数面拿不到（rc=2）。
func gapBzJudge(rows []gapAxisRecRow, lines int, unparsed []gapAxisRecBadLine, ledger string, reg *gapBzRegistry, today time.Time) (int, *gapBzReport) {
	rep := &gapBzReport{
		ledger: ledger, sha16: gapAxisRecSHA16(ledger), lines: lines,
		readAt: gapAxisRecNowISO(), rc: gapBzRCBlock,
	}
	if reg == nil { // 清单件没读出来 ⇒ 不给结论（调用方已单独报因）
		return gapBzRCBlock, rep
	}
	rep.regPath, rep.regSHA = reg.path, reg.sha16
	if rows == nil {
		rep.rc = gapBzRCBlock
		return gapBzRCBlock, rep
	}
	if len(unparsed) > 0 || lines == 0 || len(rows) == 0 {
		rep.rc = gapBzRCBlock
		return gapBzRCBlock, rep
	}
	rep.rowsN = len(rows)

	// 只认 `仍缺`（与门75 同一态名真源：gapAxisRecRemain）
	var rem []gapAxisRecRow
	for _, r := range rows {
		if s, ok := r.obj["state"].(string); ok && s == gapAxisRecRemain {
			rem = append(rem, r)
		}
	}
	rep.remN = len(rem)
	// ★ 2026-10-23（Mr2109 拍板 · 与门75 同口径）：`仍缺` 0 条 = 账面全清（目标态）⇒ 继续算。
	//   真·取数面拿不到已在上面两支拦掉（rows == nil / lines == 0 / len(rows) == 0）。

	// 两轴一次分好桶：axis ⇒ bucket ⇒ []open
	byAxis := map[string]map[string][]gapBzOpen{}
	for _, ax := range gapBzAxes {
		byAxis[ax] = map[string][]gapBzOpen{}
	}
	for _, r := range rem {
		for _, ax := range gapBzAxes {
			kind, bucket := gapAxisRecClassify(r.obj, ax)
			if kind == "anom" {
				continue // 键面缺失的行不入桶 —— 那是门75 的守恒面（差 = 这类条数），本门不重复判
			}
			o := gapBzOpen{id: gapAxisRecGID(r.obj)}
			if s, ok := r.obj["unit"].(string); ok {
				o.unit = s
			}
			if s, ok := r.obj["module"].(string); ok {
				o.module = s
			}
			if s, ok := r.obj["impact"].(string); ok {
				o.impact = s
			}
			if s, ok := r.obj["prio"].(string); ok {
				o.prio = s
			}
			byAxis[ax][bucket] = append(byAxis[ax][bucket], o)
		}
	}

	// 白名单：先按 (axis,bucket) 归堆，再逐条查它到底在放行谁
	type allowKey struct{ axis, bucket string }
	allowBy := map[allowKey][]gapBzAllow{}
	for _, a := range reg.Allow {
		allowBy[allowKey{a.Axis, a.Bucket}] = append(allowBy[allowKey{a.Axis, a.Bucket}], a)
	}
	targetSet := map[allowKey]bool{}
	for _, t := range reg.Targets {
		targetSet[allowKey{t.Axis, t.Bucket}] = true
	}

	// 白名单空转 / 过期（**在目标桶面先判**：清单外的白名单条目也是空转）
	for _, a := range reg.Allow {
		k := allowKey{a.Axis, a.Bucket}
		where := fmt.Sprintf("%s %s ⟵ %s", gapBzAxisLabel[a.Axis], a.Bucket, a.ID)
		if !targetSet[k] {
			rep.bad = append(rep.bad, gapBzBad{"空转", where + "（该桶不在 targets 里 ⇒ 这条豁免没在放行任何东西）"})
			continue
		}
		opens := byAxis[a.Axis][a.Bucket]
		if a.ID != "*" {
			hit := false
			for _, o := range opens {
				if o.id == a.ID {
					hit = true
					break
				}
			}
			if !hit {
				rep.bad = append(rep.bad, gapBzBad{"空转", where + "（该 id 不在本桶的 `仍缺` 里 ⇒ 豁免在放行空气）"})
				continue
			}
		} else if len(opens) == 0 {
			rep.bad = append(rep.bad, gapBzBad{"空转", where + "（整桶豁免但该桶已 0 未闭 ⇒ 删条目、收窄面）"})
			continue
		}
		if gapBzExpired(a.Until, today) {
			rep.bad = append(rep.bad, gapBzBad{"过期", where + fmt.Sprintf("（until=%s 已过 ⇒ 续期改日期，或把该条销账）", a.Until)})
			continue
		}
	}

	// 目标桶读数
	for _, t := range reg.Targets {
		b := gapBzBucket{axis: t.Axis, bucket: t.Bucket, open: byAxis[t.Axis][t.Bucket], why: t.Why}
		allowed := map[string]bool{}
		for _, a := range allowBy[allowKey{t.Axis, t.Bucket}] {
			if gapBzExpired(a.Until, today) {
				continue // 过期豁免不放行任何一条（上面已记病灶）
			}
			if a.ID == "*" {
				for _, o := range b.open {
					allowed[o.id] = true
				}
				b.allowI = append(b.allowI, "*")
				continue
			}
			for _, o := range b.open {
				if o.id == a.ID {
					allowed[o.id] = true
					b.allowI = append(b.allowI, a.ID)
				}
			}
		}
		b.allowN = len(allowed)
		rep.allowTot += b.allowN
		rep.targets = append(rep.targets, b)
	}

	// 非目标桶（只报告 · 不入退码）
	for _, ax := range gapBzAxes {
		var names []string
		for k := range byAxis[ax] {
			if !targetSet[allowKey{ax, k}] {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		for _, k := range names {
			rep.others = append(rep.others, gapBzBucket{axis: ax, bucket: k, open: byAxis[ax][k]})
		}
	}

	rc := gapBzRCOK
	if len(rep.bad) > 0 {
		rc = gapBzRCFail
	}
	for _, b := range rep.targets {
		if len(b.open)-b.allowN > 0 {
			rc = gapBzRCFail
		}
	}
	rep.rc = rc
	return rc, rep
}

// gapBzMustZero —— 目标桶里**必须归零**的那几条（= 未闭 ∖ 白名单放行）。
func gapBzMustZero(b gapBzBucket) []gapBzOpen {
	allowed := map[string]bool{}
	for _, id := range b.allowI {
		allowed[id] = true
	}
	var out []gapBzOpen
	for _, o := range b.open {
		if !allowed[o.id] {
			out = append(out, o)
		}
	}
	return out
}

// ── 人面渲染 ─────────────────────────────────────────────────────────────────

func gapBzRender(rep *gapBzReport) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "── %s ──\n", gapBzGateName)
	fmt.Fprintf(&sb, "账本     : %s（sha16 %s · 现读 %d 行 · 其中 `%s` %d 条）\n",
		rep.ledger, rep.sha16, rep.lines, gapAxisRecRemain, rep.remN)
	fmt.Fprintf(&sb, "清单件   : %s（sha16 %s）\n", rep.regPath, rep.regSHA)
	fmt.Fprintf(&sb, "现读时刻 : %s\n", rep.readAt)

	nZero, nBad := 0, 0
	for _, b := range rep.targets {
		if len(gapBzMustZero(b)) == 0 {
			nZero++
		}
	}
	fmt.Fprintf(&sb, "目标桶   : %d 个（已归零 %d · 未归零 %d）\n", len(rep.targets), nZero, len(rep.targets)-nZero)
	sb.WriteString("\n【销版必清清单】\n")
	for _, b := range rep.targets {
		mz := gapBzMustZero(b)
		mark := "✓"
		if len(mz) > 0 {
			mark = "✗"
			nBad++
		}
		fmt.Fprintf(&sb, "  %s %s %-40s 未闭 %d · 白名单放行 %d · 必须归零 %d\n",
			mark, gapBzAxisLabel[b.axis], b.bucket, len(b.open), b.allowN, len(mz))
		if b.why != "" {
			fmt.Fprintf(&sb, "      理由：%s\n", b.why)
		}
		if b.allowN > 0 {
			fmt.Fprintf(&sb, "      放行：%s\n", strings.Join(b.allowI, ", "))
		}
		for i, o := range mz {
			if i >= gapBzIDLimit {
				fmt.Fprintf(&sb, "      · …（还有 %d 条，见 --json）\n", len(mz)-gapBzIDLimit)
				break
			}
			fmt.Fprintf(&sb, "      · %-18s %s %s  %s\n", o.id, o.impact, o.prio, o.unit)
		}
	}

	sb.WriteString("\n【白名单（理由 + 到期日）】\n")
	if len(rep.bad) == 0 && rep.allowTot == 0 {
		sb.WriteString("  （无 · 清单内无「永远做不到 0」的条目）\n")
	} else {
		for _, bd := range rep.bad {
			fmt.Fprintf(&sb, "  ✗ %s：%s\n", bd.kind, bd.line)
		}
		if rep.allowTot > 0 && len(rep.bad) == 0 {
			fmt.Fprintf(&sb, "  本次放行 %d 条（明细见上面的「放行：」行）\n", rep.allowTot)
		}
	}

	if len(rep.others) > 0 {
		fmt.Fprintf(&sb, "\n【清单外桶（只报告 · 不入退码）】%d 个\n", len(rep.others))
		for i, b := range rep.others {
			if i >= gapBzNameLimit {
				fmt.Fprintf(&sb, "  · …（还有 %d 个桶）\n", len(rep.others)-gapBzNameLimit)
				break
			}
			fmt.Fprintf(&sb, "  · %s %-32s %d\n", gapBzAxisLabel[b.axis], b.bucket, len(b.open))
		}
	}

	sb.WriteString("\n")
	switch rep.rc {
	case gapBzRCOK:
		fmt.Fprintf(&sb, "⇒ rc=0 桶内未闭归零：%d 个目标桶全清（白名单无过期/无空转）\n", len(rep.targets))
	case gapBzRCFail:
		var parts []string
		for _, b := range rep.targets {
			mz := gapBzMustZero(b)
			if len(mz) == 0 {
				continue
			}
			ids := make([]string, 0, len(mz))
			for _, o := range mz {
				ids = append(ids, o.id)
			}
			shown := ids
			if len(shown) > gapBzIDLimit {
				shown = shown[:gapBzIDLimit]
			}
			tail := ""
			if len(ids) > len(shown) {
				tail = fmt.Sprintf(" …(+%d)", len(ids)-len(shown))
			}
			parts = append(parts, fmt.Sprintf("%s %s 剩 %d 条（%s%s）",
				gapBzAxisLabel[b.axis], b.bucket, len(mz), strings.Join(shown, ", "), tail))
		}
		for _, bd := range rep.bad {
			parts = append(parts, "白名单"+bd.kind+")")
		}
		fmt.Fprintf(&sb, "⇒ rc=1 有桶未归零：%s\n", strings.Join(parts, "；"))
	}
	return sb.String()
}

func gapBzJSON(rep *gapBzReport) string {
	bucketJSON := func(b gapBzBucket) map[string]interface{} {
		mz := gapBzMustZero(b)
		ids := make([]string, 0, len(mz))
		for _, o := range mz {
			ids = append(ids, o.id)
		}
		return map[string]interface{}{
			"axis": b.axis, "bucket": b.bucket, "open": len(b.open),
			"whitelisted": b.allowN, "must_zero": len(mz), "must_zero_ids": ids,
		}
	}
	targets := make([]map[string]interface{}, 0, len(rep.targets))
	for _, b := range rep.targets {
		targets = append(targets, bucketJSON(b))
	}
	others := make([]map[string]interface{}, 0, len(rep.others))
	for _, b := range rep.others {
		others = append(others, map[string]interface{}{"axis": b.axis, "bucket": b.bucket, "open": len(b.open)})
	}
	bad := make([]map[string]string, 0, len(rep.bad))
	for _, x := range rep.bad {
		bad = append(bad, map[string]string{"kind": x.kind, "detail": x.line})
	}
	out := map[string]interface{}{
		"tool": gapBzTool, "version": gapBzVersion, "rc": rep.rc,
		"ledger": rep.ledger, "ledger_sha16": rep.sha16, "lines": rep.lines, "rem_total": rep.remN,
		"registry": rep.regPath, "registry_sha16": rep.regSHA, "read_at": rep.readAt,
		"targets": targets, "whitelist_hits": bad, "others": others,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"tool":%q,"rc":2,"error":%q}`, gapBzTool, err.Error())
	}
	return string(b)
}

// ── --list-rules ─────────────────────────────────────────────────────────────

func gapBzListRules(w io.Writer) {
	fmt.Fprintf(w, "%s %s（%s）\n", gapBzTool, gapBzVersion, gapBzGateNum)
	fmt.Fprintln(w, "判据甲 目标桶归零：targets[] 每个 (axis,bucket) 的 `仍缺` 条数 − 白名单放行 = 0（否则 rc=1 并逐条点名）")
	fmt.Fprintln(w, "判据乙 白名单：每条 (axis,bucket,id|*) + reason + until(YYYY-MM-DD)；过期 rc=1 · 空转 rc=1")
	fmt.Fprintln(w, "判据丙 清单外桶：只打印读数，不入退码（那不是本门的面）")
	fmt.Fprintf(w, "清单件 %s\n", gapBzRegistryRel)
	fmt.Fprintf(w, "schema %s\n", gapBzSchema)
	fmt.Fprintln(w, "退码 0 全归零 / 1 有桶未归零或有白名单病灶 / 2 取数面拿不到（BLOCKED 不绿）")
	fmt.Fprintln(w, "轴闭集 unit（件面）· module（模块面）—— 与门75 同一把桶尺（gapAxisRecClassify）")
}

// ── --self-test（仓外临时夹具 · defer 回收）──────────────────────────────────

// gapBzFixture —— 造一套 (账本, 清单件) 夹具目录，返回 (账本路径, 清单路径)。
func gapBzFixture(dir string, ledgerRows []string, regJSON string) (string, string) {
	lp := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	_ = os.WriteFile(lp, []byte(strings.Join(ledgerRows, "\n")+"\n"), 0o644)
	rp := filepath.Join(dir, "bucket-zero-close.json")
	_ = os.WriteFile(rp, []byte(regJSON), 0o644)
	return lp, rp
}

func gapBzRow(id, state, unit, module, impact, prio string) string {
	return fmt.Sprintf(`{"id":%q,"state":%q,"unit":%q,"module":%q,"impact":%q,"prio":%q}`,
		id, state, unit, module, impact, prio)
}

func gapBzSelfTest(w io.Writer) int {
	pass, fail := 0, 0
	ok := func(name string, cond bool, detail string) {
		if cond {
			pass++
			fmt.Fprintf(w, "  ✓ %s\n", name)
			return
		}
		fail++
		fmt.Fprintf(w, "  ✗ %s（%s）\n", name, detail)
	}
	run := func(lp, rp string, today time.Time) (int, *gapBzReport) {
		rows, lines, un := gapAxisRecReadLedger(lp)
		reg, why := gapBzLoadRegistry(rp)
		if reg == nil {
			_ = why
			return gapBzRCBlock, nil
		}
		return gapBzJudge(rows, lines, un, lp, reg, today)
	}

	dir, err := os.MkdirTemp("", "gap-bucket-zero-selftest-")
	if err != nil {
		fmt.Fprintf(w, "自检结论：失败（造不出临时夹具：%v）\n", err)
		return gapBzRCBlock
	}
	defer os.RemoveAll(dir)

	day := time.Date(2026, 10, 22, 0, 0, 0, 0, time.Local)
	reg1 := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"命令树落点"}],"whitelist":[]}`
	open := []string{
		gapBzRow("GAP-20261011-03", "仍缺", "core/cmd/zerg/main.go", "core/cmd/zerg", "命令面", "P1"),
		gapBzRow("GAP-20260925-03", "仍缺", "无件(命令面)", "无件", "命令面", "P2"),
		gapBzRow("GAP-1", "已解", "core/cmd/zerg/main.go", "core/cmd/zerg", "命令面", "P2"),
	}
	closed := []string{
		gapBzRow("GAP-20261011-03", "已解", "core/cmd/zerg/main.go", "core/cmd/zerg", "命令面", "P1"),
		gapBzRow("GAP-20260925-03", "仍缺", "无件(命令面)", "无件", "命令面", "P2"),
	}

	// 1 现读态：目标桶 1 条未闭 ⇒ rc=1 且点名 GAP-20261011-03
	lp, rp := gapBzFixture(dir, open, reg1)
	rc, rep := run(lp, rp, day)
	ok("① 目标桶 1 条未闭 ⇒ rc=1", rc == gapBzRCFail, fmt.Sprintf("rc=%d", rc))
	named := rep != nil && strings.Contains(gapBzRender(rep), "GAP-20261011-03") &&
		strings.Contains(gapBzRender(rep), "⇒ rc=1")
	ok("② 红的那一行**点名到 id**（GAP-20261011-03）", named, "输出里没点名")

	// 2 人为销后 ⇒ rc=0
	lp2, rp2 := gapBzFixture(dir, closed, reg1)
	rc2, rep2 := run(lp2, rp2, day)
	ok("③ 该条销后 ⇒ rc=0（同桶同清单，只改一条 state）", rc2 == gapBzRCOK, fmt.Sprintf("rc=%d", rc2))
	ok("④ 绿的那一行写「全清」", rep2 != nil && strings.Contains(gapBzRender(rep2), "全清"), "输出没写全清")

	// 3 白名单在效期内 ⇒ 放行 ⇒ rc=0
	regWL := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"core/cmd/zerg","id":"GAP-20261011-03","reason":"单件拆族需跨版，本条到下一版随拆件一起销","until":"2026-12-31"}]}`
	lp3, rp3 := gapBzFixture(dir, open, regWL)
	rc3, _ := run(lp3, rp3, day)
	ok("⑤ 白名单在效期内 ⇒ 放行 ⇒ rc=0", rc3 == gapBzRCOK, fmt.Sprintf("rc=%d", rc3))

	// 4 白名单过期 ⇒ rc=1（点名到期日）
	rc4, rep4 := run(lp3, rp3, time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local))
	ok("⑥ 白名单过期（until 那天过完）⇒ rc=1",
		rc4 == gapBzRCFail && rep4 != nil && strings.Contains(gapBzRender(rep4), "2026-12-31"),
		fmt.Sprintf("rc=%d", rc4))

	// 5 白名单空转（id 已闭）⇒ rc=1
	lp5, rp5 := gapBzFixture(dir, closed, regWL)
	rc5, rep5 := run(lp5, rp5, day)
	ok("⑦ 白名单空转（该 id 已不在 `仍缺`）⇒ rc=1",
		rc5 == gapBzRCFail && rep5 != nil && strings.Contains(gapBzRender(rep5), "空转"), fmt.Sprintf("rc=%d", rc5))

	// 6 白名单指向非目标桶 ⇒ rc=1（不许拿清单外条目充数）
	regOut := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"无件","id":"*","reason":"无件形态永远有存量，这一桶不许进清单","until":"2026-12-31"}]}`
	lp6, rp6 := gapBzFixture(dir, open, regOut)
	rc6, _ := run(lp6, rp6, day)
	ok("⑧ 白名单指向清单外的桶 ⇒ rc=1（空转）", rc6 == gapBzRCFail, fmt.Sprintf("rc=%d", rc6))

	// 7 整桶豁免 `*` 在效期内 ⇒ rc=0
	regStar := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"core/cmd/zerg","id":"*","reason":"整桶豁免示例：本桶迁移期内不要求归零","until":"2026-12-31"}]}`
	lp7, rp7 := gapBzFixture(dir, open, regStar)
	rc7, _ := run(lp7, rp7, day)
	ok("⑨ 整桶豁免 `*` 在效期内 ⇒ rc=0", rc7 == gapBzRCOK, fmt.Sprintf("rc=%d", rc7))

	// 8 清单外桶不进退码：把唯一目标桶换个空名 ⇒ 无未闭 ⇒ rc=0，无件桶只在报告面
	regEmpty := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"unit","bucket":"core/cmd/zerg/other.go","why":"x"}],"whitelist":[]}`
	lp8, rp8 := gapBzFixture(dir, open, regEmpty)
	rc8, rep8 := run(lp8, rp8, day)
	ok("⑩ 清单外桶（无件）只报告、不入退码 ⇒ rc=0",
		rc8 == gapBzRCOK && rep8 != nil && len(rep8.others) > 0, fmt.Sprintf("rc=%d", rc8))

	// 9 清单件不在盘 / schema 不符 ⇒ rc=2
	rc9, _ := run(lp, filepath.Join(dir, "nope.json"), day)
	ok("⑪ 清单件不在盘 ⇒ rc=2（BLOCKED 不绿）", rc9 == gapBzRCBlock, fmt.Sprintf("rc=%d", rc9))
	badReg := `{"schema":"gap-bucket-zero/999","targets":[{"axis":"module","bucket":"x","why":"y"}],"whitelist":[]}`
	_, rpBad := gapBzFixture(dir, open, badReg)
	rc10, _ := run(lp, rpBad, day)
	ok("⑫ schema 不符 ⇒ rc=2", rc10 == gapBzRCBlock, fmt.Sprintf("rc=%d", rc10))

	// 10 账本不在盘 / 空转 ⇒ rc=2
	_, rpReg1 := gapBzFixture(dir, open, reg1) // 前面几格反复覆写同一路径 ⇒ 这里重建 reg1 后再取
	rows11, lines11, un11 := gapAxisRecReadLedger(filepath.Join(dir, "no-such-ledger.jsonl"))
	regA, _ := gapBzLoadRegistry(rpReg1)
	rc11, _ := gapBzJudge(rows11, lines11, un11, filepath.Join(dir, "no-such-ledger.jsonl"), regA, day)
	ok("⑬ 账本不在盘 ⇒ rc=2", rc11 == gapBzRCBlock, fmt.Sprintf("rc=%d", rc11))
	emptyLedger := filepath.Join(dir, "empty.jsonl")
	_ = os.WriteFile(emptyLedger, []byte("\n\n"), 0o644)
	rows12, lines12, un12 := gapAxisRecReadLedger(emptyLedger)
	rc12, _ := gapBzJudge(rows12, lines12, un12, emptyLedger, regA, day)
	ok("⑭ 账本空转（0 行）⇒ rc=2", rc12 == gapBzRCBlock, fmt.Sprintf("rc=%d", rc12))

	// 11 到期日形制坏 ⇒ 清单件不认（rc=2）
	badUntil := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"core/cmd/zerg","id":"*","reason":"r","until":"2026/12/31"}]}`
	_, rpBU := gapBzFixture(dir, open, badUntil)
	rc13, _ := run(lp, rpBU, day)
	ok("⑮ until 不是 YYYY-MM-DD ⇒ rc=2（fail-closed）", rc13 == gapBzRCBlock, fmt.Sprintf("rc=%d", rc13))

	// 12 判据乙的日界：until 当天仍有效、次日超
	ok("⑯ 到期日界：until=2026-12-31 ⇒ 12-31 有效、2027-01-01 超",
		!gapBzExpired("2026-12-31", time.Date(2026, 12, 31, 0, 0, 0, 0, time.Local)) &&
			gapBzExpired("2026-12-31", time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)),
		"日界口径打坏")

	// 13 必须归零的条数 = 未闭 − 放行（不许把放行算进已闭）
	bt := gapBzBucket{open: []gapBzOpen{{id: "A"}, {id: "B"}}, allowN: 1, allowI: []string{"A"}}
	ok("⑰ must_zero = 未闭 − 白名单放行（放行 ≠ 已闭）", len(gapBzMustZero(bt)) == 1 && gapBzMustZero(bt)[0].id == "B", "算式打坏")

	fmt.Fprintf(w, "自检结论：通过 %d 条 · 失败 %d 条\n", pass, fail)
	if fail > 0 {
		return gapBzRCFail
	}
	return gapBzRCOK
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

const gapBzUsageBlock = `usage: check-gap-bucket-zero [-h] [--ledger LEDGER] [--state-dir STATE_DIR]
                              [--registry REGISTRY] [--repo REPO] [--json]
                              [--list-rules] [--self-test]`

const gapBzHelpText = gapBzUsageBlock + `

门133 缺口账桶内未闭归零（只读判据 · 不写任何件）

判据
  甲 目标桶归零 : 清单件 targets[] 每个 (axis,bucket) 的 ` + "`仍缺`" + ` 条数 − 白名单放行 = 0
                  （否则 rc=1，逐条点名 GAP-id；桶尺与门75 同一份实现）
  乙 白名单     : 每条 (axis,bucket,id 或 *) + reason + until(YYYY-MM-DD)
                  过期 ⇒ rc=1（点名到期日）· 空转 ⇒ rc=1（指向非目标桶 / 已闭 id / 已 0 的整桶）
  丙 清单外桶   : 只打印读数，不入退码

退码
  0 目标桶全归零（含白名单放行）且白名单无过期/无空转
  1 有目标桶未归零 / 有白名单过期 / 有白名单空转
  2 取数面拿不到（账本不在盘或空转 / 清单件不在盘或 schema 不符 / targets 空）—— BLOCKED 不绿
`

func gapBzArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, gapBzUsageBlock)
	fmt.Fprintf(stderr, "check-gap-bucket-zero: error: %s\n", msg)
	return gapBzRCBlock
}

func checkGapBucketZeroGateMain(stdout, stderr io.Writer, argv []string) int {
	ledgerArg, stateDir, regArg, repo := "", "", "", ""
	jsonOut, listRules, selfTest := false, false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, gapBzHelpText)
			return gapBzRCOK
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--ledger":
			i++
			if i >= len(argv) {
				return gapBzArgErr(stderr, "argument --ledger: expected one argument")
			}
			ledgerArg = argv[i]
		case strings.HasPrefix(a, "--ledger="):
			ledgerArg = a[len("--ledger="):]
		case a == "--state-dir":
			i++
			if i >= len(argv) {
				return gapBzArgErr(stderr, "argument --state-dir: expected one argument")
			}
			stateDir = argv[i]
		case strings.HasPrefix(a, "--state-dir="):
			stateDir = a[len("--state-dir="):]
		case a == "--registry":
			i++
			if i >= len(argv) {
				return gapBzArgErr(stderr, "argument --registry: expected one argument")
			}
			regArg = argv[i]
		case strings.HasPrefix(a, "--registry="):
			regArg = a[len("--registry="):]
		case a == "--repo":
			i++
			if i >= len(argv) {
				return gapBzArgErr(stderr, "argument --repo: expected one argument")
			}
			repo = argv[i]
		case strings.HasPrefix(a, "--repo="):
			repo = a[len("--repo="):]
		default:
			return gapBzArgErr(stderr, "unrecognized arguments: "+a)
		}
	}
	if listRules {
		gapBzListRules(stdout)
		return gapBzRCOK
	}
	if selfTest {
		return gapBzSelfTest(stdout)
	}

	ledger := gapAxisRecResolveLedger(ledgerArg, stateDir)
	regPath := regArg
	if regPath == "" {
		root := gapAxisRecResolveRepo(repo)
		if root == "" {
			fmt.Fprintf(stdout, "── %s ──\n", gapBzGateName)
			fmt.Fprintf(stdout, "⇒ rc=2 BLOCKED（取数面拿不到）：仓根没解析出来（--repo / ZERG_REPO / git rev-parse 三面都没给）\n")
			return gapBzRCBlock
		}
		regPath = filepath.Join(root, gapBzRegistryRel)
	} else {
		regPath = gapAxisRecAbsExpand(regPath)
	}

	reg, why := gapBzLoadRegistry(regPath)
	if reg == nil {
		fmt.Fprintf(stdout, "── %s ──\n", gapBzGateName)
		fmt.Fprintf(stdout, "取数面 : %s\n", ledger)
		fmt.Fprintf(stdout, "清单件 : %s\n", regPath)
		fmt.Fprintf(stdout, "现读时刻: %s\n", gapAxisRecNowISO())
		fmt.Fprintf(stdout, "⇒ rc=2 BLOCKED（取数面拿不到）：%s\n", why)
		if jsonOut {
			fmt.Fprintf(stdout, `{"tool":%q,"rc":2,"blocked":%q,"ledger":%q,"registry":%q}`+"\n",
				gapBzTool, why, ledger, regPath)
		}
		return gapBzRCBlock
	}

	rows, lines, unparsed := gapAxisRecReadLedger(ledger)
	rc, rep := gapBzJudge(rows, lines, unparsed, ledger, reg, gapBzToday())
	if rep == nil {
		rep = &gapBzReport{ledger: ledger, regPath: reg.path, rc: gapBzRCBlock}
	}
	if rc == gapBzRCBlock {
		fmt.Fprintf(stdout, "── %s ──\n", gapBzGateName)
		fmt.Fprintf(stdout, "取数面 : %s\n", ledger)
		fmt.Fprintf(stdout, "清单件 : %s\n", reg.path)
		fmt.Fprintf(stdout, "现读时刻: %s\n", gapAxisRecNowISO())
		reason := "账本空转（0 行）"
		if rows == nil {
			reason = "账本不在盘或读不了"
			if len(unparsed) > 0 {
				reason = unparsed[0].why
			}
		} else if len(unparsed) > 0 {
			reason = fmt.Sprintf("有 %d 行解不开", len(unparsed))
		}
		// ★ 2026-10-23（Mr2109 拍板）：`仍缺` 0 条不再算「取数面拿不到」——
		//   账面全清是目标态；只有账本不在盘 / 0 行 / 有行解不开才 BLOCKED（上面三段）。
		fmt.Fprintf(stdout, "⇒ rc=2 BLOCKED（取数面拿不到）：%s\n", reason)
		if jsonOut {
			fmt.Fprintln(stdout, gapBzJSON(rep))
		}
		return gapBzRCBlock
	}

	fmt.Fprint(stdout, gapBzRender(rep))
	if jsonOut {
		fmt.Fprintln(stdout, gapBzJSON(rep))
	}
	return rc
}

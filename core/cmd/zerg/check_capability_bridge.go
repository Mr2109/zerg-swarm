// check_capability_bridge.go —— 判据件（Go inproc · 键 `check-capability-bridge`）：
// **网关侧「路由表 id → 快照 id」必须经桥映射**（缺口 `GAP-20261014-62`）。
//
// 病（缺口逐字）：能力闸判请求必需能力时，网关拿**路由表 id**（`fleet.yaml` 的 `models` 键，
// 形如 `example-35b-v2`）去模型登记库里找能力快照，而登记记录与其快照按**快照 id**命名
// （形如 `example-35b-v2-1-5-35b-q4-k-m`）。只按目标名逐字比 `row.ID` ⇒ 两套命名**恒不等** ⇒
// 命中不到记录 ⇒ 门槛对任何必需能力恒 fail-closed（Reason=`no_snapshot`）⇒ 正门永远拒视觉，
// 只能绕开网关直连引擎端点。桥 = 在册真源 `core/internal/contract/model-id-map.json`
// （`registry_id ↔ fleet_ids`）。
//
// 本件是「看着快照来源面」的判据件（只读 · 静态读源码 + 读桥真源；不起子进程 / 不联网 / 不写盘）：
//
//	面① 网关能力快照来源件在盘 = `core/internal/gateway/capability_gate.go`
//	     （读不到 ⇒ rc=2，**不给结论**，✗ 不当绿）。
//	面② **桥接线**（进退码）= 该件的**快照解析函数**（头含 `CapabilitySnapshot(`）体内**必须同时**
//	     出现桥真源的两枚接线标记：`contract.ModelIDMap()` 与 `RegistryIDForFleet(`。
//	     缺任一枚 ⇒ 网关侧**没接桥** ⇒ 路由表 id 与快照 id 两套命名恒不等 ⇒ rc=1 并**点名缺哪一枚**。
//	面③ **桥真源在盘且非空**（进退码）= `core/internal/contract/model-id-map.json` 存在、可解析、
//	     `entries[]` ≥ 1 条可用映射（每条须 `registry_id` 非空 ∧ `fleet_ids` 非空）——
//	     桥是两套命名之间的**数据**，空表 = 桥在而无映射 ⇒ 同样接不上 ⇒ rc=1。
//	面④ 真源读不动仍 fail-closed（**报告档 · 不进退码**）= 快照解析函数体内有「真源读不动 ⇒
//	     留痕并退回逐字比」的失败分支（`log.Printf` ∧ `fail-closed` 字样）。缺则只列出、**不改退码**。
//
// 退码：`0` = 桥接线齐（快照解析经桥映射）· `1` = 红（未接桥 / 桥真源空或不在盘）·
//
//	`2` = 不给结论（判据件不在盘 / 快照解析函数找不到 / 用法错 / 自检不过）。
//
// ★ 两态口径（本件自证）：**仓外夹具**「网关 id 与快照 id 两套命名且未接桥」⇒ 必 rc=1 并点名；
// **现仓真跑** ⇒ rc=0（本仓 `capability_gate.go` 已接真源 `contract.ModelIDMap()`）。
// ★ 自指：本件不属被扫面（只读 `core/internal/gateway/` 与 `core/internal/contract/`，
// 不扫 `core/cmd/zerg/*.go`）⇒ 无自指污染。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	cbrVersion = "1.0.0"
	cbrGWRel   = "core/internal/gateway/capability_gate.go"
	cbrMapRel  = "core/internal/contract/model-id-map.json"
)

// cbrRCS —— 三档退码（与 `tri` 三值 0 PASS / 1 FAIL / 2 BLOCKED 逐格同义）。
const cbrRCS = "0=桥接线齐 · 1=红（未接桥 / 桥真源空或不在盘）· 2=不给结论（判据件不在盘 / 用法错 / 自检不过）"

// cbrRules —— 判据正文（`--list-rules` 与报告行同源）。
const cbrRules = "面① 网关能力快照来源件在盘（core/internal/gateway/capability_gate.go）；" +
	"面② 桥接线（进退码）= 该件快照解析函数（头含 CapabilitySnapshot(）体内必须同时有 " +
	"contract.ModelIDMap() 与 RegistryIDForFleet(（路由表 id → 快照 id 的桥 · 缺任一枚 ⇒ 两套命名恒不等）；" +
	"面③ 桥真源在盘且非空（进退码）= core/internal/contract/model-id-map.json 可解析且 entries[] ≥ 1 条 " +
	"registry_id 非空 ∧ fleet_ids 非空的映射；" +
	"面④ 真源读不动仍 fail-closed（报告档 · 不进退码）= 解析函数体内有 log.Printf ∧ fail-closed 字样。"

// ── 取数面 ─────────────────────────────────────────────────────────────────────

// cbrFuncBlocks —— 把件正文按**顶层 func 声明**切成块（块头 = 该 func 的首行；块身含到下一个顶层 func 之前）。
func cbrFuncBlocks(src string) [][2]string {
	var idx []int
	if strings.HasPrefix(src, "func ") {
		idx = append(idx, 0)
	}
	const needle = "\nfunc "
	pos := 0
	for {
		k := strings.Index(src[pos:], needle)
		if k < 0 {
			break
		}
		idx = append(idx, pos+k+1)
		pos = pos + k + 1
	}
	out := make([][2]string, 0, len(idx))
	for i, a := range idx {
		b := len(src)
		if i+1 < len(idx) {
			b = idx[i+1]
		}
		blk := src[a:b]
		hdr := blk
		if nl := strings.Index(blk, "\n"); nl >= 0 {
			hdr = blk[:nl]
		}
		out = append(out, [2]string{strings.TrimSpace(hdr), blk})
	}
	return out
}

// cbrFuncNameOf —— 从块头里取函数/方法名（`func (f *T) Name(` ⇒ `Name`）。
func cbrFuncNameOf(hdr string) string {
	s := strings.TrimPrefix(hdr, "func ")
	if i := strings.Index(s, "("); i >= 0 && strings.HasPrefix(s, "(") {
		// 方法：跳过接收者括号
		rest := s[i+1:]
		if j := strings.Index(rest, ")"); j >= 0 {
			s = strings.TrimSpace(rest[j+1:])
		}
	}
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// cbrResolverBlock —— 找**快照解析函数**（块头含 `CapabilitySnapshot(`）。
func cbrResolverBlock(src string) (hdr, body string, ok bool) {
	for _, b := range cbrFuncBlocks(src) {
		if strings.Contains(b[0], "CapabilitySnapshot(") {
			return b[0], b[1], true
		}
	}
	return "", "", false
}

// cbrFinding —— 一条读数（面 + 逐字详情）。
type cbrFinding struct{ Face, Detail string }

// cbrResult —— 一次判读的完整读数。
type cbrResult struct {
	Rc       int
	Findings []cbrFinding // 进退码项（rc=1）
	Notes    []cbrFinding // 报告档项（不进退码）
	Resolver string
	Entries  int
	Bail     string // rc=2 时逐字原因
}

// cbrCompute —— 判读主体（与 `--root` 解出的仓根相对）。
func cbrCompute(root string) cbrResult {
	var res cbrResult
	gwPath := filepath.Join(root, cbrGWRel)
	gw, err := os.ReadFile(gwPath)
	if err != nil {
		res.Rc = 2
		res.Bail = fmt.Sprintf("网关能力快照来源件不在盘（%s）⇒ 取数面拿不到，不给结论", cbrGWRel)
		return res
	}
	src := string(gw)
	hdr, body, ok := cbrResolverBlock(src)
	if !ok {
		res.Rc = 2
		res.Bail = "网关能力快照来源件里找不到快照解析函数（块头含 CapabilitySnapshot(）⇒ 取数面变了，不给结论"
		return res
	}
	res.Resolver = cbrFuncNameOf(hdr)

	// ── 面② 桥接线（进退码）──
	if !strings.Contains(body, "contract.ModelIDMap()") {
		res.Findings = append(res.Findings, cbrFinding{"桥接线",
			fmt.Sprintf("%s 的快照解析函数 %s 体内**没有** contract.ModelIDMap() —— 网关侧没接 id 规范化真源 ⇒ "+
				"路由表 id（如 example-35b-v2）与快照 id（如 example-35b-v2-1-5-35b-q4-k-m）两套命名恒不等 ⇒ 任何必需能力恒 fail-closed（no_snapshot）",
				cbrGWRel, res.Resolver)})
	}
	if !strings.Contains(body, "RegistryIDForFleet(") {
		res.Findings = append(res.Findings, cbrFinding{"桥接线",
			fmt.Sprintf("%s 的快照解析函数 %s 体内**没有** RegistryIDForFleet( —— 「目标名 → 快照 id」那一步没接桥",
				cbrGWRel, res.Resolver)})
	}

	// ── 面④ 真源读不动仍 fail-closed（报告档）──
	if !(strings.Contains(body, "fail-closed") && strings.Contains(body, "log.Printf")) {
		res.Notes = append(res.Notes, cbrFinding{"真源读不动仍 fail-closed",
			fmt.Sprintf("%s 的快照解析函数 %s 体内看不到「真源读不动 ⇒ 留痕并退回逐字比（fail-closed）」的失败分支（缺 log.Printf 或 fail-closed 字样）",
				cbrGWRel, res.Resolver)})
	}

	// ── 面③ 桥真源在盘且非空（进退码）──
	mp := filepath.Join(root, cbrMapRel)
	mb, merr := os.ReadFile(mp)
	if merr != nil {
		res.Findings = append(res.Findings, cbrFinding{"桥真源",
			fmt.Sprintf("桥真源不在盘（%s）—— 两套命名之间没有映射数据", cbrMapRel)})
	} else {
		var doc struct {
			Entries []struct {
				RegistryID string   `json:"registry_id"`
				FleetIDs   []string `json:"fleet_ids"`
			} `json:"entries"`
		}
		if jerr := json.Unmarshal(mb, &doc); jerr != nil {
			res.Findings = append(res.Findings, cbrFinding{"桥真源",
				fmt.Sprintf("桥真源（%s）解析不出（%v）", cbrMapRel, jerr)})
		} else {
			n := 0
			for _, e := range doc.Entries {
				if strings.TrimSpace(e.RegistryID) == "" {
					continue
				}
				okFleet := false
				for _, f := range e.FleetIDs {
					if strings.TrimSpace(f) != "" {
						okFleet = true
						break
					}
				}
				if okFleet {
					n++
				}
			}
			res.Entries = n
			if n == 0 {
				res.Findings = append(res.Findings, cbrFinding{"桥真源",
					fmt.Sprintf("桥真源（%s）entries[] 零条可用映射（每条须 registry_id 非空 ∧ fleet_ids 非空）—— 桥在而无数据 ⇒ 两套命名接不上",
						cbrMapRel)})
			}
		}
	}

	if len(res.Findings) > 0 {
		res.Rc = 1
	}
	return res
}

// ── 报告面 ─────────────────────────────────────────────────────────────────────

func cbrJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cbrReport(res cbrResult, root string, asJSON bool) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("门: 能力闸快照来源面（路由表 id → 快照 id 必经桥映射 · GAP-20261014-62）· v%s", cbrVersion))
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	lines = append(lines, "仓根："+abs)
	lines = append(lines, "判据："+cbrRules)
	lines = append(lines, fmt.Sprintf("取数：网关来源件=%s · 快照解析函数=%s · 桥真源=%s · 桥可用映射条数=%d",
		cbrGWRel, res.Resolver, cbrMapRel, res.Entries))
	if len(res.Findings) > 0 {
		lines = append(lines, fmt.Sprintf("判红 %d 项（逐条点名 · 面/详情）：", len(res.Findings)))
		for _, f := range res.Findings {
			lines = append(lines, fmt.Sprintf("  ✗ [%s] %s", f.Face, f.Detail))
		}
	} else {
		lines = append(lines, "判红 0 项：网关侧快照解析经桥映射（路由表 id 与快照 id 两套命名已接）。")
	}
	lines = append(lines, fmt.Sprintf("报告档（不进退码）：真源读不动仍 fail-closed 面 —— 缺口 %d 项：", len(res.Notes)))
	for _, n := range res.Notes {
		lines = append(lines, fmt.Sprintf("  · [%s] %s", n.Face, n.Detail))
	}
	out := strings.Join(lines, "\n")
	if asJSON {
		out += "\n" + cbrJSONDoc(res)
	}
	return out
}

func cbrJSONDoc(res cbrResult) string {
	var b strings.Builder
	b.WriteString("{\"gate\": \"capability-bridge\", \"version\": " + cbrJSONStr(cbrVersion) +
		fmt.Sprintf(", \"rc\": %d, \"resolver\": %s, \"entries\": %d, \"findings\": [", res.Rc, cbrJSONStr(res.Resolver), res.Entries))
	for i, f := range res.Findings {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + cbrJSONStr(f.Face) + ", \"detail\": " + cbrJSONStr(f.Detail) + "}")
	}
	b.WriteString("], \"notes\": [")
	for i, n := range res.Notes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + cbrJSONStr(n.Face) + ", \"detail\": " + cbrJSONStr(n.Detail) + "}")
	}
	b.WriteString("]}")
	return b.String()
}

// cbrRun —— 默认档（真目标只读判读）。
func cbrRun(stdout io.Writer, root string, asJSON bool) int {
	res := cbrCompute(root)
	if res.Rc == 2 {
		fmt.Fprintf(stdout, "能力闸快照来源面：**判不了**（%s）⇒ rc=2 不给结论\n", res.Bail)
		fmt.Fprintf(stdout, "退码：%s\n", cbrRCS)
		return 2
	}
	fmt.Fprintln(stdout, cbrReport(res, root, asJSON))
	fmt.Fprintf(stdout, "退码：%s\n", cbrRCS)
	return res.Rc
}

// ── 帮助 / 用法错正文 ──────────────────────────────────────────────────────────

const cbrProgName = "check-capability-bridge.py"

const cbrUsageBlock = "usage: " + cbrProgName + " [-h] [--root ROOT] [--json] [--list-rules] [--self-test] [root]"

const cbrHelpText = cbrUsageBlock + "\n\n" +
	"能力闸快照来源面：网关侧路由表 id → 快照 id 必经桥映射（只读 · GAP-20261014-62）\n\n" +
	"positional arguments:\n" +
	"  root             仓根（默认当前目录）\n\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --root ROOT      同位置参数（显式档）\n" +
	"  --json           末尾追加机器面一行\n" +
	"  --list-rules     打判据 / 退码\n" +
	"  --self-test      只跑自检（合成夹具 · 不碰真目标）\n"

func cbrUsageErr(stderr io.Writer, msg string) {
	fmt.Fprintln(stderr, cbrUsageBlock)
	fmt.Fprintf(stderr, "%s: error: %s\n", cbrProgName, msg)
}

// ── 自检夹具（合成 · 仓外 tmpdir）──────────────────────────────────────────────
//
// 主负控 = 任务单要求的那一格：**网关 id 与快照 id 两套命名且未接桥** ⇒ 必报红并点名。

const cbrFixtureBridged = `package gateway

import (
	"fmt"
	"log"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

func (f *fileCapabilitySource) CapabilitySnapshot(model string) (any, error) {
	targets := []string{model}
	if m, merr := contract.ModelIDMap(); merr == nil {
		if rid, ok := m.RegistryIDForFleet(model); ok && rid != model {
			targets = append(targets, rid)
		}
	} else {
		log.Printf("真源读不动 ⇒ 只按目标名 %q 逐字比（fail-closed）", model)
	}
	for _, target := range targets {
		for _, row := range rows {
			if row.ID != target {
				continue
			}
			return nil, fmt.Errorf("record %s unreadable", row.ID)
		}
	}
	return nil, nil
}
`

const cbrFixtureUnbridged = `package gateway

import "fmt"

func (f *fileCapabilitySource) CapabilitySnapshot(model string) (any, error) {
	for _, row := range rows {
		if row.ID != model {
			continue
		}
		return nil, fmt.Errorf("record %s unreadable", row.ID)
	}
	return nil, nil
}
`

const cbrFixtureBridgedNoFailClosed = `package gateway

import "github.com/Mr2109/zerg-swarm/core/internal/contract"

func (f *fileCapabilitySource) CapabilitySnapshot(model string) (any, error) {
	targets := []string{model}
	if m, merr := contract.ModelIDMap(); merr == nil {
		if rid, ok := m.RegistryIDForFleet(model); ok && rid != model {
			targets = append(targets, rid)
		}
	}
	for _, target := range targets {
		if target != model {
			_ = target
		}
	}
	return nil, nil
}
`

const cbrFixtureMap2 = `{
  "schema": "zerg/model-id-map/v1",
  "entries": [
    {"registry_id": "example-35b-v2-1-5-35b-q4-k-m", "fleet_ids": ["example-35b-v2"]},
    {"registry_id": "gemma-4-26b-a4b-it-ud-q4-k-m", "fleet_ids": ["gemma-4-26B"]}
  ]
}
`

const cbrFixtureMapEmpty = `{"schema": "zerg/model-id-map/v1", "entries": []}`

func cbrMcFixture(base, sub, gw, mp string, withGW, withMap bool) string {
	d := filepath.Join(base, sub)
	_ = os.MkdirAll(filepath.Join(d, "core", "internal", "gateway"), 0o755)
	_ = os.MkdirAll(filepath.Join(d, "core", "internal", "contract"), 0o755)
	if withGW {
		_ = os.WriteFile(filepath.Join(d, cbrGWRel), []byte(gw), 0o644)
	}
	if withMap {
		_ = os.WriteFile(filepath.Join(d, cbrMapRel), []byte(mp), 0o644)
	}
	return d
}

// cbrSelfTest —— 成对负控（合成夹具 · 仓外 tmpdir）。
func cbrSelfTest(stdout io.Writer) int {
	type check struct {
		name string
		ok   bool
		det  string
	}
	var checks []check
	add := func(name string, ok bool, det string) {
		checks = append(checks, check{name, ok, det})
	}
	hasFinding := func(res cbrResult, face string) bool {
		for _, f := range res.Findings {
			if f.Face == face {
				return true
			}
		}
		return false
	}
	blob := func(res cbrResult) string {
		var parts []string
		for _, f := range res.Findings {
			parts = append(parts, "["+f.Face+"]"+f.Detail)
		}
		return strings.Join(parts, " | ")
	}

	base, err := os.MkdirTemp("", "capbridge-selftest-")
	if err != nil {
		base = os.TempDir()
	}
	defer os.RemoveAll(base)

	// ① 正控（必然判绿）：接桥好的夹具 ⇒ rc=0
	dGreen := cbrMcFixture(base, "green", cbrFixtureBridged, cbrFixtureMap2, true, true)
	rGreen := cbrCompute(dGreen)
	add("①正控（接桥齐）⇒ rc=0", rGreen.Rc == 0, fmt.Sprintf("rc=%d %s", rGreen.Rc, blob(rGreen)))

	// ② 负控（主病）：两套命名且**未接桥** ⇒ rc=1 且点名桥接线 + 件路径
	dBad := cbrMcFixture(base, "unbridged", cbrFixtureUnbridged, cbrFixtureMap2, true, true)
	rBad := cbrCompute(dBad)
	nBad := len(rBad.Findings)
	add("②负控（两套命名·未接桥）⇒ rc=1 且点名桥接线", rBad.Rc == 1 && hasFinding(rBad, "桥接线") && strings.Contains(blob(rBad), "RegistryIDForFleet(") && strings.Contains(blob(rBad), cbrGWRel),
		fmt.Sprintf("rc=%d 判红=%d %s", rBad.Rc, nBad, blob(rBad)))
	// 成对（✗ 禁恒绿）：正控 rc=0 与负控 rc=1 必须不同码
	add("②成对（正控 rc=0 ≠ 负控 rc=1 ⇒ 有鉴别力）", rGreen.Rc == 0 && rBad.Rc == 1 && rGreen.Rc != rBad.Rc,
		fmt.Sprintf("正控 rc=%d · 负控 rc=%d", rGreen.Rc, rBad.Rc))

	// ③ 负控：桥真源 entries 空 ⇒ rc=1 且点名桥真源
	dEmpty := cbrMcFixture(base, "mapempty", cbrFixtureBridged, cbrFixtureMapEmpty, true, true)
	rEmpty := cbrCompute(dEmpty)
	add("③负控（桥真源 entries 空）⇒ rc=1 且点名桥真源", rEmpty.Rc == 1 && hasFinding(rEmpty, "桥真源"),
		fmt.Sprintf("rc=%d %s", rEmpty.Rc, blob(rEmpty)))

	// ③b 负控：桥真源不在盘 ⇒ rc=1 且点名桥真源
	dNoMap := cbrMcFixture(base, "nomap", cbrFixtureBridged, "", true, false)
	rNoMap := cbrCompute(dNoMap)
	add("③b负控（桥真源不在盘）⇒ rc=1 且点名桥真源不在盘",
		rNoMap.Rc == 1 && hasFinding(rNoMap, "桥真源") && strings.Contains(blob(rNoMap), "不在盘"),
		fmt.Sprintf("rc=%d %s", rNoMap.Rc, blob(rNoMap)))

	// ④ 报告档（正控）：有 fail-closed 留痕 ⇒ 报告 0 项
	add("④报告档（正控 · 有 fail-closed 留痕）⇒ 报告 0 项", len(rGreen.Notes) == 0, fmt.Sprintf("note=%d", len(rGreen.Notes)))

	// ④b 报告档（负控）：无留痕 ⇒ 列出 1 项但 **不进退码**（rc 仍 0）
	dNoLog := cbrMcFixture(base, "nolog", cbrFixtureBridgedNoFailClosed, cbrFixtureMap2, true, true)
	rNoLog := cbrCompute(dNoLog)
	add("④b报告档（负控 · 无留痕）⇒ 列 1 项但 rc 仍 0（不进退码）",
		len(rNoLog.Notes) == 1 && rNoLog.Rc == 0, fmt.Sprintf("rc=%d note=%d", rNoLog.Rc, len(rNoLog.Notes)))

	// ⑤ 缺件 ⇒ rc=2 不给结论
	dNoGW := cbrMcFixture(base, "nogw", "", cbrFixtureMap2, false, true)
	rNoGW := cbrCompute(dNoGW)
	add("⑤缺件（网关来源件不在盘）⇒ rc=2 不给结论", rNoGW.Rc == 2 && strings.Contains(rNoGW.Bail, "不在盘"),
		fmt.Sprintf("rc=%d bail=%s", rNoGW.Rc, rNoGW.Bail))

	// ⑥ 取数面变了（无快照解析函数）⇒ rc=2 不给结论
	dNoFn := cbrMcFixture(base, "nofn", "package gateway\n\nfunc other() {}\n", cbrFixtureMap2, true, true)
	rNoFn := cbrCompute(dNoFn)
	add("⑥取数面变了（无快照解析函数）⇒ rc=2 不给结论", rNoFn.Rc == 2, fmt.Sprintf("rc=%d bail=%s", rNoFn.Rc, rNoFn.Bail))

	// ⑦ 未知旗标 ⇒ rc=2 用法错（stderr 明说）
	var eb strings.Builder
	rcFlag := capabilityBridgeGateMain(io.Discard, &eb, []string{"--no-such-flag"})
	add("⑦未知旗标 ⇒ rc=2 且 stderr 明说", rcFlag == 2 && strings.Contains(eb.String(), "unrecognized arguments: --no-such-flag"),
		fmt.Sprintf("rc=%d stderr=%q", rcFlag, eb.String()))

	// ⑧ 主负控在**默认档**（非自检面）也报红：cbrRun 取 rc=1 ⇒ 绿/红两态走同一条判读路
	var ob strings.Builder
	rcRun := cbrRun(&ob, dBad, false)
	add("⑧默认档（cbrRun）对未接桥夹具 rc=1 且正文含判红", rcRun == 1 && strings.Contains(ob.String(), "判红 2 项") && strings.Contains(ob.String(), "RegistryIDForFleet("),
		fmt.Sprintf("rc=%d out=%q", rcRun, ob.String()))

	bad := 0
	for _, c := range checks {
		if !c.ok {
			bad++
		}
	}
	fmt.Fprintf(stdout, "check-capability-bridge 自检（成对负控 · 合成夹具）：断言 %d 条 · 失败 %d 条\n", len(checks), bad)
	for _, c := range checks {
		tag := "PASS"
		tail := ""
		if !c.ok {
			tag = "FAIL"
			det := c.det
			if len([]rune(det)) > 400 {
				det = string([]rune(det)[:400])
			}
			tail = "  ⇐ " + det
		}
		fmt.Fprintf(stdout, "  %s %s%s\n", tag, c.name, tail)
	}
	if bad > 0 {
		return 1
	}
	return 0
}

// capabilityBridgeGateMain —— 入口（inproc 键 `check-capability-bridge`）。
func capabilityBridgeGateMain(stdout, stderr io.Writer, argv []string) int {
	rootOpt := ""
	jsonFlag, selfTest, listRules, help := false, false, false, false
	var positionals []string
	for k := 0; k < len(argv); k++ {
		a := argv[k]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		case a == "--json":
			jsonFlag = true
		case a == "--root":
			if k+1 >= len(argv) {
				cbrUsageErr(stderr, "argument --root: expected one argument")
				return 2
			}
			k++
			rootOpt = argv[k]
		case strings.HasPrefix(a, "--root="):
			rootOpt = a[len("--root="):]
		case strings.HasPrefix(a, "-") && a != "-":
			cbrUsageErr(stderr, "unrecognized arguments: "+a)
			return 2
		default:
			positionals = append(positionals, a)
		}
	}
	if help {
		io.WriteString(stdout, cbrHelpText)
		return 0
	}
	if len(positionals) > 1 {
		cbrUsageErr(stderr, "unrecognized arguments: "+strings.Join(positionals[1:], " "))
		return 2
	}
	if selfTest {
		return cbrSelfTest(stdout)
	}
	if listRules {
		fmt.Fprintln(stdout, cbrRules)
		fmt.Fprintf(stdout, "退码：%s\n", cbrRCS)
		return 0
	}
	root := "."
	if rootOpt != "" {
		root = rootOpt
	} else if len(positionals) == 1 {
		root = positionals[0]
	}
	return cbrRun(stdout, root, jsonFlag)
}

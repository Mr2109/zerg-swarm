// family_model_verify.go —— `zerg model verify <模型@机>`（缺口 GAP-20260926-04 · P0 · 命令面）。
//
// 病根（缺口正文逐字）：模型的**四方来源**（①主控下发 ②子端本地 ③模型档案 ④运行期实占）
// 只能各自单看 —— 没有一条命令能一次看出不一致（今晚撞到 gemma 的 ctx_window 声明 32768
// 与实际 131072 只能手工对拍）。
//
// 口径（Mr2109 2026-10-10 拍板 · 缺口注[5]，本件照它施工、未另定口径）：
//
//	命令名取 `model verify`（✗ 不用 `audit`）· 四方按账内已定型口径 · 人面 5 列（egg_id side
//	source value authority）+ **恒打 verdict**（三档闭集：一致 / 不一致 / 缺席）· `--json` 9 格
//	（`side` 四值闭集）· 退码 0 齐且一致 / 1 齐但判不一致 / 2 用法 / 4 鉴权 / 8 必需件取不到 /
//	11、12 透传 · **三态分离（缺席 ≠ 读不到(8) ≠ 不一致(1)）** · 用法错 stdout 0 字节 ·
//	本版四方中④（运行期实占）**无通路** ⇒ 如实打印「无通路」（该通路另立缺口账）。
//
// 四方真源（本单现读核实 · ✗ 未照抄转述件名）：
//
//	① 主控下发  = `core/internal/api/control.go:343` `json.Marshal(map[string]string{"model": req.Model})`
//	             （同形 `handlers.go` 一类面 · `egg_warmup.go`）—— 体里**只有 `model` 一键** ⇒ 对 argv/ctx 零影响。
//	② 子端本地  = `<HOME>/agent/agent_models.yaml`（子端 `agent/internal/registry/registry.go` 的注册表；条目含 `cmd`/`env_req`/`file`）。
//	③ 模型档案  = **意图** `gateway/fleet.yaml`（`config.FleetConfig.Models[名][机].CtxWindow`/`.Cmd`，`config.go:105`）
//	             ⊕ **事实** `<state>/egg-profiles/<名>.yaml`（`gateway.ProfileCtxWindow`，`eggprofile.go:34`）。
//	④ 运行期实占 = 引擎 `/slots` 的 `n_ctx` —— 本版**无通路**（如实打印「无通路」· 该通路另立缺口账）。
//
// 只读：全程零写、零副作用、不打第二枪（与只读面同一纪律 · `readonly.go` 头注）。
//
// ★ 本件自陈的**已代拍项**（拍板口径未逐字给到的三处，按最小 + 可辩原则落，并在此明列，便于复核）：
//
//  1. `--json` 9 格的具体名字 = `egg_id side source value authority verdict key state note`
//     （前 6 格对应人面 5 列 + verdict；后 3 格 `key`（比对键）/`state`（读数状态四值：present /
//     absent / unreadable / no_route）/`note`（说明））。
//  2. 行数 = **5 行**（`side` 仍是四值闭集）：③模型档案的**意图**与**事实**各占一行（`side` 同为
//     `profile`、`source` 分家）—— 因为口径逐字把 ③ 写成「意图 / 事实」两个真源。
//  3. 「谁说了算」的比对键取 `ctx_window`（缺口原症状即它）；本版把三方 ctx（②本机 argv `-c` /
//     ③意图 fleet `ctx_window` / ③事实 egg-profile `ctx_window`）摆在一起判 egg 级 verdict。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway"
)

// side 四值闭集（①主控下发 ②子端本地 ③模型档案 ④运行期实占）。
const (
	mvSideControl = "control"
	mvSideLocal   = "local"
	mvSideProfile = "profile"
	mvSideRuntime = "runtime"
)

// verdict 三档闭集（**恒打** · 拍板口径逐字）。
const (
	mvAgree    = "一致"
	mvDisagree = "不一致"
	mvAbsent   = "缺席"
)

// state 读数状态四值（`--json` 的 `state` 格）：present / absent / unreadable / no_route。
const (
	mvPresentSt    = "present"
	mvAbsentSt     = "absent"
	mvUnreadableSt = "unreadable"
	mvNoRouteSt    = "no_route"
)

// mvFields —— `--json` 的 9 格（序即登记序；命令树 `fields:` 逐字同此）。
var mvFields = []string{"egg_id", "side", "source", "value", "authority", "verdict", "key", "state", "note"}

// mvDisplay —— 人面 5 列 + verdict（第 6 列 · 恒打）。
var mvDisplay = []string{"egg_id", "side", "source", "value", "authority", "verdict"}

// mvCtxKey —— 本版比对键（缺口原症状 = ctx_window）。
const mvCtxKey = "ctx_window"

// mvCtxRE 从一段 `cmd` 文本里取 `-c <n>`（llama-server 的上下文开关）。
// 兼容两种名册写法：数组形 `"-c", "65536"` 与字符串形 `… -c 65536 …`。
var mvCtxRE = regexp.MustCompile(`-c[",\s]+"?([0-9]+)`)

// mvVerdict —— egg 级三档闭集（纯函数 · 便于用合成输入钉判据）。
//
// 口径：把**有 ctx 读数**的各方摆在一起 —— 不足两方 ⇒ 缺席（没得比）；两方及以上且全等 ⇒ 一致；
// 否则 ⇒ 不一致。control 只发模型名、无 ctx；runtime 本版无通路 —— 两者都不进 ctx 比对。
func mvVerdict(vals []int, present []bool) string {
	got := []int{}
	for i := range present {
		if present[i] {
			got = append(got, vals[i])
		}
	}
	if len(got) < 2 {
		return mvAbsent
	}
	for _, v := range got[1:] {
		if v != got[0] {
			return mvDisagree
		}
	}
	return mvAgree
}

// mvExit —— 退码（纯函数）。三态分离：读不到 ⇒ 8（不给结论）；否则 一致 ⇒ 0 · 其余 ⇒ 1。
func mvExit(verdict string, unreadable bool) int {
	if unreadable {
		return exitBlocked
	}
	if verdict == mvAgree {
		return exitOK
	}
	return exitFail
}

// mvCtxOfCmd —— 从 `cmd` 文本取 ctx（无 ⇒ ok=false）。
func mvCtxOfCmd(cmd string) (int, bool) {
	if cmd == "" {
		return 0, false
	}
	m := mvCtxRE.FindStringSubmatch(cmd)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// mvLocalRegistryPath —— 子端本地注册表路径（子端 `registry.go` 读的 `agent_models.yaml`）。
func mvLocalRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "agent", "agent_models.yaml")
}

// mvReadLocalRegistry 读子端本地注册表里某模型的条目：返回 (state, cmdText, path)。
// state ∈ present / absent / unreadable（**缺席 ≠ 读不到**：件读不到 = unreadable ⇒ 退 8；
// 件在、无此条 = absent ⇒ verdict 缺席）。
func mvReadLocalRegistry(model string) (string, string, string) {
	path := mvLocalRegistryPath()
	if path == "" {
		return mvUnreadableSt, "", ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return mvUnreadableSt, "", path
	}
	inBlock, found, cmdText := false, false, ""
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' { // 顶层键（无缩进）
			i := strings.Index(line, ":")
			if i <= 0 {
				continue
			}
			inBlock = strings.TrimSpace(line[:i]) == model
			if inBlock {
				found = true
			}
			continue
		}
		if !inBlock {
			continue
		}
		if strings.HasPrefix(t, "cmd:") {
			cmdText = strings.TrimSpace(strings.TrimPrefix(t, "cmd:"))
		}
	}
	if !found {
		return mvAbsentSt, "", path
	}
	return mvPresentSt, cmdText, path
}

// mvReadEggProfile 读模型档案（事实）：返回 (state, valueText, path)。
// 三态分家：目录读不到 = unreadable（退 8）；无该模型档案 = absent（缺席 · ≠ 不一致）。
func mvReadEggProfile(model string) (string, string, string) {
	dir := gateway.EggProfileDir()
	if dir == "" {
		return mvUnreadableSt, "", "(取不到 egg-profiles 目录)"
	}
	if _, err := os.ReadDir(dir); err != nil {
		return mvUnreadableSt, "", filepath.Join(dir, model+".yaml")
	}
	p := filepath.Join(dir, model+".yaml")
	if n, ok := gateway.ProfileCtxWindow(model); ok {
		return mvPresentSt, strconv.Itoa(n), p
	}
	return mvAbsentSt, "", p
}

// mvEmitJSON 出 `--json`（先按登记白名单核字段），再回**本命令自己的退码**（emitSelected 恒回 0）。
// 失败侧（读不到）若已 `setErr` ⇒ 包封里同出 `error` 块（`emitEnvelopeWith` 的既有口子）。
func mvEmitJSON(inv *invocation, stdout, stderr io.Writer, rows []map[string]string, rc int) int {
	if e := emitSelected(stdout, stderr, inv, inv.path, inv.fields, rows); e != exitOK {
		return e
	}
	return rc
}

// cmdModelVerify —— `zerg model verify <模型@机> [--json <字段>]`（只读对账面）。
func cmdModelVerify(inv *invocation, stdout, stderr io.Writer) int {
	// ── 用法面先判（零副作用 · stdout 0 字节 · 拍板：用法错时 stdout 0 字节）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺对象（模型@机）")
		fmt.Fprintf(stderr, "%s: `model verify` 要一个对象 `<模型@机>`（例：`%s model verify example-35b-v2@<机>`）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一个对象）")
		fmt.Fprintf(stderr, "%s: `model verify` 只吃**一个**对象 `<模型@机>`（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	target := strings.TrimSpace(inv.args[0])
	model, machine, hasAt := strings.Cut(target, "@")
	model, machine = strings.TrimSpace(model), strings.TrimSpace(machine)
	if !hasAt || model == "" || machine == "" {
		inv.setErr("usage", "bad_target", "对象要 `<模型>@<机>` 形态")
		fmt.Fprintf(stderr, "%s: 对象要 `<模型>@<机>` 形态（给的是 %q）—— 缺 `@` / 缺模型 / 缺机\n", progName, target)
		return exitUsage
	}

	// ── `--json` 字段面前置判定（与 `cmdModelShow` / `cmdCoreDaemonLs` 同款）──
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		allowed := map[string]bool{}
		for _, f := range mvFields {
			allowed[f] = true
		}
		for _, f := range inv.fields {
			if !allowed[f] {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
	}

	eggID := model + "@" + machine

	// ── 读名册件（意图侧真源）：读不到 ⇒ 8（不给结论）──
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到名册件（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	fleetRel := filepath.Join("gateway", "fleet.yaml")
	cfg, err := config.LoadFleetConfig(filepath.Join(root, fleetRel))
	if err != nil {
		inv.setErr("blocked", "fleet_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不到名册件 %s：%v（不给结论 · 退码 8）\n", progName, fleetRel, err)
		return exitBlocked
	}
	cands, ok := cfg.Models[model]
	if !ok {
		ids := make([]string, 0, len(cfg.Models))
		for k := range cfg.Models {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		inv.setErr("usage", "unknown_model", "模型不在册")
		fmt.Fprintf(stderr, "%s: 名册里没有 %q（点名不在册）\n", progName, model)
		if s := nearestName(model, ids); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		return exitUsage
	}
	var cand *config.ModelCandidate
	for i := range cands {
		if cands[i].Host == machine {
			cand = &cands[i]
			break
		}
	}

	// ── 四方读数 ──
	// ① 主控下发（只发 model 名）
	controlValue := ""
	if cand != nil {
		b, _ := json.Marshal(map[string]string{"model": model})
		controlValue = string(b)
	}
	// ② 子端本地
	localState, localCmd, localPath := mvReadLocalRegistry(model)
	localCtx, localHasCtx := mvCtxOfCmd(localCmd)
	// ③ 意图
	intentCtx, intentHasCtx, intentValue := 0, false, ""
	if cand != nil {
		intentCtx = cand.CtxWindow
		intentHasCtx = cand.CtxWindow > 0
		intentValue = mvCtxKey + "=" + strconv.Itoa(cand.CtxWindow)
		if len(cand.Cmd) > 0 {
			intentValue += " · cmd=" + strings.Join(cand.Cmd, " ")
		}
	}
	// ③ 事实
	factState, factValue, factPath := mvReadEggProfile(model)
	factCtx, factHasCtx := 0, false
	if factState == mvPresentSt {
		if n, err := strconv.Atoi(factValue); err == nil {
			factCtx, factHasCtx = n, true
		}
	}

	// ── egg 级 verdict + 退码（三态分离：读不到(8) 先于 缺席/不一致(1)）──
	verdict := mvVerdict([]int{localCtx, intentCtx, factCtx}, []bool{localHasCtx, intentHasCtx, factHasCtx})
	unreadable := localState == mvUnreadableSt || factState == mvUnreadableSt
	rc := mvExit(verdict, unreadable)

	// ── 5 行（side 四值闭集；③ 意图/事实各一行）──
	rows := []map[string]string{}
	add := func(side, source, value, authority, state, note string) {
		rows = append(rows, map[string]string{
			"egg_id": eggID, "side": side, "source": source, "value": value,
			"authority": authority, "verdict": verdict, "key": mvCtxKey,
			"state": state, "note": note,
		})
	}
	// ① 主控下发
	srcControl := "core/internal/api/control.go:343（json.Marshal({\"model\":req.Model})）"
	if cand != nil {
		add(mvSideControl, srcControl, controlValue,
			"零（主控只发模型名 ⇒ 对 argv/ctx 零影响）", mvPresentSt, "下发体只有 model 一键")
	} else {
		add(mvSideControl, srcControl, mvAbsent,
			"零（主控只发模型名）", mvAbsentSt, "名册里该机没有这个候选 ⇒ 主控无从下发")
	}
	// ② 子端本地（argv 面）
	srcLocal := localPath
	if srcLocal == "" {
		srcLocal = "~/agent/agent_models.yaml（取不到 HOME）"
	}
	localAuth := "argv 面·高（卵清单优先 · hatch_spec）"
	switch localState {
	case mvUnreadableSt:
		add(mvSideLocal, srcLocal, mvAbsent, localAuth, mvUnreadableSt,
			"子端本机注册表读不到 ⇒ 不给结论（退码 8）")
	case mvAbsentSt:
		add(mvSideLocal, srcLocal, mvAbsent, localAuth, mvAbsentSt,
			"注册表里没有这个模型 ⇒ 缺席（≠ 不一致）")
	default:
		v := localCmd
		if v == "" {
			v = "（条目在 · 无 cmd）"
		}
		add(mvSideLocal, srcLocal, v, localAuth, mvPresentSt,
			"子端本地注册表条目（cmd 里 -c 即 argv 面 ctx）")
	}
	// ③ 模型档案 · 意图
	srcIntent := fleetRel
	if cand != nil {
		add(mvSideProfile, srcIntent, intentValue,
			"预算面·中（档案事实 > fleet 声明 > 默认 · tokencap）", mvPresentSt, "fleet.yaml 该候选（意图）")
	} else {
		add(mvSideProfile, srcIntent, mvAbsent,
			"预算面·中（档案事实 > fleet 声明 > 默认）", mvAbsentSt, "fleet.yaml 里该机没有这个候选 ⇒ 缺席")
	}
	// ③ 模型档案 · 事实
	factAuth := "预算面·高（档案事实优先 · 实测）"
	switch factState {
	case mvUnreadableSt:
		add(mvSideProfile, factPath, mvAbsent, factAuth, mvUnreadableSt,
			"egg-profiles 目录读不到 ⇒ 不给结论（退码 8）")
	case mvAbsentSt:
		add(mvSideProfile, factPath, mvAbsent, factAuth, mvAbsentSt,
			"没有该模型的实测档案（未标定 / 档案无 ctx_window）⇒ 缺席")
	default:
		add(mvSideProfile, factPath, factValue, factAuth, mvPresentSt, "egg-profiles 实测档案（事实）")
	}
	// ④ 运行期实占
	add(mvSideRuntime, "引擎 /slots 的 n_ctx", "无通路",
		"最高（引擎实占）—— 本版无通路", mvNoRouteSt,
		"运行期实占本版无通路（如实打印 · 该通路另立缺口账）")

	// ── 出账 + 退码 ──
	if unreadable {
		inv.setErr("blocked", "source_unreadable", "有必需件读不到")
		fmt.Fprintf(stderr, "%s: 有必需件读不到 ⇒ **不给结论**（退码 8）—— 三态分离：缺席 ≠ 读不到(8) ≠ 不一致(1)\n", progName)
		fmt.Fprintf(stderr, "error.kind=blocked · retryable=true · remedy=fix_precondition\n")
	}
	if inv.jsonGiven {
		return mvEmitJSON(inv, stdout, stderr, rows, rc)
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(mvDisplay))
		for _, f := range mvDisplay {
			line = append(line, r[f])
		}
		table = append(table, line)
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, mvDisplay, table)
	fmt.Fprintf(stderr, "%s: %s · verdict=%s · 退码 %d（三档闭集：一致/不一致/缺席 · 三态分离：缺席 ≠ 读不到(8) ≠ 不一致(1) · ④本版无通路）\n",
		progName, eggID, verdict, rc)
	return rc
}

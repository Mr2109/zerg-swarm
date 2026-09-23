// family_fusion.go —— 模型与命令面的融合面（§十八.3 四件 · §十八.1 三层分工 · §十八.2 三条铁律）。
//
// 本件落「四件」里的**后三件**（第一件「能力路由」的 id 规范化真源由 T-41 落成）：
//
//	① `zerg ask`        问一次推理、**不落任务队列**（`task submit` 的对偶）——
//	                    能力筛是**硬**筛（`--capability`，可重复），`--prefer` 只加权不排除。
//	② `zerg plan`       **算**：产出一份**意图件**（M6 包封 · `kind:"Plan"` · 七语义件 `F1`–`F7`
//	                    + 四附加件）—— **零副作用**（不触后端写面、不落系统状态）。
//	③ `zerg apply <件>` **做**：**只吃那一份件**，不重新规划 —— 四层按序判，
//	                    **任一层不过 ⇒ 后续层不跑**（§十八.3-3 判据③）。
//	④ 意图 schema       真源 = `core/internal/contract/intent-plan.json`（四闭集 + 必备字段），
//	                    生产侧（`plan`）与消费侧（`apply`）**读同一份**，不各写一份。
//
// 退码纪律（§十八.3-4 · `调研-R2:466`：**一律不新增码** ✗）：
//
//	词表外 ⇒ `2`（usage）；在词表但无候选 ⇒ `1`（failed）；件被改 / 旧值变 / 换机 /
//	缺确认 / 传了规划旗标 ⇒ `2`。
//
// 依赖次序（§十八.3 逐字「不许倒」）：能力路由（T-41 的 id 规范化真源）→ `ask` →
// 意图 schema → `plan`/`apply` —— 本件按序落，且 `apply` 的 L2 层就是「引用回指既有编号」。
//
// 本版边界（照实说）：`apply` 的 **L4 人在环**到「确认档齐了」为止 —— 写面在批 A 起
// 一律未开放（§6.2 零写操作），故齐了也**不给结论**（退码 `2` · `detail=not_opened`），
// 与 `cmdGuarded` 同一档口径；真执行等人批通道（M18 `C4` ② 段）通。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 意图件（Plan 对象）的形状（§十八.3-3）──────────────────────────────────────
//
// 七语义件 `F1`–`F7`（动作 / 目标 / 作用域 / 前置 / 预期 / 回滚 / 依据）
// + 四附加件（`plan_id` / `plan_digest` / `approval` / `status`）。
// 一律**结构体**（不是 map）：序列化顺序确定 ⇒ `plan_digest` 可复算、`apply` 可对拍。

type planTarget struct {
	Kind string `json:"kind"` // 闭集：model / agent / core / task
	Name string `json:"name"`
}

type planScope struct {
	Nodes []string `json:"nodes"`
	Layer string   `json:"layer"`
	Host  string   `json:"host"` // 产出这件时的主机（「换机 ⇒ 2」的判据来源）
}

type planPrecondition struct {
	Ref    string `json:"ref"`    // 指回一处可读的现状（命令 + 对象）
	Expect string `json:"expect"` // 旧值（缺失 ⇒ 空串，语义 = 「只判存在」）
}

type planExpected struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type planRollback struct {
	Ref string `json:"ref"` // 退点（回指既有编号 —— L2 层判）
}

type planEvidence struct {
	Source   string `json:"source"`   // 出处类型（human / 命令面 / 证据单）
	Ref      string `json:"ref"`      // 回指既有编号（L2 层判）
	Producer string `json:"producer"` // 产出者（MF1：模型名 + 版本 + 引擎 / 或人）
}

type planApproval struct {
	DryRun bool   `json:"dry_run"` // 这件是干跑产出的吗（L3 层判）
	By     string `json:"by"`      // 谁批的（人 —— AI 填不了这一格 · §十七 铁律④）
}

type planDoc struct {
	Action        string             `json:"action"` // F1
	Target        planTarget         `json:"target"` // F2
	Scope         planScope          `json:"scope"`  // F3
	Preconditions []planPrecondition `json:"preconditions"`
	Expected      planExpected       `json:"expected"` // F5
	Rollback      planRollback       `json:"rollback"` // F6
	Evidence      planEvidence       `json:"evidence"` // F7
	PlanID        string             `json:"plan_id"`
	PlanDigest    string             `json:"plan_digest"`
	Approval      planApproval       `json:"approval"`
	Status        string             `json:"status"`
}

// planEnvelopeMeta —— 包封的 `meta`（结构体 ⇒ 序列化确定）。
type planEnvelopeMeta struct {
	Count     int    `json:"count"`
	Source    string `json:"source"`
	Changed   string `json:"changed"`
	Host      string `json:"host"`
	CreatedAt string `json:"created_at"`
}

// planEnvelope —— M6 六键包封（`items` 里恒一枚 `Plan`）。
type planEnvelope struct {
	Schema    string           `json:"schema"`
	Kind      string           `json:"kind"`
	Items     []planDoc        `json:"items"`
	Meta      planEnvelopeMeta `json:"meta"`
	Warnings  []string         `json:"warnings"`
	Truncated bool             `json:"truncated"`
}

// ---- 意图件真源的读法（读侧只读这一处）--------------------------------------------

func intentSpec(stderr io.Writer) (*contract.IntentPlanSpec, bool) {
	ip, err := contract.IntentPlan()
	if err != nil {
		fmt.Fprintf(stderr, "%s: 意图件真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return nil, false
	}
	return ip, true
}

// planCanonical —— 件的**规范字节**：整份包封按结构体序序列化（单行）。
// `apply` 用它做两件事：① 复算 `plan_digest`；② 与盘上原字节对拍（**改一字节即现形**）。
func planCanonical(env planEnvelope) []byte {
	b, _ := json.Marshal(env)
	return b
}

// planDigestOf —— 规范摘要：对**整份包封**求 sha256（把 `plan_id` 与 `plan_digest` 两个
// 「由摘要派生的格」先清空再算 ⇒ 可复算：产出侧与消费侧算的是同一串字节）。
//
// ★ 为什么要在函数里**复制一份 Items**：Go 传结构体是值传递，但 `Items` 是**切片**
//
//	—— 片头值被复制、**底层数组是同一个**。不复制就会把调用方的 `plan_id`/`plan_digest`
//	真的清空（实测症状：`apply` 判摘要前先把自己的两格擦了，于是规范字节对拍必然不符）。
func planDigestOf(env planEnvelope) string {
	cp := env
	items := make([]planDoc, len(env.Items))
	copy(items, env.Items)
	cp.Items = items
	if len(cp.Items) > 0 {
		cp.Items[0].PlanDigest = ""
		cp.Items[0].PlanID = ""
	}
	sum := sha256.Sum256(planCanonical(cp))
	return hex.EncodeToString(sum[:])
}

// planIDOf —— 干跑集指纹（§十五 `RC12`：干跑产出的对象集指纹 `plan_id`，执行时逐条对不上即拒）。
func planIDOf(digest string) string {
	if len(digest) < 12 {
		return "PLAN-" + digest
	}
	return "PLAN-" + digest[:12]
}

func planDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_PLAN_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "plans")
}

func planHost() string {
	if h := strings.TrimSpace(os.Getenv("ZERG_HOST")); h != "" {
		return h
	}
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

// ── ① `zerg ask` ───────────────────────────────────────────────────────────────

// askFields —— `--json` 面的全部字段（K1：机器面先定）。
var askFields = []string{"model", "host", "backend", "matched", "capabilities", "registry_id", "reply", "dry_run"}

// askRoute —— 一条候选路线（能力筛之后剩下的一行）。
type askRoute struct {
	Model        string
	Host         string
	Backend      string
	RegistryID   string
	Capabilities []string
}

// cmdAsk —— `zerg ask <提示> [--capability 名]… [--prefer 名]… [--model 名] [--node 名]…
//
//	[--min-ctx n] [--min-mem-gb n] [--no-fallback] [--dry-run] [--timeout 时长] [--json 字段]
//
// 退码：能力名**不在词表** ⇒ `2`；**在词表但无候选** ⇒ `1`（**不新增码**）。
// `--dry-run` = 只算路线、**零副作用**（不推理、不占槽、不发请求）。
func cmdAsk(inv *invocation, stdout, stderr io.Writer) int {
	// 0) 词表闸（`§十八.3-4`：词表真源 = modelreg.CapabilityNames，**18 值闭集**）——
	//    这一闸**在任何 HTTP 之前**判 ⇒ 词表外的名字永远不碰主控。
	caps := []string{}
	for _, c := range inv.flagVals("--capability") {
		for _, one := range strings.Split(c, ",") {
			one = strings.TrimSpace(one)
			if one == "" {
				continue
			}
			if !modelreg.CapabilityNames[one] {
				inv.setErr("usage", "bad_capability",
					fmt.Sprintf("能力名 %q 不在词表里", one))
				fmt.Fprintf(stderr, "%s: 能力名 %q **不在词表**里（`--capability` 只认闭集）⇒ 退码 2\n", progName, one)
				fmt.Fprintf(stderr, "词表真源：modelreg.CapabilityNames（18 值闭集）· 今能判的 5 条：text / vision / tools / embedding / rerank\n")
				fmt.Fprintf(stderr, "error.kind=usage · detail=bad_capability · retryable=false（§十八.3-4：词表外 ⇒ 2 · **不新增码**）\n")
				return exitUsage
			}
			caps = append(caps, one)
		}
	}
	prefer := []string{}
	for _, p := range inv.flagVals("--prefer") {
		for _, one := range strings.Split(p, ",") {
			if one = strings.TrimSpace(one); one != "" {
				if !modelreg.CapabilityNames[one] {
					inv.setErr("usage", "bad_capability", fmt.Sprintf("--prefer %q 不在词表里", one))
					fmt.Fprintf(stderr, "%s: `--prefer` 的能力名 %q 不在词表里 ⇒ 退码 2\n", progName, one)
					return exitUsage
				}
				prefer = append(prefer, one)
			}
		}
	}
	prompt := strings.TrimSpace(strings.Join(inv.args, " "))
	if prompt == "" {
		inv.setErr("usage", "missing_prompt", "缺提示（`zerg ask <提示>`）")
		fmt.Fprintf(stderr, "%s: `zerg ask` 缺提示 —— 用法：zerg ask <提示> [--capability 名]… [--dry-run]\n", progName)
		return exitUsage
	}

	// 1) 取路由表 + 能力快照（**只读**：GET /api/fleet/models + /api/models/registry）
	c := newClient()
	var fleet jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &fleet, stderr); rc != exitOK {
		return rc
	}
	var reg jsonObj
	if rc := fetchInv(inv, c, "/api/models/registry", &reg, stderr); rc != exitOK {
		return rc
	}

	// 2) 能力筛（**先**能力、**后**打分 —— §十八.3-4 的次序逐字）
	routes := askFilter(fleet, reg, caps, inv)
	if len(routes) == 0 {
		// 在词表但无候选 ⇒ `1`（**不新增码**）。缺证据 ≠ 有证据（`调研-R2` `R2-D1`）。
		inv.setErr("failed", "no_capability_candidate",
			"能力在词表里，但路由表里没有任何一条候选**证过**该能力")
		fmt.Fprintf(stderr, "%s: 能力 %v 在词表里，但**无候选**（没任何一条证据过它）⇒ 退码 1\n",
			progName, orDash(strings.Join(caps, ",")))
		fmt.Fprintf(stderr, "口径：缺证据 ≠ 有证据 —— 能力面只认**断言**（source+evidence+engines），不认「大概支持」\n")
		fmt.Fprintf(stderr, "error.kind=failed · detail=no_capability_candidate · retryable=false\n")
		return exitFail
	}

	best := routes[0]
	row := map[string]string{
		"model":        best.Model,
		"host":         best.Host,
		"backend":      best.Backend,
		"registry_id":  best.RegistryID,
		"matched":      strings.Join(caps, ","),
		"capabilities": strings.Join(best.Capabilities, ","),
		"dry_run":      fmt.Sprintf("%t", inv.dryRun),
	}

	if inv.dryRun {
		// 零副作用：只打印算出来的路线 —— 不推理、不占槽、不发任何请求。
		fmt.Fprintf(stdout, "路线（--dry-run · 零副作用 —— 未推理、未占槽、未发请求）\n")
		fmt.Fprintf(stdout, "  模型     : %s（%s · %s）\n", best.Model, best.Host, best.Backend)
		fmt.Fprintf(stdout, "  能力命中 : %s\n", orDash(strings.Join(caps, ",")))
		fmt.Fprintf(stdout, "  候选条数 : %d（能力筛后）\n", len(routes))
		if inv.jsonGiven {
			row["reply"] = ""
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, askFields, row)
		}
		return exitOK
	}

	// 3) 真问一次（这就是 `task submit` 的对偶：**不进队列**、没有 worktree、没有复查模型）
	reply, rerr := askInfer(best, prompt, inv)
	if rerr != nil {
		inv.setErr("failed", "inference_failed", rerr.Error())
		fmt.Fprintf(stderr, "%s: 推理失败：%v\n", progName, rerr)
		return exitFail
	}
	row["reply"] = reply
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, askFields, row)
	}
	fmt.Fprintln(stdout, reply)
	return exitOK
}

// askFilter 是**能力路由**的判定器（一处实现，`ask` 与将来的路由面共用）。
//
// 次序（§十八.3-4 逐字）：L3 作用域（`--node`）→ `--model` 收窄 → **能力筛（硬）** → 打分（软）。
// 能力证据取自 `/api/models/registry` 的能力快照，经 **T-41 的 id 规范化真源**（`registry_id`）
// 与路由表接起来 —— 这条接缝不拍（`R2-P1`），`ask` 就永远无表可查。
func askFilter(fleet, reg jsonObj, caps []string, inv *invocation) []askRoute {
	// registry_id ⇒ 记录（能力断言）
	byRegistry := map[string]jsonObj{}
	for _, it := range asList(reg["records"]) {
		if o := asObj(it); o != nil {
			if id, _ := o["id"].(string); id != "" {
				byRegistry[id] = o
			}
		}
	}
	nodeWant := map[string]bool{}
	for _, n := range inv.nodes {
		nodeWant[n] = true
	}
	modelWant := strings.TrimSpace(inv.modelWant)
	minCtx := atoiSafe(inv.flagVal("--min-ctx"))
	minMem := atofSafe(inv.flagVal("--min-mem-gb"))

	out := []askRoute{}
	for _, it := range asList(fleet["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		id, _ := o["id"].(string)
		host, _ := o["host"].(string)
		backend, _ := o["backend"].(string)
		registryID, _ := o["registry_id"].(string)
		if modelWant != "" && id != modelWant {
			continue
		}
		if len(nodeWant) > 0 && !nodeWant[host] {
			continue
		}
		if minMem > 0 {
			if v, ok := o["mem_gb"].(float64); !ok || v < minMem {
				continue
			}
		}
		// 能力筛是**硬**筛：没有 registry_id（接不上能力面）或没证过该能力 ⇒ 不是候选。
		rec, ok := byRegistry[registryID]
		if !ok || registryID == "" {
			continue
		}
		if minCtx > 0 {
			if v, ok := rec["context_window"].(float64); !ok || int(v) < minCtx {
				continue
			}
		}
		proven := map[string]bool{}
		for _, cit := range asList(rec["capabilities"]) {
			co := asObj(cit)
			if co == nil {
				continue
			}
			name, _ := co["name"].(string)
			val, _ := co["value"].(bool)
			if name != "" && val {
				proven[name] = true
			}
		}
		hit := []string{}
		pass := true
		for _, want := range caps {
			if proven[want] {
				hit = append(hit, want)
				continue
			}
			pass = false
		}
		if !pass {
			continue
		}
		all := []string{}
		for k := range proven {
			all = append(all, k)
		}
		sort.Strings(all)
		out = append(out, askRoute{Model: id, Host: host, Backend: backend,
			RegistryID: registryID, Capabilities: all})
	}
	// 打分（软）：`--prefer` 只**加权不排除** —— 命中的排在前面，其余保持字典序（可复现）。
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := 0, 0
		for _, p := range inv.flagVals("--prefer") {
			for _, one := range strings.Split(p, ",") {
				one = strings.TrimSpace(one)
				if one == "" {
					continue
				}
				if contract.Has(out[i].Capabilities, one) {
					si++
				}
				if contract.Has(out[j].Capabilities, one) {
					sj++
				}
			}
		}
		if si != sj {
			return si > sj
		}
		if out[i].Model != out[j].Model {
			return out[i].Model < out[j].Model
		}
		return out[i].Host < out[j].Host
	})
	return out
}

// askInfer 真问一次（经**网关**，不直连引擎、不持端口 —— §十八.1 铁律 Ⅰ）。
func askInfer(r askRoute, prompt string, inv *invocation) (string, error) {
	body := map[string]any{
		"model":    r.Model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(body)
	hc := &http.Client{Timeout: 120 * time.Second}
	resp, err := hc.Post(statepath.GatewayBaseURL()+"/v1/chat/completions", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("网关返回不是可解析的 chat 响应（HTTP %d）", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("网关返回里没有 choices（HTTP %d）", resp.StatusCode)
	}
	return parsed.Choices[0].Message.Content, nil
}

// ── ② `zerg plan` ──────────────────────────────────────────────────────────────

// planFields —— `--json` 面的全部字段。
var planFields = []string{"plan_id", "plan_digest", "action", "target_kind", "target_name",
	"nodes", "host", "status", "path", "layers_passed"}

// cmdPlan —— `zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--out 件] [--json 字段]`
//
// **零副作用**（§十八.3-2 判据⑤）：它只读现状 + 写**一件意图件**到落点目录，不触任何写面。
func cmdPlan(inv *invocation, stdout, stderr io.Writer) int {
	ip, ok := intentSpec(stderr)
	if !ok {
		return exitUsage
	}
	// 族 + 动作（前两个位置参数 —— 照 §十八.3-2 的用法串）
	args := []string{}
	for _, a := range inv.args {
		if strings.TrimSpace(a) != "" {
			args = append(args, a)
		}
	}
	if len(args) < 2 {
		inv.setErr("usage", "missing_action", "缺「族 动作」")
		fmt.Fprintf(stderr, "%s: 用法：zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--out <件>]\n", progName)
		fmt.Fprintf(stderr, "可用的「族.动作」：%s\n", strings.Join(ip.Actions, " / "))
		return exitUsage
	}
	family, action := args[0], args[1]
	targets := args[2:]
	actionID := family + "." + action
	if !contract.Has(ip.Actions, actionID) {
		inv.setErr("usage", "bad_action", fmt.Sprintf("%q 不在动作闭集里（plan schema 的四闭集之一）", actionID))
		fmt.Fprintf(stderr, "%s: 动作 %q **不在闭集**里 ⇒ 退码 2（§十八.3-3：不在闭集 ⇒ 2）\n", progName, actionID)
		fmt.Fprintf(stderr, "闭集真源：core/internal/contract/intent-plan.json 的 actions —— %s\n", strings.Join(ip.Actions, " / "))
		return exitUsage
	}
	// 目标闭集（`target.kind` 由族名推出；族名不在四值里 ⇒ 2）
	kind := family
	if !contract.Has(ip.TargetKinds, kind) {
		inv.setErr("usage", "bad_target_kind", fmt.Sprintf("族名 %q 不是合法 target.kind", kind))
		fmt.Fprintf(stderr, "%s: 族名 %q 不在 target.kind 闭集（%s）里 ⇒ 退码 2\n",
			progName, kind, strings.Join(ip.TargetKinds, " / "))
		return exitUsage
	}
	// 目标必须回指一个**既有编号**（§十七 `SD1`：目标只能承接，回指不上 ⇒ 2）——本版取
	// 证据/退点的编号闭集（`dev-targets.json` 的 D/E/F/G 四族），与提案件通道**同一张表**。
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		inv.setErr("failed", "empty_target_ledger", "编号闭集读不出来")
		fmt.Fprintf(stderr, "%s: 编号闭集读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitUsage
	}
	refWant := strings.TrimSpace(inv.flagVal("--target-ref"))
	if refWant == "" {
		refWant = strings.TrimSpace(inv.flagVal("--target"))
	}
	if refWant == "" && len(ids) > 0 {
		refWant = "待办:" + ids[0]
	}
	if !refExists(refWant, ids) {
		inv.setErr("usage", "unresolvable_ref", fmt.Sprintf("依据/退点 %q 回指不上既有编号", refWant))
		fmt.Fprintf(stderr, "%s: 依据回指 %q **回指不上**既有编号 ⇒ 退码 2（§十七 SD1「回指不上 ⇒ exit=2」）\n", progName, refWant)
		return exitUsage
	}

	name := ""
	if len(targets) > 0 {
		name = targets[0]
	}
	if name == "" {
		inv.setErr("usage", "missing_target", "缺对象名")
		fmt.Fprintf(stderr, "%s: `plan %s %s` 缺对象名（`--confirm` 要比的就是它）\n", progName, family, action)
		return exitUsage
	}
	// 前置（F4）：`--expect` 给旧值（可重复：一条前置一行）。
	pre := []planPrecondition{}
	for _, e := range inv.flagVals("--expect") {
		pre = append(pre, planPrecondition{Ref: actionID + " " + name, Expect: strings.TrimSpace(e)})
	}
	if len(pre) == 0 {
		pre = append(pre, planPrecondition{Ref: actionID + " " + name, Expect: ""})
	}
	nodes := dedupe(inv.nodes)
	doc := planDoc{
		Action:        actionID,
		Target:        planTarget{Kind: kind, Name: name},
		Scope:         planScope{Nodes: nodes, Layer: "node", Host: planHost()},
		Preconditions: pre,
		Expected:      planExpected{Before: pre[0].Expect, After: "written"},
		Rollback:      planRollback{Ref: "回滚方式见 dev rollback（本件只记退点回指）"},
		Evidence:      planEvidence{Source: "human", Ref: refWant, Producer: "命令面 zerg plan"},
		Approval:      planApproval{DryRun: inv.dryRun},
		Status:        "proposed",
	}
	env := planEnvelope{
		Schema: contractSchema,
		Kind:   "Plan",
		Items:  []planDoc{doc},
		Meta: planEnvelopeMeta{Count: 1, Source: "local（本机 · 零副作用）", Changed: "false",
			Host: planHost(), CreatedAt: time.Now().Format(time.RFC3339)},
		Warnings:  []string{},
		Truncated: false,
	}
	digest := planDigestOf(env)
	env.Items[0].PlanDigest = digest
	env.Items[0].PlanID = planIDOf(digest)
	canon := planCanonical(env)

	// 落点：`--out <件>` 优先，否则 `$ZERG_PLAN_DIR`（缺省 ~/.zerg/plans/<plan_id>.json）
	out := strings.TrimSpace(inv.flagVal("--out"))
	if out == "" {
		dir := planDir()
		if dir == "" {
			inv.setErr("failed", "no_plan_dir", "无法解析意图件落点（HOME 不可用且未给 --out）")
			fmt.Fprintf(stderr, "%s: 意图件落点解析不出来 ⇒ 不给结论\n", progName)
			return exitFail
		}
		out = filepath.Join(dir, env.Items[0].PlanID+".json")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		inv.setErr("failed", "plan_dir_create_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 意图件目录建不出来：%v\n", progName, err)
		return exitFail
	}
	if err := os.WriteFile(out, append(canon, '\n'), 0o644); err != nil {
		inv.setErr("failed", "plan_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 意图件写不进去：%v\n", progName, err)
		return exitFail
	}
	row := map[string]string{
		"plan_id": env.Items[0].PlanID, "plan_digest": digest,
		"action": actionID, "target_kind": kind, "target_name": name,
		"nodes": strings.Join(nodes, ","), "host": planHost(), "status": "proposed",
		"path": out, "layers_passed": "L1_schema,L2_reference,L3_dry_run",
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, planFields, row)
	}
	fmt.Fprintf(stdout, "%s\n", env.Items[0].PlanID)
	fmt.Fprintf(stdout, "意图件落点: %s\n", out)
	fmt.Fprintf(stdout, "零副作用口径：本命令**只读现状 + 写这一件** —— 不触任何写面、不改任何目标状态（§十八.3-2 判据⑤）\n")
	fmt.Fprintf(stdout, "下一步：zerg apply %s --confirm=%s\n", out, name)
	return exitOK
}

// refExists 判一个 `前缀:编号` 回指得上既有编号（**精确相等**，不模糊）。
func refExists(ref string, ids []string) bool {
	_, tail, ok := strings.Cut(ref, ":")
	if !ok || strings.TrimSpace(tail) == "" {
		return false
	}
	return contract.Has(ids, strings.TrimSpace(tail))
}

// ── ③ `zerg apply` ─────────────────────────────────────────────────────────────

// applyFields —— `--json` 面的全部字段。
var applyFields = []string{"plan_id", "action", "target_name", "layers_passed", "failed_layer", "detail"}

// cmdApply —— `zerg apply <件> [--confirm=<目标>] [--json 字段]`
//
// **只吃那一份件**（不重新规划）· 四层按序判（L1 schema → L2 引用存在性 → L3 干跑 → L4 人在环）。
// 任一层不过 ⇒ **后续层不跑**（§十八.3-3 判据③），且逐层点名「跑到哪、卡在哪」。
func cmdApply(inv *invocation, stdout, stderr io.Writer) int {
	ip, ok := intentSpec(stderr)
	if !ok {
		return exitUsage
	}
	passed := []string{}
	fail := func(layer, kind, detail, msg string) int {
		inv.setErr(kind, detail, msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "已过的层：%v · **卡在 %s** ⇒ 后续层不跑（§十八.3-3 判据③）\n", passed, layer)
		fmt.Fprintf(stderr, "error.kind=%s · detail=%s · retryable=false\n", kind, detail)
		if inv.jsonGiven {
			row := map[string]string{"layers_passed": strings.Join(passed, ","), "failed_layer": layer, "detail": detail}
			// K2 归一后仍是**正向**用法（与 `requireFields` 同一口径）：给了字段才出 JSON 面；
			// 没给 ⇒ 本函数已把字段清单写到 stderr，落到下面 `return exitUsage`（**码取自表**）。
			// ★ 别把它当「!requireFields」那 36 个负向调用点之一 —— 误替换会把「给了字段」当成「没给」。
			if rc := requireFields(inv, stderr); rc == exitOK {
				return selectJSON(stdout, stderr, inv, inv.path, applyFields, row)
			}
		}
		return exitUsage
	}

	path := ""
	if len(inv.args) > 0 {
		path = strings.TrimSpace(inv.args[0])
	}
	if path == "" {
		return fail("L1_schema", "usage", "missing_plan_file", "缺意图件路径（用法：zerg apply <件> --confirm=<目标>）")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		// 件读不到 ⇒ **不给结论**（2）—— 不是「件不合法」，是判据没有对象。
		return fail("L1_schema", "usage", "plan_unreadable", fmt.Sprintf("意图件读不出来：%v（件读不到 ⇒ 不给结论，不当成「件不合法」）", err))
	}

	// ── L1 schema：包封六键 + 四闭集 + 七语义件齐 + 摘要自洽 + **规范字节对拍**（改一字节即现形）
	var env planEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fail("L1_schema", "usage", "plan_not_json", fmt.Sprintf("意图件不是合法 JSON：%v", err))
	}
	if env.Schema != ip.EnvelopeSchema || !contract.Has(ip.EnvelopeKinds, env.Kind) || len(env.Items) != 1 {
		return fail("L1_schema", "usage", "plan_envelope_bad",
			fmt.Sprintf("包封不合规（schema=%q kind=%q items=%d；要 schema=%q kind∈%v items=1）",
				env.Schema, env.Kind, len(env.Items), ip.EnvelopeSchema, ip.EnvelopeKinds))
	}
	d := env.Items[0]
	if !contract.Has(ip.TargetKinds, d.Target.Kind) {
		return fail("L1_schema", "usage", "plan_target_kind_bad", fmt.Sprintf("target.kind %q 不在闭集 %v", d.Target.Kind, ip.TargetKinds))
	}
	if !contract.Has(ip.Actions, d.Action) {
		return fail("L1_schema", "usage", "plan_action_bad", fmt.Sprintf("action %q 不在闭集 %v", d.Action, ip.Actions))
	}
	if !contract.Has(ip.Statuses, d.Status) {
		return fail("L1_schema", "usage", "plan_status_bad", fmt.Sprintf("status %q 不在闭集 %v", d.Status, ip.Statuses))
	}
	if d.Target.Name == "" {
		return fail("L1_schema", "usage", "plan_target_missing", "target.name 是空的（`--confirm` 要比的就是它）")
	}
	// 摘要自洽（§十八.3-3：`plan_digest` 不符 ⇒ 拒）
	if want := planDigestOf(env); want != d.PlanDigest {
		return fail("L1_schema", "usage", "plan_digest_mismatch",
			fmt.Sprintf("plan_digest 对不上（件里 %s · 复算 %s）—— 件被改过就拒", short(d.PlanDigest), short(want)))
	}
	// 规范字节对拍：**改一字节**（哪怕只多一个空格、换行）也现形 —— 件是「批的载体」，不许手改。
	if !bytes.Equal(bytes.TrimRight(raw, "\n"), planCanonical(env)) {
		return fail("L1_schema", "usage", "plan_not_canonical",
			"件不是产出时的规范字节（被手改过：哪怕一个空格/换行）⇒ 拒（§十八.3-3：件被改一字节 ⇒ 2）")
	}
	passed = append(passed, ip.Layers[0]) // L1_schema

	// ── L2 引用存在性：依据 / 退点必须**回指既有编号**（回指不上 ⇒ 2；不新立码）
	ids, derr := contract.DevTargets()
	if derr != nil || len(ids) == 0 {
		return fail("L2_reference", "failed", "empty_target_ledger", "编号闭集读不出来 ⇒ 不给结论")
	}
	if !refExists(d.Evidence.Ref, ids) {
		return fail("L2_reference", "usage", "plan_ref_unresolvable",
			fmt.Sprintf("依据 %q 回指不上既有编号（既有编号 %d 条）", d.Evidence.Ref, len(ids)))
	}
	if strings.TrimSpace(d.Rollback.Ref) == "" {
		return fail("L2_reference", "usage", "plan_rollback_missing", "缺退点（没退点的件不许施加）")
	}
	if strings.TrimSpace(d.Evidence.Source) == "" {
		return fail("L2_reference", "usage", "plan_evidence_missing", "缺依据来源（source 空）")
	}
	passed = append(passed, ip.Layers[1]) // L2_reference

	// ── L3 干跑：件必须是**干跑产出的**，且指纹（`RC12` 的 `plan_id`）自洽；顺带判「换机 / 旧值变」
	if !d.Approval.DryRun {
		return fail("L3_dry_run", "usage", "plan_not_dry_run",
			"这件不是干跑产出的（approval.dry_run=false）—— `apply` 只吃干跑件（§十八.3-2：算与做分开）")
	}
	if d.PlanID != planIDOf(d.PlanDigest) {
		return fail("L3_dry_run", "usage", "plan_id_mismatch",
			fmt.Sprintf("plan_id 与摘要不自洽（件里 %s · 复算 %s）—— 干跑集指纹对不上即拒（§十五 RC12）",
				d.PlanID, planIDOf(d.PlanDigest)))
	}
	if d.Scope.Host != planHost() {
		return fail("L3_dry_run", "usage", "plan_host_mismatch",
			fmt.Sprintf("**换机**了：件是在 %q 上算的，本机是 %q ⇒ 拒（旧值/作用域都要重算）",
				d.Scope.Host, planHost()))
	}
	// 旧值变（`--expect` 记在件里）：主控可达就现场对一次；不可达**不吞**、也不当「没变」。
	if before := d.Expected.Before; before != "" {
		c := newClient()
		var resp jsonObj
		if rc := fetchInv(nil, c, "/api/fleet/models", &resp, stderr); rc == exitOK {
			live := ""
			for _, it := range asList(resp["models"]) {
				if o := asObj(it); o != nil {
					if id, _ := o["id"].(string); id == d.Target.Name {
						live = cell(o["backend"])
					}
				}
			}
			if live != "" && live != before {
				return fail("L3_dry_run", "conflict", "plan_superseded",
					fmt.Sprintf("旧值变了（件里 before=%q · 现场 %q）⇒ 拒（status 该是 superseded）", before, live))
			}
		}
	}
	passed = append(passed, ip.Layers[2]) // L3_dry_run

	// ── L4 人在环：`--confirm` 必须**与件里的目标逐字相同**（§4.1 K7 一个口径）
	if !inv.confirmGiven || inv.confirm != d.Target.Name {
		return fail("L4_human", "usage", "confirm_required",
			fmt.Sprintf("L4 人在环：缺确认（`--confirm=<%s>`；给的是 %q）—— `--confirm` 的值必须与**件里的**目标逐字相同",
				d.Target.Name, inv.confirm))
	}
	passed = append(passed, ip.Layers[3]) // L4_human

	// 四层全过 —— 但**写面本版未开放**（§6.2 零写操作），照 `cmdGuarded` 同一档：不给结论。
	inv.setErr("usage", "not_opened",
		fmt.Sprintf("`apply %s` 四层全过 ✓，但写面在**本版未开放**（§6.2 零写操作）⇒ 不给结论", d.Target.Name))
	fmt.Fprintf(stderr, "%s: 四层全过 ✓（%v）；但 `apply` 在**本版未开放** —— 批 A 起全程零写操作（§6.2）⇒ 不给结论\n",
		progName, passed)
	fmt.Fprintf(stderr, "排期：等人批通道（M18 C4 ② 段）真通；现在只有 `plan`（算）与 `--dry-run`（看）\n")
	fmt.Fprintf(stderr, "error.kind=usage · detail=not_opened · retryable=false\n")
	if inv.jsonGiven {
		row := map[string]string{"plan_id": d.PlanID, "action": d.Action, "target_name": d.Target.Name,
			"layers_passed": strings.Join(passed, ","), "failed_layer": "", "detail": "not_opened"}
		// K2 归一后仍是**正向**用法（同上 `apply` 那一处）：给了字段才出 JSON 面；没给 ⇒ 落到
		// 下面 `return exitUsage`（**码取自表**）。
		if rc := requireFields(inv, stderr); rc == exitOK {
			return selectJSON(stdout, stderr, inv, inv.path, applyFields, row)
		}
	}
	return exitUsage
}

// ── 小工具 ────────────────────────────────────────────────────────────────────

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "…"
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func atofSafe(s string) float64 {
	f := 0.0
	seenDot := false
	div := 1.0
	for _, r := range strings.TrimSpace(s) {
		if r == '.' && !seenDot {
			seenDot = true
			continue
		}
		if r < '0' || r > '9' {
			return 0
		}
		if seenDot {
			div *= 10
			f += float64(r-'0') / div
		} else {
			f = f*10 + float64(r-'0')
		}
	}
	return f
}

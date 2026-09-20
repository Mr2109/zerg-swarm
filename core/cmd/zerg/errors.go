// errors.go —— 错误分类与自愈（§九 M7 · 调研-M7 §4.1–§4.3）。
//
// 三条已定条文（本文件是它们的落点）：
//
//	· `E1` **退出码是 `error.kind` 的单值投影** —— `kind → code` 的映射表是**唯一真源**
//	  （就是下面这张表）；**不许**各处各判。
//	· `E7` **唯一判据点**：`retryable` **只由 `kind` 决定**（表在代码里，与 `zerg help errors`
//	  同源）；**退出码不许被用来判重试**。
//	· `E5` **适配器只做投影**：别处只许把 kind 翻译成对方方言，不许发明新分类。
//
// 字段形状（调研-M7 §4.2）：`kind` 必填、稳定标识符 `[a-z][a-z0-9_]*`（**lower_snake_case** ·
// 大写即门禁红 —— §二十一 的 `kind` 闭集判据）；`detail` 二级细分；`retryable` 必填 bool；
// `remedy` 告诉调用方「下一步做什么」，是**自愈**的入口。
package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// errorKindRow —— kind 闭集的一行。**只增不改**：改名 = 破坏性变更（走大版本 · §九 M6）。
type errorKindRow struct {
	Kind      string
	Code      int    // 退出码（E1：单值投影）
	Retryable bool   // E7：只由 kind 决定
	Remedy    string // 自愈入口
	Meaning   string
	Source    string // 定稿出处
}

// errorKinds —— kind 闭集（§九 M7 · 调研-M7 §4.2 的 B 表 · §十二 `P-030`–`P-032` 落判）。
//
// 逐条与定稿对齐（`P-030`：`declaration_rejected` = `1`；`P-031`：`conflict` **幂等优先**、
// 真冲突才非零；`P-013` ③：`unreachable` = `12`；`P-013` ②：`conflict` = `14`（**不是** `507`）；
// §十二 `P-129`：回滚成功 = `1` + `kind=ROLLED_BACK` 那一族）。
var errorKinds = []errorKindRow{
	{"failed", 1, false, "read_logs", "一般失败（跑到了、没成功）", "§九 M7 · §4.1 K3"},
	{"declaration_rejected", 1, false, "fix_declaration", "声明认不得（跑到了、被拒了 ⇒ 不是用法错）", "§十二 P-030（= 1）"},
	{"conflict", 14, false, "wait_or_reload", "冲突 / 被占（幂等优先：已在该状态按状态报）", "§十二 P-031 · P-013 ②"},
	{"insufficient_resource", 10, false, "unload_occupant", "资源不足（同参必败；卸掉占用者后可重试）", "§十二 P-013 · 调研-M7 §4.2"},
	{"timeout", 11, true, "retry_later", "超时（等到了坏结果与「等不起」分开）", "§十二 P-013 · §九 M8"},
	{"unreachable", 12, true, "check_link", "不可达（打不到主控）—— fail-closed，不偷偷换端点", "§十二 P-013 ③"},
	{"unauthenticated", 4, false, "fetch_token", "未认证（缺令牌 / 令牌不对）", "§九 M2 C7"},
	{"forbidden", 4, false, "request_access", "权限不够（`403`；`P-013` ④ 与 `4` 并码、kind 分家）", "§十二 P-013 ④"},
	{"blocked", 8, true, "fix_precondition", "不给结论（缺前置 / 不可判）——「读不到」不许当健康", "§九 M9 · 门禁 BLOCKED"},
	{"upstream_error", 1, true, "retry_after_warmup", "上游错（带 `upstream_status` 原码）", "§九 M7 · §十五.5"},
	{"circuit_open", 1, true, "wait_or_switch_node", "熔断开着（等 / 换机）", "§十五.5"},
	{"engine_error", 1, false, "switch_engine", "引擎错（同参必败）", "§十五.5"},
	{"unsupported_on_node", 2, false, "use_other_node", "该机上不支持这个动作（对象级目标错）", "§九 M13 `Z7`"},
	{"usage", 2, false, "fix_usage", "用法错（旗标 / 字段 / 目标不对）", "§4.1 K3"},
	{"interrupted", 130, false, "resume_or_rerun", "人打断（Ctrl-C · **默认只退订、不取消**）", "§十二 P-033"},
	{"rolled_back", 1, false, "inspect_receipt", "已回滚（`P-129` 的唯一说法：`1` + 本 kind）", "§十二 P-129"},
}

// errorKindOf 查 kind 那一行（闭集外 ⇒ nil）。
func errorKindOf(kind string) *errorKindRow {
	for i := range errorKinds {
		if errorKinds[i].Kind == kind {
			return &errorKinds[i]
		}
	}
	return nil
}

// codeOfKind —— E1：kind → 退码（**唯一真源**）。
func codeOfKind(kind string) int {
	if r := errorKindOf(kind); r != nil {
		return r.Code
	}
	return exitFail
}

// retryableOf —— E7：可重试性**只由 kind 决定**（不看退码）。
func retryableOf(kind string) bool {
	if r := errorKindOf(kind); r != nil {
		return r.Retryable
	}
	return false
}

// remedyOf —— 自愈入口（下一步做什么），kind 闭集外给一个中性值。
func remedyOf(kind string) string {
	if r := errorKindOf(kind); r != nil {
		return r.Remedy
	}
	return "read_logs"
}

// kindForExitCode —— 退码回落到 kind（**只用在**「没人报过 kind」的兜底一处；
// 反向映射不构成第二真源：它只认主表五档，多值码一律不猜）。
func kindForExitCode(rc int) string {
	switch rc {
	case exitOK:
		return ""
	case exitFail:
		return "failed"
	case exitUsage:
		return "usage"
	case exitAuth:
		return "unauthenticated"
	case exitBlocked:
		return "blocked"
	case exitConflict:
		return "conflict"
	case exitUnreachable:
		return "unreachable"
	case exitTimeout:
		return "timeout"
	case exitResource:
		return "insufficient_resource"
	case exitInterrupted:
		return "interrupted"
	default:
		return "failed"
	}
}

// cliError —— 一次调用里被报出来的那个错（给机器面用 · 人面照旧走 stderr）。
type cliError struct {
	Kind    string
	Detail  string
	Message string
	Where   string // local / core / agent / node:<名>
	Extra   map[string]string
}

// setErr 记下本次调用的错（谁先报谁为准 —— 与「不打第二枪」同一条纪律）。
func (inv *invocation) setErr(kind, detail, msg string) {
	if inv.err == nil {
		inv.err = &cliError{Kind: kind, Detail: detail, Message: msg, Where: "local"}
	}
}

// errJSON 渲染 `error` 块（§九 M7：kind / detail / retryable / remedy / message / where）。
func (e *cliError) errJSON() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("{\"kind\":" + jstr(e.Kind))
	if e.Detail != "" {
		b.WriteString(",\"detail\":" + jstr(e.Detail))
	}
	fmt.Fprintf(&b, ",\"retryable\":%t", retryableOf(e.Kind))
	b.WriteString(",\"remedy\":" + jstr(remedyOf(e.Kind)))
	b.WriteString(",\"exit_code\":" + fmt.Sprintf("%d", codeOfKind(e.Kind)))
	b.WriteString(",\"message\":" + jstr(e.Message))
	b.WriteString(",\"where\":" + jstr(orLocal(e.Where)))
	if len(e.Extra) > 0 {
		keys := make([]string, 0, len(e.Extra))
		for k := range e.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(",\"extra\":{")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(jstr(k) + ":" + jstr(e.Extra[k]))
		}
		b.WriteString("}")
	}
	b.WriteString("}")
	return b.String()
}

func orLocal(s string) string {
	if s == "" {
		return "local"
	}
	return s
}

// helpErrors —— `zerg help errors`：kind 闭集的自描述面（与代码同源 · E7）。
func helpErrors() string {
	var b strings.Builder
	b.WriteString("错误分类与自愈（§九 M7 · `zerg help errors` 就是 kind 闭集的自描述面）\n\n")
	b.WriteString("两条硬规矩：\n")
	b.WriteString("  · **E1 退出码是 kind 的单值投影**：这张表是唯一真源，别处不许各判。\n")
	b.WriteString("  · **E7 retryable 只由 kind 决定**：**退出码不许被用来判重试**（同码不同 kind ⇒ 可重试性不同）。\n\n")
	b.WriteString("kind 闭集（只增不改；改名 = 破坏性变更，走大版本 —— §九 M6 同款）：\n")
	w := 0
	for _, r := range errorKinds {
		if len(r.Kind) > w {
			w = len(r.Kind)
		}
	}
	for _, r := range errorKinds {
		fmt.Fprintf(&b, "  %s  %-3d  retryable=%-5t  remedy=%-20s %s  [%s]\n",
			pad(r.Kind, w), r.Code, r.Retryable, r.Remedy, r.Meaning, r.Source)
	}
	b.WriteString("\n命名纪律（§二十一 的 `kind` 闭集判据）：kind 一律 **lower_snake_case**\n")
	b.WriteString("（`[a-z][a-z0-9_]*`）—— 写成大写（如 `CONFIRM_REQUIRED`）**门禁红**\n")
	b.WriteString("（`scripts/gates/check-error-kinds.py` · 进 gates scope）。\n\n")
	b.WriteString("机器面形状（`--json` 失败时 `error` 块与包封同出）：\n")
	b.WriteString("  {\"schema\":\"zerg/v1\",\"kind\":\"…\",\"items\":[],\"meta\":{…},\"warnings\":[],\"truncated\":false,\n")
	b.WriteString("   \"error\":{\"kind\":\"conflict\",\"detail\":\"…\",\"retryable\":false,\"remedy\":\"wait_or_reload\",\n")
	b.WriteString("            \"exit_code\":14,\"message\":\"…\",\"where\":\"local\"}}\n\n")
	b.WriteString("口径（防误读）：`error.retryable` 与 `error.exit_code` 都由 kind 派生 ⇒ 两者**不独立**；\n")
	b.WriteString("调用方判「要不要重试」只读 `kind` / `retryable`，**不许**拿退出码反推。\n")
	return b.String()
}

// errEnvelope 渲染带 `error` 块的包封（`items` 恒为空数组 —— 失败时没有结果面 · §九 M6 I3）。
func errEnvelope(cmd *command, e *cliError) string {
	kind := ""
	if cmd != nil {
		kind = cmd.kind
	}
	if kind == "" {
		kind = "Error"
	}
	src := "local（本机）"
	if cmd != nil && cmd.endpoint != "" {
		src = cmd.endpoint
	}
	out := "{\"schema\":" + jstr(contractID) + ",\"kind\":" + jstr(kind) + ",\"items\":[]," +
		"\"meta\":{\"count\":0,\"source\":" + jstr(src) + "},\"warnings\":[],\"truncated\":false"
	if ej := e.errJSON(); ej != "" {
		out += ",\"error\":" + ej
	}
	return out + "}\n"
}

// emitErrEnvelope 把错误块写进 stdout（**只有** `--json <字段>` 的失败路径会走到这里）。
func emitErrEnvelope(stdout io.Writer, cmd *command, e *cliError) {
	fmt.Fprint(stdout, errEnvelope(cmd, e))
}

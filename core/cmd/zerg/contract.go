// contract.go —— 输出契约（§九 M6）：三层版本 · 包封 · `--json` 字段稳定性承诺。
//
// 出处（不新造语义）：
//
//	· §九 M6 结论：契约分三层版本（契约号 `zerg/v1` · 对象 schema 版本 · 组件版本），
//	  `zerg version --json` 三个都报；`--json` 统一外层包封；字段**只增不改**，
//	  破坏性变更走大版本；契约**覆盖机器面、不覆盖人类面**。
//	· 调研-M6 §4.1 三层版本表（`调研-M6-输出契约-20260920.md:220-228`）。
//	· §十二 `P-023`（契约号形态 = `zerg/v1` 字符串 id）· `P-025`（未知字段报错）·
//	  `P-026`（不认的 schema 主号 ⇒ 拒绝）· `P-029`（`machine` vs `node` 系字段名）。
//	· §十五.7：包封第一键一律 `schema:"zerg/v1"`，**不写** `schema_version`。
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// versionString 取组件版本（编译期注入 · §九 M15 T3：源码零版本字面量）。
func versionString() string { return version.Version }

// 契约号（§十二 P-023 定案：字符串 id，能表达「谁的契约」）。
const (
	contractID     = "zerg/v1"
	contractMajor  = 1
	contractMinor  = 0
	objectSchemaID = 1 // 对象 schema 主号（契约 v1 下的对象面；`eggs[].schema_version` 一族的口径）
)

// recognizedMajors 契约认的主号闭集（消费侧拒绝不认的主号 · §十二 P-026）。
func recognizedMajors() []int { return []int{contractMajor} }

// parseContractID 拆 `zerg/v<主>[.<次>]`。不认的形状 ⇒ ok=false（调用方给 2）。
func parseContractID(s string) (major int, minor int, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "zerg/v") {
		return 0, 0, false
	}
	rest := strings.TrimPrefix(s, "zerg/v")
	parts := strings.SplitN(rest, ".", 2)
	m, err := parseIntStrict(parts[0])
	if err != nil {
		return 0, 0, false
	}
	min := 0
	if len(parts) == 2 {
		min, err = parseIntStrict(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	return m, min, true
}

func parseIntStrict(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("空")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("非数字")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// contractVersionText 三层版本的自描述（`zerg help contract` 与 `zerg version` 共用一处）。
func contractVersionText() string {
	var b strings.Builder
	b.WriteString("三层版本（§九 M6 · 调研-M6 §4.1 —— 三层不许混成一层）：\n\n")
	fmt.Fprintf(&b, "  ① 契约号（本稿新增）    %-10s  回答「这份 `--json` 输出的**形状契约**是第几版」\n", contractID)
	fmt.Fprintf(&b, "  ② 对象 schema 版本      %-10d  回答「这枚卵/这份档案的**数据**是哪版」（`eggs[].schema_version` 一族）\n", objectSchemaID)
	fmt.Fprintf(&b, "  ③ 组件版本（semver）    %-10s  回答「哪台机跑的哪个二进制」（`/api/capabilities` 的 version/code_sha）\n", versionString())
	b.WriteString("\n硬规则：`zerg version --json` **三个都报**（契约号 + 对象 schema 版本 + 组件版本；主控版本与它的\n")
	b.WriteString("`code_sha` 与组件面并列，只从 `/api/capabilities` 取 —— §九 M15 T1）。\n")
	b.WriteString("包封第一键一律 `schema:\"zerg/v1\"`，**不写** `schema_version`（§十五.7 定案）。\n")
	return b.String()
}

// helpContract 渲染 `zerg help contract`：三层版本 + 字段稳定性承诺 + 拒绝规则。
func helpContract() string {
	var b strings.Builder
	b.WriteString("输出契约（§九 M6 · 调研-M6 §4.2/§4.4）\n\n")
	b.WriteString(contractVersionText())
	b.WriteString("\n统一外层包封（六键 + 一可选键；`--json` 的**唯一**出口 · §6.3 批 A T-06 已落地）：\n")
	b.WriteString("  schema     恒 \"zerg/v1\"（字符串 id · §十五.7）\n")
	b.WriteString("  kind       与命令一一对应、单数 CamelCase（I2）\n")
	b.WriteString("  items      恒数组、空为 []、**永不为 null**（I3）\n")
	b.WriteString("  meta       {count, source}；多机目标进 meta.node（§九 M13）\n")
	b.WriteString("  warnings   数组（空为 []）—— 降级/未证实一律在这里**显式**报，不静默\n")
	b.WriteString("  truncated  bool\n")
	b.WriteString("  __error__  出错时才在（§九 M7 的 error 块 · `zerg help errors`）\n")
	b.WriteString("\n六键的真值口径（本批落地；顶层键**一个不多一个不少** ✗ —— 变的是值，不是键）：\n")
	b.WriteString("  · warnings[]  只有**命令当场判定出的真事**才非空；无事 ⇒ 逐字 `[]`\n")
	b.WriteString("                进这一格的四类真事（逐条 · 不在表里的事不许写进来）：\n")
	b.WriteString("                  ① 预算裁条：真裁了条目 ⇒ 至少一条「已裁 N 条」（与 `truncated=true` 同批）\n")
	b.WriteString("                  ② 层/面没跑：状态落在「没跑」闭集（未跑 / 未建索引 / 未装 / 读不到）⇒ 逐层点名\n")
	b.WriteString("                  ③ 取不到真源：该取数的那一面拿不到（不是「没有」）⇒ 明说怎么拉\n")
	b.WriteString("                  ④ 降级放行：超预算降档 / 钩子判决必拦 一类**改变了本跑覆盖面**的事\n")
	b.WriteString("                拿不到真值 ⇒ **保持 `[]`**：不许用 warnings 装「我以为」✗（缺口留白，不装）\n")
	b.WriteString("  · truncated   只有**真裁了条目**（预算/上限的结果）才 `true`；没裁一律 `false`\n")
	b.WriteString("                上限存在 ≠ 裁了（余量不是「裁了」）⇒ 只看真裁掉的条数\n")
	b.WriteString("  · meta        子键**按需扩**（`layers_not_run[]` / `how_to_restore` / `dry_run` / `impact_digest` …），\n")
	b.WriteString("                缺席 = 这一趟没有这一格（**不补空值**）；五个旧子键 `count`/`source`/`changed`/\n")
	b.WriteString("                `node`/`idempotency_key` 的名字、类型、取法**永不变** ✗（同名子键不许被覆盖）\n")
	b.WriteString("\n字段稳定性承诺（本契约对消费方的**唯一**承诺面）：\n")
	b.WriteString("  ① **只增不改**：既有字段的名字、类型、语义**永不变**；新字段只许追加（I6：消费侧必须忽略未识别键）。\n")
	b.WriteString("  ② **不承诺字段序**（I7）：对象内键序由实现决定；**对象序**只由显式排序旗标决定。\n")
	b.WriteString("  ③ **破坏性变更走大版本**：删字段 / 改类型 / 改语义 ⇒ 契约号主号 +1（`zerg/v2`），旧主号继续可读。\n")
	b.WriteString("  ④ **废弃四步**：标注 deprecated（进 warnings）⇒ 文档删示例 ⇒ 导出处删字段 ⇒ 下一个大版本删字段。\n")
	b.WriteString("  ⑤ **契约覆盖机器面、不覆盖人类面**：人面（表格/装饰）**不在**承诺面内（§九 M14 `T1` 另有对齐要求）。\n")
	b.WriteString("\n拒绝规则：\n")
	b.WriteString("  · 请求未知字段 ⇒ `exit 2` + 列全部合法字段（I5 · `P-025`）\n")
	b.WriteString(fmt.Sprintf("  · `--json` 不给字段 ⇒ `exit %d` + stderr 列全部字段（K2 · 「schema 自描述」）\n",
		exitCodeOf("usage")))
	b.WriteString("  · `--schema <id>` 的主号不认 ⇒ `exit 2`（`P-026` · 本版只认 ")
	fmt.Fprintf(&b, "%v", recognizedMajors())
	b.WriteString("）\n")
	b.WriteString("  · `--json '*'` **不存在** —— 本契约要的是逗号分隔的字段名（K1）\n")
	b.WriteString("\n复跑判据（本件的证据面）：\n")
	b.WriteString("  zerg version --json name,version,contract,object_schema\n")
	b.WriteString("  zerg task ls --json id --schema zerg/v2     # ⇒ exit 2\n")
	b.WriteString("  git grep -l \"response_format\" | wc -l        # 受约束解码的落点（§二十一 第 11 条）\n")
	return b.String()
}

// ---- 兼容窗口（§九 M15 · §十二 `P-079`：三类分开给，major 不同一律拒）----

// compatWindowRow —— 窗口表一行。**三类各给一格**，不许合成「±N」一句话。
type compatWindowRow struct {
	Pair     string // 哪两件之间
	Window   string // 窗口（逐对给）
	Refuse   string // 越窗怎么办
	Evidence string
}

var compatWindow = []compatWindowRow{
	{"命令面 ↔ 主控", "minor ±1", "写明不兼容并**拒**（不许静默降级）", "§十二 P-079 · 调研-M15 §4.4"},
	{"子端 ↔ 主控", "minor [−3, 0]（且**不得比主控新**）", "同上", "§十二 P-079"},
	{"UI / 茧壁 ↔ 主控", "0（完全同版）", "同上", "§十二 P-079"},
	{"任意两件 major 不同", "—（无窗口）", "**一律拒**", "§十二 P-079 收口"},
}

// windowSummary 一行摘要（进 `zerg version --json` 的 `window` 字段与帮助头部）。
func windowSummary() string {
	parts := make([]string, 0, len(compatWindow))
	for _, r := range compatWindow {
		parts = append(parts, r.Pair+" "+r.Window)
	}
	return strings.Join(parts, " · ")
}

// helpVersionTopic —— `zerg help version`：三层版本 + 兼容窗口 + 不静默降级。
func helpVersionTopic() string {
	var b strings.Builder
	b.WriteString("版本协商（§九 M15 · 调研-M15 §4.1–§4.4）\n\n")
	b.WriteString(contractVersionText())
	b.WriteString("\n兼容窗口（**三类分开给** · §十二 `P-079`；窗口本身是**机器可读资产** · `T5`）：\n")
	w := 0
	for _, r := range compatWindow {
		if len(r.Pair) > w {
			w = len(r.Pair)
		}
	}
	for _, r := range compatWindow {
		fmt.Fprintf(&b, "  %s  窗口 %-32s %s\n", pad(r.Pair, w), r.Window, r.Refuse)
	}
	b.WriteString("\n「不静默降级」的可执行定义（调研-M15 §4.3）：\n")
	b.WriteString("  · 不兼容时**必须**明确报错（`exit 2` / `kind=unsupported_on_node` 一类），**不许**\n")
	b.WriteString("    自动挑一个「差不多能用」的版本接着跑；\n")
	b.WriteString("  · 版本真源**只从 `/api/capabilities` 三键取**（`T1`）—— `/api/core/status` 的 `version`\n")
	b.WriteString("    过去是一枚硬编码的第二版本号（已红第 2 条），现在与 capabilities **同源同值**；\n")
	b.WriteString("  · 命令面自身版本 = **编译期注入**、源码零版本字面量（`T3`）；\n")
	b.WriteString("  · 同一进程只有一个版本号（`T4`）：`zerg version` 与 `zerg --version` 同一次构建同值。\n")
	return b.String()
}

// fieldStabilityNote 给导出物与帮助共用的一句承诺（防两处写法漂）。
func fieldStabilityNote() string {
	return "字段只增不改 · 破坏性变更走大版本（" + contractID + " → 下一个主号）"
}

// schemaMajorSet 排序后的主号清单文本（错误信息里列出来）。
func schemaMajorSet() string {
	ms := recognizedMajors()
	sort.Ints(ms)
	parts := make([]string, 0, len(ms))
	for _, m := range ms {
		parts = append(parts, fmt.Sprintf("zerg/v%d", m))
	}
	return strings.Join(parts, "、")
}

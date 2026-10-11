// topics.go —— 帮助主题的**唯一分派面**（`zerg help <主题>`）。
//
// 为什么把它从 `main.go` 的 switch 里搬出来：主题会一直加（批 B 一次加了 7 个），
// 每次加一个都要改主流程 ⇒ 两处写法迟早漂。这里用**一张表**当分派真源：
// 表里没有的主题 ⇒ 报「未知帮助主题」+ 列可用主题（表就是列清单的真源）。
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// helpTopic —— 一个帮助主题（名字 → 渲染函数）。
type helpTopic struct {
	Name  string
	Brief string
	Text  func() string
}

// helpTopics —— 主题表（**唯一真源**：分派、错误信息里的「可用主题」都从这里来）。
func helpTopics() []helpTopic {
	return []helpTopic{
		{"exit-codes", "退码表（唯一真源）", helpExitCodes},
		{"config", "五级配置优先级 + 凭据四条", helpConfig},
		{"dangerous", "危险动作清单（条数以 `zerg help dangerous` 现读为准 · 三态）", helpDangerous},
		{"contract", "输出契约：三层版本 + 包封 + 字段稳定性承诺", helpContract},
		{"errors", "error.kind 闭集 + 可重试性 + 自愈入口", helpErrors},
		{"idempotency", "幂等四字段（幂等档/重跑语义/生效语义/危险档）", helpIdempotency},
		{"locks", "并发与锁：三级粒度 + 三时钟 + slot 版本号", helpLocks},
		{"long-tasks", "长任务：三档旗标 + 默认档 + 句柄", helpLongTasks},
		{"remote", "远端语义：--context 与 --node 两轴 + 名册真源", helpRemote},
		{"version", "版本协商：三层版本 + 兼容窗口（三类分开）", helpVersionTopic},
		{"watch", "事件 / 订阅通道：单端点 + Accept 协商 + 保留窗口", helpWatch},
		{"credentials", "凭据：优先级链一条 + 令牌五不进 + 401/403 归一", helpCredentials},
		{"enclosure", "茧壁边界：layer 三档 + 分区表 + argv 硬判据", helpWall},
		{"human-machine", "人机双模：三态 + 两层承诺 + 分页/颜色/format 四条", helpHumanMachine},
		{"registry", "契约登记表：真源口径 + 六处先例 + 变更流程 V0–V7", helpRegistry},
		{"offline", "离线/降级：单端点 + 四条「不偷偷」+ 例外清单 F-1–F-5", helpOffline},
		{"plugins", "插件机制：`zerg-<名>` 约定 + 零注册表 + 信任声明（P14）", helpPlugins},
		{"skill", "技能库体检：只读正门（根可配 + 现读体检格 · 死指针判红归 `-48` 门）", helpSkill},
		{"flags-consumed", "旗标消费面：登记 ⟷ 消费 对账（谁真读 · 谁接受即忽略 · 只读）", helpFlagsConsumed},
	}
}

// topicNames 排好序的主题名（错误信息与自描述用）。
func topicNames() []string {
	names := make([]string, 0, len(helpTopics()))
	for _, t := range helpTopics() {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// renderHelpTopic 分派一个主题；找不到 ⇒ 退码 2 + 列全部可用主题（K14 第三件）。
func renderHelpTopic(name string, stdout, stderr io.Writer) int {
	for _, t := range helpTopics() {
		if t.Name == name {
			fmt.Fprint(stdout, t.Text())
			return exitOK
		}
	}
	fmt.Fprintf(stderr, "%s: 未知帮助主题 %q\n", progName, name)
	fmt.Fprintf(stderr, "可用主题: %s\n", strings.Join(topicNames(), " · "))
	fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
	return exitUsage
}

// helpSkill —— `zerg help skill`：**技能库体检**的只读正门（缺口 `GAP-20260926-49`）。
//
// 口径（写死在这里，别在第二处再判一次）：
//
//	① **只读** —— 本面一个字节都不写技能库（技能库的写权限只给人，与 `zerg doctor` 里那条
//	   主仓 CA 技能目录同口径）；本面也不起子进程、不联网。
//	② **根可配** —— `ZERG_SKILLS_ROOT`（空串视同未设）→ 回落 `<home>/.hermes/skills`
//	   （**真源同** `check-outrepo-dead-refs` 的 `codrDefaultRoot`，不另起第二份推导）。
//	③ 本面是**帮助主题** ⇒ 恒退 `0`；根不可达时**照实标「不判」**、✗ 不伪造读数、✗ 不当绿。
//	   **判红档**（根不可达 ⇒ `rc=2`、扫到 0 个 `.md` ⇒ `rc=2`）在门 `check-outrepo-dead-refs`
//	   （缺口 `-48`）—— 那才是「不给结论」的那一档。
func helpSkill() string {
	var b strings.Builder
	root := codrDefaultRoot()
	b.WriteString("技能库体检（**只读** · 缺口 `GAP-20260926-49` · 正门 = `zerg help skill`）\n\n")
	fmt.Fprintf(&b, "根：%s\n", root)
	b.WriteString("  · 可配 = `ZERG_SKILLS_ROOT`（空串视同未设）→ 回落 `<home>/.hermes/skills`（**真源同** `check-outrepo-dead-refs` 的默认根，不另起第二份推导）。\n")
	b.WriteString("  · 这棵是**本机助手技能库**（`~/.hermes/skills`）；与 `zerg doctor` 里那条**主仓** CA 技能目录（`core/internal/agent/skills`）是**两套**，别混。\n")
	b.WriteString("  · **只读**：技能库不许由命令面写（写权限只给人，同主仓那套口径）。\n\n")
	b.WriteString("体检格（现读 · 只读）：\n")
	nTop, nSkill, nMd, ok := skillDoctorCounts(root)
	if !ok {
		b.WriteString("  根不可达 ⇒ **不判、不给结论**（✗ 不静默当绿）\n")
		b.WriteString("  —— 判红档在 `zerg gate check check-outrepo-dead-refs --report-only`（根不可达 ⇒ 退 2）\n")
	} else {
		fmt.Fprintf(&b, "  顶层条目   %d\n", nTop)
		fmt.Fprintf(&b, "  SKILL.md   %d\n", nSkill)
		fmt.Fprintf(&b, "  .md        %d\n", nMd)
		b.WriteString("\n死指针（`references/…` 指向不存在的件）= 门 `check-outrepo-dead-refs` 的判红档：\n")
		b.WriteString("  `zerg gate check check-outrepo-dead-refs --report-only`（只报告 · 根不可达 ⇒ 退 2 · 不给结论）\n")
	}
	b.WriteString("本面是**帮助主题** ⇒ 恒退 0；根不可达时**照实标「不判」**、✗ 不伪造读数、✗ 不当绿。\n")
	return b.String()
}

// skillDoctorCounts —— 技能库体检的**现读只读**计数：顶层条目 / `SKILL.md` / `.md` 各多少。
// 走 `filepath.WalkDir`（✗ 不起子进程）。根不可达（不是目录）⇒ `ok=false`（✗ 不伪造 0）。
func skillDoctorCounts(root string) (nTop, nSkill, nMd int, ok bool) {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return 0, 0, 0, false
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, 0, false
	}
	nTop = len(ents)
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".md") {
			nMd++
		}
		if name == "SKILL.md" {
			nSkill++
		}
		return nil
	})
	return nTop, nSkill, nMd, true
}

// fcDispatchConsumedFlags —— 「派发层替全体消费」的旗标（**唯一真源**：`zerg help flags-consumed`
// 的正文与它的**对账牙**（`flags_consumed_face_test.go`）都从这里取；分家 = 有人把这份名单抄成第二份）。
var fcDispatchConsumedFlags = []string{"--json", "--dry-run", "--yes", "--confirm", "--help"}

// helpFlagsConsumed —— `zerg help flags-consumed`：把「登记面 × 消费面」的对账口径落成一处可查的
// 自描述面（缺口 `GAP-20260928-37` · 待拍项 #20 的推荐形态 · 照 `zerg help dangerous` /
// `zerg help skill` 同形 —— 新加一个**只读帮助主题**）。
//
// 口径（写死在这里，别在第二处再判一次）：
//
//	① **纯只读** —— 本面一个字节都不写、不起子进程、不联网；是本主题 ⇒ **恒退 0**。
//	② **不换退码 · 不换行为 · 不阻断** —— 本面只把**已签字**的解析器口径说清楚，不改任何命令的收/读行为。
//	③ **口径真源 = 解析器**（`main.go` 的 `parseInvocation` 与 `invocation` 各全局旗标字段的注释逐字
//	   「全局布尔、谁用谁读 —— 不用的命令静默忽略」）；本面**只复述**，✗ 不另立第二份口径。
func helpFlagsConsumed() string {
	var b strings.Builder
	b.WriteString("旗标消费面（登记面 ⟷ 消费面 对账 · 缺口 `GAP-20260928-37` · 口径真源 = 解析器 `main.go` 的 `parseInvocation`）\n\n")
	b.WriteString("口径：解析器层登记了一批**全体命令都收下**的旗标名，但**只有用得着的命令才读它** ——\n")
	b.WriteString("     用不着的命令**接受即忽略**：不报错、不生效、退码 `0`，stdout 与不带该旗标时**逐字节相同**。\n")
	b.WriteString("     这是**签过字的口径**（`main.go` 全局旗标注释逐字「全局布尔、谁用谁读 —— 不用的命令静默忽略」），\n")
	b.WriteString("     **不是 bug**；本面的职责只是让它**看得见**。\n\n")
	b.WriteString("派发层替全体消费的只有：\n")
	for _, f := range fcDispatchConsumedFlags {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	b.WriteString("其余登记旗标**谁用谁读**（例：`--machine` —— `port ls` 收下但**不读**它 ⇒ 查的仍是本机，读数与不带时逐字节相同）。\n")
	b.WriteString("未读到者＝**接受即忽略**（退码 `0`）。要确认某条命令**真读**了哪些旗标，看 `zerg help dangerous`\n")
	b.WriteString("（危险档三态**逐条**标 `--confirm`）与 `zerg help <命令>` 的用法串。\n")
	b.WriteString("本面是**帮助主题** ⇒ 恒退 0；纯只读，不换任何命令的退码/行为。\n")
	return b.String()
}

// chat_tool_zergcmd.go — GAP-20261017-09 对话工具面收窄 ＋ GAP-20261017-11 命令树导航
//
// 用户口径（Mr2109，已入账）：「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」
// ⇒ 对话可调工具**必须是 zerg 子命令**；bash/read/write/edit/glob/grep/ls 一律不得作为对话工具。
//
// 实测铁证：会话 chat_20261017_144221_647a5a —— 模型原话「The fs run whitelist doesn't allow ls.
// … But I have bash tool directly.」⇒ 它**知道**白名单拒 ls，却转手用 bash 工具；rounds=3
// exit_kind=natural、任务未完成 ⇒ 光靠提示词要求不行，必须**从工具面撤掉**＋**执行面硬闸**。
//
// 本件三件事：
//
//	① 对话面禁用集（chatForbiddenTools：硬禁；chatPromptExcluded：仅主提示不出）
//	② zerg 工具定义（唯一参数 command）
//	③ zerg 命令硬闸＋执行器（argv 直传，✗ 不经 shell）
//
// GAP-20261017-11（命令树导航）另加两处：
//
//	④ 工具描述里的命令树导航：族清单摘要（zergCmdFamilies —— **静态导览表**）＋ 强迫先看 help ＋ 三条起手式
//	⑤ 输出被截断时，在尾部点名「换精确读面」而不是放弃（zergCmdClipNote）
//
// 实测铁证（会话 chat_20261017_151842_e25e6e）：模型已能正确发 {name:zerg, command:"zerg gap ls"}
// （工具面收窄已生效），但**只猜了这一种子命令** —— 推理原文「The output was truncated and didn't show
// the specific gap I'm looking for. I need to filter the results.」⇒ 它不知道 `zerg gap show <GAP id>`
// 这个**一字不截**的精确读面存在，也从未跑 `zerg help gap`。⇒ 描述里不给族清单与「先 help」纪律，
// 模型就只能靠猜；输出截断时不给「换哪条」，它就只能放弃。
package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// ZergCmdToolName — 对话唯一 CLI 工具名
const ZergCmdToolName = "zerg"

// ZergCmdWord — 命令首词口径（逐字比对——✗ 不收 zerg.exe / ZERG / ./zerg）
const ZergCmdWord = "zerg"

// ★ GAP-20261018-25（2026-10-18 · P0）：**执行凭据**。
//
// 病灶（实测 · 已亲核）：本地卵在 chat 会话里编造了完整的假回执 —— 提案号、rc=0、
// after_sha256、hunk 数四处全假，格式与真回执一模一样；而 chat 路径没有任何回执核验
// （loopcore 侧有，见 GAP-20261018-01 的 output_claims_verify）。
// 解法：每次工具执行发一枚短凭据 `x-<8hex>`，随输出头一起回给模型；回答里引用
// rc / sha / 字节数 / hunk 时**必须带上它**，否则按未取证处理（见 chat_exec_receipt.go）。
var (
	zergExecMu   sync.Mutex
	zergExecSeen = map[string]bool{}
)

// zergNewExecID —— 发一枚执行凭据（8 位 hex）并记进本轮集合。
func zergNewExecID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "x-00000000"
	}
	id := "x-" + hex.EncodeToString(b)
	zergExecMu.Lock()
	zergExecSeen[id] = true
	zergExecMu.Unlock()
	return id
}

// ZergExecSeenIDs —— 本轮已发凭据的快照（校验器只认这里出现过的 id）。
func ZergExecSeenIDs() map[string]bool {
	zergExecMu.Lock()
	defer zergExecMu.Unlock()
	out := make(map[string]bool, len(zergExecSeen))
	for k := range zergExecSeen {
		out[k] = true
	}
	return out
}

// zergCmdRefusalHead — 拒执文案（用户点名要逐字那句「对话只能用虫族命令」）
const zergCmdRefusalHead = "rc=2 拒执：对话只能用虫族命令"

// zergCmdTimeout — 单条命令墙钟上限（对话是交流——不是长任务；超时即拒执报因）
const zergCmdTimeout = 120 * time.Second

// zergCmdOutLimit — 回给模型的输出上限（字符——防巨型输出吃掉上下文）
const zergCmdOutLimit = 8000

// zergCmdBannedSeqs — 禁用元字符序列（不经 shell ⇒ 管道/重定向/串联/命令替换/多行一律拒执）
var zergCmdBannedSeqs = []string{";", "|", "&", "<", ">", "$(", "`", "\n", "\r"}

// chatForbiddenTools — 对话面**硬禁**工具（GAP-20261017-09）：主提示不出 ＋ 执行面 fail-closed。
//
// ★ 只在本层过滤，✗ 不改 agent.AllTools()、✗ 不动 L0ToolNames 常量本身
// （L0ToolNames 是别的通道共用的真源，见 chat_tool_registry.go）。
//
// ★ 2026-10-17 二次收窄（用户口径「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」）：
// web_search / web_fetch / skill_load / zerg_overview 这四条**也不是虫族命令** ⇒ 一并撤下。
//
//	skill_load    ⇒ 替代已是虫族命令：`zerg skill ls`（清单）→ `zerg skill show <名>`（正文，与 CA 侧同字节）
//	web_search / web_fetch ⇒ **无替代**：虫族 CLI 本版没有搜网正门（已知缺口 GAP-20261017-10 邻近面）
//	                          ⇒ 对话**暂时失去搜网能力**——如实说，✗ 不造替代、✗ 不许编搜索结果
//	zerg_overview ⇒ 它不是虫族命令（是 chat 层聚合器）⇒ 撤；要看系统用 `zerg help` / `zerg version`
var chatForbiddenTools = map[string]bool{
	"bash":  true, // 旧 shell 面（实测那条会话就是从这里绕出去的）
	"read":  true,
	"write": true,
	"edit":  true,
	"glob":  true,
	"grep":  true,
	"ls":    true,
	// 2026-10-17 二次收窄：四条非虫族命令（本单点名撤下）
	"web_search":    true,
	"web_fetch":     true,
	"skill_load":    true,
	"zerg_overview": true,
}

// chatPromptExcluded — 主提示面不出现（硬禁集 ＋ 这两个「引导器」）：
// tool_search 是 deferred 工具的总入口、kb_search 是知识库直查——留着它们等于把模型引回非虫族工具。
var chatPromptExcluded = map[string]bool{
	"tool_search": true,
	"kb_search":   true,
}

// ChatToolForbidden — 该工具是否已撤出对话工具面（执行面硬闸口径）
func ChatToolForbidden(name string) bool {
	return chatForbiddenTools[strings.ToLower(strings.TrimSpace(name))]
}

// chatPromptHidden — 该工具是否不得出现在对话主提示（<tools> 段 / tools 参数）
func chatPromptHidden(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return chatForbiddenTools[n] || chatPromptExcluded[n]
}

// ChatExecGuard — 撤下工具在**执行入口**的兜底闸（GAP-20261017-09 二次收窄）。
//
// 为什么非要这一道：agent 的 executeToolInner 只在 bash/read/write/edit/glob/grep/ls 等 case 内
// 自调 gate.Check —— web_search / web_fetch 分支**根本不查门**（2026-10-17 实测：对话路径兑旧名
// 直调真联网、返回网页内容）⇒ 光靠 ChatGate 挡不住这两条，「对话只能用虫族命令」必须在对话
// 自己的执行入口再挡一道。
//
// 返回值：nil = 放行；非 nil = 该调用已拒执（文案与 ChatGate 同一句 —— ✗ 别另写一份）。
func ChatExecGuard(session, name string, args map[string]any) error {
	if !ChatToolForbidden(name) {
		return nil
	}
	agent.ObserveToolDeny(session, name, toolobs.ReasonToolHidden, args)
	return fmt.Errorf("%s", chatZergOnlyRefusal(name))
}

// chatZergOnlyRefusal — 撤下工具的逐字拒执文案（点名工具 ＋ 给出替代路径）
func chatZergOnlyRefusal(name string) string {
	return fmt.Sprintf("%s（工具 %q 已从对话工具面撤下）。要看文件/查系统/跑脚本，一律用 %s 工具发虫族命令"+
		"（如 {\"command\":\"zerg help\"}、{\"command\":\"zerg code show <件>:<行>\"}、{\"command\":\"zerg gap ls\"}）；"+
		"要看技能先 {\"command\":\"zerg skill ls\"} 再 {\"command\":\"zerg skill show <名>\"}；"+
		"搜网本对话没有这个能力（web_search/web_fetch 已撤、虫族 CLI 也没有搜网命令）——如实说做不到",
		zergCmdRefusalHead, name, ZergCmdToolName)
}

// ChatZergToolParam — zerg 工具定义（chat 格式 tools 参数）——
// 描述与参数 schema 取自本件两个真源符号（zergCmdToolDesc / zergCmdToolProps），
// <tools> 段那一行也读同一对（单一真源——✗ 别在两处各写一份描述）。
func ChatZergToolParam() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        ZergCmdToolName,
			"description": zergCmdToolDesc,
			"parameters": map[string]any{
				"type":       "object",
				"properties": zergCmdToolProps(),
				"required":   []string{"command"},
			},
		},
	}
}

// zergCmdFamily — 命令树里的一个族（族名 ＋ 该族动作数）
type zergCmdFamily struct {
	name    string
	actions int
}

// zergCmdFamilies — 命令树族清单（**静态导览表**，GAP-20261017-11）。
//
// ★ 真源 = `zerg help --json family,actions`（现读命令树派生），✗ 不是这张表。
// 为什么不在运行时现读：命令树真身在 `cmd/zerg`（package main），chat 包 import 不了它；
// 也不该在每次渲染工具描述时起一个子进程去问 CLI（给一段文案付一次进程启动成本）。
// ⇒ 这张表**只供导览**：让模型先知道有哪些族、哪族动作多，再自己去 `zerg help <族>` 查准的。
//
// 陈旧是可发现的（✗ 不是静默的）：单测 TestZergCmdFamilyTableMatchesLiveTree 拿这张表与
// `zerg help --json family,actions` 现读输出逐条对拍（族数 + 每族动作数），命令树一变就红。
//
// 序：动作数降序、同数按族名字母序（渲染时不再排序 —— 改表就照这个序插）。
var zergCmdFamilies = []zergCmdFamily{
	{"gap", 32},
	{"agent", 20},
	{"gate", 18},
	{"task", 13},
	{"chat", 12},
	{"egg", 13},
	{"model", 11},
	{"core", 10},
	{"dev", 10},
	{"itask", 9},
	{"publish", 8},
	{"code", 8},
	{"build", 6},
	{"cocoon", 6},
	{"doc", 6},
	{"approve", 5},
	{"skill", 5},
	{"archive", 4},
	{"eval", 4},
	{"fs", 4},
	{"resource", 4},
	{"sandbox", 4},
	{"sched", 4},
	{"script", 4},
	{"api", 3},
	{"calib", 3},
	{"repo", 3},
	{"route", 3},
	{"swap", 3},
	{"ui", 3},
	{"audit", 2},
	{"context", 2},
	{"fleet", 2},
	{"gateway", 2},
	{"help", 2},
	{"wall", 2},
	{"apply", 1},
	{"ask", 1},
	{"ci", 1},
	{"config", 1},
	{"deploy", 1},
	{"doctor", 1},
	{"find", 1},
	{"impact", 1},
	{"isolate-ro", 1},
	{"ls-face", 1},
	{"mcp", 1},
	{"metrics", 1},
	{"net", 1},
	{"plan", 1},
	{"plugin", 1},
	{"port", 1},
	{"propose", 1},
	{"root", 1},
	{"update", 1},
	{"version", 1},
	{"watch", 1},
}

// zergCmdFamilySummary — 族清单摘要（形如 "gap(32) agent(20) gate(18) …"）
func zergCmdFamilySummary() string {
	var b strings.Builder
	for i, f := range zergCmdFamilies {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s(%d)", f.name, f.actions)
	}
	return b.String()
}

// zergCmdDescHead — 引言与硬约束段（工具面唯一的工具、首词、禁用元字符、已被撤下的工具、技能正门）
const zergCmdDescHead = "跑一条虫族命令（虫族 CLI —— 对话唯一可用的工具）。" +
	"【硬约束】command 的首词必须逐字 zerg（如 \"zerg version\"、\"zerg gap ls\"、" +
	"\"zerg code show core/internal/chat/chat_tools.go:69\"）；不许管道/重定向/串联/命令替换" +
	"（; | & < > $( 反引号 与多行 一律 rc=2 拒执）；没有 bash/read/write/edit/glob/grep/ls，" +
	"也没有 web_search/web_fetch/skill_load/zerg_overview（本对话没有搜网能力——要联网就如实说做不到）。" +
	"【要看技能】先 \"zerg skill ls\"（清单）再 \"zerg skill show <名>\"（正文）——技能只走这两条。"

// zergCmdDescHelpFirst — ② 强迫先看 help ＋「输出太大别拿 ls 刷屏」
const zergCmdDescHelpFirst = "【不确定子命令 ⇒ 先跑 zerg help <族>】子命令名不许猜：" +
	"不确定某个族里有哪些动作，就先 \"zerg help <族>\"（如 \"zerg help gap\"）——" +
	"它把该族每条动作的用法串、旗标、是否危险档逐条印出来，查一次的代价远小于猜错重试。" +
	"【输出太大 ⇒ 换精确读面，别拿 ls 刷屏】\"zerg … ls\" 的输出被截断、看不到你要那一条时，" +
	"改用「能按 id 精确取」的读面：\"zerg gap show <GAP id>\"（单条全文、一字不截）、" +
	"\"zerg code show <件>:<行>\"（该行及上下文）、\"zerg gate find <词>\"（按词定位判据）——" +
	"✗ 别拿 ls 一遍遍刷屏，✗ 也别就此放弃。"

// zergCmdDescExamples — ③ 三条起手式（照着发就能跑）
const zergCmdDescExamples = "【三条起手式】① \"zerg help gap\"（先看这一族有哪些动作）；" +
	"② \"zerg gap show GAP-20261017-11\"（按 id 取单条全文——不是 ls）；" +
	"③ \"zerg code find 'zergCmdToolDesc' --path core\"（按正则找件与行，再用 code show 取上下文）。"

// zergCmdDescFinish — ④ 交答案纪律（GAP-20261017-12 的提示词面）
//
// 实测：加了族清单导航后模型**方向全对**（gap show 取到 want → code find → code show，
// 真读到零命中），却以「让我看源码上下文」这类**下一步打算**收尾 ⇒ 用户拿到过程、没拿到交付物。
// 这一段只做**先手引导**（软约束）；硬闸在 loopcore 的 natural 前置闸（同类收尾会被要求重答一次）。
const zergCmdDescFinish = "【任务完成 ⇒ 直接交答案·✗ 不要再调工具】证据够了就停手，正文直接写结论" +
	"（是什么 / 在哪 / 有没有 / 结论）；✗ 写「让我再看…／我打算…／下一步去查…」**不算完成**——" +
	"那是过程不是交付；结论照任务要求的形状给（要几行就给几行）。"

// zergCmdToolDesc — zerg 工具描述（对话工具面唯一工具）：硬约束 ＋ 命令树族清单摘要 ＋
// 「先 help」纪律 ＋ 三条起手式 ＋ 交答案纪律。
//
// ★ 用 var 而非 const：族清单摘要由 zergCmdFamilies 派生（单一真源），✗ 别在字面量里再手抄一份
// 摘要（手抄一份必然与表脱钩，而脱钩了没有任何判据看得见）。
var zergCmdToolDesc = zergCmdDescBuild()

// zergCmdDescBuild — 拼描述（族数 ＋ 摘要 ＋ 四段）
func zergCmdDescBuild() string {
	return zergCmdDescHead +
		"【命令树·族清单（" + fmt.Sprintf("%d", len(zergCmdFamilies)) + " 族 · 括号内＝该族动作数）】" +
		zergCmdFamilySummary() + "。" +
		"（真源 = \"zerg help --json family,actions\"；此表仅供导览，命令树一变就要同步——有单测对拍）" +
		zergCmdDescHelpFirst + zergCmdDescExamples +
		zergCmdDescFinish
}

// zergCmdToolProps — zerg 工具参数 schema（唯一参数 command）
func zergCmdToolProps() map[string]any {
	return map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "一条虫族命令，首词逐字 zerg（例：zerg version / zerg gap ls --state 仍缺）",
		},
	}
}

// zergCmdVerdict — 执行面硬闸：返回拒执理由（""＝放行）。
//
// fail-closed 三条：① 非空 ② 无禁用元字符 ③ 首词逐字 zerg。
func zergCmdVerdict(cmd string) string {
	s := strings.TrimSpace(cmd)
	if s == "" {
		return zergCmdRefusalHead + "（参数 command 不能为空——给一条虫族命令，如 {\"command\":\"zerg version\"}）"
	}
	for _, bad := range zergCmdBannedSeqs {
		if strings.Contains(s, bad) {
			return zergCmdRefusalHead + fmt.Sprintf("（命中禁用字符 %q——不许管道/重定向/串联/命令替换/多行）", bad)
		}
	}
	argv, err := splitZergArgv(s)
	if err != nil {
		return zergCmdRefusalHead + "（" + err.Error() + "）"
	}
	if len(argv) == 0 || argv[0] != ZergCmdWord {
		got := "<空>"
		if len(argv) > 0 {
			got = argv[0]
		}
		return zergCmdRefusalHead + fmt.Sprintf("（首词必须逐字 %q——实得 %q）", ZergCmdWord, got)
	}
	return ""
}

// splitZergArgv — 极简 argv 切分（支持单/双引号与反斜杠转义——✗ 不做变量展开/通配/命令替换）。
// 切不开（引号未闭合）⇒ 报错（由调用方转成拒执，✗ 猜一个切法硬跑）。
func splitZergArgv(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	var quote byte
	inTok := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch {
			case c == quote:
				quote = 0
			case c == '\\' && quote == '"' && i+1 < len(s):
				i++
				b.WriteByte(s[i])
			default:
				b.WriteByte(c)
			}
			continue
		}
		switch c {
		case ' ', '\t':
			if inTok {
				out = append(out, b.String())
				b.Reset()
				inTok = false
			}
		case '\'', '"':
			quote = c
			inTok = true
		case '\\':
			if i+1 < len(s) {
				i++
				b.WriteByte(s[i])
				inTok = true
			}
		default:
			b.WriteByte(c)
			inTok = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("引号未闭合（%q）", string(quote))
	}
	if inTok {
		out = append(out, b.String())
	}
	return out, nil
}

// zergBinaryPath — 定位 zerg 可执行件（ZERG_BIN ＞ <工作区>/bin/zerg ＞ PATH）。
// 三态分开：设了却读不到 ⇒ 出声（✗ 静默回落 PATH——那会把「指错」伪装成「跑起来了」）。
func zergBinaryPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("ZERG_BIN")); v != "" {
		if st, err := os.Stat(v); err == nil && !st.IsDir() {
			return v, nil
		}
		return "", fmt.Errorf("ZERG_BIN 指的件不在盘上或不是普通件：%s", v)
	}
	roots := []string{statepath.WorkspaceRoot(), ChatToolsWorkDir, "."}
	for _, root := range roots {
		if root == "" {
			continue
		}
		p := filepath.Join(root, "bin", "zerg")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath(ZergCmdWord); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("找不到 zerg 可执行件（看了 ZERG_BIN、%s/bin/zerg、PATH）——对话只能用虫族命令，没有 CLI 就跑不动",
		statepath.WorkspaceRoot())
}

// ChatZergCommand — zerg 工具执行器（先硬闸，再 argv 直传；✗ 不经 shell / sh -c）。
//
// 返回形如 "rc=0\n--- stdout ---\n…"（rc 逐字给模型看——成功与失败都回 rc）。
func ChatZergCommand(args map[string]any, workDir string) (string, error) {
	cmdStr, _ := args["command"].(string)
	if why := zergCmdVerdict(cmdStr); why != "" {
		return "", errors.New(why)
	}
	argv, err := splitZergArgv(strings.TrimSpace(cmdStr))
	if err != nil { // verdict 已挡过——这里只是 fail-closed 兜底
		return "", errors.New(zergCmdRefusalHead + "（" + err.Error() + "）")
	}
	bin, err := zergBinaryPath()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), zergCmdTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, bin, argv[1:]...)
	if workDir != "" {
		c.Dir = workDir
	}
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	runErr := c.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s（超时 %s——虫族命令跑太久）", zergCmdRefusalHead, zergCmdTimeout)
	}
	rc := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			rc = ee.ExitCode()
		} else {
			return "", fmt.Errorf("zerg 命令跑不起来（%s）：%w", bin, runErr)
		}
	}
	execID := zergNewExecID()
	var b strings.Builder
	fmt.Fprintf(&b, "rc=%d\n", rc)
	// ★ GAP-20261018-25：凭据行放在 rc 之后 —— 输出**必须以 rc=N 开头**是既有契约
	// （TestChatZergToolRunsVersion 逐字断言），凭据只做增量、不抢首位。
	fmt.Fprintf(&b, "[EXEC %s] 以上结果的凭据；引用 rc / sha256 / 字节数 / hunk 时请原样带上它，没带即视为未取证\n", execID)
	b.WriteString("--- stdout ---\n")
	b.WriteString(zergClip(stdout.String()))
	if s := strings.TrimRight(stderr.String(), "\n"); s != "" {
		b.WriteString("\n--- stderr ---\n")
		b.WriteString(zergClip(s))
	}
	return b.String(), nil
}

// zergClip — 按**字符**截断（✗ 按字节切会把中文切坏——回给模型的 JSON 会变 U+FFFD）。
//
// GAP-20261017-11：截断时**必须**在尾部点名「换哪条命令」，✗ 不能只报一句「已截断」——
// 实测（会话 chat_20261017_151842_e25e6e）模型拿到被截断的 `zerg gap ls` 之后就放弃了
// （推理原文「The output was truncated and didn't show the specific gap I'm looking for.
// I need to filter the results.」），因为它不知道还有按 id 精确取的一字不截读面。
// 提示文案见 zergCmdClipNote（含原样字节数——让模型自己判断该换读面还是收窄条件）。
func zergClip(s string) string {
	s = strings.TrimRight(s, "\n")
	r := []rune(s)
	if len(r) <= zergCmdOutLimit {
		return s
	}
	return string(r[:zergCmdOutLimit]) + "\n…" + zergCmdClipNote(len(s), len(r))
}

// zergCmdClipNote — 截断提示（原样字节数 / 字符数 ＋ 「改用能按 id 精确取的子命令」）。
//
// 措辞三条：① 说清「截断的是显示、不是命令失败」（✗ 别让模型以为命令坏了）；
// ② 给两条**真存在**的精确读面（gap show / code show）＋一条定位面（help <族>）；
// ③ 明确「别拿 ls 刷屏、别放弃」——这正是那次会话里模型停下的原因。
//
// ★ GAP-20261018-09（2026-10-18）第四条：**改件不必读全文**。病灶是「`fs write` 整件覆盖 ⇒
// 要先拿到全文」＋「读面两层截断」互相锁死：19265 字节的真件只取得到约 8KB，模型判对了、
// `--dry-run` 也说可写，却拼不出「完整原文 + 只改一行」⇒ 停手不写、门禁仍红。
// ⇒ 截断提示里必须点名 `fs write --replace <补丁件>`（定点替换：每格 `old` 恰好命中 1 次），
// 让模型看见「这条路不用读全文」，而不是继续按 `--offset` 一窗窗拼。
func zergCmdClipNote(bytesN, runesN int) string {
	return fmt.Sprintf("（输出已截断，共 %d 字节 / %d 字符；只回了前 %d 字符。"+
		"要找单条目请用能按 id 精确取的子命令：`zerg gap show <GAP id>`（单条全文、一字不截）、"+
		"`zerg code show <件>:<行>`、`zerg gate find <词>`；不确定该族有哪些动作先 `zerg help <族>`。"+
		"★ 要**改**件也**不必读全文**：`zerg fs write <件> --replace <补丁件> --yes`（补丁件 = "+
		"`[{\"old\":…,\"new\":…}]` · 每格 `old` 在件里**恰好命中 1 次** ⇒ 定点改，其余字节一个不动）"+
		"—— ✗ 别为了改一行去按 --offset 一窗窗把全文拼回来。"+
		"✗ 别拿 ls 一遍遍刷屏，✗ 也别放弃）", bytesN, runesN, zergCmdOutLimit)
}

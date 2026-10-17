// chat_tool_zergcmd.go — GAP-20261017-09 对话工具面收窄：只剩虫族命令（zerg）
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
package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// ZergCmdToolName — 对话唯一 CLI 工具名
const ZergCmdToolName = "zerg"

// ZergCmdWord — 命令首词口径（逐字比对——✗ 不收 zerg.exe / ZERG / ./zerg）
const ZergCmdWord = "zerg"

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

// zergCmdToolDesc — zerg 工具描述（对话工具面唯一工具——讲清硬约束、「先 help」与技能正门）
const zergCmdToolDesc = "跑一条虫族命令（虫族 CLI —— 对话唯一可用的工具）。" +
	"【硬约束】command 的首词必须逐字 zerg（如 \"zerg version\"、\"zerg gap ls\"、" +
	"\"zerg code show core/internal/chat/chat_tools.go:69\"）；不许管道/重定向/串联/命令替换" +
	"（; | & < > $( 反引号 与多行 一律 rc=2 拒执）；没有 bash/read/write/edit/glob/grep/ls，" +
	"也没有 web_search/web_fetch/skill_load/zerg_overview（本对话没有搜网能力——要联网就如实说做不到）。" +
	"【要看技能】先 \"zerg skill ls\"（清单）再 \"zerg skill show <名>\"（正文）——技能只走这两条。" +
	"【不确定用法】先跑 \"zerg help\"（全命令树）或 \"zerg help <族名>\"（如 zerg help gap）。"

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
	var b strings.Builder
	fmt.Fprintf(&b, "rc=%d\n", rc)
	b.WriteString("--- stdout ---\n")
	b.WriteString(zergClip(stdout.String()))
	if s := strings.TrimRight(stderr.String(), "\n"); s != "" {
		b.WriteString("\n--- stderr ---\n")
		b.WriteString(zergClip(s))
	}
	return b.String(), nil
}

// zergClip — 按**字符**截断（✗ 按字节切会把中文切坏——回给模型的 JSON 会变 U+FFFD）
func zergClip(s string) string {
	s = strings.TrimRight(s, "\n")
	r := []rune(s)
	if len(r) <= zergCmdOutLimit {
		return s
	}
	return string(r[:zergCmdOutLimit]) + fmt.Sprintf("\n…（共 %d 字符——已截断）", len(r))
}

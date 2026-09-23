package agent

// bash_v101.go — bash 工具 v1.0.1「AI 专用执行契约」（2026-09-06 设计定稿）
// 设计: docs/01-设计/设计-bash-工具-AI-专用执行契约-v1.0.1.md
// 核心: 系统断言成败（模型不猜）+ 三段式返回 + 本地模型弥补层（防呆/引导/预算）
//
// 返回契约（模型可靠解析）:
//   ⚠️ exit N — 命令失败          ← exit≠0 时首行（系统断言）
//   <stdout 主体>                 ← 头尾窗口保留（中间省略落盘）
//   [stderr]                      ← stderr 独立段
//   <stderr 内容>
//   [note]/[guide]/[state_note]   ← 系统辅助信息
//   [exit_code] N [workdir] ... [duration_ms] ... [output_bytes] ...

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// bash v1.0.1 输出预算（本地模型上下文纪律弱——收紧默认）
const (
	bashOutLimit  = 6000 // bash 默认输出上限（v1.0.0 16000 → 6000——弱模型防淹没）
	bashHeadLimit = 2000 // 头部保留
	bashTailLimit = 4000 // 尾部保留（错误/日志终点常在尾）

	// bashOverflowDirName 溢出落盘目录在统一状态目录下的目录名。
	bashOverflowDirName = "zerg-bash-overflow"
	// bashOverflowLegacyDefaultDir 旧硬编码落点字面量（2026-09-06 起写死 const BashOverflowDir）。
	// 只作**迁移兼容的只读来源**：旧目录仍在访问白名单里（旧溢出文件还能被 read 续读）；
	// 不删、不改、**永不再写入**。
	bashOverflowLegacyDefaultDir = "/tmp/zerg-bash-overflow"
)

// legacyBashOverflowDir 旧溢出目录（包级变量 = 上面的字面量；用例/测试进程隔离可切换）。
var legacyBashOverflowDir = bashOverflowLegacyDefaultDir

// bashOverflowDirOverride 显式覆盖（测试隔离）：非空 ⇒ 写/白名单都用它，且不复旧目录。
var bashOverflowDirOverride = ""

// BashOverflowDir 溢出落盘目录（2026-09-18 修 /tmp 硬编码——口径同 api/tasks_persist.go）。
//
//	原写死 const BashOverflowDir = "/tmp/zerg-bash-overflow" —— macOS 重启 /tmp 即清、
//	tmp_cleaner 3 天未访问即删（溢出原文丢）；多实例还共用同一目录。
//	改为 statepath 统一状态目录派生（ZERG_STATE_DIR → ~/.zerg/state/zerg-bash-overflow）；
//	新目录不存在则首次使用自动创建（bashSpillDir）。
//
// 由常量改为函数：常量无法按 ZERG_STATE_DIR 派生，且测试需要可覆盖。
func BashOverflowDir() string {
	if p := strings.TrimSpace(bashOverflowDirOverride); p != "" {
		return p
	}
	return statepath.File(bashOverflowDirName)
}

// bashOverflowAllowDirs 溢出目录的访问白名单（NewExecContext 注入 ExtraAllowDirs——模型 read 续读）。
// 第一个永远是写落点（统一状态目录派生）；旧 /tmp/zerg-bash-overflow 若存在 ⇒ 追加为**只读**入口。
// 为什么目录类保留旧目录（与文件类「新在则旧完全不看」的差异，理由写在这）:
//
//	文件的迁移兼容是「读旧内容一次」（内容会夹除，故新在则必须不看旧）；
//	目录类没有「读内容」这一步——这里是访问**许可**，不夹除任何内容。
//	去掉旧目录 = 用过旧版本留下的溢出文件从此读不了（真回归）；保留它只是多一条只读白名单前缀，
//	写路径永远只走新目录（永不写旧目录）。显式覆盖（测试隔离）⇒ 不追加旧目录。
func bashOverflowAllowDirs() []string {
	writeDir := BashOverflowDir()
	dirs := []string{writeDir}
	if strings.TrimSpace(bashOverflowDirOverride) != "" {
		return dirs // 显式覆盖 ⇒ 不复旧（覆盖即「已指定唯一来源」）
	}
	legacy := strings.TrimSpace(legacyBashOverflowDir)
	if legacy == "" || legacy == writeDir {
		return dirs
	}
	if st, err := os.Stat(legacy); err != nil || !st.IsDir() {
		return dirs // 无旧目录——首次运行
	}
	// 本函数每次 NewExecContext（每个工具调用/任务）都会跑——迁移提示只喊一次，防日志刷屏
	bashOverflowLegacyNoticeOnce.Do(func() {
		log.Printf("📜 bash 溢出目录迁移兼容: 旧 %s 保留只读（写入只落 %s；旧目录不删不改）\n", legacy, writeDir)
	})
	return append(dirs, legacy)
}

// bashOverflowLegacyNoticeOnce 旧目录只读兼容提示（进程内一次）。
var bashOverflowLegacyNoticeOnce sync.Once

// bashSpillDir — 返回溢出目录并确保存在
func bashSpillDir() string {
	dir := BashOverflowDir()
	os.MkdirAll(dir, 0o755)
	return dir
}

// spillBashOutput — 超长/二进制输出全量落盘，返回文件路径
// 模型可 read 续读（目录在 ExtraAllowDirs 白名单——validatePath 放行）
func spillBashOutput(data []byte, tag string) string {
	dir := bashSpillDir()
	path := filepath.Join(dir, fmt.Sprintf("bash-%s-%d.log", tag, time.Now().UnixNano()))
	os.WriteFile(path, data, 0o644)
	return path
}

// bashIsBinary — 二进制防护：含 NUL 或大量非法 UTF-8 → 判定二进制
// 业界 binary guard（agentpatterns）——防乱码灌上下文
func bashIsBinary(data []byte) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	if len(data) == 0 {
		return false
	}
	bad := 0
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	for len(sample) > 0 {
		r, size := utf8.DecodeRune(sample)
		if r == utf8.RuneError && size == 1 {
			bad++
			if bad > 8 { // 采样 4K 内超过 8 个坏字符 → 二进制/乱码
				return true
			}
		}
		sample = sample[size:]
	}
	return false
}

// bashWindow — 长输出头尾窗口化 + 溢出落盘
// 返回: 展示文本 + 溢出文件路径（空=无溢出）+ 原始总长
func bashWindow(data []byte) (string, string, int) {
	total := len(data)
	if total <= bashOutLimit {
		return string(data), "", total
	}
	head := string(data[:bashHeadLimit])
	tail := string(data[total-bashTailLimit:])
	mid := total - bashHeadLimit - bashTailLimit
	path := spillBashOutput(data, "overflow")
	// 中文省略标注（模型知道还有多少没看——防误以为全貌）
	shown := fmt.Sprintf("%s\n\n…[中间省略 %d 字符——全量已存 %s——需要时用 read 该文件续读]…\n\n%s",
		head, mid, path, tail)
	return shown, path, total
}

// bashQuoteBalanced — 引号闭合检查（防 bash 等续行挂死）
// 忽略转义引号（\' \"）
func bashQuoteBalanced(s string) bool {
	for _, q := range []byte{'\'', '"'} {
		count := 0
		escaped := false
		for i := 0; i < len(s); i++ {
			c := s[i]
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == q {
				count++
			}
		}
		if count%2 != 0 {
			return false
		}
	}
	return true
}

// bashPokaYokeCheck — 防呆形态拦截（本地模型输入不稳——结构性拦截，非文档教育）
// 返回错误信息（nil=通过）。拦截给引导不纯拒绝（虫族安全哲学）
func bashPokaYokeCheck(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return fmt.Errorf("空命令——bash 需执行内容（可用 ls 看目录/read 读文件/专用工具做操作）")
	}
	// 仅注释
	if strings.HasPrefix(trimmed, "#") {
		return fmt.Errorf("命令仅为注释——无实际操作")
	}
	// 后台任务: 最后一个字段是 &（防 exit 0 假完成——活没干完）
	// 排除: 2>&1（结尾 1）/ &>（重定向）/ &&（且）——字段切分自然区分
	fields := strings.Fields(trimmed)
	if len(fields) > 0 && fields[len(fields)-1] == "&" {
		return fmt.Errorf("命令以后台 & 结尾——后台任务不等待完成易误判成功（exit 0 但活没干完）——请同步执行（去掉末尾 &）或拆分为前台命令")
	}
	// 引号未闭合（bash 会等续行——挂死）
	if !bashQuoteBalanced(trimmed) {
		return fmt.Errorf("[guide] 引号未闭合——检查转义（bash 会等待续行导致挂死）。单引号/双引号需成对")
	}
	// 交互命令（裸解释器/全屏工具——等 stdin 输入挂死）
	if len(fields) > 0 {
		switch fields[0] {
		case "vim", "vi", "nano", "less", "more", "top", "htop", "python", "python3", "node", "irb", "sqlite3", "mysql", "psql", "bash", "sh", "zsh":
			// 裸解释器（无参数脚本）→ 拦截；带脚本参数（python3 x.py）合法
			if len(fields) == 1 {
				return fmt.Errorf("[guide] %s 是交互/解释器命令——非交互执行会挂死等输入。读文件用 read 工具；列目录用 ls；要跑脚本请给完整命令（如 python3 脚本.py）", fields[0])
			}
			// 首个参数以 - 开头（python3 -i / bash -i）→ 交互模式
			if strings.HasPrefix(fields[1], "-i") || fields[1] == "-" {
				return fmt.Errorf("[guide] %s %s 进入交互模式——非交互执行会挂死。请用非交互参数", fields[0], fields[1])
			}
		}
	}
	return nil
}

// bashClassifyError — 失败错误分类引导（确定性分类——系统做，非模板建议）
// exit≠0 时按 stderr 模式分类——给 1-2 行"下一步选项"
func bashClassifyError(stderrText string, cmd string) string {
	s := strings.ToLower(stderrText)
	switch {
	case strings.Contains(s, "command not found"):
		return "[guide] 命令不存在——可能拼错或未安装。可 which <命令名> 查证，或检查是否需要专用工具"
	case strings.Contains(s, "permission denied"):
		return "[guide] 权限不足——检查文件执行权限/目录所有权"
	case strings.Contains(s, "no such file or directory"):
		return "[guide] 路径不存在——若涉及之前 cd 的目录，下条命令请用 cwd 参数显式指定工作目录"
	case strings.Contains(s, "syntax error") || strings.Contains(s, "unexpected token"):
		return "[guide] 语法错误——检查引号/转义/管道——复杂命令可拆成多条执行"
	case strings.Contains(s, "killed") || strings.Contains(s, "cannot allocate memory"):
		return "[guide] 进程被杀（内存/资源）——可拆分小批次执行"
	case strings.Contains(cmd, "cd ") && strings.Contains(s, "no such"):
		return "[guide] cd 目标目录不存在——用 ls 确认目录名后重试"
	}
	return ""
}

// bashMetaFooter — 元数据尾注（机器可读锚点——模型判断成败看这里）
func bashMetaFooter(exitCode int, workdir string, durMs int64, totalBytes int, spillPath string) string {
	var sb strings.Builder
	if spillPath != "" {
		fmt.Fprintf(&sb, "\n[truncated] 溢出全量→%s（read 可续读）", spillPath)
	}
	fmt.Fprintf(&sb, "\n[exit_code] %d\n[workdir] %s\n[duration_ms] %d\n[output_bytes] %d", exitCode, workdir, durMs, totalBytes)
	return sb.String()
}

// bashStateNote — cd/export 状态提示（无状态 shell 补偿——教模型用 cwd 参数）
func bashStateNote(command string) string {
	hasCD := regexp.MustCompile(`(^|;|&&|\|\|)\s*cd\s+`).MatchString(command)
	hasExport := regexp.MustCompile(`(^|;|&&|\|\|)\s*export\s+`).MatchString(command)
	if hasCD || hasExport {
		return "[state_note] cd/export 不跨调用持久（每次 bash 独立 shell）——后续命令若依赖该目录/环境，请用 cwd 参数显式指定工作目录"
	}
	return ""
}

// bashExpandHomeOnly — 家目录变量/波浪号展开（~/、$HOME、${HOME}→真实家目录路径）
func bashExpandHomeOnly(command string) string {
	expanded := command
	home, _ := os.UserHomeDir()
	expanded = strings.ReplaceAll(expanded, "$HOME", home)
	expanded = strings.ReplaceAll(expanded, "${HOME}", home)
	expanded = strings.ReplaceAll(expanded, "~/", home+"/")
	expanded = strings.ReplaceAll(expanded, "~ ", home+" ")
	if strings.HasSuffix(expanded, "~") {
		expanded = expanded[:len(expanded)-1] + home
	}
	return expanded
}

// BashCheckRmHome — 导出给 chat_gate 等复用（2026-09-07 事故修复）:
// 展开家目录形态后做 rm 参数级目标防护（/、家目录本身/祖先/内部 → 拒）
func BashCheckRmHome(command string) error {
	return bashRmTargetGuard(bashExpandHomeOnly(command))
}

// bashExpandDangerScan — 危险命令双检（原文 + 常见变量展开后重扫）
// 防绕过: base64 -d | sh / eval / $() / 变量拼接 rm（$HOME/$ROOT 展开到真实路径）
//
// T1.2：判定逻辑只有 bashExpandDangerScanCoded 一份（本函数转调它）——
// 观测要的"拒绝原因码"就在每个分支上给出，**不用错误文本反推**（反推=脆且会漂）。
func bashExpandDangerScan(command string) error {
	_, err := bashExpandDangerScanCoded(command)
	return err
}

// bashExpandDangerScanCoded — 同上，另返回**拒绝原因码**（toolobs.ReasonXxx；无拒绝=空串）。
func bashExpandDangerScanCoded(command string) (string, error) {
	// 第一层: 原文直接查（现状 checkDangerousCommand 已做——此处查展开形态）
	expanded := bashExpandHomeOnly(command)
	// 空变量展开危险: rm -rf $VAR（VAR 未设→rm -rf 后空——bash 会报参数缺——不真删根）
	// 但 rm -rf $VAR/ (VAR空→rm -rf /) 危险——已含 rm -rf / 模式，展开扫兜底
	if err := checkDangerousCommand(expanded); err != nil {
		return toolobs.ReasonDangerousCmd, err
	}
	// 事故修复(2026-09-07): rm 参数级目标防护——endOnly 黑名单放行 ~/xxx、$HOME/xxx、
	// 家目录绝对路径(endOnly 本意放行 /tmp/xxx,同规则漏掉家目录)——展开后逐目标判定
	if err := bashRmTargetGuard(expanded); err != nil {
		return toolobs.ReasonRmHome, err
	}
	// 解码执行链: base64 -d 后管道到 sh/bash 或 eval（绕过文字黑名单）
	lower := strings.ToLower(command)
	if strings.Contains(lower, "base64") && strings.Contains(lower, "-d") {
		if strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash") || strings.Contains(lower, "eval") || strings.Contains(lower, "`") || strings.Contains(lower, "$(") {
			return toolobs.ReasonObfuscatedCmd, fmt.Errorf("命令含解码后执行链（base64 -d → shell/eval）——高风险绕过——拒绝。建议: 直接写明文命令")
		}
	}
	if strings.Contains(lower, "eval") && (strings.Contains(lower, "$(") || strings.Contains(lower, "`") || strings.Contains(lower, "base64")) {
		return toolobs.ReasonObfuscatedCmd, fmt.Errorf("命令含 eval 动态执行（$()/反引号/base64）——高风险——拒绝。建议: 直接写明文命令")
	}
	return "", nil
}

// bashRmTargetGuard — rm 参数级目标防护（2026-09-07 家目录清空事故修复）
// 事故根因: checkDangerousCommand 的 endOnly 黑名单只拦 "rm -rf /" 与 "rm -rf ~" 字面量,
//
//	"rm -rf ~/Desktop ..."、"$HOME/..."、"~/..." 全部漏过 → 整屋个人文件被 rm
//
// 本守卫处理已展开(~/、$HOME→真实路径)的命令: 逐个解析 rm 目标,
//
//	目标 ∈ { /, 家目录本身, 家目录祖先(如 /Users), 家目录内任意路径 } → 拒绝
//	目标为 /tmp/xxx、工作区路径等安全路径 → 放行(不误伤 endOnly 设计意图)
//
// 局限: 不做 bash 完整语法解析——覆盖 CA 常用形态(空格分隔、引号、;&&||| 复合);
//
//	反引号/$() 动态拼目标已被 bashExpandDangerScan 的解码执行链检查拦截
func bashRmTargetGuard(expanded string) error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil // 拿不到家目录就退回原黑名单防线
	}
	// 按命令分隔符切段(每段独立判定; "x&&rm -rf /tmp" 这种紧贴形态也能拆出 rm 段)
	segs := regexp.MustCompile(`[;&|]+`).Split(expanded, -1)
	for _, seg := range segs {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		// 定位 rm 及其目标: 逐 token 找 "rm"(或绝对路径形态), 之后跳过旗标收集目标
		for i := 0; i < len(fields); i++ {
			cmd := strings.Trim(fields[i], "'\"")
			if cmd != "rm" && !strings.HasSuffix(cmd, "/rm") {
				continue
			}
			for j := i + 1; j < len(fields); j++ {
				tok := fields[j]
				if strings.HasPrefix(tok, "-") {
					continue // rm -r/-f/-rf/--xxx 旗标
				}
				target := strings.Trim(tok, "'\"")
				if target == "/" || target == "//" {
					return fmt.Errorf("危险命令拦截: rm 目标 %q 命中家目录保护(2026-09-07 事故修复)——拒绝。家目录/根目录及其内部不可 rm——请用精确工作区路径或专用文件工具;误删主目录事故教训见 Zerg-内部文档/issues/rm-home-guard-fix-20260907.md（已分家）", target)
				}
				target = strings.TrimSuffix(target, "/")
				// 空目标/纯重定向等非路径 token 跳过(rm 无操作数会自行报错)
				if target == "" || target == ">" || target == ">>" || target == "<" {
					continue
				}
				if rmTargetDangerous(target, home) {
					return fmt.Errorf("危险命令拦截: rm 目标 %q 命中家目录保护(2026-09-07 事故修复)——拒绝。家目录/根目录及其内部不可 rm——请用精确工作区路径或专用文件工具;误删主目录事故教训见 Zerg-内部文档/issues/rm-home-guard-fix-20260907.md（已分家）", target)
				}
			}
		}
	}
	return nil
}

// rmTargetDangerous — 目标是否落在受保护范围( / 、家目录本身/祖先/内部 )
func rmTargetDangerous(target, home string) bool {
	if target == "/" {
		return true
	}
	// 目标==家目录, 或目标是家目录祖先(/Users、~ 的上级), 或目标在家目录内
	if target == home || strings.HasPrefix(home, target+"/") || strings.HasPrefix(target, home+"/") {
		return true
	}
	return false
}

// bashRmScopeGate — v1.0.2 删除范围门控(2026-09-07 Mr2109确认——语义级根治)
// 规则: rm 每个目标(展开 ~/$HOME 后, 相对路径以执行 cwd 为基) Clean 解析后,
// 必须落在允许根集合 { 工作区, /tmp, ExtraAllowDirs(任务/白名单) } 内;
// 任一目标出域 → 整条拒绝+引导。与路径拼写无关(~/、$HOME、绝对路径一律按解析结果判定);
// /etc、外部归档卷上工作区之外等一切域外路径从此不可 rm。
func bashRmScopeGate(command, execCwd string, allowRoots []string) error {
	expanded := bashExpandHomeOnly(command)
	roots := make([]string, 0, len(allowRoots)+1)
	for _, r := range allowRoots {
		if a, err := filepath.Abs(r); err == nil && a != "" {
			roots = append(roots, a)
		}
	}
	// 决策(Mr2109 2026-09-07): /tmp 全量在允许域——endOnly"精确路径合法"延续, 现有用例不回归
	roots = append(roots, "/tmp")
	// 与家目录守卫同解析: 按 ;&&||| 切段 → 定位 rm → 跳过旗标 → 收集目标
	segs := regexp.MustCompile(`[;&|]+`).Split(expanded, -1)
	for _, seg := range segs {
		fields := strings.Fields(seg)
		for i := 0; i < len(fields); i++ {
			cmd := strings.Trim(fields[i], "'\"")
			if cmd != "rm" && !strings.HasSuffix(cmd, "/rm") {
				continue
			}
			for j := i + 1; j < len(fields); j++ {
				tok := fields[j]
				if strings.HasPrefix(tok, "-") {
					continue
				}
				target := strings.Trim(tok, "'\"")
				if target == "" || target == ">" || target == ">>" || target == "<" {
					continue
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(execCwd, target)
				}
				cleaned := filepath.Clean(target)
				if !rmInAllowedRoot(cleaned, roots) {
					return fmt.Errorf("危险命令拦截: rm 目标 %q 不在允许删除域(工作区//tmp/白名单)——v1.0.2 删除范围门控。请改工作区内路径或用专用文件工具;确需他处删除请加白名单或人工执行(设计: docs/01-设计/设计-bash-工具-删除范围门控-v1.0.2.md)", target)
				}
			}
		}
	}
	return nil
}

// rmInAllowedRoot — Clean 后的目标是否 == 允许根 或 落在根内
func rmInAllowedRoot(cleaned string, roots []string) bool {
	for _, r := range roots {
		if cleaned == r || strings.HasPrefix(cleaned, r+"/") {
			return true
		}
	}
	return false
}

// executeBashV101 — bash v1.0.1 主执行（替换 v1.0.0 executeBash）
// cwd: 可选工作目录（空=ec.WorkDir；验证在允许根内）
// timeoutS: 可选超时秒（<=0=ec.Timeout）
func (ec *ExecContext) executeBashV101(ctx context.Context, command string, cwd string, timeoutS int, gate ToolGater) (string, error) {
	start := time.Now()

	// 1. 危险命令双检（原文 + 展开形态）——T1.2：形态判定带原因码（**只用于观测**，拦截判定不变）
	if reason, err := bashExpandDangerScanCoded(command); err != nil {
		if reason == "" {
			reason = toolobs.ReasonDangerousCmd // 兜底：原因码不得缺席（空原因=无法归因）
		}
		ec.obsDeny(reason)
		return "", err
	}
	// 2. Poka-yoke 防呆形态（结构性拦截）
	if err := bashPokaYokeCheck(command); err != nil {
		ec.obsDeny(toolobs.ReasonBashPokaYoke) // T1.2 观测：结构性防呆拦截 = 拒绝分支
		return "", err
	}
	// 3. Gate 检查
	if gate != nil {
		decision, err := gate.Check("bash", command, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		switch decision.Action {
		case "block":
			return "", fmt.Errorf("命令被 gate 拦截: %s — %s", command, decision.Message)
		case "require_approval":
			// T1.2 观测：需审批且无审批通道 ⇒ 本轮不执行（拒绝类判定；block 已由 ExecuteTool 的 obsGater 记）
			ec.obsDeny(toolobs.ReasonGateApproval)
			return "", fmt.Errorf("命令需要审批: %s — %s", command, decision.Message)
		}
	}

	// 4. 工作目录解析（cwd 参数——v1.0.1 新——防跨命令路径错位）
	// bash 执行沙盒=WorkDir 内（ExtraAllowDirs 只放行 read/write 工具——不得成为 bash 执行区）
	// v1.0.1 收尾(2026-09-07): 相对 cwd 以工作区为根解析(沙盒语义)——勿用进程 cwd
	workDir := ec.WorkDir
	if cwd != "" {
		absWorkDir, _ := filepath.Abs(ec.WorkDir)
		absCwd := cwd
		if !filepath.IsAbs(absCwd) {
			absCwd = filepath.Join(absWorkDir, absCwd)
		}
		absCwd, err := filepath.Abs(absCwd) // Clean 归一（.. 逃逸会被前缀检查拦下）
		if err != nil {
			return "", fmt.Errorf("cwd 路径解析失败: %w", err)
		}
		if !(strings.HasPrefix(absCwd, absWorkDir+string(os.PathSeparator)) || absCwd == absWorkDir) {
			ec.obsDeny(toolobs.ReasonCwdOutside) // T1.2 观测：cwd 出域 = 拒绝分支
			return "", fmt.Errorf("cwd %q 不在工作区 %q 内——拒绝执行（bash 沙盒=工作区；任务目录/白名单目录用 read/write 访问）", cwd, ec.WorkDir)
		}
		if fi, err := os.Stat(absCwd); err != nil || !fi.IsDir() {
			return "", fmt.Errorf("[guide] cwd %q 不存在或不是目录——请先 ls 确认目录名", cwd)
		}
		workDir = absCwd
	}

	// v1.0.2 删除范围门控(2026-09-07 Mr2109确认): 工作目录解析后、执行前——rm 目标必须落
	// 在允许删除域(工作区//tmp/ExtraAllowDirs)——家目录守卫在 step1(bashExpandDangerScan)已先行
	if err := bashRmScopeGate(command, workDir, append([]string{ec.WorkDir}, ec.ExtraAllowDirs...)); err != nil {
		ec.obsDeny(toolobs.ReasonRmOutOfScope) // T1.2 观测：rm 目标出允许删除域 = 拒绝分支
		return "", err
	}

	// 5. 超时（timeout_s 参数覆盖默认）
	timeout := ec.Timeout
	if timeoutS > 0 {
		timeout = time.Duration(timeoutS) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 6. 执行（RTK 包装可选——ls/git 等紧凑输出）
	// 2026-09-11 B 批（可移植性）：rtk 是可选依赖，缺装时**绝不能**让所有命令失败
	// （原实现无条件加 "rtk " 前缀 → 别人机器上 ls/git/go… 全部 command not found）。
	// 开关：ZERG_RTK=0 强制关闭；默认仅当 PATH 中真有 rtk 时才包装。
	rtkCommand := command
	if os.Getenv("ZERG_RTK") != "0" {
		if _, err := exec.LookPath("rtk"); err == nil {
			fields := strings.Fields(command)
			if len(fields) > 0 {
				switch fields[0] {
				case "ls", "git", "find", "ps", "df", "du", "pip", "npm", "go", "cargo", "docker":
					rtkCommand = "rtk " + command
				}
			}
		}
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", rtkCommand)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	durMs := time.Since(start).Milliseconds()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// 7. 超时/信号判定（ctx 超时——命令被杀）
	timedOut := ctx.Err() == context.DeadlineExceeded
	if timedOut {
		exitCode = -1
	}

	// 8. 输出管理: 二进制检测 → 溢出落盘 → 窗口化
	stdoutData := stdout.Bytes()
	stderrText := stderr.String()
	totalBytes := len(stdoutData) + len(stderrText)

	var body string
	if bashIsBinary(stdoutData) {
		path := spillBashOutput(stdoutData, "binary")
		body = fmt.Sprintf("[binary_output] stdout 含二进制/不可文本内容（%d 字节）——已存 %s——用 file/xxd 检查类型，勿直接文本化", len(stdoutData), path)
	} else {
		var shown string
		shown, _, _ = bashWindow(stdoutData)
		body = shown // 溢出省略标注内已含落盘路径
	}

	// 9. 组装返回（首行断言 + stdout + stderr 段 + 元数据）
	var sb strings.Builder
	if exitCode != 0 {
		// 系统断言: 成败事实前置（弱模型不读细节——首行必须见状态）
		fmt.Fprintf(&sb, "⚠️ exit %d — 命令失败\n", exitCode)
		if timedOut {
			sb.WriteString(fmt.Sprintf("⏱️ timeout（超过 %d 秒被终止）\n", int(timeout.Seconds())))
		}
		if guide := bashClassifyError(stderrText, command); guide != "" {
			sb.WriteString(guide + "\n")
		}
		// 失败时 stdout 前 1K 仍有诊断价值（命令自身输出）
		if len(body) > 0 && len(body) <= bashOutLimit {
			sb.WriteString("\n" + body)
		} else if len(body) > 0 {
			// 失败+超长——截 stdout 头 1K
			sb.WriteString("\n" + body[:minInt(len(body), 1000)])
		}
	} else {
		if len(body) > 0 {
			sb.WriteString(body)
		}
		// stderr 非空但 exit=0 → note 提示（忽略失败信号补偿）
		if stderrText != "" {
			sb.WriteString("\n\n[stderr]\n" + truncate(stderrText, 2000))
			sb.WriteString("\n\n[note] stderr 非空但 exit=0——若属意外请检查（命令可能部分失败）")
		}
		// state_note: cd/export 提示
		if note := bashStateNote(command); note != "" && exitCode == 0 {
			sb.WriteString("\n\n" + note)
		}
	}

	// 10. 元数据尾注
	sb.WriteString(bashMetaFooter(exitCode, workDir, durMs, totalBytes, ""))

	// stderr 独立段（exit≠0 且 stderr 非空——失败场景 stderr 是主诊断）
	if exitCode != 0 && stderrText != "" {
		sb.WriteString("\n\n[stderr]\n" + truncate(stderrText, 3000))
	}
	return sb.String(), nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// jsonNumber — 解析 args 中的数字（json.Number 兼容）
func jsonNumber(v any) (int, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.Atoi(n.String())
		return i, err == nil
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

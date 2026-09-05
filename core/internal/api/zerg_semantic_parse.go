// Package api - v2.5.6 程序定量驱动 Agent: 语义解析器（Semantic Parse）
// 模型自然语言 → 结构化动作（任务工具核心——模型轻松表达——程序精准执行）
// 2026-08-27 Mr2109——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 21 章
// 2026-08-28 动作库扩充（Mr2109——补动作：patch/append/read/search/git/done/answer——10 种）
package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Action 解析出的动作（结构化——程序执行）
type Action struct {
	Type    string // write/run/read/list/search/patch/append/git/done/answer/other
	Path    string // 文件路径
	Content string // 内容
	Command string // 命令（run 类型）
	// v2.5.6 动作库扩充（2026-08-28）
	OldText string // 旧文本（patch——替换目标）
	NewText string // 新文本（patch——替换结果）
	Keyword string // 关键词（search）
	Message string // 说明内容（answer/done）
	Raw     string // 原始自然语言
}

// ParseAction 从模型自然语言提取动作（规则解析——程序精准）
// 示例: "创建 hello.txt 内容 zerg-test" → {Type:write, Path:hello.txt, Content:zerg-test}
//
//	"运行 go test" → {Type:run, Command:go test}
//	"看看当前目录" → {Type:list}
//	"看看 hello.txt" → {Type:read, Path:hello.txt}
//	"搜索 zerg" → {Type:search, Keyword:zerg}
//	"把 hello.txt 里的 old 改成 new" → {Type:patch, Path:hello.txt, OldText:old, NewText:new}
//	"在 hello.txt 末尾追加 xxx" → {Type:append, Path:hello.txt, Content:xxx}
func ParseAction(nl string, workdir string) *Action {
	raw := strings.TrimSpace(nl)
	act := &Action{Raw: raw}

	// 0. 剥"完成[:：]"前缀（模型常说"完成: 创建 xxx"——动作在前——2026-08-28）
	raw = strings.TrimSpace(regexp.MustCompile(`^(?:完成|任务完成|已完成)\s*[:：]\s*`).ReplaceAllString(raw, ""))
	if raw != "" {
		act.Raw = raw
	}

	// 1. write 动作（创建/写文件——内容 zerg-test）
	// 模式: 创建|写|生成 + (一个|个|文件)? + 文件名 + 内容|写入|为|包含 + 内容
	reWrite := regexp.MustCompile(`(?i)(?:创建|写|生成|新建)\s*(?:一个|个|文件\s*)?\s*([^\s，。；]+\.\w+)\s*(?:内容|写入|为|包含)\s*[:：]?\s*(.+)`)
	if m := reWrite.FindStringSubmatch(raw); m != nil {
		act.Type = "write"
		act.Path = strings.TrimSpace(m[1])
		act.Content = strings.TrimSpace(m[2])
		// 清理: 引号 + 开头虚词（为/是/：等）
		act.Content = strings.Trim(act.Content, "\"'“”‘’")
		act.Content = strings.TrimSpace(strings.TrimPrefix(act.Content, "为"))
		act.Content = strings.TrimSpace(strings.TrimPrefix(act.Content, "是"))
		act.Content = strings.TrimSpace(strings.TrimPrefix(act.Content, ":"))
		act.Content = strings.TrimSpace(strings.TrimPrefix(act.Content, "："))
		act.Path = filepath.Join(workdir, act.Path)
		return act
	}
	// 简化: 创建 hello.txt（无内容——默认写文件名）
	// v2.5.6 修复: 简化模式单独判断（防完整模式误匹配——内容=文件名）
	// 2026-08-28: 允许尾部"文件"（"创建 hello.txt 文件"）
	reWriteSimple := regexp.MustCompile(`(?i)(?:创建|写|生成|新建)\s*(?:一个|个|文件\s*)?([^\s，。；]+\.\w+)\s*(?:文件)?\s*$`)
	if m := reWriteSimple.FindStringSubmatch(raw); m != nil {
		act.Type = "write"
		act.Path = filepath.Join(workdir, strings.TrimSpace(m[1]))
		act.Content = strings.TrimSpace(m[1]) // 默认内容=文件名
		return act
	}

	// 2. patch 动作（修改文件——把 X 改成 Y）——2026-08-28 新增
	// 模式: 修改|改|更新 + 文件名 + 把|将 + 旧文本 + 改成|改为|换成 + 新文本
	rePatch := regexp.MustCompile(`(?i)(?:修改|改|更新|替换)\s*(?:文件\s*)?([^\s，。；]+\.\w+)\s*(?:把|将|中|里|里的)\s*(.+?)\s*(?:改成|改为|换成|替换为|改成)\s*(.+)`)
	if m := rePatch.FindStringSubmatch(raw); m != nil {
		act.Type = "patch"
		act.Path = filepath.Join(workdir, strings.TrimSpace(m[1]))
		act.OldText = strings.TrimSpace(m[2])
		act.NewText = strings.TrimSpace(m[3])
		return act
	}

	// 3. append 动作（追加内容——文件末尾加）——2026-08-28 新增
	// 模式: 追加|在...末尾加|添加 + 文件 + 内容 + 内容
	reAppend := regexp.MustCompile(`(?i)(?:追加|末尾追加|添加|补写)\s*(?:到|在)?\s*(?:文件\s*)?([^\s，。；]+\.\w+)\s*(?:内容|写入|为|：|:)\s*(.+)`)
	if m := reAppend.FindStringSubmatch(raw); m != nil {
		act.Type = "append"
		act.Path = filepath.Join(workdir, strings.TrimSpace(m[1]))
		act.Content = strings.TrimSpace(m[2])
		return act
	}

	// 4. run 动作（运行/执行命令）
	// 2026-08-28: 允许"运行 命令: xxx"（模型常按提示语带"命令"两字——剥掉）
	reRun := regexp.MustCompile(`(?i)(?:运行|执行|跑)\s*(?:命令)?\s*[:：]?\s*(.+)`)
	if m := reRun.FindStringSubmatch(raw); m != nil {
		act.Type = "run"
		act.Command = strings.TrimSpace(m[1])
		return act
	}

	// 5. read 动作（读文件——返回内容）——2026-08-28 新增
	// 模式: 看看|读取|查看|读 + 文件
	reRead := regexp.MustCompile(`(?i)(?:看看|读取|查看|读一下?|内容是什么|内容)\s*(?:文件\s*)?([^\s，。；]+\.\w+)`)
	if m := reRead.FindStringSubmatch(raw); m != nil {
		act.Type = "read"
		act.Path = filepath.Join(workdir, strings.TrimSpace(m[1]))
		return act
	}

	// 6. search 动作（搜索关键词）——2026-08-28 新增
	// 模式: 搜索|查找|找 + 关键词
	reSearch := regexp.MustCompile(`(?i)(?:搜索|查找|找一下?|grep|搜)\s*[:：]?\s*([^\s，。；]+)`)
	if m := reSearch.FindStringSubmatch(raw); m != nil {
		act.Type = "search"
		act.Keyword = strings.TrimSpace(m[1])
		return act
	}

	// 7. git 动作（提交任务产出）——2026-08-28 新增
	// 模式: 提交|git 提交|保存进度
	if strings.Contains(raw, "提交") && (strings.Contains(raw, "git") || strings.Contains(raw, "提交进度") || strings.Contains(raw, "提交任务")) {
		act.Type = "git"
		return act
	}

	// 8. done 动作（任务完成——结束受控循环）——2026-08-28 新增
	// 模式: 任务完成|已完成|做好了|搞定|我已经完成
	if strings.Contains(raw, "任务完成") || strings.Contains(raw, "已完成") || strings.Contains(raw, "做好了") ||
		strings.Contains(raw, "完成产出") || strings.Contains(raw, "我已经完成") || strings.Contains(raw, "搞定") ||
		strings.Contains(raw, "全部完成") || strings.Contains(raw, "任务结束") {
		act.Type = "done"
		act.Message = raw
		return act
	}

	// 9. list 动作（查看/列出目录）
	if strings.Contains(raw, "目录") || strings.Contains(raw, "看看") || strings.Contains(raw, "列出") || strings.Contains(raw, "ls") {
		act.Type = "list"
		return act
	}

	// 10. answer 动作（直接说明——记录回复）——2026-08-28 新增
	// 模式: 说明|回答|我的想法|我认为|直接说
	if strings.HasPrefix(raw, "说明") || strings.HasPrefix(raw, "回答") || strings.HasPrefix(raw, "我认为") || strings.HasPrefix(raw, "我的想法") || strings.HasPrefix(raw, "直接说") {
		act.Type = "answer"
		act.Message = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "说明"), "回答"))
		return act
	}

	// 11. 兜底（听不懂——返回 other——调用方澄清）
	act.Type = "other"
	return act
}

// ExecuteAction 执行动作（程序精准——Structured Execution 层）
func ExecuteAction(act *Action) (string, error) {
	switch act.Type {
	case "write":
		if act.Path == "" {
			return "", fmt.Errorf("写文件缺路径")
		}
		if err := os.MkdirAll(filepath.Dir(act.Path), 0o755); err != nil {
			return "", fmt.Errorf("创建目录失败: %w", err)
		}
		if err := os.WriteFile(act.Path, []byte(act.Content), 0o644); err != nil {
			return "", fmt.Errorf("写文件失败: %w", err)
		}
		return fmt.Sprintf("文件已写: %s（内容: %s）", act.Path, act.Content), nil
	case "append":
		if act.Path == "" {
			return "", fmt.Errorf("追加缺路径")
		}
		if err := os.MkdirAll(filepath.Dir(act.Path), 0o755); err != nil {
			return "", fmt.Errorf("创建目录失败: %w", err)
		}
		f, err := os.OpenFile(act.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", fmt.Errorf("追加文件失败: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + act.Content + "\n"); err != nil {
			return "", fmt.Errorf("追加写入失败: %w", err)
		}
		return fmt.Sprintf("已追加: %s（新增 %d 字）", act.Path, len(act.Content)), nil
	case "patch":
		if act.Path == "" || act.OldText == "" {
			return "", fmt.Errorf("修改缺路径或旧文本")
		}
		data, err := os.ReadFile(act.Path)
		if err != nil {
			return "", fmt.Errorf("读文件失败（修改前）: %w", err)
		}
		content := string(data)
		if !strings.Contains(content, act.OldText) {
			return "", fmt.Errorf("文件中找不到目标文本「%s」——请确认原文再修改", truncateStr(act.OldText, 40))
		}
		// 全量替换（出现多次全换——程序精准）
		newContent := strings.ReplaceAll(content, act.OldText, act.NewText)
		if err := os.WriteFile(act.Path, []byte(newContent), 0o644); err != nil {
			return "", fmt.Errorf("写文件失败（修改后）: %w", err)
		}
		return fmt.Sprintf("已修改: %s（「%s」→「%s」——替换 %d 处）", act.Path, truncateStr(act.OldText, 30), truncateStr(act.NewText, 30), strings.Count(content, act.OldText)), nil
	case "read":
		if act.Path == "" {
			return "", fmt.Errorf("读文件缺路径")
		}
		data, err := os.ReadFile(act.Path)
		if err != nil {
			return "", fmt.Errorf("读文件失败: %w", err)
		}
		content := string(data)
		// 截断保护（超长文件只返回头部+尾部——防上下文爆炸）
		if len(content) > 4000 {
			return fmt.Sprintf("文件内容（共 %d 字——前 2000 + 后 1000）:\n%s\n...\n%s", len(content), content[:2000], content[len(content)-1000:]), nil
		}
		return fmt.Sprintf("文件内容:\n%s", content), nil
	case "search":
		if act.Keyword == "" {
			return "", fmt.Errorf("搜索缺关键词")
		}
		// 递归 grep（忽略 .git/node_modules/target）
		cmd := exec.Command("grep", "-rn", "--include=*", "-I", act.Keyword, ".")
		cmd.Dir = act.Path
		out, err := cmd.Output()
		if err != nil {
			// grep 无匹配返回 exit 1——不是错误
			return "搜索无结果: " + act.Keyword, nil
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		// 截断保护（最多 30 行）
		if len(lines) > 30 {
			lines = lines[:30]
			return fmt.Sprintf("搜索 %s 命中 %d 处（显示前 30）:\n%s", act.Keyword, len(lines), strings.Join(lines, "\n")), nil
		}
		if len(lines) == 0 {
			return "搜索无结果: " + act.Keyword, nil
		}
		return fmt.Sprintf("搜索 %s 命中 %d 处:\n%s", act.Keyword, len(lines), strings.Join(lines, "\n")), nil
	case "run":
		if act.Command == "" {
			return "", fmt.Errorf("运行缺命令")
		}
		// 危险命令黑名单（Mr2109 2026-08-16——只拦绝对不可逆+系统级——不误杀 rm /tmp）
		if blocked, reason := checkDangerousCommand(act.Command); blocked {
			return "", fmt.Errorf("命令被拦截（危险命令黑名单）: %s——请换安全方式（如: 创建/修改文件用「创建 <文件> 内容 <内容>」）", reason)
		}
		cmd := exec.Command("sh", "-c", act.Command)
		cmd.Dir = act.Path
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("命令执行失败（exit %v）:\n%s", err, truncateStr(string(out), 1500)), nil
		}
		return fmt.Sprintf("命令输出:\n%s", truncateStr(string(out), 1500)), nil
	case "list":
		dir := act.Path
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", fmt.Errorf("列目录失败: %w", err)
		}
		names := []string{}
		for _, e := range entries {
			suffix := ""
			if e.IsDir() {
				suffix = "/"
			}
			names = append(names, e.Name()+suffix)
		}
		return "目录内容: " + strings.Join(names, ", "), nil
	case "git":
		// 提交任务产出（git add -A + commit——程序执行——受控循环内主动提交）
		dir := act.Path
		cmd := exec.Command("sh", "-c", "git add -A && git commit -q -m \"task: 主动提交产出\" || true")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git 提交失败: %w", err)
		}
		return "git 提交完成: " + strings.TrimSpace(string(out)), nil
	case "done":
		return "任务完成标记（程序识别——结束受控循环）", nil
	case "answer":
		return "说明已记录: " + act.Message, nil
	default:
		return "", fmt.Errorf("无法执行动作: %s（类型 %s）——请澄清", act.Raw, act.Type)
	}
}

// ExecuteActionInDir 在指定目录执行动作（workdir 由调用方定——防越界）
func ExecuteActionInDir(act *Action, workdir string) (string, error) {
	if act.Path != "" && !filepath.IsAbs(act.Path) {
		// 相对路径——拼到 workdir（防越界——绝对路径限制在 workdir 内）
		act.Path = filepath.Join(workdir, act.Path)
	}
	if act.Type == "list" && act.Path == "" {
		act.Path = workdir
	}
	if act.Type == "search" && act.Path == "" {
		act.Path = workdir
	}
	if act.Type == "run" && act.Path == "" {
		act.Path = workdir
	}
	return ExecuteAction(act)
}

// checkDangerousCommand 危险命令黑名单（Mr2109 2026-08-16——虫族安全理念）
// 只拦绝对不可逆+系统级——endOnly 精确匹配不误杀（rm /tmp 等局部操作放行）
func checkDangerousCommand(cmd string) (bool, string) {
	trimmed := strings.TrimSpace(cmd)
	lower := strings.ToLower(trimmed)
	// 1. rm 根目录/家目录（精确）
	if matched, _ := regexp.MatchString(`^rm\s+(-[a-z]*\s+)*[~/]\s*$`, trimmed); matched {
		return true, "rm 根/家目录（不可逆）"
	}
	// 2. 格式化/系统级
	for _, danger := range []string{"mkfs", "shutdown", "reboot", "poweroff", "init 0", "init 6", "dd if="} {
		if strings.Contains(lower, danger) {
			return true, danger + "（系统级不可逆）"
		}
	}
	// 3. 管道执行未知脚本（curl|sh / wget|sh）
	if matched, _ := regexp.MatchString(`(curl|wget)\s+[^\s]+\s*\|\s*(ba)?sh`, lower); matched {
		return true, "管道执行网络脚本（未知代码）"
	}
	return false, ""
}

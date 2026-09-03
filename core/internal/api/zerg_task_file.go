// Package api - v2.5.6 程序定量驱动 Agent: task.jsonl 数据层
// Zerg-FS: 单文件 JSONL——包罗一切——每轮一行——git diff 友好
// 2026-08-25 Mr2109——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md
package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ZergTaskLine task.jsonl 的一行（一轮的数据——包罗一切）
type ZergTaskLine struct {
	Round   int                    `json:"round"`             // 轮次（1 起）
	Phase   string                 `json:"phase"`             // plan/select/execute/close
	Step    string                 `json:"step,omitempty"`    // 当前小任务（execute 阶段）
	Output  map[string]interface{} `json:"output,omitempty"`  // 本轮产出（结构化——JSON）
	Summary string                 `json:"summary,omitempty"` // 本轮总结（一句话——下轮记忆输入）
	Time    string                 `json:"time"`              // 时间戳
}

// ZergTaskFile 任务数据文件（task.jsonl——单文件）
// 位置: <任务目录>/task.jsonl
type ZergTaskFile struct {
	Path string
}

// NewZergTaskFile 创建/打开任务数据文件
func NewZergTaskFile(taskDir string) *ZergTaskFile {
	return &ZergTaskFile{Path: filepath.Join(taskDir, "task.jsonl")}
}

// Init 初始化任务文件（首行——任务元数据）
func (z *ZergTaskFile) Init(taskID, taskType, objective string) error {
	if err := os.MkdirAll(filepath.Dir(z.Path), 0o755); err != nil {
		return fmt.Errorf("创建任务目录失败: %w", err)
	}
	// 已存在（重跑/恢复）——不覆盖
	if _, err := os.Stat(z.Path); err == nil {
		return nil
	}
	first := ZergTaskLine{
		Round: 0,
		Phase: "init",
		Output: map[string]interface{}{
			"task_id":   taskID,
			"type":      taskType,
			"objective": objective,
		},
		Summary: "任务建立",
		Time:    time.Now().Format(time.RFC3339),
	}
	return z.Append(first)
}

// Append 追加一行（每轮结束——程序写入）
func (z *ZergTaskFile) Append(line ZergTaskLine) error {
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	f, err := os.OpenFile(z.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开失败: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	return nil
}

// ReadAll 读全部行（程序用——历史完整）
func (z *ZergTaskFile) ReadAll() ([]ZergTaskLine, error) {
	data, err := os.ReadFile(z.Path)
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	var lines []ZergTaskLine
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var l ZergTaskLine
		if err := json.Unmarshal(line, &l); err != nil {
			continue // 跳过坏行（不阻塞）
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// Last 读最后一行（当前状态——程序/模型看摘要）
func (z *ZergTaskFile) Last() (*ZergTaskLine, error) {
	lines, err := z.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	return &lines[len(lines)-1], nil
}

// Summary 状态摘要（给模型的——最后几行总结——记忆有界）
// 历史看摘要——当前看全文——出问题重做（Mr2109核心）
func (z *ZergTaskFile) Summary(n int) (string, error) {
	lines, err := z.ReadAll()
	if err != nil {
		return "", err
	}
	if n <= 0 {
		n = 5 // 默认最近 5 行
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := "【任务状态】（最近 " + fmt.Sprintf("%d", len(lines)) + " 轮）:\n"
	for _, l := range lines {
		out += fmt.Sprintf("- r%d [%s] %s\n", l.Round, l.Phase, l.Summary)
	}
	return out, nil
}

// splitLines 按行分割（保持字节——不丢尾部）
func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				out = append(out, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}

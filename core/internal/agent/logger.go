package agent

// logger.go - 虫族 v2.5 日志系统（全面日志：事件/错误/审计/轨迹）
// 设计：
//   - append-only JSONL（每行一个完整 JSON，可逐行解析）
//   - 线程安全（互斥锁，Agent 可能并行调用）
//   - 存储到 .zerg/logs/{taskID}/ 目录
//   - 事件日志 events.jsonl：每工具调用/每轮循环/每模型调用
//   - 错误日志 errors.jsonl：错误类型/消息/上下文/时间戳
//   - 审计日志 audit.jsonl：工具名+完整参数+触发prompt+结果+时间戳
//   - transcript.jsonl：完整会话 transcript（M4 蒸馏数据源）

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel - 日志级别
type LogLevel string

const (
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelAudit LogLevel = "audit"
	LevelTrace LogLevel = "trace"
)

// EventType - 事件类型枚举
type EventType string

const (
	EventToolCall    EventType = "tool_call"
	EventToolResult  EventType = "tool_result"
	EventToolDenied  EventType = "tool_denied"
	EventToolError   EventType = "tool_error"
	EventModelCall   EventType = "model_call"
	EventModelResult EventType = "model_result"
	EventLoopStart   EventType = "loop_start"
	EventLoopEnd     EventType = "loop_end"
	EventPrompt      EventType = "prompt"
	EventAbort       EventType = "abort"
	EventCheckpoint  EventType = "checkpoint"
)

// EventEntry - events.jsonl 每行一条
type EventEntry struct {
	Seq       int64     
	TaskID    string    
	Type      string    
	Level     string    
	Action    string    
	ToolName  string    
	Prompt    string    
	Args      any       
	Result    string    
	Error     string    
	Step      int       
	Duration  string    
	Timestamp time.Time 
}

// AuditEntry - audit.jsonl 每行一条（M3 安全对齐）
type AuditEntry struct {
	TaskID    string    
	ToolName  string    
	Args      any       
	Prompt    string    
	Decision  string    
	Result    string    
	Error     string    
	Agent     string    
	Duration  string    
	Timestamp time.Time 
}

// TranscriptEntry - transcript.jsonl 每行一条（M4 蒸馏数据源）
type TranscriptEntry struct {
	TaskID     string    
	Step       int       
	Role       string    
	Content    string    
	ToolName   string    
	ToolCallID string    
	Args       any       
	Result     string    
	Error      string    
	Duration   string    
	Timestamp  time.Time 
}

// Logger - 虫族日志系统
// 线程安全（内部互斥锁），append-only（文件以 O_APPEND 打开）
type Logger struct {
	mu         sync.Mutex
	taskID     string
	dir        string
	events     *os.File
	errors     *os.File
	audit      *os.File
	transcript *os.File
	seq        int64
	closed     bool
}

// NewLogger - 创建日志系统（创建目录 + 4 个文件）
func NewLogger(taskID, dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	l := &Logger{taskID: taskID, dir: dir}
	var err error
	openFn := func(name string) (*os.File, error) {
		return os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	}
	if l.events, err = openFn("events.jsonl"); err != nil {
		return nil, err
	}
	if l.errors, err = openFn("errors.jsonl"); err != nil {
		return nil, err
	}
	if l.audit, err = openFn("audit.jsonl"); err != nil {
		return nil, err
	}
	if l.transcript, err = openFn("transcript.jsonl"); err != nil {
		return nil, err
	}
	return l, nil
}

// writeLine - 写一行 JSON（线程安全 + append）
func (l *Logger) writeLine(f *os.File, v any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.seq++
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	f.Write(append(data, '\n'))
}

// LogEvent - 事件日志（每工具调用/每轮循环/每模型调用）
func (l *Logger) LogEvent(etype, level, action, toolName, prompt string, args any, result, errStr, duration string) {
	l.writeLine(l.events, EventEntry{
		Seq: l.seq, TaskID: l.taskID, Type: etype, Level: level,
		Action: action, ToolName: toolName, Prompt: prompt,
		Args: args, Result: result, Error: errStr,
		Duration: duration, Timestamp: time.Now(),
	})
}

// LogError - 错误日志（带上下文——可复现）
func (l *Logger) LogError(errType, msg, context string) {
	l.writeLine(l.errors, map[string]any{
		"task_id":   l.taskID,
		"type":      errType,
		"message":   msg,
		"context":   context,
		"timestamp": time.Now(),
	})
}

// LogAudit - 审计日志（M3 安全对齐——工具名+参数+触发 prompt+结果）
func (l *Logger) LogAudit(toolName string, args any, prompt, decision, result, errStr, agent string) {
	l.writeLine(l.audit, AuditEntry{
		TaskID: l.taskID, ToolName: toolName, Args: args, Prompt: prompt,
		Decision: decision, Result: result, Error: errStr,
		Agent: agent, Timestamp: time.Now(),
	})
}

// LogTranscript - 任务轨迹（M4 蒸馏数据源——完整会话）
func (l *Logger) LogTranscript(role, content, toolName, toolCallID string, args any, result, errStr, duration string) {
	l.writeLine(l.transcript, TranscriptEntry{
		TaskID: l.taskID, Step: int(l.seq), Role: role, Content: content,
		ToolName: toolName, ToolCallID: toolCallID, Args: args,
		Result: result, Error: errStr, Duration: duration, Timestamp: time.Now(),
	})
}

// Close - 关闭所有文件
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	for _, f := range []*os.File{l.events, l.errors, l.audit, l.transcript} {
		if f != nil {
			f.Close()
		}
	}
}

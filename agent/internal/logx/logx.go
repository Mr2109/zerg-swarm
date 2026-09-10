// Package logx — 虫族 v2 统一日志系统
//
// 设计原则：
//  1. 分层级别：DEBUG < INFO < WARN < ERROR（可配置，默认 INFO）
//  2. 结构化输出：时间 级别 [组件] 消息 key=value...
//  3. 双写：stdout（实时）+ 文件（持久化，可配路径）
//  4. 请求追踪：可带 request_id / machine / model 等字段
//
// 用法：
//
//	logx.Infof("heartbeat", "心跳上报成功", "machine", "mini1")
//	logx.Errorf("server", "请求失败", "error", err, "path", "/infer")
package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

// Logger 日志器（双写 stdout + 文件）
type Logger struct {
	mu    sync.Mutex
	out   io.Writer // stdout（或任意 writer）
	file  *os.File  // 文件（可选）
	level Level     // 最低输出级别
}

var (
	defaultLogger = &Logger{out: os.Stdout, level: INFO}
)

// SetLevel 设置最低日志级别
func SetLevel(l Level) { defaultLogger.mu.Lock(); defaultLogger.level = l; defaultLogger.mu.Unlock() }

// ParseLevel 从字符串解析级别（debug/info/warn/error）
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return DEBUG
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// SetFile 启用文件输出（路径不存在则创建目录）
func SetFile(path string) error {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if defaultLogger.file != nil {
		defaultLogger.file.Close()
		defaultLogger.file = nil
	}
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defaultLogger.file = f
	return nil
}

// Close 关闭文件句柄
func Close() {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if defaultLogger.file != nil {
		defaultLogger.file.Close()
		defaultLogger.file = nil
	}
}

// log 核心：写一条日志（format 是固定消息，kv 是 key=value 字段对）
func logf(l Level, component, msg string, kv ...interface{}) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if l < defaultLogger.level {
		return
	}
	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	// 组装 key=value 字段
	var fields strings.Builder
	for i := 0; i+1 < len(kv); i += 2 {
		if fields.Len() > 0 {
			fields.WriteString(" ")
		}
		fmt.Fprintf(&fields, "%v=%v", kv[i], kv[i+1])
	}
	line := fmt.Sprintf("%s %-5s [%-10s] %s %s\n", ts, levelNames[l], component, msg, fields.String())
	defaultLogger.out.Write([]byte(line))
	if defaultLogger.file != nil {
		defaultLogger.file.Write([]byte(line))
	}
}

// Debugf 调试级
func Debugf(component, format string, kv ...interface{}) { logf(DEBUG, component, format, kv...) }

// Infof 信息级
func Infof(component, format string, kv ...interface{}) { logf(INFO, component, format, kv...) }

// Warnf 警告级
func Warnf(component, format string, kv ...interface{}) { logf(WARN, component, format, kv...) }

// Errorf 错误级
func Errorf(component, format string, kv ...interface{}) { logf(ERROR, component, format, kv...) }

package main

// logger.go — 结构化日志系统（slog + lumberjack 轮转）。
// 检阅报告（docs/检阅报告-代码日志.md）L2 实施：
//   - log/slog 标准库：分级（DEBUG/INFO/WARN/ERROR）、JSON 结构化
//   - lumberjack：按大小轮转（50MB）、保留 5 份、7 天、gzip 压缩
//   - 环境变量：ZERG_LOG_LEVEL（debug/info/warn/error）、ZERG_LOG_STDOUT=1（同时输出终端）

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// setupLogger 初始化结构化日志系统。
// 返回 *slog.Logger（同时设为 slog 默认）。
func setupLogger(logDir string) *slog.Logger {
	// 1. 日志轮转配置（核心：防止日志无限增长）
	rotate := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "zerg-core.log"),
		MaxSize:    50,   // 单文件最大 50MB
		MaxBackups: 5,    // 保留 5 个旧文件（共 ~250MB）
		MaxAge:     7,    // 超过 7 天删除
		Compress:   true, // 旧日志 gzip 压缩（50MB → ~5MB）
		LocalTime:  true, // 本地时间命名
	}

	// 2. 同时输出到 stderr（开发时看终端，ZERG_LOG_STDOUT=1 开启）
	writers := []io.Writer{rotate}
	if os.Getenv("ZERG_LOG_STDOUT") == "1" {
		writers = append(writers, os.Stderr)
	}
	multiWriter := io.MultiWriter(writers...)

	// 3. 日志级别（ZERG_LOG_LEVEL 环境变量，默认 info）
	level := slog.LevelInfo
	if lvl := os.Getenv("ZERG_LOG_LEVEL"); lvl != "" {
		if err := level.UnmarshalText([]byte(lvl)); err != nil {
			level = slog.LevelInfo
		}
	}

	// 4. 创建 slog（JSON 结构化 + 源码位置）
	logger := slog.New(slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level:     level,
		AddSource: true, // 添加源码位置（文件:行号）
	}))
	slog.SetDefault(logger)

	// v2.5.6 修复（2026-08-29——t12 日志文件 data 格式根因）:
	// 标准库 log.Printf 默认写 stderr——启动命令重定向到同一文件 → 两个 writer 并发写 → 文件损坏
	// 统一: log 也指向 lumberjack（与 slog 同通道——单 writer——有序不串扰）
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags)
	return logger
}

// setupHeartbeatLogger 初始化心跳专用日志系统。
// 写入 /tmp/zerg-heartbeat.log，50MB 轮转，与主日志分离。
func setupHeartbeatLogger() *slog.Logger {
	rotate := &lumberjack.Logger{
		Filename:   "/tmp/zerg-heartbeat.log",
		MaxSize:    50,   // 单文件最大 50MB
		MaxBackups: 5,    // 保留 5 个旧文件
		MaxAge:     7,    // 超过 7 天删除
		Compress:   true, // 旧日志 gzip 压缩
		LocalTime:  true,
	}

	// 心跳日志不输出到终端，避免刷屏
	logger := slog.New(slog.NewJSONHandler(rotate, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}))
	return logger
}

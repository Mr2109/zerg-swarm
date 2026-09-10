#!/bin/bash
# B5 主控保活 watchdog——检测 zerg-core 崩溃自动拉起 + 记录退出原因
#
# 2026-09-11 现状：**主控已由 launchd 托管**（~/Library/LaunchAgents/com.zerg.core.plist，KeepAlive=true，
# 掉线 ~3s 自动拉起——已验证）。本脚本保留为**未装 launchd 时的兜底**；它现在通过
# scripts/start-zerg-core.sh 拉起（与托管路径同一套环境），因此与 launchd 同时存在也不会冲突。
# 用法: nohup bash scripts/watchdog.sh > /tmp/zerg-watchdog.log 2>&1 &
# 检测逻辑：每 10s 查 zerg-core 进程；不在则记录退出时间+上次日志尾部，重启

BIN="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/bin/zerg-core"
LOG="/tmp/zerg-core.log"
WD_LOG="/tmp/zerg-watchdog.log"
CHECK_INTERVAL=10

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$WD_LOG"; }

log "🐛 watchdog 启动（检测间隔 ${CHECK_INTERVAL}s）"

while true; do
    PID=$(pgrep -f "zerg-core" | head -1)
    if [ -z "$PID" ]; then
        # 记录退出原因（日志尾部——panic/崩溃痕迹）
        EXIT_REASON=$(tail -5 "$LOG" 2>/dev/null | grep -iE "panic|fatal|SIGSEGV|goroutine" | head -1)
        if [ -n "$EXIT_REASON" ]; then
            log "⚠️ 检测到异常退出: $EXIT_REASON"
        else
            log "ℹ️ 主控不在运行（正常停止或无异常）"
        fi
        # 2026-09-11：统一走 scripts/start-zerg-core.sh（setsid 分离 + 源仓库根 .env），
        # 保证「看门狗拉起」与「手动启动」的环境完全一致（旧写法 cd /tmp 且不源 .env → 丢白名单/知识库路径）。
        log "🔄 拉起主控（scripts/start-zerg-core.sh）..."
        REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
        if bash "$REPO_ROOT/scripts/start-zerg-core.sh" >>"$WD_LOG" 2>&1; then
            log "✅ 主控已拉起 (PID $(pgrep -f 'bin/zerg-core' | head -1))"
        else
            log "❌ 拉起失败——见 $WD_LOG 与 $LOG 尾部"
        fi
    fi
    sleep "$CHECK_INTERVAL"
done

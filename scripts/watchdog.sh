#!/bin/bash
# B5 主控保活 watchdog——检测 zerg-core 崩溃自动拉起 + 记录退出原因
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
        log "🔄 拉起主控..."
        cd /tmp
        DYLD_LIBRARY_PATH=$HOME/lib "$BIN" > "$LOG" 2>&1 &
        NEW_PID=$!
        sleep 3
        if kill -0 "$NEW_PID" 2>/dev/null; then
            log "✅ 主控已拉起 (PID $NEW_PID)"
        else
            log "❌ 拉起失败！"
        fi
    fi
    sleep "$CHECK_INTERVAL"
done

#!/usr/bin/env bash
# start-watchdog.sh — 以「脱离会话」的方式启动主控保活看门狗（2026-09-11 新增）
#
# 2026-09-11 现状：主控已由 launchd 托管（com.zerg.core，KeepAlive）——本脚本仅在
# 不想用 launchd 时作为兜底使用。
#
# 看门狗是要长期常驻的循环进程，必须与 agent/终端会话解绑，否则会话回收即失效
# （实测：主控被会话带走时看门狗也没在跑，线上无人拉起 → UI 报"主控离线"）。
#
# 用法：bash scripts/build/start-watchdog.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WD_LOG="${ZERG_WATCHDOG_LOG:-/tmp/zerg-watchdog.log}"

if pgrep -f "watchdog.sh" >/dev/null 2>&1; then
  echo "ℹ️ 看门狗已在运行（PID $(pgrep -f 'watchdog.sh' | head -1)）——不重复启动"
  exit 0
fi

setsid nohup bash "$REPO_ROOT/scripts/svc/watchdog.sh" >>"$WD_LOG" 2>&1 </dev/null &

sleep 2
PID="$(pgrep -f 'watchdog.sh' | head -1 || true)"
if [ -n "$PID" ]; then
  echo "✅ 看门狗已启动（PID $PID · 日志 $WD_LOG · 每 10s 检查主控，掉线自动拉起）"
else
  echo "❌ 看门狗启动失败——日志尾部:" >&2
  tail -5 "$WD_LOG" >&2 || true
  exit 1
fi

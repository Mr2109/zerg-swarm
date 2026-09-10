#!/usr/bin/env bash
# start-zerg-core.sh — 启动/重启主控（2026-09-11 改为 launchd 托管）
#
# 背景（实测教训）：主控若作为 **agent/终端会话的子进程** 启动，会话被回收时主控会被一并带走
#   → 线上无人拉起，UI 报「主控离线」（2026-09-11 发生一次）。
#   现在统一由 **launchd** 托管（`~/Library/LaunchAgents/com.zerg.core.plist`，KeepAlive=true）：
#   进程 PPID=1、独立会话；被 kill / 崩溃后 ~3 秒内自动拉起（已实测）。
#
# 用法：
#   bash scripts/start-zerg-core.sh          # 已托管则重启（kickstart -k），未托管则安装并启动
#   bash scripts/start-zerg-core.sh --stop   # 停止并卸载托管（临时维护用）
#   bash scripts/start-zerg-core.sh --status # 查看托管状态
#
# 环境变量来源：仓库根 .env（由 scripts/zerg-core-daemon.sh 承载，launchd 调它）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LABEL="com.zerg.core"
PLIST="$HOME/Library/LaunchAgents/${LABEL}.plist"
UID_NUM="$(id -u)"
DOMAIN="gui/${UID_NUM}"

status() {
  if launchctl print "${DOMAIN}/${LABEL}" >/dev/null 2>&1; then
    PID="$(launchctl print "${DOMAIN}/${LABEL}" 2>/dev/null | awk '/^\tpid = /{print $3}')"
    echo "✅ 主控由 launchd 托管（PID ${PID:-?} · 标签 ${LABEL}）"
  else
    echo "ℹ️ 主控未托管（launchd 里没有 ${LABEL}）"
  fi
  pgrep -f "bin/zerg-core" >/dev/null 2>&1 && echo "   进程: PID $(pgrep -f 'bin/zerg-core' | head -1)" || echo "   进程: 无"
  lsof -nP -iTCP:8580 -sTCP:LISTEN >/dev/null 2>&1 && echo "   端口: 8580 监听中" || echo "   端口: 8580 未监听"
}

install_plist() {
  mkdir -p "$(dirname "$PLIST")"
  cat >"$PLIST" <<PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>${LABEL}</string>
	<key>ProgramArguments</key>
	<array><string>/bin/bash</string><string>${REPO_ROOT}/scripts/zerg-core-daemon.sh</string></array>
	<key>WorkingDirectory</key><string>${REPO_ROOT}</string>
	<key>KeepAlive</key><true/>
	<key>RunAtLoad</key><true/>
	<key>ThrottleInterval</key><integer>10</integer>
	<key>StandardOutPath</key><string>/tmp/zerg-core.log</string>
	<key>StandardErrorPath</key><string>/tmp/zerg-core.log</string>
	<key>EnvironmentVariables</key>
	<dict><key>PATH</key><string>/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
</dict>
</plist>
PLIST_EOF
  plutil -lint "$PLIST" >/dev/null
  echo "✓ 已生成 ${PLIST}（仓库根 = ${REPO_ROOT}）"
}

case "${1:-}" in
  --status)
    status
    exit 0
    ;;
  --stop)
    launchctl bootout "${DOMAIN}/${LABEL}" 2>/dev/null || true
    pkill -f "bin/zerg-core" 2>/dev/null || true
    echo "✓ 已停止并卸载托管（重启托管：bash scripts/start-zerg-core.sh）"
    exit 0
    ;;
esac

chmod +x "$REPO_ROOT/scripts/zerg-core-daemon.sh" 2>/dev/null || true

if [ ! -f "$PLIST" ] || grep -q "zerg-core-daemon.sh" "$PLIST" 2>/dev/null; then
  install_plist
else
  install_plist   # 每次重写：路径随仓库位置自动跟随，避免陈旧路径
fi

if launchctl print "${DOMAIN}/${LABEL}" >/dev/null 2>&1; then
  launchctl kickstart -k "${DOMAIN}/${LABEL}" >/dev/null 2>&1 || true
  echo "🔄 已重启托管主控（kickstart -k ${LABEL}）"
else
  launchctl bootstrap "${DOMAIN}" "$PLIST" 2>/dev/null || launchctl load -w "$PLIST"
  echo "🚀 已安装并启动托管主控（bootstrap ${LABEL}）"
fi

sleep 6
status

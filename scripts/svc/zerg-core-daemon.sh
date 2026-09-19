#!/usr/bin/env bash
# zerg-core-daemon.sh — launchd 托管的启动包装（2026-09-11）
#
# 被 ~/Library/LaunchAgents/com.zerg.core.plist 调用（KeepAlive=true → 掉线由系统拉起）。
# 职责：① 定位仓库根 ② 源仓库根 .env（令牌/知识库/白名单）③ 补 PATH（launchd 默认 PATH 极简，
# 会找不到 ffmpeg/pdftotext/llama-server 等外部工具）④ exec 主控（不 nohup——由 launchd 监管）。
#
# 手动测试：bash scripts/svc/zerg-core-daemon.sh   （前台运行，Ctrl-C 结束）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

# launchd 默认 PATH=/usr/bin:/bin:/usr/sbin:/sbin → 外部工具（ffmpeg/pdftotext/llama-server）会找不到
export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:${HOME}/.local/bin:${HOME}/.cargo/bin"

export ZERG_GIT_COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"

exec "$REPO_ROOT/bin/zerg-core"

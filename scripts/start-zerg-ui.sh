#!/usr/bin/env bash
# start-zerg-ui.sh — 以「脱离会话」的方式启动虫族桌面 UI（2026-09-11 新增）
#
# 背景（实测）：UI 若作为 agent/终端会话的子进程启动，会话被回收时 UI 会被一并关掉
#   （2026-09-11 主控与 UI 同时消失即此因）。
#   macOS 没有 setsid 命令 → 用 python3 的 os.setsid() 让进程脱离会话与进程组，
#   再把 stdout/stderr 接到日志、stdin 接 /dev/null。
#
# 不做 launchd KeepAlive：GUI 由人开关，关掉后不该自动弹回来（主控才需要常驻托管）。
#
# 用法：bash scripts/start-zerg-ui.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG="${ZERG_UI_LOG:-/tmp/zerg-ui.log}"
BIN="$REPO_ROOT/bin/zerg-ui"

cd "$REPO_ROOT"
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

if [ ! -x "$BIN" ]; then
  echo "❌ UI 二进制不可执行: ${BIN}（先 cargo build --release -p zerg-ui → 拷到 bin/ → codesign -s - --force）" >&2
  exit 1
fi

if pgrep -f "bin/zerg-ui" >/dev/null 2>&1; then
  echo "ℹ️ UI 已在运行（PID $(pgrep -f 'bin/zerg-ui' | head -1)）——不重复启动"
  exit 0
fi

# 脱离会话启动（os.setsid → 新会话/新进程组；日志与 stdin 重定向）
nohup python3 -c '
import os, sys
os.setsid()
log = os.open(sys.argv[2], os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o644)
os.dup2(log, 1)
os.dup2(log, 2)
devnull = os.open(os.devnull, os.O_RDONLY)
os.dup2(devnull, 0)
# 2026-09-11 修（UI 拿不到模型列表 /api/fleet/* 全 403）：把权威令牌注入环境再 exec。
# 原因：api_token() 依次读 env ZERG_AUTH_TOKEN → prefs.json 的 auth_token，而启动器一个都不给；
# 令牌轮换后 UI 一直拿旧值 → 403。权威位置=~/.zerg/token（600），与主控/子端同一来源（库内零明文）。
_tok = os.path.expanduser("~/.zerg/token")
try:
    _v = open(_tok, encoding="utf-8").read().strip()
    if _v:
        os.environ["ZERG_AUTH_TOKEN"] = _v
except Exception:
    pass
os.execv(sys.argv[1], [sys.argv[1]])
' "$BIN" "$LOG" >/dev/null 2>&1 &

sleep 4
PID="$(pgrep -f 'bin/zerg-ui' | head -1 || true)"
if [ -n "$PID" ]; then
  PPID_="$(ps -o ppid= -p "$PID" | tr -d ' ')"
  echo "✅ UI 已启动（PID $PID · PPID $PPID_ · 日志 ${LOG}）"
  echo "   会话/进程组: $(ps -o pgid=,sess= -p "$PID" | tr -s ' ')"
else
  echo "❌ UI 启动失败——日志尾部:" >&2
  tail -8 "$LOG" >&2 || true
  exit 1
fi

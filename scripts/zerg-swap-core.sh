#!/bin/bash
# zerg-swap-core.sh —— 主控换件的**受控流程**（2026-09-16 事故后固化，Mr2109 拍）。
#
# 为什么存在：2026-09-16 换件时我手搓 go build（身份丢失 ⇒ code_sha=unknown），
# 回滚时又把 codesign 的输出丢进 /dev/null（签名失败没看见 ⇒ 回滚件签名无效 ⇒
# 新进程卡死在 dyld 读自身）。三条教训就是本脚本的三道门禁。
#
# 三道门禁（缺一不可）：
#   ① 构建只走 scripts/build-all.sh（身份注入 + ad-hoc 重签都在里面）——**绝不手搓 go build**；
#   ② 换件前必须**验签**（codesign --verify，输出原样打印）——签名不过就不换；
#   ③ 起不来 ⇒ 回滚 + **回滚件同样验签** + 再启 + 打印失败现场（日志 tail）——
#      **任何命令的输出都不许被吞**（不用 >/dev/null 2>&1 掩盖）。
#
# 用法：bash scripts/zerg-swap-core.sh
# 退出码：0 换件成功 · 1 构建/验签失败（未动生产）· 2 换件后起不来但已回滚成功 · 3 回滚也失败（需人工）
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"
BIN="bin/zerg-core"
PLIST_LABEL="com.zerg.core"
ENDPOINT="http://127.0.0.1:8580/api/capabilities"
TS="$(date +%Y%m%dT%H%M%S)"
LOG="/tmp/zerg-swap-$TS.log"
BACKUP="/tmp/zerg-core.swap-backup-$TS"

say()  { echo "$@" | tee -a "$LOG"; }
hr()   { say "──────────────────────────────────────────────"; }

say "== 主控换件 $TS =="; say "日志：$LOG"
TOKEN="$(cat "$HOME/.zerg/token" 2>/dev/null || true)"
probe() { curl -s -o /tmp/swap-probe.json -w '%{http_code}' -m 4 "$ENDPOINT" -H "X-Auth-Token: $TOKEN"; }
identity() { python3 -c "
import json
try:
    d=json.load(open('/tmp/swap-probe.json'))
    print('code_sha=%s version=%s' % (d.get('code_sha'), d.get('version')))
except Exception as e:
    print('(读不到身份: %s)' % e)"; }

wait_ready() {  # $1 = 最长等秒
  local i c="000"
  for i in $(seq 1 $(( $1 / 2 ))); do
    sleep 2
    c="$(probe)"
    [ "$c" = "200" ] && break
  done
  echo "$c"
}

# ── 0. 换件前现场（必须可回滚）──────────────────────────────────
hr; say "[0] 换件前现场"
BEFORE_CODE="000"; BEFORE_CODE="$(probe)"; say "  换件前 /api/capabilities=$BEFORE_CODE"; identity | sed 's/^/  before /' | tee -a "$LOG"
if ! cp -p "$BIN" "$BACKUP"; then say "!! 备份失败 ⇒ 不换件"; exit 1; fi
say "  备份：${BACKUP}（$(stat -f%z "$BIN") 字节）"

# ── 1. 构建（只走 build-all.sh）─────────────────────────────────
hr; say "[1] 构建（scripts/build-all.sh —— 身份注入 + 重签）"
if ! bash scripts/build-all.sh >>"$LOG" 2>&1; then
  say "!! 构建失败 ⇒ 未动生产（现场见 ${LOG}）"; tail -20 "$LOG"; exit 1
fi
say "  构建完成：$(stat -f%z "$BIN") 字节"

# ── 2. 验签（门禁②：不过就不换）───────────────────────────────
hr; say "[2] 验签（codesign --verify —— 输出原样打印，不吞）"
codesign -dv --verbose=2 "$BIN" 2>&1 | grep -E "Identifier|Signature|Format" | sed 's/^/  /' | tee -a "$LOG"
if ! codesign --verify --verbose=2 "$BIN" 2>&1 | tee -a "$LOG"; then
  say "!! 验签失败 ⇒ 不换件"; exit 1
fi
say "  验签通过 ✓"

# ── 3. 换件 + 起 + 验端点 + 自证 ────────────────────────────────
hr; say "[3] kickstart + 等就绪 + 读身份"
launchctl kickstart -k "gui/$UID/$PLIST_LABEL" || { say "!! kickstart 失败"; }
AFTER_CODE="$(wait_ready 60)"
say "  换件后 /api/capabilities=$AFTER_CODE"; identity | sed 's/^/  after  /' | tee -a "$LOG"

if [ "$AFTER_CODE" = "200" ]; then
  say "✓ 换件成功"; exit 0
fi

# ── 4. 起不来 ⇒ 回滚（回滚件**同样验签**）+ 保留现场 ─────────────
hr; say "[4] 新件起不来 ⇒ 回滚（含验签）"
say "  失败现场（$LOG 尾）："; tail -25 "$LOG"
cp -p "$BACKUP" "$BIN" || { say "!! 复制回滚件失败 ⇒ 人工介入"; exit 3; }
say "  回滚件已就位：$(stat -f%z "$BIN") 字节"
codesign -s - --force --identifier "$PLIST_LABEL" "$BIN" 2>&1 | sed 's/^/  sign: /' | tee -a "$LOG"
if ! codesign --verify --verbose=2 "$BIN" 2>&1 | sed 's/^/  verify: /' | tee -a "$LOG"; then
  say "!! 回滚件验签失败 ⇒ 人工介入（这正是 2026-09-16 二次事故的形态）"; exit 3
fi
launchctl kickstart -k "gui/$UID/$PLIST_LABEL"
RB_CODE="$(wait_ready 60)"
say "  回滚后 /api/capabilities=$RB_CODE"; identity | sed 's/^/  rolled /' | tee -a "$LOG"
if [ "$RB_CODE" = "200" ]; then say "✓ 已回滚到换件前状态"; exit 2; fi
say "!! 回滚也没起来 ⇒ 人工介入"; exit 3

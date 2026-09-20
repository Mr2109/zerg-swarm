#!/bin/bash
# zerg-swap-core.sh —— 主控换件的**受控流程**（2026-09-16 事故后固化，Mr2109 拍）。
#
# 为什么存在：2026-09-16 换件时我手搓 go build（身份丢失 ⇒ code_sha=unknown），
# 回滚时又把 codesign 的输出丢进 /dev/null（签名失败没看见 ⇒ 回滚件签名无效 ⇒
# 新进程卡死在 dyld 读自身）。三条教训就是本脚本的三道门禁。
#
# 三道门禁（缺一不可）：
#   ① 构建只走 scripts/build/build-all.sh（身份注入 + ad-hoc 重签都在里面）——**绝不手搓 go build**；
#   ② 换件前必须**验签**（codesign --verify，输出原样打印）——签名不过就不换；
#   ③ 起不来 ⇒ 回滚 + **回滚件同样验签** + 再启 + 打印失败现场（日志 tail）——
#      **任何命令的输出都不许被吞**（不用 >/dev/null 2>&1 掩盖）。
#
# 两道前置闸（2026-09-20 批 B ① 加 · 换件前跑，不过 ⇒ exit 1 且**未动生产**）：
#   闸 A **工作树里没有并行的半成品**：换件是**从工作树编译**的 ⇒ 有未提交改动（`core/` `agent/`
#        下非未跟踪项）就把半成品编进生产件。计数 ≠ 0 ⇒ 不换。
#   闸 B **launchd 定义可用**：盘上 plist 引用的程序（`/bin/bash` + 守护包装脚本）**必须真存在**。
#        为什么单列这一条：脚本搬家（`git mv`）后 plist 里那句路径会变成**悬空**，而 launchd
#        **缓存的是加载时的定义** —— 此时 `kickstart -k` 会先把在跑的主控 kill 掉、再用陈旧路径
#        拉不起来（KeepAlive 每 10s 重试、一直是失败的），**回滚路径同样起不来**。这是本机真实
#        发生过一次的形态（2026-09-20：`scripts/zerg-core-daemon.sh` 已被 git mv 到 `scripts/svc/`，
#        而 plist 与 launchd 缓存都还指着老路径）。
#        ⇒ 另加一条**运行时**规矩（见 `restart_master()`）：已加载定义 ≠ 盘上 plist ⇒ 用
#          `bootout + bootstrap` **刷新定义**，只有定义一致时才 `kickstart -k`。
#
# 构建档：本脚本走 `build-all.sh --only-core`（只写 `bin/zerg-core` 一个文件）——
#   换主控**不许**顺手重编子端/UI（`bin/zerg-agentd` 是正在跑的子端件；见 build-all.sh 该档注释）。
#
# 用法：bash scripts/build/zerg-swap-core.sh
# 退出码：0 换件成功 · 1 构建/验签失败（未动生产）· 2 换件后起不来但已回滚成功 · 3 回滚也失败（需人工）
#
# 退码唯一真源 = zerg help exit-codes（契约 §三）——**本件是换件脚本自有码空间**（它不是命令面，§6.1）。
# 撞号处逐条登记在《开工记录-批A-20260920.md》T-04 节：本件把「**回滚成功**」表达成 `2`，
# 而 §十二 `P-129` 的定案是「回滚成功 = `1` + error.kind=ROLLED_BACK」⇒ 两义**未闭合**，
# 处置 = 批 C 的 T-26（定说法）与 T-28（改回滚路径：重启 + 验签 + 回执），**本批只登记不改**。
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"
BIN="bin/zerg-core"
PLIST_LABEL="com.zerg.core"
DOMAIN="gui/$UID"
PLIST="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
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

# ── launchd 定义的两个读法（都只读，不改任何东西）───────────────────────────────
# 盘上 plist 的 ProgramArguments（空格连接）；读不到 ⇒ 打印原因并 rc≠0
plist_program() {
  python3 - "$PLIST" <<'PY'
import plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception as e:
    print("(读不到 plist: %s)" % e); sys.exit(1)
print(" ".join(d.get("ProgramArguments") or []))
PY
}
# plist 里所有「看起来是路径但盘上不存在」的项（每个一行）—— 闸 B 的判据
plist_missing_files() {
  python3 - "$PLIST" <<'PY'
import os, plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception:
    sys.exit(0)
for a in (d.get("ProgramArguments") or []):
    if a.startswith("/") and not os.path.exists(a):
        print(a)
PY
}
# launchd **已加载**定义里的 ProgramArguments（缓存的那一份；没有该作业 ⇒ 空）
loaded_program() {
  launchctl print "${DOMAIN}/${PLIST_LABEL}" 2>/dev/null | python3 -c '
import re, sys
txt = sys.stdin.read()
m = re.search(r"arguments = \{(.*?)\}", txt, re.S)
if not m:
    print(""); sys.exit(0)
print(" ".join(x.strip() for x in m.group(1).split("\n") if x.strip()))
'
}

# ── 唯一的重启入口 ─────────────────────────────────────────────────────────────
# 为什么不能一律 `kickstart -k`：launchd **缓存的是加载时的作业定义**。盘上 plist 被改过
# （例如脚本被 git mv、`start-zerg-core.sh` 按新仓根重写了它）而作业没重新加载时，kickstart 仍按
# **缓存定义**拉 —— 缓存里的程序路径若已悬空，行为是「先把在跑的主控 kill 掉，再用不存在的程序
# 起，起不来，KeepAlive 每 10s 重试仍失败」。判据落在**运行时**：定义不一致就 bootout+bootstrap。
restart_master() {
  local lp dp
  lp="$(loaded_program)"
  dp="$(plist_program)"
  if [ -z "$lp" ]; then
    say "  [重启] launchd 里没有 ${PLIST_LABEL} ⇒ bootstrap（首次装载）"
    launchctl bootstrap "${DOMAIN}" "$PLIST" || launchctl load -w "$PLIST"
    return $?
  fi
  if [ "$lp" != "$dp" ]; then
    say "  [重启] 已加载定义 ≠ 盘上 plist ⇒ bootout + bootstrap（刷新定义；kickstart 用缓存定义会起不来）"
    say "         已加载: $lp"
    say "         盘上  : $dp"
    launchctl bootout "${DOMAIN}/${PLIST_LABEL}" 2>/dev/null || true
    launchctl bootstrap "${DOMAIN}" "$PLIST" || launchctl load -w "$PLIST"
    return $?
  fi
  say "  [重启] 已加载定义 == 盘上 plist ⇒ kickstart -k"
  launchctl kickstart -k "${DOMAIN}/${PLIST_LABEL}"
}

# ── 前置闸（两道；任一不过 ⇒ exit 1 且**未动生产**）────────────────────────────
hr; say "[闸] 换件前置闸（未过 ⇒ 不换件、不动生产）"
DIRTY="$(git status --porcelain core/ agent/ | grep -v '^?? ' | wc -l | tr -d ' ')"
say "  闸A 工作树半成品：core/ agent/ 未提交改动数 = ${DIRTY}（须为 0——换件是从工作树编译的）"
if [ "$DIRTY" != "0" ]; then
  git status --porcelain core/ agent/ | grep -v '^?? ' | sed 's/^/       /' | tee -a "$LOG"
  say "!! 有并行半成品 ⇒ 不换件（等它落地再换）"; exit 1
fi
say "  闸B 盘上 plist：${PLIST}"
say "      程序串：$(plist_program)"
MISSING="$(plist_missing_files)"
if [ -n "$MISSING" ]; then
  say "!! plist 引用的程序盘上不存在：${MISSING}"
  say "   ⇒ 不换件：kickstart/回滚都会起不来（launchd 用的是**缓存定义**，不只影响这一次）"
  say "   修法：bash scripts/build/start-zerg-core.sh 会按当前仓根重写 plist（它生成的形态即真源）；"
  say "         改完盘上 plist 后**不用**手工重载——本脚本的重启步会因「已加载定义 ≠ 盘上 plist」自动 bootout+bootstrap"
  exit 1
fi
say "  前置闸通过 ✓"

# ── 0. 换件前现场（必须可回滚）──────────────────────────────────
hr; say "[0] 换件前现场"
BEFORE_CODE="000"; BEFORE_CODE="$(probe)"; say "  换件前 /api/capabilities=$BEFORE_CODE"; identity | sed 's/^/  before /' | tee -a "$LOG"
if ! cp -p "$BIN" "$BACKUP"; then say "!! 备份失败 ⇒ 不换件"; exit 1; fi
say "  备份：${BACKUP}（$(stat -f%z "$BIN") 字节）"

# ── 1. 构建（只走 build-all.sh --only-core）─────────────────────
hr; say "[1] 构建（scripts/build/build-all.sh --only-core —— 身份注入 + 重签；只写 bin/zerg-core）"
if ! bash scripts/build/build-all.sh --only-core 2>&1 | tee -a "$LOG"; then
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
hr; say "[3] 重启主控（见 restart_master：定义不一致则 bootout+bootstrap）+ 等就绪 + 读身份"
restart_master || { say "!! 重启动作失败（详见上行输出）"; }
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
restart_master || { say "!! 回滚后的重启动作失败"; }
RB_CODE="$(wait_ready 60)"
say "  回滚后 /api/capabilities=$RB_CODE"; identity | sed 's/^/  rolled /' | tee -a "$LOG"
if [ "$RB_CODE" = "200" ]; then say "✓ 已回滚到换件前状态"; exit 2; fi
say "!! 回滚也没起来 ⇒ 人工介入"; exit 3

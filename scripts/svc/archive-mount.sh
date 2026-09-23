#!/usr/bin/env bash
# archive-mount.sh — 归档区**按需挂载**（v2.5.10 归档 dmg · 2026-09-23 Mr2109 拍板）
#
# 为什么有它（病根 · 现读）
#   归档区已被改成「**只读 dmg 镜像 + 原路径同名符号链接**」：
#     · dmg    = <模型类>/Zerg-归档-v2.5.10.dmg（416,751 B · 只读）
#     · 挂载点 = /Volumes/ZERG-ARCHIVE
#     · 原路径 = <模型类>/Zerg-归档 → **符号链接** → 挂载点
#   两条硬约束（现读实测，不是设计偏好）：
#     ① 只读 dmg **不能挂在 `<volume-path>` 卷内的任何路径上**（现跑被拒）⇒ 挂载点必须留在 `/Volumes/` 下；
#     ② Mr2109 口径（逐字）：「肯定是**按需加载** 而不是 一直加载」⇒ **不许**写 launchd 常驻 / 开机自动挂载。
#   于是「挂」与「摘」只能是**人显式按需**的一次动作 —— 本脚本就是那一次动作的命令面：
#   它**不自启、不常驻、不装 launchd 件**；跑一次、成一次、退一次。
#
# 做的事（三个动作 · 各自幂等）
#   on      `hdiutil attach -readonly -mountpoint <挂载点> <dmg>`；已在挂 ⇒ 如实说「已挂」，**不再挂一遍**
#   off     `hdiutil detach <挂载点>`；未挂 ⇒ 如实说「未挂」，**不再摘一遍**
#   status  报挂载态；★ **未挂时明确说明「原路径是悬空符号链接」**——那一刻读它**会失败**，
#           所以这一档**不是「正常」**（退回码 1，见下）⇒ 这就是 fail-closed 的那一格。
#
# 用法：
#   bash scripts/svc/archive-mount.sh on        # 挂上（已在挂则只报「已挂」）
#   bash scripts/svc/archive-mount.sh off       # 摘下（未挂则只报「未挂」）
#   bash scripts/svc/archive-mount.sh status    # 报挂载态（未挂 ⇒ 明说悬空符号链接 + 退 1）
#
# 环境（**只为夹具/自测能指到别的件**；默认即本机真物件 —— 照本目录 `fleet-relay.sh` 的
# `ZERG_RELAY_SSH`「沙箱注入假命令、绝不连真机」同形）：
#   ZERG_ARCHIVE_DMG      归档 dmg 路径（默认 <模型类>/Zerg-归档-v2.5.10.dmg）
#   ZERG_ARCHIVE_MOUNT    挂载点（默认 /Volumes/ZERG-ARCHIVE）
#   ZERG_ARCHIVE_LINK     原路径符号链接（默认 <模型类>/Zerg-归档）
#   ZERG_HDIUTIL          hdiutil 命令（默认 hdiutil）
#   ZERG_MOUNT_LIST       挂载表命令（默认 mount）
#
# 退出码（**照本目录现成口径** · 与 `fleet-relay.sh` 同一张表）：
#   0  = 成功（`on`/`off` 动作到位 · `status` 报「已挂」）
#   1  = 失败（动作跑了但没成 / 回读不等 · `status` 报「**未挂**」——它是按需态，但原路径此刻是
#        悬空符号链接 ⇒ **不许当正常**）
#   2  = 不给结论（前提不可判：dmg 不在 / 原路径不是符号链接 / 两处读数打架）
#  64  = 用法错误
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KIN_ROOT="$(cd "${REPO_ROOT}/.." && pwd)"

DMG="${ZERG_ARCHIVE_DMG:-${KIN_ROOT}/Zerg-归档-v2.5.10.dmg}"
MOUNT="${ZERG_ARCHIVE_MOUNT:-/Volumes/ZERG-ARCHIVE}"
LINK="${ZERG_ARCHIVE_LINK:-${KIN_ROOT}/Zerg-归档}"
HDIUTIL="${ZERG_HDIUTIL:-hdiutil}"
MOUNTLIST="${ZERG_MOUNT_LIST:-mount}"

say()  { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }

# ── 两处读数（判挂载态**只看这两处**；不许拿「目录存在」当已挂 —— 挂载点是空目录时它与未挂同形）──
# ★ 路径归一：macOS 上 `/tmp` 是 `/private/tmp` 的符号链接 ⇒ 同一处挂载在挂载表里可能写成另一串。
#   两侧都认（原始形态 + `pwd -P` 形态）：fixture 指到 /tmp 下时判据才不会假红。
#   ★ **现读现算**（不是启动时取一次）：挂载点在首次 `on` 之前**还不存在**，
#     启动时一次性取值会把 `/tmp/x` 定死成未归一形态（macOS 上是 `/private/tmp/x`）⇒
#     `on` 挂完的回读就会「读作不在」而**假红**（2026-09-23 夹具现跑抓到：首跑 rc=1、次跑才 0）。
mount_real() { ( cd "${MOUNT}" 2>/dev/null && pwd -P ) || printf '%s' "${MOUNT}"; }
mount_row() {        # ① 挂载表里那一行（无则空）
  "${MOUNTLIST}" 2>/dev/null | grep -F " on ${MOUNT} (" \
    || "${MOUNTLIST}" 2>/dev/null | grep -F " on $(mount_real) (" || true
}
hdiutil_row() {      # ② hdiutil 认的挂载点（无则空）
  "${HDIUTIL}" info 2>/dev/null | grep -F "${MOUNT}" \
    || "${HDIUTIL}" info 2>/dev/null | grep -F "$(mount_real)" || true
}
# ★ 有界等待读数落地（磁盘仲裁/挂载表更新会滞后 ⇒ 立刻回读会**假红**）：
#   只在那一次动作之后、最多 ~3s；**不是**轮询常驻 —— 本脚本仍是「跑一次 · 成一次 · 退一次」。
settle() {  # $1=想要的状态 mounted|unmounted
  local i=0
  while [ "${i}" -lt 10 ]; do
    case "$1" in
      mounted)   is_mounted && return 0 ;;
      unmounted) is_mounted || return 0 ;;
    esac
    sleep 0.3; i=$((i + 1))
  done
  return 1
}
is_mounted() { [ -n "$(mount_row)" ]; }

# preflight —— 三件前提（缺一件就**不给结论**，不许猜）
preflight() {
  if [ ! -e "${DMG}" ]; then
    warn "⛔ 不给结论：归档 dmg 不在盘上（${DMG}）—— 没有可挂的件，本脚本不硬造"
    return 2
  fi
  if [ ! -L "${LINK}" ]; then
    if [ -e "${LINK}" ]; then
      warn "⛔ 不给结论：原路径 ${LINK} **不是符号链接**（是实体）⇒ 本脚本的前提不成立（不许把实体目录当挂载点用）"
    else
      warn "⛔ 不给结论：原路径 ${LINK} 连符号链接都不在 ⇒ 挂上也没有那条路可走"
    fi
    return 2
  fi
  if ! "${HDIUTIL}" info >/dev/null 2>&1; then
    warn "⛔ 不给结论：${HDIUTIL} 跑不起来（判据取不到读数 ⇒ 绿就是假绿）"
    return 2
  fi
  return 0
}

# 交叉核对：挂载表说「挂上了」而 hdiutil 不认这条挂载点 ⇒ 两处读数打架 ⇒ 不给结论
cross_check() {
  if is_mounted && [ -z "$(hdiutil_row)" ]; then
    warn "⛔ 不给结论：挂载表说 ${MOUNT} 已挂，但 \`${HDIUTIL} info\` 里没有这条挂载点 ⇒ 两处读数打架"
    return 2
  fi
  return 0
}

status_line() {
  if is_mounted; then
    say "✅ 已挂（按需挂载态里「挂上」的那一态）"
    say "  挂载行 : $(mount_row)"
    say "  hdiutil: $(hdiutil_row)"
    say "  原路径 : ${LINK} → ${MOUNT}（符号链接**解析得通** —— 读它 = 读只读镜像内的件）"
    if [ -r "${MOUNT}" ]; then
      say "  抽验读 : $(ls -1 "${MOUNT}" 2>/dev/null | head -3 | tr '\n' ' ')"
    fi
  else
    say "⚠️ **未挂**（按需挂载态里「没挂」的那一态 —— 这是本脚本的**正常业务态**，不是脚本坏了）"
    say "  原路径 : ${LINK} → ${MOUNT}"
    if [ -d "${MOUNT}" ]; then
      say "  ★ 挂载点目录 ${MOUNT} 还在（空目录）⇒ 原路径**不报错、会静默返回空** ——"
      say "     别把它当「归档是空的」；要真挂上：\`bash scripts/svc/archive-mount.sh on\`"
    else
      say "  ★ 此刻 ${MOUNT} 不存在 ⇒ ${LINK} 是**真·悬空符号链接**：读 ${LINK}/… 会**明确失败**"
      say "     （就是 \`No such file or directory\` —— 不是「空目录」、更不是「读到空内容」）"
    fi
    say "  要读归档：先 \`bash scripts/svc/archive-mount.sh on\`；用完 \`… off\`（按需，不常驻）"
  fi
}

cmd_status() {
  preflight || return $?
  cross_check || return $?
  status_line
  if is_mounted; then
    return 0
  fi
  # 未挂 ⇒ 退 1：**不许把它报成「正常」** —— 原路径此刻是悬空符号链接，读它会失败。
  return 1
}

cmd_on() {
  preflight || return $?
  if is_mounted; then
    say "✅ 已挂（幂等：${MOUNT} 已在挂，本次**不重复挂**）"
    say "  挂载行 : $(mount_row)"
    return 0
  fi
  mkdir -p "${MOUNT}"
  if ! "${HDIUTIL}" attach -readonly -mountpoint "${MOUNT}" "${DMG}"; then
    warn "❌ 挂不上：\`${HDIUTIL} attach -readonly -mountpoint ${MOUNT} ${DMG}\` 失败"
    warn "  常见原因：挂载点被别的卷占着 · 或挂载点落在**只读卷内的路径**上（现读实测：那会被拒）"
    return 1
  fi
  # 回读验证（挂完必须再读一遍；读了不等 ⇒ 1）—— 有界等读数落地（≤3s，判定口径不变）
  if ! settle mounted; then
    warn "❌ 挂载命令返回成功，但挂载表里 ${MOUNT} **不在**（回读不等）⇒ 判失败"
    return 1
  fi
  say "✅ 已挂：${MOUNT}（只读）"
  say "  挂载行 : $(mount_row)"
  say "  原路径 : ${LINK} → ${MOUNT}（符号链接现在解析得通）"
  return 0
}

cmd_off() {
  preflight || return $?
  if ! is_mounted; then
    say "✅ 未挂（幂等：${MOUNT} 本就不在挂载表里，本次**不重复摘**）"
    return 0
  fi
  if ! "${HDIUTIL}" detach "${MOUNT}"; then
    warn "❌ 摘不下：\`${HDIUTIL} detach ${MOUNT}\` 失败（有进程正占着镜像里的件时会被拒）"
    warn "  先看谁占着：\`lsof +D ${MOUNT}\`（**不要**换 \`-force\` 硬摘 —— 那是不可逆动作）"
    return 1
  fi
  if ! settle unmounted; then
    warn "❌ 摘除命令返回成功，但挂载表里 ${MOUNT} **还在**（回读不等）⇒ 判失败"
    return 1
  fi
  # ★ 挂载点那个**空目录**要一起收掉 —— 留着它，「悬空符号链接」这句话就是假的：
  #   符号链接会解析到一个**空目录**，`ls 原路径/` 会**成功返回空**（正是本脚本要防的「静默返回空」形态）。
  #   只收**空目录**（`rmdir` 非空必失败 ⇒ 有别的件在里面时不硬删，如实报）。
  local mp_note="挂载点目录还在（非空 ⇒ 留给下次 on 复用）"
  if rmdir "${MOUNT}" 2>/dev/null; then
    mp_note="挂载点空目录已收起 ⇒ 原路径现在是**真·悬空符号链接**（读它 = \`No such file or directory\`）"
  fi
  say "✅ 已摘：${MOUNT} 不在挂载表里了"
  say "  ${mp_note}"
  say "  原路径 : ${LINK} → ${MOUNT}（要读归档就再 \`on\` 一次 —— 按需，不常驻）"
  return 0
}

usage() { sed -n '2,50p' "${0}"; }

case "${1:-}" in
  on)     cmd_on ;;
  off)    cmd_off ;;
  status) cmd_status ;;
  -h|--help|"") usage; exit 0 ;;
  *) warn "❌ 未知参数: ${1}（只认 on | off | status）"; exit 64 ;;
esac

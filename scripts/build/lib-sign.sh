#!/usr/bin/env bash
# lib-sign.sh — macOS 签名身份的**唯一**解析/施加面（2026-10-03 · GAP-20261003-10 收尾）
#
# 为什么抽成公共件：`build-all.sh`（本机部署件）与 `pack-release.sh`（发布件）**两档都要签名**，
#   而本仓铁律是「两份实现就会漂」—— 漂的形态正是最怕的那种：一件用稳定身份、另一件回落 ad-hoc
#   ⇒ macOS TCC 的本地网络授权随 cdhash 静默失效（现象：主控拨号报 no route to host）。
#   ⇒ 两个调用者 `source` **同一份**，✗ 不许各自复制一份。
#
# 契约（调用者只须保证这两项）：
#   · `SIGN`（默认 1）：0 ⇒ 两个函数都直接 return 0（不签）
#   · `TMPDIR`（可省）：探针件落点；省则落 /tmp
# 产出（调用者读它们）：
#   · `ZERG_SIGN_ID`：解析到的身份名；回落 ad-hoc 时置 `-`
#   · `SIGN_MODE`：`stable:<身份名>` 或 `adhoc`
# 边界：**只解析身份 + 施加签名** —— 不编译、不打包、不碰进程、不读仓内证物。
#   件不在场时 `sign_one` 自带 `[ -f ]` 守卫（调用者不必先判存在）。
#
# 内容来源：逐字搬自 `scripts/build/build-all.sh`（2026-10-03 前那两份内联定义），行为不变；
#   唯一语义增量是 `sign_one` 的 identifier 表补上**四段契约名通配**（原表只列了两条四段名，
#   发布档的 `zerg-core-linux-amd64` / `zerg-cli-linux-amd64` 等会掉进兜底
#   `com.zerg.<全名>` ⇒ 同一组件两套身份 ⇒ 正是 GAP-20261003-10 要清的那种错位）。
resolve_sign_identity() {   # 只解析身份，不改文件；结果落 ZERG_SIGN_ID / SIGN_MODE
  [ "${SIGN:-1}" = "1" ] || return 0
  command -v codesign >/dev/null 2>&1 || return 0
  ZERG_SIGN_ID="${ZERG_SIGN_ID:-Zerg Local Signing}"
  # ★ GAP-20261003-07 修正（2026-10-03 · 含一次自我更正）：
  #   病灶（成立的那一条）：原写法 `codesign -s … --force /tmp/.zerg-sign-probe` 签的是**一个不在盘的路径**
  #     ⇒ codesign 报 No such file ⇒ 该兜底支**恒失败** ⇒ 一旦第一支不成立就**必然静默回落 adhoc**，
  #     与「探针保底」的设计意图相反（后果：Identifier 变 ad-hoc、TCC 本地网络授权随每次重编失效）。
  #     ⇒ 探针件必须先造出来（唯一名 · 精确清理）。
  #   ★ 更正：同日我一度以为「断点① = `find-identity -p codesigning` 在身份 NOT_TRUSTED 时恒为空」——
  #     **不成立**（现读用了错 policy 名 `codesign`，真名 `codesigning`）。实测旧逻辑第一支本来就过、
  #     SIGN_MODE=stable ⇒ 本处**不是为了修降级**，只是为了把「兜底支真的兜得住」+「两态可分辨」补上。
  #   两态提示：身份在钥匙串但不受信任（CSSMERR_TP_NOT_TRUSTED）时**不影响签名**，照常签稳定身份并提示一行。
  local _probe="${TMPDIR:-/tmp}/.zerg-sign-probe.$$" _sig=""
  if [ -n "${ZERG_SIGN_ID}" ]; then : > "$_probe" 2>/dev/null; fi
  if [ -n "${ZERG_SIGN_ID}" ] && security find-identity 2>/dev/null | grep -q "$ZERG_SIGN_ID"; then
    _sig="in-keychain-not-trusted"
  elif [ -n "${ZERG_SIGN_ID}" ] && codesign -s "$ZERG_SIGN_ID" --force "$_probe" 2>/dev/null; then
    _sig="probe-ok"
  fi
  if [ -n "${_sig}" ]; then
    SIGN_MODE="stable:$ZERG_SIGN_ID"
  else
    ZERG_SIGN_ID="-"; SIGN_MODE="adhoc"
  fi
  rm -f "$_probe" 2>/dev/null
  if [ "$SIGN_MODE" = "adhoc" ]; then
    echo "   ⚠️  未找到签名身份「${ZERG_SIGN_ID}」，回退 ad-hoc（本机 TCC 授权会随重编失效）"
  elif [ "${_sig}" = "in-keychain-not-trusted" ]; then
    echo "   🔏 身份「${ZERG_SIGN_ID}」在钥匙串但**不受信任**（CSSMERR_TP_NOT_TRUSTED）—— 不影响签名，照常签稳定身份"
  fi
  return 0
}

sign_one() {  # sign_one <文件> —— 单件重签；identifier 按既有固定表
  local b="$1" ZID
  [ -f "$b" ] || return 0
  [ "${SIGN:-1}" = "1" ] || return 0
  command -v codesign >/dev/null 2>&1 || return 0
  case "$(basename "$b")" in
    # 本机部署件：三段短名（在跑的那些）
    zerg-core)   ZID=com.zerg.core ;;
    zerg-agent)  ZID=com.zerg.agent ;;
    zerg-agentd) ZID=com.zerg.agentd ;;
    zerg-ui)     ZID=com.zerg.ui ;;
    zerg-wall)   ZID=com.zerg.wall ;;
    # 四段契约名 `zerg-<组件>-<os>-<arch>`（跨架构 / 可分发件；真源
    # core/internal/selfupdate/build.go:96 ArtifactName）：identifier 必须与**同组件本机件**一致
    # —— 同一组件两枚二进制不许两套身份（GAP-20261003-10）。
    # ★ `zerg-agent-*` **不会**吞掉 `zerg-agentd-*`（`-` ≠ `d`）⇒ 两条可并存，但顺序仍按先精确后通配写。
    zerg-core-*)   ZID=com.zerg.core ;;
    zerg-agent-*)  ZID=com.zerg.agent ;;
    zerg-agentd-*) ZID=com.zerg.agentd ;;
    zerg-cli)      ZID=com.zerg.cli ;;
    zerg-cli-*)    ZID=com.zerg.cli ;;
    zerg-ui-*)     ZID=com.zerg.ui ;;
    zerg-wall-*)   ZID=com.zerg.wall ;;
    *)             ZID="com.zerg.$(basename "$b")" ;;
  esac
  codesign -s "$ZERG_SIGN_ID" --identifier "$ZID" --force "$b" >/dev/null 2>&1 \
    && echo "   🔏 已重签名 $(basename "$b")（${ZID}，${SIGN_MODE}）"
  return 0
}

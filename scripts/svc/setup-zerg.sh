#!/usr/bin/env bash
# setup-zerg.sh — 新机器一条命令 bootstrap 源码式虫族（B6，2026-09-13）
#
# 为什么需要（设计《设计-源码式发布与升级-20260913》§4.4 / §十二 B6）：
#   源码式更新（`zerg update` / 机群 `--fleet`）的前提是**那台机器上有代码树**。
#   纯二进制部署的节点（如机群节点现状）没有检出 ⇒ 机群 preflight 只能如实标 pending-manual。
#   本脚本就是把「新机器 → 有检出 + 有构建 + 有服务 + 自报身份对得上」这一条命令补齐。
#
# 一条命令做的事（顺序即依赖，每步都能失败、失败即非零退出）：
#   ① 取码     从公开镜像仓 clone（URL/ref 可覆盖；`--dir` 用已有树，脏树一律拒绝）
#   ② 工具链   核验本机 go 是否合钉住的 pin（**单一真源 = core/go.mod 的 go 指令**，
#              语义同 scripts/gates/check-gotoolchain.py）；不合就清楚招供 + 给安装指引，**绝不自行升级**
#   ③ 构建     按角色选组件集，走**仓库自己的入口** scripts/build/build-all.sh
#              （身份注入 -ldflags + ad-hoc 重签——绝不手写 go build，否则升级 verify 直接失效）
#   ④ 安装     换装到前缀（默认 <树>/bin）：.new → mv 原子换装，旧件留 .prev（照抄升级器纪律）
#   ⑤ 服务     controller ⇒ launchd plist（macOS；KeepAlive/RunAtLoad/ThrottleInterval +
#              .env/PATH 由仓库自带的 scripts/svc/zerg-core-daemon.sh 承载）
#              node       ⇒ systemd unit（管 zerg-agentd）
#              两者都幂等：已存在则**重写并 reload/kickstart**
#   ⑥ 验证     读**运行进程自报的身份**与安装件对账（controller 读 /api/capabilities 的 code_sha；
#              node 读 zerg-agentd --version）；对不上即非零退出（不假装装好了）
#
# 角色（与你给 zerg-upgrade.sh 的口径一致）：
#   controller = 主控机：core + agent（darwin 再加 ui）——服务只碰主控
#   node       = 机群节点：core（装上它，该机将来才能自己 `zerg-core update`）+ agentd（真在跑的那个）
#   ui 只在 darwin 编译/安装（设计 §3：UI 只出 Mac）——非 darwin 请求 ui 一律拒绝
#
# 用法：
#   bash scripts/svc/setup-zerg.sh                         # 主控机（macOS）默认路径
#   bash scripts/svc/setup-zerg.sh --role node             # 机群节点（linux，systemd 管 agentd）
#   bash scripts/svc/setup-zerg.sh --dir ~/zerg-swarm      # 用已有检出（脏树会被拒绝，不会静默覆盖）
#   bash scripts/svc/setup-zerg.sh --repo <url> --ref main # 换公开镜像仓 / 分支
#   bash scripts/svc/setup-zerg.sh --prefix /opt/zerg/bin  # 换安装前缀（默认 <树>/bin）
#   bash scripts/svc/setup-zerg.sh --dry-run               # 只打印计划，零副作用
#
# 环境变量（与命令行等价；命令行优先）：
#   ZERG_SETUP_DIR ZERG_SETUP_REPO ZERG_SETUP_REF ZERG_SETUP_ROLE ZERG_SETUP_COMPONENTS
#   ZERG_PREFIX ZERG_API_BASE ZERG_SETUP_CONTROLLER ZERG_SETUP_NODE_NAME
#   ZERG_SETUP_LAUNCHCTL ZERG_SETUP_LAUNCHD_DIR ZERG_SETUP_CORE_LABEL
#   ZERG_SETUP_SYSTEMCTL ZERG_SETUP_UNIT_DIR ZERG_SETUP_AGENTD_UNIT ZERG_SETUP_WAIT_S
#
# 测试接缝（沙箱隔离真机用；生产别动）：
#   ZERG_SETUP_OS=darwin|linux        覆盖平台判定（白名单两值，仅用于验收本脚本的平台拒绝分支）
#   ZERG_SETUP_LAUNCHCTL / ZERG_SETUP_SYSTEMCTL  注入假命令（**绝不碰真机 launchd/systemd**）
#   ZERG_SETUP_SYSTEMCTL 被显式设置时，node 角色的 systemd 前置检查放行（供沙箱演练）
#
# 退出码：0 成功 / 1 构建或运行失败 / 4 用法或平台不允许 /
#         5 校验或身份对账失败 / 6 已有树不干净或不是检出 / 7 工具链不合
set -euo pipefail

# 公开镜像仓（匿名可拉；与 core 的 DefaultPublicRepoURL 同仓，可用 --repo 覆盖）
DEFAULT_REPO_URL='https://github.com/Mr2109/zerg-swarm.git'

# ── 入参默认值（可被命令行/环境覆盖）────────────────────────────────────────
TREE="${ZERG_SETUP_DIR:-$HOME/zerg-swarm}"
REPO_URL="${ZERG_SETUP_REPO:-$DEFAULT_REPO_URL}"
REF="${ZERG_SETUP_REF:-main}"
ROLE="${ZERG_SETUP_ROLE:-controller}"
PREFIX="${ZERG_PREFIX:-}"
COMPONENTS="${ZERG_SETUP_COMPONENTS:-}"
API="${ZERG_API_BASE:-http://127.0.0.1:8580}"
TOKEN_FILE="${ZERG_TOKEN_FILE:-$HOME/.zerg/token}"
CONTROLLER_URL="${ZERG_SETUP_CONTROLLER:-http://127.0.0.1:8580}"
NODE_NAME="${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo node)}"
LAUNCHCTL="${ZERG_SETUP_LAUNCHCTL:-launchctl}"
LAUNCHD_DIR="${ZERG_SETUP_LAUNCHD_DIR:-$HOME/Library/LaunchAgents}"
CORE_LABEL="${ZERG_SETUP_CORE_LABEL:-com.zerg.core}"
SYSTEMCTL="${ZERG_SETUP_SYSTEMCTL:-systemctl}"
UNIT_DIR="${ZERG_SETUP_UNIT_DIR:-/etc/systemd/system}"
AGENTD_UNIT="${ZERG_SETUP_AGENTD_UNIT:-zerg-agentd}"
WAIT="${ZERG_SETUP_WAIT_S:-30}"
SYSTEMCTL_EXPLICIT=0
[ -n "${ZERG_SETUP_SYSTEMCTL:-}" ] && SYSTEMCTL_EXPLICIT=1
DRY=0; NO_SERVICE=0; NO_UI=0

usage() { sed -n '2,60p' "$0"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dir)        TREE="$2"; shift 2 ;;
    --repo)       REPO_URL="$2"; shift 2 ;;
    --ref)        REF="$2"; shift 2 ;;
    --role)       ROLE="$2"; shift 2 ;;
    --prefix)     PREFIX="$2"; shift 2 ;;
    --components) COMPONENTS="$2"; shift 2 ;;
    --controller) CONTROLLER_URL="$2"; shift 2 ;;
    --node-name)  NODE_NAME="$2"; shift 2 ;;
    --unit-dir)   UNIT_DIR="$2"; shift 2 ;;
    --dry-run)    DRY=1; shift ;;
    --no-service) NO_SERVICE=1; shift ;;
    --no-ui)      NO_UI=1; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 64 ;;
  esac
done

say()  { printf '%s\n' "$*"; }
warn() { printf '⚠️  %s\n' "$*" >&2; }
die()  { printf '❌ %s\n' "$1" >&2; exit "${2:-1}"; }
short(){ printf '%s' "$1" | cut -c1-12; }
has_comp() { case ",${1}," in *",${2},"*) return 0 ;; *) return 1 ;; esac; }

# ── 平台判定（ZERG_SETUP_OS 是白名单测试接缝，防误用改真机目标）──────────────
if [ -n "${ZERG_SETUP_OS:-}" ]; then
  case "$ZERG_SETUP_OS" in
    darwin|linux) TARGET_OS="$ZERG_SETUP_OS" ;;
    *) die "ZERG_SETUP_OS 只接受 darwin / linux（得 ${ZERG_SETUP_OS}）" 4 ;;
  esac
else
  case "$(uname -s)" in
    Darwin) TARGET_OS=darwin ;;
    Linux)  TARGET_OS=linux ;;
    *)      die "本版只支持 macOS（darwin）/ Linux——当前 $(uname -s)" 4 ;;
  esac
fi

# ── 角色 → 组件集（与 zerg-upgrade.sh resolve_components 同口径）─────────────
resolve_components() {
  local raw="$COMPONENTS" list out c seen=" "
  if [ -n "$raw" ]; then
    list="$(printf '%s' "$raw" | tr 'A-Z' 'a-z')"
  elif [ "$ROLE" = node ]; then
    list='core,agentd'
  else
    list='core,agent'
    [ "$TARGET_OS" = darwin ] && [ "$NO_UI" != 1 ] && list='core,agent,ui'
  fi
  local parts
  parts=()
  IFS=',' read -r -a parts <<< "$list"
  for c in "${parts[@]}"; do
    c="$(printf '%s' "$c" | tr -d '[:space:]')"
    [ -n "$c" ] || continue
    case "$c" in
      core|agent|agentd) ;;
      ui)
        [ "$TARGET_OS" = darwin ] || die 'ui 只在 Mac（darwin）编译——设计 §3：UI 只出 Mac' 4
        [ "$ROLE" = node ] && die 'node 角色不含 ui——UI 仅在主控机（Mac）' 4 ;;
      *) die "未知组件 ${c}（可选：core/agent/agentd/ui）" 4 ;;
    esac
    case "$seen" in *" $c "*) continue ;; esac
    seen="$seen$c "
    out="${out:+$out,}$c"
  done
  [ -n "$out" ] || die "组件集为空（--components ${raw}）" 4
  printf '%s' "$out"
}
case "$ROLE" in
  controller|node) ;;
  *) die "未知角色 ${ROLE}（可选：controller/node）" 4 ;;
esac
COMP_LIST="$(resolve_components)"
[ -n "$PREFIX" ] || PREFIX="$TREE/bin"

# ── 服务前置检查（在动手之前就说清楚，别装完才发现服务起不来）───────────────
if [ "$NO_SERVICE" != 1 ]; then
  if [ "$ROLE" = controller ] && [ "$TARGET_OS" != darwin ]; then
    die 'controller 角色的服务托管走 launchd，仅 macOS 支持——Linux 主控请加 --no-service 后自行托管' 4
  fi
  if [ "$ROLE" = node ] && [ "$TARGET_OS" != linux ] && [ "$SYSTEMCTL_EXPLICIT" != 1 ]; then
    die 'node 角色的服务托管走 systemd，仅 Linux 支持（本机没有 systemd）——确实要演练请显式设 ZERG_SETUP_SYSTEMCTL' 4
  fi
fi

# ── ① 取码 ───────────────────────────────────────────────────────────────────
take_source() {
  if [ -d "$TREE/.git" ]; then
    if [ -n "$(git -C "$TREE" status --porcelain 2>/dev/null)" ]; then
      die "已有树不干净（有未提交改动）：${TREE} —— 拒绝静默弄脏/覆盖，请先提交或清理，或换个 --dir" 6
    fi
    say "📥 已有检出：${TREE}（干净）—— fetch 后按需对齐 ${REF}"
    git -C "$TREE" fetch --depth 1 origin "$REF" || die "fetch 失败（离线 / 非法 ref ${REF}）" 1
    local cur tgt
    cur="$(git -C "$TREE" rev-parse HEAD)"
    tgt="$(git -C "$TREE" rev-parse FETCH_HEAD 2>/dev/null || echo '')"
    if [ -z "$tgt" ]; then
      warn '取不到 FETCH_HEAD —— 保持现状不动'
    elif [ "$cur" = "$tgt" ]; then
      say "   ✅ 已在 ${REF}@$(short "$tgt")，未改动工作树"
    else
      say "   ↻ 对齐到 ${REF}@$(short "$tgt")（$(short "$cur") → $(short "$tgt")）"
      git -C "$TREE" reset --hard FETCH_HEAD || die 'reset 失败' 1
    fi
    return 0
  fi
  if [ -e "$TREE" ]; then
    die "目标已存在且不是 git 检出：${TREE} —— 拒绝覆盖（换个 --dir，或先手工清理）" 6
  fi
  say "📥 clone ${REPO_URL} @ ${REF} → ${TREE}"
  mkdir -p "$(dirname "$TREE")"
  git clone --depth 1 --branch "$REF" "$REPO_URL" "$TREE" || die "clone 失败：${REPO_URL}（离线 / URL 错 / ref 错）" 1
}

# ── ② 工具链核验（单一真源 = core/go.mod 的 go 指令；语义同 check-gotoolchain.py）──
gomod_pin() { # $1 = go.mod 路径 → 形如 go1.25.5；读不到则空
  python3 - "$1" <<'PY'
import re, sys
try:
    txt = open(sys.argv[1], encoding="utf-8").read()
except Exception:
    print("")
    raise SystemExit
m = re.search(r"^go\s+([0-9][0-9.]*)\s*$", txt, re.M)
print("go" + m.group(1) if m else "")
PY
}
local_go_version() { # → 形如 go1.26.4；没有 go 则空
  command -v go >/dev/null 2>&1 || { echo ''; return 0; }
  go version 2>/dev/null | awk '{print $3}'
}
go_at_least() { # $1 = 本机 actual，$2 = 钉住 pin
  python3 - "$1" "$2" <<'PY'
import re, sys
def trip(v):
    m = re.match(r"^go?(\d+)\.(\d+)(?:\.(\d+))?$", (v or "").strip())
    if not m:
        return None
    return tuple(int(m.group(i) or 0) for i in (1, 2, 3))
a, p = trip(sys.argv[1]), trip(sys.argv[2])
sys.exit(0 if (a and p and a >= p) else 1)
PY
}
toolchain_guidance() { # $1 = pin
  printf '%s\n' "   安装指引（本脚本绝不自行升级/安装工具链）："
  printf '     · macOS ：brew install go（或 brew upgrade go）\n'
  printf '     · Linux ：从 https://go.dev/dl/ 下载 %s 或更高，解到 /usr/local/go 并加入 PATH\n' "$1"
  printf '     · 版本管理器：goenv install / g install 对应版本后重跑本脚本\n'
}
check_toolchain() {
  local gomod="$TREE/core/go.mod" pin actual
  [ -f "$gomod" ] || die "不是源码树（缺 ${gomod}）——取码失败或 --dir 指错" 7
  pin="$(gomod_pin "$gomod")"
  [ -n "$pin" ] || die "读不到 ${gomod} 的 go 指令（钉住的单一真源）" 7
  actual="$(local_go_version)"
  if [ -z "$actual" ]; then
    warn '本机找不到 go —— 无法构建'
    toolchain_guidance "$pin"
    exit 7
  fi
  say "🧰 工具链：钉住 ${pin} · 本机 ${actual}（单一真源 = core/go.mod 的 go 指令）"
  if ! go_at_least "$actual" "$pin"; then
    warn "工具链不合：本机 ${actual} 低于钉住的 ${pin} —— 拒绝构建（同 commit 不同工具链会编出不同二进制，G6）"
    toolchain_guidance "$pin"
    exit 7
  fi
  # 两模块钉住一致性（core/agent）——门禁脚本若在树里就顺手跑一次
  if [ -f "$TREE/scripts/gates/check-gotoolchain.py" ]; then
    python3 "$TREE/scripts/gates/check-gotoolchain.py" "$TREE" >/dev/null 2>&1 \
      || die 'core/agent 两模块的工具链钉住不一致（scripts/gates/check-gotoolchain.py 未过）' 7
    say '   ✅ 两模块工具链钉住一致（check-gotoolchain.py）'
  fi
  return 0
}

# ── ③ 构建（仓库自己的入口；**绝不手写 go build**）───────────────────────────
build_components() {
  local script="$TREE/scripts/build/build-all.sh" args
  args=()
  [ -f "$script" ] || die "源码树缺 scripts/build/build-all.sh —— 拒绝用手写 go build 顶替（构建必须走仓库自己的入口）" 5
  if ! has_comp ui "$COMP_LIST"; then args=(--no-ui); fi
  say "🔨 构建（bash scripts/build/build-all.sh${args:+ ${args[*]}}：身份注入 -ldflags + ad-hoc 重签）"
  ( cd "$TREE" && bash scripts/build/build-all.sh ${args[@]+"${args[@]}"} ) || die '构建失败（scripts/build/build-all.sh 退出非零）' 1
}

# ── ④ 原子安装（.new → mv；旧件留 .prev；macOS 重签名）───────────────────────
# 组件名 → 仓库 build-all.sh 的产物名（bin/zerg-core / bin/zerg-agent / …）
artifact_for() {
  case "$1" in
    core)   echo zerg-core ;;
    agent)  echo zerg-agent ;;
    agentd) echo zerg-agentd ;;
    ui)     echo zerg-ui ;;
    *)      echo '' ;;
  esac
}
install_one() { # $1 = 组件名
  local comp="$1" art src dst
  art="$(artifact_for "$comp")"
  src="$TREE/bin/$art"; dst="$PREFIX/$art"
  [ -f "$src" ] || die "构建产物缺失：${src}（组件 ${comp} 无产物，半成品不装）" 5
  cp -p "$src" "$PREFIX/${art}.new" || die "暂存失败：${PREFIX}/${art}.new" 1
  if [ -f "$dst" ]; then mv -f "$dst" "$PREFIX/${art}.prev"; fi
  mv -f "$PREFIX/${art}.new" "$dst"
  chmod +x "$dst"
  if [ "$TARGET_OS" = darwin ] && command -v codesign >/dev/null 2>&1; then
    codesign -s - --force "$dst" >/dev/null 2>&1 || warn "${comp} 重签名失败（未签名二进制会被秒杀）"
  fi
  say "   🔁 ${comp} ← bin/${art}（旧件留 .prev）"
}
install_components() {
  mkdir -p "$PREFIX"
  local c
  for c in $(printf '%s' "$COMP_LIST" | tr ',' ' '); do install_one "$c"; done
}

# ── ⑤ 服务注册（幂等：已存在则重写 + reload/kickstart）──────────────────────
DAEMON="$TREE/scripts/svc/zerg-core-daemon.sh"
write_core_plist() {
  local plist="$LAUNCHD_DIR/${CORE_LABEL}.plist" dom
  [ -f "$DAEMON" ] || die "缺仓库自带的启动包装 ${DAEMON}（它负责源 .env 与补 PATH）" 5
  mkdir -p "$LAUNCHD_DIR"
  cat > "$plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>${CORE_LABEL}</string>
	<key>ProgramArguments</key>
	<array><string>/bin/bash</string><string>${DAEMON}</string></array>
	<key>WorkingDirectory</key><string>${TREE}</string>
	<key>KeepAlive</key><true/>
	<key>RunAtLoad</key><true/>
	<key>ThrottleInterval</key><integer>10</integer>
	<key>StandardOutPath</key><string>/tmp/zerg-core.log</string>
	<key>StandardErrorPath</key><string>/tmp/zerg-core.log</string>
	<key>EnvironmentVariables</key>
	<dict><key>PATH</key><string>/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
</dict>
</plist>
PLIST
  if command -v plutil >/dev/null 2>&1; then
    plutil -lint "$plist" >/dev/null 2>&1 || die "生成的 plist 语法错误：${plist}" 5
  fi
  dom="gui/$(id -u)"
  if "$LAUNCHCTL" print "${dom}/${CORE_LABEL}" >/dev/null 2>&1; then
    "$LAUNCHCTL" kickstart -k "${dom}/${CORE_LABEL}" >/dev/null 2>&1 || warn 'kickstart 失败（主控可能仍是旧进程）'
    say "   🔄 已重写 plist 并重启托管（kickstart -k ${CORE_LABEL}）"
  else
    "$LAUNCHCTL" bootstrap "$dom" "$plist" >/dev/null 2>&1 \
      || "$LAUNCHCTL" load -w "$plist" >/dev/null 2>&1 \
      || warn 'launchctl bootstrap/load 失败——请手工检查'
    say "   🚀 已安装并启动托管（${CORE_LABEL}）"
  fi
}
write_agentd_unit() {
  local unit="$UNIT_DIR/${AGENTD_UNIT}.service"
  mkdir -p "$UNIT_DIR"
  cat > "$unit" <<UNIT
[Unit]
Description=Zerg Swarm agent daemon (zerg-agentd)
After=network-online.target

[Service]
Type=simple
EnvironmentFile=-${TREE}/.env
Environment=ZERG_NODE_NAME=${NODE_NAME}
Environment=PATH=/usr/local/bin:/usr/bin:/bin
ExecStart=${PREFIX}/zerg-agentd --controller ${CONTROLLER_URL} --machine ${NODE_NAME}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
  "$SYSTEMCTL" daemon-reload >/dev/null 2>&1 || true
  "$SYSTEMCTL" enable --now "$AGENTD_UNIT" >/dev/null 2>&1 \
    || "$SYSTEMCTL" restart "$AGENTD_UNIT" >/dev/null 2>&1 \
    || warn "systemctl 启动 ${AGENTD_UNIT} 失败——请手工检查"
  say "   🚀 已注册并启动 ${AGENTD_UNIT}.service（ExecStart = ${PREFIX}/zerg-agentd）"
}

# ── ⑥ 身份对账（运行进程自报 vs 安装件 vs 代码树）────────────────────────────
sha_match() {
  [ -n "$1" ] || return 1
  case "$1" in "$2"*) return 0 ;; esac
  case "$2" in "$1"*) return 0 ;; esac
  return 1
}
disk_identity() { # $1 = 二进制 → 自报 commit（短 sha）
  [ -x "$1" ] || { echo ''; return 0; }
  "$1" --version 2>/dev/null | awk '{print $3}' | sed 's/+.*//'
}
tree_head() { git -C "$TREE" rev-parse HEAD 2>/dev/null || echo ''; }
live_code_sha() {
  local raw='' hdr
  hdr=()
  [ -f "$TOKEN_FILE" ] && hdr=(-H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null || true)")
  raw="$(curl -s -m 3 ${hdr[@]+"${hdr[@]}"} "$API/api/capabilities" 2>/dev/null || true)"
  [ -n "$raw" ] || return 0
  printf '%s' "$raw" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: raise SystemExit
print(d.get("code_sha") or "")' 2>/dev/null || true
}
verify_controller() {
  local tree disk got t=0
  tree="$(tree_head)"; disk="$(disk_identity "$PREFIX/zerg-core")"
  say "   安装件 zerg-core 自报 : ${disk:-<读不到>}"
  say "   代码树 HEAD           : $(short "$tree")"
  [ -n "$disk" ] || die "读不到安装件 zerg-core 的自报身份（--version）" 5
  if [ -n "$tree" ] && ! sha_match "$disk" "$tree"; then
    die "安装件与代码树不一致：disk=${disk} tree=$(short "$tree")" 5
  fi
  say "   等运行进程自报（${API}/api/capabilities，最多 ${WAIT}s）…"
  while [ "$t" -lt "$WAIT" ]; do
    got="$(live_code_sha)"
    [ -n "$got" ] && break
    sleep 1; t=$((t + 1))
  done
  if [ -z "$got" ]; then
    die "运行中的主控未自报 code_sha（读不到 ${API}/api/capabilities）——无法证明活进程已换版" 5
  fi
  if ! sha_match "$got" "$disk"; then
    die "身份对账失败：运行进程自报 ${got} ≠ 安装件 ${disk}（换了盘、没换活进程）" 5
  fi
  say "   ✅ 运行进程自报 code_sha=${got} == 安装件"
}
verify_node() {
  local tree disk
  tree="$(tree_head)"; disk="$(disk_identity "$PREFIX/zerg-agentd")"
  say "   安装件 zerg-agentd 自报 : ${disk:-<读不到>}"
  say "   代码树 HEAD              : $(short "$tree")"
  [ -n "$disk" ] || die "读不到 zerg-agentd 的自报身份（--version；linux 件需在目标机上读）" 5
  if [ -n "$tree" ] && ! sha_match "$disk" "$tree"; then
    die "身份对账失败：安装件 ${disk} ≠ 代码树 $(short "$tree")" 5
  fi
  if [ "$NO_SERVICE" != 1 ]; then
    if "$SYSTEMCTL" is-active "$AGENTD_UNIT" >/dev/null 2>&1; then
      say "   ✅ 服务 ${AGENTD_UNIT} = active"
    else
      die "服务 ${AGENTD_UNIT} 未 active——bootstrap 未完成（装了件但服务没起来）" 5
    fi
  fi
  say '   ✅ 落盘自报 == 代码树（节点侧再以主控版本矩阵的心跳 code_sha 收口口径）'
}

# ══════════════════════════════════════════════════════════════════════════════
# dry-run：只打印计划，零副作用（不 clone、不构建、不写文件、不碰服务）
# ══════════════════════════════════════════════════════════════════════════════
if [ "$DRY" = 1 ]; then
  say '🧪 dry-run：只打印计划，零副作用（不 clone / 不构建 / 不写文件 / 不碰服务）'
  say "   角色     : ${ROLE}（组件 ${COMP_LIST}）"
  if [ -d "$TREE/.git" ]; then
    say "   取码     : 已有检出 ${TREE} —— fetch ${REF} 后按需对齐（脏树会拒绝）"
  elif [ -e "$TREE" ]; then
    say "   取码     : ❌ ${TREE} 已存在且不是 git 检出（真跑会拒绝）"
  else
    say "   取码     : git clone --depth 1 --branch ${REF} ${REPO_URL} ${TREE}"
  fi
  say "   工具链   : 核验本机 go >= core/go.mod 的 go 指令（不合则拒绝并给安装指引）"
  say "   构建     : bash ${TREE}/scripts/build/build-all.sh$([ "$NO_UI" = 1 ] && echo ' --no-ui')"
  say "   安装     : ${PREFIX}（.new → mv 原子换装，旧件留 .prev；macOS 重签）"
  if [ "$NO_SERVICE" = 1 ]; then
    say '   服务     : 不触碰（--no-service）'
  elif [ "$ROLE" = controller ]; then
    say "   服务     : 写 ${LAUNCHD_DIR}/${CORE_LABEL}.plist（KeepAlive/RunAtLoad/ThrottleInterval）"
    say "              再 bootstrap 或 kickstart -k（已存在则重写并重启）"
  else
    say "   服务     : 写 ${UNIT_DIR}/${AGENTD_UNIT}.service（ExecStart=${PREFIX}/zerg-agentd）"
    say "              再 daemon-reload + enable --now（已存在则重写并重启）"
  fi
  if [ "$ROLE" = node ]; then
    say "   验证     : zerg-agentd --version 自报 == 安装件 == 代码树 HEAD$( [ "$NO_SERVICE" = 1 ] || echo ' + 服务 is-active' )"
  else
    say "   验证     : ${API}/api/capabilities 的 code_sha（运行进程自报）== 安装件 == 代码树 HEAD"
  fi
  say "   收尾     : 写 ${TREE}/.install_method = git（源码检出印记）"
  # 只读的顺手核验（树已在时）
  if [ -f "$TREE/core/go.mod" ]; then
    pin="$(gomod_pin "$TREE/core/go.mod")"
    act="$(local_go_version)"
    say "   （只读核验）工具链：钉住 ${pin:-?} · 本机 ${act:-（无 go）}"
  fi
  exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
say '══ setup-zerg：源码式虫族 bootstrap ══'
say "   角色 ${ROLE} · 组件 ${COMP_LIST} · 平台 ${TARGET_OS} · 前缀 ${PREFIX}"
say ''
say '① 取码'
take_source
say ''
say '② 工具链核验'
check_toolchain
say ''
say '③ 构建'
build_components
say ''
say '④ 原子安装'
install_components
say ''
if [ "$NO_SERVICE" = 1 ]; then
  say '⑤ 服务：跳过（--no-service）'
else
  say '⑤ 服务注册'
  if [ "$ROLE" = controller ]; then write_core_plist; else write_agentd_unit; fi
fi
say ''
say '⑥ 身份对账（运行进程自报 vs 安装件 vs 代码树）'
if [ "$NO_SERVICE" = 1 ]; then
  if [ "$ROLE" = node ]; then
    tree="$(tree_head)"; disk="$(disk_identity "$PREFIX/zerg-agentd")"
    say "   安装件 zerg-agentd 自报 : ${disk:-<读不到>}"
    [ -n "$disk" ] || die '读不到 zerg-agentd 的自报身份' 5
    [ -z "$tree" ] || sha_match "$disk" "$tree" || die "身份对账失败：安装件 ${disk} ≠ 代码树 $(short "$tree")" 5
    say '   （--no-service：跳过服务 is-active 与运行进程核对）'
  else
    tree="$(tree_head)"; disk="$(disk_identity "$PREFIX/zerg-core")"
    say "   安装件 zerg-core 自报 : ${disk:-<读不到>}"
    [ -n "$disk" ] || die '读不到安装件 zerg-core 的自报身份' 5
    [ -z "$tree" ] || sha_match "$disk" "$tree" || die "安装件与代码树不一致：disk=${disk} tree=$(short "$tree")" 5
    say '   （--no-service：跳过「运行进程自报」的核对——无服务可查）'
  fi
elif [ "$ROLE" = node ]; then
  verify_node
else
  verify_controller
fi
say ''
printf 'git\n' > "$TREE/.install_method" || warn "写 ${TREE}/.install_method 失败"
say "⑦ 收尾：已写 ${TREE}/.install_method = git"
say ''
say "✅ bootstrap 完成：${ROLE}（组件 ${COMP_LIST}）→ ${PREFIX}"
if [ "$ROLE" = node ]; then
  say "   下一步：回到主控跑 \`zerg update\`（该机已可自更新）"
else
  say '   下一步：有新版时跑 `zerg update`（或主控机 `zerg-core update`）即可源码式自更新'
fi
exit 0

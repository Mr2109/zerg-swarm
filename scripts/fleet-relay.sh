#!/usr/bin/env bash
# fleet-relay.sh — 主控镜像中继（B5.1，2026-09-13 Mr2109 拍板）
#
# 为什么有它：源码式更新（B5）要求**每台机器自己取源码**，而实测节点（X3）到不了 github.com
# （`github.com` → 000，`codeload.github.com` → 301），主控（Mac）能到。
# 定案 = **主控做镜像中继**（见 docs/01-设计/设计-源码式发布与升级.md §二十）：
#   主控把本地**镜像仓**的 main 推到每台节点上的一个**裸仓**，节点从自己的裸仓取源码
#   ⇒ 节点**无需出网、也无需 root**（裸仓在该机家目录/检出目录旁）。
#
# 做的事（对本地镜像仓**只读**）：
#   ① 解析名册（ZERG_FLEET_NODES 或 gateway/fleet.yaml 的 updates.nodes——与 `--fleet` 同一份）
#   ② 逐台：确保该机上的裸仓存在（缺则 `git init --bare`——只为「没有」而建，不覆盖已有仓）
#   ③ 逐台：`git push <ssh目标>:<裸仓路径> refs/heads/main:refs/heads/main`
#      （幂等：同 sha 再跑 = up-to-date，不产生新对象、不改引用）
#   ④ 逐台：**回读验证**——在该机上 `git -C <裸仓> rev-parse refs/heads/main`，与源 sha 逐台打印
#   ⑤ 任何一台失败（不可达 / 权限不对 / 回读不等）⇒ 收尾**逐台点名**并 exit 1（不默默跳过）
#
# 用法：
#   bash scripts/fleet-relay.sh --from /tmp/zerg-mirror-20260913          # 同步名册全部节点
#   bash scripts/fleet-relay.sh --from <镜像仓> --node x3                  # 只同步某一台
#   bash scripts/fleet-relay.sh --from <镜像仓> --dry-run                  # 只打印计划（零副作用：不 ssh、不推送）
#   bash scripts/fleet-relay.sh --from <镜像仓> --to-path <该机上的路径>    # 覆盖目标裸仓路径
#   bash scripts/fleet-relay.sh --from <镜像仓> --force                    # 节点上的 main 被重写过（分叉）时才用
#
# 目标裸仓路径（优先级）：--to-path ＞ 名册里的 relay=<路径> ＞ 节点检出目录的父目录（通常即该机家目录）
# 下的 `zerg-relay.git`。与 `--fleet` 的节点步**同一口径**（同一份解析：`zerg-upgrade.sh --fleet --roster`）。
#
# 环境：
#   ZERG_FLEET_NODES       机群名册（env，优先；格式同 zerg-upgrade.sh）
#   ZERG_FLEET_YAML        名册 YAML 路径（默认 <repo>/gateway/fleet.yaml）
#   ZERG_RELAY_SSH         ssh 命令（默认 ssh；沙箱注入**假 ssh**——绝不连真机）
#   ZERG_RELAY_SSH_OPTS    ssh 附加参数（默认 '-o BatchMode=yes -o ConnectTimeout=8'）
#   ZERG_RELAY_REF         要同步的分支（默认 main）
#
# 退出码：0 全部同步且回读一致 / 1 有节点失败（逐台点名）/ 2 源仓/名册不可用 / 64 用法错误
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KERNEL="$REPO_ROOT/scripts/zerg-upgrade.sh"
REF="${ZERG_RELAY_REF:-main}"
FROM=""
TO_PATH=""
ONLY_NODE=""
DRY_RUN=0
FORCE=0
SSH_CMD="${ZERG_RELAY_SSH:-ssh}"
SSH_OPTS="${ZERG_RELAY_SSH_OPTS:--o BatchMode=yes -o ConnectTimeout=8}"

while [ $# -gt 0 ]; do
  case "$1" in
    --from)    [ $# -ge 2 ] || { printf '❌ --from 需要一个目录参数\n' >&2; exit 64; }; FROM="$2"; shift 2 ;;
    --to-path) [ $# -ge 2 ] || { printf '❌ --to-path 需要一个路径参数\n' >&2; exit 64; }; TO_PATH="$2"; shift 2 ;;
    --node)    [ $# -ge 2 ] || { printf '❌ --node 需要一个机器名\n' >&2; exit 64; }; ONLY_NODE="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --force)   FORCE=1; shift ;;
    -h|--help) sed -n '2,35p' "$0"; exit 0 ;;
    *) printf '❌ 未知参数: %s\n' "$1" >&2; exit 64 ;;
  esac
done

say() { printf '%s\n' "$*"; }
die() { printf '❌ %s\n' "$1" >&2; exit "${2:-1}"; }
sha12() { printf '%s' "$1" | cut -c1-12; }
first_line() { printf '%s' "$1" | sed -n '1p'; }

say "🔁 主控镜像中继（节点从自己的裸仓取源码——节点无需出网、无需 root）"
[ -n "$FROM" ] || die "--from 缺镜像仓目录（例：--from /tmp/zerg-mirror-20260913）" 64
[ -d "$FROM" ] || die "--from 目录不存在：${FROM}" 2
FROM="$(cd "$FROM" && pwd)"
git -C "$FROM" rev-parse --git-dir >/dev/null 2>&1 || die "--from 不是 git 仓：${FROM}" 2
# 私有权威仓本身**不是**镜像：误把它当中继源会把私有树（docs/01-设计 等）推到节点上。
# （连它的子目录一起拦——`git -C <子目录>` 会解析到同一个工作树。）
SRC_TOP="$(git -C "$FROM" rev-parse --show-toplevel 2>/dev/null || true)"
if [ -n "$SRC_TOP" ] && [ "$SRC_TOP" = "$REPO_ROOT" ]; then
  if [ "$FROM" = "$REPO_ROOT" ]; then
    die "--from 指向私有权威仓本身（${REPO_ROOT}）——中继源应当是**镜像仓**产出（publish/mirror-public.sh --out <dir>）" 2
  fi
  die "--from 落在私有权威仓的工作树里（${FROM}）——中继源应当是**镜像仓**产出（publish/mirror-public.sh --out <dir>）" 2
fi
SRC_SHA="$(git -C "$FROM" rev-parse "refs/heads/${REF}" 2>/dev/null)" || SRC_SHA=""
[ -n "$SRC_SHA" ] || die "源仓没有 refs/heads/${REF}：${FROM}" 2
SRC_COUNT="$(git -C "$FROM" rev-list --count "refs/heads/${REF}")"
SRC_DIRTY="$(git -C "$FROM" status --porcelain | wc -l | tr -d ' ')"
say "   源   : ${FROM}"
say "          ${REF}=$(sha12 "$SRC_SHA") · ${SRC_COUNT} 笔 · 工作树改动 ${SRC_DIRTY} 项（中继只读它）"
say "   ssh  : ${SSH_CMD} ${SSH_OPTS}"
say ""

# ── 名册：与 `--fleet` 同一份解析（单一真源 = zerg-upgrade.sh --fleet --roster）─────────────
ROSTER_RC=0
ROSTER="$(bash "$KERNEL" --fleet --roster 2>&1)" || ROSTER_RC=$?
[ "$ROSTER_RC" = "0" ] || die "读名册失败（bash scripts/zerg-upgrade.sh --fleet --roster 退出码 ${ROSTER_RC}）：$(first_line "$ROSTER")" 2
# 第 2 列（ssh 目标）为 '-' 的是本机（controller）行——中继只对**节点**做功
NODES="$(printf '%s\n' "$ROSTER" | awk -F'\t' 'NF >= 9 && $2 != "-"')"
[ -n "$NODES" ] || die "名册里没有节点：设 ZERG_FLEET_NODES，或在 gateway/fleet.yaml 的 updates.nodes 里列出" 2
N_NODES="$(printf '%s\n' "$NODES" | grep -c . || true)"

if [ -n "$ONLY_NODE" ]; then
  ALL_NAMES="$(printf '%s\n' "$NODES" | awk -F'\t' '{printf " %s ", $1}')"
  case "$ALL_NAMES" in
    *" $ONLY_NODE "*) ;;
    *) die "--node ${ONLY_NODE} 不在名册里（名册：${ALL_NAMES}）" 2 ;;
  esac
  say "   范围 : 只同步 ${ONLY_NODE}（名册共 ${N_NODES} 台）"
else
  say "   范围 : 名册全部节点（${N_NODES} 台）"
fi
say ""

if [ "$DRY_RUN" = "1" ]; then
  say "计划（--dry-run：不 ssh、不推送、不建仓——零副作用）:"
  while IFS=$'\t' read -r n_name n_ssh n_root n_prefix n_api n_comp n_role n_rc n_relay; do
    [ -n "$n_name" ] || continue
    [ -n "$ONLY_NODE" ] && [ "$ONLY_NODE" != "$n_name" ] && continue
    tp="$n_relay"
    [ -n "$TO_PATH" ] && tp="$TO_PATH"
    [ -n "$tp" ] || tp="$(dirname "$n_root")/zerg-relay.git"
    say "  · ${n_name}  ${n_ssh}:${tp}  ← ${REF} $(sha12 "$SRC_SHA")（${SRC_COUNT} 笔）"
  done <<< "$NODES"
  say ""
  say "（dry-run：未连任何机器、未推送任何字节）"
  exit 0
fi

# ── 执行：逐台（建仓 → 推送 → 回读验证 → 逐台打印）────────────────────────────
N_OK=0
N_FAIL=0
FAILED_NAMES=""
FAILED_DETAIL=""
while IFS=$'\t' read -r n_name n_ssh n_root n_prefix n_api n_comp n_role n_rc n_relay; do
  [ -n "$n_name" ] || continue
  [ -n "$ONLY_NODE" ] && [ "$ONLY_NODE" != "$n_name" ] && continue
  tp="$n_relay"
  [ -n "$TO_PATH" ] && tp="$TO_PATH"
  [ -n "$tp" ] || tp="$(dirname "$n_root")/zerg-relay.git"

  say "  ▶ ${n_name}  ${n_ssh}:${tp}"
  # ① 确保裸仓存在（只为「不存在」而建；已有的仓一个字节都不动）
  ensure="mkdir -p '$(dirname "$tp")' && { [ -d '${tp}/objects' ] && echo RELAY-EXISTS || { git init --bare -q '${tp}' && echo RELAY-CREATED; }; }"
  erc=0
  eout="$("$SSH_CMD" ${SSH_OPTS} "$n_ssh" "$ensure" 2>&1)" || erc=$?
  if [ "$erc" != "0" ]; then
    say "     ❌ ${n_name} 建仓/连接失败（ssh rc=${erc}）：$(first_line "$eout")"
    N_FAIL=$((N_FAIL + 1)); FAILED_NAMES="${FAILED_NAMES} ${n_name}"; FAILED_DETAIL="${FAILED_DETAIL}${n_name}(不可达或权限不对：$(first_line "$eout")) "
    continue
  fi
  # 判定只认**整行**的标记：真机的 ssh 横幅 / `ssh -v` 的回显会把整条命令（连这些字面量）打出来
  verdict="$(printf '%s\n' "$eout" | grep -E '^RELAY-(EXISTS|CREATED)$' | tail -1 || true)"
  case "$verdict" in
    RELAY-CREATED) say "     · 裸仓不存在 ⇒ 已 git init --bare" ;;
    RELAY-EXISTS)  say "     · 裸仓已存在（沿用）" ;;
    *) say "     ⚠️ 远端未报告建仓结果（原样回显）：$(first_line "$eout")" ;;
  esac

  # ② 推送（幂等：同 sha ⇒ up-to-date，零新对象）
  FORCE_FLAG=""
  [ "$FORCE" = "1" ] && FORCE_FLAG="--force"
  prc=0
  pout="$(cd "$FROM" && GIT_SSH_COMMAND="$SSH_CMD $SSH_OPTS" git push $FORCE_FLAG "$n_ssh:$tp" "refs/heads/${REF}:refs/heads/${REF}" 2>&1)" || prc=$?
  if [ "$prc" != "0" ]; then
    say "     ❌ ${n_name} 推送失败（git rc=${prc}）：$(first_line "$pout")"
    N_FAIL=$((N_FAIL + 1)); FAILED_NAMES="${FAILED_NAMES} ${n_name}"; FAILED_DETAIL="${FAILED_DETAIL}${n_name}(推送失败：$(first_line "$pout")) "
    continue
  fi
  case "$pout" in
    *up-to-date*) say "     · 已是最新（up-to-date——未推送任何对象）" ;;
    *)            say "     · 已推送 $(sha12 "$SRC_SHA")" ;;
  esac

  # ③ 回读验证：在该机上读它自己的 refs/heads/main，必须 == 源 sha
  rrc=0
  rb_raw="$("$SSH_CMD" ${SSH_OPTS} "$n_ssh" "git -C '${tp}' rev-parse refs/heads/${REF}" 2>&1)" || rrc=$?
  # 只认 sha（真机的 ssh 横幅 / 回显会混进输出）：先取**整行**的 40 位 sha，再退一步在整段输出里找
  rb="$(printf '%s\n' "$rb_raw" | grep -E '^[0-9a-f]{40}$' | tail -1 || true)"
  if [ -z "$rb" ]; then
    rb="$(printf '%s\n' "$rb_raw" | grep -oE '[0-9a-f]{40}' | tail -1 || true)"
  fi
  if [ "$rrc" != "0" ]; then
    say "     ❌ ${n_name} 回读失败（ssh rc=${rrc}）：$(first_line "$rb_raw")"
    N_FAIL=$((N_FAIL + 1)); FAILED_NAMES="${FAILED_NAMES} ${n_name}"; FAILED_DETAIL="${FAILED_DETAIL}${n_name}(回读失败) "
    continue
  fi
  if [ "$rb" = "$SRC_SHA" ]; then
    say "     ✅ 回读一致：refs/heads/${REF}=${rb}（= 源 $(sha12 "$SRC_SHA")）"
    N_OK=$((N_OK + 1))
  else
    say "     ❌ ${n_name} 回读不等：该机 ${REF}=${rb} ≠ 源 $(sha12 "$SRC_SHA")——不假装成功"
    N_FAIL=$((N_FAIL + 1)); FAILED_NAMES="${FAILED_NAMES} ${n_name}"; FAILED_DETAIL="${FAILED_DETAIL}${n_name}(回读 sha 不等:${rb}) "
  fi
done <<< "$NODES"

# ── 收尾：逐台点名（不默默跳过）──────────────────────────────────────────────
say ""
if [ "$N_FAIL" = "0" ]; then
  say "✅ 中继完成：${N_OK} 台回读一致（${REF}=$(sha12 "$SRC_SHA")）"
  say "   节点侧取源：ZERG_UPDATE_REMOTE=<该机裸仓路径>（「--fleet」的节点步会自动注入，见 zerg-upgrade.sh）"
  exit 0
fi
say "❌ 中继未完成：${N_OK} 台成功 / ${N_FAIL} 台失败"
say "   失败点名：${FAILED_NAMES# }"
say "   逐台原因：${FAILED_DETAIL% }"
say "   （节点上的 Git 版本/路径写权限/ssh 免密是三个常见原因；先单独 ssh 该机跑一次上面的建仓命令排障）"
exit 1

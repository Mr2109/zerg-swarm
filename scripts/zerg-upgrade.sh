#!/usr/bin/env bash
# zerg-upgrade.sh — 虫族本机升级器内核（L1）
#
# 设计要点（对齐 Hermes 的升级纪律，详见 docs/01-设计/设计-自动升级模块-20260911.md）：
#   · 六阶段：plan → drain → swap → restart → verify → report；每阶段只守一个真实失败类
#   · **自己不是被升级进程**：本脚本是独立短命进程，换装由它做，重启交给 launchd/启动脚本
#   · 停服务必须 bootout（主控 plist KeepAlive=true，直接 kill 会被旧二进制抢着重启）
#   · 换装原子：先写 .new 再 mv，旧件留 .prev（回滚只需 mv 回来）
#   · 校验不过就**拒绝**，且**不动已装文件**；装完起不来 → 自动回滚
#   · 每次运行都留回执（含失败路径）——回执存在于"失败时"才有意义
#
# 用法：
#   bash scripts/zerg-upgrade.sh --plan          # 只读盘点（对齐 hermes update --plan）
#   bash scripts/zerg-upgrade.sh --check         # 只问：有没有新版
#   bash scripts/zerg-upgrade.sh --status        # 四件版本矩阵（只读）
#   bash scripts/zerg-upgrade.sh                 # 执行升级（本机）
#   bash scripts/zerg-upgrade.sh --rollback      # 用 .prev 回滚
#   bash scripts/zerg-upgrade.sh --receipts      # 看最近回执
#   bash scripts/zerg-upgrade.sh --fleet --plan  # 机群：只盘点（名册 + 目标 + 矩阵，无副作用）
#   bash scripts/zerg-upgrade.sh --fleet         # 机群：每台**各自**跑源码式更新，最后核对版本矩阵
#   ... --allow-downgrade                        # 明确允许把代码换成更旧的提交（默认拒绝降级）
#   ... --role controller|node                   # 本机角色（默认 controller）
#   ... --components core,agent,agentd,ui        # 显式组件集（ui 仅 darwin；空 ⇒ 按角色推默认）
#
# 角色（B5，2026-09-13）：
#   controller = 主控机：core + agent（darwin 再加 ui）—— 服务只碰主控（+UI）
#   node       = 机群节点（X3 等）：core + agentd —— 该机上真正在跑的是 zerg-agentd（systemd），
#                没有主控服务可碰；core 装上是为了该机将来能自己 `zerg-core update`
#
# 取件源（B4 改造，2026-09-13）：
#   · **默认 = 本地 git 树构建产物**（`zerg update` fetch→本机构建→写到临时区，
#     用 ZERG_UPGRADE_SOURCE=file://<staging> 交本内核；内核只做校验+换装，构建不在这里）
#   · `--from-assets`（默认关）= 应急通道：从 GitHub Release 下载预编译资产（断网/工具链故障兜底）
#   两条通道的 manifest 同形（version/commit/tag/components/toolchain/artifacts[]）。
#
# 机群（B5，2026-09-13）：
#   `--fleet` 不再由主控交叉编译 + scp 推预编译制品；改为**每台机器各自跑源码式更新**：
#     本机   ：走本机的 `zerg update`（源码式）
#     远程机 ：ssh 上去，让**它自己** fetch→本机构建→交**它本地的**六阶段内核换装
#   主控只负责：① 编排（顺序、共享构建时间戳）② 核对版本矩阵（/api/fleet/status 三台 code_sha
#   一致且 = 目标）③ 回收各机回执（逐台一份 + 汇总一份）。
#   名册：ZERG_FLEET_NODES（env，优先）或 gateway/fleet.yaml 的 update.nodes（私有）；
#         每项 "name=ssh目标[,root=…,prefix=…,api=…,components=…,role=…]"。
#
# 开关/环境：
#   ZERG_UPGRADE_SOURCE=file:///path/to/staging   git 树构建产物目录（默认取件源）
#   --from-assets                                改用 GitHub Release 资产（应急，默认关）
#   ZERG_PREFIX=<dir>       安装前缀（默认 <repo>/bin）
#   --no-service            不碰 launchd/systemd/服务（沙箱测试用）
#   --no-ui                 不构建/换装 UI（快的自检；沙箱用）
#   --force                 有在途任务也照升（默认拒绝）
#   --json                  机器可读
#   ZERG_FLEET_NODES        机群更新名册（空格分隔的 name=ssh目标[,k=v…]）
#   ZERG_FLEET_LOCAL_CMD    本机 update 入口（默认 <prefix>/zerg-core；沙箱注入）
#   ZERG_FLEET_SSH          ssh 命令（默认 ssh；沙箱注入假 ssh——**绝不连真机**）
#   ZERG_FLEET_WAIT_S       矩阵收敛等待上限（默认 90 秒）
#   ZERG_FLEET_SKIP_MATRIX=1 跳过矩阵核对（逃生门；默认必须核对）
#   ZERG_AGENTD_UNIT        节点上的 agentd systemd 单元名（默认 x3-agent）
#   ZERG_START_AGENTD       节点上 agentd 的启停入口（默认 systemctl）——沙箱测试接缝
#   测试接缝（沙箱隔离真机，绝不误杀在跑的服务）：
#     ZERG_API_BASE / ZERG_START_CORE / ZERG_STOP_CORE / ZERG_START_UI / ZERG_UI_PATTERN / ZERG_UPGRADE_REPO
#     ZERG_UPGRADE_PLAT（白名单 darwin-arm64/linux-amd64，仅测试用——绝不用于真机构建）
#
# 退出码：0 成功 / 1 失败(已尝试回滚) / 2 无需升级 / 3 拒绝(有在途任务) / 4 校验不通过(未动文件)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${ZERG_UPGRADE_SOURCE:-}"
PREFIX="${ZERG_PREFIX:-$REPO_ROOT/bin}"
RECEIPTS="${ZERG_RECEIPTS_DIR:-$HOME/.zerg/update_receipts}"
API="${ZERG_API_BASE:-http://127.0.0.1:8580}"
TOKEN_FILE="$HOME/.zerg/token"
# git 副作用（降级门判祖先）作用在哪个检出上：默认本仓；沙箱指向玩具仓
GITREPO="${ZERG_UPGRADE_REPO:-$REPO_ROOT}"
# 服务启停入口（可注入——沙箱用假脚本，**绝不**去动真机的 launchd/UI）
STARTCORE="${ZERG_START_CORE:-$REPO_ROOT/scripts/start-zerg-core.sh}"
STARTUI="${ZERG_START_UI:-$REPO_ROOT/scripts/start-zerg-ui.sh}"
UI_PATTERN="${ZERG_UI_PATTERN:-bin/zerg-ui}"
# 节点上的 agentd 服务（B5；--role node）
AGENTD_UNIT="${ZERG_AGENTD_UNIT:-x3-agent}"
START_AGENTD="${ZERG_START_AGENTD:-}"

MODE="apply"; NO_SERVICE=0; FORCE=0; JSON=0; TAG=""; FLEET_PLAN_ONLY=0; ALLOW_DOWNGRADE=0
FROM_ASSETS=0; NO_UI=0; ROLE="controller"; COMPONENTS=""; PLAN_GIVEN=0
while [ $# -gt 0 ]; do
  case "$1" in
    --plan) PLAN_GIVEN=1; if [ "$MODE" = "fleet" ]; then FLEET_PLAN_ONLY=1; else MODE="plan"; fi; shift ;;
    --check) MODE="check"; shift ;;
    --status) MODE="status"; shift ;;
    --rollback) MODE="rollback"; shift ;;
    --receipts) MODE="receipts"; shift ;;
    --fleet) MODE="fleet"; [ "$PLAN_GIVEN" = "1" ] && FLEET_PLAN_ONLY=1; shift ;;
    --from-assets) FROM_ASSETS=1; shift ;;
    --allow-downgrade) ALLOW_DOWNGRADE=1; shift ;;
    --no-service) NO_SERVICE=1; shift ;;
    --no-ui) NO_UI=1; shift ;;
    --force) FORCE=1; shift ;;
    --json) JSON=1; shift ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --tag) TAG="$2"; shift 2 ;;
    --role) ROLE="$2"; shift 2 ;;
    --components) COMPONENTS="$2"; shift 2 ;;
    -h|--help) sed -n '2,64p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 64 ;;
  esac
done

say()  { [ "$JSON" = "1" ] || printf '%s\n' "$*"; }
die()  { printf '❌ %s\n' "$1" >&2; exit "${2:-1}"; }
os="$(uname -s)"; arch="$(uname -m)"
# 平台：默认按 uname；ZERG_UPGRADE_PLAT 是**测试接缝**（白名单两个字面量，防误用改真机构建目标）
PLAT=""
if [ -n "${ZERG_UPGRADE_PLAT:-}" ]; then
  case "$ZERG_UPGRADE_PLAT" in
    darwin-arm64|linux-amd64) PLAT="$ZERG_UPGRADE_PLAT" ;;
    *) die "ZERG_UPGRADE_PLAT 只接受 darwin-arm64 / linux-amd64（得 ${ZERG_UPGRADE_PLAT}）" 64 ;;
  esac
else
  case "${os}-${arch}" in
    Darwin-arm64) PLAT="darwin-arm64" ;;
    Linux-x86_64) PLAT="linux-amd64" ;;
    *) die "本版仅支持 darwin-arm64 / linux-amd64（当前 ${os}-${arch}）" ;;
  esac
fi
# 目标平台的操作系统名（决定代码签名 / 服务管理走哪条路；通常 == 本机 os）
TARGET_OS="${PLAT%%-*}"

# ── 角色 → 组件集（B5）；每件都要在清单里找得到，找不到就**不换装** ──────────────
ALLOWED_COMPONENTS="core agent agentd ui"
resolve_components() {
  local raw="$COMPONENTS" list=""
  if [ -n "$raw" ]; then
    list="$(printf '%s' "$raw" | tr 'A-Z' 'a-z')"
  elif [ "$ROLE" = "node" ]; then
    list="core,agentd"
  else
    list="core,agent"
    [ "$PLAT" = "darwin-arm64" ] && [ "$NO_UI" != "1" ] && list="$list,ui"
  fi
  local out="" c seen=" "
  IFS=',' read -r -a _parts <<< "$list"
  for c in "${_parts[@]}"; do
    c="$(printf '%s' "$c" | tr -d '[:space:]')"
    [ -n "$c" ] || continue
    case " $ALLOWED_COMPONENTS " in
      *" $c "*) ;;
      *) die "未知组件 '$c'（可选：core/agent/agentd/ui）" 4 ;;
    esac
    if [ "$c" = "ui" ]; then
      [ "$PLAT" = "darwin-arm64" ] || die "ui 组件只在 Mac（darwin-arm64）编译/换装——当前平台 ${PLAT}（UI 仅 Mac）" 4
      [ "$ROLE" = "node" ] && die "节点角色（--role node）不含 UI——UI 仅在主控机（Mac）" 4
      [ "$NO_UI" = "1" ] && continue
    fi
    case "$seen" in *" $c "*) continue ;; esac
    seen="$seen$c "
    out="${out:+$out,}$c"
  done
  [ -n "$out" ] || die "组件集为空（--components '$raw'）" 4
  printf '%s' "$out"
}
COMP_LIST="$(resolve_components)"
has_component() { case ",$COMP_LIST," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

# ── 身份读取（每件都必须能自报；读不到就记 unknown，不假装）──────────────────
ver_of() { # $1=二进制路径
  [ -x "$1" ] || { echo "未安装"; return; }
  "$1" --version 2>/dev/null | head -1 || echo "无法自报"
}
sha_of() { [ -f "$1" ] && shasum -a 256 "$1" | awk '{print $1}' || echo "-"; }

# ── 在途任务（drain 判据）──────────────────────────────────────────────────
inflight() {
  [ "$NO_SERVICE" = "1" ] && { echo 0; return; }
  [ -f "$TOKEN_FILE" ] || { echo 0; return; }
  local out
  out="$(curl -s -m 5 -H "X-Auth-Token: $(cat "$TOKEN_FILE")" "$API/api/tasks" 2>/dev/null || echo '')"
  [ -n "$out" ] || { echo 0; return; }
  printf '%s' "$out" | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: print(0); raise SystemExit
ts = d.get("tasks", d if isinstance(d,list) else [])
live = {"running","queued","pending","in_progress"}
print(sum(1 for t in ts if str(t.get("status","")).lower() in live))
' 2>/dev/null || echo 0
}

# ── 取源（默认：本地 git 树构建产物 ｜ --from-assets：GitHub Release 应急通道）──
# 默认取件源 = "git 树构建产物"：由 `zerg update` fetch→本机构建写到临时区后交本内核
# （ZERG_UPGRADE_SOURCE=file://<staging>）。**内核不亲自动手构建**——构建是 update 的活，
# 内核只守六阶段（drain→swap→restart→verify→report）；资产通道（--from-assets）是应急兜底。
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
fetch_manifest() {
  if [ "$FROM_ASSETS" = "1" ]; then
    command -v gh >/dev/null || die "读 GitHub Release 需要 gh；或改用默认的 git 树构建产物通道（ZERG_UPGRADE_SOURCE=file://…）"
    gh release download ${TAG:+"$TAG"} --repo "${ZERG_ASSETS_REPO:-Mr2109/zerg-swarm}" --pattern manifest.json --dir "$work" --clobber 2>/dev/null \
      || die "取不到 Release manifest（仓库可能还没有 Release）"
    SRCDIR=""
  else
    local d="${SOURCE#file://}"
    [ -n "$d" ] || die "未提供 git 树构建产物：请先跑 \`zerg update\`（它 fetch→构建→交本内核），或加 --from-assets 走应急资产通道"
    [ -f "$d/manifest.json" ] || die "构建产物目录缺 manifest.json：${d}（半成品一律不换装）"
    cp -p "$d/manifest.json" "$work/manifest.json"; SRCDIR="$d"
  fi
  mj() { python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(eval(sys.argv[2],{"d":d}))' "$work/manifest.json" "$1"; }
  SRC_VER="$(mj 'd["version"]')"; SRC_SHA="$(mj 'd["commit"]')"; SRC_TAG="$(mj 'd["tag"]')"
}
pull_artifact() { # $1=制品名 → $work/dl/<name>
  mkdir -p "$work/dl"
  if [ "$FROM_ASSETS" = "1" ]; then
    gh release download ${TAG:+"$TAG"} --repo "${ZERG_ASSETS_REPO:-Mr2109/zerg-swarm}" --pattern "$1" --dir "$work/dl" --clobber
  else
    cp -p "$SRCDIR/$1" "$work/dl/$1"
  fi
}
want_sha() { python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(next(x["sha256"] for x in d["artifacts"] if x["name"]==sys.argv[2]))' "$work/manifest.json" "$1"; }

# ────────────────────────────────────────────────────────────────────────────
case "$MODE" in
  fleet)
    # ══ 机群（B5，2026-09-13）：**每台各自跑源码式更新** ═══════════════════════
    #
    # 与改造前的根本差别：主控**不再**交叉编译、**不再** scp 推预编译制品。
    #   本机   ：走本机的 `zerg update`（它自己 fetch→本机构建→交本内核）
    #   远程机 ：ssh 上去让它**自己** fetch→本机构建→交**它本地的**六阶段内核换装
    # 主控只做三件事：① 编排（顺序 + 三台共享的构建时间戳）② 回收各机回执
    # ③ 核对 /api/fleet/status 的 code_sha 是否**三台一致且 = 目标**（不拿"命令没报错"当证据）。
    #
    # 为什么这样更对：预编译制品是"主控编的件装到异构机上"——遇到 libc/工具链/native 依赖差异
    # 会静默装上不合适的件，且无法验证"该机的构建环境是否真能产出这份代码"。改成各自现编后，
    # 每台机器的**构建事实**（工具链 pin/actual）随回执回传，才谈得上可复现与可核对（G6）。
    FLEET_WAIT_S="${ZERG_FLEET_WAIT_S:-90}"
    FLEET_SSH="${ZERG_FLEET_SSH:-ssh}"
    FLEET_REF="${ZERG_UPDATE_REF:-main}"
    FLEET_REMOTE="${ZERG_UPDATE_REMOTE:-}"
    FLEET_SKIP_MATRIX="${ZERG_FLEET_SKIP_MATRIX:-0}"
    FLEET_TIMEOUT=8

    # fmt：互为前缀即算同一提交（短/长 sha 混用是常态）
    fmatch() { [ -n "$1" ] || return 1; case "$1" in "$2"*) return 0 ;; esac; case "$2" in "$1"*) return 0 ;; esac; return 1; }

    # ── 名册：ZERG_FLEET_NODES（env，优先）或 gateway/fleet.yaml 的 update.nodes ──
    # 每行（TSV）：name · ssh · root · prefix · api · components · role · receipts
    fleet_roster() { python3 - "$REPO_ROOT" "$PREFIX" "$API" "$COMP_LIST" <<'PY'
import os, re, sys
repo, prefix, api, comp = sys.argv[1:5]
print("\t".join([os.environ.get("ZERG_FLEET_LOCAL_NAME", "local"), "-", repo, prefix, api, comp,
                 "controller", "$HOME/.zerg/update_receipts"]))
spec = os.environ.get("ZERG_FLEET_NODES", "").strip()
nodes = []
if spec:
    for tok in spec.split():
        if "=" not in tok:
            continue
        name, _, rest = tok.partition("=")
        f = {"name": name.strip(), "ssh": rest.split(",")[0].strip()}
        for extra in rest.split(",")[1:]:
            k, _, v = extra.partition("=")
            if k.strip():
                f[k.strip()] = v.strip()
        nodes.append(f)
else:
    yml = os.environ.get("ZERG_FLEET_YAML") or os.path.join(repo, "gateway", "fleet.yaml")
    try:
        txt = open(yml, encoding="utf-8").read()
    except Exception:
        txt = ""
    in_nodes = False
    for line in txt.splitlines():
        if re.match(r"^\s*nodes\s*:", line):
            in_nodes = True
            continue
        if not in_nodes:
            continue
        if re.match(r"^\S", line):        # 出了 update 段
            in_nodes = False
            continue
        m = re.match(r"^\s*-\s*\{?(.*?)\}?\s*$", line)
        if not m:
            continue
        f = {}
        for kv in re.finditer(r'([A-Za-z_]+)\s*:\s*(?:"([^"]*)"|\'([^\']*)\'|([^,}]+))', m.group(1)):
            val = kv.group(2) or kv.group(3) or kv.group(4) or ""
            f[kv.group(1).strip()] = val.strip().strip('"\'')
        if f.get("name"):
            nodes.append(f)
home = os.path.expanduser("~")
seen = set()
for n in nodes:
    name = (n.get("name") or "").strip()
    if not name or name in seen:
        continue
    seen.add(name)
    root = n.get("root") or os.path.join(home, "zerg")
    target = n.get("ssh") or n.get("host") or ""
    print("\t".join([name, target, root, n.get("prefix") or os.path.join(root, "bin"),
                     n.get("api") or "http://127.0.0.1:8580", n.get("components") or "core,agentd",
                     n.get("role") or "node", n.get("receipts") or "$HOME/.zerg/update_receipts"]))
PY
    }

    # ── 矩阵读取（只读；无令牌/无响应 ⇒ 空）──────────────────────────────────
    fleet_matrix() {
      [ -f "$TOKEN_FILE" ] || return 0
      curl -s -m 5 -H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null)" "$API/api/fleet/status" 2>/dev/null || true
    }
    matrix_sha() { # $1=机器名；stdin=矩阵 JSON → code_sha（未上报 ⇒ 空）
      python3 -c '
import json, sys
name = sys.argv[1]
raw = sys.stdin.read().strip()
try:
    d = json.loads(raw) if raw else {}
except Exception:
    print(""); raise SystemExit
ms = d.get("machines") or d.get("snapshots") or {}
m = ms.get(name) if isinstance(ms, dict) else None
print((m or {}).get("code_sha", "") if isinstance(m, dict) else "")
' "$1"
    }
    matrix_table() { # stdin=矩阵 JSON → 每台一行（人读）
      python3 -c '
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    print("  （读不到矩阵：主控无响应 / 无令牌 / 未部署 code_sha 字段）"); raise SystemExit
try:
    d = json.loads(raw)
except Exception:
    print("  （矩阵解析失败）"); raise SystemExit
ms = d.get("machines") or d.get("snapshots") or {}
if not isinstance(ms, dict) or not ms:
    print("  （矩阵为空）"); raise SystemExit
for name, m in ms.items():
    m = m if isinstance(m, dict) else {}
    print("  %-8s code_sha=%-42s code_version=%-10s healthy=%s" % (
        name, m.get("code_sha") or "<空>", m.get("code_version") or "<空>", m.get("healthy")))
' 2>/dev/null || say "  （矩阵读取失败）"
    }

    ROSTER="$(fleet_roster)"
    [ -n "$ROSTER" ] || die "机群名册为空：设 ZERG_FLEET_NODES，或在 gateway/fleet.yaml 写 update.nodes" 1
    NODE_COUNT="$(printf '%s\n' "$ROSTER" | grep -c . || true)"
    REMOTE_COUNT=$((NODE_COUNT - 1))

    # ── 目标提交（--to 优先；否则 ls-remote 远端 ref——**只读**，绝不碰工作树 G4）──
    TARGET_SHA=""
    if [ -n "$TAG" ]; then
      TARGET_SHA="$TAG"
    else
      TARGET_SHA="$(git -C "$GITREPO" ls-remote "${FLEET_REMOTE:-origin}" "$FLEET_REF" 2>/dev/null | awk 'NR==1{print $1}')" || true
    fi
    BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"   # 三台共享（G6：同平台可逐字节比对）

    say "🎛  机群升级（每台各自源码式更新；主控只编排 + 核对矩阵）"
    say "   名册   : ${NODE_COUNT} 台（本机 1 + 远程 ${REMOTE_COUNT}）"
    say "   目标   : ${FLEET_REF}@$(printf '%s' "$TARGET_SHA" | cut -c1-12)"
    say "   构建时间戳: ${BUILD_TIME}（三台共享，便于 G6 逐字节比对）"
    say ""
    say "Update plan（机群 · 顺序：远程 → 本机——动自己那步永远最后）:"
    idx=0
    while IFS=$'\t' read -r n_name n_ssh n_root n_prefix n_api n_comp n_role n_rc; do
      [ -n "$n_name" ] || continue
      idx=$((idx + 1))
      if [ "$n_role" = "controller" ]; then
        say "  ${idx}) ${n_name}（本机 · controller）: 本机 \`zerg update\`（组件 ${n_comp}）"
      else
        say "  ${idx}) ${n_name}（远程 · node ${n_ssh}）: ssh 该机 \`zerg update --role node\`（组件 ${n_comp}，root=${n_root}）"
      fi
    done < <(printf '%s\n' "$ROSTER")
    say ""
    say "机群版本矩阵（升级前）:"
    MATRIX_BEFORE="$(fleet_matrix)"
    printf '%s' "$MATRIX_BEFORE" | matrix_table

    if [ "$FLEET_PLAN_ONLY" = "1" ]; then
      say ""
      say "（--plan：只盘点，未执行任何更新、未碰任何文件）"
      exit 0
    fi
    [ -n "$TARGET_SHA" ] || die "读不到目标提交（离线 / 无远端 ${FLEET_REMOTE:-origin} / 非法 ref ${FLEET_REF}）——机群升级必须知道目标；--tag <sha> 可显式指定" 1

    # ── 执行：逐台（远程先、本机最后）────────────────────────────────────────
    NODES_TSV="$work/fleet-nodes.tsv"
    : > "$NODES_TSV"
    remote_rows="$(printf '%s\n' "$ROSTER" | awk -F'\t' 'NR>1')"
    local_rows="$(printf '%s\n' "$ROSTER" | awk -F'\t' 'NR==1')"

    run_remote_node() {
      local nm="$1" tgt="$2" root="$3" px="$4" api="$5" comp="$6" rc_dir="$7"
      local pre missing="" out rrc st detail lr kr
      [ -n "$tgt" ] && [ "$tgt" != "-" ] || { printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$nm" "-" "pending-manual" "名册缺 ssh 目标" "-" "-" "-" "-" "-" >> "$NODES_TSV"; return 0; }
      say "  ▶ 远程 ${nm}（${tgt}）：让该机自己 fetch→构建→本地六阶段换装"
      # 预检：该机上必须真的有「git 检出 + 内核脚本 + 能自更新的 CLI + Go 工具链」——缺一不可。
      # 缺什么就如实说 pending-manual 并打印 bootstrap 待执行命令（**绝不假装完成**）。
      pre="$("$FLEET_SSH" -o BatchMode=yes -o ConnectTimeout="$FLEET_TIMEOUT" "$tgt" \
        "test -d '$root/.git' && echo git-ok; test -f '$root/scripts/zerg-upgrade.sh' && echo kernel-ok; test -x '$px/zerg-core' && echo cli-ok; (command -v go >/dev/null && go version) 2>/dev/null" 2>&1 || true)"
      for need in git-ok kernel-ok cli-ok; do
        case "$pre" in *"$need"*) ;; *) missing="$missing ${need%-ok}" ;; esac
      done
      if [ -n "$missing" ]; then
        say "    ⏸ 该机不可自更新（缺：${missing# }）→ pending-manual"
        say "       bootstrap 待执行（在 ${nm} 上，root=${root}）："
        say "         git clone --depth 1 ${FLEET_REMOTE:-https://github.com/Mr2109/zerg-swarm.git} $root && cd $root && bash scripts/zerg-upgrade.sh --plan"
        printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$nm" "$tgt" "pending-manual" "缺：${missing# }" "-" "-" "-" "-" "-" >> "$NODES_TSV"
        return 0
      fi
      local cmd
      cmd="cd '$root' && ZERG_UPDATE_ROLE=node ZERG_UPDATE_REPO='$root' ZERG_PREFIX='$px' ZERG_API_BASE='$api' ZERG_UPGRADE_SCRIPT='$root/scripts/zerg-upgrade.sh' ZERG_BUILD_TIME='$BUILD_TIME' ZERG_UPDATE_REF='$FLEET_REF'"
      [ -n "$FLEET_REMOTE" ] && cmd="$cmd ZERG_UPDATE_REMOTE='$FLEET_REMOTE'"
      cmd="$cmd '$px/zerg-core' update --role node --components '$comp'"
      [ "$FORCE" = "1" ] && cmd="$cmd --force"
      [ "$NO_UI" = "1" ] && cmd="$cmd --no-ui"
      rrc=0
      out="$("$FLEET_SSH" -o BatchMode=yes -o ConnectTimeout="$FLEET_TIMEOUT" "$tgt" "$cmd" 2>&1)" || rrc=$?
      printf '%s\n' "$out" | sed 's/^/     /'
      case "$rrc" in
        0) st="ok"; detail="已在该机完成源码式更新（内核已接手换装）" ;;
        2) st="up-to-date"; detail="该机已是最新（未换装）" ;;
        3) st="refused"; detail="该机拒绝（安装方式/在途任务）" ;;
        *) st="failed"; detail="退出码 $rrc" ;;
      esac
      # 回执回读（不靠"命令没报错"当证据）：该机自己的内核回执 + 交接回执
      lr="$("$FLEET_SSH" -o BatchMode=yes -o ConnectTimeout="$FLEET_TIMEOUT" "$tgt" "cat $rc_dir/latest-update-launch.json 2>/dev/null" 2>/dev/null || true)"
      kr="$("$FLEET_SSH" -o BatchMode=yes -o ConnectTimeout="$FLEET_TIMEOUT" "$tgt" "cat $rc_dir/latest.json 2>/dev/null" 2>/dev/null || true)"
      local lr_sha="" kr_res="" kr_at="" kr_live="" tc=""
      lr_sha="$(printf '%s' "$lr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print((d.get("target") or {}).get("commit",""))' 2>/dev/null || true)"
      tc="$(printf '%s' "$lr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
t=d.get("toolchain") or {}
print("%s→%s" % (t.get("pin","?"), t.get("actual","?")))' 2>/dev/null || true)"
      kr_res="$(printf '%s' "$kr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("result",""))' 2>/dev/null || true)"
      kr_at="$(printf '%s' "$kr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("failed_at") or "")' 2>/dev/null || true)"
      kr_live="$(printf '%s' "$kr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print((d.get("verify") or {}).get("live_code_sha") or "")' 2>/dev/null || true)"
      lr_tc="$(printf '%s' "$lr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
t=d.get("toolchain") or {}
print("%s|%s|%s" % (t.get("pin",""), t.get("actual",""), t.get("policy","")))' 2>/dev/null || true)"
      lr_bt="$(printf '%s' "$lr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("build_time") or "")' 2>/dev/null || true)"
      if [ -n "$lr_sha" ]; then
        if fmatch "$lr_sha" "$TARGET_SHA"; then
          say "    📝 该机回执：target=$(printf '%s' "$lr_sha" | cut -c1-12) 工具链=${tc:-未知} 内核=${kr_res:-进行中}"
        else
          say "    ⚠️ 该机回执 target=$(printf '%s' "$lr_sha" | cut -c1-12) ≠ 目标（可能是上一轮回执）"
          lr_sha=""
        fi
      fi
      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$nm" "$tgt" "$st" "$detail" "${lr_sha:--}" "${kr_res:--}" "${kr_live:--}" "${lr_tc:--}" "${lr_bt:--}" >> "$NODES_TSV"
    }

    while IFS=$'\t' read -r n_name n_ssh n_root n_prefix n_api n_comp n_role n_rc; do
      [ -n "$n_name" ] || continue
      [ "$n_role" = "controller" ] && continue
      run_remote_node "$n_name" "$n_ssh" "$n_root" "$n_prefix" "$n_api" "$n_comp" "$n_rc"
    done < <(printf '%s\n' "$remote_rows")

    # 本机（最后动——升级器不先升自己所在的那台）
    IFS=$'\t' read -r l_name l_ssh l_root l_prefix l_api l_comp l_role l_rc <<< "$local_rows"
    LOCAL_CMD="${ZERG_FLEET_LOCAL_CMD:-$l_prefix/zerg-core}"
    local_st="failed"; local_detail=""
    say "  ▶ 本机 ${l_name}（${LOCAL_CMD} update）：源码式自更新"
    if [ ! -x "$LOCAL_CMD" ]; then
      local_st="pending-manual"; local_detail="本机缺 ${LOCAL_CMD}（设 ZERG_FLEET_LOCAL_CMD 或先装主控）"
      say "    ⏸ $local_detail"
    else
      lcmd="$LOCAL_CMD update"
      [ "$FORCE" = "1" ] && lcmd="$lcmd --force"
      [ "$NO_UI" = "1" ] && lcmd="$lcmd --no-ui"
      lrc=0
      lout="$(ZERG_BUILD_TIME="$BUILD_TIME" ZERG_UPDATE_ROLE=controller sh -c "$lcmd" 2>&1)" || lrc=$?
      printf '%s\n' "$lout" | sed 's/^/     /'
      case "$lrc" in
        0) local_st="ok"; local_detail="已在**新进程**里完成换装（G2）" ;;
        2) local_st="up-to-date"; local_detail="本机已是最新（未换装）" ;;
        3) local_st="refused"; local_detail="本机拒绝（安装方式/开发态）" ;;
        *) local_st="failed"; local_detail="退出码 $lrc" ;;
      esac
    fi
    # 本机回执回读（$RECEIPTS 就是本机的回执目录）：目标 / 工具链 / 构建时间戳
    llr=""
    [ -f "$RECEIPTS/latest-update-launch.json" ] && llr="$(cat "$RECEIPTS/latest-update-launch.json" 2>/dev/null || true)"
    l_sha="$(printf '%s' "$llr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print((d.get("target") or {}).get("commit",""))' 2>/dev/null || true)"
    l_tc="$(printf '%s' "$llr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
t=d.get("toolchain") or {}
print("%s|%s|%s" % (t.get("pin",""), t.get("actual",""), t.get("policy","")))' 2>/dev/null || true)"
    l_bt="$(printf '%s' "$llr" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("build_time") or "")' 2>/dev/null || true)"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$l_name" "-" "$local_st" "$local_detail" "${l_sha:--}" "-" "-" "${l_tc:--}" "${l_bt:--}" >> "$NODES_TSV"

    # ── 核对：矩阵收敛（三台 code_sha 一致且 = 目标）──────────────────────────
    MATRIX_FINAL=""; CONVERGED=0
    if [ "$FLEET_SKIP_MATRIX" = "1" ]; then
      say ""
      say "⚠️  ZERG_FLEET_SKIP_MATRIX=1：跳过矩阵核对（口径缺失，回执会如实标注）"
      CONVERGED=2
    else
      say ""
      say "⏳ 等矩阵收敛（最多 ${FLEET_WAIT_S}s；口径：/api/fleet/status 的 code_sha == 目标）"
      deadline=$(( $(date +%s) + FLEET_WAIT_S ))
      while : ; do
        MATRIX_FINAL="$(fleet_matrix)"
        CONVERGED=0
        while IFS=$'\t' read -r n_name _ _ _ _ _ _ _; do
          [ -n "$n_name" ] || continue
          sha="$(printf '%s' "$MATRIX_FINAL" | matrix_sha "$n_name")"
          fmatch "$sha" "$TARGET_SHA" || CONVERGED=1
        done < <(printf '%s\n' "$ROSTER")
        [ "$CONVERGED" = "0" ] && break
        [ "$(date +%s)" -ge "$deadline" ] && break
        sleep 5
      done
    fi

    say ""
    say "机群版本矩阵核对（口径：/api/fleet/status 的 code_sha == 目标 $(printf '%s' "$TARGET_SHA" | cut -c1-12)）:"
    MATRIX_OK=1
    while IFS=$'\t' read -r n_name _ _ _ _ _ _ _; do
      [ -n "$n_name" ] || continue
      sha="$(printf '%s' "$MATRIX_FINAL" | matrix_sha "$n_name")"
      if fmatch "$sha" "$TARGET_SHA"; then
        say "  ✅ ${n_name}  code_sha=$(printf '%s' "$sha" | cut -c1-12)"
      else
        MATRIX_OK=0
        say "  ❌ ${n_name}  code_sha=$( [ -n "$sha" ] && printf '%s' "$sha" | cut -c1-12 || echo '<未上报>' )（≠ 目标）"
      fi
    done < <(printf '%s\n' "$ROSTER")
    # 名册之外、但矩阵里出现且不同版的机器：如实点名（不在名册内 → 只告警，不判失败）
    printf '%s' "$MATRIX_FINAL" | python3 - "$TARGET_SHA" "$(printf '%s\n' "$ROSTER" | cut -f1 | tr '\n' ' ')" <<'PY' 2>/dev/null || true
import json, sys
target, names = sys.argv[1], set(sys.argv[2].split())
raw = sys.stdin.read().strip()
try:
    ms = (json.loads(raw) or {}).get("machines", {})
except Exception:
    ms = {}
for name, m in (ms or {}).items():
    if name in names or not isinstance(m, dict):
        continue
    sha = m.get("code_sha") or ""
    if not (sha and (sha.startswith(target) or target.startswith(sha))):
        print("  ⚠️ 名册外机器 %s code_sha=%s（≠ 目标）——未编排，仅报告" % (name, (sha or "<空>")[:12]))
PY

    # ── 回执：逐台 + 汇总（pending / 未收敛也如实记录）────────────────────────
    mkdir -p "$RECEIPTS"
    FTS="$(date -u +%Y%m%dT%H%M%SZ)"
    FRC="$RECEIPTS/fleet-$FTS.json"
    printf '%s' "$MATRIX_BEFORE" > "$work/fleet-matrix-before.json"
    printf '%s' "$MATRIX_FINAL" > "$work/fleet-matrix-after.json"
    python3 - "$FRC" "$FTS" "$FLEET_REF" "$TARGET_SHA" "$BUILD_TIME" "$COMP_LIST" \
             "$NODES_TSV" "$work/fleet-matrix-before.json" "$work/fleet-matrix-after.json" "$CONVERGED" <<'PY'
import json, sys
rc, ts, ref, target, bt, comp, nodes_tsv, before_p, after_p, converged = sys.argv[1:11]

def load(p):
    try:
        d = json.load(open(p))
    except Exception:
        return {}
    ms = d.get("machines") or d.get("snapshots") or {}
    return {k: {"code_sha": (v or {}).get("code_sha", ""), "code_version": (v or {}).get("code_version", "")}
            for k, v in (ms.items() if isinstance(ms, dict) else [])}

nodes = []
for line in open(nodes_tsv, encoding="utf-8"):
    f = line.rstrip("\n").split("\t")
    if not f or not f[0]:
        continue
    f += [""] * (9 - len(f))
    nodes.append({"name": f[0], "ssh": f[1], "status": f[2], "detail": f[3],
                  "receipt_target": f[4], "kernel_result": f[5], "live_code_sha": f[6],
                  "toolchain": f[7], "build_time": f[8]})
after = load(after_p)
for n in nodes:
    n["code_sha_after"] = after.get(n["name"], {}).get("code_sha", "")
    n["converged"] = bool(n["code_sha_after"]) and (
        n["code_sha_after"].startswith(target) or target.startswith(n["code_sha_after"]))
    if not n["converged"]:
        n["status"] = "not-converged" if n["status"] in ("ok", "up-to-date") else n["status"]
doc = {
    "schema": 2, "kind": "fleet", "at": ts,
    "mode": "per-machine-source-update",   # B5：每台各自源码式更新（不再推预编译制品）
    "target": {"ref": ref, "commit": target}, "build_time_shared": bt,
    "controller_components": comp.split(","),
    "order": ["remote(node)", "local(controller)"],
    "nodes": nodes,
    "matrix_before": load(before_p), "matrix_after": after,
    "matrix_check": {"converged": converged == "0", "skipped": converged == "2", "target": target},
}
json.dump(doc, open(rc, "w"), ensure_ascii=False, indent=2)
print("  📝 机群回执:", rc)
PY
    cp -p "$FRC" "$RECEIPTS/latest-fleet.json" 2>/dev/null || true
    cp -p "$FRC" "$RECEIPTS/fleet-receipt-$FTS.json" 2>/dev/null || true

    # ── 结论 ────────────────────────────────────────────────────────────────
    NODE_FAIL=0
    while IFS=$'\t' read -r n_name _ n_st _ _ _ _; do
      [ -n "$n_name" ] || continue
      case "$n_st" in ok|up-to-date|not-converged) ;; *) NODE_FAIL=1 ;; esac
    done < "$NODES_TSV"
    say ""
    if [ "$CONVERGED" = "0" ] && [ "$MATRIX_OK" = "1" ] && [ "$NODE_FAIL" = "0" ]; then
      say "✅ 机群升级完成：三台 code_sha 一致且 = 目标 $(printf '%s' "$TARGET_SHA" | cut -c1-12)"
      exit 0
    fi
    if [ "$CONVERGED" = "2" ]; then
      say "⚠️  机群执行完毕，但**矩阵核对被跳过**（ZERG_FLEET_SKIP_MATRIX=1）——未经核对，不算验收"
      exit 1
    fi
    say "❌ 机群未收敛：矩阵核对未过或某台未完成（详见上面的逐台结果与回执 ${FRC}）"
    exit 1
    ;;
  status)
    [ "$JSON" = "1" ] && { printf '{"core":"%s","agent":"%s","agentd":"%s","ui":"%s"}\n' "$(ver_of "$PREFIX/zerg-core")" "$(ver_of "$PREFIX/zerg-agent")" "$(ver_of "$PREFIX/zerg-agentd")" "$(sha_of "$PREFIX/zerg-ui")"; exit 0; }
    say "📦 前缀：$PREFIX"
    say "   角色 : ${ROLE}（组件 ${COMP_LIST}）"
    say "   主控 : $(ver_of "$PREFIX/zerg-core")"
    say "   子端 : $(ver_of "$PREFIX/zerg-agent")"
    say "   守护 : $(ver_of "$PREFIX/zerg-agentd")"
    say "   UI   : sha256 $(sha_of "$PREFIX/zerg-ui" | cut -c1-16)…（UI 自报见启动日志 [zerg-ui] 行）"
    exit 0
    ;;

  receipts)
    [ -d "$RECEIPTS" ] || { say "（暂无回执）"; exit 0; }
    ls -1t "$RECEIPTS"/*.json 2>/dev/null | head -5 | while read -r f; do say "  $(basename "$f")"; done
    exit 0
    ;;

  rollback)
    n=0
    for name in zerg-core zerg-agent zerg-agentd zerg-ui; do
      if [ -f "$PREFIX/$name.prev" ]; then
        cp -p "$PREFIX/$name" "$PREFIX/$name.failed" 2>/dev/null || true
        mv -f "$PREFIX/$name.prev" "$PREFIX/$name"
        [ "$TARGET_OS" = "darwin" ] && codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || true
        say "↩️  回滚 $name"
        n=$((n+1))
      fi
    done
    [ "$n" = "0" ] && die "没有 .prev 可回滚" 2
    say "✅ 已回滚 $n 件（失败件留 .failed 供取证）"
    exit 0
    ;;
esac

# 只有 apply（真换装）强制「git 树构建产物」；plan/check 无源时退回应急资产通道，
# 保持"随手看一眼版本"可用（与改造前一致）。
if [ "$FROM_ASSETS" != "1" ] && [ -z "$SOURCE" ] && [ "$MODE" != "apply" ]; then
  FROM_ASSETS=1; say "ℹ️  ${MODE} 模式无取件源 ⇒ 退回应急资产通道（--from-assets）"
fi

fetch_manifest
SRC_KIND="git-tree"
if [ "$FROM_ASSETS" = "1" ]; then SRC_KIND="assets(应急)"; fi
say "🏷  源：${SRC_TAG}（代码 ${SRC_SHA}）· 取件源=${SRC_KIND}"

# ── 组件 → 要换装的制品清单（B5）─────────────────────────────────────────────
# 组件集来自 --role/--components；**每一件都必须在该清单里**——缺件即拒（半成品/错件不换装）。
# 这与"谁是构建者"无关：主控机与节点各自构建后交同一个内核，内核只认清单。
WANT=""; MISSING=""
for _c in $(printf '%s' "$COMP_LIST" | tr ',' ' '); do
  _a="zerg-${_c}-${PLAT}"
  if [ "$(want_sha "$_a" 2>/dev/null || echo '-')" = "-" ]; then MISSING="$MISSING $_a"; else WANT="$WANT $_a"; fi
done
say "🧩 角色=${ROLE} · 组件=${COMP_LIST} · 平台=${PLAT}"
TOOLCHAIN_LINE="$(python3 - "$work/manifest.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print("（清单读取失败）"); raise SystemExit
t = d.get("toolchain") or {}
if not t:
    print("（清单无 toolchain 字段——G6 留痕缺失）"); raise SystemExit
print("pin=%s actual=%s policy=%s" % (t.get("pin", "?"), t.get("actual", "?"), t.get("policy", "?")))
PY
)"
say "🧰 构建工具链（G6 留痕）：${TOOLCHAIN_LINE}"
if [ -n "$MISSING" ]; then
  die "清单里缺制品：${MISSING# }（组件 ${COMP_LIST}——半成品/错件一律不换装，未动任何文件）" 4
fi

# 服务管辖（按角色）：主控机管主控(+UI)；节点管 agentd。管不到的组件**只换件不碰服务**并明确告警。
manages_core()    { [ "$ROLE" = "controller" ] && has_component core; }
manages_agentd()  { [ "$ROLE" = "node" ] && has_component agentd; }
stop_agentd() {
  if [ -n "$START_AGENTD" ]; then bash "$START_AGENTD" stop; return $?; fi
  systemctl stop "$AGENTD_UNIT" >/dev/null 2>&1
}
restart_agentd() {
  if [ -n "$START_AGENTD" ]; then bash "$START_AGENTD" restart || return 1; else systemctl restart "$AGENTD_UNIT" >/dev/null 2>&1 || return 1; fi
  sleep 2
  if [ -n "$START_AGENTD" ]; then bash "$START_AGENTD" is-active >/dev/null 2>&1; return $?; fi
  systemctl is-active "$AGENTD_UNIT" >/dev/null 2>&1
}

# 盘点（plan / check / apply 共用）
CUR_CORE="$(ver_of "$PREFIX/zerg-core")"
CUR_AGENT="$(ver_of "$PREFIX/zerg-agent")"
CUR_AGENTD="$(ver_of "$PREFIX/zerg-agentd")"
CUR_UI_SHA="$(sha_of "$PREFIX/zerg-ui")"
TARGET_UI_SHA="$(want_sha zerg-ui-$PLAT 2>/dev/null || echo '-')"
NEED=""
[ -n "$CUR_CORE" ] && case "$CUR_CORE" in *"$SRC_SHA"*) ;; *) NEED="yes" ;; esac
[ -z "$CUR_CORE" ] && NEED="yes"
if has_component ui; then [ "$CUR_UI_SHA" = "$TARGET_UI_SHA" ] || NEED="yes"; fi
if manages_agentd; then case "$CUR_AGENTD" in *"$SRC_SHA"*) ;; *) NEED="yes" ;; esac; fi

if [ "$MODE" = "check" ]; then
  [ -n "$NEED" ] && say "→ 有新版：${SRC_TAG}" || say "→ 已是最新：${SRC_TAG}"
  exit 0
fi

INFLIGHT="$(inflight)"

if [ "$MODE" = "plan" ]; then
  say "Update plan:"
  say "  install : ${PREFIX}（平台 ${PLAT}）"
  say "  角色    : ${ROLE} · 组件 ${COMP_LIST}"
  say "  换装件  : ${WANT# }"
  say "  工具链  : ${TOOLCHAIN_LINE}"
  say "  source  : ${SRC_KIND} ${SOURCE}"
  say "  当前版本 : 主控 $( [ -n "$CUR_CORE" ] && echo "$CUR_CORE" || echo '未安装' )"
  say "            子端 $( [ -n "$CUR_AGENT" ] && echo "$CUR_AGENT" || echo '未安装' )"
  say "            守护 $( [ -n "$CUR_AGENTD" ] && echo "$CUR_AGENTD" || echo '未安装' )"
  say "            UI  sha $( echo "$CUR_UI_SHA" | cut -c1-16 )…"
  say "  目标版本 : ${SRC_TAG}（代码 ${SRC_SHA}）"
  say "  在途任务 : ${INFLIGHT} 个$( [ "$INFLIGHT" != "0" ] && echo '（需 --force 或等待）' )"
  say "  服务     : $( [ "$NO_SERVICE" = "1" ] && echo '不触碰（--no-service）' || { _svc=""; manages_core && _svc="$_svc 主控"; has_component ui && _svc="$_svc UI"; manages_agentd && _svc="$_svc agentd"; echo "管${_svc}（停→换→启）"; } )"
  say "  将执行   : 校验 sha256 → 停服务 → 原子换装(留 .prev) → 重签名 → 重启 → verify → 回执"
  [ -n "$NEED" ] || say "  ⚠️ 看起来已是最新（仍可强制换装）"
  exit 0
fi

# ── drain ──────────────────────────────────────────────────────────────────
if [ "$INFLIGHT" != "0" ] && [ "$FORCE" != "1" ]; then
  say "⛔ 有 ${INFLIGHT} 个在途任务——拒绝升级（等它收尾，或 --force）"
  exit 3
fi

# ── 下载 + 校验（不过就拒绝，且不动已装文件）────────────────────────────────
for a in $WANT; do
  pull_artifact "$a"
  w="$(want_sha "$a")"; g="$(shasum -a 256 "$work/dl/$a" | awk '{print $1}')"
  [ "$w" = "$g" ] || die "校验失败：${a}（期望 ${w:0:12}… 实际 ${g:0:12}…）——拒绝升级，未动任何文件" 4
  say "   ✅ $a  sha256 ${g:0:12}…"
done

# ── 降级门（C13）────────────────────────────────────────────────────────────
# 只校 sha256 不够：那样"源比当前旧"时会照单换装 → 静默降级（2026-09-11 实测踩到）。
# 判据用 git 祖先关系（同一仓库里两者都在历史中才判得出）；判不出就不拦，只提示。
CUR_SHA_SHORT="$(printf '%s' "$CUR_CORE" | awk '{print $3}' | sed 's/+.*//')"
# 2026-09-13（真机首升暴露）：比对面原先固定用「已装件的 sha」。当装的是**公开仓制品**时，那个 sha
# 在私有图里根本不存在 ⇒ 恒报「无法判定」⇒ 降级拦截形同没有（公开/私有 sha 混态）。
# 改为优先与**本地代码树的 HEAD** 比 —— 构建来自这棵树，它必然在本地历史里；再退回已装件 sha 兜底。
LOCAL_HEAD="$(git -C "$GITREPO" rev-parse --short HEAD 2>/dev/null || true)"
# 2026-09-13（本机换装暴露）：源 sha 可能带 `+dirty`/`+<buildtime>` 后缀（构建身份约定），
# git 解析不了它 ⇒ 比较恒失败、降级门形同虚设。比较前一律剥后缀。
SRC_SHA_CMP="${SRC_SHA%%+*}"
CUR_SHA_SHORT="${CUR_SHA_SHORT%%+*}"
LOCAL_HEAD="${LOCAL_HEAD%%+*}"
CMP_SHA="$LOCAL_HEAD"
CMP_LABEL="本地树"
if [ -z "$CMP_SHA" ] || ! git -C "$GITREPO" cat-file -e "${SRC_SHA}^{commit}" 2>/dev/null; then
  CMP_SHA="$CUR_SHA_SHORT"; CMP_LABEL="已装件"
fi
if [ -n "$CMP_SHA" ] && [ "$CMP_SHA" != "unknown" ] && [ "$CMP_SHA" != "$SRC_SHA_CMP" ]; then
  if git -C "$GITREPO" merge-base --is-ancestor "$SRC_SHA_CMP" "$CMP_SHA" 2>/dev/null; then
    if [ "$ALLOW_DOWNGRADE" != "1" ]; then
      die "目标是旧提交（源 ${SRC_SHA} 早于${CMP_LABEL} ${CMP_SHA}）——拒绝降级；确实要降级请加 --allow-downgrade" 5
    fi
    say "⚠️  降级：源 ${SRC_SHA} 早于${CMP_LABEL} ${CMP_SHA}（--allow-downgrade 已放行）"
  elif git -C "$GITREPO" merge-base --is-ancestor "$CMP_SHA" "$SRC_SHA_CMP" 2>/dev/null; then
    say "✅ 目标比${CMP_LABEL}新（${CMP_SHA} → ${SRC_SHA}），不是降级"
  else
    say "ℹ️  无法判定新旧（源 ${SRC_SHA} 与${CMP_LABEL} ${CMP_SHA} 无祖先关系）——不做降级拦截"
  fi
fi

# ── 停服务（按角色/组件；KeepAlive → 主控必须 bootout）──────────────────────
UI_WAS_RUNNING=0
if [ "$NO_SERVICE" != "1" ]; then
  if manages_core; then
    say "⏸  停主控（launchctl bootout——KeepAlive=true 时直接 kill 会被旧二进制抢重启）"
    bash "$STARTCORE" --stop >/dev/null 2>&1 || true
  fi
  if has_component ui && pgrep -f "$UI_PATTERN" >/dev/null 2>&1; then
    UI_WAS_RUNNING=1; say "⏸  停 UI"; pkill -f "$UI_PATTERN" || true; sleep 1
  fi
  if manages_agentd; then
    say "⏸  停 agentd（unit ${AGENTD_UNIT}）"
    stop_agentd || say "   ⚠️ 停 agentd 失败（无特权？）——继续换件，重启阶段会再判"
  fi
fi

# ── 原子换装（.new → mv；旧件留 .prev；macOS 重签名）────────────────────────
swap_one() { # $1=组件名  $2=源制品名
  local name="$1" src="$2"
  cp -p "$work/dl/$src" "$PREFIX/$name.new"
  # 2026-09-11 修（真机 --fleet 升级被误判 files 失败并回滚）：sha 必须在**签名前**校验。
  # 实测：macOS ad-hoc 重签**不幂等**——未签 a045aa9d → 签一次 3f5e864c → 再签 74af1a5b。
  # 故"签名后再拿落盘字节比 manifest 的（签名前）sha"永远不符；正确做法是校验暂存件，
  # 签名后的正确性由"签名有效 + 活进程自报身份"(verify_live) 保证。
  if [ "$(sha_of "$PREFIX/$name.new")" != "$(want_sha "$src")" ]; then
    say "   ✗ $name 暂存件 sha 与 manifest 不符——**未换装**（已丢弃暂存件）"
    rm -f "$PREFIX/$name.new"
    SWAP_FAIL=1
    return 1
  fi
  [ -f "$PREFIX/$name" ] && mv -f "$PREFIX/$name" "$PREFIX/$name.prev"
  mv -f "$PREFIX/$name.new" "$PREFIX/$name"
  chmod +x "$PREFIX/$name"
  if [ "$TARGET_OS" = "darwin" ]; then
    codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || say "   ⚠️ $name 重签名失败（未签名二进制会被秒杀）"
  fi
  say "   🔁 $name ← ${src}（旧件已留 .prev）"
}
SWAP_FAIL=0
# 换装件 = 清单里声明且属于本组件集的那几件（安装名 = 制品名去掉 -<平台>）
for a in $WANT; do
  inst="$(printf '%s' "$a" | sed "s/-${PLAT}$//")"
  swap_one "$inst" "$a" || swap_one_fail=1
done
true

# ── 重启 + verify（起不来就回滚）────────────────────────────────────────────
verify_files() { # 2026-09-11 修：不再比落盘 sha（重签名已改字节）——改验"在位 + 可执行 + 签名有效"
  # 依据：sha 的真值校验已在 swap_one 里对**暂存件**做过（签名前）；这里守的是"能不能跑"这一类失败。
  local ok=1 a n
  for a in $WANT; do
    n="$(echo "$a" | sed "s/-${PLAT}$//")"
    if [ ! -f "$PREFIX/$n" ] || [ ! -x "$PREFIX/$n" ]; then ok=0; say "   ✗ $n 缺失或不可执行"; continue; fi
    if [ "$TARGET_OS" = "darwin" ]; then
      codesign -v "$PREFIX/$n" >/dev/null 2>&1 || { ok=0; say "   ✗ $n 签名无效（macOS 会秒杀 EXIT137）"; }
    fi
  done
  return $((1-ok))
}
# 运行进程自报的 code_sha（修 #42：verify 的口径是「**跑着的那份代码**自报的身份」，
# 不是磁盘二进制 --version——旧口径会把"换了盘上文件但活进程还是旧的"判成成功）。
# 纪律：**读不到就返回空**，绝不因「连不上 API」把内核整体打断（`set -o pipefail` 下
# curl 的连接失败码 7 会顺着命令替换把脚本带停——2026-09-13 沙箱实测踩到）。
live_sha() {
  local raw=""
  raw="$(curl -s -m 5 -H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null)" "$API/api/capabilities" 2>/dev/null || true)"
  [ -n "$raw" ] || return 0
  printf '%s' "$raw" | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("code_sha") or "")' 2>/dev/null || true
}
sha_match() { # $1=实际 $2=期望（互为前缀即算匹配——短 sha/长 sha 混用是常态）
  [ -n "$1" ] || return 1
  case "$1" in "$2"*) return 0 ;; esac
  case "$2" in "$1"*) return 0 ;; esac
  return 1
}

verify_live() {
  # 主控机：运行进程自报 sha == 目标（修 #42）
  # 节点：跑着的是 agentd，它的「自报」走心跳 → 由主控矩阵核对（--fleet）；本机只验落盘自报 + 服务活
  local got v ok=1
  if manages_core; then
    got="$(live_sha)"
    if [ -z "$got" ]; then
      say "   ✗ 运行中的主控未自报 code_sha（$API/api/capabilities 读不到）——无法证明活进程已换版"
      ok=0
    elif sha_match "$got" "$SRC_SHA_CMP"; then
      say "   ✓ 主控（运行进程）自报 code_sha=$got"
    else
      say "   ✗ 混版：运行进程 code_sha=$got ≠ 目标 ${SRC_SHA}（换了盘、没换活进程）"; ok=0
    fi
  fi
  for c in agent agentd; do
    has_component "$c" || continue
    v="$(ver_of "$PREFIX/zerg-$c")"
    case "$v" in
      *"$SRC_SHA_CMP"*|*"$SRC_VER"*) say "   ✓ ${c}（落盘）自报：${v}" ;;
      *) say "   ✗ $c 自报异常：$v"; ok=0 ;;
    esac
  done
  [ "$ok" = "1" ]
}

# 组件管辖（修 #42；B5 扩到节点）：只重启**它管辖的**组件；管不到就**明确提示**（绝不静默放过）。
#   主控机：主控由启动脚本/launchd 管，UI（darwin）由 start-zerg-ui.sh 管 ⇒ 本内核管；
#   节点  ：agentd 由 systemd 单元管（ZERG_AGENTD_UNIT）或 ZERG_START_AGENTD 注入的脚本管。
#   若 UI 换装后**没能**被本内核拉起来（如别处托管/启动失败）⇒ 记入 UNMANAGED 并在收尾明确告警。
UNMANAGED=""
restart_managed() {
  if manages_core; then
    say "▶️  启主控"
    bash "$STARTCORE" >/dev/null 2>&1 || true
    ready=0
    for i in $(seq 1 20); do
      if curl -s -m 2 -H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null)" "$API/api/capabilities" >/dev/null 2>&1; then ready=1; break; fi
      sleep 1
    done
    [ "$ready" = "1" ] || { say "   ✗ 主控 20s 内未就绪"; return 1; }
  fi
  if has_component ui; then
    say "▶️  启 UI"
    bash "$STARTUI" >/dev/null 2>&1 || true
    sleep 1
    if pgrep -f "$UI_PATTERN" >/dev/null 2>&1; then
      say "   ✓ UI 已重启（pid $(pgrep -f "$UI_PATTERN" | head -1)）"
    elif [ "${UI_WAS_RUNNING:-0}" = "1" ]; then
      say "   ⚠️ UI 原在运行，重启后**未起来**——请手动重启 UI（${STARTUI}），否则界面仍跑旧件"
      UNMANAGED="ui"
    else
      say "   ℹ️ UI 未在运行：仅换装，未代为启动（需要时执行 ${STARTUI}）"
    fi
  fi
  if manages_agentd; then
    say "▶️  启 agentd（unit ${AGENTD_UNIT}）"
    if restart_agentd; then
      say "   ✓ agentd 已重启并 active"
    else
      say "   ✗ agentd 重启失败或未 active（unit ${AGENTD_UNIT}）"
      return 1
    fi
  fi
  return 0
}

RESULT="ok"; FAILED_AT=""
if [ "${SWAP_FAIL:-0}" = "1" ] || [ "${swap_one_fail:-0}" = "1" ] || ! verify_files; then RESULT="rollback"; FAILED_AT="files"; fi
if [ "$RESULT" = "ok" ] && [ "$NO_SERVICE" != "1" ]; then
  if ! restart_managed; then
    RESULT="rollback"
    if manages_agentd; then FAILED_AT="agentd-start"; else FAILED_AT="core-start"; fi
  fi
fi

if [ "$RESULT" = "ok" ] && [ "$NO_SERVICE" != "1" ]; then
  say "🔎 verify（口径：运行进程自报 sha）"
  if ! verify_live; then RESULT="rollback"; FAILED_AT="identity"; fi
elif [ "$RESULT" = "ok" ]; then
  say "ℹ️  --no-service：跳过「运行进程自报」verify（无服务可查；sha 已在换装前对暂存件校验）"
fi

if [ "$RESULT" = "rollback" ]; then
  say "↩️  验证未过（${FAILED_AT}）——自动回滚"
  for name in zerg-core zerg-agent zerg-agentd zerg-ui; do
    [ -f "$PREFIX/$name.prev" ] || continue
    mv -f "$PREFIX/$name" "$PREFIX/$name.failed" 2>/dev/null || true
    mv -f "$PREFIX/$name.prev" "$PREFIX/$name"
    [ "$TARGET_OS" = "darwin" ] && codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || true
  done
  if [ "$NO_SERVICE" != "1" ]; then
    if manages_core; then bash "$STARTCORE" >/dev/null 2>&1 || true; fi
    if manages_agentd; then restart_agentd >/dev/null 2>&1 || say "   ⚠️ 回滚后 agentd 未能起来——请手动 systemctl restart ${AGENTD_UNIT}"; fi
  fi
fi

# ── 回执（成功与失败都写——失败路径才是回执存在的理由）──────────────────────
mkdir -p "$RECEIPTS"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RC="$RECEIPTS/$TS.json"
LIVE_FOR_RECEIPT=""
if manages_core; then LIVE_FOR_RECEIPT="$(live_sha)"; fi   # 节点上无主控 API——不白连（其"运行身份"由主控矩阵核对）
python3 - "$RC" "$TS" "$RESULT" "$FAILED_AT" "$PREFIX" "$PLAT" "$SOURCE" "$SRC_TAG" "$SRC_SHA_CMP" "$CUR_CORE" "$CUR_AGENT" "$INFLIGHT" "$(ver_of "$PREFIX/zerg-core")" "$(ver_of "$PREFIX/zerg-agent")" "$SRC_KIND" "${UNMANAGED:-}" "$LIVE_FOR_RECEIPT" "$ROLE" "$COMP_LIST" "$work/manifest.json" "$(ver_of "$PREFIX/zerg-agentd")" <<'PY'
import json, sys
(rc, ts, result, failed_at, prefix, plat, source, tag, sha, cur_core, cur_agent,
 inflight, new_core, new_agent, source_kind, unmanaged, live, role, components, manifest,
 new_agentd) = sys.argv[1:22]
try:
    m = json.load(open(manifest))
except Exception:
    m = {}
json.dump({
    "schema": 2, "kind": "component", "at": ts, "result": result, "failed_at": failed_at or None,
    "prefix": prefix, "platform": plat, "source": source,
    "source_kind": source_kind,
    "role": role, "components": [c for c in (components or "").split(",") if c],
    "toolchain": m.get("toolchain") or {},
    "verify": {"mode": "running-process-self-reported" if role == "controller" else "on-disk-self-reported+service",
               "target_commit": sha,
               "live_code_sha": live or None,
               "matched": bool(live) and (live == sha or sha.startswith(live) or live.startswith(sha))},
    "unmanaged_components": [u for u in (unmanaged or "").split() if u],
    "from": {"core": cur_core, "agent": cur_agent},
    "to": {"tag": tag, "commit": sha},
    "now": {"core": new_core, "agent": new_agent, "agentd": new_agentd},
    "inflight_tasks": int(inflight or 0),
}, open(rc, "w"), ensure_ascii=False, indent=2)
print("  📝 回执:", rc)
PY
cp -p "$RC" "$RECEIPTS/latest.json" 2>/dev/null || true
ls -1t "$RECEIPTS"/*.json 2>/dev/null | tail -n +21 | xargs -r rm -f    # 只留最近 20 份

if [ "$RESULT" = "ok" ]; then
  say "✅ 升级完成：${SRC_TAG}（代码 ${SRC_SHA}）· 角色 ${ROLE} · 组件 ${COMP_LIST} → $PREFIX"
  if manages_agentd; then
    say "   注：节点上的 agentd「运行身份」由主控矩阵（心跳 code_sha）核对——单机跑请用 --fleet 收口"
  fi
  if [ -n "${UNMANAGED:-}" ]; then
    say "⚠️  以下组件不在本内核管辖内、**需手动重启**：${UNMANAGED}（否则仍跑旧件）"
  fi
  exit 0
else
  say "❌ 升级失败（${FAILED_AT}）——已回滚，回执见 $RC"
  exit 1
fi

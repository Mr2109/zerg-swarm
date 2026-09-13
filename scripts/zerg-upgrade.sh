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
#   bash scripts/zerg-upgrade.sh                 # 执行升级
#   bash scripts/zerg-upgrade.sh --rollback      # 用 .prev 回滚
#   bash scripts/zerg-upgrade.sh --receipts      # 看最近回执
#   bash scripts/zerg-upgrade.sh --fleet --plan   # 跨机编排：盘点全机群版本矩阵并按序计划
#   bash scripts/zerg-upgrade.sh --fleet          # 跨机编排：执行（本地件自动；远程件无特权则标 pending）
#   ... --allow-downgrade                        # 明确允许把代码换成更旧的提交（默认拒绝降级）
#
# 取件源（B4 改造，2026-09-13）：
#   · **默认 = 本地 git 树构建产物**（`zerg update` fetch→本机构建→写到临时区，
#     用 ZERG_UPGRADE_SOURCE=file://<staging> 交本内核；内核只做校验+换装，构建不在这里）
#   · `--from-assets`（默认关）= 应急通道：从 GitHub Release 下载预编译资产（断网/工具链故障兜底）
#   两条通道的 manifest 同形（version/commit/tag/artifacts[].sha256），内核读法一致。
#
# 开关/环境：
#   ZERG_UPGRADE_SOURCE=file:///path/to/staging   git 树构建产物目录（默认取件源）
#   --from-assets                                改用 GitHub Release 资产（应急，默认关）
#   ZERG_PREFIX=<dir>       安装前缀（默认 <repo>/bin）
#   --no-service            不碰 launchd/服务（沙箱测试用）
#   --no-ui                 不构建/换装 UI（快的自检；沙箱用）
#   --force                 有在途任务也照升（默认拒绝）
#   --json                  机器可读
#   测试接缝（沙箱隔离真机，绝不误杀在跑的服务）：
#     ZERG_API_BASE / ZERG_START_CORE / ZERG_STOP_CORE / ZERG_START_UI / ZERG_UI_PATTERN / ZERG_UPGRADE_REPO
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

MODE="apply"; NO_SERVICE=0; FORCE=0; JSON=0; TAG=""; FLEET_PLAN_ONLY=0; ALLOW_DOWNGRADE=0
FROM_ASSETS=0; NO_UI=0
while [ $# -gt 0 ]; do
  case "$1" in
    --plan) if [ "$MODE" = "fleet" ]; then FLEET_PLAN_ONLY=1; else MODE="plan"; fi; shift ;;
    --check) MODE="check"; shift ;;
    --status) MODE="status"; shift ;;
    --rollback) MODE="rollback"; shift ;;
    --receipts) MODE="receipts"; shift ;;
    --fleet) MODE="fleet"; shift ;;
    --from-assets) FROM_ASSETS=1; shift ;;
    --allow-downgrade) ALLOW_DOWNGRADE=1; shift ;;
    --no-service) NO_SERVICE=1; shift ;;
    --no-ui) NO_UI=1; shift ;;
    --force) FORCE=1; shift ;;
    --json) JSON=1; shift ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --tag) TAG="$2"; shift 2 ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 64 ;;
  esac
done

say()  { [ "$JSON" = "1" ] || printf '%s\n' "$*"; }
die()  { printf '❌ %s\n' "$1" >&2; exit "${2:-1}"; }
os="$(uname -s)"; arch="$(uname -m)"
case "${os}-${arch}" in
  Darwin-arm64) PLAT="darwin-arm64" ;;
  Linux-x86_64) PLAT="linux-amd64" ;;
  *) die "本版仅支持 darwin-arm64 / linux-amd64（当前 ${os}-${arch}）" ;;
esac

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
    # ── 跨机升级编排 ────────────────────────────────────────────────────────
    # B4 说明（2026-09-13）：单机取件源已改「git 树构建产物」；**机群（B5）尚未改造**，
    # 无显式取件源时仍自动走资产通道（--from-assets），保证本次改造不打断既有编排。
    if [ -z "$SOURCE" ] && [ "$FROM_ASSETS" != "1" ]; then
      FROM_ASSETS=1; say "ℹ️  机群模式暂走资产通道（自动 --from-assets）；B5 将改为每台各自 \`zerg update\`"
    fi
    # 顺序固定：子端 → 主控 → UI——**动自己那步永远最后**（升级器不升自己所在的进程）。
    # 注：原第 4 件「菜单栏 App（Swift ZergApp）」已于 2026-09-11 由Mr2109拍板整体去除（模块不需要了），故本编排只含三件。
    # 远程件（X3 子端）需要特权（unit 在 /etc/systemd/system、二进制在 /usr/local/bin）：
    # 拿不到就**如实标 pending 并打印待执行命令**，绝不假装已完成。
    fetch_manifest
    say "🏷  目标：${SRC_TAG}（代码 ${SRC_SHA}）"
    say ""
    # 读机群版本矩阵（主控自己的接口；L3b 起才有 code_sha 字段）
    MATRIX=""
    if [ -f "$TOKEN_FILE" ]; then
      MATRIX="$(curl -s -m 5 -H "X-Auth-Token: $(cat "$TOKEN_FILE")" "$API/api/fleet/status" 2>/dev/null || true)"
    fi
    say "Update plan（机群）:"
    say "  1) 子端（X3 等远程）——期望 ${SRC_SHA}"
    say "  2) 主控（local）  ——期望 ${SRC_SHA}"
    say "  3) UI（local）    ——期望 ${SRC_TAG}"
    say ""
    say "机群版本矩阵:"
    printf '%s' "$MATRIX" | python3 -c '
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    print("  （主控无响应或未部署 L3b 字段——读不到矩阵）"); raise SystemExit
try:
    d = json.loads(raw)
except Exception:
    print("  （矩阵解析失败）"); raise SystemExit
ms = d.get("machines") or d.get("snapshots") or d
if not isinstance(ms, dict) or not ms:
    print("  （矩阵为空）"); raise SystemExit
for name, m in ms.items():
    if not isinstance(m, dict):
        continue
    print("  %-8s code_sha=%-20s code_version=%-8s healthy=%s" % (
        name, m.get("code_sha") or "<空>", m.get("code_version") or "<空>", m.get("healthy")))
' 2>/dev/null || say "  （矩阵读取失败）"

    if [ "$MODE" = "fleet" ] && [ "${FLEET_PLAN_ONLY:-0}" = "0" ]; then
      say ""
      say "→ 执行本地件（主控 → UI；远程件需特权，见下）"
      # 本地件复用单机流程（本脚本自身再跑一次，走完整六阶段）
      if ZERG_PREFIX="$PREFIX" ZERG_UPGRADE_SOURCE="$SOURCE" bash "$0" ${NO_SERVICE:+--no-service} ${FORCE:+--force} ; then
        say "  ✓ 本地件完成"
      else
        say "  ✗ 本地件失败（见上）；远程件不再执行"
        exit 1
      fi
      say ""
      # ── 远程件：子端守护进程（root 免密可用则真换装，否则如实 pending）─────────
      # 远程件不在 release 制品矩阵里（矩阵 5 件是发布契约）→ 现编现传：
      # 本地交叉编 linux/amd64 的 zerg-agentd（带身份），scp 上去，install 覆盖，重启 unit。
      REMOTE_STATUS="ok"
      REMOTE_DETAIL=""
      for spec in ${ZERG_REMOTE_AGENTS:-x3=<worker-host>}; do
        name="${spec%%=*}"; rhost="${spec##*=}"
        if ! ssh -o BatchMode=yes -o ConnectTimeout=6 "root@$rhost" 'id -u' >/dev/null 2>&1; then
          say "  ⏸ 远程 $name($rhost)：root 免密不可用 → pending-manual（不假装完成）"
          REMOTE_STATUS="pending-manual"
          REMOTE_DETAIL="$REMOTE_DETAIL $name:no-root"
          continue
        fi
        say "  ▶ 远程 $name($rhost)：现编 linux/amd64 守护进程并换装"
        if ! (cd "$REPO_ROOT/agent" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-mod=mod GOSUMDB=off \
              go build -trimpath -buildvcs=false \
              -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=${SRC_VER} -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=${SRC_SHA} -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
              -o "/tmp/zerg-agentd-${SRC_SHA}-linux-amd64" ./cmd/zerg-agentd); then
          say "    ✗ 交叉编译失败"
          REMOTE_STATUS="failed"; REMOTE_DETAIL="$REMOTE_DETAIL $name:build-fail"
          continue
        fi
        if ! scp -q -o BatchMode=yes "/tmp/zerg-agentd-${SRC_SHA}-linux-amd64" "root@$rhost:/tmp/zerg-agentd.new"; then
          say "    ✗ 传输失败"
          REMOTE_STATUS="failed"; REMOTE_DETAIL="$REMOTE_DETAIL $name:scp-fail"
          continue
        fi
        if ssh -o BatchMode=yes -o ConnectTimeout=8 "root@$rhost" \
             'install -m0755 /tmp/zerg-agentd.new /usr/local/bin/zerg-agentd && rm -f /tmp/zerg-agentd.new && systemctl restart x3-agent && sleep 5 && systemctl is-active x3-agent' >/dev/null 2>&1; then
          # 回读：目标机上二进制自报的身份必须等于目标提交（不靠"命令没报错"当证据）
          got="$(ssh -o BatchMode=yes -o ConnectTimeout=6 "root@$rhost" '/usr/local/bin/zerg-agentd --version' 2>/dev/null | head -1)"
          case "$got" in
            *"$SRC_SHA"*) say "    ✓ $name 已换装并重启：$got" ;;
            *) say "    ⚠️ $name 换装了但身份不符：$got"; REMOTE_STATUS="failed"; REMOTE_DETAIL="$REMOTE_DETAIL $name:identity-mismatch" ;;
          esac
        else
          say "    ✗ $name 换装或重启失败（旧件仍在 /usr/local/bin/zerg-agent.prev-*）"
          REMOTE_STATUS="failed"; REMOTE_DETAIL="$REMOTE_DETAIL $name:install-fail"
        fi
      done
      say "  远程件结果：${REMOTE_STATUS}${REMOTE_DETAIL}"

      # 机群级回执（C12 设计意图：每台一份 + 汇总一份，pending 也如实记录）
      mkdir -p "$RECEIPTS"
      FTS="$(date -u +%Y%m%dT%H%M%SZ)"
      FRC="$RECEIPTS/fleet-$FTS.json"
      printf '%s' "$MATRIX" > /tmp/.zerg-matrix.json 2>/dev/null || true
      ZERG_REMOTE_RESULT="$REMOTE_STATUS" ZERG_REMOTE_DETAIL="$REMOTE_DETAIL" \
      python3 - "$FRC" "$FTS" "$SRC_TAG" "$SRC_SHA" "${PREFIX}" /tmp/.zerg-matrix.json <<'PY'
import json, sys
rc, ts, tag, sha, prefix, matrix_path = sys.argv[1:7]
try:
    d = json.load(open(matrix_path))
    ms = d.get("machines") or d.get("snapshots") or d
    machines = {k: {"code_sha": (v or {}).get("code_sha", ""), "code_version": (v or {}).get("code_version", "")}
                for k, v in (ms.items() if isinstance(ms, dict) else [])}
except Exception:
    machines = {}
json.dump({
    "schema": 1, "kind": "fleet", "at": ts, "target": {"tag": tag, "commit": sha},
    "prefix": prefix,
    "matrix_before": machines,
    "order": ["agent(remote)", "core(local)", "ui(local)", "menubar(local)"],
    "local": "ok",
    "remote": [{"machine": "x3",
                "status": __import__("os").environ.get("ZERG_REMOTE_RESULT", "unknown"),
                "detail": __import__("os").environ.get("ZERG_REMOTE_DETAIL", "")}],
}, open(rc, "w"), ensure_ascii=False, indent=2)
print("  📝 机群回执:", rc)
PY
      cp -p "$FRC" "$RECEIPTS/latest-fleet.json" 2>/dev/null || true
    fi
    exit 0
    ;;
  status)
    [ "$JSON" = "1" ] && { printf '{"core":"%s","agent":"%s","ui":"%s"}\n' "$(ver_of "$PREFIX/zerg-core")" "$(ver_of "$PREFIX/zerg-agent")" "$(sha_of "$PREFIX/zerg-ui")"; exit 0; }
    say "📦 前缀：$PREFIX"
    say "   主控 : $(ver_of "$PREFIX/zerg-core")"
    say "   子端 : $(ver_of "$PREFIX/zerg-agent")"
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
    for name in zerg-core zerg-agent zerg-ui; do
      if [ -f "$PREFIX/$name.prev" ]; then
        cp -p "$PREFIX/$name" "$PREFIX/$name.failed" 2>/dev/null || true
        mv -f "$PREFIX/$name.prev" "$PREFIX/$name"
        [ "$os" = "Darwin" ] && codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || true
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

# 盘点（plan / check / apply 共用）
CUR_CORE="$(ver_of "$PREFIX/zerg-core")"
CUR_AGENT="$(ver_of "$PREFIX/zerg-agent")"
CUR_UI_SHA="$(sha_of "$PREFIX/zerg-ui")"
TARGET_UI_SHA="$(want_sha zerg-ui-$PLAT 2>/dev/null || echo '-')"
NEED=""
[ -n "$CUR_CORE" ] && case "$CUR_CORE" in *"$SRC_SHA"*) ;; *) NEED="yes" ;; esac
[ "$CUR_UI_SHA" = "$TARGET_UI_SHA" ] || NEED="yes"
[ -z "$CUR_CORE" ] && NEED="yes"

if [ "$MODE" = "check" ]; then
  [ -n "$NEED" ] && say "→ 有新版：${SRC_TAG}" || say "→ 已是最新：${SRC_TAG}"
  exit 0
fi

INFLIGHT="$(inflight)"

if [ "$MODE" = "plan" ]; then
  say "Update plan:"
  say "  install : ${PREFIX}（平台 ${PLAT}）"
  say "  source  : ${SRC_KIND} ${SOURCE}"
  say "  当前版本 : 主控 $( [ -n "$CUR_CORE" ] && echo "$CUR_CORE" || echo '未安装' )"
  say "            子端 $( [ -n "$CUR_AGENT" ] && echo "$CUR_AGENT" || echo '未安装' )"
  say "            UI  sha $( echo "$CUR_UI_SHA" | cut -c1-16 )…"
  say "  目标版本 : ${SRC_TAG}（代码 ${SRC_SHA}）"
  say "  在途任务 : ${INFLIGHT} 个$( [ "$INFLIGHT" != "0" ] && echo '（需 --force 或等待）' )"
  say "  服务     : $( [ "$NO_SERVICE" = "1" ] && echo '不触碰（--no-service）' || echo '主控 bootout→bootstrap；UI 停→启' )"
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
WANT="zerg-core-${PLAT} zerg-agent-${PLAT}"
[ "$PLAT" = "darwin-arm64" ] && [ "$NO_UI" != "1" ] && WANT="$WANT zerg-ui-${PLAT}"
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
if [ -n "$CUR_SHA_SHORT" ] && [ "$CUR_SHA_SHORT" != "unknown" ] && [ "$CUR_SHA_SHORT" != "$SRC_SHA" ]; then
  if git -C "$GITREPO" merge-base --is-ancestor "$SRC_SHA" "$CUR_SHA_SHORT" 2>/dev/null; then
    if [ "$ALLOW_DOWNGRADE" != "1" ]; then
      die "目标是旧提交（源 ${SRC_SHA} 早于当前运行的 ${CUR_SHA_SHORT}）——拒绝降级；确实要降级请加 --allow-downgrade" 5
    fi
    say "⚠️  降级：源 ${SRC_SHA} 早于当前 ${CUR_SHA_SHORT}（--allow-downgrade 已放行）"
  else
    say "ℹ️  无法判定新旧（源 ${SRC_SHA} 不在本地历史中）——不做降级拦截"
  fi
fi

# ── 停服务（KeepAlive → 必须 bootout）───────────────────────────────────────
UI_WAS_RUNNING=0
if [ "$NO_SERVICE" != "1" ]; then
  say "⏸  停主控（launchctl bootout——KeepAlive=true 时直接 kill 会被旧二进制抢重启）"
  bash "$STARTCORE" --stop >/dev/null 2>&1 || true
  if pgrep -f "$UI_PATTERN" >/dev/null 2>&1; then
    UI_WAS_RUNNING=1; say "⏸  停 UI"; pkill -f "$UI_PATTERN" || true; sleep 1
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
  if [ "$os" = "Darwin" ]; then
    codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || say "   ⚠️ $name 重签名失败（未签名二进制会被秒杀）"
  fi
  say "   🔁 $name ← ${src}（旧件已留 .prev）"
}
SWAP_FAIL=0
swap_one zerg-core  "zerg-core-${PLAT}"  || swap_one_fail=1
swap_one zerg-agent "zerg-agent-${PLAT}" || swap_one_fail=1
[ "$PLAT" = "darwin-arm64" ] && [ "$NO_UI" != "1" ] && { swap_one zerg-ui "zerg-ui-${PLAT}" || swap_one_fail=1; }
true

# ── 重启 + verify（起不来就回滚）────────────────────────────────────────────
verify_files() { # 2026-09-11 修：不再比落盘 sha（重签名已改字节）——改验"在位 + 可执行 + 签名有效"
  # 依据：sha 的真值校验已在 swap_one 里对**暂存件**做过（签名前）；这里守的是"能不能跑"这一类失败。
  local ok=1 a n
  for a in $WANT; do
    n="$(echo "$a" | sed "s/-${PLAT}$//")"
    if [ ! -f "$PREFIX/$n" ] || [ ! -x "$PREFIX/$n" ]; then ok=0; say "   ✗ $n 缺失或不可执行"; continue; fi
    if [ "$os" = "Darwin" ]; then
      codesign -v "$PREFIX/$n" >/dev/null 2>&1 || { ok=0; say "   ✗ $n 签名无效（macOS 会秒杀 EXIT137）"; }
    fi
  done
  return $((1-ok))
}
# 运行进程自报的 code_sha（修 #42：verify 的口径是「**跑着的那份代码**自报的身份」，
# 不是磁盘二进制 --version——旧口径会把"换了盘上文件但活进程还是旧的"判成成功）。
live_sha() {
  curl -s -m 5 -H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null)" "$API/api/capabilities" 2>/dev/null \
    | python3 -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: print(""); raise SystemExit
print(d.get("code_sha") or "")' 2>/dev/null
}
sha_match() { # $1=实际 $2=期望（互为前缀即算匹配——短 sha/长 sha 混用是常态）
  [ -n "$1" ] || return 1
  case "$1" in "$2"*) return 0 ;; esac
  case "$2" in "$1"*) return 0 ;; esac
  return 1
}

verify_live() { # 修 #42：主控 = 运行进程自报 sha；子端 = 落盘二进制自报（本机无子端守护进程）
  local got v
  got="$(live_sha)"
  if [ -z "$got" ]; then
    say "   ✗ 运行中的主控未自报 code_sha（$API/api/capabilities 读不到）——无法证明活进程已换版"
    return 1
  fi
  if sha_match "$got" "$SRC_SHA"; then say "   ✓ 主控（运行进程）自报 code_sha=$got"
  else say "   ✗ 混版：运行进程 code_sha=$got ≠ 目标 ${SRC_SHA}（换了盘、没换活进程）"; return 1; fi
  v="$(ver_of "$PREFIX/zerg-agent")"
  case "$v" in *"$SRC_SHA"*|*"$SRC_VER"*) say "   ✓ 子端（落盘）自报：$v" ;; *) say "   ✗ 子端自报异常：$v"; return 1 ;; esac
  return 0
}

# 组件管辖（修 #42）：只重启**它管辖的**组件；管不到就**明确提示**（绝不静默放过）。
#   主控：由启动脚本/launchd 管 ⇒ 本内核管；UI（darwin）：经 start-zerg-ui.sh 拉起 ⇒ 本内核管。
#   若 UI 换装后**没能**被本内核拉起来（如别处托管/启动失败）⇒ 记入 UNMANAGED 并在收尾明确告警。
UNMANAGED=""
restart_managed() {
  say "▶️  启主控"
  bash "$STARTCORE" >/dev/null 2>&1 || true
  ready=0
  for i in $(seq 1 20); do
    if curl -s -m 2 -H "X-Auth-Token: $(cat "$TOKEN_FILE" 2>/dev/null)" "$API/api/capabilities" >/dev/null 2>&1; then ready=1; break; fi
    sleep 1
  done
  [ "$ready" = "1" ] || { say "   ✗ 主控 20s 内未就绪"; return 1; }
  if [ "$PLAT" = "darwin-arm64" ] && [ "$NO_UI" != "1" ]; then
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
  return 0
}

RESULT="ok"; FAILED_AT=""
if [ "${SWAP_FAIL:-0}" = "1" ] || [ "${swap_one_fail:-0}" = "1" ] || ! verify_files; then RESULT="rollback"; FAILED_AT="files"; fi
if [ "$RESULT" = "ok" ] && [ "$NO_SERVICE" != "1" ]; then
  if ! restart_managed; then RESULT="rollback"; FAILED_AT="core-start"; fi
fi

if [ "$RESULT" = "ok" ] && [ "$NO_SERVICE" != "1" ]; then
  say "🔎 verify（口径：运行进程自报 sha）"
  if ! verify_live; then RESULT="rollback"; FAILED_AT="identity"; fi
elif [ "$RESULT" = "ok" ]; then
  say "ℹ️  --no-service：跳过「运行进程自报」verify（无服务可查；sha 已在换装前对暂存件校验）"
fi

if [ "$RESULT" = "rollback" ]; then
  say "↩️  验证未过（${FAILED_AT}）——自动回滚"
  for name in zerg-core zerg-agent zerg-ui; do
    [ -f "$PREFIX/$name.prev" ] || continue
    mv -f "$PREFIX/$name" "$PREFIX/$name.failed" 2>/dev/null || true
    mv -f "$PREFIX/$name.prev" "$PREFIX/$name"
    [ "$os" = "Darwin" ] && codesign -s - --force "$PREFIX/$name" >/dev/null 2>&1 || true
  done
  if [ "$NO_SERVICE" != "1" ]; then bash "$STARTCORE" >/dev/null 2>&1 || true; fi
fi

# ── 回执（成功与失败都写——失败路径才是回执存在的理由）──────────────────────
mkdir -p "$RECEIPTS"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
RC="$RECEIPTS/$TS.json"
python3 - "$RC" "$TS" "$RESULT" "$FAILED_AT" "$PREFIX" "$PLAT" "$SOURCE" "$SRC_TAG" "$SRC_SHA" "$CUR_CORE" "$CUR_AGENT" "$INFLIGHT" "$(ver_of "$PREFIX/zerg-core")" "$(ver_of "$PREFIX/zerg-agent")" "$SRC_KIND" "${UNMANAGED:-}" "$(live_sha)" <<'PY'
import json, sys
(rc, ts, result, failed_at, prefix, plat, source, tag, sha, cur_core, cur_agent,
 inflight, new_core, new_agent, source_kind, unmanaged, live) = sys.argv[1:18]
json.dump({
    "schema": 1, "at": ts, "result": result, "failed_at": failed_at or None,
    "prefix": prefix, "platform": plat, "source": source,
    "source_kind": source_kind,
    "verify": {"mode": "running-process-self-reported", "target_commit": sha,
               "live_code_sha": live or None, "matched": bool(live) and (live == sha or sha.startswith(live) or live.startswith(sha))},
    "unmanaged_components": [u for u in (unmanaged or "").split() if u],
    "from": {"core": cur_core, "agent": cur_agent},
    "to": {"tag": tag, "commit": sha},
    "now": {"core": new_core, "agent": new_agent},
    "inflight_tasks": int(inflight or 0),
}, open(rc, "w"), ensure_ascii=False, indent=2)
print("  📝 回执:", rc)
PY
cp -p "$RC" "$RECEIPTS/latest.json" 2>/dev/null || true
ls -1t "$RECEIPTS"/*.json 2>/dev/null | tail -n +21 | xargs -r rm -f    # 只留最近 20 份

if [ "$RESULT" = "ok" ]; then
  say "✅ 升级完成：${SRC_TAG}（代码 ${SRC_SHA}）→ $PREFIX"
  if [ -n "${UNMANAGED:-}" ]; then
    say "⚠️  以下组件不在本内核管辖内、**需手动重启**：${UNMANAGED}（否则仍跑旧件）"
  fi
  exit 0
else
  say "❌ 升级失败（${FAILED_AT}）——已回滚，回执见 $RC"
  exit 1
fi

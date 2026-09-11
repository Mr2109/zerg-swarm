#!/usr/bin/env bash
# install.sh — 虫族 Zerg 安装 / 升级（装升共用一份逻辑）（2026-09-11，自动升级模块 P0-d）
#
# 设计要点（对齐 Hermes 的升级纪律）：
#   · 安装与升级**同一条代码路径**——升级就是"装一个更新的版本"，避免两套逻辑各自长歪
#   · 安装永远显式（用户敲命令），**不后台静默换二进制**
#   · 每一步"能被证明"：sha256 校验不过就拒绝，绝不留半装状态
#   · 换装原子：先写 .new 再 mv，旧件留 .prev（回滚只用一条 mv）
#   · --dry-run 全程只读，先看清楚再动手（对齐 `hermes update --plan`）
#
# 用法：
#   bash install.sh --check                          # 只问：源上是什么版本、我装的是什么版本
#   bash install.sh --dry-run                        # 只读：打印将要做什么
#   bash install.sh                                  # 从最新 Release 安装/升级到默认前缀
#   bash install.sh --prefix ~/zerg-swarm            # 指定安装前缀
#   ZERG_UPGRADE_SOURCE=file:///path/to/release bash install.sh   # 用本地假源（演练/测试）
#   ZERG_UPGRADE_SOURCE=https://github.com/Mr2109/zerg-swarm bash install.sh --tag v2.5.9
#
# 源的两形态（读到的 manifest.json 必须同形）：
#   file://<dir>            —— 目录里有 manifest.json + 制品（本地演练）
#   https://github.com/...  —— 走 GitHub Release 资产（真源，用 gh 或 curl）
set -euo pipefail

REPO_SLUG="Mr2109/zerg-swarm"
SOURCE="${ZERG_UPGRADE_SOURCE:-https://github.com/$REPO_SLUG}"
PREFIX="${ZERG_PREFIX:-$HOME/zerg-swarm}"
TAG=""
MODE="install"
SERVICE=0

while [ $# -gt 0 ]; do
  case "$1" in
    --check)   MODE="check"; shift ;;
    --dry-run) MODE="dry"; shift ;;
    --prefix)  PREFIX="$2"; shift 2 ;;
    --tag)     TAG="$2"; shift 2 ;;
    --service) SERVICE=1; shift ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 64 ;;
  esac
done

say() { printf '%s\n' "$*"; }
die() { printf '❌ %s\n' "$*" >&2; exit 1; }

os="$(uname -s)"; arch="$(uname -m)"
case "$os-$arch" in
  Darwin-arm64) PLAT="darwin-arm64" ;;
  Linux-x86_64) PLAT="linux-amd64" ;;
  *) die "本版仅提供 darwin-arm64 与 linux-amd64（当前 $os-${arch}）——请看 README 的构建说明" ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# ── 1. 取源（file:// 假源 或 GitHub Release）────────────────────────────────
if [[ "$SOURCE" == file://* ]]; then
  SRCDIR="${SOURCE#file://}"
  [ -f "$SRCDIR/manifest.json" ] || die "假源缺 manifest.json：$SRCDIR"
  cp -p "$SRCDIR/manifest.json" "$work/manifest.json"
  SRCMODE="file"
else
  command -v gh >/dev/null || die "读 GitHub Release 需要 gh（brew install gh）；或改用 ZERG_UPGRADE_SOURCE=file://... 演练"
  ref="${TAG:-latest}"
  gh release download ${TAG:+"$TAG"} --repo "$REPO_SLUG" --pattern manifest.json --dir "$work" --clobber 2>/dev/null \
    || gh release download --repo "$REPO_SLUG" --pattern manifest.json --dir "$work" --clobber \
    || die "取不到 release manifest（仓库当前可能还没有 Release）"
  SRCMODE="github"
fi

jget() { python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(eval(sys.argv[2],{"d":d}))' "$1" "$2"; }
SRC_VER="$(jget "$work/manifest.json" 'd["version"]')"
SRC_SHA="$(jget "$work/manifest.json" 'd["commit"]')"
SRC_TAG="$(jget "$work/manifest.json" 'd["tag"]')"
say "🏷  源：$SRC_TAG （代码 ${SRC_SHA}）"

# ── 2. 本地已装版本（身份比对，不靠猜）──────────────────────────────────────
CUR=""
[ -x "$PREFIX/bin/zerg-core" ] && CUR="$("$PREFIX/bin/zerg-core" --version 2>/dev/null | head -1 || true)"
say "📦 目标前缀：$PREFIX"
say "   本机已装：${CUR:-（未安装）}"

if [ "$MODE" = "check" ]; then
  say "→ 源 $SRC_TAG / 本机 ${CUR:-未装}——$( [ -n "$CUR" ] && echo '如需升级：bash install.sh' || echo '如需安装：bash install.sh' )"
  exit 0
fi

# ── 3. 选件（按 manifest，不靠文件名猜）────────────────────────────────────
WANT="zerg-core-$PLAT zerg-agent-$PLAT"
[ "$PLAT" = "darwin-arm64" ] && WANT="$WANT zerg-ui-$PLAT"
say "📋 将安装：$WANT"

if [ "$MODE" = "dry" ]; then
  say "（--dry-run：只读，不下载不落盘）"
  if [ "$SRCMODE" = "file" ]; then
    for a in $WANT; do
      h=$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(next(x["sha256"] for x in d["artifacts"] if x["name"]==sys.argv[2]))' "$work/manifest.json" "$a")
      say "   $a  sha256=${h:0:16}…"
    done
  fi
  say "   → 安装到 $PREFIX/bin/（旧件留 *.prev，可 mv 回滚）；macOS 上逐件 codesign -s - --force"
  exit 0
fi

# ── 4. 取件 + 校验（不过就拒绝，绝不留半装）────────────────────────────────
mkdir -p "$work/dl"
for a in $WANT; do
  if [ "$SRCMODE" = "file" ]; then
    cp -p "$SRCDIR/$a" "$work/dl/$a" || die "假源缺件：$a"
  else
    gh release download ${TAG:+"$TAG"} --repo "$REPO_SLUG" --pattern "$a" --dir "$work/dl" --clobber || die "下载失败：$a"
  fi
  want=$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(next(x["sha256"] for x in d["artifacts"] if x["name"]==sys.argv[2]))' "$work/manifest.json" "$a")
  got=$(shasum -a 256 "$work/dl/$a" 2>/dev/null | awk '{print $1}' || sha256sum "$work/dl/$a" | awk '{print $1}')
  [ "$want" = "$got" ] || die "$a 校验失败（期望 ${want:0:16}…，实际 ${got:0:16}…）——拒绝安装"
  say "   ✅ $a  sha256 ${got:0:12}…"
done

# ── 5. 原子换装（.new → mv；旧件留 .prev）──────────────────────────────────
say "📂 更换：$PREFIX/bin/"
mkdir -p "$PREFIX/bin"
for a in $WANT; do
  name="$(echo "$a" | sed "s/-$PLAT$//")"
  cp -p "$work/dl/$a" "$PREFIX/bin/$name.new"
  [ -f "$PREFIX/bin/$name" ] && mv -f "$PREFIX/bin/$name" "$PREFIX/bin/$name.prev"
  mv -f "$PREFIX/bin/$name.new" "$PREFIX/bin/$name"
  chmod +x "$PREFIX/bin/$name"
  if [ "$os" = "Darwin" ]; then
    codesign -s - --force "$PREFIX/bin/$name" >/dev/null 2>&1 || say "   ⚠️ 重签名失败：${name}（macOS 上未签名二进制会被秒杀）"
    say "   🔏 已重签名 $name"
  fi
done
cp -p "$work/manifest.json" "$PREFIX/manifest.json"

# ── 6. 服务（可选；launchd/systemd 由用户显式要求）──────────────────────────
if [ "$SERVICE" = "1" ]; then
  say "🛠  --service：请按 README 的托管步骤安装服务（本脚本不擅自改你的 launchd/systemd）"
fi

say ""
say "✅ 完成：$SRC_TAG （代码 ${SRC_SHA}）→ $PREFIX/bin/"
say "   主控自报身份：$("$PREFIX/bin/zerg-core" --version 2>/dev/null | head -1 || echo '（用 --version 查询）')"
say "   回滚：mv $PREFIX/bin/zerg-core.prev $PREFIX/bin/zerg-core（其余同）"

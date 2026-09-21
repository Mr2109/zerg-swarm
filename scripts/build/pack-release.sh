#!/usr/bin/env bash
# pack-release.sh — 打出与 GitHub Release 同形的制品目录（2026-09-11，自动升级模块 P0-b）
#
# 为什么：自动升级的下载/校验阶段需要一个**形状固定**的制品目录，无论是
#   真源（GitHub Release 资产）还是假源（本地 file:// 演练）都必须长得一样。
#   本脚本就是"真源长什么样"的权威定义。
#
# 产物布局（dist/<版本>/release/）：
#   zerg-core-darwin-arm64        + .sha256
#   zerg-agent-darwin-arm64       + .sha256
#   zerg-ui-darwin-arm64          + .sha256
#   zerg-wall-darwin-arm64        + .sha256      ← 茧壁（Rust；linux 件由 CI 出）
#   zerg-cli-darwin-arm64         + .sha256      ← 命令面 CLI（Go；本机件 dist/<版本>/zerg）
#   zerg-core-linux-amd64         + .sha256      ← 纯 Go 交叉编译（modernc sqlite，CGO_ENABLED=0）
#   zerg-agent-linux-amd64        + .sha256
#   zerg-cli-linux-amd64          + .sha256      ← 同上（实测可编 ⇒ 本机也出，不是 CI-only）
#   checksums.txt                  ← 全部 sha256（sha256sum -c 兼容格式）
#   manifest.json                  ← 版本/代码身份/逐件 size+sha256（升级器读它，不靠文件名猜）
#
# 组件名为什么是 `cli`（2026-09-21 补件）：制品名契约是 `zerg-<组件>-<os>-<arch>` 四段（见
#   `core/internal/selfupdate/build.go:97` · `scripts/build/zerg-upgrade.sh:735` · `publish/install.sh:123`），
#   `scripts/evals/make-manifest.py` 也按四段切 ⇒ 三段的 `zerg-darwin-arm64` 会让清单生成直接报错。
#
# 用法：
#   bash scripts/build/pack-release.sh              # 构建 + 打包（含 UI，约 4 分钟）
#   bash scripts/build/pack-release.sh --no-build   # 复用 dist/<版本>/ 里已构建的二进制，只重打包
#   bash scripts/build/pack-release.sh --verify     # 只校验：manifest + 逐件 sha256 必须与文件一致
#
# 注：UI 只出 darwin-arm64（Rust 交叉到 linux 需额外链接器/工具链，本版不做——见设计稿 L4）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

MODE="build"
for arg in "$@"; do
  case "$arg" in
    --no-build) MODE="pack" ;;
    --verify) MODE="verify" ;;
    *) echo "未知参数: $arg" >&2; exit 64 ;;
  esac
done

VERSION="$(grep -m1 '^const Version = ' core/internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')"
DIST="$REPO_ROOT/dist/$VERSION"
REL="$DIST/release"

sha_of() { shasum -a 256 "$1" | awk '{print $1}'; }

# ── 组装（二进制 → 带平台后缀的制品名）──
if [ "$MODE" = "build" ]; then
  echo "=== 构建 ==="
  bash "$REPO_ROOT/scripts/build/build-all.sh" --dist
elif [ "$MODE" = "pack" ]; then
  echo "=== 跳过构建（复用 ${DIST}）==="
fi

if [ "$MODE" != "verify" ]; then
  [ -f "$DIST/build-info.json" ] || { echo "❌ 缺 $DIST/build-info.json——先跑 build-all.sh --dist" >&2; exit 1; }
  for b in zerg zerg-core zerg-agent zerg-ui; do
    [ -f "$DIST/$b" ] || { echo "❌ 缺 $DIST/$b" >&2; exit 1; }
  done

  echo "=== 交叉编译 Go 三件（linux/amd64，CGO_ENABLED=0）==="
  LDFLAGS="-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=$(python3 -c 'import json;print(json.load(open("'"$DIST"'/build-info.json"))["build_time"])')"
  (cd core && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-mod=mod GOSUMDB=off \
     go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$DIST/zerg-core-linux-amd64" ./cmd/zerg-core)
  (cd core && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-mod=mod GOSUMDB=off \
     go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$DIST/zerg-agent-linux-amd64" ./cmd/zerg-agent)
  # 命令面 CLI：纯 Go，与上面两件走**同一条**交叉编路径（实测 GOOS=linux GOARCH=amd64 CGO_ENABLED=0 可编，
  #   故它不是 CI-only 件 —— 本机档矩阵里有它，见 make-manifest.py 的 EXPECTED/CI_ONLY 口径）。
  (cd core && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-mod=mod GOSUMDB=off \
     go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$DIST/zerg-cli-linux-amd64" ./cmd/zerg)

  echo "=== 组装 release 目录 ==="
  rm -rf "$REL"
  mkdir -p "$REL"
  HOST_ARCH="$(uname -m)"
  cp -p "$DIST/zerg-core"  "$REL/zerg-core-darwin-$HOST_ARCH"
  cp -p "$DIST/zerg-agent" "$REL/zerg-agent-darwin-$HOST_ARCH"
  cp -p "$DIST/zerg-ui"    "$REL/zerg-ui-darwin-$HOST_ARCH"
  # 命令面 CLI（本机件）：名字按 `zerg-<组件>-<平台>` 契约加平台后缀（组件名 cli，理由见文件头）。
  cp -p "$DIST/zerg"       "$REL/zerg-cli-darwin-$HOST_ARCH"
  # 茧壁（Rust）：本机只出 darwin；linux 件由 CI 的 ubuntu runner 产出（Rust 不做本地交叉编）。
  ( cd "$REPO_ROOT/wall" && cargo build --release >/dev/null 2>&1 ) || { echo "!! 茧壁构建失败（wall/）"; exit 1; }
  cp -p "$REPO_ROOT/wall/target/release/zerg-wall" "$REL/zerg-wall-darwin-$HOST_ARCH" 2>/dev/null || \
    cp -p "$REPO_ROOT/wall/target/release/wall" "$REL/zerg-wall-darwin-$HOST_ARCH" || { echo "!! 茧壁产物名不符（找 zerg-wall/wall）"; exit 1; }
  cp -p "$DIST/zerg-core-linux-amd64"  "$REL/zerg-core-linux-amd64"
  cp -p "$DIST/zerg-agent-linux-amd64" "$REL/zerg-agent-linux-amd64"
  cp -p "$DIST/zerg-cli-linux-amd64"   "$REL/zerg-cli-linux-amd64"

  for f in "$REL"/zerg-*; do
    case "$f" in *.sha256) continue ;; esac
    echo "$(sha_of "$f")  $(basename "$f")" > "$f.sha256"
  done
  (cd "$REL" && shasum -a 256 zerg-* | grep -v '\.sha256' > checksums.txt)

  # 清单生成走共享脚本（CI 同一份——排除"两处逻辑长歪"）。
  # ★ 本机档（2026-09-21 修 G1）：本脚本是**本地打包路径** —— 上面茧壁那一段只编得出 darwin 一件
  #   （Rust 不本地交叉编 ⇒ 矩阵里天然没有 CI-only 的 `zerg-wall-linux-amd64`）⇒ 这里显式导出本机档。
  #   契约没被放宽：EXPECTED 7 件原样（CI 侧仍走严格档），本机档 = EXPECTED − CI_ONLY 这个**显式差集**，
  #   多一件少一件照样拒出清单（口径见 scripts/evals/make-manifest.py 顶部「两档口径」）。
  BI="$DIST/build-info.json"
  ZERG_MANIFEST_LOCAL=1 python3 "$REPO_ROOT/scripts/evals/make-manifest.py" "$REL" \
    "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["version"])' "$BI")" \
    "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["commit"])' "$BI")" \
    "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["build_time"])' "$BI")"
  echo "✅ 打包完成：$REL"
fi

# ── 校验（真源与假源共用同一把尺）──
echo "=== 校验 manifest + 逐件 sha256 ==="
python3 - "$REL" <<'PY'
import hashlib, json, os, sys
rel = sys.argv[1]
mp = os.path.join(rel, "manifest.json")
if not os.path.exists(mp):
    print("❌ 缺 manifest.json"); sys.exit(1)
m = json.load(open(mp))
bad = []
for a in m["artifacts"]:
    p = os.path.join(rel, a["name"])
    if not os.path.exists(p):
        bad.append("%s 缺失" % a["name"]); continue
    real = hashlib.sha256(open(p, "rb").read()).hexdigest()
    if real != a["sha256"]:
        bad.append("%s sha256 不符（manifest %s… vs 实际 %s…）" % (a["name"], a["sha256"][:12], real[:12]))
    if os.path.getsize(p) != a["size"]:
        bad.append("%s 体积不符（manifest %d vs 实际 %d）" % (a["name"], a["size"], os.path.getsize(p)))
print("   版本 %s · 代码 %s · 逐件 %d" % (m["version"], m["commit"], len(m["artifacts"])))
for a in m["artifacts"]:
    print("     %-30s %9d 字节  %s" % (a["name"], a["size"], a["sha256"][:16] + "…"))
if bad:
    print("❌ 校验失败："); [print("   " + b) for b in bad]; sys.exit(1)
print("✅ 校验通过（manifest 与文件一致）")
PY

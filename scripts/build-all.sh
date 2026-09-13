#!/usr/bin/env bash
# build-all.sh — 构建三件产物并写入代码身份（commit + 构建时间）（2026-09-11，自动升级模块 P0-a）
#
# 为什么需要统一构建入口：自动升级的 verify 阶段要比对"活进程的 code_sha vs 目标制品的 code_sha"，
# 所以**每个二进制都必须带身份**。手敲 go build 会漏注入 → 升级验证直接失效。
#
# 用法：
#   bash scripts/build-all.sh                    # 产出到 bin/（本机部署用）
#   bash scripts/build-all.sh --dist             # 产出到 dist/<版本>/（打包发布用，含 sha256）
#   bash scripts/build-all.sh --no-ui            # 只建 Go 两件（快）
#   bash scripts/build-all.sh --public          # UI 用 --no-default-features（对齐公开快照形态）
#   bash scripts/build-all.sh --no-sign          # 跳过 codesign（非 macOS / 调试）
#
# 身份注入：
#   Go  → -ldflags -X .../internal/version.{Commit,BuildTime}（version.go 里是 var，可注入）
#   UI  → ui/build.rs 读 git/date 写 cargo:rustc-env（ZERG_GIT_SHA / ZERG_BUILD_TIME）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

DIST=0
BUILD_UI=1
SIGN=1
for arg in "$@"; do
  case "$arg" in
    --dist) DIST=1 ;;
    --no-ui) BUILD_UI=0 ;;
    --public) PUBLIC=1 ;;   # 本地形态对齐公开快照（关私有默认 feature：示例虫茧等）
    --no-sign) SIGN=0 ;;
    *) echo "未知参数: $arg" >&2; exit 64 ;;
  esac
done

VERSION="$(grep -m1 '^const Version = ' core/internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')"
if [ -z "$VERSION" ]; then
  echo "❌ 无法从 core/internal/version/version.go 读到版本号" >&2
  exit 1
fi
SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
if [ "$SHA" != "unknown" ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  SHA="${SHA}+dirty"
fi
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [ "$DIST" = "1" ]; then
  OUT="$REPO_ROOT/dist/$VERSION"
else
  OUT="$REPO_ROOT/bin"
fi
mkdir -p "$OUT"

GOFLAGS_="-trimpath -buildvcs=false"
LDFLAGS="-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=${SHA} -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=${BUILD_TIME}"

echo "🏷  版本 $VERSION · 代码 $SHA · 构建 $BUILD_TIME"
echo "📁 输出 $OUT"

echo "→ 主控 zerg-core"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-core" ./cmd/zerg-core)

echo "→ 子端 zerg-agent"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-agent" ./cmd/zerg-agent)

# 子端守护进程（agent 模块）：**与 CLI 区分命名**——带 d = daemon/常驻（ps 里一眼看出在跑服务还是任务）
# 注意：它暂不进 release 制品矩阵（矩阵 5 件是发布契约，要不要加是单独的决定），故始终产出到 bin/。
echo "→ 守护进程 zerg-agentd（agent 模块，注入 agent 自己的 version 包）"
(cd agent && GOFLAGS=-mod=mod GOSUMDB=off go build -trimpath -buildvcs=false \
   -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=${VERSION} -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=${SHA} -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=${BUILD_TIME}" \
   -o "${REPO_ROOT}/bin/zerg-agentd" ./cmd/zerg-agentd)

# 文档虫茧的自带 Go 服务（`cocoon-docs-service`）——**独立仓** `zerg-cocoon/文档/service`。
# 为什么由宿主构建链产出：茧侧 `DocsService::default_bin()` 按「**与宿主可执行文件同目录**」
# 找它 ⇒ 必须与 zerg-ui 同落 `bin/`（服务跑在本机）。
# 边界（红线）：只构建不启动、只落 `bin/`（**不进** dist 制品矩阵、不上 scp）、
# **绝不交叉编译**（非 darwin-arm64 直接跳过）；<container-repo>不在（公开快照形态/未 clone）或无 go ⇒
# 打印原因后跳过，**不阻断**构建（茧未配置 ⇒ 界面提示「未就绪」，优雅降级）。
COCOON_SERVICE_DIR="$REPO_ROOT/../zerg-cocoon/文档/service"
if [ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ] \
   && [ -d "$COCOON_SERVICE_DIR" ] && command -v go >/dev/null 2>&1; then
  echo "→ 文档茧自带服务 cocoon-docs-service（独立仓）"
  (cd "$COCOON_SERVICE_DIR" && GOFLAGS=-mod=mod GOSUMDB=off go build -trimpath -buildvcs=false \
     -o "${REPO_ROOT}/bin/cocoon-docs-service" .)
else
  echo "→ 跳过 cocoon-docs-service（非 darwin-arm64 本机 / <container-repo>不在 / 无 go）"
fi

if [ "$BUILD_UI" = "1" ]; then
  echo "→ UI zerg-ui（cargo release）"
  # 体积对齐(2026-09-11)：公开快照会解除 ui/Cargo.toml 的 zerg-roundtable 跨仓依赖（私有默认开着）
  # 实测同一命令/同一 profile 下 __text：私有 32.1MB vs 公开 10.7MB —— 差异来自源码形态，不是构建旗标
  if [ "${PUBLIC:-0}" = "1" ]; then
    (cd ui && cargo build --release -p zerg-ui --no-default-features)
  else
    (cd ui && cargo build --release -p zerg-ui)
  fi
  cp -p ui/target/release/zerg-ui "$OUT/zerg-ui"
fi

if [ "$SIGN" = "1" ] && command -v codesign >/dev/null 2>&1; then
  for b in "$OUT"/zerg-core "$OUT"/zerg-agent "$OUT"/zerg-ui "$REPO_ROOT"/bin/zerg-agentd "$REPO_ROOT"/bin/cocoon-docs-service; do
    [ -f "$b" ] || continue
    codesign -s - --force "$b" >/dev/null 2>&1 && echo "   🔏 已重签名 $(basename "$b")"
  done
fi

# 构建身份落盘（打包/升级器读它——不靠二进制自报，因为交叉编译的 linux 件跑不起来）
cat > "$OUT/build-info.json" <<EOF
{
  "version": "$VERSION",
  "commit": "$SHA",
  "build_time": "$BUILD_TIME",
  "builder_host": "$(uname -s)/$(uname -m)"
}
EOF
echo "   📝 build-info.json: version=$VERSION commit=$SHA"

if [ "$DIST" = "1" ]; then
  echo "→ 生成校验和（sha256）"
  (cd "$OUT" && shasum -a 256 zerg-* > checksums.txt && cat checksums.txt | sed 's/^/   /')
fi

echo "✅ 构建完成"
for b in "$OUT"/zerg-*; do
  [ -f "$b" ] || continue
  printf "   %-14s %s 字节\n" "$(basename "$b")" "$(stat -f%z "$b" 2>/dev/null || stat -c%s "$b")"
done
# 茧服务始终落 bin/（不进 dist 制品矩阵）——单独列出，免得它没进上面的 zerg-* 清单被漏看。
if [ -f "${REPO_ROOT}/bin/cocoon-docs-service" ]; then
  printf "   %-14s %s 字节（文档茧自带服务）\n" "cocoon-docs-service" \
    "$(stat -f%z "${REPO_ROOT}/bin/cocoon-docs-service" 2>/dev/null || stat -c%s "${REPO_ROOT}/bin/cocoon-docs-service")"
fi

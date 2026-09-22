#!/usr/bin/env bash
# build-all.sh — 构建三件产物并写入代码身份（commit + 构建时间）（2026-09-11，自动升级模块 P0-a）
#
# 为什么需要统一构建入口：自动升级的 verify 阶段要比对"活进程的 code_sha vs 目标制品的 code_sha"，
# 所以**每个二进制都必须带身份**。手敲 go build 会漏注入 → 升级验证直接失效。
#
# 用法：
#   bash scripts/build/build-all.sh                    # 产出到 bin/（本机部署用）
#   bash scripts/build/build-all.sh --dist             # 产出到 dist/<版本>/（打包发布用，含 sha256）
#                                                      #    ★ **只写 dist/**：一个字都不写 bin/、不重签 bin/（2026-09-21 · G4）
#   bash scripts/build/build-all.sh --no-ui            # 只建 Go 两件（快）
#   bash scripts/build/build-all.sh --public          # UI 用 --no-default-features（对齐公开快照形态）
#   bash scripts/build/build-all.sh --no-sign                  # 跳过 codesign（非 macOS / 调试）
#   bash scripts/build/build-all.sh --only-cli                 # 只出命令面 bin/zerg（薄壳开发用）
#   bash scripts/build/build-all.sh --only-core                # 只出主控 bin/zerg-core（**换件专用**）
#   bash scripts/build/build-all.sh --only-compat              # 只出兼容层 bin/zerg-compat（回滚前置校验用）
#   （--dist 以外的形态会另编茧壁 zerg-wall → bin/，见 scripts/build/build-wall.sh；它不进 dist 制品矩阵）
#    ★ **--dist 的边界**（2026-09-21 · G4「打包不得换件」）：本档只写 `$OUT`（= dist/<版本>），
#      bin/ 里那三件只落 bin/ 的件（`zerg-agentd` · `cocoon-docs-service` · `zerg-wall`）**一个都不产**，
#      签名环节也**不碰 bin/**。三条实据（为什么不是偏好）：
#      ① 其中两件（`bin/zerg-agentd` · `bin/cocoon-docs-service`）是**本机正在跑**的件（launchd 托管）
#         —— 就地重编 + 重签 = 一次「换件」（cdhash 变 ⇒ macOS TCC 本地网络授权失效）；
#      ② 发布路径不消费它们：release 布局里的 `zerg-wall-darwin-*` 由
#         `scripts/build/pack-release.sh` 自己 cargo 编（第 73–75 行），矩阵里没有 agentd /
#         cocoon-docs-service —— 它们是**本机部署件**，只由**不带 --dist** 的那一档负责；
#      ③ 与 `--only-cli / --only-core / --only-compat` 同一档口径：blast radius 越小越好。
#
# --only-cli 为什么需要单独一档（2026-09-20 · T-02）：命令面 `core/cmd/zerg` 是**独立客户端二进制**，
#   它的开发/自测不该顺手重编 `bin/zerg-core` —— 主控**正在跑**，就地覆盖它的制品文件等于埋一次
#   「换件」（改 cdhash ⇒ TCC 授权失效；重启即换实现）。本档只写 `bin/zerg` 一个文件，
#   其余制品与本脚本写的 `build-info.json`（升级器的身份依据）**一个字节都不动** ✓。
#   ★ 2026-09-22 · **G-05**：本档此前在**签名段之前**就 `exit 0` ⇒ 产出件只剩 Go linker 的 ad-hoc 签名
#     （`codesign -dv` 实读 `Identifier=a.out`）。那不是「报红」，是 macOS TCC 的**本地网络授权随
#     cdhash 静默失效**（漏一次就难查）。现补上重签，且**走与 `--only-core` / `--only-compat` 同一对
#     函数**（`resolve_sign_identity` + `sign_one`）—— 脚本下面那条注释逐字要求「同一条签名路径，
#     两份实现就会漂」，所以这里**不另写一份**签名代码。
#
# --only-core 为什么也需要单独一档（2026-09-20 · 批 B ①·换主控）：反过来同样成立 —— **换主控**时
#   不该顺手重编另外四件。三条实据：
#   ① `bin/zerg-agentd` = **本机子端正在跑的那件**（`ps` 实测 PPID=1 由 launchd 托管、监听 8100）⇒
#      就地覆盖在跑的制品文件等于顺手埋一次「子端换件」，而本批的纪律是「只重启主控这一个进程、
#      子端/UI 不动」；
#   ② 在 macOS 上就地覆盖**正在运行**的可执行文件可能把那个进程直接打崩（不是换实现，是掉服务）；
#   ③ 另外三件（`zerg-agent`/`zerg-ui`/`zerg-wall`）与本次换件无关，重编只增加变量。
#   ⇒ 本档只写 `bin/zerg-core` 一个文件；`build-info.json` **也不动** —— 它是**整批制品**的聚合描述，
#     只重编一件却改写它，等于让描述与现实不符；主控自己的身份来自 `-ldflags`（进程自报 code_sha）。
#
# 身份注入：
#   Go  → -ldflags -X .../internal/version.{Commit,BuildTime}（version.go 里是 var，可注入）
#   UI  → ui/build.rs 读 git/date 写 cargo:rustc-env（ZERG_GIT_SHA / ZERG_BUILD_TIME）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

DIST=0
BUILD_UI=1
SIGN=1
ONLY_CLI=0
ONLY_CORE=0
ONLY_COMPAT=0
for arg in "$@"; do
  case "$arg" in
    --dist) DIST=1 ;;
    --no-ui) BUILD_UI=0 ;;
    --public) PUBLIC=1 ;;   # 本地形态对齐公开快照（关私有默认 feature：示例虫茧等）
    --no-sign) SIGN=0 ;;
    --only-cli) ONLY_CLI=1 ;;
    --only-core) ONLY_CORE=1 ;;
    --only-compat) ONLY_COMPAT=1 ;;
    *) echo "未知参数: $arg" >&2; exit 64 ;;
  esac
done
# 三个「只出一件」档指向不同制品 ⇒ 同时给是用法错（不猜谁优先：猜错就编错东西）
if [ $((ONLY_CLI + ONLY_CORE + ONLY_COMPAT)) -gt 1 ]; then
  echo "✗ --only-cli / --only-core / --only-compat 互斥（各只写一件：bin/zerg · bin/zerg-core · bin/zerg-compat）⇒ 用法错" >&2
  exit 64
fi

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

# ── 重签（稳定身份 + 固定 identifier）───────────────────────────────────────────
# 为什么用**稳定签名身份**（2026-09-14）：ad-hoc 重签每次都会改变 cdhash ⇒ macOS 的 TCC「本地网络」
#   把每个新 cdhash 当新应用 ⇒ 授权列表越堆越多、且旧授权失效（现象：主控拨号报 no route to host）。
#   用稳定身份后，同一条授权终身有效。无证书的机器回退 ad-hoc（不阻断构建）。
# 为什么提成函数：`--only-core` 与末尾的批量重签必须走**同一条**签名路径 —— 两份实现就会漂
#   （漂的形态正是本仓最怕的：一件用稳定身份、另一件回落 ad-hoc，TCC 授权当场失效）。
resolve_sign_identity() {   # 只解析身份，不改文件；结果落 ZERG_SIGN_ID / SIGN_MODE
  [ "${SIGN:-1}" = "1" ] || return 0
  command -v codesign >/dev/null 2>&1 || return 0
  ZERG_SIGN_ID="${ZERG_SIGN_ID:-Zerg Local Signing}"
  if security find-identity -p codesigning 2>/dev/null | grep -q "$ZERG_SIGN_ID" \
     || codesign -s "$ZERG_SIGN_ID" --force /tmp/.zerg-sign-probe 2>/dev/null; then
    SIGN_MODE="stable:$ZERG_SIGN_ID"
  else
    ZERG_SIGN_ID="-"; SIGN_MODE="adhoc"
  fi
  if [ "$SIGN_MODE" = "adhoc" ]; then
    echo "   ⚠️  未找到签名身份「${ZERG_SIGN_ID}」，回退 ad-hoc（本机 TCC 授权会随重编失效）"
  fi
  return 0
}

sign_one() {  # sign_one <文件> —— 单件重签；identifier 按既有固定表
  local b="$1" ZID
  [ -f "$b" ] || return 0
  [ "${SIGN:-1}" = "1" ] || return 0
  command -v codesign >/dev/null 2>&1 || return 0
  case "$(basename "$b")" in
    zerg-core)   ZID=com.zerg.core ;;
    zerg-agent)  ZID=com.zerg.agent ;;
    zerg-agentd) ZID=com.zerg.agentd ;;
    zerg-ui)     ZID=com.zerg.ui ;;
    zerg-wall)   ZID=com.zerg.wall ;;
    *)           ZID="com.zerg.$(basename "$b")" ;;
  esac
  codesign -s "$ZERG_SIGN_ID" --identifier "$ZID" --force "$b" >/dev/null 2>&1 \
    && echo "   🔏 已重签名 $(basename "$b")（${ZID}，${SIGN_MODE}）"
  return 0
}

# 命令面 zerg（薄壳 · 独立客户端二进制）：与 9 个既有入口同 module、复用 core/internal/*（§6.1）。
# 放在最前，因为 --only-cli 只编它一件就收工。
echo "→ 命令面 zerg（薄壳 · core/cmd/zerg）"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg" ./cmd/zerg)

if [ "$ONLY_CLI" = "1" ]; then
  # 重签（2026-09-22 · G-05）：本档此前在签名段之前收工 ⇒ 产出件只剩 Go linker 的 ad-hoc 签名
  #   （`Identifier=a.out`）。这里**复用** `--only-core` / `--only-compat` 那同一对函数
  #   （`resolve_sign_identity` + `sign_one`，identifier 走既有固定表 ⇒ `zerg` → `com.zerg.zerg`），
  #   **不新增证书、不另写一份签名代码** —— 形态与那两档逐字同源。
  resolve_sign_identity
  sign_one "$OUT/zerg"
  echo "✅ 构建完成（--only-cli：只写 bin/zerg；其余制品与 build-info.json 一个字节未动）"
  printf "   %-14s %s 字节\n" "zerg" "$(stat -f%z "$OUT/zerg" 2>/dev/null || stat -c%s "$OUT/zerg")"
  exit 0
fi

# ── --only-core：换件专用档（只写 bin/zerg-core 一个文件）───────────────────────
#   纪律：换主控时**不许**顺手重编子端/UI（`bin/zerg-agentd` 是**正在跑**的子端件，覆盖它等于
#   埋一次子端换件，且 macOS 上就地覆盖在跑的可执行文件可能把那个进程打崩）。
#   构建身份（-ldflags）与重签（稳定身份 + `com.zerg.core`）两件事与本脚本其余档**逐字同源**。
if [ "$ONLY_CORE" = "1" ]; then
  echo "→ 主控 zerg-core（--only-core）"
  (cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-core" ./cmd/zerg-core)
  resolve_sign_identity
  sign_one "$OUT/zerg-core"
  echo "✅ 构建完成（--only-core：只写 bin/zerg-core；其余制品与 build-info.json 一个字节未动）"
  printf "   %-14s %s 字节\n" "zerg-core" "$(stat -f%z "$OUT/zerg-core" 2>/dev/null || stat -c%s "$OUT/zerg-core")"
  exit 0
fi

# ── --only-compat：只出兼容层一件（回滚前置校验用；批 C · T-29 / P-132）──────────────────
#   纪律同另两档：blast radius = 一个文件；其余制品与 build-info.json 一个字节不动。
if [ "$ONLY_COMPAT" = "1" ]; then
  echo "→ 兼容层 zerg-compat（--only-compat）"
  (cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-compat" ./cmd/zerg-compat)
  resolve_sign_identity
  sign_one "$OUT/zerg-compat"
  echo "✅ 构建完成（--only-compat：只写 bin/zerg-compat；其余制品与 build-info.json 一个字节未动）"
  printf "   %-14s %s 字节\n" "zerg-compat" "$(stat -f%z "$OUT/zerg-compat" 2>/dev/null || stat -c%s "$OUT/zerg-compat")"
  exit 0
fi

echo "→ 主控 zerg-core"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-core" ./cmd/zerg-core)

echo "→ 子端 zerg-agent"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-agent" ./cmd/zerg-agent)

# 兼容层 zerg-compat（批 C · T-29 落 P-132 的「要」）：回滚前置校验②（状态可读性）**只能靠它**执行
#   （`zerg-compat check`：0 就绪 · 1 迁移/校验失败 · 2 有待迁移文件 · **3 有更高版本 schema ⇒ 拒回滚** · 4 用法错）。
# 为什么进制品矩阵：P-132 逐字「要（否则 RB4② 无处可执行；且它已在 core/cmd/ 有源码与门禁）」
#   —— 它跟其余 zerg-* 一样落 `bin/`（dist 档落 `$OUT`），进签名名单与收尾清单（`$OUT/zerg-*` 通配命中）。
echo "→ 兼容层 zerg-compat"
(cd core && GOFLAGS=-mod=mod GOSUMDB=off go build $GOFLAGS_ -ldflags "$LDFLAGS" -o "$OUT/zerg-compat" ./cmd/zerg-compat)

# ── 只落 bin/ 的三件（zerg-agentd · cocoon-docs-service · zerg-wall）：--dist 一档**整块不跑** ─────
# 为什么：--dist 是**打包档**，产物只落 dist/<版本>/；这三件既不在 dist 制品矩阵、也不被发布路径
# 消费（实据见文件头「--dist 的边界」三条）⇒ 打包时重编 + 重签它们 = 白干 + 一次换件副作用。
if [ "$DIST" = "1" ]; then
  echo "→ 跳过 zerg-agentd / cocoon-docs-service / zerg-wall（--dist：本档只写 ${OUT}，不写 bin/、不重签 bin/）"
else

# 子端守护进程（agent 模块）：**与 CLI 区分命名**——带 d = daemon/常驻（ps 里一眼看出在跑服务还是任务）
# 注意：它暂不进 release 制品矩阵（矩阵 5 件是发布契约，要不要加是单独的决定），故产出到 bin/（--dist 档不产它 —— 见文件头「--dist 的边界」）。
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

# 茧壁 zerg-wall（批 2'.6）：**随卵分发**的沙箱外壳（一档：Linux 调 bwrap / macOS 直调 Seatbelt）。
# 为什么只落 bin/、**不进** dist 制品矩阵：矩阵 5 件是**发布契约**，要不要加茧壁是单独的决定（待拍板）
#   —— 与 cocoon-docs-service 同一条边界。
# 为什么跳过只在「本机无 cargo」这一种情形：`wall/` 在发布白名单里 ⇒ 源码不在就是**仓坏了**
#   （那由 build-wall.sh 硬失败接住，不许在这里被吞）；而缺 cargo 是**环境缺工具**，与 cocoon 块同一口径。
if command -v cargo >/dev/null 2>&1; then
  echo "→ 茧壁 zerg-wall（随卵分发；不进 dist 制品矩阵）"
  bash "$REPO_ROOT/scripts/build/build-wall.sh"
else
  echo "→ 跳过茧壁 zerg-wall（本机无 cargo）"
fi

fi   # ← 只落 bin/ 的三件（--dist 档不跑这一段：见文件头「--dist 的边界」）

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
  resolve_sign_identity
  # 签名路径与 `--only-core` 档**逐字同源**（同一对函数）—— 见上面 resolve_sign_identity/sign_one 的注释。
  # bin/ 里那三件只在**本机部署档**（不带 --dist）签名：--dist 一个字节都不碰 bin/（见文件头边界注释）。
  SIGN_LIST=("$OUT"/zerg "$OUT"/zerg-core "$OUT"/zerg-agent "$OUT"/zerg-compat "$OUT"/zerg-ui)
  if [ "$DIST" != "1" ]; then
    SIGN_LIST+=("$REPO_ROOT"/bin/zerg-agentd "$REPO_ROOT"/bin/cocoon-docs-service "${REPO_ROOT}"/bin/zerg-wall)
  fi
  for b in "${SIGN_LIST[@]}"; do
    sign_one "$b"
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
# 茧服务只在本机部署档落 bin/（不进 dist 制品矩阵）——单独列出，免得它没进上面的 zerg-* 清单被漏看。
# （--dist 档不产它：不加这一格守卫就会把**上次留下的旧件**当成本次产物报出来。）
if [ "$DIST" != "1" ] && [ -f "${REPO_ROOT}/bin/cocoon-docs-service" ]; then
  printf "   %-14s %s 字节（文档茧自带服务）\n" "cocoon-docs-service" \
    "$(stat -f%z "${REPO_ROOT}/bin/cocoon-docs-service" 2>/dev/null || stat -c%s "${REPO_ROOT}/bin/cocoon-docs-service")"
fi

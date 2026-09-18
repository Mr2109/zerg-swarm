#!/usr/bin/env bash
# build-wall.sh — 构建茧壁 `zerg-wall`（批 2'.6：制品与分发）
#
# 设计依据：`docs/01-设计/设计-茧壁-统一封闭契约与等级自证.md` §五之二
# 任务表：`docs/项目文档/v2.5.9/任务单-茧壁-20260916.md` §3（2'.6）
#
# 为什么单独一个入口（而不是把这段直接塞进 build-all.sh 里）：
#   ① 茧壁要**在目标机上编**：Linux 子端机器上也要有 `zerg-wall`（与 zerg-agent「该机自编」同一口径）
#      ⇒ 一条两侧都能跑的命令，而不是只能在本机跑的构建链的一部分；
#   ② `build-all.sh` 那条链会**覆盖 `bin/` 里正在被托管的生产制品**（主控 / 子端 / UI）—— 而茧壁在本批
#      是「只编不换」的东西 ⇒ 单跑本脚本**不碰任何生产制品**（这是纪律，不是偏好）。
#
# 边界（红线）：
#   · **只编不跑**：不 exec 任何卵、不碰运行态、不重启任何服务；
#   · **不交叉编译**：本机编本机（跨平台要目标机链接器；目标机自己编）—— 与 `cocoon-docs-service` 同一口径；
#   · **只落 `bin/`**：**不进** dist 制品矩阵 —— 「矩阵 5 件」是**发布契约**，要不要加茧壁是单独的决定（待拍板）；
#   · **认不得的参数直接拒**（rc=2）：静默忽略会让「跑错路线」看不出来；
#   · **本脚本永不跳过**：不成功一律 rc=2 —— 构建脚本若以「跳过」成功退出，「没编出茧壁」就会看起来像绿；
#     要不要跳过由**调用方**决定（见 `build-all.sh` 里那一处 `command -v cargo` 守卫）。
#
# 用法：
#   bash scripts/build-wall.sh            # release → bin/zerg-wall（本机平台）
#   bash scripts/build-wall.sh --debug    # debug   → wall/target/debug/zerg-wall（判据 7 的门禁脚本读它）
#
# 签名：落 `bin/` 的制品由 `scripts/build-all.sh` 的签名环节**统一**施加（稳定身份 + 固定 identifier）
#   —— 同一件事只有一处实现，本脚本不复制第二份（cargo 产出的件本身带 ad-hoc 签名，能跑）。
#
# 退出码：0 成功 · 2 硬失败（仓不完整 / 无 cargo / 编不出来 / 编完没有制品 / 制品起不来 / 用法错）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WALL_DIR="${REPO_ROOT}/wall"
PROFILE=release

for arg in "$@"; do
  case "${arg}" in
    --debug) PROFILE=debug ;;
    --help|-h)
      echo "用法: bash scripts/build-wall.sh [--debug]"
      echo "  release（默认）→ bin/zerg-wall（本机平台，不进 dist 制品矩阵）"
      echo "  --debug        → wall/target/debug/zerg-wall（判据 7 的门禁脚本读它）"
      exit 0 ;;
    *)
      echo "❌ 未知参数: ${arg}（本脚本只认 --debug / --help，不静默忽略）" >&2
      exit 2 ;;
  esac
done

# 仓不完整 = 硬失败：`wall/` 在发布白名单里（`publish/whitelist.txt`）⇒ 它不在就是**仓坏了**，不是「可以跳过」。
if [ ! -f "${WALL_DIR}/Cargo.toml" ]; then
  echo "❌ 茧壁源码不在 ${WALL_DIR}/Cargo.toml ⇒ 仓不完整（wall/ 是仓内一等公民，不走跳过这条路）" >&2
  exit 2
fi

if ! command -v cargo >/dev/null 2>&1; then
  echo "❌ 本机没有 cargo（rust 工具链）⇒ 编不出茧壁；构建段见 wall/README.md" >&2
  exit 2
fi

size_of() { stat -f%z "$1" 2>/dev/null || stat -c%s "$1"; }
sha_of() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1"; else sha256sum "$1"; fi | awk '{print $1}'
}

echo "→ 茧壁 zerg-wall（$(uname -s)/$(uname -m)，profile=${PROFILE}）"

# CARGO_INCREMENTAL=0：dev profile 默认开增量编译 ⇒ **同一份源码连编两次 sha 不同**（批 2'.5 实测
# `98bc845e…` / `c11b85ad…`；关掉后逐字节相同）⇒ 制品 sha 只有在可复现构建下才算证据，一律关。
if [ "${PROFILE}" = "release" ]; then
  (cd "${WALL_DIR}" && CARGO_INCREMENTAL=0 cargo build --release)
  ARTIFACT="${WALL_DIR}/target/release/zerg-wall"
  OUT="${REPO_ROOT}/bin/zerg-wall"
else
  (cd "${WALL_DIR}" && CARGO_INCREMENTAL=0 cargo build)
  ARTIFACT="${WALL_DIR}/target/debug/zerg-wall"
  OUT=""
fi

# 「编了」不等于「编出来了」：cargo 报成功之后必须**看见制品**。
if [ ! -x "${ARTIFACT}" ]; then
  echo "❌ cargo 说成功，但制品不在 ${ARTIFACT} —— 拒绝把「编了」当成「编出来了」" >&2
  exit 2
fi

# 「有制品」还不等于「制品能起来」：`--version` 是茧壁自己的最小自检（不 exec 任何卵、不碰运行态）。
VERSION_OUT="$("${ARTIFACT}" --version)" || {
  echo "❌ 制品无法执行（rc!=0）：${ARTIFACT}" >&2
  exit 2
}

if [ -n "${OUT}" ]; then
  mkdir -p "$(dirname "${OUT}")"
  cp -p "${ARTIFACT}" "${OUT}"
  if [ ! -x "${OUT}" ]; then
    echo "❌ 拷贝后 ${OUT} 不可执行" >&2
    exit 2
  fi
  echo "   ✅ ${OUT#"${REPO_ROOT}"/}  ${VERSION_OUT}  $(size_of "${OUT}") 字节  sha256 $(sha_of "${OUT}")"
  echo "   ⓘ 签名：稳定身份签名由 build-all.sh 的签名环节统一施加（本脚本只编，不复制第二份签名逻辑）"
else
  echo "   ✅ ${ARTIFACT#"${REPO_ROOT}"/}  ${VERSION_OUT}  $(size_of "${ARTIFACT}") 字节  sha256 $(sha_of "${ARTIFACT}")"
fi

#!/usr/bin/env bash
# negctl-build-wall.sh — `scripts/build-wall.sh` 的**负例活体控制**（批 2'.6）
#
# 为什么要有它：构建脚本最容易坏的形态**不是编不出来**，而是**跳过却看起来像绿**
# （「没编出茧壁」和「编好了」在 CI 眼里长得一样）。所以本脚本要证的正是那几条**必须红**的路：
#
#   C0 控制组（必须先绿）：真树 `--debug` ⇒ rc=0 且 `wall/target/debug/zerg-wall` 可执行
#      —— 没有这一条，下面的「红」可能只是**一切都红**（无区分度）。
#   N1 仓不完整（临时树无 `wall/`）        ⇒ rc=2，且**不许**产出 `bin/zerg-wall`
#   N2 本机无 cargo（PATH 里没有 cargo）   ⇒ rc=2（**不许**以「跳过」成功退出）
#   N3 认不得的参数                        ⇒ rc=2（静默忽略会让「跑错路线」看不出来）
#
# 临时树 = 把脚本拷进 `/private/tmp/<名>/scripts/` 再跑 ⇒ **真仓的 `bin/` 与 `wall/` 一字不动**。
# 判据**只看 rc**（按真实退出码分支，不按输出里的字样）。回执即本脚本的 stdout。
#
# 用法：bash scripts/negctl-build-wall.sh      # 退出码 0 = 四条全按期望；1 = 有控制项不符；2 = 环境/用法问题
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${REPO_ROOT}/scripts/build-wall.sh"

[ -f "${SCRIPT}" ] || { echo "❌ 被测脚本不在: ${SCRIPT}" >&2; exit 2; }
[ -x "${REPO_ROOT}/wall/target/debug/zerg-wall" ] || {
  echo "❌ 前置件不在: wall/target/debug/zerg-wall（先 bash scripts/build-wall.sh --debug）" >&2; exit 2; }

TMP="$(mktemp -d /private/tmp/zerg-negctl-build-wall.XXXXXX)"
trap 'rm -rf "${TMP}"' EXIT

pass=0
fail=0
check() { # <名称> <期望 rc> <实际 rc> [附加断言 0/1 <说明>]  —— 不传附加断言 = 「无附加断言」，视为满足
  local name="$1" want="$2" got="$3" extra="${4:-1}" extra_msg="${5:-}"
  if [ "${got}" = "${want}" ] && [ "${extra}" = "1" ]; then
    echo "  PASS ${name}（rc=${got}）"
    pass=$((pass + 1))
  else
    echo "  ✗ 红 ${name}：期望 rc=${want} 实际 rc=${got}${extra_msg:+ · ${extra_msg}}"
    fail=$((fail + 1))
  fi
}

run_rc() { # <工作树跟> <PATH 覆盖 或 -> [参数...]
  local root="$1" path="$2"
  shift 2
  local rc=0
  if [ "${path}" = "-" ]; then
    ( cd "${root}" && bash "${root}/scripts/build-wall.sh" "$@" ) >/dev/null 2>&1 || rc=$?
  else
    ( cd "${root}" && PATH="${path}" bash "${root}/scripts/build-wall.sh" "$@" ) >/dev/null 2>&1 || rc=$?
  fi
  echo "${rc}"
}

echo "=== C0 控制组：真树 --debug 必须先绿（否则下面是「一切皆红」，无区分度）==="
rc="$(run_rc "${REPO_ROOT}" - --debug)"
extra=0
[ -x "${REPO_ROOT}/wall/target/debug/zerg-wall" ] && extra=1
check "C0 真树 --debug" 0 "${rc}" "${extra}" "产物必须可执行"

echo
echo "=== N1 仓不完整（临时树无 wall/）：必须 rc=2，且不许产出 bin/zerg-wall ==="
mkdir -p "${TMP}/n1/scripts"
cp "${SCRIPT}" "${TMP}/n1/scripts/build-wall.sh"
rc="$(run_rc "${TMP}/n1" - )"
extra=0
[ ! -e "${TMP}/n1/bin/zerg-wall" ] && extra=1
check "N1 无 wall/ 源码" 2 "${rc}" "${extra}" "不许悄悄产出 bin/zerg-wall"

echo
echo "=== N2 本机无 cargo（PATH 去掉 cargo）：必须 rc=2（不许以「跳过」成功退出）==="
mkdir -p "${TMP}/n2/scripts" "${TMP}/n2/wall"
cp "${SCRIPT}" "${TMP}/n2/scripts/build-wall.sh"
printf '# 占位：只为过「Cargo.toml 存在」这一关，构建本身不该走到\n' > "${TMP}/n2/wall/Cargo.toml"
# 用最小 PATH（不含 ~/.cargo/bin；cargo 装在那）⇒ 走「无 cargo」那条分支
rc="$(run_rc "${TMP}/n2" "/usr/bin:/bin")"
check "N2 无 cargo" 2 "${rc}"

echo
echo "=== N3 认不得的参数：必须 rc=2 ==="
rc="$(run_rc "${REPO_ROOT}" - --wall-wall)"
check "N3 陌生参数" 2 "${rc}"

echo
echo "=== 汇总：PASS ${pass} / 红 ${fail} ==="
if [ "${fail}" -ne 0 ]; then
  echo "!! 有控制项不符（上面逐条列了）—— 负例控制本身失败" >&2
  exit 1
fi
echo "✅ 负例活体控制四条全按期望（C0 绿 · N1~N3 各自真红在 rc=2）"

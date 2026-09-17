#!/usr/bin/env bash
# accept-no-probe-selftest.sh — `accept-no-probe.sh` 的自证（任务表 T1.5）
#
# 为什么必须有它：验收脚本本类事故的头号形态是**假绿** —— 一条写坏的 grep 谁都不命中，
# 于是「生产件干净」永远成立。所以这把尺子必须先量两个已知答案的自造件：
#   · prod 件（无 tag、无哨兵）        ⇒ 必须 ACCEPT，rc=0
#   · debug 件（-tags=debug + 哨兵）   ⇒ 必须 REJECT，rc=1（**负控必须红**）
# 两个 rc 都按真实退出码断言（不看输出里的字样），任一条不符 ⇒ 本脚本 rc=2（自检失败 ⇒ 拒绝用它量真件）。
# 另有牙齿检查：debug 侧必须报出「负控检出条数: 3/3」，即三条判据逐条检出探针 ——
# 否则「非零退出」可能来自别的原因，负控等于没有区分度。
#
# 自造件（临时目录里现写现编，真仓的 core/ agent/ 一字不动；两份都用 `-ldflags "-s -w"` 编，
# 以证明三条判据在 strip 之后依然有效 —— 旧判据正是在这一步失效的）：
#   main.go        普通入口，调 heavyProbeVals()
#   probe_off.go   //go:build !debug —— 生产侧的空实现
#   heavy_debug.go //go:build debug  —— 调试专属文件（文件名即判据③，哨兵字面量即判据②）
#
# 用法: bash scripts/accept-no-probe-selftest.sh
# 退出码: 0 自证通过 / 2 自检失败或环境问题（含 go 不可用、构建失败 —— 一律如实报，不伪造结论）
#
# 纪律: 无 sudo、不联网（GOPROXY=off，只用标准库）；临时目录 mktemp -d + `trap cleanup EXIT` 清理；
#       构建时显式 `GOFLAGS=`（GOFLAGS 是隐形污染源：一个环境变量就能把生产构建变成调试构建）并 GOTOOLCHAIN=local；
#       不启用 `set -o pipefail`（grep 无匹配返回 1 会误杀流水线）。
set -u

SENTINEL='ZERG_HEAVY_PROBE_V1'
DEBUGBASENAME='heavy_debug.go'
SEP='--------------------------------------------------------------------------------'

CLEANUP_DIR=''
cleanup() {
  if [ -n "${CLEANUP_DIR}" ] && [ -d "${CLEANUP_DIR}" ]; then
    rm -rf "${CLEANUP_DIR}"
  fi
}
trap cleanup EXIT

SELF_DIR=$(cd "$(dirname "$0")" && pwd)
TARGET="${SELF_DIR}/accept-no-probe.sh"

if [ ! -f "${TARGET}" ]; then
  echo "!! [blocker] 被测脚本不在: ${TARGET}" >&2
  exit 2
fi
if ! bash -n "${TARGET}" 2>&1; then
  echo '!! [blocker] 被测脚本语法不过（bash -n 红）' >&2
  exit 2
fi

# 漂移闸：自检件里的哨兵/文件名必须与被测脚本里的常量是**同一份字面量**（从被测脚本里抽出来比），
# 否则改了被测脚本而没改自检 ⇒ 自检会「通过」得毫无意义。
TARGET_SENTINEL=$(sed -n "s/^SENTINEL='\(.*\)'$/\1/p" "${TARGET}" | head -n 1)
TARGET_DEBUGFILE=$(sed -n "s/^DEBUGBASENAME='\(.*\)'$/\1/p" "${TARGET}" | head -n 1)
if [ "${TARGET_SENTINEL}" != "${SENTINEL}" ]; then
  echo "!! [blocker] 漂移：自检的 SENTINEL=${SENTINEL} 与被测脚本的 SENTINEL=${TARGET_SENTINEL} 不一致" >&2
  exit 2
fi
if [ "${TARGET_DEBUGFILE}" != "${DEBUGBASENAME}" ]; then
  echo "!! [blocker] 漂移：自检的 DEBUGBASENAME=${DEBUGBASENAME} 与被测脚本的 DEBUGBASENAME=${TARGET_DEBUGFILE} 不一致" >&2
  exit 2
fi

GO_BIN=$(command -v go 2>/dev/null || true)
if [ -z "${GO_BIN}" ]; then
  echo '!! [blocker] 本机 PATH 里没有 go —— 造不出自检件，自证无法进行（不伪造结论）。' >&2
  exit 2
fi

TMPBASE=${TMPDIR:-/tmp}
CLEANUP_DIR=$(mktemp -d "${TMPBASE%/}/accept-no-probe-selftest.XXXXXX") || {
  echo '!! [blocker] mktemp -d 失败（临时目录不可用）' >&2
  exit 2
}
MODDIR="${CLEANUP_DIR}/probe-min"
mkdir -p "${MODDIR}" || {
  echo "!! [blocker] 建不出自检模块目录: ${MODDIR}" >&2
  exit 2
}

cat >"${MODDIR}/go.mod" <<'GOMOD'
module probeMin

go 1.21
GOMOD

cat >"${MODDIR}/main.go" <<'GOMAIN'
package main

import "fmt"

func main() {
	_ = heavyProbeVals()
	fmt.Println("probe-min up")
}
GOMAIN

cat >"${MODDIR}/probe_off.go" <<'GOOFF'
//go:build !debug

package main

func heavyProbeVals() string { return "" }
GOOFF

# 调试专属文件：文件名是判据③，哨兵字面量是判据② —— 只有 -tags=debug 才编得进来
cat >"${MODDIR}/${DEBUGBASENAME}" <<'GODBG'
//go:build debug

package main

import "fmt"

const heavySentinel = "ZERG_HEAVY_PROBE_V1"

func heavyProbeVals() string {
	fmt.Println("heavy probe active: " + heavySentinel)
	return heavySentinel
}
GODBG

echo "${SEP}"
printf '自检临时树: %s\n' "${MODDIR}"
printf '被测脚本  : %s\n' "${TARGET}"
printf '判据常量  : SENTINEL=%s  DEBUGBASENAME=%s\n' "${SENTINEL}" "${DEBUGBASENAME}"
printf 'go        : %s\n' "$("${GO_BIN}" version)"

# 两个变体都带 -ldflags "-s -w"：判据必须在 strip 之后仍然成立（旧判据正是在这里失效的）
build_variant() { # $1=输出路径 其余 = go build 额外旗标
  _out=$1
  shift
  (
    cd "${MODDIR}" || exit 2
    GOFLAGS= GO111MODULE=on GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
      "${GO_BIN}" build "$@" -ldflags '-s -w' -o "${_out}" .
  )
}

PROD_BIN="${CLEANUP_DIR}/probe-min-prod"
DBG_BIN="${CLEANUP_DIR}/probe-min-debug"

if ! build_variant "${PROD_BIN}"; then
  echo '!! [blocker] 生产变体构建失败（如实报，不伪造结论）' >&2
  exit 2
fi
if ! build_variant "${DBG_BIN}" -tags=debug; then
  echo '!! [blocker] debug 变体构建失败（如实报，不伪造结论）' >&2
  exit 2
fi
printf '自检件    : prod=%s  debug=%s\n' "${PROD_BIN}" "${DBG_BIN}"
echo "${SEP}"

CASES_TOTAL=0
CASES_BAD=0
LAST_LOG=''
LAST_RC=''

# run_case <标签> <期望 rc> <件> [给被测脚本的参数...]
run_case() {
  _label=$1
  _exp=$2
  _bin=$3
  shift 3
  CASES_TOTAL=$((CASES_TOTAL + 1))
  LAST_LOG="${CLEANUP_DIR}/case-${CASES_TOTAL}.log"
  bash "${TARGET}" "${_bin}" "$@" >"${LAST_LOG}" 2>&1
  _got=$?
  LAST_RC=${_got}
  _verdict='OK ✓'
  if [ "${_got}" -ne "${_exp}" ]; then
    _verdict='MISMATCH ✗'
    CASES_BAD=$((CASES_BAD + 1))
  fi
  printf 'rc: %s  =  %s   期望 %s   %s\n' "${_label}" "${_got}" "${_exp}" "${_verdict}"
}

echo '【负控与正控（rc 按真实退出码断言，不看输出里的字样）】'
run_case 'prod 件（显式第二参数 prod）' 0 "${PROD_BIN}" prod
LOG_PROD="${LAST_LOG}"
RC_PROD="${LAST_RC}"
run_case 'prod 件（缺省第二参数）' 0 "${PROD_BIN}"
LOG_PROD_DEFAULT="${LAST_LOG}"
run_case 'debug 件（第二参数 debug = 负控）' 1 "${DBG_BIN}" debug
LOG_DBG="${LAST_LOG}"
RC_DBG="${LAST_RC}"
run_case 'debug 件（当 prod 报 = 真红灯路径）' 1 "${DBG_BIN}" prod
LOG_DBG_AS_PROD="${LAST_LOG}"

# 负控必须有牙齿：debug 侧不能是「随便什么原因非零退出」，而要三条判据逐条检出
if [ "${CASES_BAD}" -eq 0 ]; then
  if ! LC_ALL=C grep -q -F '负控检出条数: 3/3' "${LOG_DBG}"; then
    echo '!! 负控失效：debug 侧没有报出「负控检出条数: 3/3」—— 判据没有逐条检出探针'
    CASES_BAD=$((CASES_BAD + 1))
  fi
  if ! LC_ALL=C grep -q -F 'FAIL ✗' "${LOG_DBG}"; then
    echo '!! 负控失效：debug 侧没有任何 FAIL 判据行'
    CASES_BAD=$((CASES_BAD + 1))
  fi
  if ! LC_ALL=C grep -q -F 'PASS ✓' "${LOG_PROD}"; then
    echo '!! 正控失效：prod 侧没有任何 PASS 判据行'
    CASES_BAD=$((CASES_BAD + 1))
  fi
fi

echo "${SEP}"
echo '【两行 rc（本次要交的证据；取自 run_case 捕获的真实退出码）】'
printf 'prod  侧 rc = %s （期望 0）\n' "${RC_PROD}"
printf 'debug 侧 rc = %s （期望 1）\n' "${RC_DBG}"
echo '（上一段的四行 rc 表才是断言本身，这里的两行是其中主控/负控两条的退出码）'
echo "${SEP}"
echo '【prod 侧完整输出（显式 prod）】'
cat "${LOG_PROD}"
echo "${SEP}"
echo '【prod 侧完整输出（缺省参数，应与上一致）】'
cat "${LOG_PROD_DEFAULT}"
echo "${SEP}"
echo '【debug 侧完整输出（负控：三条判据必须全 FAIL 且 rc=1）】'
cat "${LOG_DBG}"
echo "${SEP}"
echo '【debug 件当 prod 报的完整输出（真红灯路径：必须 REJECT）】'
cat "${LOG_DBG_AS_PROD}"
echo "${SEP}"

if [ "${CASES_BAD}" -eq 0 ]; then
  echo "自检结论: 通过 —— 断言 ${CASES_TOTAL} 条全对（prod 侧 rc=0，debug 侧 rc=1 = 负控红）"
  exit 0
fi
echo "自检结论: 失败 —— 断言 ${CASES_TOTAL} 条里有 ${CASES_BAD} 条不符（这把尺子不可用，别拿它量真件）"
exit 2

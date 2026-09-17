#!/usr/bin/env bash
# redaction-probes.sh — 脱敏探针一键跑（任务表 T2.3 探针+基线对照 / T2.6 fuzz 门禁）
#
# 干三件事，缺一不可：
#   ① 跑 core/internal/obs/redact 的**全部**用例（15 条负例探针 + 基线对照 + fail-closed + 回扫门禁）
#   ② 跑一遍短时 fuzz（性质：不 panic / 幂等 f(x)==f(f(x)) / 落盘行合法 UTF-8 / 无原文投影 / 门禁同源）
#   ③ 打印关键数字（探针数、三条基线的泄漏数、fuzz 执行次数），并把全过程落到 /tmp 的报告文件
#
# 用法:
#   scripts/redaction-probes.sh [fuzz时间，缺省 10s；0 = 跳过 fuzz]
# 退出码: 0 全绿 / 1 有用例或 fuzz 红 / 2 环境错误（缺 go / 找不到包）
#
# 纪律: 只读仓库（fuzz 语料写进 GOCACHE，不写仓库）；不联网取依赖（本包零第三方依赖）；
#       不启用 `set -o pipefail`（grep 无匹配返回 1，会误杀整条流水线，与仓库既有脚本同口径）。
#       变量一律写 ${VAR}：本脚本的提示语是中文/全角标点，`$VAR（` 会被 bash 当成变量名的一部分
#       （实测踩过：line 45 报 unbound variable）。
set -u

FUZZTIME="${1:-10s}"
PKG='./internal/obs/redact/'

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${HERE}/.." && pwd)"
CORE="${ROOT}/core"

STAMP="$(date +%Y%m%d-%H%M%S)"
LOG="/tmp/zerg-redaction-probes-${STAMP}.log"
FUZZ_LOG="/tmp/zerg-redaction-probes-fuzz-${STAMP}.log"
REPORT="/tmp/zerg-redaction-probes-report.txt"

if ! command -v go >/dev/null 2>&1; then
  echo "环境错误：找不到 go" >&2
  exit 2
fi
if [ ! -d "${CORE}/internal/obs/redact" ]; then
  echo "环境错误：找不到包目录 ${CORE}/internal/obs/redact" >&2
  exit 2
fi

# 与仓库门禁同口径的构建环境（可用外部环境变量覆盖）
export GOFLAGS="${GOFLAGS:--mod=mod}"
export GOSUMDB="${GOSUMDB:-off}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

cd "${CORE}" || exit 2

echo "== ① 负例探针 + 基线对照（${PKG}）=="
go test "${PKG}" -count=1 -v >"${LOG}" 2>&1
RC_TEST=$?

# 只抓关键行：探针数、三条基线的泄漏数、失败用例名
grep -E 'baseline\[|probes=|三分表|^--- FAIL|^\s+--- FAIL' "${LOG}"
PROBES="$(grep -o 'probes=[0-9]*' "${LOG}" | head -1 | cut -d= -f2)"
BASE_KEY="$(grep -o 'baseline\[key-denylist-only\] leaks [0-9]*/[0-9]*' "${LOG}" | head -1 | awk '{print $3}')"
BASE_ASCII="$(grep -o 'baseline\[ascii-regex-top-level-only\] leaks [0-9]*/[0-9]*' "${LOG}" | head -1 | awk '{print $3}')"
BASE_OURS="$(grep -o 'baseline\[ours([^)]*)\] leaks [0-9]*/[0-9]*' "${LOG}" | head -1 | awk '{print $NF}')"
FAILS="$(grep -c '^--- FAIL\|^    --- FAIL' "${LOG}")"

if [ "${RC_TEST}" -ne 0 ]; then
  echo "✗ 用例红：rc=${RC_TEST}，完整输出见 ${LOG}"
  echo "== 小结 =="
  echo "探针数            : ${PROBES:-?}"
  echo "键黑名单基线漏    : ${BASE_KEY:-?}   （设计稿：14/15）"
  echo "仅顶层正则基线漏  : ${BASE_ASCII:-?}   （设计稿：10/15）"
  echo "本方案漏          : ${BASE_OURS:-?}   （设计稿：0/15）"
  echo "报告：未生成（用例红时不写结论）"
  exit 1
fi

FUZZ_EXECS='（跳过）'
RC_FUZZ=0
if [ "${FUZZTIME}" != "0" ]; then
  echo "== ② fuzz（-fuzztime ${FUZZTIME}）=="
  go test "${PKG}" -run '^$' -fuzz FuzzRedactValue -fuzztime "${FUZZTIME}" >"${FUZZ_LOG}" 2>&1
  RC_FUZZ=$?
  grep -E 'elapsed: .*execs' "${FUZZ_LOG}" | tail -1
  FUZZ_EXECS="$(grep -o 'execs: [0-9]*' "${FUZZ_LOG}" | tail -1 | awk '{print $2}')"
  if [ "${RC_FUZZ}" -ne 0 ]; then
    echo "✗ fuzz 红：rc=${RC_FUZZ}，完整输出见 ${FUZZ_LOG}"
    grep -E 'FAIL|泄漏|不幂等|panic|--- ' "${FUZZ_LOG}" | head -20
    exit 1
  fi
  echo "fuzz: PASS，execs=${FUZZ_EXECS:-?}"
else
  echo "== ② fuzz：按参数跳过 =="
fi

{
  echo "Zerg 脱敏探针报告 — ${STAMP}"
  echo "仓库        : ${ROOT}"
  echo "git rev     : $(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo '?')"
  echo "包          : ${PKG}"
  echo "构建环境    : GOFLAGS=${GOFLAGS} GOSUMDB=${GOSUMDB} GOPROXY=${GOPROXY}"
  echo
  echo "① 用例（探针 + 基线对照 + fail-closed + 回扫门禁）: PASS，rc=0，失败用例 ${FAILS} 个"
  echo "   探针数            : ${PROBES:-?}（设计稿 15）"
  echo "   键黑名单基线漏    : ${BASE_KEY:-?}（设计稿 14/15 —— 探针有效的证据：基线必须泄漏）"
  echo "   仅顶层正则基线漏  : ${BASE_ASCII:-?}（设计稿 10/15）"
  echo "   本方案漏          : ${BASE_OURS:-?}（设计稿 0/15）"
  echo
  echo "② fuzz FuzzRedactValue（-fuzztime ${FUZZTIME}）: rc=${RC_FUZZ}，execs=${FUZZ_EXECS:-（跳过）}"
  echo "   性质：不 panic · 幂等 f(x)==f(f(x)) · 落盘行合法 UTF-8 · 无原文投影 · 门禁与脱敏器同源"
  echo
  echo "原始日志    : ${LOG}"
  echo "fuzz 日志   : ${FUZZ_LOG}"
} >"${REPORT}"

echo "== 小结 =="
echo "探针数            : ${PROBES:-?}"
echo "键黑名单基线漏    : ${BASE_KEY:-?}   （设计稿：14/15）"
echo "仅顶层正则基线漏  : ${BASE_ASCII:-?}   （设计稿：10/15）"
echo "本方案漏          : ${BASE_OURS:-?}   （设计稿：0/15）"
echo "fuzz 执行次数     : ${FUZZ_EXECS:-（跳过）}"
echo "报告文件          : ${REPORT}"
echo "原始日志          : ${LOG}"

if [ "${BASE_OURS}" != "0/15" ] || [ "${RC_FUZZ}" -ne 0 ]; then
  echo "✗ 数字与设计稿不符（本方案必须 0/15）" >&2
  exit 1
fi
echo "✓ 全绿：探针 0 泄漏 · 基线照旧泄漏（探针有效）· fuzz 无失败"
exit 0

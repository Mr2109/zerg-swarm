#!/usr/bin/env bash
# accept-no-probe.sh — 「生产二进制里没有重探针」的可判定验收
# （任务表 T1.5；替代已废弃的「strings 查不到探针符号」判据）
#
# 为什么废弃旧判据（已实测，勿推翻）：Go 加 `-ldflags "-s -w"` 之后，strings **仍能**命中函数名与源文件路径
# （pclntab 保留，实测 probe.Vals 仍命中 1 次）⇒ 「生产件 strings 查不到探针符号」这条判据**不成立**，
# 它会让**真红看着像绿**。本脚本改用三条「只有调试件才有」的判据（不看 mtime、不看体积 —— 两个 tag
# 配置出的二进制体积可以完全相同）：
#   ① BuildInfo 的 -tags 里不含 debug  —— 用 `go version -m <bin>` 读（Go 1.18+ 自动记录 -tags）
#   ② 唯一哨兵字面量 SENTINEL 命中 0 次 —— 该字面量只写在调试专属文件里，生产件里根本不存在
#   ③ 调试专属文件名 DEBUGBASENAME 命中 0 次
#
# 负控（本类验收的头号假绿来源）：同一脚本在 `-tags=debug` 产的件上**必须**判 FAIL 并非零退出 ——
# 否则一条写坏的 grep 会永远通过。声明 debug 时脚本会打印「负控检出条数 N/3」并要求 N=3：
# 一条谁都不命中的 grep 会在这里漏检 ⇒ 负控不完整，本脚本如实报出来（rc 见下）。
#
# 用法:
#   scripts/accept-no-probe.sh <二进制路径> [prod|debug]
#     prod  (缺省) = 生产件：三条判据必须全 PASS ⇒ rc 0（接受）；有任一 FAIL ⇒ rc 1（拒绝）
#     debug        = 负控  ：三条判据必须全 FAIL（探针被逐条检出）⇒ rc 1（拒绝 —— 负控红正是要的结果）
#                            三条全 PASS（一条都没检出）⇒ rc 2（负控失效：判据本身可疑，或该件其实是生产件）
#   scripts/accept-no-probe.sh --selftest      # 转交 scripts/accept-no-probe-selftest.sh
# 退出码: 0 接受 / 1 拒绝 / 2 用法或环境错误（含负控失效、go 不可用、非 Go 二进制）
#
# 纪律: 无 sudo、不联网、只读被检件；临时目录 mktemp -d + `trap cleanup EXIT` 清理。
#       不启用 `set -o pipefail`（grep 无匹配返回 1，会误杀整条流水线）—— 一律按显式 rc 处理，见 count_hits()。
set -u

SENTINEL='ZERG_HEAVY_PROBE_V1'
DEBUGBASENAME='heavy_debug.go'
DEBUGTAG='debug'

CLEANUP_DIR=''
cleanup() {
  if [ -n "${CLEANUP_DIR}" ] && [ -d "${CLEANUP_DIR}" ]; then
    rm -rf "${CLEANUP_DIR}"
  fi
}
trap cleanup EXIT

usage() {
  cat <<'USAGE'
用法: accept-no-probe.sh <二进制路径> [prod|debug]
      accept-no-probe.sh --selftest
  prod  (缺省) = 生产件：三条判据必须全 PASS ⇒ rc 0；有任一 FAIL ⇒ rc 1（拒绝）
  debug        = 负控  ：三条判据必须全 FAIL ⇒ rc 1（负控红）；全 PASS ⇒ rc 2（负控失效）
退出码: 0 接受 / 1 拒绝 / 2 用法或环境错误
USAGE
}

# 命中计数：$1=文件 $2=模式 $3=fixed|ere（缺省 fixed）
# 输出非负整数；文件读不了等异常输出 ERR —— 绝不把「判不了」吞成 0（那会变成假绿）
count_hits() {
  _f=$1
  _p=$2
  _mode=${3:-fixed}
  if [ "${_mode}" = 'ere' ]; then
    _out=$(LC_ALL=C grep -c -E -e "${_p}" "${_f}" 2>/dev/null)
  else
    _out=$(LC_ALL=C grep -c -F -e "${_p}" "${_f}" 2>/dev/null)
  fi
  _rc=$?
  case "${_rc}" in
    0) printf '%s' "${_out}" ;;
    1) printf '0' ;;
    *) printf 'ERR' ;;
  esac
}

mark() {
  if [ "$1" -eq 1 ]; then
    printf 'PASS ✓'
  else
    printf 'FAIL ✗'
  fi
}

if [ "$#" -ge 1 ] && [ "$1" = '--selftest' ]; then
  HERE=$(cd "$(dirname "$0")" && pwd)
  SELFTEST="${HERE}/accept-no-probe-selftest.sh"
  if [ ! -f "${SELFTEST}" ]; then
    echo "!! 找不到自检脚本: ${SELFTEST}" >&2
    exit 2
  fi
  exec bash "${SELFTEST}"
fi

if [ "$#" -lt 1 ]; then
  usage
  exit 2
fi

BIN=$1
KIND='prod'
if [ "$#" -ge 2 ]; then
  KIND=$2
fi
case "${KIND}" in
  prod|debug) ;;
  *)
    echo "!! 第二参数只接受 prod|debug，收到: ${KIND}" >&2
    usage
    exit 2
    ;;
esac

if [ ! -e "${BIN}" ]; then
  echo "!! 被检件不存在: ${BIN}" >&2
  exit 2
fi
if [ ! -f "${BIN}" ]; then
  echo "!! 被检件不是普通文件: ${BIN}" >&2
  exit 2
fi
if [ ! -r "${BIN}" ]; then
  echo "!! 被检件不可读: ${BIN}" >&2
  exit 2
fi

TMPBASE=${TMPDIR:-/tmp}
CLEANUP_DIR=$(mktemp -d "${TMPBASE%/}/accept-no-probe.XXXXXX") || {
  echo '!! mktemp -d 失败（临时目录不可用）' >&2
  exit 2
}

# ---------- 判据①：BuildInfo 的 -tags ----------
GO_BIN=$(command -v go 2>/dev/null || true)
if [ -z "${GO_BIN}" ]; then
  echo '!! [blocker] 本机 PATH 里没有 go —— 判据①（BuildInfo 的 -tags）无法判定；本脚本绝不把「判不了」当 PASS。' >&2
  exit 2
fi

BUILDINFO="${CLEANUP_DIR}/buildinfo.txt"
BUILDINFO_ERR="${CLEANUP_DIR}/buildinfo.err"
if ! "${GO_BIN}" version -m "${BIN}" >"${BUILDINFO}" 2>"${BUILDINFO_ERR}"; then
  echo "!! [blocker] \`go version -m\` 读不出 ${BIN} 的 BuildInfo（非 Go 二进制或已损坏）；判据①无法判定。" >&2
  sed -n '1,3p' "${BUILDINFO_ERR}" >&2
  exit 2
fi

BUILD_ROWS=$(count_hits "${BUILDINFO}" '^[[:space:]]*build[[:space:]]' ere)
if [ "${BUILD_ROWS}" = 'ERR' ] || [ "${BUILD_ROWS}" -lt 1 ]; then
  echo '!! [blocker] 该件的 BuildInfo 里没有任何 build 行（Go 1.12 以前的件不记录构建设置）；判据①无法判定。' >&2
  exit 2
fi

TAGVAL=$(sed -n 's/^[[:space:]]*build[[:space:]]-tags=//p' "${BUILDINFO}" | head -n 1)
C1_PASS=0
case ",${TAGVAL}," in
  *",${DEBUGTAG},"*) C1_PASS=0 ;;
  *) C1_PASS=1 ;;
esac
if [ -z "${TAGVAL}" ]; then
  TAG_SHOWN='(无 -tags 记录)'
else
  TAG_SHOWN=${TAGVAL}
fi

# ---------- 判据②③：哨兵字面量 / 调试专属文件名 ----------
STRINGS_DUMP="${CLEANUP_DIR}/strings.txt"
if ! LC_ALL=C strings -a "${BIN}" >"${STRINGS_DUMP}" 2>/dev/null; then
  : >"${STRINGS_DUMP}"
  LC_ALL=C strings "${BIN}" >"${STRINGS_DUMP}" 2>/dev/null || {
    echo "!! [blocker] strings 对本机 / 该件不可用；判据②③无法判定。" >&2
    exit 2
  }
fi
if [ ! -s "${STRINGS_DUMP}" ]; then
  echo '!! [blocker] strings 未产出任何可打印串（件为空或不可读）；判据②③无法判定。' >&2
  exit 2
fi

SENTINEL_HITS=$(count_hits "${STRINGS_DUMP}" "${SENTINEL}" fixed)
FILE_HITS=$(count_hits "${STRINGS_DUMP}" "${DEBUGBASENAME}" fixed)
if [ "${SENTINEL_HITS}" = 'ERR' ] || [ "${FILE_HITS}" = 'ERR' ]; then
  echo '!! [blocker] 哨兵/文件名计数失败（strings 产物读不了）；判据②③无法判定。' >&2
  exit 2
fi

C2_PASS=0
if [ "${SENTINEL_HITS}" -eq 0 ]; then C2_PASS=1; fi
C3_PASS=0
if [ "${FILE_HITS}" -eq 0 ]; then C3_PASS=1; fi

NPASS=$((C1_PASS + C2_PASS + C3_PASS))
NDETECT=$((3 - NPASS))

# ---------- 报表 ----------
SHA=$(LC_ALL=C shasum -a 256 "${BIN}" 2>/dev/null | awk '{print $1}')
SIZE=$(wc -c <"${BIN}" | tr -d ' ')
if [ "${KIND}" = 'prod' ]; then
  EXPECT_TEXT='全 PASS（生产件里没有探针）'
else
  EXPECT_TEXT='全 FAIL（负控：探针必须被逐条检出）'
fi

echo '════════════════════════════════════════════════════════════'
echo "验收对象 : ${BIN}"
echo "sha256   : ${SHA}"
echo "体积     : ${SIZE} 字节（体积与 mtime 均不作为判据，只作留痕）"
echo "声明类型 : ${KIND}   期望: 三条判据 ${EXPECT_TEXT}"
echo "判据常量 : SENTINEL=${SENTINEL}  DEBUGBASENAME=${DEBUGBASENAME}"
echo '────────────────────────────────────────────────────────────'
printf '[判据①] BuildInfo 的 -tags 不含 debug\n'
printf '        预期=-tags 不含 %s    实际=-tags=%s    判定=%s\n' "${DEBUGTAG}" "${TAG_SHOWN}" "$(mark "${C1_PASS}")"
printf '[判据②] 唯一哨兵字面量命中 0 次\n'
printf '        预期=0               实际=%s               判定=%s\n' "${SENTINEL_HITS}" "$(mark "${C2_PASS}")"
printf '[判据③] 调试专属文件名命中 0 次\n'
printf '        预期=0               实际=%s               判定=%s\n' "${FILE_HITS}" "$(mark "${C3_PASS}")"
echo '────────────────────────────────────────────────────────────'
echo '说明: 已废弃判据 `strings 查不到探针符号` —— Go 加 -s -w 后 pclntab 仍在，'
echo '      strings 依旧能命中函数名与源文件路径（实测仍命中）⇒ 那条件不可能成立，本脚本不用它。'

RC=0
if [ "${KIND}" = 'prod' ]; then
  if [ "${NPASS}" -eq 3 ]; then
    echo "结论: ACCEPT —— 三条判据 ${NPASS}/3 PASS：该件是生产件，未带重探针"
    RC=0
  else
    FAILED_TEXT=''
    if [ "${C1_PASS}" -eq 0 ]; then FAILED_TEXT="${FAILED_TEXT} ①(-tags 含 ${DEBUGTAG})"; fi
    if [ "${C2_PASS}" -eq 0 ]; then FAILED_TEXT="${FAILED_TEXT} ②(哨兵命中 ${SENTINEL_HITS})"; fi
    if [ "${C3_PASS}" -eq 0 ]; then FAILED_TEXT="${FAILED_TEXT} ③(文件名命中 ${FILE_HITS})"; fi
    echo "结论: REJECT —— 三条判据 ${NPASS}/3 PASS，检出重探针；未过:${FAILED_TEXT}"
    RC=1
  fi
else
  echo "负控检出条数: ${NDETECT}/3"
  if [ "${NPASS}" -eq 0 ]; then
    echo "结论: REJECT（rc=1 = 负控红，正是期望结果）—— debug 件确实带探针，三条判据逐条检出"
    RC=1
  elif [ "${NPASS}" -eq 3 ]; then
    echo "结论: 负控失效（rc=2）—— 声明 debug 却三条全 PASS：判据可能已失效（写坏的 grep 会藏在这里），或该件其实是生产件"
    RC=2
  else
    echo "⚠ 负控不完整：${NDETECT}/3 检出，有判据没检出探针（写坏的 grep 会藏在这里）"
    echo "结论: REJECT（rc=1 = 负控红），但控制不完整 —— 该查那几条判据本身"
    RC=1
  fi
fi
echo "rc = ${RC}"
exit "${RC}"

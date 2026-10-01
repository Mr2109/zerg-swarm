#!/usr/bin/env bash
#zerg-class: write D2
# writer-lock.sh —— 写者锁（mkdir 原子锁 · 缺口 GAP-20260926-94 施工清单第 3 件）。
#
# 为什么有这件：写 scripts/gates 这一面此前没有写者互斥 —— 两个写者窗口重叠时会整件互盖（本夜真出过一次）。
# 本机没有 flock(1)（现读 command -v flock 空 · /usr/bin/flock 不存在）⇒ 只能走 mkdir 目录锁
# （设计稿现测：mkdir 新目录 rc=0 · 同名二次 rc=1 · rmdir 后 rc=0；盘 APFS /dev/disk3s5）。
#
# ★ 口径（设计稿硬结论）：本锁**不防覆盖**，只当旁路探测器 —— 单纯加锁拦不住「持锁者自己写坏」
#   或「人用编辑器 / cp 整件覆盖」。所以：
#   ★ **绝不自动破锁**：锁过期也不自动接管；只报告「锁存在 / 持有者是谁 / 已多久」。
#   ★ 判不了 ⇒ 不给结论（退 2），绝不当抢得到。
#
# 用法
# ----
#   bash scripts/dev/writer-lock.sh acquire [--dir <锁目录>] [--who <持有者>]
#   bash scripts/dev/writer-lock.sh release [--dir <锁目录>] [--who <持有者>]
#   bash scripts/dev/writer-lock.sh status  [--dir <锁目录>]
#   bash scripts/dev/writer-lock.sh --help
#   bash scripts/dev/writer-lock.sh --self-test
#
#   --dir  锁落点（默认 scripts/gates/.writer-lock —— 与 atomic-write.sh 的留档落点 scripts/gates/_history
#          同族（同一写面 scripts/gates）；相对路径按仓根解析，绝对路径原样用）
#   --who  持有者名（默认取环境 ZERG_WRITER，再退回 hostname）
#
# 三档退码（照拄 scripts/dev/atomic-write.sh 现读体例 · 不新立命名空间 · 禁自创码）
#   0 = 成功（取锁得手 / 放锁得手 / 查锁读到读数）
#       依据：atomic-write.sh:24「0 = 换件成功」· scripts/dev/isolate-ro.sh:34「0 正常」
#   1 = 确实的坏形态（放锁时你不是持有者 ⇒ 拒；锁落点被一个非目录占着 ⇒ 拒）
#       依据：atomic-write.sh:25「1 = 不成功（拒写：…）」· isolate-ro.sh:34「1 危险形态（…）」
#   2 = 不给结论（取锁时锁已存在（含陈旧）⇒ 拒 · 用法错 / 缺实参 / 未知旗标 / 自检不过 · owner 读不到判不了）
#       依据：atomic-write.sh:27「2 = 不给结论（用法错/缺件/自检不过/…）」· isolate-ro.sh:34「2 不给结论」
#
# 本件守则：不联网 · 不起后台进程 · 不 sudo · 绝不自动破锁 · 只碰 --dir 指定的锁落点。
set -uo pipefail

SUB=""
DIR_OPT=""
WHO_OPT=""
SELFTEST=0

_self="${BASH_SOURCE[0]:-$0}"
case "${_self}" in
  */*) _dir="$(cd "$(dirname "${_self}")" && pwd)" ;;
  *)   _dir="$(pwd)" ;;
esac
SELF_PATH="${_dir}/$(basename "${_self}")"
REPO_ROOT="$(cd "${_dir}/../.." && pwd)"

LOCK_REL_DEFAULT="scripts/gates/.writer-lock"

warn() { printf '%s\n' "$*" >&2; }
die()  { local rc="$1"; shift; warn "$*"; exit "${rc}"; }

usage() {
  cat <<'USAGE'
用法
  bash scripts/dev/writer-lock.sh acquire [--dir <锁目录>] [--who <持有者>]
  bash scripts/dev/writer-lock.sh release [--dir <锁目录>] [--who <持有者>]
  bash scripts/dev/writer-lock.sh status  [--dir <锁目录>]
  bash scripts/dev/writer-lock.sh --help
  bash scripts/dev/writer-lock.sh --self-test
说明
  acquire  取锁（mkdir 原子建锁目录）；锁已存在 ⇒ 拒、逐字点名持有者与时刻（不给结论）
  release  放锁（先删 owner 件、再 rmdir 锁目录）；你不是持有者 ⇒ 拒（绝不代放/绝不自动破锁）
  status   查锁（只读）：报锁在不在、持有者是谁、已持有多久
  --dir    锁落点（默认 scripts/gates/.writer-lock；相对路径按仓根解析，绝对路径原样用）
  --who    持有者名（默认取环境 ZERG_WRITER，再退回 hostname）
退码
  0 成功 · 1 确实的坏形态（非持有者放锁 / 锁落点被非目录占着）· 2 不给结论（锁已存在 / 用法错 / 未知旗标 / 自检不过）
USAGE
}

resolve_dir() {
  local d="$1"
  case "${d}" in
    /*) LOCK_DIR="${d}" ;;
    *)  LOCK_DIR="${REPO_ROOT}/${d}" ;;
  esac
  OWNER_FILE="${LOCK_DIR}/owner"
  LOCK_DISPLAY="${LOCK_DIR}"
}

owner_field() {  # $1 = 键名
  [ -f "${OWNER_FILE}" ] || return 1
  awk -F= -v k="$1" '$1==k { sub(/^[^=]*=/,""); print; exit }' "${OWNER_FILE}" 2>/dev/null
}

write_owner() {
  local now_e now_s
  now_e="$(date +%s)"
  now_s="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  {
    printf 'who=%s\n'    "${WHO}"
    printf 'pid=%s\n'    "$$"
    printf 'host=%s\n'   "$(hostname 2>/dev/null || echo unknown)"
    printf 'since=%s\n'  "${now_s}"
    printf 'since_epoch=%s\n' "${now_e}"
    printf 'dir=%s\n'    "${LOCK_DISPLAY}"
  } > "${OWNER_FILE}" 2>/dev/null || return 1
  return 0
}

show_owner() {
  local w p s e h age
  if [ ! -d "${LOCK_DIR}" ]; then
    printf '   锁状态：无锁（%s 不存在）\n' "${LOCK_DISPLAY}"
    return 0
  fi
  if [ ! -f "${OWNER_FILE}" ]; then
    printf '   锁状态：锁存在（%s）· 持有者不明（owner 件读不到）· 时刻不明\n' "${LOCK_DISPLAY}"
    return 0
  fi
  w="$(owner_field who)"; p="$(owner_field pid)"; s="$(owner_field since)"
  e="$(owner_field since_epoch)"; h="$(owner_field host)"
  age=""
  if [ -n "${e}" ]; then age="$(( $(date +%s) - e ))s"; fi
  printf '   锁状态：锁存在（%s）\n' "${LOCK_DISPLAY}"
  printf '   持有者：%s（pid %s · 主机 %s）\n' "${w:-不明}" "${p:-不明}" "${h:-不明}"
  printf '   持有时刻：%s · 已持有：%s\n' "${s:-不明}" "${age:-不明}"
}

cmd_acquire() {
  local mk_rc
  if [ -e "${LOCK_DIR}" ] && [ ! -d "${LOCK_DIR}" ]; then
    warn "   ✗ 取锁被拒：锁落点被一个非目录占着（${LOCK_DISPLAY}）⇒ 确实的坏形态"
    exit 1
  fi
  mkdir -p "$(dirname "${LOCK_DIR}")" 2>/dev/null || true
  # ★ mkdir 是原子原语：它的 rc 就是「锁在不在」的现读。rc 单独收，好让负控只动「判」、不动「建」。
  mkdir "${LOCK_DIR}" 2>/dev/null; mk_rc=$?
  if [ "${mk_rc}" -ne 0 ]; then
    warn "   ✗ 取锁被拒：锁已存在 ⇒ 不给结论（不许静默抢 · 本锁绝不自动破锁）"
    show_owner
    exit 2
  fi
  if ! write_owner; then
    rmdir "${LOCK_DIR}" 2>/dev/null || true
    die 2 "   ✗ 取锁失败：owner 件写不进去 ⇒ 已回退锁目录、不给结论"
  fi
  printf '✅ 取锁成功：%s（持有者 %s）\n' "${LOCK_DISPLAY}" "${WHO}"
  show_owner
  exit 0
}

cmd_release() {
  local ow
  if [ ! -d "${LOCK_DIR}" ]; then
    warn "   ✗ 放锁被拒：没有锁可放（${LOCK_DISPLAY} 不存在）⇒ 你不是持有者"
    exit 1
  fi
  if [ ! -f "${OWNER_FILE}" ]; then
    die 2 "   ✗ 放锁被拒：锁存在但 owner 件读不到 ⇒ 判不了你是不是持有者、不给结论（锁未动）"
  fi
  ow="$(owner_field who)"
  if [ "${ow}" != "${WHO}" ]; then
    warn "   ✗ 放锁被拒：你不是持有者（本锁持有者=${ow:-不明} · 你=${WHO}）"
    warn "     ⇒ 不许代放（本锁绝不自动破锁 · 锁未动）"
    exit 1
  fi
  if ! { rm -f "${OWNER_FILE}" && rmdir "${LOCK_DIR}"; }; then
    die 1 "   ✗ 放锁失败：锁目录清不掉（${LOCK_DISPLAY}）⇒ 锁未干净释放、确实的坏形态"
  fi
  printf '✅ 放锁成功：%s（持有者 %s 已释放）\n' "${LOCK_DISPLAY}" "${WHO}"
  exit 0
}

cmd_status() { show_owner; exit 0; }

selftest() {
  local tmp fake WL rc
  pass=0; fail=0
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/wl-selftest.XXXXXX")" || { warn "自检临时目录建不起来"; exit 2; }
  fake="${tmp}/repo"
  mkdir -p "${fake}/scripts/gates" "${fake}/scripts/dev" "${fake}/bin"
  cp "${SELF_PATH}" "${fake}/scripts/dev/writer-lock.sh"
  WL="${fake}/scripts/dev/writer-lock.sh"

  printf '  —— writer-lock.sh --self-test（合成夹具 · 逐格独立锁目录 · 全在 %s 下）——\n' "${tmp}"

  # ① 空 ⇒ 取锁成功
  bash "${WL}" acquire --dir scripts/gates/case1/.writer-lock --who wA >"${tmp}/o1" 2>&1; rc=$?
  chk "① 空目录取锁 ⇒ rc=0"     "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "① 锁目录已建出"          "[ -d \"${fake}/scripts/gates/case1/.writer-lock\" ]" ""
  chk "① owner 记下持有者 wA"   "grep -q '^who=wA$' \"${fake}/scripts/gates/case1/.writer-lock/owner\"" ""

  # ② 已持有 ⇒ 再取 ⇒ 拒 + 逐字点名持有者与时刻（独立夹具 ⇒ 负控只此处翻红）
  bash "${WL}" acquire --dir scripts/gates/case2/.writer-lock --who wA >/dev/null 2>&1
  bash "${WL}" acquire --dir scripts/gates/case2/.writer-lock --who wB >"${tmp}/o2" 2>&1; rc=$?
  chk "② 已持有再取 ⇒ rc=2"     "[ ${rc} -eq 2 ]" "rc=${rc}"
  chk "② 逐字点名持有者 wA"     "grep -q '持有者：wA' \"${tmp}/o2\"" ""
  chk "② 逐字报持有时刻"        "grep -q '持有时刻：' \"${tmp}/o2\"" ""

  # ③ 释放后 ⇒ 可再取（独立夹具）
  bash "${WL}" acquire --dir scripts/gates/case3/.writer-lock --who wA >/dev/null 2>&1
  bash "${WL}" release --dir scripts/gates/case3/.writer-lock --who wA >"${tmp}/o3a" 2>&1; rc=$?
  chk "③ 持有者放锁 ⇒ rc=0"     "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "③ 放锁后锁目录已清"      "[ ! -d \"${fake}/scripts/gates/case3/.writer-lock\" ]" ""
  bash "${WL}" acquire --dir scripts/gates/case3/.writer-lock --who wC >"${tmp}/o3b" 2>&1; rc=$?
  chk "③ 释放后再取锁 ⇒ rc=0"   "[ ${rc} -eq 0 ]" "rc=${rc}"

  # ④ 非持有者释放 ⇒ 拒（独立夹具）
  bash "${WL}" acquire --dir scripts/gates/case4/.writer-lock --who wA >/dev/null 2>&1
  bash "${WL}" release --dir scripts/gates/case4/.writer-lock --who wZ >"${tmp}/o4" 2>&1; rc=$?
  chk "④ 非持有者放锁 ⇒ rc=1（拒）" "[ ${rc} -eq 1 ]" "rc=${rc}"
  chk "④ 逐字点名你不是持有者"  "grep -q '你不是持有者' \"${tmp}/o4\"" ""
  chk "④ 锁未被放掉（仍在）"    "[ -d \"${fake}/scripts/gates/case4/.writer-lock\" ]" ""

  # ⑤ 未知旗标 ⇒ rc=2 + 逐字点名
  bash "${WL}" --bogus >"${tmp}/o5" 2>&1; rc=$?
  chk "⑤ 未知旗标 ⇒ rc=2"       "[ ${rc} -eq 2 ]" "rc=${rc}"
  chk "⑤ 逐字点名未知旗标 --bogus" "grep -q '未知旗标 --bogus' \"${tmp}/o5\"" ""

  # ⑥ --help ⇒ rc=0（零写盘）
  bash "${WL}" --help >"${tmp}/o6" 2>&1; rc=$?
  chk "⑥ --help ⇒ rc=0"         "[ ${rc} -eq 0 ]" "rc=${rc}"

  # ⑦ 查锁只读 ⇒ rc=0 且报出持有者、不动锁（不自动破锁）
  bash "${WL}" status --dir scripts/gates/case4/.writer-lock >"${tmp}/o7" 2>&1; rc=$?
  chk "⑦ 查锁 ⇒ rc=0"           "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "⑦ 查锁报出持有者 wA（只读）" "grep -q '持有者：wA' \"${tmp}/o7\"" ""

  # ⑧ 锁落点被一个非目录占着 ⇒ 拒（确实的坏形态）
  mkdir -p "${fake}/scripts/gates/case5"
  printf 'x\n' > "${fake}/scripts/gates/case5/.writer-lock"
  bash "${WL}" acquire --dir scripts/gates/case5/.writer-lock --who wA >"${tmp}/o8" 2>&1; rc=$?
  chk "⑧ 锁落点被非目录占着 ⇒ rc=1（拒）" "[ ${rc} -eq 1 ]" "rc=${rc}"

  printf '  —— 自检：%d 格过 · %d 格红（夹具 %s）——\n' "${pass}" "${fail}" "${tmp}"
  if [ "${fail}" -eq 0 ]; then rm -rf "${tmp}"; exit 0; else warn "自检有红 ⇒ 不给结论"; exit 2; fi
}

chk() {  # $1 描述 · $2 断言 · $3 读数
  if eval "$2"; then printf '    ✅ %s\n' "$1"; pass=$((pass + 1))
  else printf '    ❌ %s  [%s]\n' "$1" "${3:-}"; fail=$((fail + 1)); fi
}

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)   usage; exit 0 ;;
    --self-test) SELFTEST=1; shift ;;
    --dir)       [ $# -ge 2 ] || die 2 "用法错：--dir 缺实参（见 --help）"; DIR_OPT="$2"; shift 2 ;;
    --who)       [ $# -ge 2 ] || die 2 "用法错：--who 缺实参（见 --help）"; WHO_OPT="$2"; shift 2 ;;
    acquire|release|status) SUB="$1"; shift ;;
    -*)          die 2 "用法错：未知旗标 $1（见 --help）" ;;
    *)           die 2 "用法错：多余的位置参数 $1（见 --help）" ;;
  esac
done

if [ "${SELFTEST}" -eq 1 ]; then selftest; fi
[ -n "${SUB}" ] || die 2 "用法错：缺子命令（acquire|release|status · 见 --help）"

WHO="${WHO_OPT:-${ZERG_WRITER:-$(hostname 2>/dev/null || echo unknown)}}"
resolve_dir "${DIR_OPT:-${LOCK_REL_DEFAULT}}"

case "${SUB}" in
  acquire) cmd_acquire ;;
  release) cmd_release ;;
  status)  cmd_status ;;
esac

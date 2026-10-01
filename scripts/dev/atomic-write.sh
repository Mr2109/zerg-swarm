#!/usr/bin/env bash
#zerg-class: write D2
# atomic-write.sh —— 换件正门：原子写 + 历史留档 + 写前预校（缺口 GAP-20260926-94）。
#
# 为什么有这件：改门脚本这件上此前没有任何保护，两个写者会整件互盖（本夜真出过一次）。
# 本件把 build-all.sh:206-244 的「覆盖前 sha256 留档 -> 回读校验 -> 失败即拒换件 -> 只留不删」
# 四步抽成一只对**任意件**可用的工具，并把落点白名单口径照抄 build-all.sh:198-204。
#
# 用法
# ----
#   bash scripts/dev/atomic-write.sh --target <仓内相对路径> --from <仓外新件绝对路径>
#   bash scripts/dev/atomic-write.sh --help
#   bash scripts/dev/atomic-write.sh --self-test
#
# 步骤（逐条照拄 build-all.sh:206-244）
#   1 算旧件 sha256
#   2 落档到 scripts/gates/_history/<件名>.bak-<YYYYMMDD-HHMMSS>（同名同 sha 已存在 => 跳过不重写）
#   3 回读：重读留档件、比 sha => 不一致即失败
#   4 任一步失败 => 拒写，目标件一字节不动，非零退出 + 逐字点名
#   5 写前预校：真正写入前再读一次目标件比 sha；不一致（有人中途改过）=> 拒写
#   6 全绿才写（先写临时件再 mv），写完回读对拍
# 留档只留不删：本件函数体里没有 rm/mv/truncate 留档的口。
#
# 退码（照拄在册同族 · core/internal/contract/exitcode-namespaces.json；本件不新立命名空间）
#   0 = 换件成功
#   1 = 不成功（拒写：落点越界 / 留档失败 / 留档回读不符 / 写后对拍不符）—— 目标件未动
#     照拄 build-all.sh 的 exit 1 与 zerg-swap-core.sh「1 = 不成功（未动生产支）」
#   2 = 不给结论（用法错 / 缺件 / 自检不过 / 写前预校发现目标件被中途改动）
#     照拄 zerg-swap-core.sh「2 = 用法错」· scripts/dev/isolate-ro.sh「2 = 不给结论」· gen-glossary-exports.py:226
#
# 本件守则：不联网 · 不起后台进程 · 不 sudo · 不删任何留档。
set -uo pipefail

SELFTEST=0
TARGET_REL=""
FROM_ABS=""

_self="${BASH_SOURCE[0]:-$0}"
case "${_self}" in
  */*) _dir="$(cd "$(dirname "${_self}")" && pwd)" ;;
  *)   _dir="$(pwd)" ;;
esac
SELF_PATH="${_dir}/$(basename "${_self}")"
REPO_ROOT="$(cd "${_dir}/../.." && pwd)"

# 自检缝合点：默认值即正门口径。给自检用，好让「落点越界」这一格能真跑出来（不增任何旗标）。
HIST_REL="${ATOMIC_WRITE_DROP_REL:-scripts/gates/_history}"
HIST_WHITELIST="scripts/gates/_history"

warn() { printf '%s\n' "$*" >&2; }
die()  { local rc="$1"; shift; warn "$*"; exit "${rc}"; }

usage() {
  cat <<'USAGE'
用法
  bash scripts/dev/atomic-write.sh --target <仓内相对路径> --from <仓外新件绝对路径>
  bash scripts/dev/atomic-write.sh --help
  bash scripts/dev/atomic-write.sh --self-test
说明
  --target  要换的件（仓内相对路径，例 scripts/gates/precommit-gates.sh）
  --from    新内容（仓外新件的绝对路径）
  步骤：算旧件 sha256 -> 落 scripts/gates/_history/<件名>.bak-<时间戳> -> 回读校验
        -> 失败即拒写（目标件一个字节不动）-> 写前预校 -> 临时件落目标 -> 写后回读对拍
退码
  0 换件成功 · 1 拒写（越界/留档失败/回读不符/写后对拍不符）· 2 不给结论（用法错/缺件/自检不过/预校发现中途被改）
USAGE
}

sha_of() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else return 127; fi
}
bytes_of() { stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null; }

# 落点白名单（照抄 build-all.sh:198-204 history_drop_ok 口径）：$1 = 仓内相对落点
drop_ok() {
  case "$1" in
    "${HIST_WHITELIST}"/*) return 0 ;;
  esac
  warn "   ✗ 留档落点 ${1} 不在 ${HIST_WHITELIST}/ 之下 ⇒ 判红、拒写（目标件一个字节不动）"
  return 1
}

# 照拄 build-all.sh:206-244 archive_prev_piece：$1 = 将被就地覆盖的件（绝对路径）
archive_prev() {
  local b="$1" base hist stamp src new sha hit
  [ -f "$b" ] || return 0
  base="$(basename "$b")"
  hist="${REPO_ROOT}/${HIST_REL}"
  if ! mkdir -p "${hist}"; then
    warn "   ✗ 留档目录建不起来：${hist} ⇒ 拒写"
    return 1
  fi
  src="$(sha_of "$b")"
  for hit in "${hist}/${base}".bak-*; do
    [ -f "$hit" ] || continue
    if [ "$(sha_of "$hit")" = "${src}" ]; then
      printf '   🗄  留档跳过：%s 与已有留档 %s 逐字节相同（sha256 %s）—— 不重复留、不覆盖\n' \
        "${base}" "$(basename "$hit")" "${src}"
      return 0
    fi
  done
  stamp="$(date +%Y%m%d-%H%M%S)"
  new="${hist}/${base}.bak-${stamp}"
  if [ -e "${new}" ]; then new="${new}.$$"; fi
  if ! drop_ok "${new#"${REPO_ROOT}/"}"; then return 1; fi
  if ! cp -p "$b" "$new"; then
    warn "   ✗ 留档失败（cp -p ${b} → ${new}）⇒ 拒写"
    return 1
  fi
  # 自检缝合点：模拟「cp 出来的留档件是坏的」，好让回读校验这一格能分真假（默认不触发）
  if [ "${ATOMIC_WRITE_NEG:-}" = "corrupt_archive" ]; then : > "${new}"; fi
  # 复制后回读 sha256：与旧件逐字节相同才算留档成功（不拿 cp 的退出码当唯一判据）
  sha="$(sha_of "$new")"
  if [ "${sha}" != "${src}" ]; then
    warn "   ✗ 留档回读 sha 与旧件不同（${sha} ≠ ${src}）：$(basename "$new") ⇒ 拒写"
    return 1
  fi
  printf '   🗄  已留档旧件：%s/%s（%s 字节 · sha256 %s）\n' \
    "${HIST_REL}" "$(basename "$new")" "$(bytes_of "$new")" "${src}"
  return 0
}

selftest() {
  local tmp fake outside AW rc n arch old_sha pass fail base
  pass=0; fail=0
  base="${TMPDIR:-/tmp}"
  tmp="$(mktemp -d "${base}/aw-selftest.XXXXXX")" || { warn "自检临时目录建不起来"; exit 2; }
  fake="${tmp}/repo"; outside="${tmp}/outside"
  mkdir -p "${fake}/scripts/gates" "${fake}/scripts/dev" "${fake}/bin" "${outside}"
  cp "${SELF_PATH}" "${fake}/scripts/dev/atomic-write.sh"
  AW="${fake}/scripts/dev/atomic-write.sh"
  G="${fake}/scripts/gates/demo.sh"
  printf 'OLD v1\n' > "${G}"
  old_sha="$(sha_of "${G}")"
  printf 'NEW v2\n' > "${outside}/new2.sh"

  printf '  —— atomic-write.sh --self-test（合成夹具 · 全在 %s 下）——\n' "${tmp}"

  # ① 正常换件
  bash "${AW}" --target scripts/gates/demo.sh --from "${outside}/new2.sh" >"${tmp}/o1" 2>&1; rc=$?
  chk "① 正常换件 ⇒ rc=0" "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "① 目标件 == 新件内容" "[ \"$(cat "${G}")\" = 'NEW v2' ]" ""
  n="$(ls "${fake}/${HIST_REL}"/demo.sh.bak-* 2>/dev/null | wc -l | tr -d ' ')"
  chk "① _history 命中 1 枚" "[ ${n} -eq 1 ]" "命中 ${n}"
  arch="$(ls "${fake}/${HIST_REL}"/demo.sh.bak-* 2>/dev/null | head -1)"
  chk "② 留档件 sha == 旧件现算 sha（两串相同）" \
      "[ -n \"${arch}\" ] && [ \"$(sha_of "${arch}")\" = '${old_sha}' ]" "留档=$(sha_of "${arch}" 2>/dev/null) 旧件=${old_sha}"

  # ①b 同名同 sha 已存在 ⇒ 跳过不重写
  printf 'OLD v1\n' > "${G}"
  printf 'NEW v3\n' > "${outside}/new3.sh"
  bash "${AW}" --target scripts/gates/demo.sh --from "${outside}/new3.sh" >"${tmp}/o1b" 2>&1; rc=$?
  n="$(ls "${fake}/${HIST_REL}"/demo.sh.bak-* 2>/dev/null | wc -l | tr -d ' ')"
  chk "①b 同名同 sha 已存在 ⇒ 跳过不重写（仍 1 枚）" "[ ${rc} -eq 0 ] && [ ${n} -eq 1 ]" "rc=${rc} 命中 ${n}"

  # ③ 中途被改（能分真假）：预校发现目标件被改 ⇒ rc=2 且目标件未被写入
  printf 'OLD v1\n' > "${G}"
  printf 'NEW vX\n' > "${outside}/x.sh"
  ATOMIC_WRITE_NEG=mutate_target bash "${AW}" --target scripts/gates/demo.sh --from "${outside}/x.sh" >"${tmp}/o3" 2>&1; rc=$?
  chk "③ 中途被改 ⇒ rc=2（非零）" "[ ${rc} -eq 2 ]" "rc=${rc}"
  chk "③ 目标件未被写入新内容（未动）" \
      "[ \"$(cat "${G}")\" != 'NEW vX' ] && [ \"$(cat "${G}")\" = \"$(printf 'OLD v1\n\n# intruder')\" ]" "现值=$(cat "${G}")"
  chk "③ 回执里有 cmp 证明行" "grep -q 'cmp' \"${tmp}/o3\"" ""

  # ④ 落点越界 ⇒ 报错码正确（照拄白名单口径）
  printf 'OLD v1\n' > "${G}"
  ATOMIC_WRITE_DROP_REL=bin bash "${AW}" --target scripts/gates/demo.sh --from "${outside}/new2.sh" >"${tmp}/o4" 2>&1; rc=$?
  chk "④ 落点越界 ⇒ rc=1" "[ ${rc} -eq 1 ]" "rc=${rc}"
  chk "④ 落点越界 ⇒ 逐字点名（不在 …/_history/ 之下）" "grep -q '不在' \"${tmp}/o4\"" ""

  # ⑤ 未知旗标 ⇒ 用法错码 + 逐字
  bash "${AW}" --bogus >"${tmp}/o5" 2>&1; rc=$?
  chk "⑤ 未知旗标 ⇒ rc=2" "[ ${rc} -eq 2 ]" "rc=${rc}"
  chk "⑤ 未知旗标 ⇒ 逐字点名" "grep -q '未知旗标 --bogus' \"${tmp}/o5\"" ""

  # ⑥ 回读校验负控（能分真假）：留档件被写坏 ⇒ 回读不符 ⇒ rc=1、拒写
  #    用「此前没被留档过的内容」，否则会走「同名同 sha ⇒ 跳过」那一支、根本到不了 cp+回读。
  printf 'OLD v9 unique\n' > "${G}"
  ATOMIC_WRITE_NEG=corrupt_archive bash "${AW}" --target scripts/gates/demo.sh --from "${outside}/new2.sh" >"${tmp}/o6" 2>&1; rc=$?
  chk "⑥ 留档回读不符 ⇒ rc=1（拒写）" "[ ${rc} -eq 1 ]" "rc=${rc}"
  chk "⑥ 回读不符 ⇒ 目标件未被覆盖" "[ \"$(cat "${G}")\" = 'OLD v9 unique' ]" "现值=$(cat "${G}")"
  chk "⑥ 回读不符 ⇒ 逐字点名" "grep -q '留档回读 sha 与旧件不同' \"${tmp}/o6\"" ""

  printf '  —— 自检：%d 格过 · %d 格红（夹具 %s）——\n' "${pass}" "${fail}" "${tmp}"
  if [ "${fail}" -eq 0 ]; then rm -rf "${tmp}"; exit 0; else warn "自检有红 ⇒ 不给结论"; exit 2; fi
}

chk() {  # $1 描述 · $2 断言 · $3 读数
  if eval "$2"; then printf '    ✅ %s\n' "$1"; pass=$((pass + 1))
  else printf '    ❌ %s  [%s]\n' "$1" "${3:-}"; fail=$((fail + 1)); fi
}

# ── 参数面（照拄 gen-glossary-exports.py:221-227 与 isolate-ro.sh:380-396）─────────────
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)   usage; exit 0 ;;
    --self-test) SELFTEST=1; shift ;;
    --target)    [ $# -ge 2 ] || die 2 "用法错：--target 缺实参（见 --help）"; TARGET_REL="$2"; shift 2 ;;
    --from)      [ $# -ge 2 ] || die 2 "用法错：--from 缺实参（见 --help）";   FROM_ABS="$2"; shift 2 ;;
    -*)          die 2 "用法错：未知旗标 $1（见 --help）" ;;
    *)           die 2 "用法错：多余的位置参数 $1（见 --help）" ;;
  esac
done

if [ "${SELFTEST}" -eq 1 ]; then selftest; fi
[ -n "${TARGET_REL}" ] || die 2 "用法错：缺 --target（见 --help）"
[ -n "${FROM_ABS}" ]   || die 2 "用法错：缺 --from（见 --help）"
case "${FROM_ABS}" in /*) ;; *) die 2 "用法错：--from 要绝对路径（见 --help）" ;; esac
[ -f "${FROM_ABS}" ] || die 2 "用法错：--from 件不存在：${FROM_ABS}"

TARGET_ABS="${REPO_ROOT}/${TARGET_REL}"
[ -d "$(dirname "${TARGET_ABS}")" ] || die 2 "用法错：--target 目录不存在（见 --help）"

OLD_SHA=""
if [ -f "${TARGET_ABS}" ]; then
  OLD_SHA="$(sha_of "${TARGET_ABS}")"
  archive_prev "${TARGET_ABS}" || die 1 "留档失败/落点越界 ⇒ 拒写（${TARGET_REL} 一个字节未动）"

  # 自检缝合点：模拟「别人在本件开工后中途改了目标件」（默认不触发）
  if [ "${ATOMIC_WRITE_NEG:-}" = "mutate_target" ]; then printf '\n# intruder\n' >> "${TARGET_ABS}"; fi

  # ★ 写前预校：真正写入前再读一次目标件，与第①步记忆的旧件 sha 比
  snap="$(mktemp "${TMPDIR:-/tmp}/aw-precheck.XXXXXX")" || die 2 "拒写：预校快照建不起来"
  cp -p "${TARGET_ABS}" "${snap}"
  CUR_SHA="$(sha_of "${TARGET_ABS}")"
  if [ "${CUR_SHA}" != "${OLD_SHA}" ]; then
    warn "   ✗ 写前预校失败：目标件现值 sha 与第①步读到的旧件 sha 不同"
    warn "     记忆旧件 sha256 ${OLD_SHA}"
    warn "     目标件现 sha256 ${CUR_SHA}"
    warn "     ⇒ 有人在本件开工后中途改过 ${TARGET_REL} ⇒ 拒写、不给结论（目标件一个字节不动）"
    if cmp -s "${TARGET_ABS}" "${snap}"; then
      warn "   cmp：目标件与预校时刻读数逐字节相同（cmp -s ⇒ rc=0）⇒ 本件未写一个字节"
    else
      warn "   cmp：目标件与预校时刻读数不同（cmp -s ⇒ rc=1）—— 异常，请人工核"
    fi
    rm -f "${snap}"
    exit 2
  fi
  rm -f "${snap}"
fi

NEW_SHA="$(sha_of "${FROM_ABS}")" || die 1 "算新件 sha 失败 ⇒ 拒写（${TARGET_REL} 未动）"
TMP="${TARGET_ABS}.atomicwrite.$$"
cp -p "${FROM_ABS}" "${TMP}" || die 1 "写临时件失败 ⇒ 拒写（${TARGET_REL} 未动）"
if [ "$(sha_of "${TMP}")" != "${NEW_SHA}" ]; then
  rm -f "${TMP}"; die 1 "临时件回读 sha 不符 ⇒ 拒写（${TARGET_REL} 未动）"
fi
if ! mv -f "${TMP}" "${TARGET_ABS}"; then
  rm -f "${TMP}"; die 1 "落目标失败 ⇒ 拒写"
fi
FINAL_SHA="$(sha_of "${TARGET_ABS}")"
if [ "${FINAL_SHA}" != "${NEW_SHA}" ]; then
  die 1 "写后回读 sha 不符（${FINAL_SHA} ≠ ${NEW_SHA}）⇒ 异常，请人工核"
fi
printf '✅ 换件成功：%s（%s 字节 · sha256 %s）\n' "${TARGET_REL}" "$(bytes_of "${TARGET_ABS}")" "${FINAL_SHA}"
if [ -n "${OLD_SHA}" ]; then
  printf '   旧件 sha256 %s → 新件 sha256 %s（旧件已在 %s/ 留档）\n' "${OLD_SHA}" "${FINAL_SHA}" "${HIST_REL}"
else
  printf '   首建（无旧件可留档）\n'
fi
exit 0

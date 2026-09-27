#!/usr/bin/env bash
# isolate-ro.sh —— **只读隔离**：给「在别处试」一条不会误删真仓的路（根因 `GAP-20260926-20` P0）。
#
# 事故形态（本件要防的那一件）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，
#   其中一条 `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata`，随后
#   `rm -f <农场>/…/cli-matrix.json` —— 删掉的其实是**真仓的件**（符号链接把删除动作透传回去了）。
# ⇒ 规矩只有一条：**「在别处试」= 硬拷贝出一棵独立的工作树**；真仓只读、绝不建链接、绝不删。
#
# 三个子命令
# --------
#   new    <真仓>   [--dst <目录>] [--include <glob>]… [--exclude <路径>]… [--all]
#                   [--reject-symlinks] [--no-manifest] [--quiet]
#       在 `${TMPDIR:-/tmp}` 下硬拷贝出一棵**可随意读写**的工作树；缺省排除重件
#       （`.git` / `bin` / `data` / `dist` / `node_modules` / `vendor` / `ui/target` / `zerg-wt` …）。
#       ★ 源路径本身是**符号链接** ⇒ 打印一行说明后**按它解析到的真身硬拷贝**（`--reject-symlinks` ⇒ 直接拒绝）。
#       ★ 副本里任何**指向副本之外**（含指回真仓）的符号链接，一律**换成硬拷贝**（`--reject-symlinks` ⇒ 拒绝并点名）。
#       ★ 拷贝完在副本根写 `ISOLATED.md`（「这是副本，删无妨；真仓是 …」）+ `ISOLATED.manifest.tsv`（逐件 sha256）。
#       输出：真仓路径 · 副本路径 · 件数 · 总字节。
#
#   verify <副本> <真仓> [--quiet] [--max-examples N]
#       逐件 sha256 **对拍副本与真仓**（副本侧优先取 `new` 留下的 manifest，缺则现算）
#       ⇒ 用来**证明这一轮验证过程一个字节都没改到真仓**。rc：0 全一致 · 1 有差异（点名）· 2 判不了。
#
#   self-test
#       `--self-test` 三档：正控（隔离→写副本→对拍 rc=0）· 负控（改副本 ⇒ rc=1；真仓里造回真仓的链接 ⇒ 被拒/被换）
#       · 用法错（缺参数 / 真仓不存在 ⇒ rc=2）。合成夹具全在临时目录，**一件真东西都不碰**。
#
# 用法
# ----
#     bash scripts/dev/isolate-ro.sh new "<真仓>" --include 'core/cmd/zerg/testdata/*'
#     bash scripts/dev/isolate-ro.sh verify "$TREE" "<真仓>"
#     bash scripts/dev/isolate-ro.sh --self-test
#
# 退码：0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）
#
# 硬规矩（本件自己也守）
#   · 真仓**只读**：本件只 `rsync`/`tar` 读它、`find` 读它；**从不在真仓里 `rm`、从不往真仓里 `ln -s`**。
#   · 一切写动作只发生在临时目录 / 副本 / `--dst` 指定处。
#   · **不接管道取退码**：每条命令的 rc 单独判，红了就停（`set -o pipefail` 只作兜底）。
#   · 不联网 · 不起后台进程 · 不 sudo。
#
# 与命令面的关系（**欠账已入账，不自作主张**）
#   正门应当是 `zerg isolate-ro …`（`GAP-20260926-20` 已登记「无只读隔离工具面」）；
#   本件是**命令面补齐之前的临时实现** —— 用完请把欠账停在账上，别把它当第二条对外面 ✗。
set -uo pipefail

SELF="${BASH_SOURCE[0]}"
HERE="$(cd "$(dirname "${SELF}")" && pwd)"
PROG="$(basename "${SELF}")"
TMPBASE="${TMPDIR:-/tmp}"
TMPBASE="${TMPBASE%/}"

# 缺省排除（重件 / 构建产物 / 运行期数据）—— 通配语义见 docstring
DEFAULT_EXCLUDES=(.git .zerg bin data dist node_modules vendor zerg-wt __pycache__ .venv venv htmlcov .cargo target .build _history)
DEFAULT_NAME="isolate-ro"

usage() { sed -n '2,42p' "${SELF}" | sed 's/^# \{0,1\}//'; }

say()  { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }
die()  { local rc="$1"; shift; warn "$*"; exit "${rc}"; }

# ── 小工具 ────────────────────────────────────────────────────────────────
abs_path() { case "$1" in /*) printf '%s\n' "$1" ;; *) printf '%s\n' "$(pwd)/$1" ;; esac; }

resolved_path() {  # 解析符号链接到真身（不存在的尾巴也照解，交给调用方判在不在）
  python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$1" 2>/dev/null
}

count_files() { find "$1" -type f -print 2>/dev/null | wc -l | tr -d ' '; }

total_bytes() {
  find "$1" -type f -exec stat -f '%z' {} + 2>/dev/null \
    | awk 'BEGIN{s=0} {s+=$1} END{print s+0}'
}

hash_of() {
  if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else sha256sum "$1" | awk '{print $1}'; fi
}

human_bytes() { awk -v b="$1" 'BEGIN{ split("B KB MB GB TB",u," "); i=1; while (b>=1024 && i<5){ b/=1024; i++ } printf "%.1f %s", b, u[i] }'; }

# ── new ──────────────────────────────────────────────────────────────────
SRC_RAW=""; DST=""; REJECT=0; WANT_MANIFEST=1; QUIET=0; ALL=0
INCLUDES=(); USER_EXCLUDES=()

build_filters() {  # 打印 rsync 过滤器行（顺序即优先级：include 在前，`--exclude=*` 收尾）
  local g p acc d
  if [ "${#INCLUDES[@]}" -eq 0 ]; then
    for e in ${DEFAULT_EXCLUDES[@]+"${DEFAULT_EXCLUDES[@]}"}; do printf '%s\n' "--exclude=/$e"; done
    for e in ${USER_EXCLUDES[@]+"${USER_EXCLUDES[@]}"}; do printf '%s\n' "--exclude=$e"; done
    return 0
  fi
  for g in ${INCLUDES[@]+"${INCLUDES[@]}"}; do
    p="${g%%[*?]*}"; p="${p%/}"
    acc=""
    local oldifs="$IFS"; IFS='/'
    for d in $p; do
      [ -n "$d" ] || continue
      acc="${acc}${acc:+/}${d}"
      printf '%s\n' "--include=/$acc/"
    done
    IFS="$oldifs"
    printf '%s\n' "--include=/$g"
    printf '%s\n' "--include=/$g/***"
  done
  if [ "${ALL}" -eq 0 ]; then
    for e in ${DEFAULT_EXCLUDES[@]+"${DEFAULT_EXCLUDES[@]}"}; do printf '%s\n' "--exclude=/$e"; done
  fi
  for e in ${USER_EXCLUDES[@]+"${USER_EXCLUDES[@]}"}; do printf '%s\n' "--exclude=$e"; done
  printf '%s\n' '--exclude=*'
}

copy_tree() {  # <src> <dst>
  local src="$1" dst="$2" line rsync_bin rc
  local filters=()
  rsync_bin="$(command -v rsync || true)"
  if [ -n "${rsync_bin}" ]; then
    while IFS= read -r line; do filters+=("$line"); done < <(build_filters)
    "${rsync_bin}" -a ${filters[@]+"${filters[@]}"} "${src}/" "${dst}/"
    rc=$?
    [ "${rc}" -eq 0 ] || return "${rc}"
    return 0
  fi
  # 兜底：bsdtar（无 rsync 时）—— 有 include 白名单时明确判不了，绝不静默放宽
  if [ "${#INCLUDES[@]}" -ne 0 ]; then
    warn "  ✗ 缺 rsync ⇒ 带 --include 的隔离判不了（rc=2）：不猜、不放宽"
    return 2
  fi
  local ex=() e; for e in ${DEFAULT_EXCLUDES[@]+"${DEFAULT_EXCLUDES[@]}"}; do ex+=(--exclude "./$e"); done
  for e in ${USER_EXCLUDES[@]+"${USER_EXCLUDES[@]}"}; do ex+=(--exclude "$e"); done
  local arch="${dst}.tar"
  tar -cf "${arch}" -C "${src}" "${ex[@]}" . ; rc=$?
  [ "${rc}" -eq 0 ] || { rm -f "${arch}"; return "${rc}"; }
  tar -xf "${arch}" -C "${dst}" ; rc=$?
  rm -f "${arch}"
  return "${rc}"
}

fix_links() {  # <dst> —— 副本里指向副本之外的链接：换硬拷贝（REJECT=1 ⇒ 拒绝并计数）
  local dst="$1" n=0 bad=0 link tgt abs res
  while IFS= read -r link; do
    [ -n "${link}" ] || continue
    tgt="$(readlink "${link}")" || { bad=$((bad+1)); continue; }
    case "${tgt}" in
      /*) abs="${tgt}" ;;
      *)  abs="$(cd "$(dirname "${link}")" && pwd)/${tgt}" ;;
    esac
    res="$(resolved_path "${abs}")"
    [ -n "${res}" ] || res="${abs}"
    case "${res}" in
      "${dst}"/*) continue ;;   # 指在副本内部：安全（仍指向副本，删也删副本）
    esac
    if [ "${REJECT}" -eq 1 ]; then
      warn "  ✗ 拒绝：副本里有指向副本之外的符号链接 —— ${link#"${dst}"/} → ${res}"
      bad=$((bad+1)); continue
    fi
    if [ -d "${res}" ]; then
      rm -f "${link}"; mkdir -p "${link}"; cp -R "${res}/." "${link}/"
    elif [ -f "${res}" ]; then
      rm -f "${link}"; cp -p "${res}" "${link}"
    else
      warn "  ! 悬空符号链接（目标读不到）⇒ 留原样但记一笔：${link#"${dst}"/} → ${res}"
      bad=$((bad+1)); continue
    fi
    local rc=$?
    [ "${rc}" -eq 0 ] || { warn "  ✗ 硬拷贝链接目标失败：${link}（rc=${rc}）"; bad=$((bad+1)); continue; }
    n=$((n+1))
    say "  · 链接→硬拷贝：${link#"${dst}"/} ⇐ ${res}"
  done < <(find "${dst}" -type l -print 2>/dev/null)
  LINKS_REPLACED="${n}"
  return "${bad}"
}

write_isolated_md() {  # <dst> <real> <src_arg_is_link>
  local dst="$1" real="$2" note="$3"
  {
    printf '# 这是副本 —— 删无妨\n\n'
    printf '真仓路径：`%s`\n\n' "${real}"
    printf '副本路径：`%s`\n\n' "${dst}"
    printf '生成时间：%s\n\n' "$(date '+%Y-%m-%d %H:%M:%S %z')"
    printf '生成者：`scripts/dev/isolate-ro.sh new`\n\n'
    printf '拷贝口径：rsync -a（排除重件：%s）\n\n' "${DEFAULT_EXCLUDES[*]}"
    printf '符号链接：本树里**没有**指向真仓的链接（已逐条换成硬拷贝 %s 条）。%s\n\n' "${LINKS_REPLACED}" "${note}"
    printf '## 规矩（照 GAP-20260926-20 的根因）\n\n'
    printf '1. 只在本树里读写、删除；**不要** `ln -s` 指回真仓（指回去的链接会让删除动作透传到真仓）。\n'
    printf '2. 要更多真仓内容 ⇒ 重跑 `new`（可以 `--include`），**不要**在本树里挂真仓的链接。\n'
    printf '3. 收尾验回：`bash scripts/dev/isolate-ro.sh verify %s %s` ⇒ 必须 rc=0（证明真仓一个字节没动）。\n\n' "${dst}" "${real}"
    printf '## 清单\n\n`ISOLATED.manifest.tsv` = 本树逐件 sha256 + 字节数（对拍真仓用）。\n'
  } > "${dst}/ISOLATED.md"
}

write_manifest() {  # <dst> <real>
  local dst="$1" real="$2" rel h sz ir
  local out="${dst}/ISOLATED.manifest.tsv"   # ★ 不许与 dst 同一条 local：declare 先展开后赋值
  printf 'relpath\tsha256\tbytes\tin_real\n' > "${out}"
  while IFS= read -r rel; do
    [ -n "${rel}" ] || continue
    h="$(hash_of "${dst}/${rel}")"
    sz="$(stat -f '%z' "${dst}/${rel}" 2>/dev/null || echo 0)"
    if [ -f "${real}/${rel}" ]; then ir=yes; else ir=no; fi
    printf '%s\t%s\t%s\t%s\n' "${rel}" "${h}" "${sz}" "${ir}" >> "${out}"
  done < <(cd "${dst}" && find . -type f ! -name 'ISOLATED.md' ! -name 'ISOLATED.manifest.tsv' -print | sed 's|^\./||')
}

cmd_new() {
  [ -n "${SRC_RAW}" ] || die 2 "用法错：new 缺 <真仓>（见 --help）"
  local src_arg; src_arg="$(abs_path "${SRC_RAW}")"
  local note=''
  if [ -L "${src_arg}" ]; then
    local real; real="$(resolved_path "${src_arg}")"
    if [ "${REJECT}" -eq 1 ]; then
      die 1 "拒绝：源路径本身是符号链接（${src_arg} → ${real}）；--reject-symlinks 档不许硬拷贝其目标"
    fi
    say "· 源路径是**符号链接** ⇒ 按真身硬拷贝：${src_arg} → ${real}"
    note='（源参数是符号链接，已按真身拷。）'
    src_arg="${real}"
  fi
  [ -d "${src_arg}" ] || die 2 "判不了：真仓目录不存在 —— ${src_arg}"
  local real_repo; real_repo="$(resolved_path "${src_arg}")"
  [ -n "${real_repo}" ] || real_repo="${src_arg}"

  if [ -z "${DST}" ]; then
    DST="$(mktemp -d "${TMPBASE}/${DEFAULT_NAME}.XXXXXX")" || die 2 "判不了：mktemp -d 失败"
    # ★ **不挂 EXIT trap**：副本要活过本脚本退出（它就是给人「随便读写」的那棵树）；
    #   收尾由使用者自己 `rm -rf <副本>`（见收尾行打印）。
  else
    DST="$(abs_path "${DST}")"
    [ -e "${DST}" ] || mkdir -p "${DST}" || die 2 "判不了：建不出 --dst —— ${DST}"
  fi

  say "== 只读隔离（硬拷贝）=="
  say "· 真仓：${real_repo}"
  say "· 副本：${DST}"
  [ "${#INCLUDES[@]}" -eq 0 ] || say "· 白名单 --include：${INCLUDES[*]}"
  copy_tree "${real_repo}" "${DST}"; local rc=$?
  if [ "${rc}" -eq 2 ]; then exit 2; fi
  [ "${rc}" -eq 0 ] || die 1 "拷贝失败（rsync/tar rc=${rc}）—— 副本可能不完整，别拿它当基线"

  LINKS_REPLACED=0
  fix_links "${DST}"; rc=$?
  [ "${rc}" -eq 0 ] || die 1 "副本里有 ${rc} 条**危险符号链接**（指向副本之外/悬空）—— 见上；不许在这种树上做删除动作"

  # ★ 件数/字节**先**读（此时副本里只有真仓件；ISOLATED.md 与 manifest 是隔离件自身的产物，不计入）
  local n b
  n="$(count_files "${DST}")"
  b="$(total_bytes "${DST}")"

  if [ "${WANT_MANIFEST}" -eq 1 ]; then write_manifest "${DST}" "${real_repo}"; fi
  write_isolated_md "${DST}" "${real_repo}" "${note}"
  [ "${n}" -gt 0 ] || die 2 "空转：副本里 0 件 ⇒ 判不了（别把空树当隔离成功）"
  say "== 读数 =="
  say "· 真仓路径：${real_repo}"
  say "· 副本路径：${DST}"
  say "· 件数：${n} · 总字节：${b}（$(human_bytes "${b}")）"
  say "· 链接→硬拷贝：${LINKS_REPLACED} 条 · ISOLATED.md：${DST}/ISOLATED.md"
  say "· 验回：bash scripts/dev/isolate-ro.sh verify \"${DST}\" \"${real_repo}\""
  say "· 收尾（本件**不会**替你删）：rm -rf \"${DST}\""
  exit 0
}

# ── verify ───────────────────────────────────────────────────────────────
COPY_DIR=""; REAL_DIR=""; MAX_EXAMPLES=20

cmd_verify() {
  [ -n "${COPY_DIR}" ] && [ -n "${REAL_DIR}" ] || die 2 "用法错：verify 要 <副本> <真仓> 两个实参（见 --help）"
  local copy real; copy="$(abs_path "${COPY_DIR}")"; real="$(abs_path "${REAL_DIR}")"
  [ -d "${copy}" ] || die 2 "判不了：副本目录不存在 —— ${copy}"
  [ -d "${real}" ] || die 2 "判不了：真仓目录不存在 —— ${real}"

  local manifest="${copy}/ISOLATED.manifest.tsv" src_note="现算副本侧 sha256"
  [ -f "${manifest}" ] && src_note="副本侧取 ISOLATED.manifest.tsv（隔离当时的读数）"

  say "== 逐件 sha256 对拍（副本 ⟷ 真仓）=="
  say "· 副本：${copy}"
  say "· 真仓：${real}"
  say "· 副本侧口径：${src_note}"

  local total=0 same=0 diff=0 absent=0 rel ch rh
  local diffs=()
  while IFS= read -r rel; do
    [ -n "${rel}" ] || continue
    total=$((total+1))
    if [ -f "${manifest}" ]; then
      ch="$(awk -F'\t' -v r="${rel}" '$1==r{print $2; exit}' "${manifest}")"
    fi
    if [ -z "${ch:-}" ]; then ch="$(hash_of "${copy}/${rel}")"; fi
    if [ ! -f "${real}/${rel}" ]; then absent=$((absent+1)); diffs+=("真仓侧缺件: ${rel}"); continue; fi
    rh="$(hash_of "${real}/${rel}")"
    if [ "${ch}" = "${rh}" ]; then same=$((same+1))
    else diff=$((diff+1)); diffs+=("sha256 不一致: ${rel}"); fi
  done < <(cd "${copy}" && find . -type f ! -name 'ISOLATED.md' ! -name 'ISOLATED.manifest.tsv' -print | sed 's|^\./||')

  say "· 对拍件数：${total} · 一致：${same} · 不一致：${diff} · 真仓侧缺件：${absent}"
  if [ "${total}" -eq 0 ]; then
    die 2 "空转：副本里 0 件 ⇒ 判不了（空树不算「证明没改到真仓」）"
  fi
  if [ "${diff}" -gt 0 ] || [ "${absent}" -gt 0 ]; then
    local i=0
    for d in "${diffs[@]}"; do
      i=$((i+1)); [ "${i}" -le "${MAX_EXAMPLES}" ] || break
      say "  ✗ ${d}"
    done
    die 1 "真仓与副本对不上 ⇒ 要么真仓被改过、要么副本被改过 —— 先查清再谈结论"
  fi
  say "⇒ rc=0：副本逐件与真仓一致 ⇒ **这一轮验证一个字节都没改到真仓**"
  exit 0
}

# ── self-test（正控 / 负控 / 用法错 · 合成夹具）──────────────────────────
run_self_test() {
  local tmp bad=0
  tmp="$(mktemp -d "${TMPBASE}/${DEFAULT_NAME}-selftest.XXXXXX")" || exit 2
  trap 'rm -rf "${tmp}"' EXIT
  local fix="${tmp}/fix"
  mkdir -p "${fix}/real/core/cmd/zerg/testdata" "${fix}/real/scripts/dev" "${fix}/real/bin"
  printf 'matrix\n'      > "${fix}/real/core/cmd/zerg/testdata/cli-matrix.json"
  printf 'keep\n'        > "${fix}/real/scripts/dev/keep.sh"
  printf 'heavy\n'       > "${fix}/real/bin/big.bin"
  local real_sha_before; real_sha_before="$(hash_of "${fix}/real/core/cmd/zerg/testdata/cli-matrix.json")"

  chk() { if eval "$2"; then say "   ✓ $1"; else say "   ✗ $1${3:+（$3）}"; bad=$((bad+1)); fi; }
  say "── isolate-ro.sh 自检（成对控 · 合成夹具：${fix}）──"

  # ── 正控：隔离一棵树 ⇒ rc=0 · ISOLATED.md 在 · 副本可写 · 真仓未变 ──
  local out rc
  out="$(bash "${SELF}" new "${fix}/real" --dst "${tmp}/tree" 2>&1)"; rc=$?
  printf '%s\n' "${out}" > "${tmp}/new.log"
  chk "正控：new ⇒ rc=0" "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "正控：副本里有 ISOLATED.md" "[ -f '${tmp}/tree/ISOLATED.md' ]" ""
  chk "正控：ISOLATED.md 写着真仓路径" "grep -q '${fix}/real' '${tmp}/tree/ISOLATED.md'" ""
  chk "正控：件数/总字节都打印了（3 件 − 被排除的 bin/ 1 件 = 2" "grep -q '件数：2' '${tmp}/new.log' && grep -q '总字节：' '${tmp}/new.log'" ""
  chk "正控：缺省排除生效（bin/ 没进副本）" "[ ! -e '${tmp}/tree/bin' ]" ""
  chk "正控：硬拷贝是真件（同一字节）" "[ \"\$(shasum -a 256 < '${tmp}/tree/core/cmd/zerg/testdata/cli-matrix.json' | awk '{print \$1}')\" = '${real_sha_before}' ]" ""
  printf 'edited-in-copy\n' > "${tmp}/tree/scripts/dev/keep.sh"     # 在副本里随便写
  rm -f "${tmp}/tree/core/cmd/zerg/testdata/cli-matrix.json"        # 在副本里随便删
  out="$(bash "${SELF}" verify "${tmp}/tree" "${fix}/real" 2>&1)"; rc=$?
  chk "正控：verify ⇒ rc=0（真仓未被改）" "[ ${rc} -eq 0 ]" "rc=${rc}"
  chk "正控：真仓那件 sha256 未变（删的是副本）" "[ \"\$(shasum -a 256 < '${fix}/real/core/cmd/zerg/testdata/cli-matrix.json' | awk '{print \$1}')\" = '${real_sha_before}' ]" ""

  # ── 负控 A：真仓被改一个字节（模拟误删/误改）⇒ verify 必须 rc=1 且点名 ──
  local real_backup; cp -p "${fix}/real/scripts/dev/keep.sh" "${tmp}/keep.bak"
  printf 'SOMEONE-TOUCHED-THE-REAL-REPO\n' > "${fix}/real/scripts/dev/keep.sh"
  out="$(bash "${SELF}" verify "${tmp}/tree" "${fix}/real" 2>&1)"; rc=$?
  printf '%s\n' "${out}" > "${tmp}/neg-verify.log"
  chk "负控 A：真仓被改 ⇒ verify rc=1" "[ ${rc} -eq 1 ]" "rc=${rc}"
  chk "负控 A：点名那一件" "grep -q 'scripts/dev/keep.sh' '${tmp}/neg-verify.log'" ""
  cp -p "${tmp}/keep.bak" "${fix}/real/scripts/dev/keep.sh"

  # ── 负控 B：真仓里有一条**指回真仓**的符号链接（事故形态）⇒ 缺省档换成硬拷贝 / 严格档拒绝 ──
  mkdir -p "${fix}/farmsrc"
  ln -s "${fix}/real/core/cmd/zerg/testdata" "${fix}/farmsrc/testdata-link"
  out="$(bash "${SELF}" new "${fix}/farmsrc" --dst "${tmp}/farm" 2>&1)"; rc=$?
  printf '%s\n' "${out}" > "${tmp}/neg-link.log"
  chk "负控 B：缺省档 rc=0 且把链接换成硬拷贝（副本里 0 条链接）" "[ ${rc} -eq 0 ] && [ \"\$(find '${tmp}/farm' -type l | wc -l | tr -d ' ')\" = 0 ]" "rc=${rc}"
  chk "负控 B：链接目标内容是真件（可读、对得上）" "grep -q 'matrix' '${tmp}/farm/testdata-link/cli-matrix.json'" ""
  out="$(bash "${SELF}" new "${fix}/farmsrc" --dst "${tmp}/farm2" --reject-symlinks 2>&1)"; rc=$?
  printf '%s\n' "${out}" > "${tmp}/neg-link2.log"
  chk "负控 B'：--reject-symlinks ⇒ rc=1 且点出链接" "[ ${rc} -eq 1 ] && grep -q 'testdata-link' '${tmp}/neg-link2.log'" "rc=${rc}"

  # ── 用法错 ──
  out="$(bash "${SELF}" new 2>&1)"; rc=$?
  chk "用法错：new 缺参数 ⇒ rc=2" "[ ${rc} -eq 2 ]" "rc=${rc}"
  out="$(bash "${SELF}" new "${tmp}/no-such-repo" 2>&1)"; rc=$?
  chk "用法错：真仓不存在 ⇒ rc=2" "[ ${rc} -eq 2 ]" "rc=${rc}"
  out="$(bash "${SELF}" verify "${tmp}/tree" 2>&1)"; rc=$?
  chk "用法错：verify 缺一个实参 ⇒ rc=2" "[ ${rc} -eq 2 ]" "rc=${rc}"
  out="$(bash "${SELF}" bogus 2>&1)"; rc=$?
  chk "用法错：未知子命令 ⇒ rc=2" "[ ${rc} -eq 2 ]" "rc=${rc}"

  say "── 自检结论：$([ ${bad} -eq 0 ] && echo 全过 || echo "**不过**（${bad} 条）") ──"
  [ "${bad}" -eq 0 ] && exit 0 || exit 2
}

# ── 入口 ─────────────────────────────────────────────────────────────────
CMD=""
while [ $# -gt 0 ]; do
  case "$1" in
    new|verify) [ -z "${CMD}" ] || die 2 "用法错：只给一个子命令（见 --help）"; CMD="$1"; shift ;;
    --self-test) run_self_test ;;
    -h|--help) usage; exit 0 ;;
    --dst) [ $# -ge 2 ] || die 2 "用法错：--dst 缺实参"; DST="$2"; shift 2 ;;
    --include) [ $# -ge 2 ] || die 2 "用法错：--include 缺实参"; INCLUDES+=("$2"); shift 2 ;;
    --exclude) [ $# -ge 2 ] || die 2 "用法错：--exclude 缺实参"; USER_EXCLUDES+=("$2"); shift 2 ;;
    --max-examples) [ $# -ge 2 ] || die 2 "用法错：--max-examples 缺实参"; MAX_EXAMPLES="$2"; shift 2 ;;
    --reject-symlinks) REJECT=1; shift ;;
    --no-manifest) WANT_MANIFEST=0; shift ;;
    --all) ALL=1; shift ;;
    --quiet) QUIET=1; shift ;;
    -*) die 2 "用法错：未知旗标 $1（见 --help）" ;;
    *)
      if [ "${CMD}" = "new" ] && [ -z "${SRC_RAW}" ]; then SRC_RAW="$1"
      elif [ "${CMD}" = "verify" ] && [ -z "${COPY_DIR}" ]; then COPY_DIR="$1"
      elif [ "${CMD}" = "verify" ] && [ -z "${REAL_DIR}" ]; then REAL_DIR="$1"
      else die 2 "用法错：多余的位置参数 $1（见 --help）"; fi
      shift ;;
  esac
done

case "${CMD}" in
  new) cmd_new ;;
  verify) cmd_verify ;;
  "") usage; exit 2 ;;
  *) die 2 "用法错：未知子命令 ${CMD}（见 --help）" ;;
esac

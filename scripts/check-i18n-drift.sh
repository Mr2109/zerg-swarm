#!/usr/bin/env bash
# =============================================================================
# check-i18n-drift.sh —— 双语漂移三件套（**命令形态** · 可原地复跑）
# -----------------------------------------------------------------------------
# 规格来源：docs/01-设计/设计-文档体系-v1.0.md §0.6 P6 ⑤（守漂移三件套）
#           依据调研：docs/调研/调研-文档分块落地-检索与双语.md §2.4
#
# 三相（各相独立退码 —— 禁尾部软检查，设计稿 §6.2 血坑 1）：
#   ① exist   存在性对照：docs/zh/<path> 与 docs/en/<path> **两侧都要在**
#              对标 K8s scripts/linkchecker.py 头注释（本地化页要查「英文版在不在 + 中文版在不在」）
#              **阻塞**（rc=2）
#   ② stale   过期检测（**commit 级**）：译页记录的 `source_commit` ≠ 源页当前状态 ⇒ 过期
#              对标 MDN frontmatter `l10n.sourceCommit`（「the commit hash of the upstream commit
#              the translation is synchronized with」+「enforced by a linter」）
#              与 K8s scripts/lsync.sh（`git log -n 1 … -- <译页>` + `git diff --exit-code --numstat`）
#              **阻塞**（rc=2）。只读 git（rev-parse/log/diff），不写 git。
#              ★ 2026-09-19 开发文档分家批6：源页可**迁出仓**（工作树之外的同级目录 `Zerg-内部文档/`）——
#              那种情况下本门的 commit 级判据**不适用**（`git diff … -- <仓外路径>` 实测 rc=128
#              `fatal: … is outside repository`，照原样跑会被读成「过期」= 假红）。处置 = 目标**存在但
#              在仓外** ⇒ 单列「源页在仓外 · 不判过期」逐条打印 + 计数（`tr_out_of_repo()`），
#              既不静默放绿、也不冒充过期；仓内源页照原判据判。
#   ③ comment 源文注释：译页必须有 HTML 注释块（段落级保留源文，§2.4③）
#              **先非阻塞**（rc=1 告警）—— 与设计稿「③ 先非阻塞」一致
#
# `source_commit` 从哪读（两处，前者优先；两处都读是为了不碰 frontmatter schema）：
#   a) 译页 frontmatter 的 `source_commit:`（若 schema 将来放行该字段，直接生效）
#   b) 源文注释块里的 `source_commit: <sha>` 标记行（**当前形态**：
#      docs/site/frontmatter-schema.json 是 `additionalProperties: false` ⇒ 未申报字段写进
#      frontmatter 即红 ⇒ 在补进 schema 之前，标记行落在注释里）
#
# 用法：
#   scripts/check-i18n-drift.sh [--phase all|exist|stale|comment] [--root docs] [--langs zh,en]
#                               [--selftest]
# 退码：0 = 三相全绿 · 1 = 仅告警相命中（comment）· 2 = 有阻塞相命中（exist/stale）
#       3 = 环境/输入异常（不在仓根 / 无 git / 扫到 0 篇 —— 不静默空转）
# =============================================================================
set -u

PHASE="all"
ROOT="docs"
LANGS="zh,en"
SELFTEST=0

while [ $# -gt 0 ]; do
  case "$1" in
    --phase) PHASE="${2:-all}"; shift 2 ;;
    --root)  ROOT="${2:-docs}";  shift 2 ;;
    --langs) LANGS="${2:-zh,en}"; shift 2 ;;
    --selftest) SELFTEST=1; shift ;;
    -h|--help) grep '^#' "$0" | head -40; exit 0 ;;
    *) echo "✗ 未知参数：$1" >&2; exit 3 ;;
  esac
done

SRC_LANG="$(printf '%s' "$LANGS" | cut -d, -f1)"
DST_LANG="$(printf '%s' "$LANGS" | cut -d, -f2)"
[ -n "$SRC_LANG" ] && [ -n "$DST_LANG" ] || { echo "✗ --langs 需要两个语言码（如 zh,en）" >&2; exit 3; }

# --- 仓根与 git（只读）-------------------------------------------------------
if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
  echo "✗ 不在 git 仓内（⑵ 过期检测要读 commit 历史）⇒ 拒绝给结论" >&2
  exit 3
fi
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT" || exit 3
HEAD_SHA="$(git rev-parse HEAD 2>/dev/null || true)"
[ -n "$HEAD_SHA" ] || { echo "✗ 拿不到 HEAD（空仓？）⇒ 拒绝给结论" >&2; exit 3; }

fm_value() {   # fm_value <文件> <键>  —— 只读 frontmatter 标量键（行式，不引 YAML 依赖）
  awk -v k="$2" '
    NR==1 && $0 !~ /^---[[:space:]]*$/ { exit }
    NR==1 { infm=1; next }
    infm && /^---[[:space:]]*$/ { exit }
    infm { if (index($0, k":")==1) { v=substr($0, length(k)+2); sub(/^[[:space:]]+/,"",v); sub(/[[:space:]]+$/,"",v); gsub(/"/,"",v); print v; exit } }
  ' "$1"
}

marker_value() {   # marker_value <文件> <键> —— 源文注释块里的 `键: 值` 标记行（§2.4③ 的机器可读形态）
  grep -m1 -E "^[[:space:]]*<!--?[[:space:]]*$2:" "$1" 2>/dev/null | head -1 \
    | sed -e "s/.*$2:[[:space:]]*//" -e 's/-->.*$//' -e 's/[[:space:]].*$//'
}

source_commit_of() {   # 先 frontmatter，再注释标记行
  local v
  v="$(fm_value "$1" source_commit)"
  [ -n "$v" ] || v="$(marker_value "$1" source_commit)"
  printf '%s' "$v"
}

tr_out_of_repo() {   # tr_out_of_repo <路径> —— 目标落在 REPO_ROOT **之外** ⇒ 0（真）；仓内 / 取不到 ⇒ 1
  # ★ 2026-09-19 开发文档分家批6：源页可以**迁出仓**（工作树之外的同级目录 `Zerg-内部文档/`）⇒ 本门的 commit 级
  #   判据（`git diff <source_commit>..HEAD -- <源页>`）在那时**根本无法适用**（实测：仓外路径 ⇒
  #   `fatal: … is outside repository`、rc=128 ⇒ 会被读成「过期」= 假红）。判据一律以**路径真身**为准
  #   （dirname 的 realpath 是否落在 REPO_ROOT 之下），不看路径写法（`../Zerg-内部文档/…` 与 `Zerg-内部文档/…` 等效）。
  local p="$1" d b
  [ -n "$p" ] || return 1
  d="$(cd "$(dirname "$p")" 2>/dev/null && pwd)" || return 1
  [ -n "$d" ] || return 1
  b="$(basename "$p")"
  case "$d/$b" in
    "$REPO_ROOT"/*) return 1 ;;
    *) return 0 ;;
  esac
}

list_pages() {   # list_pages <dir> <是否排除草稿与 README: 0/1>
  [ -d "$1" ] || return 0
  if [ "$2" = "1" ]; then
    find "$1" -type f -name '*.md' ! -name 'README.md' ! -path '*_drafts/*' | sort
  else
    find "$1" -type f -name '*.md' ! -name 'README.md' | sort
  fi
}

# --- 相① 存在性对照（阻塞）---------------------------------------------------
phase_exist() {
  local rc=0 n=0 miss=0
  local f rel other
  for f in $(list_pages "$ROOT/$SRC_LANG" 1); do
    n=$((n+1))
    rel="${f#"$ROOT/$SRC_LANG/"}"
    other="$ROOT/$DST_LANG/$rel"
    if [ ! -f "$other" ]; then
      echo "   ✗ $f  缺 $DST_LANG 侧：$other"
      miss=$((miss+1))
    fi
  done
  for f in $(list_pages "$ROOT/$DST_LANG" 1); do
    n=$((n+1))
    rel="${f#"$ROOT/$DST_LANG/"}"
    other="$ROOT/$SRC_LANG/$rel"
    if [ ! -f "$other" ]; then
      echo "   ✗ $f  缺 $SRC_LANG 侧：$other"
      miss=$((miss+1))
    fi
  done
  echo "   ① 存在性对照：$SRC_LANG/$DST_LANG 两侧共扫描 $n 个页面名，缺一侧 $miss 个"
  [ "$n" -gt 0 ] || { echo "   ⚠ 两侧合计 0 篇 —— 正式面还没建？"; rc=3; }
  [ "$miss" -eq 0 ] || rc=2
  return $rc
}

# --- 相② 过期检测（commit 级；阻塞）-----------------------------------------
phase_stale() {
  local rc=0 n=0 stale=0 nodate=0 outofrepo=0
  local f tr sc src_last tr_last
  for f in $(list_pages "$ROOT/$DST_LANG" 0); do
    tr="$(fm_value "$f" translation_of)"
    [ -n "$tr" ] || continue
    n=$((n+1))
    if [ ! -f "$tr" ]; then
      echo "   ✗ $f  translation_of 指向的文件不存在：$tr"
      stale=$((stale+1)); continue
    fi
    if tr_out_of_repo "$tr"; then
      echo "   ⚠ $f  源页已在**仓外**（${tr}）—— commit 级判据不适用，不判过期、不进退码（2026-09-19 开发文档分家）"
      outofrepo=$((outofrepo+1)); continue
    fi
    sc="$(source_commit_of "$f")"
    if [ -n "$sc" ]; then
      # 形态先校验：**坏值不当作「过期」**（把格式错误读成过期是假红；照字面保留、只告警）
      case "$sc" in
        *[!0-9a-fA-F]*) echo "   ⚠ $f  source_commit 形态不合法（不是 hex sha）：$sc —— 不判过期"; nodate=$((nodate+1)); continue ;;
      esac
      if ! git cat-file -e "$sc^{commit}" >/dev/null 2>&1; then
        echo "   ⚠ $f  source_commit 指向的提交在本仓不存在：$sc —— 不判过期"; nodate=$((nodate+1)); continue
      fi
      # MDN 判据：源页在 source_commit 之后动过 ⇒ 过期
      if ! git diff --exit-code --numstat "$sc"...HEAD -- "$tr" >/dev/null 2>&1; then
        echo "   ✗ $f  过期：$tr 在 $sc..HEAD 之间改动过（source_commit=${sc}）"
        stale=$((stale+1)); continue
      fi
      # 补一条：工作树里还有**未提交**的源页改动（commit 级判据看不见它 —— 已知局限）
      if ! git diff --exit-code --numstat HEAD -- "$tr" >/dev/null 2>&1; then
        echo "   ⚠ $f  源页有未提交改动（commit 级判据看不见）：$tr"
      fi
    else
      # 没记 source_commit ⇒ 退到 K8s lsync.sh 形态：比两边最后一次提交
      nodate=$((nodate+1))
      src_last="$(git log -n 1 --pretty=format:%ct -- "$tr" 2>/dev/null || true)"
      tr_last="$(git log -n 1 --pretty=format:%ct -- "$f" 2>/dev/null || true)"
      if [ -z "$src_last" ] || [ -z "$tr_last" ]; then
        echo "   ⚠ $f  二者之一无提交历史（新文件？）—— 不判过期，只记账"
        continue
      fi
      if [ "$src_last" -gt "$tr_last" ]; then
        echo "   ✗ $f  过期（lsync 形态）：源页最后一次提交晚于译页"
        stale=$((stale+1))
      fi
    fi
  done
  echo "   ② 过期检测：$n 篇译页（$nodate 篇未记 source_commit ⇒ 退 lsync 形态；$outofrepo 篇源页在仓外 ⇒ 不判过期），过期 $stale 篇"
  [ "$stale" -eq 0 ] || rc=2
  return $rc
}

# --- 相③ 源文注释（先非阻塞）------------------------------------------------
phase_comment() {
  local rc=0 n=0 nocomment=0
  local f c
  for f in $(list_pages "$ROOT/$DST_LANG" 0); do
    n=$((n+1))
    c="$(grep -c '^[[:space:]]*<!--' "$f" 2>/dev/null || true)"
    if [ "${c:-0}" -eq 0 ]; then
      echo "   ⚠ $f  无 HTML 注释块（未按 §2.4③ 段落级保留源文）"
      nocomment=$((nocomment+1))
    fi
  done
  echo "   ③ 源文注释：$n 篇译页，缺注释 $nocomment 篇（先非阻塞）"
  [ "$nocomment" -eq 0 ] || rc=1
  return $rc
}

selftest() {
  echo "── check-i18n-drift.sh 自证（5 条 · 不碰 git 写、不建临时仓）"
  local ok=0 tmp a b
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/drift-selftest.XXXXXX")"
  printf -- '---\ntranslation_of: docs/x.md\nsource_commit: abc123\n---\nbody\n' > "$tmp/a.md"
  a="$(fm_value "$tmp/a.md" translation_of)"
  b="$(fm_value "$tmp/a.md" source_commit)"
  [ "$a" = "docs/x.md" ] && [ "$b" = "abc123" ] && { echo "  ✓ frontmatter 标量读取"; ok=$((ok+1)); } \
                                     || echo "  ✗ frontmatter 标量读取（$a / ${b}）"
  printf -- 'x\n<!-- source_commit: fd69646 -->\n' > "$tmp/b.md"
  b="$(marker_value "$tmp/b.md" source_commit)"
  [ "$b" = "fd69646" ] && { echo "  ✓ 注释标记行读取"; ok=$((ok+1)); } || echo "  ✗ 注释标记行读取（${b}）"
  a="$(source_commit_of "$tmp/b.md")"
  [ "$a" = "fd69646" ] && { echo "  ✓ 两处回退链（frontmatter → 标记行）"; ok=$((ok+1)); } \
                       || echo "  ✗ 两处回退链（${a}）"
  printf -- 'no frontmatter here\n' > "$tmp/c.md"
  a="$(fm_value "$tmp/c.md" translation_of)"
  [ -z "$a" ] && { echo "  ✓ 无 frontmatter 不误判"; ok=$((ok+1)); } || echo "  ✗ 无 frontmatter 不误判（${a}）"
  # ★ 2026-09-19 开发文档分家批6 补牙：源页「在仓外」必须被识别成**不判过期**
  #   （不能读成「过期」= 假红，也不能静默放绿 —— 它单列计数、逐条打印）。
  printf -- 'x\n' > "$tmp/out.md"
  tr_out_of_repo "$tmp/out.md"; rc_out=$?
  tr_out_of_repo "$REPO_ROOT/AGENTS.md"; rc_in=$?
  if [ "$rc_out" -eq 0 ] && [ "$rc_in" -eq 1 ]; then
    echo "  ✓ 源页在仓外可识别（仓外 ⇒ 不判过期；仓内 ⇒ 照判）"; ok=$((ok+1))
  else
    echo "  ✗ 源页在仓外可识别（仓外 rc=${rc_out} 期望 0 / 仓内 rc=${rc_in} 期望 1）"
  fi
  rm -rf "$tmp"
  if [ "$ok" -eq 5 ]; then echo "  自证：通过 ✓（5 条）"; return 0; fi
  echo "  自证：**失败** ✗（$ok/5）"; return 1
}

[ "$SELFTEST" = "1" ] && { selftest; exit $?; }

# --- 跑相（各相独立退码；总码取最严：2 > 1 > 0；任一无结论 ⇒ 3）-------------
RC_EXIST=0; RC_STALE=0; RC_COMMENT=0
TOTAL=0
run_all=0
[ "$PHASE" = "all" ] && run_all=1

if [ "$run_all" = "1" ] || [ "$PHASE" = "exist" ]; then
  echo "── 相① 存在性对照（阻塞）"
  phase_exist; RC_EXIST=$?
fi
if [ "$run_all" = "1" ] || [ "$PHASE" = "stale" ]; then
  echo "── 相② 过期检测 · commit 级（阻塞；只读 git；HEAD=${HEAD_SHA%${HEAD_SHA#???????}}）"
  phase_stale; RC_STALE=$?
fi
if [ "$run_all" = "1" ] || [ "$PHASE" = "comment" ]; then
  echo "── 相③ 源文注释（先非阻塞）"
  phase_comment; RC_COMMENT=$?
fi

for r in "$RC_EXIST" "$RC_STALE" "$RC_COMMENT"; do
  [ "$r" -gt "$TOTAL" ] && TOTAL=$r
done
echo
echo "退码：exist=$RC_EXIST stale=$RC_STALE comment=$RC_COMMENT ⇒ 总码=$TOTAL"
exit "$TOTAL"

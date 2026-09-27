#!/usr/bin/env bash
# =============================================================================
# check-i18n-drift.sh —— 双语漂移三件套（**命令形态** · 可原地复跑）
# -----------------------------------------------------------------------------
# 规格来源：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md §0.6 P6 ⑤（守漂移三件套）
#           依据调研：Zerg-内部文档/调研/调研-文档分块落地-检索与双语.md §2.4
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
#   scripts/gates/check-i18n-drift.sh [--phase all|exist|stale|comment] [--root docs] [--langs zh,en]
#                               [--self-test|--selftest] [--json]
# 退码：0 = 三相全绿 · 1 = 仅告警相命中（comment）
#       2 = **不给结论 / 阻塞**（有阻塞相命中（exist/stale）· 环境/输入异常（不在仓根 / 无 git /
#           扫到 0 篇 / 参数错）—— 不静默空转）
#       ★ 2026-09-20 承接项「术语表只有一份」第 4 条：本脚本原 `rc=3` 一律**统一成 `rc=2`** ——
#         与仓内其余门脚本（check-doc-name/meta/freshness、check-glossary.py、precommit-gates.sh）
#         的 0/1/2 三档同一口径；对外效果 = 挂在公开树（无 `docs/` 面 ⇒ 扫到 0 篇）时落
#         **BLOCKED（不给结论）**，而不是被 precommit-gates.sh 的 `tri-report` 档读成 FAIL。
# =============================================================================
set -u

# 本脚本的**绝对路径**（自证要在自建临时仓里复跑自己 ⇒ 相对路径 `cd` 之后就失效了）
SELF_ABS="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

PHASE="all"
ROOT="docs"
LANGS="zh,en"
SELFTEST=0
JSON=0

while [ $# -gt 0 ]; do
  case "$1" in
    --phase) PHASE="${2:-all}"; shift 2 ;;
    --root)  ROOT="${2:-docs}";  shift 2 ;;
    --langs) LANGS="${2:-zh,en}"; shift 2 ;;
    # ★ `Q-176` 拼法统一 —— **准拼法 `--self-test`**，旧拼法 `--selftest` 作别名（两个都认；
    #   照 `all.sh` 与仓内已统一的诸门写法，**不创式样**）。两拼法同档、同一 rc。
    --self-test|--selftest) SELFTEST=1; shift ;;
    --json) JSON=1; shift ;;
    -h|--help) grep '^#' "$0" | head -40; exit 0 ;;
    *) echo "✗ 未知参数：$1" >&2; exit 2 ;;
  esac
done

SRC_LANG="$(printf '%s' "$LANGS" | cut -d, -f1)"
DST_LANG="$(printf '%s' "$LANGS" | cut -d, -f2)"
[ -n "$SRC_LANG" ] && [ -n "$DST_LANG" ] || { echo "✗ --langs 需要两个语言码（如 zh,en）" >&2; exit 2; }

# --- 计数（跨相加总用 · 三个计数**分开**：exist / stale / comment）------------------------------
# ★ 2026-09-24 波10 序105（组4 §二.2 `W-22`）：原来三个计数只活在各相函数体里、相一结束就丢，
#   外面拿不到 ⇒ 无「漂移条数」可报、也无「判定档」可分（今天恒 `rc=2` 一个形状）。
#   现在三个计数各归各的全局，**不许合并**（「过期 ≠ 未译」这条判据就钉在这里）。
EXIST_SCAN=0;   EXIST_MISS=0
STALE_PAGES=0;  STALE_HITS=0
COMMENT_PAGES=0; COMMENT_MISS=0

# tier_of <扫描数> <命中数> <空转是否算 BLOCKED: 0|1> —— 判定档（PASS / FAIL / BLOCKED）
#   空转（扫到 0 篇）⇒ BLOCKED（不给结论）—— 本仓「空转不许给绿」那条；但只对**该相**适用，
#   所以第三个参数让它按相开关（相② 无译页 ≠ 判不了）。
tier_of() {
  if [ "$3" = "1" ] && [ "$1" -eq 0 ]; then printf 'BLOCKED'; return; fi
  if [ "$2" -gt 0 ]; then printf 'FAIL'; return; fi
  printf 'PASS'
}

# --- 仓根与 git（只读）-------------------------------------------------------
if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
  echo "✗ 不在 git 仓内（⑵ 过期检测要读 commit 历史）⇒ 拒绝给结论" >&2
  exit 2
fi
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT" || exit 2
HEAD_SHA="$(git rev-parse HEAD 2>/dev/null || true)"
[ -n "$HEAD_SHA" ] || { echo "✗ 拿不到 HEAD（空仓？）⇒ 拒绝给结论" >&2; exit 2; }

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
  EXIST_SCAN=$n; EXIST_MISS=$miss
  [ "$n" -gt 0 ] || { echo "   ⚠ 两侧合计 0 篇 —— 正式面还没建？⇒ 不给结论（rc=2，2026-09-20 由 rc=3 统一过来）"; rc=2; }
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
  STALE_PAGES=$n; STALE_HITS=$stale
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
  COMMENT_PAGES=$n; COMMENT_MISS=$nocomment
  [ "$nocomment" -eq 0 ] || rc=1
  return $rc
}

# selftest_drift —— 第 ⑥ 格：**负控「过期 ≠ 未译」**（波10 序105 · 组4 §二.2 `W-22`）。
#   为什么必须造**真 git 历史**才钉得住：相② 的判据是 **commit 级**的
#   （`git diff <source_commit>...HEAD -- <源页>`）—— 不建临时仓就造不出「源页在 source_commit
#   之后动过」这个状态，那样只能靠注释里的君子协定（本仓那条「空转不许给绿」）。
#   夹具（全落在自建 `mktemp -d` 里，**不碰本仓**）：`docs/zh/s.md` 提交 → 造一枚**两侧都在**的译页
#   `docs/en/s.md`（`translation_of` + `source_commit=<那次提交>`）提交 → 再动一次 `docs/zh/s.md`。
#   ⇒ 现跑本脚本：该件**必须**落 `stale 过期 1`，而 `exist 未译` **必须**仍是 `0`。
selftest_drift() {
  local d out rcv em st
  d="$(mktemp -d "${TMPDIR:-/tmp}/drift-negctl.XXXXXX")" || return 1
  (
    cd "$d" || exit 1
    git init -q . || exit 1
    git config user.email negctl@local && git config user.name negctl
    mkdir -p docs/zh docs/en || exit 1
    printf -- '---\ntitle: s\n---\nsrc v1\n' > docs/zh/s.md
    git add -A && git commit -qm 'src v1'
    sc="$(git rev-parse HEAD)"
    printf -- '---\ntranslation_of: docs/zh/s.md\nsource_commit: %s\n---\n<!-- 源文保留 -->\ntr\n' "$sc" > docs/en/s.md
    git add -A && git commit -qm 'tr'
    printf -- '---\ntitle: s\n---\nsrc v2\n' > docs/zh/s.md
    git add -A && git commit -qm 'src v2'
  ) >/dev/null 2>&1 || { rm -rf "$d"; return 1; }
  # 走 `--json`（机器面 · 键名全 ASCII）取数 —— 本机 `sed` 在 C 面 locale 下拿中文正则
  # 会报 `illegal byte sequence`（空白跑实测）⇒ 判据**不许**建在「中文正则跑不跑得动」上。
  out="$(cd "$d" && bash "$SELF_ABS" --json 2>/dev/null)"; rcv=$?
  em="$(printf '%s' "$out" | sed -n 's/.*"exist":\([0-9][0-9]*\).*/\1/p' | head -1)"
  st="$(printf '%s' "$out" | sed -n 's/.*"stale":\([0-9][0-9]*\).*/\1/p' | head -1)"
  rm -rf "$d"
  if [ "${em:-x}" = "0" ] && [ "${st:-x}" = "1" ] && [ "${rcv}" = "2" ]; then
    echo "  ✓ 负控：过期件只落「stale 过期」（exist 未译 0 · stale 过期 1 · rc=2）"
    return 0
  fi
  echo "  ✗ 负控：过期件的落格不对（exist 未译=${em:-取不到} · stale 过期=${st:-取不到} · rc=${rcv}）"
  return 1
}

selftest() {
  echo '── check-i18n-drift.sh 自证（6 条 · 第 ⑥ 格建**自建** 临时仓造夹具；不碰本仓 git 写）'
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
  # ★ 第 ⑥ 格（波10 序105 · `W-22` 负控）：**过期 ≠ 未译** —— 过期件不许落「未译」格。
  if selftest_drift; then ok=$((ok+1)); fi
  if [ "$ok" -eq 6 ]; then echo "  自证：通过 ✓（6 条）"; return 0; fi
  echo "  自证：**失败** ✗（$ok/6）"; return 1
}

[ "$SELFTEST" = "1" ] && { selftest; exit $?; }

# --json：人面全部转 stderr，stdout 只留一行 JSON（`3` = 原始 stdout）
if [ "$JSON" = "1" ]; then exec 3>&1 1>&2; fi

# --- 跑相（各相独立退码；总码取最严：2 > 1 > 0；任一无结论 ⇒ 2（2026-09-20 由 3 统一））-----
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

# --- 汇总：漂移条数 + 判定档两格 + 三计数分开 -------------------------------------------------
# ★ 2026-09-24 波10 序105（组4 §二.2 `W-22` · 研-影:134）：本门今天「恒 BLOCKED」的机理是
#   `rc=2` 把**两种不同的真值**（有漂移 `FAIL` vs 判不了 `BLOCKED`）并成同一个形状 ⇒ 机器面
#   既拿不到「漂几条」、也分不出是哪种 ⇒ 那叫**空转**，不叫**能判**。
#   本段**只加机器读面**：判定档（`exist` / `stale` 两格）＋ 漂移条数 ＋ 三计数（**分开**）。
#   `rc` 语义**一字不改**（`2` 仍含两义）—— 那条语义错位已在 `precommit-gates.sh:1736` 登记待拍，
#   本波**不代拍**（`W-22` 自己写的是「先……再谈竞次」）。
EXIST_TIER="$(tier_of "$EXIST_SCAN" "$EXIST_MISS" 1)"
STALE_TIER="$(tier_of "$STALE_PAGES" "$STALE_HITS" 0)"
COMMENT_TIER="$(tier_of "$COMMENT_PAGES" "$COMMENT_MISS" 0)"
[ "$COMMENT_TIER" = "FAIL" ] && COMMENT_TIER="WARN"   # 相③ 先非阻塞（rc=1 ⇒ 告警，不是失败）
DRIFT=$((EXIST_MISS + STALE_HITS))
OVERALL=PASS
case "$EXIST_TIER/$STALE_TIER" in
  *BLOCKED*) OVERALL=BLOCKED ;;
  *FAIL*)    OVERALL=FAIL ;;
esac
if [ "$JSON" = "1" ]; then
  # 机器面：stdout **只**这一行（人面已在上面被 `exec 3>&1 1>&2` 转到 stderr）。
  printf '{"schema":"zerg/i18n-drift/v1","drift":%d,"verdict":"%s","tiers":{"exist":"%s","stale":"%s","comment":"%s"},"counts":{"exist":%d,"stale":%d,"comment":%d},"rc":%d}\n' \
    "$DRIFT" "$OVERALL" "$EXIST_TIER" "$STALE_TIER" "$COMMENT_TIER" \
    "$EXIST_MISS" "$STALE_HITS" "$COMMENT_MISS" "$TOTAL" >&3
fi
echo
echo '── 汇总（机器读面 · 波10 序105 `W-22`）'
echo "漂移条数：${DRIFT}（= exist 未译 ${EXIST_MISS} + stale 过期 ${STALE_HITS}）"
echo "判定档：${OVERALL}（两格 = exist=${EXIST_TIER} · stale=${STALE_TIER}）"
echo "计数（三格分开）：exist 未译 ${EXIST_MISS} · stale 过期 ${STALE_HITS} · comment 缺注释 ${COMMENT_MISS}（${COMMENT_TIER}）"
echo
echo "退码：exist=$RC_EXIST stale=$RC_STALE comment=$RC_COMMENT ⇒ 总码=$TOTAL"
exit "$TOTAL"

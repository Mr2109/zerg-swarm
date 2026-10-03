#!/usr/bin/env bash
# decl-label.sh — 服务 label 的**单一真源读取器**（GAP-20260927-107 步④ · 2026-10-03）
#
# 为什么存在：全仓曾有 5 份硬编码 label 名单（com.zerg.core 等）各自漂移；本件让
#   要 label 的脚本**现读声明件**，✗ 不许在别处再写第二份 label 字面。
#
# 真源件 = deploy/服务声明.tsv（**唯一**）：该件自己逐字写着「本件是『现值 vs 声明』
#   差集判定的**声明侧唯一真源**」，门⑧ check-service-declaration.py 的 parse_decl 与
#   本件解析同一份、同一列口径（列：kind name 声明件 程序 进程特征 归属 处置）。
#
# 用法：
#   scripts/svc/decl-label.sh <查询> [--decl <声明件>] [--all]
#     <查询> 精确匹配第 2 列 name 或 第 5 列 进程特征；命中**恰好 1 条** ⇒ 印第 2 列 label（rc=0）。
#     命中 0 条或多条 ⇒ 不印、rc=1（**禁猜**：多义不选、缺件不补）。
#     --all  列出全部命中（每行一个 label）；0 条 ⇒ rc=1。
#   -h|--help
#
# 环境：
#   ZERG_SVCDECL_DECL  覆盖声明件路径（与门⑧/门㉒ 同名同义，默认 <仓根>/deploy/服务声明.tsv）
#
# 退码：0 命中恰好一条（或 --all 至少一条）· 1 命中数不为 1 · 2 用法错 / 声明件不存在
# ★ 只读：本件不做任何写/停/杀（与门㉒ 只读边界一致）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DECL="${ZERG_SVCDECL_DECL:-$REPO_ROOT/deploy/服务声明.tsv}"
QUERY=""
ALL=0

usage() {
  cat >&2 <<'USAGE'
用法: decl-label.sh <查询> [--decl <声明件>] [--all]
  <查询> 精确匹配声明件第 2 列(name) 或 第 5 列(进程特征)，印出第 2 列 label。
  --all  列出全部命中（默认要求恰好 1 条）。
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --decl)  [ $# -ge 2 ] || { usage; exit 2; }; DECL="$2"; shift 2 ;;
    --all)   ALL=1; shift ;;
    -h|--help) usage; exit 0 ;;
    --)      shift; break ;;
    -*)      usage; exit 2 ;;
    *)       QUERY="$1"; shift ;;
  esac
done

[ -n "$QUERY" ] || { usage; exit 2; }
[ -f "$DECL" ] || { echo "decl-label: 声明件不存在：$DECL" >&2; exit 2; }

# 逐行解析（与门⑧ parse_decl 同列口径）：跳空行/注释行/表头行；只认 kind=launchd。
#   IFS=$'\t' ⇒ 分列；name=col2 · proc=col5。**不做子串匹配**（精确相等）。
HITS=0
OUT=""
while IFS=$'\t' read -r _kind _name _decl _prog _proc _owner _disp _rest; do
  [ "$_kind" = "launchd" ] || continue
  if [ "$_name" = "$QUERY" ] || [ "$_proc" = "$QUERY" ]; then
    OUT="${OUT}${_name}"$'\n'
    HITS=$((HITS + 1))
  fi
done < "$DECL"

if [ "$ALL" = "1" ]; then
  [ "$HITS" -ge 1 ] || { echo "decl-label: 零命中：${QUERY}（声明件 ${DECL}）" >&2; exit 1; }
  printf '%s' "$OUT"
  exit 0
fi

if [ "$HITS" -eq 1 ]; then
  printf '%s\n' "${OUT%$'\n'}"
  exit 0
fi

echo "decl-label: 命中 ${HITS} 条（要恰好 1 条）——查询 '${QUERY}'（声明件 ${DECL}）" >&2
exit 1

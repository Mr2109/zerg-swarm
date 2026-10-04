#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# GAP-20260927-188 治法候选（wave16 / w8）—— 原 `scripts/gates/check-tool-version-sync.sh`
#   ＋ 三面收口（缺口逐字要求）：
#     ① 扫描面 = `tools/*.md` 的 **fs glob(P)** ∪ 台账键（盘上有而台账无 ⇒ **出声**，不再天然免扫）。
#     ② 台账值缺失 **不许静默跳过** ⇒ 落「判不了」档并出声（退码 2）。
#     ③ 补 `--self-test` 与**成对负控**（一致 ⇒ 0 · 抬头不符 ⇒ 1 · 台账无值 ⇒ 2 · 盘上多件 ⇒ 报告）。
#   ✗ 未做（越界/待拍）：④ 「全仓普查哪些门脚本没有自检面」—— 那是**新件**、其门面在
#     `precommit-gates.sh`（本片边界件），须父代理接线。
#
#   硬约束遵守：本候选只落**仓外**（wave16/w8/候件）；未改仓内任何件、未跑整套门、未碰 `bin/`。
#   用法: check-tool-version-sync.sh [--quiet] [--self-test]
#   退出码: 0 全一致 / 1 有不一致（逐条打印）/ 2 不判（台账值缺失 · 未知参数 · 自检失败）
# ─────────────────────────────────────────────────────────────────────────────
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

SELFTEST=0
QUIET=0
for a in "$@"; do
  case "$a" in
    --self-test) SELFTEST=1 ;;
    --quiet)     QUIET=1 ;;
    *) echo "FATAL: 未知参数 $a"; exit 2 ;;
  esac
done

# check_root <root> <quiet> ⇒ 打印报告，返 rc（0 一致 / 1 不一致 / 2 判不了）
check_root() {
  local root="$1" quiet="$2"
  [ -f "$root/tools/versions.json" ] || { echo "FATAL: 找不到 tools/versions.json"; return 2; }
  python3 - "$root" "$quiet" <<'PY'
import io, json, os, re, sys, glob
root = sys.argv[1]; quiet = sys.argv[2] == "1"
d = json.load(io.open(os.path.join(root, "tools", "versions.json"), encoding="utf-8"))
tools = d.get("tools", d) if isinstance(d, dict) else d
def ver_of(name):
    if isinstance(tools, dict):
        x = tools.get(name)
        if x is None:
            return None
        return x if isinstance(x, str) else (x.get("version") or x.get("ver"))
    for x in tools:
        if isinstance(x, dict) and x.get("name") == name:
            return x.get("version")
    return None
keys = sorted(tools) if isinstance(tools, dict) else [t.get("name") for t in tools if isinstance(t, dict)]
keys = [k for k in keys if k]
pat_head = re.compile(r'\*{0,2}版本\*{0,2}\s*[：:]\s*\**\s*v(\d+\.\d+\.\d+)')
# ① 扫描面 = fs glob(P) ∪ 台账键 —— 关系式，不是「台账的键自己」
md_names = sorted(os.path.splitext(os.path.basename(p))[0]
                  for p in glob.glob(os.path.join(root, "tools", "*.md")))
scan = sorted(set(keys) | set(md_names))
bad, nohead, ok, unledgered, novalue = [], [], 0, [], []
for name in scan:
    if name not in keys:
        unledgered.append(name)          # 盘上有而台账无 ⇒ 出声（报告档，不改退码）
        continue
    led = ver_of(name)
    if led is None:
        novalue.append(name)             # ② 台账值缺失 ⇒ 判不了出声（→ rc=2，不静默跳过）
        continue
    md = os.path.join(root, "tools", "%s.md" % name)
    if not os.path.exists(md):
        bad.append((name, led, "(缺文档 tools/%s.md)" % name)); continue
    head = None
    for line in io.open(md, encoding="utf-8").read().splitlines()[:20]:
        m = pat_head.search(line)
        if m:
            head = "v" + m.group(1); break
    if head is None:
        nohead.append((name, led)); continue
    if head != (led if led.startswith("v") else "v" + led):
        bad.append((name, led, head))
    else:
        ok += 1
print("  台账键 = %d ｜ 盘上 tools/*.md = %d ｜ 扫描面(并) = %d ｜ 抬头一致 = %d ｜ 抬头不一致 = %d ｜ 缺抬头 = %d"
      % (len(keys), len(md_names), len(scan), ok, len(bad), len(nohead)))
if bad and not quiet:
    print("  ── 不一致（台账为准；改抬头前先由 Mr2109 定版本值，勿自动顺延）──")
    for n, a, b in bad:
        print("    %-22s 台账=%-8s 抬头=%s" % (n, a, b))
if nohead and not quiet:
    print("  ── 缺抬头版本行（需补，版本值待定）──")
    for n, a in nohead:
        print("    %-22s 台账=%s" % (n, a))
if unledgered and not quiet:
    print("  ── 报告·盘上有而台账无（tools/*.md 在盘、versions.json 无此键 ⇒ 待补键或删件）──")
    for n in unledgered:
        print("    %-22s" % n)
if novalue and not quiet:
    print("  ── 判不了·台账值缺失（键在册但无 version 值 ⇒ 不给结论）──")
    for n in novalue:
        print("    %-22s" % n)
sys.exit(2 if novalue else (1 if (bad or nohead) else 0))
PY
}

if [ "$SELFTEST" -eq 1 ]; then
  echo "── 自检（合成夹具 · 不碰真目标）──"
  TMPD="$(mktemp -d "${TMPDIR:-/tmp}/tv-sync-selftest-XXXXXX")"
  trap 'rm -rf "$TMPD"' EXIT
  fails=0
  mkfix() {  # mkfix <dir> <versions.json 内容> [tool md 内容]
    local d="$1" vj="$2" md="${3:-}"
    mkdir -p "$d/tools"
    printf '%s' "$vj" > "$d/tools/versions.json"
    [ -n "$md" ] && printf '%s' "$md" > "$d/tools/alpha.md"
  }
  run_case() {  # run_case <名> <期望rc> <dir>
    local name="$1" want="$2" d="$3" out rc
    out="$(check_root "$d" 0 2>&1)"; rc=$?
    if [ "$rc" = "$want" ]; then echo "  ✓ ${name}（rc=${rc}）"
    else echo "  ✗ ${name}（rc=${rc} · 期望 ${want}）"; echo "$out" | sed 's/^/      /'; fails=$((fails+1)); fi
  }
  # 正控：抬头一致 ⇒ 0
  mkfix "$TMPD/ok" '{"tools":{"alpha":"v1.2.3"}}' $'# alpha\n版本：v1.2.3\n'
  run_case "正控·抬头一致 ⇒ 0" 0 "$TMPD/ok"
  # 负控：抬头不符 ⇒ 1
  mkfix "$TMPD/mis" '{"tools":{"alpha":"v1.2.3"}}' $'# alpha\n版本：v1.2.4\n'
  run_case "负控·抬头不符 ⇒ 1" 1 "$TMPD/mis"
  # 负控：台账值缺失（键在册无值）⇒ 2（旧件此处静默跳过 ⇒ 假绿）
  mkfix "$TMPD/nov" '{"tools":{"alpha":{}}}' $'# alpha\n版本：v1.2.3\n'
  run_case "负控·台账值缺失 ⇒ 2（旧件静默跳过）" 2 "$TMPD/nov"
  # 报告：盘上有而台账无 ⇒ 出声但不改退码（一致性仍 0）
  mkfix "$TMPD/unl" '{"tools":{"alpha":"v1.2.3"}}' $'# alpha\n版本：v1.2.3\n'
  printf '# beta\n版本：v0.0.1\n' > "$TMPD/unl/tools/beta.md"
  out="$(check_root "$TMPD/unl" 0 2>&1)"; rc=$?
  if [ "$rc" = "0" ] && printf '%s' "$out" | grep -q '盘上有而台账无'; then
    echo "  ✓ 报告·盘上多件出声且退码不受影响（rc=0）"
  else echo "  ✗ 报告·盘上多件（rc=${rc}）"; fails=$((fails+1)); fi
  echo "自检结论: $([ "$fails" -eq 0 ] && echo '全过' || echo "**$fails 格失败**")"
  [ "$fails" -eq 0 ] || exit 2
  exit 0
fi

rc=0
check_root "$ROOT" "$QUIET" || rc=$?
[ "$rc" -eq 0 ] && echo "  ✓ 三处版本一致（台账 / 工具文档抬头）"
exit "$rc"

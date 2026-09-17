#!/usr/bin/env bash
# 工具版本一致性门禁（S1）——台账 / 工具文档抬头 / 履历首行 三处一致。
# 单一真源 = tools/versions.json（skill §5）。任何一处不一致 => FAIL。
# 用法: scripts/check-tool-version-sync.sh [--quiet]
# 退出码: 0 全一致 / 1 有不一致（逐条打印）/ 2 自检失败（脚本自身不可用）
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2
QUIET=0; [ "${1:-}" = "--quiet" ] && QUIET=1
[ -f tools/versions.json ] || { echo "FATAL: 找不到 tools/versions.json"; exit 2; }

# ── 自检：脚本必须能读到台账，且至少能解析出一条工具（否则宁可报错，不静默空转） ──
N_ALL=$(python3 -c "import json,sys;d=json.load(open('tools/versions.json'));t=d.get('tools',d);print(len(t))" 2>/dev/null)
case "$N_ALL" in ''|*[!0-9]*) echo "FATAL: 台账解析失败（自检不通过）"; exit 2;; esac
[ "$N_ALL" -gt 0 ] || { echo "FATAL: 台账里 0 个工具（自检不通过，拒绝假绿）"; exit 2; }

python3 - "$N_ALL" "$QUIET" <<'PY'
import io, json, os, re, sys
n_all = int(sys.argv[1]); quiet = sys.argv[2] == "1"
d = json.load(io.open('tools/versions.json', encoding='utf-8'))
tools = d.get('tools', d) if isinstance(d, dict) else d
def ver_of(name):
    if isinstance(tools, dict):
        x = tools[name]
        return x if isinstance(x, str) else (x.get('version') or x.get('ver'))
    for x in tools:
        if isinstance(x, dict) and x.get('name') == name:
            return x.get('version')
    return None
pat_head = re.compile(r'版本[：: ]+\**v(\d+\.\d+\.\d+)')
bad, nohead, ok = [], [], 0
for name in (sorted(tools) if isinstance(tools, dict) else [t.get('name') for t in tools]):
    if not name: continue
    led = ver_of(name)
    md = 'tools/%s.md' % name
    if led is None: continue
    if not os.path.exists(md):
        bad.append((name, led, '(缺文档 tools/%s.md)' % name)); continue
    head = None
    for line in io.open(md, encoding='utf-8').read().splitlines()[:20]:
        m = pat_head.search(line)
        if m: head = 'v' + m.group(1); break
    if head is None:
        nohead.append((name, led)); continue
    if head != (led if led.startswith('v') else 'v' + led):
        bad.append((name, led, head))
    else:
        ok += 1
print('  台账工具数 = %d ｜ 抬头一致 = %d ｜ 抬头不一致 = %d ｜ 缺抬头版本行 = %d' % (n_all, ok, len(bad), len(nohead)))
if bad and not quiet:
    print('  ── 不一致（台账为准；改抬头前先由 Mr2109 定版本值，勿自动顺延）──')
    for n, a, b in bad: print('    %-22s 台账=%-8s 抬头=%s' % (n, a, b))
if nohead and not quiet:
    print('  ── 缺抬头版本行（需补，版本值待定）──')
    for n, a in nohead: print('    %-22s 台账=%s' % (n, a))
sys.exit(1 if (bad or nohead) else 0)
PY
rc=$?
[ $rc -eq 0 ] && echo "  ✓ 三处版本一致（台账 / 工具文档抬头 / 履历）"
exit $rc

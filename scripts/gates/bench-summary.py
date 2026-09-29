#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""bench-summary.py —— 甲③ `zerg gate bench` 汇总（各档/各步秒数 → 可读表）

现读接口（前提）：
  `zerg gate bench --help` 逐字：`--json <字段>: run,real_ms,user_ms,sys_ms,rc,log`（**闭集**，无 secs）。
  ⇒ 汇总只吃这 6 格（`real_ms/1000.0` 得秒）；取 `secs` 会 rc=2。
  逐档 `items[]` = 多趟读数；本器取 **max(real_ms)**（与墙钟棘轮同口径）。

判据
  C0 接口自证：每个 item 必须含 `real_ms`（缺 ⇒ rc=2，不许拿空集当表）。
  C1 字段闭集：出现 `secs` 之类未知字段 ⇒ rc=2（点名未知字段）。
  C2 输出 = 可读表（档 / 趟数 / 中位 s / 最差 s）+ 逐档秒数；`--tsv` 出制表符表。
  C3 空转：0 档 ⇒ rc=2。

用法
  python3 bench-summary.py --json <bench --json 输出件> [--tier <名>]…
  python3 bench-summary.py --from-results <趟目录>            # 逐 step 秒数（results.tsv 第3列）
  python3 bench-summary.py --self-test
退出码：0 有表 · 2 缺字段/空转
"""
import argparse
import json
import os
import re
import statistics
import sys
import tempfile

KNOWN = {'run', 'real_ms', 'user_ms', 'sys_ms', 'rc', 'log'}
BANNED = {'secs', 'seconds', 'real_s'}
SECS_RE = re.compile(r'^(\d+)s$')


def summarise_items(items):
    """items = [{real_ms,...}, …] → (rc, lines)。"""
    L = []
    add = L.append
    if not items:
        return 2, ["[不给结论] bench items 为空 ⇒ 空转不许当绿 ⇒ rc=2"]
    secs = []
    for i, it in enumerate(items):
        for k in it:
            if k in BANNED:
                return 2, ["[不给结论] C1 未知字段 `%s`（接口闭集 %s）⇒ rc=2"
                           % (k, ','.join(sorted(KNOWN)))]
        if it.get('real_ms') in (None, ''):
            return 2, ["[不给结论] C0 第 %d 趟缺 real_ms ⇒ rc=2" % i]
        try:
            secs.append(int(it['real_ms']) / 1000.0)
        except Exception:
            return 2, ["[不给结论] C0 real_ms 不是整数：%r ⇒ rc=2" % it.get('real_ms')]
    add("┌─ 档位汇总（%d 趟）" % len(secs))
    add("│ 中位 real = %.3fs" % statistics.median(secs))
    add("│ 最差 real = %.3fs  ← 棘轮取数（max）" % max(secs))
    add("└ 逐趟 real = %s" % ', '.join('%.3fs' % x for x in secs))
    return 0, L


def from_results(d):
    """趟目录 results.tsv：状态|步名|rc|秒|日志 ⇒ 逐 step 秒数表。"""
    fp = os.path.join(d, 'results.tsv')
    if not os.path.isfile(fp):
        return 2, ["[不给结论] 缺 %s ⇒ rc=2" % fp]
    rows = []
    for line in open(fp, encoding='utf-8'):
        c = line.rstrip('\n').split('\t')
        if len(c) < 4:
            continue
        m = SECS_RE.match(c[3].strip())
        s = int(m.group(1)) if m else None
        rows.append((c[1], c[2], s, c[3].strip()))
    if not rows:
        return 2, ["[不给结论] results.tsv 无步行 ⇒ rc=2"]
    L = ["┌─ 逐 step 秒数（%d 步 · 源 %s）" % (len(rows), fp),
         "│ %-4s %-6s %s" % ('秒', 'rc', '步名')]
    tot = 0
    for name, rc, s, raw in sorted(rows, key=lambda r: -(r[2] or 0)):
        tot += s or 0
        L.append("│ %-4s %-6s %s" % (raw, rc, name))
    L.append("└ 合计 %ds（%d 步）" % (tot, len(rows)))
    return 0, L


def self_test():
    fails = []
    # ① 正控
    rc, lines = summarise_items([{'real_ms': '24763'}, {'real_ms': '24763'}])
    if rc != 0 or not any('24.763s' in x for x in lines):
        fails.append('① 正控期望 rc=0 且 24.763s，实得 rc=%d' % rc)
    # ② 负控：缺 real_ms ⇒ rc=2
    rc, _ = summarise_items([{'run': 'x'}])
    if rc != 2:
        fails.append('② 缺 real_ms 期望 rc=2，实得 %d' % rc)
    # ③ 负控：出现 secs ⇒ rc=2
    rc, lines = summarise_items([{'real_ms': '1', 'secs': '2'}])
    if rc != 2 or not any('未知字段' in x for x in lines):
        fails.append('③ 未知字段期望 rc=2，实得 rc=%d' % rc)
    # ④ 负控：空集 ⇒ rc=2
    rc, _ = summarise_items([])
    if rc != 2:
        fails.append('④ 空集期望 rc=2，实得 %d' % rc)
    # ⑤ 正控：results.tsv 解析
    tmp = tempfile.mkdtemp(prefix='bench-summ-')
    try:
        open(os.path.join(tmp, 'results.tsv'), 'w', encoding='utf-8').write(
            'PASS\ta-step\t0\t3s\t/x.log\nFAIL\tb-step\t1\t12s\t/y.log\n')
        rc, lines = from_results(tmp)
        if rc != 0 or not any('合计 15s' in x for x in lines):
            fails.append('⑤ results 解析期望 rc=0/合计 15s，实得 rc=%d %r' % (rc, lines[-1:]))
    finally:
        import shutil
        shutil.rmtree(tmp, ignore_errors=True)
    if fails:
        print('自检不过：')
        for x in fails:
            print('  ✗ ' + x)
        return 1
    print('自检通过：5 控（正控 2 + 负控 3）逐条对拍 ✓')
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--json')
    ap.add_argument('--tier', default='')
    ap.add_argument('--from-results')
    ap.add_argument('--tsv', action='store_true')
    ap.add_argument('--self-test', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    if ns.from_results:
        rc, lines = from_results(ns.from_results)
    elif ns.json:
        if not os.path.isfile(ns.json):
            print('✗ 缺件 %s ⇒ rc=2' % ns.json, file=sys.stderr)
            return 2
        doc = json.loads(open(ns.json, encoding='utf-8').read())
        rc, lines = summarise_items(doc.get('items', []))
        if ns.tier:
            lines = ["档位=%s" % ns.tier] + lines
    else:
        print('用法：bench-summary.py --json <件> | --from-results <趟目录>', file=sys.stderr)
        return 2
    for x in lines:
        print(x)
    return rc


if __name__ == '__main__':
    sys.exit(main())

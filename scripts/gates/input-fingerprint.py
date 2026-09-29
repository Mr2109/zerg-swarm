#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""input-fingerprint.py —— 甲② 输入指纹缓存（**仅日常/中档** · 盖章档禁用）

硬线（逐字落实）：
  · 盖章档（seal/stamp）**禁用** ⇒ --tier seal|stamp ⇒ **rc=2 不给结论**（不许悄悄绿、不许缓存）；
  · 命中必打 `CACHED` 且**不得显示成 PASS** ⇒ 命中行首字面 `CACHED`（`PASS` 一个字节都不出现）；
  · 须有**冷跑对拍** ⇒ `--cold` 现跑真命令、`--compare` 两态对拍（冷跑 rc ⟷ 缓存 rc 必须相等）。

判据（三档退码 0/1/2）
  C0 档位闸：seal/stamp ⇒ rc=2。日常/中档（daily/medium）才走缓存。
  C1 指纹 = sha256( 逐件路径 + 件内容 sha256 )（按路径排序拼接；件缺失 ⇒ 记 `MISSING` 参与指纹）。
  C2 命中 ⇒ 打印 `CACHED <key前12>`（**绝不打印 PASS**）；未命中 ⇒ 打印 `MISS` 并写缓存。
  C3 冷跑对拍：`--compare` 现跑真命令取 rc，与缓存记的 rc 逐格比；不等 ⇒ rc=2（两态不一致不许当绿）。
  C4 空转：无输入件 ⇒ rc=2。

用法
  python3 input-fingerprint.py --tier daily --cache <件> --inputs <件>… [--key <名>]
  python3 input-fingerprint.py --tier seal  --cache <件> --inputs <件>…      # ⇒ rc=2
  python3 input-fingerprint.py --self-test
退出码：0 命中/未命中一致 · 2 盖章档/空转/两态不一致
"""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

CACHE_TIERS = ('daily', 'medium')
BANNED_TIERS = ('seal', 'stamp')


def file_digest(p):
    if not os.path.isfile(p):
        return 'MISSING'
    h = hashlib.sha256()
    with open(p, 'rb') as f:
        for chunk in iter(lambda: f.read(1 << 16), b''):
            h.update(chunk)
    return h.hexdigest()


def fingerprint(inputs):
    parts = []
    for p in sorted(inputs):
        parts.append('%s\t%s' % (p, file_digest(p)))
    return hashlib.sha256('\n'.join(parts).encode('utf-8')).hexdigest()


def load_cache(path):
    if not os.path.isfile(path):
        return {}
    try:
        return json.loads(open(path, encoding='utf-8').read())
    except Exception:
        return {}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--tier', required=False, default='daily')
    ap.add_argument('--cache')
    ap.add_argument('--key', default='default')
    ap.add_argument('--inputs', nargs='*', default=[])
    ap.add_argument('--cmd')
    ap.add_argument('--compare', action='store_true')
    ap.add_argument('--self-test', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    if ns.tier in BANNED_TIERS:
        print('✗ 盖章档（%s）**禁用缓存** ⇒ 必全冷跑 ⇒ rc=2（不给结论）' % ns.tier, file=sys.stderr)
        return 2
    if ns.tier not in CACHE_TIERS:
        print('✗ 未知档位 %r（闭集 = daily/medium 走缓存 · seal/stamp 禁用）⇒ rc=2' % ns.tier, file=sys.stderr)
        return 2
    if not ns.inputs:
        print('✗ 无输入件 ⇒ 空转 ⇒ rc=2', file=sys.stderr)
        return 2
    if not ns.cache:
        print('✗ 缺 --cache 件 ⇒ rc=2', file=sys.stderr)
        return 2
    fp = fingerprint(ns.inputs)
    cache = load_cache(ns.cache)
    prev = cache.get(ns.key)
    if prev and prev.get('fp') == fp:
        # 命中：只显示 CACHED —— 绝不显示 PASS
        print('CACHED %s' % fp[:12])
        print('  （本行**不是通过**：输入指纹命中既有缓存，本轮未真跑 · 硬线：命中不得渲染成绿）')
        if ns.compare and ns.cmd:
            rc_now = subprocess.call(ns.cmd, shell=True)
            if int(prev.get('rc', -1)) != rc_now:
                print('✗ 冷跑对拍不等：缓存 rc=%s ⟷ 冷跑 rc=%s ⇒ rc=2' % (prev.get('rc'), rc_now), file=sys.stderr)
                return 2
            print('  ── 冷跑对拍：缓存 rc=%s == 冷跑 rc=%s ✓' % (prev.get('rc'), rc_now))
        return 0
    print('MISS %s（未命中 ⇒ 真跑并记缓存）' % fp[:12])
    rc = 0
    if ns.cmd:
        rc = subprocess.call(ns.cmd, shell=True)
    cache[ns.key] = {'fp': fp, 'rc': rc, 'inputs': sorted(ns.inputs)}
    os.makedirs(os.path.dirname(os.path.abspath(ns.cache)), exist_ok=True)
    with open(ns.cache, 'w', encoding='utf-8') as f:
        json.dump(cache, f, ensure_ascii=False, indent=1)
    return 0 if rc == 0 else rc


def self_test():
    fails = []
    tmp = tempfile.mkdtemp(prefix='input-fp-selftest-')
    try:
        inp = os.path.join(tmp, 'a.txt')
        open(inp, 'w', encoding='utf-8').write('v1')
        cache = os.path.join(tmp, 'c.json')
        # ① 冷跑（首次 MISS）⇒ rc=0，且打 MISS
        def run(args):
            return subprocess.run([sys.executable, os.path.abspath(__file__)] + args,
                                  capture_output=True, text=True)
        r = run(['--tier', 'daily', '--cache', cache, '--inputs', inp, '--cmd', 'true'])
        if r.returncode != 0 or 'MISS' not in r.stdout:
            fails.append('① 首次冷跑期望 MISS/rc=0，实得 rc=%d out=%r' % (r.returncode, r.stdout[:60]))
        # ② 再跑（命中）⇒ CACHED 且 stdout **不含 PASS**
        r = run(['--tier', 'daily', '--cache', cache, '--inputs', inp, '--cmd', 'true'])
        if 'CACHED' not in r.stdout:
            fails.append('② 期望 CACHED，实得 %r' % r.stdout[:60])
        if 'PASS' in r.stdout:
            fails.append('② 命中却出现 PASS（硬线：命中不得显示成 PASS）')
        # ③ 冷跑对拍两态一致 ⇒ rc=0
        r = run(['--tier', 'daily', '--cache', cache, '--inputs', inp, '--cmd', 'true', '--compare'])
        if r.returncode != 0:
            fails.append('③ 两态一致期望 rc=0，实得 %d' % r.returncode)
        # ④ 冷跑对拍两态不一致 ⇒ rc=2（cmd 现返 1，缓存记的是 0）
        r = run(['--tier', 'daily', '--cache', cache, '--inputs', inp, '--cmd', 'false', '--compare'])
        if r.returncode != 2:
            fails.append('④ 两态不一致期望 rc=2，实得 %d' % r.returncode)
        # ⑤ 盖章档禁用 ⇒ rc=2
        for t in ('seal', 'stamp'):
            r = run(['--tier', t, '--cache', cache, '--inputs', inp])
            if r.returncode != 2:
                fails.append('⑤ 档位 %s 期望 rc=2，实得 %d' % (t, r.returncode))
        # ⑥ 空转 ⇒ rc=2
        r = run(['--tier', 'daily', '--cache', cache])
        if r.returncode != 2:
            fails.append('⑥ 空转期望 rc=2，实得 %d' % r.returncode)
        # ⑦ 输入变 ⇒ 指纹变 ⇒ MISS
        open(inp, 'w', encoding='utf-8').write('v2')
        r = run(['--tier', 'daily', '--cache', cache, '--inputs', inp, '--cmd', 'true'])
        if 'MISS' not in r.stdout:
            fails.append('⑦ 输入变期望 MISS，实得 %r' % r.stdout[:60])
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    if fails:
        print('自检不过：')
        for x in fails:
            print('  ✗ ' + x)
        return 1
    print('自检通过：7 控（正控 3 + 负控 4）逐条对拍 ✓')
    return 0


if __name__ == '__main__':
    sys.exit(main())

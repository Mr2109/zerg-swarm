#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""affected-steps.py —— 甲①「受影响步选择」**只报告档**（本轮不真跳步）

现读硬事实（前提，先现读再设计）：
  run_step 真跑那一行 = `( cd "${dir}" && bash -c "${cmd}" )`（precommit-gates.sh:567/569）
  ⇒ 子步**拿不到任何 run 作用域 env**；本判据**不依赖任何 env 通道** —— 它只在
     **本轮替代路径**里被调用（外层先把「本趟该跑哪几步」算出来再喂给 `--step … --only-step`），
     故与 run_step 无 env 耦合。本轮**只打印**「本趟该跑哪几步」，一个步都不跳。

判据（三档退码 0/1/2）——「怎么机检『没少选』」：
  C0 双向计数（**核心**）：声明件里**逐字命名**的步数 D 必须 == 步骤表（`--list` 自报）步数 N，
     且**命名集合逐字相等**（D==N 但集合不等 ⇒ 仍 rc=2）。**D≠N ⇒ rc=2 不给结论**
     （既有「少声明一步」= 静默漏选 ⇒ 当场拒，绝不拿漏读的声明当基线）。★ 这一格就是「没少选」。
  C1 解析自证：声明件 `steps` 每个键在表里恰好命中一次（没有幻影步、没有重名）；解析出的条目数
     必须 == 声明的键数（漏读即 rc=2）。★ 这一格是「没少读」。
  C2 受影响判定：受影响 = 该步声明的 glob 里**至少一个**命中本趟改动件（`--changed` 逐行）。
     非受影响步逐条打印（可见 —— 「跳过必留痕」，绝不显示成 PASS）。
  C3 空转：本趟无改动件 ⇒ 打印 `受影响 0 步`（可与「没跑」分辨：有 D/N 读数）。

本轮口径：**只报告**（stdout 只打印「本趟该跑哪几步」，不调用任何真跑）。

用法
  python3 affected-steps.py --list <--list 输出件> --decl <声明件.json> --changed <改动件清单>
  python3 affected-steps.py --self-test
退出码：0 全绿 · 1 ——（本轮不产生）· 2 不给结论（少声明/漏读/非报告档）
"""
import argparse
import fnmatch
import json
import os
import re
import sys
import tempfile

STEP_LINE_RE = re.compile(r'^\s*(\S+)\s+(\S+)\s+(.*)$')


def parse_list(path):
    """--list 输出 → (步名列表, 表头/计数行)。只取步骤行（首列 scope + 次列 模式 + 名）。"""
    names = []
    with open(path, encoding='utf-8') as f:
        for line in f:
            if line.startswith('──') or line.startswith('共 ') or not line.strip():
                continue
            m = STEP_LINE_RE.match(line.rstrip('\n'))
            if not m:
                continue
            scope, mode, name = m.group(1), m.group(2), m.group(3)
            if scope in ('self',) or not re.match(r'^[a-z-]+$', scope):
                continue
            names.append(name)
    return names


def parse_decl(path):
    with open(path, encoding='utf-8') as f:
        return json.load(f)


def analyse(list_path, decl_path, changed_path):
    L = []
    add = L.append
    if not os.path.isfile(list_path):
        return 2, ["[不给结论] 步骤表件缺失：%s ⇒ rc=2" % list_path]
    if not os.path.isfile(decl_path):
        return 2, ["[不给结论] 声明件缺失：%s ⇒ rc=2" % decl_path]
    table = parse_list(list_path)
    N = len(table)
    decl = parse_decl(decl_path)
    steps = decl.get('steps', {})
    if not isinstance(steps, dict) or not steps:
        return 2, ["[不给结论] 声明件 `steps` 空/非映射 ⇒ rc=2（空转不许当绿）"]
    D = len(steps)
    add("步骤表（--list 自报）步数 N=%d · 声明命名步数 D=%d" % (N, D))
    # C0 双向计数 + 集合逐字相等
    tset, dset = set(table), set(steps.keys())
    if D != N:
        return 2, L + ["[不给结论] C0 未过：声明命名 %d 步 ≠ 步骤表 %d 步 ⇒ **有步未声明 = 静默漏选** ⇒ rc=2" % (D, N)]
    if dset != tset:
        miss = sorted(tset - dset)
        ghost = sorted(dset - tset)
        return 2, L + ["[不给结论] C0 未过：命名集合 ≠ 表集合（漏=%s · 幻影=%s）⇒ rc=2"
                       % (miss[:5], ghost[:5])]
    add("C0 双向计数过：D==N==%d 且命名集合逐字相等（**没少声明**）" % N)
    # C1 解析自证
    n_entries = 0
    for name, globs in steps.items():
        if not isinstance(globs, list):
            return 2, L + ["[不给结论] C1 未过：步「%s」的声明不是列表 ⇒ rc=2" % name]
        n_entries += len(globs)
    add("C1 解析自证过：声明条目 %d 条（键 %d 个 · 表命中 %d 次）" % (n_entries, D, N))
    changed = []
    if changed_path and os.path.isfile(changed_path):
        with open(changed_path, encoding='utf-8') as f:
            changed = [x.strip() for x in f if x.strip()]
    affected, unaffected = [], []
    for name in table:
        globs = steps[name]
        hit = [c for c in changed for g in globs if fnmatch.fnmatch(c, g)]
        (affected if hit else unaffected).append(name)
    add("本趟改动件 %d 件" % len(changed))
    add("── 报告（**只报告档** · 不真跳任何步）──")
    add("受影响步 %d 步（本趟该跑）：" % len(affected))
    for n in affected:
        add("   ▶ %s" % n)
    add("非受影响步 %d 步（本轮**不跳** · 逐条留痕，绝不显示成 PASS）：" % len(unaffected))
    if not affected:
        add("   受影响 0 步（本读数可分辨「没跑」：上一行有 D/N=%d/%d 与改动件计数）" % (D, N))
    return 0, L


def self_test():
    fails = []
    tmp = tempfile.mkdtemp(prefix='affected-steps-selftest-')
    try:
        lst = os.path.join(tmp, 'list.txt')
        open(lst, 'w', encoding='utf-8').write(
            '── 步骤清单（scope 模式 名称）──\n'
            '   go     rc     a-step\n'
            '   rust   empty  b-step\n'
            '   docs   tri    c-step\n'
            '共 3 步\n')
        chg = os.path.join(tmp, 'chg.txt')
        open(chg, 'w', encoding='utf-8').write('core/cmd/x.go\n')
        # ① 正控：声明齐 + 有命中 ⇒ rc=0 且第一步受影响
        d1 = os.path.join(tmp, 'd1.json')
        json.dump({'steps': {'a-step': ['core/**'], 'b-step': ['ui/**'], 'c-step': ['docs/**']}},
                  open(d1, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, d1, chg)
        if rc != 0:
            fails.append('① 正控期望 rc=0，实得 %d' % rc)
        elif not any('▶ a-step' in x for x in lines):
            fails.append('① 正控未点名受影响步')
        # ② 负控：少声明一步（D=2, N=3）⇒ rc=2
        d2 = os.path.join(tmp, 'd2.json')
        json.dump({'steps': {'a-step': ['core/**'], 'b-step': ['ui/**']}},
                  open(d2, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, d2, chg)
        if rc != 2:
            fails.append('② 少声明期望 rc=2，实得 %d' % rc)
        elif not any('静默漏选' in x for x in lines):
            fails.append('② 未点名「静默漏选」')
        # ③ 负控：D==N 但集合不等（改名）⇒ rc=2
        d3 = os.path.join(tmp, 'd3.json')
        json.dump({'steps': {'a-step': ['core/**'], 'b-step': ['ui/**'], 'zzz': ['x/**']}},
                  open(d3, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, d3, chg)
        if rc != 2:
            fails.append('③ 集合不等期望 rc=2，实得 %d' % rc)
        # ④ 正控半边：无改动件 ⇒ rc=0 且「受影响 0 步」
        chg0 = os.path.join(tmp, 'chg0.txt')
        open(chg0, 'w', encoding='utf-8').write('')
        rc, lines = analyse(lst, d1, chg0)
        if rc != 0:
            fails.append('④ 空改动期望 rc=0，实得 %d' % rc)
        elif not any('受影响 0 步' in x for x in lines):
            fails.append('④ 未打印「受影响 0 步」')
        # ⑤ 负控：声明件空 steps ⇒ rc=2
        d5 = os.path.join(tmp, 'd5.json')
        json.dump({'steps': {}}, open(d5, 'w', encoding='utf-8'))
        rc, _ = analyse(lst, d5, chg)
        if rc != 2:
            fails.append('⑤ 空声明期望 rc=2，实得 %d' % rc)
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
    ap.add_argument('--list')
    ap.add_argument('--decl')
    ap.add_argument('--changed')
    ap.add_argument('--self-test', action='store_true')
    ap.add_argument('--json', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    if not (ns.list and ns.decl):
        print('用法：affected-steps.py --list <件> --decl <件> [--changed <件>]', file=sys.stderr)
        return 2
    rc, lines = analyse(ns.list, ns.decl, ns.changed)
    if ns.json:
        print(json.dumps({'schema': 'zerg/v1', 'kind': 'AffectedSteps', 'rc': rc, 'lines': lines},
                         ensure_ascii=False, indent=1))
    else:
        for x in lines:
            print(x)
    return rc


if __name__ == '__main__':
    sys.exit(main())

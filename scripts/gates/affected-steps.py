#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""affected-steps.py —— 甲①「受影响步选择」**只报告档**（第 0 刀 · 只算不跳）

现读硬事实（前提，先现读再设计）：
  run_step 真跑那一行 = `( cd "${dir}" && bash -c "${cmd}" )`（precommit-gates.sh:567/569）
  ⇒ 子步**拿不到任何 run 作用域 env**；本判据**不依赖任何 env 通道** —— 它只在
     **本轮替代路径**里被调用（外层先把「本趟该跑哪几步」算出来再喂给 `--step … --only-step`），
     故与 run_step 无 env 耦合。本轮**只打印**「本趟该跑哪几步」，一个步都不跳。

输入 ＝ 本单新建的**声明表** `scripts/gates/affected-map.json`（由 `check-affected-map.py --emit`
现读生成 · 件→步/面 映射真源）。**本件不跳步、不改判据、不写任何步状态**。

判据（三档退码 0/1/2）——「怎么机检『没少选』」：
  C0 双向计数（**核心**）：声明表里**逐字命名**的步数 D 必须 == 步骤表（`--list` 自报）步数 N，
     且**命名集合逐字相等**（D==N 但集合不等 ⇒ 仍 rc=2）。**D≠N ⇒ rc=2 不给结论**
     （既有「少声明一步」= 静默漏选 ⇒ 当场拒，绝不拿漏读的声明当基线）。★ 这一格就是「没少选」。
  C1 解析自证：声明表每个 `steps` 子集在表里命中一次（没有幻影步）；解析出的条目数必须 >= 键数（漏读即 rc=2）。
  C2 受影响判定：受影响 = 该步声明的 glob/前缀里**至少一个**命中本趟改动件（`--changed` 逐行）。
     非受影响步逐条打印（可见 —— 「跳过必留痕」，绝不显示成 PASS）。
  C3 空转：**真无改动件** ⇒ 打印 `受影响 0 步`（可与「算不出」分辨）。

★ 输入取不到的路径（已拍第 3 条「取不到指纹 ⇒ 不跳」）：声明表**无指纹/无基线**（缺 `input_fingerprint`
  或 `source_head`）⇒ 报「不给结论」**rc=2**，✗ 不得当空集/当绿。
★ 未声明件（改动件命中不到任何声明）⇒ 按已拍第 3 条**默认每轮都跑** ⇒ 报**全量**并单列「未声明」件。

用法
  python3 affected-steps.py --list <--list 输出件> --map <声明表.json> --changed <改动件清单>
  python3 affected-steps.py --self-test
退出码：0 只报告（结论另行标注「未可信 · 只算不跳」）· 1 ——（本轮不产生）· 2 不给结论（少声明/漏读/取不到指纹/用法错）
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
    """--list 输出 → 步名列表（只取步骤行：首列 scope + 次列 模式 + 名）。"""
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


def load_map(path):
    with open(path, encoding='utf-8') as f:
        return json.load(f)


def c0_step_face(table_names, face_names):
    """C0 双向计数 ＋ 集合逐字相等 —— **唯一真源**（「步面 D==N 且逐字相等」这一格的单一定义处）。

    ★ 调用方（**两处共用本函数 · ✗ 不得另写第二份「逐字相等」**）：
      · 本件 `analyse()` 的 C0 步（甲① 只报告档）；
      · 门84 `scripts/gates/check-affected-map.py --check` 的「③ 步面」格（拍乙 · 由 importlib 现载本件取用）。

    参数：
      table_names —— 步骤表步名表（本件取 `--list` 自报；门84 取 `add_step` 现读；两者同源自报 · 同值）；
      face_names  —— 声明表 `faces` 展平**去重**后的步名表（顺序无关：本函数只比**集合**与**计数**）。
    返回 (ok: bool, detail: str)。
    """
    N, D = len(table_names), len(face_names)
    tset, dset = set(table_names), set(face_names)
    if D != N:
        return False, ("C0 未过：声明命名 %d 步 ≠ 步骤表 %d 步 ⇒ **有步未声明 = 静默漏选**" % (D, N))
    if dset != tset:
        miss = sorted(tset - dset)
        ghost = sorted(dset - tset)
        return False, ("C0 未过：命名集合 ≠ 表集合（漏=%s · 幻影=%s）"
                       % (miss[:5], ghost[:5]))
    return True, ("C0 双向计数过：D==N==%d 且命名集合逐字相等（**没少声明**）" % N)


def entry_matches(entry, rel):
    pat = entry.get('pattern', '')
    kind = entry.get('kind')
    if kind == 'prefix' or pat.endswith('/**'):
        base = pat[:-3] if pat.endswith('/**') else pat
        return rel == base or rel.startswith(base.rstrip('/') + '/')
    if kind == 'glob':
        return fnmatch.fnmatch(rel, pat)
    return rel == pat


def affected_for(mp, changed):
    """声明表 + 改动件 ⇒ (受影响步名表, 未声明改动件表)。"""
    entries = mp.get('files') or []
    step_order, seen = [], set()
    for f in mp.get('faces', {}):
        for n in mp['faces'][f]:
            if n not in seen:
                seen.add(n)
                step_order.append(n)
    aff, undeclared = set(), []
    for c in changed:
        hit = False
        for e in entries:
            if entry_matches(e, c):
                hit = True
                for n in (e.get('steps') or []):
                    aff.add(n)
        if not hit:
            undeclared.append(c)
    if undeclared:
        # 已拍第 3 条：未声明件默认每轮都跑 ⇒ 本轮全量
        return list(step_order), undeclared
    return [n for n in step_order if n in aff], []


def analyse(list_path, map_path, changed_path):
    L = []
    add = L.append
    if not os.path.isfile(list_path):
        return 2, ["[不给结论] 步骤表件缺失：%s ⇒ rc=2" % list_path]
    if not os.path.isfile(map_path):
        return 2, ["[不给结论] 声明表缺失：%s ⇒ rc=2" % map_path]
    table = parse_list(list_path)
    N = len(table)
    mp = load_map(map_path)
    # ★ 输入取不到的路径：无指纹 / 无基线 ⇒ 不给结论
    if not mp.get('input_fingerprint') or not mp.get('source_head'):
        return 2, ["[不给结论] 声明表无指纹/无基线（缺 input_fingerprint 或 source_head）"
                   "⇒ 取不到输入基线 ⇒ 不跳 ⇒ rc=2"]
    faces = mp.get('faces') or {}
    named = []
    seen = set()
    for f in sorted(faces):
        for n in faces[f]:
            if n not in seen:
                seen.add(n)
                named.append(n)
    D = len(named)
    add("步骤表（--list 自报）步数 N=%d · 声明表命名步数 D=%d（来源 %s @ %s）"
        % (N, D, map_path, str(mp.get('source_head'))[:16]))
    # C0 双向计数 + 集合逐字相等（**唯一真源** = `c0_step_face()` · 门84 `--check` 的「步面」格同调本函数）
    ok, detail = c0_step_face(table, named)
    if not ok:
        return 2, L + ["[不给结论] %s ⇒ rc=2" % detail]
    add(detail)
    # C1 解析自证
    n_entries = 0
    for e in (mp.get('files') or []):
        if not isinstance(e.get('steps'), list):
            return 2, L + ["[不给结论] C1 未过：条目「%s」的 steps 不是列表 ⇒ rc=2" % e.get('pattern')]
        n_entries += len(e['steps'])
    add("C1 解析自证过：声明条目 %d 条（面 %d 个 · 件型条目 %d 条）"
        % (n_entries, len(faces), len(mp.get('files') or [])))
    changed = []
    if changed_path and os.path.isfile(changed_path):
        with open(changed_path, encoding='utf-8') as f:
            changed = [x.strip() for x in f if x.strip() and not x.startswith('#')]
    aff, undeclared = affected_for(mp, changed)
    add("本趟改动件 %d 件" % len(changed))
    if undeclared:
        add("★ 未声明件 %d 件（已拍第 3 条 ⇒ 默认每轮都跑 ⇒ 本轮按**全量**）：" % len(undeclared))
        for x in undeclared[:10]:
            add("   ？ %s" % x)
    add("── 报告（**只报告档** · 只算不跳 · 不真跳任何步 · ✗ 本结果**未可信**）──")
    add("本次变更 ⇒ 理论上应受影响的步集：%d 步（本趟该跑）：" % len(aff))
    for n in aff[:10]:
        add("   ▶ %s" % n)
    if len(aff) > 10:
        add("   …（共 %d 步，余 %d 步略）" % (len(aff), len(aff) - 10))
    if not changed:
        add("   受影响 0 步（本读数可分辨「没跑」：上一行有 D/N=%d/%d 与改动件计数 0）" % (D, N))
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
        base = {'input_fingerprint': 'f' * 64, 'source_head': 'deadbeef'}
        # ① 正控：声明齐 + 有命中 ⇒ rc=0 且第一步受影响
        m1 = dict(base, faces={'core': ['a-step'], 'agent': ['b-step'], 'docs': ['c-step']},
                  files=[{'pattern': 'core/**', 'kind': 'prefix', 'steps': ['a-step']},
                         {'pattern': 'ui/**', 'kind': 'prefix', 'steps': ['b-step']},
                         {'pattern': 'docs/**', 'kind': 'prefix', 'steps': ['c-step']}])
        p1 = os.path.join(tmp, 'm1.json')
        json.dump(m1, open(p1, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, p1, chg)
        if rc != 0:
            fails.append('① 正控期望 rc=0，实得 %d' % rc)
        elif not any('▶ a-step' in x for x in lines):
            fails.append('① 正控未点名受影响步')
        # ② 负控：少声明一步（D=2, N=3）⇒ rc=2
        m2 = dict(base, faces={'core': ['a-step'], 'agent': ['b-step']},
                  files=[{'pattern': 'core/**', 'kind': 'prefix', 'steps': ['a-step']}])
        p2 = os.path.join(tmp, 'm2.json')
        json.dump(m2, open(p2, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, p2, chg)
        if rc != 2:
            fails.append('② 少声明期望 rc=2，实得 %d' % rc)
        elif not any('静默漏选' in x for x in lines):
            fails.append('② 未点名「静默漏选」')
        # ③ 负控：D==N 但集合不等（步名改名）⇒ rc=2
        m3 = dict(base, faces={'core': ['a-step'], 'agent': ['b-step'], 'docs': ['zzz']},
                  files=[])
        p3 = os.path.join(tmp, 'm3.json')
        json.dump(m3, open(p3, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, p3, chg)
        if rc != 2:
            fails.append('③ 集合不等期望 rc=2，实得 %d' % rc)
        # ④ 正控半边：无边改动 ⇒ rc=0 且「受影响 0 步」
        chg0 = os.path.join(tmp, 'chg0.txt')
        open(chg0, 'w', encoding='utf-8').write('')
        rc, lines = analyse(lst, p1, chg0)
        if rc != 0:
            fails.append('④ 空改动期望 rc=0，实得 %d' % rc)
        elif not any('受影响 0 步' in x for x in lines):
            fails.append('④ 未打印「受影响 0 步」')
        # ⑤ 负控：缺指纹/基线 ⇒ rc=2（已拍第 3 条）
        p5 = os.path.join(tmp, 'm5.json')
        json.dump({'faces': {'core': ['a-step'], 'agent': ['b-step'], 'docs': ['c-step']},
                   'files': []}, open(p5, 'w', encoding='utf-8'))
        rc, lines = analyse(lst, p5, chg)
        if rc != 2:
            fails.append('⑤ 缺指纹期望 rc=2，实得 %d' % rc)
        elif not any('取不到输入基线' in x for x in lines):
            fails.append('⑤ 未点名「取不到输入基线」')
        # ⑥ 负控：声明表空 faces ⇒ rc=2（0≠3）
        p6 = os.path.join(tmp, 'm6.json')
        json.dump(dict(base, faces={}, files=[]), open(p6, 'w', encoding='utf-8'))
        rc, _ = analyse(lst, p6, chg)
        if rc != 2:
            fails.append('⑥ 空声明期望 rc=2，实得 %d' % rc)
        # ⑦ 未声明件 ⇒ 报全量（受影响 = N）
        chg7 = os.path.join(tmp, 'chg7.txt')
        open(chg7, 'w', encoding='utf-8').write('deploy/unknown.yaml\n')
        rc, lines = analyse(lst, p1, chg7)
        if rc != 0:
            fails.append('⑦ 未声明件期望 rc=0，实得 %d' % rc)
        elif not any('未声明件 1 件' in x for x in lines):
            fails.append('⑦ 未单列「未声明」件')
        elif not any('受影响的步集：3 步' in x for x in lines):
            fails.append('⑦ 未按全量报（应 3 步）')
    finally:
        import shutil
        shutil.rmtree(tmp, ignore_errors=True)
    if fails:
        print('自检不过：')
        for x in fails:
            print('  ✗ ' + x)
        return 2
    print('自检通过：7 控（正控 3 ＋ 负控 4）逐条对拍 ✓')
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--list')
    ap.add_argument('--map')
    ap.add_argument('--decl')  # 旧名保留（指向同一张声明表），不新增语义
    ap.add_argument('--changed')
    ap.add_argument('--self-test', action='store_true')
    ap.add_argument('--json', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    mp = ns.map or ns.decl
    if not (ns.list and mp):
        print('用法：affected-steps.py --list <件> --map <声明表.json> [--changed <件>]', file=sys.stderr)
        return 2
    rc, lines = analyse(ns.list, mp, ns.changed)
    if ns.json:
        print(json.dumps({'schema': 'zerg/v1', 'kind': 'AffectedSteps', 'rc': rc, 'lines': lines},
                         ensure_ascii=False, indent=1))
    else:
        for x in lines:
            print(x)
    return rc


if __name__ == '__main__':
    sys.exit(main())

#!/usr/bin/env python3
# check-compat-manifest.py — B7 / G10 门禁：跨版本状态文件的兼容清单「双向一致」检查。
#
# 与 Hermes 的 COMPAT_MANIFEST 模式同构：清单是机器可读的单一真源，门禁负责证明它与代码
# 没有漂移，并且声明的版本关系自洽。
#
# 六条规则（任一不通过 ⇒ rc=1）：
#   R1 结构：字段合法、name/file 唯一、current_schema >= min_readable >= 0 且 current >= 1
#   R2 清单 → 代码（迁移入口）：migrate_func=X ⇒ core/internal/compat/migrations.go 里必须有 func migrateX
#   R3 清单 → 代码（读写锚点）：anchor 必须出现在 anchor_file 里（证明「这个文件真有读写实现」）
#   R4 代码 → 清单（双向的另一半）：源码里被 statepath.File("…") / ui_dir().join("…") 引用的
#      状态文件必须已登记（entries 或 excluded）——新加状态文件不登记就红
#   R5 excluded 必须给出理由，且不得与 entries 重名
#   R6 dirs 声明可与代码对上（环境变量名与解析器都出现在源码里）
#
# 纪律（与 CI 分层一致，见设计稿 G9）：
#   · 本门禁**只读仓库源码与清单**，绝不读写真机状态目录（~/.zerg/**）——CI 里跑真机路径是禁忌；
#     「真机状态现在是什么版本」由 core/cmd/zerg-compat 的 check 子命令现场回答，不进 CI。
#   · 规则用 --selftest 自证：往临时树里植入 6 类反例，每一类都必须被**对应规则**抓到（rc=1），
#     而且干净清单必须全过 —— 「永远报红」的守卫和「永远不报红」的守卫一样没用。
#
# 用法：
#   python3 scripts/gates/check-compat-manifest.py [仓库根]      # 默认自动定位（本文件的上上级）
#   python3 scripts/gates/check-compat-manifest.py --selftest    # 门禁自测（反例必须全被抓到）
import argparse
import copy
import json
import os
import re
import sys
import tempfile

MANIFEST_REL = os.path.join('core', 'internal', 'compat', 'compat.json')
MIGRATIONS_REL = os.path.join('core', 'internal', 'compat', 'migrations.go')

# R4 的发现模式。**诚实说明其边界**：这是字面量扫描——状态文件如果是用变量拼出来的路径创建的，
# 扫不到。--selftest 里的 R4 反例正是「植入一个未登记的 statepath.File("…")」，用于证明这条
# 扫描真的会报红（而不是只会说通过）。新增状态文件时人手登记仍是硬要求。
GO_PATTERNS = [
    r'statepath\.File\(\s*"([^"]+)"\s*\)',
    r'filepath\.Join\(\s*stateDir\s*,\s*"([^"]+)"\s*\)',
]
RUST_PATTERNS = [
    r'ui_dir\(\)\s*\.\s*join\(\s*"([^"]+)"\s*\)',
    r'format!\(\s*"\{\}/([A-Za-z0-9_.\-]+\.json)"',
]


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, MANIFEST_REL)):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def load_manifest(root):
    with open(os.path.join(root, MANIFEST_REL), 'r', encoding='utf-8') as f:
        return json.load(f)


def read_text(path):
    try:
        with open(path, 'r', encoding='utf-8', errors='replace') as f:
            return f.read()
    except OSError:
        return ''


def source_files(root, exts, subdirs):
    out = []
    for sub in subdirs:
        base = os.path.join(root, sub)
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = [x for x in dirnames if x not in ('.git', 'target', 'vendor', '.zerg')]
            for fn in filenames:
                if any(fn.endswith(e) for e in exts):
                    out.append(os.path.join(dirpath, fn))
    return sorted(out)


# ───────────────────────── 规则 ─────────────────────────

def r1_structure(man, root):
    v = []
    if int(man.get('schema', 0)) < 1:
        v.append('清单自身的 schema 必须 >= 1（实际 %r）' % man.get('schema'))
    names, files = {}, {}
    for i, e in enumerate(man.get('entries', [])):
        w = 'entries[%d](%s)' % (i, e.get('name'))
        name = e.get('name') or ''
        f = e.get('file') or ''
        if not name or not f:
            v.append(w + '：name/file 不得为空')
            continue
        if name in names:
            v.append(w + '：name 重复')
        names[name] = 1
        key = (e.get('dir') or '') + '/' + f
        if key in files:
            v.append(w + '：file 重复（' + key + '）')
        files[key] = 1
        if e.get('owner') not in ('go', 'ui'):
            v.append(w + '：owner 必须是 go|ui（实际 %r）' % e.get('owner'))
        if e.get('kind') not in ('file', 'glob'):
            v.append(w + '：kind 必须是 file|glob（实际 %r）' % e.get('kind'))
        if e.get('envelope') not in ('inband', 'sidecar'):
            v.append(w + '：envelope 必须是 inband|sidecar（实际 %r）' % e.get('envelope'))
        if e.get('stamps_schema') not in ('layer', 'native', 'sidecar', 'none'):
            v.append(w + '：stamps_schema 非法（%r）' % e.get('stamps_schema'))
        cur = e.get('current_schema')
        mn = e.get('min_readable')
        if not isinstance(cur, int) or not isinstance(mn, int):
            v.append(w + '：current_schema/min_readable 必须是整数')
            continue
        if cur < 1:
            v.append(w + '：current_schema=%d 必须 >= 1（schema 化的意义就是有版本号）' % cur)
        if mn < 0:
            v.append(w + '：min_readable=%d 不得为负' % mn)
        # 承重不变量：当前 schema 必须覆盖最低可读版本
        if cur < mn:
            v.append(w + '：current_schema=%d < min_readable=%d —— 声明自相矛盾' % (cur, mn))
        if e.get('envelope') == 'inband' and not (e.get('schema_field') or '').strip():
            v.append(w + '：inband 信封必须给 schema_field')
        if not (e.get('anchor_file') or '').strip() or not (e.get('anchor') or '').strip():
            v.append(w + '：anchor_file/anchor 不得为空')
    return v


def r2_migration_entry(man, root):
    v = []
    path = os.path.join(root, MIGRATIONS_REL)
    src = read_text(path)
    if not src:
        return ['迁移实现文件缺失：' + MIGRATIONS_REL]
    for e in man.get('entries', []):
        fn = (e.get('migrate_func') or '').strip()
        if not fn:
            v.append('%s：migrate_func 不得为空' % e.get('name'))
            continue
        # 约定：migrate_func=X ⇒ 必须有 func migrateX
        if not re.search(r'func\s+migrate' + re.escape(fn) + r'\s*\(', src):
            v.append('%s：migrate_func=%s 在 %s 里找不到 func migrate%s（迁移入口不存在）'
                     % (e.get('name'), fn, MIGRATIONS_REL, fn))
    return v


def r3_read_write_anchor(man, root):
    v = []
    for e in man.get('entries', []):
        af = (e.get('anchor_file') or '').strip()
        anchor = (e.get('anchor') or '').strip()
        if not af or not anchor:
            continue  # R1 已报
        src = read_text(os.path.join(root, af))
        if not src:
            v.append('%s：anchor_file 不存在（%s）' % (e.get('name'), af))
            continue
        if anchor not in src:
            v.append('%s：anchor %r 不在 %s 里（该文件可能没有真正的读写实现）'
                     % (e.get('name'), anchor, af))
    return v


def declared_names(man):
    out = set()
    for e in man.get('entries', []):
        out.add(os.path.basename(e.get('file') or ''))
    for x in man.get('excluded', []):
        out.add(os.path.basename(x.get('file') or ''))
    return out


def discovered_state_files(root):
    found = {}
    for path in source_files(root, ['.go'], ['core', 'agent']):
        src = read_text(path)
        for pat in GO_PATTERNS:
            for m in re.finditer(pat, src):
                found.setdefault(m.group(1), path)
    for path in source_files(root, ['.rs'], ['ui']):
        src = read_text(path)
        for pat in RUST_PATTERNS:
            for m in re.finditer(pat, src):
                found.setdefault(m.group(1), path)
    return found


def r4_code_to_manifest(man, root):
    v = []
    declared = declared_names(man)
    for name, where in sorted(discovered_state_files(root).items()):
        if name in declared:
            continue
        v.append('状态文件 %r（被 %s 引用）未登记 —— 请写进 compat.json 的 entries（带 schema 版本）'
                 '或 excluded（带理由）' % (name, os.path.relpath(where, root)))
    return v


def r5_excluded_reasons(man, root):
    v = []
    entry_files = set(os.path.basename(e.get('file') or '') for e in man.get('entries', []))
    for i, x in enumerate(man.get('excluded', [])):
        f = (x.get('file') or '').strip()
        if not f:
            v.append('excluded[%d]：file 不得为空' % i)
            continue
        if not (x.get('reason') or '').strip():
            v.append('excluded[%d](%s)：必须给出不登记的理由（不许无理由排除）' % (i, f))
        if os.path.basename(f) in entry_files:
            v.append('excluded[%d](%s)：同时出现在 entries —— 二者互斥' % (i, f))
    return v


def r6_dirs_declaration(man, root):
    v = []
    dirs = man.get('dirs') or {}
    if not dirs:
        v.append('清单缺 dirs 段（状态目录的解析方式必须声明）')
    blobs = []
    for path in source_files(root, ['.go'], ['core']):
        blobs.append(read_text(path))
    joined = '\n'.join(blobs)
    for key, d in dirs.items():
        env = (d.get('env') or '').strip()
        resolver = (d.get('resolver') or '').strip()
        if env and env not in joined:
            v.append('dirs.%s：环境变量 %s 未在 Go 源码里出现（声明与实现脱节）' % (key, env))
        tail = resolver.split('.')[-1]
        if tail and not re.search(r'func\s+' + re.escape(tail) + r'\s*\(', joined):
            v.append('dirs.%s：解析器 %s 未在 Go 源码里定义' % (key, resolver))
    return v


RULES = [
    ('R1 清单结构 / 版本关系自洽', r1_structure),
    ('R2 迁移入口存在（清单→代码）', r2_migration_entry),
    ('R3 读写锚点存在（清单→代码）', r3_read_write_anchor),
    ('R4 状态文件已登记（代码→清单）', r4_code_to_manifest),
    ('R5 excluded 必须给理由', r5_excluded_reasons),
    ('R6 状态目录声明可对上代码', r6_dirs_declaration),
]


def run_rules(man, root):
    """返回 {规则名: [问题…]}（只含有问题的）。"""
    out = {}
    for label, fn in RULES:
        try:
            probs = fn(man, root)
        except Exception as exc:  # 规则自身崩了也算红，不许静默通过
            probs = ['规则执行异常：%r' % exc]
        if probs:
            out[label] = probs
    return out


def cmd_check(root):
    man = load_manifest(root)
    print('兼容清单: ' + os.path.join(root, MANIFEST_REL))
    print('条目 %d · 明确不登记 %d · 清单 schema %s'
          % (len(man.get('entries', [])), len(man.get('excluded', [])), man.get('schema')))
    print('')
    bad = run_rules(man, root)
    if bad:
        for label in sorted(bad):
            print('❌ ' + label)
            for p in bad[label]:
                print('     · ' + p)
        print('')
        print('门禁未通过：清单与代码不一致（%d 类问题）' % len(bad))
        return 1
    print('✅ 六条规则全过：清单与代码双向一致，且 current_schema >= min_readable >= 0')
    for e in man.get('entries', []):
        print('   %-20s %-28s schema=%d min=%d %s/%s'
              % (e.get('name'), e.get('file'), e.get('current_schema'),
                 e.get('min_readable'), e.get('envelope'), e.get('stamps_schema')))
    return 0


# ───────────────────────── 自测（反例必须全被抓到） ─────────────────────────

R1 = 'R1 清单结构 / 版本关系自洽'
R2 = 'R2 迁移入口存在（清单→代码）'
R3 = 'R3 读写锚点存在（清单→代码）'
R4 = 'R4 状态文件已登记（代码→清单）'
R5 = 'R5 excluded 必须给理由'
R6 = 'R6 状态目录声明可对上代码'


def cmd_selftest(root):
    """两类断言配对，防止守卫自欺：
    · 干净清单 + 真仓库根 ⇒ 一条问题都不许报（守卫不能永远报红）
    · 每种植入的反例 ⇒ **对应那条规则**必须报红（守卫必须能失败）
    反例只改内存里的清单（除 R4 外都用真仓库根跑，这样报红只可能来自植入的那处改动）。"""
    man = load_manifest(root)
    cases = []

    # ① 干净清单必须全过
    cases.append(('干净清单全过（无假阳性）', lambda: not run_rules(man, root)))

    # ② R1：current < min_readable（声明自相矛盾）
    b = copy.deepcopy(man)
    b['entries'][0]['min_readable'] = int(b['entries'][0]['current_schema']) + 1
    cases.append(('R1 current<min_readable', lambda: R1 in run_rules(b, root)))

    # ③ R1：current_schema = 0（等于没有版本号）
    b2 = copy.deepcopy(man)
    b2['entries'][0]['current_schema'] = 0
    cases.append(('R1 current_schema=0', lambda: R1 in run_rules(b2, root)))

    # ④ R2：migrate_func 指向不存在的函数
    b3 = copy.deepcopy(man)
    b3['entries'][0]['migrate_func'] = 'NoSuchMigrationXyz'
    cases.append(('R2 迁移入口缺失', lambda: R2 in run_rules(b3, root)))

    # ⑤ R3：anchor 指向不存在的符号
    b4 = copy.deepcopy(man)
    b4['entries'][0]['anchor'] = 'func no_such_symbol_xyz()'
    cases.append(('R3 锚点缺失', lambda: R3 in run_rules(b4, root)))

    # ⑥ R4：源码里植入一个未登记的状态文件（临时树，绝不碰真仓）
    #    配对断言：空树不许报 R4（否则「永远报红」也能通过自测）
    b5 = copy.deepcopy(man)
    b5['entries'] = []
    b5['excluded'] = []

    def r4_pair():
        with tempfile.TemporaryDirectory() as empty:
            if R4 in run_rules(b5, empty):
                return False  # 空树都报 R4 ⇒ 检测器是「永远报红」，不算数
        with tempfile.TemporaryDirectory() as tmp:
            planted = os.path.join(tmp, 'core', 'internal', 'zzprobe')
            os.makedirs(planted)
            with open(os.path.join(planted, 'probe.go'), 'w', encoding='utf-8') as f:
                f.write('package zzprobe\n\nfunc p() string { return statepath.File("brand_new_state.json") }\n')
            return R4 in run_rules(b5, tmp)

    cases.append(('R4 未登记状态文件（含空树不误报的反证）', r4_pair))

    # ⑦ R5：excluded 没有理由
    b6 = copy.deepcopy(man)
    if b6.get('excluded'):
        b6['excluded'][0]['reason'] = '   '
    else:
        b6['excluded'] = [{'file': 'x.jsonl', 'reason': ''}]
    cases.append(('R5 excluded 缺理由', lambda: R5 in run_rules(b6, root)))

    # ⑧ R6：环境变量名与代码脱节
    b7 = copy.deepcopy(man)
    for k in b7.get('dirs', {}):
        b7['dirs'][k]['env'] = 'ZERG_NO_SUCH_ENV_XYZ'
    cases.append(('R6 目录声明脱节', lambda: R6 in run_rules(b7, root)))

    failed = 0
    for label, pred in cases:
        ok = False
        try:
            ok = bool(pred())
        except Exception as exc:
            print('     （反例 %s 执行异常：%r）' % (label, exc))
        if not ok:
            failed += 1
        print('%s 反例 %s' % ('✅' if ok else '❌', label))
    print('')
    if failed:
        print('门禁自测失败：%d 个反例没被抓住 —— 这个门禁不可信' % failed)
        return 1
    print('门禁自测通过：干净清单全过（无假阳性）+ %d 类反例全部被对应规则抓到' % (len(cases) - 1))
    return 0


def main():
    ap = argparse.ArgumentParser(description='跨版本状态文件兼容清单门禁（B7 / G10）')
    ap.add_argument('root', nargs='?', help='仓库根（默认自动定位）')
    ap.add_argument('--selftest', action='store_true', help='跑门禁自测：反例必须全部被抓到')
    args = ap.parse_args()

    root = find_root(args.root or os.path.dirname(os.path.abspath(__file__)))
    if not root:
        print('找不到仓库根（缺 %s）' % MANIFEST_REL, file=sys.stderr)
        return 1
    if args.selftest:
        return cmd_selftest(root)
    return cmd_check(root)


if __name__ == '__main__':
    sys.exit(main())

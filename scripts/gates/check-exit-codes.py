#!/usr/bin/env python3
# check-exit-codes.py — 门⑨：退码表「一张表 + 同数字不再两义」（§二十一 已红第 4 条 · 开工单 T-26 ·
# §九 M3 `C6` · §十二 `P-129`）。
#
# 为什么要有它：已红第 4 条的病根是「同一个数字在同一仓里有两义」—— 这种病的表现**不是报错**，
#   是「AI 看到 `4` 判不出该换令牌还是该改用法」。它只能靠机器每次查：人不会记得第 7 处声明在哪儿。
#
# 六条规则（任一不通过 ⇒ 见下方退出码）：
#   R1 **主表自身**：`core/cmd/zerg/exitcodes.go` 的 `exitcodeTable` + `exitcodeReserved` 里
#      **数字唯一**（同表两义 ⇒ 红），且每行 name/meaning/retry 三格非空（占空格的号不算登记）。
#   R2 **门禁直通档逐字对齐**：`scripts/gates/precommit-gates.sh` 第 35 行（去行首 `# `）==
#      `core/cmd/zerg/helptext.go` 的 `gateExitLine`。
#   R3 **六处声明面全部指向同一张表**：每处都含标记行 `退码唯一真源 = zerg help exit-codes`。
#   R4 **「回滚成功」唯一说法**：`zerg-swap-core.sh` 与 `zerg-upgrade.sh` 里都必须出现 `ROLLED_BACK`，
#      且两处**不许**再把「回滚成功」写成 `2`（`= 2` / `2 已回滚成功` 两种写法都算）。
#   R5 **用码有落点**：每个声明面源码里用到的退码数字，必须落在登记表
#      （`core/internal/contract/exitcode-namespaces.json`）覆盖的集合里 ——
#      主表 / 该件自有号 / 已登记的撞号 / 已登记的越界号，四类之一。未登记 ⇒ 红。
#   R6 **空转/缺件 ⇒ rc=2**（不给结论，不许当绿）。
#
# 用法：
#   python3 scripts/gates/check-exit-codes.py [仓库根]
#   python3 scripts/gates/check-exit-codes.py --self-test    # 成对负控（1 正 + 6 反 + 1 空转）
import argparse
import json
import os
import re
import sys
import tempfile

TOOL = 'check-exit-codes'
MARKER = '退码唯一真源 = zerg help exit-codes'
SITES_REL = 'core/internal/contract/exitcode-namespaces.json'
TABLE_REL = os.path.join('core', 'cmd', 'zerg', 'exitcodes.go')
HELPTEXT_REL = os.path.join('core', 'cmd', 'zerg', 'helptext.go')
GATES_REL = os.path.join('scripts', 'gates', 'precommit-gates.sh')
SWAP_REL = os.path.join('scripts', 'build', 'zerg-swap-core.sh')
UPGRADE_REL = os.path.join('scripts', 'build', 'zerg-upgrade.sh')
ROLLBACK_BAD_RE = re.compile(r'回滚成功[^\n]{0,24}?[=＝]\s*`?2`?|`?2`?[^\n]{0,12}?已回滚成功')
ROW_RE = re.compile(r'\{\s*(\d+)\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"\s*,\s*"([^"]*)"')


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isdir(os.path.join(d, 'core', 'cmd', 'zerg')):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def read(path):
    with open(path, encoding='utf-8') as f:
        return f.read()


def parse_table(text, var):
    """取 `var = []exitcodeRow{ ... }` 段里的行。"""
    m = re.search(r'var\s+' + var + r'\s*=\s*\[\]exitcodeRow\{(.*?)\n\}', text, re.S)
    if not m:
        return None
    return [{'code': int(a), 'name': b, 'meaning': c, 'retry': d}
            for a, b, c, d in ROW_RE.findall(m.group(1))]


def used_codes(text, mode):
    """从声明面源码里取「用到的退码数字」集合。"""
    out = set()
    if mode == 'go':
        for pat in (r'os\.Exit\((\d+)\)', r'exit[A-Za-z]+\s*=\s*(\d+)', r'exitCode\s*=\s*(\d+)'):
            out |= {int(x) for x in re.findall(pat, text)}
    else:  # sh：只认 `exit <N>`（函数里的 `return <N>` 是布尔返回，不是退码）
        out |= {int(x) for x in re.findall(r'(?:^|[^A-Za-z_])exit\s+(\d+)', text, re.M)}
    return out


def detect_mode(path):
    return 'go' if path.endswith('.go') else 'sh'


def find_gate_line(text):
    """按**内容**找门禁头注释里的退码行（`# 退出码：…`）。

    ★ 为什么**不**用行号（原来是「第 35 行」）：本仓已经吃过一次行号漂移的亏 —— `-h` 曾硬编码
      `sed -n '19,140p'`，被 `G4-b`/`P-054` 改成按标记切。实测（2026-09-20 批 C · T-26）：
      头注释的退码行**已经从第 35 行漂到第 36 行**（批 B 在头注释里加了 4 行用法行）⇒ 任何
      「sed -n '35p'」形态的复跑命令当场假红。⇒ 本门按内容定位：命中数必须**恰好 1**。
    """
    hits = [ln for ln, line in enumerate(text.splitlines(), 1)
            if re.match(r'^#\s*退出码：', line)]
    return hits


def check(root):
    problems, notes = [], []
    registry_path = os.path.join(root, SITES_REL)
    if not os.path.isfile(registry_path):
        return 2, ['缺件：登记表不存在 %s（判据不可判 ⇒ 不给结论）' % SITES_REL], [], {}
    try:
        reg = json.loads(read(registry_path))
    except Exception as e:                                        # noqa: BLE001
        return 2, ['登记表不可解析：%s' % e], [], {}
    ns = {}
    for n in reg.get('namespaces', []):
        ns[n.get('owner')] = n
    if not ns:
        return 2, ['登记表里 0 个命名空间（空转 ⇒ 不给结论）'], [], {}
    sites = reg.get('sites') or []
    if not sites:
        return 2, ['登记表里 0 个声明面（空转 ⇒ 不给结论）'], [], {}

    # ── R1 主表自身：数字唯一 + 三格非空 ──────────────────────────────
    tbl_txt = read(os.path.join(root, TABLE_REL))
    main = parse_table(tbl_txt, 'exitcodeTable') or []
    reserved = parse_table(tbl_txt, 'exitcodeReserved') or []
    if not main:
        return 2, ['空转：主表解析出 0 行（判据不可判）'], [], {}
    allrows = main + reserved
    seen = {}
    for r in allrows:
        if r['code'] in seen:
            problems.append('FAIL R1 主表里数字 %d 出现两次（同数字两义）：%s ↔ %s'
                            % (r['code'], seen[r['code']]['name'], r['name']))
        seen[r['code']] = r
        for key, label in (('name', '机器名'), ('meaning', '语义'), ('retry', '可重试性')):
            if not str(r[key]).strip():
                problems.append('FAIL R1 主表 %d 的%s格是空的（占号必须写明）' % (r['code'], label))
    table_codes = {r['code'] for r in allrows}
    notes.append('主表 %d 档（0/1/2 与门禁同码同义）+ 已挂号 %d 档 ⇒ 数字集合 %s'
                 % (len(main), len(reserved), sorted(table_codes)))

    # ── R2 门禁直通档逐字对齐 ────────────────────────────────────────
    htxt = read(os.path.join(root, HELPTEXT_REL))
    m = re.search(r'const\s+gateExitLine\s*=\s*"((?:[^"\\]|\\.)*)"', htxt)
    if not m:
        return 2, ['空转：helptext.go 里找不到 gateExitLine（判据不可判）'], [], {}
    gate_const = m.group(1)
    gates_txt = read(os.path.join(root, GATES_REL))
    hits = find_gate_line(gates_txt)
    if not hits:
        return 2, ['空转：门禁头注释里找不到 `# 退出码：…` 那一行（判据不可判）'], [], {}
    if len(hits) > 1:
        problems.append('FAIL R2 门禁头注释里 `# 退出码：…` 命中 %d 行（%s）—— 同一件东西两处口径'
                        % (len(hits), hits))
    line_no = hits[0]
    line_txt = re.sub(r'^#\s?', '', gates_txt.splitlines()[line_no - 1]).strip()
    if line_txt != gate_const.strip():
        problems.append('FAIL R2 门禁头注释的退码行（现第 %d 行）与命令面 gateExitLine **不逐字一致**：\n      门禁: %s\n      命令面: %s'
                        % (line_no, line_txt, gate_const.strip()))
    else:
        notes.append('门禁直通档与命令面 gateExitLine 逐字一致（按内容定位：现第 %d 行）' % line_no)

    # ── R3 六处声明面全部指向同一张表 ────────────────────────────────
    for rel in sites:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            problems.append('FAIL R3 声明面缺件：%s' % rel)
            continue
        if MARKER not in read(p):
            problems.append('FAIL R3 %s 里没有标记行「%s」—— 它又变成一处独立说法了' % (rel, MARKER))
    notes.append('声明面 %d 处逐条点名' % len(sites))

    # ── R4 「回滚成功」唯一说法 ──────────────────────────────────────
    for rel in (SWAP_REL, UPGRADE_REL):
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            problems.append('FAIL R4 缺件：%s' % rel)
            continue
        txt = read(p)
        if 'ROLLED_BACK' not in txt:
            problems.append('FAIL R4 %s 里没有 `ROLLED_BACK`（回滚成功 = 1 + kind=ROLLED_BACK 的说法没落）' % rel)
        for ln, line in enumerate(txt.splitlines(), 1):
            if ROLLBACK_BAD_RE.search(line):
                problems.append('FAIL R4 %s:%d 又把「回滚成功」写成 2（否决的说法）：%s'
                                % (rel, ln, line.strip()[:100]))

    # ── R5 用码有落点 ───────────────────────────────────────────────
    coll = {c.get('number') for c in reg.get('collisions', [])}
    oob = {o.get('code') for o in reg.get('out_of_range', [])}
    for rel in sites:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            continue
        entry = ns.get(rel)
        if entry is None:
            problems.append('FAIL R5 %s 没有登记命名空间（新声明面必须先在登记表里立一行）' % rel)
            continue
        own = set(int(k) for k in (entry.get('declared') or {}).keys())
        internal = set(int(k) for k in (entry.get('internal') or {}).keys())
        allowed = table_codes | own | coll | oob | internal
        used = used_codes(read(p), detect_mode(p))
        extra = sorted(used - allowed)
        if extra:
            problems.append('FAIL R5 %s 用了未登记的退码 %s（登记覆盖：主表 ∪ 自有号 ∪ 已登记撞号 ∪ 已登记越界）'
                            % (rel, extra))
        else:
            notes.append('%s 用码 %s 逐条有落点' % (os.path.basename(rel), sorted(used)))

    rc = 0 if not problems else 1
    return rc, problems, notes, {'table': sorted(table_codes), 'sites': len(sites),
                                 'collisions': len(coll), 'out_of_range': len(oob)}


def print_report(root, rc, problems, notes, extra):
    print('── 门⑨ 退码表：一张表 + 同数字不再两义（§二十一 已红第 4 条 · T-26 · §九 M3 C6 · §十二 P-129）──')
    print('  真源: core/cmd/zerg/exitcodes.go · 登记表: %s' % SITES_REL)
    for n in notes:
        print('  · %s' % n)
    for p in problems:
        print('  · %s' % p)
    if rc == 0:
        print('⇒ 全绿（rc=0）：六处声明面指向同一张表 · 主表内数字唯一 · 回滚成功只有一个说法 · 用码逐条有落点')
    elif rc == 1:
        print('⇒ 门禁红（rc=1）：退码说法/编号面出现第二个口径')
    else:
        print('⇒ 不给结论（rc=2）')
    return rc


# ─────────────────────────── 自检（成对负控）───────────────────────────
GOOD_TABLE = '''package main
var exitcodeTable = []exitcodeRow{
\t{0, "ok", "成功", "—"},
\t{2, "usage", "用法错 / **不给结论**", "改用法后可重试"},
}
var exitcodeReserved = []exitcodeRow{
\t{130, "interrupted", "人打断", "—"},
}
'''
GOOD_GATE_LINE = '# 退出码：0 全绿 · 1 有失败项 · 2 **不给结论**\n'
GOOD_MSG = '示例说明。\n'
GOOD_DECL = (
    '# 退码唯一真源 = zerg help exit-codes（契约 §三）—— 本件是组件自有码空间\n'
    'const (\n\texitOK = 0\n\texitUsage = 2\n)\n'
)
GOOD_SWAP = (
    '# 退码唯一真源 = zerg help exit-codes（契约 §三）——本件是脚本自有码空间\n'
    '# 「回滚成功」= 1 + error.kind=ROLLED_BACK（不是 0，也不是 2）\n'
    'exit 0\n'
    'exit 2\n'
    'say "error.kind=ROLLED_BACK"\n'
)
GOOD_UP = (
    '# 退码唯一真源 = zerg help exit-codes（契约 §三）——本件是脚本自有码空间\n'
    '# 「回滚成功」= 1 + error.kind=ROLLED_BACK\n'
    'exit 0\n'
    'exit 1\n'
)
GOOD_ROSTER = {
    'schema': 'zerg/exitcodes-namespaces/v1',
    'namespaces': [
        {'id': 'cli', 'owner': 'core/cmd/zerg/exitcodes.go', 'scan': 'go-table'},
        {'id': 'zerg-compat', 'owner': 'core/cmd/zerg-compat/main.go', 'scan': 'go',
         'declared': {'0': 'ok', '2': '有待迁移文件'}},
        {'id': 'swap', 'owner': 'scripts/build/zerg-swap-core.sh', 'scan': 'sh',
         'declared': {'0': '换件成功', '2': '用法错'}},
        {'id': 'upgrade', 'owner': 'scripts/build/zerg-upgrade.sh', 'scan': 'sh',
         'declared': {'0': '成功', '1': '失败'}},
        {'id': 'gates', 'owner': 'scripts/gates/precommit-gates.sh', 'scan': 'sh',
         'declared': {'0': '全绿', '1': '有失败项', '2': '不给结论'}},
    ],
    'collisions': [{'number': 2, 'verdict': '已登记', 'trigger': '待拍'}],
    'out_of_range': [],
    'sites': ['core/cmd/zerg-compat/main.go', 'scripts/build/zerg-swap-core.sh',
              'scripts/build/zerg-upgrade.sh', 'scripts/gates/precommit-gates.sh'],
}


def _fixture(d, **over):
    def w(rel, text):
        p = os.path.join(d, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, 'w', encoding='utf-8') as f:
            f.write(text)
        return p

    w(TABLE_REL, over.get('table', GOOD_TABLE))
    # helptext：gateExitLine 与门禁头注释里那一行必须逐字一致（按内容定位，不靠行号）
    gate_line = over.get('gate_line', '退出码：0 全绿 · 1 有失败项 · 2 **不给结论**')
    w(HELPTEXT_REL, 'package main\n\nconst gateExitLine = %s\n' % json.dumps(gate_line, ensure_ascii=False))
    w(GATES_REL, '#!/usr/bin/env bash\n'
                 '# 退码唯一真源 = zerg help exit-codes（契约 §三）\n'
                 '# ' + over.get('gate_file_line', gate_line) + '\n')
    w('core/cmd/zerg-compat/main.go', over.get('decl', GOOD_DECL))
    w(SWAP_REL, over.get('swap', GOOD_SWAP))
    w(UPGRADE_REL, over.get('up', GOOD_UP))
    reg = json.loads(json.dumps(over.get('roster', GOOD_ROSTER)))
    w(SITES_REL, json.dumps(reg, ensure_ascii=False))
    return d


def self_test():
    cases = [
        ('正控① 好件：主表唯一 · 门禁行逐字对齐 · 四处都有标记 · 回滚说法为 1+ROLLED_BACK ⇒ rc=0', 0, {}),
        ('负控② 主表里数字重复（0 出现两次）⇒ rc=1', 1, {'table': GOOD_TABLE.replace(
            '{2, "usage", "用法错 / **不给结论**", "改用法后可重试"},',
            '{0, "dup", "重复的 0", "—"},')}),
        ('负控③ 门禁头注释第 35 行被改一个词 ⇒ rc=1', 1, {'gate_file_line': '退出码：0 全绿 · 1 有失败项 · 2 不给结论'}),
        ('负控④ 某处声明面摘掉标记行 ⇒ rc=1', 1, {'decl': GOOD_DECL.replace(
            '# 退码唯一真源 = zerg help exit-codes（契约 §三）—— 本件是组件自有码空间\n', '')}),
        ('负控⑤ 把「回滚成功」又写成 2 ⇒ rc=1', 1, {'swap': GOOD_SWAP.replace(
            '# 「回滚成功」= 1 + error.kind=ROLLED_BACK（不是 0，也不是 2）\n',
            '# 回滚成功 = 2（老说法）\n')}),
        ('负控⑥ 声明面用了未登记的退码（swap 用了 7）⇒ rc=1', 1, {'swap': GOOD_SWAP + 'exit 7\n'}),
        ('负控⑦ 登记表缺件 ⇒ rc=2（判据不可判）', 2, {'roster': None}),
        ('负控⑧ 登记表里 0 个命名空间 ⇒ rc=2（空转不许当绿）', 2, {'roster': dict(GOOD_ROSTER, namespaces=[])}),
    ]
    ok = True
    for name, want, over in cases:
        with tempfile.TemporaryDirectory() as d:
            if over.get('roster', 'x') is None:
                over = dict(over)
                over['roster'] = GOOD_ROSTER
                _fixture(d, **over)
                os.unlink(os.path.join(d, SITES_REL))
            else:
                _fixture(d, **over)
            rc, problems, notes, _ = check(d)
            flag = '✓' if rc == want else '✗'
            print('  %s %s（rc=%d）' % (flag, name, rc))
            if rc != want:
                ok = False
                for p in problems:
                    print('      · %s' % p)
    print('自检结论: ' + ('全过' if ok else '**有断言失败**'))
    return 0 if ok else 1


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser()
    ap.add_argument('root', nargs='?', default='.')
    ap.add_argument('--self-test', action='store_true')
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = find_root(a.root)
    if root is None:
        print('⛔ 不给结论：找不到 core/cmd/zerg（本门只认命令面真源所在的那棵树）')
        return 2
    rc, problems, notes, extra = check(root)
    return print_report(root, rc, problems, notes, extra)


if __name__ == '__main__':
    sys.exit(main())

#!/usr/bin/env python3
# check-cli-contract.py — 门禁⑪：**命令面自身的契约**（§九 M17 `T1`–`T6` · 开工单 T-42）。
#
# 为什么要有它（§九 M17 逐字：命令面「自己**也得被门禁守**」）：
#   命令面是薄壳，它的病**不是崩**，是**悄悄少一层**：新加一条命令忘了配 must-fail、
#   退码表被抄成第二份、`--json` 包封被改形状、矩阵被豁免洗白…… 这些都不会报错，
#   只会让「契约」慢慢变成一纸空文。判据按 §九 M17 的六条条款，**本门不重写判据**
#   （`T6`：引用 M6 `R7`，判据本体在那两条里）。
#
# 六条规则（任一不通过 ⇒ rc=1；判不了 / 空转 ⇒ rc=2，**不给结论**）：
#   T1 **逐命令测试件存在**：命令树里的**每条**命令都要在 `testdata/cli-matrix.json` 里有 ≥1 条 case，
#      或一条**带 reason** 的豁免；矩阵里点名命令树里没有的命令（幽灵）⇒ 红。
#   T2 **每条命令至少一条 must-fail**：每条 case 的 `want_rc != 0`（must-fail 不许被稀释成「跑几条」）。
#   T3 **退码表唯一**：`main.go` 的退码常量 ⟷ `exitcodes.go` 的表**逐格对齐** · 表内数字唯一
#      （**同数字两义 ⇒ 红**）· 命令面源码里的**裸数字** `return N` / `os.Exit(N)` 必须 ⊆ 合法集。
#   T4 **`--json` 形状守卫（运行时只读）**：跑 `go test ./cmd/zerg -run TestCLIContractJSONShape`
#      —— 它读的是**当前源码**的运行期行为（不是 `bin/` 里的旧制品）。
#   T5 **矩阵不留空白 + baseline 棘轮**：空白数 > baseline 里记的数 ⇒ 红（只许下调）；
#      豁免**没有 reason** ⇒ **rc=2**（「写不清理由的豁免就是洗白」`T5` 逐字）；baseline 缺/不可解析 ⇒ 2。
#   T6 **引用 M6 `R7`**：本门头部引用它 + 契约登记表 `registry.json` 的命令面条目 `gate` 字段
#      **点名本脚本**（引用面非 0 = 它真的被用，不是挂在空气上）。
#
# 用法：
#   python3 scripts/gates/check-cli-contract.py [仓库根]
#   python3 scripts/gates/check-cli-contract.py --self-test          # 成对负控（1 正控 + 8 负控 + 1 空转）
#   python3 scripts/gates/check-cli-contract.py --no-runtime         # **只给夹具用**：跳过 T4 的 go test
#
# ★ 判据出处（`T6` 的引用面）：§九 M6 `R7`（字段只增不改 / 破坏性变更走大版本）· §九 M17 `T1`–`T6`。
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")
BASELINE_REL = os.path.join("scripts", "gates", "cli-contract-baseline.json")
EXITCODES_REL = os.path.join("core", "cmd", "zerg", "exitcodes.go")
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
REGISTRY_REL = os.path.join("core", "internal", "contract", "registry.json")

CMD_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
PATH_SEG_RE = re.compile(r'"([^"]+)"')
DANGER_RE = re.compile(r'danger:\s*&dangerSpec\{(dangerD[23])')
FIELDS_RE = re.compile(r'fields:\s*\[\]string\{([^}]*)\}')
ENDPOINT_RE = re.compile(r'endpoint:\s*"([^"]*)"')
PASSTHROUGH_RE = re.compile(r'passthrough:\s*true')
ROW_RE = re.compile(r'\{\s*(\d+)\s*,\s*"([^"]+)"\s*,\s*"([^"]*)"')
CONST_RE = re.compile(r'exit([A-Z]\w*)\s*=\s*(\d+)\b')
BARE_RET_RE = re.compile(r'\b(?:return|os\.Exit)\s+\(?\s*(\d+)\b')


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, MAIN_REL)):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def read(path):
    with open(path, encoding='utf-8') as f:
        return f.read()


def parse_tree(root):
    """命令树 → [(path, shape)]；shape = dict(danger, fields, net, passthrough)。"""
    src = read(os.path.join(root, MAIN_REL))
    try:
        start = src.index('commands = []*command{')
    except ValueError:
        return []
    end = src.index('\n\t}\n', start)
    seg = src[start:end]
    out, matches = [], list(CMD_RE.finditer(seg))
    for i, m in enumerate(matches):
        parts = PATH_SEG_RE.findall(m.group(1))
        if not parts:
            continue
        nxt = matches[i + 1].start() if i + 1 < len(matches) else len(seg)
        block = seg[m.end():nxt]
        ep = ENDPOINT_RE.search(block)
        out.append((' '.join(parts), {
            'danger': m2.group(1) if (m2 := DANGER_RE.search(block)) else '',
            'fields': bool(FIELDS_RE.search(block)),
            'net': bool(ep and ep.group(1).strip()),
            'passthrough': bool(PASSTHROUGH_RE.search(block)),
        }))
    return out


def exitcode_table(root):
    """退码表（主表 + 已挂号）→ [(code, name, meaning)]。"""
    src = read(os.path.join(root, EXITCODES_REL))
    rows = []
    for var in ('exitcodeTable', 'exitcodeReserved'):
        try:
            i = src.index('var %s = []exitcodeRow{' % var)
        except ValueError:
            continue
        j = src.index('\n}', i)
        for m in ROW_RE.finditer(src[i:j]):
            rows.append((int(m.group(1)), m.group(2), m.group(3)))
    return rows


def named_consts(root):
    """`main.go` 的退码常量块 → {名: 值}。"""
    src = read(os.path.join(root, MAIN_REL))
    try:
        i = src.index('const (')
        j = src.index('\n)', i)
    except ValueError:
        return {}
    block = src[i:j]
    if 'exitOK' not in block and 'exitFail' not in block:
        return {}
    return dict((m.group(1), int(m.group(2))) for m in CONST_RE.finditer(block))


def bare_numeric_returns(root):
    """命令面源码里的**裸数字**退出码（`return N` / `os.Exit(N)`）→ [(file, N)]。"""
    d = os.path.join(root, 'core', 'cmd', 'zerg')
    out = []
    for name in sorted(os.listdir(d)):
        if not name.endswith('.go') or name.endswith('_test.go'):
            continue
        for m in BARE_RET_RE.finditer(read(os.path.join(d, name))):
            out.append((name, int(m.group(1))))
    return out


def load_matrix(root):
    p = os.path.join(root, MATRIX_REL)
    if not os.path.isfile(p):
        return None
    try:
        return json.loads(read(p))
    except json.JSONDecodeError:
        return None


def load_baseline(root):
    p = os.path.join(root, BASELINE_REL)
    if not os.path.isfile(p):
        return None
    try:
        return json.loads(read(p))
    except json.JSONDecodeError:
        return None


def registry_gate_names(root):
    p = os.path.join(root, REGISTRY_REL)
    if not os.path.isfile(p):
        return None
    try:
        d = json.loads(read(p))
    except json.JSONDecodeError:
        return None
    return [e.get('gate', '') for e in d.get('entries', [])]


def check(root, no_runtime=False):
    """返回 (rc, 行列表)。rc：0 全过 · 1 有红 · 2 不给结论。"""
    lines = []
    errs = []

    tree = parse_tree(root)
    matrix = load_matrix(root)
    table = exitcode_table(root)
    consts = named_consts(root)
    baseline = load_baseline(root)

    if not tree:
        return 2, ['⛔ 不给结论：命令树一条命令都没读到（T1 空转 = 假覆盖）']
    if matrix is None or not matrix.get('cases'):
        return 2, ['⛔ 不给结论：must-fail 矩阵缺失或为空（%s）（T2 空转 = 假覆盖）' % MATRIX_REL]
    if not table or not consts:
        return 2, ['⛔ 不给结论：退码表或退码常量没读到（T3 空转 = 假覆盖）']
    if baseline is None or 'blanks' not in baseline:
        return 2, ['⛔ 不给结论：baseline 棘轮文件缺失/不可解析（%s）—— 「清到 0 再升阻断」的锚点没了' % BASELINE_REL]

    cases = matrix['cases']
    exemptions = matrix.get('exemptions', [])
    for e in exemptions:
        if not str(e.get('reason', '')).strip():
            return 2, ['⛔ 不给结论：豁免 %r **没有 reason** —— 写不清理由的豁免就是洗白（§九 M17 T5）'
                       % e.get('command', '?')]

    tree_paths = [p for p, _ in tree]
    by_cmd = {}
    for c in cases:
        by_cmd.setdefault(c.get('command', ''), []).append(c)

    # ---- T1 覆盖 ----
    blanks = sorted([p for p in tree_paths if p not in by_cmd and p not in
                     set(e.get('command') for e in exemptions)])
    for p in blanks:
        errs.append('T1 覆盖：命令 %r 既没有 case 也没有带 reason 的豁免' % p)
    ghosts = sorted([c for c in by_cmd if c not in tree_paths])
    for g in ghosts:
        errs.append('T1 覆盖：矩阵里的 %r 是**幽灵条目**（命令树里没有这条命令）' % g)

    # ---- T2 must-fail ----
    n_mustfail = 0
    for c in cases:
        try:
            rc = int(c.get('want_rc'))
        except (TypeError, ValueError):
            errs.append('T2 must-fail：case %r 的 want_rc 不是整数' % c.get('id', '?'))
            continue
        if rc == 0:
            errs.append('T2 must-fail：case %r 的 want_rc = 0 —— must-fail 矩阵不许有「该绿」的条目'
                        % c.get('id', '?'))
            continue
        n_mustfail += 1
    for p in tree_paths:
        if p in by_cmd and not [c for c in by_cmd[p] if int(c.get('want_rc', 0)) != 0]:
            errs.append('T2 must-fail：命令 %r 一条 must-fail 都没有' % p)

    # ---- T3 退码表唯一 ----
    codes = [r[0] for r in table]
    seen = {}
    for code, name, _ in table:
        if code in seen:
            errs.append('T3 退码表：**同数字两义** —— %d 同时是 %r 与 %r（§十二 P-013 占号纪律）'
                        % (code, seen[code], name))
        seen.setdefault(code, name)
    legal = set(codes)
    for nm, val in sorted(consts.items(), key=lambda kv: kv[1]):
        if val not in legal:
            errs.append('T3 退码表：常量 exit%s = %d **不在真源表里**（表里只有 %s）'
                        % (nm, val, sorted(legal)))
    if set(consts.values()) != legal:
        errs.append('T3 退码表：常量集 %s ≠ 表内数字集 %s（两处真源开始漂了）'
                    % (sorted(set(consts.values())), sorted(legal)))
    for f, n in bare_numeric_returns(root):
        if n not in legal:
            errs.append('T3 退码表：%s 里出现**裸数字**退出码 %d —— 它不在真源表里（该走具名常量）' % (f, n))

    # ---- T5 空白 + baseline 棘轮 ----
    if len(blanks) > int(baseline['blanks']):
        errs.append('T5 棘轮：空白 %d 条 > baseline %d 条（棘轮只许**下调**；新增命令必须补 must-fail）'
                    % (len(blanks), int(baseline['blanks'])))

    # ---- T6 引用 M6 R7 ----
    gates = registry_gate_names(root)
    if gates is None:
        return 2, ['⛔ 不给结论：契约登记表读不出来（%s）—— T6 的引用面无法判' % REGISTRY_REL]
    if not any('check-cli-contract.py' in g for g in gates):
        errs.append('T6 引用：契约登记表里**没有**任何条目的 gate 字段点名 check-cli-contract.py'
                    '（引用面 = 0 ⇒ 本门挂在空气上）')
    if 'M6 R7' not in read(__file__):
        errs.append('T6 引用：本门头部没有引用 §九 M6 R7（本门不重写判据，只引用它）')

    # ---- T4 `--json` 形状守卫（运行时只读） ----
    t4 = '（--no-runtime：夹具模式跳过）'
    if not no_runtime:
        try:
            r = subprocess.run(['go', 'test', './cmd/zerg/', '-run', 'TestCLIContractJSONShape',
                                '-count=1'], cwd=os.path.join(root, 'core'),
                               capture_output=True, text=True)
        except FileNotFoundError:
            return 2, ['⛔ 不给结论：T4 的运行时守卫跑不起来（机器上没有 go）—— 「跑不起来」不许当绿']
        if r.returncode != 0:
            # 「判不了」与「判红了」必须分开：跑不起来（没有 go.mod / 没有包 / 没有 go）⇒ **2 不给结论**；
            # 真跑起来而断言失败 ⇒ 才是红。
            se = r.stderr or ''
            if any(k in se for k in ('go.mod file not found', 'cannot find package', 'no such file or directory',
                                     'is not in std', 'executable file not found', 'no Go files')):
                why = se.strip().splitlines()[0] if se.strip() else 'go test 跑不起来'
                return 2, ['⛔ 不给结论：T4 的运行时守卫在这棵树上跑不起来（%s）—— 「跑不起来」不许当绿' % why]
            errs.append('T4 `--json` 形状守卫：go test 失败 —— 包封形状被改了（§九 M6 I1/I2/I3 · R7）\n'
                        + (r.stdout or '')[-1500:] + se[-500:])
        t4 = 'go test -run TestCLIContractJSONShape ⇒ rc=0'

    lines.append('命令树 %d 条 · 矩阵 case %d 条（must-fail %d）· 空白 %d 条 · 豁免 %d 条'
                 % (len(tree_paths), len(cases), n_mustfail, len(blanks), len(exemptions)))
    lines.append('退码表 %d 格（数字唯一：%s）· 常量 %d 个 · 裸数字退出码 %d 处'
                 % (len(table), '是' if len(set(codes)) == len(codes) else '否',
                    len(consts), len(bare_numeric_returns(root))))
    lines.append('T4 ' + t4)
    lines.append('T6 契约登记表 gate 字段点名本脚本：%s'
                 % ('是' if any('check-cli-contract.py' in g for g in gates) else '否'))
    for e in errs:
        lines.append('✗ ' + e)
    return (1 if errs else 0), lines


# ── 自检（成对负控：先证明这台机器能把红判出来，再谈「全过」）────────────────────────────
GOOD_MAIN = '''package main

const (
	exitOK          = 0
	exitFail        = 1
	exitUsage       = 2
	exitInterrupted = 130
)

var commands []*command

func init() {
	commands = []*command{
		{
			path:    []string{"version"},
			fields:  []string{"name"},
			endpoint: "",
			run:     cmdVersion,
		},
		{
			path:   []string{"core", "stop"},
			danger: &dangerSpec{dangerD3, "主机名", "停主控", "§三 C 族"},
			run:    cmdGuarded,
		},
	}
}
'''
GOOD_EXITCODES = '''package main

type exitcodeRow struct {
	Code      int
	Name      string
	Meaning   string
	Retryable string
}

var exitcodeTable = []exitcodeRow{
	{0, "ok", "成功", "—"},
	{1, "failed", "失败", "可重试"},
	{2, "usage", "用法错", "—"},
}

var exitcodeReserved = []exitcodeRow{
	{130, "interrupted", "人打断", "—"},
}
'''
GOOD_MATRIX = {
    "id": "cli-contract.v1",
    "exemptions": [],
    "cases": [
        {"id": "version#未知旗标", "command": "version", "argv": ["version", "--x"],
         "want_rc": 2, "want_stdout_bytes": 0, "why": "未知旗标 = 用法错"},
        {"id": "core/stop#缺 --confirm", "command": "core stop", "argv": ["core", "stop", "x"],
         "want_rc": 2, "want_stdout_bytes": 0, "why": "D3 缺确认 ⇒ 2"},
    ],
}
GOOD_BASELINE = {"id": "cli-contract-baseline.v1", "blanks": 0,
                 "note": "只许下调：本门落地时空白已清零 ⇒ 从此任何空白即红"}
GOOD_REGISTRY = {"schema": "zerg-contract-registry/1", "entries": [
    {"id": "S-g", "name": "命令面契约（本件）",
     "gate": "scripts/gates/check-error-kinds.py · scripts/gates/check-cli-contract.py"},
]}


def fixture(root, main=GOOD_MAIN, exitcodes=GOOD_EXITCODES, matrix=GOOD_MATRIX,
            baseline=GOOD_BASELINE, registry=GOOD_REGISTRY, extra_cmd=None):
    for rel, body in ((MAIN_REL, main), (EXITCODES_REL, exitcodes)):
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, 'w', encoding='utf-8') as f:
            f.write(body)
    p = os.path.join(root, MATRIX_REL)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8') as f:
        json.dump(matrix, f, ensure_ascii=False)
    p = os.path.join(root, BASELINE_REL)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8') as f:
        json.dump(baseline, f, ensure_ascii=False)
    p = os.path.join(root, REGISTRY_REL)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8') as f:
        json.dump(registry, f, ensure_ascii=False)
    if extra_cmd:
        with open(os.path.join(root, MAIN_REL), 'a', encoding='utf-8') as f:
            f.write(extra_cmd)
    return root


def self_test():
    bad = []

    def case(name, want, got, extra=''):
        if want != got:
            bad.append('%s：要 rc=%d，实际 rc=%d %s' % (name, want, got, extra))

    tmp = tempfile.mkdtemp(prefix='clicontract-')
    try:
        # ① 正控：好夹具 ⇒ 0
        r = fixture(os.path.join(tmp, 'good'))
        rc, lines = check(r, no_runtime=True)
        case('① 正控（好夹具 ⇒ 0）', 0, rc, '\n'.join(lines))

        # ② T1 负控：命令树多一条（`agent ls`）而矩阵没它 ⇒ 1 且判词点名
        r = fixture(os.path.join(tmp, 't1'),
                    extra_cmd='\n// 追加：矩阵里没有它的 must-fail\nvar extra = []*command{}\n')
        with open(os.path.join(r, MAIN_REL), encoding='utf-8') as f:
            body = f.read()
        body = body.replace('\t}\n}\n', '\t\t{path: []string{"agent", "ls"}, fields: []string{"machine"}, run: cmdAgentLs},\n\t}\n}\n')
        with open(os.path.join(r, MAIN_REL), 'w', encoding='utf-8') as f:
            f.write(body)
        rc, lines = check(r, no_runtime=True)
        case('② T1 负控（树里有命令、矩阵没 case ⇒ 1）', 1, rc)
        if not any('T1 覆盖' in l and 'agent ls' in l for l in lines):
            bad.append('② T1 负控：判词没有点名缺 case 的那条命令')

        # ③ T2 负控：一条 case 的 want_rc 改成 0 ⇒ 1
        m = json.loads(json.dumps(GOOD_MATRIX))
        m['cases'][0]['want_rc'] = 0
        rc, lines = check(fixture(os.path.join(tmp, 't2'), matrix=m), no_runtime=True)
        case('③ T2 负控（want_rc=0 ⇒ 1）', 1, rc)
        if not any('T2 must-fail' in l for l in lines):
            bad.append('③ T2 负控：判词里没有 T2')

        # ④ T1 负控b：幽灵条目（矩阵点名树里没有的命令）⇒ 1
        m = json.loads(json.dumps(GOOD_MATRIX))
        m['cases'].append({"id": "ghost#x", "command": "no such", "argv": ["no", "such"],
                           "want_rc": 2, "want_stdout_bytes": 0, "why": "幽灵"})
        rc, lines = check(fixture(os.path.join(tmp, 't1b'), matrix=m), no_runtime=True)
        case('④ T1 负控（幽灵条目 ⇒ 1）', 1, rc)
        if not any('幽灵' in l for l in lines):
            bad.append('④ T1 负控：判词里没有「幽灵」')

        # ⑤ T3 负控：表内同数字两义 ⇒ 1
        rc, lines = check(fixture(os.path.join(tmp, 't3'),
                                  exitcodes=GOOD_EXITCODES.replace('{2, "usage", "用法错", "—"},',
                                                                   '{0, "usage", "用法错", "—"},')),
                          no_runtime=True)
        case('⑤ T3 负控（同数字两义 ⇒ 1）', 1, rc)
        if not any('同数字两义' in l for l in lines):
            bad.append('⑤ T3 负控：判词里没有「同数字两义」')

        # ⑥ T3 负控b：裸数字退出码不在表里 ⇒ 1
        r = fixture(os.path.join(tmp, 't3b'))
        with open(os.path.join(r, MAIN_REL), 'a', encoding='utf-8') as f:
            f.write('\nfunc bogus() int { return 77 }\n')
        rc, lines = check(r, no_runtime=True)
        case('⑥ T3 负控（裸数字 77 ⇒ 1）', 1, rc)
        if not any('裸数字' in l for l in lines):
            bad.append('⑥ T3 负控：判词里没有「裸数字」')

        # ⑦ T5 负控：豁免没有 reason ⇒ **rc=2**（不给结论，不是红）
        m = json.loads(json.dumps(GOOD_MATRIX))
        m['exemptions'] = [{"command": "core stop", "reason": ""}]
        rc, lines = check(fixture(os.path.join(tmp, 't5'), matrix=m), no_runtime=True)
        case('⑦ T5 负控（豁免无 reason ⇒ 2）', 2, rc)
        if not any('没有 reason' in l for l in lines):
            bad.append('⑦ T5 负控：判词里没有「没有 reason」')

        # ⑧ T5 负控b：baseline 缺失 ⇒ 2（棘轮锚点没了）
        r = fixture(os.path.join(tmp, 't5b'))
        os.remove(os.path.join(r, BASELINE_REL))
        rc, lines = check(r, no_runtime=True)
        case('⑧ T5 负控（baseline 缺失 ⇒ 2）', 2, rc)

        # ⑨ T4 负控：**打开**运行时守卫、而夹具里跑不了 go test ⇒ 2（「跑不起来」不许当绿）
        rc, lines = check(fixture(os.path.join(tmp, 't4')), no_runtime=False)
        case('⑨ T4 负控（运行时守卫跑不起来 ⇒ 2）', 2, rc)
        if not any('T4' in l for l in lines):
            bad.append('⑨ T4 负控：判词里没有 T4')

        # ⑩ T6 负控：登记表 gate 字段不点名本脚本 ⇒ 1
        reg = json.loads(json.dumps(GOOD_REGISTRY))
        reg['entries'][0]['gate'] = 'scripts/gates/check-error-kinds.py'
        rc, lines = check(fixture(os.path.join(tmp, 't6'), registry=reg), no_runtime=True)
        case('⑩ T6 负控（引用面 = 0 ⇒ 1）', 1, rc)
        if not any('T6 引用' in l for l in lines):
            bad.append('⑩ T6 负控：判词里没有 T6')

        # ⑪ 空转负控：矩阵为空 ⇒ 2
        m = json.loads(json.dumps(GOOD_MATRIX))
        m['cases'] = []
        rc, lines = check(fixture(os.path.join(tmp, 'empty'), matrix=m), no_runtime=True)
        case('⑪ 空转负控（矩阵空 ⇒ 2）', 2, rc)

        print('自检结论: %s（失败 %d 条）' % ('全过' if not bad else '不过', len(bad)))
        for b in bad:
            print('✗ ' + b)
        return 0 if not bad else 2
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument('root', nargs='?', default='.')
    ap.add_argument('--self-test', action='store_true')
    ap.add_argument('--no-runtime', action='store_true',
                    help='**只给夹具用**：跳过 T4 的 go test（真目标不许带这个开关）')
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = find_root(a.root)
    if root is None:
        print('⛔ 不给结论：找不到仓根（从 %s 上溯找不到 %s）' % (a.root, MAIN_REL))
        return 2
    rc, lines = check(root, no_runtime=a.no_runtime)
    print('门⑪ 命令面契约（§九 M17 T1–T6）· 仓根 %s' % root)
    for l in lines:
        print('  ' + l)
    if rc == 0:
        print('⇒ 命令面契约全过（rc=0）')
    elif rc == 1:
        print('⇒ 命令面契约判红（rc=1）：逐条看上面的 T-编号')
    else:
        print('⇒ 不给结论（rc=2）：缺件/空转/判不了 —— **不许当绿**')
    return rc


if __name__ == '__main__':
    sys.exit(main())

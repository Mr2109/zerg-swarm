#!/usr/bin/env python3
# check-service-declaration.py — 门⑧：服务声明树（现值 vs 声明 的差集）——§十五.3 对象清册 ·
# §二十一 已红第 3 条 · 开工单 T-25。
#
# 为什么要有它（已红第 3 条的原话口径）：幽灵服务的判据**不是 `PPID==1`**（本机三个自研守护件全是
#   `PPID=1`，那是 launchd 收养的正常形态）⇒ 判据只能是「**声明的对象集**与**实际的进程集**的差集」。
#   今天只能靠人肉 `ps`/`lsof` 撞见 —— 本门把它变成每次提交都跑一遍的机器判据。
#
# 六条规则（任一不通过 ⇒ 见下方退出码）：
#   R1 声明树可解析且非空（`deploy/服务声明.tsv`；缺件 / 解析不出行 / 无 launchd 行 ⇒ **rc=2 不给结论**）。
#   R2 `launchctl list | grep -i zerg` 的**行数 == 声明件数**（= `kind=launchd` 的行数）——
#      这就是开工单 T-25 判据②；不等 ⇒ rc=1（多一件 = 幽灵没登记；少一件 = 声明悬空）。
#   R3 每条 launchd 声明的「声明件」（`~/…` 形态）在盘上真存在 ⇒ 否则 rc=1。
#   R4 **现值进程逐条有落点**：ps 里每个自研件进程（命令行含 `bin/zerg-` 或 `bin/cocoon-`）
#      都必须被某条「进程特征」覆盖（= 在声明面可查），或本身就是一条已登记的 ghost（= 进 reap 候选）；
#      覆盖不上的 ⇒ rc=1（**未登记幽灵** —— 正是已红第 3 条的形态）。
#   R5 每条 ghost **必须写处置**（处置列含 `reap` 或 `待`）⇒ 否则 rc=1。
#   R6 **空转 = 假覆盖**：ps 里一件自研件都扫不到 ⇒ rc=2（不当绿）。
#
# 用法：
#   python3 scripts/gates/check-service-declaration.py            # 真目标（本机现值）
#   python3 scripts/gates/check-service-declaration.py --self-test # 成对负控（1 正 + 5 反 + 1 空转）
#
# 取证夹具（自检用；也便于在别的机器上复跑）：
#   ZERG_SVCDECL_DECL     声明树路径（默认 <仓根>/deploy/服务声明.tsv）
#   ZERG_SVCDECL_PS       ps 输出落文件（默认真跑 `ps -eo pid,ppid,etime,command`）
#   ZERG_SVCDECL_LAUNCHCTL launchctl 输出落文件（默认真跑 `launchctl list`）
#   ZERG_SVCDECL_HOME     `~/` 展开用的家目录（默认真家目录；自检指向临时目录）
import argparse
import os
import re
import subprocess
import sys
import tempfile

TOOL = 'check-service-declaration'
KINDS = ('launchd', 'ghost')
# 自研件的进程特征前缀（仓根相对）——两种都算「本仓的件」
PROC_HINTS = ('bin/zerg-', 'bin/cocoon-')
SELF_NAME = os.path.basename(__file__)


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


def parse_decl(path):
    """解析声明树 ⇒ (rows, problems)。行 = dict(kind,name,decl,prog,proc,owner,disposition)。"""
    rows, problems = [], []
    if not os.path.isfile(path):
        return None, ['缺件：声明树不存在 %s（判据不可判 ⇒ 不给结论）' % path]
    with open(path, encoding='utf-8') as f:
        for ln, line in enumerate(f, 1):
            line = line.rstrip('\n')
            if not line.strip() or line.lstrip().startswith('#'):
                continue
            if line.startswith('kind\t'):          # 表头
                continue
            cells = line.split('\t')
            if len(cells) < 7:
                problems.append('行 %d 列数不足（要 7 列，得到 %d）：%s' % (ln, len(cells), line[:60]))
                continue
            kind = cells[0].strip()
            if kind not in KINDS:
                problems.append('行 %d 的 kind=%r 不在闭集 %s 内' % (ln, kind, list(KINDS)))
                continue
            rows.append({'line': ln, 'kind': kind, 'name': cells[1].strip(),
                         'decl': cells[2].strip(), 'prog': cells[3].strip(),
                         'proc': cells[4].strip(), 'owner': cells[5].strip(),
                         'disposition': cells[6].strip()})
    return rows, problems


def read_lines(path, cmd):
    """优先读夹具文件；否则真跑命令。返回 (lines, mode, err)。"""
    if path:
        if not os.path.isfile(path):
            return [], 'fixture', '夹具文件不存在：%s' % path
        with open(path, encoding='utf-8') as f:
            return [l.rstrip('\n') for l in f if l.strip()], 'fixture', None
    try:
        out = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=30)
    except Exception as e:                                    # noqa: BLE001
        return [], 'live', '%s 跑不起来：%s' % (cmd, e)
    if out.returncode != 0:
        return [], 'live', '%s 退出码 %d：%s' % (cmd, out.returncode, (out.stderr or '').strip()[:120])
    return [l for l in out.stdout.splitlines() if l.strip()], 'live', None


def check(root, decl_path=None, ps_path=None, launchctl_path=None, home=None):
    decl_path = decl_path or os.environ.get('ZERG_SVCDECL_DECL') or os.path.join(root, 'deploy', '服务声明.tsv')
    ps_path = ps_path if ps_path is not None else os.environ.get('ZERG_SVCDECL_PS')
    lc_path = launchctl_path if launchctl_path is not None else os.environ.get('ZERG_SVCDECL_LAUNCHCTL')
    home = home or os.environ.get('ZERG_SVCDECL_HOME') or os.path.expanduser('~')

    problems, notes = [], []
    rows, perr = parse_decl(decl_path)
    if rows is None:
        return 2, list(perr), [], ''
    problems.extend(perr)

    launchd_rows = [r for r in rows if r['kind'] == 'launchd']
    ghost_rows = [r for r in rows if r['kind'] == 'ghost']

    # R1 空转/不可判
    if not launchd_rows:
        return 2, ['空转：声明树里 0 条 launchd 声明（判据不可判 ⇒ 不给结论）'], [], ''
    if not ghost_rows:
        notes.append('声明树里 0 条 ghost（现值若真无幽灵，这是正常态；有幽灵会被 R4 抓住）')

    # 现值：ps / launchctl
    ps_lines, psmode, ps_err = read_lines(ps_path, 'ps -eo pid,ppid,etime,command')
    lc_lines, lcmode, lc_err = read_lines(lc_path, 'launchctl list')
    if ps_err:
        return 2, ['不可判：ps 取不到（%s）' % ps_err], [], ''
    if lc_err:
        return 2, ['不可判：launchctl 取不到（%s · 非 macOS 机器属正常 ⇒ 本门不给结论、不当绿）'
                   % lc_err], [], ''
    live = [l for l in ps_lines if any(h in l for h in PROC_HINTS)]
    if not live:
        return 2, ['空转：ps 里一件自研件（%s…）都没扫到（不许当绿）' % PROC_HINTS[0]], [], ''

    # R2 判据②：launchctl 行数 == 声明件数
    lc_zerg = [l for l in lc_lines if 'zerg' in l.lower()]
    notes.append('launchctl 里 zerg 行数 = %d · 声明件数（kind=launchd）= %d' % (len(lc_zerg), len(launchd_rows)))
    if len(lc_zerg) != len(launchd_rows):
        if len(lc_zerg) > len(launchd_rows):
            problems.append('FAIL R2 launchctl 有 %d 行 zerg 作业、声明树只有 %d 条 ⇒ 差 %d 件没声明（幽灵）'
                            % (len(lc_zerg), len(launchd_rows), len(lc_zerg) - len(launchd_rows)))
        else:
            problems.append('FAIL R2 声明树 %d 条 launchd、launchctl 只 %d 行 ⇒ 声明悬空（盘上没装载或已失效）'
                            % (len(launchd_rows), len(lc_zerg)))

    # R3 声明件在盘上存在
    for r in launchd_rows:
        p = r['decl']
        exp = os.path.join(home, p[2:]) if p.startswith('~/') else p
        if not os.path.exists(exp):
            problems.append('FAIL R3 行 %d 的声明件不存在：%s ⇒ 声明悬空（%s 是盘上真源）'
                            % (r['line'], p, exp.replace(home, '~')))

    # R4 现值进程逐条有落点
    covered = []
    for ln in live:
        hit = [r for r in rows if r['proc'] and r['proc'] != '-' and r['proc'] in ln]
        if hit:
            covered.append((ln, hit[0]))
            continue
        problems.append('FAIL R4 现值进程没有落点（**未登记幽灵**）：%s' % ln.strip()[:120])

    # R5 ghost 必须写处置
    for r in ghost_rows:
        if not re.search(r'reap|待', r['disposition']):
            problems.append('FAIL R5 行 %d 的 ghost（%s）没有处置：处置列必须含 `reap` 或 `待`（登记了却不处置 = 换个地方藏）'
                            % (r['line'], r['name']))

    rc = 0 if not problems else 1
    return rc, problems, notes, 'ps=%s · launchctl=%s' % (psmode, lcmode)


def print_report(root, rc, problems, notes, modes, decl_path):
    print('── 门⑧ 服务声明树：现值（ps/launchctl）vs 声明（deploy/服务声明.tsv）差集（§十五.3 · 已红第 3 条/T-25）──')
    print('  声明树: %s' % decl_path.replace(root, '<仓根>'))
    print('  取证:   %s' % modes)
    for n in notes:
        print('  · %s' % n)
    for p in problems:
        print('  · %s' % p)
    if rc == 0:
        print('⇒ 全绿（rc=0）：现值每一条都在声明面可查，或已登记进 reap 候选')
    elif rc == 1:
        print('⇒ 门禁红（rc=1）：现值与声明的差集里有没落点的件')
    else:
        print('⇒ 不给结论（rc=2）')
    return rc


# ─────────────────────────── 自检（成对负控）───────────────────────────
GOOD_DECL = (
    '# 夹具声明树\n'
    'kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n'
    'launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n'
    'launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n'
    'ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧\t进 reap 候选 · 待 Mr2109 拍\n'
)
GOOD_PS = (
    '  68939     1       56:36 /repo/bin/zerg-core\n'
    '  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n'
    '  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n'
)
GOOD_LC = '68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n'


def _fixture(d, decl=GOOD_DECL, ps=GOOD_PS, lc=GOOD_LC, plists=('com.zerg.core.plist', 'com.zerg.agent-Mr2109.plist')):
    dp = os.path.join(d, '服务声明.tsv')
    with open(dp, 'w', encoding='utf-8') as f:
        f.write(decl)
    pp = os.path.join(d, 'ps.txt')
    with open(pp, 'w', encoding='utf-8') as f:
        f.write(ps)
    lp = os.path.join(d, 'launchctl.txt')
    with open(lp, 'w', encoding='utf-8') as f:
        f.write(lc)
    home = os.path.join(d, 'home')
    la = os.path.join(home, 'Library', 'LaunchAgents')
    os.makedirs(la, exist_ok=True)
    for name in plists:
        with open(os.path.join(la, name), 'w', encoding='utf-8') as f:
            f.write('<plist/>\n')
    return dp, pp, lp, home


def self_test():
    cases = []

    def mk(d, **kw):
        return _fixture(d, **kw)

    cases.append(('正控① 好件：2 条 launchd + 1 条已处置 ghost，ps 3 行全有落点 ⇒ rc=0', 0, None))
    cases.append(('负控② ps 多一件未登记进程（zerg-wall）⇒ rc=1', 1, 'ps'))
    cases.append(('负控③ launchctl 多一行（3 行 vs 2 条声明）⇒ rc=1', 1, 'lc'))
    cases.append(('负控④ ghost 没写处置（处置列空）⇒ rc=1', 1, 'disposition'))
    cases.append(('负控⑤ 声明件（plist）不在盘上 ⇒ rc=1', 1, 'plist'))
    cases.append(('负控⑥ ps 里一件自研件都没有 ⇒ rc=2（空转不许当绿）', 2, 'empty_ps'))
    cases.append(('负控⑦ 声明树缺件 ⇒ rc=2（判据不可判）', 2, 'no_decl'))

    ok = True
    for name, want, mode in cases:
        with tempfile.TemporaryDirectory() as d:
            kw = {}
            if mode == 'ps':
                kw['ps'] = GOOD_PS + '  99999     1       01:00 /repo/bin/zerg-wall\n'
            elif mode == 'lc':
                kw['lc'] = GOOD_LC + '55555\t0\tcom.zerg.ghost-unregistered\n'
            elif mode == 'disposition':
                kw['decl'] = GOOD_DECL.replace('进 reap 候选 · 待 Mr2109 拍', '-')
            elif mode == 'plist':
                kw['plists'] = ('com.zerg.core.plist',)
            elif mode == 'empty_ps':
                kw['ps'] = '    1     0  00:00 /sbin/launchd\n'
            dp, pp, lp, home = mk(d, **kw)
            if mode == 'no_decl':
                dp = os.path.join(d, '不存在.tsv')
            rc, problems, notes, _ = check(d, decl_path=dp, ps_path=pp, launchctl_path=lp, home=home)
            flag = '✓' if rc == want else '✗'
            print('  %s %s（rc=%d）' % (flag, name, rc))
            if rc != want:
                ok = False
                for p in problems:
                    print('      · %s' % p)
    print('自检结论: ' + ('全过' if ok else '**有断言失败**'))
    return 0 if ok else 1


def main():
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
    decl = os.environ.get('ZERG_SVCDECL_DECL') or os.path.join(root, 'deploy', '服务声明.tsv')
    rc, problems, notes, modes = check(root)
    return print_report(root, rc, problems, notes, modes, decl)


if __name__ == '__main__':
    sys.exit(main())

#!/usr/bin/env python3
# check-service-declaration.py — 门⑧：服务声明树（现值 vs 声明 的差集）——§十五.3 对象清册 ·
# §二十一 已红第 3 条 · 开工单 T-25。
#
# 为什么要有它（已红第 3 条的原话口径）：幽灵服务的判据**不是 `PPID==1`**（本机三个自研守护件全是
#   `PPID=1`，那是 launchd 收养的正常形态）⇒ 判据只能是「**声明的对象集**与**实际的进程集**的差集」。
#   今天只能靠人肉 `ps`/`lsof` 撞见 —— 本门把它变成每次提交都跑一遍的机器判据。
#
# ★ 层限（GAP-20260927-168）：本门只判**本机层**声明的行（`layer=host`）—— 读的是**本机** `launchctl`/`ps`。
#   声明树（`deploy/服务声明.tsv`）的 `layer` 列标明每行属哪一层；非本机层（如 `node:edge-node`）的行
#   **不由本门判**（只计数 · 各层自记）⇒ 防「本机声明树一致」被读成「全机群服务声明一致」。
#   缺 `layer` 列（旧 7 列件）⇒ 全部按 `host` 解析，并出一条注记（建议补列）。
#
# 六条规则（任一不通过 ⇒ 见下方退出码）：
#   R1 声明树可解析且非空（`deploy/服务声明.tsv`；缺件 / 解析不出行 / 无 launchd 行 ⇒ **rc=2 不给结论**）。
#   R2 **按名字集对拍**：`launchctl list` 的 `com.zerg.` 标签集 ⇄ 声明树 `kind=launchd` 的 `name` 集，
#      **双向**点名（`A∖B` = 盘上有·声明无 ⇒ 幽灵；`B∖A` = 声明有·盘上无 ⇒ 悬空）⇒ 差集非空即 rc=1。
#      ★ 这是开工单 T-25 判据②（「`launchctl list | grep -i zerg` 的行数与声明件数一致」）的**加强形**：
#        行数相等只是集合相等的必要非充分条件 —— 「少 2 条声明 + 多 2 条隧道」恰得 5 == 5 ⇒
#        只比行数在「数目相同·换了人」这一格恒真假绿（`GAP-20260927-100`）。
#      ★ 已登记 `kind=ghost` 的件**不算幽灵**（那是人签设计：现值有·声明无的**已登记**幽灵）⇒
#        只有「**未登记**幽灵」判红；R2 必须把这两态逐条分开（点名时各自标注）。
#   R3 每条 launchd 声明的「声明件」（`~/…` 形态）在盘上真存在 ⇒ 否则 rc=1。
#   R4 **现值进程逐条有落点**：ps 里每个自研件/外置件进程（命令行含 `bin/zerg-` 或 `bin/cocoon-` 或 `searx.webapp`）
#      都必须被某条「进程特征」覆盖（= 在声明面可查），或本身就是一条已登记的 ghost（= 进 reap 候选）；
#      覆盖不上的 ⇒ rc=1（**未登记幽灵** —— 正是已红第 3 条的形态）。
#      ★ 2026-10-02（`GAP-20260927-100` R4）：本规则分两侧 —— **R4a 作业面驱动**（以 `launchctl list`
#      的 **pid 列**取现值：有 pid 的作业即现值，用 pid 去 ps 取**那一条**命令行，再对声明行「进程特征」
#      做**词界锚** —— 抓「标签 ↔ pid ↔ 程序」不一致，现值驱动那一侧抓不到）＋ **R4b 现值驱动**（下面
#      这条：ps 里自研件进程逐条有落点）。**禁切词**：本仓路径含空格（`00 项目`），`split()[3]` 会把
#      argv[0] 截成 `<volume-path>`（实测）⇒ 锚一律走正则词界。
#   R5 每条 ghost **必须写处置**（处置列含 `reap` 或 `待`）⇒ 否则 rc=1。
#   R6 **空转 = 假覆盖**：ps 里一件自研件都扫不到 ⇒ rc=2（不当绿）。
#
# 用法：
#   python3 scripts/gates/check-service-declaration.py            # 真目标（本机现值）
#   python3 scripts/gates/check-service-declaration.py --self-test # 成对负控（20 格：正控 7 + 负控 11 + 不给结论 2 · 含 1 格真命令行退码）
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
# 自研件 / 外置件的进程特征（仓根相对 或 外置件命令行锚）——两类都算「本仓的件」
# ★ 2026-10-07（A2 收尾）：+ `searx.webapp` —— 外置 searxng（launchd `com.zerg.searxng`）进程
#   命令行 `… -m searx.webapp` 的锚；并入后 R4b **覆盖**它（未登记 ⇒ 红）。与 Go 真搬件同批。
PROC_HINTS = ('bin/zerg-', 'bin/cocoon-', 'searx.webapp')
# ★ 标签命名空间闭集（`L1` 口径 · 唯一一处）：为什么是**显式闭集**而不是 `'zerg' in label` 这类子串猜 ——
#   ① 子串猜会吞外来件（`com.example.zerg-probe` 被读成「本仓的件」⇒ 点到错人）；
#   ② 子串猜判不出「排除」（闭集判得出：闭集外的行属**未覆盖形态**，逐条打印而**不进集合**）；
#   ③ 判据要用「整标签」（第 3 列），不是整行（整行里的 PID/路径也含 `zerg` 字样 ⇒ 又一份口径）。
ZERG_LABEL_PREFIXES = ('com.zerg.',)
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
                         'disposition': cells[6].strip(),
                         # ★ 层限（GAP-20260927-168）：第 8 列 = layer（缺 ⇒ 空串 · 用时空串按 host 解析）
                         'layer': cells[7].strip() if len(cells) >= 8 else ''})
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


def parse_launchctl_labels(lines):
    """`launchctl list` 输出 ⇒ {label: 原始行}。★ 只取**第 3 列**（label）——
    PID / 退出状态在第 1/2 列，进集合必误报（`L1` 口径）⇒ 只做注记、不进集合。"""
    out = {}
    for ln in lines:
        cells = ln.split('\t')
        if len(cells) < 3:
            continue
        lab = cells[2].strip()
        if lab:
            out.setdefault(lab, ln.strip())
    return out


def parse_launchctl_jobs(lines):
    """`launchctl list` ⇒ {label: pid}（第 1 列 = PID；`-`/非数 ⇒ None = 未在跑）。
    ★ 与 `parse_launchctl_labels` 同源同一次读数 —— 唯一区别是这里要 **pid**（`GAP-20260927-100` R4a）。"""
    jobs = {}
    for ln in lines:
        cells = ln.split('\t')
        if len(cells) < 3:
            continue
        lab = cells[2].strip()
        if not lab:
            continue
        pid = cells[0].strip()
        jobs.setdefault(lab, pid if pid.isdigit() else None)
    return jobs


def ps_pid_to_command(lines):
    """`ps -eo pid,ppid,etime,command` ⇒ {pid: command 整串}（**禁切词**）。
    ★ 只按空白切**前三个字段**（pid/ppid/etime），第 4 字段起整串留给命令 —— 本仓路径含空格
      （`00 项目`），`split()[3]` 会把 argv[0] 截成 `<volume-path>`（实测 · `GAP-20260927-100` R4a）。"""
    out = {}
    for ln in lines:
        parts = ln.split(None, 3)
        if len(parts) >= 3 and parts[0].isdigit():
            out.setdefault(parts[0], parts[3] if len(parts) >= 4 else '')
    return out


def load_zerg_labels():
    """闭集内标签前缀（`GAP-20260927-107` 步② · **唯一常量**）：本仓 launchd label 命名空间。
    同族件要 label 口径一律 `import` 本函数，✗ 不许在别处再写一份 `'zerg' in label` 子串猜
    （子串猜会吞 `org.other.zergish` 这类外来件、也判不出「排除」）。"""
    return ZERG_LABEL_PREFIXES


def label_set_diff(launchd_rows, lc_lines):
    """★ `launchctl` 标签集 ⇄ 声明 `name` 集 的**唯一一处**集合比较（**别在别处再写一份**
    —— 同族件要按同源 import 本函数，写第二份即「第二份口径」）。
    ⇒ (live_labels, decl_labels, only_live, only_decl)：
      live_labels = 闭集内标签（label ⇒ 原始行 · 用于注记行号）；
      decl_labels = 声明 `name` ⇒ 声明行（用于给「件:行」）；
      only_live  = 盘上有·声明无（幽灵）；only_decl = 声明有·盘上无（悬空）。两侧**都逐条点名**。"""
    decl_labels = {r['name']: r for r in launchd_rows}
    live_labels = {lab: raw for lab, raw in parse_launchctl_labels(lc_lines).items()
                   if lab.startswith(load_zerg_labels())}
    only_live = sorted(set(live_labels) - set(decl_labels))
    only_decl = sorted(set(decl_labels) - set(live_labels))
    return live_labels, decl_labels, only_live, only_decl


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

    # ★ 层限（GAP-20260927-168）：本门只判**本机层**（`layer=host`）的行；非本机层行只计数、不由本门判。
    saw_layer_col = any(r.get('layer') for r in rows)
    host_rows = [r for r in rows if (r.get('layer') or 'host') == 'host']
    off_rows = [r for r in rows if (r.get('layer') or 'host') != 'host']
    launchd_rows = [r for r in host_rows if r['kind'] == 'launchd']
    ghost_rows = [r for r in host_rows if r['kind'] == 'ghost']

    # R0 层限自述（防「本机声明树一致」被读成「全机群服务声明一致」）
    if not saw_layer_col:
        notes.append('层限：声明树无 `layer` 列（旧 7 列件）⇒ 全部按 `host`（本机层）解析；'
                     '建议按 GAP-20260927-168 补 `layer` 列（标清每行属哪一层）')
    else:
        notes.append('R0 层限：本门判面 = `layer=host`（**本机层**）· 本层 %d 行 · 非本层 %d 行'
                     '（非本层**不由本门判** · 各层自记）' % (len(host_rows), len(off_rows)))

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

    # R2 判据②：`launchctl` 标签集 ⇄ 声明 `name` 集（**按名字集对拍** · 双向点名 · 不再只比行数）
    live_labels, decl_labels, only_live, only_decl = label_set_diff(launchd_rows, lc_lines)
    notes.append('R2 集合对拍：launchctl 闭集 %s 标签 %d 个 · 声明 name %d 个（行数 %s）'
                 % (' '.join('`%s`' % p for p in ZERG_LABEL_PREFIXES), len(live_labels), len(decl_labels),
                    '相等' if len(live_labels) == len(decl_labels) else '不等'))
    notes.append('R2 现值标签（%d）：%s' % (len(live_labels),
                 ' · '.join(sorted(live_labels)) or '（空）0 个'))
    notes.append('R2 声明标签（%d）：%s' % (len(decl_labels),
                 ' · '.join(sorted(decl_labels)) or '（空）0 个'))
    # 侧①「盘上有 · 声明无」——差集非空即逐条点名；★ 分「已登记 ghost」与「未登记幽灵」两态
    for lab in only_live:
        raw = live_labels[lab]
        reg = [g for g in ghost_rows if g['name'] == lab
               or (g['proc'] and g['proc'] != '-' and g['proc'] in raw)]
        if reg:
            notes.append('R2 `A∖B` 已登记：%s（`kind=ghost` 行「%s」· 声明树第 %d 行 · 处置=%.40s）'
                         '—— 已登记幽灵**不是**缺口（人签设计）'
                         % (lab, reg[0]['name'], reg[0]['line'], reg[0]['disposition']))
        else:
            problems.append('FAIL R2 `A∖B` **未登记幽灵**：launchctl 有标签 %s 而声明树 %d 条 launchd 里'
                            '查无此名 ⇒ 逐条点名（原始行：%s）' % (lab, len(launchd_rows), raw[:100]))
    # 侧②「声明有 · 盘上无」
    for lab in only_decl:
        r = decl_labels[lab]
        problems.append('FAIL R2 `B∖A` 声明悬空：声明树第 %d 行（`kind=launchd` · name=%s · 声明件 %s）'
                        '在 launchctl 闭集里没有同名标签 ⇒ 盘上没装载或已失效' % (r['line'], lab, r['decl']))
    if not only_live:
        notes.append('R2 `A∖B`（盘上有 · 声明无）：（空）0 个 —— 闭集内每个标签都在声明面可查')
    if not only_decl:
        notes.append('R2 `B∖A`（声明有 · 盘上无）：（空）0 个 —— 声明的每个 name 都有同名标签')
    # ★ 未覆盖形态（闭集外）逐条打印：`launchctl list` 只列当前 `gui/501` 域 ⇒ 系统域 LaunchDaemon 不在读数里，
    #   而闭集外的行**不进集合**（既不当幽灵、也不当绿）
    outside = [lab for lab in parse_launchctl_labels(lc_lines) if not lab.startswith(ZERG_LABEL_PREFIXES)]
    if outside:
        notes.append('R2 闭集外 %d 行（未覆盖形态 · 不进集合）：%s'
                     % (len(outside), ' · '.join(sorted(outside)[:8])))

    # R3 声明件在盘上存在
    for r in launchd_rows:
        p = r['decl']
        exp = os.path.join(home, p[2:]) if p.startswith('~/') else p
        if not os.path.exists(exp):
            problems.append('FAIL R3 行 %d 的声明件不存在：%s ⇒ 声明悬空（%s 是盘上真源）'
                            % (r['line'], p, exp.replace(home, '~')))

    # R4a 作业面驱动（`GAP-20260927-100` R4 · **pid 唯一锚**）：以 `launchctl list` 的 **pid 列**为作业面
    #   驱动 —— 有 pid 的作业即「现值」，用 pid（唯一、不认名字）去 ps 现读里取**那一条**命令行，再对
    #   声明行的「进程特征」列做**词界锚**。抓的是「标签 ↔ pid ↔ 程序」三者不一致（R4b 那一侧抓不到）。
    jobs = parse_launchctl_jobs(lc_lines)
    pid_cmd = ps_pid_to_command(ps_lines)
    for r in launchd_rows:
        pid = jobs.get(r['name'])
        if not pid:
            continue                      # 未在跑（`-`/空）⇒ 无现值可锚；悬空/幽灵由 R2 管
        cmd = pid_cmd.get(pid)
        if cmd is None:
            problems.append('FAIL R4 声明 %s 的作业 pid=%s 在 ps 现值里取不到命令行 ⇒ 锚不了'
                            '（ps 与 launchctl 不同源）' % (r['name'], pid))
            continue
        hint = r['proc']
        if hint and hint != '-' and not re.search(re.escape(hint) + r'(?=\s|$)', cmd):
            problems.append('FAIL R4 声明 %s（第 %d 行）的作业 pid=%s 命令行里找不到声明的「进程特征」'
                            '`%s` ⇒ 标签↔pid↔程序不一致（原始命令行：%s）'
                            % (r['name'], r['line'], pid, hint, cmd.strip()[:100]))

    # R4b 现值驱动：ps 里自研件进程逐条有落点
    covered = []
    for ln in live:
        # ★ 词界锚（`GAP-20260927-100` 面② · 禁子串猜）：声明行的「进程特征」必须作为**整词**出现在
        #   现值命令行里（后随空白或行尾）⇒ `bin/zerg-core` 不再被 `bin/zerg-core-test` 子串假绿。
        #   ✗ 不用 split()[3]：本仓路径含空格（`00 项目`）切词会把 argv[0] 截断 ⇒ 只用正则词界。
        hit = [r for r in host_rows if r['proc'] and r['proc'] != '-'
               and re.search(re.escape(r['proc']) + r'(?=\s|$)', ln)]
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
    'kind\tname\t声明件\t程序\t进程特征\t归属\t处置\tlayer\n'
    'launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\thost\n'
    'launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\thost\n'
    'ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧\t进 reap 候选 · 待 Mr2109 拍\thost\n'
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
    cases.append(('负控⑧ 数目同·名字不同（agent-Mr2109 ⇄ tunnel.mini-node-1）⇒ rc=1（**双向点名**）', 1, 'swapped'))
    cases.append(('负控⑨ 闭集外标签（com.example.zerg-probe）不进集合 ⇒ rc=0（另印「闭集外」行）', 0, 'outsider'))
    cases.append(('正控⑩ 闭集内标签但**已登记 ghost 行** ⇒ 不算幽灵（区分已知幽灵/未登记幽灵）', 0, 'ghost_reg'))
    cases.append(('负控⑪ 现值是 `bin/zerg-core-test`（≠ 声明的 `bin/zerg-core`）⇒ rc=1（**词界锚** · 杀子串假绿）', 1, 'sibling_name'))
    cases.append(('负控⑫ 闭集内新标签 `com.zerg.newthing`（声明无）⇒ rc=1（闭集内判得红；闭集外见正控⑨）', 1, 'newthing'))
    cases.append(('负控⑬ 声明 com.zerg.core 的作业 pid 上跑的**不是**声明的程序（标签↔pid↔程序不一致 · R4a）⇒ rc=1', 1, 'pid_mismatch'))
    cases.append(('负控⑭ 作业数不等（声明 2 条 launchd · launchctl 仅 1 行 ⇒ B∖A 悬空）⇒ rc=1', 1, 'count_less'))
    cases.append(('负控⑮ 瞬态工人（launchctl 有 pid · ps 已无此 pid ⇒ R4a 锚不了）⇒ rc=1', 1, 'transient'))
    cases.append(('正控⑯ 含字面 shell 包装（/bin/sh -c exec …）⇒ 词界锚仍命中 ⇒ rc=0', 0, 'shell_wrap'))
    cases.append(('正控⑰ 真命令行退码：子进程跑本件·好件夹具 ⇒ 进程 rc=0', 0, 'cli'))
    cases.append(('正控⑱ 非本层行（`layer=node:edge-node`：声明无同名标签、plist 也不在盘）⇒ **不由本门判** ⇒ rc=0（**层限** · GAP-20260927-168）', 0, 'offlayer'))
    cases.append(('正控⑲ searxng 外置进程（`searx.webapp`）有落点 ⇒ rc=0（R4b 覆盖 · hint 的牙）', 0, 'searxng_ok'))
    cases.append(('负控⑳ searxng 进程在 ps 里但声明面无落点 ⇒ rc=1（R4b 未登记幽灵 · 去掉该 hint 即看不见）', 1, 'searxng_leak'))

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
            elif mode == 'swapped':
                kw['lc'] = '68939\t0\tcom.zerg.core\n86298\t0\tcom.zerg.tunnel.mini-node-1\n'
            elif mode == 'outsider':
                kw['lc'] = GOOD_LC + '77777\t0\tcom.example.zerg-probe\n'
            elif mode == 'ghost_reg':
                kw['lc'] = GOOD_LC + '88888\t0\tcom.zerg.ghost-registered\n'
                kw['decl'] = GOOD_DECL + ('ghost\tcom.zerg.ghost-registered\t-\t-\t-\t测试\t待拍\n')
            elif mode == 'sibling_name':
                # 现值只有 `bin/zerg-core-test`：声明行的 proc 是 `bin/zerg-core` ⇒ 子串会假绿，词界锚必红
                kw['ps'] = ('  68939     1       56:36 /repo/bin/zerg-core-test\n'
                            '  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n'
                            '  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n')
            elif mode == 'newthing':
                kw['lc'] = GOOD_LC + '77777\t0\tcom.zerg.newthing\n'
            elif mode == 'pid_mismatch':
                # launchctl 说 com.zerg.core 的 pid=68939，而 ps 里 68939 跑的是 zerg-agentd ⇒ R4a 必红
                # （R4b 仍绿：agentd 行被 agent-Mr2109 的 proc 覆盖 ⇒ 这一格单测 R4a 的牙）
                kw['ps'] = ('  68939     1       56:36 /repo/bin/zerg-agentd\n'
                            '  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n'
                            '  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n')
            elif mode == 'count_less':
                kw['lc'] = '68939\t0\tcom.zerg.core\n'
            elif mode == 'transient':
                # launchctl 仍有 agent-Mr2109 的 pid=14149，但 ps 里那一条已经消失（瞬态工人）
                kw['ps'] = ('  68939     1       56:36 /repo/bin/zerg-core\n'
                            '  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n')
            elif mode == 'shell_wrap':
                # 命令行含字面 shell 包装（argv[0]=/bin/sh · -c 脚本）⇒ 禁切词/词界锚仍须命中
                kw['ps'] = ('  68939     1       56:36 /bin/sh -c exec bin/zerg-core\n'
                            '  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n'
                            '  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n')
            elif mode == 'offlayer':
                # ★ 层限（GAP-20260927-168）：追加一条**非本机层**（layer=node:edge-node）launchd 行 ——
                #   name 在 launchctl 里没有（若判则 B∖A 红）、plist 也不在盘（若判则 R3 红）。
                #   本门只判 layer=host ⇒ 该行**不由本门判** ⇒ rc 必为 0（层限的牙）。
                kw['decl'] = GOOD_DECL + ('launchd\tx3-agent.service\t/home/<user>/edge-node-agent.service\t'
                                          '/home/<user>/zerg/bin/zerg-agentd\tbin/zerg-agentd\t节点(edge-node)\t-\tnode:edge-node\n')
            elif mode == 'empty_ps':
                kw['ps'] = '    1     0  00:00 /sbin/launchd\n'
            elif mode == 'searxng_ok':
                # 外置 searxng：ps 命令行 `… -m searx.webapp` 被声明面 proc=searx.webapp 覆盖 ⇒ R4b 有落点 ⇒ 全绿
                kw['decl'] = GOOD_DECL + ('launchd\tcom.zerg.searxng\t~/Library/LaunchAgents/com.zerg.searxng.plist\t'
                                          '~/zerg-searxng/src/.venv/bin/python -m searx.webapp\tsearx.webapp\t主控\t-\thost\n')
                kw['ps'] = GOOD_PS + '  59975     1       11:25 ~/zerg-searxng/src/.venv/bin/python -m searx.webapp\n'
                kw['lc'] = GOOD_LC + '59975\t0\tcom.zerg.searxng\n'
                kw['plists'] = ('com.zerg.core.plist', 'com.zerg.agent-Mr2109.plist', 'com.zerg.searxng.plist')
            elif mode == 'searxng_leak':
                # 现值有 searxng 进程、声明面无落点 ⇒ R4b 必红；去掉 `searx.webapp` hint ⇒ 该行不可见 ⇒ 假绿（本格的牙）
                kw['ps'] = GOOD_PS + '  59975     1       11:25 ~/zerg-searxng/src/.venv/bin/python -m searx.webapp\n'
            dp, pp, lp, home = mk(d, **kw)
            if mode == 'no_decl':
                dp = os.path.join(d, '不存在.tsv')
            if mode == 'cli':
                # ⑰ **真命令行退码**：起子进程跑本件（走 main()/argparse/print_report 全链），
                #    读**进程** rc —— 夹具经 env 通道喂入（与真机取证同一条路）。
                os.makedirs(os.path.join(d, 'core', 'cmd', 'zerg'), exist_ok=True)
                env = {**os.environ, 'ZERG_SVCDECL_DECL': dp, 'ZERG_SVCDECL_PS': pp,
                       'ZERG_SVCDECL_LAUNCHCTL': lp, 'ZERG_SVCDECL_HOME': home}
                proc = subprocess.run([sys.executable, os.path.abspath(__file__), d],
                                      env=env, capture_output=True, text=True)
                rc, problems = proc.returncode, []
            else:
                rc, problems, notes, _ = check(d, decl_path=dp, ps_path=pp, launchctl_path=lp, home=home)
            flag = '✓' if rc == want else '✗'
            print('  %s %s（rc=%d）' % (flag, name, rc))
            if mode == 'swapped':                      # 点名断言：两侧名字都必须逐字出现
                joined = ' '.join(problems)
                named = ('com.zerg.tunnel.mini-node-1' in joined) and ('com.zerg.agent-Mr2109' in joined)
                print('  %s 点名断言（`A∖B` + `B∖A` 两侧逐字出现）' % ('✓' if named else '✗'))
                if not named:
                    ok = False
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
    decl = os.environ.get('ZERG_SVCDECL_DECL') or os.path.join(root, 'deploy', '服务声明.tsv')
    rc, problems, notes, modes = check(root)
    return print_report(root, rc, problems, notes, modes, decl)


if __name__ == '__main__':
    sys.exit(main())

#!/usr/bin/env python3
# check-exit-codes.py — 门⑨：退码表「一张表 + 同数字不再两义」（§二十一 已红第 4 条 · 开工单 T-26 ·
# §九 M3 `C6` · §十二 `P-129`）。
#
# 为什么要有它：已红第 4 条的病根是「同一个数字在同一仓里有两义」—— 这种病的表现**不是报错**，
#   是「AI 看到 `4` 判不出该换令牌还是该改用法」。它只能靠机器每次查：人不会记得第 7 处声明在哪儿。
#
# 七条规则（任一不通过 ⇒ 见下方退出码）：
#   R1 **主表自身**：`core/cmd/zerg/exitcodes.go` 的 `exitcodeTable` + `exitcodeReserved` 里
#      **数字唯一**（同表两义 ⇒ 红），且每行 name/meaning/retry 三格非空（占空格的号不算登记）。
#   R2 **门禁直通档逐字对齐**：`scripts/gates/precommit-gates.sh` 第 35 行（去行首 `# `）==
#      `core/cmd/zerg/helptext.go` 的 `gateExitLine`。
#   R3 **七处声明面全部指向同一张表**：每处都含标记行 `退码唯一真源 = zerg help exit-codes`。
#   R4 **「回滚成功」唯一说法**：`zerg-swap-core.sh` 与 `zerg-upgrade.sh` 里都必须出现 `ROLLED_BACK`，
#      且两处**不许**再把「回滚成功」写成 `2`（`= 2` / `2 已回滚成功` 两种写法都算）。
#   R5 **用码有落点**：每个声明面源码里用到的退码数字，必须落在登记表
#      （`core/internal/contract/exitcode-namespaces.json`）覆盖的集合里 ——
#      主表 / 该件自有号 / 已登记的撞号 / 已登记的越界号，四类之一。未登记 ⇒ 红。
#   R6 **「判不了」档口径 ⇒ 2；`8` 专留给「有 BLOCKED」**（不给结论，不许当绿）：
#      空转 / 缺件 ⇒ rc=2（既有）；**参数值无效 / 空串 / 缺实参 ⇒ 用法码 2**（**甲案** · 2026-09-28
#      拍板 · `GAP-20260928-46`）—— `8` 只给真「读不到」的 BLOCKED。★ **机检**（**只报告** ·
#      起步不阻断）：扫全仓脚本 / Go 件里把「值无效类」分支写成 `8` 的处，逐条列 件:行 + 逐字 +
#      它该归哪类；**分母 = 盘上现读集合**（`os.walk` 现读 · 禁 mtime、禁写死名单）。
#      ★ **判据面**（2026-09-28 补 · 治误标）：两类**语气词面**分开 —— 「值无效词」与「BLOCKED
#      语气词」（判不了/读不到/读不动/读不出/不给结论），ASCII 词按**词边界**（`precondition_missing`
#      不算 `missing`）；两类同现 ⇒ **BLOCKED 优先**且**逐条打印理由**（`不合法` 可能只是在描述
#      **读不到的上游产物**，例如 `all.sh:414`）。为的是「值无效类」这个读数**指的就是值无效**。
#   R7 **跨表差异逐条落判**：每个声明面用到的、与命令面主表**同号**且不在通行档 `0/1/2` 的号，
#      必须在登记表 `signed_differences` 里**逐条签字**（已签差异 ⇒ 只报告、**不阻断**）；
#      没签字 ⇒ 判红（**未声明差异**）。★ 治的是「声明面不在 `sites` 表 ⇒ 差异并存而门全绿」
#      这一类病（首例：`agent/cmd/zerg-agentd/main.go` 拿不到令牌退 `8` vs 命令面退 `4`）。
#
# 用法：
#   python3 scripts/gates/check-exit-codes.py [仓库根]
#   python3 scripts/gates/check-exit-codes.py --self-test    # 成对负控（3 正 + 6 反 + 1 空转 + 2 成对 ⑪⑫）
import argparse
import json
import os
import re
import sys
import tempfile

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


# ── R6 「判不了」档口径的机检（甲案 · **只报告** · 起步不阻断）──────────────────
#    ★ 分母是**盘上现读集合**：现走一遍树取 .py/.sh/.go，跳 VCS/依赖/产物目录与 >2MB 的件。
#      禁 mtime、禁写死名单 —— 「名单写死」正是本条病根（各件自定口径、无人看着）。
VALUE_INVALID_MARKERS = (
    '值无效', '参数无效', '空串', '空字符串', '缺实参', '缺参数', '缺必填', '用法错',
    '未知', '非法', '不合法', '无效', 'invalid', 'empty', 'missing', 'usage',
)
# 「判不了 / BLOCKED 语气」词面（**判据面** · 2026-09-28 补）：本行在说「读不到 ⇒ 不给结论」
#   —— `8` 的**语义**就在这一档。两类词同现时按 BLOCKED 优先（见 `classify_line`，**逐条打印理由**）。
#   ★ 与 `VALUE_INVALID_MARKERS` 成对：少了这一半，「不合法」这种**描述上游产物**的词就会把一条
#     真 BLOCKED 读成「值无效类」（治病现场：`all.sh:414`）；多了这一半不等于放宽 —— 不带 BLOCKED
#     语气的值无效行照旧计入（自检 ⑫ 成对断言）。
BLOCKED_IDIOM_MARKERS = (
    '判不了', '判不出', '不给结论', '读不到', '读不动', '读不出', '拿不到', 'blocked',
)
SCAN_SKIP_DIRS = {'.git', 'vendor', 'dist', 'node_modules', 'bin', '__pycache__', '.venv', 'venv'}
SCAN_SUFFIX = ('.py', '.sh', '.go')
SCAN_MAX_BYTES = 2 * 1024 * 1024
EXIT8_RE = re.compile(r'os\.Exit\(\s*8\s*\)|(?:^|[^A-Za-z_])exit\s+8(?![0-9])'
                      r'|exit[A-Za-z_]*\s*=\s*8\b|\brc\s*=\s*8\b|\bRC\s*=\s*8\b')
#
# 自指豁免（**打印理由** · 不是静默放过 · 同门㊶/门㊾ 形态）：本件自己的自检夹具里必然把
# 「值无效类 ⇒ 8」的**形态**写成字面量（正控⑨/⑩ 的 swap/decl 夹具逐字含 `exit 8`）——
# 若不豁免，R6 命中面会把**自家夹具**读成「待归一的真缺陷」⇒ 「归一进度」读数被自家污染。
# 本件**不在** R6 命中面里（逐条打印豁免理由）；别的件一律照判（本件不是真缺陷的「后门」）。
SELF_EXEMPT_BASENAME = "check-exit-codes.py"   # 硬写本件名（同门㊶）—— 不许随运行副本名漂移


def read_scan_set(root):
    """盘上现读集合（R6 的分母）：现走树，取 .py/.sh/.go，跳产物/依赖目录与超大件。"""
    out = []
    for dp, dn, fn in os.walk(root):
        dn[:] = [d for d in dn if d not in SCAN_SKIP_DIRS]
        for f in fn:
            if not f.endswith(SCAN_SUFFIX):
                continue
            p = os.path.join(dp, f)
            try:
                if os.path.getsize(p) > SCAN_MAX_BYTES:
                    continue
            except OSError:
                continue
            out.append(p)
    return out


def ascii_token_hit(word, low):
    """ASCII 标记词按**词边界**判（`\\bmissing\\b`）—— 防「子词误命中」。

    ★ 治病现场（2026-09-28）：`precondition_missing` 里含 `missing` ⇒ 旧面把一条
      「真源在但读不出 ⇒ rc=8」的**真 BLOCKED** 行读成「值无效类」。子串面会把
      **标识符/字段名**读成语气 —— 边界是判据面的一部分，不是宽容。
    """
    return re.search(r'\b' + re.escape(word) + r'\b', low) is not None


def blocked_idiom_hits(low):
    """「判不了 / BLOCKED 语气」词面：本行是否在说「读不到 ⇒ 不给结论」。"""
    return [m for m in BLOCKED_IDIOM_MARKERS if m in low]


def value_invalid_terms(low):
    """「值无效语气」词面：中文按子串、ASCII 按词边界。"""
    return [m for m in VALUE_INVALID_MARKERS
            if (m in low if not m.isascii() else ascii_token_hit(m, low))]


def classify_line(line):
    """R6 **判据面**：一条「用 8」的行归哪类 —— 返回 (类目, 值无效词, BLOCKED 词, 理由)。

    ★ 两类词同时出现 ⇒ 按**优先级 BLOCKED 优先**并**逐条打印理由**（不许静默）：
      8 在这类行里说的是「这门读不到 ⇒ 判不了」，值无效词只是**描述读不到的东西**
      （例：`--json 读不动（JSON 不合法 / 空）⇒ 判不了（rc=8）` —— `不合法` 说的是**上游产物**，
      不是本支的参数值）。**真值无效类判据未放宽**：不带 BLOCKED 语气的行照旧计入
      （证：自检 ⑫ 的 `参数值无效：空串 ⇒ exit 8` 行改前改后皆计入）。
    """
    low = line.lower()
    vi = value_invalid_terms(low)
    bd = blocked_idiom_hits(low)
    if vi and bd:
        return ('混类 ⇒ 按优先级判 **BLOCKED**（8 合法）· 待复核词已逐条打印',
                vi, bd, '两类词同现：BLOCKED 语气（%s）优先于值无效词（%s）'
                        % ('/'.join(bd), '/'.join(vi)))
    if vi:
        return ('值无效类（应走 2 · 需归一）', vi, bd,
                '只含值无效词（%s）· 无 BLOCKED 语气' % '/'.join(vi))
    return ('BLOCKED 类（8 合法）', vi, bd,
            '无值无效词 · %s' % ('BLOCKED 语气（%s）' % '/'.join(bd) if bd else '纯用 8（无语气词）'))


def scan_value_invalid_8(root):
    """R6：找「值无效类」分支里用了 8 的处。返回 (分母, 逐条, 自豁免清单)。

    ★ **只报告**：本函数**不**往 problems 里写东西（只报告起步 ⇒ 不阻断）。
    归类（**判据面**见 `classify_line`）：命中行里含值无效类标记词**且不含** BLOCKED
          语气词 ⇒ 「值无效类」（按甲案该走 `2`，需归一）；两类词同现 ⇒ 「混类」按优先级
          判 BLOCKED（**理由逐条打印**）；其余 ⇒ 「BLOCKED 类」（`8` 合法，它真在说「读不到」）。
    ★ 自指豁免（**打印理由** · 不是静默放过 · 同门㊶/门㊾）：本件自己的自检夹具把「值无效类
      ⇒ 8」的**形态**写成字面量 ⇒ 本件**不在**命中面里（进 self_exempt 逐条打印理由）；
      别的件一律照判 —— 真缺陷落在盘上任一别件里仍逐条计入（未放宽真判据）。
    """
    files = read_scan_set(root)
    hits, exempt = [], []
    for p in files:
        rel = os.path.relpath(p, root)
        if os.path.basename(p) == SELF_EXEMPT_BASENAME:
            exempt.append({'site': rel,
                           'why': '本件的自检夹具把「值无效类 ⇒ 8」的形态写成字面量'
                                  '（正控⑨/⑩ 的 swap/decl 夹具逐字含 `exit 8`）'
                                  '⇒ 自豁免（**打印理由**，不是静默放过）'})
            continue
        try:
            txt = read(p)
        except (OSError, UnicodeDecodeError):
            continue
        for ln, line in enumerate(txt.splitlines(), 1):
            if not EXIT8_RE.search(line):
                continue
            verdict, vi, bd, why = classify_line(line)
            hits.append({'site': os.path.relpath(p, root), 'line': ln,
                         'text': line.strip()[:160],
                         'verdict': verdict,
                         'why': why,
                         'value_invalid': bool(vi) and not bd,
                         'mixed': bool(vi) and bool(bd)})
    return len(files), hits, exempt


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

    # ── R3 七处声明面全部指向同一张表 ────────────────────────────────
    #     ★ 等价落点：已在 `signed_differences` 里逐条签字的面 = 明说自己是「不是同一张表」的自有码空间
    #       ⇒ 标记行由**登记表里的签字**替代（例：`agent/cmd/zerg-agentd/main.go` 归另路 · 禁碰，
    #       不许往件内加标记行）—— 只报告，不判红；签字本身仍受 R7 的落点检查。
    sig_sites = {d.get('site') for d in (reg.get('signed_differences') or [])}
    for rel in sites:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            problems.append('FAIL R3 声明面缺件：%s' % rel)
            continue
        if MARKER not in read(p):
            if rel in sig_sites:
                notes.append('标记行由签字替代（只报告）：%s 未带「%s」，但已在 signed_differences 里逐条签字'
                             % (rel, MARKER))
            else:
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

    # ── R7 跨表差异：与命令面主表同号者必须逐条落判（已签差异 ⇒ 只报告 · 未声明差异 ⇒ 红）──
    #     通行档 0/1/2 与门禁直通档**同码同义**（R2）⇒ 不算分歧；分歧从 3 起。
    #     已登记撞号（`collisions`）是既有的逐字落判面 ⇒ 本规则不重复判，专治**新出现**的同号不同义
    #     （`agent/cmd/zerg-agentd/main.go` 的 `8` 就是第一例：过去不在 `sites` ⇒ 门全绿而差异在）。
    UNIVERSAL = {0, 1, 2}
    sig_tbl = {(d.get('site'), d.get('code')): d for d in (reg.get('signed_differences') or [])}
    for rel in sites:
        p = os.path.join(root, rel)
        if not os.path.isfile(p) or ns.get(rel, {}).get('id') == 'cli':
            continue
        for c in sorted(used_codes(read(p), detect_mode(p))):
            if c not in table_codes or c in UNIVERSAL or c in coll:
                continue
            d = sig_tbl.get((rel, c))
            if d is not None:
                notes.append('已签差异（只报告 · **不阻断**）：%s 的 `%d` 与命令面 `%s` 同号不同义 —— %s'
                             % (rel, c, d.get('cli_code', '?'), d.get('reason', '')))
            else:
                problems.append('FAIL R7 %s 用了与命令面主表同号的 `%d`，却没在登记表 signed_differences 里签字'
                                '（**未声明差异**：同数字两义的第一现场 —— 要么签字说明它是「不是同一张表」的'
                                '自有码、要么去掉这个号）' % (rel, c))
    # 签字必须有落点：点名的面不在 sites / 该面根本不用那个号 ⇒ 陈旧签字也得报（不许静默）
    for d in (reg.get('signed_differences') or []):
        s = d.get('site')
        if s not in sites:
            problems.append('FAIL R7 signed_differences 点名的面 %s **不在 sites 表**（签字没有落点）' % s)
            continue
        p = os.path.join(root, s)
        if os.path.isfile(p) and int(d.get('code', -1)) not in used_codes(read(p), detect_mode(p)):
            notes.append('R7 陈旧签字（只报告）：%s 没在用 `%s`（signed_differences 那一条可撤）'
                         % (s, d.get('code')))

    # ── R6 「判不了」档口径的机检：值无效类不许用 8（**只报告** · 起步不阻断）──────
    #     甲案（2026-09-28 拍板 · `GAP-20260928-46`）：参数值无效 / 空串 / 缺实参 ⇒ 用法码 2；
    #     `8` 专留给「有 BLOCKED」（读不到不许当健康）。★ 起步**只报告** ⇒ 只进 notes，不进 problems。
    r6_denom, r6_hits, r6_exempt = scan_value_invalid_8(root)
    r6_vi = [h for h in r6_hits if h['value_invalid']]
    r6_mix = [h for h in r6_hits if h['mixed']]
    r6_bd = [h for h in r6_hits if not h['value_invalid'] and not h['mixed']]
    notes.append('R6 「判不了」档口径机检（只报告 · 不阻断）：盘上现读 %d 件（.py/.sh/.go · 跳 VCS/'
                 '依赖/产物 与 >2MB）· 命中用 8 的处 %d 条 ⇒ 「值无效类」（按甲案应走 2 · 待归一）%d 条 ·'
                 ' 「BLOCKED 类」（8 合法）%d 条 · 「混类」（两类词同现 ⇒ 按优先级判 BLOCKED ·'
                 ' 待复核词逐条打印）%d 条 · 自豁免（打印理由）%d 件'
                 % (r6_denom, len(r6_hits), len(r6_vi), len(r6_bd), len(r6_mix), len(r6_exempt)))
    for e in r6_exempt:
        notes.append('R6 ⊘ 自豁免 %s —— %s' % (e['site'], e['why']))
    for h in r6_hits:
        notes.append('R6 %s:%d [%s] 理由: %s ｜逐字: %s'
                     % (h['site'], h['line'], h['verdict'], h['why'], h['text']))

    rc = 0 if not problems else 1
    return rc, problems, notes, {'table': sorted(table_codes), 'sites': len(sites),
                                 'collisions': len(coll), 'out_of_range': len(oob),
                                 'signed_differences': len(sig_tbl),
                                 'r6_scan_set': r6_denom, 'r6_hits_8': len(r6_hits),
                                 'r6_value_invalid_8': len(r6_vi),
                                 'r6_mixed_8': len(r6_mix),
                                 'r6_blocked_8': len(r6_bd),
                                 'r6_self_exempt': len(r6_exempt)}


def print_report(root, rc, problems, notes, extra):
    print('── 门⑨ 退码表：一张表 + 同数字不再两义（§二十一 已红第 4 条 · T-26 · §九 M3 C6 · §十二 P-129）──')
    print('  真源: core/cmd/zerg/exitcodes.go · 登记表: %s' % SITES_REL)
    for n in notes:
        print('  · %s' % n)
    for p in problems:
        print('  · %s' % p)
    if rc == 0:
        print('⇒ 全绿（rc=0）：七处声明面指向同一张表 · 主表内数字唯一 · 回滚成功只有一个说法 · 用码逐条有落点 · 跨表差异逐条签字')
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
GOOD_TABLE_8 = '''package main
var exitcodeTable = []exitcodeRow{
\t{0, "ok", "成功", "—"},
\t{2, "usage", "用法错 / **不给结论**", "改用法后可重试"},
\t{8, "blocked", "有 BLOCKED（「读不到」不许当健康）", "补齐前置后可重试"},
}
var exitcodeReserved = []exitcodeRow{
\t{130, "interrupted", "人打断", "—"},
}
'''
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
        ('正控⑨ R6 只报告：主表有 8 时，脚本声明面把「值无效」写成 exit 8 也**不阻断**（rc 仍 0）', 0,
         {'table': GOOD_TABLE_8,
          'roster': dict(GOOD_ROSTER, signed_differences=[
              {'site': 'scripts/build/zerg-swap-core.sh', 'code': 8, 'cli_code': 8,
               'reason': 'R6 自证夹具：脚本声明面把值无效类写成 8'}]),
          'swap': GOOD_SWAP + '# 值无效：参数空串 ⇒ exit 8\n' + 'exit 8\n'}),
        ('正控⑩ R6 只报告：Go 声明面把「空串」写成 exitCode = 8 也**不阻断**（rc 仍 0）', 0,
         {'table': GOOD_TABLE_8,
          'roster': dict(GOOD_ROSTER, signed_differences=[
              {'site': 'core/cmd/zerg-compat/main.go', 'code': 8, 'cli_code': 8,
               'reason': 'R6 自证夹具：Go 声明面把空串类写成 8'}]),
          'decl': GOOD_DECL + '// 值无效：空串参数 ⇒ exitCode = 8\nvar exitBad = 8\n'}),
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
    # ── ⑪ 成对（自豁免 ≠ 放宽真判据）：别件真缺陷**仍逐条计入** · 本件名夹具只进自豁免 ──────
    #    面：把「值无效类 ⇒ exit 8」同时写进 ① 一个**别件名**（真缺陷）与 ② **本件名**（夹具）。
    with tempfile.TemporaryDirectory() as d:
        def _w(rel, text):
            fp = os.path.join(d, rel)
            os.makedirs(os.path.dirname(fp), exist_ok=True)
            with open(fp, 'w', encoding='utf-8') as f:
                f.write(text)
        _w('scripts/build/probe-real.sh', '# 值无效：参数空串 ⇒ exit 8\nexit 8\n')
        _w('scripts/gates/' + SELF_EXEMPT_BASENAME, '# 值无效：参数空串 ⇒ exit 8\nexit 8\n')
        _dn, _hits, _exempt = scan_value_invalid_8(d)
        _real_n = len([h for h in _hits if h['site'].endswith('probe-real.sh')])
        _fake_n = len([h for h in _hits if h['site'].endswith(SELF_EXEMPT_BASENAME)])
        _ok_pair = (_real_n == 2 and _fake_n == 0 and len(_exempt) == 1)
        print('  %s ⑪ 自豁免不放宽真判据：别件真缺陷仍计入（%d 行）· 本件名夹具只进自豁免'
              '（命中 %d 行 · 豁免 %d 件）' % ('✓' if _ok_pair else '✗', _real_n, _fake_n, len(_exempt)))
        ok = ok and _ok_pair

    # ── ⑫ 判据面两态（**误标修正** · 与 ⑪ 成对）：真 BLOCKED 不再计入值无效类 · 真「值无效写成 8」────────
    #    ① `all.sh:414` 形态（`读不动（JSON 不合法 / 空）⇒ 判不了（rc=8）`）—— 改前被「不合法」误标
    #    ② 真「值无效写成 8」形态 —— **改前改后皆计入**（证判据未放宽）
    #    ③ ASCII 子词形态（`precondition_missing` 里的 `missing`）—— 子串面误命中的第二现场
    with tempfile.TemporaryDirectory() as d3:
        def _w3(rel, text):
            fp = os.path.join(d3, rel)
            os.makedirs(os.path.dirname(fp), exist_ok=True)
            with open(fp, 'w', encoding='utf-8') as f:
                f.write(text)
        _w3('scripts/gates/allsh-414-probe.sh',
            'echo "all.sh: real-gates.sh --json 读不动（JSON 不合法 / 空）⇒ --real **判不了**'
            '（rc=8）" >&2\nexit 8\n')
        _w3('scripts/build/value-invalid-probe.sh',
            'echo "参数值无效：空串 ⇒ exit 8" >&2\n'
            'echo "缺实参 ⇒ exit 8" >&2\nexit 8\n')
        _w3('scripts/build/precondition-missing-probe.sh',
            '# 真源在但读不出 ⇒ rc=8 · meta.reason=precondition_missing\nexit 8\n')
        _d3, _h3, _e3 = scan_value_invalid_8(d3)
        _vi414 = [h for h in _h3 if h['site'].endswith('allsh-414-probe.sh') and h['value_invalid']]
        _viok = [h for h in _h3 if h['site'].endswith('value-invalid-probe.sh') and h['value_invalid']]
        _vipre = [h for h in _h3 if h['site'].endswith('precondition-missing-probe.sh')
                  and h['value_invalid']]
        _fixture414_ok = (len(_vi414) == 0 and len(_viok) == 2 and len(_vipre) == 0)
        print('  %s ⑫ 判据面两态：真 BLOCKED（all.sh:414 形态 · 读不动/判不了）**不计入**值无效类（%d 行）'
              ' · 真「值无效写成 8」**仍逐条计入**（%d 行）· ASCII 子词（precondition_missing）不再误命中'
              '（%d 行）' % ('✓' if _fixture414_ok else '✗', len(_vi414), len(_viok), len(_vipre)))
        _why414 = next((h for h in _h3 if h['site'].endswith('allsh-414-probe.sh')), None)
        print('  %s ⑫ 逐条理由（all.sh:414 形态那条判哪类）：[%s] %s'
              % ('✓' if _why414 and not _why414['value_invalid'] else '✗',
                 _why414['verdict'] if _why414 else '（没命中 · 异常）',
                 _why414['why'] if _why414 else ''))
        ok = ok and _fixture414_ok and bool(_why414) and not _why414['value_invalid']

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

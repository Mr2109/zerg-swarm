#!/usr/bin/env python3
# check-cli-contract.py — 门禁⑪：**命令面自身的契约**（§九 M17 `T1`–`T6` · 开工单 T-42）。
#
# 为什么要有它（§九 M17 逐字：命令面「自己**也得被门禁守**」）：
#   命令面是薄壳，它的病**不是崩**，是**悄悄少一层**：新加一条命令忘了配 must-fail、
#   退码表被抄成第二份、`--json` 包封被改形状、矩阵被豁免洗白…… 这些都不会报错，
#   只会让「契约」慢慢变成一纸空文。判据按 §九 M17 的六条条款，**本门不重写判据**
#   （`T6`：引用 §九 M6 R7 两条 —— 判据本体在那两条里，本门不重写）。
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
#   T5d **`why` ⟷ 值 同源**（2026-09-27 补 · 题面作 `T6`）：`why` 里的绝对数字（字节/条数/退码）
#      必须与本格的值同源，或落进四类豁免（出处/编号 · 门槛/上限 · 历史/他格 · 否定式）——
#      值漂了 why 不跟着漂时红（why 与派生列是同格内的两处真源：写面只刷值、判定口只比值）。
#   T6 **引用 M6 `R7`**：本门头部引用它 + 契约登记表 `registry.json` 的命令面条目 `gate` 字段
#      **点名本脚本**（引用面非 0 = 它真的被用，不是挂在空气上）。
#   R7 **族名闭合**（§十二 `P-038` 的落判 · 开工单 T-43 ①）：族名只有一个 `agent`；
#      命令树里不许有 `node` 族，**全仓**（按排除表）不许再出现 `zerg node <动作>` 这类写法。
#
# 用法：
#   python3 scripts/gates/check-cli-contract.py [仓库根]
#   python3 scripts/gates/check-cli-contract.py --self-test          # 成对负控（4 正控 + 13 负控 + 1 空转
#                                                                   # + 3 条 emit-matrix 合并/字节/幂等控 ·
#                                                                   # 其中 `R-6` 逐格回放 4 条 · T7 判不了一族 3 条）
#   python3 scripts/gates/check-cli-contract.py --no-runtime         # **只给夹具用**：跳过 T4 的 go test
#   python3 scripts/gates/check-cli-contract.py --emit-matrix [路径] # **增量合并**矩阵 —— 只刷新机器可派生
#                                                                   # 列（want_rc / want_stdout_bytes=**字节**数），
#                                                                   # 既有格的 id/command/argv/why 逐字保留
#                                                                   # （先用 build-all.sh --only-cli 出新制品）；
#                                                                   # 写盘后再对**新账**逐格现跑一遍（只报不判）
#   python3 scripts/gates/check-cli-contract.py --verify-live        # §六 `R-6`：拿**冻结的矩阵**逐格现跑同一
#                                                                   # `argv`（每格两遍），核 `rc` 与 stdout **字节数**
#                                                                   # —— 抓「多余格 / 幽灵格」；0 全合 / 1 有 mismatch
#                                                                   # / 2 不给结论（缺件、零格）
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

# R7（族名闭合 · §十二 `P-038` 的落判）：全仓不许再出现 `zerg node <动作>` 这类写法 ——
# 族名只有一个 `agent`；`--node` 是**对象级旗标**，不是族名。
NODE_FAMILY_RE = re.compile(r'\bzerg\s+node\s+[A-Za-z]')
# 扫描面：与 §十七/§二十一 各处复跑命令同一口径的排除表（+ 本判据脚本自己：它的规则描述里
# 必然出现那个写法）。按**文件名**窄排除，不留静默的洞（负控证明别处一出现即红）。
SCAN_EXCLUDE_DIRS = {'target', 'node_modules', 'dist', 'bin', 'vendor', 'data', '.git',
                     '.venv', 'venv', '.build'}
SCAN_EXTS = ('.go', '.sh', '.py', '.rs', '.md', '.json', '.yaml', '.yml', '.ts', '.tsx', '.js', '.toml')

CMD_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
PATH_SEG_RE = re.compile(r'"([^"]+)"')
DANGER_RE = re.compile(r'danger:\s*&dangerSpec\{(dangerD[23])')
FIELDS_RE = re.compile(r'fields:\s*\[\]string\{([^}]*)\}')
# `fields: <标识符>`（拼接变量形态 · 缺口 GAP-20260928-159）：字段表是拼接出来的（如 `devEditFields`，
# 定义在 `family_dev_edit.go`）⇒ 只认 main.go 里的字面量会把这类命令读成「没有字段表」= 字段面凭空消失。
FIELDS_IDENT_RE = re.compile(r'fields:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,')
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


def package_sources(root):
    """`core/cmd/zerg` 包内全部非测试 .go 源（拼接）—— 供 `fields: <标识符>` 跨件解析（GAP-20260928-159）。"""
    pkg = os.path.dirname(os.path.join(root, MAIN_REL))
    try:
        names = sorted(os.listdir(pkg))
    except OSError:
        return ''
    out = []
    for n in names:
        if n.endswith('.go') and not n.endswith('_test.go'):
            try:
                out.append(read(os.path.join(pkg, n)))
            except OSError:
                pass
    return '\n'.join(out)


def _src_ident_has_fields(pkg_src, ident, depth=0):
    """拼接变量的字段表：标识符定义里出现任一 `[]string{…}`（含 append 出来的）⇒ 有字段面。
    （缺口 GAP-20260928-159 · 递归 ≤ 3 层，够 `devEditFields` 那一族用。）"""
    if depth > 3:
        return False
    for mt in re.finditer(r'\b%s\b\s*=\s*([^\n]*)' % re.escape(ident), pkg_src):
        if '[]string{' in mt.group(1):
            return True
        for ref in re.findall(r'\b([A-Za-z_][A-Za-z0-9_]*)\b', mt.group(1)):
            if ref != ident and _src_ident_has_fields(pkg_src, ref, depth + 1):
                return True
    return False


def block_has_fields(pkg_src, block):
    """本命令块有没有字段表：字面量 `[]string{…}` 或拼接变量（缺口 GAP-20260928-159）。"""
    if FIELDS_RE.search(block):
        return True
    mt = FIELDS_IDENT_RE.search(block)
    return bool(mt and _src_ident_has_fields(pkg_src, mt.group(1)))


def parse_tree(root):
    """命令树 → [(path, shape)]；shape = dict(danger, fields, net, passthrough)。"""
    src = read(os.path.join(root, MAIN_REL))
    pkg_src = package_sources(root)
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
            'fields': block_has_fields(pkg_src, block),
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


def strip_go_code(src):
    """去掉 Go 源里的**字符串/字符字面量与注释**（换行原样保留、其余字符以空格顶替），
    使 T3 的裸数字判据只看**真源码位置** —— 夹具字符串里的 `func … { return 3 }`、
    注释里的 `return 9` 都**不算**退码（缺口 GAP-20261002-12：此前全文扫 ⇒ 假红三处）。
    ★ 真源码里的 `return 127` 仍原样留在结果里 ⇒ **真缺口照红**（负控面不缩水）。
    """
    out = list(src)
    CODE, LINE, BLOCK, STR, RAW, RUNE = range(6)
    st = CODE
    i, n = 0, len(src)
    while i < n:
        c = src[i]
        nx = src[i + 1] if i + 1 < n else ''
        if st == CODE:
            if c == '/' and nx == '/':
                out[i] = out[i + 1] = ' '
                i += 2
                st = LINE
                continue
            if c == '/' and nx == '*':
                out[i] = out[i + 1] = ' '
                i += 2
                st = BLOCK
                continue
            if c == '"':
                out[i] = ' '
                st = STR
            elif c == '`':
                out[i] = ' '
                st = RAW
            elif c == "'":
                out[i] = ' '
                st = RUNE
        elif st == LINE:
            if c == '\n':
                st = CODE
            else:
                out[i] = ' '
        elif st == BLOCK:
            if c == '*' and nx == '/':
                out[i] = out[i + 1] = ' '
                i += 2
                st = CODE
                continue
            if c != '\n':
                out[i] = ' '
        elif st == STR:
            if c == '\\' and nx:
                out[i] = out[i + 1] = ' '
                i += 2
                continue
            if c == '"':
                out[i] = ' '
                st = CODE
            elif c != '\n':
                out[i] = ' '
        elif st == RAW:
            if c == '`':
                out[i] = ' '
                st = CODE
            elif c != '\n':
                out[i] = ' '
        elif st == RUNE:
            if c == '\\' and nx:
                out[i] = out[i + 1] = ' '
                i += 2
                continue
            if c == "'":
                out[i] = ' '
                st = CODE
            elif c != '\n':
                out[i] = ' '
        i += 1
    return ''.join(out)


def bare_numeric_returns(root):
    """命令面源码里的**裸数字**退出码（`return N` / `os.Exit(N)`）→ [(file, N)]。"""
    d = os.path.join(root, 'core', 'cmd', 'zerg')
    out = []
    for name in sorted(os.listdir(d)):
        if not name.endswith('.go') or name.endswith('_test.go'):
            continue
        for m in BARE_RET_RE.finditer(strip_go_code(read(os.path.join(d, name)))):
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


def node_family_hits(root, self_path=None):
    """全仓（按排除表）扫 `zerg node <动作>` 写法 → ['rel:line: 原文', …]。"""
    me = os.path.abspath(self_path or __file__)
    hits = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SCAN_EXCLUDE_DIRS)
        for fn in sorted(filenames):
            if not fn.endswith(SCAN_EXTS):
                continue
            full = os.path.join(dirpath, fn)
            if os.path.abspath(full) == me:
                continue  # 本判据脚本自己（规则描述里必然出现那个写法）
            try:
                txt = read(full)
            except (OSError, UnicodeDecodeError):
                continue
            for i, line in enumerate(txt.splitlines(), 1):
                if NODE_FAMILY_RE.search(line):
                    rel = os.path.relpath(full, root)
                    hits.append('%s:%d: %s' % (rel, i, line.strip()[:140]))
    return hits


# ── 矩阵重生成（`--emit-matrix`）：矩阵是**冻结的现行为**，只有真跑才冻得住 ────────────────────
# ★ 必须用**刚从源码构建的**制品（`bash scripts/build/build-all.sh --only-cli`）：拿旧制品冻结，
#   新加的命令会被当成「未知命令」而静默漏掉一格。
# ★ 2026-09-21（收掉《开工记录》表三 #3）修掉**两个真 bug**：
#   ① **字节数**：`want_stdout_bytes` 是**字节**判据（判定口 `cli_matrix_test.go` 的 `judgeCase`
#      比的是 `out.Len()` = 字节），旧实现写的是 `len(outs[0])` = **字符数** ⇒ 含中文的输出被记小。
#      实测（现跑）：`dev proposal --json name` stdout **258 字符 / 272 字节**，矩阵里正确的值是 272
#      ⇒ 旧实现一跑就把 272 **篡改成 258**，判定口从此按错值对拍（放宽而非报红）。
#      ⇒ 现在一律取 `len(stdout.encode('utf-8'))`。
#   ② **增量合并**：旧实现把 `cases` 整表重写 ⇒ 手写格整批丢掉（实测丢 **16 格** = 14 格手写口令
#      + 2 格手改过 argv 的自动格；另有 5 格的 argv 被候选面覆盖）。现在**以既有矩阵为底**：
#      · 既有格的 `id`/`command`/`argv`/`why`（含顺序）与 `exemptions` **逐字保留** —— 一个都不删；
#      · **只刷新机器可派生列**（`want_rc` / `want_stdout_bytes` —— 它们由现跑两遍派生）；
#      · 既有格若现跑不成立（幽灵 / 两次不等 / 退码 0）⇒ **保留原值**并在输出里逐条点名（不静默改）；
#      · 只有**命令树里一格都没有的命令**才按候选面**追加**新格（追加面仍要求退码 == 判据语义要的那个）
#        —— 这是 T1 覆盖的缺口填充，不是「顺手加格」。
MATRIX_NOTE = ('命令面自身的 must-fail 矩阵（§九 M17 `T2` · 开工单 T-42 判据①）。\n'
               '口径：这不是「猜的期望值」，是**冻结的现行为（棘轮）** —— 每条口令逐条现跑两遍、'
               '退码一致、且与判据语义相符才入矩阵；改行为就必须改这一格（改不动就是回归）。\n'
               '候选口令按命令形状给（危险档 / 字段面 / 说明面 / 透传型），执行前判的那两条（未知旗标 · '
               '不认的主号）**每条命令都有**；透传型只留执行前判那条 —— 别的会把被包的脚本真跑一遍。\n'
               '重生成：`python3 scripts/gates/check-cli-contract.py --emit-matrix`'
               '（先用 `bash scripts/build/build-all.sh --only-cli` 出**新**制品）。\n'
               '★ 生成是**增量合并**（不是整表重写）：只刷新机器可派生列'
               '（`want_rc` / `want_stdout_bytes` = **现跑字节数** `len(stdout.encode())` —— 不是字符数），'
               '既有格的 id/command/argv/why 与顺序逐字保留（手写格不会被重写吃掉），'
               '只有「一格都没有的新命令」才追加候选格。\n'
               '覆盖与空白由 T1/T5 判（空白受 baseline 棘轮约束）。')


def candidate_cases(path, shape, usage_code=2):
    """按命令形状给候选 must-fail 口令 → [(名, argv, 判据要的退码, why)]。

    ★ `usage_code` = 退码表里 `usage` 那一格的码，**由 `exitcode_table(root)` 现读**后传进来
      （`K2` 甲档归一 2026-09-24 的纪律：判据值的真源是 `exitcodes.go` 的表，本文件**不许**把
      码写死在候选面里 —— 写死的话表一改，这里就悄悄跟丢，「指向真源」变成「各自一份」）。
    """
    out = []
    pre = lambda tail: path.split() + tail
    out.append(('未知旗标', pre(['--nosuchflag-zz']), 2,
                '未知旗标 = 用法错（§4.1 K14 四件套）· 执行前判，不碰任何目标'))
    out.append(('不认的契约主号', pre(['--schema', 'zerg/v9']), 2,
                '消费侧拒绝不认的 schema 主号（§十二 P-026）· 执行前判'))
    if shape['passthrough']:
        # 透传型：只留执行前判那一条。未知旗标对透传型**不**在执行前判 —— 它被原样交给被包的
        # 脚本 ⇒ 排它就等于在测试里真跑一遍全量门禁（成本与副作用都不该进单元测试）。
        return [c for c in out if c[0] == '不认的契约主号']
    if shape['danger'] == 'dangerD3':
        out.append(('缺 --confirm（D3）', pre(['x']), 2,
                    'D3 不可逆：缺确认 ⇒ fail-closed 2（§九 M3 C2 · §十二 P-014）'))
        out.append(('--confirm 值不匹配', pre(['x', '--confirm=不匹配-zz', '--yes']), 2,
                    '确认值必须与目标逐字相同 —— 给错值一律拒绝（§4.1 K7）'))
    if shape['danger'] == 'dangerD2':
        out.append(('缺 --yes（D2）', pre(['x']), 2,
                    'D2 缺 --yes ⇒ fail-closed 2（§九 M3 C2）'))
    if not shape['danger'] and shape['fields'] and not shape['net'] and path != 'help export':
        out.append(('--json 不给字段', pre(['--json']), usage_code,
                    '给了 --json 不给字段 ⇒ %d + stdout 0 字节（§4.1 K2 甲档 · 码**取自退码表** `usage`）'
                    % usage_code))
    if not shape['danger'] and not shape['fields'] and not shape['net']:
        out.append(('说明面不认 --json', pre(['--json', 'name']), 2,
                    '没有字段表的说明面 ⇒ 2（与 K2 甲档归一后的 %d 同码、**错因不同**：这里根本没有字段表）'
                    % usage_code))
    return out


def derive_columns(binz, argv):
    """派生列的唯一来源：口令现跑两遍 → ({'want_rc','want_stdout_bytes'}, '') 或 (None, 不成立的原因)。

    ★ `want_stdout_bytes` 取 **字节数** `len(stdout.encode('utf-8'))` —— 判定口（`cli_matrix_test.go`
      的 `judgeCase`）比的就是字节数；旧实现取 `len(stdout)` = 字符数（本文件头部 bug ① 的来历）。
    """
    rcs, outs = [], []
    for _ in range(2):
        p = subprocess.run([binz] + argv, capture_output=True, text=True,
                           stdin=subprocess.DEVNULL)
        rcs.append(p.returncode)
        outs.append(p.stdout)
    if '未知命令' in outs[0] or '未知命令' in outs[1]:
        return None, '幽灵命令（制品里没有它 ⇒ 制品不是新的）'
    if rcs[0] != rcs[1]:
        return None, '两次退码不等 %s' % rcs
    if rcs[0] == 0:
        return None, '退码 0 —— must-fail 矩阵不许有「该绿」的格'
    return {'want_rc': rcs[0], 'want_stdout_bytes': len(outs[0].encode('utf-8'))}, ''


def run_once(binz, argv):
    """口令现跑**一遍** ⇒ (退码, stdout **字节数**)。"""
    p = subprocess.run([binz] + argv, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return p.returncode, len(p.stdout.encode('utf-8'))


def resolve_bin(root, bin_path=None):
    """缺口 `GAP-20260926-12`：制品路径可由外部指定（`--bin <路径>`）。

    不给（`None`/空）⇒ 老口径：`<root>/bin/zerg`（逐字不变）。
    给了 ⇒ 用**绝对化后**的该路径 —— 于是「仓外临时仓根 ＋ 本机真制品」两态可用，
    不必为了跑门在临时仓根里再造一棵 `bin/`（✗ 不许碰真仓 `bin/`）。
    """
    if bin_path:
        return os.path.abspath(bin_path)
    return os.path.join(root, 'bin', 'zerg')


def live_verify(root, matrix_path=None, bin_path=None):
    """§六 `R-6` 的**新负控档**：拿**冻结的**矩阵逐格现跑同一 `argv`（每格跑两遍）、核 `rc` 与 stdout **字节数**。

    为什么要它：`T1` 只判「命令树里的命令**有没有格**」、`T5` 只判「`blanks`」——
    两件都看**账**不看**盘**。旧格若在盘上已经不成立了（制品换过、命令改了、argv 不认了），
    账仍全绿 ⇒ 这就是「**多余格 / 幽灵格**」。本档**逐格回放**，把账与盘对上。

    判据：0 = 逐格全合 · 1 = 有 mismatch（判词点名到格）· 2 = **不给结论**（矩阵不在盘 / 零格 / 找不到制品）。
    """
    binz = resolve_bin(root, bin_path)
    if not os.path.isfile(binz):
        return 2, ['⛔ 不给结论：找不到 bin/zerg —— 先用 `bash scripts/build/build-all.sh --only-cli` 出新制品']
    mpath = matrix_path or os.path.join(root, MATRIX_REL)
    if not os.path.isfile(mpath):
        return 2, ['⛔ 不给结论：矩阵不在盘（%s）' % mpath]
    try:
        m = json.loads(read(mpath))
    except (OSError, ValueError) as e:
        return 2, ['⛔ 不给结论：矩阵读不回来（%s）' % e]
    cases = [c for c in (m.get('cases') or []) if (c.get('argv') or [])]
    if not cases:
        return 2, ['⛔ 不给结论：矩阵里没有带 argv 的格（零格 ⇒ 不许当绿）']
    matched, bad_lines = 0, []
    for c in cases:
        argvv = c['argv']
        rc1, b1 = run_once(binz, argvv)
        rc2, b2 = run_once(binz, argvv)
        want_rc, want_bytes = c.get('want_rc'), c.get('want_stdout_bytes')
        if (rc1, b1) == (rc2, b2) and rc1 == want_rc and b1 == want_bytes:
            matched += 1
            continue
        how = []
        if (rc1, b1) != (rc2, b2):
            how.append('两次现跑不等（%r/%r 字节 vs %r/%r 字节）' % (rc1, b1, rc2, b2))
        if rc1 != want_rc:
            how.append('退码 %r ≠ 账上 %r' % (rc1, want_rc))
        if b1 != want_bytes:
            how.append('stdout %r 字节 ≠ 账上 %r 字节' % (b1, want_bytes))
        bad_lines.append('R-6 幽灵格/多余格：%s ⇒ argv=%r · %s' % (c.get('id'), argvv, ' · '.join(how)))
    rc = 0 if not bad_lines else 1
    lines = ['R-6 逐格现跑（每格两遍 · 核退码与 stdout 字节数）：合 %d / %d 格 · mismatch %d 格'
             % (matched, len(cases), len(bad_lines))]
    return rc, lines + bad_lines


# ── T7 绿格现跑（`tiers.*.cells` 的 `want_rc`）· 2026-09-28 补 · 缺口 `GAP-20260928-32` ─────────
# CLI 的「不给结论」退码（`gate.go` 的 `exitBlocked` · §九 退码表：8 = 不给结论，
# 不是错也不是绿）。仅 T7 的 `state_dependent` 分支用它 —— 既有 T1–T6 的退码口径一字不动。
CLI_NO_CONCLUSION_RC = 8
PLACEHOLDER_RE = re.compile(r'^<[^<>]+>$')


def tier_cells(tiers, path='tiers'):
    """递归收集 `tiers` 下**所有** `cells` 列表 ⇒ [(登记路径, 格), …]（现读真件 3 格）。

    `cells` 只认「list of dict」这一种形状（格 = dict）；别的形状一律当「没登记」——
    宁可判「零格 ⇒ 2 不给结论」，也不许把不认识的东西当格混进来（防假覆盖）。
    """
    out = []
    if isinstance(tiers, dict):
        for k, v in tiers.items():
            if not isinstance(v, dict):
                continue
            cells = v.get('cells')
            if isinstance(cells, list):
                for i, c in enumerate(cells):
                    if isinstance(c, dict):
                        out.append(('%s.%s.cells[%d]' % (path, k, i), c))
            else:
                out.extend(tier_cells(v, '%s.%s' % (path, k)))
    return out


def step_name_candidates(binz):
    """占位符 `<步名>` 的真值来源：`gate run --list` **现读**的步名表 ⇒ (步名表, '') / (None, 原因)。"""
    p = subprocess.run([binz, 'gate', 'run', '--list'], capture_output=True, text=True,
                       stdin=subprocess.DEVNULL)
    if p.returncode != 0:
        return None, '`gate run --list` 退出 %s' % p.returncode
    names = []
    for ln in (p.stdout or '').splitlines():
        if '──' in ln or '清单' in ln:          # 表头/分隔行不是步名
            continue
        parts = ln.split(None, 2)
        if len(parts) == 3 and parts[2].strip():
            names.append(parts[2].strip())
    if not names:
        return None, '步名表里一行都没读到'
    return names, ''


def tier_cell_live(root, matrix, bin_path=None):
    """T7：逐格现跑 `tiers.*.cells[]` 的 `argv`，核 **实测 rc == 该格 `want_rc`**（每格两遍）。

    为什么：`cases[]` 是 must-fail 矩阵（`want_rc == 0` 判红）⇒ 绿格（`want_rc == 0`）只能登记在
    `tiers.*.cells` 里；而 `T1`–`T6` 与 Go 侧 `matrixTier()` 都只认 `tiers.local`/`tiers.public` 两个键
    ⇒ 那几格**没有任何机检读它**（恒绿/空转类结构病：`want_rc` 写错也不红）。本档把账（矩阵）与盘
    （制品）逐格对上，判词**点名到格 id**。

    判据：0 = 逐格合 · 1 = 有 mismatch · 2 = **不给结论**（找不到制品 / 零格 / 占位符判不了）。
    ★ 占位符：格 `argv` 里 `<步名>` 这类尖括号 token **逐字不是可跑的真值**（格自己的 `why` 说
      「= 脚本 `--list` 里任一步名」）⇒ 用 `gate run --list` 现读的**第一条**步名替换，替换后**照常**
      核 `want_rc`；替换源读不到 ⇒ 2（「判不了」不许当绿）。
    ★ 只核 `rc` 与「两遍退码一致」：本批绿格的 `why` 明说**不冻结** stdout 字节数（随步名 / 结果表 /
      矩阵涨落）⇒ 拿字节数当判据会变成一颗随仓内容自爆的地雷；字节数只作附注。
    ★ 状态依赖格（`state_dependent` · 2026-09-28 补 · 缺口 `GAP-20260928-34`）：有的绿格退码取决于
      **上下文/时序** —— 如 `gate results --json` 要看**那一趟门跑的结果台账**（`results.tsv`）落没落盘：
      门跑**进行中** ⇒ 退 8（CLI 自陈**不给结论**）；门跑完再跑 ⇒ 退 0。这种格 `want_rc=0` 在「门内」
      这个上下文里**不可能为真** ⇒ 对它判红是张冠李戴（红 = 账盘不符；8 = 没账可对）。凡格上声明了
      `state_dependent`（格级可选标记 · 值 = 为什么）、账上写的又是「该绿」（`want_rc == 0`）、现跑
      两遍都退 8 ⇒ 该格判「**判不了**」：不计合、不判红、逐字打印原因。其余任何 mismatch（含没声明
      标记的格、`want_rc != 0` 的格退 8）照旧判红、点名到格。★ 不许当空转口子：整块**一格都没判合**
      ⇒ 仍返 2 不给结论（判不了不许当绿，也不许让整块判据面被清空）。
    """
    binz = resolve_bin(root, bin_path)
    if not os.path.isfile(binz):
        return 2, ['⛔ 不给结论：找不到 bin/zerg —— T7 绿格现跑跑不起来（先用 '
                   '`bash scripts/build/build-all.sh --only-cli` 出**新**制品）'], [], {}
    cells = tier_cells(matrix.get('tiers'))
    if not cells:
        return 2, ['⛔ 不给结论：矩阵 `tiers.*.cells` **一格都没有**（声明面被清空 = 空转）'
                   '—— T7 判不了，不许当绿'], [], {'cells': 0}
    names, err = [], ''
    if any(PLACEHOLDER_RE.match(t) for _, c in cells for t in (c.get('argv') or [])):
        cand, err = step_name_candidates(binz)
        names = cand or []
    if err:
        return 2, ['⛔ 不给结论：占位符（`<步名>`）判不了 —— %s' % err], [], {'cells': len(cells)}
    bad, infos, n_ok, n_blocked, blocked_notes = [], [], 0, 0, []
    for path, c in cells:
        cid = c.get('id', '?')
        argv = c.get('argv') or []
        if not argv:
            bad.append('T7 绿格现跑：**格 %s**（登记在 %s）没有 argv ⇒ 声明面没有判据面可跑'
                       % (cid, path))
            continue
        ran = [names[0] if PLACEHOLDER_RE.match(t) else t for t in argv]
        rc1, b1 = run_once(binz, ran)
        rc2, b2 = run_once(binz, ran)
        want_rc = c.get('want_rc')
        if rc1 == rc2 and rc1 == want_rc:
            n_ok += 1
            infos.append('  合：格 %s（%s）⇒ 现跑 argv=%r rc=%r（两遍一致）· stdout %d 字节'
                         % (cid, path, ran, rc1, b1))
            continue
        # ★ 2026-09-28 补（缺口 `GAP-20260928-34`）：**状态依赖绿格**的登记口径（只在 T7 内，T1–T6 一字不动）。
        #   门内（`results.tsv` 未落盘）`gate results --json` 退 8 = CLI 自陈**不给结论**；门跑完同命令退 0。
        #   把 8 判红 = 把「没账可对」当成「账盘不符」⇒ 门⑪ 的绿会依赖「本机前序跑过门禁」这一副产品。
        #   ⇒ 声明了 `state_dependent` 且账上写「该绿」的格，现跑两遍都退 8 ⇒ 判「判不了」（不计合不判红）。
        sd = c.get('state_dependent')
        if want_rc == 0 and rc1 == rc2 == CLI_NO_CONCLUSION_RC \
                and isinstance(sd, str) and sd.strip():
            n_blocked += 1
            blocked_notes.append('  判不了：格 %s（登记在 %s）现跑 argv=%r ⇒ 退码 %d（CLI 自陈**不给结论**'
                                 '· 上下文缺那一趟门跑的结果台账）—— 该格声明为 `state_dependent`：%s'
                                 % (cid, path, ran, CLI_NO_CONCLUSION_RC, sd))
            continue
        how = []
        if rc1 != rc2:
            how.append('两次现跑退码不等（%r vs %r）—— 格不稳定' % (rc1, rc2))
        if rc1 != want_rc:
            how.append('退码 %r ≠ 账上 want_rc=%r' % (rc1, want_rc))
        bad.append('T7 绿格现跑：**格 %s**（登记在 %s）现跑 argv=%r ⇒ %s'
                   % (cid, path, ran, ' · '.join(how)))
    head = ['绿格现跑（`tiers.*.cells[]` 逐格核 want_rc · 每格两遍）：合 %d / %d 格 · '
            '判不了 %d 格 · mismatch %d 格' % (n_ok, len(cells), n_blocked, len(bad))]
    stat = {'cells': len(cells), 'ok': n_ok, 'bad': len(bad), 'blocked': n_blocked,
            'info': infos, 'blocked_notes': blocked_notes}
    if n_ok == 0 and not bad:
        # 全判不了 = 判据面被清空（与「零格 ⇒ 不给结论」同种空转）⇒ 不许当绿。
        return 2, ['⛔ 不给结论：T7 绿格现跑**一格都没判合**（判不了 %d 格 = 判据面空转）—— '
                   '`state_dependent` 是给状态依赖格判「判不了」的口径，不是让整块空转的口子'
                   % n_blocked] + blocked_notes, [], stat
    return (0 if not bad else 1), head, bad, stat


def emit_matrix(root, out_path=None, dry_run=False, bin_path=None):
    """按命令树 + 现跑制品**增量合并**矩阵。

    既有格：只刷新机器可派生列（`want_rc` / `want_stdout_bytes`），其余列逐字保留、一个不删；
    新命令（命令树里有、矩阵里一格都没有）：按候选面追加候选格。
    """
    binz = resolve_bin(root, bin_path)
    if not os.path.isfile(binz):
        return 2, ['⛔ 不给结论：找不到 bin/zerg —— 先用 bash scripts/build/build-all.sh --only-cli 出新制品']
    tree = parse_tree(root)
    if not tree:
        return 2, ['⛔ 不给结论：命令树读不出来（空转）']
    # `K2` 甲档归一（2026-09-24）：候选面要的「用法错」码**现读退码表**（不写死 —— 表是真源）。
    usage_code = dict((n, c) for c, n, _ in exitcode_table(root)).get('usage', 2)
    old = load_matrix(root) or {}
    old_cases = list(old.get('cases') or [])
    tree_paths = [p for p, _ in tree]

    cases, refreshed, kept, warned = [], 0, 0, []
    for c in old_cases:
        hit = dict(c)
        argv = c.get('argv') or []
        if not argv:
            warned.append('保留原值（口令为空 ⇒ 跑不了）：%s' % (c.get('id', '?'),))
        else:
            d, why = derive_columns(binz, argv)
            if d is None:
                warned.append('保留原值（现跑不成立）：%s —— %s' % (c.get('id', '?'), why))
            else:
                if (c.get('want_rc'), c.get('want_stdout_bytes')) != (d['want_rc'], d['want_stdout_bytes']):
                    refreshed += 1
                hit.update(d)          # ← 只动这两列；id/command/argv/why 一字不改
        cases.append(hit)
        kept += 1

    # 追加面：命令树里有、**矩阵里一格都没有**的命令 ⇒ 按候选面给格（缺口填充；已有格的命令不重复加）
    covered = set(c.get('command', '') for c in old_cases)
    added, dropped = 0, []
    for path, shape in tree:
        if path in covered:
            continue
        for name, argv, want, why in candidate_cases(path, shape, usage_code):
            d, w = derive_columns(binz, argv)
            if d is None:
                dropped.append((path, name, w))
                continue
            if d['want_rc'] != want:
                dropped.append((path, name, '退码 %d ≠ 判据语义要的 %d' % (d['want_rc'], want)))
                continue
            cases.append({'id': path.replace(' ', '/') + '#' + name, 'command': path, 'argv': argv,
                          'want_rc': d['want_rc'], 'want_stdout_bytes': d['want_stdout_bytes'],
                          'why': why})
            added += 1

    doc = {'id': old.get('id') or 'cli-contract.v1', 'note': MATRIX_NOTE,
           'exemptions': old.get('exemptions') or [], 'cases': cases}
    # ★ 2026-09-23（波A `A1` · 设计 §十三 `M2`）：`tiers`（两档矩阵的**档位声明**：本机档 / 公开面档，
    #   「公开面档存哪」写死在这一段里）必须**原样带上** —— 它不是派生列，而**整表重写**会把它丢掉；
    #   丢掉之后 `core/cmd/zerg/cli_matrix_test.go` 的 `loadMatrix` 会当场 Fatal
    #   （「写不清的档位等于没有档位」）。⇒ 旧件里有就逐字带上，没有就一个字节都不加（不造空壳）。
    if old.get('tiers'):
        doc['tiers'] = old['tiers']
    dst = out_path or os.path.join(root, MATRIX_REL)
    if not dry_run:
        with open(dst, 'w', encoding='utf-8') as f:
            json.dump(doc, f, ensure_ascii=False, indent=1)
            f.write('\n')
    have = set(c.get('command', '') for c in cases)
    ghosts = sorted(set(c.get('command', '') for c in cases) - set(tree_paths))
    lines = [('矩阵**干跑 · 未落盘**（--dry-run · 缺口 GAP-20260927-372）：计划写 %s' % dst)
             if dry_run else ('矩阵已**增量合并**（不是整表重写）：%s' % dst),
             '命令树 %d 条 · 格 %d → %d（既有 %d 格逐字保留 · 其中派生列刷新 %d 格 · 新命令追加 %d 格）'
             % (len(tree_paths), len(old_cases), len(cases), kept, refreshed, added),
             '有格的命令 %d 条 · 空白 %d 条 · 豁免 %d 条'
             % (len(have), len([p for p in tree_paths if p not in have]), len(doc['exemptions']))]
    for g in ghosts:
        lines.append('⚠ 矩阵里的 %r 指向命令树里没有的命令（T1 会判幽灵）—— 增量合并不静默删格，需人工定夺' % g)
    for d in dropped:
        lines.append('丢弃（新命令的候选格·退码不符/幽灵）：%s' % (d,))
    for w in warned:
        lines.append('⚠ ' + w)
    if not old_cases:
        lines.append('⚠ 既有矩阵为空 ⇒ 本次是按候选面**新建**（不是合并）：手写格无处可保，'
                     '且命令树里没有 must-fail 新面的命令会成空白（T1/T5 会判）')
    if out_path is None and not dry_run:
        # ★ §六 `R-6`（2026-09-24 · 波6 序6 · 缺口 `G-72`）：重冻**写盘之后**拿**新账**逐格现跑一遍
        #   （每格两遍 · 核退码与 stdout 字节数）—— **只报不判**（重冻的 rc 语义不因它改）；
        #   要一个「判」的 rc 就走 `--verify-live`（同一条 `live_verify`）。
        vrc, vlines = live_verify(root, bin_path=bin_path)
        lines.extend(vlines)
        if vrc != 0:
            lines.append('⚠ 重冻后的**新账**与现跑对不上（见上）—— 账与盘不一致就是「幽灵格」；'
                         '要判红请单跑 `--verify-live`（本次 rc=%d）' % vrc)
    return 0, lines


# ── `T5d`（题面作 `T6`）`why` ⟷ 值 同源：`why` 里的**绝对数字**必须与本格的值同源（或落进四类豁免）──
#   病灶（两处「各自按设计」的交叉处，实测）：`--emit-matrix` 只刷机器可派生列、`why` 一个字符都不碰
#   （本件 `emit_matrix` 内逐字「只动这两列；id/command/argv/why 一字不改」）；判定口
#   `cli_matrix_test.go` 的 `judgeCase` 只比 `want_rc` 与 stdout 字节数、**不读 `c.Why`** ⇒ 值漂了
#   why 不跟着漂，中间**没有一处**会红（实测：`gate/matrix#--json 不给字段` 的 why 写「stdout 0 字节」
#   而 `want_stdout_bytes` = 290）。本档就是那处「会红」。
#   判据面 = 三 regex（R1 字节读数 · R2 条数读数 · R3 退码读数）；命中处的数字必须**等于**本格对应列：
#   R1 ⟷ `want_stdout_bytes` · R3 ⟷ `want_rc` · R2 ⟷ 带 `*_public` 列的格数（「两档格 N → M 条」）。
#   四类豁免（`E1`–`E4`）的窗口**全部量自现件实测距离**（见逐行注释），且**只认数字之前**的标记：
WHY_R1_BYTES_RE = re.compile(r'(\d+)\s*字节')
WHY_R2_COUNT_RE = re.compile(r'(\d+)\s*(?:条|行|格|处|项|列|个|件|层|位)')
WHY_R3_RC_RE = re.compile(r'退\s*(\d+)')
#   E1 出处/编号：数字紧跟在 `第` 后（「设计稿 §二 第 1 条」「`zerg help` 第 1 行」= 引用，不是读数）。
#   E2 门槛/上限：数字前 6 字符内含 ≤/≥/</> 或 上限/门槛/最多/至少（现件：`深度 ≤ 3 层`）。
#   E3 历史/他格：**按分句判定**（2026-09-27 收口：原「变更史词固定 40 字符窗」会被同句里
#      **无关**的历史词触发 ⇒ `version#--json 不给字段` 那格的真读数「stdout 0 字节」被误豁免
#      —— 实测该处 `0` 与「改前退 1」同句、距 25 字符 < 40 ⇒ 假阴）。
#      格级前身词窗口 200（现件实测：「前身」距 `419` 25 字符、距那处 `退 1` 151 字符；而
#      `gate/run` 那句**真判据**「两档格 8 → 7 条」最近的标记在 190 字符外 ⇒ 200 是
#      「罩得住前身叙述、罩不住真断言」的实测取法）—— **本单不动它**（实测：把它改成分句窗
#      会把 `impact#字段面·未知字段` 的真历史引用「零命中 ⇒ 退 1」判红 = 制造假阳）。
#      变更史词（改前/旧值/曾）**只认同一分句**：分句边界 = 句末标点 `；;。！!？?` / 换行 / 右括号 `）)`；
#      标记还必须在数字**之前**。实测：`…；改前退 1）` 里标记与数字同分句 ⇒ 真历史引用照放（25 处不漏）；
#      而 `…改前退 1 与自家退码表不同向）+ stdout 0 字节（…` 里 `0` 与标记之间隔着 `）+ ` 一个边界
#      ⇒ 真断言必须判（旧 40 窗在此放行 = 本单要杀的那类假阴）。
#      ★ **单向**（只看数字**之前**）：双向窗口会把 `gate/matrix` 的真断言「0 字节」一起罩掉
#      （同格另有一句在后头的「改前退 1」，实测）。
#   E4 否定式：数字前 8 字符（**先去掉 markdown `*`**，否则「**不是** 0 字节」量不到）内含
#      不是/不为/不记/不用/非。
WHY_E2_MARKS = ('≤', '≥', '<', '>', '上限', '门槛', '最多', '至少')
WHY_E2_WINDOW = 6
WHY_E3_WIDE_MARKS = ('前身', '原本', '已删', '已移走', '原值')
WHY_E3_WIDE_WINDOW = 200
WHY_E3_TIGHT_MARKS = ('改前', '旧值', '曾')
#   E3 变更史词的**分句边界**：句末标点 `；;。！!？?` · 换行 · 右括号 `）)`。
#   ★ 只收「收尾」标点，不收逗号/顿号/间隔号 `·`（实测：把 `·` 也算边界，会把
#     `gate/run#--json 不给字段（K2）` 里「靠「本机退 1」…」那处**真历史引用**判红 = 假阳）。
WHY_E3_CLAUSE_BREAK_RE = re.compile(r'[；;。！!？?\n）)]')
WHY_E4_MARKS = ('不是', '不为', '不记', '不用', '非')
WHY_E4_WINDOW = 8


def why_clause_start(why, i):
    """数字 `i` 所在**分句**的起点（`i` = 数字在 `why` 里的下标）。

    边界 = 数字**之前**最后一个句末标点（`；;。！!？?`）/ 换行 / 右括号 `）)` 之后。
    整条 `why` 没有边界（无标点长句）⇒ 0（整条 = 一个分句 —— 保守：宁可放行同条里的历史叙述，
    也不靠猜去判红；本件现件实测 521 格里没有这种无边界长句带真断言的形态）。
    """
    start = 0
    for mt in WHY_E3_CLAUSE_BREAK_RE.finditer(why, 0, i):
        start = mt.end()
    return start


def why_claim_exempt(why, mt):
    """这处命中的数字落进哪一类豁免 ⇒ 类名；`None` = **本格断言**（要判）。

    次序即语义：`E4 否定式`（最贴切的一类）先于 `E3 历史/他格` —— 实测 `script/inventory/sync`
    的「stdout **不是** 0 字节」两类都沾（前 26 字符另有「改前」）⇒ 先判 E4 才不会被 E3 抢走标签。
    `E3` 内部次序也不动：`前身词 200 字符窗` 先于 `变更史词分句`（宽的先罩 —— 两档都沾时标签取宽档，
    实测 `impact#字段面·未知字段` 的「419 字节」靠宽档放行）。
    """
    pre = why[:mt.start(1)]
    if pre.rstrip().endswith('第'):
        return 'E1 出处/编号'
    if any(m in pre[-WHY_E2_WINDOW:] for m in WHY_E2_MARKS):
        return 'E2 门槛/上限'
    if any(m in pre[max(0, len(pre) - WHY_E4_WINDOW):].replace('*', '') for m in WHY_E4_MARKS):
        return 'E4 否定式'
    if any(m in pre[max(0, len(pre) - WHY_E3_WIDE_WINDOW):] for m in WHY_E3_WIDE_MARKS):
        return 'E3 历史/他格'
    if any(m in pre[why_clause_start(why, mt.start(1)):] for m in WHY_E3_TIGHT_MARKS):
        return 'E3 历史/他格'
    return None


def why_value_claims(cases):
    """`T5d` 判定口：逐格核 `why` 里的绝对数字 ⟷ 本格的值 ⇒ `(errs, stat)`。

    `errs` 只装**不一致**（判据要的那部分）；`stat` 供机器面头行用；`cases` 不是表 ⇒ `(None, None)`
    （调用方照「不给结论 rc=2」处理 —— 「判不了」不许当绿）。
    """
    if not isinstance(cases, list):
        return None, None
    pub = len([c for c in cases
               if c.get('want_rc_public') is not None or c.get('want_stdout_bytes_public') is not None])
    errs, exempt, judged, bad_cells = [], {}, 0, set()
    for c in cases:
        why = c.get('why')
        if not isinstance(why, str):
            continue
        cid = c.get('id', '?')
        hits = []
        for kind, rx in (('R1', WHY_R1_BYTES_RE), ('R2', WHY_R2_COUNT_RE), ('R3', WHY_R3_RC_RE)):
            for mt in rx.finditer(why):
                hits.append((mt.start(), kind, mt))
        for _, kind, mt in sorted(hits, key=lambda h: h[0]):
            ex = why_claim_exempt(why, mt)
            if ex:
                exempt[ex] = exempt.get(ex, 0) + 1
                continue
            judged += 1
            num = int(mt.group(1))
            if kind == 'R1':
                val, col = c.get('want_stdout_bytes'), 'want_stdout_bytes'
            elif kind == 'R3':
                val, col = c.get('want_rc'), 'want_rc'
            else:
                val, col = pub, '带 `*_public` 列的格数'
            if val is None:
                errs.append('T5d why⟷值：%s 的 why 有一处%s读数（%r）而本格**没有 %s** ⇒ 判不了'
                            % (cid, kind, mt.group(0), col))
                bad_cells.add(cid)
                continue
            if num == val:
                continue
            quote = why[max(0, mt.start(1) - 12):mt.end(1) + 12].replace('\n', '⏎')
            errs.append('T5d why⟷值：%s ｜ why 里「%s」= %d ≠ %s = %d（原句「…%s…」）'
                        % (cid, mt.group(0).strip(), num, col, val, quote))
            bad_cells.add(cid)
    stat = {'judged': judged, 'bad_cells': len(bad_cells), 'bad': len(errs), 'exempt': exempt}
    return errs, stat


def check(root, no_runtime=False, bin_path=None):
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

    # ---- T5b 格级棘轮（2026-09-27 父代理补 · 缺口 GAP-20260926-13）----
    #   T5 锚的是「命令级空白」（该命令有没有**任何**一格）⇒ 对「同命令内部掉一格」完全失明 ✗。
    #   本档锚**格数**：must-fail 格数低于 baseline 记的数即红（格级只许上调；确需下调须显式改
    #   baseline 并写明理由 —— 不许静默掉格）。
    base_mf = baseline.get('mustfail_cases')
    if not isinstance(base_mf, int):
        return 2, ['⛔ 不给结论：baseline 棘轮缺 `mustfail_cases`（格级锚点）—— 格级掉格会静默（%s）'
                   % BASELINE_REL]
    if n_mustfail < base_mf:
        errs.append('T5b 格级棘轮：must-fail 格 %d 条 < baseline %d 条（格级只许**上调**；'
                    '确需下调须显式改 %s 并写明理由 —— 防「同命令内部掉一格」静默）'
                    % (n_mustfail, base_mf, BASELINE_REL))

    # ---- T5c 树条数**相等判**（2026-09-27 父代理补 · 补 T5/T5b 同族的「不等式恒绿」缺口）----
    #   T5（空白条数）与 T5b（格数）都是**不等式**棘轮 ⇒ 树条数本身漂了时两道判据都不红：
    #   树**掉**一条命令 = T1/T2 对那条命令整片静默失效；树**多**一条命令 = 契约被悄悄放宽。
    #   本档锚**命令树条数**：baseline 里写死的数必须**等于**现算真值（`parse_tree` 的条数）。
    base_cmds = baseline.get('commands')
    if not isinstance(base_cmds, int):
        return 2, ['⛔ 不给结论：baseline 棘轮缺 `commands`（树条数锚点）—— 树条数漂了会静默（%s）'
                   % BASELINE_REL]
    if len(tree) != base_cmds:
        errs.append('T5c 树条数相等判：现算命令树 %d 条 ≠ baseline 记的 %d 条（差 %+d）—— 两处真源开始漂了；'
                    '确需变动须显式改 %s 并写明理由'
                    % (len(tree), base_cmds, len(tree) - base_cmds, BASELINE_REL))

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

    # ---- T5e `field_surface_greens` 键现读对拍（缺口 `GAP-20260927-315`）----
    #   本键此前**全仓零读者**（双向证过）：登记了「K2 乙档绿格数」却没有任何判据核它 ⇒ 漂了也不红。
    #   现读真值 = 矩阵 `tiers.field_surface.<…>.cells` 各块长度之和（`甲·开放` 无 cells 块 ⇒ 只数乙档）。
    #   取不到 ⇒ **2 不给结论**（与同族 rc=2 口径一致）。
    fsg = baseline.get('field_surface_greens')
    if fsg is None:
        return 2, ['⛔ 不给结论：baseline 缺 `field_surface_greens`（K2 乙档绿格计数的棘轮锚）—— 判不了']
    fs_tier = (matrix.get('tiers') or {}).get('field_surface') or {}
    fs_cells, got_fs = 0, False
    if isinstance(fs_tier, dict):
        for _v in fs_tier.values():
            if isinstance(_v, dict) and isinstance(_v.get('cells'), list):
                fs_cells += len(_v['cells']); got_fs = True
    if not got_fs:
        return 2, ['⛔ 不给结论：矩阵 `tiers.field_surface.*.cells` 读不到 ⇒ T5e 判不了（不许当绿）']
    if fs_cells != int(fsg):
        errs.append('T5e field_surface_greens：矩阵现读 %d 格 ≠ baseline 记的 %d —— '
                    '该键此前零读者，现纳入对拍（棘轮只许**下调**）' % (fs_cells, int(fsg)))

    # ---- T5 空白 + baseline 棘轮 ----
    if len(blanks) > int(baseline['blanks']):
        errs.append('T5 棘轮：空白 %d 条 > baseline %d 条（棘轮只许**下调**；新增命令必须补 must-fail）'
                    % (len(blanks), int(baseline['blanks'])))

    # ---- T5d `why` ⟷ 值 同源（2026-09-27 补 · 题面作 `T6`）----
    #   与 T5b/T5c 同族（都在看「矩阵这笔账自己对不对」）：T5b 锚格数、T5c 锚树条数、本档锚
    #   **同格内 why 的绝对数 ⟷ 值列**（解绑的机制本体见 `why_value_claims` 上方一段）。
    t5d_errs, t5d_stat = why_value_claims(cases)
    if t5d_stat is None:
        return 2, ['⛔ 不给结论：why ⟷ 值的判据面拿不到（`cases` 不是表）—— 判不了不许当绿']
    errs.extend(t5d_errs)

    # ---- T6 引用 M6 R7 ----
    gates = registry_gate_names(root)
    if gates is None:
        return 2, ['⛔ 不给结论：契约登记表读不出来（%s）—— T6 的引用面无法判' % REGISTRY_REL]
    if not any('check-cli-contract.py' in g for g in gates):
        errs.append('T6 引用：契约登记表里**没有**任何条目的 gate 字段点名 check-cli-contract.py'
                    '（引用面 = 0 ⇒ 本门挂在空气上）')
    if 'M6 R7' not in read(__file__):
        errs.append('T6 引用：本门头部没有引用 §九 M6 R7（本门不重写判据，只引用它）')

    # ---- R7 族名闭合（§十二 P-038 的落判 · 开工单 T-43 ①）----
    fam = [p.split()[0] for p, _ in tree]
    for bad in sorted(set(f for f in fam if f == 'node')):
        errs.append('R7 族名：命令树里出现 `node` 族 —— 族名只有一个 `agent`（§十二 P-038）')
    hits = node_family_hits(root)
    for h in hits[:20]:
        errs.append('R7 族名：全仓还有 `zerg node <动作>` 的写法 ⇒ %s' % h)
    if len(hits) > 20:
        errs.append('R7 族名：另有 %d 处（上面只列前 20）' % (len(hits) - 20))

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

    # ---- T7 绿格现跑（`tiers.*.cells` 的 `want_rc` · 2026-09-28 补 · 缺口 GAP-20260928-32）----
    #   与 T5b/T5c/T5d 同族（都在看「矩阵这笔账自己对不对」）：那三档只判 `cases[]` 这笔账的形状，
    #   本档判**绿格那块声明面**（`tiers.*.cells`）—— 它此前没有任何机检读（恒绿/空转结构病：
    #   `want_rc` 写错也不红）。判据面走**默认档**（不需要 `--verify-live`），矩阵漂了门就红。
    t7, t7_info = '（--no-runtime：夹具模式跳过）', []
    if not no_runtime:
        rc7, head7, bad7, stat7 = tier_cell_live(root, matrix, bin_path=bin_path)
        if rc7 == 2:
            return 2, head7
        t7 = head7[0]
        t7_info = stat7.get('info') if isinstance(stat7.get('info'), list) else []
        t7_info = list(t7_info) + [x for x in (stat7.get('blocked_notes') or [])
                                   if isinstance(x, str)]
        errs.extend(bad7 or [])

    lines.append('命令树 %d 条 · 矩阵 case %d 条（must-fail %d）· 空白 %d 条 · 豁免 %d 条'
                 % (len(tree_paths), len(cases), n_mustfail, len(blanks), len(exemptions)))
    lines.append('退码表 %d 格（数字唯一：%s）· 常量 %d 个 · 裸数字退出码 %d 处'
                 % (len(table), '是' if len(set(codes)) == len(codes) else '否',
                    len(consts), len(bare_numeric_returns(root))))
    lines.append('T4 ' + t4)
    lines.append('T7 ' + t7)
    lines.extend(t7_info)
    lines.append('T6 契约登记表 gate 字段点名本脚本：%s'
                 % ('是' if any('check-cli-contract.py' in g for g in gates) else '否'))
    lines.append('R7 族名闭合（`agent` 唯一 · 全仓 `zerg node <动作>` 命中）：%d 处' % len(hits))
    lines.append('T5d why ⟷ 值 同源（三 regex：字节/条数/退码）：判 %d 处 · 不一致 %d 格 %d 处 · '
                 '免于判 %d 处（%s）'
                 % (t5d_stat['judged'], t5d_stat['bad_cells'], t5d_stat['bad'],
                    sum(t5d_stat['exempt'].values()),
                    ' · '.join('%s %d' % kv for kv in sorted(t5d_stat['exempt'].items())) or '无'))
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
    #   夹具随动（缺口 `GAP-20260927-315` 的 T5e 判据面）：T5e 以 `tiers.field_surface.*.cells` 各块
    #   长度之和为**现读真值**对拍 `baseline['field_surface_greens']`；夹具不带 `tiers.field_surface`
    #   ⇒ T5e 取不到面即 rc=2，**全部自检格一进门就被 T5e 顶掉**（19 格全红）。故夹具必须带该块，
    #   形状照真源 `core/cmd/zerg/testdata/cli-matrix.json`（甲 · 开放字段面 0 格 ＋ 乙 · 闭集字段面 2 格）。
    "tiers": {
        "field_surface": {
            "甲 · 开放字段面": {"cells": []},
            "乙 · 闭集字段面": {"cells": [
                {"id": "version#绿格甲", "argv": ["version"], "want_rc": 0, "why": "夹具绿格"},
                {"id": "core/stop#绿格乙", "argv": ["core", "stop"], "want_rc": 0, "why": "夹具绿格"},
            ]},
        },
    },
}
GOOD_BASELINE = {"id": "cli-contract-baseline.v1", "blanks": 0, "mustfail_cases": 2,
                 "commands": 2, "field_surface_greens": 2,
                 "note": "只许下调：本门落地时空白已清零 ⇒ 从此任何空白即红；`commands` 是 T5c 的**相等判**锚点（夹具树 = version + core stop 两条）"}
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

        # ⑫ R7 负控：全仓另有 `zerg node <动作>` 的写法 ⇒ 1（族名闭合有牙）
        #    ★ 夹具里那处写法**用拼接**写出来（不落成字面量）：本门自己的负控痕迹不该变成
        #      「全仓 grep」的噪声 —— 判据要把「真有人在文档/代码里这么写」和「测试自己造的反例」
        #      分开，否则任何一次复核都要先解释那 3 行。
        r = fixture(os.path.join(tmp, 'r7'))
        with open(os.path.join(r, 'docs-命令说明.md'), 'w', encoding='utf-8') as f:
            f.write('跑 `zerg ' + 'node reap` 收残留\n')
        rc, lines = check(r, no_runtime=True)
        case('⑫ R7 负控（另有 `zerg node <动作>` 写法 ⇒ 1）', 1, rc)
        if not any('R7 族名' in l for l in lines):
            bad.append('⑫ R7 负控：判词里没有 R7 族名')

        # ⑬ R7 正控：同一棵树上把那个写法去掉 ⇒ 回绿（证明红是红在那一处，不是红在夹具）
        os.remove(os.path.join(r, 'docs-命令说明.md'))
        rc, lines = check(r, no_runtime=True)
        case('⑬ R7 正控（去掉那处写法 ⇒ 0）', 0, rc)

        # ⑭ `--emit-matrix` 空转负控：夹具里没有 `bin/zerg` ⇒ **2 不给结论**（不假绿、不空转当绿）
        rc, lines = emit_matrix(os.path.join(tmp, 'good'))
        case('⑭ emit-matrix 负控（没有 bin/zerg ⇒ 2）', 2, rc)
        if not any('找不到 bin/zerg' in l for l in lines):
            bad.append('⑭ emit-matrix 负控：判词里没点名缺制品')

        # ⑮ `--emit-matrix` **字节数 + 增量合并**（合成 `bin/zerg` 夹具：输出含中文 ⇒ 字节数 ≠ 字符数）。
        #   四条判据：① 手写格逐字保留（旧实现整表重写 ⇒ 这里必红）· ② 既有格只刷派生列（argv/why 不动）·
        #   ③ 派生列是**字节数**不是字符数（bug ① 的成对负控）· ④ 新命令追加格 + 既有 exemptions 保留。
        r = os.path.join(tmp, 'emit')
        seed = json.loads(json.dumps(GOOD_MATRIX))
        seed['cases'] = [
            {"id": "version#手写格", "command": "version", "argv": ["version"],
             "want_rc": 2, "want_stdout_bytes": 0, "why": "手写：版本号后面跟东西 ⇒ 2"},
            {"id": "version#未知旗标", "command": "version", "argv": ["version", "--x"],
             "want_rc": 9, "want_stdout_bytes": 0, "why": "旧 why（候选面不许覆盖它）"},
        ]
        seed['exemptions'] = [{"command": "core stop", "reason": "夹具豁免（合并不许丢）"}]
        fixture(r, matrix=seed)
        FAKE_OUT = '用法错：中文输出（字节数 ≠ 字符数）\n'
        os.makedirs(os.path.join(r, 'bin'), exist_ok=True)
        with open(os.path.join(r, 'bin', 'zerg'), 'w', encoding='utf-8') as f:
            f.write("#!/bin/sh\nprintf '%s\\n' '用法错：中文输出（字节数 ≠ 字符数）'\nexit 2\n")
        os.chmod(os.path.join(r, 'bin', 'zerg'), 0o755)
        dst = os.path.join(tmp, 'emit-out.json')
        rc, lines = emit_matrix(r, dst)
        case('⑮ emit-matrix（增量合并 ⇒ 0）', 0, rc)
        try:
            m = json.loads(open(dst, encoding='utf-8').read())
        except (OSError, json.JSONDecodeError) as e:
            m = {}
            bad.append('⑮ emit-matrix：产物读不回来（%s）' % e)
        cases_out = m.get('cases') or []
        by = dict((c.get('id'), c) for c in cases_out)
        want_bytes, want_chars = len(FAKE_OUT.encode('utf-8')), len(FAKE_OUT)
        if want_bytes == want_chars:
            bad.append('⑮ 夹具不具区分度：中文输出的字节数与字符数相等')
        #   ① 手写格：**身份列**逐字保留、**派生列**刷新（「只更新机器可派生列」的逐字落点）
        first = cases_out[0] if cases_out else {}
        for k in ('id', 'command', 'argv', 'why'):
            if first.get(k) != seed['cases'][0].get(k):
                bad.append('⑮ 手写格的 %s 没逐字保留（第 1 格 = %r）—— 整表重写的老病' % (k, first,))
        if (first.get('want_rc'), first.get('want_stdout_bytes')) != (2, want_bytes):
            bad.append('⑮ 手写格的派生列没刷对：%r（要 rc=2 / %d 字节）' % (first, want_bytes))
        old_auto = by.get('version#未知旗标') or {}
        if old_auto.get('argv') != ["version", "--x"] or old_auto.get('why') != '旧 why（候选面不许覆盖它）':
            bad.append('⑮ 既有格的 argv/why 被候选面覆盖了：%r' % (old_auto,))
        got = old_auto.get('want_stdout_bytes')
        if (old_auto.get('want_rc'), got) != (2, want_bytes):
            bad.append('⑮ 既有格派生列没刷对：want_rc=%r want_stdout_bytes=%r（要 2 / %d 字节；'
                       '记成 %d 就是**字符数** —— bug ①）'
                       % (old_auto.get('want_rc'), got, want_bytes, want_chars))
        if not by.get('core/stop#缺 --confirm（D3）'):
            bad.append('⑮ 命令树里有、矩阵里一格都没有的命令**没有**追加候选格（新命令会成空白）')
        if (m.get('exemptions') or []) != seed['exemptions']:
            bad.append('⑮ 既有 exemptions 被丢了：%r' % (m.get('exemptions'),))
        # ⑯ `--emit-matrix` 幂等正控：同一棵树连跑两遍 ⇒ 产物**逐字节**相同
        dst2 = os.path.join(tmp, 'emit-out2.json')
        rc2, _ = emit_matrix(r, dst2)
        same = os.path.isfile(dst2) and open(dst, 'rb').read() == open(dst2, 'rb').read()
        case('⑯ emit-matrix 幂等（连跑两遍逐字节相同 ⇒ 0）', 0, rc2)
        if not same:
            bad.append('⑯ emit-matrix 不是幂等的：同一棵树上连跑两遍的产物不同')

        # ⑰-⑳ §六 `R-6`（2026-09-24 · 波6 序6 · 缺口 G-72）：**新负控档**（逐格回放，抓「多余格 / 幽灵格」）
        #   正控：账与盘对得上 ⇒ 0；负控三态：幽灵格 ⇒ 1 · 删一格 ⇒ 该器 rc≠0 · 缺制品 ⇒ 2（不假绿）。
        r6 = fixture(os.path.join(tmp, 'r6'))
        os.makedirs(os.path.join(r6, 'bin'), exist_ok=True)
        with open(os.path.join(r6, 'bin', 'zerg'), 'w', encoding='utf-8') as f:
            f.write('#!/bin/sh\nexit 2\n')          # 两格都是 rc=2 / stdout 0 字节 ⇒ 与 GOOD_MATRIX 逐格对得上
        os.chmod(os.path.join(r6, 'bin', 'zerg'), 0o755)
        rc, lines = live_verify(r6)
        case('⑰ R-6 正控（账与盘逐格对得上 ⇒ 0）', 0, rc, '\n'.join(lines))
        if not any('合 2 / 2 格' in l and 'mismatch 0 格' in l for l in lines):
            bad.append('⑰ R-6 正控：判词没有「合 2 / 2 格 · mismatch 0 格」—— %r' % (lines,))
        rc_chk, lines_chk = check(r6, no_runtime=True)
        case('⑰ R-6 正控（不动 ⇒ 该器仍 0 且四数不变）', 0, rc_chk, '\n'.join(lines_chk))
        m4 = json.loads(json.dumps(GOOD_MATRIX))
        m4['cases'][1]['want_rc'] = 0               # 账上写 0、盘上仍是 2 ⇒ 幽灵格
        r6b = fixture(os.path.join(tmp, 'r6b'), matrix=m4)
        os.makedirs(os.path.join(r6b, 'bin'), exist_ok=True)
        with open(os.path.join(r6b, 'bin', 'zerg'), 'w', encoding='utf-8') as f:
            f.write('#!/bin/sh\nexit 2\n')
        os.chmod(os.path.join(r6b, 'bin', 'zerg'), 0o755)
        rc, lines = live_verify(r6b)
        case('⑱ R-6 负控（幽灵格：账 rc=0 盘 rc=2 ⇒ 1）', 1, rc)
        if not any('幽灵格' in l and 'core/stop' in l for l in lines):
            bad.append('⑱ R-6 负控：判词没有点名那一格 —— %r' % (lines,))
        m4c = json.loads(json.dumps(GOOD_MATRIX))
        m4c['cases'] = [c for c in m4c['cases'] if c.get('command') != 'core stop']   # 删一格
        rc, lines = check(fixture(os.path.join(tmp, 'r6c'), matrix=m4c), no_runtime=True)
        if rc == 0:
            bad.append('⑲ R-6 负控：**删一格**之后该器仍 rc=0（命令没格也不红 ⇒ 账可以漏格）')
        if not any('T1 覆盖' in l and 'core stop' in l for l in lines):
            bad.append('⑲ R-6 负控：删一格后判词没有点名 `core stop` —— %r' % (lines,))
        r6d = fixture(os.path.join(tmp, 'r6d'))       # 无 bin/zerg
        rc, lines = live_verify(r6d)
        case('⑳ R-6 负控（没有制品 ⇒ 2 不给结论）', 2, rc)
        if not any('找不到 bin/zerg' in l for l in lines):
            bad.append('⑳ R-6 负控：判词没有点名缺制品')

        # ㉑-㉓ 缺口 `GAP-20260928-40`：T7（绿格现跑）「判不了」口径的**在册成对负控**（三条）。
        #   夹具 bin/zerg：argv 首词 `blk` ⇒ 退 8（CLI 自陈不给结论）；`ok` ⇒ 退 0；其余 ⇒ 退 2。
        #   ✗ 不新增退码、✗ 不改既有判据语义 —— 只把 T7 已实现的三态（判不了 / 判红 / 空转不给结论）
        #   逐态钉上负控，防后续改动把它悄悄变成恒绿或误红（与既有 ⑰-⑳ R-6 一族同体例）。
        t7r = fixture(os.path.join(tmp, 't7sd'))
        os.makedirs(os.path.join(t7r, 'bin'), exist_ok=True)
        with open(os.path.join(t7r, 'bin', 'zerg'), 'w', encoding='utf-8') as f:
            f.write('#!/bin/sh\ncase "$1" in blk) exit 8 ;; ok) exit 0 ;; *) exit 2 ;; esac\n')
        os.chmod(os.path.join(t7r, 'bin', 'zerg'), 0o755)
        # 正路（②）：state_dependent 格两遍都退 8 ⇒ 判「判不了」（不计合、不判红）⇒ 整块仍 0
        m7 = json.loads(json.dumps(GOOD_MATRIX))
        m7['tiers'] = {'local': {'cells': [
            {"id": "sd#blk", "argv": ["blk"], "want_rc": 0, "state_dependent": "要看那一趟门跑的结果台账"},
            {"id": "ok#cell", "argv": ["ok"], "want_rc": 0},
        ]}}
        rc, lines, _bad7, stat7 = tier_cell_live(t7r, m7)
        case('㉑ T7 判不了正路（state_dependent 两遍退 8 ⇒ 判不了、不判红 ⇒ 0）', 0, rc, '\n'.join(lines))
        if stat7.get('blocked') != 1 or stat7.get('ok') != 1:
            bad.append('㉑ T7 判不了正路：blocked=%r ok=%r（要 1 / 1）'
                       % (stat7.get('blocked'), stat7.get('ok')))
        # 负控（①）：声明了 state_dependent 而 want_rc≠0、退 8 ⇒ **必红并点名该格**（不是判不了）
        m7b = json.loads(json.dumps(m7))
        m7b['tiers']['local']['cells'] = [
            {"id": "sd#wrong", "argv": ["blk"], "want_rc": 2, "state_dependent": "要看那一趟门跑的结果台账"},
            {"id": "ok#cell", "argv": ["ok"], "want_rc": 0},
        ]
        rc, lines, bad7b, _stat7b = tier_cell_live(t7r, m7b)
        case('㉒ T7 负控（state_dependent 而 want_rc≠0、退 8 ⇒ 1 判红）', 1, rc, '\n'.join(lines))
        if not any('sd#wrong' in x for x in bad7b):
            bad.append('㉒ T7 负控：判词没有点名 sd#wrong')
        # 空转负控（③）：**全部格都判不了** ⇒ 2 不给结论（判不了不许当绿）
        m7c = json.loads(json.dumps(m7))
        m7c['tiers']['local']['cells'] = [
            {"id": "sd#a", "argv": ["blk"], "want_rc": 0, "state_dependent": "要看那一趟门跑的结果台账"},
            {"id": "sd#b", "argv": ["blk"], "want_rc": 0, "state_dependent": "要看那一趟门跑的结果台账"},
        ]
        rc, lines, _bad7c, _stat7c = tier_cell_live(t7r, m7c)
        case('㉓ T7 空转负控（全部格判不了 ⇒ 2 不给结论）', 2, rc, '\n'.join(lines))

        print('自检结论: %s（失败 %d 条）' % ('全过' if not bad else '不过', len(bad)))
        for b in bad:
            print('✗ ' + b)
        return 0 if not bad else 2
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


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
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument('root', nargs='?', default='.')
    ap.add_argument('--bin', metavar='PATH', default=None,
                    help='缺口 GAP-20260926-12：指定制品路径。不给 = <root>/bin/zerg；'
                         '给了 = 用该路径（绝对化后）—— 仓外临时仓根复用本机真制品，免造 bin/')
    ap.add_argument('--self-test', action='store_true')
    ap.add_argument('--no-runtime', action='store_true',
                    help='**只给夹具用**：跳过 T4 的 go test（真目标不许带这个开关）')
    ap.add_argument('--emit-matrix', metavar='PATH', nargs='?', const='',
                    help='按命令树 + 现跑 bin/zerg **重生成** must-fail 矩阵（先用 '
                         '`bash scripts/build/build-all.sh --only-cli` 出新制品）')
    ap.add_argument('--dry-run', action='store_true',
                    help='**只判定 · 不落任何产物**（缺口 GAP-20260927-372）：与 --emit-matrix 同给时只报将要合并的'
                         '增量、**不写**矩阵件；不给 --emit-matrix 时 = 只读判定（同裸跑）')
    ap.add_argument('--verify-live', action='store_true',
                    help='§六 R-6：拿冻结的矩阵逐格现跑同一 argv（每格两遍）、核 rc 与 stdout 字节数 '
                         '—— 抓「多余格 / 幽灵格」（0 全合 / 1 mismatch / 2 不给结论）')
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = find_root(a.root)
    if root is None:
        print('⛔ 不给结论：找不到仓根（从 %s 上溯找不到 %s）' % (a.root, MAIN_REL))
        return 2
    if a.verify_live:
        rc, lines = live_verify(root, bin_path=a.bin)
        print('门⑪ · §六 `R-6` 逐格回放（仓根 %s）' % root)
        for l in lines:
            print('  ' + l)
        print('⇒ R-6 ' + ('全合（rc=0）' if rc == 0 else
                          ('判红（rc=1）：账与盘对不上 —— 逐格看上面的判词' if rc == 1 else
                           '不给结论（rc=2）：缺件/零格 —— 不许当绿')))
        return rc
    if a.emit_matrix is not None:
        rc, lines = emit_matrix(root, a.emit_matrix or None, dry_run=a.dry_run, bin_path=a.bin)
        for l in lines:
            print('  ' + l)
        return rc
    rc, lines = check(root, no_runtime=a.no_runtime, bin_path=a.bin)
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

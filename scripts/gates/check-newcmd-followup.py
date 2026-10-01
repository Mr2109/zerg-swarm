#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-newcmd-followup.py —— 「新命令随动总账」**只读聚合门**（第三批 甲④ · 治「漏随动」第 7 次）

病灶（逐字照用户口径）：一条**新增命令 path** 落地时，要在**四个面**上同时随动 ——
  · 面① **dryrun 落点**：实现件若含 `--dry-run` 语义 ⇒ 必须在 `core/internal/contract/dryrun-semantics.json` 的 `件[]` 里登记（门⑩ 的口径）；
  · 面② **`cli-matrix.json` must-fail**：该 path 至少一格 `want_rc != 0`（门⑪ `T1`/`T2`）；
  · 面③ **`cli-contract-baseline.json` 计数**：`commands` == 现读 path 条数 · `mustfail_cases` == 现读格数（门⑪ `T5` 的棘轮面）；
  · 面④ **`公开标记.tsv` + 两器排除**：实现件必须在标记表里有一行；判 `public=no` 的件必须在**侧甲**
    （`scripts/build/publish-public.sh` 的 `EXCLUDES`）里，否则**两器都发**（门57 `R2` 的口径）。
★ 第四批（laneB374）按盘账 `laneB339` 的缺面清单**真扩五面**（同体例：真源件 · 判法 · 退码语义 · 自证格）：
  · 面⑤ **门73 旗标名表**：新 path 用法串里写成**取值形态**的旗标，必须在
    `func valueFlagName` ∪ `parseInvocation` 专用 case 里（真源 `core/cmd/zerg/main.go` · 门73 同一组取数）；
    不认 ⇒ 真给退 2「未知旗标」= 命令不可用（已实撞 `--by` / `--allow-cross-root`）；
  · 面⑥ **门⑤ `error.kind` 闭集**：新 path 实现件里 `setErr("…")` / `"kind":"…"` 的字面量必须在
    `core/cmd/zerg/errors.go` 的 `errorKinds` 表内（真源同一处 · 门⑤ 口径；闭集外 kind ⇒ 缺）；
  · 面⑦ **门⑨ 退码表 + 命名空间**：新 path 实现件里的 `os.Exit(N)` / `exit<名> = N` / `exitCode = N`
    必须落进 `core/cmd/zerg/exitcodes.go` 的 `exitcodeTable` ∪ `exitcodeReserved`，且被
    `core/internal/contract/exitcode-namespaces.json` 的 `cli` 命名空间覆盖（真源两处 · 门⑨ 口径）；
  · 面⑧ **门⑪ T5d 归属面（只报）**：新 path 的 must-fail 格若缺 `why` ⇒ T5d（`why`⟷值）判不了；
    T3（退码表唯一）/ T6（registry 引用面）是**全局唯一性**判据、按 path 无归属 ⇒ 本面**只报**，不改退码；
  · 面⑨ **docs 生成物（只报）**：新 path 应在 `docs/zh` + `docs/en/参考-命令行.md` 生成区有一格
    （真源 `scripts/docs/gen-cli-reference.py` 的 `--check` 目标；drift 已由 `docs` scope 兜 ⇒ 本面**只报**）。
今天这些面**各由不同的门看**，于是「新命令只补了其中几面」时**没有任何一门会红**（这就是第 7 次漏随动）。

本门做的只有一件事：**以冻结的 path 全集为基线**，把**新增 path** 逐面打印**齐/缺**，缺任一面即 rc=1 并**点名**。

判据（**只读** · 三档退码 0/1/2）
  C0 **解析自证（没少检的机检判据）**：`commands = []*command{` 块内 `path:\s*\[\]string\{` 的**命中处数 M**
     必须 == **解析出的 path 条数 N**（M != N ⇒ 有 path 被解析漏读 ⇒ **rc=2 不给结论**，不许拿漏读的集合当基线）；
     且 `N > 0`（空集 ⇒ 2）。★ 这是「没少检」那一格的判据：**不是**「看着像跑过了」。
  C1 **基线自证**：基线件 `count == len(paths)` 且 `schema` 逐字 == `newcmd-followup-baseline/1`（不成立 ⇒ 2）。
  C2 **新增 path 逐面齐缺（面①–⑨）**：`新增 = 现读 − 基线`；逐条逐面打印 `齐`/`缺`/`不适用（带 reason）`；**任一 `缺` ⇒ rc=1 并点名**。
  C3 **基线陈旧（只报不改退码）**：`基线 − 现读`（冻结了但今天已不在命令树里）逐条点名 —— 只报，供人决定要不要重冻。
  C4 **计数面**（面③ 单列）：`commands` / `mustfail_cases` 两格现值 ⟷ 现读值逐格打印；不等 ⇒ rc=1（并说明这是「随动冻结值」那一格）。
  C5 **空转**：`新增` 为空且 M==N 且基线自证通过 ⇒ rc=0，但**必须**打印 `新增 0 条` 与 `现读 N 条（M 处 path 形态）` 两行 ——
      「0 条」与「没跑」在读数上可分辨（前者有 N/M 与四面扫描件数）。

用法
  python3 scripts/gates/check-newcmd-followup.py [--root <树>] [--json] [--max-examples N]
  python3 scripts/gates/check-newcmd-followup.py --emit-baseline      # 只打印基线件正文（stdout · 不写盘）
  python3 scripts/gates/check-newcmd-followup.py --self-test          # 成对负控（合成树 · 仓外夹具）

退出码：0 全绿 · 1 有缺面/计数不等 · 2 不给结论（解析漏读/基线不合规/前置缺件/空转）
"""
import argparse
import hashlib
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))

MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")
CONTRACT_BASELINE_REL = os.path.join("scripts", "gates", "cli-contract-baseline.json")
OWN_BASELINE_REL = os.path.join("scripts", "gates", "newcmd-paths-baseline.json")
DRYRUN_REG_REL = os.path.join("core", "internal", "contract", "dryrun-semantics.json")
MARKS_REL = os.path.join("scripts", "公开标记.tsv")
EXCL_A_REL = os.path.join("scripts", "build", "publish-public.sh")
EXCL_B_REL = os.path.join("publish", "mirror-public-lib.py")

# ── 面⑤–面⑨ 真源（第四批 laneB374）──────────────────────────────────────────────
PKG_REL = os.path.join("core", "cmd", "zerg")
ERRORS_REL = os.path.join("core", "cmd", "zerg", "errors.go")
EXITCODES_REL = os.path.join("core", "cmd", "zerg", "exitcodes.go")
NAMESPACES_REL = os.path.join("core", "internal", "contract", "exitcode-namespaces.json")
DOCS_RELS = (os.path.join("docs", "zh", "参考-命令行.md"),
             os.path.join("docs", "en", "参考-命令行.md"))

SCHEMA = "newcmd-followup-baseline/1"

# 口径**逐字**取自 check-cli-contract.py（单一真源：同一段主源码、同一组正则）
CMD_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
PATH_SEG_RE = re.compile(r'"([^"]+)"')
RUN_RE = re.compile(r'\brun:\s*([A-Za-z_][A-Za-z0-9_]*)')
DRYRUN_RE = re.compile(r'dry.?run|DryRun', re.I)

# 面⑤ 门73 口径（与 check-flag-registry-parity.py 同一组取数面）
FLAG_RE = re.compile(r"(--[A-Za-z0-9-]+)")
VF_SIG = "func valueFlagName(a string) string {"
VF_END = 'return ""'
CASE_RE = re.compile(r'case a == "(--[A-Za-z0-9-]+)"')
CASE_LIT_RE = re.compile(r'case "(--[A-Za-z0-9-]+)"')
PREFIX_RE = re.compile(r'strings\.HasPrefix\(a, "(--[A-Za-z0-9-]+)=')
PARSE_SIG = "func parseInvocation"
TREE_SIG = "commands = []*command{"
USAGE_RE = re.compile(r'usage:\s*(.*)')
USAGE_LIT_RE = re.compile(r'"((?:[^"\\]|\\.)*)"')
CONST_RE = re.compile(r'const (\w+) = "((?:[^"\\]|\\.)*)"')
BARE_SEP = "|｜],、"

# 面⑥ 门⑤ 口径（与 check-error-kinds.py 同一组正则）
KIND_USE_RE = re.compile(r'setErr\(\s*"([^"]+)"')
KIND_JSON_RE = re.compile(r'"kind"\s*:\s*"([^"]+)"')
KIND_ROW_RE = re.compile(r'^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,', re.M)

# 面⑦ 门⑨ 口径（与 check-exit-codes.py 同一组取数）
OSEXIT_RES = (re.compile(r'os\.Exit\((\d+)\)'),
              re.compile(r'exit[A-Za-z]+\s*=\s*(\d+)'),
              re.compile(r'exitCode\s*=\s*(\d+)'))
TABLE_ROW_RE = re.compile(r'\{\s*(\d+)\s*,\s*"')


def read(p):
    with open(p, encoding='utf-8') as f:
        return f.read()


def parse_tree(root):
    """命令树 → [(path, run_fn)]；返回 (列表, 块内 path 形态命中处数 M, 是否取到块)。"""
    fp = os.path.join(root, MAIN_REL)
    if not os.path.isfile(fp):
        return None, 0, False
    src = read(fp)
    try:
        start = src.index('commands = []*command{')
    except ValueError:
        return None, 0, False
    end = src.index('\n\t}\n', start)
    seg = src[start:end]
    matches = list(CMD_RE.finditer(seg))
    out = []
    for i, m in enumerate(matches):
        parts = PATH_SEG_RE.findall(m.group(1))
        if not parts:
            continue
        nxt = matches[i + 1].start() if i + 1 < len(matches) else len(seg)
        block = seg[m.end():nxt]
        rn = RUN_RE.search(block)
        out.append((' '.join(parts), rn.group(1) if rn else ''))
    return out, len(matches), True


def find_impl_files(root, fn):
    """`run: <fn>` ⇒ 含 `func <fn>(` 的件（仓内相对路径 · 排序）。"""
    if not fn:
        return []
    hits = []
    for base in ('core/cmd/zerg',):
        d = os.path.join(root, base)
        for name in sorted(os.listdir(d)) if os.path.isdir(d) else []:
            if not name.endswith('.go'):
                continue
            fp = os.path.join(d, name)
            try:
                src = read(fp)
            except Exception:
                continue
            if re.search(r'\bfunc\s+' + re.escape(fn) + r'\s*\(', src):
                hits.append(os.path.join(base, name).replace(os.sep, '/'))
    return hits


def load_matrix(root):
    fp = os.path.join(root, MATRIX_REL)
    if not os.path.isfile(fp):
        return None
    try:
        return json.loads(read(fp))
    except Exception:
        return None


def load_json(root, rel):
    fp = os.path.join(root, rel)
    if not os.path.isfile(fp):
        return None
    try:
        return json.loads(read(fp))
    except Exception:
        return None


def load_dryrun_registry(root):
    doc = load_json(root, DRYRUN_REG_REL)
    if doc is None:
        return None
    return {e.get('file', '') for e in doc.get('件', []) if isinstance(e, dict)}


def load_marks(root):
    """标记表 → {件: 'yes'|'no'}；表不合规/缺件 ⇒ None（rc=2）。"""
    fp = os.path.join(root, MARKS_REL)
    if not os.path.isfile(fp):
        return None
    out = {}
    for ln, line in enumerate(read(fp).splitlines(), 1):
        if not line.strip():
            continue
        cols = line.split('\t')
        if ln == 1 and cols and cols[0].strip() == '文件':
            continue
        if len(cols) < 2:
            return None
        out[cols[0].strip()] = cols[1].strip()
    return out


def load_excludes_a(root):
    """侧甲 = `scripts/build/publish-public.sh` 的 EXCLUDES（逐行取单/双引号里的路径）。"""
    fp = os.path.join(root, EXCL_A_REL)
    if not os.path.isfile(fp):
        return None
    src = read(fp)
    try:
        i = src.index('EXCLUDES=')
    except ValueError:
        return None
    tail = src[i:]
    m = re.search(r'EXCLUDES=(.*?)\n\)', tail, re.S) or re.search(r'EXCLUDES="([^"]*)"', tail, re.S)
    if not m:
        return None
    body = m.group(1)
    return [x for x in re.findall(r"[\"']([^\"']+)[\"']", body)]


def load_excludes_b(root):
    """侧乙 = `publish/mirror-public-lib.py` 的 DROP_EXACT / DROP_PREFIX。"""
    fp = os.path.join(root, EXCL_B_REL)
    if not os.path.isfile(fp):
        return None
    src = read(fp)
    exact, prefix = [], []
    for name, box in (('DROP_EXACT', exact), ('DROP_PREFIX', prefix)):
        m = re.search(name + r'\s*=\s*[\[\(](.*?)[\]\)]', src, re.S)
        if m:
            box.extend(re.findall(r"[\"']([^\"']+)[\"']", m.group(1)))
    return exact, prefix


def excluded(path, exact, prefix):
    p = path.strip('/')
    if p in [e.strip('/') for e in exact]:
        return True
    return any(p.startswith(x.strip('/')) for x in prefix if x.strip('/'))


def cmd_paths_only(tree):
    return [p for p, _ in tree]


# ── 面⑤–面⑨ 取数（第四批 laneB374）─────────────────────────────────────────────
def strip_comments(src):
    """剥 `//` 注释（与门73 同口径：注释里写着旗标名 ⇒ 不剥会读成登记）。"""
    return "\n".join(line.split("//", 1)[0] for line in src.splitlines())


def value_form(usage, pos):
    """旗标后面那一位是不是**值**（门73 的 `value_form` 逐字口径）。"""
    if pos >= len(usage):
        return False
    c = usage[pos]
    if c in "=:":
        return True
    t = usage[pos:].lstrip()
    if not t:
        return False
    if t[0] in BARE_SEP:
        return False
    if t.startswith("--"):
        return False
    return True


def parse_usages(root):
    """命令树 → {空格连接的 path: (usage 串, 是否透传)}；取不到 ⇒ None。"""
    fp = os.path.join(root, MAIN_REL)
    if not os.path.isfile(fp):
        return None
    try:
        src = read(fp)
    except Exception:
        return None
    if TREE_SIG not in src:
        return None
    consts = {}
    pkg = os.path.join(root, PKG_REL)
    for name in sorted(os.listdir(pkg)) if os.path.isdir(pkg) else []:
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        try:
            body = strip_comments(read(os.path.join(pkg, name)))
        except Exception:
            continue
        for m in CONST_RE.finditer(body):
            consts.setdefault(m.group(1), m.group(2))
    t0 = src.index(TREE_SIG)
    t1 = src.index("\n\t}\n", t0)
    out = {}
    for block in re.split(r"\n\t\t\{", src[t0:t1]):
        mp = CMD_RE.search(block)
        if not mp:
            continue
        path = " ".join(PATH_SEG_RE.findall(mp.group(1)))
        mu = USAGE_RE.search(block)
        if not mu:
            continue
        raw = mu.group(1).strip().rstrip(",").strip()
        usage = ""
        if raw.startswith('"'):
            lit = USAGE_LIT_RE.match(raw)
            usage = lit.group(1) if lit else ""
        else:
            key = raw.split()[0].strip(",") if raw.split() else ""
            usage = consts.get(key, "")
        out[path] = (usage, "passthrough: true" in block)
    return out


def build_flag_faces(root):
    """门73 口径取数面 → (recognized 集合, usages 映射)；取不到 ⇒ None。"""
    fp = os.path.join(root, MAIN_REL)
    if not os.path.isfile(fp):
        return None
    try:
        main_src = strip_comments(read(fp))
    except Exception:
        return None
    if VF_SIG not in main_src or VF_END not in main_src or PARSE_SIG not in main_src:
        return None
    i = main_src.index(VF_SIG)
    j = main_src.index(VF_END, i)
    V = set(FLAG_RE.findall(main_src[i:j]))
    p = main_src.index(PARSE_SIG)
    seg = main_src[p:i if i > p else len(main_src)]
    D = set(CASE_RE.findall(seg)) | set(CASE_LIT_RE.findall(seg)) | set(PREFIX_RE.findall(seg))
    usages = parse_usages(root)
    if usages is None or not V:
        return None
    return (V | D), usages


def load_kinds(root):
    """门⑤ 口径：`errorKinds` 表里 kind 名集合；表空/缺件 ⇒ None。"""
    fp = os.path.join(root, ERRORS_REL)
    if not os.path.isfile(fp):
        return None
    try:
        src = read(fp)
    except Exception:
        return None
    start = src.find("var errorKinds = []errorKindRow{")
    if start < 0:
        return None
    end = src.find("\n}", start)
    if end < 0:
        return None
    return {m.group(1) for m in KIND_ROW_RE.finditer(src[start:end])} or None


def load_exitcodes(root):
    """门⑨ 口径：`exitcodeTable` ∪ `exitcodeReserved` 里的号集合；表空/缺件 ⇒ None。"""
    fp = os.path.join(root, EXITCODES_REL)
    if not os.path.isfile(fp):
        return None
    try:
        src = read(fp)
    except Exception:
        return None
    codes = set()
    for var in ("exitcodeTable", "exitcodeReserved"):
        m = re.search(r'var\s+' + var + r'\s*=\s*\[\]exitcodeRow\{(.*?)\n\}', src, re.S)
        if m:
            codes |= {int(x) for x in TABLE_ROW_RE.findall(m.group(1))}
    return codes or None


def load_ns_codes(root):
    """门⑨ 登记表：`exitcode-namespaces.json` 里 `cli` 命名空间的号集合；缺/不可解析 ⇒ None。"""
    doc = load_json(root, NAMESPACES_REL)
    if doc is None:
        return None
    for ns in doc.get("namespaces", []):
        if isinstance(ns, dict) and ns.get("id") == "cli":
            return set(int(x) for x in ns.get("table", []))
    return None


def load_docs(root):
    """面⑨ 只报用：docs/zh|en/参考-命令行.md 的正文；缺件 ⇒ 该项 None。"""
    out = []
    for rel in DOCS_RELS:
        fp = os.path.join(root, rel)
        out.append(read(fp) if os.path.isfile(fp) else None)
    return out


def impl_literals(root, impls, regexes):
    """从实现件里取字面量集合（面⑥/面⑦ 共用）。"""
    out = set()
    for f in impls:
        try:
            src = read(os.path.join(root, f))
        except Exception:
            continue
        for rx in regexes:
            for m in rx.findall(src):
                out.add(m)
    return out


def analyse(root, max_examples=20):
    """→ (rc, lines)。rc: 0/1/2。"""
    L = []
    add = L.append
    tree, m_forms, got = parse_tree(root)
    if not got:
        return 2, ["[不给结论] 取不到命令树块 `commands = []*command{`（前置缺件）⇒ rc=2（不许当绿）"]
    if tree is None:
        return 2, ["[不给结论] 命令树块解析失败 ⇒ rc=2"]
    n_paths = len(tree)
    paths = cmd_paths_only(tree)
    if n_paths == 0:
        return 2, ["[不给结论] 现读 path 集为空 ⇒ rc=2（空转不许当绿）"]
    if m_forms != n_paths:
        return 2, ["[不给结论] 解析漏读：块内 `path: []string{` 命中 %d 处 ⟷ 解析出 %d 条 ⇒ rc=2"
                   "（漏读的集合不许当基线）" % (m_forms, n_paths)]

    base = load_json(root, OWN_BASELINE_REL)
    if base is None:
        return 2, ["[不给结论] 基线件缺失/不可解析：%s ⇒ rc=2" % OWN_BASELINE_REL]
    if base.get('schema') != SCHEMA:
        return 2, ["[不给结论] 基线件 schema 逐字不是 `%s`（现读 %r）⇒ rc=2"
                   % (SCHEMA, base.get('schema'))]
    bpaths = list(base.get('paths', []))
    if base.get('count') != len(bpaths):
        return 2, ["[不给结论] 基线自证不过：count=%r ⟷ len(paths)=%d ⇒ rc=2"
                   % (base.get('count'), len(bpaths))]

    cur, old = set(paths), set(bpaths)
    new = sorted(cur - old)
    stale = sorted(old - cur)

    add("现读 path 条数：%d（块内 `path: []string{` 形态命中 %d 处 · 差 %d）"
        % (n_paths, m_forms, m_forms - n_paths))
    add("冻结基线：%d 条（head=%s）" % (len(bpaths), base.get('head', '?')))
    add("新增 path：%d 条" % len(new))
    add("基线陈旧（只报 · 不改退码）：%d 条" % len(stale))
    for s in stale[:max_examples]:
        add("  · 陈旧 %s" % s)

    matrix = load_matrix(root)
    cbase = load_json(root, CONTRACT_BASELINE_REL)
    dryreg = load_dryrun_registry(root)
    marks = load_marks(root)
    exa = load_excludes_a(root)
    exb = load_excludes_b(root)
    kinds = load_kinds(root)          # 面⑥ 真源（门⑤ 口径）
    excodes = load_exitcodes(root)    # 面⑦ 真源（门⑨ 口径）
    if (matrix is None or cbase is None or dryreg is None or marks is None
            or exa is None or exb is None or kinds is None or excodes is None):
        missing = []
        if matrix is None:
            missing.append(MATRIX_REL)
        if cbase is None:
            missing.append(CONTRACT_BASELINE_REL)
        if dryreg is None:
            missing.append(DRYRUN_REG_REL)
        if marks is None:
            missing.append(MARKS_REL)
        if exa is None:
            missing.append(EXCL_A_REL)
        if exb is None:
            missing.append(EXCL_B_REL)
        if kinds is None:
            missing.append(ERRORS_REL + "（errorKinds 表空/缺件）")
        if excodes is None:
            missing.append(EXITCODES_REL + "（退码表空/缺件）")
        return 2, L + ["[不给结论] 前置缺件/不可解析：%s ⇒ rc=2" % '、'.join(missing)]

    cases = matrix.get('cases', [])
    add("扫描面件数：cli-matrix cases=%d · dryrun 登记件=%d · 标记表行=%d · 侧甲排除=%d 条 · 侧乙 exact=%d/prefix=%d"
        % (len(cases), len(dryreg), len(marks), len(exa), len(exb[0]), len(exb[1])))

    # ── 面⑤ 门73 取数面 + 面⑦ 命名空间 + 面⑨ docs（只报）预先取一次 ────────────────
    flagged = build_flag_faces(root) if new else None
    if new and flagged is None:
        return 2, L + ["[不给结论] 面⑤ 取数面（门73 口径 · `valueFlagName`/`parseInvocation`）拿不到 ⇒ rc=2"]
    recognized, usages = (flagged if flagged else (set(), {}))
    ns_codes = load_ns_codes(root)        # None ⇒ 面⑦ 命名空间那半格不适用
    docs_texts = load_docs(root)          # 逐项 None ⇒ 面⑨ 该语种不适用
    add("扩面取数件数（%s）：门73 认旗标=%s · error.kind 闭集=%d · 退码表=%d · 命名空间 cli=%s · docs 语种在盘=%d"
        % ('新增>0 ⇒ 现读取数面' if new else '新增 0 条 ⇒ 面⑤ 取数面未跑（不许把 0 当空转）',
           len(recognized) if new else '未取',
           len(kinds), len(excodes),
           ('%d 号' % len(ns_codes)) if ns_codes is not None else '未取到',
           sum(1 for t in docs_texts if t is not None)))

    bad = 0
    for p in new:
        leaf_fn = dict(tree)[p]
        impls = find_impl_files(root, leaf_fn)
        add("── 新增 path：`zerg %s`（run=%s · 实现件 %s）" % (p, leaf_fn or '?', '、'.join(impls) or '未定位'))

        # 面① dryrun 落点
        f1_ok, f1_note = True, []
        if not impls:
            f1_ok = False
            f1_note.append('实现件未定位 ⇒ 缺（判不了）')
        else:
            for f in impls:
                fp = os.path.join(root, f)
                try:
                    has = bool(DRYRUN_RE.search(read(fp)))
                except Exception:
                    has = False
                if has and f not in dryreg:
                    f1_ok = False
                    f1_note.append('%s 含 dry-run 语义但未登记 ⇒ 缺' % f)
                elif has:
                    f1_note.append('%s 已登记 ⇒ 齐' % f)
                else:
                    f1_note.append('%s 无 dry-run 语义 ⇒ 不适用（无落点）' % f)
        add("   面① dryrun 落点：%s —— %s" % ('齐' if f1_ok else '缺', ' · '.join(f1_note)))
        bad += 0 if f1_ok else 1

        # 面② cli-matrix must-fail
        mine = [c for c in cases if c.get('command') == p]
        f2_ok = bool(mine) and all(int(c.get('want_rc', 1)) != 0 for c in mine)
        add("   面② cli-matrix must-fail：%s —— 命中 %d 格（want_rc=%s）%s"
            % ('齐' if f2_ok else '缺', len(mine),
               ','.join(str(c.get('want_rc')) for c in mine[:max_examples]) or '-',
               '' if f2_ok else '（需 ≥1 格 且 全非 0）'))
        bad += 0 if f2_ok else 1

        # 面④ 标记表 + 两器
        f4_ok, f4_note = True, []
        if not impls:
            f4_ok = False
            f4_note.append('实现件未定位 ⇒ 缺')
        for f in impls:
            pub = marks.get(f)
            if pub is None:
                f4_ok = False
                f4_note.append('%s 不在标记表 ⇒ 缺' % f)
                continue
            if pub == 'no':
                a = excluded(f, exa, [])
                b = excluded(f, exb[0], exb[1])
                if not a:
                    f4_ok = False
                    f4_note.append('%s public=no 但缺侧甲 ⇒ 两器都发（缺）' % f)
                else:
                    f4_note.append('%s public=no · 侧甲已排除（侧乙=%s）⇒ 齐'
                                   % (f, '已排除' if b else '未排除'))
            else:
                f4_note.append('%s public=%s ⇒ 齐' % (f, pub))
        add("   面④ 标记表+两器排除：%s —— %s" % ('齐' if f4_ok else '缺', ' · '.join(f4_note)))
        bad += 0 if f4_ok else 1

        # 面⑤ 门73 旗标名表（新增 path 用法串里的取值形态旗标必须被解析面认）
        f5_ok, f5_note = True, []
        usage, passthru = usages.get(p, (None, False))
        if passthru:
            f5_note.append('透传型命令 ⇒ 不适用（旗标逐字交被包脚本 · 门73 同口径）')
        elif usage is None:
            f5_ok = False
            f5_note.append('用法串未定位 ⇒ 缺（判不了）')
        else:
            recognized_hits, ghosts = [], []
            for m in FLAG_RE.finditer(usage):
                if value_form(usage, m.end()):
                    (recognized_hits if m.group(1) in recognized else ghosts).append(m.group(1))
            if ghosts:
                f5_ok = False
                for g in dict.fromkeys(ghosts):
                    f5_note.append('%s 取值形态但解析面不认 ⇒ 缺（真给退 2「未知旗标」= 命令不可用）' % g)
            if not recognized_hits and not ghosts:
                f5_note.append('用法串无取值形态旗标 ⇒ 不适用（无落点）')
            elif f5_ok:
                f5_note.append('%s 均在 valueFlagName∪parseInvocation ⇒ 齐'
                               % '、'.join(dict.fromkeys(recognized_hits)))
        add("   面⑤ 门73 旗标名表：%s —— %s" % ('齐' if f5_ok else '缺', ' · '.join(f5_note)))
        bad += 0 if f5_ok else 1

        # 面⑥ 门⑤ error.kind 闭集（实现件里的 kind 字面量必须在 errorKinds 表内）
        f6_ok, f6_note = True, []
        kind_lits = sorted(k for k in impl_literals(root, impls, (KIND_USE_RE, KIND_JSON_RE)) if k != 'kind')
        if not kind_lits:
            f6_note.append('实现件无 kind 字面量 ⇒ 不适用（无落点）')
        else:
            ghost_kinds = [k for k in kind_lits if k not in kinds]
            if ghost_kinds:
                f6_ok = False
                for k in ghost_kinds:
                    f6_note.append('%r 不在 errorKinds 闭集 ⇒ 缺（先登记再用 · 门⑤ R3 口径）' % k)
            else:
                f6_note.append('%s 均在闭集 ⇒ 齐' % '、'.join(kind_lits))
        add("   面⑥ 门⑤ error.kind 闭集：%s —— %s" % ('齐' if f6_ok else '缺', ' · '.join(f6_note)))
        bad += 0 if f6_ok else 1

        # 面⑦ 门⑨ 退码表 + 命名空间（实现件里用到的号必须登记）
        f7_ok, f7_note = True, []
        code_lits = sorted({int(x) for rx in OSEXIT_RES for x in impl_literals(root, impls, (rx,))})
        if not code_lits:
            f7_note.append('实现件无退码字面量 ⇒ 不适用（无落点）')
        else:
            off_table = [c for c in code_lits if c not in excodes]
            off_ns = [c for c in code_lits if ns_codes is not None and c not in ns_codes]
            if off_table:
                f7_ok = False
                for c in off_table:
                    f7_note.append('%d 不在 exitcodeTable∪exitcodeReserved ⇒ 缺（先登记号再启用 · 门⑨）' % c)
            if off_ns:
                f7_ok = False
                for c in off_ns:
                    f7_note.append('%d 未登记进 exitcode-namespaces.json 的 cli 命名空间 ⇒ 缺（门⑨ R5）' % c)
            if f7_ok:
                f7_note.append('%s 均在退码表%s ⇒ 齐'
                               % ('、'.join(str(c) for c in code_lits),
                                  '且被 cli 命名空间覆盖' if ns_codes is not None else '（命名空间表未取到 ⇒ 该半格不适用）'))
        add("   面⑦ 门⑨ 退码表+命名空间：%s —— %s" % ('齐' if f7_ok else '缺', ' · '.join(f7_note)))
        bad += 0 if f7_ok else 1

        # 面⑧ 门⑪ T5d 归属面（只报 · 不改退码；T3/T6 是全局唯一性判据，按 path 无归属）
        no_why = [c.get('id', '?') for c in mine if not str(c.get('why', '')).strip()]
        if not mine:
            f8_note = '无 must-fail 格 ⇒ 不适用（面② 已判）'
        elif no_why:
            f8_note = ('只报：%d 格缺 why ⇒ T5d（why⟷值）判不了（T3 已由面⑦ · T6 registry 引用面属全局 · 不改退码）：%s'
                       % (len(no_why), '、'.join(no_why[:max_examples])))
        else:
            f8_note = '每格有 why ⇒ T5d 可判（T3 已由面⑦ · T6 属全局）'
        add("   面⑧ 门⑪ T5d 归属面（只报）：—— %s" % f8_note)

        # 面⑨ docs 生成物（只报 · drift 已由 docs scope 兜）
        f9_notes = []
        for rel, txt in zip(DOCS_RELS, docs_texts):
            if txt is None:
                f9_notes.append('%s 不在盘 ⇒ 不适用' % rel)
            elif p not in txt:
                f9_notes.append('只报：%s 生成区未见 `%s`（跑 gen-cli-reference.py --check）' % (rel, p))
            else:
                f9_notes.append('%s 有格' % rel)
        add("   面⑨ docs 生成物（只报）：—— %s" % ' · '.join(f9_notes))

    # 面③ 计数（全局一格）
    n_cases = len(cases)
    bc, bm = cbase.get('commands'), cbase.get('mustfail_cases')
    c_ok = (bc == n_paths) and (bm == n_cases)
    add("── 面③ cli-contract-baseline 计数：%s" % ('齐' if c_ok else '缺'))
    add("   commands 基线=%r ⟷ 现读=%d %s" % (bc, n_paths, '✓' if bc == n_paths else '✗'))
    add("   mustfail_cases 基线=%r ⟷ 现读=%d %s" % (bm, n_cases, '✓' if bm == n_cases else '✗'))
    if not c_ok:
        bad += 1

    if new == []:
        add("新增 0 条 ⇒ 九面无需随动（本读数可分辨「没跑」：上有现读 %d 条 / %d 处 path 形态与两面扫描件数行）"
            % (n_paths, m_forms))

    if bad:
        add("⇒ rc=1：以上 %d 处「缺」逐条点名（缺任一面即红）" % bad)
        return 1, L
    add("⇒ rc=0：新增 path 逐面齐（面①–⑨）/ 计数齐 / 解析自证齐（%d 条 ⟷ %d 处）" % (n_paths, m_forms))
    return 0, L


def emit_baseline(root):
    tree, m_forms, got = parse_tree(root)
    if not got or tree is None:
        print("取不到命令树块 ⇒ 不许出基线", file=sys.stderr)
        return 2
    paths = sorted(cmd_paths_only(tree))
    doc = {
        "schema": SCHEMA,
        "head": None,
        "count": len(paths),
        "_口径": "冻结的 path 全集（= `zerg gate`-facing 命令树逐条 path 以空格连接）· 单一真源 = core/cmd/zerg/main.go 的 commands 块（与 check-cli-contract.py 同一组正则）。"
                "新增命令 ⇒ 本件必须**同批重冻**（否则 check-newcmd-followup.py 会把新 path 当「新增」逐面点名）。",
        "paths": paths,
    }
    print(json.dumps(doc, ensure_ascii=False, indent=1))
    return 0


# ─────────────────────────── 自检（成对负控 · 仓外夹具）───────────────────────────
def _mini_root(dst, n_extra_cmd=False, extra_has_matrix=True, count_drift=False,
               extra_usage='zerg nu', extra_flags=None):
    """搭一棵最小树：基线内命令 + （可选）1 条新增命令。

    extra_usage: 新增命令的用法串（面⑤ 用）；extra_flags: 追加进 valueFlagName 的旗标名。
    """
    os.makedirs(os.path.join(dst, 'core', 'cmd', 'zerg', 'testdata'), exist_ok=True)
    os.makedirs(os.path.join(dst, 'scripts', 'gates'), exist_ok=True)
    os.makedirs(os.path.join(dst, 'scripts', 'build'), exist_ok=True)
    os.makedirs(os.path.join(dst, 'publish'), exist_ok=True)
    os.makedirs(os.path.join(dst, 'core', 'internal', 'contract'), exist_ok=True)
    cmds = [('alpha', 'cmdAlpha', 'zerg alpha'), ('core', 'cmdCore', 'zerg core')]
    if n_extra_cmd:
        cmds.append(('nu', 'cmdNu', extra_usage))
    blocks = []
    for p, fn, us in cmds:
        blocks.append('\t\t{\n\t\t\tpath: %s,\n\t\t\trun: %s,\n\t\t\tusage: "%s",\n\t\t},\n'
                      % ('[]string{"%s"}' % p, fn, us))
    vf_names = ['"--base"'] + ['"%s"' % f for f in (extra_flags or [])]
    main = ('package main\n\nfunc main() {\n\tcommands = []*command{\n' + ''.join(blocks)
            + '\t}\n}\n\n'
            + 'func valueFlagName(a string) string {\n\tswitch a {\n\tcase '
            + ', '.join(vf_names) + ':\n\t\treturn a\n\t}\n\treturn ""\n}\n\n'
            + 'func parseInvocation() {}\n')
    open(os.path.join(dst, MAIN_REL), 'w', encoding='utf-8').write(main)
    for _, fn, _ in cmds:
        open(os.path.join(dst, 'core', 'cmd', 'zerg', fn + '.go'), 'w', encoding='utf-8').write(
            'package main\n\nfunc %s() {}\n' % fn)
    # 面⑥/面⑦ 真源（门⑤ / 门⑨ 口径的最小片）
    open(os.path.join(dst, ERRORS_REL), 'w', encoding='utf-8').write(
        'package main\n\nvar errorKinds = []errorKindRow{\n'
        '\t{"conflict", 14, false, "r", "m", "s"},\n'
        '\t{"usage", 2, false, "r", "m", "s"},\n}\n')
    open(os.path.join(dst, EXITCODES_REL), 'w', encoding='utf-8').write(
        'package main\n\nvar exitcodeTable = []exitcodeRow{\n'
        '\t{0, "ok", "成功", "—"},\n\t{1, "failed", "一般失败", "看 kind"},\n'
        '\t{2, "usage", "用法错", "改用法"},\n\t{14, "conflict", "冲突", "排队"},\n}\n'
        'var exitcodeReserved = []exitcodeRow{}\n')
    json.dump({'schema': 'zerg/exitcodes-namespaces/v1',
               'namespaces': [{'id': 'cli', 'table': [0, 1, 2, 14]}]},
              open(os.path.join(dst, NAMESPACES_REL), 'w', encoding='utf-8'))
    cases = []
    for p, _ in [(c[0], c[1]) for c in cmds]:
        if p == 'alpha' and n_extra_cmd:
            cases.append({'id': p + '#x', 'command': p, 'argv': [],
                          'want_rc': 2, 'want_stdout_bytes': 0, 'why': '夹具'})
        elif p != (cmds[-1][0]) or extra_has_matrix:
            cases.append({'id': p + '#x', 'command': p, 'argv': [],
                          'want_rc': 2, 'want_stdout_bytes': 0, 'why': '夹具'})
    json.dump({'id': 'cli-contract.v1', 'cases': cases},
              open(os.path.join(dst, MATRIX_REL), 'w', encoding='utf-8'))
    bc = len(cmds) - (1 if count_drift else 0)
    json.dump({'id': 'cli-contract-baseline.v1', 'blanks': 0, 'mustfail_cases': len(cases), 'commands': bc},
              open(os.path.join(dst, CONTRACT_BASELINE_REL), 'w', encoding='utf-8'))
    json.dump({'schema': 'zerg/dryrun-semantics/v1', '件': []},
              open(os.path.join(dst, DRYRUN_REG_REL), 'w', encoding='utf-8'))
    marks = '文件\tpublic\t理由\n'
    for p, fn, _ in cmds:
        marks += 'core/cmd/zerg/%s.go\tyes\t测试夹具\n' % fn
    open(os.path.join(dst, MARKS_REL), 'w', encoding='utf-8').write(marks)
    open(os.path.join(dst, EXCL_A_REL), 'w', encoding='utf-8').write('EXCLUDES=(\n ""\n)\n')
    open(os.path.join(dst, EXCL_B_REL), 'w', encoding='utf-8').write(
        'DROP_EXACT = []\nDROP_PREFIX = []\n')
    bpaths = sorted([p for p, _, _ in cmds][:2]) if n_extra_cmd else sorted([p for p, _, _ in cmds])
    json.dump({'schema': SCHEMA, 'head': 'fixture', 'count': len(bpaths), 'paths': bpaths},
              open(os.path.join(dst, OWN_BASELINE_REL), 'w', encoding='utf-8'))


def self_test():
    fails = []
    tmp = tempfile.mkdtemp(prefix='newcmd-followup-selftest-')
    try:
        # ① 正控：无新增 · 四面无随动需求 · 计数齐 ⇒ rc=0
        a = os.path.join(tmp, 'a')
        _mini_root(a)
        rc, _ = analyse(a)
        if rc != 0:
            fails.append('① 正控（无新增）期望 rc=0，实得 %d' % rc)
        # ② 负控：新增 path 缺「矩阵 must-fail」+ 计数不齐 ⇒ rc=1
        b = os.path.join(tmp, 'b')
        _mini_root(b, n_extra_cmd=True, extra_has_matrix=False)
        rc, lines = analyse(b)
        if rc != 1:
            fails.append('② 负控（新增缺面②）期望 rc=1，实得 %d' % rc)
        elif not any('面② cli-matrix must-fail：缺' in x for x in lines):
            fails.append('② 负控未点名面②')
        # ③ 负控：新增 path 四面齐（含计数随动）⇒ rc=0（成对半边）
        c = os.path.join(tmp, 'c')
        _mini_root(c, n_extra_cmd=True, extra_has_matrix=True)
        rc, _ = analyse(c)
        if rc != 0:
            fails.append('③ 负控半边（四面齐）期望 rc=0，实得 %d' % rc)
        # ④ 负控：计数漂移 ⇒ rc=1
        d = os.path.join(tmp, 'd')
        _mini_root(d, n_extra_cmd=False, count_drift=True)
        rc, lines = analyse(d)
        if rc != 1:
            fails.append('④ 计数漂移期望 rc=1，实得 %d' % rc)
        # ⑤ 负控：解析漏读（块里两处 path 形态、其中一处空数组）⇒ rc=2
        e = os.path.join(tmp, 'e')
        _mini_root(e)
        src = open(os.path.join(e, MAIN_REL), encoding='utf-8').read()
        src = src.replace('\t}\n}\n', '\t\t{\n\t\t\tpath: []string{},\n\t\t\trun: cmdAlpha,\n\t\t},\n\t}\n}\n')
        open(os.path.join(e, MAIN_REL), 'w', encoding='utf-8').write(src)
        rc, lines = analyse(e)
        if rc != 2:
            fails.append('⑤ 解析漏读期望 rc=2，实得 %d' % rc)
        # ⑥ 负控：缺基线件 ⇒ rc=2
        f = os.path.join(tmp, 'f')
        _mini_root(f)
        os.remove(os.path.join(f, OWN_BASELINE_REL))
        rc, _ = analyse(f)
        if rc != 2:
            fails.append('⑥ 缺基线件期望 rc=2，实得 %d' % rc)
        # ⑦ 负控：dryrun 落点缺（实现件含 dry-run 但未登记）⇒ rc=1 且点名面①
        g = os.path.join(tmp, 'g')
        _mini_root(g, n_extra_cmd=True, extra_has_matrix=True)
        open(os.path.join(g, 'core', 'cmd', 'zerg', 'cmdNu.go'), 'a', encoding='utf-8').write(
            '\n// dry-run 站点\nvar dryRunFlag = true\n')
        rc, lines = analyse(g)
        if rc != 1:
            fails.append('⑦ dryrun 未登记期望 rc=1，实得 %d' % rc)
        elif not any('面① dryrun 落点：缺' in x for x in lines):
            fails.append('⑦ 未点名面①')
        # ⑧ 正控半边：同一树把实现件登记进 dryrun 表 ⇒ rc=0
        h = os.path.join(tmp, 'h')
        _mini_root(h, n_extra_cmd=True, extra_has_matrix=True)
        open(os.path.join(h, 'core', 'cmd', 'zerg', 'cmdNu.go'), 'a', encoding='utf-8').write(
            '\nvar dryRunFlag = true\n')
        json.dump({'schema': 'zerg/dryrun-semantics/v1',
                   '件': [{'file': 'core/cmd/zerg/cmdNu.go', '语义': 'dryrun.v1', '零副作用判据': 'fixture'}]},
                  open(os.path.join(h, DRYRUN_REG_REL), 'w', encoding='utf-8'))
        rc, _ = analyse(h)
        if rc != 0:
            fails.append('⑧ dryrun 已登记期望 rc=0，实得 %d' % rc)
        # ⑨ 负控：public=no 且缺侧甲 ⇒ rc=1 且点名「两器都发」
        i = os.path.join(tmp, 'i')
        _mini_root(i, n_extra_cmd=True, extra_has_matrix=True)
        marks = open(os.path.join(i, MARKS_REL), encoding='utf-8').read().replace(
            'core/cmd/zerg/cmdNu.go\tyes\t', 'core/cmd/zerg/cmdNu.go\tno\t')
        open(os.path.join(i, MARKS_REL), 'w', encoding='utf-8').write(marks)
        rc, lines = analyse(i)
        if rc != 1:
            fails.append('⑨ public=no 缺侧甲期望 rc=1，实得 %d' % rc)
        elif not any('两器都发' in x for x in lines):
            fails.append('⑨ 未点名「两器都发」')
        # ⑩ 负控：件不在标记表 ⇒ rc=1
        j = os.path.join(tmp, 'j')
        _mini_root(j, n_extra_cmd=True, extra_has_matrix=True)
        marks = open(os.path.join(j, MARKS_REL), encoding='utf-8').read().replace(
            'core/cmd/zerg/cmdNu.go\tyes\t测试夹具\n', '')
        open(os.path.join(j, MARKS_REL), 'w', encoding='utf-8').write(marks)
        rc, lines = analyse(j)
        if rc != 1:
            fails.append('⑩ 件不在标记表期望 rc=1，实得 %d' % rc)
        # ⑪ 负控：新增 path 用法串里取值旗标 ∉ 解析面 ⇒ rc=1 且点名面⑤
        k = os.path.join(tmp, 'k')
        _mini_root(k, n_extra_cmd=True, extra_has_matrix=True, extra_usage='zerg nu --ghost <值>')
        rc, lines = analyse(k)
        if rc != 1:
            fails.append('⑪ 门73 旗标缺期望 rc=1，实得 %d' % rc)
        elif not any('面⑤ 门73 旗标名表：缺' in x for x in lines):
            fails.append('⑪ 未点名面⑤')
        # ⑫ 正控半边：同一用法串 + 旗标登记进 valueFlagName ⇒ rc=0
        l = os.path.join(tmp, 'l')
        _mini_root(l, n_extra_cmd=True, extra_has_matrix=True,
                   extra_usage='zerg nu --ghost <值>', extra_flags=['--ghost'])
        rc, _ = analyse(l)
        if rc != 0:
            fails.append('⑫ 门73 旗标齐期望 rc=0，实得 %d' % rc)
        # ⑬ 负控：新增实现件含闭集外 kind ⇒ rc=1 且点名面⑥
        m = os.path.join(tmp, 'm')
        _mini_root(m, n_extra_cmd=True, extra_has_matrix=True)
        open(os.path.join(m, 'core', 'cmd', 'zerg', 'cmdNu.go'), 'a', encoding='utf-8').write(
            '\nfunc x(){ setErr("ghost_kind", 1, "x") }\n')
        rc, lines = analyse(m)
        if rc != 1:
            fails.append('⑬ 门⑤ kind 缺期望 rc=1，实得 %d' % rc)
        elif not any('面⑥ 门⑤ error.kind 闭集：缺' in x for x in lines):
            fails.append('⑬ 未点名面⑥')
        # ⑭ 负控：新增实现件用未登记退码 ⇒ rc=1 且点名面⑦
        n = os.path.join(tmp, 'n')
        _mini_root(n, n_extra_cmd=True, extra_has_matrix=True)
        open(os.path.join(n, 'core', 'cmd', 'zerg', 'cmdNu.go'), 'a', encoding='utf-8').write(
            '\nfunc y(){ os.Exit(99) }\n')
        rc, lines = analyse(n)
        if rc != 1:
            fails.append('⑭ 门⑨ 退码缺期望 rc=1，实得 %d' % rc)
        elif not any('面⑦ 门⑨ 退码表+命名空间：缺' in x for x in lines):
            fails.append('⑭ 未点名面⑦')
        # ⑮ 正控半边：同一处用已登记号 14 ⇒ rc=0
        o = os.path.join(tmp, 'o')
        _mini_root(o, n_extra_cmd=True, extra_has_matrix=True)
        open(os.path.join(o, 'core', 'cmd', 'zerg', 'cmdNu.go'), 'a', encoding='utf-8').write(
            '\nfunc y(){ os.Exit(14) }\n')
        rc, _ = analyse(o)
        if rc != 0:
            fails.append('⑮ 门⑨ 退码齐期望 rc=0，实得 %d' % rc)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    if fails:
        print('自检不过：')
        for x in fails:
            print('  ✗ ' + x)
        return 1
    print('自检通过：15 控（正控 4 + 负控 11）逐条对拍 ✓')
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--root', default=DEFAULT_ROOT)
    ap.add_argument('--json', action='store_true')
    ap.add_argument('--max-examples', type=int, default=20)
    ap.add_argument('--emit-baseline', action='store_true')
    ap.add_argument('--self-test', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    if ns.emit_baseline:
        return emit_baseline(ns.root)
    rc, lines = analyse(ns.root, ns.max_examples)
    if ns.json:
        print(json.dumps({'schema': 'zerg/v1', 'kind': 'NewCmdFollowup', 'rc': rc,
                          'lines': lines}, ensure_ascii=False, indent=1))
    else:
        for x in lines:
            print(x)
    return rc


if __name__ == '__main__':
    sys.exit(main())

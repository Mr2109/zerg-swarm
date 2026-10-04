#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-affected-map.py —— 甲①「受影响面选择器」第 0 刀 · 声明表机检件（双向差集 · fail-closed）

本件管住的缝 ＝ 第二节第一道机器「反证差集」：声明表漏了一条（声明覆盖不到的输入）。
口径照 `go mod tidy` / `pre-commit check-hooks-apply`：
  ① 「盘上有件但表里没声」＝ 声明覆盖不到的输入（**设计允许的常态** ⇒ 报告，不当红；
     按已拍「未声明件默认每轮都跑」处置）；
  ② 「表里有件盘上无」＝ 悬空声明（声明指向不存在的件 ⇒ **红** ・ fail-closed）；
  ③ ★ **步面格（拍乙 · D9 步面接线）**：声明表 `faces` 逐字命名的步集合 ⟷ 步骤表（`add_step` 现读）
     步集合，「`D==N` ＋ 命名集合逐字相等」缺一即红 —— 治「门84 只判件面、判不出步面漂移」那个缝
     （见设计稿 §七-(b) / §六负控①）。★ 本格**复甲① `affected-steps.py` 的 C0 唯一实现**
     （`c0_step_face()` · 由 importlib 现载邻件取用），✗ **不另写第二份「逐字相等」**（违唯一真源即红）。

声明表（本单新件 `scripts/gates/affected-map.json`）**由本件 `--emit` 现读生成**，
来源三处（逐条现读 · ✗ 不手敲）：
  A. `scripts/gates/precommit-gates.sh`：`:312 DEFAULT_SCOPES` ＋ `:1966 build_steps` 内逐行 `add_step`
     ⇒ 每一「步」的 scope / 名 / 模式 / 工作目录 / 命令（＝步表真源）；
  B. `scripts/gates/gate-coverage.config.json`：`scan.script_globs` ＋ `exclude` ＋ `whitelist.scripts[].path`
     ⇒ 脚本候选集与硬排除面（件→面判据）；
  C. `scripts/gates/newcmd-paths-baseline.json`：命令树 path 全集 ⇒ 命令面真源件（`core/cmd/zerg/main.go` ·
     `core/cmd/zerg/testdata/cli-matrix.json` · `scripts/gates/cli-contract-baseline.json`）与读它的门步。

退码（三值 ＋ **步面单列一格** · fail-closed，照派单 ＋ 拍乙）：
  0  读回成功（① ② 两类差集皆空 · ③ 步面 `D==N` 且逐字相等）
  1  缺读数 / 悬空声明（声明件不在 · git 读数取不到 · `表里有件盘上无` 非空 · 空表无鉴别力）
  2  判不了（用法错 / `--self-test` 不过 / 步骤表或 C0 唯一真源取不到）
  3  **步面漂移**（拍乙 · 单列一格）：声明表 `faces` 命名步 ≠ 步骤表（`D≠N` 或命名集合不等）
     ★ 与「悬空声明」的 rc=1 **分列两格**（✗ 不混同一 rc）；`tri` 档下 3 与 1 同判 FAIL（阻断）。

子命令
  python3 check-affected-map.py --emit [--out <件>]     现读生成声明表（默认写 scripts/gates/affected-map.json）
  python3 check-affected-map.py --check [--map <件>]    双向差集 ＋ 步面逐字相等机检
  python3 check-affected-map.py --self-test             成对负控（rc 0 通过 / 2 不过）
"""
import argparse
import fnmatch
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile

REPO_DEFAULT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GATES = 'scripts/gates'
MAP_DEFAULT = os.path.join(GATES, 'affected-map.json')

# ── 现读来源（相对仓根）────────────────────────────────────────────────
SRC_STEP_TABLE = 'scripts/gates/precommit-gates.sh'
SRC_COVERAGE = 'scripts/gates/gate-coverage.config.json'
SRC_NEWCMD = 'scripts/gates/newcmd-paths-baseline.json'

# C 面：命令面真源件 → 读它的门步（按「门步的命令里点名的判据脚本」反查，逐条有据）
CLI_TRUTH_FILES = [
    'core/cmd/zerg/main.go',
    'core/cmd/zerg/testdata/cli-matrix.json',
    'scripts/gates/cli-contract-baseline.json',
]
CLI_READER_SCRIPTS = [
    'scripts/gates/check-cli-contract.py',
    'scripts/gates/check-flag-registry-parity.py',
    'scripts/gates/check-help-shape.py',
    'scripts/gates/check-newcmd-followup.py',
]


# ── 本件自有的扫描排除面（甲① 第 0 刀）────────────────────────────────
# 只排「明显非源码的私有 / 本机残渣」，**不碰** `gate-coverage.config.json` 的 `exclude`
# （那份额外面属门① 覆盖面判据，另件另管 —— 本单禁动）。逐条有据（`.gitignore` 自身点名忽略）：
#   `.hermes/`  Hermes 运行时落地（桌面附件 = 用户私人文档 ⇒ 隐私 + 噪声）
#   `.stage/`   试验落地目录（机器专属 launchd plist ⇒ 非源码）
#   `.env`      本机私密环境变量（`.env.example` 是入库模板 ⇒ **不排**）
#   `ID`        0 字节重定向残渣（`.gitignore` 第 77 行点名）
# 只缩「盘上件」候选集 ⇒ ✗ 不改判定逻辑、✗ 不改退码含义、✗ 不动 125 条声明条目。
SELF_EXCLUDE_PREFIXES = ('.hermes/', '.stage/')
SELF_EXCLUDE_FILES = ('.env', 'ID')


def self_excluded(rel_path):
    """本件自有扫描排除面：私人 / 本机残渣不进「盘上件」候选集（只缩扫描面）。"""
    if rel_path in SELF_EXCLUDE_FILES:
        return True
    for pre in SELF_EXCLUDE_PREFIXES:
        if rel_path == pre.rstrip('/') or rel_path.startswith(pre):
            return True
    return False


# ── 现读 A：步表 ───────────────────────────────────────────────────────
def _dq_unescape(raw):
    """bash 双引号串的还原：`\\$`→`$` · `\\``→`` ` `` · `\\"`→`"` · `\\\\`→`\\` · `\\<换行>`→删；其余反斜杠字面保留。"""
    out = []
    i = 0
    while i < len(raw):
        c = raw[i]
        if c == '\\' and i + 1 < len(raw):
            n = raw[i + 1]
            if n in '$`"\\\n':
                out.append(n)
                i += 2
                continue
            out.append(c)
            i += 1
            continue
        out.append(c)
        i += 1
    return ''.join(out)


def _read_dq(s, i):
    """s[i] == '"' ⇒ 读一个双引号串（认 \\ 转义、允许跨行）。返回 (内容, 下一位置)。"""
    if s[i] != '"':
        raise ValueError('expect "')
    i += 1
    out = []
    while i < len(s):
        c = s[i]
        if c == '\\':
            out.append(s[i:i + 2])
            i += 2
            continue
        if c == '"':
            return _dq_unescape(''.join(out)), i + 1
        out.append(c)
        i += 1
    raise ValueError('unterminated quoted string')


def parse_step_table(text):
    """现读 `add_step <scope> "<名>" <模式> "<目录>" "<命令>"` ⇒ 步骤列表（跳过 `self` 自检 scope）。"""
    steps = []
    for m in re.finditer(r'(?m)^[ \t]*add_step[ \t]+', text):
        i = m.end()
        mm = re.match(r'([A-Za-z0-9_-]+)[ \t]+', text[i:])
        if not mm:
            continue
        scope = mm.group(1)
        i += mm.end()
        if scope == 'self':
            continue
        if text[i] != '"':
            continue
        name, i = _read_dq(text, i)
        while i < len(text) and text[i] in ' \t':
            i += 1
        mm = re.match(r'([A-Za-z0-9_-]+)', text[i:])
        mode = mm.group(1)
        i += mm.end()
        while i < len(text) and text[i] in ' \t':
            i += 1
        if text[i] != '"':
            continue
        d, i = _read_dq(text, i)
        while i < len(text) and text[i] in ' \t\n':
            i += 1
        if text[i] != '"':
            continue
        cmd, i = _read_dq(text, i)
        steps.append({'scope': scope, 'name': name, 'mode': mode, 'dir': d, 'cmd': cmd})
    return steps


def face_of(dir_expr):
    """`${REPO_ROOT}/core` ⇒ `core`；`${REPO_ROOT}` ⇒ `repo`；其余（自检 `${t}` 一类）⇒ None。"""
    m = re.match(r'^\$\{REPO_ROOT\}/?(.*)$', dir_expr)
    if not m:
        return None
    rel = m.group(1).strip('/')
    return rel if rel else 'repo'


SCRIPT_RE = re.compile(r'(scripts/[A-Za-z0-9_./\u4e00-\u9fff-]+\.(?:py|sh))')


def collect_named_scripts(cmd):
    """步命令里按名点到的仓内脚本件（`scripts/…py|sh`）。"""
    return sorted(set(SCRIPT_RE.findall(cmd)))


def read_text(root, rel):
    with open(os.path.join(root, rel), encoding='utf-8') as f:
        return f.read()


# ── 现读 B/C：覆盖面配置与命令面基线 ───────────────────────────────────
def load_json(root, rel):
    return json.loads(read_text(root, rel))


def excluded(rel_path, cfg):
    parts = set(cfg.get('exclude', {}).get('path_components', []))
    if parts & set(rel_path.split('/')):
        return True
    for pre in cfg.get('exclude', {}).get('prefixes', []):
        if rel_path == pre or rel_path.startswith(pre.rstrip('/') + '/'):
            return True
    return False


# ── 生成声明表 ─────────────────────────────────────────────────────────
def build_map(root):
    steps = parse_step_table(read_text(root, SRC_STEP_TABLE))
    cov = load_json(root, SRC_COVERAGE)
    newcmd = load_json(root, SRC_NEWCMD)

    # faces: 面 → 该面的步名（按步表工作目录现读分组）
    faces = {}
    for st in steps:
        f = face_of(st['dir'])
        faces.setdefault(f, []).append(st['name'])

    entries = []
    # 面型（非 repo）：目录前缀 ⇒ 该面全部步
    for f in sorted(faces):
        if f == 'repo':
            continue
        entries.append({
            'pattern': '%s/**' % f, 'kind': 'prefix', 'face': f,
            'steps': sorted(faces[f]),
            'evidence': '步表工作目录 = ${REPO_ROOT}/%s（%d 步）' % (f, len(faces[f])),
        })

    # 件型（repo 面）：步命令里按名点到的仓内脚本 ⇒ 该步
    by_script = {}
    repo_steps = [st for st in steps if face_of(st['dir']) == 'repo']
    for st in repo_steps:
        for sc in collect_named_scripts(st['cmd']):
            by_script.setdefault(sc, []).append(st['name'])
    for sc in sorted(by_script):
        entries.append({
            'pattern': sc, 'kind': 'literal', 'face': 'repo',
            'steps': sorted(set(by_script[sc])),
            'evidence': '步命令按名点到（%d 步）' % len(set(by_script[sc])),
        })

    # 件型（repo 面）：覆盖面登记面 —— whitelist.scripts 的件 ⇒ 门①
    gate1 = [n for n in faces.get('repo', []) if 'check-gate-coverage.py' in n]
    for w in cov.get('whitelist', {}).get('scripts', []):
        entries.append({
            'pattern': w['path'], 'kind': 'literal', 'face': 'repo',
            'steps': sorted(gate1),
            'evidence': 'gate-coverage.config.json whitelist.scripts 登记面（门① 覆盖面判据）',
        })
    # 覆盖面配置件本身 ⇒ 门①
    entries.append({
        'pattern': SRC_COVERAGE, 'kind': 'literal', 'face': 'repo',
        'steps': sorted(gate1),
        'evidence': '门① 覆盖面配置真源',
    })

    # 件型（repo 面）：命令面真源 ⇒ 读它的门步（现读 newcmd 基线 path 全集）
    cli_steps = []
    for st in repo_steps:
        named = set(collect_named_scripts(st['cmd']))
        if named & set(CLI_READER_SCRIPTS):
            cli_steps.append(st['name'])
    cli_steps = sorted(set(cli_steps))
    for tf in CLI_TRUTH_FILES:
        entries.append({
            'pattern': tf, 'kind': 'literal', 'face': 'repo',
            'steps': cli_steps,
            'evidence': '命令树 path 全集真源（newcmd-paths-baseline.json count=%d）⇒ %d 门步读它'
                        % (newcmd.get('count', -1), len(cli_steps)),
        })
    entries.append({
        'pattern': SRC_NEWCMD, 'kind': 'literal', 'face': 'repo',
        'steps': sorted(set(n for n in faces.get('repo', []) if 'check-newcmd-followup.py' in n)),
        'evidence': '命令面随动冻结基线件（门78 读它）',
    })

    # 去重（同 pattern 合并 steps）
    merged = {}
    for e in entries:
        k = e['pattern']
        if k in merged:
            merged[k]['steps'] = sorted(set(merged[k]['steps']) | set(e['steps']))
            merged[k]['evidence'] = merged[k]['evidence'] + ' ／ ' + e['evidence']
        else:
            merged[k] = e
    entries = [merged[k] for k in sorted(merged)]

    # 悬空过滤：literal 指不到的件不当声明（记入 dropped_ghosts 供追溯，✗ 不当成已声明）
    dropped = []
    keep = []
    for e in entries:
        if e['kind'] == 'literal' and not os.path.isfile(os.path.join(root, e['pattern'])):
            dropped.append(e['pattern'])
            continue
        keep.append(e)
    entries = keep

    # 未声明面：脚本候选集里零命名者单列（扫描面 ⇒ 现读）
    scan_globs = cov.get('scan', {}).get('script_globs', [])
    declared_literals = set(e['pattern'] for e in entries if e['kind'] == 'literal')
    undeclared_scripts = []
    for g in scan_globs:
        import glob as _glob
        for p in sorted(_glob.glob(os.path.join(root, g))):
            rel = os.path.relpath(p, root)
            if self_excluded(rel):
                continue
            if os.path.isfile(os.path.join(root, rel)) and rel not in declared_literals:
                undeclared_scripts.append(rel)

    fp = fingerprint(root, steps, cov, newcmd)
    return {
        'schema': 'affected-map/1',
        '_口径': ('声明表（件→步/面 映射真源）· 由 `python3 scripts/gates/check-affected-map.py --emit` '
                  '**现读生成**（A 步表 ＋ B gate-coverage.config.json ＋ C newcmd-paths-baseline.json）· '
                  '✗ 手敲。未声明件按已拍「默认每轮都跑」处置（本表 files 未覆盖者即未声明）。'),
        'source_head': git_head(root),
        'input_fingerprint': fp,
        '_fingerprint_note': ('入键 = 步表文本 ＋ 每个按名点到脚本件的 sha256 ＋ gate-coverage.config.json '
                              '＋ newcmd-paths-baseline.json ＋ git HEAD（不对称口径：宁多入不可少入）。'
                              '取不到 ⇒ 下游 affected-steps.py 报「不给结论」rc=2。'),
        'faces': {f: sorted(faces[f]) for f in sorted(faces)},
        'files': entries,
        'undeclared': {
            'note': ('脚本候选集（gate-coverage.config.json scan.script_globs 现读展开）里**零命名**者 ⇒ 未声明面；'
                     '按已拍第 3 条默认每轮都跑。✗ 不得读作已声明。'),
            'count': len(undeclared_scripts),
            'sample_first20': undeclared_scripts[:20],
        },
        'dropped_ghosts': dropped,
        '_dropped_ghosts_note': '现读来源点到、但盘上查无的件（✗ 不当声明 ⇒ 不冒「表里有件盘上无」）。',
    }


def git_head(root):
    try:
        out = subprocess.run(['git', '-C', root, 'rev-parse', 'HEAD'],
                             capture_output=True, text=True, check=True).stdout.strip()
        return out[:16]
    except Exception:
        return None


def fingerprint(root, steps, cov, newcmd):
    h = hashlib.sha256()
    for st in steps:
        h.update(('%s|%s|%s|%s|%s\n' % (st['scope'], st['name'], st['mode'], st['dir'], st['cmd'])).encode('utf-8'))
        for sc in collect_named_scripts(st['cmd']):
            p = os.path.join(root, sc)
            if os.path.isfile(p):
                with open(p, 'rb') as f:
                    h.update(hashlib.sha256(f.read()).digest())
    h.update(json.dumps(cov, sort_keys=True, ensure_ascii=False).encode('utf-8'))
    h.update(json.dumps(newcmd, sort_keys=True, ensure_ascii=False).encode('utf-8'))
    head = git_head(root)
    h.update((head or '').encode('utf-8'))
    return h.hexdigest()


# ── 双向差集机检 ───────────────────────────────────────────────────────
def disk_files(root, cov):
    """盘上件 = 文件系统现走（✗ 不经 `git ls-files` —— 设计 8.2：走 git 的选择器看不见 gitignore 生成物/未跟踪件）。"""
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames
                       if d not in set(cov.get('exclude', {}).get('path_components', []))]
        for fn in filenames:
            rel = os.path.relpath(os.path.join(dirpath, fn), root)
            if excluded(rel, cov):
                continue
            if self_excluded(rel):
                continue
            out.append(rel)
    return sorted(out)


def entry_matches(entry, rel):
    pat = entry['pattern']
    if entry['kind'] == 'prefix':
        base = pat[:-3] if pat.endswith('/**') else pat
        return rel == base or rel.startswith(base.rstrip('/') + '/')
    if entry['kind'] == 'glob':
        return fnmatch.fnmatch(rel, pat)
    return rel == pat


def check_map(mp, files_on_disk):
    """纯函数（便于自检）：返回 (rc, report dict)。files_on_disk 为已按 exclude 过滤的相对路径表。"""
    if not isinstance(mp, dict) or 'files' not in mp:
        return 1, {'reason': '缺读数：声明表缺 `files`（空/坏件）⇒ fail-closed', 'class1': [], 'class2': [],
                   'declared': 0, 'both': 0}
    entries = mp.get('files') or []
    if not entries:
        return 1, {'reason': '缺读数：声明表 `files` 为空 ⇒ 无鉴别力 ⇒ fail-closed', 'class1': [], 'class2': [],
                   'declared': 0, 'both': 0}

    matched = {i: 0 for i in range(len(entries))}
    covered = set()
    for rel in files_on_disk:
        for i, e in enumerate(entries):
            if entry_matches(e, rel):
                matched[i] += 1
                covered.add(rel)

    class1 = [r for r in files_on_disk if r not in covered]        # 盘上有件但表里没声
    class2 = [entries[i]['pattern'] for i in range(len(entries)) if matched[i] == 0]  # 表里有件盘上无
    both = [entries[i]['pattern'] for i in range(len(entries)) if matched[i] > 0]
    rep = {'reason': '', 'class1': class1, 'class2': class2,
           'declared': len(entries), 'both': len(both)}
    rc = 1 if class2 else 0
    return rc, rep


# ── 步面格（拍乙 · D9 步面接线）：**复甲① C0 唯一实现** ──────────────────────
def _load_affected_steps():
    """importlib 现载**邻件** `scripts/gates/affected-steps.py`（C0 唯一真源 · ✗ 不另写第二份「逐字相等」）。
    取不到 ⇒ None（调用方按「判不了」rc=2 fail-closed）。"""
    import importlib.util
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'affected-steps.py')
    if not os.path.isfile(p):
        return None
    spec = importlib.util.spec_from_file_location('_affected_steps_c0', p)
    if spec is None or spec.loader is None:
        return None
    mod = importlib.util.module_from_spec(spec)
    prev = sys.dont_write_bytecode
    sys.dont_write_bytecode = True          # ✗ 不在仓内 scripts/gates 落 .pyc 副产物
    try:
        spec.loader.exec_module(mod)
    finally:
        sys.dont_write_bytecode = prev
    return mod


def declared_face_names(mp):
    """声明表 `faces` 展平**去重**后的步名表（门84 侧取法 · 与甲① `analyse()` 逐字同构）。"""
    faces = mp.get('faces') or {}
    named, seen = [], set()
    for f in sorted(faces):
        for n in faces[f]:
            if n not in seen:
                seen.add(n)
                named.append(n)
    return named


def check_step_face(table_names, mp):
    """步面格（拍乙）：`faces` 命名步集合 ⟷ 步骤表步集合 的「`D==N` ＋ 逐字相等」。
    ★ 复甲① `affected-steps.py` 的 C0 唯一实现（`c0_step_face()`）—— ✗ 本件不写第二份判据。
    返回 (rc, detail)：0 步面齐（绿）· 3 步面漂移（红 · **单列一格** · ✗ 不与悬空声明的 rc=1 混）·
    2 判不了（C0 真源取不到 ⇒ fail-closed）。"""
    mod = _load_affected_steps()
    if mod is None:
        return 2, 'C0 唯一真源取不到（邻件 affected-steps.py 不在）⇒ 判不了'
    ok, detail = mod.c0_step_face(list(table_names), declared_face_names(mp))
    return (0, detail) if ok else (3, detail)


def cmd_check(root, map_path):
    if not os.path.isfile(map_path):
        print('[缺读数] 声明表不在：%s ⇒ rc=1（fail-closed）' % map_path)
        return 1
    try:
        with open(map_path, encoding='utf-8') as f:
            mp = json.load(f)
    except Exception as e:
        print('[缺读数] 声明表读不出：%s ⇒ rc=1' % e)
        return 1
    cov = load_json(root, SRC_COVERAGE)
    try:
        files_on_disk = disk_files(root, cov)
    except Exception as e:
        print('[缺读数] git ls-files 取不到：%s ⇒ rc=1（fail-closed）' % e)
        return 1
    rc, rep = check_map(mp, files_on_disk)
    # ★ 步面格（拍乙）：先算 —— 与件面格（① ②）**分列各算**（✗ 不混同一 rc）
    try:
        table_names = [s['name'] for s in parse_step_table(read_text(root, SRC_STEP_TABLE))]
        if not table_names:
            sf_rc, sf_detail = 2, '步骤表解析 0 步（空转 ⇒ 判不了）'
        else:
            sf_rc, sf_detail = check_step_face(table_names, mp)
    except Exception as e:
        sf_rc, sf_detail = 2, '步骤表取不到：%s ⇒ 判不了' % e
    print('声明表：%s' % map_path)
    print('  schema=%s · source_head=%s · 指纹=%s' % (
        mp.get('schema'), mp.get('source_head'), str(mp.get('input_fingerprint'))[:16]))
    print('  声明条目 %d（both=%d）· 盘上件（已按 exclude 过滤）%d' % (rep['declared'], rep['both'], len(files_on_disk)))
    if rep['reason']:
        print('  [缺读数] %s ⇒ rc=1' % rep['reason'])
        return rc
    print('  ① 盘上有件但表里没声（未声明 ⇒ 默认每轮都跑）：%d 条' % len(rep['class1']))
    for x in rep['class1'][:10]:
        print('      - %s' % x)
    print('  ② 表里有件盘上无（悬空声明 ⇒ 红）：%d 条' % len(rep['class2']))
    for x in rep['class2'][:10]:
        print('      - %s' % x)
    sf_word = '绿' if sf_rc == 0 else ('红' if sf_rc == 3 else '判不了')
    print('  ③ 步面（件面＋步面 · D==N ＋ 逐字相等 · 复甲① C0 唯一实现）：%s · %s'
          % (sf_word, sf_detail))
    # ★ 两格**分列各算**（✗ 不混同一 rc）：步面漂移 rc=3 · 悬空声明 rc=1 · 步面判不了 rc=2
    out_rc = 2 if sf_rc == 2 else (3 if sf_rc == 3 else rc)
    print('  rc=%d（0 读回成功 · 1 悬空声明/缺读数 · 3 步面漂移 · 2 判不了）' % out_rc)
    return out_rc


# ── 成对负控 ───────────────────────────────────────────────────────────
def self_test():
    fails = []
    tmp = tempfile.mkdtemp(prefix='affected-map-selftest-')
    try:
        disk = ['core/a.go', 'core/b.go', 'scripts/gates/x.py']
        # ① 正控：声明齐（覆盖盘上每件）⇒ rc=0 · 两类差集皆空 · both=2
        m1 = {'files': [
            {'pattern': 'core/**', 'kind': 'prefix'},
            {'pattern': 'scripts/gates/x.py', 'kind': 'literal'}]}
        rc, rep = check_map(m1, disk)
        if rc != 0:
            fails.append('① 正控期望 rc=0，实得 %d' % rc)
        elif rep['class1'] or rep['class2'] or rep['both'] != 2:
            fails.append('① 正控差集/计数不对：%r' % rep)
        # ② 负控：盘上多一件没声明 ⇒ class1 非空（≠红，rc 仍 0）
        rc, rep = check_map(m1, disk + ['docs/extra.md'])
        if rc != 0:
            fails.append('② 未声明件期望 rc=0（未声明是常态），实得 %d' % rc)
        elif rep['class1'] != ['docs/extra.md']:
            fails.append('② 未点名「盘上有件但表里没声」：%r' % rep['class1'])
        # ③ 负控：表里有件盘上无（悬空声明）⇒ rc=1 点名叫出来
        m3 = {'files': [
            {'pattern': 'core/**', 'kind': 'prefix'},
            {'pattern': 'scripts/gates/ghost.py', 'kind': 'literal'}]}
        rc, rep = check_map(m3, disk)
        if rc != 1:
            fails.append('③ 悬空声明期望 rc=1，实得 %d' % rc)
        elif rep['class2'] != ['scripts/gates/ghost.py']:
            fails.append('③ 未点名悬空声明：%r' % rep['class2'])
        # ④ 负控：声明件空 files ⇒ rc=1（无鉴别力，✗ 不许空转当绿）
        rc, _ = check_map({'files': []}, disk)
        if rc != 1:
            fails.append('④ 空表期望 rc=1，实得 %d' % rc)
        # ⑤ 负控：声明件缺 files（坏件）⇒ rc=1
        rc, _ = check_map({'schema': 'affected-map/1'}, disk)
        if rc != 1:
            fails.append('⑤ 坏件期望 rc=1，实得 %d' % rc)
        # ⑥ 负控：声明表件不在 ⇒ 命令档 rc=1（真起一次进程，不走纯函数）
        rc = cmd_check(os.path.dirname(tmp), os.path.join(tmp, 'no-such-map.json'))
        if rc != 1:
            fails.append('⑥ 缺件期望 rc=1，实得 %d' % rc)
        # ⑦–⑩ 端到端（真起 cmd_check · 走 importlib 载邻件 C0 唯一实现 · 走 ③ 步面格）
        import contextlib as _ctx
        import io as _io

        def _q(map_path):
            with _ctx.redirect_stdout(_io.StringIO()):
                return cmd_check(root, map_path)

        root = os.path.join(tmp, 'repo')
        os.makedirs(os.path.join(root, 'core'))
        os.makedirs(os.path.join(root, 'scripts', 'gates'))
        with open(os.path.join(root, 'core', 'a.go'), 'w', encoding='utf-8') as f:
            f.write('package main\n')
        with open(os.path.join(root, 'scripts', 'gates', 'precommit-gates.sh'), 'w', encoding='utf-8') as f:
            f.write('DEFAULT_SCOPES=("go")\n'
                    'add_step go "a-step" rc "${REPO_ROOT}/core" "go build ./..."\n'
                    'add_step gates "b-step" tri "${REPO_ROOT}" "python3 scripts/gates/x.py"\n'
                    'add_step gates "c-step" tri "${REPO_ROOT}" "python3 scripts/gates/y.py"\n')
        with open(os.path.join(root, 'scripts', 'gates', 'gate-coverage.config.json'), 'w', encoding='utf-8') as f:
            f.write('{"scan":{"script_globs":["scripts/gates/*.py"]},'
                    '"exclude":{"path_components":[".git"],"prefixes":[]},"whitelist":{"scripts":[]}}')
        disk_ok = [{'pattern': 'core/**', 'kind': 'prefix'}]
        faces_ok = {'core': ['a-step'], 'repo': ['b-step', 'c-step']}

        def _wmap(name, files, faces):
            p = os.path.join(tmp, name)
            with open(p, 'w', encoding='utf-8') as f:
                json.dump({'schema': 'affected-map/1', 'input_fingerprint': 'f' * 64,
                           'source_head': 'deadbeef', 'files': files, 'faces': faces}, f)
            return p

        # ⑦ 正控：步面齐（D==N 且逐字相等）⇒ rc=0（③ 绿）
        if _q(_wmap('m7.json', disk_ok, faces_ok)) != 0:
            fails.append('⑦ 步面齐正控期望 rc=0')
        # ⑧ 负控：删 faces 一步（D≠N）⇒ rc=3（**步面格 · ≠ 悬空声明的 1**）
        if _q(_wmap('m8.json', disk_ok, {'core': ['a-step'], 'repo': ['b-step']})) != 3:
            fails.append('⑧ 步面漂移(D≠N)期望 rc=3')
        # ⑨ 负控：D==N 但集合不等（换名）⇒ rc=3
        if _q(_wmap('m9.json', disk_ok, {'core': ['a-step'], 'repo': ['b-step', 'zzz']})) != 3:
            fails.append('⑨ 步面集合不等期望 rc=3')
        # ⑩ 负控：悬空声明 ⇒ rc=1（**≠ 步面漂移的 3** · 两格 rc 分列不混同 · 有牙）
        if _q(_wmap('m10.json',
                    disk_ok + [{'pattern': 'scripts/gates/ghost.py', 'kind': 'literal'}],
                    faces_ok)) != 1:
            fails.append('⑩ 悬空声明期望 rc=1（✗ 不与步面 3 混同）')
    finally:
        import shutil
        shutil.rmtree(tmp, ignore_errors=True)
    if fails:
        print('自检不过：')
        for x in fails:
            print('  ✗ ' + x)
        return 2
    print('自检通过：10 控（正控 2 ＋ 负控 8）逐条对拍 ✓')
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--emit', action='store_true')
    ap.add_argument('--check', action='store_true')
    ap.add_argument('--map')
    ap.add_argument('--out')
    ap.add_argument('--root', default=REPO_DEFAULT)
    ap.add_argument('--self-test', action='store_true')
    ns = ap.parse_args()
    if ns.self_test:
        return self_test()
    if ns.emit:
        mp = build_map(ns.root)
        out = ns.out or os.path.join(ns.root, MAP_DEFAULT)
        text = json.dumps(mp, ensure_ascii=False, indent=1, sort_keys=False) + '\n'
        if ns.out == '-':
            sys.stdout.write(text)
        else:
            with open(out, 'w', encoding='utf-8') as f:
                f.write(text)
            print('已生成声明表：%s（声明条目 %d · 未声明脚本件 %d · 悬空过滤 %d）'
                  % (out, len(mp['files']), mp['undeclared']['count'], len(mp['dropped_ghosts'])))
        return 0
    if ns.check:
        mp = ns.map or os.path.join(ns.root, MAP_DEFAULT)
        return cmd_check(ns.root, mp)
    print('用法：check-affected-map.py (--emit | --check [--map <件>] | --self-test)', file=sys.stderr)
    return 2


if __name__ == '__main__':
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
check-build-tags.py — 双构建工程门禁（任务表 T6.3；设计稿 §〇 A3/A4/A5/A6）

一句话：把「默认构建」与「-tags=debug 构建」当成**两个必须等价的工程配置**来守。
背景（已实测，勿推翻）见 skill `references/debug-build-and-verification.md` 与
`docs/01-设计/设计-内建调试版-v1.2-20260917.md`（第〇节 A 组）。

六条判据
--------
(a) tag 命名（A3）
    a1 未登记的自定义 tag：构建约束里出现「既不是工具链保留名、也不在白名单」的 tag ⇒ FAIL（列 文件:tag）
    a2 白名单自身撞保留名：允许的自定义 tag 一旦取名撞上 GOOS/GOARCH/unix/cgo/gc/gccgo/go1.x ⇒ FAIL
    a3 把保留名当 tag 传：脚本/CI 里的 tag 实参（`-tags` / `--build-tags` / `--tags`，`=` 或空格形式）
       含保留名或未登记名 ⇒ FAIL（列 文件:行:实参）—— 实测就是这个形态能把平台文件编进来
    a4 裸平台 tag 未配对：无隐含后缀的 .go 里出现**正向** GOOS/GOARCH，而所在目录既没有反面
       （`!同名`）约束、也没有任何别的平台隐含文件名 ⇒ FAIL（列 文件:tag）
(b) 两个构建都过（A4 前半）：每个 Go 模块 `go build ./...` 与 `go build -tags=debug ./...` 各取**真实退出码**
(c) vet 成对跑（A4）：`go vet ./...` 与 `go vet -tags=debug ./...` 各取**真实退出码**
    —— 被 tag 排除的文件在默认配置下 vet/lint/gofmt 全部看不见，只跑一侧等于没跑
(d) 导出面奇偶校验（A5）：对受关注包在两种配置下取导出数据（`go list -export`），
    交给 `scripts/exportnames`（go/importer）读出两套导出名集合，**不一致即 FAIL**
    —— 在多写一个导出符号时两个构建都会成功，编译器抓不到「多」
(e) GOFLAGS 断言（A6）：继承环境里若带 tag 相关旗标（一个环境变量就能把生产构建变成调试构建）⇒ FAIL；
    且本脚本**所有** go 调用显式覆盖 `GOFLAGS`（绝不靠环境继承）

用法
----
    python3 scripts/check-build-tags.py                    # 自检 + 扫真目标（仓根 = 本脚本上级目录）
    python3 scripts/check-build-tags.py --self-test        # 只跑自检（合成夹具，不碰真目标）
    python3 scripts/check-build-tags.py --root <目录>       # 扫指定仓根（自检夹具也走这条入口）
    python3 scripts/check-build-tags.py --parity-pkg <相对路径>  # 指定导出面校验包（可重复；缺省用脚本顶部名单）
    python3 scripts/check-build-tags.py --no-self-test     # 跳过自检（迭代调试用；门禁别用）
    python3 scripts/check-build-tags.py --list             # 只列判据与名单，不跑

退出码：0 全绿 · 1 有 FAIL 项 · 2 用法错/前置缺件/自检不过（**不给结论**）

纪律：无 sudo · 不联网（只用本机模块缓存）· 临时目录 mktemp -d + 退出即清理 · 前台跑、不起后台进程。
"""

import atexit
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.abspath(os.path.join(HERE, '..'))
HELPER_DIR = os.path.join(HERE, 'exportnames')
SELF_PATH = os.path.abspath(__file__)

# ══════════════════════════════════════════════════════════════════
# 门禁契约（改这里 = 改契约，评审点就在这几行）
# ══════════════════════════════════════════════════════════════════

# 允许出现的**项目自定义 tag**（可扩展名单）。未登记的 tag 一律 FAIL ——
# 新 tag = 新构建配置 = 新的一对必须成对跑的检查，必须显式登记。
ALLOWED_CUSTOM_TAGS = ['debug']

# 导出面奇偶校验的**受关注包**（可扩展名单；仓内相对路径）。加包 = 扩面，删包 = 缩面。
EXPORT_PARITY_PKGS = [
    'core/internal/toolobs',
    'core/internal/infergeom',
    'core/internal/obs/redact',
]

# 工具链保留名的**非平台**部分（平台名由 `go tool dist list` 现取，见 reserved_names()）：
# 这些名字都被工具链自己占用，自定义 tag 撞上它们 = 语义被静默接管（A3）。
FIXED_RESERVED = [
    'unix', 'cgo', 'gc', 'gccgo', 'ignore',
    'race', 'msan', 'asan',
    'boringcrypto', 'openssl', 'purego', 'osusergo', 'netgo', 'fuzz',
]

# `go1.N` 发布 tag：一律按**形态**判（版本号会漂，写死一份清单必过期）
RELEASE_TAG_RE = re.compile(r'^go1\.\d+$')

# 扫描时跳过的目录（每个都写明理由，别凭感觉加）
SKIP_DIRS = {
    'vendor',        # 第三方源码，不是我们的构建面
    '.git', '.zerg', '.hermes',
    'testdata',      # go 工具本身就不编译它，里面的 .go 是夹具
    'node_modules', 'target', 'dist', 'bin', 'data',
    'docs',          # 文档里的 Go 片段（如脱敏原型）不参与构建
    'venv', '.venv',                       # 本机第三方 Python 运行时（tools/ocr/venv 等）
}

MAX_MODULE_DEPTH = 2          # 模块发现深度（仓根 → 子目录 → 孙目录）
BUILD_TIMEOUT = 900           # 单条 go 命令的上限（秒）；超时 = 判不了（rc=2），不是红
FIXTURE_TIMEOUT = 300         # 自检夹具单条命令上限
TAIL_LINES = 15               # 失败时回显的输出尾部行数


# ══════════════════════════════════════════════════════════════════
# 基础工具
# ══════════════════════════════════════════════════════════════════

class Report(object):
    """findings = 判红项（有结论：红）· undecidable = 判不了项（不给结论）"""

    def __init__(self):
        self.findings = []      # [(判据, 文本)]
        self.undecidable = []   # [文本]
        self.notes = []         # [文本]

    def fail(self, judgement, text):
        self.findings.append((judgement, text))

    def cannot(self, text):
        self.undecidable.append(text)

    def note(self, text):
        self.notes.append(text)

    def rc(self):
        if self.findings:
            return 1
        if self.undecidable:
            return 2
        return 0


def go_bin():
    for cand in ('go',):
        p = shutil.which(cand)
        if p:
            return p
    return None


def run(argv, cwd=None, env=None, stdin_data=None, timeout=BUILD_TIMEOUT):
    """返回 (rc, 输出)。rc=None 表示超时（判不了）· rc=None 且 out 以 '!!' 开头表示起不来。"""
    try:
        p = subprocess.Popen(argv, cwd=cwd, env=env,
                             stdin=subprocess.PIPE if stdin_data is not None else None,
                             stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    except OSError as e:
        return None, '!! 起不来: %s (%s)' % (' '.join(argv), e)
    try:
        out, _ = p.communicate(input=(stdin_data.encode('utf-8') if stdin_data else None),
                               timeout=timeout)
    except subprocess.TimeoutExpired:
        p.kill()
        out, _ = p.communicate()
        return None, (out or b'').decode('utf-8', 'replace') + \
            '\n!! 超时（>%ds）⇒ 判不了，不给结论' % timeout
    return p.returncode, (out or b'').decode('utf-8', 'replace')


def tail(text, n=TAIL_LINES):
    lines = [ln for ln in (text or '').splitlines() if ln.strip()]
    return '\n'.join(lines[-n:])


def tag_env():
    """所有 go 调用的环境：**显式覆盖 GOFLAGS**（A6：绝不靠环境继承）。"""
    env = dict(os.environ)
    env['GOFLAGS'] = ''
    return env


# ══════════════════════════════════════════════════════════════════
# 模块与保留名
# ══════════════════════════════════════════════════════════════════

def dist_platforms(go):
    """(goos, goarch) 集合 —— 现取，不写死（工具链升级时平台名会变）。"""
    rc, out = run([go, 'tool', 'dist', 'list'], cwd=REPO_ROOT, env=tag_env(), timeout=120)
    if rc != 0:
        return None, None, out
    goos, goarch = set(), set()
    for ln in out.splitlines():
        ln = ln.strip()
        if not ln or '/' not in ln:
            continue
        a, b = ln.split('/', 1)
        goos.add(a)
        goarch.add(b)
    if not goos or not goarch:
        return None, None, 'go tool dist list 输出为空'
    return goos, goarch, ''


def reserved_names(goos, goarch):
    s = set(goos) | set(goarch) | set(FIXED_RESERVED)
    return s


def is_reserved(tag, reserved):
    return (tag in reserved) or bool(RELEASE_TAG_RE.match(tag))


def discover_modules(root):
    """返回 [(模块路径, 模块目录)]：root 自身及 depth ≤ MAX_MODULE_DEPTH 内的 go.mod。"""
    out = []
    root = os.path.abspath(root)
    for dirpath, dirnames, filenames in os.walk(root):
        rel = os.path.relpath(dirpath, root)
        depth = 0 if rel == '.' else rel.count(os.sep) + 1
        dirnames[:] = [d for d in sorted(dirnames) if d not in SKIP_DIRS]
        if depth > MAX_MODULE_DEPTH:
            dirnames[:] = []
            continue
        if 'go.mod' in filenames:
            mp = module_path(os.path.join(dirpath, 'go.mod'))
            if mp:
                out.append((mp, dirpath))
            # 不再往模块内部找嵌套模块（本仓没有；有也别把子模块算进本模块的文件扫描）
            dirnames[:] = []
    return out


def module_path(go_mod):
    try:
        with open(go_mod, encoding='utf-8') as f:
            for ln in f:
                m = re.match(r'^module\s+(\S+)\s*$', ln.strip())
                if m:
                    return m.group(1)
    except (IOError, OSError):
        return None
    return None


def module_of(mods, path):
    """最长前缀匹配：给定目录属于哪个模块。"""
    best = None
    for mp, md in mods:
        if path == md or path.startswith(md + os.sep):
            if best is None or len(md) > len(best[1]):
                best = (mp, md)
    return best


def scan_go_files(module_dir):
    files = []
    for dirpath, dirnames, filenames in os.walk(module_dir):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        # 嵌套模块不归本模块管
        if dirpath != module_dir and 'go.mod' in filenames:
            dirnames[:] = []
            continue
        for fn in filenames:
            if fn.endswith('.go'):
                files.append(os.path.join(dirpath, fn))
    return sorted(files)


# ══════════════════════════════════════════════════════════════════
# 构建约束解析
# ══════════════════════════════════════════════════════════════════

BUILD_RE = re.compile(r'^\s*//go:build\s+(.*)$')
LEGACY_RE = re.compile(r'^\s*//\s*\+build\s+(.*)$')
TOKEN_SPLIT_RE = re.compile(r'[\s()]+')


def parse_constraints(path):
    """返回 (正向 tag 集, 反向 tag 集)。解析 //go:build（含跨行）与旧式 // +build。"""
    pos, neg = set(), set()
    try:
        with open(path, encoding='utf-8', errors='replace') as f:
            lines = f.readlines()
    except (IOError, OSError):
        return None, None
    for i, ln in enumerate(lines[:30]):     # 约束只可能在文件头部
        m = BUILD_RE.match(ln)
        if m:
            expr = m.group(1)
            # 跨行：行尾以 && / || / ( 结尾时继续
            j = i
            while expr.rstrip().endswith(('&&', '||', '(')) and j + 1 < len(lines):
                j += 1
                expr += ' ' + lines[j].strip()
            for tok in TOKEN_SPLIT_RE.split(expr):
                if not tok or tok in ('&&', '||', '!'):
                    continue
                if tok.startswith('!'):
                    neg.add(tok[1:])
                else:
                    pos.add(tok)
            continue
        m = LEGACY_RE.match(ln)
        if m:
            # 旧式：空格 = 或；逗号 = 与；! 取反
            for grp in m.group(1).split():
                for part in grp.split(','):
                    part = part.strip()
                    if not part:
                        continue
                    if part.startswith('!'):
                        neg.add(part[1:])
                    else:
                        pos.add(part)
    return pos, neg


def implicit_platforms(filename, goos, goarch):
    """文件名隐含的构建约束（Go 只认 _GOOS / _GOARCH / _GOOS_GOARCH 后缀）。"""
    base = filename[:-3] if filename.endswith('.go') else filename
    if base.endswith('_test'):
        base = base[:-5]
    parts = base.split('_')
    out = set()
    if len(parts) >= 3 and parts[-2] in goos and parts[-1] in goarch:
        out.add(parts[-2])
        out.add(parts[-1])
    if len(parts) >= 2 and (parts[-1] in goos or parts[-1] in goarch):
        out.add(parts[-1])
    return out


# ══════════════════════════════════════════════════════════════════
# 判据 (a)：tag 命名
# ══════════════════════════════════════════════════════════════════

TAG_ARG_RES = [
    re.compile(r'-{1,2}(?:build-)?tags=([^\s"\'`]+)'),
    re.compile(r'-{1,2}(?:build-)?tags\s+([^\s"\'`]+)'),
]
SCAN_EXTS = ('.sh', '.bash', '.yml', '.yaml', '.py', '.mk', '.toml', '.cfg', '.conf')
# 一个 tag 实参必须长这样（纯名字）：散文、正则、printf 模板一律不判
TAG_VALUE_RE = re.compile(r'^[A-Za-z0-9_.+-]+$')
TAG_VALUE_TRIM = ';)]}\'"'
# 只把「命令行」上的 flag 当实参：真调用点行里必带这些词之一
COMMAND_HINTS = ('go ', 'go\t', '/go', 'golangci', 'build', 'vet', 'test', 'list', 'cmd/')


def tag_arg_candidates(line):
    """从一行里取「像 tag 实参」的名字（宁缺勿滥：宁可漏散文，不可把散文当实参）。"""
    if not any(h in line for h in COMMAND_HINTS):
        return []
    out = []
    for rex in TAG_ARG_RES:
        for m in rex.finditer(line):
            for raw in m.group(1).split(','):
                name = raw.strip().strip(TAG_VALUE_TRIM)
                if not name or name.startswith('-'):
                    continue
                if not TAG_VALUE_RE.match(name):
                    continue
                out.append((name, m.group(0)))
    return out


def check_tag_naming(root, mods, goos, goarch, rep):
    reserved = reserved_names(goos, goarch)
    allowed = set(ALLOWED_CUSTOM_TAGS)

    # ── a2：白名单自身不得撞保留名（先判，因为 a1 依赖白名单是对的）──
    for t in sorted(allowed):
        if is_reserved(t, reserved):
            rep.fail('a2', '白名单里的自定义 tag %r 撞上了工具链保留名 ⇒ 语义会被静默接管（改 tag 名，别改这条规矩）' % t)

    # ── a1 + a4：扫每个模块的 .go 文件 ──
    nfiles = 0
    custom_seen = {}          # tag -> [文件…]
    for mp, md in mods:
        files = scan_go_files(md)
        nfiles += len(files)
        bydir = {}
        for f in files:
            bydir.setdefault(os.path.dirname(f), []).append(f)
        for d, dfiles in sorted(bydir.items()):
            dir_implied, dir_neg = set(), set()
            parsed = {}
            for f in dfiles:
                pos, neg = parse_constraints(f)
                parsed[f] = (pos, neg)
                dir_implied |= implicit_platforms(os.path.basename(f), goos, goarch)
                dir_neg |= (neg or set())
            for f, (pos, neg) in sorted(parsed.items()):
                rel = os.path.relpath(f, root)
                for t in sorted(pos or set()):
                    if is_reserved(t, reserved):
                        continue                     # 平台/工具链名：a4 管
                    if t not in allowed:
                        custom_seen.setdefault(t, []).append(rel)
                # a4：裸平台 tag 必须能配对
                own = implicit_platforms(os.path.basename(f), goos, goarch)
                for t in sorted((pos or set()) & (set(goos) | set(goarch))):
                    if t in own:
                        continue
                    paired = (t in dir_neg) or bool(dir_implied - {t})
                    if not paired:
                        rep.fail('a4', '%s 里用裸平台名 %r 当构建约束，而该目录既没有反面（!%s）约束、也没有别的平台隐含文件'
                                 ' ⇒ 别的平台上一条同名 tag 就能把它编进来（实测形态）' % (rel, t, t))
    for t in sorted(custom_seen):
        rep.fail('a1', '未登记的自定义 tag %r（出现在 %s）—— 新 tag = 新构建配置，必须在检查脚本的 '
                       'ALLOWED_CUSTOM_TAGS 里登记' % (t, '、'.join(sorted(set(custom_seen[t]))[:5])))
    rep.note('a1/a2/a4 扫描面：%d 个 .go 文件 · %d 个模块 · 保留名 %d 个（GOOS %d + GOARCH %d + 固定 %d + go1.x 形态）'
             % (nfiles, len(mods), len(reserved), len(goos), len(goarch), len(FIXED_RESERVED)))

    # ── a3：脚本/CI 里的 tag 实参 ──
    nargs = 0
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in sorted(dirnames) if d not in SKIP_DIRS]
        for fn in sorted(filenames):
            if not (fn.endswith(SCAN_EXTS) or fn.startswith('Makefile')):
                continue
            path = os.path.join(dirpath, fn)
            if os.path.abspath(path) == SELF_PATH:
                continue            # 本脚本自身：里面写着「判据的形态」，不是调用点（自检夹具才是）
            try:
                with open(path, encoding='utf-8', errors='replace') as f:
                    lines = f.readlines()
            except (IOError, OSError):
                continue
            for i, ln in enumerate(lines):
                for name, shown in tag_arg_candidates(ln):
                    nargs += 1
                    if is_reserved(name, reserved):
                        rep.fail('a3', '%s:%d 把保留名 %r 当 tag 实参传（%s）—— 同名 tag 能顶替平台约束'
                                 % (os.path.relpath(path, root), i + 1, name, shown))
                    elif name not in allowed:
                        rep.fail('a3', '%s:%d 传了未登记的自定义 tag %r（%s）'
                                 % (os.path.relpath(path, root), i + 1, name, shown))
    rep.note('a3 扫描面：脚本/CI 里 %d 个 tag 实参' % nargs)


# ══════════════════════════════════════════════════════════════════
# 判据 (e)：GOFLAGS 断言
# ══════════════════════════════════════════════════════════════════

GOFLAGS_TAG_RE = re.compile(r'(?:^|\s)-{1,2}(?:build-)?tags(?:\s+|=)')


def check_goflags(rep):
    inherited = os.environ.get('GOFLAGS', '')
    if GOFLAGS_TAG_RE.search(inherited):
        rep.fail('e', '继承环境 GOFLAGS=%r 里带 tag 旗标 —— 一个环境变量就能把生产构建变成调试构建；'
                      '生产侧必须显式置空并断言（本脚本自己的 go 调用已显式覆盖 GOFLAGS=\'\')' % inherited)
    rep.note('e 状态：继承 GOFLAGS=%r（本脚本所有 go 调用已显式覆盖为空）' % inherited)


# ══════════════════════════════════════════════════════════════════
# 判据 (b)(c)：两个构建都过 + vet 成对跑
# ══════════════════════════════════════════════════════════════════

STEPS = [
    ('b', '默认构建 go build ./...', ['build', '-buildvcs=false', '-o', '@OUT@', './...']),
    ('b', '调试构建 go build -tags=debug ./...', ['build', '-buildvcs=false', '-tags=debug', '-o', '@OUT@', './...']),
    ('c', '默认 vet  go vet ./...', ['vet', './...']),
    ('c', '调试 vet  go vet -tags=debug ./...', ['vet', '-tags=debug', './...']),
]
# 注意旗标顺序：`-o` 必须在包模式（./...）**之前** —— go 的旗标解析遇到第一个非旗标参数就停，
# 写成 `go build ./... -o dir/` 会把 `-o` 当包名（本次实测就是这么炸的：夹具里两个构建都红）。

# 构建产物落点：**显式 -o 到临时目录**。为什么必须这样（实测踩到）：
# `go build ./...` 会把每个 main 包的二进制**写进当前目录** —— 本次实测在 scripts/exportnames/
# 里冒出 7 MB 的 `zerg-exportnames`、core/ 里落一串 zerg-core/zerg-api/…。门禁不该往仓里写散件
# （也正因如此，暂存时必须按路径点名，绝不能用 git add -A）。
_BUILD_OUT = []


def build_outdir():
    if not _BUILD_OUT:
        d = tempfile.mkdtemp(prefix='zerg-buildtags-out-')
        _BUILD_OUT.append(d)
        atexit.register(cleanup_outdir)
    return _BUILD_OUT[0] + os.sep


def cleanup_outdir():
    for d in _BUILD_OUT:
        shutil.rmtree(d, ignore_errors=True)
    del _BUILD_OUT[:]


def check_builds(mods, go, rep):
    outdir = build_outdir()
    for mp, md in mods:
        name = os.path.basename(md)
        for judgement, label, args in STEPS:
            args = [outdir if a == '@OUT@' else a for a in args]
            rc, out = run([go] + args, cwd=md, env=tag_env())
            extra = ''
            if rc is not None and rc != 0 and 'no main packages to build' in out:
                # 纯库模块：`-o <目录>/` 会硬报这个错。此时模块里没有 main 包 ⇒ **不带 -o 跑也不会
                # 往仓里写任何二进制**（散件的成因只在有 main 包时成立）⇒ 去掉 -o 重跑，不是放宽判据。
                # 注意要连**旗标与它的值一起**去掉：只删值会留下裸 `-o`，于是 `-o ./...` 被当成输出
                # 文件名（本次实测：夹具全红在「no Go files in …」上，是正控夹具把它逮住的）。
                plain, skip = [], False
                for a in args:
                    if skip:
                        skip = False
                        continue
                    if a == '-o':
                        skip = True
                        continue
                    plain.append(a)
                rc, out = run([go] + plain, cwd=md, env=tag_env())
                extra = '（该模块无 main 包 ⇒ 不带 -o 重跑，无产物可落）'
            if rc is None:
                rep.cannot('[%s] %s: %s —— %s' % (name, label, '超时/起不来', tail(out, 3)))
                continue
            mark = '✓' if rc == 0 else '✗'
            print('     %s [%s] %-38s rc=%s%s' % (mark, name, label, rc, extra))
            if rc != 0:
                rep.fail(judgement, '[%s] %s 真实退出码=%d；输出尾部:\n%s'
                         % (name, label, rc, tail(out)))
    rep.note('构建产物落点：%s（显式 -o 到临时目录，仓内不留散件；退出即清理）' % outdir.rstrip('/'))
    return True


# ══════════════════════════════════════════════════════════════════
# 判据 (d)：导出面奇偶校验
# ══════════════════════════════════════════════════════════════════

def export_file(go, importpath, tags, module_dir):
    """取某包的导出数据文件。**必须在模块目录里跑** go list（仓根没有 go.mod）。"""
    args = ['list', '-export', '-f', '{{.Export}}']
    if tags:
        args += tags
    args.append(importpath)
    rc, out = run([go] + args, cwd=module_dir, env=tag_env(), timeout=BUILD_TIMEOUT)
    if rc is None:
        return None, '（%s）超时/起不来' % importpath
    if rc != 0:
        return None, '（%s）go list 失败 rc=%d: %s' % (importpath, rc, tail(out, 5))
    path = out.strip().splitlines()[-1].strip() if out.strip() else ''
    if not path or not os.path.exists(path):
        return None, '（%s）go list 没给出可读的导出数据文件: %r' % (importpath, path)
    return path, ''


def check_parity(root, mods, go, pkgs, rep):
    """两套导出数据 → scripts/exportnames（go/importer）→ 导出名集合比对。"""
    targets = []
    for rel in pkgs:
        d = os.path.abspath(os.path.join(root, rel))
        if not os.path.isdir(d):
            rep.cannot('导出面：登记的受关注包不存在（名单过期？）: %s' % rel)
            continue
        mod = module_of(mods, d)
        if mod is None:
            rep.cannot('导出面：%s 不在任何已发现的 Go 模块里' % rel)
            continue
        sub = os.path.relpath(d, mod[1]).replace(os.sep, '/')
        importpath = mod[0] if sub == '.' else mod[0] + '/' + sub
        pa, ea = export_file(go, importpath, [], mod[1])
        pb, eb = export_file(go, importpath, ['-tags=debug'], mod[1])
        if pa is None or pb is None:
            rep.cannot('导出面：%s 的两套导出数据取不齐 —— %s %s' % (rel, ea, eb))
            continue
        targets.append({'rel': rel, 'pkg': importpath, 'a': pa, 'b': pb})

    if not targets:
        return
    payload = json.dumps({'targets': [{'pkg': t['pkg'], 'a': t['a'], 'b': t['b']} for t in targets]})
    rc, out = run([go, 'run', '.'], cwd=HELPER_DIR, env=tag_env(),
                  stdin_data=payload, timeout=BUILD_TIMEOUT)
    blob = None
    for ln in reversed((out or '').splitlines()):
        ln = ln.strip()
        if ln.startswith('{') and ln.endswith('}'):
            try:
                blob = json.loads(ln)
            except ValueError:
                blob = None
            if blob is not None:
                break
    if blob is None:
        rep.cannot('导出面：scripts/exportnames 没产出可解析的 JSON（rc=%s）: %s' % (rc, tail(out, 5)))
        return
    bypkg = dict((r.get('pkg'), r) for r in blob.get('results', []))
    for t in targets:
        r = bypkg.get(t['pkg'])
        if r is None:
            rep.cannot('导出面：%s 的结果缺失（helper 输出里没有该包）' % t['pkg'])
            continue
        if r.get('err'):
            rep.cannot('导出面：%s 导出数据读不出来 ⇒ 判不了：%s' % (t['rel'], r['err']))
            continue
        only_a = r.get('only_a') or []
        only_b = r.get('only_b') or []
        if only_a or only_b:
            rep.fail('d', '导出面不一致 %s（%s）：默认侧共 %s 名 / debug 侧共 %s 名；'
                          '只在默认侧 %s ｜ 只在 debug 侧 %s'
                     % (t['rel'], t['pkg'], r.get('n_a'), r.get('n_b'),
                        only_a if only_a else '无', only_b if only_b else '无'))
        else:
            print('     ✓ [导出面] %-34s 两侧各 %s 名，逐名一致'
                  % (t['rel'], r.get('n_a')))
    rep.note('导出面受关注包：%s' % '、'.join(t['rel'] for t in targets))


# ══════════════════════════════════════════════════════════════════
# 报告与主流程
# ══════════════════════════════════════════════════════════════════

def print_contract():
    print('门禁契约（改脚本顶部常量 = 改契约）')
    print('  允许的自定义 tag : %s' % ', '.join(ALLOWED_CUSTOM_TAGS))
    print('  导出面受关注包   : %s' % ', '.join(EXPORT_PARITY_PKGS))
    print('  保留名固定部分   : %s' % ', '.join(FIXED_RESERVED))
    print('  平台名           : 现取自 `go tool dist list`（GOOS/GOARCH 全集）')
    print('  判据             : a1 a2 a3 a4（tag 命名）· b（两构建都过）· c（vet 成对）'
          '· d（导出面奇偶）· e（GOFLAGS）')


def report(rep, title):
    print('────────────────────────────────────────────────────────────')
    if rep.findings:
        print('判红项 %d 条：' % len(rep.findings))
        for judgement, text in rep.findings:
            print('  ✗ [判据(%s)] %s' % (judgement, text))
    else:
        print('判红项 0 条')
    if rep.undecidable:
        print('判不了项 %d 条（不给结论）：' % len(rep.undecidable))
        for t in rep.undecidable:
            print('  ⚠ %s' % t)
    for n in rep.notes:
        print('  · %s' % n)
    rc = rep.rc()
    if rc == 0:
        print('结论[%s]: 全绿 —— 两个构建配置的工程面一致（tag 命名 / 构建 / vet / 导出面 / GOFLAGS）' % title)
    elif rc == 1:
        print('结论[%s]: 红灯 —— 上面每条都点名了文件与判定依据' % title)
    else:
        print('结论[%s]: 不给结论（rc=2）—— 有条目判不了（缺件 / 超时 / 读不出导出数据）' % title)
    return rc


def scan_real(args, rep):
    go = go_bin()
    if go is None:
        rep.cannot('本机 PATH 里没有 go —— 判据 b/c/d 全部无法判定（绝不把「判不了」当绿）')
        return rep
    goos, goarch, err = dist_platforms(go)
    if goos is None:
        rep.cannot('取不到平台名集合（go tool dist list 失败）：%s' % tail(err, 3))
        return rep
    mods = discover_modules(args['root'])
    if not mods:
        rep.cannot('在 %s 下没有发现任何 Go 模块（go.mod）—— 仓不完整？' % args['root'])
        return rep
    print('── (a)(e) 静态断言（红了就不烧构建时间）──')
    check_tag_naming(args['root'], mods, goos, goarch, rep)
    check_goflags(rep)
    if rep.findings:
        rep.note('静态断言已红 ⇒ 跳过构建/vet/导出面（先修命名与 GOFLAGS，再谈成对跑）')
        return rep
    print('── (b)(c) 成对构建与 vet（每模块 4 条，各取真实退出码）──')
    check_builds(mods, go, rep)
    print('── (d) 导出面奇偶校验（go list -export + go/importer）──')
    if rep.findings:
        rep.note('构建/vet 已红 ⇒ 导出面仍照跑（同一批读数，便于一次看清）')
    check_parity(args['root'], mods, go, args['parity'], rep)
    return rep


# ══════════════════════════════════════════════════════════════════
# 自检：合成夹具（正控 + 四类负控），自检不过 ⇒ 拒绝扫真目标
# ══════════════════════════════════════════════════════════════════

CLEAN_P = '''package p

// Foo 是夹具里唯一的导出函数。
func Foo() int { return 1 }

// T 是夹具里的导出类型。
type T struct{}

// M 是 T 的导出方法。
func (T) M() int { return 2 }
'''


def fixture_base(base):
    os.makedirs(os.path.join(base, 'p'))
    with open(os.path.join(base, 'go.mod'), 'w', encoding='utf-8') as f:
        f.write('module zergfixture\n\ngo 1.21\n')
    with open(os.path.join(base, 'p', 'p.go'), 'w', encoding='utf-8') as f:
        f.write(CLEAN_P)


def make_fixture(kind, tmproot):
    base = tempfile.mkdtemp(prefix='fx-%s-' % kind, dir=tmproot)
    fixture_base(base)
    if kind == 'reserved-tag':
        # ① 一个把**保留名**当项目 tag 用的文件（无隐含后缀、无配对）
        os.makedirs(os.path.join(base, 'q'))
        with open(os.path.join(base, 'q', 'zz_bare.go'), 'w', encoding='utf-8') as f:
            f.write('//go:build linux\n\npackage q\n')
        # ② 一个未登记的自定义 tag
        with open(os.path.join(base, 'q', 'zz_custom.go'), 'w', encoding='utf-8') as f:
            f.write('//go:build zergprobe\n\npackage q\n')
        # ③ 一个把保留名当 tag 实参传的脚本
        os.makedirs(os.path.join(base, 'scripts'))
        with open(os.path.join(base, 'scripts', 'fake-gate.sh'), 'w', encoding='utf-8') as f:
            f.write('#!/bin/sh\n# 夹具：把保留名当 tag 传\ngo build -tags=linux ./...\n')
    elif kind == 'one-side':
        # 只在 debug 一侧编译的文件，且它编不过（默认配置里完全不可见 —— A4 的现场）
        os.makedirs(os.path.join(base, 'q'))
        with open(os.path.join(base, 'q', 'ok.go'), 'w', encoding='utf-8') as f:
            f.write('package q\n\nvar OK = 1\n')
        with open(os.path.join(base, 'q', 'only_debug.go'), 'w', encoding='utf-8') as f:
            f.write('//go:build debug\n\npackage q\n\nvar Bad int = "这不是数字"\n')
    elif kind == 'parity':
        # 只在 debug 一侧**多出一个导出符号**（两个构建都能过 ⇒ 编译器抓不到「多」）
        with open(os.path.join(base, 'p', 'extra_debug.go'), 'w', encoding='utf-8') as f:
            f.write('//go:build debug\n\npackage p\n\n// DebugOnly 只在调试构建里存在。\nfunc DebugOnly() int { return 3 }\n')
    return base


def run_fixture(kind, root, parity_pkgs, extra_env=None):
    argv = [sys.executable, SELF_PATH, '--root', root, '--no-self-test']
    for p in parity_pkgs:
        argv += ['--parity-pkg', p]
    env = dict(os.environ)
    env['GOFLAGS'] = ''
    if extra_env:
        env.update(extra_env)
    rc, out = run(argv, cwd=root, env=env, timeout=FIXTURE_TIMEOUT)
    return rc, out


SELF_CASES = []


def self_test():
    """正控 1 例 + 负控 4 例。断言 = 退出码 + 必须出现的判词（缺一不可）。"""
    tmproot = tempfile.mkdtemp(prefix='zerg-buildtags-selftest-')
    bad = 0
    total = 0
    print('── 门禁自检（合成夹具 · 不碰真目标）──')
    try:
        cases = [
            # (夹具, parity 包, 额外环境, 期望 rc, 期望输出里必须出现的关键字, 说明)
            ('clean', ['p'], None, 0, ['全绿'], '正控：干净夹具必须全绿（镜子不是恒红）'),
            ('reserved-tag', ['p'], None, 1,
             ['zz_bare.go', "'linux'", 'zergprobe', 'fake-gate.sh'], '负控①：保留名 tag / 未登记 tag / 保留名当实参'),
            ('one-side', ['p'], None, 1,
             ['-tags=debug', '判据(b)', '判据(c)'], '负控②：只在 debug 一侧编译（且编不过）'),
            ('parity', ['p'], None, 1, ['DebugOnly', '判据(d)'], '负控③：debug 一侧多出导出符号 ⇒ 导出面不一致'),
            ('clean', ['p'], {'GOFLAGS': '-tags=debug'}, 1, ['判据(e)'], '负控④：GOFLAGS 污染 ⇒ 生产构建静默变调试构建'),
        ]
        for kind, pkgs, envx, want_rc, wants, desc in cases:
            root = make_fixture(kind, tmproot)
            rc, out = run_fixture(kind, root, pkgs, envx)
            total += 1
            missing = [w for w in wants if w not in (out or '')]
            ok = (rc == want_rc) and not missing
            if not ok:
                bad += 1
            print('  %s %s：期望 rc=%s 实得 rc=%s%s'
                  % ('✓' if ok else '✗', desc, want_rc, rc,
                     '' if ok else '；缺关键字 %s' % missing))
            # 变红的判词原文一起打出来（自检报告本身就是证据，别只报一个 rc）
            for ln in (out or '').splitlines():
                if ln.strip().startswith('✗ [判据'):
                    print('      ↳ %s' % ln.strip())
            if not ok:
                print('      ↳ 输出尾部：')
                for ln in tail(out, 12).splitlines():
                    print('      | %s' % ln)
    finally:
        shutil.rmtree(tmproot, ignore_errors=True)
    print('自检结论: %s（用例 %d 条，失败 %d 条）'
          % ('全过' if bad == 0 else '不过', total, bad))
    if bad:
        return 2
    return 0


# ══════════════════════════════════════════════════════════════════

def parse_args(argv):
    a = {'root': REPO_ROOT, 'parity': list(EXPORT_PARITY_PKGS), 'parity_given': False,
         'self_only': False, 'no_self': False, 'list': False}
    i = 0
    while i < len(argv):
        v = argv[i]
        if v == '--root' and i + 1 < len(argv):
            a['root'] = os.path.abspath(argv[i + 1]); i += 2
        elif v == '--parity-pkg' and i + 1 < len(argv):
            p = argv[i + 1]
            if not a['parity_given']:
                a['parity'] = []          # 一旦显式指定，就**替换**缺省名单（不是追加）
                a['parity_given'] = True
            if p not in a['parity']:
                a['parity'].append(p)
            i += 2
        elif v == '--self-test':
            a['self_only'] = True; i += 1
        elif v == '--no-self-test':
            a['no_self'] = True; i += 1
        elif v == '--list':
            a['list'] = True; i += 1
        elif v in ('-h', '--help'):
            print(__doc__)
            return None
        else:
            sys.stderr.write('✗ 未知参数: %s\n' % v)
            return None
    return a


def main(argv):
    args = parse_args(argv)
    if args is None:
        return 2
    if args['self_only'] and args['no_self']:
        sys.stderr.write('✗ --self-test 与 --no-self-test 不能同用\n')
        return 2

    if args['list']:
        print_contract()
        return 0

    if not args['no_self']:
        rc = self_test()
        if rc != 0:
            sys.stderr.write('✗ 自检不过 ⇒ 拒绝扫真目标（rc=2）\n')
            return 2
        if args['self_only']:
            return 0

    root = args['root']
    if not os.path.isdir(root):
        sys.stderr.write('✗ 仓根不存在: %s\n' % root)
        return 2
    print('════════════════════════════════════════════════════════════')
    print('双构建工程门禁 · 仓根 %s' % root)
    print_contract()
    print('════════════════════════════════════════════════════════════')
    rep = Report()
    scan_real(args, rep)
    return report(rep, os.path.basename(root))


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))

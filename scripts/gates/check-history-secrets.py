#!/usr/bin/env python3
"""全历史敏感审计（B0 / G1）—— 镜像私有历史**之前**的只读前置审计。

为什么必须先做：逐提交镜像会把**过去的每一笔**也过滤后公开 ⇒ 若历史上某笔曾提交过凭据/私有路径，
那笔会**永久**进入公开历史（压平快照恰好"顺手掩盖"了这个风险）。

本脚本**只读**：遍历指定范围的每一笔提交，按**发布白名单**取出"真会被公开的文件集"，
逐 blob 扫描敏感模式，并扫描提交信息本身；命中即逐条打印（敏感值一律**打码**），exit 1。

用法：
    python3 scripts/gates/check-history-secrets.py                     # 默认：--all（全历史）
    python3 scripts/gates/check-history-secrets.py --range A..B        # 只审区间（B1 复用）
    python3 scripts/gates/check-history-secrets.py --json /tmp/audit.json
    python3 scripts/gates/check-history-secrets.py --self-test         # 成对负控自检（仓外夹具 · 不碰真目标）

退码（三档 · 与仓内其余门脚本同惯例）
------------------------------------
  0 = 通过（全历史未发现阻断级敏感内容）· 也是 `--self-test` **全过**的出口
  1 = **判红**：有阻断级命中（逐条打码打印）· 也是 `--self-test` **有格不过**的出口
  2 = 判不了（`--repo` 不是 git 仓 / 自检夹具搭不出来 / 自检用例条数对不上 ⇒ 不许当绿）
★ 为何给本件补自检面（缺口 `Q-176` 第六批已入账的那一格：门件没有 `--self-test` 面 ⇒ `all.sh`
  只能把它记 `NA` ⇒ **它永远不能让总 rc 变红** ✗）：本件真跑是 **4 分钟**的只读全历史审计
  （提交闸里按 `tri-report` **只报告**跑 · `real-gates.sh` 里属 `SLOW` 档、只在 `--full` 跑），
  自检面因此只碰**合成夹具 ＋ 现读真仓结构**、**不跑全历史** —— 否则会把 `all.sh` 的逐门自检档
  （本来 ~20s）拖成分钟级。**判据与真跑退码一字未改**（只加 `--self-test` 这一档）。
"""
import argparse
import fnmatch
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
RULES_FILE = os.path.join(REPO, "publish", "replace-rules.tsv")
WHITELIST = os.path.join(REPO, "publish", "whitelist.txt")

# 夹具隔离（设计-CI适配-v1.1 §五 第三批）：`!夹具<TAB><路径 glob>` —— 命中 glob 的件**整段跳过**
# 替换规则（不逐条判）。夹具是「拿字面量与长度当判据」的那种东西：按字节改写就是改尺
# （本批公开面实测：`LEAK: 秘密 "~/…"` / `夹具失效：blob=…（len=32）`）。三处消费者同口径：
# 本件（全历史审计）· publish/mirror-public-lib.py（正式镜像器）· scripts/build/publish-public.sh（后备压平器）。
ISO_PREFIX = "!夹具"


def is_isolated(rel, isolated):
    """rel 是否命中夹具隔离 glob（fnmatch，`*` 跨 `/` —— 与规则表里的写法同口径）。"""
    return bool(rel) and any(fnmatch.fnmatch(rel, pat) for pat in isolated)

# 阻断级（命中即不应公开）—— 逐条给"为什么"
BLOCK = [
    (r"/Volumes/(AHZ|BXC)", "私有卷绝对路径"),
    (r"~", "用户家目录路径"),
    (r"ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}", "GitHub 令牌"),
    (r"sk-[A-Za-z0-9]{20,}", "API 密钥（sk-）"),
    (r"AKIA[0-9A-Z]{16}", "AWS Access Key"),
    (r"xox[baprs]-[A-Za-z0-9-]{10,}", "Slack 令牌"),
    (r"-----BEGIN [A-Z ]*PRIVATE KEY-----", "私钥块"),
    (r"(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*[\"']?[A-Za-z0-9._\-]{12,}", "疑似明文口令/密钥赋值"),
    (r"\bBearer\s+[A-Za-z0-9._\-]{20,}", "Bearer 令牌"),
    (r"192\.168\.\d{1,3}\.\d{1,3}", "内网地址 192.168.x"),
    (r"\b10\.0\.0\.\d{1,3}", "内网地址 10.0.0.x"),
]
# 提示级（会被替换规则过滤掉，但仍值得知道）
INFO = [
    (r"(?i)\.zerg/token|auth\.json", "提到私有凭据文件路径（路径引用，非密钥值）"),
    (r"<project-a>|<project-b>", "私有项目名（替换规则会过滤）"),
    (r"(?i)Mr2109", "人物名（替换规则会过滤）"),
]
BINARY_SUFFIX = (".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip", ".gz", ".tgz", ".bundle",
                 ".dylib", ".so", ".a", ".o", ".ttf", ".ttc", ".otf", ".woff", ".woff2",
                 ".mp3", ".mp4", ".mov", ".webp", ".ico", ".icns", ".lock")
MAX_BLOB = 2 * 1024 * 1024


def sh(args, cwd=REPO, binary=False):
    r = subprocess.run(args, cwd=cwd, capture_output=True,
                       text=not binary, encoding=None if binary else "utf-8",
                       errors=None if binary else "replace")
    return r.stdout


def load_whitelist(path=WHITELIST):
    entries = []
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            entries.append(line)
    return entries


PUBLISH_SH = os.path.join(REPO, "scripts", "build", "publish-public.sh")
NOISE_SUFFIX = (".bak", ".orig", ".rej")
NOISE_DIRS = ("__pycache__",)


def load_excludes(path=PUBLISH_SH):
    """从 publish-public.sh 的 EXCLUDES=( ... ) 数组里解析排除项——单一真源，避免两处漂移。"""
    ex = []
    if not os.path.exists(path):
        return ex
    text = open(path, encoding="utf-8").read()
    m = re.search(r"EXCLUDES=\((.*?)\n\)", text, re.S)
    if not m:
        return ex
    for raw in m.group(1).split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        line = line.split("#")[0].strip().strip('"').strip("'")
        if line:
            ex.append(line)
    return ex


def excluded(rel, ex):
    if rel.endswith(NOISE_SUFFIX) or any(part in NOISE_DIRS for part in rel.split("/")):
        return True
    for e in ex:
        if rel == e or rel.startswith(e.rstrip("/") + "/"):
            return True
    return False


def load_rules(path=RULES_FILE):
    """发布替换规则：默认字面量，`regex:` 前缀为正则，`!夹具` 前缀为**夹具隔离 glob**。
    必须先套规则再扫描——否则会把"本来就会被改写成占位符"的内容误报成泄露（第一版审计的硬伤）。"""
    literals, regexes, isolated = [], [], []
    if not os.path.exists(path):
        return literals, regexes, isolated
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                continue
            pat, rep = parts[0], parts[1]
            if pat == ISO_PREFIX:
                isolated.append(rep.strip())
            elif pat.startswith("regex:"):
                try:
                    regexes.append((re.compile(pat[6:]), rep))
                except re.error:
                    pass
            else:
                literals.append((pat, rep))
    return literals, regexes, isolated


def apply_rules(text, literals, regexes, isolated=(), rel=None):
    """套替换规则；`rel` 命中夹具隔离 ⇒ **原样返回**（规则不碰夹具的字节形态）。
    没有路径的文本（如提交信息，rel=None）不适用隔离。"""
    if rel is not None and is_isolated(rel, isolated):
        return text
    for pat, rep in literals:
        if pat in text:
            text = text.replace(pat, rep)
    for rx, rep in regexes:
        text = rx.sub(rep, text)
    return text


def published_path(rel, entries):
    for e in entries:
        if e.endswith("/"):
            if rel.startswith(e):
                return True
        elif rel == e:
            return True
    return False


def mask(s):
    s = s.strip()
    return s[:6] + "…" + ("（%d 字符）" % len(s)) if len(s) > 8 else s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default=REPO, help="仓库路径（默认本仓；自测/B1 复用时可指向别处）")
    ap.add_argument("--range", dest="rng", default=None, help="提交区间，如 A..B；默认 --all")
    ap.add_argument("--json", dest="json_out", default=None)
    ap.add_argument("--max-hits", type=int, default=200)
    # ★ `Q-176` 第六批：拼法统一 —— **准拼法 `--self-test`**，旧拼法 `--selftest` 作别名
    ap.add_argument("--self-test", "--selftest", dest="selftest", action="store_true",
                    help="只跑成对负控自检（合成夹具 + 现读真仓结构 · 不碰真目标 · 不跑全历史）")
    args = ap.parse_args()

    if args.selftest:
        import atexit
        atexit.register(cleanup)
        print("── check-history-secrets.py 自证（成对正负控 + 防空绿档 · 仓外夹具 · 真退出码）")
        return self_test()

    repo = os.path.abspath(args.repo)
    if not os.path.isdir(os.path.join(repo, ".git")):
        print("✗ 不是 git 仓库：%s" % repo)
        return 2

    literals, regexes, isolated = load_rules()
    ex_list = load_excludes()
    print("排除项: %d 条（从 publish-public.sh 的 EXCLUDES 解析）+ 噪声后缀 %s" % (len(ex_list), "/".join(NOISE_SUFFIX)))
    print("替换规则: %d 条字面 + %d 条正则（扫描前先套用——审计的是『真会公开的形状』）" % (len(literals), len(regexes)))
    print("夹具隔离: %d 条 glob（命中件整段跳过替换 · §五 第三批）%s"
          % (len(isolated), "：%s" % " · ".join(isolated) if isolated else ""))
    entries = load_whitelist(os.path.join(repo, "publish", "whitelist.txt") if os.path.exists(
        os.path.join(repo, "publish", "whitelist.txt")) else WHITELIST)
    if args.rng:
        revs = sh(["git", "rev-list", args.rng], cwd=repo).split()
        scope = args.rng
    else:
        revs = sh(["git", "rev-list", "--all"], cwd=repo).split()
        scope = "--all"
    print("审计范围: %s（%d 笔提交）" % (scope, len(revs)))
    print("白名单条目: %d（只审『真会被公开』的那些路径）" % len(entries))

    blob_cache = {}  # blob sha → (kind, why, sample)
    hits = []
    info_hits = []
    scanned_files = 0
    scanned_bytes = 0

    blk = [(re.compile(rx), why) for rx, why in BLOCK]
    inf = [(re.compile(rx), why) for rx, why in INFO]

    for idx, sha in enumerate(revs, 1):
        # ① 提交信息本身也会被镜像 ⇒ 一起审
        msg = apply_rules(sh(["git", "log", "-1", "--format=%B", sha], cwd=repo), literals, regexes)
        for rx, why in blk:
            m = rx.search(msg)
            if m:
                hits.append({"commit": sha[:10], "path": "<commit-message>", "why": why,
                             "match": mask(m.group(0))})
        # ② 该提交里"会被公开"的文件
        # ls-tree 带 blob sha ⇒ 同一内容只扫一次（历史上重复出现的文件不重复读）
        listing = sh(["git", "-c", "core.quotepath=false", "ls-tree", "-r", sha], cwd=repo)
        for line in listing.split("\n"):
            parts = line.split(None, 3)
            if len(parts) != 4:
                continue
            _mode, otype, osha, rel = parts
            if otype != "blob" or not published_path(rel, entries) or excluded(rel, ex_list):
                continue
            if rel.lower().endswith(BINARY_SUFFIX):
                continue
            iso = is_isolated(rel, isolated)
            verdict = blob_cache.get((osha, iso))
            if verdict is None:
                blob = sh(["git", "cat-file", "blob", osha], cwd=repo, binary=True)
                if blob is None or len(blob) > MAX_BLOB or b"\x00" in blob[:8192]:
                    blob_cache[(osha, iso)] = ("skip", None, None)
                    continue
                scanned_files += 1
                scanned_bytes += len(blob)
                try:
                    text = blob.decode("utf-8")
                except Exception:
                    blob_cache[(osha, iso)] = ("skip", None, None)
                    continue
                text = apply_rules(text, literals, regexes, isolated, rel)  # 关键：先变换，再扫描（夹具隔离件原样通过）
                verdict = ("clean", None, None)
                for rx, why in blk:
                    m = rx.search(text)
                    if m:
                        verdict = ("block", why, mask(m.group(0)))
                        break
                else:
                    for rx, why in inf:
                        if rx.search(text):
                            verdict = ("info", why, None)
                            break
                blob_cache[(osha, iso)] = verdict
            kind, why, sample = verdict
            if kind == "block":
                hits.append({"commit": sha[:10], "path": rel, "why": why, "match": sample})
            elif kind == "info":
                info_hits.append({"commit": sha[:10], "path": rel, "why": why})
        if idx % 200 == 0:
            print("  …已审 %d/%d 笔提交（文件 %d，%.1f MB）" % (idx, len(revs), scanned_files,
                                                          scanned_bytes / 1048576))

    print("\n扫描完成：提交 %d 笔 · 文件版本 %d 个 · %.1f MB 文本" % (len(revs), scanned_files,
                                                              scanned_bytes / 1048576))
    if info_hits:
        print("\n· 提示级（%d 处，替换规则会过滤，不影响公开）:" % len(info_hits))
        for h in info_hits[:10]:
            print("    %s  %-52s %s" % (h["commit"], h["path"][:52], h["why"]))
    if hits:
        from collections import Counter, OrderedDict
        bypath = OrderedDict()
        for h in hits:
            bypath.setdefault((h["path"], h["why"]), [0, h["commit"], h["match"]])
            bypath[(h["path"], h["why"])][0] += 1
        print("\n✗ 阻断级命中 %d 处，涉及 %d 个文件（按文件归并）：" % (len(hits), len(bypath)))
        for (path, why), (cnt, first_commit, sample) in sorted(bypath.items(), key=lambda kv: -kv[1][0])[:30]:
            print("    %-56s %-20s ×%-4d 首见 %s  %s" % (path[:56], why, cnt, first_commit, sample))
        print("\n  逐条明细（前 %d）:" % min(args.max_hits, len(hits)))
        for h in hits[: args.max_hits]:
            print("    %s  %-52s %-22s %s" % (h["commit"], h["path"][:52], h["why"], h["match"]))
        if len(hits) > args.max_hits:
            print("    …（其余 %d 处省略）" % (len(hits) - args.max_hits))
    else:
        print("\n结果: 通过——全历史未发现阻断级敏感内容 ✓")

    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump({"scope": scope, "commits": len(revs), "files": scanned_files,
                       "bytes": scanned_bytes, "block_hits": hits, "info_hits": info_hits},
                      fh, ensure_ascii=False, indent=2)
        print("  JSON 报告: %s" % args.json_out)
    return 1 if hits else 0


# ══════════════════════════════════════════════════════════════════
# 自检面（`--self-test`）：成对正负控 + 三格防空绿 · 仓外夹具（$TMPDIR）· 不碰真目标
# ══════════════════════════════════════════════════════════════════
# 设计真源（式样照同目录 `check-password-argv.py` / `check-marks-coverage-parity.py` /
# `check-no-symlink-farm.py` 的 `--self-test`）：docstring 写清真源 / 判据 / 退码三档 ·
# `WANT_CASES` 写死（少跑一格即断言失败）· 夹具全落**仓外**临时目录（合成 `git init` 仓）·
# 成对正负控（改坏 ⇒ 判红；复原 ⇒ 回绿）· 防空绿（把真源弄哑 / 换成恒真 ⇒ 读数必须跟着变：
# 变了才证明那条真源**有牙齿**，不变 ⇒ 真源本来就是死的 ⇒ 自检当场红）。
#   ① 判据真源**只有三处**：`BLOCK` 模式表（命中即判红）· `published_path()` 白名单（只审
#      「真会被公开」的路径）· `load_rules()` 替换规则（**先变换、再扫描** —— 见 `load_rules`
#      的注释：不先变换就会把「本来就会被改写成占位符」的内容误报成泄露，第一版审计的硬伤）。
#      `INFO` 只提示、不判红 ⇒ 不进判据。
#   ② 判据：同一段敏感文本 —— 白名单内且规则改不掉 ⇒ **必判红**（rc=1 并点名档名）；放白名单外 /
#      命中噪声后缀 / 规则会改写 ⇒ **必放绿**（rc=0）。两向都要有，缺一向就是半张网。
#   ③ 退码：见文件头「退码」那一段（真跑的 1 是「判红」；自检的 1 只在 `--self-test` 这一档出）。
SELF_PATH = os.path.abspath(__file__)
_TMPDIRS = []
WANT_CASES = 14                    # 自证用例条数（**写死**：少跑一格即断言失败；⑬⑭ = 中文名件回归）

# 夹具里的敏感文本一律**拼装写**：本门自己就在扫这些形状，源码里不留完整形态（免得本件被自己
# 的口径或别的门当成泄露；`publish/replace-rules.tsv` 也无从命中这些片段）。
_FIX_AWS = "AKIA" + "0" * 16                        # 判据 `AKIA[0-9A-Z]{16}`（AWS Access Key）
_FIX_PEM = "-----BEGIN " + "RSA PRIVATE KEY-----"   # 判据 `-----BEGIN [A-Z ]*PRIVATE KEY-----`
_FIX_SLACK = "xoxb-" + "0" * 12                     # 判据 `xox[baprs]-[A-Za-z0-9-]{10,}`（Slack 令牌）
_FIX_HOME = "/Users/" + "Mr2109"                      # **在替换规则里**（`~` → `~`）⇒ 钉「先变换再扫描」
# ★ 实测踩到的一条口径：**一个 blob 只出一个 verdict**（`main()` 在首个命中的 `BLOCK` 模式上
#   `break`）⇒ 要同时钉住两条判据，就得摆成**两个件**（两个 blob），不能塞进一个件里。
_FIX_DIRTY = ("# 夹具：一段不该公开的文本（拼装写 · 本件源码里不留完整形态）\n"
              + 'k1 = "%s"\n' % _FIX_AWS)
_FIX_PEM_FILE = 'k2 = "%s"\n' % _FIX_PEM
_FIX_HOME_FILE = 'HOME_HINT = "%s"\n' % _FIX_HOME
_FIX_GOOD = "def main():\n    return 0\n"
_FIX_WL = "src/\n"                                  # 夹具仓自己的白名单（本门读 `<repo>/publish/whitelist.txt`）


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _mk_fixture_repo(base, name, files, msg="夹具：初始提交", whitelist=_FIX_WL):
    """在**仓外**临时目录里现搭一个最小 git 仓（真 `git init` + 一笔提交）—— 不碰真仓一个字节。"""
    root = tempfile.mkdtemp(prefix=name + "-", dir=base)
    _TMPDIRS.append(root)
    _write(os.path.join(root, "publish", "whitelist.txt"), whitelist)
    for rel, body in files.items():
        _write(os.path.join(root, rel), body)
    quiet_out, quiet_err = subprocess.DEVNULL, subprocess.DEVNULL
    ident = ["-c", "user.name=selftest", "-c", "user.email=selftest@example.invalid"]
    subprocess.run(["git", "init", "-q"], cwd=root, stdout=quiet_out, stderr=quiet_err)
    subprocess.run(["git"] + ident + ["add", "-A"], cwd=root, stdout=quiet_out, stderr=quiet_err)
    subprocess.run(["git"] + ident + ["commit", "-q", "-m", msg], cwd=root,
                   stdout=quiet_out, stderr=quiet_err)
    return root


def _run_cli(repo):
    """跑**真命令行**（子进程 ⇒ 真退出码）⇒ (rc, 两路输出合并)。"""
    proc = subprocess.run([sys.executable, SELF_PATH, "--repo", repo],
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=REPO)
    return proc.returncode, (proc.stdout.decode("utf-8", "replace")
                             + proc.stderr.decode("utf-8", "replace"))


_SELF_MOD = None


def _self_module():
    """把本件当**模块**再装一份（进程内跑**同一份判据**）—— 防空绿档要换真源，子进程换不了。"""
    global _SELF_MOD
    if _SELF_MOD is None:
        import importlib.util
        spec = importlib.util.spec_from_file_location("chs_selftest_defs", SELF_PATH)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        _SELF_MOD = mod
    return _SELF_MOD


def _run_inproc(repo):
    """进程内跑 `main()`（同一份判据 · 真返回值）⇒ (rc, 两路输出合并)。"""
    import contextlib
    import io
    mod = _self_module()
    saved_argv = sys.argv
    sys.argv = [SELF_PATH, "--repo", repo]
    out, err = io.StringIO(), io.StringIO()
    try:
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = mod.main()
    finally:
        sys.argv = saved_argv
    return rc, out.getvalue() + err.getvalue()


def _with_swapped(name, value, repo):
    """把判据真源 `name` 临时换成 `value`、在夹具上跑一次 ⇒ 跑完**必复原**（防空绿档专用）。"""
    mod = _self_module()
    saved = getattr(mod, name)
    setattr(mod, name, value)
    try:
        return _run_inproc(repo)
    finally:
        setattr(mod, name, saved)


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


def self_test():
    """成对正负控自检：判红 / 放绿 / 退码钳 / 三格防空绿 / 真目标结构（仓外夹具 · 不碰真目标）。"""
    d = tempfile.mkdtemp(prefix="chs-selftest-")
    _TMPDIRS.append(d)
    bad = 0
    total = 0

    def case(tag, desc, ok, detail):
        nonlocal bad, total
        total += 1
        bad += 0 if ok else 1
        print("  %s %s %s\n       %s" % ("✓" if ok else "✗", tag, desc, detail))

    print("── check-history-secrets.py 自检（成对正负控 · 防空绿档 · 仓外夹具 · 不碰真目标）──")
    print("[负控] 白名单内的件里放真形状敏感文本（判据 `BLOCK`）⇒ 必判红 rc=1 且逐档点名")
    print("       ★ 两条判据摆**两个件**：一个 blob 只出一个 verdict（首个命中的模式）")
    dirty = _mk_fixture_repo(d, "dirty", {"src/app.py": _FIX_DIRTY, "src/priv.py": _FIX_PEM_FILE})
    rc, text = _run_cli(dirty)
    case("①负控", "白名单内两件（AWS 形 / 私钥块）⇒ rc=1 且两档都点名",
         rc == 1 and "AWS Access Key" in text and "私钥块" in text and "阻断级命中" in text,
         "rc=%d（期望 1）· 命中文里两档都在：%s"
         % (rc, "是" if ("AWS Access Key" in text and "私钥块" in text) else "否"))

    print("[负控] 提交信息本身也会被镜像 ⇒ 一起审（`<commit-message>`）")
    repo_msg = _mk_fixture_repo(d, "commit-msg", {"src/app.py": _FIX_GOOD},
                                msg="夹具：令牌 " + _FIX_SLACK)
    rc, text = _run_cli(repo_msg)
    case("②负控", "提交信息含 Slack 令牌形 ⇒ rc=1 且点名 `<commit-message>`",
         rc == 1 and "Slack 令牌" in text and "<commit-message>" in text, "rc=%d（期望 1）" % rc)

    print("[正控] 与 ② 成对：同一棵树、只把提交信息里那一形去掉 ⇒ 回绿 rc=0")
    repo_msg_ok = _mk_fixture_repo(d, "commit-msg-ok", {"src/app.py": _FIX_GOOD},
                                   msg="夹具：干净的提交信息")
    rc, text = _run_cli(repo_msg_ok)
    case("③正控", "复原（提交信息无令牌形）⇒ rc=0 且输出「结果: 通过」",
         rc == 0 and "结果: 通过" in text, "rc=%d（期望 0）" % rc)

    print("[正控] 干净夹具仓 ⇒ rc=0（不误报）")
    clean = _mk_fixture_repo(d, "clean", {"src/app.py": _FIX_GOOD})
    rc, text = _run_cli(clean)
    case("④正控", "干净件 ⇒ rc=0 且输出「结果: 通过」", rc == 0 and "结果: 通过" in text,
         "rc=%d（期望 0）" % rc)

    print("[区分度负控] 同一段文本放进**白名单外**的件 ⇒ 必放绿（只审真会被公开的路径）")
    outside = _mk_fixture_repo(d, "outside", {"notes/secret.md": _FIX_DIRTY})
    rc, text = _run_cli(outside)
    case("⑤负控", "白名单外同一段文本 ⇒ rc=0（白名单判据不许被绕过）",
         rc == 0 and "阻断级命中" not in text, "rc=%d（期望 0）" % rc)

    print("[正控] 白名单内但命中噪声后缀（`.bak`）⇒ 不审；且**空转读数必须逐字可见**（不许静默）")
    noise = _mk_fixture_repo(d, "noise", {"src/app.py.bak": _FIX_DIRTY})
    rc, text = _run_cli(noise)
    case("⑥正控", "噪声后缀件不审 ⇒ rc=0，且打印「文件版本 0 个」（空转可见）",
         rc == 0 and "文件版本 0 个" in text,
         "rc=%d · 空转读数 %s" % (rc, "有" if "文件版本 0 个" in text else "无"))

    print("[退码钳] `--repo` 不是 git 仓 ⇒ rc=2（判不了 ≠ 绿）")
    nogit = tempfile.mkdtemp(prefix="chs-nogit-", dir=d)
    _TMPDIRS.append(nogit)
    _write(os.path.join(nogit, "publish", "whitelist.txt"), _FIX_WL)
    rc, text = _run_cli(nogit)
    case("⑦退码钳", "非 git 仓 ⇒ rc=2 且逐字说明「不是 git 仓库」",
         rc == 2 and "不是 git 仓库" in text, "rc=%d（期望 2）" % rc)

    print("[正控] 替换规则**先变换、再扫描**：白名单内的家目录形会被改写成 `~` ⇒ 不判红")
    home = _mk_fixture_repo(d, "home", {"src/app.py": _FIX_HOME_FILE})
    rc, text = _run_cli(home)
    case("⑧正控", "规则会改写的形状 ⇒ rc=0（先变换再扫描 —— 否则第一版误报的硬伤会回来）",
         rc == 0, "rc=%d（期望 0）" % rc)

    print("[防空绿档] 把判据真源弄哑 / 换成恒真 ⇒ 读数必须跟着变（不变 = 那条真源本来就是死的）")
    rc_mute, _ = _with_swapped("BLOCK", [], dirty)
    case("⑨防空绿", "清空 `BLOCK` 模式表 ⇒ ① 那棵**必红**的树退回 rc=0（判红侧不是恒红）",
         rc_mute == 0, "rc=%d（期望 0）" % rc_mute)

    rc_const, _ = _with_swapped("published_path", lambda rel, entries: True, outside)
    case("⑩防空绿", "`published_path` 换成恒真 ⇒ ⑤ 那棵**放绿**的树变红（放行侧不是恒绿）",
         rc_const == 1, "rc=%d（期望 1）" % rc_const)

    rc_norule, _ = _with_swapped("load_rules", lambda *a, **k: ([], [], []), home)
    case("⑪防空绿", "`load_rules` 换成空规则 ⇒ ⑧ 那棵放绿的树变红（「先变换再扫描」是活的）",
         rc_norule == 1, "rc=%d（期望 1）" % rc_norule)

    # ── 缺口语料：**非 ASCII（中文）名件**（先例：同目录 `check-public-tree-hazards.py` 的
    #    `cjk-name-clean-from-list` + `cjk-name-quotepath-is-real` 那一对 ·
    #    `check-publish-face-sync.py` 的 `marks-no-nonascii-file-sent`）─────────────────
    # 判据真源还有**一处**：`main()` 里取清单那条 `git ls-tree -r <sha>` 上关掉 quotePath 的
    # 那个 `-c` 开关 **必须**在 —— git 默认 quotePath 是开的，会把非 ASCII 名转义成
    # `"\350\257\264…"` 这种**八进制带引号**形态 ⇒ `published_path()` 的前缀比对失配 ⇒
    # 该件压根不进候选集 ⇒ **静默漏检**（历史上真含敏感内容的中文名件被判绿）。
    # ★ 修前这一格是**假绿**：夹具里那枚中文名件带真形状敏感文本、又在白名单内，本该 rc=1，
    #   却因取清单被转义而 rc=0 —— 谁把那个 `-c` 开关拿掉，此格必红。
    print("[判红] 白名单内**中文名件**带真形状敏感文本 ⇒ rc=1 且**原样**点名（不是转义形态）")
    cjk_rel = "src/" + "中文名件.py"
    cjk_repo = _mk_fixture_repo(d, "cjk", {cjk_rel: _FIX_DIRTY})
    rc, text = _run_cli(cjk_repo)
    case("⑬中文名件", "取清单能**原样拿到真名** ⇒ 该件被扫到、判红、逐字点名",
         rc == 1 and cjk_rel in text and ('"src/' not in text and not re.search(r"\\[0-7]{3}", text)),
         "rc=%d（期望 1）· 原样点名 %s · 无八进制转义 %s"
         % (rc, "是" if cjk_rel in text else "否",
            "否（出了 \\350 这类形态）" if re.search(r"\\[0-7]{3}", text) else "是"))

    print("[牙齿] 同仓同件：默认 quotePath 下 git **确实**把它转义出去（⇒ 丢那个 `-c` 后 ⑬ 必红）")
    naive = sh(["git", "-c", "core.quotepath=true", "ls-tree", "-r", "HEAD", "--name-only"],
               cwd=cjk_repo)
    naive_esc = bool(re.search(r"\\[0-7]{3}", naive)) and ('"src/' in naive)
    case("⑭牙齿", "默认 quotePath 下列表里是转义形态、原样名不在（不是恒绿摆设）",
         naive_esc and cjk_rel not in naive,
         "转义形态 %s · 原样名 %s（期望：有转义 / 无原样）"
         % ("在" if naive_esc else "不在", "在" if cjk_rel in naive else "不在"))

    print("[真目标] 现读真仓**结构**（**不跑全历史** —— 实测 3m55s，见文件头）⇒ 结构断言 ＋ 计数照实报")
    revs = len(sh(["git", "rev-list", "--all"]).split())
    ents = len(load_whitelist())
    lits, regexes, isolated = load_rules()
    ex = len(load_excludes())
    ok_real = (os.path.isdir(os.path.join(REPO, ".git")) and revs > 0 and ents > 0
               and (len(lits) + len(regexes)) > 0 and ex > 0)
    case("⑫真目标", "真仓：git 仓成立 · 提交数 > 0 · 白名单 / 规则 / 排除项条数 > 0", ok_real,
         "%s ⇒ 提交 %d 笔 · 白名单 %d 条 · 规则 %d 条（字面 %d + 正则 %d）· 排除项 %d 条"
         % (REPO, revs, ents, len(lits) + len(regexes), len(lits), len(regexes), ex))
    print("  ⚠ 照实记（只报 · 不当绿）：本件真跑现读 **rc=1**（全历史有阻断级命中 · 4 分钟档）——")
    print("     它的两个挂点是提交闸 `tri-report`（只报告 · 不计退码）与 `real-gates.sh --full`；")
    print("     本批只补自检面，判据与真跑退码一字未改。")

    print("  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）" % (total, WANT_CASES))
    if total != WANT_CASES:
        print("✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2（不许当绿）"
              % (WANT_CASES, total), file=sys.stderr)
        return 2
    print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
    return 0 if bad == 0 else 1


if __name__ == "__main__":
    sys.exit(main())

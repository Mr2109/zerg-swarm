#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gate-coverage.py —— 门覆盖自检（治「假绿」本体的那一层）

一句话：**断言「该被门收进来的东西，真的被某一步收进来了」** —— 门禁守的是别人，
这一层守的是**门自己**（有没有漏掉一整个目录 / 一整个 module / 一整类脚本）。

为什么存在（《债务台账-20260918》§0 一整节 11 行 + §7 建议门①）
  ① **新建的构建树可以静默不在任何门里**：`ui/`（Rust crate · 27G 构建产物 · 16 处
     `#[cfg(test)]`）与 `shared/`、`scripts/exportnames/` 两个 Go module **今天就不在**
     `scripts/precommit-gates.sh` 的任何一步里 —— 而 `precommit-gates.sh` 的 `precheck`
     只验 `ui/Cargo.toml` **在不在**、不验它**过不过** ⇒ **ui 可以编译失败而闸全绿**。
  ② **新建的脚本可以静默不接线**：`pub` scope 只查 `scripts/*.sh|*.py` 的**语法**，
     语法过 ≠ 接了门；一只刚写完的门脚本**不写进 add_step 就永远不会有人跑它**，而闸全绿。
  ③ **「没有门」这件事本身没有任何门在管**：全部判定都靠「脚本里手写的 add_step 步骤表」，
     没有一处能发现「一个新目录 / 一个新 module 没被任何 add_step 收进来」。
  ⇒ 本脚本就是那一处。**它是唯一一个「扫步骤表」的门**。

两条断言（A 阻断 · B 阻断但带基线棘轮）

  A. **构建清单目录必须被某步收进**
     扫全仓含 `go.mod` / `Cargo.toml` 的目录（排除 `.git`/`vendor`/`node_modules`/`target`/
     构建产物/venv…，名单见 config `exclude`），逐个问「有没有哪一步把这个目录收进来了」。
     命中口径（三条，任一条成立即算收进；逐条给「是哪一步」作证）：
       A-a **工作目录命中**：某步的工作目录 == 该构建目录
            例：`add_step go … rc "${REPO_ROOT}/core" "go build ./..."`
       A-b **命令串路径参数命中**：命令串里出现该目录（相对仓根或相对工作目录），
            按路径边界匹配（`shared` / `./shared` / `cd shared` / `shared/...` 都算；
            `ui` 不许被 `build`、`ruin` 之类的子串蹭到）
       A-c **通配命中**：命令串里的 shell 通配 token 经 fnmatch 后覆盖该目录
     ★ **口径写死一条**：上级目录的 `./...` **不收**下级嵌套 module（Go 的多 module 语义
       就是这样：`./...` 不跨 `go.mod` 边界）—— 所以「core 被收进」不能证明「shared 被收进」。
     ★ 今天应真红的三条（**不许填白名单洗干净**）：`ui/` · `shared/` · `scripts/exportnames/`。

  B. **scripts 脚本必须要么在步骤表、要么在白名单**（**带基线棘轮**）
     扫 `scripts/*.sh` · `scripts/*.py` · `scripts/` 下无后缀可执行件，逐个问「这个脚本
     在 `precommit-gates.sh` 里被点名了吗」，分三档（**三档之和恒等于候选总数**，计数自证）：
       ① **add_step 命令串里点名**（真被某步跑）—— 最强
       ② **闸文件里被提到、但不在任何 add_step 命令串里**（注释里点名 / 尾部软门禁调用位 /
          总控脚本自身）—— **弱挂载：形同「写在注释里的门」**，单列出来让人看见
       ③ **闸文件里查无此名** —— **棘轮命中数**，与 config `baseline` 比
     ★ **通用语法步的 glob 兜底不计入 ①②③**：`scripts/*.sh 语法（bash -n）` 与
       `scripts/*.py 语法（ast.parse）` 用通配覆盖全部 82 条 —— **语法过 ≠ 接了门**，
       把 glob 当「在表里」就是本门要治的那个假绿。glob 命中单独打印。
     ★ **一次红 71 条会把提交闸锁死** ⇒ 起步用**基线棘轮**（同 D2「只报告」的先例）：
       `命中数 > baseline ⇒ rc=1`；`≤ baseline ⇒ rc=0`，但**必须打印剩余数**（不许悄悄绿）。

例外表（四档 · 逐档计数可见 · `--list-rules` 全列）
  | 档 | 名 | 进不进退码 | 口径 |
  |---|---|---|---|
  | 一 | **硬排除**（config `exclude`） | 不进 | 结构级：`.git`/`vendor`/`node_modules`/`target`/
  |   |   |   | 构建产物/venv… —— 它们**不在候选集里**，不是「放过」 |
  | 二 | **白名单**（config `whitelist`） | 不进（**计数可见**） | 逐条给 `reason` + `date`；
  |   |   |   | **缺 reason 或缺 date ⇒ rc=2**（不是红：写不清理由 = 判不了） |
  | 三 | **基线棘轮**（config `baseline`） | 不超基线不进 | 存量债的**已登记**额度；超一条即红；
  |   |   |   | **只许下调，永不上调**（上调 = 放宽口径 ⇒ 必须走拍板） |
  | 四 | **通用语法 glob 兜底** | 不进 | 只证明「语法过了」，**不计入 B 的 ①②③**；单列 |
  ★ 悬空白名单（写了 reason/date 但盘上已无此件）⇒ **照报，不改退码**（豁免面漂移看得见）。

基线棘轮怎么下调（**号令字句**，每批一次，照抄即可）
  「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」：〈逐条脚本名〉。
    依据 = `python3 scripts/check-gate-coverage.py --baseline` 的 **③ 实测命中数**（改前/改后各跑一次）；
    动作 = 把 `scripts/gate-coverage.config.json` 的 `baseline` 由 〈旧〉 改成 〈新〉（**只改这一个数字**）；
    规矩 = **一次只许下调、永不上调**；若实测值比基线**高**（= 又冒出新脚本）⇒ 先接线或补白名单理由，
    **不许**把基线抬上去。」
  `--baseline` 会把这段与「今日实测命中数 / 当前基线 / 剩余 / 建议值」一起打印出来。

与既有门的接口（**本波不挂**；第二波由父代理统一挂进 `scripts/precommit-gates.sh`）
  本门退码口径 = 仓内三档 **0/1/2**（`2` = 缺件 / 不可判 / 空转），⇒ 挂法用 **`tri`** 模式：
      add_step cov "门覆盖自检（构建清单目录 A + 脚本接线 B）" tri "${REPO_ROOT}" \\
               "python3 scripts/check-gate-coverage.py"
  ★ 全程**不接管道**（不写 `| tee` / `| tail`），rc 由 `_judge` 按真退出码判（本门 rc=2 落 BLOCKED）。
  ★ 本门**自带 `--self-test`**（成对负控），门禁别传 `--no-self-test`：先自证「会红」再扫真目标。
  ★ 挂进来之后，`check-gate-coverage.py` **自身**就进了①档 ⇒ ③ 档命中数会由 72 回落到 71，
    届时按上面的号令把 `baseline` 由 72 下调成 71。

用法
    python3 scripts/check-gate-coverage.py                  # 自检 + 两条断言扫真目标
    python3 scripts/check-gate-coverage.py --json           # 机读
    python3 scripts/check-gate-coverage.py --list-rules     # 两条断言 + 例外表四档 + 退码口径
    python3 scripts/check-gate-coverage.py --baseline       # 基线视图（命中/基线/剩余 + 下调号令）
    python3 scripts/check-gate-coverage.py --self-test      # 只跑自检（合成夹具，不碰真目标）
    python3 scripts/check-gate-coverage.py --no-self-test    # 内部子进程 / 迭代调试用（门禁别用）
    python3 scripts/check-gate-coverage.py --root <目录> --config <文件>
    python3 scripts/check-gate-coverage.py --max-examples N  # 每桶最多列几条（默认 40）

退出码（三档 · **优先级 2 > 1 > 0**，与 check-doc-meta/name/freshness 同口径）
    0 = 全绿（A 无未收进目录；B 命中数 ≤ 基线）
    1 = **有红**（A 有目录没被任何步收进 且 不在白名单；或 B 命中数 > 基线）
    2 = **不给结论**（用法错 / 缺件：config 或闸文件不在 / config 有未实现键 / 白名单条目缺 reason
        或 date / **空转**：候选集为 0 ⇒ 假覆盖 / 计数自证不过 / 自检未过）
    ★ 有「判不了」的项时**不给结论**，即便同时有红 —— 判不了的时候报绿报红都是猜。

纪律：**只读**（除 /tmp 夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件。
"""
import argparse
import atexit
import fnmatch
import glob
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

PY = sys.executable or "python3"
HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(HERE)
LOCAL_CONFIG = os.path.join("scripts", "gate-coverage.config.json")
GATES_REL = os.path.join("scripts", "precommit-gates.sh")

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

DEFAULT_BUILD_MANIFESTS = ("go.mod", "Cargo.toml")
DEFAULT_SCRIPT_GLOBS = ("scripts/*.sh", "scripts/*.py")
DEFAULT_NOSUFFIX_DIR = "scripts"
# ── 硬排除（第一档）：结构级，不是「放过」——它们压根不进候选集 ──────────────
DEFAULT_EXCLUDE_COMPONENTS = (
    ".git", "vendor", "node_modules", "target", "__pycache__", ".history", ".obsidian",
    "dist", "bin", "data", "zerg-wt", ".zerg", "venv", ".cargo", ".venv", "htmlcov",
)
DEFAULT_EXCLUDE_PREFIXES = ("tools/ocr/venv",)

# ── config 键白名单：出现未实现的键 ⇒ rc=2（否则 config 加了规则、门照绿 = 假覆盖）──
ALLOWED_TOP = {"schema", "_comment", "_baseline_note", "baseline", "scan", "exclude", "whitelist"}
ALLOWED_SCAN = {"build_manifests", "script_globs", "nosuffix_dir"}
ALLOWED_EXCLUDE = {"path_components", "prefixes"}
ALLOWED_WL = {"build_dirs", "scripts"}
ALLOWED_WL_ITEM = {"path", "reason", "date"}

DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


class Unsupported(Exception):
    """不可判定 ⇒ 一律 rc=2（不给结论），绝不猜。"""


# ══════════════════════════════════════════════════════════════════
# 0. 步骤表提取（**唯一来源 = precommit-gates.sh 自己**；不在这里抄第二份步骤表）
# ══════════════════════════════════════════════════════════════════
# 为什么这么取：命令串里有 heredoc / 嵌套 $( ) / 中文，文本正则解析会漏会错；
#   `--emit-cmd` 只给命令串、不给工作目录（A-a 要用），所以走 **source + EXIT trap**：
#   在闸脚本自己的 shell 里跑一遍 `main --list`（不跑任何真步骤），退出时把五个 indexed
#   array 原样吐出来。★ 闸脚本尾部是 `exit "${MAIN_RC}"` ⇒ 用 EXIT trap 抢在退出瞬间 dump。
#   ★ fd 3 在 source 的 stdout 重定向之前就复制好，免得 dump 被 `>/dev/null` 吃掉。
_EXTRACT_HELPER = r'''#!/usr/bin/env bash
set -u
cd "$1" || exit 9
exec 3>&1
dump() {
  local i=0
  while [ "$i" -lt "${#STEP_NAME[@]}" ]; do
    printf '%s\t%s\t%s\t%s\t%s\n' \
      "${STEP_SCOPE[$i]}" "${STEP_NAME[$i]}" "${STEP_MODE[$i]}" "${STEP_DIR[$i]}" "${STEP_CMD[$i]}" >&3
    i=$((i+1))
  done
}
trap dump EXIT
source scripts/precommit-gates.sh --list >/dev/null 2>&1
'''


def extract_steps(root, tmpdir):
    """⇒ (steps, err)。steps = [(scope, name, mode, dir, cmd)]；err 非空 ⇒ rc=2。"""
    gates = os.path.join(root, GATES_REL)
    if not os.path.isfile(gates):
        return [], "闸文件不在：%s（缺件 ⇒ 不给结论）" % GATES_REL
    helper = os.path.join(tmpdir, "gc-extract.sh")
    with open(helper, "w", encoding="utf-8") as fh:
        fh.write(_EXTRACT_HELPER)
    try:
        proc = subprocess.run(["/bin/bash", helper, root], stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, timeout=600)
    except Exception as exc:  # noqa: BLE001 —— 跑不起来 = 判不了
        return [], "步骤表提取失败（%s）" % exc
    out = proc.stdout.decode("utf-8", "replace")
    err = proc.stderr.decode("utf-8", "replace")
    if proc.returncode != 0:
        tail = err.strip().splitlines()[-1] if err.strip() else "（stderr 为空）"
        return [], ("闸脚本 `--list` 非零退出（rc=%d）⇒ **不给结论**（判不了 = 不报绿也不报红）。"
                    "常见原因：闸自己的自检不过、或 precheck 缺件（precheck 要 "
                    "core/go.mod · agent/go.mod · shared/go.mod · scripts/exportnames/go.mod · "
                    "wall/Cargo.toml · ui/Cargo.toml · publish/whitelist.txt 七件）、或仓不完整；stderr 末行：%s"
                    % (proc.returncode, tail))
    steps = []
    for line in out.split("\n"):
        if line.count("\t") < 4:
            continue
        parts = line.split("\t", 4)
        if parts[1].startswith("自检-"):  # 合成步骤（闸自己的负控）不算真目标
            continue
        steps.append(tuple(parts))
    if not steps:
        return [], "步骤表为空（0 步）⇒ 空转，不给结论（若闸脚本被重构，请同步本脚本的提取器）"
    return steps, ""


# ══════════════════════════════════════════════════════════════════
# 1. config
# ══════════════════════════════════════════════════════════════════
def load_config(path):
    if not os.path.isfile(path):
        raise Unsupported("config 不在：%s（缺件 ⇒ 不给结论；本门没有内建默认值，"
                          "因为「基线/白名单」必须显式落盘才可审计）" % path)
    try:
        with open(path, encoding="utf-8") as fh:
            cfg = json.load(fh)
    except Exception as exc:  # noqa: BLE001
        raise Unsupported("config 不是合法 JSON：%s（%s）" % (path, exc))
    if not isinstance(cfg, dict):
        raise Unsupported("config 顶层必须是对象")
    bad = sorted(k for k in cfg if not k.startswith("_") and k not in ALLOWED_TOP)
    if bad:
        raise Unsupported("config 有本脚本未实现的顶层键：%s ⇒ 不给结论（schema 加了键、"
                          "门照绿 = 假覆盖）" % " · ".join(bad))
    scan = cfg.get("scan", {})
    if not isinstance(scan, dict):
        raise Unsupported("config.scan 必须是对象")
    bad = sorted(k for k in scan if not k.startswith("_") and k not in ALLOWED_SCAN)
    if bad:
        raise Unsupported("config.scan 有未实现的键：%s" % " · ".join(bad))
    excl = cfg.get("exclude", {})
    if not isinstance(excl, dict):
        raise Unsupported("config.exclude 必须是对象")
    bad = sorted(k for k in excl if not k.startswith("_") and k not in ALLOWED_EXCLUDE)
    if bad:
        raise Unsupported("config.exclude 有未实现的键：%s" % " · ".join(bad))
    wl = cfg.get("whitelist", {})
    if not isinstance(wl, dict):
        raise Unsupported("config.whitelist 必须是对象")
    bad = sorted(k for k in wl if not k.startswith("_") and k not in ALLOWED_WL)
    if bad:
        raise Unsupported("config.whitelist 有未实现的键：%s" % " · ".join(bad))
    for bucket in ("build_dirs", "scripts"):
        items = wl.get(bucket, [])
        if not isinstance(items, list):
            raise Unsupported("config.whitelist.%s 必须是数组" % bucket)
        for i, it in enumerate(items):
            tag = "whitelist.%s[%d]" % (bucket, i)
            if not isinstance(it, dict):
                raise Unsupported("%s 必须是对象" % tag)
            bad = sorted(k for k in it if not k.startswith("_") and k not in ALLOWED_WL_ITEM)
            if bad:
                raise Unsupported("%s 有未实现的键：%s" % (tag, " · ".join(bad)))
            p = it.get("path")
            if not isinstance(p, str) or not p.strip():
                raise Unsupported("%s 缺 path" % tag)
            if not isinstance(it.get("reason"), str) or not it["reason"].strip():
                raise Unsupported("%s（%s）**缺 reason** ⇒ 不给结论（写不清理由的豁免 = 洗白）"
                                  % (tag, p))
            d = it.get("date")
            if not isinstance(d, str) or not DATE_RE.match(d):
                raise Unsupported("%s（%s）**缺 date 或格式不是 YYYY-MM-DD**（现 %r）⇒ 不给结论"
                                  % (tag, p, d))
    bl = cfg.get("baseline")
    if not isinstance(bl, int) or isinstance(bl, bool) or bl < 0:
        raise Unsupported("config.baseline 必须是非负整数（现 %r）" % (bl,))
    return {
        "path": path,
        "baseline": bl,
        "baseline_note": cfg.get("_baseline_note", ""),
        "build_manifests": tuple(scan.get("build_manifests", DEFAULT_BUILD_MANIFESTS)),
        "script_globs": tuple(scan.get("script_globs", DEFAULT_SCRIPT_GLOBS)),
        "nosuffix_dir": scan.get("nosuffix_dir", DEFAULT_NOSUFFIX_DIR),
        "exclude_components": tuple(excl.get("path_components", DEFAULT_EXCLUDE_COMPONENTS)),
        "exclude_prefixes": tuple(excl.get("prefixes", DEFAULT_EXCLUDE_PREFIXES)),
        "wl_build": list(wl.get("build_dirs", [])),
        "wl_scripts": list(wl.get("scripts", [])),
    }


# ══════════════════════════════════════════════════════════════════
# 2. 扫描（A 构建清单目录 · B scripts 候选）
# ══════════════════════════════════════════════════════════════════
def _rel(root, p):
    return os.path.relpath(p, root).replace(os.sep, "/")


def scan_build_dirs(root, cfg):
    """⇒ [(rel_dir, [manifest…])]，已按硬排除剪枝。"""
    excl_c = set(cfg["exclude_components"])
    excl_p = [p.strip("/") for p in cfg["exclude_prefixes"]]
    out = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        rel = _rel(root, dirpath)
        rel = "" if rel == "." else rel
        kept = []
        for d in sorted(dirnames):
            if d in excl_c:
                continue
            child = ("%s/%s" % (rel, d)).lstrip("/")
            if any(child == p or child.startswith(p + "/") for p in excl_p):
                continue
            kept.append(d)
        dirnames[:] = kept
        found = sorted(m for m in cfg["build_manifests"] if m in filenames)
        if found:
            out.append((rel, found))
    return sorted(out, key=lambda t: t[0])


def scan_scripts(root, cfg):
    """⇒ (cands, nosuffix)。cands = [(rel_path, kind)]；kind ∈ {sh, py, nosuffix}。"""
    cands, seen = [], set()
    for pat in cfg["script_globs"]:
        for p in sorted(glob.glob(os.path.join(root, pat))):
            if not os.path.isfile(p):
                continue
            rel = _rel(root, p)
            if rel in seen:
                continue
            seen.add(rel)
            cands.append((rel, os.path.splitext(p)[1].lstrip(".") or "nosuffix"))
    ns_dir = os.path.join(root, cfg["nosuffix_dir"])
    nosuffix = []
    if os.path.isdir(ns_dir):
        for fn in sorted(os.listdir(ns_dir)):
            p = os.path.join(ns_dir, fn)
            if not os.path.isfile(p) or "." in fn:
                continue
            rel = _rel(root, p)
            if rel in seen:
                continue
            head = ""
            try:
                with open(p, "rb") as fh:
                    head = fh.readline(200).decode("utf-8", "replace")
            except OSError:
                pass
            if not (os.access(p, os.X_OK) or head.startswith("#!")):
                continue  # 无后缀但既不可执行也无 shebang ⇒ 不是脚本
            seen.add(rel)
            nosuffix.append(rel)
            cands.append((rel, "nosuffix"))
    return cands, nosuffix


# ══════════════════════════════════════════════════════════════════
# 3. 命中判定
# ══════════════════════════════════════════════════════════════════
_SPLIT = re.compile(r"[\s;|&()<>]+")


def tokens(cmd):
    out = []
    for t in _SPLIT.split(cmd):
        t = t.strip("\"'`")
        if t:
            out.append(t)
    return out


def _norm_ref(tok):
    """把命令串里的一个 token 归一成「目录引用」形态：去尾 /、去 ./、去尾 /..."""
    t = tok.strip("\"'`")
    t = re.sub(r"/\.\.\.$", "", t)
    t = re.sub(r"/\.$", "", t)
    t = t.rstrip("/")
    if t.startswith("./"):
        t = t[2:]
    return t


def dir_ref_in_cmd(rel_dir, cmd):
    """A-b / A-c：命令串里有没有点名这个目录（按路径边界；通配按 fnmatch）。"""
    for tok in tokens(cmd):
        if _norm_ref(tok) == rel_dir:
            return True
    for tok in tokens(cmd):
        if "*" in tok or "?" in tok:
            if (fnmatch.fnmatch(rel_dir, tok) or fnmatch.fnmatch(rel_dir + "/x", tok)
                    or fnmatch.fnmatch(rel_dir + "/x.go", tok)):
                return True
    return False


def abs_ref_in_cmd(abs_dir, cmd):
    """命令串里直接写了绝对路径（如 `cd /…/shared`）：按边界匹配绝对形态。"""
    i = cmd.find(abs_dir)
    while i != -1:
        after = cmd[i + len(abs_dir):i + len(abs_dir) + 1]
        if after == "" or after in "/ \t\"';|&":
            return True
        i = cmd.find(abs_dir, i + 1)
    return False


def judge_build_dir(root, rel_dir, steps):
    """⇒ (命中?, 证据串)。命中口径 A-a/A-b/A-c，逐条留证。"""
    abs_dir = os.path.normpath(os.path.join(root, rel_dir))
    for scope, name, _mode, sdir, cmd in steps:
        if os.path.normpath(sdir) == abs_dir:  # A-a 工作目录命中
            return True, "[%s] %s ⌁ 工作目录 = %s" % (scope, name, rel_dir)
    for scope, name, _mode, sdir, cmd in steps:
        if dir_ref_in_cmd(rel_dir, cmd) or abs_ref_in_cmd(abs_dir, cmd):  # A-b 路径参数
            return True, "[%s] %s ⌁ 命令串点名 %s" % (scope, name, rel_dir)
    return False, ""


_NAME_CACHE = {}


def name_re(name):
    r = _NAME_CACHE.get(name)
    if r is None:
        r = re.compile(r"(?<![\w.\-])" + re.escape(name) + r"(?![\w.\-])")
        _NAME_CACHE[name] = r
    return r


def script_tier(rel, gates_text, cmds_text):
    """① in_steps · ② mentioned · ③ unmentioned —— 三档之和恒等于候选总数（计数自证）。"""
    name = os.path.basename(rel)
    if name_re(name).search(cmds_text):
        return "in_steps"
    if name_re(name).search(gates_text):
        return "mentioned"
    return "unmentioned"


def glob_hit(rel, steps):
    """第四档：只被通用语法步的通配兜住（语法过 ≠ 接了门 ⇒ 不计入 ①②③）。"""
    for _scope, _name, _mode, _sdir, cmd in steps:
        for tok in tokens(cmd):
            if ("*" in tok or "?" in tok) and fnmatch.fnmatch(rel, tok):
                return True
    return False


def wl_match(rel, entries):
    """白名单命中：路径相等，或位于该路径之下（目录型条目）。"""
    for e in entries:
        p = e["path"].strip("/")
        if rel == p or rel.startswith(p + "/"):
            return e
    return None


# ══════════════════════════════════════════════════════════════════
# 4. 汇总 / 报告
# ══════════════════════════════════════════════════════════════════
def analyse(root, cfg, steps, gates_text, max_examples):
    cmds_text = "\n".join(s[4] for s in steps)

    # ── A ──────────────────────────────────────────────────────────
    a_items, a_red, a_wl, a_covered = [], [], [], []
    for rel_dir, charts in scan_build_dirs(root, cfg):
        hit, why = judge_build_dir(root, rel_dir, steps)
        wl = wl_match(rel_dir, cfg["wl_build"])
        item = {"dir": rel_dir, "manifests": charts, "covered": hit, "why": why,
                "whitelist": (wl["reason"] if wl else None), "whitelist_date": (wl["date"] if wl else None)}
        a_items.append(item)
        if hit:
            a_covered.append(item)
        elif wl:
            a_wl.append(item)
        else:
            a_red.append(item)

    # ── B ──────────────────────────────────────────────────────────
    cands, nosuffix = scan_scripts(root, cfg)
    b_items, b_red, b_wl = [], [], []
    buckets = {"in_steps": [], "mentioned": [], "unmentioned": []}
    glob_note = []
    for rel, kind in cands:
        tier = script_tier(rel, gates_text, cmds_text)
        wl = wl_match(rel, cfg["wl_scripts"])
        item = {"script": rel, "kind": kind, "tier": tier,
                "glob": glob_hit(rel, steps),
                "whitelist": (wl["reason"] if wl else None),
                "whitelist_date": (wl["date"] if wl else None)}
        b_items.append(item)
        buckets[tier].append(item)                      # 三档之和恒等于候选总数
        if tier == "unmentioned":
            if wl:
                b_wl.append(item)                       # 第二档白名单（不计入棘轮）
            else:
                b_red.append(item)                      # ③ 棘轮命中数 = 查无此名 且 不在白名单
        if item["glob"]:
            glob_note.append(item)

    # ── 例外面计数（第二档：白名单）────────────────────────────────
    stale = []
    for bucket, entries in (("build_dirs", cfg["wl_build"]), ("scripts", cfg["wl_scripts"])):
        for e in entries:
            p = e["path"].strip("/")
            if bucket == "build_dirs":
                exists = os.path.isfile(os.path.join(root, p, "go.mod")) or \
                         os.path.isfile(os.path.join(root, p, "Cargo.toml"))
            else:
                exists = os.path.isfile(os.path.join(root, p))
            if not exists:
                stale.append((bucket, p, e["reason"], e["date"]))

    rep = {
        "root": root, "config": cfg["path"], "steps": len(steps),
        "a": {"items": a_items, "covered": a_covered, "whitelisted": a_wl, "red": a_red},
        "b": {"cands": b_items, "counts": {k: len(v) for k, v in buckets.items()},
              "tier1": buckets["in_steps"], "tier2": buckets["mentioned"], "tier3": buckets["unmentioned"],
              "hit": len(b_red), "hit_items": b_red,
              "whitelisted": b_wl, "glob_only": glob_note, "nosuffix": nosuffix},
        "baseline": cfg["baseline"], "baseline_note": cfg["baseline_note"],
        "stale_whitelist": stale,
        "wl_counts": {"build_dirs": len(cfg["wl_build"]), "scripts": len(cfg["wl_scripts"])},
        "exclude_components": list(cfg["exclude_components"]),
        "exclude_prefixes": list(cfg["exclude_prefixes"]),
        "max_examples": max_examples,
    }
    rep["a"]["empty"] = len(a_items) == 0
    rep["b"]["empty"] = len(cands) == 0
    rep["certify"] = (len(a_covered) + len(a_wl) + len(a_red) == len(a_items)
                      and sum(rep["b"]["counts"].values()) == len(cands))
    return rep


def _list(items, key, max_examples, indent="     ", mark_wl=False):
    lines = []
    for it in items[:max_examples]:
        extra = ("  ← 白名单豁免 %s（%s）" % (it["whitelist"], it.get("whitelist_date", ""))
                 if mark_wl and it.get("whitelist") else "")
        lines.append("%s- %s%s" % (indent, it[key], extra))
    if len(items) > max_examples:
        lines.append("%s… 另 %d 条（--max-examples 调整）" % (indent, len(items) - max_examples))
    return lines


def print_report(rep):
    a, b = rep["a"], rep["b"]
    print("── 门覆盖自检 · 仓根 %s ──" % rep["root"])
    print("   步骤表来源：%s（%d 步，已剔除合成自检步）" % (GATES_REL, rep["steps"]))
    print("   config：%s（baseline=%d）" % (rep["config"], rep["baseline"]))
    print()

    # ── A ──
    print("== A 构建清单目录必须被某步收进（阻断）==")
    print("   候选（含 go.mod / Cargo.toml 的目录，已剪硬排除）：%d" % len(a["items"]))
    print("     ① 已被某步收进：%d" % len(a["covered"]))
    for ln in _list(a["covered"], "dir", min(rep["max_examples"], 12)):
        print(ln)
    print("     ② 白名单豁免（第二档 · 计数可见）：%d" % len(a["whitelisted"]))
    for ln in _list(a["whitelisted"], "dir", rep["max_examples"], mark_wl=True):
        print(ln)
    print("     ③ **未收进且不在白名单 = 红：%d**" % len(a["red"]))
    for ln in _list(a["red"], "dir", rep["max_examples"]):
        print(ln)
    if a["red"]:
        print("     ⇒ 处置：**真修**（给该目录补一步 add_step，工作目录指到它）"
              "或**白名单写清 reason+date**；不许为了让本门变绿而填白名单。")
    print()

    # ── B ──
    print("== B scripts 脚本必须在步骤表或白名单（阻断 · 基线棘轮）==")
    kinds = {}
    for it in b["cands"]:
        kinds[it["kind"]] = kinds.get(it["kind"], 0) + 1
    print("   候选总数：%d（%s）" % (len(b["cands"]),
                                   " · ".join("%s %d" % (k, v) for k, v in sorted(kinds.items()))))
    print("     ① add_step 命令串里点名：%d" % b["counts"]["in_steps"])
    for ln in _list(b["tier1"], "script", rep["max_examples"]):
        print(ln)
    print("     ② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：%d"
          % b["counts"]["mentioned"])
    for ln in _list(b["tier2"], "script", rep["max_examples"]):
        print(ln)
    print("     ③ 闸文件里查无此名：%d" % b["counts"]["unmentioned"])
    print("        · 其中白名单豁免（第二档 · 计数可见）：%d" % len(b["whitelisted"]))
    for ln in _list(b["whitelisted"], "script", rep["max_examples"], mark_wl=True):
        print(ln)
    print("        · 其中**棘轮命中数**（= ③ − 白名单，与 baseline 比）：%d" % b["hit"])
    rem = b["hit"] - rep["baseline"]
    if rem > 0:
        print("          ✗ **超过基线 %d 条**（命中 %d > 基线 %d）⇒ 红"
              % (rem, b["hit"], rep["baseline"]))
    else:
        print("          （命中 %d ≤ 基线 %d ⇒ 不红，**剩余 %d 条**；"
              "每批下调基线见 --baseline）" % (b["hit"], rep["baseline"], b["hit"]))
    for ln in _list(b["hit_items"], "script", rep["max_examples"]):
        print(ln)
    print("     注：另 %d 条被通用语法步的 glob 兜住（`scripts/*.sh|*.py` 语法）——"
          "**语法过 ≠ 接了门，不计入 ①②③**（第四档）" % len(b["glob_only"]))
    print()

    # ── 自证 / 例外面 ──
    print("== 计数自证 ==")
    ok = rep["certify"]
    print("   A：收进 %d + 白名单 %d + 红 %d = %d（候选 %d）%s"
          % (len(a["covered"]), len(a["whitelisted"]), len(a["red"]),
             len(a["covered"]) + len(a["whitelisted"]) + len(a["red"]), len(a["items"]),
             "✓" if len(a["covered"]) + len(a["whitelisted"]) + len(a["red"]) == len(a["items"]) else "✗"))
    print("   B：① %d + ② %d + ③ %d = %d（候选 %d）%s"
          % (b["counts"]["in_steps"], b["counts"]["mentioned"], b["counts"]["unmentioned"],
             sum(b["counts"].values()), len(b["cands"]),
             "✓" if sum(b["counts"].values()) == len(b["cands"]) else "✗"))
    if not ok:
        print("   ✗ 三档之和 ≠ 候选总数 ⇒ **计数自证不过** ⇒ 不给结论（rc=2）")
    if rep["stale_whitelist"]:
        print("   ⚠ 悬空白名单（写了 reason/date 但盘上查无此件，**不影响退码**）：%d 条"
              % len(rep["stale_whitelist"]))
        for bucket, p, reason, date in rep["stale_whitelist"][:rep["max_examples"]]:
            print("     - [%s] %s（%s）" % (bucket, p, date))
    print("   硬排除（第一档，不进候选）：%s（前缀：%s）"
          % (" · ".join(sorted(rep["exclude_components"])), " · ".join(rep["exclude_prefixes"])))
    print()


def print_baseline(rep):
    a, b = rep["a"], rep["b"]
    n = b["hit"]
    print("── 基线视图（只读 · 不改任何文件）──")
    print("   ③ 实测命中数（闸文件里查无此名的脚本数） = %d" % n)
    print("   config baseline                          = %d" % rep["baseline"])
    print("   剩余（命中数）                            = %d" % n)
    if n > rep["baseline"]:
        print("   ⇒ **超额 %d 条 ⇒ rc=1**；处置：接线（写进 add_step）或补白名单 reason+date，"
              "**不许把基线抬上去**" % (n - rep["baseline"]))
    else:
        print("   ⇒ 不超额 ⇒ rc=0（但剩余数必须打印，不许悄悄绿）")
    print("   建议值 = %d（= 今日实测；若已接线若干条 ⇒ 实测值会自己降到该数）" % n)
    if rep["baseline_note"]:
        print("   基线自述 = %s" % rep["baseline_note"])
    print()
    print("号令字句（每批下调基线时照抄，一次一批，**只许下调、永不上调**）：")
    print("  「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」："
          "〈逐条脚本名〉。")
    print("    依据 = `python3 scripts/check-gate-coverage.py --baseline` 的 ③ 实测命中数"
          "（改前/改后各跑一次）；")
    print("    动作 = 把 `scripts/gate-coverage.config.json` 的 `baseline` 由 %d 改成"
          " %d（**只改这一个数字**）；" % (rep["baseline"], n))
    print("    规矩 = 一次只许下调、永不上调；实测值比基线高（= 又冒出新脚本）⇒ 先接线或补白名单理由，"
          "不许抬基线。」")


def print_list_rules():
    print("check-gate-coverage.py 规则表（门覆盖自检）")
    print()
    print("A [FAIL·阻断] 构建清单目录必须被某步收进")
    print("   候选 = 全仓含 %s 的目录（剪掉第一档硬排除）" % " / ".join(DEFAULT_BUILD_MANIFESTS))
    print("   命中口径（三条，任一成立）：")
    print("     A-a 工作目录命中：某步的 add_step 工作目录 == 该构建目录")
    print("     A-b 命令串路径参数命中：命令串里点名该目录（按路径边界：shared / ./shared /")
    print("         cd shared / shared/... 都算；ui 不许被 build 之类的子串蹭到）")
    print("     A-c 通配命中：命令串里的 shell 通配 token 经 fnmatch 后覆盖该目录")
    print("   ★ 口径写死：上级目录的 ./... 不收下级嵌套 module（Go 的 ./... 不跨 go.mod 边界）")
    print("   ★ 今日应真红：ui/ · shared/ · scripts/exportnames/")
    print()
    print("B [FAIL·阻断·基线棘轮] scripts 脚本必须要么在步骤表、要么在白名单")
    print("   候选 = %s + %s/ 下无后缀可执行件（有执行位或 shebang）"
          % (" / ".join(DEFAULT_SCRIPT_GLOBS), DEFAULT_NOSUFFIX_DIR))
    print("   ① add_step 命令串里点名（真被某步跑） · ② 闸文件里被提到但不在任何 add_step 命令串里"
          "（弱挂载，单列） · ③ 闸文件查无此名 = 棘轮命中数")
    print("   ★ 计数自证：① + ② + ③ ≡ 候选总数")
    print("   ★ glob 兜底（scripts/*.sh|*.py 语法步）**不计入** ①②③ —— 语法过 ≠ 接了门")
    print("   ★ 命中数 > baseline ⇒ rc=1；≤ baseline ⇒ rc=0 但必须打印剩余数")
    print()
    print("例外表（四档）")
    print("   一 硬排除（config.exclude）   不进候选集（结构级，不是「放过」）")
    print("   二 白名单（config.whitelist） 逐条 reason + date；缺任一 ⇒ rc=2；计数可见")
    print("   三 基线棘轮（config.baseline）已登记存量额度；超一条即红；只许下调")
    print("   四 通用语法 glob 兜底        只证明语法过了；单列；不计入 B 的 ①②③")
    print()
    print("退码（三档 · 优先级 2 > 1 > 0）")
    print("   0 = 全绿（A 无红；B 命中 ≤ 基线）")
    print("   1 = 有红（A 有未收进目录 且 不在白名单；或 B 命中 > 基线）")
    print("   2 = 不给结论（用法错 / 缺件：config 或闸文件不在 / config 有未实现键 / 白名单条目")
    print("       缺 reason 或 date / 空转（候选集为 0 = 假覆盖）/ 计数自证不过 / 自检未过）")
    print()
    print("与既有门的接口（第二波挂接）：")
    print('   add_step cov "门覆盖自检（构建清单目录 A + 脚本接线 B）" tri "${REPO_ROOT}" \\')
    print('            "python3 scripts/check-gate-coverage.py"')


# ══════════════════════════════════════════════════════════════════
# 5. 自检（成对负控：漏一个目录必红 ↔ 补齐必绿 · 命中>基线必红 ↔ ≤基线必绿 ·
#          白名单缺 reason ⇒ rc=2 · 空转 ⇒ rc=2（绝不报绿））
# ══════════════════════════════════════════════════════════════════
SELF_PATH = os.path.abspath(__file__)
_TMPDIRS = []


def _mkroot(base):
    d = tempfile.mkdtemp(prefix=base, dir=base)
    _TMPDIRS.append(d)
    return d


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _fixture(base, name, *, steps=(), manifests=("core/go.mod",), scripts=("scripts/ok.py",),
             config=None, gates_extra="", no_gates=False, config_raw=None):
    """造一个最小仓：scripts/precommit-gates.sh（合成步骤表）+ 构建清单 + 脚本 + config。"""
    root = os.path.join(base, name)
    os.makedirs(root, exist_ok=True)
    for m in manifests:
        _write(os.path.join(root, m), "module fixture\n" if m.endswith("go.mod") else "[package]\n")
    for s in scripts:
        _write(os.path.join(root, s), "#!/usr/bin/env python3\nprint('fixture')\n")
    if not no_gates:
        lines = ["#!/usr/bin/env bash", "set -u",
                 "# fixture：本文件 scripts/precommit-gates.sh 是合成件"
                 "（仅供 check-gate-coverage.py 自检用）",
                 'REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"',
                 "STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=()",
                 'add_step() { STEP_SCOPE+=("$1"); STEP_NAME+=("$2"); STEP_MODE+=("$3");'
                 ' STEP_DIR+=("$4"); STEP_CMD+=("$5"); }',
                 "clear_steps() { STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=(); }",
                 "build_steps() {"]
        for d, cmd in steps:
            lines.append('  add_step fixture "步骤 %s" rc "${REPO_ROOT}/%s" "%s"' % (d or "root", d, cmd))
        lines += ["}", 'main() { clear_steps; build_steps; }', 'main "$@"', gates_extra]
        _write(os.path.join(root, "scripts", "precommit-gates.sh"), "\n".join(lines) + "\n")
    wl = {"build_dirs": [], "scripts": []}
    if config:
        wl = config
    if config_raw is not None:
        _write(os.path.join(root, "scripts", "gate-coverage.config.json"), config_raw)
    else:
        _write(os.path.join(root, "scripts", "gate-coverage.config.json"),
               json.dumps({"schema": "gate-coverage/1", "baseline": 0, "whitelist": wl},
                          ensure_ascii=False, indent=2))
    return root


def _run(root):
    proc = subprocess.run([PY, SELF_PATH, "--no-self-test", "--root", root],
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return proc.returncode, proc.stdout.decode("utf-8", "replace") + proc.stderr.decode("utf-8", "replace")


def self_test():
    base = tempfile.mkdtemp(prefix="gc-selftest-")
    _TMPDIRS.append(base)
    cases = []

    def check(desc, ok, extra=""):
        cases.append((desc, bool(ok), extra))

    # ── 好件：core 被某步收进 + ok.py 在步骤表里 + baseline 0 ⇒ 必绿 ─────────
    good = _fixture(base, "good", steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")])
    rc, out = _run(good)
    check("① 好件必绿（目录被收进 + 脚本在表里）⇒ rc=0", rc == 0, "rc=%d" % rc)

    # ── 成对负控 A：抽掉 core 那一步 ⇒ 必红且点名 core ─────────────────────
    miss = _fixture(base, "a-missing", steps=[("", "python3 scripts/ok.py")])
    rc, out = _run(miss)
    check("② A 漏掉一个目录 ⇒ rc=1", rc == 1, "rc=%d" % rc)
    check("② 且**点名**那个目录（core）", "core" in out and "未收进且不在白名单 = 红：1" in out)
    check("② 且提示不许填白名单洗干净", "不许为了让本门变绿而填白名单" in out)

    # ── 与 ② 成对：把 core 那一步补回来 ⇒ 必绿 ─────────────────────────────
    rc2, out2 = _run(good)
    hit2 = "③ 好件必绿 —— 与 ② 成对（补回一步即绿）"
    check(hit2 + "（rc=0）", rc2 == 0, "rc=%d" % rc2)
    check(hit2 + "（A 红数 0）", "未收进且不在白名单 = 红：0" in out2)

    # ── 成对负控 B：命中 ≤ 基线 ⇒ 绿且打印剩余；命中 > 基线 ⇒ 红 ────────────
    #    夹具：orphan.py 未点名 ⇒ 命中 1。baseline=1 ⇒ 绿；baseline=0 ⇒ 红。
    ok_wl = {"build_dirs": [], "scripts": [{"path": "scripts/ok.py", "reason": "夹具占位",
                                            "date": "2026-09-18"}]}
    rat_ok = _fixture(base, "b-ratchet-ok",
                      steps=[("core", "go build ./...")],
                      scripts=("scripts/ok.py", "scripts/orphan.py"),
                      config=ok_wl)
    _write(os.path.join(rat_ok, "scripts", "gate-coverage.config.json"),
           json.dumps({"schema": "gate-coverage/1", "baseline": 1, "whitelist": ok_wl},
                      ensure_ascii=False))
    rc, out = _run(rat_ok)
    check("④ B 命中(1) ≤ 基线(1) ⇒ rc=0", rc == 0, "rc=%d" % rc)
    check("④ 且**打印剩余数**（不许悄悄绿）", "剩余 1 条" in out)

    rat_bad = _fixture(base, "b-ratchet-exceeded",
                       steps=[("core", "go build ./...")],
                       scripts=("scripts/ok.py", "scripts/orphan.py"),
                       config=ok_wl)  # baseline=0
    rc, out = _run(rat_bad)
    check("⑤ B 命中(1) > 基线(0) ⇒ rc=1 —— 与 ④ 成对", rc == 1, "rc=%d" % rc)
    check("⑤ 且点名「超过基线 1 条」", "超过基线 1 条" in out)

    # ── 白名单缺 reason / 缺 date ⇒ rc=2（不是红：写不清理由 = 判不了）────────
    nor_eason = _fixture(base, "wl-no-reason",
                         steps=[("", "python3 scripts/ok.py")],
                         manifests=("core/go.mod",),
                         config={"build_dirs": [{"path": "core", "date": "2026-09-18"}],
                                 "scripts": []})
    rc, out = _run(nor_eason)
    check("⑥ 白名单缺 reason ⇒ rc=2（不是 rc=1）", rc == 2, "rc=%d" % rc)
    check("⑥ 且输出点名「缺 reason」", "缺 reason" in out)

    nodate = _fixture(base, "wl-no-date",
                      steps=[("", "python3 scripts/ok.py")],
                      config={"build_dirs": [{"path": "core", "reason": "夹具"}], "scripts": []})
    rc, out = _run(nodate)
    check("⑦ 白名单缺 date ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑦ 且输出点名「缺 date」", "缺 date" in out)

    # ── 空转 ⇒ rc=2（绝不报绿）──────────────────────────────────────────
    a_empty = _fixture(base, "a-empty", steps=[("", "python3 scripts/ok.py")], manifests=())
    rc, out = _run(a_empty)
    check("⑧ A 空转（0 个构建清单目录）⇒ rc=2，**绝不报绿**", rc == 2, "rc=%d" % rc)
    check("⑧ 且输出点名「空转」", "空转" in out)

    b_empty = _fixture(base, "b-empty", steps=[("core", "go build ./...")], scripts=(),
                       config_raw=json.dumps({"schema": "gate-coverage/1", "baseline": 0,
                                              "scan": {"script_globs": ["scripts/*.rb"],
                                                       "nosuffix_dir": "no-such-dir"},
                                              "whitelist": {}}, ensure_ascii=False))
    rc, out = _run(b_empty)
    check("⑨ B 空转（0 个脚本）⇒ rc=2，**绝不报绿**", rc == 2, "rc=%d" % rc)
    check("⑨ 且输出点名「空转」", "空转" in out)

    # ── 缺件 ⇒ rc=2 ─────────────────────────────────────────────────────
    no_cfg = _fixture(base, "no-config", steps=[("core", "go build ./...")],
                      config_raw="{}", )
    os.remove(os.path.join(no_cfg, "scripts", "gate-coverage.config.json"))
    rc, out = _run(no_cfg)
    check("⑩ config 缺件 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑩ 且输出点名「config 不在」", "config 不在" in out)

    no_gates = _fixture(base, "no-gates", manifests=("core/go.mod",), scripts=("scripts/ok.py",),
                        no_gates=True)
    rc, out = _run(no_gates)
    check("⑪ 闸文件缺件 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑪ 且输出点名「闸文件不在」", "闸文件不在" in out)

    # ── config 有未实现键 ⇒ rc=2（schema 加了规则、门照绿 = 假覆盖）─────────
    bad_key = _fixture(base, "cfg-bad-key", steps=[("core", "go build ./...")],
                       config_raw=json.dumps({"schema": "gate-coverage/1", "baseline": 0,
                                              "whitelist": {}, "new_rule": True}, ensure_ascii=False))
    rc, out = _run(bad_key)
    check("⑫ config 有未实现的键 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑫ 且输出点名那个键（new_rule）", "new_rule" in out)

    # ── 悬空白名单：照报、不改退码 ───────────────────────────────────────
    stale = _fixture(base, "wl-stale", steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                     config={"build_dirs": [{"path": "没了/这个目录", "reason": "夹具", "date": "2026-09-18"}],
                             "scripts": []})
    rc, out = _run(stale)
    check("⑬ 悬空白名单**不改退码**（rc=0）", rc == 0, "rc=%d" % rc)
    check("⑬ 且照报「悬空白名单」", "悬空白名单" in out)

    # ── 弱挂载档（②）真的分得出来：只在闸文件注释里提名的脚本不算 ① ─────────
    weak = _fixture(base, "weak",
                    steps=[("core", "go build ./...")],
                    scripts=("scripts/ok.py",),
                    config={"build_dirs": [], "scripts": [{"path": "scripts/ok.py", "reason": "夹具",
                                                           "date": "2026-09-18"}]},
                    gates_extra="# 这里只为「提到」scripts/ok.py，不接进任何一步")
    rc, out = _run(weak)
    check("⑭ 弱挂载档可分辨（② 单列，不并进 ①）", rc == 0 and "弱挂载" in out
          and "① add_step 命令串里点名：0" in out
          and "② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：2" in out
          and "② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：2" in out
          and "- scripts/ok.py\n" in out)

    # ── 计数自证行真的在 ────────────────────────────────────────────────
    check("⑮ 报告含「计数自证」两条恒等式", "计数自证" in out2 and "① 1 + ② 1 + ③ 0 = 2（候选 2）" in out2)

    print("── check-gate-coverage 自检（成对负控 · 真命令行 + 真退出码）──")
    bad = 0
    for desc, ok, extra in cases:
        print("   %s %s%s" % ("✓" if ok else "✗", desc, ("（%s）" % extra) if extra and not ok else ""))
        if not ok:
            bad += 1
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if bad == 0 else "**不过**", len(cases), bad))
    return bad == 0


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


atexit.register(cleanup)


# ══════════════════════════════════════════════════════════════════
# 6. main
# ══════════════════════════════════════════════════════════════════
def main(argv):
    ap = argparse.ArgumentParser(add_help=True, description="门覆盖自检（check-gate-coverage.py）")
    ap.add_argument("--root", default=REPO_ROOT_DEFAULT)
    ap.add_argument("--config", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--baseline", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    ap.add_argument("--max-examples", type=int, default=40)
    args = ap.parse_args(argv)

    if args.list_rules:
        print_list_rules()
        return EXIT_OK

    if args.self_test:
        print("── check-gate-coverage 自检（成对负控：漏一个目录必红 ↔ 补齐必绿；"
              "命中>基线必红 ↔ ≤基线必绿；白名单缺 reason/date ⇒ rc=2；空转 ⇒ rc=2）──")
        return EXIT_OK if self_test() else EXIT_BLOCKED

    if not args.no_self_test:
        print("── 自检（成对负控）──")
        if not self_test():
            print("自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）")
            return EXIT_BLOCKED
        print()

    root = os.path.abspath(args.root)
    cfg_path = args.config or os.path.join(root, LOCAL_CONFIG)
    try:
        cfg = load_config(cfg_path)
        if not os.path.isdir(root):
            raise Unsupported("仓根不存在：%s" % root)
        tmpdir = tempfile.mkdtemp(prefix="gc-steps-")
        _TMPDIRS.append(tmpdir)
        steps, err = extract_steps(root, tmpdir)
        if err:
            raise Unsupported(err)
        gates_text = open(os.path.join(root, GATES_REL), encoding="utf-8").read()
        rep = analyse(root, cfg, steps, gates_text, args.max_examples)
    except Unsupported as exc:
        print("✗ 不给结论（rc=2）：%s" % exc, file=sys.stderr)
        return EXIT_BLOCKED

    # ── 退出码（优先级 2 > 1 > 0）—— **先算结论，再输出**（--json 的 stdout 必须是纯 JSON）──
    a, b = rep["a"], rep["b"]
    n_red_a = len(a["red"])
    n_over_b = b["hit"] - rep["baseline"]
    if a["empty"] or b["empty"]:
        which = ("A（构建清单目录）" if a["empty"] else "") + ("B（scripts 脚本）" if b["empty"] else "")
        rep["verdict"] = {"rc": EXIT_BLOCKED, "code": "EMPTY_SCAN",
                          "reason": "空转：%s 候选集为 0 ⇒ 假覆盖（假绿的一种）⇒ 不给结论" % which}
        rep["rc"] = EXIT_BLOCKED
    elif not rep["certify"]:
        rep["verdict"] = {"rc": EXIT_BLOCKED, "code": "CERTIFY_FAILED",
                          "reason": "计数自证不过（三档之和 ≠ 候选总数）⇒ 不给结论"}
        rep["rc"] = EXIT_BLOCKED
    elif n_red_a or n_over_b > 0:
        rep["verdict"] = {"rc": EXIT_FAIL, "code": "RED",
                          "reason": "A 未收进 %d 条；B 棘轮命中 %d > 基线 %d（超 %d）"
                                    % (n_red_a, b["hit"], rep["baseline"], max(n_over_b, 0))}
        rep["rc"] = EXIT_FAIL
    else:
        rep["verdict"] = {"rc": EXIT_OK, "code": "OK",
                          "reason": "A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）"
                                    % (b["hit"], rep["baseline"], b["hit"])}
        rep["rc"] = EXIT_OK

    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2, sort_keys=True))
        print("rc=%d · %s" % (rep["rc"], rep["verdict"]["reason"]), file=sys.stderr)
    else:
        print_report(rep)
        if args.baseline:
            print_baseline(rep)
        print("== 结论 ==")
        print("   A 未收进：%d 条 · B 棘轮命中 %d / 基线 %d（超 %d）"
              % (n_red_a, b["hit"], rep["baseline"], max(n_over_b, 0)))
        if rep["rc"] == EXIT_BLOCKED:
            print("   ⇒ **不给结论（rc=2）**：%s" % rep["verdict"]["reason"], file=sys.stderr)
        elif rep["rc"] == EXIT_FAIL:
            print("   ⇒ 红（rc=1）：诊断见上；处置 = **真修**（补 add_step）或白名单写清 reason+date")
        else:
            print("   ⇒ 绿（rc=0）：A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）"
                  % (b["hit"], rep["baseline"], b["hit"]))
    return rep["rc"]


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

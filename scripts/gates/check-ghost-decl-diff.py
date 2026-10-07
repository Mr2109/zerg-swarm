#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-ghost-decl-diff.py v1.0.0 —— `D4`（`A7` · `Q-057`）幽灵/声明差**只读 diff**（不自动清 · 不自动补）

为什么存在（`D4` 的条件原文 · 逐字口径）
----------------------------------------
`Zerg-内部文档/项目文档/v2.5.11/缺口总账-20260922.md` §附录十五 A 表的 `D4` 行：
  条件 = 「**只做只读 diff、不自动清幽灵**」（与 `Q-057` 同批）。
`Q-057` 原文（本件的病根）：「看挂在系统上的守护件，以及**「现值有、声明无」的幽灵**」
  —— 今天靠人肉 `plutil -p ~/Library/LaunchAgents/com.zerg.core.plist | grep -A2 ProgramArguments`，
  而 `zerg core daemon ls` 的 `declared` 列**全「否」**（⇒ 看不出幽灵）。

本件把「**两侧**」变成每次都能现跑的一份只读 diff：
  · 侧①「**盘上有 / 声明无**」—— 现值进程里，不被任何 `kind=launchd` 声明的**进程特征**覆盖的条；
    每条**额外标注**它是「已登记 ghost 行」还是「**未登记**幽灵」（后者才是门⑧ R4 的红）。
  · 侧②「**声明有 / 盘上无**」—— 每条 `kind=launchd` 声明的**三环**（`launchctl` 装载 · `ps` 进程 ·
    `声明件`在盘）里**缺哪一环**，逐条点名。
  两侧**都逐条列**；某侧为空也要打印「（空）0 条」（**不许静默**）。

★ 同源（「**不许开第二份口径**」不是一句口号 · 是两条可核断言）
------------------------------------------------------------
  S1 **声明侧真源件 + 解析器**：本件 `import` 门⑧ `scripts/gates/check-service-declaration.py`
     的 `parse_decl` / `PROC_HINTS` / `read_lines`（`importlib` 按路径加载 ⇒ **同一份代码**，不是抄一遍），
     真源件 = `deploy/服务声明.tsv`（该件自己的表头逐字写着「本件是『现值 vs 声明』差集判定的**声明侧唯一真源**」），
     且与 `zerg core daemon ls` 读的**同一件**。加载失败 ⇒ **rc=2 不给结论**（不许退化成自带一份）。
  S2 **幽灵计数与 `T2` 的「两列」同源同值**：侧①条数必须**逐字等于** `zerg doctor` 的
     「回收候选（幽灵服务）」那一格的条数（现读 `zerg doctor --json name,verdict,detail,advice`，
     文本形态逐字：`1 条：pid=79317 …`）。不等 ⇒ **rc=1**（两处口径已经分叉 —— 那正是「第二份口径」的形态）。

判据（可机检 · 每条都能单独失败）
--------------------------------
  D1 两侧都出（各自有计数 + 逐条），缺声明件 / 解析不出 / ps 或 launchctl 取不到 / **空转** ⇒ rc=2。
  D2 **不带写旗标 ⇒ 仓内件 `sha256` 不变**：`--prove-readonly` 对拍「声明真源件 + 门⑧ 源件 + 本件自身
     + `git status --porcelain` 全文摘要 + `.git` 元数据面」逐件 `sha256` ⇒ 差异 0 件 ⇒ rc=0。
  D3 **负控 · 任何自动删 / 自动改 ⇒ 判红**，两道牙：
     ① **运行时**：任何写旗标（`--reap` / `--clean` / `--fix` / `--auto` / `--prune` / `--rm` / `--apply` /
        `--write` / `--delete` / `--stop` / `--kill` / `--unload` … 以及 `add`/`commit`/`push` 一族）⇒ **rc=2 拒**；
     ② **源码级**（`assert_no_write_api`）：扫本件**真目标路径那一段**（自检标记之前）——出现
        `os.remove` / `os.unlink` / `os.rmdir` / `shutil.rmtree` / `shutil.move` / `os.rename` /
        `open(…, 'w'|'a')` / `subprocess` 里的 git 写子命令 ⇒ **rc=1 判红**（自检喂合成源码正负成对）。
        ★ 自检段（合成夹具）豁免 —— 它只写 `$TMPDIR` 下的夹具，不碰真目标。

退码
----
  0 = 两侧已逐条出 + 同源断言过（**差集本身不驱动退码**：本件是只读 diff，报差就是它的活）
  1 = 同源漂移（S2 不等）/ 源码级写 API 命中 / 只读自证红
  2 = 不给结论（声明缺件·解析不出·ps/launchctl/doctor 取不到·空转·写旗标·自检不过）
  ★ 8 不出现：本件**不判「幽灵该不该杀」**（停 = 不可逆动作 ⇒ 那是 Mr2109 的拍板，照 §九 M9 口径）

用法
----
    python3 scripts/gates/check-ghost-decl-diff.py              # 真目标：现值（ps/launchctl）vs 声明（deploy/服务声明.tsv）
    python3 scripts/gates/check-ghost-decl-diff.py --json       # 机读（zerg/v1 形状）
    python3 scripts/gates/check-ghost-decl-diff.py --prove-readonly   # 只读自证（跑前后逐件 sha256）
    python3 scripts/gates/check-ghost-decl-diff.py --list-rules
    python3 scripts/gates/check-ghost-decl-diff.py --self-test  # 只跑自检（合成夹具 · 不碰真目标）
取证夹具（自检用；也便于在别的机器上复跑 · **与门⑧ 同名同义**）：
    ZERG_SVCDECL_DECL / ZERG_SVCDECL_PS / ZERG_SVCDECL_LAUNCHCTL / ZERG_SVCDECL_HOME + 本件的 --doctor

★ 接线（**不归本件** · 波④ 收口）：本件今天**没有**挂进 `scripts/gates/precommit-gates.sh`。
  波④ 需要的那一行（照 `scripts/公开标记.tsv` 的列：文件 / public / 理由）：
    scripts/gates/check-ghost-decl-diff.py	no	B 类·`D4` 幽灵/声明差只读 diff 件（`A7`·`Q-057`）：两侧逐条（盘上有·声明无 / 声明有·盘上无）+ 与 `zerg doctor` 幽灵条数同源同值 + 任何自动删/自动改判红；判据绑私有面（读 `deploy/服务声明.tsv` · `ps`/`launchctl` · `bin/zerg`）⇒ 不开源
  ★ 接线时**别把它挂成会写的东西**：本件退码 0/1/2 三档，挂 `tri` 即可（0=PASS · 1=FAIL · 2=BLOCKED）。
"""

import argparse
import ast
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-ghost-decl-diff.py v1.0.0"
SELF_NAME = os.path.basename(__file__)
SIBLING = os.path.join(HERE, "check-service-declaration.py")          # 门⑧ = 同源的那一份
DOCTOR_ITEM = "回收候选（幽灵服务）"                                     # `T2` 判据③ 的同一格（逐字）
DOCTOR_FIELDS = "name,verdict,detail,advice"
# ★ 拼接而成：**别把它写成一个字面量** —— 否则 `src.split(marker)[0]` 会在「常量定义那一行」就切掉，
#   后面真目标段的牙就形同虚设（自检用例 ④b/④c 一起把这一点钉住）。
SELFTEST_MARKER = "# ─────────────────── " + "自检"                   # 源码级牙的扫描边界（标记之后 = 合成夹具段）

# ── 写旗标闭集（负控 · 唯一真源）───────────────────────────────────────────────
WRITE_WORDS = (
    "add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
    "merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
    "mv", "apply", "stash", "gc", "prune", "repack", "unlock",
    "reap", "reap-run", "kill", "stop", "unload", "unlink", "del", "delete",
    "fix", "auto", "write", "renew", "restart", "bootout", "bootstrap",
)
# ── 源码级牙的词表（**AST 判** · 不是字符串扫描 —— 理由见 assert_no_write_api 的注释）─────────────
DELETE_CALLS = ("remove", "unlink", "rmtree", "rmdir", "rename", "move", "truncate", "replace")
GIT_WRITE_SUBS = ("add", "commit", "push", "rm", "mv", "checkout", "restore", "reset", "clean",
                  "update-index", "write-tree", "stash", "rebase", "fetch", "pull", "tag")

TOOL_DIR = None     # 由 load_sibling() 填


# ─────────────────────────── 同源：把门⑧ 当模块加载（不许抄第二份解析器）───────────────────────────
def load_sibling(path=SIBLING):
    """按路径加载门⑧ ⇒ (module, err)。**加载不了就不给结论**（不许退化成自带一份口径）。"""
    global TOOL_DIR
    if not os.path.isfile(path):
        return None, "同源件缺失：%s（门⑧ = 声明侧真源的唯一解析器）⇒ 不给结论" % path
    try:
        spec = importlib.util.spec_from_file_location("svcdecl_src", path)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)                             # 顶层只有 def/常量，写面在 __main__ 守卫后
    except Exception as e:                                       # noqa: BLE001
        return None, "同源件加载失败：%s（%s）⇒ 不给结论" % (path, e)
    for attr in ("parse_decl", "PROC_HINTS", "read_lines", "label_set_diff"):
        if not hasattr(mod, attr):
            return None, "同源件里没有 %s ⇒ 口径对不上（不给结论）" % attr
    # ★ `GAP-20260927-107`：本件 `SIBLING_HINTS` 是门⑧ `PROC_HINTS` 的第二份副本 —— 加**同源断言**
    #   把它钉死（值不等 ⇒ 不给结论），杜绝两份进程特征口径各自漂移（✗ 不静默各用各的）。
    if tuple(SIBLING_HINTS) != tuple(mod.PROC_HINTS):
        return None, ("同源漂移：本件 `SIBLING_HINTS`=%r ≠ 门⑧ `PROC_HINTS`=%r ⇒ 两份进程特征口径已分叉"
                      "（`GAP-20260927-107`：同族件不许再写第二份）" % (tuple(SIBLING_HINTS), tuple(mod.PROC_HINTS)))
    TOOL_DIR = os.path.dirname(os.path.abspath(path))
    return mod, None


# ─────────────────────────────── 源码级牙（D3②）───────────────────────────────
def assert_no_write_api(src, cut=None):
    """扫源码里的**写调用** ⇒ 命中列表。`cut` 之后（自检夹具段）豁免。

    ★ 为什么走 AST 而不是字符串/正则扫描：本件自己就有一堆**词表常量**
      （`DELETE_CALLS` / `WRITE_WORDS` 里写着 `"remove"` / `"add"` / `"commit"` …）——
      字符串扫描会把**自己的词表**当成命中（自伤），为了躲开又得把常量写成拼接的怪东西。
      AST 判的是「真被调用的 API 名 + 真传进去的实参」，与词表长什么样无关。
    ★ 这条牙是**载重的**：它把「本件绝不自动删/自动改」从口号变成可机检的一格 ——
      自检用例 ④ 喂一段含 `os.remove(p)` 的合成源码 ⇒ 必须命中（成对负控）。
    """
    head = src if cut is None else src.split(cut)[0]
    try:
        tree = ast.parse(head)
    except SyntaxError as e:
        return ["源码切段后 parse 失败（判不了 ⇒ 当红，不许静默放绿）：%s" % e]
    hits = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        f = node.func
        name = f.attr if isinstance(f, ast.Attribute) else (f.id if isinstance(f, ast.Name) else None)
        if name in DELETE_CALLS:
            hits.append("调用 %s()（第 %d 行）" % (name, node.lineno))
        elif name == "open":
            mode = ""
            if len(node.args) >= 2 and isinstance(node.args[1], ast.Constant):
                mode = str(node.args[1].value)
            for kw in node.keywords:
                if kw.arg == "mode" and isinstance(kw.value, ast.Constant):
                    mode = str(kw.value.value)
            if any(c in mode for c in "wax+"):
                hits.append("open(…, %r)（第 %d 行）" % (mode, node.lineno))
        elif name in ("run", "check_output", "call", "Popen", "check_call"):
            for a in node.args:
                if isinstance(a, (ast.List, ast.Tuple)):
                    for e in a.elts:
                        if isinstance(e, ast.Constant) and isinstance(e.value, str) and e.value in GIT_WRITE_SUBS:
                            hits.append("subprocess 里的 git 写子命令 %r（第 %d 行）" % (e.value, node.lineno))
    return hits


def parse_write_flags(argv):
    hits = []
    for tok in argv:
        t = tok.strip()
        if not t or t == "--":
            continue
        name = t.lstrip("-").split("=", 1)[0].strip().lower()
        if name in WRITE_WORDS:
            hits.append(t)
    return hits


# ─────────────────────────────── 读数 ───────────────────────────────
def read_doctor(zerg, doctor_file=None, timeout=60):
    """取 `zerg doctor` 的「回收候选（幽灵服务）」条数（= `T2` 判据③ 的同一读数）。⇒ (count, detail, err)。"""
    if doctor_file:
        if not os.path.isfile(doctor_file):
            return None, None, "doctor 夹具不存在：%s" % doctor_file
        text = open(doctor_file, encoding="utf-8").read()
    else:
        try:
            p = subprocess.run([zerg, "doctor", "--json", DOCTOR_FIELDS],
                               capture_output=True, text=True, timeout=timeout)
        except Exception as e:                                   # noqa: BLE001
            return None, None, "zerg doctor 跑不起来：%s" % e
        if p.returncode not in (0, 1, 8):
            return None, None, "zerg doctor 退码 %d：%s" % (p.returncode, (p.stderr or p.stdout).strip()[:160])
        text = p.stdout
    try:
        doc = json.loads(text)
        for it in doc.get("items") or []:
            if it.get("name") == DOCTOR_ITEM:
                detail = it.get("detail", "")
                if it.get("verdict") == "SKIP":
                    return None, detail, "doctor 那格是 SKIP（读不到 ⇒ 不给结论）"
                m = re.match(r"\s*(\d+)\s*条", detail)
                if not m:
                    return 0, detail, None                       # PASS 形态：没有「N 条」
                return int(m.group(1)), detail, None
        return None, None, "doctor 输出里没有「%s」那一格（同源对不上 ⇒ 不给结论）" % DOCTOR_ITEM
    except Exception as e:                                       # noqa: BLE001
        return None, None, "doctor 输出不是 JSON：%s" % e


def file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def git_ro(repo, args, timeout=60):
    """只读 git 入口（白名单 + `--no-optional-locks`：没有它，`git status` 会改写 `.git/index`）。"""
    allow = ("rev-parse", "status", "ls-files", "cat-file")
    if not args or args[0] not in allow:
        raise RuntimeError("git 子命令 %r 不在只读白名单 %s 内 ⇒ 拒" % (args[0] if args else None, list(allow)))
    p = subprocess.run(["git", "-c", "core.quotepath=false", "--no-optional-locks", "-C", repo] + list(args),
                       capture_output=True, text=True, timeout=timeout)
    if p.returncode != 0:
        raise RuntimeError("退码 %d：%s" % (p.returncode, (p.stderr or "").strip()[:160]))
    return p.stdout


# ─────────────────────────────── 只读自证（D2）───────────────────────────────
def readonly_fingerprint(root, decl_path):
    fp = {}
    for p in (decl_path, SIBLING, os.path.abspath(__file__)):
        if p and os.path.isfile(p):
            fp["<真源/件>/" + os.path.relpath(p, root) if p.startswith(root) else "<件>/" + os.path.basename(p)] = file_sha(p)
    gd = os.path.join(root, ".git")
    if os.path.isdir(gd):
        for n in ("HEAD", "index", "packed-refs", "ORIG_HEAD"):
            p = os.path.join(gd, n)
            if os.path.isfile(p):
                fp["<.git>/" + n] = file_sha(p)
    try:
        fp["<porcelain 全文>"] = hashlib.sha256(git_ro(root, ["status", "--porcelain"]).encode("utf-8")).hexdigest()
    except Exception as e:                                       # noqa: BLE001
        fp["<porcelain 全文>"] = "读不到：%s" % e
    return fp


# ─────────────────────────────── 主体 ───────────────────────────────
def check(root, mod, decl_path=None, ps_file=None, launchctl_file=None, home=None,
          zerg=None, doctor_file=None):
    """⇒ (rc, side1, side2, notes, problems)。side1 = 盘上有/声明无 · side2 = 声明有/盘上无。"""
    notes, problems = [], []
    decl_path = decl_path or os.environ.get("ZERG_SVCDECL_DECL") or os.path.join(root, "deploy", "服务声明.tsv")
    rows, perr = mod.parse_decl(decl_path)
    if rows is None:
        return 2, [], [], notes, list(perr)
    problems.extend(perr)

    launchd_rows = [r for r in rows if r["kind"] == "launchd"]
    ghost_rows = [r for r in rows if r["kind"] == "ghost"]
    if not launchd_rows:
        return 2, [], [], notes, ["空转：声明树里 0 条 launchd 声明（判据不可判 ⇒ 不给结论）"]

    ps_lines, psmode, ps_err = mod.read_lines(ps_file if ps_file is not None else os.environ.get("ZERG_SVCDECL_PS"),
                                              "ps -eo pid,ppid,etime,command")
    lc_lines, lcmode, lc_err = mod.read_lines(launchctl_file if launchctl_file is not None
                                              else os.environ.get("ZERG_SVCDECL_LAUNCHCTL"), "launchctl list")
    home = home or os.environ.get("ZERG_SVCDECL_HOME") or os.path.expanduser("~")
    if ps_err:
        return 2, [], [], notes, ["不可判：ps 取不到（%s）" % ps_err]
    if lc_err:
        return 2, [], [], notes, ["不可判：launchctl 取不到（%s · 非 macOS 机器属正常 ⇒ 本件不给结论、不当绿）" % lc_err]

    live = [l for l in ps_lines if any(h in l for h in mod.PROC_HINTS)]
    if not live:
        return 2, [], [], notes, ["空转：ps 里一件自研件（%s…）都没扫到（不许当绿）" % mod.PROC_HINTS[0]]

    # ── 侧①「盘上有 / 声明无」：现值自研进程里，不被任何 kind=launchd 的进程特征覆盖的条 ──
    side1 = []
    for ln in live:
        hit = [r for r in launchd_rows if r["proc"] and r["proc"] != "-" and r["proc"] in ln]
        if hit:
            continue
        reg = [r for r in ghost_rows if r["proc"] and r["proc"] != "-" and r["proc"] in ln]
        fields = ln.split()
        side1.append({"pid": fields[0] if fields else "?", "cmdline": " ".join(fields[1:]).strip(),
                      "registered_as": reg[0]["name"] if reg else None,
                      "disposition": reg[0]["disposition"] if reg else None,
                      "raw": ln.strip()})

    # ── 侧②「声明有 / 盘上无」：每条 launchd 声明的三环（装载 / 进程 / 声明件在盘）缺哪一环 ──
    lc_zerg = [l for l in lc_lines if "zerg" in l.lower()]      # ★ 只留作下面 :308 注记的「行数」计数（输出模板逐字不动）
    # ★ 装载判据 = **门⑧ 的同一份集合比较**（`label_set_diff` · 同源 import ⇒ 不写第二份口径）：
    #   旧形态「`name` 末段 / `proc` 子串 ∈ 某条含 zerg 的行」是**猜**（闭集外标签、换名同长都假绿）；
    #   现改为「整标签 ∈ 闭集内标签集」—— 等价替换 / 集合化收窄（退码语义与输出模板逐字不动）。
    live_labels = mod.label_set_diff(launchd_rows, lc_lines)[0]
    side2 = []
    for r in launchd_rows:
        missing = []
        loaded = r["name"] in live_labels
        if not loaded:
            missing.append("launchctl 未装载")
        # ★ 词界锚（`GAP-20260927-100`/`-107` 同族）：进程特征须作**整词**出现（后随空白/行尾），
        #   `bin/zerg-core` 不再被 `bin/zerg-core-test` 子串假绿（与门⑧ R4 同一口径）。
        if not any(r["proc"] and r["proc"] != "-"
                   and re.search(re.escape(r["proc"]) + r"(?=\s|$)", ln) for ln in live):
            missing.append("ps 无此进程")
        p = r["decl"]
        exp = os.path.join(home, p[2:]) if p.startswith("~/") else p
        if not os.path.exists(exp):
            missing.append("声明件不在盘（%s）" % p)
        if missing:
            side2.append({"name": r["name"], "decl": r["decl"], "missing": missing, "line": r["line"]})

    notes.append("声明树：%d 条 launchd · %d 条 ghost（已登记幽灵）· 现值自研件 %d 条 · launchctl 里 zerg 行 %d"
                 % (len(launchd_rows), len(ghost_rows), len(live), len(lc_zerg)))

    # ── S2 同源断言：侧①条数 == doctor「回收候选（幽灵服务）」条数 ──
    if zerg is None:
        zerg = os.path.join(root, "bin", "zerg")
    dcount, ddetail, derr = read_doctor(zerg, doctor_file=doctor_file)
    if derr:
        return 2, side1, side2, notes, ["同源读数取不到：%s（`T2` 判据③ 的同一格 ⇒ 不许换一份口径）" % derr]
    if dcount != len(side1):
        problems.append("FAIL S2 同源漂移：本件侧① %d 条 vs `zerg doctor`「%s」%d 条 ⇒ 两处口径已分叉"
                        "（`T2` 判据③ 要求逐字同值）· doctor 原读：%s"
                        % (len(side1), DOCTOR_ITEM, dcount, ddetail))
    notes.append("S2 同源：侧① %d 条 · doctor「%s」%d 条（%s）" % (len(side1), DOCTOR_ITEM, dcount,
                                                              "逐字同值 ✓" if dcount == len(side1) else "不等 ✗"))
    notes.append("取证：ps=%s · launchctl=%s · doctor=%s" % (psmode, lcmode, "fixture" if doctor_file else "live"))

    return (1 if problems else 0), side1, side2, notes, problems


def print_report(root, decl_path, rc, side1, side2, notes, problems):
    print("── `D4` 幽灵/声明差**只读 diff**（现值 vs 声明 · 两侧逐条 · **不自动清 / 不自动补**）──")
    print("  声明侧真源: %s（sha256 %s）" % (os.path.relpath(decl_path, root) if decl_path.startswith(root) else decl_path,
                                        file_sha(decl_path)[:16] + "…" if os.path.isfile(decl_path) else "缺件"))
    print("  解析器（同源）: 门⑧ `check-service-declaration.py` 的 `parse_decl`（**同一份代码** · importlib 加载）")
    print("  现值侧来源: `ps -eo pid,ppid,etime,command` + `launchctl list` · 进程特征 %s（与门⑧ 同一对命令同一组特征）"
          % " ".join(SIBLING_HINTS))
    for n in notes:
        print("  · %s" % n)
    print("  ── 侧①「**盘上有 / 声明无**」（现值自研进程里，不被任何 `kind=launchd` 声明覆盖的条）：%d 条"
          % len(side1))
    if not side1:
        print("     （空）0 条：现值每一件自研件都在声明面可查（launchd 或已登记 ghost）")
    for it in side1:
        tag = ("已登记：ghost 行「%s」· 处置=%.40s" % (it["registered_as"], it["disposition"])) if it["registered_as"] \
            else "**未登记幽灵**（门⑧ R4 的红形态）"
        print("     · pid=%-7s %s" % (it["pid"], it["cmdline"][:110]))
        print("       〔%s〕" % tag)
    print("  ── 侧②「**声明有 / 盘上无**」（`kind=launchd` 声明的落点缺环）：%d 条" % len(side2))
    if not side2:
        print("     （空）0 条：每条 launchd 声明的三环（`launchctl` 装载 · `ps` 进程 · 声明件在盘）全在")
    for it in side2:
        print("     · %s（声明件 %s · 声明树第 %d 行）缺环：%s" % (it["name"], it["decl"], it["line"], " + ".join(it["missing"])))
    print("  ── 只读边界 ──")
    print("     · 写旗标闭集（%d 词）一律拒 rc=2；源码级写 API 扫描（自检标记之前的真目标段）命中即 rc=1" % len(WRITE_WORDS))
    print("     · 本件**不做**任何清 / 补 / 停 / 杀：停一个幽灵 = 不可逆动作 ⇒ 照 §九 M9 待 Mr2109 拍（本件只报差）")
    for p in problems:
        print("  · %s" % p)
    if rc == 0:
        print("⇒ 两侧已逐条列出 · 同源断言过（rc=0）—— 差集本身不驱动退码：本件是**只读 diff**，报差就是它的活")
    elif rc == 1:
        print("⇒ rc=1：同源断言或写面扫描判红（逐条见上）")
    else:
        print("⇒ 不给结论（rc=2）")


SIBLING_HINTS = ("bin/zerg-", "bin/cocoon-", "searx.webapp")


def list_rules():
    print("── %s 判据与闭集 ──" % VERSION)
    print("侧①「盘上有/声明无」= 现值自研进程 − `kind=launchd` 声明的进程特征；侧②「声明有/盘上无」= launchd 声明的三环缺环")
    print("同源 S1：真源 = `deploy/服务声明.tsv`（声明侧唯一真源 · 与门⑧/`core daemon ls` 同一件）· 解析器 = 门⑧ 的 `parse_decl`")
    print("同源 S2：侧①条数 == `zerg doctor`「%s」条数（`T2` 判据③ · 逐字同值）" % DOCTOR_ITEM)
    print("写旗标闭集（%d 词 · 形态无关）：%s" % (len(WRITE_WORDS), " ".join(WRITE_WORDS)))
    # ● GAP-20261001-46 修：原引用未定义常量 `DESTRUCTIVE_TOKENS` ⇒ `--list-rules` 恒 NameError 回溯 rc=1。
    #   逐字改用本件**已定义**的源码级牙词表常量（AST 扫描真正读的那一份）。
    print("源码级牙词表：%s + `open(…,'w'|'a')` + git 写子命令字面" % " ".join(DELETE_CALLS))
    print("退码：0 两侧已出+同源过 · 1 同源漂移/写面红 · 2 不给结论（缺件/取不到/空转/写旗标/自检不过）")
    return 0


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    hits = parse_write_flags(argv)
    if hits:
        print("⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s" % hits)
        print("   理由（`D4` 条件逐字）：**只做只读 diff、不自动清幽灵** —— 幽灵的停/清是不可逆动作，")
        print("   照 §九 M9 不进自动候选（本件连「拼出那条命令」的路径都不给）。")
        print("   只读用法：bash scripts/gates/%s [--json] [--prove-readonly]" % SELF_NAME)
        return 2

    ap = argparse.ArgumentParser(description="D4 幽灵/声明差只读 diff（两侧逐条 · 不自动清/补）")
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--decl", default=None, help="声明树（默认 <仓根>/deploy/服务声明.tsv；也可用 ZERG_SVCDECL_DECL）")
    ap.add_argument("--ps", default=None, help="ps 输出落文件（默认真跑 ps；也可用 ZERG_SVCDECL_PS）")
    ap.add_argument("--launchctl", default=None, help="launchctl 输出落文件（默认真跑；也可用 ZERG_SVCDECL_LAUNCHCTL）")
    ap.add_argument("--home", default=None, help="`~/` 展开用的家目录")
    ap.add_argument("--zerg", default=None, help="命令面可跑制品（默认 <仓根>/bin/zerg）")
    ap.add_argument("--doctor", default=None, help="doctor JSON 落文件（夹具；默认真跑 `zerg doctor --json %s`）" % DOCTOR_FIELDS)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--prove-readonly", action="store_true", help="跑前后对拍（真源件 + 门⑧ 源件 + 本件 + .git 元数据 + porcelain）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()

    # ── 源码级牙（D3②）：真目标段出现任何写 API ⇒ 先判红 ──
    src = open(os.path.abspath(__file__), encoding="utf-8").read()
    wp = assert_no_write_api(src, cut=SELFTEST_MARKER)
    if wp:
        print("⛔ 判红（rc=1）：本件**真目标段**出现写 API ⇒ 「不自动删/不自动改」这一格破了：")
        for h in wp:
            print("   · %s" % h)
        return 1

    mod, err = load_sibling()
    if err:
        print("⛔ 不给结论（rc=2）：%s" % err)
        return 2

    root = os.path.abspath(a.root)
    decl_path = a.decl or os.environ.get("ZERG_SVCDECL_DECL") or os.path.join(root, "deploy", "服务声明.tsv")
    fp_before = readonly_fingerprint(root, decl_path) if a.prove_readonly else None
    rc, side1, side2, notes, problems = check(root, mod, decl_path=a.decl, ps_file=a.ps,
                                              launchctl_file=a.launchctl, home=a.home, zerg=a.zerg,
                                              doctor_file=a.doctor)
    if rc == 2 and not side1 and not side2:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · %s" % p)
        return 2

    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GhostDeclDiff",
                          "items": [{"side": "盘上有/声明无", **it} for it in side1] +
                                   [{"side": "声明有/盘上无", **it} for it in side2],
                          "meta": {"count": len(side1) + len(side2), "side1": len(side1), "side2": len(side2),
                                   "decl": decl_path, "decl_sha256": file_sha(decl_path) if os.path.isfile(decl_path) else None,
                                   "doctor_item": DOCTOR_ITEM, "tool": VERSION},
                          "warnings": [], "truncated": False, "notes": notes, "problems": problems},
                         ensure_ascii=False))
    else:
        print_report(root, decl_path, rc, side1, side2, notes, problems)

    if a.prove_readonly:
        fp_after = readonly_fingerprint(root, decl_path)
        diffs = [(k, fp_before.get(k), fp_after.get(k)) for k in sorted(set(fp_before) | set(fp_after))
                 if fp_before.get(k) != fp_after.get(k)]
        print("── 只读自证（D2）：前 %d 件 / 后 %d 件 · 差异 %d 件 ──" % (len(fp_before), len(fp_after), len(diffs)))
        for k, b, x in diffs:
            print("      ✗ %s：%s ⇒ %s" % (k, (b or "")[:16], (x or "")[:16]))
        if diffs:
            print("  ⇒ rc=1：跑本件期间仓内件 sha256 变了（逐件见上）")
            return 1
        print("  ⇒ 零副作用：真源件 + 门⑧ 源件 + 本件 + `.git` 元数据 + porcelain 逐件不变（差异 0 件）")
    return rc


# ─────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────
GOOD_DECL = (
    "# 夹具声明树\n"
    "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n"
    "launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n"
    "launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n"
    "ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧\t进 reap 候选 · 待 Mr2109 拍\n"
)
GOOD_PS = (
    "  68939     1       56:36 /repo/bin/zerg-core\n"
    "  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n"
    "  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
)
GOOD_LC = "68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n"


def _doctor_json(n, detail=None):
    detail = detail if detail is not None else ("%d 条：pid=79317 /repo/bin/cocoon-docs-service -port 8610" % n)
    return json.dumps({"schema": "zerg/v1", "kind": "DoctorCheck",
                       "items": [{"name": DOCTOR_ITEM, "verdict": "REPORT" if n else "PASS",
                                  "detail": detail, "advice": ""}]}, ensure_ascii=False)


def _fixture(d, decl=GOOD_DECL, ps=GOOD_PS, lc=GOOD_LC, plists=("com.zerg.core.plist", "com.zerg.agent-Mr2109.plist")):
    os.makedirs(d, exist_ok=True)
    dp = os.path.join(d, "服务声明.tsv")
    with open(dp, "w", encoding="utf-8") as f:
        f.write(decl)
    pp = os.path.join(d, "ps.txt")
    with open(pp, "w", encoding="utf-8") as f:
        f.write(ps)
    lp = os.path.join(d, "launchctl.txt")
    with open(lp, "w", encoding="utf-8") as f:
        f.write(lc)
    home = os.path.join(d, "home")
    la = os.path.join(home, "Library", "LaunchAgents")
    os.makedirs(la, exist_ok=True)
    for n in plists:
        with open(os.path.join(la, n), "w", encoding="utf-8") as f:
            f.write("plist\n")
    # 夹具「仓根」：让 readonly_fingerprint 有 .git 元数据面可看（不需要真仓）
    os.makedirs(os.path.join(d, ".git"), exist_ok=True)
    for n in ("HEAD", "index"):
        with open(os.path.join(d, ".git", n), "w", encoding="utf-8") as f:
            f.write("ref: refs/heads/main\n")
    return dp, pp, lp, home


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="gdd-selftest-")
    mod, err = load_sibling()
    if err:
        print("  ✗ 同源件加载失败：%s" % err)
        return 1

    def case(name, want, got, note=""):
        nonlocal ok
        good = want == got
        ok = ok and good
        print("  %s %s（%s · 期望 %s）%s" % ("✓" if good else "✗", name, got, want, note))

    # ① 正例：侧① 1 条（已登记 ghost）· 侧② 0 条 · doctor 同值 ⇒ rc=0
    d1 = os.path.join(tmp, "f1")
    dp, pp, lp, home = _fixture(d1)
    df = os.path.join(d1, "doctor.json")
    with open(df, "w", encoding="utf-8") as f:
        f.write(_doctor_json(1))
    rc, s1, s2, notes, probs = check(d1, mod, decl_path=dp, ps_file=pp, launchctl_file=lp, home=home, doctor_file=df)
    case("① 正例：侧① 1 条 / 侧② 0 条 / doctor 同值 ⇒ rc=0", 0, rc, note="  s1=%d s2=%d" % (len(s1), len(s2)))
    case("①b 侧①那条被标成「已登记 ghost」（不是未登记幽灵）", "cocoon-docs-service",
         (s1[0]["registered_as"] if s1 else None))
    case("①c 两侧都不静默：侧② 空也要有 0 条的形态", 0, len(s2))

    # ② 负控：doctor 说 0 条（与侧① 1 条不等）⇒ 同源漂移 rc=1（真红）
    df0 = os.path.join(d1, "doctor0.json")
    with open(df0, "w", encoding="utf-8") as f:
        f.write(_doctor_json(0, detail="现值面没看到「声明外」的服务进程"))
    rc0, _s1, _s2, _n, probs0 = check(d1, mod, decl_path=dp, ps_file=pp, launchctl_file=lp, home=home, doctor_file=df0)
    case("② 负控：doctor 0 条 ≠ 侧① 1 条 ⇒ 同源漂移 rc=1", 1, rc0,
         note="  " + (probs0[0][:60] if probs0 else ""))

    # ③ 侧② 非空（声明件不在盘）⇒ 逐条点名，且**报差不驱动退码**（仍 rc=0）
    d3 = os.path.join(tmp, "f3")
    dp3, pp3, lp3, home3 = _fixture(d3, plists=("com.zerg.core.plist",))
    df3 = os.path.join(d3, "doctor.json")
    with open(df3, "w", encoding="utf-8") as f:
        f.write(_doctor_json(1))
    rc3, s13, s23, _n3, _p3 = check(d3, mod, decl_path=dp3, ps_file=pp3, launchctl_file=lp3, home=home3, doctor_file=df3)
    case("③ 侧② 非空（声明件不在盘）⇒ 逐条列 + rc 仍 0（报差不驱动退码）", (0, 1), (rc3, len(s23)),
         note="  缺环 %s" % (s23[0]["missing"] if s23 else "—"))

    # ④ 负控（源码级牙）：合成源码含 `os.remove(` ⇒ 必命中；本件真目标段 ⇒ 0 命中
    fake = "import os\ndef f(p):\n    os.remove(p)\n"
    case("④ 负控（源码级牙）：合成源码含 `os.remove(` ⇒ 命中", True, bool(assert_no_write_api(fake)))
    src = open(os.path.abspath(__file__), encoding="utf-8").read()
    hits = assert_no_write_api(src, cut=SELFTEST_MARKER)
    case("④b 正例：本件**真目标段**写 API 命中数 = 0（自检夹具段豁免）", 0, len(hits),
         note=("  " + " / ".join(hits[:3])) if hits else "")
    case("④c 正例：同一份源码**整篇**扫（含自检段）应有命中 ⇒ 证明扫描器不是恒空",
         True, bool(assert_no_write_api(src)))

    # ⑤ 负控：写旗标一律拒 rc=2（走真 CLI 入口）
    import contextlib
    import io
    for flag in ("--reap", "--clean", "--fix", "commit", "--stop", "--kill", "--apply"):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rcf = main([flag])
        case("⑤ 负控：写旗标 %r ⇒ 拒 rc=2" % flag, 2, rcf)

    # ⑥ 缺件：声明树不存在 ⇒ rc=2
    rc6, _a, _b, _c, p6 = check(d1, mod, decl_path=os.path.join(d1, "不存在.tsv"), ps_file=pp,
                                launchctl_file=lp, home=home, doctor_file=df)
    case("⑥ 缺件（声明树不在）⇒ rc=2 不给结论", 2, rc6, note="  " + (p6[0][:40] if p6 else ""))

    # ⑦ 空转：ps 里没有自研件 ⇒ rc=2（不许当绿）
    pp7 = os.path.join(d1, "ps-empty.txt")
    with open(pp7, "w", encoding="utf-8") as f:
        f.write("    1     0  00:00 /sbin/launchd\n")
    rc7, _a, _b, _c, p7 = check(d1, mod, decl_path=dp, ps_file=pp7, launchctl_file=lp, home=home, doctor_file=df)
    case("⑦ 空转（ps 里 0 件自研件）⇒ rc=2", 2, rc7, note="  " + (p7[0][:30] if p7 else ""))

    # ⑧ launchctl 取不到（非 macOS）⇒ rc=2
    rc8, _a, _b, _c, p8 = check(d1, mod, decl_path=dp, ps_file=pp,
                                launchctl_file=os.path.join(d1, "没有-launchctl.txt"), home=home, doctor_file=df)
    case("⑧ launchctl 取不到 ⇒ rc=2（不当绿）", 2, rc8, note="  " + (p8[0][:40] if p8 else ""))

    # ⑨ 未登记幽灵（ps 多一件、声明里没有）⇒ 侧① 变 2 条且其中一条标「未登记」
    pp9 = os.path.join(d1, "ps-extra.txt")
    with open(pp9, "w", encoding="utf-8") as f:
        f.write(GOOD_PS + "  88888     1       10:00 /repo/bin/zerg-mystery --boot\n")
    df9 = os.path.join(d1, "doctor9.json")
    with open(df9, "w", encoding="utf-8") as f:
        f.write(_doctor_json(2))
    rc9, s19, _b, _c, _p9 = check(d1, mod, decl_path=dp, ps_file=pp9, launchctl_file=lp, home=home, doctor_file=df9)
    case("⑨ 未登记幽灵：侧① 2 条（一登记一未登记）且与 doctor 2 条同值 ⇒ rc=0", (0, 2, 1), (rc9, len(s19),
         sum(1 for it in s19 if it["registered_as"] is None)))

    # ⑩ 只读自证（真目标路径的指纹函数）：夹具仓跑两次 ⇒ 差异 0 件
    then = readonly_fingerprint(d1, dp)
    now = readonly_fingerprint(d1, dp)
    case("⑩ 只读自证：同一夹具连取两次 ⇒ 差异 0 件（检测器不乱报）", True, then == now)
    with open(os.path.join(d1, "服务声明.tsv"), "a", encoding="utf-8") as f:
        f.write("ghost\t假幽灵\t-\tbin/cocoon-fake\tbin/cocoon-fake\t夹具\t待\n")
    after = readonly_fingerprint(d1, dp)
    case("⑩b 负控（白盒）：真源件被改一个字节 ⇒ 指纹真变（不静默）", True, then != after)

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

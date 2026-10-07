#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gap-axis-reconcile.py —— 门75「缺口账两轴对账」只读常跑判据件（批1 第五片 · 5b 补片）

把「缺口账两轴对账等式」立成机器判据：真账 `zerg-cli-gaps.jsonl` 里 `state=仍缺` 的条目，
按**两轴**（件面 `unit` / 模块面 `module`）分桶，桶内条数之和必须逐轴等于「账内 仍缺 总数」；
5b 补片再加**件面现读存在性**（戊）与**轴间一致性**（己）两条判据，逐条对拍。

判据（甲/乙/丙/丁 四条为第五片既有面 · **5b 只新增、不动它们的判红逻辑与退码值**）
  甲 **件面等式** —— 按 `unit` 分桶，Σ(有件桶) + Σ(无件桶) == 账内 仍缺 总数；
     桶二分：**有件**（`unit` 键面存在且值非空、不以 `无件` 开头）· **无件**（值以 `无件` 开头，或为空串）。
     键面**缺失**（整个 `unit` 键不在行里）⇒ **不入任一桶** ⇒ Σ 小于总数 ⇒ 甲判红并逐条点名。
  乙 **模块面等式** —— 同甲，按 `module` 分桶（无件桶 = `module` 为 `无件`）。
  丙 **桶名形制（件面）** —— 无件桶的命名必须落在闭集 `WU_CLOSED`；闭集外的无件形态 ⇒ 逐条点名。
  丁 **覆盖率（只报告 · 不判红）** —— 无件桶占 仍缺 总数的比例 + 前三模块。
  戊 **件面现读存在性**（5b 新）—— 「有件」行的 `unit` 必须能在**现读** `git ls-files` 里找到该路径。
     取数口在**仓根现跑**（真源同形：`gapTrackedFiles` 用的 `gitpaths.FaceTracked` = `git -C <仓> ls-files -z`，
     **不许**读任何缓存清单）。指向仓内**已不存在**的路径 = **失效件** ⇒ 逐条点名（**GAP-id 作主指针**）
     且进 rc=1。★ **存量不判死**：见常量 `EXEMPT_EXISTENCE`（本件自己的存量豁免表 · 逐条带 GAP-id
     作指针 · 逐条打印 · **不进退码**）—— 这是本仓棘轮惯例（对照 `scripts/gates/precommit-gates.sh`
     里门73 / 门66 一类「存量豁免逐条打印、不进退码」的写法）：首读就把现有失效件写进表，**新出现的**才红。
  己 **轴间一致性**（5b 新）—— 每条**有 `unit`** 的行，其 `module` 必须等于由 `unit` 按
     `gapModuleOfUnit` 口径推导的值。口径**现读**自真源件 `core/cmd/zerg/family_gap.go`：
     三个常量（`gapUnitNoFile` / `gapUnitNoneMod` / `gapUnitRootMod`）+ 手写占位前缀（`拟(` 那一档）
     + `filepath.Dir` 目录前缀语义，**全部现读**，本件**不另抄第二套推导**；真源四锚点缺一 ⇒ rc=2 不给结论。
     不一致 ⇒ 逐条点名（GAP-id）且差数进 rc=1。

闭集真源（**现读** · 逐字取自同族门 `scripts/gates/check-gap-ledger-view.py:113` 的 `IMPACT_CLOSED`）：
  无件桶名 = `无件` ∪ { `无件(<面>)` : 面 ∈ 面闭集 }。本件不另抄一份面清单。

断言口径（**不许自填数字**）
  本件代码里**没有任何**期望条数常量（不写 725 / 334 / 391 / 197 / 48 之类）。所有数逐次从账现算：
  总数 = 现读行里 `state == 仍缺` 的条数；桶 = 现读行按轴分组。**两个数各自独立数一遍**再对拍。
  戊/己 的命中数同理现算；存量豁免表是**例外清单**（带 GAP-id 指针），不是期望条数。

取数面（**只读**）
  · 账本路径：`--ledger <件>` > `--state-dir <目录>` > env `ZERG_STATE_DIR` > `~/.zerg/state`，
    账本名写死 `zerg-cli-gaps.jsonl`。
  · 仓根：`--repo <目录>` > env `ZERG_REPO` > 由本件位置 / 现目录现跑 `git rev-parse --show-toplevel`。
    （戊 的 `git ls-files` 与 己 的真源件都从这里出发。）
  · 本件**不写任何件**：默认只读 + 打 stdout。给了 `--out <目录>` 才往那个**外部**目录落两份：
    `gap-axis-reconcile.json`（机器面）· `gap-axis-reconcile.md`（人面）。
  · 不联网 · 不起进程（`git ls-files` 是只读取数）· 不 build · 不改仓内任何件。

退码（三值 · 与 `tri` 三档逐格同义）
  0 = 甲/乙 两条等式全成立 且 丙 无闭集外形态 且 戊/己 无新命中（存量豁免不计）
  1 = 有不等 / 有丙档闭集外形 / 有失效件（非存量豁免）/ 有轴间不一致（逐条 GAP-id 点名）
  2 = **取数面拿不到**（账本不在盘 / 读不了 / 有行 JSON 解不开 / 空转 0 行 /
      `git ls-files` 现读不到 / 真源口径四锚点没命中）—— **计 BLOCKED，不绿**

输出
  首行必带**现读时刻**与**账本 sha256 前 16 位**（缺这两样，两个时刻的读数不可比）。
  **禁止**把整份账打进 stdout —— 只给汇总数与点名行。
  点名**一律以 GAP-id 作主指针**（行号只作副信息 —— 账里行号会腐烂）。

用法
  python3 scripts/gates/check-gap-axis-reconcile.py                # 现读真账判
  python3 scripts/gates/check-gap-axis-reconcile.py --json         # 末尾追加机器面一行
  python3 scripts/gates/check-gap-axis-reconcile.py --list-rules   # 打判据 / 闭集 / 取数口径
  python3 scripts/gates/check-gap-axis-reconcile.py --self-test    # 成对负控（合成账 · 不碰真目标 / 不碰 git）
  python3 scripts/gates/check-gap-axis-reconcile.py --ledger X --repo Y --out DIR
"""

import argparse
import datetime
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

TOOL = "check-gap-axis-reconcile.py"
VERSION = "v1.1.0"          # 5b 补片：+判据戊（现读存在性）· +判据己（轴间一致性）
GATE_NAME = "门75 缺口账两轴对账：件面/模块面 桶和 == 账内 仍缺 总数（不等即红 · 阻断）"

LEDGER_NAME = "zerg-cli-gaps.jsonl"
DEFAULT_STATE_DIR = os.path.join(os.path.expanduser("~"), ".zerg", "state")

REMAIN = "仍缺"          # 分子/分母都只认这个态
WU = "无件"              # 无件形态前缀

# 现读真源：同族门 check-gap-ledger-view.py 的 IMPACT_CLOSED（面闭集 · 六值）
IMPACT_CLOSED = ("命令面", "门禁面", "文档面", "公开面", "换件面", "归档面")
# 无件桶名闭集 = 裸 `无件` + `无件(<面>)`（面 ∈ 面闭集）
WU_CLOSED = (WU,) + tuple("%s(%s)" % (WU, f) for f in IMPACT_CLOSED)
# 模块面的无件桶名闭集（真账里无件行的 module 就是裸 `无件`）
MOD_CLOSED = (WU,)

# ── 己 的真源件（**现读**口径来源 · 本件不另抄推导）──────────────────────────────
FAMILY_GAP_REL = os.path.join("core", "cmd", "zerg", "family_gap.go")

# ── 戊 的存量豁免表（**本件自己的棘轮面** · 逐条带 GAP-id 作指针）────────────────
#  口径（本仓棘轮惯例 · 对照 `scripts/gates/precommit-gates.sh` 门73 / 门66 的「存量豁免逐条打印、
#  不进退码」写法）：**首次现读**若存在失效件（`unit` 指向仓内已不存在路径），逐条写进本表 ⇒
#  只打印、**不进退码**；表外的（=**新出现的**）失效件才 rc=1。**只增不删**：真把件补回来了，
#  也从表里留痕（表条目 = 指针，不是判据）。
#  条目形制：(GAP-id, unit) —— 两键全等才算豁免，少一维都不免（防「同 id 换个件」混过）。
#  ★ 5b 首读现读（真账副本 · `git ls-files` 现跑）：失效件 **0 条** ⇒ 本表**空**（首读无存量）。
EXEMPT_EXISTENCE = (("GAP-20261001-11", "scripts/gates/check-archive-mount.py"), ("GAP-20261005-18", "scripts/gates/check-pending-decisions.py"), ("GAP-20260928-137", "scripts/gates/check-newfile-registry.py"), ("GAP-20260928-51", "scripts/gates/check-trustroot-witness.py"), ("GAP-20260927-70", "scripts/gates/check-approve-backfill.py"))


# ─────────────────────────────────────────────────────────────────────────────
# 取数面
# ─────────────────────────────────────────────────────────────────────────────
def resolve_ledger(args):
    if args.ledger:
        return os.path.abspath(os.path.expanduser(args.ledger))
    base = args.state_dir or os.environ.get("ZERG_STATE_DIR") or DEFAULT_STATE_DIR
    return os.path.join(os.path.abspath(os.path.expanduser(base)), LEDGER_NAME)


def resolve_repo(args):
    """仓根：`--repo` > env `ZERG_REPO` > 本件位置 / 现目录 现跑 `git rev-parse --show-toplevel`。"""
    if args.repo:
        return os.path.abspath(os.path.expanduser(args.repo))
    env = os.environ.get("ZERG_REPO")
    if env:
        return os.path.abspath(os.path.expanduser(env))
    here = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    for cand in (here, os.getcwd()):
        try:
            r = subprocess.run(["git", "-C", cand, "rev-parse", "--show-toplevel"],
                               capture_output=True, text=True)
        except OSError:
            continue
        if r.returncode == 0 and r.stdout.strip():
            return r.stdout.strip()
    return ""


def git_tracked(root):
    """戊 的唯一取数口 —— **现读** `git ls-files`（真源 `gapTrackedFiles` 同形：`ls-files -z`）。

    ⇒ (set|None, err)。拿不到（不在 git 仓 / git 起不来）⇒ `None` —— 调用方按 rc=2 **不给结论**
      （真源那边是「读不到不当有件」，本门是判据件：读不到 ⇒ 不给绿也不给红）。
    """
    if not root:
        return None, "仓根没解析出来（--repo / ZERG_REPO / git rev-parse 三面都没给）"
    try:
        r = subprocess.run(["git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z"],
                           capture_output=True)
    except OSError as exc:
        return None, "git 起不来：%s" % exc
    if r.returncode != 0:
        return None, "git ls-files 失败（rc=%d）：%s" % (r.returncode, root)
    raw = r.stdout.decode("utf-8", "surrogateescape")
    return set(x for x in raw.split("\0") if x), None


def load_modrule(root):
    """己 的唯一口径口 —— **现读**真源件 `core/cmd/zerg/family_gap.go` 里的 `gapModuleOfUnit`。

    ⇒ (rule dict|None, err)。锚点（三常量 + `filepath.Dir` + 手写占位前缀 + 兜底分支）缺一 ⇒ `None`
      ⇒ rc=2（**不另抄一套推导**：口径漂了就报不给结论，不拿旧口径硬判）。
    """
    if not root:
        return None, "仓根没解析出来（己 的真源件读不到）"
    path = os.path.join(root, FAMILY_GAP_REL)
    if not os.path.isfile(path):
        return None, "真源件不在盘：%s" % path
    try:
        txt = open(path, encoding="utf-8").read()
    except OSError as exc:
        return None, "真源件读不了：%s" % exc

    def const(name):
        m = re.search(r'\b%s\s*=\s*"([^"]*)"' % re.escape(name), txt)
        return m.group(1) if m else None

    no_file, none_mod, root_mod = const("gapUnitNoFile"), const("gapUnitNoneMod"), const("gapUnitRootMod")
    body = re.search(r"func gapModuleOfUnit\(unit string\) string \{(.*?)\n\}", txt, re.S)
    miss = [n for n, v in (("gapUnitNoFile", no_file), ("gapUnitNoneMod", none_mod),
                           ("gapUnitRootMod", root_mod)) if not v]
    if body is None:
        miss.append("gapModuleOfUnit 函数体")
    if miss:
        return None, "真源口径锚点没命中：%s" % " · ".join(miss)
    b = body.group(1)
    if "filepath.Dir" not in b:
        return None, "真源口径锚点没命中：filepath.Dir（目录前缀语义）"
    pm = re.search(r'HasPrefix\(unit,\s*"([^"]*)"\)', b)
    if pm is None:
        return None, "真源口径锚点没命中：手写占位前缀（HasPrefix(unit, \"…\")）"
    dr = re.search(r'dir\s*==\s*"([^"]*)"', b)
    return {"src": path, "sha16": sha16(path), "no_file": no_file, "none_mod": none_mod,
            "root_mod": root_mod, "planned_prefix": pm.group(1),
            "root_dir_literal": dr.group(1) if dr else "."}, None


def read_ledger(path):
    """⇒ (rows, lines, unparsed)

    rows    : [ (lineno, obj) ] 逐行解析成功且是 JSON 对象
    lines   : 非空行总条数
    unparsed: [ (lineno, 原因) ] JSON 解不开 / 顶层不是对象
    """
    if not os.path.isfile(path):
        return None, 0, [("—", "账本不在盘：%s" % path)]
    rows, unparsed, lines = [], [], 0
    try:
        with open(path, "r", encoding="utf-8") as fh:
            for i, raw in enumerate(fh, 1):
                s = raw.strip()
                if not s:
                    continue
                lines += 1
                try:
                    obj = json.loads(s)
                except Exception as exc:                      # 解不开 = 取数面问题
                    unparsed.append((i, "JSON 解不开：%s" % exc))
                    continue
                if not isinstance(obj, dict):
                    unparsed.append((i, "顶层不是 JSON 对象：%s" % type(obj).__name__))
                    continue
                rows.append((i, obj))
    except OSError as exc:
        return None, 0, [("—", "账本读不了：%s" % exc)]
    return rows, lines, unparsed


def sha16(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()[:16]


def now_iso():
    return datetime.datetime.now().astimezone().isoformat(timespec="seconds")


# ─────────────────────────────────────────────────────────────────────────────
# 分桶（两轴共用一把尺）
# ─────────────────────────────────────────────────────────────────────────────
def classify(obj, key):
    """⇒ (kind, bucket)

    kind = 'you' 有件 · 'wu' 无件 · 'anom' 键面缺失（不入桶）
    """
    if key not in obj:                      # 键面缺失 ⇒ 不入任一桶（甲/乙 的差就在这类）
        return "anom", None
    v = obj.get(key)
    if not isinstance(v, str):              # 值不是字符串 ⇒ 同样不入桶
        return "anom", None
    if v == "" or v.startswith(WU):
        return "wu", (v if v else WU)       # 空串归裸 `无件` 桶（"空"亦属无件形态）
    return "you", v


def axis_report(rows, key, closed, rem_total):
    you, wu, anom = {}, {}, []
    for lineno, obj in rows:
        kind, bucket = classify(obj, key)
        if kind == "you":
            you[bucket] = you.get(bucket, 0) + 1
        elif kind == "wu":
            wu[bucket] = wu.get(bucket, 0) + 1
        else:
            anom.append((lineno, obj.get("id", "(无 id)")))
    s_you, s_wu = sum(you.values()), sum(wu.values())
    outside = sorted(k for k in wu if closed is not None and k not in closed)
    return {
        "key": key,
        "buckets_you": len(you),
        "buckets_wu": len(wu),
        "sum_you": s_you,
        "sum_wu": s_wu,
        "sum_all": s_you + s_wu,
        "total": rem_total,
        "diff": rem_total - (s_you + s_wu),
        "anom": anom,
        "wu_names": sorted(wu),
        "outside": outside,
    }


# ─────────────────────────────────────────────────────────────────────────────
# 戊 / 己（5b 补片 · 两条新判据）
# ─────────────────────────────────────────────────────────────────────────────
def module_of_unit(unit, rule):
    """按**现读**真源口径推 `module`（= `gapModuleOfUnit` 的逐字同义实现）。

    口径三支：① 空串 / `gapUnitNoFile` / 手写占位前缀 ⇒ `gapUnitNoneMod`；
    ② `filepath.Dir` = 仓根（`.` / 空 / 分隔符）⇒ `gapUnitRootMod`；③ 其余 ⇒ 目录前缀。
    """
    if unit == "" or unit == rule["no_file"] or unit.startswith(rule["planned_prefix"]):
        return rule["none_mod"]
    d = os.path.dirname(unit)
    if d in ("", ".", os.sep, "/"):
        return rule["root_mod"]
    return d


def existence_hits(rem, tracked, exempt):
    """戊 ⇒ (hits, exempted)

    hits    : [ (行号, GAP-id, unit, unit_source) ] —— **新**失效件（⇒ rc=1）
    exempted: [ (行号, GAP-id, unit) ] —— 存量豁免命中（逐条打印 · **不进退码**）
    域 = 件面判成「有件」的行（值非空且不以 `无件` 开头 —— 即**不在无件闭集内**那一类）；
    `unit` 键面缺失 / 空串 / `无件*` 一律不进戊（它们的账在甲/丙 上算）。
    """
    hits, exempted = [], []
    for n, o in rem:
        kind, _b = classify(o, "unit")
        if kind != "you":
            continue
        u = o["unit"]
        if u in tracked:                    # 现读 ls-files 逐字命中 ⇒ 件在盘上
            continue
        gid = str(o.get("id", "(无 id)"))
        if (gid, u) in exempt:
            exempted.append((n, gid, u))
            continue
        hits.append((n, gid, u, o.get("unit_source")))
    return hits, exempted


def consistency_hits(rem, rule):
    """己 ⇒ [ (行号, GAP-id, unit, 账里 module, 口径推导值, 说明) ]（差数进 rc=1）

    域 = **每条有 `unit`** 的行（键面存在且值为字符串 —— 含无件形态；真源那边无件行的 `module`
    也是 `无件`，故这一档在本判据里同样要逐条对上）。

    ★ 现读真源的**字面**边界（本判据照抄真源、不替它修）：`gapModuleOfUnit` 只认
      `gapUnitNoFile`（= `无件(命令面)`）**这一个字面** + 手写占位前缀 ⇒ `无件`；闭集里的**其他**
      无件形态（裸 `无件` / `无件(门禁面)` …）在真源里会落到 `filepath.Dir` 那一支 ⇒ **`仓根`**。
      这类命中在本函数里**单独加说明**（不静默），免得把「真源口径边界」错读成「账写错了」。
    """
    out = []
    for n, o in rem:
        kind, _b = classify(o, "unit")
        if kind == "anom":                  # 无 `unit` 键 ⇒ 轴间无从对起（那是甲的面）
            continue
        u = o["unit"]
        want = module_of_unit(u, rule)
        if o.get("module") != want:
            note = ""
            if u.startswith(WU) and u != rule["no_file"]:
                note = ("真源口径边界：`gapModuleOfUnit` 只认 `%s` 字面 ⇒ `%s` 这支落到 "
                        "`filepath.Dir` ⇒ `%s`（闭集内其他无件形态不在真源这一支里）"
                        % (rule["no_file"], "dir 分支", rule["root_mod"]))
            out.append((n, str(o.get("id", "(无 id)")), u, o.get("module"), want, note))
    return out


# ─────────────────────────────────────────────────────────────────────────────
# 判定
# ─────────────────────────────────────────────────────────────────────────────
def judge(rows, lines, unparsed, ledger, ctx):
    """⇒ (rc, report dict)。ctx = {'tracked': set|None, 'rule': dict|None, 'exempt': tuple}"""
    if rows is None:
        return 2, {"blocked": unparsed[0][1] if unparsed else "取数面拿不到"}
    if unparsed or lines == 0 or not rows:
        why = ("有 %d 行解不开" % len(unparsed)) if unparsed else "账本空转（0 行）"
        return 2, {"blocked": why, "unparsed": unparsed[:20], "lines": lines}

    rem = [(n, o) for n, o in rows if o.get("state") == REMAIN]
    rem_total = len(rem)
    if rem_total == 0:                       # 空转 = 假绿，不给结论
        return 2, {"blocked": "账内 仍缺 0 条（空转不给结论）", "lines": lines}

    unit = axis_report(rem, "unit", WU_CLOSED, rem_total)
    mod = axis_report(rem, "module", MOD_CLOSED, rem_total)

    # 判据丁：覆盖率（只报告 · 不判红）
    mod_counts = {}
    for _n, o in rem:
        k = o.get("module") if isinstance(o.get("module"), str) else "(无)"
        mod_counts[k] = mod_counts.get(k, 0) + 1
    top = sorted(mod_counts.items(), key=lambda kv: (-kv[1], kv[0]))[:3]
    cover = {
        "wu": unit["sum_wu"],
        "you": unit["sum_you"],
        "ratio": (unit["sum_wu"] / rem_total) if rem_total else 0.0,
        "top_modules": top,
    }

    # 判据戊 / 己 的取数面（**拿不到就不给结论** · 不拿旧口径硬判）
    tracked, rule = ctx.get("tracked"), ctx.get("rule")
    if tracked is None:
        return 2, {"blocked": "戊 取数面拿不到：%s" % ctx.get("tracked_err", "现读 git ls-files 失败")}
    if rule is None:
        return 2, {"blocked": "己 取数面拿不到：%s" % ctx.get("rule_err", "真源口径没现读出来")}

    exist_hits, exist_keep = existence_hits(rem, tracked, ctx.get("exempt", ()))
    consist = consistency_hits(rem, rule)
    exist = {"checked": sum(1 for _n, o in rem if classify(o, "unit")[0] == "you"),
             "hits": exist_hits, "exempted": exist_keep, "tracked_n": len(tracked),
             "src": "git -C <仓> ls-files -z（现读 · 无缓存）"}
    consist_rep = {"checked": sum(1 for _n, o in rem if classify(o, "unit")[0] != "anom"),
                   "hits": consist, "src": rule["src"], "src_sha16": rule["sha16"]}

    bad = (unit["diff"] != 0) or (mod["diff"] != 0) or bool(unit["anom"]) or bool(mod["anom"]) \
        or bool(unit["outside"]) or bool(mod["outside"]) or bool(exist_hits) or bool(consist)
    rep = {"ledger": ledger, "sha16": sha16(ledger), "read_at": now_iso(),
           "lines": lines, "rows": len(rows), "rem_total": rem_total,
           "unit": unit, "module": mod, "coverage": cover,
           "existence": exist, "consistency": consist_rep,
           "exempt_n": len(ctx.get("exempt", ())),
           "rc": 1 if bad else 0}
    return rep["rc"], rep


def render(rep):
    out = []
    out.append("── %s ──" % GATE_NAME)
    out.append("取数面 : %s" % rep["ledger"])
    out.append("现读时刻: %s · 账本 sha256[0:16] = %s" % (rep["read_at"], rep["sha16"]))
    out.append("账内读数: 行 %d · 对象 %d · 仍缺 %d（分子/分母唯一来源）"
               % (rep["lines"], rep["rows"], rep["rem_total"]))
    for tag, label, closed in (("unit", "甲 件面", WU_CLOSED), ("module", "乙 模块面", MOD_CLOSED)):
        a = rep[tag]
        out.append("%s : 有件桶 %d 个 Σ=%d ｜ 无件桶 %d 个 Σ=%d ｜ 桶和 %d ｜ 差 = %d %s"
                   % (label, a["buckets_you"], a["sum_you"], a["buckets_wu"], a["sum_wu"],
                      a["sum_all"], a["diff"], "✓" if a["diff"] == 0 and not a["anom"] else "✗"))
        if a["anom"]:
            out.append("   ✗ 键面缺失（不入桶 ⇒ 等式差就在这几行）· %d 条（GAP-id 作主指针）：" % len(a["anom"]))
            for ln, gid in a["anom"][:20]:
                out.append("      · %s（行 %s）· 缺 `%s` 键" % (gid, ln, a["key"]))
            if len(a["anom"]) > 20:
                out.append("      · …（只印前 20 条 · 共 %d 条）" % len(a["anom"]))
        if a["outside"]:
            out.append("   ✗ 闭集外形 · %d 个桶名不在闭集内：" % len(a["outside"]))
            for k in a["outside"][:20]:
                out.append("      · %r" % k)
        out.append("   现读桶名（无件面）: %s" % (" / ".join(repr(k) for k in a["wu_names"]) or "(无)"))
        if closed is not None:
            out.append("   闭集            : %s" % " / ".join(repr(k) for k in closed))
    c = rep["coverage"]
    out.append("丁 覆盖率（只报告）: 无件 %d / 仍缺 %d = %.1f%% ｜ 前三模块: %s"
               % (c["wu"], rep["rem_total"], c["ratio"] * 100,
                  " · ".join("%s %d" % (k, v) for k, v in c["top_modules"])))

    e = rep["existence"]
    out.append("戊 件面现读存在性 : 受检 %d 条（有件行）· 失效件 %d 条 · 存量豁免 %d 条 · 取数口 %s"
               % (e["checked"], len(e["hits"]), len(e["exempted"]), e["src"]))
    out.append("   现读清单        : %d 条（`git ls-files` 现跑 · 无缓存）" % e["tracked_n"])
    for ln, gid, u, src in e["hits"][:40]:
        out.append("   ✗ %s（行 %d）· 失效件 `%s`（unit_source=%s —— 现读清单里没有这条路径）"
                   % (gid, ln, u, src))
    if len(e["hits"]) > 40:
        out.append("   · …（只印前 40 条 · 共 %d 条）" % len(e["hits"]))
    for ln, gid, u in e["exempted"][:40]:
        out.append("   ○ %s（行 %d）· 存量豁免（**不进退码**）· `%s`" % (gid, ln, u))
    out.append("   存量豁免表      : %d 条（本表只增不删 · 逐条带 GAP-id 指针 · 表外才算新失效件）"
               % rep["exempt_n"])

    k = rep["consistency"]
    out.append("己 轴间一致性     : 受检 %d 条（有 unit 的行）· 不一致 %d 条 · 口径真源 %s（sha256[0:16]=%s）"
               % (k["checked"], len(k["hits"]), os.path.basename(k["src"]), k["src_sha16"]))
    for ln, gid, u, got, want, note in k["hits"][:40]:
        out.append("   ✗ %s（行 %d）· unit=`%s` 账里 module=%r ≠ 口径推导 %r" % (gid, ln, u, got, want))
        if note:
            out.append("      ↳ %s" % note)
    if len(k["hits"]) > 40:
        out.append("   · …（只印前 40 条 · 共 %d 条）" % len(k["hits"]))
    out.append("⇒ rc=%d %s" % (rep["rc"], {0: "PASS（甲/乙 两条等式全成立 · 戊/己 无新命中）",
                                           1: "FAIL（有不等 / 有闭集外形 / 有失效件 / 有轴间不一致）"
                                           }.get(rep["rc"], "")))
    return "\n".join(out)


# ─────────────────────────────────────────────────────────────────────────────
# list-rules
# ─────────────────────────────────────────────────────────────────────────────
def list_rules():
    print("── %s %s · 判据/闭集/取数口径 ──" % (TOOL, VERSION))
    print("判据甲  件面等式：按 `unit` 分桶，Σ(有件桶)+Σ(无件桶) == 账内 仍缺 总数")
    print("判据乙  模块面等式：同一把尺换 `module`")
    print("判据丙  桶名形制：无件桶名 ∈ 闭集；闭集外 ⇒ 逐条点名 rc=1")
    print("判据丁  覆盖率：无件 占 仍缺 比例 + 前三模块（**只报告 · 不判红**）")
    print("判据戊  件面现读存在性（5b）：有件行的 `unit` 必须在**现读** `git ls-files`（仓根现跑 · 无缓存）里；")
    print("        失效件逐条点名（GAP-id 主指针）⇒ rc=1；存量豁免表 `EXEMPT_EXISTENCE` 只打印不进退码（棘轮）")
    print("判据己  轴间一致性（5b）：有 `unit` 的行其 `module` 必须 == 由 `unit` 按 `gapModuleOfUnit` 口径推出的值；")
    print("        口径**现读**自真源 `core/cmd/zerg/family_gap.go`（三常量 + 手写占位前缀 + filepath.Dir 语义）")
    print("桶二分  有件 = 键面存在且值非空、不以 `无件` 开头 ； 无件 = 以 `无件` 开头 或 空串")
    print("        **键面缺失**（整个键不在行里）⇒ 不入任一桶 ⇒ 等式差 = 这类条数（逐条点名）")
    print("无件闭集: %s" % " / ".join(repr(k) for k in WU_CLOSED))
    print("闭集真源: check-gap-ledger-view.py 的 IMPACT_CLOSED = %s" % " / ".join(IMPACT_CLOSED))
    print("己 真源 : %s（现读 · 锚点缺一 ⇒ rc=2 不给结论）" % FAMILY_GAP_REL)
    print("戊 存量豁免表: %d 条（只增不删 · 逐条带 GAP-id 指针 · 表外才算新）· 首读现读失效件 0 条" % len(EXEMPT_EXISTENCE))
    print("退码   : 0 甲/乙 全成立且戊/己 无新命中 · 1 有不等/闭集外形/失效件/轴间不一致 · 2 取数面拿不到（BLOCKED，不绿）")
    print("取数面 : 账本 --ledger > --state-dir > env ZERG_STATE_DIR > ~/.zerg/state，账本名 = %s" % LEDGER_NAME)
    print("         仓根 --repo > env ZERG_REPO > 本件位置/现目录 `git rev-parse --show-toplevel`")
    print("点名口径: 一律以 **GAP-id** 作主指针（行号只作副信息 —— 账里行号会腐烂）")
    print("只读面 : 默认不写任何件；仅 `--out <外部目录>` 时落 json/md 两份")
    print("禁项   : 不把整份账打进 stdout（只汇总数 + 点名行）；代码无期望条数常量")


# ─────────────────────────────────────────────────────────────────────────────
# self-test（成对负控 · 合成账 · 不碰真目标 / 不碰 git）
# ─────────────────────────────────────────────────────────────────────────────
def _write_ledger(path, rows):
    with open(path, "w", encoding="utf-8") as fh:
        for r in rows:
            fh.write(json.dumps(r, ensure_ascii=False) + "\n")


def _row(rid, state=REMAIN, unit=None, module=None, drop=(), src=None):
    o = {"id": rid, "state": state, "symptom": "s", "prio": "P1", "impact": "命令面"}
    if unit is not None and "unit" not in drop:
        o["unit"] = unit
    if module is not None and "module" not in drop:
        o["module"] = module
    if src is not None:
        o["unit_source"] = src
    return o


# 自检用的**合成**口径与清单（真源件的逐字同义副本 · 只为让自检不碰 git / 不碰仓）：
#   三常量与前缀逐字取自 `core/cmd/zerg/family_gap.go` 的 `gapModuleOfUnit` 那一族。
FAKE_RULE = {"src": "(合成 · 自检用)", "sha16": "0" * 16, "no_file": "无件(命令面)",
             "none_mod": "无件", "root_mod": "仓根", "planned_prefix": "拟(", "root_dir_literal": "."}
FAKE_TRACKED = {"core/a.go"}          # 现读清单的合成面：只有这一条路径在盘上


def _ctx(tracked=FAKE_TRACKED, rule=FAKE_RULE, exempt=()):
    return {"tracked": tracked, "rule": rule, "exempt": exempt}


def run_probe(ledger_path, label, expect_rc, must_contain=(), ctx=None):
    rows, lines, unparsed = read_ledger(ledger_path)
    rc, rep = judge(rows, lines, unparsed, ledger_path, ctx if ctx is not None else _ctx())
    text = rep.get("blocked", "") if rc == 2 else render(rep)
    ok = (rc == expect_rc) and all(m in text for m in must_contain)
    print("  %-58s rc=%d 期望 %d  %s" % (label, rc, expect_rc, "✓" if ok else "✗"))
    if not ok:
        print("      ---- 实测输出 ----")
        for ln in text.splitlines()[:24]:
            print("      " + ln)
        if must_contain:
            for m in must_contain:
                if m not in text:
                    print("      缺片段: %r" % m)
    return ok, rc, rep, text


SELFTEST_N = 17          # 旧有 9 条 + 5b 新增 8 条（戊 负控/正控/存量豁免 · 己 负控/正控/真源边界 · 戊/己 取数面）


def self_test():
    tmp = tempfile.mkdtemp(prefix="gap-axis-selftest-")
    print("── %s --self-test（合成账 · 仓外临时目录 %s）──" % (TOOL, tmp))
    ok = True
    try:
        # ① 正控：10 条 仍缺（有件 6 · 无件 4）⇒ 两条等式全成立 · 戊/己 无新命中 ⇒ rc=0
        good = [_row("P%02d" % i, unit="core/a.go", module="core") for i in range(1, 7)] + \
               [_row("W%02d" % i, unit="无件(命令面)", module="无件") for i in range(1, 5)]
        p = os.path.join(tmp, "good.jsonl")
        _write_ledger(p, good)
        r, _rc, rep, text = run_probe(p, "① 正控：Σ有件 6 + Σ无件 4 == 总数 10 ⇒ rc=0", 0,
                                      ("Σ=6", "Σ=4", "差 = 0", "戊 件面现读存在性 : 受检 6 条", "失效件 0 条"))
        ok &= r

        # ② 负控（**坏账**）：同一份账，把「有件桶」里 3 条的 `unit` 键整个删掉
        #    ⇒ 这 3 条不入任一桶 ⇒ 桶和 7 ≠ 总数 10 ⇒ **必须 rc=1 且逐条点名差数 3**
        bad = [dict(x) for x in good]
        for x in bad[:3]:
            x.pop("unit", None)
        p = os.path.join(tmp, "bad.jsonl")
        _write_ledger(p, bad)
        r, _rc, rep, text = run_probe(p, "② 负控：删掉 3 条的 `unit` 键 ⇒ Σ 7 ≠ 10 ⇒ rc=1 + 点名", 1,
                                      ("差 = 3", "键面缺失", "P01", "P02", "P03"))
        ok &= (r and rep["unit"]["diff"] == 3)

        # ③ 负控：闭集外无件形态 ⇒ rc=1 逐条点名
        p = os.path.join(tmp, "outside.jsonl")
        _write_ledger(p, good + [_row("X01", unit="无件(乱写)", module="无件")])
        r, _rc, rep, text = run_probe(p, "③ 负控：无件桶名 `无件(乱写)` 在闭集外 ⇒ rc=1 点名", 1,
                                      ("闭集外形", "无件(乱写)"))
        ok &= r

        # ④ 正控：裸 `无件`（旧形态）+ 空串（"空"亦属无件形态）都落在闭集内 ⇒ rc=0
        #   ★ 真源口径（现读 `gapModuleOfUnit` 的字面边界）：裸 `无件` 不在真源那一支（真源只认
        #     `无件(命令面)`）⇒ 推成 `仓根`；空串 ⇒ `无件`。本格只考**件面桶二分**，故 module 取
        #     真源会写的那两个值（真源边界的牙齿另见 ⑰）。
        p = os.path.join(tmp, "bare.jsonl")
        _write_ledger(p, good + [_row("B01", unit="无件", module="仓根"),
                                 _row("B02", unit="", module="无件")])
        r, _rc, rep, text = run_probe(p, "④ 正控：裸 `无件` / 空串 皆落无件桶且在闭集内 ⇒ rc=0", 0,
                                      ("差 = 0", "'无件'"))
        ok &= (r and rep["unit"]["sum_wu"] == 6)

        # ⑤ 正控：`state` 不是 仍缺 的行 —— 不进分子亦不进分母 ⇒ 读数不变
        p = os.path.join(tmp, "otherstate.jsonl")
        _write_ledger(p, good + [_row("D01", state="已解", unit="core/z.go", module="core"),
                                 _row("D02", state="不做", unit="无件(命令面)", module="无件")])
        r, _rc, rep, text = run_probe(p, "⑤ 正控：state 非 仍缺 的行 不进分子亦不进分母 ⇒ rc=0", 0,
                                      ("仍缺 10", "差 = 0"))
        ok &= (r and rep["rem_total"] == 10)

        # ⑥ 负控（取数面）：整行 JSON 坏 ⇒ 必须 rc=2 BLOCKED（**不当绿**）
        p = os.path.join(tmp, "broken.jsonl")
        with open(p, "w", encoding="utf-8") as fh:
            fh.write('{"id":"Q1","state":"仍缺","unit":"core/a.go","module":"core"}\n')
            fh.write("}{ 这不是 json\n")
        r, _rc, rep, text = run_probe(p, "⑥ 负控：有行解不开 ⇒ rc=2 BLOCKED（不绿）", 2)
        ok &= r

        # ⑦ 负控（取数面）：账本不在盘 ⇒ rc=2
        r, _rc, rep, text = run_probe(os.path.join(tmp, "nope.jsonl"),
                                      "⑦ 负控：账本不在盘 ⇒ rc=2 BLOCKED", 2)
        ok &= r

        # ⑧ 负控（取数面）：空账 ⇒ rc=2（空转 = 假绿，不给结论）
        p = os.path.join(tmp, "empty.jsonl")
        _write_ledger(p, [])
        r, _rc, rep, text = run_probe(p, "⑧ 负控：0 行 ⇒ rc=2（空转不给结论）", 2)
        ok &= r

        # ⑨ 负控：模块面等式 —— 删掉 `module` 键 ⇒ 乙 差 = 3
        bad2 = [dict(x) for x in good]
        for x in bad2[:3]:
            x.pop("module", None)
        p = os.path.join(tmp, "badmod.jsonl")
        _write_ledger(p, bad2)
        r, _rc, rep, text = run_probe(p, "⑨ 负控：删掉 3 条的 `module` 键 ⇒ 乙 差 = 3 ⇒ rc=1", 1,
                                      ("差 = 3", "缺 `module` 键"))
        ok &= (r and rep["module"]["diff"] == 3)

        # ⑩ 戊 负控（**5b 新**）：合成一条 `unit` 指向**盘上不存在**的件 ⇒ rc=1 且点名那条 GAP-id
        p = os.path.join(tmp, "exist-hit.jsonl")
        _write_ledger(p, good + [_row("GAP-20260928-9001", unit="scripts/gates/不存在的件.py",
                                      module="scripts/gates", src="auto")])
        r, _rc, rep, text = run_probe(p, "⑩ 戊 负控：unit 指向不存在件 ⇒ rc=1 且点名 GAP-id", 1,
                                      ("GAP-20260928-9001", "失效件 1 条", "失效件 `scripts/gates/不存在的件.py`"))
        ok &= (r and len(rep["existence"]["hits"]) == 1)

        # ⑪ 戊 正控（**5b 新**）：`unit` 为**无件闭集内**值 ⇒ **不进戊**（受检 0 条 · 不判红）
        p = os.path.join(tmp, "exist-wu.jsonl")
        _write_ledger(p, [_row("GAP-20260928-9002", unit="无件(命令面)", module="无件", src="hand")])
        r, _rc, rep, text = run_probe(p, "⑪ 戊 正控：unit 为无件闭集内值 ⇒ 不进戊（受检 0）⇒ rc=0", 0,
                                      ("受检 0 条（有件行）", "失效件 0 条"))
        ok &= (r and rep["existence"]["checked"] == 0)

        # ⑫ 己 负控（**5b 新**）：合成一条 `module` 与 `unit` 矛盾 ⇒ rc=1 且点名那条 GAP-id
        p = os.path.join(tmp, "consist-hit.jsonl")
        _write_ledger(p, good + [_row("GAP-20260928-9003", unit="core/a.go", module="scripts",
                                      src="auto")])
        r, _rc, rep, text = run_probe(p, "⑫ 己 负控：module 与 unit 矛盾 ⇒ rc=1 且点名", 1,
                                      ("GAP-20260928-9003", "不一致 1 条", "账里 module='scripts' ≠ 口径推导 'core'"))
        ok &= (r and len(rep["consistency"]["hits"]) == 1)

        # ⑬ 己 正控（**5b 新**）：一致行（含仓根件 `仓根` 那一支 + 无件行 `无件` 那一支）⇒ rc=0
        p = os.path.join(tmp, "consist-ok.jsonl")
        _write_ledger(p, good + [_row("GAP-20260928-9004", unit="README.md", module="仓根", src="auto"),
                                 _row("GAP-20260928-9005", unit="无件(命令面)", module="无件", src="hand")])
        r, _rc, rep, text = run_probe(p, "⑬ 己 正控：module 与口径推导一致（含 仓根/无件 两支）⇒ rc=0", 0,
                                      ("不一致 0 条", "受检 12 条（有 unit 的行）"),
                                      ctx=_ctx(tracked={"core/a.go", "README.md"}))
        ok &= (r and not rep["consistency"]["hits"])

        # ⑭ 戊 存量豁免（**5b 新**）：同一条失效件，进了豁免表 ⇒ 只打印、**不进退码**（rc=0）
        p = os.path.join(tmp, "exist-exempt.jsonl")
        _write_ledger(p, good + [_row("GAP-20260928-9006", unit="scripts/gates/存量差件.py",
                                      module="scripts/gates", src="auto")])
        r, _rc, rep, text = run_probe(p, "⑭ 戊 存量豁免：失效件进表 ⇒ 打印但不进退码 ⇒ rc=0", 0,
                                      ("失效件 0 条", "存量豁免 1 条", "○ GAP-20260928-9006", "不进退码"),
                                      ctx=_ctx(exempt=(("GAP-20260928-9006", "scripts/gates/存量差件.py"),)))
        ok &= (r and len(rep["existence"]["exempted"]) == 1 and not rep["existence"]["hits"])

        # ⑮ 取数面（**5b 新**）：现读清单拿不到 ⇒ rc=2（**不给绿也不给红**）
        r, _rc, rep, text = run_probe(p, "⑮ 负控：戊 取数面（ls-files）拿不到 ⇒ rc=2 不给结论", 2,
                                      ctx={"tracked": None, "rule": FAKE_RULE, "exempt": (),
                                           "tracked_err": "合成：git ls-files 拿不到"})
        ok &= r

        # ⑯ 取数面（**5b 新**）：真源口径锚点没命中 ⇒ rc=2
        r, _rc, rep, text = run_probe(p, "⑯ 负控：己 真源口径没现读出来 ⇒ rc=2 不给结论", 2,
                                      ctx={"tracked": FAKE_TRACKED, "rule": None, "exempt": (),
                                           "rule_err": "合成：真源口径锚点缺"})
        ok &= r

        # ⑰ 己 真源口径边界（**5b 新**）：闭集里的**其他**无件形态（裸 `无件` / `无件(<别的面>)`）
        #    在真源 `gapModuleOfUnit` 里**不在** `无件` 那一支 ⇒ 推成 `仓根`；账里写 `无件` ⇒ 命中。
        #    ★ 这一格钉的是「真源口径的字面边界」，命中行会**带说明**（不静默 · 不让人误读成账写错）。
        #    现读真账 391 条无件行的 unit **全是** `无件(命令面)` ⇒ 真账上这一档**零命中**。
        p = os.path.join(tmp, "consist-nuance.jsonl")
        _write_ledger(p, good + [_row("GAP-20260928-9007", unit="无件(门禁面)", module="无件", src="hand")])
        r, _rc, rep, text = run_probe(p, "⑰ 己 真源口径边界：`无件(门禁面)`+module`无件` ⇒ rc=1 带说明", 1,
                                      ("GAP-20260928-9007", "不一致 1 条", "真源口径边界",
                                       "⇒ `仓根`", "闭集内其他无件形态不在真源这一支里"))
        ok &= (r and len(rep["consistency"]["hits"]) == 1)

    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("⇒ 自检 %s（%d 条断言 · 旧有 9 条 + 5b 新增 8 条）" % ("PASS" if ok else "FAIL", SELFTEST_N))
    return 0 if ok else 1


# ─────────────────────────────────────────────────────────────────────────────
def main(argv):
    ap = argparse.ArgumentParser(description=GATE_NAME)
    ap.add_argument("--ledger", help="账本件（默认按取数面推）")
    ap.add_argument("--state-dir", dest="state_dir", help="状态目录（默认 env ZERG_STATE_DIR / ~/.zerg/state）")
    ap.add_argument("--repo", help="仓根（默认 env ZERG_REPO / 现跑 git rev-parse）—— 戊/己 的取数面")
    ap.add_argument("--out", help="**外部**输出目录（给了才落 json/md 两份）")
    ap.add_argument("--json", action="store_true", help="末尾追加机器面一行")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.list_rules:
        list_rules()
        return 0
    if a.self_test:
        return self_test()

    ledger = resolve_ledger(a)
    root = resolve_repo(a)
    tracked, tracked_err = git_tracked(root)
    rule, rule_err = load_modrule(root)
    ctx = {"tracked": tracked, "rule": rule, "exempt": EXEMPT_EXISTENCE,
           "tracked_err": tracked_err, "rule_err": rule_err}
    rows, lines, unparsed = read_ledger(ledger)
    rc, rep = judge(rows, lines, unparsed, ledger, ctx)
    if rc == 2:
        print("── %s ──" % GATE_NAME)
        print("取数面 : %s" % ledger)
        print("现读时刻: %s" % now_iso())
        print("⇒ rc=2 BLOCKED（取数面拿不到）：%s" % rep.get("blocked"))
        for ln, why in (rep.get("unparsed") or [])[:20]:
            print("   · 行 %s · %s" % (ln, why))
        if a.json:
            print(json.dumps({"tool": TOOL, "rc": 2, "blocked": rep.get("blocked"),
                              "ledger": ledger, "read_at": now_iso()}, ensure_ascii=False))
        return 2

    print(render(rep))
    if a.out:
        os.makedirs(a.out, exist_ok=True)
        with open(os.path.join(a.out, "gap-axis-reconcile.json"), "w", encoding="utf-8") as fh:
            json.dump(rep, fh, ensure_ascii=False, indent=1, sort_keys=True)
            fh.write("\n")
        with open(os.path.join(a.out, "gap-axis-reconcile.md"), "w", encoding="utf-8") as fh:
            fh.write(render(rep) + "\n")
        print("外部落盘: %s/{gap-axis-reconcile.json,gap-axis-reconcile.md}" % a.out)
    if a.json:
        j = {k: rep[k] for k in ("sha16", "read_at", "lines", "rows", "rem_total", "rc")}
        j["tool"] = TOOL
        j["unit"] = {k: rep["unit"][k] for k in ("buckets_you", "buckets_wu", "sum_you", "sum_wu", "diff")}
        j["module"] = {k: rep["module"][k] for k in ("buckets_you", "buckets_wu", "sum_you", "sum_wu", "diff")}
        j["coverage_wu_ratio"] = round(rep["coverage"]["ratio"], 4)
        j["existence_checked"] = rep["existence"]["checked"]
        j["existence_hits"] = len(rep["existence"]["hits"])
        j["existence_exempted"] = len(rep["existence"]["exempted"])
        j["consistency_checked"] = rep["consistency"]["checked"]
        j["consistency_hits"] = len(rep["consistency"]["hits"])
        j["modrule_src_sha16"] = rep["consistency"]["src_sha16"]
        print(json.dumps(j, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

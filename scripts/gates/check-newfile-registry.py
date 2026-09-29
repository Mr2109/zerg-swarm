#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-newfile-registry.py —— 门76 **新件登记完整性**：件级 × 六张登记表（**只报告** · 起手不接闸）

一句话：**一张新落的脚本件，逐面问「这张表里有没有它」**——缺哪张表就逐条点名。
（域级对拍已由门57 `check-marks-coverage-parity.py` 做 ⇒ **本门只补件级**，不重造域级。）

────────────────────────────────────────────────────────────────────────────
分母（**写死 · 本件唯一的集合定义，改这里 = 改判据**）

  分母 = **盘上现读的「新件」集合** = `scan_ledger_face(root)` 现读件集 ∖ **冻结基线集合**。

  · `scan_ledger_face` 的口径 = `core/cmd/zerg/script_inventory_test.go:28-32`（13 个排除面）
    ＋ `:35-60 scanScripts`（`scripts/` 面 `.sh` / `.py` / **可执行无后缀且首行 `#!`**）
    ⇒ **与台账面同源**（面④ 就是按这个口径记的账）。
  · **禁 mtime**（checkout / touch / rsync 会污染它）· **禁写死名单**（写死名单第二天就过期，
    正是 GAP-20260928-69 的病根）⇒ 「新」只由**基线的差**定义。
  · 基线 = 上一趟只报告档落下的**逐件名录**（生成件 · `--dump-baseline <路径>` 生成 · 仓外）。
    基线不在盘 ⇒ **rc=2 判不了**（本门**不替**基线猜一个「全盘都是新件」的分母）。
  · 分母 **0 件 ⇒ rc=2 不给结论**（照门① `check-gate-coverage.py:704-706` 自己的口径）——
    「真的没有新件」与「判不了」是两态，必须分得开。
  · 口径差栏（**照实印，不并入判红**）：台账口径（上）与门① 候选面口径（`gate-coverage.config.json`
    的 `scan.script_globs` + `nosuffix_dir`）**不相等** ⇒ 两集的对差逐条点名。本门**分母取台账口径**，
    门① 候选面口径只作**参照读数**（不选它，是为了不与面①③⑤ 的「该不该接门」定义面抢活儿）。

六张登记表（**只读** · 一张不写）

  R1 ① `scripts/公开标记.tsv` —— 件在「文件」列？
  R2 ② 门⑩ 的干跑登记表 —— **仅当件正文命中该门自己的扫描口径**才要求登记。**口径与件名一律现读**：
        步骤表里那条干跑步的 .py ⇒ 载入该件 ⇒ 用它的常量（登记表路径）与它的扫描函数（现跑命中集）
        ⇒ 本件**不写死那个件名、也不另抄一份正则**（单一真源）；副作用照实写：本件因此**不落进**该口径的扫描面
        ⇒ 面② 对本件 = 不适用（真读数见「自指」那一行）。
  R3 ③ 门① 两张真源 —— 件 ∈ 步骤表 `add_step` 点名 **或** ∈ `whitelist.scripts`（且带 reason+date）
  R4 ④ **脚本台账**（**仓外真源** · 本单**不写**）—— 件 ∈ 台账 tsv 行集？（`--inventory <tsv>`）
  R5 ⑤ 公开链两张排除表 —— 件判 `public=no` ⇒ 件 ∈ `EXCLUDES` **且** ∈ `DROP_EXACT`（两器同批）
  R6 ⑥ **失败文案面**（**单列旁栏 · 不进退码**）—— 门① 的处置文案是否已指到**白名单档**（reason+date），
        而不是只指 `add_step`。

**边界（写死 · 防假红 · 一条不漏地写在这里）**

  · **R4 / R6 **不进退码**（单列旁栏 · 计数可见）**：R4 真源在**仓外**、写入口是 D2 写面（需人签）⇒
    本门只报「台账 tsv 里有没有它」；R6 是「处置指路」问题 ⇒ 两格都不并入「缺登记」口径。
  · **R6 不进退码**：它是「处置指路」问题，不是「登记」问题 ⇒ 单列旁栏、计数可见（分母 = 五张登记表）。
  · **弱挂载档不进判红**：件名只以 **basename** 出现在 `add_step` 行里（同名兄弟提及，例
    `scripts/build/start-zerg-core.sh` ⤙ 顶层 `./start-zerg-core.sh`）⇒ 计「弱」档、逐条点名，
    **不判红**（口径同 `check-gate-coverage.py` 的 tier ②）—— 它是「可能与代码同批接线」的疑点，
    不是缺登记。
  · **通用语法步 glob 不算接线**：被 `scripts/*.sh|*.py` 语法步兜住的件**不因此免登记**
    （门① 第四档同规：语法过 ≠ 接了门）。
  · **未跟踪件单列一栏、不并入判红**：`git ls-files` 看不见的件（本仓现读 3 件）落在门① ②白名单
    豁免格 ⇒ 若把「未跟踪」当「新件」，首跑就报假红。
  · **不判 `publish/docs/**` 内容、不判公开产物树、不判外链**：只判「这张表里有没有这件」。
  · **一件一笔**：一格一条 ⇒ 报「件 × 缺哪张表」；**跳过 / 丢件必须计数**（口径 `R-19`）。
  · **本件只读**：不改任何登记表、不跑 `zerg script inventory sync`（那是 D2 写面 · 需人签）。

退码（**只报告档** · 与 `check-registry-vs-disk.py` / `check-docs-orphans-links.py` 同规）

  0 = 判到了（逐格给数 + 逐条点名；**报告档不阻断**）
  1 = 只有 `--strict` 档才可能：R1/R2/R3/R5 任一格 > 0（逐条点名；R4 / R6 = 旁栏不进退码）
  2 = **不给结论**：分母 0 件 · 基线不在盘 · 任一登记表读不到 / 解析出 0 条（空转 = 假绿）·
      **任一面判不了**（含面④ 仓外真源不在 · 面⑥ 判据件不在）· `--self-test` 不过

用法

  python3 scripts/gates/check-newfile-registry.py                       # 查本仓（报告档）
  python3 scripts/gates/check-newfile-registry.py --dump-baseline /仓外/基线.tsv   # 生成冻结基线
  python3 scripts/gates/check-newfile-registry.py --baseline /仓外/基线.tsv --inventory /仓外/台账.tsv
  python3 scripts/gates/check-newfile-registry.py --strict --json faces,self
  python3 scripts/gates/check-newfile-registry.py --repo <仓根> --new-file scripts/x.sh   # 指分母（两态取证用）
  python3 scripts/gates/check-newfile-registry.py --self-test
"""

import argparse
import glob as globmod
import importlib.util
import json
import os
import re
import shutil
import sys
import tempfile

# 本门禁**自己**所在的仓（`--repo` 只改被查的仓，不改这个）
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
SELF_REL = os.path.join("scripts", "gates", "check-newfile-registry.py")

MARKS_REL = os.path.join("scripts", "公开标记.tsv")
STEPS_REL = os.path.join("scripts", "gates", "precommit-gates.sh")
COV_REL = os.path.join("scripts", "gates", "gate-coverage.config.json")
COV_JUDGE_REL = os.path.join("scripts", "gates", "check-gate-coverage.py")
# 门⑩ 的登记表路径**不写死**：从步骤表那条干跑步的判据件里现读（见 load_dry_module）
PUB_REL = os.path.join("scripts", "build", "publish-public.sh")
MIRROR_REL = os.path.join("publish", "mirror-public-lib.py")

# 面④ 真源在**仓外**姊妹仓（生成物 · 不是手维件）：同级 `Zerg-内部文档/项目文档/<版本>/清单-脚本现状-*.tsv`
LEDGER_SIBLING = "Zerg-内部文档"
LEDGER_GLOB = os.path.join("项目文档", "*", "清单-脚本现状-*.tsv")

# 台账/扫描口径的排除面（逐字对齐 `script_inventory_test.go:28-32`）
SCRIPT_EXCLUDES = {
    "target", "node_modules", "dist", "bin", "vendor", "data", ".git", ".venv", "venv",
    ".build", "zerg-wt", "_history", "__pycache__",
}

# 默认基线落点（**仓外** · 生成件）：家目录缓存，不落仓、不进 git
DEFAULT_BASELINE = os.path.join(os.path.expanduser("~"), ".zerg", "newfile-registry-baseline.tsv")


def rel(root, p):
    return os.path.relpath(p, root).replace(os.sep, "/")


def read_text(path):
    with open(path, encoding="utf-8", errors="replace") as fh:
        return fh.read()


# ── 分母：两套现读口径（都被复用在盘的定义，不另立第三套）──────────────────────
def scan_ledger_face(root):
    """台账口径（= `scanScripts` · `script_inventory_test.go:28-60`）。返回 rel 集；面不在 ⇒ None。"""
    base = os.path.join(root, "scripts")
    if not os.path.isdir(base):
        return None
    out = set()
    for dirpath, dirnames, filenames in os.walk(base):
        dirnames[:] = sorted(d for d in dirnames if d not in SCRIPT_EXCLUDES)
        for name in sorted(filenames):
            p = os.path.join(dirpath, name)
            is_script = name.endswith(".sh") or name.endswith(".py")
            if not is_script:
                if not os.access(p, os.X_OK):
                    continue
                try:
                    if not read_text(p).startswith("#!"):
                        continue
                except OSError:
                    continue
            out.add(rel(root, p))
    return out


def scan_gate1_face(root):
    """门① 候选面口径（= `gate-coverage.config.json` 的 `scan.script_globs` + `nosuffix_dir`）。"""
    cfg = os.path.join(root, COV_REL)
    if not os.path.isfile(cfg):
        return None, None
    try:
        data = json.loads(read_text(cfg))
    except ValueError:
        return None, None
    scan = data.get("scan") or {}
    globs = list(scan.get("script_globs") or [])
    if not globs:
        return None, data
    comps = set(((data.get("exclude") or {}).get("path_components")) or [])
    out = set()
    for g in globs:
        for p in globmod.glob(os.path.join(root, g)):
            if not os.path.isfile(p):
                continue
            r = rel(root, p)
            if any(c in r.split("/") for c in comps):
                continue
            out.add(r)
    ns = scan.get("nosuffix_dir")
    if ns:
        d = os.path.join(root, ns)
        if os.path.isdir(d):
            for name in sorted(os.listdir(d)):
                p = os.path.join(d, name)
                if os.path.isfile(p) and "." not in name:
                    out.add(rel(root, p))
    return out, data


# ── 六张表：逐张现读（只读 · 解析失败 ⇒ 面判不了，不静默）──────────────────────
def load_marks(root):
    """① `公开标记.tsv` ⇒ {rel: public}；读不到 / 0 行 ⇒ None。"""
    p = os.path.join(root, MARKS_REL)
    if not os.path.isfile(p):
        return None
    rows = {}
    for i, line in enumerate(read_text(p).splitlines()):
        if not line.strip() or i == 0:
            continue
        cols = line.split("\t")
        if len(cols) < 2:
            continue
        rows[cols[0].strip()] = cols[1].strip()
    return rows or None


def collect_paths(obj):
    """把上游判据件的扫描返回**按形状**取出「仓根相对件路径集」（不写死它的返回形状）。"""
    out = set()

    def walk(x):
        if isinstance(x, str):
            out.add(x)
        elif isinstance(x, (set, frozenset, list, tuple)):
            for i in x:
                walk(i)
        elif isinstance(x, dict):
            for k in x:
                walk(k)
    walk(obj)
    return out


def load_dry_module(root):
    """面② 的**单一真源**：从步骤表现读「那条干跑步」的判据件，载入它（不写死件名、不另抄正则）。"""
    _lines, text = load_steps(root)
    if text is None:
        return None, "步骤表读不到"
    for ln in text.splitlines():
        if "add_step" not in ln or "干跑" not in ln:
            continue
        m = re.search(r"scripts/[A-Za-z0-9_/.-]+\.py", ln)
        if not m:
            continue
        cand = os.path.join(root, m.group(0))
        if not os.path.isfile(cand):
            continue
        spec = importlib.util.spec_from_file_location("_gnr_dry_judge", cand)
        if spec is None or spec.loader is None:
            return None, "判据件载入不了：%s" % m.group(0)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        return mod, None
    return None, "步骤表里找不到「干跑」那条步的判据件（.py）"


def dry_registry(root, mod):
    """登记集：从该判据件自己声明的登记表路径现读（本件不写死那个件名）。返回 (登记集, 表路径, 问题)。"""
    relp = getattr(mod, "REGISTRY_REL", None)
    if not relp:
        return None, None, "判据件未声明登记表路径（REGISTRY_REL）"
    path = os.path.join(root, relp)
    if not os.path.isfile(path):
        return None, relp, "登记表不在盘：%s" % relp
    try:
        data = json.loads(read_text(path))
    except ValueError as e:
        return None, relp, "登记表解析失败：%s" % e
    reg = set()
    for e in (data.get("件") or []):
        if isinstance(e, dict) and e.get("file"):
            reg.add(e["file"])
    return (reg or None), relp, None


def load_steps(root):
    """③a 步骤表：返回 (add_step 行列表, 全文)；缺 ⇒ (None, None)。"""
    p = os.path.join(root, STEPS_REL)
    if not os.path.isfile(p):
        return None, None
    text = read_text(p)
    lines = [ln for ln in text.splitlines() if "add_step" in ln]
    return lines, text


def load_whitelist_scripts(root):
    """③b 门① 白名单：返回 (条目列表, 问题列表)；缺 ⇒ (None, [问题])。缺 reason/date ⇒ 判不了（照门① rc=2）。"""
    p = os.path.join(root, COV_REL)
    if not os.path.isfile(p):
        return None, ["面③ 判不了：%s 不在盘" % COV_REL]
    try:
        data = json.loads(read_text(p))
    except ValueError as e:
        return None, ["面③ 判不了：%s 解析失败（%s）" % (COV_REL, e)]
    items = ((data.get("whitelist") or {}).get("scripts")) or []
    problems = []
    for it in items:
        if not it.get("reason") or not it.get("date"):
            problems.append("面③ 白名单条目缺 reason/date：%s" % it.get("path"))
    return items, problems


def load_excludes(root):
    """⑤a `EXCLUDES=( … )`（活跃条目）。缺 / 0 条 ⇒ None。"""
    p = os.path.join(root, PUB_REL)
    if not os.path.isfile(p):
        return None
    text = read_text(p)
    m = re.search(r"EXCLUDES=\((.*?)\n\)", text, re.S)
    if not m:
        return None
    out = set()
    for ln in m.group(1).splitlines():
        s = ln.strip()
        if not s or s.startswith("#"):
            continue
        s = s.strip('"').strip("'")
        if s:
            out.add(s)
    return out or None


def load_drop_exact(root):
    """⑤b `DROP_EXACT = { … }`（精确匹配集）。缺 / 0 条 ⇒ None。（正解解析器在 Go 侧 `family_publish_set.go`，本件只读同一形状。）"""
    p = os.path.join(root, MIRROR_REL)
    if not os.path.isfile(p):
        return None
    m = re.search(r"DROP_EXACT\s*=\s*\{(.*?)\n\}", read_text(p), re.S)
    if not m:
        return None
    out = set()
    for ln in m.group(1).splitlines():
        s = ln.split("#")[0].strip().rstrip(",")
        if not s:
            continue
        out.add(s.strip('"').strip("'"))
    return out or None


def load_ledger_rows(root, inv):
    """④ 仓外台账 tsv 行集。路径缺 ⇒ None（判不了 · 需人签）。"""
    path = inv
    if path and not os.path.isabs(path):
        path = os.path.join(root, path)
    if path is None:
        sib = os.path.join(os.path.dirname(os.path.abspath(root)), LEDGER_SIBLING)
        cand = sorted(globmod.glob(os.path.join(sib, LEDGER_GLOB)))
        path = cand[-1] if cand else None
    if not path or not os.path.isfile(path):
        return None, path
    rows = set()
    for i, line in enumerate(read_text(path).splitlines()):
        if not line.strip() or i == 0:
            continue
        rows.add(line.split("\t")[0].strip())
    return (rows or None), path


def load_wording(root):
    """⑥ 面⑥：门① 判据件的「处置」文案是否已指到白名单档。返回 (是否已改, 逐字文案)；缺 ⇒ (None, None)。"""
    p = os.path.join(root, COV_JUDGE_REL)
    if not os.path.isfile(p):
        return None, None
    text = read_text(p)
    lines = text.splitlines()
    blocks = []
    for i in [j for j, ln in enumerate(lines) if "处置" in ln]:
        blk = [ln.strip() for ln in lines[max(0, i - 1):i + 4]]
        blob = " ".join(blk)
        if "处置" in blob:
            blocks.append((blk, blob))
    # 受判块（写死口径）= 逐字提到 `add_step` / 「挂门」的处置块（= 新件那条的处置指路）
    rel = [(blk, blob) for blk, blob in blocks if "add_step" in blob or "挂门" in blob]
    if not rel:
        return None, None
    fixed = all(("白名单" in blob) and ("reason" in blob) for _blk, blob in rel)
    return fixed, [next((x for x in blk if "处置" in x), blk[0]) for blk, _ in rel][:4]


# ── 主判据 ────────────────────────────────────────────────────────────────
def audit(root, new_files=None, inv=None, baseline=None):
    res = {"counts": None, "faces": {}, "problems": [], "not_judged": [], "roster": None}
    ledger_face = scan_ledger_face(root)
    if ledger_face is None:
        res["problems"].append("扫面 0 件：`%s` 不在盘 ⇒ 空转 = 假绿" % os.path.join("scripts"))
        return res
    res["roster"] = sorted(ledger_face)
    gate1_face, _ = scan_gate1_face(root)
    res["faces"]["口径差"] = {
        "台账口径": len(ledger_face),
        "门①候选面": None if gate1_face is None else len(gate1_face),
        "对差": None if gate1_face is None else sorted(ledger_face ^ gate1_face),
    }

    # 分母
    if new_files is None:
        if not baseline or not os.path.isfile(baseline):
            res["problems"].append(
                "分母判不了：冻结基线不在盘（%s）⇒ 不给结论；先跑 `--dump-baseline <仓外路径>`"
                % (baseline or DEFAULT_BASELINE))
            return res
        base = set()
        for ln in read_text(baseline).splitlines():
            ln = ln.strip()
            if ln and not ln.startswith("#"):
                base.add(ln)
        new_files = sorted(ledger_face - base)
    else:
        new_files = sorted(set(new_files))

    def cnt(name, val):
        res["faces"][name] = val

    marks = load_marks(root)
    dry_mod, dry_err = load_dry_module(root)
    dry_hits, dry_reg, dry_rel, dry_problem = None, None, None, dry_err
    if dry_mod is not None:
        _r = dry_mod.scan_hits(root)
        if _r is None or (isinstance(_r, tuple) and _r and _r[0] is None):
            dry_problem = "扫描空转 / 不可判：%s" % (
                _r[1] if isinstance(_r, tuple) and len(_r) > 1 else _r)
        else:
            dry_hits = collect_paths(_r)
            dry_reg, dry_rel, dry_problem = dry_registry(root, dry_mod)
    step_lines, step_text = load_steps(root)
    wl_items, wl_problems = load_whitelist_scripts(root)
    excludes = load_excludes(root)
    drops = load_drop_exact(root)
    led_rows, led_path = load_ledger_rows(root, inv)
    fixed, wording = load_wording(root)

    for nm, ok, why in (("①公开标记表", marks is not None, MARKS_REL),
                        ("②干跑登记", dry_hits is not None and dry_reg is not None,
                         dry_rel or (dry_problem or "门⑩ 登记表")),
                        ("③门①两张真源", step_lines is not None and wl_items is not None, STEPS_REL),
                        ("⑤两器排除表", excludes is not None and drops is not None, PUB_REL)):
        if not ok:
            res["problems"].append("面%s 判不了：%s 读不到 / 解析出 0 条（真源读不全 ⇒ 绿就是假绿）" % (nm, why))
    for p in wl_problems:
        res["problems"].append(p)
    if led_rows is None:
        res["not_judged"].append({
            "face": "④台账面", "item": led_path or "（同级 Zerg-内部文档 下无 清单-脚本现状-*.tsv）",
            "why": "仓外真源 · 本门**不写**（唯一写入口 `zerg script inventory sync` = 危险档 D2 写面 · 需人签）"})
    if fixed is None:
        res["not_judged"].append({"face": "⑥文案面", "item": COV_JUDGE_REL, "why": "判据件不在盘 / 找不着处置文案"})

    r1, r2, r3, r5, weak, fourth, dry_na = [], [], [], [], [], [], []
    for f in new_files:
        # ①
        if marks is not None and f not in marks:
            r1.append(f)
        # ② 仅当件正文命中该表自报的扫描口径
        if dry_hits is not None and dry_reg is not None:
            if f in dry_hits and f not in dry_reg:
                r2.append(f)
            elif f not in dry_hits:
                dry_na.append(f)
        # ③
        if step_lines is not None and wl_items is not None:
            wl_paths = {it.get("path") for it in wl_items}
            strong = any(f in ln for ln in step_lines)
            weakm = any(os.path.basename(f) in ln for ln in step_lines)
            if f in wl_paths:
                pass
            elif strong:
                pass
            elif weakm:
                weak.append(f)
            elif step_text is not None and (os.path.basename(f) in step_text):
                fourth.append(f)
            else:
                r3.append(f)
        # ⑤ 仅当件判 public=no
        if marks is not None and excludes is not None and drops is not None:
            if marks.get(f) == "no" and not (f in excludes and f in drops):
                r5.append(f)
    r4 = None if led_rows is None else [f for f in new_files if f not in led_rows]
    res["counts"] = {"R1": len(r1), "R2": len(r2), "R3": len(r3),
                     "R4": None if r4 is None else len(r4), "R5": len(r5),
                     "R6": (None if fixed is None else (0 if fixed else 1))}
    res["items"] = {"R1": r1, "R2": r2, "R3": r3, "R4": r4 or [], "R5": r5}
    res["weak"] = weak
    res["fourth_tier"] = fourth
    res["dry_na"] = dry_na
    res["new_files"] = new_files
    res["dry_hits"] = dry_hits
    res["ledger"] = {"path": led_path, "rows": None if led_rows is None else len(led_rows)}
    res["wording"] = {"fixed": fixed, "lines": wording}
    res["marks_public_no"] = marks
    return res


def run(root, new_files, inv, strict, baseline=None):
    res = audit(root, new_files=new_files, inv=inv, baseline=baseline)
    print("门76 新件登记完整性（件级 × 六张登记表 · **只报告**）")
    print("  仓根：%s" % root)
    d = res["faces"].get("口径差") or {}
    print("  口径差栏：台账口径 %s 件 · 门①候选面 %s 件 · 对差 %s 件（参照读数 · 不并入判红）"
          % (d.get("台账口径"), d.get("门①候选面"),
             "判不了" if d.get("对差") is None else len(d["对差"])))
    if d.get("对差"):
        for f in d["对差"][:40]:
            print("     · 口径差 %s" % f)
    if res["counts"] is None:
        for p in res["problems"]:
            print("  ✗ %s" % p)
        print("  判据结论：**判不了 ⇒ 不给结论（rc=2）**")
        return 2
    c = res["counts"]
    print("  分母（现读新件 ∖ 冻结基线）：%d 件" % len(res["new_files"]))
    for f in res["new_files"][:40]:
        print("     - %s" % f)
    print("  面① 公开标记表缺登记：%d" % c["R1"])
    for f in res["items"]["R1"]:
        print("     ✗ %s  ← 表里无此行（补 `scripts/公开标记.tsv` 一件一行 · public=no 也要写理由）" % f)
    print("  面② 干跑登记缺件：%d（口径 = 门⑩ 判据件自己的扫描函数（现读载入）· 不适用 %d 件）"
          % (c["R2"], len(res["dry_na"])))
    for f in res["items"]["R2"]:
        print("     ✗ %s  ← 正文命中干跑口径却未登记（补该登记表的 件[].file）" % f)
    print("  面③ 门① 两张真源缺件：%d（弱挂载 %d 件 · 通用语法步兜住 %d 件 —— 均不判红）"
          % (c["R3"], len(res["weak"]), len(res["fourth_tier"])))
    for f in res["items"]["R3"]:
        print("     ✗ %s  ← 既不在 add_step 点名、也不在 whitelist.scripts（补 reason+date 白名单档）" % f)
    for f in res["weak"]:
        print("     · 弱 %s  ← 仅 basename 出现在 add_step 行（疑点 · 不判红）" % f)
    led = res["ledger"]
    if c["R4"] is None:
        print("  面④ 脚本台账：**判不了**（真源 %s）" % (led["path"] or "不在盘"))
    else:
        print("  面④ 脚本台账缺登记：%d（台账 %s 行 · **单列旁栏 · 不进退码** · 写入口 = 仓外 `zerg script inventory sync` D2 需人签）"
              % (c["R4"], led["rows"]))
        for f in res["items"]["R4"]:
            print("     ✗ %s  ← 台账 tsv 无此行（治法 = 人签跑 sync · 本门只读不写）" % f)
    for nj in res["not_judged"]:
        print("     ✗判不了 %s · %s —— %s" % (nj["face"], nj["item"], nj["why"]))
    print("  面⑤ 公开链两器排除表：%d（仅判 public=no 的件 · 须两器同批）" % c["R5"])
    for f in res["items"]["R5"]:
        print("     ✗ %s  ← 表判 no，两器排除表没收它（`EXCLUDES` 与 `DROP_EXACT` 两处都要）" % f)
    w = res["wording"]
    if c["R6"] is None:
        print("  面⑥ 失败文案面：**判不了**（判据件不在盘）")
    else:
        print("  面⑥ 失败文案面（**单列旁栏 · 不进退码**）：%d" % c["R6"])
        for ln in (w["lines"] or []):
            print("     %s %s" % ("✓" if w["fixed"] else "✗", ln))
        if not w["fixed"]:
            print("     ⇒ 文案仍只指 add_step；对「语料在仓外」与「写盘型」两类件是禁路 ⇒ 应改指白名单档（reason+date）")
    self_in = SELF_REL in res["new_files"]
    print("  ★ 自指：本门自身 %s —— 在分母内：%s · 面①：%s · 面②：%s · 面③：%s · 面④：见上「面④」那几行（台账 tsv 里暂无它 · 唯一写入口是仓外 D2 写面 · 需人签）· 面⑤：%s"
          % (SELF_REL, "是" if self_in else "否（本趟不在新件集）",
             ("已登记（public=%s）" % res["marks_public_no"][SELF_REL])
             if res["marks_public_no"] and SELF_REL in res["marks_public_no"] else "未登记",
             "不适用（正文不在该口径的扫描面内）"
             if not (res.get("dry_hits") and SELF_REL in res["dry_hits"]) else "需登记",
             "缺登记" if SELF_REL in res["items"]["R3"] else "不在缺登记名单",
             "缺登记" if SELF_REL in res["items"]["R5"] else "不在缺登记名单"))
    for p in res["problems"]:
        print("  ✗ %s" % p)
    bad = c["R1"] + c["R2"] + c["R3"] + c["R5"]      # R4 / R6 = 单列旁栏（写死边界：不进退码）
    unjudged = len(res["not_judged"]) + len(res["problems"]) + (1 if c["R4"] is None else 0)
    if unjudged or not res["new_files"]:
        print("  判据结论：**判不了 / 分母 0 ⇒ 不给结论（rc=2）**（真的没有 ≠ 判不了）")
        return 2
    if bad:
        print("  判据结论：**%d 条缺登记**（逐条点名如上）；报告档 rc=%d（只报告 · 不接闸）"
              % (bad, 1 if strict else 0))
        return 1 if strict else 0
    print("  判据结论：判到了 —— 五张登记表逐面 **0 条缺登记**（面④/⑥ 见上）")
    return 0


# ── 自检（合成夹具 · 成对 · 不扫真仓）────────────────────────────────────────
# 夹具里的「干跑道」判据件 = 桩：只暴露登记表路径常量与扫描函数（口径上游仍是门⑩ 那件自己的）
STUB_REG_REL = "core/internal/contract/semantics-stub.json"
STUB_JUDGE_REL = "scripts/gates/check-semantics-stub.py"
FIXTURE_STUB = ('REGISTRY_REL = "' + STUB_REG_REL + '"\n\n\n'
                'def scan_hits(root):\n'
                '    return {"scripts/sandbox/new-probe.sh"}, None\n')

FIXTURE_MARKS = """文件\tpublic\t理由
scripts/old.sh\tno\t夹具存量件
"""

FIXTURE_COV = """{
  "scan": {"script_globs": ["scripts/*.sh", "scripts/*.py", "scripts/sandbox/*.sh"],
           "nosuffix_dir": "scripts"},
  "exclude": {"path_components": ["__pycache__", "_history"]},
  "whitelist": {"scripts": []}
}
"""

FIXTURE_DRY = """{
  "_扫描命令": "grep -r -E \\"（口径 = 判据件的扫描函数）\\" --include=*.sh --include=*.py scripts core",
  "件": [{"file": "scripts/old.sh"}]\n}\n"""

FIXTURE_STEPS = """#!/usr/bin/env bash
add_step "gates" tri-report "门76 新件登记完整性（只报告）" python3 scripts/gates/check-newfile-registry.py
add_step "gates" tri "门⑩ 干跑：一套语义（阻断）" scripts/gates/check-semantics-stub.py
add_step "gates" tri "public: 公开面标记表" python3 scripts/gates/check-public-marks.py
"""

FIXTURE_PUB = """#!/usr/bin/env bash
EXCLUDES=(
  "tools/ocr/venv"
)
"""

FIXTURE_MIRROR = """DROP_EXACT = {
    "core/internal/o1.json",
}
"""

FIXTURE_COV_JUDGE = """#!/usr/bin/env python3
def report():
    print("        ⇒ 处置：**真修** —— 该件与本批代码**同批挂门**"
          "（`scripts/gates/precommit-gates.sh` 的 `add_step` 按名调用它）；"
          "新件**不许**蹭基线棘轮。")
"""


def build_fixture(tmp, probe=None, register=None):
    """合成夹具：六张表（最小真形状）+ 一件存量件 + 可选一件新件。register 逐面写登记（两态用）。"""
    register = register or set()
    for d in ("scripts", "scripts/sandbox", "scripts/build", "scripts/gates", "publish",
              "core/internal/contract"):
        os.makedirs(os.path.join(tmp, d), exist_ok=True)
    open(os.path.join(tmp, "scripts", "old.sh"), "w").write("#!/usr/bin/env bash\necho old\n")
    marks = FIXTURE_MARKS
    cov = FIXTURE_COV
    dry = FIXTURE_DRY
    pub = FIXTURE_PUB
    mir = FIXTURE_MIRROR
    if probe:
        open(os.path.join(tmp, probe), "w").write("#!/usr/bin/env bash\necho probe\n")
        if "①" in register:
            marks += "%s\tno\t夹具新件\n" % probe
        if "②" in register:
            dry = dry.replace('{"file": "scripts/old.sh"}',
                              '{"file": "scripts/old.sh"}, {"file": "%s"}' % probe)
        if "③" in register:
            cov = cov.replace('"whitelist": {"scripts": []}',
                              '"whitelist": {"scripts": [{"path": "%s", "reason": "夹具", "date": "2026-09-28"}]}' % probe)
        if "⑤" in register:
            pub = pub.replace('\n)', '\n  "%s"\n)' % probe)
            mir = mir.replace("\n}", '\n    "%s",\n}' % probe)
    open(os.path.join(tmp, MARKS_REL), "w").write(marks)
    open(os.path.join(tmp, COV_REL), "w").write(cov)
    os.makedirs(os.path.join(tmp, os.path.dirname(STUB_REG_REL)), exist_ok=True)
    open(os.path.join(tmp, STUB_REG_REL), "w").write(dry)
    open(os.path.join(tmp, STUB_JUDGE_REL), "w").write(FIXTURE_STUB)
    open(os.path.join(tmp, STEPS_REL), "w").write(FIXTURE_STEPS)
    open(os.path.join(tmp, PUB_REL), "w").write(pub)
    open(os.path.join(tmp, MIRROR_REL), "w").write(mir)
    open(os.path.join(tmp, COV_JUDGE_REL), "w").write(FIXTURE_COV_JUDGE)


def run_self_test():
    checks = []
    probe = "scripts/sandbox/new-probe.sh"

    def case(name, want_rc, run_kwargs, setup=None):
        tmp = tempfile.mkdtemp(prefix="gnr-selftest-")
        try:
            setup(tmp)
            kwargs = {"new_files": None, "inv": None, "strict": False, "baseline": None}
            kwargs.update(run_kwargs)
            rc = run(tmp, **kwargs)
            checks.append((name, want_rc, rc, rc == want_rc))
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def all_five(tmp):
        build_fixture(tmp, probe=probe, register={"①", "②", "③", "⑤"})
        open(os.path.join(tmp, "inv.tsv"), "w").write("件\n%s\n" % probe)

    def unregistered(tmp):
        build_fixture(tmp, probe=probe)
        open(os.path.join(tmp, "inv.tsv"), "w").write("件\n%s\n" % probe)

    def no_ledger(tmp):
        build_fixture(tmp, probe=probe, register={"①", "②", "③", "⑤"})
        open(os.path.join(tmp, "inv-absent.tsv"), "w").write("件\n")

    def no_probe(tmp):
        build_fixture(tmp)
        open(os.path.join(tmp, "inv.tsv"), "w").write("件\n")

    # 断言（条数以该件自检自报为准 · 不在此写死）
    case("正控① · 五表登记齐（报告档）⇒ 0", 0, {"new_files": [probe], "inv": "inv.tsv", "strict": False},
         setup=all_five)
    case("正控② · 五表登记齐（阻断档）⇒ 0", 0, {"new_files": [probe], "inv": "inv.tsv", "strict": True},
         setup=all_five)
    case("负控① · 新件**缺登记**（报告档不阻断）⇒ 0 但逐条点名", 0,
         {"new_files": [probe], "inv": "inv.tsv", "strict": False}, setup=unregistered)
    case("负控② · 新件缺登记（阻断档）⇒ 1 红", 1,
         {"new_files": [probe], "inv": "inv.tsv", "strict": True}, setup=unregistered)
    case("判不了① · **分母 0 件**（没有新件）⇒ 2 不许绿", 2,
         {"new_files": [], "inv": None, "strict": False}, setup=no_probe)
    case("判不了② · 面④ 仓外台账真源不在 ⇒ 2（含 ⑤ 已登记 ⇒ 不许报绿）", 2,
         {"new_files": [probe], "inv": "路径不存在.tsv", "strict": False}, setup=no_ledger)
    case("边界① · 基线不在盘 ⇒ 2（不替基线猜分母）", 2,
         {"new_files": None, "inv": None, "strict": False}, setup=no_probe)

    def weak_only(tmp):
        build_fixture(tmp, probe=probe, register={"①", "②", "⑤"})   # 面③ 只以 basename 提及
        open(os.path.join(tmp, STEPS_REL), "a").write(
            'add_step "build" tri "构建" bash scripts/build/new-probe.sh\n')
        open(os.path.join(tmp, "inv.tsv"), "w").write("件\n%s\n" % probe)

    case("边界② · 仅 basename 弱挂载 ⇒ 不判红（不计入六表缺登记）", 0,
         {"new_files": [probe], "inv": "inv.tsv", "strict": True}, setup=weak_only)

    print("── 自检（合成夹具 · 成对 · 不扫真仓）──")
    bad = 0
    for name, want, got, ok in checks:
        print("  %s %s —— 要 %d · 得 %d" % ("✓" if ok else "✗", name, want, got))
        bad += 0 if ok else 1
    print("自检结论: %s（断言 %d 条 · 失败 %d）" % ("全过" if bad == 0 else "**不过**", len(checks), bad))
    return 0 if bad == 0 else 1


def main():
    ap = argparse.ArgumentParser(add_help=True,
                                 description="门76 新件登记完整性：件级 × 六张登记表（只报告 · 起手不接闸）")
    ap.add_argument("--repo", default=REPO, help="被查仓根（默认本仓）")
    ap.add_argument("--baseline", default=DEFAULT_BASELINE, help="冻结基线（仓外生成件 · --dump-baseline 生成）")
    ap.add_argument("--dump-baseline", default=None, help="把现读件集写成基线（仓外路径）后退出")
    ap.add_argument("--inventory", default=None, help="面④ 仓外台账 tsv（不给 ⇒ 面④ 判不了 · 需人签）")
    ap.add_argument("--new-file", action="append", default=None, help="直接指分母（可重复 · 两态取证用）")
    ap.add_argument("--strict", action="store_true", help="阻断档：R1–R5 任一 > 0 ⇒ rc=1（默认报告档 rc=0）")
    ap.add_argument("--self-test", action="store_true", help="只跑合成夹具自检（不扫真仓）")
    args = ap.parse_args()
    if args.self_test:
        return run_self_test()
    root = os.path.abspath(args.repo)
    if args.dump_baseline:
        face = scan_ledger_face(root)
        if face is None:
            print("✗ 扫面不在盘：%s ⇒ 不写基线" % os.path.join(root, "scripts"), file=sys.stderr)
            return 2
        os.makedirs(os.path.dirname(os.path.abspath(args.dump_baseline)), exist_ok=True)
        with open(args.dump_baseline, "w", encoding="utf-8") as fh:
            fh.write("# 门76 冻结基线（现读件集 · 生成件 · 禁手维）\n")
            for f in sorted(face):
                fh.write(f + "\n")
        print("基线已写：%s（%d 件）" % (args.dump_baseline, len(face)))
        return 0
    return run(root, args.new_file, args.inventory, args.strict, args.baseline)


if __name__ == "__main__":
    sys.exit(main())

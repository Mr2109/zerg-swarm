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
  · 分母 **0 件分两态（GAP-20261001-21 修）**：**全树扫过（扫描面非空）∖ 在盘基线 = 0 件** ⇒ **正常态 rc=0**；
    仅「扫描面为空 / 基线不在盘 / 显式空件集」= **判不了 ⇒ rc=2**（★ 零执行探针 ✗ 不许放宽）。
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
  · **写面只有一个 `--dump-baseline`**，且**默认干跑不写**（只有显式 `--yes` 才落）—— 这是本件唯一会碰盘的动作；
    它写的是**仓外生成件**（冻结基线），**不是任何登记表**。六张登记表一律只读。

退码（**只报告档** · 与 `check-registry-vs-disk.py` / `check-docs-orphans-links.py` 同规）

  0 = 判到了（逐格给数 + 逐条点名；**报告档不阻断**）
  1 = 只有 `--strict` 档才可能：R1/R2/R3/R5 任一格 > 0（逐条点名；R4 / R6 = 旁栏不进退码）
  2 = **不给结论**：基线不在盘 / 扫描面为空 / 显式空件集（零执行 = 假绿）· 任一登记表读不到 / 解析出 0 条 ·
      **任一面判不了**（含面④ 仓外真源不在 · 面⑥ 判据件不在）· `--self-test` 不过 ·
      `--json` 断面名不认 / 为空 · `--yes` 不与 `--dump-baseline` 连用 ·
      **面清单里出现未产出记录的面**（聚合器合成 `UNKNOWN · reason_code=MISSING_FACE · decided=false` ⇒ 退码钉 2）

查缺档档（`--check-missing` · **只读** · 三合并式判红）

  ★ **判红三合并式**（三条**任一非空即红** · **禁只用 `git diff`** —— 它看不见 untracked/ignored）：
    ① `git diff --exit-code HEAD -- <paths>` · ② `git ls-files --others --exclude-standard -- <paths>` ·
    ③ `git ls-files --others --ignored --exclude-standard -- <paths>`（若用 `git status --porcelain`
    必须显式 `--untracked-files=all`；本件优先 ②）。**隔离**：`git worktree add -f -q <tmp> HEAD`，
    **退出必清**（try/finally）。**生成前先清空 `--outdir`**（防旧件残留看着一致）。
  退码：`0` 判到了（干净 / 非 git 工作树不适用）· `1` **红** · `2` **判不了**（真源不可读 · 空件集 = 零执行探针）。

机器相关面（host-bound）

  面⑧「家目录批准件」真身在**本机家目录**（`~/.zerg/state/approvals/dev_edit.json`）⇒ 报
  **HOST-REQUIRED / reason_code=SKIPPED-HOST-BOUND / decided=false**，**不计入 pass 也不计入 fail**
  （既不 PASS 假绿也不硬 FAIL），但**在机器面显式列出并暴露计数**（`host_required_faces` / `host_required_count`）。

用法

  python3 scripts/gates/check-newfile-registry.py                       # 查本仓（报告档）
  python3 scripts/gates/check-newfile-registry.py --dump-baseline /仓外/基线.tsv        # 干跑：只打印计划（默认不写）
  python3 scripts/gates/check-newfile-registry.py --dump-baseline /仓外/基线.tsv --yes  # 真写冻结基线（唯一写面）
  python3 scripts/gates/check-newfile-registry.py --baseline /仓外/基线.tsv --inventory /仓外/台账.tsv
  python3 scripts/gates/check-newfile-registry.py --strict --json faces,self            # 机器面 JSON
  python3 scripts/gates/check-newfile-registry.py --repo <仓根> --new-file scripts/x.sh   # 指分母（两态取证用）
  python3 scripts/gates/check-newfile-registry.py --check-missing                        # 查缺档：三合并式（只读）
  python3 scripts/gates/check-newfile-registry.py --check-missing --paths scripts --json chk   # 机器面
  python3 scripts/gates/check-newfile-registry.py --check-missing --outdir /仓外/报告     # 生成报告（先清空该目录）
  python3 scripts/gates/check-newfile-registry.py --self-test

默认干跑（写面最小 · 比业界写器保守）

  本门**默认不写任何件**（只读报告 / 干跑）。**唯一的写面 = `--dump-baseline`**（生成冻结基线 · 仓外生成件），
  且**只有显式 `--yes` 才真落**：不带 `--yes` 时它只把计划（落点 + 将写件数）打到 stdout、**一个字节都不写**。
  这是本件与「默认真写」的写器（业界常见形态）**刻意不同**的保守面 —— 目的是「先看计划、再显式确认落点」。
  （`--yes` 不与 `--dump-baseline` 连用 ⇒ 用法错 rc=2；其余旗标一律只读。）

机器面（`--json`）

  `--json <断面>`：断面为逗号分隔、只认 `faces` 与 `self`；给了就**只出 JSON**（人面向 stdout 让位）。
  JSON 恒含：`expected_faces`（本件九面名 · **跑前先生成**）· `reported_faces`（本次实际报出的面）·
  `face_states`（各面 state）· `face_records`（各面 `state/reason_code/decided`）·
  `counts`（五态 + `HOST-REQUIRED` 计数）· `host_required_faces`/`host_required_count`（机器相关面）·
  `missing_faces`/`missing_count`（未产出记录的面 · 非空 ⇒ rc=2）· `fully_exercised` ·
  `all_green`（判到了且 0 条缺登记）；`faces` / `self` 断面分别加逐面详情与自指详情。
  断面名不认 / 为空 ⇒ 用法错 rc=2（不给结论）。`--check-missing` 档另认断面 `chk`（只出查缺档机器面）。
"""

import argparse
import glob as globmod
import hashlib
import importlib.util
import io
import json
import os
import re
import shutil
import subprocess
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

# ── 机器面：门76 的**九面**（逐字对齐人面向输出里印出的面名 · 顺序 = 打印顺序）────────────
# 为什么是这九面：六张登记表（①–⑥）＋ **⑧家目录批准件**（host-bound · 机器相关面）＋
#   口径差栏（参照读数）＋ 自指（本门对自身的登记情形）。
# ★ `expected_faces()` **跑前先生成**这张清单；任何**未产出记录**的面由 `face_records` 合成
#   `state=UNKNOWN · reason_code=MISSING_FACE · decided=false` 并**退码钉 2**（禁静默跳过缺面）。
FACES = (
    "①公开标记表",
    "②干跑登记",
    "③门①两张真源",
    "④脚本台账",
    "⑤两器排除表",
    "⑥失败文案面",
    "⑧家目录批准件",
    "口径差栏",
    "自指",
)
# 五态（`precommit-gates.sh` 的步骤级四档 PASS/FAIL/BLOCKED/REPORT ＋ 自报未知态 UNKNOWN）：
#   PASS    = 判到了且该面 0 条缺登记（R4/R6 = 无发现）
#   FAIL    = 判到了且该面 > 0 条缺登记（R1/R2/R3/R5）
#   BLOCKED = 该面自己的真源读不到 / 面判不了（面级不给结论）
#   REPORT  = 只报不判的面（R4/R6 旁栏不进退码 · 口径差栏/自指 = 参照/自指读数）
#   UNKNOWN = 整跑在**面级判读之前**就短路（分母级判不了：扫面不在盘 / 基线不在盘 / 扫描面为空）⇒ 该面结论未知
#   ★ **HOST-REQUIRED** = **机器相关面（host-bound）**：本机家目录态、不进仓、不可移植 ⇒ 既不 PASS
#     （假绿）也不硬 FAIL。聚合时**不计入 pass 也不计入 fail**，但必须**显式列出并暴露计数**
#     （green gate never hides which faces did not actually run）。
FACES_STATES = ("PASS", "FAIL", "BLOCKED", "REPORT", "UNKNOWN", "HOST-REQUIRED")

# ★ **机器相关面（host-bound）**：真身在**本机家目录**（`~/.zerg/state/approvals/dev_edit.json`），
#   不进仓、随机器而变 ⇒ 报 HOST-REQUIRED / reason_code=SKIPPED-HOST-BOUND，聚合时不计 pass/fail。
HOST_BOUND_FACES = ("⑧家目录批准件",)
APPROVALS_REL = os.path.join(".zerg", "state", "approvals", "dev_edit.json")

# 面级**原因码**（机器面 `face_records[<面>].reason_code` 的闭集）
REASON_OK = "OK"
REASON_MISSING_ENTRY = "MISSING_ENTRY"          # 判到了且该面 > 0 条缺登记（FAIL）
REASON_SOURCE_UNREADABLE = "SOURCE_UNREADABLE"  # 该面真源读不到（BLOCKED）
REASON_REPORT_ONLY = "REPORT_ONLY"              # 只报不判（REPORT）
REASON_UNJUDGED = "UNJUDGED"                    # 整跑面级判读前短路（UNKNOWN）
REASON_MISSING_FACE = "MISSING_FACE"            # ★ 未产出记录的面（聚合器合成 · 退码钉 2）
REASON_HOST_BOUND = "SKIPPED-HOST-BOUND"        # ★ 机器相关面未跑（HOST-REQUIRED）


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
        s = ln.split("#")[0].strip()          # ★ ② 剥行内注释（消假阳性 · 与 load_drop_exact 同法）
        if not s:
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


# ── 查缺档（三合并式 · 判红机制 · 隔离工作树）─────────────────────────────────
# ★ **判红三合并式**：三条**任一非空即红** ——
#     ① tracked 有改动 :  `git diff --exit-code HEAD -- <paths>`   （rc=1 ⇒ 有）
#     ② untracked      :  `git ls-files --others --exclude-standard -- <paths>`
#     ③ ignored        :  `git ls-files --others --ignored --exclude-standard -- <paths>`
#   ★ **禁只用 `git diff` 判红** —— 它看不见 untracked（②）与 ignored（③）⇒ 那两格必须单列现读。
#   ★ 若改用 `git status --porcelain`，**必须显式 `--untracked-files=all`**
#     （`git config status.showUntrackedFiles no` 会蒙蔽它）；本件**优先 `ls-files --others`** 那条路。
# ★ **隔离**：`git worktree add -f -q <tmp> HEAD` 取 HEAD 参照；**退出必清**（try/finally）。
# ★ **生成前先清空输出目录**（防旧件残留看着一致）—— 见 `run_check_missing` 的 `--outdir` 分支。
CHK_MISSING_PATHS_DEFAULT = ("scripts",)


def _git(root, args):
    """跑一条 git 命令。返回 (rc|None, 行列表, stderr)。★ 起不来（无 git / OSError）⇒ rc=None。"""
    try:
        p = subprocess.run(["git", "-C", root] + list(args),
                           capture_output=True, text=True)
    except OSError as e:
        return None, [], str(e)
    return p.returncode, [ln for ln in p.stdout.splitlines() if ln.strip()], p.stderr.strip()


def git_worktree_kind(root):
    """root 是不是 git 工作树：'yes' / 'no'（非 git 工作树）/ 'unreadable'（`.git` 在但读不了）。"""
    has_dotgit = os.path.exists(os.path.join(root, ".git"))
    rc, _out, _err = _git(root, ["rev-parse", "--is-inside-work-tree"])
    if rc is None:
        return "unreadable"          # git 起不来
    if rc == 0:
        return "yes"
    return "unreadable" if has_dotgit else "no"


def _worktree_add(root):
    """`git worktree add -f -q <tmp> HEAD`。返回 (工作树路径, 错误|None)。"""
    tmp = tempfile.mkdtemp(prefix="gnr-chkmiss-")
    wt = os.path.join(tmp, "co")
    rc, _out, err = _git(root, ["worktree", "add", "-f", "-q", wt, "HEAD"])
    if rc is None or rc != 0:
        shutil.rmtree(tmp, ignore_errors=True)
        return None, ("`git worktree add` 起不来 / 失败（rc=%s）：%s" % (rc, err))
    return wt, None


def _worktree_remove(root, wt):
    """★ 退出必清：移除工作树 + 删临时父目录（幂等）。"""
    _git(root, ["worktree", "remove", "--force", wt])
    shutil.rmtree(os.path.dirname(os.path.abspath(wt)), ignore_errors=True)


def worktree_three_way(root, paths):
    """查缺档**判红**：三合并式（tracked 改动 ∥ untracked ∥ ignored）任一非空 ⇒ `red=True`。

    判不了/无数据与红是三态、必须分得开：
      · `unreadable=True` ⇒ 真源不可读（`.git` 在但 `git` 失败 / `git` 起不来）⇒ 调用方退码 **2**；
      · `no_data=True`   ⇒ **零执行探针**（空件集：路径下无任何 tracked/untracked 件）⇒ 非绿、退码 **2**；
      · `applicable=False` ⇒ root 根本不是 git 工作树（如纯合成夹具）⇒ 三合并式不适用（不当绿也不当红）。
    ★ 隔离：在 HEAD 建一枚 worktree 当参照，**退出必清**（try/finally）；清理结果写进读数。
    """
    res = {"applicable": True, "unreadable": False, "reason": None, "paths": list(paths),
           "tracked_modified": [], "untracked": [], "ignored": [],
           "red": False, "no_data": False, "file_set_size": 0,
           "head_tracked_count": None, "worktree": None, "worktree_cleaned": False}
    kind = git_worktree_kind(root)
    if kind == "no":
        res["applicable"] = False
        res["reason"] = "非 git 工作树（`%s` 不受版本控制）⇒ 三合并式不适用" % (paths[0] if paths else "?")
        return res
    if kind == "unreadable":
        res["unreadable"] = True
        res["reason"] = "git 真源不可读（`.git` 在、但 `git rev-parse` 失败）"
        return res
    rel = [p for p in paths if p]
    # ① tracked 有改动（--exit-code：0=干净 / 1=有改动；其余 = 判不了）
    rc, out, err = _git(root, ["diff", "--name-only", "--exit-code", "HEAD", "--"] + rel)
    if rc is None or rc not in (0, 1):
        res["unreadable"] = True
        res["reason"] = "`git diff --exit-code HEAD` rc=%s（应为 0/1）：%s" % (rc, err)
        return res
    res["tracked_modified"] = sorted(out)
    # ② untracked（★ 优先 ls-files --others；③ ignored 同族）
    rc, out, err = _git(root, ["ls-files", "--others", "--exclude-standard", "--"] + rel)
    if rc is None or rc != 0:
        res["unreadable"] = True
        res["reason"] = "`git ls-files --others` rc=%s：%s" % (rc, err)
        return res
    res["untracked"] = sorted(out)
    rc, out, err = _git(root, ["ls-files", "--others", "--ignored", "--exclude-standard", "--"] + rel)
    if rc is None or rc != 0:
        res["unreadable"] = True
        res["reason"] = "`git ls-files --others --ignored` rc=%s：%s" % (rc, err)
        return res
    res["ignored"] = sorted(out)
    # 文件集大小（判「零执行探针」：空件集 ⇒ no_data、非绿）
    rc, out, _err = _git(root, ["ls-files", "--"] + rel)
    tracked_files = sorted(out) if rc == 0 else []
    res["file_set_size"] = len(set(tracked_files) | set(res["untracked"]))
    res["red"] = bool(res["tracked_modified"] or res["untracked"] or res["ignored"])
    if res["file_set_size"] == 0 and not res["red"]:
        res["no_data"] = True
    # ★ 隔离：HEAD worktree 当参照 · **退出必清**
    wt = None
    try:
        wt, werr = _worktree_add(root)
        if wt is None:
            res["unreadable"] = True
            res["reason"] = werr
            return res
        res["worktree"] = wt
        rc, out, _err = _git(wt, ["ls-files", "--"] + rel)   # HEAD 参照件集（对照读数）
        if rc == 0:
            res["head_tracked_count"] = len(out)
    finally:
        if wt is not None:
            _worktree_remove(root, wt)
            res["worktree_cleaned"] = True
    return res


def _chk_missing_rc(res):
    """查缺档退码：不适用/干净 ⇒ 0 · **红 ⇒ 1** · 判不了 / 空件集 ⇒ **2**（绝不 0）。"""
    if not res.get("applicable"):
        return 0
    if res.get("unreadable") or res.get("no_data"):
        return 2
    if res.get("red"):
        return 1
    return 0


def run_check_missing(root, paths, outdir=None, as_json=False):
    """`--check-missing`：三合并式查缺档（只读 · 隔离工作树 · 生成前清空输出目录）。

    退码：0 = 不适用/干净 · 1 = **红**（三合并式任一非空）· 2 = **判不了 / 空件集**（fail-closed）。
    ★ 遍历序决定者：三条列表各自 `sorted()`；JSON 顶层键 = 本函数插入序，写盘件 `sort_keys=True`。
    """
    sel = [p.strip() for p in (paths or []) if p.strip()] or list(CHK_MISSING_PATHS_DEFAULT)
    res = worktree_three_way(root, sel)
    rc = _chk_missing_rc(res)
    obj = {
        "schema": "zerg/gate-json/v1",
        "tool": "check-newfile-registry.py",
        "gate": "门76 查缺档（三合并式）",
        "repo": root,
        "paths": sel,
        "rc": rc,
        "applicable": res["applicable"],
        "red": res["red"],
        "no_data": res["no_data"],
        "unreadable": res["unreadable"],
        "reason": res["reason"],
        "tracked_modified": res["tracked_modified"],
        "untracked": res["untracked"],
        "ignored": res["ignored"],
        "file_set_size": res["file_set_size"],
        "head_tracked_count": res["head_tracked_count"],
        "worktree": res["worktree"],
        "worktree_cleaned": res["worktree_cleaned"],
        "outdir": os.path.abspath(outdir) if outdir else None,
    }
    if outdir:
        # ★ 生成前先**清空输出目录**（防旧件残留看着一致）
        absd = os.path.abspath(outdir)
        shutil.rmtree(absd, ignore_errors=True)
        os.makedirs(absd, exist_ok=True)
        with open(os.path.join(absd, "查缺档.json"), "w", encoding="utf-8") as fh:
            fh.write(json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=True) + "\n")
        with open(os.path.join(absd, "查缺档.tsv"), "w", encoding="utf-8") as fh:
            fh.write("状态\t路径\n")
            for leg, items in (("tracked改动", res["tracked_modified"]),
                               ("untracked", res["untracked"]), ("ignored", res["ignored"])):
                for it in items:
                    fh.write("%s\t%s\n" % (leg, it))
    if as_json:
        sys.stdout.write(json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=False) + "\n")
        return rc
    print("门76 查缺档（三合并式 · 只读 · 隔离工作树）")
    print("  仓根：%s" % root)
    print("  判据路径：%s" % ", ".join(sel))
    print("  三合并式：① tracked 改动 %d · ② untracked %d · ③ ignored %d（任一非空即红）"
          % (len(res["tracked_modified"]), len(res["untracked"]), len(res["ignored"])))
    for leg, items in (("tracked改动", res["tracked_modified"]), ("untracked", res["untracked"]),
                       ("ignored", res["ignored"])):
        for it in items:
            print("     ✗ %s %s" % (leg, it))
    if not res["applicable"]:
        print("  ⇒ 三合并式**不适用**（非 git 工作树）⇒ 退码 %d" % rc)
        return rc
    if res["unreadable"]:
        print("  ⇒ 判不了（真源不可读）：%s ⇒ **不给结论（rc=2）**" % res["reason"])
        return rc
    if res["no_data"]:
        print("  ⇒ **零执行探针**（空件集，file_set_size=0）⇒ 无数据标志置位、**非绿（rc=2）**")
        return rc
    if res["red"]:
        print("  ⇒ **红**：工作树相对 HEAD 有分歧（见上）⇒ rc=1")
        return rc
    print("  ⇒ 判到了 —— 三合并式逐条 0 件（file_set_size=%d · HEAD 参照 %s 件 · 工作树已清 %s）"
          % (res["file_set_size"], res["head_tracked_count"], "是" if res["worktree_cleaned"] else "否"))
    return rc


# ── 主判据 ────────────────────────────────────────────────────────────────
# ── ③ B1-3：基线 v2（件数 + 指纹）· 确定性 · 禁部分扫描写整树 ──────────────────
def baseline_fingerprint(face):
    """现读件集的稳定指纹（排序逐行 · 纯内容派生 · 禁 mtime / 禁读盘序）。返回 16 位 hex。"""
    h = hashlib.sha256()
    for f in sorted(face):
        h.update(f.encode("utf-8"))
        h.update(b"\n")
    return h.hexdigest()[:16]


def parse_baseline_header(text):
    """基线 v2 首行注释头（件数 / 指纹）。无头（v1 旧件 / 合成夹具）⇒ (None, None) 照读。"""
    for ln in text.splitlines()[:3]:
        if not ln.startswith("#"):
            continue
        m = re.search(r"件数=(\d+)(?:\s+指纹=([0-9a-fA-F]{6,64}))?", ln)
        if m:
            return int(m.group(1)), m.group(2)
    return None, None


def scan_ledger_twice(root):
    """双跑 cmp 自证（确定性闸）：两次扫描不等 ⇒ 判不了（禁部分扫描写整树）。"""
    a = scan_ledger_face(root)
    b = scan_ledger_face(root)
    return a, (a is not None and b is not None and set(a) == set(b))


# ── ④ B1-4 补齐档：只写纯派生面 · 面③⑤⑧ 只报「该跑哪条正门」──────────────────
def run_sync(root, yes, baseline):
    """补齐档：默认干跑（零写入）；显式 --yes 才真写，且只写纯派生面（面① 占位行），
    走 temp+fsync+rename 原子写。面③（要拍档）⑤（public=no 才要·两器同批）⑧（人签）只报「该跑哪条正门」。"""
    face, det = scan_ledger_twice(root)
    if face is None:
        print("✗ 扫面不在盘 ⇒ 不给结论（rc=2）", file=sys.stderr)
        return 2
    if not det:
        print("✗ 双跑 cmp 不一致（扫描非确定）⇒ 拒写（禁部分扫描写整树）", file=sys.stderr)
        return 2
    base = set()
    if baseline and os.path.isfile(baseline):
        try:
            btxt = read_text(baseline)
        except OSError as e:
            print("✗ 基线在盘但读不出（%s）⇒ fail-closed 拒写 rc=2" % e, file=sys.stderr)
            return 2
        for ln in btxt.splitlines():
            ln = ln.strip()
            if ln and not ln.startswith("#"):
                base.add(ln)
    newf = sorted(face - base)
    marks = load_marks(root)
    miss = [f for f in newf if marks is None or f not in marks]
    print("── 补齐档 `--sync`（%s）──" % ("真写" if yes else "干跑 · 未写件"))
    print("   面① 公开标记表：补占位行 %d 件" % len(miss))
    print("   面④ 脚本台账：该跑正门 `zerg script inventory sync --docs-root <Zerg-内部文档> --yes`（本档不代跑）")
    print("   面⑦ 判据注册表：该跑正门 `zerg gate check check-judge-registry --emit`（本档不代跑）")
    print("   面③ 门①两张真源：该跑正门 = 人加 add_step 或 whitelist.scripts（要拍档 · 本档不替人拍）")
    print("   面⑤ 两器排除表：该跑正门 = public=no 才要，EXCLUDES + DROP_EXACT 两器同批（本档不代改）")
    print("   面⑧ 批准件 scope：该跑正门 = `zerg approve new --tool dev_edit`（人签 · 非交互拒）")
    if not yes:
        print("   ⇒ **干跑未写**：加 `--yes` 才落（只写纯派生面；面③⑤⑧ 只报该跑哪条正门）")
        return 0
    if not miss:
        print("   ⇒ 计划件 0 ⇒ 零写入")
        return 0
    p = os.path.join(root, MARKS_REL)
    _dir = os.path.dirname(os.path.abspath(p))
    os.makedirs(_dir, exist_ok=True)
    fd, tmp = tempfile.mkstemp(prefix=".gnr-marks-", dir=_dir)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            if os.path.isfile(p):
                fh.write(read_text(p))
            for f in miss:
                fh.write("%s\tno\t待补理由（--sync 占位 · 人补）\n" % f)
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(tmp, p)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise
    print("   已写 面① 占位行 %d 件（原子写 · temp+fsync+rename）" % len(miss))
    return 0


def dump_baseline(root, target, yes):
    """③ B1-3 冻结基线生成口：双跑 cmp 自证 + v2 头（件数+指纹）+ 只许下调 + 原子写（temp+fsync+rename）。"""
    face, det = scan_ledger_twice(root)
    if face is None:
        print("✗ 扫面不在盘：%s ⇒ 不写基线" % os.path.join(root, "scripts"), file=sys.stderr)
        return 2
    if not det:  # ★ ③ 双跑 cmp 自证：扫描非确定 ⇒ 禁部分扫描写整树
        print("✗ 双跑 cmp 不一致（扫描非确定）⇒ 拒写（禁部分扫描写整树）", file=sys.stderr)
        return 2
    fp = baseline_fingerprint(face)
    old_cnt = None
    if os.path.isfile(target):
        try:
            old_cnt, _ = parse_baseline_header(read_text(target))
        except OSError:
            old_cnt = None
    if not yes:
        # 默认干跑：只打印计划、一个字节都不写（比「默认真写」的写器保守）
        print("── 冻结基线生成档（**干跑** · 未写件）")
        print("   目标件 : %s" % target)
        print("   将写   : %d 件（现读台账口径件集逐行 · 首行注释 + 逐件一行）" % len(face))
        if old_cnt is not None and len(face) > old_cnt:
            print("   ✗ 基线只许下调：新 %d > 旧 %d ⇒ 加 `--yes` 也拒写" % (len(face), old_cnt), file=sys.stderr)
            return 2
        print("   ⇒ **干跑未写**：加 `--yes` 才落（本件默认不写件）")
        return 0
    if old_cnt is not None and len(face) > old_cnt:  # ★ ③ 基线只许下调
        print("✗ 基线只许下调：新 %d 件 > 旧 %d 件 ⇒ 拒写（fail-closed）" % (len(face), old_cnt), file=sys.stderr)
        return 2
    _dir = os.path.dirname(os.path.abspath(target))
    os.makedirs(_dir, exist_ok=True)
    fd, _tmp = tempfile.mkstemp(prefix=".gnr-base-", dir=_dir)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write("# 门76 冻结基线 v2 件数=%d 指纹=%s（现读件集 · 生成件 · 禁手维）\n" % (len(face), fp))
            for f in sorted(face):
                fh.write(f + "\n")
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(_tmp, target)
    except BaseException:
        try:
            os.unlink(_tmp)
        except OSError:
            pass
        raise
    print("基线已写：%s（%d 件）" % (target, len(face)))
    return 0


def audit(root, new_files=None, inv=None, baseline=None):
    res = {"counts": None, "faces": {}, "problems": [], "not_judged": [], "roster": None,
           "face_judged": {}, "den_from_baseline": False, "den_scan_empty": True}
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
        try:
            _btxt = read_text(baseline)
        except OSError as _e:  # ★ ① 基线在盘但读不出 ⇒ fail-closed（未捕获 OSError 曾是 rc 回落 0 的静默绿口）
            res["problems"].append(
                "分母判不了：冻结基线在盘但读不出（%s：%s）⇒ fail-closed 不给结论（rc=2）"
                % (baseline, _e))
            return res
        _bcount, _bfp = parse_baseline_header(_btxt)
        for ln in _btxt.splitlines():
            ln = ln.strip()
            if ln and not ln.startswith("#"):
                base.add(ln)
        new_files = sorted(ledger_face - base)
        # ★ ③ B1-3：基线 v2 头（件数+指纹）· 过期即红（件在减少 ⇒ 基线已不描述本树）
        if _bcount is not None and len(ledger_face) < _bcount:
            res["problems"].append(
                "基线过期（v2 头声明 %d 件 ⟷ 现读扫面 %d 件 ⇒ 件在减少）⇒ 判红 rc=2；"
                "先跑 `--dump-baseline <仓外路径> --yes` 重打" % (_bcount, len(ledger_face)))
        # ★ 甲/乙 分档（GAP 待入账）：分母 = **非空扫描面 ∖ 在盘基线** ⇒ 差集为空是**正常态**
        #   （全树扫过、确定性无新件 ⇒ rc=0）；扫描面自身为空 = 零执行探针 ⇒ 仍判不了。
        res["den_from_baseline"] = True
        res["den_scan_empty"] = (not ledger_face)
    else:
        new_files = sorted(set(new_files))
        res["den_from_baseline"] = False
        res["den_scan_empty"] = (not ledger_face)

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

    # 机器面：逐面「真源读到了没有」（面自身判不了 ⇒ BLOCKED；读到但整跑无结论 ⇒ UNKNOWN）
    res["face_judged"] = {
        "①公开标记表": marks is not None,
        "②干跑登记": dry_hits is not None and dry_reg is not None,
        "③门①两张真源": step_lines is not None and wl_items is not None,
        "④脚本台账": led_rows is not None,
        "⑤两器排除表": excludes is not None and drops is not None,
        "⑥失败文案面": fixed is not None,
    }

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


# ── 面清单 / 面记录（**跑前先生成 expected_faces** · 缺面必须合成 · 禁静默跳过）────────
def expected_faces():
    """★ **跑前先生成** expected_faces 清单（唯一真源 = `FACES`）。"""
    return list(FACES)


def _reason_for(state):
    """五态 → 原因码（闭集见模块常量）。"""
    return {"PASS": REASON_OK, "FAIL": REASON_MISSING_ENTRY, "BLOCKED": REASON_SOURCE_UNREADABLE,
            "REPORT": REASON_REPORT_ONLY}.get(state, REASON_UNJUDGED)


def face_records(res, states=None, faces=None):
    """逐面**记录**（state / reason_code / decided）。★ 聚合器**不得跳过缺面**：

      · host-bound 面（`HOST_BOUND_FACES`）⇒ `HOST-REQUIRED · SKIPPED-HOST-BOUND · decided=false`，
        **不计 pass 也不计 fail**（机器相关面：本机家目录态）；
      · 已产出记录的面 ⇒ 照 `build_face_states` 的态 + 原因码；
      · **未产出记录的面** ⇒ 合成 `UNKNOWN · MISSING_FACE · decided=false`（退码由 `_verdict` 钉 2）。
    """
    faces = expected_faces() if faces is None else list(faces)
    states = build_face_states(res) if states is None else states
    out = {}
    for f in faces:
        if f in HOST_BOUND_FACES:
            out[f] = {"state": "HOST-REQUIRED", "reason_code": REASON_HOST_BOUND, "decided": False}
        elif f in states:
            st = states[f]
            out[f] = {"state": st, "reason_code": _reason_for(st),
                      "decided": st in ("PASS", "FAIL", "REPORT")}
        else:
            out[f] = {"state": "UNKNOWN", "reason_code": REASON_MISSING_FACE, "decided": False}
    return out


def missing_faces(res, faces=None):
    """未产出记录的面（reason_code=MISSING_FACE）名单 · `sorted`。"""
    recs = face_records(res, faces=faces)
    return sorted(f for f, r in recs.items() if r["reason_code"] == REASON_MISSING_FACE)


# ── 出口判决（**唯一真源** · 人面向与机器面共用）────────────────────────────
def _verdict(res, strict, faces=None):
    """返回 (rc, bad, unjudged)。0 = 判到了 · 1 = 缺登记（仅 --strict）· 2 = 不给结论。

    与 `run()` 的出口分支**同一套数**（`bad` / `unjudged` 只由 res 派生、无副作用）——
    改一处必须两处同源，禁各算一套（`--json` 短路不得改变退码）。
    ★ **缺面（MISSING_FACE）⇒ 退码钉 2**（fail-closed：缺面不许当齐）。
    """
    if res.get("counts") is None:
        return 2, 0, 0
    c = res["counts"]
    bad = c["R1"] + c["R2"] + c["R3"] + c["R5"]              # R4 / R6 = 单列旁栏（写死边界：不进退码）
    unjudged = len(res["not_judged"]) + len(res["problems"]) + (1 if c["R4"] is None else 0)
    # ★ 甲/乙 分档：分母由「**非空**扫描面 ∖ **在盘**基线」派生 ⇒ 差集 0 件 = 正常态（rc 落到 bad 判定）；
    #   其余空件集（显式空分母 / 扫描面为空）= **零执行探针** ⇒ 仍 rc=2（✗ 不许放宽）。
    den_ok = bool(res.get("den_from_baseline")) and not res.get("den_scan_empty")
    if unjudged or (not res["new_files"] and not den_ok):
        return 2, bad, unjudged
    if missing_faces(res, faces):                            # ★ 缺面 ⇒ 退码钉 2
        return 2, bad, unjudged
    if bad:
        return (1 if strict else 0), bad, unjudged
    return 0, bad, unjudged


# ── 机器面（`--json`）：八面 × 五态（不改人面向一字）────────────────────────
def _face_count(res, name):
    c = res.get("counts") or {}
    return {"①公开标记表": c.get("R1"), "②干跑登记": c.get("R2"), "③门①两张真源": c.get("R3"),
            "④脚本台账": c.get("R4"), "⑤两器排除表": c.get("R5"), "⑥失败文案面": c.get("R6")}.get(name)


def build_face_states(res):
    """①–⑥ + 口径差栏 + 自指 → 五态（PASS/FAIL/BLOCKED/REPORT/UNKNOWN）。口径见 FACES_STATES 注释。

    ★ **host-bound 面（⑧家目录批准件）不在这里出态** —— 由 `face_records` 统一给
      `HOST-REQUIRED · SKIPPED-HOST-BOUND`（不计 pass/fail）；未产出记录的面由 `face_records` 合成缺面。
    """
    c = res.get("counts")
    fj = res.get("face_judged") or {}
    # counts is None ⇒ audit 在读到任何面之前就短路（扫面不在盘 / 基线不在盘 / 扫描面为空）
    # ⇒ 六个登记面**一条都没判**，结论 = UNKNOWN（不是 BLOCKED：BLOCKED 留给「该面真源读不到」）。
    unknown_all = c is None
    st = {}
    for name in FACES[:6]:
        if unknown_all:
            st[name] = "UNKNOWN"
            continue
        if not fj.get(name):
            st[name] = "BLOCKED"          # 该面自己的真源读不到 / 面判不了
            continue
        n = _face_count(res, name)
        if name in ("④脚本台账", "⑥失败文案面"):
            st[name] = "REPORT" if n else "PASS"    # 旁栏不进退码：有发现记 REPORT
        else:
            st[name] = "FAIL" if n else "PASS"
    st["口径差栏"] = ("BLOCKED" if (res.get("faces") or {}).get("口径差") is None
                    else "REPORT")    # 参照读数 · 不并入判红；台账口径扫面缺 ⇒ 参照读数也判不了
    st["自指"] = "UNKNOWN" if unknown_all else "REPORT"   # 自指依赖分母/面③⑤ ⇒ 整跑短路时未知
    return st


def _verdict_text(res, rc):
    if res.get("counts") is None:
        return "判不了 ⇒ 不给结论（rc=2）"
    c = res["counts"]
    bad = c["R1"] + c["R2"] + c["R3"] + c["R5"]
    unjudged = len(res["not_judged"]) + len(res["problems"]) + (1 if c["R4"] is None else 0)
    den_ok = bool(res.get("den_from_baseline")) and not res.get("den_scan_empty")
    if unjudged or (not res["new_files"] and not den_ok):
        return "判不了 / 分母 0 ⇒ 不给结论（rc=2）"
    if not res["new_files"]:
        return "判到了 —— 全树 ∖ 冻结基线 = 0 件新件（正常态；五张登记表无新件可判）"
    if bad:
        return "%d 条缺登记（报告档 rc=%d）" % (bad, rc)
    return "判到了 —— 五张登记表逐面 0 条缺登记"


def face_detail(res, states):
    """`--json faces` 断面：逐面详情（state / judged / 缺登记数 / 命中件名）。"""
    items = res.get("items") or {}
    fj = res.get("face_judged") or {}
    c = res.get("counts") or {}
    d = res.get("faces") or {}
    out = {
        "①公开标记表": {"缺登记": c.get("R1"), "items": items.get("R1")},
        "②干跑登记": {"缺登记": c.get("R2"), "items": items.get("R2"),
                    "不适用件": res.get("dry_na")},
        "③门①两张真源": {"缺登记": c.get("R3"), "items": items.get("R3"),
                       "弱挂载": res.get("weak"), "通用语法步兜住": res.get("fourth_tier")},
        "④脚本台账": {"缺登记": c.get("R4"), "items": items.get("R4"),
                    "台账路径": (res.get("ledger") or {}).get("path"),
                    "台账行数": (res.get("ledger") or {}).get("rows")},
        "⑤两器排除表": {"缺登记": c.get("R5"), "items": items.get("R5")},
        "⑥失败文案面": {"未改文案": c.get("R6"), "文案": (res.get("wording") or {}).get("lines")},
        "口径差栏": d.get("口径差"),
    }
    for name in FACES[:6]:
        out[name]["judged"] = bool(fj.get(name))
        out[name]["state"] = states[name]
    return out


def self_detail(res):
    """`--json self` 断面：本门对自身八面的登记情形（逐字对齐人面向「★ 自指」那一行）。"""
    marks = res.get("marks_public_no")
    hits = res.get("dry_hits")
    items = res.get("items") or {}
    return {
        "path": SELF_REL,
        "在分母内": SELF_REL in (res.get("new_files") or []),
        "面①": ("已登记（public=%s）" % marks[SELF_REL]) if (marks and SELF_REL in marks) else "未登记",
        "面②": "需登记" if (hits and SELF_REL in hits) else "不适用（正文不在该口径的扫描面内）",
        "面③": "缺登记" if SELF_REL in (items.get("R3") or []) else "不在缺登记名单",
        "面⑤": "缺登记" if SELF_REL in (items.get("R5") or []) else "不在缺登记名单",
    }


def run_json(root, new_files, inv, strict, baseline, selector):
    """`--json <断面>`：只出机器面 JSON（人面向 stdout 让位）+ 与报告档同源退码。"""
    sel = [s.strip() for s in (selector or "").split(",") if s.strip()]
    unknown = [s for s in sel if s not in ("faces", "self")]
    if unknown:
        print("⛔ 用法错（rc=2）：`--json` 只认断面 faces,self；不认：%s" % ",".join(unknown),
              file=sys.stderr)
        return 2
    if not sel:
        print("⛔ 用法错（rc=2）：`--json` 需要断面（faces,self 任一）", file=sys.stderr)
        return 2
    res = audit(root, new_files=new_files, inv=inv, baseline=baseline)
    rc, _bad, _unjudged = _verdict(res, strict)
    faces = expected_faces()                 # ★ 跑前先生成 expected_faces 清单
    states = build_face_states(res)
    records = face_records(res, states=states, faces=faces)
    counts = {s: 0 for s in FACES_STATES}
    for rec in records.values():
        counts[rec["state"]] = counts.get(rec["state"], 0) + 1
    missing = sorted(f for f, r in records.items() if r["reason_code"] == REASON_MISSING_FACE)
    host_bound = sorted(f for f, r in records.items() if r["reason_code"] == REASON_HOST_BOUND)
    all_green = (rc == 0 and counts["FAIL"] == 0 and counts["BLOCKED"] == 0
                 and counts["UNKNOWN"] == 0)
    obj = {
        "schema": "zerg/gate-json/v1",
        "tool": "check-newfile-registry.py",
        "gate": "门76 新件登记完整性",
        "repo": root,
        "selector": sel,
        "strict": bool(strict),
        "rc": rc,
        "expected_faces": faces,
        "reported_faces": sorted(f for f, r in records.items()
                                 if r["reason_code"] != REASON_MISSING_FACE),
        "face_states": {f: records[f]["state"] for f in faces},
        "face_records": records,
        "counts": counts,
        "host_required_faces": host_bound,
        "host_required_count": len(host_bound),
        "missing_faces": missing,
        "missing_count": len(missing),
        "fully_exercised": (len(missing) == 0 and len(host_bound) == 0),
        "all_green": all_green,
        "verdict": _verdict_text(res, rc),
    }
    if "faces" in sel:
        obj["faces"] = face_detail(res, states)
    if "self" in sel:
        obj["self"] = self_detail(res)
    sys.stdout.write(json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=False) + "\n")
    return rc


def run(root, new_files, inv, strict, baseline=None):
    res = audit(root, new_files=new_files, inv=inv, baseline=baseline)
    rc, bad, unjudged = _verdict(res, strict)          # 出口判决唯一真源（同 `run_json`）
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
        return rc
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
    den_ok = bool(res.get("den_from_baseline")) and not res.get("den_scan_empty")
    if unjudged or (not res["new_files"] and not den_ok):
        print("  判据结论：**判不了 / 分母 0 ⇒ 不给结论（rc=2）**（真的没有 ≠ 判不了）")
        return rc
    if not res["new_files"]:
        print("  判据结论：判到了 —— **全树扫过、确定性无新件**（现读件集 ∖ 冻结基线 = 0 件 · 正常态 · rc=0）")
        return rc
    if bad:
        print("  判据结论：**%d 条缺登记**（逐条点名如上）；报告档 rc=%d（只报告 · 不接闸）"
              % (bad, rc))
        return rc
    print("  判据结论：判到了 —— 五张登记表逐面 **0 条缺登记**（面④/⑥ 见上）")
    return rc


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

    # ── 查缺档 / host-bound / 缺面（合成夹具 · 成对）──────────────────────────
    def add_check(name, want, got, ok):
        checks.append((name, want, got, ok))

    def git_fixture(root):
        """建一个**真** git 夹具（含被跟踪件 scripts/a.sh 并提交）。"""
        os.makedirs(os.path.join(root, "scripts"), exist_ok=True)
        with open(os.path.join(root, "scripts", "a.sh"), "w") as fh:
            fh.write("#!/usr/bin/env bash\necho a\n")
        env = dict(os.environ)
        env.update({"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
                    "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com"})
        for a in (("init", "-q"), ("add", "-A"), ("commit", "-q", "-m", "init")):
            subprocess.run(["git", "-C", root] + list(a), capture_output=True, text=True, env=env)
        return root

    # (A) host-bound 面 + 缺面聚合（纯数据 · 不碰 git）
    tmpA = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        build_fixture(tmpA, probe=probe, register={"①", "②", "③", "⑤"})
        open(os.path.join(tmpA, "inv.tsv"), "w").write("件\n%s\n" % probe)
        resA = audit(tmpA, new_files=[probe], inv="inv.tsv", baseline=None)
        recsA = face_records(resA)
        add_check("查缺档·面⑧ host-bound ⇒ state=HOST-REQUIRED", 1,
                  1 if recsA["⑧家目录批准件"]["state"] == "HOST-REQUIRED" else 0,
                  recsA["⑧家目录批准件"]["state"] == "HOST-REQUIRED")
        add_check("查缺档·面⑧ reason_code=SKIPPED-HOST-BOUND", 1,
                  1 if recsA["⑧家目录批准件"]["reason_code"] == REASON_HOST_BOUND else 0,
                  recsA["⑧家目录批准件"]["reason_code"] == REASON_HOST_BOUND)
        add_check("查缺档·面⑧ decided=false（不当绿也不当红）", 1,
                  1 if recsA["⑧家目录批准件"]["decided"] is False else 0,
                  recsA["⑧家目录批准件"]["decided"] is False)
        cntA = {}
        for _r in recsA.values():
            cntA[_r["state"]] = cntA.get(_r["state"], 0) + 1
        add_check("查缺档·HOST-REQUIRED 单列计数=1（不计入 PASS/FAIL）", 1,
                  1 if (cntA.get("HOST-REQUIRED") == 1 and cntA.get("FAIL", 0) == 0) else 0,
                  cntA.get("HOST-REQUIRED") == 1 and cntA.get("FAIL", 0) == 0)
        bogus = "⑨合成缺面"
        recs_missing = face_records(resA, faces=expected_faces() + [bogus])
        add_check("缺面·未产出记录的面 ⇒ 合成 UNKNOWN", 1,
                  1 if recs_missing[bogus]["state"] == "UNKNOWN" else 0,
                  recs_missing[bogus]["state"] == "UNKNOWN")
        add_check("缺面·reason_code=MISSING_FACE", 1,
                  1 if recs_missing[bogus]["reason_code"] == REASON_MISSING_FACE else 0,
                  recs_missing[bogus]["reason_code"] == REASON_MISSING_FACE)
        add_check("缺面·decided=false", 1,
                  1 if recs_missing[bogus]["decided"] is False else 0,
                  recs_missing[bogus]["decided"] is False)
        rc_m = _verdict(resA, True, faces=expected_faces() + [bogus])[0]
        add_check("缺面·退码钉 2（缺面不许当齐）", 2, rc_m, rc_m == 2)
        rc_ok = _verdict(resA, True)[0]
        add_check("正控·无缺面 ⇒ 不因聚合器变 rc", 0, rc_ok, rc_ok == 0)
        add_check("正控·缺面注入 ⇒ 2 · 不注入 ⇒ 0（区分度）", 2, rc_m,
                  rc_m == 2 and rc_ok == 0)
    finally:
        shutil.rmtree(tmpA, ignore_errors=True)

    # (B) 三合并式四负控（真 git 夹具 · 仓外）
    # 负控① 未跟踪新件（不 git add）⇒ 红；并证「git diff 单用看不见它」
    tmpG = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        git_fixture(tmpG)
        with open(os.path.join(tmpG, "scripts", "new-probe.sh"), "w") as fh:
            fh.write("#!/usr/bin/env bash\necho new\n")
        rU = worktree_three_way(tmpG, ["scripts"])
        rcU = _chk_missing_rc(rU)
        add_check("负控①·未跟踪新件（不 git add）⇒ 红 rc=1", 1, rcU,
                  rcU == 1 and rU["untracked"] == ["scripts/new-probe.sh"])
        _drc, _dout, _derr = _git(tmpG, ["diff", "--name-only", "--exit-code", "HEAD", "--", "scripts"])
        add_check("负控①·git diff 单用看不见 untracked（rc=0 · 洞真被 ② 补上）", 0, _drc,
                  _drc == 0 and _dout == [])
    finally:
        shutil.rmtree(tmpG, ignore_errors=True)
    # 负控② 被 .gitignore 命中的路径 ⇒ 红（ignored 捕获）· 绝不 rc=0
    tmpI = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        git_fixture(tmpI)
        with open(os.path.join(tmpI, ".gitignore"), "w") as fh:
            fh.write("scripts/ignored/\n")
        os.makedirs(os.path.join(tmpI, "scripts", "ignored"), exist_ok=True)
        with open(os.path.join(tmpI, "scripts", "ignored", "x.sh"), "w") as fh:
            fh.write("#!/usr/bin/env bash\necho x\n")
        rI = worktree_three_way(tmpI, ["scripts"])
        rcI = _chk_missing_rc(rI)
        add_check("负控②·.gitignore 命中 ⇒ 红（ignored 捕获）· 绝不 rc=0", 1, rcI,
                  rcI == 1 and rI["ignored"] == ["scripts/ignored/x.sh"])
    finally:
        shutil.rmtree(tmpI, ignore_errors=True)
    # 负控③ 零执行探针（空件集）⇒ no_data 置位 · 非绿 rc=2
    tmpE = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        git_fixture(tmpE)
        rE = worktree_three_way(tmpE, ["scripts-nonexistent"])
        rcE = _chk_missing_rc(rE)
        add_check("负控③·空件集（零执行探针）⇒ no_data + rc=2", 2, rcE,
                  rcE == 2 and rE["no_data"] is True and rE["file_set_size"] == 0)
    finally:
        shutil.rmtree(tmpE, ignore_errors=True)
    # 负控④ 真源不可读 ⇒ rc=2 · 绝不 rc=0
    tmpX = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        with open(os.path.join(tmpX, ".git"), "w") as fh:
            fh.write("gitdir: /nonexistent-gnr-selftest\n")
        rX = worktree_three_way(tmpX, ["scripts"])
        rcX = _chk_missing_rc(rX)
        add_check("负控④·真源不可读（.git 破损）⇒ rc=2 · 绝不 0", 2, rcX,
                  rcX == 2 and rX["unreadable"] is True)
    finally:
        shutil.rmtree(tmpX, ignore_errors=True)
    # 隔离：worktree 退出必清 · 生成前清空输出目录
    tmpW = tempfile.mkdtemp(prefix="gnr-selftest-")
    outd = tempfile.mkdtemp(prefix="gnr-selftest-out-")
    try:
        git_fixture(tmpW)
        with open(os.path.join(outd, "旧件残留.txt"), "w") as fh:
            fh.write("stale\n")
        rW = worktree_three_way(tmpW, ["scripts"])
        _lrc, lout, _lerr = _git(tmpW, ["worktree", "list"])
        add_check("隔离·worktree 退出必清（list 仅 1 条 = 主树）", 1, len(lout),
                  rW["worktree_cleaned"] is True and len(lout) == 1)
        _buf = io.StringIO()
        _old = sys.stdout
        sys.stdout = _buf
        try:
            rcW = run_check_missing(tmpW, ["scripts"], outdir=outd)
        finally:
            sys.stdout = _old
        add_check("隔离·干净夹具 ⇒ rc=0 · 生成前清空输出目录（旧件被清）", 0, rcW,
                  rcW == 0 and (not os.path.exists(os.path.join(outd, "旧件残留.txt")))
                  and os.path.exists(os.path.join(outd, "查缺档.json")))
    finally:
        shutil.rmtree(tmpW, ignore_errors=True)
        shutil.rmtree(outd, ignore_errors=True)

    # (C) ★ 甲/乙 分档（本片核心 · 成对负控）：--dump-baseline 打当前树后**不再必然 rc=2** ——
    #     「全树扫过（面非空）∖ 在盘基线 = 0 件」= 正常态 ⇒ 0；「基线不在盘 / 显式空件集 / 扫描面为空」⇒ 2（零执行 ✗ 不放宽）。
    tmpD = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        build_fixture(tmpD, probe=probe, register={"①", "②", "③", "⑤"})
        open(os.path.join(tmpD, "inv.tsv"), "w").write("件\n%s\n" % probe)
        baseD = os.path.join(tmpD, "baseline.tsv")
        with open(baseD, "w") as fh:
            fh.write("# 夹具冻结基线（现读件集快照）\n")
            for f in sorted(scan_ledger_face(tmpD)):
                fh.write(f + "\n")
        _old = sys.stdout

        def _quiet_rc(**kw):
            _buf = io.StringIO()
            sys.stdout = _buf
            try:
                return run(tmpD, **kw)
            finally:
                sys.stdout = _old

        rcA = _quiet_rc(new_files=None, inv="inv.tsv", strict=False, baseline=baseD)
        add_check("甲·全树扫过 ∖ 在盘基线 = 0 件 ⇒ 0（正常态 · dump 后不再必然 2）", 0, rcA, rcA == 0)
        rcB = _quiet_rc(new_files=None, inv="inv.tsv", strict=False,
                        baseline=os.path.join(tmpD, "no-such-baseline.tsv"))
        add_check("乙·基线不在盘（真源拿不到）⇒ 2（未放宽）", 2, rcB, rcB == 2)
        rcZ = _quiet_rc(new_files=[], inv="inv.tsv", strict=False, baseline=None)
        add_check("乙·显式空件集（零执行探针）⇒ 2（未放宽）", 2, rcZ, rcZ == 2)
    finally:
        shutil.rmtree(tmpD, ignore_errors=True)
    # 乙·扫描面为空（零执行）· 与甲 成对（同一 res 形状只差 den_scan_empty）
    def _res_den(baseline_derived, scan_empty):
        return {"counts": {"R1": 0, "R2": 0, "R3": 0, "R4": 0, "R5": 0, "R6": 0},
                "new_files": [], "not_judged": [], "problems": [], "faces": {}, "face_judged": {},
                "den_from_baseline": baseline_derived, "den_scan_empty": scan_empty}

    rcSE = _verdict(_res_den(True, True), False)[0]
    add_check("乙·扫描面为空（基线在盘但面空 = 零执行）⇒ 2（未放宽）", 2, rcSE, rcSE == 2)
    rcSN = _verdict(_res_den(True, False), False)[0]
    add_check("区分度·同一 res 形状：面非空 ⇒ 0 · 面空 ⇒ 2（甲/乙 真分开）", 0, rcSN,
              rcSN == 0 and rcSE == 2)

    # (D) ★ B1-5 成对负控（本片新增：① OSError 在盘但读不出 · ② 行内注释 · ③ 基线 v2 过期/只许下调/指纹读回 · ④ --sync 两态）
    tmpS = tempfile.mkdtemp(prefix="gnr-selftest-")
    try:
        build_fixture(tmpS, probe=probe, register={"①", "②", "③", "⑤"})
        open(os.path.join(tmpS, "inv.tsv"), "w").write("件\n%s\n" % probe)
        n_scan = len(scan_ledger_face(tmpS) or set())

        def _quiet(fn, *a, **kw):
            _b = io.StringIO()
            _oo = sys.stdout
            sys.stdout = _b
            try:
                return fn(*a, **kw)
            finally:
                sys.stdout = _oo

        # ① 基线在盘但读不出（chmod 000）⇒ 2 fail-closed ⟷ 同件可读 ⇒ 0（成对）
        baseRO = os.path.join(tmpS, "base-ro.tsv")
        with open(baseRO, "w") as fh:
            fh.write("# 门76 冻结基线 v2 件数=%d 指纹=deadbeefdeadbeef\n" % n_scan)
        os.chmod(baseRO, 0)
        rcRO = _quiet(run, tmpS, new_files=None, inv="inv.tsv", strict=False, baseline=baseRO)
        add_check("B1-5·① 基线在盘但读不出（chmod 000）⇒ 2（fail-closed · 未捕获 OSError 曾静默绿）",
                  2, rcRO, rcRO == 2)
        os.chmod(baseRO, 0o644)
        rcROk = _quiet(run, tmpS, new_files=None, inv="inv.tsv", strict=False, baseline=baseRO)
        add_check("B1-5·① 成对正控：同基线可读且件数吻合 ⇒ 0（读得出 ⇒ 判得了）", 0, rcROk, rcROk == 0)

        # ② load_excludes 剥行内注释（消假阳性）
        tmpL = tempfile.mkdtemp(prefix="gnr-selftest-")
        try:
            os.makedirs(os.path.join(tmpL, os.path.dirname(PUB_REL)), exist_ok=True)
            with open(os.path.join(tmpL, PUB_REL), "w") as fh:
                fh.write('EXCLUDES=(\n  "a.sh"   # 行内注释\n  "b.sh"\n)\n')
            _ex = load_excludes(tmpL)
        finally:
            shutil.rmtree(tmpL, ignore_errors=True)
        add_check("B1-5·② load_excludes 剥行内注释 ⇒ {a.sh,b.sh}（消假阳性）", 1,
                  1 if _ex == {"a.sh", "b.sh"} else 0, _ex == {"a.sh", "b.sh"})
        # ② 成对负控：原样不剥 ⇒ 注释尾巴混进件名（假阳性）——证明本修必要
        tmpL2 = tempfile.mkdtemp(prefix="gnr-selftest-")
        try:
            os.makedirs(os.path.join(tmpL2, os.path.dirname(PUB_REL)), exist_ok=True)
            with open(os.path.join(tmpL2, PUB_REL), "w") as fh:
                fh.write('EXCLUDES=(\n  "a.sh"   # 行内注释\n  "b.sh"\n)\n')
            _txt2 = read_text(os.path.join(tmpL2, PUB_REL)) or ""
            _m2 = re.search(r"EXCLUDES=\((.*?)\)", _txt2, re.S)
            _naive = {q.strip().strip('"').strip("'")
                      for q in (_m2.group(1).split("\n") if _m2 else [])
                      if q.strip() and not q.strip().startswith("#")}
        finally:
            shutil.rmtree(tmpL2, ignore_errors=True)
        add_check("B1-5·② 负控：原样不剥注释 ⇒ 注释尾混入件名（假阳性 · 证明本修必要）", 1,
                  1 if any("#" in x for x in _naive) else 0, any("#" in x for x in _naive))

        # ③ 基线 v2 过期即红（声明件数 > 现读扫面）
        baseEX = os.path.join(tmpS, "base-expired.tsv")
        with open(baseEX, "w") as fh:
            fh.write("# 门76 冻结基线 v2 件数=%d 指纹=0000000000000000\n" % (n_scan + 100))
        rcEX = _quiet(run, tmpS, new_files=None, inv="inv.tsv", strict=False, baseline=baseEX)
        add_check("B1-5·③ 基线 v2 过期（声明 %d > 现读 %d）⇒ 2（过期即红）" % (n_scan + 100, n_scan),
                  2, rcEX, rcEX == 2)

        # ③ 基线 v2 头（件数+指纹）可解析读回 + dump 只许下调拒写
        baseSmall = os.path.join(tmpS, "base-small.tsv")
        with open(baseSmall, "w") as fh:
            fh.write("# 门76 冻结基线 v2 件数=1 指纹=1111111111111111\n")
        _c, _f = parse_baseline_header(read_text(baseSmall))
        add_check("B1-5·③ 基线 v2 头（件数/指纹）读回一致", 1,
                  1 if (_c == 1 and _f == "1111111111111111") else 0,
                  _c == 1 and _f == "1111111111111111")
        rcDown = _quiet(dump_baseline, tmpS, os.path.join(tmpS, "base-down.tsv"), False)
        # 旧基线件数=1 < 现读 n_scan ⇒ 只许下调 ⇒ 干跑也拒写 rc=2
        rcDown2 = _quiet(dump_baseline, tmpS, baseSmall, False)
        add_check("B1-5·③ 基线只许下调（旧 1 < 现读 %d）⇒ 干跑即拒写 2" % n_scan, 2, rcDown2, rcDown2 == 2)

        # ④ --sync 干跑零写入 ⟷ --yes 真写（成对 · 原子写 · 无残留）
        _p = os.path.join(tmpS, MARKS_REL)
        _sha0 = hashlib.sha256(open(_p, "rb").read()).hexdigest()
        baseS = os.path.join(tmpS, "base-sync.tsv")
        with open(baseS, "w") as fh:
            fh.write("# 门76 冻结基线 v2 件数=%d 指纹=cafecafecafecafe\n" % n_scan)
        rcSD = _quiet(run_sync, tmpS, False, baseS)
        _sha1 = hashlib.sha256(open(_p, "rb").read()).hexdigest()
        add_check("B1-5·④ `--sync` 干跑 ⇒ 0 且零写入（marks 表 sha 逐字同）", 0, rcSD,
                  rcSD == 0 and _sha0 == _sha1)
        rcSY = _quiet(run_sync, tmpS, True, baseS)
        _sha2 = hashlib.sha256(open(_p, "rb").read()).hexdigest()
        add_check("B1-5·④ `--sync --yes` ⇒ 0 且真写（marks 表 sha 变 · 与干跑成对）", 0, rcSY,
                  rcSY == 0 and _sha0 != _sha2)
        _resid = [f for f in os.listdir(os.path.dirname(_p)) if f.startswith(".gnr-marks-")]
        add_check("B1-5·④ 原子写无残留临时件（temp+fsync+rename）", 0, len(_resid), len(_resid) == 0)
    finally:
        shutil.rmtree(tmpS, ignore_errors=True)

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
    ap.add_argument("--dump-baseline", default=None, help="把现读件集写成基线（仓外路径）后退出；**默认干跑不写**，带 `--yes` 才真落")
    ap.add_argument("--inventory", default=None, help="面④ 仓外台账 tsv（不给 ⇒ 面④ 判不了 · 需人签）")
    ap.add_argument("--new-file", action="append", default=None, help="直接指分母（可重复 · 两态取证用）")
    ap.add_argument("--strict", action="store_true", help="阻断档：R1–R5 任一 > 0 ⇒ rc=1（默认报告档 rc=0）")
    ap.add_argument("--check-missing", action="store_true",
                    help="查缺档（只读）：三合并式（tracked改动 ∥ untracked ∥ ignored）**任一非空即红**"
                         " · 隔离 HEAD 工作树（退出必清）· 退码 0/1/2")
    ap.add_argument("--paths", default=None,
                    help="查缺档判据路径（逗号分隔 · 默认 scripts）")
    ap.add_argument("--outdir", default=None,
                    help="查缺档报告落点：**生成前先清空该目录**（防旧件残留看着一致）· 只跟 `--check-missing` 连用")
    ap.add_argument("--json", default=None, metavar="SECTIONS",
                    help="机器面 JSON（逗号分隔断面：faces = 六张登记表逐面状态；self = 自指那一段）"
                         " · 给了就只出 JSON（人面向 stdout 让位）")
    ap.add_argument("--yes", action="store_true",
                    help="写面确认：只有 `--dump-baseline` 会写件，且**默认干跑不写**；带 `--yes` 才真落"
                         "（本件比「默认真写」的写器更保守）")
    ap.add_argument("--self-test", action="store_true", help="只跑合成夹具自检（不扫真仓）")
    ap.add_argument("--sync", action="store_true",
                    help="补齐档（B1-4）：默认干跑；只写纯派生面（面① 占位行）；面③⑤⑧ 只报「该跑哪条正门」；"
                         "显式 `--yes` 才真写且 write-temp+fsync+rename")
    args = ap.parse_args()
    # 用法错（rc=2 · fail-closed）：旗标组合不合法，先判、不执行任何动作
    if args.yes and not args.dump_baseline and not args.sync:
        print("⛔ 用法错（rc=2）：`--yes` 只跟 `--dump-baseline`/`--sync` 连用（本件**默认干跑不写**）", file=sys.stderr)
        return 2
    if args.sync and (args.dump_baseline or args.check_missing):
        print("⛔ 用法错（rc=2）：`--sync` 不与 `--dump-baseline`/`--check-missing` 同用", file=sys.stderr)
        return 2
    if args.json is not None and args.dump_baseline:
        print("⛔ 用法错（rc=2）：`--json` 只跟报告档连用（不与 `--dump-baseline` 同用）", file=sys.stderr)
        return 2
    if args.outdir and not args.check_missing:
        print("⛔ 用法错（rc=2）：`--outdir` 只跟 `--check-missing` 连用", file=sys.stderr)
        return 2
    if args.check_missing and args.dump_baseline:
        print("⛔ 用法错（rc=2）：`--check-missing` 不与 `--dump-baseline` 同用", file=sys.stderr)
        return 2
    if args.self_test:
        return run_self_test()
    root = os.path.abspath(args.repo)
    if args.sync:  # ④ B1-4 补齐档：只写纯派生面；面③⑤⑧ 只报「该跑哪条正门」（人拍档 / 人签）
        if args.json is not None:
            print("⛔ 用法错（rc=2）：`--sync` 不与 `--json` 同用（补齐档无人面 JSON 断面）", file=sys.stderr)
            return 2
        return run_sync(root, args.yes, args.baseline)
    if args.check_missing:
        # `--check-missing` 的机器面断面只认 `chk`（非空且逐字）
        sel = [s.strip() for s in (args.json or "").split(",") if s.strip()]
        if args.json is not None and (not sel or any(s != "chk" for s in sel)):
            print("⛔ 用法错（rc=2）：`--check-missing` 的 `--json` 只认断面 chk；不认：%s"
                  % (args.json or ""), file=sys.stderr)
            return 2
        cpaths = [p.strip() for p in (args.paths or "").split(",") if p.strip()]
        return run_check_missing(root, cpaths, outdir=args.outdir, as_json=(args.json is not None))
    if args.dump_baseline:
        return dump_baseline(root, args.dump_baseline, args.yes)
    if args.json is not None:
        return run_json(root, args.new_file, args.inventory, args.strict, args.baseline, args.json)
    return run(root, args.new_file, args.inventory, args.strict, args.baseline)


if __name__ == "__main__":
    sys.exit(main())

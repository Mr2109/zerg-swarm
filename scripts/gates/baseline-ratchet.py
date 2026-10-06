#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""baseline-ratchet.py — `C2`：baseline **只许由一条命令改**（只读 diff 先行 · 2026-09-23 · 波③）

为什么要有它（缺口账 §附录十五 A 组 `C2` · 调研-缺口-门禁证据 §二 `R-03` · `Q-006`）
-----------------------------------------------------------------------------------
本仓的「棘轮数字」（baseline）今天是**手改的常量**：门⑫ 报「202 > baseline 198」时，
人只能拿编辑器打开 `scripts/gates/doc-cmds-baseline.json` 改一个数（`Q-006` 逐字：
「门⑫ 报「202 > baseline 198」时只能手改」）。于是两件事**两条都没有**：
  ① **改之前看不到差** —— 改完才知道这一改放宽了几条；今天把 198 抬到 202，事后没有一句话
     交代「谁在何时、为什么」；
  ② **改之后没有痕** —— baseline 是**放宽判据**（同族于 one-way door），改动本身**不留审计**。
本件把「改 baseline」收成**一条命令**：
  · 不带写旗标 ⇒ **只报差**（将升/将降 **几条** + **逐条件名**），仓内件 `sha256` 一字不变；
  · 带写旗标 ⇒ 改数 **并且** 往 append-only 台账追一条「**件 · 旧值 → 新值 · 理由 · 时间**」；
  · **无记录 ⇒ 判红**（`R-03` 逐字）—— 这是本件唯一能有的牙齿：baseline 是普通 JSON 件，
    谁也拦不住别人拿编辑器手改；**能拦的是「手改之后对不上台账」**。

牙齿长在哪（说清楚，不吹）
--------------------------
`baseline` 只许**由一条命令改**这件事，脚本没法从技术上独占（件在盘上、编辑器在别人手里）。
本件给的是**可机检的两件**：① 改动的**唯一通道**（本命令，带留痕）；② 改动的**事后对账**
（现值 ≠ 台账最后一条 `new` ⇒ 判红）。所以「只许一条命令改」落地为
「**手改必被抓**」—— 判据可跑、可红、可复现，不许当口号。

条件表是**声明式**的（不写死任何数字 ✗）
----------------------------------------
本件一个数字都不记：每个条件的**现值**从件里读、**现测值**「现跑」该条件的门脚本再解析它的现印行
（`--conditions` 可机器读地打印整张表；`--diff --print-measure` 打印每条测量命令）。
今天四个条件（逐条对到它自己的门与件）：

  | 条件名                        | 门   | 件                                        | 键                     |
  |-------------------------------|------|-------------------------------------------|------------------------|
  | `gate-coverage.baseline`      | 门① | `scripts/gates/gate-coverage.config.json` | `baseline`             |
  | `cli-contract.blanks`         | 门⑪ | `scripts/gates/cli-contract-baseline.json`| `blanks`               |
  | `doc-cmds.devdocs_report_hits`| 门⑫ | `scripts/gates/doc-cmds-baseline.json`    | `devdocs_report_hits`  |
  | `i18n.allowed_count`          | i18n 门禁（**无门号** —— 它今天还不在步骤表里，缺口 `C-10`） | `ui/scripts/i18n-baseline.json` | `allowed_count` |

★ 第四条（缺口 `C-02` · 2026-09-24）为什么这样接：i18n 门的中文白名单（`allowed`）**是**一条棘轮 ——
「只许收不许放」本来就是它的口径，可它的数今天全靠手改、没有任何留痕。本条把那一格接进本表：件里多出
一格**声明数** `allowed_count`（= 白名单的声明条数），现测面**现跑** `ui/scripts/check-i18n.py --json`
读它自己印的 `baseline_allowed`。于是「放开」在两个地方各有一次判红，缺一条就是「放开了没人拦」：
  · **就地**（i18n 门自己的 G4）：白名单实条数 > 声明数 ⇒ 该门当场 **ERROR**（rc=1）；
  · **事后**（本件 `--check`）：声明数与台账最后一条对不上 ⇒ **红**（有一条没记录的手改）。
收紧（收）那一侧不挡：声明数 > 实条数只落**提示**，随后走本件写入口把数同降即可。
★ 它的门号一格照实写：这门**在步骤表里没有编号**（`precommit-gates.sh` 里 7 处点名、无一处是步骤 ——
缺口 `C-10` 未落）⇒ 本表不替它编一个号。

★ 测量是**现跑**、不是常量：跑不通 / 解析不出判据行 ⇒ **rc=2 不给结论**（不许猜、不许拿记忆充数）。
★ 门脚本自己 rc=1（= 现值已超 baseline）**仍是一次有效测量**（正是「将升」那一格要看的）；
  只有 rc=2（门自己说「不给结论」）才传染成本件的**不给结论**。

上调为什么比下调更严（`R-03` 的「放宽判据」逐字）
--------------------------------------------------
三个条件的口径都是「**只许下调、永不上调**」（件里各有一段自述）。上调 = **放宽判据** ⇒
即使带了 `--reason` 也**还要**一枚显式的 `--allow-raise`，否则 fail-closed rc=2 不写。
这不是多一道手续：今天「198 → 202」正是这一格，它必须**看得见**。

audit 台账（append-only · `scripts/gates/baseline-audit.jsonl`）
----------------------------------------------------------------
★ **本件「台账」= `scripts/gates/baseline-audit.jsonl` =【权威台账】**（`seq` 连号 = append-only 硬证据 · 值链/件链只提示不改判）—— 与 `check-gate-coverage.md` 里那份「债务台账」（叙事面 · 只作旁证）**同名不同面 ⇒ 勿互证**（口径已拍 · GAP-20260927-156）。
一行一条 JSON（compact · 一行一记录，逐行可 `json.loads`）：

  {"seq":1,"ts":"…+08:00","kind":"write|anchor","name":"gate-coverage.baseline",
   "gate":"门①","file":"…","old":69,"new":60,"reason":"…","by":"Name <mail>",
   "sha256_before":"…","sha256_after":"…","measure":"现跑 …"}

三条自证（`--check` / `--self-test` 里成对做出；硬/软分开，理由写死在下面）：
  · **连号 = 硬**：`seq` 必须 = 行号（1..N）—— 删行/改写 ⇒ 断号 ⇒ **不给结论**（append-only 被破）；
  · **值链 / 件链 = 提示（不改判）**：链会因为**正当理由**断（例：`gate-coverage.config.json`
    被别的批改了 `scan`/`whitelist` 那些键 —— 值没动、件 sha 却变了），把它们当硬错会让本件
    被一次无关的批改**永久锁死**；「值有没有被偷偷改过」这件事由下面那条红判（那才有牙）：
  · **红（rc=1）**：现值 ≠ 台账最后一条 `new` ⇒ **有一条没记录的手改**；或该条件**一条记录都没有**。

★ **红里的两态要能分开**（缺口 `Q-144` · 设计 `设计-CLI机器读面-v1.0-20260924.md` §3.3 丙档 + §4.2 第 23 行）：
「**台账件不在盘上**」与「**件在盘上但 0 条记录**」今天**同 rc=1** ⇒ 机器面分不出是哪一种。
本件**只加字段、不动退码** ✗：`rc` 照旧（两态都 `1`），另给一枚**错因码**
`reason ∈ {ledger_absent, no_record}`（`--json` 档进包封；人面档在判红那句下面补一行固定前缀短语）。
★ 两态的判因**只看台账件本身**：件不在 ⇒ `ledger_absent`；件在而**一条可用记录都没有** ⇒ `no_record`；
  有记录（或台账解不动/形状坏了 ⇒ 那两态各有自己的 `rc=2` 判词）⇒ `reason` 为 `null`，**不硬塞**。

用法
----
    python3 scripts/gates/baseline-ratchet.py                       # = --view（只读视图 + 判）
    python3 scripts/gates/baseline-ratchet.py --diff                # 只报差（将升/将降几条 + 逐条件名）
    python3 scripts/gates/baseline-ratchet.py --diff --only gate-coverage.baseline
    python3 scripts/gates/baseline-ratchet.py --check               # 无记录 ⇒ 判红（rc=1）
    python3 scripts/gates/baseline-ratchet.py --check --json        # 同上 + 包封里多一枚 `reason`（`Q-144`）
    python3 scripts/gates/baseline-ratchet.py --conditions          # 条件表（机器可读 JSON）
    python3 scripts/gates/baseline-ratchet.py --diff --readings r.tsv   # 注入现测读数（`名=值` 逐行）
    python3 scripts/gates/baseline-ratchet.py --write --reason '…'       # **唯一写入口**（改数 + 留痕）
    python3 scripts/gates/baseline-ratchet.py --write --reason '…' --allow-raise   # 上调（放宽判据）
    python3 scripts/gates/baseline-ratchet.py --anchor --reason '…'      # 补「现状锚」（不改任何件）
    python3 scripts/gates/baseline-ratchet.py --self-test                # 合成件成对负控（不碰真目标）
    python3 scripts/gates/baseline-ratchet.py --json                     # 与上列任一档同用（机器可读）

退码（照 `K3`，不新立）
----------------------
    0 = 绿（有记录且现值与台账一致 / 只报差本身跑成）
    1 = **红**（无记录 · 现值 ≠ 台账最后一条 ⇒ 「有一条没记录的手改」）
    2 = **不给结论**（缺件/键解不动/测量命令失败或解析不出/台账解不动/自检不过/用法错）
    ★ 「只报差」那档的差（将升几条）**不是红** —— 差是事实，红是「没记录」。
"""

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from datetime import datetime

# ── 台账件（append-only · 一件）───────────────────────────────────────────────
LEDGER_REL = os.path.join("scripts", "gates", "baseline-audit.jsonl")

# ── 仓根标记（同门禁脚本的口径：入口 + 一棵构建树）────────────────────────────
ROOT_MARKERS = (os.path.join("scripts", "gates", "precommit-gates.sh"), os.path.join("core", "go.mod"))

# ── 条件表（声明式 · **不写死任何数字**：现值读件、现测现跑）────────────────────
#   每格：name 条件名（逐条件名要报到它）· gate 门 · file 件 · key 件里的键
#         policy 口径（`down` = 只许下调）· unit 这个数在数什么
#         measure 现测命令（列表 = argv，直接 exec，不经 shell）· measure_re 现印行里的判据
CONDITIONS = (
    {
        "name": "gate-coverage.baseline",
        "gate": "门①",
        "file": os.path.join("scripts", "gates", "gate-coverage.config.json"),
        "key": "baseline",
        "policy": "down",
        "unit": "「闸文件里查无此名」的脚本条数",
        "measure": ["python3", os.path.join("scripts", "gates", "check-gate-coverage.py"), "--baseline"],
        "measure_re": r"③\s*实测命中数[^=]*=\s*(\d+)",
    },
    {
        "name": "cli-contract.blanks",
        "gate": "门⑪",
        "file": os.path.join("scripts", "gates", "cli-contract-baseline.json"),
        "key": "blanks",
        "policy": "down",
        "unit": "命令面矩阵的空白格条数",
        "measure": ["python3", os.path.join("scripts", "gates", "check-cli-contract.py")],
        "measure_re": r"空白\s*(\d+)\s*条",
    },
    {
        "name": "doc-cmds.devdocs_report_hits",
        "gate": "门⑫",
        "file": os.path.join("scripts", "gates", "doc-cmds-baseline.json"),
        "key": "devdocs_report_hits",
        "policy": "down",
        "unit": "开发文档面里手写命令示例的命中数",
        "measure": ["python3", os.path.join("scripts", "gates", "check-doc-cmds.py"), "--scope", "devdocs"],
        "measure_re": r"命中\s*(\d+)\s*处",
    },
    {
        # 缺口 `C-02`（2026-09-24）：i18n 中文门禁基线（白名单）进棘轮 · 方向 = 只许收不许放。
        #   现值 = 基线件里的**声明数**那一格；现测 = 现跑该门 `--json` 读它自己印的 `baseline_allowed`
        #   （= 白名单实条数）。两者不等时：门自己先红（G4「放开被拦」），本件再判「将升/将降」。
        #   `gate` 一格**不编号**：这门今天不在步骤表里（缺口 `C-10` 未落）⇒ 照实写它的名字，
        #   不给它编一个门号。
        "name": "i18n.allowed_count",
        "gate": "i18n 门禁",
        "file": os.path.join("ui", "scripts", "i18n-baseline.json"),
        "key": "allowed_count",
        "policy": "down",
        "unit": "i18n 中文门禁基线白名单的**声明条数**（只许收不许放：白名单多一条 ⇒ 门 G4 当场红）",
        "measure": ["python3", os.path.join("ui", "scripts", "check-i18n.py"), "--json"],
        "measure_re": r'"baseline_allowed"\s*:\s*(\d+)',
    },
)

MEASURE_TIMEOUT = 1800


# ── 基础 ─────────────────────────────────────────────────────────────────────
def find_root(start):
    """从 start 上溯找仓根（入口 + 一棵构建树两件都在）。找不到 ⇒ None。"""
    cur = os.path.abspath(start)
    while True:
        if all(os.path.exists(os.path.join(cur, m)) for m in ROOT_MARKERS):
            return cur
        parent = os.path.dirname(cur)
        if parent == cur:
            return None
        cur = parent


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def now_iso():
    return datetime.now().astimezone().replace(microsecond=0).isoformat()


def cond_by_name(name, conds=CONDITIONS):
    for c in conds:
        if c["name"] == name:
            return c
    return None


# ── 现值（读件）──────────────────────────────────────────────────────────────
def read_value(root, cond):
    """⇒ (现值|None, 件 sha256|None, 错因|None)。现值必须是**非负整数**（bool 不算）。"""
    path = os.path.join(root, cond["file"])
    if not os.path.isfile(path):
        return None, None, "件不在：%s" % cond["file"]
    sha = sha256_file(path)
    try:
        with open(path, encoding="utf-8") as f:
            doc = json.load(f)
    except Exception as exc:                                   # noqa: BLE001
        return None, sha, "件解不动（JSON）：%s" % exc
    if not isinstance(doc, dict) or cond["key"] not in doc:
        return None, sha, "件里没有键 `%s`" % cond["key"]
    val = doc[cond["key"]]
    if isinstance(val, bool) or not isinstance(val, int) or val < 0:
        return None, sha, "键 `%s` 不是非负整数（现 %r）" % (cond["key"], val)
    return val, sha, None


# ── 现测值（现跑该条件的门脚本 · **禁写常量**）───────────────────────────────
def measure_value(root, cond, readings=None):
    """⇒ (现测值|None, 来源串|None, 错因|None)。readings 是注入读数（只给自检/复核用）。"""
    if readings is not None and cond["name"] in readings:
        return readings[cond["name"]], "注入读数（--readings）", None
    cmd = list(cond["measure"])
    try:
        pr = subprocess.run(cmd, cwd=root, capture_output=True, text=True, timeout=MEASURE_TIMEOUT)
    except FileNotFoundError as exc:
        return None, None, "现测命令起不来（%s）：%s" % (" ".join(cmd), exc)
    except subprocess.TimeoutExpired:
        return None, None, "现测命令超时（%ds）：%s" % (MEASURE_TIMEOUT, " ".join(cmd))
    out = (pr.stdout or "") + "\n" + (pr.stderr or "")
    m = re.search(cond["measure_re"], out)
    src = "现跑 `%s`（rc=%d）" % (" ".join(cmd), pr.returncode)
    if not m:
        tail = "\n".join(out.strip().splitlines()[-6:])
        return None, src, ("现测命令输出里找不到判据行（%s）⇒ **判不了**（不许猜）\n%s" % (cond["measure_re"], tail))
    return int(m.group(1)), src, None


def read_readings(path, conds=CONDITIONS):
    """读数件：逐行 `条件名 = 值`（`#` 注释 · 空行忽略）。⇒ (dict, 错因|None)。"""
    got, errs = {}, []
    try:
        with open(path, encoding="utf-8") as f:
            lines = f.read().splitlines()
    except OSError as exc:
        return None, "读数件读不到：%s" % exc
    for i, line in enumerate(lines, 1):
        if line.lstrip().startswith("#"):      # 注释**只认整行**（条件名里可以带 `#`）
            continue
        s = line.strip()
        if not s:
            continue
        if "=" not in s:
            errs.append("第 %d 行缺 `=`（要 `条件名=值`）" % i)
            continue
        name, _, val = s.partition("=")
        name, val = name.strip(), val.strip()
        if cond_by_name(name, conds) is None:
            errs.append("第 %d 行的条件名不认：%s" % (i, name))
            continue
        if not re.fullmatch(r"\d+", val):
            errs.append("第 %d 行的值不是非负整数：%r" % (i, val))
            continue
        got[name] = int(val)
    if errs:
        return None, "；".join(errs)
    return got, None


# ── 台账 ─────────────────────────────────────────────────────────────────────
def ledger_path(root):
    return os.path.join(root, LEDGER_REL)


def load_ledger(root):
    """⇒ (entries, 错因|None)。空/缺件 ⇒ ([], None)。"""
    path = ledger_path(root)
    if not os.path.isfile(path):
        return [], None
    entries, errs = [], []
    with open(path, encoding="utf-8") as f:
        for i, line in enumerate(f, 1):
            s = line.strip()
            if not s:
                continue
            try:
                e = json.loads(s)
            except Exception as exc:                            # noqa: BLE001
                errs.append("第 %d 行解不动：%s" % (i, exc))
                continue
            if not isinstance(e, dict) or "seq" not in e or "name" not in e:
                errs.append("第 %d 行不是一条台账记录（缺 seq/name）" % i)
                continue
            entries.append(e)
    if errs:
        return entries, "；".join(errs)
    return entries, None


# ── 台账两态的**错因码**（缺口 `Q-144` · 只加判因，**不动数字与退码** ✗）────────────
LEDGER_ABSENT = "ledger_absent"      # 台账件**不在盘上**（`load_ledger` 走的是 isfile 失败那条 ⇒ 空表）
LEDGER_NO_RECORD = "no_record"       # 件在盘上，但**一条可用记录都没有**（空件 / 全是空行 / 全是坏行）
LEDGER_REASON_TEXT = {
    LEDGER_ABSENT: "台账件**不在盘上**：%s —— 真源缺件（「读不到」不许当绿）" % LEDGER_REL,
    LEDGER_NO_RECORD: "台账件**在盘上、却一条记录都没有**：%s —— 手改/新建都无处对账" % LEDGER_REL,
}


def ledger_reason(root):
    """⇒ `ledger_absent` / `no_record` / None。

    这条只管「**红（rc=1）里那两态**」——今天它们**同 rc=1**，机器面分不出是哪种（缺口 `Q-144`）：
      · 件不在盘上 ⇒ `ledger_absent`；件在而 0 条记录 ⇒ `no_record`；
      · 有记录 ⇒ None；台账**解不动**（`lerr`）或 `seq` 断号 ⇒ None —— 那两态本身走 `rc=2`，
        各有自己的判词，**不往这两个码里塞**（两义同码正是本件要治的病 ✗）。
    ★ 只读一件（`os.path.isfile` + `load_ledger`），**不写任何件**、**不改任何数**。
    """
    if not os.path.isfile(ledger_path(root)):
        return LEDGER_ABSENT
    entries, lerr = load_ledger(root)
    if lerr:
        return None
    if not entries:
        return LEDGER_NO_RECORD
    if ledger_shape_errors(entries)[0]:     # 有记录但连号被破 ⇒ 走 rc=2，不归这两态
        return None
    return None


def ledger_shape_errors(entries):
    """台账的成对自证 ⇒ (hard, soft)。

    hard = **append-only 的硬证据**：`seq` 必须连号 = 行号（1..N）—— 删行/改写 ⇒ 断号 ⇒ 不给结论。
    soft = **链的提示**（值链 / 件链）：它们**会因为正当理由断**（例：`gate-coverage.config.json`
      被别的批改了 `scan`/`whitelist` 那些键 —— 值没动、件的 sha 却变了），所以**只提示、不改判**；
      「值被改过没有」这件事由 `verdict_of` 的「现值 ≠ 台账最后一条」那条红来判（那才有牙）。
    """
    hard, soft = [], []
    for i, e in enumerate(entries):
        if e.get("seq") != i + 1:
            hard.append("第 %d 条的 seq=%r（连号要求 = %d ⇒ 有删行/改写 ⇒ append-only 被破）"
                        % (i + 1, e.get("seq"), i + 1))
    last_new, last_sha = {}, {}
    for e in entries:
        name, f = e.get("name"), e.get("file")
        if name in last_new and e.get("old") != last_new[name]:
            soft.append("`%s` 的值链断了：上一条 new=%r · 本条 old=%r" % (name, last_new[name], e.get("old")))
        if f in last_sha and e.get("sha256_before") != last_sha.get(f):
            soft.append("件 `%s` 的件链断了：上一条 sha256_after=%r · 本条 sha256_before=%r（件被别的批动过）"
                        % (f, last_sha.get(f), e.get("sha256_before")))
        last_new[name] = e.get("new")
        last_sha[f] = e.get("sha256_after")
    return hard, soft


def append_ledger(root, entry):
    """append-only 追一条。⇒ 错因|None。"""
    line = json.dumps(entry, ensure_ascii=False)
    if "\n" in line:
        return "台账行里不许有换行"
    path = ledger_path(root)
    d = os.path.dirname(path)
    if d and not os.path.isdir(d):
        return "台账目录不在：%s" % d
    with open(path, "a", encoding="utf-8") as f:
        f.write(line + "\n")
        f.flush()
        os.fsync(f.fileno())
    return None


# ── 判 ───────────────────────────────────────────────────────────────────────
def verdict_of(cond, cur_val, cur_sha, entries):
    """⇒ (档, 判词)。档 ∈ {`ok`, `warn`, `red`}。"""
    es = [e for e in entries if e.get("name") == cond["name"]]
    if not es:
        return "red", "无记录：件里这个棘轮数字**一条台账记录都没有** ⇒ 判红（`R-03`）"
    last = es[-1]
    if last.get("new") != cur_val:
        return "red", ("现值 %s ≠ 台账最后一条 new %s ⇒ 有一条**没记录的手改** ⇒ 判红（改动只许走 `--write`）"
                       % (cur_val, last.get("new")))
    if last.get("sha256_after") and cur_sha and last["sha256_after"] != cur_sha:
        return "warn", ("值有记录且对得上，但**件另有改动**（键值没动 ⇒ 不改判据宽度，只提示）："
                        "台账 %s… · 现件 %s…" % (str(last["sha256_after"])[:12], cur_sha[:12]))
    return "ok", "台账最后一条：%s → %s（%s · %s）" % (last.get("old"), last.get("new"),
                                                    last.get("ts"), last.get("kind"))


def verdict_rc(rows, hard):
    """判据面**唯一那处** rc 规则（谁要看「无记录 ⇒ 红」都走这里，不许各写一份）：
    hard（台账解不动 / seq 断号）⇒ 2；有红（无记录 · 现值 ≠ 台账）⇒ 1；否则 0。"""
    if hard:
        return 2
    return 1 if any(r["verdict"] == "red" for r in rows) else 0


def collect(root, conds, readings=None, only=None, with_measure=False):
    """逐条件收现值/现测值/判。⇒ (rows, 硬错列表, 提示列表)。"""
    entries, lerr = load_ledger(root)
    hard, soft = [], []
    if lerr:
        hard.append("台账解不动：%s" % lerr)
    else:
        h, s = ledger_shape_errors(entries)
        hard.extend(h)
        soft.extend(s)
    rows = []
    for cond in conds:
        if only and cond["name"] not in only:
            continue
        cur, sha, verr = read_value(root, cond)
        mv, msrc, merr = (None, None, None)
        if with_measure:
            mv, msrc, merr = measure_value(root, cond, readings)
        if verr:
            vd, vtxt = "n/a", "判不了：%s" % verr
        else:
            vd, vtxt = verdict_of(cond, cur, sha, entries)
        rows.append({"cond": cond, "cur": cur, "sha": sha, "err": verr,
                     "meas": mv, "meas_src": msrc, "meas_err": merr,
                     "verdict": vd, "verdict_text": vtxt})
    return rows, hard, soft


def direction(cur, meas):
    """⇒ `将升` / `将降` / `不变`。"""
    if meas is None or cur is None:
        return "—"
    if meas > cur:
        return "将升"
    if meas < cur:
        return "将降"
    return "不变"


# ── 写面（唯一通道）──────────────────────────────────────────────────────────
def rewrite_key(root, cond, new_val):
    """只改那一个数字（逐字节保留其余）⇒ (新件 sha256|None, 错因|None)。"""
    path = os.path.join(root, cond["file"])
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as exc:
        return None, "件读不到：%s" % exc
    pat = re.compile(r'("%s"\s*:\s*)(\d+)' % re.escape(cond["key"]))
    if len(pat.findall(raw)) != 1:
        return None, "件里 `%s` 命中 %d 次（要**恰好 1 次**，多了少了都不许改）" % (cond["key"], len(pat.findall(raw)))
    new_raw = pat.sub(lambda m: m.group(1) + str(new_val), raw, count=1)
    if new_raw == raw:
        return None, "改写后与原文逐字相同（没有可改的东西）"
    try:
        with open(path, "w", encoding="utf-8") as f:
            f.write(new_raw)
    except OSError as exc:
        return None, "件写不进去：%s" % exc
    return sha256_file(path), None


def who_am_i(root):
    """记录「谁」（照人签件的口径：git 身份优先）。"""
    for key in ("user.name", "user.email"):
        try:
            subprocess.run(["git", "config", key], cwd=root, capture_output=True, text=True, timeout=10)
        except (OSError, subprocess.SubprocessError):
            pass
    name = email = ""
    for key, tgt in (("user.name", "name"), ("user.email", "email")):
        try:
            pr = subprocess.run(["git", "config", key], cwd=root, capture_output=True, text=True, timeout=10)
            val = (pr.stdout or "").strip()
        except (OSError, subprocess.SubprocessError):
            val = ""
        if tgt == "name":
            name = val
        else:
            email = val
    if name and email:
        return "%s <%s>" % (name, email)
    return name or email or "（未登记 · 本机 git 身份读不到）"


# ── 打印 ─────────────────────────────────────────────────────────────────────
def print_rows(rows):
    w = max([len(r["cond"]["name"]) for r in rows] + [10])
    for r in rows:
        c = r["cond"]
        cur = "—" if r["cur"] is None else str(r["cur"])
        print("   %-*s  %-4s  %-46s 现值 %-5s  %s" % (w, c["name"], c["gate"], c["file"], cur, r["verdict_text"]))
        if r["meas"] is not None:
            print("   %-*s        现测 %-5s（%s）⇒ %s" % (w, "", r["meas"], r["meas_src"], direction(r["cur"], r["meas"])))


def print_diff(rows, hard, soft=()):
    print("── baseline 差值（只读 · **不写任何件**）──")
    up = down = same = 0
    unmeasured = []
    for r in rows:
        c = r["cond"]
        if r["meas_err"]:
            unmeasured.append((c["name"], r["meas_err"]))
            print("   %-32s 现值 %-5s 现测 **判不了** ⇒ 不给结论" % (c["name"], r["cur"]))
            continue
        d = direction(r["cur"], r["meas"])
        delta = (r["meas"] - r["cur"])
        if d == "将升":
            up += 1
        elif d == "将降":
            down += 1
        else:
            same += 1
        print("   %-32s %-5s 现值 %-5s → 现测 %-5s  %s（%+d）· %s · %s"
              % (c["name"], c["gate"], r["cur"], r["meas"], d, delta, c["unit"], r["meas_src"]))
    print("   ⇒ **将升 %d 条 · 将降 %d 条 · 不变 %d 条**（逐条件名见上）" % (up, down, same))
    if up:
        print("   ★ 将升 = **放宽判据**：`R-03` 要求可升，但**必须留痕** ⇒ `--write --reason '…' --allow-raise`")
    if down:
        print("   ★ 将降 = 顺棘轮（只许下调）：`--write --reason '…'`")
    for name, err in unmeasured:
        print("   ⛔ `%s` 判不了：%s" % (name, err))
    for e in hard:
        print("   ⛔ 台账有硬错：%s" % e)
    for e in soft:
        print("   ⚠ 台账提示（不改判）：%s" % e)


def print_view(root, rows, hard=(), soft=()):
    print("── baseline 只读视图（唯一真源 = 本件；不写任何件）──")
    entries, _ = load_ledger(root)
    print("   台账：%s（%d 条记录 · %s）"
          % (LEDGER_REL, len(entries), "件在" if os.path.isfile(ledger_path(root)) else "**件还不存在**"))
    print_rows(rows)
    for e in hard:
        print("   ⛔ 台账硬错：%s" % e)
    for e in soft:
        print("   ⚠ 台账提示（不改判）：%s" % e)


def write_json(payload):
    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=False))


# ── 各档 ─────────────────────────────────────────────────────────────────────
def do_status(root, args, conds, mode):
    only = args.only
    if mode == "diff":
        readings = None
        if args.readings:
            readings, rerr = read_readings(args.readings, conds)
            if rerr:
                print("⛔ 读数件解不动：%s ⇒ 不给结论（rc=2）" % rerr)
                return 2
        rows, hard, soft = collect(root, conds, readings=readings, only=only, with_measure=True)
        if not rows:
            print("⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）")
            return 2
        if args.json:
            write_json({"mode": "diff", "rows": [
                {k: r[k] for k in ("cur", "sha", "meas", "meas_src", "meas_err")} | {"name": r["cond"]["name"],
                                                                                    "gate": r["cond"]["gate"],
                                                                                    "file": r["cond"]["file"],
                                                                                    "direction": direction(r["cur"], r["meas"])}
                for r in rows], "ledger_hard_errors": hard, "ledger_soft_notes": soft})
            return 2 if any(r["meas_err"] for r in rows) else 0
        print_diff(rows, hard, soft)
        if any(r["meas_err"] for r in rows):
            print("⇒ **不给结论（rc=2）**：有条件的现测值取不到（现跑命令失败/解析不出判据行）")
            return 2
        print("⇒ 只报差跑成（rc=0）：差是事实，红是「没记录」——要留痕见 `--write`")
        return 0

    rows, hard, soft = collect(root, conds, only=only, with_measure=False)
    if not rows:
        print("⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）")
        return 2
    reds = [r for r in rows if r["verdict"] == "red"]
    reason = ledger_reason(root)                  # `Q-144`：红里的两态判因（只加字段，不动 rc ✗）
    if args.json:
        write_json({"mode": "view", "ledger": LEDGER_REL, "reason": reason,
                    "rows": [{"name": r["cond"]["name"], "gate": r["cond"]["gate"], "file": r["cond"]["file"],
                              "value": r["cur"], "sha256": r["sha"], "err": r["err"],
                              "verdict": r["verdict"], "verdict_text": r["verdict_text"]} for r in rows],
                    "ledger_hard_errors": hard, "ledger_soft_notes": soft})
    else:
        print_view(root, rows, hard, soft)
    if hard:
        print("⇒ **不给结论（rc=2）**：台账本身解不动/形状坏了")
        return 2
    rc = verdict_rc(rows, hard)
    if rc == 1:
        print("⇒ **判红（rc=1）**：%d 个条件无记录或与台账不符（逐条见上）" % len(reds))
        if reason:
            print("   ⛔ 错因（机器可判 · `Q-144`）：reason=%s —— %s" % (reason, LEDGER_REASON_TEXT[reason]))
        print("   闭洞的唯一通道：`--write --reason '…'`（改数 + 留痕）或 `--anchor --reason '…'`（补现状锚，不改件）")
        return 1
    print("⇒ 绿（rc=0）：每个棘轮数字都能在台账里找到它自己那条记录，且现值对得上")
    return rc


def do_write(root, args, conds):
    if not args.reason or not args.reason.strip():
        print("⛔ `--write` 必须带 `--reason '…'`（`R-03` 逐字：**必须留「谁在何时为什么」**）"
              " ⇒ fail-closed，**不写**（rc=2）")
        return 2
    readings = None
    if args.readings:
        readings, rerr = read_readings(args.readings, conds)
        if rerr:
            print("⛔ 读数件解不动：%s ⇒ 不给结论（rc=2）" % rerr)
            return 2
    rows, hard, soft = collect(root, conds, readings=readings, only=args.only, with_measure=True)
    if hard:
        print("⛔ 台账有硬错（append-only 的连号证据不过）⇒ 先把台账修对，不给结论（rc=2）：")
        for e in hard:
            print("   · %s" % e)
        return 2
    for e in soft:
        print("   ⚠ 台账提示（不改判）：%s" % e)
    if not rows:
        print("⛔ 一个条件都没选中 ⇒ 不给结论（rc=2）")
        return 2
    blocked = [r for r in rows if r["meas_err"] or r["err"]]
    if blocked:
        for r in blocked:
            print("⛔ `%s` 取不到数：%s" % (r["cond"]["name"], r["meas_err"] or r["err"]))
        print("⇒ 有条件的数取不到 ⇒ **一个字节都不写**（不给结论 rc=2）")
        return 2
    raises = [r for r in rows if r["meas"] > r["cur"]]
    if raises and not args.allow_raise:
        for r in raises:
            print("⛔ `%s` 要**上调** %s → %s（= **放宽判据**）：`R-03` 允许升，但必须显式"
                  % (r["cond"]["name"], r["cur"], r["meas"]))
        print("⇒ 上调须带 `--allow-raise`（fail-closed：不加 ⇒ 不写 rc=2）")
        return 2
    by = args.by or who_am_i(root)
    done = []
    for r in rows:
        cond, old, new = r["cond"], r["cur"], r["meas"]
        sha_before = r["sha"]
        if new == old:
            print("   · `%s` 现测 = 现值（%s）⇒ **不改件**；要留痕请用 `--anchor`" % (cond["name"], old))
            continue
        entries, _ = load_ledger(root)
        seq = (max([e.get("seq", 0) for e in entries]) + 1) if entries else 1
        sha_after, werr = rewrite_key(root, cond, new)
        if werr:
            print("⛔ `%s` 改不动：%s ⇒ 不给结论（rc=2）" % (cond["name"], werr))
            return 2
        entry = {"seq": seq, "ts": now_iso(), "kind": "write", "name": cond["name"], "gate": cond["gate"],
                 "file": cond["file"], "old": old, "new": new, "reason": args.reason.strip(), "by": by,
                 "sha256_before": sha_before, "sha256_after": sha_after, "measure": r["meas_src"]}
        lerr = append_ledger(root, entry)
        if lerr:
            print("⛔ 台账追不上（件已改 · 台账没落）：%s ⇒ 判红（rc=1）" % lerr)
            return 1
        done.append(entry)
        print("   ✓ `%s`：%s → %s · 件 `%s` · 台账第 %d 条（%s）"
              % (cond["name"], old, new, cond["file"], seq, by))
        print("     理由：%s" % args.reason.strip())
        print("     sha256：%s… → %s…" % (str(sha_before)[:12], str(sha_after)[:12]))
    if not done:
        print("⇒ 没有任何条件的数变了 ⇒ 没写（rc=0）")
        return 0
    print("⇒ 已写 %d 条（**唯一通道** · 每条都带「件 · 旧值 → 新值 · 理由 · 时间」）" % len(done))
    over = [e for e in done if e["new"] > e["old"]]
    if over:
        print("   ★ 其中**上调** %d 条（放宽判据）—— 台账里留了痕，事后可审" % len(over))
    return 0


def do_anchor(root, args, conds):
    if not args.reason or not args.reason.strip():
        print("⛔ `--anchor` 必须带 `--reason '…'`（锚也要「谁在何时为什么」）⇒ 不写（rc=2）")
        return 2
    rows, hard, soft = collect(root, conds, only=args.only, with_measure=False)
    if hard:
        print("⛔ 台账有硬错 ⇒ 不给结论（rc=2）")
        for e in hard:
            print("   · %s" % e)
        return 2
    for e in soft:
        print("   ⚠ 台账提示（不改判）：%s" % e)
    missing = [r for r in rows if r["verdict"] == "red" and r["verdict_text"].startswith("无记录")]
    for r in rows:
        if r["err"]:
            print("⛔ `%s` 现值取不到：%s ⇒ 不写（rc=2）" % (r["cond"]["name"], r["err"]))
            return 2
    by = args.by or who_am_i(root)
    entries, _ = load_ledger(root)
    seq = (max([e.get("seq", 0) for e in entries]) + 1) if entries else 1
    n = 0
    for r in missing:
        cond = r["cond"]
        entry = {"seq": seq + n, "ts": now_iso(), "kind": "anchor", "name": cond["name"], "gate": cond["gate"],
                 "file": cond["file"], "old": r["cur"], "new": r["cur"], "reason": args.reason.strip(), "by": by,
                 "sha256_before": r["sha"], "sha256_after": r["sha"], "measure": "（锚：只记现状，不改件）"}
        lerr = append_ledger(root, entry)
        if lerr:
            print("⛔ 台账追不上：%s ⇒ 判红（rc=1）" % lerr)
            return 1
        n += 1
        print("   ✓ 锚 `%s` = %s（**件一字未改** · 台账第 %d 条 · %s）" % (cond["name"], r["cur"], entry["seq"], by))
    if n == 0:
        print("⇒ 没有「无记录」的条件 ⇒ 没写（rc=0）")
        return 0
    print("⇒ 已补 %d 条现状锚（append-only · 此后这个数再动就必须走 `--write` 并留痕）" % n)
    return 0


def do_conditions(args):
    payload = {"id": "baseline-conditions.v1",
               "ledger": LEDGER_REL,
               "note": "声明式条件表：本件不记任何数字（现值读件、现测现跑该条件的门脚本）",
               "conditions": [{"name": c["name"], "gate": c["gate"], "file": c["file"], "key": c["key"],
                               "policy": c["policy"], "unit": c["unit"],
                               "measure": " ".join(c["measure"]), "measure_re": c["measure_re"]} for c in CONDITIONS]}
    write_json(payload)
    return 0


# ── 自检（合成件 · 不碰真目标）───────────────────────────────────────────────
FX_CONDS = (
    {"name": "fx.alpha", "gate": "门①", "file": os.path.join("scripts", "gates", "fx-alpha.json"),
     "key": "baseline", "policy": "down", "unit": "合成条件甲",
     "measure": ["python3", os.path.join("scripts", "gates", "fx-measure.py"), "12", "命中 12 处"],
     "measure_re": r"命中\s*(\d+)\s*处"},
    {"name": "fx.beta", "gate": "门⑪", "file": os.path.join("scripts", "gates", "fx-beta.json"),
     "key": "blanks", "policy": "down", "unit": "合成条件乙",
     "measure": ["python3", os.path.join("scripts", "gates", "fx-measure.py"), "0", "空白 0 条"],
     "measure_re": r"空白\s*(\d+)\s*条"},
)


def self_test():
    import shutil
    import tempfile

    fails = []
    tmp = tempfile.mkdtemp(prefix="zerg-baseline-selftest-")

    def case(name, got, want):
        if got == want:
            print("   ✓ %s" % name)
        else:
            print("   ✗ %s（got=%r want=%r）" % (name, got, want))
            fails.append(name)

    def case_true(name, ok):
        case(name, bool(ok), True)

    try:
        root = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(root, "core"))
        os.makedirs(os.path.join(root, "scripts", "gates"))
        open(os.path.join(root, "core", "go.mod"), "w").close()
        open(os.path.join(root, "scripts", "gates", "precommit-gates.sh"), "w").close()
        with open(os.path.join(root, "scripts", "gates", "fx-measure.py"), "w", encoding="utf-8") as f:
            f.write("import sys\nprint(sys.argv[2] if len(sys.argv) > 2 else 'nothing')\n")
        with open(os.path.join(root, "scripts", "gates", "fx-alpha.json"), "w", encoding="utf-8") as f:
            f.write('{\n  "id": "fx-alpha.v1",\n  "baseline": 5,\n  "note": "合成件"\n}\n')
        with open(os.path.join(root, "scripts", "gates", "fx-beta.json"), "w", encoding="utf-8") as f:
            f.write('{\n  "id": "fx-beta.v1",\n  "blanks": 0\n}\n')

        # ① 仓根上溯
        case("① 仓根上溯（从子目录找得到）", find_root(os.path.join(root, "scripts", "gates")), root)

        # ② 现值读得对（逐键）
        v, sha, err = read_value(root, FX_CONDS[0])
        case("② 现值读件（fx.alpha.baseline = 5）", (v, err), (5, None))
        case_true("② 件 sha256 读得到（64 位十六进制）", bool(re.fullmatch(r"[0-9a-f]{64}", sha or "")))

        # ③ 现测现跑（**不是常量**）：合成测量件印 12 ⇒ 解析得 12
        mv, msrc, merr = measure_value(root, FX_CONDS[0])
        case("③ 现测 = 现跑门脚本再解析（12）", (mv, merr), (12, None))
        case_true("③ 来源串点名了现跑的命令", "现跑" in (msrc or ""))

        # ④ 差值方向（将升/将降/不变 三档都有）
        case("④ 方向：现值 5 · 现测 12 ⇒ 将升", direction(5, 12), "将升")
        case("④ 方向：现值 5 · 现测 3 ⇒ 将降", direction(5, 3), "将降")
        case("④ 方向：现值 5 · 现测 5 ⇒ 不变", direction(5, 5), "不变")

        # ⑤ 无记录 ⇒ 红（真红：**今天真仓就是这个状态**）
        entries, _ = load_ledger(root)
        vd, vt = verdict_of(FX_CONDS[1], 0, sha256_file(os.path.join(root, FX_CONDS[1]["file"])), entries)
        case("⑤ 无记录 ⇒ 判红（rc 那档）", vd, "red")
        case_true("⑤ 判词里点明「无记录」", "无记录" in vt)

        # ⑥ 锚之后 ⇒ 绿；再手改值 ⇒ 又红（成对：红红在台账那格，不是红在夹具）
        r0 = {"cond": FX_CONDS[0], "cur": 5, "sha": sha256_file(os.path.join(root, FX_CONDS[0]["file"])),
              "err": None, "meas": None, "meas_src": None, "meas_err": None}
        rc = do_anchor(root, argparse.Namespace(reason="自检：起步现状锚", only=None, by="selftest"), FX_CONDS)
        case("⑥ --anchor 缺条件也无误 ⇒ rc=0", rc, 0)
        entries, lerr = load_ledger(root)
        case("⑥ 锚记了 2 条（两个条件各一条）", len(entries), 2)
        case("⑥ 台账件真落了盘", os.path.isfile(ledger_path(root)), True)
        case("⑥ 无形状硬错（连号）", ledger_shape_errors(entries)[0], [])
        case("⑥ 无形状提示（值链/件链）", ledger_shape_errors(entries)[1], [])
        vd, _ = verdict_of(FX_CONDS[0], 5, r0["sha"], entries)
        case("⑥ 锚之后 ⇒ 绿", vd, "ok")

        # ⑦ **手改一个字节、没记录** ⇒ 判红（这是本件最要紧的那一格）
        p = os.path.join(root, FX_CONDS[0]["file"])
        with open(p, encoding="utf-8") as f:
            raw = f.read()
        with open(p, "w", encoding="utf-8") as f:
            f.write(raw.replace('"baseline": 5', '"baseline": 9'))
        v2, sha2, _ = read_value(root, FX_CONDS[0])
        vd, vt = verdict_of(FX_CONDS[0], v2, sha2, entries)
        case("⑦ 手改值（5→9）无记录 ⇒ 判红", vd, "red")
        case_true("⑦ 判词点明「没记录的手改」", "没记录的手改" in vt)

        # ⑧ 件被删 ⇒ 判不了（不是绿）
        os.remove(p)
        _, _, verr = read_value(root, FX_CONDS[0])
        case_true("⑧ 件不在 ⇒ 有错因（判不了 ≠ 绿）", bool(verr))
        with open(p, "w", encoding="utf-8") as f:
            f.write('{\n  "baseline": 5\n}\n')

        # ⑨ 写面：缺 `--reason` ⇒ rc=2 且**件一字不动**（sha 成对）
        p = os.path.join(root, FX_CONDS[0]["file"])
        sh_before = sha256_file(p)
        rc = do_write(root, argparse.Namespace(reason="   ", only=["fx.alpha"], readings=None, allow_raise=False,
                                               by="selftest"), FX_CONDS)
        case("⑨ --write 缺理由 ⇒ rc=2（fail-closed）", rc, 2)
        case("⑨ 缺理由时件 sha256 一字不变", sha256_file(p), sh_before)

        # ⑩ 写面：**上调** 5 → 12 不带 `--allow-raise` ⇒ rc=2 且不写
        rc = do_write(root, argparse.Namespace(reason="自检：试图上调", only=["fx.alpha"], readings=None,
                                               allow_raise=False, by="selftest"), FX_CONDS)
        case("⑩ 上调不带 --allow-raise ⇒ rc=2", rc, 2)
        case("⑩ 上调被拦时件 sha256 一字不变", sha256_file(p), sh_before)

        # ⑪ 写面：显式允许上调 ⇒ 件真变 + 台账真追一条（旧值→新值 · 理由 · 时间 · 谁）
        before_n = len(load_ledger(root)[0])
        rc = do_write(root, argparse.Namespace(reason="自检：上调留痕", only=["fx.alpha"], readings=None,
                                               allow_raise=True, by="selftest"), FX_CONDS)
        case("⑪ 显式允许上调 ⇒ rc=0", rc, 0)
        v3, sha3, _ = read_value(root, FX_CONDS[0])
        case("⑪ 件里的数真变了（5 → 12）", v3, 12)
        entries, _ = load_ledger(root)
        case("⑪ 台账真追了一条（+1）", len(entries), before_n + 1)
        last = entries[-1]
        case("⑪ 那条四格齐（件/旧/新/理由+时间）",
             (last.get("file"), last.get("old"), last.get("new"), bool(last.get("reason")), bool(last.get("ts")),
              last.get("by"), last.get("kind")),
             (FX_CONDS[0]["file"], 5, 12, True, True, "selftest", "write"))
        case("⑪ 写后形状仍无硬错", ledger_shape_errors(entries)[0], [])
        case_true("⑪ 件链断被点成**提示**（件被别的批动过 ≠ 判红）",
                  any("件链断了" in s for s in ledger_shape_errors(entries)[1]))
        case("⑪ 写后 ⇒ 绿（现值 = 台账最后一条）", verdict_of(FX_CONDS[0], v3, sha3, entries)[0], "ok")

        # ⑫ 只读性：**只读那两档**跑一遍（--diff / --view 的函数级跑法）前后件 sha 不变
        sha_pre = sha256_file(p)
        rows, hard, soft = collect(root, FX_CONDS, with_measure=True, only=["fx.alpha"])
        print_diff(rows, hard, soft)
        print_view(root, rows, hard, soft)
        case("⑫ 只读两档跑完：件 sha256 一字不变", sha256_file(p), sha_pre)

        # ⑬ 台账断号（删中间一条）⇒ 形状**硬错** ⇒ 不给结论（append-only 的证据）
        lp = ledger_path(root)
        with open(lp, encoding="utf-8") as f:
            lines = f.read().splitlines()
        with open(lp, "w", encoding="utf-8") as f:
            f.write("\n".join([lines[0]] + lines[2:]) + "\n")
        entries, _ = load_ledger(root)
        case_true("⑬ 台账被删一条 ⇒ seq 断号被点成**硬错**",
                  any("连号" in e for e in ledger_shape_errors(entries)[0]))
        case("⑬ 硬错时 --write 不给结论 ⇒ 一个字节不写（rc=2）",
             do_write(root, argparse.Namespace(reason="自检：台账坏了", only=["fx.alpha"], readings=None,
                                               allow_raise=True, by="selftest"), FX_CONDS), 2)

        # ⑭ 读数注入（缺件/不认名的读数件 ⇒ 判不了）
        rp = os.path.join(tmp, "bad-readings.tsv")
        with open(rp, "w", encoding="utf-8") as f:
            f.write("fx.alpha = 3\n没人认识的条件 = 1\n")
        got, rerr = read_readings(rp, FX_CONDS)
        case_true("⑭ 读数件里有不认的条件名 ⇒ 判不了", bool(rerr))
        rp2 = os.path.join(tmp, "good-readings.tsv")
        with open(rp2, "w", encoding="utf-8") as f:
            f.write("# 复核用读数\nfx.alpha = 3\nfx.beta = 0\n")
        got, rerr = read_readings(rp2, FX_CONDS)
        case("⑭ 好读数件解得出两格", (got, rerr), ({"fx.alpha": 3, "fx.beta": 0}, None))

        # ⑮ 只改那一个数字（逐字节保留其余）：合成件的 note 行必须一字不动
        os.remove(lp)
        with open(p, "w", encoding="utf-8") as f:
            f.write('{\n  "id": "fx-alpha.v1",\n  "baseline": 5,\n  "note": "合成件"\n}\n')
        rp3 = os.path.join(tmp, "down.tsv")
        with open(rp3, "w", encoding="utf-8") as f:
            f.write("fx.alpha = 3\n")
        rc = do_write(root, argparse.Namespace(reason="自检：下调留痕", only=["fx.alpha"], readings=rp3,
                                               allow_raise=False, by="selftest"), FX_CONDS)
        case("⑮ 现值 5 · 现测 3 ⇒ 下调 rc=0", rc, 0)
        with open(p, encoding="utf-8") as f:
            after = f.read()
        case("⑮ 只改了那一个数字", after, '{\n  "id": "fx-alpha.v1",\n  "baseline": 3,\n  "note": "合成件"\n}\n')

        # ⑯ 台账两态**可判**（`Q-144`：今天两态同 rc=1 ⇒ 只加一枚判因，**不动退码** ✗）
        #    甲态 = 件不在盘上 · 乙态 = 件在盘上但 0 条记录 —— 两态 rc 必须仍同为 1，
        #    而 `reason` 必须分得开（机器面 `--json` 档一个键就读出来）。
        import contextlib
        import io as _io

        def _view_payload():
            buf = _io.StringIO()
            with contextlib.redirect_stdout(buf):
                rc = do_status(root, argparse.Namespace(only=None, readings=None, json=True),
                               FX_CONDS, "view")
            return rc, json.loads(buf.getvalue().split("\n⇒")[0])

        os.remove(lp)                                            # 甲态：台账件不在盘上
        rows_a, hard_a, _sa = collect(root, FX_CONDS, with_measure=False)
        reason_a = ledger_reason(root)
        rc_a, pay_a = _view_payload()
        case("⑯ 件不在盘上 ⇒ reason=ledger_absent", reason_a, "ledger_absent")
        case("⑯ 件不在盘上 ⇒ 仍 rc=1（同码不动）", (verdict_rc(rows_a, hard_a), rc_a), (1, 1))
        case("⑯ 包封 `--json` 档带同一枚判因", pay_a.get("reason"), "ledger_absent")
        case_true("⑯ 包封只多这一枚键（旧键一字不改）",
                  set(pay_a) - {"mode", "ledger", "rows", "ledger_hard_errors", "ledger_soft_notes"} == {"reason"})
        with open(lp, "w", encoding="utf-8"):                    # 乙态：件在盘上、一条记录都没有
            pass
        rows_b, hard_b, _sb = collect(root, FX_CONDS, with_measure=False)
        reason_b = ledger_reason(root)
        rc_b, pay_b = _view_payload()
        case("⑯ 件在但 0 条记录 ⇒ reason=no_record", reason_b, "no_record")
        case("⑯ 件在但 0 条记录 ⇒ 仍 rc=1（同码不动）", (verdict_rc(rows_b, hard_b), rc_b), (1, 1))
        case_true("⑯ 两态同 rc=1、判因却分得开（机器面从此能问是哪一种）",
                  reason_a != reason_b and pay_a.get("reason") != pay_b.get("reason"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("自检结论: %s（失败 %d 条）" % ("全过" if not fails else "**有失败**", len(fails)))
    return 0 if not fails else 2


# ── main ─────────────────────────────────────────────────────────────────────
def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--view", action="store_true", help="只读视图（默认档）：现值 + 台账 + 判")
    ap.add_argument("--diff", action="store_true", help="**只报差**：将升/将降几条 + 逐条件名（现跑测量）")
    ap.add_argument("--check", action="store_true", help="同 --view（无记录 ⇒ 判红 rc=1；本档是判据那档）")
    ap.add_argument("--conditions", action="store_true", help="打印条件表（机器可读 JSON）")
    ap.add_argument("--write", action="store_true", help="**唯一写入口**：改数 + 台账留痕（须 --reason）")
    ap.add_argument("--anchor", action="store_true", help="补「现状锚」：只记台账、**不改任何件**（须 --reason）")
    ap.add_argument("--reason", default="", help="改动理由（写/锚必填；没理由 = 不写 rc=2）")
    ap.add_argument("--by", default="", help="谁（默认读本机 git 身份）")
    ap.add_argument("--allow-raise", action="store_true", help="显式允许**上调**（放宽判据）")
    ap.add_argument("--only", action="append", default=None, metavar="条件名", help="只对指定条件生效（可重复）")
    ap.add_argument("--readings", metavar="件", help="注入现测读数（`条件名=值` 逐行）—— 只读数、不跑门")
    ap.add_argument("--root", default=".", help="仓根（默认从当前目录上溯）")
    ap.add_argument("--json", action="store_true", help="机器可读输出")
    ap.add_argument("--self-test", action="store_true", help="合成件成对负控（不碰真目标）")
    a = ap.parse_args()

    if a.self_test:
        print("── baseline-ratchet 自检（合成件 · 不碰真目标）──")
        return self_test()

    if a.conditions:
        return do_conditions(a)

    chosen = [x for x in (a.view, a.diff, a.check, a.write, a.anchor) if x]
    if len(chosen) > 1:
        print("⛔ 档位互斥（一次只做一件事）：%s ⇒ 用法错（rc=2）" % ",".join(
            ["--" + n for n in ("view", "diff", "check", "write", "anchor") if getattr(a, n)]))
        return 2

    if a.only:
        for n in a.only:
            if cond_by_name(n) is None:
                print("⛔ 条件名不认：%s（列全部：--conditions）⇒ 不给结论（rc=2）" % n)
                return 2

    root = find_root(a.root)
    if root is None:
        print("⛔ 找不到仓根（从 %s 上溯找不到 %s）⇒ 不给结论（rc=2）" % (a.root, " + ".join(ROOT_MARKERS)))
        return 2

    if a.write:
        return do_write(root, a, CONDITIONS)
    if a.anchor:
        return do_anchor(root, a, CONDITIONS)
    if a.diff:
        return do_status(root, a, CONDITIONS, "diff")
    return do_status(root, a, CONDITIONS, "view")


if __name__ == "__main__":
    sys.exit(main())

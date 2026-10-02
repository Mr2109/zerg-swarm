#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-judge-registry.py —— 判据注册表（M1-04）真源生成 ＋ 双向差集自检（M1-05）· fail-closed

本件是「判据 CLI 化」M1 开卷前置①② 的落点（设计稿 `设计-判据 CLI 化-v1.7-20260930.md` 第六节 · 9.2 前置①）：

  ① 注册表真源 = `scripts/gates/judge-registry.json`（**由本件 `--emit` 现读生成** · ✗ 不手抄名录）；
  ② 双向差集 = 注册表 ⇄ 盘上件，**两方向缺一即红**（fail-closed）：
       · 盘上有件、表里没点名  ⇒ 红（新件没登记）
       · 表里点名、盘上无件    ⇒ 红（悬空登记）
     另附三条列面机检：件名 ⇒ 路径 越出 `<仓根>/scripts/` 前缀 ⇒ 红；件名 ≠ 件名去扩展名逐字相同 ⇒ 红；
     逐件 `sha256` 与件现读不符（缓存键过期）⇒ 红。

★ 拆两半（设计稿附A A3 · 本批为门79 B1-6）：本件把「**有哪些判据 / ID / 接线**」与「**某条判据被采纳 /
  被豁免 / 四字段被声明**」**分开存放** ——

  · **派生半（甲）** = 「有哪些判据 / ID / 接线」——**纯派生**：`--emit` 现读 `<仓根>/scripts/` 盘上件即算出
    （`id`/`name`/`path`/`cwd`/`interp`/`sha256`/`tool_version`/`cache_key`/`count`/`dupes`/`entries` …）；
  · **人签半（乙）** = 「某条判据被**采纳** / 被**豁免**」（`decision ∈ {adopt, exempt}`）＋
    「某条判据的**四字段**（`needs_env`/`serial`/`hazard`/`dry_run`）被**声明**」（`decision=declare`）——
    **人为判定**、**必须人签**（SLSA Source L3：「Two party review — changes MUST be agreed to by two or more
    trusted persons prior to submission」）⇒ **单独一处**落 `scripts/gates/judge-adoption.json`（`judge-adoption/1`）；
    ★ `--emit` **一字不碰它**（派生半覆盖人签半 = 把人的决定冲掉，正是本刀要治的病）。
    ★ **第三类决定 `declare`（拍板③ · 与 `adopt`/`exempt` 并列）的载荷** = 四字段逐字段值 ＋ 人签三件套
      `by`/`at`/`sig`（放 `fields` 子对象）。四字段**闭集**：`needs_env` ∈ {true,false} ·
      `serial` ∈ {true,false} · `hazard` ∈ {none,swap,replace,publish,kill}（借 v1.2 `P9` 危险档闭集）·
      `dry_run` ∈ {none,argv,noop}；缺省 `false`/`false`/`none`/`none`。
    ★ **为什么搬进乙半**：这四字段**现读恒 false/none**（196 条无一例外），人若设过会被 `--emit` 冲掉 ——
      与「采纳/豁免」是同一种病 ⇒ 同批把它们从派生半**搬走**（派生半不再出这四列）。
  · `--check` 增**第三方向差集**：派生半在册 ⇄ 人签半决定 ——
    **采纳面**（`adopt|exempt`）在册无决定 ⇒ 红（**受 `enforce` 开关控**，见下）；
    `declare` 载荷四字段缺键 / 值越出闭集 ⇒ 红；决定悬空（点名不在册、盘上也无件）⇒ 红；
    决定值不在闭集 `{adopt,exempt,declare}` ⇒ 红；缺人签三件套 `by`/`at`/`sig` ⇒ 红；
    `exempt` **逐条打印**（可见、非静默）。

  · ★ **迁移期（防堵提交链）**：人签半顶层 **`enforce` 开关（默认 `false`）**——
    `enforce=false` 时，**在册无人签采纳/豁免决定**这一类**不判红**，只在脚注里报「未决定 N 条」；
    **上牙 = 把 `enforce` 改 `true`**，而**这一改动本身必须人签**（⇒ `enforce=true` 时要求顶层 `enforce_sig`
    的 `by`/`at`/`sig` 齐，缺一 ⇒ 红「上牙未人签」）。理由：现存 196 条**无人签决定** ⇒ 若一建档就开牙，
    本门（`tri` 阻断档）**当场变红堵死提交链** ⇒ 迁移期必须能「只报告」。
    ✗ **禁**把「把 196 条在册件自动写成一堆 `adopt`」当迁移手段（那正是「在册=已采纳」那个病）。

名录真源**同源不手抄**：与转发壳 `core/cmd/zerg/family_gate_check.go` 的 `gateCheckRoster` 同口径 ——
递归 `scripts/` 全子树，只收 `.py` / `.sh`（`__pycache__` 不进名录），判据名 = 文件名去扩展名。
⇒ 注册表**不另立第二套真源**（§6 落点承拍定第四条）。

列面（§6 第六节 · 逐条）：`id`（必填）· `name`（= 件名去扩展名逐字）· `path`（`<仓根>/scripts/` 前缀内）·
`cwd`（缺省仓根 `.`）· `interp`（`.py`⇒`python3` / `.sh`⇒`bash`）·
`sha256` ＋ `tool_version`（缓存键 · `P19`）· `cache_key` = `<sha256>|<tool_version>`。
★ **四字段 `needs_env` / `serial` / `hazard` / `dry_run` 不在派生半**（拍板③：搬进人签半作 `declare`）。
schema 名／版本／`min_tool_version` 写进件头（§9.2 前置①）。

退码（三档 · fail-closed）：
  0 = 对拍通过 / `--emit` 写成 / `--self-test` 全过
  1 = 双向差集非空 / **人签半差集非空或决定不合法** / 违反列面 / 缓存键过期（**红**）
  2 = 用法错（三选一都没给）/ 注册表读不到 / 注册表 schema 不符（加载期硬错 · `P20`）/
      **人签半读不到或 schema 不符** / 名录读不到 / `--self-test` 不过

用法：
    python3 scripts/gates/check-judge-registry.py --emit [--out <件>]                       # 现读生成注册表（甲半）
    python3 scripts/gates/check-judge-registry.py --check [--registry <件>] [--decisions <件>]  # 甲半双向差集 ＋ 甲⇄乙对拍
    python3 scripts/gates/check-judge-registry.py --self-test                               # 成对负控自检
"""
import argparse
import hashlib
import json
import os
import platform
import subprocess
import sys
import tempfile

SCHEMA = "judge-registry/1"
DECISIONS_SCHEMA = "judge-adoption/1"
# ★ 决定值闭集（拍板③：第三类 `declare` 与 `adopt`/`exempt` **并列**）
DECISION_VALUES = ("adopt", "exempt", "declare")
# **采纳面** = 只有这两个值算「采纳 / 豁免决定」；`declare` 是**正交**的四字段声明面，**不算**采纳
#   （「声明 hazard」≠「采纳某条判据」⇒ 二者不互相顶替 · 第三方向差集两轴分判）。
ADOPTION_VALUES = ("adopt", "exempt")
# ★ `declare` 载荷四字段（拍板③：搬进人签半）＋ 各自闭集
DECLARE_FIELDS = ("needs_env", "serial", "hazard", "dry_run")
DECLARE_BOOL_FIELDS = ("needs_env", "serial")
HAZARD_VALUES = ("none", "swap", "replace", "publish", "kill")   # 借 v1.2 `P9` 危险档闭集
DRY_RUN_VALUES = ("none", "argv", "noop")
FIELD_DEFAULTS = {"needs_env": False, "serial": False, "hazard": "none", "dry_run": "none"}
# ★ 迁移期开关（拍板③）：默认 **假** —— 未决定不判红；上牙（改 true）**本身必须人签**（顶层 `enforce_sig`）。
ENFORCE_DEFAULT = False
MIN_TOOL_VERSION = {"bash": "3.2", "python": "3.9"}
ROSTER_DIR = "scripts"
INTERP = {".py": "python3", ".sh": "bash"}
REGISTRY_DEFAULT = "scripts/gates/judge-registry.json"
# ★ 人签半（乙）落点：单独一处。`--emit` ✗ 不碰它；`--check` 只读。★ 迁移期件不在盘 ⇒ 乙半面只报告。
DECISIONS_DEFAULT = "scripts/gates/judge-adoption.json"

# 仓根 = 本件（scripts/gates/xxx.py）上溯三层。
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

_BASH_VER = None


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def tool_version(interp):
    """现读解释器版本（✗ 不猜 · 取不到就如实记 `unknown`）。"""
    global _BASH_VER
    if interp == "python3":
        return "python3/" + platform.python_version()
    if interp == "bash":
        if _BASH_VER is None:
            try:
                out = subprocess.run(["bash", "--version"], capture_output=True, text=True, timeout=10)
                first = (out.stdout or "").splitlines()[0] if out.stdout else ""
                # 形如 `GNU bash，版本 3.2.57(1)-release (…)` / `GNU bash, version 3.2.57(...)`
                tok = ""
                for part in first.replace("，", " ").split():
                    if part[:1].isdigit():
                        tok = part.split("(")[0]
                        break
                _BASH_VER = tok or "unknown"
            except Exception:
                _BASH_VER = "unknown"
        return "bash/" + _BASH_VER
    return interp + "/unknown"


def scan_roster(root):
    """现读名录：递归 `scripts/` 全子树，只收 `.py` / `.sh`（`__pycache__` 不进）⇒ {名: [仓相对路径…]}。

    与转发壳 `gateCheckRoster` **同口径**（判据名 = 文件名去扩展名 · 逐字相同）。
    """
    base = os.path.join(root, ROSTER_DIR)
    paths = {}
    for dp, dns, fns in os.walk(base):
        dns[:] = sorted(d for d in dns if d != "__pycache__")
        for fn in sorted(fns):
            ext = os.path.splitext(fn)[1]
            if ext not in INTERP:
                continue
            name = fn[: -len(ext)]
            rel = os.path.relpath(os.path.join(dp, fn), root).replace(os.sep, "/")
            paths.setdefault(name, []).append(rel)
    for v in paths.values():
        v.sort()
    return paths


def build_registry(root):
    """现读生成注册表（**派生半（甲）** —— 确定性：条目按 name 排序 · 键序固定）。

    ★ 本函数**只吃 `root`（盘上件）**、**不吃人签半**：签名里没有 `decisions` 入参 ⇒ 构造上不可能
      覆盖「某条判据被采纳 / 被豁免 / 四字段被声明」这些人为判定（`--self-test` 格A/格H 有牙）。
    ★ 拍板③：四字段 `needs_env`/`serial`/`hazard`/`dry_run` **不在此出**（搬进人签半作 `declare`）——
      派生半只留「现读可算」的列面；人述列面一律在乙半，避免 `--emit` 冲掉人的设定。
    """
    paths = scan_roster(root)
    dupes = sorted(n for n, v in paths.items() if len(v) > 1)
    entries = []
    for name in sorted(paths):
        if len(paths[name]) != 1:
            continue
        rel = paths[name][0]
        ext = os.path.splitext(rel)[1]
        interp = INTERP[ext]
        tv = tool_version(interp)
        sha = sha256_file(os.path.join(root, rel))
        entries.append({
            "id": name,
            "name": name,
            "path": rel,
            "cwd": ".",
            "interp": interp,
            "sha256": sha,
            "tool_version": tv,
            "cache_key": sha + "|" + tv,
        })
    return {
        "schema": SCHEMA,
        "generated_by": "scripts/gates/check-judge-registry.py --emit",
        "roster_dir": ROSTER_DIR,
        "extensions": sorted(INTERP.keys()),
        "min_tool_version": MIN_TOOL_VERSION,
        "invocation": {
            "argv": "passthrough",
            "expected_rc": "passthrough",
            "note": "入口参数与期望退码由转发壳逐字透传（`zerg gate check <判据名> [其余逐字给子件]`）；"
                    "0/1/2 三码透传，件自身崩/超时 ⇒ 壳 8 · judge_broken/timeout。",
        },
        "count": len(entries),
        "dupes": dupes,
        "entries": entries,
    }


def load_decisions(path):
    """读**人签半（乙）**（`judge-adoption.json`）⇒ dict；件不在盘 ⇒ `None`（**不是错** —— 迁移期）。

    ★ 本件对乙半**只读**：`--emit` 一字不碰它。
    ★ schema 不符 / 解析失败 ⇒ 抛 `ValueError`（加载期硬错 ⇒ `--check` rc=2 不给结论）。
    """
    if not os.path.isfile(path):
        return None
    with open(path, encoding="utf-8") as f:
        dec = json.load(f)
    if not isinstance(dec, dict) or dec.get("schema") != DECISIONS_SCHEMA:
        raise ValueError("人签半 schema 不符（须 `%s` · 现读 %r）"
                         % (DECISIONS_SCHEMA,
                            (dec or {}).get("schema") if isinstance(dec, dict) else dec))
    return dec


def _declare_field_problem(did, key, val):
    """`declare` 单字段机检 ⇒ 问题串 or `None`。

    闭集：`needs_env`/`serial` 布尔两值 · `hazard` 五档 · `dry_run` 三档。
    """
    if key in DECLARE_BOOL_FIELDS:
        if not isinstance(val, bool):
            return "declare 载荷 `%s` 非布尔（闭集 true/false · 现读 %r）：%s" % (key, val, did)
        return None
    closed = HAZARD_VALUES if key == "hazard" else DRY_RUN_VALUES
    if val not in closed:
        return "declare 载荷 `%s` 值不在闭集 %s：%s（现读 %r）" % (key, list(closed), did, val)
    return None


def _adoption_undecided(reg, decisions):
    """在册（派生半）中**没有采纳/豁免决定**（`adopt|exempt`）的判据名 · `sorted`。

    `decisions is None`（人签半未建）⇒ 全部在册件都算「未决定」。
    """
    names = sorted(e["name"] for e in reg.get("entries", [])
                   if isinstance(e, dict) and "name" in e)
    if decisions is None:
        return names
    adopted = {d.get("id") for d in decisions.get("decisions", [])
               if isinstance(d, dict) and d.get("decision") in ADOPTION_VALUES}
    return [n for n in names if n not in adopted]


def _decision_notes(decisions, reg=None):
    """乙半**可见面**（✗ 不判红）：`enforce` 状态 ＋ 逐条「采纳 / 豁免 / 声明」＋ 未决定计数。

    ★ 输出行序的决定者：`enforce` 状态行在最前；逐条备注按 `(id, decision 字面)` 排序；
      未决定行在最末，名字按 `sorted`。⇒ 零不确定序。
    """
    if decisions is None:
        n = len(_adoption_undecided(reg, None)) if reg is not None else 0
        return ["人签半（`%s`）未建 ⇒ **迁移期**：乙半面只报告不判红（未决定 %d 条）"
                % (DECISIONS_DEFAULT, n)]
    enforce = decisions.get("enforce", ENFORCE_DEFAULT)
    notes = ["人签半 `enforce`=%s ⇒ %s" % (
        enforce,
        "**上牙**：在册无采纳/豁免决定 ⇒ 判红" if enforce is True
        else "迁移期：在册无采纳/豁免决定**不判红**，只报计数")]
    for d in sorted((x for x in decisions.get("decisions", [])
                     if isinstance(x, dict) and "id" in x),
                    key=lambda x: (x["id"], str(x.get("decision")))):
        v = d.get("decision")
        if v == "declare":
            f = d.get("fields") if isinstance(d.get("fields"), dict) else {}
            tag = "声明（四字段）"
            tail = " · ".join("%s=%r" % (k, f.get(k)) for k in DECLARE_FIELDS)
        elif v == "exempt":
            tag = "豁免"
            tail = str(d.get("reason")) if d.get("reason") else ""
        elif v == "adopt":
            tag = "采纳"
            tail = ""
        else:
            tag = "决定值异常(%r)" % (v,)
            tail = ""
        notes.append("%s：%s（by %s · %s）%s" % (tag, d["id"], d.get("by", "?"), d.get("at", "?"), tail))
    if reg is not None:
        und = _adoption_undecided(reg, decisions)
        if und:
            notes.append("未决定 %d 条（在册无采纳/豁免决定 · %s）：%s"
                         % (len(und), "上牙 ⇒ 判红" if enforce is True else "迁移期只报",
                            " · ".join(und)))
    return notes


def _problem_list(root, reg, decisions=None):
    """双向差集 ＋ 列面机检 ＋ **甲⇄乙 第三方向** ⇒ 问题清单（空 = 绿）。

    `decisions is None`（人签半未建）⇒ 乙半面**一条不判**（迁移期；可见面在 `_decision_notes`）。
    """
    problems = []
    live = scan_roster(root)
    live_dupes = sorted(n for n, v in live.items() if len(v) > 1)
    for n in live_dupes:
        problems.append("盘上同名多件（名字 ⇒ 件 不唯一）：%s（%s）" % (n, " · ".join(live[n])))

    reg_entries = {}
    for e in reg.get("entries", []):
        if not isinstance(e, dict) or "name" not in e:
            problems.append("注册表条目缺 `name`：%r" % (e,))
            continue
        if e["name"] in reg_entries:
            problems.append("注册表内同名重复条目：%s" % e["name"])
        reg_entries[e["name"]] = e

    # 反方向：盘上有件、表里没点名
    for n in sorted(live):
        if n not in reg_entries:
            problems.append("盘上有件、注册表**没点名**（新件未登记）：%s（%s）"
                            % (n, " · ".join(live[n])))
    # 正方向：表里点名、盘上无件
    for n in sorted(reg_entries):
        if n not in live:
            problems.append("注册表**点名**、盘上无件（悬空登记）：%s（%s）"
                            % (n, reg_entries[n].get("path", "?")))

    # 列面机检（只对两侧都在的条目）
    for n in sorted(set(live) & set(reg_entries)):
        e = reg_entries[n]
        rel = e.get("path", "")
        if not isinstance(rel, str) or not rel.startswith(ROSTER_DIR + "/"):
            problems.append("件名 ⇒ 路径越出 `<仓根>/%s/` 前缀：%s（%s）" % (ROSTER_DIR, n, rel))
            continue
        base = os.path.splitext(os.path.basename(rel))[0]
        if base != n:
            problems.append("判据名 ≠ 件名去扩展名逐字相同：表内 %s ⇄ 件 %s" % (n, rel))
        if len(live[n]) == 1 and rel != live[n][0]:
            problems.append("表内路径与盘上现读不符：%s ⇄ %s" % (rel, live[n][0]))
        full = os.path.join(root, rel)
        if not os.path.isfile(full):
            problems.append("表内点名的件在盘上不存在：%s" % rel)
            continue
        sha = sha256_file(full)
        if sha != e.get("sha256"):
            problems.append("缓存键过期（sha256 不符 · 请 `--emit` 重生成）：%s" % rel)

    # ── 第三方向：派生半（在册）⇄ 人签半（采纳 / 豁免 / 声明）· fail-closed ──────────────────
    if decisions is not None:
        enforce = decisions.get("enforce", ENFORCE_DEFAULT)
        if not isinstance(enforce, bool):
            problems.append("人签半 `enforce` 非布尔（闭集 true/false · 现读 %r）⇒ 按 `false`（迁移期）读"
                            % (enforce,))
            enforce = ENFORCE_DEFAULT
        # ★ 上牙（enforce=true）本身必须人签（拍板③）
        if enforce is True:
            esig = decisions.get("enforce_sig")
            if not (isinstance(esig, dict) and esig.get("by") and esig.get("at") and esig.get("sig")):
                problems.append("人签半 `enforce=true`（**上牙**）**未人签**"
                                "（缺顶层 `enforce_sig` 的 `by`/`at`/`sig` 之一）")
        adopted = {}      # id -> entry（采纳面）
        declared = {}     # id -> entry（声明面 · 与采纳面**正交**）
        for d in decisions.get("decisions", []):
            if not isinstance(d, dict) or "id" not in d:
                problems.append("人签半条目缺 `id`：%r" % (d,))
                continue
            did = d["id"]
            v = d.get("decision")
            if v not in DECISION_VALUES:
                problems.append("人签半决定值不在闭集 %s：%s（现读 %r）"
                                % (list(DECISION_VALUES), did, v))
            if not (d.get("by") and d.get("at") and d.get("sig")):
                problems.append("人签半决定**未人签**（缺 `by`/`at`/`sig` 之一）：%s" % did)
            if v in ADOPTION_VALUES:
                if did in adopted:
                    problems.append("人签半内同一条目**采纳/豁免重复或矛盾**：%s" % did)
                adopted[did] = d
            elif v == "declare":
                if did in declared:
                    problems.append("人签半内同一条目**declare 重复**：%s" % did)
                declared[did] = d
                fields = d.get("fields")
                if not isinstance(fields, dict):
                    problems.append("declare 缺载荷 `fields`（四字段 %s）：%s"
                                    % (list(DECLARE_FIELDS), did))
                else:
                    for k in DECLARE_FIELDS:
                        if k not in fields:
                            problems.append("declare 载荷缺字段 `%s`：%s" % (k, did))
                            continue
                        pb = _declare_field_problem(did, k, fields.get(k))
                        if pb:
                            problems.append(pb)
        # 采纳面：在册无采纳/豁免决定 ⇒ **受 `enforce` 控**（迁移期只脚注 · 上牙判红）
        undecided = _adoption_undecided(reg, decisions)
        if enforce is True:
            for n in undecided:
                problems.append("在册判据**无人签采纳/豁免决定**（`enforce=true` 上牙）：%s" % n)
        # 悬空：决定点名不在册、盘上也无件（两轴合计 · 与 `enforce` 无关 —— 这是**写错**，不是迁移期）
        for n in sorted(set(adopted) | set(declared)):
            if n not in reg_entries and n not in live:
                problems.append("人签半**悬空**（点名不在册、盘上也无件）：%s" % n)
    return problems


def load_registry(path):
    with open(path, encoding="utf-8") as f:
        reg = json.load(f)
    if not isinstance(reg, dict) or reg.get("schema") != SCHEMA:
        raise ValueError("schema 不符（须 `%s` · 现读 %r）" % (SCHEMA, (reg or {}).get("schema")
                                                              if isinstance(reg, dict) else reg))
    return reg


def cmd_emit(root, out):
    # ★ 派生半写面：**只写**派生表（甲）。✗ 不读 / 不写人签半（乙）—— 这是「拆两半」的硬边。
    reg = build_registry(root)
    text = json.dumps(reg, ensure_ascii=False, indent=2) + "\n"
    if out == "-":
        sys.stdout.write(text)
    else:
        d = os.path.dirname(out)
        if d and not os.path.isdir(d):
            os.makedirs(d, exist_ok=True)
        with open(out, "w", encoding="utf-8") as f:
            f.write(text)
        print("判据注册表（派生半）已生成：%s（条目 %d · 同名歧义 %d）"
              % (out, reg["count"], len(reg["dupes"])))
    if reg["dupes"]:
        sys.stderr.write("⚠ 盘上有同名多件（%d）：%s\n" % (len(reg["dupes"]), " · ".join(reg["dupes"])))
    return 0


def cmd_check(root, reg_path, decisions_path):
    try:
        reg = load_registry(reg_path)
    except ValueError as e:
        print("⛔ 注册表加载期硬错：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    except OSError as e:
        print("⛔ 注册表读不到：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    try:
        decisions = load_decisions(decisions_path)
    except ValueError as e:
        print("⛔ 人签半加载期硬错：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    except OSError as e:
        print("⛔ 人签半读不到：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    # 乙半可见面（✗ 不改退码）：`enforce` 状态 + 逐条采纳/豁免/声明 + 未决定计数；未建则打迁移期脚注。
    for nt in _decision_notes(decisions, reg):
        print("  · 乙半 " + nt, file=sys.stderr)
    problems = _problem_list(root, reg, decisions)
    if problems:
        print("✗ 判据注册表对拍**红**（双向差集 / 列面 / 甲⇄乙 %d 条）：" % len(problems), file=sys.stderr)
        for p in problems:
            print("  · " + p, file=sys.stderr)
        return 1
    print("✓ 判据注册表对拍绿（rc=0）：条目 %d · 双向差集空 · 缓存键新鲜 · 甲⇄乙 %s"
          % (reg.get("count", 0), _pair_state(decisions)))
    return 0


def _pair_state(decisions):
    """甲⇄乙 对拍态一句话（绿行用 · 与 `_decision_notes` 同源）。"""
    if decisions is None:
        return "迁移期（人签半未建）"
    return "对拍过（enforce=true 上牙）" if decisions.get("enforce", ENFORCE_DEFAULT) is True \
        else "对拍过（enforce=false 迁移期）"


def _write(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as f:
        f.write(text)


def cmd_self_test():
    """成对负控：合成夹具上，正控绿 + 四条负控各自必红 ＋ **拆两半成对负控**（格A–格L）。"""
    ok = True
    with tempfile.TemporaryDirectory(prefix="judge-reg-self-") as root:
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")
        _write(root, "scripts/b.sh", "#!/usr/bin/env bash\necho b\n")
        _write(root, "scripts/sub/c.py", "#!/usr/bin/env python3\nprint('c')\n")
        _write(root, "scripts/gates/__pycache__/ignored.py", "# 缓存件，不进名录\n")

        reg = build_registry(root)
        reg_path = os.path.join(root, "reg.json")
        with open(reg_path, "w", encoding="utf-8") as f:
            json.dump(reg, f, ensure_ascii=False)

        # 正控
        probs = _problem_list(root, reg)
        if probs:
            print("✗ 正控：干净夹具被判红：%s" % probs)
            ok = False
        else:
            print("✓ 正控：干净夹具判绿（条目 %d）" % reg["count"])

        # ── 拆两半 · 甲⇄乙 成对负控（**在人签半/负控尚未扰动夹具前**跑 · 夹具此刻干净）──────────
        # 格A：`--emit` 产物 ✗ 不得含人签半任何字段（甲半 ✗ 覆盖人的决定）
        #   ★ 拍板③：连四字段（needs_env/serial/hazard/dry_run）也**不得**出现在 `--emit` 产物里。
        emit_text = json.dumps(reg, ensure_ascii=False)
        bad_emit = [k for k in ('"decision"', '"adopt"', '"exempt"', '"declare"', '"sig"', '"adopted"',
                                '"needs_env"', '"serial"', '"hazard"', '"dry_run"') if k in emit_text]
        if bad_emit:
            print("✗ 格A（甲/乙 分离）：`--emit` 产物含人签半字段：%s" % bad_emit)
            ok = False
        else:
            print("✓ 格A（甲/乙 分离）：`--emit` 产物无人签半字段（含四字段已搬走）")

        # 格B：人签半未建 ⇒ 乙半面不判红（迁移期）
        p = _problem_list(root, reg, None)
        if any("人签" in x for x in p):
            print("✗ 格B（人签半未建）：乙半面误判红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格B（人签半未建）：乙半面不判红（迁移期只报告）")

        # 人签半夹具：覆盖在册全部件 ⇒ adopt（人签三件套齐）· `enforce=false`（迁移期）
        dec = {"schema": DECISIONS_SCHEMA, "enforce": ENFORCE_DEFAULT, "decisions": [
            {"id": n, "decision": "adopt", "by": "tester", "at": "2026-10-03", "sig": "sig-%s" % n}
            for n in sorted(e["name"] for e in reg["entries"])]}

        # 格C：全覆盖 adopt ⇒ 绿
        p = _problem_list(root, reg, dec)
        if p:
            print("✗ 格C（人签半全覆盖 adopt）：该绿没绿 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格C（人签半全覆盖 adopt）：判绿")

        # 格D：抽掉一条决定 ＋ **上牙** ⇒ 红（在册无人签采纳/豁免决定）
        d2 = json.loads(json.dumps(dec))
        d2["enforce"] = True
        d2["enforce_sig"] = {"by": "tester", "at": "2026-10-03", "sig": "s-enforce"}
        d2["decisions"] = [x for x in d2["decisions"] if x["id"] != "a"]
        p = _problem_list(root, reg, d2)
        if not any("无人签采纳/豁免决定" in x for x in p):
            print("✗ 格D（enforce=true · 在册缺一条决定）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格D（enforce=true · 在册缺一条决定）：现红")

        # 格E：悬空决定 ⇒ 红
        d3 = json.loads(json.dumps(dec))
        d3["decisions"].append({"id": "ghost-x", "decision": "adopt",
                                "by": "tester", "at": "2026-10-03", "sig": "s"})
        p = _problem_list(root, reg, d3)
        if not any("悬空" in x for x in p):
            print("✗ 格E（悬空决定）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格E（悬空决定）：现红")

        # 格F：决定值不在闭集 ＋ 未人签（sig 空）⇒ 两红
        d4 = json.loads(json.dumps(dec))
        for x in d4["decisions"]:
            if x["id"] == "a":
                x["decision"] = "maybe"
                x["sig"] = ""
        p = _problem_list(root, reg, d4)
        if not (any("闭集" in x for x in p) and any("未人签" in x for x in p)):
            print("✗ 格F（闭集/未人签）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格F（闭集 / 未人签）：现红")

        # 格G：exempt **可见**（notes 逐条打印 · ✗ 不判红）
        d5 = json.loads(json.dumps(dec))
        for x in d5["decisions"]:
            if x["id"] == "a":
                x["decision"] = "exempt"
                x["reason"] = "太贵"
        notes = _decision_notes(d5, reg)
        if not any(("豁免" in n and "a" in n and "太贵" in n) for n in notes):
            print("✗ 格G（exempt 可见）：备注里没印出豁免 ⇒ %s" % notes)
            ok = False
        else:
            print("✓ 格G（exempt 可见）：备注逐条打印（非静默放过）")
        # 且 exempt 本身**不判红**
        if _problem_list(root, reg, d5):
            print("✗ 格G2（exempt ✗ 不判红）：豁免被误判红 ⇒ %s" % _problem_list(root, reg, d5))
            ok = False
        else:
            print("✓ 格G2（exempt ✗ 不判红）：豁免只报告不判红")

        # 格H：`--emit` 与人签半**无入参耦合**（R3：构造上不可能覆盖人的决定）
        import inspect as _inspect
        if "decisions" in _inspect.signature(build_registry).parameters:
            print("✗ 格H（`--emit` ✗ 不碰人签半）：`build_registry` 吃了 `decisions` 入参")
            ok = False
        else:
            print("✓ 格H（`--emit` ✗ 不碰人签半）：`build_registry` 与人签半无入参耦合")

        # ── 拍板③ · `declare` 类 ＋ 迁移期 `enforce` 开关 成对负控 ─────────────────────────────
        # 格I：declare 类（四字段值合法 · enforce=false）⇒ 不判红 · 载荷可见
        d6 = json.loads(json.dumps(dec))
        d6["enforce"] = False
        d6["decisions"] = [x for x in d6["decisions"] if x["id"] != "b"]      # 抽掉 b 的采纳
        d6["decisions"].append({"id": "b", "decision": "declare",
                                "by": "tester", "at": "2026-10-03", "sig": "sig-b",
                                "fields": {"needs_env": True, "serial": False,
                                           "hazard": "swap", "dry_run": "argv"}})
        p = _problem_list(root, reg, d6)
        if p:
            print("✗ 格I（declare 合法 + enforce=false）：该绿没绿 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格I（declare 合法 + enforce=false）：判绿（b 仅声明未采纳 ⇒ 采纳面迁移期不判红）")
        notes = _decision_notes(d6, reg)
        if not any(("声明" in n and "hazard" in n and "swap" in n and "dry_run" in n) for n in notes):
            print("✗ 格I2（declare 可见）：备注里没印出四字段 ⇒ %s" % notes)
            ok = False
        else:
            print("✓ 格I2（declare 可见）：备注逐条打印四字段（非静默）")

        # 格J：declare 载荷缺 `fields` / 缺字段 / 值越出闭集（needs_env 非布尔 · hazard / dry_run 越界）⇒ 红
        d7 = json.loads(json.dumps(dec))
        d7["enforce"] = False
        d7["decisions"].append({"id": "c", "decision": "declare",
                                "by": "tester", "at": "2026-10-03", "sig": "sig-c",
                                "fields": {"needs_env": "yes", "serial": False,
                                           "hazard": "boom", "dry_run": "yes"}})
        d7["decisions"].append({"id": "a", "decision": "declare",
                                "by": "tester", "at": "2026-10-03", "sig": "sig-a"})
        p = _problem_list(root, reg, d7)
        want = (any("`needs_env` 非布尔" in x for x in p)
                and any("`hazard` 值不在闭集" in x for x in p)
                and any("`dry_run` 值不在闭集" in x for x in p)
                and any("缺载荷 `fields`" in x for x in p))
        if not want:
            print("✗ 格J（declare 越界/缺键）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格J（declare 越界/缺键）：现红（非布尔 / 值越闭集 / 缺载荷 各自点名）")

        # 格J2：declare 载荷缺**单个**字段键 ⇒ 红
        d7b = json.loads(json.dumps(dec))
        d7b["enforce"] = False
        d7b["decisions"].append({"id": "c", "decision": "declare",
                                 "by": "tester", "at": "2026-10-03", "sig": "sig-c",
                                 "fields": {"needs_env": False, "serial": False, "hazard": "none"}})
        p = _problem_list(root, reg, d7b)
        if not any("declare 载荷缺字段 `dry_run`" in x for x in p):
            print("✗ 格J2（declare 缺字段键）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格J2（declare 缺字段键）：现红（缺 `dry_run` 点名）")

        # 格K：`enforce` 两态 —— false 未决定不判红；true（`enforce_sig` 齐）未决定判红 · 脚注报计数
        d8 = {"schema": DECISIONS_SCHEMA, "enforce": False, "decisions": []}
        d9 = {"schema": DECISIONS_SCHEMA, "enforce": True, "decisions": [],
              "enforce_sig": {"by": "tester", "at": "2026-10-03", "sig": "s-enforce"}}
        p_false = _problem_list(root, reg, d8)
        p_true = _problem_list(root, reg, d9)
        if p_false:
            print("✗ 格K1（enforce=false）：未决定被误判红 ⇒ %s" % p_false)
            ok = False
        else:
            print("✓ 格K1（enforce=false）：未决定不判红（迁移期只报告）")
        if not any("无人签采纳/豁免决定" in x for x in p_true):
            print("✗ 格K2（enforce=true）：未决定该红没红 ⇒ %s" % p_true)
            ok = False
        else:
            print("✓ 格K2（enforce=true）：未决定判红（现 %d 条）" % len(p_true))
        notes_false = _decision_notes(d8, reg)
        if not any(("未决定 %d 条" % len(reg["entries"])) in n for n in notes_false):
            print("✗ 格K3（enforce=false 脚注）：未印「未决定 N 条」 ⇒ %s" % notes_false)
            ok = False
        else:
            print("✓ 格K3（enforce=false 脚注）：脚注报「未决定 %d 条」" % len(reg["entries"]))

        # 格L：上牙（enforce=true）**未人签** ⇒ 红（拍板③：上牙本身必须人签）
        d10 = {"schema": DECISIONS_SCHEMA, "enforce": True, "decisions": []}
        p = _problem_list(root, reg, d10)
        if not any(("上牙" in x and "未人签" in x) for x in p):
            print("✗ 格L（上牙未人签）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格L（上牙未人签）：现红（enforce=true 缺 enforce_sig）")

        # 格M：`enforce` 非布尔 ⇒ 红 · 且按 false（迁移期）读
        d11 = {"schema": DECISIONS_SCHEMA, "enforce": "yes", "decisions": []}
        p = _problem_list(root, reg, d11)
        if not any("`enforce` 非布尔" in x for x in p):
            print("✗ 格M（enforce 非布尔）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 格M（enforce 非布尔）：现红（并落回迁移期读）")

        # ── 甲半负控（承前 · 顺序不变）────────────────────────────────────────────────────
        # 负控①：盘上新增件、表里没有 ⇒ 红
        _write(root, "scripts/d.py", "#!/usr/bin/env python3\nprint('d')\n")
        p = _problem_list(root, reg)
        if not any("没点名" in x for x in p):
            print("✗ 负控①（盘上有表里无）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控①（盘上有件表里无）：现红")
        os.remove(os.path.join(root, "scripts/d.py"))

        # 负控②：表里点名、盘上无件 ⇒ 红
        ghost = json.loads(json.dumps(reg))
        ghost["entries"].append({"id": "ghost", "name": "ghost", "path": "scripts/ghost.py",
                                 "sha256": "0" * 64})
        p = _problem_list(root, ghost)
        if not any("悬空登记" in x for x in p):
            print("✗ 负控②（表里点名盘上无）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控②（表里点名盘上无）：现红")

        # 负控③：件被改、缓存键过期 ⇒ 红
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a-changed')\n")
        p = _problem_list(root, reg)
        if not any("缓存键过期" in x for x in p):
            print("✗ 负控③（缓存键过期）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控③（缓存键过期）：现红")
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")

        # 负控④：同名多件 ⇒ 红
        _write(root, "scripts/sub2/a.py", "#!/usr/bin/env python3\nprint('a2')\n")
        p = _problem_list(root, reg)
        if not any("同名多件" in x for x in p):
            print("✗ 负控④（同名多件）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控④（同名多件）：现红")

        # 负控⑤：路径越出前缀 ⇒ 红
        bad = json.loads(json.dumps(reg))
        for e in bad["entries"]:
            if e["name"] == "a":
                e["path"] = "core/a.py"
        p = _problem_list(root, bad)
        if not any("前缀" in x or "悬空登记" in x for x in p):
            print("✗ 负控⑤（越出前缀）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控⑤（越出前缀）：现红")

    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="判据注册表真源生成 + 双向差集自检（M1-04/M1-05）")
    ap.add_argument("--root", default=REPO, help="仓根（默认 = 本件上溯三层）")
    g = ap.add_mutually_exclusive_group()
    g.add_argument("--emit", action="store_true", help="现读生成注册表（派生半 · ✗ 不碰人签半）")
    g.add_argument("--check", action="store_true", help="双向差集 ＋ 甲⇄乙 对拍")
    g.add_argument("--self-test", dest="self_test", action="store_true", help="成对负控自检")
    ap.add_argument("--out", default=REGISTRY_DEFAULT, help="--emit 的写出件（默认 scripts/gates/judge-registry.json）")
    ap.add_argument("--registry", default=REGISTRY_DEFAULT, help="--check 读的注册表件")
    ap.add_argument("--decisions", default=DECISIONS_DEFAULT,
                    help="--check 读的人签半（采纳/豁免/声明 · 缺省 scripts/gates/judge-adoption.json）· `--emit` ✗ 不碰它")
    args = ap.parse_args(argv)

    root = os.path.abspath(args.root)
    if args.self_test:
        return cmd_self_test()
    if args.emit:
        out = args.out if args.out == "-" else os.path.join(root, args.out) if not os.path.isabs(args.out) else args.out
        # ★ `--emit` 只写派生半：**不**解析 / 不读 `--decisions`。
        return cmd_emit(root, out)
    if args.check:
        reg = args.registry if os.path.isabs(args.registry) else os.path.join(root, args.registry)
        dec = args.decisions if os.path.isabs(args.decisions) else os.path.join(root, args.decisions)
        return cmd_check(root, reg, dec)
    sys.stderr.write("用法错：`--emit` / `--check` / `--self-test` 三选一必给一个（都不给 ⇒ rc=2）\n")
    return 2


if __name__ == "__main__":
    sys.exit(main())

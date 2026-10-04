#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-merge.py v1.0.0 —— 人签**跨机合并视图**只读对拍件（`Q-080` · 设计件推荐第 10 条）

落地面（**只**落设计件已经定形的那两格判据 + 一枚只读对拍面）
----------------------------------------------------------
本件是 `Zerg-内部文档/项目文档/v2.5.13/设计-人签跨机合并视图-v0.1.md` §六 推荐第 10 条的落地：
「**先落 `A5`/`A6` 两枚判据的夹具 + 负控**（红/2 两档先有牙）✓」。

  **`A5`（设计件 `:142` 逐字）**：「**\"负数不许默认本机\"的机器形态**：产出面里 `\"本机\"` 当值 = **0** 处」
     形态（逐字）＝ 落成后 `grep -n '\"本机\"' <产出> > <临时件>; echo $?` ⇒ 期望 **1**（0 命中）；
     **再**对它做**变异**（写死 `Mr2109`）⇒ 期望 **红**。
  **`A6`（设计件 `:143` 逐字）**：「**必填格缺一不许写这一行**（同族纪律照 `family_approve.go:679-696` `complete()`）」
     形态（逐字）＝ 造缺 `src_machine` / 缺偏移的夹具 ⇒ 期望 `row_state=缺证据` + **rc=2** + `missing_cells` 点名。

因此本件做**两件**（都在这一个文件 + `scripts/gates/fixtures/approve-merge/` 里）：
  ① **只读对拍面**：吃「人喂进来的多份快照」（设计件 §六 开放问题 6 的推荐 ①：给两枚文件路径），
     按设计件 §二.1 的 **15 格闭集**排出**第三本**的行，判**三态**（`能合`/`合不了`/`缺证据`）；
  ② **`A5`/`A6` 夹具 + 负控**：夹具是**数据**（`cases.json` + `sources/` + 反例产出），
     负控是**真反例件**（`view.bad-*.json` ＝「实现照 `core/internal/api/handlers.go:223` 的
     `t.Machine = \"本机\"` 写法把缺的来源机格填成 `本机`/`localhost`/拿 `default_node` 兜底」的**错产出**）
     ⇒ 本件必须**抓住**它们（判红 `rc=1`）。

★ **本件只读**：读来源快照 / 名册 / 产出面，**一个字节都不写** ✗（不落盘 · 不写审计 · 不写件 · 不碰 git）。
★ **缺证据就 `rc=2`，不编造**：名册读不到 / 来源件读不到 / 一行都不认 ⇒ **不出结论**（`rc=2`），
  绝不用本机名、`default_node`、`hostname` 或任何默认值把空格填上 ✗（设计件 §三.1 口径 1 + §三.2）。
★ **本件不做验签** ⇒ `sig_state` 一律「未验·…」，**不许**读成「签名坏」（设计件 §二.1 第 10 格红线）。

退码（三档 · 与设计件 §二.3 同口径）
--------------------------------------------------
  0 = 能合（全表无「合不了」也无「缺证据」）· 1 = **红**（合不了未裁 / 产出面违规）· 2 = **不给结论**
  （缺证据 / 未取数 / 自检不过 / 用法错）。`1` 与 `2` **不许互当**（设计件 §二.3 那条分界）。

两处**明标待拍**（本件不自作主张当已定 · 见文件末尾 `OPEN_POINTS` 与运行时的打印）
--------------------------------------------------
  · 设计件 §二.3 **没定**「同一次运行里既有 `合不了` 又有 `缺证据`」时哪个退码优先 ⇒ 本件自定 **红 > 2**
    （理由：红 = 「判出了错」、2 = 「今天判不了」，判出了错就不该退回「判不了」）—— 标 `待拍`。
  · 设计件 §三.1 **口径 2**（「`本机` 这个字面只许出现在件头那句『本机钟』」）与 §二.1 **第 10 格**的枚举
    值 `未验·本机无此钥` **逐字冲突**（该枚举值本身含 `本机`）⇒ 本件把「`本机` 当值」的检索面**收窄**到
    **机器面格**（`src_machine` / `src_basis` / `src_path` / `clock_basis`）**＋任何字段精确等于 `本机`**
    —— 这是**收窄后的口径**，标 `待拍`（不收窄就会把设计件自己的枚举值判红）。

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-merge.py                       # 先自检，再扫真目标（只读）
  python3 scripts/gates/check-approve-merge.py --self-test           # 只跑自检（夹具 + 变异 + 用法错）
  python3 scripts/gates/check-approve-merge.py --fixtures <目录>     # 只跑夹具（零依赖 · 不碰真目标）
  python3 scripts/gates/check-approve-merge.py --audit-view <产出.json>   # 只审计一份**产出面**（`A5`）
  python3 scripts/gates/check-approve-merge.py --source <[机名:]路径> [--source …] [--machine <名册键>]
  python3 scripts/gates/check-approve-merge.py --json                # 机器读面（视图原样 · schema approve-merge/v1）
"""

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import subprocess
import sys

VERSION = "check-approve-merge.py v1.0.0（Q-080 · 设计件推荐第 10 条：A5/A6 夹具 + 负控 + 只读对拍面）"
VIEW_SCHEMA = "approve-merge/v1"
FIXTURE_SCHEMA = "approve-merge-fixtures/1"
HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
FIXTURES_DEFAULT = os.path.join(HERE, "fixtures", "approve-merge")

# ── 15 格闭集（设计件 §二.1 · 顺序逐字照抄；`src_machine` 是**唯一**允许缺键的一格）─────────────
FIELDS = ["src_machine", "src_machine_known", "src_basis", "src_path", "tool", "approver",
          "approved_at", "key_id", "sig_alg", "sig_state", "clock_basis", "row_state",
          "missing_cells", "conflict_with", "notes"]

SRC_BASIS_ENUM = ("audit_cell", "path_prefix", "argv")      # §二.2 的**唯一合法取值源**（三条）
SIG_STATE_ENUM = ("验过", "未验·本机无此钥", "未验·件不在")    # §二.1 第 10 格
CLOCK_BASIS_ENUM = ("同机", "跨机·不判")                     # §二.1 第 11 格
ROW_STATE_ENUM = ("能合", "合不了", "缺证据")                  # §二.3 三态

# 「能合」要求的必填格（设计件 §二.3 逐字：src_machine(known) ∧ tool ∧ approver ∧ approved_at(带偏移) ∧ key_id ∧ row_state）
REQUIRED_FOR_MERGE = ("src_machine", "tool", "approver", "approved_at", "key_id", "row_state")

# 件头固定三句（设计件 §二.1 末 · 每次必打）+ 开放问题 3 的推荐句（「载体级、非不可伪造」必须进件头）
HEADERS = [
    "本视图是第三本，不是真源、不是信任根（第三本口径）",
    "未改两处（多处）原文件一个字节",
    "两机钟不是一本钟 ⇒ 涉及时刻一律 跨机·不判",
]
STRENGTH_NOTE = ("来源机列是**载体级**证据（审计行 / 路径前缀 / 显式传参 —— 三者皆可改），"
                 "**不是**不可伪造级：机器名不在待签 5 格里 ⇒ 从签名推不出来源机")

# A5 的检索面：拿本机值充数的**字面**（设计件 §三.2 / §三.3 反例 1·2·4）
MACHINE_FACE_FIELDS = ("src_machine", "src_basis", "src_path", "clock_basis")
FORBIDDEN_MACHINE_VALUES = ("本机", "localhost", "127.0.0.1", "::1")

# 待签 5 格（设计件 §1.1 ② · 前缀 zerg-approval/v1）—— 冲突判据的备用内容指纹
PAYLOAD_CELLS = ("tool", "scope", "approver", "approved_at", "note")

OPEN_POINTS = [
    "写死来源机**且**同时声称 `src_basis=argv` ⇒ 产出面上与合法 argv 传参不可分（载体是命令行）"
    " ⇒ 只有「载体为 cell/prefix 的行」与变异自检抓得住（待拍：要不要把 argv 也落进产出面）",
    "退码定序：设计件 §二.3 未定「同一次运行里 `合不了` 与 `缺证据` 并存」谁优先 ⇒ 本件自定 红 > 2（待拍）",
    "「本机」检索面：§三.1 口径 2 与 §二.1 第 10 格枚举 `未验·本机无此钥` 逐字冲突 ⇒ 本件收窄到机器面格（待拍）",
    "本笔不做验签 ⇒ `sig_state` 一律「未验·…」（`验过` 这一格今天取不到 ⇒ 不编）",
]


# ══════════════════════════════════════════════════════════════════════════════
# 一 · 取数（只读）：来源快照 / 名册 / 产出面
# ══════════════════════════════════════════════════════════════════════════════

def sha256_file(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for blk in iter(lambda: f.read(65536), b""):
            h.update(blk)
    return h.hexdigest()


def read_roster(path, nodes_arg):
    """名册 `nodes:` 键集（设计件 §二.2：取值**必须是名册键**）。取不到 ⇒ (None, 为什么)。"""
    if nodes_arg:
        keys = [k.strip() for k in nodes_arg.split(",") if k.strip()]
        return (keys, "") if keys else (None, "`--roster-nodes` 一个键都没给 ⇒ 不给结论")
    if not path or not os.path.exists(path):
        return None, "名册件不在：%s ⇒ 来源机**无法对拍**（不给结论）" % path
    keys, in_nodes, ind = [], False, None
    with open(path, encoding="utf-8") as f:
        for ln in f:
            s = ln.rstrip("\n")
            if re.match(r"^nodes:\s*$", s):
                in_nodes, ind = True, None
                continue
            if not in_nodes:
                continue
            if not s.strip() or s.lstrip().startswith("#"):
                continue
            m = re.match(r"^(\s*)([^\s:]+):\s*$", s)
            if m:
                cur = len(m.group(1))
                if ind is None:
                    ind = cur
                if cur == ind:
                    keys.append(m.group(2))
                    continue
            if ind is not None and len(s) - len(s.lstrip()) <= ind and s.strip() and ":" in s:
                # 回到顶层键（缩进 ≤ nodes 的键缩进且不是子项）⇒ nodes 段结束
                break
    if not keys:
        return None, "名册件 `%s` 里 `nodes:` 一个键都没读到 ⇒ 不给结论" % path
    return keys, ""


def split_source_arg(arg):
    """`<机名>:<路径>` 前缀（设计件 §二.2 第 2 条）⇒ (机名|None, 路径)。"""
    if ":" in arg:
        pre, rest = arg.split(":", 1)
        if pre and "/" not in pre and not pre.startswith("~") and (rest.startswith("/") or rest.startswith("~")):
            return pre, rest
    return None, arg


def read_source(spec):
    """一处来源 ⇒ 行草稿清单。spec = {arg, prefix, path}。读不到 ⇒ (None, 为什么)。"""
    p = os.path.expanduser(spec["path"])
    if not os.path.exists(p):
        return None, "来源件不在：%s" % p
    if os.path.isdir(p):
        return None, "来源件是目录（本笔只吃文件）：%s" % p
    drafts = []
    if p.endswith(".jsonl"):
        with open(p, encoding="utf-8") as f:
            for i, ln in enumerate(f, 1):
                if not ln.strip():
                    continue
                try:
                    d = json.loads(ln)
                except json.JSONDecodeError as e:
                    return None, "来源 %s 第 %d 行不是 JSON（%s）⇒ 不许猜" % (p, i, e)
                if str(d.get("event", "")) != "approve_signed":
                    continue                      # 只认人签类事件（本笔面）
                drafts.append({
                    "path": p, "line": i, "src_path": "%s:%s:%d" % (spec["prefix"] or "?", p, i),
                    "cell_machine": d.get("machine") or d.get("host"),
                    "tool": d.get("tool"), "approver": d.get("approver"),
                    "approved_at": d.get("approved_at") or d.get("at"),
                    "key_id": d.get("key_id"), "sig_alg": d.get("sig_alg"),
                    "content_sha": d.get("signed_payload_sha256"),
                    "carrier": "audit_line",
                })
    else:
        with open(p, encoding="utf-8") as f:
            try:
                obj = json.load(f)
            except json.JSONDecodeError as e:
                return None, "来源 %s 解不出来（%s）⇒ 不许猜" % (p, e)
        if not isinstance(obj, dict) or "tool" not in obj:
            return None, "来源 %s 不是批准件本体（无 `tool` 格）⇒ 认不出来就不认" % p
        body = "zerg-approval/v1\n" + json.dumps({k: obj.get(k) for k in PAYLOAD_CELLS},
                                                 ensure_ascii=False, sort_keys=True)
        drafts.append({
            "path": p, "line": 1, "src_path": "%s:%s:1" % (spec["prefix"] or "?", p),
            "cell_machine": obj.get("machine") or obj.get("host"),
            "tool": obj.get("tool"), "approver": obj.get("approver"),
            "approved_at": obj.get("approved_at"), "key_id": obj.get("key_id"),
            "sig_alg": obj.get("sig_alg"),
            "content_sha": obj.get("signed_payload_sha256") or hashlib.sha256(body.encode()).hexdigest(),
            "carrier": "ticket_body",
        })
    return drafts, ""


ISO_OFFSET = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$")
HEX16 = re.compile(r"^[0-9a-f]{16}$")


# ══════════════════════════════════════════════════════════════════════════════
# 二 · 判定口（纯函数：自检 / 夹具 / 真目标**共用同一处**）
# ══════════════════════════════════════════════════════════════════════════════

def resolve_machine(draft, prefix, machine_argv):
    """设计件 §二.2 的**优先级序**：① 审计行 cell ② 路径前缀 ③ `--machine`；三者全无 ⇒ 缺。"""
    if draft.get("cell_machine"):
        return str(draft["cell_machine"]), "audit_cell"
    if prefix:
        return prefix, "path_prefix"
    if machine_argv:
        return machine_argv, "argv"
    return None, ""


def build_rows(sources, roster, machine_argv, hostname, policy=None):
    """排「第三本」的行（设计件 §二.1 的 15 格闭集）。

    `policy` = **变异口**（只在自检里用 · 真跑恒为 None）：
      错实现「来源格缺时补一个本机值」的**机器形态** —— `{"fill_missing": <值>, "basis_override": <依据>}`。
      本件用它造「写死本机 / 拿 default_node 兜底」的变异体 ⇒ 断言自己的 `A5` 检索面**必红**。
    """
    policy = policy or {}
    rows = []
    for spec in sources:
        drafts, why = read_source(spec)
        if drafts is None:
            return None, why
        for d in drafts:
            machine, basis = resolve_machine(d, spec.get("prefix"), machine_argv)
            if policy.get("hardcode"):      # 变异：把来源机**写死**（无视载体 ⇒ §三.4 负控②）
                machine = policy["hardcode"]
                basis = basis or "argv"
            missing = []
            if d.get("sig_alg") in (None, ""):
                missing.append("sig_alg")
            if not machine:
                # ★ A6：必填格缺 ⇒ **不写这一格**（绝不许填本机名 / "本机" / default_node ✗）
                if "fill_missing" in policy:                       # ← 变异体（错实现）走这里
                    machine = policy["fill_missing"]
                    basis = policy.get("src_basis", basis)
                else:
                    missing.insert(0, "src_machine")
            if not d.get("tool"):
                missing.append("tool")
            if not d.get("approver"):
                missing.append("approver")
            at = d.get("approved_at")
            if not at or not ISO_OFFSET.match(str(at)):
                missing.append("approved_at")                       # 偏移缺 ⇒ 同样按缺处理（§二.1 第 7 格）
                at = ""                                             # ★ §二.1：缺 ⇒ 写 `""`（原文照抄，绝不补本机时区）
            kid = d.get("key_id")
            if not kid or not HEX16.match(str(kid)):
                missing.append("key_id")
                kid = ""                                            # ★ §二.1 第 8 格：缺 ⇒ 写 `""`
            known = bool(machine)
            state = "缺证据" if ([f for f in REQUIRED_FOR_MERGE if f in missing] or
                                 (machine and machine not in roster)) else "能合"
            rows.append({
                "src_machine": machine, "src_machine_known": known, "src_basis": basis or "",
                "src_path": d["src_path"], "tool": d.get("tool") or "",
                "approver": d.get("approver") or "", "approved_at": at,
                "key_id": kid, "sig_alg": d.get("sig_alg") or "",
                "sig_state": ("未验·件不在" if not os.path.exists(d["path"]) else "未验·本机无此钥"),
                "clock_basis": ("同机" if (machine and machine == hostname) else "跨机·不判"),
                "row_state": state, "missing_cells": missing,
                "conflict_with": [], "notes": [],
                "_content_sha": d.get("content_sha") or "",
                "_carrier": d["carrier"],
            })
    # 冲突（设计件 §二.3 `合不了`：同 (tool,key_id) 而在多台机内容不一致 ⇒ 两面并列、不选边、不判谁对）
    groups = {}
    for r in rows:
        if r["tool"] and r["key_id"]:
            groups.setdefault((r["tool"], r["key_id"]), []).append(r)
    for _, g in groups.items():
        shas = {r["_content_sha"] for r in g if r["_content_sha"]}
        if len(g) > 1 and len(shas) > 1:
            for r in g:
                r["conflict_with"] = sorted(x["src_path"] for x in g if x is not r)
                r["notes"].append("冲突未裁（同 tool+key_id、内容不一致）⇒ 两面并列、不选边、不判谁对")
                if r["row_state"] == "能合":
                    r["row_state"] = "合不了"
    # 名册外 / 未知来源机的交代串（不许静默）
    for r in rows:
        if r["src_machine"] and r["src_machine"] not in roster:
            r["notes"].append("该机不在名册 `nodes:` 键集内（%s）⇒ 不许进「能合」" % r["src_machine"])
        if not r["src_machine_known"]:
            r["notes"].append("来源机格缺 ⇒ `src_machine_known=false` · **不给结论**（这一格不许读成绿）")
    # 输出面：`src_machine` 缺 ⇒ **不写这一格**（设计件 §二.1 第 1 格「缺失时怎么写」逐字）
    for r in rows:
        content = r.pop("_content_sha")
        r.pop("_carrier")
        if not r["src_machine_known"]:
            r.pop("src_machine", None)
        else:
            r["missing_cells"] = [c for c in r["missing_cells"] if c != "src_machine"]
        r["_content_sha"] = content
    return rows, ""


def judge_rc(rows):
    """三态 → 整件退码。**红 > 2**（本件自定序 · 设计件未定 · 待拍）。"""
    if any(r["row_state"] == "合不了" for r in rows):
        return 1
    if any(r["row_state"] == "缺证据" for r in rows):
        return 2
    return 0


def build_view(rows, sources, hostname):
    return {
        "schema": VIEW_SCHEMA, "kind": "ApproveMergeView",
        "headers": list(HEADERS), "strength_note": STRENGTH_NOTE,
        "view_id": "approve-merge/v1",
        "sources": [{"source": "%s:%s" % (s.get("prefix") or "?", os.path.expanduser(s["path"])),
                     "sha256": (sha256_file(os.path.expanduser(s["path"]))
                                if os.path.exists(os.path.expanduser(s["path"])) else None),
                     "rows": sum(1 for r in rows if s["path"] in r["src_path"])}
                    for s in sources],
        "generated_at": dt.datetime.now().astimezone().isoformat(timespec="seconds"),
        "generated_at_note": "本机钟 · **不当新鲜度保证**（设计件 §二.1 件级字段 3）",
        "rows": [{k: r[k] for k in FIELDS if k in r} for r in rows],
    }


# ══════════════════════════════════════════════════════════════════════════════
# 三 · `A5` 检索面：审计一份**产出面**（值里不许有本机当值 · 闭集不许加格）
# ══════════════════════════════════════════════════════════════════════════════

def walk_strings(obj, path="", skip_headers=True):
    """逐格取 (JSON 路径, 字符串值) —— **件头不算值**（口径 2 明许件头那句「本机钟」）。"""
    if isinstance(obj, dict):
        for k, v in obj.items():
            if skip_headers and k in ("headers", "strength_note", "generated_at_note"):
                continue
            for x in walk_strings(v, "%s.%s" % (path, k), skip_headers):
                yield x
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            for x in walk_strings(v, "%s[%d]" % (path, i), skip_headers):
                yield x
    elif isinstance(obj, str):
        yield path, obj


def audit_view(view, roster):
    """产出面审计 ⇒ (红清单, 交代清单)。这就是 `A5` 的**机器形态**（`\"本机\"` 当值 = 0 处）。"""
    red, notes = [], []
    if not isinstance(view, dict):
        return ["产出面不是对象 ⇒ 认不出来"], notes
    if view.get("schema") != VIEW_SCHEMA:
        red.append("产出面 `schema` ≠ `%s`（读到 %r）" % (VIEW_SCHEMA, view.get("schema")))
    hs = view.get("headers")
    if not isinstance(hs, list) or list(hs) != HEADERS:
        red.append("件头固定三句不齐 / 走样（设计件 §二.1「每次必打」）")
    if not view.get("strength_note"):
        red.append("缺 `strength_note`（开放问题 3 的推荐：来源机的强度必须进件头）")
    rows = view.get("rows")
    if not isinstance(rows, list):
        return red + ["产出面无 `rows` 数组"], notes
    for i, r in enumerate(rows):
        if not isinstance(r, dict):
            red.append("rows[%d] 不是对象" % i)
            continue
        extra = [k for k in r if k not in FIELDS]
        if extra:
            red.append("rows[%d] 有 15 格闭集**之外**的格：%s" % (i, extra))
        gone = [k for k in FIELDS if k not in r and k != "src_machine"]
        if gone:
            red.append("rows[%d] 缺格（闭集内必填）：%s" % (i, gone))
        known = r.get("src_machine_known")
        if not isinstance(known, bool):
            red.append("rows[%d] `src_machine_known` 不是布尔（读到 %r）" % (i, known))
        if known is False and "src_machine" in r:
            red.append("rows[%d] `src_machine_known=false` 却**写了** `src_machine` 值（设计件 §二.1 第 1 格：不写这一格）" % i)
        if known is True and "src_machine" not in r:
            red.append("rows[%d] `src_machine_known=true` 却没有 `src_machine`（自相矛盾）" % i)
        if known is True:
            m = r.get("src_machine")
            if m not in roster:
                red.append("rows[%d] `src_machine=%r` 不在名册 `nodes:` 键集内（反例 4 同族）" % (i, m))
            if r.get("src_basis") not in SRC_BASIS_ENUM:
                red.append("rows[%d] `src_basis=%r` 不在 §二.2 的三个合法来源里"
                           "（`audit_cell`/`path_prefix`/`argv`）⇒ 拿别的东西兜底 = 编（反例 2）"
                           % (i, r.get("src_basis")))
        # ★ §三.4 负控② 的机器形态：「把实现里 `src_machine` 写死成 Mr2109 ⇒ **别机那一行必须红**」
        #   产出面可判的那一半 = 行的来源机与**它自己声明的载体**必须自洽
        #   （`src_basis=path_prefix` ⇒ `src_path` 必须以 `<src_machine>:` 开头；`audit_cell` 同理）。
        #   · 明标不可判的那一半：写死 + 同时声称 `argv` 依据 ⇒ 在产出面上与合法的 argv 传参**不可分**
        #     （只有 ① 载体是 cell/prefix 的行 ② 变异自检 抓得住）—— 见 OPEN_POINTS。
        if known is True and r.get("src_machine"):
            m, sp, bs = r.get("src_machine"), str(r.get("src_path") or ""), r.get("src_basis")
            pre = sp.split(":", 1)[0] if ":" in sp and not sp.startswith("/") else ""
            if bs in ("path_prefix", "audit_cell") and pre not in ("", "?", m):
                red.append("rows[%d] `src_basis=%s` 但回指写的机名是 %r、`src_machine` 是 %r ⇒ 值与载体不自洽"
                           "（写死来源机的错就在这一格露馅 · §三.4 负控②）" % (i, bs, pre, m))
        if r.get("row_state") not in ROW_STATE_ENUM:
            red.append("rows[%d] `row_state=%r` 不在三态里" % (i, r.get("row_state")))
        if r.get("clock_basis") not in CLOCK_BASIS_ENUM:
            red.append("rows[%d] `clock_basis=%r` 不在枚举里" % (i, r.get("clock_basis")))
        if r.get("sig_state") not in SIG_STATE_ENUM:
            red.append("rows[%d] `sig_state=%r` 不在枚举里（本笔不验签 ⇒ 不许写「签名坏」）" % (i, r.get("sig_state")))
        for f in ("missing_cells", "conflict_with", "notes"):
            if not isinstance(r.get(f), list):
                red.append("rows[%d] `%s` 不是数组（设计件 §二.1：无缺写 `[]`，**不是省掉这一格**）" % (i, f))
        if "src_machine" in r and r.get("src_machine") is None:
            red.append("rows[%d] `src_machine` 是 null（缺就把这一格**省掉**，不许写 null）" % i)
        if r.get("approved_at") and not ISO_OFFSET.match(str(r["approved_at"])):
            red.append("rows[%d] `approved_at=%r` 不带时区偏移 ⇒ 该走「缺证据」，不许当已判" % (i, r.get("approved_at")))
    # ★ A5 核心：**值**里不许出现本机当值
    for path, v in walk_strings(view):
        field = path.rsplit(".", 1)[-1].split("[")[0]
        if v in FORBIDDEN_MACHINE_VALUES:
            red.append("产出面 `%s` 的值 = %r ⇒ 拿本机值充数（A5：产出面「本机」当值 = 0 处）" % (path, v))
        elif "本机" in v and field in MACHINE_FACE_FIELDS:
            red.append("产出面 `%s` 的值 %r 含「本机」⇒ 机器面格不许拿本机当缺省（反例 1）" % (path, v))
        elif v.endswith(".local"):
            red.append("产出面 `%s` 的值 %r 是 `hostname` 形态 ⇒ 名册对拍必红（反例 4）" % (path, v))
    if not red:
        notes.append("A5：产出面「本机」当值 **0** 处 · 机器面格 `%s` 无本机字面 ✓"
                     % "、".join(MACHINE_FACE_FIELDS))
    return red, notes


# ══════════════════════════════════════════════════════════════════════════════
# 四 · 自检 / 夹具（夹具目录 = **数据**，零依赖；本笔不写第二份数据）
# ══════════════════════════════════════════════════════════════════════════════

def load_manifest(fixdir):
    p = os.path.join(fixdir, "cases.json")
    if not os.path.exists(p):
        return None, "夹具清单不在：%s" % p
    with open(p, encoding="utf-8") as f:
        try:
            m = json.load(f)
        except json.JSONDecodeError as e:
            return None, "夹具清单解不出来（%s）：%s" % (p, e)
    if m.get("schema") != FIXTURE_SCHEMA:
        return None, "夹具清单 `schema` ≠ `%s`（读到 %r）" % (FIXTURE_SCHEMA, m.get("schema"))
    return m, ""


def run_cli(argv):
    """真跑本件（**子进程 · 真退出码** · 不用管道）⇒ (rc, stdout, stderr)。"""
    p = subprocess.run([sys.executable, os.path.abspath(__file__)] + argv,
                       capture_output=True, text=True, cwd=REPO_ROOT)
    return p.returncode, p.stdout or "", p.stderr or ""


def json_from(stdout):
    """机器面 = stdout 末段那段 JSON（本件保证：`--json` 时 JSON 落在 stdout **最后**）。"""
    i = stdout.find("{")
    if i < 0:
        return {}
    try:
        return json.loads(stdout[i:])
    except Exception:
        return {}


def fixture_argv(case, fixdir):
    return [a.replace("@FIX@", fixdir) for a in case["argv"]]


def case_ok(case, got):
    """夹具期望 vs 现跑读数（只比**判据面**：rc / 行数 / 逐行三态与来源机格）。"""
    exp = case["expect"]
    if got["rc"] != exp["rc"]:
        return "rc 期望 %d · 实读 %d" % (exp["rc"], got["rc"])
    if "rows" in exp and len(got["rows"]) != len(exp["rows"]):
        return "行数期望 %d · 实读 %d" % (len(exp["rows"]), len(got["rows"]))
    for i, er in enumerate(exp.get("rows", [])):
        gr = got["rows"][i]
        for k in ("tool", "row_state", "src_basis", "src_machine_known"):
            if k not in er:
                continue
            ev, gv = er[k], gr.get(k, "<缺>")
            if ev != gv:
                return "rows[%d].%s 期望 %r · 实读 %r" % (i, k, ev, gv)
        if er.get("src_machine_absent"):
            if "src_machine" in gr:
                return "rows[%d] 缺来源机却**写了** `src_machine`（应省掉这一格）" % i
        elif "src_machine" in er:
            ev, gv = er["src_machine"], gr.get("src_machine", "<缺>")
            if ev != gv:
                return "rows[%d].src_machine 期望 %r · 实读 %r" % (i, ev, gv)
        if "missing_cells" in er and sorted(er["missing_cells"]) != sorted(gr.get("missing_cells") or []):
            return ("rows[%d].missing_cells 期望 %s · 实读 %s"
                    % (i, sorted(er["missing_cells"]), sorted(gr.get("missing_cells") or [])))
        if er.get("conflict_with") is True and not gr.get("conflict_with"):
            return "rows[%d].conflict_with 期望非空 · 实读空" % i
    if exp.get("no_machine_literal"):
        n = sum(1 for _, v in walk_strings(got["view"]) if v in FORBIDDEN_MACHINE_VALUES)
        if n:
            return "产出面出现本机字面当值 %d 处（A5 要求 0 处）" % n
    if exp.get("known_key_absent") and "src_machine" in got["rows"][0]:
        return "缺来源机的那一行**写了** `src_machine`（应省掉这一格）"
    return ""


def scan_sources(sources, roster, machine_argv, hostname, policy=None):
    rows, why = build_rows(sources, roster, machine_argv, hostname, policy=policy)
    if rows is None:
        return None, why
    return rows, ""


def run_fixtures(fixdir, quiet=False, manifest=None):
    """跑夹具目录里的**全部**案子（正控 + 负控 + 反例产出）⇒ (不过清单, 行数)。"""
    man = manifest
    if man is None:
        man, why = load_manifest(fixdir)
        if man is None:
            print("⛔ 夹具读不到：%s" % why)
            return ["夹具清单读不到：%s" % why], 0
    bad, n = [], 0
    for case in man.get("cases", []):
        n += 1
        rc, out, _ = run_cli(fixture_argv(case, fixdir))
        rcj, outj, _errj = run_cli(["--no-self-test", "--json"] + fixture_argv(case, fixdir))
        view = json_from(outj)
        got = {"rc": rc, "rows": view.get("rows", []), "view": view}
        msg = "" if view else "机器面取不到（rc=%d）" % rcj
        if got["rc"] != rcj:
            msg = msg or "人读面 rc=%d ≠ 机器面 rc=%d（两面不许不同形）" % (rc, rcj)
        msg = msg or case_ok(case, got)
        if msg:
            bad.append("[%s] %s" % (case["id"], msg))
            print("   ✗ 夹具 %s：%s" % (case["id"], msg))
        elif not quiet:
            print("   ✓ 夹具 %s（rc=%d · %d 行 · %s）" % (case["id"], rc, len(got["rows"]), case["why"]))
    for rv in man.get("rejects", []):
        n += 1
        p = os.path.join(fixdir, rv["path"])
        rc, out, err = run_cli(["--no-self-test", "--audit-view", p])
        if rc != rv["expect_rc"]:
            bad.append("[%s] 反例产出应判 rc=%d · 实读 %d（**抓不住这种错**）"
                       % (rv["id"], rv["expect_rc"], rc))
            print("   ✗ 反例 %s：该判 rc=%d · 实读 %d —— 这条负控没牙" % (rv["id"], rv["expect_rc"], rc))
        elif not quiet:
            print("   ✓ 反例 %s 被抓住（rc=%d）· %s" % (rv["id"], rc, rv["what"]))
    return bad, n


def self_test(fixdir):
    ok = bad = 0
    print("── 自检 ①/③ 正控：夹具按预期（能合 rc=0 / 合不了 rc=1 / 缺证据 rc=2）")
    man, why = load_manifest(fixdir)
    if man is None:
        print("   ✗ ① 夹具清单读不到：%s ⇒ 不给结论（rc=2）" % why)
        return 2
    faults, n = [], 0
    for case in man.get("cases", []):
        n += 1
        rcj, outj, _errj = run_cli(["--no-self-test", "--json"] + fixture_argv(case, fixdir))
        dup, _o, _e = run_cli(["--no-self-test"] + fixture_argv(case, fixdir))
        view = json_from(outj)
        if not view:
            faults.append("[%s] 机器面取不到（rc=%d）" % (case["id"], rcj))
        msg = case_ok(case, {"rc": dup, "rows": view.get("rows", []), "view": view})
        if dup != rcj:
            msg = msg or "[%s] 人读面 rc=%d ≠ 机器面 rc=%d（两面不许不同形）" % (case["id"], dup, rcj)
        if msg:
            faults.append("[%s] %s" % (case["id"], msg))
            print("   ✗ %s：%s" % (case["id"], msg))
        else:
            print("   ✓ %s：rc=%d · %d 行 · %s" % (case["id"], dup, len(view.get("rows", [])), case["why"]))
            ok += 1
    if faults:
        bad += len(faults)

    print("── 自检 ② 负控：反例产出（错实现把缺的来源机格填成 本机/localhost/拿 default_node 兜底）必须**红**")
    for rv in man.get("rejects", []):
        p = os.path.join(fixdir, rv["path"])
        rc, out, err = run_cli(["--no-self-test", "--audit-view", p])
        if rc == rv["expect_rc"]:
            print("   ✓ 抓住 %s（rc=%d）：%s" % (rv["id"], rc, rv["what"]))
            ok += 1
        else:
            print("   ✗ %s：该判 rc=%d · 实读 %d ⇒ 判据没牙" % (rv["id"], rv["expect_rc"], rc))
            bad += 1

    print("── 自检 ③ 变异（设计件 A5 末句：对它做变异——写死 Mr2109 ⇒ 期望红）")
    muts = [("写死 `本机`", {"fill_missing": "本机"}, "anon", None),
            ("写死 `localhost`", {"fill_missing": "localhost"}, "anon", None),
            ("拿 `default_node` 兜底（值恰在册 ⇒ 更毒 · 只有依据格露馅）",
             {"fill_missing": "Mr2109", "src_basis": "default_node"}, "anon", None),
            ("写死 `Mr2109`（无视载体 ⇒ §三.4 负控②「别机那一行必须红」）",
             {"hardcode": "Mr2109"}, "prefixed", "edge-node")]
    for name, pol, which, pre in muts:
        if which == "anon":
            d = os.path.join(fixdir, "A5-neg-not-machine", "sources", "anon-edit_audit.jsonl")
        else:
            d = os.path.join(fixdir, "A0-ok", "sources", "edge-node-edit_audit.jsonl")
        srcs = [{"arg": d, "prefix": pre, "path": d}]
        rows, why = build_rows(srcs, ["Mr2109", "edge-node"], None, "<host>", policy=pol)
        if rows is None:
            print("   ✗ 变异 %s：取数失败（%s）" % (name, why))
            bad += 1
            continue
        red, _ = audit_view(build_view(rows, srcs, "<host>"), ["Mr2109", "edge-node"])
        if red:
            print("   ✓ 变异 %s ⇒ 自己的 A5 检索面判红：%s" % (name, red[0][:72]))
            ok += 1
        else:
            print("   ✗ 变异 %s ⇒ 检索面**没红**（判据没牙）" % name)
            bad += 1

    # ★ 明标缺口（**不计数** · 不许当成已封）：写死来源机**且同时声称 `argv` 依据** ⇒ 在产出面上
    #   与合法的 `argv` 显式传参**不可分**（载体是命令行、不在产出面里）。这一格只有
    #   ① 载体为 cell/prefix 的行（上一格）② 变异自检 抓得住 —— 明写出来，免得被读成「已防住」。
    d0 = os.path.join(fixdir, "A5-neg-not-machine", "sources", "anon-edit_audit.jsonl")
    rows0, _ = build_rows([{"arg": d0, "prefix": None, "path": d0}], ["Mr2109", "edge-node"], None,
                          "<host>", policy={"hardcode": "Mr2109"})
    red0, _ = audit_view(build_view(rows0, [], "<host>"), ["Mr2109", "edge-node"])
    print("── 自检 ③ 明标缺口（**不计数** · 这一格今天没牙，不许读成已封）：")
    print("   · 写死 `Mr2109` **且同时声称 `src_basis=argv`** ⇒ 产出面%s —— 载体是命令行、不在产出面里 ⇒"
          "只有「载体为 cell/prefix 的行」与变异自检抓得住（待拍：要不要把 argv 也落进产出面）"
          % ("仍判红（说明还有别的牙）" if red0 else "**判不出来**（如实标出）"))

    print("── 自检 ③/③ 用法错（两态不许同形：`用法错` ≠ `缺证据`）")
    rc1, out1, err1 = run_cli(["--no-self-test", "--not-a-flag"])
    out1 = out1 + err1
    if rc1 == 2 and "用法错" in out1:
        print("   ✓ 未知旗标 ⇒ rc=2 且字样 = 「用法错」")
        ok += 1
    else:
        print("   ✗ 未知旗标 ⇒ rc=%d（期望 2 + 字样「用法错」）" % rc1)
        bad += 1
    rc2, out2, err2 = run_cli(["--no-self-test", "--source", "/no/such/source.jsonl",
                               "--roster-nodes", "Mr2109,edge-node"])
    out2 = out2 + err2
    if rc2 == 2 and "用法错" not in out2 and ("未取数" in out2 or "缺证据" in out2):
        print("   ✓ 来源件不在 ⇒ rc=2 且字样 ≠「用法错」（未取数那一档）")
        ok += 1
    else:
        print("   ✗ 来源件不在 ⇒ rc=%d／字样不对" % rc2)
        bad += 1
    rc3, out3, err3 = run_cli(["--no-self-test", "--source", "/no/such/source.jsonl",
                               "--roster", "/no/such/roster.yaml"])
    out3 = out3 + err3
    if rc3 == 2 and "名册" in out3:
        print("   ✓ 名册读不到 ⇒ rc=2（**缺证据不编造**）")
        ok += 1
    else:
        print("   ✗ 名册读不到 ⇒ rc=%d／没明说名册（不许当绿）" % rc3)
        bad += 1

    print("   自检：%d 格过 · %d 格不过（夹具 %d 案 · 只读 · 真状态目录一个字节未动）" % (ok, bad, n))
    return 0 if bad == 0 else 2


# ══════════════════════════════════════════════════════════════════════════════
# 五 · 真目标（只读扫）／产出面审计／主
# ══════════════════════════════════════════════════════════════════════════════

def default_sources(state):
    out = [{"arg": os.path.join(state, "edit_audit.jsonl"), "prefix": None,
            "path": os.path.join(state, "edit_audit.jsonl")}]
    d = os.path.join(state, "approvals")
    if os.path.isdir(d):
        for n in sorted(os.listdir(d)):
            if n.endswith(".json"):
                p = os.path.join(d, n)
                out.append({"arg": p, "prefix": None, "path": p})
    return out


def show(view, rows, roster, hostname, json_out):
    if json_out:
        print(json.dumps(view, ensure_ascii=False, indent=1))
        return True
    print("── 第三本（人签跨机合并视图 · 只读汇总）")
    for h in HEADERS:
        print("   · 件头：%s" % h)
    print("   · 强度：%s" % STRENGTH_NOTE)
    print("   · 名册键集（%d）：%s —— 来源机取值的**唯一合法来源**" % (len(roster), " · ".join(roster)))
    print("── 来源面（%d 处 · 只读快照）" % len(view["sources"]))
    for s in view["sources"]:
        print("   · %s · 认到 %d 行 · sha256 %s" % (s["source"], s["rows"], (s["sha256"] or "—")[:12]))
    print("── 逐行（%d 行 · 三态）" % len(rows))
    for i, r in enumerate(rows, 1):
        m = r.get("src_machine", "（这一格省掉 ✓）")
        print("   %d) %s · tool=%s · approver=%s · at=%s · key=%s"
              % (i, r["row_state"], r["tool"] or "—", r["approver"] or "—",
                 r["approved_at"] or "—", r["key_id"] or "—"))
        print("      来源机=%s · known=%s · 依据=%s · 钟=%s"
              % (m, r["src_machine_known"], r["src_basis"] or "—", r["clock_basis"]))
        print("      回指=%s%s" % (r["src_path"],
              (" · 缺格=" + ",".join(r["missing_cells"])) if r["missing_cells"] else ""))
        if r["conflict_with"]:
            print("      冲突对端：%s（冲突未裁 · 两面并列 · 不选边）" % " | ".join(r["conflict_with"]))
        for n in r["notes"]:
            print("      ·  %s" % n)
    return False


def scan(sources, roster, machine_argv, hostname, json_out, roster_src):
    rows, why = build_rows(sources, roster, machine_argv, hostname)
    if rows is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 缺证据不编造）" % why, file=sys.stderr)
        return 2
    view = build_view(rows, sources, hostname)
    red, notes = audit_view(view, roster)          # ★ 先自审**自己的产出面**（A5 落在本件身上）
    if red:
        print("⛔ 产出面**违规**（本笔自己的实现坏了 ⇒ 红 · **机器面也不放出去**）：", file=sys.stderr)
        for x in red:
            print("   ✗ %s" % x, file=sys.stderr)
        return 1
    rc = judge_rc(rows)
    if json_out:
        # 机器面 = **纯 JSON 落 stdout 且是最后一段**（调用方不许拿人读串去切）；人读结论走 stderr
        print("── 人读结论（机器面在 stdout 末段 · schema %s）rc=%d · %d 行 · 名册 %d 键"
              % (VIEW_SCHEMA, rc, len(rows), len(roster)), file=sys.stderr)
        for n in notes:
            print("   ·  %s" % n, file=sys.stderr)
        print(json.dumps(view, ensure_ascii=False, indent=1))
        return rc
    show(view, rows, roster, hostname, False)
    print()
    if rc == 0:
        print("⇒ 能合（rc=0）：%d 行必填格齐 · 无冲突 · 来源机全在名册（本件只读 · 未写一个字节）" % len(rows))
    elif rc == 1:
        print("⇒ **红**（rc=1）：合不了 —— 冲突**必须由人裁**、不许静默取一（两面并列 · 不选边）")
    else:
        print("⇒ **不给结论**（rc=2）：有「缺证据」的行 ⇒ **这一格不许读成绿**"
              "（缺格 = %s）" % " / ".join("、".join(r["missing_cells"]) for r in rows if r["missing_cells"]))
    for n in notes:
        print("   ·  %s" % n)
    print("   · 本笔**不做验签** ⇒ `sig_state` 一律「未验·…」· **不许**读成「签名坏」")
    for p in OPEN_POINTS:
        print("   · 待拍：%s" % p)
    return rc


def audit_only(path, roster):
    if not os.path.exists(os.path.expanduser(path)):
        print("⛔ 未取数：产出面不在 %s ⇒ 不给结论（rc=2）" % path, file=sys.stderr)
        return 2
    with open(os.path.expanduser(path), encoding="utf-8") as f:
        try:
            view = json.load(f)
        except json.JSONDecodeError as e:
            print("⛔ 未取数：产出面解不出来（%s）⇒ 不给结论（rc=2）" % e, file=sys.stderr)
            return 2
    red, notes = audit_view(view, roster)
    print("── 产出面审计（A5：产出面「本机」当值 = 0 处 · 15 格闭集）")
    print("   件：%s · schema=%s · 行数=%s" % (path, view.get("schema"), len(view.get("rows") or [])))
    for x in red:
        print("   ✗ %s" % x)
    for n in notes:
        print("   ·  %s" % n)
    if red:
        print("⇒ **红**（rc=1）：产出面把负数填成了本机值 / 或走了不该走的兜底 ⇒ 这就是「不许默认本机」的机器形态")
        return 1
    print("⇒ 产出面干净（rc=0）：值里 0 处本机字面 · 机器面格的依据都在 §二.2 的三个合法来源里")
    return 0


def main():
    global REPO_ROOT  # `--root` 生效用（缺口 GAP-20261001-63）：仅显式给 --root 时重钉
    argv = list(sys.argv[1:])
    known = {"--self-test", "--no-self-test", "--json", "--fixtures", "--audit-view",
             "--source", "--machine", "--roster", "--roster-nodes", "--root", "--state", "-h", "--help"}
    bad_flags = [a for a in argv if a.startswith("-") and a.split("=")[0] not in known]
    if bad_flags:
        print("⛔ 用法错（rc=2）：未知旗标 %s" % " · ".join("`%s`" % x for x in bad_flags), file=sys.stderr)
        print("   见 --help；**用法错 ≠ 缺证据**（两态不许同形）", file=sys.stderr)
        return 2
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=REPO_ROOT)
    ap.add_argument("--state", default=os.path.join(os.path.expanduser("~"), ".zerg", "state"))
    ap.add_argument("--roster", default=os.path.join(os.path.expanduser("~"), ".zerg", "contexts", "default.yaml"))
    ap.add_argument("--roster-nodes", default="", help="名册键集（逗号分隔 · 夹具用 ⇒ 零依赖）")
    ap.add_argument("--source", action="append", default=[])
    ap.add_argument("--machine", default="")
    ap.add_argument("--fixtures", default=FIXTURES_DEFAULT)
    ap.add_argument("--fixture-only", action="store_true")
    ap.add_argument("--audit-view", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    args = ap.parse_args(argv)

    # ── `--root` 生效（缺口 `GAP-20261001-63`）：原实现解析后**从不读取** ⇒ 静默无效 ──
    #   显式给了 `--root` ⇒ 以它重钉模块级 `REPO_ROOT`（run_cli 的 cwd 等随之生效）；
    #   不给时 args.root == REPO_ROOT ⇒ 逐字不改既有行为（默认路径未动）。
    if any(a == "--root" or a.startswith("--root=") for a in argv):
        REPO_ROOT = os.path.abspath(args.root)
    print(VERSION)
    hostname = os.uname().nodename
    if args.audit_view:
        roster, why = read_roster(args.roster, args.roster_nodes)
        if roster is None:
            print("⛔ 未取数：%s ⇒ 不给结论（rc=2）" % why, file=sys.stderr)
            return 2
        return audit_only(args.audit_view, roster)
    if args.self_test:
        return self_test(args.fixtures)
    if args.fixture_only:
        bad, n = run_fixtures(args.fixtures)
        print("   夹具：%d 案 · 不过 %d 条" % (n, len(bad)))
        return 0 if not bad else 2
    if not args.no_self_test:
        rc = self_test(args.fixtures)
        if rc != 0:
            print("⛔ 自检不过 ⇒ 不给结论（rc=2）")
            return 2
    roster, why = read_roster(args.roster, args.roster_nodes)
    if roster is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 缺证据不编造）" % why, file=sys.stderr)
        return 2
    if args.source:
        srcs = []
        for a in args.source:
            pre, p = split_source_arg(a)
            srcs.append({"arg": a, "prefix": pre, "path": p})
        print("── 真目标：人喂进来的 %d 处快照 · 名册 %d 键 · 只读" % (len(srcs), len(roster)))
    else:
        srcs = default_sources(args.state)
        print("── 真目标：本机状态目录 %s（%d 处 · 只读）" % (args.state, len(srcs)))
        print("   （现读口径：审计行含机器名格的 = 0 行 ⇒ 来源机格缺 ⇒ **今天必然 rc=2**，"
              "不许读成绿 —— 设计件 §1.3/A2）")
    return scan(srcs, roster, args.machine or None, hostname, args.json,
                args.roster if not args.roster_nodes else "argv")


if __name__ == "__main__":
    sys.exit(main())

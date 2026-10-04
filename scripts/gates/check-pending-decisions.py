#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-pending-decisions.py —— 作业单「**待拍 / 已拍**」的闭集值域 + 条数（`Q-066`）

真源与口径（**判据绑私有面** · 逐字引文全指向仓外 `Zerg-内部文档/`）
------------------------------------------------------------
· 来源条（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/任务清单-承接落地-组3-上版缺口账-20260924.md`（⚠ 该件 2026-09-28 令删、仓外归档 Zerg-内部文档-archive-20260928/缺口账旧稿/，**本仓无在树副本** ⇒ 保留原引文记当时取源·改史 ✗；已入 `dangling-refs-registry.json` 登记
  序 48（`Q-066`）—— 「作业单里「待拍／已拍」是自由文本 ⇒ **数不出**」
· 施工条（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md`
  §一 序 82 —— 判据栏 = 「「待拍 / 已拍」有**闭集值域** + **一条命令给条数**
  （**行命中 ≠ 条命中**）」
· 作业单（现读对拍的真源）：同仓 `项目文档/v2.5.12/待拍清单-20260924.md`
· 登记面（本波新件 · 本门 `--register` 读它）：
  `Zerg-内部文档/项目文档/v2.5.12/登记-待拍已拍闭集-v2.5.12-20260924.md`

本件补的那一格（**病根 = 「数不出」**）
------------------------------------
作业单的「态」今天写在**处置 / 我的推荐**那一格里，是**手写自由文本**（「已入账 ✓（`Q-159`）」·
「需你授权」·「本版已修 ✓」…）⇒ ① 没有任何机器在读数；② 更隐蔽的一格：拿 `grep -c 待拍` 当条数 ——
**行命中 ≠ 条命中**（同一条里出现两次关键词、或关键词出现在散文行里 ⇒ 数出来的不是条数）。
本门把这两条各落成一条**可现跑**的判据：

  ① **闭集值域**：登记面每一条的「拍板态」∈ 闭集 `已拍 / 已入账 / 待拍`（口径逐字写在登记面里：
     有 Mr2109 拍板原话 ⇒ `已拍`；只到「已入账 ✓（`Q-nnn`）」这一档 ⇒ `已入账`；处置栏仍是
     推荐 / 需授权 / 未收 ⇒ `待拍`）。值域外 ⇒ `rc=1` + 逐条点名「登记件:行 + 原样值」。
  ② **条数（一条命令给）**：本门现算并打印 `条 N：已拍 a · 已入账 b · 待拍 c` ——
     计数单位是**条**（作业单里的**表行** + 登记面里的**条号**），**不是行**；
     同跑并把「**行命中**」（`待拍|已拍` 的**行数**）**另打一格作对照** ⇒ 两个数**分开给**，
     谁都不许冒充条数（负控：往作业单里加几行关键词 ⇒ 行命中变、**条数不变**）。
  ③ **依据可核**：登记面每条必须带一句**逐字引文**，它在该作业单里 `grep -F` **恰好命中 1 处**
     （借用内容锚口径 · `R-17` 同族）⇒ 0 命中 / 多命中 ⇒ `rc=1` + 点名。
  ④ **两组数对拍**（照实收下 · 不是放宽）：作业单头部声明的「**13 条**」`==` §一–§四 的现读表行数；
     §〇 标题声明的「四条」`==` §〇 的现读表行数；登记面条数 `==` 作业单条数；两边**条号集合**逐条对得上。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 值域内 + 条数与条号集合逐条对拍 + 每条依据恰好命中 1 处 + 登记件与现读一致
  1 = 值域外（逐条点名）/ 条不对拍（缺条 / 多条 / 条号集合不合）/ 依据 0 命中或多命中 /
      作业单的声明数与现读不符
  2 = **不给结论**：作业单不在盘 / 登记件不在盘 / 登记面表头逐字句找不到 / 作业单 0 条（空转）/
      登记件里缺现读计数句 / 自检不过 —— 一律显式打印，不许静默当绿

用法
----
    python3 scripts/gates/check-pending-decisions.py                 # 判（只读作业单 + 登记件）
    python3 scripts/gates/check-pending-decisions.py --json          # 机读（zerg/v1 形状）
    python3 scripts/gates/check-pending-decisions.py --counts        # 只打条数（人面两行）
    python3 scripts/gates/check-pending-decisions.py --docs-root DIR # 换文档仓根（默认真源：主仓的兄弟目录）
    python3 scripts/gates/check-pending-decisions.py --list-rules
    python3 scripts/gates/check-pending-decisions.py --self-test     # 合成夹具（不碰真作业单）

★ 本笔**已挂门**（`gates` scope · `tri` 阻断 · 在默认集 · **不进 `FAST_STEPS`** —— 要读仓外
  `Zerg-内部文档/` 那两件 ⇒ 别的机器上没有那棵树时 `rc=2`（缺件不给结论）⇒ 不普遍可用 · 同门㉑/㉕/㉟/㊱ 口径）。
"""

import argparse
import collections
import contextlib
import io
import json
import os
import re
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-pending-decisions.py v1.0.0"

SOURCE_REL = "项目文档/v2.5.12/待拍清单-20260924.md"
REGISTER_REL = "项目文档/v2.5.12/登记-待拍已拍闭集-v2.5.12-20260924.md"

# ── 闭集（机器可读的唯一真源；成文面 = 仓外登记件，本门现跑对拍它）──────────────────────
DECISION_CLOSED = ("已拍", "已入账", "待拍")
DECISION_COL = 3          # 0 基：`| 条号 | 来源 | 事项 | 拍板态 | 依据 |`
ANCHOR_COL = 4
REGISTER_HEADER = "| 条号 | 来源（表 · 号） | 事项（一句） | 拍板态 | 依据（逐字引一句 · 内容锚） |"

# 作业单头部声明的条数（照实收下、现读对拍）：`**13 条**` + §〇 标题的「新出四条」
HEAD_DECL_RE = re.compile(r"\*\*(\d+)\s*条\*\*")
SEC0_DECL_RE = re.compile(r"新出(.{1,3}?)条")
CN_NUM = {"一": 1, "两": 2, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10}
# 「行命中」（**只作对照** · 不作条数）：作业单里含这些字样的**行数**
LINE_HIT_RE = re.compile(r"待拍|已拍")
TID_COL = 0               # 作业单表头首格逐字 = `#` 的才是「条表」


def cells(line):
    """把一行 markdown 表行切成单元；`\\|`（转义竖线）**不**算分隔符。"""
    s = line.strip()
    if not (s.startswith("|") and s.endswith("|")):
        return None
    return [c.strip() for c in re.split(r"(?<!\\)\|", s[1:-1])]


def is_sep(cs):
    return bool(cs) and set("".join(cs)) <= set("-: ")


def read_source_tables(text):
    """返条表清单：`[{sec, line, header, rows:[[cells],…]}]`（sec = 最近一条标题行）。"""
    lines = text.split("\n")
    out, i, sec = [], 0, ""
    while i < len(lines):
        raw = lines[i]
        if raw.lstrip().startswith("#"):
            sec = raw.lstrip("#").strip()
            i += 1
            continue
        cs = cells(raw)
        if cs is not None:
            blk, j = [], i
            while j < len(lines) and cells(lines[j]) is not None:
                blk.append((j, cells(lines[j])))
                j += 1
            if len(blk) >= 2 and is_sep(blk[1][1]) and blk[0][1] and blk[0][1][TID_COL] == "#":
                rows = [{"line": ln, "cells": c} for ln, c in blk[2:]]
                out.append({"sec": sec, "line": blk[0][0] + 1, "header": blk[0][1],
                            "rows": rows})
            i = j
            continue
        i += 1
    return out


def norm_value(raw):
    """「拍板态」的归一值：去粗体 / 行内代码标记 + 取到第一个分隔符之前（同门㉟ 口径）。"""
    s = re.sub(r"[`*]", "", raw.strip())
    for cut in ("：", "（", "(", "：", " ", "✅", "·"):
        p = s.find(cut)
        if p > 0:
            s = s[:p]
    return s.strip()


def read_register(path):
    """读登记面：返 (条清单, 问题清单, 闭集声明句在不在)。"""
    if not os.path.isfile(path):
        return None, ["登记件不在盘：%s" % path], False
    with open(path, encoding="utf-8") as f:
        txt = f.read()
    lines = txt.split("\n")
    occ = [i for i, l in enumerate(lines) if l.strip() == REGISTER_HEADER]
    if not occ:
        return None, ["登记件的表头逐字句找不到（`%s`）" % REGISTER_HEADER], False
    ncol = len(cells(REGISTER_HEADER))
    rows = []
    for o in occ:
        for i in range(o + 2, len(lines)):
            cs = cells(lines[i])
            if cs is None or len(cs) != ncol:
                break
            if is_sep(cs):
                continue
            rows.append({"line": i + 1, "tid": cs[0], "raw_decision": cs[DECISION_COL],
                         "decision": norm_value(cs[DECISION_COL]), "anchor": cs[ANCHOR_COL]})
    decl = all(("`%s`" % v) in txt or v in txt for v in DECISION_CLOSED)
    return rows, [], decl


def count_line_hits(text):
    """行命中（**对照用**）：含 `待拍|已拍` 字样的**行数** —— 它不是条数。"""
    return sum(1 for l in text.split("\n") if LINE_HIT_RE.search(l))


def run(root, docs_root, register_arg, json_out, counts_only=False):
    root = os.path.abspath(root)
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根" % root)
        return 2
    docs = os.path.abspath(docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档"))
    src = os.path.join(docs, SOURCE_REL)
    reg = register_arg or os.path.join(docs, REGISTER_REL)
    problems, bad, notes = [], [], []

    if not os.path.isfile(src):
        print("⛔ 不给结论（rc=2）：作业单不在盘：`%s`（不拿「读不到」当「没问题」）" % src)
        return 2
    with open(src, encoding="utf-8") as f:
        stext = f.read()
    tables = read_source_tables(stext)
    if not tables:
        print("⛔ 不给结论（rc=2）：作业单里找不到「条表」（表头首格逐字 `#` 的 markdown 表）")
        return 2
    src_tids = []
    for t in tables:
        for r in t["rows"]:
            src_tids.append({"sec": t["sec"], "line": r["line"], "tid": r["cells"][TID_COL]})
    if not src_tids:
        print("⛔ 不给结论（rc=2）：作业单的条表 0 行（**空转 ≠ 绿**）")
        return 2
    rows, reg_problems, decl_ok = read_register(reg)
    if rows is None:
        print("⛔ 不给结论（rc=2）：%s" % "; ".join(reg_problems))
        return 2

    # ── ② 条数对拍（**条命中**；行命中只作对照）────────────────────────────────
    head_decl = HEAD_DECL_RE.search(stext)
    head_decl = int(head_decl.group(1)) if head_decl else None
    sec0 = [t for t in tables if "本轮" in t["sec"]]
    rest = [t for t in tables if "本轮" not in t["sec"]]
    n_sec0 = sum(len(t["rows"]) for t in sec0)
    n_rest = sum(len(t["rows"]) for t in rest)
    m0 = SEC0_DECL_RE.search(stext)
    sec0_decl = CN_NUM.get(m0.group(1)) if m0 else None
    if head_decl is None:
        problems.append({"what": "head_decl_absent", "msg": "作业单头部找不到「**N 条**」声明句"})
    elif head_decl != n_rest:
        bad.append({"kind": "decl", "msg": "作业单头部声明「**%d 条**」≠ §一–§四 现读表行数 %d"
                                           "（数字声明口径：一律以现跑现印为准）" % (head_decl, n_rest)})
    if sec0 and (sec0_decl is None or sec0_decl != n_sec0):
        bad.append({"kind": "decl", "msg": "§〇 标题声明「新出%s条」≠ 现读表行数 %d"
                                           % (m0.group(1) if m0 else "?", n_sec0)})

    # 登记面 ↔ 作业单：条数 + 条号集合
    if len(rows) != len(src_tids):
        bad.append({"kind": "count", "msg": "登记面条数 %d ≠ 作业单条数 %d（**条命中**：两边都按表行 / 条号算，"
                                            "不是 `grep -c 待拍` 的行命中）" % (len(rows), len(src_tids))})
    s_set = collections.Counter(t["tid"] for t in src_tids)
    r_set = collections.Counter(r["tid"] for r in rows)
    for tid in sorted(set(s_set) | set(r_set)):
        if s_set[tid] != r_set[tid]:
            bad.append({"kind": "tid", "msg": "条号 `%s`：作业单 %d 条 ⟷ 登记面 %d 条（缺条 / 多条都要点名）"
                                              % (tid, s_set[tid], r_set[tid])})

    # ── ① 值域 + ③ 依据引文（恰好 1 处）──────────────────────────────────────
    for r in rows:
        if r["decision"] not in DECISION_CLOSED:
            bad.append({"kind": "domain", "file": os.path.basename(reg), "line": r["line"],
                        "value": r["decision"], "raw": r["raw_decision"]})
        if not r["anchor"].strip():
            bad.append({"kind": "anchor_empty", "file": os.path.basename(reg), "line": r["line"]})
        else:
            n = stext.count(r["anchor"])
            if n != 1:
                bad.append({"kind": "anchor", "file": os.path.basename(reg), "line": r["line"],
                            "anchor": r["anchor"], "hits": n})

    # ── ④ 计数句（现读逐字）+ 闭集声明句：登记件与现读一致 ────────────────────────
    cnt = collections.Counter(r["decision"] for r in rows)
    parts = " · ".join("%s %d" % (v, cnt.get(v, 0)) for v in DECISION_CLOSED)
    count_line = "条 %d：%s" % (len(rows), parts)
    line_hits = count_line_hits(stext)
    with open(reg, encoding="utf-8") as f:
        rtext = f.read()
    if count_line not in rtext:
        problems.append({"what": "register_counts_stale", "msg": "登记件里缺现读计数句「%s」" % count_line})
    if not decl_ok:
        problems.append({"what": "register_closed_absent", "msg": "登记件里缺闭集声明（三个值逐字都要在）"})

    if json_out:
        doc = {"schema": "zerg/v1", "kind": "PendingDecisions", "tool": VERSION,
               "source": src, "register": reg,
               "items": [{"tid": r["tid"], "decision": r["decision"], "anchor": r["anchor"],
                          "line": r["line"]} for r in rows],
               "meta": {"selftest": False, "docs_root": docs},
               "counts": {"items": len(rows), "by_decision": dict(cnt),
                          "line_hits": line_hits, "source_rows": len(src_tids),
                          "head_decl": head_decl, "sec0_rows": n_sec0, "sec0_decl": sec0_decl},
               "count_line": count_line, "out_of_domain": bad, "problems": problems,
               "warnings": [], "truncated": False}
        print(json.dumps(doc, ensure_ascii=False))
        if problems:
            return 2
        return 1 if bad else 0

    if counts_only:
        print(count_line)
        print("行命中 %d（对照：含「待拍|已拍」的**行**数 —— 不是条数）" % line_hits)
        return 1 if bad else (2 if problems else 0)

    print("── 作业单「待拍 / 已拍」闭集值域 + 条数（%s · `Q-066`）──" % VERSION)
    print("  作业单  : %s" % src)
    print("  登记面  : %s" % reg)
    print("  闭集    : %s" % " / ".join(DECISION_CLOSED))
    for t in tables:
        print("  %-28s 表头 `%s` · 数据行 %d" % (t["sec"][:28], "|".join(t["header"])[:40], len(t["rows"])))
    print("  ── 表行合计（**条命中**）: %d（§〇 %d + §一–§四 %d）" % (len(src_tids), n_sec0, n_rest))
    print("  ── 声明对拍: 头部「**%s 条**」⟷ §一–§四 现读 %d ｜ §〇 标题「新出%s条」⟷ 现读 %d"
          % (head_decl, n_rest, m0.group(1) if m0 else "?", n_sec0))
    print("  ── %s" % count_line)
    print("  ── 行命中 %d（**对照格**：含「待拍|已拍」的**行**数 —— 它**不是**条数；"
          "本门两个数分开给）" % line_hits)

    if problems:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · %s %s" % (p.get("what"), p.get("msg", "")))
        return 2
    if bad:
        print("✗ 共 %d 处（rc=1 · 逐条点名）：" % len(bad))
        for b in bad:
            if b["kind"] == "domain":
                print("   · %s:%d **值域外** `%s` ⟵ 原样 `%s`" % (b["file"], b["line"], b["value"], b["raw"][:60]))
            elif b["kind"] == "anchor":
                print("   · %s:%d **依据引文命中 %d 处**（要**恰好 1 处**）`%s`"
                      % (b["file"], b["line"], b["hits"], b["anchor"][:60]))
            elif b["kind"] == "anchor_empty":
                print("   · %s:%d **缺依据引文**（每条都要一句可 `grep -F` 的逐字引文）" % (b["file"], b["line"]))
            else:
                print("   · %s" % b["msg"])
        print("⇒ 处置：值域要扩 ⇒ ① 改本件 `DECISION_CLOSED` ② **同笔**改仓外登记件的闭集声明；"
              "条数不符 ⇒ 改登记件（补缺的条 / 删多的条）—— 两处**同笔**，否则下一次跑还会红。")
        return 1
    print("⇒ 全在位（rc=0）：值域内 · 条数与条号集合逐条对拍 · 每条依据恰好命中 1 处 · 登记件与现读一致")
    print("   ★ 本件只**读**作业单与登记面：两件一个字未改（登记面是本波**新件** ⇒ 只增）")
    return 0


def list_rules():
    print("── %s 判据与登记面 ──" % VERSION)
    print("  作业单  : %s" % SOURCE_REL)
    print("  登记面  : %s" % REGISTER_REL)
    print("  表头逐字: %s" % REGISTER_HEADER)
    print("  闭集    : %s（第 %d 列 · 0 基）" % (" / ".join(DECISION_CLOSED), DECISION_COL))
    print("  依据    : 第 %d 列 —— 每条一句逐字引文，在作业单里 `grep -F` **恰好 1 处**" % ANCHOR_COL)
    print("  条数    : **条命中**（作业单表行 + 登记面条号）· 另给「行命中」作**对照**（同门㉟ 的「计数句」口径）")
    print("  退码    : 0 全在位 · 1 值域外 / 条不对拍 / 依据不是唯一命中 / 声明数与现读不符 · 2 不给结论")
    print("★ 已挂门：`gates` scope · `tri` 阻断 · 在默认集 · 不进 `FAST_STEPS`（要读仓外 `Zerg-内部文档/`）。")
    return 0


# ───────────────────────────── 自检（正控 + 成对负控 · 合成夹具 · 不碰真作业单）─────────────────────
FIX_SRC = """# 合成作业单 · vX · 2026-09-24

> **性质**：合成夹具。**2 条**，每条带我的推荐。

## 〇 · 本轮新出两条（A/B **已入账 ✓**）

| # | 事项 | 现读实况 | 处置 |
|---|---|---|---|
| A | 甲事 | 现况一 | 已入账 ✓（`Q-901`） |
| B | 乙事 | 现况二 | 需你授权 |

## 一 · 机器读面

| # | 事项 | 现读实况 | 我的推荐 |
|---|---|---|---|
| 1 | 丙事 | 现况三 | 乙 · 2min ✓ |
| 2 | 丁事 | 现况四 | 不拆 ✓ |

## 二 · 尾巴
一行散文：这里提到已拍、待拍两个词（**散文行不算条**）。
"""

# 夹具的**依据引文**（每条一句 · 在 FIX_SRC 里恰好命中 1 处）
FIX_ANCHORS = {"A": "已入账 ✓（`Q-901`）", "B": "需你授权", "1": "乙 · 2min ✓", "2": "不拆 ✓"}
FIX_ROWS = (("A", "已入账"), ("B", "待拍"), ("1", "待拍"), ("2", "待拍"))


def build_register(rows, closed=None):
    """造一份合成登记面：闭集声明 + **现读计数句** + 表（条号 / 来源 / 事项 / 拍板态 / 依据）。"""
    closed = tuple(closed or DECISION_CLOSED)
    cnt = collections.Counter(d for _t, d in rows)
    parts = " · ".join("%s %d" % (v, cnt.get(v, 0)) for v in closed)
    body = "".join("| %s | § | 事 | %s | %s |\n" % (t, d, FIX_ANCHORS.get(t, FIX_ANCHORS["A"]))
                   for t, d in rows)
    return ("# 登记 · 合成夹具闭集\n\n> 闭集：%s（值域外 ⇒ rc=1 点名）。\n> 计数句：条 %d：%s\n\n%s\n|---|---|---|---|---|\n%s"
            % (" / ".join("`%s`" % v for v in closed), len(rows), parts, REGISTER_HEADER, body))


def mk_fixture(tmp, name, src=FIX_SRC, rows=FIX_ROWS, closed=None, reg_text=None):
    """合成夹具：`<tmp>/<name>/`（假仓）+ `<tmp>/<name>-docs/`（假文档仓）。"""
    root = os.path.join(tmp, name)
    docs = os.path.join(tmp, name + "-docs")
    os.makedirs(os.path.join(root, "scripts", "gates"), exist_ok=True)
    os.makedirs(os.path.join(docs, os.path.dirname(SOURCE_REL)), exist_ok=True)
    with open(os.path.join(docs, SOURCE_REL), "w", encoding="utf-8") as f:
        f.write(src)
    reg = reg_text if reg_text is not None else build_register(rows, closed)
    rp = os.path.join(docs, REGISTER_REL)
    with open(rp, "w", encoding="utf-8") as f:
        f.write(reg)
    return root, docs, rp


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="pd-selftest-")

    def case(name, want, got, extra_ok=True, note=""):
        nonlocal ok
        good = (want == got) and extra_ok
        ok = ok and good
        print("  %s %s（rc=%s · 期望 %s）%s" % ("✓" if good else "✗", name, got, want, note))

    def cap(*a, **kw):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc = run(*a, **kw)
        return rc, buf.getvalue()

    # ① 正控：值域内 + 条对拍 + 依据唯一 ⇒ rc=0，且条数逐字打出来
    r, d, reg = mk_fixture(tmp, "case-1")
    rc, out = cap(r, d, reg, False)
    case("① 正控：4 条全在闭集内 + 条号对拍 + 依据唯一 ⇒ rc=0", 0, rc, "全在位" in out)
    case("①b 条数逐字（**条命中**）：条 4：已拍 0 · 已入账 1 · 待拍 3",
         True, "条 4：已拍 0 · 已入账 1 · 待拍 3" in out)

    # ② 负控：登记件某条「拍板态」= `已派`（那**不是**本闭集的值）⇒ rc=1 + 点名到「件:行 + 原样值」
    r2, d2, reg2 = mk_fixture(tmp, "case-2", rows=(("A", "已入账"), ("B", "已派"), ("1", "待拍"), ("2", "待拍")))
    rc, out = cap(r2, d2, reg2, False)
    case("② 负控：值域外 `已派` ⇒ rc=1", 1, rc, "值域外" in out)
    case("②b 负控点名到「登记件:行 + 原样值」", True,
         ("登记-待拍已拍闭集-v2.5.12-20260924.md:" in out and "`已派`" in out))

    # ②c 真话负控：闭集同笔扩到含 `已派` ⇒ 同一份夹具回绿（红是红在那**一个值**上）
    saved = DECISION_CLOSED
    globals()["DECISION_CLOSED"] = ("已拍", "已入账", "待拍", "已派")
    r2c, d2c, reg2c = mk_fixture(tmp, "case-2c", rows=(("A", "已入账"), ("B", "已派"),
                                                       ("1", "待拍"), ("2", "待拍")))
    rc, out = cap(r2c, d2c, reg2c, False)
    case("②c 真话负控（同一夹具 · 闭集扩到含它）⇒ 回绿", 0, rc, "全在位" in out)
    globals()["DECISION_CLOSED"] = saved

    # ③ 负控：缺一条（源 4 条、登记 3 条）⇒ rc=1 且条号点名
    r3, d3, reg3 = mk_fixture(tmp, "case-3", rows=(("A", "已入账"), ("B", "待拍"), ("1", "待拍")))
    rc, out = cap(r3, d3, reg3, False)
    case("③ 负控：登记件缺条 ⇒ rc=1", 1, rc, ("条号 `2`" in out or "条数" in out))

    # ④ 负控：依据引文在作业单里 0 命中 / 多命中 ⇒ rc=1 并点名命中数
    r4, d4, reg4 = mk_fixture(tmp, "case-4", reg_text=(
        build_register((("A", "已入账"), ("B", "待拍"), ("1", "待拍"), ("2", "待拍")))
        .replace("已入账 ✓（`Q-901`） |", "这句在作业单里找不到-zz |")))
    rc, out = cap(r4, d4, reg4, False)
    case("④ 负控：依据引文 0 命中 ⇒ rc=1", 1, rc, "命中 0 处" in out)
    r4b, d4b, reg4b = mk_fixture(tmp, "case-4b", reg_text=(
        build_register((("A", "已入账"), ("B", "待拍"), ("1", "待拍"), ("2", "待拍")))
        .replace("已入账 ✓（`Q-901`）", "现况")))              # `现况` 在作业单出现 4 次 ⇒ 多命中
    rc, out = cap(r4b, d4b, reg4b, False)
    case("④b 负控：依据引文多命中（`现况` 4 处）⇒ rc=1", 1, rc, "命中 4 处" in out)

    # ⑤ 负控：头部声明数与现读表行数不符 ⇒ rc=1（数字声明口径）
    src5 = FIX_SRC.replace("**2 条**", "**9 条**")
    r5, d5, reg5 = mk_fixture(tmp, "case-5", src=src5)
    rc, out = cap(r5, d5, reg5, False)
    case("⑤ 负控：头部声明「**9 条**」≠ 现读 2 行 ⇒ rc=1", 1, rc, "≠ §一–§四 现读表行数" in out)

    # ⑥ 负控：计数句与现读不符（登记件写死旧数）⇒ rc=2（登记件与现读不一致）
    r6, d6, reg6 = mk_fixture(tmp, "case-6", reg_text=(
        build_register(FIX_ROWS).replace("条 4：已拍 0 · 已入账 1 · 待拍 3",
                                         "条 99：已拍 0 · 已入账 0 · 待拍 0")))
    rc, out = cap(r6, d6, reg6, False)
    case("⑥ 负控：登记件计数句与现读不符 ⇒ rc=2", 2, rc, "缺现读计数句" in out)

    # ⑦ 负控：空转（条表 0 行）⇒ rc=2 不给结论
    src7 = ("# 合成作业单\n\n> **性质**：0 条。\n\n## 一 · 机器读面\n\n"
            "| # | 事项 | 现读实况 | 我的推荐 |\n|---|---|---|---|\n")
    r7, d7, reg7 = mk_fixture(tmp, "case-7", src=src7)
    rc, out = cap(r7, d7, reg7, False)
    case("⑦ 负控：空转（条表 0 行）⇒ rc=2 不给结论", 2, rc, "不给结论" in out)

    # ⑧ 负控：作业单不在盘 / 登记件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）
    r8, d8, reg8 = mk_fixture(tmp, "case-8")
    os.remove(os.path.join(d8, SOURCE_REL))
    rc, _ = cap(r8, d8, reg8, False)
    case("⑧ 负控：作业单不在盘 ⇒ rc=2", 2, rc)
    r8b, d8b, reg8b = mk_fixture(tmp, "case-8b")
    os.remove(reg8b)
    rc, _ = cap(r8b, d8b, reg8b, False)
    case("⑧b 负控：登记件不在盘 ⇒ rc=2", 2, rc)

    # ⑨ 负控：登记面表头逐字句找不到 ⇒ rc=2（口径换了要同笔改本件常量）
    r9, d9, reg9 = mk_fixture(tmp, "case-9")
    with open(reg9, "w", encoding="utf-8") as f:
        f.write("# 登记 · 合成\n\n> 闭集：`已拍` / `已入账` / `待拍`\n")
    rc, out = cap(r9, d9, reg9, False)
    case("⑨ 负控：登记件表头逐字句找不到 ⇒ rc=2", 2, rc, "不给结论" in out)

    # ⑩ 负控（**本门的牙**）：行命中 ≠ 条命中 —— 往作业单里灌 5 行关键词 ⇒ 行命中变、**条数不变**
    src10 = FIX_SRC + "\n更多散文：已拍、待拍、已拍、待拍、已拍（5 行关键词 · 都不是条）\n" * 5
    r10, d10, reg10 = mk_fixture(tmp, "case-10", src=src10)
    rc, out = cap(r10, d10, reg10, False)
    case("⑩ 负控：灌 5 行关键词 ⇒ **条数不变**（仍 4 条）· rc=0（拿行命中当条数 ⇒ 本格必红）",
         0, rc, "条 4：已拍 0 · 已入账 1 · 待拍 3" in out)
    case("⑩b 同跑「行命中」对照格照打（两个数**分开给**）", True,
         ("行命中" in out and "条 4：" in out), note="  见输出对照格")

    # ⑪ `--json`：条数 / 值域 / 行命中 / 判据都在（形状自证）
    r11, d11, _reg11 = mk_fixture(tmp, "case-11")
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        rc = run(r11, d11, "", True)
    doc = json.loads(buf.getvalue())
    case("⑪ `--json` 给条数 + 分行值域 + 行命中对照 + 判据 rc", True,
         (doc["counts"]["items"] == 4 and doc["counts"]["by_decision"].get("待拍") == 3
          and doc["counts"]["line_hits"] >= 1 and doc["count_line"] == "条 4：已拍 0 · 已入账 1 · 待拍 3"),
         note="  （items=%d · line_hits=%d）" % (doc["counts"]["items"], doc["counts"]["line_hits"]))

    import shutil
    shutil.rmtree(tmp, ignore_errors=True)
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="作业单「待拍 / 已拍」闭集值域 + 条数（Q-066）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="主仓仓根（默认真源：本脚本所在仓）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认：主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--register", default=None, help="登记面路径（缺省 = 文档仓里那件）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--counts", action="store_true", help="只打条数（人面两行）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    return run(a.root, a.docs_root, a.register, a.json, a.counts)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-face-commands.py —— 公开面**命令名抽取器 + 反向探针**（`T-14` · 漏点 `E-4` · 判据 `V10`）

设计真源（**逐字，不转述**）
----------------------------
`Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md` §一 第 14 行 `T-14`：
  「**反向探针 + 公开面命令名抽取器**：树里的命令**必须在页里有格**；页里的名字/旗标**必须在树里**
   （形态照 Ansible `validate-modules` 错误码 `nonexistent-parameter-documented` /
   `undocumented-parameter` / `parameter-type-not-in-doc` **【已复核】**）」
  其判据逐字：「① **正向**：抽公开面所有 `` `zerg …` `` 串 ⇒ 逐条判「命令路径在树里 + 旗标在该命令用法串里」
   ⇒ **例外须 0** ② **反向负控**：从页里删一格 ⇒ 红 ③ 真源 = `./bin/zerg help` 现读用法串
   （不另立名单）★ 依赖 `T-10`（页先落）」
`设计-公开文档面随版收口-v1.1-20260923.md` §3.6（闸⑧ 第三条判据）与 §3.5 `V10`：
  「命令名判据 = 页里的名字 / 旗标必须在树里」——即本件；它与门㉝（`T-03` 写作红线）**共用**闸⑧ 的判据面。
  闸⑧ 的接线位（现读）＝ `scripts/build/publish-preflight.sh` 的 **⑧c**（本件落地前那里逐字申报「未接线」）。

本件判**什么**（两个方向 · 两条判据）
-------------------------------------
方向 A（**树 → 页** · 错误码 `undocumented-command`）：
  `./bin/zerg help` + `./bin/zerg help dangerous` 现读解析出的**每一条**命令（现读 **119** = 81 + 38），
  必须在**页**（`publish/docs/CLI.zh-CN.md` 与 `CLI.en.md` 的**生成区**）里有**格**
  —— 缺 ⇒ 红（Ansible 那种 `undocumented-parameter` 的同族：树里有、页里没有）。
方向 B（**页 → 树** · 错误码 `nonexistent-command-documented` / `nonexistent-parameter-documented`）：
  公开面（`publish/docs/**/*.md`）里**每一个** `` `zerg …` `` 串（行内代码形态）逐条判
  ① **命令路径在树里** ② **该串里每个 `--旗标` 在该命令的用法串里**。
  —— 两处都对不上 ⇒ 红（`T-01` 那 4 行 = 这条判据抓到的第一例：公开面教了命令面没有的旗标）。

真源（**不另立名单** ✗）
------------------------
  树 = `./bin/zerg help` 现跑输出的**用法块**（`用法:` 段）+ **命令块**（`命令（本版已开放 …）:` 段）
       + `./bin/zerg help dangerous` 的**危险块**（每行 = 该命令 · 档 · 三态 · 说明 · 出处）。
  旗标真源 = **该命令的用法串**：帮助面有 ⇒ 用它（现读 81 条）；帮助面没有 ⇒ **现跑** `zerg <命令> --help`
       的首行（`Q-137` 落地后 `--help` 逐条回该条用法串 ⇒ 现读补齐 38 条危险命令的旗标面）。
  页 = `publish/docs/CLI.*.md` 的**生成区**（`<!-- ===== 生成区 BEGIN … -->` 与 `END` 之间）。
  ⇒ 本件**不内置任何命令名 / 旗标名单**（有名单就等于造了「第二份命令清单」——
     `core/cmd/zerg/helptext.go:6–7` 逐字纪律：「命令名与用法**逐字来自命令树**，本文件不旁写第二份命令清单 ✗」）。

数据面的三处口径（**照实** · 与判据同源，不是「例外」）
--------------------------------------------------------
  R1 **通用式行**：串的首个命令词是占位符（`<对象>` 一类）⇒ 那是**形态行**（`zerg <对象> <动作> [参数] [旗标]`），
     不是命令引用 ⇒ 登记计数、**不判**（`help` 的用法块里那一行本身就是形态声明）。
  R2 **主题面**：`zerg help <主题>` 的第二段是**主题数据**（`core/cmd/zerg/topics.go`），不是命令路径
     ⇒ 路径只判族名 `help` 在树里；主题本身**真跑** `zerg help <主题>` 判存在性（rc≠0 ⇒ 红）。
     （同口径先例：`scripts/gates/check-doc-cmds.py` 的「主题数据只判族名」那一段 —— 同一条纪律，不新立。）
  R3 **族 / 子族引用**（`zerg core daemon` 这种**严格前缀**）：页里用它当分组标签 ⇒ 只要它有实体
     （是某条树命令的严格前缀）即过；★ 它没有「**该命令**的用法串」⇒ **旗标不判**（照实打印，不静默放行）。
  R4 **旗标取值 / 占位符**（`--confirm=<主机名>` · `<路径>`）：一律不作命令词（`check-doc-cmds.py` 同口径）。
  R5 危险命令（38 条）的**旗标**真源 = 现跑 `zerg <命令> --help` 的首行用法串（`Q-137` 落地后 `--help`
     逐条回该条用法串）。★ 为什么**不**用 `help dangerous` 那一行当旗标真源：那一行只声明三态
     （`--dry-run` / `--confirm` / `--yes`），**不含**该命令的完整旗标面（现读实测：`zerg repo commit --help`
     的用法串含 `--only` / `--message` / `--waive`，而危险块那一行不含）⇒ 拿那一行当旗标真源会**自造假红**。
     两处都取不到 ⇒ 该处旗标记「**未判**」（照实打印 · 不当绿、也不当红）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿：方向 A 119 条逐条有格；方向 B 零例外；两条扫面都非空。
  1 = **有红**：方向 A 缺格 / 方向 B 有例外 —— **逐条**点名 `件:串` + 错误码 + 处置方向（不许静默过滤 ✗）。
  2 = **不给结论**：页缺件 / 生成区标记找不到 / 树解析不出（空转） / 危险块自报数与解析数不符 /
      公开面 0 串 / `--self-test` 不过。

★ 本件**不做**任何 shell 调用取 rc（仓内门禁硬规矩：管道后 `$?` 是末端 rc）—— 只 `subprocess` 直调
  `bin/zerg`（取 stdout / rc 原值）与 `python3` 解释器自身的产物。

现读读数（**照实** ✗ · 本件落地当刻 · 2026-09-24）
---------------------------------------------------
  方向 A：`CLI.zh-CN.md` / `CLI.en.md` 两页生成区 **119/119** 条逐条有格 ⇒ **绿** ✓。
  方向 B：公开面 **18** 件 · 扫到 `` `zerg …` `` 串 **454** 个（通用式行 1 · 族/子族引用 8 ·
          主题面 4 个主题）⇒ **例外 2 处**（`T-14` 判据①「例外须 0」**今天不满足** ✗）：
          两页（`CLI.zh-CN.md` / `CLI.en.md`）各 1 处 —— 串 `zerg context ls --resume`：
          现读用法串 = `zerg context ls [--json <字段>]`（**不含** `--resume`），而 `--resume` 是**真旗标**
          （`core/cmd/zerg/main.go:1617` 解析 + `readonly.go:269` 读面 + `family_receipt.go:8`/`:211` 登记）
          ⇒ 根因 = **命令面那一条的用法串漏声明**（**不是**页写错）⇒ 处置落在 `core/cmd/zerg/main.go` 那一面
          （**不在本件授权内** ✗ · 本件只照实报红）。
  未判：**0** 处（所有带旗标的引用都取到了用法串）✓。
  ⇒ 本件现读 **rc=1**（有例外 · 逐条在案 · 闸⑧ 是**只报告档** ⇒ 不拦推送、但看得见）。

用法
----
    python3 scripts/gates/check-public-face-commands.py                    # 判真目标（仓根）
    python3 scripts/gates/check-public-face-commands.py --root <树>          # 判别的树（自检夹具同入口）
    python3 scripts/gates/check-public-face-commands.py --zerg <bin/zerg>    # 换真源二进制（默认 <root>/bin/zerg）
    python3 scripts/gates/check-public-face-commands.py --json counts,hits  # 机器读面
    python3 scripts/gates/check-public-face-commands.py --self-test          # 成对负控（合成夹具 · 不碰真目标）
    python3 scripts/gates/check-public-face-commands.py --no-self-test

★ 本件是**私有面件**（件头把设计真源与逐字引文指向仓外 `Zerg-内部文档/项目文档/v2.5.12/`，且逐条带私有编号
  `T-14` / `Q-127` 一类）⇒ 登记在 `scripts/公开标记.tsv` 的 `public=no` 面（三处同批：本表 +
  `scripts/build/publish-public.sh` 的 `EXCLUDES` + `publish/mirror-public-lib.py` 的 `DROP_EXACT`）。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
SELF_REL = "scripts/gates/check-public-face-commands.py"

FACE_DIR = "publish/docs"                       # 公开面（判据树）
PAGES = ("publish/docs/CLI.zh-CN.md", "publish/docs/CLI.en.md")   # 「页」= 命令参考件（`T-10` 落）
GEN_BEGIN = re.compile(r"<!--\s*=+\s*生成区 BEGIN[^>]*-->")
GEN_END = re.compile(r"<!--\s*=+\s*生成区 END[^>]*-->")

CMD_ROW = re.compile(r"^  zerg\s+(.*)$")        # help 面里一行命令（两空格缩进 + `zerg `）
TWO_SP = re.compile(r"\s{2,}")                 # 描述列的分隔（列对齐）
FLAG = re.compile(r"--[A-Za-z][A-Za-z0-9-]*")
SPAN = re.compile(r"`zerg\s+([^`]+)`")          # 行内代码形态的命令引用
DANG_HEAD = re.compile(r"危险动作（已登记\s*(\d+)\s*条")
DANG_TAIL = re.compile(r"上表\s*(\d+)\s*条")


def _run(zerg, *args):
    """直调二进制取 rc / stdout / stderr（不接管道 · 不见 shell）。"""
    try:
        p = subprocess.run([zerg] + list(args), capture_output=True, text=True)
    except OSError as e:
        return 127, "", "cannot execute %s: %s" % (zerg, e)
    return p.returncode, p.stdout, p.stderr


def _path_of(line):
    """从 help 面一行取**命令路径**：列对齐切成「用法串」字段后，取首个旗标/占位符之前的词。"""
    m = CMD_ROW.match(line)
    if not m:
        return None, None
    usage = TWO_SP.split(m.group(1))[0].strip()
    toks = usage.split()
    if not toks:
        return None, None
    if toks[0][0] == "<":
        return None, usage          # 通用式行（R1）—— 不是命令
    segs = []
    for t in toks:
        if t[0] in "-<[":
            break
        segs.append(t)
    return (" ".join(segs) or None), usage


def parse_tree(zerg):
    """真源 = `./bin/zerg help` + `./bin/zerg help dangerous` **现跑**解析。

    返回 (usage, danger, note)：
      usage  = {命令路径: 该命令的用法串}（用法块 + 命令块 · 两处同集合 · 现读 81 条）
      danger = {命令路径: 危险块那一行}（现读 38 条）
      note   = 读数与守卫结果（照实，供打印）
    守卫（任一不过 ⇒ 调用方 rc=2）：空转 / 危险块自报数与解析数不符 / 两集合相交。
    """
    note = {"blocks": {}, "guard": []}
    rc, out, err = _run(zerg, "help")
    note["blocks"]["help_rc"] = rc
    if rc != 0 or not out.strip():
        note["guard"].append("`zerg help` 现跑 rc=%s（stdout %d 字节）⇒ 树解析不上" % (rc, len(out)))
        return {}, {}, note
    lines = out.split("\n")
    i_use = next((i for i, l in enumerate(lines) if l.startswith("用法") and l.rstrip().endswith(":")), None)
    i_cmd = next((i for i, l in enumerate(lines) if l.startswith("命令（本版已开放")), None)
    i_dan = next((i for i, l in enumerate(lines) if DANG_HEAD.search(l)), None)
    if i_use is None or i_cmd is None or i_dan is None or not (i_use < i_cmd < i_dan):
        note["guard"].append("帮助面结构对不上：找不到按序的「用法块 → 命令块 → 危险块」三块"
                             "（现读 `用法:`=%s · 命令块=%s · 危险块=%s）" % (i_use, i_cmd, i_dan))
        return {}, {}, note
    use_start, use_end, cmd_end = i_use + 1, i_cmd, i_dan
    generic = []
    usage = {}
    for l in lines[use_start:use_end] + lines[use_end + 1:cmd_end]:
        p, u = _path_of(l)
        if p is None:
            if u is not None and u.split() and u.split()[0][0] == "<":
                generic.append(u)
            continue
        usage[p] = u
    note["blocks"]["generic"] = generic
    note["blocks"]["usage_n"] = len(usage)

    rc2, out2, err2 = _run(zerg, "help", "dangerous")
    note["blocks"]["danger_rc"] = rc2
    if rc2 != 0 or not out2.strip():
        note["guard"].append("`zerg help dangerous` 现跑 rc=%s ⇒ 危险块解析不上" % rc2)
        return {}, {}, note
    danger = {}
    for l in out2.split("\n"):
        p, _u = _path_of(l)
        if p is None:
            continue
        danger[p] = l.strip()                           # 全行：档 + 三态旗标 + 说明（R5）
    note["blocks"]["danger_n"] = len(danger)

    # ── 守卫（空转 / 自洽 / 相交）──────────────────────────────────────────────
    if not usage or not danger:
        note["guard"].append("树为空转：用法/命令块 %d 条 · 危险块 %d 条" % (len(usage), len(danger)))
    claimed = None
    for l in lines:
        m = DANG_HEAD.search(l)
        if m:
            claimed = int(m.group(1))
            break
    for l in out2.split("\n"):
        m = DANG_TAIL.search(l)
        if m:
            claimed = int(m.group(1)) if claimed is None else claimed
            tail = int(m.group(1))
            if tail != len(danger):
                note["guard"].append("危险块**自报数**（`上表 %d 条`）≠ 解析出的 %d 条 ⇒ 判不了"
                                     % (tail, len(danger)))
            break
    if claimed is not None and claimed != len(danger):
        note["guard"].append("危险块表头自报「已登记 %d 条」≠ 解析出的 %d 条 ⇒ 判不了"
                             % (claimed, len(danger)))
    both = set(usage) & set(danger)
    if both:
        note["guard"].append("命令块 ∩ 危险块 = %d 条（%s）⇒ 两集合本应不相交（现读 0）"
                             % (len(both), " · ".join(sorted(both)[:4])))
    note["blocks"]["claimed_danger"] = claimed
    return usage, danger, note


def gen_region(text):
    """页的生成区（有标记 ⇒ 只判区内；标记缺 ⇒ None = 判不了）。"""
    mb = GEN_BEGIN.search(text)
    me = GEN_END.search(text)
    if not mb or not me or me.start() < mb.end():
        return None
    return text[mb.end():me.start()]


def judge(root, zerg=None, self_exempt=True):
    """判一棵树。返回 (rc, lines, data)。lines 逐条照实（含未判项），不静默。"""
    root = os.path.abspath(root)
    zerg = zerg or os.path.join(root, "bin", "zerg")
    face = os.path.join(root, FACE_DIR)
    lines, blocks, hits, stat = [], [], [], {}
    data = {"root": root, "zerg": zerg, "tree": 0, "danger": 0, "spans": 0,
            "generic": 0, "group_ref": 0, "hits": [], "blocks": []}

    if not os.path.isdir(face):
        return 2, ["⛔ 缺件：%s/ 不在（缺件 ≠ 绿 ⇒ 不给结论）" % FACE_DIR], data
    if not os.path.isfile(zerg):
        # 夹具 / 负控回落：判据树换了根（`ZERG_PUBFACE_ROOT` 一族）时，真源二进制仍取**本仓现制品**
        #   （判据树 = 页，命令树真源 = 现跑二进制）—— 回落**逐条打印**，不是静默放行。
        alt = os.path.join(REPO_ROOT, "bin", "zerg")
        if os.path.isfile(alt):
            lines.append("⚠ 真源二进制改指：`%s` 不在 ⇒ 回落 `%s`（夹具/负控专用 · "
                         "与 `ZERG_PUBFACE_ROOT` 同族）" % (zerg, alt))
            data["zerg"] = zerg = alt
        else:
            return 2, ["⛔ 缺件：真源二进制 %s 不在（缺件 ≠ 绿 ⇒ 不给结论）" % zerg], data
    if not os.path.isfile(zerg):
        return 2, ["⛔ 缺件：真源二进制 %s 不在（缺件 ≠ 绿 ⇒ 不给结论）" % zerg], data

    usage, danger, note = parse_tree(zerg)
    data["tree"], data["danger"] = len(usage), len(danger)
    data["blocks"] = list(note["blocks"].get("generic", []))
    data["generic"] = len(data["blocks"])
    for g in note["guard"]:
        blocks.append(g)
    if blocks:
        return 2, ["⛔ %s" % b for b in blocks] + ["⛔ 树解析判不了 ⇒ **不给结论（rc=2）**"], data

    tree = set(usage) | set(danger)
    data["tree_total"] = len(tree)
    lines.append("真源：`%s` 现读 —— 命令块 **%d** 条 + 危险块 **%d** 条 = **%d** 条"
                 % (os.path.basename(zerg), len(usage), len(danger), len(tree)))

    # ── 扫面先取数（空转守卫在最前：0 串 = 判据无区分度）──────────────────────
    files = []
    for dp, dns, fns in os.walk(face):
        dns[:] = sorted(d for d in dns if not d.startswith("."))
        for fn in sorted(fns):
            if fn.startswith(".") or not fn.endswith(".md"):
                continue
            files.append(os.path.join(dp, fn))
    face_texts = {}
    for fp in files:
        rel = os.path.relpath(fp, root)
        if self_exempt and rel == SELF_REL:
            continue                                    # 自豁免（本件自身若落在判据树里）
        try:
            face_texts[rel] = open(fp, encoding="utf-8").read()
        except OSError as e:
            blocks.append("件读不了：%s（%s）" % (rel, e))
    spans = []
    for rel in sorted(face_texts):
        for m in SPAN.finditer(face_texts[rel]):
            spans.append((rel, m.group(1).strip()))
    data["spans"] = len(spans)
    data["files"] = len(face_texts)
    if blocks:
        return 2, ["⛔ %s" % b for b in blocks] + ["⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）"], data
    if not spans:
        return 2, ["⛔ 空转：公开面 **%d** 件里 **0** 个 `` `zerg …` `` 串 ⇒ 判据无区分度"
                   "（不给结论 · 空转 = 假覆盖）" % len(face_texts)], data

    # ── 方向 A（树 → 页）──────────────────────────────────────────────────────
    lines.append("── 方向 A（`undocumented-command`）树 **%d** 条逐条须在页的生成区里有格 ──" % len(tree))
    for rel in PAGES:
        fp = os.path.join(root, rel)
        if not os.path.isfile(fp):
            blocks.append("页缺件：%s 不在（方向 A 判不了）" % rel)
            continue
        try:
            text = open(fp, encoding="utf-8").read()
        except OSError as e:
            blocks.append("页读不了：%s（%s）" % (rel, e))
            continue
        body = gen_region(text)
        if body is None:
            blocks.append("页 %s 的生成区标记找不到（`生成区 BEGIN`/`END`）⇒ 判不了" % rel)
            continue
        miss = [c for c in sorted(tree) if ("zerg " + c) not in body]
        if miss:
            for c in miss[:12]:
                hits.append({"rel": rel, "code": "undocumented-command", "ref": "zerg " + c,
                             "why": "树里有这条命令、页的生成区里找不到它 ⇒ 树里新了、页没跟上"})
            lines.append("   ✗ %s：**%d 条**树命令在生成区里没有格（%s）"
                         % (rel, len(miss), " · ".join(miss[:6])))
        else:
            lines.append("   ✓ %s：生成区 **%d/%d** 条逐条有格" % (rel, len(tree), len(tree)))
    if blocks:
        return 2, ["⛔ %s" % b for b in blocks] + ["⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）"], data

    # ── 方向 B（页 → 树）──────────────────────────────────────────────────────
    lines.append("── 方向 B（`nonexistent-command-documented` / `nonexistent-parameter-documented`）"
                 "公开面 **%d** 件里的 `` `zerg …` `` 串逐条判 ──" % len(face_texts))
    topics = {}
    help_usage = {}                                     # path -> 现跑 `--help` 用法串（懒 · 只对帮助面缺用法串的命令）
    unjudged = []
    unparsed = []                                     # 抽不出命令路径的串（GAP-20260927-86）

    def flag_face(path):
        """该命令的**用法串**（旗标真源）：帮助面有 ⇒ 用它；没有（现读 = 38 条危险命令）⇒
        现跑 `zerg <path> --help` 取首行（`Q-137` 落地后 `--help` 逐条回该条用法串）；
        两处都取不到 ⇒ None（旗标**判不了** ⇒ 记「未判」，不静默、也不自造假红）。"""
        if path in usage:
            return usage[path]
        if path in danger:
            if path not in help_usage:
                rc3, out3, _e = _run(zerg, *(path.split() + ["--help"]))
                first = (out3.split("\n") or [""])[0].strip()
                if rc3 == 0 and first.startswith("zerg "):
                    first = first[len("zerg "):]              # 与帮助面读数同形（不带 `zerg ` 前缀）
                else:
                    first = ""
                help_usage[path] = first
            return help_usage[path] or None
        return None

    for rel, body in spans:
        toks = body.split()
        if not toks:
            continue
        if toks[0][0] == "<":
            data["generic"] += 1                        # R1 通用式行
            continue
        segs = []
        for t in toks:
            if t[0] in "-<[":
                break
            segs.append(t)
        path = " ".join(segs)
        # ★ `GAP-20260927-86`：首词是 `-`/`[` 的串（R1 守卫只认 `<`）⇒ `segs` 为空 ⇒ `path` 得**空串**（非 None）。
        #   旧实现：空 path 直落 :371 的 `parts[0]` ⇒ **IndexError 崩**，而崩溃与「有红」同退码 rc=1 ⇒ 调用方分不开。
        #   此处**登记为「判不了」并 continue**（照本门 `_path_of:126` 的 `or None` 归口 + `:241/:295` 的「判不了 ⇒ rc=2」先例）。
        if not path:
            unparsed.append("%s `zerg %s`" % (rel, body))
            continue
        used = sorted({f.split("=")[0] for f in FLAG.findall(body)})
        # R2 主题面：`zerg help <主题>` —— 第二段是主题数据（topics.go），不是命令路径。
        #    先判整条路径（`help export` 是真命令），只有**不在树里**时才按主题判、并按**真跑**验存在性。
        topic = None
        if path not in tree:
            parts = path.split()
            if parts and parts[0] == "help" and len(parts) > 1 and not parts[1].startswith(("-", "<")):
                topic, path = parts[1], "help"
        if path in tree:
            if topic is not None:
                if topic not in topics:
                    topics[topic] = _run(zerg, "help", topic)[0]
                if topics[topic] != 0:
                    hits.append({"rel": rel, "code": "nonexistent-help-topic",
                                 "ref": "`zerg %s`" % body, "why":
                                 "主题 `%s` 真跑 `zerg help %s` ⇒ rc=%s（树面里没有这个主题）"
                                 % (topic, topic, topics[topic])})
                continue
            if not used:
                continue
            uface = flag_face(path)
            if uface is None:
                unjudged.append("%s `zerg %s`（%s）" % (rel, body, path))
                continue
            bad = [f for f in used if f not in FLAG.findall(uface)]
            if bad:
                for f in bad:
                    hits.append({"rel": rel, "code": "nonexistent-parameter-documented",
                                 "ref": "`zerg %s`" % body, "why":
                                 "旗标 %s 不在 `%s` 的用法串里（现读用法串 = `zerg %s` ⇒ "
                                 "要么页写错、要么该命令的用法串漏声明）" % (f, path, uface)})
                continue
        else:
            pref = [c for c in tree if c.startswith(path + " ")]
            if pref:
                data["group_ref"] += 1                  # R3 族 / 子族引用
                continue
            hits.append({"rel": rel, "code": "nonexistent-command-documented",
                         "ref": "`zerg %s`" % body, "why":
                         "命令路径 `%s` 既不在树里、也不是任何树命令的前缀" % path})
            continue
    data["topics"] = topics
    data["help_usage"] = help_usage
    data["unjudged"] = unjudged
    data["unparsed"] = unparsed
    lines.append("   · 扫到 `` `zerg …` `` 串 **%d** 个（其中通用式行 %d · 族/子族引用 %d · 主题面 %d 个主题）"
                 % (data["spans"], data["generic"], data["group_ref"], len(topics)))
    if help_usage:
        lines.append("   · 危险命令的旗标真源：现跑 `zerg <命令> --help` 取用法串 **%d** 条（%s）"
                     % (len(help_usage), " · ".join(sorted(help_usage)[:6])))
    if unjudged:
        lines.append("   · **未判**旗标 **%d** 处（该命令的用法串两处都取不到 ⇒ 照实登记、不当绿）"
                     "：%s" % (len(unjudged), " · ".join(unjudged[:4])))
        # ★ `GAP-20260927-95`：这一档原只打印「不当绿」、退码仍落 `if hits` 之后的 0 ⇒
        #   同屏「不当绿」紧跟「全绿 rc=0」。「未判」= 取不到真源 = **判不了** ⇒ 按三档 2（不给结论）收口，
        #   优先级 2 > 1 > 0（与仓内其余门同口径）。
        lines.append("⇒ **不给结论（rc=2）**：%d 处旗标**未判**（用法串两处都取不到）——"
                     "「取不到」≠ 全绿" % len(unjudged))
        return 2, lines, data
    # ★ `GAP-20260927-86`：抽不出命令路径的串（首词 `-`/`[`）= **判不了** ⇒ 按三档 2 收口，
    #   优先级 2 > 1 > 0（与「未判旗标」同口径）：判不了优先于有红，崩溃与「有红」在退码上必须分得开。
    if unparsed:
        lines.append("   · **判不了**（抽不出命令路径的串）**%d** 处：%s"
                     % (len(unparsed), " · ".join(unparsed[:4])))
        lines.append("⇒ **不给结论（rc=2）**：%d 处 `zerg …` 串抽不出路径（首词 `-`/`[` · 判不了优先于有红）"
                     % len(unparsed))
        return 2, lines, data
    data["hits"] = hits
    if hits:
        for h in hits:
            lines.append("   ✗ %s [%s] %s —— %s" % (h["rel"], h["code"], h["ref"], h["why"]))
        lines.append("⇒ **有红（rc=1）**：方向 A/B 合计 **%d** 处例外（例外须 0 · 逐条在案、不静默过滤）"
                     % len(hits))
        return 1, lines, data
    lines.append("⇒ 全绿（rc=0）：方向 A **%d** 条逐条有格 · 方向 B **%d** 串零例外"
                 % (data["tree_total"], data["spans"]))
    return 0, lines, data


# ── 合成夹具自检（成对负控 · 不碰真目标）────────────────────────────────────────
def make_fixture(base, pages=None, drop_rows=(), extra=(), clean_exceptions=False, stub=False):
    """造一棵最小判据树：docs/ 夹具 + 页（默认真页副本 · 可删行 / 加串 / 空转桩）。"""
    root = tempfile.mkdtemp(prefix="pfcmds-", dir=base)
    os.makedirs(os.path.join(root, FACE_DIR))
    for rel in PAGES:
        pg = os.path.join(root, rel)
        if stub:
            open(pg, "w", encoding="utf-8").write(
                "<!-- ===== 生成区 BEGIN · 夹具 ===== -->\n夹具：无命令引用\n<!-- ===== 生成区 END ===== -->\n")
            continue
        text = open(os.path.join(REPO_ROOT, rel), encoding="utf-8").read()
        if clean_exceptions:
            text = text.replace("`zerg context ls --resume`", "`zerg context ls`")
        if drop_rows:
            keep = []
            for l in text.split("\n"):
                if any((d + "`") in l and ("| `" + d) in l for d in drop_rows):
                    continue
                keep.append(l)
            text = "\n".join(keep)
        if extra:
            text += "\n" + "\n".join(extra) + "\n"
        open(pg, "w", encoding="utf-8").write(text)
    for name, body in (pages or {}).items():
        fp = os.path.join(root, FACE_DIR, name)
        open(fp, "w", encoding="utf-8").write(body)
    return root


def self_test():
    lines = []
    ok = []

    def check(cond, desc):
        ok.append(bool(cond))
        lines.append("   %s %s" % ("✓" if cond else "✗", desc))

    # 前置：真源二进制在（自检要拿真树当「好件」）
    real = os.path.join(REPO_ROOT, "bin", "zerg")
    if not os.path.isfile(real):
        lines.append("   ✗ 前置缺件：%s 不在 ⇒ 自检不给结论" % real)
        return 2, lines
    base = tempfile.mkdtemp(prefix="pfcmds-st-")
    try:
        # 格 1 正控：真页（修掉现读例外）⇒ 0 例外
        r1 = make_fixture(base, clean_exceptions=True)
        rc, out, _d = judge(r1, zerg=real)
        check(rc == 0, "格 1 正控（真页 + 现读例外串改写）⇒ rc=0（实测 rc=%d）" % rc)
        # 格 2 负控（判据②的「从页里删一格 ⇒ 红」）
        r2 = make_fixture(base, clean_exceptions=True, drop_rows=("zerg version",))
        rc, out, d2 = judge(r2, zerg=real)
        check(rc == 1 and any(h["code"] == "undocumented-command" for h in d2["hits"]),
              "格 2 负控（页里删掉 `zerg version` 那一格）⇒ rc=1 + `undocumented-command`（实测 rc=%d）" % rc)
        # 格 3 负控：页里塞不存在的命令
        r3 = make_fixture(base, clean_exceptions=True, extra=("见 `zerg frobnicate --xyz`。",))
        rc, out, d3 = judge(r3, zerg=real)
        check(rc == 1 and any(h["code"] == "nonexistent-command-documented" for h in d3["hits"]),
              "格 3 负控（页里塞 `zerg frobnicate`）⇒ rc=1 + `nonexistent-command-documented`（实测 rc=%d）" % rc)
        # 格 4 负控：真命令 + 假旗标
        r4 = make_fixture(base, clean_exceptions=True, extra=("见 `zerg version --frob`。",))
        rc, out, d4 = judge(r4, zerg=real)
        check(rc == 1 and any(h["code"] == "nonexistent-parameter-documented" for h in d4["hits"]),
              "格 4 负控（真命令 + 假旗标 `--frob`）⇒ rc=1 + `nonexistent-parameter-documented`（实测 rc=%d）" % rc)
        # 格 5 负控：假帮助主题（真跑判）
        r5 = make_fixture(base, clean_exceptions=True, extra=("见 `zerg help frobnicate`。",))
        rc, out, d5 = judge(r5, zerg=real)
        check(rc == 1 and any(h["code"] == "nonexistent-help-topic" for h in d5["hits"]),
              "格 5 负控（假主题 `zerg help frobnicate`）⇒ rc=1 + `nonexistent-help-topic`（实测 rc=%d）" % rc)
        # 格 6 空转负控：公开面 0 串（两页都换成无命令引用的桩）
        r6 = make_fixture(base, stub=True)
        rc, out, _d = judge(r6, zerg=real)
        check(rc == 2, "格 6 空转负控（公开面 0 个 `zerg …` 串）⇒ rc=2（实测 rc=%d）" % rc)
        # 格 7 缺件负控：页不在
        r7 = tempfile.mkdtemp(prefix="pfcmds-no-", dir=base)
        os.makedirs(os.path.join(r7, FACE_DIR, "design"))
        rc, out, _d = judge(r7, zerg=real)
        check(rc == 2, "格 7 缺件负控（页不在 ⇒ 方向 A 判不了）⇒ rc=2（实测 rc=%d）" % rc)
        # 格 8 区分度反证：真仓现读的例外必须**真的**判得出来（不是恒绿）
        rc, out, d8 = judge(REPO_ROOT, zerg=real)
        check(rc in (0, 1) and d8["spans"] > 0,
              "格 8 真树现跑：rc=%d · 扫到 %d 串 · 例外 %d 处（照实：本格只证「判得动」，不要求 0）"
              % (rc, d8["spans"], len(d8["hits"])))
        # 格 9 判不了负控（GAP-20260927-86）：页里塞首词为 `-`/`[` 的串 ⇒ 旧版 IndexError 崩（与有红同退码 rc=1）。
        #   改后必须 **rc=2** 且 data 里记 unparsed（判不了优先于有红），不崩。
        r9 = make_fixture(base, clean_exceptions=True, extra=("见 `zerg -x 形态`。", "见 `zerg [对象] ls`。"))
        rc, out, d9 = judge(r9, zerg=real)
        check(rc == 2 and len(d9.get("unparsed", [])) >= 1,
              "格 9 判不了负控（串首词 `-`/`[` ⇒ 抽不出路径）⇒ rc=2 + data.unparsed 非空（实测 rc=%d · unparsed=%d）"
              % (rc, len(d9.get("unparsed", []))))
    finally:
        shutil.rmtree(base, ignore_errors=True)
    if all(ok):
        lines.append("   ⇒ 自检全过（%d 格）" % len(ok))
        return 0, lines
    lines.append("   ⇒ 自检不过（%d/%d 格）" % (sum(ok), len(ok)))
    return 2, lines


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['counts', 'hits', 'topics']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True, description="公开面命令名抽取器 + 反向探针（`T-14` · `V10`）")
    ap.add_argument("--root", default=REPO_ROOT, help="判据树根（默认仓根）")
    ap.add_argument("--zerg", default=None, help="真源二进制（默认 <root>/bin/zerg）")
    ap.add_argument("--json", default=None, help="机器读面：逗号分隔 counts,hits,topics")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过开跑前的自检")
    a = ap.parse_args()

    if not a.self_test and not a.no_self_test:
        src, out = self_test()
        print("── 开跑前自检（合成夹具 · 不碰真目标）──")
        for l in out:
            print(l)
        if src != 0:
            print("⛔ 自检不过 ⇒ 拒绝跑真目标（门禁自己先能被证明「会红」）")
            return 2
    elif a.self_test:
        src, out = self_test()
        print("── 自检（合成夹具 · 不碰真目标）──")
        for l in out:
            print(l)
        return src

    rc, lines, data = judge(a.root, zerg=a.zerg)
    print("门 · 公开面命令名（`T-14` · 判据 `V10` · 漏点 `E-4`）· 树根 %s" % data["root"])
    for l in lines:
        print(l)
    if a.json:
        keys = [k.strip() for k in a.json.split(",") if k.strip()]
        face = {}
        if "counts" in keys:
            face["counts"] = {"commands": data["tree"], "dangerous": data["danger"],
                              "tree_total": data.get("tree_total", 0), "spans": data.get("spans", 0),
                              "generic": data.get("generic", 0), "group_ref": data.get("group_ref", 0)}
        if "hits" in keys:
            face["hits"] = data.get("hits", [])
        if "topics" in keys:
            face["topics"] = data.get("topics", {})
        print(json.dumps(face, ensure_ascii=False, sort_keys=True))
    if rc == 2:
        print("⛔ 缺件/判不了 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿 · 不许当绿）")
    elif rc == 1:
        print("✗ 有红（rc=1）：例外须 0 · 逐条见上（处置只有两条：改页，或改命令面的用法串 —— "
              "不许放宽判据来凑绿 ✗）")
    return rc


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-help-shape.py —— `help` 面的**形状判据**（2026-09-27 · 缺口 `GAP-20260927-401`）

为什么有这个件（缺口本身）
--------------------------
  `help --all` 与裸 `help` 的**形状**此前没有任何判据钉：
    · `scripts/gates/check-help-snapshot.py` 只咬 **`help export`（⟷ 文档快照）** 那一面；
    · `core/cmd/zerg/cli_help_all_test.go` 只判「条数等式 + 危险档逐条覆盖」（**数条数、不判段**）；
    · `cli_help_twostate_test.go` 判的是**两态 rc 语义**（本件禁碰）。
  ⇒ HEAD `a71aa538` 给 `help --all` 末尾追加「族名清单」时，**行数 / 末行都无人会响**
    （当时那笔行数差分可从 `a71aa538` 现算 —— **读数一律现读 · 禁手写**）——
    本件把这三样收进判据：**族名清单段消失 / 行数漂 / 末行被改 ⇒ 判红**。

判据（三条 · 全部「现算 + 对界」· 判据里没有一个字是自填的）
--------------------------------------------------------------
  S1 **族级段（闭集相等 · 现算两源）**
     源甲 = 现跑 `bin/zerg help --all` 末尾的族级段 token；源乙 = 命令树
     `core/cmd/zerg/testdata/cli-matrix.json` 的 `cases[].command` **首段**（去重·字典序）。
     判：段头在（形如 `族级面（<N> 族 · …）:`）· 段头括注的 `<N>` **等于**现算族数 ·
     两源**逐 token 相等**（双向差集必须都空）· 段外的裸 `help` **不许**出现族级段（只在 `--all` 档）。
     牙口：删掉整段 ⇒ token 集空 ⇒ 红；漏一族 / 多一族 ⇒ 红；族名改字 ⇒ 红。
  S2 **条数与行数（关系式 + 精确闭集常量）**
     关系式（三面现跑）：`rows(help --all) == rows(help) + rows(help dangerous)`（危险档那批行的净增）。
     关系式（段自洽）：族级段行数（自段头行算到 EOF）`== 1 + ceil(len(族)/8)`（`1` = 段头一行 ·
                     `8` = 源件布局常量 `main.go:3380` 那一行按 8 族一行）；另判**段头之前那一行必须是空行**。
     常量（**现跑取数 · 非自填**）：两个面的 `lines` / `bytes` 必须逐面相等 ——
                     取法 = `./bin/zerg help --all | wc -l -c` 与 `./bin/zerg help | wc -l -c`；
                     刷新 = 本件 `--emit-baseline`（现跑打印新值，照抄进 `BASELINE`，**别手写**）。
     牙口：`--all` 少/多一行 ⇒ 常量格红；整面漂 ⇒ 常量格红；`--all` 少一条命令行 ⇒ 关系式红。
                  S3 **末行（现算重建 ⟷ 逐字）**
     末行必须是**由族名清单现算重建**的那一行（族名按字典序每 8 族一行，取最后一块，`"  " + " · ".join(块)`）⇒
     改末行任何一个字 / 掉一族 / 换分隔符都红。
     另钉裸 `help` 末行逐字（`BASELINE["help"]["last"]`）· 判据与 `--all` 末行**必须是两条不同的断言**
     （两态各自的末行都不许被改写）。

  S4 **危险档面（`help dangerous` 的形状 —— 2026-09-27 补 · 缺口 `GAP-20260927-418`）**
     病：本件此前只在 S2 关系式里**数**危险档的行数（`rows(--all) == rows(help) + rows(dangerous)`），
     该面自己的**条数 / 字节 / 逐条旗标串 / 表尾计数**一个都没钉 —— 实测（2026-09-27）`approve new`
     翻开 `ChecksConfirm` ⇒ 该面**字节 / 行数当场变**（逐条行多印一段 `--confirm=<工具名>`；
    具体读数以 `--emit-baseline` 现跑取 —— **禁手写**），
     **门69 照绿**，全靠施工卵自报才发现。S4 把该面收进同一套「现算 + 对界 + 精确闭集常量」：
     S4a **表尾计数（自洽 + 对条数）**：末行必须形如 `上表 N 条：**已开放 X 条**…**未开放 Y 条**`，
        判 `N == X + Y` 且 `N == 逐条行数`（危险档行 = 两空格 + `zerg ` 开头那一批）。
     S4b **逐条标记计数 ⟷ 表尾**：`← **已开放**` / `← 本版未开放` 两种行尾标记的**出现次数**必须
        分别等于表尾的 `X` / `Y`（表尾与逐条不许各说各话）。
     S4c **逐条行形状闭集**：每条必须形如
        `  zerg <命令路径>  D2|D3  --dry-run · [--confirm=<目标> · ]--yes  <它会动什么>  [出处]  ← 标记`，
        旗标串**只许**两种形态（印不印 `--confirm=` 由登记现算）—— 少印/多印一段、档位写成别的、标记改字 ⇒ 红。
     S4d **`--confirm=<目标>` ⟷ 该命令真实用法串（同源 · 不恒绿）**：凡逐条行印了 `--confirm=<X>`，
        取 `X` 的**核心名**（剥掉 `（…）` 括注与反引号），它必须在 `help --all` 里**该命令那一行**的
        用法串中逐字出现（用法串 = 命令树旗标定义的另一面，与本面的登记字段是两处来源）。
        **已知脱钩的处数一律现读 · 禁手写**（现算脱钩集合 · 冻结名单 = `BASELINE["help dangerous"]`
        的 `confirm_not_in_usage`；名单里那几处名字照实冻结，不假装没有）⇒
        判据 = 现算脱钩集合 **逐字等于**冻结集合：多一处 ⇒ 红（旗标串新脱钩）；少一处 ⇒ 红（缺口真被修了，
        须 `--emit-baseline` 刷新冻结值 ⇒ 不许静默漂）。
     ★ S4d 的盲区（缺口 `GAP-20260927-423` · 2026-09-27）：它只判**集合** —— 危险面印 `--confirm=<提案 id>`、
       用法串印 `--confirm=<主机名>` 时，只要 `提案 id` 在该命令用法串的**别处**（`--proposal <提案 id>`）露过面，
       集合就不变 ⇒ **不红**（`6f7edb2a` 之前 `dev edit` 那处真值冲突正是这样藏住的）。
     S4f **两面同印 `--confirm=` ⇒ 值须逐字相等（独立一条腿 · 2026-09-27 补 · 缺口 `GAP-20260927-423`）**：
        凡危险面某条印了 `--confirm=<X>`，**且**该命令在 `help --all` 的用法行里也印了 `--confirm=<Y>`
        （**两面同印的条目数一律现读 · 禁手写** —— 本件 `--json` 的 `danger_confirm_both_faces`
        格就是现跑自报的那个数）⇒ 判 `X == Y` **逐字**（不做任何规范化：`--confirm` 的值就是用户在
        终端上照敲的那一串，两面各说一个名字 = 用户照哪面敲都可能被拒 —— 正是本腿要收的账）。
        判据 = 现算「值冲突」条目集合（`[命令路径, 危险面值, 用法串值]`）**逐字等于**冻结名单
        （冻结名单 = 本件 `BASELINE["help dangerous"]["confirm_value_mismatch"]` ·
        **处数现读 · 禁手写**）：多一处 ⇒ 红（新值冲突）；少一处 ⇒ 红（真被修了 ⇒
        `--emit-baseline` 刷，不许静默漂）。
        ★ 与 S4d 的关系 = **两条独立腿**，不是改 S4d：S4d 判「那个名字在不在用法串里」（集合账），S4f 判
        「两面印的值是不是同一个」（值账）；S4d 的判定式与冻结名单（脱钩处数**现读 · 禁手写**）**一字不动** —— 免得把两种
        性质的账混进一个名单里（混进去后「新增一处脱钩」这句话就说不清了）。
        ★ 我的选择（写在件头 · 供后人对界）：**只覆盖「两面都印 `--confirm=`」的条目** —— 一面印、一面不印的
        （**两类各自的条数一律现读 · 禁手写**：一类 = S4d 已入账的脱钩处，另一类 = 把目标写在位置
        参数里的其余条目；拆项以本件现跑自报（`--json` 的脱钩集合与 `danger_confirm_both_faces` 格）
        或门日志为准）不是本腿的账，本腿不许假装判了。
        ★ 误报面（我的选择 + 理由）：两面若印**同义不同形**的值（如一面 `模型名（\`--model\`）`、一面 `模型名`）
        ⇒ 本腿判**红**（把「形态差」按**值冲突**入账，不为它另开第三份口径）—— 理由：界面上 `--confirm=<…>`
        的字面就是用户要敲的东西，两面字面不同就该有人来解释；**这类误报的处数一律现读 · 禁手写**
        （现跑值冲突集合的条数 ⇒ 本件 `--json` 的 `danger_confirm_value_mismatch` 格）。
        若将来确有正当的形态差 ⇒ 走
        `--emit-baseline` 冻结（一个看得见的决定），不许在本腿里默默放宽。
     S4e **行数/字节/条数常量**：`help dangerous` 的 `lines` / `bytes` / `rows` 必须逐格等于冻结读数
        （**现跑取数** · 禁手写 · `--emit-baseline` 刷）。★ 本条正是 `approve new` 那类病的**直接**牙口：
        逐条行多印一段旗标 / 表尾改数 ⇒ 字节或条数当场不等。
     牙口：改一条的旗标串（等长换字 ⇒ 字节不变）⇒ S4d 红并点名到条目；两面同印 `--confirm=` 而值不等
     （等长换字 · 名字集合不变 ⇒ S4d / S4e 都看不见）⇒ **S4f** 红并点名到条目；表尾计数对不上 ⇒ S4a/S4b 红；
     该面行数/字节漂 ⇒ S4e 红。**S4 只判形状**：逐条「它会动什么 / 出处」的**文字**与档位语义不在本格。

  S5 **配置主题面（`help config` 的形状 —— 2026-09-27 补 · 缺口 `GAP-20260927-440`）**
     病：判据面只跑 `help` / `help --all` / `help dangerous` 三面，**不含 `help config`** ——
     `exitcodes.go` 的 `helpConfig()`（第 102 行起）文案改动**没有任何判据会响**（`help config` 的
     **行数 / 字节一律现读 · 禁手写**：`./bin/zerg help config | wc -l -c`，与 `BASELINE` 同源 ⇒
     全裸奔）。S5 把该面收进同一套「现算 + 交叉 + 精确闭集常量」：
     S5a **同面交叉（优先级链 ⟷ 逐级说明 · 现算 · 无常数）**：`help config` 里那一条
         `  <级> > <级> > …` 链行现算出的**级名表**，必须与 `  N. **<名字>**` 这一批编号条目的
         **名字表同序逐字相等**（两边都剥掉反引号 —— 与 S4d 的剥法同口径）；另判编号必须是
         `1..N` 连号无重复。牙口：链与逐级说明**任何一处**改名 / 改序 / 掉一级 ⇒ 红（同面的两个
         独立格式必须互相对得上）；链行不许 0 条、不许抄出第二条。
     S5b **同源跨面（`help config` ⟷ 裸 `help` 的那一行 · 现算 · 无常数）**：两面的链行必须
         **逐字相等** —— 它们是**同一个真源**（`core/cmd/zerg/helptext.go` 的 `configPriorityLine`
         常量：`help config` 由 `exitcodes.go:104` 拼 `"  " + 常量`，裸 `help` 由 `helptext.go:95`
         拼同一常量）。牙口：谁把链**抄成第二份**（不复用常量）⇒ 两面字面分家 ⇒ 红。
     S5c **精确闭集常量**：`help config` 的 `lines` / `bytes` 必须逐格等于冻结读数（**现跑取数** ·
         禁手写 · `--emit-baseline` 刷）—— 这是「改 `helpConfig()` 文案 ⇒ 有人响」的**直接**牙口：
         任何一字增删 ⇒ 字节或行数当场不等。
     ★ 与旧腿的关系（我的选择 · 写在件头供后人对界）：**独立一条腿（S5）**，S1–S4 的判定式与冻结值
       **一字不动**；不改 S2 的关系式 —— `help config` 是**主题面**（`topics.go` 那张表分派），
       不参与 `rows(--all) == rows(help) + rows(dangerous)` 那条等式（该面的用法行条数**现读 · 禁手写**：
      `rows_of(help config)` 现算）。
     ★ 不假装覆盖（照实登记）：`help config` 只判**形状/结构**；「凭据」那四条条目的**文字**、主题表
       `topics.go` 的 `Brief` 串（`五级配置优先级 + 凭据四条`）与本面同源不同源**不在本格** ——
       那要新开一处「读 Go 源」的口径；本单**不开第三份口径**（也确无现成口子：本件的面表就是
       `main()` 里那一个元组，加第四面即走同一处）。

形态说明（为什么用「常量 + 关系式」而不是纯现算）
--------------------------------------------------
  · 族名 / 命令条数 / 段行数：**纯现算**（有第二个真源：命令树矩阵）。✓
  · 两个面的**总行数 / 总字节 / 裸 help 末行**：`help` 面只有一棵打印器，**没有第二个真源**能推出
    它的衬底（段头 / 隔行 / 版本行…）⇒ 纯关系式只能覆盖「相对变化」，覆盖不了「整面漂」——
    ⇒ 只能按本仓允许的第二形态：**精确闭集常量 + 成对负控**（`BASELINE` 的**每一格**读数全部 **现跑取数**
    （格数以本件 `BASELINE` 现读为准），
    刷新走 `--emit-baseline`，**禁手写**）。

退码
----
  0 = 全部腿都合 · 1 = 判红（段缺 / token 不等 / 关系式不成立 / 常量不等 / 末行不符 / `help config` 面形状不符）
  2 = **不给结论**（`bin/zerg` 不在盘 · 矩阵不在盘 / 读不回来 · 现跑退码非 0 · 输出零行）

用法
----
    python3 scripts/gates/check-help-shape.py
    python3 scripts/gates/check-help-shape.py --repo-root DIR --bin FILE --matrix FILE
    python3 scripts/gates/check-help-shape.py --emit-baseline   # 现跑打印常量新值（只读）
    python3 scripts/gates/check-help-shape.py --json
    python3 scripts/gates/check-help-shape.py --list-rules
    python3 scripts/gates/check-help-shape.py --self-test       # 合成夹具（正控 + 负控 + 对界）
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import subprocess
import sys

VERSION = "1.3.0"

MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")

# 段头形如：`族级面（<现算族数> 族 · 不必猜族名 · 逐族 `zerg help <族名>` 列该族全部动作）:`
#   族数**现读 · 禁手写** = `families_of(命令树)` 的长度（段头括注的那个数就照它判）。
SEC_HEADER_RE = re.compile(r"^族级面（(\d+) 族 ·.*）:$")
SEC_HEADER_PREFIX = "族级面（"
# 用法行前缀（与既有 Go 判据同一形态：两空格 + `zerg ` + 非空）
ROW_RE = re.compile(r"^\s{2}zerg\s+\S")

# ── 危险档面（`help dangerous` · S4）的形状件 ──
# 逐条行：`  zerg <路径>  D2|D3  --dry-run · [--confirm=<目标> · ]--yes  <它会动什么>  [出处]  ← 标记`
DANGER_ROW_RE = re.compile(
    r"^\s{2}zerg\s+(?P<path>[a-z][a-z0-9-]*(?:\s+[a-z][a-z0-9-]*)*)\s+"
    r"(?P<level>D[23])\s+"
    r"(?P<flags>--dry-run · (?:--confirm=<[^>]*> · )?--yes)\s+")
DANGER_TAIL_RE = re.compile(r"^上表 (\d+) 条：\*\*已开放 (\d+) 条\*\*.*\*\*未开放 (\d+) 条\*\*")
DANGER_CONFIRM_RE = re.compile(r"--confirm=<([^>]*)>")
DANGER_PAREN_RE = re.compile(r"（[^）]*）")
DANGER_OT_RE = re.compile(r"← \*\*已开放\*\*|← 本版未开放")
DANGER_OPEN_MARK = "← **已开放**"
DANGER_CLOSED_MARK = "← 本版未开放"

# ── 配置主题面（`help config` · S5）的形状件 ──
# 优先级链行：恰好两空格缩进 + 非编号条目行 + 至少三段 ` > ` 分隔（现读 `help config` / 裸 `help`
# 各恰好 1 条：`  旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认`）。
CFG_CHAIN_RE = re.compile(r"^  \S.*? > .+ > .+$")
# 逐级说明编号条目：`  1. **旗标**            本次调用…`
CFG_LEVEL_RE = re.compile(r"^  (\d+)\. \*\*(.+?)\*\*")
CFG_CHAIN_SPLIT = " > "

# 布局常量（源件 `core/cmd/zerg/main.go:3380` 一带）：族名清单段的**段头占 1 行**（段头之前必须另有 1 行空行）
# · 族名每行 8 族
SECTION_SCAFFOLD = 1
FAMILY_PER_LINE = 8

# ★ 精确闭集常量 —— **现跑取数**（禁手写）：
#   取法 · 1) lines/bytes：`./bin/zerg help --all | wc -l -c` · `./bin/zerg help | wc -l -c`
#          2) last：`./bin/zerg help | tail -n 1`（逐字）
#   刷新：本件 `--emit-baseline` 现跑打印，照抄。
BASELINE = {
    "help --all": {"lines": 278, "bytes": 130678},
    "help": {"lines": 222, "bytes": 89887,
             "last": "      非交互用 `--token-stdin`；详见 `zerg help credentials`）。"},
    # ★ 危险档面（S4 · 2026-09-27 补）：**现跑取数** · 禁手写 ——
    #   取法 = `./bin/zerg help dangerous | wc -l -c`（lines/bytes）· 逐条行数（`rows`）·
    #          末行逐字（`tail`）· 现算的「`--confirm` 核心名不在该命令用法串里」的条目名单
    #          （`confirm_not_in_usage` · **处数现读 · 禁手写**，已知脱钩照实冻结，不是白名单）·
    #          两面**同印** `--confirm=` 的值冲突名单（`confirm_value_mismatch` · **处数现读 · 禁手写** ——
    #          两面同印的 `--confirm=` 值**条数一律现读 · 禁手写**（本件 `--json` 的
    #          `danger_confirm_both_faces` 格 = 现跑自报的「两面同印 `--confirm=` 的 N 条值」），
    #          全部逐字相等；照实冻结、现跑取数）。
    #   刷新 = `--emit-baseline`（现跑打印 · 照抄 · 别手写）。
    "help dangerous": {"lines": 58, "bytes": 15054, "rows": 47,
                       "tail": "上表 47 条：**已开放 19 条**（确认档齐就真执行 —— 逐条见上行标记）；**未开放 28 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。",
                       "confirm_not_in_usage": ["agent reap", "itask start", "itask stop"],
                       "confirm_value_mismatch": []},
    # ★ 配置主题面（S5 · 2026-09-27 补 · 缺口 `GAP-20260927-440`）：**现跑取数** · 禁手写 ——
    #   取法 = `./bin/zerg help config | wc -l -c`（lines/bytes）；刷新 = `--emit-baseline`。
    #   来源件 = `core/cmd/zerg/exitcodes.go` 的 `helpConfig()`（主题表 `topics.go` 分派）。
    "help config": {"lines": 17, "bytes": 1392},
}


# ── 现跑 / 读文件（都不是判据，只负责取数）──
def run_face(binz: str, argv):
    if not os.path.isfile(binz):
        return None, "`%s` 不在盘" % binz
    p = subprocess.run([binz] + list(argv), capture_output=True, text=True, stdin=subprocess.DEVNULL)
    if p.returncode != 0:
        return None, "现跑 `%s` 退码 %d" % (" ".join(argv), p.returncode)
    if not p.stdout.strip():
        return None, "现跑 `%s` 零输出" % " ".join(argv)
    return p.stdout, ""


def matrix_commands(path: str):
    if not os.path.isfile(path):
        return None, "命令树矩阵不在盘：%s" % path
    try:
        with open(path, encoding="utf-8") as f:
            obj = json.load(f)
    except (OSError, ValueError) as e:
        return None, "命令树矩阵读不回来（%s）" % e
    cmds = sorted({str(c.get("command", "")).strip() for c in (obj.get("cases") or [])
                   if str(c.get("command", "")).strip()})
    if not cmds:
        return None, "命令树矩阵里一条 `command` 都没有（矩阵被改坏 / 换口径了？）"
    return cmds, ""


# ── 纯函数：形状抽取 ──
def rows_of(text: str):
    return [l for l in text.splitlines() if ROW_RE.match(l)]


def family_section(text: str):
    """⇒ (段头行号 | None, 段头括注的族数 | None, token 表, 段行数 | None)。"""
    lines = text.splitlines()
    idx = [i for i, l in enumerate(lines) if l.startswith(SEC_HEADER_PREFIX)]
    if not idx:
        return None, None, [], None
    i = idx[0]
    m = SEC_HEADER_RE.match(lines[i].strip())
    num = int(m.group(1)) if m else None
    body = lines[i + 1:]
    toks = [t.strip() for l in body for t in l.split("·") if t.strip()]
    return i, num, toks, len(lines) - i


def families_of(cmds):
    return sorted({c.split()[0] for c in cmds if c.split()})


# ── 纯函数：危险档面（S4）的形状抽取 ──
def danger_rows(text: str):
    return [l for l in text.splitlines() if ROW_RE.match(l)]


def danger_tail(text: str):
    """末非空行（表尾）⇒ (原行, N|None, X|None, Y|None)。"""
    lines = [l for l in text.splitlines() if l.strip()]
    if not lines:
        return "", None, None, None
    last = lines[-1]
    m = DANGER_TAIL_RE.match(last.strip())
    if not m:
        return last, None, None, None
    return last, int(m.group(1)), int(m.group(2)), int(m.group(3))


def usage_index(all_text: str):
    """`help --all` 的用法行 ⇒ {命令路径: 整行}（取首个命中 · 用法串的另一处来源）。"""
    out = {}
    for l in all_text.splitlines():
        m = re.match(r"^\s{2}zerg\s+([a-z][a-z0-9-]*(?:\s+[a-z][a-z0-9-]*)*)\s", l)
        if m:
            out.setdefault(m.group(1), l)
    return out


def danger_confirm_gap(danger_text: str, all_text: str):
    """现算「印了 `--confirm=<目标>` 但目标核心名**不在**该命令用法串里」的条目名单（字典序）。"""
    used = usage_index(all_text)
    gap = []
    for l in danger_rows(danger_text):
        m = DANGER_ROW_RE.match(l)
        if not m:
            continue
        c = DANGER_CONFIRM_RE.search(m.group("flags"))
        if not c:
            continue
        core = DANGER_PAREN_RE.sub("", c.group(1)).strip().strip("`").strip()
        u = used.get(m.group("path"), "")
        if not core or core not in u:
            gap.append(m.group("path"))
    return sorted(set(gap))


def danger_confirm_pairs(danger_text: str, all_text: str):
    """两面**都印** `--confirm=` 的条目 ⇒ [[命令路径, 危险面值, 用法串值]]（字典序）。

    ★ 只收「该命令在 `help --all` 的用法行里也印了 `--confirm=<Y>`」的条目（**条数现读 · 禁手写**
    = 本件 `--json` 的 `danger_confirm_both_faces` 格现跑自报）：
    一面印、一面不印的不是本腿的账（那些归 S4d / 位置参数），本腿不许假装判了。
    """
    used = usage_index(all_text)
    pairs = []
    for l in danger_rows(danger_text):
        m = DANGER_ROW_RE.match(l)
        if not m:
            continue
        c = DANGER_CONFIRM_RE.search(m.group("flags"))
        if not c:
            continue
        uc = DANGER_CONFIRM_RE.search(used.get(m.group("path"), ""))
        if not uc:
            continue
        pairs.append([m.group("path"), c.group(1), uc.group(1)])
    return sorted(pairs)


def danger_confirm_value_mismatch(danger_text: str, all_text: str):
    """现算「两面都印 `--confirm=` 但值不**逐字**相等」的条目 ⇒ [[路径, 危险面值, 用法串值]]（字典序）。

    ★ 与 `danger_confirm_gap` 是**两件事**：那边判「值在不在用法串里」（集合），这边判「两面印的值是不是
    同一个」（值）—— 值冲突但名字集合不变时（本腿要收的病）那边**看不见**。
    """
    return sorted([p for p in danger_confirm_pairs(danger_text, all_text) if p[1] != p[2]])


# ── 纯函数：配置主题面（S5）的形状抽取 ──
def cfg_chain_lines(text: str):
    """现算 `help config` / 裸 `help` 里的优先级链行（返回全表 —— 恰好 1 条才合格）。"""
    return [l for l in text.splitlines() if CFG_CHAIN_RE.match(l)]


def cfg_chain_tokens(line: str):
    """链行 ⇒ 级名表（按 ` > ` 切 · 剥反引号 —— 与 S4d 的剥法同口径）。"""
    return [t.strip().replace("`", "") for t in line.strip().split(CFG_CHAIN_SPLIT)]


def cfg_levels(text: str):
    """逐级说明的编号条目 ⇒ [(编号, 名字)]（名字剥反引号）。"""
    out = []
    for l in text.splitlines():
        m = CFG_LEVEL_RE.match(l)
        if m:
            out.append((int(m.group(1)), m.group(2).strip().replace("`", "")))
    return out


def rebuilt_last_family_line(fams):
    """族名清单末行的**现算重建**（每 8 族一行，取最后一块）。"""
    chunk = fams[(len(fams) // FAMILY_PER_LINE) * FAMILY_PER_LINE:]
    return "  " + " · ".join(chunk)


def judge(all_text: str, bare_text: str, danger_text: str, cmds, baseline=None, config_text=None) -> list:
    """判据本体（**唯一一处** · 纯函数 · 不现跑、不读盘）。

    `config_text` = `help config` 那一面；**生产路径由 `main()` 每跑必传**（见下 S5 段）。
    `None` 只留给**本件自检里那些不带该面的旧格**（S1–S4 那批旧格一字不改 —— **格数以本件
    `self_test` 现读为准**）——
    ★ 这不是「静默不判」的洞：`""`（空串）走的是**判红**那条路，自检有专格钉它。
    """
    bl = BASELINE if baseline is None else baseline
    bad = []
    fams = families_of(cmds)

    # ---- S1 族级段（闭集相等）----
    idx, num, toks, sec_lines = family_section(all_text)
    if idx is None:
        bad.append("S1 族级段：`help --all` 末尾**没有**族级段（段头 `%s` 找不到）—— "
                   "族名清单消失（这正是本件要防的病）" % SEC_HEADER_PREFIX)
    else:
        if num != len(fams):
            bad.append("S1 族级段：段头括注 %r 族 ≠ 现算命令树族数 %d" % (num, len(fams)))
        miss = [f for f in fams if f not in toks]
        extra = [t for t in toks if t not in fams]
        if miss:
            bad.append("S1 族级段：漏了 %d 族（前 5：%s）—— 命令树里有、清单里没有"
                       % (len(miss), " · ".join(miss[:5])))
        if extra:
            bad.append("S1 族级段：多了 %d 项（前 5：%s）—— 清单里有、命令树里没有"
                       % (len(extra), " · ".join(extra[:5])))
        if not miss and not extra and toks != fams:
            bad.append("S1 族级段：token 集合相同但**次序不同**（族名清单要按字典序，"
                       "`help --all` 与命令树的序必须一致）")
        if bare_text and family_section(bare_text)[0] is not None:
            bad.append("S1 边界：裸 `help` 里出现了族级段 —— 族名清单**只许**在 `--all` 这一档末尾")
        prev = all_text.splitlines()[idx - 1] if idx > 0 else None
        if prev != "":
            bad.append("S1 族级段：段头之前那一行不是空行（%r）—— 段的形状漂了" % prev)

    # ---- S2 条数与行数（关系式 + 常量）----
    r_all, r_bare, r_dang = len(rows_of(all_text)), len(rows_of(bare_text)), len(rows_of(danger_text))
    if r_all != r_bare + r_dang:
        bad.append("S2 关系式：rows(`help --all`)=%d ≠ rows(`help`)=%d + rows(`help dangerous`)=%d（= %d）"
                   % (r_all, r_bare, r_dang, r_bare + r_dang))
    if idx is not None:
        want = SECTION_SCAFFOLD + math.ceil(len(fams) / FAMILY_PER_LINE)
        if sec_lines != want:
            bad.append("S2 关系式：族级段 %d 行 ≠ 2 + ceil(%d 族 / %d 族一行) = %d"
                       % (sec_lines, len(fams), FAMILY_PER_LINE, want))
    for face, text in (("help --all", all_text), ("help", bare_text)):
        want = bl.get(face, {})
        got_l, got_b = len(text.splitlines()), len(text.encode("utf-8"))
        if "lines" in want and got_l != want["lines"]:
            bad.append("S2 常量：`%s` 行数 %d ≠ 冻结读数 %d（形状漂了 —— 取法见件头 `BASELINE` 注释）"
                       % (face, got_l, want["lines"]))
        if "bytes" in want and got_b != want["bytes"]:
            bad.append("S2 常量：`%s` 字节 %d ≠ 冻结读数 %d" % (face, got_b, want["bytes"]))

    # ---- S3 末行 ----
    all_last = all_text.splitlines()[-1] if all_text.splitlines() else ""
    want_last = rebuilt_last_family_line(fams)
    if all_last != want_last:
        bad.append("S3 末行：`help --all` 末行 %r ≠ 由族名清单现算重建的 %r" % (all_last, want_last))
    bare_last = bare_text.splitlines()[-1] if bare_text.splitlines() else ""
    want_bare = bl.get("help", {}).get("last")
    if want_bare is not None and bare_last != want_bare:
        bad.append("S3 末行：裸 `help` 末行 %r ≠ 冻结读数 %r" % (bare_last, want_bare))

    # ---- S4 危险档面（`help dangerous` 的形状）----
    dbl = bl.get("help dangerous", {})
    drows = danger_rows(danger_text)
    tail, n_total, n_open, n_closed = danger_tail(danger_text)
    if n_total is None:
        bad.append("S4a 表尾：`help dangerous` 末非空行不是表尾形状（`上表 N 条：**已开放 X 条**…**未开放 Y 条**`）"
                   "—— 现读 %r（表尾被删 / 改字 / 换口径？）" % tail[:80])
    else:
        if n_total != n_open + n_closed:
            bad.append("S4a 表尾自洽：上表 %d 条 ≠ 已开放 %d + 未开放 %d（= %d）"
                       % (n_total, n_open, n_closed, n_open + n_closed))
        if n_total != len(drows):
            bad.append("S4a 表尾 ⟷ 逐条：表尾称 %d 条 ≠ 逐条行数 %d（两处不许各说各话）"
                       % (n_total, len(drows)))
    n_om, n_cm = danger_text.count(DANGER_OPEN_MARK), danger_text.count(DANGER_CLOSED_MARK)
    if n_open is not None:
        if n_om != n_open:
            bad.append("S4b 逐条标记 ⟷ 表尾：`%s` 出现 %d 次 ≠ 表尾「已开放 %d 条」"
                       % (DANGER_OPEN_MARK, n_om, n_open))
        if n_cm != n_closed:
            bad.append("S4b 逐条标记 ⟷ 表尾：`%s` 出现 %d 次 ≠ 表尾「未开放 %d 条」"
                       % (DANGER_CLOSED_MARK, n_cm, n_closed))
    if n_om + n_cm != len(drows):
        bad.append("S4b 逐条标记：逐条行 %d 条，带标记的只有 %d 条（有行没印 ← 已开放 / ← 本版未开放）"
                   % (len(drows), n_om + n_cm))
    for i, l in enumerate(drows, 1):
        if not DANGER_ROW_RE.match(l):
            bad.append("S4c 逐条行形状：第 %d 条不符（旗标串只许 `--dry-run · [--confirm=<目标> · ]--yes` · "
                       "档位只许 D2/D3）—— %r" % (i, l[:110]))
    gap = danger_confirm_gap(danger_text, all_text)
    want_gap = sorted(dbl.get("confirm_not_in_usage", []))
    if want_gap and gap != want_gap:
        new = [p for p in gap if p not in want_gap]
        fixed = [p for p in want_gap if p not in gap]
        if new:
            bad.append("S4d 旗标串 ⟷ 真实用法串：新增 %d 处脱钩（印了 `--confirm=<目标>`，但该命令在 `help --all` "
                       "的用法串里没有这个目标的核心名）—— %s" % (len(new), " · ".join(new[:5])))
        if fixed:
            bad.append("S4d 旗标串 ⟷ 真实用法串：%d 处脱钩**已消失**（缺口真被修了 ⇒ 用 `--emit-baseline` "
                       "刷新冻结名单，别让判据静默漂）—— %s" % (len(fixed), " · ".join(fixed[:5])))
    vm = [list(x) for x in danger_confirm_value_mismatch(danger_text, all_text)]
    want_vm = sorted([list(x) for x in dbl.get("confirm_value_mismatch", [])])
    if vm != want_vm:
        new = [x for x in vm if x not in want_vm]
        gone = [x for x in want_vm if x not in vm]
        if new:
            bad.append("S4f 两面同印 `--confirm=` ⟷ 值须逐字相等：新增 %d 处**值冲突**（危险面印的名字 ≠ "
                       "用法串里印的名字 ⇒ 用户照哪面敲都可能被拒）—— %s"
                       % (len(new), " · ".join("%s（危险面 `%s` ≠ 用法串 `%s`）" % (x[0], x[1], x[2])
                                                for x in new[:5])))
        if gone:
            bad.append("S4f 两面同印 `--confirm=` ⟷ 值须逐字相等：%d 处值冲突**已消失**（缺口真被修了 ⇒ 用 "
                       "`--emit-baseline` 刷新冻结值，别让判据静默漂）—— %s"
                       % (len(gone), " · ".join("%s（危险面 `%s` ⟷ 用法串 `%s`）" % (x[0], x[1], x[2])
                                                for x in gone[:5])))
    for key, got in (("lines", len(danger_text.splitlines())),
                     ("bytes", len(danger_text.encode("utf-8"))),
                     ("rows", len(drows))):
        if key in dbl and got != dbl[key]:
            bad.append("S4e 常量：`help dangerous` %s %d ≠ 冻结读数 %d（该面漂了 —— 取法见件头 `BASELINE` 注释）"
                       % (key, got, dbl[key]))
    if "tail" in dbl and tail != dbl["tail"]:
        bad.append("S4e 常量：`help dangerous` 表尾逐字 %r ≠ 冻结读数 %r" % (tail, dbl["tail"]))

    # ---- S5 配置主题面（`help config` 的形状 · 2026-09-27 补 · 缺口 `GAP-20260927-440`）----
    if config_text is not None:
        cb = bl.get("help config", {})
        chains = cfg_chain_lines(config_text)
        chain = None
        if len(chains) != 1:
            bad.append("S5a 优先级链：`help config` 里那条链行现读 %d 条（期望恰好 1 条 —— "
                       "链被删 / 被拆行 / 被抄出第二份？）" % len(chains))
        else:
            chain = chains[0]
        lv = cfg_levels(config_text)
        nums = [n for n, _ in lv]
        if nums != list(range(1, len(nums) + 1)):
            bad.append("S5a 逐级说明：编号必须是 `1..N` 连号无重复 —— 现读 %s"
                       % (" · ".join(str(n) for n in nums) if nums else "一条都没有"))
        if chain is not None:
            toks = cfg_chain_tokens(chain)
            names = [n for _, n in lv]
            if names != toks:
                bad.append("S5a 同面交叉：优先级链 %d 级（%s）≠ 逐级说明 %d 条（%s）—— "
                           "同一个面上的两处必须同序逐字相等"
                           % (len(toks), " · ".join(toks), len(names), " · ".join(names)))
            # S5b 同源跨面：裸 `help` 那条链行（同源 = `helptext.go` 的 `configPriorityLine`）
            hchains = cfg_chain_lines(bare_text) if bare_text else []
            if len(hchains) != 1:
                bad.append("S5b 同源跨面：裸 `help` 里那条同源链行现读 %d 条（期望恰好 1 条 —— "
                           "真源常量 `configPriorityLine` 的那一行只能有一处出口）" % len(hchains))
            elif hchains[0] != chain:
                bad.append("S5b 同源跨面：`help config` 的链行 %r ≠ 裸 `help` 的链行 %r（两处同源 "
                           "= `helptext.go` 的 `configPriorityLine` 常量 ⇒ 字面必须逐字相等；"
                           "分家 = 有人把链抄成了第二份）" % (chain, hchains[0]))
        for key, got in (("lines", len(config_text.splitlines())),
                         ("bytes", len(config_text.encode("utf-8")))):
            if key in cb and got != cb[key]:
                bad.append("S5c 常量：`help config` %s %d ≠ 冻结读数 %d（该面漂了 —— "
                           "取法见件头 `BASELINE` 注释 · `--emit-baseline` 刷）" % (key, got, cb[key]))
    return bad


# ── 夹具（合成 · 不碰真目标）──
FIX_CMDS = ["agent ls", "gap ls", "gate run", "help", "version",
            "watch ls", "agent reap", "agent load", "model start", "task pause", "core restart"]

# 危险档夹具逐条（路径 · 档 · 印出的 `--confirm` 目标 · 该命令在 `--all` 里的用法行 · 行尾标记）
# ★ `agent reap` 的用法串里**故意没有**「机器名」⇒ 夹具自带 1 处已知脱钩（供 S4d 的成对负控）。
FIX_DANGER = [
    ("agent reap", "D3", "机器名",
     "  zerg agent reap --dry-run --min-age-days <n>   # 干跑单（带 plan_id）", "← 本版未开放"),
    ("agent load", "D2", "机器名",
     "  zerg agent load <机器名> <模型> --yes [--dry-run]   装模型", "← 本版未开放"),
    ("model start", "D2", "模型 id",
     "  zerg model start <模型 id> --yes [--dry-run]   装载", "← 本版未开放"),
    ("task pause", "D2", "任务 id",
     "  zerg task pause <任务 id> --yes [--dry-run]   暂停", "← 本版未开放"),
    # ★ 这一条的用法串里**故意多一个别的名字**（`--host <机器名>`，与 `--confirm=<主机名>` 等长 = 9 字节）——
    #   供 S4f 的负控⑨：把危险面那格的名字换成 `机器名` ⇒ 名字**仍在**该命令的用法串里（S4d 绿 · 字节常量
    #   也看不见），只有「两面同印的值须逐字相等」这条腿抓得住。
    ("core restart", "D3", "主机名",
     "  zerg core restart --host <机器名> --confirm=<主机名> --yes [--dry-run]   停起主控",
     "← **已开放**（确认档齐就真执行）"),
]
FIX_TAIL = ("上表 5 条：**已开放 1 条**（确认档齐就真执行 —— 逐条见上行标记）；"
            "**未开放 4 条**（真跑一律**拒执**、退码 2 = 不给结论，只有 `--dry-run` 出计划件 = 退码 0）。")

# ★ 配置主题面（S5）夹具：链行 ⟷ 逐级说明必须**同序逐字相等**（两边剥反引号）——
#   链行与裸 `help` 里那一行**同源**（真件 = `helptext.go` 的 `configPriorityLine`），
#   所以夹具的裸 `help` 面里也必须有**同一条**链行（否则 S5b 的正控就不成立）。
FIX_CHAIN = "旗标 > ZERG_* 环境变量 > 项目 .env > ~/.zerg/config.yaml > 内置默认"
# 逐级说明的五条（反引号只在**说明侧** —— 与真件同形：真件链行没有反引号，条目名有）。
FIX_CFG_LEVELS = ["旗标", "`ZERG_*` 环境变量", "项目 `.env`", "`~/.zerg/config.yaml`", "内置默认"]


def fix_config_text():
    """现造 `help config` 那面（真件的形状：链行 + 编号条目 + 凭据条目）。"""
    body = ["配置优先级（契约 §八 · §4.1 `K10` · clig.dev 五级序）：", "",
            "  " + FIX_CHAIN, "", "逐级说明（左高右低，高者覆盖低者）："]
    body += ["  %d. **%s**  夹具第 %d 级" % (i, n, i) for i, n in enumerate(FIX_CFG_LEVELS, 1)]
    body += ["", "凭据（夹具）：", "  · 夹具凭据一", "  · 夹具凭据二"]
    return "\n".join(body) + "\n"


def fix_danger_rows():
    return ["  zerg " + p + "  " + lvl + "  --dry-run · --confirm=<" + tgt + "> · --yes  它会动什么  [出处]  " + mark
            for p, lvl, tgt, _u, mark in FIX_DANGER]


def fix_texts():
    """按真件形态现造四面文本（族名清单 = 字典序 · 每 8 族一行；危险档面带逐条标记 + 表尾；
    配置主题面 = 链行 + 编号条目 + 凭据条目）。"""
    fams = families_of(FIX_CMDS)
    rows_bare = ["  zerg " + c + " ｜ 茧壁:host ｜ 说明" for c in FIX_CMDS[:5]]
    rows_all = rows_bare + [u for _p, _l, _t, u, _m in FIX_DANGER]
    all_text = ("zerg —— 形状夹具 · 深度 ≤ 3 层\n\n" + "\n".join(rows_all)
                + "\n\n族级面（%d 族 · 不必猜族名 · 逐族 `zerg help <族名>` 列该族全部动作）:\n" % len(fams)
                + "\n".join("  " + " · ".join(fams[i:i + FAMILY_PER_LINE])
                            for i in range(0, len(fams), FAMILY_PER_LINE)) + "\n")
    # ★ 裸 `help` 面里放**同一条**链行（S5b 的正控就靠它 —— 与真件同形：真件两处同源同一常量）。
    bare_text = ("zerg —— 形状夹具\n\n" + "\n".join(rows_bare)
                 + "\n配置优先级:\n  " + FIX_CHAIN + "\n" + BASELINE["help"]["last"] + "\n")
    dang_text = "\n".join(fix_danger_rows()) + "\n\n" + FIX_TAIL + "\n"
    return all_text, bare_text, dang_text, fix_config_text()


def self_test(out) -> bool:
    total, failed = 0, 0

    def case(name, want_red, bad, want_sub=None, forbid_sub=None):
        """`want_sub` = 判词里**必须**出现的子串（点名到具体那条腿）· `forbid_sub` = 必须**不**出现
        （成对控制：证明这一格是**哪条腿**在响，而不是别的腿顺手一起红）。"""
        nonlocal total, failed
        total += 1
        got_red = bool(bad)
        ok = got_red == want_red
        if ok and want_sub is not None and not any(want_sub in b for b in bad):
            ok = False
        if ok and forbid_sub is not None and any(forbid_sub in b for b in bad):
            ok = False
        if not ok:
            failed += 1
        out.write("  %s %s（期望%s · 实得%s）\n"
                  % ("✓" if ok else "✗", name, "红" if want_red else "绿", "红" if got_red else "绿"))
        for b in bad[:3]:
            out.write("      ↳ %s\n" % b)

    all_t, bare_t, dang_t, cfg_t = fix_texts()
    d_rows = danger_rows(dang_t)
    # 夹具基线（否则常量格必红 ⇒ 干扰判别）：只钉夹具自己的数
    bl = {"help --all": {"lines": len(all_t.splitlines()), "bytes": len(all_t.encode())},
          "help": {"lines": len(bare_t.splitlines()), "bytes": len(bare_t.encode()),
                   "last": BASELINE["help"]["last"]},
          "help config": {"lines": len(cfg_t.splitlines()), "bytes": len(cfg_t.encode())},
          "help dangerous": {"lines": len(dang_t.splitlines()), "bytes": len(dang_t.encode()),
                             "rows": len(d_rows), "tail": FIX_TAIL,
                             "confirm_not_in_usage": ["agent reap"],
                             "confirm_value_mismatch": []}}

    case("正控：原样 ⇒ 必绿", False, judge(all_t, bare_t, dang_t, FIX_CMDS, bl))
    # 负控（危险档面 S4 · 等长换字 ⇒ 字节常量**看不见**，只有语义判据抓得住）
    case("负控⑤：改一条的 `--confirm` 目标（等长换字 · 字节不变）⇒ 必红", True,
         judge(all_t, bare_t, dang_t.replace("--confirm=<模型 id>", "--confirm=<主机 id>"), FIX_CMDS, bl))
    case("负控⑥：表尾计数对不上（等长改数）⇒ 必红", True,
         judge(all_t, bare_t, dang_t.replace("上表 5 条", "上表 6 条"), FIX_CMDS, bl))
    case("负控⑦：删掉表尾一行 ⇒ 必红", True,
         judge(all_t, bare_t, "\n".join(dang_t.splitlines()[:-2]) + "\n", FIX_CMDS, bl))
    case("负控⑧：抹掉一条的 `--yes`（旗标串破形）⇒ 必红", True,
         judge(all_t, bare_t, dang_t.replace("--dry-run · --confirm=<主机名> · --yes",
                                             "--confirm=<主机名> --yes"), FIX_CMDS, bl))

    # 负控（S4f 的值冲突面 · 2026-09-27 补 · 缺口 GAP-20260927-423）
    case("负控⑨：两面同印 `--confirm=` 但值不等（等长换字 · 名字仍在用法串里 ⇒ S4d/字节常量都看不见）⇒ S4f 必红",
         True,
         judge(all_t, bare_t,
               dang_t.replace("zerg core restart  D3  --dry-run · --confirm=<主机名>",
                              "zerg core restart  D3  --dry-run · --confirm=<机器名>"),
               FIX_CMDS, bl), want_sub="S4f", forbid_sub="S4d")
    case("负控⑩：危险面值挪出用法串（名字集合变化 = S4d 的账）⇒ 旧 S4d 仍必红、S4f 不许抢账", True,
         judge(all_t, bare_t,
               dang_t.replace("zerg agent load  D2  --dry-run · --confirm=<机器名>",
                              "zerg agent load  D2  --dry-run · --confirm=<主机名>"),
               FIX_CMDS, bl), want_sub="S4d", forbid_sub="S4f")
    case("对界：夹具正控上 S4f 不许误报（两面同印的值逐字相等 · 现读 1 条）⇒ 绿且无 S4f 断言", False,
         judge(all_t, bare_t, dang_t, FIX_CMDS, bl), forbid_sub="S4f")

    # 负控（S5 配置主题面 · 2026-09-27 补 · 缺口 GAP-20260927-440）
    bad_chain = "  " + FIX_CHAIN.replace("内置默认", "内置缺省")
    case("正控②：配置面原样（链 ⟷ 逐级说明同序 · 与裸 `help` 同源）⇒ 必绿且不许有 S5 断言", False,
         judge(all_t, bare_t, dang_t, FIX_CMDS, bl, cfg_t), forbid_sub="S5")
    case("负控⑪：只改链行一个字（`内置默认`→`内置缺省` · **等长换字** ⇒ 字节常量看不见）"
         "⇒ S5a/S5b 必红、S5c 不许响", True,
         judge(all_t, bare_t, dang_t, FIX_CMDS, bl, cfg_t.replace("  " + FIX_CHAIN, bad_chain)),
         want_sub="S5a", forbid_sub="S5c")
    case("负控⑫：只改**裸 `help`**那条同源链行（配置面一字不动）⇒ S5b 必红、S5a 不许抢账", True,
         judge(all_t, bare_t.replace(FIX_CHAIN, FIX_CHAIN.replace("内置默认", "内置缺省")),
               dang_t, FIX_CMDS, bl, cfg_t), want_sub="S5b", forbid_sub="S5a")
    case("负控⑬：配置面掉一条逐级说明 ⇒ S5a 必红（编号连号 ⟷ 链级数）", True,
         judge(all_t, bare_t, dang_t, FIX_CMDS, bl,
               "\n".join(cfg_t.splitlines()[:5] + cfg_t.splitlines()[6:]) + "\n"), want_sub="S5a")
    case("对界：配置面喂**空串** ⇒ 必红（证明「这个面没了」不是静默绿）", True,
         judge(all_t, bare_t, dang_t, FIX_CMDS, bl, ""), want_sub="S5a")

    lines = all_t.splitlines()
    idx = [i for i, l in enumerate(lines) if l.startswith(SEC_HEADER_PREFIX)][0]
    # 负控 ①：删掉族名清单段（段前空行 + 段头 + 族名行全删）
    case("负控①：删掉族级段 ⇒ 必红", True,
         judge("\n".join(lines[:idx - 1]) + "\n", bare_t, dang_t, FIX_CMDS, bl))
    # 负控 ②：行数偏移（掉一行族名行）
    case("负控②a：族级段掉一行 ⇒ 必红", True,
         judge("\n".join(lines[:idx + 2] + lines[idx + 3:]) + "\n", bare_t, dang_t, FIX_CMDS, bl))
    case("负控②b：整面多一行 ⇒ 必红", True,
         judge(all_t + "多余一行\n", bare_t, dang_t, FIX_CMDS, bl))
    # 负控 ③：末行改写
    case("负控③a：`--all` 末行改写 ⇒ 必红", True,
         judge("\n".join(lines[:-1] + ["  task · ui · update · version · watch-XX"]) + "\n",
               bare_t, dang_t, FIX_CMDS, bl))
    case("负控③b：裸 `help` 末行改写 ⇒ 必红", True,
         judge(all_t, "\n".join(bare_t.splitlines()[:-1] + ["凭据：改过的一句。"]) + "\n",
               dang_t, FIX_CMDS, bl))
    # 负控 ④：族名清单漏一族（命令树有、清单没有）
    case("负控④：清单漏一族 ⇒ 必红", True,
         judge(all_t.replace("watch", "watchx"), bare_t, dang_t, FIX_CMDS, bl))
    # 对界：命令树漂了（树里多一条命令）⇒ 必红
    case("对界：命令树多一族 ⇒ 必红", True,
         judge(all_t, bare_t, dang_t, FIX_CMDS + ["zzz ls"], bl))

    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


LIST_RULES = """判据（三条 · 现算 + 对界 · 缺口 GAP-20260927-401）
-----------------------------------------------
  S1 族级段（闭集相等）：现跑 `help --all` 末尾段 token ⟷ 命令树（`cli-matrix.json` 的
     `cases[].command` 首段）· 段头在 · 括注族数 == 现算族数 · 双向差集空 · 裸 `help` 不带段
  S2 条数与行数：rows(--all) == rows(help) + rows(help dangerous) ·
     族级段行数 == 1 + ceil(族数/8)（段头之前必须是空行）；两个面的 lines/bytes == 冻结常量
     （现跑取数 · --emit-baseline 刷）
  S3 末行：`--all` 末行 == 由族名清单现算重建的那一行；裸 `help` 末行 == 冻结读数（逐字）
  S4 危险档面（`help dangerous` · 2026-09-27 补 · 缺口 GAP-20260927-418）：
     S4a 表尾计数（`上表 N 条：已开放 X / 未开放 Y` ⇒ N == X + Y == 逐条行数）·
     S4b 逐条行尾标记出现次数 == 表尾 X / Y ·
     S4c 逐条行形状闭集（`zerg <路径>  D2|D3  --dry-run · [--confirm=<目标> · ]--yes  …  ← 标记`）·
     S4d 逐条 `--confirm=<目标>` 的核心名 ⟷ 该命令在 `help --all` 的用法串（现算脱钩集合 == 冻结名单）·
     S4f 两面**同印** `--confirm=` 的值须**逐字相等**（现算值冲突集合 == 冻结名单 · 现读空集 —— 独立一条腿：
         S4d 只判「名字在不在用法串里」⇒ 值冲突但名字集合不变时 S4d 看不见）·
     S4e 该面的 lines / bytes / 条数 / 表尾逐字 == 冻结读数（现跑取数 · --emit-baseline 刷）
  S5 配置主题面（`help config` · 2026-09-27 补 · 缺口 GAP-20260927-440）：
     S5a 同面交叉：优先级链行现算的级名表 == 逐级说明 `N. **名字**` 的名字表（同序逐字 · 两边剥反引号）·
         编号必须 `1..N` 连号无重复 · 链行恰好 1 条 ·
     S5b 同源跨面：`help config` 的链行**逐字等于**裸 `help` 的那一行（同源 = `helptext.go` 的
         `configPriorityLine` 常量 —— 分家 = 有人抄了第二份）·
     S5c 该面的 lines / bytes == 冻结读数（现跑取数 · --emit-baseline 刷）

不假装覆盖（照实登记）
----------------------
  · 只判**形状**（段 / 条数 / 行数 / 末行）；段的**说明文字**逐字、危险档判词、层级列不在本格
  · 两态 rc 语义（`zerg <族> --help` 退 2 等）**不在本件**（那是 `cli_help_twostate_test.go` 的面，禁碰）
  · `help config` 只判**形状/结构**：链 ⟷ 逐级说明同序 · 同源链行逐字 · lines/bytes 常量；
    「凭据」那批条目的**文字**与主题表 `topics.go` 的 `Brief` 串同源不同源**不在本格**
    （要另开一处读 Go 源的口径 —— 本单不开第三份口径）
  · 建件时本件**未挂门**（挂门 + `scripts/公开标记.tsv` + `dryrun-semantics.json` 三处登记是随动口）
"""


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True, description="`help` 面的形状判据（族级段 / 条数 / 末行）")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--bin", default=None)
    ap.add_argument("--matrix", default=None)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--emit-baseline", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        sys.stdout.write("── 自检（合成夹具 · 不碰真目标）──\n")
        return 0 if self_test(sys.stdout) else 2

    binz = a.bin or os.path.join(a.repo_root, "bin", "zerg")
    matrix = a.matrix or os.path.join(a.repo_root, MATRIX_REL)

    cmds, cwhy = matrix_commands(matrix)
    if cmds is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % cwhy)
        return 2
    faces = {}
    for face, argv in (("help --all", ["help", "--all"]), ("help", ["help"]),
                       ("help dangerous", ["help", "dangerous"]), ("help config", ["help", "config"])):
        text, why = run_face(binz, argv)
        if text is None:
            sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % why)
            return 2
        faces[face] = text

    if a.emit_baseline:
        for face, text in faces.items():
            sys.stdout.write("%s\tlines=%d\tbytes=%d\tlast=%s\n"
                             % (face, len(text.splitlines()), len(text.encode("utf-8")),
                                text.splitlines()[-1] if text.splitlines() else ""))
        d = faces["help dangerous"]
        drows = danger_rows(d)
        _t, _n, _x, _y = danger_tail(d)
        sys.stdout.write(
            '  # ↑ 前四行照抄进 BASELINE。危险档面（S4）那一条照抄下面这三行；配置主题面（S5）'
            '照抄末行给出的那一条：\n'
            '    "help dangerous": {"lines": %d, "bytes": %d, "rows": %d,\n'
            '                       "tail": %r,\n'
            '                       "confirm_not_in_usage": %s,\n'
            '                       "confirm_value_mismatch": %s},\n'
            % (len(d.splitlines()), len(d.encode("utf-8")), len(drows),
               danger_tail(d)[0], json.dumps(danger_confirm_gap(d, faces["help --all"]), ensure_ascii=False),
               json.dumps(danger_confirm_value_mismatch(d, faces["help --all"]), ensure_ascii=False)))
        cf = faces["help config"]
        sys.stdout.write('    "help config": {"lines": %d, "bytes": %d},\n'
                         % (len(cf.splitlines()), len(cf.encode("utf-8"))))
        return 0

    sys.stdout.write("`help` 面形状判据 · v%s（族级段 / 条数 / 末行 / 危险档 / 配置主题）\n" % VERSION)
    fams = families_of(cmds)
    idx, num, toks, sec_lines = family_section(faces["help --all"])
    sys.stdout.write("  命令树 %d 条 ⇒ 现算 %d 族 · `help --all` %d 行 / %d 字节 · 裸 `help` %d 行 · "
                     "族级段 %s 行（token %d）\n"
                     % (len(cmds), len(fams), len(faces["help --all"].splitlines()),
                        len(faces["help --all"].encode("utf-8")), len(faces["help"].splitlines()),
                        sec_lines if idx is not None else "缺", len(toks)))
    bad = judge(faces["help --all"], faces["help"], faces["help dangerous"], cmds,
                config_text=faces["help config"])
    if bad:
        sys.stdout.write("⇒ 判红（rc=1）：\n")
        for b in bad:
            sys.stdout.write("  ✗ %s\n" % b)
    else:
        sys.stdout.write("  ✓ S1 族级段 %d 族 ⟷ 命令树逐 token 相等（裸 `help` 不带段）\n" % len(fams))
        sys.stdout.write("  ✓ S2 rows(--all)=%d == rows(help)=%d + rows(dangerous)=%d（危险档净增）；"
                         "两个面行/字节 == 冻结读数（段 %d 行 · 命令树 %d 条为族名另一源）\n"
                         % (len(rows_of(faces["help --all"])), len(rows_of(faces["help"])),
                            len(rows_of(faces["help dangerous"])),
                            sec_lines if sec_lines is not None else -1, len(cmds)))
        sys.stdout.write("  ✓ S3 末行 = 族名清单现算重建的那一行（裸 `help` 末行逐字符合）\n")
        _t, _n, _x, _y = danger_tail(faces["help dangerous"])
        sys.stdout.write("  ✓ S4 危险档面：表尾 %d 条 == 已开放 %d + 未开放 %d == 逐条行数 %d；逐条标记逐数相符；"
                         "逐条行形状全合；`--confirm` 目标 ⟷ 用法串脱钩 %d 处（= 冻结名单）；"
                         "两面同印 `--confirm=` 的 %d 条值逐字相等（值冲突 %d 处 = 冻结值）；"
                         "行/字节/条数 == 冻结读数 %d/%d/%d\n"
                         % (_n if _n is not None else -1, _x if _x is not None else -1,
                            _y if _y is not None else -1, len(danger_rows(faces["help dangerous"])),
                            len(danger_confirm_gap(faces["help dangerous"], faces["help --all"])),
                            len(danger_confirm_pairs(faces["help dangerous"], faces["help --all"])),
                            len(danger_confirm_value_mismatch(faces["help dangerous"], faces["help --all"])),
                            dbl_l := len(faces["help dangerous"].splitlines()),
                            dbl_b := len(faces["help dangerous"].encode("utf-8")),
                            dbl_r := len(danger_rows(faces["help dangerous"]))))
        cf = faces["help config"]
        sys.stdout.write("  ✓ S5 配置主题面：链行 ⟷ 逐级说明 %d 级同序逐字相等；链行 ⟷ 裸 `help` 同源逐字；"
                         "行/字节 == 冻结读数 %d/%d\n"
                         % (len(cfg_chain_tokens(cfg_chain_lines(cf)[0])) if cfg_chain_lines(cf) else -1,
                            len(cf.splitlines()), len(cf.encode("utf-8"))))
        sys.stdout.write("⇒ 五条都合（rc=0）\n")
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-help-shape", "version": VERSION,
            "commands": len(cmds), "families": len(fams),
            "all_lines": len(faces["help --all"].splitlines()),
            "all_bytes": len(faces["help --all"].encode("utf-8")),
            "bare_lines": len(faces["help"].splitlines()),
            "family_section_lines": sec_lines, "family_tokens": len(toks),
            "danger_lines": len(faces["help dangerous"].splitlines()),
            "danger_bytes": len(faces["help dangerous"].encode("utf-8")),
            "danger_rows": len(danger_rows(faces["help dangerous"])),
            "danger_confirm_not_in_usage": danger_confirm_gap(faces["help dangerous"], faces["help --all"]),
            "danger_confirm_both_faces": len(danger_confirm_pairs(faces["help dangerous"], faces["help --all"])),
            "danger_confirm_value_mismatch": danger_confirm_value_mismatch(faces["help dangerous"], faces["help --all"]),
            "config_lines": len(faces["help config"].splitlines()),
            "config_bytes": len(faces["help config"].encode("utf-8")),
            "config_chain_lines": len(cfg_chain_lines(faces["help config"])),
            "bad": bad, "mode": "judge",
        }, ensure_ascii=False) + "\n")
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())

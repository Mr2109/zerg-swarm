#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-py-go-pairing-drift.py —— py⟷Go 双实现「两态对拍」防波门（**只读** · 棘轮只许降）

病（承 `zerg-gate-cli-pitfalls` 的「py↔Go 双实现面：只改半件 ＝ 假绿」）：
  `core/cmd/zerg/gate_inproc_table.go` 把一批判据登记成**已真搬 Go · 可在进程内跑**；
  每枚键在**判据注册表**里还指着一份 py/sh 真身（★ 决-10：真身路径查 `scripts/gates/judge-registry.json`
  的 `entries[].path`，✗ 不硬拼旧口径路径 —— 真身分布在 scripts/evals·svc·docs·sandbox-probes…）。`zerg gate check <键>` 走 **Go inproc**，
  而步骤表那一步走 **py** —— 两个入口同源吗？**没有谁保证**。枚举三口径（py 件 / Go 件 /
  inproc 键）可现读；「两半都在 ＝ 风险面」在百量级 ⇒ 分叉是架构面，不是偶发。

治法（本门 · **对拍而不是读源码猜**）：
  对 `gate_inproc_table.go` 的**每条 inproc 键**，拿**同一 argv**跑两态 ——
    态① `bin/zerg gate check <键> <argv…>`（Go 进程内 · 壳 cwd=仓根 · 壳设 ZERG_REPO*）
    态② `python3|bash <注册表真身路径> <argv…>`（真身 · cwd=仓根 · 解释器取注册表 interp）
  ⇒ 断言 **stdout / stderr / 退码**三面逐字同（已声明非确定面**掩码**：时间戳 · 耗时 · 临时目录 · pid）。
  ★ 差集进 **只许下降的棘轮**（`py-go-pairing-drift.ratchet.json` 的 `known` 集）：
     现读差集 ⊆ 棘轮 ⇒ 不涨；出现**棘轮里没有**的键 ⇒ 差集涨了 ⇒ `--strict` 判红。
  另出两张点名表（**键 ⇕ 件** 的双向差集）：
    表A「A 有键 · B 无真身」= inproc 键在册、而判据注册表里无该键真身 ⇒ 真分叉（`--strict` 判红）
    表B「B 有件 · A 无键」= py 真身在盘、而未进 inproc 表 ⇒ **半接线/未搬**（常态 · 只报告）

★ 为什么是**两态对拍**而不是读源码：分叉可以「今天两边同值」时静默存在（版本真源恰等于被钉死的值）；
  判「有没有分叉」**必须造扰动**（换 argv / 换根），✗ 不能拿今日平值当「没分叉」。

★ **只读门**（✗ 不写仓内任何物）：本件只读表 / 只跑子进程 / 只读棘轮；**棘轮由人维护**，
  本件给 `--dump-ratchet`（现读差集打到 stdout · **不写盘**）供人照抄 —— 与 `--emit-baseline` 同族口径。

退码（三档 · fail-closed）
  0 = 判到了（**报告档** · 差集与两张点名表照出 · 棘轮只许降但报告档不阻断）
  1 = 只有 `--strict` 档才可能：① 差集出现棘轮里没有的键（棘轮涨了）② 表A 非空（键无件）
  2 = **不给结论**：inproc 表读不到 / 解析 0 键 / `scripts/gates` 目录读不到 / 棘轮不在盘 /
      棘轮 `probe_argv` 与本次 argv 不符 / `bin/zerg` 不在盘 / 家目录?无 —— 任一取不到输入即 rc=2
      （空域 ≠ 全绿 · 零执行探针一律不给结论）· `--self-test` 用例数对不上也退 2

掩码（已声明的**非确定面** · 逐字口径见 `MASK_RULES`）
  时间戳 `YYYY-MM-DD[ T]HH:MM:SS` · 耗时 `<数>s` / `<数> 秒` / **`<数>ms`（毫秒墙钟）** · 临时目录
  `/var/folders/…` `/tmp/…` `…/T/…` · `pid=<数>` · `tempdir=<串>` ·
  **夹具 mkdtemp 随机后缀 `fx-<类>-<后缀>`** · **夹具 git 对象哈希（7–64 位十六进制）**。
  掩码**只抹这些面**；其余逐字比。
  ★ 本批（批-2.1）新增三类非确定面，**逐键点名**其成因而**不是** py⟷Go 实现分叉（三键改后即从差集消失）：
    · `check-build-tags` ：仓外夹具目录名 `[fx-one-side-<随机后缀>]` —— py `mkdtemp` 出**字母数字**、
      Go `MkdirTemp` 出**纯数字**（py 连跑两发即异）；掩掉后缀后两态逐字同。
    · `check-contract-version-diff` ：自检档的**真目标墙钟** `NN ms` —— 两态各自耗时天然不同；
      旧掩码只认 `s`/`秒` ⇒ `ms` 漏网；掩掉后两态逐字同。
    · `check-multirepo-summary` ：仓外夹具仓的 **git HEAD / index 哈希**（每次现造夹具都不同）；
      掩掉 7–64 位十六进制串后两态逐字同。
  ★ 本批（批-2.1-续 · 2026-10-07）新增两条**掩码面** ＋ 一条**启动口径**（同属「门比声明松」的面 ·
    **逐键点名**其成因而**不是** py⟷Go 实现分叉；三键改后即从差集消失）：
    · `writeout-probe`（**回溯帧面**）：py `os.environ[<键>]` 缺键 ⇒ **裸回溯**，其帧落在**解释器面**
      且随解释器版本/安装位置漂（3.14 `File "<frozen os>", line 709, in __getitem__` ⟷ 系统
      `/usr/bin/python3` 3.9 `…/lib/python3.9/os.py", line 679, in __getitem__`）⇒ 逐字复刻＝把某版
      解释器行号**写死进仓件**（换环境必红）。MASK_RULES 新增三条：① **帧行**
      （`File "…", line N…` / `File "<frozen …>", line N…`）⇒ 固定占位符 `<TRACE_FRAME>`；
      ② **回溯脚手架**（`Traceback (most recent call last):` 头行 ＋ 其下缩进的帧/源码/`^^^` caret 行）
      ⇒ **折叠掉**（★ 只掩帧行**不足**：`^^^` caret 行 **py≥3.11 才有** ⇒ 不折叠则 py 自身跨版本即异，
      而 Go 半无对应行）；③ Go 半回溯的**具名文案尾注**（`（py 真身裸回溯…）` · 声明正文不复刻）⇒ 掩掉。
      ⇒ 两侧都只剩末行「**异常类名：正文**」，该行**逐字比**（改一字仍判异 · 见自检⑯）。
    · `gen-gap-ledger`（**解释器告警面**）：py 真身 `scripts/docs/gen-gap-ledger.py:29`（模块 docstring）
      写有 `"\\d"` ⇒ **CPython ≥3.12 编译该源文件时**向 stderr 打 `SyntaxWarning: … invalid escape
      sequence …`（≤3.11 静默；3.12/3.13/3.14 正文还不一样），程序自身从不产它 ⇒ Go 半无对应行。
      MASK_RULES 新增一条：`: SyntaxWarning: ` 行 ＋ 紧随的缩进**源码回显行** ⇒ 掩掉（掩成空 · 见自检⑰）。
    · `writer-lock`（**写工具档启动口径 · 非掩码**）：真身件头 `#zerg-class: write D2` ⇒ 壳体
      `core/cmd/zerg/family_gate_check.go:817–891` 的**写工具危险档闸**令态① 缺 `--yes` 时 fail-closed
      退 2（`yes_required` · 不执行），而态②（裸真身）照跑 ⇒ **结构性判差**（✗ 与 Go 入口无关）。
      治法：`probe()` 对**写类键**（`is_write_class()` 现读真身**件头**声明）在**键名之后**补**壳级**
      `--yes`（壳 `gateCheckStripShellFlags` 在 exec 前**消费**、✗ 不透传子件）⇒ 两态同。
      ★ 非写类键 ✗ 不带 `--yes`（**成对负控** · 见自检⑲/⑳）。

用法
    python3 scripts/gates/check-py-go-pairing-drift.py                    # 报告档 · 真目标
    python3 scripts/gates/check-py-go-pairing-drift.py --strict           # 判红档（棘轮只许降）
    python3 scripts/gates/check-py-go-pairing-drift.py --argv=--self-test # 指定两态 argv（缺省 --self-test）
    python3 scripts/gates/check-py-go-pairing-drift.py --dump-ratchet     # 现读差集打到 stdout（不写盘）
    python3 scripts/gates/check-py-go-pairing-drift.py --json             # 机器面
    python3 scripts/gates/check-py-go-pairing-drift.py --self-test        # 成对正负控 · 仓外夹具 · 不碰真目标
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

VERSION = "0.1.0（防波门 · 缺口承 zerg-gate-cli-pitfalls「双实现面只改半件＝假绿」）"
INPROC_REL = os.path.join("core", "cmd", "zerg", "gate_inproc_table.go")
GATES_REL = os.path.join("scripts", "gates")
# ★ 决-10：B 侧真身路径的**单一真源** = 判据注册表（`entries[].path` 本就带真身路径）。
#   防波门 ✗ 不再硬拼 scripts/gates/<键>.py（真身在 scripts/evals·svc·docs·sandbox-probes… 各面）。
REGISTRY_REL = os.path.join("scripts", "gates", "judge-registry.json")
RATCHET_REL = os.path.join("scripts", "gates", "py-go-pairing-drift.ratchet.json")
SCHEMA = "zerg/py-go-pairing-drift/v1"
DEFAULT_ARGV = ["--self-test"]
PROBE_TIMEOUT = 120          # 单侧单键秒级上限（超时 ⇒ 该键「不判」· 不进棘轮）

# inproc 表里的条目形态：`\t"<键>":   <Go 函数名>,`（★ 只认这一形态 · 表件里其余行不吃）
KEY_RE = re.compile(r'^\t"([^"]+)"\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*$')

# ★ 已声明的**非确定面**（掩码口径 · 只抹这些 · 其余逐字比 · 只增不改）
MASK_RULES = [
    (re.compile(r"\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?"), "<TS>"),
    # ★新增（批-2.1）：毫秒墙钟 —— 旧表只认 `s`/`秒` ⇒ `check-contract-version-diff` 的 `NN ms` 漏网。
    #   必须排在 `s\b` 那条**之前**（`ms` 以 `m` 开头、`s\b` 那条本不匹配它，前置只为口径清晰）。
    (re.compile(r"\d+(?:\.\d+)?\s*ms\b"), "<DUR>"),
    (re.compile(r"\d+(?:\.\d+)?\s*s\b"), "<DUR>"),
    (re.compile(r"\d+(?:\.\d+)?\s*秒"), "<DUR>"),
    (re.compile(r"/var/folders/[^\s\"']+"), "<TMP>"),
    (re.compile(r"/tmp/[^\s\"']+"), "<TMP>"),
    (re.compile(r"/private/[^\s\"']+"), "<TMP>"),
    (re.compile(r"pid[=:]\s*\d+"), "pid=<PID>"),
    (re.compile(r"tempdir[=:]\s*\S+"), "tempdir=<TMP>"),
    # ★新增（批-2.1）夹具目录 mkdtemp 随机后缀：`fx-<类>-<后缀>`（py 字母数字 ⟷ Go 纯数字）。
    #   保留 `<类>` 段（两侧同名）· 只抹后缀 ⇒ `check-build-tags` 的 `[fx-one-side-…]` 归一。
    #   排在十六进制掩码**之前**（后缀可能恰是纯十六进制串，先整体归一免得被当哈希）。
    (re.compile(r"(fx-[a-z]+(?:-[a-z]+)*-)[A-Za-z0-9_]+"), r"\1<FX>"),
    # ★新增（批-2.1）夹具 git 对象哈希：7–64 位十六进制且**至少含一个 a–f**（排除纯十进制计数）。
    #   ⇒ `check-multirepo-summary` 的 8 位短 sha / 40 位 HEAD / index 哈希归一。
    (re.compile(r"\b(?=[0-9a-f]*[a-f])[0-9a-f]{7,64}\b"), "<SHA>"),
    # ★新增（批-2.1）py 裸回溯的**帧行**：`File "<路径>"` / `File "<frozen os>"` ＋ `, line N[, in fn]`。
    #   成因（`writeout-probe` 键）：py `os.environ[<键>]` 缺键 ⇒ 裸回溯，其帧落在**解释器面**且随
    #   解释器版本/安装路径漂（3.14 `File "<frozen os>", line 709, in __getitem__` ⟷ 系统
    #   /usr/bin/python3 3.9 `…/lib/python3.9/os.py", line 679, in __getitem__`）⇒ 逐字复刻＝把某版
    #   解释器的行号/路径**写死进仓件**（换环境必红）⇒ 掩成固定占位符。★ 只掩**帧行本身**。
    (re.compile(r'(?m)^[ \t]*File "[^"]*", line \d+(?:, in [^\n]*)?[ \t]*$'), "<TRACE_FRAME>"),
    # ★新增（批-2.1）回溯**脚手架**（`Traceback (most recent call last):` 头行 ＋ 其下缩进的
    #   帧行/源码行/`^^^` caret 行）—— caret 行 **py≥3.11 才有**（3.9 无）⇒ 不折叠则 py 自身跨版本即异；
    #   Go 半无对应行 ⇒ 整块折叠掉，**只留末行「异常类名：正文」**（该行两侧逐字比 · ✗ 不被掩）。
    #   上一条已把帧行归一为 `<TRACE_FRAME>` ⇒ 本条同时吞占位符行。
    (re.compile(r'(?m)^Traceback \(most recent call last\):[ \t]*\r?\n(?:[ \t][^\n]*\r?\n|<TRACE_FRAME>[ \t]*\r?\n)*'), ""),
    # ★新增（批-2.1）Go 半回溯的「具名文案」尾注（声明 py 回溯正文不可复刻）—— py 侧无此注
    #   ⇒ 掩掉，令两侧都只剩「异常类名：正文」。★ 只掩这条**声明式尾注**，✗ 不掩异常类名/正文本身。
    (re.compile(r'（py 真身裸回溯[^\n]*）'), ""),
    # ★新增（批-2.1）CPython ≥3.12 编译源文件时的 `SyntaxWarning`（`gen-gap-ledger` 键 · py 真身 :29
    #   的模块 docstring 里 `"\d"`）＋ **紧随其后的源码回显行** —— 该告警由**解释器**打印、程序自身从不产它；
    #   ≤3.11 静默、3.12/3.13/3.14 正文还不一样 ⇒ Go 半无对应行 ⇒ 掩掉（掩成空串）。★ 只认
    #   `: SyntaxWarning: ` 这一行 ＋ 紧随的缩进回显行，✗ 不碰其它告警/异常面。
    (re.compile(r'(?m)^[^\n]*: SyntaxWarning: [^\n]*\r?\n(?:[ \t][^\n]*\r?\n)?'), ""),
]


def mask(text: str) -> str:
    for rx, rep in MASK_RULES:
        text = rx.sub(rep, text)
    return text


# ★ 批-2.1（写工具档）：真身件头 `#zerg-class: write D0|D2|D3` 声明（真源 = 件自己那一行 ·
#   与 `check-writer-guard.py:59` / `family_gate_check.go:465` 同串）。只现读前 80 行（同族口径）。
WRITE_CLASS_RE = re.compile(r"^#\s*zerg-class:\s*write\s+(?:D0|D2|D3)\s*$")
WRITE_CLASS_HEAD_LINES = 80


def is_write_class(root: str, rel: str) -> bool:
    """真身件头是否声明 `#zerg-class: write`（⇒ 态① 须补壳级 `--yes`）。取不到 ⇒ 当非写类（✗ 不误加 `--yes`）。"""
    p = os.path.join(root, rel)
    try:
        f = open(p, encoding="utf-8", errors="replace")
    except OSError:
        return False
    try:
        for i, ln in enumerate(f):
            if i >= WRITE_CLASS_HEAD_LINES:
                break
            if WRITE_CLASS_RE.match(ln.rstrip("\n").rstrip("\r")):
                return True
    finally:
        f.close()
    return False


def read_inproc_keys(root: str):
    """⇒ ([(键, Go 函数名)], "")；取不到 ⇒ (None, 说明)。"""
    p = os.path.join(root, INPROC_REL)
    if not os.path.isfile(p):
        return None, "inproc 表读不到：%s" % INPROC_REL
    try:
        lines = open(p, encoding="utf-8").read().splitlines()
    except OSError as e:
        return None, "inproc 表读不出：%s" % e
    out = []
    for ln in lines:
        m = KEY_RE.match(ln)
        if m:
            out.append((m.group(1), m.group(2)))
    if not out:
        return None, "inproc 表解析出 0 键（取不到输入 ⇒ 空域 ≠ 全绿）"
    return out, ""


def load_registry(root: str):
    """⇒ ({名: (相对件路径, 解释器)}, "")；注册表取不到 ⇒ (None, 说明)。

    ★ 决-10：B 侧真身路径**查现成的判据注册表**（`entries[].path`），✗ 不新造第二张对照表。
    """
    p = os.path.join(root, REGISTRY_REL)
    if not os.path.isfile(p):
        return None, "判据注册表不在盘：%s" % REGISTRY_REL
    try:
        d = json.loads(open(p, encoding="utf-8").read())
    except Exception as e:  # noqa: BLE001
        return None, "判据注册表解析不上：%s" % e
    entries = d.get("entries")
    if not isinstance(entries, list):
        return None, "判据注册表缺 entries 列表"
    out = {}
    for e in entries:
        if not isinstance(e, dict):
            continue
        name = e.get("name") or e.get("id")
        rel = e.get("path")
        if not name or not rel:
            continue
        interp = e.get("interp") or ("bash" if rel.endswith(".sh") else "python3")
        out[name] = (rel, interp)
    if not out:
        return None, "判据注册表解析出 0 条（空域 ≠ 全绿）"
    return out, ""


def scan_gates_fallback(root: str):
    """注册表取不到时的**回落**：只扫 scripts/gates 的 .py（旧口径 · 保自检夹具同形）。⇒ (map, "")。"""
    d = os.path.join(root, GATES_REL)
    if not os.path.isdir(d):
        return None, "门件目录读不到：%s" % GATES_REL
    out = {}
    for name in os.listdir(d):
        if name.endswith(".py"):
            out[name[:-3]] = (os.path.join("scripts", "gates", name), "python3")
    return out, ""


def resolve_realbodies(root: str):
    """B 侧真身 ⇒ ({名: (相对件路径, 解释器)}, 来源)。

    ★ 决-10：**优先查判据注册表**（真身路径的单一真源）；注册表取不到 ⇒ 回落旧口径
    （扫 scripts/gates 的 .py）。两条都取不到 ⇒ (None, 说明)。
    """
    rb, err = load_registry(root)
    if rb is not None:
        return rb, "registry"
    rb2, err2 = scan_gates_fallback(root)
    if rb2 is None:
        return None, "realbody 取不到：注册表(%s) · 目录扫描(%s)" % (err, err2)
    return rb2, "gates"


def load_ratchet(root: str, argv_list):
    p = os.path.join(root, RATCHET_REL)
    if not os.path.isfile(p):
        return None, "棘轮不在盘：%s（先据 --dump-ratchet 落它）" % RATCHET_REL
    try:
        d = json.loads(open(p, encoding="utf-8").read())
    except Exception as e:  # noqa: BLE001
        return None, "棘轮解析不上：%s" % e
    ra = d.get("probe_argv")
    if ra != list(argv_list):
        return None, "棘轮 probe_argv=%r ≠ 本次 argv=%r ⇒ 不可比（不给结论）" % (ra, list(argv_list))
    known = d.get("known")
    if not isinstance(known, list):
        return None, "棘轮缺 known 列表"
    return set(known), ""


def run_side(argv, cwd, timeout=PROBE_TIMEOUT):
    """⇒ (rc|None, out, err, 超时/起不来 标记)。rc=None ⇒ 没进程退码可言。"""
    try:
        p = subprocess.run(argv, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           timeout=timeout)
        return p.returncode, p.stdout.decode("utf-8", "replace"), p.stderr.decode("utf-8", "replace"), False
    except subprocess.TimeoutExpired as e:
        return None, (e.stdout or b"").decode("utf-8", "replace"), (e.stderr or b"").decode("utf-8", "replace"), True
    except OSError as e:
        return None, "", "%s" % e, True


def probe(root, key, argv_list, zerg_bin, py_bin, realbody):
    """两态对拍一条键。⇒ dict(kind, ...)；kind ∈ same/diff/unresolved。

    ★ 决-10：态② 真身路径/解释器取自判据注册表（realbody = (相对件路径, 解释器)）。
    ★ 批-2.1（写工具档）：真身若声明 `#zerg-class: write`，态①（壳）须在**键名之后**补壳级 `--yes`
      （`family_gate_check.go` 的写工具危险档闸 `:817–891`；`gateCheckStripShellFlags` 在 exec 前
      **消费** `--yes`、✗ 不透传子件）⇒ 否则态① 恒退 2（`yes_required`）而态② 照跑 ⇒ 结构性判差。
    """
    rel, interp = realbody
    side_a = [zerg_bin, "gate", "check", key]
    if is_write_class(root, rel):
        side_a += ["--yes"]  # ★ 紧贴键名（派单：`--yes` 缀在子件参数之后会引 CLI 一行告警）
    side_a += list(argv_list)
    interp_bin = py_bin if interp == "python3" else interp
    side_b = [interp_bin, os.path.join(root, rel)] + list(argv_list)
    rca, outa, erra, toa = run_side(side_a, root)
    rcb, outb, errb, tob = run_side(side_b, root)
    if toa or tob or rca is None or rcb is None:
        return {"kind": "unresolved", "a_rc": rca, "b_rc": rcb,
                "a_to": toa, "b_to": tob, "detail": "两态之一超时/起不来 ⇒ 不判"}
    ma, mb = mask(outa), mask(outb)
    ea, eb = mask(erra), mask(errb)
    if rca == rcb and ma == mb and ea == eb:
        return {"kind": "same", "a_rc": rca, "b_rc": rcb}
    return {"kind": "diff", "a_rc": rca, "b_rc": rcb,
            "out_diff": ma != mb, "err_diff": ea != eb, "rc_diff": rca != rcb,
            "a_out": ma, "b_out": mb, "a_err": ea, "b_err": eb}


def evaluate(root, argv_list, zerg_bin=None, py_bin=None, timeout=PROBE_TIMEOUT,
             require_ratchet=True, only=None):
    """⇒ (rc_data, err)。rc_data = 全量读数（不含退码判定）。"""
    keys, err = read_inproc_keys(root)
    if keys is None:
        return None, err
    realbodies, rb_src = resolve_realbodies(root)
    if realbodies is None:
        return None, rb_src
    if require_ratchet:
        known, err = load_ratchet(root, argv_list)
        if known is None:
            return None, err
    else:
        known = set()
    zerg_bin = zerg_bin or os.path.join(root, "bin", "zerg")
    if not os.path.isfile(zerg_bin):
        return None, "bin/zerg 不在盘：%s（态① 跑不了）" % zerg_bin
    py_bin = py_bin or sys.executable
    key_names = [k for k, _ in keys]
    if only:
        key_names = [k for k in key_names if k in set(only)]
    key_set = set(k for k, _ in keys)
    # 表A：A 有键 · B 无真身（★ 决-10：B 侧真身由判据注册表判在不在）
    table_a = [k for k in key_names if k not in realbodies]
    # 表B：B 有真身 · A 无键
    table_b = sorted(s for s in realbodies if s not in key_set)
    results = {}
    same, diff, unresolved = [], [], []
    for k in key_names:
        if k not in realbodies:
            continue  # 表A 已点名 · 无件可对拍
        r = probe(root, k, argv_list, zerg_bin, py_bin, realbodies[k])
        results[k] = r
        if r["kind"] == "same":
            same.append(k)
        elif r["kind"] == "diff":
            diff.append(k)
        else:
            unresolved.append(k)
    diff_set = set(diff)
    ratchet_grew = sorted(diff_set - known)
    ratchet_shrank = sorted(known - diff_set)
    return {
        "schema": SCHEMA, "root": root, "probe_argv": list(argv_list),
        "keys_total": len(key_names), "py_stems_total": len(realbodies),
        "realbody_source": rb_src,
        "probed": len(same) + len(diff) + len(unresolved),
        "same": sorted(same), "diff": sorted(diff), "unresolved": sorted(unresolved),
        "table_a_key_no_file": sorted(table_a),
        "table_b_file_no_key": table_b,
        "ratchet_known": sorted(known),
        "ratchet_grew": ratchet_grew,
        "ratchet_shrank": ratchet_shrank,
        "results": results,
    }, ""


def render(rep, strict):
    out = []
    out.append("── py⟷Go 双实现两态对拍（inproc 键 ⇕ 判据注册表真身路径）──")
    out.append("  两态 argv：%s" % (" ".join(rep["probe_argv"]) or "（空）"))
    out.append("  inproc 键 %d 条 · py 真身 %d 条 · 已对拍 %d 条（同 %d · 差 %d · 不判 %d）"
               % (rep["keys_total"], rep["py_stems_total"], rep["probed"],
                  len(rep["same"]), len(rep["diff"]), len(rep["unresolved"])))
    out.append("  棘轮：已知差 %d · 本次涨 %d · 本次降 %d"
               % (len(rep["ratchet_known"]), len(rep["ratchet_grew"]), len(rep["ratchet_shrank"])))
    if rep["diff"]:
        out.append("  ✗ 差集（两态 stdout/stderr/退码 未逐字同）：")
        for k in rep["diff"]:
            r = rep["results"].get(k, {})
            tags = []
            if r.get("rc_diff"):
                tags.append("rc %s≠%s" % (r.get("a_rc"), r.get("b_rc")))
            if r.get("out_diff"):
                tags.append("stdout 异")
            if r.get("err_diff"):
                tags.append("stderr 异")
            out.append("    · %s（%s）" % (k, " · ".join(tags)))
    if rep["unresolved"]:
        out.append("  ⚠ 不判（超时/起不来 · 不进棘轮）：%s" % " ".join(rep["unresolved"]))
    if rep["ratchet_grew"]:
        out.append("  ✗ 棘轮涨了（现读差集里棘轮没有的键）：%s" % " ".join(rep["ratchet_grew"]))
    if rep["ratchet_shrank"]:
        out.append("  ✓ 棘轮降了（棘轮里有、现读已不合）：%s" % " ".join(rep["ratchet_shrank"]))
    out.append("  表A「A 有键 · B 无件」%d 条：" % len(rep["table_a_key_no_file"]))
    for k in rep["table_a_key_no_file"]:
        out.append("    ✗ %s（inproc 在册 · 判据注册表里无真身路径）" % k)
    if not rep["table_a_key_no_file"]:
        out.append("    （空 · 每枚 inproc 键都在注册表里有真身）")
    out.append("  表B「B 有件 · A 无键」%d 条（半接线/未搬 · 只报告）：" % len(rep["table_b_file_no_key"]))
    for s in rep["table_b_file_no_key"]:
        out.append("    · %s" % s)
    verdict = "判红档" if strict else "报告档"
    greens = (not rep["table_a_key_no_file"]) and (not rep["ratchet_grew"])
    out.append("  ⇒ %s：%s" % (verdict, "全绿" if greens else
                              "有 %d 项需处理（表A %d · 棘轮涨 %d）"
                              % (len(rep["table_a_key_no_file"]) + len(rep["ratchet_grew"]),
                                 len(rep["table_a_key_no_file"]), len(rep["ratchet_grew"]))))
    return "\n".join(out)


def cmd_dump_ratchet(rep):
    """现读差集打到 stdout（**不写盘**）· 供人照抄进棘轮。"""
    body = {
        "schema": SCHEMA,
        "_口径": "只许下降：本集是「已声明可分叉」的键；现读差集必须 ⊆ 本集。新增键 ⇒ --strict 判红。",
        "probe_argv": rep["probe_argv"],
        "known": rep["diff"],
    }
    print(json.dumps(body, ensure_ascii=False, indent=2))


def main(argv=None):
    ap = argparse.ArgumentParser(description="py⟷Go 双实现两态对拍防波门（只读 · 棘轮只许降）")
    ap.add_argument("--root", default="")
    ap.add_argument("--argv", default="--self-test",
                    help="两态共用的 argv 串（按空格切；给空串 = 无参）")
    ap.add_argument("--zerg", default="", help="态① 启动器（缺省 <root>/bin/zerg）")
    ap.add_argument("--python", dest="pybin", default="", help="态② 解释器（缺省本进程 python）")
    ap.add_argument("--strict", action="store_true", help="判红档：棘轮涨 / 表A 非空 ⇒ rc=1")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--dump-ratchet", dest="dump_ratchet", action="store_true")
    ap.add_argument("--only", action="append", default=None, help="只对拍指定键（可重复 · 调试用）")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    root = os.path.abspath(a.root) if a.root else os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    argv_list = a.argv.split() if a.argv else []
    zerg_bin = a.zerg or os.path.join(root, "bin", "zerg")
    # --dump-ratchet 是**引导档**：棘轮还没落盘时也能现跑出差集（不要求棘轮在盘）
    rep, err = evaluate(root, argv_list, zerg_bin=zerg_bin, py_bin=(a.pybin or None),
                        require_ratchet=(not a.dump_ratchet), only=a.only)
    if rep is None:
        sys.stderr.write("%s: %s ⇒ 不给结论（退码 2）\n" % (VERSION, err))
        return 2
    if a.dump_ratchet:
        cmd_dump_ratchet(rep)
        return 0
    if a.json:
        print(json.dumps({k: v for k, v in rep.items() if k != "results"},
                         ensure_ascii=False, indent=2))
    else:
        print(render(rep, a.strict))
    if a.strict and (rep["table_a_key_no_file"] or rep["ratchet_grew"]):
        return 1
    return 0


# ══════════════════════════════════════════════════════════════════
# 自检面（--self-test）：成对正负控 · 仓外夹具 · 不碰真目标
# ══════════════════════════════════════════════════════════════════
WANT_CASES = 20  # ★ 决-10 后 14；★ 批-2.1-续（回溯帧面/SyntaxWarning/写工具档 `--yes`）各加成对 2 格 ⇒ 20。
_TMP = []


def _mk_fixture(base, keys=("k-alpha", "k-beta"), py_extra=(), argv=("--self-test",),
                ratchet_known=(), py_missing=()):
    """现搭仓外最小仓树：inproc 表 ＋ bin/zerg 桩（= 态①）＋ scripts/gates/*.py（态②）＋ 棘轮。

    态① 桩 `bin/zerg`：`gate check <键>` ⇒ 跑 `go/<键>.py`（模拟 Go 进程内实现）。
    态② 真件：`scripts/gates/<键>.py`。
    两侧都吃 `argv`（缺省 --self-test）；两侧输出由各自 print 决定 ⇒ 改一字节即分叉。
    """
    fix = tempfile.mkdtemp(prefix="pgpd-", dir=base)
    _TMP.append(fix)
    os.makedirs(os.path.join(fix, "core", "cmd", "zerg"))
    os.makedirs(os.path.join(fix, "scripts", "gates"))
    os.makedirs(os.path.join(fix, "bin"))
    os.makedirs(os.path.join(fix, "go"))
    # inproc 表（逐字同真表形态：\t"键":  Fn,）
    lines = ["package main", "", "var gateCheckGoEntry = map[string]func(){",
             ]
    for k in keys:
        lines.append('\t"%s":   %sGateMain,' % (k, k.replace("-", "_")))
    lines.append("}")
    open(os.path.join(fix, INPROC_REL), "w", encoding="utf-8").write("\n".join(lines) + "\n")
    # bin/zerg 桩：gate check <键> [argv…] ⇒ go/<键>.py
    stub = ('#!/bin/sh\n'
            'here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"\n'
            '[ "$1" = "gate" ] && shift\n'
            '[ "$1" = "check" ] && shift\n'
            'k="$1"; shift\n'
            'exec python3 "$here/../go/$k.py" "$@"\n')
    zb = os.path.join(fix, "bin", "zerg")
    open(zb, "w", encoding="utf-8").write(stub)
    os.chmod(zb, 0o755)
    # 两侧真件
    for k in keys:
        body = ('import sys\nif "--self-test" in sys.argv:\n    print("OUT-%s")\n'
                'else:\n    print("arg:%s")\n' % (k, " ".join(argv)))
        open(os.path.join(fix, "go", k + ".py"), "w", encoding="utf-8").write(body)
        if k not in py_missing:
            open(os.path.join(fix, "scripts", "gates", k + ".py"), "w", encoding="utf-8").write(body)
    for k in py_extra:
        open(os.path.join(fix, "scripts", "gates", k + ".py"), "w", encoding="utf-8").write("# orphan\n")
    # 棘轮
    rj = {"schema": SCHEMA, "probe_argv": list(argv), "known": sorted(ratchet_known)}
    os.makedirs(os.path.join(fix, os.path.dirname(RATCHET_REL)), exist_ok=True)
    open(os.path.join(fix, RATCHET_REL), "w", encoding="utf-8").write(
        json.dumps(rj, ensure_ascii=False))
    return fix


def _bump_side(fix, side, key, to):
    """把某件的一侧**改一个字节**：`OUT-k-alpha` → `OUT-k-alphb`（GO 侧 ⇒ go/<键>.py · PY 侧 ⇒ scripts/gates/<键>.py）。"""
    rel = os.path.join("go", key + ".py") if side == "go" else os.path.join(GATES_REL, key + ".py")
    p = os.path.join(fix, rel)
    s = open(p, encoding="utf-8").read()
    new, n = re.subn(r"OUT-[A-Za-z0-9-]+", "OUT-" + to, s)
    assert n == 1, "未改到（%s · 命中 %d 次）" % (rel, n)
    open(p, "w", encoding="utf-8").write(new)


def _write_side(fix, side, key, body):
    rel = os.path.join("go", key + ".py") if side == "go" else os.path.join(GATES_REL, key + ".py")
    open(os.path.join(fix, rel), "w", encoding="utf-8").write(body)


def _write_registry(fix, entries):
    """写一枚**仓外夹具注册表**（★ 决-10 的 B 侧真身单一真源）。entries = [(名, 相对件路径, 解释器)]。"""
    body = {"schema": "judge-registry/1", "roster_dir": "scripts",
            "entries": [{"id": n, "name": n, "path": pp, "interp": i} for n, pp, i in entries]}
    os.makedirs(os.path.dirname(os.path.join(fix, REGISTRY_REL)), exist_ok=True)
    open(os.path.join(fix, REGISTRY_REL), "w", encoding="utf-8").write(
        json.dumps(body, ensure_ascii=False))


def _run_inproc(argv):
    """进程内跑 main()（真返回值）· 捕获 stdout ⇒ (rc, out)。"""
    import contextlib
    import io
    out = io.StringIO()
    saved = sys.argv
    try:
        with contextlib.redirect_stdout(out):
            rc = main(argv)
    finally:
        sys.argv = saved
    return rc, out.getvalue()


def _ev(fix, argv=("--self-test",), **kw):
    return evaluate(fix, list(argv), **kw)


def self_test():
    d = tempfile.mkdtemp(prefix="pgpd-selftest-")
    _TMP.append(d)
    bad, total = 0, 0

    def case(tag, desc, ok, detail):
        nonlocal bad, total
        total += 1
        bad += 0 if ok else 1
        print("  %s %s %s\n       %s" % ("✓" if ok else "✗", tag, desc, detail))

    print("── check-py-go-pairing-drift.py 自检（成对正负控 · 仓外夹具 · 不碰真目标）──")

    # ①正控：两态一致 ⇒ 同
    f = _mk_fixture(d)
    rep, err = _ev(f)
    case("①正控", "健康夹具（两侧同）⇒ 差集空 · 表A 空 · rc=0",
         err == "" and rep["diff"] == [] and rep["table_a_key_no_file"] == [],
         "err=%r diff=%s 表A=%s" % (err, rep["diff"] if rep else None,
                                   rep["table_a_key_no_file"] if rep else None))

    # ②负控①：改 Go 侧一字节 ⇒ 点名该键
    _bump_side(f, "go", "k-alpha", "k-alphb")
    rep, err = _ev(f)
    case("②负控", "改**态①(Go 侧)**一字节 ⇒ 差集恰 {k-alpha}（点名该键）",
         err == "" and rep["diff"] == ["k-alpha"],
         "diff=%s" % (rep["diff"] if rep else None))

    # ③正控：复原 ⇒ 回绿（成对）
    _bump_side(f, "go", "k-alpha", "k-alpha")
    rep, err = _ev(f)
    case("③正控", "与②成对：复原 Go 侧 ⇒ 差集回空",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ④负控②：改 py 侧一字节 ⇒ 点名该键
    _bump_side(f, "py", "k-beta", "k-betb")
    rep, err = _ev(f)
    case("④负控", "改**态②(py 侧)**一字节 ⇒ 差集恰 {k-beta}",
         err == "" and rep["diff"] == ["k-beta"],
         "diff=%s" % (rep["diff"] if rep else None))
    _bump_side(f, "py", "k-beta", "k-beta")

    # ⑤负控③：退码面差也判异
    _write_side(f, "py", "k-alpha",
                'import sys\nif "--self-test" in sys.argv:\n    print("OUT-k-alpha")\nsys.exit(3)\n')
    rep, err = _ev(f)
    case("⑤负控", "态② 退码改成 3（stdout 相同）⇒ 仍判异（退码面也算）",
         err == "" and rep["diff"] == ["k-alpha"],
         "diff=%s" % (rep["diff"] if rep else None))
    _write_side(f, "py", "k-alpha",
                'import sys\nif "--self-test" in sys.argv:\n    print("OUT-k-alpha")\n'
                'else:\n    print("arg:--self-test")\n')

    # ⑥负控④：棘轮涨 —— 棘轮 known 为空而现读有差 ⇒ --strict rc=1
    f2 = _mk_fixture(d, ratchet_known=())
    _bump_side(f2, "go", "k-alpha", "k-alphb")
    rc, out = _run_inproc(["--root", f2, "--strict"])
    case("⑥负控", "棘轮 known 空 + 出现新差 ⇒ --strict rc=1（棘轮只许降）",
         rc == 1 and "棘轮涨了" in out and "k-alpha" in out, "rc=%d" % rc)

    # ⑦正控：差集已在棘轮 ⇒ --strict rc=0（成对）
    f3 = _mk_fixture(d, ratchet_known=("k-alpha",))
    _bump_side(f3, "go", "k-alpha", "k-alphb")
    rc, out = _run_inproc(["--root", f3, "--strict"])
    case("⑦正控", "与⑥成对：差集已在棘轮里 ⇒ --strict rc=0",
         rc == 0, "rc=%d" % rc)

    # ⑧表A：键无件 ⇒ 点名 + strict 判红
    f4 = _mk_fixture(d, keys=("k-alpha", "k-beta", "k-gamma"), py_missing=("k-gamma",))
    rep, err = _ev(f4)
    rc, out = _run_inproc(["--root", f4, "--strict"])
    case("⑧负控", "表A：inproc 有 k-gamma 而 scripts/gates/k-gamma.py 不在盘 ⇒ 点名 · strict rc=1",
         err == "" and rep["table_a_key_no_file"] == ["k-gamma"] and rc == 1 and "k-gamma" in out,
         "表A=%s rc=%d" % (rep["table_a_key_no_file"] if rep else None, rc))

    # ⑨表B：件无键 ⇒ 只报告 · 不判红
    f5 = _mk_fixture(d, py_extra=("k-orphan",))
    rep, err = _ev(f5)
    rc, out = _run_inproc(["--root", f5, "--strict"])
    case("⑨正控", "表B：scripts/gates/k-orphan.py 在盘而 inproc 无键 ⇒ 表B 点名 · **不**判红（rc=0）",
         err == "" and rep["table_b_file_no_key"] == ["k-orphan"] and rc == 0,
         "表B=%s rc=%d" % (rep["table_b_file_no_key"] if rep else None, rc))

    # ⑩掩码：两侧只差时间戳/耗时/临时目录 ⇒ 掩码后判同
    f6 = _mk_fixture(d, keys=("k-mask",))
    open(os.path.join(f6, "go", "k-mask.py"), "w", encoding="utf-8").write(
        'print("done 2026-10-05 12:00:00 in 0.31s at /var/folders/ab/T/xyz")\n')
    open(os.path.join(f6, GATES_REL, "k-mask.py"), "w", encoding="utf-8").write(
        'print("done 2026-10-06 01:02:03 in 9.99s at /var/folders/zz/T/qwerty")\n')
    rep, err = _ev(f6)
    case("⑩掩码", "两侧只差时间戳/耗时/临时目录（已声明非确定面）⇒ 掩码后判**同**",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ⑪空域守卫：inproc 表读不到 ⇒ rc=2
    f7 = tempfile.mkdtemp(prefix="pgpd-notable-", dir=d)
    os.makedirs(os.path.join(f7, "scripts", "gates"))
    os.makedirs(os.path.join(f7, "bin"))
    rc, out = _run_inproc(["--root", f7])
    case("⑪退码钳", "inproc 表读不到 ⇒ rc=2（不给结论 · 空域 ≠ 全绿）",
         rc == 2, "rc=%d" % rc)

    # ⑫棘轮 argv 不符 ⇒ rc=2
    f8 = _mk_fixture(d, argv=("--self-test",))
    rc, out = _run_inproc(["--root", f8, "--argv="])
    case("⑫退码钳", "棘轮 probe_argv(--self-test) ≠ 本次 argv(空) ⇒ rc=2（不可比）",
         rc == 2, "rc=%d" % rc)

    # ⑬正控（决-10）：B 侧真身在 scripts/evals（不在 gates 面）· 注册表给路径 ⇒ 表A 空 · 两态走注册表路径
    f9 = _mk_fixture(d, keys=("k-reg",), py_missing=("k-reg",))
    os.makedirs(os.path.join(f9, "scripts", "evals"))
    shutil.copy(os.path.join(f9, "go", "k-reg.py"), os.path.join(f9, "scripts", "evals", "k-reg.py"))
    _write_registry(f9, [("k-reg", "scripts/evals/k-reg.py", "python3")])
    rep, err = _ev(f9)
    rc, out = _run_inproc(["--root", f9, "--strict"])
    case("⑬正控", "决-10：真身在 scripts/evals（不在 gates 面）· 注册表给路径 ⇒ 表A 空 · 两态对拍走注册表路径 rc=0",
         err == "" and rep["table_a_key_no_file"] == [] and rep["diff"] == [] and rc == 0,
         "表A=%s diff=%s rc=%d" % (rep["table_a_key_no_file"] if rep else None,
                                   rep["diff"] if rep else None, rc))

    # ⑭负控（决-10）：注册表在盘但缺该键 ⇒ 表A 点名 · strict rc=1（与⑧表A 面成对 · 走注册表面）
    f10 = _mk_fixture(d, keys=("k-reg2",), py_missing=("k-reg2",))
    os.makedirs(os.path.join(f10, "scripts", "evals"))
    _write_registry(f10, [("k-other", "scripts/evals/k-other.py", "python3")])
    rep, err = _ev(f10)
    rc, out = _run_inproc(["--root", f10, "--strict"])
    case("⑭负控", "决-10：注册表在盘但无 k-reg2 ⇒ 表A 点名 · strict rc=1",
         err == "" and rep["table_a_key_no_file"] == ["k-reg2"] and rc == 1 and "k-reg2" in out,
         "表A=%s rc=%d" % (rep["table_a_key_no_file"] if rep else None, rc))

    # ⑮正控（回溯帧面 · 批-2.1-续）：py 侧整段裸回溯 ⟷ Go 侧一行「异常类名：正文（具名文案）」⇒ 掩后同
    f11 = _mk_fixture(d, keys=("k-tb",))
    go_tb = r'''import sys
sys.stderr.write("KeyError: 'K'（py 真身裸回溯 rc=1 · 回溯帧落在解释器面 ⇒ 正文不复刻）\n")
sys.exit(1)
'''
    open(os.path.join(f11, "go", "k-tb.py"), "w", encoding="utf-8").write(go_tb)
    py_tb = r'''import sys
sys.stderr.write("Traceback (most recent call last):\n  File \"/x/os.py\", line 679, in __getitem__\n    w = os.environ[\"PROBE_DIR\"]\n        ~~~~~~~~~~^^^^^^^^^^^^^\n  File \"<frozen os>\", line 709, in __getitem__\nKeyError: 'K'\n")
sys.exit(1)
'''
    open(os.path.join(f11, GATES_REL, "k-tb.py"), "w", encoding="utf-8").write(py_tb)
    rep, err = _ev(f11)
    case("⑮正控", "回溯帧面：py 整段裸回溯 ⟷ Go 一行具名文案 ⇒ 掩码后判**同**",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ⑯负控（成对 · 回溯帧面）：Go 侧「异常类名：正文」改一字 ⇒ 仍判异（掩码 ✗ 不掩正文）
    open(os.path.join(f11, "go", "k-tb.py"), "w", encoding="utf-8").write(
        go_tb.replace("'K'", "'M'"))
    rep, err = _ev(f11)
    case("⑯负控", "与⑮成对：异常正文 K→M ⇒ 仍判异（真差不得被掩）",
         err == "" and rep["diff"] == ["k-tb"], "diff=%s" % (rep["diff"] if rep else None))

    # ⑰正控（解释器告警面）：py 侧 SyntaxWarning 行＋源回显行 ⟷ Go 侧 stderr 空 ⇒ 掩后同
    f12 = _mk_fixture(d, keys=("k-warn",))
    open(os.path.join(f12, "go", "k-warn.py"), "w", encoding="utf-8").write("import sys\nsys.exit(0)\n")
    warn_ok = r'''import sys
sys.stderr.write('/x/gen-gap-ledger.py:29: SyntaxWarning: "\\d" is an invalid escape sequence.\n  GN_ID_RE = re.compile("^Q-\\d+$")\n')
sys.exit(0)
'''
    open(os.path.join(f12, GATES_REL, "k-warn.py"), "w", encoding="utf-8").write(warn_ok)
    rep, err = _ev(f12)
    case("⑰正控", "解释器告警面：py SyntaxWarning 行＋源回显行 ⟷ Go 空 ⇒ 掩码后判**同**",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ⑱负控（成对 · 告警面）：告警/回显行之外**另加一行真差** ⇒ 仍判异
    open(os.path.join(f12, GATES_REL, "k-warn.py"), "w", encoding="utf-8").write(r'''import sys
sys.stderr.write('/x/gen-gap-ledger.py:29: SyntaxWarning: bad\n  src line\nREAL-DIFF-LINE\n')
sys.exit(0)
''')
    rep, err = _ev(f12)
    case("⑱负控", "与⑰成对：告警行外另加一行真差 ⇒ 仍判异（掩码 ✗ 不吞真差）",
         err == "" and rep["diff"] == ["k-warn"], "diff=%s" % (rep["diff"] if rep else None))

    # ⑲正控（写工具档启动口径）：写类件 ⇒ `is_write_class` True · side_a 在**键名后**补壳级 `--yes`
    f13 = _mk_fixture(d, keys=("k-w",))
    open(os.path.join(f13, "scripts", "gates", "k-w.sh"), "w", encoding="utf-8").write(
        "#!/bin/sh\n#zerg-class: write D2\necho OUT-k-w\n")
    _seen = []
    _orig_rs = globals()["run_side"]
    globals()["run_side"] = (lambda argv, cwd, timeout=None: _seen.append(list(argv)) or (0, "", "", False))
    try:
        probe(f13, "k-w", ["--self-test"], "/bin/zerg", "python3", ("scripts/gates/k-w.sh", "bash"))
    finally:
        globals()["run_side"] = _orig_rs
    case("⑲正控", "写工具档：件头 `#zerg-class: write D2` ⇒ is_write_class True · side_a 键名后带 --yes",
         is_write_class(f13, "scripts/gates/k-w.sh") is True and _seen
         and _seen[0][:5] == ["/bin/zerg", "gate", "check", "k-w", "--yes"],
         "is_write_class=%r side_a=%s" % (is_write_class(f13, "scripts/gates/k-w.sh"),
                                          _seen[0] if _seen else None))

    # ⑳负控（成对）：非写类件 ⇒ is_write_class False · side_a ✗ 不带 --yes（不许放宽）
    open(os.path.join(f13, "scripts", "gates", "k-plain.sh"), "w", encoding="utf-8").write(
        "#!/bin/sh\necho hi\n")
    _seen.clear()
    globals()["run_side"] = (lambda argv, cwd, timeout=None: _seen.append(list(argv)) or (0, "", "", False))
    try:
        probe(f13, "k-plain", ["--self-test"], "/bin/zerg", "python3", ("scripts/gates/k-plain.sh", "bash"))
    finally:
        globals()["run_side"] = _orig_rs
    case("⑳负控", "与⑲成对：非写类件 ⇒ is_write_class False · side_a ✗ 不带 --yes",
         is_write_class(f13, "scripts/gates/k-plain.sh") is False and _seen and "--yes" not in _seen[0],
         "is_write_class=%r side_a=%s" % (is_write_class(f13, "scripts/gates/k-plain.sh"),
                                          _seen[0] if _seen else None))

    print("  自证用例：%d 条 · 断言条数写死 %d（少跑一格即失效）" % (total, WANT_CASES))
    if total != WANT_CASES:
        print("✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2" % (WANT_CASES, total),
              file=sys.stderr)
        return 2
    print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
    return 0 if bad == 0 else 1


def _cleanup():
    for x in _TMP:
        shutil.rmtree(x, ignore_errors=True)


if __name__ == "__main__":
    import atexit
    atexit.register(_cleanup)
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-flag-registry-parity.py —— 门73 **旗标两处登记面对拍**（用法串 ⟷ `valueFlagName` 名字表）

为什么要有它（本枚现读 · 2026-09-28）
--------------------------------------
`core/cmd/zerg/main.go` 里同一枚旗标要**两处登记**：
  ① **用法串**（命令树 `usage:` 给人看的用法行 —— 帮助 / markdown 导出都从它出）；
  ② **`valueFlagName` 名字表**（动作面旗标的名字表 · 代码注释自称「唯一真源」——
     不在表里的旗标在 `parseInvocation` 里一律落 **退 2「未知旗标」**）。
两处**此前无任何一致性判据** ⇒ 漏一处就**静默退 2**（已实撞两次：`--by` 曾只写在用法串里；
`--allow-cross-root` 曾只登记一处 = 新旗标等于命令不可用）。本门把这条判据落成一只
**只读、便宜（本机实测 <0.3s）、两态可辨别**的常跑闸。

判据（**本件唯一的一处定义** · 三向对拍）
------------------------------------------
取数面（**只读 · 不 build · 不联网**）：
  · 面① 用法串 = 命令树里逐条的 `usage:`（非字面量（`lsFaceUsage` / `auditCoverUsage` /
    `impactUsageLine`）**在本包里按常量名现读解析**）；
  · 面② 名字表 = `func valueFlagName` 体里的名字字面量（`//` 注释先剥 —— 注释里写着旗标名，
    不剥会把注释读成登记）；
  · 面③ 解析面 = `valueFlagName` ∪ `parseInvocation` 的**专用 case**（`--json` / `--schema` /
    `--node` / `--accept` / `--confirm` / `--direct` / `--ttl` 一类由专门分支收下的，不取名表）。
    判「认不认得」只看 **面③**；判「名字表上有没有户口」只看 **面②**。

三条判据（任一成立且不在**存量豁免表**里 ⇒ `rc=1` · 逐条点名）：
  甲 **缺一处**：某条命令的用法串里某旗标写成**取值形态**（后面跟着值占位），而面③ 不认它
     —— 真给就退 2「未知旗标」= 命令等于不可用。**透传型命令**（`passthrough: true`：
     `gate ls/run/show/self-test` / `isolate-ro`）的旗标逐字交给被包的脚本 ⇒ 不计。
  乙 **多一处（幽灵）**：面② 里的名字**在全部用法串里零提及** ⇒ 上了户口却无人可见。
  丙 **值/无值形态不一致**：面② 里的名字（按定义**取值**）在用法串里只写成**裸形**
     （后面紧跟 `]` / `|` / 行尾）⇒ 表说取值、门面说不取值。

存量豁免表（**棘轮 · 只许下调** · 逐条带原因）
  ★ 2026-09-28（缺口 `GAP-20260928-127` 的 5 项 + `GAP-20260928-128` 的 1 项 · 同日治掉）：
  本门落地时冻结的那 **6 项存量分家已逐项治掉** ⇒ 本表**清空（6 → 0）**。
  它们不是本门放行的「合法形态」，是**待收敛的存量账**：任一项修好之后，本门会在报告里
  打出「这一行请删」（修好的那一项**不再判红**，免得棘轮表烂成假绿）。表里每一行**必须**
  带原因，缺原因 ⇒ `rc=1`（不许无理由豁免）；**新增**分家项一律 `rc=1`。

退码
----
  0 = 两面对齐（除存量豁免的 N 项，逐条打印 + 计数）
  1 = 有**新的**分家项（逐条点名：面 / 旗标 / 哪条命令 / 为什么）+ 缺原因的豁免行
  2 = **不给结论**：`main.go` 不在盘 / 命令树解析不出 / 两个面任一解析出 0 项
      （空转 = 假绿 ⇒ 照门① 口径落「判不了」）

用法
----
    python3 scripts/gates/check-flag-registry-parity.py              # 现读真树判
    python3 scripts/gates/check-flag-registry-parity.py --root DIR   # 换仓根（仓外副本用）
    python3 scripts/gates/check-flag-registry-parity.py --json       # 末尾追加机器面一行
    python3 scripts/gates/check-flag-registry-parity.py --list-rules # 打判据 / 取数口径
    python3 scripts/gates/check-flag-registry-parity.py --self-test  # 成对负控（合成夹具 · 不碰真目标）

纪律：**只读**（不写仓、不写目标、不起进程）· 不联网 · 不 build（取数面全是盘上现读）。
面④ 读者面（按命令 · 报告档）：见 RULES 末三段。
面⑤ 反读面（按命令 · 2026-10-05 · 缺口 `GAP-20261005-15`）
--------------------------------------------------
面④ 只判「usage 有 ⇒ 实现必读」**一向**；反过来的「实现读了、usage 没列」照不到 ——
`gap add` 的实现（`gapAddInputFromFlags`）读 `--state`，而它的用法串不列 `--state`：
面④ 看不见它（usage 里没这枚）、面②「幽灵」也判不了（`--state` 在别的命令用法串里出现过）
⇒ 使用者从用法发现不了这枚旗标；`已派` / `已立项` 由此多出一个**未登记写入口**（可绕过
`gap assign` 的作业单 + `M-32` 占用预检）。本面把那条反向判据补上：**某命令实现按取值读了
某旗标 ⇒ 该命令的 usage 串必列它**（口径三处收窄，见 `no_usage_face`）· 首版 = 报告档。

纪律：**只读**（不写仓、不写目标、不起进程）· 不联网 · 不 build（取数面全是盘上现读）。
同族骨架：`check-fields-face-parity.py`（同一条思路：命令树声明面 ⟷ 真身面逐名对拍）。
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys
import tempfile

VERSION = "1.0.0"

PKG_REL = os.path.join("core", "cmd", "zerg")
MAIN_REL = os.path.join(PKG_REL, "main.go")

# 旗标名（`--x` / `--x-y`）——命令面闭集形状
FLAG_RE = re.compile(r"(--[A-Za-z0-9-]+)")
# 面②：`valueFlagName` 体里的名字字面量
VF_SIG = "func valueFlagName(a string) string {"
VF_END = 'return ""'
# 面③：解析面专用 case（`case a == "--x"` / `case "--x", "--y":` / `strings.HasPrefix(a, "--x=`)`
CASE_RE = re.compile(r'case a == "(--[A-Za-z0-9-]+)"')
CASE_LIT_RE = re.compile(r'case "(--[A-Za-z0-9-]+)"')
PREFIX_RE = re.compile(r'strings\.HasPrefix\(a, "(--[A-Za-z0-9-]+)=')
PARSE_SIG = "func parseInvocation"
# 命令树
TREE_SIG = "commands = []*command{"
PATH_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
USAGE_RE = re.compile(r'usage:\s*(.*)')
RUN_RE = re.compile(r'run:\s*([A-Za-z0-9_]+)')
USAGE_LIT_RE = re.compile(r'"((?:[^"\\]|\\.)*)"')
CONST_RE = re.compile(r'const (\w+) = "((?:[^"\\]|\\.)*)"')
# 裸形分隔符（ASCII 竖线 / 全角竖线 / 右中括号 / 逗号 / 顿号）
BARE_SEP = "|｜],、"

# ── 面④ 读者面（按命令 · 缺口账 `GAP-20260927-325`）───────────────────────────────
# 「**某命令 usage 串里的旗标 ⇒ 该命令的实现件必读**」—— 按命令判（全局「零读者」是错的判据：
# L4 现读 valueFlagName 全部 117 枚在包内均 ≥1 处读者，唯一零读者 `--exit-on` 是 watch 桩）。
# 「实现件」= 定义该命令 `run:` 处理函数的那个包内 `.go` 件（先剥注释）。
# 「读了」= 该旗标的**字面** `"--F"` 出现在实现件正文里，**或**该旗标落在下面的**类型化字段映射**
#   `--F → inv.<field>` 且实现件里出现 `inv.<field>`（读面经结构体字段、不写字面时的替代口径）。
# ★ 本面首版 = **报告档**（逐条列出 · **不进退码**）——「先让它看得见，再让它拦得住」。
GLOBAL_OPTS = ("--json", "--dry-run", "--yes", "--confirm", "--help")
FLAG_FIELD = {
    "--resume": "resume", "--declared": "declared", "--verified": "verified",
    "--last": "last", "--list": "list", "--all": "all", "--fast": "fast", "--full": "full",
    "--to": "to", "--reset": "reset", "--gate": "stageGate",
    "--model": "modelWant", "--slice-id": "sliceID", "--depends-on": "dependsOn",
    "--acceptance": "acceptanceDeclared", "--desc": "desc", "--priority": "priority",
    "--accept": "acceptWant", "--node": "nodes", "--context": "contextWant",
    "--schema": "schemaWant", "--wait": "wait", "--no-wait": "noWait", "--follow": "follow",
    "--quick": "quick", "--self-test": "selfTest", "--direct": "direct", "--ttl": "ttl",
    "--reload": "reload", "--force": "force", "--idempotency-key": "idemKey",
    "--manifest-only": "manifestOnly", "--plain": "plain",
    "--deep": "deep", "--for-human": "forHuman", "--for-model": "forModel",
    "--strict": "strict", "--wide": "wide",
}
FUNC_DEF_RE = re.compile(r"^func ([A-Za-z0-9_]+)\(", re.M)

# ── 存量豁免表（**棘轮 · 只许下调**）──────────────────────────────────────────────
# 键 = (面, 旗标名)；值 = 原因（**必填** · 不写原因 ⇒ rc=1）。
# ★ 2026-09-28（缺口 `GAP-20260928-127` 的 5 项 + `GAP-20260928-128` 的 1 项 · 同日治掉）：
#   本表原先冻结的 6 项分家**已逐项治掉** —— 缺一处 `--exit-on` 两处登记面补齐（名字表上户口）；
#   幽灵 `--criteria`/`--producer`/`--round` 从名字表除名、`--target-ref` 补进 `plan` 用法串；
#   形态 `--no-fallback` 从「按取值收」的名字表移进布尔面（解析面 case + `inv.kv`）。
#   ⇒ 本表**清空（6 → 0）**：棘轮只许下调，留空是**如实**、不是放宽；任何**新增**分家项仍一律判红。
EXEMPT = {}

RULES = (
    "面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；"
    "面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。"
    "甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；"
    "乙 多一处 = 面② 的名字在全部用法串里零提及；"
    "丙 形态 = 面② 的名字在用法串里只写裸形。"
    " 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1。"
    "面④ 读者面（按命令 · 报告档 · 不进退码）= 某命令 usage 串里的旗标 ⇒ 该命令的实现件"
    "（定义其 run: 处理函数的包内 .go 件 · 先剥注释）必读它（字面 \"--F\" 或类型化字段 "
    "inv.<field>）—— 缺口账 `GAP-20260927-325`（全局「零读者」是错的判据）。"
    "面⑤ 反读面（按命令 · 报告档 · 不进退码）= 某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ "
    "该命令的 usage 串必列它（面④ 的另一向 —— 「实现读 ⇒ 用法必列」，缺口 `GAP-20261005-15`）；"
    "拒执守卫（同一函数里 `inv.flagVals`/`inv.hasFlag` 存在性检查）与别处已列 不计。"
)
RCS = "0=对齐 · 1=有新的分家项（逐条点名）· 2=取数面拿不到（不给结论）"


class Bail(Exception):
    """取数面拿不到 ⇒ rc=2（**禁止**当绿）。"""


def strip_comments(src: str) -> str:
    """剥 `//` 注释（注释里写着旗标名 ⇒ 不剥会把注释读成登记）。"""
    return "\n".join(line.split("//", 1)[0] for line in src.splitlines())


def read(path: str) -> str:
    try:
        with open(path, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def pkg_sources(root: str):
    """本包全部 .go（面① 的三个常量用法串住在族件里 ⇒ 现读整包）。"""
    d = os.path.join(root, PKG_REL)
    if not os.path.isdir(d):
        raise Bail("包目录不在盘：%s" % PKG_REL)
    out = {}
    for name in sorted(os.listdir(d)):
        if name.endswith(".go") and not name.endswith("_test.go"):
            out[name] = strip_comments(read(os.path.join(d, name)))
    if not out:
        raise Bail("包目录下零个 .go 源件：%s" % PKG_REL)
    return out


def parse_faces(root: str):
    """→ (V, D, rows, consts)：面② / 面③ / 命令树逐条 / 包内常量（用法串非字面量用）。"""
    srcs = pkg_sources(root)
    main_src = srcs.get("main.go")
    if main_src is None:
        raise Bail("命令树真源不在盘：%s" % MAIN_REL)

    # ── 面② 名字表 ──────────────────────────────────────────────
    try:
        i = main_src.index(VF_SIG)
        j = main_src.index(VF_END, i)
    except ValueError:
        raise Bail("`%s` 找不到（名字表取数面变了 ⇒ 不给结论）" % VF_SIG)
    V = list(dict.fromkeys(FLAG_RE.findall(main_src[i:j])))

    # ── 面③ 解析面 ──────────────────────────────────────────────
    if PARSE_SIG not in main_src:
        raise Bail("`%s` 找不到（解析面取数面变了 ⇒ 不给结论）" % PARSE_SIG)
    p = main_src.index(PARSE_SIG)
    seg = main_src[p:i if i > p else len(main_src)]
    D = set(CASE_RE.findall(seg)) | set(CASE_LIT_RE.findall(seg)) | set(PREFIX_RE.findall(seg))

    # ── 面① 用法串（命令树逐条）────────────────────────────────
    if TREE_SIG not in main_src:
        raise Bail("命令树 `%s` 找不到（用法串取数面变了 ⇒ 不给结论）" % TREE_SIG)
    t0 = main_src.index(TREE_SIG)
    t1 = main_src.index("\n\t}\n", t0)
    tree = main_src[t0:t1]

    consts = {}
    for name, body in srcs.items():
        for m in CONST_RE.finditer(body):
            consts.setdefault(m.group(1), m.group(2))

    rows = []
    for block in re.split(r"\n\t\t\{", tree):
        mp = PATH_RE.search(block)
        if not mp:
            continue
        path = "/".join(re.findall(r'"([^"]+)"', mp.group(1)))
        mu = USAGE_RE.search(block)
        if not mu:
            continue
        raw = mu.group(1).strip().rstrip(",").strip()
        usage = ""
        if raw.startswith('"'):
            lit = USAGE_LIT_RE.match(raw)
            usage = lit.group(1) if lit else ""
        else:
            key = raw.split()[0].strip(",") if raw.split() else ""
            usage = consts.get(key, "")
        mrun = RUN_RE.search(block)
        rows.append({
            "path": path,
            "usage": usage,
            "passthrough": "passthrough: true" in block,
            "run": mrun.group(1) if mrun else "",
        })
    if not rows:
        raise Bail("命令树解析出 0 条命令（解析不出 = 判不了，不给结论）")
    if not V:
        raise Bail("面② 名字表解析出 0 项（空转 = 假绿，不给结论）")
    return V, D, rows, consts


def value_form(usage: str, pos: int) -> bool:
    """旗标后面那一位是不是**值**（`=v` / `<值>` / `…` / `n` / `k=v…` / `P0|P1|P2`）——
    裸形 = 紧跟 `]` / `|` / `｜` / `,` / 顿号 / 行尾 / 另一枚旗标。"""
    if pos >= len(usage):
        return False
    c = usage[pos]
    if c in "=:":
        return True
    t = usage[pos:].lstrip()
    if not t:
        return False
    if t[0] in BARE_SEP:
        return False
    if t.startswith("--"):
        return False
    return True


def reader_face(root: str):
    """面④ 读者面（按命令）：→ [(path, flag, impl)] —— usage 串里的旗标**本命令实现件不读**的清单。
    报告档（不进退码）。"""
    srcs = pkg_sources(root)
    func_file = {}
    for name in sorted(srcs):
        for m in FUNC_DEF_RE.finditer(srcs[name]):
            func_file.setdefault(m.group(1), name)
    _V, _D, rows, _c = parse_faces(root)
    out = []
    for r in rows:
        if r.get("passthrough"):
            continue
        impl = func_file.get(r.get("run") or "")
        body = srcs.get(impl, "") if impl else ""
        for f in sorted(set(FLAG_RE.findall(r["usage"]))):
            if f in GLOBAL_OPTS:
                continue
            if '\"%s\"' % f in body:
                continue
            fld = FLAG_FIELD.get(f)
            if fld and re.search(r"inv\." + re.escape(fld) + r"\b", body):
                continue
            out.append((r["path"], f, impl or "（取不到实现件）"))
    out.sort()
    return out


# ── 面⑤ 反读面（按命令 · 缺口账 `GAP-20261005-15`）──────────────────────────────
# 与面④ **同一条判据的另一向**：面④ = 「usage 有 ⇒ 实现必读」（照不到「实现读了、usage 没列」）；
# 本面 = 「实现**按取值**读了 ⇒ usage 必列」。缺口 `GAP-20261005-15`：`gap add` 的实现
# （`gapAddInputFromFlags`）读 `--state`，而它的用法串不列 `--state` ⇒ 面④ 照不到、
# 面② 也不算「幽灵」（`--state` 在别的命令用法串里出现过）。
# 取数口径（三处收窄 · 防误报；与面④ 同源不同向）：
#   · 实现面 = 从该命令 `run:` 处理函数出发、在本包内**可达的函数闭包**（面④ 用「定义 run: 的
#     那个件」粒度 —— 反向判会太粗：整件里读的旗标会全算到本命令头上）；
#   · 「读」= `inv.flagVal`（**取值**读）；全局选项（`--json`/`--dry-run`/`--yes`/`--confirm`/
#     `--help`）不计；
#   · **拒执守卫不计**：同一函数里同时出现 `inv.flagVals` / `inv.hasFlag`（存在性检查）⇒ 那处是
#     「本动作不收它 ⇒ 拒收」的守卫（`gap verify` / `gap note` / `gap assign` 都这样拒 `--state`），
#     不算「读值」；
#   · **别处已列不计**：可达闭包里若有某条命令的 usage 串已列该旗标（该旗标在归属命令上有户口）
#     ⇒ 不算「本命令漏列」。
# ★ 首版 = **报告档**（逐条列出 · **不进退码**）—— 与面④ 同一档（「先让它看得见，再让它拦得住」）。
GREV_RDV_RE = re.compile(r'inv\.flagVal\(\s*"(--[A-Za-z0-9-]+)"')
GREV_RDP_RE = re.compile(r'inv\.(?:flagVals|hasFlag)\(\s*"(--[A-Za-z0-9-]+)"')
GREV_CALL_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\s*\(")


def rev_func_bodies(srcs):
    """→ {函数名: (件名, 函数体)}（同名取首见 —— 与面④ `func_file` 同口径）。"""
    out = {}
    for name in sorted(srcs):
        body = srcs[name]
        ms = list(FUNC_DEF_RE.finditer(body))
        for i, m in enumerate(ms):
            a = m.start()
            b = ms[i + 1].start() if i + 1 < len(ms) else len(body)
            out.setdefault(m.group(1), (name, body[a:b]))
    return out


def rev_reach(bodies, run):
    """从 run 出发在本包内的可达函数闭包（含 run 自己）。"""
    seen = set()
    stack = [run]
    while stack:
        f = stack.pop()
        if f in seen or f not in bodies:
            continue
        seen.add(f)
        for c in GREV_CALL_RE.findall(bodies[f][1]):
            if c in bodies and c not in seen:
                stack.append(c)
    return seen


def no_usage_face(root: str):
    """面⑤ 反读面（按命令）：→ [(path, flag, 读者函数, 实现件)] —— usage 串没列的「实现取值读」。
    报告档（不进退码）。"""
    srcs = pkg_sources(root)
    bodies = rev_func_bodies(srcs)
    _V, _D, rows, _c = parse_faces(root)
    out = []
    for r in rows:
        if r.get("passthrough") or not r.get("run"):
            continue
        seen = rev_reach(bodies, r["run"])
        documented = set(FLAG_RE.findall(r["usage"]))
        for o in rows:
            if o.get("run") in seen and o.get("usage"):
                documented |= set(FLAG_RE.findall(o["usage"]))
        for f in sorted(seen):
            body = bodies[f][1]
            present = set(GREV_RDP_RE.findall(body))
            for g in sorted(set(GREV_RDV_RE.findall(body))):
                if g in GLOBAL_OPTS or g in present or g in documented:
                    continue
                out.append((r["path"], g, f, bodies[f][0]))
    out.sort()
    return out


def compute(root: str):
    """→ (violations, exempt_hits, stale, counts, no_reader, no_usage)：三条判据逐条算 + 存量表
    命中/失效 + 面④ 读者面（报告档）+ 面⑤ 反读面（报告档）。"""
    V, D, rows, _ = parse_faces(root)
    known = set(V) | D

    any_use, val_use = {}, {}
    for r in rows:
        for m in FLAG_RE.finditer(r["usage"]):
            f = m.group(1)
            any_use.setdefault(f, set()).add(r["path"])
            if value_form(r["usage"], m.end()):
                val_use.setdefault(f, set()).add(r["path"])

    viol = []          # [(面, 旗标, 谁的用法串, 说明)]
    exempt_used = set()
    for f, paths in sorted(val_use.items()):
        if f in known:
            continue
        live = sorted(p for p in paths if not any(r["path"] == p and r["passthrough"] for r in rows))
        if not live:
            continue   # 只在透传型命令的用法串里 ⇒ 设计如此（逐字交给被包的脚本）
        key = ("缺一处", f)
        if key in EXEMPT:
            exempt_used.add(key)
            continue
        viol.append(("缺一处", f, "zerg " + live[0].replace("/", " "),
                     "用法串写着取值形态，解析面（名字表 ∪ 专用 case）不认 ⇒ 真给就退 2「未知旗标」"))
    for f in sorted(V):
        if f not in any_use:
            key = ("幽灵", f)
            if key in EXEMPT:
                exempt_used.add(key)
                continue
            viol.append(("多一处", f, "（全表）", "名字表有它、全部用法串零提及 ⇒ 上了户口却无人可见"))
            continue
        bare_paths = sorted(p for p in any_use[f] if p not in val_use.get(f, set()))
        if bare_paths:
            key = ("形态", f)
            if key in EXEMPT:
                exempt_used.add(key)
                continue
            viol.append(("形态", f, "zerg " + bare_paths[0].replace("/", " "),
                         "名字表按**取值**收（%d 条用法串里 %d 处写成裸形）⇒ 值/无值两态不一致"
                         % (len(any_use[f]), len(bare_paths))))
    stale = sorted(k for k in EXEMPT if k not in exempt_used)
    no_reason = sorted(k for k, v in EXEMPT.items() if not str(v).strip())
    counts = {
        "命令树条数": len(rows),
        "名字表项数": len(V),
        "解析面项数": len(D),
        "用法串旗标数": len(any_use),
        "透传命令": sorted(r["path"] for r in rows if r["passthrough"]),
    }
    return viol, exempt_used, stale, no_reason, counts, reader_face(root), no_usage_face(root)


def report(viol, exempt_used, stale, no_reason, counts, root, as_json, no_reader=(), no_usage=()):
    lines = []
    lines.append("门73 旗标两处登记面对拍（用法串 ⟷ valueFlagName 名字表 + 实现件读者面）· v%s" % VERSION)
    lines.append("仓根：%s" % os.path.abspath(root))
    lines.append("取数：%s" % " · ".join("%s=%s" % (k, v) for k, v in counts.items()))
    lines.append("判据：%s" % RULES)
    if viol:
        lines.append("判红 %d 项（逐条点名 · 面/旗标/哪条命令）：" % len(viol))
        for face, flag, where, why in viol:
            lines.append("  \u2717 [%s] %s · %s · %s" % (face, flag, where, why))
    else:
        lines.append("新分家项 0 项：两面（除存量豁免）对齐。")
    # ── 面④ 读者面（按命令 · 报告档 · 缺口账 `GAP-20260927-325`）──
    lines.append("读者面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20260927-325`）："
                 "「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」——本命令实现件不读的 %d 项："
                 % len(no_reader))
    for path, flag, impl in no_reader:
        lines.append("  · zerg %s · %s · 实现件 %s" % (path.replace("/", " "), flag, impl))
    # ── 面⑤ 反读面（按命令 · 报告档 · 缺口账 `GAP-20261005-15`）──
    lines.append("反读面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20261005-15`）："
                 "「某命令实现按取值读（`inv.flagVal`）了某旗标 ⇒ 该命令的 usage 串必列它」"
                 "（面④ 的另一向；拒执守卫 / 别处已列 不计）——本命令实现读了而 usage 没列的 %d 项："
                 % len(no_usage))
    for path, flag, fn, impl in no_usage:
        lines.append("  · zerg %s · %s · 读者函数 %s（%s）" % (path.replace("/", " "), flag, fn, impl))
    lines.append("存量豁免 %d 项（棘轮 · 逐条打印 · 不进退码）：" % len(exempt_used))
    for key in sorted(exempt_used):
        lines.append("  ~ [%s] %s · %s" % (key[0], key[1], EXEMPT[key]))
    if stale:
        lines.append("★ 以下豁免行**已不再分家**（修好了）⇒ 请删掉本件 EXEMPT 里这一行（棘轮只许下调）：")
        for key in stale:
            lines.append("  ~ [%s] %s" % (key[0], key[1]))
    if no_reason:
        lines.append("判红：以下豁免行**没写原因**（不许无理由豁免）：")
        for key in no_reason:
            lines.append("  ~ [%s] %s" % (key[0], key[1]))
    out = "\n".join(lines)
    if as_json:
        out += "\n" + json.dumps({
            "gate": "flag-registry-parity", "version": VERSION,
            "violations": [{"face": f, "flag": g, "where": w, "why": y} for f, g, w, y in viol],
            "exempt": [{"face": k[0], "flag": k[1], "reason": EXEMPT[k]} for k in sorted(exempt_used)],
            "stale_exempt": [{"face": k[0], "flag": k[1]} for k in stale],
            "exempt_without_reason": [{"face": k[0], "flag": k[1]} for k in no_reason],
            "counts": counts,
            "no_reader": [{"command": p, "flag": f, "impl": i} for p, f, i in no_reader],
            "no_usage": [{"command": p, "flag": f, "func": fn, "impl": i} for p, f, fn, i in no_usage],
        }, ensure_ascii=False)
    return out


def run(root: str, as_json: bool, stdout=sys.stdout):
    try:
        viol, exempt_used, stale, no_reason, counts, no_reader, no_usage = compute(root)
    except Bail as e:
        print("门73 旗标两处登记面对拍：**判不了**（%s）⇒ rc=2 不给结论" % e)
        print("退码：%s" % RCS)
        return 2
    print(report(viol, exempt_used, stale, no_reason, counts, root, as_json, no_reader, no_usage))
    print("退码：%s" % RCS)
    return 1 if (viol or no_reason) else 0


# ── 自检（成对负控 · 合成夹具写在自己的临时目录里，**不碰真目标**）────────────────────
FIXTURE_MAIN = '''package zerg

type command struct {
\tpath        []string
\tusage       string
\tpassthrough bool
}

var commands []*command

func init() {
\tcommands = []*command{
\t\t{
\t\t\tpath:  []string{"task", "submit"},
\t\t\tusage: "zerg task submit --desc <描述> [--slice-id <片>] [--all]",
\t\t},
\t\t{
\t\t\tpath:  []string{"gate", "run"},
\t\t\tusage: "zerg gate run [--scope <s> | --fast] [--outdir <目录>]",
\t\t\tpassthrough: true,
\t\t},
\t}
}

func parseInvocation(args []string) {
\tfor _, a := range args {
\t\tswitch {
\t\tcase a == "--all":
\t\tcase a == "--json":
\t\t\t_ = a
\t\tcase valueFlagName(a) != "":
\t\t\t_ = a
\t\t}
\t}
}

// valueFlagName —— 唯一真源。注释里写 `--zzz-注释` 不许被读成登记。
func valueFlagName(a string) string {
\tswitch a {
\tcase "--desc", "--slice-id":
\t\treturn a
\t}
\treturn ""
}
'''

FIXTURE_EXTRA = '''package zerg

const extraUsage = "zerg probe [--root <仓根>]"
'''


def _mk_fixture(base: str) -> str:
    d = os.path.join(base, PKG_REL)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "main.go"), "w", encoding="utf-8") as f:
        f.write(FIXTURE_MAIN)
    with open(os.path.join(d, "family_probe.go"), "w", encoding="utf-8") as f:
        f.write(FIXTURE_EXTRA)
    return base


def self_test() -> int:
    """成对负控：①对齐 ⇒ 0 ②缺一处 ⇒ 1 且点名 ③多一处 ⇒ 1 且点名 ④形态 ⇒ 1 且点名
    ⑤取数面缺 ⇒ 2 ⑥透传豁免不计红 ⑦零命令树 ⇒ 2。"""
    checks = []
    base = tempfile.mkdtemp(prefix="flagparity-selftest-")

    def add(name, ok, detail=""):
        checks.append((name, bool(ok), detail))

    def run_on(root):
        try:
            viol, ex, stale, nr, counts, no_reader, no_usage = compute(root)
        except Bail as e:
            return ("bail", str(e))
        return ("ok", viol)

    try:
        d = _mk_fixture(os.path.join(base, "aligned"))
        got = run_on(d)
        add("①对齐 ⇒ 无分家项", got[0] == "ok" and got[1] == [], str(got))
        add("①' 透传命令的旗标不进判红（--scope 不在解析面）",
            got[0] == "ok" and not any(v[1] == "--scope" for v in got[1]), str(got))

        d = _mk_fixture(os.path.join(base, "missing"))
        p = os.path.join(d, PKG_REL, "main.go")
        s = open(p, encoding="utf-8").read().replace('case "--desc", "--slice-id":', 'case "--slice-id":')
        open(p, "w", encoding="utf-8").write(s)
        got = run_on(d)
        ok = got[0] == "ok" and any(v[0] == "缺一处" and v[1] == "--desc" for v in got[1])
        add("②缺一处（表里删 --desc）⇒ 判红且**点名 --desc**", ok, str(got))

        d = _mk_fixture(os.path.join(base, "extra"))
        p = os.path.join(d, PKG_REL, "main.go")
        s = open(p, encoding="utf-8").read().replace('case "--desc", "--slice-id":',
                                                     'case "--desc", "--slice-id", "--zz-new":')
        open(p, "w", encoding="utf-8").write(s)
        got = run_on(d)
        ok = got[0] == "ok" and any(v[0] == "多一处" and v[1] == "--zz-new" for v in got[1])
        add("③多一处（表里加一枚没人写的名）⇒ 判红且**点名 --zz-new**", ok, str(got))

        d = _mk_fixture(os.path.join(base, "form"))
        p = os.path.join(d, PKG_REL, "main.go")
        s = open(p, encoding="utf-8").read().replace("--slice-id <片>", "--slice-id")
        open(p, "w", encoding="utf-8").write(s)
        got = run_on(d)
        ok = got[0] == "ok" and any(v[0] == "形态" and v[1] == "--slice-id" for v in got[1])
        add("④形态（用法串取值改裸形）⇒ 判红且**点名 --slice-id**", ok, str(got))

        d = _mk_fixture(os.path.join(base, "nofile"))
        os.remove(os.path.join(d, PKG_REL, "main.go"))
        got = run_on(d)
        add("⑤命令树真源不在盘 ⇒ 判不了（Bail ⇒ rc=2）", got[0] == "bail", str(got))

        d = os.path.join(base, "empty")
        os.makedirs(os.path.join(d, PKG_REL), exist_ok=True)
        open(os.path.join(d, PKG_REL, "main.go"), "w", encoding="utf-8").write(
            "package zerg\n\nfunc valueFlagName(a string) string {\n\treturn \"\"\n}\n\n"
            "func parseInvocation(a []string) {}\n\nvar commands []*command\n")
        got = run_on(d)
        add("⑥命令树零条 ⇒ 判不了（不给结论）", got[0] == "bail", str(got))

        d = _mk_fixture(os.path.join(base, "nocomment"))
        got = run_on(d)
        add("⑦注释里的旗标名不被读成登记（面② 不含 --zzz-注释）",
            got[0] == "ok" and not any("--zzz-注释" == v[1] for v in got[1]), str(got))

        add("⑧豁免行必须带原因（EXEMPT 逐行有值）",
            all(str(v).strip() for v in EXEMPT.values()), "")
        add("⑨判据只有一处定义（本件内 RULES 与三档退码常量在位）",
            "缺一处" in RULES and "多一处" in RULES and "形态" in RULES and RCS.count("=") == 3, RCS)

        # 面④ 读者面（按命令）· 成对负控：impl 不读 ⇒ 列缺读者；补上读者 ⇒ 不再列。
        d = _mk_fixture(os.path.join(base, "reader_missing"))
        nr = reader_face(d)
        add("④读者面（负控）：run 指不到实现件 ⇒ 非全局 usage 旗标列缺读者",
            any(f == "--all" for _, f, _ in nr), str(nr[:3]))
        d = _mk_fixture(os.path.join(base, "reader_ok"))
        pm = os.path.join(d, PKG_REL, "main.go")
        t = open(pm, encoding="utf-8").read().replace(
            '\t\t\tusage: "zerg task submit --desc <描述> [--slice-id <片>] [--all]",\n\t\t},',
            '\t\t\tusage: "zerg task submit --desc <描述> [--slice-id <片>] [--all]",\n'
            '\t\t\trun:   cmdTaskSubmit,\n\t\t},')
        t += '\nfunc cmdTaskSubmit() { _ = "--desc"; _ = "--slice-id"; _ = "--all" }\n'
        open(pm, "w", encoding="utf-8").write(t)
        nr2 = reader_face(d)
        add("④读者面（正控）：实现件读全 ⇒ 该命令不再列缺读者",
            not any(p2 == "task/submit" for p2, _, _ in nr2), str(nr2))

        # 面⑤ 反读面（按命令）· 成对负控：impl 按取值读但 usage 没列 ⇒ 列缺列；usage 列了 ⇒ 不再列。
        d = _mk_fixture(os.path.join(base, "nousage_missing"))
        pm = os.path.join(d, PKG_REL, "main.go")
        t = open(pm, encoding="utf-8").read().replace(
            '\t\t\tpassthrough: true,\n\t\t},\n\t}\n}',
            '\t\t\tpassthrough: true,\n\t\t},\n'
            '\t\t{\n\t\t\tpath:  []string{"probe", "x"},\n'
            '\t\t\tusage: "zerg probe x",\n\t\t\trun:   cmdProbeNoUsage,\n\t\t},\n\t}\n}')
        t += '\nfunc cmdProbeNoUsage() { _ = inv.flagVal("--zz-read") }\n'
        open(pm, "w", encoding="utf-8").write(t)
        nu = no_usage_face(d)
        add("⑤反读面（负控）：实现按取值读但 usage 没列 ⇒ 列出该命令",
            any(pp == "probe/x" and f2 == "--zz-read" for pp, f2, _fn, _im in nu), "")
        d = _mk_fixture(os.path.join(base, "nousage_ok"))
        pm = os.path.join(d, PKG_REL, "main.go")
        t = open(pm, encoding="utf-8").read().replace(
            '\t\t\tpassthrough: true,\n\t\t},\n\t}\n}',
            '\t\t\tpassthrough: true,\n\t\t},\n'
            '\t\t{\n\t\t\tpath:  []string{"probe", "x"},\n'
            '\t\t\tusage: "zerg probe x [--zz-read <值>]",\n\t\t\trun:   cmdProbeNoUsage,\n\t\t},\n\t}\n}')
        t += '\nfunc cmdProbeNoUsage() { _ = inv.flagVal("--zz-read") }\n'
        open(pm, "w", encoding="utf-8").write(t)
        nu2 = no_usage_face(d)
        add("⑤反读面（正控）：usage 列了该旗标 ⇒ 该命令不再列",
            not any(pp == "probe/x" for pp, _f, _fn, _im in nu2), "")

        bad = [c for c in checks if not c[1]]
        print("门73 自检（成对负控 · 合成夹具）：断言 %d 条 · 失败 %d 条" % (len(checks), len(bad)))
        for name, ok, detail in checks:
            print("  %s %s%s" % ("PASS" if ok else "FAIL", name,
                                 "" if ok else "  ⇐ %s" % detail[:400]))
        return 1 if bad else 0
    finally:
        shutil.rmtree(base, ignore_errors=True)


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(add_help=True, description="门73 旗标两处登记面对拍（只读）")
    ap.add_argument("root", nargs="?", default=".", help="仓根（默认当前目录）")
    ap.add_argument("--root", dest="root_opt", default=None, help="同位置参数（显式档）")
    ap.add_argument("--json", action="store_true", help="末尾追加机器面一行")
    ap.add_argument("--list-rules", action="store_true", help="打判据 / 取数口径 / 退码")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    if a.list_rules:
        print(RULES)
        print("退码：%s" % RCS)
        print("存量豁免 %d 项：%s" % (len(EXEMPT), " · ".join("%s/%s" % k for k in sorted(EXEMPT))))
        print("面④ 读者面（按命令 · 报告档 · 不进退码）：RULES 末句 —— "
              "「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」（缺口账 `GAP-20260927-325`）")
        print("面⑤ 反读面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20261005-15`）：RULES 末句 —— "
              "「某命令实现按取值读了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向）")
        return 0
    return run(a.root_opt or a.root, a.json)


if __name__ == "__main__":
    sys.exit(main())

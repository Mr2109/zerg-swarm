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
  今天真树上**已经存在**的 6 项分家（本门落地时先冻结、逐条打印，**不进退码**）——
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
USAGE_LIT_RE = re.compile(r'"((?:[^"\\]|\\.)*)"')
CONST_RE = re.compile(r'const (\w+) = "((?:[^"\\]|\\.)*)"')
# 裸形分隔符（ASCII 竖线 / 全角竖线 / 右中括号 / 逗号 / 顿号）
BARE_SEP = "|｜],、"

# ── 存量豁免表（**棘轮 · 只许下调**）──────────────────────────────────────────────
# 键 = (面, 旗标名)；值 = 原因（**必填** · 不写原因 ⇒ rc=1）。这三类都是**今天真树上已存在**
# 的分家项：本门先把它们冻结成**可见的存量账**（逐条打印），任何**新增**分家项立刻判红。
EXEMPT = {
    ("缺一处", "--exit-on"): "`zerg watch` 的事件面主控今天没有（登记条目自带「本版不给结论」）⇒ 该命令整条未接线，`--exit-on <kind>` 待与事件面同批接线（存量账）。",
    ("幽灵", "--criteria"): "名字表上的死条目：全包零引用 + 用法串零提及 ⇒ 待清理或接线（存量账）。",
    ("幽灵", "--producer"): "名字表上的死条目：全包零引用 + 用法串零提及 ⇒ 待清理或接线（存量账）。",
    ("幽灵", "--round"): "名字表上的死条目：全包零引用 + 用法串零提及 ⇒ 待清理或接线（存量账）。",
    ("幽灵", "--target-ref"): "`family_fusion.go` 真读它（真值面），用法串一个字没写 ⇒ 待补用法串（存量账）。",
    ("形态", "--no-fallback"): "名字表按**取值**收进 kv，用法串写**裸形**（语义本就是布尔档）⇒ 待统一形态（存量账）。",
}

RULES = (
    "面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；"
    "面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。"
    "甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；"
    "乙 多一处 = 面② 的名字在全部用法串里零提及；"
    "丙 形态 = 面② 的名字在用法串里只写裸形。"
    " 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1。"
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
        rows.append({
            "path": path,
            "usage": usage,
            "passthrough": "passthrough: true" in block,
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


def compute(root: str):
    """→ (violations, exempt_hits, stale, counts)：三条判据逐条算 + 存量表命中/失效。"""
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
    return viol, exempt_used, stale, no_reason, counts


def report(viol, exempt_used, stale, no_reason, counts, root, as_json):
    lines = []
    lines.append("门73 旗标两处登记面对拍（用法串 ⟷ valueFlagName 名字表）· v%s" % VERSION)
    lines.append("仓根：%s" % os.path.abspath(root))
    lines.append("取数：%s" % " · ".join("%s=%s" % (k, v) for k, v in counts.items()))
    lines.append("判据：%s" % RULES)
    if viol:
        lines.append("判红 %d 项（逐条点名 · 面/旗标/哪条命令）：" % len(viol))
        for face, flag, where, why in viol:
            lines.append("  \u2717 [%s] %s · %s · %s" % (face, flag, where, why))
    else:
        lines.append("新分家项 0 项：两面（除存量豁免）对齐。")
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
        }, ensure_ascii=False)
    return out


def run(root: str, as_json: bool, stdout=sys.stdout):
    try:
        viol, exempt_used, stale, no_reason, counts = compute(root)
    except Bail as e:
        print("门73 旗标两处登记面对拍：**判不了**（%s）⇒ rc=2 不给结论" % e)
        print("退码：%s" % RCS)
        return 2
    print(report(viol, exempt_used, stale, no_reason, counts, root, as_json))
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
            viol, ex, stale, nr, counts = compute(root)
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
        return 0
    return run(a.root_opt or a.root, a.json)


if __name__ == "__main__":
    sys.exit(main())

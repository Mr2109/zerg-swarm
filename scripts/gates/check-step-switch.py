#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-step-switch.py — 门87：`precommit-gates.sh` 的「步名 ↔ 档位 ↔ 严格旗标」**表级一致性**静态判据。

为什么要有它（缺口 `GAP-20260927-131` ④ / `GAP-20260927-270`）
----------------------------------------------------------------
「步名 ↔ 档位 ↔ 严格旗标」的一致性此前**只在** `precommit-gates.sh` 的合成自检里写 `assert_eq`
（⑩甲 / ⑩乙 两格），**命令面无机械元门能单跑**（缺口 270 逐字：「这类表级一致性只能在门件里写
assert_eq」）⇒ 改表时那一格只能靠整趟自检兜、没法单点验。本件把那一格抽成**独立门件**：
**静态扫 `precommit-gates.sh` 自己的 `add_step` 行**（不取运行时步骤表 —— 各 scope 建表条件会随
机器变），把「表的自洽」变成一件可单跑、可挂门的机械判据。

判据（两条，各对应一道既有自检格）
----------------------------------
  甲（⑩甲）凡 `tri` 步调到**有宽松出口**的门脚本，命令串必须带**它自己的**严格旗标
           （不带 ⇒ 该步只拦 rc=2 / 异常码、**拦不住命中** = 只挂了个恒绿的步 · 门68 升档前踩过）；
           名册 = 现读「有宽松出口」的 3 只（旗标名各取自其 argparse 默认档）。
  乙（⑩乙）步名与档位**不许矛盾**：名写「只报告」⇒ 档位必须 `tri-report`；档位 `tri-report` ⇒
           名必须写「只报告」（后半条防**静默降档**）。

退码（三档 · 与 `tri` 逐格同义）
--------------------------------
  0  全表自洽（甲 0 违例 · 乙 0 违例）
  1  有违例（逐条点名：步名 + 缺失旗标 / 名档矛盾方向）
  2  **不给结论**：扫面空转（本件里一个 `add_step` 都读不到 / 目标件不在盘 ⇒ 判不了 ≠ 绿）

用法
----
    python3 scripts/gates/check-step-switch.py            # 真目标（本仓 precommit-gates.sh）
    python3 scripts/gates/check-step-switch.py --file <件> # 指定表件（诊断用）
    python3 scripts/gates/check-step-switch.py --self-test # 合成夹具自检（三档成对 · 不碰真目标）
"""

import argparse
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
DEFAULT_FILE = os.path.join(REPO, "scripts", "gates", "precommit-gates.sh")

# 步骤行：`add_step <scope> "<名>" <模式> <工作目录> "<命令…>"`（只读到模式 token 为止 —— 命令串里
# 可含转义引号/分号/竖线，本件**不进命令串内部**，只按整行文本判「点到哪只脚本 / 带没带旗标」）。
STEP_RE = re.compile(
    r'^[ \t]*add_step[ \t]+([a-z-]+)[ \t]+"((?:[^"\\]|\\.)*)"[ \t]+(rc|empty|tri|tri-report)[ \t]'
)

# 「有宽松出口」的门脚本 → 它自己的严格旗标（真源：各脚本 `argparse` 的默认档，现读 3 只）。
LOOSE_EXIT = {
    "check-doc-meta.py": "--missing=fail",       # 默认 warn
    "check-wired-scripts.py": "--strict-report",  # 默认只报告
    "check-git-path-outlet.py": "--tier rc",      # 默认 report
}

# 乙（名/档一致）只判**默认 scope 集**（与合成自检 ⑩乙 的 `sc` 逐字同源；`self` 那批合成夹具步不在面内）。
SCOPES = ("go", "rust", "pub", "tags", "docs", "gates", "tools", "slice", "public", "devdocs", "release")


def parse_steps(text):
    """⇒ [{scope,name,mode,line}]（按文件出现序；只认字面模式的 add_step 行）。"""
    steps = []
    for raw in text.splitlines():
        m = STEP_RE.match(raw)
        if m:
            steps.append({"scope": m.group(1), "name": m.group(2),
                          "mode": m.group(3), "line": raw})
    return steps


def scan(text):
    """⇒ (steps, violations)。steps 为空 ⇒ 调用方按 rc=2 处置（空转 = 判不了）。"""
    steps = parse_steps(text)
    bad = []
    for s in steps:
        name, mode, line = s["name"], s["mode"], s["line"]
        in_scope = s["scope"] in SCOPES
        if mode == "tri":
            for script, flag in LOOSE_EXIT.items():
                if script in line and flag not in line:
                    bad.append("甲 缺严格旗标：步「%s」调到有宽松出口的 %s，而命令串里无 `%s` "
                               "（不带它 ⇒ 该步只拦 rc=2/异常码、拦不住命中 = 只挂了个恒绿的步）"
                               % (name, script, flag))
        says_report = "只报告" in name
        if in_scope and mode == "tri" and says_report:
            bad.append("乙 名/档矛盾：步「%s」名写「只报告」却申报 `tri`（阻断）—— 两处对不上" % name)
        if in_scope and mode == "tri-report" and not says_report:
            bad.append("乙 名/档矛盾：步「%s」申报 `tri-report` 而名里没写「只报告」"
                       "（防静默降档：只报告档必须自报家门）" % name)
    return steps, bad


# ── 自检（合成夹具 · 三档成对 · 不碰真目标）──────────────────────────────────────────────
_GOOD = (
    'add_step gates "门X a（阻断）" tri "${REPO_ROOT}" "python3 scripts/gates/check-doc-meta.py --missing=fail"\n'
    'add_step gates "门Y b（只报告）" tri-report "${REPO_ROOT}" "python3 scripts/gates/check-b.py"\n'
)
_BAD_FLAG = (
    'add_step gates "门X a（阻断）" tri "${REPO_ROOT}" "python3 scripts/gates/check-wired-scripts.py"\n'
)
_BAD_NAME = (
    'add_step gates "门Z c（只报告）" tri "${REPO_ROOT}" "python3 scripts/gates/check-c.py"\n'
)
_BAD_DOWN = (
    'add_step gates "门W d（阻断）" tri-report "${REPO_ROOT}" "python3 scripts/gates/check-d.py"\n'
)


def self_test():
    fails = 0

    def chk(label, got, want):
        nonlocal fails
        ok = got == want
        if not ok:
            fails += 1
        print("  %s %s（期望 %s 实际 %s）" % ("✓" if ok else "✗", label, want, got))

    steps, bad = scan(_GOOD)
    chk("正控：好表 ⇒ 2 步 · 0 违例", "%d/%d" % (len(steps), len(bad)), "2/0")

    steps, bad = scan(_BAD_FLAG)
    chk("负控甲：tri 步调到 check-wired-scripts.py 却缺 --strict-report ⇒ 1 违例",
        str(len(bad)), "1")

    steps, bad = scan(_BAD_NAME)
    chk("负控乙：名写「只报告」却申报 tri ⇒ 1 违例", str(len(bad)), "1")

    steps, bad = scan(_BAD_DOWN)
    chk("负控乙·降档：申报 tri-report 却名无「只报告」 ⇒ 1 违例", str(len(bad)), "1")

    steps, bad = scan("# 只有注释，没有 add_step\n")
    chk("空转：扫不到任何 add_step ⇒ steps=0（调用方判 rc=2）", str(len(steps)), "0")

    # 自吃：本件真目标（本仓 precommit-gates.sh）扫得到步、且甲/乙 0 违例（表自洽）
    if os.path.isfile(DEFAULT_FILE):
        with open(DEFAULT_FILE, encoding="utf-8") as fh:
            _, bad = scan(fh.read())
        print("  （真目标自吃：%s）" % ("甲/乙 0 违例 ✓" if not bad else "违例 %d 条 ✗" % len(bad)))
    else:
        print("  （真目标不在盘 ⇒ 跳过自吃那一格）")

    print("── 自检：%s" % ("全过 ✓" if fails == 0 else "**%d 格不过** ✗" % fails))
    return 0 if fails == 0 else 2


def main(argv):
    ap = argparse.ArgumentParser(description="门87：步名↔档位↔严格旗标 表级一致性静态判据")
    ap.add_argument("--file", default=DEFAULT_FILE, help="被扫的表件（默认本仓 scripts/gates/precommit-gates.sh）")
    ap.add_argument("--self-test", "--selftest", dest="self_test", action="store_true",
                    help="合成夹具自检（三档成对 · 不碰真目标）")
    args = ap.parse_args(argv)

    if args.self_test:
        return self_test()

    if not os.path.isfile(args.file):
        print("✗ 表件不在盘（%s）⇒ 判不了 ⇒ 不给结论（rc=2）" % args.file, file=sys.stderr)
        return 2
    with open(args.file, encoding="utf-8") as fh:
        text = fh.read()
    steps, bad = scan(text)
    if not steps:
        print("✗ 表件里一个 `add_step` 都读不到（%s）⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）"
              % args.file, file=sys.stderr)
        return 2

    tri = sum(1 for s in steps if s["scope"] in SCOPES and s["mode"] == "tri")
    rep = sum(1 for s in steps if s["scope"] in SCOPES and s["mode"] == "tri-report")
    nall = len(steps)
    if bad:
        print("✗ 步名↔档位↔严格旗标：违例 %d 条（扫到 add_step %d 条 · 默认集 tri %d · tri-report %d）"
              % (len(bad), nall, tri, rep))
        for b in bad:
            print("   ✗ %s" % b)
        return 1
    print("✓ 步名↔档位↔严格旗标：表自洽（add_step %d 条 · 默认集 tri %d · tri-report %d · 甲/乙 违例 0）"
          % (nall, tri, rep))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

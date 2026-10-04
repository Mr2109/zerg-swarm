#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gate-point-output.py —— 逐挂点输出表元门（缺口 `GAP-20260927-152`）。

题面（缺口逐字）：档位是**挂点属性**而非门属性＝⇒ 必须**逐挂点**列出输出面（挂点×档位×输出面），
才能在两面对拍（门自述档位不可判成一对一）。本件只**只读** `scripts/gates/precommit-gates.sh` 的 `add_step` 声明，
按（挂点 scope / 档位 mode）聚合出表（档位→输出面的归属）。

口径：挂点 = `add_step` 第一个实参；档位 = `add_step` 的第三个实参（四档闭集 rc/empty/tri/tri-report）；
输出面 = 按档位归类（rc/empty/tri ⇒ 阻断面；tri-report ⇒ 只报告面）。退码：0 成表· 2 不给结论（脚本读不到 / 0 步）。

用法：python3 scripts/gates/check-gate-point-output.py [--root <dir>] [--json] [--self-test]
"""
import argparse
import json
import os
import re
import sys
import tempfile

SCRIPT_REL = os.path.join("scripts", "gates", "precommit-gates.sh")
ADD_STEP_RE = re.compile(r'^\s*add_step\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+(\S+)\s+(\S+)\s+"((?:[^"\\]|\\.)*)"')
OUTPUT_FACE = {"rc": "阻断面", "empty": "阻断面", "tri": "阻断面", "tri-report": "只报告面"}


def parse_steps(text):
    rows = []
    for ln in text.splitlines():
        m = ADD_STEP_RE.match(ln)
        if m:
            rows.append({"scope": m.group(1), "name": m.group(2), "mode": m.group(3)})
    return rows


def run(root, as_json):
    path = os.path.join(root, SCRIPT_REL)
    if not os.path.isfile(path):
        print("⛔ 不给结论（rc=2）：门禁脚本不在盘：%s" % path, file=sys.stderr)
        return 2
    try:
        text = open(path, encoding="utf-8").read()
    except OSError as e:
        print("⛔ 不给结论（rc=2）：门禁脚本读不到：%s" % e, file=sys.stderr)
        return 2
    steps = parse_steps(text)
    if not steps:
        print("⛔ 不给结论（rc=2）：现读 0 步（空转 = 假绿）—— %s" % path, file=sys.stderr)
        return 2
    order, cnt = [], {}
    for s in steps:
        k = (s["scope"], s["mode"])
        if k not in cnt:
            order.append(k); cnt[k] = 0
        cnt[k] += 1
    table = [{"scope": k[0], "mode": k[1],
              "output_face": OUTPUT_FACE.get(k[1], "（档位不在四档闭集内）"),
              "count": cnt[k]} for k in order]
    if as_json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GatePointOutput", "steps": len(steps),
                          "rows": table}, ensure_ascii=False))
    else:
        print("── 逐挂点输出表（挂点×档位×输出面）──")
        print("  %-16s %-12s %-12s %s" % ("挂点", "档位", "输出面", "门数"))
        for r in table:
            print("  %-16s %-12s %-12s %d" % (r["scope"], r["mode"], r["output_face"], r["count"]))
        print("  —— 共 %d 步 · %d 个（挂点,档位）组" % (len(steps), len(table)))
    return 0


def self_test():
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts", "gates"))
        open(os.path.join(td, SCRIPT_REL), "w", encoding="utf-8").write(
            'add_step docs "门a" tri "$R" "x.py"\n'
            'add_step docs "门b" tri-report "$R" "y.py"\n'
            'add_step gates "门c" rc "$R" "z.sh"\n')
        steps = parse_steps(open(os.path.join(td, SCRIPT_REL), encoding="utf-8").read())
        if len(steps) != 3 or steps[0]["scope"] != "docs" or steps[2]["mode"] != "rc":
            print("  ✗ 正控失败：%r" % (steps,)); ok = False
        if run(td, False) != 0:
            print("  ✗ 正控 rc 失败"); ok = False
        open(os.path.join(td, SCRIPT_REL), "w").write("# 无 add_step\n")
        if run(td, False) != 2:
            print("  ✗ 负控失败：空脚本应 rc=2"); ok = False
    print("自检 %s（3 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="逐挂点输出表元门（GAP-20260927-152）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    return run(a.root, a.json)


if __name__ == "__main__":
    sys.exit(main())

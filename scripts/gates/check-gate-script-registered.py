#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gate-script-registered.py —— 门件必须在 `scripts/公开标记.tsv` 有行（缺口 `GAP-20260927-206`）。

病：新加的门脚本（`scripts/gates/*.py|*.sh`）无人提醒登记，存量红静默躺在仓里，直到下次跑 public: 门才暴露。
本件只读：列 `scripts/gates/` 直下的 `.py`/`.sh` 门件 ⇕ 读 `scripts/公开标记.tsv` 第一列（路径），差集（未登记门件）逐条点名。
退码：0 全登记 · 1 有未登记门件 · 2 不给结论（标记表读不到 / 目录不在）。**只读**。

用法：python3 scripts/gates/check-gate-script-registered.py [--root <dir>] [--json] [--self-test]
"""
import argparse
import json
import os
import sys
import tempfile

GATES_DIR = os.path.join("scripts", "gates")
MARKS_REL = os.path.join("scripts", "公开标记.tsv")


def gate_scripts(root):
    d = os.path.join(root, GATES_DIR)
    if not os.path.isdir(d):
        return None
    out = []
    # 保 readdir 原序（零排序）
    for name in os.listdir(d):
        if name.endswith((".py", ".sh")):
            out.append(GATES_DIR + "/" + name)
    return out


def registered(root):
    p = os.path.join(root, MARKS_REL)
    if not os.path.isfile(p):
        return None
    got = set()
    with open(p, encoding="utf-8") as f:
        for ln in f:
            if not ln.strip() or ln.startswith("#"):
                continue
            got.add(ln.split("\t", 1)[0].strip())
    return got


def run(root, as_json):
    gs = gate_scripts(root)
    if gs is None:
        print("⛔ 不给结论（rc=2）：门件目录不在盘：%s" % os.path.join(root, GATES_DIR), file=sys.stderr)
        return 2
    reg = registered(root)
    if reg is None:
        print("⛔ 不给结论（rc=2）：标记表读不到：%s" % os.path.join(root, MARKS_REL), file=sys.stderr)
        return 2
    missing = [g for g in gs if g not in reg]
    if as_json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GateScriptRegistered",
                          "gate_scripts": len(gs), "unregistered": missing}, ensure_ascii=False))
    else:
        print("── 门件登记对拍（scripts/gates/* ⇕ 公开标记.tsv）──")
        print("  门件 %d 件 · 填表已登记 %d 件 · **未登记 %d 件**" % (len(gs), len(reg), len(missing)))
        for m in missing:
            print("  ✗ 未登记：%s" % m)
    return 1 if missing else 0


def self_test():
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, GATES_DIR))
        open(os.path.join(td, GATES_DIR, "check-a.py"), "w").write("# a\n")
        open(os.path.join(td, GATES_DIR, "check-b.sh"), "w").write("# b\n")
        open(os.path.join(td, MARKS_REL), "w", encoding="utf-8").write(
            "scripts/gates/check-a.py\tno\tB 类\n")
        if run(td, False) != 1:
            print("  ✗ 正控失败：应报未登记 check-b.sh ⇒ rc=1"); ok = False
        open(os.path.join(td, MARKS_REL), "a", encoding="utf-8").write(
            "scripts/gates/check-b.sh\tno\tB 类\n")
        if run(td, False) != 0:
            print("  ✗ 成对回绿失败：补登后应 rc=0"); ok = False
        os.remove(os.path.join(td, MARKS_REL))
        if run(td, False) != 2:
            print("  ✗ 负控失败：标记表缺 ⇒ rc=2"); ok = False
    print("自检 %s（3 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="门件登记对拍（GAP-20260927-206）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    return run(a.root, a.json)


if __name__ == "__main__":
    sys.exit(main())

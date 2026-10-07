#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-loose-exit-registry.py —— 「宽松出口档」门脚本**统一登记面**（缺口 `GAP-20260927-269`）

题面（缺口 `want_action` = `gate-loose-exit-registry`）：
    ★ `check-doc-meta.py` 类「有宽松出口档的门脚本」**无统一登记面**
    （哪个脚本有什么严格旗标，靠人读 argparse）。

口径（写死 —— 本件的判据）：
    · 扫面 = `scripts/gates/*.py`（可用 `--root` 换根，自检用）。
    · 「宽松出口档」的**机械判定**（`ast` 解析，不用正则猜）：
        某个 `add_argument("--X", …, default=<str>)` 的 **默认值是字符串 `warn` / `report`**
        ⇒ 该脚本**默认走只报告**（宽松出口）；
      另：`action="store_true"` 且旗标名逐字形如 `--strict-*` ⇒ 也算（它**开着才严**，缺省即宽松）。
    · **统一登记表** = 本件 `LOOSE_EXIT`（`{脚本: [{flag, default, strict, note}]}`）—— **单一真源**。
    · 判三档：
        A **漏登记**：扫面上某脚本检出宽松出口档、但该脚本不在 `LOOSE_EXIT` ⇒ 红（逐条点名）；
        B **登记陈旧**：`LOOSE_EXIT` 点名的脚本/旗标在盘上查无此字面（脚本改名/旗标删了）⇒ 红；
        C **未给严格旗标**：登记项 `strict` 为空 ⇒ rc=2（写不清严格旗标的登记 = 洗白）。
    · **退码写死**：`rc=0` 全绿 · `rc=1` A/B（逐条点名）· `rc=2` 扫面根缺件 / C / 空转。
    · **只读**：不写任何文件。

用法：
    python3 scripts/gates/check-loose-exit-registry.py [--root <dir>] [--json] [--self-test]

★ 接线状态：本件为**新门**（门108 · `gates` scope · `tri-report` 只报告档 · **不进** `FAST_STEPS`）。
  接线＝动步骤表 ＋ 公开标记表 ＋ 两器排除三表 ＋ judge-registry ＋ affected-map（**同批**）。
  本件与门⑩甲（步骤命令串必须带严格旗标）**分工不同**：甲判「步骤表带没带」，本件判
  「**脚本面有哪些宽松出口档、各自严格旗标叫什么**」并逼其逐条登记。判据件不开源。
"""
import argparse
import ast
import json
import os
import sys

# ── 宽松出口档统一登记表（单一真源）────────────────────────────────────────────
#   每项：flag（宽松旗标）· default（宽松默认档）· strict（严格旗标逐字形态）· note（为什么）。
LOOSE_EXIT = {
    "check-doc-meta.py": [
        {"flag": "--missing", "default": "warn", "strict": "--missing=fail",
         "note": "缺回填默认只警告；=fail 才判红（提交链上调它的步必须带严格档）"},
    ],
    "check-git-path-outlet.py": [
        {"flag": "--tier", "default": "report", "strict": "--tier rc",
         "note": "默认只报告；--tier rc 才把命中落成失败项"},
    ],
    "check-wired-scripts.py": [
        {"flag": "--strict-report", "default": "（缺省即宽松）", "strict": "--strict-report",
         "note": "缺省只报告未接线件；带 --strict-report 才升阻断"},
    ],
    "check-doc-freshness.py": [
        {"flag": "--strict-all", "default": "（缺省即宽松）", "strict": "--strict-all",
         "note": "缺省只报告过期件；带 --strict-all 才升阻断"},
    ],
    "check-no-symlink-farm.py": [
        {"flag": "--strict-links", "default": "（缺省即宽松）", "strict": "--strict-links",
         "note": "缺省只报告链接农场；带 --strict-links 才升阻断"},
    ],
    "check-version-sources.py": [
        {"flag": "--strict-docs", "default": "（缺省即宽松）", "strict": "--strict-docs",
         "note": "缺省只报告文档版本源漂移；带 --strict-docs 才升阻断"},
    ],
}

# `--strict-*` 形态（开着才严）——登记面必须含它
STRICT_TRUE_PREFIX = "--strict-"

# 已有 Go 真搬半的登记件（键 = 脚本基名 · 与 Go 半 `clerGoPorted` 逐条同）。
#   ★ 退役态安全：这些门的判据体已搬进 Go，py 原件退役（不在盘）属预期，不再算「B 登记陈旧」；
#   未 Go 化的登记件缺席仍判红（牙未放松）。py 在盘时 B 档旗标字面照验（逐字同旧）。
GO_PORTED = {
    "check-doc-meta.py",
    "check-git-path-outlet.py",
    "check-wired-scripts.py",
    "check-doc-freshness.py",
    "check-no-symlink-farm.py",
    "check-version-sources.py",
}


def detect(root):
    """扫 `scripts/gates/*.py` ⇒ (found, ast_errors)。found={脚本: [flag…]} 检出有宽松出口档的脚本。"""
    found, errs = {}, []
    d = os.path.join(root, "scripts", "gates")
    if not os.path.isdir(d):
        return None, errs
    for name in sorted(os.listdir(d)):
        if not name.endswith(".py"):
            continue
        p = os.path.join(d, name)
        try:
            tree = ast.parse(open(p, "r", encoding="utf-8", errors="replace").read())
        except SyntaxError as exc:
            errs.append((name, str(exc)))
            continue
        hits = []
        for node in ast.walk(tree):
            if not (isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute)
                    and node.func.attr == "add_argument"):
                continue
            flag, default, strict_true = None, None, False
            for a in node.args:
                if isinstance(a, ast.Constant) and isinstance(a.value, str) and a.value.startswith("--"):
                    flag = a.value
            for kw in node.keywords:
                if kw.arg == "default" and isinstance(kw.value, ast.Constant) and isinstance(kw.value.value, str):
                    default = kw.value.value
                if kw.arg == "action" and isinstance(kw.value, ast.Constant) and kw.value.value == "store_true":
                    strict_true = True
            if flag and default in ("warn", "report"):
                hits.append(flag)
            elif flag and strict_true and flag.startswith(STRICT_TRUE_PREFIX):
                hits.append(flag)
        if hits:
            found[name] = sorted(set(hits))
    return found, errs


def run(root, as_json):
    found, errs = detect(root)
    if found is None:
        if as_json:
            print(json.dumps({"rc": 2, "error": "scripts/gates 不存在"}, ensure_ascii=False))
        else:
            print("BLOCKED —— scripts/gates 不存在 ⇒ rc=2，不给结论", file=sys.stderr)
        return 2
    if errs:
        if as_json:
            print(json.dumps({"rc": 2, "errs": [{"file": f, "err": e} for f, e in errs]}, ensure_ascii=False))
        else:
            print("BLOCKED —— 脚本解析失败（取不到 ≠ 无宽松出口）⇒ rc=2：", file=sys.stderr)
            for f, e in errs:
                print("    %s ← %s" % (f, e), file=sys.stderr)
        return 2
    # C：登记项未给严格旗标 ⇒ rc=2
    bad_reg = sorted({s for s, items in LOOSE_EXIT.items() for it in items if not it.get("strict")})
    if bad_reg:
        if as_json:
            print(json.dumps({"rc": 2, "bad_registry": bad_reg}, ensure_ascii=False))
        else:
            print("BLOCKED —— 登记项缺「严格旗标」：%s ⇒ rc=2（写不清严格旗标 = 洗白）"
                  % "、".join(bad_reg), file=sys.stderr)
        return 2
    unregistered = sorted(s for s in found if s not in LOOSE_EXIT)
    # B：登记陈旧（脚本/旗标字面查无）
    stale = []
    for s, items in sorted(LOOSE_EXIT.items()):
        p = os.path.join(root, "scripts", "gates", s)
        if not os.path.isfile(p):
            # ★ 退役态安全：已 Go 真搬（GO_PORTED）的登记件，py 退役（不在盘）属预期 ⇒ 不算陈旧；
            # 未 Go 化件缺席仍判红（牙不放松）。
            if s not in GO_PORTED:
                stale.append("%s（脚本不在盘）" % s)
            continue
        src = open(p, "r", encoding="utf-8", errors="replace").read()
        for it in items:
            if it["flag"] not in src:
                stale.append("%s 的 %s（旗标字面查无）" % (s, it["flag"]))
    bad = len(unregistered) + len(stale)
    if as_json:
        print(json.dumps({"rc": 1 if bad else 0, "detected": found,
                          "unregistered": unregistered, "stale": stale}, ensure_ascii=False))
    else:
        print("「宽松出口档」门脚本统一登记面（GAP-20260927-269）")
        print("检出有宽松出口档的脚本 = %d 只 · 登记表 = %d 行" % (len(found), len(LOOSE_EXIT)))
        print("── A 漏登记（检出却没进登记表）：%d 处" % len(unregistered))
        for x in unregistered:
            print("   ✗ %s（检出宽松旗标：%s）" % (x, "、".join(found[x])))
        print("── B 登记陈旧（表里点名、脚本/旗标查无）：%d 处" % len(stale))
        for x in stale:
            print("   ✗ %s" % x)
        if bad:
            print("⇒ 违规 %d 处（rc=1）：宽松出口档必须逐条登记（谁有、默认怎么算、严格旗标叫什么）" % bad)
        else:
            print("⇒ 全绿（rc=0）：%d 只脚本的宽松出口档逐条登记 · 严格旗标齐全" % len(found))
    return 1 if bad else 0


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts", "gates"))
        p = os.path.join(td, "scripts", "gates")
        open(os.path.join(p, "check-doc-meta.py"), "w").write(
            "import argparse\nap=argparse.ArgumentParser()\n"
            "ap.add_argument('--missing', choices=['warn','fail'], default='warn')\n")
        saved = dict(LOOSE_EXIT)

        def use(reg):
            LOOSE_EXIT.clear(); LOOSE_EXIT.update(reg)

        DOC = {"check-doc-meta.py": [
            {"flag": "--missing", "default": "warn", "strict": "--missing=fail", "note": "n"}]}
        try:
            # ① 正控：检出 default=warn 且已登记 ⇒ rc=0
            use(DOC)
            if run(td, False) != 0:
                print("  ✗ ① 正控失败：已登记应回 0"); ok = False
            # ② must-fail A：另一脚本 default=report 未登记 ⇒ rc=1
            open(os.path.join(p, "check-new-thing.py"), "w").write(
                "import argparse\nap=argparse.ArgumentParser()\nap.add_argument('--tier', default='report')\n")
            if run(td, False) != 1:
                print("  ✗ ② A 失败：未登记宽松出口应回 1"); ok = False
            # ③ must-fail B：登记表点名一个盘上没有的 ⇒ rc=1
            os.remove(os.path.join(p, "check-new-thing.py"))
            reg = dict(DOC); reg["ghost.py"] = [
                {"flag": "--x", "default": "report", "strict": "--x rc", "note": "幽灵"}]
            use(reg)
            if run(td, False) != 1:
                print("  ✗ ③ B 失败：陈旧登记应回 1"); ok = False
            # ④ must-fail C：登记项缺严格旗标 ⇒ rc=2
            use({"check-doc-meta.py": [
                {"flag": "--missing", "default": "warn", "strict": "", "note": "空"}]})
            if run(td, False) != 2:
                print("  ✗ ④ C 失败：缺严格旗标应回 2"); ok = False
        finally:
            LOOSE_EXIT.clear(); LOOSE_EXIT.update(saved)
    print("自检 %s（4 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="「宽松出口档」门脚本统一登记面（GAP-20260927-269）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

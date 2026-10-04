#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""m2-go-py-isomorphism-scan.py —— M2「已落 Go 件」的 py 语义同构**合规普查**（候件）

出处（缺口 want_action，逐字）：
  GAP-20261001-51 ·「收口批对全部 M2 已落件做一次新口径合规普查（filepath.Join/Abs/Rel/
  EvalSymlinks 用量 · 正则未展开类 · os.ReadDir/WalkDir 排序面 · DirEntry.IsDir 的 Lstat 误用）
  ⇒ 产清单并定『哪些必须重落』」。
  GAP-20261001-70/-75 ·（inproc 分档 / 新表两难）需要「107 件 Go 搬体」的**件面清单** —— 本件
  现读产出的件表正好是那一档的输入。

为什么需要它：laneB460 当年产出的普查清单只落在 `~/.hermes/cache/scratch/...`（**24h 即被清**）
⇒ 清单不可复现（正是 GAP-20261001-71 那类「清单不进持久件」的病灶）。本件把普查**脚本化**，
清单可随时现跑复现，落仓外 TSV/JSON。

口径（**只读 · 现算 · 不做判定档**：本件只出清单与计数，**哪些必须重落**归拍板）：
  对每个 `core/cmd/zerg/*.go`（去测试件）现读计数：
    · filepath.Join / Abs / Rel / EvalSymlinks —— 新口径要求自写 PyJoin/PyAbs 一族（复刻 py 的
      纯串拼接/解析语义），裸算子 = 应改项；
    · os.ReadDir / filepath.WalkDir —— 排序面：py os.walk 产出序 vs Go ReadDir 原序（是否可观测出口）；
    · DirEntry.IsDir —— Go 用 lstat 语义、py entry.is_dir() 跟随符号链接 ⇒ 指向目录的符号链接判反；
    · 未展开正则类 `\\s`/`\\w`/`\\d`/`\\b` 出现在 Go 字符串字面量里 —— 新口径要求显式 Unicode 类；
    · 自写同构助手（`Py` 前缀函数名）。

退码（两档 · 本件是**普查**不是判据）：
  0 = 扫描完成（清单照出；红/绿由收口批按清单拍板）
  2 = **不给结论**：扫描根不在盘 / `--self-test` 不过

用法：
    python3 m2-go-py-isomorphism-scan.py --root <Zerg 仓根>            # 出 TSV
    python3 m2-go-py-isomorphism-scan.py --root <仓根> --json
    python3 m2-go-py-isomorphism-scan.py --self-test
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import tempfile

VERSION = "0.1.0（候件 · 未接线 · 缺口 GAP-20261001-51）"
SCAN_REL = os.path.join("core", "cmd", "zerg")

COUNTERS = {
    "join": r"filepath\.Join\s*\(",
    "abs": r"filepath\.Abs\s*\(",
    "rel": r"filepath\.Rel\s*\(",
    "evalsymlinks": r"filepath\.EvalSymlinks\s*\(",
    "readdir": r"os\.ReadDir\s*\(",
    "walkdir": r"filepath\.WalkDir\s*\(",
    "direntry_isdir": r"\.IsDir\s*\(",
    "pyhelper": r"\bPy[A-Z][A-Za-z0-9]*\s*\(",
}
# 未展开正则类：Go 字符串字面量里的 \s \w \d \b（反斜杠转义）
REGEX_SKEL_RE = re.compile(r"\\(s|w|d|b)\b")


def scan(root: str):
    d = os.path.join(root, SCAN_REL)
    if not os.path.isdir(d):
        return None, "扫描根不在盘：%s" % d
    rows = []
    for fn in sorted(os.listdir(d)):
        if not fn.endswith(".go") or fn.endswith("_test.go"):
            continue
        try:
            txt = open(os.path.join(d, fn), encoding="utf-8").read()
        except OSError:
            continue
        c = {k: len(re.findall(rx, txt)) for k, rx in COUNTERS.items()}
        # 脏正则类：粗略抽字符串字面量里的转义（普查面，非判据）
        unexp = len(REGEX_SKEL_RE.findall(txt))
        raw_ops = c["join"] + c["abs"] + c["rel"] + c["evalsymlinks"]
        rows.append({"file": fn, "join": c["join"], "abs": c["abs"], "rel": c["rel"],
                     "evalsymlinks": c["evalsymlinks"], "readdir": c["readdir"],
                     "walkdir": c["walkdir"], "direntry_isdir": c["direntry_isdir"],
                     "regex_unexpanded": unexp, "pyhelper": c["pyhelper"],
                     "raw_path_ops": raw_ops})
    rows.sort(key=lambda r: (-r["raw_path_ops"], -r["regex_unexpanded"], r["file"]))
    return rows, ""


def self_test(out) -> bool:
    d = tempfile.mkdtemp(prefix="m2scan-selftest-")
    os.makedirs(os.path.join(d, SCAN_REL))
    open(os.path.join(d, SCAN_REL, "x.go"), "w").write(
        'package main\nimport "path/filepath"\n'
        'func f(root string) { _ = filepath.Join(root, "a"); _ = os.ReadDir(root); '
        '_ = PyJoin("a","b"); re := "\\\\s+"; _ = re }\n')
    open(os.path.join(d, SCAN_REL, "x_test.go"), "w").write("package main\nvar _ = filepath.Join\n")
    rows, err = scan(d)
    cases = [
        ("① 扫描出来一件（测试件排除）", err == "" and [r["file"] for r in rows] == ["x.go"]),
        ("② filepath.Join 计数=1", rows and rows[0]["join"] == 1),
        ("③ os.ReadDir 计数=1", rows and rows[0]["readdir"] == 1),
        ("④ 自写 PyJoin 计数=1", rows and rows[0]["pyhelper"] == 1),
        ("⑤ 脏正则类计数>=1", rows and rows[0]["regex_unexpanded"] >= 1),
    ]
    ok = True
    for name, good in cases:
        out.write("  %s %s\n" % ("✓" if good else "✗", name))
        ok = ok and good
    out.write("  self-test: %d/%d\n" % (sum(1 for _, g in cases if g), len(cases)))
    return ok


def main() -> int:
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return 0 if self_test(sys.stdout) else 2
    root = a.root or os.getcwd()
    rows, err = scan(root)
    if err:
        sys.stderr.write("%s: %s ⇒ 不给结论（退码 2）\n" % (VERSION, err))
        return 2
    if a.json:
        print(json.dumps({"schema": "zerg/m2-go-py-isomorphism/v1", "rows": rows}, ensure_ascii=False))
        return 0
    hdr = ["file", "join", "abs", "rel", "evalsymlinks", "readdir", "walkdir",
           "direntry_isdir", "regex_unexpanded", "pyhelper", "raw_path_ops"]
    try:
        print("\t".join(hdr))
        for r in rows:
            print("\t".join(str(r[k]) for k in hdr))
        tot = {k: sum(r[k] for r in rows) for k in hdr if k != "file"}
        print("# 合计：%s" % json.dumps(tot, ensure_ascii=False))
        print("# 说明：raw_path_ops = join+abs+rel+evalsymlinks（裸算子用量，新口径应改自写 Py* 一族）；")
        print("#       「哪些必须重落」归收口批拍板 —— 本件只出可复现清单。")
    except BrokenPipeError:  # `| head` 等下游提前关管道时正常退出
        try:
            sys.stdout.close()
        except Exception:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main())

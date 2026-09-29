#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""repo-diffstat-check.py —— T20 判据体（仓外副本 · 由 GG-G4 原型演进 · 只读）。

与正门 `zerg repo diffstat`（`core/cmd/zerg/family_repo_diffstat.go` · **已接进命令树**）
**同一套判据**：报行数一律三格 —— `原始 diff` / `增` / `减`。
只给净数（或缺任一格）⇒ **未报 ⇒ 判不了（rc=2，非 0）**，不许当绿。

三格认法（点名格位 · 不抄行号）：
  · `原始 diff N`
  · 分隔符 + `增 N`   （分隔符 = 空白 / : ： = · | , ，（ ( 【）
  · 分隔符 + `减 N`
★ 增/减 要求**前置分隔符** ⇒ `净增 96`（净数措辞里的「增」）不会被误当成 add 格。

本档只读：只吃 `--file` / stdin 给的 diffstat 文本，**一字不写输入件**（成功态字节不变）。
退出码（与正门同规）：
  0 = 三格齐   ·   1 = 输入为空（无读数 · 判不了）   ·   2 = 缺格（未报 · 判不了）   ·   8 = 件读不到（判不了）
"""
import argparse
import hashlib
import re
import sys

WANT = (("raw", "原始 diff"), ("add", "增"), ("del", "减"))
PAT = {
    "raw": re.compile(r"原始\s*diff\s*[:：=]?\s*(\d+)"),
    "add": re.compile(r"(?:^|[\s:：=·|,，、（(【])增\s*[:：=]?\s*(\d+)"),
    "del": re.compile(r"(?:^|[\s:：=·|,，、（(【])减\s*[:：=]?\s*(\d+)"),
}
NET_ONLY = re.compile(r"(净|net)")


def judge(text):
    """⇒ (rc, 缺格列表, 命中格字典)。空输入 ⇒ rc=1（无读数也判不了）。"""
    hits = {k: PAT[k].search(text).group(1) for k, _ in WANT if PAT[k].search(text)}
    missing = [k for k, _ in WANT if k not in hits]
    if not text.strip():
        return 1, missing, hits
    return (0 if not missing else 2), missing, hits


def main():
    ap = argparse.ArgumentParser(description="报数三格机检（只读 · 缺格即判不了 · 与正门 repo diffstat 同规）")
    ap.add_argument("--file", default="", help="diffstat 文本件（缺省读 stdin）")
    a = ap.parse_args()
    if a.file:
        try:
            text = open(a.file, encoding="utf-8").read()
        except OSError as e:
            print("zerg: **判不了** ⇒ 不给结论（退码 8）\n  --file %s：%s" % (a.file, e), file=sys.stderr)
            print("口径：读不到既不是「三格齐」也不是「缺格」—— 不许当绿", file=sys.stderr)
            return 8
    else:
        text = sys.stdin.read()
    before = hashlib.sha256(text.encode("utf-8")).hexdigest()
    rc, missing, hits = judge(text)
    if rc == 1:
        print("!! 输入为空 ⇒ 无读数 ⇒ 判不了（rc=1）· 不许当绿", file=sys.stderr)
        return 1
    if missing:
        nm = [n for k, n in WANT if k in missing]
        print("✗ 未报：缺 %d 格（%s）—— 报行数一律三格（原始 diff / 增 / 减），只给净数视为未报"
              % (len(missing), " · ".join(nm)), file=sys.stderr)
        if NET_ONLY.search(text):
            print("  （现读含「净」字样 ⇒ 只给了净数）", file=sys.stderr)
        print("结论：判不了 ⇒ 不给结论（退 2）", file=sys.stderr)
        return 2
    print("✓ 三格齐：原始 diff=%s · 增=%s · 减=%s" % (hits["raw"], hits["add"], hits["del"]))
    assert hashlib.sha256(text.encode("utf-8")).hexdigest() == before, "输入件被改"
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""repo-diffstat-check.py —— T20 判据体（**薄壳委托** · 只读）。

★ 2026-10-04（缺口 `GAP-20261001-15` · 退场口径「薄壳委托」）：本件原来是判据的**第二实现**
（与正门 `zerg repo diffstat --report <件>` 同套判据 · 各写一份解析）。同物料两处解析 =
**双真源**，迟早漂 ⇒ 本件改**薄壳**：保留入口与退码，把判定**整件委托**给正门，逐字透传 rc。

判据本体与退码口径都在正门（`core/cmd/zerg/family_repo_diffstat.go`）一处：
  三格齐 ⇒ 0 · 输入为空 ⇒ 1 · 缺格（含只给净数）⇒ 2 · 件读不到 ⇒ 8。

接口映射：`--file <件>` ⇒ 正门 `repo diffstat --report <件>`；不给 `--file` ⇒ 读 stdin 落临时件再转交
（正门只吃件路径）。**本件不写任何仓内件**（临时件落系统临时目录并回收）。
"""
import argparse
import os
import subprocess
import sys
import tempfile


def main():
    ap = argparse.ArgumentParser(
        description="报数三格机检（**薄壳** → 正门 `zerg repo diffstat` · 只读 · 缺格即判不了）")
    ap.add_argument("--file", default="", help="diffstat 文本件（缺省读 stdin）")
    a = ap.parse_args()

    zerg = os.environ.get("ZERG_BIN") or "zerg"
    tmp = None
    if a.file:
        target = a.file
    else:
        text = sys.stdin.read()
        fd, tmp = tempfile.mkstemp(prefix="repo-diffstat-", suffix=".txt")
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(text)
        target = tmp
    try:
        p = subprocess.run([zerg, "repo", "diffstat", "--report", target])
        return p.returncode
    except FileNotFoundError:
        print("zerg: **判不了** ⇒ 不给结论（退码 8）\n  正门 `%s repo diffstat` 不可用"
              % zerg, file=sys.stderr)
        print("口径：薄壳件不自己判 —— 正门取不到 ⇒ 不许当绿", file=sys.stderr)
        return 8
    finally:
        if tmp is not None:
            try:
                os.unlink(tmp)
            except OSError:
                pass


if __name__ == "__main__":
    sys.exit(main())

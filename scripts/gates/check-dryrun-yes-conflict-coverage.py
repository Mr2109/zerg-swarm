#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-dryrun-yes-conflict-coverage.py —— `--dry-run` 与 `--yes` 同给判定的**覆盖反查门**（候件）

出处（缺口的 want_action，逐字）：
  GAP-20260927-392 ·「无门断言『某命令必须走到同给判定』…加一条按错误码反查的门：从
  dry_run_yes_conflict 列应产者，与真产者对账（缺者报红）」
  GAP-20260927-374 ·「dry-run 与 yes 互斥语义仍是两套 … 新增 dryRunYesConflict(inv) 共享判定，
  18 条逐条在唯一写盘点之前调用」
  GAP-20260927-389 ·「同给态无机器面读数 … 给 dryRunYesConflict 一档机器面」

本件做的事（**只读 · 不开子进程 · 不改任何状态**）：
  应产者 = 件读 `inv.dryRun`（命令接受了干跑态）的件（`core/cmd/zerg/*.go`，去测试件）;
  真产者 = 件里出现共享判定 `dryRunYesConflict(` 或执行门 `cmdGuarded(` 的件。
  对账：应产者 ∖ 真产者 里，落在**冻结名单**（= 只读干跑面，见 READ_ONLY_FACES）之外的 ⇒ **报红**。

为什么是「件级」而不是「命令级」：命令树路径 → 处理件 的映射要另解析 `main.go` 的 `run:` 登记，
  那会把本门绑死到命令树的内部形状（口径未定 · 见 GAP-20261001-75 那类结构两难）。件级对账
  是**现读可判**的最小面，且足以抓住「这枚件收了 --dry-run 却从不判同给」这一类欠账。

退码（三档 · fail-closed）：
  0 = 全部应产者都在真产者里（或落在冻结名单）
  1 = 有应产者件既无共享判定、也不在冻结名单 ⇒ 报红（点名到件）
  2 = **不给结论**：扫描根不在盘 / `--self-test` 不过

用法：
    python3 check-dryrun-yes-conflict-coverage.py --root <Zerg 仓根>
    python3 check-dryrun-yes-conflict-coverage.py --root <仓根> --json
    python3 check-dryrun-yes-conflict-coverage.py --self-test
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import tempfile

VERSION = "0.1.0（已接线 · 门98 · tri-report 只报告 · 缺口 GAP-20260927-392/374/389）"

SCAN_REL = os.path.join("core", "cmd", "zerg")
READS_RE = re.compile(r"\binv\.dryRun\b")
# 真产者：共享判定 `dryRunYesConflict` 或执行门 `cmdGuarded`（含把它作函数值登记 `run: cmdGuarded`）。
HELPER_RE = re.compile(r"\bdryRunYesConflict\b|\bcmdGuarded\b")
# 非命令处理件（现读 · 理由）：
#   `main.go` = 命令树定义 + **全局旗标解析器**（`main.go` 里那处 `inv.dryRun = true` 只是把
#   `--dry-run` 记进 invocation，不是某个命令的写盘点）⇒ 不在「应产者」面。
NON_HANDLER_FILES = ["main.go"]

# ── 冻结名单（**现读取数** · 非白名单式的「按名放行」：它登记的是「只读干跑面」这一**性质**的件）──
#   `export.go` 的 `--dry-run` 是**只读导出物**（只打印清单、一个字节不写）——它既不真写，
#   就不存在「--yes 被静默吞」的病灶（见 dryrun-semantics.json 里 export.go 那条登记）。
#   ★ 名单增删必须走 --emit-baseline 现读（本候件从简：直接常量 + 自检里判形态）。
READ_ONLY_FACES = ["export.go"]


def scan(root: str):
    d = os.path.join(root, SCAN_REL)
    if not os.path.isdir(d):
        return None, "扫描根不在盘：%s" % d
    rows = []
    for fn in sorted(os.listdir(d)):
        if not fn.endswith(".go") or fn.endswith("_test.go"):
            continue
        if fn in NON_HANDLER_FILES:
            continue
        p = os.path.join(d, fn)
        try:
            with open(p, encoding="utf-8") as f:
                txt = f.read()
        except OSError:
            continue
        reads = bool(READS_RE.search(txt))
        helper = bool(HELPER_RE.search(txt))
        if reads:
            rows.append({"file": fn, "reads_dry_run": True,
                         "has_conflict_helper": helper})
    return rows, ""


def judge(rows, read_only=None):
    read_only = READ_ONLY_FACES if read_only is None else read_only
    missing = [r["file"] for r in rows
               if not r["has_conflict_helper"] and r["file"] not in read_only]
    covered = [r["file"] for r in rows if r["has_conflict_helper"]]
    exempt = [r["file"] for r in rows
              if not r["has_conflict_helper"] and r["file"] in read_only]
    return missing, covered, exempt


def self_test(out) -> bool:
    ok = True
    total = 0
    d = tempfile.mkdtemp(prefix="drcov-selftest-")
    os.makedirs(os.path.join(d, SCAN_REL))
    def put(name, body):
        with open(os.path.join(d, SCAN_REL, name), "w", encoding="utf-8") as f:
            f.write(body)
    put("a.go", "package main\nfunc f(inv *invocation) int { if inv.dryRun { return 0 }; return 1 }\n")
    put("b.go", "package main\nfunc f(inv *invocation) int { if rc := dryRunYesConflict(inv, nil); rc != 0 { return rc }; _ = inv.dryRun; return 0 }\n")
    put("c.go", "package main\nfunc f(inv *invocation) int { if inv.dryRun { return 0 }; return cmdGuarded(inv, nil, nil) }\n")
    put("d_test.go", "package main\nfunc t() { _ = inv.dryRun }\n")  # 测试件：排除
    rows, err = scan(d)
    cases = []
    cases.append(("① 应产者被抓住（a.go 无判定）", err == "" and "a.go" in [r["file"] for r in rows]))
    missing, covered, exempt = judge(rows)
    cases.append(("② a.go 报红（缺者报红）", missing == ["a.go"]))
    cases.append(("③ b.go/c.go 真产者不入缺", set(covered) == {"b.go", "c.go"}))
    cases.append(("④ 测试件被排除", all(r["file"] != "d_test.go" for r in rows)))
    cases.append(("⑤ 冻结名单豁免", judge(rows, read_only=["a.go"])[2] == ["a.go"]))
    # ⑥ 空域负控（GAP-20260927-108）：0 件读 inv.dryRun ⇒ 应产者为空（main 里 ⇒ rc=2，✗ 不许当全绿）
    empty = tempfile.mkdtemp(prefix="drcov-empty-")
    os.makedirs(os.path.join(empty, SCAN_REL))
    rows_e, err_e = scan(empty)
    cases.append(("⑥ 空域：0 应产者（空域 ≠ 全绿）", err_e == "" and rows_e == []))
    for name, good in cases:
        total += 1
        out.write("  %s %s\n" % ("✓" if good else "✗", name))
        ok = ok and good
    out.write("  self-test: %d/%d\n" % (sum(1 for _, g in cases if g), total))
    return ok


def main() -> int:
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--emit-baseline", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.self_test:
        good = self_test(sys.stdout)
        return 0 if good else 2

    root = a.root or os.getcwd()
    rows, err = scan(root)
    if err:
        sys.stderr.write("%s: %s ⇒ 不给结论（退码 2）\n" % (VERSION, err))
        return 2
    if not rows:  # ★ 空集守卫（GAP-20260927-108）：空域（0 应产者）⇒ fail-closed（✗ 不许判「全绿」）
        sys.stderr.write("%s: 空域 —— %s 下 0 件读 `inv.dryRun`（取不到输入 ⇒ 空域 ≠ 全绿）⇒ 不给结论（退码 2）\n"
                         % (VERSION, SCAN_REL))
        return 2
    missing, covered, exempt = judge(rows)
    if a.emit_baseline:
        print(json.dumps({"read_only_faces": READ_ONLY_FACES}, ensure_ascii=False))
        return 0
    if a.json:
        print(json.dumps({"schema": "zerg/dryrun-yes-conflict-coverage/v1",
                          "reads": [r["file"] for r in rows],
                          "covered": covered, "exempt": exempt, "missing": missing},
                         ensure_ascii=False))
        return 1 if missing else 0
    print("%s：应产者 %d 件 · 真产者 %d 件 · 冻结名单 %d 件"
          % (VERSION, len(rows), len(covered), len(exempt)))
    if missing:
        print("⇒ 报红（rc=1）：下列件读 `inv.dryRun` 却既无共享判定、也不在只读干跑冻结名单 ——")
        print("   「--dry-run 与 --yes 同给时静默吞 --yes」的病灶面：")
        for f in missing:
            print("   - core/cmd/zerg/%s" % f)
        print("   治法：在**唯一写盘点之前**插 `if rc := dryRunYesConflict(inv, stderr); rc != exitOK { return rc }`。")
        return 1
    print("⇒ 全绿（rc=0）：应产者全部落在真产者或只读干跑冻结名单里。")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-marks-tracked-parity.py —— GAP-20260927-408 的**对拍腿**（**已挂门94** · tri-report 只报告 · 只读）。

病灶（账内逐字）：`scripts/公开标记.tsv` 里「跟踪状态」类**时点读数**无人对拍、静默过期
（`:229 仍写「本件现读未跟踪」而该件已跟踪`）。

本件只做一件事：把表内每一行「理由」列里**声称未跟踪**的行，用 `git ls-files --error-unmatch`
现读对拍；表文说「未跟踪」而现读**已跟踪** ⇒ 判红并逐行点名（行号 + 件路径 + 两侧读数）。

退码：0 全对拍 · 1 有过期行（逐行点名）· 2 不给结论（表不在盘 / 读不了 / err）。

★ 成对正负控（本片补 · `--self-test`）：合成 git 夹具落仓外临时树，跑「正控=一致 ⇒ 0 /
负控=过期 ⇒ 1 且点名 / 换回 ⇒ 0 / 空转 ⇒ 2 / 缺件 ⇒ 2」五格，缺一不算自检。
口径：读不到不许当绿；本件只读，一字不写被查件。
"""
import argparse
import os
import shutil
import subprocess
import sys
import tempfile

DEFAULT_TABLE = "scripts/公开标记.tsv"
CLAIM = "未跟踪"


def git_tracked(root, path):
    r = subprocess.run(["git", "-C", root, "ls-files", "--error-unmatch", path],
                       capture_output=True, text=True)
    return r.returncode == 0


def judge(root, table_rel):
    """对拍判定（自检可直接调）。⇒ (rc, stale:[(line,path)], checked, reason)。
    rc: 0 全对拍 · 1 有过期行 · 2 不给结论。"""
    tpath = os.path.join(root, table_rel)
    if not os.path.isfile(tpath):
        return 2, [], 0, "table-absent"
    try:
        lines = open(tpath, encoding="utf-8").read().splitlines()
    except OSError:
        return 2, [], 0, "table-unreadable"
    stale, checked = [], 0
    for i, l in enumerate(lines, 1):
        cols = l.split("\t")
        if len(cols) < 3:
            continue
        f, reason = cols[0].strip(), cols[2]
        if CLAIM not in reason:
            continue
        checked += 1
        if git_tracked(root, f):
            stale.append((i, f))
    if checked == 0:
        return 2, [], 0, "empty-domain"
    if stale:
        return 1, stale, checked, "stale"
    return 0, [], checked, "ok"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=".")
    ap.add_argument("--table", default=DEFAULT_TABLE)
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return run_self_test()
    root = os.path.abspath(a.root)
    rc, stale, checked, reason = judge(root, a.table)
    if reason in ("table-absent", "table-unreadable"):
        print("⛔ 不给结论（rc=2）：表不在盘 / 读不了 %s" % a.table, file=sys.stderr)
        return 2
    if reason == "empty-domain":
        print("⛔ 不给结论（rc=2）：表内 0 行声称「%s」（空转 = 假覆盖）" % CLAIM, file=sys.stderr)
        return 2
    if reason == "stale":
        print("✗ 过期登记 %d 行（表文说「%s」· `git ls-files` 现读为**已跟踪**）："
              % (len(stale), CLAIM), file=sys.stderr)
        for i, f in stale:
            print("  %s:%d  %s" % (a.table, i, f), file=sys.stderr)
        print("判据：时点读数必须与现读对拍 —— 表文写「未跟踪」而现读已跟踪 ⇒ 判不了 ⇒ 不给结论（退 1）")
        return 1
    print("✓ 对拍通过：%d 行「%s」声明与 `git ls-files` 现读一致" % (checked, CLAIM))
    return 0


# ── 自检（合成 git 夹具 · 一律落仓外临时树 · 成对正负控）────────────────────────
SELF_TOTAL = [0]
SELF_BAD = [0]


def assert_eq(name, got, want):
    SELF_TOTAL[0] += 1
    if got != want:
        SELF_BAD[0] += 1
        print("  ✗ 断言失败：%s —— 得到 %r，期望 %r" % (name, got, want))


def _mk_git_repo(tmp):
    """合成一枚真 git 仓：`tracked.go` 已提交（现读已跟踪）· `untracked.go` 未跟踪。"""
    r = os.path.join(tmp, "repo")
    os.makedirs(r, exist_ok=True)
    with open(os.path.join(r, "tracked.go"), "w", encoding="utf-8") as f:
        f.write("package x\n")
    subprocess.run(["git", "init", "-q"], cwd=r, capture_output=True, text=True)
    subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "add", "tracked.go"],
                   cwd=r, capture_output=True, text=True)
    subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x"],
                   cwd=r, capture_output=True, text=True)
    with open(os.path.join(r, "untracked.go"), "w", encoding="utf-8") as f:
        f.write("package x\n")
    return r


def _write_table(repo, rel, rows):
    p = os.path.join(repo, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as f:
        for f0, reason in rows:
            f.write("%s\tscripts\t%s\n" % (f0, reason))
    return rel


def run_self_test():
    print("公开标记跟踪对拍 · 自检（合成 git 夹具 · 临时树 · 成对正负控）")
    tmp = tempfile.mkdtemp(prefix="pmtp-selftest-")
    try:
        repo = _mk_git_repo(tmp)
        rel = DEFAULT_TABLE
        # ① 正控：声明「未跟踪」↔ 现读真未跟踪 ⇒ rc=0
        _write_table(repo, rel, [("untracked.go", "本件现读未跟踪")])
        rc, stale, checked, _ = judge(repo, rel)
        assert_eq("① 正控：未跟踪声明与现读一致 ⇒ rc=0", rc, 0)
        assert_eq("① 正控：恰对拍 1 行", checked, 1)
        # ② 负控：声明「未跟踪」而现读已跟踪 ⇒ rc=1 且点名 件:行
        _write_table(repo, rel, [("tracked.go", "本件现读未跟踪")])
        rc, stale, checked, _ = judge(repo, rel)
        assert_eq("② 负控：声明未跟踪↔现读已跟踪 ⇒ rc=1", rc, 1)
        assert_eq("② 负控：点名恰 1 行且两侧读数对", stale, [(1, "tracked.go")])
        # ③ 负控（换回）：把②改回真未跟踪 ⇒ 又绿
        _write_table(repo, rel, [("untracked.go", "本件现读未跟踪")])
        assert_eq("③ 负控（换回）：⇒ rc=0", judge(repo, rel)[0], 0)
        # ④ 空转负控：表内 0 行「未跟踪」⇒ rc=2（假覆盖）
        _write_table(repo, rel, [("tracked.go", "本件现读已跟踪（不进对拍面）")])
        assert_eq("④ 空转负控：0 行未跟踪声明 ⇒ rc=2", judge(repo, rel)[0], 2)
        # ⑤ 缺件负控：表不在盘 ⇒ rc=2
        assert_eq("⑤ 缺件负控：表不在盘 ⇒ rc=2", judge(repo, "scripts/没有这一份.tsv")[0], 2)
        print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (SELF_TOTAL[0], SELF_BAD[0]))
        return 0 if SELF_BAD[0] == 0 else 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-mkdir-temp-cleanup.py —— GAP-20261001-58 的**机器闸候件**（仓外 · 只读 · 只报告）。

账内「想要」逐字：落一台机器闸 —— 凡新增 `MkdirTemp` 必须有同函数内 `RemoveAll`
（或集中封装成带回收的构造器），以免再靠人记。

★ 第一版按账面字面（「同函数内 RemoveAll」）判，实测 **137/160** —— 过报，因为大量调用点
  是**helper**（建目录的函数自己不删、由调用方/`t.Cleanup`/`defer` 删），字面规则把它们全判漏。
  ⇒ 本版分三档（才判得动）：
    T1 同函数内有 `RemoveAll`（含 `defer`）⇒ 干净
    T2 同函数内无、但**同件**有 `RemoveAll`（helper / 调用方清）⇒ **只报告**
    T3 同件内**一个 `RemoveAll` 都没有** ⇒ 判漏（rc=1）
  白名单（每行带理由）压 T3。退码：0 零 T3 / 1 有 T3 / 2 不给结论（0 命中 = 空转）。
★ 本件**只报告**；**已挂门95**（tri-report）· 2026-10-04 现读更正（原「未接线」已过期）。

★ 成对正负控（本片补 · `--self-test`）：合成 Go 夹具落仓外临时树，跑「正控 T1 ⇒ 0 /
  负控 T3 ⇒ 1 且点名 件:行 / T2 helper ⇒ 0（只报告）/ 白名单 ⇒ 0 / 空转 ⇒ 2」五格，
  缺一不算自检。**白名单负控同时证明「压 T3 的出口」没把判据改瞎**。
"""
import argparse
import os
import re
import shutil
import sys
import tempfile

MKDIR = re.compile(r"os\.MkdirTemp\(")
RMALL = re.compile(r"os\.RemoveAll\(")
FUNCRE = re.compile(r"^func\s")


def func_bounds(lines, idx):
    s = 0
    for j in range(idx, -1, -1):
        if FUNCRE.match(lines[j]):
            s = j
            break
    depth, seen = 0, False
    # ★ 从**函数定义行** `s` 起算花括号（含该函数自己的 `{`）—— 若从 MkdirTemp 那行起算，
    #   外层函数的 `{` 不在扫描窗内 ⇒ 首个 `}` 把 depth 压到 -1 ⇒ 体一直延到文件尾，
    #   于是「同函数」判成「同件」（T1/T2 混档）。收窄到 [s, 配平处] 才与 docstring 的
    #   「同函数内 RemoveAll」语义一致（本修复不改 rc：T3 是**件级**判据、与边界无关）。
    for j in range(s, len(lines)):
        for ch in lines[j]:
            if ch == "{":
                depth += 1
                seen = True
            elif ch == "}":
                depth -= 1
                if seen and depth == 0:
                    return s, j
    return s, len(lines) - 1


def scan(root, dirs, whitelist):
    """⇒ (sites, t1, t2, t3:[件:行], t3f:[未白名单的 T3])。白名单按件相对路径逐行过滤。"""
    wl = set(whitelist)
    t1 = t2 = 0
    t3 = []
    sites = 0
    for d in dirs.split(","):
        for dp, dn, fn in os.walk(os.path.join(root, d)):
            if "/vendor" in dp or "/testdata" in dp:
                continue
            for f in fn:
                if not f.endswith(".go"):
                    continue
                p = os.path.join(dp, f)
                rel = os.path.relpath(p, root)
                try:
                    lines = open(p, encoding="utf-8").read().splitlines()
                except Exception:
                    continue
                file_has_rall = any(RMALL.search(l) for l in lines)
                for i, l in enumerate(lines):
                    if not MKDIR.search(l):
                        continue
                    sites += 1
                    s, e = func_bounds(lines, i)
                    body = "\n".join(lines[s:e + 1])
                    if RMALL.search(body):
                        t1 += 1
                    elif file_has_rall:
                        t2 += 1
                    else:
                        t3.append("%s:%d" % (rel, i + 1))
    t3f = [x for x in t3 if x.split(":")[0] not in wl]
    return sites, t1, t2, t3, t3f


def judge(root, dirs, whitelist):
    """⇒ (rc, sites, t1, t2, t3, t3f)。rc: 0 零 T3 / 1 有 T3 / 2 空转不给结论。"""
    sites, t1, t2, t3, t3f = scan(root, dirs, whitelist)
    if sites == 0:
        return 2, sites, t1, t2, t3, t3f
    if t3f:
        return 1, sites, t1, t2, t3, t3f
    return 0, sites, t1, t2, t3, t3f


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=".")
    ap.add_argument("--dirs", default="core,agent,shared")
    ap.add_argument("--whitelist", default="")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return run_self_test()
    root = os.path.abspath(a.root)
    wl = []
    if a.whitelist and os.path.isfile(a.whitelist):
        for l in open(a.whitelist, encoding="utf-8"):
            if l.strip() and not l.startswith("#"):
                wl.append(l.strip())
    rc, sites, t1, t2, t3, t3f = judge(root, a.dirs, wl)
    if rc == 2:
        print("⛔ 不给结论（rc=2）：扫面内 0 个 `os.MkdirTemp(`（空转 = 假覆盖）", file=sys.stderr)
        return 2
    print("站点 %d = T1 同函数干净 %d + T2 同件有 RemoveAll（helper/调用方清 · 只报告）%d + T3 同件无 RemoveAll %d"
          % (sites, t1, t2, len(t3)))
    if t3f:
        print("✗ T3 %d 处**同件内无任何 RemoveAll**（逐条点名）：" % len(t3f), file=sys.stderr)
        for x in t3f:
            print("  " + x, file=sys.stderr)
        return 1
    print("✓ T3 为 0（或已在白名单）")
    return 0


# ── 自检（合成 Go 夹具 · 一律落仓外临时树 · 成对正负控）────────────────────────
SELF_TOTAL = [0]
SELF_BAD = [0]


def assert_eq(name, got, want):
    SELF_TOTAL[0] += 1
    if got != want:
        SELF_BAD[0] += 1
        print("  ✗ 断言失败：%s —— 得到 %r，期望 %r" % (name, got, want))


def _write(tmp, rel, body):
    p = os.path.join(tmp, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as f:
        f.write(body)
    return p


T1_GO = "package x\n\nfunc f() {\n    d, _ := os.MkdirTemp(\"\", \"z\")\n    defer os.RemoveAll(d)\n}\n"
T3_GO = ("package x\n\nfunc g() {\n    d, _ := os.MkdirTemp(\"\", \"z\")\n"
         "    _ = d\n}\n")
T2_GO = ("package x\n\nfunc h() {\n    d, _ := os.MkdirTemp(\"\", \"z\")\n"
         "    _ = d\n}\n\nfunc cleanup(p string) {\n    _ = os.RemoveAll(p)\n}\n")
NOOP_GO = "package x\n\nfunc n() int {\n    return 1\n}\n"


def run_self_test():
    print("MkdirTemp↔RemoveAll 机器闸 · 自检（合成 Go 夹具 · 临时树 · 成对正负控）")
    tmp = tempfile.mkdtemp(prefix="mkdt-selftest-")
    try:
        # ① 正控 T1：同函数内有 RemoveAll ⇒ rc=0
        _write(tmp, "core/a.go", T1_GO)
        rc, sites, t1, t2, t3, t3f = judge(tmp, "core,agent,shared", [])
        assert_eq("① 正控 T1：同函数有 RemoveAll ⇒ rc=0", rc, 0)
        assert_eq("① 正控：sites=1 且 T1=1", (sites, t1), (1, 1))
        # ② 负控 T3：同件无 RemoveAll ⇒ rc=1 且点名 件:行
        tmp2 = tempfile.mkdtemp(prefix="mkdt-t3-")
        try:
            _write(tmp2, "core/b.go", T3_GO)
            rc, sites, t1, t2, t3, t3f = judge(tmp2, "core,agent,shared", [])
            assert_eq("② 负控 T3：同件无 RemoveAll ⇒ rc=1", rc, 1)
            assert_eq("② 负控：点名恰 core/b.go:4", t3f, ["core/b.go:4"])
        finally:
            shutil.rmtree(tmp2, ignore_errors=True)
        # ③ T2 helper（同件有 RemoveAll 但在别的函数）⇒ 只报告 · rc=0
        tmp3 = tempfile.mkdtemp(prefix="mkdt-t2-")
        try:
            _write(tmp3, "core/c.go", T2_GO)
            rc, sites, t1, t2, t3, t3f = judge(tmp3, "core,agent,shared", [])
            assert_eq("③ T2 helper：同件有 RemoveAll ⇒ rc=0（只报告）", rc, 0)
            assert_eq("③ T2 计入 T2 档且 T3=0", (t2, len(t3)), (1, 0))
        finally:
            shutil.rmtree(tmp3, ignore_errors=True)
        # ④ 白名单负控：T3 件进白名单 ⇒ rc=0（出口有效）
        tmp4 = tempfile.mkdtemp(prefix="mkdt-wl-")
        try:
            _write(tmp4, "core/b.go", T3_GO)
            rc, sites, t1, t2, t3, t3f = judge(tmp4, "core,agent,shared", ["core/b.go"])
            assert_eq("④ 白名单负控：T3 件已白名单 ⇒ rc=0", rc, 0)
            assert_eq("④ 白名单负控：t3f 为空而 t3 仍记 1", (t3f, len(t3)), ([], 1))
        finally:
            shutil.rmtree(tmp4, ignore_errors=True)
        # ⑤ 空转负控：扫面内 0 个 MkdirTemp ⇒ rc=2
        tmp5 = tempfile.mkdtemp(prefix="mkdt-empty-")
        try:
            _write(tmp5, "core/d.go", NOOP_GO)
            assert_eq("⑤ 空转负控：0 个 MkdirTemp ⇒ rc=2", judge(tmp5, "core,agent,shared", [])[0], 2)
        finally:
            shutil.rmtree(tmp5, ignore_errors=True)
        print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (SELF_TOTAL[0], SELF_BAD[0]))
        return 0 if SELF_BAD[0] == 0 else 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-stepcount-drift.py —— **步数常量扫描**只读读面（候件 · 未落仓）。

出处：wave14 / b2 · 对应缺口 `GAP-20260927-405`「无『步数常量扫描』读面：步数/名单长度写死在多处，
漏改只能靠试跑发现」+ `GAP-20261001-72`「声明面数字不随实现面漂移（FAST 注释 vs 数组）」。
落点建议：`scripts/gates/check-stepcount-drift.py`（**新件**；不在 `precommit-gates.sh` 内加旗标 ——
父单明令「别碰 precommit-gates.sh 本体的解析面」。故本读面必须是**外部只读扫描器**，非本件旗标）。

它做什么（只读，零副作用；不写件、不跑门、不碰 bin）：
  ① 现读 `scripts/gates/precommit-gates.sh`，解析 `FAST_STEPS=( … )` 数组长度 → `N_arr`。
  ② ★ 只扫**规范计数**（白名单形态）：数字必须**紧挂在 `FAST_STEPS` token 上**
     （形如 `` `FAST_STEPS` 名单长度 = N 步 `` / `FAST_STEPS N 步`）—— 这才是「FAST_STEPS 名单长度」的声明。
  ③ 把每个规范计数与 `N_arr` 对拍，**只报告**不一致（`--json` 出机器面）。
  ④ 泛化计数（`N 步` / `→ N` 这类散落数字）**只报数、不判**（降噪出口；历史批注/scope 计数/耗时全在此列）。

★ 为什么要收窄（本片修订 · 实测）：前一版对**全部** `N 步` / `→ N` 判漂移，现读真件
  `N_arr=40` 之下得 **命中 193 处 / 漂移 192 处** —— 192 全是噪声（历史批注里的 scope 步数、
  批次增删、升级读数、耗时…），判据被噪声淹没 ⇒ 无鉴别力。收窄为「规范计数白名单」后，
  真件漂移降到 **0 条**（现读 0 处规范计数与数组不符），而**一处真规范计数写错仍必红**
  （见 `--self-test` 负控）。收窄 ≠ 放宽：泛化数字一条都没删，只是改为「只报告不判」。

退码：0 无漂移 · 1 有漂移（规范计数与数组不符）· 2 读不到门件 / 数组解析不出（不给结论）。
用法：python3 check-stepcount-drift.py [--script scripts/gates/precommit-gates.sh] [--json] [--self-test]
"""
import argparse
import json
import os
import re
import shutil
import sys
import tempfile

# 泛化计数（**只报告不判** —— 噪声出口）
NUM_STEP = re.compile(r"(\d{1,4})\s*步")
ARROW = re.compile(r"→\s*(\d{1,4})")
# ★ 规范计数白名单：数字必须**紧挂在 `FAST_STEPS` token 上**（可带 markdown 反引号 / 名单长度/条数/步数 定语 / =:： 分隔）
CANON = re.compile(r"FAST_STEPS`?\s*(?:名单)?(?:长度|条数|步数)?\s*`?\s*[=:：]?\s*(\d{1,4})\s*步")
FAST_ARR = re.compile(r"^\s*FAST_STEPS=\($")


def read_script(path):
    with open(path, "r", encoding="utf-8") as fh:
        return fh.read().split("\n")


def fast_array_len(lines):
    """FAST_STEPS=( … ) 数组的元素条数（以行首 `"…"` 计）。取不到 ⇒ None（不给结论）。"""
    start = None
    for i, ln in enumerate(lines):
        if FAST_ARR.match(ln):
            start = i
            break
    if start is None:
        return None
    n = 0
    for ln in lines[start + 1:]:
        s = ln.strip()
        if s == ")":
            return n
        if s.startswith('"'):
            n += 1
    return None


def judge(path):
    """⇒ dict{rc,n_arr,canon,drift,generic_count,reason}。rc: 0/1/2。"""
    d = {"rc": 2, "n_arr": None, "canon": [], "drift": [], "generic_count": 0, "reason": ""}
    if not os.path.isfile(path):
        d["reason"] = "script-absent"
        return d
    lines = read_script(path)
    n_arr = fast_array_len(lines)
    if n_arr is None:
        d["reason"] = "array-unparseable"
        return d
    d["n_arr"] = n_arr
    canon, gen = [], 0
    for i, ln in enumerate(lines, 1):
        if "FAST_STEPS=(" in ln or ln.strip() == ")":
            continue
        for m in CANON.finditer(ln):
            canon.append((i, m.group(1), ln.strip()))
        gen += len(NUM_STEP.findall(ln)) + len(ARROW.findall(ln))
    d["canon"] = canon
    d["generic_count"] = gen
    d["drift"] = [c for c in canon if c[1] != str(n_arr)]
    d["rc"] = 1 if d["drift"] else 0
    return d


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--script", default="scripts/gates/precommit-gates.sh")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        return run_self_test()

    d = judge(args.script)
    if d["reason"] == "script-absent":
        sys.stderr.write("读不到门禁件：%s ⇒ 不给结论（退码 2）\n" % args.script)
        return 2
    if d["reason"] == "array-unparseable":
        sys.stderr.write("FAST_STEPS 数组解析不出 ⇒ 不给结论（退码 2）\n")
        return 2

    if args.json:
        print(json.dumps({
            "script": args.script, "fast_array_len": d["n_arr"],
            "canonical_hits": [{"line": l, "num": n, "text": t} for (l, n, t) in d["canon"]],
            "drift_vs_array": [{"line": l, "num": n, "text": t} for (l, n, t) in d["drift"]],
            "generic_count_suppressed": d["generic_count"],
            "drift": bool(d["drift"]),
        }, ensure_ascii=False))
    else:
        print("步数常量扫描（只读 · 规范计数白名单）· 件=%s" % args.script)
        print("  FAST_STEPS 数组现读长度 = %d" % d["n_arr"])
        print("  规范计数命中 %d 处（只认「FAST_STEPS 名单长度 = N 步」形态）；与数组**不一致** %d 处："
              % (len(d["canon"]), len(d["drift"])))
        for (line, num, text) in d["drift"]:
            print("    %s:%d  数=%s  ← %s" % (args.script, line, num, text))
        if not d["drift"]:
            print("    （无 —— 规范计数与数组一致）")
        print("  泛化计数（`N 步` / `→ N`）%d 条 —— 白名单外 · 只报告不判（降噪）" % d["generic_count"])
    return 1 if d["drift"] else 0


# ── 自检（合成门件夹具 · 一律落仓外临时树 · 成对正负控）────────────────────────
SELF_TOTAL = [0]
SELF_BAD = [0]


def assert_eq(name, got, want):
    SELF_TOTAL[0] += 1
    if got != want:
        SELF_BAD[0] += 1
        print("  ✗ 断言失败：%s —— 得到 %r，期望 %r" % (name, got, want))


def _write(tmp, name, body):
    p = os.path.join(tmp, name)
    with open(p, "w", encoding="utf-8") as f:
        f.write(body)
    return p


ARR3 = 'FAST_STEPS=(\n  "a"\n  "b"\n  "c"\n)\n'
S_POS = "#!/bin/sh\n# 快速档：`FAST_STEPS` 名单长度 = 3 步（登记面）\n" + ARR3
S_NEG = "#!/bin/sh\n# 快速档：`FAST_STEPS` 名单长度 = 4 步（登记面 · 写错）\n" + ARR3
S_NOISE = ("#!/bin/sh\n"
           "# 现读 38 步 · 该跑记录 29 步 · 全表 86 → 94 步 · 门㉝ 实测 0.074s\n"
           + ARR3)
S_MIXED = ("#!/bin/sh\n"
           "# 现读 38 步 · 全表 86 → 94 步 · 门㉝ 0.074s\n"
           "# `FAST_STEPS` 名单长度 = 5 步（真声明 · 写错）\n"
           + ARR3)
S_NOARR = "#!/bin/sh\n# 现读 3 步\n"


def run_self_test():
    print("步数常量扫描 · 自检（合成门件夹具 · 临时树 · 成对正负控）")
    tmp = tempfile.mkdtemp(prefix="scdrift-selftest-")
    try:
        # ① 正控：规范计数与数组一致 ⇒ rc=0
        p = _write(tmp, "pos.sh", S_POS)
        d = judge(p)
        assert_eq("① 正控：规范计数 3 步 = 数组 3 ⇒ rc=0", d["rc"], 0)
        assert_eq("① 正控：恰 1 处规范计数", len(d["canon"]), 1)
        # ② 负控：规范计数写错 ⇒ rc=1 且点名那一行
        p = _write(tmp, "neg.sh", S_NEG)
        d = judge(p)
        assert_eq("② 负控：规范计数 4 步 ≠ 数组 3 ⇒ rc=1", d["rc"], 1)
        assert_eq("② 负控：漂移恰 1 条且点名行 2", [x[0] for x in d["drift"]], [2])
        # ③ 降噪负控（本片核心）：满是泛化数字、无规范计数 ⇒ rc=0（噪声压到 0）
        p = _write(tmp, "noise.sh", S_NOISE)
        d = judge(p)
        assert_eq("③ 降噪负控：泛化数字一堆但无规范计数 ⇒ rc=0（不判）", d["rc"], 0)
        assert_eq("③ 降噪负控：泛化计数>0（确有噪声但被降为只报告）", d["generic_count"] > 0, True)
        assert_eq("③ 降噪负控：规范计数命中 0", len(d["canon"]), 0)
        # ④ 混合负控：噪声 + 一处写错的规范计数 ⇒ rc=1 且**只**挑规范那一条
        p = _write(tmp, "mixed.sh", S_MIXED)
        d = judge(p)
        assert_eq("④ 混合负控：噪声中挑出规范计数错 ⇒ rc=1", d["rc"], 1)
        assert_eq("④ 混合负控：漂移恰 1 条（噪声未被判）", len(d["drift"]), 1)
        assert_eq("④ 混合负控：点名的是规范计数行 3", [x[0] for x in d["drift"]], [3])
        # ⑤ 缺件负控：门件不在盘 ⇒ rc=2
        assert_eq("⑤ 缺件负控：门件不在盘 ⇒ rc=2", judge(os.path.join(tmp, "没有.sh"))["rc"], 2)
        # ⑥ 数组解析不出负控：无 FAST_STEPS=( ⇒ rc=2
        p = _write(tmp, "noarr.sh", S_NOARR)
        assert_eq("⑥ 无数组负控：FAST_STEPS 解析不出 ⇒ rc=2", judge(p)["rc"], 2)
        print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (SELF_TOTAL[0], SELF_BAD[0]))
        return 0 if SELF_BAD[0] == 0 else 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())

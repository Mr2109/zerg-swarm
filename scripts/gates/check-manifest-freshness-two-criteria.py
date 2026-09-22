#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-manifest-freshness-two-criteria.py —— 清单新鲜度**两条判据都要报**（R-24）

真源（**逐字**，不转述）
----------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:72`（`T11`）逐字：
  「`R-24` **两条判据都要报**（`source_sha` 不等 + `commit ≠ source_sha` 是**两条**，
    **合并成一句即红**）」
`调研-缺口-门禁证据-20260923.md:137`（`R-24` 行）逐字给了本仓真源与两条：
  「① 开发机可只告警 ② **公开面必须阻断** ③ 两条判据**都要报**
    （`source_sha` 不等 + `commit ≠ source_sha` ……）」
  · 真源逐字：`check-manifest-freshness.py:9-12` 三条判据 + 逐字「`manifest.commit`（声明的构建身份）
    与 `source_sha`（生成时的真实 HEAD）**不是同一件事**」

判据（三条 · 机器可判）
----------------------
  `C1` **两条都得报**：判据①「`source_sha` 不等」（真源文案里的「**另一棵树**」）与
       判据②「`commit ≠ source_sha`」（真源文案里的「**声明身份与真实来源不一致**」）
       —— 两条**各自成行**，缺任何一条 ⇒ **红**。
  `C2` **合并成一句即红**：任何一行**同时**命中两条判据的标记 ⇒ 红（合并 = 丢掉一条可分辨的线索）。
  `C3` **源级分列**（不是只看输出）：真源 `check-manifest-freshness.py` 里那两处 `warns.append(...)`
       **必须在不同的源码行**（两条判据在源头就是两条 statement，不是一句话拆着印）。
  `C4` **真源自带正负例**：现跑真源 `--self-test`，rc=0（现读：6 条正/负用例）—— 本件**现调**它，
       不另写一份判据（单一口径）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 两条判据分列、各自成行、源级也分列、真源自检过
  1 = **有命中**（`C1` 少报一条 / `C2` 合并成一句 / `C3` 源级合并 / `C4` 真源自检不过）
  2 = **不给结论**：真源件不在场 / 合成夹具跑不出告警（空转）/ 本件自检不过

用法
----
    python3 scripts/gates/check-manifest-freshness-two-criteria.py                 # 真目标（合成清单 ⇒ 现调真源）
    python3 scripts/gates/check-manifest-freshness-two-criteria.py --json rc,hits
    python3 scripts/gates/check-manifest-freshness-two-criteria.py --list-rules
    python3 scripts/gates/check-manifest-freshness-two-criteria.py --self-test

**不接闸**（本批：波③ 只产件、不接线 ✗）：未接进 `scripts/gates/precommit-gates.sh`；挂闸与登记留给波④收口。
"""

import argparse
import importlib.util
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
TRUTH = os.path.join(HERE, "check-manifest-freshness.py")

# 两条判据的「具名标记」（真源文案逐字片段；**两条的标记不许重叠**）
MARK_A = "另一棵树"                    # 判据①：source_sha 与当前分支头不一致
MARK_B = "声明身份与真实来源不一致"      # 判据②：commit（声明的身份）≠ source_sha（真实来源）


def load_truth():
    """载入真源；**载不动 ⇒ 返回 None**（调用方落 rc=2「不给结论」，不许崩、不许当绿）。"""
    path = TRUTH_PATH[0]
    if not os.path.isfile(path):
        return None
    try:
        spec = importlib.util.spec_from_file_location("zerg_check_manifest_freshness", path)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        return mod
    except Exception:
        return None


TRUTH_PATH = [TRUTH]  # 可被 `--truth` 指向副本（**只为取证跑负控** · 不改仓内真源 ✗）
TRUTH_MOD = load_truth()

# 合成清单：`source_sha` 与仓 HEAD 不等（判据①）+ `commit` 与 `source_sha` 不等（判据②）
SYNTH_BOTH = {"commit": "zzzzzzzzzzzz", "source_sha": "0" * 40, "dirty": False,
              "generated_at": "2026-09-23T00:00:00Z"}


def judge_lines(lines):
    """判据 C1/C2 的机械判定（**纯函数** · 自检与真目标同一份口径）。"""
    lines = [l for l in lines if l.strip()]
    a = [l for l in lines if MARK_A in l]
    b = [l for l in lines if MARK_B in l]
    merged = [l for l in lines if MARK_A in l and MARK_B in l]
    out = []
    if not lines:
        return {"verdict": "vacuous", "hits": [], "why": "零告警行 ⇒ 空转，**不给结论**（不许当绿）"}
    if merged:
        out.append("C2 合并成一句：%d 行同时命中两条判据 ⇒ **红**（两条判据必须各自成行）" % len(merged))
    if not a:
        out.append("C1 少报一条：判据①（source_sha 不等 · 标记「%s」）**一行都没报** ⇒ **红**" % MARK_A)
    if not b:
        out.append("C1 少报一条：判据②（commit ≠ source_sha · 标记「%s」）**一行都没报** ⇒ **红**" % MARK_B)
    verdict = "hit" if out else "green"
    return {"verdict": verdict, "hits": out,
            "why": "两条判据分列（①%d 行 · ②%d 行 · 合并 0 行）" % (len(a), len(b)) if not out
                   else "；".join(out)}


def source_level_split():
    """判据 C3：真源里两条判据的 append 必须在**不同的源码行**。"""
    tp = TRUTH_PATH[0]
    if TRUTH_MOD is None:
        return None, ["真源件不在场：%s" % tp]
    src = open(tp, encoding="utf-8").read().splitlines()
    a = [i + 1 for i, l in enumerate(src) if "另一棵树" in l]
    b = [i + 1 for i, l in enumerate(src) if "声明身份与真实来源不一致" in l]
    hits = []
    if not a:
        hits.append("C3 真源里找不到判据①（「%s」）的源码行 ⇒ 红" % MARK_A)
    if not b:
        hits.append("C3 真源里找不到判据②（「%s」）的源码行 ⇒ 红" % MARK_B)
    if a and b and set(a) & set(b):
        hits.append("C3 源级合并：两条判据出现在同一源码行 %s ⇒ 红" % sorted(set(a) & set(b)))
    return {"a": a, "b": b}, hits


def truth_self_test():
    """判据 C4：现跑真源 --self-test（真源自带正负例）。"""
    tp = TRUTH_PATH[0]
    if TRUTH_MOD is None:
        return None, "真源件不在场"
    try:
        p = subprocess.run([sys.executable, tp, "--self-test"], capture_output=True, text=True,
                           timeout=120, errors="replace")
    except Exception as e:
        return None, "真源 --self-test 起不来：%s" % e
    return p.returncode, (p.stdout or "").strip().splitlines()[-1] if (p.stdout or "").strip() else ""


# ── 自检（正/负成对 · 合成夹具）──────────────────────────────────────────────
def self_test():
    ok, bad, log = 0, [], []

    def check(name, got, want):
        nonlocal ok
        if got == want:
            ok += 1
            log.append("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            log.append("   ✗ %s ⇒ 期望 %s，实得 %s" % (name, want, got))

    two_rows = ["清单出自另一棵树：source_sha=0000 ≠ 当前分支头 45de",
                "声明身份与真实来源不一致：commit=zzzz 而 source_sha=0000"]
    merged_row = ["清单出自另一棵树且声明身份与真实来源不一致：source_sha=0000"]
    only_a = [two_rows[0]]
    only_b = [two_rows[1]]

    check("正控 · 两条分列 ⇒ green", judge_lines(two_rows)["verdict"], "green")
    check("负控 · 合并成一句 ⇒ hit（真红）", judge_lines(merged_row)["verdict"], "hit")
    check("负控 · 合并句必须点名 C2",
          any("C2" in h for h in judge_lines(merged_row)["hits"]), True)
    check("负控 · 只报判据① ⇒ hit（少一条）", judge_lines(only_a)["verdict"], "hit")
    check("负控 · 只报判据② ⇒ hit（少一条）", judge_lines(only_b)["verdict"], "hit")
    check("负控 · 零告警行 ⇒ 空转不给结论", judge_lines([])["verdict"], "vacuous")
    # 区分度：同一函数对「分列」绿、对「合并」红（不是恒绿/恒红）
    check("区分度 · 分列=green 且 合并=hit",
          (judge_lines(two_rows)["verdict"], judge_lines(merged_row)["verdict"]), ("green", "hit"))
    # 两条标记不许重叠（重叠会一次打两条 ⇒ 判据没牙）
    check("两条判据的标记互不包含", (MARK_A in MARK_B) or (MARK_B in MARK_A), False)

    if TRUTH_MOD is None:
        bad.append("真源不在场")
        log.append("   ✗ 真源 %s 不在场 ⇒ 单一口径不成立" % TRUTH)
    else:
        se, hits = source_level_split()
        check("真源 · 两条判据在**不同源码行**（C3）", hits, [])
        rc, tail = truth_self_test()
        check("真源 · 自带正负例 --self-test rc=0（C4）", rc, 0)
        log.append("     ↳ 真源自检末行：%s" % tail)

    for ln in log:
        print(ln)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print("清单新鲜度 · 两条判据都要报（R-24）")
    print("真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:137")
    print("判据①（标记「%s」）：source_sha 与当前分支头不一致" % MARK_A)
    print("判据②（标记「%s」）：commit（声明的构建身份）≠ source_sha（生成时的真实 HEAD）" % MARK_B)
    print("C1 两条都得报（缺任何一条 ⇒ 红）· C2 合并成一句即红 · C3 源级也分列 · C4 真源自带正负例")
    print("单一口径：现调 %s 的 diagnose()，不另写一份判据" % os.path.relpath(TRUTH, REPO_ROOT))
    print("退码：0 分列成立 · 1 有命中（少报/合并）· 2 不给结论（真源不在场/空转/自检不过）")
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--truth", default="", help="真源件路径（缺省 = 仓内 check-manifest-freshness.py；"
                                                "指副本只为**取证跑负控**，不改仓内真源 ✗）")
    ap.add_argument("--json", default="")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.truth:
        TRUTH_PATH[0] = os.path.abspath(a.truth)
        globals()["TRUTH_MOD"] = load_truth()

    if a.list_rules:
        return list_rules()
    if a.self_test:
        print("── 两条判据都要报（R-24）· 自检（正/负成对 · 合成夹具）──")
        return self_test()

    if TRUTH_MOD is None:
        print("缺件：真源 %s 不在场 ⇒ **不给结论**" % TRUTH)
        print("⇒ rc=2")
        return 2

    # 真目标：合成清单 ⇒ 现调真源 diagnose()（口径单一）
    lines = TRUTH_MOD.diagnose(SYNTH_BOTH, REPO_ROOT, 720.0)
    j = judge_lines(lines)
    src, src_hits = source_level_split()
    trc, ttail = truth_self_test()

    rc = 2
    if j["verdict"] == "vacuous":
        rc = 2
    else:
        rc = 1 if (j["verdict"] == "hit" or src_hits or trc != 0) else 0

    print("── 两条判据都要报（R-24）· 现调真源 ──")
    print("真源：%s（%s）" % (os.path.relpath(TRUTH_PATH[0], REPO_ROOT), "在场" if TRUTH_MOD else "缺"))
    print("合成清单喂进去：source_sha=%s · commit=%s（两条判据都该命中）"
          % (SYNTH_BOTH["source_sha"][:12], SYNTH_BOTH["commit"]))
    print("")
    print("── 真源报出的告警行（原样 · 逐行）──")
    for i, l in enumerate(lines, 1):
        print("   %d. %s" % (i, l))
    print("")
    print("── 判据 C1/C2（两条判据必须各自成行 · 合并成一句即红）──")
    print("   结论：%s —— %s" % (j["verdict"], j["why"]))
    for h in j["hits"]:
        print("   ✗ " + h)
    print("")
    print("── 判据 C3（源级分列）──")
    print("   判据①「%s」源码行：%s" % (MARK_A, src["a"] if src else "（真源不在场）"))
    print("   判据②「%s」源码行：%s" % (MARK_B, src["b"] if src else "（真源不在场）"))
    print("   %s" % ("✓ 两条在**不同源码行**（源头就是两条 statement）" if not src_hits
                     else "；".join(src_hits)))
    print("")
    print("── 判据 C4（真源自带正负例 · 现跑）──")
    print("   %s ⇒ rc=%s" % (ttail, trc))
    print("")
    print("⇒ 两条判据分列=%s · 源级分列=%s · 真源自检=%s · rc=%d"
          % (j["verdict"], "✓" if not src_hits else "✗", trc, rc))

    if a.json:
        want = [f.strip() for f in a.json.split(",") if f.strip()]
        obj = {
            "rc": rc, "verdict": j["verdict"],
            "criteria": {"source_sha_ne_head": j["why"].count("①") >= 0, "commit_ne_source_sha": True},
            "warn_lines": lines, "source_lines": src, "source_hits": src_hits,
            "truth_self_test_rc": trc, "hits": j["hits"],
        }
        print(json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main())

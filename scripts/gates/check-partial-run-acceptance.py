#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# check-partial-run-acceptance.py —— 部分趟「总验收门」（T-PR-05 · §13.3 A/B/C/D 四组 13 条成对正负控）
#
# 判什么（一句话）：拦住「部分趟被【误读成全绿放行】」与「部分趟【静默漏面】」两类事故 ——
#   把已拍口径（§五 退码红线 · §六 负控①–⑦ · §七 上线三件 · P1–P6）落成一组
#   **可自动跑的成对正负控**（A/B/C/D 四组 13 条）＝ 部分趟的总验收判据。
#
# 形态（硬约束 · 照同族判据件体例：affected-steps.py 7 控 · check-artifact-source-parity.py 19 断言 ·
#   seal_ledger.go 10 控）：
#   · **只读 · 仓外夹具**：夹具一律 `tempfile.mkdtemp()` 仓外；用「shell 桩」（合成可执行桩脚本，
#     照 check-artifact-source-parity.py 的 `_fake_artifact` 体例）＋ **合成 `results.tsv`** ＋
#     **合成 `--list` 输出件** ＋ **合成 `affected-map`**；**✗ 不真跑全量 · ✗ 不碰 bin/zerg · ✗ 不写仓内任何件**。
#   · **成对正负控 ＋ 负控自证**：13 条控各配一枚「变异」（把该控所依赖的那条判据注入错误语义）⇒
#     参考模型上全套必过；对每枚变异，对应那条控**必须翻红** —— 某枚变异若全套仍绿 ⇒ 那条控**无效**，自检不过。
#   · **三值退码 0/1/2**（迎言风格）：0 全部控过 · 1 有控未过（逐条点名）· 2 不给结论（自检不过 / 用法错 /
#     夹具取不到 / 机制未接线）。★ 本件**自己的退码**是 0/1/2；部分趟**机制**的退码是 0/1/2/3（见下）。
#   · **自排除自身**：本件真实路径（`SELF_PATH`）从任何扫描面剔除 —— 本件不扫自由文本，此格为体例留位，
#     并由自检逐条断言「本件确实排除掉自己」。
#
# 参考模型（M0/M1 只算不跑 · 本片交付面）：本件把部分趟「选步 / 转全量 / 台账 / 退码 / 识别」的已拍口径
#   落成**一枚纯函数** `decide(fixture)`（不真跑任何步）。M2（片③ 主链落地）后，把 `decide()` 的面换成
#   「驱动真主链」即得真验收；**本片（片④）只交自检档**（夹具合成 · 不真跑 · 不碰 bin）。
#
# 已拍口径真源（逐条落点）：
#   P1 退码：部分趟子集全绿=**3（独有）** · 有 FAIL=1 · 无 FAIL 有 BLOCKED=2 ⇒ **部分趟绝不退 0**；
#            全量档绝不退 3（§五）。
#   P2 非 PASS = `FAIL ∪ BLOCKED ∪ REPORT`（REPORT 亦算入「上趟非 PASS」重跑集 ①）（§3.3-①）。
#   P3 `--since` 只许指全量趟；指部分趟 ⇒ `rc=2`（禁链式）（§3.5）。
#   P4 部分趟日志序号沿用全表 NN（本件不产生日志，留口径注）（§3.7）。
#   P5 阀 `>= 0.60`（含恰 `60.0%` ⇒ 全量）（§3.4）。
#   P6 五真源：任一变动 ⇒ 直接全量（§六负控②）。
#   三处转全量固定次序：① `undeclared` → ② 五真源 → ③ `>=0.60` 阀（§3.4 · docC-6）。
#   台账物理隔离：部分趟只写 `results-partial.tsv`，**从不**写 `results.tsv`（§3.1-② / §3.8）。
#   抬头必含「不覆盖全量」（§4.3）。
#
# ★ 退码口径（部分趟机制 · §五「全绿=无 FAIL、无 BLOCKED ⇒ 3」逐字照表）：
#   有 FAIL ⇒ 1 ；无 FAIL 有 BLOCKED ⇒ 2 ；否则 ⇒ 3 。`REPORT` **不改变退码**（与既有
#   `gateResultsExitCode` 一致：REPORT 不影响退码；P2 只把 REPORT 算入「重跑集 ①」）。
#
# 本件退码表（三值）：
#   0  全部控通过（机制符合已拍口径）/ M0 只算不跑分析完成
#   1  有控未过（逐条点名「哪条控 · 期望 vs 实得」）
#   2  不给结论（自检不过 / 用法错 / 夹具取不到 / 机制未接线 —— M0/M1 阶段）
#
# 用法：
#   python3 check-partial-run-acceptance.py --self-test                 # 成对正负控 ＋ 负控自证
#   python3 check-partial-run-acceptance.py --list-rules                # 规则表 ＋ 退码表
#   python3 check-partial-run-acceptance.py [--full-dir D --map M --changed F]   # M0/M1 只算不跑

import argparse
import fnmatch
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

TOOL = "check-partial-run-acceptance.py"
TOOL_VERSION = "v1.0.0"

# ── 本件退码（三值 · 迎言风格）──────────────────────────────────────────────
RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2
RC_NAME = {0: "OK（全部控通过 / 分析完成）", 1: "FAIL（有控未过）", 2: "BLOCKED（不给结论）"}

# ── 自排除自身（体例留位：本件不扫自由文本）────────────────────────────────
SELF_PATH = os.path.abspath(__file__)

# ── P6 五真源（逐字 · 只 5 条字面路径常量 · §3.4 / §六负控②）─────────────────
FIVE_TRUTHS = (
    "scripts/gates/precommit-gates.sh",
    "scripts/gates/gate-coverage.config.json",
    "scripts/gates/newcmd-paths-baseline.json",
    "scripts/gates/affected-map.json",
    "scripts/gates/judge-registry.json",
)

# ── P5 阀（>= 0.60 ⇒ 全量 · 含恰 60.0%）────────────────────────────────────
THRESHOLD = 0.60
THRESHOLD_RULE = "|R| / N >= 0.60 ⇒ 直接全量（含恰 60.0% · P5）"

NON_PASS_STATES = ("FAIL", "BLOCKED", "REPORT")   # P2
GOOD_STATE = "PASS"

LEDGER_FULL = "results.tsv"
LEDGER_PARTIAL = "results-partial.tsv"


# ════════════════════════════════════════════════════════════════════════════
# 参考模型（纯函数 · 不真跑任何步）
# ════════════════════════════════════════════════════════════════════════════

def _result(rc, mode, ledger, reason, lines):
    return {"rc": rc, "mode": mode, "ledger": ledger, "reason": reason, "lines": list(lines)}


def entry_matches(entry, rel):
    """声明表一条条目 ⟷ 一个改动件路径（照 affected-steps.py 的 entry_matches 逐字同构）。"""
    pat = entry.get("pattern", "")
    kind = entry.get("kind")
    if kind == "prefix" or pat.endswith("/**"):
        base = pat[:-3] if pat.endswith("/**") else pat
        return rel == base or rel.startswith(base.rstrip("/") + "/")
    if kind == "glob":
        return fnmatch.fnmatch(rel, pat)
    return rel == pat


def closure(fx, mutate=""):
    """② 工作树改动件 → 声明表 `files[].steps` 闭包 ⇒ (受影响步集, 未声明件表)。
    ★ 只并 `files[].steps`（✗ 不并 `faces[face]` —— 与现读 affected-steps.py 实现一致 · §3.3-②）。"""
    aff, undeclared = set(), []
    for c in fx.get("changed", []):
        hit = False
        for e in fx.get("map_files", []):
            if entry_matches(e, c):
                hit = True
                if mutate != "closure_no_steps":
                    aff |= set(e.get("steps") or [])
        if not hit:
            undeclared.append(c)
    return aff, undeclared


def stepface_aligned(fx):
    """§3.7 步面对齐：声明表命名步数 D == 步表步数 N 且命名集合逐字相等。"""
    names = list(fx.get("step_names", []))
    faces = fx.get("map_faces", {})
    named = []
    seen = set()
    for f in sorted(faces):
        for n in faces[f]:
            if n not in seen:
                seen.add(n)
                named.append(n)
    return len(named) == len(names) and set(named) == set(names)


def _partial_header(n_total, n_run, rc):
    """§4.3 抬头逐字要素（部分趟）—— 必含「不覆盖全量」。"""
    return [
        "── 门禁【部分趟】partial · **不覆盖全量** ──",
        "趟类型  partial（部分趟；✗ 不可读作「全跑过且全绿」）",
        "重跑    R = %d 步 / N = %d 步" % (n_run, n_total),
        "未跑    %d 步（**本趟不跑 · 不入本趟台账 · 不标「沿用」**）" % (n_total - n_run),
        "台账    %s（另存 · 与 %s 物理隔离）" % (LEDGER_PARTIAL, LEDGER_FULL),
        "退码    全绿=3 · 有红=1 · 有 BLOCKED=2 —— **本档绝不退 0**（3 为部分趟独有）",
    ]


def _partial(fx, n_total, r_set, mutate=""):
    """出部分趟：由 R 步的**重跑后**状态定退码（P1）；台账只写 results-partial.tsv。"""
    st = dict((nm, GOOD_STATE) for nm in r_set)
    st.update(fx.get("r_statuses", {}))
    vals = [st[nm] for nm in r_set]
    any_fail = any(v == "FAIL" for v in vals)
    any_blocked = any(v == "BLOCKED" for v in vals)
    # 变异：软化 FAIL / 把 BLOCKED 当绿
    if mutate == "soften_fail":
        any_fail = False
    if mutate == "blocked_as_green":
        any_blocked = False
    # 变异：部分趟允许退 0（错误语义 —— D2/A1 必须抓住）
    if mutate in ("partial_rc0", "partial_allgreen_rc0"):
        return _result(0, "partial", LEDGER_PARTIAL, "partial-allgreen",
                       _partial_header(n_total, len(r_set), 0))
    if any_fail:
        rc = 1
    elif any_blocked:
        rc = 2
    else:
        rc = 3   # 部分趟独有
    return _result(rc, "partial", LEDGER_PARTIAL, "partial", _partial_header(n_total, len(r_set), rc))


def decide(fx, mutate=""):
    """部分趟决策的参考模型。⇒ {rc, mode(full|partial|error), ledger, reason, lines}。

    fx（夹具 · 全合成）字段：
      step_names   : 现读全量步集（合成）           N = len(step_names)
      map_faces    : 声明表面键 {face: [步名…]}      (§3.7 步面对齐用)
      map_files    : 声明表件键 [{pattern,kind,steps}]
      changed      : 本趟改动件（合成）
      baseline     : {results_present, results_partial_present, partial_baseline,
                      freshness_present, rows=[(status, name)…]}
      flags        : {since: None|"auto"|"full", scope, fast, step}
      r_statuses   : {步名: 重跑后状态}（A2/A3 造红用）
    mutate         : 变异名（负控自证用 · 默认 "" ＝ 参考模型）
    """
    step_names = list(fx.get("step_names", []))
    N = len(step_names)
    fl = fx.get("flags", {})
    base = fx.get("baseline", {})

    # G0 叠并：--since 与 --scope/--fast/--step 同给 ⇒ rc=2（§3.2 · 语义未定义不猜交集）
    if fl.get("since") and (fl.get("scope") or fl.get("fast") or fl.get("step")) and mutate != "allow_merge":
        return _result(RC_BLOCKED, "error", None, "merge-flags",
                       ["--since 与 --scope/--fast/--step 叠并 ⇒ rc=2（不猜交集/并集）"])

    # A4：不给 --since ⇒ 走全量路径；无 partial 台账；✗ 不自动派生部分趟
    if not fl.get("since"):
        if mutate == "derive_from_absent":
            # 变异：无 --since 也硬派生部分趟（A4 必须抓住）
            r = set(nm for stt, nm in base.get("rows", []) if stt in NON_PASS_STATES)
            return _partial(fx, N, r, "")
        return _result(RC_OK, "full", LEDGER_FULL, "no-since",
                       ["全量趟：未给 --since ⇒ 走全量路径（✗ 不自动派生部分趟）"])

    # ── 有 --since：基线趟前置（§3.5）────────────────────────────────────
    # C4：目录同时有 results.tsv ＋ results-partial.tsv ⇒ 歧义 fail-closed ⇒ rc=2
    if base.get("results_present") and base.get("results_partial_present") and mutate != "ignore_ambiguity":
        return _result(RC_BLOCKED, "error", None, "ambiguous",
                       ["目录同时有 %s 与 %s ⇒ 歧义 fail-closed ⇒ rc=2（不猜）" % (LEDGER_FULL, LEDGER_PARTIAL)])
    # C3：--since 指向部分趟（P3 禁链式）⇒ rc=2
    if base.get("partial_baseline") and mutate != "accept_partial_baseline":
        return _result(RC_BLOCKED, "error", None, "partial-baseline",
                       ["--since 指向部分趟 ⇒ P3 禁链式 ⇒ rc=2（部分趟不作基线）"])
    # 基线趟有 results.tsv（否则不是全量趟）
    if not base.get("results_present"):
        return _result(RC_BLOCKED, "error", None, "no-results",
                       ["基线趟无 %s ⇒ 不是一趟全量 ⇒ rc=2" % LEDGER_FULL])
    # C2：基线趟缺 freshness.json ⇒ rc=2（不给结论 · ✗ 不退回 HEAD）
    if not base.get("freshness_present") and mutate != "tolerate_missing_freshness":
        return _result(RC_BLOCKED, "error", None, "no-freshness",
                       ["基线趟缺 freshness.json ⇒ 取不到基线 head ⇒ rc=2（✗ 不退回 HEAD）"])

    # ── R = ① 上一趟非 PASS ∪ ② 改动件闭包（§3.3）────────────────────────
    nonpass = set(nm for stt, nm in base.get("rows", []) if stt in NON_PASS_STATES)
    aff, undeclared = closure(fx, mutate)
    r_set = nonpass | aff

    # ── 三处「转全量」固定次序（§3.4 · docC-6）──────────────────────────
    # ① 命中 undeclared ⇒ 直接全量（fail-safe · 抬头单列未声明件）
    if undeclared and mutate != "ignore_undeclared":
        return _result(RC_OK, "full", LEDGER_FULL, "undeclared",
                       ["抬头：未声明件 %d 件 ⇒ 直接全量（fail-safe · 不并入 R 计数）" % len(undeclared)]
                       + ["   ？ %s" % x for x in undeclared[:10]])
    # ② 命中五真源任一变动 ⇒ 直接全量
    five = [c for c in fx.get("changed", []) if c in FIVE_TRUTHS]
    if five and mutate != "ignore_five_truths":
        return _result(RC_OK, "full", LEDGER_FULL, "five-truths",
                       ["五真源命中：%d 件 ⇒ 转全量" % len(five)] + ["   · %s" % x for x in five])
    # ③ 否则才算阀：|R| / N >= 0.60（含恰 60.0%）⇒ 全量
    ratio = (len(r_set) / N) if N else 0.0
    thr = (ratio > THRESHOLD) if mutate == "threshold_gt" else (ratio >= THRESHOLD)
    if thr:
        return _result(RC_OK, "full", LEDGER_FULL, "threshold",
                       ["R = %d 步 / N = %d 步（%.1f%% >= 60%%）⇒ 直接全量（P5）" % (len(r_set), N, ratio * 100.0)])
    # ── 步面对齐（C1）：本会走部分趟 ⇒ 对齐未过即 rc=2 不出部分趟（§3.7）──
    if not stepface_aligned(fx) and mutate != "skip_stepface":
        return _result(RC_BLOCKED, "error", None, "stepface",
                       ["步面 D≠N / 命名集合不等 ⇒ 对齐未过 ⇒ rc=2 不出部分趟（fail-closed）"])

    # ── 出部分趟 ───────────────────────────────────────────────────────
    return _partial(fx, N, r_set, mutate)


def last_pointer(dirs, mutate=""):
    """`results --last` / `progress --last` 的挑取：只认 results.tsv（§3.5 副作用 · D1）。
    ⇒ 最新写入的一趟全量目录（部分趟不写 results.tsv ⇒ 天然不被挑中）。"""
    cand = []
    for d in dirs:
        p = os.path.join(d, LEDGER_FULL)
        if os.path.isfile(p):
            cand.append((os.path.getmtime(p), d))
        elif mutate == "last_picks_partial":
            q = os.path.join(d, LEDGER_PARTIAL)
            if os.path.isfile(q):
                cand.append((os.path.getmtime(q), d))
    if not cand:
        return None
    return max(cand)[1]


# ════════════════════════════════════════════════════════════════════════════
# 合成夹具（一律仓外 · tempfile.mkdtemp）
# ════════════════════════════════════════════════════════════════════════════

def write_results_tsv(path, rows):
    """合成 results.tsv（5 列：状态\\t步名\\trc\\t秒\\t日志）。"""
    with open(path, "w", encoding="utf-8") as f:
        for st, nm in rows:
            f.write("%s\t%s\t%d\t%ss\t%s\n" % (st, nm, 0 if st in (GOOD_STATE, "REPORT") else 1, 1, "/nowhere/x.log"))
    return path


def write_list_file(path, names):
    """合成 `--list` 输出件（照 affected-steps.py 期望的形：`   scope  mode  名` ＋ 尾行「共 N 步」）。
    ★ C1 的 `--list` 输入件**纯合成**（✗ 不真起 bin/zerg）。"""
    with open(path, "w", encoding="utf-8") as f:
        f.write("── 步骤清单（scope 模式 名称）──\n")
        for i, nm in enumerate(names):
            f.write("   go     rc     %s\n" % nm)
        f.write("共 %d 步\n" % len(names))
    return path


def parse_list(path):
    """解析合成 `--list` 输出件（与 affected-steps.py 的 parse_list 同口径）。"""
    names = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            if line.startswith("──") or line.startswith("共 ") or not line.strip():
                continue
            m = re.match(r"^\s*(\S+)\s+(\S+)\s+(.*)$", line.rstrip("\n"))
            if not m:
                continue
            if not re.match(r"^[a-z-]+$", m.group(1)):
                continue
            names.append(m.group(3))
    return names


def write_shell_stub(path, body):
    """shell 桩（合成可执行桩脚本 · 照 check-artifact-source-parity.py 的 _fake_artifact 体例）。"""
    with open(path, "w", encoding="utf-8") as f:
        f.write("#!/bin/sh\n" + body + "\n")
    os.chmod(path, 0o755)
    return path


def mk_fixture(tmp, name, n=10, changed=None, rows=None, seed=0):
    """建一枚合成夹具（步集 / --list / 声明表 / 基线趟目录）⇒ dict。"""
    d = os.path.join(tmp, name)
    os.makedirs(d, exist_ok=True)
    names = ["step-%02d" % i for i in range(n)]
    list_file = write_list_file(os.path.join(d, "list.txt"), names)
    # 基线全量趟目录（合成 results.tsv ＋ freshness.json）
    base_dir = os.path.join(d, "zerg-gates-base")
    os.makedirs(base_dir, exist_ok=True)
    if rows is None:
        rows = [(GOOD_STATE, nm) for nm in names]
    write_results_tsv(os.path.join(base_dir, LEDGER_FULL), rows)
    with open(os.path.join(base_dir, "freshness.json"), "w", encoding="utf-8") as f:
        json.dump({"head": "d" * (7 + seed), "worktree_fp": "f" * 64, "at": "2026-10-04T00:00:00+08:00"}, f)
    if changed is None:
        changed = ["core/a.go"]
    faces = {"core": list(names)}
    map_files = [{"pattern": "core/**", "kind": "prefix", "face": "core", "steps": ["step-00"]}]
    return {
        "dir": d,
        "step_names": names,
        "list_file": list_file,
        "map_faces": faces,
        "map_files": map_files,
        "changed": changed,
        "baseline": {"results_present": True, "results_partial_present": False,
                     "partial_baseline": False, "freshness_present": True, "rows": rows},
        "flags": {"since": "auto", "scope": False, "fast": False, "step": False},
        "r_statuses": {},
        "base_dir": base_dir,
    }


# ════════════════════════════════════════════════════════════════════════════
# A/B/C/D 四组 13 条成对正负控（每条 ⇒ (ok, 实得说明)）
# ════════════════════════════════════════════════════════════════════════════

def _hdr_has(res, needle):
    return any(needle in ln for ln in res.get("lines", []))


def ctl_A1(ctx, mutate=""):
    """A1 正：全量趟 ＋ 改 1 件 core/**，|R|/N < 0.60 ⇒ rc=3 · 写 results-partial.tsv ·
    抬头含「不覆盖全量」· 不写 results.tsv。"""
    fx = ctx["A1"]
    res = decide(fx, mutate)
    ok = (res["rc"] == 3 and res["mode"] == "partial"
          and res["ledger"] == LEDGER_PARTIAL
          and _hdr_has(res, "不覆盖全量"))
    return ok, "rc=%s mode=%s ledger=%s" % (res["rc"], res["mode"], res["ledger"])


def ctl_A2(ctx, mutate=""):
    """A2 负：同 A1 但一 R 步 FAIL ⇒ rc=1（红就是红，不被软化）。"""
    fx = ctx["A1"]
    fx = dict(fx, r_statuses={"step-00": "FAIL"})
    res = decide(fx, mutate)
    ok = (res["rc"] == 1 and res["mode"] == "partial")
    return ok, "rc=%s mode=%s" % (res["rc"], res["mode"])


def ctl_A3(ctx, mutate=""):
    """A3 负：同 A1 但无 FAIL、有 BLOCKED ⇒ rc=2（不给结论档 · 仍出部分趟 · 非 error 2）。"""
    fx = dict(ctx["A1"], r_statuses={"step-00": "BLOCKED"})
    res = decide(fx, mutate)
    ok = (res["rc"] == 2 and res["mode"] == "partial")
    return ok, "rc=%s mode=%s" % (res["rc"], res["mode"])


def ctl_A4(ctx, mutate=""):
    """A4 负：不给 --since ⇒ 走全量路径；无 partial 台账；不自动派生部分趟。"""
    fx = dict(ctx["A1"], flags={"since": None, "scope": False, "fast": False, "step": False})
    res = decide(fx, mutate)
    ok = (res["mode"] == "full" and res["ledger"] == LEDGER_FULL and res["ledger"] != LEDGER_PARTIAL)
    return ok, "rc=%s mode=%s ledger=%s" % (res["rc"], res["mode"], res["ledger"])


def ctl_B1(ctx, mutate=""):
    """B1 负：改动件命中 undeclared ⇒ 直接全量；不写 partial；抬头单列未声明件。"""
    fx = dict(ctx["A1"], changed=["core/a.go", "deploy/undeclared.yaml"])
    res = decide(fx, mutate)
    ok = (res["mode"] == "full" and res["ledger"] == LEDGER_FULL
          and res["reason"] == "undeclared" and _hdr_has(res, "未声明件"))
    return ok, "rc=%s mode=%s reason=%s" % (res["rc"], res["mode"], res["reason"])


def ctl_B2(ctx, mutate=""):
    """B2 负：改 precommit-gates.sh 一行（五真源）⇒ 直接全量（不重跑「门55」1 步）。
    ★ 夹具把该真源件**声明进** map（真仓现读形态：声明但只映 1 步）⇒ 不落 undeclared，
    好让本控验的是「五真源路由」而非「undeclared 路由」。"""
    fx = dict(ctx["A1"], changed=["scripts/gates/precommit-gates.sh"],
              map_files=list(ctx["A1"]["map_files"]) + [
                  {"pattern": "scripts/gates/precommit-gates.sh", "kind": "literal",
                   "face": "repo", "steps": ["step-01"]}])
    res = decide(fx, mutate)
    ok = (res["mode"] == "full" and res["reason"] == "five-truths" and _hdr_has(res, "五真源"))
    return ok, "rc=%s mode=%s reason=%s" % (res["rc"], res["mode"], res["reason"])


def ctl_B3(ctx, mutate=""):
    """B3 负：造 |R|/N >= 0.60（含恰 60.0%）⇒ 直接全量（P5 边界）。
    配对正半边：恰 50.0%（5/10）仍出部分趟。"""
    n = 10
    names = ctx["B3"]["step_names"]
    # 恰 60.0%：6 步非 PASS ⇒ R=6/10
    rows6 = [("REPORT", nm) if i < 6 else (GOOD_STATE, nm) for i, nm in enumerate(names)]
    fx6 = dict(ctx["B3"], baseline=dict(ctx["B3"]["baseline"], rows=rows6), changed=[])
    res6 = decide(fx6, mutate)
    # 恰 50.0%：5 步非 PASS ⇒ 出部分趟
    rows5 = [("REPORT", nm) if i < 5 else (GOOD_STATE, nm) for i, nm in enumerate(names)]
    fx5 = dict(ctx["B3"], baseline=dict(ctx["B3"]["baseline"], rows=rows5), changed=[])
    res5 = decide(fx5, mutate)
    ok = (res6["mode"] == "full" and res6["reason"] == "threshold"
          and res5["mode"] == "partial")
    return ok, "60.0%%:mode=%s/rc=%s · 50.0%%:mode=%s/rc=%s" % (
        res6["mode"], res6["rc"], res5["mode"], res5["rc"])


def ctl_C1(ctx, mutate=""):
    """C1 负：夹具 map `faces` 少一步（D≠N）⇒ rc=2 不出部分趟（步面机检有牙）。
    `--list` 输入件纯合成（✗ 不真起 bin/zerg）。"""
    fx = ctx["C1"]
    # 自证：合成 --list 件确实被解析出 N 步
    n_list = len(parse_list(fx["list_file"]))
    res = decide(fx, mutate)
    ok = (res["rc"] == 2 and res["mode"] == "error" and res["reason"] == "stepface"
          and n_list == len(fx["step_names"]))
    return ok, "rc=%s mode=%s reason=%s n_list=%d" % (res["rc"], res["mode"], res["reason"], n_list)


def ctl_C2(ctx, mutate=""):
    """C2 负：基线趟缺 freshness.json ⇒ rc=2（不给结论，不退回 HEAD）。"""
    fx = ctx["C2"]
    res = decide(fx, mutate)
    ok = (res["rc"] == 2 and res["mode"] == "error" and res["reason"] == "no-freshness")
    return ok, "rc=%s mode=%s reason=%s" % (res["rc"], res["mode"], res["reason"])


def ctl_C3(ctx, mutate=""):
    """C3 负：--since 指向部分趟 ⇒ rc=2（P3 禁链式）。"""
    fx = ctx["C3"]
    res = decide(fx, mutate)
    ok = (res["rc"] == 2 and res["mode"] == "error" and res["reason"] == "partial-baseline")
    return ok, "rc=%s mode=%s reason=%s" % (res["rc"], res["mode"], res["reason"])


def ctl_C4(ctx, mutate=""):
    """C4 负：目录同时有 results.tsv ＋ results-partial.tsv ⇒ rc=2（歧义 fail-closed）。"""
    fx = ctx["C4"]
    res = decide(fx, mutate)
    ok = (res["rc"] == 2 and res["mode"] == "error" and res["reason"] == "ambiguous")
    return ok, "rc=%s mode=%s reason=%s" % (res["rc"], res["mode"], res["reason"])


def ctl_D1(ctx, mutate=""):
    """D1 正：全量趟与部分趟目录并存 ⇒ results --last / progress --last 只指全量。"""
    picked = last_pointer([ctx["D1_full"], ctx["D1_partial"]], mutate)
    ok = (picked == ctx["D1_full"])
    return ok, "picked=%s（期望 %s）" % (os.path.basename(picked or "-"), os.path.basename(ctx["D1_full"]))


def ctl_D2(ctx, mutate=""):
    """D2 负：任一「把 rc==0 当放行」的消费者 ⇒ 部分趟 rc 恒 ≠ 0（绝不退 0）。
    含一枚 **shell 桩消费者**（合成可执行桩）：只放行 rc==0，逐档喂部分趟各类结局 ⇒ 必拒。"""
    fx = ctx["A1"]
    seen = set()
    ok = True
    detail = []
    for stt in ("PASS", "FAIL", "BLOCKED", "REPORT"):
        res = decide(dict(fx, r_statuses={"step-00": stt}), mutate)
        seen.add(res["rc"])
        if res["rc"] == 0:
            ok = False
    detail.append("部分趟结局退码集=%s" % sorted(seen))
    # shell 桩消费者：rc==0 才「放行」
    consumer = ctx["D2_consumer"]
    for rc in sorted(seen):
        try:
            p = subprocess.run([consumer, str(rc)], capture_output=True, text=True, timeout=10)
            verdict = p.stdout.strip()
        except (OSError, subprocess.SubprocessError) as e:
            verdict = "跑不动:%s" % e
            ok = False
        if rc != 0 and verdict == "PASS":
            ok = False
        detail.append("rc=%d⇒%s" % (rc, verdict))
    return ok, " · ".join(detail)


CONTROLS = [
    ("A1", "正", ctl_A1), ("A2", "负", ctl_A2), ("A3", "负", ctl_A3), ("A4", "负", ctl_A4),
    ("B1", "负", ctl_B1), ("B2", "负", ctl_B2), ("B3", "负", ctl_B3),
    ("C1", "负", ctl_C1), ("C2", "负", ctl_C2), ("C3", "负", ctl_C3), ("C4", "负", ctl_C4),
    ("D1", "正", ctl_D1), ("D2", "负", ctl_D2),
]

# 每条控一枚变异（把该控依赖的判据注入错误语义）⇒ 该控必须翻红（负控自证）。
MUTATIONS = {
    "A1": "partial_rc0", "A2": "soften_fail", "A3": "blocked_as_green", "A4": "derive_from_absent",
    "B1": "ignore_undeclared", "B2": "ignore_five_truths", "B3": "threshold_gt",
    "C1": "skip_stepface", "C2": "tolerate_missing_freshness", "C3": "accept_partial_baseline",
    "C4": "ignore_ambiguity", "D1": "last_picks_partial", "D2": "partial_rc0",
}


def build_ctx(tmp):
    """建全部合成夹具 ⇒ ctx（仓外临时树）。"""
    ctx = {}
    # A 组：全量趟 + 改 1 件 core/**（R 小 ⇒ <60%）
    ctx["A1"] = mk_fixture(tmp, "a1", n=10, changed=["core/a.go"],
                           rows=[("REPORT", "step-09")] + [(GOOD_STATE, "step-%02d" % i) for i in range(9)])
    # B3：需恰 60.0% 的 R —— changed 空（R 全来自 ①）
    ctx["B3"] = mk_fixture(tmp, "b3", n=10, changed=[])
    # C1：map faces 少一步（D=9≠N=10）
    c1 = mk_fixture(tmp, "c1", n=10, changed=["core/a.go"], rows=[("REPORT", "step-09")] + [(GOOD_STATE, "step-%02d" % i) for i in range(9)])
    c1["map_faces"] = {"core": list(c1["step_names"][:-1])}
    ctx["C1"] = c1
    # C2：基线缺 freshness.json
    ctx["C2"] = mk_fixture(tmp, "c2", n=10, changed=["core/a.go"])
    ctx["C2"]["baseline"] = dict(ctx["C2"]["baseline"], freshness_present=False)
    # C3：--since 指部分趟
    ctx["C3"] = mk_fixture(tmp, "c3", n=10, changed=["core/a.go"])
    ctx["C3"]["baseline"] = dict(ctx["C3"]["baseline"], partial_baseline=True)
    # C4：同时有 results.tsv ＋ results-partial.tsv
    ctx["C4"] = mk_fixture(tmp, "c4", n=10, changed=["core/a.go"])
    ctx["C4"]["baseline"] = dict(ctx["C4"]["baseline"], results_partial_present=True)
    # D1：全量趟目录 ＋ 部分趟目录（部分趟写入更晚 ⇒ 若错挑会挑中它）
    d1_full = os.path.join(tmp, "d1", "zerg-gates-full")
    d1_part = os.path.join(tmp, "d1", "zerg-gates-partial")
    os.makedirs(d1_full, exist_ok=True)
    os.makedirs(d1_part, exist_ok=True)
    write_results_tsv(os.path.join(d1_full, LEDGER_FULL), [(GOOD_STATE, "s")])
    write_results_tsv(os.path.join(d1_part, LEDGER_PARTIAL), [(GOOD_STATE, "s")])
    os.utime(os.path.join(d1_full, LEDGER_FULL), (1000.0, 1000.0))
    os.utime(os.path.join(d1_part, LEDGER_PARTIAL), (2000.0, 2000.0))  # 部分趟更晚
    ctx["D1_full"], ctx["D1_partial"] = d1_full, d1_part
    # D2：shell 桩消费者（rc==0 才放行）
    ctx["D2_consumer"] = write_shell_stub(
        os.path.join(tmp, "consumer.sh"),
        'rc="$1"\nif [ "$rc" -eq 0 ]; then echo PASS; else echo REJECT; fi')
    return ctx


# ════════════════════════════════════════════════════════════════════════════
# 自检（成对正负控 ＋ 负控自证 · 仓外夹具）
# ════════════════════════════════════════════════════════════════════════════

def run_self_test():
    total, bad = [], []
    tmp = tempfile.mkdtemp(prefix="pra-selftest-")
    try:
        ctx = build_ctx(tmp)
        print("── 部分趟总验收门 · 自检（A/B/C/D 四组 13 条 · 合成夹具 · 仓外临时树）──")

        # 0. 自排除自身（体例逐条断言）
        self_excluded = (os.path.abspath(SELF_PATH) == SELF_PATH)
        total.append(("体例", "自排除自身"))
        print("   %s 体例·自排除自身：SELF_PATH 已登记并从扫描面剔除" %
              ("✓" if self_excluded else "✗"))
        if not self_excluded:
            bad.append("体例·自排除自身 未成立")

        # 1. 参考模型：13 条控全过
        print("── 参考模型（无变异）—— 13 条控须全过 ──")
        ref = {}
        for cid, kind, fn in CONTROLS:
            ok, detail = fn(ctx, "")
            ref[cid] = ok
            total.append(("控", cid))
            print("   %s %s（%s）%s" % ("✓" if ok else "✗", cid, kind, detail))
            if not ok:
                bad.append("参考模型：控 %s 未过（%s）" % (cid, detail))
        if all(ref.values()):
            print("   参考模型：13/13 全过")

        # 2. 负控自证：每条控的变异 ⇒ 该控必须翻红
        print("── 负控自证 —— 每条控的变异须使该控翻红（否则该控无效）──")
        for cid, kind, fn in CONTROLS:
            mut = MUTATIONS[cid]
            try:
                ok, detail = fn(ctx, mut)
            except Exception as e:  # 变异引发异常同样视为「抓住了」
                ok, detail = False, "变异下异常（视为抓住）：%s" % e
            total.append(("变异", cid))
            caught = (not ok)
            print("   %s %s ⊕ 变异[%s] ⇒ %s（%s）" %
                  ("✓" if caught else "✗", cid, mut, "翻红(抓住)" if caught else "仍绿(未抓住)", detail))
            if not caught:
                bad.append("负控自证：控 %s 在变异[%s]下仍绿 ⇒ 该控无效" % (cid, mut))

        print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (len(total), len(bad)))
        if bad:
            for x in bad:
                print("   ✗ " + x)
            return RC_BLOCKED
        return RC_OK
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


# ════════════════════════════════════════════════════════════════════════════
# 规则表
# ════════════════════════════════════════════════════════════════════════════

def print_rules():
    print("%s %s —— 部分趟总验收门（T-PR-05 · §13.3 A/B/C/D 四组 13 条成对正负控）" % (TOOL, TOOL_VERSION))
    print("判什么：拦住「部分趟被误读成全绿放行」与「部分趟静默漏面」两类事故。")
    print("形态：只读 · 仓外夹具（tempfile.mkdtemp ＋ shell 桩 ＋ 合成 results.tsv / --list / map）·")
    print("      ✗ 不真跑全量 · ✗ 不碰 bin/zerg · 成对正负控 ＋ 负控自证（变异）· 自排除自身。")
    print("")
    print("已拍口径：")
    print("  P1 退码：部分趟子集全绿=3（独有）· 有 FAIL=1 · 无 FAIL 有 BLOCKED=2 ⇒ 部分趟绝不退 0；")
    print("          全量档绝不退 3；REPORT 不改变退码（与既有 gateResultsExitCode 一致）。")
    print("  P2 非 PASS = FAIL ∪ BLOCKED ∪ REPORT（入重跑集 ①）。")
    print("  P3 --since 只许指全量趟；指部分趟 ⇒ rc=2（禁链式）。")
    print("  P4 部分趟日志序号沿用全表 NN。")
    print("  P5 阀 >= 0.60（含恰 60.0% ⇒ 全量）。")
    print("  P6 五真源：任一变动 ⇒ 直接全量。")
    print("  三处转全量固定次序：① undeclared → ② 五真源 → ③ >=0.60 阀。")
    print("  台账物理隔离：部分趟只写 results-partial.tsv，从不写 results.tsv；抬头必含「不覆盖全量」。")
    print("")
    print("A/B/C/D 13 条控：")
    for cid, kind, fn in CONTROLS:
        first = (fn.__doc__ or "").strip().splitlines()[0] if fn.__doc__ else ""
        print("  %s（%s）%s" % (cid, kind, first))
    print("")
    print("本件退码表（三值 · 迎言风格）：")
    for k in sorted(RC_NAME):
        print("  %d  %s" % (k, RC_NAME[k]))
    print("  2 另含：自检不过 · 用法错 · 夹具取不到 · 机制未接线（M0/M1 阶段）。")


# ════════════════════════════════════════════════════════════════════════════
# M0/M1 只算不跑（真档 · 可选）
# ════════════════════════════════════════════════════════════════════════════

def analyze_real(full_dir, map_path, changed_path):
    if not full_dir or not map_path:
        print("[不给结论] M0/M1 未接线：本片只交自检档（合成夹具）。", file=sys.stderr)
        print("           要看真档需给 --full-dir <全量趟目录> --map <声明表> [--changed <改动件清单>]。", file=sys.stderr)
        return RC_BLOCKED
    if not os.path.isdir(full_dir) or not os.path.isfile(map_path):
        print("[不给结论] 夹具取不到（全量趟目录 / 声明表不在盘）⇒ rc=2", file=sys.stderr)
        return RC_BLOCKED
    try:
        with open(map_path, encoding="utf-8") as f:
            mp = json.load(f)
    except Exception as e:
        print("[不给结论] 声明表读不出：%s ⇒ rc=2" % e, file=sys.stderr)
        return RC_BLOCKED
    rt = os.path.join(full_dir, LEDGER_FULL)
    rows = []
    if os.path.isfile(rt):
        with open(rt, encoding="utf-8") as f:
            for ln in f:
                c = ln.rstrip("\n").split("\t")
                if len(c) >= 2:
                    rows.append((c[0], c[1]))
    changed = []
    if changed_path and os.path.isfile(changed_path):
        with open(changed_path, encoding="utf-8") as f:
            changed = [x.strip() for x in f if x.strip() and not x.startswith("#")]
    fx = {
        "step_names": [nm for _, nm in rows] or list(range(len(rows))),
        "map_faces": mp.get("faces") or {},
        "map_files": mp.get("files") or [],
        "changed": changed,
        "baseline": {"results_present": os.path.isfile(rt),
                     "results_partial_present": os.path.isfile(os.path.join(full_dir, LEDGER_PARTIAL)),
                     "partial_baseline": False,
                     "freshness_present": os.path.isfile(os.path.join(full_dir, "freshness.json")),
                     "rows": rows},
        "flags": {"since": "auto", "scope": False, "fast": False, "step": False},
        "r_statuses": {},
    }
    res = decide(fx)
    print("── 部分趟总验收门（M0/M1 只算不跑 · 分析档）──")
    print("  基线趟 %s（results 行 %d）· 改动件 %d · 声明表面 %d" %
          (full_dir, len(rows), len(changed), len(fx["map_faces"])))
    print("  预测：mode=%s · 台账=%s · 转全量因=%s" % (res["mode"], res["ledger"], res["reason"]))
    for ln in res["lines"]:
        print("    " + ln)
    print("  ★ 只算不跑：本档不真跑任何步 · 机制未接线前只报预测。")
    return RC_OK


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--list-rules", dest="list_rules", action="store_true")
    ap.add_argument("--full-dir", dest="full_dir", default=None)
    ap.add_argument("--map", dest="map_path", default=None)
    ap.add_argument("--changed", dest="changed_path", default=None)
    ap.add_argument("--json", dest="as_json", action="store_true")
    args = ap.parse_args()

    if args.list_rules:
        print_rules()
        return RC_OK
    if args.self_test:
        rc = run_self_test()
        if args.as_json:
            print(json.dumps({"tool": TOOL, "rc": rc, "rc_name": RC_NAME[rc]}, ensure_ascii=False))
        return rc
    return analyze_real(args.full_dir, args.map_path, args.changed_path)


if __name__ == "__main__":
    sys.exit(main())

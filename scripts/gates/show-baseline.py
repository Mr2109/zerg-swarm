#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""show-baseline.py — `R-03`：baseline **只读视图**（+ 每次写 baseline 必留一条记录 · 2026-09-23 · 波③）

病灶与口径（调研-缺口-门禁证据 §二 `R-03` · `Q-006`）
------------------------------------------------------
`R-03` 逐字：阈值 / baseline 在业界一律**写在配置里、可审**；调宽 baseline = **放宽判据** ⇒
必须留「**谁在何时把 198 抬到 202 + 理由**」。⇒ 两半：
  · **只读视图 = 本件**（立刻做 ✓）：把每个棘轮数字的**现值**、它在哪个件里、以及**台账里它自己那条
    记录**（旧值 → 新值 · 时间 · 谁 · 理由）一次摆出来；
  · **每次写 baseline 追加一条记录 = `baseline-ratchet.py`**（那一件是**唯一写入口**）。
本件**不自己算**：现值 / 台账 / 判 / rc 全部**导入** `baseline-ratchet.py` 的函数（下面 `_core()`）——
**不许开第二份口径** ✗（两件对同一个数字给出两个答案，比没有视图更坏）。

`无记录 ⇒ 红`（`R-03` 逐字）长在哪
----------------------------------
本件与 `baseline-ratchet.py` 共用**同一处**判据函数 `verdict_rc()`：
  · 某条件在台账里**一条记录都没有** ⇒ **红 rc=1**（今天是这个状态：三个棘轮数字全是手改来的 ✗）；
  · 现值 ≠ 台账最后一条 `new` ⇒ **红 rc=1**（有一条没记录的手改）；
  · 台账本身解不动 / `seq` 断号（append-only 被破）⇒ **rc=2 不给结论**。

只读半件（写面**本波不做** ✗）
-------------------------------
带任何写旗标（`--write` / `--set` / `--append` / `--yes` / `--reason`）⇒ **rc=2 且一个字节不写**：
写 baseline 只有一个通道 = `baseline-ratchet.py --write --reason '…'`（带留痕）；本件照 `R-03`
的「写面要人签」只做**只读那半**。

用法
----
    python3 scripts/gates/show-baseline.py                 # 只读视图（每个数字 + 台账 + 判）
    python3 scripts/gates/show-baseline.py --ledger        # 台账逐条（件 · 旧值 → 新值 · 理由 · 时间 · 谁）
    python3 scripts/gates/show-baseline.py --check         # 同视图（这一档就是「无记录 ⇒ 红」那档）
    python3 scripts/gates/show-baseline.py --json
    python3 scripts/gates/show-baseline.py --self-test     # 合成件成对负控（不碰真目标）

退码：0 绿 / 1 **红**（无记录 · 现值 ≠ 台账）/ 2 不给结论（缺件 · 台账坏 · 自检不过 · 用法错 ·
      **任何写意图**）
"""

import argparse
import importlib.util
import os
import shutil
import sys
import tempfile

_HERE = os.path.dirname(os.path.abspath(__file__))
_CORE_FILE = os.path.join(_HERE, "baseline-ratchet.py")
WRITE_FLAGS = ("--write", "--set", "--append", "--yes", "--reason", "--anchor")


def load_core():
    """导入同目录的 `baseline-ratchet.py`（唯一真源）⇒ module。导入失败 ⇒ None。"""
    if not os.path.isfile(_CORE_FILE):
        return None
    spec = importlib.util.spec_from_file_location("zerg_baseline_ratchet_core", _CORE_FILE)
    if spec is None or spec.loader is None:
        return None
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def render(core, root, conds, ledger_only=False, as_json=False, only=None):
    entries, lerr = core.load_ledger(root)
    rows, hard, soft = core.collect(root, conds, only=only, with_measure=False)
    rc = core.verdict_rc(rows, hard)
    payload = {
        "id": "baseline-readonly-view.v1",
        "single_source": "scripts/gates/baseline-ratchet.py",
        "ledger": core.LEDGER_REL,
        "ledger_entries": len(entries),
        "rows": [{"name": r["cond"]["name"], "gate": r["cond"]["gate"], "file": r["cond"]["file"],
                  "key": r["cond"]["key"], "value": r["cur"], "sha256": r["sha"], "err": r["err"],
                  "verdict": r["verdict"], "verdict_text": r["verdict_text"]} for r in rows],
        "ledger": [{"seq": e.get("seq"), "ts": e.get("ts"), "kind": e.get("kind"), "name": e.get("name"),
                    "file": e.get("file"), "old": e.get("old"), "new": e.get("new"),
                    "reason": e.get("reason"), "by": e.get("by")} for e in entries],
        "ledger_hard_errors": hard, "ledger_soft_notes": soft,
        "rc": rc,
    }
    if as_json:
        import json as _json
        print(_json.dumps(payload, ensure_ascii=False, indent=2))
        return rc

    print("── baseline 只读视图（`R-03` · 薄壳：单一真源 = `baseline-ratchet.py`）──")
    print("   本件不自己算：现值/台账/判/rc 全部导入同目录的 `baseline-ratchet.py`（**不许开第二份口径**）")
    if lerr:
        print("   ⛔ 台账解不动：%s" % lerr)
    print("   台账：`%s`（%d 条记录 · %s）"
          % (core.LEDGER_REL, len(entries), "件在" if os.path.isfile(core.ledger_path(root)) else "**件还不存在**"))
    if ledger_only:
        print("── 台账逐条（append-only · 旧值 → 新值 · 理由 · 时间 · 谁）──")
        for e in entries:
            print("   #%-3s %s  %s  %-30s %s → %s" % (e.get("seq"), e.get("ts"), e.get("kind"),
                                                      e.get("name"), e.get("old"), e.get("new")))
            print("        件 `%s` · 谁：%s · 理由：%s" % (e.get("file"), e.get("by"), e.get("reason")))
        if not entries:
            print("   （一条记录都没有 —— 这正是 `R-03` 判红的那一格）")
        return rc

    w = max([len(r["cond"]["name"]) for r in rows] + [10])
    for r in rows:
        c = r["cond"]
        print("   %-*s %-5s 现值 %-5s 件 `%s`（键 `%s`）" % (w, c["name"], c["gate"],
                                                             "—" if r["cur"] is None else r["cur"],
                                                             c["file"], c["key"]))
        print("   %-*s       %s" % (w, "", r["verdict_text"]))
    for e in hard:
        print("   ⛔ 台账硬错：%s" % e)
    for e in soft:
        print("   ⚠ 台账提示（不改判）：%s" % e)
    if rc == 1:
        print("⇒ **判红（rc=1）**：有棘轮数字在台账里没有对得上的记录（`R-03`：无记录 ⇒ 红）")
        print("   闭洞的唯一通道：`python3 scripts/gates/baseline-ratchet.py --write --reason '…'`"
              "（改数 + 留痕）或 `--anchor --reason '…'`（只记现状、不改件）")
    elif rc == 2:
        print("⇒ **不给结论（rc=2）**：台账解不动 / 形状坏了 / 有条件的现值取不到")
    else:
        print("⇒ 绿（rc=0）：每个棘轮数字都对得上台账里它自己那条记录")
    return rc


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--view", action="store_true", help="只读视图（默认档）")
    ap.add_argument("--check", action="store_true", help="同视图（「无记录 ⇒ 红」那档）")
    ap.add_argument("--ledger", action="store_true", help="台账逐条（旧值→新值 · 理由 · 时间 · 谁）")
    ap.add_argument("--only", action="append", default=None, metavar="条件名", help="只看指定条件（可重复）")
    ap.add_argument("--root", default=".", help="仓根（默认从当前目录上溯）")
    ap.add_argument("--json", action="store_true", help="机器可读输出")
    # ── 写意图：本件是**只读半件** ⇒ 一律 rc=2 且一个字节不写（写面要人签）──
    ap.add_argument("--write", action="store_true", help="写意图 ⇒ rc=2（写面只在 baseline-ratchet.py）")
    ap.add_argument("--set", metavar="值", help="写意图 ⇒ rc=2")
    ap.add_argument("--append", action="store_true", help="写意图 ⇒ rc=2")
    ap.add_argument("--anchor", action="store_true", help="写意图 ⇒ rc=2（锚也在 baseline-ratchet.py）")
    ap.add_argument("--yes", action="store_true", help="写意图 ⇒ rc=2")
    ap.add_argument("--reason", default="", help="写意图 ⇒ rc=2")
    ap.add_argument("--self-test", action="store_true", help="合成件成对负控（不碰真目标）")
    a = ap.parse_args()

    if a.self_test:
        print("── show-baseline 自检（合成件 · 不碰真目标）──")
        return self_test()

    core = load_core()
    if core is None:
        print("⛔ 单一真源导入不了（同目录 `baseline-ratchet.py` 不在/读不动）⇒ 不给结论（rc=2）")
        return 2

    root = core.find_root(a.root)
    if root is None:
        print("⛔ 找不到仓根（从 %s 上溯）⇒ 不给结论（rc=2）" % a.root)
        return 2

    intent = [f for f in WRITE_FLAGS if getattr(a, f.strip("-").replace("-", "_"), None)]
    if intent:
        shas = {}
        for c in core.CONDITIONS:
            p = os.path.join(root, c["file"])
            shas[c["file"]] = core.sha256_file(p) if os.path.isfile(p) else None
        print("   ⛔ 写意图：%s ⇒ 本件是**只读半件**（写面要人签 ⇒ 本波不做）" % " ".join(intent))
        print("   ⛔ **一个字节都不写**：写 baseline 的唯一通道 = "
              "`python3 scripts/gates/baseline-ratchet.py --write --reason '…'`（带留痕）")
        same = all((core.sha256_file(os.path.join(root, c["file"])) if os.path.isfile(os.path.join(root, c["file"]))
                    else None) == shas[c["file"]] for c in core.CONDITIONS)
        print("   只读性自证：三个棘轮件 sha256 跑前=跑后 ⇒ %s" % ("一字不变 ✓" if same else "**变了 ⇒ 判红** ✗"))
        print("⇒ 不写（rc=2）")
        return 2

    if a.only:
        for n in a.only:
            if core.cond_by_name(n, core.CONDITIONS) is None:
                print("⛔ 条件名不认：%s（列全部：`baseline-ratchet.py --conditions`）⇒ 不给结论（rc=2）" % n)
                return 2

    return render(core, root, core.CONDITIONS, ledger_only=a.ledger, as_json=a.json, only=a.only)


# ── 自检（合成件 · 不碰真目标）───────────────────────────────────────────────
FX_CONDS = (
    {"name": "fx.alpha", "gate": "门①", "file": os.path.join("scripts", "gates", "fx-alpha.json"),
     "key": "baseline", "policy": "down", "unit": "合成条件甲", "measure": [], "measure_re": r"(?!)"},
)


def self_test():
    fails = []
    core = load_core()
    if core is None:
        print("   ✗ 单一真源导入不了")
        return 2

    def case(name, got, want):
        if got == want:
            print("   ✓ %s" % name)
        else:
            print("   ✗ %s（got=%r want=%r）" % (name, got, want))
            fails.append(name)

    def case_true(name, ok):
        case(name, bool(ok), True)

    tmp = tempfile.mkdtemp(prefix="zerg-show-baseline-selftest-")
    try:
        root = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(root, "core"))
        os.makedirs(os.path.join(root, "scripts", "gates"))
        open(os.path.join(root, "core", "go.mod"), "w").close()
        open(os.path.join(root, "scripts", "gates", "precommit-gates.sh"), "w").close()
        fx = os.path.join(root, FX_CONDS[0]["file"])
        with open(fx, "w", encoding="utf-8") as f:
            f.write('{\n  "id": "fx-alpha.v1",\n  "baseline": 7\n}\n')

        # ① 单一真源：导入的就是那一件（条件表在、判据函数在）
        case_true("① 导入的是 `baseline-ratchet.py`（文件对得上）", str(core.__file__).endswith("baseline-ratchet.py"))
        case_true("① 判据面那处 rc 规则在核心件里（本件不另写）", callable(getattr(core, "verdict_rc", None)))
        case_true("① 核心件的条件表是**真表**（三个真条件在）",
                  all(core.cond_by_name(n, core.CONDITIONS) is not None for n in
                      ("gate-coverage.baseline", "cli-contract.blanks", "doc-cmds.devdocs_report_hits")))

        # ② 无记录 ⇒ **红**（`R-03` 逐字那一格）
        rows, hard, soft = core.collect(root, FX_CONDS)
        case("② 无台账件 ⇒ 判红 rc=1", core.verdict_rc(rows, hard), 1)
        case_true("② 判词点明「无记录」", "无记录" in rows[0]["verdict_text"])

        # ③ 只读视图真跑（合成件）⇒ rc=1，且件 sha256 不变
        sh = core.sha256_file(fx)
        rc = render(core, root, FX_CONDS)
        case("③ 只读视图（无记录）⇒ rc=1", rc, 1)
        case("③ 视图跑完：件 sha256 一字不变", core.sha256_file(fx), sh)

        # ④ 写旗标 ⇒ rc=2 且不写（三档都要真红：--write / --set / --yes）
        for tail in (["--write"], ["--set", "1"], ["--yes"]):
            old = sys.argv
            try:
                sys.argv = ["show-baseline.py"] + tail + ["--root", root]
                rc = main()
            finally:
                sys.argv = old
            case("④ 写意图 %s ⇒ rc=2" % " ".join(tail), rc, 2)
            case("④ 写意图 %s 跑完：件 sha256 一字不变" % " ".join(tail), core.sha256_file(fx), sh)

        # ⑤ 台账逐条：追一条锚后能逐条列出（理由/时间/谁 都在）
        entry = {"seq": 1, "ts": core.now_iso(), "kind": "anchor", "name": "fx.alpha", "gate": "门①",
                 "file": FX_CONDS[0]["file"], "old": 7, "new": 7, "reason": "自检：现状锚",
                 "by": "selftest", "sha256_before": sh, "sha256_after": sh, "measure": "（锚）"}
        case("⑤ 台账追得上", core.append_ledger(root, entry), None)
        rows, hard, soft = core.collect(root, FX_CONDS)
        case("⑤ 锚之后 ⇒ 绿 rc=0", core.verdict_rc(rows, hard), 0)
        rc = render(core, root, FX_CONDS, ledger_only=True)
        case("⑤ 台账逐条档 ⇒ rc=0", rc, 0)
        entries, _ = core.load_ledger(root)
        case("⑤ 台账四格齐（件/旧→新/理由/时间）",
             (entries[0]["file"], entries[0]["old"], entries[0]["new"], bool(entries[0]["reason"]),
              bool(entries[0]["ts"])), (FX_CONDS[0]["file"], 7, 7, True, True))

        # ⑥ 手改值（无记录）⇒ 又红（成对：红红在台账那格，不是红在夹具）
        with open(fx, encoding="utf-8") as f:
            raw = f.read()
        with open(fx, "w", encoding="utf-8") as f:
            f.write(raw.replace('"baseline": 7', '"baseline": 9'))
        rows, hard, soft = core.collect(root, FX_CONDS)
        case("⑥ 手改值无记录 ⇒ 判红 rc=1", core.verdict_rc(rows, hard), 1)
        case_true("⑥ 判词点明「没记录的手改」", "没记录的手改" in rows[0]["verdict_text"])

        # ⑦ 台账断号 ⇒ 不给结论（rc=2）
        lp = core.ledger_path(root)
        with open(lp, "a", encoding="utf-8") as f:
            f.write('{"seq": 9, "name": "fx.alpha", "file": "x", "old": 9, "new": 9}\n')
        rows, hard, soft = core.collect(root, FX_CONDS)
        case("⑦ 台账断号 ⇒ rc=2（不给结论，不许当绿）", core.verdict_rc(rows, hard), 2)

        # ⑧ 件不在 ⇒ 判不了（不是绿）
        os.remove(fx)
        rows, hard, soft = core.collect(root, FX_CONDS)
        case_true("⑧ 件不在 ⇒ 判词是「判不了」（≠ 绿）", rows[0]["verdict"].startswith("n/a"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("自检结论: %s（失败 %d 条）" % ("全过" if not fails else "**有失败**", len(fails)))
    return 0 if not fails else 2


if __name__ == "__main__":
    sys.exit(main())

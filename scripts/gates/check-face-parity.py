#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-face-parity.py —— 提交闸「索引面 / 面自陈」判据（缺口 `GAP-20260927-79`）。

病根（现读坐实）：提交闸对**工作树**取证而非**索引面** —— 31/31 快档步内容面取自工作树、0/31
取自索引 ⇒ 未跟踪件 / 未暂存件会替提交者变绿（假绿）。修法 = 跑闸前**物化索引**（`git checkout-index`）
并以 `ZERG_GATE_REPO_ROOT` 指过去（本判据即钉住这一形态不回退）。

判据（**硬**：命中 ⇒ rc=1；缺件 / 读不到 ⇒ rc=2「不给结论」）：
  F1 物化索引：两条提交出口都必须把**索引面**物化后跑闸 ——
     `.githooks/pre-commit` 含 `checkout-index` + `ZERG_GATE_REPO_ROOT`；
     `core/cmd/zerg/family_repo_commit.go` 含 `materializeIndex(` + `ZERG_GATE_REPO_ROOT`。
  F2 禁 `git archive HEAD`：两条出口都不得用 `git archive`（HEAD ≠ 索引 ⇒ 给假绿 · want 明写排除）。
  F3 面自陈：两条出口都必须**自陈**它跑的是「索引面」（人面输出里出现「索引面」）。

判据（**只报告**：进清单、**不计 rc**）：
  F4 同一包混入两棵树：`scripts/gates/*.py` 里既读**工作树面**选择器
     （`git diff --name-only HEAD` / `git status` / `git ls-files --others`）又读 `ZERG_GATE_REPO_ROOT`
     ⇒ 同一趟混了两棵树（报，不判硬 —— 可能是合法的两态对拍）。
  F5 台账类判据应改索引面：件名含 `ledger` / `judge` 且读 `.jsonl` 台账的判据，若用**工作树面**
     选择器（`git diff --name-only HEAD` / `git status`）而非 `--cached` / `ZERG_GATE_REPO_ROOT` ⇒ 报。

退码：0 全过 · 1 命中（硬）/ 报面有清单 · 2 判不了（缺件 / 读不到 / 自检不过）。
本件**自指面**：F4/F5 扫 `scripts/gates/*.py` ⇒ 一律排除**本件自身**（否则自己带的两串会自命中）。
"""
import argparse
import json
import os
import sys
import tempfile
from typing import Any, Dict, Tuple

VERSION = "check-face-parity/1"

DEFAULT_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# ── 硬判据的「件 + 必须含的串」表（F1 / F3）────────────────────────────────────
#   (相对件名, [必须出现的串...], 人类说明)
HARD_REQUIRE = [
    (os.path.join(".githooks", "pre-commit"),
     ["checkout-index", "ZERG_GATE_REPO_ROOT"], "出口② 提交钩子（物化索引 + 指面跑闸）"),
    (os.path.join("core", "cmd", "zerg", "family_repo_commit.go"),
     ["materializeIndex(", "ZERG_GATE_REPO_ROOT"], "出口① `repo commit`（物化索引 + 指面跑闸）"),
]
# F3 面自陈：两条出口都要在人面输出里点到「索引面」。
SELFDECL = [
    (os.path.join(".githooks", "pre-commit"), "索引面"),
    (os.path.join("core", "cmd", "zerg", "family_repo_commit.go"), "索引面"),
]
# F2：禁出现的串（HEAD ≠ 索引 ⇒ 假绿）。
FORBID = [(os.path.join(".githooks", "pre-commit"), "git archive"),
          (os.path.join("core", "cmd", "zerg", "family_repo_commit.go"), "git archive")]
# F4/F5 的工作树面选择器 vs 索引面选择器。
WORKTREE_FACE = ["git diff --name-only HEAD", "git status", "git ls-files --others"]
INDEX_FACE = ["--cached", "ZERG_GATE_REPO_ROOT", "checkout-index"]
LEDGER_HINT = ("ledger", "judge")


def read_text(path):
    """⇒ (文本, 错误)；二进制 / 读不到 ⇒ 错误非空。"""
    try:
        with open(path, "rb") as fh:
            raw = fh.read()
    except OSError as exc:
        return None, str(exc)
    try:
        return raw.decode("utf-8"), None
    except UnicodeDecodeError as exc:
        return None, "非 UTF-8：%s" % exc


def _has_code_line(txt: str, needle: str) -> bool:
    """`needle` 是否出现在**非注释**行（整行注释跳过）：说明文里写「不用 X」不算用 X。"""
    for ln in txt.splitlines():
        s = ln.strip()
        if s.startswith("#") or s.startswith("//"):
            continue
        if needle in ln:
            return True
    return False


def judge(root: str) -> Tuple[int, Dict[str, Any]]:
    """⇒ (rc, result)。result 供人面 / json 面。"""
    hard, selfdecl, forbid, mixed, ledger = [], [], [], [], []
    seen: Dict[str, str] = {}
    for rel, needles, why in HARD_REQUIRE:
        p = os.path.join(root, rel)
        txt, err = read_text(p)
        if txt is None:
            return 2, {"error": "缺件 / 读不到：%s（%s）⇒ 判不了（不许当好）" % (rel, err)}
        seen[rel] = txt
        miss = [n for n in needles if n not in txt]
        if miss:
            hard.append({"file": rel, "why": why, "missing": miss})
    for rel, needle in SELFDECL:
        txt = seen.get(rel)
        if txt is None:
            txt, err = read_text(os.path.join(root, rel))
            if txt is None:
                return 2, {"error": "缺件 / 读不到：%s（%s）⇒ 判不了" % (rel, err)}
        if needle not in txt:
            selfdecl.append({"file": rel, "missing": needle})
    for rel, needle in FORBID:
        txt = seen.get(rel)
        # ★ 只查**代码行**：说明文里写「我们不用 `git archive`」不该被自己点名（注释整行跳过）。
        if txt is not None and _has_code_line(txt, needle):
            forbid.append({"file": rel, "forbidden": needle})

    # ── F4 / F5（只报告）：扫 scripts/gates/*.py（排除本件自身）──────────────────
    me = os.path.basename(os.path.abspath(__file__))
    gdir = os.path.join(root, "scripts", "gates")
    try:
        names = sorted(n for n in os.listdir(gdir) if n.endswith(".py"))
    except OSError:
        names = []
    for n in names:
        if n == me:
            continue
        txt, err = read_text(os.path.join(gdir, n))
        if txt is None:
            continue
        has_wt = any(_has_code_line(txt, w) for w in WORKTREE_FACE)
        has_idx = any(_has_code_line(txt, i) for i in INDEX_FACE)
        if has_wt and has_idx:
            mixed.append("scripts/gates/" + n)
        if any(h in n for h in LEDGER_HINT) and ".jsonl" in txt and has_wt and ("--cached" not in txt):
            ledger.append("scripts/gates/" + n)

    rc = 1 if (hard or selfdecl or forbid) else 0
    return rc, {"hard": hard, "selfdecl": selfdecl, "forbid": forbid,
                "mixed": mixed, "ledger": ledger}


def emit_human(res, rc):
    if "error" in res:
        print("✗ " + res["error"])
        return
    print("== 提交闸「索引面 / 面自陈」判据（缺口 `GAP-20260927-79`）==")
    print("   F1 物化索引 + F2 禁 `git archive` + F3 面自陈（硬）")
    for x in res["hard"]:
        print("   ✗ F1 %s（%s）：缺 %s" % (x["file"], x["why"], " · ".join(x["missing"])))
    for x in res["forbid"]:
        print("   ✗ F2 %s：出现禁用串 %r（HEAD ≠ 索引 ⇒ 假绿）" % (x["file"], x["forbidden"]))
    for x in res["selfdecl"]:
        print("   ✗ F3 %s：面自陈缺 %r（跑哪一面必须自陈）" % (x["file"], x["missing"]))
    print("   F4 同一包混入两棵树（只报告）：%d 件" % len(res["mixed"]))
    for m in res["mixed"]:
        print("      · " + m)
    print("   F5 台账类判据应改索引面（只报告）：%d 件" % len(res["ledger"]))
    for m in res["ledger"]:
        print("      · " + m)
    if rc == 0:
        print("== 结论 ==\n   ⇒ 绿（rc=0）：两条出口均物化索引 · 无 `git archive` · 面自陈在位")
    else:
        print("== 结论 ==\n   ⇒ 红（rc=1）：有硬命中（缺物化索引 / 用了 `git archive` / 缺面自陈）")


def selftest(stdout):
    ok = True
    total = 0

    def chk(name, cond):
        nonlocal ok, total
        total += 1
        stdout.write("   %s %s\n" % ("✓" if cond else "✗", name))
        ok = ok and cond

    good_hook = ("#!/usr/bin/env bash\n# 索引面\ngit -C R checkout-index -a -f --prefix=X/\n"
                 "ZERG_GATE_REPO_ROOT=X bash gate --fast\n")
    good_go = ("// 索引面\nfunc f(){ matRoot,_:=materializeIndex(root); _=gateEnv(matRoot) }\n"
               "const k = \"ZERG_GATE_REPO_ROOT\"\n")
    with tempfile.TemporaryDirectory(prefix="cfp-selftest-") as d:
        os.makedirs(os.path.join(d, ".githooks"))
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        os.makedirs(os.path.join(d, "scripts", "gates"))
        h = os.path.join(d, ".githooks", "pre-commit")
        g = os.path.join(d, "core", "cmd", "zerg", "family_repo_commit.go")
        open(h, "w").write(good_hook)
        open(g, "w").write(good_go)
        rc, res = judge(d)
        chk("正控：两条出口齐（物化 + 指面 + 面自陈）⇒ rc=0", rc == 0)
        # 负控①：钩子去掉物化 ⇒ F1 硬命中 rc=1
        open(h, "w").write(good_hook.replace("checkout-index", "XX"))
        rc, res = judge(d)
        chk("负控①：钩子缺 checkout-index ⇒ rc=1 且 F1 点名该件",
            rc == 1 and any(x["file"].endswith("pre-commit") for x in res["hard"]))
        open(h, "w").write(good_hook)
        # 负控②：出现 git archive ⇒ F2 硬命中
        open(g, "w").write(good_go + "var zzArchive = \"git archive HEAD\"\n")
        rc, res = judge(d)
        chk("负控②：出口出现 `git archive` ⇒ rc=1 且 F2 点名",
            rc == 1 and any(x["file"].endswith("family_repo_commit.go") for x in res["forbid"]))
        open(g, "w").write(good_go)
        # 负控③：面自陈缺失 ⇒ F3 硬命中
        open(h, "w").write(good_hook.replace("索引面", "XX"))
        rc, res = judge(d)
        chk("负控③：缺「索引面」自陈 ⇒ rc=1 且 F3 点名",
            rc == 1 and any(x["file"].endswith("pre-commit") for x in res["selfdecl"]))
        open(h, "w").write(good_hook)
        # 负控④：缺件 ⇒ rc=2（判不了）
        os.remove(g)
        rc, res = judge(d)
        chk("负控④：缺 family_repo_commit.go ⇒ rc=2（不给结论）", rc == 2)
        open(g, "w").write(good_go)
        # F4：同一包混入两棵树 ⇒ 只报告（rc=0）
        open(os.path.join(d, "scripts", "gates", "zz-mixed.py"), "w").write(
            "import os, subprocess\nX = \"git diff --name-only HEAD\"\n"
            "os.environ[\"ZERG_GATE_REPO_ROOT\"]\n")
        rc, res = judge(d)
        chk("F4 只报告：混两棵树的件被点名且 rc 仍 0",
            rc == 0 and "scripts/gates/zz-mixed.py" in res["mixed"])
        # 自指面：本件自身不许被 F4 点名
        selfname = os.path.basename(os.path.abspath(__file__))
        chk("自指面：本件自身不被 F4 点名", ("scripts/gates/" + selfname) not in res["mixed"])
    stdout.write("── %s（%d 格）──\n" % ("全过" if ok else "有红", total))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(prog="check-face-parity.py", add_help=True)
    ap.add_argument("root", nargs="?", default=None)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return selftest(sys.stdout)
    root = a.root or os.environ.get("ZERG_GATE_REPO_ROOT") or DEFAULT_ROOT
    root = os.path.abspath(root)
    rc, res = judge(root)
    if a.json:
        print(json.dumps({"schema": VERSION, "rc": rc, "root": root, "result": res},
                         ensure_ascii=False))
        return rc
    emit_human(res, rc)
    return rc


if __name__ == "__main__":
    sys.exit(main())

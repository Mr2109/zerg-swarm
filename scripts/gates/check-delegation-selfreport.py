#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-delegation-selfreport.py —— 委派「**改动面 vs 自述面**」对账门（缺口 `GAP-20260927-333`）

题面（缺口账 `GAP-20260927-333` 逐字）
------------------------------------
「委派产出的『自述不实』无机器判据：卵自述『新增代码全在分叉内』，**实际删掉了既有函数
 `gapFoundDays` 的 parse 循环** ⇒ 负控才抓到。」`want_action`：「加『改动面 vs 自述面』对账判据
（**diff 触及的既有函数清单必须与自述一致**）」。

它要抓的那一类错
----------------
委派回执（卵自述）说「我只加了新代码、没动既有逻辑」，而 `git diff` 里**既有函数的函数体真被改动 /
被删** ⇒ 自述与改动面不符。本门把两边都取成**结构化的符号集合**再对拍，**不读散文、不做语义判断**：

  ① **改动面**（机器取）：`git diff <base>` 触及的、**base 里已存在**的顶层函数（`func`/`def`）
     —— 逐件逐函数比**函数体**（去尾空白归一化后逐字比）：体变了 ⇒ **modified**；base 有、新树没有
     ⇒ **removed**；新树新加 ⇒ **added**（新加不判红）。
  ② **自述面**（回执给）：一份 JSON 自述件（schema 见下），声明它**动过 / 删过**哪些既有符号、
     加了哪些新符号，以及一句**总声明** `claims_no_existing_modification`（「没动任何既有函数」）。
  ③ **对账**：改动面里 **modified ∪ removed** 的函数，**凡不在自述面声明的** ⇒ **判红并点名** ✗
     （★ 若 `claims_no_existing_modification=true` 而 modified/removed 非空 ⇒ 直接坐实「自述不实」）。
     自述面声明了、改动面却没有的符号 ⇒ 只**报告**（回执可能夸大，不判红）。

自述件 schema（`--report` 指到的仓外 JSON · 本门只读它）
------------------------------------------------------
    {"schema":"zerg/v1", "kind":"DelegationSelfReport", "model":"...",
     "files":["core/cmd/zerg/family_gap.go", ...],          # 本次声称改过的件（缺省 ⇒ 用 diff 全量）
     "declared":{"existing_symbols_modified":["gapFoundDays", ...],
                 "existing_symbols_removed":[],
                 "new_symbols_added":["gapQidOf", ...]},
     "claims_no_existing_modification": false}

判据（R 编号逐条给判红/报告的理由）
----------------------------------
  **R1 自述件可读可解析**（不给结论 rc=2）：`--report` 缺件 / 非 JSON / 缺 `declared` ⇒ rc=2。
  **R2 改动面可算**（不给结论 rc=2）：`git diff` 取不到 base / 某件两侧都读不到 ⇒ rc=2（判不了不当绿）。
  **R3 对账（判红 rc=1）**：`(modified ∪ removed) - 自述声明` 非空 ⇒ 判红逐条点名；
    `claims_no_existing_modification==true` 且 `modified ∪ removed` 非空 ⇒ 判红（总声明直接证伪）。
  **R4 空改动 / 无函数（只报告）**：diff 里一个函数都没触及 ⇒ 报告「无既有函数被触及」（不当病）。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 改动面全部在自述面之内（或改动面为空）  ·  1 = 有未声明的既有函数被改/删（自述不实）
  2 = **不给结论**：自述件读不到/解析不了 / `git` base 取不到 / 件两侧读不到 / 自检不过

用法
----
    python3 scripts/gates/check-delegation-selfreport.py --report <仓外自述.json> [--base <rev>] [--root <仓根>]
    python3 scripts/gates/check-delegation-selfreport.py --report <自述.json> --json
    python3 scripts/gates/check-delegation-selfreport.py --list-rules
    python3 scripts/gates/check-delegation-selfreport.py --self-test

★ 本件**纯只读**：只读自述件 + `git show`/`git diff` + 读工作树件 ⇒ **不写任何件、不改任何态** ·
  ✗ 不挂 `precommit-gates.sh`（登记面属收口批）· 判不了即点名、不许当绿。
★ 限度（如实写在件里 · 不假装全能）：函数边界按**顶格 `func`/`def` + 括号配平**（Go）/ 缩进（Py）识别，
  不解析泛型约束里的花括号与原始字符串；多语言件只做行区间近似 ⇒ 只报「不确定」不硬判红。
"""

import argparse
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-delegation-selfreport.py v1.0.0"

GO_FUNC_RE = re.compile(r"^func\s*(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(")
PY_DEF_RE = re.compile(r"^def\s+([A-Za-z_]\w*)\s*\(")
GENERIC_DEF_RE = re.compile(r"^(?:function|fn)\s+([A-Za-z_]\w*)\s*[\(\{]?")


# ── 源码 → 顶层函数 {name: 归一化函数体} ────────────────────────────────────────
def norm_body(lines):
    return "\n".join(l.rstrip() for l in lines if l.strip())


def _brace_scan(lines, i0):
    """从 `func` 行起按花括号配平找函数末尾（返回末行下标 · 1 起）。"""
    depth = 0
    seen = False
    for i in range(i0, len(lines)):
        for ch in lines[i]:
            if ch == "{":
                depth += 1
                seen = True
            elif ch == "}":
                depth -= 1
        if seen and depth <= 0:
            return i + 1
    return len(lines)


def _indent_scan(lines, i0):
    """Python：从 `def` 行起、缩进回到 0 的下一非空行为止。"""
    for i in range(i0 + 1, len(lines)):
        if lines[i].strip() and not lines[i][0].isspace():
            return i
    return len(lines)


def parse_go_funcs(text):
    """{name: body}（Go · 顶格 func + 花括号配平）。同名重载只留最后一个（如实限）。"""
    lines = text.split("\n")
    out = {}
    for i, ln in enumerate(lines):
        m = GO_FUNC_RE.match(ln)
        if not m:
            continue
        end = _brace_scan(lines, i)
        out[m.group(1)] = norm_body(lines[i:end])
    return out


def parse_py_funcs(text):
    lines = text.split("\n")
    out = {}
    for i, ln in enumerate(lines):
        m = PY_DEF_RE.match(ln)
        if not m:
            continue
        end = _indent_scan(lines, i)
        out[m.group(1)] = norm_body(lines[i:end])
    return out


def parse_generic_funcs(text):
    lines = text.split("\n")
    out = {}
    starts = [(i, m.group(1)) for i, ln in enumerate(lines)
              for m in [GENERIC_DEF_RE.match(ln)] if m]
    for k, (i, name) in enumerate(starts):
        end = starts[k + 1][0] if k + 1 < len(starts) else len(lines)
        out[name] = norm_body(lines[i:end])
    return out


def parse_funcs(path, text):
    ext = os.path.splitext(path)[1].lower()
    if ext == ".go":
        return parse_go_funcs(text)
    if ext == ".py":
        return parse_py_funcs(text)
    return parse_generic_funcs(text)


def analyze(base_text, new_text, path):
    """返回 (modified, removed, added)（各是名称集合）。"""
    b = parse_funcs(path, base_text)
    n = parse_funcs(path, new_text)
    modified = {k for k in b if k in n and b[k] != n[k]}
    removed = set(b) - set(n)
    added = set(n) - set(b)
    return modified, removed, added


def git_show(root, rev, rel):
    try:
        p = subprocess.run(["git", "-C", root, "show", "%s:%s" % (rev, rel)],
                           capture_output=True, text=True)
    except OSError as e:
        return None, "git 起不动：%s" % e
    if p.returncode != 0:
        return None, "git show %s:%s ⇒ rc=%d（%s）" % (rev, rel, p.returncode,
                                                     (p.stderr or "").strip()[:200])
    return p.stdout, None


def read_tree(root, rel):
    p = os.path.join(root, rel)
    try:
        with open(p, "r", encoding="utf-8", errors="replace") as f:
            return f.read(), None
    except OSError as e:
        return None, "工作树件读不到：%s" % e


def reconcile(modified, removed, declared_mod, declared_rem, claims_clean):
    undeclared_mod = sorted(set(modified) - declared_mod - declared_rem)
    undeclared_rem = sorted(set(removed) - declared_rem - declared_mod)
    reds = []
    for s in undeclared_mod:
        reds.append({"rule": "R3", "what": "既有函数 %s 函数体被改 · 自述面未声明" % s})
    for s in undeclared_rem:
        reds.append({"rule": "R3", "what": "既有函数 %s 被删 · 自述面未声明" % s})
    if claims_clean and (modified or removed):
        reds.append({"rule": "R3",
                     "what": "自述声称「未动任何既有函数」，实有 modified=%s removed=%s ⇒ 自述不实"
                             % (sorted(modified), sorted(removed))})
    return reds


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True,
                                 description="委派「改动面 vs 自述面」对账门（GAP-20260927-333）")
    ap.add_argument("--report", default=None, help="仓外自述件（JSON · schema 见件头）")
    ap.add_argument("--base", default="HEAD", help="diff 基线 rev（缺省 HEAD）")
    ap.add_argument("--root", default=REPO_ROOT, help="仓根（默认 = 本脚本所在仓）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true", help="打登记面：判据 / 退码 / 限度")
    ap.add_argument("--self-test", action="store_true", help="合成夹具自检（不碰真仓）")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        print("登记面：")
        print("  R1 自述件可读可解析（缺件/非 JSON/缺 declared ⇒ rc=2）")
        print("  R2 改动面可算（git base 取不到 / 件两侧读不到 ⇒ rc=2）")
        print("  R3 对账：改动面(modified∪removed) - 自述声明 非空 ⇒ 判红；claims_no_existing_modification 直接证伪")
        print("  R4 空改动 / 无函数 ⇒ 只报告")
        print("  退码：0 过 · 1 判红（自述不实）· 2 不给结论")
        print("  限度：Go 顶格 func + 花括号配平 / Py 缩进 / 其余行区间近似（只报不确定不硬判）")
        return 0

    # R1
    if not a.report:
        return emit_problem(a, "R1", "缺 `--report`（没有自述面无从对账）", {})
    try:
        with open(a.report, "r", encoding="utf-8") as f:
            rep = json.load(f)
    except (OSError, ValueError) as e:
        return emit_problem(a, "R1", "自述件读不到/解析不了：%s" % e, {"path": a.report})
    if not isinstance(rep, dict) or not isinstance(rep.get("declared"), dict):
        return emit_problem(a, "R1", "自述件缺 `declared` 块（schema 见件头）", {"path": a.report})
    dec = rep["declared"]
    declared_mod = set(dec.get("existing_symbols_modified") or [])
    declared_rem = set(dec.get("existing_symbols_removed") or [])
    claims_clean = bool(rep.get("claims_no_existing_modification"))

    root = os.path.abspath(a.root)
    files = rep.get("files") or []
    if not files:
        return emit_problem(a, "R2", "自述件没给 `files` ⇒ 改动面作用域不定（不由本门猜全仓 diff）", {})

    all_mod, all_rem, all_add = set(), set(), set()
    per_file = []
    for rel in files:
        base_text, err = git_show(root, a.base, rel)
        if err:
            # 件在 base 不存在（新增件）⇒ 合法，整件算 added
            continue
        new_text, err2 = read_tree(root, rel)
        if err2:
            return emit_problem(a, "R2", err2, {"file": rel})
        m, r, ad = analyze(base_text, new_text, rel)
        all_mod |= m
        all_rem |= r
        all_add |= ad
        per_file.append({"file": rel, "modified": sorted(m), "removed": sorted(r), "added": sorted(ad)})

    reds = reconcile(all_mod, all_rem, declared_mod, declared_rem, claims_clean)
    rc = 1 if reds else 0
    reports = [{"rule": "R4", "what": "无既有函数被触及（改动面只在新增）", "read": {}}] \
        if not (all_mod or all_rem) else []
    declared_untouched = sorted((declared_mod | declared_rem) - all_mod - all_rem)

    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "DelegationSelfReportReconcile", "tool": VERSION,
                          "items": [{"rule": "R3", "state": "red" if rc == 1 else "ok"}],
                          "meta": {"root": root, "base": a.base, "report": a.report,
                                   "claims_no_existing_modification": claims_clean,
                                   "modified": sorted(all_mod), "removed": sorted(all_rem),
                                   "added": sorted(all_add), "per_file": per_file},
                          "reds": reds, "problems": [],
                          "reports": reports + ([{"rule": "R3", "what": "自述声明但改动面未见：%s"
                                                  % declared_untouched, "read": {}}] if declared_untouched else []),
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    print("── 委派「改动面 vs 自述面」对账（%s）──" % VERSION)
    print("  自述件 : %s" % a.report)
    print("  基线   : %s @ %s" % (a.base, root))
    print("  改动面 : modified %d · removed %d · added %d" % (len(all_mod), len(all_rem), len(all_add)))
    for pf in per_file:
        print("    · %s: mod=%s rem=%s add=%s" % (pf["file"], pf["modified"], pf["removed"], pf["added"]))
    if declared_untouched:
        print("  · 自述声明但未见：%s（只报告）" % declared_untouched)
    for r in reds:
        print("  ✗ [%s] %s" % (r["rule"], r["what"]))
    if not (all_mod or all_rem):
        print("  · 无既有函数被触及（改动面只在新增）")
    print("%s rc=%d" % ("✓" if rc == 0 else "✗", rc))
    return rc


def emit_problem(a, rule, what, read):
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "DelegationSelfReportReconcile", "tool": VERSION,
                          "items": [{"rule": rule, "state": "problem"}],
                          "meta": {"report": getattr(a, "report", None)},
                          "reds": [], "problems": [{"rule": rule, "what": what, "read": read}],
                          "reports": [], "warnings": [], "truncated": False}, ensure_ascii=False))
    else:
        print("⛔ 不给结论（rc=2）：%s" % what)
    return 2


# ═════════════════════════════════════ 自检 ════════════════════════════════════════════
def self_test():
    ok = True

    def case(name, got, want):
        nonlocal ok
        good = got == want
        ok = ok and good
        print("  %s %s（got=%r · want=%r）" % ("✓" if good else "✗", name, got, want))

    # 复刻 GAP-20260927-333 的场景：既有函数 gapFoundDays 的 parse 循环被删（体变）
    base_go = '''package zerg

func gapFoundDays(s string) int {
\tn := 0
\tfor _, r := range s {
\t\tif r == 'x' { n++ }
\t}
\treturn n
}

func gapNextID() string { return "GAP" }
'''
    new_go = '''package zerg

func gapFoundDays(s string) int {
\treturn 0
}

func gapNextID() string { return "GAP" }

func gapQidOf(s string) string { return "Q-001" }
'''
    m, r, ad = analyze(base_go, new_go, "x.go")
    case("Go 体变检测（gapFoundDays 被改）", (sorted(m), sorted(r), sorted(ad)),
         (["gapFoundDays"], [], ["gapQidOf"]))

    # 复刻「自述不实」：claims_clean=true 而 gapFoundDays 被改 ⇒ 判红
    reds = reconcile(m, r, set(), set(), True)
    case("claims_clean=true 而体变 ⇒ 判红",
         (len(reds) >= 1, any("自述不实" in x["what"] for x in reds)), (True, True))
    # 声明了 ⇒ 不红
    reds2 = reconcile(m, r, {"gapFoundDays"}, set(), True)
    case("声明了 gapFoundDays ⇒ 仍因 claims_clean 直接证伪而红",
         any("自述不实" in x["what"] for x in reds2), True)
    reds3 = reconcile(m, r, {"gapFoundDays"}, set(), False)
    case("声明了且未声称未动 ⇒ 不红", reds3, [])
    # 未声明的删函数 ⇒ 红
    m2, r2, _ = analyze(base_go, "package zerg\n", "x.go")
    reds4 = reconcile(m2, r2, set(), set(), False)
    case("既有函数被删且未声明 ⇒ 判红点名 removed", (sorted(r2), any("被删" in x["what"] for x in reds4)),
         (["gapFoundDays", "gapNextID"], True))

    # Python 缩进识别
    base_py = "def a():\n    return 1\n\ndef b():\n    return 2\n"
    new_py = "def a():\n    return 9\n\ndef b():\n    return 2\n"
    m3, r3, a3 = analyze(base_py, new_py, "x.py")
    case("Py 缩进识别（a 被改）", (sorted(m3), sorted(r3), sorted(a3)), (["a"], [], []))

    # 函数体归一化：只差尾随空白 ⇒ 不算改
    base_w = "func f() {\n\treturn 1\n}\n"
    new_w = "func f() {   \n\treturn 1\n}\n"
    m4, _, _ = analyze(base_w, new_w, "x.go")
    case("尾随空白不算改动", m4, set())

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

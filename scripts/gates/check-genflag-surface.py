#!/usr/bin/env python3
# check-genflag-surface.py —— 生成/导出器「旗标面」机检（缺口 GAP-20260928-59）
#
# 义（缺口逐字）：生成器/导出器族（`tools/*.py` ＋ `scripts/docs/gen-*.py`）必须装「旗标面」——
#   ① 有写面者必须有旗标面（闭集）；② 未知旗标必须**拒执**（rc≠0），✗ 不许静默吞后直落默认写动作；
#   ③ 必须有 `--help`/`-h` 面（只印用法 · ✗ 不许把帮助请求落成写动作）。
#
# 三态诊断（任一即命中；一件只报一态 · 优先级 甲 > 丙 > 乙）：
#   甲·默认动作写盘       = 有写面 ∧ 无旗标面（无 argparse ∧ 无 sys.argv 处理）
#   丙·未知旗标 rc=0 且写盘 = 有写面 ∧ 无「未知旗标 ⇒ 拒执」面
#   乙·认旗标仍写盘       = 有写面 ∧ 有旗标面但无 `--help`/`-h` 面
#
# 口径：**静态只读**（✗ 禁真跑写面 —— 本族含硬编码仓外绝对路径者，真跑会改仓外件）。
#   手工旗标面的「拒执面」以字面词表（未知/不认/unrecognized/unknown）识别；argparse 面
#   （`ArgumentParser(` 且未 `add_help=False` 且 `parse_args(` 且未 `parse_known_args`）自带给两面。
#
# 档：**tri-report**（只报告 · 命中 rc=1 落 REPORT · ✗ 不阻断）；冷启动棘轮 = 首读命中（见 BASELINE）。
#   rc=2 = 取数面拿不到（仓根取不到 / `tools` 或 `scripts/docs` 不在盘）⇒ 不给结论。
from __future__ import annotations

import argparse
import fnmatch
import json
import os
import re
import sys

TOOL = "check-genflag-surface.py"
VERSION = "v1.0.0"
GATE_NAME = "生成/导出器 旗标面（默认动作写盘 / 认旗标仍写盘 / 未知旗标 rc=0 且写盘）"
SURFACES = ["tools/*.py", "scripts/docs/gen-*.py"]

RC_OK = 0
RC_REPORT = 1  # 有新命中（tri-report ⇒ 落 REPORT）
RC_BLOCK = 2   # 取数面拿不到 ⇒ 不给结论

# ── 冷启动棘轮基线（存量表）：首读即命中的件 + 态 ⇒ 只打印、不进退码。**只增不删** ──
# 条目形制：(件仓相对路径, 态码 甲|乙|丙)。表外的（=**新出现的**）才是命中 ⇒ rc=1。
# ★ 2026-10-08 首读现读（扫 6 件 · 有写面 5 件）：本族两件病史（kb_docs_sync/gen_tools_md）
#   已于 2026-09-28 治好；其余三件走 argparse ⇒ 首读命中 **0** ⇒ 基线 **空**。
BASELINE = [
    # （空 · 首读无存量命中）
]

# ── 静态取数规则（RE2 兼容 · 两半逐条同源）────────────────────────────────────
WRITE_RES = [
    r"open\s*\([^\n]*,\s*[\"'][wax]",
    r"\.write_text\s*\(",
    r"\.write_bytes\s*\(",
    r"json\.dump\s*\(",
    r"shutil\.copy",
    r"os\.replace\s*\(",
]
RE_ARGV = r"sys\.argv"
RE_PARSER = r"ArgumentParser\s*\("
RE_PARSE = r"parse_args\s*\("
RE_NO_HELP = r"add_help\s*=\s*False"
RE_KNOWN_ARGS = r"parse_known_args"
RE_HELP = r"--help|[\"']-h[\"']"
RE_REJECT = r"未知|不认|unrecognized|unknown"

# ── 态码 ─────────────────────────────────────────────────────────────────────
ST_A = "甲"  # 默认动作写盘
ST_B = "乙"  # 认旗标仍写盘
ST_C = "丙"  # 未知旗标 rc=0 且写盘
ST_LABEL = {ST_A: "甲·默认动作写盘", ST_B: "乙·认旗标仍写盘", ST_C: "丙·未知旗标 rc=0 且写盘"}


def strip_comments(text):
    """剥注释：整行注释丢弃；行尾 ` #…` 截断（两半同形 · 定序）。"""
    out = []
    for ln in text.split("\n"):
        if ln.lstrip().startswith("#"):
            continue
        i = ln.find(" #")
        if i >= 0:
            ln = ln[:i]
        out.append(ln)
    return "\n".join(out)


def classify(body):
    """静态判一件（输入 = 已剥注释正文）⇒ dict(写面/甲/乙/丙/面码)。"""
    write = any(re.search(p, body) for p in WRITE_RES)
    argv = re.search(RE_ARGV, body) is not None
    parser = re.search(RE_PARSER, body) is not None
    parse = re.search(RE_PARSE, body) is not None
    ap_ok = parser and parse and not re.search(RE_NO_HELP, body) and not re.search(RE_KNOWN_ARGS, body)
    has_face = ap_ok or argv
    help_face = ap_ok or (re.search(RE_HELP, body) is not None)
    reject_face = ap_ok or (re.search(RE_REJECT, body) is not None)

    state = ""
    if write:
        if not has_face:
            state = ST_A
        elif not reject_face:
            state = ST_C
        elif not help_face:
            state = ST_B
    return {
        "write": write, "argv": argv, "ap": ap_ok,
        "face": has_face, "help": help_face, "reject": reject_face, "state": state,
    }


def family(root):
    """本族件（仓相对路径 · 按名升序 · 去重）＋ 面缺说明（非空 ⇒ rc=2）。"""
    rels, errs = [], []
    for pat in SURFACES:
        d, base = os.path.split(pat)
        dpath = os.path.join(root, d)
        if not os.path.isdir(dpath):
            errs.append("面不在盘：" + d)
            continue
        for fn in sorted(os.listdir(dpath)):
            if fnmatch.fnmatch(fn, base) and os.path.isfile(os.path.join(dpath, fn)):
                rels.append(d + "/" + fn)
    return sorted(set(rels)), errs


def compute(root):
    rels, errs = family(root)
    rows = []
    for rel in rels:
        p = os.path.join(root, rel)
        try:
            with open(p, encoding="utf-8", errors="replace") as f:
                body = strip_comments(f.read())
        except OSError as e:  # noqa: BLE001
            errs.append("读不到：%s（%s）" % (rel, e))
            continue
        c = classify(body)
        rows.append((rel, c))
    return rows, errs


def audit(rows, base_pairs=None):
    base = set(BASELINE if base_pairs is None else base_pairs)
    hits, stock = [], []
    for rel, c in rows:
        if not c["state"]:
            continue
        if (rel, c["state"]) in base:
            stock.append((rel, c["state"]))
        else:
            hits.append((rel, c["state"]))
    return hits, stock


def render(rows, hits, stock, unknown):
    w = len([1 for _r, c in rows if c["write"]])
    out = []
    out.append("── " + GATE_NAME + " ──")
    out.append("取数面 : " + " ＋ ".join(SURFACES))
    out.append("现读   : 扫 %d 件 ｜ 有写面 %d 件 ｜ 命中 %d 件 ｜ 存量（棘轮内）%d 件"
               % (len(rows), w, len(hits), len(stock)))
    out.append("")
    out.append("每件读数（件 ｜ 有写面 ｜ 甲默认写盘 ｜ 乙认旗标仍写 ｜ 丙未知旗标 rc=0）：")
    if not rows:
        out.append("  （无）")
    for rel, c in rows:
        out.append("  %s ｜ 写面 %s ｜ 甲 %s ｜ 乙 %s ｜ 丙 %s" % (
            rel, "是" if c["write"] else "否",
            "是" if c["state"] == ST_A else "否",
            "是" if c["state"] == ST_B else "否",
            "是" if c["state"] == ST_C else "否"))
    out.append("")
    out.append("命中（新 · 棘轮表外）：")
    if not hits:
        out.append("  （无）")
    for rel, st in hits:
        out.append("  · %s ← %s" % (ST_LABEL[st], rel))
    out.append("")
    out.append("存量（棘轮内 · 只打印）：")
    if not stock:
        out.append("  （无）")
    for rel, st in stock:
        out.append("  · %s ← %s" % (ST_LABEL[st], rel))
    out.append("")
    out.append("档 : tri-report（只报告 · 命中 rc=1 落 REPORT · ✗ 不阻断）")
    if unknown:
        out.append("✗ 取数面拿不到：" + unknown + " ⇒ **不给结论**（rc=2）")
    return "\n".join(out)


def baseline_tsv(hits, stock):
    out = ["kind\tfile\tstate"]
    for rel, st in hits:
        out.append("new\t%s\t%s" % (rel, st))
    for rel, st in stock:
        out.append("stock\t%s\t%s" % (rel, st))
    return "\n".join(out)


# ── self-test（合成夹具 · 不碰真仓）────────────────────────────────────────────
def self_test():
    import shutil
    import tempfile
    checks = []

    def ck(name, got, want):
        checks.append((name, got, want, got == want))

    tmp = tempfile.mkdtemp(prefix="genflag-selftest-")
    try:
        os.makedirs(os.path.join(tmp, "tools"), exist_ok=True)
        os.makedirs(os.path.join(tmp, "scripts/docs"), exist_ok=True)
        # 甲：有写面 · 无旗标面
        _w(os.path.join(tmp, "tools/danger.py"),
           "def main():\n    open('x.md', 'w').write('a')\nmain()\n")
        # 丙：有写面 · 有手写旗标面 · 有 --help 但无未知拒执
        _w(os.path.join(tmp, "tools/half.py"),
           "import sys\nknown = {'--out'}\nif '--help' in sys.argv:\n    print('u')\n    sys.exit(0)\nopen('y.md', 'w').write('b')\n")
        # 乙：有写面 · 有手写旗标面 · 有未知拒执但无 --help
        _w(os.path.join(tmp, "tools/nohelp.py"),
           "import sys\nbad = [a for a in sys.argv[1:] if a.startswith('-') and a not in {'--mark'}]\nif bad:\n    print('✗ 未知参数: %s' % bad)\n    sys.exit(2)\nopen('z.md', 'w').write('c')\n")
        # 干净：argparse
        _w(os.path.join(tmp, "scripts/docs/gen-ok.py"),
           "import argparse\nap = argparse.ArgumentParser()\nap.add_argument('--out')\na = ap.parse_args()\nopen(a.out, 'w').write('d')\n")
        # 无写面（只 stdout）：不入写面族
        _w(os.path.join(tmp, "scripts/docs/gen-readonly.py"),
           "import sys\nprint('only stdout')\n")

        rows, errs = compute(tmp)
        ck("夹具·取数面无缺", errs, [])
        ck("夹具·扫件数", [r for r, _c in rows],
           ["scripts/docs/gen-ok.py", "scripts/docs/gen-readonly.py", "tools/danger.py",
            "tools/half.py", "tools/nohelp.py"])
        ck("夹具·有写面件数", len([1 for _r, c in rows if c["write"]]), 4)
        hits, stock = audit(rows)
        ck("夹具·命中件数", len(hits), 3)
        ck("夹具·命中态与件", hits,
           [("tools/danger.py", ST_A), ("tools/half.py", ST_C), ("tools/nohelp.py", ST_B)])
        ck("夹具·干净件无态", [c["state"] for r, c in rows if r == "scripts/docs/gen-ok.py"], [""])
        ck("夹具·只 stdout 件不入写面", [c["write"] for r, c in rows if r == "scripts/docs/gen-readonly.py"], [False])

        # 哑化证牙（负控）：把干净件的旗标面摘掉 ⇒ 必红（甲）
        _w(os.path.join(tmp, "scripts/docs/gen-ok.py"),
           "open('d.md', 'w').write('d')\n")
        rows2, _e2 = compute(tmp)
        hits2, _s2 = audit(rows2)
        ck("哑化·摘掉 argparse ⇒ 该件转甲", [c["state"] for r, c in rows2 if r == "scripts/docs/gen-ok.py"], [ST_A])
        ck("哑化·命中升到 4", len(hits2), 4)

        # 棘轮（正控）：把 danger.py 那一对收进基线 ⇒ 该命中转存量
        hits3, stock3 = audit(rows2, base_pairs=[("tools/danger.py", ST_A)])
        ck("棘轮·收进基线 ⇒ 命中 3", len(hits3), 3)
        ck("棘轮·存量 1", stock3, [("tools/danger.py", ST_A)])

        # 取数面拿不到（面缺）⇒ errs 非空（rc=2 档）
        _rows4, errs4 = compute(os.path.join(tmp, "没有此根"))
        ck("缺面 ⇒ 报错非空", errs4 != [], True)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    fails = [c for c in checks if not c[3]]
    for (name, got, want, ok) in checks:
        print("%s %s：got=%r want=%r" % ("✓" if ok else "✗", name, got, want))
    print("self-test：%d 格 · %d 失败" % (len(checks), len(fails)))
    return RC_OK if not fails else RC_REPORT


def _w(path, body):
    with open(path, "w", encoding="utf-8") as f:
        f.write(body)


def repo_root(arg_root):
    if arg_root:
        return os.path.abspath(arg_root)
    env = os.environ.get("ZERG_REPO_ROOT") or os.environ.get("ZERG_REPO")
    if env:
        return os.path.abspath(env)
    p = os.getcwd()
    while True:
        if os.path.isdir(os.path.join(p, ".git")):
            return p
        nxt = os.path.dirname(p)
        if nxt == p:
            return os.getcwd()
        p = nxt


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True, description=GATE_NAME)
    ap.add_argument("--check", action="store_true", help="只报告（tri-report 默认档）")
    ap.add_argument("--self-test", action="store_true", dest="self_test")
    ap.add_argument("--baseline", action="store_true", help="出基线表（TSV）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--repo-root", "--root", dest="root", default="")
    ap.add_argument("--version", action="store_true")
    args = ap.parse_args(argv)

    if args.version:
        print("%s %s" % (TOOL, VERSION))
        return RC_OK
    if args.self_test:
        return self_test()

    root = repo_root(args.root)
    rows, errs = compute(root)
    if errs:
        eprint("%s: %s" % (TOOL, " ｜ ".join(errs)))
        eprint("档 : tri-report · ✗ 取数面拿不到 ⇒ 不给结论（rc=2）")
        return RC_BLOCK
    hits, stock = audit(rows)
    if args.json:
        print(json.dumps({
            "schema": "zerg/genflag-surface/v1",
            "scanned": len(rows),
            "with_write": len([1 for _r, c in rows if c["write"]]),
            "hit_rows": len(hits), "stock_rows": len(stock),
            "hits": [{"file": r, "state": s} for r, s in hits],
            "stock": [{"file": r, "state": s} for r, s in stock],
        }, ensure_ascii=False, indent=1))
        return RC_REPORT if hits else RC_OK
    if args.baseline:
        print(baseline_tsv(hits, stock))
        return RC_REPORT if hits else RC_OK
    print(render(rows, hits, stock, ""))
    return RC_REPORT if hits else RC_OK


def eprint(*a):
    print(*a, file=sys.stderr)


if __name__ == "__main__":
    sys.exit(main())

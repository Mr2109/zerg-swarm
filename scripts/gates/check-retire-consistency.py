#!/usr/bin/env python3
# check-retire-consistency.py —— 退件 ↔ 登记/排除面 一致性机检（缺口 GAP-20261008-05）
#
# 义（缺口逐字）：凡 `retire_ledger` 已登记且**不在盘**的件，不得在任何登记/排除面留**悬空行**。
#
# 口径参照（✗ 另起语汇）：门75「戊 件面现读存在性」（Go 半 `core/cmd/zerg/check_gap_axis_reconcile.go`
#   的 gapAxisRecExistenceHits）—— 存在性 = **现读** `git -C <仓> ls-files -z`（无缓存）；命中词汇
#   「失效件 / 悬空行 / 存量豁免」逐字沿用；拿不到取数面 ⇒ rc=2 **不给结论**（✗ 不拿旧口径硬判）。
#
# 覆盖面（八面）：
#   ① scripts/公开标记.tsv                  ② publish/private-paths.txt
#   ③ scripts/gates/dangling-refs-registry.json
#   ④ 两器：scripts/build/publish-public.sh 的 EXCLUDES ＋ publish/mirror-public-lib.py 的
#           DROP_EXACT / DROP_PREFIX
#   ⑤ scripts/gates/gate-coverage.config.json   ⑥ scripts/gates/affected-map.json
#   ⑦ scripts/gates/judge-registry.json         ⑧ scripts/gates/slice-probes.json
#
# 档：**tri-report**（只报告·rc=1 落 REPORT·✗ 不阻断）；冷启动棘轮 = 现读悬空数（`--baseline` 出基线表）。
#   例外（有意保留者）走**白名单 ＋ 逐条理由**打印，✗ **不许静默跳过**。
#   rc=2 = 取数面拿不到（adopt-retire.json 读不了 / git ls-files 失败）⇒ **不给结论**。
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys

TOOL = "check-retire-consistency.py"
VERSION = "v1.0.0"
GATE_NAME = "退件↔登记/排除面 一致性（retire_ledger 已退且不在盘者不得留悬空行）"

LEDGER_REL = "core/internal/contract/adopt-retire.json"

RC_OK = 0
RC_REPORT = 1  # 有悬空行（tri-report ⇒ 落 REPORT，既不计失败项、也不影响退出码）
RC_BLOCK = 2   # 取数面拿不到 ⇒ 不给结论

# ── 例外白名单：有意保留的（surface_id, ref）对 + 逐条理由 ──────────────────────
# 口径：只登记「已退且不在盘、但**故意**留在某面上」的行；✗ 不许拿它掩盖真悬空。
# 条目形制：(surface_id, ref, reason) —— surface_id 见 SURFACES；ref 与被判面里逐字相同。
KEEPS = [
    # （首读现读：见 `--baseline` 输出。空 = 首读无「有意保留」项；只增不删。）
]

# ── 冷启动棘轮基线（存量表）：已退且不在盘、**首读即存在**的 (surface_id, ref) 对 ──────
# 口径：**首次现读**逐对写进本表 ⇒ 只打印、**不进退码**（= 存量）；表外的（=**新出现的**）才是「悬空」⇒ rc=1。
# **只增不删**。条目形制：(surface_id, ref) —— 两键全等才算存量。
# ★ 2026-10-08 首读现读（真账 142 退件 · `git ls-files` 现跑 · 悬空 13 行）：首读收成 3 对（已清理见下）。
BASELINE = [
    # ★ 2026-10-08 清理批（GAP-20261008-05）：首读收的 3 对（registry 9 条 + 两器 4 条）已逐面**摘行清空**
    #   ⇒ 棘轮基线**同步到新值 = 空**（照现读机制：表外的才是「悬空」；清完悬空/存量均 0）。
]

# ── 八面定义：id → (人读标签, [(仓相对路径, 抽取器名) ...]) ──────────────────────
# ★ 面 ④「两器」= 一个面两件（EXCLUDES ＋ DROP_EXACT/DROP_PREFIX），合并计数 ⇒ 恰八面。
SURFACES = [
    ("marks",    "公开标记.tsv",                  [("scripts/公开标记.tsv", "marks")]),
    ("private",  "private-paths.txt",             [("publish/private-paths.txt", "private")]),
    ("dangling", "dangling-refs-registry.json",   [("scripts/gates/dangling-refs-registry.json", "dangling")]),
    ("two",      "两器 EXCLUDES/DROP",            [("scripts/build/publish-public.sh", "excludes"),
                                                   ("publish/mirror-public-lib.py", "drop")]),
    ("coverage", "gate-coverage.config.json",     [("scripts/gates/gate-coverage.config.json", "coverage")]),
    ("affected", "affected-map.json",             [("scripts/gates/affected-map.json", "affected")]),
    ("judge",    "judge-registry.json",           [("scripts/gates/judge-registry.json", "judge")]),
    ("slice",    "slice-probes.json",             [("scripts/gates/slice-probes.json", "slice")]),
]


def eprint(*a):
    print(*a, file=sys.stderr)


def read_text(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        return f.read()


def strip_quoted(line):
    """取一行里首个双引号字符串；⾮ `"` 起头 ⇒ None（与 shell/python 数据表体例同形）。"""
    s = line.strip()
    if s.startswith("#"):
        return None
    m = re.match(r'"([^"]*)"', s)
    return m.group(1) if m else None


# ── 抽取器：每面返回「该面登记的仓相对引用」清单（去重前） ────────────────────────
def ex_marks(txt):
    out = []
    for ln in txt.split("\n"):
        if not ln.strip() or ln.lstrip().startswith("#"):
            continue
        cols = ln.split("\t")
        if cols and cols[0].strip():
            out.append(cols[0].strip())
    return out


def ex_private(txt):
    out = []
    for ln in txt.split("\n"):
        s = ln.strip()
        if not s or s.startswith("#"):
            continue
        out.append(s)
    return out


def ex_dangling(txt):
    j = json.loads(txt)
    return [e.get("file") for e in j.get("entries", []) if e.get("file")]


def _block(txt, header_re):
    m = re.search(header_re, txt)
    if not m:
        return ""
    i = m.end()
    depth = 1
    buf = []
    for ch in txt[i:]:
        if ch in "{([":
            depth += 1
        elif ch in "})]":
            depth -= 1
            if depth == 0:
                break
        buf.append(ch)
    return "".join(buf)


def ex_excludes(txt):
    body = _block(txt, r"EXCLUDES\s*=\s*\(")
    return [v for v in (strip_quoted(l) for l in body.split("\n")) if v]


def ex_drop(txt):
    out = []
    for hdr in (r"DROP_EXACT\s*=\s*\{", r"DROP_PREFIX\s*=\s*\("):
        body = _block(txt, hdr)
        out += [v for v in (strip_quoted(l) for l in body.split("\n")) if v]
    return out


def ex_coverage(txt):
    j = json.loads(txt)
    out = []
    out += list(j.get("scan", {}).get("script_globs", []))
    for k in ("build_dirs", "scripts"):
        out += [w.get("path") for w in j.get("whitelist", {}).get(k, []) if w.get("path")]
    ex = j.get("exclude", {})
    out += list(ex.get("path_components", []))
    out += list(ex.get("prefixes", []))
    return out


def ex_affected(txt):
    j = json.loads(txt)
    out = [f.get("pattern") for f in j.get("files", []) if f.get("pattern")]
    for vals in j.get("faces", {}).values():
        out += [v for v in vals if isinstance(v, str)]
    return out


def ex_judge(txt):
    j = json.loads(txt)
    return [e.get("path") for e in j.get("entries", []) if e.get("path")]


def _collect_strings(o, out):
    if isinstance(o, str):
        out.append(o)
    elif isinstance(o, list):
        for x in o:
            _collect_strings(x, out)
    elif isinstance(o, dict):
        for v in o.values():
            _collect_strings(v, out)


def ex_slice(txt):
    j = json.loads(txt)
    out = []
    for p in j.get("probes", []):
        _collect_strings(p.get("slice"), out)
    return out


EXTRACTORS = {
    "marks": ex_marks, "private": ex_private, "dangling": ex_dangling,
    "excludes": ex_excludes, "drop": ex_drop, "coverage": ex_coverage,
    "affected": ex_affected, "judge": ex_judge, "slice": ex_slice,
}


# ── 取数面 ───────────────────────────────────────────────────────────────────
def repo_root(arg_root):
    if arg_root:
        return os.path.abspath(arg_root)
    env = os.environ.get("ZERG_REPO_ROOT") or os.environ.get("ZERG_REPO")
    if env:
        return os.path.abspath(env)
    # 从 cwd 上溯找稳定锚（与同族 repoRoot 口径：本件 py 不能当锚，用 .git）
    p = os.getcwd()
    while True:
        if os.path.isdir(os.path.join(p, ".git")):
            return p
        nxt = os.path.dirname(p)
        if nxt == p:
            return os.getcwd()
        p = nxt


def git_tracked(root):
    try:
        r = subprocess.run(
            ["git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z"],
            capture_output=True, text=True, check=False,
        )
    except OSError as e:
        return None, f"git 调不起来：{e}"
    if r.returncode != 0:
        return None, f"git ls-files 失败（rc={r.returncode}）：{r.stderr.strip()}"
    return set(x for x in r.stdout.split("\0") if x), ""


def load_ledger(root):
    path = os.path.join(root, LEDGER_REL)
    if not os.path.exists(path):
        return None, None, f"账本不在盘：{LEDGER_REL}"
    try:
        j = json.loads(read_text(path))
    except Exception as e:  # noqa: BLE001
        return None, None, f"账本解析失败：{e}"
    retired = [e.get("object") for e in j.get("retire_ledger", []) if e.get("object")]
    exceptions = [e.get("object") for e in j.get("retire_exceptions", []) if e.get("object")]
    return retired, exceptions, ""


def is_dangling(ref, absent):
    if ref in absent:
        return True
    if ref.endswith("/") and any(a.startswith(ref) for a in absent):
        return True
    return False


# ── 判定（纯函数：可被 self-test 直接喂合成数据）──────────────────────────────
def compute(root, tracked, retired, exceptions, base_pairs=None):
    absent = sorted({p for p in retired if p not in tracked})
    exempt_set = set(exceptions)
    keeps = {(s, r) for (s, r, _) in KEEPS}
    base = set(BASELINE if base_pairs is None else base_pairs)

    per_surface = []          # [(id, label, rel_joined, read_err, [ (ref, kind) ...])]
    dangling_rows = []        # [(surface_id, label, ref, kind)] —— 新悬空（⇒ rc=1）
    stock_rows = []           # 存量（棘轮基线内 · 不进退码）
    exempt_rows = []          # [(surface_id, ref, reason)]
    for sid, label, parts in SURFACES:
        err = ""
        refs = []
        rels = []
        for rel, exname in parts:
            rels.append(rel)
            path = os.path.join(root, rel)
            if not os.path.exists(path):
                err = err or ("面不在盘：" + rel)
                continue
            try:
                refs += EXTRACTORS[exname](read_text(path))
            except Exception as e:  # noqa: BLE001
                err = err or ("解析失败：%s（%s）" % (e, rel))
        rows = []
        if not err:
            for ref in refs:
                if not is_dangling(ref, absent):
                    continue
                if ref in exempt_set:
                    rows.append((ref, "例外·退场例外名册"))
                    exempt_rows.append((sid, ref, "retire_exceptions 名册（有意保留）"))
                elif (sid, ref) in keeps:
                    rows.append((ref, "例外·白名单"))
                    exempt_rows.append((sid, ref, _keep_reason(sid, ref)))
                elif (sid, ref) in base:
                    rows.append((ref, "存量"))
                    stock_rows.append((sid, label, ref))
                else:
                    rows.append((ref, "悬空"))
                    dangling_rows.append((sid, label, ref, "悬空"))
        per_surface.append((sid, label, " ＋ ".join(rels), err, rows))
    return {
        "retired_n": len(retired), "exceptions_n": len(exceptions),
        "absent": absent, "tracked_n": len(tracked),
        "per_surface": per_surface, "dangling": dangling_rows,
        "stock": stock_rows, "exempt": exempt_rows,
    }


def _keep_reason(sid, ref):
    for (s, r, why) in KEEPS:
        if s == sid and r == ref:
            return why
    return "白名单（未写明理由）"


# ── 渲染 ─────────────────────────────────────────────────────────────────────
def render(rep, unknown):
    out = []
    out.append("── " + GATE_NAME + " ──")
    out.append("取数面 : " + LEDGER_REL)
    out.append("现读   : git ls-files 共 %d 件 ｜ retire_ledger %d 条 ｜ retire_exceptions %d 条 ｜ 已退且不在盘 %d 件"
               % (rep["tracked_n"], rep["retired_n"], rep["exceptions_n"], len(rep["absent"])))
    out.append("")
    total_new = 0
    total_stock = 0
    for (sid, label, rel, err, rows) in rep["per_surface"]:
        dang = [r for (r, k) in rows if k == "悬空"]
        stock = [r for (r, k) in rows if k == "存量"]
        exc = [r for (r, k) in rows if k not in ("悬空", "存量")]
        total_new += len(dang)
        total_stock += len(stock)
        mark = "✗" if dang else ("?" if err else "✓")
        note = ("  [" + err + "]") if err else ""
        out.append("  %s 面 %-24s 悬空 %d ｜ 存量 %d ｜ 例外 %d%s"
                   % (mark, label, len(dang), len(stock), len(exc), note))
        for ref in dang:
            out.append("        · 新悬空 ← %s" % ref)
        for ref in stock:
            out.append("        · 存量 ← %s" % ref)
        for ref in exc:
            out.append("        · 例外 ← %s" % ref)
    out.append("")
    out.append("合计 : 新悬空 %d 行 ｜ 存量（棘轮内）%d 行 ｜ 例外 %d 行" % (total_new, total_stock, len(rep["exempt"])))
    if rep["exempt"]:
        out.append("")
        out.append("例外/白名单（逐条理由 · ✗ 非静默跳过）：")
        for (sid, ref, why) in rep["exempt"]:
            out.append("  · [%s] %s —— %s" % (sid, ref, why))
    out.append("")
    out.append("档 : tri-report（只报告 · 新悬空 rc=1 落 REPORT · ✗ 不阻断）")
    if unknown:
        out.append("✗ 取数面拿不到：" + unknown + " ⇒ **不给结论**（rc=2）")
    return "\n".join(out)


def baseline_tsv(rep):
    out = ["surface\tpath\tnew_dangling\tstock\texempt"]
    for (sid, label, rel, err, rows) in rep["per_surface"]:
        dang = len([r for (r, k) in rows if k == "悬空"])
        stock = len([r for (r, k) in rows if k == "存量"])
        exc = len(rows) - dang - stock
        out.append("%s\t%s\t%d\t%d\t%d" % (label, rel, dang, stock, exc))
    out.append("TOTAL\t_\t%d\t%d\t%d" % (len(rep["dangling"]), len(rep["stock"]), len(rep["exempt"])))
    return "\n".join(out)


# ── self-test（合成夹具 · 不碰真仓）───────────────────────────────────────────
def self_test():
    import tempfile
    import shutil
    checks = []

    def ck(name, got, want):
        checks.append((name, got, want, got == want))

    tmp = tempfile.mkdtemp(prefix="retire-consistency-selftest-")
    try:
        os.makedirs(os.path.join(tmp, "core/internal/contract"), exist_ok=True)
        os.makedirs(os.path.join(tmp, "scripts/gates"), exist_ok=True)
        os.makedirs(os.path.join(tmp, "publish"), exist_ok=True)
        os.makedirs(os.path.join(tmp, "scripts/build"), exist_ok=True)
        # 账本：GONE 已退且不在盘；GONE2 已退但仍在盘；LIVE 在盘
        ledger = {
            "retire_ledger": [
                {"object": "scripts/x/GONE.py", "state": "已删"},
                {"object": "scripts/x/GONE2.py", "state": "已删"},
            ],
            "retire_exceptions": [],
        }
        with open(os.path.join(tmp, LEDGER_REL), "w", encoding="utf-8") as f:
            json.dump(ledger, f)
        # GONE2 真在盘（git add 后进 tracked）
        os.makedirs(os.path.join(tmp, "scripts/x"), exist_ok=True)
        open(os.path.join(tmp, "scripts/x/GONE2.py"), "w").close()
        open(os.path.join(tmp, "scripts/x/LIVE.py"), "w").close()
        # 面 ①：一行悬空（GONE.py）+ 一行注释（不算）
        with open(os.path.join(tmp, "scripts/公开标记.tsv"), "w", encoding="utf-8") as f:
            f.write("文件\tpublic\t理由\nscripts/x/GONE.py\tno\t测试\n# scripts/x/GONE.py 注释不算\n")
        # 面 ②：GONE.py
        with open(os.path.join(tmp, "publish/private-paths.txt"), "w", encoding="utf-8") as f:
            f.write("# c\nscripts/x/GONE.py\n")
        # 面 ④：两器
        with open(os.path.join(tmp, "scripts/build/publish-public.sh"), "w", encoding="utf-8") as f:
            f.write('EXCLUDES=(\n  "scripts/x/GONE.py"\n  # "scripts/x/GONE.py" 注释\n)\n')
        with open(os.path.join(tmp, "publish/mirror-public-lib.py"), "w", encoding="utf-8") as f:
            f.write('DROP_EXACT = {\n    "scripts/x/GONE.py",\n}\nDROP_PREFIX = (\n    "scripts/x/",\n)\n')
        # 其余面：空
        for rel, body in [
            ("scripts/gates/dangling-refs-registry.json", '{"entries":[]}'),
            ("scripts/gates/gate-coverage.config.json", '{"scan":{"script_globs":[]},"exclude":{},"whitelist":{}}'),
            ("scripts/gates/affected-map.json", '{"files":[],"faces":{}}'),
            ("scripts/gates/judge-registry.json", '{"entries":[]}'),
            ("scripts/gates/slice-probes.json", '{"probes":[]}'),
        ]:
            with open(os.path.join(tmp, rel), "w", encoding="utf-8") as f:
                f.write(body)
        # 真 git 定存在性（ls-files 只需 add）
        subprocess.run(["git", "init", "-q", tmp], check=True)
        subprocess.run(["git", "-C", tmp, "add", "-A"], check=True)
        tracked, err = git_tracked(tmp)
        ck("selftest·git 可取数", err, "")
        ret, exc, lerr = load_ledger(tmp)
        ck("selftest·账本可读", lerr, "")
        rep = compute(tmp, tracked, ret, exc)
        ck("selftest·absent 只 GONE.py", rep["absent"], ["scripts/x/GONE.py"])
        # 悬空行：marks 1 + private 1 + 两器 EXCLUDES 1 + 两器 DROP_EXACT 1 + DROP_PREFIX 命中 1 = 5
        ck("selftest·新悬空行数（含 DROP_PREFIX 前缀命中）", len(rep["dangling"]), 5)
        ck("selftest·例外行数", len(rep["exempt"]), 0)
        # 取数面拿不到（缺账本）⇒ lerr 非空（rc=2 档 · 防「静默当空账」）
        _, _, lerr2 = load_ledger(os.path.join(tmp, "不存在的根"))
        ck("selftest·缺账本 ⇒ 报错非空", lerr2 != "", True)
        # 棘轮：把 marks 那一对收进基线 ⇒ 该行转「存量」、新悬空 4
        rep_b = compute(tmp, tracked, ret, exc, base_pairs=[("marks", "scripts/x/GONE.py")])
        ck("selftest·棘轮命中 ⇒ 新悬空 4", len(rep_b["dangling"]), 4)
        ck("selftest·棘轮命中 ⇒ 存量 1", len(rep_b["stock"]), 1)
        # 负控：把 GONE.py 造回盘 ⇒ 悬空归 0
        open(os.path.join(tmp, "scripts/x/GONE.py"), "w").close()
        subprocess.run(["git", "-C", tmp, "add", "-A"], check=True)
        tracked2, _ = git_tracked(tmp)
        rep2 = compute(tmp, tracked2, ret, exc)
        ck("负控·件造回盘 ⇒ 悬空 0", len(rep2["dangling"]), 0)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    fails = [c for c in checks if not c[3]]
    for (name, got, want, ok) in checks:
        print("%s %s：got=%r want=%r" % ("✓" if ok else "✗", name, got, want))
    print("self-test：%d 格 · %d 失败" % (len(checks), len(fails)))
    return RC_OK if not fails else RC_REPORT


# ── main ─────────────────────────────────────────────────────────────────────
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
    tracked, gerr = git_tracked(root)
    ret, exc, lerr = load_ledger(root)
    unknown = ""
    if gerr:
        unknown = gerr
    elif lerr:
        unknown = lerr
    if unknown:
        eprint("%s: %s" % (TOOL, unknown))
        eprint("档 : tri-report · ✗ 取数面拿不到 ⇒ 不给结论（rc=2）")
        return RC_BLOCK

    rep = compute(root, tracked, ret, exc)
    rep["tracked_n"] = len(tracked)
    if args.json:
        print(json.dumps({
            "tracked": rep["tracked_n"], "retired": rep["retired_n"],
            "exceptions": rep["exceptions_n"], "absent": rep["absent"],
            "dangling_rows": len(rep["dangling"]),
            "dangling": [{"surface": s, "ref": r} for (s, _l, r, _k) in rep["dangling"]],
            "stock_rows": len(rep["stock"]),
            "stock": [{"surface": s, "ref": r} for (s, _l, r) in rep["stock"]],
            "exempt": [{"surface": s, "ref": r, "reason": w} for (s, r, w) in rep["exempt"]],
            "surface_counts": {sid: len([r for (r, k) in rows if k in ("悬空", "存量")])
                               for (sid, _l, _p, _e, rows) in rep["per_surface"]},
            "surface_new": {sid: len([r for (r, k) in rows if k == "悬空"])
                            for (sid, _l, _p, _e, rows) in rep["per_surface"]},
        }, ensure_ascii=False, indent=1))
        return RC_REPORT if rep["dangling"] else RC_OK
    if args.baseline:
        print(baseline_tsv(rep))
        return RC_REPORT if rep["dangling"] else RC_OK
    print(render(rep, ""))
    return RC_REPORT if rep["dangling"] else RC_OK


if __name__ == "__main__":
    sys.exit(main())

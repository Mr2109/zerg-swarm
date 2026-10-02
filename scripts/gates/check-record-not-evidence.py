#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-record-not-evidence.py — 元门：记录面不得充作证据（R1/R2/R3 三闭环 · 只报告起步）。

为什么有它（缺口 `GAP-20260927-141`）
------------------------------------
「拿台账当证据」是**一整类**结构病而非单张表：引用面 / 扫描面一旦把「路径索引型
表格」（名册 · 标记表 · 清单 · 台账 · registry）算作**引用**，`x ∈ S` 这种
「成员即存在」就永远为真，`未登记 N` 这一档**结构上不可能红** ⇒ 记录把被观测事实
抹掉（写下这条债这个动作本身就把债抹掉了）。

本件只报三类闭环（**只报告 · 不接闸起步**；档位 `tri-report`）：
  R1 成员即存在  —— 记录面 `S` 里列了成员 `x`，而 `x` 在**记录面之外** 0 处独立现读。
  R2 表当引用    —— 某扫描 / 引用面把「记录面」当引用源（**命中即算引用**）。
  R3 记录即已修  —— 记录行 / 数字 / 一句话被当成「已登记 · 已修 · 已闭」，该行无任何
                    字段对盘（无 sha / commit / rc / 对拍 / 现跑命令）。

两条**不误报**排除：
  · 对拍豁免：记录面每一条成员都能在盘上逐条命中（且带 sha256 时逐条对上）⇒ 不报 R1/R3。
  · 真源豁免：表头自我声明「唯一真源 / 单一来源」且漂移另有门**双向**管 ⇒ 不报 R2。

记录面判别 `is_record_surface_v2` = **名**（弱：件名 / 路径含名册·标记·清单·台账·registry 一类词）
+ **形**（≥5 数据行 · 首字段多为**仓内路径**）+ **用**（被代码读去做「名字成员判定」
且不回盘）。三条都是**启发式**（只报告档足够），不是完备判定 ⇒ 结论面逐条列出
「命中件:行 + 依据」，供人复核；升级到阻断档前必须先人工把命中逐条三分类。

用法：
  python3 scripts/gates/check-record-not-evidence.py [--root <仓根>] [--scan-dir <目录>…]
                                                     [--tables <件>…] [--json]
  python3 scripts/gates/check-record-not-evidence.py --self-test
退码：0 = 未命中 · 1 = 有命中（只报告） · 2 = 判不了（不给结论）
"""

import argparse
import hashlib
import json
import os
import re
import sys

REPO_PREFIXES = ("scripts/", "core/", "agent/", "ui/", "docs/", "publish/",
                 "mcp/", "gateway/", "tools/", "wall/", "shared/", "dist/")
FILE_EXT = (".py", ".sh", ".go", ".md", ".json", ".jsonl", ".tsv", ".csv",
            ".yml", ".yaml", ".rs", ".txt", ".toml", ".js", ".ts")
TEXT_EXT = set(FILE_EXT)
NAME_LEXICON = ("台账", "登记表", "登记", "名册", "清单", "标记表", "标记", "索引",
                "roster", "registry", "inventory", "ledger", "marks", "manifest",
                "index", "whitelist", "allow")
SKIP_DIRS = {".git", "__pycache__", "vendor", "node_modules", "target",
             "dist", "build", "bin", ".venv", "venv", ".mypy_cache"}

# R2：读表 + 扫描 / 引用构造的痕迹（启发式）
SCAN_HINT = re.compile(r"\b(glob|iglob|rglob|os\.walk|listdir|ls-files|walk|read_text|open\s*\()")
REF_HINT = re.compile(r"\b(in|not\s+in)\b|\.get\s*\(|contains\s*\(|append\s*\(")
# R3：记录里断言「已完成」的词 + 「对盘字段」的词
DONE_RE = re.compile(r"已修|已闭|已解|已登记|已审|已销|已清|关闭|fixed|closed|resolved|done")
RECON_RE = re.compile(r"sha256|sha16|sha1|commit|rc=|rc\b|对拍|diff|命令|cmd|--|回读|现跑|verify")
# 真源豁免：自我声明唯一真源 + 双向管
SRC_DECL_RE = re.compile(r"唯一真源|单一真源|唯一来源|单一来源|唯一来源件|唯一真源件")
BIDIR_RE = re.compile(r"双向|差集|对拍")
STATUS_RE_NONE = None  # 占位（保持可读）


def is_repo_path(tok):
    """这一格像不像「仓内路径」（形判据用）。"""
    t = tok.strip().strip('`"\'').rstrip(",")
    if not t or " " in t or len(t) < 3:
        return False
    if t.startswith(REPO_PREFIXES):
        return True
    return ("/" in t) and t.endswith(FILE_EXT)


def name_hit(path):
    """名（弱）：件名 / 路径里带名册·标记·清单·台账·registry 一类词。"""
    low = path.lower()
    return any(k.lower() in low for k in NAME_LEXICON)


def text_files(root, dirs):
    """遍历 scan 目录，返回 {相对路径: 文本}（跳过二进制 / 大件 / 忽略目录）。"""
    out = {}
    for d in dirs:
        base = os.path.join(root, d)
        if not os.path.isdir(base):
            continue
        for dp, dns, fns in os.walk(base):
            dns[:] = [x for x in dns if x not in SKIP_DIRS]
            for fn in fns:
                ext = os.path.splitext(fn)[1]
                if ext not in TEXT_EXT:
                    continue
                ap = os.path.join(dp, fn)
                try:
                    if os.path.getsize(ap) > 2 * 1024 * 1024:
                        continue
                    with open(ap, encoding="utf-8", errors="replace") as fh:
                        out[os.path.relpath(ap, root)] = fh.read()
                except OSError:
                    continue
    return out


def read_table(text):
    """返回 (header_line, rows) —— rows = [(首字段, 原行, [格…]), …]。

    `header` 收三类行：整行注释（`#` / `//`，真源声明常写在这里）+ 首个非路径行。
    """
    lines = [l for l in text.split("\n") if l.strip()]
    comments = [l for l in lines if l.strip().startswith(("#", "//"))]
    header, rows = None, []
    for i, l in enumerate(lines):
        s = l.strip()
        if s.startswith("#") or s.startswith("//"):
            continue
        if re.match(r"^[|\-:\s]+$", s):          # md 分隔行
            continue
        if s.count("|") >= 2:
            cells = [c.strip() for c in s.strip("|").split("|")]
        elif "\t" in l:
            cells = [c.strip() for c in l.split("\t")]
        elif "," in l and l.count(",") >= 2:
            cells = [c.strip() for c in l.split(",")]
        else:
            cells = s.split()
        if not cells:
            continue
        first = cells[0].strip().strip('`"\'')
        if header is None and not is_repo_path(first):
            header = l
            continue
        rows.append((first, l, cells))
    head_all = "\n".join(comments + ([header] if header else []))
    return head_all, rows


def analyze(root, dirs, tables=None):
    files = text_files(root, dirs)
    findings = []
    if tables:
        surfaces = list(tables)
    else:
        surfaces = []
        for rel, text in files.items():
            if os.path.splitext(rel)[1] not in (".tsv", ".csv", ".md", ".jsonl", ".txt", ".json"):
                continue
            if not name_hit(rel):
                continue
            header, rows = read_table(text)
            paths = [r[0] for r in rows if is_repo_path(r[0])]
            if len(rows) >= 5 and len(paths) * 2 >= len(rows):
                surfaces.append(rel)

    for rel in sorted(set(surfaces)):
        ap = os.path.join(root, rel)
        if not os.path.isfile(ap):
            findings.append({"rule": "R?", "where": rel + ":0",
                             "why": "声明的记录面不在盘上（判不了）"})
            continue
        text = files.get(rel)
        if text is None:
            try:
                text = open(ap, encoding="utf-8", errors="replace").read()
            except OSError:
                continue
        header, rows = read_table(text)
        members = [r for r in rows if is_repo_path(r[0])]
        if len(rows) < 5 or len(members) * 2 < len(rows):
            continue

        head_text = (header or "") + "\n" + "\n".join(l for _, l, _ in rows[:3])
        src_exempt = bool(SRC_DECL_RE.search(head_text)) and bool(BIDIR_RE.search(head_text))

        # 对拍豁免：每条成员都在盘上命中（带 sha256 时逐条对上）
        pair_exempt = bool(members)
        for first, raw, cells in members:
            ap2 = os.path.join(root, first)
            if not os.path.exists(ap2):
                pair_exempt = False
                break
            want = None
            for c in cells:
                if re.fullmatch(r"[0-9a-f]{64}", c.strip()):
                    want = c.strip()
                    break
            if want and os.path.isfile(ap2):
                try:
                    got = hashlib.sha256(open(ap2, "rb").read()).hexdigest()
                except OSError:
                    got = None
                if got != want:
                    pair_exempt = False
                    break

        # R1 成员即存在：成员在记录面之外 0 处独立现读
        if not pair_exempt:
            for first, raw, _ in members:
                indep = 0
                for other, otext in files.items():
                    if other == rel:
                        continue
                    if first in otext:
                        indep += 1
                if indep == 0:
                    findings.append({"rule": "R1", "where": rel + ":1",
                                     "why": "成员即存在却无独立现读：`%s` 在记录面外 0 处" % first})

        # R2 表当引用：某代码面把记录面当引用源（命中即算引用）
        if not src_exempt:
            for other, otext in files.items():
                if other == rel or os.path.splitext(other)[1] not in (".py", ".sh", ".go"):
                    continue
                base = os.path.basename(rel)
                if base not in otext and rel not in otext:
                    continue
                if SCAN_HINT.search(otext) and REF_HINT.search(otext):
                    findings.append({"rule": "R2", "where": other + ":1",
                                     "why": "引用 / 扫描面含记录面 `%s`（命中即算引用）" % rel})

        # R3 记录即已修：断言「完成」而无对盘字段
        if not pair_exempt:
            for first, raw, _ in rows:
                if DONE_RE.search(raw) and not RECON_RE.search(raw):
                    findings.append({"rule": "R3", "where": rel + ":1",
                                     "why": "记录即已修（无对盘字段）：`%s`" % raw.strip()[:60]})

    meta = {"root": root, "scan_dirs": list(dirs), "surfaces": sorted(set(surfaces)),
            "findings": len(findings)}
    return findings, meta


def report(findings, meta, as_json):
    if as_json:
        print(json.dumps({"meta": meta, "findings": findings}, ensure_ascii=False, indent=1))
        return
    print("── check-record-not-evidence：记录面不得充作证据（R1/R2/R3 · 只报告）──")
    print("  扫描面：%s（%d 个记录面候选）" % (" · ".join(meta["scan_dirs"]), len(meta["surfaces"])))
    for s in meta["surfaces"]:
        print("   · 记录面：%s" % s)
    for r in ("R1", "R2", "R3"):
        hits = [f for f in findings if f["rule"] == r]
        print("  【%s】命中 %d 条" % (r, len(hits)))
        for h in hits[:20]:
            print("    · %s  %s" % (h["where"], h["why"]))
        if len(hits) > 20:
            print("    … 另 %d 条（--json 取全量）" % (len(hits) - 20))
    print("⇒ %s（rc=%d）" % ("有命中 · 只报告（不阻断）" if findings else "未命中",
                           1 if findings else 0))


def _mk(path, content):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(content)


def self_test():
    import shutil
    import tempfile
    base = tempfile.mkdtemp(prefix="record-not-evidence-selftest-")
    fails = []
    checks = []

    def case(label, got, want):
        ok = (got == want)
        checks.append((label, ok, got, want))
        if not ok:
            fails.append(label)
        print("  %s %s ⇒ 得 %r（要 %r）" % ("✓" if ok else "✗", label, got, want))

    try:
        # ① must-fail：喂表必报（成员不在盘上 + 有代码面把表当引用源）
        d = os.path.join(base, "feed")
        _mk(os.path.join(d, "scripts", "名册.tsv"),
            "件\t说明\n" + "".join("scripts/ghost%d.py\tghost zero-ref\n" % i for i in range(6)))
        _mk(os.path.join(d, "scripts", "scan.py"),
            "import glob, os\nfor p in glob.glob('scripts/*'):\n"
            "    if os.path.basename(p) in open('scripts/名册.tsv').read():\n"
            "        print('counted as referenced')\n")
        fs, _ = analyze(d, ["scripts"])
        case("① must-fail · 喂表必报（R1 或 R2 命中 ≥1）", len(fs) >= 1, True)

        # ② must-not-report：双向对拍（每条都在盘上 + sha 对上 + 表头声明唯一真源 + 双向）
        d2 = os.path.join(base, "reconcile")
        rows = []
        for i in range(5):
            p = "scripts/real%d.py" % i
            _mk(os.path.join(d2, p), "# real %d\n" % i)
            h = hashlib.sha256(open(os.path.join(d2, p), "rb").read()).hexdigest()
            rows.append("%s\t%s" % (p, h))
        _mk(os.path.join(d2, "scripts", "清单.tsv"),
            "# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-x.py 双向差集管\n"
            "件\tsha256\n" + "\n".join(rows) + "\n")
        _mk(os.path.join(d2, "scripts", "check-x.py"),
            "import glob\nnames = open('scripts/清单.tsv').read()\n"
            "for p in glob.glob('scripts/*'):\n    assert p in names\n")
        fs2, _ = analyze(d2, ["scripts"])
        case("② must-not-report · 双向对拍 + 真源 + 对拍豁免 ⇒ 0 命中", len(fs2), 0)

        # ③ must-fail：R3 记录即已修（断言完成 · 无对盘字段）
        d3 = os.path.join(base, "done")
        _mk(os.path.join(d3, "scripts", "台账.tsv"),
            "件\t状态\n" + "".join("scripts/ghost%d.py\t已修\n" % i for i in range(6)))
        fs3, _ = analyze(d3, ["scripts"])
        case("③ must-fail · 记录即已修（R3 命中 ≥1）",
             any(f["rule"] == "R3" for f in fs3), True)

        # ④ 真源豁免：声明唯一真源 + 双向 ⇒ 不报 R2（即使有扫描面读它）
        d4 = os.path.join(base, "src")
        _mk(os.path.join(d4, "scripts", "registry.tsv"),
            "# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-y.py 双向差集管\n"
            "件\t说明\n" + "".join("scripts/ghost%d.py\tg\n" % i for i in range(6)))
        _mk(os.path.join(d4, "scripts", "check-y.py"),
            "import glob\nS = set(open('scripts/registry.tsv').read().split())\n"
            "for p in glob.glob('scripts/*'):\n    print(p in S)\n")
        fs4, _ = analyze(d4, ["scripts"])
        case("④ 真源豁免 ⇒ 不报 R2", any(f["rule"] == "R2" for f in fs4), False)
    finally:
        shutil.rmtree(base, ignore_errors=True)

    total = len(checks)
    nf = len(fails)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if nf == 0 else "不过", total, nf))
    return 0 if nf == 0 else 1


def repo_root():
    here = os.path.abspath(__file__)
    return os.path.dirname(os.path.dirname(os.path.dirname(here)))


def main(argv):
    ap = argparse.ArgumentParser(description="记录面不得充作证据（R1/R2/R3 元门 · 只报告）")
    ap.add_argument("--root", default=None, help="仓根（默认从本件位置上溯三层）")
    ap.add_argument("--scan-dir", action="append", default=None,
                    help="扫描目录（可重复；默认 scripts + core/cmd/zerg）")
    ap.add_argument("--tables", nargs="*", default=None,
                    help="显式指定记录面件（相对仓根；给了就不自动发现）")
    ap.add_argument("--json", action="store_true", help="机器可读输出")
    ap.add_argument("--self-test", action="store_true", help="合成夹具成对负控（不碰真目标）")
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    root = a.root or repo_root()
    if not os.path.isdir(root):
        print("⛔ 仓根不在：%s ⇒ 不给结论（rc=2）" % root)
        return 2
    dirs = a.scan_dir or ["scripts", "core/cmd/zerg"]
    findings, meta = analyze(root, dirs, a.tables)
    report(findings, meta, a.json)
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

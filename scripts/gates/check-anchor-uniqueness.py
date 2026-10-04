#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-anchor-uniqueness.py —— **内容锚唯一性自检**（组4 §二.4 `W-45` · `研-禁:130` `R-17` · `研-禁:133` `R-20`）

真源与口径（**判据绑私有面** · 逐字引文全指向仓外 `Zerg-内部文档/`）
------------------------------------------------------------
· 施工条（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md`
  §一 序 127 —— 事项 = 「引用一律内容锚（逐字句、可 `grep -F`，行号只作导航）
  **＋ 锚 0 命中 / 多命中必报**；编号空间显式带**类型前缀**（吃过 `R-xx` vs `Rxx` 的亏）」
  判据栏 = 「各表有内容锚列（可 `grep -F` 逐字命中）；锚 0 命中 / 多命中 ⇒ **必报**
  （负控：造一多命中锚 ⇒ 判红）」
· 已判过的那一句（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/承接自-v2.5.11/开工记录-批A-20260920.md:2117`
  —— 「**行号锚点本身就是病**」（本件 = 把那一句落成**能跑的**判据）
· 登记件（成文面 · 逐字对拍）：`Zerg-内部文档/项目文档/v2.5.12/登记-内容锚-v2.5.12-20260924.md`

本件判什么（三格 · 每格能单独失败）
----------------------------------
  ① **锚号形态**：`锚号` 必须带**类型前缀**（正则 `^[A-Z]{1,3}-\\d+` —— 前缀是类型，
     不是装饰：`R-17` 与 `R17` 是两个号空间的教训写在这一格）；不合形态 ⇒ 红 + 点名。
  ② **锚唯一性**：每条内容锚在其**目标件**里 `grep -F` 的**命中行数**必须 **恰好 1**。
     `0` ⇒ 红（「锚 0 命中」= 锚指向的东西已经不在那儿了）；`≥2` ⇒ 红（多命中 = 锚不唯一，
     指过去也定不到一处）—— 两个方向**都报**，且逐条给**命中行号**。
  ③ **锚号唯一**：同一条登记件里同号两行 ⇒ 红（一个号指两处 = 号空间被污染）。

**行号只作导航**（本件自己的口径）：判据吃的是**逐字句**，本件输出的行号只是**报告用**的
指路（人不看行号，机器也不拿它比）—— 这就是 `R-17` 那一句「行号只作导航」的落点。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 每条锚都唯一、每个锚号都带前缀、无重号
  1 = 有红（0 命中 / 多命中 / 锚号缺前缀 / 重号 / 目标件位形态错）—— 逐条点名
  2 = **不给结论**：登记件不在盘 / 登记 0 条（空转）/ 目标件不在盘 / 自检不过

用法
----
    python3 scripts/gates/check-anchor-uniqueness.py                 # 判（读仓外登记件 + 逐条现扫目标件）
    python3 scripts/gates/check-anchor-uniqueness.py --json           # 机读（zerg/v1 形状）
    python3 scripts/gates/check-anchor-uniqueness.py --register PATH  # 换登记件
    python3 scripts/gates/check-anchor-uniqueness.py --list-rules     # 打判据 / 形态 / 现读
    python3 scripts/gates/check-anchor-uniqueness.py --self-test      # 只跑自检（合成夹具 · 不碰真件）

★ 本笔**不挂门**（件头如实）：`scripts/gates/precommit-gates.sh` 不在本波件面里 ⇒ 本件今天
  只作**单跑档**；要挂门是**另一笔**的活（连带刷新 `add_step` 自检计数）。
"""

import argparse
import collections
import json
import os
import re
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-anchor-uniqueness.py v1.0.0"

REGISTER_REL = "项目文档/v2.5.12/登记-内容锚-v2.5.12-20260924.md"

# ── 锚号形态（**类型前缀是判据的一部分**）────────────────────────────────────────
ANCHOR_RE = re.compile(r"^[A-Z]{1,3}-\d+(?:-\d+)*$")
PREFIX_RE = re.compile(r"^([A-Z]{1,3})-")

# ── 目标件位前缀（两棵树的显式区分 · 不靠猜）────────────────────────────────────
DOCS_PREFIX = "@docs/"
REPO_PREFIX = "@repo/"


def cells(line):
    """markdown 表行 ⇒ 单元；`\\|`（转义竖线）不算分隔符。"""
    s = line.strip()
    if not (s.startswith("|") and s.endswith("|")):
        return None
    return [c.strip() for c in re.split(r"(?<!\\)\|", s[1:-1])]


def bare(s):
    return re.sub(r"[`*]", "", s.strip()).strip()


def parse_register(path):
    """读登记件 ⇒ (行表, 问题)。行表 = [ {anchor, target, text, note, line} ]"""
    if not os.path.isfile(path):
        return [], ["登记件不在盘：%s" % path]
    with open(path, encoding="utf-8") as f:
        lines = f.read().splitlines()
    rows = []
    for i, ln in enumerate(lines, 1):
        cs = cells(ln)
        if not cs or len(cs) < 3:
            continue
        anchor, target, text = bare(cs[0]), bare(cs[1]), cs[2].strip()
        if target.startswith(DOCS_PREFIX) or target.startswith(REPO_PREFIX) or ANCHOR_RE.match(anchor):
            rows.append({"anchor": anchor, "target": target, "text": text,
                         "note": bare(cs[3]) if len(cs) > 3 else "", "line": i})
    return rows, []


def resolve(root, docs, target):
    """`@docs/<rel>` ⇒ 文档仓；`@repo/<rel>` ⇒ 主仓（两个树显式分开 · 不猜）。"""
    if target.startswith(DOCS_PREFIX):
        return os.path.join(docs, target[len(DOCS_PREFIX):]), "docs"
    if target.startswith(REPO_PREFIX):
        return os.path.join(root, target[len(REPO_PREFIX):]), "repo"
    return "", "?"


def current_version(root):
    """现读版本真源 core/internal/version/version.go 的 const Version（取不到 ⇒ 空串）。"""
    p = os.path.join(root, "core", "internal", "version", "version.go")
    try:
        with open(p, encoding="utf-8") as f:
            m = re.search(r'Version\s*=\s*"([^"]+)"', f.read())
        return m.group(1) if m else ""
    except OSError:
        return ""


def register_version(register):
    """登记件路径里的版本段（如 v2.5.12 ⇒ 2.5.12）。"""
    m = re.search(r"v(\d+\.\d+\.\d+)", register)
    return m.group(1) if m else ""


def hits_of(path, text):
    """`grep -F` 语义：命中**行数** + 命中行号（判据吃「唯一」，行号只作导航报告）。"""
    with open(path, encoding="utf-8", errors="replace") as f:
        return [i for i, ln in enumerate(f.read().splitlines(), 1) if text and text in ln]


def run(root, docs_root, register, json_out):
    root = os.path.abspath(root)
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根" % root)
        return 2
    docs = os.path.abspath(docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档"))
    if not os.path.isdir(docs):
        print("⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`" % docs)
        return 2

    rows, problems = parse_register(register)
    if problems:
        for p in problems:
            print("⛔ 不给结论（rc=2）：%s" % p)
        return 2
    if not rows:
        print("⛔ 不给结论（rc=2）：登记件里 0 条锚（空转 = 假绿）—— %s" % register)
        return 2

    reds, uniques, counts = [], [], collections.Counter()
    for r in rows:
        counts[r["anchor"]] += 1
        if not ANCHOR_RE.match(r["anchor"]):
            reds.append({"anchor": r["anchor"], "why": "锚号缺类型前缀（形态 `^[A-Z]{1,3}-\\d+` 不过）",
                         "line": r["line"], "target": r["target"], "text": r["text"]})
            continue
        if not r["text"]:
            reds.append({"anchor": r["anchor"], "why": "内容锚为空", "line": r["line"],
                         "target": r["target"], "text": r["text"]})
            continue
        if "\\|" in r["text"]:
            reds.append({"anchor": r["anchor"], "why": "内容锚里带了转义竖线 `\\|` ⇒ 没法逐字对拍（换一段不含 `|` 的逐字句）",
                         "line": r["line"], "target": r["target"], "text": r["text"]})
            continue
        path, which = resolve(root, docs, r["target"])
        if not which or not path:
            reds.append({"anchor": r["anchor"], "why": "目标件位形态错（须 `@docs/…` 或 `@repo/…`）",
                         "line": r["line"], "target": r["target"], "text": r["text"]})
            continue
        if not os.path.isfile(path):
            problems.append("目标件不在盘：%s（锚 `%s` · 第 %d 行）" % (path, r["anchor"], r["line"]))
            continue
        hl = hits_of(path, r["text"])
        if not hl:
            reds.append({"anchor": r["anchor"], "why": "锚 0 命中（逐字句在目标件里找不到）",
                         "line": r["line"], "target": r["target"], "text": r["text"], "hits": []})
        elif len(hl) > 1:
            reds.append({"anchor": r["anchor"], "why": "锚多命中 %d 处（锚不唯一 ⇒ 指不到一处）" % len(hl),
                         "line": r["line"], "target": r["target"], "text": r["text"], "hits": hl})
        else:
            uniques.append({"anchor": r["anchor"], "target": r["target"], "hit": hl[0],
                            "text": r["text"], "line": r["line"]})
    for a, n in sorted(counts.items()):
        if n > 1:
            reds.append({"anchor": a, "why": "锚号重号 %d 次（一个号指两处 ⇒ 号空间被污染）" % n,
                         "line": 0, "target": "-", "text": "-"})

    # 缺口 `GAP-20260926-106`：登记过期告警（键失指 ⇒ 点名）。登记件路径里的版本段 ↔ 现读版本真源不符
    #   ⇒ 内容锚可能已失指（只告警，不改退码：给后续人一个可读信号，而不是静默漂着）。
    warnings = []
    _regv, _cur = register_version(register), current_version(root)
    if _regv and _cur and _regv != _cur:
        warnings.append("登记件版本 v%s 与当前版 v%s 不符 ⇒ 疑**登记过期**"
                        "（内容锚可能已失指；缺口 GAP-20260926-106）" % (_regv, _cur))

    if json_out:
        doc = {"schema": "zerg/v1", "kind": "AnchorUniqueness", "tool": VERSION,
               "items": uniques, "meta": {"anchors": len(rows), "unique": len(uniques),
                                          "reds": len(reds), "register": register},
               "out_of_domain": reds, "problems": problems, "warnings": warnings, "truncated": False}
        print(json.dumps(doc, ensure_ascii=False))
        if problems:
            return 2
        return 1 if reds else 0

    print("── 内容锚唯一性自检（%s · `W-45` / `R-17`）──" % VERSION)
    print("  主仓根  : %s" % root)
    print("  文档仓根: %s" % docs)
    print("  登记件  : %s" % register)
    for _w in warnings:
        print("  ⚠ 告警：%s" % _w)
    if problems:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · %s" % p)
        return 2
    print("  %-14s %-56s %s" % ("锚号", "目标件（命中行）", "内容锚（逐字句 · 截断）"))
    for u in uniques:
        print("  %-14s %-56s %s" % (u["anchor"], "%s:%d" % (u["target"], u["hit"]), u["text"][:42]))
    print("  ── 锚 %d 条：唯一 %d · 红 %d" % (len(rows), len(uniques), len(reds)))
    if reds:
        print("✗ 必报 %d 条（rc=1 · 逐条点名）：" % len(reds))
        for r in reds:
            tail = ""
            if r.get("hits"):
                tail = " · 命中行 %s" % ", ".join(str(h) for h in r["hits"])
            print("   · 第 %s 行 锚 `%s` → %s ⇒ %s%s" %
                  (r["line"] or "-", r["anchor"], r["target"], r["why"], tail))
        print("⇒ 处置（**不是**改本门）：① 0 命中 ⇒ 内容锚那一句已不在目标件（改锚或改件）；"
              "② 多命中 ⇒ 加长逐字句到唯一；③ 缺前缀 / 重号 ⇒ 改锚号。")
        return 1
    print("⇒ 全唯一（rc=0）：每条内容锚在其目标件里 `grep -F` 恰好命中 1 处 · 每个锚号带类型前缀")
    print("   ★ 本件只**读**目标件：一个字节未改（改它们是别的笔的活）")
    return 0


def list_rules():
    print("── %s 判据 ──" % VERSION)
    print("  ① 锚号形态 `^[A-Z]{1,3}-\\d+(-\\d+)*$`（**类型前缀**是判据的一部分）")
    print("  ② 内容锚在目标件里 `grep -F` 命中**行数**恰好 1（0 ⇒ 红 · ≥2 ⇒ 红；两向都报 + 命中行号）")
    print("  ③ 锚号唯一（同号两行 ⇒ 红）")
    print("  目标件位前缀：`%s<相对文档仓>` / `%s<相对主仓>`（两棵树显式分开）" % (DOCS_PREFIX, REPO_PREFIX))
    print("  退码：0 全唯一 · 1 有红（逐条点名）· 2 不给结论（登记件缺/空转/目标件缺/自检不过）")
    print("  现成登记件：%s" % REGISTER_REL)
    print("★ 本笔不挂门（见件头）：挂门是另一笔的活。")
    return 0


# ───────────────────────────────── 自检（成对负控 · 合成夹具）─────────────────────────────────
FIX = "@docs/件.md"


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="acu-selftest-")
    repo = os.path.join(tmp, "repo")
    docs = os.path.join(tmp, "Zerg-内部文档")
    os.makedirs(os.path.join(repo, "scripts", "gates"))
    os.makedirs(docs)
    reg = os.path.join(docs, "登记.md")

    def write_reg(body):
        with open(reg, "w", encoding="utf-8") as f:
            f.write("# 合成登记件\n\n| 锚号（类型前缀） | 目标件 | 内容锚（逐字句 · 可 `grep -F`） | 备注 |\n")
            f.write("|---|---|---|---|\n")
            f.write(body)

    def write_target(body):
        with open(os.path.join(docs, "件.md"), "w", encoding="utf-8") as f:
            f.write(body)

    def case(name, want_rc, got_rc, extra_ok=True, note=""):
        nonlocal ok
        good = (want_rc == got_rc) and extra_ok
        ok = ok and good
        print("  %s %s（rc=%s · 期望 %s）%s" % ("✓" if good else "✗", name, got_rc, want_rc, note))

    def cap(*a, **kw):
        import contextlib
        import io
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc = run(*a, **kw)
        return rc, buf.getvalue()

    # ① 正控：两条锚都唯一
    write_target("头。\n唯一句甲在此。\n中间。\n唯一句乙在此。\n尾。\n")
    write_reg("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n| `Q-036-01` | `%s` | 唯一句乙在此。 | b |\n" % (FIX, FIX))
    rc, out = cap(repo, docs, reg, False)
    case("① 正控：两条锚各命中 1 ⇒ rc=0", 0, rc, "全唯一" in out)

    # ② 负控：锚 0 命中
    write_reg("| `W-45-01` | `%s` | 压根不在件里的句子。 | a |\n| `Q-036-01` | `%s` | 唯一句乙在此。 | b |\n" % (FIX, FIX))
    rc, out = cap(repo, docs, reg, False)
    case("② 负控：锚 0 命中 ⇒ rc=1", 1, rc, "锚 0 命中" in out)

    # ③ 负控：锚多命中（同一逐字句在目标件出现两次）
    write_target("头。\n重复句在此。\n中间。\n重复句在此。\n尾。\n")
    write_reg("| `W-45-01` | `%s` | 重复句在此。 | a |\n" % FIX)
    rc, out = cap(repo, docs, reg, False)
    case("③ 负控：锚多命中 ⇒ rc=1", 1, rc, "多命中" in out)
    case("③b 多命中逐条给命中行号", True, ("命中行 2, 4" in out), note="  见输出")

    # ④ 负控：锚号缺类型前缀（`45-01` —— 就是 `R-xx` vs `Rxx` 那一课的形态）
    write_target("头。\n唯一句甲在此。\n尾。\n")
    write_reg("| `45-01` | `%s` | 唯一句甲在此。 | a |\n" % FIX)
    rc, out = cap(repo, docs, reg, False)
    case("④ 负控：锚号缺类型前缀 ⇒ rc=1", 1, rc, "缺类型前缀" in out)

    # ⑤ 负控：锚号重号
    write_reg("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n| `W-45-01` | `%s` | 唯一句甲在此。 | b |\n" % (FIX, FIX))
    rc, out = cap(repo, docs, reg, False)
    case("⑤ 负控：同号两行 ⇒ rc=1", 1, rc, "重号" in out)

    # ⑥ 负控：目标件位形态错（不带 `@docs/` / `@repo/` 前缀）
    write_reg("| `W-45-01` | `项目文档/件.md` | 唯一句甲在此。 | a |\n")
    rc, out = cap(repo, docs, reg, False)
    case("⑥ 负控：目标件位缺前缀 ⇒ rc=1", 1, rc, "目标件位形态错" in out)

    # ⑦ 空转（登记件里 0 条锚）⇒ rc=2
    write_reg("")
    rc, out = cap(repo, docs, reg, False)
    case("⑦ 空转（登记 0 条）⇒ rc=2 不给结论", 2, rc, "不给结论" in out)

    # ⑧ 目标件不在盘 ⇒ rc=2（不拿「读不到」当「唯一」）
    write_reg("| `W-45-01` | `@docs/没有这件.md` | 唯一句甲在此。 | a |\n")
    rc, out = cap(repo, docs, reg, False)
    case("⑧ 目标件不在盘 ⇒ rc=2", 2, rc, "不给结论" in out)

    # ⑨ 登记件不在盘 ⇒ rc=2
    rc, out = cap(repo, docs, os.path.join(docs, "没有这件.md"), False)
    case("⑨ 登记件不在盘 ⇒ rc=2", 2, rc, "不给结论" in out)

    # ⑩ 真话负控：把锚改成唯一（同件同登记位）⇒ 回绿
    write_reg("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n" % FIX)
    rc, out = cap(repo, docs, reg, False)
    case("⑩ 真话负控（同一夹具 · 改回唯一）⇒ 回绿", 0, rc, "全唯一" in out)

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True, description="内容锚唯一性自检（W-45 / R-17）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="主仓仓根（默认真源：本脚本所在仓）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认：主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--register", default=None, help="登记件路径（缺省 = 文档仓里那件）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    reg = a.register
    if not reg:
        base = a.docs_root or os.path.join(os.path.dirname(os.path.abspath(a.root)), "Zerg-内部文档")
        reg = os.path.join(base, REGISTER_REL)
    return run(a.root, a.docs_root, reg, a.json)


if __name__ == "__main__":
    sys.exit(main())

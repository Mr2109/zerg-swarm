#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-docs-orphans-links.py —— 文档「孤儿 / 本地断链」**拆两格**（只报不拦 · 2026-09-24 波4 序89）

判据来源（**绑私有面** · 件头只写编号与读法，不外引设计真源的原文）
------------------------------------------------------------------
  · 组4 §一 **`W-04`**（`拍-C:48` · `拍-清:146` 的 `R24` 照做条）逐字：**文档孤儿 / 断链拆两格**
    （**孤儿 = 主仓导出树** · **本地断链 = 两棵树**）· **先只报不拦** · **外链明确不进**。
  · `W-06`（后续条 · 2026-09-24 **波6 序91 已落** —— 见下面 ③④ 两格）逐字：两词分开写、不许互称 ——
    **入链 = 0**（域 = 另一棵树整棵）vs **不在 nav 白名单**（域 = 有 `mkdocs.yml` 的树）。
  · ★ **序91 落点（本件）**：③ `inbound_zero`（**入链 = 0** · 域 = **第二棵树整棵**）·
    ④ `not_in_nav`（**不在 nav 白名单** · 域 = **有 `mkdocs.yml` 的树**）· 术语守卫 `term_guard`
    （两词**不许互称** · 自检 ⑤ 是它的负控）+ 一行「**本仓无 `mkdocs.yml`**」登记。
  · 本件**不替** `W-05`（换 `lychee --offline`）—— 那条**同件面 ⇒ 串行**。

两格是什么（写死）
------------------
  ① `local_broken`（**本地断链** · 域 = **两棵树**）：markdown 件里的**仓内相对链接**解析后**指不到**
     真件 ⇒ 该**件**记一处。域 = `--repo-root`（默认 = 本文件上溯两级）+ `--docs-root`
     （默认 = 同级目录 `Zerg-内部文档`；不在盘 ⇒ 该树**照实标「缺」**、另一棵树照判 · **不**因它判不了）。
  ② `orphans`（**孤儿** · 域 = **主仓导出树**）：导出树里**入链 = 0** 的 `.md` 件。
     「入链」的判据 = 树内**别的**件有一条**仓内相对**链接解析后落在它身上（**同名同路径**才算）。
     ★ **不是**「有没有被人读过」、**不是** mtime/体积/占用 ⇒ 只数**链接**（照 `W-06` 的「入链 = 0」同一枚度量）。
  ③ `inbound_zero`（**入链 = 0** · 域 = **第二棵树整棵** · `W-06` 序91）：**与 ② 同名度量、不同域** ——
     ② 的域是**主仓导出树**、③ 的域是**另一棵树整棵**；两格的件**不许互称**（`term_guard`）。
     ★ 与 ② 不是一件事：导出树是**产物**（没链进产物 = 发布面漏了）、另一棵树是**源**（没链进源 = 源里悬着）。
  ④ `not_in_nav`（**不在 nav 白名单** · 域 = **有 `mkdocs.yml` 的树** · `W-06` 序91）：**另一枚词、另一个域**
     —— 只在有 `mkdocs.yml` 的树里判（抽白名单 = `mkdocs.yml` 的 `nav:`）。**本仓无 `mkdocs.yml`**
     ⇒ 这一格**无适用域**（照实打印「无适用域」并登记那一行），**不是**「0 件」✗。

外链**明确不进**（★ 这一条是负控钉住的，不是注释里的君子协定）
------------------------------------------------------------
  `http://` · `https://` · `ftp://` · `//host/…`（协议相对）· `mailto:` · `tel:` · `#锚`（无路径）·
  `/绝对路径`（从树根起 · 不按件所在目录解析）· 含 `{`/`}` 的模板串 · `<>` 包裹 · 空串
  ⇒ **一律不进任何一格**（自检 ③ 就是同一段坏文本，一处外链 + 一处真断链，只有真断链那一处被数）。

退码（**只报不拦** —— 与仓内其余门脚本不同，本门**没有 1**）
--------------------------------------------------------------
  0 = 判得了（**无论两格命中多少** —— 「先只报不拦」逐字：命中**不改**退出码 ✗）
  2 = **不给结论**：`--repo-root` 不在盘 / 扫面 **0 件** / 件读不了 / `--self-test` 不过
      ★ 导出树不在盘 ⇒ **孤儿格 = 判不了**（照实打印 `orphans = None`），**其余格照判**、rc 仍 0
        —— 「这一格判不了」与「整门判不了」是两件事，不许混（本仓那条「缺件 ≠ 绿」也只钉被影响的那一格）。

用法
----
    python3 scripts/gates/check-docs-orphans-links.py                  # 两格全判（只报）
    python3 scripts/gates/check-docs-orphans-links.py --json           # 末尾追加机器面一行
    python3 scripts/gates/check-docs-orphans-links.py --tree DIR       # 换导出树（默认 $ZERG_PUB_TREE）
    python3 scripts/gates/check-docs-orphans-links.py --list-rules     # 打判据 / 两格定义 / 覆盖与未覆盖
    python3 scripts/gates/check-docs-orphans-links.py --self-test      # 只跑自检（合成夹具，不碰真目标）
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import tempfile
import urllib.parse

VERSION = "1.0.0"
DEFAULT_TREE = "/tmp/zerg-pub-final-20260921"

# ── 链接抽取：行内 `](target)` 与 `](<target>)`；参考式定义 `]: target` ──────────────
LINK_RE = re.compile(r"\]\(\s*(<[^>]*>|[^)\s]*)(?:\s+\"[^\"]*\")?\s*\)")
DEF_RE = re.compile(r"^\s*\[[^\]]+\]:\s*(\S+)")

# ── 外链 / 非仓内形态（**明确不进任何一格**）────────────────────────────────────
EXTERNAL_SCHEMES = ("http://", "https://", "ftp://", "ftps://", "mailto:", "tel:", "data:", "//")


def is_external(target: str) -> str:
    """外链 / 非仓内形态 ⇒ 返回理由（空串 = 也许算仓内，交给调用方继续判）。"""
    t = target.strip()
    if t.startswith("<") and t.endswith(">"):
        t = t[1:-1].strip()
    if t == "":
        return "空目标"
    low = t.lower()
    for s in EXTERNAL_SCHEMES:
        if low.startswith(s):
            return "外链（%s…）" % s
    if t.startswith("#"):
        return "纯锚（无路径）"
    if t.startswith("/"):
        return "绝对路径（不按件所在目录解析）"
    if "{" in t or "}" in t:
        return "模板串（含花括号）"
    return ""


def _norm(path: str) -> str:
    return os.path.normpath(path).replace(os.sep, "/")


def scan_tree(root: str):
    """扫一棵树里的 .md 件 ⇒ {仓相对路径: [行号] 或 全文}。返回 (files, note)。"""
    if not os.path.isdir(root):
        return None, "不在盘"
    out = {}
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "venv", ".venv", "__pycache__")]
        for fn in fns:
            if not fn.endswith(".md"):
                continue
            full = os.path.join(dp, fn)
            rel = _norm(os.path.relpath(full, root))
            try:
                with open(full, "r", encoding="utf-8", errors="replace") as fh:
                    out[rel] = fh.read()
            except OSError:
                out[rel] = None
    return out, ("%d 件 .md" % len(out))


def links_of(text: str):
    """逐行抽链接 ⇒ [(行号, 原文目标)]。"""
    got = []
    for i, line in enumerate(text.splitlines(), 1):
        for m in LINK_RE.finditer(line):
            got.append((i, m.group(1)))
        m = DEF_RE.match(line)
        if m:
            got.append((i, m.group(1)))
    return got


def local_broken(root: str):
    """一格：本地断链（域 = 一棵树）⇒ (件数, 明细, 判不了理由)。"""
    if root is None or not os.path.isdir(root):
        return None, [], "根不在盘"
    files, _ = scan_tree(root)
    if not files:
        return 0, [], "扫面 0 件 .md"
    have = set(files)
    hits = []
    for rel in sorted(files):
        text = files[rel]
        if text is None:
            return None, [], "件读不了：%s" % rel
        for lineno, raw in links_of(text):
            why = is_external(raw)
            if why:
                continue
            tgt = raw.strip()
            if tgt.startswith("<") and tgt.endswith(">"):
                tgt = tgt[1:-1].strip()
            tgt = urllib.parse.unquote(tgt.split("#", 1)[0])
            if tgt == "":
                continue
            resolved = _norm(os.path.join(os.path.dirname(rel), tgt))
            if resolved in have or os.path.exists(os.path.join(root, resolved)):
                continue
            hits.append({"file": rel, "line": lineno, "raw": tgt, "resolved": resolved})
    return len({h["file"] for h in hits}), hits, ""


def orphans(tree: str):
    """二格：孤儿（域 = 导出树）⇒ (件数, 明细, 判不了理由)。"""
    if tree is None or not os.path.isdir(tree):
        return None, [], "导出树不在盘：%s" % (tree if tree else "（没给）")
    allf = set()
    for dp, dns, fns in os.walk(tree):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "venv", ".venv", "__pycache__")]
        for fn in fns:
            allf.add(_norm(os.path.relpath(os.path.join(dp, fn), tree)))
    mds = {p for p in allf if p.endswith(".md")}
    if not mds:
        return 0, [], "扫面 0 件 .md"
    inbound = {}
    for rel in sorted(mds):
        try:
            with open(os.path.join(tree, rel), "r", encoding="utf-8", errors="replace") as fh:
                text = fh.read()
        except OSError:
            return None, [], "件读不了：%s" % rel
        for _lineno, raw in links_of(text):
            if is_external(raw):
                continue
            tgt = raw.strip()
            if tgt.startswith("<") and tgt.endswith(">"):
                tgt = tgt[1:-1].strip()
            tgt = urllib.parse.unquote(tgt.split("#", 1)[0])
            if tgt == "":
                continue
            resolved = _norm(os.path.join(os.path.dirname(rel), tgt))
            if resolved in allf:
                inbound[resolved] = inbound.get(resolved, 0) + 1
    orphan = sorted(p for p in mds if inbound.get(p, 0) == 0)
    return len(orphan), [{"file": p, "inbound": 0} for p in orphan], ""


# ── `W-06`（2026-09-24 · 波6 序91）：两词**分开写、不许互称** ──────────────────────────
#   「入链 = 0」与「不在 nav 白名单」是**两枚词、两个域**：前者数**链接**（域 = 另一棵树整棵），
#   后者数**白名单**（域 = 有 `mkdocs.yml` 的树）。互称一次就是把两个域混成一个 ⇒ 判词没法追。
TERM_OF = {
    "inbound_zero": "入链 = 0",
    "not_in_nav": "不在 nav 白名单",
}


def term_guard(kind: str, word: str) -> str:
    """两词**不许互称**：只认自己那一枚 ⇒ '' 或判词（自检 ⑤ 是它的负控）。"""
    want = TERM_OF.get(kind)
    if want is None:
        return "未知格 kind=%r（只认 %s）" % (kind, " / ".join(sorted(TERM_OF)))
    if word != want:
        return "格 `%s` 的用词是「%s」，不是「%s」—— 两词不许互称" % (kind, want, word)
    return ""


def has_mkdocs(root: str) -> bool:
    """这一棵树有没有 `mkdocs.yml`（`not_in_nav` 格的**域判据**）。"""
    return os.path.isfile(os.path.join(root, "mkdocs.yml"))


def nav_whitelist(root: str):
    """抽 `mkdocs.yml` 的 `nav:` 白名单 ⇒ `(set(相对路径), '')`；无 `mkdocs.yml` ⇒ `(None, 理由)`。

    ★ **本仓无 `mkdocs.yml`** ⇒ 这一格**无适用域**（照实打印、登记一行），**不是**「0 件」✗。
    """
    p = os.path.join(root, "mkdocs.yml")
    if not os.path.isfile(p):
        return None, "本仓无 `mkdocs.yml`"
    try:
        with open(p, encoding="utf-8") as f:
            text = f.read()
    except OSError as e:
        return None, "`mkdocs.yml` 读不了（%s）" % e
    out, seen = set(), False
    for line in text.splitlines():
        stripped = line.strip()
        if re.match(r"^nav\s*:", stripped):
            seen = True
            continue
        if not seen:
            continue
        if stripped and not line.startswith((" ", "\t")):
            break                      # 顶格新键 ⇒ `nav:` 块到此为止
        for m in re.finditer(r"[\w./\-]+\.md", line):
            out.add(m.group(0))
    return out, ""


def not_in_nav(root: str):
    """④ 不在 nav 白名单（域 = **有 `mkdocs.yml` 的树**）⇒ `(件数 | None, 明细, 判不了理由)`。"""
    wl, why = nav_whitelist(root)
    if wl is None:
        return None, [], why
    files, note = scan_tree(root)
    if files is None:
        return None, [], note
    if not files:
        return None, [], "扫面 0 件（无 `.md`）"
    hits = [{"file": p} for p in sorted(files) if p not in wl]
    return len(hits), hits, ""


LIST_RULES = """判据（两格 · 只报不拦）
------------------------
  ① local_broken 本地断链 · 域 = 两棵树（--repo-root + --docs-root）
     判据 = markdown 件里的**仓内相对**链接（行内 `](t)` + 参考式 `]: t`）解析后指不到真件
     计数单位 = **件**（同一件多处断链只算一件）；明细 = 件:行 + 原文 + 解析后路径
  ② orphans 孤儿 · 域 = 导出树（--tree）
     判据 = 树内 `.md` 件的**入链数 = 0**（树内别的件有一条仓内相对链接解析后落在它身上 = 有入链）
     计数单位 = **件**；明细 = 件（入链 0）
  ③ inbound_zero **入链 = 0** · 域 = **第二棵树整棵**（--docs-root · `W-06` 序91）
     判据 = 同一枚度量（链接数），**另一个域**（源树整棵）；与 ② **不许互称**
  ④ not_in_nav **不在 nav 白名单** · 域 = **有 `mkdocs.yml` 的树**（`W-06` 序91）
     判据 = `mkdocs.yml` 的 `nav:` 白名单外的 `.md` 件；**本仓无 `mkdocs.yml`** ⇒ **无适用域**（不是 0 件 ✗）

外链明确不进（负控钉住）
------------------------
  http(s) / ftp(s) / 协议相对 `//` / mailto / tel / data / 纯锚 `#x` / 绝对路径 `/x` /
  含花括号的模板 / `<>` 包裹 / 空串 ⇒ **不进任何一格**

未覆盖（不假装全覆盖）
----------------------
  · 目录型链接（指向目录而非件）只判「目录在不在」，不追目录内默认件
  · 非 `.md` 件里的链接（.html/.txt/.py）**不扫**（本题面只管文档）
  · 另一棵树里的**导出树**（跨仓的产出树）不判
  · nav 白名单 / `mkdocs.yml` 那一半 = `W-06`（2026-09-24 **波6 序91 已落** ⇒ 落成上面 ③④ 两格；
    **本仓无 `mkdocs.yml`** 那一行**照实登记**、不假装覆盖）

退码
----
  0 = 判得了（命中多少都不改码 —— 「先只报不拦」）
  2 = 不给结论（仓根缺 / 扫面 0 件 / 件读不了 / 自检不过）
"""


# ── 自检（成对负控 · 合成夹具 · 不碰真目标）──────────────────────────────────────
def self_test(out) -> bool:
    total = 0
    failed = 0

    def case(name, want, got):
        nonlocal total, failed
        total += 1
        ok = want == got
        if not ok:
            failed += 1
        out.write("  %s %s（期望 %s · 实得 %s）\n" % ("✓" if ok else "✗", name, want, got))

    tmp = tempfile.mkdtemp(prefix="coan-selftest-")
    # 夹具①：断链 1 处 + 外链若干（外链一处都不许被数）
    t1 = os.path.join(tmp, "t1")
    os.makedirs(os.path.join(t1, "sub"), exist_ok=True)
    with open(os.path.join(t1, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n\n[好](sub/b.md) · [坏](sub/nope.md)\n\n"
                "[外](https://example.com/x.md) · [锚](#sec) · [绝](/etc/hosts) · "
                "[邮](mailto:a@b.c) · [协相对](//host/x) · [模]({{var}}/x.md)\n")
    with open(os.path.join(t1, "sub", "b.md"), "w", encoding="utf-8") as f:
        f.write("# b\n\n[回](../a.md)\n")
    n, hits, why = local_broken(t1)
    case("① 本地断链：件数（只数真断链 · 外链 6 处全不进）", 1, n)
    case("① 本地断链：点名到 件:行 + 解析后路径", ["a.md:3", "sub/nope.md"],
         [("%s:%d" % (hits[0]["file"], hits[0]["line"])) if hits else "", hits[0]["resolved"] if hits else ""])

    # 夹具②：完好链接 ⇒ 0 件
    t2 = os.path.join(tmp, "t2")
    os.makedirs(t2, exist_ok=True)
    with open(os.path.join(t2, "a.md"), "w", encoding="utf-8") as f:
        f.write("[好](b.md)\n")
    with open(os.path.join(t2, "b.md"), "w", encoding="utf-8") as f:
        f.write("[回](a.md)\n")
    case("② 正控：全好 ⇒ 本地断链 0 件", 0, local_broken(t2)[0])

    # 夹具③：孤儿 —— c.md 无人链 ⇒ 1 件；a/b 互相链 ⇒ 不是孤儿
    t3 = os.path.join(tmp, "t3")
    os.makedirs(t3, exist_ok=True)
    with open(os.path.join(t3, "a.md"), "w", encoding="utf-8") as f:
        f.write("[b](b.md)\n")
    with open(os.path.join(t3, "b.md"), "w", encoding="utf-8") as f:
        f.write("[a](a.md)\n")
    with open(os.path.join(t3, "c.md"), "w", encoding="utf-8") as f:
        f.write("没人链我（[外链不算入链](https://example.com/a.md)）\n")
    n, hits, _why = orphans(t3)
    case("③ 孤儿：入链 = 0 的件数（外链不算入链）", 1, n)
    case("③ 孤儿：点名到件", ["c.md"], [h["file"] for h in hits])

    # 夹具④：判不了 —— 根不在盘
    case("④ 判不了：根不在盘 ⇒ local_broken 返回 判不了", True,
         local_broken(os.path.join(tmp, "nope"))[0] is None)
    case("④ 判不了：导出树不在盘 ⇒ orphans 返回 判不了", True,
         orphans(os.path.join(tmp, "nope"))[0] is None)

    # 夹具⑤-⑦ `W-06`（2026-09-24 · 波6 序91）：两词**分开写、不许互称** + 两格各有独立域
    case("⑤ W-06 术语守卫：本词 ⇒ 放行", "", term_guard("inbound_zero", "入链 = 0"))
    case("⑤ W-06 术语守卫（负控：把「入链 0」说成「不在 nav」⇒ 判词非空）", True,
         bool(term_guard("inbound_zero", "不在 nav 白名单")))
    case("⑤ W-06 术语守卫（负控：反向互称也判）", True,
         bool(term_guard("not_in_nav", "入链 = 0")))
    t6a = os.path.join(tmp, "t6a")
    os.makedirs(t6a, exist_ok=True)
    with open(os.path.join(t6a, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n")
    case("⑥ W-06 nav：无 `mkdocs.yml` ⇒ **无适用域**（判不了 · 不是 0 件）", True,
         not_in_nav(t6a)[0] is None)
    case("⑥ W-06 nav：无 `mkdocs.yml` ⇒ 理由点名本仓无", True, "无 `mkdocs.yml`" in not_in_nav(t6a)[2])
    t6b = os.path.join(tmp, "t6b")
    os.makedirs(t6b, exist_ok=True)
    with open(os.path.join(t6b, "mkdocs.yml"), "w", encoding="utf-8") as f:
        f.write("site_name: x\nnav:\n  - 首页: index.md\n")
    with open(os.path.join(t6b, "index.md"), "w", encoding="utf-8") as f:
        f.write("# i\n")
    with open(os.path.join(t6b, "loose.md"), "w", encoding="utf-8") as f:
        f.write("# l\n")
    n6, hits6, _w6 = not_in_nav(t6b)
    case("⑥ W-06 nav：白名单外的 `.md` 被数（1 件）", 1, n6)
    case("⑥ W-06 nav：点名到件（白名单里的 index.md 不算）", ["loose.md"], [h["file"] for h in hits6])
    n7, hits7, _w7 = orphans(t3)
    case("⑦ W-06 ③ 域 = 第二棵树整棵：与 ② 同度量、换域照数（1 件）", 1, n7)
    case("⑦ W-06 ③ 与 ② 的点名件同一枚（度量同源 · 域不同）", [h["file"] for h in orphans(t3)[1]],
         [h["file"] for h in hits7])

    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True, description="文档孤儿 / 本地断链两格（只报不拦）")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--docs-root", default=None,
                    help="第二棵树（默认 = 同级目录 Zerg-内部文档；不在盘 ⇒ 照实标缺）")
    ap.add_argument("--tree", default=None, help="导出树（默认 $ZERG_PUB_TREE 或 %s）" % DEFAULT_TREE)
    ap.add_argument("--max-examples", type=int, default=20)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        sys.stdout.write("── 自检（合成夹具 · 不碰真目标）──\n")
        return 0 if self_test(sys.stdout) else 2

    if not os.path.isdir(a.repo_root):
        sys.stdout.write("⛔ 仓根不在盘：%s ⇒ 不给结论（rc=2）\n" % a.repo_root)
        return 2
    docs_root = a.docs_root if a.docs_root is not None else os.path.join(os.path.dirname(a.repo_root), "Zerg-内部文档")
    tree = a.tree if a.tree is not None else os.environ.get("ZERG_PUB_TREE", DEFAULT_TREE)

    sys.stdout.write("文档孤儿 / 本地断链（两格 · 只报不拦）· v%s\n" % VERSION)
    sys.stdout.write("  判据见 `--list-rules`：① local_broken（域 = 两棵树）· ② orphans（域 = 导出树）\n")
    sys.stdout.write("  域① 主仓根 = %s\n" % a.repo_root)
    sys.stdout.write("  域① 第二棵树 = %s%s\n" % (docs_root, "" if os.path.isdir(docs_root) else "  ★不在盘（该树照实标缺 · 不影响另一棵）"))
    sys.stdout.write("  域② 导出树 = %s%s\n" % (tree, "" if os.path.isdir(tree) else "  ★不在盘（孤儿格 = 判不了）"))

    bad = 0
    br_counts = {}
    br_all = []
    for label, root in (("主仓", a.repo_root), ("第二棵树", docs_root)):
        n, hits, why = local_broken(root)
        br_counts[label] = n
        if n is None:
            sys.stdout.write("  ⛔ 本地断链 · %s：判不了（%s）\n" % (label, why))
            if label == "主仓":
                bad = 1
        else:
            sys.stdout.write("  ✓ 本地断链 · %s：%d 件\n" % (label, n))
            for h in hits[: a.max_examples]:
                sys.stdout.write("      %s:%d  [%s] ⇒ %s（指不到）\n" % (h["file"], h["line"], h["raw"], h["resolved"]))
            br_all.extend(hits)
    if len(br_all) > a.max_examples * 2:
        sys.stdout.write("      （明细截断：共 %d 处 · 上面每树各取前 %d 行）\n" % (len(br_all), a.max_examples))

    on, ohits, owhy = orphans(tree)
    if on is None:
        sys.stdout.write("  ⛔ 孤儿 · 导出树：判不了（%s）—— 这一格判不了 ≠ 整门判不了\n" % owhy)
    else:
        sys.stdout.write("  ✓ 孤儿 · 导出树：%d 件（入链 = 0）\n" % on)
        for h in ohits[: a.max_examples]:
            sys.stdout.write("      %s（入链 0）\n" % h["file"])

    # ③④ `W-06`（2026-09-24 · 波6 序91）：**另一枚词、另一个域** —— 与 ① ② 分开写、**不许互称**
    inz_word, nav_word = TERM_OF["inbound_zero"], TERM_OF["not_in_nav"]
    for kind, word in (("inbound_zero", inz_word), ("not_in_nav", nav_word)):
        guard = term_guard(kind, word)
        if guard:
            sys.stdout.write("  ⛔ 术语守卫（W-06）：%s ⇒ 不给结论（rc=2）\n" % guard)
            return 2
    inz, inz_hits, inz_why = orphans(docs_root)          # 同一枚度量（链接数）· **换域** = 第二棵树整棵
    if inz is None:
        sys.stdout.write("  ⛔ %s · 第二棵树整棵：判不了（%s）—— 这一格判不了 ≠ 整门判不了\n"
                         % (inz_word, inz_why))
    else:
        sys.stdout.write("  ✓ %s · 第二棵树整棵：%d 件（与 ② 同度量 · 不同域）\n" % (inz_word, inz))
        for h in inz_hits[: a.max_examples]:
            sys.stdout.write("      %s（%s）\n" % (h["file"], inz_word))
    nav_n, nav_hits, nav_why = not_in_nav(a.repo_root)
    sys.stdout.write("  域④ 有 `mkdocs.yml` 的树 = %s ⇒ %s\n"
                     % (a.repo_root if has_mkdocs(a.repo_root) else "（本仓无 `mkdocs.yml`）",
                        "判" if has_mkdocs(a.repo_root) else "**无适用域**（照实登记 · 不假装覆盖）"))
    if nav_n is None:
        sys.stdout.write("  ⛔ %s · 本仓：无适用域（%s）—— 「无适用域」≠「0 件」✗\n" % (nav_word, nav_why))
    else:
        sys.stdout.write("  ✓ %s · 本仓：%d 件\n" % (nav_word, nav_n))
        for h in nav_hits[: a.max_examples]:
            sys.stdout.write("      %s（%s）\n" % (h["file"], nav_word))

    if br_counts.get("主仓", 0) is None:
        return 2
    sys.stdout.write("\n结论（只报不拦）：本地断链 主仓 %s 件 / 第二棵树 %s 件 · 孤儿 %s 件 · "
                     "%s 第二棵树整棵 %s 件 · %s 本仓 %s ⇒ rc=0（命中不改码 ✗）\n"
                     % (br_counts.get("主仓"), br_counts.get("第二棵树"), on,
                        inz_word, inz, nav_word, nav_n))
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1",
            "gate": "check-docs-orphans-links",
            "version": VERSION,
            "local_broken": {"count": sum(x for x in br_counts.values() if isinstance(x, int)),
                             "by_tree": br_counts},
            "orphans": {"count": on, "domain": tree},
            "inbound_zero": {"count": inz, "domain": docs_root, "word": inz_word},
            "not_in_nav": {"count": nav_n, "domain": a.repo_root if has_mkdocs(a.repo_root) else None,
                           "word": nav_word, "why": nav_why or None},
            "mode": "report-only",
        }, ensure_ascii=False) + "\n")
    return 0 if bad == 0 else 2


if __name__ == "__main__":
    sys.exit(main())

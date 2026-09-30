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

遮蔽（**不是**新判据 —— 把「本来就不是链接的文本」挡在抽取之外 · 2026-09-26 重测）
----------------------------------------------------------------------------
  ★ 现读病（旧版**在整行上**跑正则 ⇒ 抽走了两处**非链接文本**）：
    ① **围栏代码块**（` ``` ` / `~~~` 起止）—— 整块；
    ② **行内码 span**（反引号成对 · 支持多反引号定界）—— 块内。
    这两处的 `[文字](目标)` 是**被引用的文本**（别仓的坏链原话、链接语法示例、修法脚本里的字符串字面量）
    ⇒ **不是本仓的链接**。治法 = **遮蔽**：进抽取前把这两块替换成**等长空白**（保行号与列偏移 ⇒ 落点照旧可点名），
    遮蔽**之外**的同一段坏文本照旧判（成对负控钉住：围栏里不报 ∧ 围栏外照报）。
    口径与 `check-zh-en.py` 的 `strip_noncode` 同族（「先剥引用/代码，再判正文」）—— **复用已有管线**、不加新机制。
  ★ 遮蔽**不是**豁免名单、**不放宽判据**：它挡的是「不是链接的文本」，判据（解析后指不到真件）一字不改。

域里**不进**的两类（同为 2026-09-26 现读踩出来的）
--------------------------------------------------
  · **嵌套仓**（目录里有 `.git` = 子模块工作树 / 半克隆）**不进扫面域** —— 那是**别仓的件**：
    `vendor/rtk` 是子模块（`.gitmodules` 逐字 `path = vendor/rtk` · `url = https://github.com/rtk-ai/rtk.git`），
    父仓只跟踪一个 gitlink、**不跟踪它的内容**（`git ls-files vendor/rtk` = 1）。本门判不了、也不该判。
    照实打印「剪掉的嵌套仓 N 个」（登记一行）—— **不是静默跳过** ✗。
  · 已有剪枝目录（`.git` / `node_modules` / `venv` / `.venv` / `__pycache__`）照旧。

外链**明确不进**（★ 这一条是负控钉住的，不是注释里的君子协定）
------------------------------------------------------------
  `http://` · `https://` · `ftp://` · `//host/…`（协议相对）· `mailto:` · `tel:` · `#锚`（无路径）·
  `/绝对路径`（从树根起 · 不按件所在目录解析）· 含 `{`/`}` 的模板串 · `<>` 包裹 · 空串
  ⇒ **一律不进任何一格**（自检 ③ 就是同一段坏文本，一处外链 + 一处真断链，只有真断链那一处被数）。

基准偏移登记（`BASE_OFFSET`：件在 A 处、链接按 B 基准写 —— **收窄判据**，与 `check-links-lychee.py` 同式样）
----------------------------------------------------------------------------------------------------------
  ★ 为什么（现读证据 · 2026-09-26 重测本件）：`publish/mirror-public-lib.py` 的 `map_path` 把
    `publish/docs/README.md` / `README.zh-CN.md` 映到**公开树仓根**（README 家族放仓库根、语言切换互链），
    而本件按**原位**（`publish/docs/`）解析相对链接 ⇒ 这两个件里每条 `docs/...` / 根文件链接都被解析成
    `publish/docs/docs/...` · `publish/docs/LICENSE`（指不到）⇒ 现跑 **30 处全假红**
    （复核：`README.md` 15/15 命中 · `README.zh-CN.md` 15/15 命中；**原位 0/30 命中**）⇒ **真断链 0 条**。
  ★ 治法（收窄判据 · 与 lychee 门同一枚登记、同一枚复核）：命中登记 ⇒ **按登记基准重解析 + 现读存在性复核**：
      复核命中 ⇒ 记「已登记假红（基准偏移 · 已复核）」· **不计入断链数**（逐条印出落点，看得见）；
      复核落空 ⇒ **照旧判红** —— 豁免只挡「基准错」这一半，真断链一条也不放。
  ★ 登记体检（本改动的牙）：登记**过期** ⇒ rc=2 不给结论 · **逐字点名**，两种（**只对「登记件在扫面域内」的条目判**）：
      ① 登记基准不是目录（`<root>/<基准>` 找不到 ⇒ 基准指空）
      ② 登记件**在扫面域内却一条原位错都没有**（链接已改回原位可达 / 该件不再需要偏移 ⇒ 该撤登记）
    件被改名/移走 ⇒ 条目落到域外 ⇒ 不判过期（**也无害**：那时它不再产出任何假红）。

退码（**只报不拦** —— 与仓内其余门脚本不同，本门**没有 1**）
--------------------------------------------------------------
  0 = 判得了（**无论两格命中多少** —— 「先只报不拦」逐字：命中**不改**退出码 ✗）
  2 = **不给结论**：`--repo-root` 不在盘 / 扫面 **0 件** / 件读不了 / `--self-test` 不过 /
      **导出树读不到**（取树次序 `--tree` → 主名 env → 兼容别名 env → 无；都指不到真件 ⇒ 同此档）
      ★ 导出树 ⇒ **孤儿格 = 判不了**（照实打印 `orphans = None`）· **其余格照判**（照实打印），
        但导出树是本门判据的**必需输入** ⇒ **缺件 ≠ 绿**：本门退 **2 不给结论**（与 B12/B14/B16/B17 同口径）。
        ★ v1.1.0 及以前此处 rc 仍 0 = **静默绿** ✗（默认值写死一条早已不在盘的日期路径所致）—— 2026-10-01 收口。

env 名（2026-10-01 统一 · 同族口径）
----------------------------------
  · 主名 `ZERG_PUB_TREE`（与同族 `core/cmd/zerg/family_impact_layers.go` 逐字一致）。
  · 兼容别名 `ZERG_PUBLIC_TREE`（门侧 `precommit-gates.sh` 现用名）—— **只在主名为空时读它**。
  · 两者都设 ⇒ **主名优先**；都空 ⇒ 同「导出树读不到」（rc=2）。
  · ✗ 不许两边各改成一个互相不认识的第三个名字（那正是本口子的来源）。

用法
----
    python3 scripts/gates/check-docs-orphans-links.py                  # 两格全判（只报）
    python3 scripts/gates/check-docs-orphans-links.py --json           # 末尾追加机器面一行
    python3 scripts/gates/check-docs-orphans-links.py --tree DIR       # 换导出树（默认 $ZERG_PUB_TREE · 兼容 $ZERG_PUBLIC_TREE）
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

VERSION = "1.2.0"
# ★ 导出树默认值：**空**（不默写任何路径）。
#   旧值是一条**写死日期的 /tmp 快照路径**（早已不在盘）⇒ 树缺时本门沿默认值走、孤儿格判不了却仍 `rc=0`
#   = **静默绿** ✗（`GAP-20261001-01` 的唯一真口子 · 2026-10-01 收口）。
#   取树次序（写死）：`--tree` → `$ZERG_PUB_TREE`（主名 · 与同族 `core/cmd/zerg/family_impact_layers.go` 逐字一致）
#   → `$ZERG_PUBLIC_TREE`（兼容别名 · 门侧 `precommit-gates.sh` 现用名）→ 无。
DEFAULT_TREE = ""
PUB_TREE_ENV = "ZERG_PUB_TREE"            # 主名（go/py 同族）
PUB_TREE_ENV_ALIAS = "ZERG_PUBLIC_TREE"   # 兼容别名（门侧现名 · 只在主名为空时读它）

# ── 基准偏移登记（「件在 A 处、链接按 B 基准写」—— **收窄判据**，不是豁免名单）──────────────
#   为什么/治法/体检三态见件头「基准偏移登记」一节（与 `check-links-lychee.py` 同一枚登记面、同一枚复核。
#   ★ 键 = 件在**树根**下的相对路径（**无行号锚** —— 行号会漂，件+基准不漂）；值 = `(基准目录, 理由)`。
#   本仓现读：30 处全假红（15 + 15）· 真断链 0 条（复核见件头）。
BASE_OFFSET = {
    "publish/docs/README.md":       ("publish", "公开树仓根定位（README 家族放公开树仓根）"),
    "publish/docs/README.zh-CN.md": ("publish", "公开树仓根定位（与 en 版同基准 · 语言切换互链）"),
}

# ── 链接抽取：行内 `](target)` 与 `](<target>)`；参考式定义 `]: target` ──────────────
LINK_RE = re.compile(r"\]\(\s*(<[^>]*>|[^)\s]*)(?:\s+\"[^\"]*\")?\s*\)")
DEF_RE = re.compile(r"^\s*\[[^\]]+\]:\s*(\S+)")

# ── 遮蔽（**不是**判据）：围栏代码块 + 行内码 span ⇒ 抽取前替换成**等长空白**（保行号/列偏移）──
FENCE_RE = re.compile(r"^\s*(`{3,}|~{3,})")
INLINE_CODE_RE = re.compile(r"(`+)(.+?)\1")

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


def resolve_tree(cli_tree):
    """取导出树 + 来源（次序写死 · 见件头「env 名」与「退码」两节）：
    `--tree` → 主名 env → 兼容别名 env → 无。都没有 ⇒ 返回 ("", "")（调用方按「读不到」处理）。"""
    if cli_tree is not None:
        return cli_tree, "--tree"
    for name in (PUB_TREE_ENV, PUB_TREE_ENV_ALIAS):
        val = os.environ.get(name, "")
        if val.strip():
            return val.strip(), "$" + name
    return "", ""


# ── 遮蔽（**不是**判据：挡的是「本来就不是链接的文本」）──────────────────────────────
def _blank(line: str) -> str:
    """等长空白（保行号与列偏移 ⇒ 遮蔽外的落点照旧可逐字点名）。"""
    return " " * len(line)


def mask_inline_code(line: str) -> str:
    """剥**行内码 span**（反引号成对 · 支持多反引号定界）；不成对的照旧（不猜）。"""
    out = list(line)
    for m in INLINE_CODE_RE.finditer(line):
        for i in range(m.start(), m.end()):
            out[i] = " "
    return "".join(out)


def mask_code(text: str) -> str:
    """把 **围栏代码块**（整块）与**行内码 span**（块内）换成等长空白，只留「链接区」。

    ★ 这是抽取的**前置**一步，不是豁免名单：遮蔽外的真断链一条不放（自检⑧⑨成对钉住）。
    """
    out, fence = [], None
    for line in text.split("\n"):
        m = FENCE_RE.match(line)
        if fence is None:
            if m:
                fence = m.group(1)[0]
                out.append(_blank(line))
            else:
                out.append(mask_inline_code(line))
        else:
            if m and m.group(1)[0] == fence:
                fence = None
            out.append(_blank(line))
    return "\n".join(out)


# ── 域（扫面不走的两类：已有剪枝目录 + **嵌套仓**）─────────────────────────────────
PRUNE_DIRS = (".git", "node_modules", "venv", ".venv", "__pycache__")
NESTED_SEEN = {}          # {树根: [剪掉的嵌套仓相对路径]} —— 给 main 打印「照实登记」那一行


def tree_walk(root: str):
    """`os.walk` 的**域内**版：剪掉已有剪枝目录 + **嵌套仓**（目录里有 `.git` = 别仓的工作树 ⇒ 域外）。

    返回 `[(dp, dns, fns)]`；剪掉哪些嵌套仓记进 `NESTED_SEEN[root]`（**照实打印 · 不静默** ✗）。
    """
    triples, nested = [], []
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d not in PRUNE_DIRS)
        for d in list(dns):
            if os.path.exists(os.path.join(dp, d, ".git")):
                nested.append(_norm(os.path.relpath(os.path.join(dp, d), root)))
                dns.remove(d)
        triples.append((dp, list(dns), fns))
    NESTED_SEEN[root] = nested
    return triples


def scan_tree(root: str):
    """扫一棵树里的 .md 件 ⇒ {仓相对路径: 全文}。返回 (files, note)。"""
    if not os.path.isdir(root):
        return None, "不在盘"
    out = {}
    for dp, _dns, fns in tree_walk(root):
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
    note = "%d 件 .md" % len(out)
    if NESTED_SEEN.get(root):
        note += " · 剪掉嵌套仓 %d 个（域外：别仓的件）" % len(NESTED_SEEN[root])
    return out, note


def links_of(text: str):
    """逐行抽链接 ⇒ [(行号, 原文目标)]。**先遮蔽**（围栏块 + 行内码 span）再抽。"""
    got = []
    for i, line in enumerate(mask_code(text).splitlines(), 1):
        for m in LINK_RE.finditer(line):
            got.append((i, m.group(1)))
        m = DEF_RE.match(line)
        if m:
            got.append((i, m.group(1)))
    return got


def offset_ok(rel: str, tgt: str, root: str, have) -> str:
    """命中「基准偏移登记」的错 ⇒ **按登记基准重解析 + 现读存在性复核**。

    复核命中 ⇒ 返理由（记已登记假红）；`''` = 不豁免（未登记 / 复核落空 ⇒ **照旧判红**）。
    """
    ent = BASE_OFFSET.get(rel)
    if not ent:
        return ""
    base, why = ent
    cand = _norm(os.path.join(base, tgt))
    if cand in have or os.path.exists(os.path.join(root, cand)):
        return "%s（登记基准 `%s`）⇒ 复核命中 `%s`" % (why, base, cand)
    return ""


def offset_registry_check(root: str, files, raw_hits) -> list:
    """**登记体检**（本改动的牙）：登记过期 ⇒ 逐字点名（两种，见件头）。

    只对**登记件在扫面域内**的条目判（夹具树 / 别的域 ⇒ 不参与本跑、不判过期）。
    """
    stale = []
    if not isinstance(files, dict):
        return stale
    hit_files = set(h["file"] for h in raw_hits)
    for rel, ent in sorted(BASE_OFFSET.items()):
        if rel not in files:
            continue
        base = ent[0]
        if not os.path.isdir(os.path.join(root, base)):
            stale.append("%s ⇒ **登记基准 `%s` 不是目录**（基准指空 ⇒ 撤或改登记）" % (rel, base))
            continue
        if rel not in hit_files:
            stale.append("%s ⇒ **在扫面域内却一条原位错都没有**"
                         "（链接已改回原位可达 / 该件不再需要偏移 ⇒ 请撤登记）" % rel)
    return stale


def local_broken_detail(root: str):
    """一格：本地断链（域 = 一棵树）⇒ `(红件数, 红明细, 已登记假红明细, 登记过期, 判不了理由)`。"""
    if root is None or not os.path.isdir(root):
        return None, [], [], [], "根不在盘"
    files, _ = scan_tree(root)
    if not files:
        return 0, [], [], [], "扫面 0 件 .md"
    have = set(files)
    raw, hits, exempt = [], [], []
    for rel in sorted(files):
        text = files[rel]
        if text is None:
            return None, [], [], [], "件读不了：%s" % rel
        for lineno, rawtgt in links_of(text):
            why = is_external(rawtgt)
            if why:
                continue
            tgt = rawtgt.strip()
            if tgt.startswith("<") and tgt.endswith(">"):
                tgt = tgt[1:-1].strip()
            tgt = urllib.parse.unquote(tgt.split("#", 1)[0])
            if tgt == "":
                continue
            resolved = _norm(os.path.join(os.path.dirname(rel), tgt))
            if resolved in have or os.path.exists(os.path.join(root, resolved)):
                continue
            rec = {"file": rel, "line": lineno, "raw": tgt, "resolved": resolved}
            raw.append(rec)
            if offset_ok(rel, tgt, root, have):          # 登记 ⇒ 按基准重解析 + 现读复核
                exempt.append(rec)
            else:
                hits.append(rec)
    stale = offset_registry_check(root, files, raw)
    return len({h["file"] for h in hits}), hits, exempt, stale, ""


def local_broken(root: str):
    """一格：本地断链（域 = 一棵树）⇒ `(件数, 明细, 判不了理由)`（兼容三元形态）。"""
    n, hits, _ex, _stale, why = local_broken_detail(root)
    return n, hits, why


def orphans(tree: str):
    """二格：孤儿（域 = 导出树）⇒ (件数, 明细, 判不了理由)。"""
    if tree is None or not os.path.isdir(tree):
        return None, [], "导出树不在盘：%s" % (tree if tree else "（没给）")
    allf = set()
    for dp, _dns, fns in tree_walk(tree):
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

遮蔽（抽取**前置**一步 · **不是**判据 · 2026-09-26 重测落的）
------------------------------------------------------------
  · **围栏代码块**（` ``` ` / `~~~` 起止）**整块** + **行内码 span**（反引号成对 · 支持多反引号定界）
    ⇒ 抽取前换成**等长空白**（保行号/列偏移）⇒ 块内的 `[字](目标)` **不进**
    （那两处是**被引用的文本**：别仓坏链原话 / 链接语法示例 / 修法脚本里的字符串字面量）
  · 遮蔽**之外**的同一段坏文本照旧判 —— 成对负控钉住：**块内不报 ∧ 块外必报**（本仓现读：19 处 = 围栏 8 + 行内码 11，全假红）
  · 口径与 `check-zh-en.py` 的 `strip_noncode` 同族（先剥引用/代码，再判正文）—— **复用已有管线**、不加新机制

域里**不进**（2026-09-26 重测落的）
----------------------------------
  · **嵌套仓**（目录里有 `.git` = 子模块工作树 / 半克隆）⇒ 那是**别仓的件**，本门判不了也不该判
    ⇒ 剪掉并**照实打印**（登记一行 · 不静默）：本仓 `vendor/rtk` = 子模块 `rtk-ai/rtk`（父仓只跟踪 gitlink）
  · 已有剪枝目录（`.git` / `node_modules` / `venv` / `.venv` / `__pycache__`）照旧

基准偏移登记（`BASE_OFFSET` · **收窄判据** · 与 `check-links-lychee.py` 同式样）
------------------------------------------------------------------------------
  · 键 = 件相对**树根**路径（**无行号锚**）· 值 = `(基准目录, 理由)`
  · 命中登记 ⇒ **按登记基准重解析 + 现读存在性复核**：复核命中 ⇒ 记「已登记假红（基准偏移 · 已复核）」· **不计入断链数**（逐条印落点）
  · 复核落空 ⇒ **照旧判红**（豁免只挡「基准错」这一半）· 登记**过期** ⇒ **rc=2 + 逐字点名**（基准指空 / 件在域内却 0 原位错）

未覆盖（不假装全覆盖）
----------------------
  · 目录型链接（指向目录而非件）只判「目录在不在」，不追目录内默认件
  · 非 `.md` 件里的链接（.html/.txt/.py）**不扫**（本题面只管文档）
  · 另一棵树里的**导出树**（跨仓的产出树）不判
  · **嵌套仓内部**（子模块自己的文档）不判 —— 域外（见上「域里不进」）
  · nav 白名单 / `mkdocs.yml` 那一半 = `W-06`（2026-09-24 **波6 序91 已落** ⇒ 落成上面 ③④ 两格；
    **本仓无 `mkdocs.yml`** 那一行**照实登记**、不假装覆盖）

退码
----
  0 = 判得了（命中多少都不改码 —— 「先只报不拦」）
  2 = 不给结论（仓根缺 / 扫面 0 件 / 件读不了 / **基准偏移登记过期** / 自检不过 /
                **导出树读不到** —— 缺件 ≠ 绿）
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

    # ── ⑧⑨ 遮蔽：围栏块（成对 —— 块内不报 ∧ 块外必报）────────────────────────────
    t8 = os.path.join(tmp, "t8")
    os.makedirs(t8, exist_ok=True)
    with open(os.path.join(t8, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n\n```sh\npython3 x.py '[坏](gone.md)'\n```\n")   # 围栏里：被引用的文本
    case("⑧ 遮蔽·围栏块里的路径 ⇒ **不报**（不是链接）", 0, local_broken(t8)[0])
    with open(os.path.join(t8, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n\n[坏](gone.md)\n\n```sh\npython3 x.py '[也坏](gone2.md)'\n```\n")  # 同一段坏文本：块外一处
    n8, h8, _w8 = local_broken(t8)
    case("⑨ 遮蔽（成对）：块外那一处**照报**（件数 1 · 只报块外那处）", (1, ["gone.md"]),
         (n8, [h["raw"] for h in h8]))

    # ── ⑩⑪ 遮蔽：行内码 span（成对）──────────────────────────────────────────────
    t9 = os.path.join(tmp, "t9")
    os.makedirs(t9, exist_ok=True)
    with open(os.path.join(t9, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n\n抽 `[字](目标)` 时把 ``[坏](gone.md)`` 也抽走了。\n")
    case("⑩ 遮蔽·行内码 span 里的路径 ⇒ **不报**", 0, local_broken(t9)[0])
    with open(os.path.join(t9, "a.md"), "w", encoding="utf-8") as f:
        f.write("# a\n\n真链接 [坏](gone.md)（另有行内码 ``[x](gone2.md)``）\n")
    n9, h9, _w9 = local_broken(t9)
    case("⑪ 遮蔽（成对）：行内码外的真断链**照报**（件数 1 · 只报码外那处）", (1, ["gone.md"]),
         (n9, [h["raw"] for h in h9]))

    # ── ⑫⑬ 域：嵌套仓（子模块工作树 = 别仓的件）（成对）──────────────────────────
    t10 = os.path.join(tmp, "t10")
    os.makedirs(os.path.join(t10, "vendor", "sub"), exist_ok=True)
    with open(os.path.join(t10, "vendor", "sub", ".git"), "w", encoding="utf-8") as f:
        f.write("gitdir: /elsewhere/.git/modules/sub\n")     # 子模块工作树：`.git` 是**文件**
    with open(os.path.join(t10, "vendor", "sub", "doc.md"), "w", encoding="utf-8") as f:
        f.write("# v\n\n[坏](gone.md)\n")                     # 别仓的件里的真断链 ⇒ 域外
    case("⑫ 域·嵌套仓（子模块工作树）里的件 ⇒ **不进域、不报**", 0, local_broken(t10)[0])
    with open(os.path.join(t10, "doc.md"), "w", encoding="utf-8") as f:
        f.write("# v\n\n[坏](gone.md)\n")                     # 同一段坏文本，但**在域内**
    n10, h10, _w10 = local_broken(t10)
    case("⑬ 域（成对）：同一段坏文本在域内 ⇒ **必报**（件数 1）", (1, ["doc.md"]),
         (n10, [h["file"] for h in h10]))

    # ── ⑭⑮ 基准偏移登记（收窄判据 · 成对：复核命中不报 ∧ 复核落空照报）────────────
    t11 = os.path.join(tmp, "t11")
    os.makedirs(os.path.join(t11, "publish", "docs"), exist_ok=True)
    with open(os.path.join(t11, "publish", "docs", "QUICKSTART.en.md"), "w", encoding="utf-8") as f:
        f.write("# q\n")
    with open(os.path.join(t11, "publish", "LICENSE"), "w", encoding="utf-8") as f:
        f.write("Apache-2.0\n")
    with open(os.path.join(t11, "publish", "docs", "README.md"), "w", encoding="utf-8") as f:
        f.write("# r\n\n见 [q](docs/QUICKSTART.en.md) 与 [lic](LICENSE)。\n")   # 按公开树仓根写
    _n, _red, _ex, _st, _w = local_broken_detail(t11)
    case("⑭ 登记·按基准复核命中 ⇒ **不报**（红 0 · 已登记假红 2 处 · 登记未过期）", (0, 2, 0),
         (_n, len(_ex), len(_st)))
    with open(os.path.join(t11, "publish", "docs", "README.md"), "w", encoding="utf-8") as f:
        f.write("# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n也见 [x](docs/NOPE.md)。\n")
    _n, _red, _ex, _st, _w = local_broken_detail(t11)
    case("⑮ 登记（成对）：基准也指不到的 ⇒ **照旧判红并点名**", (1, ["docs/NOPE.md"]),
         (_n, [h["raw"] for h in _red]))

    # ── ⑯⑰ 登记体检（本改动的牙）：两种过期态**必须出声**─────────────────────────
    t12 = os.path.join(tmp, "t12")
    os.makedirs(os.path.join(t12, "publish", "docs"), exist_ok=True)
    with open(os.path.join(t12, "publish", "docs", "QUICKSTART.en.md"), "w", encoding="utf-8") as f:
        f.write("# q\n")
    with open(os.path.join(t12, "publish", "docs", "README.md"), "w", encoding="utf-8") as f:
        f.write("# r\n\n见 [q](QUICKSTART.en.md)。\n")          # 原位就可达 ⇒ 一条原位错都没有
    _n, _red, _ex, _st, _w = local_broken_detail(t12)
    case("⑯ 登记体检①：登记件在域内却 0 原位错 ⇒ 判**登记过期**并点名", (0, 1),
         (_n, 1 if any("请撤登记" in s for s in _st) else len(_st)))
    _old = BASE_OFFSET["publish/docs/README.md"]
    BASE_OFFSET["publish/docs/README.md"] = ("no-such-base", "夹具：基准指空")
    try:
        _st2 = offset_registry_check(t12, {"publish/docs/README.md": "x"},
                                     [{"file": "publish/docs/README.md"}])
    finally:
        BASE_OFFSET["publish/docs/README.md"] = _old
    case("⑰ 登记体检②：登记基准**不是目录** ⇒ 点名（基准指空）", True,
         bool(_st2) and "不是目录" in _st2[0])

    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True, description="文档孤儿 / 本地断链两格（只报不拦）")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--docs-root", default=None,
                    help="第二棵树（默认 = 同级目录 Zerg-内部文档；不在盘 ⇒ 照实标缺）")
    ap.add_argument("--tree", default=None,
                    help="导出树（默认 $%s · 兼容别名 $%s；都没有 ⇒ 不给结论）"
                         % (PUB_TREE_ENV, PUB_TREE_ENV_ALIAS))
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
    tree, tree_src = resolve_tree(a.tree)   # 次序：--tree → $ZERG_PUB_TREE → $ZERG_PUBLIC_TREE → 无

    sys.stdout.write("文档孤儿 / 本地断链（两格 · 只报不拦）· v%s\n" % VERSION)
    sys.stdout.write("  判据见 `--list-rules`：① local_broken（域 = 两棵树）· ② orphans（域 = 导出树）\n")
    sys.stdout.write("  域① 主仓根 = %s\n" % a.repo_root)
    sys.stdout.write("  域① 第二棵树 = %s%s\n" % (docs_root, "" if os.path.isdir(docs_root) else "  ★不在盘（该树照实标缺 · 不影响另一棵）"))
    sys.stdout.write("  域② 导出树 = %s（来源 %s）%s\n"
                     % (tree if tree else "（未给）", tree_src or "无",
                        "" if (tree and os.path.isdir(tree))
                        else "  ★读不到（孤儿格 = 判不了 ⇒ 本门不给结论 rc=2）"))

    bad = 0
    br_counts = {}
    br_all = []
    br_exempt = {}
    br_stale = []
    for label, root in (("主仓", a.repo_root), ("第二棵树", docs_root)):
        n, hits, exempt, stale, why = local_broken_detail(root)
        br_counts[label] = n
        br_exempt[label] = exempt
        br_stale.extend(stale)
        if n is None:
            sys.stdout.write("  ⛔ 本地断链 · %s：判不了（%s）\n" % (label, why))
            if label == "主仓":
                bad = 1
        else:
            sys.stdout.write("  ✓ 本地断链 · %s：%d 件\n" % (label, n))
            for h in hits[: a.max_examples]:
                sys.stdout.write("      %s:%d  [%s] ⇒ %s（指不到）\n" % (h["file"], h["line"], h["raw"], h["resolved"]))
            br_all.extend(hits)
            if exempt:   # 已登记假红**逐条印出落点**（看得见，不是静默放行）
                sys.stdout.write("      ⊕ 已登记假红（基准偏移 · 已逐条复核 · 不计入上面 %d 处）：\n"
                                 % len({h["file"] for h in hits}))
                for h in exempt[: a.max_examples]:
                    sys.stdout.write("          %s:%d  [%s] ⟸ %s\n"
                                     % (h["file"], h["line"], h["raw"], offset_ok(h["file"], h["raw"], root, set())))
        for nested in NESTED_SEEN.get(root, []):
            sys.stdout.write("      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n" % nested)
    if len(br_all) > a.max_examples * 2:
        sys.stdout.write("      （明细截断：共 %d 处 · 上面每树各取前 %d 行）\n" % (len(br_all), a.max_examples))
    if br_stale:   # ★ 登记失效必须出声（登记体检 · 牙）⇒ 不给结论
        sys.stdout.write("  ⛔ 基准偏移登记过期 %d 条 —— 登记指不到现实（逐字点名）：\n" % len(br_stale))
        for s in br_stale:
            sys.stdout.write("      · %s\n" % s)
        sys.stdout.write("  ⇒ 不给结论（rc=2）—— 登记过期不许静默（治法：撤或改 `BASE_OFFSET`）\n")
        return 2

    on, ohits, owhy = orphans(tree)
    for nested in NESTED_SEEN.get(tree, []):
        sys.stdout.write("      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n" % nested)
    if on is None:
        sys.stdout.write("  ⛔ 孤儿 · 导出树：判不了（%s）—— 这一格判不了 ≠ 整门判不了；"
                         "★ 导出树是本门判据的必需输入 ⇒ **缺件 ≠ 绿**：本门不给结论（rc=2）\n" % owhy)
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
    for nested in NESTED_SEEN.get(docs_root, []):
        sys.stdout.write("      ⊘ 剪掉的嵌套仓（域外 · 别仓的件）：%s\n" % nested)
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
    if on is None:   # ★ 导出树读不到 ⇒ 孤儿格判不了 · 缺件 ≠ 绿 ⇒ 本门不给结论（零放宽）
        sys.stdout.write("\n结论：**不给结论（rc=2）** —— 导出树读不到（%s）\n"
                         "      取树次序 `--tree` → `$%s` → `$%s`（与 B12/B14/B16/B17 同口径 · 缺件 ≠ 绿）\n"
                         % (owhy, PUB_TREE_ENV, PUB_TREE_ENV_ALIAS))
        return 2
    ex_all = {k: len(v) for k, v in br_exempt.items()}
    sys.stdout.write("\n结论（只报不拦）：本地断链 主仓 %s 件 / 第二棵树 %s 件 · "
                     "已登记假红（基准偏移）主仓 %d 处 / 第二棵树 %d 处（已逐条复核 · 不计）· 孤儿 %s 件 · "
                     "%s 第二棵树整棵 %s 件 · %s 本仓 %s ⇒ rc=0（命中不改码 ✗）\n"
                     % (br_counts.get("主仓"), br_counts.get("第二棵树"),
                        ex_all.get("主仓", 0), ex_all.get("第二棵树", 0), on,
                        inz_word, inz, nav_word, nav_n))
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1",
            "gate": "check-docs-orphans-links",
            "version": VERSION,
            "local_broken": {"count": sum(x for x in br_counts.values() if isinstance(x, int)),
                             "by_tree": br_counts},
            "offset_exempt": {"count": sum(ex_all.values()), "by_tree": ex_all,
                              "registry": sorted(BASE_OFFSET), "stale": []},
            "domain_pruned": {r: NESTED_SEEN.get(r, []) for r in (a.repo_root, docs_root, tree)},
            "orphans": {"count": on, "domain": tree},
            "inbound_zero": {"count": inz, "domain": docs_root, "word": inz_word},
            "not_in_nav": {"count": nav_n, "domain": a.repo_root if has_mkdocs(a.repo_root) else None,
                           "word": nav_word, "why": nav_why or None},
            "mode": "report-only",
        }, ensure_ascii=False) + "\n")
    return 0 if bad == 0 else 2


if __name__ == "__main__":
    sys.exit(main())

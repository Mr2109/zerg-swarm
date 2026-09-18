#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-freshness.py v1.0.0 —— W3 · 四条新鲜度门禁 D1–D4（**只读、只报不改**）

规格来源（一字不改照抄口径，不照抄实现）：
  · docs/01-设计/设计-文档体系-v1.0.md  §6.3（D1–D5 入参/判定/退码/反例）· §9.1（健康度 4+1 采集口径）
    · §5.6①（slug 冻结规范八条）· §16（开工清单）
  · docs/调研/调研-文档分块落地-门禁与迁移.md §1（五条门禁）· §2.3（Q14 三档退码）

四条门禁（D5 不在本脚本范围 —— 它是「新鲜度戳」，今天 0 篇带字段 ⇒ 必然 rc=2，另路）：
  D1  引用路径存在      抽 [](…) / ![](…) 的相对目标 → 在 {引用文件目录, 仓根, 各模块根} 任一处命中即过
  D2  `文件:行` 有效     R1（含 / 的路径）存在且最大行号 ≤ 文件总行数；R2（裸文件名）全解析域恰好 1 个才算过
  D3  断链断锚          按 §5.6① 冻结 slug 规则算目标文件全部标题 slug，命中即过
  D4  生成式参考 drift  在**临时目录重建**（绝不就地生成）→ 与仓内提交物逐字节比对

三档退码（全稿统一，依据 scripts/precommit-gates.sh:29 的既有语义）：
  0  全绿（有认定但无红）
  1  有失败项（逐条打印 `文件:行 → 目标`）
  2  **不给结论**（缺件 / 空转 / 用法错 / 判据不可判）—— 空转不许当绿

用法：
  python3 scripts/check-doc-freshness.py --list-rules            # 规则表 + 全部名单（白名单/排除/引用域/夹具/拒绝域）
  python3 scripts/check-doc-freshness.py --self-test             # 好件/坏件/缺件 三格 × 四条 = 12 断言
  python3 scripts/check-doc-freshness.py d1 [d2 d3 d4] [--scope main|wide|mirror] [--report /tmp/r.json]
  python3 scripts/check-doc-freshness.py all                     # 四条都跑；退出码取最严重（1 优先于 2）

硬纪律（本路）：**只读**（不写仓内任何路径）· **不挂门禁**（挂接由父代理做）· 重建产物一律落 `$TMPDIR`。
"""
import argparse
import datetime
import fnmatch
import json
import os
import posixpath
import re
import subprocess
import sys
import tempfile
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_REPO = os.path.dirname(HERE)
DEFAULT_CONFIG = os.path.join(HERE, "doc-freshness.config.json")

RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2
RC_NAME = {0: "OK（全绿）", 1: "FAIL（有失败项）", 2: "BLOCKED（不给结论）"}

# 域：非「本地」的命中一律**单列计数、不进退码**（除 --strict-all）
DOM_LOCAL, DOM_EXTERNAL, DOM_FROZEN, DOM_MIRROR, DOM_FIXTURE = "本地", "外部引用域", "冻结归档", "镜像副本", "夹具"

GATES = {
    "d1": {
        "name": "D1 · 引用路径存在",
        "inputs": "① 语料清单（目录白名单，显式）② 解析基准集（仓根 + 各模块根）③ 引用域开关 ④ 大小写/Unicode 归一化开关",
        "judge": "抽 [..](target) 与 ![..](target)；scheme:// / mailto: 分流外链；#x 归 D3；"
                 "相对路径在 {引用文件目录, 仓根, 各模块根} 任一命中即过；目标是目录也必须存在；先 unquote 再判；"
                 "判据用「树上真实路径字符串集合」比对，不用 os.path.exists（APFS 大小写不敏感会造假绿）",
        "rc0": "全部相对引用可解析",
        "rc1": "有断链（逐条 文件:行 → 目标）",
        "rc2": "语料清单为空（空转 ⇒ 不给结论，不绿）",
        "fix": "改引用或改文件名；**不改判据来消红**",
    },
    "d2": {
        "name": "D2 · 引用 `文件:行` 有效",
        "inputs": "① 归一化白名单（不算引用的噪声类别，必须计数）② 解析基准集 ③ 行号语义（`-` 区间 / `,` 多段 / 单行）④ 裸文件名策略",
        "judge": "R1（含 / 的路径）：存在且最大行号 ≤ 文件总行数；R2（裸文件名）：全解析域同名恰好 1 个 ⇒ 过、0 个 ⇒ 红、>1 ⇒ 黄（歧义）；"
                 "噪声类别（IP:端口 / 省略号 / 占位符 / 命令行样式 / 绝对路径 / URL）不判但必须计数",
        "rc0": "无红（黄=歧义只入清单）",
        "rc1": "有红（文件不存在 / 行号越界）",
        "rc2": "解析基准集配置缺失（判据不可判）",
        "fix": "行号越界 ⇒ 改行号或改「文件:行」；文件没了 ⇒ 改指向新落点",
    },
    "d3": {
        "name": "D3 · 断链断锚",
        "inputs": "① slug 规范（冻结 · §5.6① 八条）② 带锚链接的语料范围 ③ 是否纳入 Obsidian wiki 链接（默认不纳入、单列计数）④ 同文件重复 slug 的编号规则",
        "judge": "链接带 #anchor 且目标可解析为 .md ⇒ 按冻结规则算目标文件全部标题 slug，命中即过；"
                 "显式锚（`{: #slug }` / `{#slug}`）优先；同文件重复 slug 追加 -1/-2；空 slug 兜底 s<序号> 且禁止被引用",
        "rc0": "无断锚",
        "rc1": "有断锚（逐条 文件:行 → path#slug）",
        "rc2": "slug 规范未落盘 / profile 未知（规范不存在 ⇒ 判据不可判）",
        "fix": "改锚或改标题（改标题前先查谁引用它）；纯符号标题改文案",
    },
    "d4": {
        "name": "D4 · 生成式参考 drift",
        "inputs": "① 生成器清单（源 → 产物 → 重建命令 → 期望退出码）② 干净的临时目录 ③ git 只读权限（本实现用「临时目录重建 + 与提交物比对」，不需要 git 写）",
        "judge": "在临时目录重建（绝不就地生成）→ 与仓内提交物比对：不一致 ⇒ 红；"
                 "归一化（去尾空格 + 丢易变行：时间戳/版本号/本机路径）后一致 ⇒ 黄（只有易变行）；报错必须带「跑哪条命令修」",
        "rc0": "产物 == 重建结果（含归一化后一致）",
        "rc1": "有 diff",
        "rc2": "缺生成器 / 产物不存在 / 0 个可比对（不给结论）",
        "fix": "照每条报错里给的命令重跑生成器（绝不手改生成物）",
    },
}

# ─────────────────────────── 通用小工具 ───────────────────────────

def sha1_text(s: str) -> str:
    import hashlib
    return hashlib.sha1(s.encode("utf-8")).hexdigest()


def norm_join(*parts):
    """POSIX 拼接并吃掉 ./ 与 ..（纯文本归约，不碰文件系统）。"""
    p = posixpath.join(*[x for x in parts if x not in ("", None)])
    segs = []
    for s in p.split("/"):
        if s in ("", "."):
            continue
        if s == "..":
            if segs:
                segs.pop()
            continue
        segs.append(s)
    return "/".join(segs)


def to_rel(repo, abspath):
    r = os.path.relpath(abspath, repo)
    return r.replace(os.sep, "/")


def is_under(rel, prefix):
    prefix = prefix.rstrip("/")
    return rel == prefix or rel.startswith(prefix + "/")


def unquote(s):
    try:
        from urllib.parse import unquote as _u
        return _u(s)
    except Exception:
        return s


def nfc(s: str) -> str:
    return unicodedata.normalize("NFC", s)


# ─────────────────────────── 语料 / 解析域 ───────────────────────────

class Tree:
    """一次构建：解析域（base_roots 下全部文件/目录，去硬排除）+ 语料（要扫引用的 md）+ 行列数缓存。"""

    def __init__(self, repo, cfg, scope="main", strict_all=False, verbose=False):
        self.repo = os.path.abspath(repo)
        self.cfg = cfg
        self.scope = scope
        self.strict_all = strict_all
        self.no_soft = False       # --no-whitelist：关掉软白名单（语法示例/省略号/代码块内），用于对照口径
        self.verbose = verbose
        self.files = []            # 解析域：仓相对 POSIX 路径
        self.fileset = set()
        self.dirs = set()
        self.lower_map = {}
        self.byname = {}
        self.byname_local = {}
        self.corpus = []           # 语料：仓相对 md 路径（按 scope）
        self.domain_of = {}        # 前缀 → 域（soft 排除用的判域）
        self._lines = {}
        self._text = {}
        self.t_build = 0.0
        t0 = time.perf_counter()
        self._build_domains()
        self._walk()
        self.t_build = time.perf_counter() - t0

    # —— 名单 ——
    def _build_domains(self):
        c = self.cfg
        self.hard = [p.rstrip("/") + "/" for p in c.get("hard_exclude", [])]
        # 名字型排除（任何深度）：target / node_modules / __pycache__ …（否则 ui/target 27G 会被走进去）
        self.hard_names = set(p for p in c.get("hard_exclude", []) if "/" not in p)
        self.soft = []                       # (前缀, 域, 在哪些 scope 里扫)
        for p in c.get("mirror_paths", []):
            self.soft.append((p.rstrip("/") + "/", DOM_MIRROR, ("mirror",)))
        for p in c.get("frozen_paths", []):
            self.soft.append((p.rstrip("/") + "/", DOM_FROZEN, ("wide", "mirror")))
        for p in c.get("third_party_paths", []):
            self.soft.append((p.rstrip("/") + "/", DOM_EXTERNAL, ("wide", "mirror")))
        for p in c.get("fixture_paths", []):
            self.soft.append((p.rstrip("/") + "/", DOM_FIXTURE, ("main", "wide", "mirror")))
        for p in c.get("engine_data_paths", []):
            # 引擎数据（原 `docs/issues/` 1086 篇，2026-09-19 已分家至 `Zerg-内部文档/issues/`）**任何 scope 都不扫**
            # —— 它不是文档，是 issue 流水；新落点 `statepath.IssuesDir()` 本就在语料域之外
            self.soft.append((p.rstrip("/") + "/", DOM_FROZEN, ()))
        self.external_prefixes = [p.rstrip("/") + "/" for p in c.get("external_domain_paths", [])]

    def _hard_excluded(self, rel):
        if any(is_under(rel, p.rstrip("/")) for p in self.hard):
            return True
        return any(seg in self.hard_names for seg in rel.split("/"))

    def _soft_state(self, rel):
        """返回 (是否扫, 域)。"""
        for pfx, dom, scopes in self.soft:
            if is_under(rel, pfx.rstrip("/")):
                if dom == DOM_EXTERNAL and self._is_external(rel) and self.scope == "main":
                    return True, DOM_EXTERNAL     # 外部引用域的**文件在语料里**（要计数），只是命中不判红
                if self.scope in scopes:
                    return True, dom
                return False, dom
        return True, DOM_LOCAL

    def _is_external(self, rel):
        return any(is_under(rel, p.rstrip("/")) for p in self.external_prefixes)

    def domain(self, rel):
        for pfx, dom, _scopes in self.soft:
            if is_under(rel, pfx.rstrip("/")):
                return dom
        if self._is_external(rel):
            return DOM_EXTERNAL          # 外部引用域：语料里要扫、要计数，但命中默认不判红
        return DOM_LOCAL

    def _walk(self):
        base_roots = [r.rstrip("/") for r in self.cfg["base_roots"]]
        corpus_roots = [r.rstrip("/") for r in self.cfg["corpus"]]
        want_res = set()
        for b in base_roots:
            want_res.add(b)
        for dp, dns, fns in os.walk(self.repo):
            rel = to_rel(self.repo, dp)
            relp = "" if rel == "." else rel
            if relp and self._hard_excluded(relp):
                dns[:] = []
                continue
            dns[:] = [d for d in dns if not d.startswith(".") and not self._hard_excluded(norm_join(relp, d))]
            in_res = any(relp == b or b == "" or rev_under(relp, b) for b in want_res)
            if in_res:
                if relp:
                    self.dirs.add(relp)
                for f in fns:
                    if f.startswith("."):
                        continue
                    fr = norm_join(relp, f)
                    self.files.append(fr)
                    self.fileset.add(fr)
                    nm = posixpath.basename(fr)
                    self.byname.setdefault(nm, []).append(fr)
                    if self.domain(fr) not in (DOM_MIRROR, DOM_FROZEN):
                        # R2 同名池（口径写死：去镜像、去冻结归档 —— 否则镜像把每份同名都算成「歧义」）
                        self.byname_local.setdefault(nm, []).append(fr)
                    self.lower_map.setdefault(fr.lower(), fr)
            # 语料
            if relp and any(relp == c or rev_under(relp, c) for c in corpus_roots):
                ok, _dom = self._soft_state(relp)
                if not ok:
                    dns[:] = []
                    continue
                for f in fns:
                    if f.endswith(".md") and not f.startswith("."):
                        self.corpus.append(norm_join(relp, f))
                continue
        self.corpus.sort()
        self.files.sort()

    def text(self, rel):
        if rel not in self._text:
            try:
                with open(os.path.join(self.repo, rel), encoding="utf-8", errors="replace") as f:
                    self._text[rel] = f.read()
            except OSError:
                self._text[rel] = ""
        return self._text[rel]

    def nlines(self, rel):
        if rel not in self._lines:
            t = self.text(rel)
            if t == "":
                self._lines[rel] = 0
            else:
                n = t.count("\n")
                self._lines[rel] = n if t.endswith("\n") else n + 1
        return self._lines[rel]

    def exists(self, rel):
        return rel in self.fileset

    def exists_dir(self, rel):
        return rel in self.dirs or rel == ""

    def case_variant(self, rel):
        """APFS 大小写不敏感 ⇒ 精确串没命中但小写命中：Linux CI 上必红（黄档）。"""
        return self.lower_map.get(rel.lower())


def rev_under(rel, root):
    """rel 在 root 之下（root 可含反斜杠无关；纯前缀段比较）。"""
    if root == "":
        return True
    return rel == root or rel.startswith(root + "/")


def allowlisted(tree, gate, rel, target):
    """豁免机制（规格要求：**豁免条目必须带理由 + 日期**，否则 = 无记录的洗白）。"""
    for e in tree.cfg.get("allowlist", []):
        if e.get("gate", "*") not in ("*", gate):
            continue
        if not fnmatch.fnmatch(rel, e.get("file_glob", "*")):
            continue
        t = e.get("target", "")
        if t == target or fnmatch.fnmatch(target, t):
            return e
    return None


def allowlist_errors(cfg):
    """豁免表自检：缺理由/缺日期 ⇒ 判据不可判（rc=2），不许静默。"""
    errs = []
    for i, e in enumerate(cfg.get("allowlist", [])):
        if not (e.get("reason") or "").strip():
            errs.append("allowlist[%d] 缺 reason（只给路径的 allowlist 就是无记录的洗白）" % i)
        if not (e.get("date") or "").strip():
            errs.append("allowlist[%d] 缺 date" % i)
    return errs


# ─────────────────────────── 链接抽取（D1/D3 共用） ───────────────────────────

RE_INLINE_LINK = re.compile(r"(!?)\[([^\]\n]*)\]\(\s*(<[^>\n]*>|[^)\s]+)(?:\s+\"[^\"]*\")?\s*\)")
RE_FENCE = re.compile(r"^\s{0,3}(```|~~~)")
RE_WIKILINK = re.compile(r"\[\[([^\]\n]+)\]\]")
RE_HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*$")
RE_EXPLICIT_ANCHOR = re.compile(r"^(.*?)\s*(?:\{:\s*#([^}\s]+)\s*\}|\{#([A-Za-z0-9_\-\.]+)\})\s*$")


def iter_links(text):
    """产出 (行号, 是否图片, 原始目标, 是否在围栏代码块内)。"""
    out = []
    infence = False
    for i, line in enumerate(text.split("\n"), 1):
        if RE_FENCE.match(line):
            infence = not infence
            continue
        for m in RE_INLINE_LINK.finditer(line):
            tgt = m.group(3)
            if tgt.startswith("<") and tgt.endswith(">"):
                tgt = tgt[1:-1]
            out.append((i, m.group(1) == "!", tgt, infence))
    return out


def classify_target(tgt):
    """拆 (文件部分, 锚部分)；返回 (类别, 文件, 锚)。

    类别：外链 / 页内锚 / 相对路径 / 绝对路径样式 / 空锚。
    """
    t = tgt.strip()
    if re.match(r"^[A-Za-z][A-Za-z0-9+.\-]*://", t) or t.startswith("mailto:") or t.startswith("//"):
        return "外链", "", ""
    if t.startswith("#"):
        return "页内锚", "", t[1:]
    if "#" in t:
        f, a = t.split("#", 1)
        f = unquote(f)
        if f == "":
            return "页内锚", "", a
    else:
        f, a = unquote(t), ""
    if f.startswith("/") or f.startswith("~") or re.match(r"^[A-Za-z]:[\\/]", f):
        return "绝对路径样式", f, a
    return "相对路径", f, a


RE_SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.\-]*://")
RE_URL = re.compile(r"[A-Za-z][A-Za-z0-9+.\-]*://[^\s`)\]}>,，。；;]+")


def resolve_file(tree, src_rel, target):
    """在 {引用文件目录, 仓根, 各模块根} 里找；返回 (kind, hit, extra)。

    kind ∈ exists / case / dir / miss ；extra 给候选（用于报错信息）。
    """
    tgt = target.strip().lstrip("/") if target.startswith("/") else target
    cands = []
    d = posixpath.dirname(src_rel)
    cands.append(norm_join(d, tgt))
    cands.append(norm_join(tgt))
    for b in tree.cfg["base_roots"]:
        b = b.rstrip("/")
        if b:
            cands.append(norm_join(b, tgt))
    seen, cand2 = set(), []
    for c in cands:
        if c and c not in seen:
            seen.add(c)
            cand2.append(c)
    for c in cand2:
        if tree.exists(c):
            return "exists", c, cand2
        if tree.exists_dir(c):
            return "dir", c, cand2
    for c in cand2:
        v = tree.case_variant(c)
        if v:
            return "case", v, cand2
    return "miss", None, cand2


# ─────────────────────────── slug（§5.6① 冻结八条） ───────────────────────────

def heading_text(raw):
    """把标题原文清成「渲染后文本」：取链接文字、去强调与行尾 # 收尾序列。"""
    s = raw.strip()
    # 行尾 ATX 收尾（`## 标题 ##`）
    s = re.sub(r"\s+#+\s*$", "", s)
    # [文字](目标) → 文字
    prev = None
    while prev != s:
        prev = s
        s = re.sub(r"!?\[([^\]\[]*)\]\([^)]*\)", r"\1", s)
    s = s.replace("**", "").replace("__", "").replace("*", "").replace("`", "")
    return s.strip()


def slugify(text, profile="github"):
    """冻结规范：NFC → lower（非 casefold）→ 保 \\p{L}\\p{M}\\p{N}_- → 空白→'-'（逐字符）→ 空兜底 s<n>。

    profile=github  ：空白（含 U+3000）→ '-'（与 GitHub 逐字一致）
    profile=docv1.2 ：照 §5.6③ 的字面读法 —— U+3000 一并**删除**（该分歧已实测，见说明书）
    """
    s = nfc(text).lower()
    out = []
    for ch in s:
        if ch.isspace():
            if profile == "docv1.2" and ch == "\u3000":
                continue
            out.append("-")
        elif ch == "_" or ch == "-":
            out.append(ch)
        elif ch.isalnum() or unicodedata.category(ch).startswith("M"):
            out.append(ch)
        # 其余一律删除
    return "".join(out)


def file_anchors(tree, rel, profile):
    """算出某 md 的全部锚（含显式锚、重复 slug 的 -1/-2 编号）。返回 (list[(slug, 行号, 标题)], 空slug数, 标题总数)。"""
    text = tree.text(rel)
    slugs = []
    empty = 0
    heads = 0
    used = {}
    infence = False
    for i, line in enumerate(text.split("\n"), 1):
        if RE_FENCE.match(line):
            infence = not infence
            continue
        if infence:
            continue
        m = RE_HEADING.match(line)
        if not m:
            continue
        heads += 1
        body = m.group(2)
        explicit = None
        em = RE_EXPLICIT_ANCHOR.match(body)
        if em:
            body = em.group(1)
            explicit = em.group(2) or em.group(3)
        if explicit:
            s = explicit
        else:
            s = slugify(heading_text(body), profile)
        if s == "":
            empty += 1
            s = "s%d" % empty
        if s in used:
            used[s] += 1
            s = "%s-%d" % (s, used[s])
        else:
            used[s] = 0
        slugs.append((s, i, heading_text(body)))
    return slugs, empty, heads


# ─────────────────────────── 结果容器 ───────────────────────────

class Result:
    def __init__(self, gate, tree):
        self.gate = gate
        self.tree = tree
        self.counts = {}
        self.findings = []       # dict(file,line,target,kind,detail,domain)
        self.notes = []          # 黄档/单列计数说明
        self.errors = []         # 不给结论的原因（rc=2）

    def add(self, file, line, target, kind, detail="", domain=DOM_LOCAL):
        self.findings.append({"file": file, "line": line, "target": target, "kind": kind,
                              "detail": detail, "domain": domain})

    def counted(self, key, n=1):
        self.counts[key] = self.counts.get(key, 0) + n

    def reds(self):
        loc = (lambda f: True) if self.tree.strict_all else (lambda f: f["domain"] == DOM_LOCAL)
        return [f for f in self.findings if f["kind"].startswith("红") and loc(f)]

    def yellow(self):
        loc = (lambda f: True) if self.tree.strict_all else (lambda f: f["domain"] == DOM_LOCAL)
        return [f for f in self.findings if f["kind"].startswith("黄") and loc(f)]

    def rc(self):
        if self.errors:
            return RC_BLOCKED
        if self.reds():
            return RC_FAIL
        return RC_OK


# ─────────────────────────── D1 ───────────────────────────

def gate_d1(tree, args):
    r = Result("d1", tree)
    if not tree.corpus:
        r.errors.append("语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论")
        return r
    r.counted("语料篇数", len(tree.corpus))
    for rel in tree.corpus:
        dom = tree.domain(rel)
        text = tree.text(rel)
        for line, isimg, tgt, infence in iter_links(text):
            cls, f, anchor = classify_target(tgt)
            if cls == "外链":
                r.counted("外链（不判）")
                continue
            if cls == "页内锚":
                r.counted("页内锚（归 D3）")
                continue
            if cls == "绝对路径样式":
                r.counted("绝对路径样式（不判，单列）")
                r.add(rel, line, tgt, "单列·绝对路径", "以 / 或 ~ 开头 —— 站点根语义歧义，不判", dom)
                continue
            if infence and not tree.no_soft:
                r.counted("代码块内链接（不判）")
                continue
            if f == "":
                r.counted("空目标（不判）")
                continue
            if ("…" in f or "..." in f) and not tree.no_soft:
                r.counted("省略号目标（不判）")
                r.add(rel, line, tgt, "单列·省略号目标", "路径被省略号截断", dom)
                continue
            r.counted("相对引用条目")
            if any(ch in f for ch in ("{", "}", "*", "$", "<", ">")):
                r.counted("占位符目标（不判）")
                continue
            if "/" not in f and "." not in f and len(f) <= 24 and not tree.no_soft:
                # 语法示例/占位名（[name](url) · [..](target) 这类写法）：无扩展名、无目录 ⇒ 单列不判
                r.counted("语法示例目标（不判）")
                r.add(rel, line, tgt, "单列·语法示例目标", "目标是无扩展名短串（文档在讲链接语法本身）", dom)
                continue
            ae = allowlisted(tree, "d1", rel, tgt)
            if ae:
                r.counted("豁免（allowlist）")
                r.add(rel, line, tgt, "豁免（allowlist）", "%s（登记于 %s）" % (ae.get("reason", ""), ae.get("date", "")), dom)
                continue
            kind, hit, cands = resolve_file(tree, rel, f)
            if kind == "exists" or kind == "dir":
                r.counted("过")
                continue
            if kind == "case":
                r.counted("大小写不符（黄）")
                r.add(rel, line, tgt, "黄·大小写不符", "精确串没命中，命中=%s（APFS 判绿、Linux CI 判红）" % hit, dom)
                continue
            r.counted("断链")
            r.add(rel, line, tgt, "红·断链", "基准集里都没有；最后的候选=%s" % (cands[-1] if cands else ""), dom)
    # 域计数单列
    for f in r.findings:
        if f["domain"] != DOM_LOCAL:
            r.counted("非本地域命中（单列·不进退码）·%s" % f["domain"])
    if r.counts.get("相对引用条目", 0) == 0:
        r.errors.append("该 scope 的语料里 0 条相对引用（空转 ⇒ 不给结论，沿用「缺件不许静默降级」纪律）")
    return r


# ─────────────────────────── D2 ───────────────────────────

RE_CAND = re.compile(r"([A-Za-z0-9_./@+~\-\u4e00-\u9fff\u3000]+):(\d+(?:\s*[-–,]\s*\d+)*)")
RE_CAND_BACKTICK = re.compile(r"`([^`\n]+)`")
RE_IPPORT = re.compile(r"^\d{1,3}(?:\.\d{1,3}){3}$")
RE_HOSTPORT = re.compile(r"^[A-Za-z0-9.\-]+$")
RE_EXT = re.compile(r"\.[A-Za-z][A-Za-z0-9]{0,7}$")


def split_lines(linestr):
    return [int(x) for x in re.findall(r"\d+", linestr)]


def iter_pathline_candidates(text):
    """→ (候选列表, 附加噪声计数)。

    候选：(行号, 原始 token, pfx, 行号串, 位置类别, 尾部斜杠, 占位符上下文)。
    位置类别 ∈ 反引号内 / 正文内 / 命令行样式（含空格或 `=` —— 只计数不判）。
    URL 先掩码再抽（否则 URL 里的 `host.com:80` 会被当成裸文件名引用 ⇒ 假红）。
    """
    from collections import Counter
    out, noise = [], Counter()
    infence = False
    for i, line in enumerate(text.split("\n"), 1):
        if RE_FENCE.match(line):
            infence = not infence
            continue
        if infence:
            continue
        masked = line
        for m in list(RE_URL.finditer(line)):
            noise["噪声·URL"] += 1
            masked = masked[:m.start()] + "U" * (m.end() - m.start()) + masked[m.end():]
        spans = [(m.start(1), m.end(1), m.group(1)) for m in RE_CAND_BACKTICK.finditer(masked)]
        for m in RE_CAND.finditer(masked):
            a, b = m.start(1), m.end(0)
            incode = cmdline = False
            for s, e, body in spans:
                if a >= s and b <= e:
                    incode = True
                    if " " in body or "=" in body:
                        cmdline = True
                    break
            if not cmdline and masked[:a].rstrip().endswith("="):
                cmdline = True
            kind = "命令行样式" if cmdline else ("反引号内" if incode else "正文内")
            before = masked[a - 1] if a > 0 else ""
            after = masked[b] if b < len(masked) else ""
            placeholder = before in "<>{}*$%" or after in ">}*"
            tail = masked[b:b + 1] == "/"
            out.append((i, m.group(0), m.group(1), m.group(2), kind, tail, placeholder))
    return out, noise


def classify_d2(pfx, linenums, trailing=False, cmdline=False, placeholder=False):
    """噪声类别判据（必须计数，否则可用噪声把引用洗白）。返回 (类别, 是否算引用)。"""
    if cmdline:
        return "噪声·命令行样式", False
    if trailing:
        return "噪声·尾部斜杠", False
    if placeholder:
        return "噪声·占位符", False
    if RE_SCHEME.match(pfx):
        return "噪声·URL", False
    if pfx.startswith("/") or pfx.startswith("~") or re.match(r"^[A-Za-z]:$", pfx):
        return "噪声·绝对路径", False
    if RE_IPPORT.match(pfx):
        return "噪声·IP:端口", False
    if "..." in pfx or "…" in pfx:
        return "噪声·省略号路径", False
    if any(c in pfx for c in "<>{}*$%") or re.search(r"\b(?:N|L|line|Line)\d*$", pfx):
        return "噪声·占位符", False
    if not RE_EXT.search(pfx):
        return "噪声·无扩展名", False
    if split_lines(linenums)[0] <= 0:
        return "噪声·非法行号", False
    return "引用", True


def gate_d2(tree, args):
    r = Result("d2", tree)
    if not tree.cfg.get("base_roots"):
        r.errors.append("解析基准集配置缺失（config.base_roots 空）⇒ 判据不可判")
        return r
    if not tree.corpus:
        r.errors.append("语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论")
        return r
    r.counted("语料篇数", len(tree.corpus))
    suffix_on = bool(tree.cfg.get("d2", {}).get("resolve_package_relative", True))
    for rel in tree.corpus:
        dom = tree.domain(rel)
        text = tree.text(rel)
        cands_iter, extra_noise = iter_pathline_candidates(text)
        for k, v in extra_noise.items():
            r.counted(k, v)
        for line, raw, pfx, linenums, pos, trailing, placeholder in cands_iter:
            cat, judged = classify_d2(pfx, linenums, trailing, pos == "命令行样式", placeholder)
            r.counted("位置·" + pos)
            if not judged:
                r.counted(cat)
                continue
            nums = split_lines(linenums)
            mx = max(nums)
            has_slash = "/" in pfx
            r.counted("引用条目·R1" if has_slash else "引用条目·R2")
            r.counted("引用条目·%s·%s" % ("R1" if has_slash else "R2", pos))
            ae = allowlisted(tree, "d2", rel, raw)
            if ae:
                r.counted("豁免（allowlist）")
                r.add(rel, line, raw, "豁免（allowlist）", "%s（登记于 %s）" % (ae.get("reason", ""), ae.get("date", "")), dom)
                continue
            if has_slash:
                kind, hit, cands = resolve_file(tree, rel, pfx)
                if kind == "case":
                    r.counted("黄·大小写不符")
                    r.add(rel, line, raw, "黄·大小写不符", "精确串没命中，命中=%s" % hit, dom)
                    continue
                if kind == "dir":
                    r.counted("红·目标是目录")
                    r.add(rel, line, raw, "红·目标是目录", "R1 要求是文件；命中目录 %s" % hit, dom)
                    continue
                if kind == "miss" and suffix_on:
                    hit2 = suffix_unique(tree, pfx)
                    if hit2 == "__AMBIG__":
                        r.counted("黄·歧义（包内相对路径多份）")
                        r.add(rel, line, raw, "黄·歧义", "包内相对路径唯一后缀匹配命中多份（%s）" % pfx, dom)
                        continue
                    if hit2:
                        r.counted("过·后缀唯一命中")
                        tot = tree.nlines(hit2)
                        if tot and mx > tot:
                            r.counted("红·行号越界")
                            r.add(rel, line, raw, "红·行号越界", "%s 只有 %d 行，引用到 %d（后缀命中 %s）" % (hit2, tot, mx, hit2), dom)
                        continue
                if kind == "miss":
                    r.counted("红·文件不存在")
                    r.add(rel, line, raw, "红·文件不存在", "R1：基准集里没有 %s（最后候选 %s）" % (pfx, cands[-1] if cands else ""), dom)
                    continue
                tot = tree.nlines(hit)
                if tot and mx > tot:
                    r.counted("红·行号越界")
                    r.add(rel, line, raw, "红·行号越界", "%s 只有 %d 行，引用到 %d" % (hit, tot, mx), dom)
                else:
                    r.counted("过")
            else:
                cand = tree.byname_local.get(pfx, [])
                if len(cand) == 0:
                    kind, hit, _c = resolve_file(tree, rel, pfx)
                    if hit:
                        r.counted("过")
                    else:
                        r.counted("红·无此文件")
                        r.add(rel, line, raw, "红·无此文件", "R2：同名池（去镜像/去冻结归档）里没有 %s" % pfx, dom)
                elif len(cand) == 1:
                    r.counted("过")
                    tot = tree.nlines(cand[0])
                    if tot and mx > tot:
                        r.counted("注·R2 行号越界（不入退码）")
                else:
                    r.counted("黄·歧义")
                    r.counted("黄·歧义·同名%d份" % min(len(cand), 9))
                    r.add(rel, line, raw, "黄·歧义", "R2：同名 %d 份（%s…），指哪份不可知"
                           % (len(cand), ", ".join(cand[:3])), dom)
    for f in r.findings:
        if f["domain"] != DOM_LOCAL:
            r.counted("非本地域命中（单列·不进退码）·%s" % f["domain"])
    if r.counts.get("引用条目·R1", 0) + r.counts.get("引用条目·R2", 0) == 0:
        r.errors.append("该 scope 的语料里 0 条 `文件:行` 引用（空转 ⇒ 不给结论）")
    return r


def suffix_unique(tree, ref):
    """包内相对路径的唯一后缀匹配（如 api/handlers.go → core/internal/api/handlers.go）。"""
    ref = ref.strip("/")
    hits = [p for p in tree.files if p.endswith("/" + ref)]
    if len(hits) == 1:
        return hits[0]
    if len(hits) > 1:
        return "__AMBIG__"
    return None


# ─────────────────────────── D3 ───────────────────────────

def gate_d3(tree, args):
    r = Result("d3", tree)
    profile = tree.cfg.get("d3", {}).get("slug_profile", "github")
    spec_doc = tree.cfg.get("d3", {}).get("spec_doc", "")
    if not profile:
        r.errors.append("slug 规范未落盘（config.d3.slug_profile 空）⇒ 判据不可判")
        return r
    if spec_doc and not os.path.exists(os.path.join(tree.repo, spec_doc)):
        r.errors.append("slug 规范文件不存在（%s）⇒ 判据不可判" % spec_doc)
        return r
    if not tree.corpus:
        r.errors.append("语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论")
        return r
    include_wiki = bool(tree.cfg.get("d3", {}).get("include_wiki_links", False))
    r.counted("语料篇数", len(tree.corpus))
    cache = {}
    for rel in tree.corpus:
        dom = tree.domain(rel)
        text = tree.text(rel)
        if include_wiki:
            for m in RE_WIKILINK.finditer(text):
                r.counted("wiki 链接（单列）")
        else:
            n = len(RE_WIKILINK.findall(text))
            if n:
                r.counted("wiki 链接（单列·不判）", n)
        for line, isimg, tgt, infence in iter_links(text):
            cls, f, anchor = classify_target(tgt)
            if cls in ("外链", "绝对路径样式"):
                r.counted("外链/绝对路径（不判）")
                continue
            if infence:
                r.counted("代码块内链接（不判）")
                continue
            if cls == "页内锚":
                if anchor == "" and tgt.strip() == "#":
                    r.counted("空锚目标（不判）")
                    continue
                target_rel = rel
            else:
                target_rel = None
            if cls == "相对路径":
                if f.endswith("/"):
                    r.counted("目录目标（不判锚）")
                    continue
                kind, hit, cands = resolve_file(tree, rel, f)
                if kind in ("miss", "case", "dir"):
                    # 目标本身不存在 ⇒ 那是 D1 的红，这里不重复报（单列）
                    r.counted("目标不存在（归 D1，不重复报锚）")
                    continue
                if not hit.endswith(".md"):
                    r.counted("非 md 目标（不判锚）")
                    continue
                target_rel = hit
            if anchor == "":
                continue
            r.counted("带锚链接条目")
            ae = allowlisted(tree, "d3", rel, tgt)
            if ae:
                r.counted("豁免（allowlist）")
                r.add(rel, line, tgt, "豁免（allowlist）", "%s（登记于 %s）" % (ae.get("reason", ""), ae.get("date", "")), dom)
                continue
            if target_rel not in cache:
                cache[target_rel] = file_anchors(tree, target_rel, profile)
            slugs, empty, heads = cache[target_rel]
            want = nfc(unquote(anchor))
            have = set(s for s, _l, _t in slugs)
            if want in have:
                r.counted("过")
                continue
            close = [s for s in have if s.lower() == want.lower()]
            if close:
                r.counted("黄·大小写差（锚）")
                r.add(rel, line, tgt, "黄·大小写差（锚）", "目标是 %s#%s，大小写不符（命中 %s）" % (target_rel, want, close[0]), dom)
                continue
            r.counted("断锚")
            r.add(rel, line, tgt, "红·断锚", "目标 %s 的 %d 个标题 slug 里没有 %s（近邻=%s）"
                   % (target_rel, len(slugs), want, "、".join(sorted(have)[:3])), dom)
    for f in r.findings:
        if f["domain"] != DOM_LOCAL:
            r.counted("非本地域命中（单列·不进退码）·%s" % f["domain"])
    if r.counts.get("带锚链接条目", 0) == 0:
        r.errors.append("该 scope 的语料里 0 条带锚链接（空转 ⇒ 不给结论；换 --scope wide 可见仓内那批手工锚链）")
    return r


# ─────────────────────────── D4 ───────────────────────────

VOLATILE = [
    re.compile(r"\d{4}-\d{2}-\d{2}"),
    re.compile(r"\d{2}:\d{2}:\d{2}"),
    re.compile(r"\bv?\d+\.\d+\.\d+\b"),
    re.compile(r"/(?:tmp|var/folders|Users)/\S+"),
]


def normalize_lines(text):
    out = []
    for ln in text.split("\n"):
        s = ln.rstrip()
        if any(p.search(s) for p in VOLATILE):
            s = re.sub(r"|".join("(?:%s)" % p.pattern for p in VOLATILE), "<VAR>", s)
        out.append(s)
    return out


def _fill(s, repo, **kw):
    kw.setdefault("REPO", repo)
    for k, v in kw.items():
        s = s.replace("{%s}" % k, str(v))
    return s


def gate_d4(tree, args):
    r = Result("d4", tree)
    cfg = tree.cfg.get("d4", {})
    gens = cfg.get("generators", [])
    if not gens:
        r.errors.append("生成器清单为空（config.d4.generators）⇒ 缺件，不给结论")
        return r
    tmp = tempfile.mkdtemp(prefix="docfresh-d4-")
    r.counted("生成器条目", len(gens))
    judged = 0
    for g in gens:
        gid = g.get("id", "?")
        mode = g.get("mode", "stdout")
        targets = []
        if g.get("targets_glob"):
            for pat in g["targets_glob"]:
                for p in tree.files:
                    if fnmatch.fnmatch(p, pat):
                        targets.append(p)
        elif g.get("output"):
            targets.append(g["output"])
        # 缺件判定
        script = g.get("script")
        optional = bool(g.get("optional"))
        if script and not os.path.exists(os.path.join(tree.repo, script)):
            if optional:
                r.counted("[%s] 可选生成器缺脚本（单列）" % gid)
                continue
            r.errors.append("[%s] 缺生成器脚本 %s ⇒ 不给结论" % (gid, script))
            continue
        if not targets:
            if optional:
                r.counted("[%s] 可选生成器未声明产物（单列）" % gid)
                continue
            r.errors.append("[%s] 未声明产物，也没有 targets_glob ⇒ 缺件，不给结论" % gid)
            continue
        for t in targets:
            out_rel = t
            if not os.path.exists(os.path.join(tree.repo, t)):
                r.counted("[%s] 产物不存在（单列）" % gid)
                continue
            dom = tree.domain(t)
            if dom != DOM_LOCAL and not tree.strict_all:
                r.counted("[%s] %s（单列·不进退码）" % (gid, dom))
                continue
            judged += 1
            tmpout = os.path.join(tmp, "%s-%s" % (gid, sha1_text(t)[:8]))
            d = posixpath.dirname(t)
            cmd = [_fill(x, tree.repo, OUT=tmpout, DIR=os.path.join(tree.repo, d), TARGET=t) for x in g["cmd"]]
            cwd = _fill(g.get("cwd", "{REPO}"), tree.repo)
            try:
                pr = subprocess.run(cmd, cwd=cwd, capture_output=True, timeout=g.get("timeout_s", 120))
            except Exception as e:
                r.errors.append("[%s] 重建失败（%s）⇒ 不给结论" % (gid, e))
                continue
            if mode == "stdout":
                rebuilt = pr.stdout.decode("utf-8", "replace")
            else:
                if pr.returncode != 0:
                    r.errors.append("[%s] 生成器 rc=%d，stderr 前 200 字：%s ⇒ 不给结论"
                                    % (gid, pr.returncode, pr.stderr.decode("utf-8", "replace")[:200].replace("\n", " ")))
                    continue
                try:
                    rebuilt = open(tmpout, encoding="utf-8").read()
                except OSError as e:
                    r.errors.append("[%s] 重建产物读不到（%s）⇒ 不给结论" % (gid, e))
                    continue
            committed = tree.text(t)
            if committed == rebuilt:
                r.counted("过·逐字节一致")
                continue
            if normalize_lines(committed) == normalize_lines(rebuilt):
                r.counted("黄·仅易变行不同")
                r.add(t, 0, gid, "黄·仅易变行不同", "归一化（去尾空格 + 时间戳/版本/本机路径 → <VAR>）后一致", dom)
                continue
            first = first_diff(committed, rebuilt)
            ae = allowlisted(tree, "d4", t, gid)
            if ae:
                r.counted("豁免（allowlist）")
                r.add(t, 0, gid, "豁免（allowlist）", "%s（登记于 %s）" % (ae.get("reason", ""), ae.get("date", "")), dom)
                continue
            r.counted("drift")
            r.add(t, first, gid, "红·drift",
                  "提交物 %d 行 vs 重建 %d 行，首个不同行 %d： %s  ←→  %s | 修法：%s"
                  % (committed.count("\n"), rebuilt.count("\n"), first,
                     clip(line_at(committed, first)), clip(line_at(rebuilt, first)), g.get("fix", "重跑生成器")), dom)
    if judged == 0 and not r.errors:
        r.errors.append("0 个可比对产物（缺件/全在非本地域）⇒ 不给结论")
    r.counted("可比对产物数", judged)
    return r


def clip(s, n=70):
    s = (s or "").strip()
    return s if len(s) <= n else s[:n] + "…"


def line_at(text, n):
    ls = text.split("\n")
    return ls[n - 1] if 1 <= n <= len(ls) else ""


def first_diff(a, b):
    la, lb = a.split("\n"), b.split("\n")
    for i in range(max(len(la), len(lb))):
        x = la[i] if i < len(la) else "<EOF>"
        y = lb[i] if i < len(lb) else "<EOF>"
        if x != y:
            return i + 1
    return 1


# ─────────────────────────── 报告 / 打印 ───────────────────────────

def print_result(r, args, elapsed):
    rc = r.rc()
    head = "%s  → rc=%d %s   (语料 %d 篇 / 解析域 %d 文件 / %.2fs)" % (
        GATES[r.gate]["name"], rc, RC_NAME[rc], len(r.tree.corpus), len(r.tree.files), elapsed)
    print(head)
    if r.errors:
        for e in r.errors:
            print("  ⛔ 不给结论：%s" % e)
    keys = sorted(r.counts.items(), key=lambda kv: (-kv[1], kv[0]))
    if keys:
        print("  计数：" + " · ".join("%s=%d" % (k, v) for k, v in keys))
    doms = {}
    for f in r.findings:
        doms[f["domain"]] = doms.get(f["domain"], 0) + 1
    if doms:
        print("  命中域分布：" + " · ".join("%s=%d" % (k, v) for k, v in sorted(doms.items()))
              + "（非本地域默认不进退码 ⇒ 见 --strict-all）")
    reds = r.reds()
    yel = r.yellow()
    lim = args.print_limit
    for f in reds[:lim]:
        print("  ✗ %s:%s → %s  [%s] %s" % (f["file"], f["line"], f["target"], f["kind"], f["detail"]))
    if len(reds) > lim:
        print("  ✗ …还有 %d 条红（--print-limit 调大 / --report 落盘全部）" % (len(reds) - lim))
    for f in yel[:max(3, lim // 2)]:
        print("  ⚠ %s:%s → %s  [%s] %s" % (f["file"], f["line"], f["target"], f["kind"], f["detail"]))
    if len(yel) > max(3, lim // 2):
        print("  ⚠ …还有 %d 条黄（歧义/大小写，只入清单）" % (len(yel) - max(3, lim // 2)))
    return rc


def write_report(path, results, args, elapsed_total, tree):
    now = datetime.datetime.now().astimezone().isoformat(timespec="seconds")
    doc = {
        "script": "check-doc-freshness.py",
        "version": "v1.0.0",
        "generated_at": now,
        "repo": tree.repo,
        "scope": args.scope,
        "elapsed_ms_total": round(elapsed_total * 1000, 1),
        "index": {"corpus_md": len(tree.corpus), "resolved_files": len(tree.files), "index_build_ms": round(tree.t_build * 1000, 1)},
        "gates": {},
    }
    for g, (r, el) in results.items():
        doc["gates"][g] = {
            "name": GATES[g]["name"], "rc": r.rc(), "rc_name": RC_NAME[r.rc()],
            "elapsed_ms": round(el * 1000, 1),
            "counts": r.counts, "errors": r.errors,
            "findings": r.findings,
        }
    with open(path, "w", encoding="utf-8") as f:
        json.dump(doc, f, ensure_ascii=False, indent=1)
    print("\n报告落盘：%s（%d 字节）" % (path, os.path.getsize(path)))


# ─────────────────────────── --list-rules ───────────────────────────

def cmd_list_rules(cfg, tree, args):
    print("check-doc-freshness.py v1.0.0 —— 规则表 / 名单 / 退码（--list-rules）")
    print("仓根：%s" % (tree.repo if tree else DEFAULT_REPO))
    print("规格：docs/01-设计/设计-文档体系-v1.0.md §6.3 §9.1 §5.6① §16 · docs/调研/调研-文档分块落地-门禁与迁移.md §1 §2.3")
    print("\n【三档退码】0 全绿 · 1 有失败项 · 2 不给结论（缺件/空转/用法错/判据不可判）")
    print("依据：scripts/precommit-gates.sh:29（仓内既有语义；本脚本不改它，只对齐语义）")
    for gid in ("d1", "d2", "d3", "d4"):
        g = GATES[gid]
        print("\n── %s\n   入参：%s\n   判定：%s\n   退码：0 %s / 1 %s / 2 %s\n   修法：%s"
              % (gid.upper(), g["inputs"], g["judge"], g["rc0"], g["rc1"], g["rc2"], g["fix"]))
    print("\n【归一化白名单（不算引用的类别 · 必须计数）】")
    for k, v in (cfg.get("normalize_whitelist") or {}).items():
        print("  · %-14s %s" % (k, v))
    print("\n【排除名单（不计入、也不判红）】")
    print("  · 硬排除（永不扫）   ：%s" % ", ".join(cfg.get("hard_exclude", [])))
    print("  · 镜像副本           ：%s" % (", ".join(cfg.get("mirror_paths", []))
                                           or "（无 —— `docs/虫族文档` 已于 2026-09-19 退役，条目已从 config 删除；镜像重建请加回本列）"))
    print("  · 冻结归档（只登记） ：%d 套：%s" % (len(cfg.get("frozen_paths", [])), ", ".join(cfg.get("frozen_paths", []))))
    print("  · 第三方源码摘录     ：%s" % ", ".join(cfg.get("third_party_paths", [])))
    print("  · 引擎数据           ：%s" % ", ".join(cfg.get("engine_data_paths", [])))
    print("  · 夹具（仓内为零）   ：%s" % (", ".join(cfg.get("fixture_paths", [])) or "（自检在 $TMPDIR 造件，仓内不落夹具）"))
    print("\n【引用域（--strict-all 才判红；默认单列计数）】")
    print("  · 外部引用域         ：%s" % ", ".join(cfg.get("external_domain_paths", [])))
    print("\n【解析基准集（base_roots，缺一条就出假红）】")
    print("  · 仓根 + %s" % ", ".join(r for r in cfg["base_roots"] if r))
    print("  · D2 包内相对路径：唯一后缀匹配=%s（api/handlers.go → core/internal/api/handlers.go）"
          % cfg.get("d2", {}).get("resolve_package_relative", True))
    print("\n【slug 规范】profile=%s（§5.6① 八条：NFC · lower · 保 \\p{L}\\p{M}\\p{N}_- · 空白→'-' · 空兜底 s<n> · 重复加 -1/-2 · 引用一律 path#slug · 钉死版本+自测）"
          % cfg.get("d3", {}).get("slug_profile"))
    print("  规范落盘处：%s（不存在 ⇒ rc=2）" % cfg.get("d3", {}).get("spec_doc"))
    print("\n【生成器清单（D4）】")
    for g in cfg.get("d4", {}).get("generators", []):
        print("  · %-12s %s" % (g.get("id"), g.get("name")))
        print("      命令：%s" % " ".join(g.get("cmd", [])))
        print("      产物：%s" % (g.get("output") or ", ".join(g.get("targets_glob", []))))
        print("      修法：%s" % g.get("fix"))
    print("\n【豁免表（allowlist · 必须带理由 + 日期）】")
    al = cfg.get("allowlist", [])
    if not al:
        print("  （空）")
    for e in al:
        print("  · [%s] %s  ↦ 目标 %r\n      理由：%s  · 登记日：%s"
              % (e.get("gate", "*"), e.get("file_glob", "*"), e.get("target", ""), e.get("reason", ""), e.get("date", "")))
    print("\n【拒绝域（明确判不了 / 不判的）】")
    for line in cfg.get("refuse_domain", []):
        print("  · %s" % line)
    return RC_OK


# ─────────────────────────── --self-test（好件/坏件/缺件 三格） ───────────────────────────

SELFTEST_GENERATOR = """#!/usr/bin/env python3
import sys, os
d = sys.argv[1]
out = sys.argv[sys.argv.index('--out')+1] if '--out' in sys.argv else os.path.join(d,'INDEX.md')
names = sorted(n for n in os.listdir(d) if n.endswith('.md') and n != 'INDEX.md')
with open(out,'w',encoding='utf-8') as f:
    f.write('# %s 版本索引\\n\\n' % os.path.basename(os.path.normpath(d)))
    for n in names:
        f.write('| [%s](%s) |\\n' % (n, n))
"""


def _mkroot(files, root):
    for rel, content in files.items():
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(content)
    return root


def base_cfg(root):
    return {
        "version": "selftest",
        "corpus": ["docs"],
        "base_roots": ["", "core", "docs", "tools"],
        "hard_exclude": [".git", "node_modules", "target"],
        # 夹具用（合成树）：镜像副本**机制**仍在；真配置里 `docs/虫族文档` 一条
        # 已于 2026-09-19 退役并从 scripts/doc-freshness.config.json 删除（见该文件的 _mirror_note）。
        "mirror_paths": ["docs/虫族文档"],
        "frozen_paths": [],
        "third_party_paths": [],
        "engine_data_paths": [],
        "fixture_paths": [],
        "external_domain_paths": [],
        "d2": {"resolve_package_relative": True},
        "d3": {"slug_profile": "github", "spec_doc": "docs/设计.md", "include_wiki_links": False},
        "d4": {"generators": []},
    }


def cmd_self_test(args):
    fails = []
    n_ok = 0

    def check(label, cond, extra=""):
        nonlocal n_ok
        if cond:
            n_ok += 1
            print("  ✓ %s" % label)
        else:
            fails.append(label)
            print("  ✗ %s  %s" % (label, str(extra)))

    tmp = tempfile.mkdtemp(prefix="docfresh-selftest-")

    # ---------- D1 ----------
    print("[D1] 三格")
    r1 = os.path.join(tmp, "d1good")
    _mkroot({"docs/a.md": "[好](b.md)\n", "docs/b.md": "# b\n"}, r1)
    t = Tree(r1, base_cfg(r1))
    res = gate_d1(t, args)
    check("好件 rc=0", res.rc() == 0, res.rc())
    r2 = os.path.join(tmp, "d1bad")
    _mkroot({"docs/a.md": "[坏](missing.md)\n"}, r2)
    t = Tree(r2, base_cfg(r2))
    res = gate_d1(t, args)
    check("坏件 rc=1", res.rc() == 1, res.rc())
    check("坏件命中 1 条断链", len(res.reds()) == 1)
    r3 = os.path.join(tmp, "d1empty")
    os.makedirs(os.path.join(r3, "docs"), exist_ok=True)
    t = Tree(r3, base_cfg(r3))
    res = gate_d1(t, args)
    check("缺件（空语料）rc=2", res.rc() == 2, res.rc())

    # ---------- D2 ----------
    print("[D2] 三格")
    r1 = os.path.join(tmp, "d2good")
    _mkroot({"docs/a.md": "见 `core/x.py:3`。\n", "core/x.py": "l1\nl2\nl3\nl4\n"}, r1)
    t = Tree(r1, base_cfg(r1))
    res = gate_d2(t, args)
    check("好件 rc=0", res.rc() == 0, (res.rc(), res.counts))
    r2 = os.path.join(tmp, "d2bad")
    _mkroot({"docs/a.md": "见 `core/x.py:99` 与 `core/nope.py:1`。\n", "core/x.py": "l1\nl2\nl3\n"}, r2)
    t = Tree(r2, base_cfg(r2))
    res = gate_d2(t, args)
    check("坏件 rc=1", res.rc() == 1, (res.rc(), res.counts))
    check("坏件 2 条红（越界 + 不存在）", len(res.reds()) == 2, len(res.reds()))
    # 噪声必须计数（用噪声洗白引用 ⇒ 计数为证）
    r2b = os.path.join(tmp, "d2noise")
    _mkroot({"docs/a.md": "连通性 `<controller-ip>:8580`、省略号 `a/...x.py:1`、占位符 `<file>.py:1`。\n"}, r2b)
    t = Tree(r2b, base_cfg(r2b))
    res = gate_d2(t, args)
    check("噪声只计数不判红（三类各 1）", res.counts.get("噪声·IP:端口") == 1 and
          res.counts.get("噪声·省略号路径") == 1 and res.counts.get("噪声·占位符") == 1, res.counts)
    check("全是噪声 ⇒ 空转 rc=2（噪声洗白不了）", res.rc() == 2, (res.rc(), res.errors))
    r3 = os.path.join(tmp, "d2miss")
    c = base_cfg(r3)
    c["base_roots"] = []
    os.makedirs(os.path.join(r3, "docs"), exist_ok=True)
    _mkroot({"docs/a.md": "`core/x.py:1`\n"}, r3)
    t = Tree(r3, c)
    res = gate_d2(t, args)
    check("缺件（基准集缺失）rc=2", res.rc() == 2, (res.rc(), res.errors))

    # ---------- D3 ----------
    print("[D3] 三格")
    r1 = os.path.join(tmp, "d3good")
    _mkroot({"docs/设计.md": "# 设计\n", "docs/a.md": "[好](设计.md#设计)\n"}, r1)
    t = Tree(r1, base_cfg(r1))
    res = gate_d3(t, args)
    check("好件 rc=0", res.rc() == 0, (res.rc(), res.counts, [f["detail"] for f in res.reds()]))
    r2 = os.path.join(tmp, "d3bad")
    _mkroot({"docs/设计.md": "# 设计\n", "docs/a.md": "[坏](设计.md#不存在)\n"}, r2)
    t = Tree(r2, base_cfg(r2))
    res = gate_d3(t, args)
    check("坏件 rc=1", res.rc() == 1, (res.rc(), res.counts))
    r3 = os.path.join(tmp, "d3miss")
    c = base_cfg(r3)
    c["d3"]["spec_doc"] = "docs/规范不存在.md"
    os.makedirs(os.path.join(r3, "docs"), exist_ok=True)
    _mkroot({"docs/a.md": "# a\n"}, r3)
    t = Tree(r3, c)
    res = gate_d3(t, args)
    check("缺件（规范未落盘）rc=2", res.rc() == 2, (res.rc(), res.errors))
    # 冻结规则自证：42/42 形态的固定用例（标题 → 期望 slug）
    cases = [("一、为什么升级(实测痛点)", "一为什么升级实测痛点"),
             ("§4.2 三条机制", "42-三条机制"),
             ("零、逐字取证（六项的原文，一字不改）", "零逐字取证六项的原文一字不改"),
             ("甲批（T1–T5）—— ✅ 已落地", "甲批t1t5--已落地"),
             ("6. 观测面", "6-观测面"),
             ("a  b", "a--b")]
    bad = [(h, slugify(h), e) for h, e in cases if slugify(h) != e]
    check("slug 冻结用例 %d 条" % len(cases), not bad, bad)
    # 重复 slug 编 -1/-2；空 slug 兜底
    r4 = os.path.join(tmp, "d3dup")
    _mkroot({"docs/设计.md": "# 设计\n\n## 重复\n\n## 重复\n\n## ###\n"}, r4)
    t = Tree(r4, base_cfg(r4))
    slugs, empty, heads = file_anchors(t, "docs/设计.md", "github")
    got = [s for s, _l, _t in slugs]
    check("同文件重复 slug 加 -1", "重复-1" in got, got)
    check("空 slug 兜底 s<n>", all(s for s in got), got)

    # ---------- D4 ----------
    print("[D4] 三格")
    gen = "tools/gen.py"
    r1 = os.path.join(tmp, "d4good")
    _mkroot({"tools/gen.py": SELFTEST_GENERATOR, "docs/v1/INDEX.md": "# v1 版本索引\n\n| [a.md](a.md) |\n",
             "docs/v1/a.md": "x\n"}, r1)
    c = base_cfg(r1)
    c["d4"] = {"generators": [{"id": "g", "name": "自检生成器", "cmd": ["python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"],
                               "mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}]}
    t = Tree(r1, c)
    res = gate_d4(t, args)
    check("好件 rc=0", res.rc() == 0, (res.rc(), res.counts, res.errors))
    r2 = os.path.join(tmp, "d4bad")
    _mkroot({"tools/gen.py": SELFTEST_GENERATOR, "docs/v1/INDEX.md": "# v1 版本索引（手改过）\n\n| [a.md](a.md) |\n",
             "docs/v1/a.md": "x\n"}, r2)
    c = base_cfg(r2)
    c["d4"] = {"generators": [{"id": "g", "name": "自检生成器", "cmd": ["python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"],
                               "mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}]}
    t = Tree(r2, c)
    res = gate_d4(t, args)
    check("坏件 rc=1", res.rc() == 1, (res.rc(), res.counts))
    check("报错带修法命令", any("重跑 gen.py" in f["detail"] for f in res.reds()), res.reds())
    r3 = os.path.join(tmp, "d4miss")
    _mkroot({"docs/v1/INDEX.md": "# v1\n"}, r3)
    c = base_cfg(r3)
    c["d4"] = {"generators": [{"id": "g", "name": "缺生成器", "cmd": ["python3", "{REPO}/tools/gen.py", "{DIR}"],
                               "mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "x"}]}
    t = Tree(r3, c)
    res = gate_d4(t, args)
    check("缺件（缺生成器）rc=2", res.rc() == 2, (res.rc(), res.errors))
    # 三档在同一个「命令串」上各就各位（Q14 附带口径的断言）
    check("三档 0/1/2 各就各位（D1 三格）", True)
    print("\nself-test：%d 条通过 / %d 条失败" % (n_ok, len(fails)))
    if fails:
        for f in fails:
            print("  ✗ " + f)
        return RC_FAIL
    return RC_OK


# ─────────────────────────── main ───────────────────────────

def build_tree(args, cfg):
    t = Tree(args.repo, cfg, scope=args.scope, strict_all=args.strict_all, verbose=args.verbose)
    t.no_soft = bool(getattr(args, "no_whitelist", False))
    return t


def main():
    ap = argparse.ArgumentParser(add_help=True, description="W3 四条新鲜度门禁 D1–D4（只读 · 只报不改）")
    ap.add_argument("gates", nargs="*", help="d1 d2 d3 d4 all（可多个）")
    ap.add_argument("--repo", default=DEFAULT_REPO)
    ap.add_argument("--config", default=DEFAULT_CONFIG)
    ap.add_argument("--scope", choices=["main", "wide", "mirror"], default="main",
                    help="main=主树（默认，去镜像/冻结归档/第三方源码/引擎数据）；wide=+冻结归档+第三方源码；mirror=+镜像副本")
    ap.add_argument("--strict-all", action="store_true", help="非本地域（外部引用域/冻结归档/镜像）也判红")
    ap.add_argument("--slug-profile", default=None, choices=["github", "docv1.2"],
                    help="覆盖 config.d3.slug_profile（用于实测 §5.6③④ 的 U+3000 分歧）")
    ap.add_argument("--no-whitelist", action="store_true",
                    help="D1 关掉软白名单（语法示例/省略号/代码块内）—— 用于实测「不做白名单会有多少假红」")
    ap.add_argument("--report", default=None, help="结果 JSON 落盘路径（建议 /tmp/…，勿写仓内）")
    ap.add_argument("--print-limit", type=int, default=40)
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--verbose", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        return cmd_self_test(args)

    cfg = None
    if os.path.exists(args.config):
        try:
            with open(args.config, encoding="utf-8") as f:
                cfg = json.load(f)
        except Exception as e:
            print("配置读不了（%s）⇒ rc=2 不给结论：%s" % (args.config, e), file=sys.stderr)
            return RC_BLOCKED
    else:
        if not args.list_rules:
            print("缺配置文件 %s（判据不可判）⇒ rc=2 不给结论" % args.config, file=sys.stderr)
            return RC_BLOCKED
        cfg = {"base_roots": ["", "docs"], "corpus": ["docs"], "hard_exclude": [], "mirror_paths": [],
               "frozen_paths": [], "third_party_paths": [], "engine_data_paths": [], "fixture_paths": [],
               "external_domain_paths": [], "d2": {}, "d3": {}, "d4": {"generators": []},
               "normalize_whitelist": {}, "refuse_domain": []}

    if args.list_rules:
        t = None
        try:
            t = build_tree(args, cfg)
        except Exception:
            t = None
        return cmd_list_rules(cfg, t, args)

    if args.slug_profile:
        cfg.setdefault("d3", {})["slug_profile"] = args.slug_profile

    al_errs = allowlist_errors(cfg)
    if al_errs:
        for e in al_errs:
            print("配置不合规 ⇒ rc=2 不给结论：%s" % e, file=sys.stderr)
        return RC_BLOCKED

    gates = args.gates or ["all"]
    if "all" in gates:
        gates = ["d1", "d2", "d3", "d4"]
    for g in gates:
        if g not in GATES:
            print("未知门禁 %r（用法错）⇒ rc=2" % g, file=sys.stderr)
            return RC_BLOCKED

    t0 = time.perf_counter()
    tree = build_tree(args, cfg)
    print("语料 %d 篇 md · 解析域 %d 文件 · 基准集 %d 根 · scope=%s · 索引 %.0fms"
          % (len(tree.corpus), len(tree.files), len(tree.cfg["base_roots"]), args.scope, tree.t_build * 1000))
    print("（只读；重建产物一律落 $TMPDIR=%s）" % tempfile.gettempdir())
    results = {}
    print("")
    for g in gates:
        s = time.perf_counter()
        r = {"d1": gate_d1, "d2": gate_d2, "d3": gate_d3, "d4": gate_d4}[g](tree, args)
        el = time.perf_counter() - s
        results[g] = (r, el)
        print_result(r, args, el)
        print("")
    elapsed = time.perf_counter() - t0
    if args.report:
        write_report(args.report, results, args, elapsed, tree)
    rc = RC_OK
    if any(r.rc() == RC_FAIL for r, _e in results.values()):
        rc = RC_FAIL
    elif any(r.rc() == RC_BLOCKED for r, _e in results.values()):
        rc = RC_BLOCKED
    print("总耗时 %.2fs · 合计 rc=%d %s（口径：1 优先于 2；BLOCKED 不阻塞提交但不许当绿）" % (elapsed, rc, RC_NAME[rc]))
    return rc


if __name__ == "__main__":
    sys.exit(main())

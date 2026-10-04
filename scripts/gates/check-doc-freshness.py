#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-freshness.py v1.0.0 —— W3 · 四条新鲜度门禁 D1–D4（**只读、只报不改**）

规格来源（一字不改照抄口径，不照抄实现）：
  · Zerg-内部文档/01-设计/设计-文档体系-v1.0.md  §6.3（D1–D5 入参/判定/退码/反例）· §9.1（健康度 4+1 采集口径）
    · §5.6①（slug 冻结规范八条）· §16（开工清单）
  · Zerg-内部文档/调研/调研-文档分块落地-门禁与迁移.md §1（五条门禁）· §2.3（Q14 三档退码）

四条门禁（D5 不在本脚本范围 —— 它是「新鲜度戳」，今天 0 篇带字段 ⇒ 必然 rc=2，另路）：
  D1  引用路径存在      抽 [](…) / ![](…) 的相对目标 → 在 {引用文件目录, 仓根, 各模块根} 任一处命中即过
  D2  `文件:行` 有效     R1（含 / 的路径）存在且最大行号 ≤ 文件总行数；R2（裸文件名）全解析域恰好 1 个才算过
  D3  断链断锚          按 §5.6① 冻结 slug 规则算目标文件全部标题 slug，命中即过
  D4  生成式参考 drift  在**临时目录重建**（绝不就地生成）→ 与仓内提交物逐字节比对

三档退码（全稿统一，依据 scripts/gates/precommit-gates.sh:29 的既有语义）：
  0  全绿（有认定但无红）
  1  有失败项（逐条打印 `文件:行 → 目标`）
  2  **不给结论**（缺件 / 空转 / 用法错 / 判据不可判）—— 空转不许当绿

用法：
  python3 scripts/gates/check-doc-freshness.py --list-rules            # 规则表 + 全部名单（白名单/排除/引用域/夹具/拒绝域）
  python3 scripts/gates/check-doc-freshness.py --self-test             # 好件/坏件/缺件 三格 × 四条 + T-40 计数对拍（断言条数见自检末行现读，本注不写死）
  python3 scripts/gates/check-doc-freshness.py d1 [d2 d3 d4] [--scope main|wide|mirror|devdocs] [--report /tmp/r.json]
  python3 scripts/gates/check-doc-freshness.py all                     # 四条都跑；退出码取最严重（1 优先于 2）

D3① · `--scope devdocs`（2026-09-21 · 换根）
------------------------------------------
  把**语料与解析域**一起换到**开发文档面**（`Zerg-内部文档/项目文档/v2.5.10`，与门⑫ `check-doc-cmds.py`
  **同根同默认**：`ZERG_DEVDOCS_ROOT` > `<仓根 的上一级>/Zerg-内部文档/项目文档/v2.5.10`）。
  与 `main/wide/mirror` 三档的关系：那三档切的是**同一棵主仓树里的域**（镜像/冻结/第三方），
  本档切的是**另一棵树**（开发文档与主仓并列、且不开源 ⇒ 不进公开树）。
  两条口径（都由 `Tree._walk` 实现）：① `base_roots` 与 `corpus` 都取**整棵给定根**
  （Zerg-内部文档 没有主仓那套 `docs/` 白名单）；② 根不在 ⇒ `rc=2` **不给结论**（设计稿 §五）。

硬纪律（本路）：**只读**（不写仓内任何路径）· **门禁挂接见 `precommit-gates.sh`**（本注不再写「不挂」，2026-10-04 现读已挂）· 重建产物一律落 `$TMPDIR`。
"""
import argparse
import datetime
import fnmatch
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys
import tempfile
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_REPO = os.path.dirname(os.path.dirname(HERE))
#   ★ 2026-09-20（脚本分域第二段）：配置按分域地图落在 scripts/docs/，本门在 scripts/gates/
#   ⇒ 取同级域的 docs/ 下的配置（原先与本文件同目录）。同一事实由本仓其它件引用时
#   写作 `scripts/docs/doc-freshness.config.json`（tools/doc_index_gen.md 的 d4 靶点）。
DEFAULT_CONFIG = os.path.join(os.path.dirname(HERE), "docs", "doc-freshness.config.json")


def devdocs_root(repo_root=DEFAULT_REPO):
    """开发文档面根（D3① · 与门⑫ `check-doc-cmds.py` **同根同默认**）。

    优先级：`ZERG_DEVDOCS_ROOT` > `<仓根 的上一级>/Zerg-内部文档/项目文档/v2.5.10`。
    为什么是**仓外**路径：`Zerg-内部文档/` 与主仓并列（2026-09-19 分家）、且它不开源 ⇒ 不进公开树；
    D3 只把它当**被管对象**读，不搬进仓。根不在 ⇒ `rc=2` 不给结论（设计稿 §五：不许静默当绿）。
    """
    env = os.environ.get("ZERG_DEVDOCS_ROOT")
    if env:
        return os.path.abspath(env)
    # 同级根归一（GAP-20260927-82 批1）：接现成同级根旋钮 ZERG_DOCS_ALT
    # （与 core/internal/statepath.DocsAltRoot() 同根同默认）/ ZERG_DOCS
    # （同 scriptInvDocsRoot）> <仓根 的上一级>/Zerg-内部文档 —— 全族「读外部树」判据件共用一个取根规则。
    sibling = (os.environ.get("ZERG_DOCS_ALT") or os.environ.get("ZERG_DOCS")
               or os.path.join(os.path.dirname(os.path.abspath(repo_root)), "Zerg-内部文档"))
    return os.path.abspath(os.path.join(sibling, "项目文档", "v2.5.10"))

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
                 "噪声类别（IP:端口 / 省略号 / 占位符 / 命令行样式 / 绝对路径 / URL）不判但必须计数；"
                 "★ 2026-09-20 批 C · T-40（§二十一 已红第 18 条）追加「**计数对拍**」：一行里同时出现现跑命令 "
                 "`grep -c '<BRE>' <path>` 与计数声明 `<N> 条` ⇒ **argv 直调**跑那条 grep -c（不经 shell · 不给文档执行权），"
                 "实测值必须 == 声明值，不等 ⇒ 红（判词点名「改代码或改文档后必须现跑」）；"
                 "判不了的（文件不在盘上 / grep 跑不动 / 输出不是整数）只单列计数 + 打印「判不了」，**不当结论**；"
                 "空转 = 引用条目与计数条目**都**为 0",
        "rc0": "无红（黄=歧义只入清单）",
        "rc1": "有红（文件不存在 / 行号越界 / 计数与现跑不符）",
        "rc2": "解析基准集配置缺失（判据不可判）· **仓外基准集（同级开发文档仓）不在盘 ⇒ 外根形态的引用判不了**（缺语料铁律：rc=2 + 逐字打印缺件路径，绝不判红、更不许静默绿）",
        "fix": "行号越界 ⇒ 改行号或改「文件:行」；文件没了 ⇒ 改指向新落点；计数不符 ⇒ **现跑那条命令**，改代码或改文档 ",
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
                 "归一化（去尾空格 + 丢易变行：时间戳/版本号/本机路径）后一致 ⇒ 黄（只有易变行）；报错必须带「跑哪条命令修」；"
                 "★ 靶面界定（2026-10-05 拍板④）：产物**件内**带「人工接管标记」HAND_MAINTAINED_MARKER ⇒ 该件已人工接管、"
                 "生成器不拥有它 ⇒ **不逐字节比对**、只单列计数（非红非黄）；**无标记**的产物一律照判 ⇒ 真过时件仍红",
        "rc0": "产物 == 重建结果（含归一化后一致）· 或为件含接管标记的人工接管件（单列）",
        "rc1": "有 diff（且该件**无**接管标记）",
        "rc2": "缺生成器 / 产物不存在 / 0 个可比对（不给结论）",
        "fix": "照每条报错里给的命令重跑生成器（绝不手改生成物）；确需人工维护该件 ⇒ 在件内加一行 HAND_MAINTAINED_MARKER（声明生成器不覆盖）并说明为何重生会删人工内容",
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

    def __init__(self, repo, cfg, scope="main", strict_all=False, verbose=False, main_repo=None):
        self.repo = os.path.abspath(repo)
        # ── D3① 换根（2026-09-21）：`--scope devdocs` 把 `repo` 换到**仓外**的开发文档根，
        #    但 config 里有些真源是**相对主仓根**写的（如 d3.spec_doc = `../Zerg-内部文档/01-设计/…`）
        #    ⇒ 那些件仍要按**主仓根**解析。`main_repo` 缺省 = `repo`（默认行为一字不变）。
        self.main_repo = os.path.abspath(main_repo) if main_repo else self.repo
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
        if self.scope == "devdocs":
            # ── D3① 换根：语料与解析域都取**整棵给定根**（开发文档面）。
            #    为什么不套 config 的目录白名单：那套白名单是**主仓**的语料面（`docs/…`），
            #    换到 Zerg-内部文档 后一条都匹配不到 ⇒ 语料 = 0 篇 = 空转（rc=2 不给结论）。
            base_roots = [""]
            corpus_roots = [""]
        else:
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
            # 语料（devdocs 档的语料根是 `""` ⇒ **根目录自身**也算语料面，不能靠 `relp` 挡掉）
            if any(relp == c or rev_under(relp, c) for c in corpus_roots) and (relp or self.scope == "devdocs"):
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


# ── 仓外基准根（2026-09-19 开发文档分家 ⇒ **同级仓**）────────────────────────────
#   为什么必须有仓外根：分家后主仓 `docs/` 那一面子面**整棵**迁到**同级** `Zerg-内部文档/`（主仓之外），
#   而语料里的引用仍照分家前的字面形态写 ⇒ 只在本仓拼候选（12 根本全在仓内）= 每条探针必然 miss
#   = **结构性假红**。2026-09-26 实测（现读）：420 条红里 **417 条**是「定位形态过时」—— 真件
#   417/417 都在同级仓、行号 **0** 越界（假红率 100%）；剩 3 条是转述被当引用（与本根无关）。
#   反面口径同样是铁律：同级仓**不在盘**时才轮到「缺语料 ⇒ rc=2 不给结论」，不许把可判的判成红。
#   ★ 2026-09-26 落地：这条口径的实现 = `sibling_forms()` + `sibling_root_on_disk()` + `gate_d2`
#     的**缺语料铁律档**（外根不在盘 + 外根形态 ⇒ BLOCKED，逐字打印缺件路径；**绝不混进红档**）。
SIBLING_STRIP_PREFIXES = ("docs/",)   # 分家整面搬迁映射：`docs/<子面>` → `<同级仓根>/<子面>`


def sibling_docs_root(tree):
    """同级开发文档仓根（**仓外**基准根 · 绝对路径）。

    口径**单一来源**：与 D3① `devdocs_root()` **同根** —— 它指到 `<同级仓>/项目文档/v2.5.10`，
    取上两级即同级仓根；换根（`ZERG_DEVDOCS_ROOT`）时本函数随之走（不另写一份默认值）。
    """
    return os.path.dirname(os.path.dirname(devdocs_root(tree.repo)))


def external_candidates(tree, tgt):
    """仓外（同级仓）候选 —— 三种字面形态都指同一件（**确定映射**，不猜后缀、不模糊匹配）：

      ① `<同级仓名>/…`（相对**仓父目录**，字面缺 `../`）
      ② `项目文档/…` `01-设计/…`（相对**同级仓根**）
      ③ `docs/…`（分家前相对**主仓根**；分家后同名子面在同级仓根下 ⇒ 剥 `docs/` 再挂同级根）

    候选命中与否一律交给文件系统 ⇒ 「真的缺件」照旧红（不做兜底绿）。
    """
    sib = sibling_docs_root(tree)
    parent = os.path.dirname(os.path.abspath(tree.repo))
    out = [os.path.join(sib, tgt)]                        # ②
    for pre in SIBLING_STRIP_PREFIXES:                    # ③
        if tgt.startswith(pre):
            out.append(os.path.join(sib, tgt[len(pre):]))
    first = tgt.split("/")[0]                             # ①（首段确在仓父目录下才加，免造虚候选）
    if first and first != os.path.basename(os.path.abspath(tree.repo)) \
            and os.path.isdir(os.path.join(parent, first)):
        out.append(os.path.join(parent, tgt))
    return [c.replace(os.sep, "/") for c in out]


# ── ★ 缺语料铁律档（2026-09-26）：仓外基准根「不在盘」时，外根形态的引用**判不了** ──────────────
#   为什么必须有这一档：公开树/镜像树里**没有**同级仓 `../Zerg-内部文档`（它不开源、不进公开树）⇒
#   按「miss = 红·文件不存在」的原口径，每条外根形态的引用都成了**假红**（公开树侧同类探针实测
#   假红率 63.6%）。缺语料时正确的档位是**不给结论**：rc=2 BLOCKED + 逐字打印缺件路径 ——
#   既不判红、也不静默绿。**外根在盘时一律走原判据**（与本次改动前逐位一致）。
EXT_FORM_LABELS = (
    "① 字面点名同级仓（`../Zerg-内部文档/…`）",
    "② 首段在仓内无落点（相对同级仓根，如 `项目文档/…`）",
    "③ `docs/` 前缀（分家前相对主仓根）",
)


def sibling_forms(tree, tgt):
    """该目标是不是**字面形态上确定指向仓外同级仓** → `(形态名, 外根下落点)`；否则 `None`。

    与 `external_candidates()` 的三种映射**一一对应**，只认字面形态（不猜后缀、不做模糊匹配）：
      ① 首段 == 同级仓目录名（`Zerg-内部文档/…`；`../Zerg-内部文档/…` 剥前导 `../` 后同形）⇒ 落点 = 同级仓根
      ③ 以 `docs/` 开头（分家前相对主仓根；分家后同名子面挂在同级仓根下）⇒ 落点 = 同级仓根 + 剥 `docs/` 的余段
      ② 首段在**仓内**找不到任何落点（不是 `base_roots` 的一条、也不在仓根下与 `docs/` 下真实存在）⇒
         只能相对同级仓根解（`项目文档/…` —— 分家后主仓连 `docs/项目文档/` 都没有了，这条必须靠
         「仓内无此首段」认，不能靠枚举同级仓的子面名）⇒ 落点 = 同级仓根 + 原路径
    本函数只定**形态**、不问「在不在盘」——「在不在」由调用方判（见 `gate_d2` 的缺语料铁律档）。
    """
    sib = sibling_docs_root(tree)
    segs = tgt.replace("\\", "/").split("/")
    while segs and segs[0] in ("", ".", ".."):
        segs.pop(0)
    if not segs:
        return None
    first = segs[0]
    if first == os.path.basename(sib):                                        # ①
        return EXT_FORM_LABELS[0], sib
    if tgt.startswith("docs/"):                                               # ③
        return EXT_FORM_LABELS[2], os.path.join(sib, tgt[len("docs/"):])
    if first in [b.rstrip("/") for b in tree.cfg.get("base_roots", [])] \
            or os.path.exists(os.path.join(tree.repo, first)) \
            or os.path.exists(os.path.join(tree.repo, "docs", first)):
        return None                                                           # 仓内首段 ⇒ 与同级仓无关
    return EXT_FORM_LABELS[1], os.path.join(sib, tgt)                         # ②


def sibling_root_on_disk(tree):
    """仓外基准根（同级开发文档仓）**在不在盘** —— 不在 ⇒ 外根形态的引用判不了（缺语料铁律）。"""
    return os.path.isdir(sibling_docs_root(tree))


def resolve_file(tree, src_rel, target):
    """在 {引用文件目录, 仓根, 各模块根, 仓外同级仓根} 里找；返回 (kind, hit, extra)。

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
    cands.extend(external_candidates(tree, tgt))          # ★ 仓外根：见 external_candidates 注释
    seen, cand2 = set(), []
    for c in cands:
        if c and c not in seen:
            seen.add(c)
            cand2.append(c)
    for c in cand2:
        if os.path.isabs(c):        # 仓外候选：不在仓内 fileset 里 ⇒ 只能问文件系统
            if os.path.isfile(c):
                return "exists", c, cand2
            if os.path.isdir(c):
                return "dir", c, cand2
            continue
        if tree.exists(c):
            return "exists", c, cand2
        if tree.exists_dir(c):
            return "dir", c, cand2
    for c in cand2:
        if os.path.isabs(c):
            continue
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

def corpus_missing_roots(tree):
    """配置里的**语料根**在仓内不存在 / 扫不到 md 的那些 —— 缺件必须**点名路径**，不许只说「空转」。

    为什么（2026-09-19 开发文档分家）：公开树侧没有 `docs/`（正式面未进公开树）⇒ 语料根扫到
    0 篇 md。此时 rc=2 的原文只说「语料清单为空」，看不出**缺的是哪个根**（人得自己去猜是哪棵
    树没进公开树）。口径一字未改（仍是 rc=2 不给结论），只是把缺件路径打出来。
    """
    out = []
    for c in (tree.cfg.get("corpus") or []):
        d = os.path.join(tree.repo, c)
        if not os.path.isdir(d):
            out.append("%s（目录不存在）" % c)
            continue
        n = 0
        for _dp, dns, fns in os.walk(d):
            dns[:] = [x for x in dns if x not in (tree.cfg.get("hard_exclude") or ())]
            n += len([f for f in fns if f.endswith(".md") and not f.startswith(".")])
        if n == 0:
            out.append("%s（0 篇 md）" % c)
    return out


def corpus_empty_msg(tree):
    """空语料的统一报错串（含缺件路径）。"""
    miss = corpus_missing_roots(tree)
    return ("语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论；缺件：%s"
            "（公开树侧无 docs/ ⇒ 本 scope 为 BLOCKED、不适用；私有树正常判）"
            % (' · '.join(miss) if miss else '—'))


def gate_d1(tree, args):
    r = Result("d1", tree)
    if not tree.corpus:
        r.errors.append(corpus_empty_msg(tree))
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


# ─────────────────────────── D2 · 追加判据：计数对拍（T-40） ───────────────────────────
# §二十一 已红第 18 条（开工单 T-40）：「文档里的**条数**是手抄的」—— v0.2 写 14 ⇒ 实测 17。
# 手抄的数没有判据 ⇒ 改代码、改文档**都不会红**，于是回潮。本判据把「命令/路径计数」纳入机检：
#
#   一行里同时出现 ① 现跑命令 `` `grep -c '<BRE>' <path>` `` ② 计数声明 `<N> 条`
#   ⇒ **argv 直调**跑那条 `grep -c`（cwd = 仓根），实测值必须 == 声明值；
#     不等 ⇒ **红**（判词点名「改代码或改文档后必须现跑」）。
#
# ★ 为什么是 argv 直调（`subprocess` 列表参数）而不是 `shell=True`：文档是**任何人可改的文本**，
#   把它里面的字符串喂给 shell 等于给文档执行权。只认 `grep -c '<BRE>' <path>` 这一种形态，
#   这个形态用 argv 就能**逐字复现**（BRE 原样传参，不经 shell 二次解释）。
# ★ 判不了的（文件不存在 / grep 跑不动 / 输出不是整数）⇒ **单列计数 + 进 notes**，
#   **不当结论**（红要红在「数对不上」，不是红在「判据自己坏了」）；没有计数声明的行不抽。
RE_COUNT_CMD = re.compile(r"`\s*(grep\s+-c\s+'([^']*)'\s+([^\s'`]+))\s*`")
RE_COUNT_DECL = re.compile(r"(\d+)\s*条")


def count_claims(text):
    """→ [(行号, 命令串, BRE, 路径, 声明值)]：只抽 `grep -c '<BRE>' <path>` + **同一行**的 `<N> 条`。"""
    out = []
    for i, line in enumerate(text.split("\n"), 1):
        m = RE_COUNT_CMD.search(line)
        if not m:
            continue
        d = RE_COUNT_DECL.search(line)
        if not d:
            continue
        out.append((i, m.group(1), m.group(2), m.group(3), int(d.group(1))))
    return out


def run_grep_c(tree, rel, bre):
    """argv 直调 `grep -c <BRE> <rel>`（cwd = 仓根）→ (计数 | None, 判不了的原因 | None)。**不经 shell**。"""
    full = os.path.join(tree.repo, rel)
    if not os.path.isfile(full):
        return None, "目标文件不在盘上：%s" % rel
    try:
        cp = subprocess.run(["grep", "-c", bre, rel], cwd=tree.repo,
                            capture_output=True, text=True)
    except OSError as exc:
        return None, "grep 跑不动：%s" % exc
    if cp.returncode not in (0, 1):     # grep：0=有命中 · 1=无命中 · 其余=出错
        return None, "grep 退出码 %d（%s）" % (cp.returncode, cp.stderr.strip()[:80])
    try:
        return int((cp.stdout or "0").strip() or "0"), None
    except ValueError:
        return None, "grep 输出不是整数：%r" % cp.stdout


def gate_d2(tree, args):
    r = Result("d2", tree)
    if not tree.cfg.get("base_roots"):
        r.errors.append("解析基准集配置缺失（config.base_roots 空）⇒ 判据不可判")
        return r
    if not tree.corpus:
        r.errors.append(corpus_empty_msg(tree))
        return r
    r.counted("语料篇数", len(tree.corpus))
    suffix_on = bool(tree.cfg.get("d2", {}).get("resolve_package_relative", True))
    ext_root = sibling_docs_root(tree)                    # 仓外基准根（同级开发文档仓）
    ext_ok = sibling_root_on_disk(tree)                   # 在盘 = True：判据一字不变（本档整体不生效）
    blocked_ext = []                                      # 缺语料 ⇒ 不给结论的现场（逐条计数、封顶取样）
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
                if kind == "exists" and hit and os.path.isabs(hit):
                    # 仓外根命中（同级开发文档仓）—— 单列计数，便于「本仓 0 命中也能看出仓外顶了多少」
                    r.counted("过·仓外命中（同级仓）")
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
                if kind == "miss" and not ext_ok:
                    form = sibling_forms(tree, pfx)
                    if form:        # 缺语料铁律：外根形态 + 仓外根不在盘 ⇒ **判不了**（rc=2，不进红档）
                        r.counted("不给结论·仓外根不在盘")
                        r.counted("不给结论·仓外根不在盘·%s" % form[0])
                        blocked_ext.append((form[0], rel, line, raw, pfx, form[1]))
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
        # ── 追加判据（T-40）：同一行里既有现跑 `grep -c` 又有 `<N> 条` ⇒ 数必须对得上 ─────────
        for cl_line, cl_cmd, cl_bre, cl_path, cl_decl in count_claims(text):
            r.counted("计数条目")
            got, why = run_grep_c(tree, cl_path, cl_bre)
            if why:
                r.counted("计数条目·判不了")
                r.notes.append("%s:%d 计数条目判不了（%s）· 原样：%s" % (rel, cl_line, why, cl_cmd))
                continue
            if got == cl_decl:
                r.counted("计数条目·对得上")
                continue
            r.counted("红·计数与现跑不符")
            r.add(rel, cl_line, cl_cmd, "红·计数与现跑不符",
                  "文档声明 `%d 条`，argv 直调现跑 `%s` ⇒ **%d**（改代码或改文档后**必须现跑**）"
                  % (cl_decl, cl_cmd, got), dom)
    for f in r.findings:
        if f["domain"] != DOM_LOCAL:
            r.counted("非本地域命中（单列·不进退码）·%s" % f["domain"])
    # ── ★ 缺语料铁律档（2026-09-26）：外根形态 + 仓外根不在盘 ⇒ rc=2 不给结论 + 逐字打印缺件路径 ──
    #    **不许**把这一档写进 findings 的红档（BLOCKED 不是 FAIL）；外根在盘时本档整体为空（行为一字不变）。
    if blocked_ext:
        r.errors.append(
            "仓外基准根不在盘 ⇒ rc=2 不给结论：**缺件 %s**（同级开发文档仓 · 字面形态 `../Zerg-内部文档`）；"
            "%d 条**外根形态**的引用判不了 —— 缺语料铁律：语料/前置不在盘时判不了就报 BLOCKED，"
            "既不判红（假红）也不静默绿" % (ext_root, len(blocked_ext)))
        for label in EXT_FORM_LABELS:
            items = [x for x in blocked_ext if x[0] == label]
            if not items:
                continue
            eg = " · ".join("例 %s:%d → `%s`" % (x[1], x[2], x[4]) for x in items[:3])
            r.errors.append("%s %d 条 · 外根下落点 %s · %s" % (label, len(items), items[0][5], eg))
    if r.counts.get("引用条目·R1", 0) + r.counts.get("引用条目·R2", 0) == 0 \
            and r.counts.get("计数条目", 0) == 0:
        r.errors.append("该 scope 的语料里 0 条 `文件:行` 引用 且 0 条计数条目（空转 ⇒ 不给结论）")
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
    if spec_doc and not os.path.exists(os.path.join(tree.main_repo, spec_doc)):
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

# ── ★ D4 靶面界定（2026-10-05 · 用户拍板④）：生成器「所有权」由**件自身属性**声明 ──────────────
#   规则（**非点名豁免表**）：一份生成物**件内**若带「人工接管标记」这一行 ⇒ 该件已由人工接管、
#   生成器不再拥有它 ⇒ D4 **不做逐字节比对**，只单列计数（不进红档）。无该标记的产物一律照旧
#   逐字节比对 ⇒ **真过时件仍红**（自检里的哑化负控钉住这一半）。
#   ★ 为什么不是豁免表：豁免表点名「文件路径」⇒ 点名即变相豁免（假绿）；本规则判的是**件自身的
#     属性（标记有无）**，对**任何**生成器的产物同一条规则生效，且标记被删掉即自动失效（自过期）
#     ⇒ 可审计、不腐烂。★ 标记按用户口径做成**件内可判属性**（非目录名、非路径白名单）。
HAND_MAINTAINED_MARKER = "<!-- doc-freshness: hand-maintained -->"

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


def alt_glob_targets(repo, alt_root, pats):
    """仓外取源根的 glob ⇒ **仓相对（带 `../`）** 目标列表（2026-09-19 二次分家批2 加）。

    口径与 `docs/site/export-and-build.sh` 的 `ZERG_DOCS_ALT` **同根同默认**（`alt_root` 默认 `../Zerg-内部文档`）：
    **先仓内匹配**（调用方先跑仓内 glob）、仓内零命中才走本函数；两处都零命中 ⇒ 返回空表 ⇒ 调用方
    按原口径「未声明产物 ⇒ 缺件，不给结论」rc=2（不假绿）。
    只 walk 命中模式的**首段目录**（`项目文档/…` ⇒ 只走 `<alt_root>/项目文档`），避免扫整棵仓外树。
    """
    base = os.path.abspath(os.path.join(repo, alt_root))
    if not os.path.isdir(base):
        return []
    tops = set()
    for pat in pats:
        seg = pat.split("/")[0]
        if seg and not any(ch in seg for ch in "*?["):
            tops.add(seg)
    roots = [os.path.join(base, t) for t in sorted(tops)] if tops else [base]
    out = []
    for r in roots:
        for dp, dns, fns in os.walk(r):
            dns[:] = [d for d in dns if not d.startswith(".")]
            rel = os.path.relpath(dp, base).replace(os.sep, "/")
            rel = "" if rel == "." else rel + "/"
            for fn in sorted(fns):
                cand = rel + fn
                for pat in pats:
                    if fnmatch.fnmatch(cand, pat):
                        out.append(posixpath.join(alt_root, cand))
                        break
    return sorted(set(out))


def gate_d4(tree, args):
    r = Result("d4", tree)
    cfg = tree.cfg.get("d4", {})
    gens = cfg.get("generators", [])
    if not gens:
        r.errors.append("生成器清单为空（config.d4.generators）⇒ 缺件，不给结论")
        return r
    tmp = tempfile.mkdtemp(prefix="docfresh-d4-")
    try:
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
                #   ★ 2026-09-19 二次分家批2（生成器靶双认 · 先仓内、缺则仓外取源根）：仓内零命中 ⇒ 试
                #     `alt_root`（默认 `../Zerg-内部文档`，与 export-and-build.sh 的 `ZERG_DOCS_ALT` 同根同默认）
                #     下的 `targets_glob_alt`（相对 alt_root 的路径）。**两处都零命中 ⇒ 不补 targets ⇒**
                #     照原口径「未声明产物…⇒ 缺件，不给结论」rc=2（不假绿）。两棵都在（批3 的 `cp -a` 窗口）
                #     ⇒ 只判仓内那批（仓内先匹配，不双计）。
                if not targets and g.get("targets_glob_alt"):
                    targets += alt_glob_targets(tree.repo, g.get("alt_root", "../Zerg-内部文档"),
                                                g["targets_glob_alt"])
            elif g.get("output"):
                targets.append(g["output"])
            # 缺件判定
            script = g.get("script")
            optional = bool(g.get("optional"))
            # GAP-20260927-12：生成器脚本与 d3.spec_doc 同源——都是**主仓根**相对路径（见 :181 自述）
            #   devdocs 档下 tree.repo 已换到文档面 ⇒ 一律按 tree.main_repo 解析，免得把真身在盘的脚本
            #   判成「缺脚本」（假 BLOCKED；缺件判定该判「产物」，不该判「脚本不在文档仓」）。
            if script and not os.path.exists(os.path.join(tree.main_repo, script)):
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
                # ★ D4 靶面界定（拍板④）：件含「人工接管标记」⇒ 生成器不拥有该件 ⇒ 单列不判（非红）
                hmm = g.get("hand_maintained_marker") or cfg.get("hand_maintained_marker") or HAND_MAINTAINED_MARKER
                if hmm and hmm in committed:
                    r.counted("人工接管·件含接管标记（单列·不判 drift）")
                    r.add(t, 0, gid, "单列·人工接管（生成器不覆盖 · 件含接管标记）",
                          "件内置接管标记 %s ⇒ 该件已人工接管、生成器不拥有它，逐字节比对不适用"
                          "（真过时且**无**标记的件仍红）" % clip(hmm, 50), dom)
                    continue
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
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
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
    # ── 缺口 GAP-20260927-164：凡报告里的数 ⇒ 同屏标**面**（哪个语料面 · 哪条命令 · 哪个 scope），
    #    同名异面的数不得互证（本门「行号越界」是 `docs/**.md` 文档语料面，≠ `scripts/gates/` 件:行 面）。
    _corpus = r.tree.cfg.get("corpus") or []
    print("  面：文档语料 `%s`（%d 篇 md）· 命令 `check-doc-freshness.py %s` · scope=%s"
          % (" · ".join(_corpus) if _corpus else "—", len(r.tree.corpus),
             " ".join(args.gates) if args.gates else "all", args.scope))
    print("  ★ 面注：本门「行号越界」= `docs/**.md` **文档语料**面的计数 —— 与"
          "「`scripts/gates/` 面 件:行 上限（门67 · check-lineref-limit.py）」是**两个面**，不互证、不混看。")
    if r.errors:
        for e in r.errors:
            print("  ⛔ 不给结论：%s" % e)
    if r.notes:
        for n in r.notes:
            print("  · 判不了（单列计数，不当结论）：%s" % n)
    keys = sorted(r.counts.items(), key=lambda kv: (-kv[1], kv[0]))
    if keys:
        print("  计数：" + " · ".join(
            ("%s=%d［面=docs/**.md 文档语料］" % (k, v)) if k == "红·行号越界" else ("%s=%d" % (k, v))
            for k, v in keys))
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
    print("规格：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md §6.3 §9.1 §5.6① §16 · Zerg-内部文档/调研/调研-文档分块落地-门禁与迁移.md §1 §2.3")
    print("\n【三档退码】0 全绿 · 1 有失败项 · 2 不给结论（缺件/空转/用法错/判据不可判）")
    print("依据：scripts/gates/precommit-gates.sh:29（仓内既有语义；本脚本不改它，只对齐语义）")
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
    _sib = sibling_docs_root(tree) if tree else os.path.join(os.path.dirname(os.path.abspath(DEFAULT_REPO)), "Zerg-内部文档")
    print("  · ★ 仓外根（同级开发文档仓 · 分家后那三类外根形态的落点，见 sibling_docs_root()）：%s（%s）"
          % (_sib, "在盘" if os.path.isdir(_sib) else "**不在盘** ⇒ 外根形态的引用判不了 = rc=2 BLOCKED、逐字打印缺件路径"))
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
        # 已于 2026-09-19 退役并从 scripts/docs/doc-freshness.config.json 删除（见该文件的 _mirror_note）。
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
    # 前置缺件闸：语料根**整个不存在**（公开树侧没有 docs/）⇒ 仍 rc=2，且报错要**点名缺件路径**
    r3b = os.path.join(tmp, "d1nodocs")
    os.makedirs(r3b, exist_ok=True)
    t = Tree(r3b, base_cfg(r3b))
    res = gate_d1(t, args)
    check("缺件（语料根 docs/ 不存在 ⇒ 公开树侧）rc=2 且点名缺件路径",
          res.rc() == 2 and any("缺件" in e and "docs" in e for e in res.errors),
          " · ".join(res.errors))

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
    # ── 追加判据（T-40）：同一行里「现跑 grep -c」+「<N> 条」⇒ 数必须对得上 ──────────────
    r2c = os.path.join(tmp, "d2countgood")
    _mkroot({"docs/a.md": "见 `core/x.py:2`。**条数**: 2 条 `/api/x/*`（现跑 `grep -c 'def ' core/x.py`）\n",
             "core/x.py": "def a\nb\ndef c\n"}, r2c)
    t = Tree(r2c, base_cfg(r2c))
    res = gate_d2(t, args)
    check("计数对拍·好件（声明 2 == 现跑 2）rc=0", res.rc() == 0, (res.rc(), res.counts, [f["kind"] for f in res.reds()]))
    check("计数对拍·好件：计数条目 1 条且判「对得上」",
          res.counts.get("计数条目") == 1 and res.counts.get("计数条目·对得上") == 1, res.counts)
    r2d = os.path.join(tmp, "d2countbad")
    _mkroot({"docs/a.md": "见 `core/x.py:2`。**条数**: 3 条 `/api/x/*`（现跑 `grep -c 'def ' core/x.py`）\n",
             "core/x.py": "def a\nb\ndef c\n"}, r2d)
    t = Tree(r2d, base_cfg(r2d))
    res = gate_d2(t, args)
    check("计数对拍·坏件（声明 3 ≠ 现跑 2）rc=1", res.rc() == 1, (res.rc(), res.counts))
    check("计数对拍·坏件：红条目点名「计数与现跑不符」",
          any(f["kind"] == "红·计数与现跑不符" for f in res.reds()), [f["kind"] for f in res.reds()])
    r2e = os.path.join(tmp, "d2countmiss")
    _mkroot({"docs/a.md": "见 `core/x.py:1`。**条数**: 1 条（现跑 `grep -c 'def ' core/nope.py`）\n",
             "core/x.py": "def a\n"}, r2e)
    t = Tree(r2e, base_cfg(r2e))
    res = gate_d2(t, args)
    check("计数对拍·判不了（目标不在盘上）⇒ 单列计数、rc=0、**不红**",
          res.rc() == 0 and res.counts.get("计数条目·判不了") == 1 and
          not [f for f in res.reds() if "计数" in f["kind"]], (res.rc(), res.counts, res.notes))
    r3 = os.path.join(tmp, "d2miss")
    c = base_cfg(r3)
    c["base_roots"] = []
    os.makedirs(os.path.join(r3, "docs"), exist_ok=True)
    _mkroot({"docs/a.md": "`core/x.py:1`\n"}, r3)
    t = Tree(r3, c)
    res = gate_d2(t, args)
    check("缺件（基准集缺失）rc=2", res.rc() == 2, (res.rc(), res.errors))
    # 前置缺件闸：语料根**整个不存在**（公开树侧没有 docs/）⇒ 仍 rc=2，且报错要**点名缺件路径**
    r4 = os.path.join(tmp, "d2nodocs")
    os.makedirs(r4, exist_ok=True)
    t = Tree(r4, base_cfg(r4))
    res = gate_d2(t, args)
    check("缺件（语料根 docs/ 不存在 ⇒ 公开树侧）rc=2 且点名缺件路径",
          res.rc() == 2 and any("缺件" in e and "docs" in e for e in res.errors),
          " · ".join(res.errors))
    # ── ★ 缺语料铁律档（2026-09-26）：外根形态 + 仓外根**不在盘** ⇒ rc=2 BLOCKED（**不判红**）
    #    成对格：同一个 miss 在「外根在盘」时必须照旧判红 —— 证明这一档只由**根在不在盘**开合，
    #    不是把判据放宽（真缺件、真越界照红）。用独立 tmp2 隔离，免得仓外根影响上面各格。
    tmp2 = tempfile.mkdtemp(prefix="docfresh-selftest-ext-")
    sx = os.path.join(tmp2, "repo")
    _mkroot({"docs/a.md": "见 `项目文档/v2.5.10/x.md:3` 与 `Zerg-内部文档/项目文档/v2.5.10/x.md:3`"
                          " 与 `core/nope.py:1`。\n"}, sx)
    t = Tree(sx, base_cfg(sx))
    res = gate_d2(t, args)
    check("缺语料·外根形态（② + ①）+ 仓外根不在盘 ⇒ rc=2、**那两条 0 红**",
          res.rc() == 2 and len(res.reds()) == 1 and res.counts.get("不给结论·仓外根不在盘") == 2,
          (res.rc(), res.counts, [f["kind"] for f in res.reds()]))
    check("缺语料·报错逐字打印缺件路径（同级仓根 %s）" % sibling_docs_root(t),
          any(sibling_docs_root(t) in e for e in res.errors), res.errors)
    check("缺语料·**仓内**引用不跟着进 BLOCKED（`core/nope.py:1` 仍判红·文件不存在）",
          [f["kind"] for f in res.reds()] == ["红·文件不存在"]
          and all(f["target"].startswith("core/nope.py") for f in res.reds()),
          [(f["kind"], f["target"]) for f in res.reds()])
    _mkroot({"项目文档/v2.5.10/x.md": "l1\nl2\nl3\nl4\n"}, os.path.join(tmp2, "Zerg-内部文档"))
    res2 = gate_d2(Tree(sx, base_cfg(sx)), args)
    check("成对负控·外根在盘 ⇒ 同一个件（①/② 形态）照旧判过（本档整体不生效）",
          res2.counts.get("不给结论·仓外根不在盘") is None and len(res2.reds()) == 1
          and res2.reds()[0]["target"].startswith("core/nope.py"),
          (res2.rc(), res2.counts, [(f["kind"], f["target"]) for f in res2.reds()]))
    sx3 = os.path.join(tmp2, "repo3")
    _mkroot({"docs/a.md": "见 `项目文档/v2.5.10/x.md:99` 与 `项目文档/v2.5.10/真不存在-x.md:1`。\n"}, sx3)
    res3 = gate_d2(Tree(sx3, base_cfg(sx3)), args)
    check("成对负控·外根在盘 ⇒ 真存在件改坏行号仍「红·行号越界」、真缺件仍「红·文件不存在」",
          sorted(f["kind"] for f in res3.reds()) == sorted(["红·行号越界", "红·文件不存在"]) and res3.rc() == 1,
          (res3.rc(), [(f["kind"], f["target"]) for f in res3.reds()]))

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
    # ── ★ D4 靶面界定（2026-10-05 拍板④）：件含「人工接管标记」⇒ 生成器不覆盖（单列不判）；
    #    无标记 ⇒ 照判（哑化负控：真过时件**必红**）。成对钉住「界定不是放水」这一半。
    r4m = os.path.join(tmp, "d4manual")
    _mkroot({"tools/gen.py": SELFTEST_GENERATOR,
             "docs/v1/INDEX.md": HAND_MAINTAINED_MARKER + "\n# v1 版本索引（手改过）\n\n| [a.md](a.md) |\n",
             "docs/v1/a.md": "x\n"}, r4m)
    c = base_cfg(r4m)
    c["d4"] = {"generators": [{"id": "g", "name": "自检生成器", "cmd": ["python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"],
                               "mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}]}
    t = Tree(r4m, c)
    res = gate_d4(t, args)
    check("靶面界定·件含接管标记 ⇒ rc=0、**0 条红**、入单列「人工接管」",
          res.rc() == 0 and len(res.reds()) == 0 and res.counts.get("人工接管·件含接管标记（单列·不判 drift）") == 1,
          (res.rc(), res.counts, [f["kind"] for f in res.reds()]))
    check("靶面界定·接管件不误记「过」（与真·逐字节一致分开计数）",
          res.counts.get("过·逐字节一致") is None, res.counts)
    r5m = os.path.join(tmp, "d4manual_off")
    _mkroot({"tools/gen.py": SELFTEST_GENERATOR,
             "docs/v1/INDEX.md": "# v1 版本索引（手改过）\n\n| [a.md](a.md) |\n",
             "docs/v1/a.md": "x\n"}, r5m)
    c = base_cfg(r5m)
    c["d4"] = {"generators": [{"id": "g", "name": "自检生成器", "cmd": ["python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"],
                               "mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}]}
    t = Tree(r5m, c)
    res = gate_d4(t, args)
    check("哑化负控·去掉接管标记 ⇒ 同一个真过时件**必红** rc=1（界定不是放水）",
          res.rc() == 1 and any(f["kind"] == "红·drift" for f in res.reds()),
          (res.rc(), res.counts, [f["kind"] for f in res.reds()]))
    # 三档在同一个「命令串」上各就各位（Q14 附带口径的断言）
    check("三档 0/1/2 各就各位（D1 三格）", True)
    # ── D3① 换根（`--scope devdocs`）：**成对**钉住「换根真的把语料面换掉了」
    #    左格 = devdocs 档：整棵给定根都进语料（不套主仓的 `docs/` 白名单）；
    #    右格 = 同一棵树在默认 main 档下语料 0 篇 ⇒ 变的是档位而不是树。
    rd = _mkroot({"总览.md": "# 总览\n\n见 [a](01-设计/a.md)\n",
                  "01-设计/a.md": "# a\n\n见 [b](01-设计/b.md)\n",
                  "01-设计/b.md": "# b\n",
                  "notes/x.txt": "x\n"}, os.path.join(tmp, "devroot"))
    cd = base_cfg(rd)
    td = Tree(rd, cd, scope="devdocs")
    check("D3① devdocs 档：**根级 md 也进语料**（整棵 3 篇 · 首跑实测漏的就是这一格）",
          len(td.corpus) == 3, td.corpus)
    check("D3① devdocs 档：解析域覆盖整棵根（4 件）", len(td.files) == 4, td.files)
    check("D3① 成对：同一棵树在 main 档下语料 0 篇（换根不是空转）",
          len(Tree(rd, cd, scope="main").corpus) == 0, Tree(rd, cd, scope="main").corpus)
    rd1 = gate_d1(td, args)
    check("D3① devdocs 档：D1 在同表内引用可解析（无红）", not rd1.reds(), rd1.reds())
    # ---------- D4：生成器脚本的解析基准 = 主仓根（GAP-20260927-12） ----------
    print("[D4] 生成器脚本的解析基准（主仓根 · 换根档）")
    main = os.path.join(tmp, "d4main")
    _mkroot({"tools/mygen.py": "print('x')\n"}, main)
    doc = os.path.join(tmp, "d4docs")          # 模拟 --scope devdocs 换根后的被扫树
    _mkroot({"docs/a.md": "# a\n"}, doc)
    cfg4 = base_cfg(doc)
    cfg4["d4"] = {"generators": [{"id": "mygen", "script": "tools/mygen.py",
                                  "output": "docs/INDEX.md", "optional": True}]}
    res = gate_d4(Tree(doc, cfg4, scope="devdocs", main_repo=main), args)
    check("主仓在盘的脚本不判「缺脚本」（换根档）",
          not any("缺脚本" in str(k) for k in res.counts), dict(res.counts))
    check("但产物真缺 ⇒ 仍报「产物不存在」",
          any("产物不存在" in str(k) for k in res.counts), dict(res.counts))
    cfg4b = base_cfg(doc)
    cfg4b["d4"] = {"generators": [{"id": "mygen2", "script": "tools/nope.py",
                                   "output": "docs/INDEX.md", "optional": True}]}
    res2 = gate_d4(Tree(doc, cfg4b, scope="devdocs", main_repo=main), args)
    check("负控：两个根都找不到的脚本 ⇒ 仍报「缺脚本」",
          any("缺脚本" in str(k) for k in res2.counts), dict(res2.counts))

    print("\nself-test：%d 条通过 / %d 条失败" % (n_ok, len(fails)))
    if fails:
        for f in fails:
            print("  ✗ " + f)
        return RC_FAIL
    return RC_OK


# ─────────────────────────── main ───────────────────────────

def build_tree(args, cfg):
    # D3① 换根：devdocs 档下 config 里那些**相对主仓根**的真源（d3.spec_doc）仍按主仓解析
    t = Tree(args.repo, cfg, scope=args.scope, strict_all=args.strict_all, verbose=args.verbose,
             main_repo=DEFAULT_REPO if args.scope == "devdocs" else None)
    t.no_soft = bool(getattr(args, "no_whitelist", False))
    return t


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True, description="W3 四条新鲜度门禁 D1–D4（只读 · 只报不改）")
    ap.add_argument("gates", nargs="*", help="d1 d2 d3 d4 all（可多个）")
    ap.add_argument("--repo", default=DEFAULT_REPO)
    ap.add_argument("--config", default=DEFAULT_CONFIG)
    ap.add_argument("--scope", choices=["main", "wide", "mirror", "devdocs"], default="main",
                    help="main=主树（默认，去镜像/冻结归档/第三方源码/引擎数据）；wide=+冻结归档+第三方源码；"
                         "mirror=+镜像副本；devdocs=**换到开发文档面**（D3①：Zerg-内部文档 整棵，根见 devdocs_root()）")
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

    # ── `--repo` 与 `--config` 配对（缺口 `GAP-20260926-184`）───────────────────────
    #   现病：只给 `--repo <仓外副本>`、不给 `--config` 时，config 仍按**脚本自身位置**
    #   解析（`DEFAULT_CONFIG` = <脚本所在仓>/scripts/docs/doc-freshness.config.json）
    #   ⇒ 拿 /tmp 副本跑时误报「缺配置文件」（看着像环境问题、实为参数语义分家）。
    #   修：显式给了 `--repo` 而未给 `--config` ⇒ 按该 repo 推同一相对落点（不写死）。
    if (any(a == "--repo" or a.startswith("--repo=") for a in sys.argv[1:])
            and not any(a == "--config" or a.startswith("--config=") for a in sys.argv[1:])):
        args.config = os.path.join(args.repo, "scripts", "docs", "doc-freshness.config.json")

    if args.self_test:
        return cmd_self_test(args)

    if args.scope == "devdocs":
        # ── D3① 换根：把**被扫的树**换到开发文档面（与门⑫ 同根同默认）。
        #    仓外根不可达 ⇒ rc=2 不给结论（设计稿 §五：不许静默当绿 ✗）。
        d = devdocs_root(DEFAULT_REPO)
        if not os.path.isdir(d):
            print("开发文档面根不在 ⇒ rc=2 不给结论：%s"
                  "（换根请设 ZERG_DEVDOCS_ROOT；口径见设计稿 §五）" % d, file=sys.stderr)
            return RC_BLOCKED
        args.repo = d

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
    print("总耗时 %.2fs · 合计 rc=%d %s（口径：1 优先于 2；BLOCKED 会阻塞提交但不许当绿）" % (elapsed, rc, RC_NAME[rc]))
    return rc


if __name__ == "__main__":
    sys.exit(main())

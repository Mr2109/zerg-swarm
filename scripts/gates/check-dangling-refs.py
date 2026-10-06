#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-dangling-refs.py v1.3.0 —— 悬空引用（`docs/…` 与跨仓 `Zerg-内部文档/…`·`项目文档/…` 指向不存在）
＋ GAP 号引用（注释/文档引的 `GAP-号` 不在缺口账内）

为什么存在（2026-09-22 · 2.5.11 第 3 项 · D 档）
------------------------------------------------
2026-09-19 文档分家（设计稿/调研稿/评审稿/问题单/模块档案整树搬到**同级仓外目录**
`Zerg-内部文档/`）之后，主仓里留下一批**指向已搬走目标的路径引用**：正则眼里它们是普通字符串，
编译器/渲染器都不会报，人眼在 277 处里也看不过来 ⇒ 它们会一直烂在工作树里。
本门把「主仓里 `docs/…` 解析后**不存在**」写成一条**可跑的判据**，并**分两类报**。

两类必须分开（本门的灵魂 —— 混在一起就会把台账改坏）
------------------------------------------------------
  · `live`（**活指路**）：引文在说明性正文/注释里，作用 = 「告诉读者真源/规格/依据在哪」。
    目标搬了 ⇒ **该改**（改指真源现落点）。
  · `ledger`（**机读清单 / 基线台账**）：引文把路径当**数据条目**记下来（机读件里的路径值 ·
    表格式清单 · 夹具/探针里的假路径 · 行内明写「台账/快照/冻结/已删除/当时/沿革」的记录措辞）。
    它记的是**当时的事实** ⇒ **不该改**（改了是改史）⇒ 走**登记表**（写进表 = 有意保留）。

★ 豁免 ≠ 隐藏（同 `check-zh-en.py` 的 `KNOWN_FP` / `doc-freshness.config.json` 的 allowlist 口径）：
  登记过的条目**照样逐条打印**，只标 `已登记` 且**不计命中**；每条**必须带 `reason` + `date`**
  （缺任一 ⇒ rc=2 —— 只给路径的登记表 = 无记录的洗白，一律不给结论 ✗）。

判据与输出（每处一行七格）
--------------------------
  `件:行 · 引文原文 · 解析后路径 · 在不在三棵树（主仓 / Zerg-内部文档 / Zerg-归档）· live|ledger · 判类理由 · 建议`

抓哪些形态（**已覆盖**）
------------------------
  ① 直书：`docs/…`（反引号里 / 注释里 / 字符串里 / frontmatter 里）—— 只看**仓内相对**的写法，
     `https://…/docs/x` 这类「前面还粘着东西」的**不算**（那不是主仓相对引用）
  ② `filepath.Join("docs", …, "件")` / `path.Join(…)` —— 把紧邻的字符串字面量按序拼成一条路径；
     首段是变量（如 `filepath.Join(workDir, "docs", "issues")`）时**按仓内相对判**并在输出里标出
  ③ 字符串拼接：`.py` 里的 `"docs/" + base` 这类**前缀形态** —— 只判到前缀目录（后缀是变量，判不了）
  ④ **跨仓直书**（2026-10-05 · `GAP-20261005-18`）：`Zerg-内部文档/<rel>` 与裸 `项目文档/<rel>` 两类形态 ——
     解析到**文仓根**（`ctx.docs_root`）后**不存在** ⇒ 悬空、存在 ⇒ 不报。此前本门只判裸 `docs/…` ⇒
     门件的**仓外/文仓引用无门看着**、出处链静默断（本条补上这一面）
  ⑤ **GAP 号引用**（2026-10-05 · `GAP-20261005-54`）：注释/文档里引的 `GAP-<8位日期>-<号>` 必须在
     **缺口账**（真源 `<ZERG_STATE_DIR|~/.zerg/state>/zerg-cli-gaps.jsonl` · 可用 `--ledger` 覆盖）里存在 ⇒
     不存在 ⇒ 引文悬空（逐字点名 件:行 · 引的号）。此前本门只判路径引用 ⇒ **丙面**（注释引的号账内不存在）
     **无门看着**、账外号静默烂在引用里。判定**复用本门同一个 `classify()`**：只有 `live` 类算命中（rc=1），
     `ledger`（夹具假号 / 机读件 / 数据字串 / 表格行）与`已登记`= **例外面**，照样逐条打印、不计命中。
     例外面另含**测试夹具件**（`*_test.*` / `testdata/` / `tests/` / `fixtures/` 里故意造的假号）与
     **本件自身**（`check-dangling-refs.py` 的自检夹具号 —— 自扫面命中的是它自己的夹具，非真引用）。
     账取不到 ⇒ 本类**未判**（打印 ⛔ 与候选数；**不翻**其余面的退码 —— 与归档树缺席同口径）

**未覆盖形态**（不假装全覆盖 —— 逐条钉在 `--list-rules` 与运行输出末行）
----------------------------------------------------------------------
  · 纯 URL（`https://…/docs/…`）—— 不是仓内相对引用，本门不判（`Zerg-内部文档/…` / 裸 `项目文档/…` **已判**，见上「已覆盖 ④」）
  · 变量拼进中段：`"docs/" + lang + "/x.md"`（后缀/中段是变量）—— 只判前缀目录那一层
  · 运行期路径：`filepath.Join(workDir, "docs", "issues")` 真身是**运行期在 workDir 下**建的目录，
    与主仓同名不同树 ⇒ 报出来但按 ledger 判（要有意保留就登记）
  · 软链 / 大小写变体 / 非 UTF-8 件 / 未 `git add` 的新件（本门只扫 `git ls-files`）
  · markdown 相对链接 `[](../x.md)` —— 那是**另一条判据**（`check-doc-freshness.py` D1），本门不重复
  · GAP 号引用类：账内**在册与否**取不到真源时**不判**（未判 · 不翻退码）；`--ledger` 覆盖仅供自检/夹具；
    **面半边**（引的号进不进「对面」）**无稳健机检** ⇒ 本门**不做**（见 `GAP-20261005-54`）

★ v1.3.0（批-09 · `GAP-20260926-111/112/114`）
----------------------------------------------
  · `--group-by src,tgt,form,cls`（GAP-20260926-111）：把**逐处清单**换成**按维度聚合**
    （源件域 `src` / 目标域 `tgt` / 形态 `form` / 判类 `cls`，逗号可多选；聚合面 = docs 路径类
    ＋ GAP 号类**全清单**）。此前分类只能靠**仓外自建脚本**。默认关 ⇒ **默认档一字未改**。
  · `--prefix-candidates`（GAP-20260926-112）：把**未登记的 `ledger` 命中**按（件 · 目录前缀）
    聚成**可登记的 registry entry 草稿**（JSON 数组 · `ref_prefix` = 该引文的目录前缀）。
    草稿的 `reason` / `date` **需人工补全**（本门只给候选，不代写理由 —— 登记表缺 reason ⇒ rc=2）。
    默认关 ⇒ 默认档一字未改。
  · **判据面修正**（GAP-20260926-114）：`filepath.Join(<变量>, "docs", …)` 这类**运行期路径**
    （`note` = 拼接首段=变量）**改判 `ledger`**。原判 `live` 与本门自己的 `suggest()` **自相矛盾**
    （suggest 打「运行期路径 ⇒ 不改只报」，`classify()` 却判 live 算命中）⇒ 误报类 FP-A。
    修正后**命中总数不变**（命中 = live + ledger），只动 **live / ledger 分栏**。
    配套：自检第 ④ 格加断言（运行期 ⇒ `ledger` · 字面量 ⇒ `live` 成对负控）。

退码（三档 · 与仓内既有门一致；**本门退码** —— 本步**接线档位** `tri-report` ⇒ rc=1 落
      REPORT：只报告、不计失败项、不影响退出码）
--------------------------------
  0 = 无命中（或命中全在登记表里）
  1 = 有**未登记**的悬空引用（live + ledger 两类合计）
  2 = **不给结论**（不在仓根 / 扫到 0 件 / 登记表缺 reason·date·形态不对 / 自检不过 /
      **同级语料树 `Zerg-内部文档` 取不到根** —— 落点判不了 ⇒ 不当 0 也不当 1）

★ 同级语料树取不到根 ⇒ **不给结论档（rc=2）**（2026-09-27 · `GAP-20260927-82` 批2）
----------------------------------------------------------------------
  改前：`tree_of()`（141 行）对缺席树一律 `continue` 跳过 ⇒ 同级语料树不在盘时照判主仓、
  末行落 `rc∈{0,1}` = **静默**（输出只印「**不在** —— 落点只报主仓」，读者会把「判不了」当成「已判完」）✗。
  改后：同级语料树 `Zerg-内部文档` **取不到根**（`ZERG_DOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS` /
  `<仓父>/Zerg-内部文档` 都不在盘）⇒ 打印缺件路径 + 换根候选后 `return 2`（逐条命中**仍照原样打印**，不藏）。
  取根规则与同族件归一（批1）：`ZERG_DOCS_ROOT` > `ZERG_DOCS_ALT` / `ZERG_DOCS` > `<仓父>/Zerg-内部文档`；
  `ZERG_ARCHIVE_ROOT` > `<仓父>/Zerg-归档`（**不新引 env 名**）。
  ★ **归档树缺席不翻退码**（只报主仓落点 · 与原档逐字一致）：本机 `../Zerg-归档` 是**未挂载的断链**
  （`-> /Volumes/ZERG-ARCHIVE`）⇒ 按「任一不在盘 ⇒ 2」会把**默认档**从 rc=1 翻成 rc=2 = 改现役行为 ✗
  （本批铁律：默认档零行为变更）；归档域的收口挂**批3**（与 `check-archive-mount.py` 同批）。
  **`Zerg-内部文档` 在盘时判据一字未改**（成对负控见 `--self-test` ⑩：语料树缺席 ⇒ 报 1 条；在盘 ⇒ 报 0 条）。

用法
----
    python3 scripts/gates/check-dangling-refs.py                 # 全仓扫（只报告）
    python3 scripts/gates/check-dangling-refs.py --all           # 打印全部命中（默认每类截 30 行）
    python3 scripts/gates/check-dangling-refs.py --live-only     # 只打 live 类
    python3 scripts/gates/check-dangling-refs.py --json          # 末尾追加机器面一行
    python3 scripts/gates/check-dangling-refs.py --root DIR --registry F   # 对夹具跑（自检内部用）
    python3 scripts/gates/check-dangling-refs.py --ledger FILE    # 覆盖缺口账（夹具/自检用）
    python3 scripts/gates/check-dangling-refs.py --group-by src,tgt,form,cls  # 按维度聚合（替逐处清单）
    python3 scripts/gates/check-dangling-refs.py --prefix-candidates         # 打可登记前缀候选草稿
    python3 scripts/gates/check-dangling-refs.py --list-rules    # 打判据 / 两类定义 / 覆盖与未覆盖
    python3 scripts/gates/check-dangling-refs.py --self-test     # 只跑自检（合成夹具，不碰真目标）
"""
import datetime
import json
import os
import re
import subprocess
import sys

VERSION = "check-dangling-refs.py v1.3.0"

# ── 扫描面 ───────────────────────────────────────────────────────────────────
SKIP_DIRS = ("vendor", "bin", "dist", "node_modules", "target", ".git", ".venv", "venv",
             "__pycache__", ".obsidian", ".history", "htmlcov", ".cargo")
SKIP_PREFIX = ("tools/ocr/venv/",)

# ── 三棵树（主仓 = cwd；另两棵默认取**同级目录**，可用环境变量覆盖）──────────
DOCS_TREE_NAME = "Zerg-内部文档"
ARCH_TREE_NAME = "Zerg-归档"

# ── 引文提取 ─────────────────────────────────────────────────────────────────
# 路径正文字符集：字母数字 + `_-./+@%~#` + CJK（件名里有中文）。
# **不含** 中英文标点 / 空白 / 引号 / 括号 / 通配符 —— 那样才不会把后面的散文一起吞掉。
PATHCHARS = r"A-Za-z0-9_\-./+@%~#一-鿿"
RE_DIRECT = re.compile(r"(?<![A-Za-z0-9_\-./])docs/[%s]*" % PATHCHARS)
RE_ANCHOR = re.compile(r"^:(\d+(?:-\d+)?)")
RE_JOIN = re.compile(r"(?<![A-Za-z0-9_.])(?:filepath|path)\.Join\s*\(([^()]*)\)")
RE_CONCAT = re.compile(r"""["']docs/["']\s*\+|["']docs["']\s*\+\s*["']/""")
RE_STRLIT = re.compile(r"""["']([^"']*)["']""")

# ── 跨仓引用（2026-10-05 · GAP-20261005-18）───────────────────────────────────
# 两类形态：`Zerg-内部文档/<rel>`（显式带文仓名）与裸 `项目文档/<rel>`（隐式指文仓）。
# 判据同「裸 docs/…」：解析到**文仓根**（ctx.docs_root）后不存在 ⇒ 悬空。
# 前字符不是路径体才算引用（与 RE_DIRECT 同手法，避免吞掉 `../项目文档/` 这类相对链）。
RE_XDOCS = re.compile(r"(?<![A-Za-z0-9_\-./])Zerg-内部文档/[%s]*" % PATHCHARS)
RE_XBARE = re.compile(r"(?<![A-Za-z0-9_\-./\u4e00-\u9fff])项目文档/[%s]*" % PATHCHARS)
XREPO_PREFIX = "Zerg-内部文档/"
XBARE_PREFIX = "项目文档/"

# ── GAP 号引用类（2026-10-05 · GAP-20261005-54）──────────────────────────────
# 注释/文档里引的 `GAP-<8位日期>-<号>` 必须在**缺口账**（真源 = 状态目录下的
# `zerg-cli-gaps.jsonl`）里存在；不存在 ⇒ 引文悬空（逐字点名 件:行 · 引的号）。
# 账是**仓外私有面**（`ZERG_STATE_DIR` > `~/.zerg/state`），可 `--ledger` 覆盖（夹具用）。
# 复用本门既有 live/ledger 两类 ＋ 登记表口径：只有 `live` 类判为**命中**（rc=1）；
# `ledger`（夹具假号 / 机读件 / 数据字串 / 表格行）与`已登记`= **例外面**，照样逐条打印、不计命中。
RE_GAP = re.compile(r"GAP-\d{8}-\d+")
GAP_LEDGER_FILE = "zerg-cli-gaps.jsonl"
# 例外面·测试夹具件：`*_test.<ext>` / `testdata/` / `tests?/` / `fixtures?/` 里「故意造的假号」——
# 与既有 `classify()` 同向（那几条判据本就是夹具容器），但**只对 GAP 号引用类**生效
# （改 classify() 会改动 docs 路径类的现役读数，✗ 不碰）。注意逃逸引号 `\"id\":\"GAP-…\"` 形态
# 进不了「引号紧包」判据（右侧是 `\` 不是引号）⇒ 靠本条兜住。
GAP_TEST_CONTAINER_RE = re.compile(r"(?:^|/)(?:testdata|tests?|fixtures?)/|_test\.(?:go|py|sh|ts|js)$")
# 本件自身的仓内相对路径（自检夹具命中自扫面 ⇒ 例外面；与登记表里既有条目同形态 · 硬写相对路径最稳）
GAP_SELF_REL = "scripts/gates/check-dangling-refs.py"


def is_gap_test_container(rel):
    return bool(GAP_TEST_CONTAINER_RE.search(rel))


def is_gap_self_file(ctx, rel):
    """本件（判据件自身）—— 它正文里的 GAP 号全是**自检夹具里的假号** ⇒ 例外面（照样打印·不计命中）。

    两道判：① 仓内相对路径 == `GAP_SELF_REL`（与登记表/件头引用同形态 · 最稳）；
    ② 兜底按**运行路径**判（本件被改名/移位时仍认得出）。
    """
    if rel == GAP_SELF_REL:
        return True
    try:
        return os.path.realpath(os.path.join(ctx.root, rel)) == os.path.realpath(os.path.abspath(__file__))
    except Exception:
        return False

# ── 分类（ledger 的四条容器判据 + live 的两条）──────────────────────────────
MACHINE_EXT = (".json", ".tsv", ".csv", ".txt")
FIXTURE_REFS = ("docs/a.md", "docs/x.md", "docs/y.md", "docs/zh/a.md", "docs/en/a.md",
                "docs/设计.md", "docs/活文档.md", "docs/规范不存在.md", "docs/design")
FIXTURE_WORDS = ("夹具", "探针", "self_test", "self-test", "自检", "用例", "仿真",
                 "fixture", "假路径", "负控")
RECORD_WORDS = ("台账", "基线", "快照", "冻结", "已删除", "已迁", "迁至", "迁出", "分家", "移出",
                "搬到", "原路径", "原本", "沿革", "当时", "历史", "登记", "豁免", "白名单",
                "allowlist", "清单", "保留", "不动", "空转", "曾是", "镜像副本")
LIVE_WORDS = ("规格", "真源", "依据", "设计稿", "调研稿", "评审稿", "出处", "详见",
              "参照", "来源", "见 `", "遵循")

RE_TABLE_ROW = re.compile(r"^\s*\|")

# ── 登记表形态 ───────────────────────────────────────────────────────────────
REGISTRY_DEFAULT = "scripts/gates/dangling-refs-registry.json"
RE_DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


class Ctx(object):
    """一次扫描的全部输入（真跑与自检**共用同一条路** —— 判据只写一份）。"""

    def __init__(self, root, docs_root, arch_root, files, registry, gap_ids=None):
        self.root = root
        self.docs_root = docs_root
        self.arch_root = arch_root
        self.files = files          # 仓内相对路径列表
        self.registry = registry     # {"entries": [...]} 或 {}
        self.gap_ids = gap_ids       # 缺口账在册号集（None = 账取不到 ⇒ GAP 号引用类不给结论）
        self._base_index = None
        self.read_fail = []

    # 懒建 basename 索引（只在真有悬空时才走 —— 真跑 0.2s 级）
    def base_index(self):
        if self._base_index is None:
            idx = {}
            for tree in (self.docs_root, self.arch_root):
                if not tree or not os.path.isdir(tree):
                    continue
                for dp, dn, fn in os.walk(tree):
                    dn[:] = [d for d in dn if d != ".git"]
                    for f in fn:
                        idx.setdefault(f, []).append(os.path.relpath(os.path.join(dp, f), tree))
            self._base_index = idx
        return self._base_index


def tree_of(ctx, rel):
    """解析后路径在**三棵树**里的落点（主仓优先；仓外两棵按「同路」+「去掉 docs/ 前缀的 D 档主形」）。"""
    if rel.startswith(XREPO_PREFIX) or rel.startswith(XBARE_PREFIX):
        return xrepo_tree_of(ctx, rel)
    out = []
    if os.path.exists(os.path.join(ctx.root, rel)):
        out.append("主仓/" + rel)
    rel2 = rel[len("docs/"):] if rel.startswith("docs/") else rel
    for name, tree in ((DOCS_TREE_NAME, ctx.docs_root), (ARCH_TREE_NAME, ctx.arch_root)):
        if not tree or not os.path.isdir(tree):
            continue
        if os.path.exists(os.path.join(tree, rel)):
            out.append(name + "/" + rel)
        elif rel2 != rel and os.path.exists(os.path.join(tree, rel2)):
            out.append(name + "/" + rel2)
    if not out and rel.startswith("docs/"):
        base = os.path.basename(rel.rstrip("/"))
        for hit in ctx.base_index().get(base, [])[:3]:
            out.append("（同名件）" + DOCS_TREE_NAME + "/" + hit)
    return out


def absent_corpus_trees(docs_root, arch_root):
    """**该翻 rc=2** 的缺席同级树（唯一真源 · 见件头 ★ · `GAP-20260927-82` 批2）。

    只认**同级语料树** `Zerg-内部文档`（主仓 `docs` 面引用的落点基准）：取不到 ⇒ 不给结论，既不当 0
    也不当 1。归档树缺席**不翻退码**（只报主仓落点）—— 本机 `../Zerg-归档` 是**未挂载的断链**，
    若按「任一不在盘 ⇒ 2」会把**默认档**从 rc=1 翻成 rc=2 = 改变现役行为 ✗（本批铁律：默认档零行为变更）。
    """
    return [(n, t) for n, t in ((DOCS_TREE_NAME, docs_root),) if not t or not os.path.isdir(t)]


def registry_hit(ctx, file_rel, ref):
    """登记表命中 ⇒ 返回 (理由, 判类, 日期, 登记形态)；否则 None。

    三种登记形态（**不许整件洗白**）：
      · `ref`        逐处登记（这一件里**这一条**引文）
      · `ref_prefix` 前缀登记（这一件里**以该前缀开头**的全部引文 —— 如 `docs/项目文档/`）
      · 两者都不给   ⇒ **rc=2 拒**（整件豁免 = 无记录的洗白；真机读件也请写 `ref_prefix: "docs/"`）
    """
    best = None
    for e in ctx.registry.get("entries", []):
        if e.get("file") != file_rel:
            continue
        eref = e.get("ref", "")
        epre = e.get("ref_prefix", "")
        if eref:
            if eref != ref:
                continue
            return (e.get("reason", ""), e.get("class", "ledger"), e.get("date", ""), "逐处")
        if epre and ref.startswith(epre):
            # 最长前缀优先（更具体的那条说了算）
            if best is None or len(epre) > len(best[0]):
                best = (epre, e)
    if best:
        e = best[1]
        return (e.get("reason", ""), e.get("class", "ledger"), e.get("date", ""),
                "前缀[%s]" % best[0])
    return None


def classify(ctx, file_rel, ref, line_text):
    """→ (cls, why)。顺序即优先级：登记表 > 夹具 > 机读件 > 表格行 > 记录措辞 > 指路措辞 > 默认。

    默认 = **live**：正文/注释里一条指不出去的路径引用，本来就是断的指针 ⇒ 宁可多报（本门只报告）。
    """
    reg = registry_hit(ctx, file_rel, ref)
    if reg:
        why, cls, date, form = reg
        return ("已登记", "登记表·%s：%s（%s）" % (form, why, date))
    if ref in FIXTURE_REFS or any(w in line_text for w in FIXTURE_WORDS):
        return ("ledger", "夹具/探针里的假路径")
    if file_rel.lower().endswith(MACHINE_EXT):
        return ("ledger", "机读件（清单/基线容器）")
    # 引文被引号**紧包** ⇒ 它是代码/数据里的字串（判据常量 · 配置值 · 夹具键值 · 台账 JSON 数据），
    # 不是散文里的指路 —— 这类「记当时路径」的东西改了就是改判据/改数据，一律 ledger。
    pos = line_text.find(ref)
    if pos >= 0:
        before = line_text[:pos].rstrip()
        after = line_text[pos + len(ref):].lstrip()
        if before[-1:] in ('"', "'") and after[:1] in ('"', "'"):
            return ("ledger", "代码/数据里的字串（判据常量 · 配置值 · 夹具键值）")
    if RE_TABLE_ROW.match(line_text):
        return ("ledger", "表格式清单/台账行")
    for w in RECORD_WORDS:
        if w in line_text:
            return ("ledger", "记录当时事实的措辞「%s」" % w)
    for w in LIVE_WORDS:
        if w in line_text:
            return ("live", "指路措辞「%s」" % w)
    return ("live", "默认：正文/注释里的路径引用（非记录性容器）")


def suggest(cls, ref, where, note=""):
    if cls == "已登记":
        return "按登记表保留（有意保留当时路径）· 不计命中"
    if "拼接首段=变量" in note:
        return "运行期路径（拼接首段是变量，不由仓根解析）⇒ 不改只报；要有意保留就登记（未覆盖形态之一）"
    if cls == "ledger":
        return "登记（记当时路径，不该改）" if not where else "登记；若确属活指路再改指 " + where[0]
    if where:
        return "改指 " + where[0]
    return "不改只报：三棵树都没有这个目标 ⇒ 先定去向（搬走 / 改名 / 删）"


def _looks_like_ref(ref):
    """是不是「引用形态」而不是被正则误吞的散文（如 `docs/` 的目录 ⇒ `docs/的目录`）。

    判据：目录型（带尾 `/`）⇒ 算；多段（`docs/<段>/…`）⇒ 算；单段且含 ASCII/数字/`._-` ⇒ 算。
    被这一条滤掉的不算命中，但**计数打印**（不静默 —— 口径见 --list-rules）。
    """
    if ref.endswith("/"):
        return True
    body = ref[len("docs/"):] if ref.startswith("docs/") else ref
    segs = [s for s in body.split("/") if s]
    if len(segs) >= 2:
        return True
    if not segs:
        return False
    return bool(re.search(r"[A-Za-z0-9._\-]", segs[0]))


def _looks_like_xref(ref):
    """跨仓引用是不是「引用形态」（比照 `_looks_like_ref`，但先剥跨仓前缀）。

    目录型 / 多段 / 单段且含 ASCII·数字·`._-` ⇒ 算；被滤掉的不算命中，但计数打印。
    """
    if ref.endswith("/"):
        return True
    body = ref
    for pfx in (XREPO_PREFIX, XBARE_PREFIX):
        if body.startswith(pfx):
            body = body[len(pfx):]
            break
    segs = [s for s in body.split("/") if s]
    if len(segs) >= 2:
        return True
    if not segs:
        return False
    return bool(re.search(r"[A-Za-z0-9._\-]", segs[0]))


def ref_exists(ctx, ref):
    """引用解析后的存在性：跨仓引用判**文仓根**；裸 `docs/…` 判**主仓根**（现役口径一字不变）。"""
    if ref.startswith(XREPO_PREFIX):
        if not ctx.docs_root:
            return False
        return os.path.exists(os.path.join(ctx.docs_root, ref[len(XREPO_PREFIX):].rstrip("/")))
    if ref.startswith(XBARE_PREFIX):
        if not ctx.docs_root:
            return False
        return os.path.exists(os.path.join(ctx.docs_root, ref.rstrip("/")))
    return os.path.exists(os.path.join(ctx.root, ref.rstrip("/")))


def xrepo_tree_of(ctx, rel):
    """跨仓引用的落点：在**文仓树**（Zerg-内部文档；归档树按同名形）里找现落点。"""
    out = []
    if rel.startswith(XREPO_PREFIX):
        sub = rel[len(XREPO_PREFIX):]
        for name, tree in ((DOCS_TREE_NAME, ctx.docs_root), (ARCH_TREE_NAME, ctx.arch_root)):
            if tree and os.path.isdir(tree) and os.path.exists(os.path.join(tree, sub)):
                out.append(name + "/" + sub)
        return out
    if ctx.docs_root and os.path.isdir(ctx.docs_root) \
            and os.path.exists(os.path.join(ctx.docs_root, rel)):
        return [DOCS_TREE_NAME + "/" + rel]
    return []


# ── 聚合 / 前缀候选（v1.3.0 · GAP-20260926-111/112；默认关 ⇒ 默认档零行为变更）──────
GROUP_DIMS = ("src", "tgt", "form", "cls")


def _dim_value(h, dim):
    """聚合维度取值（`--group-by`）。取值面**逐字确定**（两半逐字同）。"""
    if dim == "src":
        f = h["file"]
        return f.split("/")[0] if "/" in f else "(根级件)"
    if dim == "tgt":
        if h["form"] == "GAP号":
            return "(GAP号)"
        p = h["path"]
        for pfx in ("docs/", XREPO_PREFIX, XBARE_PREFIX):
            if p.startswith(pfx):
                p = p[len(pfx):]
                break
        segs = [s for s in p.split("/") if s]
        return segs[0] if segs else "(空)"
    if dim == "form":
        return h["form"]
    return h["cls"]


def _dir_prefix(ref):
    """引文的**目录前缀**（可登记 `ref_prefix` 候选）：最后一段之前 + `/`；单段 ⇒ 空（不可前缀化）。"""
    r = ref.rstrip("/")
    if "/" not in r:
        return ""
    return r[:r.rfind("/") + 1]


def prefix_candidates(hits, today=None):
    """未登记的 `ledger` 命中 ⇒ registry entry 草稿（按 件·目录前缀 聚合 · 排序确定）。"""
    groups = {}
    for h in hits:
        if h["cls"] != "ledger":
            continue
        pre = _dir_prefix(h["path"])
        if not pre:
            continue
        groups.setdefault((h["file"], pre), []).append(h["path"])
    date = today or datetime.date.today().isoformat()
    out = []
    for key in sorted(groups):
        f, pre = key
        items = groups[key]
        samples = sorted(set(items))[:3]
        out.append({"file": f, "ref_prefix": pre, "class": "ledger",
                    "reason": "候选：%d 处（--prefix-candidates 生成 · 需人工补理由）" % len(items),
                    "date": date, "count": len(items), "samples": " · ".join(samples)})
    return out


def extract(line, stats):
    """从一行里抽引文 → [(ref, 形态, 说明)]。"""
    out = []
    for m in RE_DIRECT.finditer(line):
        if _looks_like_ref(m.group(0)):
            out.append((m.group(0), "直书", ""))
        else:
            stats["refs_prose_skipped"] = stats.get("refs_prose_skipped", 0) + 1
    for m in RE_JOIN.finditer(line):
        # 只取**连续的字面量段**：首段是变量（workDir 这类）先跳过；遇到变量即止 ——
        # 否则 `filepath.Join(workDir, "docs", "issues", id+".md")` 会被拼成 `docs/issues/.md`（假的）
        lits, started, varfirst = [], False, False
        for arg in m.group(1).split(","):
            arg = arg.strip()
            lit = RE_STRLIT.fullmatch(arg)
            if lit:
                started = True
                lits.append(lit.group(1))
            elif not started:
                if arg:
                    varfirst = True
                continue
            else:
                break
        path = "/".join(x for x in lits if x != "")
        if not path.startswith("docs") or (path != "docs" and not path.startswith("docs/")):
            continue
        note = "拼接首段=" + ("变量（运行期根）" if varfirst else "字面量")
        out.append((path, "filepath.Join", note))
    for _ in RE_CONCAT.finditer(line):
        out.append(("docs", "字符串拼接", "前缀形态（后缀是变量 ⇒ 只判前缀目录）"))
    for rx in (RE_XDOCS, RE_XBARE):
        for m in rx.finditer(line):
            if _looks_like_xref(m.group(0)):
                out.append((m.group(0), "跨仓直书", ""))
            else:
                stats["refs_prose_skipped"] = stats.get("refs_prose_skipped", 0) + 1
    return out


def _read_text(path):
    try:
        raw = open(path, "rb").read()
    except Exception:
        return None
    if b"\x00" in raw:
        return None
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return None


def scan(ctx):
    """→ (统计, 命中列表)。命中 = 悬空且**未登记**。"""
    stats = {"files_scanned": 0, "files_seen": len(ctx.files), "refs": 0, "dangling": 0,
             "refs_prose_skipped": 0}
    hits = []
    seen = set()
    gap = {"hits": [], "exempt": [], "candidates": 0,
           "judged": ctx.gap_ids is not None, "unjudged": []}
    for rel in sorted(ctx.files):
        if any(rel.split("/")[0] == d for d in SKIP_DIRS) or rel.startswith(SKIP_PREFIX):
            continue
        full = os.path.join(ctx.root, rel)
        if not os.path.isfile(full):
            continue
        text = _read_text(full)
        if text is None:
            stats.setdefault("files_unreadable", []).append(rel)
            continue
        stats["files_scanned"] += 1
        for ln, line_text in enumerate(text.split("\n"), 1):
            for ref, form, note in extract(line_text, stats):
                # 行锚（`件:30`）不算路径的一部分 —— 但要在引文里留痕
                anchor = RE_ANCHOR.match(line_text[line_text.find(ref) + len(ref):]) \
                    if ref in line_text else None
                shown = ref + ((":" + anchor.group(1)) if anchor else "")
                stats["refs"] += 1
                key = (rel, ln, form, ref)
                if key in seen:
                    continue
                seen.add(key)
                if ref_exists(ctx, ref):
                    continue  # 落点存在 ⇒ 不悬空（跨仓引用判文仓根 · 裸 docs/… 判主仓根）
                stats["dangling"] += 1
                cls, why = classify(ctx, rel, ref, line_text)
                if cls == "live" and note.startswith("拼接首段=变量"):
                    # 运行期树（拼接首段是变量）⇒ 引文记的是「运行期在 <变量> 下」这一**事实**，
                    # 与主仓同名树无关 ⇒ ledger。原判 live 与本门 suggest() 自相矛盾
                    # （suggest 打「不改只报」，judge 却算命中）⇒ `GAP-20260926-114`。
                    cls, why = "ledger", "运行期路径（拼接首段是变量 · 运行期根 ≠ 仓根）⇒ 记运行时事实，不改"
                where = tree_of(ctx, ref)
                hits.append({"file": rel, "line": ln, "ref": shown, "path": ref, "form": form,
                             "note": note, "cls": cls, "why": why, "where": where,
                             "suggest": suggest(cls, ref, where, note),
                             "src": line_text.strip()[:160]})
            for m in RE_GAP.finditer(line_text):
                gid = m.group(0)
                gkey = (rel, ln, "GAP", gid)
                if gkey in seen:
                    continue
                seen.add(gkey)
                gap["candidates"] += 1
                if ctx.gap_ids is None:
                    gap["unjudged"].append("%s:%d · %s" % (rel, ln, gid))
                    continue
                if gid in ctx.gap_ids:
                    continue  # 账内已有这个号 ⇒ 不悬空
                cls, why = classify(ctx, rel, gid, line_text)
                if cls == "live" and is_gap_self_file(ctx, rel):
                    cls, why = "ledger", "本件（判据件自身）的自检夹具号 · 例外面"
                elif cls == "live" and is_gap_test_container(rel):
                    cls, why = "ledger", "测试夹具件（*_test.*/testdata/tests）里故意造的假号"
                rec = {"file": rel, "line": ln, "ref": gid, "path": gid, "form": "GAP号",
                       "note": "", "cls": cls, "why": why, "where": [],
                       "src": line_text.strip()[:160]}
                if cls == "live":
                    rec["suggest"] = "引的号账内不存在 ⇒ 改指在册号，或先在账立案（GAP 号引用悬空）"
                    gap["hits"].append(rec)
                else:
                    gap["exempt"].append(rec)   # ledger / 已登记 = 例外面（夹具假号·机读件·数据字串·表格行）
    stats["gap"] = gap
    return stats, hits


# ── 缺口账（GAP 号存在性真源 · 仓外）────────────────────────────────────────
def gap_ledger_path():
    """缺口真源落点：`ZERG_STATE_DIR` > `~/.zerg/state`（与 CLI `stateDirOf()` 同口径）。"""
    env = os.environ.get("ZERG_STATE_DIR")
    base = env if env else os.path.join(os.path.expanduser("~"), ".zerg", "state")
    return os.path.join(base, GAP_LEDGER_FILE)


def load_gap_ids(path):
    """读缺口账在册号集。→ (set, None) 或 **(None, 原因)**（取不到 ⇒ 本类**不给结论**）。"""
    if not path or not os.path.exists(path):
        return None, "不在盘：%s" % path
    ids = set()
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    o = json.loads(line)
                except Exception:
                    continue
                if isinstance(o, dict):
                    i = o.get("id")
                    if isinstance(i, str) and i:
                        ids.add(i)
    except Exception as e:
        return None, "读不了 %s：%s" % (path, e)
    if not ids:
        return None, "账里一个在册号都没有（空账 = 判不了）：%s" % path
    return ids, None


# ── 登记表 ───────────────────────────────────────────────────────────────────
def load_registry(path):
    """读了要**自证合格**：缺 reason / 缺 date / date 形态不对 / class 表外 ⇒ rc=2（拒给结论）。"""
    if not path:
        return {}, []
    if not os.path.exists(path):
        return {"entries": []}, []
    try:
        doc = json.load(open(path, encoding="utf-8"))
    except Exception as e:
        return None, ["登记表不是合法 JSON：%s" % e]
    if not isinstance(doc, dict) or not isinstance(doc.get("entries"), list):
        return None, ["登记表缺 `entries` 数组（schema=dangling-refs-registry/1）"]
    bad = []
    for i, e in enumerate(doc["entries"]):
        tag = "entries[%d]" % i
        if not isinstance(e, dict) or not e.get("file"):
            bad.append(tag + " 缺 `file`")
            continue
        if not e.get("ref") and not e.get("ref_prefix"):
            bad.append(tag + " 既没 `ref` 也没 `ref_prefix` —— **整件豁免 = 洗白**，不批"
                       "（机读件也请写 `ref_prefix: \"docs/\"`）")
        if not e.get("reason"):
            bad.append(tag + " 缺 `reason`（只给路径的登记 = 无记录的洗白）")
        if not RE_DATE.match(str(e.get("date", ""))):
            bad.append(tag + " 缺 `date` 或形态不是 YYYY-MM-DD")
        if e.get("class", "ledger") not in ("live", "ledger"):
            bad.append(tag + " class 不在 {live, ledger}")
    return doc, bad


# ── 自检（三格：必红 / 豁免 / 无输入；另加拼接式一格）────────────────────────
def _mkfile(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w", encoding="utf-8").write(text)
    return rel


def selftest():
    import tempfile
    ok = True
    cases = 0
    tmp = tempfile.mkdtemp(prefix="cdr-selftest-")
    repo = os.path.join(tmp, "Zerg")
    docs = os.path.join(tmp, "Zerg-内部文档")
    os.makedirs(os.path.join(docs, "01-设计"), exist_ok=True)
    open(os.path.join(docs, "01-设计", "设计-真件.md"), "w").write("真源\n")

    def run(files, registry, root=repo, d=docs, gap_ids=None):
        ctx = Ctx(root, d, None, files, registry or {"entries": []}, gap_ids=gap_ids)
        return scan(ctx)

    # ① 必红：指向真不存在的路径 ⇒ 报出
    f1 = _mkfile(repo, "scripts/x.py", "# 依据：docs/01-设计/不存在.md\n")
    st, hits = run([f1], {})
    got = [h for h in hits if h["ref"] == "docs/01-设计/不存在.md"]
    cases += 1
    ok = ok and len(got) == 1 and got[0]["cls"] == "live" and not got[0]["where"]
    print("  %s [必红夹具] 悬空 1 处 = %s · 判类 = %s · 落点 = %s（期望 live · 三树都无）"
          % ("✓" if len(got) == 1 and got[0]["cls"] == "live" else "✗",
             len(got), got[0]["cls"] if got else "—", got[0]["where"] if got else "—"))

    # ② 必红（活指路 → 真在 Zerg-内部文档）⇒ 报出 + 落点 + 建议改指
    f2 = _mkfile(repo, "scripts/y.py", "# 真源：docs/01-设计/设计-真件.md\n")
    st, hits = run([f2], {})
    got = [h for h in hits if h["path"] == "docs/01-设计/设计-真件.md"]
    cases += 1
    good = len(got) == 1 and got[0]["cls"] == "live" and got[0]["where"] and "改指" in got[0]["suggest"]
    ok = ok and good
    print("  %s [必红夹具·有落点] 命中 %s 处 · 落点 = %s · 建议 = %s（期望 live + 改指 Zerg-内部文档）"
          % ("✓" if good else "✗", len(got), got[0]["where"] if got else "—",
             got[0]["suggest"] if got else "—"))

    # ③ 豁免夹具：同一处写进登记表 ⇒ 标「已登记」且**不计命中**
    reg = {"entries": [{"file": f2, "ref": "docs/01-设计/设计-真件.md",
                        "class": "ledger", "reason": "自检夹具：有意保留", "date": "2026-09-22"}]}
    st, hits = run([f2], reg)
    got = [h for h in hits if h["path"] == "docs/01-设计/设计-真件.md"]
    cases += 1
    good = len(got) == 1 and got[0]["cls"] == "已登记" and "不计命中" in got[0]["suggest"]
    ok = ok and good
    print("  %s [豁免夹具] 标 %s · 建议 = %s（期望「已登记」+ 不计命中）"
          % ("✓" if good else "✗", got[0]["cls"] if got else "—", got[0]["suggest"] if got else "—"))

    # ④ 拼接式夹具：filepath.Join("docs", …) 必须认出来（正则扫漏的那一面）
    f4 = _mkfile(repo, "core/z.go",
                 '\tissuesDir := filepath.Join(workDir, "docs", "issues")\n'
                 '\tpath := path.Join("docs", "01-设计", "设计-真件.md")\n')
    st, hits = run([f4], {})
    j1 = [h for h in hits if h["ref"] == "docs/issues" and h["form"] == "filepath.Join"]
    j2 = [h for h in hits if h["path"] == "docs/01-设计/设计-真件.md" and h["form"] == "filepath.Join"]
    cases += 1
    # 判类成对负控（`GAP-20260926-114`）：**运行期路径**（拼接首段=变量）⇒ ledger（不算命中）；
    # **字面量**拼接 ⇒ live。两条同场 ⇒ 断言的是「判类看的是运行期性，不是拼接形态」。
    good = len(j1) == 1 and len(j2) == 1 and j1[0]["cls"] == "ledger" and j2[0]["cls"] == "live"
    ok = ok and good
    print("  %s [拼接夹具] filepath.Join 认出 %s 处(运行期⇒%s) · path.Join 认出 %s 处(字面量⇒%s)"
          "（期望 1/1 · ledger/live）"
          % ("✓" if good else "✗", len(j1), j1[0]["cls"] if j1 else "—",
             len(j2), j2[0]["cls"] if j2 else "—"))

    # ⑤ 前缀夹具：'docs/' + 变量 ⇒ 前缀目录在 ⇒ 不报（只判到前缀）
    f5 = _mkfile(repo, "publish/z.py", 'name = "docs/" + base\n')
    os.makedirs(os.path.join(repo, "docs"), exist_ok=True)
    st, hits = run([f5], {})
    cases += 1
    good = not [h for h in hits if h["form"] == "字符串拼接"]
    ok = ok and good
    print("  %s [前缀夹具] 前缀目录在 ⇒ 不报 = %s（期望 True）" % ("✓" if good else "✗", good))

    # ⑥ 无输入必须报错（不许静默空转 = 假绿）
    empty = os.path.join(tmp, "empty")
    os.makedirs(empty, exist_ok=True)
    rc = real_exit_code(empty, None, force_files=[])
    cases += 1
    ok = ok and rc == 2
    print("  %s [无输入夹具] 空树 rc = %s（期望 2 —— 拒绝静默空转）" % ("✓" if rc == 2 else "✗", rc))

    # ⑦ 登记表缺 reason/date ⇒ rc=2（登记表自身要自证）
    badreg = os.path.join(tmp, "bad-registry.json")
    open(badreg, "w", encoding="utf-8").write('{"entries":[{"file":"a","ref":"docs/x"}]}')
    doc, bad = load_registry(badreg)
    cases += 1
    good = doc is None or len(bad) >= 2
    ok = ok and good
    print("  %s [登记表夹具] 缺 reason+date 报 %s 条（期望 ≥2 ⇒ rc=2·拒给结论）" % ("✓" if good else "✗", len(bad)))

    # ⑧ 整件豁免（既无 ref 也无 ref_prefix）⇒ rc=2（不许洗白）
    blankreg = os.path.join(tmp, "blanket-registry.json")
    open(blankreg, "w", encoding="utf-8").write(
        '{"entries":[{"file":"scripts/x.py","class":"ledger","reason":"整件免了","date":"2026-09-22"}]}')
    doc, bad = load_registry(blankreg)
    cases += 1
    good = len(bad) == 1 and "洗白" in bad[0]
    ok = ok and good
    print("  %s [整件豁免夹具] 报 %s 条（期望 1 条 · 点名「整件豁免 = 洗白」）"
          % ("✓" if good else "✗", len(bad)))

    # ⑨ 前缀登记必须**真能**命中，且**不**越界（前缀外的引文照样算命中）
    f9 = _mkfile(repo, "scripts/w.py", 'x = "docs/项目文档/v2.5.9/x.md"\ny = "docs/01-设计/a.md"\n')
    pre = {"entries": [{"file": f9, "ref_prefix": "docs/项目文档/", "class": "ledger",
                        "reason": "前缀登记自检", "date": "2026-09-22"}]}
    st, hits = run([f9], pre)
    inside = [h for h in hits if h["path"].startswith("docs/项目文档/")]
    outside = [h for h in hits if h["path"].startswith("docs/01-设计/")]
    cases += 1
    good = len(inside) == 1 and inside[0]["cls"] == "已登记" and len(outside) == 1 \
        and outside[0]["cls"] != "已登记"
    ok = ok and good
    print("  %s [前缀登记夹具] 前缀内=已登记 %s · 前缀外仍命中 %s（期望 1/1 —— 前缀不许越界洗白）"
          % ("✓" if good else "✗", inside[0]["cls"] if inside else "—",
             outside[0]["cls"] if outside else "—"))

    # ⑩ 缺语料夹具：同级**语料树**取不到根 ⇒ 报缺（真跑据此 `return 2` · `GAP-20260927-82` 批2）；
    #    成对负控 = 语料树在盘 ⇒ 报 0 条（**在盘时判据一字未改**）；归档树缺席**不翻退码**（本机是断链）。
    miss_bad = absent_corpus_trees(os.path.join(tmp, "no-such-Zerg-内部文档"), None)
    miss_arch = absent_corpus_trees(docs, os.path.join(tmp, "no-such-Zerg-归档"))
    cases += 1
    good = len(miss_bad) == 1 and miss_arch == []
    ok = ok and good
    print("  %s [缺语料夹具] 语料树缺席报 %s 条 · 语料树在盘(归档缺席)报 %s 条（期望 1 / 0 ⇒ 缺根不给结论·在盘照旧）"
          % ("✓" if good else "✗", len(miss_bad), len(miss_arch)))

    # ⑪ 跨仓夹具（`GAP-20261005-18`）：`Zerg-内部文档/…` / 裸 `项目文档/…` 指向**文仓里不存在**的件 ⇒ 报出；
    #    成对负控：同一处指向**文仓里存在**的件 ⇒ 不报（跨仓面只在「文仓根下不存在」时命中）。
    f11 = _mkfile(repo, "scripts/gates/xref.py",
                  "# 真源：`Zerg-内部文档/01-设计/不存在的旧文.md` 与 `项目文档/不存在的账.md` 与 "
                  "`Zerg-内部文档/01-设计/设计-真件.md`\n")
    st, hits = run([f11], {}, root=repo, d=docs)
    miss = [h for h in hits if h["ref"] in ("Zerg-内部文档/01-设计/不存在的旧文.md",
                                            "项目文档/不存在的账.md")]
    present = [h for h in hits if h["path"] == "Zerg-内部文档/01-设计/设计-真件.md"]
    cases += 1
    good = len(miss) == 2 and len(present) == 0
    ok = ok and good
    print("  %s [跨仓夹具] 文仓里不存在 ⇒ 报 %s 处 · 文仓里存在 ⇒ 报 %s 处（期望 2 / 0）"
          % ("✓" if good else "✗", len(miss), len(present)))

    # ⑫ GAP 号引用·悬空（档 · 成对负控之一）：注释里引的号账内不存在 ⇒ 命中（点名 件:行 · 引的号）
    led = {"GAP-20261005-54", "GAP-20261005-60"}
    fg1 = _mkfile(repo, "scripts/gates/g1.py", "# 见缺口 `GAP-20260928-999`（账内无）\n")
    st, _ = run([fg1], {}, gap_ids=led)
    gh = st["gap"]["hits"]
    cases += 1
    good = len(gh) == 1 and gh[0]["ref"] == "GAP-20260928-999" \
        and gh[0]["file"] == fg1 and gh[0]["line"] == 1
    ok = ok and good
    print("  %s [GAP悬空夹具] 命中 %s 处 · 号 = %s（期望 1 · GAP-20260928-999）"
          % ("✓" if good else "✗", len(gh), gh[0]["ref"] if gh else "—"))

    # ⑬ GAP 号引用·全在账 ⇒ 不报（成对负控之二：账内有的号不该报）
    fg2 = _mkfile(repo, "scripts/gates/g2.py", "# 见缺口 `GAP-20261005-54`（账内有）\n")
    st, _ = run([fg2], {}, gap_ids=led)
    cases += 1
    good = st["gap"]["hits"] == []
    ok = ok and good
    print("  %s [GAP全在账夹具] 命中 %s 处（期望 0 —— 账内在册号不报）"
          % ("✓" if good else "✗", len(st["gap"]["hits"])))

    # ⑭ GAP 号引用·负控（哑化）：同号写进**数据字串**（引号紧包）⇒ 例外面，不计命中也不哑报
    fg3 = _mkfile(repo, "scripts/gates/g3.py", 'fx = {"id": "GAP-20260928-777"}\n')
    st, _ = run([fg3], {}, gap_ids=led)
    g3 = st["gap"]
    cases += 1
    good = len(g3["hits"]) == 0 and len(g3["exempt"]) == 1
    ok = ok and good
    print("  %s [GAP负控夹具] 命中 %s · 例外面 %s（期望 0 / 1 —— 数据字串是例外面，不哑报）"
          % ("✓" if good else "✗", len(g3["hits"]), len(g3["exempt"])))

    # ⑮ GAP 号引用·账取不到 ⇒ 未判（不翻退码 · 不静默成绿）
    fg4 = _mkfile(repo, "scripts/gates/g4.py", "# 见缺口 `GAP-20260928-888`\n")
    st, _ = run([fg4], {}, gap_ids=None)
    cases += 1
    good = st["gap"]["hits"] == [] and st["gap"]["candidates"] >= 1 \
        and len(st["gap"]["unjudged"]) == 1 and st["gap"]["judged"] is False
    ok = ok and good
    print("  %s [GAP账缺夹具] 命中 %s · 未判 %s · judged=%s（期望 0 / ≥1 / False）"
          % ("✓" if good else "✗", len(st["gap"]["hits"]),
             len(st["gap"]["unjudged"]), st["gap"]["judged"]))

    assert cases == 15, "自检用例数必须为 15，实际 %d（少跑即失效）" % cases
    print("  自检：%s（15 格：必红×2 · 豁免×1 · 拼接×1 · 前缀×1 · 无输入×1 · 登记表×1 · 整件豁免×1 ·"
          " 前缀越界×1 · 缺语料×1 · 跨仓×1 · GAP悬空×1 · GAP全在账×1 · GAP负控×1 · GAP账缺×1）"
          % ("通过 ✓" if ok else "**失败** ✗"))
    return ok


def real_exit_code(root, registry_path, force_files=None, gap_ids=None):
    """自检用的「跑真判据」入口：只取退码，不打长清单。"""
    if force_files is None:
        files = git_files(root)
    else:
        files = force_files
    if not files:
        print("✗ 扫到 0 个件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2
    reg, bad = load_registry(registry_path)
    if bad:
        return 2
    ctx = Ctx(root, os.path.join(os.path.dirname(root), DOCS_TREE_NAME),
              os.path.join(os.path.dirname(root), ARCH_TREE_NAME), files, reg or {},
              gap_ids=gap_ids)
    st, hits = scan(ctx)
    if st["files_scanned"] == 0:
        print("✗ 扫到 0 个件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2
    gap_hits = (st.get("gap") or {}).get("hits", [])
    return 0 if (not hits and not gap_hits) else 1


# ── 真跑 ─────────────────────────────────────────────────────────────────────
def git_files(root):
    try:
        out = subprocess.run(["git", "ls-files", "-z"], cwd=root, capture_output=True)
    except Exception as e:
        print("✗ 跑不了 git ls-files：%s ⇒ 不给结论" % e, file=sys.stderr)
        return None
    if out.returncode != 0:
        print("✗ git ls-files rc=%d（不是 git 仓？）⇒ 不给结论" % out.returncode, file=sys.stderr)
        return None
    return [p for p in out.stdout.decode("utf-8", "replace").split("\0") if p]


def print_rules():
    print(VERSION + " —— 判据 / 两类定义 / 覆盖与未覆盖")
    print("\n【判据】主仓（`git ls-files` 的件）里形如 `docs/…` 的引用，解析后（相对**仓根**）不存在 ⇒ 悬空。")
    print("\n【两类】live = 活指路（该改）· ledger = 机读清单/基线台账（记当时路径，不该改 ⇒ 登记）")
    print("  登记表：" + REGISTRY_DEFAULT)
    print("  三种登记形态（**整件豁免 = 洗白 ⇒ rc=2 拒**）：")
    print("    · `ref`        逐处：这一件里的**这一条**引文")
    print("    · `ref_prefix` 前缀：这一件里**以该前缀开头**的全部引文（前缀外照旧命中 —— 不许越界）")
    print("    · 每条必须带 `reason` + `date`（YYYY-MM-DD），缺任一 ⇒ rc=2（与 doc-freshness.config.json 同口径）")
    print("  写进表 = 有意保留：输出里标 `已登记` 且**不计命中**，但**照样逐条打印**（豁免 ≠ 隐藏）")
    print("\n【分类优先级（自动规则 · 每处都打印理由）】")
    print("  ① 登记表命中           ⇒ 已登记（不计命中）")
    print("  ② 夹具/探针假路径      ⇒ ledger（例：docs/a.md；行含 夹具/探针/自检/负控）")
    print("  ③ 机读件后缀 %s ⇒ ledger" % ", ".join(MACHINE_EXT))
    print("  ④ 引文被引号紧包 ⇒ ledger（代码/数据字串：判据常量 · 配置值 · 夹具键值）")
    print("  ⑤ 行首是 Markdown 表格行 ⇒ ledger（表格式清单/台账）")
    print("  ⑥ 行内有记录措辞（以下任一）⇒ ledger：")
    print("     " + " · ".join(RECORD_WORDS))
    print("  ⑦ 行内有指路措辞（以下任一）⇒ live：")
    print("     " + " · ".join(LIVE_WORDS))
    print("  ⑧ 否则 ⇒ live（默认：正文/注释里的断指针，宁可多报 —— 本门只报告）")
    print("\n【已覆盖形态】直书 `docs/…`（含反引号/注释/字符串/frontmatter）·")
    print("              `filepath.Join(\"docs\", …)` / `path.Join(…)` 拼接式 · .py 里 `\"docs/\" + x` 前缀形态 ·")
    print("              **跨仓直书** `Zerg-内部文档/<rel>` 与裸 `项目文档/<rel>`（2026-10-05 · GAP-20261005-18）")
    print("【未覆盖形态（明说 · 不假装全覆盖）】")
    print("  · 纯 URL（`https://…/docs/…`）—— 不是仓内相对引用，本门不判")
    print("  · 变量拼进中段/后缀：`\"docs/\" + lang + \"/x.md\"` —— 只判到前缀目录那一层")
    print("  · 运行期树：`filepath.Join(workDir, \"docs\", \"issues\")` 真身是 workDir 下的目录 ——")
    print("    报出来但按 ledger 判（要有意保留就登记），**不许按主仓同名树改**")
    print("  · 软链 / 大小写变体 / 非 UTF-8 件（记入 files_unreadable）· 未 `git add` 的新件")
    print("  · markdown 相对链接 `[](../x.md)` —— 属 `check-doc-freshness.py` D1，本门不重复")
    print("\n【GAP 号引用类（2026-10-05 · GAP-20261005-54）】")
    print("  判据：注释/文档里引的 `GAP-<8位日期>-<号>` 必须在**缺口账**里存在")
    print("        （真源 <ZERG_STATE_DIR|~/.zerg/state>/" + GAP_LEDGER_FILE + "；可 `--ledger` 覆盖）")
    print("  不存在 ⇒ 悬空（逐字点名 件:行 · 引的号）· 账取不到 ⇒ 本类未判（不给结论 · 但**不翻**其余面的退码）")
    print("  例外面（照样逐条打印 · 不计命中）：本件自身(自检夹具号) / 测试夹具件(*_test.* / testdata/ / tests/ 里的假号) /")
    print("                                    机读件(.json/.tsv/.csv/.txt) / 引号紧包的数据字串 / Markdown 表格行 /")
    print("                                    登记表命中（`ref`=号 或 `ref_prefix`）")
    print("  判定复用本门**同一个** `classify()`：只有 `live` 类算命中，`ledger`/`已登记` = 例外面")
    print("\n【退码】0 无命中（或全登记/全例外面）· 1 有未登记悬空（docs 路径类 ＋ GAP 号引用类）·")
    print("        2 不给结论（不在仓根/0 件/登记表不合格/同级语料树 Zerg-内部文档 取不到根/自检不过）")


def main(argv):
    root = os.path.abspath(".")
    registry_path = os.path.join(root, REGISTRY_DEFAULT)
    ledger_path = gap_ledger_path()
    show_all = False
    live_only = False
    json_out = False
    dump_path = ""
    group_by = []
    prefix_cands = False
    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("--selftest", "--self-test"):
            return 0 if selftest() else 1
        if a == "--list-rules":
            print_rules()
            return 0
        if a == "--all":
            show_all = True
        elif a == "--live-only":
            live_only = True
        elif a == "--json":
            json_out = True
        elif a == "--group-by":
            i += 1
            group_by = [d for d in argv[i].split(",") if d]
        elif a == "--prefix-candidates":
            prefix_cands = True
        elif a == "--dump":
            i += 1
            dump_path = os.path.abspath(argv[i])
        elif a == "--root":
            i += 1
            root = os.path.abspath(argv[i])
            registry_path = os.path.join(root, REGISTRY_DEFAULT)
        elif a == "--registry":
            i += 1
            registry_path = os.path.abspath(argv[i])
        elif a == "--ledger":
            i += 1
            ledger_path = os.path.abspath(argv[i])
        else:
            print("✗ 未知参数 %r（用法见 --list-rules）" % a, file=sys.stderr)
            return 2
        i += 1

    for d in group_by:
        if d not in GROUP_DIMS:
            print("✗ --group-by 维度不认识：%r（闭集：src|tgt|form|cls）" % d, file=sys.stderr)
            return 2

    if not os.path.isdir(os.path.join(root, "docs")) and not os.path.isdir(os.path.join(root, "core")):
        print("✗ 不在仓根（`docs/` 与 `core/` 都不存在）：%s ⇒ 拒绝静默空转" % root, file=sys.stderr)
        return 2

    files = git_files(root)
    if not files:
        print("✗ 扫到 0 个件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2

    reg, bad = load_registry(registry_path)
    if bad:
        print("✗ 登记表不合格 ⇒ **不给结论**（写不清理由的豁免就是洗白）：", file=sys.stderr)
        for b in bad:
            print("   · " + b, file=sys.stderr)
        return 2

    gap_ids, gap_problem = load_gap_ids(ledger_path)

    parent = os.path.dirname(root)
    # 取根规则与同族件归一（批1）：ZERG_DOCS_ROOT > 同级根 ZERG_DOCS_ALT/ZERG_DOCS > <仓父>/Zerg-内部文档。
    sibling = (os.environ.get("ZERG_DOCS_ALT") or os.environ.get("ZERG_DOCS")
               or os.path.join(parent, DOCS_TREE_NAME))
    docs_root = os.environ.get("ZERG_DOCS_ROOT") or sibling
    arch_root = os.environ.get("ZERG_ARCHIVE_ROOT") or os.path.join(parent, ARCH_TREE_NAME)
    absent = absent_corpus_trees(docs_root, arch_root)
    ctx = Ctx(root, docs_root, arch_root, files, reg or {}, gap_ids=gap_ids)
    stats, hits = scan(ctx)
    if stats["files_scanned"] == 0:
        print("✗ 扫到 0 个件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2

    by = {"live": [], "ledger": [], "已登记": []}
    for h in hits:
        by[h["cls"]].append(h)
    n_live, n_led, n_reg = len(by["live"]), len(by["ledger"]), len(by["已登记"])
    files_hit = len({h["file"] for h in hits})

    print("%s —— 主仓悬空引用（`docs/…` 指向不存在）" % VERSION)
    print("仓根：%s" % root)
    for name, tree in ((DOCS_TREE_NAME, docs_root), (ARCH_TREE_NAME, arch_root)):
        if os.path.isdir(tree):
            state = "在"
        elif name == DOCS_TREE_NAME:
            state = "**不在盘** —— 落点判不了 ⇒ 本门不给结论（rc=2）"
        else:
            state = "**不在** —— 落点只报主仓"      # 归档树缺席不翻退码（与现役档逐字一致 · 见件头 ★）
        print("  %s：%s（%s）" % (name, tree, state))
    print("扫描面：git ls-files %d 件 ⇒ 文本件 %d · 提取引用 %d 处（另滤掉散文误吞 %d 处）· 悬空 %d 处"
          % (stats["files_seen"], stats["files_scanned"], stats["refs"],
             stats.get("refs_prose_skipped", 0), stats["dangling"]))
    print("  缺口账（GAP 号存在性真源）：%s（%s）" % (
        ledger_path,
        "在" if gap_ids is not None else "**取不到** ⇒ GAP 号引用类未判：" + str(gap_problem)))
    if stats.get("files_unreadable"):
        print("  ⚠ 非 UTF-8 / 读不了 %d 件（未覆盖形态之一）：%s"
              % (len(stats["files_unreadable"]), " · ".join(stats["files_unreadable"][:3])))
    print("\n合计（docs/ 路径类）：悬空 %d 处 / %d 件 ⇒ **命中 %d 处**（live %d · ledger %d）· 已登记 %d 处（不计命中）"
          % (stats["dangling"], files_hit, n_live + n_led, n_live, n_led, n_reg))
    gap = stats.get("gap") or {"hits": [], "exempt": [], "candidates": 0,
                               "judged": gap_ids is not None, "unjudged": []}
    g_hits, g_exempt = gap["hits"], gap["exempt"]
    n_gap = len(g_hits)
    if gap["judged"]:
        print("合计（GAP 号引用类）：候选 %d 处 ⇒ **命中 %d 处**（live）· 例外面 %d 处（不计命中）"
              % (gap["candidates"], len(g_hits), len(g_exempt)))
    else:
        print("合计（GAP 号引用类）：候选 %d 处 ⇒ **未判**（缺口账取不到 ⇒ 本类不给结论）"
              % gap["candidates"])

    cap = 10 ** 9 if show_all else 30

    def dump(title, rows):
        print("\n── %s：%d 处" % (title, len(rows)))
        for h in rows[:cap]:
            print("   %s:%d · 引文 `%s` · 解析 `%s` · 落点 %s · 【%s】%s · 建议：%s"
                  % (h["file"], h["line"], h["ref"], h["path"],
                     ("、".join(h["where"]) or "三树都无"), h["cls"], h["why"], h["suggest"]))
            if h["form"] != "直书":
                print("        （形态=%s%s）" % (h["form"], ("·" + h["note"]) if h["note"] else ""))
        if len(rows) > cap:
            print("   …另 %d 处（--all 全打）" % (len(rows) - cap))

    if group_by:
        # `--group-by`：逐处清单换成**按维度聚合**（聚合面 = docs 路径类 ＋ GAP 号类全清单）。
        allh = list(hits) + list(g_hits) + list(g_exempt)
        print("\n── 聚合（--group-by=%s）：共 %d 处（docs 路径类 %d + GAP 号类 %d）"
              % (",".join(group_by), len(allh), len(hits), len(g_hits) + len(g_exempt)))
        for dim in group_by:
            cnt = {}
            for h in allh:
                v = _dim_value(h, dim)
                cnt[v] = cnt.get(v, 0) + 1
            print("  【%s】" % dim)
            for v, n in sorted(cnt.items(), key=lambda kv: (-kv[1], kv[0])):
                print("     %s ⇒ %d 处" % (v, n))
    if n_live and not group_by:
        dump("live 命中（活指路 · 该改）", by["live"])
    if n_led and not live_only and not group_by:
        dump("ledger 命中（机读清单/基线台账 · 记当时路径 ⇒ 登记，不改）", by["ledger"])
    if n_reg and not live_only and not group_by:
        dump("已登记（有意保留 · 不计命中）", by["已登记"])

    if g_hits and not group_by:
        print("\n── GAP 号引用命中（注释/文档引的号账内不存在 · 该改或先立案）：%d 处" % len(g_hits))
        for h in g_hits[:cap]:
            print("   %s:%d · 引的号 `%s` · 【%s】%s · 建议：%s"
                  % (h["file"], h["line"], h["ref"], h["cls"], h["why"], h["suggest"]))
        if len(g_hits) > cap:
            print("   …另 %d 处（--all 全打）" % (len(g_hits) - cap))
    if g_exempt and not live_only and not group_by:
        print("\n── GAP 号引用例外面（夹具假号 / 机读件 / 快照 / 数据字串 · 不计命中）：%d 处" % len(g_exempt))
        for h in g_exempt[:cap]:
            print("   %s:%d · 引的号 `%s` · 【%s】%s" % (h["file"], h["line"], h["ref"], h["cls"], h["why"]))
        if len(g_exempt) > cap:
            print("   …另 %d 处（--all 全打）" % (len(g_exempt) - cap))
    if gap["unjudged"]:
        print("\n⛔ GAP 号引用类**未判**（缺口账取不到：%s）—— 候选 %d 处（前几处：%s）"
              % (gap_problem, len(gap["unjudged"]), " · ".join(gap["unjudged"][:3])))

    if prefix_cands:
        cands = prefix_candidates(hits)
        print("\n── 可登记前缀候选（未登记的 ledger 命中 · `--prefix-candidates`）：%d 条"
              "（草稿 ⇒ `ref_prefix` 可直接粘进退 `entries`，`reason`/`date` 需人工补全）" % len(cands))
        print(json.dumps(cands, ensure_ascii=False, indent=1))

    print("\n未覆盖形态（明说，不假装全覆盖）：纯 URL · 变量拼进中段或后缀 · 运行期路径树 · "
          "软链/大小写变体/非 UTF-8 件/未 git add 的新件 · markdown 相对链接（属 D1）")

    if json_out:
        print(json.dumps({"schema": "dangling-refs/1", "repo": root,
                          "dangling": stats["dangling"], "files": files_hit,
                          "live": n_live, "ledger": n_led, "registered": n_reg,
                          "hit": n_live + n_led, "registry": registry_path,
                          "gap_judged": gap["judged"], "gap_candidates": gap["candidates"],
                          "gap_hit": len(g_hits), "gap_exempt": len(g_exempt),
                          "gap_ledger": ledger_path},
                         ensure_ascii=False))
    if dump_path:
        try:
            json.dump({"schema": "dangling-refs/1", "repo": root, "stats": stats,
                       "hits": hits}, open(dump_path, "w", encoding="utf-8"),
                      ensure_ascii=False, indent=1)
            print("（全量命中已落盘：%s）" % dump_path)
        except Exception as e:
            print("✗ 落不了盘 %s：%s ⇒ 不给结论" % (dump_path, e), file=sys.stderr)
            return 2

    if absent:
        # 缺语料 ⇒ **不给结论**（rc=2）：既不当 0（无问题）也不当 1（有问题）—— `GAP-20260927-82` 批2。
        for name, tree in absent:
            print("\n⛔ 不给结论（rc=2）：同级语料树 `%s` 取不到根（%s）—— 落点判不了"
                  "\n   换根候选：`ZERG_DOCS_ROOT`，或同级根 `ZERG_DOCS_ALT`/`ZERG_DOCS`"
                  % (name, tree))
        return 2
    n_path = n_live + n_led
    if n_path == 0 and n_gap == 0:
        print("\n结论：无命中（或全在登记表 / 例外面里）⇒ rc=0")
        return 0
    if n_path:
        print("\n结论：有 %d 处**未登记**的悬空引用（docs 路径类）⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）"
              % n_path)
    if n_gap:
        print("\n结论：有 %d 处**未登记**的悬空引用（GAP 号引用类）⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）"
              % n_gap)
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

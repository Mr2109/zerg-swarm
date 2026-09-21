#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-dangling-refs.py v1.0.0 —— 主仓悬空引用（`docs/…` 指向不存在）

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
     `https://…/docs/x`、`Zerg-内部文档/docs/x` 这类「前面还粘着东西」的**不算**（那不是主仓相对引用）
  ② `filepath.Join("docs", …, "件")` / `path.Join(…)` —— 把紧邻的字符串字面量按序拼成一条路径；
     首段是变量（如 `filepath.Join(workDir, "docs", "issues")`）时**按仓内相对判**并在输出里标出
  ③ 字符串拼接：`.py` 里的 `"docs/" + base` 这类**前缀形态** —— 只判到前缀目录（后缀是变量，判不了）

**未覆盖形态**（不假装全覆盖 —— 逐条钉在 `--list-rules` 与运行输出末行）
----------------------------------------------------------------------
  · URL / 跨仓引用（`https://…/docs/…`、`其他仓/docs/…`）—— 不是主仓相对引用，本门不判
  · 变量拼进中段：`"docs/" + lang + "/x.md"`（后缀/中段是变量）—— 只判前缀目录那一层
  · 运行期路径：`filepath.Join(workDir, "docs", "issues")` 真身是**运行期在 workDir 下**建的目录，
    与主仓同名不同树 ⇒ 报出来但按 ledger 判（要有意保留就登记）
  · 软链 / 大小写变体 / 非 UTF-8 件 / 未 `git add` 的新件（本门只扫 `git ls-files`）
  · markdown 相对链接 `[](../x.md)` —— 那是**另一条判据**（`check-doc-freshness.py` D1），本门不重复

退码（三档 · 与仓内既有门一致）
--------------------------------
  0 = 无命中（或命中全在登记表里）
  1 = 有**未登记**的悬空引用（live + ledger 两类合计）
  2 = **不给结论**（不在仓根 / 扫到 0 件 / 登记表缺 reason·date·形态不对 / 自检不过）

用法
----
    python3 scripts/gates/check-dangling-refs.py                 # 全仓扫（只报告）
    python3 scripts/gates/check-dangling-refs.py --all           # 打印全部命中（默认每类截 30 行）
    python3 scripts/gates/check-dangling-refs.py --live-only     # 只打 live 类
    python3 scripts/gates/check-dangling-refs.py --json          # 末尾追加机器面一行
    python3 scripts/gates/check-dangling-refs.py --root DIR --registry F   # 对夹具跑（自检内部用）
    python3 scripts/gates/check-dangling-refs.py --list-rules    # 打判据 / 两类定义 / 覆盖与未覆盖
    python3 scripts/gates/check-dangling-refs.py --self-test     # 只跑自检（合成夹具，不碰真目标）
"""
import json
import os
import re
import subprocess
import sys

VERSION = "check-dangling-refs.py v1.0.0"

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

    def __init__(self, root, docs_root, arch_root, files, registry):
        self.root = root
        self.docs_root = docs_root
        self.arch_root = arch_root
        self.files = files          # 仓内相对路径列表
        self.registry = registry     # {"entries": [...]} 或 {}
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
                if os.path.exists(os.path.join(ctx.root, ref.rstrip("/"))):
                    continue  # 在仓内 ⇒ 不悬空
                stats["dangling"] += 1
                cls, why = classify(ctx, rel, ref, line_text)
                where = tree_of(ctx, ref)
                hits.append({"file": rel, "line": ln, "ref": shown, "path": ref, "form": form,
                             "note": note, "cls": cls, "why": why, "where": where,
                             "suggest": suggest(cls, ref, where, note),
                             "src": line_text.strip()[:160]})
    return stats, hits


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

    def run(files, registry, root=repo, d=docs):
        ctx = Ctx(root, d, None, files, registry or {"entries": []})
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
    good = len(j1) == 1 and len(j2) == 1
    ok = ok and good
    print("  %s [拼接夹具] filepath.Join 认出 %s 处 · path.Join 认出 %s 处（期望各 1）"
          % ("✓" if good else "✗", len(j1), len(j2)))

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

    assert cases == 9, "自检用例数必须为 9，实际 %d（少跑即失效）" % cases
    print("  自检：%s（9 格：必红×2 · 豁免×1 · 拼接×1 · 前缀×1 · 无输入×1 · 登记表×1 · 整件豁免×1 ·"
          " 前缀越界×1）" % ("通过 ✓" if ok else "**失败** ✗"))
    return ok


def real_exit_code(root, registry_path, force_files=None):
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
              os.path.join(os.path.dirname(root), ARCH_TREE_NAME), files, reg or {})
    st, hits = scan(ctx)
    if st["files_scanned"] == 0:
        print("✗ 扫到 0 个件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2
    return 0 if not hits else 1


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
    print("              `filepath.Join(\"docs\", …)` / `path.Join(…)` 拼接式 · .py 里 `\"docs/\" + x` 前缀形态")
    print("【未覆盖形态（明说 · 不假装全覆盖）】")
    print("  · URL / 跨仓引用（`https://…/docs/…`、`别仓/docs/…`）—— 首字符前还粘着东西，本门不判")
    print("  · 变量拼进中段/后缀：`\"docs/\" + lang + \"/x.md\"` —— 只判到前缀目录那一层")
    print("  · 运行期树：`filepath.Join(workDir, \"docs\", \"issues\")` 真身是 workDir 下的目录 ——")
    print("    报出来但按 ledger 判（要有意保留就登记），**不许按主仓同名树改**")
    print("  · 软链 / 大小写变体 / 非 UTF-8 件（记入 files_unreadable）· 未 `git add` 的新件")
    print("  · markdown 相对链接 `[](../x.md)` —— 属 `check-doc-freshness.py` D1，本门不重复")
    print("\n【退码】0 无命中（或全登记）· 1 有未登记悬空 · 2 不给结论（不在仓根/0 件/登记表不合格/自检不过）")


def main(argv):
    root = os.path.abspath(".")
    registry_path = os.path.join(root, REGISTRY_DEFAULT)
    show_all = False
    live_only = False
    json_out = False
    dump_path = ""
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
        else:
            print("✗ 未知参数 %r（用法见 --list-rules）" % a, file=sys.stderr)
            return 2
        i += 1

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

    parent = os.path.dirname(root)
    docs_root = os.environ.get("ZERG_DOCS_ROOT") or os.path.join(parent, DOCS_TREE_NAME)
    arch_root = os.environ.get("ZERG_ARCHIVE_ROOT") or os.path.join(parent, ARCH_TREE_NAME)
    ctx = Ctx(root, docs_root, arch_root, files, reg or {})
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
        print("  %s：%s（%s）" % (name, tree, "在" if os.path.isdir(tree) else "**不在** —— 落点只报主仓"))
    print("扫描面：git ls-files %d 件 ⇒ 文本件 %d · 提取引用 %d 处（另滤掉散文误吞 %d 处）· 悬空 %d 处"
          % (stats["files_seen"], stats["files_scanned"], stats["refs"],
             stats.get("refs_prose_skipped", 0), stats["dangling"]))
    if stats.get("files_unreadable"):
        print("  ⚠ 非 UTF-8 / 读不了 %d 件（未覆盖形态之一）：%s"
              % (len(stats["files_unreadable"]), " · ".join(stats["files_unreadable"][:3])))
    print("\n合计：悬空 %d 处 / %d 件 ⇒ **命中 %d 处**（live %d · ledger %d）· 已登记 %d 处（不计命中）"
          % (stats["dangling"], files_hit, n_live + n_led, n_live, n_led, n_reg))

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

    if n_live:
        dump("live 命中（活指路 · 该改）", by["live"])
    if n_led and not live_only:
        dump("ledger 命中（机读清单/基线台账 · 记当时路径 ⇒ 登记，不改）", by["ledger"])
    if n_reg and not live_only:
        dump("已登记（有意保留 · 不计命中）", by["已登记"])

    print("\n未覆盖形态（明说，不假装全覆盖）：URL/跨仓引用 · 变量拼进中段或后缀 · 运行期路径树 · "
          "软链/大小写变体/非 UTF-8 件/未 git add 的新件 · markdown 相对链接（属 D1）")

    if json_out:
        print(json.dumps({"schema": "dangling-refs/1", "repo": root,
                          "dangling": stats["dangling"], "files": files_hit,
                          "live": n_live, "ledger": n_led, "registered": n_reg,
                          "hit": n_live + n_led, "registry": registry_path},
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

    if n_live + n_led == 0:
        print("\n结论：无命中（或全在登记表里）⇒ rc=0")
        return 0
    print("\n结论：有 %d 处**未登记**的悬空引用 ⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）"
          % (n_live + n_led))
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

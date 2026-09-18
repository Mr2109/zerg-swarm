#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-name.py —— 命名规范化门（把《清单-命名规范化》已落地的规则写成可执行判据）

规格与真源（只读）
  `docs/01-设计/清单-命名规范化-20260918.md`（182 行 · 17,025 B）
  `docs/01-设计/清单-命名规范化-20260918.tsv`（147 行 = 表头 1 + 条目 146；列 =
     当前路径 · 建议路径 · 不规范类型 · 引用它的文件数 · 风险档 · 备注）
  `docs/01-设计/设计-文档体系-v1.0.md`（v1.2 稿）§16.1（K8 扩项 = 命名规范化专项）

**本脚本只报告，不改任何文件**（`--self-test` 之外不写盘；自检只写 /tmp 夹具）。

七条判据（规则号与《清单》§1 的七类一一对应；档位来自《清单》§2 风险档规则与 §5）
  N1 [FAIL] ①大小写     文件词干全为大写拉丁字母（长度 ≥ 3）—— 「与 mkdocs `index.md` 约定冲突」
  N2 [FAIL] ②空格/全角   名称含 U+0020 / U+3000 / 全角标点（，。、；：？！（）【】「」“”）
  N3 [FAIL] ③日期后缀   词干含 `-YYYYMMDD`（8 位）或 `-YYYYMM`（6 位，《清单》§5 ⑦「位数不全」）
                          —— 建议形态：**日期移入 frontmatter `updated_at`**，文件名去日期
  N4 [FAIL] ③版本段      `-vX.Y` 段存在但其后还有分隔段 —— 建议形态：版本段**移到尾部**
                          （本仓约定 `名-…-vX.Y[-YYYYMMDD]`）
  N5 [WARN] ④前缀序号    同层目录内 `NN-` 前缀与无前缀并存（各 ≥ 2）—— **结构级 · 只登记不改**
                          （《清单》§5：牵动 nav / 白名单）
  N6 [FAIL] ⑥同基名      同目录同基名（去**最后一个**扩展名后相同）、扩展名不同，
                          且至少一方的扩展名在文档/脚本面内（.md/.mdx/.py/.sh/.tsv/.json/.yaml…）
  N7 [WARN] ⑤中英紧邻    同一分隔段内汉字与拉丁字母**直接相邻**（无 `-`）—— 《清单》§5：
                          「⚠ 机械插 `-` **需人审**（专有名词 / 产品名情形）」⇒ 告警不阻断

三条硬口径（都是被咬过的）
  ① **目录名与文件名的档位不同**：改**目录名**是结构级动作（牵动 nav / 白名单 / 引用），
     《清单》§5 ④ 明写「只登记不改」⇒ 目录名命中一律 **WARN**；文件名命中才算 FAIL。
  ② **冻结区不计入退码**：`docs/项目文档/`（17 套快照 §8.1）与 `docs/issues/`（引擎数据 §1.3）
     按《清单》§2「高（冻结·不改）」**只登记不改** ⇒ 命中数照报，但不进 rc（`--count-frozen` 可改）。
  ③ **例外必须可数**：例外分三档、逐条给理由与出处，`--list-exempt` 全列出并**自证条数**
     （A 档 = 《清单》已登记的 **22 条**；B 档 = 规则级规范豁免；C 档 = 本轮新增登记）。
     豁免不是隐藏 —— 报告里每一档都有命中计数。

用法
    python3 scripts/check-doc-name.py                    # 全仓扫描域（同《清单》§0 口径）
    python3 scripts/check-doc-name.py --target docs      # 只看 docs/
    python3 scripts/check-doc-name.py --json
    python3 scripts/check-doc-name.py --list-rules
    python3 scripts/check-doc-name.py --list-exempt
    python3 scripts/check-doc-name.py --self-test         # 成对负控（好件必绿 / 坏件必红 / 缺件必 rc=2）
    python3 scripts/check-doc-name.py --no-self-test ...  # 内部子进程用（防递归）

退出码（三档）
    0 = 全绿（告警不阻断）· 1 = 有不合规项（只报告不改）·
    2 = 不给结论（用法错 / 目标缺件 / 扫描域为空 / 例外表条数不符 / 自检未过）
"""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile

PY = sys.executable or "python3"
HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(HERE)

# 扫描域排除名单：逐字照抄《清单-命名规范化》§0（EXCL / PRE / 隐藏项口径）
EXCL_DIRS = ("node_modules", "target", "__pycache__", ".history", ".obsidian", "dist",
             ".zerg", "zerg-wt", "venv", ".cargo", ".git")
EXCL_PREFIX = (".git/", "vendor/", "docs/虫族文档/", "docs/调研/multi-agent-源码/",
               "tools/ocr/venv/")
FROZEN_PREFIX = ("docs/项目文档/", "docs/issues/")

FULLW = "\u3000\uff08\uff09\u3010\u3011\u300c\u300d\uff0c\u3002\u3001\uff1a\uff1b\uff01\uff1f\u201c\u201d"
DOCISH_EXT = (".md", ".markdown", ".mdx", ".tsv", ".csv", ".json", ".yaml", ".yml", ".py",
              ".sh", ".rs", ".go", ".txt")
CAPS_STEM = re.compile(r"^[A-Z][A-Z0-9_]{2,}$")
DATE8 = re.compile(r"-\d{8}(?=$|[-.])")
DATE6 = re.compile(r"-\d{6}(?=$|[-.])")
VER_MID = re.compile(r"-v\d+(?:\.\d+)*-")
VER_TAIL = re.compile(r"-v\d+(?:\.\d+)*$")
NN_PREFIX = re.compile(r"^\d{2,3}-")
CJK = re.compile(r"[\u4e00-\u9fff]")
LAT = re.compile(r"[A-Za-z]")

# 「行业约定保留名」（规则级 · B 档）—— 只在**文档面之外**豁免：
#   文档面（docs/）内的同名件照判（mkdocs `index.md` 约定 / K8），仓根与工具面的约定件不判。
RESERVED_STEM = frozenset((
    "README.md", "AGENTS.md", "SKILL.md", "LICENSE", "NOTICE", "SECURITY.md",
    "CONTRIBUTING.md", "THIRD_PARTY_LICENSES.md", "VERSION", "MAKEFILE",
))

RULES = [
    ("N1", "FAIL", "①大小写：文件词干全为大写拉丁字母（≥3）",
     "《清单》§1 ①（与 mkdocs `index.md` 约定冲突；本卷 APFS 大小写不敏感）"),
    ("N2", "FAIL", "②空格/全角：名称含 U+0020 / U+3000 / 全角标点",
     "《清单》§1 ②"),
    ("N3", "FAIL", "③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM",
     "《清单》§5 ③（日期移入 frontmatter updated_at）+ §5 ⑦（6 位日期）"),
    ("N4", "FAIL", "③版本段不在尾位：-vX.Y 其后还有分隔段",
     "《清单》§5 ③（`名-…-vX.Y[-YYYYMMDD]`，版本段移尾）"),
    ("N5", "WARN", "④前缀序号不一致：同层 NN- 与无前缀并存",
     "《清单》§5 ④（结构级 · 只登记不改 · 牵动 nav/白名单）"),
    ("N6", "FAIL", "⑥同目录同基名、扩展名不同（去最后一个扩展名后相同）",
     "《清单》§1 ⑥（K8 那类）+ §4.3 允许对"),
    ("N7", "WARN", "⑤中英紧邻：同一分隔段内汉字与拉丁字母直接相邻",
     "《清单》§5 ⑤（机械插 `-` 需人审）"),
]

# ── A 档：《清单-命名规范化》已登记的「不改」条目 22 条 ──────────────────
# 逐条抄自清单/TSV：id · TSV 行号 · 不规范类型码 · 匹配形态 · 理由 · 出处。
# 匹配形态四选一：path（精确相对路径）· prefix（目录前缀）· name（基名，任意深度）
#               · pair（目录 + 去最后扩展名的基名）
EXEMPT_A = [
    {"id": "A01", "row": 2, "type": "④", "kind": "prefix", "rules": ["N5"],
     "paths": ["docs/常青/", "docs/skills/", "docs/调研/", "docs/03-评审/"],
     "reason": "四个正式面候选目录**无 NN- 前缀**，而 01-设计/02-调研/thunderbolt 有 ⇒ 结构级",
     "src": "清单 §5 ④ · TSV 第 2 行（建议 = 不改 · 结构级，仅登记）"},
    {"id": "A02", "row": 3, "type": "①", "kind": "path", "rules": ["N1"],
     "paths": ["README.md"],
     "reason": "行业约定文件；`publish/whitelist.txt` 公开面入口",
     "src": "清单 §4.2 · TSV 第 3 行（建议 = 不改）"},
    {"id": "A03", "row": 5, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("core/internal/compat", "compat")],
     "reason": "Go 包同名约定（compat.go + compat.json）",
     "src": "清单 §4.3 · TSV 第 5 行（建议 = 不改 · 登记为允许对）"},
    {"id": "A04", "row": 6, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("core/internal/compat", "compat")],
     "reason": "同上（JSON 侧）",
     "src": "清单 §4.3 · TSV 第 6 行（同上）"},
    {"id": "A05", "row": 8, "type": "①", "kind": "path", "rules": ["N1"],
     "paths": ["AGENTS.md"],
     "reason": "行业约定文件（agent 入口）；`publish/private-paths.txt:29` 明列",
     "src": "清单 §4.2 · TSV 第 8 行（建议 = 不改）"},
    {"id": "A06", "row": 15, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("scripts", "edit-assert")],
     "reason": "无扩展可执行件；改名 = 断全部 `python3 scripts/edit-assert` 调用方",
     "src": "清单 §4.3 · TSV 第 15 行（建议 = 不改 · 登记为允许对）"},
    {"id": "A07", "row": 16, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("scripts", "edit-assert")],
     "reason": "`.md` = 同一脚本的说明书",
     "src": "清单 §4.3 · TSV 第 16 行（同上）"},
    {"id": "A08", "row": 21, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("tools", "doc_index_gen")],
     "reason": "`tools/*.md`（145 个）为工具说明书，是既有约定",
     "src": "清单 §4.3 · TSV 第 21 行（同上）"},
    {"id": "A09", "row": 22, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("tools", "doc_index_gen")],
     "reason": "同上（实现侧）",
     "src": "清单 §4.3 · TSV 第 22 行（同上）"},
    {"id": "A10", "row": 23, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("tools", "kb_docs_sync")],
     "reason": "同上（工具说明书）",
     "src": "清单 §4.3 · TSV 第 23 行（同上）"},
    {"id": "A11", "row": 24, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("tools", "kb_docs_sync")],
     "reason": "同上（实现侧）",
     "src": "清单 §4.3 · TSV 第 24 行（同上）"},
    {"id": "A12", "row": 28, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("scripts", "mutate-scan")],
     "reason": "无扩展可执行件（同 edit-assert）",
     "src": "清单 §4.3 · TSV 第 28 行（同上）"},
    {"id": "A13", "row": 29, "type": "⑥", "kind": "pair", "rules": ["N6"],
     "paths": [("scripts", "mutate-scan")],
     "reason": "`.md` = 同一脚本的说明书",
     "src": "清单 §4.3 · TSV 第 29 行（同上）"},
    {"id": "A14", "row": 30, "type": "④", "kind": "prefix", "rules": ["N5"],
     "paths": ["docs/项目文档/v2.0.0/", "docs/项目文档/v2.1/", "docs/项目文档/v2.2/",
               "docs/项目文档/v2.3/", "docs/项目文档/v2.4/", "docs/项目文档/v2.6/"],
     "reason": "六目录全套无 NN- 前缀，而 v2.5.0–v2.5.9 全套有 ⇒ 跨快照不一致 ⇒ 冻结归档",
     "src": "清单 §5 ④ · TSV 第 30 行（建议 = 不改 · 冻结归档）"},
    {"id": "A15", "row": 31, "type": "①", "kind": "prefix_name", "rules": ["N1"],
     "paths": ["docs/项目文档/"], "name": "INDEX.md",
     "reason": "17 套快照各一份 INDEX.md（§2.2 实测 17 个副本 / 17 种内容）⇒ 冻结归档只登记不改",
     "src": "清单 §2 / §4.5 · TSV 第 31 行（建议 = 不改 · 冻结归档）"},
    {"id": "A16", "row": 32, "type": "④", "kind": "prefix", "rules": ["N5"],
     "paths": ["docs/项目文档/v2.5.10/"],
     "reason": "同目录内 01-..13- 前缀与无前缀条目并存 ⇒ 冻结归档",
     "src": "清单 §5 ④ · TSV 第 32 行（建议 = 不改 · 冻结归档）"},
    {"id": "A17", "row": 33, "type": "⑦", "kind": "path", "rules": ["N3", "N4", "N6"],
     "paths": ["docs/issues/tmp-产物清单-20260914.md"],
     "reason": "`tmp-` 临时前缀进了正式目录 ⇒ 冻结归档（docs/issues/ 在外）",
     "src": "清单 §5 ⑦ · TSV 第 33 行（建议 = 不改 · 冻结归档）"},
    {"id": "A18", "row": 78, "type": "④+⑤", "kind": "prefix", "rules": ["N5", "N7"],
     "paths": ["docs/调研/协作骨架v2.0-核验-20260918/", "docs/调研/协作骨架v2.1-仓内核对-20260918/"],
     "reason": "目录名里中英紧邻（`骨架v2.0`）+ 版本号 + 日期三件混写 ⇒ 结构级只登记",
     "src": "清单 §5 ④ · TSV 第 78 行（建议 = 不改 · 结构级，仅登记）"},
    {"id": "A19", "row": 95, "type": "⑦", "kind": "path", "rules": ["N3", "N4", "N6"],
     "paths": ["gateway/fleet.yaml.bak"],
     "reason": "移出仓（或 `gateway/.bak/`）—— 备件残片不入正式树",
     "src": "清单 §5 ⑦ · TSV 第 95 行（建议 = 移出仓）"},
    {"id": "A20", "row": 143, "type": "③", "kind": "path", "rules": ["N3", "N4"],
     "paths": ["docs/常青/白皮书-虫族经济模型-v0.1.md"],
     "reason": "版本号已在尾位，与本仓 `名-…-vX.Y[-YYYYMMDD]` 约定一致",
     "src": "清单 §5 · TSV 第 143 行（建议 = 不改 · 版本号已在尾位）"},
    {"id": "A21", "row": 146, "type": "⑦", "kind": "path", "rules": ["N3", "N4", "N6"],
     "paths": ["gateway/fleet.yaml.bak-20260914-110609"],
     "reason": "移出仓（同 A19）",
     "src": "清单 §5 ⑦ · TSV 第 146 行（建议 = 移出仓）"},
    {"id": "A22", "row": 147, "type": "①", "kind": "name", "rules": ["N1"],
     "paths": ["LICENSE", "NOTICE", "SECURITY.md", "CONTRIBUTING.md", "THIRD_PARTY_LICENSES.md"],
     "reason": "公开面法定/治理文件，全大写是行业约定",
     "src": "清单 §5 · TSV 第 147 行（建议 = 不改 · 符合行业约定）"},
]

# ── B 档：规则级规范豁免（不在《清单》22 条里；逐条给理由，报告里照报命中数）──
EXEMPT_B = [
    {"id": "B01", "kind": "reserved", "rules": ["N1"],
     "paths": ["README.md", "AGENTS.md", "SKILL.md", "VERSION"],
     "reason": "行业/工具约定保留名 —— **只在文档面之外生效**（docs/ 内的同名件照判）"},
    {"id": "B02", "kind": "pair_ext", "rules": ["N6"],
     "paths": [("go", (".mod", ".sum")), ("Cargo", (".toml", ".lock"))],
     "reason": "构建工具链约定（Go module / Cargo），不属文档命名面。"
               "**休眠申报**：N6 的口径（至少一方是文档/脚本面扩展）已把它们挡在门外 ⇒ 本仓当前 0 命中"},
    {"id": "B03", "kind": "pair_doc_machine", "rules": ["N6"],
     "paths": [((".md", ".markdown", ".mdx"), (".tsv", ".csv", ".json"))],
     "reason": "人读件 + 机读件同基名（本仓既有形态：`清单-命名规范化-20260918.md/.tsv` = 设计件 + 机读件）。"
               "**我方判断 · 与本脚本自身的差异已登记**：去掉这一条，该对会被判 ⑥（见 `scripts/check-doc-name.md` §差异）"},
]

# ── C 档：本轮新增登记（W2 交付件自身的 `脚本 + 说明书` 对，同 `scripts/edit-assert` 既有约定）──
EXEMPT_C = [
    {"id": "C01", "kind": "pair", "rules": ["N6"], "paths": [("scripts", "check-doc-meta")],
     "reason": "本轮 W2 交付：门脚本 + 说明书（同 `scripts/edit-assert` 既有约定）· **待父代理批准**"},
    {"id": "C02", "kind": "pair", "rules": ["N6"], "paths": [("scripts", "check-doc-name")],
     "reason": "同上（本文档脚本 + 说明书）· **待父代理批准**"},
]
EXEMPT_A_EXPECTED = 22
EXEMPT_B_EXPECTED = 3
EXEMPT_C_EXPECTED = 2


def _rule_ids(type_code):
    m = {"①": ["N1"], "②": ["N2"], "③": ["N3", "N4"], "④": ["N5"], "⑤": ["N7"],
         "⑥": ["N6"], "⑦": ["N3", "N4", "N6"]}
    out = []
    for ch in type_code.replace("+", " ").split():
        out += m.get(ch, [])
    return out


for _row in EXEMPT_A:
    _from_type = _rule_ids(_row["type"])
    _row["rules"] = sorted(set(_from_type) | set(_row["rules"]))

EXEMPT_ALL = EXEMPT_A + EXEMPT_B + EXEMPT_C


def exempt_row_for(rel, rule, kind, pair_key=None, stem=None, base=None):
    """返回命中的 A/B/C 档豁免行（None = 不豁免）。kind = file|dir|pair。"""
    # 目录条目一律按「带尾斜杠」比较（豁免表的 prefix 形态写的是 `xxx/`）
    rel_n = rel if (kind != "dir" or rel.endswith("/")) else rel + "/"
    for row in EXEMPT_ALL:
        if rule not in row["rules"]:
            continue
        rk = row["kind"]
        if rk == "path":
            if (kind == "file" and rel in row["paths"]) or (kind == "dir" and rel_n in row["paths"]):
                return row
        elif rk == "prefix":
            if kind in ("file", "dir") and any(rel_n.startswith(p) for p in row["paths"]):
                return row
        elif rk == "name":
            if kind == "file" and base in row["paths"]:
                return row
        elif rk == "prefix_name":
            if kind == "file" and base == row["name"] and any(rel.startswith(p) for p in row["paths"]):
                return row
        elif rk == "reserved":
            if kind == "file" and base in row["paths"] and not rel.startswith("docs/"):
                return row
        elif rk == "pair":
            if kind == "pair" and pair_key in row["paths"]:
                return row
    return None


def pair_ext_exempt(stem, exts):
    """B02 工具链同基名（key = 基名）；B03 人读件 + 机读件（key = 人读扩展名元组）。"""
    s = set(exts)
    for row in EXEMPT_B:
        if row["kind"] == "pair_ext":
            for key, want in row["paths"]:
                if key == stem and s <= set(want):
                    return row
        elif row["kind"] == "pair_doc_machine":
            for doc, machine in row["paths"]:
                if (s & set(doc)) and (s & set(machine)) and len(s) == 2:
                    return row
    return None


# ── 扫描 ──────────────────────────────────────────────────────────────
def walk(root, targets):
    """返回 [(rel, kind, abs_or_None)]；kind = file|dir。点文件一律跳过（同《清单》§0 隐藏项口径）。"""
    out = []
    seen = set()

    def add(rel, kind, ap):
        if rel in seen or rel in ("", "."):
            return
        seen.add(rel)
        out.append((rel, kind, ap))

    for t in targets:
        if os.path.isfile(t):
            add(os.path.relpath(t, root).replace(os.sep, "/"), "file", os.path.abspath(t))
            continue
        if os.path.isdir(t):
            # 目标目录**自身**也是一个待判名字（点名一个坏目录时不能漏掉它）
            add(os.path.relpath(t, root).replace(os.sep, "/"), "dir", os.path.abspath(t))
        for dp, dns, fns in os.walk(t):
            rel = os.path.relpath(dp, root).replace(os.sep, "/")
            relp = "" if rel == "." else rel + "/"
            if relp and any(relp.startswith(p) for p in EXCL_PREFIX):
                dns[:] = []
                continue
            dns[:] = [d for d in dns if d not in EXCL_DIRS and not d.startswith(".")]
            for d in sorted(dns):
                add(relp + d, "dir", os.path.join(dp, d))
            for f in sorted(fns):
                if f.startswith("."):
                    continue
                add(relp + f, "file", os.path.join(dp, f))
    out.sort()
    return out


def judge_name(rel, kind):
    """返回 [(rule, 档, 说明)]；档 = FAIL|WARN。目录命中一律 WARN（结构级）。"""
    base = os.path.basename(rel)
    stem = os.path.splitext(base)[0]
    hits = []

    def add(rule, msg):
        sev = "FAIL" if kind == "file" else "WARN"
        row = exempt_row_for(rel, rule, kind, stem=stem, base=base)
        hits.append((rule, sev, msg, row))

    if CAPS_STEM.match(stem):
        add("N1", "词干 %r 全为大写拉丁字母" % stem)
    bad = [c for c in base if c in FULLW or c in (" ",)]
    if bad:
        add("N2", "名称含 %s" % " / ".join("U+%04X" % ord(c) for c in sorted(set(bad))))
    md = DATE8.search(stem) or DATE6.search(stem)
    if md:
        add("N3", "词干含日期后缀 %r ⇒ 日期应移入 frontmatter `updated_at`" % md.group(0))
    if VER_MID.search(stem) and not VER_TAIL.search(stem):
        add("N4", "版本段 %r 不在尾位 ⇒ 应移到尾部（`名-…-vX.Y[-YYYYMMDD]`）"
            % VER_MID.search(stem).group(0))
    segs = [s for s in re.split(r"[-_.\s]", stem) if s]
    for seg in segs:
        joined = False
        for i in range(len(seg) - 1):
            a, b = seg[i], seg[i + 1]
            if (CJK.match(a) and LAT.match(b)) or (LAT.match(a) and CJK.match(b)):
                add("N7", "段 %r 内汉字与拉丁字母直接相邻（%s%s）" % (seg, a, b))
                joined = True
                break
        if joined:
            break
    return hits


def judge_same_stem(entries):
    """entries = [(rel, kind, ap)]；返回 [(rel_base, rule, 档, 说明, row)]（N6）。"""
    import collections
    groups = collections.defaultdict(list)
    for rel, kind, _ap in entries:
        if kind != "file":
            continue
        dn = os.path.dirname(rel)
        base = os.path.basename(rel)
        stem = os.path.splitext(base)[0]
        ext = base[len(stem):]
        groups[(dn, stem)].append((rel, ext))
    out = []
    for (dn, stem), members in sorted(groups.items()):
        exts = sorted(set(e for _r, e in members))
        if len(members) < 2 or len(exts) < 2:
            continue
        if not any(e in DOCISH_EXT for e in exts):
            continue                      # 非文档/脚本面（备件残片、构建清单）不判 ⑥
        key = (dn, stem)
        row = exempt_row_for(members[0][0], "N6", "pair", pair_key=key)
        if row is None and exts:
            row = pair_ext_exempt(stem, exts)
        out.append((key, [r for r, _e in members], exts, row))
    return out


def judge_prefix_mix(entries):
    """同层 NN- 与无前缀并存 ⇒ N5（WARN）。返回 [(目录, 有前缀数, 无前缀数, 子目录, row)]。"""
    import collections
    by_dir = collections.defaultdict(lambda: [[], []])
    children = collections.defaultdict(list)
    for rel, kind, _ap in entries:
        dn = os.path.dirname(rel)
        base = os.path.basename(rel)
        if kind == "dir":
            children[dn].append(rel + "/")
        by_dir[dn][0 if NN_PREFIX.match(base) else 1].append(base)
    out = []
    for dn, (pre, no) in sorted(by_dir.items()):
        if len(pre) >= 2 and len(no) >= 2:
            # 豁免：A 档把「结构级前缀不一致」登记在**那几层目录**上 ⇒ 父目录与它的子目录都查一遍
            row = exempt_row_for(dn + "/", "N5", "dir") if dn else None
            if row is None:
                for child in children.get(dn, []):
                    row = exempt_row_for(child, "N5", "dir")
                    if row is not None:
                        break
            out.append((dn or "<仓根>", len(pre), len(no), children.get(dn, []), row))
    return out


# ── 报告 ─────────────────────────────────────────────────────────────
def run(root, targets, count_frozen=False):
    entries = walk(root, targets)
    if not entries:
        raise RuntimeError("扫描域为空（一个文件/目录都没有）")
    if not any(k == "file" for _r, k, _a in entries):
        raise RuntimeError("扫描域里没有任何文件（只有目录名 ⇒ 判不了）")
    per_rule = {}
    per_rule_live = {}
    per_rule_frozen = {}
    examples = {}
    exempt_hits = {}
    frozen = {"FAIL": 0, "WARN": 0}
    live = {"FAIL": 0, "WARN": 0}
    for rel, kind, _ap in entries:
        is_frozen = any(rel.startswith(p) for p in FROZEN_PREFIX)
        for rule, sev, msg, row in judge_name(rel, kind):
            per_rule[rule] = per_rule.get(rule, 0) + 1
            if row is not None:
                exempt_hits[row["id"]] = exempt_hits.get(row["id"], 0) + 1
            else:
                (frozen if is_frozen else live)[sev] += 1
                tgt = per_rule_frozen if is_frozen else per_rule_live
                tgt[rule] = tgt.get(rule, 0) + 1
                examples.setdefault(rule, []).append(
                    "%s%s → %s" % ("[冻结] " if is_frozen else "", rel, msg))
    for key, members, exts, row in judge_same_stem(entries):
        is_frozen = any(m.startswith(p) for p in FROZEN_PREFIX for m in members)
        per_rule["N6"] = per_rule.get("N6", 0) + 1
        if row is not None:
            exempt_hits[row["id"]] = exempt_hits.get(row["id"], 0) + 1
        else:
            (frozen if is_frozen else live)["FAIL"] += 1
            tgt = per_rule_frozen if is_frozen else per_rule_live
            tgt["N6"] = tgt.get("N6", 0) + 1
            examples.setdefault("N6", []).append(
                "%s%s/%s → 同基名不同扩展 %s"
                % ("[冻结] " if is_frozen else "", key[0] or "<仓根>", key[1], "/".join(exts)))
    for dn, np_, nn, children, row in judge_prefix_mix(entries):
        per_rule["N5"] = per_rule.get("N5", 0) + 1
        if row is not None:
            exempt_hits[row["id"]] = exempt_hits.get(row["id"], 0) + 1
        else:
            live["WARN"] += 1
            per_rule_live["N5"] = per_rule_live.get("N5", 0) + 1
            examples.setdefault("N5", []).append(
                "%s → 同层混用 NN- 前缀（前缀 %d · 无前缀 %d）" % (dn, np_, nn))
    if count_frozen:
        # --count-frozen：冻结区也进退码（默认不进：《清单》§2「高（冻结·不改）」）
        live["FAIL"] += frozen["FAIL"]
        live["WARN"] += frozen["WARN"]
        for k, v in per_rule_frozen.items():
            per_rule_live[k] = per_rule_live.get(k, 0) + v
    return {"entries": len(entries), "per_rule": per_rule, "examples": examples,
            "exempt_hits": exempt_hits, "live": live, "frozen": frozen,
            "per_rule_live": per_rule_live, "per_rule_frozen": per_rule_frozen}


def print_report(rep, args):
    print("扫描域：%d 个文件/目录（排除名单同《清单》§0）" % rep["entries"])
    print("  不合规 %d（冻结区 %d）· 告警 %d（冻结区 %d）"
          % (rep["live"]["FAIL"], rep["frozen"]["FAIL"],
             rep["live"]["WARN"], rep["frozen"]["WARN"]))
    print("  ★ 冻结区 = docs/项目文档/ · docs/issues/（《清单》§2「高（冻结·不改）」）⇒ 只报不计退码")
    for rid, sev, what, src in RULES:
        n = rep["per_rule"].get(rid, 0)
        if not n:
            continue
        print("  %s [%s] %s —— 命中 %d（可改面 %d · 冻结区 %d）"
              % (rid, sev, what, n, rep["per_rule_live"].get(rid, 0),
                 rep["per_rule_frozen"].get(rid, 0)))
        for line in rep["examples"].get(rid, [])[:args.max_examples]:
            print("      · %s" % line)
        if n > args.max_examples:
            print("      · …另 %d 条（--max-examples 调整）" % (n - args.max_examples))
    print("  豁免命中（逐条可查 · --list-exempt）："
          + (" · ".join("%s×%d" % (k, v) for k, v in sorted(rep["exempt_hits"].items()))
             if rep["exempt_hits"] else "无"))


def decide(rep):
    if rep["live"]["FAIL"]:
        return 1, "FAIL：%d 个不合规命中（只报告不改）" % rep["live"]["FAIL"]
    return 0, "OK：无不合规命中（告警 %d · 已豁免 %d）" % (
        rep["live"]["WARN"], sum(rep["exempt_hits"].values()))


def list_rules():
    print("check-doc-name.py 规则表（来源 = 《清单-命名规范化》§1 七类 + §2 风险档 + §5 处置）")
    for rid, sev, what, src in RULES:
        print("  %-3s [%-4s] %s" % (rid, sev, what))
        print("       出处：%s" % src)
    print("  档位口径：**文件名**命中才计入退码；**目录名**命中一律 WARN（结构级 · 只登记不改）；")
    print("            N5/N7 为 WARN；冻结区（docs/项目文档/ · docs/issues/）不进退码。")
    print("  退码：0 全绿 · 1 有不合规项 · 2 不给结论（优先级 2>1>0）。")


def _flat(p):
    """把豁免对象拍成一行字符串（形态里可以嵌 tuple，如 go+(.mod,.sum)）。"""
    if isinstance(p, (tuple, list)):
        return "/".join(_flat(x) for x in p)
    return str(p)


def list_exempt():
    print("例外表（三档 · 逐条给理由与出处）")
    print("A 档 · 《清单-命名规范化》已登记的「不改」条目：%d 条（期望 %d）"
          % (len(EXEMPT_A), EXEMPT_A_EXPECTED))
    for row in EXEMPT_A:
        print("  %s [%s] 类型 %s · TSV 第 %d 行 · 形态 %s" % (row["id"], "+".join(row["rules"]),
                                                          row["type"], row["row"], row["kind"]))
        print("      对象：%s" % " , ".join(_flat(p) for p in row["paths"]))
        print("      理由：%s" % row["reason"])
        print("      出处：%s" % row["src"])
    print("B 档 · 规则级规范豁免：%d 条（期望 %d）" % (len(EXEMPT_B), EXEMPT_B_EXPECTED))
    for row in EXEMPT_B:
        print("  %s [%s] 形态 %s\n      对象：%s\n      理由：%s"
              % (row["id"], "+".join(row["rules"]), row["kind"],
                 " , ".join(_flat(p) for p in row["paths"]), row["reason"]))
    print("C 档 · 本轮新增登记（待父代理批准）：%d 条（期望 %d）" % (len(EXEMPT_C), EXEMPT_C_EXPECTED))
    for row in EXEMPT_C:
        print("  %s [%s] 形态 %s\n      对象：%s\n      理由：%s"
              % (row["id"], "+".join(row["rules"]), row["kind"],
                 " , ".join(_flat(p) for p in row["paths"]), row["reason"]))
    return 0


# ── 自检（成对负控）──────────────────────────────────────────────────
GOOD_FILES = {
    "docs/01-设计/设计-文档体系-v1.0.md": "# x\n",
    "docs/调研/调研-Hermes-与虫族对话机制对比.md": "# x\n",
    "docs/zh/模块-主控.md": "# x\n",
    "README.md": "# x\n",
    "scripts/edit-assert": "#!/usr/bin/env python3\n",
    "scripts/edit-assert.md": "# 说明书\n",
    "docs/项目文档/v2.5.10/01-模块-主控.md": "# x\n",
}
# 坏件（文件名 ⇒ 期望命中规则）——逐条成对
BAD_FILES = [
    ("docs/INDEX.md", "N1"),
    ("docs/研究 - 自我纠错.md", "N2"),
    ("docs/01-设计/设计-文档体系-20260918.md", "N3"),
    ("docs/02-调研/调研-虫族同类项目与定位-202608.md", "N3"),
    ("docs/01-设计/设计-v2.5.10-卵功能与对话机制.md", "N4"),
    ("docs/01-设计/设计-文档体系.md", None),          # 好件（对照组）
]
BAD_DIRS = [
    ("docs/调研/协作骨架vX-核验/", "N7"),
    ("docs/新目录-20260918/", "N3"),
]
BAD_PAIRS = [
    ("docs/01-设计/设计-文档体系.md", "docs/01-设计/设计-文档体系.py", "N6"),
]
BAD_PREFIXMIX_DIR = "docs/混用目录"


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _sub(script, *args):
    p = subprocess.run([PY, script] + list(args), stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, universal_newlines=True)
    return p.returncode, p.stdout


def self_test(script_path):
    lines = []
    ok = True
    tmp = tempfile.mkdtemp(prefix="check-doc-name-", dir="/tmp")
    good = os.path.join(tmp, "good")
    for rel, text in GOOD_FILES.items():
        _write(os.path.join(good, rel), text)
    bad = os.path.join(tmp, "bad")
    for rel, _rule in BAD_FILES:
        _write(os.path.join(bad, rel), "# x\n")
    for rel, _rule in BAD_DIRS:
        _write(os.path.join(bad, rel, "a.md"), "# x\n")
    for a, b, _rule in BAD_PAIRS:
        _write(os.path.join(bad, a), "# x\n")
        _write(os.path.join(bad, b), "x\n")
    mix = os.path.join(bad, BAD_PREFIXMIX_DIR)
    for rel in ("01-a.md", "02-b.md", "c.md", "d.md"):
        _write(os.path.join(mix, rel), "# x\n")
    pairs = os.path.join(tmp, "pairs")
    _write(os.path.join(pairs, "docs", "01-设计", "机读对.md"), "# x\n")
    _write(os.path.join(pairs, "docs", "01-设计", "机读对.tsv"), "x\n")
    empty = os.path.join(tmp, "empty")
    os.makedirs(empty, exist_ok=True)

    base = ["--repo-root", tmp, "--no-self-test"]

    def case(label, expect_rc, args, must_contain=(), must_not_contain=()):
        rc, out = _sub(script_path, *(base + list(args)))
        good_case = (rc == expect_rc)
        for s in must_contain:
            if s not in out:
                good_case = False
        for s in must_not_contain:
            if s in out:
                good_case = False
        lines.append("%s %s（期望 rc=%d，实得 rc=%d）"
                     % ("✓" if good_case else "✗", label, expect_rc, rc))
        if not good_case:
            lines.append("    输出尾部：%s" % out.strip().splitlines()[-4:])
        return good_case

    # ① 好件必绿（含已登记允许对 edit-assert / edit-assert.md）——
    #    该夹具**自己当仓根**（豁免表的 pair 键含目录，必须与仓根口径一致）
    ok &= case("好件必绿（含允许对 edit-assert）", 0,
               ["--target", good, "--repo-root", good],
               must_contain=["OK", "A06"])
    # ② 坏件必红（逐条命中它自己的规则号）
    for rel, rule in BAD_FILES:
        if rule is None:
            continue
        ok &= case("坏件必红 %s" % rel, 1, ["--target", os.path.join(bad, rel)],
                   must_contain=[rule])
    ok &= case("整棵坏树必红且含 N6（同基名）", 1, ["--target", bad], must_contain=["N6"])
    # ③ 目录名命中只算 WARN（结构级）⇒ rc=0 + 命中数可查
    for rel, rule in BAD_DIRS:
        ok &= case("目录名命中只告警 %s" % rel, 0,
                   ["--target", os.path.join(bad, rel.rstrip("/"))], must_contain=[rule, "告警"])
    # ④ 同基名（未登记的对）必红 —— 判据按目录成组 ⇒ 扫该文件所在目录
    for a, _b, rule in BAD_PAIRS:
        ok &= case("同基名未登记必红 %s" % a, 1,
                   ["--target", os.path.join(bad, os.path.dirname(a))],
                   must_contain=[rule])
    # ④′ 同基名的**已豁免**形态（人读件 + 机读件 .md/.tsv ⇒ B03）必绿 —— 豁免机制要有牙齿
    ok &= case("同基名 .md/.tsv 对按 B03 豁免", 0, ["--target", pairs],
               must_contain=["B03", "OK"])
    # ⑤ 前缀序号混用 = WARN（rc=0）
    ok &= case("前缀序号混用只告警", 0, ["--target", mix], must_contain=["N5", "告警"])
    # ⑥ 缺件必 rc=2
    ok &= case("缺件 · 目标路径不存在", 2, ["--target", os.path.join(tmp, "no-such")],
               must_contain=["不给结论"])
    ok &= case("缺件 · 扫描域为空", 2, ["--target", empty], must_contain=["不给结论"])
    # ⑦ 例外表条数自证（A 档 22 条 · B 档 3 条 · C 档 2 条）
    rc, out = _sub(script_path, "--list-exempt")
    cnt_ok = (rc == 0 and ("A 档 · 《清单-命名规范化》已登记的「不改」条目：22 条（期望 22）" in out)
              and ("B 档 · 规则级规范豁免：3 条（期望 3）" in out)
              and ("C 档 · 本轮新增登记（待父代理批准）：2 条（期望 2）" in out))
    lines.append("%s 例外表条数自证（A22 / B3 / C2）" % ("✓" if cnt_ok else "✗"))
    ok &= cnt_ok
    # ⑧ 只报告不改
    import hashlib

    def tree_sha(d):
        h = hashlib.sha256()
        for dp, dns, fns in os.walk(d):
            dns.sort()
            for f in sorted(fns):
                p = os.path.join(dp, f)
                h.update(os.path.relpath(p, d).encode())
                with open(p, "rb") as fh:
                    h.update(fh.read())
        return h.hexdigest()

    before = tree_sha(tmp)
    _sub(script_path, *(base + ["--target", bad]))
    same = (before == tree_sha(tmp))
    lines.append("%s 只报告不改（夹具树逐字节不变：%s）"
                 % ("✓" if same else "✗", "一致" if same else "被改动"))
    ok &= same

    expected = (1                                   # ① 好件必绿
                + len([1 for _r, x in BAD_FILES if x])   # ② 坏件必红（逐条）
                + 1                                 # ②′ 整棵坏树
                + len(BAD_DIRS)                     # ③ 目录名只告警
                + len(BAD_PAIRS)                    # ④ 同基名未登记必红
                + 1                                 # ④′ 同基名 B03 豁免
                + 1                                 # ⑤ 前缀混用只告警
                + 2                                 # ⑥ 缺件 ×2（目标 / 空扫描域）
                + 1                                 # ⑦ 例外表条数自证
                + 1)                                # ⑧ 只报告不改
    got = sum(1 for ln in lines if ln[:1] in ("✓", "✗"))
    lines.append("自检用例：期望 %d 条 · 实跑 %d 条" % (expected, got))
    if got != expected:
        lines.append("✗ 用例条数不符（有静默截断）")
        ok = False
    lines.append("自检临时目录：%s（未删除）" % tmp)
    return ok, lines


def main(argv):
    ap = argparse.ArgumentParser(description="命名规范化门（只报告不改）")
    ap.add_argument("targets", nargs="*")
    ap.add_argument("--target", action="append", default=[])
    ap.add_argument("--repo-root", default=REPO_ROOT_DEFAULT)
    ap.add_argument("--max-examples", type=int, default=5)
    ap.add_argument("--count-frozen", action="store_true", help="把冻结区计入退码")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--list-exempt", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    args = ap.parse_args(argv)

    for n, exp in (("A", EXEMPT_A_EXPECTED), ("B", EXEMPT_B_EXPECTED), ("C", EXEMPT_C_EXPECTED)):
        got = {"A": len(EXEMPT_A), "B": len(EXEMPT_B), "C": len(EXEMPT_C)}[n]
        if got != exp:
            print("BLOCKED：例外表 %s 档条数不符（期望 %d，实得 %d）⇒ 不给结论" % (n, exp, got))
            return 2

    if args.list_rules:
        list_rules()
        return 0
    if args.list_exempt:
        return list_exempt()
    if args.self_test:
        ok, lines = self_test(os.path.abspath(__file__))
        print("check-doc-name 自检（成对负控：好件必绿 / 坏件必红 / 缺件必 rc=2）")
        for ln in lines:
            print("  " + ln)
        return 0 if ok else 2
    if not args.no_self_test:
        ok, lines = self_test(os.path.abspath(__file__))
        if not ok:
            print("自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）")
            for ln in lines:
                print("  " + ln)
            return 2

    root = os.path.abspath(args.repo_root)
    if not os.path.isdir(root):
        print("BLOCKED：仓根不是目录 ⇒ 不给结论：%s" % root)
        return 2
    targets = [os.path.abspath(x) for x in (list(args.targets) + list(args.target))] or [root]
    for t in targets:
        if not os.path.exists(t):
            print("BLOCKED：目标不存在 ⇒ 不给结论：%s" % t)
            return 2
    try:
        rep = run(root, targets, args.count_frozen)
    except RuntimeError as e:
        print("BLOCKED：%s ⇒ 不给结论" % e)
        return 2
    rc, verdict = decide(rep)
    if args.json:
        print(json.dumps({"rc": rc, "verdict": verdict, "entries": rep["entries"],
                          "live": rep["live"], "frozen": rep["frozen"],
                          "exempt_hits": rep["exempt_hits"],
                          "per_rule": dict(sorted(rep["per_rule"].items())),
                          "per_rule_live": dict(sorted(rep["per_rule_live"].items())),
                          "per_rule_frozen": dict(sorted(rep["per_rule_frozen"].items()))},
                         ensure_ascii=False, indent=2))
        return rc
    print("check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）")
    print_report(rep, args)
    print(verdict)
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-version-sources.py v1.0.0 —— 版本源一致性门禁（门② · 目标：阻断档）

断言**四处版本源同版**（真源 = version.go；理由见仓根 VERSION 的指针文本）：
  ① 仓根 VERSION                        —— **指针**：只断言「**无数值**」且「**文本指向真源**」
  ② core/internal/version/version.go    —— `const Version = "X"`（**基准**）
  ③ ui/Cargo.toml                       —— `[package] version = "X"`
  ④ wall/Cargo.toml                     —— `[package] version = "X"`（茧壁 crate，升级比较要用）

另**只报告**（**不进退码**）：**活文档里仍写着旧版本号字面量**的清单
  · 旧版集合默认 = 「直接前驱版」= `docs/项目文档/v*/` 里已知的 3 段版本号中 < 当前版的最大者
  · 排除域（冻结/镜像/证据/Tool 目录）见 RULES 的 doc_excludes；命中逐条列 `文件:行`
  · 命中**分两类**：`版本名/路径形态`（`v2.5.9`、`…/v2.5.9/…`、`变更-v2.5.9.md` —— 指的是旧版的名字，登记为主）
    与 `裸数字形态`（`2.5.9 → 2.5.10`、`2.5.9 增补` 之类 —— 需人判读：可能是陈旧陈述、也可能是沿革陈述）
  · ★ 本项**只报告**：不构成判决；判读由人做（要不要加严、门槛多少待拍板）。`--strict-docs` 可当场加严。

退码（三档，与 scripts/precommit-gates.sh 的 `tri` 模式同口径 —— 该脚本第 29 行起）：
  0 全绿（成对判定：四处同版 + VERSION 无数值 + 指针指向真源）
  1 有失败项（版本不一致 / VERSION 出现数值 / 指针不指真源 / `--strict-docs` 下的活文档命中）
  2 **不给结论**（缺件 · 不可解析 · 空转 · 用法错）—— 空转不许当绿

用法：
  python3 scripts/check-version-sources.py                    # 扫真仓（退出码见上）
  python3 scripts/check-version-sources.py [仓库根]           # 指定根（自检夹具就走这条）
  python3 scripts/check-version-sources.py --json             # 机器可读（stdout 只有 JSON）
  python3 scripts/check-version-sources.py --list-rules       # 规则表（判据 / 排除域 / 旧版集合 / 退码映射）
  python3 scripts/check-version-sources.py --self-test        # 成对负控自检（好件必绿 ↔ 每处改一个数必红）
  python3 scripts/check-version-sources.py --strict-docs      # 活文档命中 ⇒ 计入 rc=1（默认只报告）
  python3 scripts/check-version-sources.py --stale 2.5.9 --stale 2.5.8   # 手工指定旧版集合
  python3 scripts/check-version-sources.py --stale-all        # 旧版集合 = 全部已知 3 段历史版本

硬纪律：**只读**（不写仓内任何路径；自检一律落 `tempfile.mkdtemp()` 的临时目录）· **不挂门禁**
（挂接由父代理做：进 `scripts/precommit-gates.sh` 的步骤表，模式 `tri` —— rc=2 记 BLOCKED、不计失败项）。
"""
import argparse
import io
import json
import os
import re
import shutil
import sys
import tempfile

TOOL = "check-version-sources.py"
TOOL_VERSION = "v1.0.0"

RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2
RC_NAME = {0: "OK（全绿）", 1: "FAIL（有失败项）", 2: "BLOCKED（不给结论）"}

# ── 四处版本源（相对仓根的路径）────────────────────────────────────
P_VERSION_FILE = "VERSION"
P_GO = os.path.join("core", "internal", "version", "version.go")
P_UI = os.path.join("ui", "Cargo.toml")
P_WALL = os.path.join("wall", "Cargo.toml")

# VERSION 的指针文本必须点名的两个真源（判据是**路径串**，不是措辞）
VERSION_POINTERS = ["core/internal/version/version.go", "ui/Cargo.toml"]

# 3 段版本号（前后不紧邻数字/点，避免把 2.5.9.1 或 12.5.9 切碎）
NUM_RE = re.compile(r"(?<![\d.])(\d+\.\d+\.\d+)(?![\d.])")

# 活文档语料域：docs/**（走排除域）+ 仓根 *.md
CORPUS_DOCS_PREFIX = "docs/"
CORPUS_ROOT_MD = True
SKIP_DIR_NAMES = {".obsidian", ".git", "target", "node_modules", ".venv", "__pycache__"}
MAX_FILE_BYTES = 2 * 1024 * 1024  # 超过 2 MB 的文档不读（登记为跳过，不进退码）

# 排除域（**冻结/镜像/证据/历史报告**）：这些域里的旧版本号是设计，不是债
DOC_EXCLUDES_FIXED = [
    ("Zerg-内部文档/", "开发文档分家（2026-09-19）：设计稿/调研稿/评审稿/问题单整树在**工作树之外**的"
                   "同级目录 ⇒ 仓内若出现同名路径（搬回 / 软链 / 副本），其版本号属归档料，只登记不判"),
]
#   ★ 2026-09-19 开发文档分家批6：原两条 `docs/02-调研/raw/`（原始调研证据）与 `docs/issues/`（历史报告 /
#     事故记录）**已整目录迁至 `Zerg-内部文档/`**（零命中：两者今天在盘上都不存在）⇒ 条目**删除**。
#     **同时补牙**：删掉两条后本列若不补，就只剩 `DOC_EXCLUDES_RULE` 的「他版快照」一域在咬 ⇒ 排除域判据
#     变弱。故新增一条 `Zerg-内部文档/` 域，并在自检夹具里造一份假 `Zerg-内部文档/01-设计/旧文.md`（写着旧版号）
#     ⇒ 自检⑯ 的「排除域零命中」仍被这条咬住，另加**防空转断言**：每个声明在 `DOC_EXCLUDES_FIXED` 里的
#     前缀，夹具里必须真的有文件（域被改名/删掉而忘了改夹具 ⇒ 自检红，不许悄悄变弱）。
#   原第一项 `docs/虫族文档/`（镜像副本）已于更早一次退役时删除，沿革见说明书 §3。
# ★ 2026-09-19：原第一项 `docs/虫族文档/`（镜像副本，公开面镜像 · 冻结）**已退役**（备份见
#   ~/zerg-backup/…）⇒ 死条目删除（它今天匹配 0 个文件 ⇒ 删它不改任何计数：语料/清单逐位一致）。
#   镜像若日后重建，请把 ("docs/虫族文档/", "镜像副本（已退役后重建）") 加回本列。
DOC_EXCLUDES_RULE = [
    ("docs/项目文档/v<X>/（X ≠ 当前版）", "他版快照目录（17 套历史快照，冻结归档，勿改）"),
    ("docs/项目文档/v<X>/（X = 当前版）", "**不排除** —— 它是活文档，命中要进清单"),
    ("docs/.* 与 */.hidden", "工具/隐藏目录（如 docs/.obsidian/）"),
]
DOC_EXCLUDES_NOTE = "排除域只影响**只报告**那一段；四处版本源的断言不受任何排除域影响。"


# ─────────────────────────── 解析器 ───────────────────────────

def read_text(path):
    """读 UTF-8 文本；返回 (text, err)。err != None ⇒ 不可解析（判据不可判 ⇒ BLOCKED）。"""
    try:
        with io.open(path, "r", encoding="utf-8") as f:
            return f.read(), None
    except (IOError, OSError) as e:
        return None, "读不到（%s）" % e
    except UnicodeDecodeError as e:
        return None, "不是 UTF-8 文本（%s）" % e


def parse_go_const(text, name="Version"):
    """`const Version = "X"` ⇒ (值, 行号)；不匹配 ⇒ (None, None)。"""
    for i, line in enumerate(text.splitlines(), 1):
        m = re.match(r'\s*const\s+%s\s*=\s*"([^"]+)"\s*$' % re.escape(name), line)
        if m:
            return m.group(1), i
    return None, None


def parse_cargo_pkg_version(text):
    """TOML 里 **[package] 段**的 version ⇒ (值, 行号)；段内没有 ⇒ (None, None)。

    只在 [package] 段里取 —— 依赖行的版本（`serde = { version = "1" }`）不算；
    package 段的 version 缺失（或写到别的段里）⇒ 判据不可判（rc=2），不许回落猜。
    """
    section = None
    for i, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if line.startswith("[") and line.endswith("]"):
            section = line.strip("[]").strip()
            continue
        if section == "package":
            m = re.match(r'version\s*=\s*"([^"]+)"', line)
            if m:
                return m.group(1), i
    return None, None


def version_key(v):
    """版本号排序键（3 段数字；非 3 段给 (-1,) ），仅用于比较，不做语义判定。"""
    parts = v.split(".")
    try:
        return tuple(int(x) for x in parts)
    except ValueError:
        return (-1,)


# ─────────────────────────── 只报告：活文档旧版本号 ───────────────────────────

def derive_stale_set(root, canonical, use_all=False):
    """旧版集合 = `docs/项目文档/v*/` 里已知的 3 段版本号中 < 当前版者。

    默认只取**直接前驱版**（max of those < canonical）；use_all=True 时取全部。
    返回 (stale_list, known_list, note)。
    """
    d = os.path.join(root, "docs", "项目文档")
    known = []
    if os.path.isdir(d):
        for name in sorted(os.listdir(d)):
            if not name.startswith("v"):
                continue
            v = name[1:]
            if NUM_RE.fullmatch(v):
                known.append(v)
    if not canonical:
        return [], known, "当前版未知 ⇒ 旧版集合为空（只报告项，不进退出码）"
    older = sorted([v for v in set(known) if v != canonical and version_key(v) < version_key(canonical)],
                   key=version_key)
    if not older:
        return [], known, "没有历史版本目录可比 ⇒ 旧版集合为空（只报告项，不进退出码）"
    if use_all:
        return older, known, "旧版集合 = 全部历史版本（--stale-all）"
    return [older[-1]], known, "旧版集合 = 直接前驱版（默认；要收全部用 --stale-all）"


def is_excluded_zone(rel, canonical):
    """返回排除原因；None = 属活文档（要扫）。"""
    for pref, _why in DOC_EXCLUDES_FIXED:
        if rel.startswith(pref):
            return "固定排除域"
    if rel.startswith("docs/项目文档/"):
        rest = rel[len("docs/项目文档/"):]
        dirver = rest.split("/")[0]
        if dirver.startswith("v") and canonical and dirver[1:] != canonical:
            return "他版快照"
        if dirver.startswith("v") and dirver[1:] != canonical:
            return "他版快照"
    segs = rel.split("/")
    for s in segs[:-1]:
        if s.startswith("."):
            return "隐藏/工具目录"
    if segs[-1].startswith("."):
        return "隐藏文件"
    return None


def collect_corpus(root, canonical):
    """活文档语料：docs/**（排排除域）+ 仓根 *.md。返回 (files, skipped[{path,why}], excluded_dirs)。"""
    files, skipped, excluded_dirs = [], [], {}
    docs = os.path.join(root, "docs")
    if os.path.isdir(docs):
        for dirpath, dirnames, filenames in os.walk(docs):
            dirnames[:] = [d for d in dirnames if d not in SKIP_DIR_NAMES and not d.startswith(".")]
            for fn in sorted(filenames):
                p = os.path.join(dirpath, fn)
                rel = os.path.relpath(p, root).replace(os.sep, "/")
                why = is_excluded_zone(rel, canonical)
                if why:
                    excluded_dirs.setdefault(rel.split("/")[0] + "/" + (rel.split("/")[1] if len(rel.split("/")) > 2 else ""), 0)
                    excluded_dirs[rel.split("/")[0] + "/" + (rel.split("/")[1] if len(rel.split("/")) > 2 else "")] += 1
                    continue
                files.append(rel)
    if CORPUS_ROOT_MD:
        for fn in sorted(os.listdir(root)):
            if fn.endswith(".md") and os.path.isfile(os.path.join(root, fn)):
                files.append(fn)
    return sorted(set(files)), skipped, excluded_dirs


def classify_occurrence(line, m):
    """出现形态：REF = 版本名/路径形态（指的是旧版的名字）；BARE = 裸数字形态（需人判读）。"""
    before = line[:m.start()]
    after = line[m.end():]
    if before[-1:] in ("v", "V"):
        return "REF"
    if after[:1] == "/":
        return "REF"
    if before[-1:] == "/":
        return "REF"
    if after[:1] in ("-", "_", ".", "+"):
        return "REF"
    return "BARE"


def scan_live_docs(root, stale_set, canonical):
    """只报告：活文档里仍写着旧版本号字面量。返回统计 dict（本函数**不决定退出码**）。"""
    files, skipped, excluded = collect_corpus(root, canonical)
    hits = []
    scanned = 0
    for rel in files:
        p = os.path.join(root, rel)
        try:
            if os.path.getsize(p) > MAX_FILE_BYTES:
                skipped.append({"path": rel, "why": "超过 %d 字节" % MAX_FILE_BYTES})
                continue
        except OSError:
            skipped.append({"path": rel, "why": "取不到大小"})
            continue
        text, err = read_text(p)
        if err:
            skipped.append({"path": rel, "why": err})
            continue
        scanned += 1
        for i, line in enumerate((text or "").splitlines(), 1):
            for token in stale_set:
                for m in re.finditer(re.escape(token) + r"(?![\d.])", line):
                    hits.append({
                        "path": rel, "line": i, "stale": token,
                        "class": classify_occurrence(line, m),
                        "text": line.strip()[:200],
                    })
    totals = {
        "corpus_files": len(files), "scanned_files": scanned,
        "hit_files": len(set(h["path"] for h in hits)), "hits": len(hits),
        "bare": len([h for h in hits if h["class"] == "BARE"]),
        "ref": len([h for h in hits if h["class"] == "REF"]),
    }
    return {"stale_versions": list(stale_set), "totals": totals, "hits": hits,
            "skipped": skipped, "excluded_dirs": excluded}


# ─────────────────────────── 四处版本源断言（阻断档）───────────────────────────

def check_sources(root, stale_set=None, strict_docs=False):
    """跑一遍全部判据。返回 dict（含 rc / sources / problems / blockers / docs）。"""
    problems, blockers = [], []
    sources = []

    def need(path):
        """存在且可读 ⇒ 文本；否则登记 BLOCKED 并返回 None。"""
        ap = os.path.join(root, path)
        if not os.path.exists(ap):
            blockers.append("缺件：%s 不存在（判据不可判 ⇒ 不给结论）" % path)
            return None
        text, err = read_text(ap)
        if err:
            blockers.append("不可解析：%s %s" % (path, err))
            return None
        return text

    # ① 基准：version.go 的 const Version
    canonical = None
    txt_go = need(P_GO)
    if txt_go is not None:
        val, ln = parse_go_const(txt_go)
        if not val:
            blockers.append("不可解析：%s 里找不到 `const Version = \"X\"`（判据不可判）" % P_GO)
        else:
            canonical = val
        sources.append({"id": "go", "role": "基准（真源）", "path": P_GO, "line": ln,
                        "value": val, "ok": bool(val)})
    else:
        sources.append({"id": "go", "role": "基准（真源）", "path": P_GO, "line": None,
                        "value": None, "ok": False})

    # ② VERSION（指针）
    txt_ver = need(P_VERSION_FILE)
    if txt_ver is not None:
        nums = NUM_RE.findall(txt_ver)
        num_lines = [i for i, l in enumerate(txt_ver.splitlines(), 1) if NUM_RE.search(l)]
        ptr_missing = [p for p in VERSION_POINTERS if p not in txt_ver]
        if not txt_ver.strip():
            blockers.append("空转：%s 是空文件（既无数值也无指针 ⇒ 判据不可判）" % P_VERSION_FILE)
        if nums:
            problems.append("FAIL %s:%s 出现版本数值 %s —— 本文件已**指针化**，不许再写数值"
                            % (P_VERSION_FILE, ",".join(str(x) for x in num_lines), ", ".join(sorted(set(nums)))))
        if txt_ver.strip() and ptr_missing:
            problems.append("FAIL %s 的指针文本没点名真源：缺 %s（指针必须指出真源在哪）"
                            % (P_VERSION_FILE, " / ".join(ptr_missing)))
        sources.append({"id": "version_file", "role": "指针（非真源）", "path": P_VERSION_FILE,
                        "line": num_lines[0] if num_lines else None, "value": None,
                        "ok": not nums and not ptr_missing and bool(txt_ver.strip()),
                        "note": "无数值行=%s · 指向真源=%s" % (not nums, not ptr_missing)})
    else:
        sources.append({"id": "version_file", "role": "指针（非真源）", "path": P_VERSION_FILE,
                        "line": None, "value": None, "ok": False})

    # ③ Cargo.toml × 2（ui / wall）
    for sid, path, label in (("ui", P_UI, "UI 侧（egui 桌面端）"), ("wall", P_WALL, "茧壁 crate（升级比较用）")):
        txt = need(path)
        if txt is None:
            sources.append({"id": sid, "role": label, "path": path, "line": None, "value": None, "ok": False})
            continue
        val, ln = parse_cargo_pkg_version(txt)
        if not val:
            blockers.append("不可解析：%s 的 [package] 段里没有 version（判据不可判）" % path)
        sources.append({"id": sid, "role": label, "path": path, "line": ln, "value": val,
                        "ok": bool(val), "note": "取自 [package] 段"})

    # ④ 同版断言（除基准外，每一处都必须等于基准）
    if canonical:
        base_src = [s for s in sources if s["id"] == "go"][0]
        for s in sources:
            if s["id"] == "go" or not s["value"]:
                continue
            if s["value"] != canonical:
                problems.append("FAIL %s:%s version = %s ≠ 基准 %s（基准真源 = %s:%s 的 `const Version`）"
                                "—— 四处版本源（version.go / ui / wall）必须同版"
                                % (s["path"], s["line"], s["value"], canonical,
                                   base_src["path"], base_src["line"]))
                s["ok"] = False

    # ⑤ 只报告：活文档旧版本号（**不进退码**，除非 --strict-docs）
    docs = None
    if stale_set:
        docs = scan_live_docs(root, stale_set, canonical)
        t = docs["totals"]
        if t["scanned_files"] == 0:
            blockers.append("空转：活文档语料 0 个文件被扫到（旧版集合非空 ⇒ 判据不可判，不给结论）")
        if strict_docs and t["hits"]:
            problems.append("FAIL --strict-docs：活文档里仍有 %d 处旧版本号（%d 个文件）"
                            % (t["hits"], t["hit_files"]))

    rc = RC_OK
    if problems:
        rc = RC_FAIL
    elif blockers:
        rc = RC_BLOCKED
    return {"tool": TOOL, "tool_version": TOOL_VERSION, "root": os.path.abspath(root),
            "canonical": canonical, "sources": sources,
            "problems": problems, "blockers": blockers, "docs": docs, "rc": rc}


# ─────────────────────────── 输出 ───────────────────────────

def print_report(res, strict_docs):
    print("== 版本源一致性（门② %s %s）==" % (TOOL, TOOL_VERSION))
    print("仓库根: %s" % res["root"])
    print("基准（真源）: %s 里 const Version = \"%s\"" % (P_GO, res["canonical"] or "（未解出）"))
    print()
    for s in res["sources"]:
        mark = "✓" if s["ok"] else "✗"
        val = s["value"] if s["value"] is not None else "—"
        loc = ":%s" % s["line"] if s["line"] else ""
        extra = ("（%s）" % s["note"]) if s.get("note") else ""
        print("  %s  %-38s %-8s %s%s" % (mark, s["path"] + loc, val, s["role"], extra))
    print()
    if res["problems"]:
        print("问题（%d 条）：" % len(res["problems"]))
        for p in res["problems"]:
            print("  %s" % p)
    else:
        print("问题（0 条）：四处版本源同版 ✓")
    if res["blockers"]:
        print("不给结论（%d 条）：" % len(res["blockers"]))
        for b in res["blockers"]:
            print("  %s" % b)
    d = res["docs"]
    print()
    if not d:
        print("-- 只报告：活文档里的旧版本号字面量 —— 未扫（旧版集合为空）--")
    else:
        t = d["totals"]
        print("-- 只报告：活文档里的旧版本号字面量（旧版 = %s）--" % ", ".join(d["stale_versions"]))
        print("  语料: %d 个文件（活文档域）· 实扫 %d 个 · 跳过 %d 个"
              % (t["corpus_files"], t["scanned_files"], len(d["skipped"])))
        print("  命中: %d 个文件 / %d 处（裸数字 %d 处 · 版本名/路径形态 %d 处）"
              % (t["hit_files"], t["hits"], t["bare"], t["ref"]))
        print("  ★ 本项**只报告**：%s" % ("（--strict-docs 已开：命中计入 rc=1）" if strict_docs
                                          else "不计失败项、不影响退出码；判读由人做"))
        if d["hits"]:
            byfile = {}
            for h in d["hits"]:
                byfile.setdefault(h["path"], []).append(h)
            for path in sorted(byfile, key=lambda p: (-len([x for x in byfile[p] if x["class"] == "BARE"]),
                                                      -len(byfile[p]), p)):
                hs = byfile[path]
                bare = len([x for x in hs if x["class"] == "BARE"])
                print("    %s  %d 处（裸数字 %d）" % (path, len(hs), bare))
                for h in hs:
                    if h["class"] == "BARE":
                        print("       L%-5d [裸] %s" % (h["line"], h["text"]))
            print("    （版本名/路径形态的逐行清单用 --json 取；本表只展开需人判读的「裸数字」行）")
    print()
    print("结果: %s  rc=%d" % (RC_NAME[res["rc"]], res["rc"]))
    return res["rc"]


def print_rules(res_extra=None):
    print("== 规则表（%s %s）==" % (TOOL, TOOL_VERSION))
    print("""
[判据 · 阻断档]
  R1 四处同版   core/internal/version/version.go 的 `const Version = "X"` = **基准**；
               ui/Cargo.toml 与 wall/Cargo.toml 的 **[package] 段** version 必须等于基准；
               不等 ⇒ rc=1，逐条打印 `文件:行 值 ≠ 基准`（不含基准自身）
  R2 VERSION 无数值   仓根 VERSION 里出现任何 `X.Y.Z` 字面量 ⇒ rc=1（逐行点名）。
               理由：VERSION 已**指针化**（真源是 version.go + Cargo.toml），写数字就是第二份真相
  R3 VERSION 指真源   指针文本必须同时点名 `core/internal/version/version.go` 与 `ui/Cargo.toml`
               （判据是**路径串**，不是措辞；缺一 ⇒ rc=1）
  R4 缺件/不可解析   任一源缺失 · 不是 UTF-8 · version.go 无 `const Version` ·
               Cargo.toml 的 [package] 段无 version · VERSION 是空文件 ⇒ rc=2（**不给结论**）
  R5 空转防护   活文档语料 0 个文件被扫到（而旧版集合非空）⇒ rc=2（0 个可比对 ≠ 绿）

[只报告 · 不进退出码]
  D1 活文档旧版本号   语料域 = docs/**（走下方排除域）+ 仓根 *.md；
               旧版集合默认 = 直接前驱版（`--stale X.Y.Z` 可指定 / `--stale-all` 收全部历史版）；
               逐处分类：REF=版本名/路径形态（`v2.5.9`、`…/v2.5.9/`、`变更-v2.5.9.md`）·
               BARE=裸数字形态（`2.5.9 → 2.5.10`、`2.5.9 增补`）—— 后者才是需人判读的候选；
               命中只列清单。`--strict-docs` 可当场把命中计进 rc=1（加严开关，默认关）""")
    print("[活文档语料的排除域]")
    for pat, why in DOC_EXCLUDES_FIXED:
        print("  ✗ %-34s %s" % (pat, why))
    for pat, why in DOC_EXCLUDES_RULE:
        print("  %s %-34s %s" % ("✓" if u"不排除" in why else "✗", pat, why))
    print("  %s" % DOC_EXCLUDES_NOTE)
    if res_extra:
        d = res_extra
        print("\n[本仓现算]")
        print("  当前版（真源）= %s" % (d["canonical"] or "（未解出）"))
        print("  已知 3 段历史版本目录: %s" % (", ".join(d["known"]) or "（无）"))
        print("  旧版集合: %s   —— %s" % (", ".join(d["stale"]) or "（空）", d["stale_note"]))
    print("""
[退码映射（三档；与 scripts/precommit-gates.sh 的 tri 模式同口径）]
  0  全绿            四处同版 + VERSION 无数值 + 指针指真源
  1  有失败项        R1/R2/R3 任一红（或 --strict-docs 下的 D1 命中）
  2  **不给结论**    R4/R5（缺件/不可解析/空转/用法错）—— BLOCKED 不许当绿
  挂接口径：进 precommit-gates.sh 步骤表时用 `tri`（rc=2 记 BLOCKED、不计失败项）；
  若哪一天 D1 要转阻断档，**改判定档位、不改判据**（门脚本一字不动）。""")


# ─────────────────────────── 自检（成对负控）───────────────────────────

FIX_GO = '''package version

// Version 当前发布版（不带前缀）。
const Version = "%s"

const Tag = "v" + Version
'''
FIX_UI = '''[package]
name = "zerg-ui"
version = "%s"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
'''
FIX_WALL = '''[package]
name = "zerg-wall"
version = "%s"   # 锚到版本真源
edition = "2021"
'''
FIX_VERSION = u'''# 虫族 Zerg 版本号
# 本文件已非真源：真源见 core/internal/version/version.go 与 ui/Cargo.toml
'''
FIX_UI_NO_PKG_VERSION = '''[dependencies]
serde = { version = "%s", features = ["derive"] }
'''


def _mk_fixture(base, go_v, ui_v, wall_v, version_text):
    for d in ("core/internal/version", "ui", "wall", "docs/zh", "docs/项目文档/v9.9.8",
              "Zerg-内部文档/01-设计"):
        if not os.path.isdir(os.path.join(base, d)):
            os.makedirs(os.path.join(base, d))
    w = lambda rel, txt: io.open(os.path.join(base, rel), "w", encoding="utf-8").write(txt)
    w(P_GO, FIX_GO % go_v)
    w(P_UI, FIX_UI % ui_v)
    w(P_WALL, FIX_WALL % wall_v)
    w(P_VERSION_FILE, version_text)
    # 活文档（写着旧版号 ⇒ 只报告清单里必须出现）
    w("docs/zh/活文档.md", u"# 活文档\n当前版本 9.9.8 已发布。\n见 `docs/项目文档/v9.9.8/变更-v9.9.8.md`。\n")
    # 冻结节（他版快照 + 固定排除域 ⇒ 不入清单）
    w("docs/项目文档/v9.9.8/变更-v9.9.8.md", u"旧账：9.9.8 于昨日发布。\n")
    w("Zerg-内部文档/01-设计/旧文.md", u"旧设计（已分家到工作树之外）：9.9.8。\n")


def self_test():
    """成对负控：好件必绿 ↔ 每一处改一个数必红；缺件必 rc=2；只报告项不进退出码。"""
    base = tempfile.mkdtemp(prefix="check-version-sources-selftest-")
    cases = []          # (名, 期望rc, run(), 期望命中的判词子串)
    results = []

    def fresh():
        """重建夹具（好件；docs 里含活文档/冻结节）。"""
        for d in os.listdir(base):
            p = os.path.join(base, d)
            shutil.rmtree(p) if os.path.isdir(p) else os.remove(p)
        _mk_fixture(base, "9.9.9", "9.9.9", "9.9.9", FIX_VERSION)

    def patch(rel, old, new):
        p = os.path.join(base, rel)
        txt = io.open(p, encoding="utf-8").read()
        assert old in txt, "夹具锚点未命中：%s 里没有 %r" % (rel, old)
        io.open(p, "w", encoding="utf-8").write(txt.replace(old, new, 1))

    def rm(rel):
        os.remove(os.path.join(base, rel))

    def run(**kw):
        return check_sources(base, **kw)

    # ── 正控：好件必绿（且活文档命中只报告 ⇒ rc 仍 0）
    def c_good():
        fresh()
        return run(stale_set=["9.9.8"])
    cases.append(("正控① 四处同版 9.9.9 + VERSION 纯指针 ⇒ 必绿（活文档命中只报告）",
                  0, c_good, None))

    # ── 负控：每一处改一个数 ⇒ 必红（四处里的三处：version.go / ui / wall）＋ VERSION 数值行
    def c_go():
        fresh(); patch(P_GO, 'const Version = "9.9.9"', 'const Version = "9.9.8"')
        return run(stale_set=["9.9.8"])
    cases.append(("负控② version.go 改一个数 ⇒ 必红",
                  1, c_go, P_GO))

    def c_ui():
        fresh(); patch(P_UI, 'version = "9.9.9"\nedition', 'version = "9.9.8"\nedition')
        return run(stale_set=["9.9.8"])
    cases.append(("负控③ ui/Cargo.toml 改一个数 ⇒ 必红", 1, c_ui, P_UI))

    def c_wall():
        fresh(); patch(P_WALL, 'version = "9.9.9"', 'version = "9.9.8"')
        return run(stale_set=["9.9.8"])
    cases.append(("负控④ wall/Cargo.toml 改一个数 ⇒ 必红", 1, c_wall, P_WALL))

    def c_ver_num():
        fresh()
        io.open(os.path.join(base, P_VERSION_FILE), "a", encoding="utf-8").write(u"9.9.9\n")
        return run(stale_set=["9.9.8"])
    cases.append(("负控⑤ VERSION 里又出现数值行 ⇒ 必红", 1, c_ver_num, P_VERSION_FILE))

    def c_ver_ptr():
        fresh()
        io.open(os.path.join(base, P_VERSION_FILE), "w", encoding="utf-8").write(u"# 版本号见某处\n")
        return run(stale_set=["9.9.8"])
    cases.append(("负控⑥ VERSION 指针不再点名真源 ⇒ 必红", 1, c_ver_ptr, u"没点名真源"))

    # ── 缺件 ⇒ rc=2（不给结论）
    for rel, name in ((P_VERSION_FILE, u"负控⑦ 缺 VERSION ⇒ rc=2"),
                      (P_GO, u"负控⑧ 缺 version.go ⇒ rc=2"),
                      (P_UI, u"负控⑨ 缺 ui/Cargo.toml ⇒ rc=2"),
                      (P_WALL, u"负控⑩ 缺 wall/Cargo.toml ⇒ rc=2")):
        def mk(rel=rel):
            def f():
                fresh(); rm(rel)
                return run(stale_set=["9.9.8"])
            return f
        cases.append((name, 2, mk(), u"缺件"))

    def c_no_pkg_ver():
        fresh()
        io.open(os.path.join(base, P_UI), "w", encoding="utf-8").write(FIX_UI_NO_PKG_VERSION % "9.9.9")
        return run(stale_set=["9.9.8"])
    cases.append((u"负控⑪ Cargo.toml 的 [package] 段无 version（只有依赖行有）⇒ rc=2（不许回落猜）",
                  2, c_no_pkg_ver, u"不可解析"))

    def c_empty_version_file():
        fresh()
        io.open(os.path.join(base, P_VERSION_FILE), "w", encoding="utf-8").write(u"")
        return run(stale_set=["9.9.8"])
    cases.append((u"负控⑫ VERSION 是空文件 ⇒ rc=2（空转/不可判）", 2, c_empty_version_file, u"空文件"))

    def c_empty_corpus():
        fresh()
        shutil.rmtree(os.path.join(base, "docs"))
        return run(stale_set=["9.9.8"])
    cases.append((u"负控⑬ 活文档语料 0 个文件被扫到 ⇒ rc=2（空转不许当绿）",
                  2, c_empty_corpus, u"空转"))

    # ── 成对：同一夹具 + 只报告档 vs --strict-docs 加严档
    def c_docs_report_only():
        fresh()
        return run(stale_set=["9.9.8"])
    cases.append((u"成对⑭ 活文档写着旧版号 ⇒ 默认**只报告**（rc 仍 0，但清单里有它）",
                  0, c_docs_report_only, u"__DOC_HIT__"))

    def c_docs_strict():
        fresh()
        return run(stale_set=["9.9.8"], strict_docs=True)
    cases.append((u"成对⑮ 同一夹具 + --strict-docs ⇒ rc=1（加严档有牙齿）",
                  1, c_docs_strict, u"--strict-docs"))

    def c_excluded():
        fresh()
        return run(stale_set=["9.9.8"])
    cases.append((u"成对⑯ 冻结节/排除域里的旧版号**不入清单**（排除域生效）",
                  0, c_excluded, u"__EXCLUDE_ONLY__"))

    # 跑
    n_pass = 0
    for name, want_rc, fn, needle in cases:
        try:
            res = fn()
            got_rc = res["rc"]
            ok = (got_rc == want_rc)
            detail = u"rc=%d（期望 %d）" % (got_rc, want_rc)
            if ok and needle == u"__DOC_HIT__":
                paths = [h["path"] for h in (res["docs"]["hits"] if res["docs"] else [])]
                ok = u"docs/zh/活文档.md" in paths
                detail += u" · 清单含活文档=%s" % (u"docs/zh/活文档.md" in paths)
            elif ok and needle == u"__EXCLUDE_ONLY__":
                paths = [h["path"] for h in (res["docs"]["hits"] if res["docs"] else [])]
                # ★ 2026-09-19 分家批6：原「镜像副本」域退役时已删；原 `docs/02-调研/raw/` 与 `docs/issues/`
                #   两条随分家删除（零命中）⇒ **补牙**：本格改由新补的 `Zerg-内部文档/` 固定排除域（夹具里那份
                #   假 `Zerg-内部文档/01-设计/旧文.md`，写着旧版号）+ 「他版快照」规则把牙咬住（补牙 ≠ 放宽）。
                #   另加**防空转断言**：每个声明在 `DOC_EXCLUDES_FIXED` / 他版快照里的域，夹具必须真的有文件
                #   （域被删/改名而忘了改夹具 ⇒ 本格红，排除域判据不许悄悄变弱）。
                prefixes = [p for p, _why in DOC_EXCLUDES_FIXED] + [u"docs/项目文档/"]
                bad = [p for p in paths if any(p.startswith(x) for x in prefixes)]
                vac = []
                for pfx in prefixes:
                    n = 0
                    for _dp, _dns, _fns in os.walk(os.path.join(base, pfx)):
                        n += len(_fns)
                    if n == 0:
                        vac.append(pfx)
                ok = (not bad) and (u"docs/zh/活文档.md" in paths) and (not vac)
                detail += u" · 排除域零命中=%s · 夹具覆盖各排除域=%s%s" % (
                    not bad, not vac, (u"（空转域：%s）" % u", ".join(vac)) if vac else u"")
            elif ok and needle:
                blob = u"\n".join(res["problems"] + res["blockers"])
                ok = needle in blob
                detail += u" · 判词含「%s」=%s" % (needle, needle in blob)
            results.append({"case": name, "ok": bool(ok), "detail": detail, "want_rc": want_rc, "got_rc": got_rc})
            if ok:
                n_pass += 1
        except Exception as e:  # noqa: BLE001 —— 自检自身崩了也算不合格（要看得见）
            results.append({"case": name, "ok": False,
                            "detail": u"异常：%s: %s" % (type(e).__name__, e),
                            "want_rc": want_rc, "got_rc": None})
    return results, n_pass, len(cases), base


def print_self_test(json_out=False):
    try:
        results, n_pass, n_total, base = self_test()
    except Exception as e:  # noqa: BLE001 —— 夹具建不起来 ⇒ 不给结论
        print("✗ 自检跑不起来：%s: %s ⇒ rc=2（不给结论）" % (type(e).__name__, e))
        return RC_BLOCKED
    finally:
        pass
    if json_out:
        print(json.dumps({"tool": TOOL, "tool_version": TOOL_VERSION, "mode": "self-test",
                          "pass": n_pass, "total": n_total, "cases": results},
                         ensure_ascii=False, indent=2))
    else:
        print("== 成对负控自检（%s %s）==" % (TOOL, TOOL_VERSION))
        print("夹具：临时目录里的合成四源 + 活文档/冻结节（**不碰真仓**）\n")
        for r in results:
            print("  %s %s\n      %s" % ("PASS" if r["ok"] else "FAIL", r["case"], r["detail"]))
        greens = [r for r in results if r["want_rc"] == 0]
        reds = [r for r in results if r["want_rc"] != 0]
        print("\n  区分度：好件/只报告格 %d 个（期望绿）↔ 负控格 %d 个（期望非绿）—— "
              "好件绿而每个负控非绿才叫有牙齿" % (len(greens), len(reds)))
        print("\n结果: %s（%d/%d 条断言通过）  rc=%d"
              % (u"通过" if n_pass == n_total else u"不通过", n_pass, n_total,
                 0 if n_pass == n_total else 1))
    shutil.rmtree(base, ignore_errors=True)
    return RC_OK if n_pass == n_total else RC_FAIL


# ─────────────────────────── 入口 ───────────────────────────

def repo_root(arg=None):
    if arg:
        return os.path.abspath(arg)
    d = os.path.abspath(os.path.dirname(__file__))
    for _ in range(6):
        if os.path.isdir(os.path.join(d, "core")) and os.path.isdir(os.path.join(d, "ui")):
            return d
        d = os.path.dirname(d)
    return os.getcwd()


def main(argv):
    ap = argparse.ArgumentParser(add_help=False)
    ap.add_argument("root", nargs="?")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--strict-docs", action="store_true")
    ap.add_argument("--stale", action="append", default=[])
    ap.add_argument("--stale-all", action="store_true")
    ap.add_argument("-h", "--help", action="store_true")
    try:
        a = ap.parse_args(argv)
    except SystemExit:
        print("✗ 用法错（参数不认识）⇒ rc=2（不给结论）", file=sys.stderr)
        return RC_BLOCKED
    if a.help:
        print(__doc__)
        return RC_OK

    root = repo_root(a.root)

    stale, known, stale_note = derive_stale_set(root, None, a.stale_all)
    # 当前版要先解出来才能算默认旧版集合
    canonical = None
    txt_go = os.path.join(root, P_GO)
    if os.path.exists(txt_go):
        t, _ = read_text(txt_go)
        if t:
            canonical, _ln = parse_go_const(t)
    if a.stale:
        stale, stale_note = list(a.stale), "旧版集合由 --stale 指定"
    else:
        stale, known, stale_note = derive_stale_set(root, canonical, a.stale_all)

    if a.self_test:
        return print_self_test(json_out=a.json)

    if a.list_rules:
        print_rules({"canonical": canonical, "stale": stale, "known": known, "stale_note": stale_note})
        return RC_OK

    res = check_sources(root, stale_set=stale, strict_docs=a.strict_docs)
    res["stale_note"] = stale_note
    if a.json:
        print(json.dumps(res, ensure_ascii=False, indent=2))
        return res["rc"]
    return print_report(res, a.strict_docs)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

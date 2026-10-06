#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-empty-domain-guards.py —— 空集守卫元门（缺口 GAP-20260927-108）

病象（现读坐实 · 逐条见账）：
    域的边界是「空集」与「不存在」时，多道门输出满绿（静默 rc=0）——
    · `check_docs.py` 连不存在的路径都 rc=0 且打印满绿「配对完整/标题对齐/链接有效/源声明齐备」；
    · `check-placeholder-residue.py:74-80+199-202` 非 git 目录报「通过——未发现残留」；
    · `check-curl-timeout.py:138/150` 空 root 报「口径成立」（**同件两种行为**：缺件档 rc=2 而空 root 档 rc=0）；
    · `check-manifest-freshness.py:137-139` 空清单报「✓ 清单新鲜」；
    · `check-gotoolchain.py:163/177` trip() 为 None 则比较静默跳过；`check-build-tags.py:553-554` if not targets: return。
    根因同一：**扫描面取不到输入（空域）⇒ 判据整块消失 ⇒ 假绿**。

本件为**元门**（结构断言 + 成对自检 + baseline 棘轮只许下调），做四件事：
    ① 提供**规范空集守卫** `require_domain()`（Python 版）与 `BASH_GUARD_SNIPPET`（Bash 版）——
       空域 ⇒ 打印 BLOCKED 并回 rc=2；有件 ⇒ 回 rc=0。（现读 `code find require_domain` 由此命中。）
    ② **结构断言（域级 · GAP-20260927-108 首刀）**：扫 `scripts/gates/*.py`，凡**扫面型**件
       （含 os.walk/scandir/listdir/glob/rglob）必须在**判定出口前**带**域级空集守卫** ——
       ① `require_domain(<被扫集合>, …)` · ② 空集检查子句（`if not <集合>:` / `len(<集合>) <op> N` /
       `<集合> == 0`）**且其块内** fail-closed · ③ `空域|空集` **且近旁** fail-closed。
       ★ **✗ 不认文件级命中**：件里任一处 `len(x) >= N` / `is None` / `not isdir` 一律**不算守卫**
       （旧口径 `GUARD_RE` 是文件级 —— 实测 `check-build-tags.py:310` 的无关 `len(parts) >= 3`
       把 `:553 if not targets: return` 这个真洞洗成绿）⇒ 缺守卫者**逐条点名 rc=1**。
    ③ **baseline 棘轮**：现读缺守卫件数 <= BASELINE 才绿（**只许下调**；下调 = 补齐守卫）。
       绿档也照实打印「剩余 N 条」（✗ 不许悄悄绿 —— 同门① 的口径）。
    ④ **非扫面型 · 已人工复核（登记表 · ✗ 不扩本门扫面）**：6 门**结构上**不含
       os.walk/scandir/listdir/glob ⇒ `SCAN_RE` 扫不到（**非规则漏写、0 命中**）。
       它们的空域风险已由**件内各自 fail-closed** 处理（逐条理由见 `NON_SCAN_REVIEWED`）⇒
       本门输出**逐条明说**它们不在扫面范围、**不计入「缺守卫」数**（✗ 不以「缺守卫」名义
       造成误导）；**登记 ≠ 豁免**：登记项仍参与扫面，另加一致性断言
       （登记项现读为扫面型 ⇒ 判「登记与结构不符」rc=1）。
    附 **报告档**（✗ 不判红）：调用 subprocess.run/Popen 却不查 returncode / check= 的件数列数（口径④）。

口径（写死）：
    · 缺守卫件数 > baseline ⇒ rc=1（棘轮上调 = 口径变坏）；<= baseline ⇒ rc=0；
    · **登记与结构不符**（`NON_SCAN_REVIEWED` 里的件现读含扫面调用）⇒ rc=1（登记过期可见）；
    · **缺语料 / 扫描根读不到 ⇒ rc=2 不给结论**（✗ 空域不许当绿 —— 本件自己先守此线）；
    · **只读**：不写任何文件（`--emit-baseline` 只把现读数打到 stdout，✗ 不写盘）；
    · 结构断言是**域级启发式**（守卫＝空集检查＋出口前 fail-closed）：白名单 / baseline 是它的人工降噪口。

用法：
    python3 scripts/gates/check-empty-domain-guards.py [--root <目录>] [--baseline N] [--json]
    python3 scripts/gates/check-empty-domain-guards.py --self-test
    python3 scripts/gates/check-empty-domain-guards.py --emit-baseline     # 现读缺守卫数（只打印）

★ 接线状态：**已挂门90**（add_step gates · tri-report 只报告档）。
  2026-10-06 首刀（GAP-20260927-108 · 本波）：`GUARD_RE`（文件级）⇒ **域级**（空集检查＋判定出口前
  fail-closed）＋成对负控（档④′ 无关 `len(x) >= 3` · 档④″ 空集检查未 fail-closed）⇒
  现读真基线 **14**（旧读数 0 = 文件级假阳不可信态；`--emit-baseline` 现算 · ✗ 不用 0）。
  ★ 随动面（本文件外 · 另路/父代）：`precommit-gates.sh:3507` 门90 步名「缺 0 ≤基线 0」、
  `judge-registry.json` 缓存键（`--emit`）须随动。
  2026-10-04 现读更正：原「仓外候件 / 未挂进任何门步」措辞已过期（接线早已落）。
  2026-10-05 收口：补 ④ 非扫面型 · 已人工复核登记表（6 门）· ✗ 不扩本门扫面。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

# ── ① 规范空集守卫（Python 版）· 现读 `code find require_domain` 由此命中 ──────────
def require_domain(items, *, what, min_count=1, rc=2):
    """空集守卫：扫描面取不到输入（空域）时 fail-closed。

    用法：`n = require_domain(found, what="scripts/gates 扫面");` —— 回 0 表示域非空；
    回非 0（默认 2）表示 BLOCKED（调用方应 `return rc`，✗ 不得继续判绿）。
    `items` 可传序列（取 len）或整数（已是计数）。
    """
    n = items if isinstance(items, int) else len(items)
    if n < min_count:
        print("BLOCKED —— 空域 / 取不到输入：%s（现读 %d < 写死下限 %d）⇒ rc=%d 不给结论"
              % (what, n, min_count, rc), file=sys.stderr)
        return rc
    return 0


# Bash 版（②：无后缀语法/发布面脚本用）—— 文档常量，供被扫描件或人照抄：
BASH_GUARD_SNIPPET = r'''
require_domain() {  # require_domain <计数> <写死下限> <说明>
  local n="$1" min="${2:-1}" what="${3:-扫描面}"
  if [ "${n:-0}" -lt "${min}" ]; then
    echo "BLOCKED —— 空域 / 取不到输入：${what}（现读 ${n:-0} < 写死下限 ${min}）⇒ rc=2 不给结论" >&2
    return 2
  fi
  return 0
}
'''

# ── ② 结构断言的启发式 ────────────────────────────────────────────────────────
# 扫面型：会「按目录/模式取一批件」的调用
SCAN_RE = re.compile(
    r"os\.walk\s*\(|os\.scandir\s*\(|os\.listdir\s*\(|glob\.glob\s*\(|\.rglob\s*\(|\.glob\s*\("
)
# 域级空集守卫（GAP-20260927-108 首刀）—— ✗ 不认「文件级」命中：
#   件里任何一处 `len(x) >= N` / `is None` / `not os.path.isdir` 一律**不算守卫**
#   （那是与「被扫集合是否为空」脱钩的普通比较 —— 本门旧口径的病根）。
#   只认三族，且**每一族都必须落「判定出口前」**：
#     ① `require_domain(<被扫集合>, …)` 调用（✗ 排除 `def require_domain(` 本体行）；
#     ② **空集检查子句**：`if not <集合>:` / `if not len(<集合>):` /
#        `if len(<集合>) <op> N:`（op ∈ {<,<=,==}） / `if <集合> == 0:`；
#     ③ `空域` / `空集` 概念词。
#   ★ 「在判定出口前」＝ fail-closed 判决（rc=2 / 不给结论 / BLOCKED / 退码 2 / 空转 /
#     未取数 / EXIT_NOCONCL / EXIT_BLOCKED / RC_BLOCKED / sys.exit(2) / 判不了 / 找不到）
#     必须落在**该子句自己的缩进块里**（② 族）或**近旁 240 字内**（③ 族）；
#     ✗ 不认「同件别处恰好有个 return 2」（那是文件级假阳 · 实测 `check-build-tags.py` 的
#       `:310 len(parts) >= 3` 曾把 `:553 if not targets: return` 这个真洞洗成绿）。
REQUIRE_DOMAIN_RE = re.compile(r"require_domain\s*\(")
EMPTY_WORD_RE = re.compile(r"空域|空集")
DEF_TAIL_RE = re.compile(r"def\s+$")
FAILCLOSED_RE = re.compile(
    r"return\s+2\b|rc\s*=?\s*2\b|不给结论|BLOCKED|空转|fail[- ]?closed|未取数|"
    r"sys\.exit\(\s*2|退码\s*2|EXIT_NOCONCL|EXIT_BLOCKED|RC_BLOCKED|判不了|找不到"
)
# 空集检查子句（② 族）：捕获那个「集合表达式」（含 下标 / 属性：`res["scanned"]` / `self.x`）
_CE = r"([A-Za-z_][\w.]*(?:\[[^\]\n]*\])?)"
GUARD_EMPTY_RES = (
    re.compile(r"if\s+not\s+" + _CE + r"\s*:"),
    re.compile(r"if\s+not\s+len\s*\(\s*" + _CE + r"\s*\)\s*:"),
    re.compile(r"if\s+len\s*\(\s*" + _CE + r"\s*\)\s*(?:<|<=|==)\s*\d+\s*:"),
    re.compile(r"if\s+" + _CE + r"\s*==\s*0\b\s*:"),
)


def _clause_block_failclosed(text, colon_idx, if_indent, span=2000):
    """`if …:` 的**直接块**（到下一个缩进 <= if_indent 的非空行止）**本级**有无 fail-closed。

    ★ 只认**直接块本级**（缩进 == 块首缩进）的 fail-closed 判决 ——
      ✗ 不认嵌套子 `if` 里的 `return 2`（那不是「这个空集检查」的判定出口；
        否则 `if not args['no_self']:` ⇒ 内层 `if rc != 0: return 2` 会把无关件洗成绿）。
    """
    lines = text[colon_idx:colon_idx + span].split("\n")
    block = []  # [(indent, text)]
    for j, ln in enumerate(lines):
        if j == 0:
            if ln.strip():
                block.append((if_indent + 1, ln))
            continue
        if ln.strip() == "":
            continue
        ind = len(ln) - len(ln.lstrip(" \t"))
        if ind <= if_indent:
            break
        block.append((ind, ln))
    if not block:
        return False
    direct = min(ind for ind, _ in block)
    seg = "\n".join(t for ind, t in block if ind == direct)
    if FAILCLOSED_RE.search(seg):
        return True
    # 多行**字典空转档**：`return {\n "rc": 2, …}`（`"rc": 2` 只出现在 fail-closed 空转档里）
    whole = "\n".join(t for _, t in block)
    return bool(re.search(r"[\"']rc[\"']?\s*:\s*2\b", whole))


def has_domain_guard(text):
    """回 True ⇔ 件里含**域级空集守卫**（三族之一，且落「判定出口前」）。"""
    for m in REQUIRE_DOMAIN_RE.finditer(text):
        if not DEF_TAIL_RE.search(text[:m.start()]):  # ✗ 排除 `def require_domain(` 本体
            return True
    for m in EMPTY_WORD_RE.finditer(text):
        if FAILCLOSED_RE.search(text[m.start():m.start() + 240]):
            return True
    for rx in GUARD_EMPTY_RES:
        for m in rx.finditer(text):
            line_start = text.rfind("\n", 0, m.start()) + 1
            if_indent = len(text[line_start:m.start()]) - len(text[line_start:m.start()].lstrip(" \t"))
            colon = text.find(":", m.end() - 1)
            if colon != -1 and _clause_block_failclosed(text, colon + 1, if_indent):
                return True
    return False

# 附报告档（✗ 不判红）：subprocess 调用与 returncode 检查
SUBPROC_RE = re.compile(r"subprocess\.(?:run|Popen)\s*\(")
RCCHECK_RE = re.compile(r"returncode|check\s*=\s*True")

EXTS = (".py",)
# ★ baseline 棘轮（只许下调）：现读缺守卫件数，首测值见下方常量
BASELINE = 14  # 棘轮：2026-10-06 域级口径首刀后 `--emit-baseline` 现算真值（旧 0 = 文件级假阳不可信态）。
               # ✗ 只许下调（补齐守卫即下调）；上调 = 放宽口径 ⇒ 须走拍板。空域候选见 --baseline 0 现读名单。
SELF_PATH = os.path.abspath(__file__)

# ── ④ 非扫面型 · 已人工复核（登记表 · ✗ 不扩本门扫面 · GAP-20260927-108 尾批）──────────
#   口径先例（**照它的形**，✗ 不新造）：同目录 `check-links-lychee.py` / `check-docs-orphans-links.py`
#     的「已登记假红（基准偏移 · 已复核）· **不计入** <计数>（**逐条印出落点，看得见**）」——
#     即：登记项**逐条印出** ＋ 写明**复核依据** ＋ **不计入**那个会被误读的计数。
#   适用面：6 门**结构上**不含 os.walk/scandir/listdir/glob ⇒ `SCAN_RE` **0 命中**（✗ 非规则漏写），
#     其空域风险由**件内各自 fail-closed** 处理 ⇒ **不归本元门扫面面管**。
#   ★ 登记 ≠ 豁免：登记项**仍**参与扫面（若它哪天变成扫面型且缺守卫，照旧点名）；
#     另加「登记与结构不符」（登记项现读含扫面调用）一致性断言 ⇒ 登记过期当场可见（rc=1 · 只报告档）。
#   ✗ 本表**不是**白名单豁免口 —— 它只声明「这些件不在本门扫面范围」，不改任何判绿条件。
NON_SCAN_REVIEWED = (
    ("check-placeholder-residue.py",
     "已 fail-closed：scan() 取不到（git grep 退码 ≥2）⇒ rc=2，不再把「取不到」读成「零命中」（GAP-20260927-105）"),
    ("check-manifest-freshness.py",
     "已 fail-closed：judge() 取不到树 ⇒ EXIT_BLOCKED（rc=2），不与「无告警」混（GAP-20260927-105 同族）"),
    ("check-gotoolchain.py",
     "已补件内 fail-closed：trip() 取不到可解析版本 ⇒ rc=2，不静默跳过比较（GAP-20260927-108）"),
    ("check-doc-section-numbers.py",
     "已 fail-closed：缺件 / 读不到 ⇒ rc=2 不给结论"),
    ("check-public-tree-hazards.py",
     "已 fail-closed：域取不到（git rev-list 非零退出）⇒ rc=2 不给结论"),
    ("check_version.py",
     "已补件内 fail-closed：版本真源取不到（None）⇒ rc=2，不与「两处不一致」混（GAP-20260927-108）"),
)
NON_SCAN_REVIEWED_NAMES = tuple(n for n, _ in NON_SCAN_REVIEWED)


def scan_files(root):
    files = []
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "__pycache__")]
        for fn in sorted(fns):
            if fn.endswith(EXTS):
                files.append(os.path.join(dp, fn))
    return sorted(files)


def structural_scan(root):
    """回 (scanned, unguarded[name...], subproc_unchecked) 或 (None, None, None)。"""
    if not os.path.isdir(root):
        return None, None, None
    scanned, unguarded, subproc_unchecked = 0, [], 0
    for p in scan_files(root):
        if os.path.abspath(p) == SELF_PATH:  # 本件自己定义 require_domain ⇒ 不参与
            continue
        try:
            text = open(p, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        scanned += 1
        if SCAN_RE.search(text) and not has_domain_guard(text):
            unguarded.append(os.path.relpath(p, root))
        if SUBPROC_RE.search(text) and not RCCHECK_RE.search(text):
            subproc_unchecked += 1
    return scanned, unguarded, subproc_unchecked


def registry_stale(root):
    """④ 登记表一致性：回**现读为扫面型**的登记项名（登记过期 ⇒ rc=1）。

    ✗ 不查「登记项不在盘」—— `--root` 可指合成根（自检用）时登记项本就不在场，属正常，
    不许据此判红（自检必须 hermetic · 不依赖运行期在场物）。
    """
    stale = []
    for name in NON_SCAN_REVIEWED_NAMES:
        p = os.path.join(root, name)
        if not os.path.isfile(p):
            continue
        try:
            text = open(p, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        if SCAN_RE.search(text):
            stale.append(name)
    return stale


def run(root, baseline, as_json=False, emit_baseline=False):
    scanned, unguarded, subproc_unchecked = structural_scan(root)
    if scanned is None:
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % root, file=sys.stderr)
        return 2

    # ★ 空集守卫（本门自身 · 复用本件定义的 require_domain · GAP-20260927-108 收口）：
    #   扫描面 0 件（空域）⇒ 判不了「缺守卫几条」⇒ ✗ 不许把「空转」当绿（本件自己先守此线）。
    if require_domain(scanned, what="扫面根 %s 下的 .py 件" % root, min_count=1) != 0:
        return 2

    live = len(unguarded)
    if emit_baseline:
        print(live)
        return 0

    stale = registry_stale(root)
    rc = 1 if (live > baseline or stale) else 0
    if as_json:
        print(json.dumps({"schema": "zerg/v1", "kind": "EmptyDomainGuards", "root": root,
                          "scanned": scanned, "baseline": baseline,
                          "unguarded": live, "unguarded_files": unguarded,
                          "subproc_unchecked": subproc_unchecked,
                          "non_scan_reviewed": list(NON_SCAN_REVIEWED_NAMES),
                          "non_scan_reviewed_count": len(NON_SCAN_REVIEWED_NAMES),
                          "registry_stale": stale}, ensure_ascii=False))
        return rc

    print("扫描根：%s" % root)
    print("扫面型件 %d 条 · **缺空集守卫 %d 条**（baseline=%d · 只许下调）· 附〔subprocess 未查 returncode %d 条（只报告）〕"
          % (scanned, live, baseline, subproc_unchecked))
    if unguarded:
        for f in unguarded:
            print("  ✗ %s —— 扫面型但无「域级空集守卫」（空集检查＋出口前 fail-closed · 空域会静默 rc=0）" % f)
    # ── ④ 非扫面型 · 已人工复核（登记表 · 逐条印出 · ✗ 不计入上面「缺守卫」数）──
    print("非扫面型 · 已人工复核 %d 门（✗ 不在本门扫面范围：结构性不含 os.walk/scandir/listdir/glob "
          "⇒ SCAN_RE 0 命中；空域风险由件内各自 fail-closed 处理，**不计入上面「缺守卫」数**）："
          % len(NON_SCAN_REVIEWED))
    for name, reason in NON_SCAN_REVIEWED:
        mark = "✗" if name in stale else "✓"
        print("  %s %s —— %s" % (mark, name, reason))
    if stale:
        print("  ⇒ 登记与结构不符（登记项现读为扫面型 ⇒ 登记过期，登记面须随动）：%s" % "、".join(stale))

    if live > baseline:
        print("⇒ 缺守卫 %d > baseline %d ⇒ rc=1（棘轮上调 = 口径变坏）" % (live, baseline))
    elif stale:
        print("⇒ 登记与结构不符 %d 门 ⇒ rc=1（登记过期，登记面须随动）" % len(stale))
    else:
        print("⇒ 缺守卫 %d <= baseline %d ⇒ rc=0（仍注明剩余 %d 条，✗ 不许悄悄绿）"
              % (live, baseline, live))
    return rc


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """六档：守卫正控/负控 · 元门正控/负控 · 缺语料 · 登记表（登记与结构不符）。"""
    bad = [0]
    print("── check-empty-domain-guards.py --self-test 九档（正控 0 · 负控 1/2 · 缺语料/空域 2 · 登记 1）")
    tmp = tempfile.mkdtemp(prefix="emptydomain-selftest-")
    try:
        # ① 守卫正控：有件 ⇒ 0
        r0 = require_domain(["a", "b"], what="合成·有件")
        _st_chk(bad, "档① require_domain 正控：有件 ⇒ rc=0", 0, r0, "")
        # ② 守卫负控：空域 ⇒ 2
        r2 = require_domain([], what="合成·空域")
        _st_chk(bad, "档② require_domain 负控：空域 ⇒ rc=2", 2, r2, "")

        def put(sub, name, body):
            d = os.path.join(tmp, sub)
            os.makedirs(d, exist_ok=True)
            with open(os.path.join(d, name), "w", encoding="utf-8") as f:
                f.write(body)
            return d

        # ③ 元门正控：扫面型 + 有守卫 ⇒ 不点名 ⇒ rc=0
        d_ok = put("ok", "guarded.py",
                   "import os\n"
                   "def main():\n"
                   "    found = []\n"
                   "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                   "    if require_domain(found, what='x', min_count=1):\n        return 2\n"
                   "    return 0\n")
        # ④ 元门负控：扫面型 + 无守卫 ⇒ 点名 ⇒ rc=1
        d_bad = put("bad", "unguarded.py",
                    "import os\n"
                    "def main():\n"
                    "    found = []\n"
                    "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                    "    return 0\n")
        # ⑥ 登记表（④ 非扫面型 · 已人工复核）：登记项现读为扫面型 ⇒ 判「登记与结构不符」rc=1，
        #    同时证明登记项**不被豁免扫面**（它照旧进缺守卫名单）。
        d_stale = put("regstale", "check_version.py",
                      "import os\n"
                      "def main():\n"
                      "    found = []\n"
                      "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                      "    return 0\n")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档③ 元门正控：扫面型件带守卫 ⇒ rc=0", 0, *run("--root", d_ok, "--baseline", "0"))
        _st_chk(bad, "档④ 元门负控：扫面型件缺守卫 ⇒ rc=1（点名）", 1, *run("--root", d_bad, "--baseline", "0"))
        # ④′ 域级负控：扫面型 + **只**有无关于 `len(x) >= 3` ⇒ rc=1（不靠无关比较洗绿 · 病根标本）
        d_unrel = put("unrel", "unrelated_len.py",
                      "import os\n"
                      "def main():\n"
                      "    found = []\n"
                      "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                      "    parts = 'a_b_c'.split('_')\n"
                      "    if len(parts) >= 3:\n        return 0\n"
                      "    return 0\n")
        _st_chk(bad, "档④′ 域级负控：扫面型 + 只含无关 `len(x) >= 3` ⇒ rc=1（不靠无关比较洗绿）", 1,
                *run("--root", d_unrel, "--baseline", "0"))
        # ④″ 域级负控：扫面型 + 有空集检查但**未 fail-closed**（`if not found: return 0`）⇒ rc=1
        d_nofc = put("nofc", "empty_no_fc.py",
                     "import os\n"
                     "def main():\n"
                     "    found = []\n"
                     "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                     "    if not found:\n        return 0\n"
                     "    return 0\n")
        _st_chk(bad, "档④″ 域级负控：扫面型 + 空集检查但未 fail-closed（`if not found: return 0`）⇒ rc=1", 1,
                *run("--root", d_nofc, "--baseline", "0"))
        _st_chk(bad, "档⑤ 缺语料：`--root` 不存在 ⇒ rc=2（不给结论）", 2,
                *run("--root", os.path.join(tmp, "no-such-root")))
        # ⑤′ 空域（GAP-20260927-108 收口 · 本门自身守此线）：`--root` 是**空目录**（存在但 0 件）⇒ rc=2（空转 ≠ 绿）
        d_empty = os.path.join(tmp, "empty-root")
        os.makedirs(d_empty, exist_ok=True)
        _st_chk(bad, "档⑤′ 空域：`--root` 空目录（0 件）⇒ rc=2（空转 ≠ 绿）", 2,
                *run("--root", d_empty, "--baseline", "0"))
        rc6, out6 = run("--root", d_stale, "--baseline", "0")
        _st_chk(bad, "档⑥ 登记表：登记项现读为扫面型 ⇒ rc=1（登记与结构不符 · 未被豁免扫面）", 1, rc6, out6)
        _st_chk(bad, "档⑥′ 登记表：报话明说「非扫面型 · 已人工复核」且点名「登记与结构不符」", 0,
                0 if ("非扫面型 · 已人工复核" in out6 and "登记与结构不符" in out6) else 1, out6)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


# 自检里 require_domain 需在模块作用域可见（本文件即如此）；此处引用防「未用」误报：
_ = BASH_GUARD_SNIPPET


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=os.path.join(
        os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__)))),
        "scripts", "gates"))
    ap.add_argument("--baseline", type=int, default=BASELINE)
    ap.add_argument("--emit-baseline", action="store_true")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    return run(a.root, a.baseline, a.json, a.emit_baseline)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-publish-layer-names.py —— 「CLI 层名闭集 ⟷ 真源层名口径」对拍（缺口 `GAP-20260927-412`）

为什么有这个件（缺口本身）
--------------------------
  `zerg publish set` 的**层名闭集**（`layer` 那一格吐得出哪些值）此前**只在 Go 侧写死一处**
  （`core/cmd/zerg/family_publish_set.go` 的 `publishSetLayer*` 常量块），而**真源侧**另有一套
  层名口径（`publish/mirror-public-lib.py:942 SKIP_LABELS` = 只读预演「跳过 M 件」逐层分类计数的
  **同一批标签**）。两侧此前**零对拍** ⇒ 真源增/删/改一个层名时，CLI 会**静默落后**：
  · 真源加一层 ⇒ CLI 永远答不出那一层（`publish set` 的「哪一层拦的」少一格，而没有任何门会响）；
  · CLI 自造一个真源没有的层名 ⇒ 人面出现一个「按什么算的」讲不出来的层名，同样无人管。

本件是那一面镜子（与 `check-publish-face-sync.py` **分工不同**：那一件对的是
  「白名单选中的路径 × 两器排除口径 × 私有面黑名单」的**逐件**漂移，本件对的是**层名闭集本身**）。

判据（四条 · 全部现读 · 判据里没有一个层名是自填的）
----------------------------------------------------
  L1 **真源层名闭集**（现读 `publish/mirror-public-lib.py`）：模块级赋值 `SKIP_LABELS`（`ast` 解析，
     非正则糊）⇒ 真源层名的**唯一真源**。另判**真源自洽**两条（防「加了层名却没接线」）：
       · `Assets.exclude_kind` 函数体里 `return "…"` 的字符串字面量 ⊆ `SKIP_LABELS`
       · `list_only_report` 里 `skip["…"]` 的下标字面量 ⊆ `SKIP_LABELS`
  L2 **CLI 层名闭集**（现读 `core/cmd/zerg/family_publish_set.go`）：`publishSetLayer<名> = "…"`
     常量块的字符串字面量 ⇒ CLI 声明的闭集。
  L3 **对拍**（双向·逐字）：
       甲（真源 ⟶ CLI）：∀S ∈ 真源闭集，∃C ∈ CLI 闭集 使 `C == S` 或 `C.startswith(S + "（")`
          （唯一允许的形态：CLI 在真源名后**加括注限定** —— 现读那一例就是 `非 blob（子模块 gitlink）`）。
          缺 ⇒ **红**（真源增层 / 改层名 ⇒ CLI 落后）。
       乙（CLI ⟶ 真源）：∀C ∈ CLI 闭集，命中上式 **或** 在 `CLI_ONLY` 表里（**逐条带锚点**：
          件 + 逐字锚 + 为什么真源侧没有同名层）。两样都不是 ⇒ **红**（CLI 自造层名）。
          `CLI_ONLY` 的锚点是**现读校验**的：锚那串字在真源件里找不着 ⇒ 红（锚漂了 = 白名单腐化）。
  L4 **运行面 ⟷ 声明面**（现跑真子进程）：拿**探针集**（`PROBES`，逐条列出「这一件该命中哪一层」）
     跑 `bin/zerg publish set <探针…> --json path,caliber,will_publish,layer,non_blob_basis`，
     观测集 = 各件 `layer` 值（`—` 除外）∪ `non_blob_basis` 里出现的闭集名 ⇒ 要求
     **观测集 == CLI 闭集**（双向差集皆空）。这一条治的是「声明了一套、真跑吐另一套」。
     ★ 探针集的鉴别力由 `PROBES` 自己保证（逐条给期望层名）：任一探针**没打出它期望的那一层**
       ⇒ 红（探针退化 ⇒ 本判据空转）。

不假装覆盖（照实登记）
----------------------
  · 「层名」= `layer` 那一格的**闭集值**；各层的**判定实现**（哪一件该被哪一层拦）不在本件
    （那是 `check-publish-face-sync.py` / `cli_publish_set_test.go` 的面）。
  · 本件只读四个源 + 现跑一个只读档（`publish set … --json`，**不写盘**）；不碰真源两器、不改任何件。

退码
----
  0 = 四条都合 · 1 = 判红（真源增层 / CLI 自造层名 / 真源不自洽 / 运行面与声明面不符 / 探针退化）
  2 = **不给结论**（真源件或 CLI 件不在盘 · `SKIP_LABELS` / 常量块 / 探针解析不出 · `bin/zerg` 拿不到）

用法
----
    python3 scripts/gates/check-publish-layer-names.py
    python3 scripts/gates/check-publish-layer-names.py --repo-root DIR --bin FILE
    python3 scripts/gates/check-publish-layer-names.py --no-live     # 只跑 L1–L3（仓外夹具用）
    python3 scripts/gates/check-publish-layer-names.py --json
    python3 scripts/gates/check-publish-layer-names.py --list-rules
    python3 scripts/gates/check-publish-layer-names.py --self-test   # 合成夹具（正控 + 成对负控）
"""

from __future__ import annotations

import argparse
import ast
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

VERSION = "1.0.0"

LIB_REL = os.path.join("publish", "mirror-public-lib.py")
CLI_REL = os.path.join("core", "cmd", "zerg", "family_publish_set.go")

TRUE_LABELS_NAME = "SKIP_LABELS"

# CLI 层名常量的取法：`publishSetLayer<名> = "逐字层名"`（const 块里的声明行）。
# ★ 「不判层名」的**哨兵**（`publishSetLayerNone = "—"`）**不算层名** —— 它是「没命中任何层」那一格的值，
#   由 `publishSetLayerNone` 这个名字**现读**识别（不靠抄值），另单判它与本件 `LAYER_NONE` 同值。
CLI_LAYER_RE = re.compile(r'^\s*(publishSetLayer[A-Za-z]+)\s*=\s*"([^"]+)"\s*$', re.M)
CLI_LAYER_SENTINEL_ID = "publishSetLayerNone"

# ── CLI_ONLY：CLI 侧**有意**存在、真源侧**没有同名层**的层名（逐条带锚点 · 锚点现读校验）──
#   形态：(CLI 层名, 锚点件(仓相对), 锚点逐字, 理由)
#   ★ 锚点必须**现读**在真源件里找得到 —— 锚漂了（真源那句话被删/改）⇒ 红，防「白名单腐化」。
#   ★ 这张表**不是**「CLI 可以随便加层名」的口子：加一条要能指出真源里对应的那处处置。
CLI_ONLY = (
    ("白名单未命中", LIB_REL, "# 不在白名单",
     "真源只读预演对「不在白名单」的件是**静默 continue**（既不「会发」也不计入「跳过」的计数）"
     "⇒ 真源侧**没有**这一层的层名；CLI 的 `publish set` 答的是「这件**会不会**发 + 哪一层拦的」"
     "⇒ 必须给「白名单未命中」一个可点的层名（否则「不发」这一格答不出拦在哪里）。锚点 = 真源那句 continue 的注释。"),
    ("私有面硬门禁", LIB_REL, "私有面门禁：镜像树出现私有路径（硬门禁）",
     "真源侧的处置点 = `Assets.private_face_hit`（`publish/private-paths.txt`）+ 真跑原位 `:1225-1227` 的"
     "**整批中止**（那份清单是硬门禁 ⇒ 优先级最高）；但真源的**层名口径**里没有它"
     "（`SKIP_LABELS` 那五格是「跳过」分类，硬门禁是**中止**不是跳过）⇒ CLI 单立一个层名。"
     "锚点 = 真源真跑那一处的中止文字。"),
)

# ── 探针集（逐条给「这一件该命中哪一层」· 现读实证）──────────────────────────────────
#   取法：拿真仓现跑的 `zerg publish set <件> --json path,layer` 逐条读过（不是推的）。
#   ★ 6 个可判层各一枚 + 1 枚「会发」正控 —— `非 blob（子模块 gitlink）` 那层**免树口径不判**
#     （真源：git 树项模式 `TEXT_MODES`），它的名字只出现在 `non_blob_basis` 那一格里（L4 那半条腿覆盖）。
PROBES = (
    ("README-private.md", "白名单未命中"),
    ("scripts/公开标记.tsv", "EXCLUDES"),
    ("publish/mirror-verify-tree.py", "DROP_EXACT"),
    ("core/zerg-agent2", "DROP_PREFIX"),
    ("publish/mirror-allow.txt", "映射丢弃"),
    ("publish/replace-rules.tsv", "私有面硬门禁"),
)
# 正控（「会发」那一件）：`layer` 必须是 `—`（不命中任何层）
PROBE_EMIT = "core/cmd/zerg/family_publish_set.go"
LAYER_NONE = "—"


# ── 现读两侧（都不是判据，只负责取数）────────────────────────────────────────────
def read_true_labels(path):
    """真源层名闭集：`SKIP_LABELS` 的 `ast` 现读 ⇒ ([层名…], 说明)。"""
    if not os.path.isfile(path):
        return None, "真源件不在盘：%s" % path
    try:
        src = open(path, encoding="utf-8").read()
    except OSError as e:
        return None, "真源件读不回来（%s）" % e
    try:
        tree = ast.parse(src)
    except SyntaxError as e:
        return None, "真源件语法不合 ⇒ 解析不出（%s）" % e
    for node in tree.body:
        if isinstance(node, ast.Assign):
            for t in node.targets:
                if isinstance(t, ast.Name) and t.id == TRUE_LABELS_NAME:
                    if not isinstance(node.value, ast.Tuple):
                        return None, "%s 不是元组（真源换了形态 ⇒ 本件拒绝猜）" % TRUE_LABELS_NAME
                    out = []
                    for elt in node.value.elts:
                        if not isinstance(elt, ast.Constant) or not isinstance(elt.value, str):
                            return None, "%s 里有非字符串成员（真源换了形态 ⇒ 本件拒绝猜）" % TRUE_LABELS_NAME
                        out.append(elt.value)
                    if not out:
                        return None, "%s 是空元组（真源层名闭集为空 = 无区分度 ⇒ 不给结论）" % TRUE_LABELS_NAME
                    return out, ""
    return None, "真源件里找不到模块级赋值 %s（层名口径的单一真源没了）" % TRUE_LABELS_NAME


def read_true_usage(path):
    """真源自洽取数 ⇒ (exclude_kind 的 return 字面量集, list_only_report 的 skip[字面量] 集, 说明)。

    为什么要有这一条：层名光在 `SKIP_LABELS` 里躺着不算接线 —— 分类口（`exclude_kind`）与计数口
    （`skip[…"]`）**必须**都只吐闭集里的名字，否则真源自己就先漂了。
    """
    if not os.path.isfile(path):
        return None, None, "真源件不在盘：%s" % path
    tree = ast.parse(open(path, encoding="utf-8").read())
    returns, subscripts = set(), set()
    for node in ast.walk(tree):
        if isinstance(node, ast.FunctionDef) and node.name == "exclude_kind":
            for sub in ast.walk(node):
                if isinstance(sub, ast.Return) and isinstance(sub.value, ast.Constant) \
                        and isinstance(sub.value.value, str):
                    returns.add(sub.value.value)
        if isinstance(node, ast.FunctionDef) and node.name == "list_only_report":
            for sub in ast.walk(node):
                if isinstance(sub, ast.Subscript) and isinstance(sub.value, ast.Name) \
                        and sub.value.id == "skip" and isinstance(sub.slice, ast.Constant) \
                        and isinstance(sub.slice.value, str):
                    subscripts.add(sub.slice.value)
    return returns, subscripts, ""


def read_cli_sentinel(path):
    """「没命中任何层」那一格的值（`publishSetLayerNone` 的逐字值）⇒ (值 | None)。"""
    if not os.path.isfile(path):
        return None
    for ident, val in CLI_LAYER_RE.findall(open(path, encoding="utf-8").read()):
        if ident == CLI_LAYER_SENTINEL_ID:
            return val
    return None


def read_cli_layers(path):
    """CLI 层名闭集 ⇒ ([层名…], 说明)。取法 = 常量块逐行（声明即闭集）。

    ★ 哨兵（`publishSetLayerNone`）**不算层名**：它是「没命中任何层」那一格的**值** ——
      本函数把它单列返回，由 `main` 判它与 `LAYER_NONE` 同值（防「哨兵被人改成某个层名」）。
    """
    if not os.path.isfile(path):
        return None, "CLI 件不在盘：%s" % path
    try:
        src = open(path, encoding="utf-8").read()
    except OSError as e:
        return None, "CLI 件读不回来（%s）" % e
    out, sentinel = [], None
    for ident, val in CLI_LAYER_RE.findall(src):
        if ident == CLI_LAYER_SENTINEL_ID:
            sentinel = val
            continue
        out.append(val)
    if not out:
        return None, "CLI 件里一条 `publishSetLayer<名> = \"…\"` 都没解析出来（形态换了 ⇒ 不给结论）"
    if sentinel is None:
        return None, "CLI 件里没有哨兵常量 `%s`（「没命中任何层」那一格的值取不到 ⇒ 不给结论）" \
                     % CLI_LAYER_SENTINEL_ID
    return out, ""


def observed_from_live(items_doc, cli_layers):
    """运行面观测集 = 各件 `layer` 值 ∪ `non_blob_basis` 里出现的闭集名。"""
    seen = set()
    per = {}
    for it in items_doc:
        rel, layer = it.get("path"), it.get("layer") or ""
        per[rel] = layer
        if layer and layer != LAYER_NONE:
            seen.add(layer)
        basis = it.get("non_blob_basis") or ""
        for name in cli_layers:
            if name in basis:
                seen.add(name)
    return seen, per


# ── 判据本体（**唯一一处** · 纯函数：不吃盘、不起进程）────────────────────────────────
def judge(true_labels, true_returns, true_subs, cli_layers, anchors_ok,
          observed=None, per=None, probes=PROBES):
    """⇒ bad 列表（空 = 全合）。`observed=None` ⇒ 只判 L1–L3（`--no-live`）。"""
    bad = []

    # L1 真源自洽
    for name in sorted(true_returns):
        if name not in true_labels:
            bad.append("L1 真源自洽：`exclude_kind` 返回的 %r 不在 %s 里 "
                       "（真源自己就先漂了：分类口吐了一个闭集外的层名）" % (name, TRUE_LABELS_NAME))
    for name in sorted(true_subs):
        if name not in true_labels:
            bad.append("L1 真源自洽：只读预演的 `skip[%r]` 不在 %s 里（计数口吐了闭集外的层名）"
                       % (name, TRUE_LABELS_NAME))

    def matched(c, s):
        return c == s or c.startswith(s + "（")

    # L3 甲：真源 ⟶ CLI
    for s in true_labels:
        if not any(matched(c, s) for c in cli_layers):
            bad.append("L3 甲 对拍：真源层名 %r **没有**对应的 CLI 层名（真源增层 / 改名 ⇒ CLI 静默落后）"
                       % s)
    # L3 乙：CLI ⟶ 真源 ∪ CLI_ONLY（锚点现读校验）
    for c in cli_layers:
        if any(matched(c, s) for s in true_labels):
            continue
        entry = next((e for e in CLI_ONLY if e[0] == c), None)
        if entry is None:
            bad.append("L3 乙 对拍：CLI 层名 %r 在真源闭集（%s）里没有对应、也不在 CLI_ONLY 表里 "
                       "⇒ CLI 自造层名（谁都不知道这一层按什么算的）"
                       % (c, " · ".join(true_labels)))
            continue
        if not anchors_ok.get(c, False):
            bad.append("L3 乙 对拍：CLI 层名 %r 的 CLI_ONLY 锚点现读**找不着**了"
                       "（%s 里的 %r —— 真源那处处置被删/改了 ⇒ 这条白名单腐化，必须重判）"
                       % (c, entry[1], entry[2]))

    if observed is None:
        return bad

    # L4 运行面 ⟷ 声明面
    for name in sorted(observed - set(cli_layers)):
        bad.append("L4 运行面：真跑吐出了闭集外的层名 %r（声明了一套、真跑另一套）" % name)
    for name in cli_layers:
        if name not in observed:
            bad.append("L4 运行面：CLI 声明的层名 %r 在探针集里**一次都没观测到**"
                       "（要么探针退化、要么这一层已经吐不出来了）" % name)
    for rel, want in probes:
        got = (per or {}).get(rel)
        if got is None:
            bad.append("L4 探针：%s 不在现跑结果里（探针没了 ⇒ 无区分度）" % rel)
        elif got != want:
            bad.append("L4 探针：%s 实得层名 %r ≠ 期望 %r（探针退化 / 那一层被人挪走了）"
                       % (rel, got, want))
    got_emit = (per or {}).get(PROBE_EMIT)
    if got_emit is None:
        bad.append("L4 正控：%s 不在现跑结果里" % PROBE_EMIT)
    elif got_emit != LAYER_NONE:
        bad.append("L4 正控：%s 实得层名 %r ≠ %r（「会发」那一件不该命中任何层）"
                   % (PROBE_EMIT, got_emit, LAYER_NONE))
    return bad


# ── 夹具（合成 · **真文件**：把真件 copy 到夹具根再逐格改一处）─────────────────────────
def _slug(name):
    return re.sub(r"[^A-Za-z0-9._-]", "_", name)


def make_fixture(fx_root, case, lib_src, go_src, lib_patch=None, go_patch=None):
    """在**仓外**夹具根里造一份真件副本（真文件，不用 symlink 影子根）。"""
    d = os.path.join(fx_root, case)
    os.makedirs(os.path.join(d, os.path.dirname(LIB_REL)), exist_ok=True)
    os.makedirs(os.path.join(d, os.path.dirname(CLI_REL)), exist_ok=True)
    lib = lib_src if lib_patch is None else lib_patch(lib_src)
    go = go_src if go_patch is None else go_patch(go_src)
    with open(os.path.join(d, LIB_REL), "w", encoding="utf-8") as fh:
        fh.write(lib)
    with open(os.path.join(d, CLI_REL), "w", encoding="utf-8") as fh:
        fh.write(go)
    return d


def _add_true_layer(src):
    """真源多一层（真源增层的那一格）—— 在 `SKIP_LABELS` 里插一个新层名 + 在计数口接上线。"""
    return src.replace('SKIP_LABELS = ("EXCLUDES"',
                       'SKIP_LABELS = ("夹具新层", "EXCLUDES"', 1)


def _add_cli_layer(src):
    """CLI 多一层（CLI 自造层名的那一格）—— 常量块里插一条真源没有的层名。"""
    return src.replace('\tpublishSetLayerWhitelist = "白名单未命中"',
                       '\tpublishSetLayerWhitelist = "白名单未命中"\n'
                       '\tpublishSetLayerFixtureX  = "夹具自造层"', 1)


CASES = (("正控-两侧全合", None, None, []),
         ("负控①-CLI多一层", None, _add_cli_layer,
          ["L3 乙 对拍：CLI 层名 '夹具自造层'"]),
         ("负控②-真源多一层", _add_true_layer, None,
          ["L3 甲 对拍：真源层名 '夹具新层'"]))


def self_test(out, fx_root=None, repo=None) -> bool:
    """成对负控：CLI 多一层 ⇒ 红 · 真源多一层 ⇒ 红 · 两侧全合 ⇒ 绿（真文件夹具 · 不删不链）。"""
    if repo is None:
        here = os.path.dirname(os.path.abspath(__file__))
        repo = os.path.dirname(os.path.dirname(here))
    lib_src = open(os.path.join(repo, LIB_REL), encoding="utf-8").read()
    go_src = open(os.path.join(repo, CLI_REL), encoding="utf-8").read()
    fx_root = fx_root or os.path.join(tempfile.gettempdir(), "zerg-publish-layer-names-fx")
    out.write("── 自检（合成夹具 · 真件副本 · 不碰真目标）──\n夹具根：%s\n" % fx_root)
    total = failed = 0
    for case, lp, gp, want_red in CASES:
        d = make_fixture(fx_root, case, lib_src, go_src, lp, gp)
        labels, why1 = read_true_labels(os.path.join(d, LIB_REL))
        cli, why2 = read_cli_layers(os.path.join(d, CLI_REL))
        if labels is None or cli is None:
            out.write("  ❌ %s：夹具读不回来（%s / %s）\n" % (case, why1, why2))
            failed += 1
            total += 1
            continue
        rets, subs, _ = read_true_usage(os.path.join(d, LIB_REL))
        fx_lib = open(os.path.join(d, LIB_REL), encoding="utf-8").read()
        bad = judge(labels, rets, subs, cli, {e[0]: (e[2] in fx_lib) for e in CLI_ONLY})
        hit = [b for b in bad if any(b.startswith(w) for w in want_red)] if want_red else bad
        ok = bool(hit) if want_red else (not bad)   # 负控：那一条断言**必须**红；正控：一条都不许红
        total += 1
        if not ok:
            failed += 1
        out.write("  %s %s（期望%s · 实得%s%s）\n"
                  % ("✓" if ok else "✗", case, "红" if want_red else "绿",
                     "红" if bad else "绿", "：" + bad[0] if bad and not ok else ""))
    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


LIST_RULES = """判据（四条 · 现读 · 缺口 GAP-20260927-412）
--------------------------------------------
  L1 真源层名闭集：`publish/mirror-public-lib.py` 的 `SKIP_LABELS`（ast 现读）；另判真源自洽
     （`exclude_kind` 的 return 字面量 ⊆ 闭集 · `list_only_report` 的 `skip[…"]` ⊆ 闭集）
  L2 CLI 层名闭集：`core/cmd/zerg/family_publish_set.go` 的 `publishSetLayer<名> = "…"` 常量块
  L3 双向对拍：真源每个层名必须在 CLI 闭集里有对应（相等或 CLI 名 = `真源名（…）`）；
     CLI 每个层名必须对应真源层名，或在 CLI_ONLY 表里（逐条带真源锚点 · 锚点现读校验）
  L4 运行面 ⟷ 声明面：现跑 `publish set <探针> --json` ⇒ 观测集（layer 值 ∪ non_blob_basis
     里出现的闭集名）必须 == CLI 闭集；逐条探针必须打出它期望的那一层；「会发」正控必须 = `—`

不假装覆盖：各层的**判定实现**（哪一件该被哪一层拦）不在本件（那是 check-publish-face-sync.py
与 cli_publish_set_test.go 的面）。本件只读四个源 + 现跑一个只读档，不写盘。
"""


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True,
                                 description="CLI 层名闭集 ⟷ 真源层名口径 对拍")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--bin", default=None)
    ap.add_argument("--no-live", dest="no_live", action="store_true",
                    help="只跑 L1–L3（仓外夹具用；缺 `bin/zerg` 时的显式出口）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--self-test-dir", dest="self_test_dir", default=None)
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        return 0 if self_test(sys.stdout, a.self_test_dir, a.repo_root) else 2

    lib = os.path.join(a.repo_root, LIB_REL)
    go = os.path.join(a.repo_root, CLI_REL)
    true_labels, why = read_true_labels(lib)
    if true_labels is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % why)
        return 2
    cli_layers, why = read_cli_layers(go)
    if cli_layers is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % why)
        return 2
    true_returns, true_subs, _ = read_true_usage(lib)
    if true_returns is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % _)
        return 2

    lib_text = open(lib, encoding="utf-8").read()
    anchors_ok = {e[0]: (e[2] in lib_text) for e in CLI_ONLY}
    sentinel = read_cli_sentinel(go)

    sys.stdout.write("CLI 层名闭集 ⟷ 真源层名口径 对拍 · v%s\n" % VERSION)
    sys.stdout.write("  真源闭集（%s · %d）：%s\n"
                     % (TRUE_LABELS_NAME, len(true_labels), " · ".join(true_labels)))
    sys.stdout.write("  CLI 闭集（publishSetLayer* · %d）：%s\n"
                     % (len(cli_layers), " · ".join(cli_layers)))

    observed = per = None
    if a.no_live:
        sys.stdout.write("  运行面：**未跑**（--no-live：只判 L1–L3）\n")
    else:
        binz = a.bin or os.path.join(a.repo_root, "bin", "zerg")
        argv = [binz, "publish", "set"] + [p for p, _ in PROBES] + [PROBE_EMIT]
        p = subprocess.run(argv + ["--json", "path,caliber,will_publish,layer,non_blob_basis"],
                           capture_output=True, text=True, cwd=a.repo_root,
                           stdin=subprocess.DEVNULL)
        if p.returncode != 0:
            sys.stdout.write("⛔ 不给结论（rc=2）：现跑 `publish set … --json` 退码 %d"
                             "（stderr 前 200 字：%s）\n" % (p.returncode, (p.stderr or "").strip()[:200]))
            return 2
        try:
            doc = json.loads(p.stdout)
        except ValueError as e:
            sys.stdout.write("⛔ 不给结论（rc=2）：现跑输出不是 JSON（%s）\n" % e)
            return 2
        items = doc.get("items") or []
        if not items:
            sys.stdout.write("⛔ 不给结论（rc=2）：现跑 `items` 为空（零件 ⇒ 无区分度）\n")
            return 2
        observed, per = observed_from_live(items, cli_layers)
        sys.stdout.write("  运行面（现跑 %d 件）：观测层名 %d 个 —— %s\n"
                         % (len(items), len(observed), " · ".join(sorted(observed))))

    bad = judge(true_labels, true_returns, true_subs, cli_layers, anchors_ok, observed, per)
    if sentinel != LAYER_NONE:
        bad.append("L2 哨兵：「没命中任何层」那一格的值 %r ≠ 本件认的 %r"
                   "（`%s` 被人改动了 —— 那一格一改，「会发」的件就会被打上一个层名）"
                   % (sentinel, LAYER_NONE, CLI_LAYER_SENTINEL_ID))
    if bad:
        sys.stdout.write("⇒ 判红（rc=1）：\n")
        for b in bad:
            sys.stdout.write("  ✗ %s\n" % b)
    else:
        sys.stdout.write("  ✓ L1 真源自洽（分类口/计数口只吐闭集里的名字）\n")
        sys.stdout.write("  ✓ L2 CLI 闭集 %d 个（现读常量块）\n" % len(cli_layers))
        sys.stdout.write("  ✓ L3 双向对拍：真源 %d 个层名全部有 CLI 对应；CLI 侧多出的 %d 个都有"
                         "真源锚点（现读校验过）\n"
                         % (len(true_labels), len([c for c in cli_layers
                                                   if not any(c == s or c.startswith(s + "（")
                                                              for s in true_labels)])))
        if observed is not None:
            sys.stdout.write("  ✓ L4 运行面 == 声明面（观测 %d 个层名 · 探针 %d 枚逐条命中期望 ·"
                             " 「会发」正控 = %s）\n" % (len(observed), len(PROBES), LAYER_NONE))
        sys.stdout.write("⇒ 四条都合（rc=0）\n")
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-publish-layer-names", "version": VERSION,
            "true_labels": list(true_labels), "cli_layers": list(cli_layers),
            "cli_only": [e[0] for e in CLI_ONLY],
            "live": observed is not None,
            "observed": sorted(observed) if observed is not None else None,
            "bad": bad, "mode": "judge",
        }, ensure_ascii=False) + "\n")
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())

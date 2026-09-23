#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""gen-cli-reference.py —— 正式面「参考-命令行」页的生成器（`docs/zh` + `docs/en` **同构双版**）。

为什么要有它（正式面 CLI 文档的**参考篇**必须机械生成）
------------------------------------------------------
命令面是「契约 + 唯一入口」，它的参考页如果**手写**命令名与旗标，就一定会与命令树分叉（§一 1.2 的
病灶同源：九件 cmd 各写各的）。本生成器把「命令名 / 旗标 / `--json` 字段 / 投影端点 / 茧壁层级 /
退码表」全部**现跑**出来 —— 页里一个命令名都不是手写的：

  · `bin/zerg help export --out <临时目录>` → 命令清单表 · 危险动作表 · 退码表 · 配置优先级 · 计数
  · `bin/zerg help`                        → 每条命令的**用法串**（旗标那一列）
  · `core/cmd/zerg/main.go` 的命令树        → **独立的第二份真源**，用来做命令名对拍（0 差才算过）

三档（与仓内其它门脚本同一口径：0/1/2）
  0  `--check` 全过（产物与现跑逐字节一致 · 命令名对拍 0 差）
  1  `--check` 判红（产物 drift：谁改了页没重生成，或命令树变了页没跟上）
  2  **不给结论**（缺 `bin/zerg` / 命令树读不出 / 导出物解析不上 / 表格形状不认识 —— 一律不猜、不假绿）

只放开源内容的硬约束（写进渲染器、不靠人自觉）
  · 导出物的「出处」列（指向设计稿 §与开工单号）**整列不进公开面**；
  · 其余单元格里的 `（§…）` 类引用**机械剥掉**；剥完若全页仍残留 `§` ⇒ **rc=2 拒绝出页**（fail-closed）；
  · 本页只讲**本版现跑**：不写设计稿/调研稿/待办的未来形状。

用法：
  python3 scripts/docs/gen-cli-reference.py --emit     # 生成/重生成两份页（先 build-all.sh --only-cli）
  python3 scripts/docs/gen-cli-reference.py --check    # 判 drift + 命令名对拍（不改仓）
"""
import argparse
import glob
import os
import re
import shutil
import subprocess
import sys
import tempfile

ZH_REL = os.path.join("docs", "zh", "参考-命令行.md")
EN_REL = os.path.join("docs", "en", "参考-命令行.md")
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
GEN_REL = os.path.join("scripts", "docs", "gen-cli-reference.py")
RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2

# ── locale 模板（两版**同构**：同一份数据、同一节序，只有本页自己的散文分语种）──────────────────
L = {
    "zh": {
        "title": "参考-命令行",
        "h1": "参考-命令行",
        "lead": [
            "**本页由命令树现算生成**（`python3 scripts/docs/gen-cli-reference.py --emit`）：数据来自"
            " `zerg help export` 与 `zerg help` 的现跑输出 —— 命令名、旗标、`--json` 字段、投影端点、"
            "茧壁层级、退码表**逐字来自命令树，一个都不是手写的**。",
            "改命令面 ⇒ 重跑生成器（`--check` 会判 drift：产物与现跑不一致就红）；"
            "本页的命令名与命令树源码**现算对拍 = 0 差**。",
            "本版现跑：命令面 **{total}** 条 = 已开放 **{open}** 条 + 危险动作 **{danger}** 条"
            "（危险动作本版一律**未开放**，只有 `--dry-run` 计划面可看）。",
            "本页只讲**本版**：不写设计稿/调研稿/待办里的未来形状；导出物里指向过程档案的「出处」列"
            "**不进公开面**（公开面只讲虫族是什么）。",
        ],
        "h_cmds": "命令族与全部命令",
        "cmds_note": [
            "「命令」列 = 命令树里的命令名；「用法（旗标）」列 = 命令树 `usage` 字段**逐字**"
            "（`zerg help` 的用法块是它的投影，生成器断言两者**逐字一致**）；",
            "其余四列（说明 / `--json` 字段 / 投影的远端端点 / 茧壁层级）逐字来自 `zerg help export`。",
            "单元格里的 `|` 按 markdown 规矩转义成 `\\|`（**唯一**的排版改写，未动任何字符）。",
        ],
        "h_open": "已开放（{n} 条）",
        "a_open": "已开放",
        "h_danger": "危险动作（{n} 条 · 已登记 · 本版未开放）",
        "a_danger": "危险动作",
        "th_open": ["命令", "用法（旗标）", "说明", "`--json` 字段", "投影的远端端点", "茧壁层级"],
        "th_danger": ["命令", "档", "三态", "`--confirm` 的目标", "它会动什么", "茧壁层级"],
        "danger_note": "危险动作的「档 / 三态 / `--confirm` 的目标 / 它会动什么」逐字来自 `zerg help export`"
                       "（导出物里还有一列「出处」= 把命令对回设计稿的追溯面，**本页不承载**）。",
        "h_exit": "退出码表",
        "exit_note": "逐字来自 `zerg help export` 的退码表（唯一真源在 `core/cmd/zerg/exitcodes.go`；"
                     "`zerg help exit-codes` 是它的自描述面）。",
        "th_exit": ["码", "名", "语义", "可重试性"],
        "h_json": "机器面形状要点",
        "json_note": "本节是**形状契约**的要点（真源 = 代码内 schema；`zerg help contract` 是它的自描述面）。",
        "json_body": [
            "**三层版本**（不许混成一层）：① 契约号 `schema:\"zerg/v1\"` —— 这份 `--json` 输出的**形状契约**"
            "是第几版；② 对象 schema 版本 —— 这枚卵/这份档案的**数据**是哪版；③ 组件版本（semver）—— "
            "哪台机跑的哪个二进制。`zerg version --json` **三个都报**。",
            "**统一外层包封**（`--json` 的唯一出口，六键恒在）：`schema`（恒 `\"zerg/v1\"`）· `kind`"
            "（与命令一一对应、单数 CamelCase）· `items`（**恒数组**，空为 `[]`、永不为 `null`）· "
            "`meta`（`count` / `source`；多机目标进 `node`）· `warnings`（降级/未证实一律在这里**显式**报，"
            "不静默）· `truncated`（bool）；出错时才多一块 `__error__`。",
            "**字段稳定性承诺**：① 既有字段的名字/类型/语义**只增不改**（消费侧必须忽略未识别键）；"
            "② **不承诺字段序**——对象序只由显式排序旗标决定；③ 破坏性变更**走大版本**（旧主号继续可读）；"
            "④ 废弃四步：标注 deprecated（进 `warnings`）⇒ 文档删示例 ⇒ 导出处删字段 ⇒ 下一个大版本删字段。",
            "**拒绝规则**：请求未知字段 ⇒ 退码 `2` + 列全部合法字段；`--json` 不给字段 ⇒ 退码 `1` + "
            "stderr 列全部字段（schema 自描述）；`--schema` 主号不认 ⇒ 退码 `2`；`--json '*'` **不存在** —— "
            "要的是逗号分隔的字段名。",
            "**三态输出**：人面（TTY 下的表格，只是渲染）· 行式面（非 TTY 或 `--plain`：制表符分隔，字段集"
            "与字段序与人面**一模一样**）· 机器面（`--json <字段>`：包封 + 点名字段，不因 TTY 改形状）。",
        ],
        "h_config": "配置优先级",
        "config_note": "逐字来自 `zerg help export`（旗标最优先，内置默认兜底）。",
        "footer": "本页是**导出物**：真源永远是命令树源码（`{main}`）与代码内 schema；"
                  "改命令面请重跑 `python3 {gen} --emit`，并用 `--check` 复判。",
    },
    "en": {
        "title": "CLI reference (参考-命令行)",
        "h1": "参考-命令行",
        "lead": [
            "**This page is generated from the command tree** "
            "(`python3 scripts/docs/gen-cli-reference.py --emit`): the data comes from live runs of "
            "`zerg help export` and `zerg help` — command names, flags, `--json` fields, projected "
            "endpoints, wall layer and the exit-code table are quoted **verbatim from the command tree; "
            "not one name is hand-written**.",
            "Change the command surface ⇒ re-run the generator (`--check` fails on drift); the command "
            "names on this page are cross-checked against the command tree source with **0 diff**.",
            "This build (live run): **{total}** commands = **{open}** open + **{danger}** dangerous "
            "(dangerous actions are **not opened in this version**; only their `--dry-run` plan face is "
            "readable).",
            "This page documents **this build only**: it carries no future shapes from design notes, "
            "research notes or to-do lists, and the export's provenance column (design-doc references) "
            "**is left out of the published face**.",
            "Cells are quoted verbatim from the CLI's own (Chinese) help output — the contract on this "
            "page is the command names, flags, field lists and exit codes; the CLI's own prose is not "
            "translated here.",
        ],
        "h_cmds": "命令族与全部命令",
        "cmds_note": [
            "The “command” column is the command name from the tree; the “usage (flags)” column quotes "
            "the tree's own `usage` field verbatim (the `zerg help` usage block is its projection; the "
            "generator asserts the two agree character for character).",
            "The other four columns (summary / `--json` fields / projected endpoint / wall layer) are "
            "quoted verbatim from `zerg help export`.",
            "A literal `|` inside a cell is escaped as `\\|` for markdown (the **only** typographic "
            "change; no character was altered).",
        ],
        "h_open": "Open ({n})",
        "a_open": "已开放",
        "h_danger": "Dangerous actions ({n} · registered · not opened in this build)",
        "a_danger": "危险动作",
        "th_open": ["command", "usage (flags)", "summary", "`--json` fields", "projected endpoint",
                    "wall layer"],
        "th_danger": ["command", "tier", "three states", "`--confirm` target", "what it touches",
                      "wall layer"],
        "danger_note": "“tier / three states / `--confirm` target / what it touches” are quoted verbatim "
                       "from `zerg help export` (the export also carries a provenance column mapping a "
                       "command back to the design doc — **this page does not carry it**).",
        "h_exit": "退出码表",
        "exit_note": "Quoted verbatim from `zerg help export` (single source of truth: "
                     "`core/cmd/zerg/exitcodes.go`; `zerg help exit-codes` is its self-describing face).",
        "th_exit": ["code", "name", "meaning", "retryable"],
        "h_json": "机器面形状要点",
        "json_note": "This section is the outline of the **shape contract** (source of truth: the schema "
                     "in code; `zerg help contract` is its self-describing face).",
        "json_body": [
            "**Three version layers** (never collapsed into one): (1) contract id `schema:\"zerg/v1\"` — "
            "which version of the `--json` **shape contract** this is; (2) object schema version — which "
            "version of the **data** (an egg, an archive); (3) component version (semver) — which binary "
            "on which machine. `zerg version --json` reports **all three**.",
            "**One outer envelope** (the only `--json` exit, six keys always present): `schema` (always "
            "`\"zerg/v1\"`) · `kind` (one per command, singular CamelCase) · `items` (**always an array**, "
            "`[]` when empty, never `null`) · `meta` (`count` / `source`; a multi-machine target adds "
            "`node`) · `warnings` (degradation is always reported **explicitly** here, never silently) · "
            "`truncated` (bool); an `__error__` block appears only on failure.",
            "**Field stability promise**: (1) existing field names/types/meanings are **only added to, "
            "never changed** (consumers must ignore unknown keys); (2) **field order is not promised** — "
            "object order is decided only by an explicit sort flag; (3) breaking changes **take a major "
            "version bump** (the old major keeps working); (4) the four deprecation steps: mark "
            "deprecated (into `warnings`) ⇒ drop examples from docs ⇒ drop the field from the export ⇒ "
            "drop it in the next major.",
            "**Rejection rules**: an unknown field ⇒ exit `2` + list every legal field; `--json` with no "
            "fields ⇒ exit `1` + list every field on stderr (the schema describing itself); an "
            "unrecognised `--schema` major ⇒ exit `2`; `--json '*'` **does not exist** — what it wants is "
            "a comma-separated field list.",
            "**Three output modes**: human face (aligned table on a TTY, rendering only) · line face "
            "(non-TTY or `--plain`: tab-separated, with a field set and field order **identical** to the "
            "machine face (`--json <字段>`: envelope + named fields, whose shape does not "
            "change with the TTY).",
        ],
        "h_config": "配置优先级",
        "config_note": "Quoted verbatim from `zerg help export` (flags first, built-in defaults last).",
        "footer": "This page is an **export product**: the source of truth is always the command tree "
                  "source (`{main}`) plus the schema in code. After changing the command surface, re-run "
                  "`python3 {gen} --emit` and re-judge with `--check`.",
    },
}

CITATION_RE = re.compile(r"[（(][^（）()]*§[^（）()]*[）)]")
PATH_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
CONST_RE = re.compile(r'^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*"([^"]*)"\s*$', re.M)
NOFIELDS = "（无机器面）"
LAYERS = ("host", "node", "space")

# ── 列锚：**按表头名**取列（不是按列数）──────────────────────────────────────────────────────
# 为什么（`M-5` 的落地）：`zerg help export` 的危险动作表在 2.5.11 里**多了一列**「本版」
# （7 → 8 列，`core/cmd/zerg/export.go` 的表头行），而本器原来把列数与「出处」列的位次
# 都写死（`7` / `drop_col=5`）⇒ 当场 rc=2（列数 8 ≠ 7）。按**表头名**取列之后：导出物再长列
# 或者调列序都不会把公开面看错列；但表头里认不出的形状仍然 fail-closed（rc=2，不猜、不假绿）。
PUBLIC_COLS = {
    "命令清单": ("命令", "说明", "`--json` 字段", "投影的远端端点", "茧壁层级"),
    "危险动作": ("命令", "档", "三态", "`--confirm` 的目标", "它会动什么", "茧壁层级"),
    "退码表": ("码", "名", "语义", "可重试性"),
}
# 锚列 = 「形态是闭集、尺寸恒 1」的列（用表头名钉住）；其余是自由列，吸收被 `|` 切开的格子。
COL_ANCHORS = {
    "命令": re.compile(r"^`zerg(?: [^`]+)?`$"),
    "档": re.compile(r"^D[123]$"),
    "本版": re.compile(r"^\*{0,2}(?:已开放|未开放)\*{0,2}$"),
    "三态": re.compile(r"^--dry-run(?: |·|$)"),
    # 字段列只有两种形态：逗号列表 或 `（无机器面）`（现读 62 / 15 · **没有**单标识符形态）
    # ⇒ 逗号是必需的，否则 `zerg model opts` 那行的端点列 `GET` 会被当成字段列锚点。
    "`--json` 字段": re.compile(r"^(?:%s|%s)$"
                                % (re.escape(NOFIELDS),
                                   r"[A-Za-z_][A-Za-z0-9_]*(?:,[A-Za-z_][A-Za-z0-9_]*)+")),
    "茧壁层级": re.compile(r"^`?(?:host|node|space)`?$"),
}


# ── 取数：现跑的命令面 ──────────────────────────────────────────────────────────────────────
def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, MAIN_REL)):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def strip_citations(text):
    """剥掉单元格里的 `（§…）` 类引用（设计稿编号）—— 公开面不承载过程档案的追溯。"""
    prev = None
    out = text
    while prev != out:
        prev = out
        out = CITATION_RE.sub("", out)
    return out.strip()


def const_usages(root):
    """包内「`名字 = "字面量"`」表 —— `usage: <标识符>` 那种写法要回源件里查它的字面量。

    为什么（**第二处阻塞** · 与列锚同一天撞上）：命令树里现读有 **1 条**命令的 `usage` 不是字面量
    而是包内常量（`impactUsageLine`，定义在 `core/cmd/zerg/family_impact.go`，件内注释逐字写着
    「与命令树的 `usage` 逐字同源 · 本件不旁写第二份」）。只认字面量的老解析器对这条读出空串
    ⇒ 与 `zerg help` 的用法行对不上 ⇒ rc=2。查不到标识符的字面量 ⇒ 上层仍按空串处理（rc=2，不猜）。
    """
    out = {}
    pkg = os.path.dirname(os.path.join(root, MAIN_REL))
    for name in sorted(glob.glob(os.path.join(pkg, "*.go"))):
        for m in CONST_RE.finditer(read(name)):
            out.setdefault(m.group(1), m.group(2))
    return out


def tree_from_source(root):
    """命令树源码（`main.go`）→ [(命令名, 用法串)]（**独立于导出物**的第二份真源）。

    为什么要按块切：`usage` 是每条命令自己的字段（旗标就在那一串里），而 `zerg help` 的用法块
    **只列已开放的命令**（危险动作不在里面）⇒ 旗标列的真源只能是命令树源码。
    """
    src = read(os.path.join(root, MAIN_REL))
    try:
        start = src.index('commands = []*command{')
        end = src.index('\n\t}\n', start)
    except ValueError:
        return None
    seg = src[start:end]
    consts = const_usages(root)
    out, matches = [], list(PATH_RE.finditer(seg))
    for i, m in enumerate(matches):
        parts = re.findall(r'"([^"]+)"', m.group(1))
        if not parts:
            continue
        nxt = matches[i + 1].start() if i + 1 < len(matches) else len(seg)
        block = seg[m.end():nxt]
        u = re.search(r'usage:\s*"([^"]*)"', block)
        if u:
            usage = u.group(1)
        else:
            ident = re.search(r"usage:\s*([A-Za-z_][A-Za-z0-9_]*)", block)
            usage = consts.get(ident.group(1), "") if ident else ""
        out.append((" ".join(parts), usage))
    return out or None


def run(binz, argv):
    p = subprocess.run([binz] + argv, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    return p.returncode, p.stdout, p.stderr


def usage_lines(binz):
    """`zerg help` 的用法串 → {命令名: 用法串}（逐字；命令名来自命令树，人没写过一个）。"""
    rc, out, err = run(binz, ["help"])
    if rc != 0:
        return None, "`zerg help` 退码 %d（%s）" % (rc, (err or out).strip()[:200])
    lines = []
    in_usage = False
    for line in out.splitlines():
        if line.strip() == "用法:":
            in_usage = True
            continue
        if in_usage:
            if not line.strip():
                in_usage = False
                continue
            s = line.strip()
            if s.startswith("zerg ") or s == "zerg":
                lines.append(s)
    if not lines:
        return None, "`zerg help` 里没找到用法行（空转 = 假覆盖）"
    return lines, ""


def map_usage(paths, lines):
    """把用法行按**最长命令名**归属到各条命令（`zerg dev proposal new …` 归 `dev proposal new`）。"""
    out = {}
    used = set()
    for path in sorted(paths, key=lambda p: len(p.split()), reverse=True):
        pref = "zerg " + path
        for i, line in enumerate(lines):
            if i in used:
                continue
            if line == pref or line.startswith(pref + " "):
                out[path] = line
                used.add(i)
                break
    return out


def parse_export_table(lines, heading):
    """导出物某个小节的那一张 markdown 表 → (表头名列, [行单元格…])。

    表头名列 = **列锚的真源**（`M-5`）：导出物加了列之后，「按列数写死」的锚当场失配 ⇒ 一律
    改从表头名出发。表头行读不出 / 小节里没有表 ⇒ 抛 `ValueError`（⇒ rc=2 不给结论）。
    """
    start = None
    for i, l in enumerate(lines):
        if l.startswith("## ") and heading in l:
            start = i
            break
    if start is None:
        raise ValueError("导出物里找不到小节：%s" % heading)
    header, rows = None, []
    for l in lines[start + 1:]:
        if l.startswith("## "):
            break
        if not l.startswith("|"):
            continue
        if set(l) <= set("|- "):
            continue          # 分隔行
        cells = [c.strip() for c in l.strip().strip("|").split("|")]
        if header is None:
            header = cells     # 表头行（本节只有一张表 ⇒ 第一行恒是表头）
            continue
        rows.append(cells)
    if not header:
        raise ValueError("导出物小节里没有表：%s" % heading)
    return header, rows


def anchor_of(name):
    """表头名 → 锚谓词（没登记 ⇒ `None` = 自由列，可以吸收被 `|` 切开的格子）。"""
    return COL_ANCHORS.get(name)


def _distributions(surplus, k):
    """把 `surplus` 个多出来的格子分给 k 个自由列的全部方案（靠前的列先分）。"""
    if k == 1:
        yield (surplus,)
        return
    for first in range(surplus, -1, -1):
        for rest in _distributions(surplus - first, k - 1):
            yield (first,) + rest


def align_row(cells, header):
    """把「被单元格内的 `|` 切开」的行拼回表头列的尺寸（**靠锚列对齐** · 多出的格子进自由列）。

    所有锚列都必须对上才算拼成；拼不回去 ⇒ `ValueError`（rc=2，不猜 —— 这正是「按列数写死」
    的老写法失配时的下场，本器不把它降级成假绿）。
    """
    n = len(header)
    if len(cells) == n:
        return cells
    if len(cells) < n:
        raise ValueError("列数 %d < 表头 %d 列：%s" % (len(cells), n, " | ".join(cells)))
    free = [i for i, name in enumerate(header) if anchor_of(name) is None]
    if not free:
        raise ValueError("列数 %d ≠ 表头 %d 列，且表头里没有自由列可吸收多出的格子：%s"
                         % (len(cells), n, " | ".join(header)))
    for combo in _distributions(len(cells) - n, len(free)):
        take = [1] * n
        for k, i in enumerate(free):
            take[i] += combo[k]
        out, p, ok = [], 0, True
        for j, name in enumerate(header):
            seg = cells[p:p + take[j]]
            p += take[j]
            rx = anchor_of(name)
            if rx is not None and not rx.match(seg[0]):
                ok = False
                break
            out.append(" | ".join(seg))
        if ok:
            return out
    raise ValueError("列数 %d ≠ 表头 %d 列且按表头名拼不回去（锚列对不上）：%s"
                     % (len(cells), n, " | ".join(cells)))


def cells_for_table(lines, heading, cols, layer_col="茧壁层级"):
    """→ [(cells…)]：**按表头名**取公开列（导出物多出来的列不进这一页）· 每格先剥设计稿引用。

    现读多出来、**不进公开面**的两列：`本版`（逐条开放状态 —— 属 `zerg help dangerous` 的人面）
    与 `出处`（把命令对回设计稿的追溯面）。列名在表头里找不到 ⇒ `ValueError`（rc=2）；
    `layer_col=""` = 本表没有茧壁层级列（退码表）。
    """
    header, raw_rows = parse_export_table(lines, heading)
    idx = []
    for name in cols:
        if name not in header:
            raise ValueError("导出物表头里找不到列名 %r（现读表头：%s）"
                             % (name, " | ".join(header)))
        idx.append(header.index(name))
    out = []
    for raw in raw_rows:
        cells = align_row(raw, header)
        if layer_col:
            layer = cells[header.index(layer_col)].strip("`")
            if layer not in LAYERS:
                raise ValueError("茧壁层级列不是闭集三值（%r）：%s" % (layer, " | ".join(cells)))
        out.append([strip_citations(cells[i]) for i in idx])
    return out


def live_data(root):
    """现跑取数 → (data, err)。err 非空 ⇒ rc=2 不给结论。"""
    binz = os.path.join(root, "bin", "zerg")
    if not os.path.isfile(binz):
        return None, ("找不到 bin/zerg —— 先跑 `bash scripts/build/build-all.sh --only-cli` "
                      "出**新**制品（旧制品会把新命令当未知命令静默漏掉）")
    paths = tree_from_source(root)
    if not paths:
        return None, "命令树读不出来（%s 空/形状变了）—— 判据不可判" % MAIN_REL
    umap_all = dict(paths)

    tmp = tempfile.mkdtemp(prefix="zerg-cli-ref-")
    try:
        rc, out, err = run(binz, ["help", "export", "--out", tmp])
        if rc != 0:
            return None, "`zerg help export --out` 退码 %d（%s）" % (rc, (err or out).strip()[:200])
        cands = sorted(glob.glob(os.path.join(tmp, "导出-命令面帮助-*.md")))
        if not cands:
            return None, "导出物没落盘（空转 = 假覆盖）"
        exp = read(cands[-1]).splitlines()
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    # 身份与计数（逐字取自导出物的抬头）
    ident, counts = "", ""
    for l in exp[:10]:
        if l.startswith("> 命令面身份："):
            ident = l.split("：", 1)[1].strip()
        if l.startswith("> 本版："):
            counts = l.split("：", 1)[1].strip()
    nums = re.findall(r"\*\*(\d+)\*\*", counts)
    if len(nums) != 2:
        return None, "导出物抬头的计数读不出来：%r" % counts
    n_open, n_danger = int(nums[0]), int(nums[1])

    try:
        open_rows = cells_for_table(exp, "命令清单", PUBLIC_COLS["命令清单"])
        danger_rows = cells_for_table(exp, "危险动作", PUBLIC_COLS["危险动作"])
        exit_rows = cells_for_table(exp, "退码表", PUBLIC_COLS["退码表"], layer_col="")
    except ValueError as e:
        return None, "导出物表格解析不上（%s）—— 不猜，拒绝出页" % e
    if len(open_rows) != n_open or len(danger_rows) != n_danger or len(exit_rows) < 2:
        return None, ("导出物自相矛盾：抬头说 %d 开/%d 危，表里读到 %d/%d，退码 %d 行"
                      % (n_open, n_danger, len(open_rows), len(danger_rows), len(exit_rows)))

    # 退码表的「占号纪律 / 门禁直通档」两行（逐字取自导出物，非表格行）
    tail_lines = [l.strip() for l in exp if l.strip().startswith(("占号纪律", "门禁直通档"))]
    if len(tail_lines) != 2:
        return None, "退码表的两条纪律行读不出来（读到 %d 条）" % len(tail_lines)
    cfg = [l.strip() for l in exp if l.strip() and not l.startswith(("#", ">", "|", "-"))]
    cfg_line = next((l for l in cfg if l.startswith("旗标 >") or "环境变量" in l), None)
    if not cfg_line:
        return None, "配置优先级行读不出来"

    usages, why = usage_lines(binz)
    if usages is None:
        return None, why
    # 旗标列的真源 = 命令树源码的 `usage`（87 条全有）；`zerg help` 的用法块是它的投影 ⇒ **逐字对拍**
    help_map = map_usage([p for p, _ in paths], usages)
    for path, line in sorted(help_map.items()):
        if umap_all.get(path) != line:
            return None, "两处真源不一致：`zerg help` 的用法行与命令树 usage 不同（%s）\n    help: %s\n    tree: %s" % (
                path, line, umap_all.get(path))
    n_help = len([1 for p, _ in paths if p in help_map])
    miss_usage = [p for p, _ in paths if not umap_all.get(p)]
    if miss_usage:
        return None, "这些命令在命令树里没有 usage 串（旗标列没法现算）：%s" % " / ".join(miss_usage)

    names_page = [r[0].strip("`").replace("zerg ", "") for r in open_rows + danger_rows]
    return {
        "ident": ident, "counts": counts, "n_open": n_open, "n_danger": n_danger,
        "open": open_rows, "danger": danger_rows, "exit": exit_rows,
        "exit_tail": tail_lines, "cfg": cfg_line, "usage": umap_all,
        "names_page": names_page, "names_tree": [p for p, _ in paths],
        "n_help_agree": n_help,
    }, ""


# ── 渲染：两份**同构**页（同一数据、同一节序；只有本页散文分语种）────────────────────────────────
def esc(cell):
    return cell.replace("|", "\\|")


def fm(loc, data, zh_body=""):
    today = "2026-09-21"
    head = ["---", "title: %s" % L[loc]["title"], "type: reference",
            "status: authoritative" if loc == "zh" else "status: draft",
            "reviewed_at: %s" % today if loc == "zh" else None,
            "source_of_truth:", "  - %s" % MAIN_REL, "  - core/cmd/zerg/exitcodes.go",
            "  - core/cmd/zerg/export.go", "  - %s" % GEN_REL,
            "owner: Mr2109", "updated_at: %s" % today, "ai_assisted: ai"]
    if loc == "en":
        head += ["source_lang: zh", "translation_of: %s" % ZH_REL.replace(os.sep, "/"),
                 "translation_status: human_draft"]
    head += ["---", ""]
    if loc == "en":
        # 源文注释块 = **段落级保留源文**（`docs/en/README.md` §2 rule 5）：把 zh 正文**机械**嵌进来。
        # 为什么要嵌（术语门 `T1 B5` 的判据）：正文的行内代码集合必须与注释块里的集合**逐字相等**
        #   —— 只写一句「见 zh 侧」⇒ 注释块的集合几乎是空的 ⇒ 正文里 214 个标识符全被判成「多出」。
        # 嵌入是生成器做的（`render(loc, data, zh_body)`），人一个字符都不抄。
        head += ["<!-- ===== 源文 / source text（%s · 段落级保留：双语正式面「源文注释」条 + docs/en/README.md 的 rule 5）====="
                 % ZH_REL.replace(os.sep, "/"),
                 "     本页正文的行内代码标识符集合与本块**逐字相同**（生成器 `--check` 现算对拍）。",
                 "",
                 zh_body,
                 "===== 源文结束 / end of source text ===== -->", ""]
    head += [""]
    return "\n".join(h for h in head if h is not None)


def render(loc, data, zh_body=""):
    t = L[loc]
    b = [fm(loc, data, zh_body)]
    b.append("# %s {: #%s }" % (t["h1"], t["h1"]))
    b.append("")
    for line in t["lead"]:
        b.append("> " + line.format(total=data["n_open"] + data["n_danger"],
                                    open=data["n_open"], danger=data["n_danger"]))
    b.append("> 身份 / identity：`%s`" % data["ident"])
    b.append("")

    b.append("## %s {: #%s }" % (t["h_cmds"], t["h_cmds"]))
    b.append("")
    for line in t["cmds_note"]:
        b.append("- " + line)
    b.append("")
    # 小节标题里带计数、**锚不带**（锚是地址，不许随计数漂 —— 同 §5.6② 的「锚冻结」）
    b.append("### %s {: #%s }" % (t["h_open"].format(n=data["n_open"]), t["a_open"]))
    b.append("")
    b.append("| " + " | ".join(t["th_open"]) + " |")
    b.append("|" + "---|" * len(t["th_open"]))
    for cells in data["open"]:
        cmd = cells[0].strip("`").replace("zerg ", "")
        b.append("| `%s` | `%s` | %s |" % (cells[0].strip("`"),
                                           esc(data["usage"][cmd]), esc(cells[1])))
        b[-1] += " %s | %s | %s |" % (esc(cells[2]), esc(cells[3]), esc(cells[4]))
    b.append("")
    b.append("### %s {: #%s }" % (t["h_danger"].format(n=data["n_danger"]), t["a_danger"]))
    b.append("")
    b.append("> " + t["danger_note"])
    b.append("")
    b.append("| " + " | ".join(t["th_danger"]) + " |")
    b.append("|" + "---|" * len(t["th_danger"]))
    for cells in data["danger"]:
        b.append("| `%s` | %s | %s | %s | %s | %s |"
                 % (cells[0].strip("`"), esc(cells[1]), esc(cells[2]), esc(cells[3]),
                    esc(cells[4]), esc(cells[5])))
    b.append("")

    b.append("## %s {: #%s }" % (t["h_exit"], t["h_exit"]))
    b.append("")
    b.append("> " + t["exit_note"])
    b.append("")
    b.append("| " + " | ".join(t["th_exit"]) + " |")
    b.append("|" + "---|" * len(t["th_exit"]))
    for cells in data["exit"]:
        b.append("| " + " | ".join(esc(c) for c in cells) + " |")
    b.append("")
    for l in data["exit_tail"]:
        b.append(l)
        b.append("")

    b.append("## %s {: #%s }" % (t["h_json"], t["h_json"]))
    b.append("")
    b.append("> " + t["json_note"])
    b.append("")
    for line in t["json_body"]:
        b.append("- " + line)
    b.append("")

    b.append("## %s {: #%s }" % (t["h_config"], t["h_config"]))
    b.append("")
    b.append("> " + t["config_note"])
    b.append("")
    b.append("    " + data["cfg"])
    b.append("")
    b.append("---")
    b.append("")
    b.append(t["footer"].format(main=MAIN_REL, gen=GEN_REL))
    b.append("")
    return "\n".join(b)


def body_of(page):
    """去掉 frontmatter 的正文（源文注释块里要嵌的就是它）。"""
    if not page.startswith("---\n"):
        return page
    try:
        end = page.index("\n---\n", 3)
    except ValueError:
        return page
    return page[end + 5:]


def build(root):
    data, err = live_data(root)
    if err:
        return None, err
    zh = render("zh", data)
    en = render("en", data, zh_body=body_of(zh))
    return {"zh": zh, "en": en, "data": data}, ""


CODE_RE = re.compile(r"`([^`\n]+)`")
COMMENT_RE = re.compile(r"<!--.*?-->", re.S)


def code_spans(text):
    """剥掉源文注释块后的行内代码标识符集合（术语门 `T1 B5` 的判据：两版必须**逐字同一集合**）。"""
    return set(CODE_RE.findall(COMMENT_RE.sub("", text)))


def check(root, pages):
    """drift（产物 vs 现跑逐字节）+ 命令名对拍（页 ⟷ 源码树，0 差）+ 双语标识符零改动。"""
    errs = []
    for loc, rel in (("zh", ZH_REL), ("en", EN_REL)):
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            errs.append("缺产物：%s（本版按 --emit 生成过吗）" % rel)
            continue
        if read(p) != pages[loc]:
            errs.append("drift：%s 与现跑不一致（改了命令面没重生成？重跑 --emit）" % rel)
    data = pages["data"]
    page_set = set(data["names_page"])
    tree_set = set(data["names_tree"])
    if page_set != tree_set:
        miss = sorted(tree_set - page_set)
        extra = sorted(page_set - tree_set)
        errs.append("命令名对拍：页里缺 %d 条（%s…）· 页里多 %d 条（%s…）"
                    % (len(miss), " / ".join(miss[:5]), len(extra), " / ".join(extra[:5])))
    if len(page_set) != data["n_open"] + data["n_danger"]:
        errs.append("计数对拍：页里 %d 条 ≠ 导出物抬头 %d + %d"
                    % (len(page_set), data["n_open"], data["n_danger"]))
    for loc, rel in (("zh", ZH_REL), ("en", EN_REL)):
        if "§" in pages[loc]:
            errs.append("%s 里残留设计稿引用（§）—— 公开面不承载过程档案（fail-closed）" % rel)
    # 双语「标识符零改动」（§5.6② · docs/en/README.md §2 rule 5）：正文行内代码集合两版**逐字相等**
    sz, se = code_spans(pages["zh"]), code_spans(pages["en"])
    if sz != se:
        errs.append("双语标识符集合不一致：zh 独有 %s · en 独有 %s"
                    % (sorted(sz - se)[:5] or "无", sorted(se - sz)[:5] or "无"))
    return errs


def main():
    ap = argparse.ArgumentParser(add_help=True, description="正式面「参考-命令行」页的生成器（zh/en 同构）")
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--emit", action="store_true", help="生成/重生成两份页")
    g.add_argument("--check", action="store_true", help="判 drift + 命令名对拍（不改仓）")
    a = ap.parse_args()
    root = find_root(os.getcwd())
    if root is None:
        print("⛔ 不给结论：找不到仓根（上溯找不到 %s）" % MAIN_REL)
        return RC_BLOCKED
    pages, err = build(root)
    if err:
        print("⛔ 不给结论（rc=2）：%s" % err)
        return RC_BLOCKED
    if a.emit:
        for loc, rel in (("zh", ZH_REL), ("en", EN_REL)):
            p = os.path.join(root, rel)
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "w", encoding="utf-8") as f:
                f.write(pages[loc])
            print("  已生成 %s（%d 字节）" % (rel, len(pages[loc].encode("utf-8"))))
        d = pages["data"]
        print("  命令树 %d 条 = 已开放 %d + 危险动作 %d · 退码 %d 格 · 用法串 %d 条"
              "（其中 %d 条与 `zerg help` 的用法块逐字一致）"
              % (len(d["names_tree"]), d["n_open"], d["n_danger"], len(d["exit"]), len(d["usage"]),
                 d["n_help_agree"]))
        errs = check(root, pages)
        if errs:
            print("⇒ 生成后立刻复判就有问题（rc=1）：")
            for e in errs:
                print("  ✗ " + e)
            return RC_FAIL
        print("⇒ 已生成并复判通过（drift 0 · 命令名对拍 0 差）rc=0")
        return RC_OK
    errs = check(root, pages)
    d = pages["data"]
    print("参考-命令行 · 仓根 %s" % root)
    print("  现跑：命令树 %d 条（页 %d 条）· 退码 %d 格 · 用法串 %d 条（%d 条与 `zerg help` 逐字一致）"
          % (len(d["names_tree"]), len(d["names_page"]), len(d["exit"]), len(d["usage"]),
             d["n_help_agree"]))
    print("  命令名对拍（页 ⟷ %s）：%s" % (MAIN_REL,
                                        "0 差" if set(d["names_page"]) == set(d["names_tree"]) else "有差"))
    for loc, rel in (("zh", ZH_REL), ("en", EN_REL)):
        same = os.path.isfile(os.path.join(root, rel)) and read(os.path.join(root, rel)) == pages[loc]
        print("  %-28s drift=%s" % (rel, "无" if same else "**有**"))
    if errs:
        print("⇒ 判红（rc=1）：逐条看上面")
        for e in errs:
            print("  ✗ " + e)
        return RC_FAIL
    print("⇒ 全过（rc=0）：产物与现跑逐字节一致 · 命令名与命令树 0 差")
    return RC_OK


if __name__ == "__main__":
    sys.exit(main())

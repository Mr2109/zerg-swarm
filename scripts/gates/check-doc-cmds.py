#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-cmds.py — 门⑫ · 文档-命令一致性（M12 · §九 M12 + 开发文档门禁 D3 · 开工单 T-51）。

为什么要有它（§九 M12 逐字口径）
--------------------------------
命令面（`core/cmd/zerg` 的命令树）是**唯一入口**；文档（正式面 `docs/` 与开发文档面 `Zerg-内部文档/`）
里出现的 **`zerg …` 命令示例**如果拼不出命令树里的一条命令，那不是「文档写得糙」，
是**两套契约开始分叉** —— §一 1.2 的病灶同源（9 件 cmd 各写各的、退码四套并存）。
本门把「文档里的 zerg 示例必须能在命令清单里解析到」写成**可机检**的判据：
清单真源 = `bin/zerg --help` 的导出面？不 —— 真源是**命令树源码**（`core/cmd/zerg/main.go`
的 `path: []string{…}` 字面量；帮助导出物是它的下游，M16 的「导出物」口径同一条）。

M12 判到的深度（§九 M12：**判到 L2（对象 + 动作）起步**）
--------------------------------------------------------
  L1  第 1 段（族名/裸动词）：`zerg doctor` → `doctor`
  L2  前 2 段（对象 + 动作）：`zerg dev proposal` → `dev proposal`（**起步深度**，本门判到这里）
  L3+ 第 3 段只放 id/名（§3.1 深度上限）—— 不判（`zerg task show <id>` 的 `<id>` 不是命令名）。

★ 命名词与旗标、旗标取值分开（2026-09-21 修 · 实测的假红）
--------------------------------------------------------
  `CMD_WORD.findall` 会把**旗标名与旗标取值**也当成命令词 —— 实测 `` `zerg version --json name` ``
 抽出 `['version','json','name']` ⇒ L2 = `('version','json')` **不在命令树里** ⇒ **真命令被判红**
 （真仓复跑：`zerg version --json name` / `zerg doctor --quick --json name,verdict` /
 `zerg update <主机名> --confirm=<主机名>` / `zerg ask <提示> [--min-ctx n]` 四条真示例全红 ——
 最后那条漏的是**方括号里的旗标取值** `n]`，故旗标词正则也认 `[`/`(` 前缀）。
 这不是「文档写错」，是**判据把数据当命令**：
  判据要的是「命令名能不能解析」，旗标（`-` 开头）与紧跟旗标的取值不是命令名 ⇒
  `command_words()` 先剥掉它们再判。
  另一处同族：`zerg help <主题>` —— 第二段是**主题数据**（`core/cmd/zerg/topics.go`，
  不在命令树里）⇒ 只判族名 `help` 在树里（族名不在 ⇒ 照旧红）。
  两处都带成对负控（见 `--self-test` ⑦⑧⑨：好例带旗标 ⇒ 绿；坏命令名照旧 ⇒ 红）。

★ 身份行 / 非命令噪声：抽出前先认 + 抽出后按命令词合法性过滤（2026-09-24 治本 · 缺口 `Q-152`）
--------------------------------------------------------------------------------------
  正式面那行抬头是**元数据**，不是命令示例：
      > 身份 / identity：zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z
  它只要被**行内反引号**包住，`ZERG_INLINE` 就会把 `zerg 2.5.11 ab1ac075+dirty …` 整串当命令示例抽出，
  于是**同一形态两种命**（都不是「认出来了」）：
    · sha 首位是**字母** ⇒ 抽出 `['ab1ac075']` ⇒ 前两段/首段都不在命令树 ⇒ **假红** ✗；
    · sha 首位是**数字** ⇒ 一个命令词都抽不出 ⇒ 落进 `if not words` ⇒ **静默放过** ⇒ 长期潜伏 ✗。
  治法两件并用，各自独立成立：
    ① **抽出前先认**（`IDENTITY_SEG`）：把 `zerg [<版本号>] <sha>[+dirty] <时刻>` 这一段**连同它外面的
       反引号**整段摘掉 —— 只摘**这一段**、**不整行跳过** ⇒ 同一条行上别的反引号代码照旧判
       （禁「认了身份行就把整行放过」✗：那会把同一行上的真命令一起放过）。
    ② **抽出后按命令词合法性过滤**（`segment_words`）：`zerg` 紧跟的**第一段**既不是命令词、也不是旗标
       ⇒ 整串不是命令示例（版本号 / sha / 时刻 / 占位符 / 路径都属这一类）—— 这一条把「抽不出词 ⇒ 静默」
       的口也堵上：丢的一定是**认出来的非命令**，而不是空串。
  其余判据**一字未改**（L2 起步深度 · `help <主题>` 只判族名 · 旗标与旗标取值剥离）；
  被跳过的两类**逐类计数并打印**（`识别跳过：…`）。
  照实登记的**未覆盖形态**：`zerg <sha>+dirty`（带 sha 但**无时刻**、且 sha 首位是字母）仍按命令判 ——
  今天正式面用的是`版本号 + sha + 时刻`形态，这一形态本件**不覆盖**、不假装全覆盖 ✗。

★ 缺件/空转 ⇒ **不给结论档（rc=2）**：三条语料分支一律走 BLOCKED（2026-09-24 治本 · 缺口 `Q-157`）
------------------------------------------------------------------------------------------
  三条分支 = ① formal 的 `docs/` **整棵不在** · ② formal 的 `docs/` 在但**扫到 0 篇** ·
             ③ devdocs 面**扫到 0 篇**。
  改前三条都走 `errs.append` ⇒ 经末行 `return (RC_FAIL if errs else RC_OK)` **恒落 rc=1**，
  与本文件头「退码」那一行（缺件/空转 ⇒ **2**）及 K3 自述不符 —— 「**判据不可判**」被记成
  「**有阻断红**」是两种命：读的人会去改文档，而真问题是语料不在。
  改后三条一律进 `blocked` ⇒ **先于 `errs`** 判 `RC_BLOCKED`（`2` 是**既有**那一档，**不新立码** ✗）。
  成对负控在本件 `--self-test` ⑮/⑯/⑰ 三格：每格都是「这一条分支给 2」＋「同一条分支在语料
  真在且非空时**照旧** 0/1」—— 不许为了拿到 2 而把红判没了。
  另一条**刻意不动**：devdocs 根**整棵不在**（`../Zerg-内部文档/…` 在公开树里天生没有）仍走
  「本机判不了这一棵（不红、给 REPORT 0）」那一行 —— 它是登记在案的既有口径，**不在**本次三格内 ✗。

两棵语料（§二 2.3 文档体系 + §6.4 D3 的接缝）
---------------------------------------------
  formal  = 正式面 `docs/**`（**手写命令示例在这里应当是阻断**；今天仓内 0 命中 ⇒ 一上来就能挂阻断档）
  devdocs = 开发文档面 `Zerg-内部文档/项目文档/v2.5.10/`（**先只报告档起步**：1250+ 篇历史稿，
            一次全红等于锁死提交闸 —— 照 D2 先例与《待办 D3》② 的节奏，**清到 0 再升 `tri`**）

两棵的档位**不许一样**：formal 阻断 + devdocs 只报告 —— 这个差本身就是 D3 的落地节奏
（《待办 D3》四步：①换根 ②先挂只报告 ③逐类收敛 ④升阻断）。

tri-report 语义（§6.3 S6 的「起步 tri-report + baseline 棘轮」）
--------------------------------------------------------------
  devdocs 侧的命中走 **REPORT**（只入清单、不进失败项、不影响退出码）；命中数受
  `scripts/gates/doc-cmds-baseline.json` 的 **baseline 棘轮**约束：只许下调、上调 = 红。
  formal 侧走 **tri**（rc=1 阻断）—— 基线 = 0（今天仓内 0 命中 ⇒ 起步即阻断，不放假绿）。

退码（§4.1 K3，不新立）：0 全过 · 1 有阻断红 · 2 **不给结论**（缺件/空转/判据不可判/自检不过）。
  语料缺件/空转的**三条分支**（`docs/` 不在 · `docs/` 0 篇 · devdocs 0 篇）恒走 **2**（见上 ★ · `Q-157`）。

用法：
  python3 scripts/gates/check-doc-cmds.py                      # 两棵都判（默认档）
  python3 scripts/gates/check-doc-cmds.py --scope formal       # 只判正式面
  python3 scripts/gates/check-doc-cmds.py --scope devdocs      # 只判开发文档面（语料不在主仓 —— 走 ZERG_DEVDOCS_ROOT 或默认 ../Zerg-内部文档/项目文档/v2.5.10）
  python3 scripts/gates/check-doc-cmds.py --self-test          # 成对负控（好件/坏件/空转 + 旗标/主题 + 身份行/噪声 + 语料缺件三条分支）；**组数以末行自印为准**（不写死）
"""
import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_REPO = os.path.dirname(os.path.dirname(HERE))
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
BASELINE_REL = os.path.join("scripts", "gates", "doc-cmds-baseline.json")

RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2

# 排除表：与仓内其它门的扫描口径同一张（grep --exclude-dir 的同款），不留静默的洞。
EXCL = {"target", "node_modules", "dist", "bin", "vendor", "data", ".git",
        ".venv", "venv", ".build", "__pycache__", ".history", ".obsidian"}

# 围栏/行内代码里的 `zerg …` 抽取：
#   ① 围栏块 ```…```（含 ~~~）里的整段
#   ② 行内反引号 `zerg …`
# 只认「zerg 后面跟命令词」的形态；`zerg` 单独出现（如「跑 zerg」）不抽。
ZERG_INLINE = re.compile(r'`zerg(\s+[^\s`][^`]*)?`')
FENCE = re.compile(r'^(```|~~~)')
CMD_WORD = re.compile(r'[A-Za-z][A-Za-z0-9_-]*')
# 旗标词：`-h` / `--json` / `--confirm=<目标>`；**方括号里也算**（用法串把可选旗标写成 `[--json <字段>]`，
# 那种写法后面跟的取值同样是取值 ⇒ 认它才能把值剥掉。实测：`zerg ask <提示> [--min-ctx n]` 的 `n]`
# 曾是唯一漏网的那个取值（抽成 `('ask','n')` ⇒ 假红）。
FLAGWORD = re.compile(r'^[\[\(]*-{1,2}[A-Za-z]')
# 身份行那一段（含可选版本号 · sha（可带 `+dirty`）· ISO 时刻）—— 见文件头「身份行 / 非命令噪声」①。
#   把这一段连同它外面的反引号整段摘掉；**不整行跳过**（同行其余代码照旧判）。
IDENTITY_SEG = re.compile(
    r'`?zerg\s+'
    r'(?:[0-9]+\.[0-9]+[0-9A-Za-z.+\-]*\s+)?'            # 可选：版本号（`2.5.11`）
    r'[0-9a-fA-F]{7,64}(?:\+dirty)?\s+'                   # sha（可带 `+dirty`）
    r'[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9:]{5,8}Z?'       # 时刻（ISO8601）
    r'[^`]*`?')
# 版本号形态：`2.5.11` / `2.5.11-rc1`（治法② 用它把「版本串」与「其它噪声」分开计数）。
VERSION_TOKEN = re.compile(r'^[0-9]+(?:\.[0-9]+)+[0-9A-Za-z.+\-]*$')


def segment_words(seg):
    """一段 `zerg …` 之后的行文 → (类别, 命令词)；类别 ∈ {'cmd','identity','noise'}（治法②）。

    判据（2026-09-24 · 治本）：`zerg` 紧跟的**第一段**既不是命令词、也不是旗标
      ⇒ 整串**不是命令示例**（版本号 / sha / 时刻 / 占位符 / 路径都属这一类）。
    这一类**一律丢**（返回 `[]`），但**按类别计数** —— 丢的是「认出来的非命令」，不是「抽不出来的空串」；
    修前那种「sha 首位是数字 ⇒ 抽不出词 ⇒ 落进 `if not words` 静默放过」的口就是这么堵上的。
    第一段是**旗标**（`zerg --json …`）的串**照旧走既有旗标剥离那套**（不在这里改判、不放宽）。
    """
    toks = seg.split()
    first = toks[0] if toks else ""
    if first and not CMD_WORD.match(first) and not FLAGWORD.match(first):
        return ("identity" if VERSION_TOKEN.match(first) else "noise"), []
    return "cmd", command_words(seg)


def count_skip(stats, kind):
    """两类「认出来才丢」的段数分别计数（`stats` 为 None 时只当没传）。"""
    if stats is None or kind == "cmd":
        return
    stats[kind] = stats.get(kind, 0) + 1


def command_words(seg):
    """一段 `zerg …` 之后的行文 → **命令词**（剥掉旗标与**紧跟旗标的取值**）。

    为什么必须剥（实测的假红，2026-09-21）：`CMD_WORD.findall` 会把旗标名与旗标取值也算成命令词 ——
    `` `zerg version --json name` `` ⇒ `['version','json','name']` ⇒ L2 = `('version','json')`
    不在命令树里 ⇒ **真命令被判红**。判据要的是「命令名能不能解析到」，旗标与其取值不是命令名。
    """
    out, skip = [], False
    for tok in seg.split():
        if FLAGWORD.match(tok):
            skip = True          # 旗标词本身不进（它的取值在下一个词里）
            continue
        if skip:
            skip = False
            continue             # 紧随旗标的那个词 = 取值，不是命令名
        m = CMD_WORD.match(tok)
        if m:
            out.append(m.group(0))
    return out


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def parse_tree(root):
    """命令树 → {('dev','proposal'): True, …}（L1 与 L2 都登记：L2 命中先查，查不到退化查 L1）。"""
    src = read(os.path.join(root, MAIN_REL))
    try:
        start = src.index('commands = []*command{')
    except ValueError:
        return None
    end = src.index('\n\t}\n', start)
    seg = src[start:end]
    out = {}
    for m in re.finditer(r'path:\s*\[\]string\{([^}]*)\}', seg):
        parts = re.findall(r'"([^"]+)"', m.group(1))
        if not parts:
            continue
        out[tuple(parts)] = True
        out[tuple(parts[:1])] = True          # L1：裸动词/族名也登记
        out[tuple(parts[:2])] = True          # L2：对象 + 动作
    return out


def extract_zerg_segments(text, stats=None):
    """一段 markdown → [(行号, ['dev','proposal','new',…])]（围栏内 + 行内都抽；只抽 zerg 开头）。

    `stats`（可选 · dict）传进来时另计三类**认出来才丢**的数（见文件头「身份行 / 非命令噪声」）：
      `identity_lines` 行级摘掉的身份行段数 · `identity` 段级判为版本串形态的段数 · `noise` 其余非命令噪声。
    """
    hits, in_fence = [], False
    for lineno, line in enumerate(text.splitlines(), 1):
        # 治法①（抽出前先认）：身份行那一段整段摘掉 —— **不整行跳过**，同行其余反引号代码照旧判。
        line, n_id = IDENTITY_SEG.subn("", line)
        if stats is not None and n_id:
            stats["identity_lines"] = stats.get("identity_lines", 0) + n_id
        stripped = line.strip()
        if FENCE.match(stripped):
            in_fence = not in_fence
            continue
        # 围栏块里的示例是文档里 `zerg …` 的主要形态：围栏正文行整行抽（去掉提示符前缀）；
        # 围栏外的普通行只抽行内反引号里的形态（防误伤「跑 zerg」这类叙述句）。
        if in_fence:
            for seg in re.split(r'(?<!\S)(?:\$|PS\s*[A-Za-z]>|>)\s*', stripped):
                m = re.match(r'zerg(\s+\S.*)?$', seg.strip())
                if m:
                    kind, words = segment_words(m.group(1) or "")   # 治法②
                    count_skip(stats, kind)
                    hits.append((lineno, words))
        else:
            for m in ZERG_INLINE.finditer(line):
                kind, words = segment_words(m.group(1) or "")       # 治法②
                count_skip(stats, kind)
                hits.append((lineno, words))
    return hits


def collect_md(root, rel_dir):
    """一棵语料树 → [(relpath, 绝对路径)]（排除表逐字对齐 EXCL）。"""
    base = os.path.join(root, rel_dir)
    if not os.path.isdir(base):
        return None
    out = []
    for dp, dn, fn in os.walk(base):
        dn[:] = [d for d in dn if d not in EXCL]
        for f in fn:
            if f.endswith(".md"):
                ap = os.path.join(dp, f)
                out.append((os.path.relpath(ap, root), ap))
    return out


def judge_md(files, tree, stats=None):
    """→ (阻断命中数, 报告命中数, 明细行)。formal 的命中 = 阻断；devdocs 的命中 = 只报告。"""
    n_block, n_report, details = 0, 0, []
    for rel, ap in files:
        try:
            text = read(ap)
        except OSError as e:
            details.append("读不了 %s：%s" % (rel, e))
            continue
        for lineno, words in extract_zerg_segments(text, stats):
            # 空串 = **已认出**的非命令（身份行/版本串/噪声），不是修前那种「抽不出来 ⇒ 静默」（见文件头 ★）。
            if not words:
                continue
            # L2 优先：前两段能解析 ⇒ 过；否则退化查 L1（裸动词）
            if tuple(words[:2]) in tree or tuple(words[:1]) in tree:
                continue
            n = n_block
            details.append("%s:%d → zerg %s（命令树里没有它；最像的：见 `zerg --help`）"
                           % (rel, lineno, " ".join(words[:2])))
            if n == 0:
                pass
    return n_block, n_report, details


def judge(files, tree, blocking, stats=None):
    n = 0
    details = []
    for rel, ap in files:
        try:
            text = read(ap)
        except OSError as e:
            details.append("读不了 %s：%s" % (rel, e))
            continue
        for lineno, words in extract_zerg_segments(text, stats):
            # 空串 = **已认出**的非命令（身份行/版本串/噪声），不是修前那种「抽不出来 ⇒ 静默」（见文件头 ★）。
            if not words:
                continue
            # L2 优先（§九 M12 起步深度：对象 + 动作）。L1 只救**单段**命令（裸动词如 `zerg doctor`）——
            # 两段里第一段族名对、第二段动作错 ⇒ 必须红（否则 `dev propozal` 这类坏动作被族名放行）。
            # ★ 2026-09-21：`help <主题>` 的第二段是**主题数据**（在 `topics.go`，不在命令树里）
            #   ⇒ 只判族名 `help` 在树里（族名不在 ⇒ 照旧红，不是无脑放行）。
            l2 = tuple(words[:2]) if len(words) >= 2 else None
            l1set = {t[0] for t in tree if len(t) == 1}
            if words[0] == 'help' and words[0] in l1set:
                continue
            if (l2 and l2 in tree) or (len(words) == 1 and words[0] in l1set):
                continue
            n += 1
            details.append("%s:%d → `zerg %s` 不在命令树（L2 判据）" % (rel, lineno, " ".join(words[:2])))
    if blocking:
        return n, 0, details
    return 0, n, details


def load_baseline(root):
    p = os.path.join(root, BASELINE_REL)
    try:
        with open(p, encoding="utf-8") as f:
            d = json.load(f)
        int(d["devdocs_report_hits"])
        return d
    except (OSError, KeyError, ValueError):
        return None


def check(root, scopes):
    if not os.path.isfile(os.path.join(root, MAIN_REL)):
        return RC_BLOCKED, ["⛔ 不给结论：命令树读不出来（%s 缺件/空转）—— 判据不可判 ⇒ 不给结论" % MAIN_REL]
    tree = parse_tree(root)
    if tree is None or not tree:
        return RC_BLOCKED, ["⛔ 不给结论：命令树读不出来（%s 缺件/空转）—— 判据不可判 ⇒ 不给结论" % MAIN_REL]
    baseline = load_baseline(root)
    if baseline is None:
        return RC_BLOCKED, ["⛔ 不给结论：baseline 缺件或解不动（%s）—— 棘轮没法判 ⇒ 不给结论" % BASELINE_REL]
    errs, lines = [], []
    blocked = []                     # 缺件/空转 ⇒ **不给结论档**（rc=2 · 见文件头 ★ · 缺口 `Q-157`）
    devdocs_hits, formal_hits = 0, 0
    detail_all = []
    stats = {}                       # 认出来才丢的两类计数（身份行 / 版本串形态 · 非命令噪声）
    for scope in scopes:
        if scope == "formal":
            files = collect_md(root, "docs")
            if files is None:
                blocked.append("⛔ 不给结论：formal 语料不在（docs/ 整棵缺席 · 语料域缺件）—— 判据不可判")
                continue
            if not files:
                blocked.append("⛔ 不给结论：formal 扫到 0 篇（空转 = 假绿）—— 判据不可判")
                continue
            n, _, details = judge(files, tree, blocking=True, stats=stats)
            formal_hits += n
            if n:
                errs.append("formal（正式面 · **阻断**）：%d 处手写 `zerg …` 示例拼不到命令树（§九 M12）" % n)
        else:
            docs_root = os.environ.get("ZERG_DEVDOCS_ROOT") or os.path.join(
                os.path.dirname(root), "Zerg-内部文档", "项目文档", "v2.5.10")
            files = collect_md(os.path.dirname(docs_root), os.path.basename(docs_root))
            if files is None:
                lines.append("devdocs：开发文档面不在（%s）—— 本机判不了这一棵（不红、给 REPORT 0）" % docs_root)
                continue
            if not files:
                blocked.append("⛔ 不给结论：devdocs 扫到 0 篇（空转 = 假绿）—— 判据不可判")
                continue
            _, r, details = judge(files, tree, blocking=False, stats=stats)
            devdocs_hits += r
            lines.append("devdocs（只报告档 · §6.3 S6 起步）：命中 %d 处（baseline %d）"
                         % (r, int(baseline["devdocs_report_hits"])))
        detail_all.extend(details[:20])
    # 棘轮（devdocs 侧）：只许下调
    if devdocs_hits > int(baseline["devdocs_report_hits"]):
        errs.append("T-51 棘轮：devdocs 命中 %d > baseline %d（只许**下调**）"
                    % (devdocs_hits, int(baseline["devdocs_report_hits"])))
    lines.append("命令树 %d 条（L1+L2）· formal 命中 %d（阻断档）· devdocs 命中 %d（只报告档）"
                 % (len(tree), formal_hits, devdocs_hits))
    lines.append("识别跳过（**不计入**上面两个数）：身份行 / 版本串形态 %d 段 · 非命令噪声 %d 段"
                 " —— ★ 此二类是**认出来了**才丢（≠ 修前那种「抽不出词 ⇒ 静默」）"
                 % (stats.get("identity_lines", 0) + stats.get("identity", 0), stats.get("noise", 0)))
    lines.extend("  " + d for d in detail_all)
    lines.extend(blocked)                    # 缺件/空转：先于红判「不给结论」（`Q-157`）
    for e in errs:
        lines.append("✗ " + e)
    if blocked:
        # 「判据不可判」不是「有阻断红」—— 两种命，两条出口（红那几条仍照原样打印在上面，不藏）。
        return RC_BLOCKED, lines
    return (RC_FAIL if errs else RC_OK), lines


def self_test():
    """成对负控：好件绿 / 坏件红 / 空转 2 —— 先证明这台机器能把红判出来。"""
    import tempfile
    bad = []
    ran = []                     # 跑过的格数（末行「N 组」**自印**，不写死 —— 免得像 `Q-158` 那样过期）
    tmp = tempfile.mkdtemp(prefix="doc-cmds-selftest-")
    os.makedirs(os.path.join(tmp, "core", "cmd", "zerg"), exist_ok=True)
    os.makedirs(os.path.join(tmp, "scripts", "gates"), exist_ok=True)
    tree_src = (
        "var commands []*command\n"
        "func init() {\n\tcommands = []*command{\n"
        '\t\t{path: []string{"version"}, run: cmdVersion},\n'
        '\t\t{path: []string{"dev", "proposal"}, run: cmdDevProposal},\n'
        # ★ 2026-09-21：⑦/⑨ 两组控要用带旗标/带主题的**真命令名** ⇒ 夹具树补这几条
        '\t\t{path: []string{"help"}, run: cmdHelp},\n'
        '\t\t{path: []string{"doctor"}, run: cmdDoctor},\n'
        '\t\t{path: []string{"update"}, run: cmdUpdate},\n'
        '\t\t{path: []string{"agent", "logs"}, run: cmdAgentLogs},\n'
        '\t\t{path: []string{"task", "ls"}, run: cmdTaskLs},\n'
        "\t}\n}\n"
    )
    open(os.path.join(tmp, MAIN_REL), "w", encoding="utf-8").write(tree_src)
    json.dump({"devdocs_report_hits": 0, "note": "selftest"},
              open(os.path.join(tmp, BASELINE_REL), "w", encoding="utf-8"))

    def case(name, want, got):
        ran.append(name)
        if want != got:
            bad.append("%s：want rc=%s got rc=%s" % (name, want, got))
        print("  %s ⇒ rc=%s" % (name, got))

    def id_skips(lines_):
        """读现跑输出里那行「身份行 / 版本串形态 N 段」—— 认出来了才丢的**证据**（≥1 才算认过）。"""
        for l in lines_:
            m = re.search(r'身份行 / 版本串形态\s*(\d+)\s*段', l)
            if m:
                return int(m.group(1))
        return -1

    # ① 正控：好件（示例拼得到命令树）⇒ 0
    docs = os.path.join(tmp, "docs")
    os.makedirs(docs, exist_ok=True)
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "跑这个：\n```sh\nzerg dev proposal list\n```\n行内 `zerg version` 也过。\n")
    rc, _ = check(tmp, ["formal"])
    case("① 正控（好件 ⇒ 0）", 0, rc)

    # ② 负控：坏件（拼不到）⇒ 1（formal 阻断）
    open(os.path.join(docs, "bad.md"), "w", encoding="utf-8").write(
        "```sh\nzerg dev propozal list\n```\n")
    rc, lines = check(tmp, ["formal"])
    case("② 负控（坏件 ⇒ 1）", 1, rc)
    if not any("propozal" in l for l in lines):
        bad.append("② 负控：判词里没有坏命令名")

    # ③ 空转：命令树读不出来 ⇒ 2（不给结论，不许当绿）
    os.rename(os.path.join(tmp, MAIN_REL), os.path.join(tmp, MAIN_REL) + ".bak")
    rc, _ = check(tmp, ["devdocs"])
    case("③ 空转（缺命令树 ⇒ 2）", 2, rc)
    os.rename(os.path.join(tmp, MAIN_REL) + ".bak", os.path.join(tmp, MAIN_REL))

    # ④ 负控：baseline 缺件 ⇒ 2
    os.rename(os.path.join(tmp, BASELINE_REL), os.path.join(tmp, BASELINE_REL) + ".bak")
    rc, _ = check(tmp, ["devdocs"])
    case("④ 负控（baseline 缺 ⇒ 2）", 2, rc)
    os.rename(os.path.join(tmp, BASELINE_REL) + ".bak", os.path.join(tmp, BASELINE_REL))

    # ⑤ devdocs 只报告档：坏件 ⇒ rc 仍 0（命中走 REPORT，不进退码）——
    #    但棘轮（baseline=0 · 命中 1）要把它拉红 ⇒ 判 1（棘轮有牙）
    devdocs = os.path.join(tmp, "devdocs")
    os.makedirs(devdocs, exist_ok=True)
    open(os.path.join(devdocs, "bad.md"), "w", encoding="utf-8").write(
        "```sh\nzerg dev propozal list\n```\n")
    os.environ["ZERG_DEVDOCS_ROOT"] = devdocs
    rc, lines = check(tmp, ["devdocs"])
    del os.environ["ZERG_DEVDOCS_ROOT"]
    case("⑤ devdocs 命中超 baseline ⇒ 棘轮红（1）", 1, rc)

    # ⑥ 棘轮上调放行（baseline=1 · 命中 1）⇒ 回绿（证明红红在棘轮那格，不是红在夹具）
    json.dump({"devdocs_report_hits": 1, "note": "selftest"},
              open(os.path.join(tmp, BASELINE_REL), "w", encoding="utf-8"))
    os.environ["ZERG_DEVDOCS_ROOT"] = devdocs
    rc, _ = check(tmp, ["devdocs"])
    del os.environ["ZERG_DEVDOCS_ROOT"]
    case("⑥ devdocs 命中 ≤ baseline ⇒ 0（棘轮回绿）", 0, rc)

    # ⑦ 正控（修前的**假红** · 2026-09-21）：真命令带旗标与旗标取值 ⇒ 必须绿
    #    （修前 `zerg version --json name` 抽成 ['version','json','name'] ⇒ L2=('version','json') ⇒ 假红）
    os.remove(os.path.join(docs, "bad.md"))
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "```sh\nzerg version --json name\n$ zerg doctor --quick --json name,verdict\n"
        "zerg update <主机名> --confirm=<主机名>\nzerg agent logs <机器名> --json machine\n"
        "zerg doctor [--min-ctx n]\n```\n"
        "行内 `zerg task ls --json id,status` 也过。\n")
    rc, lines = check(tmp, ["formal"])
    case("⑦ 正控（真命令带旗标/旗标取值 ⇒ 0；修前是假红）", 0, rc)
    if rc:
        bad.append("⑦ 正控：旗标与旗标取值仍被当成命令词 —— 判词 %s" % (" | ".join(lines[:4]),))

    # ⑧ 负控：**剥旗标不许把区分度剥没了** —— 坏命令名带旗标照旧红
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "```sh\nzerg dev propozal --json name\n```\n")
    rc, lines = check(tmp, ["formal"])
    case("⑧ 负控（坏命令名 + 旗标 ⇒ 1）", 1, rc)
    if not any("propozal" in l for l in lines):
        bad.append("⑧ 负控：判词里没有坏命令名")

    # ⑨ 正控：`help <主题>`（主题表在 `topics.go`，**不在**命令树里）⇒ 0（修前是假红）；
    #    族名错 ⇒ 仍红（族名判据一字未改 —— 见 ⑧ 的同一分支）
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "退码表见 `zerg help exit-codes`，输出契约见 `zerg help contract`。\n")
    rc, lines = check(tmp, ["formal"])
    case("⑨ 正控（`help <主题>` ⇒ 0；修前是假红）", 0, rc)
    if rc:
        bad.append("⑨ 正控：`help <主题>` 仍被判红 —— 判词 %s" % (" | ".join(lines[:4]),))

    # ── 身份行 / 非命令噪声（2026-09-24 · 治本 · 缺口 `Q-152`）──────────────────
    # ⑩ 正控：身份行包在**行内反引号**里 · sha 首字符是**字母** ⇒ 改前抽出 ['ab1ac075'] ⇒ **假红**
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`\n")
    rc, lines = check(tmp, ["formal"])
    case("⑩ 正控（身份行 · 行内反引号 · sha 首字母 ⇒ 0；修前是假红）", 0, rc)
    if id_skips(lines) < 1:
        bad.append("⑩ 正控：身份行没被**认出来**（识别计数 = 0）—— 换个 sha 值就会两态摇摆")

    # ⑪ 正控：同一形态 · sha 首字符是**数字** ⇒ 改前 rc 也是 0，但那是落进 `if not words` 的**静默放过**。
    #    本格断言的不只是 rc=0，还有「**认出来了**」（识别计数 ≥ 1）—— 潜伏口靠这一格才堵上。
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "> 身份 / identity：`zerg 2.5.11 54a3dc51+dirty 2026-09-23T17:53:17Z`\n")
    rc, lines = check(tmp, ["formal"])
    case("⑪ 正控（身份行 · sha 首数字 ⇒ 0）", 0, rc)
    if id_skips(lines) < 1:
        bad.append("⑪ 正控：sha 首数字那一格仍是**静默**的（识别计数 = 0）⇒ 潜伏口没堵上")

    # ⑫ 正控：**无时刻**的「版本号 + sha」形态 —— `IDENTITY_SEG`（治法①）不匹配它
    #    ⇒ 管这一格的只有治法②（命令词合法性）⇒ 这一格证明**治法② 独立成立**（两条治法互为备份）
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "> 版本 / version：`zerg 2.5.11 ab1ac075+dirty`\n")
    rc, lines = check(tmp, ["formal"])
    case("⑫ 正控（版本号 + sha · 无时刻 ⇒ 0 · 治法② 独立成立）", 0, rc)
    if id_skips(lines) < 1:
        bad.append("⑫ 正控：治法② 那一格没认出来（识别计数 = 0）")

    # ⑬ 负控：身份行 + **同一行上**另一处真错误 ⇒ 必须红且点名 ——
    #    证明治法① **只摘那一段、不整行跳过**（整行跳过会把同一行上的真命令一起放过）
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`"
        " · 另见 `zerg dev propozal list`\n")
    rc, lines = check(tmp, ["formal"])
    case("⑬ 负控（身份行同一行上的真错误 ⇒ 1 · 治法① 不整行跳过）", 1, rc)
    if not any("propozal" in l for l in lines):
        bad.append("⑬ 负控：判词里没有坏命令名")

    # ⑭ 负控：身份行与真错误**同篇**（围栏里的坏命令）⇒ 照旧红 —— 认噪声不许把真命令一起放过
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "> 身份 / identity：`zerg 2.5.11 ab1ac075+dirty 2026-09-23T17:53:17Z`\n\n"
        "```sh\nzerg dev propozal list\n```\n")
    rc, lines = check(tmp, ["formal"])
    case("⑭ 负控（身份行 + 同篇真错误 ⇒ 1）", 1, rc)
    if id_skips(lines) < 1:
        bad.append("⑭ 负控：身份行没被认出来")
    if not any("propozal" in l for l in lines):
        bad.append("⑭ 负控：判词里没有坏命令名")

    # ── 语料三条分支：缺件/空转 ⇒ **不给结论档 rc=2**（2026-09-24 · 治本 · 缺口 `Q-157`）────────
    # 改前这三条走 `errs` ⇒ 末行 `RC_FAIL if errs else RC_OK` ⇒ **恒 rc=1**（「判据不可判」被记成
    # 「有阻断红」）；改后一律进 `blocked` ⇒ 先于 `errs` 判 `RC_BLOCKED`（`2` 是既有那档，**不新立码**）。
    # 每格都带**成对负控**：同一条分支在语料**真在且非空**时照旧 0 / 1 —— 不许为了拿 2 把红判没了。
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "行内 `zerg version` 一行好件。\n")            # 先把 ⑭ 留下的件换成干净好件（免得串味）
    rc, _ = check(tmp, ["formal"])
    case("⑮b 成对负控（formal · 语料在且好件 ⇒ 0）", 0, rc)

    # ⑮ formal · `docs/` **整棵不在** ⇒ 2（改前 rc=1）
    os.rename(docs, docs + ".bak")
    rc, lines = check(tmp, ["formal"])
    case("⑮ 缺件（formal · docs/ 不在 ⇒ 2；改前 rc=1）", 2, rc)
    if not any("docs/ 整棵缺席" in l for l in lines):
        bad.append("⑮ 缺件：判词里没说清「docs/ 整棵缺席」")
    os.rename(docs + ".bak", docs)

    # ⑯ formal · `docs/` **在但扫到 0 篇** ⇒ 2（改前 rc=1）；成对负控 = 塞坏件回去 ⇒ 照旧 1
    holder = os.path.join(tmp, "good.md.hold")
    os.rename(os.path.join(docs, "good.md"), holder)
    rc, _ = check(tmp, ["formal"])
    case("⑯ 空转（formal · docs/ 在但 0 篇 ⇒ 2；改前 rc=1）", 2, rc)
    open(os.path.join(docs, "bad.md"), "w", encoding="utf-8").write("```sh\nzerg dev propozal list\n```\n")
    rc, _ = check(tmp, ["formal"])
    case("⑯b 成对负控（0 篇那棵塞回**坏件** ⇒ 照旧 1）", 1, rc)
    os.remove(os.path.join(docs, "bad.md"))
    os.rename(holder, os.path.join(docs, "good.md"))

    # ⑰ devdocs · **扫到 0 篇** ⇒ 2（改前 rc=1）；成对负控 = 同一根塞回好件 ⇒ 0
    empty_dev = os.path.join(tmp, "devdocs-zero")
    os.makedirs(empty_dev, exist_ok=True)
    os.environ["ZERG_DEVDOCS_ROOT"] = empty_dev
    rc, _ = check(tmp, ["devdocs"])
    case("⑰ 空转（devdocs · 0 篇 ⇒ 2；改前 rc=1）", 2, rc)
    open(os.path.join(empty_dev, "good.md"), "w", encoding="utf-8").write("行内 `zerg version` 一行好件。\n")
    rc, _ = check(tmp, ["devdocs"])
    case("⑰b 成对负控（devdocs · 0 篇那根塞回好件 ⇒ 0）", 0, rc)
    del os.environ["ZERG_DEVDOCS_ROOT"]

    if bad:
        print("自检失败：")
        for b in bad:
            print("  ✗ " + b)
        return 2
    print("门⑫ 自检全过（%d 组成对/正控/空转/语料缺件三条分支/身份行噪声负控）" % len(ran))
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True,
                                 description="门⑫ · 文档-命令一致性（M12 + D3 · T-51）")
    ap.add_argument("--scope", action="append", default=[], choices=["formal", "devdocs"],
                    help="语料域（可重复；缺省 = 两棵都判）")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = os.path.abspath(DEFAULT_REPO)
    if not os.path.isfile(os.path.join(root, MAIN_REL)):
        print("⛔ 不给结论：找不到仓根（%s 不在）" % MAIN_REL)
        return RC_BLOCKED
    scopes = a.scope or ["formal", "devdocs"]
    rc, lines = check(root, scopes)
    print("门⑫ 文档-命令一致性（§九 M12 · §6.4 D3 · 开工单 T-51）· 仓根 %s" % root)
    for l in lines:
        print("  " + l)
    if rc == RC_OK:
        print("⇒ 全过（rc=0）")
    elif rc == RC_FAIL:
        print("⇒ 判红（rc=1）：逐条看上面的命中")
    else:
        print("⇒ 不给结论（rc=2）：缺件/空转 —— **不许当绿**")
    return rc


if __name__ == "__main__":
    sys.exit(main())

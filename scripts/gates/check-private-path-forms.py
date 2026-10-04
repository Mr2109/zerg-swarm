#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""门禁：**过滤 / 脱敏面的形态枚举 + 成对负控**（私有路径 / 地址的「形态」不许只认一两个字面量）。

令（任务单 `任务清单-本版全做-v2.5.12-20260924.md` §一 序136 = 组4 §二.4 `W-54` ·
`研-禁:139` `R-26` · 与组3 `Q-062` 同条）逐字：

    过滤 / 脱敏面判据要**枚举** + 带**成对负控**（`hasHomePath` 只认 `/Users/` · `/home/`
    两个字面量 ⇒ `~/` 形态静默漏脱敏）

验收判据（该行逐字）：判据栏 = 「判据**枚举全部形态**（`~` / `$HOME` / 绝对）；**成对负控**：
造 `~/` 形态 ⇒ 判红」。

为什么这条要成门（病理，不是洁癖）
----------------------------------
`core/internal/obs/redact/redact.go` 的 `hasHomePath` 是 `reHomeDir` 的**必要条件门**（`Contains`
换掉正则 = 「安全的快路径」与「会静默丢覆盖的启发式」的分界线）。它只要**少认一个形态**，
那一种写法就**整段跳过脱敏** —— 而**漏遮是不报错的**：日志里那行照样出、没一处红。
v2.5.11 已经补了第三个字面量 `~/`（该件注释逐字留痕）；本门要管的是**下一次**：
**判据面**（`scripts/gates/**` 与发布链）必须按**形态**枚举，而不是按**当时见过的实例**枚举 ——
`~` 是**实例**，`/Users/<user>` 才是**形态**；实例口径下换一台机器/换一个用户名就整段漏。

判据（四条 · 逐条可机检）
------------------------
- **E1 出处可核**：本门形态表每一格的「出处」是一句**逐字句**，必须能在它点名的真源件里
  `grep -F` 命中（`命中 = 0` 或 `>1` 都算**红**）—— **出处不许手写记忆**。
- **E2 枚举齐全（两个方向）**：
  - E2a **规则 → 形态**：`publish/replace-rules.tsv` 的**每一条**替换规则（跳过注释行、跳过
    `!夹具` 隔离行）的 pattern 必须被**至少一格**的**认领正则**命中；否则 ⇒ 红（**枚举不全**）。
  - E2b **脱敏器 → 形态**：从 `core/internal/obs/redact/redact.go` 的 `hasHomePath`（`Contains`
    字面量）与 `reHomeDir`（正则顶层分支）**现读抽出**的每个形态，必须被至少一格认领；否则 ⇒ 红。
  - ★ **E2a/E2b 判红前先问一句「它是不是已登记的『非形态』」**（缺口 `GAP-20260926-152`）：
    E2a/E2b 原先把**任何**未被形态格认领的规则 / 脱敏器形态都判红，**分不清**「真泄漏形态
    未登记」（该红）与「公开示例值 / 规则模式未登记」（不该红）⇒ 要么放过真形态、要么对公开
    示例误红。治法（照本门 E3「覆盖 or 登记」**同范式**）：给 E2a/E2b 补一条**登记腿** ——
    逐字登记在 `NON_FORM_RULES` / `NON_FORM_FRAGMENTS`（带理由 + 日期）里的模式 ⇒ 进 `nonform[]`
    **逐跑打印**（不静默）、**不当红**；**未登记**的仍是红（`真形态` 负控照旧有牙）。
    ★ 两表**默认空** ⇒ 与旧行为**逐字同形**（未登记 ⇒ 照旧红）；只增「可登记」这一口子，
    不改任何既有判据（既有 16 格自检全过）。
- **E3 覆盖 or 登记**：形态表每格的「门禁侧」列，要么点名一个**在盘**的判据件且该件里
  `grep -F` 命中该格的**探针字面**；要么逐字写「不覆盖 + 理由 + 日期」。两样都没有 ⇒ **红**
  （点名「空指」或「无覆盖且无理由」）。
- **E4 实例 ≠ 形态（只打不红 · 进 `meta`）**：门禁侧探针若含**具体用户名 / 主机名**
  （`/Users/<字母开头>` 而不是 `/Users/<user>` 这种占位）⇒ 计「实例口径」1 条并**逐条打印** ——
  它今天能过不代表明天能过（这正是 `~/` 那一格的来源）。

2026-09-26 补格（本笔 · 形态枚举缺口收口）
--------------------------------------
`publish/replace-rules.tsv` 另有 9 条形态**没有任何格认领**（门跑出来是 E2a 红 9 处）。逐处定档两条路：
**有新规可依的 ⇒ 加认领格**（F-13「私有仓名（仓外私有文档仓名 `Zerg-内部文档`）」—— 真源判据件
`check-public-face-wording.py` 甲档 A4 的正则**就是** `Zerg-内部文档`；★ 它**不是凭据形态**，
不许归进令牌 / 口令类）；**无规可依的 ⇒ 照 F-02 / F-03 / F-09 / F-12 先例逐字登记为存量债**
（F-14 模型代号大小写变体 · F-15 派生标识符 / 环境变量 · F-16 家目录 base64 编码变形）。
★ **绝不为消红而静默、绝不放宽 E1–E4 任何判据**：债务格照样进 `debt[]` **逐跑打印**。
（注：E1 的 F-11 那处红**不是本门能治的** —— 它要改 `publish/replace-rules.tsv`
的注释列，而那是**禁面 + 可能进指纹** ⇒ 本笔**不碰**，如实留红。）

退码：`0` 四判据全过（**未覆盖的形态已逐条登记 + 打印** = 存量债**可见**）/ `1` 有红（逐条点名
「族号 + 件 + 行 + 原样」）/ `2` **不给结论**（真源件缺 · 空转 · 自检不过）。

用法
----
    python3 scripts/gates/check-private-path-forms.py [仓根] [--json] [--self-test] [--list-forms] [--baseline] [--list-unclaimed-only]

`--json`：机器读面。`--list-forms`：逐条列形态表（族号 / 认领正则 / 出处 / 门禁侧）。
"""

import argparse
import json
import os
import re
import sys
import tempfile

SCHEMA = "zerg-gate/1"
KIND = "PrivatePathForms"

REPLACE_RULES = "publish/replace-rules.tsv"
REDACT_GO = "core/internal/obs/redact/redact.go"

# ── 形态表（**本门判据的真源登记面** · 逐格带出处 + 门禁侧）─────────────────────────────
# 字段：
#   id       族号（稳定 · 对账用）
#   name     形态名（人面）
#   claim    认领正则（判「真源里这条规则 / 这个形态归我」）
#   src_file 出处件（相对仓根）
#   src_probe 出处**逐字句**（必须能在 src_file 里 grep -F 命中 1 次）
#   gate_file 门禁侧件（相对仓根；None ⇒ 必须给 debt）
#   gate_probe 门禁侧**探针字面**（必须能在 gate_file 里 grep -F 命中）
#   debt    gate_file 为 None 时的逐字理由 + 日期（**打印**，不静默）
FORMS = [
    dict(id="F-01", name="家目录·绝对（POSIX）",
         claim=r"Users|home",
         src_file=REDACT_GO, src_probe='strings.Contains(s, "/Users/")',
         gate_file="scripts/gates/check-hardcoded-private-paths.py", gate_probe="~",
         debt=None),
    dict(id="F-02", name="家目录·波浪号（`~/…`）",
         claim=r"~/",
         src_file=REDACT_GO, src_probe='strings.Contains(s, "~/")',
         gate_file=None,
         debt="现读（2026-09-24）：`scripts/gates/**` 面**没有一件**把 `~/` 当**泄漏形态**枚举"
              "（`check-hardcoded-private-paths.py` 的 NEEDLES 只有 3 个字面量、全是绝对路径实例）"
              "⇒ 登记为**存量债**（脱敏器侧已修：`hasHomePath` 第三个字面量 `~/` 是 v2.5.11 加的；"
              "**门禁侧那一半未落** ⇒ 本格不许悄悄绿，逐跑打印）"),
    dict(id="F-03", name="家目录·变量（`$HOME` / `${HOME}`）",
         claim=r"\$\{?HOME",
         src_file=REDACT_GO, src_probe="它不做 `$` 展开",
         gate_file=None,
         debt="现读（2026-09-24）：脱敏器自己**逐字声明不做 `$` 展开**（出处逐字见左栏 · `redact.go:381`）"
              "⇒ `$HOME` 形态**不在本版两面的判据面内**（不是漏，是**显式不覆盖**）；"
              "登记在此以免日后被读成「漏项」"),
    dict(id="F-04", name="私有卷（`/Volumes/<卷>`）",
         claim=r"/Volumes/",
         src_file=REPLACE_RULES, src_probe="regex:/Volumes/(AHZ|BXC)",
         gate_file="scripts/gates/check-hardcoded-private-paths.py", gate_probe="<volume-path>",
         debt=None),
    dict(id="F-05", name="私有仓路径（`<volume-path>`）",
         claim=r"<volume-path>",
         src_file=REPLACE_RULES, src_probe="regex:<repo>",
         gate_file="scripts/gates/check-hardcoded-private-paths.py", gate_probe="<volume-path>",
         debt=None),
    dict(id="F-06", name="内网地址（`192.168.*` / `10.0.*`）",
         claim=r"192\.168|10\.0|(?:\d{1,3}\.){3}\d{1,3}",
         src_file=REPLACE_RULES, src_probe="regex:192\\.168\\.\\d{1,3}\\.\\d{1,3}",
         gate_file="scripts/gates/check-public-marks.py", gate_probe="192.168",
         debt=None),
    dict(id="F-07", name="裸用户名（`Mr2109`）",
         claim=r"^Mr2109$",
         src_file=REPLACE_RULES, src_probe="Mr2109\tMr2109",
         gate_file="scripts/gates/check-hardcoded-private-paths.py", gate_probe="Mr2109",
         debt=None),
    dict(id="F-08", name="人物称谓（私有拍板人称谓 / 署名）",
         claim=r"Mr2109|Mr2109|脑\(Hermes\)|Mr2109",
         src_file=REPLACE_RULES, src_probe="Mr2109\tMr2109",
         gate_file="scripts/gates/check-public-face-wording.py", gate_probe="Mr2109",
         debt=None),
    dict(id="F-09", name="主机名（`*.local`）",
         claim=r"\.local",
         src_file=REPLACE_RULES, src_probe="<host>",
         gate_file=None,
         debt="现读（2026-09-24）：`scripts/gates/**` 面无一件枚举主机名形态"
              "（它只在 `publish/replace-rules.tsv` 与发布链里被替换，**没有判据件盯它**）"
              "⇒ 登记为**存量债**（本格逐跑打印）"),
    dict(id="F-10", name="私有项目名",
         claim=r"示例虫茧|<container-repo>|<project-a>|<project-b>",
         src_file=REPLACE_RULES, src_probe="示例虫茧\t示例虫茧",
         gate_file="scripts/gates/check-public-face-wording.py", gate_probe="示例虫茧",
         debt=None),
    dict(id="F-11", name="私有模型代号",
         claim=r"Qwable|example-30b|example-moe-36b|example-35b-v2-|gemma-4-26B",
         src_file=REPLACE_RULES, src_probe="example-8b-quant",
         gate_file="scripts/gates/check-public-face-wording.py", gate_probe="Qwable",
         debt=None),
    dict(id="F-12", name="品牌署名（`Copyright <年> The Zerg Swarm Authors`）",
         claim=r"^regex:Copyright",
         src_file=REPLACE_RULES, src_probe="regex:Copyright \\d{4} The Zerg Swarm Authors",
         gate_file=None,
         debt="现读（2026-09-24）：`scripts/gates/**` 面无一件枚举版权行形态"
              "（发布链自己会改写它，但**没有判据件盯「改写后对不对」**）⇒ 登记为**存量债**（逐跑打印）"),
    # ── 2026-09-26 补格（`publish/replace-rules.tsv` 注释区 9 条未认领形态 · 逐处定档）──────────────
    #   F-13 = **认领格**（**有新规可依**）：`check-public-face-wording.py` 甲档 A4 的正则**就是** `Zerg-内部文档`，
    #          判「仓外私有文档仓名」⇒ 本格归**私有仓名**类。★ 它不是**凭据形态**（不是令牌 / 口令 / 密钥），
    #          登记理由不许写成「凭据」——别把族归错。
    #   F-14 / F-15 / F-16 = **不覆盖 + 登记**（**无规可依**：`scripts/gates/**` 面没有一件判这些形态）
    #          ⇒ 照 F-02 / F-03 / F-09 / F-12 的现成先例逐字登记为存量债（**逐跑打印**，不静默）；
    #          **绝不为消红而放宽 E1–E4 任何判据**，也**绝不**把它们塞进某格 claim 了事 ✗。
    dict(id="F-13", name="私有仓名（仓外私有文档仓名 `Zerg-内部文档`）",
         claim=r"Zerg-内部文档",
         src_file=REPLACE_RULES, src_probe="Zerg-内部文档\tZerg-内部文档",
         gate_file="scripts/gates/check-public-face-wording.py", gate_probe="Zerg-内部文档",
         debt=None),
    dict(id="F-14", name="私有模型代号·大小写变体（小写 `example-moe-36b` / `example-35b-v2` · 大写全长 `example-35b-v2` / `example-35b` · 大写家族前缀 `example-35b-v2-`）",
         claim=r"example-moe-36b|Ornith|example-35b-v2",
         src_file=REPLACE_RULES, src_probe="example-moe-36b\texample-moe-36b",
         gate_file=None,
         debt="现读（2026-09-26）：`scripts/gates/**` 面**无一件**判这些**大小写变体**形态 —— "
              "`check-public-face-wording.py` 的 A10 词表只收全形（`example-8b-quant` / `example-30b` / "
              "`example-moe-36b` / `example-35b` / `example-35b-v2` / `example-26b-review`）、"
              "A11 只收**大写**前缀 `example-moe-36b`，且两条都是 `re.compile(pat)`（**大小写敏感** ⇒ 大写变体不落网）"
              "⇒ 小写 `example-moe-36b` / `example-35b-v2` · 大写全长 `example-35b-v2` / `example-35b` · 大写家族前缀 `example-35b-v2-` "
              "**5 条 tsv 规则没有判据件盯** ⇒ 登记为**存量债**（照 F-02 先例：私有侧规则已在、**门禁侧那一半未落** "
              "⇒ 本格不许悄悄绿，逐跑打印）"),
    dict(id="F-15", name="私有模型代号·派生标识符 / 环境变量（`ExampleAdapter` · `EXAMPLE_TEMPLATE`）",
         claim=r"ExampleAdapter|EXAMPLE_TEMPLATE",
         src_file=REPLACE_RULES, src_probe="ExampleAdapter\tExampleAdapter",
         gate_file=None,
         debt="现读（2026-09-26）：这两条 tsv 规则收的是**代码标识符**（`ExampleAdapter` / `NewExampleAdapter` / "
              "`TestExampleAdapter_*` ⇒ 替换值必须是**合法 Go 标识符**）与**环境变量名**（`ZERG_EXAMPLE_TEMPLATE` ⇒ "
              "`ZERG_EXAMPLE_TEMPLATE`，大写 + 下划线）—— `scripts/gates/**` 面**没有一件判这两种形态**："
              "`check-public-face-wording.py` 判的是**写作面用词**（甲档 12 条，不判标识符 / 环境变量）、"
              "`check-hardcoded-private-paths.py` 的 NEEDLES 全是**绝对路径实例** ⇒ 登记为**存量债**（本格逐跑打印）"),
    dict(id="F-16", name="家目录·base64 编码变形（凭据路径夹具 `L1VzZXJz…`）",
         claim=r"L1VzZXJz",
         src_file=REPLACE_RULES, src_probe="L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
         gate_file=None,
         debt="现读（2026-09-26）：该条 tsv 规则是**家目录形态的 base64 编码变形**（解开 = `<家目录>/.config/zerg/token`，"
              "即**私有机密文件的路径**）—— `scripts/gates/**` 面**没有一件**把「家目录 / 凭据路径的**编码变形**」"
              "当枚举形态：`check-public-marks.py` 的 `PRIVATE_PATTERNS` 收的是**明文** `/Users/<user>/` · "
              "`/home/<user>/` 与令牌**字面**（`ghp_…` / `sk-…` / `AKIA…`），**不收 base64 编码形态** ⇒ "
              "登记为**存量债**（本格逐跑打印）"),
]

# ── 「非形态」登记面（缺口 `GAP-20260926-152` · 2026-10-04）────────────────────────────────
#   病灶（账内逐字）：E2a/E2b 把**任何**未被形态格认领的 `replace-rules.tsv` 规则 / 脱敏器形态
#   判红 ⇒ **分不清**「真泄漏形态未登记」（该红）与「公开示例值 / 规则模式未登记」（不该红）。
#   治法：照本门 E3「覆盖 or 登记」**同范式**，给 E2a/E2b 补一条**登记腿** —— 一条模式若逐字
#   登记在此（带理由 + 日期），即视为「已辨识为**非形态**」⇒ 进 `nonform[]` 逐跑打印（不静默），
#   **不当红**；未登记的仍是红（`真形态` 负控照旧有牙，见自检 ⑪/⑮）。
#   ★ 登记**必须**逐字点明「为何这不是泄漏形态」——不许写成「不覆盖」了事（那是 E3 的债务格口径，
#     不是本表的口径：本表是「已判定它**不是**泄漏形态」，E3 债务是「该覆盖而门禁侧还没覆盖」）。
#   ★ 两表**默认空** ⇒ 行为与旧版**逐字同形**。
NON_FORM_RULES = [
    # (pattern 逐字, 理由 逐字, 日期)
]
NON_FORM_FRAGMENTS = [
    # (origin 逐字, frag 逐字, 理由 逐字, 日期)
]


def _nonform_of(kind, key):
    """该 key 是否已登记为「非形态」⇒ 返回登记条 `(…, reason, date)`；未登记 ⇒ `None`。

    `kind`：`"rule"`（查 `NON_FORM_RULES`，比对 pattern）/ `"fragment"`（查 `NON_FORM_FRAGMENTS`，
    比对 origin+frag）。**逐字**比对（不吃归一化 —— 登记就是要「点名到那一行」）。
    """
    if kind == "rule":
        return next((r for r in NON_FORM_RULES if r[0] == key), None)
    if kind == "fragment":
        return next((r for r in NON_FORM_FRAGMENTS if (r[0], r[1]) == tuple(key)), None)
    return None


# 门禁侧探针含「具体用户名 / 主机名」的判据（E4 只打不红）
INSTANCE_RE = re.compile(r"/(?:Users|home)/[A-Za-z]")


def read_text(root, rel):
    p = os.path.join(root, rel)
    if not os.path.isfile(p):
        return None
    with open(p, "r", encoding="utf-8", errors="replace") as fh:
        return fh.read()


def grep_F_count(text, probe):
    return text.count(probe)


def claim_hit(claim, text):
    r"""认领正则判一次 —— **归一化后**再判一遍（真源里 `\.`/`\\` 是**转义写法**，
    形态格写的是**语义形态**；不归一化就会把「同一形态的两种写法」判成两回事）。"""
    if re.search(claim, text):
        return True
    norm = text.replace("\\.", ".").replace("\\\\", "\\")
    return bool(re.search(claim, norm))


def rules_of(root):
    """现读 `publish/replace-rules.tsv` 的替换规则 pattern 列（跳过注释与 `!夹具` 隔离行）。"""
    text = read_text(root, REPLACE_RULES)
    if text is None:
        return None, []
    out = []
    for idx, line in enumerate(text.splitlines(), start=1):
        s = line.strip()
        if not s or s.startswith("#") or s.startswith("!"):
            continue
        pat = s.split("\t", 1)[0]
        if not pat:
            continue
        out.append((idx, pat))
    return text, out


def redactor_forms(root):
    """现读 `redact.go` 抽出的形态集 —— 两条真源：

    ① `hasHomePath` 的 `strings.Contains(s, "X")` 字面量（**必要条件门** · `Contains` 换正则的地那一步）；
    ② `reHomeDir` 正则的**顶层分支**（按括号深度切 `|`，不切嵌套里的 `|`）。
    """
    text = read_text(root, REDACT_GO)
    if text is None:
        return None, []
    forms = []
    m = re.search(r"func hasHomePath\(.*?\n}", text, re.S)
    if m:
        for lit in re.findall(r'strings\.Contains\(s,\s*"((?:[^"\\]|\\.)*)"\)', m.group(0)):
            forms.append(("hasHomePath", lit))
    m = re.search(r"reHomeDir\s*=\s*regexp\.MustCompile\(`([^`]*)`\)", text)
    if m:
        pat = m.group(1)
        depth = 0
        cur = ""
        for ch in pat:
            if ch in "(?":            # `(?:` 开组：深度在下一个字符处加
                if ch == "(":
                    depth += 1
            elif ch == ")":
                depth -= 1
            if ch == "|" and depth == 0:
                forms.append(("reHomeDir", cur))
                cur = ""
                continue
            cur += ch
        if cur:
            forms.append(("reHomeDir", cur))
    return text, forms


def run(root):
    res = {"rc": 0, "red": [], "debt": [], "nonform": [], "instances": [], "meta": {}}
    text_rules, rules = rules_of(root)
    redact_text, rforms = redactor_forms(root)
    if text_rules is None or redact_text is None:
        missing = [f for f, t in ((REPLACE_RULES, text_rules), (REDACT_GO, redact_text)) if t is None]
        return {"rc": 2, "reason": f"真源件不在盘：{missing}（不给结论）", "red": [], "debt": [], "nonform": [], "instances": [], "meta": {}}
    if not rules:
        return {"rc": 2, "reason": f"{REPLACE_RULES} 里 0 条替换规则（空转 ⇒ 不给结论）",
                "red": [], "debt": [], "nonform": [], "instances": [], "meta": {}}

    # ── E1 出处可核（逐格 · 命中必须恰好 1 次）──────────────────────────────────────
    for f in FORMS:
        txt = read_text(root, f["src_file"])
        if txt is None:
            res["red"].append(dict(rule="E1", form=f["id"], file=f["src_file"], line=None, raw=f["src_probe"],
                                   why="出处件不在盘"))
            continue
        c = grep_F_count(txt, f["src_probe"])
        if c != 1:
            res["red"].append(dict(rule="E1", form=f["id"], file=f["src_file"], line=None, raw=f["src_probe"],
                                   why=f"出处逐字句在该件里命中 {c} 次（必须恰好 1 次）⇒ 出处不许手写记忆"))

    # ── E2a 规则 → 形态（★ `GAP-20260926-152`：判红前先问「是不是已登记的非形态」）──────────
    unclaimed = []
    for lineno, pat in rules:
        hit = [f["id"] for f in FORMS if claim_hit(f["claim"], pat)]
        if not hit:
            reg = _nonform_of("rule", pat)
            if reg is not None:
                res["nonform"].append(dict(rule="E2a", file=REPLACE_RULES, line=lineno, raw=pat,
                                           why="**非形态**（已登记 · %s）：%s" % (reg[2], reg[1])))
                continue
            unclaimed.append((lineno, pat))
            res["red"].append(dict(rule="E2a", form=None, file=REPLACE_RULES, line=lineno, raw=pat,
                                   why="替换规则**没有任何形态格认领** ⇒ 形态枚举不全（换台机器/换个实例就整段漏）"))

    # ── E2b 脱敏器 → 形态（★ 同上：判红前先问「是不是已登记的非形态」）──────────────────────
    orphan_forms = []
    for origin, frag in rforms:
        hit = [f["id"] for f in FORMS if claim_hit(f["claim"], frag)]
        if not hit:
            reg = _nonform_of("fragment", (origin, frag))
            if reg is not None:
                res["nonform"].append(dict(rule="E2b", file=REDACT_GO, line=None,
                                           raw=f"{origin}: {frag}",
                                           why="**非形态**（已登记 · %s）：%s" % (reg[3], reg[2])))
                continue
            orphan_forms.append((origin, frag))
            res["red"].append(dict(rule="E2b", form=None, file=REDACT_GO, line=None, raw=f"{origin}: {frag}",
                                   why="脱敏器枚举到的形态**没有形态格认领** ⇒ 门禁侧不知道要判它"))

    # ── E3 覆盖 or 登记 ─────────────────────────────────────────────────────────
    for f in FORMS:
        if f["gate_file"]:
            txt = read_text(root, f["gate_file"])
            if txt is None:
                res["red"].append(dict(rule="E3", form=f["id"], file=f["gate_file"], line=None, raw=f["gate_probe"],
                                       why="门禁侧点名了一件**不在盘**的判据件 ⇒ **空指**"))
                continue
            c = grep_F_count(txt, f["gate_probe"])
            if c < 1:
                res["red"].append(dict(rule="E3", form=f["id"], file=f["gate_file"], line=None, raw=f["gate_probe"],
                                       why="门禁侧探针在该件里 0 命中 ⇒ 覆盖是名义上的（空指）"))
            if INSTANCE_RE.search(f["gate_probe"]):
                res["instances"].append(dict(form=f["id"], file=f["gate_file"], probe=f["gate_probe"],
                                             why="门禁侧是**实例口径**（写死具体用户名）而不是**形态口径**"
                                                 "（`/Users/<user>`）—— 换机器 / 换用户名即整段漏（E4 只打不红）"))
        else:
            if not f["debt"]:
                res["red"].append(dict(rule="E3", form=f["id"], file=None, line=None, raw="",
                                       why="门禁侧空且**没写理由 + 日期** ⇒ 不许这么放过（要绿必须登记）"))
            else:
                res["debt"].append(dict(form=f["id"], name=f["name"], reason=f["debt"]))

    res["rc"] = 1 if res["red"] else 0
    res["meta"] = {"rules": len(rules), "forms": len(FORMS),
                   "unclaimed_rules": len(unclaimed), "orphan_forms": len(orphan_forms),
                   "debt_forms": len(res["debt"]), "instance_level": len(res["instances"]),
                   "nonform": len(res["nonform"])}
    return res


# ── 自检（**成对负控**：正控 ⇒ rc=0 · must-fail ⇒ rc=1 · 真话负控 ⇒ 回绿）──────────────
def _mk(td, rules_lines, redact_lines, forms=None):
    """合成夹具仓 —— 真源两件 + **形态表点名的每个门禁侧件**（各写入自己的探针字面）。

    为什么门禁侧件也要合成：E3 判的是「该件在盘 **且** 探针字面命中」；夹具里不放它们，
    红的会是「空指」而不是被测的那条判据 ⇒ 自检测不到真东西。
    """
    os.makedirs(os.path.join(td, "publish"), exist_ok=True)
    os.makedirs(os.path.join(td, "core/internal/obs/redact"), exist_ok=True)
    with open(os.path.join(td, REPLACE_RULES), "w", encoding="utf-8") as fh:
        fh.write("\n".join(rules_lines) + "\n")
    with open(os.path.join(td, REDACT_GO), "w", encoding="utf-8") as fh:
        fh.write("\n".join(redact_lines) + "\n")
    stubs = {}
    for f in (forms if forms is not None else FORMS):
        gf, gp = f.get("gate_file"), f.get("gate_probe")
        if gf and gp:
            stubs.setdefault(gf, []).append(gp)
    for gf, gps in stubs.items():
        dst = os.path.join(td, gf)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        with open(dst, "w", encoding="utf-8") as fh:
            fh.write("# 合成门禁侧件（自检夹具 · 只为 E3 的探针命中）\n")
            for gp in gps:
                fh.write(f"NEEDLE = {gp!r}\n")
    os.makedirs(os.path.join(td, "scripts/gates"), exist_ok=True)
    with open(os.path.join(td, "scripts/gates/fake-gate.py"), "w", encoding="utf-8") as fh:
        fh.write("# 合成判据件（自检夹具）\nNEEDLES = (\"~\", \"<volume-path>")\n")


RULES_BASE = [
    "# 合成替换规则（自检夹具）",
    "Mr2109\tMr2109\t称谓",
    "<host>\t<host>",
    "regex:<repo>\t<repo>",
    "regex:/Volumes/(AHZ|BXC)\t<volume-path>",
    "~\t~",
    "<controller-host>\t<worker-host>",
    "regex:192\\.168\\.\\d{1,3}\\.\\d{1,3}\t<worker-ip>",
    "regex:\\b10\\.0\\.\\d{1,3}\\.\\d{1,3}\\b\t<cluster-ip>",
    "regex:Copyright \\d{4} The Zerg Swarm Authors\tCopyright 2026 The Zerg Swarm Authors",
    "示例虫茧\t示例虫茧",
    "example-8b-quant\texample-8b-quant",
    "Mr2109\tMr2109\t裸用户名",
    # ── 2026-09-26 补格同步：形态表新增 4 格 ⇒ 夹具须带这 4 格的**出处逐字句**（E1 判的就是「夹具里命中恰好 1 次」），
    #    否则 ① 正控会红在「出处 0 命中」而不是被测的那条判据（夹具与真源同形是**本件自检的前提**）。
    "example-moe-36b\texample-moe-36b",
    "example-35b-v2\texample-35b-v2",
    "example-35b\texample-35b",
    "ExampleAdapter\tExampleAdapter",
    "EXAMPLE_TEMPLATE\tEXAMPLE_TEMPLATE",
    "example-35b-v2-\texample-35b-v2-",
    "example-35b-v2\texample-35b-v2",
    "L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=\tL1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
    "Zerg-内部文档\tZerg-内部文档",
    "!夹具\tcore/internal/obs/redact/*_test.go",
]

REDACT_BASE = [
    "package redact",
    "func hasHomePath(s string) bool {",
    '\treturn strings.Contains(s, "/Users/") || strings.Contains(s, "/home/") || strings.Contains(s, "~/")',
    "}",
    "// 分流，是为了**不留第二份 reHomeDir 的副本**（两份规则 = 下一次漂移的种子）；它不做 `$` 展开，",
    "var (\treHomeDir = regexp.MustCompile(`~/|/(?:Users|home)/[A-Za-z0-9._-]+`)\n)",
]


def _forms_copy():
    import copy
    return copy.deepcopy(FORMS)


def _run_with(td, forms, rules=None, non_form_rules=None, non_form_fragments=None):
    """在夹具上跑 run()，但把形态表 / 非形态登记表换成传入的那一份（**不改盘上真件**）。

    ★ `rules` 给了就**真的落到夹具盘上**（2026-09-26 修：此前形参收下却**不写**⇒ ④/④b 其实跑在
    旧夹具上 = **空转的负控**，格名说「同夹具」而夹具没变 ⇒ 判据没被测到）。
    ★ `non_form_rules` / `non_form_fragments` 给了就换上（`GAP-20260926-152` 的登记腿自检用）。
    """
    global FORMS, NON_FORM_RULES, NON_FORM_FRAGMENTS
    saved = FORMS
    saved_nf, saved_nff = NON_FORM_RULES, NON_FORM_FRAGMENTS
    try:
        if rules is not None:
            with open(os.path.join(td, REPLACE_RULES), "w", encoding="utf-8") as fh:
                fh.write("\n".join(rules) + "\n")
        FORMS = forms
        if non_form_rules is not None:
            NON_FORM_RULES = non_form_rules
        if non_form_fragments is not None:
            NON_FORM_FRAGMENTS = non_form_fragments
        return run(td)
    finally:
        FORMS = saved
        NON_FORM_RULES, NON_FORM_FRAGMENTS = saved_nf, saved_nff


def self_test(root):
    import copy
    ok = 0
    bad = 0
    lines = []

    def check(name, got, want):
        nonlocal ok, bad
        good = got == want
        ok += 1 if good else 0
        bad += 0 if good else 1
        lines.append(f"  {'✓' if good else '✗'} {name}（期望 rc={want} · 现跑 rc={got}）")

    with tempfile.TemporaryDirectory(prefix="w15-forms-") as td:
        _mk(td, RULES_BASE, REDACT_BASE)

        # ① 正控：形态齐 ⇒ rc=0
        check("① 正控：规则面 + 脱敏器面全被形态格认领 ⇒ rc=0", _run_with(td, _forms_copy())["rc"], 0)

        # ② must-fail：加一条 `~/…` 规则、并**删掉波浪号那一格** ⇒ rc=1（**这正是原病**）
        f2 = [f for f in _forms_copy() if f["id"] != "F-02"]
        r2 = _run_with(td, f2)
        check("② must-fail：`~/…` 形态无格（删 F-02）⇒ rc=1", r2["rc"], 1)
        named = any("E2b" == d["rule"] or "E2a" == d["rule"] for d in r2["red"])
        check("②b 同上 ⇒ **点名**到「形态无格」（不是笼统红）", 1 if named else 0, 1)

        # ③ 成对回绿：把 F-02 补回 ⇒ rc=0
        check("③ 成对：把 F-02 补回 ⇒ 回绿 rc=0", _run_with(td, _forms_copy())["rc"], 0)

        # ④ must-fail：`$HOME` 形态被认领的格**删掉** ⇒ rc=1（脱敏器逐字声明不做 `$` 展开，
        #    但**形态表必须有格**，否则「显式不覆盖」会被读成「漏项」）
        rules4 = RULES_BASE + ["$HOME/key\t<home>"]
        r4 = _run_with(td, _forms_copy(), rules=rules4)
        check("④ must-fail：`$HOME` 规则 + 有格认领 ⇒ 仍 rc=0（有格即算枚举到）", r4["rc"], 0)
        f4 = [f for f in _forms_copy() if f["id"] != "F-03"]
        _mk(td, rules4, REDACT_BASE)
        r4b = _run_with(td, f4)
        check("④b must-fail：同夹具把 F-03 也删掉 ⇒ rc=1", r4b["rc"], 1)
        _mk(td, RULES_BASE, REDACT_BASE)

        # ⑤ 成对回绿 + 不静默：F-03 = 「不覆盖 + 理由」⇒ rc=0 且**逐条打印**该格
        r5 = _run_with(td, _forms_copy())
        check("⑤ 成对：`$HOME` 格走「不覆盖 + 理由」⇒ rc=0", r5["rc"], 0)
        check("⑤b 同上 ⇒ 该格进 `debt[]`（**逐跑打印**，不静默）",
              len([d for d in r5["debt"] if d["form"] == "F-03"]), 1)

        # ⑥ must-fail：门禁侧空 + 无理由 ⇒ rc=1
        f6 = _forms_copy()
        for f in f6:
            if f["id"] == "F-04":
                f["gate_file"] = None
                f["debt"] = None
        check("⑥ must-fail：门禁侧空且**无理由** ⇒ rc=1", _run_with(td, f6)["rc"], 1)

        # ⑦ 真话负控：门禁侧点一件**不在盘**的判据件 ⇒ rc=1（空指）
        f7 = _forms_copy()
        for f in f7:
            if f["id"] == "F-06":
                f["gate_file"] = "scripts/gates/nonexistent-gate.py"
        r7 = _run_with(td, f7)
        check("⑦ 真话负控：门禁侧点名**不在盘**的件 ⇒ rc=1（空指）", r7["rc"], 1)
        check("⑦b 同上 ⇒ 理由逐字含「空指」",
              1 if any("空指" in d["why"] for d in r7["red"]) else 0, 1)

        # ⑧ 真话负控：出处句改成**真源里命不中**的字面 ⇒ rc=1（出处不许手写记忆）
        f8 = _forms_copy()
        for f in f8:
            if f["id"] == "F-08":
                f["src_probe"] = "这一句在真源件里不存在"
        r8 = _run_with(td, f8)
        check("⑧ 真话负控：出处句 `grep -F` 0 命中 ⇒ rc=1", r8["rc"], 1)

        # ⑨ E4 实例口径：探针 `~` ⇒ 进 instances[]（只打不红）
        r9 = _run_with(td, _forms_copy())
        check("⑨ E4 只打不红：实例口径探针进 `instances[]` 且 rc 不变",
              1 if (r9["meta"]["instance_level"] >= 1 and r9["rc"] == 0) else 0, 1)

        # ⑩ 空转 / 缺件 ⇒ rc=2（不给结论 ⇒ 不许当绿）
        empty = os.path.join(td, "empty")
        os.makedirs(empty, exist_ok=True)
        check("⑩ 缺真源件（空转）⇒ rc=2（不给结论 · 不许当绿）", _run_with(empty, _forms_copy())["rc"], 2)

        # ⑪ 真话负控（**成对**）：注入一条**真形态**、表里任何格都不认领 ⇒ rc=1 且**点名**该行原文
        #    （夹具 = 真源同形；A72109 是 `check-public-marks.py` `PRIVATE_PATTERNS` 里的真私有账号形态）
        _mk(td, RULES_BASE, REDACT_BASE)
        rules11 = RULES_BASE + ["A72109\tMr2109\tGitee 镜像账号（真形态 · 表里无格）"]
        r11 = _run_with(td, _forms_copy(), rules=rules11)
        check("⑪ 真话负控：注入**真形态**未被任何格认领 ⇒ rc=1", r11["rc"], 1)
        check("⑪b 同上 ⇒ E2a **点名**该行原文（不是笼统红）",
              1 if any(d["rule"] == "E2a" and "A72109" in d["raw"] for d in r11["red"]) else 0, 1)

        # ⑫ 真话负控：把某格**认领正则弄哑** ⇒ 它本该认领的那处必报（认领是不是**真在生效**）
        _mk(td, RULES_BASE, REDACT_BASE)
        f12 = _forms_copy()
        for f in f12:
            if f["id"] == "F-13":
                f["claim"] = r"ZZZ-弄哑的认领正则"
        r12 = _run_with(td, f12)
        check("⑫ 真话负控：F-13 认领正则弄哑 ⇒ rc=1", r12["rc"], 1)
        check("⑫b 同上 ⇒ 报出 `Zerg-内部文档` 那行（E2a 点名）",
              1 if any(d["rule"] == "E2a" and "Zerg-内部文档" in d["raw"] for d in r12["red"]) else 0, 1)

        # ⑬ 真话负控：把某**登记债务**改回**未登记**（门禁侧空 + 无理由 + 无日期）⇒ 必报
        _mk(td, RULES_BASE, REDACT_BASE)
        f13 = _forms_copy()
        for f in f13:
            if f["id"] == "F-16":
                f["debt"] = None
        r13 = _run_with(td, f13)
        check("⑬ 真话负控：债务格改回**未登记**（gate 空 + 无理由）⇒ rc=1", r13["rc"], 1)
        check("⑬b 同上 ⇒ E3 **点名**到 F-16",
              1 if any(d["rule"] == "E3" and d["form"] == "F-16" for d in r13["red"]) else 0, 1)

        # ⑭ 非形态登记（E2a · `GAP-20260926-152`）：注入一条**公开示例**规则 + **逐字登记**它
        #    ⇒ 不红、进 `nonform[]`（逐跑打印 · 不静默）= 「门分得清真形态 vs 非形态」。
        _mk(td, RULES_BASE, REDACT_BASE)
        rules14 = RULES_BASE + ["example-35b-v9\texample-35b-v9\t公开示例（非泄漏形态）"]
        r14 = _run_with(td, _forms_copy(), rules=rules14,
                        non_form_rules=[("example-35b-v9", "公开示例值（非私有泄漏形态）", "2026-10-04")])
        check("⑭ 非形态登记：公开示例规则 + 逐字登记 ⇒ rc=0", r14["rc"], 0)
        check("⑭b 同上 ⇒ 该规则进 `nonform[]`（逐跑打印 · 不静默）",
              1 if any(x["raw"] == "example-35b-v9" for x in r14["nonform"]) else 0, 1)

        # ⑮ must-fail：**同一条规则不登记** ⇒ rc=1（须登记才放行 —— 登记腿不是把红都洗掉）
        r15 = _run_with(td, _forms_copy(), rules=rules14, non_form_rules=[])
        check("⑮ must-fail：同规则不登记 ⇒ rc=1（真形态负控照旧有牙）", r15["rc"], 1)
        check("⑮b 同上 ⇒ E2a **点名**该行原文",
              1 if any(d["rule"] == "E2a" and "example-35b-v9" in d["raw"] for d in r15["red"]) else 0, 1)

    print("\n".join(lines))
    print(f"自检结论: {'全过' if bad == 0 else '有失败'}（断言 {ok + bad} 条 · 失败 {bad} 条）")
    return 0 if bad == 0 else 2


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("root", nargs="?", default=None)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--list-forms", dest="list_forms", action="store_true")
    ap.add_argument("--baseline", dest="baseline", action="store_true",
                    help="只读基线档：落当前命中快照供对拍，**不判** ⇒ rc 恒 0（GAP-20260926-153）")
    ap.add_argument("--list-unclaimed-only", dest="list_unclaimed_only", action="store_true",
                    help="只列**未被任何形态格认领**的规则/脱敏形态（只列不判 · rc=0）")
    a = ap.parse_args()
    root = a.root or os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

    if a.list_forms:
        for f in FORMS:
            print(f"  {f['id']}  {f['name']}")
            print(f"        认领：{f['claim']}")
            print(f"        出处：{f['src_file']} ⇒ 「{f['src_probe']}」")
            print(f"        门禁侧：{f['gate_file'] or '（不覆盖）→ ' + f['debt']}")
        return 0

    if a.self_test:
        return self_test(root)

    res = run(root)
    if res["rc"] == 2:
        print(f"⇒ {res['reason']}")
        return 2

    if a.list_unclaimed_only:
        # 缺口 `GAP-20260926-153`：只列**没被任何形态格认领**的那批（E2a 规则 / E2b 脱敏形态），
        # 不看 rc —— 「未认领」本身就是这一步要看的读数。
        unclaimed = [r for r in res["red"] if r.get("rule") in ("E2a", "E2b") and r.get("form") is None]
        for r in unclaimed:
            loc = f"{r['file']}:{r['line']}" if r.get("line") else (r.get("file") or "—")
            print(f"  ? [{r['rule']}] {loc}  {r['raw']}")
        print(f"⇒ 未被任何形态格认领：{len(unclaimed)} 条（只列不判 · rc=0）")
        return 0

    if a.baseline:
        # 缺口 `GAP-20260926-153`：**只读基线档** —— 落当前命中/债务/实例/非形态的**快照**供对拍，
        # 本档**不做判定** ⇒ rc 恒 0（与默认判档区分：判档才把 red 计成 rc=1）。
        snap = {"schema": SCHEMA, "kind": "PrivatePathFormsBaseline",
                "meta": dict(res["meta"], debt=res["debt"], instances=res["instances"],
                             nonform=res["nonform"]),
                "red": res["red"]}
        print(json.dumps(snap, ensure_ascii=False, indent=2))
        return 0

    if a.json:
        # 缺口 `GAP-20260926-173`：`--json` 补**逐格四件套机器面**（每个形态格的
        # id/name/claim/出处件+探针/门禁侧件+探针/debt）—— 先前只给人面三段。
        form_tuples = [{"id": f["id"], "name": f["name"], "claim": f["claim"],
                        "src_file": f["src_file"], "src_probe": f["src_probe"],
                        "gate_file": f.get("gate_file"), "gate_probe": f.get("gate_probe"),
                        "debt": f.get("debt")} for f in FORMS]
        print(json.dumps({"schema": SCHEMA, "kind": KIND, "items": res["red"],
                          "form_tuples": form_tuples,
                          "meta": dict(res["meta"], debt=res["debt"], instances=res["instances"],
                                       nonform=res["nonform"])},
                         ensure_ascii=False, indent=2))
        return res["rc"]

    print("── 过滤 / 脱敏面：**形态枚举** + 成对负控（私有路径 / 地址不许只认一两个字面量）──")
    print(f"  形态格 {res['meta']['forms']} 条 · 真源规则 {res['meta']['rules']} 条")
    for d in res["debt"]:
        print(f"  ⊘ [{d['form']}] {d['name']} —— **门禁侧不覆盖（存量债 · 逐条登记 · 照实打印）**")
        print(f"      {d['reason']}")
    for d in res["nonform"]:
        loc = f"{d['file']}:{d['line']}" if d.get("line") else (d["file"] or "—")
        print(f"  ⊘【非形态 · 已登记】[{d['rule']}] {loc}  {d['raw']}")
        print(f"      {d['why']}")
    for i in res["instances"]:
        print(f"  ⚠ E4 [{i['form']}] {i['file']} 的探针 `{i['probe']}` 是**实例口径** —— {i['why']}")
    for r in res["red"]:
        loc = f"{r['file']}:{r['line']}" if r.get("line") else (r["file"] or "—")
        print(f"  ✗ [{r['rule']}] {loc}  {r['raw']}")
        print(f"      {r['why']}")
    if res["rc"] == 0:
        print(f"⇒ 形态枚举全过（rc=0）：规则 {res['meta']['rules']} 条全被格认领 · "
              f"未覆盖形态 {res['meta']['debt_forms']} 条**已逐条登记并打印**（不静默）· "
              f"实例口径 {res['meta']['instance_level']} 条（只打不红）")
    else:
        print(f"⇒ 有红 {len(res['red'])} 处（rc=1）：逐条见上")
    return res["rc"]


if __name__ == "__main__":
    sys.exit(main())

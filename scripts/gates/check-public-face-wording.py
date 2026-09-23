#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-face-wording.py —— 公开面写作红线自检（`T-03` · 漏点 `M-3` · 判据 `V9`）

设计真源（逐字，不转述）
------------------------
`Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md` §一 第 3 行 `T-03`：
  「**补公开面写作红线自检**（甲档逐词须 0 + 乙档白名单式豁免**逐条打印理由**）」
  判据逐字：「甲档一行：`grep -rnoE \"AHZ|BXC|ZERG-ARCHIVE|Zerg-内部文档|Q-[0-9]{3}|父代理|本枚|开工单|认领单|2\\.5\\.\" publish/docs
  | wc -l` ⇒ **现读 0** ✓（基线现成 ⇒ 可当 `V9` 真值）；★ 乙档 **4 类假红源**须**逐条打印理由**放行
  （✗ 不许静默过滤）」
`Zerg-内部文档/项目文档/v2.5.12/设计-公开文档面随版收口-v1.1-20260923.md` §3.7（甲/乙两档表 · 红线）
与 §3.5 `V9`（= `M-3`）：
  「甲档真值：`AHZ`/`BXC`/`ZERG-ARCHIVE`/`Zerg-内部文档`/`Q-1xx`/私有称谓/`2.5.` ⇒ 现读**全 0** ✓」·
  「乙档 —— 允许，但要**逐条能解释**（本件现读的 4 类『假红源』· **不列出来就会自造假红** ✗）」·
  「★ 而且**照逐词写法写会当场自造假红 4 类**（`/Volumes/My Disk` 示例值 2 处 · `~/.zerg` **60 处** ·
   `Mr2109` 2 处 clone URL · `§` 2 处条目分隔符）—— **`~/.zerg` 与 `Mr2109` 这两条按字面 0 是不可能达成的**」。

为什么要有它（不是「在文档里写一句」）
--------------------------------------
红线**必须可现跑**（设计 §3.7 首句逐字：「红线**必须可现跑**（否则等于没有）；且**逐词写法要先跑一遍基线**，
否则第一条规矩就自造假红 ✗」）—— 公开面一旦漏了私有面是**不可撤回**的（推出去就收回不了），
所以它得挂在**必经之路**上（提交闸 `scripts/gates/precommit-gates.sh` 的 `gates` scope · 模式 `tri`）。
★ 本件是**写作面**的机检：它判**用词**（甲档逐词 + 白名单式豁免）。它**不判**命令名（那是 `T-14`/`V10` 的面）、
**不判**件新旧（那是 `T-15` 闸⑧ 第一条判据）、**不判**根面版本行（`T-02` 已落）。

判据（任一不满足 ⇒ rc=1；缺件/空转/自检不过 ⇒ rc=2）
---------------------------------------------------
  A **甲档逐词（12 条规则 · 每条给「为什么禁」）**：逐条扫**扫面**；命中**且未被白名单条目豁免** ⇒ 红
    （逐条点名 `件:行` + 命中片段 + 该规则的「为什么禁」）。
    A1 「私有卷路径」`/Volumes/` + 私有卷名（`AHZ`/`BXC`/`ZERG-ARCHIVE`）
       —— 为什么禁：私有挂载点会**带出仓外结构**（设计 §3.7 甲档第 1 行逐字）。
    A2 「任意 `/Volumes/` 卷路径（网兜 · 未登记即红）」`/Volumes/` + 任何非私有卷名的路径
       —— 为什么禁：同上；★ 判据逐字（设计 §3.7 乙档第 1 行）「`/Volumes/` 后跟的**不是**本机私有卷名 ⇒ 放行」
       ⇒ 但「放行」在本件里**必须走白名单逐条登记**（取**严格方向**：未登记的新卷名照样红 ——
       「配置/示例改了所以全绿」正是最难发现的那一类假绿）。
    A3 「私有卷/备份区代号」`AHZ` / `BXC` —— 为什么禁：卷代号本身就是**仓外结构**的名字（设计 §3.7 甲档第 2 行）。
    A4 「仓外私有文档仓名」`Zerg-内部文档` —— 为什么禁：它是**同级仓外的私有文档仓**，公开面点它的名 = 带出仓外结构
       （设计 §3.7 甲档第 3 行）。
    A5 「开发面缺口号」`Q-` + 数字 —— 为什么禁：**内部台账编号**；公开面对读者毫无意义，且暴露私有台账形状
       （设计 §3.7 甲档第 4 行逐字「缺口号 `Q-1xx` · 内部台账编号」）。★ 判据点名 `Q-1[0-9][0-9]`；
       本件取网兜 `\\bQ-\\d{1,4}\\b`（同族符号一次收净 · 现读 0 命中）。
    A6 「私有编写角色称谓」`父代理` / `本枚` / `开工单` / `认领单` / `助手`
       —— 为什么禁：**把内部写作角色带出去**（设计 §3.7 甲档第 5 行逐字）。
    A6b 「私有称谓（人物署名/私有项目名/机型）」`Mr2109` / `Mr2109` / `Mr2109` / 主机名 / `示例虫茧` / `<container-repo>` /
       `<project-a>` / `<project-b>` / Gitee 镜像账号 —— 为什么禁：**与发布链同一批脱敏对象**
       （真源 `publish/replace-rules.tsv` 的「人物/身份」「私有项目名」两段 + `check-public-marks.py`
       的 `PRIVATE_PATTERNS`）。现读公开面 **0** 命中。
    A7 「版本叙事/版本串」`2\\.5\\.` —— 为什么禁：铁律「公开面**只讲最新版**、不写版本叙事」
       （`publish/private-paths.txt` 的「公开面只允许正式面」那一段）。★ 判据是 **`2.5.` 前缀**，
       **不是**三段版本正则 —— 后者会撞 `192.0.2.10`（设计 §3.5 `V4` 逐字 · 本件现跑复现 3 处）。
    A8 「家目录状态目录前缀」`~/.zerg*` —— 为什么禁：`M-3` 把它写进「禁」的清单（同一族：带出**本机/用户私有**
       路径）；★ 而设计 §3.7 把它放**乙档**（公开面本来就有的用户配置路径）⇒ 本件取 §3.7，逐条豁免 + 打印理由。
    A9 「公开仓主名（裸账号名）」`Mr2109` —— 为什么禁：`M-3` 把它写进「禁」的清单；★ 设计 §3.7 逐字
       「**禁的是**『内部写作角色称谓』，不是作者名」⇒ 本件取 §3.7：**逐条**（`件 × 行`）豁免 + 打印理由。
    A10 「私有模型代号（`replace-rules.tsv` 全形）」`example-8b-quant` / `example-30b` /
       `example-moe-36b` / `example-35b` / `example-35b-v2` / `example-26b-review`
       —— 为什么禁：发布链的**私有模型代号**（真源 `publish/replace-rules.tsv` 的「私有模型代号（决策 4）」段）。
    A11 「私有模型代号（**现读到的前缀形态**）」`example-moe-36b` —— 为什么禁：同 A10；★ 2026-09-23 起替换规则
       也收**前缀形态**（`example-moe-36b → example-moe-36b` · 排在全长形态那条**之后**）⇒ 公开快照被盖住；
       本规则仍**保留**：本件判的是**私有树**（真源里的代号不遮）⇒ 命中逐条豁免（见丙档 ②）。
    A12 「文档面出处标记」`〔<字母><数字>〕`（形如 `〔S6〕`）—— 为什么禁：形态上是**开发面符号**
       （设计 §3.7 甲档第 4 行那一类）；★ 该族 4 处（`SECURITY.md`）**已抹于本笔**（现读 0 处）⇒
       丙档 ① 的登记同步**收窄移除**（判据点保留 · 不是放宽）。
  B **白名单式豁免（乙档 4 类 + 丙档 2 类 · 逐条打印理由）**：每处命中**逐条打印**
    `件:行 · 命中片段 · 该规则的「为什么禁」 · 该条目的「为什么豁免」`；
    **不许**静默过滤、**不许**「计数器加一不打印」（这是本件的立件理由之一：设计 §3.7 逐字
    「✗ 不许静默过滤」）。★ 白名单条目**每条都带理由**，且**命中 0 处的条目也照实打印**
    （悬空登记只打印、**不判红** —— 判红会让「件里把那个词删掉」变成假红；宽度本件照实写进条目理由）。
  C **守卫（空转/缺件一律 rc=2，不许当绿）**：`publish/docs/` 不存在 / 扫面件 **0 件** / 根面 8 件缺任一件 /
    任一件**读不了**（`OSError`）⇒ **rc=2 不给结论**。
  D **自豁免（脚本自身/词表行不许自红）**：本件内置词表**必然**含禁词**字面量**（`AHZ` · `Zerg-内部文档` · `Mr2109` ·
    `/Volumes/` …）⇒ 本件自己就是「照字面必红」的典型（仓内先例：`check-glossary.py:379/387/389` 等 6 处把
    自己的词表行判红）。两条一起：
      d1 **结构性**：本件**不在扫面里**（扫面 = `publish/docs/**` + 根面 8 件）；
      d2 **显式**：若本件被当扫面件喂进来（`SELF_REL`）⇒ 逐条打印理由放行（不是静默跳过）。
    ① ② 两格都由 `--self-test` 立格证明（格 14 证「词表真的含禁词」· 格 15/15b 用同一份夹具证
    「豁免开着 ⇒ 0、关掉 ⇒ 1」—— **非空转**）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿：甲档命中的**每一处**都有白名单条目逐条豁免且**逐条打印了理由**；扫面非空；根面 8 件齐且都读得动。
  1 = **有红**：甲档命中且**未**豁免 —— 逐条点名 `件:行` · 片段 · 为什么禁（处置只有两条：改件，或把
      该处**逐条**登进白名单**并写清理由** —— 不许放宽判据来凑绿 ✗）。
  2 = **不给结论**：`publish/docs/` 不存在 / 扫面 0 件 / 根面缺件 / 件读不了 / `--self-test` 不过。

★ 扫描面（**现读 = 24 件** · 件数复核**仍待拍** —— 本笔不动）：`publish/docs/**`（**16** 件 · 含 `design/` 2 件）+ 根面 **8** 件
  （`LICENSE` · `NOTICE` · `THIRD_PARTY_LICENSES.md` · `CONTRIBUTING.md` · `SECURITY.md` ·
   `config.example.yaml` · `gitleaks.toml` · `install.sh`）。
★ 白名单的**钉法**：`(规则名, 件 glob, 行正则, 理由)`；glob 走 `fnmatch`、`*` 跨 `/`
  （与 `publish/replace-rules.tsv` 的夹具隔离同一个约定）。
  为什么**不钉行号**：公开面正在随版编辑（v2.5.12 收口 20 条任务）⇒ 上游插一行就能把每一格豁免变成**假红**，
  而本仓铁律里「假红」与「假绿」同罪。**代价照实说**：同一个件里**再出现**同一个字面量仍会放行
  ⇒ 这是本机制的已知宽度，逐条写在条目理由里，处置留待拍板。
★ **不接管道取 rc**（本仓门禁硬规矩：管道后 `$?` 是末端 rc）—— 本件自己不做任何 shell 调用。

现读口径差（照实记 · 本件发现的与设计稿不一致处）
-------------------------------------------------
  ① `助手`：设计 §3.7 甲档**表**列了它，但 `T-03` 那**一行自检**（判据逐字）**没有**它 ⇒ 两者不一致。
     本件现跑复现：那一行 grep（逐字）⇒ **0** ✓（与设计稿一致）；**补上 `助手`** ⇒ **2**
     （`publish/docs/design/记忆体系.md:3` · `:72`）。
     ★ **2026-09-23 Mr2109 拍：取甲档表（含 `助手`）** —— 本条**已落定、不再是待定项**。
     落地（现读）：A6 规则词表逐字含 `助手`（见 `RULES` 的 A6 条）；上述 **2 处**逐条登进白名单 +
     **逐条打印**豁免理由（理由：**现读是产品词义（AI 助手）**，不是内部自称）。
  ② `~/.zerg` 与 `Mr2109`：`M-3` 把两者写进「禁」的清单，§3.7 把两者放**乙档**（允许）⇒ 本件取 §3.7。
     「按字面 0 不可能达成」这一句由本件的 A8/A9 两条规则 + 两类豁免条目**坐实**（现读 62 处 / 6 处）。
  ③ 「版本串」：`T-03` 判据用 `2\\.5\\.`（前缀）· 设计 §3.5 `V4` 逐字「**判据是 `2.5.` 前缀**，**不是**三段
     版本正则（后者会撞 `192.0.2.10`）」⇒ 本件用前缀。现读 `192.0.2.10` **3 处**
     （`CONFIGURATION.en.md:123` · `CONFIGURATION.zh-CN.md:123` · `config.example.yaml:50`）——
     按三段正则写就**当场 3 处假红**（本件现跑复现，不是推断）。
  ④ 本件**故意不设**的判据点：`P\\d{1,2}`（拍板号形态）—— 现读它会命中
     `publish/docs/design/格式反馈协议-FFP.md:48–52` 的**表格行标签** `P1`–`P5`（文档内的局部编号，
     不是拍板号）⇒ 设它就是**自造假红源**。故本件只收 `〔S…〕` 这一类（形态独有 · **该族已抹于本笔**
     —— 原 4 处全在 `SECURITY.md`，现读 0 处），**不设** `P\\d+` 网兜。

用法
----
    python3 scripts/gates/check-public-face-wording.py              # 判真目标（仓根）
    python3 scripts/gates/check-public-face-wording.py --root <树>   # 判别的树（自检夹具同入口）
    python3 scripts/gates/check-public-face-wording.py --json rules,hits,exempt,unexempt,face
    python3 scripts/gates/check-public-face-wording.py --self-test   # 成对负控（合成夹具 · 不碰真目标）
    python3 scripts/gates/check-public-face-wording.py --no-self-test

★ 本件是**私有面件**（内置词表逐条写着哪些词属私有面 —— 让它跟着公开面走等于把「哪些别写」的答案
  连同判据一起发出去）⇒ 登记在 `scripts/公开标记.tsv` 的 `public=no` 面。
"""
import argparse
import fnmatch
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-public-face-wording.py v1.0.0"

DOCS_REL = "publish/docs"
SELF_REL = "scripts/gates/check-public-face-wording.py"

# 根面 8 件（设计 §2.3「根面 8 件对外件」· 与 `T-03`/`T-11` 同一批读数）
ROOT_FACE = (
    "publish/LICENSE",
    "publish/NOTICE",
    "publish/THIRD_PARTY_LICENSES.md",
    "publish/CONTRIBUTING.md",
    "publish/SECURITY.md",
    "publish/config.example.yaml",
    "publish/gitleaks.toml",
    "publish/install.sh",
)

SKIP_SUFFIX = ("~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store")

# ── 甲档：12 条规则（id · 名称 · 正则 · 为什么禁）─────────────────────────────────────────
#   ★ 每条都带「为什么禁」（设计 §3.7 甲档表的同一口径：**逐词写清为什么**）；
#   ★ 正则一律用**能命中、不空转**的形态（每条在 `--self-test` 里都有一格负控证明「会红」）。
RULES = (
    ("A1", "私有卷路径（`/Volumes/` + 私有卷名）",
     r"/Volumes/(?:AHZ|BXC|ZERG-ARCHIVE)\b",
     "私有挂载点会带出**仓外结构**（设计 §3.7 甲档第 1 行）"),
    ("A2", "任意 `/Volumes/` 卷路径（网兜 · 未登记即红）",
     r"/Volumes/(?!(?:AHZ|BXC|ZERG-ARCHIVE)\b)[^\n\"'`)\]）]*",
     "同上；判据逐字（§3.7 乙档第 1 行）「`/Volumes/` 后跟的**不是**本机私有卷名 ⇒ 放行」"
     "⇒ 放行**必须走白名单逐条登记**（未登记的新卷名照样红）"),
    ("A3", "私有卷/备份区代号 `AHZ` / `BXC`",
     r"(?<![A-Za-z0-9])(?:AHZ|BXC)(?![A-Za-z0-9])",
     "卷代号本身就是**仓外结构**的名字（设计 §3.7 甲档第 2 行）"),
    ("A4", "仓外私有文档仓名 `Zerg-内部文档`",
     r"Zerg-内部文档",
     "它是**同级仓外的私有文档仓**；公开面点它的名 = 带出仓外结构（设计 §3.7 甲档第 3 行）"),
    ("A5", "开发面缺口号 `Q-` + 数字",
     r"\bQ-\d{1,4}\b",
     "**内部台账编号**：对读者无意义，且暴露私有台账形状（设计 §3.7 甲档第 4 行）"),
    ("A6", "私有编写角色称谓（`父代理`/`本枚`/`开工单`/`认领单`/`助手`）",
     r"父代理|本枚|开工单|认领单|助手",
     "**把内部写作角色带出去**（设计 §3.7 甲档第 5 行逐字）"),
    ("A6b", "私有称谓（人物署名/私有项目名/机型 · 与发布链同批脱敏对象）",
     r"Mr2109|Mr2109|脑\(Hermes\)|Mr2109deMac-Studio\.local|(?<![A-Za-z0-9_])Mr2109(?![A-Za-z0-9_])"
     r"|示例虫茧|<container-repo>|<project-a>|<project-b>|A72109",
     "真源 `publish/replace-rules.tsv` 的「人物/身份」+「私有项目名」两段，"
     "与 `check-public-marks.py` 的 `PRIVATE_PATTERNS` 同批口径"),
    ("A7", "版本叙事 / 版本串（`2.5.` 前缀）",
     r"2\.5\.",
     "铁律：公开面**只讲最新版**、不写版本叙事（`publish/private-paths.txt` 的「公开面只允许正式面」段）；"
     "★ 判据用**前缀**而非三段正则 —— 后者会撞 `192.0.2.10`（设计 §3.5 `V4` 逐字）"),
    ("A8", "家目录状态目录前缀 `~/.zerg*`",
     r"~/\.zerg[a-zA-Z0-9._-]*",
     "`M-3` 把它写进「禁」的清单（同一族：带出**本机/用户私有**路径）；"
     "★ 设计 §3.7 把它放**乙档**（公开面本来就有的用户配置路径）⇒ 本件逐条豁免 + 打印理由"),
    ("A9", "公开仓主名（裸账号名）`Mr2109`",
     r"\bMr2109\b",
     "`M-3` 把它写进「禁」的清单；★ 设计 §3.7 逐字「**禁的是**『内部写作角色称谓』，不是作者名」"
     "⇒ 本件逐条（件 × 行）豁免 + 打印理由"),
    ("A10", "私有模型代号（`replace-rules.tsv` 全形）",
     r"example-8b-quant\.IQ4_XS|example-30b|example-moe-36b|example-35b-v2-1\.[05]-35b|example-26b-review",
     "发布链的**私有模型代号**（真源 `publish/replace-rules.tsv` 的「私有模型代号（决策 4）」段）"),
    ("A11", "私有模型代号（现读到的**前缀形态**）`example-moe-36b`",
     r"example-moe-36b",
     "同 A10；★ 2026-09-23 起替换规则也收前缀形态（见丙档 ②）。本规则**保留**：本件扫的是**私有树**"
     "（真源里的代号不遮）⇒ 逐条豁免而不是装看不见"),
    ("A12", "文档面出处标记 `〔<字母><数字>〕`（形如 `〔S6〕`）",
     r"〔[A-Za-z]{1,3}\d{1,3}〕",
     "形态上是**开发面符号**（设计 §3.7 甲档第 4 行那一类）；该族 4 处残留（`SECURITY.md`）"
     "**已抹于本笔**（改中性说法 ⇒ 现读 0 处）⇒ 丙档 ① 的白名单登记同步**收窄移除**"),
)

# ── 白名单（乙档 4 类 + 丙档 2 类 · **每条都带理由** · 逐命中逐条打印）────────────────────────
#   条目 = (规则名, 件 glob（tuple · fnmatch · `*` 跨 `/`）, 行正则, 为什么豁免)
#   ★ 一条规则可以被多条条目覆盖（如 A9 的四种语境各一条）；同一条目允许跨多件（glob）。
WHITELIST = (
    # 乙档 ① —— `/Volumes/…` 示例值（设计 §3.7 乙档第 1 行）
    ("A2", ("publish/docs/*",),
     re.compile(r'ZERG_KB_PATH="/Volumes/My Disk/knowledge\.db"'),
     "乙档 ① `/Volumes/…` **示例值**（设计 §3.7 乙档第 1 行逐字）：该行讲的是"
     "「路径含空格/中文时 `.env` 里要加双引号」，`/Volumes/My Disk` 是**举例用的卷名**"
     "（不是本机私有卷名 `AHZ`/`BXC`/`ZERG-ARCHIVE`）⇒ 判「允许」✓（现读 2 处："
     "`QUICKSTART.en.md:28` · `QUICKSTART.zh-CN.md:28`）"),
    # 乙档 ② —— `~/.zerg/**`（设计 §3.7 乙档第 2 行）
    ("A8", ("publish/docs/*", "publish/SECURITY.md", "publish/config.example.yaml"),
     re.compile(r"~/\.zerg"),
     "乙档 ② `~/.zerg/**`（设计 §3.7 乙档第 2 行逐字）：这些是**公开面本来就有的用户配置路径**"
     "（文档要讲配置在哪儿：`~/.zerg/token` · `~/.zerg/state/` · `~/.zerg-chat/chat.db` …）；"
     "**禁的是**「私有卷/私有仓的**绝对路径**」✗，**不是**产品自己的家目录状态目录 ⇒ 判「允许」✓。"
     "现读 **62 处** = `publish/docs/**` **60**（与设计 §3.7 的 60 逐字相同）+ 根面 **2**"
     "（`SECURITY.md:13` 的 `~/.zerg/token` · `config.example.yaml:6` 的 `~/.zerg`）"),
    # 乙档 ③ —— `Mr2109`（设计 §3.7 乙档第 3 行）· 四种语境各一条（逐条给理由，不含混）
    ("A9", ("publish/docs/README.md", "publish/docs/README.zh-CN.md"),
     re.compile(r"git clone https://github\.com/Mr2109/zerg-swarm\.git"),
     "乙档 ③ `Mr2109`（设计 §3.7 乙档第 3 行逐字）：`git clone https://github.com/Mr2109/zerg-swarm.git`"
     " = **公开仓主名**（README 的克隆 URL · 必须出现）⇒ 判「允许」✓"
     "（**禁的是**「内部写作角色称谓」，不是作者名 · 现读 2 处：`README.md:75` · `README.zh-CN.md:73`）"),
    ("A9", ("publish/NOTICE",),
     re.compile(r"github\.com/Mr2109/zerg-swarm"),
     "丙档追补（**本件现读** · §3.7 只列了 README 两处）：同一账号出现在**版权/出处行**"
     "（`NOTICE:5` 的 `(https://github.com/Mr2109/zerg-swarm)`）—— 与上一条**同一个公开身份**"
     "（公开仓主名），不是私有面 ⇒ 允许 ✓"),
    # ★ 2026-09-23 收窄移除：原 A9 条目（`publish/SECURITY.md` 的 `Mr2109 拍板`）随该件**改中性说法**
    #   一并删掉 —— 它引的那一行（原 `:23` 的「**P10**（2026-09-21 Mr2109 拍板「按你意见」）」）已抹，
    #   现读命中 **0**；条目留着就是**悬空登记引着一行不存在的原文**（理由句句指向已删正文）。
    #   ★ 删条目 ≠ 放宽判据：A9 规则点**照旧在**，`Mr2109` 若在该件再现且不匹配其余条目 ⇒ 照红。
    ("A9", ("publish/install.sh",),
     re.compile(r"ZERG_UPGRADE_SOURCE=https://github\.com/Mr2109/zerg-swarm"),
     "丙档追补（**本件现读**）：`install.sh:17` 的拉取源**示例行**—— 公开仓主名（用户要照着填的那个名字）"
     "⇒ 允许 ✓"),
    ("A9", ("publish/install.sh",),
     re.compile(r'REPO_SLUG="Mr2109/zerg-swarm"'),
     "丙档追补（**本件现读**）：`install.sh:24` 的 `REPO_SLUG` = 公开仓主名（升级器的拉取源）⇒ 允许 ✓"),
    # 丙档 ① 追补 —— `助手`（设计 §3.7 甲档表有它 · 但它自己那「一行自检」没有它）
    #   ★ **2026-09-23 Mr2109 拍：取甲档表（含 `助手`）** ⇒ 本条即该拍板的落地：A6 词表含 `助手`，
    #     现读的 2 处（`记忆体系.md:3` · `:72`）逐条豁免 + **逐条打印**理由（禁静默过滤）。
    ("A6", ("publish/docs/design/记忆体系.md",),
     re.compile(r"助手把该记住的事|助手自己的笔记"),
     "丙档追补（**本件现读** · 设计 §3.7 **未列**）：`助手` 在 §3.7 甲档里是**私有编写角色称谓**"
     "（内部代理自称）；但现读这 2 处（`记忆体系.md:3` 的「让**对话助手**把该记住的事写进跨会话持久记忆」·"
     " `:72` 的 `memory` 作用域叫「**助手**自己的笔记」）都是**产品词义（AI 助手）**"
     "（被讲的产品对象 + 用户配置作用域名），不是内部自称 ⇒ 逐条豁免。"
     "★ **2026-09-23 Mr2109 拍：取甲档表（含 `助手`）**（口径差逐字见件头「现读口径差」①）——"
     "本条目是那条拍板的落地：2 处逐条豁免 · 逐条打印理由"),
    # 丙档 ① —— 出处标记 `〔S…〕`：**本条已抹于本笔**（2026-09-23）。
    #   原条目登记的是 `publish/SECURITY.md` 的 4 处 `〔S30〕`/`〔S2〕`/`〔S6〕`（`:22` 两处 · `:23` · `:31`）；
    #   该件已改中性说法（外部出处照留 · 内部符号去掉）⇒ 现读 **0 处** ⇒ 条目**移除**
    #   （不是转成悬空登记：留着一个引 4 处已删正文的理由 = 让后来人以为件里还带着这些符号）。
    #   ★ A12 判据点**保留** —— 命中变 0 是「件改了」，不是「判据放宽」；该族再现照样红。
    # 丙档 ② —— 私有模型代号前缀形态
    ("A11", ("publish/THIRD_PARTY_LICENSES.md",),
     re.compile(r"example-moe-36b"),
     "丙档 ②（**本件现读** · 设计 §3.7 未列）：`THIRD_PARTY_LICENSES.md:8` 的 fork 举例"
     "「（用户自装；或其 fork，如 **example-moe-36b** 变体）」—— `example-moe-36b` 是"
     "`publish/replace-rules.tsv` 里的**私有模型代号**；★ 2026-09-23：该表已补**前缀形态**一条"
     "（`example-moe-36b → example-moe-36b`，紧排在全长形态那条**之后**）⇒ **公开快照**里这一处已被规则盖住。"
     "本条目仍**必须留**：本门扫的是**私有树**（真源里的代号不遮）⇒ 逐条豁免 + 打印理由"),
)


def _rel(root, path):
    return os.path.relpath(path, root).replace(os.sep, "/")


def collect_face(root):
    """扫面 → (件列表, 缺件问题)。件列表 = 仓相对路径（字典序）。

    扫面 = `publish/docs/**`（递归 · 含 `design/`）+ 根面 8 件。
    `publish/docs/` 不存在 / 根面缺件 ⇒ 进 problems（调用方按 rc=2 处理）。
    """
    problems = []
    rels = []
    docs_dir = os.path.join(root, DOCS_REL)
    if not os.path.isdir(docs_dir):
        problems.append("扫面根不存在：%s/" % DOCS_REL)
    else:
        for dp, dns, fns in os.walk(docs_dir):
            dns[:] = sorted(d for d in dns if not d.startswith("."))
            for fn in sorted(fns):
                if fn.startswith(".") or fn.endswith(SKIP_SUFFIX):
                    continue
                rels.append(_rel(root, os.path.join(dp, fn)))
    for rel in ROOT_FACE:
        if os.path.isfile(os.path.join(root, rel)):
            rels.append(rel)
        else:
            problems.append("根面 8 件缺件：%s" % rel)
    return sorted(rels), problems


def scan_text(text):
    """对一段文本套**全部**甲档规则 ⇒ [(规则 id, 规则名, 为什么禁, 行号, 命中片段, 该行整行)]。"""
    hits = []
    lines = text.split("\n")
    for rid, title, pat, why in RULES:
        rx = re.compile(pat)
        for m in rx.finditer(text):
            ln = text.count("\n", 0, m.start()) + 1
            hits.append((rid, title, why, ln, m.group(0), lines[ln - 1]))
    return hits


def _exempt(hit, rel):
    """该命中是否被某条白名单条目豁免 ⇒ 命中的条目或 None。"""
    rid, _t, _w, _ln, _frag, line = hit
    for wrid, globs, rx, reason in WHITELIST:
        if wrid != rid:
            continue
        if not any(fnmatch.fnmatch(rel, g) for g in globs):
            continue
        if rx.search(line):
            return (wrid, globs, reason)
    return None


def judge(root, face_override=None, self_exempt=True):
    """判一棵树 ⇒ (rc, 输出行, data)。

    `face_override` 只在 `--self-test` 里用（把本件自己当扫面件喂进来，证自豁免两格）。
    """
    out = []
    out.append("── 公开面写作红线自检（`T-03` · 漏点 `M-3` · 判据 `V9`）：甲档逐词 + 白名单式豁免 ──")
    out.append("   树：%s" % root)
    if face_override is None:
        rels, problems = collect_face(root)
        if problems:
            for p in problems:
                out.append("   ⛔ %s" % p)
            out.append("   ⛔ 扫面缺件 %d 处 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）" % len(problems))
            return 2, out, {"face": 0, "rules": {}, "hits": 0, "exempt": 0, "unexempt": 0}
    else:
        rels, problems = list(face_override), []
    if not rels:
        out.append("   ⛔ 扫面 **0 件** ⇒ 空转，拒绝给结论（rc=2）"
                   "（「配置没了所以全绿」是最难发现的那一类假绿）")
        return 2, out, {"face": 0, "rules": {}, "hits": 0, "exempt": 0, "unexempt": 0}

    texts, unreadable, self_hits = {}, [], []
    for rel in rels:
        if self_exempt and rel == SELF_REL:
            self_hits.append("   ⚪ 自豁免 `%s`：本件内置词表**必然**含禁词字面量（`AHZ`/`Zerg-内部文档`/`Mr2109`/"
                             "`/Volumes/` …）⇒ **结构性不在扫面里**；被显式喂进来时按本行逐条打印理由放行"
                             "（不是静默跳过；牙齿见 `--self-test` 格 14/15/15b）" % rel)
            continue
        try:
            with open(os.path.join(root, rel), encoding="utf-8") as f:
                texts[rel] = f.read()
        except OSError as exc:
            unreadable.append("   ⛔ 件读不了：%s（%s）" % (rel, exc))
    if unreadable:
        out.extend(unreadable)
        out.append("   ⛔ 有件读不了 ⇒ **不给结论（rc=2）**（读不全就判不出真值）")
        return 2, out, {"face": len(rels), "rules": {}, "hits": 0, "exempt": 0, "unexempt": 0}

    out.extend(self_hits)
    out.append("   扫面：`%s/**` + 根面 8 件 = **%d 件**（逐件现读 · 件名逐条见 `--json face=1` 时的统计）"
               % (DOCS_REL, len(texts)))

    per_rule, exempted, unexempted = {}, [], []
    for rel in sorted(texts):
        for hit in scan_text(texts[rel]):
            rid = hit[0]
            per_rule[rid] = per_rule.get(rid, 0) + 1
            w = _exempt(hit, rel)
            if w is None:
                unexempted.append((rel,) + hit)
            else:
                exempted.append((rel,) + hit + (w[2],))

    out.append("   甲档逐词（12 条规则 · 命中即红 · 除白名单豁免）—— 一行一词：")
    for rid, title, _pat, why in RULES:
        n = per_rule.get(rid, 0)
        ex = sum(1 for e in exempted if e[1] == rid)
        mark = "✅" if n == 0 else ("⚪" if ex == n else "✗")
        tail = "" if n == 0 else ("（命中 %d → **全数豁免**）" % n if ex == n
                                  else "（命中 %d → 豁免 %d · **未豁免 %d**）" % (n, ex, n - ex))
        out.append("     %s %-4s %s 命中 %d %s" % (mark, rid, title, n, tail))
        out.append("          为什么禁：%s" % why)

    out.append("   乙档/丙档白名单命中（**逐条打印理由** · 不许静默过滤）：")
    if exempted:
        for rel, rid, title, why, ln, frag, line, reason in exempted:
            out.append("     ⚪ %s  %s:%d  「%s」" % (rid, rel, ln, frag))
            out.append("          为什么禁：%s" % why)
            out.append("          ↳ 本条豁免理由：%s" % reason)
    else:
        out.append("     （本跑 0 处命中 —— 白名单条目命中数逐条见下）")
    out.append("   白名单条目现读命中数（**每条都带理由 · 悬空登记也照实打印**；"
               "悬空**不判红** —— 「件里把那词删掉」不该变成假红；条目宽度逐条写在理由里）：")
    for i, (wrid, globs, rx, reason) in enumerate(WHITELIST, 1):
        n = sum(1 for e in exempted if e[1] == wrid and e[7] == reason)
        out.append("     #%-2d %s  件 glob=%s  行正则=%s  现读命中 %d 处%s"
                   % (i, wrid, "|".join(globs), rx.pattern, n, "" if n else "（悬空登记）"))
        out.append("          条目理由：%s" % reason)

    data = {"face": len(texts), "rules": per_rule, "hits": sum(per_rule.values()),
            "exempt": len(exempted), "unexempt": len(unexempted)}
    if unexempted:
        out.append("   ✗ 甲档命中**未豁免** %d 处（逐条点名 · 处置只有两条：改件，或**逐条**登进白名单并写清理由）："
                   % len(unexempted))
        for rel, rid, title, why, ln, frag, line in unexempted:
            out.append("       ── %s  %s:%d  「%s」  ← 为什么禁：%s" % (rid, rel, ln, frag, why))
        out.append("⇒ **有红（rc=1）**：公开面出现了私有面用词 —— 公开面不可撤回，先改件再提交")
        return 1, out, data

    out.append("   合计：甲档命中 **%d** 处 · 其中**豁免 %d** 处 · **未豁免 0** 处 · 扫面 %d 件"
               % (data["hits"], data["exempt"], data["face"]))
    out.append("⇒ 全绿（rc=0）：甲档逐词在**白名单之外**的命中为 0（每处豁免都已逐条打印理由）")
    return 0, out, data


# ── 自检（成对负控 · 合成夹具 · 先自证「会红」再扫真目标）──────────────────────────────────

def make_fixture(base, docs=None, root_files=None, drop_root=()):
    """最小扫面：`publish/docs/**`（合成件）+ 根面 8 件（合成件）。

    `docs` = {相对 docs 的路径: 内容}；`root_files` = {根面相对路径: 内容}
    （默认把 8 件都写成干净件）；`drop_root` = 要**故意不写**的根面件（缺件负控用）。
    """
    os.makedirs(os.path.join(base, DOCS_REL), exist_ok=True)
    for rel, txt in (docs or {"README.md": "# 干净件（合成夹具）\n"}).items():
        p = os.path.join(base, DOCS_REL, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(txt)
    for rel in ROOT_FACE:
        if rel in drop_root:
            continue
        p = os.path.join(base, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        body = (root_files or {}).get(rel, "合成根面件（干净）\n")
        with open(p, "w", encoding="utf-8") as f:
            f.write(body)
    return base


def self_test():
    checks = []

    def check(cond, desc):
        checks.append((bool(cond), desc))
        print("   %s %s" % ("✓" if cond else "✗", desc))

    t = tempfile.mkdtemp(prefix="pubface-wording-selftest-")
    try:
        # ① 正控 PC：合成扫面（干净）⇒ rc=0 且合计行出现
        b = make_fixture(os.path.join(t, "ok"), {"README.md": "# 公开面（合成）\n配置在 `~/.zerg/`。\n"})
        rc, o, d = judge(b)
        check(rc == 0 and d["face"] == 9, "① 正控 PC：干净扫面（docs 1 件 + 根面 8 件）⇒ rc=0（扫面 9 件）")

        # ② 负控 A1：私有卷名 ⇒ 必红
        b = make_fixture(os.path.join(t, "a1"), {"README.md": "挂载点 = /Volumes/ZERG-ARCHIVE\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["unexempt"] == 1 and d["rules"].get("A1") == 1,
              "② 负控 A1：`/Volumes/ZERG-ARCHIVE` ⇒ rc=1（私有卷路径有牙）")

        # ③ 负控 A2：未登记的卷名（网兜）⇒ 必红 —— 「不是私有卷名就静默放行」是不许的
        b = make_fixture(os.path.join(t, "a2"), {"README.md": "示例：/Volumes/Data/x.db\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A2") == 1,
              "③ 负控 A2：未登记卷名 `/Volumes/Data/x.db` ⇒ rc=1（放行必须逐条登记，不许静默放行）")

        # ④ 负控 A3：`AHZ` / `BXC` ⇒ 必红
        b = make_fixture(os.path.join(t, "a3"), {"README.md": "备份区 AHZ 与 BXC\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A3") == 2, "④ 负控 A3：`AHZ`/`BXC` ⇒ rc=1（卷代号有牙 · 命中 2）")

        # ⑤ 负控 A4：`Zerg-内部文档` ⇒ 必红
        b = make_fixture(os.path.join(t, "a4"), {"README.md": "见 Zerg-内部文档/项目文档/\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A4") == 1, "⑤ 负控 A4：`Zerg-内部文档` ⇒ rc=1（仓外私有文档仓名有牙）")

        # ⑥ 负控 A5：缺口号 `Q-999`（父任务点名的那个栗子）⇒ 必红
        b = make_fixture(os.path.join(t, "a5"), {"README.md": "（缺口 Q-999 · 仍在账）\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A5") == 1, "⑥ 负控 A5：缺口号 `Q-999` ⇒ rc=1（开发面符号有牙）")

        # ⑦ 负控 A6/A6b：私有称谓 ⇒ 必红
        b = make_fixture(os.path.join(t, "a6"), {"README.md": "由父代理分派；本枚认领单已交。\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A6", 0) >= 2, "⑦ 负控 A6：`父代理`/`本枚`/`认领单` ⇒ rc=1（写作角色称谓有牙）")
        b = make_fixture(os.path.join(t, "a6b"), {"README.md": "Mr2109拍板 · <project-a>项目\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A6b") == 2, "⑦b 负控 A6b：`Mr2109`/`<project-a>` ⇒ rc=1（发布链同批脱敏对象有牙）")
        # ⑦c/⑦d **`助手` 成对负控**（2026-09-23 Mr2109 拍「取甲档表（含 `助手`）」的落地格）：
        #   ⑦c = 同一字面量落在**白名单之外**的件 ⇒ rc=1（判据在词表里、不是只在白名单里）；
        #   ⑦d = 它落在**已登记**的件 × 行（`记忆体系.md` 的 `助手自己的笔记`）⇒ rc=0 且**逐条打印**豁免。
        #   ⇒ 两格成对才有区分度：只留 ⑦d 会变成「有豁免条目 = 判据没牙」，只留 ⑦c 会变成「件里一出现就红」。
        b = make_fixture(os.path.join(t, "a6c"), {"README.md": "由助手分派这件事（合成夹具 · 未登记件）\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A6") == 1 and d["unexempt"] == 1,
              "⑦c 负控 A6·`助手`：落在**白名单之外**的件 ⇒ rc=1（拍板「取甲档表（含 `助手`）」有牙）")
        b = make_fixture(os.path.join(t, "a6d"),
                         {"design/记忆体系.md": "| `memory` | 助手自己的笔记 | `MEMORY.md` |\n"})
        rc, o, d = judge(b)
        check(rc == 0 and d["rules"].get("A6") == 1 and d["exempt"] == 1,
              "⑦d 成对：同一处 `助手` 在**已登记的件 × 行** ⇒ rc=0（豁免 1 · 与 ⑦c 成对有区分度）")

        # ⑧ 负控 A7 + **假红对照**：`2.5.10` 必红；而 `192.0.2.10` **不许**命中（设计 §七 第 4 条）
        b = make_fixture(os.path.join(t, "a7"), {"README.md": "支持线 2.5.10\n"})
        rc, o, d = judge(b)
        check(rc == 1 and d["rules"].get("A7") == 1, "⑧ 负控 A7：版本串 `2.5.10` ⇒ rc=1（版本叙事有牙）")
        b = make_fixture(os.path.join(t, "a7b"), {"README.md": "主机 192.0.2.10（RFC 5737 示例段）\n"})
        rc, o, d = judge(b)
        check(rc == 0 and d["hits"] == 0,
              "⑧b 假红对照：`192.0.2.10` **不**命中（判据用 `2.5.` 前缀而非三段正则 · §3.5 `V4` 逐字）")

        # ⑨ 白名单命中 ⇒ **不算错**且**逐条打印理由**（禁静默过滤）
        b = make_fixture(os.path.join(t, "wl"), {
            "QUICKSTART.md": ' ZERG_KB_PATH="/Volumes/My Disk/knowledge.db"\n',
            "README.md": "配置见 `~/.zerg/token`。\n"})
        rc, o, d = judge(b)
        txt = "\n".join(o)
        check(rc == 0 and d["exempt"] == 2 and d["unexempt"] == 0,
              "⑨ 白名单命中 ⇒ 不算错（示例值 `/Volumes/My Disk` + `~/.zerg` 各 1 ⇒ 豁免 2 · rc=0）")
        check(txt.count("↳ 本条豁免理由：") == 2 and txt.count("为什么禁：") >= 2
              and txt.count("条目理由：") == len(WHITELIST),
              "⑨b **逐条打印**：2 处豁免各打出「为什么禁 + ↳ 本条豁免理由」两行，"
              "且 %d 条白名单条目逐条打出「条目理由」（✗ 静默过滤 / ✗ 只加计数不打印）" % len(WHITELIST))

        # ⑩ 白名单**宽度**：同一字面量落到**别的件** ⇒ 仍必红（豁免钉在「件 × 行」，不是全局放行）
        b = make_fixture(os.path.join(t, "wl-width"), {
            "QUICKSTART.md": ' ZERG_KB_PATH="/Volumes/My Disk/knowledge.db"\n',
            "OTHER.md": ' X="/Volumes/My Disk/other.db"\n'})
        rc, o, d = judge(b)
        check(rc == 1 and d["unexempt"] == 1,
              "⑩ 白名单宽度：同一字面量出现在**未登记的件**里 ⇒ rc=1（豁免不是全局放行）")

        # ⑪ 扫面不存在 ⇒ rc=2（不给结论）
        os.makedirs(os.path.join(t, "nodocs"), exist_ok=True)
        rc, o, d = judge(os.path.join(t, "nodocs"))
        check(rc == 2, "⑪ 扫面缺：`publish/docs/` 不在 ⇒ rc=2（缺件 ≠ 绿）")

        # ⑫ 根面 8 件缺一件 ⇒ rc=2
        b = make_fixture(os.path.join(t, "noroot"), drop_root=("publish/install.sh",))
        rc, o, d = judge(b)
        check(rc == 2, "⑫ 根面缺件：8 件缺 `install.sh` ⇒ rc=2（缺件 ≠ 绿）")

        # ⑬ 件读不了 ⇒ rc=2（把一个「件」造成目录 ⇒ open() 抛 OSError）
        b = make_fixture(os.path.join(t, "unreadable"))
        os.remove(os.path.join(b, "publish", "gitleaks.toml"))
        os.makedirs(os.path.join(b, "publish", "gitleaks.toml"), exist_ok=True)
        rc, o, d = judge(b)
        check(rc == 2, "⑬ 件读不了（`gitleaks.toml` 是目录）⇒ rc=2（读不全就判不出真值）")

        # ⑭ 自豁免 ①：本件自身**真的**含禁词字面量（证明豁免不是空转）
        with open(os.path.join(REPO_ROOT, SELF_REL), encoding="utf-8") as f:
            own = f.read()
        own_hits = scan_text(own)
        rid_set = {h[0] for h in own_hits}
        check(len(own_hits) > 0 and {"A1", "A2", "A4", "A5", "A6b", "A9"} <= rid_set,
              "⑭ 自豁免 ①：本件自身命中 %d 处（含 %s）⇒ 词表**真的**含禁词字面量（豁免非空转）"
              % (len(own_hits), "/".join(sorted(rid_set))))

        # ⑮ 自豁免 ②：把本件自己当扫面件喂进来 ⇒ 开着豁免 rc=0 且**打印**理由
        rc, o, d = judge(REPO_ROOT, face_override=[SELF_REL])
        check(rc == 0 and any("自豁免" in x for x in o),
              "⑮ 自豁免 ②：`face=[本件]` ⇒ rc=0 且**打印**「自豁免」行（不许静默跳过）")

        # ⑮b 成对对照：**关掉**豁免再扫同一件 ⇒ rc=1（这就是那一格的牙齿）
        rc, o, d = judge(REPO_ROOT, face_override=[SELF_REL], self_exempt=False)
        check(rc == 1 and d["unexempt"] > 0,
              "⑮b 自豁免的牙齿：同一份夹具**关掉**豁免 ⇒ rc=1（开了 0 / 关掉 1 ⇒ 这一格有区分度）")

        # ⑯ 假红对照：裸 `§`（条目分隔符形态）**不**命中任何规则（设计 §3.7 乙档第 4 行）
        b = make_fixture(os.path.join(t, "sec"), {"design/记忆体系.md": "条目之间以换行 + `§` + 换行连接。\n§\n"})
        rc, o, d = judge(b)
        check(rc == 0 and d["hits"] == 0, "⑯ 假红对照：裸 `§`（条目分隔符）⇒ **0 命中**（本门不把 `§` 当禁词）")

        # ⑰ 假红对照：`P1`（表格行标签）**不**命中（本门故意不设 `P\\d+` 网兜 · 件头口径差④）
        b = make_fixture(os.path.join(t, "prow"), {"design/FFP.md": "| P1 | 工具错误通道 | 旧行为 |\n"})
        rc, o, d = judge(b)
        check(rc == 0 and d["hits"] == 0, "⑰ 假红对照：表格行标签 `P1` ⇒ **0 命中**（不设 `P\\d+` 网兜 · 见口径差④）")

        # ⑱ 区分度：同一份夹具「命中 ⇒ 1」vs「删掉 ⇒ 0」（判据不是恒红/恒绿）
        b1 = make_fixture(os.path.join(t, "disc-a"), {"README.md": "见 Zerg-内部文档/项目文档/\n"})
        b2 = make_fixture(os.path.join(t, "disc-b"), {"README.md": "见同级文档仓\n"})
        r1, _, _ = judge(b1)
        r2, _, _ = judge(b2)
        check(r1 == 1 and r2 == 0, "⑱ 区分度：同夹具「命中 ⇒ 1」「删掉 ⇒ 0」（判据有牙 · 不恒绿/不恒红）")

        # ⑲ 三档齐：0 / 1 / 2 三条出口都在同一套判据上（本组前述各格已分别覆盖，这里点名汇总）
        check({judge(os.path.join(t, "ok"))[0], judge(os.path.join(t, "a1"))[0],
               judge(os.path.join(t, "nodocs"))[0]} == {0, 1, 2},
              "⑲ 三态齐：能（0）/ 错（1）/ 不给结论（2）三条出口都真实存在（同一套判据）")
    finally:
        shutil.rmtree(t, ignore_errors=True)

    bad = [d for ok, d in checks if not ok]
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "不过：" + " · ".join(bad), len(checks), len(bad)))
    return 0 if not bad else 2


def main():
    ap = argparse.ArgumentParser(description="公开面写作红线自检（publish/docs/** + 根面 8 件）")
    ap.add_argument("--root", default=REPO_ROOT, help="判哪棵树（默认仓根）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检直接判真目标（留给调用方显式选择）")
    ap.add_argument("--json", default="", help="按名输出 JSON 字段（逗号分隔）")
    args = ap.parse_args()

    print(VERSION)
    if args.self_test:
        return self_test()
    if not args.no_self_test:
        rc_st = self_test()
        if rc_st != 0:
            print("⛔ 自检不过 ⇒ 拒绝判真目标（门禁自己先能被证明「会红」）")
            return 2
    rc, lines, data = judge(os.path.abspath(args.root))
    for ln in lines:
        print(ln)
    if args.json:
        want = [w.strip() for w in args.json.split(",") if w.strip()]
        print(json.dumps({k: data.get(k) for k in want}, ensure_ascii=False, sort_keys=True))
    return rc


if __name__ == "__main__":
    sys.exit(main())

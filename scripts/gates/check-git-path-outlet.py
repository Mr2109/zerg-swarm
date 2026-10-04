#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-git-path-outlet.py —— 调用点判据门：件名统一出口（C-6）。

判据来源（逐条有据）
--------------------
  · 《设计-件名统一出口与调用点判据-v0.1.md》§5.1（门形态）+ §4.2 **C-6**（调用点判据）；
    §2.3/§2.5/§2.6 三条机制真值（`-c` 非全免疫 · 拼法两种 · 位置铁律）
  · 《任务清单-件名统一出口与调用点判据-v1.0.md》**B1**（门）+ **B2**（自带回归格 · 带牙齿）
  · 缺口账 `GAP-20260927-216` 的 **③（调用点判据门）**
  · 例外形状照 Clippy `disallowed_methods` 的 **`reason` + `replacement`** 两字段（调研结论 1.13）

要解决的问题（一句话）
----------------------
判定链的地基是「这个件是哪一件」。件名若以**转义形态**（首尾双引号 + 反斜杠八进制）进入判定，
四类判定会**集体静默反结论**（rc=0、报告照出）。本门在**调用点级**把「裸取件名」判红，
让这种写法**写不出来**，而不是靠人记得加开关。

判据（黑名单五类 · 逐条给模式与要抓的具体形态）
----------------------------------------------
  **GPO-F1 · 裸取件名（五类件名面）**
      模式：一次 git 调用，子命令 ∈ {ls-files · ls-tree -r|--name-only · diff --name-only ·
            status --porcelain|--short · grep}，而 argv 里 **`-z` 与 `-c` 都不带**。
      要抓：`git ls-files` ⇒ 非 ASCII 名被输出成 `"路径/\\346\\226\\207…"` ⇒ 下游 startswith /
            逐字相等 / HasSuffix(".md") 三种比对**全部失效**（首尾多一对引号）。

  **GPO-F2 · `-c` 后置**（位置铁律违背）
      模式：同一次 git 调用里 `-c` 出现在**子命令之后**。
      要抓：`… status --short -c core.quotepath=false`（rc=129）· `… diff --stat -c …`（rc=128）·
            `… ls-files … -c …`（`-c` 被当 `--cached` ⇒ **rc=0 但 0 行**，静默错，比报错更坏）。

  **丙类误报的处置（v1.2.0）**
      ① **行首注释整行不判**（井号 / 双斜杠 / 块注释星号 / HTML 注释 起头）—— 散文不是 argv；
      ② F3 空行**不再回退整窗口**（旧版把命中记到空行上 · 假红）；
      ③ F5 判面加「不带免疫开关」（合规调用进下游比对不算「未经出口」）；
      ④ 其余文字面（帮助串 / 消息串 / 文档字符串）走 `TEXT_ALLOW`：逐 (件, 规则, **锚**) 登记。

  **GPO-F3 · 大写拼法**
      模式：出现大写拼法那一形（本门记为 RULES 里的 `GPO-F3`）。
      要抓：同键两拼法共存 ⇒「按字符串扫面只认一种 ⇒ 漏一半」（当日血证：一枚卵只搜大写，
            误报三件没修）。★ **本单只认全小写为正确拼法**；本门**只判红、不改件**
            （全仓统一已由 D1 完成了）。

  **GPO-F4 · 其余会转义子命令无免疫开关**
      模式：子命令 ∈ {diff-tree -r|--name-only · show --name-only · log --name-only · diff --stat}
            且 argv 里 `-z` 与 `-c` 都不带。
      要抓：与 F1 同源、只是面不同（当日实测**会转义 11 种**，唯一确证不转义 = `rev-list --objects`）。

  **GPO-F5 · 取件名输出直入匹配 / JSON / 拼路径（未经出口）**
      模式：同一条逻辑行内，上面五类面的调用**之后紧接**消费动作（管道进 grep · `$(` 命令替换 ·
            `.match(`/`.startswith(`/`.endswith(` · `json.dump(s)` · `os.path.join` /
            `filepath.Join` / `path.Join` · `strings.HasPrefix|HasSuffix|Contains`）。
      要抓：设计稿 §一「受害四类」的前两类（按路径比对 · 拼路径取属性）—— 转义名直接进比对。

白名单例外（**仅出口实现件本身 · 每条例外必带 reason + replacement**）
----------------------------------------------------------------------
  唯一合法域 = `core/internal/gitpaths/`（C-1 的那个「一个」出口）。
  ★ 例外是**逐 (件, 规则)** 登记的，不是整件豁免；登记表校验三条：
     ① 件路径必须在 `core/internal/gitpaths/` 之下；② `reason` 与 `replacement` **都非空**；
     ③ 件在盘上真存在。任一不满足 ⇒ **rc=2 不给结论**（例外登记不合格 = 门自己不可信）。

档位（★ 本单只上线 `report` 档）
--------------------------------
  `--tier report`（**默认**）：只报告 —— 命中不阻断，rc=0。上线档位（P-8：新门先看误报率）。
  `--tier rc`（别名 `--strict`）：命中 ⇒ rc=1。升档属另批（E1）。

退码
----
  0 = 判到（report 档恒 0；rc 档全绿也是 0）
  1 = 只有 `rc` 档才可能：有命中（★ 白名单例外命中的**不算**）
  2 = **不给结论**：扫面 0 件（空转 = 假绿）/ 例外登记不合格 / `--self-test` 不过 / 用法错

扫面自身对 CJK 名不瞎（★ 不许踩本门自己抓的坑）
-----------------------------------------------
  列件走 `os.walk` —— Python 直接拿目录项（字节→`str`），**全程不经 git**，因此天生不受
  「非 ASCII 名被加引号 + 八进制」影响；自检里另有一格**真造中文名件、断言它出现在扫面清单里**
  （行为型，不靠「源码里有没有某串」）。

用法
----
    python3 scripts/gates/check-git-path-outlet.py                  # 扫本仓 · report 档
    python3 scripts/gates/check-git-path-outlet.py --tier rc        # 阻断档（命中 ⇒ rc=1）
    python3 scripts/gates/check-git-path-outlet.py --json full      # 机器读面
    python3 scripts/gates/check-git-path-outlet.py --repo <仓根>
    python3 scripts/gates/check-git-path-outlet.py --list-rules
    python3 scripts/gates/check-git-path-outlet.py --self-test      # 自带回归格（仓外夹具）
★ 本件**不挂门**（未登记进 precommit-gates.sh / 未动任何命令串）—— 挂门属另批 B3。
"""

from __future__ import annotations

import argparse
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

VERSION = "1.2.0"

SELF_REL = "scripts/gates/check-git-path-outlet.py"

# ── 件名子命令名 / 旗标名：**拼接构造** —— 本件自扫时不许自指命中 ─────────────────
N_LSFILES = "ls" + "-" + "files"
N_LSTREE = "ls" + "-" + "tree"
N_DIFF = "diff"
N_DIFFTREE = "diff" + "-" + "tree"
N_SHOW = "show"
N_LOG = "log"
N_STATUS = "status"
N_GREP = "grep"

F_NAME_ONLY = "--" + "name-only"
F_PORCELAIN = "--" + "porcelain"
F_SHORT = "--" + "short"
F_STAT = "--" + "stat"
F_Z = "-" + "z"
F_C = "-" + "c"
KEY_UPPER = "core." + "quote" + "Path"     # ★ 黑名单：大写拼法
KEY_LOWER = "core." + "quote" + "path"     # ★ 正解：全小写拼法

GIT_TOK = "git"

RULES = (
    ("GPO-F1", "裸取件名：五类件名面，argv 里 -z / -c 都不带"),
    ("GPO-F2", "-c 后置在子命令之后（位置铁律违背）"),
    ("GPO-F3", "大写拼法 " + KEY_UPPER + "（本单只认全小写 " + KEY_LOWER + "）"),
    ("GPO-F4", "会转义子命令（名单其余形态）argv 无免疫开关"),
    ("GPO-F5", "取件名输出直入匹配 / JSON / 拼路径（未经出口）"),
)
RULE_IDS = tuple(r[0] for r in RULES)

# 例外域：**仅出口实现件本身**（C-1）
OUTLET_PREFIX = "core/internal/gitpaths/"

# 白名单例外登记 —— 照 Clippy `disallowed_methods` 形状：逐条带 reason + replacement。
# ★ 现读 **零登记**（D1 完成后撤掉的两条）：出口域两件（`gitpaths.go` / `gitpaths_test.go`）
#   里的字面量已全小写 ⇒ GPO-F3 在该域不再命中 ⇒ 原先那两条**按 (件, 规则) 的整件豁免**闲置。
#   整件豁免（按路径豁免整件 · 不分锚）闲置 = 纯**假绿**风险：将来出口件里再冒出大写字面，
#   会被静默吞掉（真回归判成绿）。本门口径「失效方向只许是假红」⇒ 撤登记、恢复牙齿：
#   现读下出口件再出现大写字面 ⇒ **门直接报红**（见 `--self-test` 的仓外夹具格 / 当批负控）。
#   登记**能力**保留：真需要豁免时照下面的形状逐 (件, 规则) 加回，reason / replacement 写当读真相。
OUTLET_ALLOW = ()

# ── 文字面（**非 argv**）例外登记 —— 逐 **(件, 规则, 锚)** + reason + replacement ──────
# 丙类误报的正解：命中的那行**根本不是真 argv**（注释 / 帮助串 / 消息串 / 文档字符串散文）。
# 形态与 `OUTLET_ALLOW` 同形（reason + replacement 两者必填），但多一条 **锚**：
#   锚 = 命中那行**逐字**必须有的一段原文 ⇒ 排除**只对那一处文字**生效，**不是整件豁免**
#   （登记锚失效 ⇒ 那处又报红 —— 失效方向是「假红」，不会变成假绿）。
# ★ 本表只收**文字面**（非 argv）；真 argv 的裸调**一律不许进这里**（那是 C 组 / D1 的活）。
# ★ 锚里的拼法键**照本件的拼接构造**（`KEY_UPPER` / `GIT_TOK`）—— 写死字面量本件会自指命中。
TEXT_ALLOW = (
    {
        "path": "core/cmd/zerg/contract.go", "rule": "GPO-F1",
        "anchor": 'b.WriteString("  ' + GIT_TOK + " " + N_GREP + " -l",
        "reason": ("帮助串：这一处是把「复跑判据」印给用户看的**文字**（`b.WriteString` 的常量），"
                   "不是 argv —— 没有哪台机器会去跑它，故不产生转义名。"),
        "replacement": "若要真给可复跑命令，交由「命令串产地」统一（另批）；本门不判文字面。",
    },
    {
        "path": "scripts/gates/check-dangling-refs.py", "rule": "GPO-F1",
        "anchor": 'print("✗ ' + GIT_TOK + " " + N_LSFILES + " rc=%d",
        "reason": ("消息串：出错时印给人看的文本；同函数真调用在**另一行**且已带 `-z`"
                   "（同函数把 stdout 按 NUL 逐字切分 = 证据）。"),
        "replacement": "无需改：真调用已合规；本行只是报错文本里的字面。",
    },
    {
        "path": "scripts/gates/check-publish-face-sync.py", "rule": "GPO-F1",
        "anchor": 'print("✗ ' + GIT_TOK + " " + N_LSFILES + " 失败：%s（rc=%d）",
        "reason": ("消息串：`r.returncode != 0` 分支里的报错文本；真调用在**上一行**且已带"
                   " `-c core.quotepath=false`（同件 L102）。"),
        "replacement": "无需改：真调用已合规。",
    },
    {
        "path": "scripts/gates/who-references.py", "rule": "GPO-F1",
        "anchor": 'print("✗ ' + GIT_TOK + " " + N_LSFILES + " rc=%d",
        "reason": ("消息串：同 `check-dangling-refs.py` 的报错文本；真调用已带 `-z`"
                   "（同函数按 NUL 切分 stdout）。"),
        "replacement": "无需改：真调用已合规。",
    },
    {
        "path": "scripts/gates/check-links-lychee.py", "rule": "GPO-F3",
        "anchor": "① `-c " + KEY_LOWER + "=false`：**必须**",
        "reason": ("文档字符串散文：这行讲清「为什么必须带 `-c " + KEY_LOWER + "=false`」"
                   "（`D1` 已把本行统一成全小写，锚照现读正文对齐）—— 拼法键在此是"
                   "**被引用的宾语**，不是被人使用的拼法。"),
        "replacement": "无需改：讲坑的文字面登记不算「误用拼法」。",
    },
    {
        "path": "scripts/gates/check-links-lychee.py", "rule": "GPO-F3",
        "anchor": "同一个坑在账内 `W-31` 已记过一次（`" + KEY_UPPER + "=true`",
        "reason": "文档字符串散文：引用账内旧坑的原文（同上）。",
        "replacement": "无需改。",
    },
    {
        "path": "scripts/gates/check-public-face-anchor.py", "rule": "GPO-F3",
        "anchor": "**只在 tracked 的 diff 面里判**",
        "reason": ("文档字符串散文：本行把**正解写法**（`-c " + KEY_LOWER + "=false` · 全小写）"
                   "抄进说明里当范例 —— 拼法键出现在**被演示的命令**里，是范例不是调用。"),
        "replacement": "无需改：范例文字面。",
    },
    {
        "path": "scripts/gates/check-public-face-anchor.py", "rule": "GPO-F3",
        "anchor": 'lines.append("diff 面：`' + GIT_TOK + " -c " + KEY_LOWER + "=false",
        "reason": "报告文字串：往人读的报告里印「本门用的是哪条命令」（自证面），非 argv。",
        "replacement": "无需改：报告文字面。",
    },
    {
        "path": "scripts/gates/check-gap-axis-reconcile.py", "rule": "GPO-F1",
        "anchor": "现读 " + GIT_TOK + " " + N_LSFILES + " 失败",
        "reason": ("消息串：`ctx.get` 的第二实参 = **兜底文字**（戊 取数面拿不到时印给人看的报错文本），"
                   "不是 argv —— 没有哪台机器会去跑它。同件真调用在 L143 且已带 `-z`（三锚点：`-C` +"
                   " `core.quotepath=false` + `-z` · 同函数按 NUL 逐字切分 stdout = 证据）。"),
        "replacement": "无需改：真调用已合规；本行只是报错文本里的字面。",
    },
    # ── 2026-10-02 · M2 py→Go 端口补齐（同族丙类误报 · 逐 (件,规则,锚)）────────────
    #    下面五条与上面 py 侧三条**同族同因**：命中行是**消息串**（非 argv），
    #    同函数/同件的真调用已合规（`-z` 或 `-C`+`-c core.quotepath=false`+`-z`）。
    #    其中 ⑷⑸ 的文案对现实现已陈旧（实走文件系统现走 ⇒ 不跑 git），只登记、不改文案。
    {
        "path": "core/cmd/zerg/check_affected_map.go", "rule": "GPO-F1",
        "anchor": ("fmt.Fprintf(stdout, \"[缺读数] " + GIT_TOK + " " + N_LSFILES + " 取不到：%s"),
        "reason": ("消息串：`derr != nil` 分支里的报错文本；真取数走 `os.ReadDir` 文件系统现走"
                   "（同函数 L944 注释逐字「✗ 不经 git」）⇒ 没有哪台机器会去跑这条 git 命令。"
                   "该文案对现实现已陈旧，另批校正时本锚随之更新。"),
        "replacement": "无需改：本行只是报错文本里的字面。",
    },
    {
        "path": "core/cmd/zerg/check_dangling_refs.go", "rule": "GPO-F1",
        "anchor": ("fmt.Fprintf(stderr, \"✗ " + GIT_TOK + " " + N_LSFILES + " rc=%d"),
        "reason": ("消息串：Go 搬 py `check-dangling-refs.py` 的报错文本（py 侧同形已登记）；"
                   "同函数真调用在 L1184 且已带 `-z`（随后按 NUL 逐字切分 stdout = 证据）。"),
        "replacement": "无需改：真调用已合规；本行只是报错文本里的字面。",
    },
    {
        "path": "core/cmd/zerg/check_gap_axis_reconcile.go", "rule": "GPO-F1",
        "anchor": ("e = \"现读 " + GIT_TOK + " " + N_LSFILES + " 失败\""),
        "reason": ("消息串：Go 搬 py `check-gap-axis-reconcile.py` 的兜底文本（py 侧同形已登记）；"
                   "同件真调用在 L212 且三锚点齐（`-C` + `core.quotepath=false` + `-z`）。"),
        "replacement": "无需改：真调用已合规；本行只是报错文本里的字面。",
    },
    {
        "path": "core/cmd/zerg/who_references.go", "rule": "GPO-F1",
        "anchor": ("fmt.Fprintf(stderr, \"✗ " + GIT_TOK + " " + N_LSFILES + " rc=%d"),
        "reason": ("消息串：Go 搬 py `who-references.py` 的报错文本（py 侧同形已登记）；"
                   "同函数真调用在 L533 且已带 `-z`（随后按 NUL 切分 stdout = 证据）。"),
        "replacement": "无需改：真调用已合规；本行只是报错文本里的字面。",
    },
    {
        "path": "scripts/gates/check-affected-map.py", "rule": "GPO-F1",
        "anchor": ("print('[缺读数] " + GIT_TOK + " " + N_LSFILES + " 取不到：%s"),
        "reason": ("消息串：`except` 分支里的报错文本；真取数走 `disk_files` 的 `os.walk`"
                   "（同函数 L351-352 注释逐字「✗ 不经 git」）⇒ 本行非 argv。"
                   "该文案对现实现已陈旧，另批校正时本锚随之更新。"),
        "replacement": "无需改：本行只是报错文本里的字面。",
    },
    # ── 2026-10-04 · 门⑦/门68 收口：`precommit-gates.sh` 的**步骤名文字面**（非 argv）────────
    #    命中行是 `add_step … "<步骤名>" …` 里印给人看的**步骤名**；真取数走被点名的
    #    `check-public-marks-tracked-parity.py`（该件自带取件名现读对拍，见门94）。
    {
        "path": "scripts/gates/precommit-gates.sh", "rule": "GPO-F1",
        "anchor": ('门94 公开标记表时点对拍：理由列「未跟踪」↔ ' + GIT_TOK + " " + N_LSFILES + " 现读"),
        "reason": ("步骤名文字面：`add_step` 印给用户看的**步骤名**（引号内的常量），不是 argv —— "
                   "没有哪台机器会去跑它；真取数走被点名的 `check-public-marks-tracked-parity.py`"
                   "（该件自带 " + GIT_TOK + " " + N_LSFILES + " 现读对拍）。"),
        "replacement": "无需改：真调用在 check-public-marks-tracked-parity.py；本行只是步骤名里的字面。",
    },
)

# ── 文字面掩蔽：行首即注释的行，**整行**不判（宁可漏报，不可把散文判成 argv）───────
COMMENT_MARKERS = ("#", "//", "/*", "*", "<!--")


def _mask_line(line: str) -> str:
    """行首是注释标记 ⇒ 返回空串（该行无 argv 可判）。

    ★ 只认**行首**标记（含前置缩进）：行内的井号 / 双斜杠可能落在字符串或正则里，
      随手截断会把真调用切掉 ⇒ 那才是把判据改坏。
    """
    s = line.lstrip()
    for m in COMMENT_MARKERS:
        if s.startswith(m):
            if m == "*" and len(s) > 1 and not s[1].isspace():
                continue
            return ""
    return line


def _anchor_hit(entry, first_line):
    """锚是否落在命中那一行 —— 登记只对**这一处文字**生效（不是整件豁免）。"""
    a = str(entry.get("anchor", ""))
    return bool(a) and a in (first_line or "")


def validate_text_allow(root, check_exists=True):
    """文字面登记校验：锚非空 · reason/replacement 非空 · 规则号在表 · 件在盘上 · **锚真在该件**。"""
    bad = []
    for e in TEXT_ALLOW:
        p, r, a = e.get("path", ""), e.get("rule"), str(e.get("anchor", ""))
        if not a.strip():
            bad.append("文字面登记 %s · %s 缺 anchor（无锚 = 整件豁免 = 不许）" % (p, r))
        if not str(e.get("reason", "")).strip():
            bad.append("文字面登记 %s · %s 缺 reason" % (p, r))
        if not str(e.get("replacement", "")).strip():
            bad.append("文字面登记 %s · %s 缺 replacement" % (p, r))
        if r not in RULE_IDS:
            bad.append("文字面登记 %s 的规则号 %r 不在规则表里" % (p, r))
        if not check_exists:
            continue
        fp = os.path.join(root, p)
        if not p or not os.path.isfile(fp):
            bad.append("文字面登记的件 %s 在盘上不存在（悬空登记）" % p)
            continue
        try:
            with open(fp, "rb") as f:
                text = f.read().decode("utf-8")
        except (OSError, UnicodeDecodeError):
            bad.append("文字面登记的件 %s 读不动" % p)
            continue
        if a and a not in text:
            bad.append("文字面登记 %s · %s 的锚在件里找不到（锚失效 ⇒ 该处又报红）" % (p, r))
    return bad

# 扫面域（相对仓根）
SCAN_ROOTS = ("scripts", "core", "agent", "publish", "tools", "ui")
SCAN_EXTS = (".py", ".sh", ".go", ".rs", ".bash", ".zsh")
SKIP_DIRS = frozenset((".git", "vendor", "node_modules", "target", "dist", "bin",
                       "data", ".zerg", "__pycache__", "zerg-wt", ".venv", "venv"))
MAX_FILE_BYTES = 2 * 1024 * 1024
MAX_WINDOW_LINES = 4          # 逻辑窗口：本行 + 后 3 行（够 Go/Python 的跨行 argv）
MAX_WINDOW_CHARS = 4000

# F5 的消费动作（同一逻辑行内出现 ⇒ 取件名输出未经出口就进了下游）
CONSUMERS = (
    ".match(", ".search(", ".startswith(", ".endswith(", ".splitlines()",
    "json.dump", "os.path.join", "filepath.Join", "path.Join",
    "strings.HasPrefix", "strings.HasSuffix", "strings.Contains",
    ".MatchString(", "grep -", "for ", "$(",
)

# 未判件（**点名登记 · 不静默**）
NOT_JUDGED = (
    (SELF_REL,
     "自指：本件就是判据的产地（规则表 / 夹具串 / 消费动作表都在本件里）⇒ 扫自己必自指命中。"
     "**点名登记，不判**；本件自身的正确性靠 `--self-test` 的仓外夹具格保证。"),
)

TOKEN_RE = re.compile(r"[^\s]+")
STRIP_CHARS = "\"'`(),[]{}:="
STOP_TOKENS = frozenset(("", "|", "||", "&&", ";", ")", "]", "}", ">", ">>", "<", "then", "do", "done", "fi", "else"))


def _norm(tok: str) -> str:
    return tok.strip(STRIP_CHARS)


def _tokens(text: str):
    return TOKEN_RE.findall(text)


def _has_z(toks):
    for t in toks:
        if t == F_Z:
            return True
        if t.startswith("-") and not t.startswith("--") and len(t) > 1 and "z" in t[1:]:
            return True
    return False


def _quote_key_in(toks):
    """argv 里是否带 `-c <含拼法键的值>`（合写 `-ccore…` 也算）。"""
    for i, t in enumerate(toks):
        if t == F_C and i + 1 < len(toks) and "quote" in toks[i + 1]:
            return True
        if t.startswith(F_C) and len(t) > 2 and "quote" in t[2:]:
            return True
    return False


def _face(sub, args):
    """⇒ (面名, 是否五类面) / (None, False) 表示这次调用不取件名。"""
    if sub == N_LSFILES:
        return "ls-files", True
    if sub == N_LSTREE:
        if "-r" in args or F_NAME_ONLY in args:
            return "ls-tree", True
        return None, False
    if sub == N_DIFF:
        if F_NAME_ONLY in args:
            return "diff --name-only", True
        if F_STAT in args:
            return "diff --stat", False
        return None, False
    if sub == N_STATUS:
        if F_PORCELAIN in args or F_SHORT in args:
            return "status --porcelain/--short", True
        return None, False
    if sub == N_GREP:
        return "grep", True
    if sub == N_DIFFTREE:
        if "-r" in args or F_NAME_ONLY in args:
            return "diff-tree", False
        return None, False
    if sub == N_SHOW:
        if F_NAME_ONLY in args:
            return "show --name-only", False
        return None, False
    if sub == N_LOG:
        if F_NAME_ONLY in args:
            return "log --name-only", False
        return None, False
    return None, False


def parse_invocations(tokens):
    """从 token 序列里抽 git 调用 ⇒ [{sub, args, c_index, sub_index, region_end}]。

    ★ 只认「`git` 之后紧跟（跳过全局选项 `-C <p>` / `-c <kv>` / `--git-dir=…`）一个已知子命令」
    这一形态 —— 宁可漏报也不要把散文里的「git 的某子命令」当调用。
    """
    out = []
    known = {N_LSFILES, N_LSTREE, N_DIFF, N_DIFFTREE, N_SHOW, N_LOG, N_STATUS, N_GREP}
    i = 0
    while i < len(tokens):
        if tokens[i] != GIT_TOK:
            i += 1
            continue
        j = i + 1
        c_index = None
        while j < len(tokens):
            t = tokens[j]
            if t == F_C:
                c_index = j
                j += 2
                continue
            if t == "-C":
                j += 2
                continue
            if t.startswith("--git-dir") or t.startswith("--work-tree") or t == "--no-pager":
                j += 1
                continue
            if t.startswith("-"):
                j += 1
                continue
            break
        if j >= len(tokens):
            i += 1
            continue
        sub = tokens[j]
        if sub not in known:
            i += 1
            continue
        # 参数区：到分隔符为止
        args = []
        k = j + 1
        while k < len(tokens) and _norm(tokens[k]) not in STOP_TOKENS:
            args.append(_norm(tokens[k]))
            k += 1
        # `-c` 若落在**子命令之后**（参数区里）也要记下来 —— 这正是 GPO-F2 要抓的形态
        if c_index is None:
            for d, av in enumerate(args):
                if av == F_C:
                    c_index = j + 1 + d
                    break
        out.append({"sub": sub, "args": args, "c_index": c_index, "sub_index": j,
                    "region_end": k, "git_index": i})
        i = k
    return out


def judge_window(text, skip_rules=(), head_tokens=None, first_line=None):
    """判一条逻辑窗口 ⇒ [(规则, 说明, 片段)]。

    ★ 去重铁律：一次 git 调用**只按它自己所在那一行**报一次 ——
      · 调用只认「`git` token 落在窗口**第一行**」的那些（`head_tokens` 是第一行的 token 数）；
      · GPO-F3 只扫**第一行原文**。
      没有这两条，同一次调用会在随后的 3 行窗口里重复报 4 次（虚高计数 = 假红）。
    """
    hits = []
    toks = _tokens(text)
    head_n = len(toks) if head_tokens is None else head_tokens
    # ★ 空行**不再回退到整窗口**：first_line 为空串（空行）⇒ 这一行没有可判的文本。
    #   旧写法在空行上退化成「拿 4 行窗口来判」⇒ 命中记在**空行**上（假红）。
    line_text = text if first_line is None else first_line
    # ── GPO-F3：大写拼法（**逐行**文本面，含注释/常量/文档串）──────────────
    if "GPO-F3" not in skip_rules and KEY_UPPER in line_text:
        hits.append(("GPO-F3", "大写拼法 " + KEY_UPPER, _snippet(line_text, KEY_UPPER)))
    for inv in parse_invocations(toks):
        if inv["git_index"] >= head_n:
            continue
        face, canon = _face(inv["sub"], inv["args"])
        if face is None:
            continue
        # ★ 免疫开关的判面 = **整个调用**（从 `git` 到参数区末尾），不是只看子命令之后 ——
        #   否则全局区的 `-c core.quotepath=false` 会被漏掉 ⇒ 合规写法被误报成裸调（假红）。
        argtoks = toks[inv["git_index"]: inv["region_end"]]
        immune = _has_z(argtoks) or _quote_key_in(argtoks)
        # ── GPO-F2：`-c` 后置 ────────────────────────────────────────────
        if "GPO-F2" not in skip_rules and inv["c_index"] is not None and inv["c_index"] > inv["sub_index"]:
            hits.append(("GPO-F2", "`-c` 落在子命令 %s 之后" % inv["sub"], _snippet(text, F_C)))
        # ── GPO-F1 / GPO-F4：会转义子命令而 argv 无免疫开关 ────────────────
        if not immune:
            rid = "GPO-F1" if canon else "GPO-F4"
            if rid not in skip_rules:
                hits.append((rid, "子命令 %s 无 `-z` 也无 `-c`（会转义面）" % inv["sub"], _snippet(text, inv["sub"])))
        # ── GPO-F5：取件名输出直入消费动作（**未经出口**）──────────────────
        #   ★ 「未经出口」的判面 = 这次调用**不带免疫开关**：带了 `-z` / `-c` 的输出已是真名，
        #     进下游比对 / 拼路径是**合规**写法 ⇒ 不许报（旧版漏看这一条 ⇒ 合规调用被误报）。
        if "GPO-F5" not in skip_rules and not immune:
            tail = " ".join(toks[inv["region_end"]:]) if inv["region_end"] < len(toks) else ""
            for c in CONSUMERS:
                if c in tail or (c == "$(" and ("$(" + GIT_TOK) in text):
                    hits.append(("GPO-F5", "%s 面输出直入 %r（未经出口）" % (face, c), _snippet(text, c)))
                    break
    return hits


def _snippet(text, needle):
    i = text.find(needle)
    if i < 0:
        return text.strip()[:120]
    lo = max(0, i - 40)
    return text[lo:i + len(needle) + 40].strip()


def logical_windows(lines):
    """⇒ [(起始行号, 窗口文本, 第一行 token 数, 第一行原文)]。

    ★ 窗口 = 本行 + 后 3 行（够 Go/Python 的跨行 argv），**不跨语句结构合并**
    （按括号深度合并会把整个函数并成一行 ⇒ 一处 `-z` 免疫整函数 = 假绿）。
    """
    out = []
    n = len(lines)
    masked = [_mask_line(x) for x in lines]     # ★ 行首注释整行不判（文字面掩蔽）
    for i in range(n):
        buf = ""
        head = None
        for k in range(i, min(n, i + MAX_WINDOW_LINES)):
            buf = (buf + " " + masked[k]) if buf else masked[k]
            if head is None:
                head = len(_tokens(buf))
            if len(buf) > MAX_WINDOW_CHARS:
                break
        out.append((i + 1, buf, head or 0, masked[i]))
    return out


def iter_files(root, scan_roots=SCAN_ROOTS, exts=SCAN_EXTS, skip_dirs=SKIP_DIRS):
    """列件 —— ★ 走 `os.walk`（不经 git）⇒ 对 CJK 名**天生不瞎**。

    返回 (files, skipped)。skipped = [(相对路径, 原因)]，**不许静默丢**。
    """
    files, skipped, seen = [], [], set()
    for sub in scan_roots:
        base = os.path.join(root, sub)
        if os.path.isfile(base):
            cand = [(os.path.dirname(base), [], [os.path.basename(base)])]
            root_abs = os.path.dirname(base)
        elif os.path.isdir(base):
            cand = os.walk(base)
            root_abs = base
        else:
            continue
        for dirpath, dirnames, filenames in cand:
            dirnames[:] = [d for d in dirnames if d not in skip_dirs]
            for fn in sorted(filenames):
                if not fn.endswith(exts):
                    continue
                p = os.path.join(dirpath, fn)
                rel = os.path.relpath(p, root)
                if rel in seen:
                    continue
                seen.add(rel)
                try:
                    if os.path.getsize(p) > MAX_FILE_BYTES:
                        skipped.append((rel, "件大于 %d 字节" % MAX_FILE_BYTES))
                        continue
                    with open(p, "rb") as f:
                        raw = f.read()
                    raw.decode("utf-8")
                except (OSError, UnicodeDecodeError) as e:
                    skipped.append((rel, "读不动/非 UTF-8：%s" % type(e).__name__))
                    continue
                files.append(rel)
    return sorted(files), skipped


def scan(root, skip_rules=(), scan_roots=SCAN_ROOTS, exts=SCAN_EXTS, skip_dirs=SKIP_DIRS):
    """扫面 ⇒ dict（hits / exceptions / skipped / scanned）。只读。"""
    files, skipped = iter_files(root, scan_roots, exts, skip_dirs)
    allow = {}
    for e in OUTLET_ALLOW:
        allow.setdefault((e["path"], e["rule"]), e)
    text_allow = {}
    for e in TEXT_ALLOW:
        text_allow.setdefault((e["path"], e["rule"]), []).append(e)
    hits, exceptions = [], []
    for rel in files:
        try:
            with open(os.path.join(root, rel), "rb") as f:
                text = f.read().decode("utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        lines = text.splitlines()
        for ln, window, head_n, first_line in logical_windows(lines):
            for rid, why, snip in judge_window(window, skip_rules, head_n, first_line):
                rec = {"file": rel, "line": ln, "rule": rid, "why": why, "snippet": snip}
                if (rel, rid) in allow:
                    rec["exception"] = allow[(rel, rid)]
                    exceptions.append(rec)
                elif any(_anchor_hit(e, first_line) for e in text_allow.get((rel, rid), ())):
                    e = [x for x in text_allow[(rel, rid)] if _anchor_hit(x, first_line)][0]
                    rec["exception"] = e
                    rec["kind"] = "text"
                    exceptions.append(rec)
                else:
                    hits.append(rec)
    return {"scanned": files, "skipped": skipped, "hits": hits, "exceptions": exceptions}


def validate_allowlist(root, check_exists=True):
    """例外登记三条校验 ⇒ 不合格原因列表（非空 ⇒ rc=2）。"""
    bad = []
    for e in OUTLET_ALLOW:
        p = e.get("path", "")
        if not p.startswith(OUTLET_PREFIX):
            bad.append("例外件 %r 不在出口域 %s 之下（white-list 只许出口实现件本身）" % (p, OUTLET_PREFIX))
        if not str(e.get("reason", "")).strip():
            bad.append("例外 %s · %s 缺 reason" % (p, e.get("rule")))
        if not str(e.get("replacement", "")).strip():
            bad.append("例外 %s · %s 缺 replacement" % (p, e.get("rule")))
        if check_exists and p and not os.path.isfile(os.path.join(root, p)):
            bad.append("例外件 %s 在盘上不存在（悬空例外）" % p)
        if e.get("rule") not in RULE_IDS:
            bad.append("例外 %s 的规则号 %r 不在规则表里" % (p, e.get("rule")))
    return bad


# ─────────────────────────── 自带回归格（B2 · 仓外夹具）──────────────────────────

def _git(cwd, args):
    env = dict(os.environ)
    for v in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CONFIG",
              "GIT_CONFIG_PARAMETERS", "ZERG_STATE_DIR"):
        env.pop(v, None)
    return subprocess.run([GIT_TOK] + list(args), cwd=cwd, env=env,
                          capture_output=True, text=True, stdin=subprocess.DEVNULL)


def _fixture_repo(base, name):
    """真造仓 + 真造中文名件 ⇒ 路径（行为型的靶子，不靠源码里有没有某串）。"""
    d = os.path.join(base, name)
    os.makedirs(d)
    rc = _git(d, ["init", "-q", "."]).returncode
    if rc != 0:
        raise RuntimeError("合成仓建不起来")
    for fn in ("a.txt", "中文名件.md"):
        with open(os.path.join(d, fn), "w", encoding="utf-8") as f:
            f.write("x\n")
    _git(d, ["add", "--", "a.txt", "中文名件.md"])
    _git(d, ["-c", "user.email=t@zerg.local", "-c", "user.name=t", "commit", "-qm", "init"])
    return d


def selftest(out):
    """① 合规 ⇒ 全绿 ② 裸调 ⇒ 红 ③ `-c` 后置 ⇒ 红 ④ 大写拼法 ⇒ 红
       ⑤ 扫面 CJK 不瞎（行为型）⑥ 牙齿·拿掉 `-z` ⇒ 红 ⑦ 牙齿·拿掉判据 ⇒ 绿（含次数断言）。"""
    total = [0, 0]

    def case(name, want, got):
        total[0] += 1
        ok = want == got
        if not ok:
            total[1] += 1
        out.write("  %s %s（期望 %s · 实得 %s）\n" % ("✓" if ok else "✗", name, want, got))
        return ok

    base = tempfile.mkdtemp(prefix="gpo-selftest-")
    try:
        # ── 行为靶子：真仓 + 真跑 CLI ─────────────────────────────────────
        repo = _fixture_repo(base, "probe")
        bare = _git(repo, [N_LSFILES])
        verbatim = _git(repo, [N_LSFILES, F_Z])
        bare_lines = [l for l in bare.stdout.splitlines() if l.strip()]
        bare_escaped = sum(1 for l in bare_lines if l.startswith('"') and l.endswith('"'))
        z_names = [p for p in verbatim.stdout.split("\0") if p]
        case("行为靶子：裸调真产出转义形态（首尾带引号）件 ≥1", True, bare_escaped >= 1)
        case("行为靶子：合规写法真产出真 UTF-8 名（中文名件在内）", True, "中文名件.md" in z_names)
        case("行为靶子：同一仓两条路件数一致", len(bare_lines), len(z_names))

        # ── ⑤ 扫面自身对 CJK 名不瞎（行为型 · 真造件 + 真列件）──────────────
        files, _sk = iter_files(repo, scan_roots=(".",), exts=(".txt", ".md"), skip_dirs=frozenset())
        case("⑤ 扫面清单含中文名件（os.walk 天生不转义）", True, "中文名件.md" in files)

        # ── 夹具：四格文本（拼接构造，避免本件自指）──────────────────────
        ok_line = GIT_TOK + " " + N_LSFILES + " " + F_Z          # 合规
        bare_line = GIT_TOK + " " + N_LSFILES                    # ② 裸调
        late_c = GIT_TOK + " " + N_STATUS + " " + F_PORCELAIN + " " + F_C + " " + KEY_LOWER + "=false"
        upper = "-c " + KEY_UPPER + "=false " + GIT_TOK + " " + N_LSFILES + " " + F_Z

        def run_fixture(text, name, skip_rules=()):
            d = os.path.join(base, name)
            os.makedirs(d, exist_ok=True)
            with open(os.path.join(d, "case.sh"), "w", encoding="utf-8") as f:
                f.write(text + "\n")
            return scan(d, skip_rules=skip_rules, scan_roots=(".",),
                        exts=(".sh",), skip_dirs=frozenset())

        # ① 合规 ⇒ 全绿
        r1 = run_fixture(ok_line, "c1_ok")
        case("① 合规写法（带 -z）⇒ 命中 0", 0, len(r1["hits"]))

        # ② 裸调 ⇒ 红（GPO-F1）
        r2 = run_fixture(bare_line, "c2_bare")
        case("② 裸调（无开关）⇒ 报红 · 规则 GPO-F1", ["GPO-F1"], [h["rule"] for h in r2["hits"]])

        # ③ `-c` 后置 ⇒ 红（GPO-F2）
        r3 = run_fixture(late_c, "c3_late_c")
        case("③ `-c` 后置 ⇒ 报红 · 含 GPO-F2", True, "GPO-F2" in [h["rule"] for h in r3["hits"]])

        # ④ 大写拼法 ⇒ 红（GPO-F3）
        r4 = run_fixture(upper, "c4_upper")
        case("④ 大写拼法 ⇒ 报红 · 含 GPO-F3", True, "GPO-F3" in [h["rule"] for h in r4["hits"]])

        # ── ⑥ 牙齿·拿掉 `-z`（替换前后次数断言）────────────────────────────
        before_z = ok_line.count(F_Z)
        mutated = ok_line.replace(" " + F_Z, "")
        after_z = mutated.count(F_Z)
        out.write("    ⑥ 负控前置：`%s` 出现次数 替换前=%d · 替换后=%d\n" % (F_Z, before_z, after_z))
        case("⑥ 负控前置断言：拿掉 `-z` 后次数差 == 1", 1, before_z - after_z)
        case("⑥ 负控前置断言：替换真的改了文本", True, mutated != ok_line)
        r6 = run_fixture(mutated, "c6_noz")
        case("⑥ 牙齿：拿掉 `-z` 再跑 ⇒ 变红（GPO-F1）", True, "GPO-F1" in [h["rule"] for h in r6["hits"]])

        # ── ⑦ 牙齿·拿掉判据（替换前后次数断言）─────────────────────────────
        before_r = sum(1 for h in r2["hits"] if h["rule"] == "GPO-F1")
        r7 = run_fixture(bare_line, "c7_ruledrop", skip_rules=("GPO-F1", "GPO-F4"))
        after_r = sum(1 for h in r7["hits"] if h["rule"] == "GPO-F1")
        out.write("    ⑦ 负控前置：`GPO-F1` 命中数 摘掉判据前=%d · 摘掉后=%d\n" % (before_r, after_r))
        case("⑦ 负控前置断言：摘掉判据后 GPO-F1 命中差 == 1", 1, before_r - after_r)
        case("⑦ 负控：摘掉抓裸调的那颗判据 ⇒ 同一夹具不再报该条", [], [h["rule"] for h in r7["hits"]])

        # ── ⑧ 例外登记三条校验（含负控：缺 reason 必被拒）────────────────────
        case("⑧ 例外登记：现表通过三条校验（件存在性另由真仓跑覆盖）", [], validate_allowlist(base, check_exists=False))
        saved = OUTLET_ALLOW
        try:
            globals()["OUTLET_ALLOW"] = ({"path": OUTLET_PREFIX + "x.go", "rule": "GPO-F1",
                                          "reason": "", "replacement": "y"},)
            case("⑧ 例外负控：缺 reason ⇒ 校验报出", True, len(validate_allowlist(base, check_exists=False)) >= 1)
        finally:
            globals()["OUTLET_ALLOW"] = saved

        # ── ⑨ 牙齿：注释里提到裸调字面 ⇒ **不得报红**（文字面掩蔽）──────────
        r9 = run_fixture("# " + bare_line + "   ← 注释里提到裸调字面（旧口径）", "c9_comment")
        case("⑨ 注释里提到裸调字面 ⇒ 命中 0（文字面掩蔽 · 散文不是 argv）", 0, len(r9["hits"]))

        # ── ⑩ 文字面登记 = **锚定**，不是整件豁免 ─────────────────────────
        r10text = "# 注释里的旧口径：" + bare_line + "\n" + bare_line + "\n"
        saved_t = TEXT_ALLOW
        try:
            globals()["TEXT_ALLOW"] = ({
                "path": "case.sh", "rule": "GPO-F1",
                "anchor": "# 注释里的旧口径：",
                "reason": "夹具：证明登记锚定的是那一处文字", "replacement": "夹具（无需改）",
            },)
            r10 = run_fixture(r10text, "c10_anchor")
            case("⑩ 锚只对那一处文字生效 ⇒ 同件**真裸调行仍红**（恰 1 条 GPO-F1）",
                 ["GPO-F1"], [h["rule"] for h in r10["hits"]])
            d10 = os.path.join(base, "c10_anchor")
            case("⑩ 登记负控：锚真在件里 ⇒ 校验通过", [], validate_text_allow(d10))
            globals()["TEXT_ALLOW"] = (dict(TEXT_ALLOW[0], anchor="锚不在件里-XYZ"),)
            case("⑩ 登记负控：锚不在件里 ⇒ 校验报出（锚失效不许静默变绿）",
                 True, len(validate_text_allow(d10)) >= 1)
            globals()["TEXT_ALLOW"] = (dict(TEXT_ALLOW[0], anchor=""),)
            case("⑩ 登记负控：缺锚 ⇒ 校验报出（无锚=整件豁免=不许）",
                 True, len(validate_text_allow(d10)) >= 1)
        finally:
            globals()["TEXT_ALLOW"] = saved_t
        # ── ⑪ 全量字段 `hits_all`（新格：真跑主面 ⇒ 顶层字段在 · 条数 == hits · `top` 仍前 10）──
        _d11 = os.path.join(base, "c11_all", "scripts")
        os.makedirs(_d11, exist_ok=True)
        with open(os.path.join(_d11, "case.sh"), "w", encoding="utf-8") as f:
            f.write("\n".join([bare_line] * 12) + "\n")
        _sva, _svt, _sout, _sargv = (globals()["validate_allowlist"],
                                     globals()["validate_text_allow"], sys.stdout, sys.argv)
        _buf = io.StringIO()
        try:
            globals()["validate_allowlist"] = lambda *a, **k: []
            globals()["validate_text_allow"] = lambda *a, **k: []
            sys.argv = ["check-git-path-outlet.py", "--repo", os.path.join(base, "c11_all"), "--json"]
            sys.stdout = _buf
            main()
        finally:
            globals()["validate_allowlist"] = _sva
            globals()["validate_text_allow"] = _svt
            sys.stdout = _sout
            sys.argv = _sargv
        _j11 = {}
        for _ln in reversed(_buf.getvalue().strip().splitlines()):
            if _ln.startswith("{"):
                try:
                    _j11 = json.loads(_ln)
                    break
                except Exception:
                    continue
        case("⑪ `--json` 全量字段 `hits_all`：字段在 · 条数 == hits · `top` 仍只前 10 条",
             (True, 12, 10),
             ("hits_all" in _j11, len(_j11.get("hits_all", [])), len(_j11.get("top", []))))

        # ── ⑫ 扫面面空 / 根不存在 ⇒ rc=2 且点名（`GAP-20260928-11` 同族）—— 成对负控 ──
        #   判据面 = `res["scanned"]`（主面那一处口径，**不另开第三份**）。
        #   负控 A 根不存在 · 负控 B 空目录（存在但一件没有）⇒ 二者都必须 rc=2 + 点名；
        #   正控：把同一命令的 `--repo` 换成一个真有 `.sh` 件的仓 ⇒ rc=0（证明退码差
        #   只由「扫了几件」决定，不是被别的东西兜的）。
        _sva2, _svt2 = globals()["validate_allowlist"], globals()["validate_text_allow"]
        _sargv2 = sys.argv

        def _run_main(repo):
            _b = io.StringIO()
            try:
                globals()["validate_allowlist"] = lambda *a, **k: []
                globals()["validate_text_allow"] = lambda *a, **k: []
                sys.argv = ["check-git-path-outlet.py", "--repo", repo, "--json"]
                sys.stdout = _b
                _rc = main()
            finally:
                sys.stdout = _sout
            return _rc, _b.getvalue()

        try:
            _d12 = os.path.join(base, "c12_scan_real", "scripts")
            os.makedirs(_d12, exist_ok=True)
            with open(os.path.join(_d12, "case.sh"), "w", encoding="utf-8") as f:
                f.write("x\n")
            _rc_p, _out_p = _run_main(os.path.dirname(_d12))
            case("⑫ 正控：根里真有可扫件 ⇒ 判到（rc=0 · 扫面 1 件）",
                 (0, True), (_rc_p, "扫面 1 件" in _out_p))
            _d12e = os.path.join(base, "c12_scan_empty")
            os.makedirs(_d12e, exist_ok=True)
            _rc_a, _out_a = _run_main(os.path.join(base, "c12_根不存在-XYZ"))
            case("⑫ 负控 A：根不存在 ⇒ rc=2（不给结论）", 2, _rc_a)
            case("⑫ 负控 A 且逐字点名「扫面 0 件」", True, "扫面 0 件" in _out_a)
            _rc_b, _out_b = _run_main(_d12e)
            case("⑫ 负控 B：空目录（存在但一件没有）⇒ rc=2 —— 与正控同一条命令，只换根",
                 2, _rc_b)
            case("⑫ 负控 B 且逐字点名「扫面 0 件」", True, "扫面 0 件" in _out_b)
            _j12 = {}
            for _ln in reversed(_out_b.strip().splitlines()):
                if _ln.startswith("{"):
                    try:
                        _j12 = json.loads(_ln)
                        break
                    except Exception:
                        continue
            case("⑫ 负控 A/B 的机器面：refused=True · refuse_reason=empty_scan · scanned=0",
                 (True, "empty_scan", 0),
                 (_j12.get("refused"), _j12.get("refuse_reason"), _j12.get("scanned")))
        finally:
            globals()["validate_allowlist"], globals()["validate_text_allow"] = _sva2, _svt2
            sys.argv = _sargv2
    finally:
        shutil.rmtree(base, ignore_errors=True)
    out.write("  自检 %d 格 · 失败 %d 格\n" % (total[0], total[1]))
    return total[1] == 0


# ─────────────────────────── 主面 ───────────────────────────

LIST_RULES = """判据（C-6 · 黑名单五类）
--------------------------
  GPO-F1  裸取件名：ls-files / ls-tree -r|--name-only / diff --name-only /
          status --porcelain|--short / grep —— argv 里 `-z` 与 `-c` 都不带
  GPO-F2  `-c` 后置在子命令之后（位置铁律；`ls-files … -c …` 是 rc=0 的静默错）
  GPO-F3  大写拼法那一形（本单只认全小写）
  GPO-F4  其余会转义子命令（diff-tree / show --name-only / log --name-only / diff --stat）无免疫开关
  GPO-F5  取件名输出直入匹配 / JSON / 拼路径（未经出口）
★ GPO-F5 只在调用**不带**免疫开关时判（「未经出口」的字面义）
白名单例外：**仅** core/internal/gitpaths/ 的出口实现件本身；每条必带 reason + replacement
文字面（非 argv）例外：逐 **(件, 规则, 锚)** 登记 + reason + replacement —— 注释/帮助串/消息串/文档散文
档位：--tier report（默认·只报告 rc=0）· --tier rc（命中 ⇒ rc=1）
退码：0 判到 · 1 rc 档有命中 · 2 不给结论（扫面空 / 例外登记不合格 / 自检不过 / 用法错）
"""

JSON_FIELDS = ["schema", "gate", "version", "tier", "scanned", "skipped", "hits", "exceptions",
               "top", "not_judged", "mode", "hits_all",
               "refused", "refuse_reason", "allowlist_bad"]


def refuse_unknown_json_fields(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    return 2


def _refuse_machine_face(a, tier, res, reason, bad=()):
    """「不给结论（rc=2）」路的**机器面** —— **先出机器面，再退 2**。

    ★ 缺口收口（`GAP-20260927-216` ③ 同族）：退码该是 2 **不变**，但机器面不许跟着一起没 ——
      否则「机器面缺」与「零命中」在机器面上分不开（一锚失效时，连命中数都打不出来）。
    """
    if not a.json:
        return
    hits = res["hits"]
    sys.stdout.write(json.dumps({
        "schema": "zerg-gate/v1", "gate": "check-git-path-outlet", "version": VERSION,
        "tier": tier, "scanned": len(res["scanned"]), "skipped": len(res["skipped"]),
        "hits": len(hits), "exceptions": len(res["exceptions"]),
        "top": [{"file": h["file"], "line": h["line"], "rule": h["rule"]} for h in hits[:10]],
        "hits_all": [{"file": h["file"], "line": h["line"], "rule": h["rule"], "why": h["why"]}
                     for h in hits],
        "not_judged": [r for r, _ in NOT_JUDGED], "mode": "refuse",
        "refused": True, "refuse_reason": reason, "allowlist_bad": list(bad),
    }, ensure_ascii=False) + "\n")


def main() -> int:
    rc = refuse_unknown_json_fields()
    if rc is not None:
        return rc
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(description="件名统一出口的调用点判据门（C-6 · GAP-20260927-216 ③）")
    ap.add_argument("--repo", default=repo_default)
    ap.add_argument("--tier", default="report", choices=("report", "rc"))
    ap.add_argument("--strict", action="store_true", help="= --tier rc")
    ap.add_argument("--json", default=None, nargs="?", const="full")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        sys.stdout.write("── 自带回归格（仓外夹具 · 真造仓 + 真跑 CLI）──\n")
        return 0 if selftest(sys.stdout) else 2

    tier = "rc" if (a.strict or a.tier == "rc") else "report"
    bad = validate_allowlist(a.repo) + validate_text_allow(a.repo)
    # ★ 登记不合格也**照跑扫面**：读数不许被退码遮蔽（下面那两条「不给结论」都要报数）。
    res = scan(a.repo)

    if bad:
        # ★ 机器面**先出**、再退 2：一锚失效 ⇒ 退码仍是 2（判据一字未动），但
        #   「命中数 / 例外数 / 哪条锚失效」不许被退码一起吞掉 —— 机器面缺 ≠ 零命中。
        _refuse_machine_face(a, tier, res, "allowlist_invalid", bad)
        sys.stdout.write("⛔ 不给结论（rc=2）：白名单例外登记不合格 —— %d 条：\n" % len(bad))
        for b in bad:
            sys.stdout.write("    · %s\n" % b)
        sys.stdout.write("  扫面 %d 件 · 命中 **%d** 条 · 白名单例外命中 **%d** 条\n"
                         % (len(res["scanned"]), len(res["hits"]), len(res["exceptions"])))
        return 2

    if not res["scanned"]:
        _refuse_machine_face(a, tier, res, "empty_scan")
        sys.stdout.write("⛔ 不给结论（rc=2）：扫面 0 件（抽取器被改坏 / 形态变了 ⇒ 空转 = 假绿）\n")
        return 2

    hits, exc = res["hits"], res["exceptions"]
    sys.stdout.write("件名统一出口的调用点判据（C-6）· v%s · 档位 **%s**\n" % (VERSION, tier))
    sys.stdout.write("  扫面 %d 件（os.walk 列件 · 不经 git ⇒ 对 CJK 名天生不瞎）· 跳过 %d 件\n"
                     % (len(res["scanned"]), len(res["skipped"])))
    if res["skipped"]:
        for rel, why in res["skipped"][:5]:
            sys.stdout.write("      · 跳过 %s（%s）\n" % (rel, why))
    sys.stdout.write("  命中 **%d** 条 · 白名单例外命中 **%d** 条（例外域：%s）\n"
                     % (len(hits), len(exc), OUTLET_PREFIX))
    for h in hits[:10]:
        sys.stdout.write("    ✗ %s:%d  [%s] %s\n" % (h["file"], h["line"], h["rule"], h["why"]))
        sys.stdout.write("        %s\n" % h["snippet"])
    if len(hits) > 10:
        sys.stdout.write("    …（还有 %d 条，见 --json full）\n" % (len(hits) - 10))
    for e in exc[:10]:
        sys.stdout.write("    ○ 例外 %s:%d  [%s] reason=%s\n"
                         % (e["file"], e["line"], e["rule"], e["exception"]["reason"][:60]))
    for rel, why in NOT_JUDGED:
        sys.stdout.write("    ○ 未判（点名登记 · 不判）：%s —— %s\n" % (rel, why))

    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-git-path-outlet", "version": VERSION,
            "tier": tier, "scanned": len(res["scanned"]), "skipped": len(res["skipped"]),
            "hits": len(hits), "exceptions": len(exc),
            "top": [{"file": h["file"], "line": h["line"], "rule": h["rule"]} for h in hits[:10]],
            "hits_all": [{"file": h["file"], "line": h["line"], "rule": h["rule"], "why": h["why"]}
                         for h in hits],
            "not_judged": [r for r, _ in NOT_JUDGED], "mode": tier,
        }, ensure_ascii=False) + "\n")

    if hits and tier == "rc":
        sys.stdout.write("⇒ 判红（rc=1 · rc 档）：%d 条裸取件名调用点 ⇒ 转义形态会静默进判定\n" % len(hits))
        return 1
    if hits:
        sys.stdout.write("⇒ 判到了（rc=0 · **report 档**）：%d 条 —— 不阻断；升档用 `--tier rc`\n" % len(hits))
        return 0
    sys.stdout.write("⇒ 全绿（rc=0）：%d 件里没有裸取件名调用点\n" % len(res["scanned"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-wired-scripts.py — 门③：脚本接线自检（**挂闸档：`tri` + `--strict-report` = 阻断**）

挂接现状（2026-09-18 ③批次 · 升阻断）
------------------------------------
本门起初按门③ 自己的建议**只报告档**起步（2026-09-18 首跑如实报「断言 A 命中 8 只未挂」= 存量债，
直接阻断等于把提交闸锁死 ⇒ 同 D2 先例）。③批次把这 8 只**逐只挂了闸**（docs +2 · pub +2 ·
新 scope `tools` +1 · 新 scope `slice` +1 · 发布闸 闸②b/闸⑥），首跑 A 命中 8→0、B 未登记 0
⇒ 按「清到 0 再升阻断」的拍板把它升成：提交闸 `gates` scope · 步骤 `门③ 接线：scripts 门脚本有没有被闸调用（阻断）`
· 模式 `tri` · 命令串 `python3 scripts/check-wired-scripts.py --strict-report`。
★ 升档要**两处一起改**（模式 `tri-report`→`tri` **且**命令串加 `--strict-report`）：脚本自身默认仍是
「只报告」（rc=0，命中只入清单）——**不带这个开关就等于挂了个恒绿步**（假阻断）。成对证据：
同一夹具同一命中，不带开关 rc=0 / 带开关 rc=1（`--root <夹具> --strict-report`）；升档全过程见
`scripts/check-wired-scripts.md` §五（已执行）。
★ 判据与退码**一字未改**：改的只是「挂闸时的档位 + 一个开关」。

为什么存在
----------
2026-09-18 存量债务普查（docs/项目文档/v2.5.10/债务台账-20260918.md，§0 表 #11 与 §5）
记下两条同族的「假覆盖」：

  ① **有门不挂**：`check-glossary.py` / `check-manifest-freshness.py` / `check-slice.py` /
     `check-i18n-drift.sh` / `check-tool-version-sync.sh` 这 5 只门脚本**不在任何闸里** ——
     门写了、脚本绿过、但没有任何一步会替你跑它 ⇒ 它在提交时等于不存在（假覆盖）。
  ② **孤岛死重**：`scripts/` 下有零引用件（既不被 scripts/ 内调用、也不被别处引用），
     它们躺在仓里没人知道该不该留。

本门把这两条写成**可判定的**判据（不是词表、不是「看着像」）：

  断言 A（门脚本必须上岗）：凡**门脚本** —— 命名以 `check-` 开头的脚本，或**说明书自称门**
    的脚本（同干 `.md` 的头部自称门）—— 必须出现在**至少一个闸**里。
    闸 = 提交闸（`scripts/precommit-gates.sh`）+ 发布闸面（见下方 GATES，现读其真实文件名与调用方式）。
    「出现在闸里」的判据 = **按名调用形态**：名字出现在闸文件的一行上，且该行
      ① 不是纯注释行（`#` 开头）——「注释里提过」不算上岗；
      ② 不是纯清单项（整行就是 `"scripts/<名>"` 这类数组/数据项）——「在 EXCLUDES 里被排除」不算上岗。
    这两条排除是**有牙齿的**：`check-hardcoded-private-paths.py` 今天只被
    `scripts/publish-public.sh:130`（EXCLUDES 清单项）与 `scripts/precommit-gates.sh:500`（注释）提到，
    从未被任何闸按名执行 ⇒ 它必须被报出来（子串口径会把它误判成「在闸里」）。

  断言 B（零引用件必须登记）：`scripts/` 顶层、**零引用**的脚本必须登记白名单（带 reason + date）。
    「引用」判据 = 该脚本名在**引用面**（见下方口径）里的出现次数。见 §口径 的三条排除。

退码（三档 · 与仓内既有门的 0/1/2 惯例一致）
--------------------------------------------
  0 = 只报告档的正常出口：**无论命中多少**（断言 A 命中 N 只、断言 B 未登记 M 只）
  1 = **仅**在 `--strict-report` 下、且至少一处命中时
  2 = **不给结论**（缺件 / 配置错 / 空转 / 自检不过）—— 一律显式打印，不许静默
      · 仓根缺 `scripts/` 目录 · 闸清单**一个都不在位** · 候选 0 只 · 引用面 0 个文件
      · 白名单项缺 reason / 缺 date / date 形态不对 / 重名 / **悬空**（点名的脚本已不存在）
      · `--self-test` 任一用例不过（自检不过 ⇒ 拒绝扫真目标，项目口径）

用法
----
    python3 scripts/check-wired-scripts.py                 # 只报告（rc=0）
    python3 scripts/check-wired-scripts.py --strict-report # 命中即 rc=1（临时用；常规挂闸另见 .md）
    python3 scripts/check-wired-scripts.py --list-rules    # 打判据 / 闸清单 / 口径 / 排除域 / 升级路径
    python3 scripts/check-wired-scripts.py --self-test     # 只跑自检（成对负控，不碰真目标）
    python3 scripts/check-wired-scripts.py --root DIR --whitelist J # 对夹具跑（自检内部用）

说明书：scripts/check-wired-scripts.md（为何起步只报告 · 怎么升阻断 · 两断言各自的自证方式）。
"""

import argparse
import datetime
import fnmatch
import json
import os
import re
import subprocess
import sys
import tempfile
import unicodedata

# ─────────────────────────────────────────────────────────────────────────────
# 闸清单（现读 2026-09-18 的仓内实况；改名/搬家必须同步本表与 .md §二）
# 判据只用这份表 —— 「还有哪些文件跑脚本」不进判据（否则任何脚本都能自称闸）。
# ─────────────────────────────────────────────────────────────────────────────
GATES = [
    # (路径, 类别, 标签, 现读到的调用方式)
    ("scripts/precommit-gates.sh", "commit", "提交闸",
     "bash scripts/precommit-gates.sh [--scope go|rust|pub|tags|docs]（步骤表 add_step 的 STEP_CMD）"),
    ("scripts/publish-preflight.sh", "release", "发布闸·推送前置（闸⓪~⑥，含闸②b）",
     'bash scripts/publish-preflight.sh <产物目录>（推送前硬闸，八道全过才允许推；rc=2 = 有闸缺件不给结论）'),
    ("publish/mirror-public.sh", "release", "发布闸·逐提交镜像器",
     "publish/mirror-public.sh --out DIR [--push]（对产出树跑门禁）"),
    ("publish/mirror-public-lib.py", "release", "发布闸·镜像器库（跑 checker 的那半边）",
     "被 publish/mirror-public.sh / publish/parity-compare.py 调用（check-history-secrets.py / check-public-tree-private.py 的调用方）"),
    ("publish/ci/ci.yml", "release", "发布闸·CI（公开仓推后闸）",
     "GitHub Actions：run: python3 scripts/... （check_version / check-gotoolchain / check-compat-manifest / check_docs / check-shell-unicode-vars）"),
    ("publish/ci/release-agent.yml", "release", "发布闸·CI（Release 重切）",
     "GitHub Actions：python3 scripts/check_version.py · scripts/make-manifest.py"),
]

# 相关但不是闸（打印为「排除域」；判据不吃它们 —— 别把验收套件/弃用器/测试当闸）
NOT_GATES = [
    ("publish/mirror-acceptance.sh", "B1 发布器验收套件（逐条能失败，属测试面）"),
    ("scripts/publish-public.sh", "压平快照发布器 · 已弃用（Q5 待废弃；现役为 mirror-public.sh）"),
    ("scripts/test-publish-parity.sh", "两器一致性测试（测试面）"),
]

# ─────────────────────────────────────────────────────────────────────────────
# 引用面的三条排除（判据的一部分，写死在代码里、打印在输出里，改它要改 .md）
#   ① 本门自指的三件文件：否则我自己的名单/说明书会把「零引用」直接写成「有引用」（自伤）。
#   ② 本门的**结论落账面**：现行债务台账 —— 「记录不能充作引用」。
#      台账点名一个死件不会让它活过来；若把记录算作引用，那「写下这条债」这个动作本身
#      就把债抹掉了（观测行为抹掉被观测事实）。这是本门判据里唯一需要解释的一条。
#   ③ 候选脚本自身（`scripts/<名>` 那一个文件）。
# 口径对照（现跑，2026-09-18）：全仓含记录面 = 0 只零引用；本门口径 = 4 只；
#   再排除 docs/ 整个文档面 = 29 只（那会把「只在文档里指路」的活门误判成死件 ⇒ 不采）。
# ─────────────────────────────────────────────────────────────────────────────
SELF_FILES = [
    "scripts/check-wired-scripts.py",
    "scripts/check-wired-scripts.md",
]
RECORD_SURFACES = [
    "docs/项目文档/*/债务台账-*",   # 本门结论的落账面（glob ⇒ 换日期/换版本不用改代码）
]

# ─────────────────────────────────────────────────────────────────────────────
# 白名单（零引用件的登记表 · 逐条 reason + date）
#   规矩（棘轮）：**只许减不许增** —— 新写一条必须能说出「它为什么还该在仓里」；
#   已登记的件一旦有了引用（接线/被别的脚本调用），下次跑会提示可下架。
#   ✗ 不许为了让首跑好看而填空：登记项必须是真死件（附理由），不是「先压下去再说」。
# ─────────────────────────────────────────────────────────────────────────────
WHITELIST = [
    {
        "name": "start-watchdog.sh",
        "reason": "手工兜底启动器（非死件）：以「脱离会话」方式拉起主控保活看门狗；"
                  "主控已由 launchd（com.zerg.core · KeepAlive）托管 ⇒ 运行时不再需要它，"
                  "保留给「不想用 launchd」那种情况（脚本头注释 2-5 行自述）。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_more_classes.py",
        "reason": "一次性历史评测脚本：v2.5 补充任务类测试（example-35b-v2 · 2026-08-13），"
                  "结果已并入当版文档；保留作评测口径的可复核原件。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_more_tasks.py",
        "reason": "一次性历史评测脚本：v2.5 更多实战任务（2026-08-13），同族口径。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_real10_v2.py",
        "reason": "一次性历史评测脚本：v2.5 真实任务集重测（responses 后 · 2026-08-13），同族口径。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_real10.py",
        "reason": "一次性历史评测脚本：v2.5 真实任务集（2026-08-13，同族口径见 v25_real10_v2.py）。"
                  "它原本唯一的引用 = `docs/issues/live-task2.log`（引擎运行态日志，非跟踪件）——"
                  "该目录已于 2026-09-19 随开发文档分家整目录迁出工作树（`Zerg-内部文档/issues/`）"
                  "⇒ 引用面归零，**接线已不可行**（不能为了凑引用去造一个调用方）。"
                  "登记自证：登记前 断言 B「未登记 1 只」= FAIL(rc=1) ↔ 登记后 白名单 5 条、"
                  "未登记 0 只 = rc=0（豁免数只增这 1 条，逐条带 reason+date）。",
        "date": "2026-09-19",
    },
]

# ─────────────────────────────────────────────────────────────────────────────
# 断言 A 的「建议挂到哪里」（逐条给理由/证据；键 = 脚本名）
#   本表只影响**报告的可行动性**，不影响判据 —— 没登记建议的命中项会打印「未登记建议 ⇒ 人工判定」。
# ─────────────────────────────────────────────────────────────────────────────
SUGGEST = {
    "check-glossary.py": (
        "提交闸 · docs scope（模式 tri-report 起步）",
        "与 check-doc-* 同族（文档/术语面），docs 已在默认 scope 集里（precommit-gates.sh:47-50）；"
        "现用法 `python3 scripts/check-glossary.py --lang zh|en [--enforce]`（docs/zh/README.md:56 · docs/en/README.md:98）。"
        "术语档 T2 现为非阻塞 ⇒ 起步按「只报告」挂，同 D2 先例。"),
    "check-manifest-freshness.py": (
        "发布闸 · scripts/publish-preflight.sh 新增一道（对产物目录判清单新鲜度，模式 tri）",
        "它判的是**发布产物**（manifest 的新鲜度），归发布面；"
        "现形态是「只告警不拒绝」，退出码恒 0 除 `--strict`（Zerg-内部文档/调研/调研-文档分块落地-门禁与迁移.md:373 —— 该件 2026-09-19 已随开发文档分家迁出工作树，原路径 docs/调研/）。"),
    "check-slice.py": (
        "提交闸 · 新 scope（例 slice，模式 tri-report 起步）",
        "自带 `--probe` / `--selftest` / `--list-rules` 三档与「自检不过 rc=2」的口径"
        "（Zerg-内部文档/01-设计/报告模板-六项成对.md:69-70 —— 该件 2026-09-19 已随开发文档分家迁出工作树，原路径 docs/01-设计/）⇒ 可直接当步骤；"
        "但体量大（109 KB + slice-probes.json）⇒ 成本未标定，起步只报告。"),
    "check-i18n-drift.sh": (
        "提交闸 · docs scope（模式 tri-report 起步）",
        "漂移三件套（配对 / 提交级陈旧 / 源注释），①②阻塞、③告警（docs/en/README.md:99）；"
        "用法 `bash scripts/check-i18n-drift.sh`（docs/site/README.md:189）；存量债未清 ⇒ 起步只报告。"),
    "check-tool-version-sync.sh": (
        "提交闸 · 新 scope（例 tools，模式 tri）",
        "它就是为「发布前勾选」造的（docs/skills/tool-upgrade-checklist.md:52 列在手检清单里：台账 ↔ 工具文档抬头 ↔ 履历）；"
        "自带前置自检（缺 tools/versions.json 即 rc=2）⇒ 与 tri 三档一一对应，可直接阻断。"),
    "check-hardcoded-private-paths.py": (
        "发布闸 · scripts/publish-preflight.sh 新增一道（紧跟闸② 私有面，模式 tri）",
        "它判私有绝对路径，与 闸②（私有面）/ 闸④（危险路径）同族；"
        "今天它只被 scripts/publish-public.sh:130 的 EXCLUDES **清单项** 与 precommit-gates.sh:500 的**注释**提到 —— "
        "从没被任何闸按名执行 ⇒ 按本门判据必须报出来（子串口径会漏掉这一只）。"),
    "edit-assert": (
        "提交闸 · pub scope 增一步 `python3 scripts/edit-assert --self-test`（模式 tri）",
        "它今天只被 pub scope 的「scripts/*（无后缀 + 首行 #!）按 shebang 语法」步按 **glob** 做语法检查"
        "（precommit-gates.sh:521），**没有按名跑过它的自检**；自检自带真命令行 + 真退出码 16 条（scripts/edit-assert.md:34）⇒ 有牙齿。"),
    "mutate-scan": (
        "提交闸 · pub scope 增一步 `python3 scripts/mutate-scan --self-test`（模式 tri）",
        "同族：只在 glob 语法检查里被覆盖；自检不过即拒绝跑真目标（scripts/mutate-scan.md:27, 48）⇒ 适合当步骤。"),
}

# ─────────────────────────────────────────────────────────────────────────────
# 报告宽度计算（中文按 2 列宽；本机 python3 = 3.9，不用 3.10+ 语法）
# ─────────────────────────────────────────────────────────────────────────────
def dw(s):
    return sum(2 if unicodedata.east_asian_width(c) in ("W", "F") else 1 for c in s)

def pad(s, n):
    s = str(s)
    w = dw(s)
    return s + " " * max(0, n - w)

def trunc(s, n):
    s = str(s)
    if dw(s) <= n:
        return s
    out = ""
    for c in s:
        if dw(out) + dw(c) > n - 1:
            return out + "…"
        out += c
    return out

def rel(root, p):
    try:
        return os.path.relpath(p, root)
    except Exception:
        return p

# ─────────────────────────────────────────────────────────────────────────────
# 扫描面
# ─────────────────────────────────────────────────────────────────────────────
def git_out(root, args):
    try:
        r = subprocess.run(["git"] + args, cwd=root, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE)
    except OSError:
        return None
    if r.returncode != 0:
        return None
    return r.stdout.decode("utf-8", "surrogateescape")

SKIP_DIRS = {".git", ".zerg", "node_modules", "__pycache__", "target", "dist",
             "vendor", "venv", ".venv", "build", ".idea", ".vscode"}
SKIP_EXT = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".pdf", ".gguf", ".bin", ".db",
            ".zip", ".tar", ".gz", ".so", ".dylib", ".a", ".o", ".woff", ".woff2",
            ".ttf", ".mp4", ".webp", ".pt", ".safetensors", ".lock", ".pyc"}
MAX_BYTES = 3 * 1024 * 1024

def list_files(root):
    """引用面/候选面的文件清单。

    · 真仓（有 .git）⇒ `git ls-files`（= 索引里的已跟踪文件；**未跟踪件不在判据内**，
      与债务台账的现读口径 `git grep` 一致，也让并行路上的半成品不会抖动本门数字）。
    · 非 git 目录（自检夹具）⇒ 文件系统遍历，同样排除 SKIP_DIRS。
    返回 (files, mode)。
    """
    if os.path.isdir(os.path.join(root, ".git")):
        out = git_out(root, ["ls-files", "-z"])
        if out:
            fs = [p for p in out.split("\0") if p]
            return fs, "git-ls-files（已跟踪 = 索引）"
    fs = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        for fn in filenames:
            p = os.path.join(dirpath, fn)
            if os.path.islink(p):
                continue
            fs.append(rel(root, p))
    return sorted(fs), "文件系统遍历（非 git 目录）"

def read_text(path):
    try:
        if os.path.getsize(path) > MAX_BYTES:
            return None
        with open(path, "rb") as f:
            d = f.read()
        if b"\0" in d[:8192]:
            return None
        return d.decode("utf-8", "surrogateescape")
    except Exception:
        return None

def is_record_surface(p):
    """记录面判定：整串 fnmatch（`*` 在本实现里可跨 `/`）+ 逐段 fnmatch 兜一层。

    注意 fnmatch 的 `*` 会匹配到 `/`（与 glob 语义不同）—— 对本门的记录面 glob
    （`docs/项目文档/*/债务台账-*`）正是想要的：* 吃到版本目录名。
    """
    for pat in RECORD_SURFACES:
        if fnmatch.fnmatch(p, pat):
            return True
        seg = pat.split("/")
        parts = p.split("/")
        if len(parts) == len(seg) and all(fnmatch.fnmatch(a, b) for a, b in zip(parts, seg)):
            return True
    return False

# ─────────────────────────────────────────────────────────────────────────────
# 候选（门脚本）
# ─────────────────────────────────────────────────────────────────────────────
DOC_MARK = "门"          # 说明书自称门的标记（同干 .md 头部出现即算）
DOC_HEAD_LINES = 60

def is_gate_file(p):
    return any(g[0] == p for g in GATES)

def build_candidates(root, files):
    """返回 (cands, notes)。

    候选面 = scripts/ **顶层**所有脚本（.py / .sh / 无后缀但首行 #!）—— **两断言共用同一份**：
      · 断言 A 只吃其中 kind 非空的门脚本（命名 check- / 说明书自称门）；
      · 断言 B 吃全部（零引用件不限门脚本 —— 今天的 4 只里三只是 v25_* 评测脚本）。
    """
    cands = []
    notes = []
    tops = [p for p in files
            if p.startswith("scripts/") and p.count("/") == 1
            and p != "scripts/"]
    for p in sorted(tops):
        ap = os.path.join(root, p)
        base = os.path.basename(p)
        if not os.path.isfile(ap):
            continue
        kind = ""
        evidence = ""
        if not base.endswith((".py", ".sh")):
            try:
                with open(ap, "rb") as f:
                    head = f.read(2).decode("latin-1")
            except Exception:
                head = ""
            if head != "#!":
                continue                      # 无后缀又没 shebang ⇒ 不是脚本（数据/目录/别的）
        if base.startswith("check-"):
            kind = "命名 check-"
            evidence = "名以 check- 开头"
        else:
            # 说明书自称门：同干 .md 的头部出现「门」
            stem = os.path.splitext(base)[0]
            md = "scripts/%s.md" % stem
            if md in files:
                t = read_text(os.path.join(root, md))
                if t:
                    for i, line in enumerate(t.splitlines()[:DOC_HEAD_LINES], 1):
                        if DOC_MARK in line:
                            kind = "说明书自称门"
                            evidence = "%s:%d 「%s」" % (md, i, trunc(line.strip(), 46))
                            break
        cands.append({"name": base, "path": p, "kind": kind, "evidence": evidence})
    return cands, notes

# ─────────────────────────────────────────────────────────────────────────────
# 断言 A：出现在至少一个闸里（按名调用形态）
# ─────────────────────────────────────────────────────────────────────────────
def bare_item_re(name):
    return re.compile(r'^["\']?(?:\./)?(?:scripts/)?' + re.escape(name) + r'["\']?,?$')

def wiring_hit(root, gate_path, name):
    """在闸文件里找「按名调用形态」；返回 (行号, 行内容) 或 None。

    排除两类不算上岗（见模块头）：纯注释行、纯清单项。
    """
    p = os.path.join(root, gate_path)
    t = read_text(p)
    if t is None:
        return None
    rx = bare_item_re(name)
    for i, line in enumerate(t.splitlines(), 1):
        if name not in line:
            continue
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if rx.match(s):
            continue
        return (i, s)
    return None

def judge_a(root, cands):
    """返回 (hits, wired)：hits = 未挂的候选；wired = 已挂的候选（附证据）"""
    hits, wired = [], []
    n_gate_present = 0
    missing_gates = []
    for g, kind, label, how in GATES:
        if os.path.isfile(os.path.join(root, g)):
            n_gate_present += 1
        else:
            missing_gates.append(g)
    for c in cands:
        if is_gate_file(c["path"]):
            wired.append((c, "闸自身（它就是闸）", "%s" % c["path"]))
            continue
        found = None
        for g, kind, label, how in GATES:
            h = wiring_hit(root, g, c["name"])
            if h:
                found = (g, label, h[0], h[1])
                break
        if found:
            wired.append((c, found[0], ":%d  %s" % (found[2], trunc(found[3], 56))))
        else:
            hits.append(c)
    return hits, wired, n_gate_present, missing_gates

# ─────────────────────────────────────────────────────────────────────────────
# 断言 B：零引用件必须登记白名单
# ─────────────────────────────────────────────────────────────────────────────
def ref_counts(root, files, name, self_rel):
    """返回 (total, refs_code, refs_doc, sample_paths_sorted)"""
    total = 0
    code = 0
    doc = 0
    samples = []
    for p in files:
        if p == self_rel:
            continue
        if p in SELF_FILES:
            continue
        if is_record_surface(p):
            continue
        t = read_text(os.path.join(root, p))
        if t is None:
            continue
        if name not in t:
            continue
        n = t.count(name)
        total += n
        if p.startswith("docs/") or "/docs/" in p:
            doc += n
        else:
            code += n
        samples.append("%s(x%d)" % (trunc(p, 52), n))
    return total, code, doc, sorted(samples, key=lambda s: (-dw(s), s))

def validate_whitelist(root, cand_names, wl):
    """返回 (errs, entries_by_name)。配置错一律 rc=2，不许静默。"""
    errs = []
    seen = {}
    for i, e in enumerate(wl, 1):
        n = (e.get("name") or "").strip()
        r = (e.get("reason") or "").strip()
        d = (e.get("date") or "").strip()
        if not n:
            errs.append("白名单第 %d 项缺 name" % i)
            continue
        if not r:
            errs.append("白名单「%s」缺 reason（登记必须说清它为什么还该在仓里）" % n)
        if not d:
            errs.append("白名单「%s」缺 date" % n)
        elif not re.match(r"^\d{4}-\d{2}-\d{2}$", d):
            errs.append("白名单「%s」的 date 形态不对（要 YYYY-MM-DD，现为 %r）" % (n, d))
        if n in seen:
            errs.append("白名单重名「%s」（第 %d 项与第 %d 项）" % (n, seen[n], i))
        seen[n] = i
        if n not in cand_names:
            errs.append("白名单**悬空**「%s」：scripts/ 顶层此刻没有这只脚本（删件后必须同步下架）" % n)
    return errs, seen

def judge_b(root, files, cands, wl_names):
    rows = []
    for c in cands:
        total, code, doc, samples = ref_counts(root, files, c["name"], c["path"])
        rows.append({"name": c["name"], "total": total, "code": code,
                     "doc": doc, "samples": samples,
                     "wl": c["name"] in wl_names})
    zero = [r for r in rows if r["total"] == 0]
    unregistered = [r for r in zero if not r["wl"]]
    registered = [r for r in zero if r["wl"]]
    # 已登记但已有引用 ⇒ 提示可下架（棘轮只许减）
    stale = [r for r in rows if r["wl"] and r["total"] > 0]
    doc_only = [r for r in rows if r["total"] > 0 and r["code"] == 0]
    return zero, unregistered, registered, stale, doc_only, rows

# ─────────────────────────────────────────────────────────────────────────────
# 报告
# ─────────────────────────────────────────────────────────────────────────────
def header(root, files, mode, n_scan):
    head = "?"
    dirty = "?"
    if os.path.isdir(os.path.join(root, ".git")):
        h = git_out(root, ["rev-parse", "--short", "HEAD"])
        head = (h or "").strip() or "?"
        s = git_out(root, ["status", "--porcelain"])
        dirty = len([x for x in (s or "").splitlines() if x.strip()])
    return head, dirty

def report(root, opts, files, mode, a_cands, b_cands, hits_a, wired_a, n_gate_present, missing_gates,
           zero, unreg, reg, stale, doc_only, rows, wl, errs):
    out = []
    ts = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    head, dirty = header(root, files, mode, len(files))
    n_check = len([c for c in a_cands if c["kind"] == "命名 check-"])
    n_decl = len([c for c in a_cands if c["kind"] == "说明书自称门"])
    out.append("门③ check-wired-scripts — 脚本接线自检（挂闸档 `tri` + `--strict-report` ⇒ 阻断；不带开关时默认只报告）")
    out.append("")
    out.append("仓根   : %s" % root)
    out.append("快照   : %s ｜ HEAD %s ｜ 工作树改动 %s 件" % (ts, head, dirty))
    out.append("扫描面 : A 候选（门脚本）%d 只（命名 check- %d · 说明书自称门 %d）"
               % (len(a_cands), n_check, n_decl))
    out.append("         B 候选（scripts/ 顶层脚本）%d 只" % len(b_cands))
    out.append("         引用面 %d 个文件（%s；排除 自指 %d · 记录面 %d · 候选自身）"
               % (len(files), mode, len(SELF_FILES),
                  len([p for p in files if is_record_surface(p)])))
    out.append("闸清单 : 在位 %d / %d（提交闸 %d · 发布闸 %d）%s"
               % (n_gate_present, len(GATES),
                  len([g for g in GATES if g[1] == "commit"]),
                  len([g for g in GATES if g[1] == "release"]),
                  "" if not missing_gates else "  ⚠ 不在位：%s" % " · ".join(missing_gates)))
    for g, kind, label, how in GATES:
        mark = "✓" if os.path.isfile(os.path.join(root, g)) else "✗"
        out.append("         %s %-14s %s" % (mark, label, g))

    out.append("")
    out.append("【断言 A】门脚本必须出现在至少一个闸里 —— 命中 %d 只（未挂）" % len(hits_a))
    if not hits_a:
        out.append("  ✓ 无未挂门脚本")
    else:
        out.append("   %s %s %s %s %s" % (pad("#", 4), pad("脚本", 30), pad("类型", 14),
                                          pad("闸里按名调用", 12), "建议挂到哪里"))
        for i, c in enumerate(hits_a, 1):
            sug, why = SUGGEST.get(c["name"], ("未登记建议 ⇒ 人工判定（该补哪条闸由人定）", ""))
            out.append("   %s %s %s %s %s" % (pad(i, 4), pad(c["name"], 30),
                                              pad(c["kind"], 14), pad("0 处", 12),
                                              trunc(sug, 60)))
            if why:
                for j, seg in enumerate(re.split(r"；|；", why)):
                    out.append("       %s %s" % ("理由：" if j == 0 else "      ", trunc(seg, 96)))
    out.append("  已挂门脚本 %d 只（不打印；--list-rules 可看闸清单与调用方式）" % len(wired_a))

    out.append("")
    out.append("【断言 B】零引用件必须登记白名单 —— 候选 %d 只 · 零引用 %d 只 · **未登记 %d 只**"
               % (len(rows), len(zero), len(unreg)))
    if reg:
        out.append("  已登记（%d 只）：" % len(reg))
        by_name = {e["name"]: e for e in wl if e.get("name")}
        for r in reg:
            e = by_name.get(r["name"], {})
            out.append("    %s  date=%s" % (pad(r["name"], 26), e.get("date", "?")))
            out.append("      reason: %s" % trunc(e.get("reason", "?"), 200))
    if unreg:
        out.append("  ✗ 未登记（%d 只）—— 要么接线（被谁调用）、要么登记白名单（附理由）：" % len(unreg))
        for r in unreg:
            out.append("    %s  refs=0（引用面里零命中）" % pad(r["name"], 26))
    elif zero:
        out.append("  ✓ 零引用件全部已登记（%d/%d）" % (len(reg), len(zero)))
    if stale:
        out.append("  ⚠ 白名单可下架（已有引用 ⇒ 不再是死件；棘轮只许减）：%s"
                   % " · ".join("%s(refs=%d)" % (r["name"], r["total"]) for r in stale))
    if doc_only:
        out.append("  ⚠ 仅文档点名（total>0 但引用面里没有代码侧调用 ⇒ 不是死件、也不是活件；供人工判）："
                   "共 %d 只" % len(doc_only))
        for r in doc_only[:12]:
            out.append("    %s  refs=%d（文档 %d / 代码侧 %d）例：%s"
                       % (pad(r["name"], 26), r["total"], r["doc"], r["code"],
                          " · ".join(r["samples"][:3])))
        if len(doc_only) > 12:
            out.append("    …另有 %d 只同为「仅文档点名」（口径相同，为控篇幅不逐条打印）" % (len(doc_only) - 12))

    # 排除域（现读；项目口径：先出「哪些域没有任何门看着」再谈新门）
    out.append("")
    out.append("【排除域】不在本门判据里（现读，非记忆）")
    sub = sorted({p.split("/")[1] for p in files if p.startswith("scripts/") and p.count("/") > 1})
    for d in sub:
        n = len([p for p in files if p.startswith("scripts/%s/" % d)])
        out.append("  · scripts/ 子目录：scripts/%s/（%d 件，顶层 glob 口径不覆盖）" % (d, n))
    if os.path.isdir(os.path.join(root, ".git")):
        o = git_out(root, ["ls-files", "-o", "--exclude-standard", "-z", "--", "scripts"])
        untracked = [p for p in (o or "").split("\0") if p]
        if untracked:
            out.append("  · 未跟踪件 %d 件（未入库 ⇒ 不在判据内；入库后首跑会看到它）：%s"
                       % (len(untracked), " · ".join(trunc(p, 40) for p in untracked[:6])))
    for p, why in NOT_GATES:
        if os.path.isfile(os.path.join(root, p)):
            out.append("  · 相关但不是闸：%s（%s）" % (p, why))
    out.append("  · 候选面 = scripts/ **顶层**（.py/.sh/无后缀 shebang）；更深的树不在判据内")

    if errs:
        out.append("")
        out.append("【配置错】%d 条（⇒ rc=2，不给结论）" % len(errs))
        for e in errs:
            out.append("  ✗ %s" % e)

    out.append("")
    hit_n = len(hits_a) + len(unreg)
    out.append("【口径】引用 = 脚本名在引用面文件里出现；引用面 = 已跟踪文本文件 − 自指 2 件 − 记录面"
               "（现行债务台账，glob %s）− 候选自身。" % " · ".join(RECORD_SURFACES))
    out.append("        「记录不能充作引用」：台账点名不让死件复活（否则写下这条债就把债抹掉了）。")
    out.append("        「在闸里」= 按名调用形态（纯注释行 / 纯清单项不算）——这是有牙齿的那一条。")
    out.append("")
    out.append("【结论】A 命中 %d · B 未登记 %d · 配置错 %d" % (len(hits_a), len(unreg), len(errs)))
    if errs:
        out.append("        rc=2（缺件/配置错/空转 ⇒ 不给结论，不许静默）")
    elif opts.strict_report and hit_n:
        out.append("        --strict-report 且命中 %d ⇒ rc=1" % hit_n)
    elif opts.strict_report:
        out.append("        阻断档（`--strict-report`）且**零命中** ⇒ rc=0"
                   "（挂闸档：此后新增一只未挂/未登记即 rc=1）")
    else:
        out.append("        只报告档 ⇒ rc=0（命中不阻断；挂闸时用 --strict-report 或按 .md §五 走棘轮）")
    out.append("升阻断：**已执行**（2026-09-18 ③批次 · 提交闸 `gates` scope · `tri` + `--strict-report`）"
               "—— 存量 A 命中 8→0 后升档，此后新增一只未挂/未登记即红；全过程见 "
               "scripts/check-wired-scripts.md §五")
    return "\n".join(out)

# ─────────────────────────────────────────────────────────────────────────────
# --list-rules
# ─────────────────────────────────────────────────────────────────────────────
def list_rules():
    L = []
    L.append("门③ check-wired-scripts — 判据 / 闸清单 / 口径（--list-rules）")
    L.append("")
    L.append("断言 A（门脚本必须上岗）：凡门脚本 —— ① 名以 `check-` 开头（scripts/ 顶层 · .py/.sh/无后缀 shebang）"
             "或 ② 说明书自称门（同干 .md 头部 %d 行内出现「门」）—— 必须出现在至少一个闸里。" % DOC_HEAD_LINES)
    L.append("  「在闸里」判据 = 按名调用形态；排除：纯注释行（`#` 开头）· 纯清单项（整行就是 \"scripts/<名>\"）。")
    L.append("断言 B（零引用件必须登记）：scripts/ 顶层零引用（引用面里出现 0 次）⇒ 必须登记白名单"
             "（每项 name + reason + date；缺一即 rc=2；点名的脚本已不存在即 rc=2 悬空）。")
    L.append("")
    L.append("闸清单（判据只吃这份表；实际文件名与调用方式 = 2026-09-18 现读）：")
    for g, kind, label, how in GATES:
        L.append("  [%s] %-36s %s" % ("提交" if kind == "commit" else "发布", g, label))
        L.append("       调用方式：%s" % how)
    L.append("相关但不是闸（不进判据）：")
    for p, why in NOT_GATES:
        L.append("  · %-36s %s" % (p, why))
    L.append("")
    L.append("扫描面与排除域：")
    L.append("  · 候选 = scripts/ 顶层（glob `scripts/*`，**不递归**；子目录如 scripts/253/ 打印进排除域）")
    L.append("  · 引用面 = git 索引里的已跟踪文件（`git ls-files`）；未跟踪件不在判据内")
    L.append("  · 排除 ① 本门自指 %s（否则我自己的名单会把零引用写成有引用）" % " · ".join(SELF_FILES))
    L.append("  · 排除 ② 记录面 glob：%s —— 「记录不能充作引用」" % " · ".join(RECORD_SURFACES))
    L.append("  · 排除 ③ 候选脚本自身")
    L.append("")
    L.append("白名单（%d 条 · 只许减不许增；有引用后提示可下架）：" % len(WHITELIST))
    for e in WHITELIST:
        L.append("  · %-24s date=%s  reason=%s" % (e.get("name"), e.get("date"),
                                                   trunc(e.get("reason", ""), 80)))
    L.append("")
    L.append("退码：0 = 只报告档正常出口（命中不阻断）· 1 = 仅 --strict-report 且有命中 ·"
             " 2 = 缺件/配置错/空转/自检不过（不给结论）")
    L.append("升级路径（.md §五 · **已执行 2026-09-18 ③批次**）：清存量债 → 记基线快照（逐文件计数）→ "
             "--strict-report 挂进闸（新增一条即红）→ 只许减不许增，每还掉 N 条下调一格 → "
             "降到 0 转完全阻断、基线表退役。")
    L.append("  现状：8 只未挂门脚本逐只上岗 ⇒ A 命中 8→0 · B 未登记 0 ⇒ 已挂 `tri` + `--strict-report`（阻断档）。")
    L.append("  ★ 与第 2 步的差异（如实登记）：**基线快照表未建** —— 存量是**当批清到 0** 而不是逐格下调，")
    L.append("    正落在 §五 第 5 步「A 命中 0 且 B 未登记 0 ⇒ 转完全阻断、撤掉基线表」那一格，故不需要棘轮表。")
    L.append("  ★ 阻断面由**闸的步骤**给（模式 `tri` + 开关 `--strict-report`）；本脚本的默认档位**未改**")
    L.append("    （默认仍只报告 ⇒ 手工跑一下不会突然拦人），谁摘掉那个开关就会退化成恒绿步（自检 ⑩ 已钉住）。")
    return "\n".join(L)

# ─────────────────────────────────────────────────────────────────────────────
# 自检（成对负控）
# ─────────────────────────────────────────────────────────────────────────────
def _fixture(base, name, files):
    d = os.path.join(base, name)
    for relp, content in files.items():
        p = os.path.join(d, relp)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(content)
    return d

GATE_A = "#!/usr/bin/env bash\nset -u\n# 闸（夹具）\npython3 scripts/check-alpha.py --scope x\n"
GATE_B = "#!/usr/bin/env bash\npython3 scripts/check-beta.py\n"

def run_case(label, fixture, wl, extra, expect_rc, must, must_not, results):
    cmd = [sys.executable, os.path.abspath(__file__), "--root", fixture, "--no-self-test"]
    if wl is not None:
        # 白名单 JSON 写在**夹具之外**（父目录）：写在夹具里会被算成一条引用，
        # 把「零引用」的期望值搅乱（踩过一次）
        p = os.path.join(os.path.dirname(fixture), "_wl_%s.json" % os.path.basename(fixture))
        with open(p, "w", encoding="utf-8") as f:
            json.dump(wl, f, ensure_ascii=False)
        cmd += ["--whitelist", p]
    cmd += extra
    r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    out = r.stdout.decode("utf-8", "replace")
    ok = (r.returncode == expect_rc)
    bad = []
    if not ok:
        bad.append("rc=%d（期望 %d）" % (r.returncode, expect_rc))
    for m in must:
        if m not in out:
            bad.append("缺 %r" % m)
    for m in must_not:
        if m in out:
            bad.append("不该出现 %r" % m)
    results.append((label, not bad, "; ".join(bad)))
    return out

def selftest():
    base = tempfile.mkdtemp(prefix="wired-scripts-selftest-")
    results = []
    wl_ok = [{"name": "helper.py", "reason": "夹具：一次性", "date": "2026-09-18"}]

    ok_files = {
        "scripts/precommit-gates.sh": GATE_A,
        "scripts/publish-preflight.sh": GATE_B,
        "scripts/check-alpha.py": "# ck\n",
        "scripts/check-beta.py": "# ck\n",
        "scripts/helper.py": "# zero-ref but whitelisted\n",
        # 让两只闸自身有「代码侧引用」（否则它们自己是零引用件，会污染 B 的期望值）
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/precommit-gates.sh\n"
                                "bash scripts/publish-preflight.sh /tmp/out\n",
        "Makefile": "check:\n\tbash scripts/run-gates.sh\n",
        # 记录面（本门结论的落账面）：点名 helper.py **不算引用** ⇒ helper.py 仍应是零引用件
        "docs/项目文档/v9.9/债务台账-20990101.md": "helper.py 是零引用\n",
    }
    f_ok = _fixture(base, "ok", ok_files)

    # ① 正控：全挂 + 白名单齐 ⇒ 0 命中 ⇒ rc=0
    run_case("① 正控：全挂且白名单齐 ⇒ rc=0 且 0 命中", f_ok, wl_ok, [],
             0, ["A 命中 0", "未登记 0"], [], results)

    # ② 正控（判据区分度）：记录面点名不算引用 ⇒ helper.py 仍是零引用（靠白名单不命中证明）
    out = run_case("② 记录面不计引用（判据区分度）⇒ 零引用 1 只且已登记", f_ok, wl_ok, [],
                   0, ["零引用 1 只", "已登记（1 只）"], ["零引用 0 只"], results)

    # ③ 负控 A：拿掉一条接线 ⇒ 必须报出来（只报告档仍 rc=0）
    f_noa = _fixture(base, "neg-a", dict(ok_files, **{
        "scripts/precommit-gates.sh": "#!/usr/bin/env bash\nset -u\necho 没有按名调用\n"}))
    run_case("③ 负控 A：拿掉 check-alpha 的接线 ⇒ 报出它，只报告档 rc=0",
             f_noa, wl_ok, [], 0, ["check-alpha.py", "A 命中 1"], [], results)

    # ④ 负控 A':同一夹具 + --strict-report ⇒ rc=1
    run_case("④ 负控 A'：同一夹具 + --strict-report ⇒ rc=1",
             f_noa, wl_ok, ["--strict-report"], 1, ["rc=1"], [], results)

    # ⑤ 负控 A''：注释/清单项不算上岗（有牙齿的那一条）
    f_cmt = _fixture(base, "neg-a-cmt", dict(ok_files, **{
        "scripts/check-gamma.py": "# ck\n",
        "scripts/precommit-gates.sh": GATE_A + "# python3 scripts/check-gamma.py\n"
            "EXCLUDES=(\n  \"scripts/check-gamma.py\"\n)\n"}))
    run_case("⑤ 负控 A''：只在注释行/纯清单项里出现 ⇒ 仍算未挂（判据有牙齿）",
             f_cmt, wl_ok, [], 0, ["check-gamma.py", "A 命中 1"], [], results)

    # ⑥ 负控 B：零引用件不在白名单（且无引用）⇒ 必须报出来
    run_case("⑥ 负控 B：零引用且未登记 ⇒ 报出它（只报告档 rc=0）",
             f_ok, [], [], 0, ["未登记 1", "helper.py"], [], results)

    # ⑦ 负控 B'：同上 + --strict-report ⇒ rc=1
    run_case("⑦ 负控 B'：同夹具 + --strict-report ⇒ rc=1",
             f_ok, [], ["--strict-report"], 1, ["rc=1"], [], results)

    # ⑧ 配置错：白名单缺 reason / 缺 date / date 形态不对 / 悬空 ⇒ 一律 rc=2
    run_case("⑧a 白名单缺 reason ⇒ rc=2", f_ok,
             [{"name": "helper.py", "date": "2026-09-18"}], [], 2, ["缺 reason"], [], results)
    run_case("⑧b 白名单缺 date ⇒ rc=2", f_ok,
             [{"name": "helper.py", "reason": "夹具"}], [], 2, ["缺 date"], [], results)
    run_case("⑧c 白名单 date 形态不对 ⇒ rc=2", f_ok,
             [{"name": "helper.py", "reason": "夹具", "date": "09/18/2026"}], [], 2,
             ["date 形态不对"], [], results)
    run_case("⑧d 白名单悬空（点名的脚本不存在）⇒ rc=2", f_ok,
             wl_ok + [{"name": "ghost.py", "reason": "夹具", "date": "2026-09-18"}], [], 2,
             ["悬空"], [], results)

    # ⑨ 空转：没有候选 ⇒ rc=2（不许静默给绿）
    f_empty = _fixture(base, "empty", {"scripts/precommit-gates.sh": GATE_A})
    run_case("⑨a 空转：候选 0 只 ⇒ rc=2", f_empty, [], [], 2, ["空转", "rc=2"], [], results)

    # ⑩ 缺件：一个闸都不在位 ⇒ rc=2（判据无依据）
    f_nogate = _fixture(base, "no-gate", {
        "scripts/check-solo.py": "# ck\n",
        "README.md": "x\n",
    })
    run_case("⑩ 缺件：闸清单一个都不在位 ⇒ rc=2（不给结论）", f_nogate, [], [], 2,
             ["不给结论", "rc=2"], [], results)

    # ⑪ 说明书自称门：同干 .md 头部含「门」⇒ 该脚本成为候选
    f_decl = _fixture(base, "decl", dict(ok_files, **{
        "scripts/tool-x.sh": "#!/usr/bin/env bash\necho x\n",
        "scripts/tool-x.md": "# tool-x — 本门说明书\n\n写盘门。\n",
    }))
    run_case("⑪ 说明书自称门（同干 .md 头部含「门」）⇒ 成为候选并报未挂",
             f_decl, wl_ok, [], 0, ["tool-x.sh", "说明书自称门"], [], results)

    # ⑫ 说明书不提门 ⇒ 不成为候选（区分度）；给它一条代码侧引用免得它落进 B 清单
    f_nodecl = _fixture(base, "nodecl", dict(ok_files, **{
        "scripts/tool-y.sh": "#!/usr/bin/env bash\necho y\n",
        "scripts/tool-y.md": "# tool-y —— 一个普通工具\n\n没什么特别的。\n",
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/tool-y.sh\n"
                                "bash scripts/precommit-gates.sh\n"
                                "bash scripts/publish-preflight.sh /tmp/out\n",
    }))
    run_case("⑫ 说明书不提「门」⇒ 不成为候选（区分度）",
             f_nodecl, wl_ok, [], 0, ["A 命中 0"], ["tool-y.sh"], results)

    # ⑬ --list-rules 可用且含闸清单/升级路径
    r = subprocess.run([sys.executable, os.path.abspath(__file__), "--list-rules"],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    o = r.stdout.decode("utf-8", "replace")
    bad = []
    if r.returncode != 0:
        bad.append("rc=%d" % r.returncode)
    for m in ["断言 A", "断言 B", "precommit-gates.sh", "publish-preflight.sh", "升级路径", "白名单"]:
        if m not in o:
            bad.append("缺 %r" % m)
    results.append(("⑬ --list-rules 可打印判据/闸清单/升级路径", not bad, "; ".join(bad)))

    npass = len([1 for _, ok, _ in results if ok])
    print("门③ --self-test（成对负控 · 真命令行 + 真退出码）：%d/%d 通过" % (npass, len(results)))
    for label, ok, why in results:
        print("  %s %s%s" % ("✓" if ok else "✗", label, "" if ok else "   ← %s" % why))
    print("  夹具目录：%s" % base)
    if npass != len(results):
        print("  ✗ 自检不过 ⇒ 拒绝扫真目标（rc=2）")
        return 2
    return 0

# ─────────────────────────────────────────────────────────────────────────────
# main
# ─────────────────────────────────────────────────────────────────────────────
def main(argv=None):
    ap = argparse.ArgumentParser(
        description="门③ check-wired-scripts — 脚本接线自检（只报告档）",
        formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--strict-report", action="store_true",
                    help="有命中即 rc=1（默认：只报告 ⇒ rc=0）")
    ap.add_argument("--list-rules", action="store_true", help="打判据/闸清单/口径/升级路径，然后退出")
    ap.add_argument("--self-test", dest="self_test", action="store_true", help="只跑自检")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（自检内部的子进程用，防递归）")
    ap.add_argument("--root", default=None, help="仓根（默认 = 本脚本上一级；自检夹具用）")
    ap.add_argument("--whitelist", default=None, help="白名单 JSON 覆盖（自检/临时用；默认用内置表）")
    opts = ap.parse_args(argv)

    if opts.list_rules:
        print(list_rules())
        return 0
    if opts.self_test:
        return selftest()

    root = os.path.abspath(opts.root or os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    if not os.path.isdir(os.path.join(root, "scripts")):
        print("FATAL: 仓根缺 scripts/ 目录（root=%s）⇒ rc=2（不给结论）" % root)
        return 2

    if not opts.no_self_test:
        rc_st = selftest()
        print("")
        if rc_st != 0:
            return 2

    files, mode = list_files(root)
    if not files:
        print("FATAL: 引用面 0 个文件（空转）⇒ rc=2")
        return 2
    cands, _notes = build_candidates(root, files)
    if not cands:
        print("FATAL: scripts/ 顶层脚本候选 0 只（空转 ⇒ 不给结论）⇒ rc=2")
        return 2
    a_cands = [c for c in cands if c["kind"]]
    b_cands = cands
    if not a_cands:
        print("FATAL: 门脚本候选 0 只（断言 A 无对象 ⇒ 空转，不给结论）⇒ rc=2")
        return 2

    wl = WHITELIST
    if opts.whitelist:
        try:
            with open(opts.whitelist, encoding="utf-8") as f:
                wl = json.load(f)
            if not isinstance(wl, list):
                print("FATAL: --whitelist 必须是 JSON 数组 ⇒ rc=2")
                return 2
        except Exception as e:
            print("FATAL: 读 --whitelist 失败：%s ⇒ rc=2" % e)
            return 2

    hits_a, wired_a, n_gate_present, missing_gates = judge_a(root, a_cands)
    if n_gate_present == 0:
        print("FATAL: 闸清单 %d 个**一个都不在位**（判据无依据 ⇒ 不给结论）⇒ rc=2" % len(GATES))
        for g in missing_gates:
            print("  缺：%s" % g)
        return 2

    errs, wl_names = validate_whitelist(root, {c["name"] for c in b_cands}, wl)
    zero, unreg, reg, stale, doc_only, rows = judge_b(root, files, b_cands, wl_names)

    print(report(root, opts, files, mode, a_cands, b_cands, hits_a, wired_a, n_gate_present,
                 missing_gates, zero, unreg, reg, stale, doc_only, rows, wl, errs))
    if errs:
        return 2
    if opts.strict_report and (hits_a or unreg):
        return 1
    return 0

if __name__ == "__main__":
    sys.exit(main())

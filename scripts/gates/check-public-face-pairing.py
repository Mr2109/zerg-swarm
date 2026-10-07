#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-face-pairing.py —— 公开面**成对闸**（`T-12` · 漏点 `E-1` · 缺口 `Q-128`）

真源（**逐字** · 不转述）
------------------------
`Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md:45`（`T-12`）逐字：
  「| **`T-12`** | **成对闸（机器化）**：形态照 nodejs.org `.github/workflows/translations-pr-lint.yml`
   （`paths:` **负向 glob** + 机器人分支豁免）**【已复核】** | 高 | `C` | **负控**：只改 zh 不改 en ⇒ 闸**红**；
   **正控**：不碰任何公开面件 ⇒ **rc=0**；★ 上闸前必须先有 `T-11b` 的例外登记（否则 `design/` 两件当场假红 ✗）
   | `E-1` · §〇 修正② | 未开始 |」

同件 `:119`（依赖表）逐字：「`T-12` 成对闸 | 档 `C` 第 1 条 | **被** `T-11b` **挡**；不挡别的 |」
同件 `:120`（依赖表）逐字：「`T-11b` 挡 `T-12`（例外不先登记 ⇒ 成对闸上闸即假红 ✗）」
`Zerg-内部文档/项目文档/v2.5.12/设计-公开文档面随版收口-v1.1-20260923.md:384`（`E-1`）逐字：
  「| **`E-1`** | 缺**成对闸可机器化**的现成形态：`paths:` **负向 glob** + **机器人分支豁免** | 「双语成对」若只写进散文
   ⇒ 会腐化；nodejs 这套证明「**一 PR 一语言** + 负向 glob 圈定扫描面」是可机检的 | nodejs.org
   `.github/workflows/translations-pr-lint.yml`：`name: Incoming Translation Checks` + `paths:` 负向 glob
   （`!apps/site/pages/en/**`）+ 机器人分支豁免 **【已复核】** | **高** | §〇 修正② + §3.6（成对闸） |」
同件 `:21`（修正② ④）逐字：「④ **一 PR 一语言**（一次改动只碰一种语言，成对提交）」
同件 `:410`（实施顺序第 4 步）逐字：「**成对闸与漂移锚** … 落点在**成对件的判据**（六对 + 两件单语的例外要显式登记）」

参照件（`E-1` 的真源 · 本件**本轮现读复核** 2026-09-24 · `curl` 取回 **58 行**）
------------------------------------------------------------------------------
  · `name: Incoming Translation Checks`
  · `on.pull_request_target.paths` 逐字（**正向 + 负向成对列出**）：
      `- 'apps/site/pages/**/*.md'` · `- 'apps/site/pages/**/*.mdx'` ·
      `- '!apps/site/pages/en/**/*.md'` · `- '!apps/site/pages/en/**/*.mdx'` ·
      `- 'packages/i18n/src/locales/*.json'` · `- '!packages/i18n/src/locales/en.json'` ·
      `- 'apps/site/snippets/**/*.bash'` · `- '!apps/site/snippets/en/**/*.bash'`
  · 机器人分支豁免（job 上的 `if:`）逐字：
      `(github.event.pull_request.head.repo.full_name != 'nodejs/nodejs.org') ||`
      `(github.event.pull_request.head.repo.full_name == 'nodejs/nodejs.org' &&`
      ` github.event.pull_request.head.ref != 'chore/crowdin')`
      ⇒ 语义 = **机器人直推分支（Crowdin 的 `chore/crowdin`）不判**。

口径差（**照实记** · 三处 · 不藏）
---------------------------------
  ① 参照件的负向 glob 剔的是**源语言**（`!apps/site/pages/en/**`）—— 它只判「**译侧** PR」；
     本仓要判的是**成对**（两侧都要动）⇒ **同一形态、不同向**：本仓的负向 glob 用来
     「**圈定扫描面 + 剔掉已登记的单一语例外**」（`design/` 2 件）。方向差在这里，逐字写清。
  ② 参照件是 GH Actions（`pull_request_target` · PR 面）；本仓是**私有仓 + 提交面 / 本版窗口**
     ⇒ 落地形态 = 本件（判**改动面**）+ 闸⑧ 的 ⑧d 按名调用（`--rev` 指窗口）。**不新造 PR 面** ✗。
  ③ 参照件的豁免读 `github.event.pull_request.head.ref`；本仓无 PR 面 ⇒ 读 `git symbolic-ref --short -q HEAD`
     （**分支面**）与逐笔作者（**笔面**）。两处都**逐条打印**、不许静默豁免 ✗。

单一真源（**一律复用 · 绝不另抄一份**）
------------------------------------
  · **例外登记面** = `T-11b` 正式登记的 `scripts/gates/check-public-face-anchor.py`
    （`T-13` 那件 · `SINGLE_LANG_EXCEPTIONS` · 逐条带理由 · 登记时 2026-09-24）⇒ 本件 **importlib 现读它**。
    读不到 / 空 ⇒ **rc=2**（缺件 ≠ 绿）。★ 登记面缺席 ⇒ `design/` 两件当场假红 —— 这正是 `T-12` 逐字
    那条前置「★ 上闸前必须先有 `T-11b` 的例外登记（否则 `design/` 两件当场假红 ✗）」的机器化落点。
  · **成对映射** = `X.zh-CN.md` ↔ `X.en.md`；`README.zh-CN.md` ↔ `README.md`（镜像对）—— 与闸⑧b ③ /
    `T-13` 的 `J0` **同一形状**（三处形状一致，不是三份解析器：本件只做**改动面**那一半）。

判据（逐条 · 机器可判）
----------------------
  `P0` **扫描面（`paths:` 形态）**：正向 = `<面>/**/*.md`；**负向 glob 逐条现读自登记面**
       （每条登记件 → 其所在子树的 `!<面>/<子目录>/**`，逐条打印）。
  `P1` **机器人面豁免**（`T-12` 形态第二条）：① **分支面** —— 当前分支命中机器人直推分支表 ⇒ 整跑豁免
       （打印分支名 + 理由）；② **笔面** —— `--rev A..B` 时逐笔读作者，命中机器人形态（`[bot]` / `-bot@`）
       ⇒ **那笔的改动被剔除**、逐笔打印「机器人笔 ⇒ 不判」（照参照件 `if:` 那一格的同一条语义）。
  `P2` **负控（本条）**：改动面里「**成对件只动一侧**」⇒ **rc=1**，逐对点名（逐字分两向：
       「只改 zh 不改 en」/「只改 en 不改 zh」）；改动面里「**无对应语言件、又未登记例外**」的件 ⇒ 同判 **rc=1**
       （形态 = 「只改 zh 不改 en」的**新增**那一支）。
  `P3` **正控（本条）**：改动面里 **0 件**公开面件（不碰任何公开面件）⇒ **rc=0** —— 逐字打印计数，**不静默**。
  `P4` **只管 tracked 的改动面**（照 `T-13` 的 `J4`）：**untracked** 件逐条打印「不在改动面 ⇒ 不判」
       （本仓多写手并行 ⇒ 别把兄弟写手在途的未跟踪件判成红 ✗）。
  `P5` **静态成对性不在本件**（**只打印、不判红**）：扫面里既不成对、又不在改动面的件 ⇒ 逐条打印
       「静态成对性由 `T-13` 的 `J0` / 闸⑧b ③ 判，本件不重复判红」（**不抢别人的面**）。
  `P6` **空转守卫**：面不在 / 扫面 0 件 / 成对 0 对 / 登记面读不到 / `git` 读不到 / `rev` 解不出
       / `--self-test` 不过 ⇒ **rc=2（不给结论）**。

退码（三档 · 与仓内其余门脚本同口径）
------------------------------------
  0 = 全绿：改动面里没有单边改动（同批或**不碰公开面件**）· 或整跑被机器人面豁免。
  1 = **有成对红**：至少一对「只动一侧」，或改动面里出现未登记的单语件 ⇒ 逐条点名。
      （处置只有两条：补上另一侧，或撤掉单边改动；**不许**放宽判据来凑绿 ✗）
  2 = **不给结论**：见 `P6`；另加 **拒收的逃生开关**（见下）。

用法
----
    python3 scripts/gates/check-public-face-pairing.py                       # 先自检，再判 HEAD → 工作树
    python3 scripts/gates/check-public-face-pairing.py --no-self-test        # 跳过自检（闸里按名调用用这个）
    python3 scripts/gates/check-public-face-pairing.py --rev HEAD~3          # 判「HEAD~3 → 工作树」这一段
    python3 scripts/gates/check-public-face-pairing.py --rev A..B            # 判一笔提交区间（逐笔按作者归属）
    python3 scripts/gates/check-public-face-pairing.py --root <树> --face publish/docs
    python3 scripts/gates/check-public-face-pairing.py --list-rules
    python3 scripts/gates/check-public-face-pairing.py --json
    python3 scripts/gates/check-public-face-pairing.py --self-test           # 合成 git 夹具 · 不碰真目标

拒收的逃生开关（一律 **rc=2** · 照同族先例 `check-keep-marks.py` 拒 `--block` 一类旗标）
  `--waive` / `--allow-unpaired` / `--skip-pairs` / `--ignore-missing` —— 例外只能**显式登记**
  （登记面在 `T-11b` 那件里），不许用开关把判据**绕过去** ✗。

★ **不接管道取 rc**（本仓门禁硬规矩：管道后 `$?` 是末端 rc）—— 本件自己不做任何 shell 调用，
  只经 `subprocess` 直接起 `git` 并读它的 `returncode`。
★ 自检自带工作区隔离（`GIT_CONFIG_GLOBAL` / `GIT_CONFIG_SYSTEM` 指 `/dev/null` + `HOME` 指夹具 +
  合成 git 仓）⇒ 不碰真目标、不碰真仓的 `git` 配置。

接线（闸⑧ 的 ⑧d · 2026-09-24）
-----------------------------
本件被 `scripts/build/publish-preflight.sh` 的 **闸⑧d** 按名调用（与 ⑧a 门㉝ / ⑧c `T-14` 同形态：
**不复制任何判据实现**），判据树同走 `${ZERG_PUBFACE_ROOT}` 旋钮；窗口 = 「当前 `const Version` 所在笔 → 工作树」。
"""

import argparse
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile

VERSION = "check-public-face-pairing.py v1.0（`T-12` 成对闸 · 2026-09-24 落件 · 接线 = 闸⑧ ⑧d）"

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))

FACE_DEFAULT = "publish/docs"                   # 扫描面（仓根相对 · 公开文档面）
ZH_SUFFIX = ".zh-CN.md"                         # 真源侧（zh-CN）
EN_SUFFIX = ".en.md"                            # 派生侧（en）
README_ZH, README_EN = "README.zh-CN.md", "README.md"
WALK_PRUNE = {".git", "node_modules", "target"}
REGISTRATION_REL = "scripts/gates/check-public-face-anchor.py"   # `T-11b` 登记面（单一真源）

# 退役态（B轨 · 2026-10-07）：默认登记面件（`check-public-face-anchor.py`）退役后的**内化真源**
#   （与其 Go 半 `cpfaExceptions` 逐条同源 · file/reason 逐条同）。py 件在盘时仍现读其
#   `SINGLE_LANG_EXCEPTIONS`（单一真源不变）；退役（不在盘 · 其 Go 半在盘）时回落到本表。
RETIRED_EXCEPTIONS = [
    {"file": "design/格式反馈协议-FFP.md",
     "reason": "中文单语「理念件」（`Q-128` ②③ · `T-11b` **正式登记**）：公开面映射 `A13` **只有 zh 一条**（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；★ 「要不要补 en」= `Q-128` ④ 开放问题 · **本表不定**"},
    {"file": "design/记忆体系.md",
     "reason": "中文单语「理念件」（同 `Q-128` · `T-11b` **正式登记**）：公开面映射 `A14` **只有 zh 一条**（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；★ 「要不要补 en」= 同上开放问题 · **本表不定**"},
]

REFUSE_FLAGS = ("--waive", "--allow-unpaired", "--skip-pairs", "--ignore-missing")

# 机器人直推分支表（`T-12` 形态第二条 · 逐条带理由）。★ 本仓现读**没有** `chore/crowdin` 这条分支 ——
#   列出它是为了「形态与参照件一致 + 有一格可自证」；命中即整跑豁免，且**豁免必须打印**（不许静默）。
BOT_BRANCH_PREFIXES = (
    ("chore/crowdin", "参照件的机器人直推分支（nodejs.org 的 Crowdin 机器人）逐字同名 —— 本仓现读无此分支；留作形态一致 + 自证有一格"),
    ("bot/", "通用机器人分支前缀"),
    ("dependabot/", "GitHub 依赖机器人分支"),
    ("renovate/", "Renovate 机器人分支"),
)
BOT_AUTHOR_RE = re.compile(r"\[bot\]|-bot@|\+bot@")

EXIT_OK, EXIT_RED, EXIT_BLOCKED = 0, 1, 2


class Refused(Exception):
    """拒绝给结论（rc=2）。"""


# ── git（只经 subprocess · 不接管道）────────────────────────────────────────
def git_env(base=None):
    env = dict(os.environ)
    if base is not None:
        env["GIT_CONFIG_GLOBAL"] = os.devnull
        env["GIT_CONFIG_SYSTEM"] = os.devnull
        env["HOME"] = base
        env["GIT_AUTHOR_NAME"] = env["GIT_COMMITTER_NAME"] = "PairingSelfTest"
        env["GIT_AUTHOR_EMAIL"] = env["GIT_COMMITTER_EMAIL"] = "selftest@example.invalid"
    return env


def git_run(root, args, env=None):
    try:
        p = subprocess.run(["git", "-C", root, "-c", "core.quotepath=false"] + list(args),
                           capture_output=True, text=True, env=env)
        return p.returncode, p.stdout
    except OSError:
        return 127, ""


def git_ok(root, env=None):
    rc, _ = git_run(root, ["rev-parse", "--git-dir"], env=env)
    return rc == 0


def rev_resolvable(root, rev, env=None):
    if not rev:
        return True
    parts = rev.split("..") if ".." in rev else [rev]
    for p in parts:
        rc, _ = git_run(root, ["rev-parse", "--quiet", "--verify", "%s^{commit}" % (p or "HEAD")], env=env)
        if rc != 0:
            return False
    return True


def split_names(text):
    out = []
    for ln in text.split("\n"):
        ln = ln.strip()
        if ln:
            out.append(ln)
    return out


# ── 扫描面（`paths:` 形态 · 正向 glob + 负向 glob）──────────────────────────
def enumerate_face(face_dir):
    """扫面逐件（相对面的路径 · 排序）。只收 .md；点件 / 点目录不入。"""
    files = []
    for dp, dns, fns in os.walk(face_dir):
        dns[:] = sorted(d for d in dns if not d.startswith(".") and d not in WALK_PRUNE)
        for fn in sorted(fns):
            if fn.startswith(".") or not fn.endswith(".md"):
                continue
            rel = os.path.relpath(os.path.join(dp, fn), face_dir).replace(os.sep, "/")
            files.append(rel)
    return sorted(files)


def load_registered_exceptions(registration_path=REGISTRATION_REL, root=None):
    """现读 `T-11b` 的登记面（`SINGLE_LANG_EXCEPTIONS`）—— **单一真源**，不另抄一份。

    读不到 / 形状不对 / 空 ⇒ 抛 `Refused`（缺件 ≠ 绿）。`root` 非空时登记面按**判据树**解相对路径
    （自检夹具用 · 真跑一律 = 本仓根）。"""
    base = os.path.abspath(root) if root else REPO_ROOT
    path = registration_path if os.path.isabs(registration_path) else os.path.join(base, registration_path)
    if not os.path.isfile(path):
        # 退役态（B轨 · 2026-10-07）：登记面件退役（缺席 · 其 Go 半在盘）⇒ 回落到**内化登记**（✗ 不 fail-closed）。
        # 缺件且无 Go 半（真消失）⇒ 照旧 rc=2（缺件 ≠ 绿）。
        go_half = os.path.join(base, "core", "cmd", "zerg",
                               os.path.basename(path)[:-3].replace("-", "_") + ".go") \
            if registration_path.endswith(".py") else ""
        if go_half and os.path.isfile(go_half):
            return [dict(r) for r in RETIRED_EXCEPTIONS]
        raise Refused("缺件：例外登记面 %s 不在（`T-11b` 的登记面 · 单一真源）⇒ 判不了「哪几件是已登记的单语例外」"
                      "（缺件 ≠ 绿；登记面缺席 ⇒ `design/` 两件会当场假红 ✗）" % registration_path)
    try:
        spec = importlib.util.spec_from_file_location("zerg_pairing_registration", path)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        rows = list(getattr(mod, "SINGLE_LANG_EXCEPTIONS"))
    except Exception as exc:                                        # noqa: BLE001 —— 读不全就不给结论
        raise Refused("读不了：例外登记面 %s 解析失败（%s）⇒ **不给结论（rc=2）**" % (registration_path, exc))
    out = []
    for row in rows:
        if not isinstance(row, dict) or not row.get("file"):
            raise Refused("形状不对：例外登记面 %s 的条目缺 `file` 键（%r）⇒ 判不了" % (registration_path, row))
        out.append({"file": str(row["file"]).strip("/"), "reason": str(row.get("reason", ""))})
    if not out:
        raise Refused("空登记面：%s 的 `SINGLE_LANG_EXCEPTIONS` 一条都没有 ⇒ **不给结论**（空表 = 假覆盖）"
                      % registration_path)
    return out


def negative_globs(face, exceptions):
    """负向 glob 逐条（由登记件所在子树现读推出 · 逐条打印 · 同一子树的多件逐条点名）。"""
    order, globs = [], {}
    for row in exceptions:
        d = os.path.dirname(row["file"])
        g = "!%s/**/*.md" % (os.path.join(face, d) if d else face)
        if g not in globs:
            globs[g] = []
            order.append(g)
        globs[g].append(row["file"])
    return [(g, globs[g]) for g in order]


def pair_of(rel):
    """成对映射（与闸⑧b ③ / `T-13` 的 `J0` 同形状）。"""
    d, b = os.path.split(rel)
    pre = (d + "/") if d else ""
    if b == README_ZH:
        return pre + README_EN
    if b == README_EN:
        return pre + README_ZH
    if b.endswith(ZH_SUFFIX):
        return pre + b[: -len(ZH_SUFFIX)] + EN_SUFFIX
    if b.endswith(EN_SUFFIX):
        return pre + b[: -len(EN_SUFFIX)] + ZH_SUFFIX
    return None


# ── 改动面（只 tracked · untracked 逐条打印「不判」）───────────────────────
def current_branch(root, env=None):
    rc, out = git_run(root, ["symbolic-ref", "--short", "-q", "HEAD"], env=env)
    return out.strip() if rc == 0 else ""


def bot_branch_of(branch):
    for prefix, why in BOT_BRANCH_PREFIXES:
        if branch.startswith(prefix):
            return prefix, why
    return None


def changed_face(root, face, rev, env=None):
    """改动面 → (判据集合, 逐笔行, untracked 件)。逐笔归属只在 `A..B` 形态下做。"""
    notes, untracked = [], []
    rc, out = git_run(root, ["ls-files", "-o", "--exclude-standard", "--", face], env=env)
    if rc == 0:
        untracked = split_names(out)
    if rev and ".." in rev:
        a, b = rev.split("..", 1)
        rc, out = git_run(root, ["rev-list", "--reverse", "%s..%s" % (a, b or "HEAD")], env=env)
        if rc != 0:
            raise Refused("判不了：`rev-list %s` 读不出（区间解不出）⇒ **不给结论（rc=2）**" % rev)
        judged, bot_hits = set(), []
        for sha in split_names(out):
            rc_a, author = git_run(root, ["show", "-s", "--format=%an <%ae>", sha], env=env)
            rc_n, names = git_run(root, ["show", "--name-only", "--format=", sha], env=env)
            if rc_a != 0 or rc_n != 0:
                raise Refused("判不了：读不出提交 %s 的作者/件列表 ⇒ **不给结论（rc=2）**" % sha[:8])
            author = author.strip()
            if BOT_AUTHOR_RE.search(author):
                touched = split_names(names)
                if touched:
                    bot_hits.append("%s %s（%d 件）" % (sha[:8], author, len(touched)))
                continue
            judged.update(split_names(names))
        if bot_hits:
            notes.append("机器人笔豁免（照参照件 `if:` 那一格：机器人面不判）：%s" % " · ".join(bot_hits))
        # 工作树未提交改动（无笔归属 ⇒ 不适用机器人笔豁免 · 逐条打印这一口径）
        rc, out = git_run(root, ["diff", "--name-only", "HEAD"], env=env)
        if rc != 0:
            raise Refused("判不了：`git diff HEAD` 读不出 ⇒ **不给结论（rc=2）**")
        judged.update(split_names(out))
    else:
        base = rev or "HEAD"
        rc, out = git_run(root, ["diff", "--name-only", base], env=env)
        if rc != 0:
            raise Refused("判不了：`git diff --name-only %s` 读不出 ⇒ **不给结论（rc=2）**" % base)
        judged = set(split_names(out))
    return judged, notes, untracked


# ── 判据主体 ───────────────────────────────────────────────────────────────
def judge(root, face, rev=None, registration_path=REGISTRATION_REL, env=None, quiet=False):
    lines = []

    def say(s):
        lines.append(s)
        if not quiet:
            print(s)

    face_dir = os.path.join(root, face)
    if not os.path.isdir(face_dir):
        raise Refused("缺件：面 %s/ 不在 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）" % face)
    if not git_ok(root, env=env):
        raise Refused("判不了：%s 不是 git 仓（读不到改动面）⇒ **不给结论（rc=2）**" % root)
    if not rev_resolvable(root, rev, env=env):
        raise Refused("判不了：rev `%s` 解不出 ⇒ **不给结论（rc=2）**（拒绝猜）" % rev)

    files = enumerate_face(face_dir)
    if not files:
        raise Refused("空转：%s/ 下 0 件（「件没了所以全绿」是最难发现的那一类假绿）⇒ **不给结论（rc=2）**" % face)

    exceptions = load_registered_exceptions(registration_path, root=root)
    ex_files = set()
    for row in exceptions:
        if row["file"] not in files:
            say("   ⚠ 悬空登记：例外登记面的 `%s` 不在扫面里（照实打印 · 不改退码 · 与 `T-13` 的 `J0` 同处置）" % row["file"])
        ex_files.add(row["file"])

    # ── P0 扫描面（`paths:` 形态 · 逐条负向 glob）────────────────────────────
    say("   P0 扫描面（`paths:` 形态）：")
    say("      - '%s/**/*.md'" % face)
    for g, srcs in negative_globs(face, exceptions):
        say("      %s   ← 已登记的单语例外（%s）" % (g, " · ".join(srcs)))

    judged_face = [rel for rel in files if rel not in ex_files]
    pairs = {}
    unpaired = []
    for rel in judged_face:
        q = pair_of(rel)
        if q is None or q not in judged_face:
            unpaired.append(rel)
            continue
        zh, en = (rel, q) if rel.endswith(ZH_SUFFIX) or rel == README_ZH else (q, rel)
        pairs[zh] = en
    say("   扫面 %d 件（成对件 %d 件 = **%d 对** + 已登记单语例外 **%d** 件 + 无对应语言件 **%d** 件）"
        % (len(files), len(judged_face) - len(unpaired), len(pairs), len(ex_files), len(unpaired)))
    if not pairs:
        raise Refused("空转：成对件 **0 对**（判不出任何成对性）⇒ **不给结论（rc=2）**")

    # ── P1 机器人面豁免（分支面）────────────────────────────────────────────
    branch = current_branch(root, env=env)
    hit = bot_branch_of(branch) if branch else None
    if hit:
        say("   P1 机器人面豁免：当前分支 `%s` 命中 `%s`（%s）⇒ 整跑豁免（照参照件 `if:` 那一格）" % (branch, hit[0], hit[1]))
        say("⇒ rc=0（机器人面豁免 · **豁免已打印**、不静默）")
        return EXIT_OK, lines, None

    changed, notes, untracked = changed_face(root, face, rev, env=env)
    for n in notes:
        say("   P1 %s" % n)
    # 改动面归一化：以**面内相对路径**为准（成对映射 / 登记件 / 不成对件都是面内相对）
    ch_face, ch_out = [], []
    for c in sorted(changed):
        rel = c[len(face) + 1:] if c.startswith(face + "/") else None
        (ch_face if rel else ch_out).append(rel if rel else c)
    ch_set = set(ch_face)
    if untracked:
        for u in untracked:
            say("   P4 untracked ⇒ 不在改动面 ⇒ 不判：%s" % u)

    say("   改动面：共 %d 件（公开面内 **%d** 件 · 面外 %d 件不判）· rev=`%s`"
        % (len(changed), len(ch_face), len(ch_out), rev or "HEAD"))

    reds, exempt_hits = [], []
    for rel in ch_face:
        if rel not in judged_face:
            if rel in ex_files:
                exempt_hits.append(rel)
                say("   已登记的单语例外在改动面里 ⇒ **不判成对**（逐条打印）：%s" % rel)
            continue
    for zh, en in sorted(pairs.items()):
        in_zh, in_en = zh in ch_set, en in ch_set
        if in_zh and in_en:
            say("   同批 ✓ %s + %s" % (zh, en))
        elif in_zh:
            reds.append("只改 zh 不改 en：%s 在改动面里，而 %s **一字未动**" % (zh, en))
        elif in_en:
            reds.append("只改 en 不改 zh：%s 在改动面里，而 %s **一字未动**" % (en, zh))
    for rel in unpaired:
        if rel in ch_set:
            reds.append("单边出现：%s 在改动面里，却既无对应语言件、又未登记例外" % rel)
        else:
            say("   静态成对性（只打印 · 不判红）：%s 不成对 —— 该面由 `T-13` 的 `J0` / 闸⑧b ③ 判，本件不重复判红"
                % rel)

    # ── P2 负控 / P3 正控 ─────────────────────────────────────────────────
    if reds:
        for r in reds:
            say("   ✗ P2 负控命中：%s" % r)
        say("⇒ **有红（rc=1）**：%d 处单边改动（处置：补上另一侧，或撤掉这一次单边改动 —— **不许**放宽判据凑绿 ✗）"
            % len(reds))
        return EXIT_RED, lines, None
    if not ch_face:
        say("   P3 正控：改动面里 **0 件**公开面件（不碰任何公开面件）⇒ rc=0")
    else:
        say("   P3 正控：公开面内改动 %d 件 · 全部成对同批（或全数落在已登记例外里：%d 件）⇒ rc=0"
            % (len(ch_face), len(exempt_hits)))
    say("⇒ 全绿（rc=0）")
    return EXIT_OK, lines, None


def counts_of(root, face, rev=None, registration_path=REGISTRATION_REL, env=None):
    """`--json` 用的计数面（现跑重数一遍 · 不靠上面的打印）。"""
    out = {"face": face, "rev": rev or "HEAD"}
    face_dir = os.path.join(root, face)
    files = enumerate_face(face_dir) if os.path.isdir(face_dir) else []
    ex = load_registered_exceptions(registration_path, root=root)
    ex_files = {r["file"] for r in ex}
    judged = [f for f in files if f not in ex_files]
    pairs = [f for f in judged if pair_of(f) and pair_of(f) in judged]
    out.update({"files": len(files), "exceptions": len(ex_files),
                "pair_files": len(pairs), "pairs": len(pairs) // 2,
                "unpaired": len(judged) - len(pairs)})
    try:
        changed, _, untracked = changed_face(root, face, rev, env=env)
    except Refused:
        changed, untracked = set(), []
    out.update({"changed_all": len(changed),
                "changed_face": len([c for c in changed if c.startswith(face + "/")]),
                "untracked": len(untracked)})
    return out


# ── 自检（合成 git 夹具 · 用后删 · 不碰真目标）──────────────────────────────
FIX_PAIR = {"A.zh-CN.md": "# A 真源\n", "A.en.md": "# A derived\n"}
FIX_README = {"README.zh-CN.md": "# 真源\n", "README.md": "# Derived\n"}
FIX_EXC = {"design/格式反馈协议-FFP.md": "# 理念件（zh 单语）\n"}
FIX_REG_SRC = '''# 自检夹具 · 登记面（形状照真件三键：file/mapping/lines）
SINGLE_LANG_EXCEPTIONS = [
    {"file": "design/格式反馈协议-FFP.md", "mapping": "A13", "lines": 263, "reason": "自检夹具"},
]
'''


def fix_write(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as f:
        f.write(text)


def mk_fixture(base):
    """合成 git 仓：一对成对件 + README 对 + 一件已登记的单语例外 + 一件未登记的单语件
    + 一件面外件（`docs/OTHER.md` · 不在扫描面里）+ 合成登记面。"""
    root = os.path.join(base, "repo")
    for rel, txt in list(FIX_PAIR.items()) + list(FIX_README.items()) + list(FIX_EXC.items()):
        fix_write(root, os.path.join("publish/docs", rel), txt)
    fix_write(root, "publish/docs/design/未登记理念.md", "# 未登记（无 en · 只在扫面里 · 不在改动面）\n")
    reg = os.path.join(root, "scripts/gates/reg_fixture.py")
    fix_write(root, os.path.relpath(reg, root), FIX_REG_SRC)
    fix_write(root, "docs/OTHER.md", "# 面外（不在扫描面里）\n")
    env = git_env(base)
    for a in (["init", "-q", "-b", "main"], ["add", "-A"],
              ["commit", "-q", "-m", "fixture"]):
        rc, out = git_run(root, a, env=env)
        if rc != 0:
            raise SystemExit("自检夹具建不起来（git %s ⇒ rc=%d：%s）" % (" ".join(a), rc, out.strip()))
    return root, os.path.relpath(reg, root)


def self_test():
    ok = 0
    fails = []

    def check(label, want, got):
        nonlocal ok
        if want == got:
            ok += 1
            print("   自检 ✓ %-46s rc=%s" % (label, got))
        else:
            fails.append("%s（要 rc=%s · 得 rc=%s）" % (label, want, got))
            print("   自检 ✗ %-46s rc=%s（要 %s）" % (label, got, want))

    def run(root, reg_rel, args):
        try:
            rc, _lines, _ = judge(root, "publish/docs", registration_path=reg_rel, env=git_env(root), quiet=True, **args)
            return rc
        except Refused:
            return EXIT_BLOCKED

    base = tempfile.mkdtemp(prefix="zerg-pairing-selftest-")
    try:
        root, reg = mk_fixture(base)
        env = git_env(base)

        # ① 正控：不碰任何公开面件（改面外件 `docs/OTHER.md` —— 它不在扫描面里）
        fix_write(root, "docs/OTHER.md", "# 面外（不在扫描面里）改了\n")
        check("正控：不碰公开面件 ⇒ 0", EXIT_OK, run(root, reg, {}))
        git_run(root, ["checkout", "--", "docs/OTHER.md"], env=env)

        # ② 负控：只改 zh 不改 en
        fix_write(root, "publish/docs/A.zh-CN.md", "# A 真源（改）\n")
        check("负控：只改 zh 不改 en ⇒ 1", EXIT_RED, run(root, reg, {}))
        git_run(root, ["checkout", "--", "publish/docs/A.zh-CN.md"], env=env)

        # ③ 负控：只改 en 不改 zh
        fix_write(root, "publish/docs/A.en.md", "# A derived（改）\n")
        check("负控：只改 en 不改 zh ⇒ 1", EXIT_RED, run(root, reg, {}))
        git_run(root, ["checkout", "--", "publish/docs/A.en.md"], env=env)

        # ④ 正控：成对同批
        fix_write(root, "publish/docs/A.zh-CN.md", "# A 真源（改）\n")
        fix_write(root, "publish/docs/A.en.md", "# A derived（改）\n")
        check("正控：成对同批 ⇒ 0", EXIT_OK, run(root, reg, {}))
        git_run(root, ["checkout", "--", "publish/docs/A.zh-CN.md", "publish/docs/A.en.md"], env=env)

        # ⑤ 负向 glob 格：改已登记的单语例外 ⇒ 不判成对
        fix_write(root, "publish/docs/design/格式反馈协议-FFP.md", "# 理念件（改了）\n")
        check("负向 glob：改已登记单语例外 ⇒ 0", EXIT_OK, run(root, reg, {}))
        git_run(root, ["checkout", "--", "publish/docs/design/格式反馈协议-FFP.md"], env=env)

        # ⑥ 负控：改动面里出现未登记的单语件（**笔面** · 提交一笔 ⇒ 它才进 tracked 改动面）
        fix_write(root, "publish/docs/design/未登记理念.md", "# 未登记（无 en · 未登记 · 这笔真的动了它）\n")
        git_run(root, ["commit", "-q", "-a", "-m", "改未登记的单语件"], env=env)
        check("负控：改动面里未登记单语件 ⇒ 1", EXIT_RED, run(root, reg, {"rev": "HEAD~1..HEAD"}))
        git_run(root, ["reset", "-q", "--hard", "HEAD~1"], env=env)

        # ⑦ 未登记的单语件「不在改动面」⇒ 只打印不判红（静态成对性归 J0 / ⑧b ③）
        check("不成对件不在改动面 · 只打印不判红 ⇒ 0", EXIT_OK, run(root, reg, {}))

        # ⑧ 机器人笔豁免（`A..B` 形态 · 机器人作者那一笔只改 zh ⇒ 剔除后无单边改动）
        fix_write(root, "publish/docs/A.zh-CN.md", "# A 真源（机器人改）\n")
        rc, out = git_run(root, ["commit", "-q", "-a", "--author=dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
                                 "-m", "bot: 只改 zh"], env=env)
        if rc != 0:
            check("自检夹具：机器人笔建不起来", 0, rc)
        else:
            check("机器人笔豁免（只改 zh 的机器人笔）⇒ 0", EXIT_OK, run(root, reg, {"rev": "HEAD~1..HEAD"}))

        # ⑨ 同一笔改成「人笔」⇒ 同一改动面变红（成对负控：证明豁免真的按作者分辨）
        fix_write(root, "publish/docs/A.zh-CN.md", "# A 真源（人改）\n")
        git_run(root, ["commit", "-q", "-a", "-m", "人: 只改 zh"], env=env)
        check("同一改动面（人笔）⇒ 1（成对负控）", EXIT_RED, run(root, reg, {"rev": "HEAD~1..HEAD"}))

        # ⑩ 机器人分支豁免（分支面）
        git_run(root, ["checkout", "-q", "-b", "chore/crowdin"], env=env)
        fix_write(root, "publish/docs/A.zh-CN.md", "# A 真源（机器人分支改）\n")
        check("机器人分支豁免（chore/crowdin）⇒ 0", EXIT_OK, run(root, reg, {}))
        git_run(root, ["checkout", "--", "publish/docs/A.zh-CN.md"], env=env)
        git_run(root, ["checkout", "-q", "main"], env=env)

        # ⑪ untracked 件 ⇒ 不在改动面 ⇒ 不判
        fix_write(root, "publish/docs/B.zh-CN.md", "# B（未跟踪 · 无 en）\n")
        check("untracked 件不在改动面 ⇒ 0", EXIT_OK, run(root, reg, {}))
        os.remove(os.path.join(root, "publish/docs/B.zh-CN.md"))

        # ⑫ 空扫面 ⇒ rc=2
        empty = os.path.join(base, "empty")
        os.makedirs(os.path.join(empty, "publish/docs"), exist_ok=True)
        git_run(empty, ["init", "-q"], env=git_env(base))
        rc = run(empty, reg, {})
        check("空扫面（0 件）⇒ 2", EXIT_BLOCKED, rc)

        # ⑬ 面缺件 ⇒ rc=2
        check("面不在 ⇒ 2", EXIT_BLOCKED, run(base, reg, {}))

        # ⑭ rev 解不出 ⇒ rc=2
        check("rev 解不出 ⇒ 2", EXIT_BLOCKED, run(root, reg, {"rev": "no-such-rev"}))

        # ⑮ 例外登记面读不到 ⇒ rc=2（缺件 ≠ 绿 · `T-11b` 前置的机器化落点）
        check("例外登记面缺件 ⇒ 2", EXIT_BLOCKED, run(root, "scripts/gates/no-such-reg.py", {}))
    finally:
        import shutil
        shutil.rmtree(base, ignore_errors=True)

    total = 15
    if len(fails) or ok != total:
        print("   自检**不过**：断言 %d 格 · 通过 %d · 失败 %d" % (total, ok, len(fails)))
        for f in fails:
            print("     ✗ %s" % f)
        if ok != total and not fails:
            print("     ✗ 断言条数对不上（通过 %d ≠ 声明 %d）—— 用例被静默截断" % (ok, total))
        return EXIT_BLOCKED
    print("   自检 ✓ 断言 %d 格全过（失败 0 格）" % total)
    return EXIT_OK


def list_rules():
    rows = [
        ("P0", "扫描面（`paths:` 形态）：正向 `<面>/**/*.md` + 负向 glob 逐条现读自 `T-11b` 登记面"),
        ("P1", "机器人面豁免：分支面（机器人直推分支前缀表）+ 笔面（`[bot]` / `-bot@` 作者 · `A..B` 形态）"),
        ("P2", "负控：成对件只动一侧 ⇒ rc=1（逐对点名 · 两向分列）· 改动面里未登记的单语件 ⇒ 同判 rc=1"),
        ("P3", "正控：不碰任何公开面件 ⇒ rc=0（逐字打印计数 · 不静默）"),
        ("P4", "只管 tracked 的改动面；untracked 件逐条打印「不在改动面 ⇒ 不判」"),
        ("P5", "静态成对性只打印不判红（`T-13` 的 `J0` / 闸⑧b ③ 的面 · 不重复判）"),
        ("P6", "空转/缺件/登记面读不到/git 读不到/rev 解不出/自检不过 ⇒ rc=2（不给结论）"),
    ]
    print("── %s ──" % VERSION)
    print("   面（默认）: %s · 成对映射: %s ↔ %s · %s ↔ %s" % (FACE_DEFAULT, ZH_SUFFIX, EN_SUFFIX, README_ZH, README_EN))
    print("   单一真源（例外登记面）: %s" % REGISTRATION_REL)
    print("   负向 glob 形态（参照件 nodejs translations-pr-lint.yml 逐字）: '!<目录>/**' 剔掉已登记的单语例外子树")
    print("   机器人分支表（逐条带理由）:")
    for p, why in BOT_BRANCH_PREFIXES:
        print("     · %-16s %s" % (p, why))
    print("   拒收的逃生开关（一律 rc=2）: %s" % " · ".join(REFUSE_FLAGS))
    for k, v in rows:
        print("   %s %s" % (k, v))


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    for f in REFUSE_FLAGS:
        if f in argv:
            print("⛔ 拒收：`%s` —— 例外只能**显式登记**（登记面在 %s 里）⇒ **不给结论（rc=2）**"
                  % (f, REGISTRATION_REL))
            return EXIT_BLOCKED
    ap = argparse.ArgumentParser(add_help=True, description="公开面成对闸（`T-12` · 漏点 `E-1`）")
    ap.add_argument("--root", default=REPO_ROOT, help="判据树（默认 = 本仓根）")
    ap.add_argument("--face", default=FACE_DEFAULT, help="扫描面（仓根相对 · 默认 %s）" % FACE_DEFAULT)
    ap.add_argument("--rev", default=None, help="改动面：单 rev（<rev> → 工作树）或区间 A..B（逐笔按作者归属）")
    ap.add_argument("--registration", default=REGISTRATION_REL, help="例外登记面（单一真源 · 默认 %s）" % REGISTRATION_REL)
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成 git 夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（闸里按名调用用这个）")
    ap.add_argument("--list-rules", action="store_true", help="列出判据 / 退码 / 单一真源")
    ap.add_argument("--json", action="store_true", help="机器面（rc + 计数）")
    a = ap.parse_args(argv)

    if a.list_rules:
        list_rules()
        return EXIT_OK
    if a.self_test:
        return self_test()
    if not a.no_self_test:
        rc_st = self_test()
        if rc_st != EXIT_OK:
            print("⛔ 自检不过 ⇒ 拒绝扫真目标（同族先例 check-publish-face-sync.py ④）")
            return EXIT_BLOCKED

    root = os.path.abspath(a.root)
    try:
        if a.json:
            c = counts_of(root, a.face, rev=a.rev, registration_path=a.registration)
            c["rc"] = EXIT_OK
            print(json.dumps(c, ensure_ascii=False, sort_keys=True))
            return EXIT_OK
        print("── 公开面成对闸（`T-12` · 漏点 `E-1`）──")
        rc, _lines, _ = judge(root, a.face, rev=a.rev, registration_path=a.registration)
        return rc
    except Refused as exc:
        print("⛔ %s" % exc)
        return EXIT_BLOCKED


if __name__ == "__main__":
    sys.exit(main())

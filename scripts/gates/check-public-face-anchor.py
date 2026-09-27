#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-face-anchor.py —— 公开面**漂移锚**（`T-13` · 漏点 `E-2` · 缺口 `Q-128`）

设计真源（**逐字**，不转述）
--------------------------
`Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md:46`（`T-13`）逐字：
  「**`T-13`** | **漂移锚**：形态照 `google/comprehensive-rust` 的 `check-msgid-changes.py`
   （docstring 逐字「Find changed msgid fields without a change in POT-Creation-Date」）**【已复核】** |
   **高** | `C` | **负控**：「**锚未变而正文变 = 红**」（只改译文不动锚 ⇒ 红）；
   **正控**：锚与正文同批 ⇒ 绿 | `E-2` | 未开始 |」
参照件（`E-2` 的真源 · 已复核）—— `google/comprehensive-rust`
  `.github/workflows/check-msgid-changes.py` 件头 docstring 逐字：
  「Find changed msgid fields without a change in POT-Creation-Date.」
  其判据逐字（转写其代码的三步）：① 逐个改动件看 `POT-Creation-Date` 是否变；
  ② 变了 ⇒ 认作 `msgmerge --update` 的产物，逐字打印「Assuming … was changed automatically,
  skipping msgid check」并**跳过**；③ 没变 ⇒ 再看 `msgid` 区（正文）有没有改动行，有 ⇒ 逐字
  「Changed msgid in file …:N!」并 `exit(1)`。
`Zerg-内部文档/项目文档/v2.5.12/设计-公开文档面随版收口-v1.1-20260923.md:21`（修正②①）逐字：
  「**漂移锚**（成对件的锚必须能机判「一动就得两边动」）」
同件 `:385`（`E-2`）逐字：
  「缺「**锚未变而正文变 = 红**」的现成形态 … 翻译派生最阴的腐化是「**锚没变、译文静默过期**」
   ⇒ 没有这条判据，成对件「看着同步、实际过期」」
同件 `:410`（实施顺序第 4 步）逐字：
  「**成对闸与漂移锚** … 落点在**成对件的判据**（六对 + 两件单语的例外要显式登记）」
`Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md:121`（依赖表 `T-13` 行）逐字：
  「`T-13` 漂移锚 | 档 `C` 第 2 条 | **不挡**（可作闸⑧ 的加固判据）」
`Zerg-内部文档/项目文档/v2.5.12/缺口总账-20260923.md:130`（`Q-128`）逐字摘：
  「**子任务甲**：7 件 en 派生件各补「派生自 zh-CN（真源在 zh 侧）」一行 + 落「**锚未变而正文变 = 红**」
   的派生过期检测」

口径：本件把「锚」与「正文」落在哪一侧（**本件现读 · 逐条给理由**）
------------------------------------------------------------------------
逐字口径只给了判据（锚/正文各是什么**没写**）⇒ 本件必须把两侧**指定下来**，且**只指定一次**：

  **锚 = 成对件里的真源侧**（`*.zh-CN.md` / `README.zh-CN.md`）
  **正文 = 成对件里的派生侧**（`*.en.md` / `README.md`）

  ① 映射关系照参照件**一刀不变**：参照件的锚（`POT-Creation-Date`）与正文（`msgid`）**同在一件**里，
     判据形状 = 「**锚没动而正文动了 ⇒ 红；两者同批 ⇒ 绿**」。本仓的分法是 `zh-CN` 真源 / `en` 派生，
     两者**分在两件** ⇒ 判据形状原样搬过来（同批 ⇒ 绿 · 单边 ⇒ 红），只换了「锚/正文在哪件」。
  ② 与设计稿 `:21` 的逐字（「锚必须能机判**「一动就得两边动」**」）**同向**：真源动 ⇒ 派生必须跟着动；
     派生单边动 ⇒ 就是「只改译文不动锚」。
  ③ **本件现读**（`git show --stat` 逐笔）：`publish/docs` 最近 **8 笔**改动**全部**是「中英一对」
     （`29e01e87`/`df9177f7`/`502a6e43`/`1c261004`/`2200db1a`/`2b39ab45`/`f269d99a`/`843adce7` —— 逐笔两件）
     ⇒ 「一边动另一边不动」这种形态**今天在真仓里不存在**，本件判的正是**将来出现的单边改动**。

★ **另一种读法（本件不取 · 照实登记）**：把「锚」读成**派生件自己那行真源声明**
  （`Source of truth: the Chinese original (…zh-CN.md) …` / `T-11a` 要补的「派生自 zh-CN（真源在 zh 侧）」）
  ⇒ 判据就成了「改派生件的正文**必须同时动那一行**」。**不取它的两条理由**：
  ⓐ 那一行是**指针、不带指纹**（本件现读 `CONFIGURATION.en.md`/`MODELS.en.md`/`TOOLS.en.md` 三件，逐字只有
     `[X.zh-CN.md](X.zh-CN.md)` 链接、无 `sha256`/版本号一类可比物）⇒ 它**判不出**「一动就得两边动」，
     取它等于「每次改正文都必红」（把**全部**正文编辑判红 = 判据没有区分度 ✗）；
  ⓑ 本仓铁律「**假红**与**假绿**同罪」⇒ 取一个必自造假红的读法，等于把「上闸前先登记」那条前置**原地废掉**。
  ⇒ 该读法要在**同一条线上**落地，得先给那一行加**可比指纹**（属 `T-11a` 的写面）⇒ 本件**不越界自造** ✗。

判据（逐条 · 机器可判）
----------------------
  `J0` **成对性 + 例外登记（前置 · 缺即 rc=2）**：扫面里**每一件**必须归入两类之一 ——
       ⓐ **成对**（`X.zh-CN.md` ↔ `X.en.md`；`README.zh-CN.md` ↔ `README.md` 这个镜像对也在内）
       ⓑ **已登记的单一语例外**（本件 `SINGLE_LANG_EXCEPTIONS` · **每条带理由**）。
       既不成对、又没登记的件 ⇒ **rc=2**（逐条点名 + 指向 `T-11b` 的登记面）。
       —— 这正是收口清单依赖表「`T-12`/`T-13` **前置 = `T-11b`**」的机器化落点：
       例外不先登记 ⇒ 成对件当场假红 ✗。
  `J1` **`T-13` 本条 · 负控**：成对件里「**锚未变而正文变**」（真源侧不在 diff 面、派生侧在）
       ⇒ **红（rc=1）**，逐条点名该对 + 指出「正文侧动了、锚侧一字未动」。
  `J2` **正控**：锚与正文**同批**（两侧都在 diff 面）⇒ **绿**（逐对打印「同批」）。
  `J3` **附报 · `T-11a` ② 面（**不改退码**）**：「锚变而正文未变」（真源侧动了、派生侧没跟）
       ⇒ 逐条打印「**派生过期（stale）**」。照 `T-13` 逐字只判 ①（负控/正控两格），
       ② 「en 落后 zh 要**可读出来**」是 `T-11a` 那一行的面 ⇒ 本件**只报不判**（改了退码就等于替它判了）。
  `J4` **只在 tracked 的 diff 面里判**：diff 面 = `git -c core.quotepath=false diff --name-only <rev> -- <扫面>`（`<rev>` 缺省 `HEAD`）。
       **untracked 件不在 diff 面** ⇒ **逐条打印「不在 diff 面 ⇒ 不判」**（本仓多写手并行 ⇒
       别把兄弟写手在途的未跟踪件判成红 ✗）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿：扫面全成对（或全是**已登记**的单一语例外）· diff 面里**没有**单边改动（同批或空 diff 面）。
  1 = **有漂移**：至少一对「锚未变而正文变」⇒ 逐条点名（处置只有两条：补上真源侧的改动，或撤掉单边改动；
      **不许**放宽判据来凑绿 ✗）。
  2 = **不给结论**：扫面不在 / 扫面 **0 件**（空转）/ 件不成对又**未登记例外** / `git` 读不到（非仓 / `rev` 解不出）/
      件读不了 / `--self-test` 不过 / **拒收的逃生开关**（见下）。

用法
----
    python3 scripts/gates/check-public-face-anchor.py                    # 判真目标（先自检，再判）
    python3 scripts/gates/check-public-face-anchor.py --rev HEAD~3       # 判「HEAD~3 → 工作树」这段
    python3 scripts/gates/check-public-face-anchor.py --rev A..B         # 判一笔提交区间
    python3 scripts/gates/check-public-face-anchor.py --face publish/docs --root <树>
    python3 scripts/gates/check-public-face-anchor.py --list-rules
    python3 scripts/gates/check-public-face-anchor.py --self-test        # 合成 git 夹具 · 不碰真目标
    python3 scripts/gates/check-public-face-anchor.py --json rc,counts,drift,stale

★ **拒收的逃生开关**：`--waive` / `--allow-unregistered` / `--ignore-missing` / `--skip-pairs`
  一律 **rc=2 被拒** —— 例外只能**显式登记**（`SINGLE_LANG_EXCEPTIONS` 逐条带理由），
  不许用开关把判据**绕过去**（同族先例：`check-keep-marks.py` 拒 `--block` 一类旗标）。
★ **不接管道取 rc**（本仓门禁硬规矩：管道后 `$?` 是末端 rc）—— 本件自己不做任何 shell 调用，
  只经 `subprocess` 直接起 `git` 并读它的 `returncode`。

**不接闸**（本批：只产件、不接线 ✗）：本件**未**接进 `scripts/gates/precommit-gates.sh`
（`T-13` 依赖表逐字：「**不挡**（可作闸⑧ 的加固判据）」⇒ 挂闸与挂 scope 属**另批**）；
也**未**进 `scripts/公开标记.tsv`（登记面）、**未**进任何台账（缺口总账 / 收口清单 §一 状态列）——
三者一律按单写入者纪律留给收口批，**本件一个既有件的一行都没改** ✓。
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile

VERSION = "check-public-face-anchor.py v1.0（`T-13` 漂移锚 · 2026-09-24 落件 · **未接线**）"

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))

FACE_DEFAULT = "publish/docs"          # 扫面（仓根相对 · 公开文档面）
ZH_SUFFIX = ".zh-CN.md"                # 真源侧（= 锚）
EN_SUFFIX = ".en.md"                   # 派生侧（= 正文）
WALK_PRUNE = {".git", "node_modules", "target"}
REFUSE_FLAGS = ("--waive", "--allow-unregistered", "--ignore-missing", "--skip-pairs")

# ── 单一语例外登记表（**正式登记** · `T-11b` 接手落地 · 2026-09-24）──────────────────
# 每条 = 一件「公开面里没有对应派生件/真源件」的件 + **一条理由**（逐条带理由 = 硬要求）。
# ★ **登记面 = 本表**（单一真源）：`T-13` 落地时这里是**占位**（逐条写「登记归 `T-11b`」）；
#   `T-11b` 已**接手 / 复核**并逐条补全「映射号 + 行数 + 出处」。`T-15` 闸⑧ 的成对性那一格
#   （`scripts/build/publish-preflight.sh` 内联 `SINGLE_LANG`）是**现读位置**，口径以本表为准。
# 真源（**逐字** · 三处）：
#   `Zerg-内部文档/项目文档/v2.5.12/任务清单-公开文档面收口-20260923.md:44`（`T-11b`）——
#     「把 `design/格式反馈协议-FFP.md`（263 行）与 `design/记忆体系.md`（346 行）的**单语例外显式登记**
#      （映射 `A13`/`A14` **各只有 zh 一条**）」；
#   缺口 `Q-128` ③ ——「`design/` 两件的**单语例外显式登记**（逐件一条理由）⇒ **上闸前**先登记，
#     否则成对闸假红 ✗」；设计稿 `:410` ——「两件单语的例外要显式登记」。
# ★ 映射出处（**现读**）：`publish/mirror-public-lib.py:310–311` —— `A13`/`A14` 两行**各只有 zh 一条**。
# ★ **登记 ≠ 决定**：`Q-128` ④ 的开放问题（这两件**要不要补 en 对应件**）**本表不定**（须一句拍）。
# ★ 悬空条目（登记了、件却不在扫面里）⇒ **只打印、不判红**（同族口径：
#   `check-public-face-wording.py` 的白名单「悬空登记只打印、不判红」——判红会让「把件删掉」变假红）。
SINGLE_LANG_EXCEPTIONS = [
    {"file": "design/格式反馈协议-FFP.md", "mapping": "A13", "lines": 263,
     "reason": "中文单语「理念件」（`Q-128` ②③ · `T-11b` **正式登记**）：公开面映射 `A13` **只有 zh 一条**"
               "（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；"
               "★ 「要不要补 en」= `Q-128` ④ 开放问题 · **本表不定**"},
    {"file": "design/记忆体系.md", "mapping": "A14", "lines": 346,
     "reason": "中文单语「理念件」（同 `Q-128` · `T-11b` **正式登记**）：公开面映射 `A14` **只有 zh 一条**"
               "（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；"
               "★ 「要不要补 en」= 同上开放问题 · **本表不定**"},
]


# ── 扫面枚举 / 成对分类 ────────────────────────────────────────────────────────
def enumerate_face(face_dir):
    """現读扫面：`<扫面>/**/*.md`（**排序稳定** · 逐件不许丢）。"""
    out = []
    for dirpath, dirnames, filenames in os.walk(face_dir):
        dirnames[:] = sorted(d for d in dirnames if d not in WALK_PRUNE)
        for fn in sorted(filenames):
            if not fn.endswith(".md"):
                continue
            rel = os.path.relpath(os.path.join(dirpath, fn), face_dir)
            out.append(rel.replace(os.sep, "/"))
    return sorted(out)


def classify(files_):
    """把扫面分成三段：成对 / 单一语 / 未成对（**逐件给原因**）。"""
    fs = set(files_)
    pairs, singles, unpaired, consumed = [], [], [], set()
    # ① 先认真源侧（`*.zh-CN.md`）
    for rel in files_:
        if not rel.endswith(ZH_SUFFIX):
            continue
        stem = rel[: -len(ZH_SUFFIX)]
        cands = [stem + EN_SUFFIX]
        if stem == "README":
            cands.append("README.md")              # README.zh-CN.md ↔ README.md（公开面唯一的镜像对）
        en = next((c for c in cands if c in fs), None)
        if en:
            pairs.append((rel, en))
            consumed |= {rel, en}
        else:
            unpaired.append((rel, "真源侧没有对应派生件（试过：%s）" % " / ".join(cands)))
    # ② 剩下的件：派生侧但真源缺 ⇒ 未成对；真源侧已在 ① 里登记过 ⇒ 不重复；其余 ⇒ 单一语
    for rel in files_:
        if rel in consumed:
            continue
        if rel.endswith(EN_SUFFIX):
            stem = rel[: -len(EN_SUFFIX)]
            unpaired.append((rel, "派生侧没有对应真源件（试过：%s）" % (stem + ZH_SUFFIX)))
        elif rel.endswith(ZH_SUFFIX):
            pass                       # 已在 ① 里进 unpaired（真源缺派生件）—— 不许在这里重复计数
        else:
            singles.append(rel)
    return pairs, singles, unpaired


# ── git 读面（只读 · 不写仓）────────────────────────────────────────────────────
def git_run(root, args, env=None):
    return subprocess.run(["git", "-c", "core.quotepath=false", "-C", root] + list(args), capture_output=True, text=True,
                          encoding="utf-8", errors="replace", env=env)


def git_ok(root):
    p = git_run(root, ["rev-parse", "--git-dir"])
    return p.returncode == 0, p.stderr.strip()


def rev_resolvable(root, rev):
    p = git_run(root, ["rev-parse", "--verify", "--quiet", rev + "^{commit}"])
    return p.returncode == 0


def git_changed(root, rev, face_rel):
    """diff 面（**tracked 改动** · 仓根相对 → 扫面相对）⇒ (set, err)。"""
    p = git_run(root, ["-c", "core.quotepath=false", "diff", "--name-only", rev, "--", face_rel])
    if p.returncode != 0:
        return None, (p.stderr.strip() or "git diff rc=%d" % p.returncode)
    pre = face_rel.rstrip("/") + "/"
    out = set()
    for ln in p.stdout.split("\n"):
        ln = ln.strip()
        if not ln:
            continue
        out.add(ln[len(pre):] if ln.startswith(pre) else ln)
    return out, ""


# ── 判定 ───────────────────────────────────────────────────────────────────────
def judge(root, face_rel, rev, exceptions):
    """⇒ (rc, lines, data)。**不改任何件**（只 walk + git 只读）。"""
    lines, data = [], {}
    face_dir = os.path.join(root, face_rel)

    # 守卫一：扫面不在 ⇒ rc=2（缺件 ≠ 绿）
    if not os.path.isdir(face_dir):
        lines.append("⛔ 不给结论（rc=2）：扫面不在 ⇒ %s" % os.path.join(root, face_rel))
        return 2, lines, {"rc": 2, "verdict": "missing-face", "face": face_rel}

    files_ = enumerate_face(face_dir)
    # 守卫二：空转 ⇒ rc=2（扫到 0 件就不给结论 · 照仓内「空转 = 假覆盖」口径）
    if not files_:
        lines.append("⛔ 不给结论（rc=2）：扫面 0 件（空转 = 假覆盖）⇒ %s" % face_dir)
        return 2, lines, {"rc": 2, "verdict": "vacuous", "face": face_rel, "files": 0}

    pairs, singles, unpaired = classify(files_)
    exc_rel = [e.get("file", "") for e in exceptions if e.get("file")]
    exc = set(exc_rel)
    dangling = [r for r in exc_rel if r not in files_]              # 悬空登记（只打印 · 不判红）
    unregistered = [r for r in singles if r not in exc] + [u[0] for u in unpaired if u[0] not in exc]

    data.update({"face": face_rel, "files": len(files_), "pairs": len(pairs),
                 "singles": len(singles), "unpaired": len(unpaired),
                 "exceptions": len(exc_rel), "dangling_exceptions": dangling})

    lines.append("── 公开面漂移锚（`T-13` · 负控「锚未变而正文变 = 红」· 正控「锚与正文同批 ⇒ 绿」）──")
    lines.append("VERSION: " + VERSION)

    # `J0` 成对性 + 例外登记（前置）
    if unregistered:
        lines.append("")
        lines.append("⛔ 不给结论（rc=2）：`J0` 成对性 + 例外登记 —— 下列件**既不成对、又未登记例外**：")
        for r in unregistered:
            why = dict(unpaired).get(r, "单一语件（无 `%s` 对应件）" % EN_SUFFIX)
            lines.append("   · %s —— %s" % (r, why))
        lines.append("   ⇒ 例外必须先**显式登记**（本件 `SINGLE_LANG_EXCEPTIONS` · 逐条带理由）；")
        lines.append("      登记面 = 本表（`T-11b` **已正式登记** · 2026-09-24 落地 · 两件 `design/` 中文单语例外）。")
        lines.append("   ✗ 不许用 `%s` 一类开关绕过（本件一律拒 rc=2）。" % " / ".join(REFUSE_FLAGS[:3]))
        data["rc"] = 2
        data["verdict"] = "unregistered"
        data["unregistered"] = unregistered
        return 2, lines, data

    # 守卫三：git 读面
    ok, err = git_ok(root)
    if not ok:
        lines.append("⛔ 不给结论（rc=2）：`git` 读不到 ⇒ %s（%s）" % (root, err or "非仓"))
        data.update({"rc": 2, "verdict": "no-git"})
        return 2, lines, data
    if not rev_resolvable(root, rev):
        lines.append("⛔ 不给结论（rc=2）：`rev` 解不出 ⇒ %s（仓 %s）" % (rev, root))
        data.update({"rc": 2, "verdict": "bad-rev", "rev": rev})
        return 2, lines, data

    changed, cerr = git_changed(root, rev, face_rel)
    if changed is None:
        lines.append("⛔ 不给结论（rc=2）：取 diff 面失败 ⇒ %s" % cerr)
        data.update({"rc": 2, "verdict": "no-diff"})
        return 2, lines, data

    # `J1`/`J2`/`J3` 逐对判
    drift, stale, both, untouched = [], [], [], []
    for zh, en in pairs:
        z, e = zh in changed, en in changed
        if z and e:
            both.append((zh, en))                       # J2 正控：锚与正文同批 ⇒ 绿
        elif e and not z:
            drift.append((zh, en))                      # J1 负控：锚未变而正文变 ⇒ 红
        elif z and not e:
            stale.append((zh, en))                      # J3 附报：派生过期（不改退码）
        else:
            untouched.append((zh, en))

    # 单一语例外件在 diff 面里 ⇒ 照实报「无锚可比 · 本件不判」
    exc_changed = [r for r in exc_rel if r in changed]
    # `J4` untracked 不在 diff 面 ⇒ 逐条打印（别把兄弟写手在途件判成红）
    not_in_diff = [r for r in files_ if r not in changed]

    lines.append("扫面：%s —— 现读 %d 件 = 成对 **%d 对** + 单一语 **%d 件**（已登记例外 %d 条）"
                 % (face_rel, len(files_), len(pairs), len(singles), len(exc_rel)))
    lines.append("diff 面：`git -c core.quotepath=false diff --name-only %s -- %s` —— tracked 改动 **%d** 件"
                 % (rev, face_rel, len(changed)))
    lines.append("")
    lines.append("── 逐对判定（`J1` 负控 / `J2` 正控 / `J3` 附报）──")
    for zh, en in drift:
        lines.append("   [**红·漂移**] %s ↔ %s —— **锚未变而正文变**（正文侧在 diff 面、锚侧一字未动）"
                     % (zh, en))
    for zh, en in both:
        lines.append("   [绿·同批]   %s ↔ %s —— 锚与正文同批（正控）" % (zh, en))
    for zh, en in stale:
        lines.append("   [附报·派生过期] %s ↔ %s —— **锚变而正文未变**（`T-11a` ② 面 · 本件只报不判 · "
                     "不动退码）" % (zh, en))
    for zh, en in untouched:
        lines.append("   [未判·无输入] %s ↔ %s —— 两侧都不在 diff 面" % (zh, en))
    if exc_changed:
        lines.append("")
        lines.append("── 单一语例外件在 diff 面里（**无锚可比 ⇒ 本件不判**）──")
        for r in exc_changed:
            lines.append("   · %s（已登记例外 · 处置面 = 该件自己的门）" % r)
    if dangling:
        lines.append("")
        lines.append("── 悬空例外登记（**只打印 · 不判红** · 同族口径）──")
        for r in dangling:
            lines.append("   · %s（登记了但扫面里没有这件 ⇒ 悬空）" % r)
    if not_in_diff:
        lines.append("")
        lines.append("── `J4` 不在 diff 面（**不判** · 本仓多写手并行 ⇒ 兄弟写手的在途件不算漂移）──")
        for r in not_in_diff:
            lines.append("   · %s" % r)

    rc = 1 if drift else 0
    lines.append("")
    lines.append("⇒ 成对 %d · 同批 %d · **漂移 %d** · 派生过期(附报) %d · 未判 %d 对 · 不在 diff 面 %d 件 · rc=%d"
                 % (len(pairs), len(both), len(drift), len(stale), len(untouched),
                    len(not_in_diff), rc))
    if rc == 1:
        lines.append("   处置只有两条：**补上真源侧的改动**，或**撤掉单边改动** —— ✗ 不许放宽判据来凑绿。")
    else:
        lines.append("   `J0` 成对性 + 例外登记过 ✓ · `J1` 无漂移 ✓（`J3` 的派生过期只报 · 不改退码）")

    data.update({
        "rc": rc, "verdict": "drift" if drift else "green", "rev": rev,
        "counts": {"pairs": len(pairs), "both": len(both), "drift": len(drift),
                   "stale": len(stale), "untouched": len(untouched),
                   "changed": len(changed), "not_in_diff": len(not_in_diff),
                   "singles": len(singles), "exceptions": len(exc_rel),
                   "dangling_exceptions": len(dangling), "unregistered": 0},
        "drift": [{"zh": z, "en": e} for z, e in drift],
        "stale": [{"zh": z, "en": e} for z, e in stale],
        "both": [{"zh": z, "en": e} for z, e in both],
        "not_in_diff": not_in_diff,
        "exceptions": exc_rel,
    })
    return rc, lines, data


# ── 自检（合成 git 夹具 · 不碰真目标）──────────────────────────────────────────
def git_env(base):
    """**工作区隔离**（本仓铁律：自证用例必须自带隔离，否则会往真仓落盘）—— 只给夹具树用。"""
    env = dict(os.environ)
    env.update({
        "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_SYSTEM": os.devnull,
        "GIT_AUTHOR_NAME": "anchor-selftest", "GIT_AUTHOR_EMAIL": "anchor@example.invalid",
        "GIT_COMMITTER_NAME": "anchor-selftest", "GIT_COMMITTER_EMAIL": "anchor@example.invalid",
        "HOME": base,
    })
    return env


def mk_fixture(base, files):
    """合成仓：`<base>/repo/<扫面>/<rel>` = files，`git init` + 一笔初始提交。"""
    root = os.path.join(base, "repo")
    face = os.path.join(root, FACE_DEFAULT)
    os.makedirs(face, exist_ok=True)
    for rel, txt in (files or {}).items():
        p = os.path.join(face, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(txt)
    env = git_env(base)
    git_run(root, ["init", "-q", "-b", "main"], env=env)
    git_run(root, ["add", "-A"], env=env)
    git_run(root, ["commit", "-q", "-m", "init"], env=env)
    return root


def touch(root, rel, txt):
    p = os.path.join(root, FACE_DEFAULT, rel)
    with open(p, "a", encoding="utf-8") as f:
        f.write(txt)


PAIR_A = {"A.zh-CN.md": "# A 真源\n", "A.en.md": "# A derived\n"}
README_PAIR = {"README.zh-CN.md": "# 真源\n", "README.md": "# Derived\n"}


def self_test():
    checks = []

    def check(cond, desc):
        checks.append((bool(cond), desc))
        print("   %s %s" % ("✓" if cond else "✗", desc))

    t = tempfile.mkdtemp(prefix="pubface-anchor-selftest-")
    try:
        # ① 正控：锚与正文**同批** ⇒ rc=0（绿）
        r = mk_fixture(os.path.join(t, "both"), dict(PAIR_A))
        touch(r, "A.zh-CN.md", "真源加了半行\n")
        touch(r, "A.en.md", "derived added half a line\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and d["counts"]["both"] == 1 and d["counts"]["drift"] == 0,
              "① 正控：锚与正文**同批** ⇒ rc=0（同批 1 · 漂移 0）")

        # ② 负控（`T-13` 本条的牙齿）：**锚未变而正文变** ⇒ rc=1（红）
        r = mk_fixture(os.path.join(t, "drift"), dict(PAIR_A))
        touch(r, "A.en.md", "只改了译文这一行\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 1 and d["counts"]["drift"] == 1
              and any("锚未变而正文变" in ln for ln in o),
              "② 负控：「只改译文不动锚」⇒ rc=1（漂移 1 · 逐条点名「锚未变而正文变」）")

        # ③ 附报：**锚变而正文未变** ⇒ rc=0 且 stale=1（**不改退码** —— 照 `T-13` 只判 ①）
        r = mk_fixture(os.path.join(t, "stale"), dict(PAIR_A))
        touch(r, "A.zh-CN.md", "真源动了\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and d["counts"]["stale"] == 1
              and any("派生过期" in ln for ln in o),
              "③ 附报：锚变而正文未变 ⇒ rc=0 且报「派生过期」1 处（`T-11a` ② 面 · **不动退码**）")

        # ④ 无输入：两侧都不动 ⇒ diff 面为空 ⇒ rc=0
        r = mk_fixture(os.path.join(t, "none"), dict(PAIR_A))
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and d["counts"]["changed"] == 0 and d["counts"]["untouched"] == 1,
              "④ 无输入：diff 面为空 ⇒ rc=0（未判 1）")

        # ⑤ README 镜像对识别（`README.zh-CN.md` ↔ `README.md` · 公开面唯一不同名的一对）
        r = mk_fixture(os.path.join(t, "readme"), dict(README_PAIR))
        touch(r, "README.en.md", "")  # 噪音：不该被当派生件（无真源）
        os.remove(os.path.join(r, FACE_DEFAULT, "README.en.md"))
        touch(r, "README.md", "derived only\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 1 and d["counts"]["pairs"] == 1 and d["counts"]["drift"] == 1,
              "⑤ 镜像对：`README.md` 认作 `README.zh-CN.md` 的派生侧 ⇒ 单边改 ⇒ rc=1（成对 1）")

        # ⑥ 未成对（真源缺派生件）且未登记 ⇒ rc=2
        r = mk_fixture(os.path.join(t, "zhonly"), {"B.zh-CN.md": "# B 真源\n"})
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 2 and d.get("unregistered") == ["B.zh-CN.md"],
              "⑥ 未成对：`B.zh-CN.md` 无派生件、未登记 ⇒ rc=2（`J0` 前置有牙）")

        # ⑦ 未成对（派生缺真源）且未登记 ⇒ rc=2
        r = mk_fixture(os.path.join(t, "enonly"), {"C.en.md": "# C derived\n"})
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 2 and d.get("unregistered") == ["C.en.md"],
              "⑦ 未成对：`C.en.md` 无真源件、未登记 ⇒ rc=2")

        # ⑧ 单一语例外**已登记** ⇒ 不假红 · rc=0（登记有牙：与 ⑨ 成对才有区分度）
        r = mk_fixture(os.path.join(t, "exc-ok"),
                       {"design/格式反馈协议-FFP.md": "# 理念件（单语）\n", **PAIR_A})
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and d["counts"]["singles"] == 1 and d["counts"]["exceptions"] == 2,
              "⑧ 登记例外：单语件在**已登记**表里 ⇒ rc=0（单一语 1 · 不假红）")

        # ⑨ 同一件**撤掉登记** ⇒ rc=2（证明登记面不是摆设）
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", [])
        check(rc == 2 and "design/格式反馈协议-FFP.md" in (d.get("unregistered") or []),
              "⑨ 与 ⑧ 成对：同一件**撤掉登记** ⇒ rc=2（登记面有区分度 · 不恒绿）")

        # ⑩ 悬空例外（登记了但件不在）⇒ **只打印 · 不判红** ⇒ rc=0
        r = mk_fixture(os.path.join(t, "dangling"), dict(PAIR_A))
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and d["counts"]["dangling_exceptions"] == 2
              and any("悬空" in ln for ln in o),
              "⑩ 悬空登记：2 条例外件都不在扫面 ⇒ 只打印「悬空」· rc=0（同族口径 · 不判红）")

        # ⑪ 空转（扫面 0 件）⇒ rc=2
        r = os.path.join(t, "vacuous")
        os.makedirs(os.path.join(r, FACE_DEFAULT), exist_ok=True)
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 2 and d["verdict"] == "vacuous", "⑪ 空转：扫面 0 件 ⇒ rc=2（空转 = 假覆盖）")

        # ⑫ 缺件（扫面不在）⇒ rc=2
        r = os.path.join(t, "noface")
        os.makedirs(r, exist_ok=True)
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 2 and d["verdict"] == "missing-face", "⑫ 缺件：扫面不在 ⇒ rc=2（缺件 ≠ 绿）")

        # ⑬ 非仓 / `rev` 解不出 ⇒ rc=2（不给结论 · 不假装绿也不假装红）
        r = os.path.join(t, "nogit")
        os.makedirs(os.path.join(r, FACE_DEFAULT), exist_ok=True)
        open(os.path.join(r, FACE_DEFAULT, "A.zh-CN.md"), "w").write("# A\n")
        open(os.path.join(r, FACE_DEFAULT, "A.en.md"), "w").write("# A\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 2 and d["verdict"] in ("no-git", "bad-rev"),
              "⑬ 非仓 / `rev` 解不出 ⇒ rc=2（读不到就不给结论）")

        # ⑭ `J4` untracked 件**不在 diff 面** ⇒ 不判 ⇒ rc=0（多写手并行时别判成红）
        r = mk_fixture(os.path.join(t, "untracked"), dict(PAIR_A))
        open(os.path.join(r, FACE_DEFAULT, "D.zh-CN.md"), "w").write("# D 真源\n")
        open(os.path.join(r, FACE_DEFAULT, "D.en.md"), "w").write("# D derived\n")
        rc, o, d = judge(r, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)
        check(rc == 0 and "D.en.md" in d["not_in_diff"] and "D.zh-CN.md" in d["not_in_diff"],
              "⑭ `J4`：untracked 对**不在 diff 面** ⇒ 不判、rc=0（且在输出里逐条点名）")

        # ⑮ 逃生开关（`--waive`）端到端 ⇒ rc=2 被拒
        try:
            p = subprocess.run([sys.executable, os.path.abspath(__file__), "--waive"],
                               capture_output=True, text=True, timeout=180, cwd=REPO_ROOT)
            check(p.returncode == 2 and "拒" in (p.stdout + p.stderr),
                  "⑮ 逃生开关：`--waive` 端到端 ⇒ rc=2 被拒（例外只能显式登记）")
        except Exception as e:
            check(False, "⑮ 逃生开关：`--waive` 起不来：%s" % e)

        # ⑯ 区分度 + 三态齐：同一夹具「改一 ⇒ 1」「同批 ⇒ 0」「撤登记 ⇒ 2」
        r1 = mk_fixture(os.path.join(t, "disc-a"), dict(PAIR_A))
        touch(r1, "A.en.md", "single side\n")
        r2 = mk_fixture(os.path.join(t, "disc-b"), dict(PAIR_A))
        touch(r2, "A.en.md", "both\n")
        touch(r2, "A.zh-CN.md", "both\n")
        r3 = mk_fixture(os.path.join(t, "disc-c"), {"E.zh-CN.md": "# E\n"})
        got = {judge(r1, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)[0],
               judge(r2, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)[0],
               judge(r3, FACE_DEFAULT, "HEAD", SINGLE_LANG_EXCEPTIONS)[0]}
        check(got == {0, 1, 2},
              "⑯ 区分度 + 三态齐：同夹具能出 {0, 1, 2} 三条出口（判据不恒绿 / 不恒红）")
    finally:
        shutil.rmtree(t, ignore_errors=True)

    bad = [dsc for ok, dsc in checks if not ok]
    print("自检结论: %s（断言 **%d** 格 · 失败 %d 格）"
          % ("全过" if not bad else "**不过**：" + " · ".join(bad), len(checks), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print(VERSION)
    print("真源：任务清单-公开文档面收口-20260923.md:46（`T-13`）· 设计-公开文档面随版收口-v1.1-20260923.md:21/:385/:410")
    print("参照件：google/comprehensive-rust `.github/workflows/check-msgid-changes.py`（`E-2` · 已复核）")
    print("口径：**锚 = 真源侧**（`*.zh-CN.md` / `README.zh-CN.md`）· **正文 = 派生侧**（`*.en.md` / `README.md`）")
    print("`J0` 成对性 + 例外登记（前置 · 既不成对又未登记 ⇒ rc=2 · 逐条带理由 · 登记面 = `SINGLE_LANG_EXCEPTIONS`（`T-11b` 已正式登记））")
    print("`J1` 负控：**锚未变而正文变 = 红**（rc=1 · 逐对点名）")
    print("`J2` 正控：锚与正文**同批** ⇒ 绿")
    print("`J3` 附报（**不改退码**）：锚变而正文未变 ⇒ 「派生过期（stale）」（`T-11a` ② 面 · 只报不判）")
    print("`J4` 只在 tracked 的 diff 面里判（untracked 逐条打印「不在 diff 面 ⇒ 不判」）")
    print("退码：0 全绿 · 1 有漂移（锚未变而正文变）· 2 不给结论（缺件/空转/未登记/读不到/自检不过/拒收的开关）")
    print("拒收的逃生开关：%s（一律 rc=2）" % " / ".join(REFUSE_FLAGS))
    print("例外登记表（**正式登记** · `T-11b` 2026-09-24 接手落地 · 单一真源）：")
    for e in SINGLE_LANG_EXCEPTIONS:
        print("   · %s —— %s" % (e["file"], e["reason"]))
    print("扫面：%s（仓根相对 · `--face` 可换）· 只判 `*.md`" % FACE_DEFAULT)
    print("**不接闸**：未进 `precommit-gates.sh` · 未进 `scripts/公开标记.tsv` · 未进任何台账")
    return 0


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['rc', 'verdict', 'face', 'files', 'pairs', 'singles', 'unpaired', 'exceptions', 'dangling_exceptions', 'unregistered', 'rev', 'counts', 'drift', 'stale', 'both', 'not_in_diff']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
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
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(
        description="公开面漂移锚（`T-13`）：成对件「锚未变而正文变 = 红」（只报件 · 不改任何件）")
    ap.add_argument("--root", default=REPO_ROOT, help="判哪棵树（默认仓根）")
    ap.add_argument("--face", default=FACE_DEFAULT, help="扫面（仓根相对 · 默认 %s）" % FACE_DEFAULT)
    ap.add_argument("--rev", default="HEAD", help="diff 面的基线（默认 HEAD ⇒ 判「HEAD → 工作树」）")
    ap.add_argument("--json", default="", help="按名输出 JSON 字段（逗号分隔 · 空 = 全量）")
    ap.add_argument("--list-rules", action="store_true", help="只打印判据与口径")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成 git 夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检直接判真目标（调用方显式选择）")
    ap.add_argument(*REFUSE_FLAGS, dest="refuse", action="store_true",
                    help="（**故意不支持**）想把例外绕过去 ⇒ 一律拒 rc=2")
    a = ap.parse_args()

    if a.refuse:
        print("⛔ 不给结论（rc=2）：%s 一类「把例外绕过去」的开关**本质被拒** —— "
              "例外只能**显式登记**（本件 `SINGLE_LANG_EXCEPTIONS` · 逐条带理由）。"
              % " / ".join(REFUSE_FLAGS))
        return 2
    if a.list_rules:
        return list_rules()
    if a.self_test:
        print(VERSION)
        return self_test()

    print(VERSION)
    if not a.no_self_test:
        rc_st = self_test()
        if rc_st != 0:
            print("⛔ 自检不过 ⇒ 拒绝判真目标（门禁自己先能被证明「会红」）")
            return 2
    face_rel = a.face.rstrip("/")
    rc, lines, data = judge(os.path.abspath(a.root), face_rel, a.rev, SINGLE_LANG_EXCEPTIONS)
    for ln in lines:
        print(ln)
    if a.json:
        want = [w.strip() for w in a.json.split(",") if w.strip()]
        obj = data if not want else {k: data.get(k) for k in want}
        print(json.dumps(obj, ensure_ascii=False, sort_keys=True))
    return rc


if __name__ == "__main__":
    sys.exit(main())

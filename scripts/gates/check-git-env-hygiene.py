#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-git-env-hygiene.py —— 门禁：**hook / 门禁脚本里对外仓或合成仓的 git 调用先清环境**。

判据来源（逐条私有编号 ⇒ 本件**不进公开面**）
------------------------------------------------
  · 组4 §二.4 `W-43`（`研-禁:120` · `R-07`）逐字：「把『hook 里对**外仓 / 合成仓**的每一次 git
    调用都要先清环境』提成通用纪律 + 一条自检（正源 = `git rev-parse --local-env-vars` 的**全部**分量）」。
  · 官方原文（本机 `git help hooks` 现读，同一句）："If your hook needs to invoke Git commands in a
    foreign repository or in a different working tree of the same repository, then it should clear
    these environment variables so they do not interfere with Git operations at the foreign location."
    ＋官方给的形态：`foreign_desc=$(unset $(git rev-parse --local-env-vars); git -C ../foreign-repo describe)`
  · 纪律条文（成文那一半）= 同目录 `check-git-env-hygiene.md`（本件的说明书 · 与脚本同步改）。

纪律一句话
----------
  凡脚本对**外仓**或**自建合成仓**发 git 调用，调用前必须把 `git rev-parse --local-env-vars`
  列出的**每一个**变量从环境里清掉 —— 名单**现跑读**、**不许写死**（git 版本一变，判据跟着变）。

为什么要有它（本仓现成形态，不是假想）
--------------------------------------
  `scripts/gates/precommit-gates.sh` 的 ⑮′ 组会在**合成仓**里跑一串 git 调用，而它每次只清
  `GIT_INDEX_FILE` / `GIT_DIR` / `GIT_WORK_TREE` **三个** —— 正源现读是 **15 个**。少清的那些今天
  没炸，只是**没人看着**（`R-07` 逐字记的就是这一格：「手法在、纪律未成文」）。

判据（两条 · 都**不写死数字**）
-------------------------------
  ① **清面 ⊇ 全量**：每个站点的清除集合必须覆盖 `git rev-parse --local-env-vars` 的**全部**分量。
  ② **扫面非空**：站点 0 个 ⇒ rc=2（扫面空 = 假绿 —— 抽取器被改坏/形态变了，不许当绿）。

站点域（写死 · 逐条说明谁在管辖）
--------------------------------
  · `scripts/gates/**`（`.sh`）里的 `env -u <变量>… git …` 形态 —— 门禁脚本对外仓 / 合成仓的调用；
  · 同域的 `unset $(git rev-parse --local-env-vars)` 官方形态 —— **算全清**（它就是正源形态本身）。
  · **未判（点名登记，不判）**：`publish/mirror-public-lib.py` —— 它**故意**设 `GIT_DIR` /
    `GIT_WORK_TREE`（它就是那个外仓的驱动器），只清**泄漏进来的** `GIT_INDEX_FILE` ⇒ 「清面 ⊇ 全量」
    这条口径对它**不适用**（判它就是假红）。逐条理由写在下方 `NOT_JUDGED` 里，**不静默**。
  · **边界（写死）**：**裸** `git …`（无 `env` 前缀）不进判据 —— 「它是不是在动外仓 / 合成仓」是
    **语义**问题，静态判不出；本件只判**显式清环境**的那些站点（在**本仓**里跑的裸调用属正常形态）。

退码
----
  0 = 判到了（**报告档**：逐站点点名 + 清面计数 —— 命中不阻断，与 `check-registry-vs-disk.py` 同规）
  1 = 只有 `--strict` 档才可能：任一站点清面不全（逐站点点名）
  2 = **不给结论**：`git rev-parse --local-env-vars` 读不到 / 站点 0 个（空转 = 假绿）/ `--self-test` 不过

用法
----
    python3 scripts/gates/check-git-env-hygiene.py                # 查本仓（报告档）
    python3 scripts/gates/check-git-env-hygiene.py --strict       # 阻断档（任一站点不全 ⇒ rc=1）
    python3 scripts/gates/check-git-env-hygiene.py --json full,sites,gaps,not_judged
    python3 scripts/gates/check-git-env-hygiene.py --repo <仓根>
    python3 scripts/gates/check-git-env-hygiene.py --list-rules
    python3 scripts/gates/check-git-env-hygiene.py --self-test    # 成对自检（合成仓 · 不扫真目标）
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

VERSION = "1.0.0"

SELF_REL = "scripts/gates/check-git-env-hygiene.py"

# 站点域：门禁 / hook 侧的 shell 件（相对仓根）。
SITE_GLOBS = ("scripts/gates",)

# `env -u A -u B git …` ⇒ 清面 = 那串 `-u` 的操作数。
CLEAR_ENV_RE = re.compile(r"\benv((?:\s+-u\s+\S+)+)\s+git\b")
CLEAR_U_RE = re.compile(r"-u\s+(\S+)")
# 官方形态：`unset $(git rev-parse --local-env-vars); git …` ⇒ 算**全清**。
OFFICIAL_RE = re.compile(r"unset\s+\$\(\s*git\s+rev-parse\s+--local-env-vars\s*\)")

# 点名**未判**件（写死理由 · 不静默跳过）。
NOT_JUDGED = (
    ("publish/mirror-public-lib.py",
     "该件**故意**设 `GIT_DIR` / `GIT_WORK_TREE`（它就是那个外仓的驱动器），只清泄漏进来的 "
     "`GIT_INDEX_FILE`（现读 `env.pop(\"GIT_INDEX_FILE\", None)`）⇒「清面 ⊇ 全量」这条口径对它**不适用**，"
     "判它即假红；本件只**点名登记**，不改它（`publish/` 生效面另有限制）。"),
)


def full_env_vars(root: str):
    """`git rev-parse --local-env-vars` 现跑 ⇒ (名单, 理由)。正源**只此一处**，不写死名单。"""
    try:
        p = subprocess.run(["git", "-c", "core.quotepath=false", "rev-parse", "--local-env-vars"], cwd=root,
                           capture_output=True, text=True, stdin=subprocess.DEVNULL)
    except OSError as e:
        return None, "跑不动 git（%s）" % e
    if p.returncode != 0:
        return None, "`git rev-parse --local-env-vars` 退码 %d" % p.returncode
    names = [l.strip() for l in p.stdout.splitlines() if l.strip()]
    if not names:
        return None, "正源返回 0 个变量（空转 ⇒ 判不了）"
    return names, ""


def clears_all(cleared, full) -> bool:
    """判据①的本体：清面 ⊇ 全量（`full` 由现跑给 · `cleared` 为 None = 官方全清形态 ⇒ 视为全清）。"""
    if cleared is None:
        return True
    return set(full).issubset(set(cleared))


def scan_shell_sites(root: str):
    """扫站点 ⇒ [(相对路径, 行号, 形态, 清面或 None, 该行原文截断)]。只读。"""
    sites = []
    for sub in SITE_GLOBS:
        d = os.path.join(root, sub)
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            if not name.endswith(".sh"):
                continue
            path = os.path.join(d, name)
            rel = os.path.relpath(path, root)
            try:
                with open(path, encoding="utf-8") as f:
                    lines = f.read().splitlines()
            except OSError:
                continue
            for i, ln in enumerate(lines, 1):
                if OFFICIAL_RE.search(ln):
                    sites.append((rel, i, "官方形态(unset 全清)", None, ln.strip()[:120]))
                    continue
                m = CLEAR_ENV_RE.search(ln)
                if m:
                    cleared = CLEAR_U_RE.findall(m.group(1))
                    sites.append((rel, i, "env -u", cleared, ln.strip()[:120]))
    return sites


# ─────────────────────────── 自检（成对 · 合成仓） ───────────────────────────

def _git(cwd, args, env):
    p = subprocess.run(["git", "-c", "core.quotepath=false"] + list(args), cwd=cwd, env=env, capture_output=True, text=True)
    return p.returncode, p.stdout.strip(), p.stderr.strip()


def _fixture(base, name):
    """造一个合成仓（件 + 一次提交）⇒ 路径。提交身份走 `-c`，不依赖全局配置。"""
    d = os.path.join(base, name)
    os.makedirs(d)
    env = dict(os.environ)
    for v in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS"):
        env.pop(v, None)
    rc, _, err = _git(d, ["init", "-q", "."], env)
    if rc != 0:
        raise RuntimeError("合成仓建不起来：%s" % err)
    with open(os.path.join(d, "a.txt"), "w", encoding="utf-8") as f:
        f.write("a\n")
    _git(d, ["add", "--", "a.txt"], env)
    _git(d, ["-c", "user.email=selftest@zerg.local", "-c", "user.name=selftest",
             "commit", "-qm", "init"], env)
    return d


def _probe(cwd, env):
    """在合成仓里取**两条取证**：`--git-dir` + `--show-toplevel`（归一成绝对路径）。
    为什么两条：不同的泄漏变量显在不同的量上（`GIT_DIR` ⇒ git-dir 漂 · `GIT_WORK_TREE` ⇒ toplevel 漂）
    —— 只钉一条会漏另一个（本仓「两个不同真值不许并成一个形状」）。"""
    rc, out, _ = _git(cwd, ["rev-parse", "--git-dir", "--show-toplevel"], env)
    lines = [l.strip() for l in out.splitlines() if l.strip()]
    gitdir = os.path.realpath(os.path.join(cwd, lines[0])) if len(lines) > 0 else ""
    top = os.path.realpath(os.path.join(cwd, lines[1])) if len(lines) > 1 else ""
    return rc, gitdir, top


def self_test(out, full) -> bool:
    """成对自检：**正控**（全清 ⇒ 污染不起作用）· **负控**（留一个 ⇒ 判红）· 文本判据成对 · 抽取器。"""
    total, failed = 0, 0

    def case(name, want, got):
        nonlocal total, failed
        total += 1
        ok = want == got
        if not ok:
            failed += 1
        out.write("  %s %s（期望 %s · 实得 %s）\n" % ("✓" if ok else "✗", name, want, got))

    base = tempfile.mkdtemp(prefix="git-env-hygiene-")
    try:
        real = _fixture(base, "real")
        foreign = _fixture(base, "foreign")

        clean = dict(os.environ)
        for v in full:
            clean.pop(v, None)
        rc0, want_gitdir, want_top = _probe(real, clean)
        case("前置：干净环境下两条取证都 rc=0", 0, rc0)

        def polluted():
            e = dict(os.environ)
            e.update(GIT_DIR=os.path.join(foreign, ".git"),
                     GIT_WORK_TREE=foreign,
                     GIT_INDEX_FILE=os.path.join(foreign, ".git", "index"))
            return e

        def leaked(keep):
            """污染环境 + 清掉 `full` 里除 `keep` 之外的**全部**变量（None = 全清）。"""
            e = polluted()
            for v in full:
                if v != keep:
                    e.pop(v, None)
            return e

        def drifted(env):
            rc, gd, top = _probe(real, env)
            return rc != 0 or gd != want_gitdir or top != want_top

        # 前置：污染**真起作用**（否则下面的正控是假的 —— 没污染时清不清都一样）。
        case("前置：污染后不清理 ⇒ 取证漂到假仓", True, drifted(polluted()))

        # ① 正控：**清全部** ⇒ 在 real 里跑，取证到 real 自己。
        rc1, gd1, top1 = _probe(real, leaked(keep=None))
        case("① 正控：清全部 ⇒ rc=0", 0, rc1)
        case("① 正控：清全部 ⇒ git-dir / toplevel 都取证到 real 自己", True,
             gd1 == want_gitdir and top1 == want_top)

        # ② 负控：**留一个** ⇒ 判红（这一格就是判据「留一个 ⇒ 判红」的实体）。
        case("② 负控：留 GIT_DIR 一个 ⇒ 判红", True, drifted(leaked(keep="GIT_DIR")))
        case("② 负控：留 GIT_WORK_TREE 一个 ⇒ 判红", True, drifted(leaked(keep="GIT_WORK_TREE")))
        case("② 负控：判据本体吃得下这一格（clears_all 判 False）", False,
             clears_all([v for v in full if v != "GIT_DIR"], full))

        # ③ 文本判据成对：全清 ⇒ True · 少一个 ⇒ False · 官方形态（None）⇒ True。
        case("③ 判据①：清面 = 全量 ⇒ True", True, clears_all(list(full), full))
        case("③ 判据①：少一个 ⇒ False", False, clears_all([v for v in full if v != "GIT_INDEX_FILE"], full))
        case("③ 判据①：官方形态（unset 全清）⇒ True", True, clears_all(None, full))

        # ④ 抽取器：合成 sh 文本上抽得到两种形态，清面各按写下的数。
        sample = (
            "cd ${root} && env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git diff --name-only\n"
            "foreign_desc=$(unset $(git rev-parse --local-env-vars); git -C ../foreign describe)\n"
            "git status --porcelain   # 裸调用 · **不进**判据\n"
        )
        with open(os.path.join(base, "sample.sh"), "w", encoding="utf-8") as f:
            f.write(sample)
        hits = []
        for ln in sample.splitlines():
            if OFFICIAL_RE.search(ln):
                hits.append(None)
                continue
            m = CLEAR_ENV_RE.search(ln)
            if m:
                hits.append(CLEAR_U_RE.findall(m.group(1)))
        case("④ 抽取器：两种形态各抽一条（第三行裸调用不抽）", 2, len(hits))
        case("④ 抽取器：env 形态清 3 个 ⇒ 判不全（现读站点就是这一形态）", False,
             clears_all(hits[0], full))
        case("④ 抽取器：官方形态 ⇒ 判全", True, clears_all(hits[1], full))
    finally:
        shutil.rmtree(base, ignore_errors=True)
    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


LIST_RULES = """判据（`W-43` / `R-07` · 两条）
------------------------
  ① 清面 ⊇ 全量：站点清掉的集合必须覆盖 `git rev-parse --local-env-vars` 的**全部**分量（现跑读）
  ② 扫面非空：站点 0 个 ⇒ rc=2（空转 = 假绿）
站点域：`scripts/gates/**`（.sh）的 `env -u … git` 形态 + `unset $(git rev-parse --local-env-vars)` 官方形态
未判（点名登记）：publish/mirror-public-lib.py（故意设 GIT_DIR/GIT_WORK_TREE ⇒ 口径不适用）
边界：**裸** git 调用（无 `env` 前缀）不进判据（「在不在动外仓」是语义问题，静态判不出）
退码：0 判到（报告档）· 1 `--strict` 且任一站点不全 · 2 不给结论（正源读不到 / 站点 0 个 / 自检不过）
"""


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['schema', 'gate', 'version', 'full_env_vars', 'sites', 'gaps', 'gap_sites', 'not_judged', 'mode']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


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


def main() -> int:
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(description="hook / 门禁脚本对外仓·合成仓 git 调用的清环境自检（W-43 · R-07）")
    ap.add_argument("--repo", default=repo_default)
    ap.add_argument("--strict", action="store_true")
    ap.add_argument("--json", default=None)
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0

    full, why = full_env_vars(a.repo)
    if full is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % why)
        return 2

    if a.self_test:
        sys.stdout.write("── 自检（合成仓夹具 · 不扫真目标）──\n")
        sys.stdout.write("  正源：`git rev-parse --local-env-vars` 现读 %d 个变量\n" % len(full))
        return 0 if self_test(sys.stdout, full) else 2

    sites = scan_shell_sites(a.repo)
    sys.stdout.write("hook / 门禁脚本的 git 调用清环境自检（`W-43` · `R-07`）· v%s\n" % VERSION)
    sys.stdout.write("  正源：`git rev-parse --local-env-vars` 现读 **%d** 个变量\n" % len(full))
    if not sites:
        sys.stdout.write("⛔ 不给结论（rc=2）：站点 0 个（抽取器被改坏 / 形态变了 ⇒ 空转 = 假绿）\n")
        return 2
    gaps = []
    for rel, ln, style, cleared, raw in sites:
        ok = clears_all(cleared, full)
        have = len(full) if cleared is None else len(set(cleared) & set(full))
        if not ok:
            gaps.append((rel, ln, sorted(set(full) - set(cleared or []))))
        sys.stdout.write("  %s %s:%d  [%s] 清面 %d/%d\n" % ("✓" if ok else "✗", rel, ln, style, have, len(full)))
        sys.stdout.write("      %s\n" % raw)
    sys.stdout.write("  站点 **%d** 个 · 达标 %d · 未达标 %d\n" % (len(sites), len(sites) - len(gaps), len(gaps)))
    for rel, ln, miss in gaps:
        sys.stdout.write("  ✗ 未达标 %s:%d —— 未清的变量（%d 个）：%s\n" % (rel, ln, len(miss), " ".join(miss)))
    for rel, reason in NOT_JUDGED:
        sys.stdout.write("  ○ 未判（点名登记 · 不判）：%s —— %s\n" % (rel, reason))
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-git-env-hygiene", "version": VERSION,
            "full_env_vars": full, "sites": len(sites), "gaps": len(gaps),
            "gap_sites": [{"file": f, "line": l, "missing": m} for f, l, m in gaps],
            "not_judged": [r for r, _ in NOT_JUDGED], "mode": "strict" if a.strict else "report",
        }, ensure_ascii=False) + "\n")
    if gaps and a.strict:
        sys.stdout.write("⇒ 判红（rc=1 · `--strict`）：%d 个站点清面不全 ⇒ 少清的变量会泄漏进外仓/合成仓\n" % len(gaps))
        return 1
    if gaps:
        sys.stdout.write("⇒ 判到了（rc=0 · **报告档**）：%d 个站点清面不全 —— 不阻断；"
                         "要阻断用 `--strict`\n" % len(gaps))
        return 0
    sys.stdout.write("⇒ 全绿（rc=0）：%d 个站点清面都覆盖正源全量\n" % len(sites))
    return 0


if __name__ == "__main__":
    sys.exit(main())

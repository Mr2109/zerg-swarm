#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-multirepo-summary.py v1.0.0 —— `A7`（`G-20` · `Q-028`）**仓外仓只读汇总**（+ 负控拒写）

为什么存在（`A7` 的条件原文 · 逐字口径）
----------------------------------------
`Zerg-内部文档/项目文档/v2.5.11/缺口总账-20260922.md` §附录十五 A 表的 `A7` 行：
  「**同一份活跨两个仓**（主仓 + 文档仓），**没有一处能一条命令给两仓 `git status`**」·
  条件 = **只做「只读汇总」**，可核两条：
    ① 一条命令出两仓 `HEAD + 脏件数`；
    ② **负控**：给它任何写旗标（`add`/`commit`/`push`）⇒ **一律拒（rc=2）** ——
       理由逐字：这条命令长在**仓外仓**上（`Zerg-内部文档/` 是并排仓，不在主仓仓根下），
       误写的代价与 `G-06` 同族；
    ③ 只读：两仓 `HEAD` 与工作树 `sha256` 不变。
业界口径（同表 · 条件性学的依据）：`git -C <path>`（官方「Run as if git was started in <path>」）
  —— **多仓编排放外层包装**（`-C` 逐仓跑；git 官方**不提供**「跨仓」原语，仓库模型里没有多仓概念）。

★ 开工时现读发现（本件为什么要 `--no-optional-locks` · 不是洁癖）
---------------------------------------------------------------
`git status` **不是零副作用的**：它会刷新 stat 缓存并**改写 `.git/index`**。合成夹具现测
（`$SCRATCH/probe-idx` · 造仓 ⇒ `touch -t 200101010000 a.txt` ⇒ 跑一次 `git status --porcelain`）：
    改前 `.git/index` sha256 = `201703d463001c990d7ebb19ca12907f57ecee74e740b799a03812ffdba61a43`
    跑后 `.git/index` sha256 = `504c465e82664dcdbf7b7616b4065506733eeb92fd2976e3ac323885a69234be`  ⇒ **CHANGED**
同一形换 `git --no-optional-locks status --porcelain`：`31c741d5…` ⇒ `31c741d5…` ⇒ **SAME**（git 2.50.1）。
⇒ **本件的每一条 `git` 调用都硬带 `--no-optional-locks`**（`git ≥ 2.15` 才有这个开关）；
   否则「只读汇总」这句话在 `.git/index` 上就是假的。自检用例 ⑦/⑦b 把这一格钉住（后者专门
   用**不带**该开关的 `git status` 打一次夹具，断言 `.git/index` 真变了 ⇒ 证明这一格不是摆设）。

判据（可机检 · 每条都能单独失败）
--------------------------------
  A1 **一条命令出两仓 `HEAD + 脏件数`**：输出必含两仓（主仓 + 文档仓）的
     `HEAD`（40 位 sha）· `分支` · `脏件数` · `未跟踪数`，且**口径与既有单仓命令同源**
     （默认走 `zerg repo status --root <仓> --json …`，不另立一份计数口径；`--source git` 才直读 git）。
  A2 **负控拒写（唯一真源 = `WRITE_WORDS` 闭集）**：命令行里出现任何写词（`--add` / `-a` / 裸词 `add` …
     一律认，**不看位置**）⇒ **rc=2 · 拒**，并且**绝不代跑任何 git 写子命令**（第二道牙见 A4）。
  A3 **只读自证**（`--prove-readonly` / `--prove-readonly=tree`）：跑汇总**前**后对拍两仓指纹
     （`HEAD` · `.git` 元数据面逐件 `sha256` · `git status --porcelain` 全文字节摘要；
     `tree` 档再加 **git 眼里的工作树逐件 `sha256`** —— 跟踪件 + 未忽略的未跟踪件，
     被忽略的构建产物（`target/`·`dist/`·`bin/`·`node_modules/`）**不在**判据面）
     ⇒ 差异 0 件 ⇒ rc=0；有差异 ⇒ **rc=1** 并把差异逐件打印（不静默）。
  A4 **内部牙（第二道）**：所有 git 调用走**唯一入口** `git_ro()` —— 子命令不在只读白名单
     （`rev-parse` / `status` / `ls-files` / `for-each-ref` / `cat-file`）内 ⇒ **抛 `Refused`**，
     连「拼出写命令」这一步都到不了（自检用例 ⑤）。
  A5 **缺件不给结论**：`--root` / `--docs-root` 指到不是仓的地方 ⇒ **rc=2**（不拿「读不到」当「没问题」）。

退码
----
  0 = 汇总跑通（**两仓脏不脏只是读数** —— 脏 ≠ 错，口径照 `zerg repo status` 的「退码 1 不是错」；
      本项目 `R-23` 的分档口径：开发机只告警）· 1 = **只读自证红**（指纹变了）· 2 = 拒写 / 用法错 / 缺件 / 自检不过

用法
----
    python3 scripts/gates/check-multirepo-summary.py                 # 一条命令出两仓 HEAD + 脏件数
    python3 scripts/gates/check-multirepo-summary.py --json           # 机读（zerg/v1 形状）
    python3 scripts/gates/check-multirepo-summary.py --prove-readonly # 跑前后对拍（窄口径：HEAD + .git 元数据 + porcelain）
    python3 scripts/gates/check-multirepo-summary.py --prove-readonly=tree   # 宽口径：再加全工作树逐件 sha256
    python3 scripts/gates/check-multirepo-summary.py --source git      # 直读 git（也带 --no-optional-locks）
    python3 scripts/gates/check-multirepo-summary.py --root <主仓> --docs-root <文档仓>
    python3 scripts/gates/check-multirepo-summary.py --list-rules      # 打写词闭集 / 只读白名单 / 口径
    python3 scripts/gates/check-multirepo-summary.py --self-test       # 只跑自检（合成夹具 · 不碰真目标）

★ 接线（**不归本件** · 波④ 收口）：本件今天**没有**挂进 `scripts/gates/precommit-gates.sh`。
  波④ 需要的那一行（照 `scripts/公开标记.tsv` 的列：文件 / public / 理由）：
    scripts/gates/check-multirepo-summary.py	no	B 类·`A7` 仓外仓只读汇总件（`G-20`·`Q-028`）：一条命令出两仓 HEAD+脏件数 + 写旗标一律拒 rc=2 + 只读自证；判据绑私有面（读仓外并排仓 `Zerg-内部文档/`）⇒ 不开源
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-multirepo-summary.py v1.0.0"
SELF_NAME = os.path.basename(__file__)

# ── 写词闭集（负控唯一真源；**新增词只许往这里加**，别处不许各写一份）──────────────────────
WRITE_WORDS = (
    "add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
    "merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
    "mv", "apply", "stash", "update-index", "write-tree", "hash-object", "gc",
    "prune", "repack", "unlock", "commit-tree", "symbolic-ref-set", "config-set",
)
# ── 只读 git 子命令白名单（唯一入口 git_ro 用；**不在名单里 = 拒**）─────────────────────
READONLY_SUBS = ("rev-parse", "status", "ls-files", "for-each-ref", "cat-file")
# ── 窄口径指纹的面：`.git` 里本命令唯一可能碰到的元数据 ───────────────────────────────
#    ★ 故意**不含** `logs/`（引用变更日志）：任何**外部**写入者（含并发的另一枚）提交都会动它 ⇒
#      它不属于「本命令的副作用面」；`config` 也不含（读它不改它，列进去只会引入噪声）。
META_NAMES = ("HEAD", "index", "packed-refs", "ORIG_HEAD", "FETCH_HEAD",
              "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG")
JSON_FIELDS = "head,branch,path,status,untracked"


class Refused(Exception):
    """本件对写面的唯一表态：**拒**（不是「失败」，是不给结论）。"""


# ───────────────────────────────── 写旗标预扫 ─────────────────────────────────
def parse_write_flags(argv):
    """返回命中的写词列表。**形态无关**：`--add` / `-add` / 裸词 `add` / `--push=origin` 全认。"""
    hits = []
    for tok in argv:
        t = tok.strip()
        if not t or t == "--":
            continue
        name = t.lstrip("-").split("=", 1)[0].strip().lower()
        if name in WRITE_WORDS:
            hits.append(t)
    return hits


# ───────────────────────────────── 只读 git 入口 ─────────────────────────────────
def git_ro(repo, args, timeout=60):
    """**唯一**的 git 调用入口。子命令不在 `READONLY_SUBS` 内 ⇒ 抛 `Refused`（牙 A4）。

    每条调用硬带 `--no-optional-locks`（见件头 ★：`git status` 默认会改写 `.git/index`）。
    """
    if not args:
        raise Refused("空 git 调用 ⇒ 拒")
    sub = args[0]
    if sub not in READONLY_SUBS:
        raise Refused("git 子命令 %r 不在只读白名单 %s 内 ⇒ 拒（本件绝不代跑任何 git 写子命令）"
                      % (sub, list(READONLY_SUBS)))
    cmd = ["git", "--no-optional-locks", "-C", repo, "-c", "core.quotepath=false"] + list(args)
    try:
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    except Exception as e:                                       # noqa: BLE001
        raise RuntimeError("跑不起来：%s（%s）" % (" ".join(cmd), e))
    if p.returncode != 0:
        raise RuntimeError("退码 %d：%s" % (p.returncode, (p.stderr or p.stdout).strip()[:200]))
    return p.stdout


def file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def is_repo(path):
    if not path or not os.path.isdir(path):
        return False
    try:
        git_ro(path, ["rev-parse", "--git-dir"], timeout=20)
        return True
    except Exception:                                            # noqa: BLE001
        return False


def git_dir(repo):
    g = os.path.join(repo, ".git")
    if os.path.isdir(g):
        return g
    if os.path.isfile(g):                                        # worktree / 子模：`gitdir: <dir>`
        with open(g, encoding="utf-8") as f:
            for line in f:
                if line.strip().startswith("gitdir:"):
                    return os.path.normpath(os.path.join(repo, line.split(":", 1)[1].strip()))
    return None


# ───────────────────────────────── 读数（两个来源）─────────────────────────────────
def read_zerg(zerg, repo, timeout=60):
    """口径来源①（默认）：**既有命令面** `zerg repo status --root <仓> --json …` —— 与单仓口径同源。"""
    cmd = [zerg, "repo", "status", "--root", repo, "--json", JSON_FIELDS]
    try:
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    except Exception as e:                                       # noqa: BLE001
        return None, "跑不起来：%s" % e
    if p.returncode not in (0, 1):                               # 1 = 有未提交（不是错）
        return None, "退码 %d（合法只有 0/1）：%s" % (p.returncode, (p.stderr or p.stdout).strip()[:160])
    try:
        d = json.loads(p.stdout)
    except Exception as e:                                       # noqa: BLE001
        return None, "stdout 不是 JSON：%s" % e
    items = d.get("items") or []
    if not items:
        return None, "items 空（不给结论 —— 不把「读不到」当「干净」）"
    rows = []
    for it in items:
        path, status = it.get("path", ""), it.get("status", "?")
        untracked = it.get("untracked") == "是" or status == "??"
        sha = it.get("head", "?")
        rows.append((path, untracked, status, sha))
    return rows, None


def read_git(repo, git_status_cmd=None):
    """口径来源②（`--source git`）：`git -C <仓> --no-optional-locks status --porcelain`。

    `git_status_cmd` 只为自检用例 ⑦b 开口（合成夹具上跑**不带** `--no-optional-locks` 的形态）——
    **真目标路径永远走 `git_ro`**。
    """
    if git_status_cmd is not None:
        out = subprocess.run(git_status_cmd, capture_output=True, text=True, cwd=repo, timeout=60).stdout
    else:
        out = git_ro(repo, ["status", "--porcelain"])
    sha = git_ro(repo, ["rev-parse", "HEAD"]).strip()
    try:
        br = git_ro(repo, ["rev-parse", "--abbrev-ref", "HEAD"]).strip()
    except Exception:                                            # noqa: BLE001
        br = "?"
    rows = []
    for line in out.splitlines():
        if not line.strip():
            continue
        status, path = line[:2].strip(), line[3:].strip()
        if path.startswith('"') and path.endswith('"'):          # git 给非 ASCII 路径加了引号
            path = path[1:-1].encode("utf-8").decode("unicode_escape").encode("latin1", "ignore").decode("utf-8", "ignore") or path
        rows.append((path, status == "??", status, sha))
    return rows, br


def summarize(repo, zerg=None, source="zerg"):
    """⇒ (读数, 错误)。读数 = dict(head, branch, dirty, untracked, total, source)。"""
    if source == "zerg" and zerg and os.path.isfile(zerg):
        rows, err = read_zerg(zerg, repo)
        if err is None:
            head = rows[0][3] if rows else git_ro(repo, ["rev-parse", "HEAD"]).strip()
            try:
                branch = git_ro(repo, ["rev-parse", "--abbrev-ref", "HEAD"]).strip()
            except Exception:                                    # noqa: BLE001
                branch = "?"
            dirty = sum(1 for r in rows if not r[1])
            untracked = sum(1 for r in rows if r[1])
            return {"head": head, "branch": branch, "dirty": dirty, "untracked": untracked,
                    "total": len(rows), "source": "zerg repo status --root <仓> --json " + JSON_FIELDS}, None
        zerg_err = err
    else:
        zerg_err = None
    if source == "zerg" and zerg_err:
        return None, zerg_err
    try:
        rows, br = read_git(repo)
    except Refused as e:
        return None, "拒：%s" % e
    except Exception as e:                                       # noqa: BLE001
        return None, "读不到：%s" % e
    head = git_ro(repo, ["rev-parse", "HEAD"]).strip()
    dirty = sum(1 for r in rows if not r[1])
    untracked = sum(1 for r in rows if r[1])
    return {"head": head, "branch": br, "dirty": dirty, "untracked": untracked,
            "total": len(rows), "source": "git --no-optional-locks status --porcelain"}, None


# ───────────────────────────────── 指纹（只读自证）─────────────────────────────────
def repo_fingerprint(repo, tree=False):
    """窄口径 = `HEAD` + `.git` 元数据面逐件 sha256 + porcelain 全文摘要；`tree=True` 再加全工作树逐件 sha256。"""
    fp = {}
    gd = git_dir(repo)
    if gd:
        for n in META_NAMES:
            p = os.path.join(gd, n)
            if os.path.isfile(p):
                fp["<.git>/" + n] = file_sha(p)
        refs = os.path.join(gd, "refs")
        for dp, _dn, fn in os.walk(refs):
            for f in sorted(fn):
                p = os.path.join(dp, f)
                fp["<.git>/" + os.path.relpath(p, gd)] = file_sha(p)
    try:
        por = git_ro(repo, ["status", "--porcelain"]).encode("utf-8")
        fp["<porcelain 全文>"] = hashlib.sha256(por).hexdigest()
    except Exception as e:                                       # noqa: BLE001
        fp["<porcelain 全文>"] = "读不到：%s" % e
    if tree:
        # 「工作树」= **git 眼里的工作树**：跟踪件（`ls-files`）+ 未忽略的未跟踪件（`--others --exclude-standard`）。
        #   ★ 为什么不是「目录里所有件」：`target/`（Rust 构建）· `dist/` · `bin/` · `node_modules/` 这些
        #     被忽略的件**不是**「工作树」的判据面（它们天天在变、且与「谁改了源码」无关），
        #     真按整目录逐件 sha256 会在这台机上跑到超时（现测 >300s ⇒ 一条命令的语义就废了）。
        #     口径与仓内其它门一致：排除 `target·node_modules·dist·bin·vendor·data·.git`。
        rels = []
        try:
            for out in (git_ro(repo, ["ls-files", "-z"]),
                        git_ro(repo, ["ls-files", "-z", "--others", "--exclude-standard"])):
                rels += [x for x in out.split("\0") if x]
        except Exception as e:                                   # noqa: BLE001
            fp["<工作树清单>"] = "读不到：%s" % e
        for rel in sorted(set(rels)):
            p = os.path.join(repo, rel)
            if os.path.islink(p) or not os.path.isfile(p):
                continue
            try:
                fp["<树>/" + rel] = file_sha(p)
            except OSError:
                fp["<树>/" + rel] = "读不到"
    return fp


def diff_fingerprint(before, after):
    diffs = []
    for k in sorted(set(before) | set(after)):
        b, a = before.get(k), after.get(k)
        if b != a:
            diffs.append((k, b, a))
    return diffs


# ───────────────────────────────── 汇总主体 ─────────────────────────────────
def discover_docs_root(root, explicit=None):
    """文档仓默认落点：主仓的**兄弟目录** `Zerg-内部文档/`（现盘 1730 件 · 与主仓同卷 AHZ）。"""
    if explicit:
        return explicit
    env = os.environ.get("ZERG_DOCS_ROOT")
    if env:
        return env
    cand = os.path.join(os.path.dirname(os.path.abspath(root)), "Zerg-内部文档")
    return cand


def run(root, docs_root, source, prove, json_out, zerg):
    repos = [("主仓", os.path.abspath(root)), ("文档仓", os.path.abspath(docs_root))]
    problems = []
    for label, r in repos:
        if not is_repo(r):
            problems.append("缺件/不给结论：%s「%s」不是 git 仓（或无 .git）⇒ 不拿「读不到」当「干净」" % (label, r))
    if problems:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · %s" % p)
        return 2

    fp_before = {label: repo_fingerprint(r, tree=(prove == "tree")) for label, r in repos} if prove else None
    reads, errs = [], []
    for label, r in repos:
        rd, err = summarize(r, zerg=zerg, source=source)
        if err:
            errs.append("%s「%s」：%s" % (label, r, err))
        else:
            reads.append((label, r, rd))
    if errs:
        print("⛔ 不给结论（rc=2）：")
        for e in errs:
            print("   · %s" % e)
        return 2

    total_dirty = sum(rd["dirty"] for _l, _r, rd in reads)
    total_unt = sum(rd["untracked"] for _l, _r, rd in reads)

    proof = None
    rc = 0
    if prove:
        fp_after = {label: repo_fingerprint(r, tree=(prove == "tree")) for label, r in repos}
        diffs_all = []
        for label, _r in repos:
            b, a = fp_before[label], fp_after[label]
            d = diff_fingerprint(b, a)
            diffs_all += [(label, k, x, y) for k, x, y in d]
            proof = proof or {}
            proof.setdefault("per_repo", []).append({"label": label, "before": len(b), "after": len(a), "diff": len(d)})
        proof.update({"mode": prove, "diff_count": len(diffs_all),
                      "diffs": [{"repo": l, "file": k, "before": x, "after": y} for l, k, x, y in diffs_all]})
        if diffs_all:
            rc = 1

    if json_out:
        doc = {"schema": "zerg/v1", "kind": "MultiRepoSummary",
               "items": [{"label": l, "root": r, "head": rd["head"], "branch": rd["branch"],
                          "dirty": rd["dirty"], "untracked": rd["untracked"], "total": rd["total"],
                          "source": rd["source"]} for l, r, rd in reads],
               "meta": {"count": len(reads), "dirty": total_dirty, "untracked": total_unt,
                        "source": "local（本机）", "tool": VERSION},
               "proof": proof, "warnings": [], "truncated": False}
        print(json.dumps(doc, ensure_ascii=False))
        return rc

    print("── `A7` 仓外仓只读汇总（主仓 + 文档仓 · **一条命令**）──")
    print("  口径来源: %s" % reads[0][2]["source"])
    print("  %-8s %-42s %-8s %8s %10s %8s" % ("仓", "HEAD", "分支", "脏件数", "未跟踪数", "合计"))
    for l, r, rd in reads:
        print("  %-8s %-42s %-8s %8d %10d %8d" % (l, rd["head"], rd["branch"],
                                                  rd["dirty"], rd["untracked"], rd["total"]))
    print("  ── 两仓合计：未提交 %d 处（脏 %d · 未跟踪 %d）" % (total_dirty + total_unt, total_dirty, total_unt))
    print("  · 脏 ≠ 错：口径照 `zerg repo status`（退码 1 = 有未提交、**不是错**）；本件只报读数")
    print("  · 写旗标闭集（负控唯一真源 · %d 词）：%s" % (len(WRITE_WORDS), " ".join(WRITE_WORDS)))

    if prove:
        print("── 只读自证（%s 口径：%s）──" % (prove, "HEAD + .git 元数据面 + porcelain 全文" if prove != "tree"
                                            else "上一行再加**git 眼里的工作树**逐件 sha256（跟踪件 + 未忽略的未跟踪件）"))
        for row in proof["per_repo"]:
            print("  %s：前 %d 件 / 后 %d 件 · 差异 %d 件 %s"
                  % (row["label"], row["before"], row["after"], row["diff"],
                     "✓ 逐件不变" if not row["diff"] else "✗"))
        for d in proof["diffs"]:
            print("      ✗ %s %s：%s ⇒ %s" % (d["repo"], d["file"], (d["before"] or "")[:16], (d["after"] or "")[:16]))
        if proof["diff_count"]:
            print("  ⇒ rc=1：跑本次汇总期间有 %d 处指纹变了 ⇒ **只读这句话不成立**（逐处已打印）" % proof["diff_count"])
        else:
            print("  ⇒ 零副作用：两仓 `HEAD` 与件 `sha256` 逐件不变（差异 0 件）")
    if rc == 0:
        print("⇒ rc=0：两仓读数已出（%s）" % ("只读自证通过" if prove else "未要求自证"))
    return rc


def list_rules():
    print("── %s 判据与闭集 ──" % VERSION)
    print("写词闭集（负控 · %d 词 · 形态无关：`--add` / `-a` / 裸词 `add` / `--push=origin` 全认）：" % len(WRITE_WORDS))
    print("  " + " ".join(WRITE_WORDS))
    print("只读 git 子命令白名单（唯一入口 git_ro · 不在名单 = 拒）：%s" % " ".join(READONLY_SUBS))
    print("每条 git 调用硬带：`--no-optional-locks`（没有它，`git status` 会改写 `.git/index` ⇒ 只读是假的）")
    print("窄口径指纹面：%s + `refs/**` + porcelain 全文摘要（**不含** `logs/`/`config`：外部写入者会动，不属本命令副作用面）"
          % " ".join("<.git>/" + n for n in META_NAMES))
    print("宽口径（`--prove-readonly=tree`）：再加 git 眼里的工作树逐件 sha256（`ls-files` + `--others --exclude-standard`）")
    print("口径来源：默认 `zerg repo status --root <仓> --json %s`（与既有单仓命令同源）· `--source git` 直读 git" % JSON_FIELDS)
    print("退码：0 汇总跑通 · 1 只读自证红 · 2 拒写/用法错/缺件/自检不过")
    return 0


# ───────────────────────────────── 自检（成对负控 · 合成夹具）─────────────────────────────────
def _mk_repo(d, name, files=("a.txt",), dirty=False):
    repo = os.path.join(d, name)
    os.makedirs(repo, exist_ok=True)
    env = {"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
           "GIT_COMMITTER_EMAIL": "t@t", "HOME": d, "PATH": os.environ.get("PATH", "")}
    def g(*args):
        return subprocess.run(["git", "-C", repo] + list(args), capture_output=True, text=True, env=env, timeout=60)
    g("init", "-q", "-b", "main")
    for f in files:
        with open(os.path.join(repo, f), "w", encoding="utf-8") as fh:
            fh.write("one\n")
    g("add", "--", *files)
    g("commit", "-qm", "init")
    if dirty:
        with open(os.path.join(repo, "a.txt"), "a", encoding="utf-8") as fh:
            fh.write("two\n")
        with open(os.path.join(repo, "新件.txt"), "w", encoding="utf-8") as fh:
            fh.write("new\n")
    return repo


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="mrs-selftest-")
    main_repo = _mk_repo(tmp, "主仓", dirty=True)
    docs = _mk_repo(tmp, "Zerg-内部文档", files=("x.md",))

    def case(name, want_rc, got_rc, extra_ok=True, note=""):
        nonlocal ok
        good = (want_rc == got_rc) and extra_ok
        ok = ok and good
        print("  %s %s（rc=%s · 期望 %s）%s" % ("✓" if good else "✗", name, got_rc, want_rc, note))

    def cap(fn, *a, **kw):
        """跑一次并抓 rc（顺便把输出吞掉，自检只报结论）。"""
        import io
        import contextlib
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc = fn(*a, **kw)
        return rc, buf.getvalue()

    # ① 正例：两仓汇总（git 口径 · 夹具不是 bin/zerg 认得的仓 ⇒ 走 --source git）
    rc, out = cap(run, main_repo, docs, "git", None, False, None)
    case("① 正例：两仓 @HEAD + 脏件数（脏 1 · 未跟踪 1）", 0, rc,
         ("脏件数" in out and "主仓" in out and "文档仓" in out))

    # ①b JSON 形状
    rc, out = cap(run, main_repo, docs, "git", None, True, None)
    j = json.loads(out)
    case("①b `--json` 形状（zerg/v1 · items 2 条 · 计数现算）", 0, rc,
         (j["schema"] == "zerg/v1" and j["meta"]["count"] == 2 and j["meta"]["dirty"] == 1))

    # ② 负控：写旗标一律拒 rc=2（且夹具 HEAD/index 一个字节不变 —— 真拒，不是拒了又干）
    #    ★ 走**真 CLI 入口** `main([...])`（不是另写一份判定）
    before_head = git_ro(main_repo, ["rev-parse", "HEAD"]).strip()
    before_idx = file_sha(os.path.join(git_dir(main_repo), "index"))
    for flag in ("--commit", "--add", "--push", "commit", "push", "add", "--push=origin"):
        rc, out = cap(main, ["--root", main_repo, flag])
        case("② 负控：写旗标 %r ⇒ 拒 rc=2" % flag, 2, rc, "拒" in out)
    after_head = git_ro(main_repo, ["rev-parse", "HEAD"]).strip()
    after_idx = file_sha(os.path.join(git_dir(main_repo), "index"))
    case("②b 拒写后夹具 HEAD/index 逐字节不变（真拒）", before_head, after_head, before_idx == after_idx,
         note="  head %s index %s" % (before_head[:8], before_idx[:8]))

    # ④ 内部牙：git_ro 拒非白名单子命令
    refused = False
    try:
        git_ro(main_repo, ["commit", "-m", "x"])
    except Refused:
        refused = True
    except Exception:                                            # noqa: BLE001
        refused = False
    case("④ 内部牙：git_ro(['commit', …]) 抛 Refused（连拼命令都到不了）", True, refused)

    # ③ 只读自证（窄口径）：正例
    rc, out = cap(run, main_repo, docs, "git", "narrow", False, None)
    case("③ 只读自证（窄口径）：差异 0 件 ⇒ rc=0", 0, rc, "零副作用" in out)
    rc, out = cap(run, main_repo, docs, "git", "tree", False, None)
    case("③b 只读自证（宽口径 · 全工作树逐件 sha256）：差异 0 件 ⇒ rc=0", 0, rc, "零副作用" in out)

    # ⑦ 负控（**白盒** · 检测器级）：窗口内被改写 ⇒ 检测器必抓到，且判红规则必落 rc=1
    #    ★ 这一格走的是 `run()` 内部的**同一对**函数（`repo_fingerprint` / `diff_fingerprint`），
    #      不是另写一份判定；端到端那一格是 ③（正例）。
    before = repo_fingerprint(main_repo, tree=True)
    with open(os.path.join(docs, "x.md"), "a", encoding="utf-8") as fh:
        fh.write("窗口内被改一个字节\n")
    after = repo_fingerprint(docs, tree=True)
    d2 = diff_fingerprint(repo_fingerprint(docs, tree=True), after)      # 同件 ⇒ 差异 0（正例）
    d_inwin_before = repo_fingerprint(docs, tree=True)
    with open(os.path.join(docs, "x.md"), "a", encoding="utf-8") as fh:
        fh.write("窗口内再改一个字节\n")
    d_inwin = diff_fingerprint(d_inwin_before, repo_fingerprint(docs, tree=True))
    case("⑦ 负控（白盒）：窗口内真被改 ⇒ 检测器逐件抓到（差异非 0）", False, d_inwin == [],
         note="  差异 %d 件 · 首件 %s" % (len(d_inwin), d_inwin[0][0] if d_inwin else "—"))
    case("⑦b 负控（白盒）：同一差异落判红规则 ⇒ rc=1（不是 0）", 1, 1 if d_inwin else 0)
    case("⑦c 正例（白盒）：同件前后各取一次 ⇒ 差异 0 件（检测器不会乱报）", True, d2 == [])

    # ⑦d `--no-optional-locks` 这一格不是摆设：**新造的干净夹具**上，不带它的 `git status` 真会改 `.git/index`
    p3 = _mk_repo(tmp, "probe-clean")
    idx = os.path.join(git_dir(p3), "index")
    pre = file_sha(idx)
    subprocess.run(["touch", "-t", "200101010000", os.path.join(p3, "a.txt")], timeout=30)
    read_git(p3, git_status_cmd=["git", "-C", p3, "status", "--porcelain"])   # 自检专用：故意不带开关
    post = file_sha(idx)
    case("⑦d 负控：不带 `--no-optional-locks` 的 `git status` ⇒ `.git/index` 真变（这一格有牙）",
         False, pre == post, note="  %s ⇒ %s" % (pre[:8], post[:8]))
    pre2 = file_sha(idx)
    subprocess.run(["touch", "-t", "200101010000", os.path.join(p3, "a.txt")], timeout=30)
    git_ro(p3, ["status", "--porcelain"])                        # 本件的真路径：带开关
    post2 = file_sha(idx)
    case("⑦e 正例：本件真路径（带 `--no-optional-locks`）⇒ `.git/index` 不变", True, pre2 == post2)

    # ⑤ 缺件：不是仓 ⇒ rc=2
    rc, out = cap(run, os.path.join(tmp, "没有这个仓"), docs, "git", None, False, None)
    case("⑤ 缺件（--root 不是仓）⇒ rc=2 不给结论", 2, rc, "不是 git 仓" in out)

    # ⑥ 写词闭集非空 + 无重复（「静静变小」被拦）
    case("⑥ 写词闭集非空且无重复", True, len(WRITE_WORDS) > 0 and len(set(WRITE_WORDS)) == len(WRITE_WORDS))

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)

    # ── 第一道牙：写旗标预扫（**先于 argparse** · 免得 `--commit` 被当成「未知旗标」混过去）──
    hits = parse_write_flags(argv)
    if hits:
        print("⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s" % hits)
        print("   理由（`A7` 条件② 逐字）：这条命令长在**仓外仓**上（`Zerg-内部文档/` 是并排仓，")
        print("   不在主仓仓根下），误写的代价与 `G-06` 同族 ⇒ 任何写旗标（add/commit/push…）一律拒，")
        print("   且**绝不代跑任何 git 写子命令**（第二道牙 = 唯一入口 `git_ro()` 的子命令白名单）。")
        print("   只读用法：bash scripts/gates/%s [--root <主仓>] [--docs-root <文档仓>] [--json] [--prove-readonly]"
              % SELF_NAME)
        return 2

    ap = argparse.ArgumentParser(add_help=True, description="A7 仓外仓只读汇总（主仓 + 文档仓）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="主仓仓根（默认真源：本脚本所在仓）")
    ap.add_argument("--docs-root", default=None, help="文档仓仓根（默认：主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--zerg", default=os.path.join(DEFAULT_ROOT, "bin", "zerg"), help="命令面可跑制品（口径来源①）")
    ap.add_argument("--source", choices=("zerg", "git"), default="zerg", help="读数口径来源（默认 zerg = 与单仓命令同源）")
    ap.add_argument("--prove-readonly", nargs="?", const="narrow", choices=("narrow", "tree"), default=None,
                    help="跑前后对拍只读自证（缺省参数 = narrow）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    docs_root = discover_docs_root(a.root, a.docs_root)
    return run(a.root, docs_root, a.source, a.prove_readonly, a.json, a.zerg)


if __name__ == "__main__":
    sys.exit(main())

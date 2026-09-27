#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-zero-side-effect.py v1.0.0 —— `C4`（`Q-009`）**零副作用证明**（只对「只读 / 已封」件）

为什么存在（`C4` 的条件原文 · 逐字口径）
----------------------------------------
`Zerg-内部文档/项目文档/v2.5.11/缺口总账-20260922.md`：
  · 行 68：`Q-009` | `C4` | 「**同一判据跑两遍、逐字对拍**（防假绿）」—— 今天手写
    `diff /tmp/st-before.txt /tmp/st-after.txt && echo "git status: 逐字不变"`。
  · §附录十五 A 表：`C4` 的条件 = 「**只对只读/已封件出零副作用证明**」· 括号里逐字：「**对象可限定**
    ✓（**可变件只报 rc=8** ✓）」。
本件就是那一格：给「本次运行**没有副作用**」出一份**带基准的证明**，而不是一句「我看着没变」。

判据（可机检 · 每条都能单独失败）
--------------------------------
  C1 **被证明对象自带可比基准**（只读 / 已封件）⇒ 出零副作用证明 **rc=0**。
     基准三来源（**优先级从高到低** · 逐条打印用的是哪一条）：
       ① `manifest` —— 清单件里登记了该件的 `sha256`（**强**：跨机、跨仓、跨时刻可核）；
       ② `head-blob` —— 件在 `HEAD` 树里且工作树现算与 blob **逐字相同**（**强**：仓自己的对象库）；
       ③ `readonly-bit` —— 件 mode **无 owner 写位**（**弱**：它证的是「写不进去」，不是「没被写过」——
          属 `A8` 那个卡点「今天 38/38 只是 mode 位 ✗ owner 能 `chmod` 回来」，本件照实标注强度）。
     证明行 = 件 · 类别 · 基准来源 + 强度 · 基准值 · 现算值 · 相等? · 时刻。
  C2 **对可变件（工作树）不许假装证明** ⇒ 只报「**无可比基准**」+ **rc=8**（逐条给「为什么没基准」：
     工作树 dirty / 未跟踪 / 有写位且与 `HEAD` 不等 / 不在任何清单里）。
  C3 **负控 · 给可变件伪造基准 ⇒ 判红**：`--assume <件>=<sha256>` 一旦落在**可变件**上 ⇒ **rc=1**
     （不许用「我抄来的一个值」把可变件洗成已证）。同一开关落在只读/已封件上时**不是**伪造：
     值相等 ⇒ 记「复核一致」（不红）；值不等 ⇒ rc=1（基准不符 ⇒ 该件**已被改**）。

退码（**优先级 1 > 8 > 0**，逐档都明写在输出里）
------------------------------------------------
  0 = 对象集全部出了证明（每一件都有自带可比基准且相符）
  1 = 红（基准不符 / 给可变件伪造基准 / 源码级写 API 命中）
  2 = 不给结论（对象缺件 / 清单件缺或解析不出 / 自检不过 / 写旗标）
  8 = **有可变件**（无可比基准 ⇒ 不假装：可变件那几条只报「无可比基准」）★ 这条是本件的重点

口径说明（为什么要按「自带基准」分类，而不是按「这份活该不该动」）
--------------------------------------------------------------
`C4` 要防的是**假绿**：判据跑两遍对了就对拍，但对**本来就会变的件**（工作树里正被改的件）
「两遍一样」永远成立（或者永远不成立）—— 那种对拍**证不出零副作用**。所以本件把对象分成两档：
「**可比**（有基线）」才出证明，「**不可比**」就明说 rc=8，一个字节都不假装。

用法
----
    python3 scripts/gates/check-zero-side-effect.py                 # 真目标：归档区只读件（现读 · 清单位自带基准）
    python3 scripts/gates/check-zero-side-effect.py --sealed <件>…   # 只证这些件（可混仓内已封件）
    python3 scripts/gates/check-zero-side-effect.py --workspace      # 判工作树里的可变件（现读 git status）⇒ rc=8
    python3 scripts/gates/check-zero-side-effect.py --archive /…/Zerg-归档   # 换归档区落点
    python3 scripts/gates/check-zero-side-effect.py --manifest <清单件>      # 自带基准来源（sha256␣路径 或 归档记录表格）
    python3 scripts/gates/check-zero-side-effect.py --assume <件>=<sha256>   # 负控：伪造基准（给可变件 ⇒ 判红）
    python3 scripts/gates/check-zero-side-effect.py --json / --prove-readonly / --list-rules / --self-test

★ 接线（**不归本件** · 波④ 收口）：本件今天**没有**挂进 `scripts/gates/precommit-gates.sh`。
  ★★ **接线时注意退码**：本件会出 **rc=8**（可变件 · 不给结论档），这与仓里门脚本惯用的 0/1/2 **不同** ——
     `precommit-gates.sh` 的 `tri` 档把 8 当**异常码 ⇒ 计 FAIL**（照它的口径「非 0/1/2 的异常码一律按 FAIL」），
     所以波④ 要么给这一步挑一个**认 8 的档**（例如把它包一层 shell：`rc=$?; [ "$rc" = 8 ] && exit 8`…），
     要么只把它跑成 `--sealed` 那一档（只读/已封件 ⇒ 只出 0/1/2）。**别直接挂 `tri` 了事。**
  波④ 需要的那一行（照 `scripts/公开标记.tsv` 的列：文件 / public / 理由）：
    scripts/gates/check-zero-side-effect.py	no	B 类·`C4` 零副作用证明件（`Q-009`）：只对只读/已封件出带基准的证明（清单·HEAD blob·只读位）+ 可变件只报「无可比基准 rc=8」+ 给可变件伪造基准判红；判据绑私有面（读仓外归档区与其归档记录 · `bin/zerg`）⇒ 不开源
"""

import argparse
import ast
import hashlib
import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
SIBLING_BASE = os.path.dirname(DEFAULT_ROOT)                          # …/模型类
DEFAULT_ARCHIVE = os.path.join(SIBLING_BASE, "Zerg-归档")
DEFAULT_SEALED = ("deploy/服务声明.tsv", "scripts/公开标记.tsv")
VERSION = "check-zero-side-effect.py v1.1.0"
SELF_NAME = os.path.basename(__file__)

RC_OK, RC_RED, RC_NOCONC, RC_NOBASE = 0, 1, 2, 8

WRITE_WORDS = ("add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
               "merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
               "mv", "apply", "stash", "gc", "prune", "repack", "chmod", "chown", "touch",
               "seal", "seal-run", "fix", "write", "delete", "del", "unlink", "reap")
DELETE_CALLS = ("remove", "unlink", "rmtree", "rmdir", "rename", "move", "truncate", "replace")
GIT_WRITE_SUBS = ("add", "commit", "push", "rm", "mv", "checkout", "restore", "reset", "clean",
                  "update-index", "write-tree", "stash", "rebase", "fetch", "pull", "tag")
# 源码级牙的扫描边界（拼接而成：别写成字面量，否则 split 会在这一行就切掉 —— 自检用例 ⑩b/⑩c 钉住）
SELFTEST_MARKER = "# ─────────────────────────── " + "自检"
SHA256_RE = re.compile(r"[0-9a-f]{64}")


# ─────────────────────────────── 基础 ───────────────────────────────
def file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:                                  # 只读 'rb'（不是写）
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def is_readonly_bit(path):
    """owner 写位清了 ⇒ 只读位（**弱基准**：`A8` 那个卡点说的就是它）。"""
    try:
        return not bool(os.stat(path).st_mode & stat.S_IWUSR)
    except OSError:
        return False


def git_ro(repo, args, timeout=60):
    allow = ("rev-parse", "status", "ls-files", "cat-file", "hash-object")
    if not args or args[0] not in allow:
        raise RuntimeError("git 子命令 %r 不在只读白名单 %s 内 ⇒ 拒" % (args[0] if args else None, list(allow)))
    if args[0] == "hash-object" and "-w" in args:                # `hash-object -w` 会写对象库 ⇒ 拒
        raise RuntimeError("`hash-object -w` 会写对象库 ⇒ 拒")
    p = subprocess.run(["git", "--no-optional-locks", "-C", repo, "-c", "core.quotepath=false"] + list(args),
                       capture_output=True, text=True, timeout=timeout)
    if p.returncode != 0:
        raise RuntimeError("退码 %d：%s" % (p.returncode, (p.stderr or "").strip()[:160]))
    return p.stdout


def repo_root_of(path):
    """往上找 git 仓根（判「已封」用）。⇒ 仓根 或 None。"""
    d = os.path.dirname(os.path.abspath(path))
    for _ in range(12):
        if os.path.isdir(os.path.join(d, ".git")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            return None
        d = parent
    return None


def head_blob_sha(path):
    """件在 `HEAD` 树里 ⇒ (blob sha256, 仓根)；否则 (None, 仓根或 None)。"""
    root = repo_root_of(path)
    if not root:
        return None, None
    rel = os.path.relpath(os.path.abspath(path), root)
    try:
        out = git_ro(root, ["cat-file", "-p", "HEAD:" + rel])
    except Exception:                                            # noqa: BLE001
        return None, root
    return hashlib.sha256(out.encode("utf-8")).hexdigest(), root


# ─────────────────────────── 清单（自带基准来源）───────────────────────────
def parse_manifest(path, archive_root=None):
    """解析自带基准清单 ⇒ (dict{相对路径: sha256}, problems)。

    认两种形态（都是仓内既有件的样子，**不新造一种**）：
      ① `sha256␣路径`（逐行 · 与 `shasum -a 256` 输出同形）；
      ② 归档记录的 markdown 表格行：含一个整 64 位十六进制 token 与一个 `Zerg-归档/…` 路径 token。
    """
    problems, table = [], {}
    if not os.path.isfile(path):
        return None, ["清单件缺：%s（自带基准取不到 ⇒ 不给结论）" % path]
    with open(path, encoding="utf-8", errors="replace") as f:
        for ln, line in enumerate(f, 1):
            line = line.rstrip("\n")
            if not line.strip():
                continue
            # ① sha256 路径
            m = re.match(r"^\s*([0-9a-f]{64})[\s]+(.+?)\s*$", line)
            if m:
                table[m.group(2).strip()] = m.group(1)
                continue
            # ② 归档记录表格行
            if line.lstrip().startswith("|"):
                toks = re.findall(r"`([^`]+)`", line)
                sha = next((t for t in toks if SHA256_RE.fullmatch(t.strip())), None)
                arc = next((t for t in toks if "Zerg-归档/" in t), None)
                if sha and arc:
                    key = arc.split("Zerg-归档/", 1)[1]
                    table[key] = sha.strip()
                continue
        # 空表不给结论：清单在、但一行都没解析出来 ⇒ 判据不可判
    if not table:
        return None, ["清单件解析不出任何一行（%s）⇒ 不给结论（不把「解析不出」读成「没问题」）" % path]
    if archive_root:
        # 允许清单里的路径直接落在归档区（相对根）—— 两种键都收
        for k in list(table):
            if not k.startswith("Zerg-归档/"):
                table.setdefault("Zerg-归档/" + k, table[k])
    return table, problems


def manifest_of(path, archive_root, table):
    """该件在清单里的登记值（键按「归档区相对路径」与「全路径」两种形态找）。⇒ (sha, key)。"""
    cands = []
    ap = os.path.abspath(path)
    if archive_root and ap.startswith(os.path.abspath(archive_root) + os.sep):
        rel = os.path.relpath(ap, os.path.abspath(archive_root))
        cands += [rel, "Zerg-归档/" + rel, os.path.basename(os.path.dirname(os.path.abspath(archive_root))) + "/" + rel]
    cands.append(ap)
    for c in cands:
        if c in table:
            return table[c], c
    return None, None


# ─────────────────────────── 判定 + 出证明 ───────────────────────────
def _upd(row, **kw):
    """把 `class_` 这个开关名映回 `class`（`class` 是 Python 关键字，不能当形参名）。"""
    row.update({("class" if k == "class_" else k): v for k, v in kw.items()})
    return row


def classify_and_prove(path, archive_root, table, assume=None):
    """⇒ dict（每条一个·见下）。**只读**：只 stat / open('rb') / git cat-file。"""
    row = {"path": os.path.abspath(path), "exists": os.path.isfile(path), "class": None,
           "baseline_from": None, "strength": None, "baseline": None, "actual": None,
           "match": None, "note": "", "rc": RC_OK}
    if not row["exists"]:
        _upd(row, rc=RC_NOCONC, class_="缺件", note="件不在盘（不给结论）")
        row["class"] = "缺件"
        return row
    row["actual"] = file_sha(path)
    ro = is_readonly_bit(path)
    blob, root = head_blob_sha(path)
    msha, mkey = manifest_of(path, archive_root, table or {})

    # 基准来源（优先级：清单 > HEAD blob（且逐字相同）> 只读位）
    if msha:
        _upd(row, class_="已封（清单登记）", baseline_from="manifest:" + str(mkey), strength="强（清单）",
                   baseline=msha)
    elif blob and blob == row["actual"]:
        _upd(row, class_="已封（HEAD blob）", baseline_from="head-blob:%s" % os.path.relpath(path, root),
                   strength="强（HEAD blob）", baseline=blob)
    elif ro:
        _upd(row, class_="只读（mode 位）", baseline_from="readonly-bit", strength="弱（只读位 · A8 卡点）",
                   baseline=None)
    elif blob is not None:
        _upd(row, class_="可变（工作树 · 与 HEAD 不等）", baseline=None,
                   note="dirty：工作树现算 %s… ≠ HEAD blob %s… ⇒ **无可比基准**（不许假装证明）"
                        % (row["actual"][:12], blob[:12]), rc=RC_NOBASE)
        if assume:
            _upd(row, class_="可变（工作树）", note=row["note"] + " · 且调用方还给了一个基准 %s… ⇒ **伪造基准 ⇒ 判红**"
                       % assume[:12], rc=RC_RED)
        return row
    else:
        _upd(row, class_="可变（工作树 · 不在任何仓 / 未跟踪）", baseline=None,
                   note="未跟踪或不在仓里 ⇒ **无可比基准**（不许假装证明）", rc=RC_NOBASE)
        if assume:
            _upd(row, note=row["note"] + " · 且调用方还给了一个基准 %s… ⇒ **伪造基准 ⇒ 判红**" % assume[:12],
                       rc=RC_RED)
        return row

    # 有基准 ⇒ 出证明（或红）
    if row["baseline"] is None or assume:
        ref = row["baseline"] if row["baseline"] is not None else assume
        if assume and row["baseline"] and assume != row["baseline"]:
            _upd(row, rc=RC_RED, note="调用方给的基准 %s… ≠ 自带基准 %s…（只读/已封件上给错值 ⇒ 判红）"
                       % (assume[:12], row["baseline"][:12]))
            row["baseline"] = assume
            row["match"] = False
            return row
        if assume:
            row["note"] = "复核一致（调用方给的基准与前两条来源逐字相同）"
            row["baseline"] = assume
        else:
            row["note"] = "自带基准只到「不可写」这一层（弱）：任何写都会失败 ⇒ 零副作用成立"
        row["match"] = True
        row["rc"] = RC_OK
        return row
    row["match"] = (row["actual"] == row["baseline"])
    if not row["match"]:
        _upd(row, rc=RC_RED, note="**基准不符**：现算 %s… ≠ 基准 %s… ⇒ 该件已被改（或基准过期）"
                   % (row["actual"][:12], row["baseline"][:12]))
    else:
        row["note"] = "现算 == 基准（逐字）⇒ 本次运行对该件零副作用"
    return row


# 归档区**自述元文件** —— 显式**不属**"被证明对象"（对象集 = 归档记录里登记的正文件）。
# 理由：它们是镜像自述件（README / 版次索引 / 归档记录），随镜像只读、不进归档件表
#       ⇒ manifest / head-blob / readonly-bit 三条基准来源都取不到 ✗。
# 处置：在此**登记并逐条打印**，**不静默过滤** ✗（`Q-117` · Mr2109 2026-09-23 拍「甲」）。
ARCHIVE_META_EXACT = ("README.md", "版次索引.md")
ARCHIVE_META_PREFIX = ("归档记录-",)


def is_archive_meta(path):
    """按件名判：归档区自述元文件（README.md / 版次索引.md / 归档记录-*.md）。"""
    name = os.path.basename(path)
    return name in ARCHIVE_META_EXACT or any(name.startswith(p) for p in ARCHIVE_META_PREFIX)


def collect_default(archive_root, sealed_in, workspace, root):
    """默认真目标集：归档区只读件（现读）+ 点名的仓内已封件；`--workspace` 换成现读工作树可变件。"""
    paths, why = [], []
    if workspace:
        try:
            out = git_ro(root, ["status", "--porcelain"])
        except Exception as e:                                   # noqa: BLE001
            return [], ["工作树读数取不到：%s（不给结论）" % e]
        for line in out.splitlines():
            if len(line) > 3:
                p = line[3:].strip()
                if p.endswith("/"):
                    continue
                paths.append(os.path.join(root, p))
        why.append("工作树（现读 `git status --porcelain`）")
        return paths, why
    if sealed_in:
        return [os.path.abspath(p) if not os.path.isabs(p) else p for p in sealed_in], ["调用方点名（--sealed）"]
    if os.path.isdir(archive_root):
        metas = []
        for dp, _dn, fn in os.walk(archive_root):
            for f in sorted(fn):
                p = os.path.join(dp, f)
                if is_archive_meta(p):
                    metas.append(os.path.relpath(p, archive_root))
                    continue
                paths.append(p)
        why.append("归档区（现读 %s）" % archive_root)
        if metas:
            why.append("元文件 %d 枚**不属对象集**（%s）—— 它们是归档区自述件（随镜像只读 · 不进件表 ⇒ 无自带基准）"
                       % (len(metas), " · ".join(sorted(metas))))
    if not paths:
        return [], ["归档区不存在（%s）⇒ 不给结论（不把「读不到」当「没问题」）" % archive_root]
    for rel in DEFAULT_SEALED:
        p = os.path.join(root, rel)
        if os.path.isfile(p):
            paths.append(p)
            why.append("仓内点名件 %s" % rel)
    return paths, why


def run(archive_root, manifest_path, assume_map, sealed_in, workspace, root, json_out):
    table = None
    if manifest_path:
        table, mp = parse_manifest(manifest_path, archive_root=archive_root)
        if table is None:
            print("⛔ 不给结论（rc=2）：")
            for p in mp:
                print("   · %s" % p)
            return RC_NOCONC
    elif os.path.isdir(archive_root):
        # 自动发现同族的归档记录件（自带基准就在那里面 —— 不新造清单）
        for f in sorted(os.listdir(archive_root)):
            d = os.path.join(archive_root, f)
            if not os.path.isdir(d):
                continue
            for g in sorted(os.listdir(d)):
                if g.startswith("归档记录-") and g.endswith(".md"):
                    table, _mp = parse_manifest(os.path.join(d, g), archive_root=archive_root)
                    if table:
                        manifest_path = os.path.join(d, g)
                        break
            if table:
                break

    paths, why = collect_default(archive_root, sealed_in, workspace, root)
    if not paths:
        print("⛔ 不给结论（rc=2）：")
        for w in why:
            print("   · %s" % w)
        return RC_NOCONC

    now = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    rows = [classify_and_prove(p, archive_root, table, assume=assume_map.get(os.path.abspath(p)) or assume_map.get(p))
            for p in paths]
    proved = [r for r in rows if r["rc"] == RC_OK and r["baseline"] is not None]
    weak = [r for r in rows if r["rc"] == RC_OK and r["baseline"] is None]
    nobase = [r for r in rows if r["rc"] == RC_NOBASE]
    red = [r for r in rows if r["rc"] == RC_RED]
    missing = [r for r in rows if r["rc"] == RC_NOCONC]

    if json_out:
        print(json.dumps({"schema": "zerg/v1", "kind": "ZeroSideEffectProof",
                          "items": [{k: r[k] for k in ("path", "class", "baseline_from", "strength",
                                                       "baseline", "actual", "match", "note")} | {"rc": r["rc"]}
                                    for r in rows],
                          "meta": {"count": len(rows), "proved": len(proved), "weak": len(weak),
                                   "no_baseline": len(nobase), "red": len(red), "missing": len(missing),
                                   "at": now, "archive": archive_root, "manifest": manifest_path, "tool": VERSION},
                          "warnings": [], "truncated": False}, ensure_ascii=False))
    else:
        print("── `C4` 零副作用证明（**只对只读/已封件出证明** · 可变件只报「无可比基准」）──")
        print("  时刻: %s · 对象集来源: %s · 自带基准清单: %s"
              % (now, " + ".join(why), manifest_path or "（无）"))
        for r in rows:
            if r["rc"] == RC_OK:
                base = r["baseline"] if r["baseline"] else "（无值 · 只到不可写这一层）"
                print("  ✓ 证明 %s" % r["path"])
                print("      类别=%s · 基准来源=%s · 强度=%s" % (r["class"], r["baseline_from"], r["strength"]))
                print("      基准=%s · 现算=%s · 相符=%s" % (str(base)[:16], r["actual"][:16], r["match"]))
                print("      %s" % r["note"])
            elif r["rc"] == RC_NOBASE:
                print("  ⊘ 无可比基准（rc=8）%s" % r["path"])
                print("      类别=%s · %s" % (r["class"], r["note"]))
            elif r["rc"] == RC_RED:
                print("  ✗ 判红（rc=1）%s" % r["path"])
                print("      %s" % r["note"])
            else:
                print("  ⛔ 不给结论 %s：%s" % (r["path"], r["note"]))
        print("── 计数：对象 %d（出证明 %d · 弱基准 %d · **无可比基准 %d** · 判红 %d · 缺件 %d）"
              % (len(rows), len(proved), len(weak), len(nobase), len(red), len(missing)))
        print("   ★ 出具证明的对象 = 只读/已封件（自带可比基准）；**可变件一个都不假装**")

    if red:
        print("⇒ rc=1：有 %d 条判红（基准不符 / 给可变件伪造基准 · 逐条见上）" % len(red))
        return RC_RED
    if missing:
        print("⇒ rc=2：有 %d 条件就不在盘（不给结论）" % len(missing))
        return RC_NOCONC
    if nobase:
        print("⇒ rc=8：有 %d 条**可变件（工作树）无可比基准** ⇒ 不许假装证明（逐条已列）" % len(nobase))
        return RC_NOBASE
    return RC_OK


def list_rules():
    print("── %s 判据 ──" % VERSION)
    print("C1 只读/已封件 ⇒ 出证明 rc=0；基准三来源（优先级）：① 清单登记（强）② `HEAD` blob（强，且工作树逐字相同）"
          "③ 只读位（弱 · `A8` 卡点）")
    print("C2 可变件（工作树 dirty / 未跟踪）⇒ 只报「无可比基准」· **rc=8**")
    print("C3 负控：`--assume <件>=<sha>` 落在可变件上 ⇒ **rc=1 判红**；落在只读/已封件上：值等 ⇒ 记「复核一致」· 值不等 ⇒ 判红")
    print("退码优先级：1（红）> 8（有可变件）> 0（全部出证明）· 2 = 不给结论（缺件/清单解析不出/自检不过）")
    print("写旗标闭集（%d 词 · 形态无关）：%s" % (len(WRITE_WORDS), " ".join(WRITE_WORDS)))
    print("源码级牙（AST 判）：写调用 %s + `open(…,'w'|'a'|'x'|'+')` + subprocess 里的 git 写子命令" % " / ".join(DELETE_CALLS))
    return RC_OK


# ─────────────────────────── 源码级牙（C4 的「不假装」延伸到源码层）───────────────────────────
def assert_no_write_api(src, cut=None):
    """AST 判源码里的写调用 ⇒ 命中列表（`cut` 之后 = 自检夹具段，豁免）。"""
    head = src if cut is None else src.split(cut)[0]
    try:
        tree = ast.parse(head)
    except SyntaxError as e:
        return ["源码切段后 parse 失败（判不了 ⇒ 当红）：%s" % e]
    hits = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        f = node.func
        name = f.attr if isinstance(f, ast.Attribute) else (f.id if isinstance(f, ast.Name) else None)
        if name in DELETE_CALLS:
            hits.append("调用 %s()（第 %d 行）" % (name, node.lineno))
        elif name == "open":
            mode = ""
            if len(node.args) >= 2 and isinstance(node.args[1], ast.Constant):
                mode = str(node.args[1].value)
            for kw in node.keywords:
                if kw.arg == "mode" and isinstance(kw.value, ast.Constant):
                    mode = str(kw.value.value)
            if any(c in mode for c in "wax+"):
                hits.append("open(…, %r)（第 %d 行）" % (mode, node.lineno))
        elif name in ("run", "check_output", "call", "Popen", "check_call"):
            for a in node.args:
                if isinstance(a, (ast.List, ast.Tuple)):
                    for e in a.elts:
                        if isinstance(e, ast.Constant) and isinstance(e.value, str) and e.value in GIT_WRITE_SUBS:
                            hits.append("subprocess 里的 git 写子命令 %r（第 %d 行）" % (e.value, node.lineno))
    return hits


def parse_assume(items):
    out = {}
    for it in items or []:
        if "=" not in it:
            continue
        p, v = it.split("=", 1)
        out[p.strip()] = v.strip()
        out[os.path.abspath(p.strip())] = v.strip()
    return out


def parse_write_flags(argv):
    hits = []
    for tok in argv:
        t = tok.strip()
        if not t or t == "--":
            continue
        name = t.lstrip("-").split("=", 1)[0].strip().lower()
        if name in WRITE_WORDS:
            hits.append(t)
    return hits


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    hits = parse_write_flags(argv)
    if hits:
        print("⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s" % hits)
        print("   理由：本件**只出证明**，一个字节都不改（`C4` 只对只读/已封件说「零副作用」）。")
        print("   只读用法：bash scripts/gates/%s [--sealed <件>…] [--workspace] [--manifest <清单件>]" % SELF_NAME)
        return RC_NOCONC

    ap = argparse.ArgumentParser(description="C4 零副作用证明（只读/已封件出证明 · 可变件 rc=8）")
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--archive", default=DEFAULT_ARCHIVE, help="归档区根（默认与主仓同卷的兄弟目录 Zerg-归档/）")
    ap.add_argument("--manifest", default=None, help="自带基准清单件（默认自动发现归档记录-*.md）")
    ap.add_argument("--sealed", nargs="*", default=None, help="只证这些件（相对仓根或绝对路径）")
    ap.add_argument("--workspace", action="store_true", help="判工作树可变件（现读 git status）⇒ 期望 rc=8")
    ap.add_argument("--assume", action="append", default=None, help="负控：<件>=<sha256> 伪造基准（给可变件 ⇒ 判红）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()

    src = open(os.path.abspath(__file__), encoding="utf-8").read()      # 只读
    wp = assert_no_write_api(src, cut=SELFTEST_MARKER)
    if wp:
        print("⛔ 判红（rc=1）：本件**真目标段**出现写调用 ⇒ 「不假装」这一格破了：")
        for h in wp:
            print("   · %s" % h)
        return RC_RED

    sealed = None
    if a.sealed:
        sealed = [p if os.path.isabs(p) else os.path.join(os.path.abspath(a.root), p) for p in a.sealed]
    return run(a.archive, a.manifest, parse_assume(a.assume), sealed, a.workspace, os.path.abspath(a.root), a.json)


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────
def _mk_git_repo(d):
    repo = os.path.join(d, "repo")
    os.makedirs(repo, exist_ok=True)
    env = {"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
           "GIT_COMMITTER_EMAIL": "t@t", "HOME": d, "PATH": os.environ.get("PATH", "")}

    def g(*args):
        return subprocess.run(["git", "-C", repo] + list(args), capture_output=True, text=True, env=env, timeout=60)
    g("init", "-q", "-b", "main")
    with open(os.path.join(repo, "clean.txt"), "w", encoding="utf-8") as f:
        f.write("已封件\n")
    with open(os.path.join(repo, "dirty.txt"), "w", encoding="utf-8") as f:
        f.write("会被改的件\n")
    g("add", "--", "clean.txt", "dirty.txt")
    g("commit", "-qm", "init")
    with open(os.path.join(repo, "dirty.txt"), "a", encoding="utf-8") as f:
        f.write("改过了\n")
    return repo


def _mk_archive(d):
    arch = os.path.join(d, "Zerg-归档")
    sub = os.path.join(arch, "批一")
    os.makedirs(sub, exist_ok=True)
    x = os.path.join(sub, "x.md")
    with open(x, "w", encoding="utf-8") as f:
        f.write("只读件\n")
    os.chmod(x, 0o444)
    man = os.path.join(sub, "归档记录-夹具.md")
    sha_of_x = file_sha(x)                                       # 先落件再算摘要（不是反过来）
    with open(man, "w", encoding="utf-8") as f:
        f.write("# 归档记录 · 夹具\n\n| # | 原路径 | 篇数 | 字节 | sha256 | 归档路径 | 搬出日期 | 理由 |\n"
                "|---|---|---|---|---|---|---|---|\n"
                "| 1 | `x.md` | 1 | 10 | `%s` | `Zerg-归档/批一/x.md` | 2026-09-21 | 夹具 |\n" % sha_of_x)
    os.chmod(man, 0o444)                                        # 真归档区里记录件也是 `-r--r--r--`（照现读）
    return arch, x, man


def _run_cap(fn, *a, **kw):
    import contextlib
    import io
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        rc = fn(*a, **kw)
    return rc, buf.getvalue()


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="zse-selftest-")
    repo = _mk_git_repo(tmp)
    clean = os.path.join(repo, "clean.txt")
    dirty = os.path.join(repo, "dirty.txt")
    arch, x, man = _mk_archive(tmp)

    def case(name, want, got, note=""):
        nonlocal ok
        good = want == got
        ok = ok and good
        print("  %s %s（%s · 期望 %s）%s" % ("✓" if good else "✗", name, got, want, note))

    byp = {r["path"]: r for r in [classify_and_prove(x, arch, {"批一/x.md": file_sha(x)})]}

    # ① 正例：只读件 + 清单登记一致 ⇒ 出证明（基准来源 = 清单）
    r1 = classify_and_prove(x, arch, {"批一/x.md": file_sha(x)})
    case("① 正例：只读件 + 清单登记一致 ⇒ rc=0 且基准来源=清单", (0, True), (r1["rc"], str(r1["baseline_from"]).startswith("manifest")))

    # ② 负控：清单登记值 ≠ 现算 ⇒ 判红 rc=1（真红 · 基准不符）
    bad = "0" * 64
    r2 = classify_and_prove(x, arch, {"批一/x.md": bad})
    case("② 负控：清单登记值 ≠ 现算 ⇒ rc=1（基准不符）", 1, r2["rc"], note="  " + r2["note"][:40])

    # ③ 正例：仓内 clean 件（无清单）⇒ 基准 = HEAD blob ⇒ rc=0
    r3 = classify_and_prove(clean, arch, {})
    case("③ 正例：仓内已封件 ⇒ 基准来源=HEAD blob · rc=0", (0, True), (r3["rc"], str(r3["baseline_from"]).startswith("head-blob")))

    # ④ 负控（判据② · 真红）：工作树 dirty 件 ⇒ 「无可比基准」rc=8
    r4 = classify_and_prove(dirty, arch, {})
    case("④ 负控：工作树可变件 ⇒ 无可比基准 · rc=8", 8, r4["rc"], note="  " + r4["note"][:44])

    # ⑤ 负控（判据③ · 真红）：给可变件伪造基准 ⇒ rc=1
    r5 = classify_and_prove(dirty, arch, {}, assume="a" * 64)
    case("⑤ 负控：给可变件伪造基准 ⇒ rc=1 判红", 1, r5["rc"], note="  " + r5["note"][:44])

    # ⑥ 正例：只读件 + 给的基准与现算逐字相同 ⇒ 复核一致 · rc=0
    r6 = classify_and_prove(x, arch, {"批一/x.md": file_sha(x)}, assume=file_sha(x))
    case("⑥ 正例：只读件 + 基准值与现算相同 ⇒ rc=0（复核一致）", 0, r6["rc"], note="  " + r6["note"][:30])

    # ⑦ 负控：只读/已封件上给错值 ⇒ rc=1
    r7 = classify_and_prove(clean, arch, {}, assume="b" * 64)
    case("⑦ 负控：已封件上给一个错基准 ⇒ rc=1", 1, r7["rc"], note="  " + r7["note"][:40])

    # ⑧ 缺件：--sealed 指向不存在 ⇒ rc=2
    rc8, out8 = _run_cap(run, arch, man, {}, [os.path.join(tmp, "没有这个件")], False, repo, False)
    case("⑧ 缺件（--sealed 不在盘）⇒ rc=2 不给结论", 2, rc8, note="  " + out8.strip().splitlines()[-1][:40])

    # ⑨ 清单件缺 ⇒ rc=2
    rc9, out9 = _run_cap(run, arch, os.path.join(tmp, "没有这个清单.md"), {}, None, False, repo, False)
    case("⑨ 清单件缺 ⇒ rc=2 不给结论", 2, rc9)

    # ⑩ 混合：只读件（可证）+ 工作树可变件 ⇒ 有可变件 ⇒ rc=8（不许报 0）
    rc10, out10 = _run_cap(run, arch, man, {}, [x, dirty], False, repo, False)
    case("⑩ 混合：只读件 + 可变件 ⇒ rc=8（不许报 0）", 8, rc10,
         note="  证明 %s" % ("有" if "✓ 证明" in out10 else "无"))

    # ⑪ 归档记录表格形态：能自动发现 + 用表里登记的 sha 出证明
    rc11, out11 = _run_cap(run, arch, None, {}, None, False, repo, False)
    case("⑪ 自动发现归档记录（表格形态）⇒ rc=0 且表里登记值生效", 0, rc11,
         note="  明细 %s" % ("含 ✓ 证明" if "✓ 证明" in out11 else "缺"))

    # ⑫ 源码级牙（AST）：合成源码含 shutil.rmtree( ⇒ 命中；本件真目标段 0 命中；整篇应有命中
    case("⑫ 负控（源码级牙）：合成源码含 `shutil.rmtree(` ⇒ 命中", True,
         bool(assert_no_write_api("import shutil\ndef f(p):\n    shutil.rmtree(p)\n")))
    src = open(os.path.abspath(__file__), encoding="utf-8").read()
    hits = assert_no_write_api(src, cut=SELFTEST_MARKER)
    case("⑫b 正例：本件真目标段写调用命中 = 0", 0, len(hits), note=("  " + " / ".join(hits[:2])) if hits else "")
    case("⑫c 正例：整篇（含自检段）扫应有命中 ⇒ 扫描器不恒空", True, bool(assert_no_write_api(src)))

    # ⑬ 写旗标一律拒 rc=2（走真 CLI 入口 main）
    for flag in ("--chmod", "--seal", "commit", "--fix", "add"):
        rc13, _o = _run_cap(main, [flag])
        case("⑬ 负控：写旗标 %r ⇒ 拒 rc=2" % flag, 2, rc13)

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

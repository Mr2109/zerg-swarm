#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# check-artifact-source-parity.py —— 门74「制品 ↔ 源码两代分离」只读判据件
#
# 判什么：盘上那件**制品**（默认 `bin/zerg`）与**现读源码树**是不是同一代 ——
#   病象（缺口 `GAP-20260928-119`）：正门只写源码、不触发重编 ⇒ 源码已经往前走了几代，
#   而 `bin/zerg` 还是老的一件；快速档里**没有任何门**能把这个对拍出来（实测：源码改完
#   不重编，`zerg gate run --fast` 照样全绿 rc=0）。
#
# 两个面（判据各只有一个定义处，都不写死时点读数）：
#   面A **提交面**：制品自报的代码 sha ⟷ 本仓 `git rev-parse HEAD`
#     · 真源：`scripts/build/build-all.sh:111`（`SHA="$(git rev-parse --short HEAD)"`）
#       ⇒ 制品里那个短 sha 就是**建构那一刻的 HEAD**；建构后才落的提交，sha 会“落后”。
#     · 制品侧取值面：`<制品> --version` 的一行（格式真源 = `core/internal/version/version.go`
#       的 `func Line`：`<件名> <版本> <commit>[+dirty] <build_time>`）。
#   面B **时点面**：制品自报的建构时刻 ⟷ **制品输入面**最后一次改动
#     · 制品输入面 = 能进那件二进制的东西（现读，不写死清单）：
#       ① `core/**`、`shared/**` 下的 `.go` 件（跳过 `*_test.go` —— 测试件不进制品）；
#       ② 两个模块的 `go.mod` / `go.sum`；
#       ③ 源码里 `//go:embed` 指令真的点到的那几个资产件（现读指令再解析，
#          不另抄一份清单 —— `core/internal/contract/*.json`、`core/internal/compat/compat.json` 等）。
#     · 判据：制品建构时刻**早于**输入面最后一次改动 ⇒ 那几件改动没进制品 = **落后**。
#
# ★ 为什么不拿「制品 sha ≠ HEAD sha」当红（本件最要紧的一格）：
#   本仓常态是「先改源码 → 重编（工作树还脏）→ 再提交」。这种时序下制品的 sha 是
#   **提交前的**那一个（通常还带 `+dirty`），而制品其实**已经含**了随后落进提交的那些改动 ——
#   若拿 sha 不等就判红，等于每提交一笔就红一次：那不是严格，那是把闸废掉。
#   故：`dirty` 只**如实登记**（面C，不判红）；`sha 落后` 也只在与面B 合判时才升级为红 ——
#   面B 说「制品建构之后输入面又改了」才是**真的**分离。两件事分开说、分开判。
#
# 三档退码（与 `scripts/gates/precommit-gates.sh` 的 `tri` 档逐格同义）：
#   0 = 同源（制品不落后于现读源码）
#   1 = 分离（逐条点名分在哪：源码侧值 vs 制品侧值）
#   2 = **不给结论**（制品不在盘 / 跑不起来 / 自报面认不出 / HEAD 取不到 / 输入面空转）
#
# 只读（硬约束）：不写仓内仓外任何件、不落审计、不读状态目录、不起后台进程、不接管道取码；
#   自检夹具一律落 `tempfile.mkdtemp()` 的临时树。**不重编、不跑 go build / go test。**
#
# 用法：
#   python3 scripts/gates/check-artifact-source-parity.py                     # 判真仓的 bin/zerg
#   python3 scripts/gates/check-artifact-source-parity.py --root <仓根> --artifact <件>
#   python3 scripts/gates/check-artifact-source-parity.py --json
#   python3 scripts/gates/check-artifact-source-parity.py --list-rules
#   python3 scripts/gates/check-artifact-source-parity.py --self-test

import argparse
import datetime
import glob
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

TOOL = "check-artifact-source-parity.py"
TOOL_VERSION = "v1.0.0"

RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2
RC_NAME = {0: "OK（同源）", 1: "FAIL（两代分离）", 2: "BLOCKED（不给结论）"}

DEFAULT_ARTIFACT_REL = os.path.join("bin", "zerg")
INPUT_ROOTS = ("core", "shared")
GO_MOD_FILES = ("go.mod", "go.sum")
SKIP_DIR_NAMES = {".git", "__pycache__", "node_modules", "target", "testdata"}
VERSION_TIMEOUT = 15  # 秒（只跑 --version，耗时以该件自检自报为准 · 不在此写死）

# 制品自报面（`version.Line`）：<件名> <版本> <commit>[+dirty] <build_time>
ID_RE = re.compile(
    r"(?P<comp>\S+)\s+(?P<ver>\S+)\s+(?P<commit>[0-9a-fA-F]{7,40})(?P<dirty>\+dirty)?"
    r"\s+(?P<build>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s*$")
EMBED_RE = re.compile(r"^\s*//go:embed\s+(?P<pat>.+?)\s*$")
TAG_RE = re.compile(r"\b(" + "|".join(INPUT_ROOTS) + r")\b")

ENV_STRIP = ("ZERG_STATE_DIR", "ZERG_REPO", "ZERG_PROPOSAL_DIR")


# ── 取数面 ───────────────────────────────────────────────────────────────────
def read_artifact_identity(art):
    """现跑 `<art> --version` 取制品自报身份。⇒ (idict | None, 失败原因 | None, 原始输出)"""
    if not os.path.exists(art):
        return None, "制品不在盘：%s" % art, ""
    if not os.access(art, os.X_OK):
        return None, "制品不可执行（不是可跑的一件）：%s" % art, ""
    env = {k: v for k, v in os.environ.items() if k not in ENV_STRIP}
    try:
        p = subprocess.run([art, "--version"], capture_output=True, text=True,
                           timeout=VERSION_TIMEOUT, env=env)
    except subprocess.TimeoutExpired:
        return None, "跑 `--version` 超时（>%ds）" % VERSION_TIMEOUT, ""
    except OSError as e:
        return None, "跑不起来：%s" % e, ""
    out = (p.stdout or "") + ("\n" + p.stderr if (p.stderr or "").strip() else "")
    if p.returncode != 0:
        return None, "`--version` rc=%d" % p.returncode, out.strip()
    for line in (p.stdout or "").splitlines():
        m = ID_RE.search(line.strip())
        if m:
            d = m.groupdict()
            ts = datetime.datetime.strptime(d["build"], "%Y-%m-%dT%H:%M:%SZ").replace(
                tzinfo=datetime.timezone.utc).timestamp()
            return {"commit": d["commit"].lower(), "commit_raw": d["commit"],
                    "dirty": bool(d["dirty"]), "version": d["ver"],
                    "component": d["comp"], "build_epoch": ts,
                    "build_time": d["build"]}, None, out.strip()
    return None, "自报面认不出（没有 `<件名> <版本> <commit>[+dirty] <ISO8601Z>` 那样一行）", out.strip()


def git(root, *args):
    try:
        p = subprocess.run(["git", "-C", root] + list(args), capture_output=True,
                           text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return None
    if p.returncode != 0:
        return None
    return p.stdout.strip()


def collect_input_face(root):
    """现读制品输入面 ⇒ ([(路径, mtime)…], embed 件数, 扫描面件数)"""
    files, embeds = [], []
    for rel in INPUT_ROOTS:
        base = os.path.join(root, rel)
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, fnames in os.walk(base):
            dirnames[:] = [d for d in dirnames
                           if d not in SKIP_DIR_NAMES and not d.startswith((".", "_"))]
            for fn in fnames:
                if fn.startswith((".", "_")) or fn.endswith("_test.go"):
                    continue
                p = os.path.join(dirpath, fn)
                if fn.endswith(".go"):
                    files.append(p)
                    try:
                        with open(p, "r", encoding="utf-8", errors="replace") as fh:
                            for line in fh:
                                m = EMBED_RE.match(line)
                                if not m:
                                    continue
                                for tok in m.group("pat").split():
                                    for hit in glob.glob(os.path.join(dirpath, tok)):
                                        if os.path.isfile(hit):
                                            embeds.append(hit)
                    except OSError:
                        pass
                elif fn in GO_MOD_FILES:
                    files.append(p)
    files = sorted(set(files) | set(embeds))
    out = []
    for p in files:
        try:
            out.append((p, os.stat(p).st_mtime))
        except OSError:
            pass
    return out, len(set(embeds)), len(files)


def _iso(epoch):
    return datetime.datetime.fromtimestamp(epoch, datetime.timezone.utc).strftime(
        "%Y-%m-%dT%H:%M:%SZ")


def _local(epoch):
    return datetime.datetime.fromtimestamp(epoch).strftime("%Y-%m-%d %H:%M:%S")


def judge(root, art):
    """⇒ (rc, lines, data)"""
    root = os.path.abspath(root)
    art = os.path.abspath(art)
    lines, data = [], {"root": root, "artifact": art}
    lines.append("门74 制品↔源码两代分离（只读 · 现读对拍）")
    lines.append("  制品侧 %s" % art)

    ident, err, raw = read_artifact_identity(art)
    if ident is None:
        lines.append("  ✗ 2 不给结论：%s" % err)
        if raw:
            lines.append("    制品自报原文：%s" % raw.replace("\n", " ⏎ "))
        lines.append("    ★ 「取不到数」不当绿 —— 无制品 / 认不出 ⇒ BLOCKED")
        data["reason"] = err
        return RC_BLOCKED, lines, data

    data.update({"artifact_commit": ident["commit"], "artifact_dirty": ident["dirty"],
                 "artifact_version": ident["version"], "artifact_build": ident["build_time"]})
    lines.append("  制品侧：版本 %s · 代码 %s%s · 构建 %s（现跑 `--version` 取）"
                 % (ident["version"], ident["commit_raw"],
                    "+dirty" if ident["dirty"] else "", ident["build_time"]))

    head_full = git(root, "rev-parse", "HEAD")
    if not head_full:
        lines.append("  ✗ 2 不给结论：源码侧 HEAD 取不到（%s 不是 git 仓 / git 不可用）" % root)
        data["reason"] = "HEAD unavailable"
        return RC_BLOCKED, lines, data
    head_short = git(root, "rev-parse", "--short", "HEAD") or head_full[:7]
    status = git(root, "status", "--porcelain")
    dirty_count = len([x for x in (status or "").splitlines() if x.strip()])
    data.update({"head_full": head_full, "head_short": head_short, "tree_dirty": dirty_count})
    lines.append("  源码侧 %s：HEAD %s（短 %s）· 工作树未提交 %d 件"
                 % (root, head_full, head_short, dirty_count))

    face, embed_n, _ = collect_input_face(root)
    if not face:
        lines.append("  ✗ 2 不给结论：制品输入面空转（%s 下 0 件可读 —— 空转不许当绿）"
                     % " / ".join(INPUT_ROOTS))
        data["reason"] = "input face empty"
        return RC_BLOCKED, lines, data
    newest_path, newest_mt = max(face, key=lambda t: t[1])
    data.update({"input_files": len(face), "embed_files": embed_n,
                 "newest_input": os.path.relpath(newest_path, root),
                 "newest_input_mtime": _iso(newest_mt)})
    lines.append("  源码侧制品输入面：%d 件（含 go:embed 资产 %d 件）· 最后一次改动 %s  %s"
                 % (len(face), embed_n, _iso(newest_mt), os.path.relpath(newest_path, root)))

    # ── 面A 提交面 ──
    art_commit = ident["commit"]
    same = head_full.startswith(art_commit) or head_short.startswith(art_commit) \
        or art_commit.startswith(head_short)
    known = git(root, "cat-file", "-e", art_commit) is not None
    behind = None
    if same:
        face_a = "同一笔（制品自报 sha 就是现读 HEAD）"
    elif known and git(root, "merge-base", "--is-ancestor", art_commit, "HEAD") is not None:
        cnt = git(root, "rev-list", "--count", "%s..HEAD" % art_commit)
        behind = int(cnt) if (cnt or "").isdigit() else None
        face_a = "制品那一代是 HEAD 的祖先 · 落后 %s 笔（建构之后又落了提交）" % (
            behind if behind is not None else "?")
    elif known:
        face_a = "**分叉**：制品那一代不是现读 HEAD 的祖先（两条线）"
    else:
        face_a = "**制品那一代不在本仓对象库**（sha 不是本仓任何一代）"
    data.update({"face_a": face_a, "behind": behind, "ancestor_known": known})

    # ── 面B 时点面 ──
    lagging = sorted([(p, mt) for p, mt in face if mt > ident["build_epoch"]],
                     key=lambda t: t[1], reverse=True)
    data.update({"lagging_count": len(lagging),
                 "lagging": [{"file": os.path.relpath(p, root), "mtime": _iso(mt)}
                             for p, mt in lagging[:10]]})
    face_b_lag = bool(lagging)

    lines.append("  面A 提交面：%s" % face_a)
    lines.append("  面B 时点面：制品建构 %s（%s）%s 输入面最后一次改动 %s  ⇒ %s"
                 % (ident["build_time"], _local(ident["build_epoch"]),
                    "早于" if face_b_lag else "不早于", _iso(newest_mt),
                    "**落后**" if face_b_lag else "制品已包含现读输入面"))
    lines.append("  面C 脏标记：制品自报 %s —— dirty 是常态（建构时工作树有未提交改动），"
                 "**本件不因 dirty 判红**" % ("+dirty" if ident["dirty"] else "无 dirty"))

    rc = RC_OK
    if face_b_lag:
        rc = RC_FAIL
        lines.append("  ✗ 1 **分离（落后）**：制品建构之后，下列制品输入面又改过 —— 这些改动没进制品：")
        for p, mt in lagging[:10]:
            lines.append("    · %s（源码侧 %s ＞ 制品侧 %s）"
                         % (os.path.relpath(p, root), _iso(mt), ident["build_time"]))
        if len(lagging) > 10:
            lines.append("    · …另 %d 件（同一判据，此处只列前 10）" % (len(lagging) - 10))
        lines.append("    ⇒ 治法：`bash scripts/build/build-all.sh --only-cli` 重编（旧件自动留档入 bin/_history/）")
    elif not known:
        rc = RC_BLOCKED
        lines.append("  ✗ 2 不给结论：面A 说「制品那一代不在本仓对象库」⇒ 两代关系判不了（不猜、不当绿）")
    elif behind is not None and behind > 0:
        lines.append("  ✓ 0 同源（提交面落后 %s 笔是**常态**：先改 → 重编 → 再提交，制品已含 HEAD 的内容；"
                     "判据落在面B）" % behind)
    else:
        lines.append("  ✓ 0 同源：面A 同笔 · 面B 未落后")
    if rc == RC_OK and behind is None and not same:
        lines.append("  ✓ 0 同源：面B 未落后（判据落在面B）")
    data["rc"] = rc
    data["rc_name"] = RC_NAME[rc]
    return rc, lines, data


# ── 自检（合成夹具 · 一律落临时树）────────────────────────────────────────────
SELF_TOTAL = [0]
SELF_BAD = [0]


def assert_eq(name, got, want):
    SELF_TOTAL[0] += 1
    if got != want:
        SELF_BAD[0] += 1
        print("  ✗ 断言失败：%s —— 得到 %r，期望 %r" % (name, got, want))


def _mk_repo(tmp, tag, sha_body):
    """合成一个真 git 仓：一筆提交 + 一个 core/*.go 输入件 + 一个假制品。"""
    r = os.path.join(tmp, tag)
    os.makedirs(os.path.join(r, "core"), exist_ok=True)
    with open(os.path.join(r, "core", "a.go"), "w") as f:
        f.write("package main\n" + sha_body + "\n")
    subprocess.run(["git", "init", "-q"], cwd=r, capture_output=True, text=True)
    subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "add", "-A"],
                   cwd=r, capture_output=True, text=True)
    subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x"],
                   cwd=r, capture_output=True, text=True)
    return r


def _fake_artifact(root, name, commit, build_iso, dirty=False):
    p = os.path.join(root, name)
    with open(p, "w") as f:
        f.write("#!/bin/sh\necho \"zerg 2.5.12 %s%s %s\"\n"
                % (commit, "+dirty" if dirty else "", build_iso))
    os.chmod(p, 0o755)
    return p


def _set_mtime(path, iso):
    ts = datetime.datetime.strptime(iso, "%Y-%m-%dT%H:%M:%SZ").replace(
        tzinfo=datetime.timezone.utc).timestamp()
    os.utime(path, (ts, ts))


def run_self_test():
    print("门74 制品↔源码两代分离 · 自检（合成夹具 · 临时树）")
    tmp = tempfile.mkdtemp(prefix="g74-selftest-")
    try:
        # ① 正控：制品 sha = HEAD 短 sha，且建构晚于输入面 ⇒ rc=0
        r1 = _mk_repo(tmp, "same", "// v1")
        h1 = git(r1, "rev-parse", "HEAD")
        a1 = _fake_artifact(r1, "art1", h1[:16], "2026-09-28T10:00:00Z")
        _set_mtime(os.path.join(r1, "core", "a.go"), "2026-09-28T09:00:00Z")
        rc, lines, d1 = judge(r1, a1)
        assert_eq("① 制品 sha = HEAD 且建构不早于输入面 ⇒ rc=0", rc, 0)

        # ② 分离（落后）：制品建构早于输入面最后一次改动 ⇒ rc=1 且点名那一件
        r2 = _mk_repo(tmp, "lag", "// v1")
        h2 = git(r2, "rev-parse", "HEAD")
        a2 = _fake_artifact(r2, "art2", h2[:16], "2026-09-27T00:00:00Z")
        _set_mtime(os.path.join(r2, "core", "a.go"), "2026-09-28T09:00:00Z")
        rc, lines, d2 = judge(r2, a2)
        assert_eq("② 制品建构早于输入面改动 ⇒ rc=1（真落后）", rc, 1)
        assert_eq("② 逐条点名分在哪（报的是 core/a.go）",
                  sum(1 for ln in lines if "· core/a.go（源码侧" in ln), 1)
        assert_eq("② 报告里同时印出两侧值（制品侧建构时刻）",
                  sum(1 for ln in lines if "2026-09-27T00:00:00Z" in ln) >= 1, True)

        # ③ 负控（换回）：同一夹具把制品换回「建构晚于输入面」⇒ 回 0
        r3 = _mk_repo(tmp, "back", "// v1")
        h3 = git(r3, "rev-parse", "HEAD")
        a3 = _fake_artifact(r3, "art3", h3[:16], "2026-09-28T10:00:00Z")
        _set_mtime(os.path.join(r3, "core", "a.go"), "2026-09-28T09:00:00Z")
        assert_eq("③ 负控（换回）⇒ rc=0", judge(r3, a3)[0], 0)

        # ④ ★ 反误报（本件最要紧的一格）：dirty 常态 + 提交面落后 ⇒ 必须 rc=0
        r4 = _mk_repo(tmp, "dirty", "// v1")
        subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "commit",
                        "-q", "--allow-empty", "-m", "y"], cwd=r4,
                       capture_output=True, text=True)  # HEAD 前进一笔（制品 sha 现在是祖先）
        h4_old = git(r4, "rev-list", "--max-count=1", "HEAD~1")
        a4 = _fake_artifact(r4, "art4", h4_old[:16], "2026-09-28T10:00:00Z", dirty=True)
        _set_mtime(os.path.join(r4, "core", "a.go"), "2026-09-28T09:00:00Z")
        rc, lines, d4 = judge(r4, a4)
        assert_eq("④ dirty + 提交面落后 1 笔 + 建构不早于输入面 ⇒ rc=0（禁把常态判红）", rc, 0)
        assert_eq("④ 面C 如实登记了 dirty",
                  sum(1 for ln in lines if "面C 脏标记：制品自报 +dirty" in ln), 1)
        assert_eq("④ 面A 如实说明落后笔数", d4["behind"], 1)

        # ⑤ 取数面拿不到（制品不在盘 · argv 指向不存在路径）⇒ rc=2
        rc, lines, _ = judge(r3, os.path.join(tmp, "没有这一件", "zerg"))
        assert_eq("⑤ 制品不在盘 ⇒ rc=2（不给结论）", rc, 2)
        assert_eq("⑤ 报的是「制品不在盘」", sum(1 for ln in lines if "制品不在盘" in ln), 1)

        # ⑥ 制品自报面认不出 ⇒ rc=2
        bad = os.path.join(r3, "art-bad")
        with open(bad, "w") as f:
            f.write("#!/bin/sh\necho 我不知道\n")
        os.chmod(bad, 0o755)
        rc, lines, _ = judge(r3, bad)
        assert_eq("⑥ 自报面认不出 ⇒ rc=2", rc, 2)
        assert_eq("⑥ 报的是「自报面认不出」", sum(1 for ln in lines if "自报面认不出" in ln), 1)

        # ⑦ 制品那一代不在本仓对象库 ⇒ rc=2（两代关系判不了 · 不当绿也不误红）
        a7 = _fake_artifact(r3, "art7", "deadbee1234", "2026-09-28T10:00:00Z")
        rc, lines, _ = judge(r3, a7)
        assert_eq("⑦ 制品 sha 不是本仓对象 ⇒ rc=2（判不了）", rc, 2)

        # ⑧ 输入面空转 ⇒ rc=2
        r8 = os.path.join(tmp, "empty")
        os.makedirs(os.path.join(r8, "core"), exist_ok=True)
        subprocess.run(["git", "init", "-q"], cwd=r8, capture_output=True, text=True)
        a8 = _fake_artifact(r8, "art8", "0" * 16, "2026-09-28T10:00:00Z")
        rc, lines, _ = judge(r8, a8)
        assert_eq("⑧ 输入面空转 ⇒ rc=2（空转不许当绿）", rc, 2)

        # ⑨ 测试件不算制品输入面（*_test.go 改了不许判红）—— 防误报
        r9 = _mk_repo(tmp, "testonly", "// v1")
        h9 = git(r9, "rev-parse", "HEAD")
        a9 = _fake_artifact(r9, "art9", h9[:16], "2026-09-28T10:00:00Z")
        _set_mtime(os.path.join(r9, "core", "a.go"), "2026-09-28T09:00:00Z")
        tp = os.path.join(r9, "core", "a_test.go")
        with open(tp, "w") as f:
            f.write("package main\n")
        _set_mtime(tp, "2026-09-28T11:00:00Z")
        assert_eq("⑨ 只改了 *_test.go ⇒ rc=0（测试件不进制品）", judge(r9, a9)[0], 0)

        # ⑩ go:embed 资产改了要判红（资产是编译进制品的）
        r10 = _mk_repo(tmp, "embed", "// v1")
        with open(os.path.join(r10, "core", "a.go"), "w") as f:
            f.write("package main\n\n//go:embed asset.json\nvar raw string\n")
        with open(os.path.join(r10, "core", "asset.json"), "w") as f:
            f.write("{}\n")
        subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "add", "-A"],
                       cwd=r10, capture_output=True, text=True)
        subprocess.run(["git", "-c", "user.email=s@x", "-c", "user.name=s", "commit",
                        "-qm", "e"], cwd=r10, capture_output=True, text=True)
        h10 = git(r10, "rev-parse", "HEAD")
        a10 = _fake_artifact(r10, "art10", h10[:16], "2026-09-28T10:00:00Z")
        _set_mtime(os.path.join(r10, "core", "a.go"), "2026-09-28T09:00:00Z")
        _set_mtime(os.path.join(r10, "core", "asset.json"), "2026-09-28T11:00:00Z")
        rc, lines, d10 = judge(r10, a10)
        assert_eq("⑩ go:embed 资产被现读认了出来（1 件）", d10["embed_files"], 1)
        assert_eq("⑩ 资产改了（建构之后）⇒ rc=1", rc, 1)
        assert_eq("⑩ 点名的是 core/asset.json",
                  sum(1 for ln in lines if "· core/asset.json（源码侧" in ln), 1)

        # ⑪ --json 面可解析且字段齐（机器可读面不许各写一份口径）
        _, _, dj = judge(r1, a1)
        js = json.loads(json.dumps(dj, ensure_ascii=False))
        assert_eq("⑪ --json 面字段齐（rc / 制品侧 / 源码侧三格）",
                  all(k in js for k in ("rc", "artifact_commit", "head_full")), True)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (SELF_TOTAL[0], SELF_BAD[0]))
    return 0 if SELF_BAD[0] == 0 else 1


def print_rules():
    print("门74 制品↔源码两代分离 · 规则表（%s %s）" % (TOOL, TOOL_VERSION))
    print("判据面A 提交面：制品 `--version` 自报 commit ⟷ `git rev-parse HEAD`")
    print("  · 真源 scripts/build/build-all.sh:111（SHA=$(git rev-parse --short HEAD)）")
    print("判据面B 时点面：制品自报 build_time ⟷ 制品输入面最后一次改动 mtime")
    print("  输入面 = %s 下非测试 %s + %s + 源码 //go:embed 点到的资产（现读，不写死）"
          % ("/".join(INPUT_ROOTS), "*.go", "/".join(GO_MOD_FILES)))
    print("判据面C 脏标记：只登记不判红（dirty 是常态）")
    print("退码：0 同源 · 1 分离（逐条点名）· 2 不给结论（制品不在盘/跑不起来/自报面认不出/"
          "HEAD 取不到/输入面空转）")
    print("只读：不写任何件 · 不重编 · 不跑 go build / go test · 不读状态目录")


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=None)
    ap.add_argument("--artifact", default=None)
    ap.add_argument("--json", dest="as_json", action="store_true")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--list-rules", dest="list_rules", action="store_true")
    args = ap.parse_args()

    if args.list_rules:
        print_rules()
        return RC_OK
    if args.self_test:
        return run_self_test()

    root = os.path.abspath(args.root or os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    art = os.path.abspath(args.artifact or os.path.join(root, DEFAULT_ARTIFACT_REL))
    rc, lines, data = judge(root, art)
    if args.as_json:
        print(json.dumps(data, ensure_ascii=False, indent=2))
    else:
        for ln in lines:
            print(ln)
    return rc


if __name__ == "__main__":
    sys.exit(main())

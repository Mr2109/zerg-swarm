#!/usr/bin/env python3
# check-contract-version-diff.py — 契约「版本间对拍」（旧版 ↔ 新版 `registry.json`）· 立项单 `R5` / 账 `G-27`。
#
# 为什么有这一件（缺口原话）：`设计-变更影响面-v1.6.md:712` 逐字
#   「**要，但另立项**，且**先只对 `registry.json` 的 7 条**」
#   —— 理由是「这是**唯一能回答『破了什么承诺』**的面；范围不先钉死会变成全仓快照工程」。
# 口径四条（同件 `:650` 写死 · 原型墙钟 0.062s）：
#   ① 基线 = `git log -1 --format=%H -- core/internal/contract/registry.json`（**上一次改动该件的提交**）**及其父**
#      —— **不落新快照件** ✗（同形一手：`buf breaking --against '.git#branch=main'`，基线可以是 Git 引用）；
#   ② 比什么 = `entries[].id` 集合 + 每条 **6 字段**（`name` / `truth` / `truth_shape` / `version_field` /
#      `gate` / `change_class`）+ `change_flow` / `authority` **整体**
#      —— `note` / `schema` / `change_note` **不在判据面**（照口径②，不自行扩面）；
#   ③ 怎么出 = **一行一条差异**（`<id> <字段> 旧→新`）· **零差异明写「无变更」**；
#   ④ 存哪 = **只存对拍结果** —— 产物必须带**两枚指纹**（码侧 `head_sha` + **基线 rev**；同族但不同字段面，
#      别混用 ✗）⇒ 本件把两枚指纹印在人面与 `--json` 机器面上（`--out` 可另存一份结果，**不落新快照件**）。
# 退路位（同件 `:650` 写死）：基线 rev 取不到（浅克隆 / 该件只提交过一次）⇒ **明写「无上一版可比」**，
#   **不许拿工作树当基线糊过去** ✗。
#
# 三档退码（`G-27` 判据「rc=0/1 三档」）：
#   rc=0  **无变更**（零差异 ⇒ 逐字印「无变更」）；rc=1  **有差异**（逐条见上）；rc=2  **不给结论**
#   （根 / 表 / git / 基线 rev / 父提交 / 旧版件 任一读不到 —— 本仓铁律：读不到不给结论，**不许当绿** ✗）。
#   ★ rc=1 只是报告档的「有」档：本件**不是** must-be-zero 的门（改契约本身是合法动作），
#     兼容级别「只许报、不许据此红绿」（§十 `R5` 同口径）⇒ 接线时按**报告档**挂，别当阻断门 ✗。
#
# 纪律：
#   · 本件**只读**（不写 `registry.json`、不落快照件、不写状态目录；`--out` 只在你点名时才写）；
#   · **基线只许取 git 引用** —— 工作树件只作「新版」读源，永不当基线；
#   · 自带 `--self-test`（成对：正控 2 组 + 负控 11 组）—— 「永远报红」与「永远不报红」一样没用；
#   · 缺读数一律 rc=2（不给结论）。
#
# 用法：
#   python3 scripts/gates/check-contract-version-diff.py                       # 默认在仓内自动定位根
#   python3 scripts/gates/check-contract-version-diff.py --root <仓根>
#   python3 scripts/gates/check-contract-version-diff.py --base-rev <rev>      # 指定基线（默认自动取）
#   python3 scripts/gates/check-contract-version-diff.py --old-file <旧件> --new-file <新件>   # 夹具用（两份 json 直接对拍）
#   python3 scripts/gates/check-contract-version-diff.py --json <字段,逗号分隔>
#   python3 scripts/gates/check-contract-version-diff.py --out <结果件路径>     # 另存一份结果（默认只打印）
#   python3 scripts/gates/check-contract-version-diff.py --self-test            # 自检（成对正/负控）
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time

REGISTRY_REL = os.path.join("core", "internal", "contract", "registry.json")
ROOT_ANCHOR = os.path.join("core", "internal", "version", "version.go")
ENTRY_FIELDS = ("name", "truth", "truth_shape", "version_field", "gate", "change_class")
WHOLE_TOP_KEYS = ("authority", "change_flow")
JSON_FIELDS = ("head_sha", "base_rev", "old_entries", "new_entries", "diff_lines",
               "verdict", "worktree_dirty", "elapsed_ms")


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, ROOT_ANCHOR)):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def git(root, args):
    """⇒ (rc, stdout)。git 不在盘 / 不是仓 ⇒ rc≠0（调用侧判 rc=2 不给结论）。"""
    try:
        p = subprocess.run(["git", "-c", "core.quotepath=false"] + args, cwd=root, capture_output=True, text=True)
    except (OSError, subprocess.SubprocessError) as e:  # noqa: BLE001 —— 取不到就是不给结论
        return 127, str(e)
    return p.returncode, p.stdout.strip()


def flatten(doc, prefix=""):
    """把顶层块摊成「路径 ⇒ 值」的字典（`authority` / `change_flow` 是**整体**比，逐键给差异行）。"""
    out = {}
    if isinstance(doc, dict):
        for k, v in doc.items():
            out.update(flatten(v, "%s.%s" % (prefix, k) if prefix else str(k)))
    elif isinstance(doc, list):
        for i, v in enumerate(doc):
            out.update(flatten(v, "%s[%d]" % (prefix, i)))
    else:
        out[prefix] = doc
    return out


def show(v):
    """值的人面写法（长串截断，免得一行吃掉整个终端）。"""
    s = json.dumps(v, ensure_ascii=False) if not isinstance(v, str) else v
    return s if len(s) <= 120 else s[:117] + "…"


def compare_docs(old, new):
    """核心对拍（纯函数 · 夹具与真跑同一条路径）⇒ 差异行 list。"""
    lines = []
    oe = old.get("entries") or []
    ne = new.get("entries") or []
    om = {str(e.get("id", "")): e for e in oe}
    nm = {str(e.get("id", "")): e for e in ne}
    for i in [x for x in om if x not in nm]:
        lines.append("%s 整条 旧→新（**删除**条目：该 id 不在新版里）" % i)
    for i in [x for x in nm if x not in om]:
        lines.append("%s 整条 旧→新（**新增**条目：该 id 不在旧版里）" % i)
    for i in [x for x in nm if x in om]:
        for f in ENTRY_FIELDS:
            ov, nv = om[i].get(f), nm[i].get(f)
            if ov != nv:
                lines.append("%s %s 旧→新 ｜ 旧=%s 新=%s" % (i, f, show(ov), show(nv)))
    for k in WHOLE_TOP_KEYS:
        fo, fn = flatten({k: old.get(k)}), flatten({k: new.get(k)})
        for path in sorted(set(fo) | set(fn)):
            if fo.get(path, "<无>") != fn.get(path, "<无>"):
                lines.append("顶层 %s 旧→新 ｜ 旧=%s 新=%s"
                             % (path, show(fo.get(path, "<无>")), show(fn.get(path, "<无>"))))
    return lines


def read_json(path):
    """⇒ (doc, err)。读不到 / 解析不了 ⇒ err 非空（调用侧判 rc=2）。"""
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f), None
    except Exception as e:  # noqa: BLE001 —— 读不了就是不猜
        return None, str(e)


def resolve_base(root, rel):
    """① 口径：基线 = 上一次改动该件的提交（`git log -1 --format=%H -- <件>`）及其父。
    ⇒ (base_rev, parent_rev, err)。**永不回落工作树** ✗。"""
    rc, out = git(root, ["log", "-1", "--format=%H", "--", rel])
    if rc != 0:
        return None, None, "git log 取不到该件的历史（rc=%d）：%s" % (rc, out)
    base = out.splitlines()[0].strip() if out else ""
    if not base:
        return None, None, "该件在 git 历史里**查无提交**（没提交过 / 浅克隆里没有它）"
    rc, out = git(root, ["rev-parse", base + "^"])
    if rc != 0:
        return base, None, "基线 rev 取不到**父提交**（该件只提交过一次 · 仓太浅）：%s" % out
    return base, out.splitlines()[0].strip(), None


def check(root, registry_path, base_rev=None, old_file=None, new_file=None):
    """⇒ (rc, lines, info)。rc=2 = 不给结论（根 / 表 / git / 基线 / 旧版件 任一读不到）。"""
    t0 = time.time()
    info = {"head_sha": "", "base_rev": "", "old_entries": 0, "new_entries": 0,
            "diff_lines": 0, "verdict": "", "worktree_dirty": False, "elapsed_ms": 0}
    lines = []

    def done(rc):
        info["elapsed_ms"] = int(round((time.time() - t0) * 1000))
        return rc, lines, info

    # ── 夹具档：两份 json 直接对拍（不碰 git；负控/自检走这条路） ─────────────
    if old_file and new_file:
        old, e1 = read_json(old_file)
        new, e2 = read_json(new_file)
        if e1 or e2:
            lines.append("⛔ 不给结论：夹具件读不到 / 解析失败（旧：%s · 新：%s）—— 两侧都得读得到才谈差异"
                         % (e1 or "ok", e2 or "ok"))
            info["verdict"] = "blocked"
            return done(2)
        return finish(info, lines, old, new, new_file, done)

    if root is None:
        lines.append("⛔ 不给结论：解析不到仓根（找 %s 失败）—— 设 ZERG_REPO=<仓根>" % ROOT_ANCHOR)
        info["verdict"] = "blocked"
        return done(2)
    if not os.path.isfile(registry_path):
        lines.append("⛔ 不给结论：新版件读不到（%s）—— 表不在盘 ⇒ 无对象可对拍" % registry_path)
        info["verdict"] = "blocked"
        return done(2)

    rc, head = git(root, ["rev-parse", "HEAD"])
    if rc != 0:
        lines.append("⛔ 不给结论：取不到 `head_sha`（`git rev-parse HEAD` rc=%d）：%s" % (rc, head))
        info["verdict"] = "blocked"
        return done(2)
    info["head_sha"] = head.splitlines()[0].strip()

    if not base_rev:
        got, _parent, err = resolve_base(root, registry_path)
        if err or not got:
            lines.append("⛔ 不给结论：**无上一版可比** —— %s" % (err or "基线 rev 取到空值"))
            lines.append("   （退路位口径：基线 rev 取不到就明写「无上一版可比」，"
                         "**不许拿工作树当基线糊过去** ✗）")
            info["verdict"] = "blocked"
            return done(2)
        base_rev = got

    rc, parent = git(root, ["rev-parse", base_rev + "^"])
    if rc != 0:
        lines.append("⛔ 不给结论：**无上一版可比** —— 基线 rev %s 取不到父提交（该件只提交过一次 · 仓太浅）"
                     % base_rev)
        info["verdict"] = "blocked"
        return done(2)
    parent = parent.splitlines()[0].strip()
    info["base_rev"] = base_rev

    rel = os.path.relpath(registry_path, root)
    rc, raw = git(root, ["show", "%s:%s" % (parent, rel)])
    if rc != 0:
        lines.append("⛔ 不给结论：旧版件在基线父提交上读不到（`git show %s:%s` rc=%d）"
                     % (parent[:12], rel, rc))
        info["verdict"] = "blocked"
        return done(2)
    try:
        old = json.loads(raw)
    except Exception as e:  # noqa: BLE001
        lines.append("⛔ 不给结论：旧版件解析失败（%s:%s）：%s" % (parent[:12], rel, e))
        info["verdict"] = "blocked"
        return done(2)

    new, err = read_json(registry_path)
    if err:
        lines.append("⛔ 不给结论：新版件解析失败（%s）：%s" % (registry_path, err))
        info["verdict"] = "blocked"
        return done(2)

    _rc, dirty = git(root, ["status", "--porcelain", "--", rel])
    info["worktree_dirty"] = bool(dirty) if _rc == 0 else False
    return finish(info, lines, old, new, registry_path, done, old_label="%s:%s" % (parent[:12], rel))


def finish(info, lines, old, new, new_path, done, old_label=None):
    lines.append("旧版 = %s ｜ 新版 = %s" % (old_label or "（夹具旧件）", new_path))
    lines.append("两枚指纹：`head_sha` = %s ｜ 基线 rev = %s"
                 % (info["head_sha"] or "（夹具档无）", info["base_rev"] or "（夹具档无）"))
    oe = old.get("entries") or []
    ne = new.get("entries") or []
    info["old_entries"] = len(oe)
    info["new_entries"] = len(ne)
    diffs = compare_docs(old, new)
    info["diff_lines"] = len(diffs)
    lines.append("条数：旧 %d / 新 %d ｜ 差异行 %d" % (len(oe), len(ne), len(diffs)))
    if diffs:
        lines.extend(diffs)
        info["verdict"] = "changed"
        return done(1)
    lines.append("无变更")
    info["verdict"] = "unchanged"
    return done(0)


def self_test(root):
    """成对自检：正控 2 组 + 负控 8 组。夹具一律在临时目录（**不碰真目标** ✗）。"""
    bad, n = [], 0

    def case(name, want_rc, got_rc, want_word=None, lines=None):
        nonlocal n
        n += 1
        if got_rc != want_rc:
            bad.append("✗ %s：want rc=%d got rc=%d" % (name, want_rc, got_rc))
        elif want_word and not any(want_word in l for l in (lines or [])):
            bad.append("✗ %s：rc 对了但判词里没有 %r" % (name, want_word))

    real = os.path.join(root, REGISTRY_REL) if root else REGISTRY_REL
    tmp = tempfile.mkdtemp(prefix="zerg-cvd-selftest-")
    try:
        # ① 正控甲：同件对同件（夹具）⇒ rc=0 且逐字印「无变更」
        rc, lines, _ = check(None, real, old_file=real, new_file=real)
        case("① 正控：同件对同件 ⇒ rc=0 且印「无变更」", 0, rc, "无变更", lines)

        # ① 正控乙：真目标（旧版取基线父 · 新版取盘上件）⇒ 只许 0 或 1（正经跑），不许 2
        rc_real, lines_real, info_real = check(root, real)
        n += 1
        if rc_real not in (0, 1):
            bad.append("✗ ② 正控：真目标对拍给了 rc=%d（不给结论 ⇒ 基线面或表读不到）" % rc_real)
        elif not info_real["base_rev"] or not info_real["head_sha"]:
            bad.append("✗ ② 正控：两枚指纹没印全（head_sha/base_rev）")

        good, _ = read_json(real)
        if good is None:
            bad.append("✗ 自检前置：真表读不到（%s）" % real)
            raise SystemExit(1)

        def fixture(name, mutate):
            doc = json.loads(json.dumps(good, ensure_ascii=False))
            mutate(doc)
            p = os.path.join(tmp, name)
            with open(p, "w", encoding="utf-8") as fh:
                json.dump(doc, fh, ensure_ascii=False, indent=1)
            return check(None, real, old_file=real, new_file=p)

        with open(real, encoding="utf-8") as fh:
            old_text = fh.read()
        oldp = os.path.join(tmp, "old.json")
        with open(oldp, "w", encoding="utf-8") as fh:
            fh.write(old_text)

        def pair(name, mutate, base=oldp):
            doc = json.loads(old_text)
            mutate(doc)
            p = os.path.join(tmp, name)
            with open(p, "w", encoding="utf-8") as fh:
                json.dump(doc, fh, ensure_ascii=False, indent=1)
            return check(None, real, old_file=base, new_file=p)

        # ② 负控：改一条字段（gate 追加一个脚本）⇒ rc=1 且差异行点到该 id + 该字段
        rc, lines, _ = pair("field.json", lambda d: d["entries"][0].update(
            {"gate": d["entries"][0]["gate"] + " · scripts/gates/check-nonexistent-xyz.py"}))
        case("③ 负控：改一条字段 ⇒ rc=1（点到 <id> <字段>）", 1, rc, "gate 旧→新", lines)

        # ③ 负控：新增一条 ⇒ rc=1 且印「新增」
        rc, lines, _ = pair("add.json", lambda d: d["entries"].append(
            {"id": "S-z", "name": "夹具新条", "truth": "core/internal/contract/registry.json",
             "truth_shape": "x", "version_field": "schema", "gate": "scripts/gates/check-contract-registry.py",
             "change_class": "B", "change_note": "x"}))
        case("④ 负控：新增一条 ⇒ rc=1 且印「新增」", 1, rc, "S-z 整条 旧→新", lines)

        # ④ 负控：删一条 ⇒ rc=1 且印「删除」
        rc, lines, _ = pair("del.json", lambda d: d["entries"].pop())
        case("⑤ 负控：删一条 ⇒ rc=1 且印「删除」", 1, rc, "删除", lines)

        # ⑤ 负控：改 authority 一个键（顶层整体比）⇒ rc=1 且点到该键路径
        rc, lines, _ = pair("auth.json", lambda d: d["authority"].update({"rule": "夹具改过的规则"}))
        case("⑥ 负控：改 authority 一个键 ⇒ rc=1", 1, rc, "顶层 authority.rule 旧→新", lines)

        # ⑥ 负控：改 change_flow 一处 ⇒ rc=1
        rc, lines, _ = pair("flow.json", lambda d: d["change_flow"].update({"author_not_approver": False}))
        case("⑦ 负控：改 change_flow 一处 ⇒ rc=1", 1, rc, "顶层 change_flow.author_not_approver 旧→新", lines)

        # ⑦ 负控：旧件读不到 ⇒ rc=2（不给结论）
        rc, lines, _ = check(None, real, old_file=os.path.join(tmp, "根本没这份旧件.json"), new_file=oldp)
        case("⑧ 负控：旧件读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

        # ⑧ 负控：新版件读不到 ⇒ rc=2（不给结论）
        rc, lines, _ = check(root, os.path.join(tmp, "根本没这份表.json"))
        case("⑨ 负控：新版件读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

        # ⑨ 负控：基线 rev 取不到 ⇒ rc=2 且逐字印「无上一版可比」（**不许拿工作树当基线**）
        rc, lines, _ = check(root, real, base_rev="0000000000000000000000000000000000000000")
        case("⑩ 负控：基线 rev 取不到 ⇒ rc=2 且印「无上一版可比」", 2, rc, "无上一版可比", lines)

        # ⑩ 负控：根解析不到 ⇒ rc=2（不给结论）
        rc, lines, _ = check(None, real)
        case("⑪ 负控：根解析不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

        # ⑪ 负控：拿一个**没有该件**的临时仓当目标 ⇒ rc=2 且印「无上一版可比」（禁工作树糊过去）
        bare = os.path.join(tmp, "bare")
        os.makedirs(bare, exist_ok=True)
        for cmd in (["init", "-q"], ["config", "user.email", "t@t"], ["config", "user.name", "t"]):
            git(bare, cmd)
        with open(os.path.join(bare, "x.txt"), "w", encoding="utf-8") as fh:
            fh.write("x")
        git(bare, ["add", "x.txt"])
        git(bare, ["commit", "-qm", "init"])
        rc, lines, _ = check(bare, os.path.join(bare, REGISTRY_REL))
        case("⑫ 负控：目标仓里查无该件 ⇒ rc=2（新版件读不到）", 2, rc, "不给结论", lines)

        # ⑬ 正控：真跑墙钟（原型 0.062s 档 · 只印数，不设阈值当门）
        print("   · 真目标墙钟：%d ms（差异行 %d · 旧 %d / 新 %d · verdict=%s）"
              % (info_real["elapsed_ms"], info_real["diff_lines"],
                 info_real["old_entries"], info_real["new_entries"], info_real["verdict"]))
        n += 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if bad:
        print("✗ 自检失败（%d 组断言未过 / 共 %d 组）：" % (len(bad), n))
        for b in bad:
            print("   " + b)
        return 1
    print("✓ 自检全过（%d 组：正控 2 + 负控 11 · 成对）" % n)
    return 0


def main():
    ap = argparse.ArgumentParser(description="契约「版本间对拍」（旧版 ↔ 新版 registry.json · R5/G-27）")
    ap.add_argument("--root", default=None, help="仓库根（默认自动定位：找 %s）" % ROOT_ANCHOR)
    ap.add_argument("--registry", default=None, help="新版表路径（默认 <根>/%s）" % REGISTRY_REL)
    ap.add_argument("--base-rev", default=None, help="基线 rev（默认按口径① 自动取 `git log -1 -- <件>`）")
    ap.add_argument("--old-file", default=None, help="夹具：旧件路径（与 --new-file 成对；不碰 git）")
    ap.add_argument("--new-file", default=None, help="夹具：新件路径")
    ap.add_argument("--json", default=None, help="机器面字段（逗号分隔 · 闭集：%s）" % ",".join(JSON_FIELDS))
    ap.add_argument("--out", default=None, help="另存结果件（默认只打印到 stdout）")
    ap.add_argument("--self-test", "--selftest", dest="self_test", action="store_true",
                    help="只跑自检（合成夹具 · 不碰真目标）")
    a = ap.parse_args()

    if a.json is not None:
        want = [f for f in a.json.split(",") if f]
        unknown = [f for f in want if f not in JSON_FIELDS]
        if not want or unknown:
            print("✗ `--json` 字段表外：%s（可选字段：%s）"
                  % (",".join(unknown) or "（空）", ",".join(JSON_FIELDS)), file=sys.stderr)
            return 2
    if bool(a.old_file) != bool(a.new_file):
        print("✗ `--old-file` 与 `--new-file` 必须成对（夹具档两侧都要给）", file=sys.stderr)
        return 2

    root = find_root(a.root or os.environ.get("ZERG_REPO") or os.path.dirname(os.path.abspath(__file__)))
    if a.self_test:
        print("门 · 契约「版本间对拍」（旧版 ↔ 新版 registry.json · R5/G-27）· 自检档"
              "（真表只读 + 合成夹具 · 不碰真目标）")
        return self_test(root)

    reg = a.registry or (os.path.join(root, REGISTRY_REL) if root else REGISTRY_REL)
    rc, lines, info = check(root, reg, base_rev=a.base_rev,
                            old_file=a.old_file, new_file=a.new_file)
    if a.json is not None:
        print(json.dumps({f: info[f] for f in a.json.split(",") if f in JSON_FIELDS},
                         ensure_ascii=False, sort_keys=True))
        return rc
    print("门 · 契约「版本间对拍」（旧版 ↔ 新版 registry.json · R5/G-27）· 仓根 %s" % root)
    print("  新版件：%s" % (a.new_file if a.new_file else reg))
    if a.old_file:
        print("  旧件（夹具）：%s" % a.old_file)
    for l in lines:
        print("  " + l)
    print("⇒ %s（rc=%d · 墙钟 %d ms）"
          % ({"unchanged": "无变更", "changed": "有差异（报告档 · 不是判红）",
              "blocked": "不给结论"}[info["verdict"]], rc, info["elapsed_ms"]))
    if a.out:
        try:
            with open(a.out, "w", encoding="utf-8") as fh:
                json.dump({"head_sha": info["head_sha"], "base_rev": info["base_rev"],
                           "old_entries": info["old_entries"], "new_entries": info["new_entries"],
                           "diff_lines": info["diff_lines"], "verdict": info["verdict"],
                           "registry_sha256": hashlib.sha256(open(reg, "rb").read()).hexdigest()
                           if os.path.isfile(reg) else "",
                           "lines": lines}, fh, ensure_ascii=False, indent=1)
            print("  结果件已写：%s" % a.out)
        except OSError as e:
            print("  ⛔ 结果件写不了（%s）—— 对拍结论照上（退码不变）" % e, file=sys.stderr)
    return rc


if __name__ == "__main__":
    sys.exit(main())

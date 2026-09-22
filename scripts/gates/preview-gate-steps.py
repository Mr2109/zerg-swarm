#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""preview-gate-steps.py — `R-02`：门禁步骤表**只读预览**（干跑逐条 diff · 2026-09-23 · 波③）

病灶（调研-缺口-门禁证据 §二 `R-02` · `Q-005`）
------------------------------------------------
「改档 / 加步」在业界是**改配置 + code review**（`R-02` 逐字：「声明式 + 可 diff」是它的
**全部价值**）。本仓今天是**手改 `precommit-gates.sh`**：加一步就是在 `build_steps` 的
`case` 分支里插一行 `add_step` —— 改之前**看不到差**（全表会 +N 步？有没有顺手把某步的模式
从 `tri` 改成 `tri-report`？），改之后只能靠「跑一遍看看」。而加步是 **one-way door**：
它动的是**所有人的必经之路**（提交闸）。

只读半件做什么（写面**本波不做** ✗）
------------------------------------
  · **现状真源 = 现跑 `--list`**（脚本自己点名的唯一真源 = 它自己的 `add_step` 行 ——
    本件不另写一份步骤表 ✗；`--list-cache` 可用一次现跑的快照，避免重复跑 15s 的自检）；
  · **提案 = 声明式步骤表**（TSV / JSON：`scope ⇥ mode ⇥ 名称`；注释**只认整行** `#` ——
    名称里可以带 `#`，不许把名称当注释切掉）—— 这就是「改配置」那一半：
    作者把**想要的表**写出来，本件把它与现状**逐条 diff**：
      `+N 步`（将新增，逐条点名）· `-N 步`（将删，逐条点名）· `~N 处`（**模式变化** / scope 变化）；
  · **写面要人签**（`加步 = 动所有人的必经之路`）⇒ 只读半件**没有**写面：
    带任何写旗标 ⇒ 判「未带 `--yes`」或「无人签批准件」⇒ **不写且 rc=2**（两个条件今天恒真 ⇒
    一律 rc=2；这不是「暂时没做」，是**本波的红线**：写面要人签，人签核验器未落地 ⇒ fail-closed）。

判据（可机检 ✓ · 逐条对到 `R-02` 原文）
----------------------------------------
  ① 干跑 ⇒ **逐条 diff**：`+N 步` / `-N 步` / **模式变化** `~N 处`（逐条点名，不是只报个数）；
  ② **只读**：跑前后**目标件**（`scripts/gates/precommit-gates.sh`）`sha256` 一字不变；
  ③ 未带 `--yes` **或** 无人签批准件 ⇒ **不写且 rc=2**（成对：两条**分开**判、分别报）；
  ④ 提案是**声明式**的：模式名必须在闭集里（`rc` / `empty` / `tri` / `tri-report` ——
     表外模式名在闸里会被 `_judge` 硬红）⇒ 表外/缺列/解不动 ⇒ **rc=2 不给结论**（不许猜）。

用法
----
    python3 scripts/gates/preview-gate-steps.py --template            # 把现状打成提案骨架（TSV · 只印不写）
    python3 scripts/gates/preview-gate-steps.py --proposal p.tsv      # 只读预览：逐条 diff（+N / -N / 模式变化）
    python3 scripts/gates/preview-gate-steps.py --proposal p.json --json
    python3 scripts/gates/preview-gate-steps.py --list-cache /tmp/gates-list.txt --proposal p.tsv   # 复用现跑快照
    python3 scripts/gates/preview-gate-steps.py --proposal p.tsv --apply --yes --approval a.json   # 写面 ⇒ rc=2（本波不做）
    python3 scripts/gates/preview-gate-steps.py --self-test           # 合成件成对负控（不碰真目标）

退码（照 `K3`，不新立）
----------------------
    0 = 只读预览跑成（**有没有差都不影响这个码**：差是事实，不是红）
    1 = 保留（本件今天不产红：它判的是「有没有差」与「能不能写」，两者各有各的码）
    2 = **不给结论 / 不写**：真源取不到 · 提案解不动 · 模式名在闭集外 · **任何写意图**
        （未带 `--yes` / 无人签批准件 / 写面本波不做）· 自检不过 · 用法错
"""

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys

DEFAULT_TARGET = os.path.join("scripts", "gates", "precommit-gates.sh")
DEFAULT_LIST_CMD = ["bash", os.path.join("scripts", "gates", "precommit-gates.sh"), "--list"]
MODE_CLOSED = ("rc", "empty", "tri", "tri-report")          # 闸的档位闭集（表外 ⇒ `_judge` 硬红）
LIST_HEADER_RE = re.compile(r"──\s*步骤清单")
LIST_TOTAL_RE = re.compile(r"^共\s*(\d+)\s*步\s*$")
WRITE_FLAGS = ("--write", "--apply", "--commit", "--yes", "--approval")


# ── 现状真源（现跑 `--list` / 快照）─────────────────────────────────────────
def run_list(root, cmd=None):
    """现跑 `--list` ⇒ (输出, 错因|None)。**只读**：`--list` 不跑任何真步骤。"""
    cmd = list(cmd or DEFAULT_LIST_CMD)
    try:
        pr = subprocess.run(cmd, cwd=root, capture_output=True, text=True, timeout=1800)
    except FileNotFoundError as exc:
        return None, "现跑命令起不来（%s）：%s" % (" ".join(cmd), exc)
    except subprocess.TimeoutExpired:
        return None, "现跑命令超时：%s" % " ".join(cmd)
    return (pr.stdout or "") + "\n" + (pr.stderr or ""), None


def parse_list(text):
    """解析 `--list` 输出 ⇒ (steps, 声明总数|None, 错因|None)。steps = [{'scope','mode','name'}]。"""
    lines = text.splitlines()
    start = None
    for i, ln in enumerate(lines):
        if LIST_HEADER_RE.search(ln):
            start = i + 1
            break
    if start is None:
        return [], None, "`--list` 输出里找不到「步骤清单」段（真源取不到）"
    steps, total = [], None
    for ln in lines[start:]:
        s = ln.strip()
        if not s:
            continue
        m = LIST_TOTAL_RE.match(s)
        if m:
            total = int(m.group(1))
            break
        parts = s.split(None, 2)
        if len(parts) != 3:
            return [], None, "步骤行解不动（要 `scope 模式 名称` 三格）：%r" % ln
        steps.append({"scope": parts[0], "mode": parts[1], "name": parts[2]})
    if total is None:
        return [], None, "`--list` 输出里没有「共 N 步」那一行（计数自证做不了）"
    if total != len(steps):
        return [], None, "计数自证不过：现印「共 %d 步」但逐行解出 %d 条 ⇒ 不许当绿" % (total, len(steps))
    return steps, total, None


def load_current(root, args):
    """⇒ (steps, 来源串, 错因|None)。"""
    if args.list_cache:
        try:
            with open(args.list_cache, encoding="utf-8") as f:
                text = f.read()
        except OSError as exc:
            return [], None, "--list-cache 读不到：%s" % exc
        steps, total, err = parse_list(text)
        return steps, "快照 `%s`（共 %d 步）" % (args.list_cache, total or 0), err
    text, err = run_list(root, args.list_cmd)
    if err:
        return [], None, err
    steps, total, err = parse_list(text)
    return steps, "现跑 `%s`（共 %s 步）" % (" ".join(args.list_cmd or DEFAULT_LIST_CMD), total), err


# ── 提案（声明式步骤表）────────────────────────────────────────────────────
def parse_proposal(path):
    """⇒ (steps, 错因|None)。`.tsv` / `.json` 两形；模式名必须在闭集里。"""
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as exc:
        return None, "提案读不到：%s" % exc
    raw = []
    if path.lower().endswith(".json"):
        try:
            doc = json.loads(text)
        except Exception as exc:                                     # noqa: BLE001
            return None, "提案 JSON 解不动：%s" % exc
        items = doc.get("steps") if isinstance(doc, dict) else doc
        if not isinstance(items, list) or not items:
            return None, "提案 JSON 里没有 `steps` 列表（或它是空的）"
        for i, it in enumerate(items, 1):
            if isinstance(it, dict):
                raw.append((it.get("scope"), it.get("mode"), it.get("name"), i))
            elif isinstance(it, str):
                parts = it.split("\t") if "\t" in it else it.split(None, 2)
                raw.append((parts[0] if len(parts) > 0 else None,
                            parts[1] if len(parts) > 1 else None,
                            parts[2] if len(parts) > 2 else None, i))
            else:
                return None, "提案 JSON 第 %d 项不是对象也不是串" % i
    else:
        for i, ln in enumerate(text.splitlines(), 1):
            if ln.lstrip().startswith("#"):        # 注释**只认整行**：名称里可以带 `#`
                continue
            s = ln.rstrip("\n")
            if not s.strip():
                continue
            parts = s.split("\t") if "\t" in s else s.split(None, 2)
            if len(parts) != 3:
                return None, "提案第 %d 行解不动（要 `scope ⇥ mode ⇥ 名称` 三格）：%r" % (i, ln)
            raw.append((parts[0], parts[1], parts[2].strip(), i))
    steps, seen = [], {}
    for scope, mode, name, i in raw:
        if not scope or not mode or not name:
            return None, "提案第 %d 项缺格（scope/mode/名称 三格都要有）" % i
        if mode not in MODE_CLOSED:
            return None, ("提案第 %d 项的模式名在**闭集外**：%r（闭集 = %s —— 表外模式名在闸里会被"
                          "硬红 ⇒ 本件也不放它过）" % (i, mode, " / ".join(MODE_CLOSED)))
        if name in seen:
            return None, "提案里名称重了：%r（第 %d 项与第 %d 项 —— 名称是步骤的身份，重名判不了）" % (name, i, seen[name])
        seen[name] = i
        steps.append({"scope": scope, "mode": mode, "name": name})
    if not steps:
        return None, "提案里 0 步（空表 = 判不了，不许当「全删」）"
    return steps, None


def diff_steps(cur, prop):
    """⇒ (added, removed, changed, notes)。名称是身份；`~` 记模式变化 / scope 变化。"""
    cm = {s["name"]: s for s in cur}
    pm = {s["name"]: s for s in prop}
    added = [pm[n] for n in pm if n not in cm]
    removed = [cm[n] for n in cm if n not in pm]
    changed, notes = [], []
    for n in pm:
        if n not in cm:
            continue
        a, b = cm[n], pm[n]
        if a["mode"] != b["mode"]:
            changed.append((a, b, "模式变化"))
        elif a["scope"] != b["scope"]:
            changed.append((a, b, "scope 变化（同一道门归到别的 scope）"))
    cur_scopes = {s["scope"] for s in cur}
    for s in prop:
        if s["scope"] not in cur_scopes:
            notes.append("提案引入了现表里没有的 scope：`%s`（现表闭集 = %s）—— 那要改 `build_steps` 的 "
                         "`case` 分支（本件只预览，不判；线要不要接见波④）"
                         % (s["scope"], " · ".join(sorted(cur_scopes))))
    return added, removed, changed, sorted(set(notes))


def build_diff(cur, prop, cur_src, prop_path):
    """算差 + 拼逐条打印行。⇒ (added, removed, changed, notes, lines)。"""
    L = []
    L.append("── 门禁步骤表只读预览（`R-02` · **不写任何件**）──")
    L.append("   现状真源：%s" % cur_src)
    L.append("   提案：`%s`（%d 步）" % (prop_path, len(prop)))
    added, removed, changed, notes = diff_steps(cur, prop)
    L.append("   +%d 步（**将新增**）" % len(added))
    for s in added:
        L.append("     + %-8s %-11s %s" % (s["scope"], s["mode"], s["name"]))
    L.append("   -%d 步（**将删**）" % len(removed))
    for s in removed:
        L.append("     - %-8s %-11s %s" % (s["scope"], s["mode"], s["name"]))
    L.append("   ~%d 处（**模式变化 / scope 变化**）" % len(changed))
    for a, b, why in changed:
        if a["mode"] != b["mode"]:
            L.append("     ~ 「%s」模式 %s → %s（scope %s）" % (a["name"], a["mode"], b["mode"], a["scope"]))
        else:
            L.append("     ~ 「%s」scope %s → %s（模式 %s 未变）" % (a["name"], a["scope"], b["scope"], a["mode"]))
    for n in notes:
        L.append("   ⚠ %s" % n)
    net = len(prop) - len(cur)
    L.append("   ⇒ 逐条 diff：**+%d 步 / -%d 步 / 模式变化 %d 处**（净 %+d）"
             % (len(added), len(removed), len(changed), net))
    L.append("   ⇒ 步数：现状 %d → 提案 %d" % (len(cur), len(prop)))
    if not (added or removed or changed):
        L.append("   ⇒ 逐字相同：提案与现状**一步不差**")
    return added, removed, changed, notes, L


def do_template(root, args):
    steps, src, err = load_current(root, args)
    if err:
        print("⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）" % err)
        return 2
    print("# 门禁步骤表提案骨架（现跑真源 = %s · **只印不写**）" % src)
    print("# 一行一步：scope ⇥ mode ⇥ 名称；模式闭集 = %s" % " / ".join(MODE_CLOSED))
    print("# 改完存成分件，`--proposal <件>` 就能出逐条 diff；**写面要人签（本波不做）** ✗")
    for s in steps:
        print("%s\t%s\t%s" % (s["scope"], s["mode"], s["name"]))
    print("# 共 %d 步" % len(steps))
    return 0


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def write_intent_refusal(args, target_path):
    """写意图 ⇒ (rc, 逐条判词)。**一个字节都不写。**"""
    out = []
    if not args.yes:
        out.append("判①：**未带 `--yes`** ⇒ 不写")
    if not args.approval:
        out.append("判②：**无人签批准件**（没给 `--approval <件>`）⇒ 不写")
    elif not os.path.isfile(args.approval):
        out.append("判②：批准件不在（`%s` 读不到）⇒ **没人签** ⇒ 不写" % args.approval)
    if args.yes and args.approval and os.path.isfile(args.approval):
        out.append("判③：`--yes` 与批准件都在 ⇒ 但**写面本波不做**（只读半件 · 加步 = 动所有人的"
                   "必经之路 ⇒ 要人签）⇒ 人签核验器未落地 ⇒ **fail-closed 不写**")
    out.append("目标件（一字未动）：`%s`" % target_path)
    return 2, out


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--proposal", metavar="件", help="提案（声明式步骤表：`.tsv` / `.json`）")
    ap.add_argument("--template", action="store_true", help="把现状打成提案骨架（TSV · **只印不写**）")
    ap.add_argument("--list-cache", metavar="件", help="复用一次现跑 `--list` 的快照（避免重跑自检）")
    ap.add_argument("--list-cmd", metavar="串", help="覆盖现跑命令（默认 `bash scripts/gates/precommit-gates.sh --list`）")
    ap.add_argument("--target", default=DEFAULT_TARGET, help="写面目标（默认提交闸脚本 · 本波**只读**）")
    ap.add_argument("--apply", action="store_true", help="写意图（本波一律 rc=2：写面要人签）")
    ap.add_argument("--write", action="store_true", help="同 `--apply`")
    ap.add_argument("--commit", action="store_true", help="同 `--apply`")
    ap.add_argument("--yes", action="store_true", help="写意图的前置（缺 ⇒ rc=2）")
    ap.add_argument("--approval", metavar="件", help="人签批准件（缺 ⇒ rc=2）")
    ap.add_argument("--root", default=".", help="仓根（默认当前目录）")
    ap.add_argument("--json", action="store_true", help="机器可读输出")
    ap.add_argument("--self-test", action="store_true", help="合成件成对负控（不碰真目标）")
    a = ap.parse_args()

    if a.self_test:
        print("── preview-gate-steps 自检（合成件 · 不碰真目标）──")
        return self_test()

    root = os.path.abspath(a.root)
    target = os.path.join(root, a.target)
    if not os.path.isfile(target):
        print("⛔ 目标件不在：%s ⇒ 不给结论（rc=2）" % a.target)
        return 2
    sha_pre = sha256_file(target)

    intent = a.apply or a.write or a.commit or a.yes or bool(a.approval)
    if intent:
        rc, lines = write_intent_refusal(a, a.target)
        for ln in lines:
            print("   ⛔ %s" % ln)
        print("⇒ **不写**（rc=%d）：写面要人签 ⇒ 本波只做**只读半件**" % rc)
        print("   只读性自证：目标件 `%s` sha256 %s…（跑前） = %s…（跑后）⇒ %s"
              % (a.target, sha_pre[:12], sha256_file(target)[:12],
                 "一字不变 ✓" if sha256_file(target) == sha_pre else "**变了 ⇒ 判红** ✗"))
        return rc

    if a.template and not a.proposal:
        return do_template(root, a)

    if not a.proposal:
        steps, src, err = load_current(root, a)
        if err:
            print("⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）" % err)
            return 2
        print("── 门禁步骤表（只读预览 · 无提案）──")
        print("   真源：%s" % src)
        print("   ⇒ %d 步（要看差请给 `--proposal <件>`；要提案骨架用 `--template`）" % len(steps))
        print("   ⇒ 只读（目标件 sha256 %s… · 一字未动）" % sha_pre[:12])
        return 0

    steps, src, err = load_current(root, a)
    if err:
        print("⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）" % err)
        return 2
    prop, perr = parse_proposal(a.proposal)
    if perr:
        print("⛔ 提案解不动：%s ⇒ 不给结论（rc=2）" % perr)
        return 2
    added, removed, changed, notes, lines = build_diff(steps, prop, src, a.proposal)
    if not a.json:
        for ln in lines:
            print(ln)
    else:
        print(json.dumps({"id": "gate-steps-diff.v1", "current_source": src, "proposal": a.proposal,
                          "steps_current": len(steps), "steps_proposal": len(prop or []),
                          "added": added, "removed": removed,
                          "changed": [{"name": b["name"], "from_mode": x["mode"], "to_mode": b["mode"],
                                       "from_scope": x["scope"], "to_scope": b["scope"]} for x, b, _ in changed],
                          "notes": notes}, ensure_ascii=False, indent=2))
    sha_post = sha256_file(target)
    print("   只读性自证：目标件 `%s` sha256 %s… → %s… ⇒ %s"
          % (a.target, sha_pre[:12], sha_post[:12], "一字不变 ✓" if sha_post == sha_pre else "**变了 ⇒ 判红** ✗"))
    print("⇒ 只读预览跑成（rc=0）：差是事实；**写面要人签 ⇒ 本波不做** ✗")
    return 0


# ── 自检（合成件 · 不碰真目标）───────────────────────────────────────────────
FX_LIST = """自检结论: 全过（断言 3 条 · 失败 0 条）
── 步骤清单（scope 模式 名称）──
   go    empty  gofmt -l core
   gates tri    门① 覆盖（阻断）
   gates tri    门② 版本源（阻断）
   docs  tri-report docs: glossary 术语表 zh+en（只报告）
共 4 步
"""


def self_test():
    import shutil
    import tempfile

    fails = []

    def case(name, got, want):
        if got == want:
            print("   ✓ %s" % name)
        else:
            print("   ✗ %s（got=%r want=%r）" % (name, got, want))
            fails.append(name)

    def case_true(name, ok):
        case(name, bool(ok), True)

    tmp = tempfile.mkdtemp(prefix="zerg-preview-steps-selftest-")
    try:
        cur = os.path.join(tmp, "list.txt")
        with open(cur, "w", encoding="utf-8") as f:
            f.write(FX_LIST)

        # ① 真源解析 + 计数自证
        steps, total, err = parse_list(FX_LIST)
        case("① `--list` 解出 4 步", (len(steps), total, err), (4, 4, None))
        case("① 三格逐条解对（含中文名里的空格）",
             steps[1], {"scope": "gates", "mode": "tri", "name": "门① 覆盖（阻断）"})
        bad = FX_LIST.replace("共 4 步", "共 5 步")
        case_true("① 负控：现印步数与逐行解出的条数不符 ⇒ 判不了", bool(parse_list(bad)[2]))
        case_true("① 负控：没有「步骤清单」段 ⇒ 判不了", bool(parse_list("nothing here\n")[2]))

        # ② 提案：TSV / JSON 两形
        p_tsv = os.path.join(tmp, "p.tsv")
        with open(p_tsv, "w", encoding="utf-8") as f:
            f.write("# 提案：加一步 · 删一步 · 改一档\n"
                    "go\tempty\tgofmt -l core\n"
                    "gates\ttri\t门① 覆盖（阻断）\n"
                    "gates\ttri\t门② 版本源（阻断）\n"           # docs 那步被删（没写它）
                    "gates\ttri-report\t门㉑ 新门：只报告起步\n")   # 新增
        prop, perr = parse_proposal(p_tsv)
        case("② TSV 提案解出 4 步", (len(prop) if prop else None, perr), (4, None))
        steps0, _, _ = parse_list(FX_LIST)
        added, removed, changed, notes = diff_steps(steps0, prop)
        case("② 逐条 diff：+1 步", [s["name"] for s in added], ["门㉑ 新门：只报告起步"])
        case("② 逐条 diff：-1 步", [s["name"] for s in removed], ["docs: glossary 术语表 zh+en（只报告）"])

        # ③ 模式变化（同名字 · 同 scope · 只改档）
        p_mode = os.path.join(tmp, "mode.tsv")
        with open(p_mode, "w", encoding="utf-8") as f:
            f.write("go\tempty\tgofmt -l core\n"
                    "gates\ttri-report\t门① 覆盖（阻断）\n"       # tri ⇒ tri-report（放宽）
                    "gates\ttri\t门② 版本源（阻断）\n"
                    "docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n")
        prop2, _ = parse_proposal(p_mode)
        a2, r2, c2, _ = diff_steps(steps0, prop2)
        case("③ 模式变化被点出（1 处）", len(c2), 1)
        case("③ 变化那一步点名到「模式变化」", (c2[0][2], c2[0][0]["mode"], c2[0][1]["mode"]),
             ("模式变化", "tri", "tri-report"))
        case("③ 模式变化**不算**增删（+0/-0）", (len(a2), len(r2)), (0, 0))

        # ④ scope 变化（同一道门归到别的 scope）
        p_scope = os.path.join(tmp, "scope.tsv")
        with open(p_scope, "w", encoding="utf-8") as f:
            f.write("go\tempty\tgofmt -l core\n"
                    "release\ttri\t门① 覆盖（阻断）\n"
                    "gates\ttri\t门② 版本源（阻断）\n"
                    "docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n")
        prop3, _ = parse_proposal(p_scope)
        a3, r3, c3, _ = diff_steps(steps0, prop3)
        case("④ scope 变化被点出", (len(c3), c3[0][2]), (1, "scope 变化（同一道门归到别的 scope）"))

        # ⑤ 逐字相同 ⇒ 一步不差
        p_same = os.path.join(tmp, "same.tsv")
        with open(p_same, "w", encoding="utf-8") as f:
            f.write("go\tempty\tgofmt -l core\n"
                    "gates\ttri\t门① 覆盖（阻断）\n"
                    "gates\ttri\t门② 版本源（阻断）\n"
                    "docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n")
        prop4, _ = parse_proposal(p_same)
        a4, r4, c4, _ = diff_steps(steps0, prop4)
        case("⑤ 逐字相同 ⇒ +0/-0/变化 0", (len(a4), len(r4), len(c4)), (0, 0, 0))

        # ⑥ 负控：模式名在闭集外 ⇒ 判不了（照闸里 `_judge` 会硬红的口径）
        p_badmode = os.path.join(tmp, "badmode.tsv")
        with open(p_badmode, "w", encoding="utf-8") as f:
            f.write("gates\t阻断\t门① 覆盖（阻断）\n")
        _, perr = parse_proposal(p_badmode)
        case_true("⑥ 负控：模式名在闭集外 ⇒ 判不了", bool(perr))
        # ⑦ 负控：缺列 / 重名 / 空表
        p_miss = os.path.join(tmp, "miss.tsv")
        with open(p_miss, "w", encoding="utf-8") as f:
            f.write("gates\ttri\n")
        case_true("⑦ 负控：缺格 ⇒ 判不了", bool(parse_proposal(p_miss)[1]))
        p_dup = os.path.join(tmp, "dup.tsv")
        with open(p_dup, "w", encoding="utf-8") as f:
            f.write("gates\ttri\t门① 覆盖（阻断）\ngates\ttri\t门① 覆盖（阻断）\n")
        case_true("⑦ 负控：重名 ⇒ 判不了", bool(parse_proposal(p_dup)[1]))
        p_empty = os.path.join(tmp, "empty.tsv")
        open(p_empty, "w").close()
        case_true("⑦ 负控：空表 ⇒ 判不了（不许当「全删」）", bool(parse_proposal(p_empty)[1]))
        p_json = os.path.join(tmp, "p.json")
        with open(p_json, "w", encoding="utf-8") as f:
            json.dump({"id": "proposal.v1", "steps": [{"scope": "gates", "mode": "tri", "name": "门① 覆盖（阻断）"}]}, f)
        case("⑦ JSON 提案同解", len(parse_proposal(p_json)[0] or []), 1)
        p_json_bad = os.path.join(tmp, "bad.json")
        with open(p_json_bad, "w", encoding="utf-8") as f:
            f.write("{不是 json")
        case_true("⑦ 负控：JSON 解不动 ⇒ 判不了", bool(parse_proposal(p_json_bad)[1]))

        # ⑧ 写意图三档：**分离判**，一律 rc=2，且目标件一字不动
        tgt = os.path.join(tmp, "fake-gate.sh")
        with open(tgt, "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\nadd_step gates \"门①\" tri . true\n")
        sh = sha256_file(tgt)
        ns = lambda **kw: argparse.Namespace(**{**dict(
            proposal=None, template=False, list_cache=None, list_cmd=None, root=tmp,
            target="fake-gate.sh", apply=False, write=False, commit=False, yes=False,
            approval=None, json=False, self_test=False), **kw})
        rc, lines = write_intent_refusal(ns(apply=True), "fake-gate.sh")
        case("⑧ 写意图（未带 --yes）⇒ rc=2", rc, 2)
        case_true("⑧ 判词点明「未带 --yes」", any("未带 `--yes`" in x for x in lines))
        rc, lines = write_intent_refusal(ns(apply=True, yes=True), "fake-gate.sh")
        case("⑧ 写意图（有 --yes · 无批准件）⇒ rc=2", rc, 2)
        case_true("⑧ 判词点明「无人签批准件」", any("无人签批准件" in x for x in lines))
        ap_file = os.path.join(tmp, "approval.json")
        with open(ap_file, "w", encoding="utf-8") as f:
            f.write('{"signed_by": "没人", "note": "合成件"}\n')
        rc, lines = write_intent_refusal(ns(apply=True, yes=True, approval=ap_file), "fake-gate.sh")
        case("⑧ 写意图（--yes + 批准件都在）⇒ **仍 rc=2**（写面本波不做 · fail-closed）", rc, 2)
        case_true("⑧ 判词点明「写面本波不做」", any("写面本波不做" in x for x in lines))
        case("⑧ 三档都跑完：目标件 sha256 一字不变", sha256_file(tgt), sh)

        # ⑩ 名称里带 `#` **不许**被当注释截断（首跑实测的假差：`#!` 那一步被切成了「首行」）
        p_hash = os.path.join(tmp, "hash.tsv")
        with open(p_hash, "w", encoding="utf-8") as f:
            f.write("# 整行注释\n"
                    "gates\ttri\tscripts/*（无后缀 + 首行 #!）按 shebang 语法\n")
        prop5, perr5 = parse_proposal(p_hash)
        case("⑩ 名称里的 `#` 不被截断", prop5[0]["name"] if prop5 else perr5,
             "scripts/*（无后缀 + 首行 #!）按 shebang 语法")

        # ⑨ 只读预览真跑一遍（用合成快照 + 合成提案）· 前后件 sha 不变
        rc = main_argv(["--root", tmp, "--list-cache", cur, "--proposal", p_tsv,
                        "--target", "fake-gate.sh"])
        case("⑨ 只读预览 ⇒ rc=0", rc, 0)
        case("⑨ 只读预览跑完：目标件 sha256 一字不变", sha256_file(tgt), sh)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("自检结论: %s（失败 %d 条）" % ("全过" if not fails else "**有失败**", len(fails)))
    return 0 if not fails else 2


def main_argv(argv):
    """自检里按 argv 跑一遍 main()（不碰真目标：目标/真源都是合成件）。"""
    old = sys.argv
    try:
        sys.argv = ["preview-gate-steps.py"] + list(argv)
        return main()
    finally:
        sys.argv = old


if __name__ == "__main__":
    sys.exit(main())

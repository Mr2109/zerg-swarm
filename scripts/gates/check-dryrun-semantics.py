#!/usr/bin/env python3
# check-dryrun-semantics.py — 门⑩：`--dry-run` **一套语义**（§二十一 已红第 16 条 · 开工单 T-38）。
#
# 病灶（照已红原话）：全仓 `dry-run` = 117 行 / 18 件、**四套语义**（迁移预演 / 采样判词与执行分离 /
# 收卵前静态判据模拟 / 脚本侧各写各的）。四套语义的病不是「名字不同」，而是**下次加一处 dry-run 时
# 没有任何东西会红** —— 于是第五套、第六套照旧。
#
# 本门禁把「一套语义」变成可机检的四条：
#   R1 **覆盖**：扫描命令命中的件，逐件必须在登记表里（未登记 ⇒ 红 —— 这就是「不许再有第五套」）；
#   R2 **幽灵**：登记表里的件，必须在现跑命中里（登记了但件已消失 ⇒ 红 —— 台账不许挂在空气上）；
#   R3 **字段**：逐件 `语义` 必须 = 顶层唯一 id（`dryrun.v1`，不许第二套）+ `零副作用判据` 非空（就一句话，得有）；
#   R4 **单一定义**：顶层 id/一句话/零副作用定义齐全，且**归并前的四套语义**必须留档（只报不看 ≠ 没归并过）。
# 空转（扫描命中 0 件 / 登记 0 件）⇒ rc=2（不给结论），不许当绿。
#
# 用法：
#   python3 scripts/gates/check-dryrun-semantics.py              # 真目标（仓根）
#   python3 scripts/gates/check-dryrun-semantics.py --root <dir> # 指根（自检夹具用）
#   python3 scripts/gates/check-dryrun-semantics.py --self-test  # 1 正控 + 6 负控
#
# 退码：0 全绿 · 1 红（R1–R4 任一）· 2 不给结论（空转/前置缺件）

import argparse
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates → 仓根
REGISTRY_REL = os.path.join("core", "internal", "contract", "dryrun-semantics.json")
SCAN_DIRS = ["core", "agent", "scripts", "wall", "gateway", "ui"]
SCAN_EXTS = ["*.go", "*.sh", "*.py", "*.rs"]
SKIP_DIRS = {"venv", ".venv"}
# 排除面（**判据面自身，不是 dry-run 语义站点**）：本门禁脚本里必然出现 `dry-run` 字样（它就是判据文本）。
# 口径：按**文件名**排除（窄排除，不按目录/正则）—— 成对负控见自检②（另加一件立刻红）。
SELF = os.path.basename(os.path.abspath(__file__))

SEMANTICS_ID = "dryrun.v1"


def scan_hits(root):
    """现跑扫描（与登记表里 `_扫描命令` 同一条口径）。"""
    cmd = ["grep", "-r", "-E", "dry.?run|DryRun"]
    for e in SCAN_EXTS:
        cmd.append("--include=" + e)
    for d in SKIP_DIRS:
        cmd.append("--exclude-dir=" + d)
    cmd.append("--exclude=" + SELF)   # 判据面自身（见上）
    cmd += [d for d in SCAN_DIRS if os.path.isdir(os.path.join(root, d))]
    if len(cmd) <= 5 + len(SKIP_DIRS):
        return None, None  # 一个扫描面都不存在 = 空转
    p = subprocess.run(cmd, cwd=root, capture_output=True, text=True)
    if p.returncode not in (0, 1):  # grep: 0=有命中 1=无命中
        return None, p.stderr.strip()
    files, lines = set(), 0
    for ln in p.stdout.splitlines():
        lines += 1
        f = ln.split(":", 1)[0]
        files.add(os.path.normpath(f))
    return (files, lines), None


def check(root, registry, hits):
    """四格判据 → （问题列表, 计数）。"""
    problems = []
    files, nlines = hits

    # R4 单一定义
    rid = registry.get("id")
    if rid != SEMANTICS_ID:
        problems.append("R4 顶层 id 应为 %s，实为 %r（不许第二套语义）" % (SEMANTICS_ID, rid))
    for k in ("_一句话", "_零副作用定义"):
        if not (registry.get(k) or "").strip():
            problems.append("R4 顶层缺 %s（一套语义得有定义）" % k)
    if len(registry.get("_归并前的四套语义") or []) < 4:
        problems.append("R4 「归并前的四套语义」必须留档 4 条（只报不看 ≠ 没归并过）")

    entries = registry.get("件") or []
    reg = {}
    for e in entries:
        f = os.path.normpath(e.get("file", ""))
        if not f:
            problems.append("R3 有登记项缺 file 字段")
            continue
        if f in reg:
            problems.append("R3 登记项重复：%s" % f)
        reg[f] = e

    # R3 字段
    for f, e in sorted(reg.items()):
        if e.get("语义") != SEMANTICS_ID:
            problems.append("R3 %s 的语义应为 %s，实为 %r" % (f, SEMANTICS_ID, e.get("语义")))
        judge = (e.get("零副作用判据") or "").strip()
        if len(judge) < 10:
            problems.append("R3 %s 的「零副作用判据」为空/太短（一句话得有）" % f)

    # R1 覆盖 / R2 幽灵
    missing = sorted(files - set(reg))
    ghost = sorted(set(reg) - files)
    for f in missing:
        problems.append("R1 未登记件（= 又一套语义）：%s" % f)
    for f in ghost:
        problems.append("R2 幽灵登记（件已消失，台账挂在空气上）：%s" % f)

    counts = {
        "现跑件": len(files),
        "现跑行": nlines,
        "登记件": len(reg),
        "未登记": len(missing),
        "幽灵": len(ghost),
    }
    return problems, counts


def run(root, quiet=False):
    reg_path = os.path.join(root, REGISTRY_REL)
    if not os.path.isfile(reg_path):
        print("R0 登记表不存在：%s（空转 ⇒ 不给结论）" % reg_path)
        return 2
    with open(reg_path, encoding="utf-8") as fh:
        try:
            registry = json.load(fh)
        except json.JSONDecodeError as exc:
            print("R0 登记表不是合法 JSON：%s" % exc)
            return 1
    hits, err = scan_hits(root)
    if hits is None:
        print("R0 扫描面为空/不可跑（空转 ⇒ 不给结论）%s" % (("：" + err) if err else ""))
        return 2
    files, nlines = hits
    if len(files) == 0 or len(registry.get("件") or []) == 0:
        print("R0 空转：现跑命中 %d 件 / 登记 %d 件 ⇒ 不给结论" % (len(files), len(registry.get("件") or [])))
        return 2
    problems, counts = check(root, registry, hits)
    if not quiet:
        print("── 门⑩ `--dry-run` 一套语义（§二十一 已红第 16 条 · T-38）──")
        print("  根: %s" % root)
        print("  语义 id: %s · 现跑 %d 行 / %d 件 · 登记 %d 件"
              % (registry.get("id"), counts["现跑行"], counts["现跑件"], counts["登记件"]))
        print("  排除面（判据面自身）: %s" % SELF)
        for p in problems:
            print("  · FAIL " + p)
        if not problems:
            print("⇒ 全绿（rc=0）：现跑件全部登记 · 无幽灵 · 逐件判词非空 · 单一定义")
        else:
            print("⇒ 门禁红（rc=1）：%d 条（未登记 %d · 幽灵 %d）"
                  % (len(problems), counts["未登记"], counts["幽灵"]))
    return 1 if problems else 0


# ── 自检（1 正控 + 6 负控）──────────────────────────────────────────────

GOOD_REG = {
    "schema": "zerg/dryrun-semantics/v1",
    "id": "dryrun.v1",
    "_一句话": "干跑 = 只出计划/只算不执行，状态逐字不变",
    "_零副作用定义": "不落盘 · 不删 · 不推 · 不发消息 · 不造目录",
    "_归并前的四套语义": ["迁移预演", "采样判词与执行分离", "收卵前静态判据模拟", "脚本侧各写各的"],
    "件": [
        {"file": "core/a.go", "类": "命令面", "语义": "dryrun.v1", "零副作用判据": "跑前后 git status 逐字不变"},
        {"file": "scripts/b.sh", "类": "脚本侧", "语义": "dryrun.v1", "零副作用判据": "只打印，不 ssh、不推送"},
    ],
}


def _mk(root, reg, files):
    os.makedirs(os.path.join(root, "core", "internal", "contract"), exist_ok=True)
    for rel in files:
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write("# dry-run 夹具\n" + rel + "\n")
    with open(os.path.join(root, REGISTRY_REL), "w", encoding="utf-8") as fh:
        json.dump(reg, fh, ensure_ascii=False)


def self_test():
    ok = True
    cases = []

    # ① 正控：好件 ⇒ rc=0
    with tempfile.TemporaryDirectory() as d:
        _mk(d, GOOD_REG, ["core/a.go", "scripts/b.sh"])
        rc = run(d, quiet=True)
        cases.append(("正控① 好件：逐件登记 + 判词齐 + 单一定义 ⇒ rc=0", 0, rc))

    # ② 负控：多出一件没登记 ⇒ rc=1（= 「不许再有第五套」）
    with tempfile.TemporaryDirectory() as d:
        _mk(d, GOOD_REG, ["core/a.go", "scripts/b.sh", "agent/c.go"])
        rc = run(d, quiet=True)
        cases.append(("负控② 未登记件（又多一套语义）⇒ rc=1", 1, rc))

    # ③ 负控：登记了但件没了 ⇒ rc=1
    with tempfile.TemporaryDirectory() as d:
        _mk(d, GOOD_REG, ["core/a.go"])  # scripts/b.sh 消失
        rc = run(d, quiet=True)
        cases.append(("负控③ 幽灵登记（件已消失）⇒ rc=1", 1, rc))

    # ④ 负控：第二套语义 id ⇒ rc=1
    reg = json.loads(json.dumps(GOOD_REG))
    reg["件"][1]["语义"] = "dryrun.v2"
    with tempfile.TemporaryDirectory() as d:
        _mk(d, reg, ["core/a.go", "scripts/b.sh"])
        rc = run(d, quiet=True)
        cases.append(("负控④ 一件写成第二套语义（dryrun.v2）⇒ rc=1", 1, rc))

    # ⑤ 负控：判词空 ⇒ rc=1
    reg = json.loads(json.dumps(GOOD_REG))
    reg["件"][0]["零副作用判据"] = ""
    with tempfile.TemporaryDirectory() as d:
        _mk(d, reg, ["core/a.go", "scripts/b.sh"])
        rc = run(d, quiet=True)
        cases.append(("负控⑤ 零副作用判据为空（没判词）⇒ rc=1", 1, rc))

    # ⑥ 负控：空转（扫描 0 命中）⇒ rc=2
    with tempfile.TemporaryDirectory() as d:
        _mk(d, GOOD_REG, [])
        rc = run(d, quiet=True)
        cases.append(("负控⑥ 一件都没扫到（空转）⇒ rc=2", 2, rc))

    # ⑦ 负控：登记表缺件 ⇒ rc=2
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core"), exist_ok=True)
        with open(os.path.join(d, "core", "a.go"), "w", encoding="utf-8") as fh:
            fh.write("# dry-run\n")
        rc = run(d, quiet=True)
        cases.append(("负控⑦ 登记表缺件 ⇒ rc=2", 2, rc))

    print("── 自检（1 正控 + 6 负控 · 全部合成夹具，不碰真目标）──")
    for name, want, got in cases:
        mark = "✓" if want == got else "✗"
        if want != got:
            ok = False
        print("  %s %s（rc=%d）" % (mark, name, got))
    print("自检结论: %s" % ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True, description="门⑩：dry-run 一套语义")
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--quiet", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    return run(a.root, quiet=a.quiet)


if __name__ == "__main__":
    sys.exit(main())

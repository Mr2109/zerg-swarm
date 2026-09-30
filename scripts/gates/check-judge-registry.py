#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-judge-registry.py —— 判据注册表（M1-04）真源生成 ＋ 双向差集自检（M1-05）· fail-closed

本件是「判据 CLI 化」M1 开卷前置①② 的落点（设计稿 `设计-判据 CLI 化-v1.7-20260930.md` 第六节 · 9.2 前置①）：

  ① 注册表真源 = `scripts/gates/judge-registry.json`（**由本件 `--emit` 现读生成** · ✗ 不手抄名录）；
  ② 双向差集 = 注册表 ⇄ 盘上件，**两方向缺一即红**（fail-closed）：
       · 盘上有件、表里没点名  ⇒ 红（新件没登记）
       · 表里点名、盘上无件    ⇒ 红（悬空登记）
     另附三条列面机检：件名 ⇒ 路径 越出 `<仓根>/scripts/` 前缀 ⇒ 红；件名 ≠ 件名去扩展名逐字相同 ⇒ 红；
     逐件 `sha256` 与件现读不符（缓存键过期）⇒ 红。

名录真源**同源不手抄**：与转发壳 `core/cmd/zerg/family_gate_check.go` 的 `gateCheckRoster` 同口径 ——
递归 `scripts/` 全子树，只收 `.py` / `.sh`（`__pycache__` 不进名录），判据名 = 文件名去扩展名。
⇒ 注册表**不另立第二套真源**（§6 落点承拍定第四条）。

列面（§6 第六节 · 逐条）：`id`（必填）· `name`（= 件名去扩展名逐字）· `path`（`<仓根>/scripts/` 前缀内）·
`cwd`（缺省仓根 `.`）· `interp`（`.py`⇒`python3` / `.sh`⇒`bash`）· `needs_env`（默认 `false`）·
`serial`（默认 `false` · `P18`）· `hazard`（列名唯一取 `hazard` · `P9`）· `dry_run`（`none`/`argv`/`noop` · 默认 `none`）·
`sha256` ＋ `tool_version`（缓存键 · `P19`）· `cache_key` = `<sha256>|<tool_version>`。
schema 名／版本／`min_tool_version` 写进件头（§9.2 前置①）。

退码（三档 · fail-closed）：
  0 = 对拍通过 / `--emit` 写成 / `--self-test` 全过
  1 = 双向差集非空 / 违反列面 / 缓存键过期（**红**）
  2 = 用法错（三选一都没给）/ 注册表读不到 / schema 不符（加载期硬错 · `P20`）/ 名录读不到 / `--self-test` 不过

用法：
    python3 scripts/gates/check-judge-registry.py --emit [--out <件>]        # 现读生成注册表
    python3 scripts/gates/check-judge-registry.py --check [--registry <件>]  # 双向差集对拍
    python3 scripts/gates/check-judge-registry.py --self-test                # 成对负控自检
"""
import argparse
import hashlib
import json
import os
import platform
import subprocess
import sys
import tempfile

SCHEMA = "judge-registry/1"
MIN_TOOL_VERSION = {"bash": "3.2", "python": "3.9"}
ROSTER_DIR = "scripts"
INTERP = {".py": "python3", ".sh": "bash"}
REGISTRY_DEFAULT = "scripts/gates/judge-registry.json"

# 仓根 = 本件（scripts/gates/xxx.py）上溯三层。
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

_BASH_VER = None


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def tool_version(interp):
    """现读解释器版本（✗ 不猜 · 取不到就如实记 `unknown`）。"""
    global _BASH_VER
    if interp == "python3":
        return "python3/" + platform.python_version()
    if interp == "bash":
        if _BASH_VER is None:
            try:
                out = subprocess.run(["bash", "--version"], capture_output=True, text=True, timeout=10)
                first = (out.stdout or "").splitlines()[0] if out.stdout else ""
                # 形如 `GNU bash，版本 3.2.57(1)-release (…)` / `GNU bash, version 3.2.57(...)`
                tok = ""
                for part in first.replace("，", " ").split():
                    if part[:1].isdigit():
                        tok = part.split("(")[0]
                        break
                _BASH_VER = tok or "unknown"
            except Exception:
                _BASH_VER = "unknown"
        return "bash/" + _BASH_VER
    return interp + "/unknown"


def scan_roster(root):
    """现读名录：递归 `scripts/` 全子树，只收 `.py` / `.sh`（`__pycache__` 不进）⇒ {名: [仓相对路径…]}。

    与转发壳 `gateCheckRoster` **同口径**（判据名 = 文件名去扩展名 · 逐字相同）。
    """
    base = os.path.join(root, ROSTER_DIR)
    paths = {}
    for dp, dns, fns in os.walk(base):
        dns[:] = sorted(d for d in dns if d != "__pycache__")
        for fn in sorted(fns):
            ext = os.path.splitext(fn)[1]
            if ext not in INTERP:
                continue
            name = fn[: -len(ext)]
            rel = os.path.relpath(os.path.join(dp, fn), root).replace(os.sep, "/")
            paths.setdefault(name, []).append(rel)
    for v in paths.values():
        v.sort()
    return paths


def build_registry(root):
    """现读生成注册表（确定性：条目按 name 排序 · 键序固定）。"""
    paths = scan_roster(root)
    dupes = sorted(n for n, v in paths.items() if len(v) > 1)
    entries = []
    for name in sorted(paths):
        if len(paths[name]) != 1:
            continue
        rel = paths[name][0]
        ext = os.path.splitext(rel)[1]
        interp = INTERP[ext]
        tv = tool_version(interp)
        sha = sha256_file(os.path.join(root, rel))
        entries.append({
            "id": name,
            "name": name,
            "path": rel,
            "cwd": ".",
            "interp": interp,
            "needs_env": False,
            "serial": False,
            "hazard": "none",
            "dry_run": "none",
            "sha256": sha,
            "tool_version": tv,
            "cache_key": sha + "|" + tv,
        })
    return {
        "schema": SCHEMA,
        "generated_by": "scripts/gates/check-judge-registry.py --emit",
        "roster_dir": ROSTER_DIR,
        "extensions": sorted(INTERP.keys()),
        "min_tool_version": MIN_TOOL_VERSION,
        "invocation": {
            "argv": "passthrough",
            "expected_rc": "passthrough",
            "note": "入口参数与期望退码由转发壳逐字透传（`zerg gate check <判据名> [其余逐字给子件]`）；"
                    "0/1/2 三码透传，件自身崩/超时 ⇒ 壳 8 · judge_broken/timeout。",
        },
        "count": len(entries),
        "dupes": dupes,
        "entries": entries,
    }


def _problem_list(root, reg):
    """双向差集 ＋ 列面机检 ⇒ 问题清单（空 = 绿）。"""
    problems = []
    live = scan_roster(root)
    live_dupes = sorted(n for n, v in live.items() if len(v) > 1)
    for n in live_dupes:
        problems.append("盘上同名多件（名字 ⇒ 件 不唯一）：%s（%s）" % (n, " · ".join(live[n])))

    reg_entries = {}
    for e in reg.get("entries", []):
        if not isinstance(e, dict) or "name" not in e:
            problems.append("注册表条目缺 `name`：%r" % (e,))
            continue
        if e["name"] in reg_entries:
            problems.append("注册表内同名重复条目：%s" % e["name"])
        reg_entries[e["name"]] = e

    # 反方向：盘上有件、表里没点名
    for n in sorted(live):
        if n not in reg_entries:
            problems.append("盘上有件、注册表**没点名**（新件未登记）：%s（%s）"
                            % (n, " · ".join(live[n])))
    # 正方向：表里点名、盘上无件
    for n in sorted(reg_entries):
        if n not in live:
            problems.append("注册表**点名**、盘上无件（悬空登记）：%s（%s）"
                            % (n, reg_entries[n].get("path", "?")))

    # 列面机检（只对两侧都在的条目）
    for n in sorted(set(live) & set(reg_entries)):
        e = reg_entries[n]
        rel = e.get("path", "")
        if not isinstance(rel, str) or not rel.startswith(ROSTER_DIR + "/"):
            problems.append("件名 ⇒ 路径越出 `<仓根>/%s/` 前缀：%s（%s）" % (ROSTER_DIR, n, rel))
            continue
        base = os.path.splitext(os.path.basename(rel))[0]
        if base != n:
            problems.append("判据名 ≠ 件名去扩展名逐字相同：表内 %s ⇄ 件 %s" % (n, rel))
        if len(live[n]) == 1 and rel != live[n][0]:
            problems.append("表内路径与盘上现读不符：%s ⇄ %s" % (rel, live[n][0]))
        full = os.path.join(root, rel)
        if not os.path.isfile(full):
            problems.append("表内点名的件在盘上不存在：%s" % rel)
            continue
        sha = sha256_file(full)
        if sha != e.get("sha256"):
            problems.append("缓存键过期（sha256 不符 · 请 `--emit` 重生成）：%s" % rel)
    return problems


def load_registry(path):
    with open(path, encoding="utf-8") as f:
        reg = json.load(f)
    if not isinstance(reg, dict) or reg.get("schema") != SCHEMA:
        raise ValueError("schema 不符（须 `%s` · 现读 %r）" % (SCHEMA, (reg or {}).get("schema")
                                                              if isinstance(reg, dict) else reg))
    return reg


def cmd_emit(root, out):
    reg = build_registry(root)
    text = json.dumps(reg, ensure_ascii=False, indent=2) + "\n"
    if out == "-":
        sys.stdout.write(text)
    else:
        d = os.path.dirname(out)
        if d and not os.path.isdir(d):
            os.makedirs(d, exist_ok=True)
        with open(out, "w", encoding="utf-8") as f:
            f.write(text)
        print("判据注册表已生成：%s（条目 %d · 同名歧义 %d）"
              % (out, reg["count"], len(reg["dupes"])))
    if reg["dupes"]:
        sys.stderr.write("⚠ 盘上有同名多件（%d）：%s\n" % (len(reg["dupes"]), " · ".join(reg["dupes"])))
    return 0


def cmd_check(root, reg_path):
    try:
        reg = load_registry(reg_path)
    except ValueError as e:
        print("⛔ 注册表加载期硬错：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    except OSError as e:
        print("⛔ 注册表读不到：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
        return 2
    problems = _problem_list(root, reg)
    if problems:
        print("✗ 判据注册表对拍**红**（双向差集/列面 %d 条）：" % len(problems), file=sys.stderr)
        for p in problems:
            print("  · " + p, file=sys.stderr)
        return 1
    print("✓ 判据注册表对拍绿（rc=0）：条目 %d · 双向差集空 · 缓存键新鲜" % reg.get("count", 0))
    return 0


def _write(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as f:
        f.write(text)


def cmd_self_test():
    """成对负控：合成夹具上，正控绿 + 四条负控各自必红。"""
    ok = True
    with tempfile.TemporaryDirectory(prefix="judge-reg-self-") as root:
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")
        _write(root, "scripts/b.sh", "#!/usr/bin/env bash\necho b\n")
        _write(root, "scripts/sub/c.py", "#!/usr/bin/env python3\nprint('c')\n")
        _write(root, "scripts/gates/__pycache__/ignored.py", "# 缓存件，不进名录\n")

        reg = build_registry(root)
        reg_path = os.path.join(root, "reg.json")
        with open(reg_path, "w", encoding="utf-8") as f:
            json.dump(reg, f, ensure_ascii=False)

        def fails(label, mutate, expect_red=True):
            nonlocal ok
            r2 = build_registry(root)
            mutate()
            probs = _problem_list(root, r2 if expect_red and label.startswith("pos") else reg)
            red = len(probs) > 0
            # 正控：未扰动 ⇒ 应绿
            if label == "pos":
                if red:
                    print("✗ 正控：干净夹具被判红：%s" % probs)
                    ok = False
                else:
                    print("✓ 正控：干净夹具判绿")
            else:
                if expect_red and not red:
                    print("✗ 负控[%s]：该红没红" % label)
                    ok = False
                elif expect_red and red:
                    print("✓ 负控[%s]：现红（%s）" % (label, probs[0]))

        # 正控
        probs = _problem_list(root, reg)
        if probs:
            print("✗ 正控：干净夹具被判红：%s" % probs)
            ok = False
        else:
            print("✓ 正控：干净夹具判绿（条目 %d）" % reg["count"])

        # 负控①：盘上新增件、表里没有 ⇒ 红
        _write(root, "scripts/d.py", "#!/usr/bin/env python3\nprint('d')\n")
        p = _problem_list(root, reg)
        if not any("没点名" in x for x in p):
            print("✗ 负控①（盘上有表里无）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控①（盘上有件表里无）：现红")
        os.remove(os.path.join(root, "scripts/d.py"))

        # 负控②：表里点名、盘上无件 ⇒ 红
        ghost = json.loads(json.dumps(reg))
        ghost["entries"].append({"id": "ghost", "name": "ghost", "path": "scripts/ghost.py",
                                 "sha256": "0" * 64})
        p = _problem_list(root, ghost)
        if not any("悬空登记" in x for x in p):
            print("✗ 负控②（表里点名盘上无）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控②（表里点名盘上无）：现红")

        # 负控③：件被改、缓存键过期 ⇒ 红
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a-changed')\n")
        p = _problem_list(root, reg)
        if not any("缓存键过期" in x for x in p):
            print("✗ 负控③（缓存键过期）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控③（缓存键过期）：现红")
        _write(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")

        # 负控④：同名多件 ⇒ 红
        _write(root, "scripts/sub2/a.py", "#!/usr/bin/env python3\nprint('a2')\n")
        p = _problem_list(root, reg)
        if not any("同名多件" in x for x in p):
            print("✗ 负控④（同名多件）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控④（同名多件）：现红")

        # 负控⑤：路径越出前缀 ⇒ 红
        bad = json.loads(json.dumps(reg))
        for e in bad["entries"]:
            if e["name"] == "a":
                e["path"] = "core/a.py"
        p = _problem_list(root, bad)
        if not any("前缀" in x or "悬空登记" in x for x in p):
            print("✗ 负控⑤（越出前缀）：该红没红 ⇒ %s" % p)
            ok = False
        else:
            print("✓ 负控⑤（越出前缀）：现红")

    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="判据注册表真源生成 + 双向差集自检（M1-04/M1-05）")
    ap.add_argument("--root", default=REPO, help="仓根（默认 = 本件上溯三层）")
    g = ap.add_mutually_exclusive_group()
    g.add_argument("--emit", action="store_true", help="现读生成注册表")
    g.add_argument("--check", action="store_true", help="双向差集对拍")
    g.add_argument("--self-test", dest="self_test", action="store_true", help="成对负控自检")
    ap.add_argument("--out", default=REGISTRY_DEFAULT, help="--emit 的写出件（默认 scripts/gates/judge-registry.json）")
    ap.add_argument("--registry", default=REGISTRY_DEFAULT, help="--check 读的注册表件")
    args = ap.parse_args(argv)

    root = os.path.abspath(args.root)
    if args.self_test:
        return cmd_self_test()
    if args.emit:
        out = args.out if args.out == "-" else os.path.join(root, args.out) if not os.path.isabs(args.out) else args.out
        return cmd_emit(root, out)
    if args.check:
        reg = args.registry if os.path.isabs(args.registry) else os.path.join(root, args.registry)
        return cmd_check(root, reg)
    sys.stderr.write("用法错：`--emit` / `--check` / `--self-test` 三选一必给一个（都不给 ⇒ rc=2）\n")
    return 2


if __name__ == "__main__":
    sys.exit(main())

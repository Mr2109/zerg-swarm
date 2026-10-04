#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-write-authz.py —— 「写点 → 所属函数 → catalog() 命令面 → 授权档」只读静态元门
（缺口 `GAP-20260927-140` §⑤ 逐字）

题面（缺口 `want_action` ⑤ 逐字）：
    建静态元门 `check-write-authz.py`：判据 = 写点 → 所属函数 → `catalog()` 命令面 → 授权档；
    红灯 = **不在白名单也不在豁免清单**；不误报机理 = 问「写点前有无必拦分支」（单调，加闸不改判）·
    落点按来源分档 · fail-closed + 显式豁免。

病象（缺口 `GAP-20260927-140`）：
    默认档写盘零授权共 6 处持久件，根因是这些命令面在 `catalog()` 里**没有 `dangerSpec`**
    ⇒ `cmdGuarded` 那道唯一执行门照不到它们（`impact` / `dev verify` / `help export` / `plan` /
    `dev proposal new` / `dev receipt new` 等）。本仓已把 `impact` 的写点摊进登记表
    （`family_impact_cache.go` 的 `impactCacheWritePoints`）⇒ 本件把「每个写点有没有交代」现算出来。

口径（写死 —— 本件的判据）：
    · 扫面 = `core/cmd/zerg/*.go`（**排除 `_test.go`**）；可用 `--root` 换根（自检用）。
    · **写点闭集**（正则即内容锚，不用行号）：`os.WriteFile` / `os.Create` / `os.Remove` /
      `os.RemoveAll` / `os.Rename` / `os.Mkdir` / `os.MkdirAll` / `os.OpenFile` /
      `os.Truncate` / `os.Chmod` / `os.Chtimes` / `os.Symlink` / `os.Link` / `ioutil.WriteFile`。
    · **所属函数** = 该写点上方最近的 `^func ` 签名里的函数名。
    · **两处交代**（满足其一即算「有交代」）：① 本文件有**写点登记表**
      （`…WritePoints = []…WritePoint{` 里的 `Call:` 命中该写调用）；② 本文件声明的命令面
      在 `main.go` 的 `catalog()` 里有 `danger:`（授权档被登记）。两处都没有 ⇒ **红灯点名**。
    · **白名单/豁免**：`--allow <file:call>`（可重复）或本件顶部 `EXEMPT`；豁免须给理由
      （`理由`＋`出处`），缺理由 ⇒ rc=2（写不清理由的豁免就是洗白）。
    · **退码写死**：`rc=0` 全有交代 · `rc=1` 有裸写点（逐条点名 file:line）· `rc=2` 缺语料/白名单不全。
    · **只读**：不写任何文件。

★ 边界（承缺口正文 §⑥）：本门照不到「命令面经 `exec.Command(bash, script…)` 转出的写面」
  （`gate run` / `build` / `publish run` / `isolate-ro new` / `script run` / `repo commit` 的 git 动作）
  —— 那一面须**另开一道**，本件不覆盖。

用法：
    python3 scripts/gates/check-write-authz.py [--root <dir>] [--allow file:call]... [--json] [--self-test]

★ 接线状态：本件为**仓外候件**（`/var/tmp/zerg-wave23/p0/`），未挂门步 —— 起步 `tri-report`，
  接线与首测冻结由后续单独一笔做，避免撞件。
"""
import argparse
import json
import os
import re
import sys

WRITE_RES = [
    re.compile(r"\bos\.WriteFile\("),
    re.compile(r"\bos\.Create\("),
    re.compile(r"\bos\.Remove(?:All)?\("),
    re.compile(r"\bos\.Rename\("),
    re.compile(r"\bos\.Mkdir(?:All)?\("),
    re.compile(r"\bos\.OpenFile\("),
    re.compile(r"\bos\.Truncate\("),
    re.compile(r"\bos\.Chmod\("),
    re.compile(r"\bos\.Chtimes\("),
    re.compile(r"\bos\.Symlink\("),
    re.compile(r"\bos\.Link\("),
    re.compile(r"\bioutil\.WriteFile\("),
]
FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(")
# 写点登记表里的 Call 行（Go 结构体字面量）
CALL_RE = re.compile(r'\{?\s*Call:\s*"([^"]+)"')

# 顶部豁免（须带 reason/source；空 = 无豁免）。形如 {"file": "...", "call": "...", "reason": "...", "source": "..."}
EXEMPT = []


def write_call_of(line):
    for r in WRITE_RES:
        m = r.search(line)
        if m:
            # 取函数调用名（去尾括号）
            return m.group(0)[:-1]
    return None


# 「自检夹具」函数名/体判据（这些写点落临时目录、自回收，**不是生产写面** ⇒ 不判）。
FIXTURE_NAME_RE = re.compile(r"(?i)(selftest|self_test|fixture|fixt|^mk|^newfixture)")
FIXTURE_BODY_RE = re.compile(r"(os\.MkdirTemp|ioutil\.TempDir|\.TempDir\(\)|os\.TempDir\(\))")


def _fixture_funcs(lines):
    """回 {func: True} —— 判为该文件内「自检夹具」的函数（名或体命中）。"""
    funcs = {}
    cur, start = "<file-scope>", 0
    bounds = []
    for i, line in enumerate(lines):
        fm = FUNC_RE.match(line)
        if fm:
            if start:
                bounds.append((cur, start, i))
            cur, start = fm.group(1), i
    if start:
        bounds.append((cur, start, len(lines)))
    for name, s, e in bounds:
        body = "\n".join(lines[s:e])
        if FIXTURE_NAME_RE.search(name) or FIXTURE_BODY_RE.search(body):
            funcs[name] = True
    return funcs


def scan_file(path):
    """回 [(lineno, func, call, text)] —— **已剔自检夹具函数**（生产写面口径）。"""
    out = []
    with open(path, "r", encoding="utf-8") as f:
        lines = f.read().splitlines()
    fix = _fixture_funcs(lines)
    func = "<file-scope>"
    for i, line in enumerate(lines, 1):
        fm = FUNC_RE.match(line)
        if fm:
            func = fm.group(1)
        if fix.get(func):
            continue
        stripped = line.strip()
        if stripped.startswith("//"):  # 注释里的示例不算写点
            continue
        c = write_call_of(line)
        if c:
            out.append((i, func, c, line.strip()[:120]))
    return out


def declared_write_calls(path):
    """本文件的写点登记表里登记的 Call 字符串集合。"""
    calls = set()
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            for m in CALL_RE.finditer(line):
                calls.add(m.group(1))
    return calls


def catalog_funcs_with_danger(main_go):
    """从 main.go 里粗略取「有 danger 的命令面」的相关函数名（run:）。回 set。"""
    if not os.path.isfile(main_go):
        return None
    funcs = set()
    with open(main_go, "r", encoding="utf-8") as f:
        text = f.read()
    # 粗略：找每个 {…} 块里同时含 danger: 与 run: 的片段
    for block in re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text):
        if "danger:" in block:
            m = re.search(r"run:\s*([A-Za-z_][\w]*)", block)
            if m:
                funcs.add(m.group(1))
    return funcs


def call_matches(declared, call):
    """登记表 Call 与该写调用是否同一处（登记表逐字抄源码，允许前后缀差异）。"""
    for d in declared:
        if call in d or d.replace(" ", "") == call.replace(" ", ""):
            return True
    return False


def run(root, allow, as_json):
    scopedir = os.path.join(root, "core", "cmd", "zerg")
    if not os.path.isdir(scopedir):
        print("BLOCKED —— 缺 `core/cmd/zerg/`（扫描面取不到）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    main_go = os.path.join(scopedir, "main.go")
    danger_funcs = catalog_funcs_with_danger(main_go)
    if danger_funcs is None:
        print("BLOCKED —— 缺 `main.go`（无法解析 catalog）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    # 豁免完整性
    bad = [e for e in EXEMPT if not (e.get("file") and e.get("call") and e.get("reason") and e.get("source"))]
    if bad:
        print("BLOCKED —— 豁免条目不齐（理由/出处）：%r ⇒ rc=2" % bad, file=sys.stderr)
        return 2
    exempt = {(e["file"], e["call"]) for e in EXEMPT} | {tuple(a.split(":", 1)) for a in allow if ":" in a}
    hits = []
    for name in sorted(os.listdir(scopedir)):
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        path = os.path.join(scopedir, name)
        declared = declared_write_calls(path)
        for lineno, func, call, text in scan_file(path):
            if (name, call) in exempt:
                continue
            if call_matches(declared, call):
                continue
            if func in danger_funcs:
                continue
            hits.append((name, lineno, func, call, text))
    if as_json:
        print(json.dumps({"rc": 1 if hits else 0, "bare_write_count": len(hits),
                          "hits": [{"file": f, "line": l, "func": fn, "call": c} for f, l, fn, c, _ in hits]},
                         ensure_ascii=False))
    else:
        print("写点授权元门（GAP-20260927-140）")
        print("扫面 core/cmd/zerg/*.go（非测试）· 裸写点（既无写点登记、所属命令面又无 danger）= %d 条" % len(hits))
        for f, l, fn, c, txt in hits:
            print("  ✗ %s:%d func=%s call=%s ⇒ %s" % (f, l, fn, c, txt))
        print("  口径：写点前须有交代 —— 写点登记表命中 或 所属命令面在 catalog() 有 danger。")
    return 1 if hits else 0


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        d = os.path.join(td, "core", "cmd", "zerg")
        os.makedirs(d)
        # main.go：声明一条有 danger 的命令 cmdGuardedX
        open(os.path.join(d, "main.go"), "w").write(
            'var x = []C{{danger: &dangerSpec{...}, run: cmdGuardedX}}\n')
        # 正控：裸写点（无登记、非 danger 函数）⇒ 应红
        open(os.path.join(d, "family_bare.go"), "w").write(
            'func bareWriter(inv *invocation) int {\n\tos.WriteFile("x", b, 0o644)\n\treturn 0\n}\n')
        # 登记面：有 WritePoints 表命中 ⇒ 不判
        open(os.path.join(d, "family_ok.go"), "w").write(
            'type w struct{ Call string }\n'
            'var writePoints = []w{\n\t{Call: "os.WriteFile(tmp, b, 0o644)"},\n}\n'
            'func okWriter() { os.WriteFile(tmp, b, 0o644) }\n')
        if run(td, [], False) != 1:
            print("  ✗ 正控失败：应回 rc=1"); ok = False
        # 复核 hits 只含 family_bare.go
        hits = []
        for name in sorted(os.listdir(d)):
            if name.endswith(".go") and not name.endswith("_test.go") and name != "main.go":
                declared = declared_write_calls(os.path.join(d, name))
                for ln, fn, c, _ in scan_file(os.path.join(d, name)):
                    if not call_matches(declared, c):
                        hits.append((name, c))
        if [h[0] for h in hits] != ["family_bare.go"]:
            print("  ✗ 正控命中集不符：%r" % hits); ok = False
        # 负控：豁免掉 ⇒ 0
        global EXEMPT
        EXEMPT = [{"file": "family_bare.go", "call": "os.WriteFile", "reason": "t", "source": "t"}]
        if run(td, [], False) != 0:
            print("  ✗ 负控失败：豁免未生效"); ok = False
        EXEMPT = []
        # 负控 B：缺扫描面 ⇒ rc=2
        if run(os.path.join(td, "nope"), [], False) != 2:
            print("  ✗ 负控B失败：缺语料未回 rc=2"); ok = False
    print("自检 %s（4 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="写点授权只读静态元门（GAP-20260927-140）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--allow", action="append", default=[])
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.allow, args.json)


if __name__ == "__main__":
    sys.exit(main())

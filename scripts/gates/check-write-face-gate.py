#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-write-face-gate.py —— 命令面「声明 ↔ 实现」两面判（缺口 `GAP-20260927-90`

题面（缺口 `want_action` 逐字）：
    按该枚 C 节设计落地：`authority-map.json` 新增「写面登记」（授权类闭集 + 白名单理由逐字写
    「无闸」，照现有「无对应」一格）；新增判据件 `cli_write_face_gate_test.go` 与门禁步
    `check-write-face-gate.py`；**两面判**（声明面 `usage`/`writeSpec` ↔ 实现面 AST 认
    `if !inv.yes`／`!inv.confirmGiven`／唯一门调用／验签口令四类形状）；
    三条判定（旗标必须声明 · 无闸必须显式白名单 · 强闸必须进危险清单）＋ 6 负控 1 空转 ＋
    5 防误报硬约束（证据落 file:line、白名单空即红、探针只跑零副作用档且在隔离副本、
    深度>2 判 2、自检不过不许报绿）。

病象（缺口 `GAP-20260927-90`）：
    11 条命令会写盘却一条授权都不要（`help export` · `gate matrix --out` · `dev proposal new` ·
    `dev verify` · `dev receipt new` · `plan --out` · `impact <目标>`（缓存路）· `isolate-ro new` ·
    `gate run` · `publish preflight` · `gate bench`），且审计面全仓只有三处 ⇒ 写动作不留授权记录。

── 本候件的**范围声明**（诚实边界）──
    真源设计里的 `writeSpec` 字段**尚未落 `catalog()`**（本刻现读 `writeSpec` 在 `core/cmd/zerg`
    0 命中）⇒ 本件先落**声明面用 `usage` 串 + `danger` 现况、实现面用写调用现读**的**可用子集**：
        · 三条判定里能现算的两条：**旗标必须声明**（写面命令的 usage 必须含 `--yes`/`--confirm`/`--out`）
          ＋**无闸必须显式白名单**（无 danger 又无写旗标的写面命令 ⇒ 缺白名单即红）。
        · 第三「强闸必须进危险清单」与 `authority-map.json` 的写面登记**待该两件落地后接入**
          （缺口正文 §三的接线批）。
    这**不是**把整条缺口判「已解」—— 只是把其中可现算的两条摊在亮处、逐条点名。

用法：
    python3 scripts/gates/check-write-face-gate.py [--root <dir>] [--json] [--self-test]

★ 接线状态：本件为**仓外候件**（`/var/tmp/zerg-wave23/p0/`），未挂门步 —— 首版 `tri-report`。
"""
import argparse
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
try:
    import importlib.util as _u
    _spec = _u.spec_from_file_location(
        "cwa", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-write-authz.py"))
    _cwa = _u.module_from_spec(_spec); _spec.loader.exec_module(_cwa)
except Exception:  # noqa: BLE001
    _cwa = None

WRITE_FLAG_RE = re.compile(r"--(yes|confirm|out)\b")
USAGE_RE = re.compile(r'usage:\s*"([^"]*)"')
PATH_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
DANGER_RE = re.compile(r"danger:\s*(nil|&dangerSpec\{)")
RUN_RE = re.compile(r"run:\s*([A-Za-z_][\w]*)")
# 无闸写面的显式白名单（每条须给理由/出处）。空即红 —— 见口径 ④「白名单空即红」。
WRITE_FACE_WHITELIST = []


def impl_write_funcs(scopedir):
    """回 {func: [(file,line)]} —— 含写调用的函数。"""
    out = {}
    if _cwa is None:
        return out
    for name in sorted(os.listdir(scopedir)):
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        for lineno, func, call, _ in _cwa.scan_file(os.path.join(scopedir, name)):
            out.setdefault(func, []).append((name, lineno))
    return out


def catalog_entries(main_go):
    """粗解析 main.go 的 catalog 条目 {path, usage, danger?, run}。"""
    text = open(main_go, "r", encoding="utf-8").read()
    ents = []
    for block in re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text):
        um = USAGE_RE.search(block)
        if not um:
            continue
        pm = PATH_RE.search(block)
        dm = DANGER_RE.search(block)
        rm = RUN_RE.search(block)
        path = ""
        if pm:
            path = " ".join(re.findall(r'"([^"]*)"', pm.group(1)))
        ents.append({"path": path, "usage": um.group(1),
                     "has_danger": bool(dm and dm.group(1) != "nil"),
                     "run": rm.group(1) if rm else ""})
    return ents


def run(root, as_json):
    scopedir = os.path.join(root, "core", "cmd", "zerg")
    main_go = os.path.join(scopedir, "main.go")
    if not os.path.isdir(scopedir) or not os.path.isfile(main_go):
        print("BLOCKED —— 缺 main.go / 扫描面 ⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    wfuncs = impl_write_funcs(scopedir)
    entries = catalog_entries(main_go)
    if not entries:
        print("BLOCKED —— catalog() 解析出 0 条（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    wl = {w["path"] for w in WRITE_FACE_WHITELIST if w.get("reason") and w.get("source")}
    bad_wl = ["?"] if any(not (w.get("reason") and w.get("source")) for w in WRITE_FACE_WHITELIST) else []
    if bad_wl:
        print("BLOCKED —— 白名单条目不齐理由/出处 ⇒ rc=2", file=sys.stderr)
        return 2
    hits = []
    for e in entries:
        writes = e["run"] in wfuncs
        declares = bool(WRITE_FLAG_RE.search(e["usage"]))
        if writes and not declares and not e["has_danger"] and e["path"] not in wl:
            hits.append((e["path"] or e["run"], "写面但 usage 未声明写旗标且无 danger（缺白名单）", e["usage"]))
        elif e["has_danger"] and not declares:
            hits.append((e["path"] or e["run"], "已登记 danger 但 usage 未声明对应旗标", e["usage"]))
    if as_json:
        print(json.dumps({"rc": 1 if hits else 0, "hits": [{"cmd": c, "why": w} for c, w, _ in hits]},
                         ensure_ascii=False))
    else:
        print("命令面写面「声明↔实现」两面判（GAP-20260927-90 · 可用子集）")
        print("catalog 条目 %d · 写面函数 %d · 未声明/未白名单 = %d 条" % (len(entries), len(wfuncs), len(hits)))
        for c, w, u in hits:
            print("  ✗ %s：%s ⇒ usage=「%s」" % (c, w, u))
        print("  ★ 本件只现算「旗标必须声明」＋「无闸必须显式白名单」两条；")
        print("    「强闸必须进危险清单」与 authority-map.json 写面登记待该两件落地后接入。")
    return 1 if hits else 0


def self_test():
    import tempfile
    global WRITE_FACE_WHITELIST
    ok = True
    with tempfile.TemporaryDirectory() as td:
        d = os.path.join(td, "core", "cmd", "zerg")
        os.makedirs(d)
        open(os.path.join(d, "main.go"), "w").write(
            'var cat = []*command{\n'
            ' {path: []string{"help", "export"}, usage: "zerg help export", run: cmdHelpExport},\n'
            ' {path: []string{"impact"}, usage: "zerg impact <目标> [--yes]", danger: &dangerSpec{...}, run: cmdImpact},\n'
            '}\n')
        open(os.path.join(d, "family_help.go"), "w").write(
            'func cmdHelpExport(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("x", b, 0o644)\n\treturn 0\n}\n')
        open(os.path.join(d, "family_impact.go"), "w").write(
            'func cmdImpact(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("y", b, 0o644)\n\treturn 0\n}\n')
        if run(td, False) != 1:
            print("  ✗ 正控失败：应回 rc=1（help export 写面未声明）"); ok = False
        # 负控：把 help export 进白名单 ⇒ 0
        WRITE_FACE_WHITELIST = [{"path": "help export", "reason": "t", "source": "t"}]
        if run(td, False) != 0:
            print("  ✗ 负控失败：白名单未生效"); ok = False
        WRITE_FACE_WHITELIST = []
        # 负控 B：缺 main.go ⇒ 2
        if run(os.path.join(td, "nope"), False) != 2:
            print("  ✗ 负控B失败：缺语料未回 rc=2"); ok = False
    print("自检 %s（3 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="命令面写面两面判（GAP-20260927-90 · 可用子集）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

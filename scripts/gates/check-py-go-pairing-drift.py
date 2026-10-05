#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-py-go-pairing-drift.py —— py⟷Go 双实现「两态对拍」防波门（**只读** · 棘轮只许降）

病（承 `zerg-gate-cli-pitfalls` 的「py↔Go 双实现面：只改半件 ＝ 假绿」）：
  `core/cmd/zerg/gate_inproc_table.go` 把一批判据登记成**已真搬 Go · 可在进程内跑**；
  每枚键在 `scripts/gates/<键>.py` 还留着一份 py 真身。`zerg gate check <键>` 走 **Go inproc**，
  而步骤表那一步走 **py** —— 两个入口同源吗？**没有谁保证**。枚举三口径（py 件 / Go 件 /
  inproc 键）可现读；「两半都在 ＝ 风险面」在百量级 ⇒ 分叉是架构面，不是偶发。

治法（本门 · **对拍而不是读源码猜**）：
  对 `gate_inproc_table.go` 的**每条 inproc 键**，拿**同一 argv**跑两态 ——
    态① `bin/zerg gate check <键> <argv…>`（Go 进程内 · 壳 cwd=仓根 · 壳设 ZERG_REPO*）
    态② `python3 scripts/gates/<键>.py <argv…>`（py 真身 · cwd=仓根）
  ⇒ 断言 **stdout / stderr / 退码**三面逐字同（已声明非确定面**掩码**：时间戳 · 耗时 · 临时目录 · pid）。
  ★ 差集进 **只许下降的棘轮**（`py-go-pairing-drift.ratchet.json` 的 `known` 集）：
     现读差集 ⊆ 棘轮 ⇒ 不涨；出现**棘轮里没有**的键 ⇒ 差集涨了 ⇒ `--strict` 判红。
  另出两张点名表（**键 ⇕ 件** 的双向差集）：
    表A「A 有键 · B 无件」= inproc 键在册、而 `scripts/gates/<键>.py` 不在盘 ⇒ 真分叉（`--strict` 判红）
    表B「B 有件 · A 无键」= py 真身在盘、而未进 inproc 表 ⇒ **半接线/未搬**（常态 · 只报告）

★ 为什么是**两态对拍**而不是读源码：分叉可以「今天两边同值」时静默存在（版本真源恰等于被钉死的值）；
  判「有没有分叉」**必须造扰动**（换 argv / 换根），✗ 不能拿今日平值当「没分叉」。

★ **只读门**（✗ 不写仓内任何物）：本件只读表 / 只跑子进程 / 只读棘轮；**棘轮由人维护**，
  本件给 `--dump-ratchet`（现读差集打到 stdout · **不写盘**）供人照抄 —— 与 `--emit-baseline` 同族口径。

退码（三档 · fail-closed）
  0 = 判到了（**报告档** · 差集与两张点名表照出 · 棘轮只许降但报告档不阻断）
  1 = 只有 `--strict` 档才可能：① 差集出现棘轮里没有的键（棘轮涨了）② 表A 非空（键无件）
  2 = **不给结论**：inproc 表读不到 / 解析 0 键 / `scripts/gates` 目录读不到 / 棘轮不在盘 /
      棘轮 `probe_argv` 与本次 argv 不符 / `bin/zerg` 不在盘 / 家目录?无 —— 任一取不到输入即 rc=2
      （空域 ≠ 全绿 · 零执行探针一律不给结论）· `--self-test` 用例数对不上也退 2

掩码（已声明的**非确定面** · 逐字口径见 `MASK_RULES`）
  时间戳 `YYYY-MM-DD[ T]HH:MM:SS` · 耗时 `<数>s` / `<数> 秒` · 临时目录 `/var/folders/…` `/tmp/…`
  `…/T/…` · `pid=<数>` · `tempdir=<串>`。掩码**只抹这些面**；其余逐字比。

用法
    python3 scripts/gates/check-py-go-pairing-drift.py                    # 报告档 · 真目标
    python3 scripts/gates/check-py-go-pairing-drift.py --strict           # 判红档（棘轮只许降）
    python3 scripts/gates/check-py-go-pairing-drift.py --argv=--self-test # 指定两态 argv（缺省 --self-test）
    python3 scripts/gates/check-py-go-pairing-drift.py --dump-ratchet     # 现读差集打到 stdout（不写盘）
    python3 scripts/gates/check-py-go-pairing-drift.py --json             # 机器面
    python3 scripts/gates/check-py-go-pairing-drift.py --self-test        # 成对正负控 · 仓外夹具 · 不碰真目标
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

VERSION = "0.1.0（防波门 · 缺口承 zerg-gate-cli-pitfalls「双实现面只改半件＝假绿」）"
INPROC_REL = os.path.join("core", "cmd", "zerg", "gate_inproc_table.go")
GATES_REL = os.path.join("scripts", "gates")
RATCHET_REL = os.path.join("scripts", "gates", "py-go-pairing-drift.ratchet.json")
SCHEMA = "zerg/py-go-pairing-drift/v1"
DEFAULT_ARGV = ["--self-test"]
PROBE_TIMEOUT = 120          # 单侧单键秒级上限（超时 ⇒ 该键「不判」· 不进棘轮）

# inproc 表里的条目形态：`\t"<键>":   <Go 函数名>,`（★ 只认这一形态 · 表件里其余行不吃）
KEY_RE = re.compile(r'^\t"([^"]+)"\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*$')

# ★ 已声明的**非确定面**（掩码口径 · 只抹这些 · 其余逐字比）
MASK_RULES = [
    (re.compile(r"\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?"), "<TS>"),
    (re.compile(r"\d+(?:\.\d+)?\s*s\b"), "<DUR>"),
    (re.compile(r"\d+(?:\.\d+)?\s*秒"), "<DUR>"),
    (re.compile(r"/var/folders/[^\s\"']+"), "<TMP>"),
    (re.compile(r"/tmp/[^\s\"']+"), "<TMP>"),
    (re.compile(r"/private/[^\s\"']+"), "<TMP>"),
    (re.compile(r"pid[=:]\s*\d+"), "pid=<PID>"),
    (re.compile(r"tempdir[=:]\s*\S+"), "tempdir=<TMP>"),
]


def mask(text: str) -> str:
    for rx, rep in MASK_RULES:
        text = rx.sub(rep, text)
    return text


def read_inproc_keys(root: str):
    """⇒ ([(键, Go 函数名)], "")；取不到 ⇒ (None, 说明)。"""
    p = os.path.join(root, INPROC_REL)
    if not os.path.isfile(p):
        return None, "inproc 表读不到：%s" % INPROC_REL
    try:
        lines = open(p, encoding="utf-8").read().splitlines()
    except OSError as e:
        return None, "inproc 表读不出：%s" % e
    out = []
    for ln in lines:
        m = KEY_RE.match(ln)
        if m:
            out.append((m.group(1), m.group(2)))
    if not out:
        return None, "inproc 表解析出 0 键（取不到输入 ⇒ 空域 ≠ 全绿）"
    return out, ""


def scan_py_stems(root: str):
    d = os.path.join(root, GATES_REL)
    if not os.path.isdir(d):
        return None, "门件目录读不到：%s" % GATES_REL
    stems = set()
    for name in os.listdir(d):
        if name.endswith(".py"):
            stems.add(name[:-3])
    return stems, ""


def load_ratchet(root: str, argv_list):
    p = os.path.join(root, RATCHET_REL)
    if not os.path.isfile(p):
        return None, "棘轮不在盘：%s（先据 --dump-ratchet 落它）" % RATCHET_REL
    try:
        d = json.loads(open(p, encoding="utf-8").read())
    except Exception as e:  # noqa: BLE001
        return None, "棘轮解析不上：%s" % e
    ra = d.get("probe_argv")
    if ra != list(argv_list):
        return None, "棘轮 probe_argv=%r ≠ 本次 argv=%r ⇒ 不可比（不给结论）" % (ra, list(argv_list))
    known = d.get("known")
    if not isinstance(known, list):
        return None, "棘轮缺 known 列表"
    return set(known), ""


def run_side(argv, cwd, timeout=PROBE_TIMEOUT):
    """⇒ (rc|None, out, err, 超时/起不来 标记)。rc=None ⇒ 没进程退码可言。"""
    try:
        p = subprocess.run(argv, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           timeout=timeout)
        return p.returncode, p.stdout.decode("utf-8", "replace"), p.stderr.decode("utf-8", "replace"), False
    except subprocess.TimeoutExpired as e:
        return None, (e.stdout or b"").decode("utf-8", "replace"), (e.stderr or b"").decode("utf-8", "replace"), True
    except OSError as e:
        return None, "", "%s" % e, True


def probe(root, key, argv_list, zerg_bin, py_bin):
    """两态对拍一条键。⇒ dict(kind, ...)；kind ∈ same/diff/unresolved。"""
    side_a = [zerg_bin, "gate", "check", key] + list(argv_list)
    side_b = [py_bin, os.path.join(root, GATES_REL, key + ".py")] + list(argv_list)
    rca, outa, erra, toa = run_side(side_a, root)
    rcb, outb, errb, tob = run_side(side_b, root)
    if toa or tob or rca is None or rcb is None:
        return {"kind": "unresolved", "a_rc": rca, "b_rc": rcb,
                "a_to": toa, "b_to": tob, "detail": "两态之一超时/起不来 ⇒ 不判"}
    ma, mb = mask(outa), mask(outb)
    ea, eb = mask(erra), mask(errb)
    if rca == rcb and ma == mb and ea == eb:
        return {"kind": "same", "a_rc": rca, "b_rc": rcb}
    return {"kind": "diff", "a_rc": rca, "b_rc": rcb,
            "out_diff": ma != mb, "err_diff": ea != eb, "rc_diff": rca != rcb,
            "a_out": ma, "b_out": mb, "a_err": ea, "b_err": eb}


def evaluate(root, argv_list, zerg_bin=None, py_bin=None, timeout=PROBE_TIMEOUT,
             require_ratchet=True, only=None):
    """⇒ (rc_data, err)。rc_data = 全量读数（不含退码判定）。"""
    keys, err = read_inproc_keys(root)
    if keys is None:
        return None, err
    py_stems, err = scan_py_stems(root)
    if py_stems is None:
        return None, err
    if require_ratchet:
        known, err = load_ratchet(root, argv_list)
        if known is None:
            return None, err
    else:
        known = set()
    zerg_bin = zerg_bin or os.path.join(root, "bin", "zerg")
    if not os.path.isfile(zerg_bin):
        return None, "bin/zerg 不在盘：%s（态① 跑不了）" % zerg_bin
    py_bin = py_bin or sys.executable
    key_names = [k for k, _ in keys]
    if only:
        key_names = [k for k in key_names if k in set(only)]
    key_set = set(k for k, _ in keys)
    # 表A：A 有键 · B 无件
    table_a = [k for k in key_names if k not in py_stems]
    # 表B：B 有件 · A 无键
    table_b = sorted(s for s in py_stems if s not in key_set)
    results = {}
    same, diff, unresolved = [], [], []
    for k in key_names:
        if k not in py_stems:
            continue  # 表A 已点名 · 无件可对拍
        r = probe(root, k, argv_list, zerg_bin, py_bin)
        results[k] = r
        if r["kind"] == "same":
            same.append(k)
        elif r["kind"] == "diff":
            diff.append(k)
        else:
            unresolved.append(k)
    diff_set = set(diff)
    ratchet_grew = sorted(diff_set - known)
    ratchet_shrank = sorted(known - diff_set)
    return {
        "schema": SCHEMA, "root": root, "probe_argv": list(argv_list),
        "keys_total": len(key_names), "py_stems_total": len(py_stems),
        "probed": len(same) + len(diff) + len(unresolved),
        "same": sorted(same), "diff": sorted(diff), "unresolved": sorted(unresolved),
        "table_a_key_no_file": sorted(table_a),
        "table_b_file_no_key": table_b,
        "ratchet_known": sorted(known),
        "ratchet_grew": ratchet_grew,
        "ratchet_shrank": ratchet_shrank,
        "results": results,
    }, ""


def render(rep, strict):
    out = []
    out.append("── py⟷Go 双实现两态对拍（inproc 键 ⇕ scripts/gates/<键>.py）──")
    out.append("  两态 argv：%s" % (" ".join(rep["probe_argv"]) or "（空）"))
    out.append("  inproc 键 %d 条 · py 真身 %d 条 · 已对拍 %d 条（同 %d · 差 %d · 不判 %d）"
               % (rep["keys_total"], rep["py_stems_total"], rep["probed"],
                  len(rep["same"]), len(rep["diff"]), len(rep["unresolved"])))
    out.append("  棘轮：已知差 %d · 本次涨 %d · 本次降 %d"
               % (len(rep["ratchet_known"]), len(rep["ratchet_grew"]), len(rep["ratchet_shrank"])))
    if rep["diff"]:
        out.append("  ✗ 差集（两态 stdout/stderr/退码 未逐字同）：")
        for k in rep["diff"]:
            r = rep["results"].get(k, {})
            tags = []
            if r.get("rc_diff"):
                tags.append("rc %s≠%s" % (r.get("a_rc"), r.get("b_rc")))
            if r.get("out_diff"):
                tags.append("stdout 异")
            if r.get("err_diff"):
                tags.append("stderr 异")
            out.append("    · %s（%s）" % (k, " · ".join(tags)))
    if rep["unresolved"]:
        out.append("  ⚠ 不判（超时/起不来 · 不进棘轮）：%s" % " ".join(rep["unresolved"]))
    if rep["ratchet_grew"]:
        out.append("  ✗ 棘轮涨了（现读差集里棘轮没有的键）：%s" % " ".join(rep["ratchet_grew"]))
    if rep["ratchet_shrank"]:
        out.append("  ✓ 棘轮降了（棘轮里有、现读已不合）：%s" % " ".join(rep["ratchet_shrank"]))
    out.append("  表A「A 有键 · B 无件」%d 条：" % len(rep["table_a_key_no_file"]))
    for k in rep["table_a_key_no_file"]:
        out.append("    ✗ %s（inproc 在册 · scripts/gates/%s.py 不在盘）" % (k, k))
    if not rep["table_a_key_no_file"]:
        out.append("    （空 · 每枚 inproc 键都有 py 真身）")
    out.append("  表B「B 有件 · A 无键」%d 条（半接线/未搬 · 只报告）：" % len(rep["table_b_file_no_key"]))
    for s in rep["table_b_file_no_key"]:
        out.append("    · scripts/gates/%s.py" % s)
    verdict = "判红档" if strict else "报告档"
    greens = (not rep["table_a_key_no_file"]) and (not rep["ratchet_grew"])
    out.append("  ⇒ %s：%s" % (verdict, "全绿" if greens else
                              "有 %d 项需处理（表A %d · 棘轮涨 %d）"
                              % (len(rep["table_a_key_no_file"]) + len(rep["ratchet_grew"]),
                                 len(rep["table_a_key_no_file"]), len(rep["ratchet_grew"]))))
    return "\n".join(out)


def cmd_dump_ratchet(rep):
    """现读差集打到 stdout（**不写盘**）· 供人照抄进棘轮。"""
    body = {
        "schema": SCHEMA,
        "_口径": "只许下降：本集是「已声明可分叉」的键；现读差集必须 ⊆ 本集。新增键 ⇒ --strict 判红。",
        "probe_argv": rep["probe_argv"],
        "known": rep["diff"],
    }
    print(json.dumps(body, ensure_ascii=False, indent=2))


def main(argv=None):
    ap = argparse.ArgumentParser(description="py⟷Go 双实现两态对拍防波门（只读 · 棘轮只许降）")
    ap.add_argument("--root", default="")
    ap.add_argument("--argv", default="--self-test",
                    help="两态共用的 argv 串（按空格切；给空串 = 无参）")
    ap.add_argument("--zerg", default="", help="态① 启动器（缺省 <root>/bin/zerg）")
    ap.add_argument("--python", dest="pybin", default="", help="态② 解释器（缺省本进程 python）")
    ap.add_argument("--strict", action="store_true", help="判红档：棘轮涨 / 表A 非空 ⇒ rc=1")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--dump-ratchet", dest="dump_ratchet", action="store_true")
    ap.add_argument("--only", action="append", default=None, help="只对拍指定键（可重复 · 调试用）")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    root = os.path.abspath(a.root) if a.root else os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    argv_list = a.argv.split() if a.argv else []
    zerg_bin = a.zerg or os.path.join(root, "bin", "zerg")
    # --dump-ratchet 是**引导档**：棘轮还没落盘时也能现跑出差集（不要求棘轮在盘）
    rep, err = evaluate(root, argv_list, zerg_bin=zerg_bin, py_bin=(a.pybin or None),
                        require_ratchet=(not a.dump_ratchet), only=a.only)
    if rep is None:
        sys.stderr.write("%s: %s ⇒ 不给结论（退码 2）\n" % (VERSION, err))
        return 2
    if a.dump_ratchet:
        cmd_dump_ratchet(rep)
        return 0
    if a.json:
        print(json.dumps({k: v for k, v in rep.items() if k != "results"},
                         ensure_ascii=False, indent=2))
    else:
        print(render(rep, a.strict))
    if a.strict and (rep["table_a_key_no_file"] or rep["ratchet_grew"]):
        return 1
    return 0


# ══════════════════════════════════════════════════════════════════
# 自检面（--self-test）：成对正负控 · 仓外夹具 · 不碰真目标
# ══════════════════════════════════════════════════════════════════
WANT_CASES = 12
_TMP = []


def _mk_fixture(base, keys=("k-alpha", "k-beta"), py_extra=(), argv=("--self-test",),
                ratchet_known=(), py_missing=()):
    """现搭仓外最小仓树：inproc 表 ＋ bin/zerg 桩（= 态①）＋ scripts/gates/*.py（态②）＋ 棘轮。

    态① 桩 `bin/zerg`：`gate check <键>` ⇒ 跑 `go/<键>.py`（模拟 Go 进程内实现）。
    态② 真件：`scripts/gates/<键>.py`。
    两侧都吃 `argv`（缺省 --self-test）；两侧输出由各自 print 决定 ⇒ 改一字节即分叉。
    """
    fix = tempfile.mkdtemp(prefix="pgpd-", dir=base)
    _TMP.append(fix)
    os.makedirs(os.path.join(fix, "core", "cmd", "zerg"))
    os.makedirs(os.path.join(fix, "scripts", "gates"))
    os.makedirs(os.path.join(fix, "bin"))
    os.makedirs(os.path.join(fix, "go"))
    # inproc 表（逐字同真表形态：\t"键":  Fn,）
    lines = ["package main", "", "var gateCheckGoEntry = map[string]func(){",
             ]
    for k in keys:
        lines.append('\t"%s":   %sGateMain,' % (k, k.replace("-", "_")))
    lines.append("}")
    open(os.path.join(fix, INPROC_REL), "w", encoding="utf-8").write("\n".join(lines) + "\n")
    # bin/zerg 桩：gate check <键> [argv…] ⇒ go/<键>.py
    stub = ('#!/bin/sh\n'
            'here="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"\n'
            '[ "$1" = "gate" ] && shift\n'
            '[ "$1" = "check" ] && shift\n'
            'k="$1"; shift\n'
            'exec python3 "$here/../go/$k.py" "$@"\n')
    zb = os.path.join(fix, "bin", "zerg")
    open(zb, "w", encoding="utf-8").write(stub)
    os.chmod(zb, 0o755)
    # 两侧真件
    for k in keys:
        body = ('import sys\nif "--self-test" in sys.argv:\n    print("OUT-%s")\n'
                'else:\n    print("arg:%s")\n' % (k, " ".join(argv)))
        open(os.path.join(fix, "go", k + ".py"), "w", encoding="utf-8").write(body)
        if k not in py_missing:
            open(os.path.join(fix, "scripts", "gates", k + ".py"), "w", encoding="utf-8").write(body)
    for k in py_extra:
        open(os.path.join(fix, "scripts", "gates", k + ".py"), "w", encoding="utf-8").write("# orphan\n")
    # 棘轮
    rj = {"schema": SCHEMA, "probe_argv": list(argv), "known": sorted(ratchet_known)}
    os.makedirs(os.path.join(fix, os.path.dirname(RATCHET_REL)), exist_ok=True)
    open(os.path.join(fix, RATCHET_REL), "w", encoding="utf-8").write(
        json.dumps(rj, ensure_ascii=False))
    return fix


def _bump_side(fix, side, key, to):
    """把某件的一侧**改一个字节**：`OUT-k-alpha` → `OUT-k-alphb`（GO 侧 ⇒ go/<键>.py · PY 侧 ⇒ scripts/gates/<键>.py）。"""
    rel = os.path.join("go", key + ".py") if side == "go" else os.path.join(GATES_REL, key + ".py")
    p = os.path.join(fix, rel)
    s = open(p, encoding="utf-8").read()
    new, n = re.subn(r"OUT-[A-Za-z0-9-]+", "OUT-" + to, s)
    assert n == 1, "未改到（%s · 命中 %d 次）" % (rel, n)
    open(p, "w", encoding="utf-8").write(new)


def _write_side(fix, side, key, body):
    rel = os.path.join("go", key + ".py") if side == "go" else os.path.join(GATES_REL, key + ".py")
    open(os.path.join(fix, rel), "w", encoding="utf-8").write(body)


def _run_inproc(argv):
    """进程内跑 main()（真返回值）· 捕获 stdout ⇒ (rc, out)。"""
    import contextlib
    import io
    out = io.StringIO()
    saved = sys.argv
    try:
        with contextlib.redirect_stdout(out):
            rc = main(argv)
    finally:
        sys.argv = saved
    return rc, out.getvalue()


def _ev(fix, argv=("--self-test",), **kw):
    return evaluate(fix, list(argv), **kw)


def self_test():
    d = tempfile.mkdtemp(prefix="pgpd-selftest-")
    _TMP.append(d)
    bad, total = 0, 0

    def case(tag, desc, ok, detail):
        nonlocal bad, total
        total += 1
        bad += 0 if ok else 1
        print("  %s %s %s\n       %s" % ("✓" if ok else "✗", tag, desc, detail))

    print("── check-py-go-pairing-drift.py 自检（成对正负控 · 仓外夹具 · 不碰真目标）──")

    # ①正控：两态一致 ⇒ 同
    f = _mk_fixture(d)
    rep, err = _ev(f)
    case("①正控", "健康夹具（两侧同）⇒ 差集空 · 表A 空 · rc=0",
         err == "" and rep["diff"] == [] and rep["table_a_key_no_file"] == [],
         "err=%r diff=%s 表A=%s" % (err, rep["diff"] if rep else None,
                                   rep["table_a_key_no_file"] if rep else None))

    # ②负控①：改 Go 侧一字节 ⇒ 点名该键
    _bump_side(f, "go", "k-alpha", "k-alphb")
    rep, err = _ev(f)
    case("②负控", "改**态①(Go 侧)**一字节 ⇒ 差集恰 {k-alpha}（点名该键）",
         err == "" and rep["diff"] == ["k-alpha"],
         "diff=%s" % (rep["diff"] if rep else None))

    # ③正控：复原 ⇒ 回绿（成对）
    _bump_side(f, "go", "k-alpha", "k-alpha")
    rep, err = _ev(f)
    case("③正控", "与②成对：复原 Go 侧 ⇒ 差集回空",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ④负控②：改 py 侧一字节 ⇒ 点名该键
    _bump_side(f, "py", "k-beta", "k-betb")
    rep, err = _ev(f)
    case("④负控", "改**态②(py 侧)**一字节 ⇒ 差集恰 {k-beta}",
         err == "" and rep["diff"] == ["k-beta"],
         "diff=%s" % (rep["diff"] if rep else None))
    _bump_side(f, "py", "k-beta", "k-beta")

    # ⑤负控③：退码面差也判异
    _write_side(f, "py", "k-alpha",
                'import sys\nif "--self-test" in sys.argv:\n    print("OUT-k-alpha")\nsys.exit(3)\n')
    rep, err = _ev(f)
    case("⑤负控", "态② 退码改成 3（stdout 相同）⇒ 仍判异（退码面也算）",
         err == "" and rep["diff"] == ["k-alpha"],
         "diff=%s" % (rep["diff"] if rep else None))
    _write_side(f, "py", "k-alpha",
                'import sys\nif "--self-test" in sys.argv:\n    print("OUT-k-alpha")\n'
                'else:\n    print("arg:--self-test")\n')

    # ⑥负控④：棘轮涨 —— 棘轮 known 为空而现读有差 ⇒ --strict rc=1
    f2 = _mk_fixture(d, ratchet_known=())
    _bump_side(f2, "go", "k-alpha", "k-alphb")
    rc, out = _run_inproc(["--root", f2, "--strict"])
    case("⑥负控", "棘轮 known 空 + 出现新差 ⇒ --strict rc=1（棘轮只许降）",
         rc == 1 and "棘轮涨了" in out and "k-alpha" in out, "rc=%d" % rc)

    # ⑦正控：差集已在棘轮 ⇒ --strict rc=0（成对）
    f3 = _mk_fixture(d, ratchet_known=("k-alpha",))
    _bump_side(f3, "go", "k-alpha", "k-alphb")
    rc, out = _run_inproc(["--root", f3, "--strict"])
    case("⑦正控", "与⑥成对：差集已在棘轮里 ⇒ --strict rc=0",
         rc == 0, "rc=%d" % rc)

    # ⑧表A：键无件 ⇒ 点名 + strict 判红
    f4 = _mk_fixture(d, keys=("k-alpha", "k-beta", "k-gamma"), py_missing=("k-gamma",))
    rep, err = _ev(f4)
    rc, out = _run_inproc(["--root", f4, "--strict"])
    case("⑧负控", "表A：inproc 有 k-gamma 而 scripts/gates/k-gamma.py 不在盘 ⇒ 点名 · strict rc=1",
         err == "" and rep["table_a_key_no_file"] == ["k-gamma"] and rc == 1 and "k-gamma" in out,
         "表A=%s rc=%d" % (rep["table_a_key_no_file"] if rep else None, rc))

    # ⑨表B：件无键 ⇒ 只报告 · 不判红
    f5 = _mk_fixture(d, py_extra=("k-orphan",))
    rep, err = _ev(f5)
    rc, out = _run_inproc(["--root", f5, "--strict"])
    case("⑨正控", "表B：scripts/gates/k-orphan.py 在盘而 inproc 无键 ⇒ 表B 点名 · **不**判红（rc=0）",
         err == "" and rep["table_b_file_no_key"] == ["k-orphan"] and rc == 0,
         "表B=%s rc=%d" % (rep["table_b_file_no_key"] if rep else None, rc))

    # ⑩掩码：两侧只差时间戳/耗时/临时目录 ⇒ 掩码后判同
    f6 = _mk_fixture(d, keys=("k-mask",))
    open(os.path.join(f6, "go", "k-mask.py"), "w", encoding="utf-8").write(
        'print("done 2026-10-05 12:00:00 in 0.31s at /var/folders/ab/T/xyz")\n')
    open(os.path.join(f6, GATES_REL, "k-mask.py"), "w", encoding="utf-8").write(
        'print("done 2026-10-06 01:02:03 in 9.99s at /var/folders/zz/T/qwerty")\n')
    rep, err = _ev(f6)
    case("⑩掩码", "两侧只差时间戳/耗时/临时目录（已声明非确定面）⇒ 掩码后判**同**",
         err == "" and rep["diff"] == [], "diff=%s" % (rep["diff"] if rep else None))

    # ⑪空域守卫：inproc 表读不到 ⇒ rc=2
    f7 = tempfile.mkdtemp(prefix="pgpd-notable-", dir=d)
    os.makedirs(os.path.join(f7, "scripts", "gates"))
    os.makedirs(os.path.join(f7, "bin"))
    rc, out = _run_inproc(["--root", f7])
    case("⑪退码钳", "inproc 表读不到 ⇒ rc=2（不给结论 · 空域 ≠ 全绿）",
         rc == 2, "rc=%d" % rc)

    # ⑫棘轮 argv 不符 ⇒ rc=2
    f8 = _mk_fixture(d, argv=("--self-test",))
    rc, out = _run_inproc(["--root", f8, "--argv="])
    case("⑫退码钳", "棘轮 probe_argv(--self-test) ≠ 本次 argv(空) ⇒ rc=2（不可比）",
         rc == 2, "rc=%d" % rc)

    print("  自证用例：%d 条 · 断言条数写死 %d（少跑一格即失效）" % (total, WANT_CASES))
    if total != WANT_CASES:
        print("✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2" % (WANT_CASES, total),
              file=sys.stderr)
        return 2
    print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
    return 0 if bad == 0 else 1


def _cleanup():
    for x in _TMP:
        shutil.rmtree(x, ignore_errors=True)


if __name__ == "__main__":
    import atexit
    atexit.register(_cleanup)
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""gate-report.py —— 门禁面「逐条跑 + 汇总为 JSON」的机器读编排件（2026-09-25 · C-03 读面留痕）

病灶与口径（C-03）
------------------
现有门禁（`scripts/gates/check-*.py`）每门各自可跑、各自可 `--json`，但**没有一个编排件**把
「跑哪几门 → 每门退码/耗时/关键判据行/是否已登记假红」一次汇总成一份**机器可读**的账。人只能
逐门读 stdout，门多了就对不齐。本件把「跑一批门禁 + 汇总成 JSON」写成一条可跑的命令。

本件**不重算任何门禁的判据**：它只做三件事 ——
  ① 按清单逐门 `subprocess` 跑真门禁（取**真退出码**，不管道、不猜）；
  ② 计时（秒，三位小数）；
  ③ 从该门 stdout 里捞「关键判据行」—— 默认捞含「结论/命中/rc=/未登记/悬空/FAIL/红」的行，
     或 `--key-regex` 自定；`--json` 下把该门自己的 `--json` 输出（若它吐 JSON）一并原样收进 `raw`。

只读不写（铁律 ③）
------------------
本件**只跑、只读、只汇总**：绝不写工作树、不改任何门禁文件、不写任何产物（除非你显式
`--dump <path>` 把汇总 JSON 落到你指定的文件）。任何写意图 = `--dump` 以外的旗标 ⇒ rc=2 且零字节。
跑门禁本身会不会改文件由**门禁自己**决定；本件只保证自己不动手。

用法
----
    python3 scripts/gates/gate-report.py                       # 跑 gates/ 全部 *.py 门禁（*.sh 跳过）
    python3 scripts/gates/gate-report.py --gate check-foo.py --gate check-bar.py   # 点名跑
    python3 scripts/gates/gate-report.py --list                    # 列本目录认得哪些门禁
    python3 scripts/gates/gate-report.py --key-regex '结论'          # 自定「关键判据行」正则
    python3 scripts/gates/gate-report.py --json
    python3 scripts/gates/gate-report.py --dump /tmp/gates.json   # 显式落盘汇总（唯一被允许的写）
    python3 scripts/gates/gate-report.py --registry <路径>        # 给悬空引用门指定登记表
    python3 scripts/gates/gate-report.py --root <仓根>            # 换跑的门禁根（默认仓根）

退码：0 无待办（所有被跑门禁 rc=0）/ 1 有待办（任一被跑门禁 rc≠0，即「有未收敛项」）/
      2 不给结论（清单为空 · 某门禁跑不了/崩 · 用法错 · 任何非 --dump 的写意图）
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

_HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.abspath(os.path.join(_HERE, "..", ".."))

# 写旗标：除 --dump 外任何想「落地/改动」的意图都一律拒
WRITE_FLAGS = ("--dump",)
WRITE_INTENT_FLAGS = (
    "--write", "--set", "--append", "--yes", "--reason", "--force",
    "--overwrite", "--apply", "--commit", "--push",
)

# 门禁按名字认「它要不要登记表」，好判断「假红」—— 悬空引用门 + 任何 registry 门禁
REGISTRY_GATE = "check-dangling-refs"


def _is_json_line(s):
    return s.lstrip().startswith("{") and '"schema"' in s or s.lstrip().startswith("{") and ("\"rc\"" in s or "\"hits\"" in s or "\"stats\"" in s)


def _looks_like_json(s):
    s = s.strip()
    if not (s.startswith("{") and s.endswith("}")):
        return False
    try:
        json.loads(s)
        return True
    except Exception:
        return False


def list_gates(root):
    """列 root/gates 目录下所有 *.py 门禁（*.sh 跳过，编排件自身也跳过）。"""
    d = os.path.join(root, "scripts", "gates")
    out = []
    if not os.path.isdir(d):
        return out
    for fn in sorted(os.listdir(d)):
        if not fn.endswith(".py"):
            continue
        if fn in ("gate-report.py", "who-references.py"):
            continue
        out.append(fn)
    return out


def run_gate(gate_path, argv_extra, root):
    """真跑一门门禁：取真退出码 + 计时 + stdout。跑不了返回 rc=2 的失败记录。"""
    cmd = [sys.executable, gate_path] + argv_extra
    t0 = time.time()
    try:
        proc = subprocess.run(cmd, cwd=root, capture_output=True, text=True, timeout=600)
    except subprocess.TimeoutExpired:
        return {"rc": 2, "elapsed_s": round(time.time() - t0, 3),
                "stdout": "", "stderr": "⏱ 门禁超时（>600s）", "errored": True}
    except Exception as e:
        return {"rc": 2, "elapsed_s": round(time.time() - t0, 3),
                "stdout": "", "stderr": "✗ 跑不了该门禁：%s" % e, "errored": True}
    out = proc.stdout or ""
    err = proc.stderr or ""
    return {"rc": proc.returncode, "elapsed_s": round(time.time() - t0, 3),
            "stdout": out, "stderr": err, "errored": False}


def extract_key_lines(stdout, key_re):
    """从门禁 stdout 里捞关键判据行（去重、保留顺序）。"""
    pat = re.compile(key_re)
    seen = set()
    lines = []
    for ln in stdout.splitlines():
        if pat.search(ln):
            # 太长就截到 200 字符；按「行内容」去重
            cut = ln.strip()
            if len(cut) > 200:
                cut = cut[:200] + "…"
            if cut in seen:
                continue
            seen.add(cut)
            lines.append(cut)
    return lines[:20]  # 每门最多留 20 条关键行，别撑爆 JSON


_UNREADABLE_REASON = "已登记/未登记 计数读不出（人面文案变体且无机读面）⇒ 不给结论"


def _dangling_machine_face(stdout):
    """读 check-dangling-refs 的机读面（--json 一行 · schema=dangling-refs/1）。

    返回 (registered, hit) 或 None —— hit 即「未登记」数（live+ledger）。
    GAP-20261005-30 第一刀：真源优先走机读面，人面只作退化路。
    """
    for ln in stdout.splitlines():
        s = ln.strip()
        if not (s.startswith("{") and s.endswith("}")):
            continue
        try:
            o = json.loads(s)
        except Exception:
            continue
        if isinstance(o, dict) and o.get("schema") == "dangling-refs/1":
            reg, hit = o.get("registered"), o.get("hit")
            if type(reg) is int and type(hit) is int:
                return reg, hit
    return None


def extract_registered_falses(stdout, gate_name):
    """判断该门是否「已登记假红」：悬空引用门若命中几乎全在登记表里 = 假红。

    口径：门名含 check-dangling-refs，且 stdout 里出现「已登记」且「不计命中」字样，
    同时「未登记」数相对「已登记」数很小 ⇒ 标 registered_falses=true。
    """
    if REGISTRY_GATE not in gate_name:
        return False, None
    _mf = _dangling_machine_face(stdout)
    if _mf is not None:
        _reg, _hit = _mf
        return (_reg > 0 and (_hit == 0 or _reg >= 5 * _hit)), {
            "unregistered": _hit, "registered": _reg
        }
    if "已登记" not in stdout or "不计命中" not in stdout:
        return False, None
    m_un = re.search(r"有 (\d+) 处.*未登记", stdout)
    m_reg = re.search(r"已登记 (\d+) 处", stdout)
    n_un = int(m_un.group(1)) if m_un else None
    n_reg = int(m_reg.group(1)) if m_reg else None
    if n_un is None or n_reg is None:
        return False, {"unreadable": True, "reason": _UNREADABLE_REASON}
    return (n_reg > 0 and (n_un == 0 or n_reg >= 5 * n_un)), {
        "unregistered": n_un, "registered": n_reg
    }


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────
# ── `Q-176` 第六批：`--self-test` 三档（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
SELF_PATH = os.path.abspath(__file__)


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """三档：合成门禁全在临时目录（正控 0 · 负控 1 · 用法错 2）。"""
    bad = [0]
    print("── gate-report.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
    tmp = tempfile.mkdtemp(prefix="gatereport-selftest-")
    try:
        repo = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(repo, "scripts", "gates"))
        for name, code in (("ok-gate.py", 0), ("bad-gate.py", 1)):
            with open(os.path.join(repo, "scripts", "gates", name), "w", encoding="utf-8") as f:
                f.write("#!/usr/bin/env python3\nimport sys\nprint('rc=0 夹具')\nsys.exit(%d)\n" % code)

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH, "--root", repo] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：夹具门 rc=0 ⇒ 汇总 rc=0", 0, *run("--gate", "ok-gate.py"))
        _st_chk(bad, "档② 负控：夹具门 rc=1 ⇒ 汇总 rc=1 并点名", 1, *run("--gate", "bad-gate.py"))
        _st_chk(bad, "档③ 用法错：不认的旗标 ⇒ rc=2", 2, *run("--no-such-flag"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main(argv):
    if "--self-test" in argv or "--selftest" in argv:
        return self_test()
    ap = argparse.ArgumentParser(
        prog="gate-report.py",
        description="逐条跑门禁清单并汇总为 JSON（默认只读不写）",
        add_help=True,
    )
    ap.add_argument("--root", default=DEFAULT_ROOT, help="门禁跑在哪个仓根（默认本仓根）")
    ap.add_argument("--gate", action="append", default=[],
                    help="点名跑的门禁 *.py（可多次）；缺省 = 跑 gates/ 全部")
    ap.add_argument("--list", action="store_true", help="列本目录认得哪些门禁，不打跑")
    ap.add_argument("--key-regex", default="结论|命中|rc=|未登记|悬空|FAIL|红|pass|fail",
                    help="从门禁 stdout 里捞关键判据行的正则（默认命中「结论/rc=/未登记…」）")
    ap.add_argument("--json", action="store_true", help="机器读面：输出汇总 JSON")
    ap.add_argument("--dump", default="", help="把汇总 JSON 显式落盘（唯一被允许的写行为）")
    ap.add_argument("--registry", default="", help="给悬空引用门指定登记表路径")
    ap.add_argument("--json-only", action="store_true",
                    help="只 stdout 输出 JSON 本体（无任何 human 行）—— 管线用")
    args = ap.parse_args(argv)

    # 写意图：除 --dump 外的任何写旗标 = 拒
    for f in WRITE_INTENT_FLAGS:
        if f in argv:
            if not args.json_only:
                print("✗ %s：写意图一律拒 ⇒ rc=2 且一个字节不写" % f, file=sys.stderr)
            return 2

    if not os.path.isdir(os.path.join(args.root, "scripts", "gates")):
        print("✗ 不在仓根（scripts/gates/ 不存在）：%s ⇒ 拒绝静默空转" % args.root, file=sys.stderr)
        return 2

    gates = args.gate or list_gates(args.root)
    if not gates:
        if not args.json_only:
            print("✗ 门禁清单为空（--gate 未点名且 gates/ 无 *.py）⇒ 不给结论", file=sys.stderr)
        return 2

    if args.list:
        for g in gates:
            print(g)
        return 0

    registry_extra = ["--registry", args.registry] if args.registry else []
    results = []
    any_nonzero = False
    for g in gates:
        gate_path = os.path.join(args.root, "scripts", "gates", g)
        if not os.path.isfile(gate_path):
            results.append({"gate": g, "rc": 2, "elapsed_s": 0.0, "errored": True,
                            "error": "门禁文件不存在", "key_lines": [],
                            "registered_falses": False})
            any_nonzero = True
            continue
        per_gate = registry_extra + (["--json"] if REGISTRY_GATE in g else [])
        rec = run_gate(gate_path, per_gate, args.root)
        key_lines = extract_key_lines(rec["stdout"], args.key_regex)
        is_ff, ff = extract_registered_falses(rec["stdout"], g)
        if isinstance(ff, dict) and ff.get("unreadable"):
            print("✗ %s：%s ⇒ 拒给结论（rc=2）" % (g, ff.get("reason", "")), file=sys.stderr)
            return 2
        # 该门自己的 JSON（若吐）原样收进 raw
        raw = None
        for ln in rec["stdout"].splitlines():
            if _looks_like_json(ln):
                try:
                    raw = json.loads(ln)
                    break
                except Exception:
                    continue
        rec_out = {
            "gate": g,
            "rc": rec["rc"],
            "elapsed_s": rec["elapsed_s"],
            "errored": rec["errored"],
            "stderr": (rec["stderr"] or "").strip()[-500:],
            "key_lines": key_lines,
            "registered_falses": bool(is_ff),
            "registered_falses_detail": ff,
        }
        if raw is not None:
            rec_out["raw_json"] = raw
        results.append(rec_out)
        if rec["rc"] != 0:
            any_nonzero = True

    summary = {
        "schema": "gate-report/1",
        "repo": os.path.abspath(args.root),
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "gates_run": len(results),
        "any_pending": any_nonzero,
        "passed": [r["gate"] for r in results if r["rc"] == 0],
        "pending": [r["gate"] for r in results if r["rc"] != 0],
        "results": results,
    }

    # 落盘（唯一被允许的写）
    if args.dump:
        try:
            with open(args.dump, "w", encoding="utf-8") as fh:
                json.dump(summary, fh, ensure_ascii=False, indent=2)
        except Exception as e:
            print("✗ 落不了盘 %s：%s ⇒ 不给结论" % (args.dump, e), file=sys.stderr)
            return 2

    if args.json_only:
        print(json.dumps(summary, ensure_ascii=False))
        return 0 if not any_nonzero else 1

    # human 面
    print("gate-report · 仓 %s · %d 门" % (args.root, len(results)))
    print("─" * 72)
    for r in results:
        status = "PASS" if r["rc"] == 0 else ("ERR" if r["rc"] == 2 else "PENDING")
        tag = " [假红·已登记]" if r["registered_falses"] else ""
        print("%-34s rc=%s  %6.3fs  %s%s" % (r["gate"], r["rc"], r["elapsed_s"], status, tag))
        for kl in r["key_lines"][:3]:
            print("        · %s" % kl)
        if r.get("errored"):
            print("        ✗ %s" % r.get("error", ""))
        elif r["stderr"]:
            print("        ! %s" % r["stderr"].splitlines()[-1][:160])
    print("─" * 72)
    verdict = "无待办（全部门禁绿）" if not any_nonzero else "有待办（下列门禁 rc≠0）"
    print("结论：%s ⇒ rc=%d" % (verdict, 0 if not any_nonzero else 1))
    if any_nonzero:
        pend = [r["gate"] for r in results if r["rc"] != 0]
        print("  待办门禁：%s" % (", ".join(pend) if pend else "—"))
    return 0 if not any_nonzero else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

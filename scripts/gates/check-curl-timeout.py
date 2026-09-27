#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-curl-timeout.py —— 脚本面口径：**凡 `curl` 真调用必须带上界**（缺口 Q-199）

病象（逐字见 `Zerg-内部文档/项目文档/v2.5.13/缺口总账-20260925.md` §附录四十四 表内 `Q-199`）：
    「macOS 无 `timeout` ✗：用 `curl -m 240` 与后台 ＋ process poll 绕开」
    ⇒ 口径未写死 ⇒ **下一处照样会漏**（现读真调用里已漏 `1` 处）。

本件只判**真调用**（判据现跑）：
    · 真调用 = `.sh` 件里的 `curl `（整行注释 `#` 开头的**不算**）＋ `.py` 件里同行出现
      `subprocess` / `os.system` / `check_output` / `check_call` / `run(` 的那一类；
    · **夹具字面量 / 注释不算**（例：`check-public-tree-hazards.py` 的元组字面量、
      `check-public-marks.py` 的 `"#!/bin/sh\\ncurl …"` 夹具串、`zerg-upgrade.sh` 的注释）——
      这一档**只登记、不判红**，逐行打印出来供人复核（不判 = 不静默，两态分得开）。
    · 判据：**真调用逐行都带 `-m`**。漏一处 ⇒ **rc=1 且逐行点名**；全带上界 ⇒ rc=0。
    · 缺件 / 读不到扫描根 ⇒ **rc=2 不给结论**（BLOCKED，不许当绿 ✗）。

用法：
    python3 scripts/gates/check-curl-timeout.py [--root <目录>] [--json]
    （`--root` 只为自检/负控用：指一个临时目录 ⇒ 坏样件必须被点名且 rc=1。）
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
)
PY_HINT = re.compile(r"subprocess|os\.system|check_output|check_call|Popen|\.run\(")
CURL = re.compile(r"(^|[\s;|&(])curl\s")


def scan(root):
    """回 (real, fixtures)：real 里每项 (相对路径, 行号, 行文本)；fixtures 同形。"""
    real, fixtures = [], []
    if not os.path.isdir(root):
        return None, None
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in (".git", "node_modules", "__pycache__")]
        for fn in sorted(filenames):
            if not fn.endswith((".sh", ".py")):
                continue
            p = os.path.join(dirpath, fn)
            try:
                text = open(p, encoding="utf-8", errors="replace").read()
            except OSError:
                continue
            rel = os.path.relpath(p, root)
            for i, ln in enumerate(text.splitlines(), 1):
                if not CURL.search(ln):
                    continue
                s = ln.strip()
                is_comment = s.startswith("#")
                is_py = fn.endswith(".py")
                # .py 里真调用要同行有 subprocess 之类；否则一律算夹具/字面量
                looks_real = (not is_py) or bool(PY_HINT.search(ln))
                if is_comment or not looks_real:
                    fixtures.append((rel, i, s))
                else:
                    real.append((rel, i, s))
    return real, fixtures


def has_bound(line):
    # `-m <秒>` / `--max-time <秒>` / `--max-time=<秒>`
    return bool(re.search(r"(^|\s)(-m|--max-time)[=\s]?\s*[0-9]", line))


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────
# ── `Q-176` 第六批：`--self-test` 三档（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
#    纪律：合成件**全在临时目录**（`tempfile.mkdtemp` + `finally` 清理）⇒ 不读不写仓内件 ·
#          子进程**真跑本件**取真退出码（不猜）；写 stderr 一律 `print(…, file=sys.stderr)`（不用 `sys.stderr.write`）。
SELF_PATH = os.path.abspath(__file__)


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """三档：合成件全在临时目录（`--root` 指夹具根 · 正控 0 · 负控 1 · 缺件 2）。"""
    bad = [0]
    print("── check-curl-timeout.py --self-test 三档（正控 0 · 负控 1 · 缺件/用法错 2）")
    tmp = tempfile.mkdtemp(prefix="curltimeout-selftest-")
    try:
        ok_d, bad_d = os.path.join(tmp, "ok"), os.path.join(tmp, "bad")
        os.makedirs(ok_d); os.makedirs(bad_d)
        with open(os.path.join(ok_d, "ok.sh"), "w", encoding="utf-8") as f:
            f.write("curl -m 5 https://example.invalid/x\n")
        with open(os.path.join(bad_d, "bad.sh"), "w", encoding="utf-8") as f:
            f.write("curl https://example.invalid/x\n")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：curl 真调用带 `-m` ⇒ rc=0", 0, *run("--root", ok_d))
        _st_chk(bad, "档② 负控：curl 真调用漏上界 ⇒ rc=1", 1, *run("--root", bad_d))
        _st_chk(bad, "档③ 缺件：`--root` 读不到 ⇒ rc=2（不给结论）", 2, *run("--root", os.path.join(tmp, "no-such")))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=os.path.join(REPO_ROOT, "scripts"))
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()

    real, fixtures = scan(a.root)
    if real is None:
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % a.root, file=sys.stderr)
        return 2

    miss = [(p, i, s) for (p, i, s) in real if not has_bound(s)]
    if a.json:
        print(json.dumps({
            "schema": "zerg/v1", "kind": "CurlTimeoutScan", "root": a.root,
            "real_calls": len(real), "without_bound": len(miss),
            "fixture_or_comment": len(fixtures),
            "missing": [{"file": p, "line": i} for (p, i, _) in miss],
        }, ensure_ascii=False))
        return 1 if miss else 0

    print("扫描根：%s" % a.root)
    print("真调用 %d 行 · 其中**漏上界 %d 行** · 夹具/注释 %d 行（只登记、不判红）"
          % (len(real), len(miss), len(fixtures)))
    if miss:
        print("⇒ 漏上界的真调用（逐行点名）：")
        for p, i, s in miss:
            print("  %s:%d  %s" % (p, i, s[:120]))
        print("⇒ 口径：凡 curl 真调用必须带 `-m <秒>`（长跑用大值 ＋ 后台 ＋ process poll）"
              "—— rc=1")
        return 1
    print("⇒ 真调用逐行都带 `-m`（口径成立）—— rc=0")
    print("★ 夹具/注释档照实列出（不判红 · 供复核）：")
    for p, i, s in fixtures:
        print("  [夹具/注释] %s:%d  %s" % (p, i, s[:100]))
    return 0


if __name__ == "__main__":
    sys.exit(main())

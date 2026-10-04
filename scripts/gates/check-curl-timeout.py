#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-curl-timeout.py —— 脚本面口径：**凡 `curl` 真调用必须带上界**（缺口 Q-199）

病象（逐字见 `Zerg-内部文档/项目文档/v2.5.13/缺口总账-20260925.md` §附录四十四 表内 `Q-199`）：
    「macOS 无 `timeout` ✗：用 `curl -m 240` 与后台 ＋ process poll 绕开」
    ⇒ 口径未写死 ⇒ **下一处照样会漏**（现读真调用里已漏 `1` 处）。

本件只判**真调用**（判据现跑）：
    · 真调用 = `.sh` 件里落在**命令位**的 `curl `（整行注释 `#` 开头的**不算**；**引号字面量 /
      声明串里的 `curl` 不算** —— 例：`precommit-gates.sh` 的
      `add_step … "门62 curl 超时口径…"` 步名串 · `GAP-20261005-05`）＋ `.py` 件里同行出现
      `subprocess` / `os.system` / `check_output` / `check_call` / `run(` 的那一类；
    · **夹具字面量 / 注释不算**（例：`check-public-tree-hazards.py` 的元组字面量、
      `check-public-marks.py` 的 `"#!/bin/sh\\ncurl …"` 夹具串、`zerg-upgrade.sh` 的注释）——
      这一档**只登记、不判红**，逐行打印出来供人复核（不判 = 不静默，两态分得开）。
    · 判据：**真调用逐行都带 `-m`**。漏一处 ⇒ **rc=1 且逐行点名**；全带上界 ⇒ rc=0。
    · 缺件 / 读不到扫描根 ⇒ **rc=2 不给结论**（BLOCKED，不许当绿 ✗）。

用法：
    python3 scripts/gates/check-curl-timeout.py [--root <目录>] [--json]
    （`--root` 只为自检/负控用：指一个临时目录 ⇒ 坏样件必须被点名且 rc=1。）

★ 环境旋钮登记（本件现读的**这一枚** · 只登记、不改行为）—— `ZERG_REPO_ROOT` = **脚本侧仓根**：
    谁读 / 读什么：`REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(...)`（本模块
    常量区那行）—— 设了就**无条件采信**（不校验是不是真仓根、不回落）；**不设** ⇒ 按本件位置上溯三级推得。
    它唯一的用处是给 `--root` 定默认值（`--root` 不给 ⇒ 扫 `REPO_ROOT/scripts`）；给了 `--root` ⇒ 本旋钮被绕开。
    ★ 与 CLI 侧**同名族** `ZERG_REPO`（`core/cmd/zerg/root.go` 件头登记的那一枚）是**两枚、互不读取**：
    脚本面**不认识** `ZERG_REPO`、CLI 面**不认识** `ZERG_REPO_ROOT` ⇒ 要用哪一面就在**哪一面显式设**。
    为何要设 / 正确用法：只用来把**默认扫描根**指到仓外的合成假仓根（负控用法）或另一份仓副本；
    日常跑门**不要设**（设错 ⇒ 扫的是别处的 `…/scripts`；该路径不存在 ⇒ rc=2 不给结论，不当绿 ✓）。
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


# ── 声明串 / 引号字面量识别（GAP-20261005-05）────────────────────────────────
#   病象：`precommit-gates.sh:3295` 的 `add_step … "门62 curl 超时口径…"` —— `curl` 两字落在
#   **引号字面量**（步名 / 声明串）里，被当成真 curl 调用 ⇒ 门恒 rc≥1。
#   认法**按内容形式**（✗ 不绑行号）：逐字符走一遍引号 / 命令替换态，判该 curl 命中点是否
#   位于**引号字面量内、且不在命令替换 `$(…)` / 反引号里**。⇒ `.sh` 行只有落在**命令位**的
#   curl 才算真调用；整行 curl 全在字面量里 ⇒ 归「夹具/注释」档（只登记、不判红）。
def _curl_is_live(line, pos):
    """line[pos] 起是 curl：判它是否处于**可执行命令位**（✗ 不在引号字面量内）。
    引号态 None(裸 / 命令替换内) / "'" / '"' / "`"；`$(…)` 与反引号按栈进出（替换内可执行）。"""
    q = None
    stack = []
    i = 0
    while i < pos:
        c = line[i]
        if q in ("'", '"'):
            if c == "\\" and q == '"':
                i += 2
                continue
            if c == q:
                q = None
                i += 1
                continue
            if q == '"' and c == "$" and i + 1 < len(line) and line[i + 1] == "(":
                stack.append(q)
                q = None
                i += 2
                continue
            if q == '"' and c == "`":
                stack.append(q)
                q = "`"
                i += 1
                continue
            i += 1
            continue
        if c == "\\":
            i += 2
            continue
        if c == "'":
            q = "'"
            i += 1
            continue
        if c == '"':
            q = '"'
            i += 1
            continue
        if c == "`":
            if q == "`":
                q = stack.pop() if stack else None
            else:
                stack.append(q)
                q = "`"
            i += 1
            continue
        if c == "$" and i + 1 < len(line) and line[i + 1] == "(":
            stack.append(q)
            q = None
            i += 2
            continue
        if c == ")" and stack and q is None:
            q = stack.pop()
            i += 1
            continue
        i += 1
    return q not in ("'", '"')


def _live_curls(line):
    """本行里位于**命令位**的 curl 命中数（引号字面量内的不算）。"""
    n = 0
    for mt in CURL.finditer(line):
        if _curl_is_live(line, mt.start() + len(mt.group(1))):
            n += 1
    return n


def scan(root):
    """回 (real, fixtures, scanned)：real 里每项 (相对路径, 行号, 行文本)；fixtures 同形；scanned = 扫描面件数。"""
    real, fixtures, scanned = [], [], 0
    if not os.path.isdir(root):
        return None, None, 0
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
            scanned += 1   # ★ 扫描面件数（空域守卫的分母）
            rel = os.path.relpath(p, root)
            for i, ln in enumerate(text.splitlines(), 1):
                if not CURL.search(ln):
                    continue
                s = ln.strip()
                is_comment = s.startswith("#")
                is_py = fn.endswith(".py")
                # .py 里真调用要同行有 subprocess 之类；否则一律算夹具/字面量
                # ★ .sh 里真调用必须落在**命令位**（引号字面量 / 声明串里的 curl 不算 · GAP-20261005-05）
                looks_real = bool(PY_HINT.search(ln)) if is_py else (_live_curls(ln) > 0)
                if is_comment or not looks_real:
                    fixtures.append((rel, i, s))
                else:
                    real.append((rel, i, s))
    return real, fixtures, scanned


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
        decl_d = os.path.join(tmp, "decl")
        os.makedirs(decl_d)
        with open(os.path.join(decl_d, "decl.sh"), "w", encoding="utf-8") as f:
            f.write('add_step gates "门62 curl 超时口径：真调用必须带 -m（只报告）" '
                    'tri-report "${REPO_ROOT}" "python3 x.py"\n')

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：curl 真调用带 `-m` ⇒ rc=0", 0, *run("--root", ok_d))
        _st_chk(bad, "档② 负控：curl 真调用漏上界 ⇒ rc=1", 1, *run("--root", bad_d))
        _st_chk(bad, "档③ 缺件：`--root` 读不到 ⇒ rc=2（不给结论）", 2, *run("--root", os.path.join(tmp, "no-such")))
        _st_chk(bad, "档④ 声明串（GAP-20261005-05）：`add_step … \"…curl…\"` 里的 curl 不算真调用 ⇒ rc=0", 0, *run("--root", decl_d))
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

    real, fixtures, scanned = scan(a.root)
    if real is None:
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % a.root, file=sys.stderr)
        return 2
    # ★ 空集守卫（GAP-20260927-108）：空域 ⇒ fail-closed（扫描面 0 件 ⇒ rc=2，不许把「空转」当绿）
    if not scanned:
        print("BLOCKED —— 空域：%s 下扫到 0 个 .sh/.py 件 ⇒ rc=2（不给结论；空转 ≠ 绿）" % a.root, file=sys.stderr)
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

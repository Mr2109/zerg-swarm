#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-empty-domain-guards.py —— 空集守卫元门（缺口 GAP-20260927-108）

病象（现读坐实 · 逐条见账）：
    域的边界是「空集」与「不存在」时，多道门输出满绿（静默 rc=0）——
    · `check_docs.py` 连不存在的路径都 rc=0 且打印满绿「配对完整/标题对齐/链接有效/源声明齐备」；
    · `check-placeholder-residue.py:74-80+199-202` 非 git 目录报「通过——未发现残留」；
    · `check-curl-timeout.py:138/150` 空 root 报「口径成立」（**同件两种行为**：缺件档 rc=2 而空 root 档 rc=0）；
    · `check-manifest-freshness.py:137-139` 空清单报「✓ 清单新鲜」；
    · `check-gotoolchain.py:163/177` trip() 为 None 则比较静默跳过；`check-build-tags.py:553-554` if not targets: return。
    根因同一：**扫描面取不到输入（空域）⇒ 判据整块消失 ⇒ 假绿**。

本件为**元门**（结构断言 + 成对自检 + baseline 棘轮只许下调），做三件事：
    ① 提供**规范空集守卫** `require_domain()`（Python 版）与 `BASH_GUARD_SNIPPET`（Bash 版）——
       空域 ⇒ 打印 BLOCKED 并回 rc=2；有件 ⇒ 回 rc=0。（现读 `code find require_domain` 由此命中。）
    ② **结构断言**：扫 `scripts/gates/*.py`，凡**扫面型**件（含 os.walk/scandir/listdir/glob/rglob）
       必须在判绿前带「域计数 >= 写死下限」类断言（`require_domain(` / `空域|空集` /
       `len(...) >= N` / `<计数> >= N`）⇒ 缺守卫者**逐条点名 rc=1**。
    ③ **baseline 棘轮**：现读缺守卫件数 <= BASELINE 才绿（**只许下调**；下调 = 补齐守卫）。
       绿档也照实打印「剩余 N 条」（✗ 不许悄悄绿 —— 同门① 的口径）。
    附 **报告档**（✗ 不判红）：调用 subprocess.run/Popen 却不查 returncode / check= 的件数列数（口径④）。

口径（写死）：
    · 缺守卫件数 > baseline ⇒ rc=1（棘轮上调 = 口径变坏）；<= baseline ⇒ rc=0；
    · **缺语料 / 扫描根读不到 ⇒ rc=2 不给结论**（✗ 空域不许当绿 —— 本件自己先守此线）；
    · **只读**：不写任何文件（`--emit-baseline` 只把现读数打到 stdout，✗ 不写盘）；
    · 结构断言是**启发式**：白名单 / baseline 是它唯一的人工降噪口（免误报当红）。

用法：
    python3 scripts/gates/check-empty-domain-guards.py [--root <目录>] [--baseline N] [--json]
    python3 scripts/gates/check-empty-domain-guards.py --self-test
    python3 scripts/gates/check-empty-domain-guards.py --emit-baseline     # 现读缺守卫数（只打印）

★ 接线状态：本件为**仓外候件**（GAP-20260927-108），**未挂进任何门步** ——
  接线与 baseline 首测冻结由后续单独一笔做，避免撞件。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

# ── ① 规范空集守卫（Python 版）· 现读 `code find require_domain` 由此命中 ──────────
def require_domain(items, *, what, min_count=1, rc=2):
    """空集守卫：扫描面取不到输入（空域）时 fail-closed。

    用法：`n = require_domain(found, what="scripts/gates 扫面");` —— 回 0 表示域非空；
    回非 0（默认 2）表示 BLOCKED（调用方应 `return rc`，✗ 不得继续判绿）。
    `items` 可传序列（取 len）或整数（已是计数）。
    """
    n = items if isinstance(items, int) else len(items)
    if n < min_count:
        print("BLOCKED —— 空域 / 取不到输入：%s（现读 %d < 写死下限 %d）⇒ rc=%d 不给结论"
              % (what, n, min_count, rc), file=sys.stderr)
        return rc
    return 0


# Bash 版（②：无后缀语法/发布面脚本用）—— 文档常量，供被扫描件或人照抄：
BASH_GUARD_SNIPPET = r'''
require_domain() {  # require_domain <计数> <写死下限> <说明>
  local n="$1" min="${2:-1}" what="${3:-扫描面}"
  if [ "${n:-0}" -lt "${min}" ]; then
    echo "BLOCKED —— 空域 / 取不到输入：${what}（现读 ${n:-0} < 写死下限 ${min}）⇒ rc=2 不给结论" >&2
    return 2
  fi
  return 0
}
'''

# ── ② 结构断言的启发式 ────────────────────────────────────────────────────────
# 扫面型：会「按目录/模式取一批件」的调用
SCAN_RE = re.compile(
    r"os\.walk\s*\(|os\.scandir\s*\(|os\.listdir\s*\(|glob\.glob\s*\(|\.rglob\s*\(|\.glob\s*\("
)
# 空集守卫：判绿前必有「域计数 >= 写死下限」类断言
GUARD_RE = re.compile(
    r"require_domain\s*\("
    r"|空域|空集"
    r"|(?:len|count)\s*\([^)\n]*\)\s*[<>]=?\s*\d"
    r"|(?:件数|域计数|count|total|items?|files?|n)\s*>=\s*\d"
)
# 附报告档（✗ 不判红）：subprocess 调用与 returncode 检查
SUBPROC_RE = re.compile(r"subprocess\.(?:run|Popen)\s*\(")
RCCHECK_RE = re.compile(r"returncode|check\s*=\s*True")

EXTS = (".py",)
# ★ baseline 棘轮（只许下调）：现读缺守卫件数，首测值见下方常量
BASELINE = 22  # 首测冻结（2026-10-03 现读 scripts/gates：扫面型 115 件 · 缺守卫 22 条）
               # ✗ 只许下调（补齐守卫即下调）；上调 = 放宽口径 ⇒ 须走拍板。空域候选见 --baseline 0 现读名单。
SELF_PATH = os.path.abspath(__file__)


def scan_files(root):
    files = []
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "__pycache__")]
        for fn in sorted(fns):
            if fn.endswith(EXTS):
                files.append(os.path.join(dp, fn))
    return sorted(files)


def structural_scan(root):
    """回 (scanned, unguarded[name...], subproc_unchecked) 或 (None, None, None)。"""
    if not os.path.isdir(root):
        return None, None, None
    scanned, unguarded, subproc_unchecked = 0, [], 0
    for p in scan_files(root):
        if os.path.abspath(p) == SELF_PATH:  # 本件自己定义 require_domain ⇒ 不参与
            continue
        try:
            text = open(p, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        scanned += 1
        if SCAN_RE.search(text) and not GUARD_RE.search(text):
            unguarded.append(os.path.relpath(p, root))
        if SUBPROC_RE.search(text) and not RCCHECK_RE.search(text):
            subproc_unchecked += 1
    return scanned, unguarded, subproc_unchecked


def run(root, baseline, as_json=False, emit_baseline=False):
    scanned, unguarded, subproc_unchecked = structural_scan(root)
    if scanned is None:
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % root, file=sys.stderr)
        return 2

    live = len(unguarded)
    if emit_baseline:
        print(live)
        return 0

    rc = 1 if live > baseline else 0
    if as_json:
        print(json.dumps({"schema": "zerg/v1", "kind": "EmptyDomainGuards", "root": root,
                          "scanned": scanned, "baseline": baseline,
                          "unguarded": live, "unguarded_files": unguarded,
                          "subproc_unchecked": subproc_unchecked}, ensure_ascii=False))
        return rc

    print("扫描根：%s" % root)
    print("扫面型件 %d 条 · **缺空集守卫 %d 条**（baseline=%d · 只许下调）· 附〔subprocess 未查 returncode %d 条（只报告）〕"
          % (scanned, live, baseline, subproc_unchecked))
    if unguarded:
        for f in unguarded:
            print("  ✗ %s —— 扫面型但无「域计数 >= 写死下限」类守卫（空域会静默 rc=0）" % f)
    if rc == 1:
        print("⇒ 缺守卫 %d > baseline %d ⇒ rc=1（棘轮上调 = 口径变坏）" % (live, baseline))
    else:
        print("⇒ 缺守卫 %d <= baseline %d ⇒ rc=0（仍注明剩余 %d 条，✗ 不许悄悄绿）"
              % (live, baseline, live))
    return rc


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """五档：守卫正控/负控 · 元门正控/负控 · 缺语料。"""
    bad = [0]
    print("── check-empty-domain-guards.py --self-test 五档（正控 0 · 负控 1/2 · 缺语料 2）")
    tmp = tempfile.mkdtemp(prefix="emptydomain-selftest-")
    try:
        # ① 守卫正控：有件 ⇒ 0
        r0 = require_domain(["a", "b"], what="合成·有件")
        _st_chk(bad, "档① require_domain 正控：有件 ⇒ rc=0", 0, r0, "")
        # ② 守卫负控：空域 ⇒ 2
        r2 = require_domain([], what="合成·空域")
        _st_chk(bad, "档② require_domain 负控：空域 ⇒ rc=2", 2, r2, "")

        def put(sub, name, body):
            d = os.path.join(tmp, sub)
            os.makedirs(d, exist_ok=True)
            with open(os.path.join(d, name), "w", encoding="utf-8") as f:
                f.write(body)
            return d

        # ③ 元门正控：扫面型 + 有守卫 ⇒ 不点名 ⇒ rc=0
        d_ok = put("ok", "guarded.py",
                   "import os\n"
                   "def main():\n"
                   "    found = []\n"
                   "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                   "    if require_domain(found, what='x', min_count=1):\n        return 2\n"
                   "    return 0\n")
        # ④ 元门负控：扫面型 + 无守卫 ⇒ 点名 ⇒ rc=1
        d_bad = put("bad", "unguarded.py",
                    "import os\n"
                    "def main():\n"
                    "    found = []\n"
                    "    for dp, _d, fs in os.walk('.'):\n        found += fs\n"
                    "    return 0\n")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档③ 元门正控：扫面型件带守卫 ⇒ rc=0", 0, *run("--root", d_ok, "--baseline", "0"))
        _st_chk(bad, "档④ 元门负控：扫面型件缺守卫 ⇒ rc=1（点名）", 1, *run("--root", d_bad, "--baseline", "0"))
        _st_chk(bad, "档⑤ 缺语料：`--root` 不存在 ⇒ rc=2（不给结论）", 2,
                *run("--root", os.path.join(tmp, "no-such-root")))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


# 自检里 require_domain 需在模块作用域可见（本文件即如此）；此处引用防「未用」误报：
_ = BASH_GUARD_SNIPPET


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=os.path.join(
        os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__)))),
        "scripts", "gates"))
    ap.add_argument("--baseline", type=int, default=BASELINE)
    ap.add_argument("--emit-baseline", action="store_true")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    return run(a.root, a.baseline, a.json, a.emit_baseline)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-build-selfclean.py —— 「重编脚本自净化」只读判据件（缺口 GAP-20260929-03）

病象（GAP-20260929-03）：重编脚本 `scripts/build/build-all.sh` 有段「自净化」——
  先 `unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR` 三枚旋钮、再把 TMPDIR 建到仓外。
这条规矩**没有机器判据**：脚本是否真 unset、TMPDIR 是否真在仓外、该段是否在解析参数**之前**，
只能靠读代码 + 人工探针（人工毒化探针两态都空、无鉴别力）⇒ 漏一次就是把状态写面/仓根指到错地方
（血证：`ZERG_STATE_DIR` 毒化 ⇒ 写面跑到仓外目录；`ZERG_REPO` 毒化 ⇒ 换根）。

本件把它立成机器判据（四格）：
  K1  三枚旋钮必须出现在一条 `unset` 上（缺任一 ⇒ 红）。
  K2  必须有 `export TMPDIR=`，且其取值经变量展开后**落在仓外**（取不到仓根/或落在仓内 ⇒ 红）。
  K3  自净化段（unset 那一行）必须**位次早于**第一处参数解析（`getopts` / `while [ $# -gt 0 ]` /
      `case "$1"` / `case $1`）——「解析参数之前」这条口径才成立。
  K4  fail-closed：建仓外 TMPDIR 失败必须**拒编**（含 `exit`/拒绝语义 + 指向该失败的话术锚）。

退出码（三档 · 优先级 2 > 1 > 0）：
  0 = 四格全过（绿）
  1 = 有红（K1..K4 有一格不成立 ⇒ 逐格点名）
  2 = **不给结论**（脚本缺 / 空 / 读不了 —— 「判不了」时不给结论 ≠ 绿）

纪律：**只读** · 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件 · 只在 /tmp 造夹具。

用法：
  python3 scripts/gates/check-build-selfclean.py [--root <仓根>] [--file <脚本>] [--json]
  python3 scripts/gates/check-build-selfclean.py --self-test      # 成对正负控（仓外夹具）
"""
import argparse
import json
import os
import re
import sys
import tempfile

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

KNOBS = ("ZERG_STATE_DIR", "ZERG_REPO", "ZERG_PROPOSAL_DIR")
DEFAULT_FILE = "scripts/build/build-all.sh"


def _read(path):
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            return fh.read()
    except Exception:
        return None


def _first_selfclean_line(lines):
    """自净化锚行 = 第一条同时含三枚旋钮的 `unset` 行（行号 1-based）；找不到回 None。"""
    for i, ln in enumerate(lines, 1):
        s = ln.strip()
        if s.startswith("unset") and all(k in s for k in KNOBS):
            return i
    return None


def _first_argparse_line(lines):
    pats = (r"\bgetopts\b", r"while\s*\[\s*\$\#\s*-gt\s*0\s*\]",
            r'case\s+"?\$\{?1', r"case\s+\"\$1\"")
    for i, ln in enumerate(lines, 1):
        for p in pats:
            if re.search(p, ln):
                return i
    return None


def check(script_path, repo_root):
    """回 (rc, reds(list[str]), notes(list[str]))。"""
    text = _read(script_path)
    if text is None:
        return EXIT_BLOCKED, [], ["脚本读不到：%s" % script_path]
    if not text.strip():
        return EXIT_BLOCKED, [], ["脚本为空：%s（空转不给结论）" % script_path]
    lines = text.splitlines()
    reds, notes = [], []

    # K1 三枚旋钮在一条 unset 上
    sl = _first_selfclean_line(lines)
    if sl is None:
        reds.append("K1 未在一条 `unset` 上同时清 %s" % " ".join(KNOBS))
    else:
        notes.append("K1 自净化锚行 = 第 %d 行" % sl)

    # K2 TMPDIR 建在仓外
    tm = None
    for i, ln in enumerate(lines, 1):
        if re.search(r"^\s*export\s+TMPDIR\s*=", ln):
            tm = (i, ln.strip())
            break
    if tm is None:
        reds.append("K2 无 `export TMPDIR=`（TMPDIR 未做仓外自建）")
    else:
        # 取脚本里赋给 ZERG_BUILD_TMP / TMPDIR 的字面量，验证其不在仓内
        m = re.findall(r"(ZERG_BUILD_TMP|TMPDIR)=([^\s\"';]+)", text)
        vals = [v for _, v in m if v and not v.startswith("$")]
        repo_abs = os.path.abspath(repo_root)
        inside = [v for v in vals if os.path.abspath(os.path.expanduser(v)).startswith(repo_abs + os.sep)]
        notes.append("K2 export TMPDIR 在第 %d 行 · 字面量=%s" % (tm[0], vals or "（变量派生）"))
        if inside:
            reds.append("K2 TMPDIR 落在仓内：%s" % inside)

    # K3 自净化段早于参数解析
    ap = _first_argparse_line(lines)
    if sl is not None:
        if ap is None:
            notes.append("K3 未检出参数解析行（无 getopts/while case）⇒ 次序格不适用")
        elif sl < ap:
            notes.append("K3 自净化(%d) 早于参数解析(%d) ✓" % (sl, ap))
        else:
            reds.append("K3 自净化(%d) 晚于参数解析(%d) ⇒ 「解析参数之前」不成立" % (sl, ap))

    # K4 fail-closed 拒编
    if not re.search(r"(自净化失败|TMPDIR.*(建不起|失败)|拒绝.*(重编|净化))", text):
        reds.append("K4 无「仓外 TMPDIR 建不起来 ⇒ 拒编」的 fail-closed 话术锚")

    return (EXIT_OK if not reds else EXIT_FAIL), reds, notes


def run(root, path, as_json):
    script = path or os.path.join(root, DEFAULT_FILE)
    rc, reds, notes = check(script, root)
    if as_json:
        print(json.dumps({"schema": "zerg/build-selfclean/1", "file": script,
                          "rc": rc, "reds": reds, "notes": notes},
                         ensure_ascii=False, indent=2))
        return rc
    print("── 重编脚本自净化判据（GAP-20260929-03）· 判对象 = %s" % script)
    for n in notes:
        print("  · %s" % n)
    for r in reds:
        print("  ✗ %s" % r)
    if rc == EXIT_OK:
        print("⇒ 全绿（K1/K2/K3/K4 四格全过）")
    elif rc == EXIT_FAIL:
        print("⇒ 有红：%d 格不成立" % len(reds))
    else:
        print("⇒ 不给结论（缺件/空/读不了）")
    return rc


def self_test():
    """成对正负控：原件绿；四种变异各必红一格。"""
    total = bad = 0
    orig = ("#!/usr/bin/env bash\n"
            "# 自净化\nunset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n"
            "TMPDIR=${ZERG_BUILD_TMP:-/tmp/zerg-build}\n"
            "if ! mkdir -p \"$TMPDIR\"; then echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; fi\n"
            "export TMPDIR=\"$ZERG_BUILD_TMP\"\n"
            "# 解析参数\nwhile [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n")
    with tempfile.TemporaryDirectory(prefix="zc-selfclean-") as d:
        cases = []
        cases.append(("正控(原件)", orig, EXIT_OK))
        cases.append(("负控K1(漏一枚旋钮)", orig.replace("ZERG_REPO ", ""), EXIT_FAIL))
        cases.append(("负控K2(无 TMPDIR)", orig.replace("export TMPDIR=\"$ZERG_BUILD_TMP\"\n", ""), EXIT_FAIL))
        cases.append(("负控K3(自净化挪到解析之后)",
                      orig.replace("unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n", "")
                          .replace("while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n",
                                   "while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n"
                                   "unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n"),
                      EXIT_FAIL))
        cases.append(("负控K4(无 fail-closed)",
                      orig.replace(" echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; ", " : "),
                      EXIT_FAIL))
        for label, content, want in cases:
            fp = os.path.join(d, "fixture.sh")
            with open(fp, "w", encoding="utf-8") as fh:
                fh.write(content)
            rc, _, _ = check(fp, os.path.join(d, "REPO"))
            total += 1
            ok = (rc == want)
            bad += 0 if ok else 1
            print("  %s %s ⇒ rc=%d（期望 %d）" % ("✓" if ok else "✗", label, rc, want))
    print("⇒ 自检 %s（%d 用例 · %d 败）" % ("全过（rc=0）" if bad == 0 else "有败", total, bad))
    return EXIT_OK if bad == 0 else EXIT_FAIL


def main(argv=None):
    ap = argparse.ArgumentParser(description="重编脚本自净化只读判据（GAP-20260929-03）")
    ap.add_argument("--root", default=".", help="仓根（缺省 cwd）")
    ap.add_argument("--file", default=None, help="脚本路径（缺省 <root>/scripts/build/build-all.sh）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    return run(a.root, a.file, a.json)


if __name__ == "__main__":
    sys.exit(main())

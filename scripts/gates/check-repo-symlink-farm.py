#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-repo-symlink-farm.py —— 家目录「指向本仓的符号链接」只读判据（缺口 GAP-20260928-44 · 第二腿）

病象（逐字见缺口 `GAP-20260928-44` 的 `want_action`）：
    家目录里出现**指向真仓的符号链接**时，工具与门会把**家目录链接当仓根** —— 拿链接当仓根、
    门按错根扫语料 ⇒ 全树读数失真而无人报。历史正因此让读数失真（链接本体 2026-09-28 已删，
    但无任何门会判「家目录里有指向本仓的链接」⇒ 该形态可再次静默出现）。

判据（**只读** · 可挂进既有门，不新开一道）：
    · 扫点名根（`$HOME` 浅层）里**指向本仓路径**的符号链接；命中 ⇒ **判红**（rc=1）＋
      逐条打印「链接 → 解析后目标」与处置建议（删链接 / 登记）；
    · ★ **只判「指向本仓」**，**✗ 不判**「家目录里有没有链接」（后者是正常用法，判了必误伤）；
    · 取不到目标面（`$HOME` 读不到 / 本仓根解析不出）⇒ **不给结论**（rc=2），与同族 rc=2 口径一致。

退码（三档 · fail-closed · 与同族只读判据一致）：
    0 = 浅层无「指向本仓」的符号链接
    1 = 命中（判红）：浅层**存在**指向本仓的符号链接（逐条打印）
    2 = **不给结论**：`$HOME` 读不到 / 本仓根解析不出 / `--self-test` 不过

用法：
    python3 scripts/gates/check-repo-symlink-farm.py                 # 扫本机 $HOME 浅层 ⟷ 本仓
    python3 scripts/gates/check-repo-symlink-farm.py --repo <本仓根>  # 显式本仓根
    python3 scripts/gates/check-repo-symlink-farm.py --home <目录>    # 显式扫描根（负控/夹具用）
    python3 scripts/gates/check-repo-symlink-farm.py --json          # 机器面（同结构多渲染）
    python3 scripts/gates/check-repo-symlink-farm.py --self-test     # 成对负控自检

★ 环境旋钮登记（只登记、只用于把**默认扫描根**指到别处，日常不要设）：
    `ZERG_SYM_SCAN_HOME` = 扫描根（缺省 `$HOME`）；`ZERG_REPO_ROOT` = 本仓根（缺省按本件位置上溯三级）。
"""
import argparse
import json
import os
import sys

# ── 真源（缺省）───────────────────────────────────────────────────────
REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# 浅层深度：$HOME 本身（1）+ 一层子目录（2）。更深不扫（家目录越深越易误伤，且历史形态即在浅层）。
MAX_DEPTH = 2
# 跳过目录名（隐藏目录 / macOS 系统家目录常客）—— ✗ 不判其中的链接，避免误伤。
SKIP_DIRNAMES = {"Library", ".Trash", "Applications", ".git", "node_modules"}


def _norm(path):
    """规范化：去尾斜杠 + realpath 失败则退回 abspath（不抛）。"""
    try:
        return os.path.realpath(path)
    except OSError:
        return os.path.abspath(path)


def _is_within(child, parent):
    """child 是否在 parent 之内（含相等）。用 realpath 后按成分前缀比（避免 /a/bc 命中 /a/b）。"""
    child = _norm(child)
    parent = _norm(parent)
    if child == parent:
        return True
    return child.startswith(parent.rstrip(os.sep) + os.sep)


def scan(home, repo):
    """扫 home 浅层（depth ≤ MAX_DEPTH）的符号链接，收「解析后目标指向 repo」的条目。

    ⇒ (hits, notes)；hits 每项 = {"link": <链接路径>, "target": <realpath 目标>}。
    """
    hits, notes = [], []
    if not os.path.isdir(home):
        return None, ["扫描根读不到：%s ⇒ 不给结论" % home]
    repo_real = _norm(repo)
    base_depth = home.rstrip(os.sep).count(os.sep)
    for dirpath, dirnames, filenames in os.walk(home, followlinks=False):
        depth = dirpath.rstrip(os.sep).count(os.sep) - base_depth
        if depth >= MAX_DEPTH:
            dirnames[:] = []
        else:
            dirnames[:] = [d for d in dirnames if d not in SKIP_DIRNAMES and not d.startswith(".")]
        for name in sorted(filenames + dirnames):
            p = os.path.join(dirpath, name)
            if not os.path.islink(p):
                continue
            try:
                resolved = _norm(p)  # realpath 跟随链接
            except OSError:
                continue
            if _is_within(resolved, repo_real):
                hits.append({"link": p, "target": resolved})
    notes.append("扫描根=%s（浅层 depth≤%d）· 本仓根=%s" % (home, MAX_DEPTH, repo_real))
    return hits, notes


def run_check(home, repo):
    """⇒ (rc, report)。"""
    rep = {"home": home, "repo": repo, "hits": [], "notes": [], "problems": []}
    hits, notes = scan(home, repo)
    rep["notes"] = notes
    if hits is None:
        rep["problems"].append("扫描根读不到 ⇒ 不给结论")
        return 2, rep
    rep["hits"] = hits
    if hits:
        rep["problems"].append("家目录浅层存在**指向本仓**的符号链接 %d 条 ⇒ 判红" % len(hits))
        return 1, rep
    return 0, rep


def cmd_self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as tmp:
        home = os.path.join(tmp, "home")
        repo = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(repo, "core"), exist_ok=True)
        os.makedirs(home, exist_ok=True)

        # 正控①：无链接 ⇒ rc=0
        rc, _ = run_check(home, repo)
        if rc != 0:
            print("✗ 正控①：干净家目录却判红 rc=%s" % rc)
            ok = False
        else:
            print("✓ 正控①：干净家目录 ⇒ rc=0")

        # 负控①：浅层直挂「指向本仓」的链接 ⇒ rc=1
        link1 = os.path.join(home, "zerg-repo-symlink")
        os.symlink(repo, link1)
        rc, rep = run_check(home, repo)
        if rc != 1 or len(rep["hits"]) != 1:
            print("✗ 负控①：指向本仓的链接未被判红 rc=%s hits=%s" % (rc, rep["hits"]))
            ok = False
        else:
            print("✓ 负控①：指向本仓的链接 ⇒ rc=1（%s）" % rep["hits"][0]["link"])
        os.remove(link1)

        # 负控②：指向**别处**的链接 ⇒ 不判红（只判「指向本仓」）
        outside = os.path.join(tmp, "elsewhere")
        os.makedirs(outside, exist_ok=True)
        link2 = os.path.join(home, "elsewhere-link")
        os.symlink(outside, link2)
        rc, _ = run_check(home, repo)
        if rc != 0:
            print("✗ 负控②：指向别处的链接被误判 rc=%s" % rc)
            ok = False
        else:
            print("✓ 负控②：指向别处的链接 ⇒ 不判红（rc=0）")
        os.remove(link2)

        # 负控③：二层子目录里的「指向本仓」链接 ⇒ 判红（浅层 depth≤2 覆盖）
        sub = os.path.join(home, "work")
        os.makedirs(sub, exist_ok=True)
        link3 = os.path.join(sub, "repo0")
        os.symlink(repo, link3)
        rc, rep = run_check(home, repo)
        if rc != 1:
            print("✗ 负控③：二层子目录里的指向本仓链接未被判红 rc=%s" % rc)
            ok = False
        else:
            print("✓ 负控③：二层子目录里的指向本仓链接 ⇒ rc=1")
        os.remove(link3)

        # 负控④：扫描根不存在 ⇒ rc=2（不给结论，不当绿）
        rc, _ = run_check(os.path.join(tmp, "nope"), repo)
        if rc != 2:
            print("✗ 负控④：扫描根不存在却给了结论 rc=%s" % rc)
            ok = False
        else:
            print("✓ 负控④：扫描根不存在 ⇒ rc=2（不给结论）")

        # 负控⑤：/a/bc 成分前缀不得被 /a/b 命中
        if _is_within(os.path.join(tmp, "abc"), os.path.join(tmp, "ab")):
            print("✗ 负控⑤：路径前缀误命中（/abc 命中 /ab）")
            ok = False
        else:
            print("✓ 负控⑤：成分前缀比正确（/abc ⊄ /ab）")
    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="家目录「指向本仓的符号链接」只读判据（GAP-20260928-44）")
    ap.add_argument("--home", default=os.environ.get("ZERG_SYM_SCAN_HOME") or os.path.expanduser("~"))
    ap.add_argument("--repo", default=REPO_ROOT)
    ap.add_argument("--json", action="store_true", help="机器面（只印 JSON）")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return cmd_self_test()
    home = os.path.abspath(os.path.expanduser(args.home))
    repo = os.path.abspath(args.repo)
    rc, rep = run_check(home, repo)
    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2))
        return rc
    print("check-repo-symlink-farm —— 家目录「指向本仓的符号链接」只读判据")
    for n in rep["notes"]:
        print("  · " + n)
    if rep["hits"]:
        for h in rep["hits"]:
            print("  ✗ 链接 → 目标：%s → %s" % (h["link"], h["target"]))
            print("     建议：删该链接（rm '%s'），或若确需保留则登记为已知用法" % h["link"])
    if rep["problems"]:
        print("✗ 判红 %d 条：" % len(rep["problems"]))
        for p in rep["problems"]:
            print("  · " + p)
        return rc
    print("✓ 全绿（rc=0）：家目录浅层无「指向本仓」的符号链接")
    return rc


if __name__ == "__main__":
    sys.exit(main())

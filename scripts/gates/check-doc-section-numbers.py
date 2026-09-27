#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-section-numbers.py —— 「脏件结构性体检」两格（缺口 Q-201）

病象（逐字见 `Zerg-内部文档/项目文档/v2.5.13/缺口总账-20260925.md` §附录四十四 表内 `Q-201`）：
    「无『未提交改动的结构性体检』✗：脏件只能靠**人现读** ＋ `grep -c '^## 6\\.'` 数编号冲突
      ⇒ 判『半截』**无机器判据** ✗」⇒ 修法 = 一条**只读**读数，出照回执原文的**两格**：
        ① **标题号重复**（同号标题出现两次以上 ⇒ 重编号是半截的）
        ② **被引用节号缺失**（正文里的 `§<号>` 指不到任何标题 ⇒ 引用了不存在的节）

口径（写死）：
    · 两格**各出逐条名单 ＋ 一个 rc**；两格皆空 ⇒ **rc=0**；任一条命中 ⇒ **rc=1 并逐行点名**；
    · **缺件 / 读不到 ⇒ rc=2 不给结论**（**不许当绿** ✗）；
    · **只读**：本件不写任何文件（**无副作用** · 可任意复跑）；
    · 号形：标题取 `^#+\\s*§?<号>`（`§附录四十五` / `§12` / `12.3` / `一` 这类中文号都收），
      引用取正文里的 `§<号>`（含 `§附录<号>`）。

用法：
    python3 scripts/gates/check-doc-section-numbers.py <件.md> [<件2.md> ...] [--json]
    （**负控**：把一份好件里某个节号改重 ⇒ 该格当场非零且点名。）
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HEAD = re.compile(r"^\s{0,3}#{1,6}\s*§?\s*([附录一二三四五六七八九十百零〇0-9][附录一二三四五六七八九十百零〇0-9.\-]*?)(?=[\s·、（(．]|$)")
REF = re.compile(r"§\s*([附录一二三四五六七八九十百零〇0-9][附录一二三四五六七八九十百零〇0-9.\-]*?)(?=[\s·、）)，,。；;:：]|$)")
APPENDIX = re.compile(r"^附录([一二三四五六七八九十百零〇0-9.]+)$")


def norm(s):
    """归一号形：`附录四十五` / `45` / `12.3` ⇒ 可比对键（去掉 `附录` 前缀与空白）。"""
    s = s.strip()
    if s.startswith("附录"):
        s = s[len("附录"):]
    return s.strip()


def check(path):
    try:
        text = open(path, encoding="utf-8", errors="replace").read()
    except OSError as e:
        return None, str(e)

    heads = []  # (行号, 号, 逐字标题)
    for i, ln in enumerate(text.splitlines(), 1):
        m = HEAD.match(ln)
        if m:
            heads.append((i, norm(m.group(1)), ln.strip()[:80]))

    seen, dup = {}, []
    for i, num, raw in heads:
        seen.setdefault(num, []).append((i, raw))
    for num, occ in seen.items():
        if len(occ) > 1:
            dup.append((num, occ))

    have = set(seen)
    has_appendix = any(APPENDIX.match(h) or h.isdigit() for h in have) and any(
        k.startswith("附录") for k in have)
    in_file, cross = [], []  # ②a 件内（判红） / ②b 跨件或自由引用（只登记，不判红）
    for i, ln in enumerate(text.splitlines(), 1):
        for m in REF.finditer(ln):
            num = norm(m.group(1))
            if not num or num in ("见", "下", "上"):
                continue
            target = ("附录" + num) if (has_appendix and not num.startswith("附录")) else num
            # 件内档：本件确有「附录」系标题 ⇒ 只判 `§附录<号>` 这一类（同件互指的硬约定）
            if m.group(1).startswith("附录"):
                if num not in have:
                    in_file.append((i, "附录" + num, ln.strip()[:100]))
            elif num in have:
                continue
            else:
                cross.append((i, num, ln.strip()[:100]))

    return {
        "file": path,
        "headings": len(heads),
        "duplicate_titles": [{"num": n, "lines": [i for i, _ in occ]} for n, occ in dup],
        "dangling_refs": [{"num": n, "line": i, "text": t} for i, n, t in in_file],
        "cross_refs": len(cross),
    }, None


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
    """三档：合成 md 全在临时目录（正控 0 · 负控 1 · 用法错 2）。"""
    bad = [0]
    print("── check-doc-section-numbers.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
    tmp = tempfile.mkdtemp(prefix="docsec-selftest-")
    try:
        clean = os.path.join(tmp, "clean.md")
        with open(clean, "w", encoding="utf-8") as f:
            f.write("## 附录一\n正文甲\n\n## 附录二\n正文乙\n")
        dup = os.path.join(tmp, "dup.md")
        with open(dup, "w", encoding="utf-8") as f:
            f.write("## 附录一\n正文甲\n\n## 附录一\n正文乙\n")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：标题号不重复、无悬空 `§` 引用 ⇒ rc=0", 0, *run(clean))
        _st_chk(bad, "档② 负控：同一标题号出现两次 ⇒ rc=1", 1, *run(dup))
        _st_chk(bad, "档③ 用法错：不给件 ⇒ rc=2", 2, *run())
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("files", nargs="*")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    if not a.files:
        print("用法错：至少要给一件（rc=2）", file=sys.stderr)
        return 2

    results, blocked = [], []
    for f in a.files:
        r, err = check(f)
        if r is None:
            blocked.append((f, err))
        else:
            results.append(r)

    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "DocSectionNumbers",
                          "results": results,
                          "blocked": [{"file": f, "err": e} for f, e in blocked]},
                         ensure_ascii=False))
        if blocked:
            return 2
        bad = any(r["duplicate_titles"] or r["dangling_refs"] for r in results)
        return 1 if bad else 0

    for f, err in blocked:
        print("BLOCKED —— 读不到：%s（%s）⇒ rc=2 不给结论" % (f, err), file=sys.stderr)

    hit = False
    for r in results:
        d, g = r["duplicate_titles"], r["dangling_refs"]
        print("== %s" % r["file"])
        print("   标题共 %d 个 · ① 标题号重复 %d 处 · ② 件内引用缺失 %d 个"
              "（另一档：跨件/自由引用 %d 个 —— **只登记、不判红**）"
              % (r["headings"], len(d), len(g), r.get("cross_refs", 0)))
        if d:
            hit = True
            for x in d:
                print("   ① 号 `%s` 出现 %d 次 ⇒ 行 %s" % (x["num"], len(x["lines"]),
                                                        ", ".join(str(i) for i in x["lines"])))
        for x in g:
            hit = True
            print("   ② 引用 `§%s`（行 %d）本件无同名标题 ⇒ %s" % (x["num"], x["line"], x["text"]))

    if blocked:
        return 2
    print("⇒ %s" % ("两格有命中（rc=1）" if hit else "两格皆空（rc=0）"))
    return 1 if hit else 0


if __name__ == "__main__":
    sys.exit(main())

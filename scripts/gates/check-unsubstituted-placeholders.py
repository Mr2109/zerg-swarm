#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-unsubstituted-placeholders.py —— 判据「**未代入占位符 = 0**」（产物句面 · 块C §六④）

判据（逐字照令）
----------------
扫**产物句面**（`docs/**/*.md` + `publish/docs/*.md`），判据 = **出现形如 `{counts}` 的命名花括号占位
⇒ `rc=1` + 逐件逐行点名**；**两值（0/1）、不给「没结论」那档**（纯本地读 ⇒ 「判不了」的形态不存在）；
★ **不许**只做「两侧对拍」那一档。

最小检测句（逐字取自 `Zerg-内部文档/项目文档/v2.5.12/调研-块C-生成物同步门-20260924.md` §三.3）
----------------------------------------------------------------------------------
    PAT = re.compile(r"\\{[A-Za-z_][A-Za-z0-9_.]*\\}")
    for f in sorted(glob.glob("docs/**/*.md", recursive=True) + glob.glob("publish/docs/*.md")):
        for i, l in enumerate(open(f, ...), 1):
            if m := PAT.search(l): print(f"{f}:{i}: {m.group(0)}"); bad += 1
    sys.exit(1 if bad else 0)

为什么这一格必须自己立（病根已发生过一次）
------------------------------------------
`gen-cli-reference.py` 的 `danger_note` 曾漏一格 `.format(counts=…)` ⇒ **产物里留下原文 `{counts}`**，
而默认档 `--check` 判的是「**产物 ⟷ 现跑**」两侧 ⇒ **两侧一样 ⇒ 一模一样 ⇒ 假绿**（闭身 `f0196818`）。
⇒ 「两侧对拍」这一族判据**在结构上**发现不了「同一个洞出现在两侧」。gnu gettext 的逐字反例坐实它
（`msgfmt --check-format` 逐字：「both strings should have the same number of `%` format specifiers」
⇒ **两侧都缺就一句话不说**）⇒ 本判据**只扫产物自身的句面**，不拿任何第二份东西来对拍。

三处机检要点（逐条为什么）
--------------------------
  ① **扫「产物件」而不是「模板 + 代入值」** —— 模板侧判据抓不到「真的漏了一格 `.format()`」：那一格的
     形态就是**产物里留原文**；
  ② **`{` 后面必须是标识符**（`[A-Za-z_]`）—— 排除 JSON / 代码围栏里的花括号，**只报命名占位**，
     否则就是 `gettext --check-format` 那种**假红**；
  ③ **`rc` 两值（0/1）**，**不给「没结论」那一档** —— 本判据是**纯本地读**，真出现取不到的形态一律走 1
     （本件把「缺根 / 扫面 0 件」也按 **1** 记，并逐字写明「两值制 ⇒ 不给没结论档 ⇒ 按红记」）。

第三处排除（**本件现读加的口径 · 写在这里照实**）
------------------------------------------------
逐字的最小检测句在**今天的真树**上会给出 **86 处**命中 —— 逐条看，**全部**是 REST 路由模板
（形如 `/api/models/{name}` · `/api/logs/{kind}` · `GET /v1/context/{session_id}`）与代码区里的花括号，
**没有一条**是未代入的模板变量 ⇒ 那是**假红**（设计稿自己要求负控「改前绿」，见本件末「现读」）。
⇒ 本件在逐字判据上**只加两处排除**（都不放宽「命名占位」这一条本身）：
  · **代码区**（` ``` ` 围栏内 + 行内 `` ` `` 内）⇒ 那是代码/契约，不是**句面**；
  · **路由模板**（`{` **紧跟在 `/` 之后**）⇒ REST 路径参数是端点契约的一部分，不是待代入的模板变量。
**现读**：86 处 = 围栏内 12 + 行内代码 39 + 路由模板 35 ⇒ **剩余 0 处**（`rc=0`，与设计稿「改前绿」同向）。
★ 真病根那一条（漏 `.format()` 留下的 `{counts}`）落在 **`>` 引用注记行的正文**上 —— **不在**代码区、
**不在**路由位 ⇒ 照抓（负控见 `--self-test` 第 2 格）。

用法
----
  python3 scripts/gates/check-unsubstituted-placeholders.py            # 扫真目标（仓根现读上溯）
  python3 scripts/gates/check-unsubstituted-placeholders.py --root <dir>
  python3 scripts/gates/check-unsubstituted-placeholders.py --self-test # 合成夹具 · 不碰真目标
  python3 scripts/gates/check-unsubstituted-placeholders.py --list-rules

退出码：`0` 全过（0 处）· `1` 有残留（逐件逐行点名）**或**判不了（缺根 / 扫面 0 件 —— 两值制 ⇒ 按红记）。
本件**不写任何件**（只读）。
"""
import argparse
import glob
import os
import re
import shutil
import sys
import tempfile

VERSION = "check-unsubstituted-placeholders.py v1.0（`Q-166` / 块C §六④ · 2026-09-24 落件）"

PAT = re.compile(r"\{[A-Za-z_][A-Za-z0-9_.]*\}")     # ← 逐字照 调研-块C §三.3 最小检测句
FACE_GLOBS = ("docs/**/*.md", "publish/docs/*.md")   # ← 逐字照 块C §六④ 的判据面
MAX_PRINT = 200                                      # 打印上限（点名不省略件，只截条数并写明剩余）


def find_root(start):
    """上溯找仓根：同时有 `docs/` 与 `publish/` 的那一层（找不到 ⇒ None ⇒ 判不了）。"""
    d = os.path.abspath(start)
    while True:
        if os.path.isdir(os.path.join(d, "docs")) and os.path.isdir(os.path.join(d, "publish")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            return None
        d = parent


def scan_files(root):
    out, seen = [], set()
    for g in FACE_GLOBS:
        for p in sorted(glob.glob(os.path.join(root, g), recursive=True)):
            if p in seen or not os.path.isfile(p):
                continue
            seen.add(p)
            out.append(os.path.relpath(p, root).replace(os.sep, "/"))
    return sorted(out)


def inline_spans(line):
    """行内代码 span（`` ` `` 成对）—— 返回 [(start, end), …]（end 不含右反引号）。"""
    spans, i, n = [], 0, len(line)
    while i < n:
        if line[i] == "`":
            j = line.find("`", i + 1)
            if j < 0:
                break
            spans.append((i + 1, j))
            i = j + 1
        else:
            i += 1
    return spans


def hits_in_text(rel, text):
    """逐行判：略过围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）⇒ 只报命名占位。"""
    hits, infence = [], False
    for i, line in enumerate(text.split("\n"), 1):
        if line.lstrip().startswith("```"):
            infence = not infence
            continue
        if infence:
            continue
        spans = inline_spans(line)
        for m in PAT.finditer(line):
            s = m.start()
            if any(a <= s < b for a, b in spans):
                continue                                  # 行内代码区
            if s > 0 and line[s - 1] == "/":
                continue                                  # 路由模板（REST 路径参数）
            hits.append((rel, i, m.group(0), line.strip()))
    return hits


def judge(root):
    files = scan_files(root)
    if not files:
        print("   ⛔ 判不了：扫面 **0 件**（两条 glob 都没匹配到产物件 —— 空转不许当绿）")
        print("   ⇒ 本判据是**两值制（0/1）**、不给「没结论」档 ⇒ **按红记（rc=1）**")
        return 1, files, []
    hits, unreadable = [], []
    for rel in files:
        try:
            with open(os.path.join(root, rel), encoding="utf-8", errors="replace") as f:
                text = f.read()
        except OSError as exc:
            unreadable.append("%s（%s）" % (rel, exc))
            continue
        hits += hits_in_text(rel, text)
    if unreadable:
        for u in unreadable:
            print("   ⛔ 件读不了：%s" % u)
        print("   ⇒ 两值制 ⇒ **按红记（rc=1）**（读不全就判不出真值 · 不给「没结论」档）")
        return 1, files, hits
    return (1 if hits else 0), files, hits


def report(root, rc, files, hits, quiet=False):
    if not quiet:
        print("── 判据「未代入占位符 = 0」（产物句面）· 块C §六④ / `Q-166` ──")
        print("   判据树 / 仓根：%s" % root)
        print("   判据面（逐字照令）：%s = **%d 件**" % (" + ".join(FACE_GLOBS), len(files)))
        print("   检测形态（逐字照 §三.3 最小检测句）：%s" % PAT.pattern)
        print("   三处排除（见件头 · 只排「不是命名占位」的两类）：围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）")
    if hits:
        if not quiet:
            print("   ✗ 命中 **%d 处**（逐件逐行点名）：" % len(hits))
        for rel, i, tok, line in hits[:MAX_PRINT]:
            print("     %s:%d: %s   ← %s" % (rel, i, tok, line[:110]))
        if len(hits) > MAX_PRINT:
            print("     …另有 %d 处（打印上限 %d）" % (len(hits) - MAX_PRINT, MAX_PRINT))
        if not quiet:
            print("   补救：改生成器/模板里那一处（缺 `.format(...)` 就补上，或把该占位删掉）⇒ 重跑 `--emit`")
            print("⇒ **不通过（rc=1）**：产物里 %d 处未代入的命名花括号占位" % len(hits))
        return 1
    if not quiet:
        print("   ✓ 命中 **0 处**（本判据两值：0 / 1 ⇒ 不给「没结论」档）")
        print("⇒ **全过（rc=0）**：产物句面无未代入的命名花括号占位")
    return 0


# ── 自检（合成夹具 · 不碰真目标）─────────────────────────────────────────────
def self_test():
    t = tempfile.mkdtemp(prefix="cunsub-")
    n, bad = 0, 0

    def check(what, got, want):
        nonlocal n, bad
        n += 1
        ok = (got == want)
        bad += 0 if ok else 1
        print("   %s %s（实测 %s · 期望 %s）" % ("✓" if ok else "✗", what, got, want))

    def fixture(name, files):
        d = os.path.join(t, name)
        for rel, text in files.items():
            p = os.path.join(d, rel)
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "w", encoding="utf-8") as f:
                f.write(text)
        os.makedirs(os.path.join(d, "publish", "docs"), exist_ok=True)
        return d

    # 格 1 正控：干净产物 ⇒ 0
    d1 = fixture("clean", {"docs/zh/参考-命令行.md": "# 参考\n\n命令 `zerg help` 只读。\n"})
    check("格 1 正控（干净句面 ⇒ rc=0）", report(d1, *judge(d1), quiet=True), 0)
    # 格 2 **must-fail 负控**：句面留一处 `{counts}`（= 漏 `.format()` 的形态）⇒ 1 + 逐一例点名
    p2 = os.path.join(d1, "publish", "docs", "CLI.zh-CN.md")
    with open(p2, "w", encoding="utf-8") as f:
        f.write("> 本版共 {counts} 条已开放命令。\n")
    rc2, _f2, h2 = judge(d1)
    check("格 2 负控（句面 `{counts}` ⇒ rc=1）", rc2, 1)
    check("格 2 负控（点名到 件:行: 占位）",
          "%s:%d: %s" % (h2[0][0] if h2 else "—", h2[0][1] if h2 else 0, h2[0][2] if h2 else "—"),
          "publish/docs/CLI.zh-CN.md:1: {counts}")
    # 格 2b 成对（区分度）：同一夹具删掉那一处 ⇒ 0（「改了 ⇒ 1 / 删了 ⇒ 0」）
    with open(p2, "w", encoding="utf-8") as f:
        f.write("> 本版共 118 条已开放命令。\n")
    check("格 2b 成对（同一夹具删掉占位 ⇒ rc=0）", judge(d1)[0], 0)
    # 格 3 假红对照：路由模板 `/api/models/{name}`（真树那 35 处那一类）⇒ 0
    d3 = fixture("route", {"docs/zh/模块-主控.md": "- **舰队**: `GET /api/models/{name}`（单枚模型）\n"})
    check("格 3 假红对照（行内代码里的路由模板 ⇒ rc=0）", judge(d3)[0], 0)
    with open(os.path.join(d3, "docs", "zh", "模块-主控.md"), "a", encoding="utf-8") as f:
        f.write("- 裸路由模板（无行内代码）：GET /api/logs/{kind} 读日志\n")
    check("格 3b 假红对照（裸路由模板 `{` 紧跟 `/` ⇒ rc=0）", judge(d3)[0], 0)
    # 格 4 假红对照：代码区（围栏 + 行内）里的 `{counts}` ⇒ 0（要点②：只报命名占位，不报代码）
    d4 = fixture("code", {"docs/zh/x.md":
                          "示例：\n\n```text\n{counts} 与 {\"counts\": 3}\n```\n\n行内代码里的 `{counts}` 不算。\n"})
    check("格 4 假红对照（围栏内 / 行内代码里的 `{counts}` ⇒ rc=0）", judge(d4)[0], 0)
    with open(os.path.join(d4, "docs", "zh", "x.md"), "a", encoding="utf-8") as f:
        f.write("句面漏代：本版共 {counts} 条。\n")
    check("格 4b 成对（同一件句面加一处 ⇒ rc=1 · 代码区那条仍然不算）", judge(d4)[0], 1)
    # 格 5 空转负控：扫面 0 件 ⇒ **1**（两值制 ⇒ 不给「没结论」档）
    d5 = fixture("empty", {"docs/zh/空.md": ""})
    os.remove(os.path.join(d5, "docs", "zh", "空.md"))
    check("格 5 空转负控（扫面 0 件 ⇒ rc=1 · 两值制不给「没结论」档）", judge(d5)[0], 1)
    # 格 6 缺根 ⇒ 1（同上口径）
    check("格 6 缺根负控（找不到仓根 ⇒ rc=1）", judge(os.path.join(t, "no-such"))[0], 1)
    # 格 7 读不了件 ⇒ 1（用 `chmod 000` 造读不了的 .md 件）
    d7 = fixture("unreadable", {"docs/zh/好.md": "干净\n", "docs/zh/坏.md": "读不了\n"})
    p7 = os.path.join(d7, "docs", "zh", "坏.md")
    os.chmod(p7, 0)
    check("格 7 读不了负控（件读不了 ⇒ rc=1）", judge(d7)[0], 1)
    os.chmod(p7, 0o644)
    # 格 8 真树现跑：只证「判得动」（照实打印现值 · 不要求 0）
    root = find_root(os.getcwd())
    if root:
        rc8 = report(root, *judge(root), quiet=True)
        print("   ✓ 格 8 真树现跑：rc=%d（照实：本格只证「判得动」，不写死期望值）" % rc8)
    else:
        print("   · 格 8 真树现跑：跳（cwd 上溯找不到仓根）")
    shutil.rmtree(t, ignore_errors=True)
    print("   ⇒ 自检%s（断言 %d 格 · 失败 %d 格）" % ("全过" if not bad else "不过", n, bad))
    return 1 if bad else 0


def main():
    ap = argparse.ArgumentParser(add_help=True, description=VERSION)
    ap.add_argument("--root", default=None, help="判据树 / 仓根（默认：cwd 上溯找同时有 docs/ 与 publish/ 的那一层）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    ap.add_argument("--list-rules", action="store_true", help="打印判据 / 判据面 / 排除 / 退码")
    a = ap.parse_args()
    if a.list_rules:
        print(VERSION)
        print("  判据面（逐字照令）：%s" % " + ".join(FACE_GLOBS))
        print("  检测形态（逐字照 §三.3 最小检测句）：%s" % PAT.pattern)
        print("  排除（只排「不是命名占位」的两类）：围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）")
        print("  退码：0 = 0 处 · 1 = 有残留 或 判不了（两值制 · 不给「没结论」档）")
        print("  不许：只做「两侧对拍」（gettext 反例：两侧同洞 ⇒ 照样假绿）")
        return 0
    if a.self_test:
        print("── 自检（合成夹具 · 不碰真目标）──")
        return self_test()
    root = os.path.abspath(a.root) if a.root else find_root(os.getcwd())
    if not root or not os.path.isdir(root):
        print("⛔ 判不了：找不到仓根（同时有 `docs/` 与 `publish/` 的那一层）⇒ 两值制 ⇒ 按红记（rc=1）")
        return 1
    rc, files, hits = judge(root)
    return report(root, rc, files, hits)


if __name__ == "__main__":
    sys.exit(main())

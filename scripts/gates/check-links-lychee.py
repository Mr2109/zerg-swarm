#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-links-lychee.py v1.0.0 —— 「**本地断链 · lychee 壳**」判据件（组4 §一 `W-05` · `拍-C:49` /
`拍-清:147` 的 `R47` 照做条 · 2026-09-24 波5 序90）。

拍定（逐字 · 组4 清单 `W-05` 行）：「断链件**换 `lychee --offline`**（自写脚本**降为对拍件**）·
**不引** `markdown-link-check` · 必须补「**空转不许给绿**」」。

本件与自写件的关系（**不是二选一，是分工**）
------------------------------------------
  · 本件 = **断链件**（判据面 = `lychee --offline` 现跑）：本地断链**由 lychee 判**，我们只做
    ① 域与件清单 ② 退码映射 ③ 逐条点名 ④ 空转/缺件**不给结论** 四件事。
  · `scripts/gates/check-docs-orphans-links.py` = **对拍件**（仍挂在自己那一步上 · 两件各自跑）：
    它那两格（`local_broken` 本地断链 / `orphans` 孤儿）里「本地断链」这一格与本件**同域** ⇒ 两件
    读数不同即**分叉**，同一提交里并排看得见（这就是「降为对拍件」的落地形态 —— **不删、不摘挂**）。
  · 为什么换：自写正则挑本地链接**天生看不到正则有洞的那一半**（`%20` 编码 · 反斜杠 · 大小写 ·
    锚点后带 query …），而 `lychee --offline` 用的是**真解析器**（Markdown/HTML）⇒ 「差不多对」的
    正则兜底不再承担判据。

为什么必须是 `--offline`（**外链明确不进**的机制，不是我们手写的排除表）
----------------------------------------------------------------------
`lychee --offline` 把**除本地路径以外**的链接（`http(s)://` · `mailto:` 等）一律标 `Excluded`
**不检查** —— 现跑实测（本件自检⑷）：一段里只放外链 ⇒ `errors 0` ⇒ 本件退 0。⇒ 「外链不进」是
**工具的固有语义**，不是我们在脚本里维护一张越来越长的排除名单（名单会漏，语义不会）。

退码（三档 · 与仓内既有门同惯例；**「读不到」不许当绿**）
--------------------------------------------------------
  0 = 判过：`lychee --offline` 报 **errors 0**（无本地断链）
  1 = 判红：报 **errors > 0** —— 逐条点名 `件:行:列 → 目标`（本步起步档 `tri-report` ⇒ 只报告）
  2 = **不给结论**（一律显式打印，不假绿）：
      · `lychee` 不在盘（`shutil.which` 找不到）· 清单**0 件**（**空转不许给绿**）·
      · `git ls-files` 跑不了（不是 git 仓 / 仓根缺）· `lychee` 自身退码不在 {0, 2}（意外错）·
      · `--format json` 产物解析不了（形状变了 ⇒ 不猜）· `--self-test` 任一格不过（**自检不过拒绝扫真目标**）

用法
----
    python3 scripts/gates/check-links-lychee.py                     # 扫仓根 git 跟踪的 *.md
    python3 scripts/gates/check-links-lychee.py --devdocs <旁树根>   # 两棵树一起扫（域同自写件）
    python3 scripts/gates/check-links-lychee.py --files <清单文件>   # 显式清单（一行一件 · 自检/取证用）
    python3 scripts/gates/check-links-lychee.py --self-test         # 成对负控（不碰真目标）
    python3 scripts/gates/check-links-lychee.py --root <仓根>
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates/ → 仓根

LYCHEE = "lychee"
# `--offline`：只判本地；外链一律 Excluded（**不检查**）。
# `--no-progress`：不留进度条字符（机器读面）。
# `--format json`：唯一机器读面（本件只认它 —— 文本面随版本漂，形状写死在 _parse 里、读不出就退 2）。
LYCHEE_ARGS = ["--offline", "--no-progress", "--format", "json", "--files-from"]


# ── 件清单（域）────────────────────────────────────────────────────────────────────────────────
def git_md_files(root):
    """仓根里 **git 跟踪**的 `*.md` 清单。

    ★ 口径两条（都是实测踩出来的，写死在这里）：
      ① `-c core.quotePath=false`：**必须**。`git ls-files` 默认把非 ASCII 名输出成
         `"core\\350\\256\\276..."` 这种带双引号的转义形态 ⇒ 喂给 lychee 现跑逐字报
         `Error: Cannot parse inputs from arguments … not found as file and not a valid URL`（rc=1）。
         同一个坑在账内 `W-31` 已记过一次（`core.quotePath=true` 让 `f.startswith()` 永远配不上）。
      ② 只收 `git ls-files` 的件：**未跟踪的新件不进**（域写死、不静默扩面）—— 要扫未跟踪件请用
         `--files` 显式给清单（那时域由调用者负责）。
    """
    cmd = ["git", "-c", "core.quotePath=false", "ls-files", "*.md"]
    try:
        p = subprocess.run(cmd, cwd=root, capture_output=True, text=True, stdin=subprocess.DEVNULL)
    except OSError as e:
        return None, "git 跑不了（%s）" % e
    if p.returncode != 0:
        return None, "git ls-files 退 %d（不是 git 仓 / 仓根不对）：%s" % (p.returncode, p.stderr.strip()[:200])
    out = [ln.strip() for ln in p.stdout.splitlines() if ln.strip()]
    return out, ""


def build_file_list(root, devdocs, files_from):
    """扫面 = 仓根（git 跟踪的 *.md）[+ 旁树（同样只收 git 跟踪件）]；`--files` 给定 ⇒ 用它逐字。"""
    if files_from:
        try:
            with open(files_from, encoding="utf-8") as f:
                items = [ln.strip() for ln in f if ln.strip() and not ln.startswith("#")]
        except OSError as e:
            return None, "清单读不了（%s）：%s" % (files_from, e)
        return [os.path.abspath(os.path.join(root, i)) if not os.path.isabs(i) else i for i in items], ""

    files, err = git_md_files(root)
    if err:
        return None, err
    absfiles = [os.path.join(root, f) for f in files]
    if devdocs:
        dd = os.path.abspath(devdocs)
        if not os.path.isdir(dd):
            return None, "旁树不是目录（%s）" % dd
        more, err = git_md_files(dd)
        if err:
            return None, "旁树 %s：%s" % (dd, err)
        absfiles += [os.path.join(dd, f) for f in more]
    return absfiles, ""


# ── lychee 调用 + 产物解析（形状读不出 ⇒ 不给结论）────────────────────────────────────────────────
def run_lychee(files):
    if shutil.which(LYCHEE) is None:
        return None, None, "%s 不在盘（`which lychee` 找不到）⇒ 缺判据 ⇒ 不给结论" % LYCHEE
    with tempfile.NamedTemporaryFile("w", encoding="utf-8", suffix=".txt", delete=False) as tf:
        tf.write("\n".join(files) + "\n")
        listpath = tf.name
    try:
        p = subprocess.run([LYCHEE] + LYCHEE_ARGS + [listpath], capture_output=True, text=True,
                           stdin=subprocess.DEVNULL)
    except OSError as e:
        return None, None, "%s 跑不了（%s）" % (LYCHEE, e)
    finally:
        try:
            os.unlink(listpath)
        except OSError:
            pass
    if p.returncode not in (0, 2):  # 0 = 无错 · 2 = 有错（lychee 自家口径）；其余 = 意外错
        return None, p.returncode, "%s 退 %d（不在 {0, 2} ⇒ 意外错，不是判据结果）：%s" % (
            LYCHEE, p.returncode, (p.stderr or "").strip()[:200])
    try:
        data = json.loads(p.stdout)
    except (ValueError, TypeError) as e:
        return None, p.returncode, "%s 的 `--format json` 产物解析不了（形状变了 ⇒ 不猜）：%s" % (LYCHEE, e)
    for key in ("errors", "error_map"):
        if key not in data:
            return None, p.returncode, "json 缺 `%s` 键（形状变了 ⇒ 不猜）" % key
    return data, p.returncode, ""


def broken_lines(data, root):
    """`error_map` → 逐条 `件:行:列 → 目标`（顺序 = 件名 + 行号，稳定输出，便于对拍）。"""
    rows = []
    for fname, errs in data.get("error_map", {}).items():
        rel = fname
        try:
            if os.path.isabs(fname):
                rel = os.path.relpath(fname, root)
        except ValueError:
            rel = fname
        for e in errs or []:
            span = e.get("span") or {}
            url = e.get("url", "")
            if url.startswith("file://"):
                url = url[len("file://"):]
                try:
                    import urllib.parse
                    url = urllib.parse.unquote(url)
                except Exception:
                    pass
            rows.append("%s:%s:%s → %s" % (rel, span.get("line", "?"), span.get("column", "?"), url))
    return sorted(rows)


# ── 主判据 ────────────────────────────────────────────────────────────────────────────────────
def judge(root, devdocs, files_from, quiet=False):
    """返回 (rc, 行清单)。**唯一一处**判定逻辑：正控跑真目标、自检跑夹具，都走这里。"""
    lines = []
    files, err = build_file_list(root, devdocs, files_from)
    if err:
        return 2, ["⛔ 不给结论：%s" % err]
    if not files:  # ★ 空转不许给绿（拍定逐字要求的那一条）
        return 2, ["⛔ 不给结论：扫面 **0 件**（空转不许给绿）—— 域 = 仓根 git 跟踪的 `*.md`"
                   "（+ `--devdocs` 旁树）；`--files` 给了空清单同判"]
    data, lychee_rc, err = run_lychee(files)
    if err:
        return 2, ["⛔ 不给结论：%s" % err]
    n_err = int(data.get("errors", 0))
    if not quiet:
        lines.append("扫面 %d 件 · lychee 退 %d · links %s（unique %s · successful %s · excludes %s）"
                     % (len(files), lychee_rc, data.get("total"), data.get("unique"),
                        data.get("successful"), data.get("excludes")))
        lines.append("`--offline` ⇒ 外链一律 `Excluded`（**不检查**）⇒ 「外链不进」是工具语义，不是本件的排除名单")
    if n_err == 0:
        return 0, lines + ["⇒ 无本地断链（errors 0）· rc=0"]
    rows = broken_lines(data, root)
    lines.append("⇒ 本地断链 **%d 处**（errors %d）⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）" % (len(rows), n_err))
    lines += ["   " + r for r in rows[:40]]
    if len(rows) > 40:
        lines.append("   …（另有 %d 处，见 lychee json 产物）" % (len(rows) - 40))
    return 1, lines


# ── 自检（成对负控 · 不碰真目标）────────────────────────────────────────────────────────────────
def self_test():
    import tempfile as _tf
    bad = []
    cases = []

    def case(name, want, got):
        cases.append(name)
        if want != got:
            bad.append("%s ⇒ 实得 rc=%s（要 %s）" % (name, got, want))

    with _tf.TemporaryDirectory() as t:
        # ⑴ 正控：本地链接指到存在的件 ⇒ 0
        os.makedirs(os.path.join(t, "good"), exist_ok=True)
        open(os.path.join(t, "good", "a.md"), "w", encoding="utf-8").write("# t\n\n[ok](./b.md)\n")
        open(os.path.join(t, "good", "b.md"), "w", encoding="utf-8").write("# b\n")
        with open(os.path.join(t, "l1.txt"), "w", encoding="utf-8") as f:
            f.write(os.path.join(t, "good", "a.md") + "\n")
        rc, _ = judge(t, None, os.path.join(t, "l1.txt"), quiet=True)
        case("⑴ 正控（本地链接都在）⇒ 0", 0, rc)

        # ⑵ 负控：本地链接指向不存在的件 ⇒ 1（探针没牙即红）
        open(os.path.join(t, "good", "c.md"), "w", encoding="utf-8").write("# c\n\n[bad](./nope.md)\n")
        with open(os.path.join(t, "l2.txt"), "w", encoding="utf-8") as f:
            f.write(os.path.join(t, "good", "c.md") + "\n")
        rc, out = judge(t, None, os.path.join(t, "l2.txt"), quiet=True)
        case("⑵ 负控（本地断链）⇒ 1", 1, rc)
        if not any("nope.md" in ln for ln in out):
            bad.append("⑵ 负控：点名那一条没印出目标件名（逐条点名是本件的一半）")

        # ⑶ 负控：扫面 0 件 ⇒ 2（★ 空转不许给绿 —— 拍定逐字要求的那一条）
        with open(os.path.join(t, "l3.txt"), "w", encoding="utf-8") as f:
            f.write("# 注释行不算件\n\n")
        rc, _ = judge(t, None, os.path.join(t, "l3.txt"), quiet=True)
        case("⑶ 负控（空转 0 件）⇒ 2", 2, rc)

        # ⑷ 正控：只放外链 ⇒ 0（证明「外链明确不进」是 `--offline` 的固有语义）
        open(os.path.join(t, "good", "d.md"), "w", encoding="utf-8").write(
            "# d\n\n[ext](https://example.invalid/x)\n")
        with open(os.path.join(t, "l4.txt"), "w", encoding="utf-8") as f:
            f.write(os.path.join(t, "good", "d.md") + "\n")
        rc, _ = judge(t, None, os.path.join(t, "l4.txt"), quiet=True)
        case("⑷ 正控（只有外链）⇒ 0", 0, rc)

        # ⑸ 判不了：lychee 不在盘 ⇒ 2（缺判据 ≠ 绿）
        oldpath = os.environ.get("PATH", "")
        os.environ["PATH"] = ""
        try:
            rc, _ = judge(t, None, os.path.join(t, "l1.txt"), quiet=True)
        finally:
            os.environ["PATH"] = oldpath
        case("⑸ 负控（lychee 不在盘）⇒ 2", 2, rc)

    if bad:
        print("✗ 自检不过（%d 格）—— **拒绝扫真目标**：" % len(bad))
        for b in bad:
            print("   · " + b)
        return 2
    print("✓ 自检全过（%d 格：正控 2 + 负控 3 · 成对）" % len(cases))
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--devdocs", default=None, help="旁树根（给了就并进扫面：域 = 两棵树）")
    ap.add_argument("--files", default=None, help="显式件清单（一行一件 · 覆盖 git 扫面）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（成对负控 · 不碰真目标）")
    a = ap.parse_args()

    if a.self_test:
        return self_test()

    if not os.path.isdir(a.root):
        print("⛔ 不给结论：仓根不是目录（%s）" % a.root)
        return 2
    rc, lines = judge(a.root, a.devdocs, a.files)
    for ln in lines:
        print(ln)
    return rc


if __name__ == "__main__":
    sys.exit(main())

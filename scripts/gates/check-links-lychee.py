#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-links-lychee.py v1.0.0 —— 「**本地断链 · lychee 壳**」判据件（组4 §一 `W-05` · `拍-C:49` /
`拍-清:147` 的 `R47` 照做条 · 2026-09-24 波5 序90）。

拍定（逐字 · 组4 清单 `W-05` 行）：「断链件**换 `lychee --offline`**（自写脚本**降为对拍件**）·
**不引** `markdown-link-check` · 必须补「**空转不许给绿**」」。

本件与自写件的关系（**不是二选一，是分工**）
------------------------------------------
  · 本件 = **断链件**（判据面 = `lychee --offline` 现跑）：本地断链**由 lychee 判**，我们只做
    ① 域与件清单 ② 退码映射 ③ 逐条点名 ④ 空转/缺件**不给结论** 四件事，外加
    ⑤ **基准偏移复核**（`BASE_OFFSET`：件在 A 处、链接按 B 基准写的**收窄判据** —— 见下节）。
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
  0 = 判过：`lychee --offline` 报 **errors 0**，**或** errors > 0 但**每一条**都被「基准偏移登记」
      复核命中（`BASE_OFFSET` ⇒ 假红 ⇒ 逐条印出落点、不计入断链数）
  1 = **本门判红**：**复核后**仍有 errors > 0 —— 逐条点名 `件:行:列 → 目标`（**本门退码**
      —— 本步**接线档位** `tri-report` ⇒ 该 rc=1 落 REPORT：只报告、不计失败项、不影响退出码）
  2 = **不给结论**（一律显式打印，不假绿）：
      · `lychee` 不在盘（`shutil.which` 找不到）· 清单**0 件**（**空转不许给绿**）·
      · `git ls-files` 跑不了（不是 git 仓 / 仓根缺）· `lychee` 自身退码不在 {0, 2}（意外错）·
      · `--format json` 产物解析不了（形状变了 ⇒ 不猜）· `--self-test` 任一格不过（**自检不过拒绝扫真目标**）
      · **基准偏移登记过期**（`BASE_OFFSET` 的体检不过 ⇒ 逐字点名，见该表上方注释）

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


# ── 基准偏移登记（「件在 A 处、链接按 B 基准写」—— **收窄判据**，不是豁免名单）─────────────────────
#   ★ 为什么（现读证据 · 2026-09-26 重测本件）：`publish/mirror-public-lib.py:526-530` 的 `map_path`
#     把 `publish/docs/README.md` / `README.zh-CN.md` 映到**公开树仓根**（该行逐字注释：「README 家族
#     放仓库根（语言切换互链）」），而本件按 `git ls-files` 的**原位**（`publish/docs/`）解析相对链接
#     ⇒ 这两个件里每一条 `docs/...` / 根文件链接都被解析成 `publish/docs/docs/...` · `publish/docs/LICENSE`
#     （指不到）⇒ 现跑 30 处**全假红**。复核（按登记基准重解析 + 现读存在性）：
#       `publish/docs/README.md` 15/15 命中 · `README.zh-CN.md` 15/15 命中；**原位 0/30 命中**
#       ⇒ 30 处里 **真断链 0 条**。病根同 `check-doc-freshness` 那族：**解析基准少了一条**。
#   ★ 治法（收窄判据）：命中登记 ⇒ **按登记基准重解析 + 用现读存在性复核**：
#       复核命中 ⇒ 记「已登记假红(基准偏移 · 已复核)」· **不计入断链数**（逐条印出落点，看得见）；
#       复核落空 ⇒ **照旧判红** —— 豁免只挡「基准错」这一半，真断链一条也不放。
#     本件**只可能把红改小**，且每一次豁免都由一次**现读存在性**背书（不是名单信任）。
#   ★ 登记体检（本改动的牙 · 与 `check-zh-en.py` 的 `registry_stale` 同精神：登记失效**必须出声**）：
#     登记**过期** ⇒ rc=2 不给结论 · **逐字点名**，两种（**只对「登记件在扫面域内」的条目判** ——
#     `--files` 取证式单件扫描 / 夹具树里登记不参与本跑，不许借此把登记判成过期）：
#       ① 登记基准不是目录（`<root>/<基准>` 找不到 ⇒ 基准指空）
#       ② 登记件**在扫面域内却一条原位错都没有**（链接已改回原位可达 / 该件不再需要偏移 —— 该撤登记）
#     件被改名/移走 ⇒ 条目落到域外 ⇒ 不判过期（**也无害**：那时它不再产出任何假红；要撤登记走审计）
BASE_OFFSET = {
    "publish/docs/README.md":       ("publish", "公开树仓根定位"),
    "publish/docs/README.zh-CN.md": ("publish", "公开树仓根定位（与 en 版同基准 · 语言切换互链）"),
}


# ── 件清单（域）────────────────────────────────────────────────────────────────────────────────
def git_md_files(root):
    """仓根里 **git 跟踪**的 `*.md` 清单。

    ★ 口径两条（都是实测踩出来的，写死在这里）：
      ① `-c core.quotepath=false`：**必须**。`git ls-files` 默认把非 ASCII 名输出成
         `"core\\350\\256\\276..."` 这种带双引号的转义形态 ⇒ 喂给 lychee 现跑逐字报
         `Error: Cannot parse inputs from arguments … not found as file and not a valid URL`（rc=1）。
         同一个坑在账内 `W-31` 已记过一次（`core.quotePath=true` 让 `f.startswith()` 永远配不上）。
      ② 只收 `git ls-files` 的件：**未跟踪的新件不进**（域写死、不静默扩面）—— 要扫未跟踪件请用
         `--files` 显式给清单（那时域由调用者负责）。
    """
    cmd = ["git", "-c", "core.quotepath=false", "ls-files", "*.md"]
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


def rel_in_root(path, root):
    """仓根相对路径（拿不到就原样返回 —— 绝不因格式而抛）。"""
    try:
        return os.path.relpath(path, root)
    except ValueError:
        return path


def broken_records(data, root):
    """`error_map` → 逐条 `(件绝对路径, 行, 列, 目标(去 `file://` 并解码))`（稳定序 = 件名 + 行号）。"""
    rows = []
    for fname, errs in data.get("error_map", {}).items():
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
            rows.append((fname, span.get("line", "?"), span.get("column", "?"), url))
    return sorted(rows, key=lambda r: (rel_in_root(r[0], root), str(r[1]), str(r[2]), r[3]))


def fmt_rec(rec, root):
    """逐条点名形态 `件:行:列 → 目标`（件名按仓根相对印）。"""
    return "%s:%s:%s → %s" % (rel_in_root(rec[0], root), rec[1], rec[2], rec[3])


def offset_ok(src_abs, url_abs, root):
    """命中「基准偏移登记」的错 ⇒ **按登记基准重解析 + 现读存在性复核**。

    返回复核理由（复核命中 ⇒ 记已登记假红）；`None` = 不豁免（未登记 / 复核落空 ⇒ 照旧判红）。
    """
    ent = BASE_OFFSET.get(rel_in_root(src_abs, root))
    if not ent:
        return None
    base, why = ent
    try:
        raw = os.path.relpath(url_abs, os.path.dirname(src_abs))
    except ValueError:
        return None
    cand = os.path.normpath(os.path.join(root, base, raw.split("#", 1)[0]))
    if os.path.exists(cand):
        return "%s（登记基准 `%s`）⇒ 复核命中 `%s`" % (why, base, rel_in_root(cand, root))
    return None


def offset_registry_check(root, files, recs):
    """**登记体检**（本改动的牙）：登记过期 ⇒ 逐字点名。

    过期两态见 `BASE_OFFSET` 上方注释（基准不是目录 / 件在扫面域内却 0 命中）。
    **只对「登记件在扫面域内」的条目判**：`--files` 取证式单件扫描 / 夹具树里登记不参与本跑。
    """
    stale = []
    dom = set(os.path.abspath(f) for f in files)
    hit_files = set(rel_in_root(r[0], root) for r in recs)
    for rel, ent in sorted(BASE_OFFSET.items()):
        base = ent[0]
        p = os.path.abspath(os.path.join(root, rel))
        if p not in dom:
            continue   # 域外（`--files` 取证式单件扫描 / 夹具树）⇒ 登记不参与本跑，**不判过期**
        if not os.path.isdir(os.path.join(root, base)):
            stale.append("%s ⇒ **登记基准 `%s` 不是目录**（基准指空 ⇒ 撤或改登记）" % (rel, base))
            continue
        if rel not in hit_files:
            stale.append("%s ⇒ **在扫面域内却一条原位错都没有**（链接已改回原位可达 / 该件不再需要偏移 ⇒ 请撤登记）" % rel)
    return stale


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
    recs = broken_records(data, root)
    stale = offset_registry_check(root, files, recs)
    if stale:  # ★ 登记失效必须出声（登记表体检 · 牙）
        return 2, lines + ["⛔ 不给结论：**基准偏移登记过期 %d 条** —— 登记指不到现实（逐字点名）："
                           % len(stale)] + ["   · " + s for s in stale]
    exempt, red = [], []
    for rec in recs:
        why = offset_ok(rec[0], rec[3], root)
        (exempt if why else red).append((rec, why))
    if red:
        lines.append("⇒ 本地断链 **%d 处**（errors %d；另有 **%d 处**为登记假红（基准偏移 · 已逐条复核）⇒ 不计）"
                     "⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）" % (len(red), n_err, len(exempt)))
        shown = red[:40]
        lines += ["   " + fmt_rec(r, root) for r, _ in shown]
        if len(red) > 40:
            lines.append("   …（另有 %d 处，见 lychee json 产物）" % (len(red) - 40))
        lines += ["   ⊕ 已登记假红（基准偏移 · 已复核 · 不计入上面 %d 处）：" % len(red)]
        lines += ["      %s  ⟸ %s" % (fmt_rec(r, root), why) for r, why in exempt]
        return 1, lines
    if exempt:  # 全绿但豁免非空 ⇒ 逐条印出落点（看得见，不是静默放行）
        lines += ["   ⊕ 已登记假红（基准偏移 · 已逐条复核 · 不计入断链数）："]
        lines += ["      %s  ⟸ %s" % (fmt_rec(r, root), why) for r, why in exempt]
    lines.append("⇒ 无本地断链（errors %d%s）· rc=0"
                 % (n_err, "；%d 处为登记假红（基准偏移 · 已复核）" % len(exempt) if exempt else ""))
    return 0, lines


# ── 自检（成对负控 · 不碰真目标）────────────────────────────────────────────────────────────────
def self_test():
    import tempfile as _tf
    bad = []
    cases = []
    kinds = []

    def case(name, want, got, kind="负控"):
        cases.append(name)
        kinds.append(kind)
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
        case("⑴ 正控（本地链接都在）⇒ 0", 0, rc, "正控")

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
        case("⑷ 正控（只有外链）⇒ 0", 0, rc, "正控")

        # ⑸ 判不了：lychee 不在盘 ⇒ 2（缺判据 ≠ 绿）
        oldpath = os.environ.get("PATH", "")
        os.environ["PATH"] = ""
        try:
            rc, _ = judge(t, None, os.path.join(t, "l1.txt"), quiet=True)
        finally:
            os.environ["PATH"] = oldpath
        case("⑸ 负控（lychee 不在盘）⇒ 2", 2, rc)

        # ⑹ 正控（登记豁免口）：夹具树里**逐字复刻真目标形状** `publish/docs/README.md`
        #    —— 它的链接按**公开树仓根**（`publish/`）写 ⇒ 原位指不到、按登记基准复核得到 ⇒ 0。
        #    同一件里再放 ⑴ 一条**反引号里的路径** 与 ⑵ 一条**引文/正文里提到的裸路径**（不是链接）
        #    ⇒ 必须**不报**（「转述/引文不是引用」那族的负控，折叠进本格）。
        os.makedirs(os.path.join(t, "publish", "docs"), exist_ok=True)
        open(os.path.join(t, "publish", "docs", "QUICKSTART.en.md"), "w", encoding="utf-8").write("# q\n")
        open(os.path.join(t, "publish", "docs", "README.md"), "w", encoding="utf-8").write(
            "# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n`docs/NEVER-A-FILE.md` 是反引号里的路径。\n\n"
            "> 引文：详见 docs/ALSO-NEVER.md 一节。\n")
        with open(os.path.join(t, "l5.txt"), "w", encoding="utf-8") as f:
            f.write(os.path.join(t, "publish", "docs", "README.md") + "\n")
        rc, out = judge(t, None, os.path.join(t, "l5.txt"), quiet=True)
        case("⑹ 正控（登记件 · 按基准复核命中）⇒ 0", 0, rc, "正控")
        if any("NEVER" in ln for ln in out):
            bad.append("⑹ 正控：反引号/引文里的路径被当链接报了（本件不许越界到非链接文本）")

        # ⑺ 负控（豁免口的牙）：同一登记件里塞一条**按基准也指不到**的链接 ⇒ 必须照旧判红且点名
        with open(os.path.join(t, "publish", "docs", "README.md"), "w", encoding="utf-8") as f:
            f.write("# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n也见 [x](docs/NOPE.md)。\n")
        rc, out = judge(t, None, os.path.join(t, "l5.txt"), quiet=True)
        case("⑺ 负控（登记件 · 基准复核落空）⇒ 1", 1, rc)
        if not any("NOPE.md" in ln for ln in out):
            bad.append("⑺ 负控：基准复核落空那一条没被点名（豁免口不许掩真断链）")

        # ⑻ 负控（登记表的牙 · 登记过期）：另一株夹具树里，登记件**原位就可达** ⇒ 一条错都没有
        #    ⇒ 登记已无对象 ⇒ 必须判「登记过期」rc=2 并点名（旧式样里登记失效是静默的）
        with _tf.TemporaryDirectory() as t2:
            os.makedirs(os.path.join(t2, "publish", "docs"), exist_ok=True)
            open(os.path.join(t2, "publish", "docs", "QUICKSTART.en.md"), "w", encoding="utf-8").write("# q\n")
            open(os.path.join(t2, "publish", "docs", "README.md"), "w", encoding="utf-8").write(
                "# r\n\n见 [q](QUICKSTART.en.md)。\n")
            with open(os.path.join(t2, "l6.txt"), "w", encoding="utf-8") as f:
                f.write(os.path.join(t2, "publish", "docs", "README.md") + "\n")
            rc, out = judge(t2, None, os.path.join(t2, "l6.txt"), quiet=True)
            case("⑻ 负控（登记过期：登记件在域内 0 命中）⇒ 2", 2, rc)
            if not any("登记过期" in ln or "请撤登记" in ln for ln in out):
                bad.append("⑻ 负控：登记过期没被逐字点名（登记表的牙不见了）")

    if bad:
        print("✗ 自检不过（%d 格）—— **拒绝扫真目标**：" % len(bad))
        for b in bad:
            print("   · " + b)
        return 2
    print("✓ 自检全过（%d 格：正控 %d + 负控 %d · 成对）"
          % (len(cases), kinds.count("正控"), kinds.count("负控")))
    return 0


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
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

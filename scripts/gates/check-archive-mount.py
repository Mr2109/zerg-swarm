#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-archive-mount.py —— 门㉛：归档区**挂载档**判据（`A8` 的「挂载档不是 chmod」那一半 + 核心一格）。

真源（逐字）
------------
  · `Zerg-内部文档/项目文档/v2.5.11/设计-归档区挂载档-A8-v1.1-20260923.md`
      §三 3.2（甲案 = 只读 dmg 挂 `/Volumes/ZERG-ARCHIVE` + 原路径换**同名符号链接**）
      §三 3.3 ★「**未挂 ⇒ 必须「明确失败」，不许「静默返回空」**」（本门的**核心判据**）
      §四 判据 1 / 2 / 3 / 4 / 9 · §五 负控 ① ④ ⑤ ⑥ ⑦ ⑧（逐条对照见下表）
  · 已拍口径（Mr2109 2026-09-23）：「上**挂载档**（**不是 `chmod`** ✗）· 持久化 = **按需加载**（不常驻）」。

本门为什么存在（要防的两个假绿）
--------------------------------
  ① **未挂时的「静默返回空」**：`archive-mount.sh off` 之后若把**挂载点那个空目录**留着，原路径
     （符号链接 → 挂载点）就会解析到一个**空目录** ⇒ `ls 原路径/` **会成功返回空** —— 读的人会把它
     读成「归档是空的」（而不是「档没挂上」）。**未挂必须明确失败**：`No such file or directory`。
  ② **挂了但读到空 / 读到错件**：档挂上了也可能挂错卷、卷是空的、或被换成一个可写的同路径目录
     —— 那时「挂了」是真的、「内容是归档」是假的。⇒ 本门把「挂载行含 `read-only`」「件数 ≥ 基线」
     「样本件读得到」「两处读数（`mount` 行 ⟷ `hdiutil info`）一致」四件一起判。

判据（A 档 = 阻断档判词 · 全部可机检）
------------------------------------
  A1 **挂载态现读**：按**挂载点路径**取那一行（`mount` 输出里含 `" on <挂载点> ("` 的那行 —— 不按卷名：
     现读反证见设计稿 §三 3.5），该行的选项表必须含 `read-only`。
  A2 **原路径那一格**：原路径必须是**符号链接**且 `readlink` == 挂载点（防「有人把实体目录放回去」⇒
     那时数据面与挂载面同时成立两份，读数不再唯一）。
  A3 **★核心：未挂 ⇒ 读必须明确失败**：未挂态下，解析原路径**必须**是 `ENOENT`（件不在 / 悬空符号链接）
     —— 解析到一个**目录**（含空目录）一律判红：那正是「静默返回空」的形态。
  A4 **挂了但读到空**：挂载点内**文件**数必须 ≥ `--min-files`（本机现读基线 **38**）且 > 0。
  A5 **挂了但读到错件**：样本件（默认 `版次索引.md`）必须**读得到且非空**；读不到 ⇒ 「挂的不是本档」。
  A6 **两处读数一致**：`mount` 说挂了而 `hdiutil info` 不认（或反过来）⇒ **rc=2 不给结论**（读数打架）。

成对负控（`--self-test` 13 格 · 合成夹具 · 一件真东西都不碰）
----------------------------------------------------------
  ① 未挂 + 原路径**不存在** ⇒ 0（这就是「明确失败」的样子）
  ①′ 未挂 + **悬空符号链接** ⇒ 0（同上 · 与 ① 成对）
  ② 未挂 + 原路径是**空目录** ⇒ 1（**静默返回空** ⇒ 本门要防的核心形态）—— 与 ① 成对
  ③ 未挂 + 原路径是**非空实体目录** ⇒ 1（数据面与挂载面同时两份）
  ④ 已挂 read-only + 件数够 + 样本读得到 ⇒ 0
  ⑤ 已挂**可写**（挂载行无 `read-only`）⇒ 1 —— 与 ④ 成对
  ⑥ 已挂 read-only 但**件数 0** ⇒ 1（「挂了但读到空」）—— 与 ④ 成对
  ⑦ 已挂 read-only 但**挂的是别的卷**（原路径不是符号链接）⇒ 1（「不是本档」）—— 与 ④ 成对
  ⑦′ 已挂 read-only 但**样本件读不到** ⇒ 1（「读到错件」）—— 与 ④ 成对
  ⑧ 两处读数打架（`mount` 有行 · `hdiutil` 不认）⇒ 2（不给结论）
  ⑨ 挂载表件读不到 / 0 行 ⇒ 2（空转 = 假覆盖，不许当绿）
  ⑩ 旋钮**显式**指的归档树根在盘上取不到 ⇒ 2（不给结论）—— 与 ⑩′ 成对
  ⑩′ 未挂**断链**（符号链接在）⇒ **不**算缺席（默认档常态 · 仍按 A3 判）

用法
----
    python3 scripts/gates/check-archive-mount.py                     # 真目标（本机归档区 + 现读挂载表）
    python3 scripts/gates/check-archive-mount.py --root <仓根>        # 换仓根（默认取本件上一级）
    python3 scripts/gates/check-archive-mount.py --mount-table <件> --path <原路径> --min-files N
    python3 scripts/gates/check-archive-mount.py --no-hdiutil         # 只判挂载表（跳过两处读数那一格）
    python3 scripts/gates/check-archive-mount.py --self-test          # 13 格合成夹具（不碰真目标）
    python3 scripts/gates/check-archive-mount.py --list-rules

取根体例（与同族批1/批2 同口径 · ✗ 不造新名）
------------------------------------------------
  归档树根（原路径）= `--path` 旗标 > env `ZERG_ARCHIVE_ROOT`（**现读在册唯一名**）> 默认 `<仓根同级>/Zerg-归档`。
  挂载点 = `--mountpoint` 旗标 > 默认 `/Volumes/ZERG-ARCHIVE`（**无 env 旋钮** —— 现读在册名单里没有挂载点那个名 · 不造新名）。
  缺席分支：**旋钮显式指的**归档树根在盘上完全取不到（连符号链接都没有）⇒ rc=2 **不给结论**；
  **默认档（无旋钮）不判这一档** —— 未挂**断链**（符号链接在、目标 = 挂载点不在）是常态（A3 核心格）。

退码：0 全绿 · 1 红（A1–A5 任一）· 2 **不给结论**（挂载表 0 行 / 两处读数打架 / 旋钮显式指的归档树根取不到 / `--self-test` 不过）

红线：**本门只读** —— 不挂载、不卸载、不写任何件、不 `chmod`、不删（归档区一个字节都不碰）。
     「挂/摘」只有一个入口：`bash scripts/svc/archive-mount.sh on|off|status`（按需 · 不信手挂）。
"""

import argparse
import os
import re
import subprocess
import sys
import tempfile

EXIT_OK, EXIT_RED, EXIT_NOCONCL = 0, 1, 2

DEFAULT_MOUNTPOINT = "/Volumes/ZERG-ARCHIVE"
DEFAULT_ARCHIVE_NAME = "Zerg-归档"      # 原路径的**名字**（三处硬编码就用这个字符串 · 名字一动三处全穿）
DEFAULT_SAMPLE = "版次索引.md"
DEFAULT_MIN_FILES = 38                  # 本机现读基线（设计稿 §三 3.2 表：38 件 + 5 目录 = 43 项）

RULES = [
    ("A1", "挂载行（按挂载点路径取）必须含 `read-only`", "判据 1 · 负控 ⑤⑦"),
    ("A2", "原路径必须是符号链接且 readlink == 挂载点", "判据 4 · 负控 ④"),
    ("A3", "★核心：未挂 ⇒ 读原路径必须**明确失败**（ENOENT）；解析到目录（含空）⇒ 红", "判据 9 · 负控 ④"),
    ("A4", "挂了但读到空：件数必须 ≥ --min-files 且 > 0", "判据 3 · 负控 ⑤⑥"),
    ("A5", "挂了但读到错件：样本件必须读得到且非空", "判据 2 的同一族 · 负控 ⑧"),
    ("A6", "两处读数（mount 行 ⟷ hdiutil info）不一致 ⇒ rc=2", "§三 3.3 交叉核对 · 负控 ⑥"),
]


class Verdict:
    def __init__(self):
        self.reds = []      # (码, 判词)
        self.notes = []     # 只报不判（读数）
        self.noconcl = []   # 不给结论

    def red(self, code, msg):
        self.reds.append((code, msg))

    def note(self, msg):
        self.notes.append(msg)

    def noconcl_(self, msg):
        self.noconcl.append(msg)


def parse_mount_table(text, mountpoint):
    """按**挂载点路径**取那一行（不是按卷名 —— 见设计稿 §三 3.5 的反证）。"""
    for line in text.splitlines():
        if " on %s (" % mountpoint in line:
            return line.rstrip("\n")
    return None


def mount_line_is_readonly(line):
    m = re.search(r"\(([^)]*)\)", line)
    opts = m.group(1) if m else ""
    return "read-only" in [o.strip() for o in opts.split(",")]


def hdiutil_knows(mountpoint, info_text):
    """`hdiutil info` 里认不认这个挂载点（两处读数的第二处）。"""
    return mountpoint in info_text


def count_files(path):
    n = 0
    for root, _dirs, files in os.walk(path):
        n += len(files)
    return n


def archive_root_absent(path):
    """归档树根在盘上**完全取不到**（既不是符号链接、也不是实体档/目录）。

    注意：**悬空符号链接不算缺席** —— 那是「未挂档」的常态（符号链接在、目标 = 挂载点不在），
    正是本门 A3 核心格要判的形态（读它**必须明确失败** = 对的样子）。缺席 = 连符号链接都没有。
    """
    return (not os.path.islink(path)) and (not os.path.exists(path))


def judge(verdict, mount_text, path, mountpoint, min_files, sample, hdiutil_text, hdiutil_checked):
    """纯函数式判定（合成夹具与真目标走同一条路 —— 自检因此有区分度）。"""
    if mount_text is None or not mount_text.strip():
        verdict.noconcl_("挂载表读不到 / 0 行 ⇒ 空转不给结论（工具：`mount`）")
        return
    mounted_line = parse_mount_table(mount_text, mountpoint)

    # ---- 原路径那一格 ----
    is_link = os.path.islink(path)
    exists = os.path.exists(path)
    if is_link:
        verdict.note("原路径 %s = 符号链接 → %s（判据 4 现读）" % (path, os.readlink(path)))
    elif exists:
        verdict.note("原路径 %s = **实体目录**（判据 4 要求它是符号链接）" % path)
    else:
        verdict.note("原路径 %s 不存在 / 解析不到（fail-closed 的那一态）" % path)

    if mounted_line is None:
        # ---- 未挂态：核心判据 ----
        verdict.note("挂载表里没有 ` on %s (` 那一行 ⇒ **未挂档**（按需加载的常态）" % mountpoint)
        if os.path.isdir(path) and not is_link:
            verdict.red("A3", "未挂档但原路径解析成一个**目录**（%s）⇒ 读它会**成功返回空**，"
                              "正是要防的「静默返回空」形态 ⇒ 红（挂载点那个空目录必须收掉）" % path)
        elif os.path.isdir(path) and is_link:
            verdict.red("A3", "未挂档但原路径（经符号链接）解析成一个**目录**（%s → %s）⇒ 读它会**成功返回空** ⇒ 红"
                       % (path, os.path.realpath(path)))
        elif exists and not os.path.isdir(path):
            verdict.red("A3", "未挂档但原路径是一个**普通件**（%s）⇒ 读它不是「明确失败」⇒ 红" % path)
        else:
            verdict.note("未挂档且原路径**解析不到**（ENOENT / 悬空符号链接）⇒ 读它**明确失败** ⇒ 这就是对的样子")
        if hdiutil_checked and hdiutil_text and hdiutil_knows(mountpoint, hdiutil_text):
            verdict.noconcl_("挂载表说未挂、`hdiutil info` 认这个挂载点 ⇒ **两处读数打架** ⇒ rc=2 不给结论")
        return

    # ---- 已挂态 ----
    verdict.note("挂载行现读：%s" % mounted_line)
    if not mount_line_is_readonly(mounted_line):
        verdict.red("A1", "挂载行**不含 `read-only`** ⇒ 挂的是**可写**档（chmod 那一条挡不住 owner 自己改回来）⇒ 红")
    if hdiutil_checked:
        if not hdiutil_text:
            verdict.noconcl_("`hdiutil info` 取不到 ⇒ 两处读数只剩一处 ⇒ 不给结论")
        elif not hdiutil_knows(mountpoint, hdiutil_text):
            verdict.noconcl_("挂载表说挂了、`hdiutil info` 不认 %s ⇒ **两处读数打架** ⇒ rc=2 不给结论" % mountpoint)
    if not is_link:
        verdict.red("A2", "已挂档但原路径**不是符号链接**（实体目录/件）⇒ 数据面与挂载面会同时成立两份，"
                          "读数不再唯一 ⇒ 红（判据 4）")
    # 件面
    if os.path.isdir(path):
        n = count_files(path)
        verdict.note("经原路径数到的**文件**数 = %d（基线 ≥ %d）" % (n, min_files))
        if n == 0:
            verdict.red("A4", "已挂档但**件数 0** ⇒ 「挂了但读到空」⇒ 分辨得出「挂了但卷里没东西」⇒ 红")
        elif n < min_files:
            verdict.red("A4", "已挂档但件数 %d < 基线 %d ⇒ 档不是本档（换了卷 / 拷了半份）⇒ 红" % (n, min_files))
        sp = os.path.join(path, sample)
        try:
            with open(sp, "rb") as f:
                body = f.read()
        except OSError as e:
            verdict.red("A5", "已挂档但**样本件读不到**（%s：%s）⇒ 「挂了但读到错件」⇒ 红" % (sp, e.strerror or e))
        else:
            if len(body) == 0:
                verdict.red("A5", "样本件 %s **读到了但是 0 字节** ⇒ 读到错件 ⇒ 红" % sp)
            else:
                verdict.note("样本件 %s 读得到（%d 字节）⇒ 不是空卷、也不是错件" % (sample, len(body)))
    else:
        verdict.red("A4", "挂载点 %s 原地取不到（挂载行在、路径不在）⇒ 读数打架 ⇒ 红" % mountpoint)


def read_text(cmd=None, path=None):
    """读一处读数：给了 `path` 就读件（夹具档），否则跑 `cmd`（现读档）。读不到 ⇒ None（不许当空串）。"""
    if path:
        try:
            with open(path, encoding="utf-8", errors="replace") as f:
                return f.read()
        except OSError:
            return None
    if not cmd:
        return None
    p = subprocess.run(list(cmd), capture_output=True, text=True)
    if p.returncode != 0:
        return None
    return p.stdout


def run_self_test():
    """13 格合成夹具（成对为主）—— 一件真东西都不碰。

    夹具里的挂载点路径是**临时目录**（不是 `/Volumes/ZERG-ARCHIVE`）⇒ 挂载表那一行**按夹具自己的
    挂载点**拼出来 —— 这一条同时现证了「按挂载点路径取行」而非按卷名（设计稿 §三 3.5）。
    """
    bad = 0
    total = 0

    def case(name, want, mode, build=None, hdi="knows", min_files=2, sample=DEFAULT_SAMPLE):
        nonlocal bad, total
        total += 1
        with tempfile.TemporaryDirectory() as d:
            mp = os.path.join(d, "mnt")
            os.makedirs(mp, exist_ok=True)
            path = os.path.join(d, DEFAULT_ARCHIVE_NAME)
            if build:
                build(d, mp, path)
            if mode is None:
                mount_text = None
            elif mode == "none":
                mount_text = "dev on / (apfs, local, read-only, sealed)\n"
            elif mode == "ro":
                mount_text = "dev on %s (apfs, local, nodev, read-only, journaled)\n" % mp
            else:  # rw
                mount_text = "dev on %s (apfs, local, nodev, journaled)\n" % mp
            if hdi == "knows":
                hdi_text, checked = ("hdiutil: %s\n" % mp), True
            elif hdi == "unknown":
                hdi_text, checked = "hdiutil:（没有这一条）\n", True
            else:
                hdi_text, checked = "", False
            v = Verdict()
            judge(v, mount_text, path, mp, min_files, sample, hdi_text, checked)
            rc = EXIT_NOCONCL if v.noconcl else (EXIT_RED if v.reds else EXIT_OK)
            ok = rc == want
            if not ok:
                bad += 1
            why = "" if ok else " ｜ " + (v.reds[0][1] if v.reds else str(v.noconcl))
            print("  %s %s ⇒ rc=%d（期望 %d）%s" % ("✓" if ok else "✗", name, rc, want, why))

    def no_mount(_d, _mp, _p):
        pass

    def empty_dir(_d, _mp, path):
        os.makedirs(path, exist_ok=True)

    def real_dir_with_files(_d, _mp, path):
        os.makedirs(path, exist_ok=True)
        for i in range(3):
            open(os.path.join(path, "f%d.md" % i), "w").write("x")
        open(os.path.join(path, "版次索引.md"), "w").write("索引")

    def dangling_link(d, _mp, path):
        os.symlink(os.path.join(d, "mnt-gone"), path)   # 挂载点目录被收掉 ⇒ 真·悬空符号链接

    def mounted_ok(d, mp, path):
        os.symlink(mp, path)
        for i in range(3):
            open(os.path.join(mp, "f%d.md" % i), "w").write("x")
        open(os.path.join(mp, "版次索引.md"), "w").write("索引")

    def mounted_empty(d, mp, path):
        os.symlink(mp, path)

    def mounted_real_dir(_d, _mp, path):
        os.makedirs(path, exist_ok=True)
        for i in range(3):
            open(os.path.join(path, "f%d.md" % i), "w").write("x")
        open(os.path.join(path, "版次索引.md"), "w").write("索引")

    def mounted_ok_no_sample(d, mp, path):
        os.symlink(mp, path)
        for i in range(3):
            open(os.path.join(mp, "f%d.md" % i), "w").write("x")

    case("① 未挂 + 原路径不存在 ⇒ 读**明确失败**（对的样子）", EXIT_OK, "none", build=no_mount, hdi="unknown")
    case("①′ 未挂 + 悬空符号链接 ⇒ 读**明确失败**（对的样子）", EXIT_OK, "none", build=dangling_link, hdi="unknown")
    case("② 未挂 + 原路径是**空目录** ⇒ 静默返回空（要防的核心形态）", EXIT_RED, "none", build=empty_dir, hdi="unknown")
    case("③ 未挂 + 原路径是**非空实体目录** ⇒ 数据面/挂载面同时两份", EXIT_RED, "none", build=real_dir_with_files, hdi="unknown")
    case("④ 已挂 read-only + 件数够 + 样本读得到 ⇒ 绿", EXIT_OK, "ro", build=mounted_ok)
    case("⑤ 已挂**可写**（挂载行无 read-only）⇒ 红", EXIT_RED, "rw", build=mounted_ok)
    case("⑥ 已挂 read-only 但**件数 0** ⇒ 「挂了但读到空」", EXIT_RED, "ro", build=mounted_empty)
    case("⑦ 已挂 read-only 但原路径是**实体目录** ⇒ 「不是本档」", EXIT_RED, "ro", build=mounted_real_dir)
    case("⑦′ 已挂 read-only 但**样本件读不到** ⇒ 「读到错件」", EXIT_RED, "ro", build=mounted_ok_no_sample)
    case("⑧ 两处读数打架（挂载表有行 · `hdiutil` 不认）⇒ 不给结论", EXIT_NOCONCL, "ro", build=mounted_ok, hdi="unknown")
    case("⑨ 挂载表 0 行 ⇒ 空转不给结论", EXIT_NOCONCL, None)

    # ⑩ 新档（批3）的牙：旋钮**显式**指的归档树根在盘上取不到 ⇒ 缺席档 rc=2；未挂**断链**不算缺席（成对）。
    with tempfile.TemporaryDirectory() as d10:
        absent_p = os.path.join(d10, "no-such-arch")
        dangling_p = os.path.join(d10, "dangling-arch")
        os.symlink(os.path.join(d10, "gone"), dangling_p)
        for label, want, probe in (
                ("⑩ 旋钮指到不在盘的归档树根 ⇒ 缺席档（rc=2 档）", True, absent_p),
                ("⑩′ 未挂断链（符号链接在）⇒ **不**算缺席（默认档常态）", False, dangling_p)):
            total += 1
            got = archive_root_absent(probe)
            ok = got == want
            if not ok:
                bad += 1
            print("  %s %s（archive_root_absent=%s · 期望 %s）" % ("✓" if ok else "✗", label, got, want))

    print("  ── 自检：%d 格 · %d 格不过" % (total, bad))
    return 0 if bad == 0 else EXIT_NOCONCL


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
    ap.add_argument("--root", default="")
    ap.add_argument("--mount-table", default="")
    ap.add_argument("--path", default="")
    ap.add_argument("--mountpoint", default=DEFAULT_MOUNTPOINT)
    ap.add_argument("--min-files", type=int, default=DEFAULT_MIN_FILES)
    ap.add_argument("--sample", default=DEFAULT_SAMPLE)
    ap.add_argument("--hdiutil-info", default="")
    ap.add_argument("--no-hdiutil", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        print("门㉛ 归档挂载档 —— 判据（A 档）；缺陷一格 = rc=1；读数打架/空转/旋钮指空的树根 = rc=2")
        for c, m, src in RULES:
            print("  %s %s（%s）" % (c, m, src))
        print("  红线：本门只读 —— 不挂/不摘/不写/不 chmod；挂摘只走 `bash scripts/svc/archive-mount.sh on|off|status`")
        return EXIT_OK

    if a.self_test:
        print("── 自检（合成夹具 · 13 格成对负控）──")
        rc = run_self_test()
        print("自检结论: %s" % ("全过" if rc == 0 else "不过（拒绝扫真目标）"))
        return rc

    # 仓根 = 本件上溯三级（`scripts/gates/<件>` → 仓根）；归档树根（原路径）= **仓根同级**的 `Zerg-归档`
    # （三处硬编码用的就是这个名字 —— 现读 `check-dangling-refs.py:646 ARCH_TREE_NAME` 等三点）。
    # 取根体例（与批1/批2 同口径 · ✗ 不造新名）：`--path` 旗标 > env `ZERG_ARCHIVE_ROOT`（**现读在册唯一名**）
    # > 默认 `<仓根同级>/Zerg-归档`（默认值逐字不变 ⇒ **无旋钮档零行为变更**）。
    root = a.root or os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    explicit_arch = a.path or (os.environ.get("ZERG_ARCHIVE_ROOT") or "")
    path = explicit_arch or os.path.join(os.path.dirname(os.path.abspath(root)), DEFAULT_ARCHIVE_NAME)
    # 缺席分支（批3 收口 · 同族「取不到根 ⇒ 不给结论」体例）：**旋钮显式指的**归档树根若在盘上完全取不到
    # （既不是符号链接、也不是实体档/目录）⇒ rc=2 **不给结论**（✗ 不当 0）。
    # ⚠ 默认档（无旋钮）**不判这一档**：本机 `../Zerg-归档` 是**未挂断链**（符号链接在、目标不在），
    #   那是本门 A3 核心格要判的常态（「未挂 ⇒ 读它明确失败」）⇒ 硬一律判 rc=2 会翻改现役默认档 ⇒ 只收口旋钮档。
    if explicit_arch and archive_root_absent(path):
        print("门㉛ 归档挂载档（`A8`）· 仓根 %s" % root)
        print("  ⛔ 归档树根（旋钮**显式**指到）在盘上取不到：%s" % path)
        print("     （既不是符号链接、也不是实体档/目录 ⇒ 挂载档判不了 ⇒ **不给结论**）")
        print("     旋钮现读：`--path`=%r · env `ZERG_ARCHIVE_ROOT`=%r"
              % (a.path, os.environ.get("ZERG_ARCHIVE_ROOT") or ""))
        print("     ⚠ 默认档（无旋钮）不判这一档 —— 未挂断链是常态（见件内注释）")
        print("⇒ **不给结论**（rc=2）")
        return EXIT_NOCONCL
    mount_text = read_text(path=a.mount_table) if a.mount_table else read_text(cmd=["mount"])
    if mount_text is None and not a.mount_table:
        print("门㉛ 归档挂载档：挂载表读不到（`mount` 跑了没成）⇒ **不给结论**（退码 2）")
        return EXIT_NOCONCL
    hdiutil_text = ""
    checked = False
    if not a.no_hdiutil:
        checked = True
        hdiutil_text = read_text(path=a.hdiutil_info) if a.hdiutil_info else (read_text(cmd=["hdiutil", "info"]) or "")

    print("门㉛ 归档挂载档（`A8`）· 仓根 %s" % root)
    print("  原路径     : %s" % path)
    print("  挂载点     : %s" % a.mountpoint)
    print("  件数基线   : ≥ %d（现读基线 38）" % a.min_files)
    v = Verdict()
    judge(v, mount_text, path, a.mountpoint, a.min_files, a.sample, hdiutil_text, checked)
    for n in v.notes:
        print("  · %s" % n)
    for code, msg in v.reds:
        print("  ✗ %s %s" % (code, msg))
    for msg in v.noconcl:
        print("  ⛔ %s" % msg)
    if v.noconcl:
        print("⇒ **不给结论**（rc=2）：%d 条" % len(v.noconcl))
        return EXIT_NOCONCL
    if v.reds:
        print("⇒ 红（rc=1）：%d 条" % len(v.reds))
        return EXIT_RED
    print("⇒ 全绿（rc=0）：挂载档 read-only · 原路径是符号链接 · 未挂则明确失败 · 件数与样本件都对得上")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

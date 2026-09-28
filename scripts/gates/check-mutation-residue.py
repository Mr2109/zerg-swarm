#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# check-mutation-residue.py —— 门72「变异残态」只读判据件
#
# 判什么：工作树里有没有躺着「已经变异、还没复原」的靶件 —— 即 `scripts/gates/mutation-smoke.sh`
#   突变点表里**每一点**的原文锚（`MUT_OLD`）是否还在、还在几条。
#   （病象：该烟测脚本原先在真树里原地改 + trap 复原，被 SIGKILL / 进程组收割时 trap 抓不到
#    ⇒ 突变体静默留在工作树里，靠人事后比 sha256 才发现。治本 = 烟测改到仓外副本跑；
#    本件治的是**另一半**：常跑门检每次把「残态」当场抓出来。）
#
# 真源（唯一）：`scripts/gates/mutation-smoke.sh`（本件**不写死**靶名 / 行号 / 候选行）——
#   ① `bash <该件> --list` 现读：**首行**是机读表头（`#` + TAB 连接的列名 —— 列名与列序的**唯一定义处**
#      是同件源码里的 `MUT_COLS` 数组），其后逐点出行；本件**按列名取数**（见 NEED_COLS），
#      **不按列数** ⇒ 生产侧往右**加列不破本件**（多出来的列一律忽略）；要的列缺一格才 rc=2；
#   ② 同件源码里的 `MUT_FILE` / `MUT_OLD` / `MUT_NEW` 三个数组：`--list` **不含原文锚**
#      ⇒ 原文锚只此一处，仍在**同一件**上 ⇒ 判据只有一个定义处（不另立第二套口径）。
#   ★ 两读必须**互相对得上**（点号 / 名 / 件 / 判据逐条相等，条数相等）；对不上 ⇒ rc=2 不给结论
#     （判据漂了就报「判不了」，绝不硬编靶名去猜）。
#
# 三档退码（与 `scripts/gates/precommit-gates.sh` 的 `tri` 档一一对应）：
#   0 = 全点**原形**（原文锚各命中恰 1 处）· 1 = 有**残态**（逐条点名那一件）· 2 = **判不了**。
# 逐点三态：
#   原形 = 原文锚命中恰 1 处；
#   畸形 = 原文锚命中 0 处（再细分：检出改后值 = 「改了」／两者都不在 = 「整行被删」）；
#   判不了 = 锚命中 > 1 处（歧义）· 靶件不在 · 表读不出 · 两读对不上。
#
# 只读（硬约束）：不写仓内 / 仓外任何件、不落审计、不读状态目录、不建临时文件（自检除外 ·
#   自检件一律落 `$TMPDIR` 下的合成树）、不起后台进程、不接管道取码。
#
# 用法：
#   python3 scripts/gates/check-mutation-residue.py [--root <仓根>]
#   python3 scripts/gates/check-mutation-residue.py --self-test

import argparse
import os
import re
import subprocess
import sys
import tempfile

TRUTH_REL = "scripts/gates/mutation-smoke.sh"
ARRAYS = ("MUT_FILE", "MUT_OLD", "MUT_NEW", "MUT_TEST", "MUT_NAME")

# ── 列契约（`--list` 的机读表头 · 缺口 GAP-20260928-116）───────────────────────────────
#   本件**只按列名取列**（**不按列数**）—— 原来写死 `len(f) != 4` ⇒ 生产侧往右加一列
#   整件就 rc=2（判不了，且是**静默**的：加列的人看不到「我这个下游被写死过」）。
#   现读口径：`--list` 首行 = `#` + TAB 连接的列名（生产侧 `MUT_COLS` 数组是列名与列序的
#   **唯一**定义处）；本件只认 NEED_COLS 里这几列，**多出来的列一律忽略**，
#   缺它要的那一列才 rc=2（不给结论 ≠ 绿）。表头认不出（首行不是 `#` 打头 / 列名重复）也 rc=2。
HEADER_PREFIX = "#"
NEED_COLS = ("点号", "名", "件", "点名判据")

_STR_RE = re.compile(r'^"((?:[^"\\]|\\.)*)"$')


def _unescape(s):
    return s.replace('\\"', '"').replace('\\\\', "\\")


def parse_arrays(src):
    """从真源件源码里读出五个索引数组；任何一处读不出 ⇒ None（判不了，不猜）。"""
    out = {}
    for name in ARRAYS:
        m = re.search(r"^%s=\(\s*$(.*?)^\)\s*$" % re.escape(name), src, re.M | re.S)
        if m is None:
            return None
        items = []
        for line in m.group(1).splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            mm = _STR_RE.match(line)
            if mm is None:
                return None
            items.append(_unescape(mm.group(1)))
        out[name] = items
    n = len(out["MUT_FILE"])
    if n == 0:
        return None
    for name in ARRAYS:
        if len(out[name]) != n:
            return None
    return out


def read_list(root, truth):
    """`bash <真源> --list` 现读；返回 (行列表, 列名→位次, 错误说明)。

    首行 = 机读表头（`#` + TAB 连接的列名）；其后为数据行。**按列名定位**：
    表头里多出来的列忽略（加列不破本件）；本件要的列缺一格 ⇒ 返回错误（rc=2）。
    """
    try:
        p = subprocess.run(["bash", truth, "--list"], cwd=root,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    except OSError as e:
        return None, None, "起不来：%s" % e
    if p.returncode != 0:
        return None, None, "--list rc=%d" % p.returncode
    lines = [raw for raw in p.stdout.decode("utf-8", "replace").splitlines()
             if raw.strip()]
    if not lines:
        return None, None, "--list 空表（0 点）"
    head = lines[0]
    if not head.startswith(HEADER_PREFIX):
        return None, None, "--list 首行不是表头（%s 打头的列名行不在）⇒ 列契约读不出" % HEADER_PREFIX
    cols = head[len(HEADER_PREFIX):].split("\t")
    if any(not c for c in cols) or len(set(cols)) != len(cols):
        return None, None, "--list 表头的列名有空格 / 重复（%s）" % head
    missing = [c for c in NEED_COLS if c not in cols]
    if missing:
        return None, None, "--list 表头缺本件要的列：%s（表头现读 %s）" % ("，".join(missing), head)
    idx = {c: cols.index(c) for c in NEED_COLS}
    rows = []
    for raw in lines[1:]:
        f = raw.split("\t")
        if len(f) != len(cols):
            return None, None, "--list 某行列数 %d ≠ 表头列数 %d" % (len(f), len(cols))
        rows.append([f[idx[c]] for c in NEED_COLS])
    if not rows:
        return None, None, "--list 空表（0 点 · 只有表头）"
    return rows, cols, ""


def judge(root, truth):
    """返回 (rc, 输出行列表)。rc：0 全原形 · 1 有残态 · 2 判不了。"""
    out = []
    out.append("== 门72 变异残态（只读）· 仓根 = %s" % root)
    if not os.path.isfile(truth):
        out.append("判不了：真源件不在（%s）⇒ 突变点表读不出 ⇒ 不给结论（rc=2）" % TRUTH_REL)
        return 2, out
    try:
        src = open(truth, encoding="utf-8").read()
    except OSError as e:
        out.append("判不了：真源件读不出（%s）⇒ 不给结论（rc=2）" % e)
        return 2, out
    arrays = parse_arrays(src)
    if arrays is None:
        out.append("判不了：真源件的五个索引数组读不出 / 条数不齐 / 0 点 ⇒ 不给结论（rc=2）")
        return 2, out
    rows, cols, err = read_list(root, truth)
    if rows is None:
        out.append("判不了：`--list` 不可解析（%s）⇒ 不给结论（rc=2）" % err)
        return 2, out
    n = len(arrays["MUT_FILE"])
    out.append("== 突变点表 = %s（`--list` 现读 %d 点 · 表头 %d 列：%s · 本件按名取 %d 列 · 原文锚同件源码现读 %d 点）"
               % (TRUTH_REL, len(rows), len(cols), " / ".join(cols), len(NEED_COLS), n))
    if len(rows) != n:
        out.append("判不了：两读条数不等（--list %d ≠ 源码数组 %d）⇒ 不给结论（rc=2）"
                   % (len(rows), n))
        return 2, out
    for i, f in enumerate(rows):
        if f[0] != str(i) or f[1] != arrays["MUT_NAME"][i] or f[2] != arrays["MUT_FILE"][i] \
                or f[3] != arrays["MUT_TEST"][i]:
            out.append("判不了：两读第 %s 点对不上（--list 名/件/判据 ≠ 源码数组）⇒ 不给结论（rc=2）" % i)
            return 2, out

    bad = []
    bad2 = []
    for i, f in enumerate(rows):
        rel = arrays["MUT_FILE"][i]
        old = arrays["MUT_OLD"][i]
        new = arrays["MUT_NEW"][i]
        path = os.path.join(root, rel)
        if not old:
            out.append("%d\t判不了\t%s\t原文锚为空 ⇒ 判不了" % (i, rel))
            bad2.append(rel)
            continue
        if not os.path.isfile(path):
            out.append("%d\t判不了\t%s\t靶件不在（%s）⇒ 判不了" % (i, rel, "缺件"))
            bad2.append(rel)
            continue
        try:
            body = open(path, encoding="utf-8").read()
        except OSError as e:
            out.append("%d\t判不了\t%s\t读不出（%s）⇒ 判不了" % (i, rel, e))
            bad2.append(rel)
            continue
        hits = body.count(old)
        if hits == 1:
            out.append("%d\t原形\t%s\t原文锚命中 1 处 ✓（%s）" % (i, rel, arrays["MUT_NAME"][i]))
        elif hits == 0:
            if new and new in body:
                out.append("%d\t畸形\t%s\t原文锚命中 0 处 · 检出改后值「%s」⇒ 残态（%s）"
                           % (i, rel, new, arrays["MUT_NAME"][i]))
            else:
                out.append("%d\t畸形\t%s\t原文锚命中 0 处 · 改后值也不在（含锚整行被删）⇒ 残态（%s）"
                           % (i, rel, arrays["MUT_NAME"][i]))
            bad.append((i, rel))
        else:
            out.append("%d\t判不了\t%s\t原文锚命中 %d 处（应为 1）⇒ 歧义 ⇒ 判不了" % (i, rel, hits))
            bad2.append(rel)

    if bad:
        out.append("✗ 残态点名（工作树里躺着未复原的突变）：")
        for i, rel in bad:
            out.append("    · %s（第 %s 点 · %s）" % (rel, i, arrays["MUT_NAME"][i]))
        out.append("⇒ 有 %d 点残态（rc=1）：本读数有牙 —— 残态即判红，不许当绿" % len(bad))
        return 1, out
    if bad2:
        out.append("⇒ 有 %d 点判不了（rc=2 · 不给结论 ≠ 绿）：%s" % (len(bad2), " ".join(bad2)))
        return 2, out
    out.append("⇒ 全 %d 点**原形**（rc=0）：工作树里没有未复原的突变" % n)
    return 0, out


# ── 自检（合成树 · $TMPDIR 下 · 不碰真目标）─────────────────────────────────────────────
SELF_TOTAL = 0
SELF_BAD = 0


def assert_eq(desc, actual, expected):
    global SELF_TOTAL, SELF_BAD
    SELF_TOTAL += 1
    if str(actual) == str(expected):
        print("   ✓ %s" % desc)
    else:
        print("   ✗ %s（实际=%s 期望=%s）" % (desc, actual, expected))
        SELF_BAD += 1


SYNTH = """#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\\t'
  printf '#%s\\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\\t%s\\t%s\\t%s\\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
"""

# 加一列（右端）的合成生产侧：表头加一格 + 行加一格 + 数组加一格 —— **既有四格逐字不变**。
# 本件**必须**对这份照样 rc=0（缺口 GAP-20260928-116 的核心判据）。
SYNTH_ADDCOL = (SYNTH
                .replace('  "点名判据"\n)',
                         '  "点名判据"\n  "备注"\n)')
                .replace("printf '%d\\t%s\\t%s\\t%s\\n' \"$i\" \"${MUT_NAME[$i]}\" "
                         "\"${MUT_FILE[$i]}\" \"${MUT_TEST[$i]}\"",
                         "printf '%d\\t%s\\t%s\\t%s\\t%s\\n' \"$i\" \"${MUT_NAME[$i]}\" "
                         "\"${MUT_FILE[$i]}\" \"${MUT_TEST[$i]}\" \"第 $i 点备注\""))

# 缺一列（本件要的「点名判据」那一格从表头去掉）⇒ 必须 rc=2（不给结论 ≠ 绿）。
SYNTH_DROPCOL = (SYNTH
                 .replace('  "点名判据"\n)',
                          '  "别的东西"\n)'))

GOOD_A = "func main() {\n\tif err := adp.Start(); err != nil {\n\t\tpanic(err)\n\t}\n}\n"
GOOD_B = "cfg := Config{\n\tTemperature:     0.8,\n}\n"


def _mktree(tmp, name, a_body, b_body, script=None):
    root = os.path.join(tmp, name)
    os.makedirs(os.path.join(root, "scripts", "gates"))
    with open(os.path.join(root, TRUTH_REL), "w", encoding="utf-8") as fh:
        fh.write(SYNTH if script is None else script)
    with open(os.path.join(root, "a.go"), "w", encoding="utf-8") as fh:
        fh.write(a_body)
    with open(os.path.join(root, "b.go"), "w", encoding="utf-8") as fh:
        fh.write(b_body)
    return root


def run_self_test():
    tmp = tempfile.mkdtemp(prefix="mutresidue-selftest-")
    print("── 门72 自检（合成件 · $TMPDIR 下 · 不碰真目标）──")

    # ① 全原形 ⇒ rc=0
    r1 = _mktree(tmp, "ok", GOOD_A, GOOD_B)
    rc, lines = judge(r1, os.path.join(r1, TRUTH_REL))
    assert_eq("① 合成树全原形 ⇒ rc=0", rc, 0)
    assert_eq("① 输出逐点报「原形」= 2 行", sum(1 for ln in lines if "\t原形\t" in ln), 2)

    # ② 改后值在（0.8 → 0.9）⇒ rc=1 且**点名那一件**
    r2 = _mktree(tmp, "mut-new", GOOD_A, GOOD_B.replace("0.8", "0.9"))
    rc, lines = judge(r2, os.path.join(r2, TRUTH_REL))
    assert_eq("② 换值型残态 ⇒ rc=1", rc, 1)
    assert_eq("② 畸形行数 = 1", sum(1 for ln in lines if "\t畸形\t" in ln), 1)
    assert_eq("② 逐条点名的是 b.go", sum(1 for ln in lines if "· b.go（第 1 点" in ln), 1)

    # ③ 删行型残态（NEW 为空）⇒ rc=1 且点名 a.go
    r3 = _mktree(tmp, "mut-del", "func main() {\n}\n", GOOD_B)
    rc, lines = judge(r3, os.path.join(r3, TRUTH_REL))
    assert_eq("③ 删行型残态 ⇒ rc=1", rc, 1)
    assert_eq("③ 逐条点名的是 a.go", sum(1 for ln in lines if "· a.go（第 0 点" in ln), 1)

    # ④ 锚多命中（歧义）⇒ rc=2（判不了 ≠ 绿）
    r4 = _mktree(tmp, "ambig", GOOD_A + "adp.Start(); err != nil {\n", GOOD_B)
    rc, _ = judge(r4, os.path.join(r4, TRUTH_REL))
    assert_eq("④ 原文锚多命中 ⇒ 判不了 rc=2（不给结论）", rc, 2)

    # ⑤ 靶件不在 ⇒ rc=2
    r5 = _mktree(tmp, "missing", GOOD_A, GOOD_B)
    os.remove(os.path.join(r5, "a.go"))
    rc, _ = judge(r5, os.path.join(r5, TRUTH_REL))
    assert_eq("⑤ 靶件不在 ⇒ 判不了 rc=2", rc, 2)

    # ⑥ 两读对不上（源码数组与 --list 的输出不一致）⇒ rc=2
    r6 = _mktree(tmp, "drift", GOOD_A, GOOD_B,
                 script=SYNTH.replace('"${MUT_NAME[$i]}"', '"改过的名字"'))
    rc, _ = judge(r6, os.path.join(r6, TRUTH_REL))
    assert_eq("⑥ 表漂了（源码数组 ≠ --list）⇒ 判不了 rc=2", rc, 2)

    # ⑦ 真源件缺 ⇒ rc=2
    r7 = _mktree(tmp, "notruth", GOOD_A, GOOD_B)
    os.remove(os.path.join(r7, TRUTH_REL))
    rc, _ = judge(r7, os.path.join(r7, TRUTH_REL))
    assert_eq("⑦ 真源件不在 ⇒ 判不了 rc=2", rc, 2)

    # ⑧ --list 行与表头不对齐（行列数不足）⇒ rc=2
    r8 = _mktree(tmp, "badlist", GOOD_A, GOOD_B,
                 script=SYNTH.replace("printf '%d\\t%s\\t%s\\t%s\\n'",
                                      "printf '%d\\t%s\\n'"))
    rc, _ = judge(r8, os.path.join(r8, TRUTH_REL))
    assert_eq("⑧ `--list` 行与表头不对齐 ⇒ 判不了 rc=2", rc, 2)

    # ⑨ ★ 加一列（右端 · 既有四格逐字不变）⇒ **必须 rc=0**（缺口 GAP-20260928-116 的核心判据：
    #   按列名取数之后，生产侧长列不再把下游打成 rc=2）
    assert_eq("⑨ 夹具 SYNTH_ADDCOL 真的加上了第五列（替换生效）",
              SYNTH_ADDCOL != SYNTH and SYNTH_ADDCOL.count("\\t%s") == SYNTH.count("\\t%s") + 1, True)
    r9 = _mktree(tmp, "addcol", GOOD_A, GOOD_B, script=SYNTH_ADDCOL)
    rc, lines = judge(r9, os.path.join(r9, TRUTH_REL))
    assert_eq("⑨ 生产侧右端加一列 ⇒ rc=0（加列不破本件）", rc, 0)
    assert_eq("⑨ 加列后仍逐点报「原形」= 2 行", sum(1 for ln in lines if "原形" in ln and "原文锚命中 1 处" in ln), 2)

    # ⑩ 缺本件要的那一列（表头「点名判据」没了）⇒ rc=2（不给结论 ≠ 绿）
    assert_eq("⑩ 夹具 SYNTH_DROPCOL 真的去掉了要的那一列（替换生效）",
              SYNTH_DROPCOL != SYNTH, True)
    r10 = _mktree(tmp, "dropcol", GOOD_A, GOOD_B, script=SYNTH_DROPCOL)
    rc, lines = judge(r10, os.path.join(r10, TRUTH_REL))
    assert_eq("⑩ 表头缺「点名判据」列 ⇒ 判不了 rc=2", rc, 2)
    assert_eq("⑩ 报的是「缺列」不是别的",
              sum(1 for ln in lines if "缺本件要的列：点名判据" in ln), 1)

    # ⑪ 表头整行不在（老式无表头的 `--list`）⇒ rc=2（列契约读不出，不猜）
    r11 = _mktree(tmp, "noheader", GOOD_A, GOOD_B,
                  script=SYNTH.replace("printf '#%s\\n' \"${MUT_COLS[*]}\"\n", ""))
    rc, _ = judge(r11, os.path.join(r11, TRUTH_REL))
    assert_eq("⑪ `--list` 无表头 ⇒ 判不了 rc=2", rc, 2)

    print("── 自检结论：断言 %d 条 · 失败 %d 条 ──" % (SELF_TOTAL, SELF_BAD))
    return 0 if SELF_BAD == 0 else 1


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=None)
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    args = ap.parse_args()
    if args.self_test:
        return run_self_test()
    root = args.root or os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    root = os.path.abspath(root)
    rc, lines = judge(root, os.path.join(root, TRUTH_REL))
    for ln in lines:
        print(ln)
    return rc


if __name__ == "__main__":
    sys.exit(main())

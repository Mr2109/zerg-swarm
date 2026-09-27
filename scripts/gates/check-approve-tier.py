#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-tier.py v1.0.0 —— 人签「档位可读面」的判据件（2.5.11 人签 D 批 · `D1`）

为什么存在（`D1` 的任务原文，逐字口径）
--------------------------------------------------
`D1` 要 `approve ls` 人类面**加一列「档」**、`--json` 加**可选**字段，落法顺序 = **先按加列落，红了退一格**。
本件就是那一格的**判据面**：把「现跑的六列 / 八行 / 九字段**语义与顺序一字不动**」写成**可跑的判据**，
外加 `D1` 判据④ 的词汇面（**允许** `passphrase`、**不许** `token` / `PIN`）与 `M-20` 的现跑那一格。

★ 本件**只判、不改**：它读现网状态目录（只读）与合成夹具，一个字节都不写；它**不落地任何新列 / 新字段** ——
「档位标记打什么形态」（显式打 `strength=passphrase` 还是只靠 `sig_alg=ed25519` 那一行 · `P26`）
**仍待 Mr2109 亲选**，落地等那一拍（`v1.3 §9.4` 的 `P26` 行 · `D1` 的判据面**先立**）。

判据（T1–T6 · 每条都能单独失败）
--------------------------------------------------
  T1 **`approve ls` 人面六列**：`tool / approver / approved_at / scope / state / path` 必须**按序**出现
     （**新增列允许** —— 只加列、不删列、不换序；**换序或删列 ⇒ 红**）。
  T2 **`approve show` 人面八行**：`工具 / 批准者 / 签的时间 / 范围 / 理由 / 签名 / 判决 / 落点` 按序出现
     （新增行允许 · 换序或删行 ⇒ 红）——`D3` 的「钥匙身份」行就是**新增行**那一类。
  T3 **字段面九键**：`family_approve.go` 的 `approveFields` 逐字 = 九键，且 `strength` / `key_id_in_use`
     **不在**里面（`D1` 红线：不许把它塞进必填集 ⇒ 旧消费者当场坏）。
  T4 **字段面先判**：`--json <未知字段>` ⇒ **rc=2**（空结果集下也不许悄悄变合法）。另打一行**读数**：
     `--json strength` 今天的退码（`P26` 未拍 ⇒ 这一格只是读数，**不判**）。
  T5 **词汇面**：人面输出**不许**出现 `token` / `PIN`（`M-20` ②）；`passphrase` **允许**（`D1` 判据④）。
  T6 **`M-20` 现跑那一格**：`approve keygen --strength token --by <名>` ⇒ **rc=2 · 未知旗标**，
     且**沙箱状态目录**（真状态目录零写）。

退码
--------------------------------------------------
  0 = 全绿（T1–T6 全过，且自检 10 格全过）
  1 = 有红（任一条判据破）
  2 = **不给结论**（`bin/zerg` 不在 · 夹具造不出来 · 三处读数取不到）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-tier.py            # 先自检，再扫真目标
  python3 scripts/gates/check-approve-tier.py --self-test # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-tier.py --root <仓根>
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-tier.py v1.0.0"

EXPECTED_LS_COLS = ["tool", "approver", "approved_at", "scope", "state", "path"]
# ── 历史已知列名集（`GAP-20260927-29`）：**只增不减** —— 判「丢了已知列」那一格 ──────────
# 现读全量（`approve ls` 现跑表头 · `family_approve.go:235` 的落表）：六列 + `D1` 落地的
# 第七列「档」。`EXPECTED_LS_COLS` 管**顺序 + 必填**（新增列允许）；本集管**丢失** ——
# 输入列名集里少掉其中任一列（改名 / 删列 / 被合并）⇒ 红。两判据**共存**：缺的那一列若
# 不在必填集里（如「档」），有序超集判据**抓不住** ⇒ 才要本集。★ 落地新列时在此**追加**。
KNOWN_LS_COLS = ["tool", "approver", "approved_at", "scope", "state", "path", "档"]
# ── 历史已知行标签集（`GAP-20260927-29`）：**只增不减** —— 判「丢了已知标签」那一格 ────────
# 现读全量（`approve show` 现跑 11 行）：必填八行 + `D1` 落地的「档」+ `D3` 落地的
# 「钥匙身份」「有效期」。`EXPECTED_SHOW_LABELS` 管**顺序 + 必填**（新增行允许）；本集管
# **丢失** —— 输入行标签集里少掉其中任一标签（改名 / 删行 / 被合并）⇒ 红。两判据**共存**：
# 缺的那个标签若不在必填集里（如「有效期」），有序超集判据**抓不住** ⇒ 才要本集。
# ★ 落地新行时在此**追加**。
KNOWN_SHOW_LABELS = ["工具", "批准者", "签的时间", "范围", "理由", "签名", "档", "钥匙身份", "有效期", "判决", "落点"]
EXPECTED_SHOW_LABELS = ["工具", "批准者", "签的时间", "范围", "理由", "签名", "判决", "落点"]
EXPECTED_FIELDS = ["tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"]
MUST_NOT_BE_REQUIRED = ["strength", "key_id_in_use"]
FORBIDDEN_TOKENS = ["token", "PIN"]
ALLOWED_TOKENS = ["passphrase"]

APPROVE_GO_REL = os.path.join("core", "cmd", "zerg", "family_approve.go")


# ── 判定口（纯函数：自检与真目标**共用同一套** ⇒ 自检绿不代表真目标绿之外还有第二套口径）──────

def judge_ordered_superset(actual, expected, what):
    """**有序超集**：`expected` 必须按序在 `actual` 里出现（新增项允许 · 换序/缺项 ⇒ 问题行）。"""
    problems, i = [], 0
    for e in expected:
        if e not in actual[i:]:
            problems.append("%s：%r 在该序里找不到（换序或删项 ⇒ 红 · 新增项允许）" % (what, e))
            continue
        i = actual.index(e, i) + 1
    return problems


def judge_columns(cols, expected=None):
    return judge_ordered_superset(cols, expected or EXPECTED_LS_COLS, "T1 `approve ls` 列")


def judge_known_columns_lost(cols, known=None):
    """**已知列名丢失**：`known`（历史全量）里任一列名不在 `cols` 里 ⇒ 问题行。

    与 `judge_ordered_superset`（顺序 + 新增列允许）**互补共存**：后者管「必填列按序在不在」、
    前者管「全量已知列有没有少」—— 少掉的那一列不在必填集里时，只有本判据会红。
    """
    known = known if known is not None else KNOWN_LS_COLS
    return ["T1 破：已知列 %r 丢了（改名 / 删列 / 被合并 ⇒ 红 · 已知列名集只增不减）" % c
            for c in known if c not in cols]


def judge_labels(labels, expected=None):
    return judge_ordered_superset(labels, expected or EXPECTED_SHOW_LABELS, "T2 `approve show` 行")


def judge_known_labels_lost(labels, known=None):
    """**已知行标签丢失**：`known`（历史全量）里任一标签不在 `labels` 里 ⇒ 问题行。

    与 `judge_labels`（顺序 + 新增行允许）**互补共存**：后者管「必填标签按序在不在」、
    前者管「全量已知标签有没有少」—— 少掉的那个标签不在必填集里时，只有本判据会红。
    """
    known = known if known is not None else KNOWN_SHOW_LABELS
    return ["T2 破：已知行 %r 丢了（改名 / 删行 / 被合并 ⇒ 红 · 已知行标签集只增不减）" % c
            for c in known if c not in labels]


def judge_fields(fields):
    problems = []
    if fields != EXPECTED_FIELDS:
        problems.append("T3 字段面 ≠ 九键：现读 = %s" % ",".join(fields))
    for bad in MUST_NOT_BE_REQUIRED:
        if bad in fields:
            problems.append("T3 破：%r 被塞进必填集（`D1` 红线 · 旧消费者会当场坏）" % bad)
    return problems


def judge_vocab(text):
    problems = []
    for bad in FORBIDDEN_TOKENS:
        if bad in text:
            problems.append("T5 破：输出里出现 %r（`M-20` ② 不许）" % bad)
    return problems


def parse_ls_columns(stdout):
    """列面 = **表头那一行**（`approve ls` 首行逐字是六列名 · tab 分格）；数据行另核格数一致。"""
    lines = [ln for ln in stdout.splitlines() if ln.strip()]
    if not lines:
        return []
    return lines[0].split("\t")


def judge_row_widths(stdout):
    """每一行的格数必须与表头一致（少一格 = 某列没渲染出来 ⇒ 红）。"""
    lines = [ln for ln in stdout.splitlines() if ln.strip()]
    if not lines:
        return ["T1 破：`approve ls` 一行都没有（表头也缺）"]
    n = len(lines[0].split("\t"))
    return ["T1 破：第 %d 行格数 %d ≠ 表头 %d" % (i + 1, len(ln.split("\t")), n)
            for i, ln in enumerate(lines) if len(ln.split("\t")) != n]


def parse_show_labels(stdout):
    labels = []
    for ln in stdout.splitlines():
        m = re.match(r"^(\S+)\s*:", ln)
        if m:
            labels.append(m.group(1))
    return labels


def parse_fields_from_go(src):
    m = re.search(r"var approveFields = \[\]string\{([^}]*)\}", src)
    if not m:
        return None
    return re.findall(r'"([^"]+)"', m.group(1))


# ── 现跑 ─────────────────────────────────────────────────────────────────────────

def resolve_bin(repo, tmpdir):
    """读数要用**真制品**：先在 `bin/zerg`（本机自举档产物），没有就**现场构建到临时目录**。

    ★ `bin/` 不入库（`.gitignore` 第 25 行）⇒ 新克隆上没有制品：现场构建是为了让这一步在
      「没跑过自举档」的机器上**仍给结论**，而不是恒 `BLOCKED`（同门④/门⑧ 那条「不给结论 ≠ 绿」
      口径的**上游**解法）。构建产物落在调用方的临时目录里 —— **不碰 `bin/`** ✗。
    """
    exe = os.path.join(repo, "bin", "zerg")
    if os.path.exists(exe):
        return exe, ""
    if shutil.which("go") is None:
        return None, "`bin/zerg` 不在且 `go` 不在 PATH ⇒ 取不到读数"
    out = os.path.join(tmpdir, "zerg-approve-tier")
    p = subprocess.run(["go", "build", "-buildvcs=false", "-o", out, "./cmd/zerg"],
                       cwd=os.path.join(repo, "core"), capture_output=True, text=True)
    if p.returncode != 0 or not os.path.exists(out):
        return None, "`bin/zerg` 不在且现场构建失败（rc=%d）：%s" % (p.returncode, p.stderr.strip()[-200:])
    return out, "现场构建（`bin/` 不入库）"


def run_cli(exe, repo, state_dir, argv):
    if exe is None:
        return None, "", ""
    env = dict(os.environ)
    env["ZERG_STATE_DIR"] = state_dir
    p = subprocess.run([exe] + argv, cwd=repo, env=env, capture_output=True, text=True)
    return p.returncode, p.stdout, p.stderr


def make_fixture(state_dir):
    """合成夹具：一枚**无签名**的批准件（形状与在册同格式）—— 只用来取**渲染面**的列 / 行。"""
    d = os.path.join(state_dir, "approvals")
    os.makedirs(d, exist_ok=True)
    tk = {
        "tool": "合成件", "approver": "Mr2109", "approved_at": "2026-09-22T10:00:00+08:00",
        "scope": "*", "note": "D1 判据件的夹具（无签名 ⇒ 判决那一列照实说「无签名」）",
        "sig_alg": "ed25519", "key_id": "0000000000000000", "sig": "",
    }
    with open(os.path.join(d, "合成件.json"), "w", encoding="utf-8") as f:
        json.dump(tk, f, ensure_ascii=False, indent=1)
        f.write("\n")


def collect_readings(repo, state_dir, real_state_dir, tmpdir):
    """取三处读数（列 / 行 / 字段）。取不到 ⇒ (None, 未取数理由)。"""
    exe, note = resolve_bin(repo, tmpdir)
    rc_ls, out_ls, err_ls = run_cli(exe, repo, state_dir, ["approve", "ls"])
    if rc_ls is None:
        return None, "取不到读数：%s" % note
    rc_show, out_show, err_show = run_cli(exe, repo, state_dir, ["approve", "show", "合成件"])
    go = os.path.join(repo, APPROVE_GO_REL)
    if not os.path.exists(go):
        return None, "本件读不到 `%s` ⇒ 取不到字段面" % APPROVE_GO_REL
    with open(go, encoding="utf-8") as f:
        fields = parse_fields_from_go(f.read())
    if fields is None:
        return None, "`approveFields` 在 %s 里解不出来 ⇒ 取不到字段面" % APPROVE_GO_REL
    rc_unknown, out_unknown, err_unknown = run_cli(exe, repo, state_dir, ["approve", "ls", "--json", "nosuchfield-xyz"])
    rc_strength, _, err_strength = run_cli(exe, repo, state_dir, ["approve", "ls", "--json", "strength"])
    return {
        "ls": (rc_ls, out_ls, err_ls),
        "show": (rc_show, out_show, err_show),
        "fields": fields,
        "unknown": (rc_unknown, out_unknown, err_unknown),
        "strength": (rc_strength, err_strength),
        "real": real_state_dir, "exe": exe, "bin_note": note,
    }, ""


# ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

def self_test(repo):
    """10 格：① 正控（真输出过）② 换序 ⇒ 红 ③ 删列 ⇒ 红 ④ show 删行 ⇒ 红
    ⑤ 含 `token` ⇒ 红 ⑥ 含 `passphrase` ⇒ 不红 ⑦ 字段表塞 `strength` ⇒ 红
    ⑧ 数据行少一格 ⇒ 红 ⑨ 丢已知列 ⇒ 红（有序超集抓不住）⑩ 丢已知标签 ⇒ 红（同族）。"""
    ok = 0
    bad = 0
    tmp = tempfile.mkdtemp(prefix="zerg-approve-tier-selftest-")
    try:
        make_fixture(tmp)
        r, why = collect_readings(repo, tmp, None, tmp)
        if r is None:
            print("⛔ 自检① 取不到读数：%s ⇒ 不给结论" % why)
            return None
        rc_ls, out_ls, _ = r["ls"]
        cols = parse_ls_columns(out_ls)
        labels = parse_show_labels(r["show"][1])
        if rc_ls == 0 and not judge_columns(cols) and not judge_labels(labels) and not judge_fields(r["fields"]) \
                and not judge_row_widths(out_ls):
            print("   ✓ ① 正控：现跑人面六列 / 八行 / 九字段都过（列数 %d · 行标签 %d）" % (len(cols), len(labels)))
            ok += 1
        else:
            print("   ✗ ① 正控失败：现跑面就被判红了（列 %s · 行 %s）" % (cols, labels))
            bad += 1
        narrow = "\n".join(["\t".join(cols)] + ["\t".join(["x"] * (len(cols) - 1))])
        if judge_row_widths(narrow):
            print("   ✓ ⑧ 负控：数据行**少一格** ⇒ 判红")
            ok += 1
        else:
            print("   ✗ ⑧ 负控失败：少一格还判绿")
            bad += 1
        swap = list(cols)
        if len(swap) >= 2:
            swap[0], swap[1] = swap[1], swap[0]
        if judge_columns(swap):
            print("   ✓ ② 负控：六列**换序** ⇒ 判红")
            ok += 1
        else:
            print("   ✗ ② 负控失败：换序了还判绿")
            bad += 1
        drop = list(cols)
        if EXPECTED_LS_COLS[-1] in drop:
            drop.remove(EXPECTED_LS_COLS[-1])
        if judge_columns(drop):
            print("   ✓ ③ 负控：**删一列**（必填列 `path`）⇒ 判红")
            ok += 1
        else:
            print("   ✗ ③ 负控失败：删列了还判绿")
            bad += 1
        lost_known = list(KNOWN_LS_COLS)[:-1]      # 历史全量**去掉末列**（现读 = 第七列「档」）
        j_sup = judge_columns(lost_known)
        j_lost = judge_known_columns_lost(lost_known)
        if not j_sup and j_lost:
            print("   ✓ ⑨ 负控：**丢一列已知列**（%r）⇒ 新判据判红；同一输入**现有有序超集抓不住**"
                  " (%d 条) —— 本单缺口 %s 的当场证据" % (KNOWN_LS_COLS[-1], len(j_sup), "GAP-20260927-29"))
            ok += 1
        else:
            print("   ✗ ⑨ 负控失败：丢已知列 %r 没按预期分叉（有序超集 %d 条 · 已知列丢失 %d 条）"
                  % (KNOWN_LS_COLS[-1], len(j_sup), len(j_lost)))
            bad += 1
        new_labels = [x for x in KNOWN_SHOW_LABELS if x not in EXPECTED_SHOW_LABELS]
        lost_label = new_labels[-1]      # 历史全量里**非必填**的末一个（现读 = 「有效期」）
        lsup = list(labels)
        if lost_label in lsup:
            lsup.remove(lost_label)
        j_l_sup = judge_labels(lsup)
        j_l_lost = judge_known_labels_lost(lsup)
        if not j_l_sup and j_l_lost:
            print("   ✓ ⑩ 负控：**丢一行已知行**（%r）⇒ 新判据判红；同一输入**现有有序超集抓不住**"
                  " (%d 条) —— 本单缺口 %s 的当场证据" % (lost_label, len(j_l_sup), "GAP-20260927-29"))
            ok += 1
        else:
            print("   ✗ ⑩ 负控失败：丢已知行 %r 没按预期分叉（有序超集 %d 条 · 已知行丢失 %d 条）"
                  % (lost_label, len(j_l_sup), len(j_l_lost)))
            bad += 1
        ldrop = list(labels)
        if EXPECTED_SHOW_LABELS[-1] in ldrop:
            ldrop.remove(EXPECTED_SHOW_LABELS[-1])
        if judge_labels(ldrop):
            print("   ✓ ④ 负控：**删一行**（必填标签 `%s`）⇒ 判红" % EXPECTED_SHOW_LABELS[-1])
            ok += 1
        else:
            print("   ✗ ④ 负控失败：删行了还判绿")
            bad += 1
        if judge_vocab(out_ls + "\n" + "档：token 触摸\n"):
            print("   ✓ ⑤ 负控：输出里出现 `token` ⇒ 判红（`M-20` ②）")
            ok += 1
        else:
            print("   ✗ ⑤ 负控失败：出现 `token` 还判绿")
            bad += 1
        if not judge_vocab(out_ls + "\n" + "档：passphrase（口令）\n"):
            print("   ✓ ⑥ 正控：输出里出现 `passphrase` ⇒ **不**判红（生效档自己 · `D1` 判据④）")
            ok += 1
        else:
            print("   ✗ ⑥ 正控失败：`passphrase` 被当成禁词了")
            bad += 1
        if judge_fields(list(r["fields"]) + ["strength"]):
            print("   ✓ ⑦ 负控：字段表里塞 `strength` ⇒ 判红（`D1` 红线）")
            ok += 1
        else:
            print("   ✗ ⑦ 负控失败：塞进必填集还判绿")
            bad += 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("   自检：%d 格过 · %d 格不过（合成夹具目录已清理 · 真状态目录一个字节未动）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo):
    real = os.path.join(os.path.expanduser("~"), ".zerg", "state")
    if not os.path.isdir(real):
        real = None
    tmp = tempfile.mkdtemp(prefix="zerg-approve-tier-")
    try:
        make_fixture(tmp)
        r, why = collect_readings(repo, tmp, real, tmp)
        if r is None:
            print("⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 不许把读不到当绿）" % why)
            return 2
        problems = []
        rc_ls, out_ls, err_ls = r["ls"]
        cols = parse_ls_columns(out_ls)
        problems += judge_columns(cols)
        problems += judge_known_columns_lost(cols)
        problems += judge_row_widths(out_ls)
        labels = parse_show_labels(r["show"][1])
        problems += judge_labels(labels)
        problems += judge_known_labels_lost(labels)
        problems += judge_fields(r["fields"])
        problems += judge_vocab(out_ls + "\n" + r["show"][1])
        print("── 读数（三处 · 每处带口径）")
        print("   T1 `approve ls` 列（rc=%d · 首行 = 表头）：%s" % (rc_ls, " | ".join(cols)))
        print("   T2 `approve show` 行标签（rc=%d）：%s" % (r["show"][0], " | ".join(labels)))
        print("   T3 `approveFields`（%s：var approveFields）= %s" % (APPROVE_GO_REL, ",".join(r["fields"])))
        print("   T4 `--json nosuchfield-xyz` ⇒ rc=%d（要 2）" % r["unknown"][0])
        print("   ·  读数（**不判** · `P26` 未拍）：`--json strength` ⇒ rc=%d" % r["strength"][0])
        print("   T5 词汇面：禁词 %s 命中 = %d · 允许词 %s" % (
            "/".join(FORBIDDEN_TOKENS),
            sum((out_ls + "\n" + r["show"][1]).count(b) for b in FORBIDDEN_TOKENS),
            "/".join(ALLOWED_TOKENS)))
        if rc_ls != 0:
            problems.append("T1 破：`approve ls` 退码 %d（要 0）" % rc_ls)
        if r["show"][0] != 0:
            problems.append("T2 破：`approve show` 退码 %d（要 0）" % r["show"][0])
        if r["unknown"][0] != 2:
            problems.append("T4 破：`--json <未知字段>` 退码 %d（要 2 · 字段面先判）" % r["unknown"][0])
        # T6 `M-20` 现跑：`--strength` 今天必须仍是未知旗标（真状态目录零写）
        sandbox = tempfile.mkdtemp(prefix="zerg-approve-tier-strength-")
        try:
            rc_s, out_s, err_s = run_cli(r["exe"], repo, sandbox, ["approve", "keygen", "--strength", "token", "--by", "夹具"])
            if rc_s is None:
                problems.append("T6 破：取不到 `--strength` 读数（%s）" % r["bin_note"])
            elif rc_s != 2:
                problems.append("T6 破：`approve keygen --strength token` 退码 %d（要 2 · 未知旗标）" % rc_s)
            else:
                print("   T6 `M-20` 现跑：`keygen --strength token` ⇒ rc=2（原样：%s）" % (err_s.strip().splitlines() or [""])[0])
            if sandbox != real and os.path.isdir(os.path.join(sandbox, "approvals")):
                problems.append("T6 破：沙箱外落了件（%s/approvals）" % sandbox)
        finally:
            shutil.rmtree(sandbox, ignore_errors=True)
        if problems:
            print("⇒ 门禁红（rc=1）：%d 条" % len(problems))
            for p in problems:
                print("   ✗ %s" % p)
            return 1
        print("⇒ 全绿（rc=0）：T1–T6 全过 · 漂移 0 条（档位标记的落地形态仍待 `P26` 拍）")
        return 0
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


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
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（**只有已自证过的调用方才该用**）")
    args = ap.parse_args()
    repo = os.path.abspath(args.root)
    print(VERSION)
    if not args.no_self_test:
        if not self_test(repo):      # GAP-20260927-26：self_test 返 bool（`:282`）⇒ 比 None 恒假 ✗
            print("⛔ 自检不过（或取不到读数）⇒ 不给结论（rc=2）")
            return 2
    if args.self_test:
        return 0
    print("── 真目标：%s" % repo)
    return scan(repo)


if __name__ == "__main__":
    sys.exit(main())

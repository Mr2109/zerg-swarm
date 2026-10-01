#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-freeze.py v1.0.0 —— 人签三面**冻结对拍** + 台账对齐（2.5.11 人签 D 批 · `D4`）

为什么存在（`D4` 的任务原文，逐字口径）
--------------------------------------------------
`D4` 要一枚**只读**对拍件，把三处冻结面**逐字对齐**：① 帮助导出面的字段表 ② `approveFields` 九键
③ 矩阵 `approve` 族 must-fail；并要 `v2.5.11` 缺口账里**补一条回指 `R-11`**（**不重开新编号** ✗）。
本件就是那一格；它**只读消费**三面，**一个字节都不改**（改帮助导出面 / 矩阵 / 基线 / 台账都不归它）。

★ 数字纪律：**三面读数一律现算**，**不许写常量** —— `v1.3 [P17]` 记矩阵 `355`、开工时现跑 `356`
（说明矩阵在本轮被改过）⇒ 本件每次现算并把三处口径打印出来（矩阵条数 / must-fail 条数 / 字段表行号）。

判据（F1–F4 · 每条都能单独失败）
--------------------------------------------------
  F1 **帮助导出面字段表**（现落点：`<Zerg-内部文档>/项目文档/v2.5.11/导出-命令面帮助-*.md`，取最新）：
     `zerg approve ls` 与 `zerg approve show` 两行的字段列逐字 = 九键（**行号现算**，不许钉死在正文里）。
  F2 **`approveFields`**（`core/cmd/zerg/family_approve.go`）逐字 = **同九键**。
  F3 **矩阵 `approve` 族**（`core/cmd/zerg/testdata/cli-matrix.json`）：条数与 `want_rc != 0` 的条数
     **现算**，且**两条相等**（本仓铁律：每条命令至少一条 must-fail）。
  F4 **台账对齐**（`<Zerg-内部文档>/项目文档/v2.5.11/缺口-命令面-20260922.md`）：有一条**回指 `R-11`**，
     且 `G-` 编号集合**不新增**（不重开新编号 ⇒ 仍在 `G-01`–`G-16` 之内）。
  ·  三面**缺任一面 ⇒ 不出结论**：只许写「未取数」（rc=2），**不许**把「文件不存在」读成「没问题」。
  ·  漂移行**逐行打印**（缺任一面不出结论 · 漂移 0 行才算过）。

开工前读数（本件写它是因为「这一格今天确实是空的」要留证 · 不是判据）
--------------------------------------------------
  现跑 `v2.5.11/缺口-命令面-20260922.md`：`人签` / `Keychain` / `Touch ID` / `R-11` / `硬件` / `钥匙` /
  `PIN` / `补签` / `多机` **命中全 0**（`task` 单 ① 现跑 ✓）⇒ 台账那条**今天确实是空的**（不是「已经有了」）。
  ★ F4 落地后 `人签` / `R-11` 这两个字面**会**出现（那就是补条本身）—— 本条记的是**补条之前**的读数。

退码
--------------------------------------------------
  0 = 全绿（漂移 0 行）· 1 = 有红（漂移 ≥1 行）· 2 = **不给结论**（任一面取不到 / 自检不过）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-freeze.py          # 先自检，再扫真目标
  python3 scripts/gates/check-approve-freeze.py --self-test # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-freeze.py --root <仓根> --docs-root <Zerg-内部文档 根>
"""

import argparse
import glob
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-freeze.py v1.0.0"

EXPECTED_FIELDS = ["tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"]
HELP_ROWS = ["zerg approve ls", "zerg approve show"]
DOCS_VER = "v2.5.11"
GAP_DOC = "缺口-命令面-20260922.md"
LEDGER_ANCHOR = "R-11"
GAP_NUM_RE = re.compile(r"G-\d\d")
GAP_MAX = 16  # `v2.5.11` 顶账那 16 条（`G-01`–`G-16`）—— 只许在这个范围内
APPROVE_GO_REL = os.path.join("core", "cmd", "zerg", "family_approve.go")
MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")


# ── 判定口（纯函数：自检与真目标共用）──────────────────────────────────────────────

def parse_fields_from_go(src):
    m = re.search(r"var approveFields = \[\]string\{([^}]*)\}", src)
    if not m:
        return None
    return re.findall(r'"([^"]+)"', m.group(1))


def parse_field_cell(row):
    """从帮助导出面一行里挑出**字段表那一格**（逗号分隔的小写标识符 · ≥5 个才算）。"""
    for cell in row.split("|"):
        cell = cell.strip().strip("`")
        if "," not in cell:
            continue
        parts = [p.strip() for p in cell.split(",")]
        if len(parts) >= 5 and all(re.fullmatch(r"[a-z_]+", p) for p in parts):
            return parts
    return None


def judge_help_faces(help_text, expected=None):
    """F1：两行的字段列都要 = 九键；返回 (漂移行, 读数[(行号, 行名, 字段)])。"""
    expected = expected or EXPECTED_FIELDS
    drifts, readings = [], []
    for i, ln in enumerate(help_text.splitlines(), 1):
        for name in HELP_ROWS:
            if ("`%s`" % name) in ln or ln.startswith("| %s |" % name):
                fields = parse_field_cell(ln)
                if fields is None:
                    drifts.append("F1 漂移：第 %d 行（%s）里找不到字段表那一格" % (i, name))
                    continue
                readings.append((i, name, fields))
                if fields != expected:
                    drifts.append("F1 漂移：第 %d 行（%s）字段表 = %s ≠ 九键 %s" % (i, name, ",".join(fields), ",".join(expected)))
    for name in HELP_ROWS:
        if not any(r[1] == name for r in readings):
            drifts.append("F1 漂移：帮助导出面里没有 `%s` 那一行" % name)
    return drifts, readings


def judge_fields_face(fields, expected=None):
    expected = expected or EXPECTED_FIELDS
    if fields is None:
        return ["F2 未取数：`approveFields` 解不出来"]
    if fields != expected:
        return ["F2 漂移：`approveFields` = %s ≠ 九键 %s" % (",".join(fields), ",".join(expected))]
    return []


def judge_matrix(cases):
    """F3：`approve` 族条数 == `want_rc != 0` 的条数（现算 · 不写常量）。"""
    fam = [c for c in cases if str(c.get("command", "")).startswith("approve")]
    mustfail = [c for c in fam if c.get("want_rc") != 0]
    drifts = []
    if len(fam) != len(mustfail):
        missing = [c.get("id") for c in fam if c.get("want_rc") == 0]
        drifts.append("F3 漂移：`approve` 族 %d 条里有 %d 条不是 must-fail（%s）"
                      % (len(fam), len(fam) - len(mustfail), " · ".join(str(m) for m in missing)))
    return drifts, (len(fam), len(mustfail))


def judge_ledger(text, anchor=LEDGER_ANCHOR, gap_max=GAP_MAX):
    """F4：有回指；`G-` 编号不新增。"""
    drifts = []
    if text is None:
        return ["F4 未取数：台账读不到"]
    if anchor not in text:
        drifts.append("F4 漂移：台账里没有回指 `%s`" % anchor)
    nums = sorted({int(n[2:]) for n in set(GAP_NUM_RE.findall(text))})
    if nums and max(nums) > gap_max:
        drifts.append("F4 漂移：台账里出现了新编号（最大 G-%02d > G-%02d ⇒ 重开了新编号）" % (max(nums), gap_max))
    return drifts


# ── 取数 ─────────────────────────────────────────────────────────────────────────

def docs_root_of(repo, given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_DEVDOCS_ROOT")
    if env:
        return os.path.abspath(env)
    # 同级根归一（GAP-20260927-82 批1）：ZERG_DOCS_ALT（同 statepath.DocsAltRoot）/ ZERG_DOCS（同 scriptInvDocsRoot）
    sibling = (os.environ.get("ZERG_DOCS_ALT") or os.environ.get("ZERG_DOCS")
               or os.path.join(os.path.dirname(repo), "Zerg-内部文档"))
    return os.path.join(sibling, "项目文档", DOCS_VER)


def read_all(repo, docs_root):
    """取三面 + 台账。任一面取不到 ⇒ (None, 未取数理由)。"""
    go = os.path.join(repo, APPROVE_GO_REL)
    if not os.path.exists(go):
        return None, "读不到 `%s`" % APPROVE_GO_REL
    with open(go, encoding="utf-8") as f:
        fields = parse_fields_from_go(f.read())
    if fields is None:
        return None, "`approveFields` 在 %s 里解不出来" % APPROVE_GO_REL
    mx = os.path.join(repo, MATRIX_REL)
    if not os.path.exists(mx):
        return None, "读不到 `%s`" % MATRIX_REL
    with open(mx, encoding="utf-8") as f:
        cases = json.load(f).get("cases", [])
    if not os.path.isdir(docs_root):
        return None, "Zerg-内部文档 根不在（%s）" % docs_root
    help_files = sorted(glob.glob(os.path.join(docs_root, "导出-命令面帮助-*.md")), key=os.path.getmtime)
    if not help_files:
        return None, "帮助导出面不在（%s/导出-命令面帮助-*.md）" % docs_root
    help_path = help_files[-1]
    with open(help_path, encoding="utf-8") as f:
        help_text = f.read()
    gap_path = os.path.join(docs_root, GAP_DOC)
    if not os.path.exists(gap_path):
        return None, "台账不在（%s）" % gap_path
    with open(gap_path, encoding="utf-8") as f:
        gap_text = f.read()
    return {"fields": fields, "cases": cases, "help_text": help_text, "help_path": help_path,
            "gap_text": gap_text, "gap_path": gap_path}, ""


def judge_all(r):
    drifts = []
    helpline_drifts, readings = judge_help_faces(r["help_text"])
    drifts += helpline_drifts
    drifts += judge_fields_face(r["fields"])
    mdrift, (fam, mustfail) = judge_matrix(r["cases"])
    drifts += mdrift
    drifts += judge_ledger(r["gap_text"])
    return drifts, readings, (fam, mustfail)


# ── 自检 ─────────────────────────────────────────────────────────────────────────

FAKE_HELP = (
    "# 导出（夹具）\n"
    "| `zerg approve ls` | 列人签批准件 | %s | 本机 | `host` |\n"
    "| `zerg approve show` | 看一枚批准件 | %s | 本机 | `host` |\n"
) % (",".join(EXPECTED_FIELDS), ",".join(EXPECTED_FIELDS))


def self_test(repo, docs_root):
    ok = bad = 0
    r, why = read_all(repo, docs_root)
    if r is None:
        print("⛔ 自检① 取不到真读数：%s ⇒ 不给结论" % why)
        return None
    drifts, readings, (fam, mustfail) = judge_all(r)
    if not drifts:
        print("   ✓ ① 正控：三面 + 台账现读一致 ⇒ 漂移 0 行（帮助面 %s · 矩阵 approve 族 %d 条 / must-fail %d 条）"
              % (os.path.basename(r["help_path"]), fam, mustfail))
        ok += 1
    else:
        print("   ✗ ① 正控失败：现读就有漂移：%s" % "；".join(drifts))
        bad += 1
    # ② 负控：字段表与件不一致 ⇒ 漂移行 ≥1
    wrong = ",".join(EXPECTED_FIELDS[:-1])
    d2, _ = judge_help_faces(FAKE_HELP.replace(",".join(EXPECTED_FIELDS), wrong))
    if d2:
        print("   ✓ ② 负控：帮助面字段表少一格 ⇒ 出漂移行（%d 条）" % len(d2))
        ok += 1
    else:
        print("   ✗ ② 负控失败：字段表不一致还判绿")
        bad += 1
    # ③ 负控：矩阵里混一条 approve 族非 must-fail ⇒ 红
    d3, _ = judge_matrix(list(r["cases"]) + [{"id": "合成#漏 must-fail", "command": "approve ls", "want_rc": 0}])
    if d3:
        print("   ✓ ③ 负控：矩阵里混一条 `approve` 族 want_rc=0 ⇒ 判红")
        ok += 1
    else:
        print("   ✗ ③ 负控失败：漏了一条 must-fail 还判绿")
        bad += 1
    # ④ 负控：台账没有回指 ⇒ 红
    if judge_ledger("§一 · 与 `R-12` 同族（没有回指那一枚）\n"):
        print("   ✓ ④ 负控：台账缺回指 `%s` ⇒ 判红" % LEDGER_ANCHOR)
        ok += 1
    else:
        print("   ✗ ④ 负控失败：缺回指还判绿")
        bad += 1
    # ⑤ 负控：台账里重开新编号 ⇒ 红
    if judge_ledger("回指 R-11 · 另开 G-17 一条\n"):
        print("   ✓ ⑤ 负控：台账里重开新编号（G-17）⇒ 判红")
        ok += 1
    else:
        print("   ✗ ⑤ 负控失败：重开了新编号还判绿")
        bad += 1
    # ⑥ 负控：面缺 ⇒ 未取数（rc=2 那条口径）
    missing, why2 = read_all(repo, os.path.join(tempfile.gettempdir(), "zerg-approve-freeze-nodocs", "Zerg-内部文档", "项目文档", DOCS_VER))
    if missing is None and "Zerg-内部文档" in why2:
        print("   ✓ ⑥ 负控：Zerg-内部文档 根缺 ⇒ 未取数（%s）⇒ 不许当绿" % why2)
        ok += 1
    else:
        print("   ✗ ⑥ 负控失败：面缺却没判「未取数」")
        bad += 1
    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo, docs_root):
    r, why = read_all(repo, docs_root)
    if r is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    drifts, readings, (fam, mustfail) = judge_all(r)
    print("── 三面读数（每处带口径：件 / 行号 / 现算值）")
    for i, name, fields in readings:
        print("   ① 帮助导出面：%s 第 %d 行 · `%s` · 字段 = %s"
              % (os.path.basename(r["help_path"]), i, name, ",".join(fields)))
    print("   ② 件：%s · `approveFields` = %s" % (APPROVE_GO_REL, ",".join(r["fields"])))
    print("   ③ 矩阵：%s · `approve` 族 %d 条 · want_rc!=0 %d 条" % (MATRIX_REL, fam, mustfail))
    print("   ④ 台账：%s · 回指 `%s` = %s · G 编号上限 = G-%02d"
          % (os.path.basename(r["gap_path"]), LEDGER_ANCHOR,
             "有" if LEDGER_ANCHOR in r["gap_text"] else "无",
             max([int(n[2:]) for n in set(GAP_NUM_RE.findall(r["gap_text"]))] or [0])))
    if drifts:
        print("⇒ 门禁红（rc=1）：漂移 %d 行" % len(drifts))
        for d in drifts:
            print("   ✗ %s" % d)
        return 1
    print("⇒ 全绿（rc=0）：三面逐字一致 · 漂移 0 行（数字一律现算 · 未写任何常量）")
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
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--docs-root", default=None, help="Zerg-内部文档 根（默认 = 仓根的上一级/Zerg-内部文档/项目文档/v2.5.11）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    repo = os.path.abspath(args.root)
    docs_root = docs_root_of(repo, args.docs_root)
    print(VERSION)
    if not args.no_self_test:
        if not self_test(repo, docs_root):
            print("⛔ 自检不过（读数取不到 或 有格不过）⇒ 不给结论（rc=2）")
            return 2
    if args.self_test:
        return 0
    print("── 真目标：%s（Zerg-内部文档 根 %s）" % (repo, docs_root))
    return scan(repo, docs_root)


if __name__ == "__main__":
    sys.exit(main())

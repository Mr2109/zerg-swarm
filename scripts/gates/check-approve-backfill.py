#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-backfill.py v1.0.0 —— 「补签件与在场件**可分**」的判据件（2.5.11 人签 E 批 · `E2`）

为什么存在（`E2` 的任务原文 + 已拍的落点）
--------------------------------------------------
`E2` 要一枚判据件，把「补签件」与「在场件」**分别认出来**（正控：带明标的夹具认得出；负控：
**抹掉明标 ⇒ 必须判红**）。落点两选一，**Mr2109 已拍 = B 案**（`任务单-人签实施-20260922.md §附十`）：
**A 案**（件里加一格 `signed_not_in_person`）撞 `v1.3 §4.2`「批准件八字段**一字不加 / 不减**」⇒ 不落；
**B 案**（明标只落**审计侧**）不撞 §4.2 ✓ —— 本件判的就是 **B 案**那半。

★ 本件**只判、不改**：读 `core/cmd/zerg/family_approve.go`（静态面）、读真审计件（只读）、
读真批准件（只读）；**一个字节都不写** ✗（不写审计、不落在册件、不建归档位）。

判据（B1–B5 · 每条都能单独失败）
--------------------------------------------------
  B1 **件字段面不动**（B 案的直接判据）：`approvalTicketFile` 的 JSON 标签仍是**八格**
     `tool/approver/approved_at/scope/note/sig_alg/key_id/sig`，且**不含** `signed_not_in_person`
     / `in_person`（明标**不许**进件）。
  B2 **明标在审计侧**：审计行结构里有 `signed_not_in_person` 这一格，且审计行的字段名**不蹭**
     现网 `edit_audit.jsonl` 那六格（`before_sha256` / `after_sha256` / `before_bytes` /
     `after_bytes` / `approval` / `approver` · `I-6`）。
  B3 **判定口有区分度**：正控（明标 `true` ⇒ 「补签」· `false` ⇒ 「在场」）；**负控**（抹掉这一格
     ⇒ **判红** —— 「没这一格」**不是**「在场」，否则两件在审计里长得一模一样）。
  B4 **现网取数（只读）**：真审计件现跑行数 + `approve_signed` 事件**今天**是 0 条（补签那一路
     本版**不实现**）⇒ 如实写「本版尚无补签件」（**不许**把「读不到」当绿：读不到 ⇒ rc=2 未取数）。
  B5 **在册件零改**：`dev_edit.json` 既不是补签件、也不带明标 ⇒ 它的 `sha256` 与登记值一致
     （明标没进件 = 这一枚件一个字节没动）。

退码
--------------------------------------------------
  0 = 全绿（B1–B5 全过，且自检 6 格全过）· 1 = 有红 · 2 = **不给结论**（取不到读数）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-backfill.py             # 先自检，再扫真目标
  python3 scripts/gates/check-approve-backfill.py --self-test # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-backfill.py --root <仓根> --state <状态目录>
"""

import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-backfill.py v1.0.0"

APPROVE_GO_REL = os.path.join("core", "cmd", "zerg", "family_approve.go")
TICKET_EIGHT = ["tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "sig"]
MARK_FIELD = "signed_not_in_person"
EVENT = "approve_signed"
LEGACY_SIX = ["before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval", "approver"]
DEV_EDIT_SHA256 = "bccec4748c1d54206b751ecaeb6f617497accb7c21b0052a448e87a980ac3f18"


# ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────────

def parse_ticket_json_tags(src):
    """`approvalTicketFile` 结构上的 `json:"..."` 标签（B1 的取数口）。"""
    m = re.search(r"type approvalTicketFile struct \{(.*?)\n\}", src, re.S)
    if not m:
        return None
    return re.findall(r'json:"([^"]+)"', m.group(1))


def judge_ticket_fields(tags):
    """B1：八格**逐字**在、且明标**不在**件里（顺序也照今天那一份 ⇒ 一字不动）。"""
    if tags is None:
        return ["B1 未取数：`approvalTicketFile` 的结构体解不出来"]
    problems = []
    if tags != TICKET_EIGHT:
        problems.append("B1 破：`approvalTicketFile` 标签 = %s ≠ 八格 %s" % (",".join(tags), ",".join(TICKET_EIGHT)))
    if MARK_FIELD in tags:
        problems.append("B1 破：明标 %r 进了**件字段面** —— 那是 A 案（已拍 = B 案 · 不许改件字段面 ✗）" % MARK_FIELD)
    if "in_person" in tags:
        problems.append("B1 破：`in_person` 进了件字段面（同上 · A 案形态）")
    return problems


def parse_audit_tags(src):
    """`approvalAuditLine` 的 `json:"..."` 标签（B2 的取数口）。"""
    m = re.search(r"type approvalAuditLine struct \{(.*?)\n\}", src, re.S)
    if not m:
        return None
    return re.findall(r'json:"([^"]+)"', m.group(1))


def parse_own_fields(src):
    """`approveAuditOwnFields`（本事件**自己的**字段名 · B2 的“新名字”那一份）。"""
    m = re.search(r"var approveAuditOwnFields = \[\]string\{([^}]*)\}", src)
    if not m:
        return None
    return re.findall(r'"([^"]+)"', m.group(1))


def judge_audit_side(tags, own=None):
    """B2：明标在**审计侧** + **新**字段名不蹭现网六格（`approver` 那一格是**沿用**、不是新名）。"""
    if tags is None:
        return ["B2 未取数：`approvalAuditLine` 的结构体解不出来"]
    problems = []
    if MARK_FIELD not in tags:
        problems.append("B2 破：审计行里**没有**明标 %r（B 案的明标就该落在这里）" % MARK_FIELD)
    for bad in ("before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval"):
        if bad in tags:
            problems.append("B2 破：审计行蹭了现网那一格 %r（`I-6`：新概念要**自己的字段名**）" % bad)
    for t in (own or []):
        if t in LEGACY_SIX:
            problems.append("B2 破：**新**字段名 %r 与现网六格同名（`I-6`）" % t)
    return problems


def mark_of(raw_line):
    """判定口：一行审计原始 JSON ⇒ ("在场"|"补签", None) / (None, 理由)。"""
    try:
        row = json.loads(raw_line)
    except Exception as e:  # noqa: BLE001 —— 解不动就照实报，不猜
        return None, "审计行解不动：%s" % e
    if MARK_FIELD not in row:
        return None, "这一行**没有明标**（%r 不在）⇒ 不可分：当场判红" % MARK_FIELD
    v = row[MARK_FIELD]
    if not isinstance(v, bool):
        return None, "明标不是布尔（%r）⇒ 判红" % (v,)
    return ("补签" if v else "在场"), None


def judge_mark_lines(lines):
    """B3：一批审计行 —— 返回 (问题行, 读数[(行号, 判定)])。"""
    problems, readings = [], []
    for i, ln in enumerate(lines, 1):
        if not ln.strip():
            continue
        verdict, err = mark_of(ln)
        if err:
            problems.append("B3 破（第 %d 行）：%s" % (i, err))
            continue
        readings.append((i, verdict))
    if not readings and not problems:
        problems.append("B3 破：一行都没判到（空夹具 ⇒ 不许当「全在场」）")
    return problems, readings


def judge_event_count(rows):
    """B4：现网审计行数 + `approve_signed` 事件条数（现算，不许写常量）。"""
    n = len([r for r in rows if str(r).strip()])
    ev = 0
    for r in rows:
        if not str(r).strip():
            continue
        try:
            if json.loads(r).get("event") == EVENT:
                ev += 1
        except Exception:  # noqa: BLE001
            pass
    return n, ev


# ── 取数 ─────────────────────────────────────────────────────────────────────────

def state_dir_of(given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_STATE_DIR")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def read_all(repo, state):
    go = os.path.join(repo, APPROVE_GO_REL)
    if not os.path.exists(go):
        return None, "读不到 `%s`" % APPROVE_GO_REL
    with open(go, encoding="utf-8") as f:
        src = f.read()
    ticket_tags = parse_ticket_json_tags(src)
    if ticket_tags is None:
        return None, "`approvalTicketFile` 在 %s 里解不出来" % APPROVE_GO_REL
    own_fields = parse_own_fields(src)
    if own_fields is None:
        return None, "`approveAuditOwnFields` 在 %s 里解不出来（`E1` 自己的字段名那一份不在）" % APPROVE_GO_REL
    audit_tags = parse_audit_tags(src)
    if audit_tags is None:
        return None, "`approvalAuditLine` 在 %s 里解不出来（`E1` 的审计行结构不在 ⇒ 明标无处落）" % APPROVE_GO_REL
    audit = os.path.join(state, "edit_audit.jsonl")
    if not os.path.isdir(state):
        return None, "状态目录不在（%s）" % state
    rows = []
    if os.path.exists(audit):
        with open(audit, encoding="utf-8") as f:
            rows = f.read().splitlines()
    else:
        return None, "审计件不在（%s）⇒ 未取数（**不许**把「文件不存在」当没问题）" % audit
    ticket = os.path.join(state, "approvals", "dev_edit.json")
    if not os.path.exists(ticket):
        return None, "在册那枚批准件不在（%s）⇒ 未取数" % ticket
    import hashlib
    with open(ticket, "rb") as f:
        dev_sha = hashlib.sha256(f.read()).hexdigest()
    return {"src": src, "ticket_tags": ticket_tags, "audit_tags": audit_tags, "own_fields": own_fields, "rows": rows,
            "state": state, "audit_path": audit, "ticket": ticket, "dev_sha": dev_sha}, ""


def judge_all(r):
    problems = []
    problems += judge_ticket_fields(r["ticket_tags"])
    problems += judge_audit_side(r["audit_tags"], r["own_fields"])
    # B3 的**真目标**那一半：判定口在**当前源码上**必须有区分度 ——
    # 正控两行（在场 / 补签）要过，**负控**（抹掉明标）要**判红**。
    problems += judge_mark_lines([json.dumps({"event": EVENT, MARK_FIELD: False}),
                                  json.dumps({"event": EVENT, MARK_FIELD: True})])[0]
    if not judge_mark_lines([json.dumps({"event": EVENT})])[0]:
        problems.append("B3 破：抹掉明标的那一行**没判红**（判定口没有区分度）")
    n, ev = judge_event_count(r["rows"])
    if ev > 0:
        problems.append("B4 注意：现网审计里已有 %d 条 `%s`（本版补签那一路**不实现** ⇒ 该是 0 条）" % (ev, EVENT))
    if r["dev_sha"] != DEV_EDIT_SHA256:
        problems.append("B5 破：在册 `dev_edit.json` 的 sha256 = %s ≠ 登记值（明标没进件 = 它一个字节不该动）" % r["dev_sha"])
    return problems, (n, ev)


# ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

def self_test(repo, state):
    ok = bad = 0

    def check(cond, msg_ok, msg_bad):
        nonlocal ok, bad
        if cond:
            print("   ✓ %s" % msg_ok)
            ok += 1
        else:
            print("   ✗ %s" % msg_bad)
            bad += 1

    check(not judge_ticket_fields(TICKET_EIGHT), "① 正控：件八格**逐字**在 ⇒ 不判红（B1）",
          "① 正控失败：八格齐全还被判红")
    check(bool(judge_ticket_fields(TICKET_EIGHT + [MARK_FIELD])),
          "② 负控：明标进了**件字段面** ⇒ 判红（A 案形态 · 已拍 B 案）",
          "② 负控失败：明标进件还判绿")
    check(not judge_audit_side(["at", "event", "tool", MARK_FIELD, "approval_path"], ["approval_path"]),
          "③ 正控：明标在**审计侧**且不蹭六格 ⇒ 不判红（B2）",
          "③ 正控失败：B 案形态被判红")
    check(bool(judge_audit_side(["at", "event", "before_sha256", MARK_FIELD], ["approval_path"])),
          "④ 负控：审计行蹭了 `before_sha256` ⇒ 判红（`I-6`）",
          "④ 负控失败：蹭了现网那格还判绿")
    v_in, e_in = mark_of(json.dumps({MARK_FIELD: False}))
    v_back, e_back = mark_of(json.dumps({MARK_FIELD: True}))
    check(v_in == "在场" and e_in is None and v_back == "补签" and e_back is None,
          "⑤ 正控：明标 `false` ⇒ 在场 · `true` ⇒ 补签（有区分度）",
          "⑤ 正控失败：两种明标判不出区别（%s/%s）" % (v_in, v_back))
    v_no, e_no = mark_of(json.dumps({"event": EVENT}))
    check(v_no is None and e_no is not None, "⑥ 负控：**抹掉明标** ⇒ 判红（不可分不许蒙过去）",
          "⑥ 负控失败：没明标还判绿")
    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo, state):
    r, why = read_all(repo, state)
    if r is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    problems, (n, ev) = judge_all(r)
    print("── 读数（每处带口径）")
    print("   ① 件字段面（%s · `approvalTicketFile`）= %s" % (APPROVE_GO_REL, ",".join(r["ticket_tags"])))
    print("   ② 审计行字段名（同件 · `approvalAuditLine`）= %s" % ",".join(r["audit_tags"]))
    print("   ②-补 本事件**自己的**字段名（%s · `approveAuditOwnFields`）= %s" % (APPROVE_GO_REL, ",".join(r["own_fields"])))
    print("   ③ 审计件（%s）现跑 %d 行 · `%s` 事件 %d 条" % (r["audit_path"], n, EVENT, ev))
    print("   ④ 在册件（%s）sha256 = %s" % (r["ticket"], r["dev_sha"]))
    if ev == 0:
        print("   ·  口径：本版**尚无补签件**（补签那一路本版不实现）—— 明标只在**审计侧**记「不是当场签的」")
    if problems:
        print("⇒ 门禁红（rc=1）：%d 条" % len(problems))
        for p in problems:
            print("   ✗ %s" % p)
        return 1
    print("⇒ 全绿（rc=0）：件字段面八格不动 · 明标在审计侧 · 判定口有区分度 · 在册件零改")
    return 0


def main():
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--state", default=None, help="状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    repo = os.path.abspath(args.root)
    state = state_dir_of(args.state)
    print(VERSION)
    if not args.no_self_test and not self_test(repo, state):
        return 1
    if args.self_test:
        return 0
    print("── 真目标：%s（状态目录 %s）" % (repo, state))
    return scan(repo, state)


if __name__ == "__main__":
    sys.exit(main())

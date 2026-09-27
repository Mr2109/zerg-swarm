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
  B4 **现网取数（只读）**：真审计件现跑行数 + `approve_signed` 事件条数 + 其中**补签**条数
     （判 `signed_not_in_person=true`；换件判据：在场签名也追加 `approve_signed` ⇒ 按字面数会误伤 ·
     2026-09-23 改宽）；补签那一路本版**不实现** ⇒ 补签该是 0 条（**不许**把「读不到」当绿：读不到 ⇒ rc=2 未取数）。
  B5 **在册件零改**：`dev_edit.json` 既不是补签件、也不带明标 ⇒ 它的 `sha256` 与登记值一致
     （明标没进件 = 这一枚件一个字节没动）。

退码
--------------------------------------------------
  0 = 全绿（B1–B5 全过，且自检全过）· 1 = 有红 · 2 = **不给结论**（取不到读数）

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
# ★ 本常量 = **批准件重签后**的现读值（2026-09-28 02:16:25 Mr2109 重签 `dev_edit.json`：全量 scope + 追加 deploy/，
#   key_id `8b69ad95939df5cf`；审计行在 `<状态目录>/edit_audit.jsonl`）。**任何时候重签了该件，须同一批把本常量
#   跟到新值** —— 否则门⑰ 会以「B5 破」恒红（今晚即如此）。
DEV_EDIT_SHA256 = "afb8dc93d571dc6ae1abf4844ac0f54206e6556d4f5fb89f229426f77c9bccbe"


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
    """B4：现网审计行数 + `approve_signed` 事件条数 + 其中**补签**条数（现算，不许写常量）。

    2026-09-23 改宽（Mr2109「按建议」）：补签与否看**明标那一格** `signed_not_in_person`，
    不按事件名字面数（正常签名也追加 `approve_signed` ⇒ 旧判据会把在场签名误判成补签）。
    区分度更强、不误伤：`true` ⇒ 补签（本版不实现 ⇒ 该是 0）· `false`/缺 ⇒ 在场。
    """
    n = len([r for r in rows if str(r).strip()])
    ev = 0
    bf = 0
    for r in rows:
        if not str(r).strip():
            continue
        try:
            d = json.loads(r)
            if d.get("event") == EVENT:
                ev += 1
                if d.get(MARK_FIELD) is True:
                    bf += 1
        except Exception:  # noqa: BLE001
            pass
    return n, ev, bf


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
    n, ev, bf = judge_event_count(r["rows"])
    if bf > 0:
        problems.append("B4 破：现网审计里有 %d 条**补签**事件（`%s=true`）—— 本版补签那一路**不实现** ⇒ 该是 0 条（在场签名含 `%s` 事件属正常，不判）" % (bf, MARK_FIELD, EVENT))
    if r["dev_sha"] != DEV_EDIT_SHA256:
        problems.append("B5 破：在册 `dev_edit.json` 的 sha256 = %s ≠ 登记值（明标没进件 = 它一个字节不该动）" % r["dev_sha"])
    return problems, (n, ev, bf)


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
    # ── ★ 回归自测格（缺口 `GAP-20260928-217` · 2026-09-28 补）：**真判据面**不许哑化 ──
    #   病灶（今天已治，本格是牙齿）：本自测此前只喂**判定口零件**（`judge_ticket_fields` /
    #   `judge_audit_side` / `mark_of` 三个纯函数 + 常量 `TICKET_EIGHT`），**真判据面
    #   `read_all()` / `parse_*()` / `judge_event_count()` / `judge_all()` / `scan()`
    #   一个都不进自检** ⇒ 把 `scan()` 体首行后插一行 `return []` 哑化后 `--self-test`
    #   仍 `rc=0`「6 格过」（假安全；2026-09-28 **仓外硬拷贝**实测复现：哑化后
    #   `py_compile` rc=0、自测 rc=0）。同族体例照拄 `check-public-tree-hazards.py`
    #   的两格（`scan-paths-catches-hazard` / `scan-forms-catches-hazard`）。
    #   治法：拿一份**按现读判据面形状造的合成夹具**（仓外 `tempfile.mkdtemp`：假
    #   `family_approve.go` + 假 `edit_audit.jsonl` + 假在册件）喂给**真判据面** `scan()`，
    #   断言它**真判红并逐字点名** —— 谁把 `scan()` / `judge_all()` / `judge_event_count()` /
    #   `parse_*()` / `read_all()` 哑化成空结果，本格立刻变红。
    #   夹具建不起来（临时目录不可写）⇒ **红**：缺语料不许静默绿（下列格全红）。
    import contextlib as _ctx
    import io as _io
    import shutil as _shutil
    import tempfile as _tempfile

    _FIELDS_GO = "".join('\t%s string `json:"%s"`\n' % (_t, _t) for _t in TICKET_EIGHT)
    _SIG_GO = '\tsig string `json:"sig"`\n'
    _MARK_GO = '\tzzMARKzz bool `json:"zzMARKzz"`\n'.replace("zzMARKzz", MARK_FIELD)
    _AUDIT_GO = ('type approvalAuditLine struct {\n'
                 '\tat string `json:"at"`\n'
                 '\tevent string `json:"event"`\n'
                 '\ttool string `json:"tool"`\n'
                 '\tapproval_path string `json:"approval_path"`\n'
                 '\tzzMARKzz bool `json:"zzMARKzz"`\n'
                 '}\n').replace("zzMARKzz", MARK_FIELD)
    _OWN_GO = ('var approveAuditOwnFields = []string{"at", "event", "tool", "approval_path", "zzMARKzz"}\n'
               ).replace("zzMARKzz", MARK_FIELD)
    _GO_OK = ("package main\n\n"
              "type approvalTicketFile struct {\n" + _FIELDS_GO + "}\n\n" + _AUDIT_GO + "\n" + _OWN_GO)
    # ⓐ 正控夹具一（A 案形态）：把明标**塞进件字段面** —— B1 该判红的那一处
    _GO_BAD = _GO_OK.replace(_SIG_GO, _SIG_GO + _MARK_GO)
    _ROW_IN = json.dumps({"event": EVENT, MARK_FIELD: False})
    _ROW_BACK = json.dumps({"event": EVENT, MARK_FIELD: True})
    _fx_root = _tempfile.mkdtemp(prefix="backfill-selftest-scan-")
    _hz_ticket = ("夹具建不起来", -99)      # ⓐ 正控①：明标进件（B1）
    _hz_row = ("夹具建不起来", -99)         # ⓐ 正控②：台帐里一条补签（B4）
    _clean = ("夹具建不起来", -99)          # ⓒ 反向对照：干净夹具
    _noaudit = ("夹具建不起来", -99)        # 静默绿守卫：审计件缺失

    def _mk_fixture(_name, _go_src, _rows):
        _root = os.path.join(_fx_root, _name)
        _repo = os.path.join(_root, "repo")
        _state = os.path.join(_root, "state")
        os.makedirs(os.path.join(_repo, os.path.dirname(APPROVE_GO_REL)), exist_ok=True)
        os.makedirs(os.path.join(_state, "approvals"), exist_ok=True)
        with open(os.path.join(_repo, APPROVE_GO_REL), "w", encoding="utf-8") as _fh:
            _fh.write(_go_src)
        if _rows is not None:
            with open(os.path.join(_state, "edit_audit.jsonl"), "w", encoding="utf-8") as _fh:
                _fh.write("".join(_r + "\n" for _r in _rows))
        with open(os.path.join(_state, "approvals", "dev_edit.json"), "w", encoding="utf-8") as _fh:
            _fh.write("{}\n")
        return _repo, _state

    def _run_fixture(_repo, _state):
        _buf = _io.StringIO()
        with _ctx.redirect_stdout(_buf):
            _rc = scan(_repo, _state)
        return _buf.getvalue(), _rc

    try:
        # 干净面：八格逐字 + 明标只在审计侧 + 台帐一行在场
        _fx_clean = _mk_fixture("clean", _GO_OK, [_ROW_IN])
        # ⓐ 正控①：件字段面被植入明标（A 案形态）
        _fx_ticket = _mk_fixture("ticket-mark", _GO_BAD, [_ROW_IN])
        # ⓐ 正控②：台帐里多一条**补签**行（`signed_not_in_person=true`）
        _fx_row = _mk_fixture("audit-backfill", _GO_OK, [_ROW_IN, _ROW_BACK])
        # 静默绿守卫：审计件不在（同一夹具只差这一枚件）
        _fx_noaudit = _mk_fixture("no-audit", _GO_OK, None)
        _clean = _run_fixture(*_fx_clean)
        _hz_ticket = _run_fixture(*_fx_ticket)
        _hz_row = _run_fixture(*_fx_row)
        _noaudit = _run_fixture(*_fx_noaudit)
    except OSError as _exc:
        print("   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿（下列格全红）" % ("backfill-fixture", _exc))

    def _cell(_label, _cond, _why):
        check(_cond, "%s | %s" % (_label, _why), "%s | %s" % (_label, _why))

    # ⓪ 先证夹具**真喂到了**真判据面（否则下列格是空转）
    _cell("scan-fixture-reaches-judge",
          _clean[1] == 1 and "B5 破" in _clean[0] and "件字段面" in _clean[0],
          "真判据面 `scan()` 在合成夹具上取到数并走到判据（现读 rc=%s · 读到 B5 破=%s · 读到件字段面=%s）"
          % (_clean[1], "B5 破" in _clean[0], "件字段面" in _clean[0]))
    # ⓐ 正控①：明标进件 ⇒ 真判据面 rc=1 **且逐字点名** `B1 破`
    _cell("scan-catches-mark-in-ticket",
          _hz_ticket[1] == 1 and "B1 破" in _hz_ticket[0],
          "`scan()` 对「明标进件」真判红并逐字点名 `B1 破`（现读 rc=%s · 点名=%s）"
          % (_hz_ticket[1], "B1 破" in _hz_ticket[0]))
    # ⓐ 正控②：台帐里一条补签 ⇒ rc=1 且逐字点名 `B4 破` + 条数
    _cell("scan-catches-backfill-row",
          _hz_row[1] == 1 and "B4 破" in _hz_row[0] and "1 条" in _hz_row[0],
          "`scan()` 对台帐里的补签行真判红并点名条数（现读 rc=%s · 点名 `B4 破`=%s · 1 条=%s）"
          % (_hz_row[1], "B4 破" in _hz_row[0], "1 条" in _hz_row[0]))
    # ⓒ 反向对照：干净夹具上植入面**一处都不许命中**（⇒ 本组格不是恒红）
    _cell("scan-clean-has-no-induced-hit",
          _clean[1] == 1 and not any(_k in _clean[0] for _k in ("B1 破", "B3 破", "B4 破")),
          "干净夹具上植入面零命中（唯一红 = 合成在册件 sha 无前像的 B5 · 现读 rc=%s · 命中=%s）"
          % (_clean[1], [_k for _k in ("B1 破", "B3 破", "B4 破") if _k in _clean[0]]))
    # ⓑ 反控（哑化模型）：判据面被换成**空结果** ⇒「有问题的」与「干净的」**两态同形** ⇒ 本格必红
    _cell("scan-face-not-blind",
          not (("B1 破" in _hz_ticket[0]) == ("B1 破" in _clean[0])
               and ("B4 破" in _hz_row[0]) == ("B4 破" in _clean[0])),
          "判据面哑化（空结果 ⇒ 两态同形）时本格判红 —— 现读两态已分开（正控命中 %s/%s · 对照命中 %s/%s）"
          % ("B1 破" in _hz_ticket[0], "B4 破" in _hz_row[0],
             "B1 破" in _clean[0], "B4 破" in _clean[0]))
    # 静默绿守卫：审计件缺失 ⇒ rc=2（哑化 `scan()` / `read_all()` ⇒ 出 0/空 ⇒ 本格必红）
    _cell("scan-refuses-without-corpus",
          _noaudit[1] == 2 and "未取数" in _noaudit[0],
          "审计件缺失时 rc=2 且写明「未取数」（现读 rc=%s · 未取数=%s）"
          % (_noaudit[1], "未取数" in _noaudit[0]))
    _shutil.rmtree(_fx_root, ignore_errors=True)

    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo, state):
    r, why = read_all(repo, state)
    if r is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    problems, (n, ev, bf) = judge_all(r)
    print("── 读数（每处带口径）")
    print("   ① 件字段面（%s · `approvalTicketFile`）= %s" % (APPROVE_GO_REL, ",".join(r["ticket_tags"])))
    print("   ② 审计行字段名（同件 · `approvalAuditLine`）= %s" % ",".join(r["audit_tags"]))
    print("   ②-补 本事件**自己的**字段名（%s · `approveAuditOwnFields`）= %s" % (APPROVE_GO_REL, ",".join(r["own_fields"])))
    print("   ③ 审计件（%s）现跑 %d 行 · `%s` 事件 %d 条（其中**补签** %d 条）" % (r["audit_path"], n, EVENT, ev, bf))
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

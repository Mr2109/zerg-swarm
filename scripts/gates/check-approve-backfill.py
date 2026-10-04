#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-backfill.py v1.1.0 —— 「补签件与在场件**可分**」的判据件（2.5.11 人签 E 批 · `E2`）

为什么存在（`E2` 的任务原文 + 已拍的落点）
--------------------------------------------------
`E2` 要一枚判据件，把「补签件」与「在场件」**分别认出来**（正控：带明标的夹具认得出；负控：
**抹掉明标 ⇒ 必须判红**）。落点两选一，**Mr2109 已拍 = B 案**（`任务单-人签实施-20260922.md §附十`）：
**A 案**（件里加一格 `signed_not_in_person`）撞 `v1.3 §4.2`「批准件八字段**一字不加 / 不减**」⇒ 不落；
**B 案**（明标只落**审计侧**）不撞 §4.2 ✓ —— 本件判的就是 **B 案**那半。

★ 本件**只判、不改**：读 `core/cmd/zerg/family_approve.go`（静态面）、读真审计件（只读）、
读真批准件（只读）；**一个字节都不写** ✗（不写审计、不落在册件、不建归档位）。
`v1.1.0`（缺口 `GAP-20260928-88`）给**本件自己**（不是给那些只读目标）加了一条**正门刷新通道**：
常量 `DEV_EDIT_SHA256` 的值由 `--refresh-constant [--yes]` 从**真源件**现读刷进来，不再靠人肉改件。
通道的**写面只动那一个常量的字面量**（最小文本替换 + 先落仓外备份），**读判据一个字没放松**。

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
     （明标没进件 = 这一枚件一个字节没动）。**红行必须印出应有值**（缺口 `GAP-20260928-88`：
     只说「≠ 登记值」不说该是多少 ⇒ 只能人肉发现 + 手改门件）。

常量刷新通道（缺口 `GAP-20260928-88` · 判据 B5 的**配套**，不是它的替代）
--------------------------------------------------
  `--emit-constant`      **只读**：把**现读应有的**常量值打到 stdout（真源件现算 sha256 + 前缀解释 +
                         现成的那一行字面量）。**不写件**（rc：0 = 与件内一致 · 1 = 不一致 · 2 = 未取数）。
  `--refresh-constant`   **默认干跑不写**：只印「现值 → 应有值 + 计划的最小替换（含行号）」。
  `--refresh-constant --yes`  真写：**最小文本替换**（只换常量行的那 64 位字面量，其余逐字节不动 ·
                          替换前后**字节数相等**）＋ **先落仓外备份**（默认 `~/.zerg/state/gate-backups/`，
                          落仓内 ⇒ 拒写 rc=2）。写前先过自检（不过 ⇒ 拒写 rc=1）。
  ★ 两者都**不放宽任何判据**：B5 仍逐字比 sha256，常量错了仍 rc=1 —— 刷新通道只是把「人肉改件」
  换成「正门刷值」。

退码
--------------------------------------------------
  0 = 全绿（B1–B5 全过，且自检全过）· 1 = 有红 · 2 = **不给结论**（取不到读数）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-backfill.py             # 先自检，再扫真目标
  python3 scripts/gates/check-approve-backfill.py --self-test # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-backfill.py --root <仓根> --state <状态目录>
  python3 scripts/gates/check-approve-backfill.py --emit-constant            # 只读：应有值是多少
  python3 scripts/gates/check-approve-backfill.py --refresh-constant         # 干跑：印计划 · 不写
  python3 scripts/gates/check-approve-backfill.py --refresh-constant --yes   # 真写（最小替换 + 仓外备份）
"""

import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-backfill.py v1.1.0"

APPROVE_GO_REL = os.path.join("core", "cmd", "zerg", "family_approve.go")
TICKET_EIGHT = ["tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "sig"]
MARK_FIELD = "signed_not_in_person"
EVENT = "approve_signed"
LEGACY_SIX = ["before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval", "approver"]
# ★ 本常量 = **批准件重签后**的现读值（2026-09-28 02:16:25 Mr2109 重签 `dev_edit.json`：全量 scope + 追加 deploy/，
#   key_id `8b69ad95939df5cf`；审计行在 `<状态目录>/edit_audit.jsonl`）。**任何时候重签了该件，走本件自己的正门
#   刷新通道把它跟到新值**（先 `--emit-constant` 看应有值，再 `--refresh-constant` 干跑，最后带 `--yes` 落）——
#   不然门⑰ 会以「B5 破」恒红；**禁**手改本行（那是缺口 `GAP-20260928-88` 记的那条坏路）。
DEV_EDIT_SHA256 = "c75b84ee3c1eb4bdf24d93ea46543d24abd0b775bf2ba8a46a543d309ac6ce17"

# ── 常量刷新的取数口（emit / refresh / 自检同读这一处 · 单一真源）────────────────────
CONST_NAME = "DEV_EDIT_SHA256"
CONST_RE = re.compile(r'(?m)^(' + CONST_NAME + r' = ")([0-9a-f]{64})(")$')
TICKET_REL = os.path.join("approvals", "dev_edit.json")


def sha256_file(path):
    import hashlib
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def find_constant(src):
    """返回 `(现读字面量, 行号, (start, end))`；解不出来 ⇒ `(None, None, None)`。"""
    m = CONST_RE.search(src)
    if not m:
        return None, None, None
    return m.group(2), src[:m.start()].count("\n") + 1, (m.start(2), m.end(2))


def default_backup_dir():
    return os.path.join(os.path.expanduser("~"), ".zerg", "state", "gate-backups")


def _inside(path, root):
    path, root = os.path.abspath(path), os.path.abspath(root)
    return path == root or path.startswith(root + os.sep)


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


# ── 常量刷新通道（只读 emit + 最小替换写入 · 缺口 `GAP-20260928-88`）──────────────────

def emit_constant(gate_path, dev_sha, out=None):
    """只读档：把**现读应有的**常量值打到 stdout —— **不写件**。

    rc：0 = 件内常量 == 应有值 · 1 = **不一致**（=门⑰ 现读会 B5 破 · 该走 `--refresh-constant`）·
        2 = 未取数（常量行解不出来 / 真源件读不到）。
    """
    out = sys.stdout if out is None else out
    out.write("── 常量发射档（只读 · 一个字节都不写）\n")
    out.write("   常量       : %s（件内 = %s）\n" % (CONST_NAME, gate_path))
    out.write("   口径       : 在册批准件的**整件文件** sha256（含末尾换行）—— 人做一次合法重签\n")
    out.write("                （`zerg approve new`）该值必变 ⇒ 冻在件里就会陈腐（缺口 GAP-20260928-88）\n")
    if dev_sha is None:
        out.write("⛔ 未取数：真源件读不到 ⇒ **不出结论**（rc=2）\n")
        return 2
    out.write("   真源件     : %s\n" % TICKET_REL)
    out.write("   应有值     : %s\n" % dev_sha)
    try:
        with open(gate_path, encoding="utf-8") as f:
            src = f.read()
    except OSError as e:
        out.write("⛔ 未取数：件读不到（%s）⇒ **不出结论**（rc=2）\n" % e)
        return 2
    cur, line, _ = find_constant(src)
    if cur is None:
        out.write("⛔ 未取数：常量行 `%s = \"…\"` 在件里解不出来 ⇒ **不出结论**（rc=2）\n" % CONST_NAME)
        return 2
    out.write("   件内现值   : %s（第 %d 行）\n" % (cur, line))
    if cur == dev_sha:
        out.write("   判定       : 一致 ⇒ 门⑰ 的 B5 该过\n")
        out.write("   %s = \"%s\"\n" % (CONST_NAME, cur))
        return 0
    out.write("   判定       : **不一致** ⇒ 门⑰ 现读 B5 破 · 该刷成下面这一行\n")
    out.write("   %s = \"%s\"\n" % (CONST_NAME, dev_sha))
    out.write("   正门       : python3 scripts/gates/check-approve-backfill.py"
              " --refresh-constant --yes（先干跑不带 `--yes` · 只读再看一遍用 `--emit-constant`）\n")
    return 1


def refresh_constant(gate_path, dev_sha, apply=False, backup_dir=None, repo_root=None, out=None):
    """常量刷新档：**默认干跑不写**；`apply=True`（=命令行 `--yes`）才落。

    写面 = **最小文本替换**：只换 `CONST_RE` 命中的那 64 位字面量，其余**逐字节不动**（替换前后
    字节数相等 —— 等长替换）。写前先把改前整件**备份到仓外**（默认 `~/.zerg/state/gate-backups/`；
    备份落点在仓内 ⇒ 拒写 rc=2）。
    rc：0 = 落成 / 干跑印完 / 现值已等于应有值 · 2 = 未取数或落点非法（拒写）。
    """
    out = sys.stdout if out is None else out
    if dev_sha is None:
        out.write("⛔ 未取数：真源件读不到 ⇒ 拒写（rc=2）\n")
        return 2
    try:
        with open(gate_path, "rb") as f:
            raw = f.read()
    except OSError as e:
        out.write("⛔ 未取数：件读不到（%s）⇒ 拒写（rc=2）\n" % e)
        return 2
    src = raw.decode("utf-8")
    cur, line, span = find_constant(src)
    if cur is None:
        out.write("⛔ 未取数：常量行 `%s = \"…\"` 解不出来 ⇒ 拒写（rc=2）\n" % CONST_NAME)
        return 2
    out.write("── 常量刷新档（%s）\n" % ("**干跑** · 未写件" if not apply else "**真写**"))
    out.write("   目标件 : %s\n" % gate_path)
    out.write("   替换点 : 第 %d 行 · 最小文本替换（只动那 64 位字面量 · 其余逐字节不动）\n" % line)
    out.write("   现值   : %s\n" % cur)
    out.write("   应有值 : %s\n" % dev_sha)
    if cur == dev_sha:
        out.write("   ⇒ 现值已 == 应有值 ⇒ 无需动笔（rc=0）\n")
        return 0
    if not apply:
        out.write("   ⇒ **干跑未写**：加 `--yes` 才落（本门默认不写件）\n")
        return 0
    bdir = os.path.abspath(backup_dir or default_backup_dir())
    if repo_root and _inside(bdir, repo_root):
        out.write("⛔ 拒写：备份落点在**仓内**（%s ⊂ %s）—— 备份必须落仓外（rc=2）\n" % (bdir, repo_root))
        return 2
    os.makedirs(bdir, exist_ok=True)
    import time as _time
    bpath = os.path.join(bdir, os.path.basename(gate_path) + "." + _time.strftime("%Y%m%d-%H%M%S") + ".bak")
    with open(bpath, "wb") as f:
        f.write(raw)
    new_raw = (src[:span[0]] + dev_sha + src[span[1]:]).encode("utf-8")
    with open(gate_path, "wb") as f:
        f.write(new_raw)
    out.write("   备份   : %s（仓外 · 改前整件 %d B）\n" % (bpath, len(raw)))
    out.write("   字节   : 前 %d B → 后 %d B（等长替换 ⇒ 除那一个字面量外一字未动）\n" % (len(raw), len(new_raw)))
    out.write("   ⇒ 已落（rc=0）：复跑门⑰ 单步该转绿（自检格数不变）\n")
    return 0


# ── 取数 ─────────────────────────────────────────────────────────────────────────

def state_dir_of(given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_STATE_DIR")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def warn_if_sandbox_state(given, stream):
    """★ 缺口 `GAP-20260927-70`：本门此前对环境变量 `ZERG_STATE_DIR` **静默取用** —— 当别会话把它
    留在环境里指向沙箱目录时，本门读的是**沙箱**状态目录、退码被计成红（头号假红源）。
    照同目录 `check-trustroot-witness.py` 的**同形**警示：`--state` 未显式给而环境变量赢 ⇒ 逐字声明
    这是沙箱读数、不是真状态目录 `~/.zerg/state`。（写 `stderr`：本门 `--emit-constant` 的 stdout 是
    机器面，不许掺诊断行。）"""
    if given is None and os.environ.get("ZERG_STATE_DIR"):
        print("★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"
              "**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）"
              % os.environ["ZERG_STATE_DIR"], file=stream)


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
    ticket = os.path.join(state, TICKET_REL)
    if not os.path.exists(ticket):
        return None, "在册那枚批准件不在（%s）⇒ 未取数" % ticket
    dev_sha = sha256_file(ticket)
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
    # ★ 缺口 `GAP-20260928-88`：红行**必须印出应有值**（旧版只说「≠ 登记值」⇒ 只能人肉发现 + 手改门件）。
    #   判据**一字不放宽**：仍逐字比 sha256；打印只是把「该改成多少」一并给出（正门 = `--refresh-constant`）。
    if r["dev_sha"] != DEV_EDIT_SHA256:
        problems.append("B5 破：在册 `dev_edit.json` 的 sha256 = %s ≠ 登记值（明标没进件 = 它一个字节不该动）"
                        "· **应改为** %s = \"%s\" —— 正门刷值：`python3 scripts/gates/check-approve-backfill.py"
                        " --refresh-constant`（先干跑）· 加 `--yes` 才落（只读看值：`--emit-constant`）"
                        % (r["dev_sha"], CONST_NAME, r["dev_sha"]))
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

    # ── ★ 缺口 `GAP-20260928-88` 的自证格（2026-09-30 补）：刷新通道**两态**都要有牙 ──
    #   牙齿对象：谁把 `--emit-constant` 弄成写件 / 把 `--refresh-constant` 弄成默认就写 /
    #   把最小替换弄成整件重写 / 把备份落在仓内，本组格立刻变红。
    _fx2 = _tempfile.mkdtemp(prefix="backfill-const-selftest-")
    _gate_fx = os.path.join(_fx2, "check-approve-backfill.py")
    _old_sha = "0" * 64
    _new_sha = "1" * 64
    _gate_src = ("#!/usr/bin/env python3\n# 注释行（非 ASCII · 锚定逐字节不动）\n"
                 '%s = "%s"\nprint("tail")\n' % (CONST_NAME, _old_sha))
    with open(_gate_fx, "w", encoding="utf-8") as _fh:
        _fh.write(_gate_src)
    _before = open(_gate_fx, "rb").read()

    def _cap(_fn, *a, **kw):
        _buf = _io.StringIO()
        with _ctx.redirect_stdout(_buf):
            _rc = _fn(*a, **kw)
        return _buf.getvalue(), _rc

    _emit_out, _rc_emit = _cap(emit_constant, _gate_fx, _new_sha)
    check(_rc_emit == 1 and _new_sha in _emit_out and _old_sha in _emit_out
          and open(_gate_fx, "rb").read() == _before,
          "⑦ 正控（`--emit-constant`）：**只读不写件**且印出**应有值**（现读 rc=%s · 应有值在=%s · 件未动=%s）"
          % (_rc_emit, _new_sha in _emit_out, open(_gate_fx, "rb").read() == _before),
          "⑦ 正控失败：emit 档没印出应有值或**动了件**（rc=%s · 应有值在=%s · 件未动=%s）"
          % (_rc_emit, _new_sha in _emit_out, open(_gate_fx, "rb").read() == _before))
    _dry_bdir = os.path.join(_fx2, "bak-dry")
    _dry_out, _rc_dry = _cap(refresh_constant, _gate_fx, _new_sha, False, _dry_bdir, None)
    check(_rc_dry == 0 and open(_gate_fx, "rb").read() == _before and not os.path.exists(_dry_bdir)
          and "干跑" in _dry_out,
          "⑧ 负控（干跑默认不写）：不带 `--yes` ⇒ **零写入**（rc=%s · 件未动=%s · 备份目录未建=%s）"
          % (_rc_dry, open(_gate_fx, "rb").read() == _before, not os.path.exists(_dry_bdir)),
          "⑧ 负控失败：干跑档**写了件**或建了备份（rc=%s · 件未动=%s）"
          % (_rc_dry, open(_gate_fx, "rb").read() == _before))
    _bdir = os.path.join(_fx2, "bak")
    _wr_out, _rc_wr = _cap(refresh_constant, _gate_fx, _new_sha, True, _bdir, os.path.join(_fx2, "repo"))
    _after = open(_gate_fx, "rb").read()
    _baks = os.listdir(_bdir) if os.path.isdir(_bdir) else []
    _bak_path = os.path.join(_bdir, _baks[0]) if _baks else ""
    # 最小替换的**逐字节**判据：改后 == 改前把那一处 64 位字面量原样换成新值（其余字节逐字不动）
    _bsrc = _gate_src.encode("utf-8")
    _bi = _bsrc.index(_old_sha.encode("utf-8"))
    _expect = _bsrc[:_bi] + _new_sha.encode("utf-8") + _bsrc[_bi + len(_old_sha):]
    check(_rc_wr == 0 and _after == _expect and len(_after) == len(_before)
          and len(_baks) == 1 and open(_bak_path, "rb").read() == _before,
          "⑨ 正控（`--refresh-constant --yes`）：最小替换（等长 %d→%d B · 常量外逐字节不动 · 字节级==期望）"
          "+ 仓外备份 1 份=改前态" % (len(_before), len(_after)),
          "⑨ 正控失败：替换不是最小 / 未落备份（rc=%s · 字节级相等=%s · 等长=%s · 备份数=%s）"
          % (_rc_wr, _after == _expect, len(_after) == len(_before), len(_baks)))
    _in_repo_out, _rc_in_repo = _cap(refresh_constant, _gate_fx, _old_sha, True,
                                     os.path.join(_fx2, "repo", "bak-in-repo"), os.path.join(_fx2, "repo"))
    check(_rc_in_repo == 2 and "仓内" in _in_repo_out,
          "⑩ 负控（备份落仓内 ⇒ 拒写 rc=2 并逐字点名）：现读 rc=%s" % _rc_in_repo,
          "⑩ 负控失败：备份落仓内还写了（rc=%s）" % _rc_in_repo)
    _shutil.rmtree(_fx2, ignore_errors=True)

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
    ap.add_argument("--emit-constant", action="store_true",
                    help="只读档：把**现读应有的**常量值打到 stdout（含真源件现算 sha256 + 前缀解释 · 不写件）")
    ap.add_argument("--refresh-constant", action="store_true",
                    help="刷新常量：**默认干跑不写**；带 `--yes` 才落（最小文本替换 + 先落仓外备份）")
    ap.add_argument("--yes", action="store_true", help="与 `--refresh-constant` 连用 = 真写（干跑默认不写）")
    ap.add_argument("--backup-dir", default=None,
                    help="刷新时的备份落点（**写档必须显式给** · 落仓内 ⇒ 拒写）")
    ap.add_argument("--gate", default=None,
                    help="刷新/取数的目标件（**写档必须显式给**；缺省仅用于只读档 = 本件自身）")
    args = ap.parse_args()
    repo = os.path.abspath(args.root)
    state = state_dir_of(args.state)
    warn_if_sandbox_state(args.state, sys.stderr)
    gate_path = os.path.abspath(args.gate) if args.gate else os.path.abspath(__file__)
    ticket = os.path.join(state, TICKET_REL)
    dev_sha = sha256_file(ticket) if os.path.exists(ticket) else None

    if args.emit_constant and args.refresh_constant:
        print("⛔ 用法错（rc=2）：`--emit-constant` 与 `--refresh-constant` 互斥（一个只读、一个可能写）", file=sys.stderr)
        return 2
    if args.yes and not args.refresh_constant:
        print("⛔ 用法错（rc=2）：`--yes` 只跟 `--refresh-constant` 连用（本门**默认干跑不写**）", file=sys.stderr)
        return 2
    if args.emit_constant:
        print(VERSION)
        return emit_constant(gate_path, dev_sha)
    if args.refresh_constant:
        print(VERSION)
        # ● GAP-20261001-56 修：写面通道须**显式且默认关闭** —— 目标件与备份目录都须为显式入参，
        #   否则判据件可**自我改写**（`--gate` 缺省曾指向脚本自身），且备份会落进真实状态目录。
        if args.yes and not args.gate:
            print("⛔ 拒写（rc=2）：`--refresh-constant --yes` 必须显式给 `--gate <件>`（禁默认自我改写）", file=sys.stderr)
            return 2
        if args.yes and not args.backup_dir:
            print("⛔ 拒写（rc=2）：`--refresh-constant --yes` 必须显式给 `--backup-dir <仓外目录>`（禁默认落状态目录）", file=sys.stderr)
            return 2
        if not args.no_self_test and not self_test(repo, state):
            print("⛔ 自检不过 ⇒ **拒写**（rc=1）：先修自检再谈刷值")
            return 1
        if dev_sha is None:
            print("⛔ 未取数：在册批准件不在（%s）⇒ **拒写**（rc=2）" % ticket)
            return 2
        return refresh_constant(gate_path, dev_sha, apply=args.yes, backup_dir=args.backup_dir, repo_root=repo)
    print(VERSION)
    if not args.no_self_test and not self_test(repo, state):
        return 1
    if args.self_test:
        return 0
    print("── 真目标：%s（状态目录 %s）" % (repo, state))
    return scan(repo, state)


if __name__ == "__main__":
    sys.exit(main())

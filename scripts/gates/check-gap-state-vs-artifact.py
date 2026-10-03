#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gap-state-vs-artifact.py —— 缺口真源「**态 ↔ 证物**」四态只读门（缺口 `GAP-20260927-134`）

题面（缺口账 `GAP-20260927-134` 的 `want_action` 逐字）
------------------------------------------------------
「建只读门 check-gap-state-vs-artifact.py（四态）：
   R1 已解条必须带「命令 + rc + 读数指纹」（缺则**存量只报、新增判红**）；
   R2 仍缺条点名件在盘/已挂门 ⇒ **只出提醒、不改 state**；
   R3 notes 末条「仍缺」⟷ solved_evidence「已解」⇒ **rc=1**；
   R4 真源 sha/行数指纹，两次读数被写过 ⇒ **rc=2 不给结论**。
 不误报机理：**只判结构+在盘/在册、不判语义「有效」**（真源自己写死「存在≠有效」）；
 判不了即点名不许当绿；报与拦分离；先把件名/命令升级为结构化字段再谈接退码。」

判据（R 编号逐条给判红/报告的理由）
----------------------------------
  **R1 已解证据（存量只报 · 新增判红）**
    `已解` 行的 `solved_evidence` 必须同时含三样**结构**标记：
      ① **命令**（反引号里的 `zerg …` / `.py`|`.sh`|`.go` 路径 / `cmd=` / `go test` 之一）
      ② **真退码**（`rc=` / `退码` / `returns` / 裸 `rc N` 之一）
      ③ **读数**（数字 token 一枚 —— 计数 / 耗时 / 指纹片段）
    · 行 `found_at` **早于** `--cutoff`（缺省 = 本条缺口自己的立案时刻 `2026-09-27T04:30:45`）⇒
      **存量**，缺项**只报告**（历史不追改 ✗）。
    · **不早于** cutoff ⇒ **新增**，缺任一项 ⇒ **判红**。
  **R2 态 ↔ 落点件（只报告 · 不改 state ✗）**
    `仍缺` / `已派` / `已立项` 行的 `unit`（件面）是**仓内路径形态**时，现读它是否**在盘**、
    是否被 `precommit-gates.sh` **点名挂门**。命中 ⇒ 进 reports（「点名件已在盘 / 已挂门」）——
    ★ **只提醒、绝不改 state、绝不判红**（「件在盘」≠「缺口已闭」，这条正是本门要防的误读）。
  **R3 同条自相矛盾（判红 rc=1）**
    `state` ∈ {已解, 不做}（闭态）而**可见 notes 的末条**逐字含「仍缺」⇒ 同一条记录自相矛盾 ⇒ 判红
    并点名。★ 作废注（`notes_void`）不计入「末条」（读面按 `family_gap.go` 同口径过滤）。
  **R4 真源指纹（不给结论 rc=2）**
    现读真源**两次**（sha256 + 行数）。两次不一致 ⇒ 期间被别的会话写过 ⇒ **rc=2 不给结论** ✗
    （不许把「数在动」当稳定读数 · 与本账 GAP-20260927-134 的「计数不可复现」逐字同源）。
    另 `--expect-sha16` / `--expect-lines` 给了现值就与现读比，不符 ⇒ rc=2（点名两值）。
  **R5 销案证据点名仓内路径必须存在（存量只报 · 新增判红）**
    `solved_evidence`（销案证据）正文里点名的**仓内相对路径**必须**现读在盘**；不在盘 ⇒ 违例。
    · 「仓内相对路径」形态 = 含 `/` · 非 URL（无 `://`、不以 `http` 起）· 非绝对（不以 `/` 起）
      · 无 `...`（Go 模式串）· **首段是仓根现有顶层条目**（⇒ 属本仓，排掉兄弟仓/仓外路径）。
      （例：`core/cmd/zerg/chat_tool_overview.go` 判；`docs-gate/x.py`〔文仓〕、`~/…`、`/etc/…` 不判。）
    · ★ **语境门槛**：只有出现在**真路径主张**语境里的路径才判 —— A) 被反引号包住（`core/x.go`）；
      B) 紧跟在指令性前缀之后（见常量 `PATH_CLAIM_PREFIXES`）。纯散文里提一嘴的路径名
      （组件名 / 目录名 / 产物名 / 干跑示意件名）**不判**。
    · 一条证据点名多串 ⇒ 逐个判，违例串逐条点名（计数 + 每个串 + 所属 gap id）。
    · **存量违例只报告、不进 rc**（历史不追改 ✗）；**新账 / 新增违例 ⇒ 判红**：
        新账 = 号内日期（`GAP-YYYYMMDD-…`）≥ `2026-09-28`；新增违例 = 该条 `found_at >= --cutoff`
        （后者与 R1 的存量/新增划界**同一判据**）。
    · ★ 降级 ≠ 隐身：存量违例也必须**逐条点名**（违例数 + 每个串 + id），只是不计入 rc。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = R1 新增条证据齐 + R3 无自相矛盾 + R4 两次读数一致 + R5 无「新增」违例
  1 = 判红：R1 有新增条缺证据 / R3 有自相矛盾条 / R5 有新增违例
  2 = **不给结论**：真源读不到 / 某行不是 JSON / R4 两次读数不一致 / `--expect-*` 不符 / 仓根不是目录 / 自检不过
      —— 一律显式打印（「读不到」不许当绿 ✗）

用法
----
    python3 scripts/gates/check-gap-state-vs-artifact.py
    python3 scripts/gates/check-gap-state-vs-artifact.py --json
    python3 scripts/gates/check-gap-state-vs-artifact.py --source <jsonl>   # 换真源（缺省 = <状态目录>/zerg-cli-gaps.jsonl）
    python3 scripts/gates/check-gap-state-vs-artifact.py --cutoff '2026-09-27T04:30:45'
    python3 scripts/gates/check-gap-state-vs-artifact.py --list-rules
    python3 scripts/gates/check-gap-state-vs-artifact.py --self-test

★ 本件**纯只读**：不碰真源（只读）· 不碰任何落地件（只 `stat`）· 不挂 `precommit-gates.sh`（登记面属收口批）
  · 不写任何文件 · 判不了即点名、绝不替真源改态 ✗。
"""

import argparse
import hashlib
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-gap-state-vs-artifact.py v1.0.0"

SOURCE_NAME = "zerg-cli-gaps.jsonl"
STATE_DIR_DEFAULT = os.path.join(os.path.expanduser("~"), ".zerg", "state")
STEP_TABLE = os.path.join(REPO_ROOT, "scripts", "gates", "precommit-gates.sh")

OPEN_STATES = ("仍缺", "已派", "已立项", "回归")
CLOSED_STATES = ("已解", "不做")
# 本条缺口（GAP-20260927-134）自己的立案时刻 ⇒ 之前 = 存量、之后 = 新增。
CUTOFF_DEFAULT = "2026-09-27T04:30:45"

# ── R1 结构标记（只判「有没有」，不判「有效不有效」）────────────────────────────
CMD_PATS = (
    re.compile(r"`\s*zerg\s"),            # 反引号里的 zerg 命令
    re.compile(r"\bzerg\s+[a-z]"),         # 裸 zerg 子命令
    re.compile(r"[\w./-]+\.(?:py|sh|go)\b"),  # 脚本/程序路径
    re.compile(r"\bgo\s+test\b"),
    re.compile(r"\bcmd\s*="),
)
RC_PATS = (
    re.compile(r"\brc\s*[=:＝]\s*-?\d+"),
    re.compile(r"退码\s*[=:：]?\s*[-0-9]"),
    re.compile(r"\breturns?\s+\d+"),
    re.compile(r"\brc\s+-?\d+\b"),
)
READ_PATS = (
    re.compile(r"[\d一二三四五六七八九十百千]\s*(?:条|件|行|处|次|个|s|秒|ms|%)"),
    re.compile(r"sha(?:256)?[\s:：=]*[0-9a-f]{6,}"),
    re.compile(r"\b\d{2,}\b"),
)
# 可见 notes 末条里出现「仍缺」即判同条自相矛盾（state 为闭态时）。
CONTRADICT_RE = re.compile(r"仍缺")

# ── R5 销案证据点名仓内路径必须存在（存量只报 · 新增判红）──────────────────────
# 「仓内相对路径」形态 = 含 `/` · 非 URL · 非绝对 · 无 `...` · 首段属仓根顶层条目。
# 左边界 `(?<![\w./~-])` 防把更长路径的后缀（如 `~/.zerg/x`、`/Users/…/x`）当成独立仓内路径。
EV_PATH_RE = re.compile(r"(?<![\w./~-])([A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)+)")
# 新账号内日期分界：GAP-YYYYMMDD-… 的日期 ≥ 此值 ⇒ 新账（新增 ⇒ 判红）。
NEW_ID_DATE = "2026-09-28"
_ID_DATE_RE = re.compile(r"GAP-(\d{4})(\d{2})(\d{2})-")

# ── R5 语境门槛（只有「真路径主张」才判 · 纯散文提及不判）───────────
# 真路径主张 = A) 路径被反引号包住（`core/x.go`，显式码段 ⇒ 断为字面路径）；
#              B) 路径紧跟在指令性前缀之后（`--path ` / `cmd=` ⇒ 断为「此地真发指令」）。
# ★ 裸 `--file ` 故意不入默认集：真仓 GAP-20260927-262 的干跑示例
#   （--file …/zzz_laneB71_probe.go）正是「非主张」形态，`--file ` 与之同形会误咬。
#   收窄/放开只需改这一个常量（门槛参数）。
PATH_CLAIM_PREFIXES = ("--path ", "cmd=")


def repo_top_level(repo_root):
    """仓根现有顶层条目集合（去掉 `.git`）—— 判「首批属本仓」用。取不到 ⇒ 空集（R5 不判）。"""
    try:
        return {e for e in os.listdir(repo_root) if e != ".git"}
    except OSError:
        return set()


def _backtick_spans(ev):
    """证据正文里的反引号码段区间（半开 [a, b)，指两枚反引号之间）。"""
    spans, i = [], 0
    while True:
        a = ev.find("`", i)
        if a < 0:
            break
        b = ev.find("`", a + 1)
        if b < 0:
            break
        spans.append((a + 1, b))
        i = b + 1
    return spans


def path_in_claim_context(ev, m):
    """R5 语境门槛：该路径是否出现在「真路径主张」语境里。True ⇒ 判；False ⇒ 隐（散文提及）。
    A) 落在反引号码段内；B) 紧跟在 PATH_CLAIM_PREFIXES 之一之后。"""
    for a, b in _backtick_spans(ev):
        if a <= m.start() and m.end() <= b:
            return True
    return ev[:m.start()].endswith(PATH_CLAIM_PREFIXES)


def evidence_path_tokens(ev, top):
    """从销案证据正文抽「真路径主张」里的仓内相对路径串（去重保序）。
    ★ 语境门槛：只有真路径主张才抽 —— 反引号包住 或 紧跟指令性前缀；纯散文提及不抽。"""
    out = []
    for m in EV_PATH_RE.finditer(ev or ""):
        t = m.group(1)
        if "://" in t or t.startswith(("http", "/")):   # URL / 绝对路径 ⇒ 非仓内相对路径
            continue
        if "..." in t:                                   # Go 包模式串 `./...`
            continue
        if t.split("/", 1)[0] not in top:                # 首段不属仓根顶层 ⇒ 兄弟仓 / 仓外
            continue
        if not path_in_claim_context(ev, m):             # ★ 语境门槛：非真路径主张 ⇒ 不抽
            continue
        if t not in out:
            out.append(t)
    return out


def row_is_new(row, cutoff):
    """存量/新增划界（与 R1 同口径 + 号内日期分界）：新增 ⇒ True。"""
    found = str(row.get("found_at", ""))
    if (not found) or (found >= cutoff):                 # 与 R1 逐字同判据
        return True
    m = _ID_DATE_RE.search(str(row.get("id", "")))
    if m and ("%s-%s-%s" % (m.group(1), m.group(2), m.group(3))) >= NEW_ID_DATE:
        return True
    return False


def default_source():
    sd = os.environ.get("ZERG_STATE_DIR") or STATE_DIR_DEFAULT
    return os.path.join(sd, SOURCE_NAME)


def read_raw(path):
    try:
        with open(path, "rb") as f:
            return f.read(), None
    except OSError as e:
        return None, "真源读不到：%s" % e


def parse_rows(raw):
    """逐行成 dict。某行不是 JSON ⇒ (None, 说明)。"""
    rows = []
    for i, ln in enumerate(raw.decode("utf-8", "replace").split("\n"), 1):
        if not ln.strip():
            continue
        try:
            rows.append(json.loads(ln))
        except ValueError as e:
            return None, "真源第 %d 行不是 JSON（不给结论）：%s" % (i, e)
    return rows, None


def fingerprint(raw):
    return hashlib.sha256(raw).hexdigest(), len([l for l in raw.decode("utf-8", "replace").split("\n") if l.strip()])


def any_match(pats, text):
    return any(p.search(text) for p in pats)


def evidence_missing(ev):
    """返回缺项的项名清单（命令 / 退码 / 读数）—— 全有 ⇒ []。"""
    ev = ev or ""
    miss = []
    if not any_match(CMD_PATS, ev):
        miss.append("命令")
    if not any_match(RC_PATS, ev):
        miss.append("退码")
    if not any_match(READ_PATS, ev):
        miss.append("读数")
    return miss


def visible_notes(row):
    """过滤作废注（`notes_void.n` 指向的 notes 下标 1 起）。"""
    notes = list(row.get("notes") or [])
    voided = {v.get("n") for v in (row.get("notes_void") or []) if isinstance(v, dict)}
    return [n for i, n in enumerate(notes, 1) if i not in voided]


def unit_is_path(unit):
    u = (unit or "").strip()
    if not u or "无件" in u or u.startswith("拟("):
        return False
    return ("/" in u) or u.endswith((".py", ".sh", ".go", ".md", ".json", ".tsv"))


def judge(rows, repo_root, cutoff):
    """跑 R1/R2/R3/R5，返回 (rc, reds, reports)。R4 由调用方在 judge 之前另判。"""
    reds, reports = [], []
    step_table = os.path.join(repo_root, "scripts", "gates", "precommit-gates.sh")
    top = repo_top_level(repo_root)
    for r in rows:
        rid, st = str(r.get("id", "?")), r.get("state", "")
        # R1
        if st == "已解":
            miss = evidence_missing(r.get("solved_evidence"))
            if miss:
                found = str(r.get("found_at", ""))
                new = (not found) or (found >= cutoff)
                msg = "%s【已解】证据缺 %s" % (rid, "/".join(miss))
                if new:
                    reds.append({"rule": "R1", "what": msg + "（新增 ⇒ 判红）", "read": {"found_at": found}})
                else:
                    reports.append({"rule": "R1", "what": msg + "（存量 ⇒ 只报告）", "read": {"found_at": found}})
        # R2
        if st in ("仍缺", "已派", "已立项") and unit_is_path(r.get("unit")):
            unit = r["unit"].strip()
            abs_p = os.path.join(repo_root, unit)
            on_disk = os.path.exists(abs_p)
            wired = False
            try:
                with open(step_table, "r", encoding="utf-8", errors="replace") as f:
                    wired = os.path.basename(unit) in f.read()
            except OSError:
                wired = False
            if on_disk or wired:
                reports.append({"rule": "R2",
                                "what": "%s【%s】点名件 %s" % (rid, st, unit),
                                "read": {"on_disk": on_disk, "wired": wired}})
        # R3
        if st in CLOSED_STATES:
            vn = visible_notes(r)
            if vn and CONTRADICT_RE.search(vn[-1]):
                reds.append({"rule": "R3", "what": "%s【%s】可见 notes 末条含「仍缺」⇒ 同条自相矛盾" % (rid, st),
                             "read": {"tail": vn[-1][-120:]}})
        # R5 销案证据点名的仓内路径必须现读在盘（存量只报 · 新增判红）
        ev = r.get("solved_evidence")
        if ev:
            miss = [t for t in evidence_path_tokens(ev, top)
                    if not os.path.exists(os.path.join(repo_root, t))]
            if miss:
                what = ("%s 销案证据点名仓内路径 %d 条不在盘：%s"
                        % (rid, len(miss), " · ".join(miss)))
                read = {"paths": miss, "found_at": str(r.get("found_at", ""))}
                if row_is_new(r, cutoff):
                    reds.append({"rule": "R5", "what": what + "（新增 ⇒ 判红）", "read": read})
                else:
                    reports.append({"rule": "R5", "what": what + "（存量 ⇒ 只报告 · 不进 rc）", "read": read})
    return (1 if reds else 0), reds, reports


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True,
                                 description="缺口真源「态 ↔ 证物」四态只读门（GAP-20260927-134）")
    ap.add_argument("--source", default=None, help="真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）")
    ap.add_argument("--repo-root", default=REPO_ROOT, help="仓根（R2 判「在盘」用 · 默认 = 本脚本所在仓）")
    ap.add_argument("--cutoff", default=CUTOFF_DEFAULT, help="新增/存量分界时刻（R1）")
    ap.add_argument("--expect-sha16", default=None, help="期望真源 sha256 前 16 位（R4 · 给了就比）")
    ap.add_argument("--expect-lines", type=int, default=None, help="期望真源行数（R4 · 给了就比）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true", help="打登记面：判据 / 闭集 / 分界")
    ap.add_argument("--self-test", action="store_true", help="合成夹具自检（不碰真源）")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        print("登记面（机器可读真源）：")
        print("  真源件名 : %s（状态目录缺省 %s）" % (SOURCE_NAME, STATE_DIR_DEFAULT))
        print("  开态闭集 : %s" % " / ".join(OPEN_STATES))
        print("  闭态闭集 : %s" % " / ".join(CLOSED_STATES))
        print("  新增分界 : found_at >= %s ⇒ 新增（缺证据判红）" % a.cutoff)
        print("  R1 三标记: 命令(CMD_PATS %d 条) · 退码(RC_PATS %d 条) · 读数(READ_PATS %d 条)"
              % (len(CMD_PATS), len(RC_PATS), len(READ_PATS)))
        print("  R2 只报告: 开态条 unit 是仓内路径形态 ⇒ 判「在盘 / 已挂门」并提醒，**不改 state**")
        print("  R3 判红  : 闭态条可见 notes 末条含「仍缺」⇒ rc=1")
        print("  R4 不给结论: 真源两次读指纹不一致 / --expect-* 不符 ⇒ rc=2")
        print("  R5 判红  : solved_evidence 点名的仓内相对路径必须现读在盘；不在盘且为「新增」⇒ rc=1")
        print("  R5 形态  : 含 `/` · 非 URL · 非绝对 · 无 `...` · 首段属仓根现有顶层条目（⇒ 属本仓）")
        print("  R5 语境  : 只有「真路径主张」才判 —— 反引号包住 或 紧跟指令性前缀 %s；纯散文提及不判"
              % (", ".join(p.strip() for p in PATH_CLAIM_PREFIXES),))
        print("  R5 划界  : found_at >= %s ∨ 号内日期 >= %s ⇒ 新增（其余存量只报告 · 不进 rc）"
              % (a.cutoff, NEW_ID_DATE))
        print("  退码     : 0 过 · 1 判红 · 2 不给结论")
        return 0

    path = os.path.abspath(a.source or default_source())
    raw1, err = read_raw(path)
    if err:
        return emit_problem(a, "R0", err, {"path": path}, path)
    fp1, ln1 = fingerprint(raw1)
    raw2, err2 = read_raw(path)
    if err2:
        return emit_problem(a, "R0", err2, {"path": path}, path)
    fp2, ln2 = fingerprint(raw2)
    if (fp1, ln1) != (fp2, ln2):
        return emit_problem(a, "R4", "真源在两次读数之间被写过（%s/%d ⇒ %s/%d）⇒ 不给结论"
                            % (fp1[:12], ln1, fp2[:12], ln2), {}, path)
    if a.expect_sha16 and fp1[:16] != a.expect_sha16:
        return emit_problem(a, "R4", "真源 sha16 不符：期望 %s · 现读 %s" % (a.expect_sha16, fp1[:16]), {}, path)
    if a.expect_lines is not None and ln1 != a.expect_lines:
        return emit_problem(a, "R4", "真源行数不符：期望 %d · 现读 %d" % (a.expect_lines, ln1), {}, path)
    rows, perr = parse_rows(raw1)
    if perr:
        return emit_problem(a, "R4", perr, {}, path)

    repo_root = os.path.abspath(a.repo_root)
    if not os.path.isdir(repo_root):
        # R2/R5 都靠仓根判「在盘」；根不在 ⇒ 判不了 ⇒ 不给结论（不许当绿 ✗）
        return emit_problem(a, "R0", "仓根不是目录（R2/R5 判不了 · 不给结论）：%s" % repo_root,
                            {"repo_root": repo_root}, path)
    rc, reds, reports = judge(rows, repo_root, a.cutoff)
    meta = {"source": path, "repo_root": repo_root, "sha16": fp1[:16], "lines": ln1, "rows": len(rows)}
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GapStateVsArtifact", "tool": VERSION,
                          "items": [{"rule": "R1", "state": "red" if any(r["rule"] == "R1" for r in reds) else "ok"},
                                    {"rule": "R2", "state": "report"},
                                    {"rule": "R3", "state": "red" if any(r["rule"] == "R3" for r in reds) else "ok"},
                                    {"rule": "R4", "state": "ok"},
                                    {"rule": "R5", "state": "red" if any(r["rule"] == "R5" for r in reds)
                                     else ("report" if any(r["rule"] == "R5" for r in reports) else "ok")}],
                          "meta": meta, "reds": reds, "problems": [],
                          "reports": reports, "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    print("── 缺口真源「态 ↔ 证物」判据门 R1–R5（%s）──" % VERSION)
    print("  真源 : %s（sha16 %s · %d 行）" % (path, fp1[:16], ln1))
    print("  仓根 : %s" % repo_root)
    print("  分界 : found_at >= %s ∨ 号内日期 >= %s ⇒ 新增" % (a.cutoff, NEW_ID_DATE))
    print("  R1  : %s" % ("判红 %d 条" % sum(1 for r in reds if r["rule"] == "R1")
                          if any(r["rule"] == "R1" for r in reds) else "过"))
    print("  R2  : 只报告 %d 条（点名件在盘/已挂门 · 不改 state）" % sum(1 for r in reports if r["rule"] == "R2"))
    print("  R3  : %s" % ("判红 %d 条" % sum(1 for r in reds if r["rule"] == "R3")
                          if any(r["rule"] == "R3" for r in reds) else "过"))
    print("  R4  : 过（两次读 %s/%d 一致）" % (fp1[:12], ln1))
    r5_reds = [r for r in reds if r["rule"] == "R5"]
    r5_reps = [r for r in reports if r["rule"] == "R5"]
    print("  R5  : %s｜销案证据点名仓内路径违例：新增 %d 条（判红）· 存量 %d 条（只报告）"
          % ("判红 %d 条" % len(r5_reds) if r5_reds else "过", len(r5_reds), len(r5_reps)))
    for r in reds:
        print("  ✗ [%s] %s" % (r["rule"], r["what"]))
    # ★ 降级 ≠ 隐身：R5 存量违例逐条点名（不计入 rc，但一条不省）
    if r5_reps:
        print("  ─ R5 存量违例（只报告 · 不进 rc · 逐条点名）%d 条 ─" % len(r5_reps))
        for r in r5_reps:
            print("     · [R5] %s" % r["what"])
    other_rep = [r for r in reports if r["rule"] != "R5"]
    for r in other_rep[:20]:
        print("  · [%s] %s" % (r["rule"], r["what"]))
    if len(other_rep) > 20:
        print("  …（R2 等只报告共 %d 条，余 %d 条略）" % (len(other_rep), len(other_rep) - 20))
    print("%s rc=%d" % ("✓" if rc == 0 else "✗", rc))
    return rc


def emit_problem(a, rule, what, read, path):
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GapStateVsArtifact", "tool": VERSION,
                          "items": [{"rule": rule, "state": "problem"}],
                          "meta": {"source": path}, "reds": [],
                          "problems": [{"rule": rule, "what": what, "read": read}],
                          "reports": [], "warnings": [], "truncated": False}, ensure_ascii=False))
    else:
        print("⛔ 不给结论（rc=2）：%s" % what)
    return 2


# ═════════════════════════════════════ 自检 ════════════════════════════════════════════

# ── R5 ④ 回归夹具：真仓 4 条散文误伤件的销案证据逐字（语境门槛下必须不判）──────
R5_REGRESSION_EVIDENCES = {
    'GAP-20260927-208':
        '父代理据 L35（core/agent 桶路）现读销案：该件 MaxWorkers 已改指针 + 0=显式暂停语义 + 配置 Mr2109:0 均已落地 ⇒ 已修未销，现销（命门面：zerg gap ls）',
    'GAP-20260927-262':
        '销案两态对拍 · 改前{cmd=zerg dev edit --proposal DEV-0485 --file core/cmd/zerg/family_dev_edit.go --from before.go --dry-run --json result · rc=0 · reading=schema 命中 0 条, warnings 命中 0 条（干跑整段不发包封）} · 改后{cmd=zerg dev edit --proposal DEV-9001 --file core/cmd/zerg/zzz_laneB71_probe.go --from newcontent.go --dry-run --json result · rc=0 · reading={"schema":"zerg/v1","kind":"DevEdit","items":[{"result":"planned"}],"meta":{...},"warnings":["影响面摘要：**新增件**..."],"truncated":false}} · 散文=laneB71 2026-09-29：262 已修未销（症状早被更晚提交修掉，账仍旧态）。现读 family_dev_edit.go 干跑档末尾 emitDevEditPlan 已补 --json 出口（selectJSON→emitSelected→emitEnvelopeWith(inv)，读 inv.envWarn/envMeta），:405-419 把真事进 warnings[]。两态：改前（仓外 overlay 复原版 zerg-before）干跑 --json result 输出 schema/warnings 各 0 命中（信封整段未发）；改后（正门 go build -overlay 自建 zerg-src）同跑出六键包封，真事跑 warnings 逐字非空。改动仅在 ~/.hermes/cache/scratch/laneB71-devedit/ 仓外副本。',
    'GAP-20261003-10':
        '销案两态对拍 · 改前{cmd=zerg gate run --step 门① 覆盖：构建清单目录 + 脚本接线（阻断） --only-step · rc=1 · reading=FAIL｜本笔加新件 lib-sign.sh 后，门① ④ 格逐字点名「新件未挂门（不在 git 索引 ⇒ 新建后没与代码同批挂 add_step）」} · 改后{cmd=zerg gate run --step 门① 覆盖：构建清单目录 + 脚本接线（阻断） --only-step · rc=0 · reading=PASS｜④ 新件未挂门 = 0（lib-sign.sh 走第二档白名单：source 型公共库·无独立入口·挂 add_step=跑空壳假绿）· ③ 62 − 白名单 7 = 棘轮 55 ≤ baseline 56 · 计数自证 121+10+62 = 193 ✓} · 散文=want 逐条达成：①『若留』⇒ 三件一个没删（两枚旧件按 GAP-20261003-11 归档进 bin/_history/）；②『给一条正式产出支』⇒ build-all.sh:441-448 两条支（源 agent/cmd/zerg-agentd，与其余件同一对签名函数）；③『纳入签名』⇒ :500-501 入 SIGN_LIST + sign_one 挂 identifier；④『若废』那一支未走（用户拍板=留）。★ 末句本轮做完：把 pack-release.sh 真产的件补进签名盯防面 ⇒ 抽 scripts/build/lib-sign.sh 公共件（两档 source 同一份；铁律：两份实现就会漂）+ pack-release.sh 在算 sha256 之前对 release/ 逐件签名（顺序硬约束：签名改字节 ⇒ 先签后算，否则 manifest/checksums 记签名前的值、末尾校验判红）+ sign_one identifier 表补四段契约名通配（原表只列两条 ⇒ zerg-core-linux-amd64 等会掉进兜底 com.zerg.全名 ⇒ 同组件两套身份）。单验：仓外假 release/ 五件逐件真签，并用真 codesign -dv 独立复读 identifier 全对（core/cli/agent/ui），ELF 形态 size=157/158/159。提交 b34fbf9e → 33b6b4b4 → 0051d395 → 29c972b5。★ 发布档签字的真跑判据仍无 CLI 档（zerg build release 真跑未开放）⇒ 已并入 GAP-20261003-12 口径。',
    'GAP-20261003-11':
        "销案两态对拍 · 改前{cmd=zerg gate run --step 'release: B7b 跨架构三件签名（P4 先报告）（只报告）' --only-step · rc=0 · reading=PASS 1/FAIL 0｜名单=b34fbf9e 那版三件（bin/zerg-agent-darwin-amd64 · bin/zerg-agent-linux-amd64 · bin/zerg-agentd-linux-amd64）—— ★ 那绿是「签了」不是「能产」，本账要说的正是这个假绿} · 改后{cmd=zerg gate run --step 'release: B7b 跨架构三件签名（P4 先报告）（只报告）' --only-step · rc=0 · reading=PASS 1/FAIL 0｜名单已按真血缘更正为两件（bin/zerg-agentd-linux-amd64 · bin/zerg-agentd-darwin-amd64，源 agent/cmd/zerg-agentd）} · 散文=两件旧件已从「跨架构件」口径摘除并归档：门B7b 名单与 publish/SECURITY.md 按真血缘更正（提交 33b6b4b4，源 agent/cmd/zerg-agentd）；盘上两枚移入 bin/_history/（sha16 c11b2cb825dcad7d · 24f9ca62f3f39267，原名释放给命令面 CLI 组件 agent）；agent/README.md 旧名三处随之更正（提交 0051d395）。",
}

def self_test():
    ok = True

    def case(name, got, want):
        nonlocal ok
        good = got == want
        ok = ok and good
        print("  %s %s（got=%r · want=%r）" % ("✓" if good else "✗", name, got, want))

    # R1 三标记
    case("证据齐（命令+rc+读数）⇒ 无缺项",
         evidence_missing("现读 `zerg gap ls` ⇒ rc=0 ⇒ 411 条"), [])
    case("证据缺读数 ⇒ ['读数']",
         evidence_missing("跑 `zerg gap ls` rc=0"), ["读数"])
    case("证据缺退码 ⇒ ['退码']",
         evidence_missing("现读 scripts/gates/x.py 命中 12 处"), ["退码"])
    case("空证据 ⇒ 三项全缺",
         evidence_missing(""), ["命令", "退码", "读数"])

    # R1 存量/新增分界
    rows = [
        {"id": "old", "state": "已解", "solved_evidence": "无", "found_at": "2026-09-20T00:00:00"},
        {"id": "new", "state": "已解", "solved_evidence": "无", "found_at": "2026-09-28T00:00:00"},
    ]
    rc, reds, reports = judge(rows, "/nonexistent", CUTOFF_DEFAULT)
    case("R1 存量只报 / 新增判红（rc=1 · reds 只有 new）",
         (rc, [r["read"]["found_at"] for r in reds],
          [r["read"]["found_at"] for r in reports]),
         (1, ["2026-09-28T00:00:00"], ["2026-09-20T00:00:00"]))

    # R3 自相矛盾（含作废注过滤）
    rows3 = [{"id": "c", "state": "已解", "solved_evidence": "`zerg x` rc=0 5 条",
              "found_at": "2026-09-28", "notes": ["仍缺：前提还在"]}]
    rc, reds, _ = judge(rows3, "/nonexistent", CUTOFF_DEFAULT)
    case("R3 闭态 + 末条含「仍缺」⇒ rc=1", (rc, reds[0]["rule"]), (1, "R3"))
    rows3b = [{"id": "c", "state": "已解", "solved_evidence": "`zerg x` rc=0 5 条",
               "found_at": "2026-09-28", "notes": ["仍缺：前提还在"],
               "notes_void": [{"void": True, "n": 1}]}]
    rc, reds, _ = judge(rows3b, "/nonexistent", CUTOFF_DEFAULT)
    case("R3 作废注不计入末条 ⇒ 不红", (rc, reds), (0, []))

    # R2 只报告（不判红）—— 用本件自身当「在盘」的fixture
    rows2 = [{"id": "r2", "state": "仍缺", "unit": "scripts/gates/check-gap-state-vs-artifact.py"}]
    rc, reds, reports = judge(rows2, REPO_ROOT, CUTOFF_DEFAULT)
    case("R2 点名件在盘 ⇒ 只报告不判红", (rc, reds, any(r["rule"] == "R2" for r in reports)),
         (0, [], True))
    rows2b = [{"id": "r2", "state": "仍缺", "unit": "无件(命令面)"}]
    rc, reds, reports = judge(rows2b, REPO_ROOT, CUTOFF_DEFAULT)
    case("R2 非路径 unit ⇒ 不报", (rc, reports), (0, []))

    # unit 形态
    case("unit_is_path", (unit_is_path("scripts/gates/x.py"), unit_is_path("无件"),
                          unit_is_path("拟(门)"), unit_is_path("")),
         (True, False, False, False))

    # ── R5 销案证据点名仓内路径必须存在（成对负控 · 自带 hermetic 仓根，不碰真仓/真源）──
    tmp = tempfile.mkdtemp(prefix="gap-r5-selftest-")
    try:
        os.makedirs(os.path.join(tmp, "core", "cmd", "zerg"))
        os.makedirs(os.path.join(tmp, "bin"))
        with open(os.path.join(tmp, "core", "cmd", "zerg", "main.go"), "w"):
            pass  # 正控用的真路径
        top = set(os.listdir(tmp))  # 真路径主张抽串用的仓根顶层
        base = {"state": "已解", "found_at": "2026-09-28T00:00:00", "id": "GAP-20260928-99"}
        real = dict(base, solved_evidence="现读 `core/cmd/zerg/main.go` rc=0 12 条")
        bogus = dict(base, solved_evidence="现读 `core/cmd/zerg/chat_tool_overview.go` rc=0 12 条")
        # 正控：点名真路径（新账）⇒ 绿
        rc, reds, reps = judge([real], tmp, CUTOFF_DEFAULT)
        case("R5 正控：点名仓内真路径（新账）⇒ 绿 rc=0", (rc, reds), (0, []))
        # 负控：点名不存在的路径（新账）⇒ 必红且点名「那一串 + 所属 gap id」
        rc, reds, reps = judge([bogus], tmp, CUTOFF_DEFAULT)
        case("R5 负控：点名仓内不存在路径（新账）⇒ rc=1 + 点名串与 id",
             (rc, [r["rule"] for r in reds],
              "chat_tool_overview.go" in reds[0]["what"], "GAP-20260928-99" in reds[0]["what"]),
             (1, ["R5"], True, True))
        print("      ↳ 负控点名行（逐字）：✗ [R5] %s" % reds[0]["what"])

        # ① 反引号内的真路径 ⇒ 判（抽为真主张；在盘 ⇒ 绿）
        case("R5 ① 反引号内真路径 ⇒ 判（抽真主张 · 在盘绿）",
             (evidence_path_tokens(real["solved_evidence"], top),
              judge([real], tmp, CUTOFF_DEFAULT)[0]),
             (["core/cmd/zerg/main.go"], 0))
        # ② 反引号内的不存在路径 ⇒ 红（负控已跑，此处再断一遍口径）
        case("R5 ② 反引号内不存在路径 ⇒ 红（rc=1 + 点名串）",
             (rc, [r["rule"] for r in reds], "chat_tool_overview.go" in reds[0]["what"]),
             (1, ["R5"], True))
        # ③ 散文提到的真路径名（不加反引号）⇒ 不判（语境门槛拦下）
        prose = dict(base, solved_evidence="散文=排查时提到 core/cmd/zerg/main.go（未加反引号）rc=0 12 条")
        _prc, _preds, _ = judge([prose], tmp, CUTOFF_DEFAULT)
        case("R5 ③ 散文提及真路径名（无反引号）⇒ 不判",
             (_prc, _preds, evidence_path_tokens(prose["solved_evidence"], top)),
             (0, [], []))
        # 口径 B：紧跟在指令性前缀（cmd=）之后的不存在路径 ⇒ 仍判（真主张）
        pfx = dict(base, id="GAP-20260928-97",
                   solved_evidence="现跑 cmd=core/cmd/zerg/zzz_pfx_probe.go rc=0 12 条")
        _prc, _preds, _ = judge([pfx], tmp, CUTOFF_DEFAULT)
        case("R5 口径B：紧跟前缀 cmd= 的不存在路径 ⇒ 判（红）",
             (_prc, [_r["rule"] for _r in _preds]), (1, ["R5"]))
        # ④ 4 条真实误伤件逐字回归 ⇒ 不判（语境门槛下不再是路径主张）
        reg_rows = [dict(base, id=k, solved_evidence=v)
                    for k, v in sorted(R5_REGRESSION_EVIDENCES.items())]
        _prc, _preds, _ = judge(reg_rows, tmp, CUTOFF_DEFAULT)
        case("R5 ④ 4 条真实误伤件（新账）⇒ R5 不判",
             [_r["rule"] for _r in _preds if _r["rule"] == "R5"], [])
        case("R5 ④ 4 条误伤件均不被抽为真主张",
             {k: evidence_path_tokens(v, top) for k, v in sorted(R5_REGRESSION_EVIDENCES.items())},
             {k: [] for k in sorted(R5_REGRESSION_EVIDENCES)})
        # 存量：旧账 + 不存在路径 ⇒ 只报告、不进 rc（降级≠隐身：进 reports）
        old = dict(bogus, id="GAP-20260920-01", found_at="2026-09-20T00:00:00")
        rc, reds, reps = judge([old], tmp, CUTOFF_DEFAULT)
        case("R5 存量：旧账 + 不存在路径 ⇒ rc=0 且进 reports",
             (rc, reds, any(r["rule"] == "R5" for r in reps)), (0, [], True))
        # 号内日期 ≥ 2026-09-28 ⇒ 新账（即便 found_at 早于 cutoff）
        idnew = dict(bogus, id="GAP-20261001-01", found_at="2026-09-20T00:00:00")
        rc, reds, _ = judge([idnew], tmp, CUTOFF_DEFAULT)
        case("R5 新账按号内日期（>=2026-09-28）⇒ 判红", (rc, [r["rule"] for r in reds]), (1, ["R5"]))
        # URL / 绝对 / 非仓内顶层 ⇒ 不判（判不了即不抽 ✗ 不臆造）
        skip = dict(base, solved_evidence="见 `https://github.com/x/y.go` · `/etc/hosts` · `otherrepo/a/b.py` rc=0 12 条")
        rc, reds, reps = judge([skip], tmp, CUTOFF_DEFAULT)
        case("R5 URL/绝对/非仓内顶层 ⇒ 不判 rc=0", (rc, reds), (0, []))
        case("R5 抽串：`~/.zerg/x` 的后缀不被当独立仓内路径",
             evidence_path_tokens("取 ~/.zerg/selftest", {"core", "scripts"}), [])
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

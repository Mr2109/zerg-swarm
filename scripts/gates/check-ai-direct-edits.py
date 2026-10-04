#!/usr/bin/env python3
# check-ai-direct-edits.py — 门⑬：AI 直改纪律（§20.3 `H1` · §20.1 步 2/步 9 · 开工单 T-61）。
#
# 病灶（`H1` 逐字）：「AI 侧**零直改**：取证只读、执行走命令面、产物走候选区」，
# 判据 = **造一次「AI 直接改仓内文件」的动作 ⇒ 留痕里没有对应的命令面调用 ⇒ 门红**。
# 今天这条纪律**只写在文字里**（靠人自觉）⇒ 换会话 / 换 AI 就失效。
#
# 本门禁把「零直改」变成**可机检的物**：读**交接回执**（§20.3 `H3` 的一页一轮），
# 逐轮判 —— 回执里说动过仓（`commits` 非空），`evidence` 里就必须有**至少一条命令面调用**。
#
# 五条判据（任一不过 ⇒ rc=1）：
#   R1 **H1**：`commits` 非空 ⇒ `evidence` 至少一条 `zerg …`（**这一条就是「AI 直改」的红线**）；
#   R2 **H3 五格**：`round_id` / `trace_id` / `what` / `next` 非空，且 `trace_id` = `TR-` + 12 位十六进制；
#   R3 **件名 ↔ round_id**：文件名必须 = `<round_id>.json`（台账不许挂在空气上）；
#   R4 **命令面调用真实存在**：`evidence` 里点名的 `zerg <路径>` 必须能在**命令树**里解析到
#      （命令树的可机读投影 = `core/cmd/zerg/testdata/cli-matrix.json` 的 `command` 列 —— 门⑪ 的同一份）。
#   R5 **轮级账 ⟷ 件级账 双向对账**（缺口 `GAP-20260927-81`「既漏又假」的两半）：
#      ★ 旧行为的两半：**假** = R1 只要 `evidence` 里出现**任意** `zerg <词>`（含**只读**调用）就放行
#        ⇒ 一次「手改后补一条含任意 zerg 调用的回执」即可绕绿（报绿而 H1 根本没判）；
#        **漏** = 本门只扫**回执目录**一层 `.json`，**不读件级账**（`edit_audit.jsonl`）⇒ 回执账一旦断供
#        （轮级账停在某天、而件级账里主仓写轮仍天天在涨），本门照报「零直改纪律全过」—— 那个绿与今天的改动无关。
#      ⇒ 修法（两半同批）= 把**件级账的主仓写轮行**（`event == "edit"` 且非 `sibling_repo`）当第二个判据源：
#        · **R5a（假 · 写轮对账）**：每一「说动过仓」的页，其**轮窗**（前页回执之后、本页 `created_at` 之前）
#          内件级账必须有 ≥1 主仓写轮行；无 ⇒ 红（这一轮说动过仓，件级账里却没有写轮留痕 ⇒ 不是走命令面写的）。
#          ★ 页 `created_at` **早于件级账最早一行** ⇒ 本腿**不上判**（件级账判不了它 —— 不许拿缺账当红）。
#        · **R5b（漏 · 覆盖自证）**：件级账里最新主仓写轮**晚于**最新回执 ⇒ **rc=2 判不了**
#          （回执账没覆盖那些轮 ⇒ 本门**不得**报绿，出声）。
#        ★ 两腿都不做**逐件**匹配（两账无公共键：件级 vs 轮级）—— 只做**轮级/覆盖**对账，避免逐件匹配造的假红。
# 空转（回执目录不存在 / 0 件）⇒ **rc=2**（不给结论）—— 「没回执」不是「绿」，正是 `H3` 要治的那一格。
#
# 用法：
#   python3 scripts/gates/check-ai-direct-edits.py                 # 真目标（$ZERG_RECEIPT_DIR 或 ~/.zerg/state/receipts）
#   python3 scripts/gates/check-ai-direct-edits.py --dir <目录>     # 指目录（自检夹具用）
#   python3 scripts/gates/check-ai-direct-edits.py --ledger <件>    # 指件级账（自检夹具用；默认 <状态目录>/edit_audit.jsonl）
#   python3 scripts/gates/check-ai-direct-edits.py --self-test      # 1 正控 + 10 负控 + 2 空转
#
# 退码：0 全绿 · 1 红（R1–R5a 任一）· 2 不给结论（空转 / 件级账读不出来 / R5b 覆盖自证）

import argparse
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates → 仓根
MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")
LEDGER_REL = os.path.join("edit_audit.jsonl")

REQ = ["round_id", "trace_id", "what", "next"]
TRACE_RE = re.compile(r"^TR-[0-9a-f]{12}$")
# 命令面调用 = 一行的开头或标点之后出现 `zerg <段>`
ZERG_RE = re.compile(r"(?:^|[\s（(])zerg\s+([A-Za-z][A-Za-z0-9_-]*)")
# ★ 缺口 `GAP-20260927-59`：抽词器此前把 `zerg --help` / `zerg -h` / `zerg <版本号>+dirty`
#   一律抽成空序列 ⇒ R1 把「走了命令面（只是没有子命令）」误判成「AI 直改」= 假红。
#   正解：显式认这些非子命令调用形态（旗标 / 版本号），它们满足 R1（走了命令面）但
#   不进 R4（不拿旗标/版本号去命令树里解析）。
ZERG_NONCMD_RE = re.compile(r"(?:^|[\s（(])zerg\s+(?:--?[A-Za-z]|[0-9])")


def default_dir():
    d = os.environ.get("ZERG_RECEIPT_DIR", "").strip()
    if d:
        return d
    s = os.environ.get("ZERG_STATE_DIR", "").strip()
    if s:
        return os.path.join(s, "receipts")
    return os.path.join(os.path.expanduser("~"), ".zerg", "state", "receipts")


def default_ledger():
    """件级账（`edit_audit.jsonl`）默认落点：状态目录下一件（与回执账同根）。"""
    s = os.environ.get("ZERG_STATE_DIR", "").strip()
    if s:
        return os.path.join(s, LEDGER_REL)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state", LEDGER_REL)


def load_command_tree(root):
    """命令树投影：cli-matrix.json 的 command 列（门⑪ 的同一份，不另抄）。

    返第一段集合（`gate ls` ⇒ `gate`）—— `H1` 判的是「这条调用走没走命令面」，
    粒度到**族/裸动词**即可（到整条路径会把「族名 + 动作」的写法全判成幽灵）。
    """
    p = os.path.join(root, MATRIX_REL)
    try:
        with open(p, encoding="utf-8") as f:
            m = json.load(f)
    except Exception as e:
        return None, "命令树投影读不出来（%s）：%s" % (MATRIX_REL, e)
    names = set()
    for c in m.get("cases", []):
        if isinstance(c.get("command"), str) and c["command"]:
            names.add(c["command"].split()[0])
    return names, None


def load_ledger(path):
    """件级账的主仓写轮行时刻（升序）。

    真源 = `edit_audit.jsonl` 里 `event == "edit"` 且**非** `sibling_repo`（= 落主仓相对路径件）的行；
    只需 `at`（ISO-8601 · 同形 ⇒ 字典序即时序）。`at` 缺失的行跳过（不是写轮读数）。
    返 (ats, err)：err 非空 ⇒ 件级账读不出来 ⇒ 调用方按 rc=2 落（缺账不当绿）。
    """
    if not os.path.isfile(path):
        return None, "件级账读不出来（%s）：件不存在" % path
    ats = []
    try:
        with open(path, encoding="utf-8") as f:
            for ln in f:
                ln = ln.strip()
                if not ln:
                    continue
                try:
                    o = json.loads(ln)
                except Exception:
                    continue
                if not isinstance(o, dict):
                    continue
                if o.get("event") != "edit" or o.get("sibling_repo"):
                    continue
                a = str(o.get("at", "")).strip()
                if a:
                    ats.append(a)
    except Exception as e:
        return None, "件级账读不出来（%s）：%s" % (path, e)
    ats.sort()
    return ats, None


def check_dir(d, tree, ledger):
    """返回 (rc, 行列表)。rc: 0 全绿 · 1 红 · 2 不给结论。`ledger` = 件级账写轮时刻（升序）。

    `ledger is None` ⇒ 逐页照旧判（R1–R4），**不**跑 R5（自检里「件级账不在场」的那几格走这条）。
    """
    lines = []
    if not os.path.isdir(d):
        return 2, ["⛔ 不给结论：回执目录不存在（%s）—— 没有回执 ≠ 绿（§20.3 H3）" % d]
    ents = sorted(e for e in os.listdir(d) if e.endswith(".json"))
    if not ents:
        return 2, ["⛔ 不给结论：回执目录里 0 页（%s）—— 没有回执 ≠ 绿（§20.3 H3）" % d]
    errs = []
    pages = []   # (name, created_at, commits 条数)
    for name in ents:
        p = os.path.join(d, name)
        try:
            with open(p, encoding="utf-8") as f:
                r = json.load(f)
        except Exception as e:
            errs.append("R2 回执不是合法 JSON：%s（%s）" % (name, e))
            continue
        rid = r.get("round_id", "")
        # R2 五格 + trace_id 形态
        for k in REQ:
            if not str(r.get(k, "")).strip():
                errs.append("R2 回执缺格 %s：%s" % (k, name))
        if rid and not TRACE_RE.match(str(r.get("trace_id", ""))):
            errs.append("R2 trace_id 形态不对（要 TR- + 12 位十六进制）：%s ⇒ %r"
                        % (name, r.get("trace_id")))
        # R3 件名 ↔ round_id
        if rid and name != rid + ".json":
            errs.append("R3 件名与 round_id 对不上：文件 %s · round_id %r" % (name, rid))
        ev = r.get("evidence") or []
        if isinstance(ev, str):
            ev = [ev]
        evtext = " · ".join(str(x) for x in ev)
        commits = r.get("commits") or []
        if isinstance(commits, str):
            commits = [commits]
        # R1 H1：动过仓 ⇒ 必须有一条命令面调用
        calls = ZERG_RE.findall(evtext)
        noncmd = ZERG_NONCMD_RE.findall(evtext)   # zerg --help|-h|<版本号> 等非子命令调用
        if commits and not calls and not noncmd:
            errs.append("R1 **AI 直改**：%s 说动过仓（commits=%d 条）而证据里**没有任何 `zerg …` 调用** "
                        "⇒ 门红（§20.3 H1：执行只能走命令面）" % (name, len(commits)))
        # R4 点名的命令面调用必须真在命令树里
        if tree is not None:
            for c in calls:
                if c not in tree:
                    errs.append("R4 证据里点名的命令面调用 `zerg %s` 在命令树里**解析不到**（幽灵调用）：%s"
                                % (c, name))
        pages.append((name, str(r.get("created_at", "")).strip(), len(commits)))
    # ── R5：回执账（轮级）⟷ 件级账（件级）双向对账（GAP-20260927-81 的两半）────────────
    if ledger is not None and ledger:
        first_at, newest_at = ledger[0], ledger[-1]
        stamps = sorted({ca for (_, ca, _) in pages if ca})
        # R5a（假）：说动过仓的页，其「轮窗」内件级账须有 ≥1 主仓写轮行。
        for (name, ca, ncom) in pages:
            if ncom <= 0 or not ca:
                continue
            if ca < first_at:
                continue   # 该页早于件级账最早一行 ⇒ 件级账判不了它（不上判 · 不许拿缺账当红）
            lo = ""
            for k in stamps:
                if k < ca:
                    lo = k
            win = [a for a in ledger if (lo == "" or a > lo) and a <= ca]
            if not win:
                errs.append("R5a **假绿绕行**：%s 说动过仓（commits=%d 条）而件级账在这一轮"
                            "（%s 之后、%s 之前）**没有任何主仓写轮行** ⇒ 不是走命令面写的"
                            "（§20.3 H1：执行只能走命令面）"
                            % (name, ncom, lo or "件级账起点", ca))
    if errs:
        return 1, errs
    # R5b（漏）：件级账里最新主仓写轮晚于最新回执 ⇒ 回执账没覆盖 ⇒ 判不了（不得报绿）。
    if ledger is not None and ledger:
        newest_at = ledger[-1]
        stamps = sorted({ca for (_, ca, _) in pages if ca})
        if stamps and newest_at > stamps[-1]:
            late = sum(1 for a in ledger if a > stamps[-1])
            return 2, ["⛔ 不给结论（R5b 覆盖自证）：件级账里最新主仓写轮 %s **晚于**最新回执 %s —— "
                       "有 %d 个主仓写轮没有任何回执覆盖 ⇒ 回执账断供，本门判不了"
                       "（旧行为在这报绿＝假绿 · GAP-20260927-81）" % (newest_at, stamps[-1], late)]
    lines.append("回执 %d 页逐页过：H1 零直改 ✓ · H3 五格 ✓ · 件名 ✓ · 命令面调用可解析 ✓" % len(ents))
    if ledger is not None and ledger:
        lines.append("R5 轮级账 ⟷ 件级账双向对账 ✓（主仓写轮 %d 行 · 最新 %s ≤ 最新回执）"
                     % (len(ledger), ledger[-1]))
    return 0, lines


# ── 自检（成对负控：每条判据都要有「喂错的东西必红」的那一格）────────────────────

def _mk(d, name, obj):
    with open(os.path.join(d, name), "w", encoding="utf-8") as f:
        json.dump(obj, f, ensure_ascii=False)


def _mkledger(path, ats, sibling_at=""):
    """造一件件级账夹具：`ats` 各写一条 `event=edit` 主仓写轮行；`sibling_at` 写一条 sibling 行（不上判）。"""
    with open(path, "w", encoding="utf-8") as f:
        for a in ats:
            f.write(json.dumps({"at": a, "event": "edit", "file": "core/x.go",
                                "mode": "replace", "by": "Mr2109"}, ensure_ascii=False) + "\n")
        if sibling_at:
            f.write(json.dumps({"at": sibling_at, "event": "edit", "file": "docs/a.md",
                                "sibling_repo": "Zerg-内部文档"}, ensure_ascii=False) + "\n")


def _good(rid="R-20260920T000000", **kw):
    r = {"round_id": rid, "trace_id": "TR-0123456789ab", "what": "做了事",
         "next": "下一步", "blockers": [], "evidence": ["zerg gate run --fast"],
         "commits": ["deadbee"], "created_at": "2026-09-20T00:00:00+08:00"}
    r.update(kw)
    return r


def self_test(root):
    fails = []
    t = tempfile.mkdtemp(prefix="zerg-gate13-")
    try:
        # ① 空转：目录不存在 ⇒ 2
        rc, _ = check_dir(os.path.join(t, "nope"), None, None)
        if rc != 2:
            fails.append("自检① 目录不存在应 rc=2，实测 %d" % rc)
        # ② 空转：目录在但 0 页 ⇒ 2
        empty = os.path.join(t, "empty")
        os.makedirs(empty)
        rc, _ = check_dir(empty, None, None)
        if rc != 2:
            fails.append("自检② 0 页应 rc=2，实测 %d" % rc)
        # ③ 正控：合规 ⇒ 0
        ok = os.path.join(t, "ok")
        os.makedirs(ok)
        _mk(ok, "R-20260920T000001.json", _good("R-20260920T000001"))
        rc, _ = check_dir(ok, {"gate"}, None)
        if rc != 0:
            fails.append("自检③ 合规回执应 rc=0，实测 %d" % rc)
        # ④ 负控 R1：动过仓但证据里没有命令面调用 ⇒ 1
        n1 = os.path.join(t, "n1")
        os.makedirs(n1)
        _mk(n1, "R-20260920T000002.json", _good("R-20260920T000002", evidence=["vim core/x.go"]))
        rc, _ = check_dir(n1, {"gate"}, None)
        if rc != 1:
            fails.append("自检④ R1 应 rc=1（AI 直改），实测 %d" % rc)
        # ⑤ 负控 R1 不误判：没动仓（commits 空）⇒ 0
        n2 = os.path.join(t, "n2")
        os.makedirs(n2)
        _mk(n2, "R-20260920T000003.json", _good("R-20260920T000003", commits=[], evidence=["只读 grep"]))
        rc, _ = check_dir(n2, {"gate"}, None)
        if rc != 0:
            fails.append("自检⑤ 没动仓应 rc=0（不误判），实测 %d" % rc)
        # ⑥ 负控 R2：trace_id 形态坏 ⇒ 1
        n3 = os.path.join(t, "n3")
        os.makedirs(n3)
        _mk(n3, "R-20260920T000004.json", _good("R-20260920T000004", trace_id="trace-bad"))
        rc, _ = check_dir(n3, {"gate"}, None)
        if rc != 1:
            fails.append("自检⑥ R2 应 rc=1，实测 %d" % rc)
        # ⑦ 负控 R3：件名 ≠ round_id ⇒ 1
        n4 = os.path.join(t, "n4")
        os.makedirs(n4)
        _mk(n4, "R-错的.json", _good("R-20260920T000005"))
        rc, _ = check_dir(n4, {"gate"}, None)
        if rc != 1:
            fails.append("自检⑦ R3 应 rc=1，实测 %d" % rc)
        # ⑧ 负控 R4：点名的命令面调用在命令树里解析不到 ⇒ 1
        n5 = os.path.join(t, "n5")
        os.makedirs(n5)
        _mk(n5, "R-20260920T000006.json", _good("R-20260920T000006", evidence=["zerg 绝无此命令"]))
        rc, _ = check_dir(n5, {"gate"}, None)
        if rc != 1:
            fails.append("自检⑧ R4 应 rc=1（幽灵调用），实测 %d" % rc)
        # ⑨ 负控 R1 不误判：`zerg --help`（非子命令调用）⇒ 0（GAP-20260927-59）
        n6 = os.path.join(t, "n6")
        os.makedirs(n6)
        _mk(n6, "R-20260920T000007.json", _good("R-20260920T000007", evidence=["zerg --help"]))
        rc, _ = check_dir(n6, {"gate"}, None)
        if rc != 0:
            fails.append("自检⑨ zerg --help 应 rc=0（非子命令调用 · 不误判），实测 %d" % rc)
        # ⑩ 负控 R1 不误判：`zerg 2.5.12+dirty`（版本号形态）⇒ 0（GAP-20260927-59）
        n7 = os.path.join(t, "n7")
        os.makedirs(n7)
        _mk(n7, "R-20260920T000008.json", _good("R-20260920T000008", evidence=["zerg 2.5.12+dirty"]))
        rc, _ = check_dir(n7, {"gate"}, None)
        if rc != 0:
            fails.append("自检⑩ zerg 版本号 应 rc=0（版本号形态 · 不误判），实测 %d" % rc)
        # ⑪ 负控 R5a（假绿绕行）：手改后补一条含**任意** zerg 调用的回执 ⇒ 1（GAP-20260927-81）
        #     两页：第一页合法（轮窗内有主仓写轮行）· 第二页说动过仓但件级账该窗零写轮 ⇒ 红。
        n8 = os.path.join(t, "n8")
        os.makedirs(n8)
        _mk(n8, "R-20260920T100000.json",
            _good("R-20260920T100000", created_at="2026-09-20T10:00:00+08:00",
                  evidence=["zerg dev edit --proposal DEV-1"]))
        _mk(n8, "R-20260920T120000.json",
            _good("R-20260920T120000", created_at="2026-09-20T12:00:00+08:00",
                  evidence=["zerg gate ls"]))   # 手改后补的：只引一条**只读**调用
        led8 = os.path.join(t, "n8-ledger.jsonl")
        _mkledger(led8, ["2026-09-20T09:00:00+08:00"])
        rc, _ = check_dir(n8, {"gate", "dev"}, load_ledger(led8)[0])
        if rc != 1:
            fails.append("自检⑪ R5a 应 rc=1（手改补回执绕绿 · 假绿），实测 %d" % rc)
        # ⑫ 负控 R5b（覆盖自证）：件级账最新主仓写轮晚于最新回执 ⇒ 2（不是绿）
        n9 = os.path.join(t, "n9")
        os.makedirs(n9)
        _mk(n9, "R-20260920T100100.json",
            _good("R-20260920T100100", created_at="2026-09-20T10:00:00+08:00",
                  evidence=["zerg dev edit --proposal DEV-1"]))
        led9 = os.path.join(t, "n9-ledger.jsonl")
        _mkledger(led9, ["2026-09-20T09:00:00+08:00", "2026-09-20T12:00:00+08:00"])
        rc, _ = check_dir(n9, {"gate", "dev"}, load_ledger(led9)[0])
        if rc != 2:
            fails.append("自检⑫ R5b 应 rc=2（覆盖自证 · 判不了），实测 %d" % rc)
        # ⑬ 正控 R5：写轮在轮窗内、且没有更晚的写轮 ⇒ 0
        n10 = os.path.join(t, "n10")
        os.makedirs(n10)
        _mk(n10, "R-20260920T100200.json",
            _good("R-20260920T100200", created_at="2026-09-20T10:00:00+08:00",
                  evidence=["zerg dev edit --proposal DEV-1"]))
        led10 = os.path.join(t, "n10-ledger.jsonl")
        _mkledger(led10, ["2026-09-20T09:00:00+08:00"], sibling_at="2026-09-21T00:00:00+08:00")
        rc, _ = check_dir(n10, {"gate", "dev"}, load_ledger(led10)[0])
        if rc != 0:
            fails.append("自检⑬ R5 正控应 rc=0（写轮在轮窗内 · sibling 行不上判），实测 %d" % rc)
        # ⑭ 正控 R5 不误判：回执早于件级账最早一行 ⇒ 本腿不上判 ⇒ 0（真仓首件即此态）
        n11 = os.path.join(t, "n11")
        os.makedirs(n11)
        _mk(n11, "R-20260919T235900.json",
            _good("R-20260919T235900", created_at="2026-09-19T23:59:00+08:00"))
        led11 = os.path.join(t, "n11-ledger.jsonl")
        _mkledger(led11, ["2026-09-20T09:00:00+08:00"])
        rc, _ = check_dir(n11, {"gate"}, load_ledger(led11)[0])
        if rc != 2:
            fails.append("自检⑭ 回执早于件级账最早一行 ⇒ R5a 不上判、但 R5b 仍应 rc=2，实测 %d" % rc)
        # ⑮ 正控 R5a 不上判 + 无更晚写轮 ⇒ 0（件级账起点晚于页、且页就是最新）
        n12 = os.path.join(t, "n12")
        os.makedirs(n12)
        _mk(n12, "R-20260919T230000.json",
            _good("R-20260919T230000", created_at="2026-09-19T23:00:00+08:00"))
        led12 = os.path.join(t, "n12-ledger.jsonl")
        _mkledger(led12, [])   # 空件级账
        rc, _ = check_dir(n12, {"gate"}, load_ledger(led12)[0])
        if rc != 0:
            fails.append("自检⑮ 空件级账 + 页早于其起点 ⇒ 应 rc=0（不上判），实测 %d" % rc)
    finally:
        shutil.rmtree(t, ignore_errors=True)
    if fails:
        print("门⑬ 自检失败 %d 条：" % len(fails))
        for f in fails:
            print("  ✗ " + f)
        return 1
    print("门⑬ 自检全过：3 正控 + 10 负控 + 2 空转（成对负控）")
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
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--dir", default="")
    ap.add_argument("--ledger", default="")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return self_test(a.root)
    d = a.dir or default_dir()
    tree, terr = load_command_tree(a.root)
    if terr:
        print("⛔ 不给结论：" + terr)
        return 2
    lpath = a.ledger or default_ledger()
    ats, lerr = load_ledger(lpath)
    if lerr:
        print("⛔ 不给结论：" + lerr)
        return 2
    rc, lines = check_dir(d, tree, ats)
    head = "门⑬ AI 直改纪律（§20.3 H1 · 回执 %s）· 仓根 %s" % (d, a.root)
    print(head)
    for ln in lines:
        print("  " + ln)
    if rc == 0:
        print("⇒ 零直改纪律全过（rc=0）")
    elif rc == 1:
        print("⇒ 有红项（rc=1）：上面逐条点名")
    else:
        print("⇒ 不给结论（rc=2）")
    return rc


if __name__ == "__main__":
    sys.exit(main())

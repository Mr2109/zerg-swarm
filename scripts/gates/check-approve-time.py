#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-time.py v1.0.0 —— 审计时刻**对拍**（2.5.11 人签 D 批 · `D5`）

为什么存在（`D5` 的任务原文，逐字口径）
--------------------------------------------------
`D5` 要一枚**只读**对拍件：**同一个件**的审计时刻要能排出一条**不与签发 / 提交矛盾**的序；
排不出来的行**标出来**并给**可解释口径**（`task` 单 §二 `D5` 判据①）。本件就是那一格。

★ **本件只读**：它读 `~/.zerg/state/approvals/<件>.json`、`~/.zerg/state/edit_audit.jsonl`
与仓里 `git log --format=%cI`，**一个字节都不写** ✗ —— 不动件、不动审计、不动 git（不 add / 不 commit）✗。

三处读数（每处都带口径：件名 / 行号 / `head_sha`）
--------------------------------------------------
  A **件**：`<state>/approvals/<件>.json` ⇒ 件名 · `approved_at` 落在**第几行** · 件 sha256 · 件落盘时刻（mtime）
  B **审计**：`<state>/edit_audit.jsonl` ⇒ 总行数 · **键到本件**的行（`approval` 字段以本件路径结尾）·
    它们的行号与 `at`（`by` / 机器名那类字段**只用于回指分组**，**不**当判据 · `D5` 判据②）
  C **提交**：`git -C <仓根> log --format=%H%x09%cI` ⇒ `head_sha` + 提交时刻表（现算 · 不写常量）

判据（排不出来的行 ⇒ 标出来 · 红的那两条能单独失败）
--------------------------------------------------
  R1 **同一世代里审计早于签发**（红）：存在键到本件的行满足 `件落盘时刻 ≤ at < 件 approved_at`
     ⇒ 同一世代内审计早于签发 ⇒ 排不出任何不矛盾的序。
  R2 **`at` 排不出序**（红）：`at` 不是带时区的 RFC3339（解析不了 ⇒ 这条时刻不可用）。
  ·  **排得出但要交代**（**不判红**）：① `at < 件落盘时刻` ⇒ 归「**更早世代**」（件被同名换过 ⇒
    同名不覆盖 ⇒ 要人删件才干净 · 口径照实写）② 该行**之后还没有提交** ⇒ 标「还没提交（等本次提交）」
     ③ 含机器名那类字段且**不是本机**的行 ⇒ 只要求排得出不矛盾的序。
  ·  **机器判不了（明标 · 不许假装判过）**：① **拿什么当「当时」的时基**（`v1.3 §8.4` 的 `P21`
     **仍待 Mr2109 亲选** ⇒ 本件只排「能被现跑读数支撑」的序）② `--ttl` 的过期只看**本地钟**（跨机不保证）。
  ·  三处读数**缺任一处 ⇒ 不出结论**：只许写「未取数」（rc=2）。

开工前读数（现跑 · 原样）
--------------------------------------------------
  件 `dev_edit.json`：`approved_at = 2026-09-21T20:53:35+08:00`（**第 4 行**）· 件落盘 `2026-09-21T20:53:35`
    ⇒ **两者同秒**（mtime == 签发时刻）⇒ **R1 那一格今天空着**（不是判据不会红 —— 见自检②）。
  审计 `edit_audit.jsonl`：**总 179 行** · **键到本件 136 行**（首条 `at=2026-09-21T12:54:47+08:00` ·
    尾条 `at=2026-09-22T09:46:43+08:00`）· 无 `approval` 字段的 **43 行**（那 43 行属于**加字段之前**的世代）
    · 含机器名字段的 **0 行**（跨机那一格今天也空着）。

退码
--------------------------------------------------
  0 = 排出了不矛盾的序（0 条红）· 1 = 有红（R1 / R2 命中）· 2 = **不给结论**（任一处读数取不到 / 自检不过）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-time.py                 # 先自检，再扫真目标
  python3 scripts/gates/check-approve-time.py --self-test     # 只跑自检（夹具 · 不碰真状态目录）
  python3 scripts/gates/check-approve-time.py --state ~/.zerg/state --root <仓根> --ticket dev_edit
"""

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-time.py v1.0.0"
DEFAULT_TICKET = "dev_edit"
TOL = dt.timedelta(seconds=0)  # 判据里不塞「容忍秒数」—— 同秒即同世代（口径写死为 0）


# ── 取数（全部只读）──────────────────────────────────────────────────────────────

def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def parse_iso(s):
    """带时区的 RFC3339 ⇒ aware datetime；解析不了 / 没时区 ⇒ None（R2 那一格）。"""
    if not isinstance(s, str):
        return None
    try:
        d = dt.datetime.fromisoformat(s.replace("Z", "+00:00"))
    except ValueError:
        return None
    return d if d.tzinfo is not None else None


def read_ticket(state, ticket):
    p = os.path.join(state, "approvals", "%s.json" % ticket)
    if not os.path.exists(p):
        return None, "件不在：%s" % p
    with open(p, encoding="utf-8") as f:
        lines = f.read().splitlines()
    obj, ln_no = None, None
    for i, ln in enumerate(lines, 1):
        if '"approved_at"' in ln:
            ln_no = i
            break
    with open(p, encoding="utf-8") as f:
        try:
            obj = json.load(f)
        except json.JSONDecodeError as e:
            return None, "件解不出来（%s）：%s" % (p, e)
    st = os.stat(p)
    return {
        "name": "%s.json" % ticket, "path": p, "sha256": sha256_file(p),
        "approved_at": obj.get("approved_at"), "line": ln_no,
        "mtime": dt.datetime.fromtimestamp(st.st_mtime).astimezone().isoformat(timespec="seconds"),
        "mtime_dt": dt.datetime.fromtimestamp(st.st_mtime).astimezone(),
        "at_dt": parse_iso(obj.get("approved_at")),
    }, ""


def read_audit(state, ticket):
    p = os.path.join(state, "edit_audit.jsonl")
    if not os.path.exists(p):
        return None, "审计件不在：%s" % p
    with open(p, encoding="utf-8") as f:
        raw = f.read().splitlines()
    rows, keyed = [], []
    for i, ln in enumerate(raw, 1):
        if not ln.strip():
            continue
        try:
            d = json.loads(ln)
        except json.JSONDecodeError:
            rows.append({"line": i, "at": None, "bad": "不是 JSON"})
            continue
        rec = {"line": i, "at": d.get("at"), "at_dt": parse_iso(d.get("at")),
               "by": d.get("by"), "machine": d.get("machine") or d.get("host"),
               "had_approval_field": "approval" in d, "bad": None}
        rows.append(rec)
        if str(d.get("approval", "")).endswith("%s.json" % ticket):
            keyed.append(rec)
    return {"path": p, "total": len(raw), "rows": rows, "keyed": keyed}, ""


def read_commits(repo):
    if not os.path.isdir(os.path.join(repo, ".git")):
        return None, "仓根里没有 .git：%s" % repo
    p = subprocess.run(["git", "-C", repo, "-c", "core.quotepath=false", "log", "--format=%H%x09%cI"],
                       capture_output=True, text=True)
    if p.returncode != 0:
        return None, "`git log` 退码 %d：%s" % (p.returncode, p.stderr.strip())
    commits = []
    for ln in p.stdout.splitlines():
        if "\t" not in ln:
            continue
        sha, iso = ln.split("\t", 1)
        commits.append({"sha": sha, "iso": iso, "dt": parse_iso(iso)})
    if not commits:
        return None, "`git log` 一行都没有"
    return {"head_sha": commits[0]["sha"], "commits": commits}, ""


def read_all(repo, state, ticket):
    t, why = read_ticket(state, ticket)
    if t is None:
        return None, why
    a, why = read_audit(state, ticket)
    if a is None:
        return None, why
    c, why = read_commits(repo)
    if c is None:
        return None, why
    return {"ticket": t, "audit": a, "commits": c}, ""


# ── 判定口（纯函数：自检与真目标共用）──────────────────────────────────────────────

def first_commit_at_or_after(commits, when):
    """该时刻之后**最早**的那笔提交 ⇒ 这一行「该次提交时刻」；没有 ⇒ None（标「还没提交」）。"""
    for c in reversed(commits["commits"]):  # git log 是新的在前 ⇒ 倒序 = 时间正序
        if c["dt"] is not None and c["dt"] >= when:
            return c
    return None


def judge(reads, hostname=None):
    """排序 + 分类。返回 (红行, 交代行, 逐行读数)。"""
    t, a, c = reads["ticket"], reads["audit"], reads["commits"]
    red, notes, per_row = [], [], []
    for r in a["keyed"]:
        when = r["at_dt"]
        if when is None:
            red.append("R2 `%s` 第 %d 行：`at=%r` 不是带时区的 RFC3339 ⇒ 这条时刻排不出序"
                       % (a["path"], r["line"], r["at"]))
            per_row.append((r["line"], r["at"], "R2 红", None, None, None))
            continue
        gen = "本世代"
        if t["at_dt"] is not None and t["mtime_dt"] <= when < t["at_dt"]:
            red.append("R1 `%s` 第 %d 行：`at=%s` 落在 [件落盘 %s, 件签发 %s) 里 ⇒ 同一世代里审计早于签发"
                       % (a["path"], r["line"], r["at"], t["mtime"], t["approved_at"]))
            per_row.append((r["line"], r["at"], "R1 红", None, None, None))
            continue
        if when < t["mtime_dt"]:
            gen = "更早世代"
            notes.append("交代 `%s` 第 %d 行：`at=%s` < 件落盘 `%s` ⇒ 归**更早世代**（件被同名换过 ⇒ "
                         "同名不覆盖 ⇒ 要人删件才干净）" % (a["path"], r["line"], r["at"], t["mtime"]))
        ck = first_commit_at_or_after(c, when)
        if ck is None:
            notes.append("交代 `%s` 第 %d 行：`at=%s` 之后**还没有提交** ⇒ 标「还没提交（等本次提交）」"
                         % (a["path"], r["line"], r["at"]))
        if r["machine"] and hostname and r["machine"] != hostname:
            notes.append("交代 `%s` 第 %d 行：机器名 `%s` ≠ 本机 ⇒ 只要求排得出不矛盾的序（跨机那一格）"
                         % (a["path"], r["line"], r["machine"]))
        per_row.append((r["line"], r["at"], gen, ck["sha"] if ck else None,
                        ck["iso"] if ck else None, r["machine"]))
    return red, notes, per_row


# ── 自检（先自证会红 · 夹具全在临时目录里，真状态目录一个字节不碰）──────────────────

def build_fixture(tmp, approved_at, mtime_shift, rows):
    """造一枚件 + 一份审计（`mtime` 用 `os.utime` 摆到指定秒 ⇒ 判据不看墙上钟 · 只看读数）。"""
    d = os.path.join(tmp, "approvals")
    os.makedirs(d, exist_ok=True)
    tk = {"tool": DEFAULT_TICKET, "approver": "Mr2109", "approved_at": approved_at,
          "scope": "*", "note": "D5 夹具", "sig_alg": "ed25519", "key_id": "0" * 16, "sig": ""}
    p = os.path.join(d, "%s.json" % DEFAULT_TICKET)
    with open(p, "w", encoding="utf-8") as f:
        json.dump(tk, f, ensure_ascii=False, indent=1)
        f.write("\n")
    base = parse_iso(approved_at)
    mt = (base + dt.timedelta(seconds=mtime_shift)).timestamp()
    os.utime(p, (mt, mt))
    with open(os.path.join(tmp, "edit_audit.jsonl"), "w", encoding="utf-8") as f:
        for at, extra in rows:
            rec = {"at": at, "file": "x", "approval": p}
            rec.update(extra or {})
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")
    return tmp


def self_test(repo, real_state):
    ok = bad = 0
    tmp = tempfile.mkdtemp(prefix="zerg-approve-time-selftest-")
    try:
        # ① 正控：真读数排出一条不矛盾的序
        r, why = read_all(repo, real_state, DEFAULT_TICKET)
        if r is None:
            print("⛔ 自检① 取不到真读数：%s ⇒ 不给结论" % why)
            return None
        red, notes, per_row = judge(r)
        if not red:
            print("   ✓ ① 正控：真件（%s · `at` 第 %d 行 · 件 sha %s…）排出 %d 行序 · 红 0 行 · 交代 %d 行 · head %s"
                  % (r["ticket"]["name"], r["ticket"]["line"], r["ticket"]["sha256"][:12],
                     len(per_row), len(notes), r["commits"]["head_sha"][:12]))
            ok += 1
        else:
            print("   ✗ ① 正控失败：真读数就有红：%s" % "；".join(red))
            bad += 1
        # ② 负控 R1：件落盘早于签发，审计行落在窗口里 ⇒ 红
        f2 = build_fixture(os.path.join(tmp, "r1"), "2026-09-21T20:53:35+08:00", -600,
                           [("2026-09-21T20:45:00+08:00", None)])
        r2, _ = read_all(repo, f2, DEFAULT_TICKET)
        red2, _, _ = judge(r2)
        if red2 and "R1" in red2[0]:
            print("   ✓ ② 负控：审计行落在 [件落盘, 件签发) 窗口里 ⇒ R1 判红（这一格不是「不会红」）")
            ok += 1
        else:
            print("   ✗ ② 负控失败：窗口里那行没判红")
            bad += 1
        # ③ 负控 R2：`at` 没时区 ⇒ 红
        f3 = build_fixture(os.path.join(tmp, "r2"), "2026-09-21T20:53:35+08:00", 0,
                           [("2026-09-21T20:54:00", None)])
        r3, _ = read_all(repo, f3, DEFAULT_TICKET)
        red3, _, _ = judge(r3)
        if red3 and "R2" in red3[0]:
            print("   ✓ ③ 负控：`at` 不带时区 ⇒ R2 判红（排不出序）")
            ok += 1
        else:
            print("   ✗ ③ 负控失败：没时区的 `at` 没判红")
            bad += 1
        # ④ 正控：更早世代 + 跨机 ⇒ **不**判红，只交代
        f4 = build_fixture(os.path.join(tmp, "gen"), "2026-09-21T20:53:35+08:00", 0,
                           [("2026-09-21T12:54:47+08:00", {"machine": "别人家的机器"})])
        r4, _ = read_all(repo, f4, DEFAULT_TICKET)
        red4, notes4, _ = judge(r4, hostname="本机")
        if not red4 and any("更早世代" in n for n in notes4) and any("跨机" in n for n in notes4):
            print("   ✓ ④ 正控：更早世代 + 跨机 ⇒ 不判红 · 只交代（%d 条交代）" % len(notes4))
            ok += 1
        else:
            print("   ✗ ④ 正控失败：更早世代 / 跨机被误判红了")
            bad += 1
        # ⑤ 负控：任一处读数缺 ⇒ 未取数（rc=2 那条口径）
        r5, why5 = read_all(repo, os.path.join(tmp, "空的"), DEFAULT_TICKET)
        if r5 is None and "件不在" in why5:
            print("   ✓ ⑤ 负控：件缺 ⇒ 未取数（%s）⇒ 不许当绿" % why5)
            ok += 1
        else:
            print("   ✗ ⑤ 负控失败：件缺却没判「未取数」")
            bad += 1
        # ⑥ 负控：提交表缺 ⇒ 未取数
        r6, why6 = read_all(os.path.join(tmp, "不是仓"), f2, DEFAULT_TICKET)
        if r6 is None and "没有 .git" in why6:
            print("   ✓ ⑥ 负控：`git log` 取不到 ⇒ 未取数（%s）⇒ 不许当绿" % why6)
            ok += 1
        else:
            print("   ✗ ⑥ 负控失败：提交表缺却没判「未取数」")
            bad += 1
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("   自检：%d 格过 · %d 格不过（夹具目录已清理 · 真状态目录一个字节未动）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo, state, ticket, hostname):
    r, why = read_all(repo, state, ticket)
    if r is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    t, a, c = r["ticket"], r["audit"], r["commits"]
    red, notes, per_row = judge(r, hostname=hostname)
    print("── 三处读数（每处带口径）")
    print("   A 件：%s · `approved_at` 在第 %d 行 = %s · 件落盘 = %s · 件 sha256 = %s"
          % (t["name"], t["line"] or -1, t["approved_at"], t["mtime"], t["sha256"]))
    print("   B 审计：%s · 总 %d 行 · 键到本件 %d 行 · 无 `approval` 字段 %d 行"
          % (a["path"], a["total"], len(a["keyed"]), sum(1 for x in a["rows"] if not x["had_approval_field"])))
    print("   C 提交：head_sha = %s · 提交表 %d 笔" % (c["head_sha"], len(c["commits"])))
    print("── 逐行排序（前 3 / 后 3 · 全量 %d 行）" % len(per_row))
    for row in (per_row[:3] + [("…", "…", "…", "…", "…", "…")] + per_row[-3:]) if len(per_row) > 6 else per_row:
        print("   行 %s · at %s · 世代 %s · 该次提交 %s %s"
              % (row[0], row[1], row[2], (row[3] or "—")[:12], row[4] or "—"))
    gen = [n for n in notes if "更早世代" in n]
    other = [n for n in notes if "更早世代" not in n]
    if gen:
        rows = [int(m) for m in re.findall(r"第 (\d+) 行", "\n".join(gen))]
        print("   ·  交代（合并 · %d 行）：**更早世代**（行 %d–%d）⇒ `at` < 件落盘 ⇒ 件被同名换过 · "
              "同名不覆盖 ⇒ 要人删件才干净" % (len(gen), min(rows), max(rows)))
    for n in other[:8]:
        print("   ·  %s" % n)
    if len(other) > 8:
        print("   ·  交代：同类另有 %d 条（略 · 口径同上）" % (len(other) - 8))
    print("── 机器判不了（明标 · 不假装判过）")
    print("   · 时基：拿什么当「当时」⇒ `v1.3 §8.4 P21` **仍待 Mr2109 亲选** ⇒ 本件只排「能被现跑读数支撑」的序")
    print("   · `--ttl` 的过期只看**本地钟**（跨机不保证） ⇒ 本件不拿墙上钟当判据时基")
    if red:
        print("⇒ 门禁红（rc=1）：%d 条排不出序" % len(red))
        for x in red:
            print("   ✗ %s" % x)
        return 1
    print("⇒ 全绿（rc=0）：%d 行排出了不矛盾的序 · 红 0 条 · 交代 %d 条（本件只读 · 未写一个字节）"
          % (len(per_row), len(notes)))
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
    ap.add_argument("--state", default=os.path.join(os.path.expanduser("~"), ".zerg", "state"),
                    help="状态目录（只读）")
    ap.add_argument("--ticket", default=DEFAULT_TICKET, help="对拍哪一枚件（默认 dev_edit）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    repo, state = os.path.abspath(args.root), os.path.abspath(args.state)
    hostname = os.uname().nodename
    print(VERSION)
    if not args.no_self_test:
        if not self_test(repo, state):
            print("⛔ 自检不过（读数取不到 或 有格不过）⇒ 不给结论（rc=2）")
            return 2
    if args.self_test:
        return 0
    print("── 真目标：件 %s（状态目录 %s · 只读）· 仓根 %s" % (args.ticket, state, repo))
    return scan(repo, state, args.ticket, hostname)


if __name__ == "__main__":
    sys.exit(main())

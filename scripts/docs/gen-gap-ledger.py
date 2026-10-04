#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""gen-gap-ledger.py —— 「结转账」生成件（`缺口总账-<日期>.md`）的落盘面（设计稿 v1.3 §0 第 ③ 序）。

为什么要有它（③ = `zerg gap export` 接管结账单生成）
------------------------------------------------------
`gap export` 已能把真源出成「只列未闭」的 markdown 片段（① 序落地），但**没人把它落成文件** ⇒
文档侧的结转账仍是手写件：`check-gap-ledger-view.py` 的 R1b 判「**生成件未接管**」、R2/R3 判体积与单行
—— 门① 因此现跑 rc=1。本件就是那道**落盘面**：`gap export` 的 stdout **逐字节**落成当期结转账。

三条口径（本件与 export 各管一段，**不重叠**）
  · 内容 = `bin/zerg gap export` 的 stdout **逐字节照落** —— 本件**不生成、不排版、不增删一个字** ✓
    （所以「生成件 == 直出」是本件的 --check 判据，而不是靠人自觉）。
  · 落点名 = **现读派生**：文档仓根 = 主仓的兄弟目录 `Zerg-内部文档/`；版本线目录 = `项目文档/*` 里
    **现读含结转账**的最新版本线（与门① 同一条 `vkey` 口径）；文件名日期 = **export 人面首行里的日期**
    ⇒ 本件自己**不取钟、不掷随机数**（幂等：同一真源 + 同一代码 ⇒ 同一次跑内逐字节相同 ✓）。
  · **只写生成件那一件**：旧版账（同目录/上版目录里的历史 `缺口总账-*.md`）**一个字节不改** ✗
    —— 新版取代旧版、旧版留存（设计稿 §2 的 superseded 链）。

★ qid 面（`GAP-20260927-122` / `-130` 的治本那一半 · 2026-10-03 补）
--------------------------------------------------------------------
真源 jsonl 与结转账之间的「号」有两套：**本机序号** `GAP-YYYYMMDD-NN` 与**版账号** `Q-nnn`
（`family_gap.go:72-82` 的设计句）。二者之间**没有映射面**即 `GAP-20260926-70/-139` 那条缺口。
治本 = 真源那一行**加一格 `qid`**（机器可读的 `Q-nnn`），只强制新增行。本件**不是**那一格的写面
（写面 = Go 的 `gap add` / `gapRecord`，本件一个字都改不到它）；本件能做、且只该做的，是**读面校验**：

  · 现读真源 → 若**没有一行**带 `qid` ⇒ 只**报告**「qid 字段未启用 · 0/N 条」，**不判红**（未启用态
    不许当病 ⇒ 不许把「还没落」错读成「落坏了」）。
  · 一旦**有行**带 `qid` ⇒ 校验两条**结构**判据（不判语义）：① 形态 `^Q-\d+$` ② 全局**唯一**（同号
    两行 ⇒ 判红）。任一不过 ⇒ **rc=1**（这就是 `GAP-20260927-130` 要的那条负控：
    「qid 故意写错 ⇒ --check 退 1」）。
  · `--check`（缺省档）里**折叠**这条读数；`--emit`/`--dry-run` 里只报告、不改退码（写面不该被读面
    的账挡下）。`--qid-check` 可**单独**只跑这一面。
  · 读真源读不到 / 某行不是 JSON ⇒ **rc=2「不给结论」**（与门① 的「读不到不许当绿」同一条口径）。

三档（与 `scripts/docs/` 同类生成器同一口径：0/1/2）
  0  生成件与现跑直出**逐字节一致**（`--check`）/ 写盘成功且写后读回逐字节一致（`--emit`）
  1  判红：生成件与直出**有漂移**（谁改了生成件没重生成，或真源变了没重生成）；或 qid 面结构不合法
  2  **不给结论**：缺 `bin/zerg` / export 跑不动（非 0 退码）/ 产出里取不到日期行 / 文档仓不在盘 /
      `--qid-check` 时真源读不到
     —— 一律**不猜、不假绿**（照 `gap export` 自己的 fail-closed 口径）

用法
----
    python3 scripts/docs/gen-gap-ledger.py --check      # 判漂移（不改任何件）
    python3 scripts/docs/gen-gap-ledger.py --emit       # 落盘（唯一写面 · 只写生成件）
    python3 scripts/docs/gen-gap-ledger.py --dry-run    # 只打落点与差异（零副作用）
    python3 scripts/docs/gen-gap-ledger.py --qid-check  # 只跑 qid 读面校验（不碰生成件）
    python3 scripts/docs/gen-gap-ledger.py --self-test  # 合成夹具自检（不碰真源/真账/真文档仓）
    python3 scripts/docs/gen-gap-ledger.py --docs-root DIR --version-dir vX   # 落点覆盖（默认现读派生）
    python3 scripts/docs/gen-gap-ledger.py --source PATH   # 换真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）
"""

import argparse
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
# ★ 导出器 = **本脚本所在仓**的 `bin/zerg`（不随 `--docs-root`/`--root` 换：假仓根拿不到假导出器）
EXPORT_BIN = os.path.join(REPO_ROOT, "bin", "zerg")
EXPORT_ARGS = ("gap", "export")
EXPORT_TIMEOUT = 90
VERSION = "gen-gap-ledger.py v1.1.0"

# ── 登记面（与门 `check-gap-ledger-view.py` 同一口径 · 逐字对齐）──────────────────
DOCS_DIRNAME = "Zerg-内部文档"
LEDGER_DIR = "项目文档"
LEDGER_NAME_RE = re.compile(r"^缺口总账-(\d{8})\.md$")
#   文件名日期 = export 人面首行现读（`## 缺口真源导出（**只列未闭**） · 2026-09-26`）
EXPORT_DATE_RE = re.compile(r"·\s*(\d{4})-(\d{2})-(\d{2})\s*$", re.M)
#   豁免（与门同一条）：路径段命中 ⇒ 承接副本、不算版本线候选
EXEMPT_PATH_SEGS = ("承接自",)

# ── qid 面登记（读面 · 与门① 同一条状态目录口径）──────────────────────────────────
SOURCE_NAME = "zerg-cli-gaps.jsonl"
STATE_DIR_DEFAULT = os.path.join(os.path.expanduser("~"), ".zerg", "state")
#   qid 形态：`Q-nnn`（版账全局连续号 · 位数不钉死 3 位：将来过 999 不许误判）
QID_RE = re.compile(r"^Q-\d+$")


def run_export():
    """现跑导出器（cwd = 本脚本所在仓 · 环境原样透传 —— 与门① 跑的是**同一条**面）。"""
    try:
        p = subprocess.run([EXPORT_BIN] + list(EXPORT_ARGS), cwd=REPO_ROOT,
                           capture_output=True, text=True, timeout=EXPORT_TIMEOUT)
    except FileNotFoundError as e:
        return "no_bin", None, {"cmd": EXPORT_BIN, "msg": str(e)}
    except subprocess.TimeoutExpired:
        return "timeout", None, {"cmd": EXPORT_BIN, "msg": "超时 %ds" % EXPORT_TIMEOUT}
    if p.returncode != 0:
        return "rc", None, {"cmd": "%s %s" % (EXPORT_BIN, " ".join(EXPORT_ARGS)),
                            "rc": p.returncode, "stderr": (p.stderr or "").strip()[-400:]}
    return "ok", p.stdout, {"cmd": "%s %s" % (EXPORT_BIN, " ".join(EXPORT_ARGS)), "rc": 0}


def vkey(ver):
    """版本线排序键：`v2.5.13` ⇒ (2,5,13)（**不用字符串比** —— `v2.5.9` 那种会反序）。"""
    nums = re.findall(r"\d+", ver)
    return (tuple(int(x) for x in nums), ver)


def ledger_candidates(docs):
    """文档仓里 `项目文档/<版>/**/缺口总账-<8 位>.md`（未豁免）⇒ 候选清单。"""
    base = os.path.join(docs, LEDGER_DIR)
    if not os.path.isdir(base):
        return []
    out = []
    for ver in sorted(os.listdir(base)):
        vdir = os.path.join(base, ver)
        if not os.path.isdir(vdir):
            continue
        for root, _dirs, files in os.walk(vdir):
            for fn in files:
                if not LEDGER_NAME_RE.match(fn):
                    continue
                rel = os.path.relpath(os.path.join(root, fn), docs)
                if any(seg in rel for seg in EXEMPT_PATH_SEGS):
                    continue
                out.append({"ver": ver, "rel": rel})
    return out


def pick_version_dir(docs):
    """版本线目录 = 现读含结转账的**最新**版本线（与门① 的 `vkey` 同口径）；读不到 ⇒ None。"""
    cand = ledger_candidates(docs)
    if not cand:
        return None
    return sorted(cand, key=lambda x: vkey(x["ver"]))[-1]["ver"]


def export_date(out):
    """从 export 人面首行取日期 ⇒ `YYYYMMDD`（取不到 ⇒ None ⇒ 不给结论）。"""
    m = EXPORT_DATE_RE.search(out)
    return ("%s%s%s" % (m.group(1), m.group(2), m.group(3))) if m else None


def target_path(docs, version_dir, date8):
    return os.path.join(docs, LEDGER_DIR, version_dir, "缺口总账-%s.md" % date8)


def read_bytes(path):
    if not os.path.isfile(path):
        return None
    with open(path, "rb") as f:
        return f.read()


def write_atomic(path, data):
    """写 = 临时件（固定后缀 · **不带随机量**）+ 原子 `rename`；写后读回逐字节比。"""
    tmp = path + ".tmp"
    with open(tmp, "wb") as f:
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)
    return read_bytes(path) == data


# ═════════════════════════════ qid 读面（GAP-20260927-122 / -130）═════════════════════════
def default_source():
    """真源 jsonl 缺省落点：`$ZERG_STATE_DIR/zerg-cli-gaps.jsonl` ⇒ 退回 `~/.zerg/state/...`。"""
    sd = os.environ.get("ZERG_STATE_DIR") or STATE_DIR_DEFAULT
    return os.path.join(sd, SOURCE_NAME)


def read_source_rows(path):
    """读真源逐行成 dict。读不到 / 某行不是 JSON ⇒ 返回 (None, 说明串)。"""
    try:
        with open(path, "r", encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        return None, "真源读不到：%s" % e
    rows = []
    for i, ln in enumerate(raw.split("\n"), 1):
        if not ln.strip():
            continue
        try:
            rows.append(json.loads(ln))
        except ValueError as e:
            return None, "真源第 %d 行不是 JSON（账面解读不了 ⇒ 不给结论）：%s" % (i, e)
    return rows, None


def qid_validate(rows):
    """qid 读面的**结构**校验（不判语义「有效」）。

    返回 dict：enabled（有没有一行带 qid）· total · with_qid · bad_format（[(id, qid), …]）·
    dup（[(qid, [id, …]), …]）· rc（None = 未启用不给结论 / 0 = 过 / 1 = 判红）。
    """
    total = len(rows)
    pairs = [(str(r.get("id", "")), r.get("qid")) for r in rows if r.get("qid")]
    res = {"enabled": bool(pairs), "total": total, "with_qid": len(pairs),
           "bad_format": [], "dup": [], "rc": None}
    if not pairs:
        return res
    seen = {}
    for rid, q in pairs:
        q = str(q)
        if not QID_RE.match(q):
            res["bad_format"].append((rid, q))
        seen.setdefault(q, []).append(rid)
    for q, ids in sorted(seen.items()):
        if len(ids) > 1:
            res["dup"].append((q, ids))
    res["rc"] = 1 if (res["bad_format"] or res["dup"]) else 0
    return res


def qid_verdict(q):
    """把 qid 校验结果写成一句人面判词（+ rc）。"""
    if not q["enabled"]:
        return "qid 字段未启用 · 0/%d 条带 qid（未启用态不判红）" % q["total"]
    bad = ["%s→%s" % (i, x) for i, x in q["bad_format"]]
    dups = ["%s×%d" % (x, len(ids)) for x, ids in q["dup"]]
    if q["rc"] == 0:
        return "qid 面过 · %d/%d 条带 qid（形态合法 · 号唯一）" % (q["with_qid"], q["total"])
    return "qid 面判红 · 形态非法 %d 处（%s）· 号重复 %d 处（%s）" % (
        len(bad), " ".join(bad[:8]) or "-", len(dups), " ".join(dups[:8]) or "-")


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="结转账生成件落盘面：`zerg gap export` 直出 ⇒ `缺口总账-<日期>.md`")
    ap.add_argument("--emit", action="store_true", help="落盘（唯一写面 · 只写生成件那一件）")
    ap.add_argument("--check", action="store_true", help="只判：生成件 == 现跑直出？（不改任何件）")
    ap.add_argument("--dry-run", action="store_true", help="只打落点与差异（零副作用）")
    ap.add_argument("--qid-check", action="store_true", help="只跑 qid 读面校验（不碰生成件）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认 = 主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--version-dir", default=None, help="版本线目录（默认真源：现读含结转账的最新版本线）")
    ap.add_argument("--source", default=None, help="真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--self-test", action="store_true", help="合成夹具自检（不碰真源/真账/真文档仓）")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()

    # ── 只跑 qid 面（独立档：不读生成件、不跑导出器）────────────────────────────
    if a.qid_check:
        return qid_check_mode(a)

    mode = "emit" if a.emit else ("check" if a.check else ("dry" if a.dry_run else "check"))

    docs = os.path.abspath(a.docs_root or os.path.join(os.path.dirname(REPO_ROOT), DOCS_DIRNAME))
    ver = a.version_dir or (pick_version_dir(docs) if os.path.isdir(docs) else None)

    def bail(what, read):
        if a.json:
            print(json.dumps({"schema": "zerg/v1", "kind": "GapLedgerGen", "tool": VERSION,
                              "items": [{"rule": "R0", "state": "problem"}],
                              "meta": {"mode": mode, "docs_root": docs, "version_dir": ver,
                                       "export_bin": EXPORT_BIN},
                              "reds": [], "problems": [{"rule": "R0", "what": what, "read": read}],
                              "reports": [], "warnings": [], "truncated": False}, ensure_ascii=False))
        print("⛔ 不给结论（rc=2）：%s" % what)
        for k in ("cmd", "msg", "stderr", "path"):
            if read.get(k):
                print("   %s: %s" % (k, str(read[k]).replace("\n", " ")[:200]))
        return 2

    if not os.path.isdir(docs):
        return bail("文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）" % docs, {"path": docs})
    if ver is None:
        return bail("文档仓里一件 `缺口总账-*.md` 都没有 ⇒ 版本线目录判不出（不给结论）",
                    {"glob": os.path.join(docs, LEDGER_DIR, "<版>", "缺口总账-<日期>.md")})
    st, out, eread = run_export()
    if st != "ok":
        return bail("`zerg gap export` 跑不动（%s）⇒ 直出取不到（不许当绿）" % st, eread)
    out = out or ""
    date8 = export_date(out)
    if not date8:
        return bail("export 产出里取不到日期行（首行形状变了？）⇒ 文件名落点判不出", {"bytes": len(out.encode())})
    tgt = target_path(docs, ver, date8)
    want = out.encode("utf-8")
    has = read_bytes(tgt)
    drift = (has is None) or (has != want)

    # qid 读面（不跑导出器以外的东西 · 读面失败 ⇒ 只报告，只有「启用且非法」才判红）
    q = qid_probe(a.source)

    if mode == "emit":
        if not has:
            verdict = "落盘（新件）"
        elif has == want:
            verdict = "落盘（逐字节相同 · 重生成无变化）"
        else:
            verdict = "落盘（覆盖漂移件 · 旧件 %d 字节 ⇒ 新件 %d 字节）" % (len(has), len(want))
        if not write_atomic(tgt, want):
            return bail("写后读回不一致：`%s`" % tgt, {"path": tgt})
        rc = 0
    elif drift:
        rc, verdict = 1, ("生成件%s ⇒ 判红（漂移）" % ("不在盘" if has is None else
                                                    "与直出不符（件 %d 字节 vs 直出 %d 字节）"
                                                    % (len(has), len(want))))
    else:
        rc, verdict = 0, "生成件 == 直出（逐字节相同 · %d 字节）" % len(want)

    # ★ 只 check 档折叠 qid 面（写面/干跑不许被读面账挡下）
    qid_rc = None
    if q.get("enabled") and q.get("rc") == 1:
        qid_rc = 1
        if mode == "check":
            rc = 1

    rel = os.path.relpath(tgt, os.path.dirname(docs))
    if a.json:
        reds = ([{"rule": "R1", "what": verdict, "read": {"target": tgt}}] if (drift and mode != "emit") else [])
        if qid_rc == 1 and mode == "check":
            reds.append({"rule": "R2", "what": qid_verdict(q), "read": {"qid": q}})
        print(json.dumps({"schema": "zerg/v1", "kind": "GapLedgerGen", "tool": VERSION,
                          "items": [{"rule": "R0", "state": "red" if rc == 1 else "ok"},
                                    {"rule": "R2", "state": "red" if qid_rc == 1 else
                                     ("ok" if q.get("enabled") and q.get("rc") == 0 else "report")}],
                          "meta": {"mode": mode, "docs_root": docs, "version_dir": ver,
                                   "target": tgt, "rel": rel, "bytes": len(want),
                                   "export_bin": EXPORT_BIN, "export_cmd": eread["cmd"],
                                   "qid": {"enabled": q.get("enabled"), "with_qid": q.get("with_qid"),
                                           "total": q.get("total"), "rc": q.get("rc"),
                                           "probe": q.get("probe")}},
                          "reds": reds,
                          "problems": ([{"rule": "R2", "what": q["problem"], "read": {}}]
                                       if q.get("problem") else []),
                          "reports": [{"rule": "-", "what": verdict, "read": {}},
                                      {"rule": "R2", "what": qid_verdict(q), "read": {}}],
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    print("── 结转账生成件（%s · 设计稿 v1.3 §0 第 ③ 序）──" % VERSION)
    print("  导出器 : %s %s（**现跑**）" % (EXPORT_BIN, " ".join(EXPORT_ARGS)))
    print("  文档仓 : %s" % docs)
    print("  落点   : %s（%d 字节 · 版本线 %s）" % (rel, len(want), ver))
    print("  模式   : %s（%s）" % (mode, "写盘" if mode == "emit" else "只判/只打 · 不改任何件"))
    print("%s %s" % ("✓" if rc == 0 else "✗", verdict))
    print("  qid    : %s（真源 `%s`）" % (qid_verdict(q), q.get("probe")))
    return rc


def qid_probe(source):
    """读面探针：任何读不到 ⇒ 记 problem、**不判红**（返回的 dict 里 enabled=False）。"""
    path = os.path.abspath(source or default_source())
    rows, err = read_source_rows(path)
    if err:
        return {"enabled": False, "total": 0, "with_qid": 0, "bad_format": [], "dup": [],
                "rc": None, "probe": path, "problem": err}
    q = qid_validate(rows)
    q["probe"] = path
    return q


def qid_check_mode(a):
    """`--qid-check`：只跑 qid 读面。读不到 ⇒ rc=2；未启用 ⇒ rc=0（报告）；非法 ⇒ rc=1。"""
    path = os.path.abspath(a.source or default_source())
    rows, err = read_source_rows(path)
    if err:
        if a.json:
            print(json.dumps({"schema": "zerg/v1", "kind": "GapQidCheck", "tool": VERSION,
                              "items": [{"rule": "R2", "state": "problem"}],
                              "meta": {"source": path}, "reds": [],
                              "problems": [{"rule": "R2", "what": err, "read": {}}],
                              "reports": [], "warnings": [], "truncated": False}, ensure_ascii=False))
        else:
            print("⛔ 不给结论（rc=2）：%s" % err)
        return 2
    q = qid_validate(rows)
    rc = 0 if not q["enabled"] else q["rc"]
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GapQidCheck", "tool": VERSION,
                          "items": [{"rule": "R2", "state": "red" if rc == 1 else "ok"}],
                          "meta": {"source": path, "total": q["total"], "with_qid": q["with_qid"],
                                   "enabled": q["enabled"]},
                          "reds": ([{"rule": "R2", "what": qid_verdict(q),
                                     "read": {"bad_format": q["bad_format"], "dup": q["dup"]}}]
                                   if rc == 1 else []),
                          "problems": [], "reports": [{"rule": "R2", "what": qid_verdict(q), "read": {}}],
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    print("── qid 读面（GAP-20260927-122 / -130）──")
    print("  真源   : %s" % path)
    print("%s %s" % ("✓" if rc == 0 else "✗", qid_verdict(q)))
    return rc


# ═════════════════════════════════════ 自检（合成夹具 · 不碰真源/真账/真文档仓）════════════════
def self_test():
    """测**纯函数**（版本线挑选 · 日期取读 · qid 校验）—— 不跑真导出器、不碰真文档仓。"""
    import shutil
    import tempfile
    ok = True

    def case(name, got, want):
        nonlocal ok
        good = got == want
        ok = ok and good
        print("  %s %s（got=%r · want=%r）" % ("✓" if good else "✗", name, got, want))

    tmp = tempfile.mkdtemp(prefix="gap-ledger-selftest-")
    try:
        docs = os.path.join(tmp, DOCS_DIRNAME)
        for rel in ("v2.5.12/缺口总账-20260923.md", "v2.5.9/缺口总账-20260901.md",
                    "v2.5.13/缺口总账-20260925.md", "v2.5.12/承接自-v2.5.11/缺口总账-20260922.md"):
            p = os.path.join(docs, LEDGER_DIR, rel)
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "w", encoding="utf-8") as f:
                f.write("# x\n")
        #   版本线 `v2.5.9` 不能因**字符串比**压过 `v2.5.13`；豁免件（承接自）不算候选
        case("版本线挑选（v2.5.13 > v2.5.9 · 豁免件不算）", pick_version_dir(docs), "v2.5.13")
        case("落点形状", os.path.relpath(target_path(docs, "v2.5.13", "20260926"), docs),
             os.path.join(LEDGER_DIR, "v2.5.13", "缺口总账-20260926.md"))
        case("日期取读（首行称日期）", export_date("## 缺口真源导出（**只列未闭**） · 2026-09-26\n"),
             "20260926")
        case("日期取读（取不到 ⇒ None · 不给结论）", export_date("## 无日期行\n"), None)
        case("空文档仓 ⇒ 版本线判不出", pick_version_dir(os.path.join(tmp, "没有此目录")), None)

        # ── qid 面：成对（未启用不红 / 合法过 / 形态坏红 / 重号红）────────────────────────
        rows_none = [{"id": "GAP-20260101-1"}, {"id": "GAP-20260101-2"}]
        q = qid_validate(rows_none)
        case("qid 未启用 ⇒ enabled=False · rc=None（不判红）", (q["enabled"], q["rc"]), (False, None))
        rows_ok = [{"id": "a", "qid": "Q-001"}, {"id": "b", "qid": "Q-002"}, {"id": "c"}]
        q = qid_validate(rows_ok)
        case("qid 合法 ⇒ rc=0 · 计 2/3", (q["enabled"], q["rc"], q["with_qid"]), (True, 0, 2))
        rows_bad = [{"id": "a", "qid": "Q1"}, {"id": "b", "qid": "q-002"}]
        q = qid_validate(rows_bad)
        case("qid 形态非法 ⇒ rc=1 · 两处点名", (q["rc"], q["bad_format"]),
             (1, [("a", "Q1"), ("b", "q-002")]))
        rows_dup = [{"id": "a", "qid": "Q-007"}, {"id": "b", "qid": "Q-007"}]
        q = qid_validate(rows_dup)
        case("qid 重号 ⇒ rc=1 · 点名 Q-007×2", (q["rc"], q["dup"]), (1, [("Q-007", ["a", "b"])]))

        # ── 负控：真源读不到（含非 JSON 行）⇒ (None, 说明) ───────────────────────────────
        bad = os.path.join(tmp, "坏真源.jsonl")
        with open(bad, "w", encoding="utf-8") as f:
            f.write('{"id":"x"}\n不是 json\n')
        rows, err = read_source_rows(bad)
        case("非 JSON 行 ⇒ 不给结论（rows=None · 有说明）",
             (rows is None, bool(err and "不给结论" in err)), (True, True))
        rows2, err2 = read_source_rows(os.path.join(tmp, "没有此件.jsonl"))
        case("真源不存在 ⇒ 不给结论", (rows2 is None, bool(err2)), (True, True))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

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

三档（与 `scripts/docs/` 同类生成器同一口径：0/1/2）
  0  生成件与现跑直出**逐字节一致**（`--check`）/ 写盘成功且写后读回逐字节一致（`--emit`）
  1  判红：生成件与直出**有漂移**（谁改了生成件没重生成，或真源变了没重生成）
  2  **不给结论**：缺 `bin/zerg` / export 跑不动（非 0 退码）/ 产出里取不到日期行 / 文档仓不在盘
      —— 一律**不猜、不假绿**（照 `gap export` 自己的 fail-closed 口径）

用法
----
    python3 scripts/docs/gen-gap-ledger.py --check      # 判漂移（不改任何件）
    python3 scripts/docs/gen-gap-ledger.py --emit       # 落盘（唯一写面 · 只写生成件）
    python3 scripts/docs/gen-gap-ledger.py --dry-run    # 只打落点与差异（零副作用）
    python3 scripts/docs/gen-gap-ledger.py --self-test  # 合成夹具自检（不碰真源/真账/真文档仓）
    python3 scripts/docs/gen-gap-ledger.py --docs-root DIR --version-dir vX   # 落点覆盖（默认现读派生）
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
VERSION = "gen-gap-ledger.py v1.0.0"

# ── 登记面（与门 `check-gap-ledger-view.py` 同一口径 · 逐字对齐）──────────────────
DOCS_DIRNAME = "Zerg-内部文档"
LEDGER_DIR = "项目文档"
LEDGER_NAME_RE = re.compile(r"^缺口总账-(\d{8})\.md$")
#   文件名日期 = export 人面首行现读（`## 缺口真源导出（**只列未闭**） · 2026-09-26`）
EXPORT_DATE_RE = re.compile(r"·\s*(\d{4})-(\d{2})-(\d{2})\s*$", re.M)
#   豁免（与门同一条）：路径段命中 ⇒ 承接副本、不算版本线候选
EXEMPT_PATH_SEGS = ("承接自",)


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


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="结转账生成件落盘面：`zerg gap export` 直出 ⇒ `缺口总账-<日期>.md`")
    ap.add_argument("--emit", action="store_true", help="落盘（唯一写面 · 只写生成件那一件）")
    ap.add_argument("--check", action="store_true", help="只判：生成件 == 现跑直出？（不改任何件）")
    ap.add_argument("--dry-run", action="store_true", help="只打落点与差异（零副作用）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认 = 主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--version-dir", default=None, help="版本线目录（默认真源：现读含结转账的最新版本线）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--self-test", action="store_true", help="合成夹具自检（不碰真源/真账/真文档仓）")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
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

    rel = os.path.relpath(tgt, os.path.dirname(docs))
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "GapLedgerGen", "tool": VERSION,
                          "items": [{"rule": "R0", "state": "red" if rc == 1 else "ok"}],
                          "meta": {"mode": mode, "docs_root": docs, "version_dir": ver,
                                   "target": tgt, "rel": rel, "bytes": len(want),
                                   "export_bin": EXPORT_BIN, "export_cmd": eread["cmd"]},
                          "reds": ([{"rule": "R1", "what": verdict, "read": {"target": tgt}}]
                                   if rc == 1 else []),
                          "problems": [], "reports": [{"rule": "-", "what": verdict, "read": {}}],
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    print("── 结转账生成件（%s · 设计稿 v1.3 §0 第 ③ 序）──" % VERSION)
    print("  导出器 : %s %s（**现跑**）" % (EXPORT_BIN, " ".join(EXPORT_ARGS)))
    print("  文档仓 : %s" % docs)
    print("  落点   : %s（%d 字节 · 版本线 %s）" % (rel, len(want), ver))
    print("  模式   : %s（%s）" % (mode, "写盘" if mode == "emit" else "只判/只打 · 不改任何件"))
    print("%s %s" % ("✓" if rc == 0 else "✗", verdict))
    return rc


# ═════════════════════════════════════ 自检（合成夹具 · 不碰真源/真账/真文档仓）════════════════
def self_test():
    """只测**纯函数**（版本线挑选 · 日期取读）—— 不跑真导出器、不碰真文档仓。"""
    import tempfile
    ok = True

    def case(name, got, want):
        nonlocal ok
        good = got == want
        ok = ok and good
        print("  %s %s（got=%r · want=%r）" % ("✓" if good else "✗", name, got, want))

    tmp = tempfile.mkdtemp(prefix="gap-ledger-selftest-")
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
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

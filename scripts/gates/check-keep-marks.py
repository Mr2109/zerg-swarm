#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-keep-marks.py —— 案卷 `keep` 列（`R21` · **只增不减 · 不接闸**）· 只提示

真源（**逐字**，不转述）
----------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:63-65`（`T9`）逐字：
  「### `T9` · `R21` 案卷 `keep` 列（只增不减 · **不接闸**）
   - **判据**：① 每条 `keep` 标记**必带「理由」**；缺理由那条**仍计命中**（rc 不变 ✓）
     ② **不接闸**（不参与放行判定 ✓）③ 起点真空格：`ui/` 现读 24 处 `#[allow(dead_code)]`
     （`ui/src/api.rs` 8 处）· Go 侧 0 处 ⇒ 只提示」
  「- **落点**：案卷件 + `scripts/gates/` 1 枚」
`拍板清单-影响面-v1.6-20260922.md:350` 逐字：
  「**在案卷里加一个只增不减的 `keep` 列**」·「与已治好的「内容锚」同路（不钉行号 ⇒ 不漂）；
    Mr2109 标过 `keep` 的件机器不得动，落成一行成本、长效免问」
`设计-变更影响面-v1.6.md:728` 逐字：「**立，但先只做提示**（理由**必填**）」

判据（五条 · 机器可判）
----------------------
  `M1` **命中不许隐藏**：扫到的每一条标记都进 `hits[]`（**逐条列出**），一条都不许静默丢。
  `M2` **理由必填 · 缺理由仍计命中**：案卷 `entries[]` 里按「件 + 内容锚」登记且 `reason` 非空的
       ⇒ 算「带理由」；**其余一律算「缺理由」，但照样计命中**；
       且 **rc 不变** —— 缺理由**不改退码**（本件不接闸）。
  `M3` **不接闸**：本件**不参与放行判定**。任何想把它接进放行判定的旗标
       （`--block` / `--enforce` / `--fail-on-missing`）**一律拒 rc=2**。
       退码只由「**案卷件自身好坏**」决定 ⇒ 与代码里标记的命中数**无关**。
  `M4` **只增不减**：`seq` 严格递增且不重号 · `history_removed` 必须为空（删条 = 红）。
  `M5` **内容锚唯一**：每条登记的 `anchor` 必须在**它指的那件里恰好命中 1 次**
       （0 次 = 锚漂了 / >1 次 = 锚不唯一 ⇒ 红）；**不钉行号**（行号只作导航）。
  附加现读（**不改退码**）：扫到的处数 vs 案卷 `baseline`（起点对账）—— 漂了照实报。

退码（**注意**：只提示档 —— 代码里有标记、或标记缺理由，都**不改退码**）
------------------------------------------------------------------------
  0 = 跑出来了（**哪怕 24 处全都缺理由也还是 0** —— 这正是「缺理由那条仍计命中（rc 不变）」的落点）
  1 = **案卷件自身坏了**（删条 / `seq` 乱 / 锚 0 或 >1 命中 / 登记缺 `reason` 字段）
  2 = **不给结论**：案卷件读不到 / JSON 坏 / 判据域不存在（`ui/` 与 `core/` 都没有）/
      `--self-test` 不过 / **想接闸**（`--block` 一类旗标）

用法
----
    python3 scripts/gates/check-keep-marks.py                    # 只提示：逐条列出标记 + 缺理由点名
    python3 scripts/gates/check-keep-marks.py --json rc,counts
    python3 scripts/gates/check-keep-marks.py --list-rules
    python3 scripts/gates/check-keep-marks.py --self-test        # 正/负成对（含「删条」「锚漂」两条真红）

**不接闸**（本批：波③ 只产件、不接线 ✗）：本件**未**接进 `scripts/gates/precommit-gates.sh`；
挂闸与登记（`scripts/公开标记.tsv` · 两器）按单写入者纪律留给波④收口 —— 且**R21 的口径本身就是「只提示」**。
"""

import argparse
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
DOSSIER = os.path.join(HERE, "keep-marks-registry.json")

# 扫面：排除构建目录（`ui/target/**` 里 rav1e 生成的 `out/built.rs` 不是本仓源码）
PRUNE_DIRS = {".git", "target", "node_modules", "vendor", "dist", ".venv", "venv"}
RUST_MARK = re.compile(r"allow\s*\([^)]*\bdead_code\b")
GO_MARK = re.compile(r"(//\s*lint:ignore\s+U1000|//\s*nolint|//\s*noinspection)")
GO_INLINE_REASON = re.compile(r"//\s*lint:ignore\s+U1000\s+(.+)")
REFUSE_FLAGS = ("--block", "--enforce", "--fail-on-missing", "--参与放行")


def scan_marks(root=None):
    """现读扫面 → [ {kind,file,line,anchor,inline_reason} ]（**排序稳定** · 逐条不许丢）。"""
    root = root or REPO_ROOT
    out = []
    for domain, exts, rx, kind in (("ui", (".rs",), RUST_MARK, "rust-allow-dead-code"),
                                   ("core", (".go",), GO_MARK, "go-ignore")):
        base = os.path.join(root, domain)
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = sorted(d for d in dirnames if d not in PRUNE_DIRS)
            for fn in sorted(filenames):
                if not fn.endswith(exts):
                    continue
                p = os.path.join(dirpath, fn)
                try:
                    lines = open(p, encoding="utf-8", errors="replace").read().splitlines()
                except OSError:
                    continue
                for i, ln in enumerate(lines, 1):
                    if rx.search(ln):
                        m = GO_INLINE_REASON.search(ln)
                        out.append({"kind": kind,
                                    "file": os.path.relpath(p, root),
                                    "line": i,
                                    "anchor": ln.strip(),
                                    "inline_reason": (m.group(1).strip() if m else "")})
    return out


def judge(dossier, marks, root=None):
    """纯函数：案卷 × 现读标记 → 判定（自检与真目标同一份口径）。"""
    root = root or REPO_ROOT
    entries = (dossier or {}).get("entries", []) or []
    removed = (dossier or {}).get("history_removed", []) or []

    # 索引：件 + 内容锚 → 登记的 keep 行
    idx = {}
    for e in entries:
        idx[(e.get("file", ""), (e.get("anchor", "") or "").strip())] = e

    hits, with_reason, without_reason = [], 0, 0
    for m in marks:
        e = idx.get((m["file"], m["anchor"]))
        reason = ""
        src = ""
        if e is not None:
            reason = (e.get("reason") or "").strip()
            src = "案卷登记"
        if not reason and m.get("inline_reason"):
            reason = m["inline_reason"]
            src = "码内（`//lint:ignore U1000 <理由>` 形态）"
        row = {**m, "reason": reason, "reason_src": src, "has_reason": bool(reason)}
        if reason:
            with_reason += 1
        else:
            without_reason += 1
        hits.append(row)

    # 案卷件自身的问题（**只有这些改退码**）
    dossier_hits = []
    if removed:
        dossier_hits.append("M4 只增不减**被破**：`history_removed` 非空（%d 条）⇒ 删了行 ⇒ 红" % len(removed))
    seqs = [e.get("seq") for e in entries]
    if any(s is None for s in seqs):
        dossier_hits.append("M4 登记行缺 `seq`（只增不减的序号是它的凭据）⇒ 红")
    else:
        ints = [int(s) for s in seqs if isinstance(s, (int, str)) and str(s).isdigit()]
        if len(ints) != len(seqs) or sorted(ints) != ints or len(set(ints)) != len(ints):
            dossier_hits.append("M4 `seq` 不是严格递增 / 有重号 ⇒ 红")
    for e in entries:
        if "reason" not in e:
            dossier_hits.append("M2 登记行**没有 `reason` 字段**：%s ⇒ 红（理由必填）" % e.get("file", "?"))
            continue
        f, a = e.get("file", ""), (e.get("anchor", "") or "").strip()
        if not f or not a:
            dossier_hits.append("M5 登记行缺 `file`/`anchor`：%r ⇒ 红" % e)
            continue
        p = os.path.join(root, f)
        if not os.path.isfile(p):
            dossier_hits.append("M5 锚指向的件不在场：%s（锚 = %r）⇒ 红" % (f, a[:60]))
            continue
        n = open(p, encoding="utf-8", errors="replace").read().count(a)
        if n != 1:
            dossier_hits.append("M5 内容锚必须恰好命中 1 次：%s 里命中 %d 次（锚 = %r）⇒ 红" % (f, n, a[:60]))
    return {"hits": hits, "with_reason": with_reason, "without_reason": without_reason,
            "dossier_hits": dossier_hits, "entries": len(entries)}


def rc_of(state):
    """**不接闸**的落点：代码标记的命中数与「缺理由」**不改退码**；只有案卷件自身好坏改。"""
    if state.get("verdict") == "vacuous":
        return 2
    return 1 if state.get("dossier_hits") else 0


# ── 自检（正/负成对 · 合成案卷 × 合成标记）──────────────────────────────────────
D_DOSSIER = {"seq_max": 2, "history_removed": [],
             "entries": [{"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                          "anchor": "#[allow(dead_code)]",
                          "reason": "合成理由：对外 API 面暂时保留", "registered_at": "2026-09-23T00:00:00Z"},
                         {"seq": 2, "keep": "yes", "file": "ui/src/api.rs",
                          "anchor": "#[allow(dead_code)]", "reason": " "}]}
D_MARKS = [{"kind": "rust-allow-dead-code", "file": "ui/src/api.rs", "line": 86,
            "anchor": "#[allow(dead_code)]", "inline_reason": ""}]


def _mkfix(n):
    """合成夹具树：`ui/src/api.rs` 里让锚恰好出现 n 次（验 M5 的 1 次口径）。"""
    import tempfile
    d = tempfile.mkdtemp(prefix="zerg-keep-fix-")
    os.makedirs(os.path.join(d, "ui", "src"))
    open(os.path.join(d, "ui", "src", "api.rs"), "w", encoding="utf-8").write(
        "".join("#[allow(dead_code)]\nfn f%d() {}\n" % i for i in range(n)))
    return d


def self_test():
    ok, bad, log = 0, [], []

    def check(name, got, want):
        nonlocal ok
        if got == want:
            ok += 1
            log.append("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            log.append("   ✗ %s ⇒ 期望 %s，实得 %s" % (name, want, got))

    fix1, fix2 = _mkfix(1), _mkfix(2)
    d = dict(D_DOSSIER)
    base = judge(d, D_MARKS, fix1)
    # ① 命中不许隐藏：扫到 N 条 ⇒ hits 就是 N 条
    check("M1 命中不许隐藏（扫到 1 条 ⇒ hits 1 条）", len(base["hits"]), 1)
    # ② 缺理由仍**计命中**（进 hits · 不被丢弃）
    no_reason = judge({**d, "entries": [D_DOSSIER["entries"][1]]}, D_MARKS, fix1)
    check("M2 缺理由那条**仍计命中**（hits 仍是 1 条）", len(no_reason["hits"]), 1)
    check("M2 缺理由 ⇒ without_reason 计数 +1", no_reason["without_reason"], 1)
    # ③ rc 不变：全带理由 与 全缺理由，退码**同值**（且案卷件自身无红时都是 0）
    with_r = judge({**d, "entries": [{"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                                      "anchor": "#[allow(dead_code)]", "reason": "理由"}]},
                   D_MARKS, fix1)
    check("M2 **rc 不变**（带理由 rc=0 · 缺理由 rc=0）",
          (rc_of({**with_r, "verdict": "ok"}), rc_of({**no_reason, "verdict": "ok"})), (0, 0))
    check("M2 **rc 不变** 的同值性（两者相等，与具体值无关）",
          rc_of({**with_r, "verdict": "ok"}) == rc_of({**no_reason, "verdict": "ok"}), True)
    # ④ 只增不减：删条 ⇒ 红（**这是真红**）
    rm = judge({**d, "history_removed": [{"seq": 1, "file": "ui/src/api.rs"}]}, D_MARKS, fix1)
    check("M4 负控 · `history_removed` 非空 ⇒ rc=1（真红）", rc_of({**rm, "verdict": "ok"}), 1)
    # ⑤ 序号乱 ⇒ 红
    bad_seq = judge({**d, "entries": [{"seq": 2, "keep": "yes", "file": "ui/src/api.rs",
                                       "anchor": "#[allow(dead_code)]", "reason": "r"},
                                      {"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                                       "anchor": "#[allow(dead_code)]", "reason": "r"}]},
                    D_MARKS, fix1)
    check("M4 负控 · `seq` 非严格递增 ⇒ rc=1（真红）", rc_of({**bad_seq, "verdict": "ok"}), 1)
    # ⑥ 内容锚不唯一 ⇒ 红（夹具里锚出现 2 次）
    dup = judge({**d, "entries": [{"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                                   "anchor": "#[allow(dead_code)]", "reason": "r"}]},
                D_MARKS, fix2)
    check("M5 负控 · 锚命中 2 次 ⇒ rc=1（锚不唯一 · 真红）", rc_of({**dup, "verdict": "ok"}), 1)
    # ⑦ 锚漂了（0 命中）⇒ 红
    gone = judge({**d, "entries": [{"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                                    "anchor": "#[allow(dead_code)] // 这条锚不存在",
                                    "reason": "r"}]}, D_MARKS, fix1)
    check("M5 负控 · 锚 0 命中 ⇒ 红", any("恰好命中 1 次" in h for h in gone["dossier_hits"]), True)
    # ⑧ 登记行缺 reason 字段 ⇒ 红（与「理由写成空白」区别开）
    miss = judge({**d, "entries": [{"seq": 1, "keep": "yes", "file": "ui/src/api.rs",
                                    "anchor": "#[allow(dead_code)]"}]}, D_MARKS, fix1)
    check("M2 负控 · 登记行缺 `reason` 字段 ⇒ rc=1（真红）", rc_of({**miss, "verdict": "ok"}), 1)
    # ⑨ 空转：域不存在 ⇒ 不给结论
    check("空转 · 判据域不存在 ⇒ rc=2", rc_of({"verdict": "vacuous"}), 2)
    # ⑩ 不接闸：想接闸的旗标必须被拒（端到端跑一次自己）
    try:
        p = subprocess.run([sys.executable, os.path.abspath(__file__), "--block"],
                           capture_output=True, text=True, timeout=120, cwd=REPO_ROOT)
        check("M3 负控 · `--block`（想接闸）⇒ rc=2 被拒", p.returncode, 2)
    except Exception as e:
        check("M3 负控 · `--block` 端到端", "起不来：%s" % e, 2)

    for ln in log:
        print(ln)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print("案卷 `keep` 列（`R21` · 只增不减 · **不接闸**）· 真源：任务清单-缺口收口-20260923.md:63-65")
    print("M1 命中不许隐藏（逐条列出）· M2 理由必填、**缺理由仍计命中 且 rc 不变**")
    print("M3 **不接闸**（不参与放行判定；%s 一律拒 rc=2）" % " / ".join(REFUSE_FLAGS[:3]))
    print("M4 只增不减（`seq` 严格递增 · `history_removed` 必须为空）")
    print("M5 内容锚恰好命中 1 次（**不钉行号**）")
    print("标记形态：Rust `#[allow(dead_code)]`（ui/**/*.rs）· Go `//lint:ignore U1000 <理由>`（core/**/*.go）")
    print("扫描排除：%s" % " · ".join(sorted(PRUNE_DIRS)))
    print("案卷件：%s" % os.path.relpath(DOSSIER, REPO_ROOT))
    print("退码：0 跑出来（含「24 处全缺理由」）· 1 案卷件自身坏了 · 2 不给结论（缺件/想接闸/自检不过）")
    return 0


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['rc', 'counts', 'baseline', 'dossier_hits', 'hits']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--dossier", default=DOSSIER)
    ap.add_argument("--root", default=REPO_ROOT)
    ap.add_argument("--json", default="")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--block", "--enforce", "--fail-on-missing", dest="refuse", action="store_true",
                    help="（**故意不支持**）想把本列接进放行判定 ⇒ 一律拒 rc=2")
    a = ap.parse_args()

    if a.refuse:
        print("拒绝：本列**不接闸**（不参与放行判定）—— 逐字判据：② **不接闸**（不参与放行判定 ✓）")
        print("⇒ rc=2（接了闸就把「只提示」变了味：R21 的口径是「立，但先只做提示」）")
        return 2

    if a.list_rules:
        return list_rules()
    if a.self_test:
        print("── 案卷 keep 列（R21）· 自检（正/负成对 · 含「删条」「锚漂」两条真红）──")
        return self_test()

    if not os.path.isfile(a.dossier):
        print("缺件：案卷件读不到（%s）⇒ **不给结论**（缺件不许静默放绿）" % a.dossier)
        print("⇒ rc=2")
        return 2
    try:
        dossier = json.load(open(a.dossier, encoding="utf-8"))
    except Exception as e:
        print("案卷件 JSON 解析失败（%s）⇒ **不给结论**" % e)
        print("⇒ rc=2")
        return 2

    marks = scan_marks(a.root)
    if not os.path.isdir(os.path.join(a.root, "ui")) and not os.path.isdir(os.path.join(a.root, "core")):
        print("判据域不存在（`ui/` 与 `core/` 都没有）⇒ **不给结论**（空转不许当绿）")
        print("⇒ rc=2")
        return 2

    st = judge(dossier, marks, a.root)
    rc = rc_of({**st, "verdict": "ok"})
    base = dossier.get("baseline", {}) or {}
    ui_n = len([m for m in st["hits"] if m["kind"] == "rust-allow-dead-code"])
    go_n = len([m for m in st["hits"] if m["kind"] == "go-ignore"])
    api_n = len([m for m in st["hits"] if m["file"] == "ui/src/api.rs"])

    print("── 案卷 `keep` 列（`R21`）· **只提示**（不接闸 · 不参与放行判定）──")
    print("案卷件：%s（登记 %d 行 · `history_removed` %d 条）"
          % (os.path.relpath(a.dossier, REPO_ROOT), st["entries"], len(dossier.get("history_removed") or [])))
    print("扫面（排除构建目录 %s）：" % " · ".join(sorted(PRUNE_DIRS)))
    print("   ui/**/*.rs  `#[allow(dead_code)]`：%d 处（`ui/src/api.rs` %d 处）" % (ui_n, api_n))
    print("   core/**/*.go `//lint:ignore U1000` / `nolint` / `noinspection`：%d 处" % go_n)
    if base:
        print("   起点对账（案卷 `baseline` · **漂了只照实报**，不改退码）：ui 记 %s · api.rs 记 %s · Go 记 %s"
              % (base.get("ui_allow_dead_code_处"), base.get("ui_src_api_rs_处"), base.get("go_处")))
    print("")
    print("── 逐条（**命中不许隐藏** · 一条一行）──")
    for m in st["hits"]:
        tag = "带理由" if m["has_reason"] else "**缺理由**"
        print("   %s:%d  [%s]  %s%s"
              % (m["file"], m["line"], tag, m["anchor"][:78],
                 ("  ← " + m["reason"][:40] + "（%s）" % m["reason_src"]) if m["has_reason"] else ""))
    print("")
    print("── 判据对账 ──")
    print("   M1 命中不许隐藏：扫到 %d 条 ⇒ 列出 %d 条" % (len(marks), len(st["hits"])))
    print("   M2 理由必填：带理由 %d 条 · **缺理由 %d 条**（缺理由**仍计命中** ✓；**rc 不变** ✓ —— 缺理由不改退码）"
          % (st["with_reason"], st["without_reason"]))
    print("   M3 **不接闸**：本件不参与放行判定（退码只看「案卷件自身好坏」，与标记命中数无关）")
    print("   M4 只增不减：`history_removed` %d 条 · 登记 %d 行"
          % (len(dossier.get("history_removed") or []), st["entries"]))
    if st["dossier_hits"]:
        print("   ✗ 案卷件自身的红（只有这些改退码）：")
        for h in st["dossier_hits"]:
            print("      ✗ " + h)
    else:
        print("   ✓ 案卷件自身无红")
    if st["without_reason"]:
        print("   ⇒ **只提示**：有 %d 条标记缺理由（起点真空格：%s）—— 照实登记，不接闸、不拦提交"
              % (st["without_reason"],
                 "Go 侧现读 0 处" if go_n == 0 else "Go 侧现读 %d 处" % go_n))
    print("⇒ rc=%d" % rc)

    if a.json:
        want = [f.strip() for f in a.json.split(",") if f.strip()]
        obj = {"rc": rc, "counts": {"ui": ui_n, "ui_api_rs": api_n, "go": go_n,
                                    "with_reason": st["with_reason"], "without_reason": st["without_reason"]},
               "baseline": base, "dossier_hits": st["dossier_hits"],
               "hits": [{"file": m["file"], "line": m["line"], "anchor": m["anchor"],
                         "has_reason": m["has_reason"], "reason_src": m["reason_src"]} for m in st["hits"]]}
        print(json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main())

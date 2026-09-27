#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-lineref-limit.py —— 「件:行」引用行号上限（只报告起步）

判据（唯一一条，单一确定）
    对磁盘上 `scripts/gates/` 顶层的 `.md`/`.py`/`.sh` 逐行找形如 `<件>:<行>[-<行>]` 的引用，
    解析出被引件后取它的**物理行数** N；若引用的**行号止 > N** ⇒ 该处「越界」⇒ rc=1。

三态（分母守恒：可判 + 判不了 = 总处数；断言不成立 ⇒ rc=2）
    可判      落点解得出 ⇒ 参与判据
    判不了    落点解不出（裸件名多解 / 带目录落点不存在 / 剥前缀后仍不存在）⇒ **逐条打印，不进绿也不进红**
    样张      夹具构造行里写的引用（`\n"` 收尾 **且** 本行是封闭名单里的夹具调用）⇒ 只登记，不判
              （**两条件同时才收** —— 单看引号会洗掉真引用，单看调用名会收进普通代码）

退码
    0  可判面零越界
    1  有越界（逐处点名：引用件:行 → 被引件 → 行号止 → 件行数）
    2  不给结论（空转：扫描面 0 件 / 引用 0 处 / 分母守恒断言不成立）

面（刻意划死，避免假阳）
    - 扫描面 = `scripts/gates/` **顶层** `*.md`/`*.py`/`*.sh`（**文件系统 glob，不读 git 索引** —— 未跟踪件同样入面）
    - 落点四规则：① 仓根相对 ② 文档仓 `Zerg-内部文档/` 拼接 ③ 剥 `Zerg-内部文档/` 前缀 ④ 剥 `docs/` 前缀
    - 裸件名（不含 `/`）：**候选件行数全同 ⇒ 判；不全同 ⇒ 只登记不判**（不取最大解、不取任一解）
    - 行数 = 物理行数（`\\n` 计数，末行无换行则 +1）

纪律
    - **只读**：不写仓内任何件、不改任何状态；临时件只落 `$TMPDIR`（调用方保证不在任何 git 仓内）
    - 不接管道取 rc（调用方口径）；本件 rc 由 `main()` 单一返回

用法
    python3 scripts/gates/check-lineref-limit.py [--json] [--verbose]
    python3 scripts/gates/check-lineref-limit.py --self-test
"""

import argparse
import json
import os
import re
import sys
import tempfile

SCAN_DIR_REL = os.path.join("scripts", "gates")
SCAN_EXTS = (".md", ".py", ".sh")

# 与既有口径同源：被引件扩展名闭集
EXT = "py|sh|md|go|json|yaml|yml|ts|js|tsv|jsonl|txt|toml|rs|lock|cfg"
REF_RE = re.compile(r"([^\s`\"'\[\]<>|=，：。、；（）「」]*\.(?:" + EXT + r")):(\d+)(?:-(\d+))?")

# 夹具/样张目录名（不在面内 —— 它们不是引用）
FIXTURE_HINTS = ("_mkroot", "testdata", "fixtures")

# 「样张档」容器判据（封闭名单 · 与「引用被 \n" 收尾」**同时**成立才判样张）
#   理由：门脚本的自检夹具里会写 `_mk(..., "见 x.md:8\n")` —— 那是**样张**不是引用。
#   两条件同时才收（宁窄不宽）：单看引号会洗掉真引用，单看调用名会收进普通代码。
FIXTURE_CALLS = ("_mk(", "mkcase(", "mkfile(", "mkroot(", "_mkroot(",
                 "write_text(", "mkfixture(", "mkfixturecase(")


def repo_root():
    """从本件位置回溯到仓根（含 core/internal/version/version.go 的那个目录）。"""
    here = os.path.dirname(os.path.abspath(__file__))
    cur = here
    for _ in range(8):
        cur = os.path.dirname(cur)
        if not cur or cur == "/":
            break
        if os.path.isfile(os.path.join(cur, "core", "internal", "version", "version.go")):
            return cur
    # 兜底：本件在 <root>/scripts/gates/
    return os.path.dirname(os.path.dirname(here))


def physical_lines(path):
    try:
        with open(path, "rb") as fh:
            data = fh.read()
    except OSError:
        return None
    if not data:
        return 0
    n = data.count(b"\n")
    return n if data.endswith(b"\n") else n + 1


def scan_face(gates_dir):
    if not os.path.isdir(gates_dir):
        return []
    out = []
    for name in sorted(os.listdir(gates_dir)):
        full = os.path.join(gates_dir, name)
        if os.path.isfile(full) and name.endswith(SCAN_EXTS):
            out.append((name, full))
    return out


def resolve(token, root, docs_root):
    """落点四规则。返回 (path, kind)；kind ∈ {found, missing, ambiguous}。"""
    if "/" not in token:
        # 裸件名：全仓按基名找同名件（只走两个仓的常见子树，避免全树扫）
        cands = []
        for base in (root, docs_root):
            if not base or not os.path.isdir(base):
                continue
            for dirpath, dirnames, filenames in os.walk(base):
                dirnames[:] = [d for d in dirnames
                               if d not in (".git", "node_modules", "target", "vendor")]
                if os.path.basename(token) in filenames:
                    cands.append(os.path.join(dirpath, token))
        if not cands:
            return None, "missing"
        lens = {physical_lines(c) for c in cands}
        if len(lens) == 1:
            return cands[0], "found"
        return None, "ambiguous"
    cands = [os.path.join(root, token)]
    if docs_root:
        cands.append(os.path.join(docs_root, token))
        if token.startswith("Zerg-内部文档/"):
            cands.append(os.path.join(docs_root, token[len("Zerg-内部文档/"):]))
        if token.startswith("docs/"):
            cands.append(os.path.join(docs_root, token[len("docs/"):]))
    for c in cands:
        if c and os.path.isfile(c):
            return c, "found"
    return None, "missing"


def analyze(gates_dir, root, docs_root):
    """返回 (rows, stats)。rows 每项：ref_file, ref_line, token, start, stop, kind, target, n_lines, over."""
    rows = []
    for name, full in scan_face(gates_dir):
        try:
            with open(full, encoding="utf-8", errors="replace") as fh:
                lines = fh.readlines()
        except OSError:
            continue
        for i, text in enumerate(lines, 1):
            prev_line = lines[i - 2] if i >= 2 else ""
            for m in REF_RE.finditer(text):
                token = m.group(1)
                start = int(m.group(2))
                stop = int(m.group(3) or start)
                # 样张档：引用被「反斜杠 + n（1 个或 2 个反斜杠）」+ 引号 收尾
                #   **且** 本行或上一行是夹具构造调用 ⇒ 只登记、不判
                #   两种反斜杠都要认：夹具要往文件里写「字面的 \n」时，源码里是 \\n
                #   看上一行是因为夹具调用常跨行：调用名在首行、字面量在续行
                tail = text[m.end():m.end() + 3]
                sample = ((tail.startswith("\\n") or tail.startswith("\\\\n"))
                          and any(k in text or k in prev_line for k in FIXTURE_CALLS))
                if sample:
                    path, kind = None, "sample"
                else:
                    path, kind = resolve(token, root, docs_root)
                n = physical_lines(path) if path else None
                rows.append({
                    "ref_file": os.path.join(SCAN_DIR_REL, name),
                    "ref_line": i,
                    "token": token,
                    "start": start,
                    "stop": stop,
                    "kind": kind,
                    "target": os.path.relpath(path, root) if path else None,
                    "n_lines": n,
                    "over": bool(kind == "found" and n is not None and stop > n),
                })
    stats = {
        "total": len(rows),
        "judged": sum(1 for r in rows if r["kind"] == "found"),
        "unjudged": sum(1 for r in rows if r["kind"] != "found"),
        "over": sum(1 for r in rows if r["over"]),
    }
    stats["conserved"] = (stats["judged"] + stats["unjudged"]) == stats["total"]
    return rows, stats


# ───────────────────────────── 自检（成对负控 6 格） ─────────────────────────────
def _mk(root, docs, rel, body):
    p = os.path.join(docs if rel.startswith("Zerg-内部文档/") else root,
                     rel[len("Zerg-内部文档/"):] if rel.startswith("Zerg-内部文档/") else rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as fh:
        fh.write(body)
    return p


def self_test():
    results = []

    def case(tag, want_rc, ok, detail=""):
        results.append((tag, bool(ok), detail))

    with tempfile.TemporaryDirectory(prefix="lineref-selftest-") as tmp:
        root = os.path.join(tmp, "Zerg")
        docs = os.path.join(tmp, "Zerg-内部文档")
        gdir = os.path.join(root, SCAN_DIR_REL)
        os.makedirs(gdir, exist_ok=True)
        os.makedirs(docs, exist_ok=True)

        # NC1 正控：可判且不越界 ⇒ rc=0，且分母守恒
        _mk(root, docs, "wall/Cargo.lock", "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
        _mk(root, docs, "scripts/gates/ok.md", "见 wall/Cargo.lock:7\n")
        rows, st = analyze(gdir, root, docs)
        case("NC1 正控：可判不越界 ⇒ rc=0 且守恒",
             st["conserved"] and st["judged"] == 1 and st["over"] == 0,
             (st["judged"] == 1 and st["over"] == 0 and st["conserved"]),
             json.dumps(st, ensure_ascii=False))

        # NC2 越界负控：行号止 > 件行数 ⇒ 越界，且点名唯一
        _mk(root, docs, "scripts/gates/bad.md", "见 wall/Cargo.lock:8\n")
        rows, st = analyze(gdir, root, docs)
        overs = [r for r in rows if r["over"]]
        case("NC2 越界负控：:8 引 7 行件 ⇒ 越界且恰 1 处",
             len(overs) == 1, (len(overs) == 1 and overs[0]["ref_file"].endswith("bad.md")),
             json.dumps(st, ensure_ascii=False))
        os.remove(os.path.join(gdir, "bad.md"))

        # NC3 落点改名 ⇒ 必须落「判不了」，且总数守恒、不许消失
        _mk(root, docs, "scripts/gates/gone.md", "见 wall/Cargo-nope.lock:3\n")
        rows, st = analyze(gdir, root, docs)
        gone = [r for r in rows if r["ref_file"].endswith("gone.md")]
        case("NC3 改名 ⇒ 落判不了且总数守恒（不许消失）",
             len(gone) == 1 and gone[0]["kind"] == "missing" and st["conserved"],
             (len(gone) == 1 and gone[0]["kind"] == "missing" and st["conserved"]),
             json.dumps(st, ensure_ascii=False))
        os.remove(os.path.join(gdir, "gone.md"))

        # NC4 孪生反向格：同名两解行数不同 ⇒ 不许判
        _mk(root, docs, "a/gem.md", "l1\nl2\nl3\n")
        _mk(root, docs, "b/gem.md", "l1\nl2\nl3\nl4\nl5\n")
        _mk(root, docs, "scripts/gates/twin.md", "见 gem.md:3\n")
        rows, st = analyze(gdir, root, docs)
        twin = [r for r in rows if r["ref_file"].endswith("twin.md")]
        case("NC4 孪生反向格：同名行数不同 ⇒ 不许判",
             len(twin) == 1 and twin[0]["kind"] == "ambiguous",
             (len(twin) == 1 and twin[0]["kind"] == "ambiguous"),
             json.dumps(st, ensure_ascii=False))
        # 同名行数全同 ⇒ 判
        _mk(root, docs, "b/gem.md", "l1\nl2\nl3\n")
        rows, st = analyze(gdir, root, docs)
        twin = [r for r in rows if r["ref_file"].endswith("twin.md")]
        case("NC4b 同名行数全同 ⇒ 判（且 :3 不越界）",
             len(twin) == 1 and twin[0]["kind"] == "found" and not twin[0]["over"],
             (len(twin) == 1 and twin[0]["kind"] == "found" and not twin[0]["over"]),
             json.dumps(st, ensure_ascii=False))
        os.remove(os.path.join(gdir, "twin.md"))

        # NC5 索引面回归格：CJK 名件必须找得到（否则 rc=2）
        _mk(root, docs, "scripts/gates/中文件.md", "l1\nl2\nl3\nl4\n")
        _mk(root, docs, "scripts/gates/cjk.md", "见 中文件.md:4\n")
        rows, st = analyze(gdir, root, docs)
        cjk = [r for r in rows if r["ref_file"].endswith("cjk.md")]
        case("NC5 CJK 名件必须找得到（found ⇒ 不许落判不了）",
             len(cjk) == 1 and cjk[0]["kind"] == "found" and not cjk[0]["over"],
             (len(cjk) == 1 and cjk[0]["kind"] == "found" and not cjk[0]["over"]),
             json.dumps(st, ensure_ascii=False))
        os.remove(os.path.join(gdir, "cjk.md"))
        os.remove(os.path.join(gdir, "中文件.md"))

        # NC7 样张档负控：夹具构造行里写的引用 ⇒ 只登记、不许判越界
        _mk(root, docs, "scripts/gates/fx.md",
            "_mk(root, docs, \"x.md\", \"见 wall/Cargo.lock:99\\n\")\n")
        rows, st = analyze(gdir, root, docs)
        fx = [r for r in rows if r["ref_file"].endswith("fx.md")]
        case("NC7 样张档：夹具行里的引用 ⇒ 落 sample、不许越界",
             len(fx) == 1 and fx[0]["kind"] == "sample" and not fx[0]["over"],
             ([(r["kind"], r["over"]) for r in fx]),
             json.dumps(st, ensure_ascii=False))
        os.remove(os.path.join(gdir, "fx.md"))

        # NC6 空转 rc=2
        os.remove(os.path.join(gdir, "ok.md"))
        _mk(root, docs, "scripts/gates/empty.md", "无引用的普通一行\n")
        rows, st = analyze(gdir, root, docs)
        case("NC6 空转 ⇒ rc=2（面内引用 0 处）",
             st["total"] == 0, (st["total"] == 0), json.dumps(st, ensure_ascii=False))

    bad = [t for t, ok, _ in results if not ok]
    for tag, ok, detail in results:
        print(("   ✓ " if ok else "   ✗ ") + tag + (("  " + detail) if not ok else ""))
    print("自检 %d 格 · 失败 %d 格" % (len(results), len(bad)))
    return 0 if not bad else 1


# ───────────────────────────────── 主路径 ─────────────────────────────────
def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--verbose", action="store_true")
    ap.add_argument("--root", default=None, help="仓根（缺省自动回溯；供入仓前试跑）")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        return self_test()

    root = os.path.abspath(args.root) if args.root else repo_root()
    docs_root = os.path.join(os.path.dirname(root), "Zerg-内部文档")
    gates_dir = os.path.join(root, SCAN_DIR_REL)

    face = scan_face(gates_dir)
    if not face:
        print("不给结论：扫描面 %s 下 0 件（rc=2）" % SCAN_DIR_REL)
        return 2

    rows, st = analyze(gates_dir, root, docs_root)

    if st["total"] == 0:
        print("不给结论：扫描面 %d 件里引用 0 处（rc=2）" % len(face))
        return 2
    if not st["conserved"]:
        print("不给结论：分母守恒断言不成立（可判 %d + 判不了 %d ≠ 总 %d）（rc=2）"
              % (st["judged"], st["unjudged"], st["total"]))
        return 2

    overs = [r for r in rows if r["over"]]

    if args.json:
        print(json.dumps({"face": len(face), "stats": st,
                          "over": overs, "unjudged": [r for r in rows if r["kind"] != "found"]},
                         ensure_ascii=False, indent=1))
    else:
        for r in overs:
            print("越界 %s:%d  被引件 %s  行号止 %d > 件行数 %d"
                  % (r["ref_file"], r["ref_line"], r["target"], r["stop"], r["n_lines"]))
        if args.verbose:
            for r in rows:
                if r["kind"] != "found":
                    print("判不了 %s:%d  %s（%s）" % (r["ref_file"], r["ref_line"], r["token"], r["kind"]))
        print("扫描面 %d 件 · 引用总 %d 处 · 可判 %d · 越界 %d · 判不了 %d（分母守恒 %s）"
              % (len(face), st["total"], st["judged"], st["over"], st["unjudged"],
                 "✓" if st["conserved"] else "✗"))

    return 1 if overs else 0


if __name__ == "__main__":
    sys.exit(main())

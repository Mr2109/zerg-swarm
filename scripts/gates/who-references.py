#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""who-references.py —— 「给一件路径 ⇒ 答哪些件引用了它、引用数、多少指向已归档/已断」（2026-09-25 · C-07 读面）

病灶与口径（C-07）
------------------
门禁的机器读面缺两样：①「跑完只能人读 stdout」；② 缺「谁引用了这件」的机器读面——给一条仓内
路径，答「几处引用、几处将指向归档/已不存在的件」。本件补第 ② 条：它不重算任何门禁判据，
只回答一个**只读**问题——

    给定一条仓内路径 P（可以是件、目录、或路径前缀）⇒ 找出主仓里**所有引用了 P 的（件,行,引文）**，
    统计引用数，并逐处判断「引用目标」是不是一个**已不存在/已归档**的路径（落在同级的
    `Zerg-内部文档/` / `Zerg-归档/` 两棵仓外树里，或根本三树都没有）⇒ 标「将指向归档」或「已断」。

只读不写（铁律 ③）
------------------
本件**只读、只查、只汇总**：绝不写工作树、不写任何文件（除非你显式 `--dump <path>` 把结果落盘）。
任何非 --dump 的写意图 ⇒ rc=2 且零字节。

用法
----
    python3 scripts/gates/who-references.py <路径>                       # 人读表 + 退码
    python3 scripts/gates/who-references.py <路径> --json                # 机器读面（逐引用 + 分类）
    python3 scripts/gates/who-references.py <路径> --json-only           # 只 stdout 输出 JSON 本体
    python3 scripts/gates/who-references.py <路径> --kind file|dir|prefix # 目标形态（默认 auto）
    python3 scripts/gates/who-references.py <路径> --dump /tmp/wr.json   # 显式落盘（唯一被允许的写）
    python3 scripts/gates/who-references.py --root <仓根>               # 换查的仓（默认本仓根）
    python3 scripts/gates/who-references.py --self-test                 # 成对负控（合成夹具 · 不碰真目标）

退码：0 无待办（该路径零引用 / 或所有引用都指向仍存在的真目标）/ 1 有待办（存在引用指向
      已归档/已不存在的目标 ⇒ 「几处将指向归档」）/ 2 不给结论（路径不存在于仓 · 空转 ·
      用法错 · 任何非 --dump 的写意图 · **参数无效**：路径是空串/纯空白 —— 空串会匹配全部件
      = 空转当绿，不许当绿。甲案 `GAP-20260928-46`：值无效 / 空串 / 缺实参一律走本档 2，
      本件不再退 8（8 只给「读不到」的 BLOCKED））
"""

import argparse
import json
import os
import re
import subprocess
import sys

_HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.abspath(os.path.join(_HERE, "..", ".."))

WRITE_INTENT_FLAGS = (
    "--write", "--set", "--append", "--yes", "--reason", "--force",
    "--overwrite", "--apply", "--commit", "--push",
)

# 跳过不参与引用的子树（构建产物 / 依赖 / 运行态 —— 同 dangling-refs 门同款口径）
SKIP_DIRS = ("vendor", "bin", "dist", "node_modules", "target", ".git", ".venv", "venv",
             "docs", "ui", "mcp", "wall", "deploy", "zerg-evals", "compress_models",
             "styles", "data", "core", "agent")
SKIP_PREFIX = ("tools/ocr/venv/",)

# 一条「引用」长什么样：目标路径出现在引文里。默认盯「被反引号/引号/引号包裹或裸路径」的目标。
# 也接受「裸路径」（如注释 `见 docs/项目文档/`）—— 用 word-boundary 匹配。
RE_STR_LIT = re.compile(r'''["'`‘’“”]([^\n"'`‘’“”]{1,400})["'`‘’“”]''')
RE_WORD_BOUND = re.compile(r"(?:^|[^/\w])(%s)(?:[/\w]|$)")


def git_files(root):
    try:
        out = subprocess.run(["git", "ls-files", "-z"], cwd=root, capture_output=True)
    except Exception as e:
        print("✗ 跑不了 git ls-files：%s ⇒ 不给结论" % e, file=sys.stderr)
        return None
    if out.returncode != 0:
        print("✗ git ls-files rc=%d（不是 git 仓？）⇒ 不给结论" % out.returncode, file=sys.stderr)
        return None
    return [p for p in out.stdout.decode("utf-8", "replace").split("\0") if p]


def _read_text(path):
    try:
        raw = open(path, "rb").read()
    except Exception:
        return None
    if b"\x00" in raw:
        return None
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return None


def resolve_trees(root):
    """返回 (主仓, Zerg-内部文档, Zerg-归档) 三个根目录。"""
    parent = os.path.dirname(root)
    docs = os.environ.get("ZERG_DOCS_ROOT") or os.path.join(parent, "Zerg-内部文档")
    arch = os.environ.get("ZERG_ARCHIVE_ROOT") or os.path.join(parent, "Zerg-归档")
    return root, docs, arch


def exists_anywhere(target_rel, root, docs, arch):
    """目标（相对仓根的路径）在三棵树里是否存在 ⇒ (in_repo, found_in_tree, tree_label)。

    found_in_tree ∈ {None, "Zerg-内部文档", "Zerg-归档"}。
    """
    if target_rel.startswith("/"):  # 绝对路径：只在主仓判
        in_repo = os.path.exists(os.path.join(root, target_rel.lstrip("/")))
        return in_repo, in_repo, None if in_repo else None
    if os.path.exists(os.path.join(root, target_rel)):
        return True, True, None
    # 仓外两棵（已归档/已搬走）：允许去掉「docs/」前缀的同路匹配（同 dangling-refs 门口径）
    # found_in_tree=True 即「将指向已归档」—— C-07 的 pending 正指这里。
    rel2 = target_rel[len("docs/"):] if target_rel.startswith("docs/") else target_rel
    for label, tree in (("Zerg-内部文档", docs), ("Zerg-归档", arch)):
        if tree and os.path.isdir(tree) and os.path.exists(os.path.join(tree, target_rel)):
            return False, True, label
        if rel2 != target_rel and tree and os.path.isdir(tree) and os.path.exists(os.path.join(tree, rel2)):
            return False, True, label
    return False, False, None


def iter_targets(cand):
    """从候选目标串里拆出若干「可能路径」片段。"""
    # 优先整串（已去首尾空白）
    if cand:
        yield cand.strip()


def scan_for(root, query, kind):
    """返回引用列表：每条 = {file, line, quote, target, target_kind, found_in_tree, tree_label}。

    query 是要查的路径（已归一化为相对仓根）。kind=file 精确到件/目录；prefix 按前缀。
    """
    hits = []
    files = git_files(root)
    if not files:
        return None
    seen = set()
    for rel in files:
        parts = rel.split("/")
        if any(p == d for p in parts for d in SKIP_DIRS):
            continue
        if rel.startswith(SKIP_PREFIX):
            continue
        full = os.path.join(root, rel)
        if not os.path.isfile(full):
            continue
        text = _read_text(full)
        if text is None:
            continue
        for ln, line in enumerate(text.split("\n"), 1):
            # 引文形态：反引号/引号包裹的目标
            for m in RE_STR_LIT.finditer(line):
                cand = m.group(1)
                if _matches(cand, query, kind):
                    target = _as_target_rel(cand)
                    if target is None or not _is_within_repo(target, query):
                        continue
                    key = (rel, ln, cand)
                    if key in seen:
                        continue
                    seen.add(key)
                    in_repo, found, label = exists_anywhere(target, *_trees)
                    hits.append({"file": rel, "line": ln, "quote": cand, "target": target,
                                 "found_in_repo": in_repo, "found_in_tree": found,
                                 "tree_label": label})
            # 裸路径形态（注释 / 散文里的引用）
            for m in re.finditer(r"(?:^|\s)(%s)[\w./\-–—]*" % re.escape(query), line):
                cand = m.group(1)
                key = (rel, ln, cand)
                if key in seen:
                    continue
                seen.add(key)
                target = _as_target_rel(cand)
                if target is None:
                    continue
                in_repo, found, label = exists_anywhere(target, *_trees)
                hits.append({"file": rel, "line": ln, "quote": cand, "target": target,
                             "found_in_repo": in_repo, "found_in_tree": found,
                             "tree_label": label})
    return hits


def _matches(cand, query, kind):
    """候选目标串是否命中 query（file/dir/prefix）。"""
    c = cand.strip()
    if kind == "file":
        return c == query or c.startswith(query.rstrip("/") + "/")
    if kind == "dir":
        base = query.rstrip("/")
        return c == base or c.startswith(base + "/")
    # prefix
    return c == query or c.startswith(query.rstrip("/") + "/") or query.rstrip("/") in c


def _as_target_rel(cand):
    """把引文里的目标串归一化为相对仓根路径（去掉可能的路径前缀如 ./ ）。"""
    c = cand.strip()
    if c.startswith("./"):
        c = c[2:]
    return c


def _is_within_repo(target, query):
    """目标是否落在 query 所代表的那棵子树里（避免把无关全局字符串当引用）。

    粗略：目标以 query（去尾斜杠）为前缀，或以 query 的某一段为 word boundary 命中。
    """
    q = query.rstrip("/")
    if target == q:
        return True
    if target.startswith(q + "/"):
        return True
    # 若 query 是「docs/项目文档/」这类，目标「docs/项目文档/xxx」自然命中；
    # 若 query 是短名（如「台账」），放宽到 word boundary —— 但那样太松，这里保守：仅前缀。
    return False


def selftest():
    """成对负控：真目标 ⇒ rc=0；归档目标 ⇒ rc=1。不碰真目标。"""
    import tempfile

    tmp = tempfile.mkdtemp(prefix="wr-selftest-")
    repo = os.path.join(tmp, "Zerg")
    docs = os.path.join(tmp, "Zerg-内部文档")
    os.makedirs(os.path.join(docs, "调研"), exist_ok=True)
    os.makedirs(os.path.join(repo, "docs"), exist_ok=True)
    # 主仓里造一个真件被引用
    open(os.path.join(repo, "docs/真件.md"), "w").write("真件\n")
    open(os.path.join(repo, "readme.md"), "w").write("见 `docs/真件.md`\n")
    # 仓外造一个已归档件被引用
    open(os.path.join(docs, "调研/旧稿.md"), "w").write("旧稿\n")
    open(os.path.join(repo, "notes.md"), "w").write("旧参考 `docs/调研/旧稿.md`\n")
    # 让临时仓是 git 仓（scan_for 走 git ls-files）
    import subprocess
    subprocess.run(["git", "init", "-q"], cwd=repo, capture_output=True)
    subprocess.run(["git", "config", "user.email", "t@t"], cwd=repo, capture_output=True)
    subprocess.run(["git", "add", "-A"], cwd=repo, capture_output=True)
    subprocess.run(["git", "commit", "-q", "-m", "seed"], cwd=repo, capture_output=True)

    # ① 真目标：零引用指向已断 ⇒ rc=0
    r = run_selftest(repo, docs, "docs/真件.md")
    print("  %s [真目标] rc=%s（期望 0）" % ("✓" if r == 0 else "✗", r))
    ok = r == 0

    # ② 归档目标：引用指向已不在主仓、在 Zerg-内部文档 ⇒ rc=1
    r = run_selftest(repo, docs, "docs/调研/旧稿.md")
    print("  %s [归档目标] rc=%s（期望 1）" % ("✓" if r == 1 else "✗", r))
    ok = ok and r == 1

    # ③ 不存在路径 ⇒ rc=2 不给结论
    r = run_selftest(repo, docs, "docs/不存在.md")
    print("  %s [不存在路径] rc=%s（期望 2）" % ("✓" if r == 2 else "✗", r))
    ok = ok and r == 2
    return ok


def run_selftest(repo, docs, query):
    global _trees
    _trees = (repo, docs, os.path.join(os.path.dirname(repo), "Zerg-归档"))
    hits = scan_for(repo, query, "auto")
    if hits is None:
        return 2
    if not hits:
        return 2  # 零引用 = 不给结论
    pending = [h for h in hits if not h["found_in_repo"]]
    return 1 if pending else 0


def main(argv):
    global _trees
    ap = argparse.ArgumentParser(
        prog="who-references.py",
        description="给一件路径 ⇒ 哪些件引用了它、引用数、多少指向已归档/已断",
        add_help=True,
    )
    ap.add_argument("path", nargs="?", default=None, help="仓内路径（件 / 目录 / 路径前缀）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="查的仓根（默认本仓根）")
    ap.add_argument("--kind", choices=["auto", "file", "dir", "prefix"], default="auto",
                    help="目标形态（默认 auto：按是否存在目录优先判 file/dir）")
    ap.add_argument("--json", action="store_true", help="机器读面（= 与 --json-only 同效 · GAP-20261001-22 修：原为死旗标）")
    ap.add_argument("--json-only", action="store_true", help="只 stdout 输出 JSON 本体")
    ap.add_argument("--dump", default="", help="把结果显式落盘（唯一被允许的写行为）")
    ap.add_argument("--self-test", action="store_true", help="跑成对负控（合成夹具）")
    args = ap.parse_args(argv)

    # ● GAP-20261001-22 修：`--json` 原被 argparse 解析后**从不被读取**（死旗标 ⇒ 行为恒等于缺省 ·
    #   假承诺）。凡声明过的旗标必须有实际效果 ⇒ 令 `--json` ≡ `--json-only`（同为机器读面）。
    if args.json:
        args.json_only = True

    if args.self_test:
        ok = selftest()
        print("自检：%s（3 格：真目标 rc=0 · 归档目标 rc=1 · 不存在路径 rc=2）"
              % ("通过 ✓" if ok else "**失败** ✗"))
        return 0 if ok else 1

    for f in WRITE_INTENT_FLAGS:
        if f in argv:
            if not args.json_only:
                print("✗ %s：写意图一律拒 ⇒ rc=2 且一个字节不写" % f, file=sys.stderr)
            return 2

    if args.self_test:
        ok = selftest()
        print("自检：%s（3 格：真目标 rc=0 · 归档目标 rc=1 · 不存在路径 rc=2）"
              % ("通过 ✓" if ok else "**失败** ✗"))
        return 0 if ok else 1

    root = os.path.abspath(args.root)
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("✗ 不在仓根：%s ⇒ 拒绝静默空转" % root, file=sys.stderr)
        return 2

    # 无参 ⇒ 用法错（rc=2），不是崩：`path` 是 nargs="?" ⇒ 缺省即 None，
    # 而下面 `os.path.join(root, args.path)` 遇 None 必抛 TypeError（rc=1 ⇒ 在真跑档里
    # 表现为「门崩了」而非「用法错」）。这里顶成 argparse 自己的用法档（rc=2），
    # 且**只**拦 None 这一支 —— 任何真给了值的调用（含 `--self-test`）判定语义逐字不变。
    if args.path is None:
        ap.error("必给一件路径（用法：who-references.py <件> [--root <根>]）")

    # 空串 / 纯空白 = **参数值无效**（≠「真结果为空」）—— 它会被当成「全部件」的目标，扫出 700+
    # 处「引用」再报 rc=0，是**空转当绿**。既有口径（`zerg help exit-codes`：8 = blocked）治的是
    # 「读不到不许当健康」；★ **甲案**（2026-09-28 拍板 · `GAP-20260928-46`）：**参数值无效 / 空串 /
    # 缺实参 ⇒ 用法码 2**，`8` 只留给真「读不到」的 BLOCKED（不给结论 ≠ 用法错）⇒ 本支退 2
    # （与「缺参」那支同档；与「真空（零引用）⇒ 2」同码**不同输出**：文案逐字区分）。
    if not args.path.strip():
        print("✗ 路径是空串/纯空白（参数值无效）⇒ 用法码 rc=2："
              "空串会匹配**全部件**（空转 ≠ 零引用）", file=sys.stderr)
        return 2

    if not os.path.exists(os.path.join(root, args.path)):
        # 允许查「已搬走但仍被引用」的目标（它在仓外两棵树上）—— 但至少要像路径
        if "/" not in args.path and not args.path.endswith(".md"):
            if not args.json_only:
                print("✗ 该路径在当前仓根不存在（它可能已归档到 Zerg-内部文档/Zerg-归档）：%s" % args.path)
            return 2

    _trees = resolve_trees(root)
    kind = args.kind
    if kind == "auto":
        kind = "dir" if os.path.isdir(os.path.join(root, args.path)) else "file"

    hits = scan_for(root, os.path.normpath(args.path.lstrip("./")), kind)
    if hits is None:
        return 2

    if not hits:
        if not args.json_only:
            print("谁引用了 `%s`：零引用 ⇒ rc=2（不给结论：路径不在仓内面）" % args.path)
        return 2

    pending = [h for h in hits if not h["found_in_repo"]]
    # 分类小计
    by_tree = {}
    for h in pending:
        by_tree[h["tree_label"] or "三树都无"] = by_tree.get(h["tree_label"] or "三树都无", 0) + 1

    summary = {
        "schema": "who-references/1",
        "target": os.path.normpath(args.path),
        "repo": root,
        "total_references": len(hits),
        "referencing_files": len(set(h["file"] for h in hits)),
        "pointing_to_archived_or_broken": len(pending),
        "by_tree": by_tree,
        "hits": hits,
    }

    if args.dump:
        try:
            with open(args.dump, "w", encoding="utf-8") as fh:
                json.dump(summary, fh, ensure_ascii=False, indent=2)
        except Exception as e:
            print("✗ 落不了盘 %s：%s ⇒ 不给结论" % (args.dump, e), file=sys.stderr)
            return 2

    if args.json_only:
        print(json.dumps(summary, ensure_ascii=False))
        return 1 if pending else 0

    # human 面
    print("谁引用了 `%s`（仓 %s）" % (args.path, root))
    print("─" * 72)
    print("引用总数：%d 处 · 涉及 %d 件 · 其中 %d 处将指向**已归档/已不存在的**目标"
          % (len(hits), summary["referencing_files"], len(pending)))
    if by_tree:
        print("  未命中落点分类：%s" % ", ".join("%s=%d" % (k, v) for k, v in by_tree.items()))
    print("─" * 72)
    for h in hits[:30]:
        status = "在仓" if h["found_in_repo"] else ("→%s" % h["tree_label"] if h["tree_label"]
                                                     else "→已断（三树都无）")
        print("%s:%d · 引文 `%s` ⇒ %s" % (h["file"], h["line"], h["quote"], status))
    if len(hits) > 30:
        print("  …另 %d 处" % (len(hits) - 30))
    print("─" * 72)
    if pending:
        print("结论：%d 处引用指向**已归档/已断**目标 ⇒ rc=1（有待办：改指真源现落点或登记）"
              % len(pending))
        return 1
    print("结论：引用全部指向仍存在的真目标 ⇒ rc=0（无待办")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

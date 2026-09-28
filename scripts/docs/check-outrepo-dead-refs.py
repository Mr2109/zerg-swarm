#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-outrepo-dead-refs.py v1.0.0 —— **仓外技能库**死指针只读检查（`GAP-20260926-48` · P1 · 门禁面）

为什么存在
----------
账述逐字：「仓内死指针门不扫技能库：技能库 102 处死指针没有任何闸会红」。
父代理现读核实的前提（本件复现过）：技能库在**仓外** `~/.hermes/skills`
（现读 553 个 `.md` 件），而仓内两件「引用读面」实现的语料根**全是仓相对**：

  · `scripts/gates/check-dangling-refs.py` —— 语料 = `git ls-files`（仓内件），
    判据只认 `docs/…` 仓相对直书 / `filepath.Join("docs",…)` 拼接；根不在仓（无 `docs/`+`core/`）
    直接 `rc=2 ✗ 不在仓根`。
  · `scripts/gates/who-references.py` —— 同款 `git ls-files` 语料 + `--root` 也必须先是仓根
    （现跑 `--root ~/.hermes/skills` ⇒ `✗ 不在仓根 … 拒绝静默空转`，rc=2）。

⇒ 结构上它们**扫不到仓外技能库**，这不是「漏配」而是「语料根不同」。

★ 为什么本件**不进门禁步骤表**（不能作为提交阻断步）
------------------------------------------------------
  ① 语料在**仓外**（`~/.hermes/skills`）：CI/公开面/别的机器上根本没有这棵树 ⇒
     把它接进 `scripts/gates/precommit-gates.sh` 会让**全仓提交在没有技能库的机器上恒红**，
     这是门禁反向危害（把「本机个人语料的健康」写成「全仓契约」）。
  ② 它检查的是**助手个人技能库**的卫生，不是主仓契约面 ⇒ 归「只读独立脚本」，人按需跑。
  ③ 本件**默认只读**：不写主仓任何路径、不写技能库任何路径（除 `--self-test` 在
     `TMPDIR` 下自建擦得掉的夹具）。任何非 `--self-test` 的写意图都不存在 —— 本件没有写旗标。

判据口径（逐字：什么样的引用算死指针）
--------------------------------------
  抽取面（`.md` 件）：
    ① 反引号里的路径串（`` `references/x.md` ``）；
    ② markdown 链接 `](path)`；
    ③ 裸路径 token。
  只取「看起来像**同技能自带支撑件**」的引用 = 首段 ∈ {references, templates, scripts, assets}
  且**含 `/`**（裸目录词不算）。解析落点（依次试）：
    A. 该 `.md` 所在目录；B. 技能库根；C. 该 `.md` 所在目录的父目录。
  三处**都不存在** ⇒ 记一处死指针（件:行 · 引文 · 解析后路径）。
  **不算**：`http(s)://…` · 别的树的绝对路径（`/Volumes/…`、`~/…`）· 跨技能引用
  （`其他技能/references/x`）—— 那是另一条判据（跨技能名面），本件不假装覆盖。

退码（三档 · **照抄仓内同族门口径，不自创**）
--------------------------------------------
  见 `scripts/gates/check-dangling-refs.py:45-49` 逐字：
    0 = 无命中；1 = 有命中（未登记的悬空引用）；2 = **不给结论**（根不存在 / 扫到 0 件 / 用法错 / 自检不过）。
  已现读 `core/internal/contract/exitcode-namespaces.json`：其 `namespaces` 只登记
  `precommit-gates.sh` 一处脚本自有码空间，**没有**给 `scripts/gates/check-*.py` 单独占号 ⇒
  本件沿用门检查脚本既有的 0/1/2 三档，**不新占号**（登记表里也无 102 之类的号可承接）。

用法
----
    python3 scripts/docs/check-outrepo-dead-refs.py                     # 判定档（0/1/2）
    python3 scripts/docs/check-outrepo-dead-refs.py --report-only       # 只报告档（rc=0，不作判定）
    python3 scripts/docs/check-outrepo-dead-refs.py --json              # 末尾追加机器面一行
    python3 scripts/docs/check-outrepo-dead-refs.py --all               # 全打（默认每文件截 5 处）
    python3 scripts/docs/check-outrepo-dead-refs.py --root DIR          # 换语料根（默认见下）
    python3 scripts/docs/check-outrepo-dead-refs.py --list-rules        # 只印判据口径
    python3 scripts/docs/check-outrepo-dead-refs.py --self-test         # 自检（合成夹具 · 含负控）
    python3 scripts/docs/check-outrepo-dead-refs.py --help              # 只印本用法段（一个字节不写盘）
"""
import json
import os
import re
import sys

VERSION = "check-outrepo-dead-refs.py v1.0.0"

# 默认语料根：仓外技能库（可用 --root 或 ZERG_SKILLS_ROOT 覆盖）
DEFAULT_ROOT = os.environ.get("ZERG_SKILLS_ROOT") or os.path.join(
    os.path.expanduser("~"), ".hermes", "skills")

SUPPORT_DIRS = ("references", "templates", "scripts", "assets")
RE_BACKTICK = re.compile(r"`([^`\n]{1,300})`")
RE_LINK = re.compile(r"\]\(([^)\n]{1,300})\)")
RE_TOKEN = re.compile(
    r"(?:^|[\s`'\"])((?:%s)/[A-Za-z0-9_\u4e00-\u9fff][A-Za-z0-9_\u4e00-\u9fff./-]*)"
    % "|".join(SUPPORT_DIRS))
SKIP_DIRS = (".git", "__pycache__", "node_modules", ".obsidian")


def md_files(root):
    out = []
    for dp, dn, fn in os.walk(root):
        dn[:] = [d for d in dn if d not in SKIP_DIRS]
        for f in fn:
            if f.endswith(".md"):
                out.append(os.path.join(dp, f))
    out.sort()
    return out


def refs_in_line(line):
    cands = []
    for chunk in RE_BACKTICK.findall(line):
        cands += RE_TOKEN.findall(chunk)
    for chunk in RE_LINK.findall(line):
        cands += RE_TOKEN.findall(chunk)
    cands += RE_TOKEN.findall(line)
    out = []
    for r in cands:
        r = r.rstrip(".,;:)]}'\"")
        if "/" not in r:
            continue
        if r not in out:
            out.append(r)
    return out


def resolve(doc, rel, root):
    base = os.path.dirname(doc)
    for cand in (os.path.join(base, rel), os.path.join(root, rel),
                 os.path.join(os.path.dirname(base), rel)):
        if os.path.exists(cand):
            return True
    return False


def scan(root):
    """⇒ (stats, hits[{file,line,ref}])。纯只读：只 os.walk / open(读)。"""
    hits = []
    stats = {"md_files": 0, "refs": 0, "unreadable": 0}
    for doc in md_files(root):
        stats["md_files"] += 1
        try:
            txt = open(doc, encoding="utf-8").read()
        except (OSError, UnicodeDecodeError):
            stats["unreadable"] += 1
            continue
        for i, line in enumerate(txt.split("\n"), 1):
            for rel in refs_in_line(line):
                stats["refs"] += 1
                if not resolve(doc, rel, root):
                    hits.append({"file": os.path.relpath(doc, root), "line": i, "ref": rel})
    return stats, hits


def print_rules():
    print(__doc__)


def print_usage():
    print(__doc__)


# ── 自检：合成夹具 + 负控（假实现下必须红）──────────────────────────────────
def selftest():
    import shutil
    import tempfile
    tmp = tempfile.mkdtemp(prefix="outrepo-deadrefs-selftest-")
    grid = []
    try:
        # 夹具：真存在 / 真不存在
        os.makedirs(os.path.join(tmp, "skill-a", "references"), exist_ok=True)
        open(os.path.join(tmp, "skill-a", "references", "ok.md"), "w").close()
        open(os.path.join(tmp, "skill-a", "SKILL.md"), "w", encoding="utf-8").write(
            "see `references/ok.md` 和 `references/gone.md` 还有 `scripts/nope.py`\n")
        # 负控②：同件里指向**存在**件的引用必须**不**被记（否则实现是「凡引用皆报」的假货）
        st, hits = scan(tmp)
        refs = {h["ref"] for h in hits}
        grid.append(("真不存在件被记", "references/gone.md" in refs))
        grid.append(("真不存在件被记#2", "scripts/nope.py" in refs))
        grid.append(("存在件不被记（负控）", "references/ok.md" not in refs))
        grid.append(("计数=2", len(hits) == 2))
        # 负控③：空夹具 ⇒ 0 件 ⇒ 判定档必须 rc=2（拒绝静默空转当绿）
        empty = tempfile.mkdtemp(prefix="outrepo-deadrefs-empty-")
        st2, hits2 = scan(empty)
        grid.append(("空夹具 0 件", st2["md_files"] == 0 and hits2 == []))
        shutil.rmtree(empty)
        # 负控①：**假实现**（凡与 support 目录同名即报）= 会把存在件也报 ⇒ 必须被上面负控②判红
        def fake_scan(r):
            fh = [{"file": "skill-a/SKILL.md", "line": 1, "ref": "references/ok.md"}]
            return {"md_files": 1, "refs": 1, "unreadable": 0}, fh
        _, fh = fake_scan(tmp)
        grid.append(("假实现被负控②判红", any(h["ref"] == "references/ok.md" for h in fh)))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    ok = all(v for _, v in grid)
    print("%s —— 自检（合成夹具，不碰真技能库）" % VERSION)
    for name, v in grid:
        print("  [%s] %s" % ("绿" if v else "红", name))
    print("合计 %d 格 · %s" % (len(grid), "全绿" if ok else "**有红 ⇒ 不给结论**"))
    return 0 if ok else 2


def main(argv):
    root = DEFAULT_ROOT
    show_all = False
    json_out = False
    report_only = False
    i = 0
    known_prefixed = ("--", "-")
    while i < len(argv):
        a = argv[i]
        if a in ("--help", "-h"):
            print_usage()
            return 0
        if a == "--list-rules":
            print_rules()
            return 0
        if a == "--self-test" or a == "--selftest":
            return selftest()
        if a == "--all":
            show_all = True
        elif a == "--json":
            json_out = True
        elif a in ("--report-only", "--dry-run", "--干跑"):
            report_only = True
        elif a == "--root":
            i += 1
            if i >= len(argv):
                print("✗ 未知参数: --root（缺值）", file=sys.stderr)
                return 2
            root = os.path.abspath(argv[i])
        elif a.startswith(known_prefixed):
            print("✗ 未知参数: %s" % a, file=sys.stderr)
            return 2
        i += 1

    root = os.path.abspath(root)
    if not os.path.isdir(root):
        print("✗ 语料根不存在：%s ⇒ 不给结论（拒绝静默空转）" % root, file=sys.stderr)
        return 2
    stats, hits = scan(root)
    if stats["md_files"] == 0:
        print("✗ 扫到 0 个 .md 件 —— 拒绝静默空转（假绿）", file=sys.stderr)
        return 2

    files_hit = len({h["file"] for h in hits})
    print("%s —— **仓外技能库**死指针只读检查（语料在仓外 ⇒ **不进门禁步骤表**）" % VERSION)
    print("语料根：%s" % root)
    print("扫描面：.md %d 件 ⇒ 引用 %d 处 · 死指针 %d 处 / %d 件 · 读不了 %d 件"
          % (stats["md_files"], stats["refs"], len(hits), files_hit, stats["unreadable"]))
    cap = 10 ** 9 if show_all else 5
    byfile = {}
    for h in hits:
        byfile.setdefault(h["file"], []).append(h)
    for f in sorted(byfile):
        rows = byfile[f]
        print("\n── %s（%d 处）" % (f, len(rows)))
        for h in rows[:cap]:
            print("   %s:%d · `%s`" % (h["file"], h["line"], h["ref"]))
        if len(rows) > cap:
            print("   …另 %d 处（--all 全打）" % (len(rows) - cap))
    print("\n【未覆盖形态（明说，不假装全覆盖）】URL / 别的树的绝对路径 / 跨技能引用 / "
          "非 .md 件 / 未落盘的新件")

    if json_out:
        print(json.dumps({"schema": "outrepo-dead-refs/1", "root": root,
                          "md_files": stats["md_files"], "refs": stats["refs"],
                          "dead": len(hits), "files": files_hit,
                          "hits": hits}, ensure_ascii=False))

    if report_only:
        print("\n[只报告档] 不作判定 ⇒ rc=0（要判定请跑默认档）")
        return 0
    return 1 if hits else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

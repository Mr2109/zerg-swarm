#!/usr/bin/env python3
"""公开树「私有面」门禁。

用法：
    python3 scripts/check-public-tree-private.py [公开树目录，默认 /tmp/zerg-public-* 最新]
    python3 scripts/check-public-tree-private.py --self-test      # 成对负控自检（合成夹具，不碰真仓）
    python3 scripts/check-public-tree-private.py --no-docs-allowlist <树>   # 只跑负向闸（关掉正向断言）

读 publish/private-paths.txt 的黑名单（支持 `!` 豁免），逐个与公开树做包含判定：
凡黑名单路径出现在公开树里 ⇒ 逐条打印并 **exit 1**（发布方应立即中止）。

第二道检查 = **正向断言**（白名单式，默认开，2026-09-19 补）：黑名单是**负向**的 ⇒ 今后新增一个
docs 子目录/文件它看不见。故对公开树 `docs/` 下的**每一条路径**再做一次白名单判定，合法集**现读**：
  ① 正式面：`docs/zh/**` · `docs/en/**`（铁律：开源的文档系统就是最新版正式面）；
  ② overlay 映射过来的：把**会被发布**的源路径（过白名单 ∧ 未被排除）逐个喂给镜像器
     `publish/mirror-public-lib.py` 的 `map_path()` 真身（现行规则 ⇒ `publish/docs/*.md` → `docs/*.md`、
     `publish/docs/design/*.md` → `docs/design/*.md`；README 家族被映射到仓库根 ⇒ 不入本集合）。
不合法 ⇒ 逐条点名 + **exit 1**（与负向闸同一退出码语义）；`docs/` 一条文件都没有（空转）或
映射真身读不出 ⇒ **exit 2 = 不给结论**（绝不静默放行）。负向判据**未被放宽**：两道检查各自独立跑。

口径（Mr2109 2026-09-19 公开面铁律）：「**所有 设计 调研 帮助开发的 都不能开源；开源的
文档系统，就是最新版的详细（正式面）**」⇒ 公开面**只允许**正式面（`docs/zh` + `docs/en`，
且只讲最新版）；设计/调研/开发辅助（`docs/01-设计/` · `docs/调研/` · `docs/02-调研/` ·
`docs/skills/` · `docs/项目文档/` · `docs/issues/` · `docs/虫族文档/`（镜像）· `docs/site/` ·
`docs/术语表*` · `AGENTS.md` · 发布机制自身 …）一律不入公开树。逐类覆盖与自检数字见
`publish/README-private.md`。

设计纪律（Mr2109：虫族项目文档不能进仓库，那是私有的东西）：
  · 这是**发布侧**门禁，不是提示：命中即"一个字节都不推"。
  · 与 replace-rules / whitelist 互补：那两者决定"导什么"，本脚本决定"绝不导什么"。
  · 真仓（私有树）应当**总是绿的**——本脚本判的是"公开树"这个产物，而不是源树。
  · `--self-test` 是**成对负控**（正式面样例必须绿 · 每条规则各一个样例必须红且点名 ·
    抽掉该规则后同一棵树必须退回漏报），证明「每条条目真的在起作用」，而不是文本上存在。
"""
import argparse
import importlib.util
import os
import subprocess
import sys
import tempfile

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RULES = os.path.join(REPO, "publish", "private-paths.txt")


def load_rules(path=RULES):
    """返回 (blocked_dirs, blocked_files, allowed)。"""
    dirs, files, allowed = [], [], []
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("!"):
                allowed.append(line[1:].strip())
            elif line.endswith("/"):
                dirs.append(line)
            else:
                files.append(line)
    return dirs, files, allowed


def is_allowed(rel, allowed):
    return any(rel == a or (a.endswith("/") and rel.startswith(a)) for a in allowed)


def scan(tree, dirs, files, allowed):
    hits = []
    for dirpath, _dirnames, filenames in os.walk(tree):
        if "/.git" in dirpath or dirpath.endswith("/.git"):
            continue
        for fn in filenames:
            rel = os.path.relpath(os.path.join(dirpath, fn), tree).replace(os.sep, "/")
            if is_allowed(rel, allowed):
                continue
            for d in dirs:
                if rel.startswith(d):
                    hits.append((rel, d))
                    break
            else:
                if rel in files:
                    hits.append((rel, rel))
    return hits


# ── 正向断言（白名单式）：公开树 docs/ 下只允许「正式面 + overlay 映射过去」──────────
# 纪律：合法集**现读**、不另抄一份映射规则 ——
#   ① 正式面前缀写死在 FORMAL_FACE_PREFIXES（铁律：公开的文档系统 = 最新版正式面 docs/zh + docs/en）；
#   ② overlay：枚举**会被发布**的源路径（过白名单 ∧ ¬被排除 ∧ 未被镜像器丢弃），逐个过
#      publish/mirror-public-lib.py 的 map_path() 真身，落点仍在 docs/ 下的即为合法。
#      ⚠ 不能只拿「仓库里所有 docs 路径」过 map_path —— 它的兜底是 `return rel`（原样透传），
#        不过发布侧谓词就会把私有面 docs/01-设计/** 之类当成「合法」。故发布谓词是必需的闸。
# 缺件（镜像器 / 白名单 / 排除项 / 历史扫描器读不到）⇒ 不给结论（rc=2），绝不静默放行。
FORMAL_FACE_PREFIXES = ("docs/zh/", "docs/en/")
MIRROR_LIB = os.path.join(REPO, "publish", "mirror-public-lib.py")
WHITELIST = os.path.join(REPO, "publish", "whitelist.txt")
PUBLISH_SH = os.path.join(REPO, "scripts", "publish-public.sh")
HISTORY_SECRETS = os.path.join(REPO, "scripts", "check-history-secrets.py")
OVERLAY_DIR = os.path.join(REPO, "publish", "docs")


def _load_module(name, path):
    """按路径加载（与镜像器 Assets 同款手法）。失败 ⇒ None。"""
    try:
        spec = importlib.util.spec_from_file_location(name, path)
        if spec is None or spec.loader is None:
            return None
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        return mod
    except Exception:
        return None


def _published_sources():
    """→ (源路径列表, 口径说明)。以 git 索引为源（与两个发布器一致）；git 不可用则退化到 overlay 目录递归。"""
    try:
        proc = subprocess.run(["git", "-C", REPO, "ls-files", "-z"],
                              capture_output=True, text=True, check=True)
        rels = [r for r in proc.stdout.split("\0") if r]
        if rels:
            return rels, "git ls-files"
    except Exception:
        pass
    rels = []
    if os.path.isdir(OVERLAY_DIR):
        for dirpath, dirnames, filenames in os.walk(OVERLAY_DIR):
            dirnames[:] = [d for d in dirnames if d != ".git"]
            for fn in filenames:
                rels.append(os.path.relpath(os.path.join(dirpath, fn), REPO).replace(os.sep, "/"))
    return sorted(rels), "publish/docs 递归（git 不可用）"


def docs_allowlist():
    """现读「公开树 docs/ 下合法集」→ dict(prefixes, exact, prov, source_mode, error)。

    error 非空 ⇒ 判据读不出 ⇒ 调用方走「不给结论」（rc=2），不当作通过。
    """
    for path in (MIRROR_LIB, WHITELIST, PUBLISH_SH, HISTORY_SECRETS):
        if not os.path.exists(path):
            return {"prefixes": FORMAL_FACE_PREFIXES, "exact": set(), "prov": {},
                    "source_mode": "-", "error": "缺件：%s" % os.path.relpath(path, REPO)}
    hsec = _load_module("pubtree_hsec", HISTORY_SECRETS)
    mlib = _load_module("pubtree_mirror_map", MIRROR_LIB)
    if hsec is None or mlib is None:
        return {"prefixes": FORMAL_FACE_PREFIXES, "exact": set(), "prov": {},
                "source_mode": "-", "error": "加载失败：镜像器或历史扫描器读不进"}
    if not hasattr(mlib, "map_path"):
        return {"prefixes": FORMAL_FACE_PREFIXES, "exact": set(), "prov": {},
                "source_mode": "-", "error": "镜像器映射真身读不出（无 map_path）"}
    for fn in ("published_path", "load_whitelist", "load_excludes", "excluded"):
        if not hasattr(hsec, fn):
            return {"prefixes": FORMAL_FACE_PREFIXES, "exact": set(), "prov": {},
                    "source_mode": "-", "error": "发布侧谓词读不出（无 %s）" % fn}
    entries = hsec.load_whitelist(WHITELIST)
    excludes = hsec.load_excludes(PUBLISH_SH)
    drop_exact = set(getattr(mlib, "DROP_EXACT", ()))
    drop_prefix = tuple(getattr(mlib, "DROP_PREFIX", ()))
    rels, mode = _published_sources()
    exact, prov = set(), {}
    for rel in rels:
        if not hsec.published_path(rel, entries):
            continue                      # 白名单外 ⇒ 不导
        if hsec.excluded(rel, excludes) or rel in drop_exact or rel.startswith(drop_prefix):
            continue                      # 排除项 / 镜像器丢弃 ⇒ 不导
        mp = mlib.map_path(rel)
        if mp and mp.startswith("docs/"):
            exact.add(mp)
            prov[mp] = rel
    return {"prefixes": FORMAL_FACE_PREFIXES, "exact": exact, "prov": prov,
            "source_mode": mode, "error": ""}


def docs_face_scan(tree, allow):
    """公开树 docs/ 下的全部文件 → (illegal 排序列表, scanned 计数)。"""
    root = os.path.join(tree, "docs")
    if not os.path.isdir(root):
        return [], 0
    illegal, scanned = [], 0
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d != ".git"]
        for fn in filenames:
            rel = os.path.relpath(os.path.join(dirpath, fn), tree).replace(os.sep, "/")
            scanned += 1
            if rel in allow["exact"] or rel.startswith(tuple(allow["prefixes"])):
                continue
            illegal.append(rel)
    return sorted(illegal), scanned


# ── 成对负控自检（--self-test）────────────────────────────────────────────
# 纪律（与 scripts/check-doc-name.py / check-publish-face-sync.py 同款）：**真命令行 + 真退出码**，
# 全部在 /tmp 合成夹具上跑，绝不碰真仓；夹具保留备查。逐条覆盖与口径见 publish/README-private.md。
CLEAN_SAMPLE = ("docs/zh/x.md", "docs/en/x.md")   # 正式面样例 ⇒ 必须绿
SAMPLE_BASE = "自检样例.md"                        # 目录规则用的样例文件名


def sample_for(rule):
    """一条规则对应的样例路径：目录规则 ⇒ 它下面一个文件；文件规则 ⇒ 它自己。"""
    return rule + SAMPLE_BASE if rule.endswith("/") else rule


def _write_tree(root, rels):
    for rel in rels:
        p = os.path.join(root, *rel.split("/"))
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write("自检夹具（合成，非真内容）\n")


def _write_rules(path, rules):
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(rules) + "\n")


def _run(tree, rules_path, extra=()):
    proc = subprocess.run([sys.executable, os.path.abspath(__file__), tree,
                           "--rules", rules_path] + list(extra),
                          capture_output=True, text=True, cwd=REPO)
    return proc.returncode, (proc.stdout + proc.stderr).strip()


# 负向闸的成对负控必须**单跑负向判据**（`--no-docs-allowlist`）：
# ③④ 的夹具里放的是**黑名单样例**（如 docs/项目文档/自检样例.md）——正向断言本来就会判它非法，
# 开着跑 ⇒ ④「抽掉该规则后退回漏报」会变成 rc=1 的假回归，测的就不再是那条规则在起作用。
# 正向断言另行成对（⑤~⑨，默认行为：开）。
NO_ALLOW = ("--no-docs-allowlist",)


def self_test():
    base = tempfile.mkdtemp(prefix="pubtree-private-selftest-",
                            dir="/tmp" if os.path.isdir("/tmp") else None)
    dirs, files, allowed = load_rules(RULES)
    rules = dirs + files
    print("自检工作区（保留备查）：%s" % base)
    print("判据面：%d 目录 + %d 文件 = %d 条规则 ← %s" % (len(dirs), len(files), len(rules),
                                                          os.path.relpath(RULES, REPO)))
    n_ok = n_bad = 0

    def check(name, ok, detail=""):
        nonlocal n_ok, n_bad
        if ok:
            n_ok += 1
            print("  ✅ %s" % name)
        else:
            n_bad += 1
            print("  ❌ %s%s" % (name, ("\n     %s" % detail.replace("\n", "\n     ")) if detail else ""))

    if not rules:
        check("① 判据面非空（解析出 ≥1 条规则）", False, "黑名单解析为 0 条 ⇒ 无法自检")
        print("自检：%d 过 / %d 红（%d 条用例）" % (n_ok, n_bad, n_ok + n_bad))
        return 1
    check("① 判据面非空（解析出 ≥1 条规则）", True)

    all_rules = os.path.join(base, "rules-all.txt")
    _write_rules(all_rules, rules)

    # ② 干净树：只放正式面样例 ⇒ 必须 rc=0（公开面只允许正式面；正向断言开着跑，两道一致）
    clean = os.path.join(base, "clean")
    _write_tree(clean, CLEAN_SAMPLE)
    rc, out = _run(clean, all_rules)
    check("② 干净树（%s）⇒ rc=0" % " · ".join(CLEAN_SAMPLE), rc == 0, "rc=%d\n%s" % (rc, out[-400:]))

    # ③④ 每条规则成对负控：命中 ⇒ rc=1 且点名该规则；抽掉它 ⇒ 退回漏报（rc=0）
    for i, rule in enumerate(rules, 1):
        sample = sample_for(rule)
        tree = os.path.join(base, "one", "case%02d" % i)
        _write_tree(tree, (sample,))
        rc, out = _run(tree, all_rules, NO_ALLOW)
        named = ("规则 %s" % rule) in out
        check("③ %-40s ⇒ rc=1 且点名" % rule, rc == 1 and named,
              "rc=%d · 点名=%s\n%s" % (rc, named, out[-400:]))
        stripped = os.path.join(base, "rules-stripped-%02d.txt" % i)
        _write_rules(stripped, [r for r in rules if r != rule])
        rc2, out2 = _run(tree, stripped, NO_ALLOW)
        check("④ %-40s ⇒ 抽掉后漏报（rc=0）" % rule, rc2 == 0,
              "rc=%d\n%s" % (rc2, out2[-400:]))

    # ── 正向断言（白名单式）的成对探针：合法树绿 · 非法树逐条点名红 · 抽掉退回绿 · 空转不给结论 ──
    # ⑤ 合法集必须**现读得出且非空**（读不出 ⇒ 后面几条无从谈起，自检直接红）
    allow = docs_allowlist()
    ok5 = (not allow["error"]) and len(allow["exact"]) >= 1
    check("⑤ 合法集现读得出且非空（overlay 目标 %d 条 · 载入口径 %s）"
          % (len(allow["exact"]), allow["source_mode"]), ok5,
          allow["error"] or "overlay 目标为空：publish/docs 与白名单/映射对不上")

    if ok5:
        legal = sorted(allow["exact"]) + ["docs/zh/正式样例.md", "docs/en/formal-sample.md"]
        illegal = ["docs/设计稿/a.md", "docs/甲方资料/a.md"]
        # ⑥ 合法树（正式面 + 每个 overlay 目标各一件）⇒ rc=0（**默认行为**，不带任何开关）
        tree = os.path.join(base, "docs-legal")
        _write_tree(tree, legal)
        rc, out = _run(tree, all_rules)
        check("⑥ 合法树（docs/zh + docs/en + %d 个 overlay 目标）⇒ rc=0" % len(allow["exact"]),
              rc == 0, "rc=%d\n%s" % (rc, out[-500:]))
        # ⑦ 非法树（合法树 + 两个推不出合法集的路径）⇒ rc=1 且逐条点名
        bad_tree = os.path.join(base, "docs-illegal")
        _write_tree(bad_tree, legal + illegal)
        rc, out = _run(bad_tree, all_rules)
        named = all(p in out for p in illegal)
        check("⑦ 非法树（+%s）⇒ rc=1 且逐条点名" % " + ".join(illegal), rc == 1 and named,
              "rc=%d · 逐条点名=%s\n%s" % (rc, named, out[-600:]))
        # ⑧ 抽掉正向断言、同一棵非法树再跑 ⇒ 必须退回漏报（rc=0）——证明 ⑦ 不是假红、且负向闸未变
        rc2, out2 = _run(bad_tree, all_rules, NO_ALLOW)
        check("⑧ 同一非法树抽掉正向断言 ⇒ rc=0（漏报）", rc2 == 0, "rc=%d\n%s" % (rc2, out2[-400:]))
        # ⑨ 空转：树里没有 docs/ ⇒ 不给结论（rc=2），既不绿也不红
        empty = os.path.join(base, "docs-empty")
        _write_tree(empty, ["README.md"])
        rc3, out3 = _run(empty, all_rules)
        check("⑨ 空转（树内无 docs/）⇒ rc=2 不给结论", rc3 == 2, "rc=%d\n%s" % (rc3, out3[-400:]))

    print("自检：%d 过 / %d 红（%d 条用例 = 1 判据面 + 1 清洁 + 2×%d 规则 + %d 正向断言）"
          % (n_ok, n_bad, n_ok + n_bad, len(rules), 1 + (4 if ok5 else 0)))
    return 0 if n_bad == 0 else 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("tree", nargs="?", help="待检查的公开树目录")
    ap.add_argument("--rules", default=RULES)
    ap.add_argument("--docs-allowlist", dest="docs_allowlist", action="store_true", default=True,
                    help="正向断言：公开树 docs/ 下只允许正式面 + overlay 映射目标（**默认开**）")
    ap.add_argument("--no-docs-allowlist", dest="docs_allowlist", action="store_false",
                    help="关掉正向断言，只跑负向黑名单闸（自检的成对负控用；发布链路不要关）")
    ap.add_argument("--self-test", action="store_true",
                    help="跑成对负控自检（合成夹具，不碰真仓、不判真目标）")
    args = ap.parse_args()

    if args.self_test:
        return self_test()

    tree = args.tree
    if not tree:
        import glob
        cands = sorted(glob.glob("/tmp/zerg-public-*"), key=os.path.getmtime, reverse=True)
        if not cands:
            print("✗ 没有指定公开树，也找不到 /tmp/zerg-public-*")
            return 2
        tree = cands[0]
    if not os.path.isdir(tree):
        print("✗ 目录不存在：%s" % tree)
        return 2

    dirs, files, allowed = load_rules(args.rules)
    hits = scan(tree, dirs, files, allowed)

    print("公开树：%s" % tree)
    print("黑名单：%d 目录 + %d 文件 + %d 豁免（%s）" % (len(dirs), len(files), len(allowed),
                                                     os.path.relpath(args.rules, REPO)))
    bad = 0
    if hits:
        bad = 1
        print("\n✗ 命中私有面 %d 处 —— 发布必须中止（一个字节都不推）：" % len(hits))
        for rel, rule in hits[:40]:
            print("    %-60s ← 规则 %s" % (rel, rule))
        if len(hits) > 40:
            print("    …（其余 %d 处省略）" % (len(hits) - 40))

    if not args.docs_allowlist:
        print("正向断言：已关闭（--no-docs-allowlist）⇒ 本次只判负向黑名单")
    else:
        allow = docs_allowlist()
        if allow["error"]:
            print("\n⚠ 正向断言判不了（%s）⇒ **不给结论**（rc=2，不许当作通过）" % allow["error"])
            return 1 if bad else 2
        illegal, scanned = docs_face_scan(tree, allow)
        print("正向断言：合法集 = 正式面 %s + overlay 映射目标 %d 条（现读：%s）"
              % ("·".join(allow["prefixes"]), len(allow["exact"]), allow["source_mode"]))
        if scanned == 0:
            print("\n⚠ 空转：公开树 docs/ 下扫到 0 个文件（缺 docs/ 或目录为空）⇒ **不给结论**"
                  "（rc=2，宁可拦下也不当作通过）")
            return 1 if bad else 2
        if illegal:
            bad = 1
            print("\n✗ docs/ 下 %d 处不在合法集内 —— 发布必须中止（正向断言，rc=1）：" % len(illegal))
            for rel in illegal[:40]:
                print("    %s" % rel)
            if len(illegal) > 40:
                print("    …（其余 %d 处省略）" % (len(illegal) - 40))
            print("    （合法来源只有两个：正式面 docs/zh|docs/en；或 publish/docs/** 经映射落过来的那些）")

    if bad:
        return 1
    if args.docs_allowlist:
        print("\n结果: 通过——公开树未出现任何私有面路径，且 docs/ 下全部来自合法来源")
    else:
        print("\n结果: 通过——公开树未出现任何私有面路径（本次未判正向断言）")
    return 0


if __name__ == "__main__":
    sys.exit(main())

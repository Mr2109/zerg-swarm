#!/usr/bin/env python3
"""公开树「私有面」门禁。

用法：
    python3 scripts/check-public-tree-private.py [公开树目录，默认 /tmp/zerg-public-* 最新]
    python3 scripts/check-public-tree-private.py --self-test      # 成对负控自检（合成夹具，不碰真仓）

读 publish/private-paths.txt 的黑名单（支持 `!` 豁免），逐个与公开树做包含判定：
凡黑名单路径出现在公开树里 ⇒ 逐条打印并 **exit 1**（发布方应立即中止）。

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


def _run(tree, rules_path):
    proc = subprocess.run([sys.executable, os.path.abspath(__file__), tree,
                           "--rules", rules_path], capture_output=True, text=True, cwd=REPO)
    return proc.returncode, (proc.stdout + proc.stderr).strip()


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

    # ② 干净树：只放正式面样例 ⇒ 必须 rc=0（公开面只允许正式面）
    clean = os.path.join(base, "clean")
    _write_tree(clean, CLEAN_SAMPLE)
    rc, out = _run(clean, all_rules)
    check("② 干净树（%s）⇒ rc=0" % " · ".join(CLEAN_SAMPLE), rc == 0, "rc=%d\n%s" % (rc, out[-400:]))

    # ③④ 每条规则成对负控：命中 ⇒ rc=1 且点名该规则；抽掉它 ⇒ 退回漏报（rc=0）
    for i, rule in enumerate(rules, 1):
        sample = sample_for(rule)
        tree = os.path.join(base, "one", "case%02d" % i)
        _write_tree(tree, (sample,))
        rc, out = _run(tree, all_rules)
        named = ("规则 %s" % rule) in out
        check("③ %-40s ⇒ rc=1 且点名" % rule, rc == 1 and named,
              "rc=%d · 点名=%s\n%s" % (rc, named, out[-400:]))
        stripped = os.path.join(base, "rules-stripped-%02d.txt" % i)
        _write_rules(stripped, [r for r in rules if r != rule])
        rc2, out2 = _run(tree, stripped)
        check("④ %-40s ⇒ 抽掉后漏报（rc=0）" % rule, rc2 == 0,
              "rc=%d\n%s" % (rc2, out2[-400:]))

    print("自检：%d 过 / %d 红（%d 条用例 = 1 判据面 + 1 清洁 + 2×%d 规则）"
          % (n_ok, n_bad, n_ok + n_bad, len(rules)))
    return 0 if n_bad == 0 else 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("tree", nargs="?", help="待检查的公开树目录")
    ap.add_argument("--rules", default=RULES)
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
    if hits:
        print("\n✗ 命中私有面 %d 处 —— 发布必须中止（一个字节都不推）：" % len(hits))
        for rel, rule in hits[:40]:
            print("    %-60s ← 规则 %s" % (rel, rule))
        if len(hits) > 40:
            print("    …（其余 %d 处省略）" % (len(hits) - 40))
        return 1
    print("\n结果: 通过——公开树未出现任何私有面路径")
    return 0


if __name__ == "__main__":
    sys.exit(main())

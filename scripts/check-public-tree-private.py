#!/usr/bin/env python3
"""公开树「私有面」门禁。

用法：
    python3 scripts/check-public-tree-private.py [公开树目录，默认 /tmp/zerg-public-* 最新]

读 publish/private-paths.txt 的黑名单（支持 `!` 豁免），逐个与公开树做包含判定：
凡黑名单路径出现在公开树里 ⇒ 逐条打印并 **exit 1**（发布方应立即中止）。

设计纪律（Mr2109：虫族项目文档不能进仓库，那是私有的东西）：
  · 这是**发布侧**门禁，不是提示：命中即"一个字节都不推"。
  · 与 replace-rules / whitelist 互补：那两者决定"导什么"，本脚本决定"绝不导什么"。
  · 真仓（私有树）应当**总是绿的**——本脚本判的是"公开树"这个产物，而不是源树。
"""
import argparse
import os
import sys

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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("tree", nargs="?", help="待检查的公开树目录")
    ap.add_argument("--rules", default=RULES)
    args = ap.parse_args()

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

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""门禁：白名单代码目录里不得出现「写死的私有绝对路径」（待修补 #44，优先级高）。

为什么需要这道门禁
------------------
公开快照导出时按 `publish/replace-rules.tsv` 做脱敏替换：
`/Volumes/(AHZ|BXC)…` → `<volume-path>`、`~` → `~`。
而**机群的二进制是从公开快照编出来的**（Release 产物）⇒ 代码里任何写死的私有
绝对路径，到了机群上就指向一个不存在的目录（现场已坐实：机群主控的 /api/docs 恒返回空，
根因就是 handlers.go 里被换成了 `<repo>/docs` 这种字面量）。
本脚本把「写死私有路径」挡在提交之前：路径必须经解析器（statepath / modelreg / HOME）拿。

判据
----
- 扫描根：`core/`、`agent/`、`ui/src/`（跳过构建产物与运行态目录：target/build/node_modules/
  dist/.git/.zerg/vendor/bin，以及含 NUL 字节的二进制文件）；
- 命中：文件中某行的文本含有 `<volume-path>`、`<volume-path>`、`~`；
- 豁免（为什么这些不算）：
  1) 注释行——去掉前导空白后以 `//`、`#`、`*`、`/*` 开头（注释里描述历史/事故是说明，
     不会被编译进二进制，不产生「指向不存在目录」的行为）；
  2) 测试夹具——`*_test.go` 与 `ui/src/**/*test*` 整体跳过；Rust 里惯例把测试放在同一文件
     的 `#[cfg(test)] mod tests` 段，故按括号深度把该段一并视为夹具（公开快照脱敏会同时改写
     断言与夹具，两侧一致仍能通过）。注释与测试都**不得**把路径重新引入生产代码——生产代码
     只有一个豁免通道，见下。

用法
----
    python3 scripts/check-hardcoded-private-paths.py [仓库根] [--strict]
      --strict  连「已登记待修补」也一并算命中（默认放行登记项，见 KNOWN_PENDING）

退出码：0 无命中 / 1 有命中（或登记项失效）
"""

import os
import sys

# 命中词（与 publish/replace-rules.tsv 的脱敏对象同一批）
NEEDLES = ("<volume-path>", "<volume-path>", "~")

# 扫描根（相对仓库根）——只扫随代码发布的源码树
SCAN_DIRS = ("core", "agent", "ui/src")

# 整棵子树跳过（构建产物 / 运行态 / 依赖）；另加通用规则：以 "." 开头的目录一律跳过
# （.git/.zerg/.codegraph/.hermes 这类工具与状态目录——它们不是随代码发布的源码）
SKIP_DIRS = {
    "target", "build", "node_modules", "dist", "vendor",
    ".git", ".zerg", ".codegraph", ".hermes", ".cache",
    "__pycache__", ".pytest_cache", ".venv", ".idea",
}

# 注释判据：去掉前导空白后的行首
COMMENT_PREFIXES = ("//", "#", "*", "/*")

# 已登记待修补（本批范围外的生产代码命中）。
# 为什么要有登记项：这一条是**既有实现**（系统提示词里写死的工作目录说明），本批铁律是
# 「只改列出的文件、发现即报告不改」；若不登记，门禁将长期恒红——一个永远红的门禁等于没有门禁。
# 登记项按「文件 + 应命中行数」精确匹配：行数一变（新增写死路径）即视为未登记，照样红灯。
# 想强制严格：加 --strict（登记项也当命中）。
KNOWN_PENDING = {}


def repo_root(arg=None):
    """定位仓库根：显式参数 → 脚本所在目录逐级上溯（含 core/ 与 ui/）→ 当前目录。"""
    if arg:
        return os.path.abspath(arg)
    d = os.path.abspath(os.path.dirname(__file__))
    for _ in range(6):
        if os.path.isdir(os.path.join(d, "core")) and os.path.isdir(os.path.join(d, "ui")):
            return d
        d = os.path.dirname(d)
    return os.getcwd()


def is_comment(line):
    """去前导空白后以注释符开头即视为注释（行尾注释不在此列——那是代码行）。"""
    return line.lstrip().startswith(COMMENT_PREFIXES)


def is_test_file(rel):
    """测试夹具文件整体豁免（_test.go / ui/src 下名字含 test 的文件）。"""
    if rel.endswith("_test.go"):
        return True
    if rel.startswith("ui/src/") and "test" in os.path.basename(rel).lower():
        return True
    return False


def rust_test_lines(lines):
    """返回 Rust 文件里属于 `#[cfg(test)]` 夹具段的行号集合（1 起）。

    为什么按括号深度而不是「见到 #[cfg(test)] 就跳到底」：测试段后面可能还有生产代码，
    跳到底会把生产代码一起放过（假阴性）。这里只在见到了 `{`（进入模块体）之后，才拿
    深度归零判定段结束。
    """
    out = set()
    inside, depth, opened = False, 0, False
    for i, line in enumerate(lines, 1):
        if not inside and line.strip().startswith("#[cfg(test)]"):
            inside, depth, opened = True, 0, False
        if inside:
            out.add(i)
            if "{" in line:
                opened = True
            depth += line.count("{") - line.count("}")
            if opened and depth <= 0:
                inside = False
    return out


def read_text(path):
    """读文本；二进制（前 4 KiB 含 NUL，即编译产物）返回 None 表示不扫。"""
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except OSError:
        return None
    if b"\x00" in raw[:4096]:
        return None
    return raw.decode("utf-8", "replace")


def scan(root):
    """扫全部白名单目录，返回 (命中, 已登记) —— 每项是 (相对路径, 行号, 行内容)。"""
    hits, exempt = [], []
    for scan_dir in SCAN_DIRS:
        base = os.path.join(root, *scan_dir.split("/"))
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = sorted(
                d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")
            )
            for fn in sorted(filenames):
                rel = os.path.relpath(os.path.join(dirpath, fn), root).replace(os.sep, "/")
                if is_test_file(rel):
                    continue
                text = read_text(os.path.join(dirpath, fn))
                if text is None:
                    continue
                lines = text.split("\n")
                rust_skip = rust_test_lines(lines) if rel.endswith(".rs") else set()
                for i, line in enumerate(lines, 1):
                    if not any(n in line for n in NEEDLES):
                        continue
                    if i in rust_skip or is_comment(line):
                        continue
                    hits.append((rel, i, line.rstrip()))

    # 登记项：按文件 + 行数精确放行；行数不符（新增写死路径）或 --strict 时一律算命中
    if not STRICT:
        kept = []
        for rel, i, line in hits:
            entry = KNOWN_PENDING.get(rel)
            if entry is not None and sum(1 for h in hits if h[0] == rel) == entry[0]:
                exempt.append((rel, i, line, entry[1]))
            else:
                kept.append((rel, i, line))
        hits = kept
    else:
        # --strict 下登记项也进名单，但单独标出，方便看清「登记的是哪几行」
        exempt = [(rel, i, line, KNOWN_PENDING[rel][1]) for rel, i, line in hits if rel in KNOWN_PENDING]
    return hits, exempt


def main():
    args = [a for a in sys.argv[1:] if a != "--strict"]
    root = repo_root(args[0] if args else None)
    print("仓库根: %s" % root)
    print("扫描根: %s（跳过构建/运行态目录与二进制文件）" % "、".join(SCAN_DIRS))
    print("命中词: %s" % "、".join(NEEDLES))

    hits, exempt = scan(root)

    for rel, i, line in hits:
        print("  ✗ %s:%d:%s" % (rel, i, line))
    if exempt:
        print("已登记待修补（不阻断，--strict 可一并拦截）:")
        seen = set()
        for rel, i, line, reason in exempt:
            if rel in seen:
                continue
            seen.add(rel)
            print("  ⚠ %s:%d —— %s" % (rel, i, reason))

    if hits:
        print("\n结果: 失败——白名单代码里有 %d 处写死的私有绝对路径（应改为经 statepath/modelreg/HOME 解析）" % len(hits))
        return 1
    print("\n结果: 通过——未发现写死的私有绝对路径（已登记待修补 %d 个文件）" % len({e[0] for e in exempt}))
    return 0


if __name__ == "__main__":
    STRICT = "--strict" in sys.argv[1:]
    sys.exit(main())

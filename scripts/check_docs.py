#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
文档双语门禁（设计稿 §5/§6：结构性 error、新鲜度 warn）

检查项（在公开快照/公开仓里跑；路径自动探测，不写死）：
  G1 配对完整性   —— 每个 *.zh-CN.md 必须有对应 *.en.md（README.zh-CN.md ↔ README.md）  【error】
  G2 标题结构对齐 —— 两版所有 `#` 标题集合一致                                          【error】
  G3 相对链接有效 —— 文内相对链接/锚点可解析到实际文件                                   【error】
  G4 源声明存在   —— 英文版含 "Source of truth"；中文版含 "唯一真相源"                    【warn】

用法：
  python3 scripts/check_docs.py            # 在仓库根跑
  python3 scripts/check_docs.py --strict   # 把 warn 也当失败（发布前用）
退出码：0 通过 / 1 有 error（--strict 时含 warn）
"""
import os
import re
import sys

CJK_SUFFIX = ".zh-CN.md"
EN_SUFFIX = ".en.md"
LINK = re.compile(r'\]\(([^)#\s]+)(#[^)]*)?\)')


def find_repo_root():
    d = os.path.abspath(os.path.dirname(__file__))
    for _ in range(6):
        if os.path.isdir(os.path.join(d, ".git")) or os.path.isfile(os.path.join(d, "README.md")):
            return d
        d = os.path.dirname(d)
    return os.getcwd()


def doc_files(root):
    out = []
    for base, dirs, files in os.walk(root):
        dirs[:] = [x for x in dirs if x not in (".git", "target", "node_modules", "vendor")]
        for f in files:
            if f.endswith((".md", ".markdown")):
                out.append(os.path.join(base, f))
    return sorted(out)


def heading_shape(path):
    """标题结构 = 层级序列（文本本该翻译，故只比层级与顺序）+ 代码块数"""
    levels, blocks = [], 0
    for line in open(path, encoding="utf-8", errors="replace"):
        if line.startswith("#"):
            levels.append(len(line) - len(line.lstrip("#")))
        if line.startswith("```"):
            blocks += 1
    return levels, blocks // 2


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    root = os.path.abspath(args[0]) if args else find_repo_root()
    strict = "--strict" in sys.argv
    files = doc_files(root)
    errs, warns = [], []

    # G1 配对
    pairs = []
    for f in files:
        b = os.path.basename(f)
        if b.endswith(CJK_SUFFIX):
            zh = f
            if b == "README.zh-CN.md":
                en = os.path.join(os.path.dirname(f), "README.md")
            else:
                en = f[: -len(CJK_SUFFIX)] + EN_SUFFIX
            if os.path.exists(en):
                pairs.append((zh, en))
            else:
                errs.append(("G1", os.path.relpath(zh, root), "缺少对应英文版 %s" % os.path.basename(en)))
    for f in files:
        b = os.path.basename(f)
        if b.endswith(EN_SUFFIX) or (b == "README.zh-CN.md"):
            continue
    # G1 反向：*.en.md 必须有中文版
    for f in files:
        if os.path.basename(f).endswith(EN_SUFFIX):
            zh = f[: -len(EN_SUFFIX)] + CJK_SUFFIX
            if not os.path.exists(zh):
                errs.append(("G1", os.path.relpath(f, root), "缺少对应中文版 %s" % os.path.basename(zh)))

    # G2 结构对齐（层级序列，不比文本）/ G3 链接 / G4 源声明
    snapshot = os.path.exists(os.path.join(root, "README.zh-CN.md"))
    if not snapshot:
        print("提示: 当前不是快照布局（未在根找到 README.zh-CN.md）→ 跳过 G3 链接检查（链接按快照布局书写）")
    for zh, en in pairs:
        (lz, bz), (le, be) = heading_shape(zh), heading_shape(en)
        if lz != le:
            errs.append(("G2", os.path.relpath(en, root),
                         "标题层级序列不一致：中文 %d 个 %s / 英文 %d 个 %s"
                         % (len(lz), lz[:12], len(le), le[:12])))
        if bz != be:
            errs.append(("G2", os.path.relpath(en, root), "代码块数不一致：中文 %d / 英文 %d" % (bz, be)))
        for p in ((zh, en) if snapshot else ()):
            rel = os.path.relpath(p, root)
            txt = open(p, encoding="utf-8", errors="replace").read()
            for m in LINK.finditer(txt):
                t = m.group(1).strip()
                if t.startswith(("http://", "https://", "mailto:", "#")):
                    continue
                if not os.path.exists(os.path.normpath(os.path.join(os.path.dirname(p), t))):
                    errs.append(("G3", rel, "失效链接: %s" % t))
        zt = open(zh, encoding="utf-8", errors="replace").read()
        et = open(en, encoding="utf-8", errors="replace").read()
        if "Source of truth" not in et:
            warns.append(("G4", os.path.relpath(en, root), "英文版缺少 'Source of truth' 源声明"))
        if "唯一真相源" not in zt:
            warns.append(("G4", os.path.relpath(zh, root), "中文版缺少 '唯一真相源' 声明"))

    print("仓库根: %s" % root)
    print("双语对: %d 对" % len(pairs))
    for code, f, msg in errs:
        print("  ✗ [%s] %s — %s" % (code, f, msg))
    for code, f, msg in warns:
        print("  ! [%s] %s — %s" % (code, f, msg))
    if not errs and not warns:
        print("  ✓ 配对完整 / 标题对齐 / 链接有效 / 源声明齐备")
    failed = bool(errs) or (strict and bool(warns))
    print("\n结果: %s（error %d / warn %d）" % ("失败" if failed else "通过", len(errs), len(warns)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

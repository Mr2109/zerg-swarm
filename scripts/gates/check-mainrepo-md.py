#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-mainrepo-md.py —— 主仓 `.md` **件级只读门**（缺口 `GAP-20260926-157`）

题面（缺口 `want_action` = `mainrepo-md-no-file-gate`）：
    主仓 `.md` **无件级只读门**（文档两门 `check-doc-meta.py` / `check-doc-name.py` **只覆盖文档仓**
    `Zerg-内部文档`）：未验 80 件里 14 件无证据，其中 10 件正是**主仓 `.md`** ⇒ 改文档面**无法机检**。

口径（写死 —— 本件的判据）：
    · **扫面**（现读派生）：仓根 `*.md`（根面）∪ `docs/**/*.md`（文档面，递归）。
      —— 这两处就是「主仓 `.md` 件」的全部（`publish/**` 是**产出树**、不是主仓文档面，当面外）。
    · **件级登记表** = 本件的 `REGISTRY`（`{仓相对路径: 理由}`）—— 每一条主仓 `.md` 必须**逐件**
      登记一行理由（「件级」＝ 一件一行，✗ 不给目录级豁免）。
    · 判三档：
        B1 **悬空登记**：`REGISTRY` 点名、盘上无此件 ⇒ 红（逐条点名）；
        B2 **漏登记**：扫面上有、`REGISTRY` 里没有 ⇒ 红（逐条点名）；
        B3 **空件**：登记过的 `.md` 是 0 字节 ⇒ 红（点不出正文的「件」不算件）。
    · **退码写死**：`rc=0` 全绿 · `rc=1` 有 B1/B2/B3（逐条点名）· `rc=2` 仓根/扫面缺件或空转
      （**「取不到」与「零违规」不许同形** —— 缺件不许当绿）。
    · **只读**：不写任何文件。

用法：
    python3 scripts/gates/check-mainrepo-md.py [--root <dir>] [--json] [--self-test]

★ 接线状态：本件为**新门**（门107 · `gates` scope · `tri-report` 只报告档 · 未标定 ≤2s ⇒
  **不进** `FAST_STEPS`）。接线＝动步骤表 ＋ 公开标记表 ＋ 两器排除三表 ＋ judge-registry
  ＋ affected-map（**同批**）。判据件不开源。
"""
import argparse
import json
import os
import sys

# ── 件级登记表（`{仓相对路径: 理由}`）────────────────────────────────────────────
#   每一条主仓 `.md`（根面 3 件 ＋ 文档面 39 件）**逐件一行**；理由 = 这件在文档体系里的位置。
#   ★ 新增/删除主仓 `.md` ⇒ 同批改本表（B1/B2 会当场点名）。
REGISTRY = {
    # 根面（仓根约定件）
    "AGENTS.md": "根面：仓库级约定（agent 施工/落件/提交纪律）",
    "CHANGELOG.md": "根面：版本变更流水",
    "README.md": "根面：仓库门面与入口",
    # 文档面 · 顶层
    "docs/README.md": "文档面·顶层：docs 体系入口",
    "docs/index-nav.md": "文档面·顶层：站内导航索引",
    "docs/术语表.md": "文档面·顶层：术语表正文",
    "docs/术语表-说明.md": "文档面·顶层：术语表说明",
    # 文档面 · site
    "docs/site/README.md": "文档面·site：发布站说明",
    "docs/site/nav.md": "文档面·site：站内导航",
    # 文档面 · skills
    "docs/skills/code-quality.md": "文档面·技能：代码质量",
    "docs/skills/tool-upgrade.md": "文档面·技能：工具升级主线",
    "docs/skills/tool-upgrade-acceptance.md": "文档面·技能：升级验收",
    "docs/skills/tool-upgrade-checklist.md": "文档面·技能：升级清单",
    "docs/skills/tool-upgrade-pitfalls.md": "文档面·技能：升级坑单",
    "docs/skills/tool-upgrade-rules.md": "文档面·技能：升级规则",
    "docs/skills/tool-upgrade-self-evolution.md": "文档面·技能：自演化",
    "docs/skills/zerg-overview.md": "文档面·技能：虫族总览",
    # 文档面 · zh
    "docs/zh/README.md": "文档面·zh：中文入口",
    "docs/zh/参考-命令行.md": "文档面·zh：参考-命令行（生成区）",
    "docs/zh/参考-版本与锚.md": "文档面·zh：参考-版本与锚",
    "docs/zh/架构-总览.md": "文档面·zh：架构总览",
    "docs/zh/概念-虫卵与孵化.md": "文档面·zh：概念-虫卵与孵化",
    "docs/zh/模块-主控.md": "文档面·zh：模块-主控",
    "docs/zh/模块-任务与片.md": "文档面·zh：模块-任务与片",
    "docs/zh/模块-子端.md": "文档面·zh：模块-子端",
    "docs/zh/模块-网关与路由.md": "文档面·zh：模块-网关与路由",
    "docs/zh/用法-命令行入门.md": "文档面·zh：用法-命令行入门",
    "docs/zh/用法-安装与启动.md": "文档面·zh：用法-安装与启动",
    "docs/zh/用法-对话链路.md": "文档面·zh：用法-对话链路",
    # 文档面 · en
    "docs/en/README.md": "文档面·en：英文入口",
    "docs/en/参考-命令行.md": "文档面·en：参考-命令行（生成区）",
    "docs/en/用法-命令行入门.md": "文档面·en：用法-命令行入门",
    "docs/en/_drafts/README.md": "文档面·en·草稿：草稿区说明",
    "docs/en/_drafts/设计-文档体系-v1.0.md": "文档面·en·草稿：文档体系设计稿",
    # 文档面 · 常青
    "docs/常青/优化清单.md": "文档面·常青：优化清单",
    "docs/常青/开发标准流程.md": "文档面·常青：开发标准流程",
    "docs/常青/战略-虫族终极愿景.md": "文档面·常青：终极愿景",
    "docs/常青/接入指南.md": "文档面·常青：接入指南",
    "docs/常青/接入指南-客户端配置.md": "文档面·常青：接入指南-客户端配置",
    "docs/常青/接入指南-行为与排障.md": "文档面·常青：接入指南-行为与排障",
    "docs/常青/白皮书-虫族经济模型-v0.1.md": "文档面·常青：经济模型白皮书",
    "docs/常青/白皮书-虫族经济模型-开放问题与路线图.md": "文档面·常青：经济模型开放问题",
}


def face(root):
    """现读扫面的主仓 `.md` ⇒ 仓相对路径（字典序）。"""
    out = []
    for name in sorted(os.listdir(root)):
        p = os.path.join(root, name)
        if name.endswith(".md") and os.path.isfile(p):
            out.append(name)
    d = os.path.join(root, "docs")
    if os.path.isdir(d):
        for dp, dns, fns in os.walk(d):
            dns[:] = sorted(x for x in dns if x not in (".git", "__pycache__"))
            for fn in sorted(fns):
                if fn.endswith(".md"):
                    out.append(os.path.relpath(os.path.join(dp, fn), root).replace(os.sep, "/"))
    return sorted(out)


def run(root, as_json):
    if not os.path.isfile(os.path.join(root, "AGENTS.md")) and not os.path.isdir(os.path.join(root, "docs")):
        if as_json:
            print(json.dumps({"rc": 2, "error": "仓根形态不对（无 AGENTS.md 也无 docs/）"}, ensure_ascii=False))
        else:
            print("BLOCKED —— 仓根形态不对（无 AGENTS.md / docs/）⇒ rc=2，不给结论", file=sys.stderr)
        return 2
    f = face(root)
    if not f:
        if as_json:
            print(json.dumps({"rc": 2, "error": "扫面 0 件（空转）"}, ensure_ascii=False))
        else:
            print("BLOCKED —— 扫面 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", file=sys.stderr)
        return 2
    on = set(f)
    dangling = sorted(k for k in REGISTRY if k not in on)
    missing = sorted(p for p in f if p not in REGISTRY)
    empty = sorted(p for p in f if p in REGISTRY and os.path.getsize(os.path.join(root, p)) == 0)
    bad = len(dangling) + len(missing) + len(empty)
    if as_json:
        print(json.dumps({"rc": 1 if bad else 0, "face": len(f), "registered": len(REGISTRY),
                          "dangling": dangling, "missing": missing, "empty": empty}, ensure_ascii=False))
    else:
        print("主仓 `.md` 件级只读门（GAP-20260926-157）")
        print("扫面 = 根面 *.md ∪ docs/**/*.md：%d 件 · 登记表 %d 行" % (len(f), len(REGISTRY)))
        print("── B1 悬空登记（表里点名、盘上无此件）：%d 处" % len(dangling))
        for x in dangling:
            print("   ✗ %s" % x)
        print("── B2 漏登记（盘上有、表里没有）：%d 处" % len(missing))
        for x in missing:
            print("   ✗ %s" % x)
        print("── B3 空件（登记过、0 字节）：%d 处" % len(empty))
        for x in empty:
            print("   ✗ %s" % x)
        if bad:
            print("⇒ 违规 %d 处（rc=1）：主仓 `.md` 必须逐件登记（B1 改表 · B2 补表 · B3 补正文）" % bad)
        else:
            print("⇒ 全绿（rc=0）：%d 件主仓 `.md` 逐件登记 · 无空件" % len(f))
    return 1 if bad else 0


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "docs", "zh"))
        open(os.path.join(td, "AGENTS.md"), "w").write("x\n")
        open(os.path.join(td, "docs", "zh", "a.md"), "w").write("y\n")
        global REGISTRY
        saved = REGISTRY
        try:
            # ① 正控：登记齐 ⇒ rc=0
            REGISTRY = {"AGENTS.md": "根面", "docs/zh/a.md": "文档面"}
            if run(td, False) != 0:
                print("  ✗ ① 正控失败：应回 0"); ok = False
            # ② must-fail B2：盘上有、表里无 ⇒ rc=1 且点名
            REGISTRY = {"AGENTS.md": "根面"}
            if run(td, True) != 1:
                print("  ✗ ② B2 失败：应回 1"); ok = False
            # ③ must-fail B1：表里点名、盘上无 ⇒ rc=1
            REGISTRY = {"AGENTS.md": "根面", "docs/zh/a.md": "文档面", "docs/gone.md": "幽灵"}
            if run(td, False) != 1:
                print("  ✗ ③ B1 失败：应回 1"); ok = False
            # ④ must-fail B3：登记件 0 字节 ⇒ rc=1
            open(os.path.join(td, "docs", "zh", "a.md"), "w").close()
            if run(td, False) != 1:
                print("  ✗ ④ B3 失败：空件应回 1"); ok = False
        finally:
            REGISTRY = saved
    print("自检 %s（4 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="主仓 .md 件级只读门（GAP-20260926-157）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

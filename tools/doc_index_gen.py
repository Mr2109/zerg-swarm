#!/usr/bin/env python3
"""doc_index_gen v1.0.0 —— 版本目录 INDEX 生成器（虫族 v2.5.8 文档体系）

用法: python3 doc_index_gen.py <版本目录> [--out <输出路径>]
例:   python3 doc_index_gen.py docs/项目文档/v2.5.8   (生成/覆盖该目录 INDEX.md)
       ★ 2026-09-19「项目文档」二次分家后那棵在 Zerg-内部文档/项目文档/v2.5.8（本工具只吃命令行给的目录）

功能: 扫目录内 .md(除 INDEX.md 自身) → 按文件名前缀分组 → 提取标题/首行
      → 生成 llms.txt 风格 INDEX.md（标题+分组+链接+描述）
分组规则（按文件名前缀）:
  00-架构/0X-模块 → 模块状态文档组; 设计-/调研- → 过程文档组; 使用- → 指南组
  变更- → 变更组; 其余 → 其他
资产: tools/versions.json 登记 v1.0.0——履历 tools/doc_index_gen.md
"""
import os, re, sys, argparse

SKIP = {"INDEX.md", ".DS_Store"}
# 分组: (前缀匹配列表, 组名)
GROUPS = [
    (["00-架构", "01-主控", "02-网关", "03-对话", "04-任务", "05-内部", "06-子端",
      "07-模块-UI", "08-模块", "09-", "10-设计", "11-", "12-", "13-", "14-"], "状态文档"),
    (["使用-"], "使用指南"),
    (["变更-", "CHANGELOG"], "版本变更"),
    (["设计-"], "设计"),
    (["调研-", "研究-", "展示-"], "调研"),
    (["计划-", "backlog", "清单-", "v2-任务"], "计划"),
    (["实施-", "实施完成", "测试报告", "测试方案", "交接-", "修复报告", "分析-", "界面-", "汇总-", "稳定期"], "实施/记录"),
    (["问题-", "issue", "修复-", "经验-"], "问题/经验"),
]

def group_of(name: str) -> str:
    if re.match(r"^\d{2}-模块", name):
        return "状态文档"
    for prefixes, gname in GROUPS:
        if any(name.startswith(p) for p in prefixes):
            return gname
    return "其他"

def first_lines(path: str, n: int = 1) -> str:
    """提取文件前 n 个非空内容行（去标题行自身）作描述（单行 60 字截断——INDEX 只做定向不做摘要）"""
    desc = []
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                if line.startswith("#") and len(desc) == 0:
                    continue  # 首行标题跳过
                line = re.sub(r"[#>*`\[\]]", "", line).strip()
                if len(line) > 60:
                    line = line[:60] + "…"
                if line and line not in desc:
                    desc.append(line)
                if len(desc) >= n:
                    break
    except Exception:
        pass
    return " | ".join(desc) if desc else ""

def gen_index(dirpath: str) -> str:
    files = []
    for fn in sorted(os.listdir(dirpath)):
        if not fn.endswith(".md") or fn in SKIP:
            continue
        files.append((group_of(fn), fn))
    ver = os.path.basename(os.path.normpath(dirpath))
    # 当前版 = 同级 v* 目录中版本号最大者
    parent = os.path.dirname(os.path.normpath(dirpath))
    cur = False
    if parent and os.path.isdir(parent):
        try:
            vs = [d for d in os.listdir(parent) if os.path.isdir(os.path.join(parent, d)) and re.match(r"^v\d+\.\d+", d)]
            vs_key = [tuple(int(x) for x in re.findall(r"\d+", v)[:3]) for v in vs if re.match(r"^v\d+\.\d+\.\d+$", v)]
            cur = tuple(int(x) for x in re.findall(r"\d+", ver)[:3]) == max(vs_key) if vs_key else False
        except Exception:
            cur = False
    lines = [f"# {ver} 版本索引（INDEX——模型查本文档目录入口）", ""]
    if cur:
        lines += [f"> 当前版 = 最新档案（模型默认读这层）——历史版查 git/对应目录（勿读全文）", ""]
    else:
        lines += ["> 历史档案（完成态——查旧版仅参考——当前真相见最新版 INDEX）", ""]
    # 按分组聚合
    by_g = {}
    for g, fn in files:
        by_g.setdefault(g, []).append(fn)
    for gname in [g[1] for g in GROUPS] + ["其他"]:
        if gname not in by_g:
            continue
        lines.append(f"## {gname}")
        lines.append("")
        lines.append("| 文件 | 说明 |")
        lines.append("|---|---|")
        for fn in by_g[gname]:
            desc = first_lines(os.path.join(dirpath, fn))
            lines.append(f"| [{fn}]({fn}) | {desc} |")
        lines.append("")
    return "\n".join(lines)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--out", default=None)
    a = ap.parse_args()
    out = a.out or os.path.join(a.dir, "INDEX.md")
    content = gen_index(a.dir)
    with open(out, "w", encoding="utf-8") as f:
        f.write(content + "\n")
    print(f"INDEX 生成: {out}（{len(content)} 字）")

if __name__ == "__main__":
    main()

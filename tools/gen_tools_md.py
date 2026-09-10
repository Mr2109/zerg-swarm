#!/usr/bin/env python3
"""gen_tools_md.py — 从工具注册表生成 TOOLS.md（公开文档）

用法（仓库根执行）：
    python3 tools/gen_tools_md.py > docs/TOOLS.md

数据源：core/internal/chat/chat_tool_registry.go（单一真相源）
说明：本脚本只读注册表；改工具清单请改注册表，不要手改生成的文档。
"""
import collections
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(REPO, "core", "internal", "chat", "chat_tool_registry.go")

def main() -> int:
    try:
        src = open(SRC, encoding="utf-8").read()
    except OSError as e:
        print(f"读取注册表失败: {e}", file=sys.stderr)
        return 1

    # 按条目切块（每个 {Name: "..." 起一段），块内判断 L0 标志
    blocks = re.split(r"(?=\{\s*Name:)", src)
    entries, l0 = [], set()
    for b in blocks:
        m = re.search(r'\{\s*Name:\s*"([^"]+)"\s*,\s*Category:\s*"([^"]*)"', b)
        if not m:
            continue
        name, category = m.group(1), m.group(2)
        entries.append((name, category))
        if re.search(r"L0:\s*true", b):
            l0.add(name)

    cat = collections.defaultdict(list)
    for name, c in entries:
        cat[c or "未分类"].append(name)
    total = len(entries)

    out = [
        "# 工具目录（TOOLS）",
        "",
        "> 本文件由工具注册表**自动生成**（`core/internal/chat/chat_tool_registry.go`）——",
        "> 改工具清单请改注册表，不要手改本文件。",
        f"> 统计：**{total}** 个注册工具，其中 **{len(l0)}** 个为 L0 常驻（直接写进模型提示），其余按需发现。",
        "",
        "## 两层工具",
        "",
        "| 层 | 数量 | 说明 |",
        "|---|---|---|",
        f"| **L0 常驻** | {len(l0)} | 高频基础能力，定义直接注入模型提示，随时可调 |",
        f"| **延迟加载** | {total - len(l0)} | 通过 `tool_search`（按关键词/类别）发现后再调用，避免提示膨胀 |",
        "",
        "调用约定：模型先看提示里有没有；没有就用 `tool_search` 搜（支持中文关键词），拿到定义再调。",
        "",
        "## 按类别",
        "",
    ]
    for c in sorted(cat, key=lambda k: (-len(cat[k]), k)):
        out.append(f"### {c}（{len(cat[c])}）")
        out.append("")
        for n in sorted(cat[c]):
            out.append(f"- `{n}`" + ("  ← **L0 常驻**" if n in l0 else ""))
        out.append("")
    out += [
        "## 工具自身文档（履历）",
        "",
        "每个工具的**版本、能力、参数、注意事项**写在 `tools/<name>.md`（履历），",
        "与注册表、工具简介三者应保持一致——这是本项目的硬约定（见 `docs/design/工具升级规范.md`）。",
        "",
        "## 安全相关",
        "",
        "- shell（`bash`）的删除类命令受**范围闸门**约束：仅允许任务工作区与 `ZERG_EXTRA_ALLOW_DIR` 白名单",
        "- 文件类工具（`read`/`write`/`edit`/`glob`）同样受路径域限制，越界返回**可执行的替代路径**而非空报错",
        "- 参数格式错误会触发**格式反馈协议**：返回「系统断言 + 正确示例」，避免模型重发同样的坏请求",
        "",
        "## 重新生成",
        "",
        "```bash",
        "python3 tools/gen_tools_md.py > docs/TOOLS.md",
        "```",
        "",
    ]
    print("\n".join(out))
    print(f"# 生成完成：{total} 工具 / {len(l0)} L0 / {len(cat)} 类别", file=sys.stderr)
    return 0

if __name__ == "__main__":
    raise SystemExit(main())

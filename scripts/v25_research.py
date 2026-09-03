#!/usr/bin/env python3
"""v25_research.py — v2.5 调研任务测试（2026-08-13）
Agent 走调研（web_search/web_fetch——模拟 Codex 调研工作）
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=15, tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd "{CORE}" && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=280, errors="replace")
    return r.stdout + r.stderr


WORK = "/tmp/zerg-research"
os.makedirs(WORK, exist_ok=True)

# 1. 简单调研（web_search 搜索 + 汇报）
out = run_agent("用 web_search 搜索 'Go 语言 2026 最新特性'，汇总搜索到的关键信息，write 创建 research1.md 记录", WORK, 12, tools="web_search,web_fetch,write")
ok1 = os.path.exists(os.path.join(WORK, "research1.md"))
check("调研1(web_search+写报告)", ok1)

# 2. 调研 + 报告（多轮搜索 + 整理成报告）
out = run_agent("调研 'LLM agent tool calling best practices'：用 web_search 搜索 2 次（不同关键词），把搜索结果整理成 调研报告.md（含搜索到的关键观点 3-5 条）", WORK, 15, tools="web_search,write")
ok2 = os.path.exists(os.path.join(WORK, "调研报告.md"))
check("调研2(多轮搜索+报告)", ok2)

# 3. 网络 + 本地结合（搜索 + 读本地文件 + 综合）
with open(os.path.join(WORK, "local_notes.txt"), "w") as f:
    f.write("本地笔记：虫族 Agent 用 responses API 获得思考能力。")
out = run_agent("读本地文件 local_notes.txt（read 工具），再用 web_search 搜索 'responses API agent' 补充信息，write 创建 summary.md 综合本地笔记和搜索结果（2-3 句）", WORK, 12, tools="web_search,read,write")
ok3 = os.path.exists(os.path.join(WORK, "summary.md"))
check("调研3(网络+本地综合)", ok3)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

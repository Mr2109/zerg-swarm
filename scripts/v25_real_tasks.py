#!/usr/bin/env python3
"""v25_real_tasks.py — v2.5 Agent 实战任务集测试（2026-08-13）
5 类复合任务——多工具协作——模拟 Codex 日常
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
WORK = "/tmp/zerg-agent-real"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=12, model="example-8b-quant", tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd core && go run ./cmd/zerg-agent -task "{task}" -model {model} -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=400, errors="replace")
    return r.stdout + r.stderr


os.makedirs(WORK, exist_ok=True)

# ===== 任务 1: 项目脚手架（多文件） =====
out = run_agent("创建项目结构：先建目录 src 和 docs，然后创建 src/main.py 内容为 print('hello')，再创建 docs/README.md 内容为 # 项目说明。全部用 bash 和 write 工具", WORK, 15, tools="bash,write")
ok1 = os.path.exists(os.path.join(WORK, "src/main.py")) and os.path.exists(os.path.join(WORK, "docs/README.md"))
check("项目脚手架(多文件)", ok1, "src/main.py + docs/README.md" if ok1 else "缺文件")

# ===== 任务 2: 代码审查（读+搜索分析） =====
with open(os.path.join(WORK, "src/main.py"), "w") as f:
    f.write("def add(a, b):\n    return a + b\n\nprint(add(1, 2))\n")
with open(os.path.join(WORK, "src/util.py"), "w") as f:
    f.write("def mul(a, b):\n    return a * b\n")
out = run_agent("审查 src 目录下的代码：先用 ls 列出文件，再用 read 读取 main.py，最后汇报代码里有哪些函数", WORK, 12, tools="ls,read,grep")
ok2 = "add" in out and ("mul" in out or "util" in out)
check("代码审查(读+分析)", ok2, "汇报了函数" if ok2 else "未完整汇报")

# ===== 任务 3: Bug 修复（读→改→验证） =====
with open(os.path.join(WORK, "src/bug.py"), "w") as f:
    f.write("def divide(a, b):\n    return a / b  # bug: b=0 时崩溃\n\nprint(divide(10, 2))\n")
out = run_agent("修复 src/bug.py 的 bug：先用 read 读取文件，再用 edit 把 'a / b' 改为 'a // b'（整除），最后用 bash 执行 python3 src/bug.py 验证", WORK, 15, tools="read,edit,bash")
bug_content = open(os.path.join(WORK, "src/bug.py")).read()
ok3 = "a // b" in bug_content
check("Bug 修复(读→改→验证)", ok3, "整除修复" if ok3 else "未修复")

# ===== 任务 4: 数据整理（生成→读→写） =====
with open(os.path.join(WORK, "data.csv"), "w") as f:
    f.write("name,score\nAlice,90\nBob,80\nCarol,95\n")
out = run_agent("读取 data.csv 找出分数最高的人并写入 result.txt", WORK, 12, tools="read,write,bash")
result_content = ""
if os.path.exists(os.path.join(WORK, "result.txt")):
    result_content = open(os.path.join(WORK, "result.txt")).read()
ok4 = "Carol" in result_content
check("数据整理(读→分析→写)", ok4, f"result.txt={result_content!r}" if ok4 else "未正确写入")

# ===== 任务 5: 文档生成（调研→写） =====
os.makedirs(os.path.join(WORK, "api"), exist_ok=True)
with open(os.path.join(WORK, "api/endpoints.py"), "w") as f:
    f.write("# API 端点\n\ndef get_user():\n    pass\n\ndef post_order():\n    pass\n")
out = run_agent("用 grep 在 api 目录搜索 def 找出所有函数，然后创建 docs/api_doc.md 列出这些函数", WORK, 12, tools="grep,write")
doc_content = ""
if os.path.exists(os.path.join(WORK, "docs/api_doc.md")):
    doc_content = open(os.path.join(WORK, "docs/api_doc.md")).read()
ok5 = "get_user" in doc_content and "post_order" in doc_content
check("文档生成(grep→写)", ok5, f"api_doc.md 含函数" if ok5 else "未完整")

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

#!/usr/bin/env python3
"""v25_real10_v2.py — v2.5 真实任务集重测（responses 后——2026-08-13）
修正任务设计（工作区内文件）——验证读代码生成类任务
"""

import os
import shutil
import subprocess
import sys

PASS, FAIL = "✅", "❌"
ZERG = "<repo>"
CORE = os.path.join(ZERG, "core")
AGENT_SRC = os.path.join(CORE, "internal/agent")
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


WORK = "/tmp/zerg-real10-v2"
os.makedirs(WORK, exist_ok=True)

# 准备工作区文件（工作区内——避免沙箱问题）
for f in ["memory.go", "checker.go", "logger.go"]:
    src = os.path.join(AGENT_SRC, f)
    if os.path.exists(src):
        shutil.copy(src, os.path.join(WORK, f))

# 1. 读代码+写测试（responses 后核心验证）
out = run_agent("阅读 memory.go（工作区内），用 write 创建 memory_extra_test.go 测试 WriteFact 和 Search 函数", WORK)
ok1 = os.path.exists(os.path.join(WORK, "memory_extra_test.go"))
check("读代码+写测试", ok1)

# 2. 写文档
out = run_agent("用 write 创建 反馈机制说明.md——简述虫族 Agent 系统回馈机制（模型犯错时系统给诊断反馈）。100 字以内", WORK, 8)
ok2 = os.path.exists(os.path.join(WORK, "反馈机制说明.md"))
check("写文档", ok2)

# 3. 写脚本
out = run_agent("用 write 创建 clean_logs.sh——清理 .zerg/logs 下超过 7 天的日志目录（find + rm）", WORK, 8)
ok3 = os.path.exists(os.path.join(WORK, "clean_logs.sh"))
check("写脚本", ok3)

# 4. 读代码+总结
out = run_agent("阅读 checker.go（工作区内），用 write 创建 checker_notes.txt 总结这个文件实现了什么（2-3 句）", WORK, 12)
ok4 = os.path.exists(os.path.join(WORK, "checker_notes.txt"))
check("读代码+总结", ok4)

# 5. 读文件+提取
out = run_agent("用 read 读取 logger.go 前 30 行，write 创建 logger_head.txt 记录主要内容", WORK, 10)
ok5 = os.path.exists(os.path.join(WORK, "logger_head.txt"))
check("读文件+提取", ok5)

# 6. 综合（读+总结+写）
shutil.copy(os.path.join(ZERG, "docs/设计-系统回馈机制.md"), os.path.join(WORK, "feedback.md"))
out = run_agent("用 read 读取 feedback.md（工作区）前 40 行，用 write 创建 feedback_summary.txt 总结核心思想（2-3 句）", WORK, 12)
ok6 = os.path.exists(os.path.join(WORK, "feedback_summary.txt"))
check("综合(读+总结+写)", ok6)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

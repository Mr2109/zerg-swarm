#!/usr/bin/env python3
"""v25_agent_tasks.py — v2.5 Agent 多工具实战测试（2026-08-13）
测试 4 类任务：写文件/读文件/改文件/bash 执行——验证 Agent 全工具能力
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
WORK = "/tmp/zerg-agent-tasks"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=10, model="example-8b-quant"):
    r = subprocess.run(
        f'go run ./cmd/zerg-agent -task "{task}" -model {model} -workdir {workdir} -max-turns {max_turns} 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, cwd=CORE)
    return r.stdout + r.stderr


# 准备
os.makedirs(WORK, exist_ok=True)

# 1. 写文件（已验证）
out = run_agent("创建文件 write_test.txt 内容为 zerg-agent-write", WORK, 8)
ok_file = os.path.exists(os.path.join(WORK, "write_test.txt"))
content = ""
if ok_file:
    content = open(os.path.join(WORK, "write_test.txt")).read()
check("写文件", ok_file and content == "zerg-agent-write", f"内容={content!r}")

# 2. 读文件（先创建基准文件）
with open(os.path.join(WORK, "read_test.txt"), "w") as f:
    f.write("read-me-content-42")
out = run_agent("读取文件 read_test.txt 的内容并汇报", WORK, 8)
check("读文件", "read-me-content-42" in out, "Agent 输出含内容" if "read-me-content-42" in out else "未汇报")

# 3. 改文件（edit——替换内容）
with open(os.path.join(WORK, "edit_test.txt"), "w") as f:
    f.write("old-value-123")
out = run_agent("编辑文件 edit_test.txt 把 old-value-123 替换为 new-value-999 使用 edit 工具", WORK, 10)
edit_content = open(os.path.join(WORK, "edit_test.txt")).read() if os.path.exists(os.path.join(WORK, "edit_test.txt")) else ""
check("改文件(edit)", edit_content == "new-value-999", f"内容={edit_content!r}")

# 4. bash 执行
out = run_agent("用 bash 工具执行 ls 并汇报有哪些文件", WORK, 8)
check("bash 执行", "bash" in out.lower() or any(f in out for f in ["write_test.txt", "read_test.txt"]),
      "Agent 执行了 bash" if "bash" in out.lower() else "未确认 bash 执行")

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

#!/usr/bin/env python3
"""v25_more_tasks.py — v2.5 更多实战任务（2026-08-13）
编码/文件操作向——5 个新任务
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
WORK = "/tmp/zerg-agent-more"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=12, model="example-8b-quant", tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd core && go run ./cmd/zerg-agent -task "{task}" -model {model} -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=350, errors="replace")
    return r.stdout + r.stderr


os.makedirs(WORK, exist_ok=True)

# 任务 A: 多文件创建（write 连续 3 文件）
out = run_agent("依次创建三个文件：a.py 内容 def a()，b.py 内容 def b()，c.py 内容 def c()。用 write 工具", WORK, 12, tools="write")
okA = all(os.path.exists(os.path.join(WORK, f"{x}.py")) for x in ["a", "b", "c"])
check("多文件创建", okA)

# 任务 B: 文件列表+读取（ls 后 read 指定文件）
with open(os.path.join(WORK, "config.json"), "w") as f:
    f.write('{"port": 8080, "host": "localhost"}')
out = run_agent("用 ls 列出工作区文件，然后读取 config.json 汇报端口号", WORK, 10, tools="ls,read")
okB = "8080" in out
check("列表+读取", okB, "" if okB else "未汇报端口")

# 任务 C: 追加内容（read 后 write 覆盖——保留原内容+新行）
with open(os.path.join(WORK, "log.txt"), "w") as f:
    f.write("line1\n")
out = run_agent("读取 log.txt 内容，然后写入新内容（原内容加一行 line2）", WORK, 10, tools="read,write")
logc = open(os.path.join(WORK, "log.txt")).read() if os.path.exists(os.path.join(WORK, "log.txt")) else ""
okC = "line1" in logc and "line2" in logc
check("追加内容", okC, f"log={logc!r}" if not okC else "")

# 任务 D: 重命名（bash mv）
with open(os.path.join(WORK, "old_name.txt"), "w") as f:
    f.write("rename-me")
out = run_agent("用 bash 把 old_name.txt 重命名为 new_name.txt", WORK, 8, tools="bash")
okD = os.path.exists(os.path.join(WORK, "new_name.txt"))
check("文件重命名", okD)

# 任务 E: 综合——建目录+写文件+验证（bash mkdir + write + bash ls 验证）
out = run_agent("用 bash 创建目录 project，然后写 project/main.go 内容 package main，最后用 bash ls project 验证", WORK, 12, tools="bash,write")
okE = os.path.exists(os.path.join(WORK, "project/main.go"))
check("综合任务", okE)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

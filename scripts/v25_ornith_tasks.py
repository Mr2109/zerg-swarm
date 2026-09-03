#!/usr/bin/env python3
"""v25_example-35b-v2_tasks.py — v2.5 Agent 大量任务测试（example-35b-v2 主模型——2026-08-13）
覆盖：写/读/编辑/搜索/多文件/完成确认——验证实战稳定性
"""

import os
import subprocess
import sys
import argparse

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
WORK = "/tmp/zerg-agent-example-35b-v2"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=8, tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd core && go run ./cmd/zerg-agent -task "{task}" -model {MODEL} -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, errors="replace")
    return r.stdout + r.stderr



# 模型参数（可复用——对比测试）
parser = argparse.ArgumentParser()
parser.add_argument("--model", default="example-35b", help="模型名")
parser.add_argument("--workdir", default=WORK, help="工作区目录")
args = parser.parse_args()
MODEL = args.model
WORK = args.workdir
os.makedirs(WORK, exist_ok=True)

# 1. 写文件（单文件）
out = run_agent("创建文件 a.txt 内容为 example-35b-v2-write", WORK)
c = open(os.path.join(WORK, "a.txt")).read() if os.path.exists(os.path.join(WORK, "a.txt")) else ""
check("写文件", c == "example-35b-v2-write", repr(c))

# 2. 多文件创建
out = run_agent("依次创建 f1.py 内容 x、f2.py 内容 y、f3.py 内容 z", WORK, 10)
ok2 = all(os.path.exists(os.path.join(WORK, f"{n}.py")) for n in ["f1", "f2", "f3"])
check("多文件创建", ok2)

# 3. 读文件+汇报
with open(os.path.join(WORK, "data.txt"), "w") as f:
    f.write("hello-example-35b-v2-42")
out = run_agent("读取 data.txt 内容并汇报", WORK)
check("读文件", "hello-example-35b-v2-42" in out)

# 4. 编辑文件
with open(os.path.join(WORK, "edit.txt"), "w") as f:
    f.write("old-value-777")
out = run_agent("编辑 edit.txt 把 old-value-777 改为 new-value-888", WORK, 10)
c4 = open(os.path.join(WORK, "edit.txt")).read() if os.path.exists(os.path.join(WORK, "edit.txt")) else ""
check("编辑文件", "new-value-888" in c4, repr(c4))

# 5. bash 执行
out = run_agent("用 bash 执行 ls 并汇报", WORK)
ok5 = "a.txt" in out or "edit.txt" in out
check("bash执行", ok5)

# 6. grep 搜索
os.makedirs(os.path.join(WORK, "src"), exist_ok=True)
with open(os.path.join(WORK, "src/app.py"), "w") as f:
    f.write("def run():\n    pass\n\ndef stop():\n    pass\n")
out = run_agent("用 grep 在 src 搜索 def 并汇报函数", WORK, 8, tools="grep,read")
ok6 = "run" in out and "stop" in out
check("grep搜索", ok6)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

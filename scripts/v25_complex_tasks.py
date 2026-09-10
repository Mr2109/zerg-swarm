#!/usr/bin/env python3
"""v25_complex_tasks.py — v2.5 Agent 复合任务测试（example-35b-v2——2026-08-13）
多工具协作完整任务链——模拟真实干活
"""

import os
import subprocess
import sys
import argparse

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 仓库根（2026-09-11 B 批：去私有路径）

PASS, FAIL = "✅", "❌"
CORE = f"{REPO}/core"
WORK = "/tmp/zerg-agent-complex"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=15):
    r = subprocess.run(
        f'cd core && go run ./cmd/zerg-agent -task "{task}" -model {MODEL} -workdir {workdir} -max-turns {max_turns} 2>&1',
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

# 1. 项目搭建（bash 建目录 + write 多文件 + bash 验证）
out = run_agent("搭建小项目：bash 创建目录 src，write 创建 src/main.py 内容 print('hello')，bash 执行 python3 src/main.py 验证", WORK)
ok1 = os.path.exists(os.path.join(WORK, "src/main.py"))
check("项目搭建(建目录+写+运行)", ok1)

# 2. Bug 修复（read 定位 + edit 修复 + bash 验证）
with open(os.path.join(WORK, "buggy.py"), "w") as f:
    f.write("def div(a, b):\n    return a / b\n\nprint(div(10, 0))\n")
out = run_agent("修复 buggy.py：read 读取，edit 把 a / b 改为 a // b，bash 运行验证不崩溃", WORK)
c2 = open(os.path.join(WORK, "buggy.py")).read() if os.path.exists(os.path.join(WORK, "buggy.py")) else ""
ok2 = "a // b" in c2
check("Bug修复(读+改+验证)", ok2, repr(c2[:60]) if not ok2 else "")

# 3. 数据整理（read 读数据 + 分析 + write 写结果）
with open(os.path.join(WORK, "scores.csv"), "w") as f:
    f.write("name,score\nAlice,90\nBob,80\nCarol,95\n")
out = run_agent("读取 scores.csv 找出分数最高的人，写入 result.txt", WORK)
c3 = open(os.path.join(WORK, "result.txt")).read() if os.path.exists(os.path.join(WORK, "result.txt")) else ""
ok3 = "Carol" in c3
check("数据整理(读+分析+写)", ok3, repr(c3[:50]) if not ok3 else "")

# 4. 文档生成（grep 搜函数 + read 读实现 + write 写文档）
os.makedirs(os.path.join(WORK, "api"), exist_ok=True)
with open(os.path.join(WORK, "api/endpoints.py"), "w") as f:
    f.write("# API\n\ndef get_user():\n    pass\n\ndef post_order():\n    pass\n")
out = run_agent("grep 在 api 搜 def 找出函数，write 创建 docs/api.md 列出这些函数", WORK)
c4 = open(os.path.join(WORK, "docs/api.md")).read() if os.path.exists(os.path.join(WORK, "docs/api.md")) else ""
ok4 = "get_user" in c4 and "post_order" in c4
check("文档生成(grep+写)", ok4, repr(c4[:80]) if not ok4 else "")

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

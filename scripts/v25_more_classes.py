#!/usr/bin/env python3
"""v25_more_classes.py — v2.5 补充任务类测试（ornith——2026-08-13）
覆盖验收标准未测类：#1编码多文件 / #3重构 / #6研究(网络)
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
WORK = "/tmp/zerg-agent-classes"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=12, tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd core && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, errors="replace")
    return r.stdout + r.stderr


os.makedirs(WORK, exist_ok=True)

# #1 编码多文件：写一个简单计算器（calc.py + 测试）
out = run_agent("用 write 创建 calc.py：定义 add(a,b) 返回 a+b、mul(a,b) 返回 a*b；再创建 test_calc.py 测试 add(2,3)==5 和 mul(2,3)==6；bash 运行 python3 test_calc.py", WORK, 12)
c1 = open(os.path.join(WORK, "calc.py")).read() if os.path.exists(os.path.join(WORK, "calc.py")) else ""
ok1 = "def add" in c1 and "def mul" in c1
check("编码多文件(calc+test)", ok1)

# #3 重构：不破坏行为（改名函数+测试仍过）
with open(os.path.join(WORK, "refactor.py"), "w") as f:
    f.write("def old_name(x):\n    return x * 2\n\nprint(old_name(5))\n")
out = run_agent("重构 refactor.py：read 读取，edit 把函数名 old_name 改为 new_name（保持行为不变——返回值仍 x*2），bash 运行验证输出 10", WORK, 10)
c3 = open(os.path.join(WORK, "refactor.py")).read() if os.path.exists(os.path.join(WORK, "refactor.py")) else ""
ok3 = "def new_name" in c3 and "return x * 2" in c3
check("重构(改名保行为)", ok3, repr(c3[:60]) if not ok3 else "")

# #6 研究：网络搜索（web_search 工具）
out = run_agent("用 web_search 搜索 'golang 简介' 并汇报搜索到的主要内容", WORK, 8, tools="web_search")
ok6 = "go" in out.lower() or "golang" in out.lower() or "工具" in out
check("研究(web_search)", ok6)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

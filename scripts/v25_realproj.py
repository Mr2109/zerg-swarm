#!/usr/bin/env python3
"""v25_realproj.py — v2.5 真实项目任务集（2026-08-13）
在 zerg-evals 真实 Go 模块——多文件/重构/复杂任务
"""

import os
import shutil
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 仓库根（2026-09-11 B 批：去私有路径）

PASS, FAIL = "✅", "❌"
CORE = f"{REPO}/core"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=15):
    r = subprocess.run(
        f'cd "{CORE}" && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns {max_turns} 2>&1',
        shell=True, capture_output=True, text=True, timeout=280, errors="replace")
    return r.stdout + r.stderr


WORK = "/tmp/zerg-realproj2"
os.makedirs(WORK, exist_ok=True)

# 1. 多文件项目搭建（3 个文件 + 测试 + 运行）
out = run_agent("用 write 创建 3 个文件：calc.go（Add(a,b)返回a+b、Mul(a,b)返回a*b）、main.go（调用 Add(2,3) 打印结果）、calc_test.go（测试 Add(2,3)==5），然后 bash 运行 go test 和 go run 验证", WORK)
ok1 = all(os.path.exists(os.path.join(WORK, f)) for f in ["calc.go", "main.go", "calc_test.go"])
check("多文件项目搭建", ok1)

# 2. 重构（函数改名保行为 + 测试验证）
with open(os.path.join(WORK, "calc.go"), "w") as f:
    f.write("package main\n\nfunc Sum2(a, b int) int {\n\treturn a + b\n}\n")
with open(os.path.join(WORK, "calc_test.go"), "w") as f:
    f.write('package main\n\nimport "testing"\n\nfunc TestSum2(t *testing.T) {\n\tif Sum2(1, 2) != 3 { t.Fatal("Sum2(1,2) != 3") }\n}\n')
out = run_agent("重构 calc.go：read 读取，edit 把函数名 Sum2 改为 Add（保持行为不变——返回 a+b），bash 运行 go test 验证测试仍通过", WORK)
c2 = open(os.path.join(WORK, "calc.go")).read() if os.path.exists(os.path.join(WORK, "calc.go")) else ""
ok2 = "func Add" in c2 and "a + b" in c2
check("重构(改名保行为)", ok2)

# 3. 复杂功能（读取文件处理 + 输出）
with open(os.path.join(WORK, "data.txt"), "w") as f:
    f.write("apple\nbanana\napple\ncherry\nbanana\napple\n")
out = run_agent("读取 data.txt（read 工具），统计每个词出现次数（分析），write 创建 stats.txt 写入统计结果（格式：词 次数，每行一个）", WORK)
c3 = open(os.path.join(WORK, "stats.txt")).read() if os.path.exists(os.path.join(WORK, "stats.txt")) else ""
ok3 = "apple" in c3 and "3" in c3
check("数据分析(读+统计+写)", ok3, repr(c3[:60]) if not ok3 else "")

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

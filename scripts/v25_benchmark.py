#!/usr/bin/env python3
"""v25_benchmark.py — v2.5 Codex vs 虫族 Agent 对标测试（2026-08-13）
同一任务集——对比完成率（验收标准：替代率 ≥80%）
"""

import os
import subprocess
import sys

WORK = "/tmp/zerg-benchmark"
os.makedirs(WORK, exist_ok=True)

TASKS = [
    ("写文件", "创建文件 hello.txt 内容为 benchmark-write"),
    ("Bug修复", "修复 buggy.py：read 读取，edit 把 a / b 改为 a // b"),
    ("数据分析", "读取 scores.csv 找出分数最高的人，写入 result.txt"),
    ("文档生成", "grep 在 api 搜 def 找出函数，write 创建 docs/api.md 列出函数"),
]


def run_codex(task, workdir):
    r = subprocess.run(
        f'NO_PROXY="127.0.0.1,localhost" codex exec --skip-git-repo-check "{task}" 2>&1',
        shell=True, capture_output=True, text=True, timeout=180, errors="replace")
    return r.stdout + r.stderr


def run_agent(task, workdir):
    r = subprocess.run(
        f'cd "<repo>/core" && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns 10 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, errors="replace")
    return r.stdout + r.stderr


def verify(task_name, workdir):
    if task_name == "写文件":
        p = os.path.join(workdir, "hello.txt")
        return os.path.exists(p) and "benchmark-write" in open(p).read()
    if task_name == "Bug修复":
        p = os.path.join(workdir, "buggy.py")
        return os.path.exists(p) and "a // b" in open(p).read()
    if task_name == "数据分析":
        p = os.path.join(workdir, "result.txt")
        return os.path.exists(p) and "Carol" in open(p).read()
    if task_name == "文档生成":
        p = os.path.join(workdir, "docs/api.md")
        return os.path.exists(p) and "get_user" in open(p).read()
    return False


print("=" * 50)
print("🐛 Codex vs 虫族 Agent 对标（ornith 同模型）")
print("=" * 50)

for name, task in TASKS:
    # Codex
    cw = os.path.join(WORK, "codex")
    os.makedirs(cw, exist_ok=True)
    os.chdir(cw)
    # 准备输入文件
    if name == "Bug修复":
        open("buggy.py", "w").write("def div(a, b):\n    return a / b\n\nprint(div(10, 0))\n")
    if name == "数据分析":
        open("scores.csv", "w").write("name,score\nAlice,90\nBob,80\nCarol,95\n")
    if name == "文档生成":
        os.makedirs("api", exist_ok=True)
        open("api/endpoints.py", "w").write("# API\n\ndef get_user():\n    pass\n\ndef post_order():\n    pass\n")
    try:
        codex_out = run_codex(task, cw)
        codex_ok = verify(name, cw)
    except Exception as e:
        codex_ok = False
        codex_out = str(e)

    # Agent
    aw = os.path.join(WORK, "agent")
    os.makedirs(aw, exist_ok=True)
    if name == "Bug修复":
        open(os.path.join(aw, "buggy.py"), "w").write("def div(a, b):\n    return a / b\n\nprint(div(10, 0))\n")
    if name == "数据分析":
        open(os.path.join(aw, "scores.csv"), "w").write("name,score\nAlice,90\nBob,80\nCarol,95\n")
    if name == "文档生成":
        os.makedirs(os.path.join(aw, "api"), exist_ok=True)
        open(os.path.join(aw, "api/endpoints.py"), "w").write("# API\n\ndef get_user():\n    pass\n\ndef post_order():\n    pass\n")
    try:
        agent_out = run_agent(task, aw)
        agent_ok = verify(name, aw)
    except Exception as e:
        agent_ok = False
        agent_out = str(e)

    print(f"{name}: Codex={'✅' if codex_ok else '❌'} | Agent={'✅' if agent_ok else '❌'}")

print("=" * 50)

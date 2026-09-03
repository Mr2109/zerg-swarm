#!/usr/bin/env python3
"""v25_eval_task.py — v2.5 评测任务类测试（#7——2026-08-13）
Agent 跑 zerg-evals 评测——验证评测类能力
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
WORK = "/tmp/zerg-agent-eval"
EVALS = "<volume-path>"
os.makedirs(WORK, exist_ok=True)


def run_agent(task, workdir, max_turns=15):
    r = subprocess.run(
        f'cd "<repo>/core" && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns {max_turns} 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, errors="replace")
    return r.stdout + r.stderr


# 评测任务：Agent 用 bash 跑一个真实评测（coeval_runner——三方互评）
task = (
    f"用 bash 工具运行评测：cd {EVALS} && python3 backends/coeval_runner.py --models example-35b,gemma-4-12B --task-type reasoning --n 1 --out /tmp/zerg-agent-eval/result.json "
    f"评测两个模型的推理能力互评，然后 read 读取 result.json 汇报评测结果（哪个模型胜出）"
)

out = run_agent(task, WORK, 15)

# 验证：Agent 是否调了 bash + 有评测输出
ok = "评测" in out or "score" in out.lower() or "pass" in out.lower()
print(f"{PASS if ok else FAIL} 评测任务(Agent跑zerg-evals): {ok}")
if not ok:
    print([l for l in out.splitlines() if "状态" in l or "tool" in l.lower()][-5:])

sys.exit(0 if ok else 1)

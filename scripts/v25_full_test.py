#!/usr/bin/env python3
"""v25_full_test.py — v2.5 全面测试（2026-08-13）
覆盖：单元测试/编译/工具/循环/上下文/记忆/日志/真实任务
"""

import json
import os
import subprocess
import sys
import tempfile

PASS, FAIL = "✅", "❌"
CORE = f"{REPO}/core"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run(cmd, cwd=None, timeout=180):
    r = subprocess.run(cmd + " 2>&1; echo __RC__=$?", shell=True, capture_output=True, text=True, timeout=timeout, cwd=cwd)
    out = r.stdout
    rc = 0
    for line in out.splitlines():
        if line.startswith("__RC__="):
            rc = int(line.split("=")[1])
    return rc, out[-400:]


# ===== 1. 编译 =====
rc, out = run("go build ./...", cwd=CORE)
check("全量编译", rc == 0, out.strip()[-60:])

# ===== 2. 单元测试 =====
rc, out = run("go test ./internal/agent/ -v 2>&1 | grep -c PASS", cwd=CORE)
check("单元测试", rc == 0, f"{out.strip().splitlines()[-1] if out.strip() else '?'} 个 PASS")

# ===== 3. go vet =====
rc, out = run("go vet ./internal/agent/", cwd=CORE)
check("go vet 干净", rc == 0)

# ===== 4. 模块文件完整性 =====
agent_dir = os.path.join(CORE, "internal/agent")
files = os.listdir(agent_dir)
expected = ["agent.go", "types.go", "tools.go", "exec.go", "loop.go", "checker.go",
            "context.go", "net_tools.go", "recovery.go", "subagent.go", "memory.go", "logger.go"]
missing = [f for f in expected if f not in files]
check("12 模块文件", len(missing) == 0, f"缺: {missing}" if missing else "全部在")

# ===== 5. 工具定义 =====
tools_src = open(os.path.join(agent_dir, "tools.go")).read()
tools = ["bash", "read", "write", "edit", "glob", "grep", "ls"]
miss_tools = [t for t in tools if t not in tools_src]
check("7 核心工具", len(miss_tools) == 0)

# ===== 6. 循环终止状态 =====
# 终止状态常量在 agent.go（ReasonComplete 等）
agent_src = open(os.path.join(agent_dir, "agent.go")).read()
loop_src = open(os.path.join(agent_dir, "loop.go")).read()
reasons = ["complete", "max_turns", "token_budget", "user_abort", "model_error", "tool_error", "escalate", "blocked"]
miss_r = [r for r in reasons if f'"{r}"' not in agent_src]
check("循环终止状态", len(miss_r) == 0, f"缺: {miss_r}" if miss_r else "8+ 状态在")

# ===== 7. 日志系统 =====
logger_src = open(os.path.join(agent_dir, "logger.go")).read()
logs = ["events.jsonl", "errors.jsonl", "audit.jsonl", "transcript.jsonl"]
miss_l = [l for l in logs if l not in logger_src]
check("4 类日志文件", len(miss_l) == 0)

# ===== 8. 记忆 =====
mem_src = open(os.path.join(agent_dir, "memory.go")).read()
check("记忆系统", "MemoryStore" in mem_src and "Search" in mem_src)

# ===== 9. 子 agent =====
sub_src = open(os.path.join(agent_dir, "subagent.go")).read()
check("子 agent", "SpawnSubagent" in sub_src or "Subagent" in sub_src)

# ===== 10. 日志实际写入验证 =====
tmpdir = tempfile.mkdtemp()
test_code = '''
package main

import (

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # 仓库根（2026-09-11 B 批：去私有路径）
    "fmt"
    "os"
    "zerg/core/internal/agent"
)

func main() {
    dir := os.Args[1]
    l, err := agent.NewLogger("test_full", dir)
    if err != nil { fmt.Println("ERR:", err); os.Exit(1) }
    defer l.Close()
    l.LogEvent(string(agent.EventToolCall), "info", "execute", "bash", "测试", map[string]any{"cmd": "ls"}, "成功", "", "0.1s")
    l.LogError("tool_error", "测试错误", "上下文")
    l.LogAudit("write", map[string]any{"path": "/tmp/x"}, "写文件", "allow", "成功", "", "agent")
    l.LogTranscript("user", "修复", "", "", nil, "", "", "")
    for _, name := range []string{"events.jsonl", "errors.jsonl", "audit.jsonl", "transcript.jsonl"} {
        p := dir + "/" + name
        data, err := os.ReadFile(p)
        if err != nil || len(data) == 0 { fmt.Println("FAIL:", name); os.Exit(1) }
    }
    fmt.Println("LOGS_OK")
}
'''
# go run 必须放 module 内（internal 包限制）——用 cmd/logtest
logtest_dir = os.path.join(CORE, "cmd/logtest")
os.makedirs(logtest_dir, exist_ok=True)
with open(os.path.join(logtest_dir, "main.go"), "w") as f:
    f.write(test_code)
logdir = os.path.join(tmpdir, "logs")
rc, out = run(f"go run ./cmd/logtest {logdir}", cwd=CORE)
check("日志实际写入", rc == 0 and "LOGS_OK" in out, out.strip()[-60:])

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

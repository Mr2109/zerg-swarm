#!/usr/bin/env python3
"""v25_m1_accept.py — v2.5 M1a+M1b 验收（2026-08-13）
检查：agent 包编译 + 7 工具定义 + exec 执行 + gate 引用
"""

import json
import subprocess
import sys

PASS, FAIL = "✅", "❌"
CORE = "<repo>/core"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run(cmd, cwd=None):
    r = subprocess.run(cmd + " 2>&1; echo __RC__=$?", shell=True, capture_output=True, text=True, timeout=180, cwd=cwd)
    out = r.stdout
    rc = 0
    for line in out.splitlines():
        if line.startswith("__RC__="):
            rc = int(line.split("=")[1])
    return rc, out[-300:]


# 1. 编译
rc, out = run("go build ./...", cwd=CORE)
check("agent 包编译", rc == 0, out.strip()[-80:])

# 2. go vet
rc, out = run("go vet ./internal/agent/", cwd=CORE)
check("go vet 干净", rc == 0, out.strip()[-80:])

# 3. 7 工具定义存在
import os
agent_dir = os.path.join(CORE, "internal/agent")
files = os.listdir(agent_dir)
check("agent 目录文件", len(files) >= 3, f"{len(files)} 个文件: {files}")

# 4. tools.go 含 7 工具
tools_src = ""
for f in ["tools.go"]:
    p = os.path.join(agent_dir, f)
    if os.path.exists(p):
        tools_src = open(p).read()
tools = ["bash", "read", "write", "edit", "glob", "grep", "ls"]
missing = [t for t in tools if f'"{t}"' not in tools_src and t not in tools_src]
check("7 工具定义", len(missing) == 0, f"缺: {missing}" if missing else "全部在")

# 5. exec.go 工具执行存在
exec_src = ""
for f in ["exec.go"]:
    p = os.path.join(agent_dir, f)
    if os.path.exists(p):
        exec_src = open(p).read()
check("exec.go 执行", "func" in exec_src and ("bash" in exec_src or "Bash" in exec_src))

# 6. gate 引用
check("gate 检查引用", "gate" in exec_src or "Gate" in exec_src)

# 7. 测试跑
rc, out = run("go test ./internal/agent/", cwd=CORE)
check("agent 测试", rc == 0, out.strip()[-80:])

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

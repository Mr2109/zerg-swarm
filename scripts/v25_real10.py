#!/usr/bin/env python3
"""v25_real10.py — v2.5 Agent 10 个真实小任务（2026-08-13）
从虫族仓库真实待办拆解——单文件粒度——真实价值+真实检验
"""

import os
import subprocess
import sys

PASS, FAIL = "✅", "❌"
ZERG = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORE = os.path.join(ZERG, "core")
EVALS = os.environ.get("ZERG_EVALS_DIR", "")
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run_agent(task, workdir, max_turns=12, tools=""):
    tflag = f" -tools {tools}" if tools else ""
    r = subprocess.run(
        f'cd "{CORE}" && go run ./cmd/zerg-agent -task "{task}" -model example-35b -workdir {workdir} -max-turns {max_turns}{tflag} 2>&1',
        shell=True, capture_output=True, text=True, timeout=300, errors="replace")
    return r.stdout + r.stderr


os.makedirs("/tmp/zerg-real10", exist_ok=True)

# 1. 写测试用例（真实：给 memory.go 补测试）
out = run_agent("阅读 {ZERG}/core/internal/agent/memory.go，用 write 创建 memory_extra_test.go 测试 WriteFact 和 Search 函数（测试内容简单即可：写入一个事实，搜索能搜到）", "/tmp/zerg-real10", 15)
ok1 = os.path.exists(os.path.join("/tmp/zerg-real10", "memory_extra_test.go"))
check("补测试(memory_extra_test.go)", ok1)

# 2. 写文档（真实：给系统回馈机制写使用说明）
out = run_agent("用 write 创建 /tmp/zerg-real10/反馈机制说明.md——简述虫族 Agent 系统回馈机制：模型犯错时系统给诊断反馈，模型据此纠正。100 字以内", "/tmp/zerg-real10", 8)
ok2 = os.path.exists(os.path.join("/tmp/zerg-real10", "反馈机制说明.md"))
check("写文档(反馈机制说明)", ok2)

# 3. 写脚本（真实：清理日志脚本）
out = run_agent("用 write 创建 /tmp/zerg-real10/clean_logs.sh——清理 .zerg/logs 下超过 7 天的日志目录（find + rm 命令）", "/tmp/zerg-real10", 8)
ok3 = os.path.exists(os.path.join("/tmp/zerg-real10", "clean_logs.sh"))
check("写脚本(clean_logs.sh)", ok3)

# 4. 数据分析（真实：统计仓库 Go 文件数）
out = run_agent("用 bash 统计 <repo>/core 下所有 .go 文件数量，write 创建 /tmp/zerg-real10/go_files.txt 写入数量", "/tmp/zerg-real10", 10)
ok4 = os.path.exists(os.path.join("/tmp/zerg-real10", "go_files.txt"))
check("统计(go文件数)", ok4)

# 5. 修复真实 bug（模拟：读代码找问题）
out = run_agent("阅读 <repo>/core/internal/agent/checker.go，用 write 创建 /tmp/zerg-real10/checker_notes.txt 总结这个文件实现了什么（2-3 句）", "/tmp/zerg-real10", 10)
ok5 = os.path.exists(os.path.join("/tmp/zerg-real10", "checker_notes.txt"))
check("代码理解(checker总结)", ok5)

# 6. 配置文件操作（真实：改配置）
out = run_agent("用 read 读取 <repo>/core/internal/agent/logger.go 前 30 行，write 创建 /tmp/zerg-real10/logger_head.txt 记录主要内容", "/tmp/zerg-real10", 10)
ok6 = os.path.exists(os.path.join("/tmp/zerg-real10", "logger_head.txt"))
check("读文件操作(logger头)", ok6)

# 7. 搜索+总结（真实：找函数定义）
out = run_agent("用 grep 在 <repo>/core/internal/agent/ 搜索 'func diagnoseFailure' 找到它所在的文件和行号，write 创建 /tmp/zerg-real10/diag_loc.txt", "/tmp/zerg-real10", 10, tools="grep,write")
ok7 = os.path.exists(os.path.join("/tmp/zerg-real10", "diag_loc.txt"))
check("搜索定位(diagnoseFailure)", ok7)

# 8. 环境检查（真实：查端口）
out = run_agent("用 bash 执行 lsof -i :8082 检查 8082 端口占用情况，write 创建 /tmp/zerg-real10/port8082.txt 记录结果摘要", "/tmp/zerg-real10", 8)
ok8 = os.path.exists(os.path.join("/tmp/zerg-real10", "port8082.txt"))
check("环境检查(端口8082)", ok8)

# 9. 生成配置文件（真实：JSON 配置）
out = run_agent("用 write 创建 /tmp/zerg-real10/agent_config.json——内容 {\"model\":\"example-35b\",\"temperature\":0.8,\"max_turns\":30}", "/tmp/zerg-real10", 8)
ok9 = os.path.exists(os.path.join("/tmp/zerg-real10", "agent_config.json"))
check("生成JSON配置", ok9)

# 10. 综合（真实：多工具——读README+总结）
out = run_agent("用 read 读取 <repo>/docs/设计-系统回馈机制.md 前 40 行，用 write 创建 /tmp/zerg-real10/feedback_summary.txt 总结核心思想（2-3 句）", "/tmp/zerg-real10", 12)
ok10 = os.path.exists(os.path.join("/tmp/zerg-real10", "feedback_summary.txt"))
check("综合(读+总结+写)", ok10)

print("\n=== 总结 ===")
ok = sum(1 for _, o, _ in results if o)
print(f"{ok}/{len(results)} 通过")
sys.exit(0 if ok == len(results) else 1)

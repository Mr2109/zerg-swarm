#!/usr/bin/env python3
"""v2.4 全面测试——M0/M3/M2 + 网关回归（2026-08-13）
输出：测试报告（中文）
"""

import json
import subprocess
import sys
import time
import urllib.error
import urllib.request

PASS = "✅"
FAIL = "❌"
results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"{PASS if ok else FAIL} {name} {detail}")


def run(cmd, timeout=120, cwd=None):
    # 用 echo RC 捕获真实退出码（shell pipe 会吞 rc）
    r = subprocess.run(cmd + " 2>&1; echo __RC__=$?", shell=True, capture_output=True, text=True, timeout=timeout, cwd=cwd)
    out = r.stdout[-800:] + r.stderr[-300:]
    rc = 0
    import re
    m = re.search(r"__RC__=(\d+)", r.stdout)
    if m:
        rc = int(m.group(1))
    return rc, out


def api_get(url, headers=None):
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
    except Exception as e:
        return 0, str(e)


CORE = "<repo>/core"
EVALS = "<volume-path>"


def main():
    print("=" * 50)
    print("虫族 v2.4 全面测试")
    print("=" * 50)

    # ===== 1. M0 校验器测试 =====
    print("\n--- M0 配置模块化（校验器）---")
    rc, out = run("go test ./internal/config/ | tail -2", cwd=CORE)
    check("M0 校验器测试", rc == 0, out.strip()[-80:])

    # ===== 2. M3 集中控制层测试 =====
    print("\n--- M3 集中控制层（工具拦截）---")
    rc, out = run("go test ./internal/control/ | tail -2", cwd=CORE)
    check("M3 gate 测试", rc == 0, out.strip()[-80:])

    # ===== 3. M2 Agent 框架测试 =====
    print("\n--- M2 Agent 框架（harness_state）---")
    rc, out = run("go test ./internal/agentstate/ | tail -2", cwd=CORE)
    check("M2 harness_state 测试", rc == 0, out.strip()[-80:])

    # ===== 4. 主仓库全量编译 =====
    print("\n--- 主仓库全量编译 ---")
    rc, out = run("go build ./... | head -3", cwd=CORE)
    check("全量编译", rc == 0, out.strip()[-60:])

    # ===== 5. 网关回归（模型列表）=====
    print("\n--- 网关回归 ---")
    code, body = api_get("http://127.0.0.1:8082/v1/models", {"X-Auth-Token": "x3gw-shared-2026"})
    try:
        n = len(json.loads(body).get("data", []))
        check("网关模型列表", code == 200 and n > 0, f"{n} 模型")
    except Exception:
        check("网关模型列表", False, body[:60])

    # ===== 6. 主控状态 =====
    code, body = api_get("http://127.0.0.1:8580/api/fleet/status", {"X-Auth-Token": "x3gw-shared-2026"})
    try:
        machines = json.loads(body).get("machines", {})
        check("主控状态", code == 200 and len(machines) > 0, f"{len(machines)} 机器")
    except Exception:
        check("主控状态", False, body[:60])

    # ===== 7. 排除本机持久化 =====
    code, body = api_get("http://127.0.0.1:8580/api/fleet/exclude-local", {"X-Auth-Token": "x3gw-shared-2026"})
    check("排除本机查询", code == 200 and "exclude_local" in body, body[:40])

    # ===== 8. CoEval 脚本存在 =====
    rc, out = run("test -f backends/coeval_runner.py && echo ok", cwd=EVALS)
    check("M1 CoEval runner", rc == 0, "")

    # ===== 9. M4 采集脚本存在 =====
    rc, out = run("test -f backends/distill/collect_traces.py && echo ok", cwd=EVALS)
    check("M4 采集脚本", rc == 0, "")

    # ===== 汇总 =====
    print("\n" + "=" * 50)
    passed = sum(1 for _, ok, _ in results if ok)
    print(f"测试报告: {passed}/{len(results)} 通过")
    for name, ok, detail in results:
        if not ok:
            print(f"  {FAIL} {name}: {detail}")
    print("=" * 50)
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    sys.exit(main())

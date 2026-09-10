#!/usr/bin/env python3
"""compare.py — 虫族 Agent 测试结果对比（2026-08-13）
用法: python3 compare.py <run1> <run2>
输出: 各层通过率对比 + 每项 PASS/FAIL 变化（提升/退化/持平）
"""

import os
import re
import sys


def parse_run(run_dir):
    """解析一次运行的测试结果"""
    results = {}
    for layer in ["layer2-basic", "layer3-complex"]:
        path = os.path.join(run_dir, f"{layer}.txt")
        if not os.path.exists(path):
            continue
        with open(path) as f:
            content = f.read()
        # 解析 ✅/❌ 行
        for line in content.splitlines():
            m = re.match(r'([✅❌]) (.+?)(?:\s+\S+)?$', line.strip())
            if m:
                status = m.group(1)
                name = m.group(2).strip()
                results[f"{layer}:{name}"] = (status == "✅", line.strip())
    return results


def main():
    if len(sys.argv) < 3:
        base = os.path.join(os.environ.get("ZERG_EVALS_DIR", ""), "results/agent/runs")
        runs = sorted(os.listdir(base)) if os.path.exists(base) else []
        print("用法: compare.py <run1> <run2>")
        print(f"可用运行: {runs}")
        return

    r1, r2 = sys.argv[1], sys.argv[2]
    base = os.path.join(os.environ.get("ZERG_EVALS_DIR", ""), "results/agent/runs")
    d1 = os.path.join(base, r1)
    d2 = os.path.join(base, r2)

    res1 = parse_run(d1)
    res2 = parse_run(d2)

    print(f"📊 对比: {r1} vs {r2}")
    print("=" * 50)

    all_keys = set(res1) | set(res2)
    improved = []
    regressed = []
    same = []

    for k in sorted(all_keys):
        name = k.split(":", 1)[1]
        if k in res1 and k in res2:
            ok1, _ = res1[k]
            ok2, _ = res2[k]
            if ok1 == ok2:
                same.append(name)
            elif ok2 and not ok1:
                improved.append(name)
            else:
                regressed.append(name)
        elif k in res2:
            improved.append(f"{name}（新增）")

    if improved:
        print("⬆️ 提升项:")
        for n in improved:
            print(f"   {n}")
    if regressed:
        print("⬇️ 退化项（回归！）:")
        for n in regressed:
            print(f"   {n}")
    if same:
        print("= 持平项:")
        for n in same:
            print(f"   {n}")

    # 通过率
    total1 = len(res1)
    total2 = len(res2)
    ok1 = sum(1 for v, _ in res1.values() if v)
    ok2 = sum(1 for v, _ in res2.values() if v)
    print("=" * 50)
    print(f"{r1}: {ok1}/{total1} 通过 ({ok1 / max(total1, 1) * 100:.0f}%)")
    print(f"{r2}: {ok2}/{total2} 通过 ({ok2 / max(total2, 1) * 100:.0f}%)")
    if regressed:
        print("⚠️ 有回归——检查退化项！")
    else:
        print("✅ 无回归——能力持平或提升")


if __name__ == "__main__":
    main()

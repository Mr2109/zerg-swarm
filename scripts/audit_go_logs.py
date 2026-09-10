#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Go 日志/横幅中文字面量审计（多语言 L4——决策③「日志=英文」，分批推进的进度尺）。

用法：
  python3 scripts/audit_go_logs.py           # 汇总（总数 + 优先批 + 文件 Top）
  python3 scripts/audit_go_logs.py --list    # 逐条列出（文件:行 + 内容）
  python3 scripts/audit_go_logs.py --batch 启动   # 只看某优先批

优先批（设计稿 §4.4 顺序）：启动 / 认证 / 模型加载 / 任务生命周期 / 熔断 —— 优先英文化；
纯内部诊断可晚改；**注释保持中文**（不在此脚本口径内）。
"""
import argparse
import collections
import os
import re

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORE = os.path.join(ROOT, "core")
CJK = re.compile(r'[\u4e00-\u9fff]')
LOG = re.compile(r'(log\.Printf|log\.Println|log\.Print|fmt\.Printf|fmt\.Println|fmt\.Print|'
                 r'fmt\.Fprintf|fmt\.Fprintln|println\(|eprintln\(|slog\.|Fatalf\()')

# 优先批关键词 → 批名
BATCHES = [
    ("启动", ["启动", "监听", "横幅", "初始化", "就绪"]),
    ("认证", ["令牌", "认证", "auth", "token", "AUTH"]),
    ("模型加载", ["模型", "model", "llama", "适配器", "adapter"]),
    ("任务生命周期", ["任务", "task", "派单", "worktree", "调度"]),
    ("熔断", ["熔断", "冷却", "失败"]),
]


def batch_of(line):
    for name, kws in BATCHES:
        for kw in kws:
            if kw in line:
                return name
    return "其他/内部诊断"


def scan():
    rows = []
    for root, dirs, names in os.walk(CORE):
        dirs[:] = [d for d in dirs if d not in ("vendor", "testdata")]
        for n in sorted(names):
            if not n.endswith(".go") or n.endswith("_test.go"):
                continue
            path = os.path.join(root, n)
            rel = os.path.relpath(path, ROOT)
            for i, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
                s = line.strip()
                if s.startswith("//") or not CJK.search(s) or not LOG.search(s):
                    continue
                rows.append((rel, i, batch_of(s), s[:150]))
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--list", action="store_true", help="逐条列出")
    ap.add_argument("--batch", help="只看某优先批")
    args = ap.parse_args()

    rows = scan()
    by_batch = collections.Counter(r[2] for r in rows)
    by_file = collections.Counter(r[0] for r in rows)

    print("=== Go 含中文的日志/横幅语句：%d 条 ===" % len(rows))
    print("\n按优先批（设计稿 §4.4 顺序）：")
    for name, _ in BATCHES + [("其他/内部诊断", [])]:
        print("  %-14s %d" % (name, by_batch.get(name, 0)))
    print("\n文件 Top 15：")
    for f, n in by_file.most_common(15):
        print("  %-52s %d" % (f, n))

    if args.list or args.batch:
        sel = [r for r in rows if not args.batch or r[2] == args.batch]
        print("\n=== 明细（%d 条）===" % len(sel))
        for rel, i, b, s in sel:
            print("  [%s] %s:%d %s" % (b, rel, i, s))


if __name__ == "__main__":
    main()

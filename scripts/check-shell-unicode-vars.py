#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""扫描 shell 脚本里 `$VAR` 后紧跟非 ASCII 字节的隐患（bash 会把多字节当变量名一部分 → unbound variable）。

规则：把 $VAR 改成 ${VAR}。只处理本类风险，不改动其它内容；逐文件报告改动数。
"""
import re
import sys

# 用法：
#   python3 scripts/check-shell-unicode-vars.py --check scripts/*.sh   # 只报，命中即 exit 1（门禁用）
#   python3 scripts/check-shell-unicode-vars.py scripts/*.sh           # 就地修（$VAR → ${VAR}）
CHECK = "--check" in sys.argv[1:]
FILES = [a for a in sys.argv[1:] if a != "--check"]

# $VAR（不含 ${...} 形式、不含 $1 等位置参数）后面紧跟一个 >=0x80 的字节
PAT = re.compile(rb"\$([A-Za-z_][A-Za-z0-9_]*)(?=[\x80-\xff])")

total = 0
for path in FILES:
    try:
        raw = open(path, "rb").read()
    except FileNotFoundError:
        continue
    hits = PAT.findall(raw)
    if not hits:
        continue
    if not CHECK:
        new = PAT.sub(lambda m: b"${" + m.group(1) + b"}", raw)
        open(path, "wb").write(new)
    total += len(hits)
    print("  %-34s 修 %d 处: %s" % (path.split("/")[-1], len(hits),
                                     ", ".join(sorted({h.decode() for h in hits}))))
print(("命中" if CHECK else "合计修复") + " %d 处" % total)
sys.exit(1 if (CHECK and total) else 0)

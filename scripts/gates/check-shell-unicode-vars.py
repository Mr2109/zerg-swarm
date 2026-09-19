#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""扫描 shell 脚本里 `$VAR` 后紧跟非 ASCII 字节的隐患（bash 会把多字节当变量名一部分 → unbound variable）。

规则：把 $VAR 改成 ${VAR}。只处理本类风险，不改动其它内容；逐文件报告改动数。整行注释（首个非空白字符为 #）一律跳过（注释里的示例不是真隐患）。
"""
import re
import sys

# 用法：
#   python3 scripts/gates/check-shell-unicode-vars.py --check scripts/*.sh   # 只报，命中即 exit 1（门禁用）
#   python3 scripts/gates/check-shell-unicode-vars.py scripts/*.sh           # 就地修（$VAR → ${VAR}）
CHECK = "--check" in sys.argv[1:]
FILES = [a for a in sys.argv[1:] if a != "--check"]
if not FILES:
    sys.stderr.write("✗ 必须显式传入要扫描的脚本（例如 scripts/*.sh）。\n"
                     "  无参数时不静默空转——这正是本工具此前的漏报根因（$VAR后接全角标点未被发现）。\n"
                     "  用法: python3 scripts/gates/check-shell-unicode-vars.py [--check] scripts/*.sh publish/*.sh\n")
    sys.exit(2)

# $VAR（不含 ${...} 形式、不含 $1 等位置参数）后面紧跟一个 >=0x80 的字节
PAT = re.compile(rb"\$([A-Za-z_][A-Za-z0-9_]*)(?=[\x80-\xff])")

total = 0
for path in FILES:
    try:
        raw = open(path, "rb").read()
    except FileNotFoundError:
        continue
    def _mask_comments(b):
        """把整行注释（首个非空白字符为 #）的字节换成空格，保留换行与偏移。"""
        keep = bytearray(b); start = 0
        for ln in b.split(b"\n"):
            if ln.lstrip().startswith(b"#"):
                for i in range(start, start + len(ln)):
                    if keep[i] != 0x0a:
                        keep[i] = 0x20
            start += len(ln) + 1
        return bytes(keep)
    masked = _mask_comments(raw)
    hits = PAT.findall(masked)
    if not hits:
        continue
    if not CHECK:
        out = bytearray(raw)
        for m in PAT.finditer(masked):
            out[m.start():m.end()] = b"${" + m.group(1) + b"}"
        new = bytes(out)
        open(path, "wb").write(new)
    total += len(hits)
    print("  %-34s 修 %d 处: %s" % (path.split("/")[-1], len(hits),
                                     ", ".join(sorted({h.decode() for h in hits}))))
print(("命中" if CHECK else "合计修复") + " %d 处" % total)
sys.exit(1 if (CHECK and total) else 0)

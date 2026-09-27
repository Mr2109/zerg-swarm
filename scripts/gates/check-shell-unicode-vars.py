#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""扫描 shell 脚本里 `$VAR` 后紧跟非 ASCII 字节的隐患（bash 会把多字节当变量名一部分 → unbound variable）。

规则：把 $VAR 改成 ${VAR}。只处理本类风险，不改动其它内容；逐文件报告改动数。整行注释（首个非空白字符为 #）一律跳过（注释里的示例不是真隐患）。
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile

# 用法：
#   python3 scripts/gates/check-shell-unicode-vars.py --check scripts/*.sh   # 只报，命中即 exit 1（门禁用）
#   python3 scripts/gates/check-shell-unicode-vars.py scripts/*.sh           # 就地修（$VAR → ${VAR}）
# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


rc_q176 = refuse_json_without_machine_face()
if rc_q176 is not None:
    sys.exit(rc_q176)

# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────
# ── `Q-176` 第六批：`--self-test` 三档（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
#    纪律：合成件**全在临时目录**（`tempfile.mkdtemp` + `finally` 清理）⇒ 不读不写仓内件 ·
#          子进程**真跑本件**取真退出码（不猜）；写 stderr 一律 `print(…, file=sys.stderr)`（不用 `sys.stderr.write`）。
SELF_PATH = os.path.abspath(__file__)


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """三档：`--check` 只报不写 ⇒ 夹具全在临时目录（正控 0 · 负控 1 · 用法错 2）。"""
    bad = [0]
    print("── check-shell-unicode-vars.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
    tmp = tempfile.mkdtemp(prefix="shellunicode-selftest-")
    try:
        clean = os.path.join(tmp, "clean.sh")
        with open(clean, "w", encoding="utf-8") as f:
            f.write('echo "${VAR}中文"\n')
        dirty = os.path.join(tmp, "dirty.sh")
        with open(dirty, "w", encoding="utf-8") as f:
            f.write('echo "$VAR中文"\n')

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH, "--check"] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：`${VAR}` 花括号写法 ⇒ rc=0", 0, *run(clean))
        _st_chk(bad, "档② 负控：`$VAR` 后紧跟非 ASCII ⇒ rc=1", 1, *run(dirty))
        _st_chk(bad, "档③ 用法错：不给文件（不静默空转）⇒ rc=2", 2, *run())
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
    sys.exit(self_test())

CHECK = "--check" in sys.argv[1:]
FILES = [a for a in sys.argv[1:] if a != "--check"]
if not FILES:
    print("✗ 必须显式传入要扫描的脚本（例如 scripts/*.sh）。\n"
                     "  无参数时不静默空转——这正是本工具此前的漏报根因（$VAR后接全角标点未被发现）。\n"
                     "  用法: python3 scripts/gates/check-shell-unicode-vars.py [--check] scripts/*.sh publish/*.sh", file=sys.stderr)
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

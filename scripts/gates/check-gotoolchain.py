#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Go 工具链钉住一致性门禁（G6，2026-09-13｜B5）

为什么需要：源码式更新 = **每台机器自己编**。若两台机器的 go 版本不同，同一个 commit
会编出不同的二进制（内联/优化/标准库都在变），「同 commit + 同平台 ⇒ 同 sha256」这条
断言就静默失效。故工具链版本必须有**单一真源**，且 core 与 agent 两个模块同值。

⚠️ 一个实测坑（2026-09-13 踩到）：`toolchain` 指令若与同文件的 `go` 指令**同值**，
go 命令会把它当冗余**直接删掉**（`go build` 期间改 go.mod）⇒ 那样"钉住"文件里根本留不住。
故本仓口径：**pin = core/go.mod 的 `go` 指令**（go 永不删它）；agent 侧用显式的
`toolchain`（值 ≠ 其 go 指令，能留下）把该模块也钉到同一份工具链上。

断言（任一不满足即 rc=1）：
  1. core/go.mod 有可解析的 `go` 指令（= pin）
  2. agent/go.mod 的「自身要求」= toolchain（若有）否则 go 指令 —— 必须 <= pin
  3. 若两模块都写了 `toolchain`，两者必须同值；且任何 toolchain 必须 >= 模块自己的 go 指令

用法：python3 scripts/gates/check-gotoolchain.py [仓库根]
退出码：0 通过 / 1 失败
"""
import os
import re
import sys

TOOLCHAIN = re.compile(r"^toolchain\s+(go[\d.]+)\s*$", re.M)
GO_DIRECTIVE = re.compile(r"^go\s+([\d.]+)\s*$", re.M)


def repo_root(arg=None):
    if arg:
        return os.path.abspath(arg)
    d = os.path.abspath(os.path.dirname(__file__))
    for _ in range(6):
        if os.path.isdir(os.path.join(d, "core")) and os.path.isdir(os.path.join(d, "agent")):
            return d
        d = os.path.dirname(d)
    return os.getcwd()


def trip(v):
    m = re.match(r"^go?(\d+)\.(\d+)(?:\.(\d+))?$", v or "")
    if not m:
        return None
    return tuple(int(m.group(i)) if m.group(i) else 0 for i in range(1, 4))


def parse(path):
    txt = open(path, encoding="utf-8").read()
    tc = TOOLCHAIN.search(txt)
    gd = GO_DIRECTIVE.search(txt)
    return (tc.group(1) if tc else None), ("go" + gd.group(1) if gd else None)


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


# ── `Q-176` 第五批：`--self-test` 三档自检（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
#    纪律：**只在临时目录里造件**（不读不写仓内件）· 子进程**真跑本件**取真退出码 · 退出即清。
USAGE = "用法：python3 scripts/gates/check-gotoolchain.py [仓库根] [--self-test]"
SELF_PATH = os.path.abspath(__file__)


def self_test():
    """三档自检：两模块同值 ⇒ rc=0；分叉/超 pin ⇒ rc=1；未知旗标 ⇒ rc=2。"""
    import shutil
    import subprocess
    import tempfile
    tmp = tempfile.mkdtemp(prefix="gotoolchain-selftest-")
    bad = [0]

    def chk(label, want, rc, out):
        good = (rc == want)
        if not good:
            bad[0] += 1
        tail = "" if good else " ｜ " + (out.strip().splitlines() or [""])[-1][:90]
        print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))

    def run(*argv):
        p = subprocess.run([sys.executable, SELF_PATH] + list(argv), capture_output=True, text=True)
        return p.returncode, p.stdout + p.stderr

    def fixture(name, core, agent):
        d = os.path.join(tmp, name)
        os.makedirs(os.path.join(d, "core"))
        os.makedirs(os.path.join(d, "agent"))
        for mod, body in (("core", core), ("agent", agent)):
            with open(os.path.join(d, mod, "go.mod"), "w", encoding="utf-8") as f:
                f.write(body)
        return d

    try:
        print("── check-gotoolchain.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
        clean = fixture("clean", "module core\n\ngo 1.22.0\n", "module agent\n\ngo 1.22.0\n")
        rc, out = run(clean)
        chk("档① 正控：两模块同值（core 的 go 指令为 pin）", 0, rc, out)

        higher = fixture("higher", "module core\n\ngo 1.22.0\n",
                         "module agent\n\ngo 1.22.0\n\ntoolchain go1.23.0\n")
        rc, out = run(higher)
        chk("档② 负控：agent 要求 go1.23.0 高于 pin go1.22.0", 1, rc, out)

        diverge = fixture("diverge", "module core\n\ngo 1.22.0\n\ntoolchain go1.22.0\n",
                          "module agent\n\ngo 1.23.0\n\ntoolchain go1.23.0\n")
        rc, out = run(diverge)
        chk("档②′ 成对负控：两模块 toolchain 不同值（单一真源被破坏）", 1, rc, out)

        rc, out = run(clean, "--no-such-flag")
        chk("档③ 用法错：未知旗标（机器面缺/输入错 ≠ 零命中）", 2, rc, out)

        rc, out = run("--json")
        chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    argv = sys.argv[1:]
    if "--self-test" in argv:
        return self_test()
    unknown = [a for a in argv if a.startswith("-")]
    if unknown:
        print("✗ 用法错（rc=2）：未知旗标 %s" % " ".join(unknown), file=sys.stderr)
        print("  %s" % USAGE, file=sys.stderr)
        return 2
    root = repo_root(argv[0] if argv else None)
    print("仓库根: %s" % root)
    mods = {}
    fails = []
    for name in ("core", "agent"):
        p = os.path.join(root, name, "go.mod")
        if not os.path.exists(p):
            fails.append("%s/go.mod 缺失" % name)
            continue
        pin, godir = parse(p)
        mods[name] = {"toolchain": pin, "go": godir}
        print("  %-6s go=%-10s toolchain=%s" % (name, godir or "（缺）", pin or "（无）"))
        if not godir:
            fails.append("%s/go.mod 没有可解析的 go 指令" % name)
        if pin and godir and trip(pin) and trip(godir) and trip(pin) < trip(godir):
            fails.append("%s：toolchain %s 低于自身 go 指令 %s" % (name, pin, godir))
    if fails or "core" not in mods or "agent" not in mods:
        for f in fails:
            print("  - %s" % f)
        print("\n结果: 失败")
        return 1

    # pin = core 的 toolchain（若有）否则 core 的 go 指令
    pin = mods["core"]["toolchain"] or mods["core"]["go"]
    own = mods["agent"]["toolchain"] or mods["agent"]["go"]
    tcs = [m["toolchain"] for m in mods.values() if m["toolchain"]]
    if len(set(tcs)) > 1:
        fails.append("两模块 toolchain 不同值：%s —— 单一真源被破坏" % tcs)
    if trip(own) and trip(pin) and trip(own) > trip(pin):
        fails.append("agent 要求 %s 高于 pin %s —— 两份工具链会分叉" % (own, pin))
    if fails:
        print("\n结果: 失败")
        for f in fails:
            print("  - %s" % f)
        return 1
    print("\n结果: 通过——工具链 pin = %s（core 的 go 指令为单一真源；agent 钉到 %s）" % (pin, own))
    return 0


if __name__ == "__main__":
    sys.exit(main())

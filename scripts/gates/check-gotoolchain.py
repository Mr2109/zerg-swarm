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


def main():
    root = repo_root(sys.argv[1] if len(sys.argv) > 1 else None)
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

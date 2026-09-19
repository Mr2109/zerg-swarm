#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
版本号一致性门禁（APP-A23 单一来源，2026-09-11）

断言两处「版本真源」一致：
  1. ui/Cargo.toml        version = "X.Y.Z"     （UI 侧：标题/底栏/模块箱由 env!(CARGO_PKG_VERSION) 编译期取值）
  2. core/internal/version/version.go  const Version = "X.Y.Z"  （Go 侧：横幅 / capabilities / openapi）

用法：python3 scripts/gates/check_version.py [仓库根]
退出码：0 一致 / 1 不一致或解析失败
"""
import os
import re
import sys


def repo_root(arg=None):
    if arg:
        return os.path.abspath(arg)
    d = os.path.abspath(os.path.dirname(__file__))
    for _ in range(6):
        if os.path.isdir(os.path.join(d, "core")) and os.path.isdir(os.path.join(d, "ui")):
            return d
        d = os.path.dirname(d)
    return os.getcwd()


def rust_version(root):
    p = os.path.join(root, "ui", "Cargo.toml")
    m = re.search(r'^version\s*=\s*"([^"]+)"', open(p, encoding="utf-8").read(), re.M)
    return m.group(1) if m else None


def go_version(root):
    p = os.path.join(root, "core", "internal", "version", "version.go")
    if not os.path.exists(p):
        return None
    m = re.search(r'^const Version = "([^"]+)"', open(p, encoding="utf-8").read(), re.M)
    return m.group(1) if m else None


def main():
    root = repo_root(sys.argv[1] if len(sys.argv) > 1 else None)
    r, g = rust_version(root), go_version(root)
    print("仓库根: %s" % root)
    print("  ui/Cargo.toml version      = %s" % r)
    print("  core version.Version       = %s" % g)
    if not r or not g:
        print("\n结果: 失败——未能解析版本号（Cargo.toml 或 version.go 缺失/格式变化）")
        return 1
    if r != g:
        print("\n结果: 失败——版本号不一致（收版时两处必须同改）")
        return 1
    print("\n结果: 通过——版本号一致 %s" % r)
    return 0


if __name__ == "__main__":
    sys.exit(main())

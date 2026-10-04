#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
版本号一致性门禁（APP-A23 单一来源，2026-09-11）

断言两处「版本真源」一致：
  1. ui/Cargo.toml        version = "X.Y.Z"     （UI 侧：标题/底栏/模块箱由 env!(CARGO_PKG_VERSION) 编译期取值）
  2. core/internal/version/version.go  const Version = "X.Y.Z"  （Go 侧：横幅 / capabilities / openapi）

用法：python3 scripts/gates/check_version.py [仓库根]
退出码：0 一致 / 1 不一致 / 2 **不给结论**（版本真源取不到 = 空域）或用法错
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile


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
    if not os.path.exists(p):   # ★ 空集守卫（GAP-20260927-108）：件不存在 ⇒ 取不到（返 None），不抛 traceback
        return None
    m = re.search(r'^version\s*=\s*"([^"]+)"', open(p, encoding="utf-8").read(), re.M)
    return m.group(1) if m else None


def go_version(root):
    p = os.path.join(root, "core", "internal", "version", "version.go")
    if not os.path.exists(p):
        return None
    m = re.search(r'^const Version = "([^"]+)"', open(p, encoding="utf-8").read(), re.M)
    return m.group(1) if m else None


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
    """三档：合成仓全在临时目录（正控 0 · 负控 1 · 用法错 2）。"""
    bad = [0]
    print("── check_version.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
    tmp = tempfile.mkdtemp(prefix="version-selftest-")
    try:
        def mk(name, cargo_v, go_v):
            root = os.path.join(tmp, name)
            os.makedirs(os.path.join(root, "ui"))
            os.makedirs(os.path.join(root, "core", "internal", "version"))
            with open(os.path.join(root, "ui", "Cargo.toml"), "w", encoding="utf-8") as f:
                f.write("[package]\nversion = \"%s\"\n" % cargo_v)
            with open(os.path.join(root, "core", "internal", "version", "version.go"),
                      "w", encoding="utf-8") as f:
                f.write("package version\n\nconst Version = \"%s\"\n" % go_v)
            return root

        same = mk("same", "1.2.3", "1.2.3")
        diff = mk("diff", "1.2.3", "9.9.9")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：两侧版本号一致 ⇒ rc=0", 0, *run(same))
        _st_chk(bad, "档② 负控：两侧版本号不一致 ⇒ rc=1", 1, *run(diff))
        _st_chk(bad, "档③ 用法错：不认的旗标 ⇒ rc=2（不静默当初仓库根）", 2, *run("--no-such-flag"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main():
    argv = sys.argv[1:]
    if "--self-test" in argv or "--selftest" in argv:
        return self_test()
    unknown = [a for a in argv if a.startswith("-")]
    if unknown:
        print("用法错：本件不认的旗标 %s ⇒ rc=2（**不许**静默当成仓根）" % " ".join(unknown), file=sys.stderr)
        print("用法：python3 scripts/gates/check_version.py [仓库根]", file=sys.stderr)
        return 2
    root = repo_root(argv[0] if argv else None)
    r, g = rust_version(root), go_version(root)
    print("仓库根: %s" % root)
    print("  ui/Cargo.toml version      = %s" % r)
    print("  core version.Version       = %s" % g)
    # ★ 空集守卫（GAP-20260927-108）：域 = 「取到的版本真源」——
    #   任一侧取不到（件缺失 / 无 version 行 / 解析不出 ⇒ None）时**比较整块消失** ⇒ 不许当「一致」绿；
    #   同族口径：**不存在 / 取不到 ⇒ 不给结论（rc=2）**，与「两处不一致」的真红（rc=1）分得开。
    if r is None or g is None:  # ★ 空集守卫（GAP-20260927-108）：版本真源取不到（空域）⇒ rc=2（不给结论）
        print("\n结果: 不给结论（rc=2）—— 版本真源取不到（ui/Cargo.toml 或 core version.go 缺失/无 version 行）")
        return 2
    if r != g:
        print("\n结果: 失败——版本号不一致（收版时两处必须同改）")
        return 1
    print("\n结果: 通过——版本号一致 %s" % r)
    return 0


if __name__ == "__main__":
    sys.exit(main())

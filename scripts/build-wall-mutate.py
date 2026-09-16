#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""`scripts/negctl-build-wall.sh` 的**变异验证**（批 2'.6）—— 证「这份负例控制真的会红」。

为什么必须做：负例控制若不红，唯一的表现就是**一路全绿**（和「实现正确」长得一模一样）。
所以把被测实现（`scripts/build-wall.sh`）**真的改坏**，断言负例控制**必须红在那一条控制项上**，
再还原、断言 sha256 逐字节一致、断言复跑绿。

三条变异各自打在**一条**控制项上（换着红，才说明不是「一切都红」）：

| 变异 | 改坏的实现 | 必须红在 |
|---|---|---|
| M1 | 无 cargo 时改成**以 0 退出**（「跳过却像绿」——正是本脚本要防的形态） | N2 |
| M2 | 仓不完整（无 `wall/`）时改成以 0 退出 | N1 |
| M3 | 认不得的参数**静默忽略**（改成以 0 退出） | N3 |

判据只看**真实退出码**与**哪一条控制项红**（不按输出里的字样判定）；变异体不改语法（`bash -n` 先过），
避免把「语法坏了」误当「判据红了」。还原写在 `finally` 里 —— 变异脚本把工作树改坏不还原是这类脚本
最贵的失败方式。

用法：`python3 scripts/build-wall-mutate.py`
退出码：0 三条变异都按期望红且还原后复跑绿 · 1 有变异不符期望 · 2 硬失败（环境/前置件/锚点命中数 != 1）
"""

import hashlib
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TARGET = os.path.join(ROOT, "scripts", "build-wall.sh")
NEGCTL = os.path.join(ROOT, "scripts", "negctl-build-wall.sh")

EXIT_OK, EXIT_MISMATCH, EXIT_HARD = 0, 1, 2

# (名称, 锚点原文, 替换成, 必须红在那条控制项)
MUTATIONS = [
    (
        "M1 无 cargo 时以 0「跳过」",
        'echo "❌ 本机没有 cargo（rust 工具链）⇒ 编不出茧壁；构建段见 wall/README.md" >&2\n  exit 2\n',
        'echo "❌ 本机没有 cargo（rust 工具链）⇒ 编不出茧壁；构建段见 wall/README.md" >&2\n  exit 0\n',
        "N2",
    ),
    (
        "M2 仓不完整（无 wall/）时以 0 退出",
        'echo "❌ 茧壁源码不在 ${WALL_DIR}/Cargo.toml ⇒ 仓不完整（wall/ 是仓内一等公民，不走跳过这条路）" >&2\n  exit 2\n',
        'echo "❌ 茧壁源码不在 ${WALL_DIR}/Cargo.toml ⇒ 仓不完整（wall/ 是仓内一等公民，不走跳过这条路）" >&2\n  exit 0\n',
        "N1",
    ),
    (
        "M3 认不得的参数静默忽略",
        'echo "❌ 未知参数: ${arg}（本脚本只认 --debug / --help，不静默忽略）" >&2\n      exit 2 ;;',
        'echo "❌ 未知参数: ${arg}（本脚本只认 --debug / --help，不静默忽略）" >&2\n      exit 0 ;;',
        "N3",
    ),
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def run_negctl():
    p = subprocess.run(["bash", NEGCTL], cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode, p.stdout.decode("utf-8", "replace")


def main():
    for path in (TARGET, NEGCTL):
        if not os.path.exists(path):
            print("!! 前置件不在: %s" % path)
            return EXIT_HARD
    if not os.access(os.path.join(ROOT, "wall", "target", "debug", "zerg-wall"), os.X_OK):
        print("!! 前置件不在: wall/target/debug/zerg-wall（先 bash scripts/build-wall.sh --debug）")
        return EXIT_HARD

    original = open(TARGET, "rb").read()
    original_sha = sha256(TARGET)
    print("=== 起点：负例控制必须先绿（否则「红」没有意义）===")
    rc, out = run_negctl()
    if rc != EXIT_OK:
        print(out)
        print("!! 起点不绿（rc=%d）⇒ 不进入变异，先修控制本身" % rc)
        return EXIT_HARD
    print("  PASS 起点绿（rc=0）")
    print("  sha256(scripts/build-wall.sh) = %s" % original_sha[:16])

    bad = 0
    try:
        for name, anchor, replacement, expect_red in MUTATIONS:
            print()
            print("=== 变异：%s（必须红在 %s）===" % (name, expect_red))
            text = open(TARGET, "r", encoding="utf-8").read()
            hits = text.count(anchor)
            if hits != 1:
                print("!! 锚点命中 %d 次（必须恰好 1）⇒ 硬失败，本变异作废" % hits)
                return EXIT_HARD
            open(TARGET, "w", encoding="utf-8").write(text.replace(anchor, replacement, 1))

            # 变异体先过语法，避免把「语法坏了」误当「判据红了」
            p = subprocess.run(["bash", "-n", TARGET], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            if p.returncode != 0:
                print("!! 变异体语法不过 ⇒ 本变异作废（坏的应是行为，不是语法）")
                return EXIT_HARD

            rc, out = run_negctl()
            red_line = "✗ 红 %s" % expect_red
            has_red = red_line in out
            print("  负例控制 rc=%d；期望红的那条在输出里：%s" % (rc, "有" if has_red else "没有"))
            for line in out.splitlines():
                if line.strip().startswith("✗ 红"):
                    print("   " + line.strip())
            if rc == EXIT_MISMATCH and has_red:
                print("  ✓ 按期望红（判据红，不是 traceback）")
            else:
                print("  ✗ 不符期望：rc=%d（期望 %d）· 期望红在 %s" % (rc, EXIT_MISMATCH, expect_red))
                bad += 1
            open(TARGET, "wb").write(original)
    finally:
        # 还原写在 finally：脚本被 Ctrl-C / 被杀也把源码放回去
        open(TARGET, "wb").write(original)

    now_sha = sha256(TARGET)
    print()
    print("=== 还原复核 ===")
    if now_sha != original_sha:
        print("!! 还原后 sha 不一致：%s != %s" % (now_sha[:16], original_sha[:16]))
        return EXIT_HARD
    print("  PASS 还原后 sha256 逐字节一致（%s）" % now_sha[:16])

    rc, out = run_negctl()
    print("  复跑负例控制 rc=%d" % rc)
    if rc != EXIT_OK:
        print(out)
        print("!! 还原后复跑不绿 ⇒ 工作树没回到原状")
        return EXIT_HARD
    print("  PASS 复跑绿")

    print()
    if bad:
        print("!! %d 条变异不符期望 —— 负例控制的区分度不足" % bad)
        return EXIT_MISMATCH
    print("✅ 三条变异各自红在预期的控制项上 ⇒ 还原一致 ⇒ 复跑绿")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

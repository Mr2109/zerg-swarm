#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""迁移桥的**变异验证**（规程 §2.2：新增门禁必须能失败；探针与证据落进仓，不留 /tmp）。

做法：把**两侧各改坏一次**（Rust 侧 / Go 侧），每一步都断言「门禁必须红」，然后还原、断言
「sha 逐字节一致」再断言「复跑必须绿」。任何一步不符合预期即整体 rc=1 —— 这个脚本本身就是
「桥能不能红」的判据。

    python3 scripts/wall-bridge-mutate.py            # 两次变异都跑
    python3 scripts/wall-bridge-mutate.py --only M1  # 只跑一次（调试用）

退出码：`0` 两次变异都按预期（红 ⇒ 还原 ⇒ 绿）· `1` 有一步不符合预期 · `2` 硬失败（备份/构建/脚本本身跑不起来）。

**还原是 finally 里做的**：脚本被 Ctrl-C / 被杀也会把源码放回去（并校验 sha）—— 变异脚本把工作树改坏
不还原，是这类脚本最贵的失败方式。
"""

import argparse
import hashlib
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WALL_DIR = os.path.join(ROOT, "wall")
GATE = os.path.join(ROOT, "scripts", "compare-wall-argv.py")

EXIT_OK, EXIT_UNEXPECTED, EXIT_HARD = 0, 1, 2


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def cargo():
    p = shutil.which("cargo")
    if p:
        return p
    alt = os.path.join(os.path.expanduser("~"), ".cargo", "bin", "cargo")
    return alt if os.path.exists(alt) else ""


def build_wall():
    c = cargo()
    if not c:
        return 2, "找不到 cargo（PATH 与 ~/.cargo/bin 都没有）"
    p = subprocess.run([c, "build", "--manifest-path", os.path.join(WALL_DIR, "Cargo.toml")],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if p.returncode != 0:
        return p.returncode, p.stdout.decode("utf-8", "replace")[-1500:]
    return 0, ""


def run_gate():
    """跑桥门禁，返回 (rc, 摘要)。摘要挑出「红/绿/对拍」那几行，便于回执引用。"""
    p = subprocess.run([sys.executable, GATE], cwd=ROOT,
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    out = p.stdout.decode("utf-8", "replace")
    keys = ("红 ", "绿 ", "!!", "对拍全绿", "对拍不一致")
    lines = [l.strip() for l in out.splitlines() if l.strip().startswith(keys)]
    return p.returncode, lines


MUTATIONS = [
    {
        "id": "M1",
        "side": "Rust（茧壁）",
        "file": "wall/src/platform/linux.rs",
        "why": "拿掉 `--die-with-parent`（隔离项少一条）",
        "anchor": '    argv.push("--unshare-pid".to_string());\n'
                  '    argv.push("--die-with-parent".to_string());\n',
        "repl": '    argv.push("--unshare-pid".to_string());\n',
        "rebuild": True,
    },
    {
        "id": "M2",
        "side": "Go（孵化器）",
        "file": "agent/internal/hatch/hatch.go",
        "why": "对调 `--proc /proc` 与 `--dev /dev`（顺序漂移）",
        "anchor": '\t\t"--proc", "/proc",\n\t\t"--dev", "/dev",\n',
        "repl": '\t\t"--dev", "/dev",\n\t\t"--proc", "/proc",\n',
        "rebuild": False,  # go test 自己会重编
    },
]


def mutate(path, anchor, repl):
    """一次写盘：锚点必须**恰好命中一次**，否则不动文件（宁可整批中止，也不留半改的树）。"""
    with open(path, "r", encoding="utf-8") as f:
        text = f.read()
    n = text.count(anchor)
    if n != 1:
        return "锚点命中 %d 次（应为 1）—— 未写盘" % n
    with open(path, "w", encoding="utf-8") as f:
        f.write(text.replace(anchor, repl))
    return ""


def main():
    ap = argparse.ArgumentParser(description="迁移桥变异验证：两侧各改坏一次，门禁必须红")
    ap.add_argument("--only", default="", help="只跑某一条（M1 / M2）")
    args = ap.parse_args()

    if not os.path.exists(GATE):
        print("!! 找不到门禁脚本 %s —— 硬失败" % GATE)
        return EXIT_HARD

    print("=== 迁移桥变异验证（%s）===" % os.path.relpath(GATE, ROOT))
    rc, log = build_wall()
    if rc != 0:
        print("!! 基线构建失败（rc=%d）：\n%s" % (rc, log))
        return EXIT_HARD
    rc, lines = run_gate()
    print("基线：门禁 rc=%d · %s" % (rc, lines[-1] if lines else "(无摘要)"))
    if rc != 0:
        print("!! 基线就不是绿的 ⇒ 硬失败（不许在红基线上做变异）")
        return EXIT_HARD

    bad = []
    tbl = []
    for m in MUTATIONS:
        if args.only and m["id"] != args.only:
            continue
        path = os.path.join(ROOT, m["file"])
        pre = sha(path)
        tmpdir = tempfile.mkdtemp(prefix="zerg-mutate-")
        bak = os.path.join(tmpdir, os.path.basename(path))
        shutil.copy(path, bak)
        try:
            err = mutate(path, m["anchor"], m["repl"])
            if err:
                print("!! [%s] %s" % (m["id"], err))
                return EXIT_HARD
            if sha(path) == pre:
                print("!! [%s] 写盘后 sha 未变 —— 变异没生效" % m["id"])
                return EXIT_HARD
            if m["rebuild"]:
                rc, log = build_wall()
                if rc != 0:
                    print("!! [%s] 变异体构建失败：\n%s" % (m["id"], log))
                    return EXIT_HARD
            rc, lines = run_gate()
            red_ok = rc == 1
            print("%s [%s] 变异后：rc=%d（期望 1）· %s" % (
                "✓" if red_ok else "✗", m["id"], rc, lines[-1] if lines else ""))
            if not red_ok:
                bad.append("%s 变异后门禁 rc=%d（期望 1）⇒ 这面镜子抓不住该漂移" % (m["id"], rc))
        finally:
            shutil.copy(bak, path)          # 先回拷
            os.utime(path, None)            # 再顶 mtime（cargo 靠 mtime 判新旧）
            shutil.rmtree(tmpdir, ignore_errors=True)

        post = sha(path)
        restored = post == pre
        print("%s [%s] 还原后 sha %s" % ("✓" if restored else "✗", m["id"],
                                        "逐字节一致" if restored else "不一致！%s ≠ %s" % (post, pre)))
        if not restored:
            bad.append("%s 还原后 sha 不一致：%s ≠ %s" % (m["id"], post, pre))
            return EXIT_HARD
        if m["rebuild"]:
            rc, log = build_wall()
            if rc != 0:
                print("!! [%s] 还原后构建失败：\n%s" % (m["id"], log))
                return EXIT_HARD
        rc, lines = run_gate()
        green_ok = rc == 0
        print("%s [%s] 复跑：rc=%d（期望 0）· %s" % (
            "✓" if green_ok else "✗", m["id"], rc, lines[-1] if lines else ""))
        if not green_ok:
            bad.append("%s 还原后复跑 rc=%d（期望 0）" % (m["id"], rc))
        tbl.append((m["id"], m["side"], m["why"], pre))

    print("")
    if bad:
        for b in bad:
            print("!! " + b)
        return EXIT_UNEXPECTED
    for mid, side, why, pre in tbl:
        print("  %s %s %s · 变异前 sha=%s…" % (mid, side, why, pre[:12]))
    print("变异验证全绿：每条都「红 ⇒ 还原 sha 一致 ⇒ 复跑绿」")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

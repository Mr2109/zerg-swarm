#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""判据 7 门禁的 `--wall` 路线**回执生成器**（批 2'.5）。

它不只是「打印一份输出」，而是**一条能失败的判据**：脚本自己断言四条跑法都按期望返回，并把结果
写进回执（红也照写，写清哪一条红）—— 回执与门禁版本必须对应，否则回执只是好看。

    python3 scripts/two-states-wall-evidence.py

退出码：`0` 全绿 · `1` 断言红 · `2` 硬失败（环境/探针/二进制问题，**不静默跳过**）。

回执落仓 `scripts/sandbox-probes/evidence-two-states-wall.txt`。
**回执里一律用仓内相对路径 + `/private/tmp/…` 靶子路径**（`scripts/` 与 `wall/` 都在发布白名单里
⇒ 回执会进公开面，私有绝对路径不许出现 —— 门禁自身也用 `display_path()` 做了这件事）。

四条跑法（都是真实 rc）：
  ⓪ 制品可复现自检              `CARGO_INCREMENTAL=0` 连编两次 ⇒ 必须逐字节相同（回执里的制品 sha 才不是编号）
  ① 默认路线（行为不许变）        `verify-two-states.py`
  ② 茧壁路线 + 两路对拍           `verify-two-states.py --wall wall/target/debug/zerg-wall`
  ③ 自检（四态成对 + 路线自检）    `verify-two-states.py --self-test --wall <同>`
  ④ 负例活体控制（四条都必须 rc=2） `bash scripts/sandbox-probes/negctl-two-states.sh`
  ⑤ 门禁变异验证（改坏门禁 ⇒ 自检必红）`python3 scripts/two-states-gate-mutate.py`
"""

import hashlib
import os
import shutil
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
P = os.path.join("scripts", "sandbox-probes")
GATE = os.path.join(P, "verify-two-states.py")
WALL_BIN_REL = os.path.join("wall", "target", "debug", "zerg-wall")
WALL_BIN = os.path.join(ROOT, WALL_BIN_REL)
RECEIPT = os.path.join(P, "evidence-two-states-wall.txt")

EXIT_OK, EXIT_RED, EXIT_HARD = 0, 1, 2

results = []   # (名称, ok, 说明)
log = []


def emit(text):
    log.append(text)


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def run(args):
    p = subprocess.run(args, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode, p.stdout.decode("utf-8", "replace")


def rel(p):
    """能相对就相对（回执卫生：仓内绝对路径不进公开面）。"""
    if not p.startswith("/"):
        return p
    try:
        r = os.path.relpath(p, ROOT)
    except ValueError:
        return p
    return r if not r.startswith("..") else p


def build_reproducible():
    """`CARGO_INCREMENTAL=0` 连编两次（中间 `touch` 强制重编）⇒ 必须**逐字节相同**。

    为什么要它：回执里记了制品 sha，而**默认（增量编译）下同一份源码连编两次 sha 并不相同**
    （本机实测：`98bc845e…` / `c11b85ad…`）⇒ 不先把构建钉成可复现的，那个 sha 就只是个编号。
    `touch src/main.rs` 只动 mtime、不动内容（工作树保持干净）。
    """
    cargo = shutil.which("cargo") or os.path.join(os.path.expanduser("~"), ".cargo", "bin", "cargo")
    if not os.path.exists(cargo):
        return [], "找不到 cargo（PATH 与 ~/.cargo/bin 都没有）"
    env = dict(os.environ)
    env["CARGO_INCREMENTAL"] = "0"
    shas = []
    for _ in range(2):
        subprocess.run(["touch", os.path.join(ROOT, "wall", "src", "main.rs")])
        p = subprocess.run([cargo, "build"], cwd=os.path.join(ROOT, "wall"), env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if p.returncode != 0:
            return shas, "cargo build 失败：%s" % p.stdout.decode("utf-8", "replace")[-300:]
        shas.append(sha256(WALL_BIN))
    return shas, ""


def check(name, ok, detail=""):
    results.append((name, bool(ok), detail))
    print("%s %s%s" % ("✓" if ok else "✗", name, ("  —— " + detail) if detail else ""))
    return ok


def main():
    if sys.platform != "darwin":
        print("!! 这不是 macOS —— 本回执只对 macOS 一档有意义（Linux 侧要等真机跑一次）⇒ 硬失败")
        return EXIT_HARD
    if not os.path.isfile(os.path.join(ROOT, GATE)):
        print("!! 门禁不在 %s ⇒ 硬失败" % GATE)
        return EXIT_HARD
    if not os.path.isfile(WALL_BIN):
        print("!! 茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`（本脚本不替你编）⇒ 硬失败"
              % WALL_BIN_REL)
        return EXIT_HARD

    rc, out = run(["uname", "-m"]); arch = out.strip()
    rc_sw, sw = run(["sw_vers", "-productVersion"])
    emit("=== 环境 ===")
    emit("macOS：%s（%s）" % (sw.strip(), arch))
    emit("Python：%s" % sys.version.split()[0])
    emit("茧壁：%s ⇒ %s" % (WALL_BIN_REL, run([WALL_BIN, "--version"])[1].strip()))
    emit("")
    emit("=== ⓪ 制品可复现自检（CARGO_INCREMENTAL=0 连编两次，中间 touch 强制重编）===")
    emit("  为什么要它：回执里记了制品 sha，而**默认（增量编译）下同一份源码连编两次 sha 不同**（本机实测）")
    emit("  ⇒ 先把构建钉成可复现的，回执里那个 sha 才不是一个编号。")
    shas, berr = build_reproducible()
    if not shas:
        emit("!! %s" % berr)
        emit("")
        check("⓪ 制品可复现（CARGO_INCREMENTAL=0 连编两次逐字节相同）", False, berr)
        with open(RECEIPT, "w", encoding="utf-8") as f:
            f.write("\n".join(log) + "\n")
        print("!! 硬失败：%s（回执已落盘，写明卡在哪）" % berr)
        return EXIT_HARD
    emit("  build#1：%s" % shas[0])
    emit("  build#2：%s" % shas[1])
    emit("  两次%s" % ("**逐字节相同**（可复现构建）" if shas[0] == shas[1] else "**不同**（产物不可复现）"))
    emit("")
    check("⓪ 制品可复现（CARGO_INCREMENTAL=0 连编两次逐字节相同）", shas[0] == shas[1],
          shas[1][:16] + "…")
    emit("=== 制品与源码 sha256（哪一版门禁跑出来的）===")
    for f in (GATE, os.path.join(P, "negctl-two-states.sh"), os.path.join(P, "miniprobe.py"),
              os.path.join(P, "conn_jit.py"), os.path.join(P, "lx2-linux.py"),
              os.path.join(P, "pF.sb"), "scripts/two-states-gate-mutate.py",
              "scripts/two-states-wall-evidence.py", WALL_BIN_REL,
              "wall/src/platform/macos.rs", "wall/src/spec.rs"):
        p = os.path.join(ROOT, f)
        emit("  %s  %s" % (sha256(p), f))
    _, head = run(["git", "rev-parse", "HEAD"])
    emit("")
    emit("生成时的 HEAD（本批的父提交；回执与它证据的改动同批提交）：%s" % head.strip())
    emit("判据（可复跑）：python3 scripts/two-states-wall-evidence.py")
    emit("  ① python3 scripts/sandbox-probes/verify-two-states.py")
    emit("  ② python3 scripts/sandbox-probes/verify-two-states.py --wall %s" % WALL_BIN_REL)
    emit("  ③ python3 scripts/sandbox-probes/verify-two-states.py --self-test --wall %s" % WALL_BIN_REL)
    emit("  ④ bash scripts/sandbox-probes/negctl-two-states.sh")
    emit("  ⑤ python3 scripts/two-states-gate-mutate.py")
    emit("")

    runs = [
        ("① 默认路线（行为与历史一致）", ["python3", GATE], 0),
        ("② 茧壁路线 + 两路对拍", ["python3", GATE, "--wall", WALL_BIN], 0),
        ("③ 自检（护栏 + 四态成对 + 路线自检）", ["python3", GATE, "--self-test", "--wall", WALL_BIN], 0),
        ("④ 负例活体控制（四条都必须硬失败）", ["bash", os.path.join(P, "negctl-two-states.sh")], 0),
        ("⑤ 门禁变异验证（三条都必须红 ⇒ 还原 ⇒ 复跑绿）", ["python3", "scripts/two-states-gate-mutate.py"], 0),
    ]

    outs = {}
    for title, argv, expect in runs:
        rc, out = run(argv)
        outs[len(outs)] = out
        emit("=== %s ===" % title)
        emit("$ %s" % " ".join(rel(a) for a in argv))
        for line in out.rstrip("\n").splitlines():
            emit("  " + line)
        emit("rc=%d（期望 %d）" % (rc, expect))
        emit("")
        check(title, rc == expect, "rc=%d" % rc)

    # ④ 的内容断言：四条负例各自都真的打出了「期望 2」（复用的那一跑，不重跑）
    neg = outs[3]
    neg_ok = ["负例① rc=2", "负例② rc=2", "负例③ rc=2", "负例④ rc=2"]
    check("④ 内容断言：四条负例逐条 rc=2（不是「整体不为 0」）",
          all(t in neg for t in neg_ok),
          "、".join(t for t in neg_ok if t not in neg) or "四条齐")

    # ⑤ 的内容断言：三条变异各自红在一个不同的判据上（复用的那一跑）
    mut = outs[4]
    marks = ["[G1] 茧壁路线静默回落", "[G2] 路线起不来时当成", "[G3] 断言 B 不再判"]
    check("⑤ 内容断言：G1/G2/G3 三条变异都被记录（各自红在不同判据）",
          all(m in mut for m in marks), "、".join(m for m in marks if m not in mut) or "三条齐")

    # ② 的内容断言：两路对拍「一致」且茧壁断言两态都 PASS（复用的那一跑）
    wy = outs[1]
    check("② 内容断言：两路对拍=一致 · 茧壁断言 A/B 都 PASS",
          "两路对拍（茧壁 vs 原生黄金配方）" in wy and "两路逐项一致" in wy
          and wy.count("PASS") >= 4,
          "PASS 出现 %d 次" % wy.count("PASS"))

    red = [n for n, ok, _ in results if not ok]
    emit("=== 断言汇总（%d 条，红 %d 条）===" % (len(results), len(red)))
    for name, ok, detail in results:
        emit("  %s %s%s" % ("PASS" if ok else "FAIL", name, ("  —— " + detail) if detail else ""))
    if red:
        emit("")
        emit("!! 红：%s" % "、".join(red))

    with open(RECEIPT, "w", encoding="utf-8") as f:
        f.write("\n".join(log) + "\n")
    print("")
    print("回执：%s（%d 行）" % (RECEIPT, len(log)))
    return EXIT_RED if red else EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

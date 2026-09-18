#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""macOS 一档（Seatbelt **直调**）的活体回执生成器（批 2'.4）。

它不只是「打印一份输出」，而是**一条能失败的判据**：脚本自己断言四态与回执一致，并把断言结果写进
回执文件（红也照写，写清哪一条红）—— 回执与门禁版本必须对应，否则回执只是好看。

    python3 scripts/wall-macos-evidence.py

退出码：`0` 全绿 · `1` 断言红 · `2` 硬失败（环境/探针/二进制问题，**不静默跳过**）。

回执落仓 `wall/evidence-macos-seatbelt.txt`。**回执里一律用仓内相对路径 + `/private/tmp/…`
靶子路径**（`wall/` 在发布白名单里 ⇒ 回执会进公开面，私有绝对路径不许出现 —— 上一份回执在这上面
踩过一次，见 `76a15ddb`）。

靶子：`scripts/sandbox-probes/` 的两支探针（**先复制进靶子目录再跑**：这样配方与回执里都不会出现
仓库的私有绝对路径）。每条断言都有**基线对照**：基线不可达/本来就写得进去 ⇒ 没有区分度 ⇒ rc=2。
"""

import hashlib
import json
import os
import shutil
import subprocess
import sys
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WALL = os.path.join(ROOT, "wall")
BIN = os.path.join(WALL, "target", "debug", "zerg-wall")
PROBES = os.path.join(ROOT, "scripts", "sandbox-probes")
RECEIPT = os.path.join(WALL, "evidence-macos-seatbelt.txt")
PY = "/usr/bin/python3"

EXIT_OK, EXIT_RED, EXIT_HARD = 0, 1, 2

results = []   # (名称, ok, 说明)


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def run(args, cwd=None, env=None):
    e = dict(os.environ)
    if env:
        e.update(env)
    p = subprocess.run(args, cwd=cwd or ROOT, stdout=subprocess.PIPE,
                       stderr=subprocess.PIPE, env=e)
    return (p.returncode,
            p.stdout.decode("utf-8", "replace"),
            p.stderr.decode("utf-8", "replace"))


def check(name, ok, detail=""):
    results.append((name, bool(ok), detail))
    print("%s %s%s" % ("✓" if ok else "✗", name, ("  —— " + detail) if detail else ""))
    return ok


def rel(path):
    """能相对就相对（回执卫生：私有绝对路径不进证据）。"""
    try:
        r = os.path.relpath(path, ROOT)
    except ValueError:
        return path
    return r if not r.startswith("..") else path


def make_space(tag):
    """建靶子目录；探针**复制进去**跑（配方里就不会出现仓的绝对路径）。"""
    space = "/private/tmp/zerg-wall-evidence-%s-%d" % (tag, int(time.time()))
    shutil.rmtree(space, ignore_errors=True)
    for d in ("work", "weights", "probe"):
        os.makedirs(os.path.join(space, d))
    with open(os.path.join(space, "weights", "model.gguf"), "wb") as f:
        f.write(b"not-a-real-weight")
    for name in ("miniprobe.py", "writeout-probe.py"):
        shutil.copy(os.path.join(PROBES, name), os.path.join(space, "probe", name))
    return space


def write_spec(space, probe_name, env_pairs, name):
    spec = {
        "egg_id": "wall-seatbelt-evidence",
        "schema_version": 1,
        "engine_path_in_space": PY,
        "engine_args": [os.path.join(space, "probe", probe_name)],
        "weight_path": os.path.join(space, "weights"),
        "weight_files": [os.path.join(space, "weights", "model.gguf")],
        "env": dict(env_pairs),
        "extra_ro_binds": ["%s:/models/model.gguf" % os.path.join(space, "weights", "model.gguf")],
        "extra_rw_binds": [],
        "work_dir": os.path.join(space, "work"),
    }
    path = os.path.join(space, name)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(spec, f, ensure_ascii=False, indent=2)
        f.write("\n")
    return path


def main():
    if sys.platform != "darwin":
        print("!! 这不是 macOS —— 本回执只对 macOS 一档有意义（Linux 侧走 verify-two-states.py）⇒ 硬失败")
        return EXIT_HARD
    if not os.path.isfile(PY):
        print("!! %s 不在 ⇒ 环境问题 ⇒ 硬失败（不静默跳过）" % PY)
        return EXIT_HARD
    for n in ("miniprobe.py", "writeout-probe.py"):
        if not os.path.isfile(os.path.join(PROBES, n)):
            print("!! 探针缺失 scripts/sandbox-probes/%s ⇒ 硬失败" % n)
            return EXIT_HARD
    if not os.path.isfile(BIN):
        print("!! 茧壁二进制不在 %s ⇒ 先 `cargo build`（本脚本不替你编）⇒ 硬失败" % rel(BIN))
        return EXIT_HARD

    blocks = []          # 回执正文的分节
    log = []

    def emit(text):
        log.append(text)

    # ── 环境与版本（回执自证哪一版跑出来的）──
    _, uname, _ = run(["uname", "-m"])
    _, swvers, _ = run(["sw_vers", "-productVersion"])
    rc, ver, _ = run([BIN, "--version"])
    emit("=== 环境 ===")
    emit("macOS：%s（%s）" % (swvers.strip(), uname.strip()))
    emit("茧壁：%s（rc=%d）" % (ver.strip(), rc))
    emit("茧壁制品 sha256（%s）：%s" % (rel(BIN), sha256(BIN)))
    for f in ("wall/src/platform/macos.rs", "wall/src/platform/mod.rs", "wall/src/platform/linux.rs",
              "wall/src/shim.rs", "wall/src/lib.rs", "wall/src/main.rs",
              "wall/tests/plan_macos.rs", "wall/tests/macos_seatbelt.rs",
              "scripts/wall-macos-evidence.py", "scripts/wall-macos-mutate.py",
              "scripts/sandbox-probes/writeout-probe.py"):
        emit("源码 sha256（%s）：%s" % (f, sha256(os.path.join(ROOT, f))))
    rc, head, _ = run(["git", "rev-parse", "HEAD"])
    emit("生成时的 HEAD（本批的父提交；回执与它证据的改动同批提交）：%s" % head.strip())
    emit("判据（可复跑）：python3 scripts/wall-macos-evidence.py"
         " · 变异验证：python3 scripts/wall-macos-mutate.py"
         " · 纯函数用例：cd wall && cargo test")
    emit("")

    egress_host = os.environ.get("PROBE_EGRESS_HOST", "1.1.1.1")
    egress_port = os.environ.get("PROBE_EGRESS_PORT", "443")

    # ── ① 四态：基线 vs 封闭态 ──
    space = make_space("four")
    four_env = {"PROBE_DIR": os.path.join(space, "work"),
                "PROBE_EGRESS_HOST": egress_host, "PROBE_EGRESS_PORT": egress_port}
    rc_b, out_b, err_b = run([PY, os.path.join(space, "probe", "miniprobe.py")], env=four_env)
    spec = write_spec(space, "miniprobe.py",
                      [("PROBE_DIR", os.path.join(space, "work")),
                       ("PROBE_EGRESS_HOST", egress_host), ("PROBE_EGRESS_PORT", egress_port)],
                      "spec-miniprobe.json")
    rc_w, out_w, err_w = run([BIN, "run", "--spec", spec])

    emit("=== ① 四态（靶子 = scripts/sandbox-probes/miniprobe.py，已复制进 /private/tmp 靶子目录）===")
    emit("基线命令（无封闭，同一判别目标 %s:%s）：" % (egress_host, egress_port))
    emit("  python3 <靶子>/probe/miniprobe.py   ⇒ rc=%d" % rc_b)
    emit("  输出：%s" % out_b.strip())
    emit("茧壁命令：")
    emit("  wall/target/debug/zerg-wall run --spec <靶子>/spec-miniprobe.json   ⇒ rc=%d" % rc_w)
    emit("  子进程输出（stdout）：%s" % out_w.strip())
    emit("  茧壁留痕（stderr，含计划与**策略原文**）：")
    for line in err_w.rstrip("\n").splitlines():
        emit("    " + line)
    emit("")

    base = json.loads(out_b.strip()) if out_b.strip() else {}
    got = json.loads(out_w.strip()) if out_w.strip() else {}
    check("①-a 基线 unix socket 可用（靶子本身是好的）", base.get("unix_bind") == "ok",
          "基线 unix_bind=%s" % base.get("unix_bind"))
    check("①-b 基线出网可达（⇒ 出网断言有区分度）", base.get("egress") == "OPEN",
          "基线 egress=%s（目标 %s）" % (base.get("egress"), base.get("egress_target")))
    if base.get("egress") != "OPEN":
        print("!! 基线不可达 ⇒ 出网那一格没有区分度 ⇒ 硬失败（用 PROBE_EGRESS_HOST/PORT 换一个可达目标）")
        blocks.append("\n".join(log))
        with open(RECEIPT, "w", encoding="utf-8") as f:
            f.write("\n".join(blocks) + "\n")
        return EXIT_HARD
    check("①-c 封闭态 rc=0（跑起来了）", rc_w == 0, "rc=%d" % rc_w)
    check("①-d 封闭态 unix socket 仍可用（bind 那一档没加过滤）", got.get("unix_bind") == "ok",
          "unix_bind=%s" % got.get("unix_bind"))
    check("①-e 封闭态 loopback bind 仍可用", str(got.get("tcp_bind", "")).startswith("ok:"),
          "tcp_bind=%s" % got.get("tcp_bind"))
    check("①-f 封闭态空间内可写", got.get("write_work") == "ok", "write_work=%s" % got.get("write_work"))
    check("①-g 封闭态出网被拦（与历史回执 pF 一致）", str(got.get("egress", "")).startswith("BLOCKED"),
          "egress=%s" % got.get("egress"))
    check("①-h run 留下策略留痕（封闭那一步真的发生了）",
          "(deny default)" in err_w and "sandbox_init" in err_w)

    # ── ② 写空间外必须被拦（授权级封闭的本体）──
    space2 = make_space("writeout")
    outside = os.path.join(space2, "outside.txt")
    w_env = {"PROBE_DIR": os.path.join(space2, "work"), "PROBE_OUTSIDE": outside}
    rc_b2, out_b2, _ = run([PY, os.path.join(space2, "probe", "writeout-probe.py")], env=w_env)
    spec2 = write_spec(space2, "writeout-probe.py",
                       [("PROBE_DIR", os.path.join(space2, "work")), ("PROBE_OUTSIDE", outside)],
                       "spec-writeout.json")
    if os.path.exists(outside):
        os.remove(outside)      # 基线会把文件造出来 ⇒ 先删，封闭态那条断言才有区分度
    rc_w2, out_w2, err_w2 = run([BIN, "run", "--spec", spec2])

    emit("=== ② 写空间外（授权级封闭的本体：空间内可写、空间外写不进去）===")
    emit("基线命令：python3 <靶子>/probe/writeout-probe.py   ⇒ rc=%d" % rc_b2)
    emit("  输出：%s" % out_b2.strip())
    emit("基线产物已删（否则「封闭态不许造出该文件」那条断言恒假）：exists=%s" % os.path.exists(outside))
    emit("茧壁命令：wall/target/debug/zerg-wall run --spec <靶子>/spec-writeout.json   ⇒ rc=%d" % rc_w2)
    emit("  子进程输出（stdout）：%s" % out_w2.strip())
    emit("")

    b2 = json.loads(out_b2.strip()) if out_b2.strip() else {}
    g2 = json.loads(out_w2.strip()) if out_w2.strip() else {}
    check("②-a 基线空间内可写", b2.get("write_inside") == "ok", "write_inside=%s" % b2.get("write_inside"))
    check("②-b 基线空间外写得进去（⇒ 这一格有区分度）", b2.get("write_outside") == "OPEN",
          "write_outside=%s" % b2.get("write_outside"))
    check("②-c 封闭态空间内仍可写", g2.get("write_inside") == "ok", "write_inside=%s" % g2.get("write_inside"))
    check("②-d 封闭态空间外被拦", str(g2.get("write_outside", "")).startswith("BLOCKED"),
          "write_outside=%s" % g2.get("write_outside"))
    check("②-e 空间外的文件确实不存在", not os.path.exists(outside), outside)

    # ── ③ 变异验证（改坏实现 ⇒ 用例必须红 ⇒ 还原）──
    emit("=== ③ 变异验证（scripts/wall-macos-mutate.py：四条各自红在一个不同的判据上）===")
    rc_m, out_m, _ = run([sys.executable, os.path.join(ROOT, "scripts", "wall-macos-mutate.py")])
    for line in out_m.rstrip("\n").splitlines():
        emit("  " + line)
    check("③ 变异验证全绿（四条都「红 ⇒ 还原 sha 一致 ⇒ 复跑绿」）", rc_m == 0, "rc=%d" % rc_m)

    # ── ④ 断言汇总 + 落盘 ──
    red = [n for n, ok, _ in results if not ok]
    emit("")
    emit("=== 断言汇总（%d 条，红 %d 条）===" % (len(results), len(red)))
    for name, ok, detail in results:
        emit("  %s %s%s" % ("PASS" if ok else "FAIL", name, ("  —— " + detail) if detail else ""))
    if red:
        emit("")
        emit("!! 红：%s" % "、".join(red))

    shutil.rmtree(space, ignore_errors=True)
    shutil.rmtree(space2, ignore_errors=True)
    with open(RECEIPT, "w", encoding="utf-8") as f:
        f.write("\n".join(log) + "\n")
    print("")
    print("回执：%s（%d 行）" % (rel(RECEIPT), len(log)))
    return EXIT_RED if red else EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

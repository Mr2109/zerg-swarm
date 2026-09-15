#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""迁移桥门禁（任务 2'.3）：同一批卵配方，**两侧 argv 逐条对拍**。

  一侧 = Go  `hatch.BuildBwrapArgv`（经 `agent/internal/hatch/wall_bridge_test.go` 装配，测试即出口）
  另一侧 = Rust `zerg-wall plan --platform linux`

为什么要有它：茧壁是「同一个落点」的第二份实现 —— 两侧只要有一处「顺手改进」（少一个 `--unshare-*`、
绑定顺序换一下、包装 exec 少个 `$0`），真机上就是「看着起来了、封闭面却不同」的静默故障。对拍是
唯一能在**不换件**的前提下把这种漂移钉死的手段（本批不碰运行态 ⇒ 迁移桥就是本批的验收）。

**文件名即期望值**（`wall/testdata/bridge-specs/README.md` 有表；不另开清单 —— 两份清单必然漂移）：

  `ok-*`      两侧都应接受 ⇒ 再比 argv（两半判据见下）
  `reject-*`  两侧都应拒绝；任一侧放行 ⇒ 红

**两半判据**（差异写实，不藏）：
  ① `wall.argv[0]` 必须是 `bwrap`（可执行名，且不得以 `-` 开头）；
  ② `wall.argv[1:]` 与 Go 的 argv **逐条相同**（长度也要相同）。

退出码：`0` 全绿 · `1` 有对拍不一致 · `2` **硬失败**（探针/配方/产物缺失、Go 侧跑不起来、
茧壁二进制不在、无区分度、脚本自检不过）。硬失败**不许**被读成「通过」。

跑法：
    python3 scripts/compare-wall-argv.py
    python3 scripts/compare-wall-argv.py --self-test          # 先证「这面镜子能红」
    python3 scripts/compare-wall-argv.py --evidence <文件>     # 落一份完整回执（进仓）
"""

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPECS_DIR = os.path.join(ROOT, "wall", "testdata", "bridge-specs")
AGENT_DIR = os.path.join(ROOT, "agent")
WALL_DIR = os.path.join(ROOT, "wall")
DEFAULT_WALL_BIN = os.path.join(WALL_DIR, "target", "debug", "zerg-wall")

# 期望值前缀（文件名即期望值）
EXPECT_OK = "ok-"
EXPECT_REJECT = "reject-"

EXIT_OK, EXIT_MISMATCH, EXIT_HARD = 0, 1, 2


# ── 判据层（纯函数：好让它自己被自检喂合成输入） ─────────────────────────────

def executable_verdict(wall_argv):
    """判据①：`wall.argv[0]` 必须是被执行件名（`bwrap`），不是某个选项。"""
    if not wall_argv:
        return "argv 是空的 —— 没有可执行的东西"
    head = wall_argv[0]
    if head != "bwrap":
        return "argv[0] 应是可执行名 bwrap，实得 %r" % head
    if head.startswith("-"):
        return "argv[0] 以 '-' 开头 ⇒ `run` 永远起不来（缺陷形态）：%r" % head
    return ""


def argv_verdict(go_argv, wall_argv):
    """判据②：`wall.argv[1:]` 与 Go 侧 argv 逐条相同（长度与每一项都要）。"""
    tail = wall_argv[1:]
    if len(tail) != len(go_argv):
        return "长度不同：Go %d 项 / 茧壁 %d 项（去掉可执行名后）" % (len(go_argv), len(tail))
    for i, (g, w) in enumerate(zip(go_argv, tail)):
        if g != w:
            return "第 %d 项不同：Go %r ≠ 茧壁 %r" % (i, g, w)
    return ""


def compare_ok(go_res, wall_rc, wall_stdout, wall_stderr):
    """好配方：两侧都应接受，再比 argv。返回 (bool, 详情)。"""
    go_err = go_res.get("error") or ""
    if go_err:
        return False, "Go 侧拒绝了本该通过的配方：%s" % go_err
    if wall_rc != 0:
        return False, "茧壁 rc=%d（应为 0）：%s" % (wall_rc, wall_stderr.strip()[:400])
    try:
        payload = json.loads(wall_stdout.strip())
    except Exception as exc:  # noqa: BLE001 —— 报原文比报异常类型有用
        return False, "茧壁 stdout 不是一行合法 JSON（%s）：%r" % (exc, wall_stdout[:200])
    wall_argv = payload.get("argv") or []
    bad = executable_verdict(wall_argv)
    if bad:
        return False, bad
    bad = argv_verdict(go_res.get("argv") or [], wall_argv)
    if bad:
        return False, bad
    return True, "%d 项逐条一致（含可执行名共 %d 项）" % (len(go_res.get("argv") or []), len(wall_argv))


def compare_reject(go_res, wall_rc, wall_stdout, wall_stderr):
    """坏配方：两侧都拒才算一致（任一侧放行 ⇒ 红）。"""
    go_err = go_res.get("error") or ""
    if not go_err:
        return False, "Go 侧放行了一枚本该被拒的配方（argv 都出来了）"
    if wall_rc == 0:
        return False, "茧壁放行了一枚本该被拒的配方：%s" % wall_stdout.strip()[:300]
    return True, "两侧都拒（茧壁 rc=%d；Go：%s）" % (wall_rc, go_err[:160])


def compare_one(name, go_res, wall_rc, wall_stdout, wall_stderr):
    if name.startswith(EXPECT_OK):
        return compare_ok(go_res, wall_rc, wall_stdout, wall_stderr)
    if name.startswith(EXPECT_REJECT):
        return compare_reject(go_res, wall_rc, wall_stdout, wall_stderr)
    return False, "文件名前缀认不得（只有 %s / %s 两种；文件名即期望值，不许静默跳过）" % (
        EXPECT_OK, EXPECT_REJECT)


# ── 两侧的取数 ──────────────────────────────────────────────────────────────

def run_go_dump(specs_dir, out_path):
    """跑 Go 侧出口（`-run` 点名那一条；不带管道，直接拿真实 rc）。"""
    env = dict(os.environ)
    env.setdefault("GOFLAGS", "-mod=mod")
    env.setdefault("GOSUMDB", "off")
    env.setdefault("GOPROXY", "https://goproxy.cn,direct")
    env["ZERG_BRIDGE_SPEC_DIR"] = specs_dir          # 必须绝对路径：go test 的 cwd 是包目录
    env["ZERG_BRIDGE_OUT"] = out_path
    cmd = ["go", "test", "./internal/hatch/", "-run", "^TestWallBridgeDumpBwrapArgv$", "-count=1"]
    p = subprocess.run(cmd, cwd=AGENT_DIR, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode, p.stdout.decode("utf-8", "replace")


def ensure_wall_bin(path, no_build):
    """茧壁二进制：不在就编（零依赖 crate ⇒ 不需要网络）。编不出来 ⇒ 硬失败。"""
    if os.path.exists(path):
        return path, ""
    if no_build:
        return "", "茧壁二进制 %s 不在，且指定了 --no-build（拒：不许静默跳过）" % path
    cargo = shutil.which("cargo") or os.path.join(os.path.expanduser("~"), ".cargo", "bin", "cargo")
    if not os.path.exists(cargo):
        return "", "找不到 cargo（试过 PATH 与 ~/.cargo/bin）—— 先装 Rust 工具链"
    p = subprocess.run([cargo, "build", "--manifest-path", os.path.join(WALL_DIR, "Cargo.toml")],
                       cwd=WALL_DIR, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if p.returncode != 0:
        return "", "cargo build 失败（rc=%d）：\n%s" % (p.returncode, p.stdout.decode("utf-8", "replace")[-2000:])
    if not os.path.exists(path):
        return "", "cargo build 说成功，但 %s 还是不在（拒绝把「编了」当成「编出来了」）" % path
    return path, ""


def run_wall_plan(wall_bin, spec_path):
    p = subprocess.run([wall_bin, "plan", "--spec", spec_path, "--platform", "linux"],
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return (p.returncode,
            p.stdout.decode("utf-8", "replace"),
            p.stderr.decode("utf-8", "replace"))


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


# ── 自检：先证「这面镜子能红」 ───────────────────────────────────────────────

def self_test(rows, emit=None):
    """两条：①合成输入喂判据层；②拿**真实产物**改坏一侧，判据必须红。

    为什么必须做②：合成输入只能证明判据函数写对了，证明不了「喂进去的真是这两侧的真实产物」
    —— 上一轮的教训是「探针没红时先怀疑探针」。
    """
    if emit is None:
        emit = print
    ok = True

    def check(label, cond, detail=""):
        nonlocal ok
        if cond:
            emit("  [自检 ok] %s" % label)
        else:
            ok = False
            emit("  [自检 红] %s %s" % (label, detail))

    good_go = ["--ro-bind", "/usr", "/usr", "--unshare-pid", "--die-with-parent"]
    good_wall = ["bwrap"] + good_go

    check("① 两侧一致 ⇒ 绿", not executable_verdict(good_wall) and not argv_verdict(good_go, good_wall))
    check("② 中段漂移 ⇒ 红",
          bool(argv_verdict(good_go, good_wall[:2] + ["--ro-bind-x"] + good_wall[3:])))
    check("③ 少一项 ⇒ 红", bool(argv_verdict(good_go, good_wall[:-1])))
    check("④ 可执行名不对 ⇒ 红", bool(executable_verdict(["--ro-bind"] + good_go)))
    check("⑤ 可执行名以 '-' 开头 ⇒ 红", bool(executable_verdict(["-bwrap"] + good_go)))
    check("⑥ 空 argv ⇒ 红", bool(executable_verdict([])))

    for row in rows:
        if not row["name"].startswith(EXPECT_OK):
            continue
        # ② 真实产物三面改坏：判据必须各自红
        name = row["name"]
        g = row["go_argv"]
        w = row["wall_argv"]
        check("① 真实产物 %s：原样 ⇒ 绿" % name,
              not executable_verdict(w) and not argv_verdict(g, w))
        check("② 真实产物 %s：茧壁中段改坏 ⇒ 红" % name,
              bool(argv_verdict(g, w[:3] + ["--mutation-drift"] + w[4:])))
        check("③ 真实产物 %s：茧壁可执行名改坏 ⇒ 红" % name,
              bool(executable_verdict(["sandbox-exec"] + w[1:])))
        break  # 一份够；多份只是重复同一件事

    if ok:
        print("自检：全部通过（含真实产物改坏的那三条）")
    return ok


def main():
    ap = argparse.ArgumentParser(description="茧壁迁移桥：Go BuildBwrapArgv ↔ zerg-wall plan 逐条对拍")
    ap.add_argument("--wall", default=DEFAULT_WALL_BIN, help="茧壁二进制路径（默认 wall/target/debug/zerg-wall）")
    ap.add_argument("--specs", default=SPECS_DIR, help="配方目录（默认 wall/testdata/bridge-specs）")
    ap.add_argument("--no-build", action="store_true", help="二进制不在也不许编译（直接硬失败）")
    ap.add_argument("--self-test", action="store_true", help="先跑判据自检（不过就拒绝当真值）")
    ap.add_argument("--evidence", default="", help="把完整回执写到这个文件（可进仓）")
    args = ap.parse_args()

    lines = []           # 回执正文

    def say(msg):
        print(msg)
        lines.append(msg)

    # ① 配方目录：不存在 / 没有 ok 或没有 reject ⇒ 硬失败（无区分度）
    if not os.path.isdir(args.specs):
        print("!! 配方目录不在：%s —— 硬失败（不许静默当「没东西可比」）" % args.specs)
        return EXIT_HARD
    names = sorted(n for n in os.listdir(args.specs) if n.endswith(".json"))
    n_ok = sum(1 for n in names if n.startswith(EXPECT_OK))
    n_rej = sum(1 for n in names if n.startswith(EXPECT_REJECT))
    if not names or n_ok == 0 or n_rej == 0:
        print("!! 配方集合没有区分度（共 %d 份：ok %d / reject %d）—— 硬失败："
              "「零条对拍、零条失败」的绿比红更危险" % (len(names), n_ok, n_rej))
        return EXIT_HARD
    unknown = [n for n in names if not (n.startswith(EXPECT_OK) or n.startswith(EXPECT_REJECT))]
    if unknown:
        print("!! 认不得的文件名前缀：%s —— 硬失败（文件名即期望值）" % ", ".join(unknown))
        return EXIT_HARD

    # ② 茧壁二进制
    wall_bin, why = ensure_wall_bin(args.wall, args.no_build)
    if not wall_bin:
        print("!! %s" % why)
        return EXIT_HARD

    # ③ Go 侧产物
    dump_path = os.path.join("/tmp", "zerg-bridge-go-%d.json" % os.getpid())
    if os.path.exists(dump_path):
        os.remove(dump_path)
    go_rc, go_log = run_go_dump(os.path.abspath(args.specs), dump_path)
    if go_rc != 0 or not os.path.exists(dump_path):
        print("!! Go 侧出口跑不起来（rc=%d）：\n%s" % (go_rc, go_log[-2000:]))
        return EXIT_HARD
    try:
        with open(dump_path, "r", encoding="utf-8") as f:
            go_dump = json.load(f)
    except Exception as exc:  # noqa: BLE001
        print("!! 读不出 Go 侧产物 %s：%s" % (dump_path, exc))
        return EXIT_HARD

    missing = [n for n in names if n not in go_dump]
    if missing:
        print("!! Go 侧产物缺这几份配方：%s —— 硬失败（缺 = 没比过，不是通过）" % ", ".join(missing))
        return EXIT_HARD

    # ④ 逐份对拍
    rows = []
    bad_rows = []
    for n in names:
        spec_path = os.path.join(args.specs, n)
        wall_rc, wall_out, wall_err = run_wall_plan(wall_bin, spec_path)
        ok, detail = compare_one(n, go_dump[n], wall_rc, wall_out, wall_err)
        wall_argv = []
        if wall_rc == 0 and wall_out.strip():
            try:
                wall_argv = json.loads(wall_out.strip()).get("argv") or []
            except Exception:  # noqa: BLE001
                wall_argv = []
        rows.append({
            "name": n,
            "ok": ok,
            "detail": detail,
            "go_argv": go_dump[n].get("argv") or [],
            "go_error": go_dump[n].get("error") or "",
            "wall_rc": wall_rc,
            "wall_argv": wall_argv,
            "wall_stderr": wall_err.strip(),
        })
        if not ok:
            bad_rows.append(rows[-1])

    say("=== 茧壁迁移桥（任务 2'.3）：Go hatch.BuildBwrapArgv ↔ zerg-wall plan ===")
    say("配方目录：%s（%d 份：ok %d / reject %d）" % (args.specs, len(names), n_ok, n_rej))
    say("茧壁二进制：%s" % wall_bin)
    for r in rows:
        say("%-4s %-42s %s" % ("绿" if r["ok"] else "红", r["name"], r["detail"]))

    if args.self_test:
        say("")
        say("--- 判据自检（先证这面镜子能红）---")
        if not self_test(rows, emit=say):
            print("!! 判据自检不过 ⇒ 拒绝把本次结果当真值（硬失败）")
            return EXIT_HARD

    # ⑤ 判定与回执（回执里必须带上**本次判定**那一行 —— 先写文件后打判定会让回执缺结论）
    print("")
    if bad_rows:
        say("!! 对拍不一致 %d 份（见上）" % len(bad_rows))
        verdict = EXIT_MISMATCH
    else:
        say("对拍全绿：%d 份配方两侧 argv 逐条一致" % len(rows))
        verdict = EXIT_OK

    if args.evidence:
        with open(args.evidence, "w", encoding="utf-8") as f:
            f.write("\n".join(lines))
            f.write("\n")
            f.write("\n--- 逐份明细（两侧 argv 原文）---\n")
            for r in rows:
                f.write("\n[%s] %s\n" % ("绿" if r["ok"] else "红", r["name"]))
                if r["go_error"]:
                    f.write("  Go   ：拒 —— %s\n" % r["go_error"])
                else:
                    f.write("  Go   ：%d 项\n" % len(r["go_argv"]))
                    for i, a in enumerate(r["go_argv"]):
                        f.write("    %3d %s\n" % (i, a))
                f.write("  茧壁 ：rc=%d\n" % r["wall_rc"])
                for i, a in enumerate(r["wall_argv"]):
                    f.write("    %3d %s\n" % (i, a))
            f.write("\n--- 制品与源码 sha256（本机制品对拍时用于复核）---\n")
            for p in [wall_bin,
                      os.path.join(WALL_DIR, "src", "platform", "linux.rs"),
                      os.path.join(WALL_DIR, "tests", "plan_linux.rs"),
                      os.path.join(AGENT_DIR, "internal", "hatch", "hatch.go"),
                      os.path.join(AGENT_DIR, "internal", "hatch", "wall_bridge_test.go"),
                      # 本脚本自己的 sha 也写进来：回执被引用时能自证「是哪一版门禁跑的」
                      os.path.abspath(__file__)]:
                if os.path.exists(p):
                    f.write("%s  %s\n" % (sha256(p), p))
        print("回执已落：%s" % args.evidence)

    return verdict


if __name__ == "__main__":
    sys.exit(main())

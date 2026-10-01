#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""judge-lock.py —— 判据**件级互斥锁 ＋ 槽池取槽**（M1-09 · 只读判据件 · fail-closed）

件级锁：`<锁目录>/judge-<判据名>.lock` —— 同一判据名**至多一个**持有者；
不同判据名**互不阻塞**（件级 ≠ 全表级）。锁目录与既有全表实例锁 `gate-run.lock`
**同源**（逐字同一表达式）：`${ZERG_GATE_LOCK_DIR:-${TMPDIR:-/tmp}/zerg-gate-instance}/`
（参 `scripts/gates/precommit-gates.sh` 的 `gate_instance_lock_path`）。

取锁原语：内核 `flock(LOCK_EX|LOCK_NB)`（Python `fcntl.flock`）—— 与 Go 侧 / 脚本侧那把
`flock(LOCK_EX|LOCK_NB)` **同口径**（无 TTL · 锁文件常驻不删）。

★ 单件跑**不拿**全表锁：本件**从不**打开 `gate-run.lock`；全表互斥由门脚本本体负责。
  拿不到件级锁时**如实报**（`detail=judge_locked_by_other`）· 不静默降级。

★ 槽池（与全表跑**同一座**槽池取槽）：容量 = **8**（并发预算「**取小不相加**」的取小档 ——
  不是把各面预算相加得 24）。`--slots N` 请求的 N 必须 ≤ 8；**超预算 ⇒ 不给结论（rc=2）**，
  ✗ 不硬起 / ✗ 不超发。8 个槽全被占 ⇒ **排队等**，等过 `--wait` 秒仍拿不到 ⇒
  **不给结论（rc=2）**，✗ 不当绿。

退码（全用现读自有号 · ✗ 不新占号）：
  0  = 正控：件级锁 ＋ 槽位都拿到（可安全放行）
  2  = **不给结论**：用法错 / 判据名非法 / 超槽池预算 / 排队过久拿不到槽 /
       同名判据已被别的持有者占着（`detail=judge_locked_by_other` —— **此刻取不到结论**，
       ✗ 不是「同 fp 内容不同」那种幂等冲突）/ `--self-test` 不过
  1  = 保留（本件不产出该档；承主表 `failed`）

用法：
    python3 scripts/gates/judge-lock.py --self-test
    python3 scripts/gates/judge-lock.py --name <判据名> [--slots N] [--wait 秒]
"""
import argparse
import errno
import fcntl
import os
import subprocess
import sys
import tempfile
import time

SLOT_CAPACITY = 8           # 取小不相加：槽池容量 = 8（不是各面预算相加的 24）
SLOT_PREFIX = "judge-slot-"
JUDGE_PREFIX = "judge-"

RC_OK = 0
RC_FAIL = 1
RC_USAGE = 2
# 同名判据锁被占 = 「此刻取不到结论」（主人 2026-10-01 裁）：不占 14
# （14 是「同 fp 内容不同」的幂等冲突语义，拿来标锁竞争即误标）⇒ 与 RC_USAGE 同档走 2。
RC_CONFLICT = 2

# 自检用（不参与真目标档的语义）：持锁等待秒数，用于在同一进程外制造**真第二个进程**。
SELFTEST_HOLD_FLAG = "--_selftest-hold"


def lock_dir():
    """锁目录 —— 与 `gate-run.lock` **同源**（逐字同一表达式）。"""
    base = os.environ.get("ZERG_GATE_LOCK_DIR")
    if not base:
        base = os.path.join(os.environ.get("TMPDIR") or "/tmp", "zerg-gate-instance")
    return base


def judge_lock_path(name, base=None):
    return os.path.join(base or lock_dir(), JUDGE_PREFIX + name + ".lock")


def slot_lock_path(i, base=None):
    return os.path.join(base or lock_dir(), "%s%d.lock" % (SLOT_PREFIX, i))


def _try_lock(path):
    """非阻塞拿内核 flock；拿到 ⇒ 返 fd，拿不到 ⇒ 返 None（**如实**，不降级）。"""
    d = os.path.dirname(path)
    os.makedirs(d, exist_ok=True)
    fd = os.open(path, os.O_CREAT | os.O_RDWR, 0o644)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError as e:
        os.close(fd)
        if e.errno in (errno.EWOULDBLOCK, errno.EAGAIN, errno.EACCES):
            return None
        raise
    return fd


def acquire_judge(name, base=None):
    return _try_lock(judge_lock_path(name, base))


def acquire_slot(slots, wait, base=None):
    """在同一座容量 8 的槽池里取一格；全占则排队，等过 wait 秒 ⇒ None（不给结论）。"""
    slots = max(1, min(int(slots), SLOT_CAPACITY))
    deadline = time.time() + max(0.0, float(wait))
    while True:
        for i in range(slots):
            fd = _try_lock(slot_lock_path(i, base))
            if fd is not None:
                return fd, i
        if time.time() >= deadline:
            return None, -1
        time.sleep(0.05)


def cmd_acquire(name, slots, wait):
    if not name or "/" in name or name.strip() != name or os.sep in name:
        print("⛔ 判据名非法（空 / 含路径分隔符 / 首尾空白）⇒ 不给结论（rc=2）", file=sys.stderr)
        return RC_USAGE
    if slots > SLOT_CAPACITY:
        print("⛔ 超槽池预算：请求 --slots %d > 容量 %d ⇒ **不硬起 / 不超发** ⇒ 不给结论（rc=2）"
              % (slots, SLOT_CAPACITY), file=sys.stderr)
        return RC_USAGE

    jfd = acquire_judge(name)
    if jfd is None:
        print("✗ 件级锁被占：judge-%s.lock 已有持有者 ⇒ detail=judge_locked_by_other（rc=2 · 此刻取不到结论）"
              % name, file=sys.stderr)
        return RC_CONFLICT

    sfd, idx = acquire_slot(slots, wait)
    if sfd is None:
        os.close(jfd)
        print("✗ 槽池全占（容量 %d · 等过 %ss 仍无空槽）⇒ 排队不给结论（rc=2）· 不硬起 / 不超发"
              % (SLOT_CAPACITY, wait), file=sys.stderr)
        return RC_USAGE

    print("✓ 件级锁 judge-%s.lock ＋ 槽位 %d/%d 已取（rc=0）· 锁目录=%s"
          % (name, idx + 1, SLOT_CAPACITY, lock_dir()))
    return RC_OK


def _selftest_hold(name, slots, wait, hold):
    """自检子进程：真持锁 hold 秒 —— 必须先打印 HELD 供父进程同步（**两个真进程**）。"""
    rc = cmd_acquire(name, slots, wait)
    if rc != RC_OK:
        print("CHILD_RC=%d" % rc)
        return rc
    sys.stdout.write("HELD\n")
    sys.stdout.flush()
    time.sleep(hold)
    return RC_OK


def _run_child(args, base):
    """起**真第二个进程**拿锁（同进程内 flock 可重复加锁 ⇒ 必须跨进程才有效）。"""
    env = dict(os.environ)
    env["ZERG_GATE_LOCK_DIR"] = base
    return subprocess.Popen(
        [sys.executable, os.path.abspath(__file__)] + args,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env)


def cmd_self_test():
    ok = True
    with tempfile.TemporaryDirectory(prefix="judge-lock-self-") as base:
        # ── 正控：无竞争 ⇒ 拿得到件级锁 ＋ 槽位（rc=0）──────────────
        rc = _call(base, ["--name", "alpha", "--slots", "4", "--wait", "0"])
        if rc == RC_OK:
            print("✓ 正控：无竞争 ⇒ 件级锁＋槽位都得（rc=0）")
        else:
            print("✗ 正控：无竞争却拿不到锁（rc=%d）" % rc)
            ok = False

        # ── 成对负控 A：**同名**两路竞争 ⇒ 第二路必 rc=2 ───────────
        child = _run_child(["--name", "dup", "--slots", "2", "--wait", "0",
                            SELFTEST_HOLD_FLAG, "5"], base)
        held = ""
        while True:
            line = child.stdout.readline()
            if not line:
                break
            line = line.strip()
            if line == "HELD":
                held = line
                break
        if held != "HELD":
            print("✗ 负控A 前置：子进程没拿到锁（读回 %r）" % held)
            ok = False
        else:
            rc_same = _call(base, ["--name", "dup", "--slots", "1", "--wait", "0"])
            if rc_same == RC_CONFLICT:
                print("✓ 负控A（同名两路竞争）：第二路现 rc=2（被占 · 此刻取不到结论）")
            else:
                print("✗ 负控A（同名两路竞争）：该红没红 ⇒ rc=%d（**锁形同虚设**）" % rc_same)
                ok = False

            # ── 成对负控 B：**不同名**不互阻 ⇒ 该路必拿得到（rc=0）──
            rc_other = _call(base, ["--name", "other", "--slots", "2", "--wait", "0"])
            if rc_other == RC_OK:
                print("✓ 负控B（不同件不互阻）：另一判据名照拿（rc=0）")
            else:
                print("✗ 负控B（不同件不互阻）：被同名锁误阻 ⇒ rc=%d" % rc_other)
                ok = False

            # ── 成对负控 C：槽池被占满 ⇒ 排队过久 ⇒ rc=2（不给结论）──
            rc_slot = _call(base, ["--name", "third", "--slots", "1", "--wait", "0"])
            if rc_slot == RC_USAGE:
                print("✓ 负控C（槽池占满 · 排队过久）：现 rc=2（不给结论 · 不硬起）")
            else:
                print("✗ 负控C（槽池占满）：该不给结论却没给 ⇒ rc=%d" % rc_slot)
                ok = False

            # ── 负控 D：超槽池预算 ⇒ rc=2（✗ 不超发）────────────────
            rc_budget = _call(base, ["--name", "fourth", "--slots", "9", "--wait", "0"])
            if rc_budget == RC_USAGE:
                print("✓ 负控D（超预算 --slots 9 > 8）：现 rc=2（不硬起 / 不超发）")
            else:
                print("✗ 负控D（超预算）：该拒没拒 ⇒ rc=%d" % rc_budget)
                ok = False

        try:
            child.terminate()
            child.wait(timeout=5)
        except Exception:
            pass

    return RC_OK if ok else RC_USAGE


def _call(base, args, capture=False):
    """起**真第二个进程**跑一次 CLI（同进程内 flock 可重复加锁 ⇒ 锁类自检必须跨进程）。"""
    p = _run_child(list(args), base)
    out, _ = p.communicate(timeout=30)
    if capture:
        return p.returncode, out
    return p.returncode


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    # 自检子进程形态（内部用 · 不进真目标面）
    hold = 0.0
    if SELFTEST_HOLD_FLAG in argv:
        i = argv.index(SELFTEST_HOLD_FLAG)
        hold = float(argv[i + 1])
        del argv[i:i + 2]
        ap = argparse.ArgumentParser(add_help=False)
        ap.add_argument("--name", required=True)
        ap.add_argument("--slots", type=int, default=SLOT_CAPACITY)
        ap.add_argument("--wait", type=float, default=0.0)
        a = ap.parse_args(argv)
        return _selftest_hold(a.name, a.slots, a.wait, hold)

    ap = argparse.ArgumentParser(description="判据件级互斥锁 ＋ 槽池取槽（M1-09）")
    g = ap.add_mutually_exclusive_group()
    g.add_argument("--self-test", dest="self_test", action="store_true", help="成对负控自检")
    g.add_argument("--name", help="判据名（取件级锁 judge-<名>.lock）")
    ap.add_argument("--slots", type=int, default=SLOT_CAPACITY,
                    help="请求槽数（≤ %d；超预算 ⇒ rc=2）" % SLOT_CAPACITY)
    ap.add_argument("--wait", type=float, default=0.0, help="全占时排队等待秒数（超时 ⇒ rc=2）")
    args = ap.parse_args(argv)

    if args.self_test:
        return cmd_self_test()
    if args.name:
        return cmd_acquire(args.name, args.slots, args.wait)
    sys.stderr.write("用法错：`--self-test` 或 `--name <判据名>` 二选一必给一个（都不给 ⇒ rc=2）\n")
    return RC_USAGE


if __name__ == "__main__":
    sys.exit(main())

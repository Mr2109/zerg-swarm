#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""verify-two-states.py —— 判据 7「两态可验」门禁（批 1 任务 1.4）

判据原文（设计稿 §六 判据 7）：
  「给 loopback 但拦出网」必须**两态可验**：能证明「空间内通信可用」且「出网被拦」，
  两条**都必须有红的版本**；探针要落进仓（`scripts/sandbox-probes/`），不许留在 /tmp。

本脚本把这条判据做成可执行门禁（纯本地、无害靶子、不碰生产/不换件/不重启）：

  断言 A「空间内通信可用」＝封闭空间内 unix socket bind 成功 **且** loopback 自己连自己能收到回包；
  断言 B「出网被拦」    ＝封闭空间内对**判别目标**的连接被拦 **且 基线（无封闭）同一目标可达**。
                          后半句是关键：少了基线可达这一格，B 可能是「探针本身坏了」造成的
                          假阳性（§10.7 结论 2 的测量学教训：宿主无出网时该格没有区分度）。

退出码（判据不许含糊 ⇒ 本脚本**不静默跳过**）：
  0 = 两态都成立（绿）
  1 = 断言红（真的没成立）
  2 = 环境/探针问题（探针缺失或路径漂移、沙箱程序缺失、基线无区分度）—— 硬失败，不是跳过

用法：
  python3 scripts/sandbox-probes/verify-two-states.py               # 真验（默认，逐条打印实测值）
  python3 scripts/sandbox-probes/verify-two-states.py --self-test   # 变异验证：四态成对 + 护栏自检
  python3 scripts/sandbox-probes/verify-two-states.py --json         # 同上，附机器可读输出
  TWO_STATE_TARGETS="<cluster-ip>:8580" python3 ... verify-two-states.py  # 覆盖判别目标（宿主无出网时必用）

平台：macOS 走 Seatbelt（`sandbox-exec` + 仓内 pF 黄金配方）；Linux 走 bwrap 全套 unshare。
本机（macOS）可跑 macOS 分支；**Linux 分支必须在 Linux 上跑一次才算验过**（脚本在非 Linux 上不假装验过）。
"""

import json
import os
import shutil
import subprocess
import sys

PROBE_ROOT = os.path.dirname(os.path.abspath(__file__))
PY = sys.executable or "python3"
RUN_TIMEOUT_S = 60

# —— 探针与沙箱程序：缺一即硬失败（判据：不许静默跳过）——
REQUIRED = {
    "darwin": ["miniprobe.py", "conn_jit.py", "pF.sb"],
    "linux": ["lx2-linux.py", "conn_jit.py"],
}
# —— 变异（红的版本）所需的额外件：只被 --self-test 用到 ——
MUTATION_EXTRA = {
    "darwin": ["pA.sb", "pE.sb"],
    "linux": [],
}
ENCLOSER = {"darwin": "sandbox-exec", "linux": "bwrap"}

# 判别目标候选（按序取第一个**基线可达**的）：公网 → 局域网。
# 宿主无出网（实测 X3 基线对公网即 Timeout）时会自动落到局域网那一格（§10.7 结论 2）。
DEFAULT_TARGETS = ["1.1.1.1:443", "<worker-host>:8580"]

# 「可达」的证据词法：OPEN = 连上了；ConnectionRefused = 对方回了 RST ⇒ **包已经出空间了**，同样证明可达。
# 于是 miniprobe 的 `BLOCKED:ConnectionRefusedError` 与 lx2 的裸 `ConnectionRefusedError` 都算可达。
REACHABLE_TOKENS = ("OPEN", "ConnectionRefused")
# 其余一切（PermissionError / OSError / Timeout / FAIL:* / 空串）一律当**被拦**：
# 未知词法当「被拦」是保守方向 —— 宁可红，也不许把「不知道」说成「已封闭」。

LINUX_WORK = "/tmp/lxwork"          # Linux 侧空间内可写区（如探针里 --tmpfs /tmp 之后新造）


class HardFail(Exception):
    """环境/探针问题 ⇒ rc=2（硬失败，不是跳过）。"""


class ProbeError(HardFail):
    pass


# ————————————————————————— 纯函数（--self-test 直接喂合成输入做变异验证）—————————————————————————

def classify_reachable(value):
    v = (value or "").strip()
    if not v:
        return False
    for tok in REACHABLE_TOKENS:
        if tok in v:
            return True
    return False


def claim_loopback(inside):
    """断言 A。返回 (True/False, 判词)。"""
    if not inside:
        return (False, "缺封闭态探针结果")
    unix_v = inside.get("unix_bind")
    lp_v = inside.get("loopback")
    if unix_v == "ok" and lp_v == "ok":
        return (True, "unix socket bind=ok · loopback 自己连自己=ok")
    return (False, "unix_bind=%s · loopback_connect=%s" % (unix_v, lp_v))


def claim_egress(inside, baseline):
    """断言 B。返回 (True/False/None, 判词)；None = 无区分度（⇒ 硬失败，不是红也不是绿）。"""
    if not inside or not baseline:
        return (False, "缺基线或封闭态探针结果")
    b_v = baseline.get("egress")
    i_v = inside.get("egress")
    if not classify_reachable(b_v):
        return (None, "基线对 %s 也不可达（%s）⇒ 本机这一格无区分度，换判别目标"
                      "（TWO_STATE_TARGETS）再跑" % (baseline.get("target"), b_v))
    if classify_reachable(i_v):
        return (False, "封闭态出网仍可达（%s）—— 正是本断言的红版本" % i_v)
    return (True, "基线可达（%s）· 封闭态被拦（%s）" % (b_v, i_v))


def require_probes(root, names, platform):
    missing = [n for n in names if not os.path.isfile(os.path.join(root, n))]
    if missing:
        raise HardFail("探针缺失或路径漂移：%s（在 %s 下）⇒ 判据 7 要求硬失败，不许静默跳过"
                       % (", ".join(missing), root))
    binname = ENCLOSER.get(platform)
    if binname and not shutil.which(binname):
        raise HardFail("沙箱程序 %s 不在 PATH ⇒ 无法验「封闭」这一态（硬失败，不是跳过）" % binname)


# ————————————————————————— 真跑 —————————————————————————

def run_probe(probe, env_extra, prefix):
    argv = list(prefix) + [PY, os.path.join(PROBE_ROOT, probe)]
    env = dict(os.environ)
    env.update(env_extra)
    try:
        p = subprocess.run(argv, capture_output=True, text=True, timeout=RUN_TIMEOUT_S,
                           env=env, cwd=PROBE_ROOT)
    except subprocess.TimeoutExpired:
        raise ProbeError("探针超时（>%ss）：%s" % (RUN_TIMEOUT_S, probe))
    if p.returncode != 0:
        raise ProbeError("探针 rc=%d：%s\n%s" % (p.returncode, probe, (p.stderr or "").strip()[:400]))
    for line in reversed((p.stdout or "").strip().splitlines()):
        line = line.strip()
        if line.startswith("{"):
            try:
                return json.loads(line)
            except ValueError:
                continue
    raise ProbeError("探针没有输出 JSON：%s（stdout=%r）" % (probe, (p.stdout or "")[:200]))


def darwin_prefix(profile):
    if profile is None:
        return []
    return ["sandbox-exec", "-f", os.path.join(PROBE_ROOT, profile)]


def linux_prefix(state):
    """state: "none" | "L2"（含 --unshare-net）| "L1"（共享宿主网络栈）。"""
    if state == "none":
        return []
    argv = ["bwrap", "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc",
            "--tmpfs", "/tmp", "--dir", LINUX_WORK,
            "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--unshare-cgroup"]
    if state == "L2":
        argv.append("--unshare-net")
    argv.append("--die-with-parent")
    return argv


def prefix_for(platform, profile):
    return darwin_prefix(profile) if platform == "darwin" else linux_prefix("none" if profile is None else profile)


def parse_target(spec):
    host, _, port = spec.partition(":")
    return host.strip(), int(port.strip())


def probe_suite(platform, prefix, target):
    """跑一支平台对应探针组，归一成同一形状（unix_bind / loopback / egress / wx_jit / target）。"""
    host, port = target
    if platform == "darwin":
        env = {"PROBE_EGRESS_HOST": host, "PROBE_EGRESS_PORT": str(port)}
        mini = run_probe("miniprobe.py", env, prefix)
        conn = run_probe("conn_jit.py", env, prefix)
        return {"unix_bind": mini.get("unix_bind"), "tcp_bind": mini.get("tcp_bind"),
                "loopback": conn.get("loopback_connect"), "wx_jit": conn.get("wx_jit_call"),
                "egress": mini.get("egress"), "target": mini.get("egress_target")}
    env = {"PROBE_LAN_HOST": host, "PROBE_LAN_PORT": str(port)}
    lx = run_probe("lx2-linux.py", env, prefix)
    conn = run_probe("conn_jit.py", env, prefix)
    return {"unix_bind": lx.get("unix_bind"), "tcp_bind": lx.get("tcp_loopback_bind"),
            "loopback": conn.get("loopback_connect"), "wx_jit": conn.get("wx_jit_call"),
            "egress": lx.get("lan_connect"), "target": lx.get("lan_target")}


def pick_target(platform, targets):
    """逐个跑**基线**，取第一个基线可达者；都不通 ⇒ 硬失败（无区分度）。"""
    tried = []
    for spec in targets:
        target = parse_target(spec)
        res = probe_suite(platform, [], target)
        tried.append((spec, res.get("egress")))
        if classify_reachable(res.get("egress")):
            return spec, res, tried
    raise HardFail("候选判别目标全都基线不可达 ⇒ 出网探针无区分度（%s）⇒ 用 TWO_STATE_TARGETS "
                   "指定一个本机可达的局域网目标再跑" % "; ".join("%s→%s" % t for t in tried))


def targets_from_env():
    raw = os.environ.get("TWO_STATE_TARGETS", "").strip()
    return [s.strip() for s in raw.split(",") if s.strip()] if raw else list(DEFAULT_TARGETS)


# ————————————————————————— 模式一：真验 —————————————————————————

def mode_verify(platform, as_json):
    require_probes(PROBE_ROOT, REQUIRED[platform], platform)
    spec, baseline, tried = pick_target(platform, targets_from_env())
    inside = probe_suite(platform, prefix_for(platform, "pF.sb" if platform == "darwin" else "L2"), parse_target(spec))

    a_ok, a_note = claim_loopback(inside)
    b_ok, b_note = claim_egress(inside, baseline)

    rows = [
        ("判别目标", spec, "基线实测 %s ⇒ %s" % (baseline.get("egress"),
                                            "可达（有区分度）" if classify_reachable(baseline.get("egress")) else "不可达")),
        ("断言 A 空间内通信可用", "PASS" if a_ok else "FAIL", a_note),
        ("断言 B 出网被拦", "PASS" if b_ok is True else ("FAIL" if b_ok is False else "无区分度"), b_note),
        ("（信息）JIT W^X", str(inside.get("wx_jit")), "§10.10：基线/封闭态都应 ok:42（判据 7 不含它）"),
    ]
    print("茧壁 · 判据 7 两态可验（平台 %s · 沙箱 %s）" % (platform, ENCLOSER.get(platform)))
    for name, val, note in rows:
        print("  %-22s %-8s %s" % (name, val, note))
    if len(tried) > 1:
        print("  （判别目标试探顺序：%s）" % "; ".join("%s→%s" % t for t in tried))

    if as_json:
        print(json.dumps({"platform": platform, "target": spec, "baseline": baseline,
                          "inside": inside, "claimA": a_ok, "claimB": b_ok}, ensure_ascii=False))

    if b_ok is None:
        return 2
    return 0 if (a_ok and b_ok) else 1


# ————————————————————————— 模式二：变异验证（自己也要能失败）—————————————————————————

EXPECT = {
    "darwin": [
        {"name": "基线（无封闭）", "profile": None, "A": True, "B": False, "jit": "ok:42",
         "why": "B 的红版本：出网本就可达"},
        {"name": "pF 黄金配方", "profile": "pF.sb", "A": True, "B": True, "jit": "ok:42",
         "why": "两态都该成立"},
        {"name": "pA 无网络规则", "profile": "pA.sb", "A": False, "B": True, "jit": None,
         "why": "A 的红版本：unix/loopback 被拒"},
        {"name": "pE 全放网络 network*", "profile": "pE.sb", "A": True, "B": False, "jit": None,
         "why": "B 的红版本：封闭态出网仍开"},
    ],
    "linux": [
        {"name": "基线（无封闭）", "profile": None, "A": True, "B": False, "jit": "ok:42",
         "why": "B 的红版本：出网本就可达"},
        {"name": "L2 全套 unshare + --unshare-net", "profile": "L2", "A": True, "B": True, "jit": "ok:42",
         "why": "两态都该成立"},
        {"name": "L1 共享宿主网络栈", "profile": "L1", "A": True, "B": False, "jit": None,
         "why": "B 的红版本：没 unshare 网络 ⇒ 出网仍开"},
    ],
}


def selftest_guards(platform):
    """护栏自检：合成输入下，判定层必须按预期红/绿；探针缺失必须硬失败。"""
    cases = []
    cases.append(("classify: OPEN ⇒ 可达", classify_reachable("OPEN") is True))
    cases.append(("classify: ConnectionRefused（裸词）⇒ 可达", classify_reachable("ConnectionRefusedError") is True))
    cases.append(("classify: BLOCKED:ConnectionRefused ⇒ 可达（对方回了 RST）",
                  classify_reachable("BLOCKED:ConnectionRefusedError") is True))
    cases.append(("classify: BLOCKED:PermissionError ⇒ 被拦", classify_reachable("BLOCKED:PermissionError") is False))
    cases.append(("classify: 空串 ⇒ 被拦（不知道不许当做到）", classify_reachable("") is False))

    lo_red = claim_loopback({"unix_bind": "FAIL:PermissionError", "loopback": "FAIL:PermissionError"})
    cases.append(("断言 A 能红：unix/loopback 被拒 ⇒ FAIL", lo_red[0] is False))
    lo_green = claim_loopback({"unix_bind": "ok", "loopback": "ok"})
    cases.append(("断言 A 能绿：unix/loopback 都 ok ⇒ PASS", lo_green[0] is True))

    eg_red = claim_egress({"egress": "OPEN", "target": "t:1"}, {"egress": "OPEN", "target": "t:1"})
    cases.append(("断言 B 能红：封闭态仍可达 ⇒ FAIL", eg_red[0] is False))
    eg_green = claim_egress({"egress": "BLOCKED:PermissionError", "target": "t:1"},
                            {"egress": "OPEN", "target": "t:1"})
    cases.append(("断言 B 能绿：基线可达 + 封闭态被拦 ⇒ PASS", eg_green[0] is True))
    eg_nodisc = claim_egress({"egress": "BLOCKED:PermissionError", "target": "t:1"},
                             {"egress": "Timeout", "target": "t:1"})
    cases.append(("断言 B 无区分度：基线也不可达 ⇒ 既非红也非绿（rc=2 口径）", eg_nodisc[0] is None))

    try:
        require_probes("/nonexistent-probe-root", ["miniprobe.py"], platform)
        cases.append(("探针缺失必须硬失败", False))
    except HardFail:
        cases.append(("探针缺失必须硬失败", True))

    bad = [n for n, ok in cases if not ok]
    for n, ok in cases:
        print("    [%s] %s" % ("ok" if ok else "✗", n))
    return (len(bad) == 0, bad)


def mode_selftest(platform, as_json):
    print("  ① 护栏自检（合成输入）")
    ok_guard, bad_guard = selftest_guards(platform)
    if not ok_guard:
        print("  ✗ 护栏自检未过：%s ⇒ 本脚本自身的判定层不可信，先修它" % bad_guard)
        return 1

    print("  ② 四态成对（活体变异：把封闭件换成不该成立的形态，断言必须变红）")
    require_probes(PROBE_ROOT, REQUIRED[platform] + MUTATION_EXTRA[platform], platform)
    spec, baseline, _ = pick_target(platform, targets_from_env())
    target = parse_target(spec)
    print("     判别目标 %s（基线 %s）" % (spec, baseline.get("egress")))

    rows, bad = [], []
    for exp in EXPECT[platform]:
        inside = baseline if exp["profile"] is None else probe_suite(
            platform, prefix_for(platform, exp["profile"]), target)
        a_ok, a_note = claim_loopback(inside)
        if exp["profile"] is None:
            b_ok, b_note = (False, "基线即无封闭态：出网可达（%s）" % inside.get("egress"))
        else:
            b_ok, b_note = claim_egress(inside, baseline)
        jit = inside.get("wx_jit")
        good = (a_ok == exp["A"]) and (b_ok == exp["B"]) and (exp["jit"] is None or jit == exp["jit"])
        rows.append({"state": exp["name"], "A": a_ok, "B": b_ok, "jit": jit, "expect": exp, "ok": good,
                     "note": "%s | %s | JIT=%s" % (a_note, b_note, jit)})
        if not good:
            bad.append(exp["name"])

    for r in rows:
        print("    [%s] %-32s A=%s(期望%s) B=%s(期望%s) JIT=%s(期望%s) —— %s"
              % ("ok" if r["ok"] else "✗", r["state"], r["A"], r["expect"]["A"],
                 r["B"], r["expect"]["B"], r["jit"],
                 r["expect"]["jit"] if r["expect"]["jit"] else "(不约束)", r["expect"]["why"]))
    if as_json:
        print(json.dumps({"platform": platform, "target": spec, "states": rows}, ensure_ascii=False))
    if bad:
        print("  ✗ 有态与文档口径不符：%s ⇒ 要么环境变了、要么封闭件变了，需人看（不许按「大概没事」放过）"
              % "、".join(bad))
        return 1
    print("  ✓ 四态成对全部与文档口径一致：断言 A 与断言 B **各自都有红的版本**（判据 7 的「两态可验」）")
    return 0


def main():
    args = [a for a in sys.argv[1:]]
    as_json = "--json" in args
    if "-h" in args or "--help" in args:
        print(__doc__)
        return 0
    platform = sys.platform
    if platform.startswith("darwin"):
        platform = "darwin"
    elif platform.startswith("linux"):
        platform = "linux"
    else:
        print("✗ 本脚本只覆盖 darwin/linux（%s 无已实测的封闭配方：Windows AppContainer 属未验证 ✗）"
              % sys.platform)
        return 2
    try:
        if "--self-test" in args:
            return mode_selftest(platform, as_json)
        return mode_verify(platform, as_json)
    except HardFail as e:
        print("✗ 硬失败（rc=2，不是跳过）：%s" % e)
        return 2


if __name__ == "__main__":
    sys.exit(main())

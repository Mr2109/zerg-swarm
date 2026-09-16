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
  python3 scripts/sandbox-probes/verify-two-states.py --wall <茧壁二进制>
                                                                    # 同上，但**多跑一条茧壁路线**并与原生路线**对拍**
  python3 scripts/sandbox-probes/verify-two-states.py --self-test   # 变异验证：四态成对 + 护栏自检（+ 路线自检）
  python3 scripts/sandbox-probes/verify-two-states.py --json         # 同上，附机器可读输出
  TWO_STATE_TARGETS="<cluster-ip>:8580" python3 ... verify-two-states.py  # 覆盖判别目标（宿主无出网时必用）

**两条路线**（批 2'.5）：`--wall` 缺席时**行为与历史一致**（原生：macOS `sandbox-exec` + `pF.sb`；
Linux 自组 bwrap argv）。`--wall` 在时**多一条**：封闭态改由 `zerg-wall run --spec <配方>` 施加
（配方由本脚本按探针生成），并与原生「黄金配方」路线**逐项对拍**（A/B/JIT）——茧壁是同一落点的
第二份实现，它的验收就是「与基准一致」。差异**一律照实红**，包括已知差异（Linux 侧茧壁沿用现行
生产配方、缺 `--unshare-net`）——已知差异也不写成期望值刷绿（补它 = 改孵化行为，属待拍板口径）。
`--wall` 时茧壁**自己的留痕**必须在（计划 JSON + 施加的封闭）：缺 ⇒ 硬失败 ——
防的是最贵的一种假绿：路线被改坏后**静默回落**成原生执行，探针照样输出、断言照样全过。

平台：macOS 走 Seatbelt（`sandbox-exec` + 仓内 pF 黄金配方）；Linux 走 bwrap 全套 unshare。
本机（macOS）可跑 macOS 分支；**Linux 分支必须在 Linux 上跑一次才算验过**（脚本在非 Linux 上不假装验过）。
"""

import json
import os
import shutil
import subprocess
import sys

PROBE_ROOT = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(PROBE_ROOT))   # scripts/sandbox-probes → 仓根（回执卫生：路径显示用）
PY = sys.executable or "/usr/bin/python3"
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

# —— 茧壁路线（`--wall <zerg-wall>`，批 2'.5）：判据 7 的两态在茧壁下重跑一遍，与原生路线**对拍** ——
WALL_SPEC_DIR = "/private/tmp/zerg-two-states-wall"      # 配方落点（仓外，随手可清）
# macOS：茧壁策略的可写区 = 配方里的 `work_dir` ∪ 可写绑定的宿主侧。取 `pF.sb` 放行的**同一片**，
# 两条路线才是「同面」对拍（面不同就是比两个东西）。
DARWIN_WALL_WORK_DIR = "/private/tmp/ipc-test"
# Linux：茧壁不产 `--dir`（只有 `--tmpfs /tmp`）⇒ 工作目录取空间内**已存在**的 /tmp；
# 探针自己在里面造 /tmp/lxwork（原生路线那条 `--dir /tmp/lxwork` 是本脚本自己补的，茧壁**不该**补）。
LINUX_WALL_WORK_DIR = "/tmp"
# 茧壁**自己的留痕**（缺失 ⇒ 这条路线没真的经过茧壁 ⇒ 硬失败）：计划 JSON + 它施加的封闭
WALL_TRACE = {
    "darwin": ["茧壁计划（run）：", "茧壁策略（本进程直调 Seatbelt", "(deny default)"],
    "linux": ["茧壁计划（run）：", '"bwrap"'],
}

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


def require_probes(root, names, platform, route=None):
    missing = [n for n in names if not os.path.isfile(os.path.join(root, n))]
    if missing:
        raise HardFail("探针缺失或路径漂移：%s（在 %s 下）⇒ 判据 7 要求硬失败，不许静默跳过"
                       % (", ".join(missing), display_path(root)))
    binname = ENCLOSER.get(platform)
    if binname and not shutil.which(binname):
        raise HardFail("沙箱程序 %s 不在 PATH ⇒ 无法验「封闭」这一态（硬失败，不是跳过）" % binname)
    if route is not None:
        # 茧壁二进制不在 ⇒ 硬失败（**不替人编**：门禁里不做隐式构建 —— 与 wall-macos-evidence.py 同一口径）
        if not os.path.isfile(route):
            raise HardFail("茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`"
                           "（门禁不静默跳过，也不替人编）" % display_path(route))
        # 配方里的 engine_path_in_space 必须是**绝对路径**（茧壁侧 fail-closed 也会拒，这里给一句人话）
        if not os.path.isabs(PY):
            raise HardFail("解释器路径 %s 不是绝对路径 ⇒ 茧壁配方没法用它（硬失败）" % PY)


# ————————————————————————— 真跑 —————————————————————————

def display_path(p):
    """把路径显示成**仓内相对路径**（回执/错误串会进公开面：私有绝对路径不许出现）。

    只对绝对路径动手：`plan` / `--spec` 这类参数原样返回（对它们做 relpath 只会更乱）。
    """
    if not p.startswith("/"):
        return p
    try:
        r = os.path.relpath(p, REPO_ROOT)
    except ValueError:
        return p
    return r if not r.startswith("..") else p


def run_probe(argv, probe, env_extra, route=None, platform=None):
    env = dict(os.environ)
    env.update(env_extra)
    try:
        p = subprocess.run(argv, capture_output=True, text=True, timeout=RUN_TIMEOUT_S,
                           env=env, cwd=PROBE_ROOT)
    except subprocess.TimeoutExpired:
        raise ProbeError("超时（>%ss）：%s（命令：%s）"
                         % (RUN_TIMEOUT_S, probe, " ".join(display_path(a) for a in argv)))
    except OSError as e:
        raise ProbeError("起不来（%s）：%s ⇒ 硬失败（不是跳过）"
                         % (e, " ".join(display_path(a) for a in argv)))
    if p.returncode != 0:
        # 茧壁**拒绝**配方时 rc=2 ⇒ 这条路线根本没跑起来 ⇒ 硬失败：
        # 绝不许把「没跑起来」读成「已封闭」（那正是「看着正常、跑不起来」的形态）
        raise ProbeError("rc=%d：%s\n%s\n命令：%s" % (p.returncode, probe,
                        (p.stderr or "").strip()[:400],
                        " ".join(display_path(a) for a in argv)))
    if route is not None:
        require_wall_trace(platform, p.stderr)
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


def wall_spec(platform, probe, env_extra):
    """探针 → 茧壁配方（`spec.json`）—— 茧壁的**唯一输入**（它不消费第二套语义）。"""
    return {
        "egg_id": "probe-two-states-%s" % os.path.splitext(probe)[0].replace("_", "-"),
        "schema_version": 1,
        "engine_path_in_space": PY,
        "engine_args": [os.path.join(PROBE_ROOT, probe)],
        "weight_path": PROBE_ROOT,          # 只出证（茧壁的 validate 要求权重至少声明一处）
        "env": dict(env_extra),
        "work_dir": DARWIN_WALL_WORK_DIR if platform == "darwin" else LINUX_WALL_WORK_DIR,
    }


def write_wall_spec(platform, probe, env_extra):
    """配方落盘（仓外，随手可清）；返回路径。

    `env` **进配方**而不是只靠继承：这样走的是**生产同一条包装 exec 路径**（有 env ⇒ `/bin/sh -c`
    包装），且茧壁会把 `env:<名>` 如实列进 allowlist（放行面自报）。
    """
    os.makedirs(WALL_SPEC_DIR, exist_ok=True)
    path = os.path.join(WALL_SPEC_DIR, "spec-%s-%s.json" % (platform, os.path.splitext(probe)[0]))
    with open(path, "w", encoding="utf-8") as f:
        json.dump(wall_spec(platform, probe, env_extra), f, ensure_ascii=False, indent=2, sort_keys=True)
        f.write("\n")
    return path


def require_wall_trace(platform, stderr):
    """茧壁路线必须留下**茧壁自己**的痕（计划 JSON + 它施加的封闭）。

    这条断言防的是最贵的一种假绿：路线被改坏后**静默回落**成原生执行（甚至裸跑）——
    探针照样输出、断言照样全过，而「封闭由茧壁施加」这件事根本没发生。
    """
    missing = [t for t in WALL_TRACE[platform] if t not in (stderr or "")]
    if missing:
        raise ProbeError("茧壁路线没有茧壁自己的留痕（缺 %s）⇒ 这条路线**没有真的经过茧壁** ⇒ 硬失败"
                         % "、".join(repr(t) for t in missing))


def confined_argv(platform, profile, route, probe, env_extra):
    """「封闭态」下跑一支探针的完整命令行：原生路线（sandbox-exec / bwrap）或茧壁路线（`--wall`）。"""
    if route is None:
        return list(prefix_for(platform, profile)) + [PY, os.path.join(PROBE_ROOT, probe)]
    return [route, "run", "--spec", write_wall_spec(platform, probe, env_extra)]


def parse_target(spec):
    host, _, port = spec.partition(":")
    return host.strip(), int(port.strip())


def probe_suite(platform, target, profile=None, route=None):
    """跑一支平台对应探针组，归一成同一形状（unix_bind / loopback / egress / wx_jit / target）。

    `profile` 与 `route` **二选一**：`route=None` = 原生路线（`profile` = Seatbelt 配方名 / Linux 档位，
    None 表示**无封闭的基线**）；`route=<茧壁二进制>` = 茧壁路线（封闭由茧壁产）。
    两个同时给 ⇒ 当场拒：混着比就说不清「差异是谁造成的」。
    """
    if route is not None and profile is not None:
        raise HardFail("调用侧写错：profile=%r 与茧壁路线同时给了（两条路线的封闭面不许混）" % profile)
    host, port = target
    if platform == "darwin":
        env = {"PROBE_EGRESS_HOST": host, "PROBE_EGRESS_PORT": str(port)}
        mini = run_probe(confined_argv(platform, profile, route, "miniprobe.py", env),
                         "miniprobe.py", env, route, platform)
        conn = run_probe(confined_argv(platform, profile, route, "conn_jit.py", env),
                         "conn_jit.py", env, route, platform)
        return {"unix_bind": mini.get("unix_bind"), "tcp_bind": mini.get("tcp_bind"),
                "loopback": conn.get("loopback_connect"), "wx_jit": conn.get("wx_jit_call"),
                "egress": mini.get("egress"), "target": mini.get("egress_target")}
    env = {"PROBE_LAN_HOST": host, "PROBE_LAN_PORT": str(port)}
    lx = run_probe(confined_argv(platform, profile, route, "lx2-linux.py", env),
                   "lx2-linux.py", env, route, platform)
    conn = run_probe(confined_argv(platform, profile, route, "conn_jit.py", env),
                     "conn_jit.py", env, route, platform)
    return {"unix_bind": lx.get("unix_bind"), "tcp_bind": lx.get("tcp_loopback_bind"),
            "loopback": conn.get("loopback_connect"), "wx_jit": conn.get("wx_jit_call"),
            "egress": lx.get("lan_connect"), "target": lx.get("lan_target")}


def pick_target(platform, targets):
    """逐个跑**基线**，取第一个基线可达者；都不通 ⇒ 硬失败（无区分度）。"""
    tried = []
    for spec in targets:
        target = parse_target(spec)
        res = probe_suite(platform, target)
        tried.append((spec, res.get("egress")))
        if classify_reachable(res.get("egress")):
            return spec, res, tried
    raise HardFail("候选判别目标全都基线不可达 ⇒ 出网探针无区分度（%s）⇒ 用 TWO_STATE_TARGETS "
                   "指定一个本机可达的局域网目标再跑" % "; ".join("%s→%s" % t for t in tried))


def targets_from_env():
    raw = os.environ.get("TWO_STATE_TARGETS", "").strip()
    return [s.strip() for s in raw.split(",") if s.strip()] if raw else list(DEFAULT_TARGETS)


def verdict_cell(ok):
    """一行判词的取值格：True / False / None（无区分度 —— 既非红也非绿）。"""
    return "PASS" if ok is True else ("FAIL" if ok is False else "无区分度")


def two_state_tuple(a_ok, b_ok, inside):
    """一路的两态结果（A / B / JIT）—— 对拍按这三个字段逐项比。"""
    return {"A": a_ok, "B": b_ok, "jit": inside.get("wx_jit")}


def compare_two_routes(platform, native, wall):
    """对拍：茧壁路线的两态必须与**原生黄金配方**路线逐项一致。

    判据 7 的基准是原生路线（macOS `pF.sb` / Linux 全套 unshare）；茧壁是「同一落点的第二份实现」
    ⇒ 它的验收就是**与基准一致**。差异一律照实红 —— **已知差异也照实红**：Linux 侧茧壁沿用现行
    生产配方（只 `--unshare-pid`，缺 `--unshare-net`）⇒ 出网仍开，那条红是如实反映、不是门禁坏了；
    补它 = 改孵化行为，属待 Mr2109 拍板的口径（自主作业日志·等他拍板区第 8 条）。
    """
    diffs = []
    for key, name in (("A", "断言 A（空间内通信）"), ("B", "断言 B（出网被拦）"), ("jit", "JIT W^X")):
        if wall.get(key) != native.get(key):
            diffs.append("%s：原生 %s / 茧壁 %s" % (name, native.get(key), wall.get(key)))
    if not diffs:
        return True, "两路逐项一致（A=%s · B=%s · JIT=%s）" % (native["A"], native["B"], native["jit"])
    note = "差异：" + "；".join(diffs)
    if platform == "linux":
        note += ("。**已知缺口**：茧壁的 Linux 配方 = 现行生产配方（只 `--unshare-pid`，没有 "
                 "`--unshare-net`）⇒ 空间内出网仍开；补它 = 改孵化行为，等 Mr2109 拍板")
    else:
        note += ("。macOS 侧茧壁的策略 = `pF.sb` 黄金配方**逐条** ⇒ 出现差异就是策略被放宽或漏了一条，"
                 "不许当「没差」放过")
    return False, note


# ————————————————————————— 模式一：真验 —————————————————————————

def mode_verify(platform, as_json, route=None):
    require_probes(PROBE_ROOT, REQUIRED[platform], platform, route)
    spec, baseline, tried = pick_target(platform, targets_from_env())
    target = parse_target(spec)

    inside = probe_suite(platform, target, profile=("pF.sb" if platform == "darwin" else "L2"))
    a_ok, a_note = claim_loopback(inside)
    b_ok, b_note = claim_egress(inside, baseline)

    wall_ok, wall_note, wall_inside, w_ok = None, "", None, None
    if route is not None:
        wall_inside = probe_suite(platform, target, route=route)
        wa_ok, wa_note = claim_loopback(wall_inside)
        wb_ok, wb_note = claim_egress(wall_inside, baseline)
        wall_ok, wall_note = compare_two_routes(platform, two_state_tuple(a_ok, b_ok, inside),
                                               two_state_tuple(wa_ok, wb_ok, wall_inside))
        w_ok = (wa_ok, wa_note, wb_ok, wb_note)

    rows = [
        ("判别目标", spec, "基线实测 %s ⇒ %s" % (baseline.get("egress"),
                                            "可达（有区分度）" if classify_reachable(baseline.get("egress")) else "不可达")),
        ("断言 A 空间内通信可用", verdict_cell(a_ok), a_note),
        ("断言 B 出网被拦", verdict_cell(b_ok), b_note),
        ("（信息）JIT W^X", str(inside.get("wx_jit")), "§10.10：基线/封闭态都应 ok:42（判据 7 不含它）"),
    ]
    if route is not None:
        rows += [
            ("茧壁断言 A（信息）", verdict_cell(w_ok[0]), w_ok[1]),
            ("茧壁断言 B（信息）", verdict_cell(w_ok[2]), w_ok[3]),
            ("两路对拍（茧壁 vs 原生黄金配方）", "一致" if wall_ok else "不一致", wall_note),
        ]

    print("茧壁 · 判据 7 两态可验（平台 %s · %s）"
          % (platform, "原生 %s" % ENCLOSER.get(platform) if route is None
             else "原生 %s + 茧壁 %s" % (ENCLOSER.get(platform), display_path(route))))
    for name, val, note in rows:
        print("  %-26s %-8s %s" % (name, val, note))
    if len(tried) > 1:
        print("  （判别目标试探顺序：%s）" % "; ".join("%s→%s" % t for t in tried))

    if as_json:
        print(json.dumps({"platform": platform, "target": spec, "baseline": baseline,
                          "inside": inside, "claimA": a_ok, "claimB": b_ok,
                          "wall_route": route, "wall_inside": wall_inside,
                          "wallSameAsNative": wall_ok}, ensure_ascii=False))

    if b_ok is None:
        return 2
    return 0 if (a_ok and b_ok and wall_ok is not False) else 1


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


def selftest_route(platform, route):
    """路线自检（`--wall`）：把「这条路线自己会不会假绿」钉住。

    ③-a 配方被茧壁拒（陌生 `schema_version`）⇒ 路线必须**硬失败**（rc=2），
         绝不许把「没跑起来」当成「已封闭」；
    ③-b 生成的 **Linux 配方**茧壁必须接受（`plan --platform linux` 是纯函数 ⇒ 本机也能先验），
         并把「有没有 `--unshare-net`」当场打出来（那句是**已知缺口**的机器留痕，不是断言）。
    """
    cases = []
    os.makedirs(WALL_SPEC_DIR, exist_ok=True)
    bad = os.path.join(WALL_SPEC_DIR, "spec-route-selftest-reject.json")
    with open(bad, "w", encoding="utf-8") as f:
        json.dump({"egg_id": "route-selftest", "schema_version": 99,
                   "engine_path_in_space": PY, "weight_path": "/tmp"}, f)
    try:
        run_probe([route, "run", "--spec", bad], "route-selftest(坏配方)", {}, route, platform)
        cases.append(("③-a 茧壁拒绝配方时路线必须硬失败（不许当「已封闭」）", False))
    except HardFail:
        cases.append(("③-a 茧壁拒绝配方时路线必须硬失败（不许当「已封闭」）", True))

    spec_path = write_wall_spec("linux", "lx2-linux.py",
                               {"PROBE_LAN_HOST": "127.0.0.1", "PROBE_LAN_PORT": "1"})
    p = subprocess.run([route, "plan", "--spec", spec_path, "--platform", "linux"],
                       capture_output=True, text=True)
    argv, ok = [], p.returncode == 0
    if ok:
        try:
            argv = json.loads(p.stdout.strip()).get("argv") or []
            ok = bool(argv) and argv[0] == "bwrap"
        except ValueError:
            ok = False
    if not ok:
        print("     茧壁 stderr：%s" % (p.stderr or "").strip()[:300])
    cases.append(("③-b 生成的 Linux 配方茧壁必须接受（且 argv[0]=bwrap）", ok))
    if ok:
        print("     现状（信息，非断言）：Linux 配方的 argv %s `--unshare-net` —— 缺它就是出网仍开，"
              "这是**已知缺口**（现行生产配方同形，待拍板第 8 条）"
              % ("有" if "--unshare-net" in argv else "没有"))
    for n, okk in cases:
        print("    [%s] %s" % ("ok" if okk else "✗", n))
    return all(okk for _, okk in cases), [n for n, okk in cases if not okk]


def mode_selftest(platform, as_json, route=None):
    print("  ① 护栏自检（合成输入）")
    ok_guard, bad_guard = selftest_guards(platform)
    if not ok_guard:
        print("  ✗ 护栏自检未过：%s ⇒ 本脚本自身的判定层不可信，先修它" % bad_guard)
        return 1

    print("  ② 四态成对（活体变异：把封闭件换成不该成立的形态，断言必须变红）")
    require_probes(PROBE_ROOT, REQUIRED[platform] + MUTATION_EXTRA[platform], platform, route)
    spec, baseline, _ = pick_target(platform, targets_from_env())
    target = parse_target(spec)
    print("     判别目标 %s（基线 %s）" % (spec, baseline.get("egress")))

    rows, bad = [], []
    for exp in EXPECT[platform]:
        inside = baseline if exp["profile"] is None else probe_suite(
            platform, target, profile=exp["profile"])
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
    if route is not None:
        wall_inside = probe_suite(platform, target, route=route)
        wa_ok, wa_note = claim_loopback(wall_inside)
        wb_ok, wb_note = claim_egress(wall_inside, baseline)
        ref = [r for r in rows if r["expect"]["profile"] == ("pF.sb" if platform == "darwin" else "L2")]
        if not ref:
            print("  ✗ 表里找不到原生黄金配方那一行 ⇒ 对拍没有基准（自检不过）")
            return 1
        ref = ref[0]
        ok, note = compare_two_routes(platform, {"A": ref["A"], "B": ref["B"], "jit": ref["jit"]},
                                      two_state_tuple(wa_ok, wb_ok, wall_inside))
        print("    [%s] %-32s A=%s B=%s JIT=%s —— %s"
              % ("ok" if ok else "✗", "茧壁（--wall，与原生对拍）", wa_ok, wb_ok,
                 wall_inside.get("wx_jit"), note if not ok else note))
        if not ok:
            bad.append("茧壁路线与原生黄金配方对拍不一致")

    if as_json:
        print(json.dumps({"platform": platform, "target": spec, "states": rows}, ensure_ascii=False))
    if bad:
        print("  ✗ 有态与文档口径不符：%s ⇒ 要么环境变了、要么封闭件变了，需人看（不许按「大概没事」放过）"
              % "、".join(bad))
        return 1
    if route is not None:
        print("  ③ 路线自检（--wall）")
        ok_route, bad_route = selftest_route(platform, route)
        if not ok_route:
            print("  ✗ 路线自检不过：%s" % bad_route)
            return 1
    print("  ✓ 四态成对全部与文档口径一致：断言 A 与断言 B **各自都有红的版本**（判据 7 的「两态可验」）")
    return 0


def parse_args(argv):
    """严格解析：**认不得的参数一律拒**（静默忽略参数会让「跑错路线」看不出来）。

    返回 `(mode, route, selftest, as_json)`；`mode` = `"run"` / `"help"`。
    """
    route, selftest, as_json = None, False, False
    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("-h", "--help"):
            return ("help", None, False, False)
        if a == "--self-test":
            selftest = True
        elif a == "--json":
            as_json = True
        elif a == "--wall" or a.startswith("--wall="):
            if route is not None:
                raise HardFail("--wall 给了两次——拒（说不清用哪一个）")
            if a == "--wall":
                i += 1
                if i >= len(argv):
                    raise HardFail("--wall 后面缺值——拒（要给茧壁二进制路径）")
                route = argv[i]
            else:
                route = a.split("=", 1)[1]
            if not route.strip():
                raise HardFail("--wall 的值是空串——拒")
        else:
            raise HardFail("认不得的参数 %r ——拒（不许静默忽略；用法见 --help）" % a)
        i += 1
    return ("run", route, selftest, as_json)


def main():
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
        mode, route, selftest, as_json = parse_args(sys.argv[1:])
        if mode == "help":
            print(__doc__)
            return 0
        if route is not None:
            # 二进制路径一律先**绝对化**：探针是带 `cwd=PROBE_ROOT` 起的（原生路线与茧壁路线都一样），
            # 相对路径在那一刻就失效（实测：`--wall wall/target/debug/zerg-wall` 会以「起不来」收场）。
            route = os.path.abspath(route)
        if selftest:
            return mode_selftest(platform, as_json, route)
        return mode_verify(platform, as_json, route)
    except HardFail as e:
        print("✗ 硬失败（rc=2，不是跳过）：%s" % e)
        return 2


if __name__ == "__main__":
    sys.exit(main())

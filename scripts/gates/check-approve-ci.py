#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-ci.py v1.0.0 —— 「**CI 永不人签**」口径的判据件（2.5.11 人签 F 批 · `F3`）

为什么存在（`F3` 的任务原文 + `v1.3 §2 S3`）
--------------------------------------------------
病灶（`v1.3 §2 S3` · `M-14`）：本仓**没有一句**写「CI 永不人签」✗ ⇒ 一旦有人在 CI 里补一枚钥匙，
**口径是空的**（判据无处可依）。本件把这条口径落成**可判的格**，并把它与**成文口径**绑在一起
（口径件不在 ⇒ 也不出结论）。

★ 本件**只判、不签、不写**：跑命令面看**失败面**、读仓外口径件、读 CI 面；**一个字节都不写**
✗（不签字、不落件、不在 CI 面落审计、不动状态目录）。

判据（C0–C5 · 每条都能单独失败）
--------------------------------------------------
  C0 **本机器件面读数**（`security list-smartcards` · `/dev/hidraw*` · `~/.ssh/*.pub` 里 `sk-` 命中）——
     只作读数与 C2 的**前置条件**（`v1.3 [P11]`：本机**零器件** ⇒ 那条 rc 只证明**失败面**）。
  C1 **无 TTY ⇒ 拒**：`approve new --tool terminal --by <名> --note <理由>` 与 `approve keygen --by <名>`
     各自 **rc=2 且 stdout 0 字节**（`M-14` 前半 · 沿用今天两条 must-fail 的期望值，**一条不改**）；
     同批断言**真状态目录零写**（跑前跑后 `approvals/` 的文件名 + sha256 逐字节相同）。
  C2 **无器件 ⇒ 立即失败、不挂死**（**判据带时限**；不带时限就判不出「挂死」）：
     · CLI 面：`approve keygen --strength token`（token 档旗标）⇒ **rc=2 且 stdout 0 字节**
       —— 今天这档在命令面上**还不存在**（`I-4`：名字还没被占）⇒ 读数如实写「CLI 面无 token 档路径」；
     · 器件面（**本机零器件时**才判）：`ssh-keygen -t ed25519-sk -O verify-required` 那一路 ⇒
       **在时限内退出**且 **rc≠0** 且**没落件**（`v1.3 [P11]` 的实测形态：`rc=255`）⇒
       超时 = **判红（挂死）**。★ 本机**有**器件 ⇒ 这一格**只交代不判**（对象不成立），**不算绿**。
  C3 **成文口径在位**：仓外开发文档面里那枚口径件必须在，且逐字带四句：
     「CI 永不人签」· 无 TTY ⇒ rc=2 · 无器件 ⇒ 立即失败不挂死 · CI 的正路 = **人先签好一枚件**
     （**不上 CI 钥匙**）；缺件/缺句 ⇒ **不给结论**（rc=2）。★ 读不到仓外根 ⇒ rc=2（同门⑮ 口径）。
  C4 **CI 面不许有能签的钥匙**（红线）：`publish/ci/*.yml` 里出现 `zerg approve new` /
     `zerg approve keygen` / `operator.key` ⇒ **判红**（给了 CI 能签的东西）。
  C5 **不可机检（如实标 · 不许写成命令判据）**：「CI 侧有没有偷偷放审批私钥 / 长期令牌」（`H-9`）
     只能**人复看** ⇒ 本件只**打印**这一条，**不判**。

★★ 本件**不证明**任何**在场**闸（这条写在判词里，防止被读成「档已生效」）：
   「无器件 ⇒ rc≠0」只证明**失败面**；今天本机零器件 ⇒ 它**不证明**「模型签不出来」。
   另：`BLOCKED`（rc=2 不给结论）**不等于**「拒」—— 两者**不许**混。

退码
--------------------------------------------------
  0 = 全绿 · 1 = 有红（rc 不是 2 / stdout 非空 / 真状态目录被写 / 挂死 / CI 面有能签的钥匙）
      · 2 = **不给结论**（缺 `bin/zerg` · 读不到口径件/仓外根 —— 缺件 ≠ 绿）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-ci.py                # 先自检，再扫真目标
  python3 scripts/gates/check-approve-ci.py --self-test    # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-ci.py --root <仓根> --state <状态目录> [--timeout 秒]
"""

import argparse
import glob
import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-ci.py v1.0.0"

CI_FACE = (os.path.join("publish", "ci", "ci.yml"), os.path.join("publish", "ci", "release-agent.yml"))
CI_FORBIDDEN = ("zerg approve new", "zerg approve keygen", "approve keygen --", "operator.key")
DEVDOCS_TAIL = os.path.join("项目文档", "v2.5.11")
CI_口径件 = "口径-CI永不人签-20260923.md"
# 成文口径里**必须逐字出现**的四句（机检用最小片段 · 见头部 C3）
口径_MUST = ("CI 永不人签", "rc=2", "stdout 0 字节", "立即失败、不挂死", "人先签好", "不上 CI 钥匙")
NO_TTY_CASES = (
    ("approve new", ["approve", "new", "--tool", "terminal", "--by", "<名>", "--note", "<理由>"]),
    ("approve keygen", ["approve", "keygen", "--by", "<名>"]),
)


# ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────────

def judge_no_tty(rc, out_bytes):
    """C1：无 TTY ⇒ rc=2 且 stdout 0 字节（期望值一条不改）。"""
    problems = []
    if rc != 2:
        problems.append("C1 破：无 TTY 下 rc=%s ≠ 2（期望值 = 2，**一条不改**）" % rc)
    if len(out_bytes) != 0:
        problems.append("C1 破：无 TTY 下 stdout %d 字节 ≠ 0（人签面不许往 stdout 打东西）" % len(out_bytes))
    return problems


def judge_device_path(rc, timed_out, landed, has_device):
    """C2 器件面：本机**零器件**时判（时限内 rc≠0 且没落件）；有器件 ⇒ 只交代不判。"""
    if has_device:
        return [], "本机**有**器件 ⇒ 这一格的对象不成立 ⇒ **只交代不判**（不算绿）"
    problems = []
    if timed_out:
        problems.append("C2 破：器件路径**超时没退出** ⇒ 判红（「挂死」这一格就是它）")
    elif rc == 0:
        problems.append("C2 破：无器件却 rc=0（该立即失败不挂死）")
    if landed:
        problems.append("C2 破：**落了件**（无器件那一路不该落下任何件）")
    return problems, "本机零器件 ⇒ 判：时限内退出 + rc≠0 + 没落件"


def judge_口径(text):
    """C3：成文口径四句必须逐字在。"""
    miss = [s for s in 口径_MUST if s not in text]
    if miss:
        return ["C3 破：口径件缺句 %s（口径没落成文 = 判据无处可依）" % "、".join("「%s」" % m for m in miss)]
    return []


def judge_ci_face(texts):
    """C4：CI 面不许有能签的钥匙。"""
    problems = []
    for rel, txt in texts:
        for pat in CI_FORBIDDEN:
            if pat in txt:
                problems.append("C4 破：CI 面 `%s` 里出现 `%s` ⇒ 给了 CI 能签的东西（红线）" % (rel, pat))
    return problems


# ── 取数（只读 · 只跑失败面）──────────────────────────────────────────────────────

def state_dir_of(given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_STATE_DIR")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def read_installed_key_id(state):
    """只读在册公钥的 `key_id`（`E5` 的 `--confirm` 要逐字点名它）。"""
    p = os.path.join(state, "approvals", "operator.pub")
    try:
        with open(p, encoding="utf-8") as f:
            return json.load(f).get("key_id") or ""
    except Exception:  # noqa: BLE001 —— 读不出 ⇒ 那一格只交代，不当红
        return ""


def snapshot_approvals(state):
    """真状态目录的 `approvals/` 快照（文件名 + sha256）⇒ 跑前跑后逐字节比对。"""
    d = os.path.join(state, "approvals")
    out = {}
    if not os.path.isdir(d):
        return out
    for name in sorted(os.listdir(d)):
        p = os.path.join(d, name)
        if not os.path.isfile(p):
            continue
        with open(p, "rb") as f:
            out[name] = hashlib.sha256(f.read()).hexdigest()
    return out


def run_cmd(argv, state, timeout=60, stdin_null=True):
    """跑一条命令面（**不接管道** · rc 取真退出码）⇒ (rc, stdout bytes, stderr text, 超时)。"""
    env = dict(os.environ)
    env["ZERG_STATE_DIR"] = state
    kw = {"stdout": subprocess.PIPE, "stderr": subprocess.PIPE, "cwd": None, "env": env}
    if stdin_null:
        kw["stdin"] = subprocess.DEVNULL
    try:
        p = subprocess.run(argv, timeout=timeout, **kw)
        return p.returncode, p.stdout, p.stderr.decode("utf-8", "replace"), False
    except subprocess.TimeoutExpired as exc:
        return None, exc.stdout or b"", "（超时被强杀）", True


def has_device():
    """本机器件面读数（`security list-smartcards` · `/dev/hidraw*` · `~/.ssh/*.pub` 里 `sk-`）。"""
    detail = []
    smart = "（`security` 不在 PATH / 非 macOS）"
    try:
        p = subprocess.run(["security", "list-smartcards"], stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, timeout=60)
        smart = (p.stdout + p.stderr).decode("utf-8", "replace").strip().splitlines()
        smart = smart[-1] if smart else "（无输出）"
    except Exception as exc:  # noqa: BLE001
        smart = "（跑不起来：%s）" % exc
    detail.append("`security list-smartcards` ⇒ %s" % smart)
    hid = sorted(glob.glob("/dev/hidraw*"))
    detail.append("`/dev/hidraw*` ⇒ %d 个" % len(hid))
    sk = 0
    for p in sorted(glob.glob(os.path.join(os.path.expanduser("~"), ".ssh", "*.pub"))):
        try:
            with open(p, encoding="utf-8", errors="replace") as f:
                if "sk-" in f.read():
                    sk += 1
        except Exception:  # noqa: BLE001
            pass
    detail.append("`~/.ssh/*.pub` 里 `sk-` 命中 ⇒ %d" % sk)
    return (len(hid) > 0 or sk > 0), " · ".join(detail)


def devdocs_root_of(root, given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_DEVDOCS_ROOT")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.dirname(root), "Zerg-内部文档", DEVDOCS_TAIL)


# ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

def self_test():
    ok = bad = 0

    def check(cond, msg_ok, msg_bad):
        nonlocal ok, bad
        if cond:
            print("   ✓ %s" % msg_ok)
            ok += 1
        else:
            print("   ✗ %s" % msg_bad)
            bad += 1

    check(not judge_no_tty(2, b""), "① 正控：rc=2 且 stdout 0 字节 ⇒ 不判红（C1 的期望形态）",
          "① 正控失败：期望形态被判红")
    check(bool(judge_no_tty(0, b"")), "② 负控：rc=0 ⇒ 判红（无 TTY 竟然签得出来）",
          "② 负控失败：rc=0 还判绿")
    check(bool(judge_no_tty(2, "签好了".encode("utf-8"))),
          "③ 负控：stdout 非空 ⇒ 判红（判据是 0 字节）",
          "③ 负控失败：stdout 非空还判绿")
    check(bool(judge_no_tty(1, b"")), "④ 负控：rc=1（改了退码）⇒ 判红（期望值一条不改）",
          "④ 负控失败：退码被改还判绿")

    p, w = judge_device_path(255, False, False, False)
    check(not p, "⑤ 正控：时限内 rc=255 且没落件 ⇒ 不判红（`[P11]` 的实测形态）",
          "⑤ 正控失败：实测形态被判红")
    p2, w2 = judge_device_path(None, True, False, False)
    check(bool(p2), "⑥ 负控：**超时被强杀** ⇒ 判红（判据带时限就是为了这一格）",
          "⑥ 负控失败：挂死还判绿")
    p3, w3 = judge_device_path(0, False, False, False)
    check(bool(p3), "⑦ 负控：无器件却 rc=0 ⇒ 判红", "⑦ 负控失败：rc=0 还判绿")
    p4, w4 = judge_device_path(255, False, True, False)
    check(bool(p4), "⑧ 负控：落了件 ⇒ 判红（无器件那一路不许落件）", "⑧ 负控失败：落件还判绿")
    p5, w5 = judge_device_path(0, False, False, True)
    check(not p5 and "不判" in w5, "⑨ 正控：本机**有器件** ⇒ 只交代不判（对象不成立 ≠ 绿）",
          "⑨ 正控失败：有器件时还硬判")

    good = "".join(口径_MUST)
    check(not judge_口径(good), "⑩ 正控：口径四句齐 ⇒ 不判红（C3）", "⑩ 正控失败：齐了还判红")
    for i, s in enumerate(口径_MUST):
        check(bool(judge_口径(good.replace(s, ""))),
              "⑪-%d 负控：抹掉「%s」⇒ 判红（口径缺句 = 判据无处可依）" % (i + 1, s),
              "⑪-%d 负控失败：缺了「%s」还判绿" % (i + 1, s))

    check(not judge_ci_face([("publish/ci/ci.yml", "# 只跑构建与测试\nzerg gate run --fast\n")]),
          "⑫ 正控：CI 面没有能签的动作/钥匙 ⇒ 不判红（C4）", "⑫ 正控失败：干净还判红")
    for i, pat in enumerate(CI_FORBIDDEN):
        check(bool(judge_ci_face([("publish/ci/ci.yml", "run: %s\n" % pat)])),
              "⑬-%d 负控：CI 面出现 `%s` ⇒ 判红（红线）" % (i + 1, pat),
              "⑬-%d 负控失败：%s 出现还判绿" % (i + 1, pat))

    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节 · 未签任何件）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(root, state, devdocs, timeout_s):
    reds, nocalls = [], []
    zerg = os.path.join(root, "bin", "zerg")
    if not os.path.exists(zerg):
        print("⛔ 未取数：`bin/zerg` 不在（%s）⇒ **不出结论**（rc=2）—— 制品不入库，"
              "请先走自举档 `zerg build all --only cli`" % zerg)
        return 2

    print("── C0 本机器件面读数（只读）")
    dev, dev_detail = has_device()
    print("   %s ⇒ 结论：%s" % (dev_detail, "**有器件**" if dev else "**零器件**（同 `[P11]`）"))

    print("── C1 无 TTY ⇒ 拒（真状态目录 = %s）" % state)
    before = snapshot_approvals(state)
    cases = list(NO_TTY_CASES)
    # 第三个夹具走「**已经存在在册钥**」那条路（`E5`：换钥要 `--confirm=<旧 key_id>` + `--yes`）——
    # 它才真正打到「**非交互一律蒙不过**」那句上（前两条在别的理由上也会 rc=2 ⇒ 区分度不够）。
    kid = read_installed_key_id(state)
    if kid:
        cases.append(("approve keygen --confirm=%s --yes" % kid,
                      ["approve", "keygen", "--by", "<名>", "--confirm=%s" % kid, "--yes"]))
    else:
        print("   · 交代：在册公钥读不出 `key_id` ⇒ 第三个夹具（`--confirm=<旧 key_id> --yes`）本跑跳过")
    for name, argv in cases:
        rc, out, err, to = run_cmd([zerg] + argv, state, timeout=timeout_s)
        print("   · `zerg %s </dev/null` ⇒ rc=%s · stdout **%d 字节** · stderr 首行 = %s"
              % (name, rc, len(out), (err.strip().splitlines() or ["（空）"])[0]))
        reds += judge_no_tty(rc, out)
        reds += (["C1 破：`zerg %s` **超时没退出**（无 TTY 该立即拒）" % name] if to else [])
    after = snapshot_approvals(state)
    if before != after:
        reds.append("C1 破：**真状态目录被写了**（跑前 %d 件 → 跑后 %d 件；被拒的命令面不许落件）"
                    % (len(before), len(after)))
    else:
        print("   · 真状态目录零写（`approvals/` 跑前跑后 %d 件逐字节相同）" % len(before))

    print("── C2 无器件 ⇒ 立即失败、不挂死（时限 = %ds）" % timeout_s)
    rc, out, err, to = run_cmd([zerg, "approve", "keygen", "--strength", "token", "--by", "<名>"],
                               state, timeout=timeout_s)
    print("   · CLI 面 token 档旗标：`zerg approve keygen --strength token` ⇒ rc=%s · stdout %d 字节 · "
          "stderr 首行 = %s" % (rc, len(out), (err.strip().splitlines() or ["（空）"])[0]))
    if rc == 2 and len(out) == 0:
        print("   · 口径：今天这档在**命令面**上还不存在（`I-4`：名字还没被占）⇒ CLI 面无 token 档路径 ⇒ "
              "器件面那一路才是今天**唯一能跑到的** token 档路径")
    else:
        reds.append("C2 破：CLI 面 token 档旗标 rc=%s / stdout %d 字节（期望 rc=2 且 0 字节）" % (rc, len(out)))
    sand = tempfile.mkdtemp(prefix="approve-ci-")
    keyf = os.path.join(sand, "probe_sk")
    rc2, out2, err2, to2 = run_cmd(["ssh-keygen", "-t", "ed25519-sk", "-O", "verify-required",
                                    "-N", "", "-C", "probe", "-f", keyf], state, timeout=timeout_s)
    landed = os.path.exists(keyf) or os.path.exists(keyf + ".pub")
    print("   · 器件面（`ssh-keygen -t ed25519-sk -O verify-required` · 沙箱 %s）⇒ rc=%s · 超时=%s · "
          "落件=%s · stdout 首行 = %s · stderr 首行 = %s"
          % (sand, rc2, to2, landed, (out2.decode("utf-8", "replace").strip().splitlines() or ["（空）"])[0],
             (err2.strip().splitlines() or ["（空）"])[0]))
    p2, why2 = judge_device_path(rc2, to2, landed, dev)
    reds += p2
    print("   · 判定口径：%s" % why2)

    print("── C3 成文口径在位（仓外开发文档面 = %s）" % devdocs)
    doc = os.path.join(devdocs, CI_口径件)
    if not os.path.isdir(devdocs) or not os.path.exists(doc):
        nocalls.append("C3 **未取数**：口径件不在（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿" % doc)
    else:
        with open(doc, encoding="utf-8") as f:
            txt = f.read()
        print("   · 口径件 %s（%d 字节）" % (CI_口径件, len(txt.encode())))
        reds += judge_口径(txt)

    print("── C4 CI 面不许有能签的钥匙（红线）")
    texts = []
    for rel in CI_FACE:
        full = os.path.join(root, rel)
        if os.path.exists(full):
            with open(full, encoding="utf-8", errors="replace") as f:
                texts.append((rel, f.read()))
    print("   · 读到的 CI 面件：%s" % (", ".join(r for r, _ in texts) or "（0 件）"))
    reds += judge_ci_face(texts)

    print("── C5 不可机检（如实标 · 不许写成命令判据）")
    print("   · 「CI 侧有没有偷偷放审批私钥 / 长期令牌」（`H-9`）只能**人复看** ⇒ **本件不判** "
          "（判了就是假闸）")
    print("   · 「无器件 ⇒ rc≠0」**只证明失败面**，**不证明**任何**在场**闸；"
          "`BLOCKED`（rc=2 不给结论）**不等于**「拒」—— 两者不许混")

    if reds or nocalls:
        rc_out = 1 if reds else 2
        print("⇒ 门禁%s（rc=%d）：红 %d 条 · 不给结论 %d 条"
              % ("红" if rc_out == 1 else "不给结论", rc_out, len(reds), len(nocalls)))
        for p in reds:
            print("   ✗ %s" % p)
        for p in nocalls:
            print("   ⛔ %s" % p)
        return rc_out
    print("⇒ 全绿（rc=0）：无 TTY 两条 rc=2 且 stdout 0 字节 · 真状态目录零写 · "
          "无器件那一路立即失败不挂死 · 成文口径在位 · CI 面无能签的钥匙")
    return 0


def main():
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--state", default=None, help="状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）")
    ap.add_argument("--devdocs", default=None, help="开发文档面根（默认 = `ZERG_DEVDOCS_ROOT` > 仓同级 `Zerg-内部文档/项目文档/v2.5.11`）")
    ap.add_argument("--timeout", type=int, default=60, help="每一格的时限（秒 · 默认 60）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    root = os.path.abspath(args.root)
    state = state_dir_of(args.state)
    devdocs = devdocs_root_of(root, args.devdocs)
    print(VERSION)
    if not args.no_self_test and not self_test():
        return 1
    if args.self_test:
        return 0
    print("── 真目标：%s（状态目录 %s）" % (root, state))
    if args.state is None and os.environ.get("ZERG_STATE_DIR"):
        print("★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"
              "**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）"
              % os.environ["ZERG_STATE_DIR"])
    return scan(root, state, devdocs, args.timeout)


if __name__ == "__main__":
    sys.exit(main())

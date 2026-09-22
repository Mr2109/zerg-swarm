#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-approve-closedset.py v1.0.0 —— 档位闭集与契约槽的**只登记**对拍件（2.5.11 人签 F 批 · `F4`）

为什么存在（`F4` 的任务原文 · 护栏任务）
--------------------------------------------------
病灶（`v1.3 §9.3 I-7` · §⑥ `I4`）：契约 `core/internal/contract/registry.json` 今天**没有** `sig_alg`
这一格 ⇒ 将来的算法档扩充是**新增条目**（不是复用一条）；而方向是**单向**的（删档 = 旧件永远验不了）。
本任务**只登记、不新增** ⇒ 需要一枚对拍件保证「**本批没有人偷偷新增 `sig_alg` 条目**」。

★ 本件**只判、只登记**：读契约、读仓外登记表、读两器口径；**一个字节都不写** ✗
（不改 `registry.json` 任何既有条目、不新增 `sig_alg` 条目、不占任何预留旗标名）。

判据（S1–S5 · 每条都能单独失败）
--------------------------------------------------
  S1 **闭集没被顺手扩**（`registry.json` 与批前**逐字相同**的四格读数）：
     顶层键**逐字且按序** = `schema/note/authority/entries/change_flow`（**5** 键）·
     `entries` **13** 条 · 含 `sig_alg` **0** 次 · 含 `approve` **1** 次。
     任一格变了 ⇒ **判红**（要扩闭集 = **另一次有闸的动作**，先拍再改，**不是顺手**）。
     ★ 整件 `sha256` **只印不判**（无关的**合法**改动 —— 例如 `note` 的字 —— 不该变假红）。
  S2 **单向铁律登记在位**：仓外登记表必须逐字带 `v1.3 §⑥ I4` 那一句
     「只许加、不许减：一旦有件按新档签过，删掉这一档的代码 = 那些件永远验不了」，
     且带**状态词**「未落地」与「名字还没被占」（**不许**把「登记」写成「已实现」）。
     缺件/缺句 ⇒ **不给结论**（rc=2）。
  S3 **`M-20` 仍成立**：`approve keygen --strength token --by <名>` ⇒ **rc=2**
     （`I-4` 的现跑依据：**旗标名还没被占**）。跑在**沙箱**状态目录 ⇒ 真状态目录零写。
  S4 **不许占预留旗标名**：`--token` / `--rekey` 也各自 ⇒ **rc=2**（三个名字一个都没被占）。
  S5 **公开面零改**（`F4` 判据④ 点名的那条命令）：`scripts/gates/check-publish-face-sync.py`
     ⇒ **rc=0**（两器口径一致 + 表↔两器无漂移）。★ 口径：「零改」指的是**生效面的判定口径零漂移**，
     **不是**「七件一个字节都不许动」—— 新门禁件按名登记进 `EXCLUDES`/`DROP_EXACT`/标记表
     这一条**接线**是**必须做**的（形同 D/E 两批：只**增行**，既有件一行不改）。

退码
--------------------------------------------------
  0 = 全绿 · 1 = 有红（闭集被扩 / 铁律登记缺句 / 旗标名被占 / 公开面漂移）
      · 2 = **不给结论**（契约读不到 · 登记表读不到 · 缺 `bin/zerg` 或那份同步门脚本）

用法
--------------------------------------------------
  python3 scripts/gates/check-approve-closedset.py                # 先自检，再扫真目标
  python3 scripts/gates/check-approve-closedset.py --self-test    # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-approve-closedset.py --root <仓根> --devdocs <开发文档面> [--static-only]
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-approve-closedset.py v1.0.0"

REGISTRY_REL = os.path.join("core", "internal", "contract", "registry.json")
DEVDOCS_TAIL = os.path.join("项目文档", "v2.5.11")
登记表件 = "登记表-档位闭集与契约槽-20260923.md"
SYNC_GATE_REL = os.path.join("scripts", "gates", "check-publish-face-sync.py")

TOP_KEYS = ["schema", "note", "authority", "entries", "change_flow"]     # 批前现读（5 键 · 按序）
ENTRIES_N = 13                                                          # 批前现读
SIG_ALG_N = 0                                                           # ★ 必须保持 0
APPROVE_N = 1                                                           # 批前现读
# `v1.3 §⑥ I4` 逐字（登记表必须带这一句 —— 一个标点都不许改）
I4_VERBATIM = "只许加、不许减：一旦有件按新档签过，删掉这一档的代码 = 那些件永远验不了"
登记表_MUST = (I4_VERBATIM, "未落地", "名字还没被占")
RESERVED_FLAGS = ("--strength", "--token", "--rekey")                   # 预留旗标名（`P27` · 一个都不许占）


# ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────────

def read_registry(raw):
    try:
        d = json.loads(raw.decode("utf-8"))
    except Exception as exc:  # noqa: BLE001
        return None, "契约不是合法 JSON（%s）" % exc
    if not isinstance(d, dict):
        return None, "契约不是 JSON 对象"
    return d, ""


def judge_closedset(d, text):
    """S1：四格读数 == 批前登记值。"""
    problems = []
    keys = list(d.keys())
    if keys != TOP_KEYS:
        problems.append("S1 破：顶层键 = %s ≠ 批前 5 键 %s（顺序也照批前那一份）"
                        % (",".join(keys), ",".join(TOP_KEYS)))
    ents = d.get("entries")
    if not isinstance(ents, list) or len(ents) != ENTRIES_N:
        problems.append("S1 破：`entries` = %s 条 ≠ 批前 %d 条（闭集被顺手扩/缩）"
                        % (len(ents) if isinstance(ents, list) else "（不是数组）", ENTRIES_N))
    n_sig = text.count("sig_alg")
    if n_sig != SIG_ALG_N:
        problems.append("S1 破：含 `sig_alg` **%d** 次 ≠ **%d**（`I-7`：那是**新增条目**，"
                        "先拍再改 ⇒ 顺手加进来就是越线）" % (n_sig, SIG_ALG_N))
    n_app = text.count("approve")
    if n_app != APPROVE_N:
        problems.append("S1 破：含 `approve` %d 次 ≠ 批前 %d 次" % (n_app, APPROVE_N))
    return problems


def judge_ledger(text):
    """S2：登记表必须逐字带 I4 那一句 + 两个状态词（不许把「登记」写成「已实现」）。"""
    miss = [s for s in 登记表_MUST if s not in text]
    if miss:
        return ["S2 破：登记表缺 %s（缺件/缺句 ⇒ 不就是**登记**；`R10`：不许把预留接口读成「已经升过了」）"
                % "、".join("「%s」" % m for m in miss)]
    return []


def judge_flag_not_taken(name, rc, out_len):
    """S3/S4：预留旗标名还没被占 ⇒ rc=2 且 stdout 0 字节。"""
    problems = []
    if rc != 2:
        problems.append("S%s 破：`%s` ⇒ rc=%s ≠ 2（名字**已经被占了**？占名 = 把接口面形态先定死 ⇒ "
                        "随接口一起拍，不许顺手占）" % ("3" if name == "--strength" else "4", name, rc))
    if out_len != 0:
        problems.append("S%s 破：`%s` ⇒ stdout %d 字节 ≠ 0" % ("3" if name == "--strength" else "4",
                                                              name, out_len))
    return problems


def judge_sync(rc, tail):
    """S5：公开面零改 = 那份同步门 rc=0。"""
    if rc != 0:
        return ["S5 破：`%s` rc=%s ≠ 0（公开面**有漂移**）—— stderr 末行：%s"
                % (SYNC_GATE_REL, rc, tail)]
    return []


# ── 取数（只读）───────────────────────────────────────────────────────────────────

def state_dir_of(given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_STATE_DIR")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def devdocs_root_of(root, given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_DEVDOCS_ROOT")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.dirname(root), "Zerg-内部文档", DEVDOCS_TAIL)


def run_cmd(argv, state, timeout=60):
    env = dict(os.environ)
    env["ZERG_STATE_DIR"] = state
    try:
        p = subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           stdin=subprocess.DEVNULL, env=env, timeout=timeout)
        return p.returncode, p.stdout, p.stderr.decode("utf-8", "replace")
    except subprocess.TimeoutExpired:
        return None, b"", "（超时被强杀）"
    except Exception as exc:  # noqa: BLE001
        return None, b"", "（跑不起来：%s）" % exc


# ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

def self_test(root, devdocs):
    ok = bad = 0

    def check(cond, msg_ok, msg_bad):
        nonlocal ok, bad
        if cond:
            print("   ✓ %s" % msg_ok)
            ok += 1
        else:
            print("   ✗ %s" % msg_bad)
            bad += 1

    reg_path = os.path.join(root, REGISTRY_REL)
    if not os.path.exists(reg_path):
        print("   ✗ 自检前置：契约不在（%s）⇒ 自检跑不起来" % reg_path)
        return False
    with open(reg_path, "rb") as f:
        good_raw = f.read()
    good_txt = good_raw.decode("utf-8")

    d, why = read_registry(good_raw)
    check(d is not None and not judge_closedset(d, good_txt),
          "① 正控：真契约四格 == 批前登记值 ⇒ 不判红（S1）", "① 正控失败：真契约被判红（%s）" % why)

    d0, _ = read_registry(good_raw)
    d0 = d0 or {}
    d2 = json.loads(json.dumps(d0))
    d2["entries"] = list(d2["entries"]) + [{"id": "sigalg.probe", "note": "signature algorithm closed set",
                                            "sig_alg": "ed25519"}]
    p2 = judge_closedset(d2, json.dumps(d2, ensure_ascii=False))
    check(bool(p2) and any("sig_alg" in x for x in p2),
          "② **负控（`F4` 原文点名的那一格）**：造一条含 `sig_alg` 的条目 ⇒ **判红**",
          "② 负控失败：含 `sig_alg` 还判绿 —— 那就是「本批没人偷偷新增」这句被判假")
    d2b = json.loads(json.dumps(d0))
    d2b["entries"] = [dict(e, note=e.get("note", "") + " sig_alg") if isinstance(e, dict) else e
                      for e in d2b["entries"]]
    p2b = judge_closedset(d2b, json.dumps(d2b, ensure_ascii=False))
    check(bool(p2b) and any("sig_alg" in x for x in p2b) and len(p2b) == 1,
          "②b 成对负控：**条数不变**、只把 `sig_alg` 悄悄塞进正文 ⇒ **仍判红**（那一格自己有牙）",
          "②b 失败：条数不变就判绿（= 那一格只靠条数，`sig_alg` 这格是虚的）")

    d3 = json.loads(json.dumps(d0))
    d3["entries"] = list(d3["entries"])[:ENTRIES_N - 1]
    check(bool(judge_closedset(d3, json.dumps(d3, ensure_ascii=False))),
          "③ 负控：`entries` 少一条（13 → 12）⇒ 判红", "③ 负控失败：条数变了还判绿")
    d4 = json.loads(json.dumps(d0))
    d4.pop("change_flow", None)
    check(bool(judge_closedset(d4, json.dumps(d4, ensure_ascii=False))),
          "④ 负控：顶层少一键（5 → 4）⇒ 判红", "④ 负控失败：键少了还判绿")

    led_good = "## 单向铁律\n" + I4_VERBATIM + "\n状态：**未落地** · 预留旗标名**名字还没被占**\n"
    check(not judge_ledger(led_good), "⑤ 正控：登记表逐字带 I4 + 两个状态词 ⇒ 不判红（S2）",
          "⑤ 正控失败：齐了还判红")
    check(bool(judge_ledger(led_good.replace("未落地", "已落地"))),
          "⑥ 负控：把「未落地」写成「已落地」⇒ 判红（`R10`：不许把预留读成已实现）",
          "⑥ 负控失败：状态词被改成「已落地」还判绿")
    check(bool(judge_ledger(led_good.replace(I4_VERBATIM, ""))),
          "⑦ 负控：抹掉 `I4` 逐字句 ⇒ 判红", "⑦ 负控失败：铁律句被抹掉还判绿")

    check(not judge_flag_not_taken("--strength", 2, 0),
          "⑧ 正控：`--strength` ⇒ rc=2 且 stdout 0 字节 ⇒ 不判红（`M-20`）", "⑧ 正控失败：期望形态被判红")
    check(bool(judge_flag_not_taken("--strength", 0, 0)),
          "⑨ 负控：`--strength` rc=0 ⇒ 判红（名字被占了）", "⑨ 负控失败：被占了还判绿")
    check(bool(judge_flag_not_taken("--rekey", 2, 5)),
          "⑩ 负控：`--rekey` stdout 非空 ⇒ 判红", "⑩ 负控失败：stdout 非空还判绿")

    check(not judge_sync(0, ""), "⑪ 正控：同步门 rc=0 ⇒ 不判红（S5）", "⑪ 正控失败：rc=0 还判红")
    check(bool(judge_sync(1, "漂移 3 条")), "⑫ 负控：同步门 rc=1 ⇒ 判红（公开面有漂移）",
          "⑫ 负控失败：rc=1 还判绿")

    # ⑬/⑭ **真命令行 + 真退出码** 的成对负控（判据口在真目标上有没有牙）
    tmp = tempfile.mkdtemp(prefix="closedset-")
    ledp = os.path.join(tmp, 登记表件)
    with open(ledp, "w", encoding="utf-8") as f:
        f.write(led_good)
    fx_bad = os.path.join(tmp, "reg_sigalg.json")
    d5, _ = read_registry(good_raw)
    d5["entries"] = list(d5["entries"]) + [{"id": "sigalg.probe", "sig_alg": "ed25519"}]
    with open(fx_bad, "w", encoding="utf-8") as f:
        f.write(json.dumps(d5, ensure_ascii=False, indent=1))
    fx_good = os.path.join(tmp, "reg_good.json")
    with open(fx_good, "wb") as f:
        f.write(good_raw)

    def spawn(reg):
        p = subprocess.run([sys.executable, os.path.abspath(__file__), "--no-self-test",
                            "--static-only", "--registry", reg, "--devdocs", tmp],
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        return p.returncode, p.stdout.decode("utf-8", "replace")

    rc_bad, out_bad = spawn(fx_bad)
    check(rc_bad == 1 and "S1 破" in out_bad,
          "⑬ **真命令行负控**：夹具契约含 `sig_alg` ⇒ 本件 **rc=1** 且点名 `S1 破`",
          "⑬ 真命令行负控失败：rc=%s（该 1）· 输出里有没有 S1 破 = %s" % (rc_bad, "S1 破" in out_bad))
    rc_good, out_good = spawn(fx_good)
    check(rc_good == 0 and "全绿" in out_good,
          "⑭ **真命令行正控**：夹具契约 == 批前 ⇒ 本件 **rc=0**",
          "⑭ 真命令行正控失败：rc=%s（该 0）" % rc_good)

    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节 · 未新增任何条目）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(root, state, devdocs, registry, static_only, timeout):
    reds, nocalls = [], []
    reg_path = registry if registry else os.path.join(root, REGISTRY_REL)
    if not os.path.exists(reg_path):
        print("⛔ 未取数：契约不在（%s）⇒ **不出结论**（rc=2）" % reg_path)
        return 2
    with open(reg_path, "rb") as f:
        raw = f.read()
    d, why = read_registry(raw)
    if d is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    txt = raw.decode("utf-8")
    print("── S1 闭集对拍（%s）" % reg_path)
    print("   · 顶层键 %d 个 = %s（批前 %s）" % (len(d), ",".join(d.keys()), ",".join(TOP_KEYS)))
    print("   · `entries` %s 条（批前 %d） · 含 `sig_alg` %d 次（★必须保持 %d） · 含 `approve` %d 次（批前 %d）"
          % (len(d.get("entries", [])) if isinstance(d.get("entries"), list) else "（不是数组）",
             ENTRIES_N, txt.count("sig_alg"), SIG_ALG_N, txt.count("approve"), APPROVE_N))
    print("   · 整件 sha256 = %s（**只印不判**：无关的合法改动不该变假红）"
          % hashlib.sha256(raw).hexdigest())
    reds += judge_closedset(d, txt)

    print("── S2 单向铁律登记（仓外开发文档面 = %s）" % devdocs)
    led = os.path.join(devdocs, 登记表件)
    if not os.path.isdir(devdocs) or not os.path.exists(led):
        nocalls.append("S2 **未取数**：登记表不在（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿" % led)
    else:
        with open(led, encoding="utf-8") as f:
            lt = f.read()
        print("   · 登记表 %s（%d 字节）" % (登记表件, len(lt.encode())))
        reds += judge_ledger(lt)

    if static_only:
        print("   · `--static-only`：本跑只判静态两格（自检内部用）")
    else:
        zerg = os.path.join(root, "bin", "zerg")
        print("── S3/S4 预留旗标名还没被占（`P27` · 沙箱状态目录 %s）" % state)
        if not os.path.exists(zerg):
            nocalls.append("S3/S4 **未取数**：`bin/zerg` 不在（%s）⇒ 不给结论（rc=2）—— "
                           "先走自举档 `zerg build all --only cli`" % zerg)
        else:
            for flag in RESERVED_FLAGS:
                rc, out, err = run_cmd([zerg, "approve", "keygen", flag, "token", "--by", "<名>"],
                                       state, timeout=timeout)
                print("   · `zerg approve keygen %s token` ⇒ rc=%s · stdout %d 字节 · stderr 首行 = %s"
                      % (flag, rc, len(out), (err.strip().splitlines() or ["（空）"])[0]))
                reds += judge_flag_not_taken(flag, rc, len(out))

        print("── S5 公开面零改（`F4` 判据④ 点名的那条命令）")
        sync = os.path.join(root, SYNC_GATE_REL)
        if not os.path.exists(sync):
            nocalls.append("S5 **未取数**：同步门不在（%s）⇒ 不给结论（rc=2）" % sync)
        else:
            rc, out, err = run_cmd([sys.executable, sync], state, timeout=timeout)
            tail = (err.strip().splitlines() or [""])[-1]
            print("   · `python3 %s` ⇒ rc=%s" % (SYNC_GATE_REL, rc))
            reds += judge_sync(rc, tail)

    print("── 口径（照实写 · 防止被读成「已经加了」）")
    print("   · 本件**只登记**：`sig_alg` 条目今天**不存在**、预留旗标名**还没被占** ⇒ "
          "**未落地**（`R10`：不许把预留接口读成「已经升过了」）")
    print("   · 「零改」指的是**生效面的判定口径零漂移**；新门禁件按名登记进 `EXCLUDES`/`DROP_EXACT`/"
          "标记表这条**接线**必须做（只**增行**，既有件一行不改）")
    if reds or nocalls:
        rc_out = 1 if reds else 2
        print("⇒ 门禁%s（rc=%d）：红 %d 条 · 不给结论 %d 条"
              % ("红" if rc_out == 1 else "不给结论", rc_out, len(reds), len(nocalls)))
        for p in reds:
            print("   ✗ %s" % p)
        for p in nocalls:
            print("   ⛔ %s" % p)
        return rc_out
    print("⇒ 全绿（rc=0）：闭集四格与批前逐字相同（`sig_alg` 保持 0）· 单向铁律已在登记表 · "
          "三个预留旗标名都没被占 · 公开面无漂移")
    return 0


def main():
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--state", default=None, help="状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）")
    ap.add_argument("--devdocs", default=None, help="开发文档面根（默认 = `ZERG_DEVDOCS_ROOT` > 仓同级 `Zerg-内部文档/项目文档/v2.5.11`）")
    ap.add_argument("--registry", default=None, help="契约路径（默认 = 仓内 `%s`；自检夹具用）" % REGISTRY_REL)
    ap.add_argument("--static-only", action="store_true", help="只判静态两格（自检内部用 · 不跑命令面）")
    ap.add_argument("--timeout", type=int, default=60, help="每一格的时限（秒 · 默认 60）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    root = os.path.abspath(args.root)
    state = state_dir_of(args.state)
    devdocs = devdocs_root_of(root, args.devdocs)
    print(VERSION)
    if not args.no_self_test and not self_test(root, devdocs):
        return 1
    if args.self_test:
        return 0
    print("── 真目标：%s（状态目录 %s）" % (root, state))
    if args.state is None and os.environ.get("ZERG_STATE_DIR"):
        print("★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"
              "**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）"
              % os.environ["ZERG_STATE_DIR"])
    return scan(root, state, devdocs, args.registry, args.static_only, args.timeout)


if __name__ == "__main__":
    sys.exit(main())

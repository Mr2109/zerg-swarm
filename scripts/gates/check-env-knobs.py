#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-env-knobs.py —— 「门禁/提交流程环境旋钮」**登记面**（缺口 `GAP-20260927-439`）

题面（缺口 `want_action`）：开一个**环境旋钮登记面**（列出每个 `ZERG_*` 旋钮：**谁读 · 换什么 ·
    毒化后表现**），供门脚本与文档共用。缺口正文：`scripts/gates/` 只有 `check-git-env-hygiene.py`
    （git 环境）与 `check-shell-unicode-vars.py`（shell 变量），**没有覆盖 `ZERG_*` 这类会让命令换真源
    的环境旋钮**（`GAP-20260927-73/-74` 的「环境毒化名单」今天不存在）。

口径（写死 —— 本件的判据）：
    · **扫面** = 「门禁/提交流程的**旋钮宿主**」＝ 步骤表与包装脚本那几件（**显式点名**，不用通配）：
        `precommit-gates.sh` · `all.sh` · `real-gates.sh` · `shard-go-test.sh` ·
        `judge-lock.py` · `judge-targets.sh` · `mutation-smoke.sh`（皆在 `scripts/gates/` 下）。
      —— 这一面就是「毒化一次提交/一趟门」的那批旋钮宿主（逐个子门脚本的一堆 `ZERG_ROOT` 一类
      入参旋钮不入面：它们是**子门自己的入参**，不换提交链的真源）。
    · 「旋钮」的**机械判定**（**消费形态**，不是「出现过这串」）：`os.Getenv("ZERG_…")` /
      `os.environ.get("ZERG_…")` / `environ["ZERG_…"]` / `${ZERG_…:-…}` / `${ZERG_…}`。
    · **统一登记表** = 本件 `KNOBS`（`{旋钮: {readers, swap, poison}}`）—— **单一真源**：
        `readers` 谁读（宿主件）· `swap` 换什么真源 · `poison` 毒化后表现。
    · 判三档：A **漏登记**（消费面检出、`KNOBS` 里没有 ⇒ 红）· B **登记陈旧**（`KNOBS` 点名、消费面
      查无 ⇒ 红）· C **登记项三格不齐**（readers/swap/poison 任一空 ⇒ rc=2，写不清就是洗白）。
    · **退码写死**：`rc=0` 全绿 · `rc=1` A/B（逐条点名）· `rc=2` 扫面缺件 / C / 空转。
    · **只读**：不写任何文件。

用法：
    python3 scripts/gates/check-env-knobs.py [--root <dir>] [--json] [--self-test]

★ 接线状态：本件为**新门**（门109 · `gates` scope · `tri-report` 只报告档 · **不进** `FAST_STEPS`）。
  接线＝动步骤表 ＋ 公开标记表 ＋ 两器排除三表 ＋ judge-registry ＋ affected-map（**同批**）。
  本件与门99（关自检旋钮）**分工不同**：门99 只收「关自检」型旋钮且限提交链步；本件收**消费面全部**
  `ZERG_*` 旋钮并逼其逐条登记三格。判据件不开源。
"""
import argparse
import json
import os
import re
import sys

# ── 旋钮宿主面（显式点名 —— 毒化一次提交/一趟门的那批件）────────────────────────
KNOB_HOSTS = (
    "scripts/gates/precommit-gates.sh",
    "scripts/gates/all.sh",
    "scripts/gates/real-gates.sh",
    "scripts/gates/shard-go-test.sh",
    "scripts/gates/judge-lock.py",
    "scripts/gates/judge-targets.sh",
    "scripts/gates/mutation-smoke.sh",
)

# ── 环境旋钮登记表（单一真源）──────────────────────────────────────────────────
#   三格：readers 谁读 · swap 换什么真源 · poison 毒化后表现。
KNOBS = {
    "ZERG_GATE_REPO_ROOT": {"readers": "precommit-gates.sh", "swap": "仓根（本件专用旋钮）",
                            "poison": "把整趟门指到另一棵树 ⇒ 门判的不是你要提交的那个仓"},
    "ZERG_REPO_ROOT": {"readers": "all.sh · real-gates.sh", "swap": "仓根（与命令面 ZERG_REPO 同名族、互不读取）",
                       "poison": "门/包装脚本读的是别处的仓 ⇒ 覆盖面静默缩小"},
    "ZERG_GATE_LOCK_HELD": {"readers": "precommit-gates.sh", "swap": "单实例闸（自报「已持锁」）",
                            "poison": "置 1 即绕过互斥 ⇒ 一门生门跑（并发多趟）"},
    "ZERG_GATE_LOCK_DIR": {"readers": "judge-lock.py", "swap": "锁目录", "poison": "两趟各锁各的 ⇒ 单实例闸失能"},
    "ZERG_GATE_NO_LOCK": {"readers": "precommit-gates.sh", "swap": "单实例闸（显式关锁）",
                          "poison": "显式关锁 ⇒ 并发跑门"},
    "ZERG_GATE_NO_LOCK_REASON": {"readers": "precommit-gates.sh", "swap": "关锁的理由申报",
                                 "poison": "空理由也放行 ⇒ 关锁不留痕"},
    "ZERG_PUBLIC_TREE": {"readers": "precommit-gates.sh", "swap": "公开产物树根", "poison": "指到别处 ⇒ 公开面判据判错树"},
    "ZERG_RELEASE_DIR": {"readers": "precommit-gates.sh", "swap": "发布产物目录", "poison": "指到别处 ⇒ B11 占位残留判错目录"},
    "ZERG_GAP_INTAKE": {"readers": "precommit-gates.sh", "swap": "缺口入账开关", "poison": "静默跳过入账"},
    "ZERG_GATES_SELFTEST_DEPTH": {"readers": "precommit-gates.sh", "swap": "自检递归深度", "poison": "改深度 ⇒ 自检段行为改变"},
    "ZERG_LANE_TOKEN": {"readers": "precommit-gates.sh", "swap": "趟令牌", "poison": "换令牌 ⇒ 锁/标记归属改写"},
    "ZERG_SHARD_LOCK_DIR": {"readers": "shard-go-test.sh", "swap": "分片器锁目录", "poison": "弱化「同仓同刻只许一趟」的口径"},
    "ZERG_STATE_DIR": {"readers": "precommit-gates.sh · judge-targets.sh", "swap": "状态目录（账/批准件/真源所在）",
                       "poison": "指沙箱 ⇒ 状态目录门「未取数 ⇒ rc=2」；CLI `gap ls` 也退 8"},
    "ZERG_DEVDOCS_ROOT": {"readers": "precommit-gates.sh（及其调的子门，多读点）", "swap": "文档仓面根",
                          "poison": "一变量两语义（仓根 vs 语料目录）⇒ 门扫错面 / 非 UTF-8 件把工具读崩、假红"},
    "ZERG_DOCS_ROOT": {"readers": "precommit-gates.sh", "swap": "文档仓根 / 语料目录（语义二义）",
                       "poison": "同上：外层导出即把门⑫ 的语料面换掉 ⇒ 读数不是判据"},
    "ZERG_SVCDECL_DECL": {"readers": "precommit-gates.sh", "swap": "服务声明件（与现值源对拍）",
                          "poison": "换成夹具 ⇒ 声明 vs 现值差集恒空 ⇒ 门⑧ 恒绿"},
    "ZERG_TARGET_REPO": {"readers": "judge-targets.sh", "swap": "判据靶仓根", "poison": "判的仓不是目标仓"},
    "ZERG_TARGET_ROOT": {"readers": "judge-targets.sh", "swap": "判据靶根", "poison": "同上"},
    "ZERG_TARGET_GO": {"readers": "judge-targets.sh", "swap": "靶 go 工具链", "poison": "换工具链 ⇒ 编译面判据失真"},
    "ZERG_ALLSH_GATES": {"readers": "all.sh", "swap": "all.sh 收的门面", "poison": "少收/多收门 ⇒ 读数不完整"},
    "ZERG_ALLSH_UNITTEST": {"readers": "all.sh", "swap": "all.sh 走自检夹具路", "poison": "走夹具 ≠ 真跑（关自检型）"},
    "ZERG_ALLSH_NO_PERL": {"readers": "all.sh", "swap": "all.sh 走无 perl 夹具路", "poison": "同上"},
    "ZERG_REAL_GATES_JOBS": {"readers": "real-gates.sh", "swap": "并发度", "poison": "超订/串行 ⇒ 读数与墙钟漂"},
    "ZERG_REAL_GATES_UNITTEST": {"readers": "real-gates.sh", "swap": "real-gates 走自检夹具路", "poison": "走夹具 ≠ 真跑（关自检型）"},
}

CONSUME_RES = (
    re.compile(r'os\.(?:Getenv|getenv)\(\s*[\'"]([A-Z][A-Z0-9_]*)[\'"]'),
    re.compile(r"environ\.get\(\s*['\"]([A-Z][A-Z0-9_]*)['\"]"),
    re.compile(r"environ\[\s*['\"]([A-Z][A-Z0-9_]*)['\"]"),
    re.compile(r"\$\{([A-Z][A-Z0-9_]*):-"),
    re.compile(r"\$\{([A-Z][A-Z0-9_]*)\}"),
)


def consumed(root):
    """扫旋钮宿主面 ⇒ (found, miss)。found={旋钮: sorted(宿主件)}；miss=盘上缺的宿主件。"""
    found, miss = {}, []
    for rel in KNOB_HOSTS:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            miss.append(rel)
            continue
        src = open(p, "r", encoding="utf-8", errors="replace").read().splitlines()
        for line in src:
            for cr in CONSUME_RES:
                for m in cr.finditer(line):
                    n = m.group(1)
                    if n.startswith("ZERG_"):
                        found.setdefault(n, set()).add(rel)
    return {k: sorted(v) for k, v in found.items()}, miss


def run(root, as_json):
    found, miss = consumed(root)
    if not found:
        if as_json:
            print(json.dumps({"rc": 2, "error": "旋钮宿主面 0 消费（空转/缺件）", "miss": miss}, ensure_ascii=False))
        else:
            print("BLOCKED —— 旋钮宿主面 0 消费（取不到 ≠ 无旋钮）⇒ rc=2；盘上缺：%s"
                  % ("、".join(miss) if miss else "（无）"), file=sys.stderr)
        return 2
    bad_reg = sorted({k for k, v in KNOBS.items()
                      if not (v.get("readers") and v.get("swap") and v.get("poison"))})
    if bad_reg:
        if as_json:
            print(json.dumps({"rc": 2, "bad_registry": bad_reg}, ensure_ascii=False))
        else:
            print("BLOCKED —— 登记项三格不齐：%s ⇒ rc=2（写不清 = 洗白）" % "、".join(bad_reg),
                  file=sys.stderr)
        return 2
    unregistered = sorted(k for k in found if k not in KNOBS)
    stale = sorted(k for k in KNOBS if k not in found)
    bad = len(unregistered) + len(stale)
    if as_json:
        print(json.dumps({"rc": 1 if bad else 0, "consumed_count": len(found), "registered_count": len(KNOBS),
                          "unregistered": unregistered, "stale": stale}, ensure_ascii=False))
    else:
        print("门禁/提交流程环境旋钮登记面（GAP-20260927-439）")
        print("消费面检出旋钮 = %d 个 · 登记表 = %d 行 · 宿主 %d 件" % (len(found), len(KNOBS), len(KNOB_HOSTS)))
        print("── A 漏登记（消费面检出、登记表里没有）：%d 个" % len(unregistered))
        for k in unregistered:
            print("   ✗ %s（读者：%s）" % (k, "、".join(found[k])))
        print("── B 登记陈旧（表里点名、消费面查无）：%d 个" % len(stale))
        for k in stale:
            print("   ✗ %s" % k)
        if bad:
            print("⇒ 违规 %d 处（rc=1）：旋钮必须逐条登记（谁读 · 换什么真源 · 毒化后表现）" % bad)
        else:
            print("⇒ 全绿（rc=0）：%d 个旋钮逐条登记三格" % len(found))
    return 1 if bad else 0


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts", "gates"))
        host = os.path.join(td, "scripts", "gates", "precommit-gates.sh")
        open(host, "w").write('R="${ZERG_GATE_REPO_ROOT:-/x}"\n')
        saved = dict(KNOBS)

        def use(knobs):
            KNOBS.clear(); KNOBS.update(knobs)

        ROOTK = {"ZERG_GATE_REPO_ROOT": {"readers": "precommit-gates.sh", "swap": "仓根", "poison": "指到别处"}}
        try:
            # ① 正控：会话旋钮已登记 ⇒ rc=0
            use(ROOTK)
            if run(td, False) != 0:
                print("  ✗ ① 正控失败：应回 0"); ok = False
            # ② must-fail A：新旋钮未登记 ⇒ rc=1
            open(host, "w").write('R="${ZERG_GATE_REPO_ROOT:-/x}"\ne="${ZERG_BRAND_NEW:-}"\n')
            if run(td, False) != 1:
                print("  ✗ ② A 失败：未登记旋钮应回 1"); ok = False
            # ③ must-fail B：登记表点名、消费面查无 ⇒ rc=1
            open(host, "w").write('R="${ZERG_GATE_REPO_ROOT:-/x}"\n')
            reg = dict(ROOTK); reg["ZERG_GHOST"] = {"readers": "x", "swap": "y", "poison": "z"}
            use(reg)
            if run(td, False) != 1:
                print("  ✗ ③ B 失败：登记陈旧应回 1"); ok = False
            # ④ must-fail C：登记项三格不齐 ⇒ rc=2
            use({"ZERG_GATE_REPO_ROOT": {"readers": "", "swap": "y", "poison": "z"}})
            if run(td, False) != 2:
                print("  ✗ ④ C 失败：三格不齐应回 2"); ok = False
            # ⑤ must-fail 空转：宿主面 0 消费 ⇒ rc=2
            open(host, "w").write("# 无旋钮\n")
            use(ROOTK)
            if run(td, False) != 2:
                print("  ✗ ⑤ 空转失败：应回 2"); ok = False
        finally:
            KNOBS.clear(); KNOBS.update(saved)
    print("自检 %s（5 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="门禁/提交流程环境旋钮登记面（GAP-20260927-439）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

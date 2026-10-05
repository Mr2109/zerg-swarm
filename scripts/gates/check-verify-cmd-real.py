#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-verify-cmd-real.py —— 「判据须能真判」门（缺口 `GAP-20260926-55` · 报告档首版）

题面（缺口逐字）：
  `zerg gap add` 的 `--repro-cmd/--verify-cmd` 是**必填**但成**样板值**：填的人只能写
  `zerg task ls` 一类，**判不了任何一条**。

判据：一条账的某一列「**能真判**」 ⟺ 两关都过 ——
  ① **解析到命令树** —— 去掉 `zerg` 前缀（首词 basename == `zerg`）后，其 token 序列必须是
     命令树里某条命令 token 序列的**前缀**（真源 = 命令树机器面，见下）。
  ② **不是通用命令** —— 通用命令 = **内省型**命令（**闭集 · 逐字写死**，见 `GENERIC_HEADS`
     / `GENERIC_VERB_TAIL`）：`zerg <help|doctor> [<任意>]` · `zerg <族> <ls|status>`。
     这类命令只**列举 / 打印**某命名空间的概览，**任何一格都不是对本缺口靶子的判定**
     ⇒ 命中即「样板值」（判不了任何一条）。
  ★ 通用命令是**闭集**（形态 + 动词闭集，逐 token 比对 · **无子串 / 无模糊匹配**）：
     凡本条闭集之外的命令**一律不判为样板**（宁可漏报、不许乱咬——同族纪律）。

面分（本门覆盖哪几面 · 哪几面**不**覆盖，逐条声明）：
  · **verify 面**：①② 两关都判 —— 这是 `-926-55` 的主症面。
  · **repro 面**：**只判 ②「非通用命令」**。`--repro-cmd` 设计上收**自由文本**
    （`go test …` / `gh …` / 散文都合法），与写侧口径一致 ⇒ 本门**不擅自**给它加
    「解析到命令树」这一关。那面属 `GAP-20260927-246` 的**写侧面**（`gap add` 对
    `--repro-cmd` 的守门），**不归本门**、本门不判并如实声明。
  · **未覆盖（如实列）**：`gap add` 的**写侧**当场拒（`family_gap.go` 的 `gapJudgeArgv`
    只校「在不在命令树里」、不校「判不判得了」）· `GAP-20260925-03` 的**豁免形态**
    （`--verify-cmd` 记「某条命令缺失」这类自指缺口）—— 两者都要动 **Go 写侧件**，
    本门是**存量只读检测**，**不覆盖写侧**。

命令树真源：`core/cmd/zerg/testdata/cli-matrix.json` 的 `cases[].command` 列。
  ★ 为什么它能当命令树面：门⑪ `T1` 保证它与 `main.go` 的命令树**逐字同尺**
    （「每条命令至少一条 case」+「不许有幽灵」）—— 见 `core/cmd/zerg/family_gap.go:377`
    （逐字「解析面 = `main.go` 的命令树（门⑪ `T1` 保证它与 `cli-matrix.json` 的 `command` 列
    **逐字同尺**）」）。

棘轮：存量样板值**大量**存在 ⇒ 本门**首日取现读数**为基线（`BASELINE`，见下），
  **只许下调、永不上调**（上调 = 放宽口径 ⇒ 必须走拍板）。

档位：**报告档首版** —— 默认**不阻断退码**（即便样板值 > 基线，也只报告、`rc=0`）；
  `--strict` ⇒ 某列样板值 > 基线 ⇒ `rc=1`（阻断）。
  ★ 转档条件：把现存样板值**逐批补成真判据**、基线降到 **0** 后，本门转 `tri` 阻断档。

退码（三档 · 优先级 2 > 1 > 0）：
  0 = 报告完成（**报告档**；`--strict` 下且**无**超额）
  1 = **有红**（仅 `--strict` 下：某列样板值 > 基线）
  2 = **不给结论**（用法错 / 缺口账缺件 / 命令树缺件 / 账为空 / 自检不过）

用法：
  python3 scripts/gates/check-verify-cmd-real.py              # 自检 + 扫真账（报告档）
  python3 scripts/gates/check-verify-cmd-real.py --json       # stdout 只有 JSON
  python3 scripts/gates/check-verify-cmd-real.py --self-test  # 只跑自检（合成夹具）
  python3 scripts/gates/check-verify-cmd-real.py --list-rules # 判据 + 闭集 + 退码口径
  python3 scripts/gates/check-verify-cmd-real.py --baseline   # 棘轮视图（只读）
  python3 scripts/gates/check-verify-cmd-real.py --strict     # 超额即 rc=1
  python3 scripts/gates/check-verify-cmd-real.py --root <仓根> --ledger <账件> --matrix <命令树件>

纪律：**只读**（除 /tmp 夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件。
★ 登记面：本件**未接步骤表**（报告档首版）⇒ 按 `scripts/gates/gate-coverage.config.json` 的
  **第二档白名单**登记（`whitelist.scripts`，带 `reason`+`date`）⇒ 门① 报「白名单豁免」而非
  「悬空」；同时在 `scripts/公开标记.tsv` 登记为 `no`（判据件不开源）。
"""
import argparse
import atexit
import json
import os
import shutil
import sys
import tempfile

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))
MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")
LEDGER_FILE = "zerg-cli-gaps.jsonl"

# ══════════════════════════════════════════════════════════════════
# 通用命令**闭集**（唯一一处定义 · 逐字写死 · 无子串 / 无模糊匹配）
# ══════════════════════════════════════════════════════════════════
#   为什么是「内省型」：这类命令的**每一格**都只在对**命名空间**做概览
#   （列举 / 打印帮助 / 打印诊断 / 打印状态），**没有一格**指向本缺口的靶子
#   ⇒ 裸跑它 = 「什么也没判」。这正是 `GAP-20260926-55` 统计出的「样板值」。
#   为什么是**闭集**：① `ls` / `status` 是「列举类动词」；② `help` / `doctor` 是
#   「帮助 / 诊断类头」。闭集之外（如 `gate find <kw>` / `gate run --fast` / `model show <名>`）
#   **一律不判为样板** —— 宁可漏报，不许乱咬。
GENERIC_HEADS = ("help", "doctor")        # `zerg <head>` / `zerg <head> <任意>`
GENERIC_VERB_TAIL = ("ls", "status")      # `zerg <族> <ls|status>`

# ── 棘轮基线（**首日现读数** · 只许下调、永不上调）────────────────────────────
#   现值取 2026-10-05 现读（账 1180 行 · 命令树 188 条）：
#     verify 面不可判 = 675（通用命令 675 · 不解析 0）· repro 面样板 = 526
#   ★ 与 `GAP-20260926-55` 注内「最大重复 98 / 103」同源口径（本条按**整列全量**数，
#     故绝对值更大）；《口径》= 一条账的该列命中闭集即记 1。
BASELINE = {"verify": 675, "repro": 526}
BASELINE_NOTE = ("2026-10-05 首测（本门新建当天）：账 1180 行 · 命令树 188 条 ⇒ "
                 "verify 面不可判 = 675（其中 通用命令 675 · 不解析命令树 0）· "
                 "repro 面样板 = 526。★ 只认 `BASELINE` 那两格；本段历史值仅供追溯。")


class Unsupported(Exception):
    """不可判定 ⇒ 一律 rc=2（不给结论），绝不猜。"""


# ══════════════════════════════════════════════════════════════════
# 1. 通用命令闭集判定 + 命令树解析
# ══════════════════════════════════════════════════════════════════
def norm_cmd(s):
    return " ".join((s or "").strip().strip("\"'").split())


def tokens_of(cmd):
    """一条命令 ⇒ 去 `zerg` 前缀后的 token 列表（首词 basename == zerg 才剥）。"""
    toks = norm_cmd(cmd).split()
    if toks and os.path.basename(toks[0]) == "zerg":
        toks = toks[1:]
    return toks


def is_generic_command(toks):
    """命中通用命令**闭集**？⇒ True / False（逐 token 比对 · 无子串匹配）。

    闭集（去 `zerg` 前缀后）：
      · 1 词：`<heads>`                                      （`zerg help` / `zerg doctor`）
      · 2 词：首词 ∈ `<heads>`  或  尾词 ∈ `<verb_tail>`       （`zerg help <族>` /
               `zerg doctor --json` / `zerg <族> ls` / `zerg <族> status`）
      · 其余：不判。
    """
    n = len(toks)
    if n == 1:
        return toks[0] in GENERIC_HEADS
    if n == 2:
        return toks[0] in GENERIC_HEADS or toks[1] in GENERIC_VERB_TAIL
    return False


def resolves_in_tree(toks, tree):
    """去 `zerg` 前缀后的 token 序列，是不是命令树里某条命令 token 序列的**前缀**？

    ★ 用**前缀**而非整等：`zerg gate find check-x` 解析到命令 `gate find` + 实参；
      `zerg gate ls` 解析到命令 `gate ls`。空 token ⇒ False（只有 `zerg` 前缀 = 不是命令）。
    """
    if not toks:
        return False
    return any(cmd == toks[:len(cmd)] for cmd in tree)


# ══════════════════════════════════════════════════════════════════
# 2. 真源装载（命令树机器面 + 缺口账）
# ══════════════════════════════════════════════════════════════════
def default_ledger():
    state_dir = os.environ.get("ZERG_STATE_DIR") or os.path.join(
        os.path.expanduser("~"), ".zerg", "state")
    return os.path.join(state_dir, LEDGER_FILE)


def load_tree(root, matrix):
    path = matrix or os.path.join(root, MATRIX_REL)
    if not os.path.isfile(path):
        raise Unsupported("命令树机器面缺件：%s（缺件 ⇒ 不给结论）" % path)
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except Exception as exc:  # noqa: BLE001
        raise Unsupported("命令树机器面不是合法 JSON：%s（%s）" % (path, exc))
    cases = data.get("cases") if isinstance(data, dict) else None
    if not isinstance(cases, list) or not cases:
        raise Unsupported("命令树机器面 %s 的 `cases` 为空 ⇒ 空转，不给结论" % path)
    tree = []
    for c in cases:
        cmd = c.get("command") if isinstance(c, dict) else None
        if isinstance(cmd, str) and cmd.strip():
            tree.append(cmd.split())
    if not tree:
        raise Unsupported("命令树机器面里 0 条 `command` ⇒ 空转，不给结论")
    return tree, path


def load_ledger(ledger):
    path = ledger or default_ledger()
    if not os.path.isfile(path):
        raise Unsupported("缺口账真源缺件：%s（缺件 ⇒ 不给结论）" % path)
    rows = []
    with open(path, encoding="utf-8") as fh:
        for i, line in enumerate(fh, 1):
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except Exception as exc:  # noqa: BLE001
                raise Unsupported("缺口账第 %d 行不是合法 JSON：%s（%s）" % (i, exc))
    if not rows:
        raise Unsupported("缺口账为空（0 行）⇒ 空转，不给结论")
    return rows, path


# ══════════════════════════════════════════════════════════════════
# 3. 判定 / 汇总
# ══════════════════════════════════════════════════════════════════
def analyse(rows, tree):
    v_generic, v_unresolved, r_generic = [], [], []
    for r in rows:
        rid = r.get("id") or "?"
        tv = tokens_of(r.get("verify_cmd", ""))
        if is_generic_command(tv):
            v_generic.append({"id": rid, "cmd": norm_cmd(r.get("verify_cmd", "")), "why": "通用命令（闭集）"})
        elif not resolves_in_tree(tv, tree):
            v_unresolved.append({"id": rid, "cmd": norm_cmd(r.get("verify_cmd", "")),
                                 "why": "不解析命令树"})
        tr = tokens_of(r.get("repro_cmd", ""))
        if is_generic_command(tr):
            r_generic.append({"id": rid, "cmd": norm_cmd(r.get("repro_cmd", "")), "why": "通用命令（闭集）"})
    return {
        "total": len(rows),
        "verify": {"generic": v_generic, "unresolved": v_unresolved,
                   "bad": len(v_generic) + len(v_unresolved)},
        "repro": {"generic": r_generic, "bad": len(r_generic)},
    }


def verdict_of(rep, strict, baseline=None):
    """⇒ (rc, code, reason)。报告档：默认恒 0；--strict 才按超额判红。

    ★ `baseline` 可注入（自检用零基线/自定义基线验成对负控）；调用方一律传默认 `BASELINE`。
    """
    baseline = baseline or BASELINE
    extra_v = rep["verify"]["bad"] - baseline["verify"]
    extra_r = rep["repro"]["bad"] - baseline["repro"]
    over = (extra_v > 0) or (extra_r > 0)
    if strict and over:
        return (EXIT_FAIL, "RED",
                "超额：verify %d > 基线 %d（超 %d）· repro %d > 基线 %d（超 %d）"
                % (rep["verify"]["bad"], baseline["verify"], max(extra_v, 0),
                   rep["repro"]["bad"], baseline["repro"], max(extra_r, 0)))
    tag = "超额（报告档不阻断）" if over else "不超额"
    return (EXIT_OK, "REPORT",
            "报告档 · %s：verify 样板 %d / %d（通用 %d · 不解析 %d）· "
            "repro 样板 %d / %d"
            % (tag, rep["verify"]["bad"], rep["total"],
               len(rep["verify"]["generic"]), len(rep["verify"]["unresolved"]),
               rep["repro"]["bad"], rep["total"]))


# ══════════════════════════════════════════════════════════════════
# 4. 报告
# ══════════════════════════════════════════════════════════════════
def print_report(rep, ledger_path, matrix_path, max_examples, sink=None):
    sink = sink or sys.stdout
    v, r = rep["verify"], rep["repro"]
    print("── 判据须能真判门（GAP-20260926-55 · 报告档首版）──", file=sink)
    print("   缺口账真源：%s（%d 行）" % (ledger_path, rep["total"]), file=sink)
    print("   命令树真源：%s" % matrix_path, file=sink)
    print("   基线（棘轮 · 只许下调）：verify=%d · repro=%d"
          % (BASELINE["verify"], BASELINE["repro"]), file=sink)
    print(file=sink)
    print("== verify_cmd 面（①② 两关都判）==", file=sink)
    print("   不可判 = **%d / %d**（其中 通用命令 %d · 不解析命令树 %d）"
          % (v["bad"], rep["total"], len(v["generic"]), len(v["unresolved"])), file=sink)
    for it in v["generic"][:max_examples]:
        print("     - [通用] %s  ← %s" % (it["id"], it["cmd"]), file=sink)
    if len(v["generic"]) > max_examples:
        print("     … 另 %d 条通用命令（--max-examples 调整）" % (len(v["generic"]) - max_examples), file=sink)
    for it in v["unresolved"][:max_examples]:
        print("     - [不解析] %s  ← %s" % (it["id"], it["cmd"]), file=sink)
    print(file=sink)
    print("== repro_cmd 面（只判 ②「非通用命令」· 解析面不判）==", file=sink)
    print("   样板 = **%d / %d**（通用命令）" % (r["bad"], rep["total"]), file=sink)
    for it in r["generic"][:max_examples]:
        print("     - [通用] %s  ← %s" % (it["id"], it["cmd"]), file=sink)
    if len(r["generic"]) > max_examples:
        print("     … 另 %d 条通用命令（--max-examples 调整）" % (len(r["generic"]) - max_examples), file=sink)
    print("   ★ 未覆盖形态：`--repro-cmd` 设计上收**自由文本** ⇒ 「解析到命令树」这一关"
          "**不适用**（属 `GAP-20260927-246` 的写侧面）。", file=sink)
    print(file=sink)
    rc, code, reason = verdict_of(rep, False)
    print("== 结论 ==", file=sink)
    print("   %s" % reason, file=sink)
    print("   ⇒ 报告档（rc=0）；`--strict` 时超额 ⇒ rc=1", file=sink)


def print_baseline(rep, sink=None):
    sink = sink or sys.stdout
    print("── 棘轮视图（只读 · 不改任何文件）──", file=sink)
    print("   verify 面不可判 现测 = %d · 基线 = %d · 剩余（可下调到）= %d"
          % (rep["verify"]["bad"], BASELINE["verify"], rep["verify"]["bad"]), file=sink)
    print("   repro  面样板   现测 = %d · 基线 = %d · 剩余 = %d"
          % (rep["repro"]["bad"], BASELINE["repro"], rep["repro"]["bad"]), file=sink)
    print("   自述 = %s" % BASELINE_NOTE, file=sink)
    print("   号令：一次只许**下调**基线（改成现测值），**永不上调**；"
          "超额 ⇒ 先补真判据或挪出闭集，✗ 不许抬基线。", file=sink)


def print_list_rules(sink=None):
    sink = sink or sys.stdout
    print("check-verify-cmd-real.py 规则表（判据须能真判门 · GAP-20260926-55）", file=sink)
    print("  判据：一条账某列「能真判」 ⟺ ① 解析到命令树  且  ② 不是通用命令", file=sink)
    print("  通用命令**闭集**（去 zerg 前缀后 · 逐 token 比对 · 无子串匹配）：", file=sink)
    print("     · 1 词：%s" % " / ".join(GENERIC_HEADS), file=sink)
    print("     · 2 词：首词 ∈ %s  或  尾词 ∈ %s"
          % (" / ".join(GENERIC_HEADS), " / ".join(GENERIC_VERB_TAIL)), file=sink)
    print("  面分：verify 面 ①② 都判 · repro 面 只判 ②（解析面不适用）", file=sink)
    print("  棘轮：BASELINE verify=%d · repro=%d（只许下调）"
          % (BASELINE["verify"], BASELINE["repro"]), file=sink)
    print("  退码：0 报告完成 · 1（仅 --strict 超额）· 2 不给结论", file=sink)


# ══════════════════════════════════════════════════════════════════
# 5. 自检（成对负控 · 走真退码）
# ══════════════════════════════════════════════════════════════════
_TMPDIRS = []


def _fixture(base, name, matrix_cmds, ledger_rows):
    root = os.path.join(base, name)
    os.makedirs(os.path.join(root, "core", "cmd", "zerg", "testdata"))
    with open(os.path.join(root, MATRIX_REL), "w", encoding="utf-8") as fh:
        json.dump({"cases": [{"command": c} for c in matrix_cmds]}, fh, ensure_ascii=False)
    ledger = os.path.join(root, "ledger.jsonl")
    with open(ledger, "w", encoding="utf-8") as fh:
        for row in ledger_rows:
            fh.write(json.dumps(row, ensure_ascii=False) + "\n")
    return root, ledger


def self_test(sink=None):
    if sink is None:
        sink = sys.stdout
    base = tempfile.mkdtemp(prefix="vcr-selftest-")
    _TMPDIRS.append(base)
    cases = []

    def check(desc, ok, extra=""):
        cases.append((desc, bool(ok), extra))

    # ── 闭集（纯函数）逐格 ──────────────────────────────────────────────
    check("闭集：`zerg task ls` ⇒ 通用", is_generic_command(tokens_of("zerg task ls")))
    check("闭集：`zerg gate ls` ⇒ 通用", is_generic_command(tokens_of("zerg gate ls")))
    check("闭集：`zerg gap ls` ⇒ 通用", is_generic_command(tokens_of("zerg gap ls")))
    check("闭集：`zerg help dangerous` ⇒ 通用", is_generic_command(tokens_of("zerg help dangerous")))
    check("闭集：`zerg help` ⇒ 通用", is_generic_command(tokens_of("zerg help")))
    check("闭集：`zerg doctor` ⇒ 通用", is_generic_command(tokens_of("zerg doctor")))
    check("闭集：`zerg doctor --json` ⇒ 通用", is_generic_command(tokens_of("zerg doctor --json")))
    check("闭集：`zerg repo status` ⇒ 通用", is_generic_command(tokens_of("zerg repo status")))
    check("闭集：`zerg gate find check-x` ⇒ **非**通用（闭集外不咬）",
          not is_generic_command(tokens_of("zerg gate find check-x")))
    check("闭集：`zerg gate run --fast` ⇒ **非**通用", not is_generic_command(tokens_of("zerg gate run --fast")))
    check("闭集：`zerg model show <名>` ⇒ **非**通用",
          not is_generic_command(tokens_of("zerg model show m1")))
    check("闭集：`./bin/zerg gate ls`（带路径前缀）⇒ 通用（basename 剥法）",
          is_generic_command(tokens_of("./bin/zerg gate ls")))

    # ── 命令树解析（纯函数）逐格 ────────────────────────────────────────
    tree = [c.split() for c in ("gate ls", "gate find", "task ls", "help", "doctor", "gap ls", "repo status")]
    check("解析：`zerg gate find kw` ⇒ 前缀命中 `gate find`",
          resolves_in_tree(tokens_of("zerg gate find kw"), tree))
    check("解析：`zerg gate ls` ⇒ 命中 `gate ls`", resolves_in_tree(tokens_of("zerg gate ls"), tree))
    check("解析：`zerg 不存在命令abc` ⇒ **不解析**（负控 ⊥ 上一格）",
          not resolves_in_tree(tokens_of("zerg 不存在命令abc"), tree))
    check("解析：只有 `zerg` 前缀 ⇒ 不解析", not resolves_in_tree(tokens_of("zerg"), tree))

    # ── analyse + 真退码（成对负控）────────────────────────────────────
    root, ledger = _fixture(base, "mix",
                            matrix_cmds=("gate ls", "gate find", "task ls", "help"),
                            ledger_rows=[
                                {"id": "G-T1", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"},
                                {"id": "G-T2", "verify_cmd": "zerg gate find check-x",
                                 "repro_cmd": "go test ./... -run X"},
                            ])
    rep = analyse(*_load(root, ledger))
    check("混合夹具：verify 不可判 = 1（G-T1 通用 · G-T2 真判）", rep["verify"]["bad"] == 1,
          "bad=%d" % rep["verify"]["bad"])
    check("混合夹具：repro 样板 = 1（G-T1 通用 · G-T2 自由文本不判）", rep["repro"]["bad"] == 1,
          "bad=%d" % rep["repro"]["bad"])
    zero = {"verify": 0, "repro": 0}
    big = {"verify": 99, "repro": 99}
    rc, code, why = verdict_of(rep, strict=False, baseline=zero)
    check("报告档：超额（1 > 0）仍 rc=0（**不阻断**）", rc == EXIT_OK, "rc=%d" % rc)
    rc_s, code_s, _ = verdict_of(rep, strict=True, baseline=zero)
    check("--strict：超额 ⇒ rc=1（与上一格成对）", rc_s == EXIT_FAIL, "rc=%d" % rc_s)
    check("--strict：不超额（1 ≤ 99）⇒ rc=0（闭集判据非恒红）",
          verdict_of(rep, strict=True, baseline=big)[0] == EXIT_OK)

    # 负控：把样板换成真判据 ⇒ 计数归 0（证明闭集不是恒真）
    rep0 = analyse(*_load(root, ledger, rows=[
        {"id": "G-T2", "verify_cmd": "zerg gate find check-x", "repro_cmd": "go test ./... -run X"}]))
    check("负控：无样板夹具 ⇒ verify 0 · repro 0", rep0["verify"]["bad"] == 0 and rep0["repro"]["bad"] == 0)

    # ── 缺件 / 空转 ⇒ rc=2（真退码）────────────────────────────────────
    rc = main(["--no-self-test", "--root", root, "--ledger", os.path.join(root, "nope.jsonl")])
    check("缺件：账读不到 ⇒ rc=2", rc == EXIT_BLOCKED, "rc=%d" % rc)

    empty_root, empty_ledger = _fixture(base, "empty", matrix_cmds=("gate ls",), ledger_rows=[])
    rc = main(["--no-self-test", "--root", empty_root, "--ledger", empty_ledger])
    check("空转：账 0 行 ⇒ rc=2（绝不报绿）", rc == EXIT_BLOCKED, "rc=%d" % rc)

    nomx_root, nomx_ledger = _fixture(base, "nomx", matrix_cmds=("gate ls",),
                                      ledger_rows=[{"id": "G-1", "verify_cmd": "zerg gate ls"}])
    os.remove(os.path.join(nomx_root, MATRIX_REL))
    rc = main(["--no-self-test", "--root", nomx_root, "--ledger", nomx_ledger])
    check("缺件：命令树缺件 ⇒ rc=2", rc == EXIT_BLOCKED, "rc=%d" % rc)

    # ── 弃用：真仓现读**不进自检**（自检必须 hermetic · 不依赖运行期在场物）──────────
    #   真仓两态读数由人手动跑（`--root <仓根> --baseline`），✗ 不放自检里：
    #   真账由多路并发追加（append-only + flock）⇒ 自检读到半写行会假红。

    print("── check-verify-cmd-real 自检（成对负控 · 真退码）──", file=sink)
    bad = 0
    for desc, ok, extra in cases:
        print("   %s %s%s" % ("✓" if ok else "✗", desc,
                              ("（%s）" % extra) if extra and not ok else ""), file=sink)
        if not ok:
            bad += 1
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if bad == 0 else "**不过**", len(cases), bad), file=sink)
    return bad == 0


def _load(root, ledger, rows=None):
    """测试辅助：装载命令树 + 账（可覆盖账行）。"""
    tree, _mp = load_tree(root, None)
    if rows is not None:
        return rows, tree
    _r, _lp = load_ledger(ledger)
    return _r, tree


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


atexit.register(cleanup)


# ══════════════════════════════════════════════════════════════════
# 6. main
# ══════════════════════════════════════════════════════════════════
def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True, description="判据须能真判门（GAP-20260926-55）")
    ap.add_argument("--root", default=REPO_ROOT_DEFAULT)
    ap.add_argument("--ledger", default="")
    ap.add_argument("--matrix", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    ap.add_argument("--baseline", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--strict", action="store_true")
    ap.add_argument("--max-examples", type=int, default=12)
    args = ap.parse_args(argv)

    if args.json and (args.self_test or args.list_rules):
        which = "--self-test" if args.self_test else "--list-rules"
        print("✗ 用法错（rc=2）：`--json` 与 `%s` 互斥（后者只产人读文本、不产 JSON）" % which,
              file=sys.stderr)
        return EXIT_BLOCKED

    if args.list_rules:
        print_list_rules()
        return EXIT_OK

    if args.self_test:
        print("── check-verify-cmd-real 自检（成对负控 · 真退码）──")
        return EXIT_OK if self_test() else EXIT_BLOCKED

    if not args.no_self_test:
        st = sys.stderr if args.json else sys.stdout
        print("── 自检（成对负控）──", file=st)
        if not self_test(sink=st):
            print("自检未过 ⇒ 拒绝扫真账（rc=2 不给结论）", file=st)
            return EXIT_BLOCKED
        print(file=st)

    root = os.path.abspath(args.root)
    try:
        tree, matrix_path = load_tree(root, args.matrix or "")
        rows, ledger_path = load_ledger(args.ledger or "")
        rep = analyse(rows, tree)
    except Unsupported as exc:
        print("✗ 不给结论（rc=2）：%s" % exc, file=sys.stderr)
        if args.json:
            print(json.dumps({"schema": "zerg/v1", "kind": "VerifyCmdRealError",
                              "rc": EXIT_BLOCKED, "reason": str(exc)},
                             ensure_ascii=False, sort_keys=True))
        return EXIT_BLOCKED

    rc, code, reason = verdict_of(rep, args.strict)
    if args.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "VerifyCmdReal",
                          "total": rep["total"], "baseline": dict(BASELINE),
                          "verify": {"bad": rep["verify"]["bad"],
                                     "generic": len(rep["verify"]["generic"]),
                                     "unresolved": len(rep["verify"]["unresolved"]),
                                     "items": rep["verify"]["generic"] + rep["verify"]["unresolved"]},
                          "repro": {"bad": rep["repro"]["bad"],
                                    "generic": len(rep["repro"]["generic"]),
                                    "items": rep["repro"]["generic"]},
                          "strict": bool(args.strict),
                          "rc": rc, "code": code, "reason": reason},
                         ensure_ascii=False, sort_keys=True))
        print("rc=%d · %s" % (rc, reason), file=sys.stderr)
    else:
        print_report(rep, ledger_path, matrix_path, args.max_examples)
        if args.baseline:
            print_baseline(rep)
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

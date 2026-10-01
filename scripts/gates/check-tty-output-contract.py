#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-tty-output-contract.py —— 输出契约 · TTY / 非 TTY **两态**（R-15）

真源（**逐字**，不转述）
----------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:72`（`T11`）逐字：
  「`R-15` **只在输出契约那一步**判 TTY/非 TTY 两态（`--json` 面两跑**逐字相同**；
    跑两遍的步数 **≤1**）」
`调研-缺口-门禁证据-20260923.md:128`（`R-15` 行）逐字：
  「TTY / 非 TTY 两态输出的差异是**经典面**（`isatty` 分支）；本仓源件逐字「`script -q /dev/null …`」
    就是它的手搓法」·「【**条件性**】只在**输出契约**那一步判两态；**不许**让全量档为此跑两遍（成本 × 2）」·
    「只读：`script -q /dev/null` 与非 TTY 两跑，`--json` 面**逐字相同**（TTY 差异只允许出现在人面）；
    跑两遍的步数 ≤ 1」

三条硬规矩（本件就是照这三条做的）
----------------------------------
1. **只判一个面**：`--json` 面（stdout）。**人面**（stderr / 提示行）两态可以不同 —— 本件**不判**它。
2. **跑两遍的步数 ≤ 1**：本件一次只判**一个**目标（判据表的默认目标 = 1 条）；给 2 条以上 ⇒ **rc=2**
   （拒绝把全量档拉去跑两遍 —— 成本 × 2 是写明不许的）。
3. **真 TTY**：TTY 侧一律走 `script -q <transcript> sh -c '<目标> 2><err>'`
   —— 目标进程的 **stdout 是 PTY**（`isatty(1)=1`），`stderr` 落文件（两态同形）。
   本件**自证**这一点：两侧各写一枚 `[ -t 1 ]` 探针文件；TTY 侧不是 1 / 非 TTY 侧不是 0 ⇒ **rc=2**（不给结论）。

归一（**只有两条 · 都是 PTY 层的，不是目标进程的行为**）
--------------------------------------------------------
  `N1` `script` 自己写进 transcript 的 `^D\\b\\b` 前缀（`script` 的收尾回显）
  `N2` PTY 的 `ONLCR`：`\\n` → `\\r\\n`
除这两条以外的**任何**字节差异 ⇒ **命中**（`rc=1`）—— 归一是**有上界的**，不是「跑一遍洗一遍」。
输出里**同时**给：`原始字节` 判等结果 · `归一后` 判等结果 · 命中时的首个差异偏移与原样上下文。

退码（三档）
------------
  0 = `--json` 面归一后**逐字相同**（且 rc 同值）
  1 = **有命中**（归一后仍不同 ⇒ 逐字差异点名；或两侧退码不同）
  2 = **不给结论**：目标跑不出来 / 两侧空转（零字节）/ 探针证不出真 TTY / 一次给了 >1 个目标 / 自检不过

用法
----
    python3 scripts/gates/check-tty-output-contract.py                      # 默认目标 = 输出契约档那一条
    python3 scripts/gates/check-tty-output-contract.py --target-cmd '<命令>'  # 换目标（**一次只许 1 条**）
    python3 scripts/gates/check-tty-output-contract.py --list-rules
    python3 scripts/gates/check-tty-output-contract.py --self-test         # 合成目标 · 含 must-fail 真红
    python3 scripts/gates/check-tty-output-contract.py --json rc,equal_raw,equal_norm

**不接闸**（本批：波③ 只产件、不接线 ✗）：未接进 `scripts/gates/precommit-gates.sh`；挂闸留给波④收口。
本件**不进**任何时候都跑的档 —— 它自己要跑两遍目标，只该挂在「输出契约」那一步上（判据原样）。
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))

# ── 判据表：**默认只一条**（「跑两遍的步数 ≤ 1」是判据，不是建议）────────────────────
DEFAULT_TARGETS = [
    {
        "id": "OC1",
        "name": "输出契约档 · `--json` 六键包封（只读 · 本机状态）",
        "cmd": "bin/zerg repo status --json head",
        "why": "它走完整的六键包封（`schema/kind/items/meta/warnings/truncated`）+ 人面在 stderr ⇒ "
               "正好把「机器面 vs 人面」两态分开：`--json` 面必须两态逐字相同，人面允许不同。",
    },
]
MAX_TARGETS = 1  # **判据「跑两遍的步数 ≤ 1」的机械落点**：一次跑的目标数上限

N1_PREFIX = b"^D\x08\x08"   # script 自己的回显前缀
TIMEOUT = 120


def run_pair(cmd, timeout=TIMEOUT):
    """跑两态 → dict（**同一份 harness 供自检与真目标用**）。"""
    d = tempfile.mkdtemp(prefix="zerg-tty-")
    try:
        cap_n, cap_t = os.path.join(d, "cap_nontty"), os.path.join(d, "cap_tty")
        err_n, err_t = os.path.join(d, "err_nontty"), os.path.join(d, "err_tty")
        mk_n, mk_t = os.path.join(d, "isatty_nontty"), os.path.join(d, "isatty_tty")
        res = {"dir": d, "cmd": cmd}

        # 非 TTY：stdout → 文件（isatty(1)=0）· stderr → 文件
        inner_n = "%s >%s 2>%s; [ -t 1 ] && echo 1 >%s || echo 0 >%s" % (cmd, cap_n, err_n, mk_n, mk_n)
        # TTY：`script -q` 给目标进程一个 PTY ⇒ stdout 是 PTY（isatty(1)=1）· stderr 仍落文件
        inner_t = "%s 2>%s; [ -t 1 ] && echo 1 >%s || echo 0 >%s" % (cmd, err_t, mk_t, mk_t)

        t0 = time.time()
        try:
            pn = subprocess.run(["sh", "-c", inner_n], capture_output=True, timeout=timeout, cwd=REPO_ROOT)
        except subprocess.TimeoutExpired:
            return {**res, "note": "非 TTY 跑超时（>%ds）" % timeout}
        except OSError as e:
            return {**res, "note": "非 TTY 跑不起来：%s" % e}

        if shutil.which("script") is None:
            return {**res, "note": "`script` 不在 PATH ⇒ 造不出真 TTY（缺件不给结论）"}
        try:
            pt = subprocess.run(["script", "-q", cap_t, "sh", "-c", inner_t],
                                capture_output=True, timeout=timeout, cwd=REPO_ROOT)
        except subprocess.TimeoutExpired:
            return {**res, "note": "TTY 跑超时（>%ds）" % timeout}
        except OSError as e:
            return {**res, "note": "TTY 跑不起来：%s" % e}

        def rd(p):
            try:
                return open(p, "rb").read()
            except OSError:
                return None

        out_n, out_t = rd(cap_n), rd(cap_t)
        if out_n is None or out_t is None:
            return {**res, "note": "一侧没落 transcript（缺件）"}
        return {**res,
                "out_n": out_n, "out_t": out_t,
                "err_n": rd(err_n) or b"", "err_t": rd(err_t) or b"",
                "rc_n": pn.returncode, "rc_t": pt.returncode,
                "isatty_n": (rd(mk_n) or b"").strip().decode() or "取不到",
                "isatty_t": (rd(mk_t) or b"").strip().decode() or "取不到",
                "secs": time.time() - t0}
    finally:
        shutil.rmtree(d, ignore_errors=True)


def normalize_tty(raw):
    """TTY 侧归一：只剥 PTY 层那两条（`N1` `script` 前缀 · `N2` `ONLCR`）。返回 (bytes, 用了哪几条)。"""
    used = []
    b = raw
    if b.startswith(N1_PREFIX):
        b = b[len(N1_PREFIX):]
        used.append("N1 script 前缀 ^D\\b\\b")
    if b"\r\n" in b:
        b = b.replace(b"\r\n", b"\n")
        used.append("N2 PTY ONLCR（\\r\\n → \\n）")
    return b, used


def first_diff(a, b):
    """首个差异偏移 + 两侧原样上下文（不猜、不概括）。"""
    n = min(len(a), len(b))
    i = 0
    while i < n and a[i] == b[i]:
        i += 1
    lo = max(0, i - 30)
    return {"offset": i, "a_len": len(a), "b_len": len(b),
            "a_ctx": a[lo:i + 30].decode("utf-8", "replace"),
            "b_ctx": b[lo:i + 30].decode("utf-8", "replace")}


def judge(run):
    """三档判定（纯函数 · 自检与真目标同一份）。"""
    note = run.get("note")
    if note:
        return 2, ["不给结论：" + note], {}
    if run["isatty_n"] != "0" or run["isatty_t"] != "1":
        return 2, ["不给结论：探针证不出真两态（非 TTY isatty(1)=%s · TTY isatty(1)=%s）"
                   % (run["isatty_n"], run["isatty_t"])], {}

    norm_t, used = normalize_tty(run["out_t"])
    # 空转判据看**归一后**：TTY 侧的 `script` 前缀不是目标的产出，不许拿它冒充「有输出」（零输出不算绿）
    if not run["out_n"].strip() and not norm_t.strip():
        return 2, ["不给结论：两侧 stdout 归一后都是零字节（TTY 侧只剩 `script` 前缀）⇒ 空转"], {}
    info = {"normalize_used": used,
            "equal_raw": run["out_n"] == run["out_t"],
            "equal_norm": run["out_n"] == norm_t,
            "rc_equal": run["rc_n"] == run["rc_t"], "rc_n": run["rc_n"], "rc_t": run["rc_t"]}
    hits = []
    if not info["equal_norm"]:
        d = first_diff(run["out_n"], norm_t)
        info["diff"] = d
        hits.append("`--json` 面**归一后仍不同**（首个差异在偏移 %d · 非 TTY %d 字节 / TTY %d 字节）"
                    % (d["offset"], d["a_len"], d["b_len"]))
        hits.append("   非 TTY：%r" % d["a_ctx"])
        hits.append("   TTY   ：%r" % d["b_ctx"])
    if not info["rc_equal"]:
        hits.append("两侧**退码不同**（非 TTY rc=%d · TTY rc=%d）—— 输出契约含退码"
                    % (run["rc_n"], run["rc_t"]))
    return (1 if hits else 0), hits, info


# ── 自检（正/负成对 · 合成目标 · 含 must-fail 真红）────────────────────────────
def self_test():
    ok, bad, log = 0, [], []

    def check(name, got, want):
        nonlocal ok
        if got == want:
            ok += 1
            log.append("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            log.append("   ✗ %s ⇒ 期望 %s，实得 %s" % (name, want, got))

    same = "printf '{\"a\":1}\\n'"
    tty_color = "if [ -t 1 ]; then printf '\\033[32m{\"a\":1}\\033[0m\\n'; else printf '{\"a\":1}\\n'; fi"
    silent = "true"

    rc, hits, info = judge(run_pair(same))
    check("正控 · 两态同形的合成目标 ⇒ rc=0", (rc, info.get("equal_norm")), (0, True))
    check("正控 · harness 真给了两态（非 TTY 0 / TTY 1）", (info.get("rc_n") is not None), True)

    rc, hits, info = judge(run_pair(tty_color))
    check("负控(must-fail) · TTY 会加 ANSI ⇒ 必须真红 rc=1", rc, 1)
    check("负控(must-fail) · 命中必须点名「归一后仍不同」",
          any("归一后仍不同" in h for h in hits), True)
    check("负控(must-fail) · 该例 `raw` 与 `norm` 都该不同",
          (info.get("equal_raw"), info.get("equal_norm")), (False, False))

    rc, hits, info = judge(run_pair(silent))
    check("空转 · 零字节两边 ⇒ 不给结论 rc=2", rc, 2)

    # 探针证不出两态 ⇒ 不给结论（**合成**：直接把 isatty 读数改坏，证明判据有牙）
    bad_run = {**run_pair(same), "isatty_t": "0"}
    rc, hits, info = judge(bad_run)
    check("负控 · 假 TTY（探针说 isatty(1)=0）⇒ 不给结论 rc=2", rc, 2)

    check("判据 · 一次跑两遍的目标数上限 = 1（「跑两遍的步数 ≤1」）", MAX_TARGETS, 1)
    check("判据表 · 默认目标恰好 1 条", len(DEFAULT_TARGETS), 1)

    for ln in log:
        print(ln)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print("输出契约 · TTY/非 TTY 两态（R-15）· 真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:128")
    print("判据：`--json` 面两跑**逐字相同**（TTY 差异只允许在人面）· 跑两遍的步数 **≤1**")
    print("TTY 侧手搓法（本仓源件逐字）：`script -q /dev/null …` ⇒ 本件用 `script -q <transcript> sh -c '<目标> 2><err>'`")
    print("归一白名单（只两条 · 都是 PTY 层）：N1 `script` 前缀 · N2 ONLCR（\\r\\n→\\n）")
    print("一次跑的目标数上限：%d（给多了 ⇒ rc=2，拒绝把成本 ×2）" % MAX_TARGETS)
    for t in DEFAULT_TARGETS:
        print("  %s %s\n     命令：%s\n     为什么：%s" % (t["id"], t["name"], t["cmd"], t["why"]))
    print("退码：0 逐字相同 · 1 有命中 · 2 不给结论（跑不出/空转/假 TTY/目标>1/自检不过）")
    return 0


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['rc', 'target', 'cmd', 'step_runs', 'max_targets', 'equal_raw', 'equal_norm', 'normalize_used', 'rc_n', 'rc_t', 'hits', 'note']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--target-cmd", action="append", default=[],
                    help="换目标（可重复出现；**但一次只许 1 条** —— 多给 ⇒ rc=2）")
    ap.add_argument("--json", default="")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        return list_rules()
    if a.self_test:
        print("── 输出契约 TTY 两态（R-15）· 自检（合成目标 · 含 must-fail 真红）──")
        return self_test()

    targets = [{"id": "OC%d" % (i + 1), "name": "命令行给定", "cmd": c, "why": "—"}
               for i, c in enumerate(a.target_cmd)] or DEFAULT_TARGETS
    if len(targets) > MAX_TARGETS:
        print("不给结论：一次给了 **%d** 个目标 ⇒ 会把「跑两遍」的步数从 1 顶到 %d"
              % (len(targets), len(targets)))
        print("判据逐字：跑两遍的步数 **≤1**（不许让全量档为此跑两遍 · 成本 × 2）")
        print("⇒ rc=2")
        return 2

    t = targets[0]
    print("── 输出契约 · TTY/非 TTY 两态（R-15）──")
    print("目标：%s %s" % (t["id"], t["name"]))
    print("命令：%s（cwd=%s）" % (t["cmd"], REPO_ROOT))
    print("为什么：%s" % t["why"])
    print("跑两遍的步数：1（判据上限 %d）" % MAX_TARGETS)
    run = run_pair(t["cmd"])
    rc, hits, info = judge(run)

    if run.get("note"):
        print("")
        print("   ✗ " + run["note"])
    else:
        print("")
        print("── 两态实测（现跑）──")
        print("   非 TTY：stdout %d 字节 · stderr %d 字节 · rc=%d · 探针 isatty(1)=%s"
              % (len(run["out_n"]), len(run["err_n"]), run["rc_n"], run["isatty_n"]))
        print("   TTY   ：stdout %d 字节 · stderr %d 字节 · rc=%d · 探针 isatty(1)=%s"
              % (len(run["out_t"]), len(run["err_t"]), run["rc_t"], run["isatty_t"]))
        print("   归一用了：%s" % ("、".join(info.get("normalize_used") or []) or "（无 —— 本来就是逐字同）"))
        print("   原始字节判等：%s（PTY 层必然改行尾 ⇒ 这一格**只作参照**）"
              % ("相同" if info.get("equal_raw") else "不同"))
        print("")
        print("── 判据：`--json` 面（stdout）归一后逐字相同 ──")
        print("   %s" % ("✓ 逐字相同（TTY 差异不在机器面）" if info.get("equal_norm")
                         else "✗ 归一后仍不同 ⇒ 命中"))
        for h in hits:
            print("   ✗ " + h)
        if not info.get("rc_equal"):
            print("   （附加判据：两侧退码 %d / %d）" % (info["rc_n"], info["rc_t"]))
    print("")
    print("⇒ rc=%d" % rc)

    if a.json:
        want = [f.strip() for f in a.json.split(",") if f.strip()]
        obj = {"rc": rc, "target": t["id"], "cmd": t["cmd"], "step_runs": 1,
               "max_targets": MAX_TARGETS, "equal_raw": info.get("equal_raw"),
               "equal_norm": info.get("equal_norm"), "normalize_used": info.get("normalize_used"),
               "rc_n": info.get("rc_n"), "rc_t": info.get("rc_t"), "hits": hits,
               "note": run.get("note", "")}
        print(json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main())

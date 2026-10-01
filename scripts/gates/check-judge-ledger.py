#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-judge-ledger.py —— 判据运行台账**读侧**对拍（M1-08 读侧 · 只读判据件 · fail-closed）

台账真源：`scripts/gates/judge-run-ledger.jsonl`（**append-only** · 一行 = 一次转发，无论成败）。
六字段（逐行 JSON 必含 · 缺任一 ⇒ 红）：`raw_rc` · `detail` · `sha256` · `head` ·
`judge_resolved` · `dry_run`（末格是 ledger 行自己的「这一趟是不是干跑」标记 —— 本件**只用它
当被检字段名**，不是另立一套干跑语义；真跑/干跑语义见 `core/internal/contract/dryrun-semantics.json`
的 `dryrun.v1`）。

★ **本件只读**：不建台账、不追加、不改一个字节（台账的**写入方**不在本件 —— 属壳侧写点）。
★ **台账不存在 / 空件 / 字段不全 ⇒ 显式退 2（不给结论）**，✗ 不得当绿（空转 = 假绿）。

硬边（红 · rc=1）：
  · 逐行缺六字段之一；
  · `raw_rc` **被顶替退码** —— 缺 `raw_rc` 却拿 `rc` / `exit_code` / `code` / `shell_rc` 顶格
    （= 把「壳退码」写进「子件原样退码」这一列 ⇒ 两义同形）：`raw_rc` 与壳退码**分列**，不许同形；
  · `--baseline <仓外快照>` 给了时，快照里任一旧行在现读台账里找不到（**旧行被改 / 被删** ⇒
    违反 append-only）。

退码（三档 · 全用现读自有号 · ✗ 不新占号）：
  0 = 对拍绿（六字段齐 · 无顶替 · append-only 成立）
  1 = 红（缺字段 / `raw_rc` 顶退码 / 旧行被改删）
  2 = **不给结论**（台账读不到 / 空件 / 用法错 / `--self-test` 不过）

用法：
    python3 scripts/gates/check-judge-ledger.py --check [--ledger <件>] [--baseline <仓外快照>]
    python3 scripts/gates/check-judge-ledger.py --self-test
"""
import argparse
import json
import os
import sys
import tempfile

LEDGER_DEFAULT = "scripts/gates/judge-run-ledger.jsonl"
REQUIRED = ["raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"]
# 「壳退码」的别名 —— 出现在行里而 `raw_rc` 缺席 ⇒ 判 `raw_rc` 被顶替（硬边）。
RC_ALIASES = ["rc", "exit_code", "code", "shell_rc", "shell_exit_code"]

# 仓根 = 本件（scripts/gates/xxx.py）上溯三层。
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

RC_OK = 0
RC_RED = 1
RC_NO_VERDICT = 2


def read_ledger(path):
    """读台账 ⇒ (rows, err)。err 非空 = 读不到（不给结论）。"""
    if not os.path.isfile(path):
        return None, "台账不存在：%s" % path
    rows = []
    try:
        with open(path, encoding="utf-8") as f:
            for lineno, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                try:
                    rows.append((lineno, json.loads(line)))
                except ValueError as e:
                    rows.append((lineno, {"__parse_error__": str(e)}))
    except OSError as e:
        return None, "台账读不到：%s" % e
    return rows, None


def problem_list(rows, raw_lines=None, baseline_lines=None):
    """逐行列面 + 顶替 + append-only ⇒ 问题清单（空 = 绿）。"""
    problems = []
    for lineno, obj in rows:
        if "__parse_error__" in obj:
            problems.append("第 %d 行 JSON 解析失败：%s" % (lineno, obj["__parse_error__"]))
            continue
        if not isinstance(obj, dict):
            problems.append("第 %d 行不是 JSON 对象：%r" % (lineno, obj))
            continue
        missing = [k for k in REQUIRED if k not in obj]
        if "raw_rc" in missing:
            aliases = [a for a in RC_ALIASES if a in obj]
            if aliases:
                problems.append("第 %d 行 **`raw_rc` 被顶替**（硬边 · `raw_rc` 与壳退码须分列，"
                                "不许同形）：用 %s 顶格" % (lineno, " / ".join(aliases)))
            else:
                problems.append("第 %d 行缺必填字段 `raw_rc`" % lineno)
        for k in missing:
            if k != "raw_rc":
                problems.append("第 %d 行缺必填字段 `%s`" % (lineno, k))
        if "raw_rc" in obj and not isinstance(obj["raw_rc"], int) or isinstance(obj.get("raw_rc"), bool):
            problems.append("第 %d 行 `raw_rc` 不是整数：%r" % (lineno, obj.get("raw_rc")))
        if "judge_resolved" in obj and not isinstance(obj["judge_resolved"], bool):
            problems.append("第 %d 行 `judge_resolved` 不是布尔：%r" % (lineno, obj.get("judge_resolved")))
        if "dry_run" in obj and not isinstance(obj["dry_run"], bool):
            problems.append("第 %d 行 `dry_run` 不是布尔：%r" % (lineno, obj.get("dry_run")))

    # append-only：基线（仓外快照）里的每一行都必须在现读台账里按序出现
    if baseline_lines is not None and raw_lines is not None:
        have = set()
        for line in raw_lines:
            have.add(line)
        for i, bl in enumerate(baseline_lines, 1):
            if bl not in have:
                problems.append("append-only 破（基线第 %d 行在现读台账里找不到 ⇒ 旧行被改/被删）"
                                % i)
    return problems


def load_lines(path):
    try:
        with open(path, encoding="utf-8") as f:
            return [l.strip() for l in f if l.strip()], None
    except OSError as e:
        return None, "基线读不到：%s" % e


def cmd_check(ledger, baseline):
    rows, err = read_ledger(ledger)
    if err:
        print("⛔ %s ⇒ **不给结论（rc=2）**" % err, file=sys.stderr)
        return RC_NO_VERDICT
    if not rows:
        print("⛔ 台账**空件/无有效行**（%s）⇒ **不给结论（rc=2）** ✗ 不当绿" % ledger, file=sys.stderr)
        return RC_NO_VERDICT

    raw_lines, err2 = load_lines(ledger)
    if err2:
        print("⛔ %s ⇒ 不给结论（rc=2）" % err2, file=sys.stderr)
        return RC_NO_VERDICT

    baseline_lines = None
    if baseline:
        baseline_lines, berr = load_lines(baseline)
        if berr:
            print("⛔ %s ⇒ 不给结论（rc=2）" % berr, file=sys.stderr)
            return RC_NO_VERDICT

    problems = problem_list(rows, raw_lines, baseline_lines)
    if problems:
        print("✗ 判据运行台账对拍**红**（%d 条）：" % len(problems), file=sys.stderr)
        for p in problems:
            print("  · " + p, file=sys.stderr)
        return RC_RED
    print("✓ 判据运行台账对拍绿（rc=0）：行 %d · 六字段齐 · append-only 成立%s"
          % (len(rows), "（基线 %s）" % baseline if baseline else ""))
    return RC_OK


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def _row(**kw):
    base = {"raw_rc": 0, "detail": "ok", "sha256": "a" * 64, "head": "6aa1120e",
            "judge_resolved": True, "dry_run": False}
    base.update(kw)
    return json.dumps(base, ensure_ascii=False)


def cmd_self_test():
    """成对负控：正控绿 ＋ 五条负控各自必红 / 必不给结论（✗ 负控禁恒绿）。"""
    ok = True
    with tempfile.TemporaryDirectory(prefix="judge-ledger-self-") as d:
        led = os.path.join(d, "judge-run-ledger.jsonl")

        def run():
            return _call(led, None)

        # ── 正控：两行合法（六字段齐）⇒ rc=0 ────────────────────
        _write(led, _row() + "\n" + _row(raw_rc=1, detail="judge_red") + "\n")
        rc = run()
        if rc == RC_OK:
            print("✓ 正控：两行合法 ⇒ rc=0")
        else:
            print("✗ 正控：合法台账被判 rc=%d" % rc)
            ok = False

        # ── 负控①：缺字段（抹 detail）⇒ rc=1 ────────────────────
        bad = json.loads(_row())
        bad.pop("detail")
        _write(led, json.dumps(bad, ensure_ascii=False) + "\n")
        rc = run()
        if rc == RC_RED:
            print("✓ 负控①（缺 `detail` 字段）：现 rc=1（红）")
        else:
            print("✗ 负控①（缺字段）：该红没红 ⇒ rc=%d" % rc)
            ok = False

        # ── 负控②：`raw_rc` 被 `rc` 顶替（硬边）⇒ rc=1 ──────────
        bad = json.loads(_row())
        bad["rc"] = bad.pop("raw_rc")
        _write(led, json.dumps(bad, ensure_ascii=False) + "\n")
        rc = run()
        if rc == RC_RED:
            print("✓ 负控②（`raw_rc` 被 `rc` 顶替）：现 rc=1（红 · 硬边成立）")
        else:
            print("✗ 负控②（顶替退码）：该红没红 ⇒ rc=%d" % rc)
            ok = False

        # ── 负控③：空台账 ⇒ rc=2（不给结论 ✗ 不当绿）────────────
        _write(led, "")
        rc = run()
        if rc == RC_NO_VERDICT:
            print("✓ 负控③（空台账）：现 rc=2（不给结论 · ✗ 不当绿）")
        else:
            print("✗ 负控③（空台账）：该不给结论却没给 ⇒ rc=%d" % rc)
            ok = False

        # ── 负控④：台账不存在 ⇒ rc=2 ────────────────────────────
        rc = _call(os.path.join(d, "absent.jsonl"), None)
        if rc == RC_NO_VERDICT:
            print("✓ 负控④（台账不存在）：现 rc=2（不给结论）")
        else:
            print("✗ 负控④（台账不存在）：该不给结论却没给 ⇒ rc=%d" % rc)
            ok = False

        # ── 负控⑤：旧行被改 / 被删（对基线）⇒ rc=1 ──────────────
        baseline = os.path.join(d, "baseline.jsonl")
        _write(baseline, _row(sha256="b" * 64) + "\n")
        _write(led, _row(sha256="c" * 64) + "\n")
        rc = _call(led, baseline)
        if rc == RC_RED:
            print("✓ 负控⑤（对基线：旧行被改/被删）：现 rc=1（append-only 破）")
        else:
            print("✗ 负控⑤（旧行被改删）：该红没红 ⇒ rc=%d" % rc)
            ok = False

    return RC_OK if ok else RC_NO_VERDICT


def _call(ledger, baseline):
    return cmd_check(ledger, baseline)


def main(argv=None):
    ap = argparse.ArgumentParser(description="判据运行台账读侧对拍（M1-08 读侧 · fail-closed）")
    ap.add_argument("--root", default=REPO, help="仓根（默认 = 本件上溯三层）")
    g = ap.add_mutually_exclusive_group()
    g.add_argument("--check", action="store_true", help="台账六字段 + append-only 对拍")
    g.add_argument("--self-test", dest="self_test", action="store_true", help="成对负控自检")
    ap.add_argument("--ledger", default=LEDGER_DEFAULT, help="台账件（默认 %s）" % LEDGER_DEFAULT)
    ap.add_argument("--baseline", default=None, help="仓外快照（append-only 对拍 · 可选）")
    args = ap.parse_args(argv)

    root = os.path.abspath(args.root)
    if args.self_test:
        return cmd_self_test()
    if args.check:
        ledger = args.ledger if os.path.isabs(args.ledger) else os.path.join(root, args.ledger)
        baseline = args.baseline
        if baseline and not os.path.isabs(baseline):
            baseline = os.path.join(root, baseline)
        return cmd_check(ledger, baseline)
    sys.stderr.write("用法错：`--check` / `--self-test` 二选一必给一个（都不给 ⇒ rc=2）\n")
    return RC_NO_VERDICT


if __name__ == "__main__":
    sys.exit(main())

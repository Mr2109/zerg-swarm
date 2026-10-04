#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-selftest-knobs.py —— 「提交链上的门不接受关自检旋钮」只读元门（缺口 `GAP-20260927-74`）

题面（缺口 `want_action` 逐字）：
    **提交链上的门一律不接受「关自检」型旋钮**，或把它们改为需要显式且打印的
    `--selftest-off` 旗标（并在输出里申报）。

病象（缺口 `GAP-20260927-74`）：
    `CHECK_DOC_NAME_META_OFF=1` 可静默关掉 `check-doc-name` 的元自检层，而该门在**提交链里**；
    另有 `ZERG_ALLSH_UNITTEST` / `ZERG_REAL_GATES_UNITTEST` 让两个包装脚本改走自检夹具路并**据此给 rc**。
    ⇒ 门在提交链上是否真有牙，被一个**仓外环境变量**静默改写，且**不留痕**。

口径（写死 —— 本件的判据）：
    · 扫面 = 提交链门的脚本面（`scripts/gates/*.py` · `*.sh`）＋ Go 真身
      （`core/cmd/zerg/*.go`）；可用 `--root` 换根（自检用）。
    · 「关自检型旋钮」形状（正则，内容锚**不用行号**）：
        ① `<NAME>` 里含 `UNITTEST` / `SELFTEST` / `SELFTEST_OFF` / `META_OFF` / `_NO_PERL`
           并 `os.Getenv`/`os.environ.get`/`${NAME:-…}`/`[ -n "$NAME" ]` 形态消费；
        ② 消费点**在提交链步的件**（`precommit-gates.sh` 里 `add_step` 出现过该件名）
           ⇒ 该旋钮是「提交链旋钮」，**必须**有显式白名单理由，否则红灯点名。
    · **登记五件套**（每条被接受的豁免必须给全）：`形态 id` / `内容锚`（源码逐字片段）/
      `理由` / `出处`（§节或缺口号）/ `看着它的门` —— 缺任一 ⇒ rc=2（写不清理由的豁免就是洗白）。
    · 白名单 = 本件顶部的 `KNOB_ALLOW`（逐条给出五件套）；空即红。
    · **退码写死**：`rc=0` 全被登记 · `rc=1` 有未登记旋钮（逐条点名）· `rc=2` 缺语料/白名单不全。
    · **只读**：不写任何文件。

用法：
    python3 scripts/gates/check-selftest-knobs.py [--root <dir>] [--json] [--self-test]

★ 接线状态：本件为**仓外候件**（`/var/tmp/zerg-wave23/p0/`），未挂门步 —— 起步 `tri-report`，
  接线由后续单独一笔做，避免撞件。
"""
import argparse
import json
import os
import re
import sys

# 五件套白名单：目前为空 ⇒ 现读即红（这正是缺口要的：先点名、再逐条接线或改写）。
# 每条形如 {"id": "...", "anchor": "<源码逐字>", "reason": "...", "source": "...", "gate": "..."}
KNOB_ALLOW = []

# ── 旋钮分档（缺口 `GAP-20260927-74` · 门本体由「只报告」改**分档**）────────────────────
#   把三枚已知旋钮本体纳入分档，按「危害」分层（此前门只报「未登记」、不分档）：
#     isolated     走自检夹具路（不碰真目标、不进真 rc 判定）⇒ 危害低，登记后可留；
#     silent_off   静默关掉**真目标**的判据/元自检层 ⇒ 危害高（门在盘、看着绿，判据其实没跑）；
#     unclassified 未分档（默认）⇒ 按 silent_off 同等待遇（保守，不给未分类旋钮留口子）。
KNOB_TIERS = ("isolated", "silent_off", "unclassified")
KNOB_TIER = {
    "CHECK_DOC_NAME_META_OFF": "silent_off",  # 关 check-doc-name 的元自检层（该门在提交链里）
    "ZERG_ALLSH_UNITTEST": "isolated",        # all.sh 走自检夹具路
    "ZERG_ALLSH_NO_PERL": "isolated",         # all.sh 走无 perl 的夹具路
}


def knob_tier(name):
    return KNOB_TIER.get(name, "unclassified")

# 关自检型旋钮的「名字形态」（内容锚 = 变量名本身，不用行号）
NAME_RE = re.compile(r"\b([A-Z][A-Z0-9_]*(?:UNITTEST|SELFTEST_OFF|SELFTEST|META_OFF|NO_PERL)[A-Z0-9_]*)\b")
# 消费形态：getenv / environ.get / ${NAME:-} / [ -n "$NAME" ] / [ "$NAME" = ... ]
CONSUME_RES = [
    re.compile(r"os\.Getenv\(\s*\"([A-Z][A-Z0-9_]*)\""),
    re.compile(r"environ\.get\(\s*[\"']([A-Z][A-Z0-9_]*)[\"']"),
    re.compile(r"\$\{([A-Z][A-Z0-9_]*):-"),
    re.compile(r"\[\s*-n\s+\"\$\{?([A-Z][A-Z0-9_]*)\}?\""),
    re.compile(r"\[\s*\"\$\{?([A-Z][A-Z0-9_]*)\}?\"\s*="),
]


def iter_files(root):
    for sub in ("scripts/gates", "core/cmd/zerg"):
        d = os.path.join(root, sub)
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            if name.endswith((".py", ".sh", ".go")):
                yield os.path.join(d, name), name


def in_commit_chain(precommit_src, name):
    return precommit_src is not None and name in precommit_src


def scan(root):
    """回 (hits, precommit_present)。hits = [(file, line, name, consume)] 未登记旋钮。"""
    precommit = os.path.join(root, "scripts/gates/precommit-gates.sh")
    pc_src = None
    if os.path.isfile(precommit):
        try:
            pc_src = open(precommit, "r", encoding="utf-8").read()
        except OSError:
            pc_src = None
    allowed = {e["id"] for e in KNOB_ALLOW} | {e["anchor"] for e in KNOB_ALLOW}
    hits = []
    for path, name in iter_files(root):
        try:
            src = open(path, "r", encoding="utf-8").read().splitlines()
        except OSError:
            continue
        for i, line in enumerate(src, 1):
            nm = None
            for cr in CONSUME_RES:
                m = cr.search(line)
                if m:
                    nm = m.group(1)
                    break
            if not nm:
                m = NAME_RE.search(line)
                nm = m.group(1) if m else None
            if not nm:
                continue
            if not NAME_RE.match(nm) and not re.search(r"(UNITTEST|SELFTEST|META_OFF|NO_PERL)", nm):
                continue
            if not in_commit_chain(pc_src, name):
                continue  # 非提交链件不判（本件只管提交链上的门）
            if nm in allowed:
                continue
            hits.append((name, i, nm, line.strip()[:120]))
    return hits, pc_src is not None


def run(root, as_json):
    hits, pc = scan(root)
    if not pc:
        print("BLOCKED —— 缺 `scripts/gates/precommit-gates.sh`（无法判定提交链）⇒ rc=2", file=sys.stderr)
        return 2
    # 白名单五件套完整性
    bad_allow = []
    for e in KNOB_ALLOW:
        if not all(e.get(k) for k in ("id", "anchor", "reason", "source", "gate")):
            bad_allow.append(e.get("id", "?"))
    if bad_allow:
        print("BLOCKED —— 白名单条目不齐五件套：%s ⇒ rc=2" % ",".join(bad_allow), file=sys.stderr)
        return 2
    if as_json:
        tier_counts = {t: 0 for t in KNOB_TIERS}
        for _f, _l, n, _t in hits:
            tier_counts[knob_tier(n)] += 1
        print(json.dumps({"rc": 1 if hits else 0, "knob_count": len(hits),
                          "hits": [{"file": f, "line": l, "knob": n, "tier": knob_tier(n)}
                                   for f, l, n, _ in hits],
                          "tier_counts": tier_counts},
                         ensure_ascii=False))
    else:
        print("提交链「关自检旋钮」元门（GAP-20260927-74）")
        print("提交链上未登记的关自检型旋钮 = %d 条" % len(hits))
        for f, l, n, txt in hits:
            print("  ✗ %s:%d 旋钮 %s [档 %s] ⇒ %s" % (f, l, n, knob_tier(n), txt))
        print("  口径：提交链上的门不接受关自检型旋钮；要保留须进 KNOB_ALLOW 五件套，")
        print("        或改成显式且打印的 --selftest-off 旗标（并在输出里申报）。")
    return 1 if hits else 0


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts/gates"))
        os.makedirs(os.path.join(td, "core/cmd/zerg"))
        # 提交链步里点名 check-doc-name.py
        open(os.path.join(td, "scripts/gates/precommit-gates.sh"), "w").write(
            'add_step docs "门x" tri "$R" "python3 scripts/gates/check-doc-name.py"\n')
        open(os.path.join(td, "scripts/gates/check-doc-name.py"), "w").write(
            'meta = os.environ.get("CHECK_DOC_NAME_META_OFF") != "1"\n')
        hits, pc = scan(td)
        if not pc or [h[2] for h in hits] != ["CHECK_DOC_NAME_META_OFF"]:
            print("  ✗ 正控失败：期望命中 CHECK_DOC_NAME_META_OFF，实得 %r" % (hits,))
            ok = False
        if run(td, False) != 1:
            print("  ✗ 正控 rc 失败：应回 1")
            ok = False
        # 负控：换成非提交链件 ⇒ 0
        open(os.path.join(td, "scripts/gates/precommit-gates.sh"), "w").write("# 无 add_step\n")
        if scan(td)[0]:
            print("  ✗ 负控失败：非提交链件不该判")
            ok = False
    print("自检 %s（3 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="提交链「关自检旋钮」只读元门（GAP-20260927-74）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--by-tier", dest="by_tier", action="store_true", help="只按分档打计数（GAP-20260927-74）")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    if args.by_tier:
        hits, pc = scan(args.root)
        if not pc:
            print("BLOCKED —— 缺 precommit-gates.sh（无法判定提交链）⇒ rc=2", file=sys.stderr)
            return 2
        cnt = {t: 0 for t in KNOB_TIERS}
        for _f, _l, n, _t in hits:
            cnt[knob_tier(n)] += 1
        for t in KNOB_TIERS:
            print("%-13s %d" % (t, cnt[t]))
        return 1 if hits else 0
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

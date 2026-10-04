#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gap-closure-drift.py —— 「在册仍缺 ⟷ 落点件在盘」只读漂移门（缺口 `GAP-20260927-122`）

题面（缺口 `want_action` ② 逐字）：
    加只读门 `check-gap-closure-drift.py`：**态含「仍缺」而落点件在盘 ⇒ 判红点名**；
    缺件 ⇒ rc=2；档位（退码）**照件已用档位写死**、**不抄账里的 8**。

病象（缺口 `GAP-20260927-122`）：
    缺口账两处真源必然漂移 ⇒ 「在册文字与落地件脱钩」成族：正本 markdown 与
    `~/.zerg/state/zerg-cli-gaps.jsonl` 之间无同步面 ⇒ 某条 `state=仍缺` 而其点名落点件
    早已在盘（下一批才会被别的门偶然发现）。本件把这一格**现算**出来：只读、不写、不改态。

口径（写死 —— 本件的判据）：
    · 输入 = 真源 jsonl（默认 `~/.zerg/state/zerg-cli-gaps.jsonl`，可用 `--ledger` 换）。
    · **只判「落点件在盘」这一格**：`state` 含「仍缺」且 `unit` 是**仓内相对路径**（非
      「无件(命令面)」这类占位）且该件**在 `<root>` 下真实存在** ⇒ **判红点名**（行 id · unit）。
    · **真源读不到 / 缺件 ⇒ rc=2 不给结论**（✗ 不许当绿）。
    · **退码档位写死**：`rc=0` 无漂移 · `rc=1` 有漂移（逐条点名）· `rc=2` 缺语料。✗ 不从账里读 8。
    · **只读**：不写任何文件、不改任何 `state`（本件与 `check-gap-state-vs-artifact.py` 的分工：
      后者判「态↔证物四态」，本件只判「仍缺 ⟷ 点名件在盘」这一条最简漂移）。
    · `unit` 允许多路径（源账里以 ` ; ` 分隔）⇒ 逐段判，任一段在盘即命中。
    · 白名单：`--allow <id>` 可重复，给「有意保留在盘但账还没收口」的行一条显式降噪口。

用法：
    python3 scripts/gates/check-gap-closure-drift.py [--ledger <path>] [--root <dir>]
                                                     [--allow <id>]... [--json] [--self-test]

★ 接线状态：**已挂门101**（tri-report 只报告档）· 2026-10-04 现读更正 ——
  原「仓外候件 / 未挂进任何门步」措辞已过期（公开标记表 / precommit 一步均已落）。
"""
import argparse
import json
import os
import sys

PLACEHOLDER_UNITS = {"无件(命令面)", "无件", "", "-"}


def load_rows(ledger):
    """读真源 jsonl ⇒ (rows, err)。任一行解析失败 ⇒ err（fail-closed）。"""
    if not os.path.isfile(ledger):
        return None, "真源不存在：%s" % ledger
    rows = []
    try:
        with open(ledger, "r", encoding="utf-8") as f:
            for i, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                try:
                    rows.append(json.loads(line))
                except Exception as e:  # noqa: BLE001
                    return None, "真源第 %d 行不是合法 JSON：%s" % (i, e)
    except OSError as e:
        return None, "真源读不到：%s" % e
    return rows, None


def units_of(row):
    """把 `unit` 拆成仓内相对路径列表（去占位 / 去空）。"""
    raw = (row.get("unit") or "").strip()
    if raw in PLACEHOLDER_UNITS:
        return []
    out = []
    for seg in raw.split(";"):
        seg = seg.strip()
        if not seg or seg in PLACEHOLDER_UNITS:
            continue
        out.append(seg)
    return out


def drift_rows(rows, root, allow):
    """回 [(id, unit_on_disk)] —— 仍缺且点名件在盘（不在 allow 里）。"""
    hits = []
    for r in rows:
        state = str(r.get("state") or "")
        if "仍缺" not in state:
            continue
        rid = r.get("id") or "?"
        if rid in allow:
            continue
        for u in units_of(r):
            # 只认仓内相对路径（不含前导 / 的仓外绝对路径不判 —— 那不是「仓内落件」）
            if u.startswith("/") or u.startswith("~"):
                continue
            p = os.path.join(root, u)
            if os.path.isfile(p) or os.path.isdir(p):
                hits.append((rid, u))
    return hits


def run(ledger, root, allow, as_json):
    rows, err = load_rows(ledger)
    if err is not None:
        print("BLOCKED —— %s ⇒ rc=2 不给结论" % err, file=sys.stderr)
        return 2
    if not rows:
        print("BLOCKED —— 真源解析出 0 行（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    hits = drift_rows(rows, root, allow)
    if as_json:
        print(json.dumps({"rc": 1 if hits else 0, "drift_count": len(hits),
                          "drift": [{"id": i, "unit": u} for i, u in hits]},
                         ensure_ascii=False))
    else:
        print("缺口「仍缺 ⟷ 落点件在盘」漂移门（GAP-20260927-122）")
        print("真源 %s · 读入 %d 行 · 仍缺且点名件在盘 = %d 条" % (ledger, len(rows), len(hits)))
        for i, u in hits:
            print("  ✗ %s：点名件已在盘 ⇒ %s（账滞后 or 未收口；核后改账或经 --allow 显式降噪）" % (i, u))
    return 1 if hits else 0


def self_test():
    """成对负控：造一份临时真源 + 临时 root ⇒ 命中 1 / 不命中 0。"""
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        root = os.path.join(td, "repo")
        os.makedirs(os.path.join(root, "core", "cmd", "zerg"))
        on_disk = os.path.join(root, "core", "cmd", "zerg", "landed.go")
        open(on_disk, "w").close()
        ledger = os.path.join(td, "gaps.jsonl")
        with open(ledger, "w", encoding="utf-8") as f:
            f.write(json.dumps({"id": "GAP-T-1", "state": "仍缺",
                                "unit": "core/cmd/zerg/landed.go"}) + "\n")
            f.write(json.dumps({"id": "GAP-T-2", "state": "仍缺",
                                "unit": "core/cmd/zerg/absent.go"}) + "\n")
            f.write(json.dumps({"id": "GAP-T-3", "state": "已解",
                                "unit": "core/cmd/zerg/landed.go"}) + "\n")
            f.write(json.dumps({"id": "GAP-T-4", "state": "仍缺",
                                "unit": "无件(命令面)"}) + "\n")
        # 正控：命中 1（GAP-T-1）
        h = drift_rows(load_rows(ledger)[0], root, set())
        if [x[0] for x in h] != ["GAP-T-1"]:
            print("  ✗ 正控失败：期望命中 [GAP-T-1]，实得 %r" % (h,))
            ok = False
        # 负控 A：--allow 掉 GAP-T-1 ⇒ 0
        if drift_rows(load_rows(ledger)[0], root, {"GAP-T-1"}):
            print("  ✗ 负控A失败：--allow 未生效")
            ok = False
        # 负控 B：缺真源 ⇒ rc=2
        if run(os.path.join(td, "nope.jsonl"), root, set(), False) != 2:
            print("  ✗ 负控B失败：缺语料未回 rc=2")
            ok = False
        # 正控 rc=1
        if run(ledger, root, set(), False) != 1:
            print("  ✗ 正控 rc 失败：应回 1")
            ok = False
    print("自检 %s（4 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="缺口「仍缺 ⟷ 落点件在盘」只读漂移门（GAP-20260927-122）")
    ap.add_argument("--ledger", default=os.path.expanduser("~/.zerg/state/zerg-cli-gaps.jsonl"))
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--allow", action="append", default=[])
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.ledger, args.root, set(args.allow), args.json)


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-registry-key-unique.py —— 统一「登记键唯一性」只读门（缺口 `GAP-20260926-175`）

题面（缺口逐字）：
    无统一「登记键唯一性」门：同一条机制（**命中行数==1** + 登记过期点名）散在 ≥3 件里
    各写一遍，无单命令可复跑 ⇒ 反行号化只能规定形态复用。

病象：
    同一份登记件里，同一个键（`id` / `name` / `key`）出现两次 ⇒ 读面「命中 N 次」而不是
    「恰好 1 次」，运行期取谁不定；这种重复**没有任何门**在盘上拦它，只能靠人现读。

口径（写死 —— 本件的判据）：
    · 扫面 = 逐个登记件（默认从仓根下的 `scripts/gates/` 与 `core/internal/contract/` 收
      `*.json`）。可用 `--root` 换根（自检用）。
    · 「登记件」的判定（内容锚，不用件名）：某 json 的**顶层某键的值是一个「全为对象的
      非空列表」** ⇒ 那一列就是一份登记表（顶层键名不限：`entries` / `items` /
      `decisions` / `probes` / `cases` … 皆可）。
    · 「键字段」的判定：在该列**每个对象里都出现**的第一个已知键（有序候选
      `id` → `name` → `key` → `slug` → `rule`）；一列里找不到任一候选键 ⇒ **该列无键面**，
      记「跳过（无键面）」，✗ 不判红（无键面不是重复）。
    · **判据 = 该列内键值逐字唯一**：出现 ≥2 次 ⇒ 逐条点名（键值 · 出现次数 · 该列路径）。
    · **退码写死**：`rc=0` 全唯一（或有跳过但无重复）· `rc=1` 有重复键（逐条点名）·
      `rc=2` 扫面根不存在 / 某 json 读不出或不是合法 JSON（**「取不到」与「零重复」不许同形**）。
    · **只读**：不写任何文件。

用法：
    python3 scripts/gates/check-registry-key-unique.py [--root <dir>] [--json] [--self-test]

★ 接线状态：**已挂门106**（`scripts/gates/precommit-gates.sh` 的 `add_step` · tri-report 只报告档）·
  2026-10-05 现读更正 —— 原「未接线 / 仓外候件」措辞已过期（接线早已落）。本件自身 `--self-test` 已实跑 rc=0。
"""
import argparse
import json
import os
import sys

# 已知键候选（有序）：取「该列每个对象里都出现」的第一个。
# ★ 只收**标识符类**字段；`rule` / `语义` / `判据` 一类**描述性文本**不是键（两条登记
#   共用同一句说明是合法的 ⇒ 拿它当键会误报，实测在 ai-boundary.json[file_drop_doors] 上
#   就误报过一次），故**不收**。
KEY_CANDIDATES = ("id", "name", "key", "slug")
SCAN_DIRS = ("scripts/gates", "core/internal/contract")


def load_json(path):
    """回 (obj, err|None)。"""
    try:
        with open(path, "r", encoding="utf-8") as fh:
            return json.load(fh), None
    except (IOError, OSError) as exc:
        return None, "读不出：%s" % exc
    except ValueError as exc:
        return None, "不是合法 JSON：%s" % exc


def entry_lists(obj):
    """从一个 json 顶层取「全为对象的非空列表」的列 ⇒ [(top_key, list)]。"""
    out = []
    if not isinstance(obj, dict):
        return out
    for k, v in obj.items():
        if isinstance(v, list) and v and all(isinstance(x, dict) for x in v):
            out.append((k, v))
    return out


def key_field(entries):
    """回该列用的键字段；找不到 ⇒ None（无键面）。"""
    for cand in KEY_CANDIDATES:
        if all(cand in e for e in entries):
            return cand
    return None


def scan(root):
    """回 (dups, skipped, bad, scanned)。dups=[(file, topkey, field, value, count)]；
    skipped=[(file, topkey)]；bad=[(file, err)]；
    scanned = 现读的登记件（*.json）件数（= 扫面取到的输入 · 供空集守卫）。"""
    dups, skipped, bad, scanned = [], [], [], 0
    for sub in SCAN_DIRS:
        d = os.path.join(root, sub)
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            if not name.endswith(".json"):
                continue
            scanned += 1
            fpath = os.path.join(d, name)
            obj, err = load_json(fpath)
            if err:
                bad.append((os.path.join(sub, name), err))
                continue
            for topkey, entries in entry_lists(obj):
                field = key_field(entries)
                if field is None:
                    skipped.append((os.path.join(sub, name), topkey))
                    continue
                seen = {}
                for e in entries:
                    val = e.get(field)
                    if isinstance(val, list):
                        val = json.dumps(val, ensure_ascii=False, sort_keys=True)
                    seen.setdefault(val if isinstance(val, str) else repr(val), 0)
                    seen[val if isinstance(val, str) else repr(val)] += 1
                for val, cnt in sorted(seen.items()):
                    if cnt >= 2:
                        dups.append((os.path.join(sub, name), topkey, field, val, cnt))
    return dups, skipped, bad, scanned


def run(root, as_json):
    if not os.path.isdir(os.path.join(root, "scripts/gates")) and \
       not os.path.isdir(os.path.join(root, "core/internal/contract")):
        if as_json:
            print(json.dumps({"rc": 2, "error": "扫面根不存在（scripts/gates 与 core/internal/contract 皆无）"},
                             ensure_ascii=False))
        else:
            print("BLOCKED —— 扫面根不存在（缺 scripts/gates 与 core/internal/contract）⇒ rc=2，不给结论",
                  file=sys.stderr)
        return 2
    dups, skipped, bad, scanned = scan(root)
    # ★ 空集守卫（GAP-20260927-108）：空域 ⇒ fail-closed（扫面 0 件登记件 ⇒ rc=2，不许把「空转」当绿）
    if not scanned:
        if as_json:
            print(json.dumps({"rc": 2, "error": "空域：%s 下 0 个 *.json 登记件（取不到输入 ⇒ 空转 ≠ 零重复）"
                              % "、".join(SCAN_DIRS)}, ensure_ascii=False))
        else:
            print("BLOCKED —— 空域：扫面（%s）下 0 个 *.json 登记件（取不到输入）⇒ rc=2 不给结论（空转 ≠ 零重复）"
                  % "、".join(SCAN_DIRS), file=sys.stderr)
        return 2
    if bad:
        if as_json:
            print(json.dumps({"rc": 2, "bad": [{"file": f, "err": e} for f, e in bad]},
                             ensure_ascii=False))
        else:
            print("BLOCKED —— 登记件读不出/非法 JSON（「取不到」与「零重复」不许同形）⇒ rc=2：",
                  file=sys.stderr)
            for f, e in bad:
                print("    %s ← %s" % (f, e), file=sys.stderr)
        return 2
    rc = 1 if dups else 0
    if as_json:
        print(json.dumps({"rc": rc, "dup_count": len(dups),
                          "dups": [{"file": f, "list": t, "field": fld, "value": v, "count": c}
                                   for f, t, fld, v, c in dups],
                          "skipped_no_key": [{"file": f, "list": t} for f, t in skipped]},
                         ensure_ascii=False))
    else:
        print("统一「登记键唯一性」门（GAP-20260926-175）")
        print("重复键 = %d 条" % len(dups))
        for f, t, fld, v, c in dups:
            print("  ✗ %s[%s] 键 %s=%s 出现 %d 次（要求恰好 1 次）" % (f, t, fld, v, c))
        if skipped:
            print("  跳过（无键面 · 非重复）：%d 列" % len(skipped))
            for f, t in skipped:
                print("    · %s[%s]" % (f, t))
        print("  口径：一份登记件里同一个键只许出现一次（命中行数==1）；重复即判红并点名。")
    return rc


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts/gates"))
        os.makedirs(os.path.join(td, "core/internal/contract"))
        # 正控：一份登记件里 id 重复 ⇒ rc=1，且点名到重复键
        open(os.path.join(td, "scripts/gates/x-registry.json"), "w", encoding="utf-8").write(json.dumps(
            {"schema": "s", "entries": [{"id": "a", "p": 1}, {"id": "b"}, {"id": "a"}]},
            ensure_ascii=False))
        dups, skipped, bad, _scanned = scan(td)
        if bad or len(dups) != 1 or dups[0][3] != "a" or dups[0][4] != 2:
            print("  ✗ 正控失败：期望 a 出现 2 次，实得 %r / bad=%r" % (dups, bad))
            ok = False
        if run(td, False) != 1:
            print("  ✗ 正控 rc 失败：应回 1")
            ok = False
        # 负控一：键唯一 ⇒ rc=0
        open(os.path.join(td, "scripts/gates/x-registry.json"), "w", encoding="utf-8").write(json.dumps(
            {"entries": [{"id": "a"}, {"id": "b"}]}, ensure_ascii=False))
        if run(td, False) != 0:
            print("  ✗ 负控失败：键唯一应回 0")
            ok = False
        # 负控二：该列无已知键字段 ⇒ 跳过、不判红
        open(os.path.join(td, "core/internal/contract/y.json"), "w", encoding="utf-8").write(json.dumps(
            {"items": [{"foo": 1}, {"foo": 1}]}, ensure_ascii=False))
        if run(td, False) != 0:
            print("  ✗ 负控失败：无键面不该判红")
            ok = False
        # 负控三：非法 JSON ⇒ rc=2（取不到 ≠ 零重复）
        open(os.path.join(td, "core/internal/contract/z.json"), "w", encoding="utf-8").write("{ not json")
        if run(td, False) != 2:
            print("  ✗ 负控失败：非法 JSON 应回 2")
            ok = False
        # 负控四：空域（扫面 0 件 *.json 登记件）⇒ rc=2（空集守卫 · GAP-20260927-108）
        empty_root = os.path.join(td, "empty-root")
        os.makedirs(os.path.join(empty_root, "scripts/gates"))
        os.makedirs(os.path.join(empty_root, "core/internal/contract"))
        if scan(empty_root)[3] != 0:
            print("  ✗ 负控失败：空域应得 0 件登记件")
            ok = False
        if run(empty_root, False) != 2:
            print("  ✗ 负控失败：空域（0 件登记件）应回 2（空集守卫）")
            ok = False
    print("自检 %s（5 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="统一「登记键唯一性」只读门（GAP-20260926-175）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())

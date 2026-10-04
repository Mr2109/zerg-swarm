#!/usr/bin/env python3
# check-dev-targets.py —— 候选门（候件 · 仓外放置，未入库）
#
# 覆盖缺口 GAP-20260927-197：给 core/internal/contract/dev-targets.json 的
# 「set(ids) == set(families 展平) 且 ids == 展平序」加一条**静态断言**门。
#
# 病灶（现读）：families / source_line 是**无消费者的自由文本**（全仓 14 命中无脚本读它），
# ids 是唯一机器真源（contract.go:307-319 只解码 schema+ids）⇒ 三者可任意漂移而不报。
#
# 口径：只读 dev-targets.json；families 按固定族序（D/E/F/G）展平；要求
#   ① set(ids) == set(展平)   ② ids 逐元素 == 展平序（顺序也一致）
# 退码：0 一致 · 1 不一致（逐条点名）· 2 件读不到/解析失败/空（不给结论）
#
# 用法：python3 check-dev-targets.py [<dev-targets.json 路径>] [--self-test]
import json
import sys

FAMILY_ORDER = ("D", "E", "F", "G")


def flatten(families):
    out = []
    for fam in FAMILY_ORDER:
        for x in families.get(fam, []):
            out.append(x)
    return out


def check(path):
    try:
        with open(path, encoding="utf-8") as f:
            d = json.load(f)
    except FileNotFoundError:
        return 2, ["缺件：%s 不存在（判据不可判 ⇒ 不给结论）" % path]
    except Exception as e:  # noqa: BLE001
        return 2, ["解析失败：%s（%s）⇒ 不给结论" % (path, e)]
    ids = d.get("ids")
    families = d.get("families")
    if not isinstance(ids, list) or not isinstance(families, dict):
        return 2, ["形状不符：缺 ids(list) 或 families(dict) ⇒ 不给结论"]
    if not ids:
        return 2, ["空转：ids 为空 ⇒ 不给结论"]
    flat = flatten(families)
    problems = []
    if sorted(ids) != sorted(flat):
        only_ids = sorted(set(ids) - set(flat))
        only_flat = sorted(set(flat) - set(ids))
        problems.append("FAIL 集合不等：仅 ids 有 %s · 仅 families 展平有 %s" % (only_ids, only_flat))
    if ids != flat:
        # 顺序（首处不同）
        n = min(len(ids), len(flat))
        idx = next((i for i in range(n) if ids[i] != flat[i]), n)
        problems.append("FAIL 顺序不等：第 %d 位 ids=%r 展平=%r（len ids=%d 展平=%d）"
                        % (idx + 1, ids[idx] if idx < len(ids) else None,
                           flat[idx] if idx < len(flat) else None, len(ids), len(flat)))
    return (0 if not problems else 1), problems


def self_test():
    ok = True
    good = {"ids": ["D1", "D2", "E1"], "families": {"D": ["D1", "D2"], "E": ["E1"], "F": [], "G": []}}
    cases = []
    cases.append(("正控 好件（集合与序均一致）⇒ rc=0", 0, good))
    bad_set = {"ids": ["D1", "D2", "E1"], "families": {"D": ["D1"], "E": ["E1"]}}
    cases.append(("负控 集合不等（families 漏 D2）⇒ rc=1", 1, bad_set))
    bad_order = {"ids": ["D2", "D1", "E1"], "families": {"D": ["D1", "D2"], "E": ["E1"]}}
    cases.append(("负控 顺序不等（ids 换序）⇒ rc=1", 1, bad_order))
    empty = {"ids": [], "families": {"D": []}}
    cases.append(("负控 空 ids ⇒ rc=2", 2, empty))
    for name, want, body in cases:
        import tempfile, os
        with tempfile.TemporaryDirectory() as t:
            p = os.path.join(t, "dev-targets.json")
            with open(p, "w", encoding="utf-8") as f:
                json.dump(body, f, ensure_ascii=False)
            rc, probs = check(p)
        flag = "✓" if rc == want else "✗"
        print("  %s %s（rc=%d）" % (flag, name, rc))
        if rc != want:
            ok = False
            for x in probs:
                print("      · %s" % x)
    rc, _ = check("/nonexistent/nope.json")
    flag = "✓" if rc == 2 else "✗"
    print("  %s 负控 缺件 ⇒ rc=2（rc=%d）" % (flag, rc))
    if rc != 2:
        ok = False
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main():
    args = sys.argv[1:]
    if "--self-test" in args:
        return self_test()
    args = [a for a in args if not a.startswith("--")]
    path = args[0] if args else "core/internal/contract/dev-targets.json"
    rc, problems = check(path)
    print("── 门 · dev-targets 静态断言（GAP-20260927-197）：set(ids)==set(展平) 且 ids==展平序 ──")
    for p in problems:
        print("  · %s" % p)
    if rc == 0:
        print("⇒ 全绿（rc=0）：ids 与 families 展平集合相等且同序")
    elif rc == 1:
        print("⇒ 门禁红（rc=1）：ids 与 families 展平不一致")
    else:
        print("⇒ 不给结论（rc=2）")
    return rc


if __name__ == "__main__":
    sys.exit(main())

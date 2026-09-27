#!/usr/bin/env python3
# check-error-kinds.py — 门禁：命令面 `error.kind` 闭集与命名纪律（§九 M7 · §二十一 的 kind 判据）。
#
# 为什么要有这只门（不是「顺手加的」）：
#   §九 M7 把「机器可读的 `error.kind`」定为契约的一部分，AI 自愈**只读 kind**（`E7`）。
#   kind 一旦写成大写 / 写成中文 / 各处各写一套，消费侧的「按 kind 分派」就会静默失配 ——
#   而失配的表现是「AI 重试了不该重试的东西」，不是报错 ⇒ 必须**机器每次查**，不靠记性。
#
# 三条规则（任一不通过 ⇒ rc=1）：
#   R1 **闭集形状**：`core/cmd/zerg/errors.go` 的 `errorKinds` 表里每个 kind 必须匹配
#      `^[a-z][a-z0-9_]*$`（lower_snake_case）—— 大写（如 `CONFIRM_REQUIRED`）**门禁红**。
#   R2 **闭集唯一 + 码已登记**：kind 不许重复；每个 kind 的退码必须是**退码表里已登记的号**
#      （`exitcodes.go` 的主表 + 已挂号表 —— 新 kind 想用个新码而不登记号，就红）。
#      ★ 口径（防误读）：**同一个码下允许多个 kind 且可重试性不同** —— 那正是 `E7` 的前提
#      （「拿退出码判重试」会出错，所以只许读 kind）；本门**不**把这种情况判红。
#   R3 **用法面**：`core/cmd/zerg/*.go` 里所有 `setErr("…"` / `kind="…"` 的字面量必须在闭集内
#      （写了个闭集外的 kind ⇒ 红；这样「新加 kind 忘了登记」也会被拦）。
#
# 空转 = 假覆盖：扫到 0 个 kind ⇒ **rc=2（不给结论）**，不是绿（照 §17.6 `E-M6` 的口径）。
#
# 用法：
#   python3 scripts/gates/check-error-kinds.py [仓库根]
#   python3 scripts/gates/check-error-kinds.py --self-test   # 成对负控（会红 / 会绿 / 空转）
import argparse
import os
import re
import sys

KIND_RE = re.compile(r'^[a-z][a-z0-9_]*$')
# errors.go 的闭集表行形如：	{"conflict", 14, false, "wait_or_reload", "…", "…"},
ROW_RE = re.compile(r'^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,', re.M)
# 用法面：setErr("<kind>"…) 或 kind = "<kind>" / kind:"<kind>"
USE_RE = re.compile(r'setErr\(\s*"([^"]+)"')
JSON_KIND_RE = re.compile(r'"kind"\s*:\s*"([^"]+)"')


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, 'core', 'cmd', 'zerg', 'errors.go')):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def parse_closed_set(text):
    """从 errors.go 里读闭集表（只读真源，不另抄一份）。"""
    start = text.find('var errorKinds = []errorKindRow{')
    end = text.find('\n}', start)
    if start < 0 or end < 0:
        return []
    rows = []
    for m in ROW_RE.finditer(text[start:end]):
        rows.append({'kind': m.group(1), 'code': int(m.group(2)),
                     'retryable': m.group(3) == 'true'})
    return rows


def check(root):
    """返回 (rc, 报告行列表)。rc 0=绿 1=红 2=不给结论。"""
    errs, warns = [], []
    errs_path = os.path.join(root, 'core', 'cmd', 'zerg', 'errors.go')
    with open(errs_path, encoding='utf-8') as f:
        rows = parse_closed_set(f.read())
    if not rows:
        return 2, ['⛔ 不给结论：闭集表为空（`errorKinds` 一个 kind 都没解析到 ⇒ 空转 = 假覆盖）']

    seen = {}
    for r in rows:
        if not KIND_RE.match(r['kind']):
            errs.append(f"R1 kind 命名不合规（必须 lower_snake_case）：{r['kind']!r}")
        if r['kind'] in seen:
            errs.append(f"R2 kind 重复：{r['kind']}")
        seen[r['kind']] = r
        if r['code'] < 0:
            errs.append(f"R2 退码为负：{r['kind']} → {r['code']}")

    # R2 的另一半：每个 kind 的码必须在退码表里登记过（新码不登记 = 闭集外的号）
    with open(os.path.join(root, 'core', 'cmd', 'zerg', 'exitcodes.go'), encoding='utf-8') as f:
        declared = set()
        for line in f:
            m = re.match(r'^\s*\{(\d+),\s*"', line)
            if m:
                declared.add(int(m.group(1)))
    if not declared:
        return 2, ['⛔ 不给结论：退码表一个码都没解析到（空转 = 假覆盖）']
    for r in rows:
        if r['code'] not in declared:
            errs.append(f"R2 kind {r['kind']!r} 用的码 {r['code']} **不在退码表里**"
                        f"（先登记号再启用：占号纪律 §九 M7 `E8`）")

    # R3 用法面
    dirpath = os.path.join(root, 'core', 'cmd', 'zerg')
    used = set()
    for name in sorted(os.listdir(dirpath)):
        if not name.endswith('.go') or name.endswith('_test.go'):
            continue
        with open(os.path.join(dirpath, name), encoding='utf-8') as f:
            text = f.read()
        used |= set(USE_RE.findall(text))
        used |= set(JSON_KIND_RE.findall(text))
    used.discard('kind')
    for u in sorted(used):
        if u not in seen:
            errs.append(f"R3 用到闭集外的 kind：{u!r}（先登记进 `errorKinds` 再用）")

    lines = []
    for r in rows:
        lines.append(f"  kind={r['kind']:<22} code={r['code']:<4} retryable={str(r['retryable']):<5}")
    lines.append(f"闭集 {len(rows)} 个 kind · 用法面命中 {len(used)} 个字面量")
    if errs:
        for e in errs:
            lines.append('✗ ' + e)
        return 1, lines
    for w in warns:
        lines.append('… ' + w)
    return 0, lines


def self_test():
    """成对负控：写死的夹具树里，坏件必红、好件必绿、空转必不给结论。"""
    import tempfile
    ok = True

    def mk(tmp, body):
        p = os.path.join(tmp, 'core', 'cmd', 'zerg')
        os.makedirs(p, exist_ok=True)
        with open(os.path.join(p, 'errors.go'), 'w', encoding='utf-8') as f:
            f.write(body)
        # 夹具的退码表：只放闭集里用得上的号（真源形状的一小片）
        with open(os.path.join(p, 'exitcodes.go'), 'w', encoding='utf-8') as f:
            f.write('var exitcodeTable = []exitcodeRow{\n'
                    '\t{0, "ok", "成功", "—"},\n\t{1, "failed", "一般失败", "看 kind"},\n'
                    '\t{2, "usage", "用法错", "改用法"},\n\t{14, "conflict", "冲突", "排队"},\n}\n')
        return tmp

    good = 'var errorKinds = []errorKindRow{\n\t{"conflict", 14, false, "r", "m", "s"},\n}\n'
    with tempfile.TemporaryDirectory() as t:
        rc, _ = check(mk(t, good))
        print(f"  自检① 好件（lower_snake）⇒ rc={rc}（要 0）")
        ok &= rc == 0
    bad = 'var errorKinds = []errorKindRow{\n\t{"CONFIRM_REQUIRED", 2, false, "r", "m", "s"},\n}\n'
    with tempfile.TemporaryDirectory() as t:
        rc, lines = check(mk(t, bad))
        print(f"  自检② 大写 kind ⇒ rc={rc}（要 1）· 命中: {[l for l in lines if 'R1' in l]}")
        ok &= rc == 1
    empty = 'var errorKinds = []errorKindRow{\n}\n'
    with tempfile.TemporaryDirectory() as t:
        rc, _ = check(mk(t, empty))
        print(f"  自检③ 空闭集 ⇒ rc={rc}（要 2 —— 空转 = 假覆盖，不给结论）")
        ok &= rc == 2
    # 成对（必须有牙）：kind 用了一个退码表里没登记的号 ⇒ 红
    ghost = ('var errorKinds = []errorKindRow{\n'
             '\t{"ghost_kind", 99, false, "r", "m", "s"},\n}\n')
    with tempfile.TemporaryDirectory() as t:
        rc, lines = check(mk(t, ghost))
        print(f"  自检④ 未登记的码 99 ⇒ rc={rc}（要 1）· {[l for l in lines if 'R2' in l]}")
        ok &= rc == 1
    print('自检结论: ' + ('全过' if ok else '**有断言失败**'))
    return 0 if ok else 1


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser()
    ap.add_argument('root', nargs='?', default='.')
    ap.add_argument('--self-test', action='store_true')
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = find_root(a.root)
    if root is None:
        print('⛔ 不给结论：找不到 core/cmd/zerg/errors.go（本门只认命令面真源所在的那棵树）')
        return 2
    rc, lines = check(root)
    print('── 命令面 error.kind 闭集（§九 M7）──')
    for l in lines:
        print(l)
    if rc == 0:
        print('⇒ 全绿（rc=0）')
    elif rc == 1:
        print('⇒ 门禁红（rc=1）：kind 闭集或命名纪律被破坏')
    else:
        print('⇒ 不给结论（rc=2）')
    return rc


if __name__ == '__main__':
    sys.exit(main())

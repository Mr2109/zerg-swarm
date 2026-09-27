#!/usr/bin/env python3
# check-wall-layer.py — 门禁：茧壁边界的**可验硬判据**（§九 M10 `X1`/`X2`/`EDGE-NODE` · §十二 `P-051`）。
#
# 为什么要有它（`P-051` 逐字：茧壁的语义判据「**今天只有 rust 三条编/测、零语义判据**」）：
#   茧壁的强度全在「边界有没有被悄悄放宽」上 —— 而放宽的表现**不是报错**，是**一条命令面件
#   出现在空间内**。这种事只能机器每次查。
#
# 四条规则（任一不通过 ⇒ rc=1）：
#   R1 **层级闭集**：`layer` 的三值必须是 `host`/`node`/`space`（`X1`；自造第四档 ⇒ 红）。
#   R2 **`space` 档恒为空集**：命令树里**不许**出现 `layer: "space"`（`X2`：空间内零命令面；
#      B 类现状空集）。
#   R3 **argv 零命令面**（`EDGE-NODE`）：全部 plan 产物的 `argv` 里，**命令面件**（`zerg` · `zerg-core` ·
#      `zerg-agentd` · `zerg-wall` · `zerg-compat` · `zerg-model` · `precommit-gates.sh`）**零命中**。
#      ★ 豁免（有出处，不是放水）：`zerg-egg` 是**引擎自己的启动壳**，按 §3.4 D 类逐字
#      「引擎自己的启动参数**不算**命令面」⇒ 不在禁列。
#   R4 **空转 = 假覆盖**：闭集表、命令树、plan 产物三样任一为空 ⇒ **rc=2（不给结论）**，不是绿。
#
# 用法：
#   python3 scripts/gates/check-wall-layer.py [仓库根]
#   python3 scripts/gates/check-wall-layer.py --self-test   # 成对负控（好件绿 / 三条反例各自红 / 空转 2）
import argparse
import json
import os
import re
import sys
import tempfile

CLOSED = ('host', 'node', 'space')
FORBIDDEN_ARGV = {'zerg', 'zerg-core', 'zerg-agentd', 'zerg-wall', 'zerg-compat',
                  'zerg-model', 'precommit-gates.sh'}
# 层常量：`layerHost = "host"` 一类（真源在 core/cmd/zerg/wall.go）。
CONST_RE = re.compile(r'layer(Host|Node|Space)\s*=\s*"([^"]+)"')
# 命令级标记：`layer: "space"`（命令树里逐条写的那种）
CMD_LAYER_RE = re.compile(r'layer:\s*"([^"]+)"')


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, 'core', 'cmd', 'zerg', 'wall.go')):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def check(root):
    errs = []
    cli_dir = os.path.join(root, 'core', 'cmd', 'zerg')
    src = ''
    for name in sorted(os.listdir(cli_dir)):
        if name.endswith('.go') and not name.endswith('_test.go'):
            with open(os.path.join(cli_dir, name), encoding='utf-8') as f:
                src += f.read()

    # R1 闭集
    consts = dict((m.group(1).lower(), m.group(2)) for m in CONST_RE.finditer(src))
    if len(consts) < 3:
        return 2, ['⛔ 不给结论：层级闭集常量没读全（R1 空转 = 假覆盖）']
    bad_const = {k: v for k, v in consts.items() if v not in CLOSED}
    for k, v in sorted(bad_const.items()):
        errs.append(f"R1 层级闭集被改：layer{k.title()} = {v!r}（闭集只有 {'/'.join(CLOSED)}）")
    if set(consts.values()) != set(CLOSED):
        errs.append(f"R1 闭集不足三档或多了档：{sorted(consts.values())}")

    # R2 space 档恒空
    marks = CMD_LAYER_RE.findall(src)
    if not marks and not src:
        return 2, ['⛔ 不给结论：命令树里一个 layer 标记都没读到（空转 = 假覆盖）']
    space_marks = [m for m in marks if m == 'space']
    for m in space_marks:
        errs.append("R2 命令树里出现 `layer: \"space\"` —— 空间内**零命令面**（X2/P-048 取空集）")
    for m in marks:
        if m not in CLOSED:
            errs.append(f"R1/R2 命令树里出现闭集外的层级 {m!r}")

    # R3 plan 产物的 argv
    plans, scanned = [], 0
    for base, dirs, files in os.walk(os.path.join(root, 'wall')):
        dirs[:] = [d for d in dirs if d != 'target']
        for fn in files:
            if fn.endswith('.txt') or fn.endswith('.json'):
                p = os.path.join(base, fn)
                try:
                    with open(p, encoding='utf-8') as f:
                        txt = f.read()
                except Exception:
                    continue
                # 两种形态都要认：整文件是 JSON，**或者** 一行一条 JSON（`wall/evidence-plan-*.txt`
                # 就是后者 —— 出证文件是「一行一件」的行分隔 JSON）。
                cands = []
                whole = None
                try:
                    whole = json.loads(txt)
                except Exception:
                    whole = None
                if whole is not None:
                    cands.append(whole)
                else:
                    for line in txt.splitlines():
                        line = line.strip()
                        if not line.startswith('{'):
                            continue
                        try:
                            cands.append(json.loads(line))
                        except Exception:
                            continue
                for obj in cands:
                    if isinstance(obj, dict) and isinstance(obj.get('argv'), list):
                        plans.append((p, obj['argv']))
    for base, dirs, files in os.walk(os.path.join(root, 'agent')):
        dirs[:] = [d for d in dirs if d not in ('target', 'testdata')]
        for fn in files:
            if not fn.endswith('.json'):
                continue
            p = os.path.join(base, fn)
            try:
                with open(p, encoding='utf-8') as f:
                    obj = json.load(f)
            except Exception:
                continue
            if isinstance(obj, dict) and isinstance(obj.get('argv'), list):
                plans.append((p, obj['argv']))
    if not plans:
        return 2, ['⛔ 不给结论：一份带 `argv` 的 plan 产物都没找到（R3 空转 = 假覆盖）']
    for p, argv in plans:
        scanned += 1
        for tok in argv:
            base_tok = os.path.basename(str(tok))
            if base_tok in FORBIDDEN_ARGV:
                errs.append(f"R3 {os.path.relpath(p, root)} 的 argv 里出现命令面件 {base_tok!r}")

    lines = [f"  层级闭集 = {'/'.join(sorted(consts.values()))}（三档）",
             f"  命令树 layer 标记 {len(marks)} 处 · 其中 space = {len(space_marks)}（须为 0）",
             f"  扫到 plan 产物 {scanned} 份（argv 均无命令面件）"]
    if errs:
        lines += ['✗ ' + e for e in errs]
        return 1, lines
    return 0, lines


def self_test():
    ok = True

    def fixture(root, src, plans):
        cli = os.path.join(root, 'core', 'cmd', 'zerg')
        os.makedirs(cli, exist_ok=True)
        with open(os.path.join(cli, 'wall.go'), 'w', encoding='utf-8') as f:
            f.write(src)
        wdir = os.path.join(root, 'wall')
        os.makedirs(wdir, exist_ok=True)
        with open(os.path.join(wdir, 'plan.json'), 'w', encoding='utf-8') as f:
            json.dump({'argv': plans}, f)
        return root

    good_src = ('const (\n\tlayerHost  = "host"\n\tlayerNode  = "node"\n'
                '\tlayerSpace = "space"\n)\n'
                'var c = command{layer: "host"}\n')
    with tempfile.TemporaryDirectory() as t:
        rc, _ = check(fixture(t, good_src, ['--ro-bind', '/usr', '/usr', 'zerg-egg']))
        print(f"  自检① 好件（闭集齐 · space 空 · argv 只有 zerg-egg 豁免）⇒ rc={rc}（要 0）")
        ok &= rc == 0
    with tempfile.TemporaryDirectory() as t:
        bad = good_src.replace('layerNode  = "node"', 'layerNode  = "target"')
        rc, lines = check(fixture(t, bad, ['--ro-bind', '/usr', '/usr']))
        print(f"  自检② 自造第四档 ⇒ rc={rc}（要 1）· {[l for l in lines if 'R1' in l]}")
        ok &= rc == 1
    with tempfile.TemporaryDirectory() as t:
        bad = good_src.replace('layer: "host"', 'layer: "space"')
        rc, lines = check(fixture(t, bad, ['--ro-bind', '/usr', '/usr']))
        print(f"  自检③ 命令树里出现 space 档 ⇒ rc={rc}（要 1）· {[l for l in lines if 'R2' in l]}")
        ok &= rc == 1
    with tempfile.TemporaryDirectory() as t:
        rc, lines = check(fixture(t, good_src, ['/bin/sh', '-c', 'exec "$@"', 'zerg-core']))
        print(f"  自检④ argv 里混进命令面件 ⇒ rc={rc}（要 1）· {[l for l in lines if 'R3' in l]}")
        ok &= rc == 1
    with tempfile.TemporaryDirectory() as t:
        cli = os.path.join(t, 'core', 'cmd', 'zerg')
        os.makedirs(cli, exist_ok=True)
        with open(os.path.join(cli, 'wall.go'), 'w', encoding='utf-8') as f:
            f.write(good_src)
        rc, _ = check(t)  # 没有 plan 产物 ⇒ 空转
        print(f"  自检⑤ 没有 plan 产物 ⇒ rc={rc}（要 2 —— 空转不给结论）")
        ok &= rc == 2
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
        print('⛔ 不给结论：找不到 core/cmd/zerg/wall.go（本门只认命令面真源所在的那棵树）')
        return 2
    rc, lines = check(root)
    print('── 茧壁边界：layer 闭集 + space 空集 + argv 零命令面（§九 M10 X1/X2/EDGE-NODE）──')
    for l in lines:
        print(l)
    if rc == 0:
        print('⇒ 全绿（rc=0）')
    elif rc == 1:
        print('⇒ 门禁红（rc=1）：茧壁边界被放宽')
    else:
        print('⇒ 不给结论（rc=2）')
    return rc


if __name__ == '__main__':
    sys.exit(main())

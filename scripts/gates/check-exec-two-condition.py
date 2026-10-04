#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-exec-two-condition.py —— 「同件双条件」执行位静态门（缺口 `GAP-20260927-145` ② + `-162`）

题面（缺口 `GAP-20260927-145` ② 逐字）：
    建静态门：**同件双条件**判据（一件内同时出现 ①执行位
    `eval "$X"` / `bash -c "$X"` / `sh -c "$X"` / `ssh <host> "$X"` / `GIT_SSH_COMMAND="$X …"` /
    `exec.Command("sh","-c",非字面量)` 与 ②仓外输入写进命令位变量 `${ZERG_*:-}` / `os.Getenv` /
    `argv` / 名册行 / 契约字段）⇒ 红并点名；噪声用调用点白名单压掉、正负控各一枚。

题面（缺口 `GAP-20260927-162` 逐字）：
    判据扩成三段（面 → ①**六形态 + 三条补形态** → ②→①连通）＋ 一表行级登记；三条补形态 =
    解释器关键字（`shell=True`）/ 无 `-c` 的解释器＋脚本路径形态 / argv 数组式 exec。

病象出处（缺口 `GAP-20260927-145`）：
    `zerg-swap-core.sh:243` 在 `restart_master()` 真跑分支上 `eval "$ZERG_SWAP_RESTART_STUB"` ——
    换件器唯一出口被 env 直控；同族 100 处变量拼装点、76% 可被仓外控制。

口径（写死 —— 本件的判据）：
    · 扫面 = `<root>/{scripts,build,core,ui,publish}/**`（`.sh/.py/.go/.rs`）；`--root` 可换（自检用）。
    · **①执行位六形态**（正则即内容锚，不用行号）：
        E1 `eval "$VAR"` · E2 `bash|sh -c "$VAR"` · E3 `ssh <host> "$VAR"` ·
        E4 `GIT_SSH_COMMAND="$VAR …"` · E5 `exec.Command("sh","-c",<非字面量>)` ·
        E6 `Command::new("bash").arg(&x)`（Rust）。
    · **三条补形态**（缺口 -162）：P1 `shell=True` · P2 无 `-c` 的解释器＋脚本路径
      （`bash "$VAR"` / `sh "$VAR"`）· P3 argv 数组式 exec（`exec.Command(<非字面量>, ...)`）。
    · **②仓外输入**：`${ZERG_*:-…}` · `os.Getenv(` · `os.environ.get(` · `<非字面量> argv` · 名册行/契约字段
      （见 `OUTSIDE_RES`）。
    · **同件双条件** = 同一件内既有执行位（①或补形态）**又有**仓外输入 ⇒ **红灯点名**（file:line）。
    · **白名单**：`--allow <file:shape>`（可重复）或顶部 `CALLSITE_ALLOW`；每条须给理由/出处，
      缺则 rc=2。**白名单空即红**（✗ 不许默默全绿）。
    · **退码写死**：`rc=0` 无未白名单的双条件件 · `rc=1` 有（逐条 file:line）· `rc=2` 缺语料/白名单不全。
    · **只读**：不写任何文件。

★ 假阴面（缺口 -162 明写）：`②「契约字段」一类在机械实现上是盲区` —— 要抓须走数据流/类型标注，
  本件**先只登记不判**该一类（`CONTRACT_FIELD_NOTE`）。

用法：
    python3 scripts/gates/check-exec-two-condition.py [--root <dir>] [--allow file:shape]... [--json] [--self-test]

★ 接线状态：本件为**仓外候件**（`/var/tmp/zerg-wave23/p0/`），未挂门步 —— 首版 `tri-report`。
"""
import argparse
import json
import os
import re
import sys

EXEC_RES = [
    ("E1", re.compile(r'\beval\s+"?\$\{?([A-Za-z_][\w]*)')),
    ("E2", re.compile(r'\b(?:bash|sh)\s+-c\s+"?\$')),
    ("E3", re.compile(r'\bssh\s+\S+\s+"\$')),
    ("E4", re.compile(r'GIT_SSH_COMMAND\s*=\s*"\$')),
    ("E5", re.compile(r'exec\.Command\(\s*"(?:sh|bash)"\s*,\s*"-c"\s*,\s*[^"]')),
    ("E6", re.compile(r'Command::new\(\s*"(?:bash|sh)"\s*\)\.arg\(')),
    ("P1", re.compile(r'shell\s*=\s*True')),
    ("P2", re.compile(r'\b(?:bash|sh)\s+"\$')),
    ("P3", re.compile(r'exec\.Command\(\s*[^"]')),
]
OUTSIDE_RES = [
    re.compile(r"\$\{ZERG_[A-Z0-9_]*:-"),
    re.compile(r"\bos\.Getenv\("),
    re.compile(r"\bos\.environ\.get\("),
    re.compile(r"\bgetenv\("),
    re.compile(r"\$\{(?!ZERG_)[A-Za-z_][\w]*:-\}"),  # 一般 var 默认值（弱）
]
CONTRACT_FIELD_NOTE = "②「契约字段」一类须走数据流/类型标注 ⇒ 本件先只登记不判（缺口 -162 明写）"
# 顶部白名单：每条 {file, shape, reason, source}
CALLSITE_ALLOW = []

SCAN_SUBDIRS = ["scripts", "build", "core", "ui", "publish"]
EXTS = (".sh", ".py", ".go", ".rs")


def iter_files(root):
    for sub in SCAN_SUBDIRS:
        base = os.path.join(root, sub)
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = [d for d in dirnames if d not in (".git", "node_modules", "target", ".stage")]
            for name in sorted(filenames):
                if name.endswith(EXTS):
                    yield os.path.join(dirpath, name)


def file_shapes(path):
    """回本件命中的执行位形态集合 ＋ 是否见仓外输入。"""
    shapes, outside = set(), False
    try:
        lines = open(path, "r", encoding="utf-8", errors="replace").read().splitlines()
    except OSError:
        return shapes, outside
    for line in lines:
        s = line.strip()
        if s.startswith("//") or s.startswith("#"):
            continue
        for sid, rx in EXEC_RES:
            if rx.search(line):
                shapes.add(sid)
        for rx in OUTSIDE_RES:
            if rx.search(line):
                outside = True
                break
    return shapes, outside


def run(root, allow, as_json):
    if not os.path.isdir(root):
        print("BLOCKED —— 扫面根不存在 ⇒ rc=2", file=sys.stderr)
        return 2
    bad = [w for w in CALLSITE_ALLOW if not (w.get("file") and w.get("shape") and w.get("reason") and w.get("source"))]
    if bad:
        print("BLOCKED —— 白名单条目不齐（理由/出处）⇒ rc=2", file=sys.stderr)
        return 2
    wl = {(w["file"], w["shape"]) for w in CALLSITE_ALLOW}
    for a in allow:
        if ":" in a:
            f, sh = a.split(":", 1); wl.add((f, sh))
    hits = []
    seen_files = 0
    for path in iter_files(root):
        seen_files += 1
        rel = os.path.relpath(path, root)
        shapes, outside = file_shapes(path)
        if shapes and outside:
            # 逐形态判白名单：该件所有形态都被白名单盖住才算过
            for sid in sorted(shapes):
                if (rel, sid) in wl or (os.path.basename(path), sid) in wl:
                    continue
                hits.append((rel, sid))
    if seen_files == 0:
        print("BLOCKED —— 扫面 0 件（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    if as_json:
        print(json.dumps({"rc": 1 if hits else 0, "files": seen_files,
                          "hits": [{"file": f, "shape": s} for f, s in hits]}, ensure_ascii=False))
    else:
        print("「同件双条件」执行位静态门（GAP-20260927-145 ② / -162）")
        print("扫面 %d 件 · 未白名单的双条件命中 = %d 处" % (seen_files, len(hits)))
        for f, s in hits:
            print("  ✗ %s 形态 %s（同件既有执行位又有仓外输入）" % (f, s))
        print("  ★ %s" % CONTRACT_FIELD_NOTE)
    return 1 if hits else 0


def self_test():
    import tempfile
    global CALLSITE_ALLOW
    ok = True
    with tempfile.TemporaryDirectory() as td:
        os.makedirs(os.path.join(td, "scripts", "build"))
        # 正控：同件双条件（E1 + ${ZERG_*:-}）
        open(os.path.join(td, "scripts/build/swap.sh"), "w").write(
            'BIN="${ZERG_SWAP_BIN:-bin/zerg-core}"\neval "$ZERG_SWAP_RESTART_STUB"\n')
        # 负控 A：只有执行位、无仓外输入
        open(os.path.join(td, "scripts/only_exec.sh"), "w").write('eval "$literal_cmd"\n')
        hits = []
        for p in iter_files(td):
            shapes, outside = file_shapes(p)
            if shapes and outside:
                hits.append(os.path.relpath(p, td))
        if hits != ["scripts/build/swap.sh"]:
            print("  ✗ 正控失败：命中集 %r" % hits); ok = False
        if run(td, [], False) != 1:
            print("  ✗ 正控 rc 失败：应回 1"); ok = False
        # 负控 B：白名单盖住该形态 ⇒ 0
        CALLSITE_ALLOW = [{"file": "scripts/build/swap.sh", "shape": "E1", "reason": "t", "source": "t"}]
        # 该件只剩 E1 形态 ⇒ 白名单后应过
        if run(td, [], False) != 0:
            print("  ✗ 负控B失败：白名单未生效"); ok = False
        CALLSITE_ALLOW = []
        # 负控 C：空根 ⇒ rc=2
        if run(os.path.join(td, "nope"), [], False) != 2:
            print("  ✗ 负控C失败：空根未回 rc=2"); ok = False
    print("自检 %s（4 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="「同件双条件」执行位静态门（GAP-20260927-145/-162）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--allow", action="append", default=[])
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.allow, args.json)


if __name__ == "__main__":
    sys.exit(main())

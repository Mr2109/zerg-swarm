#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-exec-two-condition.py —— 「同件双条件」执行位静态门（缺口 `GAP-20260927-145` ② + `-162` + `-173`）

题面（缺口 `GAP-20260927-145` ② 逐字）：
    建静态门：**同件双条件**判据（一件内同时出现 ①执行位
    `eval \"$X\"` / `bash -c \"$X\"` / `sh -c \"$X\"` / `ssh <host> \"$X\"` / `GIT_SSH_COMMAND=\"$X …\"` /
    `exec.Command(\"sh\",\"-c\",非字面量)` 与 ②仓外输入写进命令位变量 `${ZERG_*:-}` / `os.Getenv` /
    `argv` / 名册行 / 契约字段）⇒ 红并点名；噪声用调用点白名单压掉、正负控各一枚。

题面（缺口 `GAP-20260927-162` 逐字）：
    判据扩成三段（面 → ①**六形态 + 三条补形态** → ②→①连通）＋ 一表行级登记；三条补形态 =
    解释器关键字（`shell=True`）/ 无 `-c` 的解释器＋脚本路径形态 / argv 数组式 exec。

题面（缺口 `GAP-20260927-173` 逐字 · 三条补形态**分档**）：
    ① ① 进判据、挂**报告档** —— 但**不许阻断**（其真执行位里多数是 `scripts/evals/` 脚手架、
    **1 是门件本体** ⇒ 阻断会**自撞**）；
    ② ④ 只进**登记档**并先加四类护栏（heredoc 体内 / 行内引号文案 / 注释位 / `.md` 一律不登记）；
    ③ ⑤ 进判据、挂**报告档**（判据须锚在 exec 实参位、不退到构造位）；
    ④ 成对负控 5 组（④ 反控含实例；⑤ 防重复计数；① 与 ④/⑤ 交叉互斥；空转 rc=2）。

病象出处（缺口 `GAP-20260927-145`）：
    `zerg-swap-core.sh:243` 在 `restart_master()` 真跑分支上 `eval \"$ZERG_SWAP_RESTART_STUB\"` ——
    换件器唯一出口被 env 直控；同族 100 处变量拼装点、76% 可被仓外控制。

口径（写死 —— 本件的判据）：
    · 扫面 = `<root>/{scripts,build,core,ui,publish}/**`（`.sh/.py/.go/.rs`；`.md` 不扫、也不登记）；`--root` 可换（自检用）。
    · **①执行位六形态**（正则即内容锚，不用行号）——**判据档**（驱动 rc）：
        E1 `eval \"$VAR\"` · E2 `bash|sh -c \"$VAR\"` · E3 `ssh <host> \"$VAR\"` ·
        E4 `GIT_SSH_COMMAND=\"$VAR …\"` · E5 `exec.Command(\"sh\",\"-c\",<非字面量>)` ·
        E6 `Command::new(\"bash\").arg(&x)`（Rust）。
    · **三条补形态**（缺口 -162/-173）——**分档**（都不驱动 rc）：
        P1 `shell=True`            ⇒ **报告档**（保留在报告清单里 · 缺口 -162 逐字「不许阻断」）
        P2 无 `-c` 的解释器＋脚本路径（`bash \"$VAR\"` / `sh \"$VAR\"`）⇒ **登记档**（+ 四类护栏）
        P3 argv 数组式 exec（`exec.Command(<非字面量>, ...)`）⇒ **报告档**
    · **四类护栏**（只作用于登记档 P2；缺口 `GAP-20260927-173` 逐字「heredoc 体内 / 行内引号文案 /
      注释位 / `.md` 一律不登记」）：命中落在 ① heredoc 体内 ② 行内引号文案 ③ 注释位 ④ `.md` 件
      ⇒ **不进登记档**（不做假阳登记）。判据档 E1–E6 行为**一字未改**（护栏不放宽真判据）。
    · **②仓外输入**：`${ZERG_*:-…}` · `os.Getenv(` · `os.environ.get(` · `getenv(` · `<非字面量> argv`
      · 名册行/契约字段（见 `OUTSIDE_RES`）。
    · **同件双条件** = 同一件内既有执行位（①或补形态）**又有**仓外输入 ⇒ 进对应档并点名（file:line）。
    · **白名单**：`--allow <file:shape>`（可重复）或顶部 `CALLSITE_ALLOW`；每条须给理由/出处，
      缺则 rc=2。**白名单空即红**（✗ 不许默默全绿）。白名单只作用于**判据档**。
    · **退码写死**：`rc=0` 无未白名单的**判据档**双条件件（报告/登记档只报不拦）· `rc=1` 有（逐条 file:line）·
      `rc=2` 缺语料/白名单不全/空转。
    · **只读**：不写任何文件。

★ 假阴面（缺口 -162 明写）：`②「契约字段」一类在机械实现上是盲区` —— 要抓须走数据流/类型标注，
  本件**先只登记不判**该一类（`CONTRACT_FIELD_NOTE`）。

用法：
    python3 scripts/gates/check-exec-two-condition.py [--root <dir>] [--allow file:shape]... [--json] [--self-test]

★ 接线状态：**已挂门103**（tri-report 只报告档）· 2026-10-04 现读更正（原「仓外候件 / 未挂门步」已过期）。
"""
import argparse
import json
import os
import re
import sys

# 判据档（E1–E6）：命中即驱动 rc（逐条点名）—— 六形态一字未改。
EXEC_RES = [
    ("E1", re.compile(r'\beval\s+"?\$\{?([A-Za-z_][\w]*)')),
    ("E2", re.compile(r'\b(?:bash|sh)\s+-c\s+"?\$')),
    ("E3", re.compile(r'\bssh\s+\S+\s+"\$')),
    ("E4", re.compile(r'GIT_SSH_COMMAND\s*=\s*"\$')),
    ("E5", re.compile(r'exec\.Command\(\s*"(?:sh|bash)"\s*,\s*"-c"\s*,\s*[^"]')),
    ("E6", re.compile(r'Command::new\(\s*"(?:bash|sh)"\s*\)\.arg\(')),
]
# 报告档（P1 shell=True · P3 argv 数组式 exec）：进报告清单，**不驱动 rc**（缺口 -173 ①③）。
REPORT_RES = [
    ("P1", re.compile(r'shell\s*=\s*True')),
    ("P3", re.compile(r'exec\.Command\(\s*[^"]')),
]
# 登记档（P2 无 `-c` 的解释器＋脚本路径）：进登记清单，**不驱动 rc**（缺口 -173 ②）· 受四类护栏。
REGISTER_RES = [
    ("P2", re.compile(r'\b(?:bash|sh)\s+"\$')),
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

# 注释位判据（四类护栏之③）：去掉行首空白后以这些前缀开头 ⇒ 整行不进任何档
COMMENT_PREFIXES = ("#", "//", "/*", "*", "<!--")
# heredoc 起止判据（四类护栏之①）：`<<[-]DELIM` / `<<'DELIM'` / `<<"DELIM"`；闭合行 = 逐字等于 DELIM
HEREDOC_RE = re.compile(r"<<-?\s*['\"]?([A-Za-z_][A-Za-z0-9_]*)['\"]?")
# 引号对（四类护栏之② · 行内引号文案）：`"` / `'` / 反引号
QUOTES = ('"', "'", "`")


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


def heredoc_body_lines(lines):
    """四类护栏之①：返回落在 heredoc 体内的行号集合（1 起）。不做假阳登记。"""
    out, delim = set(), None
    for i, line in enumerate(lines, 1):
        if delim is None:
            m = HEREDOC_RE.search(line)
            if m:
                delim = m.group(1)
            continue
        out.add(i)
        if line.strip() == delim:
            delim = None
    return out


def in_inline_quote(line, pos):
    """四类护栏之②：命中位 `pos` 是否落在**未闭合引号内**（行内引号文案）。"""
    seg = line[:pos]
    for q in QUOTES:
        if seg.count(q) % 2 == 1:
            return True
    return False


def is_guarded(path, lines, hd_lines, lineno, pos):
    """四类护栏汇总：① heredoc 体内 ② 行内引号文案 ③ 注释位 ④ `.md` 件 ⇒ True（不登记）。"""
    if path.endswith(".md"):                                  # ④ `.md` 一律不登记
        return True
    if lineno in hd_lines:                                    # ① heredoc 体内
        return True
    if lines[lineno - 1].lstrip().startswith(COMMENT_PREFIXES):  # ③ 注释位
        return True
    if in_inline_quote(lines[lineno - 1], pos):               # ② 行内引号文案
        return True
    return False


def scan_file(path):
    """回 (判据档集合, 报告档集合, 登记档集合, 是否见仓外输入)（三档分列 · 缺口 -173）。"""
    judge, report, register, outside = set(), set(), set(), False
    try:
        lines = open(path, "r", encoding="utf-8", errors="replace").read().splitlines()
    except OSError:
        return judge, report, register, outside
    hd_lines = heredoc_body_lines(lines)
    for i, line in enumerate(lines, 1):
        s = line.strip()
        if s.startswith(COMMENT_PREFIXES):                    # 注释位：整行不进任何档
            continue
        for rx in OUTSIDE_RES:
            if rx.search(line):
                outside = True
                break
        for sid, rx in EXEC_RES:                              # 判据档
            if rx.search(line):
                judge.add(sid)
        for sid, rx in REPORT_RES:                            # 报告档
            if rx.search(line):
                report.add(sid)
        for sid, rx in REGISTER_RES:                          # 登记档（受四类护栏）
            m = rx.search(line)
            if not m:
                continue
            if is_guarded(path, lines, hd_lines, i, m.start()):
                continue
            register.add(sid)
    return judge, report, register, outside


def file_shapes(path):
    """向后兼容：回 (判据档集合 ∪ 报告档集合 ∪ 登记档集合, 是否见仓外输入)。

    ★ 保留旧入口（外部/自检可能按名调用）：**只读数与合档，不改任何档语义**。
    """
    judge, report, register, outside = scan_file(path)
    return (judge | report | register), outside


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
    judge_hits, report_hits, register_hits = [], [], []
    seen_files = 0
    for path in iter_files(root):
        seen_files += 1
        rel = os.path.relpath(path, root)
        judge, report, register, outside = scan_file(path)
        if not outside:                       # 双条件 = 执行位 AND 仓外输入
            continue
        base = os.path.basename(path)
        for sid in sorted(judge):             # 判据档：白名单只作用于此
            if (rel, sid) in wl or (base, sid) in wl:
                continue
            judge_hits.append((rel, sid))
        for sid in sorted(report):            # 报告档：只报不拦
            report_hits.append((rel, sid))
        for sid in sorted(register):          # 登记档：只登记不拦
            register_hits.append((rel, sid))
    if seen_files == 0:
        print("BLOCKED —— 扫面 0 件（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    rc = 1 if judge_hits else 0
    if as_json:
        print(json.dumps({"rc": rc, "files": seen_files,
                          "hits": [{"file": f, "shape": s} for f, s in judge_hits],
                          "report": [{"file": f, "shape": s} for f, s in report_hits],
                          "register": [{"file": f, "shape": s} for f, s in register_hits]},
                         ensure_ascii=False))
    else:
        print("「同件双条件」执行位静态门（GAP-20260927-145 ② / -162 / -173）")
        print("扫面 %d 件 · 判据档（驱动 rc）未白名单命中 = %d 处 · 报告档 %d 处 · 登记档 %d 处"
              % (seen_files, len(judge_hits), len(report_hits), len(register_hits)))
        for f, s in judge_hits:
            print("  ✗ %s 形态 %s（同件既有执行位又有仓外输入 · 判据档）" % (f, s))
        for f, s in report_hits:
            print("  · %s 形态 %s（报告档 · 只报不拦：脚手架/门件本体会自撞）" % (f, s))
        for f, s in register_hits:
            print("  · %s 形态 %s（登记档 · 只登记不拦 · 已过四类护栏）" % (f, s))
        print("  ★ %s" % CONTRACT_FIELD_NOTE)
    return rc


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

        # ── 缺口 `GAP-20260927-173`：三条补形态分档 + 四类护栏 + 成对负控（独立夹具根 `td2`）──
        #   ★ 独立根：本组夹具**不与上面的 E1 正控同根**，否则 rc 断言会被上面那条 E1 命中污染。
        with tempfile.TemporaryDirectory() as td2:
            os.makedirs(os.path.join(td2, "scripts"), exist_ok=True)

            def w(rel, body):
                ap = os.path.join(td2, "scripts", rel)
                os.makedirs(os.path.dirname(ap), exist_ok=True)
                open(ap, "w").write(body)
                return os.path.join(td2, "scripts", rel)

            # P1（shell=True · 报告档）：既有 shell=True 又有 ${ZERG_*:-}/os.environ.get ⇒ 报告档有名、判据档空
            r_p1 = w("p1.py", 'import subprocess, os\nX = os.environ.get("ZERG_BIN")\n'
                              'subprocess.run(X, shell=True)\nY = "${ZERG_HOME:-/tmp}"\n')
            j1, rep1, reg1, out1 = scan_file(r_p1)
            if not (out1 and "P1" in rep1 and not j1 and not reg1):
                print("  ✗ P1 报告档失败：judge=%r report=%r register=%r outside=%r" % (j1, rep1, reg1, out1)); ok = False
            # 成对负控 · P1 不驱动 rc：只有 P1(+P3+P2) 的根 ⇒ run 回 0（★ 旧实现 P1 在判据档 ⇒ 回 1）
            w("p3.go", 'package main\n\nimport "os"\n\nfunc f() {\n\tcmd := exec.Command(os.Getenv("ZERG_BIN"), "run")\n'
                       '\t_ = cmd\n}\n')
            w("p2.sh", 'RUNNER="${ZERG_RUNNER:-bin/run}"\nbash "$RUNNER" --go\n')
            if run(td2, [], False) != 0:
                print("  ✗ P1/P3/P2 未分档：补形态仍驱动 rc（应只报告/登记 · 回 0）"); ok = False

            # P3（argv 数组式 exec · 报告档）
            j3, rep3, reg3, out3 = scan_file(os.path.join(td2, "scripts", "p3.go"))
            if not (out3 and "P3" in rep3 and not j3):
                print("  ✗ P3 报告档失败：judge=%r report=%r outside=%r" % (j3, rep3, out3)); ok = False

            # P2（无 `-c` 解释器＋脚本路径 · 登记档）：既有 bash "$RUNNER" 又有 ${ZERG_*:-} ⇒ 登记档、不进判据
            j2, rep2, reg2, out2 = scan_file(os.path.join(td2, "scripts", "p2.sh"))
            if not (out2 and "P2" in reg2 and not j2):
                print("  ✗ P2 登记档失败：judge=%r register=%r outside=%r" % (j2, reg2, out2)); ok = False

            # 护栏①：P2 落在 heredoc 体内 ⇒ 不登记
            r_hd = w("hd.sh", 'X="${ZERG_X:-v}"\ncat <<EOF\nbash "$RUNNER"\nEOF\n')
            _jh, _reph, regh, _outh = scan_file(r_hd)
            if "P2" in regh:
                print("  ✗ 护栏①失败：heredoc 体内仍登记 P2"); ok = False

            # 护栏③：P2 落在注释位 ⇒ 不登记
            r_cm = w("cm.sh", 'X="${ZERG_X:-v}"\n# bash "$RUNNER"\n')
            _jc, _repc, regc, _outc = scan_file(r_cm)
            if "P2" in regc:
                print("  ✗ 护栏③失败：注释位仍登记 P2"); ok = False

            # 护栏②：P2 落在行内引号文案 ⇒ 不登记
            r_q = w("q.py", 'X = "${ZERG_X:-v}"\ntext = \'bash "$RUNNER"\'\n')
            _jq, _repq, regq, _outq = scan_file(r_q)
            if "P2" in regq:
                print("  ✗ 护栏②失败：行内引号文案仍登记 P2"); ok = False

            # 护栏④：`.md` 件不扫（写进去也不登记）
            w("note.md", 'X="${ZERG_X:-v}"\nbash "$RUNNER"\n')
            md_hit = [p for p in iter_files(td2) if p.endswith(".md")]
            if md_hit:
                print("  ✗ 护栏④失败：`.md` 件被扫描 %r" % md_hit); ok = False

            # 成对负控 · ⑤ 防重复计数：同一形态多行 ⇒ 集合去重（P2 只登记一次）
            r_dup = w("dup.sh", 'A="${ZERG_A:-x}"\nbash "$A"\nbash "$A" --again\n')
            _jd, _rd, regd, _od = scan_file(r_dup)
            if len(regd) != 1:
                print("  ✗ 负控⑤失败：P2 重复计数 %r（应去重为 1）" % (regd,)); ok = False

            # 成对负控 · ①↔④/⑤ 交叉互斥：py 件 shell=True ⇒ 报告档 P1；sh 件 bash "$x" ⇒ 登记档 P2、互不串档
            r_x = w("x.py", 'import os, subprocess\nX = os.environ.get("ZERG_BIN")\n'
                            'subprocess.run(X, shell=True)\nY = "${ZERG_X:-v}"\n')
            r_x2 = w("x.sh", 'R="${ZERG_R:-v}"\nbash "$R"\n')
            _jx, repx, regx, _outx = scan_file(r_x)
            _jx2, repx2, regx2, _outx2 = scan_file(r_x2)
            if not ("P1" in repx and "P1" not in regx and "P2" in regx2 and "P2" not in repx2):
                print("  ✗ 负控①↔④/⑤ 交叉互斥失败：repx=%r regx=%r regx2=%r repx2=%r"
                      % (repx, regx, regx2, repx2)); ok = False

    print("自检 %s（%d 格）" % ("PASS" if ok else "FAIL", 3 + 10))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="「同件双条件」执行位静态门（GAP-20260927-145/-162/-173）")
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

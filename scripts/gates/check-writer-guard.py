#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-writer-guard.py —— 门禁面口径：**写者守卫旁路探测器**（缺口 GAP-20260926-94 施工清单第 5 件）。

病灶（现读 `scripts/dev/atomic-write.sh` 件头逐字）：
    改门脚本这件上此前没有任何保护，两个写者会整件互盖（本夜真出过一次）。
已落地的两道正门 —— `scripts/dev/atomic-write.sh`（换件正门：覆盖前留档 + 回读校验 + 写前预校）与
`scripts/dev/writer-lock.sh`（写者锁：mkdir 原子锁、绝不自动破锁）——**只管走正门的写者**。
本件补的是它们的**反面**：仓内还有没有脚本**绕开这两道正门**、直接对仓内件做整件覆盖写。

★ 本件**只读**：不写仓、不建库、不起服务、不联网、不 push、不起后台常驻进程；只读被扫脚本的正文。
★ 本件**不参与任何写入**，只在提交 / 巡检时回答一个问题：
    「仓内有没有哪个脚本在整件覆盖写仓内件时绕过了 atomic-write.sh / writer-lock.sh？」

判据（一刀，可机检）：
    命中 = 一行里出现**整件覆盖写**动作，且它的**落点是一个字面仓内相对路径**，而且该脚本正文里
           **没有**任何 `atomic-write.sh` 的调用面 —— 即「会写仓内件、却没走换件正门」。
    不命中 = ① 落点含变量（`${tmp}` / `$(mktemp)` / `$OUT` …）⇒ 落的是临时面而非仓内件；
             ② 脚本正文里出现 `atomic-write.sh` ⇒ 它是走正门的写者（合规写法）。
    ★ 判红面 = 源码面（`scripts/` · `publish/` · `.githooks/` · `core/` · `agent/`）；
      产物面（`bin/` · `dist/` · `data/`）不判 —— 那是构建产物不是仓内源码件。
    ★ 注释行（行首 `#`）不判 —— 说明文字里的写法不是真动作。

退码（三档 · 照拄在册同族 `scripts/gates/check-ai-direct-edits.py` / `scripts/dev/atomic-write.sh` ·
本件不新立命名空间 · 禁自创码）：
    0 = 全绿（真目标**零命中**）—— 或 `--report-only` 干跑档（命中只登记、不改退码）
    1 = 红（真目标**命中绕过**，逐条给 件:行 + 逐字命中行 + 为何判为绕过）
    2 = 不给结论（用法错 / 未知旗标 / 扫描根读不到 / 扫到 0 个脚本 ⇒ 空转不当绿 / `--self-test` 不过）

用法：
    python3 scripts/gates/check-writer-guard.py                       # 扫本仓（默认根 = 本件上溯三级）
    python3 scripts/gates/check-writer-guard.py --root <目录>          # 扫别的根（夹具/负控用）
    python3 scripts/gates/check-writer-guard.py --report-only          # 干跑档：命中只登记、退码恒 0
    python3 scripts/gates/check-writer-guard.py --json hits,count      # 人面 JSON（字段面按下面 FIELDS）
    python3 scripts/gates/check-writer-guard.py --self-test            # 合成夹具：3 负控（必命中）+ 2 正控（不得命中）
    python3 scripts/gates/check-writer-guard.py --help                 # 只印用法，不跑动作

FIELDS（`--json` 可选字段闭集 · 不在闭集 ⇒ 退 2 并点名）：
    root, scanned, count, hits, report_only, compliant, rc, verdict
"""

import argparse
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates -> 仓根

FIELDS = ("root", "scanned", "count", "hits", "report_only", "compliant", "rc", "verdict",
          "write_ledger_missing", "write_ledger_stale")

# ── 写工具分类台账腿（G0/G1/G2 · 名录收写工具 · 强制危险档的**分类真源机检**）──────────────
#   真源 = 每件自己头部的一行 `#zerg-class: write D2|D3`（同 core/cmd/zerg/main.go 的 dangerSpec 形状）。
#   ★ 本串与 core/cmd/zerg/family_gate_check.go 的 `gateCheckClassRE` **同一串**（语义逐字同）。
CLASS_RE = re.compile(r"^#\s*zerg-class:\s*(read|write)\s+(D0|D2|D3)\s*$")
LEDGER_REL = os.path.join("scripts", "gates", "gate-check-write-tools.json")
LEDGER_SCHEMA = "gate-check-write-tools/1"
ROSTER_SUFFIXES = (".py", ".sh")  # 与 gateCheckRoster 的扩展名闭集同源


# 扫描面：只收脚本件（后缀族 + 无后缀但有 shebang）
SCRIPT_SUFFIXES = (".sh", ".bash", ".zsh", ".py")
SKIP_DIRS = {
    ".git", ".obsidian", "node_modules", "vendor", "target", "__pycache__",
    "dist", "bin", "data", "zerg-wt", ".zerg", "venv", ".venv", ".cargo",
    "htmlcov", ".history", "_history", "scratch",
}

# 判红面 = 源码面（产物面 bin/ dist/ data/ 不在内）
REPO_SRC_RE = re.compile(r"^(?:scripts|publish|core|agent|docs|\.githooks|tools)/[A-Za-z0-9._/@+-]+$")

# 整件覆盖写的四种现成形态（逐条现读本仓：「先现读全仓到底哪些脚本真的在覆盖写仓内件，再定搜索面」）
REDIRECT_RE = re.compile(r"(?<![0-9>&])>(?![>&=])\s*([^\s;|)&]+)")          # > 落点（truncate 覆盖）
CPMV_RE = re.compile(r"(?:^|[;&|]|\s)(?:cp|mv|install)\s+(?:-[A-Za-z-]+\s+)*\S+\s+([^\s;|)&]+)")
TEE_RE = re.compile(r"\btee\s+(?:-a\s+)?([^\s;|)&]+)")
DD_RE = re.compile(r"\bof=([^\s;|)&]+)")
PYOPEN_RE = re.compile(r"\bopen\s*\(\s*([^,)]+)\s*,\s*['\"](?:w|wb|wt|w\+)['\"]")
# Python 的落点在**实参位**上，不在内容位上 —— 两条分开写（照拄「write_text 的第一实参是内容」这一事实）
PYW_TEXT_RE = re.compile(r"\.(?:write_text|write_bytes)\s*\(")
PYW_PATH_RE = re.compile(r"\b(?:Path|PurePath)\s*\(\s*([^,)]+)")
PYW_COPY_RE = re.compile(r"\bshutil\.(?:copy|copy2|copyfile)\s*\(\s*[^,]+,\s*([^,)]+)")

FORMS = (
    ("REDIRECT", REDIRECT_RE, "重定向 `>` 整件覆盖落点"),
    ("CP/MV", CPMV_RE, "`cp`/`mv`/`install` 整件覆盖落点"),
    ("TEE", TEE_RE, "`tee` 落点（同名件被覆盖）"),
    ("DD", DD_RE, "`dd of=` 整件覆盖落点"),
    ("PY-OPEN-W", PYOPEN_RE, "Python `open(..., 'w')` 整件覆盖落点"),
)

GUARD_TOKEN = "atomic-write.sh"  # 走正门的标记（出现即视为「已声明走换件正门」）


def usage_text():
    return (
        "用法\n"
        "  python3 scripts/gates/check-writer-guard.py [--root <目录>] [--report-only]\n"
        "  python3 scripts/gates/check-writer-guard.py --json <字段,…>\n"
        "  python3 scripts/gates/check-writer-guard.py --self-test\n"
        "  python3 scripts/gates/check-writer-guard.py --help\n"
        "说明\n"
        "  只读旁路探测器：扫根下脚本，找「整件覆盖写仓内源码件、却没走 scripts/dev/atomic-write.sh」的行。\n"
        "  --root        扫描根（默认 = 本件上溯三级的仓根）\n"
        "  --report-only 干跑档：命中只登记、退码恒 0\n"
        "  --json        人面 JSON（字段闭集：" + ",".join(FIELDS) + "）\n"
        "  --self-test   合成夹具：3 负控（必命中）+ 2 正控（不得命中）\n"
        "退码\n"
        "  0 全绿 / 干跑档 · 1 红（命中绕过）· 2 不给结论（用法错 / 未知旗标 / 根读不到 / 扫到 0 个脚本 / 自检不过）\n"
    )


def is_repo_literal(tok, root=None):
    """字面仓内相对路径？（含变量 ⇒ 不是；绝对路径 ⇒ 不是）

    ★ 判别力硬要求（现读全仓普查后的收口）：目标必须是**盘上真存在的仓内件** ——
      「整件覆盖」判的是**换掉一个已有的件**，不是**新建一个件**。不加这一格时，
      本仓的假阳全部来自两类非写者：
        ① 自检夹具在**临时树**里造的 `core/main.go` / `scripts/run.sh` / `docs/zh/s.md`
           （这些路径在主仓**根本不存在**，是 `cd "$tmp"` 之后造出来的）；
        ② 文档/用法串里被当成样例打印的同形写法（`tools/gen_tools_md.py` 件头）。
      加了「目标件必须现读在盘」这一格，两类同时落地为不命中（现读 16 → 0）。
      ★ 代价（照实登记）：对**尚不存在的路径**的整件覆盖**看不见**（那不是换件是新建）——
        本件的口径就是「换件正门的旁路」，不含「新建件」。
    """
    t = tok.strip().strip("'\"")
    if not t or "$" in t or t.startswith("/") or t.startswith("~"):
        return False
    if REPO_SRC_RE.match(t) is None:
        return False
    if root is not None and not os.path.isfile(os.path.join(root, t)):
        return False
    return True


def scan_file(path, rel, root=None):
    """返回该件的命中列表 [(行号, 逐字行, 形态名, 说明, 落点)]。"""
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            text = f.read()
    except OSError:
        return None
    if GUARD_TOKEN in text:            # 走正门的写者 = 合规写法 ⇒ 整件跳过
        return []
    hits = []
    in_doc = False
    for i, line in enumerate(text.splitlines(), 1):
        toggle = line.count('"""') + line.count("'''")
        if in_doc:                      # 三引号块内（Python 文档串/长串）不判 —— 说明文字不是真动作
            if toggle % 2 == 1:
                in_doc = False
            continue
        if toggle % 2 == 1:
            in_doc = True
        s = line.strip()
        if not s or s.startswith("#"):  # 注释行 / 空行不判
            continue
        for name, rx, why in FORMS:
            for m in rx.finditer(line):
                tok = m.group(1)
                if is_repo_literal(tok, root):
                    hits.append((i, line.rstrip("\n"), name,
                                 why + "（" + name + "）", tok.strip().strip("'\"")))
        # Python：write_text/write_bytes 的落点在 Path(…) 实参位上
        if PYW_TEXT_RE.search(line):
            for m in PYW_PATH_RE.finditer(line):
                tok = m.group(1)
                if is_repo_literal(tok, root):
                    hits.append((i, line.rstrip("\n"), "PY-PATHLIB",
                                 "Python `write_text`/`write_bytes` 的 `Path(…)` 落点整件覆盖（PY-PATHLIB）",
                                 tok.strip().strip("'\"")))
        for m in PYW_COPY_RE.finditer(line):
            tok = m.group(1)
            if is_repo_literal(tok, root):
                hits.append((i, line.rstrip("\n"), "PY-SHUTIL-COPY",
                             "Python `shutil.copy*` 的目标实参整件覆盖（PY-SHUTIL-COPY）",
                             tok.strip().strip("'\"")))
    return hits


def collect_scripts(root):
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            p = os.path.join(dirpath, fn)
            rel = os.path.relpath(p, root)
            if fn.startswith(".") and not fn.startswith(".githooks"):
                continue
            if fn.endswith(SCRIPT_SUFFIXES):
                out.append((p, rel))
                continue
            if "." not in fn:  # 无后缀件：有执行位或有 shebang 才算脚本
                try:
                    if os.access(p, os.X_OK):
                        out.append((p, rel)); continue
                    with open(p, "rb") as f:
                        if f.read(2) == b"#!":
                            out.append((p, rel))
                except OSError:
                    pass
    return out


def parse_decl(path):
    """现读件**头部**（前 80 行）的 `#zerg-class:` 声明 ⇒ (class, level)；无 ⇒ None。"""
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            for i, line in enumerate(f):
                if i >= 80:
                    break
                m = CLASS_RE.match(line.rstrip("\r\n"))
                if m:
                    return m.group(1), m.group(2)
    except OSError:
        return None
    return None


def collect_declared(root):
    """递归列举 `<root>/scripts/` 的 `.py`/`.sh`（与 gateCheckRoster 同域），收有声明行的件。"""
    out = {}
    base = os.path.join(root, "scripts")
    for dirpath, dirnames, filenames in os.walk(base):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            if not fn.endswith(ROSTER_SUFFIXES):
                continue
            p = os.path.join(dirpath, fn)
            d = parse_decl(p)
            if d:
                out[os.path.relpath(p, root)] = d
    return out


def load_ledger(root):
    with open(os.path.join(root, LEDGER_REL), encoding="utf-8") as f:
        return json.load(f)


def check_ledger(root):
    """G0 解析自证（⇒2）· G1 双向差集（⇒1 点名）· G2 覆盖腿（⇒1 点名）。返回 (rc, info, notes)。"""
    info = {"write_ledger_missing": 0, "write_ledger_stale": 0}
    notes = []
    # ── G0 解析自证（不合 ⇒ rc=2 不给结论）─────────────────────────────────
    try:
        led = load_ledger(root)
    except Exception as e:
        return 2, info, ["⛔ G0 不给结论：台账读不到/解析失败（%s）：%s" % (LEDGER_REL, e)]
    if led.get("schema") != LEDGER_SCHEMA:
        return 2, info, ["⛔ G0 不给结论：台账 schema 不对（%r ≠ %r）" % (led.get("schema"), LEDGER_SCHEMA)]
    try:
        re.compile(led.get("regex", ""))
    except re.error as e:
        return 2, info, ["⛔ G0 不给结论：台账 regex 编译不过：%s" % e]
    items = led.get("items")
    if not isinstance(items, list) or not items:
        return 2, info, ["⛔ G0 不给结论：台账 items 空集（空转不当绿）"]
    if led.get("count") != len(items):
        return 2, info, ["⛔ G0 不给结论：台账 count(%r) ≠ items 数(%d)" % (led.get("count"), len(items))]
    for it in items:
        f = it.get("file") or ""
        if not f.startswith("scripts" + os.sep) and not f.startswith("scripts/"):
            return 2, info, ["⛔ G0 不给结论：台账点名件不在 `scripts/` 域（%r）" % f]
        if not os.path.isfile(os.path.join(root, f)):
            return 2, info, ["⛔ G0 不给结论：台账点名件不在盘（%r）" % f]
    # ── G1 双向差集（⇒ rc=1 逐条点名）──────────────────────────────────────
    declared = collect_declared(root)
    lset = {it.get("file"): (it.get("class"), it.get("level")) for it in items}
    stale = sorted(f for f in declared if f not in lset)                 # 件头声明了、台账没登记（新增漏登记）
    missing = sorted(f for f in lset if f not in declared)               # 台账点名、件头无声明（陈旧）
    mismatch = sorted("%s(台账 %r ⟷ 件头 %r)" % (f, lset[f], declared[f])
                      for f in lset if f in declared and lset[f] != declared[f])
    info["write_ledger_missing"] = len(missing) + len(mismatch)
    info["write_ledger_stale"] = len(stale)
    rc = 0
    if stale:
        rc = 1
        for f in stale:
            notes.append("G1 新落写工具漏登记（件头声明了 · 台账 items 无）：%s ⇒ 同批登记 `%s`" % (f, LEDGER_REL))
    if missing:
        rc = 1
        for f in missing:
            notes.append("G1 台账陈旧（台账点名 · 件头无声明）：%s" % f)
    if mismatch:
        rc = 1
        for d in mismatch:
            notes.append("G1 class/level 与件头不符：%s" % d)
    # ── G2 覆盖腿（凡正文命中写形态的件必须显式标 write ⇒ rc=1 点名）─────────
    base = os.path.join(root, "scripts")
    if os.path.isdir(base):
        for p, rel in sorted(collect_scripts(base), key=lambda x: x[1]):
            hits = scan_file(p, rel, root)
            if not hits:
                continue
            decl = parse_decl(p)
            if decl is None or decl[0] != "write":
                rc = 1
                info["write_ledger_missing"] += 1
                notes.append("G2 漏标写形态：scripts/%s 命中整件覆盖写（%d 处）却未显式声明 `#zerg-class: write D2|D3`"
                             % (rel, len(hits)))
    return rc, info, notes


def check(root, report_only=False):
    """返回 (rc, info). rc: 0 / 1 / 2."""
    info = {"root": root, "scanned": 0, "count": 0, "hits": [],
            "report_only": bool(report_only), "compliant": 0}
    if not os.path.isdir(root):
        return 2, info, ["⛔ 不给结论：扫描根读不到（%s）—— 空转不当绿" % root]
    scripts = collect_scripts(root)
    if not scripts:
        return 2, info, ["⛔ 不给结论：扫描面 0 个脚本件（%s）—— 空转不当绿" % root]
    info["scanned"] = len(scripts)
    for p, rel in sorted(scripts, key=lambda x: x[1]):
        hs = scan_file(p, rel, root)
        if hs is None:
            continue
        if not hs:
            info["compliant"] += 1
            continue
        for (ln, txt, form, why, dst) in hs:
            info["hits"].append({"file": rel, "line": ln, "text": txt,
                                 "form": form, "why": why, "target": dst})
    info["count"] = len(info["hits"])
    if report_only:
        return 0, info, []
    if info["count"]:
        return 1, info, ["%s:%d  [%s] %s ⇒ 整件覆盖写仓内件 %s 而未走 %s（判为绕过）"
                         % (h["file"], h["line"], h["form"], h["text"], h["target"], GUARD_TOKEN)
                         for h in info["hits"]]
    return 0, info, []


def print_human(info, notes, limit=3):
    if notes:
        for n in notes:
            sys.stderr.write(n + "\n")
    if info["count"] == 0:
        print("写者守卫旁路：零命中 ✓（扫 %d 个脚本件 · 零命中件 %d · 根 %s）"
              % (info["scanned"], info["compliant"], info["root"]))
        return
    print("写者守卫旁路：命中 %d 条（扫 %d 个脚本件）· 聚合计数如下，逐条面用 --json hits"
          % (info["count"], info["scanned"]))
    buckets = {}
    for h in info["hits"]:
        buckets[h["form"]] = buckets.get(h["form"], 0) + 1
    for k in sorted(buckets):
        print("   %-10s %d 条" % (k, buckets[k]))
    for h in info["hits"][:limit]:
        print("   %s:%d  [%s]  %s" % (h["file"], h["line"], h["form"], h["text"]))


def selftest():
    """Selftest fixture: 3 negative controls (must hit) + 2 positive controls (must not hit)."""
    tmp = tempfile.mkdtemp(prefix="writer-guard-selftest.", dir=os.environ.get("TMPDIR"))
    checks = []

    def chk(name, cond, detail=""):
        checks.append((name, bool(cond), detail))

    try:
        fake = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(fake, "scripts", "gates"))
        os.makedirs(os.path.join(fake, "scripts", "dev"))
        # 待覆盖的目标件（真存一件，好让夹具像真仓）
        with open(os.path.join(fake, "scripts", "gates", "demo.sh"), "w", encoding="utf-8") as f:
            f.write("OLD\n")

        # ── 负控（必须命中）───────────────────────────────────────────────
        with open(os.path.join(fake, "scripts", "bad-cp.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\n# 负控①\ncp /tmp/new scripts/gates/demo.sh\n")
        with open(os.path.join(fake, "scripts", "bad-redirect.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\n# 负控②\nprintf 'NEW\\n' > scripts/gates/demo.sh\n")
        with open(os.path.join(fake, "scripts", "bad-py.py"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env python3\n# 负控③\nfrom pathlib import Path\n"
                    "Path('scripts/gates/demo.sh').write_text('NEW')\n")
        # ── 正控（不得命中）───────────────────────────────────────────────
        with open(os.path.join(fake, "scripts", "ok-atomic.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\n# 正控①\n"
                    "bash scripts/dev/atomic-write.sh --target scripts/gates/demo.sh --from /tmp/new\n")
        with open(os.path.join(fake, "scripts", "ok-tmp.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\n# 正控②\n"
                    'tmp="$(mktemp -d)"\nprintf NEW > "${tmp}/demo.sh"\ntee "$OUT" >/dev/null\n')

        rc, info, notes = check(fake, report_only=False)
        names = sorted(os.path.basename(h["file"]) for h in info["hits"])
        cnt = {}
        for h in info["hits"]:
            cnt[os.path.basename(h["file"])] = cnt.get(os.path.basename(h["file"]), 0) + 1
        chk("① 负控 bad-cp.sh 必须命中", cnt.get("bad-cp.sh", 0) >= 1, "命中 %d" % cnt.get("bad-cp.sh", 0))
        chk("② 负控 bad-redirect.sh 必须命中", cnt.get("bad-redirect.sh", 0) >= 1,
            "命中 %d" % cnt.get("bad-redirect.sh", 0))
        chk("③ 负控 bad-py.py 必须命中", cnt.get("bad-py.py", 0) >= 1, "命中 %d" % cnt.get("bad-py.py", 0))
        chk("④ 正控 ok-atomic.sh 不得命中", cnt.get("ok-atomic.sh", 0) == 0,
            "命中 %d" % cnt.get("ok-atomic.sh", 0))
        chk("⑤ 正控 ok-tmp.sh 不得命中", cnt.get("ok-tmp.sh", 0) == 0, "命中 %d" % cnt.get("ok-tmp.sh", 0))
        chk("⑥ 负控件整体判红（rc=1）", rc == 1, "rc=%d" % rc)
        chk("⑦ 区别力：命中件集 == 3 个负控件", sorted(set(names)) == ["bad-cp.sh", "bad-py.py", "bad-redirect.sh"],
            "命中件 = %s" % ",".join(sorted(set(names))))

        # ── 空转负控：空根 ⇒ rc=2（不当绿）──────────────────────────────
        empty = os.path.join(tmp, "empty")
        os.makedirs(empty)
        rc2, _, _ = check(empty)
        chk("⑧ 空根 ⇒ rc=2 不给结论", rc2 == 2, "rc=%d" % rc2)

        # ── 干跑档负控：同一夹具 report_only ⇒ rc=0 但计数不变 ──────────────
        rc3, info3, _ = check(fake, report_only=True)
        chk("⑨ 干跑档 rc=0 且计数不丢", rc3 == 0 and info3["count"] == info["count"],
            "rc=%d count=%d" % (rc3, info3["count"]))

        # ── 台账腿（G0/G1/G2 成对负控 · 写工具分类真源机检）────────────────────
        lfake = os.path.join(tmp, "lrepo")
        os.makedirs(os.path.join(lfake, "scripts", "dev"))
        os.makedirs(os.path.join(lfake, "scripts", "gates"))
        with open(os.path.join(lfake, "scripts", "gates", "demo.sh"), "w", encoding="utf-8") as f:
            f.write("OLD\n")
        with open(os.path.join(lfake, "scripts", "dev", "w.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\n#zerg-class: write D2\nprintf NEW > scripts/gates/demo.sh\n")
        lf = os.path.join(lfake, "scripts", "gates", "gate-check-write-tools.json")

        def wr_ledger(items):
            with open(lf, "w", encoding="utf-8") as f:
                json.dump({"schema": LEDGER_SCHEMA, "head": "x", "count": len(items),
                           "regex": CLASS_RE.pattern, "items": items}, f)

        wr_ledger([{"file": "scripts/dev/w.sh", "class": "write", "level": "D2"}])
        lrc, _, _ = check_ledger(lfake)
        chk("⑩ 台账腿正控（声明齐 + 台账齐）⇒ rc=0", lrc == 0, "rc=%d" % lrc)

        with open(os.path.join(lfake, "scripts", "dev", "x.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\necho hi\n")
        wr_ledger([{"file": "scripts/dev/w.sh", "class": "write", "level": "D2"},
                   {"file": "scripts/dev/x.sh", "class": "write", "level": "D2"}])
        l2, _, ln2 = check_ledger(lfake)
        chk("⑪ 台账多一条 ⇒ rc=1 且点名", l2 == 1 and any("x.sh" in n for n in ln2), "rc=%d" % l2)

        os.remove(os.path.join(lfake, "scripts", "dev", "x.sh"))
        with open(os.path.join(lfake, "scripts", "dev", "y.sh"), "w", encoding="utf-8") as f:
            f.write("#!/usr/bin/env bash\nprintf NEW > scripts/gates/demo.sh\n")
        wr_ledger([{"file": "scripts/dev/w.sh", "class": "write", "level": "D2"}])
        l3, _, ln3 = check_ledger(lfake)
        chk("⑫ 未标写形态件 ⇒ rc=1 且点名", l3 == 1 and any("y.sh" in n for n in ln3), "rc=%d" % l3)
        os.remove(os.path.join(lfake, "scripts", "dev", "y.sh"))

        wr_ledger([])
        l4, _, _ = check_ledger(lfake)
        chk("⑬ 台账空集 ⇒ rc=2 不给结论", l4 == 2, "rc=%d" % l4)

        bad = [c for c in checks if not c[1]]
        for name, ok, det in checks:
            print("   %s %s%s" % ("✓" if ok else "✗", name, ("（%s）" % det) if det else ""))
        if bad:
            sys.stderr.write("⛔ 自检不过 %d 格 ⇒ 退 2（不给结论）\n" % len(bad))
            return 2
        print("   ── 自检 %d 格全过（3 负控必命中 · 2 正控不得命中 · 空转必 2 · 干跑档同计数）──" % len(checks))
        return 0
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def emit_json(info, rc, fields):
    data = {
        "root": info["root"], "scanned": info["scanned"], "count": info["count"],
        "hits": info["hits"], "report_only": info["report_only"],
        "compliant": info["compliant"], "rc": rc,
        "verdict": {0: "green", 1: "red", 2: "blocked"}[rc],
        "write_ledger_missing": info.get("write_ledger_missing", 0),
        "write_ledger_stale": info.get("write_ledger_stale", 0),
    }
    print(json.dumps({k: data[k] for k in fields}, ensure_ascii=False, indent=2))


def main(argv):
    ap = argparse.ArgumentParser(add_help=False)
    ap.add_argument("--root", default=None)
    ap.add_argument("--json", dest="json_fields", default=None)
    ap.add_argument("--report-only", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--help", action="store_true")
    args, unknown = ap.parse_known_args(argv)
    if args.help:
        sys.stdout.write(usage_text())
        return 0
    if unknown:
        sys.stderr.write("⛔ 不给结论（退 2）：未知旗标 " + " ".join(unknown)
                         + " ⇒ 本件只认 --root / --json / --report-only / --self-test / --help\n")
        sys.stderr.write(usage_text())
        return 2
    fields = None
    if args.json_fields is not None:
        fields = [f.strip() for f in args.json_fields.split(",") if f.strip()]
        bad = [f for f in fields if f not in FIELDS]
        if not fields or bad:
            sys.stderr.write("⛔ 不给结论（退 2）：--json 字段不在闭集 " + ",".join(FIELDS)
                             + ("；点名非法字段 " + ",".join(bad) if bad else "；字段面为空") + "\n")
            return 2
    if args.self_test:
        print("  —— check-writer-guard.py --self-test（合成夹具）——")
        return selftest()
    root = args.root or DEFAULT_ROOT
    report_only = bool(args.report_only)
    rc, info, notes = check(root, report_only=report_only)
    lrc, linfo, lnotes = check_ledger(root)
    info.update(linfo)
    notes = notes + lnotes
    if not report_only:
        rc = max(rc, lrc)
    if fields is not None:
        emit_json(info, rc, fields)
        return rc
    print_human(info, notes)
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

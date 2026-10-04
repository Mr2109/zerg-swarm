#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""门禁：抓取 / 解析纪律 —— 必须用**解析器**或**显式分界符**，不许「差不多对」的正则兜。

令（任务单 `任务清单-本版全做-v2.5.12-20260924.md` §一 序124 = 组4 §二.4 `W-42` ·
`研-归:156` §五 栗②）逐字：

    抓取 / 解析要有解析器或显式分界符（`grep -o '--strict-report'` 被当选项 ⇒ 必须 `-e`；
    `sed 's/<[^>]*>//g'` 剥标签会整段吞正文）—— 不许用「差不多对」的正则兜

验收判据（该行逐字）：判据栏 = 「抓取脚本逐条用解析器或显式分界符；**负控**：`grep -o`
不以 `-e`/`--` 起 ⇒ 判红」。

为什么这条要成门（两条**实证**，不是风格偏好）
------------------------------------------------
1) `grep -o '--strict-report' <件>` —— GNU/BSD `grep` 用 getopt 解析参数：`'--strict-report'` 以 `-` 起头，
   会被**当成选项**吃掉 ⇒ 要么报 `unrecognized option`、要么在别的变体上静默抓不到。**抓不到
   而不报错**，正是「差不多对」最贵的失败模式：它把「0 命中」伪装成「已判过」。
   修法只有两种：`grep -o -e '--strict-report'`、或 `grep -o -- '--strict-report'`（**显式分界符**）。
2) `sed 's/<[^>]*>//g'` —— 用贪婪正则剥 HTML/XML 标签：标签**未闭合**或**跨行**时，`[^>]*`
   会一路吃到下一个 `>`（或到行尾整段吞掉正文）⇒ 抓出来的「文本」少了半段，而**症状是静默的**
   （行数变了、内容变了，没有一处报错）。剥标签要么用**解析器**（`html.parser` / `xml.etree`），
   要么先划出显式分界符再取。

判据（三条 · 逐条可机检）
------------------------
- **P1 `grep` 模式实参以 `-` 起头**：对 `scripts/` 面下每一处 `grep` 调用（含嵌在别的命令串里的
  一层），把参数按 **`shlex` 解析器**切开（**不用正则兜**），逐 token 判：出现一个「以 `-` 起头、
  **不在**已知选项表里、且**不是** `-e`/`--regexp`/`--` 的实参」的 token ⇒ **命中**。
  （已知选项表 = grep 的短/长选项 + `-A<n>`/`-B<n>`/`-C<n>`/`-m<n>` 这类带数值的形态。）
- **P2 `sed` 剥标签**：`sed` 的脚本实参里出现字面串 `<[^>]*>`（即贪婪标签剥离）⇒ **命中**。
- **P3 空转不许当绿**：扫描面 = 0 件、或用法错、或 `--self-test` 不过 ⇒ **rc=2（不给结论）** ——
  「没扫到」不等于「扫过且干净」。

退码：`0` 六面全过 / `1` 有命中（逐条点名「件:行 + 原样」）/ `2` **不给结论**（缺件 · 空转 · 自检不过）。

用法
----
    python3 scripts/gates/check-parser-discipline.py [仓根] [--json] [--self-test] [--list-forms]

`--json`：机器读面（`{"schema":…,"items":[…],"meta":{…}}`），与本版六键包封同形（只读面）。
`--list-forms`：逐条列本门认的**合法形态**（写给「我到底该怎么写」）。
"""

import argparse
import json
import os

import sys
import tempfile

SCHEMA = "zerg-gate/1"
KIND = "ParserDiscipline"

# ── 扫描面：`scripts/` 面下的 .sh / .py / 无后缀 shebang 件（排除本件自身 · 排除构建产物）──
SCAN_SUBDIRS = ("", "build", "calib", "docs", "evals", "gates", "sandbox", "svc", "wall", "edge-node")
SCAN_EXTS = (".sh", ".py")
SKIP_DIR_NAMES = {".git", "__pycache__", ".venv", "venv", "node_modules", "target", "dist", "bin", "_history"}

# 自指豁免（**打印理由**，不是静默放过）：本件正文里必然出现 `grep -o '--strict-report'`
# 与 `sed 's/<[^>]*>//g'` 两个**反面教材字面**（判据栏逐字），若不自豁免 ⇒ 门自己恒红。
SELF_EXEMPT_BASENAME = "check-parser-discipline.py"

# ── P1：grep 的已知选项表（短选项可捆绑 · 数值选项 `-A3` 形态单列）────────────────────
# `I` = GNU grep 的「忽略二进制件」（--binary-files=without-match）—— 原表漏了它。
# 它由本次「宿主语素归一」暴露：先前 argv 形态的真实调用（subprocess.run([... "grep",
# "-I", ...])）里 grabber 被逗号粘连（`grep,`）⇒ 整条调用**根本没进过判据**；归一化后显形，
# `-I` 必须被认成选项（不是「以 - 起头的模式实参」）。补表 = 认识一个真选项，**判据一字未动**。
GREP_SHORT_FLAGS = set("EeFfGivclLnhHoqswxrRPaAbmzdDZU") | {"I"}
GREP_LONG_OPTS = {
    "regexp", "file", "ignore-case", "invert-match", "count", "files-with-matches",
    "files-without-match", "line-number", "no-filename", "with-filename", "only-matching",
    "quiet", "silent", "no-messages", "recursive", "dereference-recursive",
    "extended-regexp", "fixed-strings", "basic-regexp", "perl-regexp", "word-regexp",
    "line-regexp", "byte-offset", "label", "text", "binary-files", "exclude", "exclude-dir",
    "include", "directories", "devices", "after-context", "before-context", "context",
    "max-count", "null", "null-data", "initial-tab", "color", "colour", "no-ignore-case",
    "help", "version", "group-separator", "no-group-separator", "binary",
}
NUMERIC_SHORT = set("ABCm")

# 抓取/解析面：只有这些调用算「抓取器」（`grep` / `sed` / `awk` 里的取数形态）
GRABBERS = {"grep", "sed", "awk"}

# ── P2：剥标签（贪婪）字面 ──────────────────────────────────────────────────────────
TAG_STRIP_LITERAL = "<[^>]*>"

# ── 宿主语言标点（**同族口径** · 先例 = scripts/gates/check-git-path-outlet.py:292 的
#    `STRIP_CHARS`、scripts/gates/check-verdict-record.py 的「夹具数据 vs 命令位」）──────
# 为什么要有这一层（本门误报的机理）：`_tokenize` 是**字符级状态机**，而 `,` `[` `]` `{` `}`
# `:` `=` 不是 shell 元字符 ⇒ 不切词。于是**源码里的数据字面**（宿主语言字面量列表的一个元素）
#         ".MatchString(", "grep -", "for ", "$(",
# 里的那一段被拼成 token `grep -,`，再被下面「再嵌一层」重切成 `grep` + `-,` ⇒ `-,` 以 `-`
# 起头、又不在已知选项表里 ⇒ 被判成「破折号模式」。**那是数据，不是一次调用**：`grep -` 连
# 模式、连件都没有，抓不到任何东西，不构成 getopt 吞实参那种失败模式（那种形态必有模式实参）。
# ⇒ 判据只看**词素**：归一化时把**首尾**宿主标点剥掉（**只剥首尾**：词内的 `=` 不动，
# 否则 `grep --exclude=dir` 会被剥成未知长选项 ⇒ 反向造假红）。
HOST_PUNCT = "\"'`(),[]{}:=,"


def _norm_host(tok):
    """剥掉**首尾**宿主语言标点 ⇒ 判据只看词素本身（同族口径 · 不是放宽阈值）。"""
    return tok.strip(HOST_PUNCT)


def _is_known_grep_option(tok):
    """tok 是不是 grep 的已知选项（含捆绑短选项与 `-A3` 形态）。"""
    if not tok.startswith("-") or tok == "-":
        return False
    if tok == "--":
        return True
    if tok.startswith("--"):
        return tok[2:].split("=", 1)[0] in GREP_LONG_OPTS
    body = tok[1:]
    if not body:
        return False
    if body[0] in NUMERIC_SHORT and body[1:].isdigit():
        return True
    return all(ch in GREP_SHORT_FLAGS for ch in body)


def _scan_tokens(tokens, lineno):
    """对一个已经按 shlex 切开的 token 串扫 P1/P2；返回命中列表。"""
    hits = []
    i = 0
    n = len(tokens)
    while i < n:
        tok = tokens[i]
        base = os.path.basename(_norm_host(tok))
        if base not in GRABBERS:
            i += 1
            continue
        # 收本调用的实参（到下一个「像命令」的 token 或结束）—— 逐 token 归一到**词素**
        args = []
        j = i + 1
        while j < n:
            a = _norm_host(tokens[j])
            if a in ("|", "||", "&&", ";", ">", ">>"):
                break
            if a == ")" or a == "`" or a == "$(":
                break
            args.append(a)
            j += 1
        if base == "grep":
            hits.extend(_scan_grep_args(args, lineno))
        elif base == "sed":
            hits.extend(_scan_sed_args(args, lineno))
        i = max(j, i + 1)
    return hits


def _scan_grep_args(args, lineno):
    hits = []
    after_e = False
    after_dashdash = False
    for a in args:
        if after_dashdash:
            # 显式分界符之后的第一个 token 就是模式 —— 就算它以 `-` 起头也安全
            after_dashdash = False
            continue
        if after_e:
            after_e = False
            continue
        if a == "--":
            after_dashdash = True
            continue
        if a in ("-e", "--regexp"):
            after_e = True
            continue
        if a == "-" or a == "":
            continue
        if a.startswith("-"):
            if not _is_known_grep_option(a):
                hits.append({
                    "kind": "P1",
                    "line": lineno,
                    "raw": a,
                    "why": "模式实参以 `-` 起头、又不带 `-e`/`--` 分界符 ⇒ getopt 会把它当选项吃掉"
                           "（`grep -o '--strict-report'` 那种抓不到而不报错的形态）",
                })
            continue
        # 第一个非选项 token = 模式位置；到此为止
        break
    return hits


def _scan_sed_args(args, lineno):
    hits = []
    for a in args:
        if a.startswith("-") and a != "-":
            continue
        if TAG_STRIP_LITERAL in a:
            hits.append({
                "kind": "P2",
                "line": lineno,
                "raw": a,
                "why": "贪婪正则剥标签：标签未闭合 / 跨行时 `[^>]*` 会一路吞掉正文（静默少半段）"
                       "⇒ 要么用解析器（html.parser / xml.etree），要么先划显式分界符",
            })
    return hits


def _tokenize(line):
    r"""把一行 shell 切成 token —— **字符级状态机**（引号 / 转义 / 命令替换 / 元字符），不用正则兜。

    为什么不用 `shlex.split` 一行了事：`N=$(grep -o '--x' "$F" | wc -l)` 这种行里 `grep`
    被 `$(` 粘在 `N=` 上 ⇒ `shlex` 切出来的 token 是 `N=$(grep`，`basename` 对不上 `grep`
    ⇒ 整条调用被漏掉。状态机把 `$( ) \` | & ; < >` 与空白都当分界符 ⇒ `grep` 一定独立成 token。
    """
    toks = []
    cur = ""
    i = 0
    n = len(line)
    quote = None
    while i < n:
        ch = line[i]
        if quote:
            if ch == "\\" and quote == '"' and i + 1 < n:
                cur += line[i + 1]
                i += 2
                continue
            if ch == quote:
                quote = None
                i += 1
                continue
            cur += ch
            i += 1
            continue
        if ch in ("'", '"'):
            quote = ch
            i += 1
            continue
        if ch == "\\" and i + 1 < n:
            cur += line[i + 1]
            i += 2
            continue
        if ch == "$" and i + 1 < n and line[i + 1] == "(":
            if cur:
                toks.append(cur)
                cur = ""
            i += 2
            continue
        if ch.isspace() or ch in "()|&;<>`":
            if cur:
                toks.append(cur)
                cur = ""
            i += 1
            continue
        cur += ch
        i += 1
    if cur:
        toks.append(cur)
    # ── 再嵌一层：嵌在**别的命令串**里的抓取调用（`add_step … "… ; grep -o '--x' f ; …"`）──
    #    这一层是「显式分界符」纪律最容易漏的地方：外层引号一裹，抓取串就成了一个 token。
    out = []
    for t in toks:
        out.append(t)
        inner = _norm_host(t)          # 引号外的宿主标点（`,` `)` `]` …）不是内层命令的一部分
        if " " in inner and ("grep" in inner or "sed" in inner or "awk" in inner):
            out.extend(_tokenize(inner.replace("\\$", "$")))
    return out


def scan_file(path, rel):
    hits = []
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            lines = fh.read().splitlines()
    except OSError as exc:                       # pragma: no cover
        return None, str(exc)
    for idx, line in enumerate(lines, start=1):
        stripped = line.lstrip()
        # 注释行不算（说明 / 反面教材写在注释里是对的）
        if stripped.startswith("#") or stripped.startswith("//"):
            continue
        toks = _tokenize(line)
        for h in _scan_tokens(toks, idx):
            h["file"] = rel
            hits.append(h)
    return hits, None


def collect(root):
    files = []
    for sub in SCAN_SUBDIRS:
        d = os.path.join(root, "scripts", sub) if sub else os.path.join(root, "scripts")
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            p = os.path.join(d, name)
            if not os.path.isfile(p):
                continue
            if os.path.basename(p) in SKIP_DIR_NAMES:
                continue
            if not (name.endswith(SCAN_EXTS) or os.access(p, os.X_OK)):
                continue
            files.append(p)
    return files


def run(root, want_json=False):
    files = collect(root)
    if not files:
        return {
            "rc": 2,
            "reason": "扫描面 0 件（不给结论 · 「没扫到」不等于「扫过且干净」）",
            "hits": [],
            "scanned": 0,
            "exempt": [],
        }
    hits = []
    exempt = []
    for p in files:
        rel = os.path.relpath(p, root)
        if os.path.basename(p) == SELF_EXEMPT_BASENAME:
            exempt.append({
                "file": rel,
                "why": "本门自身：判据栏逐字要求正文里出现 `grep -o '--strict-report'` 与 `sed 's/<[^>]*>//g'` "
                       "两个反面教材 ⇒ 自豁免（**打印理由**，不是静默放过）",
            })
            continue
        h, err = scan_file(p, rel)
        if err is not None:
            return {"rc": 2, "reason": f"读不到 {rel}：{err}（不给结论）", "hits": [], "scanned": len(files), "exempt": exempt}
        hits.extend(h)
    rc = 1 if hits else 0
    return {"rc": rc, "reason": "", "hits": hits, "scanned": len(files), "exempt": exempt}


# ── 自检（**成对负控**：正控 ⇒ rc=0 · must-fail ⇒ rc=1 · 真话负控 ⇒ 回绿）──────────────
SELF_TESTS = [
    ("① 正控：解析器形态（`python3 -c` 抽 JSON 字段）⇒ 无命中", "#!/bin/bash\npython3 -c 'import json,sys; print(json.load(sys.stdin)[\"a\"])' < f.json\n", 0),
    ("② 正控：`grep -oE '[0-9]+'`（模式不以 `-` 起头）⇒ 无命中", "#!/bin/bash\nX=$(printf '%s' \"$S\" | grep -oE '[0-9]+' | head -1)\n", 0),
    ("③ must-fail：`grep -o '--strict-report'`（**不以 `-e`/`--` 起**）⇒ rc=1", "#!/bin/bash\nN=$(grep -o '--strict-report' \"$F\" | wc -l)\n", 1),
    ("④ must-fail：`grep -oE '--co'`（捆绑短选项 + 破折号模式）⇒ rc=1", "#!/bin/bash\ngrep -oE '--co' \"$F\"\n", 1),
    ("⑤ must-fail：`grep -c -- '--co'` 的反面 —— `grep -c '--co'`（无 `--`）⇒ rc=1", "#!/bin/bash\ngrep -c '--co' \"$F\"\n", 1),
    ("⑥ 成对回绿：同一模式改 `grep -o -e '--strict-report'`（**显式分界符**）⇒ rc=0", "#!/bin/bash\nN=$(grep -o -e '--strict-report' \"$F\" | wc -l)\n", 0),
    ("⑦ 成对回绿：同一模式改 `grep -o -- '--strict-report'`（`--` 分界符）⇒ rc=0", "#!/bin/bash\nN=$(grep -o -- '--strict-report' \"$F\" | wc -l)\n", 0),
    ("⑧ must-fail：`sed 's/<[^>]*>//g'` 剥标签 ⇒ rc=1", "#!/bin/bash\nT=$(sed 's/<[^>]*>//g' \"$F\")\n", 1),
    ("⑨ 成对回绿：改 `python3 -c 'html.parser'` 剥标签 ⇒ rc=0", "#!/bin/bash\nT=$(python3 -c 'import html,sys; print(html.unescape(sys.stdin.read()))' < \"$F\")\n", 0),
    ("⑩ 成对回绿：普通 `sed 's/x/y/g'`（非剥标签）⇒ rc=0", "#!/bin/bash\nT=$(sed 's/x/y/g' \"$F\")\n", 0),
    ("⑪ 真话负控：把 ③ 那一行**注释掉** ⇒ 回绿（命中的确实是那一行，不是噪声）", "#!/bin/bash\n# N=$(grep -o '--strict-report' \"$F\" | wc -l)\n", 0),
    ("⑫ 一层嵌入：命令串里塞 `grep -o '--x'` ⇒ 仍判得出 ⇒ rc=1", "#!/bin/bash\nadd_step x \"n\" tri \"$R\" \"grep -o '--x' f; rc=$?; exit $rc\"\n", 1),
    # ⑭/⑮ = 本门误报的**两态对拍**（数据字面 vs 真命令位）：⑭ 是本次修的那一处（同形），
    # ⑮ 证**判据没被放宽** —— 同一形态只要落在真命令位上就照样红。
    ("⑭ 正控（**数据字面** · 本门误报根因形态）：宿主语言字面量列表里的 `\"grep -\"` 元素 ⇒ 不判（rc=0）",
     "#!/usr/bin/env python3\nCONSUMERS = (\n    \".MatchString(\", \"grep -\", \"for \", \"$(\",\n)\n", 0),
    ("⑮ 负控（**证未放宽真判据**）：同一形态落在**真命令位**（`grep -o '--x' f`）⇒ 照旧 rc=1",
     "#!/bin/bash\ngrep -o '--x' f\n", 1),
    ("⑯ 正控（补表后）：`git grep -I -n PLACEHOLDER .`（`-I` 是 GNU grep 真选项）⇒ 不判（rc=0）",
     "#!/bin/bash\ngit grep -I -n PLACEHOLDER .\n", 0),
    ("⑰ 负控（**补表没放宽模式实参判据**）：`git grep -I -n '--strict-report' .` ⇒ 照旧 rc=1",
     "#!/bin/bash\ngit grep -I -n '--strict-report' .\n", 1),
]


def self_test(root):
    ok = 0
    bad = 0
    lines = []
    with tempfile.TemporaryDirectory(prefix="w15-parser-") as td:
        sub = os.path.join(td, "scripts")
        os.makedirs(sub, exist_ok=True)
        f = os.path.join(sub, "fixture.sh")
        for name, body, want in SELF_TESTS:
            with open(f, "w", encoding="utf-8") as fh:
                fh.write(body)
            res = run(td)
            got = res["rc"]
            good = (got == want)
            ok += 1 if good else 0
            bad += 0 if good else 1
            lines.append(f"  {'✓' if good else '✗'} {name}（期望 rc={want} · 现跑 rc={got}）")
        # ⑬ 空转 ⇒ rc=2（不给结论 ⇒ 不许当绿）
        empty = os.path.join(td, "scripts-empty")
        os.makedirs(empty, exist_ok=True)
        res = run(empty)
        good = res["rc"] == 2
        ok += 1 if good else 0
        bad += 0 if good else 1
        lines.append(f"  {'✓' if good else '✗'} ⑬ 空转（扫描面 0 件）⇒ rc=2（不给结论 · 不许当绿）（现跑 rc={res['rc']}）")
    print("\n".join(lines))
    print(f"自检结论: {'全过' if bad == 0 else '有失败'}（断言 {ok + bad} 条 · 失败 {bad} 条）")
    return 0 if bad == 0 else 2


LEGAL_FORMS = [
    ("grep 的模式以 `-` 起头", "`grep -o -e '--strict-report' <件>` 或 `grep -o -- '--strict-report' <件>`（显式分界符）"),
    ("从结构化件（json/yaml/toml）取字段", "用解析器（`python3 -c 'json.load'` / `yaml.safe_load`），不用 `grep`/`sed`/`awk` 抠"),
    ("剥 HTML/XML 标签", "用解析器（`html.parser` / `xml.etree`）或先划显式分界符，不用 `s/<[^>]*>//g`"),
    ("同一命令行里有 `|` / `&&` / `;`", "`grep` 之后没有下一条命令；`--` 与 `-e` 的分界符照上一条"),
]


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("root", nargs="?", default=None)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--list-forms", dest="list_forms", action="store_true")
    a = ap.parse_args()

    root = a.root or os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

    if a.list_forms:
        for form, how in LEGAL_FORMS:
            print(f"  · {form}\n      ⇒ {how}")
        return 0

    if a.self_test:
        return self_test(root)

    res = run(root)
    if a.json:
        print(json.dumps({
            "schema": SCHEMA, "kind": KIND,
            "items": [{"file": h["file"], "line": h["line"], "kind": h["kind"], "raw": h["raw"], "why": h["why"]}
                      for h in res["hits"]],
            "meta": {"scanned": res["scanned"], "hits": len(res["hits"]),
                     "exempt": res["exempt"], "reason": res["reason"]},
        }, ensure_ascii=False, indent=2))
        return res["rc"]

    print("── 抓取 / 解析纪律（`scripts/` 面 · **无解析器则必须显式分界符**）──")
    print(f"  扫描 {res['scanned']} 件")
    for e in res["exempt"]:
        print(f"  ⊘ 自豁免：{e['file']} —— {e['why']}")
    for h in res["hits"]:
        print(f"  ✗ {h['file']}:{h['line']}  [{h['kind']}]  {h['raw']}")
        print(f"      {h['why']}")
    if res["rc"] == 0:
        print(f"⇒ 抓取/解析面全过（rc=0）：{res['scanned']} 件里 0 处「破折号模式」0 处「贪婪剥标签」")
    elif res["rc"] == 1:
        print(f"⇒ 有命中 {len(res['hits'])} 处（rc=1）：破折号模式 ⇒ 加 `-e`/`--`；剥标签 ⇒ 换解析器")
    else:
        print(f"⇒ {res['reason']}")
    return res["rc"]


if __name__ == "__main__":
    sys.exit(main())

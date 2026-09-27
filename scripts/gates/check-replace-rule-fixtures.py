#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-replace-rule-fixtures.py —— 替换规则级机检：**命中件不许是测试夹具**（§八 待拍 3 的落点）

设计真源（逐字，不转述）
------------------------
`Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md` §十三 `M8` 逐字：
  「§八 3 提到『私有面保留一条「替换规则命中件不许进测试夹具」机检』✗ 但**形状/判据/落点全无**
    ⇒ 补件名 + **三档退码** + 负控（合成一条替换规则命中 `core/**/*_test.go` ⇒ 必红 ✓）」。
「**公开面替换规则本身是防线，但它会「改坏东西」**：`publish/replace-rules.tsv`
把家目录绝对路径换成 `~/…`、把用户名换成 `Mr2109` …… 这次它改穿了脱敏的必要条件门与两条夹具的
长度假设。**结论：替换规则需要一个「替换后自洽」的机检**」。
★ 本件**不放私有面字面量**（家目录路径 / 用户名 / 内网地址一律写成占位 `~/…` · `Mr2109` · `<worker-ip>`）——
  在它登记进 `scripts/公开标记.tsv`（`public=no`）与两器排除表之前，这样最安全：即使被导出也不泄私有面。

为什么单立这一件（本仓实测，不是设想）
--------------------------------------
公开树 = `whitelist ∪ replace-rules` 的**产物**。替换规则按**字节**改件 —— 它分不清
「要脱敏的源码字符串」与「**测试夹具**」（夹具是**拿字面量与长度当判据**的那种东西）：
  · `core/internal/obs/redact/failclosed_test.go`：夹具里的**家目录绝对路径**被家目录规则换成 `~/…` ⇒
    脱敏**必要条件门**（只认 `/Users/`、`/home/` 两个前缀）整段跳过 ⇒ 公开树
    `LEAK: 秘密 "~/…" 以投影 "~/…" 残留`。
  · `core/internal/obs/redact/limits_test.go`：夹具里含用户名的字符串（21 字节）被用户名规则
    （`→ Mr2109`，**多 2 字节**）改成 23 字节 ⇒ base64 变 32 字符带 `=` ⇒ 打穿夹具自带的
    `len(blob) < 24` 守卫 ⇒ `夹具失效：blob="cGVlciBNcjIxMDkgcnVuIGhlcmUgb2s="（len=32）`。
  · `core/internal/gateway/obs_queued_test.go`：夹具里的**内网地址样例值**被内网地址规则换成
    `"<worker-ip>"` ⇒ 与 `encoding/json` 的 `\u003c` 转义撞车 ⇒ 公开面必红。
⇒ 这三处**都不是环境问题**（本机全绿、公开树一 clone 就红），只有一条尺接得住：**逐条规则 × 逐件夹具**对表。

判据（一条，写死在这里）
------------------------
**`publish/replace-rules.tsv` 的任一条规则，都不许命中「会进公开面的测试夹具」**（`*_test.go`，扫描面见下）。
命中即**逐条点名**（规则原文 · 件 · 行号 · 命中片段）——**不给豁免表**（豁免表就是「把红写进白名单」，正是本件要防的）。

扫描面（两边都写死，缺一即「不给结论」）
----------------------------------------
  · **规则面**：`publish/replace-rules.tsv`（`pattern<TAB>replacement<TAB>说明`；`#` 注释与空行跳过；
    `regex:` 前缀走 `re` —— 与两器同口径：`publish/mirror-public-lib.py` / `scripts/build/publish-public.sh`）。
  · **件面**：`publish/whitelist.txt` 里**会进公开面的**路径前缀下的 `*_test.go`（+ 仓内通用排除表：
    `target` · `node_modules` · `dist` · `bin` · `vendor` · `data` · `.git` · `.venv` · `venv` · `.build`）。
    白名单不给 ⇒ 判据不可判（rc=2）——**不许**退化成「全仓 *_test.go 扫一遍」（扫描面一变，
    红因就变了：私有面夹具被自己的替换规则命中不算病）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿（一条规则都没命中夹具）
  1 = **有命中**（逐条点名：规则 / 件 / 行号 / 片段 —— 这一档就是本件要报的红）
  2 = **不给结论**：规则表缺件或不可解析 / 表里**一条真规则都没有**（空转）/ 白名单缺件 /
      扫描面**一件 `*_test.go` 都没扫到**（空转）/ `--self-test` 不过 —— **缺件与空转一律不当绿**

机器面字段（`--json <字段,逗号分隔>` · 缺口 `Q-143`）
----------------------------------------------------
**只加字段、不动判据与退码** ✗（设计 `设计-CLI机器读面-v1.0-20260924.md` §3.2 坑 8 + §4.2 第 22 行）：
老四格语义**一字不改** —— `rules` / `fixtures` / `hits` / `files`（`files` **仍是去重集**，命中件去重后排序）。
新增三格**按名请求才出现**（老调用方一个字节不受影响）：

  · `files_dedup` —— `files` 的**同值别名**（把「去重集 ↔ 去重和」这一对摆到同一命名面上）；
  · `files_sum`   —— **未去重和** = 逐条命中规则各自命中件数**相加**（「`64 + 7 = 71`」这类
                     **逐条相加**读法的机器面落点 —— 今天只能人眼抄再手算 ✗）；
  · `per_rule`    —— 逐规则一条 `{rule, files, hits, lines}`：`files` = 该规则命中的件（排序去重）、
                     `hits` = 该规则的命中**行数**、`lines` = 命中处**逐条 `{file, line, text}`**（`{件,行}` 真源）。

用法
----
    python3 scripts/gates/check-replace-rule-fixtures.py                       # 判真目标（仓根）
    python3 scripts/gates/check-replace-rule-fixtures.py --root <树>            # 判别的树（自检夹具同入口）
    python3 scripts/gates/check-replace-rule-fixtures.py --extra-rule '<pat>'   # 合成一条规则（负控用）
    python3 scripts/gates/check-replace-rule-fixtures.py --max-examples 5       # 每条规则最多列几处
    python3 scripts/gates/check-replace-rule-fixtures.py --json rules,hits,files
    python3 scripts/gates/check-replace-rule-fixtures.py --json files_sum,per_rule  # 去重和 + 逐规则 {件,行}
    python3 scripts/gates/check-replace-rule-fixtures.py --list-rules
    python3 scripts/gates/check-replace-rule-fixtures.py --self-test            # 成对负控（合成夹具 · 不碰真目标）

★ 本件是**私有面件**（读 `publish/` 的规则表与白名单；公开树里这两件都不在）⇒ 不随公开面发布，
  登记在 `scripts/公开标记.tsv` 的 `public=no` 面。**未接闸**：挂 `scripts/gates/precommit-gates.sh`
  的动作按**单写入者纪律**留给收口者（本波只产件 · 不碰那件 ✗）。
"""
import argparse
import fnmatch
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))          # scripts/gates → 仓根

RULES_REL = os.path.join("publish", "replace-rules.tsv")
WHITELIST_REL = os.path.join("publish", "whitelist.txt")

# 机器面字段表（`--json` 认的名字）—— **唯一真源**：`json_payload` 装包封与
# 「不认的名 ⇒ 用法错 `rc=2` ＋ 印本表」两处**同读这一处**（缺口 `Q-150`）。
JSON_FIELDS = ["rules", "hits", "files", "fixtures", "files_dedup", "files_sum", "per_rule"]

# 夹具隔离（§五 第三批）：`!夹具<TAB><路径 glob>` —— 命中 glob 的件**整段跳过替换规则**，
# 因此它们**不进本件的件面**（规则不碰它们 ⇒ 拿规则去判它们就是自造红）。
# 这不是豁免表：豁免表是「把红写进白名单、红还在」；隔离是「规则不再作用于该件」，
# 有票可查（规则表里逐条登记）+ 三处消费者同口径（镜像器 / 后备压平器 / 本件）。
ISO_PREFIX = "!夹具"


def is_isolated(rel, isolated):
    """rel 是否命中夹具隔离 glob（fnmatch，`*` 跨 `/`）。"""
    return bool(rel) and any(fnmatch.fnmatch(rel, pat) for pat in isolated)


def isolation_audit(isolated, iso_files, scanned):
    """夹具隔离条目体检 —— 只加**可见性**与**防自伤**，不放宽任何判据。

    返回 (逐条 [(glob, 对上的件数)], 空转 glob 列表, 是否把整个件面吃光)。

    ★ **看得见**：一条 `!夹具` glob 一件都对不上 ⇒ 它登记了却没生效（原口径下**一声不响**：
      隔离条目的存在/失效在输出里没有任何落点，只能靠人翻规则表）。
    ★ **不许吃光**：隔离若把件面整个清空（`scanned` 件全被排除）⇒ 判据变成空转，
      而那正是本件要防的病（「把红写进白名单」）。此支为**收紧**：原口径下它会被当成绿。
    注：逐条记件数是**信息格**，只有「吃光」一支改判；「少吃」一律照判（隔离本来就是为了少判）。
    """
    per = []
    for pat in isolated:
        n = sum(1 for rel in iso_files if fnmatch.fnmatch(rel, pat))
        per.append((pat, n))
    dead = [p for p, n in per if n == 0]
    eaten = bool(isolated) and scanned > 0 and len(iso_files) >= scanned
    return per, dead, eaten

# 通用排除表（与 §十七/§二十一 各处复跑命令同一口径）
EXCLUDE_DIRS = {"target", "node_modules", "dist", "bin", "vendor", "data", ".git",
                ".venv", "venv", ".build", "_history"}

# 夹具判据（**写死一处**）：会进公开面的 Go 测试件。
FIXTURE_SUFFIX = "_test.go"


# ── 规则表 ────────────────────────────────────────────────────────────────
def parse_rules(path, extra=()):
    """读 `pattern<TAB>replacement<TAB>说明` → [(原样行, 是否正则, pattern, replacement, 说明)]，
    外加 `!夹具<TAB><路径 glob>` 收成的**夹具隔离表**。

    解析口径与两器一致：`#` 注释与空行跳过；`regex:` 前缀 = 正则（`sed -E` / `re` 语义）；
    `!夹具` = 夹具隔离（**不是规则**：它不进判据，进的是件面排除表）。
    返回 (rules, isolated, error)：error 非空 ⇒ 调用方按「不给结论」处理。
    """
    if not os.path.isfile(path):
        return [], [], "规则表缺件：%s（替换规则的单一真源不在场 ⇒ 判据不可判）" % RULES_REL
    rules, isolated = [], []
    with open(path, encoding="utf-8") as f:
        for i, line in enumerate(f.read().split("\n"), 1):
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                return [], [], "规则表第 %d 行不是 `pattern<TAB>replacement...` 形态：%r" % (i, line[:80])
            pat, rep = parts[0], parts[1]
            if pat == ISO_PREFIX:
                if not rep.strip():
                    return [], [], "规则表第 %d 行是空的夹具隔离（`!夹具` 后面没有 glob ⇒ 判据不可判）：%r" % (i, line[:80])
                isolated.append(rep.strip())
                continue
            note = parts[2] if len(parts) > 2 else ""
            is_re = pat.startswith("regex:")
            rules.append((line, is_re, pat[len("regex:"):] if is_re else pat, rep, note))
    for p in extra:                                          # 合成规则（负控用）：只给 pattern，replacement 无关紧要
        is_re = p.startswith("regex:")
        rules.append(("（合成规则 · 负控用）" + p, is_re, p[len("regex:"):] if is_re else p, "<合成>", "命令行注入"))
    return rules, isolated, ""


def rule_matches(rule, line):
    """一条规则是否命中一行（与两器同口径：字面量 / 正则）。正则写坏 ⇒ 抛 re.error 给调用方按「不给结论」处理。"""
    _raw, is_re, pat, _rep, _note = rule
    if is_re:
        return re.search(pat, line) is not None
    return pat in line


# ── 扫描面 ────────────────────────────────────────────────────────────────
def whitelist_prefixes(path):
    """白名单里**会进公开面的**路径前缀（目录以 `/` 结尾 = 递归；否则当精确件，对 `*_test.go` 面无用 ⇒ 忽略）。"""
    if not os.path.isfile(path):
        return None
    out = []
    with open(path, encoding="utf-8") as f:
        for line in f.read().split("\n"):
            s = line.strip()
            if not s or s.startswith("#") or s.endswith("__EXCLUDES_ABOVE__"):
                continue
            # 白名单的「明确排除」注释块里的行也是注释（以 # 开头）⇒ 上面已过滤
            if s.endswith("/"):
                out.append(s)
    return out


def collect_fixtures(root, prefixes, isolated=()):
    """扫「会进公开面的测试夹具」→ ([(相对路径, 行号, 行文本)], [被夹具隔离排除的件], 未隔离前的夹具件数)。

    `isolated` 里的 glob 命中的件**不进件面**（它们被规则表显式隔离 ⇒ 规则不碰它们 ⇒ 不判），
    但**逐件登记**在第二个返回值里 —— 隔离只许看得见，不许静默少判。
    """
    hits, iso_files, scanned = [], [], 0
    for pre in prefixes:
        base = os.path.join(root, pre.rstrip("/"))
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
            for fn in filenames:
                if not fn.endswith(FIXTURE_SUFFIX):
                    continue
                full = os.path.join(dirpath, fn)
                rel = os.path.relpath(full, root)
                scanned += 1
                if is_isolated(rel, isolated):
                    iso_files.append(rel)
                    continue
                try:
                    with open(full, encoding="utf-8", errors="replace") as f:
                        for i, line in enumerate(f, 1):
                            hits.append((rel, i, line.rstrip("\n")))
                except OSError:
                    continue
    return hits, sorted(iso_files), scanned


# ── 判决（唯一出口 · 自检直接钉它）────────────────────────────────────────
def judge(rules, fixtures, max_examples=5):
    """返回 (命中表, 报错)：命中表 = [(规则原文, 命中件, 例子, 命中行数, 逐件明细)]。

    ★ 判据只有一句：**规则的命中件里出现 `*_test.go` ⇒ 红**。没有豁免、没有例外
      （豁免表 = 把红写进白名单，正是本件要防的病）。
    ★ 判据一字未改（缺口 `Q-143` 只加字段 ✗）：这里多带出去的第 5 格是**同一遍扫描的原始明细**
      （`{件: [(行号, 片段), …]}`），只给人读/机器面用 —— 它不参与判，也不改任何一格的语义。
    """
    hits, err = [], ""
    for rule in rules:
        if rule[1] and rule[2] == "":
            return [], "规则表里有一条**空正则**（%r）⇒ 判据不可判" % rule[0][:60]
        per_file = {}
        for rel, lineno, line in fixtures:
            if not rel.endswith(FIXTURE_SUFFIX):
                continue
            try:
                if rule_matches(rule, line):
                    per_file.setdefault(rel, []).append((lineno, line.strip()[:160]))
            except re.error as e:
                return [], "规则表里有一条**正则写坏了**（%r）：%s" % (rule[0][:60], e)
        if per_file:
            total = sum(len(v) for v in per_file.values())
            ex = []
            for rel in sorted(per_file)[:max_examples]:
                ex.extend((rel, ln, txt) for ln, txt in per_file[rel][:2])
            hits.append((rule[0], sorted(per_file), ex, total, per_file))
    return hits, err


def json_payload(want, n_rules, n_fixtures, hits):
    """按 `--json` 请求的字段名装机器面 ⇒ dict（缺口 `Q-143`：**只加字段、不动判据与退码** ✗）。

    老四格（`rules` / `fixtures` / `hits` / `files`）逐格语义**一字不改** —— 尤其 `files`
    仍是**去重集**（命中件去重后排序），不改名、不改值。
    新三格按名请求才出现（`files_dedup` = `files` 同值别名 · `files_sum` = **未去重和** ·
    `per_rule[]` = 逐规则 `{rule, files, hits, lines}`，`lines` = 逐条 `{file, line, text}`）。
    """
    dedup = sorted(set(f for _r, fl, _e, _t, _d in hits for f in fl))
    obj = {}
    if "rules" in want:
        obj["rules"] = n_rules
    if "fixtures" in want:
        obj["fixtures"] = n_fixtures
    if "hits" in want:
        obj["hits"] = len(hits)
    if "files" in want:
        obj["files"] = dedup
    if "files_dedup" in want:
        obj["files_dedup"] = dedup
    if "files_sum" in want:
        obj["files_sum"] = sum(len(fl) for _r, fl, _e, _t, _d in hits)
    if "per_rule" in want:
        obj["per_rule"] = [{"rule": raw.strip(), "files": fl, "hits": tot,
                            "lines": [{"file": rel, "line": ln, "text": txt}
                                      for rel in sorted(detail) for ln, txt in detail[rel]]}
                           for raw, fl, _ex, tot, detail in hits]
    return obj


# ── 自检（成对负控 · 合成夹具 · 不碰真目标）─────────────────────────────────
# ★ 自检夹具**一律用合成字面量**（`/srv/demo` · `198.51.100.7`（RFC 5737 文档段）· `userdemo`）——
#   本件不放私有面真实值（家目录 / 用户名 / 内网地址），机理与真实规则**逐条同形**即可。
SELF_CASES = [
    # (名, 规则, 夹具文本, 期望)
    ("负控 MF1 合成规则命中 `core/**/*_test.go`（**必红**）",
     "(合成)regex:/srv/demo", "core/internal/demo/fixture_test.go", 'home := "/srv/demo/x.key"', "hit"),
    ("负控 MF2 内网地址规则命中夹具（真实规则 `→ <worker-ip>` 的同形合成）",
     "(合成)regex:198\\.51\\.100\\.7", "core/internal/demo/obs_test.go", '"host":"198.51.100.7"', "hit"),
    ("负控 MF3 多字节替换命中夹具长度假设（真实规则 `→ Mr2109` 多 2 字节的同形合成）",
     "(合成)userdemo", "core/internal/demo/limits_test.go", 'secret := "peer userdemo run here ok"', "hit"),
    ("正控 PC1 同一条规则只命中**非夹具**件 ⇒ 绿（判据不许打成「见到那个词就红」）",
     "(合成)userdemo", "core/internal/demo/impl.go", 'home := "/srv/demo/" + user', "green"),
    ("正控 PC2 规则与件完全不沾 ⇒ 绿",
     "(合成)绝不出现的串-zz", "core/internal/demo/fixture_test.go", 'want := 1', "green"),
    ("空转 VAC1 扫描面**一件夹具都没有** ⇒ 不给结论（不许当绿）",
     "(合成)userdemo", "", "", "vacuous"),
]


def self_test():
    ok, bad = 0, []
    for name, rule_pat, rel, line, want in SELF_CASES:
        is_re = rule_pat.startswith("(合成)regex:")
        pat = rule_pat.split("regex:", 1)[1] if "regex:" in rule_pat else rule_pat.replace("(合成)", "")
        rules = [(rule_pat, is_re, pat, "<合成>", "自检")]
        fixtures = [(rel, 1, line)] if rel else []
        if not fixtures:
            got = "vacuous"
        else:
            hits, err = judge(rules, fixtures)
            if err:
                got = "error"
            else:
                got = "hit" if hits else "green"
        if got == want:
            ok += 1
            print("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            print("   ✗ %s ⇒ 期望 %s，实得 %s" % (name, want, got))
    # 区分度：同一段文本、同一条规则 —— 在夹具件里必须命中、在非夹具件里必须不命中
    r = ("(合成)userdemo", False, "userdemo", "<合成>", "自检")
    in_fix, _ = judge([r], [("core/x/a_test.go", 1, "s := \"userdemo\"")])
    off_fix, _ = judge([r], [("core/x/a.go", 1, "s := \"userdemo\"")])
    if in_fix and not off_fix:
        ok += 1
        print("   ✓ 区分度：同规则在 `*_test.go` ⇒ 命中、在非测试件 ⇒ 不命中（判据有牙）")
    else:
        bad.append("区分度")
        print("   ✗ 区分度：夹具里 %d 处命中、非夹具里 %d 处命中（判据没牙）"
              % (len(in_fix), len(off_fix)))
    # 夹具隔离（§五 第三批）：成对负控 —— 同一件、同一条规则：不隔离 ⇒ 必红；隔离 ⇒ 必绿。
    tmpd = tempfile.mkdtemp(prefix="a4-iso-")
    try:
        rp = os.path.join(tmpd, "rules.tsv")
        with open(rp, "w", encoding="utf-8") as f:
            f.write("regex:/srv/demo\t<home>\t合成规则\n")
            f.write("!夹具\tcore/internal/demo/*_test.go\t合成隔离\n")
        rules2, iso2, err2 = parse_rules(rp)
        fx = [("core/internal/demo/fixture_test.go", 1, 'home := "/srv/demo/x.key"')]
        kept = [(r, ln, t) for r, ln, t in fx if not is_isolated(r, iso2)]
        hits_on, _ = judge(rules2, fx)          # 不隔离 ⇒ 规则命中夹具（必红）
        hits_off, _ = judge(rules2, kept)       # 隔离 ⇒ 件面里没有它（必绿）
        if not err2 and iso2 and hits_on and not hits_off:
            ok += 1
            print("   ✓ 夹具隔离：同一件同一规则 —— 不隔离 ⇒ 命中 %d 条、隔离 ⇒ 0 条（隔离只许少判、且不许静默）"
                  % len(hits_on))
        else:
            bad.append("夹具隔离")
            print("   ✗ 夹具隔离：err=%r iso=%r 不隔离命中 %d / 隔离命中 %d"
                  % (err2, iso2, len(hits_on), len(hits_off)))
    finally:
        shutil.rmtree(tmpd, ignore_errors=True)
    # 机器面「**只加字段**」成对负控（`Q-143`）：老四格一字不改 · 新三格按名请求才出现。
    #   造两条规则**共命中同一批件**（去重集 2 ≠ 未去重和 3）⇒ 两个数必须分得开。
    import json as _json
    rules_m = [("regex:userdemo", True, "userdemo", "<合成>", "自检"),
               ("regex:/srv/demo", True, "/srv/demo", "<合成>", "自检")]
    fx_m = [("core/x/a_test.go", 3, 's := "userdemo"'),
            ("core/x/b_test.go", 1, 'u := "userdemo"'),
            ("core/x/a_test.go", 9, 'h := "/srv/demo"')]
    hits_m, err_m = judge(rules_m, fx_m)
    old = json_payload(["rules", "fixtures", "hits", "files"], len(rules_m), len(fx_m), hits_m)
    if not err_m and sorted(old) == ["files", "fixtures", "hits", "rules"]:
        ok += 1
        print("   ✓ 只加字段：老四格请求出来的键集恰 = {files, fixtures, hits, rules}（一字未改）")
    else:
        bad.append("老四格键集")
        print("   ✗ 老四格键集：err=%r keys=%r" % (err_m, sorted(old)))
    if old.get("files") == ["core/x/a_test.go", "core/x/b_test.go"] and len(old.get("files") or []) == 2:
        ok += 1
        print("   ✓ `files` 仍是**去重集**（2 个件 · 排序）—— 语义一字不改")
    else:
        bad.append("files 去重集")
        print("   ✗ `files` 去重集：%r" % (old.get("files"),))
    new = json_payload(["files_dedup", "files_sum", "per_rule"], len(rules_m), len(fx_m), hits_m)
    if sorted(new) == ["files_dedup", "files_sum", "per_rule"] and new["files_sum"] == 3 \
            and new["files_dedup"] == old["files"]:
        ok += 1
        print("   ✓ 新增三格按名请求才出现：`files_dedup` = 去重集（2）· `files_sum` = **未去重和 3**"
              "（2 + 1 · 「逐条相加」从此机器面读）")
    else:
        bad.append("files_sum")
        print("   ✗ 新三格：keys=%r files_sum=%r dedup=%r" % (sorted(new), new.get("files_sum"),
                                                             new.get("files_dedup")))
    pr = new.get("per_rule") or []
    if len(pr) == 2 and pr[0].get("hits") == 2 and pr[1].get("hits") == 1 \
            and pr[0].get("lines") == [{"file": "core/x/a_test.go", "line": 3, "text": 's := "userdemo"'},
                                       {"file": "core/x/b_test.go", "line": 1, "text": 'u := "userdemo"'}]:
        ok += 1
        print("   ✓ `per_rule[]` 逐规则给出 `{rule, files, hits, lines}` · `lines` = 逐条 `{件,行,原文}`"
              "（末例 `core/x/b_test.go:1`）")
    else:
        bad.append("per_rule")
        print("   ✗ per_rule：%s" % _json.dumps(pr, ensure_ascii=False)[:200])
    # 夹具隔离的**可见性 / 防自伤**成对负控（本波）：逐条 glob 记件数 + 吃光整面必须拒判。
    per_i, dead_i, _e0 = isolation_audit(["core/internal/demo/*_test.go", "nope/*_test.go"],
                                         ["core/internal/demo/fixture_test.go"], 3)
    if per_i == [("core/internal/demo/*_test.go", 1), ("nope/*_test.go", 0)] \
            and dead_i == ["nope/*_test.go"] and not _e0:
        ok += 1
        print("   ✓ 隔离体检：逐条 glob 记件数 · 一件都没对上的条目**点名**（登记了却没生效 ⇒ 看得见）")
    else:
        bad.append("隔离体检")
        print("   ✗ 隔离体检：per=%r dead=%r eaten=%r" % (per_i, dead_i, _e0))
    _p, _d, eaten_all = isolation_audit(["*_test.go"], ["a_test.go", "b_test.go"], 2)
    _p2, _d2, eaten_half = isolation_audit(["*_test.go"], ["a_test.go"], 2)
    if eaten_all and not eaten_half:
        ok += 1
        print("   ✓ 隔离不许吃光：件面全被隔离 ⇒ 拒判（吃光=%r）· 只隔离一半 ⇒ 照判（吃光=%r）"
              % (eaten_all, eaten_half))
    else:
        bad.append("隔离吃光")
        print("   ✗ 隔离吃光：全吃=%r 半吃=%r" % (eaten_all, eaten_half))
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


# ── 主流程 ────────────────────────────────────────────────────────────────
def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=REPO_ROOT, help="判哪棵树（默认仓根）")
    ap.add_argument("--rules", default="", help="规则表路径（默认 <root>/publish/replace-rules.tsv）")
    ap.add_argument("--whitelist", default="", help="白名单路径（默认 <root>/publish/whitelist.txt）")
    ap.add_argument("--extra-rule", action="append", default=[], help="合成一条规则（可重复 · **负控**用）")
    ap.add_argument("--max-examples", type=int, default=5, help="每条规则最多列几件（默认 5）")
    ap.add_argument("--json", default="", help="机器面字段（逗号分隔：%s；**不认的名 ⇒ 用法错 rc=2**）" % ",".join(JSON_FIELDS))
    ap.add_argument("--list-rules", action="store_true", help="打判据 / 扫描面 / 档位")
    ap.add_argument("--self-test", action="store_true", help="成对负控（合成夹具 · 不碰真目标）")
    args = ap.parse_args()

    # ★ 缺口 `Q-150`：`--json` 的**字段名打错**不许静默吐 `{}`（与「零命中」同形 ⇒ 探针
    #   打错一个字就拿到空包封、且不报错）。口径 = **用法错 `rc=2` ＋ 逐条印可选字段表** ——
    #   **只把「不认的名」这一支从静默改成拒绝**，不动任何业务判据与退码（合法字段照旧走命中面）。
    want = []
    if args.json:
        want = [f.strip() for f in args.json.split(",") if f.strip()]
        bad = [f for f in want if f not in JSON_FIELDS]
        if bad:
            print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad))
            print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS))
            print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
            return 2

    if args.list_rules:
        print("⑮ 替换规则级机检：命中件不许是测试夹具（§十三 M8 · §八 待拍 3 的落点）")
        print("真源：Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md §七 + §十三 M8")
        print("  规则面：%s（`pattern<TAB>replacement<TAB>说明` · `regex:` 走正则）" % RULES_REL)
        print("  夹具隔离：同一件里的 `!夹具<TAB><路径 glob>` ⇒ 该件整段不入件面（§五 第三批；它不是豁免，是规则不再作用于该件）")
        print("  件面  ：%s 里**会进公开面**的路径前缀下的 `*_test.go`" % WHITELIST_REL)
        print("  判据  ：任一条规则命中 `*_test.go` ⇒ 红（逐条点名规则/件/行号/片段 · **不给豁免表**）")
        print("退码：0 全绿 · 1 有命中 · 2 不给结论（规则表/白名单缺件 · 空转 · 自检不过 · **`--json` 字段名不认**）")
        print("机器面：`--json <字段,逗号分隔>` —— 老四格 `rules,fixtures,hits,files`（`files` = **去重集**）"
              "+ 新三格 `files_dedup`（同值别名）/ `files_sum`（**未去重和**）/ `per_rule[]`（逐规则"
              "`{rule, files, hits, lines}` · `lines` = 逐条 `{件,行,原文}`）—— 只加字段，判据与退码不动；"
              "**不认的字段名 ⇒ 用法错 `rc=2` ＋ 印字段表**（缺口 `Q-150`：打错的名与零命中必须分得开）")
        return 0

    if args.self_test:
        print("── 替换规则级机检 · 自检（成对负控 · 合成夹具）──")
        return self_test()

    root = os.path.abspath(args.root)
    rules_path = args.rules or os.path.join(root, RULES_REL)
    wl_path = args.whitelist or os.path.join(root, WHITELIST_REL)

    rules, isolated, err = parse_rules(rules_path, args.extra_rule)
    if err:
        print("⛔ %s" % err)
        print("⇒ 不给结论（rc=2）：缺件/不可解析 ≠ 绿")
        return 2
    if not rules:
        print("⛔ 空转：规则表里**一条真规则都没有**（%s）—— 没有规则就没有可判的东西" % rules_path)
        print("⇒ 不给结论（rc=2）")
        return 2

    prefixes = whitelist_prefixes(wl_path)
    if prefixes is None:
        print("⛔ 白名单缺件：%s（扫描面定义不出来 ⇒ 判据不可判）" % WHITELIST_REL)
        print("⇒ 不给结论（rc=2）")
        return 2
    if not prefixes:
        print("⛔ 空转：白名单里**一条递归前缀都没有**（%s）" % wl_path)
        print("⇒ 不给结论（rc=2）")
        return 2

    fixtures, iso_files, scanned = collect_fixtures(root, prefixes, isolated)
    print("── 替换规则级机检：命中件不许是测试夹具（§十三 M8）──")
    print("   树：%s" % root)
    print("   规则面：%s ⇒ **%d 条**规则（%d 条正则 · %d 条字面量%s）"
          % (os.path.relpath(rules_path, root) if rules_path.startswith(root) else rules_path,
             len(rules), sum(1 for r in rules if r[1]), sum(1 for r in rules if not r[1]),
             " · 含命令行合成规则 %d 条" % len(args.extra_rule) if args.extra_rule else ""))
    if isolated:
        print("   夹具隔离：%d 条 glob（%s）⇒ 排除 **%d 个** `*_test.go` 出件面"
              % (len(isolated), " · ".join(isolated), len(iso_files)))
        for rel in iso_files:
            print("      ── %s" % rel)
    else:
        print("   夹具隔离：**0 条** glob —— 规则表里没有生效的 `!夹具` 条目"
              " ⇒ 隔离机制在场但**空转**，件面 %d 个夹具**全部在判据内**" % scanned)
    per_iso, dead_iso, eaten_iso = isolation_audit(isolated, iso_files, scanned)
    for _pat, _n in per_iso:
        print("      · 隔离条目 `!夹具 %s` ⇒ 对上 %d 件%s"
              % (_pat, _n, "  ⚠ 一件都没对上（登记了却没生效）" if _n == 0 else ""))
    if eaten_iso:
        print("⛔ 夹具隔离把整个件面吃光（%d/%d 件全被隔离）⇒ 判据空转" % (len(iso_files), scanned))
        print("⇒ 不给结论（rc=2）：**隔离吃光 ≠ 绿** —— 那正是本件要防的「把红写进白名单」")
        return 2
    print("   件面：白名单 %d 条前缀下的测试夹具 ⇒ **%d 个** `*_test.go`（%d 行）"
          % (len(prefixes), len(set(r for r, _ in [(f[0], 0) for f in fixtures])), len(fixtures)))
    if not fixtures:
        print("⛔ 空转：扫描面一件 `*_test.go` 都没扫到（扫描面写错 or 树不对）")
        print("⇒ 不给结论（rc=2）：**空转 ≠ 绿**")
        return 2

    hits, err = judge(rules, fixtures, args.max_examples)
    if err:
        print("⛔ %s" % err)
        print("⇒ 不给结论（rc=2）")
        return 2

    if want:
        print(json.dumps(json_payload(want, len(rules), len(fixtures), hits), ensure_ascii=False))

    if not hits:
        print("⇒ 全绿（rc=0）：%d 条规则一条都没命中测试夹具" % len(rules))
        return 0

    print("   ✗ 命中：**%d 条**规则打到了测试夹具（共 %d 个件）"
          % (len(hits), len(set(f for _r, fl, _e, _t, _d in hits for f in fl))))
    for raw, files, ex, total, _detail in hits:
        print("   ── 规则：%s" % raw.strip()[:140])
        print("      命中件 %d 个 · 命中行 %d 处（列前 %d 件）" % (len(files), total, len(set(e[0] for e in ex))))
        for rel, ln, txt in ex:
            print("      %s:%d: %s" % (rel, ln, txt))
    print("⇒ 有命中（rc=1）：**替换规则命中件不许是测试夹具** —— 规则改的是公开面夹具的"
          "字面量/长度假设，本机看不出来、公开树一 clone 就红")
    return 1


if __name__ == "__main__":
    sys.exit(main())

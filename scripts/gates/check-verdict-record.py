#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-verdict-record.py —— 「算不算过」的机器读落点 + 稳定指纹（`W-47` · `Q-010`/`Q-076`）

真源与口径（**判据绑私有面** · 逐字引文全指向仓外 `Zerg-内部文档/`）
------------------------------------------------------------
· 施工条（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md`
  §一 序 129 —— 判据栏 = 「判据件落**仓内**（非 `/tmp`）；指纹由内容算出
  （**负控：只改行号 ⇒ 指纹不变；改内容 ⇒ 指纹变**）」
· 来源件（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/承接自-v2.5.11/调研-缺口-门禁证据-20260923.md`
  `R-12`（`Q-010` · `Q-076`）：病 = 「自写判据件落 `/tmp`，跑完就散在临时面」；
  学 = 「『算不算过』落成 JSON 判据件（不许散在 `/tmp`）」+「带**稳定指纹**（内容算出，不是行号）」；
  判据 = 「判据件可回读：同一改动两跑，**指纹相同**；指纹不同 ⇒ 明说『**仓变了，结果不可比**』」
· 同族（仓外 · 逐字）：同件 `R-17`（`Q-014`）「**行号锚点本身就是病**」·
  `R-16`（`Q-026`）「凡『在 / 不在』的答案都要带**口径** + 树 sha」。

本件补的那一格
--------------
「算不算过」今天有两个病根（都在上述源件里）：① 结论只活在**临时面**（`/tmp/...`，跑完即散）；
② 结论的身份靠**位置/行号**（件一改就不可比，更坏的是：拿两次**不同内容**的结论当同一件事比）。
本门把这两条各落成一条**可现跑**的判据：

  ① **落点面**：判据件自己的结论**落点**不许在 `/tmp` 上（扫判据面 `scripts/gates/**` 的
     `check-*.py|sh` + `mutate-scan`，命中 ⇒ `rc=1` + 逐条点名「件:行 + 原样」）。
     ★ **自检夹具登记**（在册 `GAP-20260928-96` · 同门㉝ `check-public-face-wording.py:236-301`
     的 `WHITELIST` 同形）：判据件的**自检夹具**里必然把「写 /tmp」的形态写成**合成标本串**
     （`NEG_CASES`/`POS_CASES` 用例正文 —— 跑起来一个字不写 `/tmp`）⇒ 那不是病、是**夹具数据**；
     这类命中走 `FIXTURE_EXEMPT` **逐条登记 + 逐条打印理由**、不计入命中数（**不是静默过滤** ✗）。
     登记只盖**夹具字面量形态**那一行；**命令位调用 / `open(…,"w")` 写盘**在**同一件里照旧红**
     （⇒ 不放宽真目标判据）。条目命中 0 处 = **悬空登记**，照实打印、**不判红**（同门㉝ 口径）；
     表长 > `FIXTURE_EXEMPT_MAX` ⇒ `rc=2`（防把「往登记表里加条目」当治本 ✗）。
     本门的结论落点 = **状态目录** `<state>/verdict_records.jsonl`（`--record` 显式落一条 · 非 `/tmp`）。
  ② **指纹面**：指纹由**内容**算出 —— subject 归一成「**条**」：
     `add_step` 条 = `scope|步名|档位|工作目录|命令串`（**去缩进 / 去排版**）；
     基线件 = **规范化 JSON**（`sort_keys` + 紧凑分隔符）的 sha256 + 条数。
     ⇒ 只改行号 / 挪行 / 加空行 / 重排版 ⇒ **指纹不变**；改一个字节内容 / 增删条 ⇒ **指纹变**。

「同一改动两跑指纹相同 / 不同 ⇒ 明说『仓变了，结果不可比』」的落点 = `--compare`
（比**同一 subject 的最后两条**记录）：相同 ⇒ `rc=0`；不同 ⇒ `rc=2` **且逐字打印那一句**。

★ 默认档（门跑的就是它）**只判** ① + ②（本跑两遍现算须逐字相同），并把「与最近一条记录的关系」
  **打印出来**，但**那一格不改红绿** —— 否则「按任务单给门表加一条 `add_step`」这件事本身
  就会把本步判红（`Q-036`/`W-45` 的先例：判据面一变就恒红 = **自残**，不是门）。
  要判可比性 ⇒ 显式 `--compare`（判据面一变就**明说不可比**，绝不悄悄比）。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 落点面 0 命中 + 指纹面两遍现算逐字相同（+ 记录位有 ⇒ 照实打印可比性读法）
  1 = **落点面命中**（判据件的结论落点在 `/tmp` 上 · 逐条点名）
  2 = **不给结论**：门表 / 基线件不在盘 · `add_step` 解析到 0 条（空转）· 两遍现算**不同** ·
      自检不过 · 登记表超硬钳（`FIXTURE_EXEMPT` > `FIXTURE_EXEMPT_MAX`） · `--compare` 时**不足两跑**或
      **指纹不同**（后者逐字打印「仓变了，结果不可比」）

用法
----
    python3 scripts/gates/check-verdict-record.py                  # 判（落点面 + 指纹面 · 只读）
    python3 scripts/gates/check-verdict-record.py --json           # 机读（zerg/v1 形状）
    python3 scripts/gates/check-verdict-record.py --fingerprint    # 只打指纹（人面一行）
    python3 scripts/gates/check-verdict-record.py --record         # 把本跑结论落成一条记录（写状态目录）
    python3 scripts/gates/check-verdict-record.py --compare        # 比同一 subject 的最后两条记录
    python3 scripts/gates/check-verdict-record.py --state-dir DIR  # 换状态目录（缺省 `~/.zerg/state`）
    python3 scripts/gates/check-verdict-record.py --list-rules
    python3 scripts/gates/check-verdict-record.py --self-test      # 合成夹具（不碰真仓 / 真状态目录）

★ 本笔**已挂门**（`gates` scope · `tri` 阻断 · 在默认集 · **不进 `FAST_STEPS`** —— 它要读
  **门表自身**与两件命令面基线件；只读、无副作用）。
"""

import argparse
import contextlib
import fnmatch
import hashlib
import io
import json
import os
import re
import shutil
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-verdict-record.py v1.1.0"

SUBJECT = "gate-face"                      # subject 名（记录按它分组）
GATE_TABLE_REL = "scripts/gates/precommit-gates.sh"
BASELINE_RELS = ("scripts/gates/cli-contract-baseline.json",
                 "core/cmd/zerg/testdata/cli-matrix.json")
RECORD_NAME = "verdict_records.jsonl"      # 结论件落点（**状态目录**，不是 /tmp）
JUDGE_DIR_REL = "scripts/gates"

# `add_step <scope> "<步名>" <档位> "<工作目录>" "<命令串>"` —— 五格都要（**不含行号**）。
ADD_STEP_RE = re.compile(
    r'^\s*add_step\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+"((?:[^"\\]|\\.)*)"\s*$',
    re.M)

# ── 落点面：判据件把结论写到 `/tmp` 的**写盘**形态（不是「提到 /tmp」）─────────────────
#   ① shell 重定向：`> /tmp/x` / `>>/tmp/x`（`<string>` 这类样本因前一位是字母被排除）
#   ② python 打开：`open("/tmp/…", "w")` / `open('/tmp/…','a')`
LANDING_PATTERNS = (
    ("shell-redirect", re.compile(r"(?<![A-Za-z<=])>{1,2}\s*/tmp/")),
    ("py-open-write", re.compile(r"""open\(\s*["']/tmp/[^"']*["']\s*,\s*["'][wa]""")),
)

# ── 自检夹具登记（在册 `GAP-20260928-96` · **同族先例同形**）──────────────────────────
#   先例（逐字）：门㉝ `scripts/gates/check-public-face-wording.py:236-301` 的 `WHITELIST`
#   —— 条目 = (规则名, 件 glob（tuple · fnmatch · `*` 跨 `/`）, 行正则, 为什么豁免)；
#   本件照它的**登记式白名单**做，只把第一格（规则名）换成**件 glob 单格**（本面只有一条规则族）。
#   逐命中**逐条打印理由** · 悬空条目（命中 0 处）**照实打印、不判红**（同门㉝ 那一格口径）。
#   ★ 为什么有它：判据件自己的**自检夹具**里必然把「写 /tmp」的形态写成**合成标本串**
#     （`NEG_CASES`/`POS_CASES` 的用例正文 —— 跑起来一个字不写 `/tmp`）⇒ 那不是病，是**夹具数据**；
#     不登记 ⇒ 门把「标本串」当真病（假红）。
#   ★ 与「放宽真判据」的界线（**逐条可核**）：条目只盖**夹具字面量形态**那一行
#     （行首缩进 + 单引号起 + `> /tmp/` + 收尾 `',` 后跟档位串）；**命令位调用**
#     （`subprocess.run("… > /tmp/x")`）与 `open(…,"w")` 写盘在**同一件里照旧红** —— 见 `--self-test` ⑬b/⑬c。
#   ★ `件 glob` 用**精确件名**（不是路径前缀 · 同门52 `EXEMPT_QS` 口径「精确文件名 + 形状匹配」）。
#   ★ 表长度硬钳（同 `EXEMPT_QS` 的「多于 1 条 ⇒ rc=2」）：条目数 > `FIXTURE_EXEMPT_MAX`
#     ⇒ rc=2 **不给结论**（防把「往登记表里加条目」当治本 ✗）。
FIXTURE_EXEMPT_MAX = 4
FIXTURE_EXEMPT = (
    # `check-token-argv.py` 自检 `NEG_CASES` ③b（heredoc 在卵里生成单元件）里的落点标本串
    (("scripts/gates/check-token-argv.py",),
     re.compile(r"^\s*'[^']*> /tmp/[^']*',\s*\"[A-Z][A-Z0-9-]*\"\)"),
     "自检夹具（合成标本串）：`NEG_CASES` 用例正文里的标本路径 —— 跑起来一个字不写 /tmp"
     "⇒ 判「夹具数据」，不是「结论落点」；同一件里若改成命令位调用写 /tmp ⇒ 照旧红"),
    # `check-wired-scripts.py` 自检 ⑮ 正控（接线面夹具）里的重定向标本串
    (("scripts/gates/check-wired-scripts.py",),
     re.compile(r"^\s*'[^']*check-[a-z][a-z-]*\.py[^']*> /tmp/[^']*'"),
     "自检夹具（合成标本串）：自证用例正文里那条重定向标本串（现读为变量前缀形态）"
     "⇒ 夹具数据不是结论落点；命令位真写 /tmp 照旧红"),
)

# 判据件面（落点面扫的件集）：`scripts/gates/**` 的 `check-*.py` / `check-*.sh` / `mutate-scan`
def judge_files(root):
    d = os.path.join(root, JUDGE_DIR_REL)
    if not os.path.isdir(d):
        return None
    out = []
    for n in sorted(os.listdir(d)):
        p = os.path.join(d, n)
        if not os.path.isfile(p):
            continue
        if n == "mutate-scan" or (n.startswith("check-") and n.endswith((".py", ".sh"))):
            out.append(p)
    return out


def norm(s):
    """归一：**连续空白折成一个空格** —— 缩进 / 换行 / 对齐都不进判据面（行号不是判据）。"""
    return re.sub(r"\s+", " ", s).strip()


def unesc(s):
    """把 shell 双引号里的 `\\x` 还原成 `x`（`\\`` 这类转义不进指纹的字面量）。"""
    return re.sub(r"\\(.)", r"\1", s)


def subject_items(root):
    """返 (items, problems)。items = 归一后的「条」清单（**顺序无关**：算指纹时排序）。"""
    items, problems = [], []
    p = os.path.join(root, GATE_TABLE_REL)
    if not os.path.isfile(p):
        problems.append({"what": "gate_table_absent", "path": p})
    else:
        with open(p, encoding="utf-8") as f:
            txt = f.read()
        n = 0
        for m in ADD_STEP_RE.finditer(txt):
            items.append("step|%s|%s|%s|%s|%s" % (norm(m.group(1)), norm(unesc(m.group(2))),
                                                 norm(m.group(3)), norm(m.group(4)),
                                                 norm(unesc(m.group(5)))))
            n += 1
        if n == 0:
            problems.append({"what": "add_step_empty", "path": p})
    for rel in BASELINE_RELS:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            problems.append({"what": "baseline_absent", "path": p})
            continue
        with open(p, encoding="utf-8") as f:
            raw = f.read()
        try:
            obj = json.loads(raw)
        except ValueError as e:
            problems.append({"what": "baseline_unparsable", "path": p, "msg": str(e)})
            continue
        canon = json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        n_items = len(obj) if isinstance(obj, (list, dict)) else 0
        items.append("json|%s|%d|%s" % (rel, n_items, hashlib.sha256(canon.encode()).hexdigest()))
    return items, problems


def fingerprint(items, subject=SUBJECT):
    body = "subject=%s\n" % subject + "\n".join(sorted(items))
    return "sha256:" + hashlib.sha256(body.encode("utf-8")).hexdigest()


def fixture_exempt_of(rel, line):
    """命中行是否被「自检夹具登记」盖住 ⇒ 返 (条目序号, 为什么豁免)，否则 None。"""
    for idx, (globs, rx, why) in enumerate(FIXTURE_EXEMPT, 1):
        if not any(fnmatch.fnmatch(rel, g) for g in globs):
            continue
        if rx.search(line):
            return idx, why
    return None


def scan_landing(root):
    """落点面：返 (命中清单, 扫到的件数, 自豁免清单, 夹具登记豁免清单, 悬空登记清单)。

    自豁免两档（**结构性 · 打印理由** · 同门㉝ 口径）：
      ① 本件自己（判据表里必然把「写 /tmp」的形态写成字面量）⇒ 整件不在扫面里；
      ② 别的件的**自检夹具合成标本串** ⇒ 走 `FIXTURE_EXEMPT` 逐条登记（在册 `GAP-20260928-96`）：
         **逐条打印理由** · 不计入命中数（**不是静默过滤** ✗）；登记条目**命中 0 处**（悬空）⇒
         照实打印、**不判红**（同门㉝ 那一格口径）。
    """
    hits, files, exempt, fixture, stale = [], judge_files(root), [], [], []
    if files is None:
        return None, None, exempt, fixture, stale
    me = os.path.basename(__file__)
    seen = [0] * len(FIXTURE_EXEMPT)
    for p in files:
        rel = os.path.relpath(p, root)
        if os.path.basename(p) == me:
            exempt.append({"file": rel,
                           "reason": "自豁免：本件的判据表 / 注释把「写 /tmp」的形态写成字面量（结构性）"})
            continue
        with open(p, encoding="utf-8", errors="replace") as f:
            for i, line in enumerate(f.read().splitlines(), 1):
                for label, rx in LANDING_PATTERNS:
                    if rx.search(line):
                        fx = fixture_exempt_of(rel, line)
                        if fx:
                            seen[fx[0] - 1] += 1
                            fixture.append({"file": rel, "line": i, "form": label,
                                            "raw": line.strip(), "rule": fx[0], "reason": fx[1]})
                        else:
                            hits.append({"file": rel, "line": i,
                                         "form": label, "raw": line.strip()})
                        break
    for idx, (globs, _rx, why) in enumerate(FIXTURE_EXEMPT, 1):
        if seen[idx - 1] == 0:
            stale.append({"rule": idx, "globs": list(globs), "reason": why})
    return hits, files, exempt, fixture, stale


def state_dir(cli=None):
    if cli:
        return cli
    env = os.environ.get("ZERG_STATE_DIR", "").strip()
    if env:
        return env
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def load_records(state, subject=SUBJECT):
    p = os.path.join(state, RECORD_NAME)
    if not os.path.isfile(p):
        return [], p
    out = []
    with open(p, encoding="utf-8") as f:
        for i, line in enumerate(f.read().splitlines(), 1):
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            if rec.get("subject") == subject:
                rec["_line"] = i
                out.append(rec)
    return out, p


def append_record(state, rec):
    os.makedirs(state, exist_ok=True)
    p = os.path.join(state, RECORD_NAME)
    with open(p, "a", encoding="utf-8") as f:
        f.write(json.dumps(rec, ensure_ascii=False, sort_keys=True) + "\n")
    return p


def judge(root):
    """本门的判据（**只读**）：落点面 + 指纹面两遍现算。返 (rc, report)。"""
    rep = {"root": os.path.abspath(root), "landing_hits": [], "judge_files": 0,
           "fingerprint": "", "fingerprint2": "", "items": 0, "problems": [], "self_exempt": [],
           "fixture_exempt": [], "fixture_stale": []}
    if len(FIXTURE_EXEMPT) > FIXTURE_EXEMPT_MAX:
        rep["problems"].append({"what": "fixture_exempt_clamp",
                                "msg": "自检夹具登记 %d 条 > 硬钳 %d ⇒ 不给结论（防把『往登记表里加条目』当治本）"
                                       % (len(FIXTURE_EXEMPT), FIXTURE_EXEMPT_MAX)})
    hits, files, exempt, fixture, stale = scan_landing(root)
    if files is None:
        rep["problems"].append({"what": "judge_dir_absent", "path": os.path.join(root, JUDGE_DIR_REL)})
    else:
        rep["judge_files"] = len(files)
        rep["landing_hits"] = hits
        rep["self_exempt"] = exempt
        rep["fixture_exempt"] = fixture
        rep["fixture_stale"] = stale
    # 指纹面：**两遍独立读盘**（各自重扫一遍 ⇒ 顺序/缓存都不能影响结果）
    items1, prob1 = subject_items(root)
    items2, prob2 = subject_items(root)
    rep["items"] = len(items1)
    rep["fingerprint"] = fingerprint(items1)
    rep["fingerprint2"] = fingerprint(items2)
    rep["problems"] += prob1 + prob2
    if rep["problems"]:
        return 2, rep
    if rep["fingerprint"] != rep["fingerprint2"]:
        rep["problems"].append({"what": "two_runs_differ",
                                "a": rep["fingerprint"], "b": rep["fingerprint2"]})
        return 2, rep
    if rep["landing_hits"]:
        return 1, rep
    return 0, rep


def machine_doc(rep, rc, state, records, extra=None):
    doc = {
        "schema": "zerg/v1", "kind": "VerdictRecord", "tool": VERSION,
        "subject": SUBJECT,
        "fingerprint": rep.get("fingerprint", ""),
        "items": rep.get("items", 0),
        "verdict": {"rc": rc, "of": "check-verdict-record.py（落点面 + 指纹面）"},
        "landing_hits": rep.get("landing_hits", []),
        "landing_hit_count": len(rep.get("landing_hits", [])),
        "self_exempt": rep.get("self_exempt", []),
        "fixture_exempt": rep.get("fixture_exempt", []),
        "fixture_exempt_stale": rep.get("fixture_stale", []),
        "judge_files": rep.get("judge_files", 0),
        "record_path": os.path.join(state, RECORD_NAME),
        "records": len(records),
        "last_record": ({"ts": records[-1].get("ts"), "fingerprint": records[-1].get("fingerprint")}
                        if records else None),
        "problems": rep.get("problems", []),
        "warnings": [], "truncated": False,
    }
    if extra:
        doc.update(extra)
    return doc


def cmd_compare(state, subject=SUBJECT):
    """比同一 subject 的**最后两条**记录 ⇒ 相同 = 同一改动（可比）；不同 ⇒ 明说不可比（rc=2）。"""
    recs, p = load_records(state, subject)
    print("── 「算不算过」可比性对拍（%s）──" % VERSION)
    print("  结论件  : %s" % p)
    print("  同 subject 记录数: %d" % len(recs))
    if len(recs) < 2:
        print("⛔ 不给结论（rc=2）：不足两跑（同 subject 记录 %d 条 ≤ 1）⇒ 判不了可比性"
              "（**不拿「没得比」当「比过了」**）" % len(recs))
        return 2
    a, b = recs[-2], recs[-1]
    print("  前一跑  : ts=%s 指纹=%s" % (a.get("ts"), a.get("fingerprint")))
    print("  后一跑  : ts=%s 指纹=%s" % (b.get("ts"), b.get("fingerprint")))
    print("  判据 rc : %s / %s" % ((a.get("verdict") or {}).get("rc"), (b.get("verdict") or {}).get("rc")))
    if a.get("fingerprint") == b.get("fingerprint"):
        print("⇒ 同一改动两跑**指纹相同**（rc=0）：两条记录说的是同一份内容 ⇒ 结果**可比**")
        return 0
    print("✗ **仓变了，结果不可比**（rc=2 · 逐字就是这一句）：两条记录的指纹不同")
    print("  ⇒ 处置：要么把**后一跑**当新身份（另当一改动的结论看），要么回滚到前一跑的改动 ——")
    print("    **不许**把两次不同内容的结论并成一个数（`R-16` 同族口径）。")
    return 2


def cmd_fingerprint(root):
    items, prob = subject_items(root)
    if prob:
        print("⛔ 不给结论（rc=2）：判不了 subject：%s" % "; ".join(str(x) for x in prob))
        return 2
    print("%s  subject=%s 条=%d" % (fingerprint(items), SUBJECT, len(items)))
    return 0


def run(root, state, args):
    rc, rep = judge(root)
    recs, _p = load_records(state)
    last = recs[-1] if recs else None

    if args.record:
        rec = {"schema": "zerg/v1", "kind": "VerdictRecord", "tool": VERSION,
               "subject": SUBJECT, "fingerprint": rep.get("fingerprint", ""),
               "items": rep.get("items", 0),
               "verdict": {"rc": rc, "of": "check-verdict-record.py（落点面 + 指纹面）"},
               "ts": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
               "root": rep.get("root", "")}
        p = append_record(state, rec)
        print("已落结论件  : %s" % p)
        print("  指纹      : %s（条 %d）" % (rec["fingerprint"], rec["items"]))
        print("  判据      : rc=%d（本门本跑的「算不算过」）" % rc)
        print("★ 落点是**状态目录**（非 `/tmp`）· 记录只增不改（`--compare` 比最后两条）")
        return rc

    if args.json:
        print(json.dumps(machine_doc(rep, rc, state, recs), ensure_ascii=False))
        return rc

    print("── 「算不算过」机器读落点 + 稳定指纹（%s · `W-47`）──" % VERSION)
    print("  仓根      : %s" % rep["root"])
    print("  结论件落点: %s（**状态目录** · 非 `/tmp`）" % os.path.join(state, RECORD_NAME))
    print("  判据件面  : %d 件（%s 的 check-*.py|sh + mutate-scan · 全只读）"
          % (rep["judge_files"], JUDGE_DIR_REL))
    for e in rep.get("self_exempt", []):
        print("              · 自豁免 %s —— %s" % (e["file"], e["reason"]))
    for e in rep.get("fixture_exempt", []):
        print("              · 夹具登记豁免 %s:%d [%s]（条目 %d）—— %s"
              % (e["file"], e["line"], e["form"], e["rule"], e["reason"]))
    for e in rep.get("fixture_stale", []):
        print("              · 悬空登记（条目 %d · %s）命中 0 处 —— 照实打印 · **不判红**（同门㉝ 口径）"
              % (e["rule"], " ".join(e["globs"])))
    print("  subject   : %s（门表 add_step 条 + 两件命令面基线件的规范化 JSON）" % SUBJECT)
    print("  指纹      : %s（条 %d）" % (rep["fingerprint"], rep["items"]))
    print("  两遍现算  : %s ⇒ %s" % (rep["fingerprint2"],
                                     "逐字相同（内容算出 · 与行号无关）" if rep["fingerprint"] == rep["fingerprint2"]
                                     else "**不同**（判不了）"))

    if rep["problems"]:
        print("⛔ 不给结论（rc=2）：")
        for p in rep["problems"]:
            print("   · %s：%s %s" % (p.get("what"), p.get("path", ""), p.get("msg", "")))
        return 2
    if rc == 1:
        print("✗ 落点面命中 %d 处（rc=1 · 逐条点名）：判据件把结论写到了 `/tmp` 上" % len(rep["landing_hits"]))
        for h in rep["landing_hits"]:
            print("   · %s:%d [%s] 原样 `%s`" % (h["file"], h["line"], h["form"], h["raw"][:90]))
        print("⇒ 处置（**不是**改本门）：把该判据件的结论落点挪进**仓内**（`scripts/gates/**`）"
              "或**状态目录** `<state>/…`，并同笔写清为什么。")
        return 1

    if last:
        same = (last.get("fingerprint") == rep["fingerprint"])
        print("  最近记录  : ts=%s 指纹=%s" % (last.get("ts"), last.get("fingerprint")))
        if same:
            print("              ⇒ 与本跑**逐字相同**：同一改动 ⇒ 与那一次的结论**可比**")
        else:
            print("              ⇒ 与本跑**不同** ⇒ **仓变了，结果不可比**"
                  "（本格照实打印 · **不定本步红绿** —— 要判可比性请显式 `--compare`）")
    else:
        print("  最近记录  : 无（结论件记录数 0）⇒ 本条只判到「落点面 + 本跑两遍现算」；"
              "要判可比性：先 `--record` 两跑再 `--compare`")
    print("⇒ 判据（rc=0）：落点面 0 命中（判据件的结论没有散在 `/tmp`）"
          " + 指纹面两遍现算逐字相同（内容算出 · 非行号）"
          " + 自检夹具登记豁免 %d 处（逐条打印理由 · 不算命中）" % len(rep["fixture_exempt"]))
    print("   ★ 本件只**读**门表与两件基线：一个字未改（`--record` 才写，且只写状态目录）")
    return 0


def list_rules():
    print("── %s 判据与落点 ──" % VERSION)
    print("  subject : %s" % SUBJECT)
    print("    · 门表条 : %s —— `add_step` 五格归一成 `scope|步名|档位|工作目录|命令串`" % GATE_TABLE_REL)
    for r in BASELINE_RELS:
        print("    · 基线件 : %s —— 规范化 JSON（sort_keys + 紧凑分隔符）的 sha256 + 条数" % r)
    print("  指纹    : sha256(subject + 逐条排序拼接) —— **不含行号 / 缩进 / 排版**")
    print("  落点面  : 扫 %s 的 check-*.py|sh + mutate-scan，查「结论写 /tmp」的**写盘**形态：" % JUDGE_DIR_REL)
    for label, rx in LANDING_PATTERNS:
        print("    · %-14s %s" % (label, rx.pattern))
    print("  结论件  : <状态目录>/%s（JSONL · `--record` 追加 · **非 /tmp**）" % RECORD_NAME)
    print("  夹具登记: `FIXTURE_EXEMPT` %d 条（硬钳 %d）—— 条目 = (件 glob, 行正则, 为什么豁免) ·"
          " 逐命中打印理由 · 悬空不判红" % (len(FIXTURE_EXEMPT), FIXTURE_EXEMPT_MAX))
    for idx, (globs, rx, _why) in enumerate(FIXTURE_EXEMPT, 1):
        print("    · 条目%d %s  %s" % (idx, " ".join(globs), rx.pattern))
    print("  退码    : 0 两条面皆过 · 1 落点面命中 · 2 不给结论（缺件/空转/两遍不同/自检不过/"
          "不足两跑/指纹不同）")
    print("★ 已挂门：`gates` scope · `tri` 阻断 · 在默认集 · 不进 `FAST_STEPS`。")
    return 0


# ───────────────────────────── 自检（正控 + 成对负控 · 合成夹具 · 不碰真仓 / 真状态目录）─────────────
FIXTURE_GATE = """#!/usr/bin/env bash
# 合成门表（夹具）
run_suite() {
  case "$1" in
    gates)
      add_step gates "门甲 夹具一（阻断）" tri "${REPO_ROOT}" "python3 scripts/gates/check-a.py"
      add_step gates "门乙 夹具二（只报告）" tri-report "${REPO_ROOT}" "python3 scripts/gates/check-b.py"
      ;;
  esac
}
"""

FIXTURE_BASE = {"commands": 3, "mustfail_cases": 5, "blanks": 0}
FIXTURE_MATRIX = {"cases": [{"command": "zerg a", "want_rc": 2}, {"command": "zerg b", "want_rc": 2}]}

# 「**同一批条** · 行号 / 缩进 / 次序全变了」的变体（负控②用）—— 逐条与 FIXTURE_GATE 同内容。
FIXTURE_GATE_SHUFFLED = """#!/usr/bin/env bash
# 合成门表（夹具 · 同一批条）

run_suite() {

        case "$1" in



                gates)
      # 一条注释（不改任何条）

                add_step gates      "门乙 夹具二（只报告）"     tri-report "${REPO_ROOT}"    "python3 scripts/gates/check-b.py"


                add_step   gates "门甲 夹具一（阻断）"  tri   "${REPO_ROOT}" "python3 scripts/gates/check-a.py"
                ;;
        esac
}
"""


def mk_fixture(tmp, name, gate=FIXTURE_GATE, base=None, matrix=None, judge_body=None,
               judge_name="check-a.py"):
    """合成夹具：`<tmp>/<name>/` = 假仓（门表 + 两件基线 + 判据件面）。"""
    root = os.path.join(tmp, name)
    os.makedirs(os.path.join(root, "scripts", "gates"), exist_ok=True)
    os.makedirs(os.path.join(root, "core", "cmd", "zerg", "testdata"), exist_ok=True)
    with open(os.path.join(root, GATE_TABLE_REL), "w", encoding="utf-8") as f:
        f.write(gate)
    with open(os.path.join(root, BASELINE_RELS[0]), "w", encoding="utf-8") as f:
        json.dump(base or FIXTURE_BASE, f, ensure_ascii=False, indent=2)
    with open(os.path.join(root, BASELINE_RELS[1]), "w", encoding="utf-8") as f:
        json.dump(matrix or FIXTURE_MATRIX, f, ensure_ascii=False, indent=2)
    clean = judge_body if judge_body is not None else (
        '#!/usr/bin/env python3\n"""夹具判据件（落点在仓内）"""\nprint("ok")\n')
    with open(os.path.join(root, "scripts", "gates", judge_name), "w", encoding="utf-8") as f:
        f.write(clean)
    return root


# 自检夹具：**判据件自己的合成标本串**（在册 `GAP-20260928-96`）
#   件名必须与 `FIXTURE_EXEMPT` 的 `件 glob` 逐字相同（登记按**精确件名**匹配 · 不是路径前缀）。
SPEC_CASE = ("#!/usr/bin/env python3\n"
             "# 合成夹具判据件（自检 NEG_CASES：标本串，跑起来一个字不写 /tmp）\n"
             "NEG_CASES = [\n"
             "    (\"③b\", \"单元正文在 .sh 的 heredoc 里\", \"neg3b\", \"deploy.sh\",\n"
             "     '#!/usr/bin/env bash\\ncat > /tmp/edge-node-agent.service << UNIT_EOF\\n', \"UNIT-OPT-ARGV\"),\n"
             "]\n")
# **真写**（命令位调用）——登记**不许**盖住它（同件、同 /tmp，形态不同 ⇒ 照旧红）
REAL_WRITE = ("#!/usr/bin/env python3\nimport subprocess\n"
              "subprocess.run(\"git status > /tmp/st-before.txt\", shell=True)\n")


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="vr-selftest-")

    def case(name, want, got, extra_ok=True, note=""):
        nonlocal ok
        good = (want == got) and extra_ok
        ok = ok and good
        print("  %s %s（rc=%s · 期望 %s）%s" % ("✓" if good else "✗", name, got, want, note))

    def cap(fn, *a, **kw):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc = fn(*a, **kw)
        return rc, buf.getvalue()

    # ① 正控：同一改动两跑 ⇒ 指纹逐字相同（合成夹具两遍独立现算）
    r1 = mk_fixture(tmp, "case-1")
    rc, out = cap(run, r1, os.path.join(tmp, "state-1"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("① 正控：落点面 0 命中 + 两遍现算逐字相同 ⇒ rc=0", 0, rc, "逐字相同" in out)

    # ② 负控：**只改行号**（加空行 + 挪行 + 改缩进 + 换次序）⇒ 指纹**不变**（⇒ 不是行号算的）
    r2 = mk_fixture(tmp, "case-2", gate=FIXTURE_GATE_SHUFFLED)
    f1 = fingerprint(subject_items(r1)[0])
    f2 = fingerprint(subject_items(r2)[0])
    case("② 负控：只改行号 / 加空行 / 挪行 / 改缩进 / 换次序 ⇒ 指纹**不变**", True, f1 == f2,
         note="  （两跑指纹逐字同：%s…）" % f1[7:23])

    # ③ 负控：改**内容**（步名一格）⇒ 指纹**变**
    r3 = mk_fixture(tmp, "case-3", gate=FIXTURE_GATE.replace("门甲 夹具一", "门甲 夹具一改"))
    f3 = fingerprint(subject_items(r3)[0])
    case("③ 负控：改内容（步名一格）⇒ 指纹**变**", True, f3 != f1)

    # ③b 负控：增删条 ⇒ 指纹变
    r3b = mk_fixture(tmp, "case-3b", gate=FIXTURE_GATE.replace(
        '      add_step gates "门乙 夹具二（只报告）" tri-report "${REPO_ROOT}" "python3 scripts/gates/check-b.py"\n', ""))
    f3b = fingerprint(subject_items(r3b)[0])
    case("③b 负控：删一条 add_step ⇒ 指纹变（少一条 == 另一份内容）", True, f3b != f1)

    # ④ 负控：**重排版**两件基线（缩进 / 换行 / 键序）⇒ 指纹不变
    import collections
    base4 = collections.OrderedDict([("blanks", 0), ("mustfail_cases", 5), ("commands", 3)])
    r4 = mk_fixture(tmp, "case-4", base=base4)
    with open(os.path.join(r4, BASELINE_RELS[0]), "w", encoding="utf-8") as f:
        f.write(json.dumps(base4) + "\n" * 3)          # 紧凑单行 + 尾部空行
    f4 = fingerprint(subject_items(r4)[0])
    case("④ 负控：基线件只重排版（缩进/换行/键序）⇒ 指纹不变", True, f4 == f1)

    # ⑤ 负控：`--compare` 两跑不同 ⇒ rc=2 **且逐字**「仓变了，结果不可比」
    st = os.path.join(tmp, "state-cmp")
    st1 = os.path.join(tmp, "state-cmp1")
    for root, s in ((r1, st1), (r3, st1)):
        rc0, _ = cap(run, root, s, argparse.Namespace(record=True, json=False, compare=False))
    rc, out = cap(cmd_compare, st1)
    case("⑤ 负控：两跑指纹不同 ⇒ `--compare` rc=2", 2, rc, "仓变了，结果不可比" in out)

    # ⑤b 真话负控：同一夹具两跑 ⇒ `--compare` 回绿（rc=0 + 逐字「指纹相同」）
    rc0, _ = cap(run, r1, st, argparse.Namespace(record=True, json=False, compare=False))
    rc0, _ = cap(run, r1, st, argparse.Namespace(record=True, json=False, compare=False))
    rc, out = cap(cmd_compare, st)
    case("⑤b 真话负控：同一改动两跑 ⇒ `--compare` rc=0 + 逐字「指纹相同」",
         0, rc, "指纹相同" in out)

    # ⑤c 负控：不足两跑 ⇒ rc=2 不给结论
    st2 = os.path.join(tmp, "state-one")
    cap(run, r1, st2, argparse.Namespace(record=True, json=False, compare=False))
    rc, out = cap(cmd_compare, st2)
    case("⑤c 负控：只能一跑 ⇒ `--compare` rc=2（不给结论）", 2, rc, "不足两跑" in out)

    # ⑥ 负控：落点面 —— 夹具里一个判据件把结论写 `/tmp/st-before.txt` ⇒ rc=1 + 点名「件:行」
    dirty = '#!/usr/bin/env python3\nimport subprocess\nsubprocess.run("git status > /tmp/st-before.txt", shell=True)\n'
    r6 = mk_fixture(tmp, "case-6", judge_body=dirty)
    rc, out = cap(run, r6, os.path.join(tmp, "state-6"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑥ 负控：判据件把结论写 `/tmp` ⇒ rc=1", 1, rc, "落点面命中" in out)
    case("⑥b 负控点名到「件:行 + 原样」", True,
         ("scripts/gates/check-a.py:3" in out and "/tmp/st-before.txt" in out))

    # ⑥c 真话负控：把同一处落点挪进仓内 ⇒ 回绿（证明红是红在**那一处落点**上）
    clean = dirty.replace("/tmp/st-before.txt", "scripts/gates/.verdict-before.txt")
    r6c = mk_fixture(tmp, "case-6c", judge_body=clean)
    rc, out = cap(run, r6c, os.path.join(tmp, "state-6c"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑥c 真话负控：落点挪进仓内 ⇒ 回绿", 0, rc, "落点面 0 命中" in out)

    # ⑦ 负控：空转（门表里 0 条 add_step）⇒ rc=2 不给结论
    r7 = mk_fixture(tmp, "case-7", gate="#!/usr/bin/env bash\n# 合成门表：一条 add_step 都没有\n")
    rc, out = cap(run, r7, os.path.join(tmp, "state-7"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑦ 负控：空转（0 条 add_step）⇒ rc=2 不给结论", 2, rc, "不给结论" in out)

    # ⑧ 负控：基线件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）
    r8 = mk_fixture(tmp, "case-8")
    os.remove(os.path.join(r8, BASELINE_RELS[1]))
    rc, out = cap(run, r8, os.path.join(tmp, "state-8"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑧ 负控：基线件不在盘 ⇒ rc=2", 2, rc, "不给结论" in out)

    # ⑨ 负控：基线件解不动（JSON 坏）⇒ rc=2
    r9 = mk_fixture(tmp, "case-9")
    with open(os.path.join(r9, BASELINE_RELS[0]), "w", encoding="utf-8") as f:
        f.write("{ 坏 JSON \n")
    rc, out = cap(run, r9, os.path.join(tmp, "state-9"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑨ 负控：基线件解不动 ⇒ rc=2（不许默认成绿）", 2, rc, "不给结论" in out)

    # ⑩ `--json` 机读：判据结论 + 指纹 + 落点命中都在（形状自证）
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        rc = run(r1, os.path.join(tmp, "state-10"), argparse.Namespace(
            record=False, json=True, compare=False))
    doc = json.loads(buf.getvalue())
    case("⑩ `--json` 给指纹 + 条数 + 判据 rc + 结论件落点", True,
         (doc["fingerprint"].startswith("sha256:") and doc["items"] == 4 and doc["verdict"]["rc"] == 0
          and doc["record_path"].endswith(RECORD_NAME)),
         note="  （items=%d）" % doc["items"])

    # ⑪ 落点面扫的是**写盘**形态，不是「提到 /tmp」：合成判据件只在注释里提 /tmp ⇒ 不红
    r11 = mk_fixture(tmp, "case-11", judge_body="#!/usr/bin/env python3\n# 历史：旧法把结论落在 /tmp/x（已挪走）\nprint(1)\n")
    rc, out = cap(run, r11, os.path.join(tmp, "state-11"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑪ 对照：只在注释里『提到 /tmp』⇒ 不算落点命中（rc=0）", 0, rc, "落点面 0 命中" in out)

    # ⑫ 自豁免（**结构性 · 打印理由**）：同名件（本件自己）里的形态字面量不进扫面
    r12 = mk_fixture(tmp, "case-12")
    with open(os.path.join(r12, "scripts", "gates", "check-verdict-record.py"), "w",
              encoding="utf-8") as f:
        f.write('#!/usr/bin/env python3\nPAT = r"> /tmp/x"\nprint("bash < /tmp/x")\n')
    rc, out = cap(run, r12, os.path.join(tmp, "state-12"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑫ 自豁免：同名件（本件自己）不在扫面里 · 理由照打", 0, rc,
         ("自豁免" in out and "落点面 0 命中" in out))
    case("⑫b 自豁免只放**本件**、不放别人（同名判据件别处仍照判）", True,
         ("scripts/gates/check-a.py" not in out))

    # ⑬ 正控（在册 `GAP-20260928-96`）：判据件的**自检夹具合成标本串**（`NEG_CASES` 用例正文）
    #    ⇒ 走 `FIXTURE_EXEMPT` 登记 ⇒ **不算落点命中**（rc=0 · 逐条打印理由）
    r13 = mk_fixture(tmp, "case-13", judge_name="check-token-argv.py", judge_body=SPEC_CASE)
    rc, out = cap(run, r13, os.path.join(tmp, "state-13"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑬ 正控：合成标本串（夹具 NEG_CASES 正文）⇒ 登记豁免 · rc=0", 0, rc,
         ("夹具登记豁免" in out and "落点面 0 命中" in out))
    case("⑬b 登记豁免**逐条打印理由** + 未登记的条目**照实打印悬空 · 不判红**", True,
         ("自检夹具（合成标本串）" in out and "悬空登记" in out))

    # ⑬c 负控（**证未放宽真判据**）：**同一件**里的**真写**（命令位调用）⇒ 登记盖不住 ⇒ 照旧红
    r13c = mk_fixture(tmp, "case-13c", judge_name="check-token-argv.py", judge_body=REAL_WRITE)
    rc, out = cap(run, r13c, os.path.join(tmp, "state-13c"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑬c 负控：同件里命令位真写 /tmp ⇒ 照旧红（rc=1）", 1, rc, "落点面命中" in out)

    # ⑬d 负控：另一登记件（check-wired-scripts.py）里的 `open(…,"w")` 写盘 ⇒ 照旧红
    r13d = mk_fixture(tmp, "case-13d", judge_name="check-wired-scripts.py",
                      judge_body='#!/usr/bin/env python3\nf = open("/tmp/w.log", "w")\n')
    rc, out = cap(run, r13d, os.path.join(tmp, "state-13d"), argparse.Namespace(
        record=False, json=False, compare=False))
    case("⑬d 负控：另一登记件里 open(...,\"w\") 写盘 ⇒ 照旧红（rc=1）", 1, rc, "落点面命中" in out)

    shutil.rmtree(tmp, ignore_errors=True)
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="「算不算过」的机器读落点 + 稳定指纹（W-47）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认真源：本脚本所在仓）")
    ap.add_argument("--state-dir", default=None,
                    help="状态目录（缺省 $ZERG_STATE_DIR 或 ~/.zerg/state）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--fingerprint", action="store_true", help="只打 subject 指纹（人面一行）")
    ap.add_argument("--record", action="store_true", help="把本跑结论落成一条记录（写状态目录）")
    ap.add_argument("--compare", action="store_true", help="比同一 subject 的最后两条记录")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    st = state_dir(a.state_dir)
    if a.fingerprint:
        return cmd_fingerprint(a.root)
    if a.compare:
        return cmd_compare(st)
    return run(a.root, st, a)


if __name__ == "__main__":
    sys.exit(main())

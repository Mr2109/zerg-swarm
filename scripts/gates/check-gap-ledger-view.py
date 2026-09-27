#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gap-ledger-view.py —— 缺口账「**视图分离**」① 三方对拍 + 体积/单行纪律 + 指纹写保护门

题面（仓外 · 逐字引用 = `Zerg-内部文档/项目文档/v2.5.13/设计-缺口账视图分离-v1.3-20260925.md`）
--------------------------------------------------------------------
· §0 第 **①** 序（实现顺序 · 钉死）——「落地门 `scripts/gates/check-gap-ledger-view.py`」，
  判据栏逐字「它现跑**必须红**（真源 9 条 vs 结账单未闭 166 条 ⇒ 直接坐实「导出链未落地」✓）」
· §9 判据表 ——「视图一致（三方对拍条数与逐条一致）· 体积 ≤300 行 / ≤60KB（R-01）·
  单行 ≤200 字符（R-02）· 写保护（结账单指纹匹配 R-05）」
· §7 —— `R-01` 体积上限（**≤300 行 / ≤60KB**）· `R-02` 单行纪律（**≤200 字符**）·
  `R-03` ★「缺**真源 ↔ export 输出 ↔ 结账单**三方一致性校验 ⇒ 漂移捕不到」·
  `R-05`「结账单**写保护**：带生成指纹；手改 ⇒ 门红」
· §10.5 —— 写保护强度推荐「**门红 + 审计留痕**（不阻断提交 ✗ 防误伤历史件）」

本件补的那一格（**病根 = 两处账之间没有机器在读**）
--------------------------------------------------
`GAP-20260925-02` 的 symptom 原话：CLI 真源（`<状态目录>/zerg-cli-gaps.jsonl`）与文档侧的结转账
（`Zerg-内部文档/…/缺口总账-*.md`）之间**没有机器同步面** ⇒ 必然漂移、全靠人手抄。所以本件的判据
**不是**「把两边数字对齐」（那是 ③ 的活），而是：

  ① **现跑三方**：真源（逐行 `id`+`state`）↔ `zerg gap export` **今天真跑的输出** ↔ 结转账件（生成件）；
  ② **体积/单行**：结转账件现读 **行数 / 字节 / 最长单行**，超上限 ⇒ 判红（逐字打实际值）；
  ③ **写保护**：生成件里的**指纹**与 export 现跑产出里的指纹**逐字比对**；
  ④ **残件只报告**：旧版账（`v2.5.x/缺口总账-*.md` 一类历史件）**列出来**（行数/字节/最长行），
     **不入退码** ✗ —— 历史不改 ✓。

判据（R1…R6 · **逐条给判红/报告的理由**）
----------------------------------------
  **R1 三方对拍（判红）**
    · R1a 真源 ↔ export 现跑：真源条数/未闭/已闭 与导出件头三数逐字比；真源**未闭** id 集与
      导出正文逐条 `id`+`state` 比 ⇒ 缺哪个 / 多哪个 / 哪个态不一致，**逐条点名**（超上限截断并打计数）。
    · R1b 真源 ↔ 结转账件（第三方腿）：账件里**逐条条目标记**（行首的条目行 / 表元 · **结构锚**）的 id
      与真源未闭 id 集比；**正文散文里引用的号不算条目** ✗（如某条 symptom 里写「与 Q-075 同根」）；
      **账件不是生成件**（缺生成标记）时另记「**生成件未接管**」⇒ 同样是 R1 的红（今天红的主因）。
  **R2 体积（判红）**：结转账件 ≤ **300 行** · ≤ **60KB**；超 ⇒ 判红并**逐字打实际值**。
  **R3 单行（判红）**：结转账件**最长行 ≤ 200 字符**；超 ⇒ 判红并打**行号 + 长度**（+ 前 60 字）。
  **R4 写保护（指纹 · 有则判红 / 无则只报告）**：先现读 export 今天的产出**到底有没有指纹** ——
    没有 ⇒ 只报告「**无指纹 ⇒ 无法判手改**」（**不自己发明指纹格式** ✗ · 不改 export ✗）；
    有 ⇒ 生成件指纹必须**逐字相同**，生成件缺指纹或与现跑不符 ⇒ 判红。
  **R5 缺件/空转（判红 = rc=2）**：真源不可读 / 0 条 / export 跑不动（非 0 退码）/ 产出解析不出 /
    文档仓里一件 `缺口总账-*.md` 都没有 —— 一律 **rc=2「不给结论」**，**不许当绿** ✗。
  **R6 旧版账（只报告）**：`项目文档/<版>/缺口总账-*.md` 一类的**非当期**历史件 ⇒ 列进 reports
    （行数/字节/最长行）但**不入退码** ✗（历史不改 ✓）；`承接自-*/` 下的承接副本另标「豁免 · 只报告」。

登记与豁免（**唯二两条 · 不是放宽判据**）
----------------------------------------
· **登记面**（本件常量 = 机器可读的唯一真源 · 逐条见 `--list-rules`）：未闭/已闭态闭集（与
  `zerg gap export` 的 `gapOpenStates` 同口径）· 面闭集 · prio 闭集 · 三个上限 · 生成标记逐字句。
· **豁免**（只此一条）：路径段 `承接自`（上版账的**承接副本**）⇒ 历史件、只报告。白名单之外一律照判 ✓。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 三方对拍一致 + 体积/单行合规 + （有指纹时）指纹匹配
  1 = 三方不一致（逐条点名）/ 体积超 / 单行超 / 指纹不符 —— 判红
  2 = **不给结论**：真源不可读或空转 / `zerg gap export` 跑不动（不许当绿 ✗）/ 产出解析不出 /
      文档仓一件账都没有 / 自检不过 —— 一律显式打印

用法
----
    python3 scripts/gates/check-gap-ledger-view.py                    # 判（读真源 + 现跑 export + 读账件）
    python3 scripts/gates/check-gap-ledger-view.py --json              # 机读（zerg/v1 形状）
    python3 scripts/gates/check-gap-ledger-view.py --docs-root DIR     # 换文档仓根（默认真源：主仓的兄弟目录）
    python3 scripts/gates/check-gap-ledger-view.py --source PATH       # 换真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）
    python3 scripts/gates/check-gap-ledger-view.py --root DIR          # 换主仓仓根（只用来定位 docs/ 兄弟目录）
    python3 scripts/gates/check-gap-ledger-view.py --list-rules        # 打判据 / 登记面 / 豁免 / 上限
    python3 scripts/gates/check-gap-ledger-view.py --self-test         # 只跑自检（合成夹具 · 不碰真源/真账）

★ 导出器**只认本门隔壁那一个**：`<本脚本所在仓>/bin/zerg gap export`（**不**随 `--root` 换 ✗）——
  免得一个假仓根就能拿假导出器把自己判绿。
★ 本笔**只新增本件**：不碰真源 jsonl / 不碰 `gap export` 实现（Go）/ 不碰旧版账与任何 `Zerg-内部文档` 件 /
  不挂 `precommit-gates.sh`（另一批在改它 ⚠）· 本件纯**只读**（`gap export` 自身也只读）✓。
"""

import argparse
import collections
import json
import os
import re
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
# ★ 导出器 = **本脚本所在仓**的 `bin/zerg`（不随 --root 换：假仓根拿不到假导出器）
EXPORT_BIN = os.path.join(DEFAULT_ROOT, "bin", "zerg")
EXPORT_ARGS = ("gap", "export")
EXPORT_TIMEOUT = 90
VERSION = "check-gap-ledger-view.py v1.0.0"

# ── 登记面（机器可读的唯一真源）────────────────────────────────────────────────
SOURCE_NAME = "zerg-cli-gaps.jsonl"
STATE_DIR_DEFAULT = os.path.join(os.path.expanduser("~"), ".zerg", "state")
LEDGER_DIR = "项目文档"
LEDGER_NAME_RE = re.compile(r"^缺口总账-(\d{8})\.md$")
#   ★ 拆件后的**件族**形状（2026-09-27 现读真产出现形状）：索引件 `缺口总账-<日期>.md`
#   + 页件 `缺口总账-<日期>-<面>-pNN.md`（面 = 六值闭集 · pNN 两位）。日期两种形态都收：
#   `20260927`（历史件既有形态）与 `2026-09-27`（拆件产出形态）—— 归一化只去 `-` 再比。
LEDGER_DATE_PAT = r"(\d{8}|\d{4}-\d{2}-\d{2})"
INDEX_NAME_RE = re.compile(r"^缺口总账-%s\.md$" % LEDGER_DATE_PAT)
PAGE_NAME_RE = re.compile(r"^缺口总账-%s-([^-]+)-p(\d{2})\.md$" % LEDGER_DATE_PAT)
#   生成标记逐字句（`zerg gap export` 人面第 3 行现读逐字 · 本件照抄、不新造）
GENERATED_MARKER = "> 生成件：由 `zerg gap export`"
#   ★ 生成标记的逐字句**含「命令怎么调的」**：真 `zerg gap export` 人面 = 上面那一句；拆件产出
#   （`zerg gap export --out <目录>`）把旗标写进反引号里 ⇒ 收尾反引号那一字对不上（现读真产出坐实）。
#   ⇒ 收两种形态：完整句 + **同一句去掉收尾反引号的前缀**（同源派生 · 不新造口径）。
GENERATED_MARKERS = (GENERATED_MARKER, GENERATED_MARKER.rstrip("`"))
#   未闭 / 已闭态闭集（与 export 的 gapOpenStates / gapStateClosed 同口径）
OPEN_STATES = ("仍缺", "已派", "已立项", "回归")
CLOSED_STATES = ("已解", "不做")
STATE_CLOSED = OPEN_STATES + CLOSED_STATES
IMPACT_CLOSED = ("命令面", "门禁面", "文档面", "公开面", "换件面", "归档面")
PRIO_CLOSED = ("P0", "P1", "P2")
#   三个上限（设计稿 §3 / R-01 / R-02）
MAX_LINES = 300
MAX_BYTES = 60 * 1024
MAX_LINE_CHARS = 200
#   逐条点名上限（超出打计数 + 前 N 条）
NAME_LIMIT = 5

# ── 豁免（**只此一条**：路径段命中即历史件 ⇒ 只报告，不入 R1b/R2/R3）──────────────
EXEMPT_PATH_SEGS = ("承接自",)

# ── 指纹载体（**先现读今天的产出有没有**；今天没有 ⇒ R4 只报告，不发明格式 ✗）──────
FINGERPRINT_CARRIERS = (
    re.compile(r"指纹[^\n]{0,40}?([0-9a-fA-F]{8,64})"),
    re.compile(r"\bfp\s*[=:]\s*([0-9a-fA-F]{8,64})"),
    re.compile(r"\bsha256\s*[=:]\s*([0-9a-fA-F]{8,64})", re.I),
)

# ── 解析式（导出人面 / 账件）───────────────────────────────────────────────────
HDR_RE = re.compile(r"真源条数 \*\*(\d+)\*\* · 未闭 \*\*(\d+)\*\* · 已闭 \*\*(\d+)\*\*"
                    r"（其中「不做」\*\*(\d+)\*\*）")
FACE_RE = re.compile(r"^> 按面：(.+)$", re.M)
BULLET_RE = re.compile(r"^- `([^`]+)` (\S+) · \*\*([^*]+)\*\* · ")
FOOT_RE = re.compile(r"未闭 (\d+) 条 · 单行上限 (\d+) 字符")
#   ★ 条目 id 的取法 = **结构锚**（只认条目标记：行首 `- \`ID\`` / 表元 `| \`ID\` |`）——
#   账件正文（symptom 散文）里**引用别的号**是常态（如某条写「与 Q-075 同根」），那些号**不是条目**；
#   拿全文扫出来的号当条目 ⇒ 真源未闭里没有的散文引用会被误判成「账件里**多**」✗（假红）。
ITEM_MARK_BULLET_RE = re.compile(r"^- `(GAP-\d{8}-\d+|Q-\d{3})`(?:\s|$)")
ITEM_MARK_TABLE_RE = re.compile(r"^\|\s*`?(GAP-\d{8}-\d+|Q-\d{3})`?(?:\s*\||\s*$)")


def dash(txt, n=60):
    """点名的安全截断（只给看，不改任何件）。"""
    s = txt.strip().replace("\n", " ")
    return s if len(s) <= n else s[:n] + "…"


# ═════════════════════════════════════ 读盘面 ═════════════════════════════════════
def read_source(path):
    """读真源 jsonl ⇒ (状态, 读数)。状态 ∈ `ok` / `absent` / `unreadable` / `badline` / `empty`。"""
    if not os.path.isfile(path):
        return "absent", {"path": path}
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        return "unreadable", {"path": path, "msg": str(e)}
    recs, bad = [], []
    for i, ln in enumerate(raw.splitlines(), 1):
        s = ln.strip()
        if not s:
            continue
        try:
            o = json.loads(s)
        except ValueError as e:
            bad.append({"line": i, "msg": str(e)})
            continue
        recs.append({"line": i, "id": str(o.get("id", "")), "state": str(o.get("state", "")),
                     "impact": str(o.get("impact", "")), "prio": str(o.get("prio", ""))})
    if bad:
        return "badline", {"path": path, "bad": bad, "recs": recs}
    if not recs:
        return "empty", {"path": path, "lines": len(raw.splitlines()), "bytes": len(raw.encode())}
    ids = [r["id"] for r in recs]
    dup = [k for k, v in collections.Counter(ids).items() if v > 1]
    read = {
        "path": path, "lines": len(raw.splitlines()), "bytes": len(raw.encode()),
        "recs": len(recs), "ids": ids, "dup_ids": dup,
        "states": dict(collections.Counter(r["state"] for r in recs)),
        "open": sorted(r["id"] for r in recs if r["state"] in OPEN_STATES),
        "closed": sorted(r["id"] for r in recs if r["state"] in CLOSED_STATES),
        "state_of": {r["id"]: r["state"] for r in recs},
        "out_of_set": [{"line": r["line"], "id": r["id"], "field": f, "value": r[f]}
                       for r in recs for f in ("state", "impact", "prio")
                       if (f == "state" and r["state"] not in STATE_CLOSED)
                       or (f == "impact" and r["impact"] not in IMPACT_CLOSED)
                       or (f == "prio" and r["prio"] and r["prio"] not in PRIO_CLOSED)],
    }
    return "ok", read


def run_export(state_dir):
    """现跑导出器。★ 子进程环境里把 `ZERG_STATE_DIR` 钉到**真源所在目录** ⇒ 导出器读的就是本门读的那一件。"""
    env = dict(os.environ)
    env["ZERG_STATE_DIR"] = state_dir
    try:
        p = subprocess.run([EXPORT_BIN] + list(EXPORT_ARGS), cwd=DEFAULT_ROOT, env=env,
                           capture_output=True, text=True, timeout=EXPORT_TIMEOUT)
    except FileNotFoundError as e:
        return "no_bin", {"cmd": EXPORT_BIN, "msg": str(e)}, ""
    except subprocess.TimeoutExpired:
        return "timeout", {"cmd": EXPORT_BIN}, ""
    return ("ok" if p.returncode == 0 else "rc"), {"cmd": EXPORT_BIN, "rc": p.returncode,
                                                   "stderr": p.stderr}, p.stdout


def parse_export(out):
    """解析导出现跑产出 ⇒ dict 或 None（解析不出 ⇒ 不给结论）。"""
    h = HDR_RE.search(out)
    f = FOOT_RE.search(out)
    if not h or not f:
        return None
    faces = {}
    fm = FACE_RE.search(out)
    if fm:
        for part in fm.group(1).split("·"):
            part = part.strip()
            if " " in part:
                name, num = part.rsplit(" ", 1)
                if num.isdigit():
                    faces[name.strip()] = int(num)
    bullets = [{"id": m.group(1), "prio": m.group(2), "state": m.group(3)}
               for m in (BULLET_RE.match(l) for l in out.splitlines()) if m]
    return {"total": int(h.group(1)), "open": int(h.group(2)), "closed": int(h.group(3)),
            "no_decide": int(h.group(4)), "faces": faces, "bullets": bullets,
            "foot_open": int(f.group(1)), "foot_line_max": int(f.group(2)),
            "generated_marker": GENERATED_MARKER in out,
            "fingerprint": find_fingerprint(out)}


def find_fingerprint(text):
    """指纹载体现读（**有的格式一条都不发明**）：命中 ⇒ 去掉定界符的指纹串，否则 None。"""
    for rx in FINGERPRINT_CARRIERS:
        m = rx.search(text)
        if m:
            return m.group(1)
    return None


def ledger_ids(txt):
    """账件的**条目 id 集**：只认行首条目标记（`- \\`ID\\``）/ 表元（`| \\`ID\\` |`）—— 结构锚 ✓。
    散文里引用的号**不计**（它们不是条目 · 见 `ITEM_MARK_*_RE` 上方的注释）✗。"""
    ids = []
    for ln in txt.splitlines():
        for rx in (ITEM_MARK_BULLET_RE, ITEM_MARK_TABLE_RE):
            m = rx.match(ln)
            if m:
                ids.append(m.group(1))
                break
    return sorted(set(ids))


def ledger_stats(path):
    """一件 markdown 账的现读：行数 / 字节 / 最长行（行号+长度+前 60 字）/ 生成标记 / 指纹 / id 集。"""
    with open(path, encoding="utf-8") as f:
        txt = f.read()
    lines = txt.splitlines()
    mx_i = max(range(len(lines)), key=lambda i: len(lines[i])) if lines else -1
    h = HDR_RE.search(txt)
    return {
        "path": path, "lines": len(lines), "bytes": len(txt.encode()),
        "maxline_no": mx_i + 1 if mx_i >= 0 else 0,
        "maxline_len": len(lines[mx_i]) if mx_i >= 0 else 0,
        "maxline_head": dash(lines[mx_i], 60) if mx_i >= 0 else "",
        "generated": GENERATED_MARKER in txt,
        "fingerprint": find_fingerprint(txt),
        "ids": ledger_ids(txt),
        "header": {"total": int(h.group(1)), "open": int(h.group(2)), "closed": int(h.group(3))}
                  if h else None,
        "text": txt,
    }


def collect_ledgers(docs):
    """文档仓里的 `项目文档/<版>/缺口总账-<日期>.md` ⇒ (当期件, 历史件清单, 豁免清单)（按版本线+日期取最新）。"""
    base = os.path.join(docs, LEDGER_DIR)
    found = []
    if os.path.isdir(base):
        for ver in sorted(os.listdir(base)):
            vdir = os.path.join(base, ver)
            if not os.path.isdir(vdir):
                continue
            for root, _dirs, files in os.walk(vdir):
                for fn in files:
                    m = LEDGER_NAME_RE.match(fn)
                    if not m:
                        continue
                    p = os.path.join(root, fn)
                    rel = os.path.relpath(p, docs)
                    found.append({"path": p, "rel": rel, "ver": ver, "date": m.group(1),
                                  "exempt": any(seg in rel for seg in EXEMPT_PATH_SEGS)})
    #   当期件 = 未豁免里 (版本, 日期) 最大的一件；其余全是历史（只报告）
    def vkey(ver):
        """版本线排序键：`v2.5.13` ⇒ (2,5,13)（**不用字符串比** —— `v2.5.9` 那种会反序）。"""
        nums = re.findall(r"\d+", ver)
        return (tuple(int(x) for x in nums), ver)

    cand = [x for x in found if not x["exempt"]]
    cand.sort(key=lambda x: (vkey(x["ver"]), x["date"]))
    current = cand[-1] if cand else None
    history = [x for x in found if current is None or x["path"] != current["path"]]
    return current, history, [x for x in found if x["exempt"]]


def collect_ledgers_family(docs):
    """★ 件族（**新增** · 2026-09-27 拆件后）：索引件 / 页件族（三锁）/ 历史件 / 豁免件。

    三锁（页件必须三条同时满足才进族；不满足 ⇒ 落历史件**只报告**，绝不静默丢弃 ✗）：
      ① 名：`PAGE_NAME_RE` 枚举式匹配，且「面」∈ 六值闭集、页号两位；
      ② 日期：与索引件**归一化后**同日期（`2026-09-27` == `20260927`）；
      ③ 同目录：与索引件同一目录（`os.path.dirname` 逐字相等）。
    ★ 排序键补件名 + **索引件优先**：旧键 `(vkey(ver), date)` 不含件名 ⇒ 索引件与页件同键、
      `cand[-1]` 可能挑中**页件**（真缺陷）。本函数先分池，当期 = **索引池**里 (版本, 日期, 件名) 最大者。
    """
    base = os.path.join(docs, LEDGER_DIR)
    found = []
    if os.path.isdir(base):
        for ver in sorted(os.listdir(base)):
            vdir = os.path.join(base, ver)
            if not os.path.isdir(vdir):
                continue
            for root, _dirs, files in os.walk(vdir):
                for fn in sorted(files):
                    pm = PAGE_NAME_RE.match(fn)
                    m = INDEX_NAME_RE.match(fn)
                    if not pm and not m:
                        continue
                    p = os.path.join(root, fn)
                    rel = os.path.relpath(p, docs)
                    g = pm or m
                    found.append({"path": p, "rel": rel, "ver": ver,
                                  "dir": os.path.dirname(p),
                                  "kind": "page" if pm else "index",
                                  "date": g.group(1), "date_key": g.group(1).replace("-", ""),
                                  "face": pm.group(2) if pm else "",
                                  "page": pm.group(3) if pm else "",
                                  "exempt": any(seg in rel for seg in EXEMPT_PATH_SEGS)})

    def vkey(ver):
        nums = re.findall(r"\d+", ver)
        return (tuple(int(x) for x in nums), ver)

    live = [x for x in found if not x["exempt"]]
    idx_pool = [x for x in live if x["kind"] == "index"]
    idx_pool.sort(key=lambda x: (vkey(x["ver"]), x["date_key"], x["path"]))
    index = idx_pool[-1] if idx_pool else None
    pages, orphans = [], []
    if index is not None:
        for x in live:
            if x["kind"] != "page":
                continue
            ok = (x["face"] in IMPACT_CLOSED and x["date_key"] == index["date_key"]
                  and x["dir"] == index["dir"])
            (pages if ok else orphans).append(x)
        pages.sort(key=lambda x: (x["face"], x["page"], x["path"]))
    else:
        orphans = [x for x in live if x["kind"] == "page"]
    family = ([index] if index else []) + pages
    fam_paths = set(x["path"] for x in family)
    history = [x for x in found if x["path"] not in fam_paths]
    return {"index": index, "pages": pages, "family": family, "history": history,
            "exempt": [x for x in found if x["exempt"]], "orphan_pages": orphans}


def split_diff(want, got):
    """两个 id 集的差集（(仅 want 有, 仅 got 有)）。"""
    w, g = set(want), set(got)
    return sorted(w - g), sorted(g - w)


def name_list(ids, what):
    """逐条点名（超上限截断打计数）。"""
    if not ids:
        return []
    head = "、".join("`%s`" % i for i in ids[:NAME_LIMIT])
    if len(ids) > NAME_LIMIT:
        head += "（共 %d 条 · 此处只点前 %d 条）" % (len(ids), NAME_LIMIT)
    return ["%s %d 条：%s" % (what, len(ids), head)]


# ═════════════════════════════════════ 判据 ══════════════════════════════════════
def run(root, docs_root, source, json_out, export_cmd=None):
    root = os.path.abspath(root)
    problems, reds, reports = [], [], []
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根" % root)
        return 2
    if not os.path.isfile(EXPORT_BIN):
        print("⛔ 不给结论（rc=2）：导出器不在盘：`%s`（读不到导出面 ⇒ 判不了三方）" % EXPORT_BIN)
        return 2
    docs = os.path.abspath(docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档"))
    if not os.path.isdir(docs):
        print("⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）" % docs)
        return 2

    src = os.path.abspath(source or os.path.join(STATE_DIR_DEFAULT, SOURCE_NAME))

    # ── R5：真源现读（不可读 / 空转 ⇒ rc=2，不许当绿）────────────────────────────
    st, sread = read_source(src)
    if st != "ok":
        problems.append({"rule": "R5", "what": "真源%s" % {"absent": "不在盘", "unreadable": "读不到",
                                                         "badline": "有非 JSON 行",
                                                         "empty": "0 条（空转）"}[st], "read": sread})
    # ── R5：导出器现跑 ────────────────────────────────────────────────────────────
    exst, eread, eout = ("skip", {}, "")
    exp = None
    if not problems:
        cmd = export_cmd if export_cmd else [EXPORT_BIN] + list(EXPORT_ARGS)
        if export_cmd:
            env = dict(os.environ)
            env["ZERG_STATE_DIR"] = os.path.dirname(src)
            try:
                p = subprocess.run(list(cmd), cwd=DEFAULT_ROOT, env=env, capture_output=True,
                                   text=True, timeout=EXPORT_TIMEOUT)
                exst, eread, eout = ("ok" if p.returncode == 0 else "rc"), \
                    {"cmd": " ".join(cmd), "rc": p.returncode, "stderr": p.stderr[-400:]}, p.stdout
            except (OSError, subprocess.TimeoutExpired) as e:
                exst, eread, eout = "no_bin", {"cmd": " ".join(cmd), "msg": str(e)}, ""
        else:
            exst, eread, eout = run_export(os.path.dirname(src))
        if exst != "ok":
            problems.append({"rule": "R5", "what": "`zerg gap export` 跑不动（rc=%s）" % eread.get("rc"),
                             "read": {"cmd": eread.get("cmd"), "msg": eread.get("msg") or eread.get("stderr")}})
        else:
            exp = parse_export(eout)
            if exp is None:
                problems.append({"rule": "R5", "what": "导出产出解析不出（件头/件尾对不上）",
                                 "read": {"bytes": len(eout.encode())}})

    # ── 当期结转账件 + 历史件（R6 只报告）────────────────────────────────────────
    #   ★ 选件 = **件族**（索引池 / 页池分选 · 索引件优先为当期）
    fam = collect_ledgers_family(docs)
    current, pages, history, exempt = fam["index"], fam["pages"], fam["history"], fam["exempt"]
    led = None
    if current is None:
        problems.append({"rule": "R5", "what": "文档仓里一件当期**索引件** `缺口总账-<日期>.md` 都没有"
                         "（空转 ⇒ 判不了三方）",
                         "read": {"glob": "%s/<版>/缺口总账-<日期>.md" % LEDGER_DIR}})
    else:
        led = ledger_stats(current["path"])
        led["rel"] = current["rel"]
    fam_led = []  # ★ 族内逐件现读（R2/R3 逐件判的靶子）
    for x in fam["family"]:
        s = ledger_stats(x["path"])
        s["rel"], s["kind"] = x["rel"], x["kind"]
        fam_led.append((x, s))
    for x in history:
        s = ledger_stats(x["path"])
        reports.append({"rule": "R6", "what": "历史账（只报告 · 不入退码）%s" % x["rel"],
                        "read": {"lines": s["lines"], "bytes": s["bytes"],
                                 "maxline_no": s["maxline_no"], "maxline_len": s["maxline_len"],
                                 "exempt": x["exempt"]}})

    # ── R1 三方对拍 ──────────────────────────────────────────────────────────────
    r1 = {"src": {"recs": sread.get("recs"), "open": len(sread.get("open", [])),
                  "closed": len(sread.get("closed", []))},
          "export": None, "ledger": None, "diffs": []}
    if not problems:
        r1["export"] = {"total": exp["total"], "open": exp["open"], "closed": exp["closed"],
                        "no_decide": exp["no_decide"], "bullets": len(exp["bullets"]),
                        "foot_open": exp["foot_open"]}
        # R1a：真源 ↔ export 现跑（件头三数 + 逐条 id/state）
        if exp["total"] != sread["recs"]:
            reds.append({"rule": "R1a", "what": "条数不一致：真源 %d 条 vs 导出件头「真源条数 %d」"
                         % (sread["recs"], exp["total"]), "read": {}})
        if exp["open"] != exp["foot_open"]:
            reds.append({"rule": "R1a", "what": "导出件内自相矛盾：件头「未闭 %d」vs 件尾「未闭 %d 条」"
                         % (exp["open"], exp["foot_open"]), "read": {}})
        if exp["open"] + exp["closed"] != exp["total"]:
            reds.append({"rule": "R1a", "what": "导出件头算术不闭合：未闭 %d + 已闭 %d ≠ 条数 %d"
                         % (exp["open"], exp["closed"], exp["total"]), "read": {}})
        exp_ids = [b["id"] for b in exp["bullets"]]
        miss, extra = split_diff(sread["open"], exp_ids)
        if miss:
            reds.append({"rule": "R1a", "what": "真源未闭里有、导出正文里**缺**", "read": {}})
            reds[-1]["name"] = name_list(miss, "缺")
        if extra:
            reds.append({"rule": "R1a", "what": "导出正文里有、真源未闭里**多**", "read": {}})
            reds[-1]["name"] = name_list(extra, "多")
        for b in exp["bullets"]:
            want = sread["state_of"].get(b["id"])
            if want is not None and want != b["state"]:
                reds.append({"rule": "R1a", "what": "状态不一致 `%s`：真源「%s」vs 导出「%s」"
                             % (b["id"], want, b["state"]), "read": {}})
        dup = sread["dup_ids"]
        if dup:
            reds.append({"rule": "R1a", "what": "真源 id 重复（主键必须唯一）", "read": {},
                         "name": name_list(dup, "重号")})
        if exp["no_decide"] > exp["closed"]:
            reds.append({"rule": "R1a", "what": "「不做」%d > 已闭 %d（不可能）"
                         % (exp["no_decide"], exp["closed"]), "read": {}})
        # R1b：真源 ↔ 当期结转账件（第三方腿）
        fam_ids = sorted(set(i for _x, _s in fam_led for i in _s["ids"]))
        page_ids = [(_s["rel"], _s["ids"]) for _x, _s in fam_led if _x["kind"] == "page"]
        ldiff_miss, ldiff_extra = split_diff(sread["open"], fam_ids)
        r1["ledger"] = {"rel": led["rel"], "lines": led["lines"], "bytes": led["bytes"],
                        "generated": led["generated"], "ids": len(led["ids"]),
                        "header": led["header"], "pages": len(pages),
                        "family_ids": len(fam_ids)}
        if not any(_mk in led["text"] for _mk in GENERATED_MARKERS):
            reds.append({"rule": "R1b", "what": "**生成件未接管**：当期结转账件 `%s` 里没有生成标记逐字句"
                         "「%s」（或**同一句去掉收尾反引号**的前缀形态）⇒ 它仍是手写账（第三方腿不是真源视图）"
                         % (led["rel"], GENERATED_MARKER),
                         "read": {}})
        elif led["header"]:
            if led["header"]["open"] != exp["open"]:
                reds.append({"rule": "R1b", "what": "账件件头「未闭 %d」vs 导出现跑「未闭 %d」不一致"
                             % (led["header"]["open"], exp["open"]), "read": {}})
            if led["header"]["total"] != sread["recs"]:
                reds.append({"rule": "R1b", "what": "账件件头「真源条数 %d」vs 真源现读 %d 条不一致"
                             % (led["header"]["total"], sread["recs"]), "read": {}})
        if ldiff_miss:
            reds.append({"rule": "R1b", "what": "真源未闭里有、账件里**缺**", "read": {}})
            reds[-1]["name"] = name_list(ldiff_miss, "缺")
        if ldiff_extra:
            reds.append({"rule": "R1b", "what": "账件里有、真源未闭里**多**（含已闭/历史号）", "read": {}})
            reds[-1]["name"] = name_list(ldiff_extra, "多")
        #   ★ R1c：页件两两 id 不交 —— 同一号在两页各点一次时【并集仍然对】⇒ 必须独立判
        seen, dup_pairs = {}, []
        for _rel, _ids in page_ids:
            for _i in _ids:
                if _i in seen:
                    dup_pairs.append((_i, seen[_i], _rel))
                seen.setdefault(_i, _rel)
        if dup_pairs:
            reds.append({"rule": "R1c", "what": "页件两两 id 有交集（同一号在多页重复 ⇒ 并集会把重页完全掩盖）：%d 处"
                         % len(dup_pairs), "read": {"first": [list(x) for x in dup_pairs[:3]]}})
            reds[-1]["name"] = name_list(sorted(set(x[0] for x in dup_pairs)), "重号")
        #   ★ R1d：Σ页条目 == 索引件头「未闭」—— 漏页时并集少号、重页时并集掩盖 ⇒ 独立的一格
        if pages and led["header"]:
            _ptot = sum(len(v) for _r, v in page_ids)
            if _ptot != led["header"]["open"]:
                reds.append({"rule": "R1d", "what": "Σ页条目 %d ≠ 索引件头「未闭 %d」（漏页 / 重页 / 多算）"
                             % (_ptot, led["header"]["open"]),
                             "read": {"page_entries": _ptot, "index_open": led["header"]["open"]}})

    # ── R2 体积 ─────────────────────────────────────────────────────────────────
    #   ★ R2 体积：**逐件**（索引件 + 每一页件各判一次）
    for _x, _s in fam_led:
        over = []
        if _s["lines"] > MAX_LINES:
            over.append("行数 %d > 上限 %d" % (_s["lines"], MAX_LINES))
        if _s["bytes"] > MAX_BYTES:
            over.append("字节 %d > 上限 %d" % (_s["bytes"], MAX_BYTES))
        if over:
            reds.append({"rule": "R2", "what": "结转账件体积超限：%s（件 `%s`）" % (" · ".join(over), _s["rel"]),
                         "read": {"lines": _s["lines"], "bytes": _s["bytes"]}})

    # ── R3 单行 ─────────────────────────────────────────────────────────────────
    #   ★ R3 单行：**逐件**
    for _x, _s in fam_led:
        if _s["maxline_len"] > MAX_LINE_CHARS:
            reds.append({"rule": "R3", "what": "结转账件最长行 %d 字符 > 上限 %d（第 %d 行 · 前 60 字：%s）（件 `%s`）"
                         % (_s["maxline_len"], MAX_LINE_CHARS, _s["maxline_no"], _s["maxline_head"],
                            _s["rel"]),
                         "read": {"line": _s["maxline_no"], "len": _s["maxline_len"]}})

    # ── R4 写保护（指纹 · 有则判红 / 无则只报告）──────────────────────────────────
    r4 = {"export_fp": exp["fingerprint"] if exp else None,
          "ledger_fp": led["fingerprint"] if led else None,
          "ledger_generated": bool(led and led["generated"]), "verdict": ""}
    if led is not None and exp is not None:
        if not exp["fingerprint"] and not led["fingerprint"]:
            r4["verdict"] = ("无指纹 ⇒ 无法判手改（R-05 未落地 · **只报告**）：`gap export` 今天的人面产出里"
                             "**没有**可对拍指纹，本门不发明格式 ⇒ 不入退码")
        elif exp["fingerprint"] and not led["fingerprint"] and led["generated"]:
            r4["verdict"] = "生成件缺指纹（导出器有指纹而账件里没有）⇒ 判红"
            reds.append({"rule": "R4", "what": "生成件缺指纹（导出器现跑带指纹 `%s`）"
                         % exp["fingerprint"][:16], "read": {}})
        elif exp["fingerprint"] and not led["fingerprint"]:
            r4["verdict"] = "账件不是生成件 ⇒ 无指纹可判（只报告）"
        elif exp["fingerprint"] != led["fingerprint"]:
            r4["verdict"] = "指纹不符 ⇒ 判红（手改嫌疑）"
            reds.append({"rule": "R4", "what": "指纹不符：导出现跑 `%s` vs 账件 `%s`"
                         % (exp["fingerprint"][:16], led["fingerprint"][:16]), "read": {}})
        else:
            r4["verdict"] = "指纹逐字一致（`%s…`）" % exp["fingerprint"][:16]
    reports.append({"rule": "R4", "what": "写保护现读：%s" % (r4["verdict"] or "判不了（缺件）"),
                    "read": {"export_fp": r4["export_fp"], "ledger_fp": r4["ledger_fp"]}})
    if sread.get("out_of_set"):
        reports.append({"rule": "-", "what": "真源里有值不在闭集（导出器会自行判红；本门照实数报告）",
                        "read": {"n": len(sread["out_of_set"]), "first": sread["out_of_set"][:3]}})

    # ── 输出 ────────────────────────────────────────────────────────────────────
    if json_out:
        doc = {
            "schema": "zerg/v1", "kind": "GapLedgerView", "tool": VERSION,
            "items": [{"rule": r, "state": "red"} for r in sorted({x["rule"] for x in reds})] +
                     [{"rule": r, "state": "problem"} for r in sorted({x["rule"] for x in problems})],
            "meta": {"source": src, "docs_root": docs, "root": root, "export_bin": EXPORT_BIN,
                     "limits": {"max_lines": MAX_LINES, "max_bytes": MAX_BYTES,
                                "max_line_chars": MAX_LINE_CHARS},
                     "export": r1["export"], "ledger": r1["ledger"],
                     "history_ledgers": len(history), "exempt_ledgers": len(exempt),
                     "ledger_family": {"index": led["rel"] if led else None,
                                       "pages": len(pages),
                                       "family_ids": len(fam_ids) if not problems else None,
                                       "orphan_pages": len(fam["orphan_pages"])},
                     "fingerprint": r4, "generated_taken_over": bool(led and led["generated"])},
            "reds": reds, "problems": problems, "reports": reports, "warnings": [],
            "truncated": False,
        }
        print(json.dumps(doc, ensure_ascii=False))
        if problems:
            return 2
        return 1 if reds else 0

    print("── 缺口账「视图分离」三方对拍 + 体积/单行 + 指纹写保护（%s · 设计稿 v1.3 §0 ①）──" % VERSION)
    print("  真源    : %s" % src)
    print("  导出器  : %s %s（**现跑**）" % (EXPORT_BIN, " ".join(EXPORT_ARGS)))
    print("  文档仓  : %s" % docs)
    print("  当期索引件: %s" % (led["rel"] if led else "（一件都没有）"))
    print("  页件    : %d 件（历史件 %d · 豁免件 %d · 非族页件 %d）"
          % (len(pages), len(history), len(exempt), len(fam["orphan_pages"])))
    if not problems:
        print("  ── R1 三方对拍 ──")
        print("     真源    : %d 条 · 未闭 %d · 已闭 %d" % (sread["recs"], len(sread["open"]), len(sread["closed"])))
        print("     导出现跑: 件头 %d 条 · 未闭 %d · 已闭 %d（不做 %d）· 正文 %d 条 · 件尾未闭 %d"
              % (exp["total"], exp["open"], exp["closed"], exp["no_decide"], len(exp["bullets"]), exp["foot_open"]))
        # ★ 2026-09-27：这一格（人面）与 `:503` 的判定**同一枚判据** —— 都走 `GENERATED_MARKERS`
        #   （逐字句 + 「同一句去掉收尾反引号」两形态）。修前这里用 `led["generated"]`
        #   （`GENERATED_MARKER` 单数 · 只认收尾反引号那一形态）⇒ 门真绿时人面反而打
        #   「生成件=**否**✗」，人看会被引到错结论（判据与人面**分离**）。
        print("     账件    : %s（生成件=%s）· 行 %d · 字节 %d · id %d 个"
              % (led["rel"],
                  "是" if any(_mk in led["text"] for _mk in GENERATED_MARKERS) else "**否**✗",
                  led["lines"], led["bytes"], len(led["ids"])))
        print("     件族    : 索引 1 + 页 %d ⇒ 族内 id 并集 %d 个 · Σ页条目 %d"
              % (len(pages), len(fam_ids),
                 sum(len(_s["ids"]) for _x, _s in fam_led if _x["kind"] == "page")))
        print("  ── R2 体积（逐件）──")
        for _x, _s in fam_led:
            print("     [%s] 行数 %d / 上限 %d %s · 字节 %d / 上限 %d %s"
                  % (_s["rel"], _s["lines"], MAX_LINES, "✓" if _s["lines"] <= MAX_LINES else "**超**✗",
                     _s["bytes"], MAX_BYTES, "✓" if _s["bytes"] <= MAX_BYTES else "**超**✗"))
        print("  ── R3 单行（逐件）──")
        for _x, _s in fam_led:
            print("     [%s] 最长行 %d 字符（第 %d 行）/ 上限 %d %s"
                  % (_s["rel"], _s["maxline_len"], _s["maxline_no"], MAX_LINE_CHARS,
                     "✓" if _s["maxline_len"] <= MAX_LINE_CHARS else "**超**✗"))
        print("  ── R4 写保护 ──")
        print("     导出产出指纹: %s · 账件指纹: %s" % (r4["export_fp"] or "**无**",
                                                     r4["ledger_fp"] or "**无**"))
        print("     结论: %s" % r4["verdict"])
    if reports:
        print("  ── 只报告（**不入退码** ✗）──")
        for r in reports:
            print("     · [%s] %s" % (r["rule"], r["what"]))

    if problems:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · [%s] %s" % (p["rule"], p["what"]))
            rd = p["read"]
            if rd.get("path"):
                print("       盘面：%s" % rd["path"])
            if rd.get("msg"):
                print("       原样：%s" % dash(str(rd["msg"]), 120))
        return 2
    if reds:
        print("✗ 判红 %d 处（rc=1 · 逐条点名）：" % len(reds))
        for r in reds:
            print("   · [%s] %s" % (r["rule"], r["what"]))
            for n in r.get("name", []):
                print("       %s" % n)
        print("⇒ 处置（**不是**改本门）：① 三方对拍红 ⇒ 那是 ③（`zerg gap export` 接管结账单生成）的活；"
              "② 体积/单行红 ⇒ 生成件由 export 产出（它已自带 ≤200 单行纪律）")
        return 1
    print("⇒ 三方一致 + 体积/单行合规（rc=0）· 件 `%s`" % (led["rel"] if led else "-"))
    print("   ★ 本件只**读**：真源、账件一个字未改（写它们 = 别的笔的活）")
    return 0


def list_rules():
    print("── %s 判据 / 登记面 / 豁免 / 上限 ──" % VERSION)
    print("  R1 三方对拍（判红）：真源(jsonl 逐行 id+state) ↔ `zerg gap export` 现跑 ↔ 当期结转账件")
    print("  R2 体积（判红）：当期账件 ≤ %d 行 · ≤ %d 字节" % (MAX_LINES, MAX_BYTES))
    print("  R3 单行（判红）：当期账件最长行 ≤ %d 字符（打行号+长度）" % MAX_LINE_CHARS)
    print("  R4 写保护（有指纹则判红 · 无则只报告）：导出产出指纹 ⟷ 生成件指纹 逐字比")
    print("  R5 缺件/空转（rc=2 · 不许当绿）：真源不可读/0 条 · export 跑不动 · 产出一件账都没有")
    print("  R6 旧版账（只报告）：非当期 `%s/<版>/%s` ⇒ 列行数/字节/最长行，不入退码"
          % (LEDGER_DIR, "缺口总账-<日期>.md"))
    print("  登记面：真源 `%s`（状态目录缺省 `%s`）" % (SOURCE_NAME, STATE_DIR_DEFAULT))
    print("     态闭集: 未闭 %s ｜ 已闭 %s" % ("/".join(OPEN_STATES), "/".join(CLOSED_STATES)))
    print("     面闭集: %s" % " / ".join(IMPACT_CLOSED))
    print("     prio  : %s" % " / ".join(PRIO_CLOSED))
    print("     生成标记（逐字句 · 现读 export 人面第 3 行）: %s" % GENERATED_MARKER)
    print("  豁免（**只此一条**）：路径段命中 %s ⇒ 历史件只报告" % " / ".join(EXEMPT_PATH_SEGS))
    print("  指纹载体（现读，未发明格式）：%s" % " ｜ ".join(rx.pattern for rx in FINGERPRINT_CARRIERS))
    print("  导出器（只认本门隔壁那一个）：%s %s" % (EXPORT_BIN, " ".join(EXPORT_ARGS)))
    print("  退码：0 三方一致且合规 · 1 不一致/超限/指纹不符 · 2 不给结论（缺件/空转/导出跑不动/自检不过）")
    return 0


# ═════════════════════════════════════ 自检（成对负控 · 合成夹具 · 不碰真源/真账）══════════════════
FIXTURE_EXPORT = r'''# -*- coding: utf-8 -*-
"""合成导出器（自检专用 · 读 <ZERG_STATE_DIR>/export-fixture.md 并照退码文件退出）。"""
import os, sys
d = os.environ.get("ZERG_STATE_DIR", "")
rcf = os.path.join(d, "export-rc")
rc = int(open(rcf).read().strip()) if os.path.exists(rcf) else 0
p = os.path.join(d, "export-fixture.md")
if os.path.exists(p):
    sys.stdout.write(open(p, encoding="utf-8").read())
if rc:
    sys.stderr.write("fixture export 故意失败 rc=%d\n" % rc)
sys.exit(rc)
'''


def mk_export_md(items, total, closed, no_decide, line_max=200):
    """照 `zerg gap export` 人面形状造一份导出片段（件头 + 逐面小节 + 件尾）。"""
    per = collections.Counter(i["impact"] for i in items)
    out = ["## 缺口真源导出（**只列未闭**） · 2026-09-26", "",
           "> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— **不许手改** ✗（手改会被门禁判红）。",
           "> 真源条数 **%d** · 未闭 **%d** · 已闭 **%d**（其中「不做」**%d**）" % (total, len(items), closed, no_decide),
           "> 按面：%s" % " · ".join("%s %d" % (f, per[f]) for f in IMPACT_CLOSED), ""]
    for f in IMPACT_CLOSED:
        grp = [i for i in items if i["impact"] == f]
        if not grp:
            continue
        out.append("### %s（%d 条）" % (f, len(grp)))
        out.append("")
        for i in grp:
            ln = "- `%s` %s · **%s** · %s · 想要 `w` · 判据 `zerg task ls`" % (i["id"], i["prio"], i["state"], i["symptom"])
            out.append(ln if len(ln) <= line_max else ln[:line_max - 1] + "…")
        out.append("")
    out.append("（本片段由 CLI 生成 · 未闭 %d 条 · 单行上限 %d 字符）" % (len(items), line_max))
    return "\n".join(out) + "\n"


def rec(i, state="仍缺", impact="命令面", prio="P0", symptom="s"):
    return {"id": i, "fp": "fp" + i[-2:], "handmade": "", "impact": impact, "prio": prio,
            "state": state, "symptom": symptom, "want_action": "w", "want_family": "w",
            "verify_cmd": "zerg task ls", "repro_cmd": "", "found_at": "2026-09-26T00:00:00+08:00",
            "last_verified_at": "", "solved_at": "", "solved_evidence": ""}


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="glv-selftest-")
    fixture = os.path.join(tmp, "fixture-export.py")
    with open(fixture, "w", encoding="utf-8") as f:
        f.write(FIXTURE_EXPORT)
    n_case = [0]

    def mk(src_recs, exp_md, led_lines, history=(), export_rc=0, led_name="缺口总账-20260926.md"):
        n_case[0] += 1
        base = os.path.join(tmp, "case-%02d" % n_case[0])
        repo = os.path.join(base, "repo")
        os.makedirs(os.path.join(repo, "scripts", "gates"), exist_ok=True)
        state = os.path.join(base, "state")
        os.makedirs(state, exist_ok=True)
        with open(os.path.join(state, SOURCE_NAME), "w", encoding="utf-8") as f:
            for r in src_recs:
                f.write(json.dumps(r, ensure_ascii=False) + "\n")
        with open(os.path.join(state, "export-rc"), "w", encoding="utf-8") as f:
            f.write(str(export_rc))
        if exp_md is not None:
            with open(os.path.join(state, "export-fixture.md"), "w", encoding="utf-8") as f:
                f.write(exp_md)
        docs = os.path.join(base, "Zerg-内部文档")
        vdir = os.path.join(docs, LEDGER_DIR, "vX")
        os.makedirs(vdir, exist_ok=True)
        if led_lines is not None:
            with open(os.path.join(vdir, led_name), "w", encoding="utf-8") as f:
                f.write("\n".join(led_lines) + "\n")
        for rel, lines in history:
            hp = os.path.join(docs, LEDGER_DIR, rel)
            os.makedirs(os.path.dirname(hp), exist_ok=True)
            with open(hp, "w", encoding="utf-8") as f:
                f.write("\n".join(lines) + "\n")
        return repo, docs, os.path.join(state, SOURCE_NAME)

    def case(name, want_rc, got_rc, extra_ok=True, note=""):
        nonlocal ok
        good = (want_rc == got_rc) and extra_ok
        ok = ok and good
        print("  %s %s（rc=%s · 期望 %s）%s" % ("✓" if good else "✗", name, got_rc, want_rc, note))

    def cap(*a, **kw):
        import contextlib
        import io
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc = run(*a, **kw)
        return rc, buf.getvalue()

    def led_of(items, marker=True, extra=()):
        """生成件形态的账件（= 导出片段原样落盘）。"""
        body = mk_export_md(items, len(items) + 1, 1, 0).splitlines()
        line2 = "> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— " \
                "**不许手改** ✗（手改会被门禁判红）。"
        body[2] = line2 if marker else "> 说明：本件手写。"
        return list(body) + list(extra)

    A = rec("GAP-20260926-01", symptom="甲")
    B = rec("GAP-20260926-02", state="已解", symptom="乙")
    exp_ab = mk_export_md([A], 2, 1, 0)

    # ① 正控：三方一致 + 体积/单行合规 ⇒ rc=0
    r, d, s = mk([A, B], exp_ab, led_of([A]))
    rc, out = cap(r, d, s, False, [sys.executable, fixture])
    case("① 正控：三方一致（生成件接管）⇒ rc=0", 0, rc, "三方一致" in out)
    case("⑩ R4 只报告：无指纹 ⇒ 不入退码", True, ("无指纹 ⇒ 无法判手改" in out and rc == 0))

    # ② 负控 R1a：真源多一条（导出缺）⇒ rc=1 且点名
    C = rec("GAP-20260926-03", symptom="丙")
    r2, d2, s2 = mk([A, C, B], exp_ab, led_of([A]))
    rc, out = cap(r2, d2, s2, False, [sys.executable, fixture])
    case("② 负控 R1a：真源未闭 2 条 vs 导出 1 条 ⇒ rc=1", 1, rc)
    case("②b 点名到「缺哪个」（`GAP-20260926-03`）", True, ("`GAP-20260926-03`" in out and "[R1a]" in out))

    # ②c 真话负控：同一夹具只把导出补齐 ⇒ 回绿
    exp_abc = mk_export_md([A, C], 3, 1, 0)
    r2c, d2c, s2c = mk([A, C, B], exp_abc, led_of([A, C]))
    rc, out = cap(r2c, d2c, s2c, False, [sys.executable, fixture])
    case("②c 真话负控（只把导出补齐）⇒ 回绿", 0, rc, "三方一致" in out)

    # ③ 负控 R1a：导出多一条 ⇒ rc=1 点名「多」
    r3, d3, s3 = mk([A, B], mk_export_md([A, C], 3, 1, 0), led_of([A, C]))
    rc, out = cap(r3, d3, s3, False, [sys.executable, fixture])
    case("③ 负控 R1a：导出正文多一条 ⇒ rc=1 点名「多」", 1, rc, ("多" in out and "`GAP-20260926-03`" in out))

    # ④ 负控 R1a：同一个 id 状态不一致 ⇒ rc=1
    exp_st = mk_export_md([dict(A, state="已派")], 2, 1, 0)
    r4, d4, s4 = mk([A, B], exp_st, led_of([dict(A, state="已派")]))
    rc, out = cap(r4, d4, s4, False, [sys.executable, fixture])
    case("④ 负控 R1a：状态不一致（仍缺 vs 已派）⇒ rc=1", 1, rc, "状态不一致" in out)

    # ⑤ 负控 R1b：账件不是生成件（手写）⇒ rc=1「生成件未接管」
    r5, d5, s5 = mk([A, B], exp_ab, led_of([A], marker=False))
    rc, out = cap(r5, d5, s5, False, [sys.executable, fixture])
    case("⑤ 负控 R1b：账件缺生成标记 ⇒ rc=1「生成件未接管」", 1, rc, "生成件未接管" in out)

    # ⑥ 负控 R2：行数 301 ⇒ rc=1（打实际值）
    long_led = led_of([A]) + ["<!-- pad %d -->" % i for i in range(305)]
    r6, d6, s6 = mk([A, B], exp_ab, long_led)
    rc, out = cap(r6, d6, s6, False, [sys.executable, fixture])
    case("⑥ 负控 R2：行数 300+ ⇒ rc=1（逐字打实际值）", 1, rc,
         ("[R2]" in out and "行数" in out and "上限 300" in out))

    # ⑥b 负控 R2：字节 > 60KB（行数不动）⇒ rc=1
    fat = led_of([A]) + ["<!-- %s -->" % ("x" * 120) for _ in range(520)]
    r6b, d6b, s6b = mk([A, B], exp_ab, fat)
    rc, out = cap(r6b, d6b, s6b, False, [sys.executable, fixture])
    case("⑥b 负控 R2：字节 60KB+ ⇒ rc=1", 1, rc, ("上限 %d" % MAX_BYTES) in out)

    # ⑦ 负控 R3：最长行 210 字符 ⇒ rc=1（打行号与长度）
    r3l = led_of([A]) + ["<!-- " + ("y" * 201) + " -->"]
    r7, d7, s7 = mk([A, B], exp_ab, r3l)
    rc, out = cap(r7, d7, s7, False, [sys.executable, fixture])
    case("⑦ 负控 R3：最长行 210 字符 ⇒ rc=1（行号+长度）", 1, rc,
         ("[R3]" in out and "最长行 210 字符" in out and "第 12 行" in out))

    # ⑧ 负控 R4：导出器带指纹而生成件缺 ⇒ rc=1；指纹不符 ⇒ rc=1；一致 ⇒ rc=0
    exp_fp = exp_ab + "\n> 生成指纹 sha256:deadbeefdeadbeef\n"
    r8, d8, s8 = mk([A, B], exp_fp, led_of([A]))
    rc, out = cap(r8, d8, s8, False, [sys.executable, fixture])
    case("⑧ 负控 R4：导出带指纹而生成件缺 ⇒ rc=1", 1, rc, "[R4]" in out)
    r8b, d8b, s8b = mk([A, B], exp_fp, led_of([A]) + ["> 生成指纹 sha256:cafebabe"])
    rc, out = cap(r8b, d8b, s8b, False, [sys.executable, fixture])
    case("⑧b 负控 R4：指纹不符 ⇒ rc=1", 1, rc, "指纹不符" in out)
    r8c, d8c, s8c = mk([A, B], exp_fp, led_of([A]) + ["> 生成指纹 sha256:deadbeefdeadbeef"])
    rc, out = cap(r8c, d8c, s8c, False, [sys.executable, fixture])
    case("⑧c 正控 R4：指纹逐字一致 ⇒ rc=0", 0, rc, "指纹逐字一致" in out)

    # ⑨ 负控 R5：真源不在盘 / 空转 / 导出跑不动 / 一件账都没有 ⇒ 一律 rc=2
    r9, d9, s9 = mk([A], exp_ab, led_of([A]))
    os.remove(s9)
    rc, out = cap(r9, d9, s9, False, [sys.executable, fixture])
    case("⑨ 负控 R5：真源不在盘 ⇒ rc=2（不拿读不到当没问题）", 2, rc, "不给结论" in out)
    r9b, d9b, s9b = mk([], exp_ab, led_of([A]))
    rc, out = cap(r9b, d9b, s9b, False, [sys.executable, fixture])
    case("⑨b 负控 R5：真源 0 条（空转）⇒ rc=2", 2, rc, "不给结论" in out)
    r9c, d9c, s9c = mk([A], exp_ab, led_of([A]), export_rc=3)
    rc, out = cap(r9c, d9c, s9c, False, [sys.executable, fixture])
    case("⑨c 负控 R5：导出跑不动（rc=3）⇒ rc=2（不许当绿）", 2, rc, "不给结论" in out)
    r9d, d9d, s9d = mk([A], exp_ab, None)
    rc, out = cap(r9d, d9d, s9d, False, [sys.executable, fixture])
    case("⑨d 负控 R5：文档仓一件账都没有 ⇒ rc=2", 2, rc, "不给结论" in out)
    r9e, d9e, s9e = mk([A], "这不是导出片段", led_of([A]))
    rc, out = cap(r9e, d9e, s9e, False, [sys.executable, fixture])
    case("⑨e 负控 R5：产出解析不出 ⇒ rc=2", 2, rc, "解析不出" in out)

    # ⑪ 只报告：历史账超限也不入退码（当期件合规）
    hist = [("vOld/缺口总账-20260101.md", ["# 旧账"] + ["| %d |" % i for i in range(500)])]
    r11, d11, s11 = mk([A, B], exp_ab, led_of([A]), history=hist)
    rc, out = cap(r11, d11, s11, False, [sys.executable, fixture])
    case("⑪ 只报告 R6：历史账 500 行 ⇒ 仍 rc=0（不入退码）", 0, rc, "[R6]" in out)

    # ⑫ 正控 R1b：正文散文里引用的号**不是条目** ⇒ 不计（真源没有也不报「多」· 改前必假红 ✗）
    prose_led = led_of([A], extra=["- `%s` P1 · **仍缺** · 与 `Q-999` 同根（散文引用 · 非条目）" % A["id"]])
    r12, d12, s12 = mk([A, B], exp_ab, prose_led)
    rc, out = cap(r12, d12, s12, False, [sys.executable, fixture])
    case("⑫ 正控 R1b：散文引用的 `Q-999` 不算条目 ⇒ rc=0", 0, rc,
         ("三方一致" in out and "Q-999" not in out))

    # ⑬ 负控 R1b：**条目标记**形式的 `Q-999`（真源没有）⇒ 必须判红「多」（不只认散文放宽 ✗）
    mark_led = led_of([A], extra=["- `Q-999` P1 · **仍缺** · 作为条目标记出现（真源无此号）"])
    r13, d13, s13 = mk([A, B], exp_ab, mark_led)
    rc, out = cap(r13, d13, s13, False, [sys.executable, fixture])
    case("⑬ 负控 R1b：条目标记形式的 `Q-999` ⇒ rc=1 点名「多」", 1, rc,
         ("[R1b]" in out and "`Q-999`" in out and "多" in out))


    # ── ★ 件族（索引件 + 页件）：2026-09-27 拆件后**新增**的格（老 13 格一字未动）────────
    def mk_family(src_recs, exp_md, index_lines, page_files, export_rc=0):
        """件族夹具（**新函数** · 不动 mk）：索引件 + 页件同落 `项目文档/vX/`。"""
        n_case[0] += 1
        base = os.path.join(tmp, "case-%02d" % n_case[0])
        repo = os.path.join(base, "repo")
        os.makedirs(os.path.join(repo, "scripts", "gates"), exist_ok=True)
        state = os.path.join(base, "state")
        os.makedirs(state, exist_ok=True)
        with open(os.path.join(state, SOURCE_NAME), "w", encoding="utf-8") as f:
            for r in src_recs:
                f.write(json.dumps(r, ensure_ascii=False) + "\n")
        with open(os.path.join(state, "export-rc"), "w", encoding="utf-8") as f:
            f.write(str(export_rc))
        with open(os.path.join(state, "export-fixture.md"), "w", encoding="utf-8") as f:
            f.write(exp_md)
        docs = os.path.join(base, "Zerg-内部文档")
        vdir = os.path.join(docs, LEDGER_DIR, "vX")
        os.makedirs(vdir, exist_ok=True)
        with open(os.path.join(vdir, "缺口总账-2026-09-27.md"), "w", encoding="utf-8") as f:
            f.write("\n".join(index_lines) + "\n")
        for fn, lines in page_files:
            with open(os.path.join(vdir, fn), "w", encoding="utf-8") as f:
                f.write("\n".join(lines) + "\n")
        return repo, docs, os.path.join(state, SOURCE_NAME)

    def index_md(items, pages, marker=True):
        """索引件正文：件头（HDR 认得出未闭数）+ 逐页清单。"""
        per = collections.Counter(i["impact"] for i in items)
        head = ["> 生成件：由 `zerg gap export` 从真源 `/x/zerg-cli-gaps.jsonl` 生成 —— "
                "**不许手改** ✗（手改会被门禁判红）。"] if marker else ["> 说明：本件手写。"]
        out = ["## 缺口真源导出（**只列未闭**） · 2026-09-27", ""] + head + [
            "> 真源条数 **%d** · 未闭 **%d** · 已闭 **%d**（其中「不做」**%d**）"
            % (len(items) + 1, len(items), 1, 0),
            "> 按面：%s" % " · ".join("%s %d" % (f, per[f]) for f in IMPACT_CLOSED),
            "> 拆件：一级 = 面 · 二级 = 每页 120 条 · 页件名 `缺口总账-<日期>-<面>-pNN.md`", ""]
        for face, cnt in pages:
            out.append("### %s（%d 条）" % (face, cnt))
            out.append("")
            out.append("- 第 1 页：`缺口总账-2026-09-27-%s-p01.md` · 面内 1–%d 条" % (face, cnt))
        return out

    def page_md(items, face, pno=1, total=None):
        """页件正文（真产出形状：页头逐字 + 条目行）。"""
        out = ["# %s · 缺口总账 2026-09-27" % face, "",
               "> 第 %d 页·本页 %d 条·面内 1–%d 条·本面共 %d 条"
               % (pno, len(items), len(items), total if total is not None else len(items)),
               "> 真源 = `/x/zerg-cli-gaps.jsonl` · 面内顺序 = `(prioRank, id)` 稳定序 · "
               "单行上限 200 字符 · 生成件**不许手改** ✗", ""]
        for i in items:
            out.append("- `%s` %s · **%s** · %s" % (i["id"], i["prio"], i["state"], i["symptom"]))
        return out

    E = rec("GAP-20260927-01", symptom="戊")
    F = rec("GAP-20260927-02", symptom="己")
    G = rec("GAP-20260927-03", impact="门禁面", symptom="庚")
    fam_items = [E, F, G]
    exp_efg = mk_export_md(fam_items, 4, 1, 0)
    idx_lines = index_md(fam_items, [("命令面", 2), ("门禁面", 1)])
    p_cmd = page_md([E, F], "命令面")
    p_doc = page_md([G], "门禁面")

    # ⑭ 正控 · 件族：索引 + 2 页（并集 == 真源未闭 · Σ页 == 索引件头未闭）⇒ rc=0
    r14, d14, s14 = mk_family(fam_items + [B], exp_efg, idx_lines,
                              [("缺口总账-2026-09-27-命令面-p01.md", p_cmd),
                               ("缺口总账-2026-09-27-门禁面-p01.md", p_doc)])
    rc, out = cap(r14, d14, s14, False, [sys.executable, fixture])
    case("⑭ 正控 · 件族（索引+2 页 · 并集一致）⇒ rc=0", 0, rc, "三方一致" in out)

    # ⑮ 负控 · 漏页：p02 缺 ⇒ Σ页 2 ≠ 索引件头 3（并集也少号）⇒ rc=1
    r15, d15, s15 = mk_family(fam_items + [B], exp_efg, idx_lines,
                              [("缺口总账-2026-09-27-命令面-p01.md", page_md([E], "命令面", total=2)),
                               ("缺口总账-2026-09-27-门禁面-p01.md", p_doc)])
    rc, out = cap(r15, d15, s15, False, [sys.executable, fixture])
    case("⑮ 负控 · 漏页（Σ页 2 ≠ 索引件头未闭 3）⇒ rc=1 + [R1d]", 1, rc,
         ("[R1d]" in out and "[R1b]" in out))

    # ⑯ 负控 · 重页：F 同时出现在两页 ⇒ 并集仍对（掩盖）⇒ 必须由 R1c 抓到 ⇒ rc=1
    r16, d16, s16 = mk_family(fam_items + [B], exp_efg, idx_lines,
                              [("缺口总账-2026-09-27-命令面-p01.md", p_cmd),
                               ("缺口总账-2026-09-27-命令面-p02.md", page_md([F, G], "命令面", pno=2))])
    rc, out = cap(r16, d16, s16, False, [sys.executable, fixture])
    case("⑯ 负控 · 重页（并集仍一致 ⇒ 靠 R1c 抓）⇒ rc=1 + [R1c]", 1, rc,
         ("[R1c]" in out and "[R1b]" not in out and "[R1d]" in out))

    # ⑰ 三锁 · 日期锁：异日期的页件**不进族**（落历史 · 只报告）⇒ rc=0
    r17, d17, s17 = mk_family(fam_items + [B], exp_efg, idx_lines,
                              [("缺口总账-2026-09-27-命令面-p01.md", p_cmd),
                               ("缺口总账-2026-09-27-门禁面-p01.md", p_doc),
                               ("缺口总账-2026-09-26-命令面-p01.md", page_md([E, F], "命令面"))])
    rc, out = cap(r17, d17, s17, False, [sys.executable, fixture])
    case("⑰ 三锁（日期锁）：异日期页件不入族 · 只报告 ⇒ rc=0 + [R6]", 0, rc,
         ("[R6]" in out and "三方一致" in out))

    print("自检格数: %d" % n_case[0])
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="缺口账视图分离：三方对拍 + 体积/单行纪律 + 指纹写保护（设计稿 v1.3 §0 ①）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="主仓仓根（默认真源：本脚本所在仓 · 只用来定位兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认：主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--source", default=None, help="真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    return run(a.root, a.docs_root, a.source, a.json)


if __name__ == "__main__":
    sys.exit(main())

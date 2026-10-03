#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-idspace-map.py v1.0.0 —— **号段映射一格**（`Q-073`）：记号唯一 · 各线分列 · **同号两义必登记**

真源（**逐字** · 不转述）
------------------------
`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md` §一 序84
（组3 §一 序54 · `Q-073`）判据栏逐字：
  「两账的号段映射有一格（`N-xx → Q-xx` 唯一）；负控：同号两义 ⇒ 红」
现读栏逐字（本枚转引 · 组3 已现读）：
  「组3 现读 `grep -c 'R-11' 缺口总账-20260922.md` ⇒ **6** · `grep -c 人签` ⇒ **56**
   ⇒ 上版自述与现读不符 ✗」（**账件本单不碰** ✗ ⇒ 本件只判**登记面**，不替账件择一）

判据对象（**一格**）
-------------------
`Zerg-内部文档/项目文档/v2.5.12/登记-号段映射-v2.5.12-20260924.md`（本波新件 · 仓外）的两张表：
  · §二 **映射表**（表头逐字）：
      `| 记号 | 号线 | 真源件 | 映射到 | 同记号几处 | 依据（现读） |`
  · §三 **同号两义 / 跨线同名登记**（表头逐字）：
      `| 记号 | 两处（件 · 号线） | 判 | 登记理由（逐字引一句 · 内容锚） |`

判据（六格 · 每格能单独失败）
-----------------------------
  `K1` **记号在同一条号线上唯一**：同一 `(号线, 记号)` 在映射表里**只许出现一次**；
       重复 ⇒ **红并点名**（比的是「号线 + 记号」这一对 —— 同一记号**住不同号线**是**合法**的两条映射，
       见 `K5`）。★ **唯一白名单**：该行「同记号几处」列逐字写成 `≥2：登记行 + 入账行`
       （`N` 号线那种「一处登记行 + 一处入账行 = 同一条」的**显式声明**形态）。
  `K2` **记号形态**：`^[A-Z]{1,3}-\\d{1,3}$`（值域外 ⇒ 红）。
  `K3` **「映射到」值域**：`^Q-\\d{3}$` **或** 以 `无 Q 号` 开头的一句话（值域外 ⇒ 红）。
  `K4` **「同记号几处」值域**：`1` **或** 逐字 `≥2：登记行 + 入账行`（值域外 ⇒ 红）。
  `K5` **跨线同名必登记（负控面）**：凡在映射表里**跨 ≥2 条号线重名**的记号 ⇒
       **必须**在 §三 表里逐条出现（不写即红 —— 这正是「同号两义」的病：
       `G-` 三线 · `T-` 三线 · `R`/`R′` 两线同住一处、只看记号就必对错账）。
  `K6` **空转 / 缺件**：登记件不在盘 · 两张表的表头逐字句找不到 · 映射表 **0 行** ⇒
       **rc=2（不给结论）** —— 缺件与空转一律**不当绿**。

退码（三档）
------------
  0 = 六格全过（且 🧾 报表：记号行数 · 号线数 · 跨线重名数 · 登记复数）
  1 = **有红**：同线重号未登记 / 形态越域 / 跨线同名未登记（逐条点名「件:行 + 原样值」）
  2 = **不给结论**：登记件缺 · 表头缺 · 映射表空 · `--self-test` 不过

用法
----
    python3 scripts/gates/check-idspace-map.py                    # 真目标（仓外登记件）
    python3 scripts/gates/check-idspace-map.py --doc <件>          # 换一件登记面（自检夹具用）
    python3 scripts/gates/check-idspace-map.py --self-test        # 正/负成对 · 不碰真目标
    python3 scripts/gates/check-idspace-map.py --list-rules
    python3 scripts/gates/check-idspace-map.py --json verdict,rows,lines,cross

挂门：`scripts/gates/precommit-gates.sh` · `gates` scope · 门㊴ · `tri` 阻断 · 在默认集 ·
       **不进** `FAST_STEPS`（要读**仓外** `Zerg-内部文档/` 那件 ⇒ 别的机器上不普遍可用 · 同门㉑/㉔/㉕/㉟/㊱ 口径）。
"""
import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
DEFAULT_DOC = os.path.join(REPO_ROOT, "..", "Zerg-内部文档", "项目文档", "v2.5.12",
                           "登记-号段映射-v2.5.12-20260924.md")

HDR_MAP = "| 记号 | 号线 | 真源件 | 映射到 | 同记号几处 | 依据（现读） |"
HDR_CROSS = "| 记号 | 两处（件 · 号线） | 判 | 登记理由（逐字引一句 · 内容锚） |"

RE_ID = re.compile(r"^[A-Z]{1,3}-\d{1,3}$")
RE_MAPTO = re.compile(r"^Q-\d{3}$")
NOPREFIX = "无 Q 号"
DUP_OK = "≥2：登记行 + 入账行"          # 唯一白名单（逐字）


def cell(s: str) -> str:
    return s.strip().strip("`").strip()


def parse_table(lines, header, width):
    """按**表头逐字句**取一张表 ⇒ (起行号, [ [cell,…] ])。表头找不到 ⇒ (None, None)。"""
    start = None
    for i, ln in enumerate(lines):
        if ln.strip() == header:
            start = i
            break
    if start is None:
        return None, None
    rows = []
    for i in range(start + 1, len(lines)):
        ln = lines[i]
        if not ln.lstrip().startswith("|"):
            break
        cells = [c for c in ln.strip().strip("|").split("|")]
        if all(set(c.strip()) <= set("-: ") for c in cells):        # 分隔行
            continue
        if len(cells) < width:
            rows.append((i + 1, None))                              # 列数不足 ⇒ 记成坏行
            continue
        rows.append((i + 1, [cell(c) for c in cells[:width]]))
    return start + 1, rows


def judge(doc_path):
    """⇒ (verdict, 报表 dict, 红列表 [ (行号, 判据, 原样说明) ]) · verdict ∈ ok/red/noconc。"""
    if not doc_path or not os.path.isfile(doc_path):
        return "noconc", {"why": "登记件不在盘：%s" % doc_path}, []
    # ● GAP-20261001-23② 修：登记件非 UTF-8 时原 open(encoding=utf-8) 抛 UnicodeDecodeError
    #   （**非** OSError）⇒ 回溯 rc=1。读取失败一律「不给结论 rc=2」，不得以回溯冒充判据崩了。
    try:
        lines = open(doc_path, encoding="utf-8").read().splitlines()
    except (OSError, UnicodeDecodeError) as e:
        return "noconc", {"why": "登记件读不了（%s）" % e}, []
    m_start, map_rows = parse_table(lines, HDR_MAP, 6)
    if m_start is None:
        return "noconc", {"why": "映射表表头逐字句找不到"}, []
    if not map_rows:
        return "noconc", {"why": "映射表 0 行 ⇒ 空转（不当绿）"}, []
    c_start, cross_rows = parse_table(lines, HDR_CROSS, 4)
    if c_start is None:
        return "noconc", {"why": "同号两义登记表表头逐字句找不到"}, []

    reds = []
    seen = {}                      # (号线, 记号) ⇒ 行号
    by_id = {}                     # 记号 ⇒ set(号线)
    for lineno, cells in map_rows:
        if cells is None:
            reds.append((lineno, "K0", "列数不足 6（行不完整）"))
            continue
        ident, line, _src, mapto, howmany, _why = cells
        if not RE_ID.match(ident):
            reds.append((lineno, "K2", "记号形态越域：%r" % ident))
        if not (RE_MAPTO.match(mapto) or mapto.startswith(NOPREFIX)):
            reds.append((lineno, "K3", "「映射到」越域：%r" % mapto))
        if howmany not in ("1", DUP_OK):
            reds.append((lineno, "K4", "「同记号几处」越域：%r" % howmany))
        key = (line, ident)
        if key in seen and howmany != DUP_OK:
            reds.append((lineno, "K1", "同线重号（%s · %s）—— 首次在 :%d，且本行没写登记形态 %r"
                         % (line, ident, seen[key], DUP_OK)))
        seen.setdefault(key, lineno)
        by_id.setdefault(ident, set()).add(line)

    cross_ids = {r[1][0] for r in cross_rows if r[1]}
    cross_n = 0
    for ident, lset in sorted(by_id.items()):
        if len(lset) >= 2:
            cross_n += 1
            if ident not in cross_ids:
                reds.append((0, "K5", "跨线同名未登记：%s 住 %d 条号线（%s）却不在 §三 表里"
                             % (ident, len(lset), " · ".join(sorted(lset)))))

    rep = {"rows": len(map_rows), "lines": len({r[1][1] for r in map_rows if r[1]}),
           "cross": cross_n, "cross_registered": len(cross_ids),
           "skipped": []}
    return ("red" if reds else "ok"), rep, reds


# ─── 自检（正/负成对 · 合成夹具 · 不碰真目标） ───────────────────────────────────
SELFTEST_MARKER = "# ─── 自检"


def _fixture(tmpdir, map_rows, cross_rows):
    p = os.path.join(tmpdir, "登记件.md")
    body = ["# 夹具", "", "## 二 · 映射表", "", HDR_MAP, "|---|---|---|---|---|---|"]
    body += ["| %s |" % " | ".join(r) for r in map_rows]
    body += ["", "> 口径（夹具）", "", "## 三 · 同号两义 / 跨线同名登记", "", HDR_CROSS,
             "|---|---|---|---|"]
    body += ["| %s |" % " | ".join(r) for r in cross_rows]
    open(p, "w", encoding="utf-8").write("\n".join(body) + "\n")
    return p


GOOD_MAP = [
    ["`Q-073`", "`Q`", "缺口总账-20260923.md", "`Q-073`", "1", "§一.7 结转行"],
    ["`N-05`", "`N`", "缺口总账-20260923.md", "`Q-121`", "1", "§一.4 逐字"],
    ["`N-06`", "`N`", "缺口总账-20260923.md", "无 Q 号（原记号保留 · 转实）", DUP_OK, "§附录三十二"],
    ["`G-10`", "`G①`", "任务清单-缺口总账-v2.5.12-20260924.md", "无 Q 号（`C-02` 线）", "1", "§一 第二列"],
    ["`G-10`", "`G②`", "缺口总账-20260923.md", "`Q-110`", "1", "§一.7 第三列"],
    ["`T-2`", "`T②`", "任务清单-本版全做-v2.5.12-20260924.md", "无 Q 号（与 `G-05` 同条）", "1", "§一 `G-05` 行"],
]
GOOD_CROSS = [["`G-10`", "两件（`G①`／`G②`）", "**两义** ✗", "`G①` → `C-02` · `G②` → `Q-110`"]]


def self_test():
    import tempfile
    ok = []
    failed = []

    def case(name, want, got, note=""):
        (ok if want == got else failed).append((name, want, got, note))

    with tempfile.TemporaryDirectory(prefix="zerg-idspace-") as d:
        # ① 好夹具
        doc = _fixture(d, GOOD_MAP, GOOD_CROSS)
        v, rep, reds = judge(doc)
        case("① 好夹具 ⇒ rc=0", "ok", v, "红 %d 条" % len(reds))
        # ② 同线重号（未登记形态）
        bad = GOOD_MAP + [["`G-10`", "`G①`", "另一件", "无 Q 号（旧线）", "1", "另一行"]]
        v, _, reds = judge(_fixture(d, bad, GOOD_CROSS))
        case("② 负控：同线重号 ⇒ rc=1", "red", v)
        case("②b 负控点名到「件:行 + K1」", True,
             any(r[1] == "K1" and r[0] > 0 for r in reds), "%r" % (reds[:1]))
        # ③ 真话负控：同线重号但**本行**写了登记形态 ⇒ 回绿
        var = GOOD_MAP + [["`N-06`", "`N`", "同一件", "无 Q 号（原记号保留 · 转实）", DUP_OK, "§附录三十二 入账行"]]
        v, _, _ = judge(_fixture(d, var, GOOD_CROSS))
        case("③ 真话负控：写成登记形态 ⇒ 回绿", "ok", v)
        # ④ 跨线同名未登记（同号两义）
        v, _, reds = judge(_fixture(d, GOOD_MAP, []))
        case("④ 负控：跨线同名未登记 ⇒ rc=1", "red", v)
        case("④b 负控点名 K5 + 记号", True,
             any(r[1] == "K5" and "G-10" in r[2] for r in reds), "%r" % (reds[:1]))
        # ⑤ 真话负控：写进 §三 ⇒ 回绿
        v, _, _ = judge(_fixture(d, GOOD_MAP, GOOD_CROSS))
        case("⑤ 真话负控：登记进 §三 ⇒ 回绿", "ok", v)
        # ⑥ 记号形态越域
        v, _, reds = judge(_fixture(d, [["`q73`", "`Q`", "件", "`Q-073`", "1", "x"]], []))
        case("⑥ 负控：记号形态越域 ⇒ rc=1", "red", v)
        case("⑥b 点名 K2 且带原样值", True, any(r[1] == "K2" and "q73" in r[2] for r in reds))
        # ⑦ 「映射到」越域
        v, _, reds = judge(_fixture(d, [["`Q-073`", "`Q`", "件", "Q73", "1", "x"]], []))
        case("⑦ 负控：「映射到」越域 ⇒ rc=1", "red", v)
        case("⑦b 点名 K3", True, any(r[1] == "K3" for r in reds))
        # ⑧ 「同记号几处」越域
        v, _, reds = judge(_fixture(d, [["`Q-073`", "`Q`", "件", "`Q-073`", "2", "x"]], []))
        case("⑧ 负控：「同记号几处」越域 ⇒ rc=1", "red", v)
        case("⑧b 点名 K4", True, any(r[1] == "K4" for r in reds))
        # ⑨ 表头逐字句找不到 ⇒ 不给结论
        p = os.path.join(d, "坏件.md")
        open(p, "w", encoding="utf-8").write("# 坏件\n\n没有那两行表头\n")
        case("⑨ 表头缺 ⇒ rc=2", "noconc", judge(p)[0])
        # ⑩ 映射表 0 行 ⇒ 不给结论
        p2 = os.path.join(d, "空表.md")
        open(p2, "w", encoding="utf-8").write(HDR_MAP + "\n|---|---|---|---|---|---|\n\n" + HDR_CROSS + "\n")
        case("⑩ 映射表空 ⇒ rc=2", "noconc", judge(p2)[0])
        # ⑪ 缺件 ⇒ 不给结论
        case("⑪ 登记件缺 ⇒ rc=2", "noconc", judge(os.path.join(d, "不在盘.md"))[0])
        # ⑫ 真目标（**仓外**登记件）—— ● GAP-20261001-23① 修：原 case 直接读仓外真件 ⇒ 自检不 hermetic，
        #   目标树缺席时由 0 翻 1（恒红 · 非本机缺陷却报红）。缺树 ⇒ **不给结论 rc=2**，不翻红。
        if not os.path.isfile(DEFAULT_DOC):
            print("  ~ ⑫/⑫b/⑫c 跳过：真目标登记件不在盘（%s）⇒ 自检依赖仓外件、不可 hermetic 判定 ⇒ 不给结论" % DEFAULT_DOC)
            return 2
        v, rep, reds = judge(DEFAULT_DOC)
        case("⑫ 真目标：登记件在场 + 六格全过", "ok", v, "行 %d · 红 %d" % (rep.get("rows", -1), len(reds)))
        case("⑫b 真目标映射表规模 ≥ 30 行", True, rep.get("rows", 0) >= 30, "行 %d" % rep.get("rows", -1))
        case("⑫c 真目标跨线重名 ≥ 8（G×7 + R-11）", True, rep.get("cross", 0) >= 8,
             "cross %d" % rep.get("cross", -1))

    for name, want, got, note in ok:
        print("   ✓ %s（%s）" % (name, note or got))
    for name, want, got, note in failed:
        print("   ✗ %s：期望 %s · 实得 %s（%s）" % (name, want, got, note))
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not failed else "有失败", len(ok) + len(failed), len(failed)))
    return 0 if not failed else 1


RULES = [
    "K1 记号在同一条号线上唯一（唯一白名单：逐字「%s」）" % DUP_OK,
    "K2 记号形态 ^[A-Z]{1,3}-\\d{1,3}$",
    "K3 「映射到」= ^Q-\\d{3}$ 或「%s…」的一句话" % NOPREFIX,
    "K4 「同记号几处」= 1 或 逐字「%s」" % DUP_OK,
    "K5 跨 ≥2 条号线重名的记号必须在 §三 逐条登记（同号两义 ⇒ 红）",
    "K6 缺件 / 表头缺 / 映射表空 ⇒ rc=2（不给结论 · 不当绿）",
]


# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['verdict', 'rows', 'lines', 'cross', 'reds']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main(argv=None):
    rc_q176 = refuse_unknown_json_fields(argv)
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--doc", default=DEFAULT_DOC)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--json", default="")
    a = ap.parse_args(argv)

    if a.list_rules:
        for i, r in enumerate(RULES, 1):
            print("  %d. %s" % (i, r))
        return 0
    if a.self_test:
        return self_test()

    verdict, rep, reds = judge(a.doc)
    if a.json:
        want = {k.strip() for k in a.json.split(",") if k.strip()}
        obj = {"verdict": verdict, "rows": rep.get("rows", 0), "lines": rep.get("lines", 0),
               "cross": rep.get("cross", 0), "reds": len(reds)}
        print(json.dumps({k: v for k, v in obj.items() if k in want},
                         ensure_ascii=False, sort_keys=True))
        return {"ok": 0, "red": 1, "noconc": 2}[verdict]

    print("── 号段映射一格（Q-073）· 判对象 = %s" % a.doc)
    if verdict == "noconc":
        print("   不给结论：%s" % rep.get("why"))
        print("⇒ rc=2（缺件 / 表头缺 / 空转一律不当绿）")
        return 2
    print("   映射表 %d 行 · 号线 %d 条 · 跨线重名 %d 个 · §三 登记 %d 条"
          % (rep["rows"], rep["lines"], rep["cross"], rep["cross_registered"]))
    if reds:
        print("── 红 %d 条（逐条点名 · 不合并）──" % len(reds))
        for lineno, rule, why in reds:
            print("   ✗ %s :%d %s" % (rule, lineno, why))
        print("⇒ rc=1：有红（同线重号未登记 / 形态越域 / 跨线同名未登记）")
        return 1
    print("── 六格全过：K1 记号同线唯一 · K2/K3/K4 形态在域 · K5 跨线重名全登记 · K6 非空转")
    print("⇒ rc=0")
    return 0


if __name__ == "__main__":
    sys.exit(main())

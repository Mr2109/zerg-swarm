#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gap-ledger-face.py —— **两套手写账的「表头 + 状态值域」闭集登记自检**（`Q-036`）

真源与口径（**判据绑私有面** · 逐字引文全指向仓外 `Zerg-内部文档/`）
------------------------------------------------------------
· 题面（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/缺口总账-20260923.md` §一 序 33
  —— 「两套手写账表头／状态值域不一、无机器在读 | 已立项／P1 | 未核 ✗」
· 上游实证（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.11/调研-缺口自发现与自进化-20260922.md`
  §二.4 —— 「**同一族账、三套表头、两套状态值域** —— 且**没有任何机器在读它们**
  （`grep -c 缺口` 在门禁里 = 0）」
· 施工条（仓外 · 逐字）：`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md`
  §一 序 77 —— 判据栏 = 「两套账的表头 / 状态值域**有闭集登记**；负控：**值域外 ⇒ `rc=1` 并点名**」
· 闭集登记件（成文面 · 逐字对拍）：本公司外并排仓
  `Zerg-内部文档/项目文档/v2.5.12/登记-两套账面闭集-v2.5.12-20260924.md`（本门 `--register` 读它）

本件补的那一格（**Q-036 的病根 = 「无机器在读」**）
--------------------------------------------------
病根不是「两套值域不一样」（手写账本来就允许不一样），而是**没有任何机器在读它们** ⇒
值域漂了、表头改了，**没有一个出口会红**。所以本件的判据**不是**「把两套统一」✗
（那是改这两件账的活，不属本门），而是：

  ① 把「表头逐字句 + 状态列 + 值域闭集」**登记**成机器可读的一份（本件常量 + 仓外登记件）；
  ② **每一次现跑**都拿盘上的两份账去读：表头是否还是登记的那一句、状态列里的每一个值
     是否还在闭集内；
  ③ **值域外 ⇒ `rc=1` + 逐条点名**（件:行 + 原样值 + 所属面）—— 这就是「有机器在读」。

登记面（**现读 2026-09-24 的盘上实况 · 4 个面**；「三套表头」照实 = 4 个登记面 ——
`v2.5.11` 那一套表头有**两个变体**（§一 与 §二/§三 的「建议形态」列不同），照实分成两面，
**不是**把两读裁成一个）
--------------------------------------------------------------------
  A  `v2.5.10/缺口-命令面-20260921.md` §一–§十 · 列 3「现有命令能吗」 · 闭集 `✓ / 半 / 无`
  B  同件 §十三 · 列 3「状态」 · 闭集 `✓已解 / ⟳本轮补 / ✗仍缺`
  C  `v2.5.11/缺口-命令面-20260922.md` §一 · 列 6「状态」 · 闭集 `已解 / 已派 / 已立项 / 仍缺`
  D  同件 §二/§三 · 列 6「状态」 · 同 C 的闭集（**表头变体单列**：`建议形态（已落地 / 在册）` ⟷ `建议形态`）

交叉引用白名单（**照实收下 · 不是放宽判据**）：`v2.5.10` 的「现有命令能吗」列里有**两处**
`同 §四` 形态（跨节指路、不是取值）⇒ 登记成白名单正则 `^同 §`；白名单之外一律按值域外判。
★ 白名单**只有这一条**，且它管的是「跨节指路」这一种语法形态，**不是**给某个具体值开后门。

退码（三档 · 与仓内既有门同惯例）
--------------------------------
  0 = 四个面的每一个值都在闭集内（或命中交叉引用白名单），且登记件与现读一致
  1 = **值域外**（逐条点名）/ 登记件与现读不一致（面计数不符 / 闭集值不在登记件里）
  2 = **不给结论**：账件不在盘 / 登记件不在盘 / 表头逐字句在盘上找不到 / 某面 0 行（空转）/
      自检不过 —— 一律显式打印，不许静默当绿

用法
----
    python3 scripts/gates/check-gap-ledger-face.py                  # 判（读盘上两件账 + 仓外登记件）
    python3 scripts/gates/check-gap-ledger-face.py --json            # 机读（zerg/v1 形状）
    python3 scripts/gates/check-gap-ledger-face.py --docs-root DIR   # 换文档仓根（默认真源：主仓的兄弟目录）
    python3 scripts/gates/check-gap-ledger-face.py --register PATH   # 换登记件（缺省 = 仓外那件）
    python3 scripts/gates/check-gap-ledger-face.py --no-register     # 只判账、不对拍登记件
    python3 scripts/gates/check-gap-ledger-face.py --list-rules      # 打四个面 / 闭集 / 白名单 / 现读
    python3 scripts/gates/check-gap-ledger-face.py --self-test       # 只跑自检（合成夹具 · 不碰真账）

★ 本笔**不挂门**（件头如实）：`scripts/gates/precommit-gates.sh` 不在本波件面里 ⇒ 本件今天
  只作**单跑档**；要挂门是**另一笔**的活（连带刷新 `add_step` 自检计数）。
"""

import argparse
import collections
import json
import os
import re
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-gap-ledger-face.py v1.0.0"

LEDGER_A = "项目文档/v2.5.10/缺口-命令面-20260921.md"
LEDGER_B = "项目文档/v2.5.11/缺口-命令面-20260922.md"
REGISTER_REL = "项目文档/v2.5.12/登记-两套账面闭集-v2.5.12-20260924.md"

# ── 登记面（机器可读的唯一真源；成文面 = 仓外登记件，本门现跑对拍它）───────────────
#   字段：id / ledger(相对 docs 根) / col(0 基列号) / header(逐字句) / closed(闭集 · 元组) / label
FACES = (
    {
        "id": "A",
        "label": "v2.5.10 §一–§十 · 现有命令能吗",
        "ledger": LEDGER_A,
        "col": 3,
        "header": "| 编号 | 我们要干的事（人话一句） | 今天怎么干的（手搓了什么 · 原样引一句） | "
                  "现有命令能吗 | 建议的新命令（形态 + 输入 + 输出 + 退码） | 优先级 |",
        "closed": ("✓", "半", "无"),
    },
    {
        "id": "B",
        "label": "v2.5.10 §十三 · 状态",
        "ledger": LEDGER_A,
        "col": 3,
        "header": "| 编号 | 事（人话一句） | 今天怎么手搓的（**原样引一句**） | 状态 | "
                  "建议命令（形态） | 优先级 |",
        "closed": ("✓已解", "⟳本轮补", "✗仍缺"),
    },
    {
        "id": "C",
        "label": "v2.5.11 §一 · 状态",
        "ledger": LEDGER_B,
        "col": 6,
        "header": "| 编号 | 现象 | 今天怎么手搓的（必填） | 影响面 | 建议形态（已落地 / 在册） | "
                  "优先级 | 状态 |",
        "closed": ("已解", "已派", "已立项", "仍缺"),
    },
    {
        "id": "D",
        "label": "v2.5.11 §二/§三 · 状态（表头变体）",
        "ledger": LEDGER_B,
        "col": 6,
        "header": "| 编号 | 现象 | 今天怎么手搓的（必填） | 影响面 | 建议形态 | 优先级 | 状态 |",
        "closed": ("已解", "已派", "已立项", "仍缺"),
    },
)

# ── 交叉引用白名单（**只有这一条** · 语法形态，不是取值）──────────────────────────
CROSSREF_RE = re.compile(r"^同\s*§")

# ── 登记件里必须逐字出现的两样（对拍用）──────────────────────────────────────────
#   ① 每个面的闭集值（去重后逐条）；② 每个面的现读计数句「面 X … 数据行 N」
REGISTER_VALUES_MUST_APPEAR = True


def cells(line):
    """把一行 markdown 表行切成单元；`\\|`（转义竖线）**不**算分隔符。"""
    s = line.strip()
    if not (s.startswith("|") and s.endswith("|")):
        return None
    return [c.strip() for c in re.split(r"(?<!\\)\|", s[1:-1])]


def is_sep(cs):
    return bool(cs) and set("".join(cs)) <= set("-: ")


def norm_value(raw):
    """状态单元的**归一值**：去掉粗体/行内代码标记，再取到第一个分隔符之前。

    为什么这么归一（**不是为了「宽松」**）：这两件账的状态列是**手写**的 —— 值后面常跟
    「：`zerg build ls`（实测 rc=0）…」这类补充说明。判据要判的是**取值**，不是那条说明
    的措辞 ⇒ 归一取「值」这一截；说明那一截一个字都不进判据面（也一个字不改账）。
    """
    s = re.sub(r"[`*]", "", raw.strip())
    for cut in ("：", "（", "(", " ", "✅", "·"):
        i = s.find(cut)
        if i > 0:
            s = s[:i]
    return s.strip()


def scan_face(docs, face):
    """返回 (状态, 读数, 越界清单)。

    状态 ∈ `ok` / `no_ledger` / `no_header` / `empty`；读数 = dict(headers, rows, values)
    """
    path = os.path.join(docs, face["ledger"])
    if not os.path.isfile(path):
        return "no_ledger", {"path": path}, []
    with open(path, encoding="utf-8") as f:
        lines = f.read().splitlines()
    ncol = len(cells(face["header"]))
    occ = [i for i, l in enumerate(lines) if l.strip() == face["header"]]
    if not occ:
        return "no_header", {"path": path}, []
    values = collections.Counter()
    rows = 0
    bad = []
    for o in occ:
        for i in range(o + 2, len(lines)):
            cs = cells(lines[i])
            if cs is None or len(cs) != ncol:
                break
            if is_sep(cs):
                continue
            rows += 1
            raw = cs[face["col"]]
            # 白名单判在**归一之前**：`同 §四` 是「跨节指路」这一种**语法形态**，
            #   归一（取到第一个分隔符之前）会把它切成 `同` ⇒ 先判形态、再判取值。
            if CROSSREF_RE.match(re.sub(r"[`*]", "", raw.strip())):
                values["同 §（跨节指路·白名单）"] += 1
                continue
            v = norm_value(raw)
            values[v] += 1
            if v not in face["closed"]:
                bad.append({"ledger": face["ledger"], "line": i + 1, "value": v,
                            "raw": raw, "face": face["id"]})
    read = {"path": path, "headers": len(occ), "header_lines": [o + 1 for o in occ],
            "rows": rows, "values": dict(values)}
    if rows == 0:
        return "empty", read, bad
    return "ok", read, bad


def scan_register(path, faces, reads):
    """对拍仓外登记件：① 每个面的闭集值逐字在册；② 每个面的现读计数句在册。"""
    problems = []
    if not os.path.isfile(path):
        return ["登记件不在盘：%s" % path]
    with open(path, encoding="utf-8") as f:
        txt = f.read()
    for face in faces:
        for v in face["closed"]:
            if REGISTER_VALUES_MUST_APPEAR and ("`%s`" % v) not in txt and v not in txt:
                problems.append("登记件里找不到面 %s 的闭集值 `%s`" % (face["id"], v))
        rd = reads.get(face["id"])
        if rd and rd.get("rows"):
            line = "面 %s · 表头 %d 处 · 数据行 %d" % (face["id"], rd["headers"], rd["rows"])
            if line not in txt:
                problems.append("登记件与现读不一致：缺计数句「%s」" % line)
    return problems


def run(root, docs_root, register, json_out):
    root = os.path.abspath(root)
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根" % root)
        return 2
    docs = os.path.abspath(docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档"))
    if not os.path.isdir(docs):
        print("⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）" % docs)
        return 2
    reports, bad_all, problems = [], [], []
    reads = {}
    for face in FACES:
        st, read, bad = scan_face(docs, face)
        if st in ("no_ledger", "no_header", "empty"):
            problems.append({"face": face["id"], "state": st, "read": read})
        reads[face["id"]] = read
        bad_all += bad
        reports.append((face, st, read))

    if register:
        problems += [{"face": "-", "state": "register", "read": {"msg": m}}
                     for m in scan_register(register, FACES, reads)]

    if json_out:
        doc = {
            "schema": "zerg/v1", "kind": "GapLedgerFace", "tool": VERSION,
            "items": [{"face": f["id"], "label": f["label"], "ledger": f["ledger"],
                       "col": f["col"], "closed": list(f["closed"]),
                       "headers": rd.get("headers", 0), "rows": rd.get("rows", 0),   # ● GAP-20261001-29：面缺席（no_ledger/no_header）时 rd 只有 path ⇒ 原 rd["headers"] 取 KeyError 崩 rc=1；改与正文分支同口径的 .get
                       "values": rd.get("values", {}), "state": st} for f, st, rd in reports],
            "meta": {"faces": len(FACES), "register": register or "",
                     "docs_root": os.path.join(root, "Zerg-内部文档")},
            "out_of_domain": bad_all, "problems": problems, "warnings": [],
            "truncated": False,
        }
        print(json.dumps(doc, ensure_ascii=False))
        if problems:
            return 2
        return 1 if bad_all else 0

    print("── 两套手写账「表头 + 状态值域」闭集自检（%s · `Q-036`）──" % VERSION)
    print("  文档仓根: %s" % docs)
    print("  登记件  : %s" % (register or "（--no-register ⇒ 本跑不对拍登记件）"))
    print("  %-3s %-34s %-6s %-8s %s" % ("面", "账件 · 面", "表头", "数据行", "现读值域（闭集内）"))
    for face, st, rd in reports:
        vals = " ".join("%s×%d" % (k, v) for k, v in sorted(rd.get("values", {}).items()))
        print("  %-3s %-34s %-6s %-8s %s" % (face["id"], face["label"] + " ← " + face["ledger"].split("/")[-1],
                                             rd.get("headers", "-"), rd.get("rows", "-"), vals))
        print("      闭集: %s" % " / ".join(face["closed"]))
    print("  ── 合计：%d 面 · 表头 %d 处 · 数据行 %d" %
          (len(FACES), sum(rd.get("headers", 0) for _f, _s, rd in reports),
           sum(rd.get("rows", 0) for _f, _s, rd in reports)))

    if problems:
        print("⛔ 不给结论（rc=2）：")
        for p in problems:
            print("   · 面 %s %s：%s" % (p["face"], p["state"], p["read"].get("msg") or p["read"].get("path")))
        return 2
    if bad_all:
        print("✗ 值域外 %d 处（rc=1 · 逐条点名）：" % len(bad_all))
        for b in bad_all:
            print("   · %s:%d 面 %s 值 `%s` ⟵ 原样 `%s`" %
                  (b["ledger"], b["line"], b["face"], b["value"], b["raw"][:70]))
        print("⇒ 处置（**不是**改本门）：值域要扩 ⇒ ① 改本件 FACES 的 `closed` "
              "② 同笔改仓外登记件 —— 两处同笔，否则下一次跑还会红。")
        return 1
    print("⇒ 值域内（rc=0）：4 个面的每一个值都在登记闭集内（或命中交叉引用白名单 `^同 §`）")
    print("   ★ 本件只**读**这两件账：两件账一个字未改（写它们 = 另一笔的活）")
    return 0


def list_rules():
    print("── %s 判据与登记面 ──" % VERSION)
    for f in FACES:
        print("  面 %s · %s" % (f["id"], f["label"]))
        print("     账件 : %s" % f["ledger"])
        print("     列   : 第 %d 列（0 基）" % f["col"])
        print("     表头 : %s" % f["header"])
        print("     闭集 : %s" % " / ".join(f["closed"]))
    print("交叉引用白名单（唯一条）：%s" % CROSSREF_RE.pattern)
    print("归一法：去 `` ` ``/`*` ⇒ 取到第一个「：/（/(/空格/✅/·」之前（说明那一截不进判据面）")
    print("退码：0 值域内 · 1 值域外或登记件与现读不一致 · 2 不给结论（缺件/表头找不到/空转/自检不过）")
    print("★ 本笔不挂门（见件头）：挂门是另一笔的活。")
    return 0


# ───────────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真账）─────────────────────────────
FIXTURE_FACE = {
    "id": "A", "label": "夹具面", "ledger": "项目文档/vX/账.md", "col": 1,
    "header": "| 号 | 状态 | 备注 |",
    "closed": ("已解", "仍缺"),
}


def self_test():
    ok = True
    tmp = tempfile.mkdtemp(prefix="glf-selftest-")
    # 自检**全程**只跑合成面（不碰真账 · 也不读真账）——
    #   这就是「不碰真目标」那条口径的机械落点：把真 FACES 换成一个合成面。
    global FACES
    real_faces = FACES
    FACES = (dict(FIXTURE_FACE, col=1),)

    def mk(rows, header=FIXTURE_FACE["header"]):
        """造一份合成夹具：`<case>/repo`（仓根）+ `<case>/Zerg-内部文档/项目文档/vX/账.md`（**兄弟布局**）+ 登记件。"""
        base = os.path.join(tmp, "case-%d" % len(rows))
        docs = os.path.join(base, "Zerg-内部文档")
        os.makedirs(os.path.join(base, "repo", "scripts", "gates"), exist_ok=True)
        os.makedirs(os.path.join(docs, "项目文档", "vX"), exist_ok=True)
        led = os.path.join(docs, "项目文档", "vX", "账.md")
        with open(led, "w", encoding="utf-8") as f:
            f.write("# 合成账\n\n")
            f.write(header + "\n")
            f.write("|---|---|---|\n")
            for r in rows:
                f.write(r + "\n")
            f.write("\n尾。\n")
        # 登记件（照真登记件的形态：闭集值逐字 + 计数句）
        reg = os.path.join(docs, "项目文档", "vX", "登记-合成.md")
        with open(reg, "w", encoding="utf-8") as f:
            f.write("闭集 `已解` / `仍缺`\n")
            n = len([r for r in rows if r.strip()])
            f.write("面 A · 表头 1 处 · 数据行 %d\n" % n)
        return os.path.join(base, "repo"), docs, led, reg

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

    # ① 正控：全部在闭集内 ⇒ rc=0
    r, d, _led, reg = mk(["| x | 已解 | a |", "| y | 仍缺 | b |"])
    rc, out = cap(r, d, reg, False)
    case("① 正控：两个值都在闭集内 ⇒ rc=0", 0, rc, "值域内" in out)

    # ② 负控：值域外（造一行 `已派` —— 那是**另一套**值域里的值）⇒ rc=1 且点名到行
    r2, d2, _l2, reg2 = mk(["| x | 已解 | a |", "| y | 已派 | b |", "| z | 仍缺 | c |"])
    rc, out = cap(r2, d2, reg2, False)
    case("② 负控：值域外 `已派` ⇒ rc=1", 1, rc, "值域外" in out)
    case("②b 负控点名到「件:行 + 原样值」", True, ("账.md:6" in out and "已派" in out),
         note="  见输出逐条点名")

    # ②c 真话负控：把闭集改成含 `已派`（登记件同笔扩）⇒ 同一份夹具回绿
    #     （证明红是红在那**一个值**上、不是红在夹具，也不是红在「红本身」）
    saved = FACES
    FACES = (dict(FIXTURE_FACE, col=1, closed=("已解", "已派", "仍缺")),)
    with open(reg2, "w", encoding="utf-8") as f:
        f.write("闭集 `已解` / `已派` / `仍缺`\n面 A · 表头 1 处 · 数据行 3\n")
    rc, out = cap(r2, d2, reg2, False)
    case("②c 真话负控（同一夹具 · 闭集扩到含它）⇒ 回绿", 0, rc, "值域内" in out)
    FACES = saved

    # ③ 交叉引用白名单：`同 §四` ⇒ 不算值域外
    r3, d3, _l3, reg3 = mk(["| x | 同 §四 | a |", "| y | 仍缺 | b |"])
    rc, out = cap(r3, d3, reg3, False)
    case("③ 白名单：`同 §四`（跨节指路）⇒ 不算值域外", 0, rc, "值域内" in out)

    # ④ 空转 ⇒ rc=2（表头在、0 行 —— 不许当绿）
    r4, d4, _l4, reg4 = mk([])
    rc, out = cap(r4, d4, reg4, False)
    case("④ 空转（表头在 · 0 行）⇒ rc=2 不给结论", 2, rc, "不给结论" in out)

    # ⑤ 表头逐字句在盘上找不到 ⇒ rc=2
    r5, d5, _l5, reg5 = mk(["| x | 已解 | a |"], header="| 号 | 状态 |")
    rc, out = cap(r5, d5, reg5, False)
    case("⑤ 表头逐字句找不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论" in out)

    # ⑥ 账件不在盘 ⇒ rc=2
    r6, d6, led6, reg6 = mk(["| x | 已解 | a |"])
    os.remove(led6)
    rc, out = cap(r6, d6, reg6, False)
    case("⑥ 账件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）", 2, rc, "不给结论" in out)

    # ⑦ 登记件对拍：① 闭集值不在登记件里 ⇒ rc=2；② 计数句不符 ⇒ rc=2
    r7, d7, _l7, reg7 = mk(["| x | 已解 | a |", "| y | 仍缺 | b |"])
    with open(reg7, "w", encoding="utf-8") as f:
        f.write("面 A · 表头 1 处 · 数据行 2\n")           # 少了闭集值 `仍缺`
    rc, out = cap(r7, d7, reg7, False)
    case("⑦ 登记件缺一个闭集值 ⇒ rc=2", 2, rc, "闭集值" in out)
    with open(reg7, "w", encoding="utf-8") as f:
        f.write("闭集 `已解` / `仍缺`\n面 A · 表头 1 处 · 数据行 9\n")   # 计数句不符
    rc, out = cap(r7, d7, reg7, False)
    case("⑦b 登记件计数句与现读不符 ⇒ rc=2", 2, rc, "缺计数句" in out)

    # ⑧ 登记件位空（= `--no-register` 的形态）⇒ 只判账 ⇒ rc=0
    rc, out = cap(r7, d7, "", False)
    case("⑧ 登记件位空（`--no-register` 形态）⇒ 只判账 rc=0", 0, rc, "值域内" in out)

    FACES = real_faces      # 复原真登记面（自检跑完不许把它留在合成面上）
    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    ap = argparse.ArgumentParser(add_help=True,
                                 description="两套手写账「表头 + 状态值域」闭集自检（Q-036）")
    ap.add_argument("--root", default=DEFAULT_ROOT, help="主仓仓根（默认真源：本脚本所在仓）")
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认：主仓的兄弟目录 Zerg-内部文档/）")
    ap.add_argument("--register", default=None, help="登记件路径（缺省 = 文档仓里那件）")
    ap.add_argument("--no-register", action="store_true", help="只判账、不对拍登记件")
    ap.add_argument("--json", action="store_true", help="机读输出（zerg/v1 形状）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()
    docs_root = a.docs_root
    register = ""
    if not a.no_register:
        if a.register:
            register = a.register
        else:
            base = a.docs_root or os.path.join(os.path.dirname(os.path.abspath(a.root)), "Zerg-内部文档")
            register = os.path.join(base, REGISTER_REL)
    return run(a.root, docs_root, register, a.json)


if __name__ == "__main__":
    sys.exit(main())

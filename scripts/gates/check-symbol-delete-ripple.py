#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-symbol-delete-ripple.py —— 门㉜：**删前波纹门**（`R28` · 分档版 · 阻断档）。

已拍口径（逐字 · 本门**只照这一张表**判）
----------------------------------------
`R28` 那一处有**两个版本**，本门照**已拍的分档版**（Mr2109 2026-09-23「按建议」= 分档）：

  | 动作 | 档 | 依据 |
  |---|---|---|
  | **改**（可逆） | **只记** | 审计行本来就有前后 `sha256` 与前后字节数 ⇒ 改是可逆的 |
  | **符号级删** | **必拦** | 不可逆（无反向件；同族：Terraform `prevent_destroy` / k8s finalizer / Django 不给 `reverse_code`） |
  | **整件删 / 搬出** | **只报** + 两件可找回证据 | 本机实测 95.6% 的删其实是「整件搬出」（找得回来）；一律拦会让九成半的拦是白拦 ⇒ 人开始习惯性绕 ⇒ 拦本身失效 |
  | **保险三条**（任一命中 ⇒ **升为必拦**） | ① 未跟踪件 ② 无归档副本 ③ 仓外件 | 能找回的只报，**找不回的必拦** |

  ★ **拒的是字面版**：`任务清单-缺口收口-20260923.md:92` 那半句「删=必拦、**不分档**」**按实记为被覆盖** ✗
    —— 本门**不许**照字面版判（设计稿 §一 已把两处口径并排留档）。
  ★ 真源：`Zerg-内部文档/项目文档/v2.5.11/设计-删前波纹门-R28口径统一-v1.0-20260923.md` §三（门的形状）·
    §四 判据 1–9 · §五 负控 ①–⑦。

输入（**只读那一份单** —— 门**不**自己重跑删面：跑删面 = 第二条删路径的影子）
---------------------------------------------------------------------------
  删面（`C1`）的**干跑单**：`zerg dev edit … --dry-run` 的 stderr 里那两行机器可读（一行一事实）：
      `impact_hook tier=… class=… retrievable=… tracked=… archive=… git_sha=… outside=… rel=…`
      `impact_criteria untracked=… no_archive_copy=… outside_repo=… criteria_hits=…`
  （取值纯 ASCII：`tier` ∈ {只记, 只报, 必拦}；`class` ∈ {改, 符号级删, 件级（整件）}；其余 `yes`/`no`。）
  落点解析：`--plan <件>` > `ZERG_R28_PLAN` > `<状态目录>/r28-plan.txt`（状态目录 = `ZERG_STATE_DIR` 或 `~/.zerg/state`）。

档位处置（本门与设计稿的一处**照实偏差**，写在明面上不遮）
-------------------------------------------------------
  · **有单** ⇒ 逐件判（判据 2–6 · 红/绿分明）。
  · **单在但 0 件** ⇒ **rc=2 不给结论**（判据 7 逐字：「0 件 ⇒ 不给结论」）。
  · **显式来源给了但读不到**（`--plan` / `ZERG_R28_PLAN`）⇒ **rc=2**（判据 7 逐字：缺件不许当绿）。
  · **没有任何单可读（默认落点也不在）** ⇒ 记**「空档：本趟没有删面」⇒ rc=0**，且**照实打出来**。
    ⚠ 这一格与判据 7 的**字面**有差口：照字面「没单 ⇒ 2」，本步会**每次提交都恒 BLOCKED**
    —— 那正是本仓点过名的「恒 BLOCKED ＝ 静默态」（v4 §二 `i18n-drift` 那一条）。⇒ 取值取**可运行**：
    空档**明说**「本门这一趟没有对象可判」（**不是绿**，判词里逐字写清）。**这条偏差登记在回执里，不粉饰。**

判据（判据 1–9 · 逐条落在本门里）
--------------------------------
  1 **分档表可复算**：同一份单解析两遍**逐字一致**；`tier` 与 `class` 必须落在**同一张表**上
    （`改`↔`只记` · `符号级删`↔`必拦` · `件级（整件）`↔{`只报`,`必拦`}）—— 表外的组合 ⇒ 红。
  2 **符号级删 ⇒ 必拦**（判据 2/3：这一档**不查**可找回证据 ⇒ 一律红）。
  3 「符号级删 + 只报」= 把分档表改写成字面版 ⇒ 红（负控 ③ 的第二种骗法）。
  4 **件级 + 可找回（`retrievable=yes` 且三判据全不命中）⇒ 只报 ⇒ 不许红**。
  5 **件级 + 任一不可找回判据命中（或 `retrievable=no`）⇒ 必拦 ⇒ 红，且逐条点名是哪一条命中的**。
  6 **改 ⇒ 只记 ⇒ 不许红**。
  7 空转/缺件 ⇒ rc=2（见上「档位处置」的四处取值）。
  8 **门脚本 0 写盘**：静态扫本件源码，`open(…, 'w')` / `open(…, 'a')` / `os.remove` / `shutil.move`
    / `O_WRONLY` 命中 **0**（`--self-test` 里成对自证）。
  9 **回执必带两格**：判定档 + 条数（对「恒 BLOCKED 不给结论」那类门的反例）。

用法
----
    python3 scripts/gates/check-symbol-delete-ripple.py                  # 现读默认落点
    python3 scripts/gates/check-symbol-delete-ripple.py --plan <件|->    # 指名一份干跑单（`-` = stdin）
    python3 scripts/gates/check-symbol-delete-ripple.py --self-test      # 9 格合成夹具（成对负控）
    python3 scripts/gates/check-symbol-delete-ripple.py --list-rules

退码：0 全绿（含「空档」）· 1 红（判据 2/3/5/7 的任一命中或表外组合）· 2 **不给结论**（见「档位处置」）

红线：**只读** —— 不写任何件、不删任何件、不碰 `dev edit` 的真写前置（人签批准件 / 审计先落盘 / 语法闸）。
"""

import argparse
import os
import re
import sys

EXIT_OK, EXIT_RED, EXIT_NOCONCL = 0, 1, 2

CLASS_EDIT = "改"
CLASS_SYMBOL = "符号级删"
CLASS_FILE = "件级（整件）"
TIER_RECORD, TIER_REPORT, TIER_BLOCK = "只记", "只报", "必拦"

# 判据 1 的那张表（**唯一真源**：允许的 `class ↔ tier` 组合）
ALLOWED_TIER = {
    CLASS_EDIT: {TIER_RECORD},
    CLASS_SYMBOL: {TIER_BLOCK},
    CLASS_FILE: {TIER_REPORT, TIER_BLOCK},
}
CRITERIA = ["untracked", "no_archive_copy", "outside_repo"]
CRITERIA_CN = {"untracked": "未跟踪件", "no_archive_copy": "无归档副本", "outside_repo": "仓外件"}

HOOK_RE = re.compile(r"^impact_hook\s+(.*)$", re.M)
CRIT_RE = re.compile(r"^impact_criteria\s+(.*)$", re.M)


def kvs(seg):
    out = {}
    for tok in seg.strip().split():
        if "=" in tok:
            k, v = tok.split("=", 1)
            out[k] = v
    return out


def parse_plan(text):
    """解一份干跑单：`impact_hook` 行与**紧随其后**的 `impact_criteria` 行配成一件。"""
    items = []
    hooks = list(HOOK_RE.finditer(text))
    for i, m in enumerate(hooks):
        end = hooks[i + 1].start() if i + 1 < len(hooks) else len(text)
        seg = text[m.end():end]
        cm = CRIT_RE.search(seg)
        if not cm:
            items.append({"hook": kvs(m.group(1)), "crit": None, "err": "这一件**没有 `impact_criteria` 行**（判据面缺格 ⇒ 判不了）"})
            continue
        items.append({"hook": kvs(m.group(1)), "crit": kvs(cm.group(1)), "err": ""})
    return items


def judge(items):
    reds, notes, noc = [], [], []
    if not items:
        noc.append("干跑单里**一条 `impact_hook` 都没有**（0 件）⇒ 不给结论（判据 7：空转不是绿）")
        return reds, notes, noc
    for it in items:
        h, c = it["hook"], it["crit"] or {}
        rel = h.get("rel", "?")
        cls, tier = h.get("class", ""), h.get("tier", "")
        if it["err"]:
            noc.append("%s：%s" % (rel, it["err"]))
            continue
        if cls not in ALLOWED_TIER:
            noc.append("%s：`class=%s` 不在闭集 {改, 符号级删, 件级（整件）} ⇒ 判不了 ⇒ 不给结论" % (rel, cls))
            continue
        if tier not in ALLOWED_TIER[cls]:
            reds.append("判据1/3 %s：`class=%s` 配 `tier=%s` —— **落在分档表外**（这张表是已拍口径；"
                        "照字面版「删=必拦不分档」或把必拦档改写成只报，都在这里红）" % (rel, cls, tier))
            continue
        hits = [k for k in CRITERIA if c.get(k) == "yes"]
        if cls == CLASS_SYMBOL:
            reds.append("判据2 %s：**符号级删 ⇒ 必拦**（不可逆 · 这一档不查可找回证据）· tier=%s" % (rel, tier))
            continue
        if cls == CLASS_EDIT:
            notes.append("%s：改 ⇒ 只记（可逆 ⇒ 不许红）" % rel)
            continue
        # 件级（整件）
        if hits or h.get("retrievable") == "no":
            who = "、".join(CRITERIA_CN[k] for k in hits) if hits else "可找回证据不成立（retrievable=no）"
            reds.append("判据5 %s：件级 + **不可找回** ⇒ 升必拦 ⇒ 红 · 命中：%s（criteria_hits=%s）"
                        % (rel, who, c.get("criteria_hits", "?")))
            continue
        if h.get("retrievable") != "yes":
            noc.append("%s：件级但 `retrievable` 既不是 yes 也不是 no（取值 %r）⇒ 判不了 ⇒ 不给结论"
                       % (rel, h.get("retrievable")))
            continue
        # 判据 4：只报要求**两件证据都在**（git 历史 / 归档副本 至少一件，且件被跟踪）
        has_git = h.get("git_sha", "-") not in ("", "-")
        has_arch = bool(h.get("archive", "")) and h.get("archive") != "-"
        if c.get("untracked") == "yes" or not (has_git or has_arch):
            reds.append("判据5/7 %s：`tier=只报` 但**可找回证据缺件**（tracked=%s · git_sha=%s · archive=%s）"
                        "⇒ 必红（「缺一不许删」）" % (rel, h.get("tracked", "?"), h.get("git_sha", "-"), h.get("archive", "-")))
            continue
        notes.append("%s：件级 + 可找回（git_sha=%s · archive=%s）⇒ 只报（不许红）"
                     % (rel, h.get("git_sha", "-"), h.get("archive", "-") or "-"))
    return reds, notes, noc


def verdict_face(reds, noc, items):
    """判据 9：回执必带「判定档 + 条数」两格。"""
    tiers = [it["hook"].get("tier", "?") for it in items if not it["err"]]
    face = "混合" if len(set(tiers)) > 1 else (tiers[0] if tiers else "无对象")
    return "判定档=%s · 条数=%d" % (face, len(items))


def self_test():
    bad = 0
    total = 0

    def plan(tier, cls, rel="a/b.go", retrievable="yes", tracked="yes", archive="Zerg-归档/x",
             git_sha="deadbeef", outside="no", **crit):
        c = {"untracked": "no", "no_archive_copy": "no", "outside_repo": "no", "criteria_hits": "0"}
        c.update(crit)
        return ("something\nimpact_hook tier=%s class=%s retrievable=%s tracked=%s archive=%s git_sha=%s outside=%s rel=%s\n"
                "impact_criteria untracked=%s no_archive_copy=%s outside_repo=%s criteria_hits=%s\n"
                % (tier, cls, retrievable, tracked, archive, git_sha, outside, rel,
                   c["untracked"], c["no_archive_copy"], c["outside_repo"], c["criteria_hits"]))

    def case(name, want, text):
        nonlocal bad, total
        total += 1
        items = parse_plan(text)
        again = parse_plan(text)
        r1 = judge(items)
        r2 = judge(again)
        rc = EXIT_NOCONCL if r1[2] else (EXIT_RED if r1[0] else EXIT_OK)
        # 判据 1：两跑逐字一致
        same = (r1 == r2)
        ok = (rc == want) and same
        if not ok:
            bad += 1
        why = "" if ok else " ｜ " + ("两跑不一致" if not same else (r1[0][0] if r1[0] else str(r1[2])))
        print("  %s %s ⇒ rc=%d（期望 %d）%s" % ("✓" if ok else "✗", name, rc, want, why))

    case("① 符号级删（分档=必拦）⇒ 红", EXIT_RED, plan(TIER_BLOCK, CLASS_SYMBOL))
    case("② 符号级删被改写成「只报」（字面版/表外组合）⇒ 红", EXIT_RED, plan(TIER_REPORT, CLASS_SYMBOL))
    case("③ 件级 + 可找回 ⇒ 只报 ⇒ **不许红**", EXIT_OK, plan(TIER_REPORT, CLASS_FILE))
    case("③′ 成对：件级 + 无归档副本 ⇒ 升必拦 ⇒ 红", EXIT_RED, plan(TIER_BLOCK, CLASS_FILE, no_archive_copy="yes", criteria_hits="1"))
    case("④ 件级 + 未跟踪 ⇒ 升必拦 ⇒ 红", EXIT_RED, plan(TIER_BLOCK, CLASS_FILE, tracked="no", untracked="yes", criteria_hits="1"))
    case("⑤ 件级 + 仓外件 ⇒ 升必拦 ⇒ 红", EXIT_RED, plan(TIER_BLOCK, CLASS_FILE, outside="yes", outside_repo="yes", criteria_hits="1"))
    case("⑥ 件级 + `retrievable=no` ⇒ 升必拦 ⇒ 红", EXIT_RED, plan(TIER_BLOCK, CLASS_FILE, retrievable="no"))
    case("⑦ 件级 + 只报但**证据缺件**（无归档副本且无 git 历史）⇒ 红", EXIT_RED,
         plan(TIER_REPORT, CLASS_FILE, archive="-", git_sha="-"))
    case("⑧ 改 ⇒ 只记 ⇒ **不许红**", EXIT_OK, plan(TIER_RECORD, CLASS_EDIT))
    case("⑨ 空单（0 条 `impact_hook`）⇒ 不给结论", EXIT_NOCONCL, "没有单\n")
    # 判据 8：静态扫本件源码 —— **真调用**里 0 个写动作（用 `ast`，免得把判词/注释里那几个词当命中）
    total += 1
    import ast
    tree = ast.parse(open(os.path.abspath(__file__), encoding="utf-8").read())
    writes = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        f = node.func
        name = f.id if isinstance(f, ast.Name) else (f.attr if isinstance(f, ast.Attribute) else None)
        if name == "open":
            mode = None
            if len(node.args) > 1 and isinstance(node.args[1], ast.Constant):
                mode = node.args[1].value
            for kw in node.keywords:
                if kw.arg == "mode" and isinstance(kw.value, ast.Constant):
                    mode = kw.value.value
            if isinstance(mode, str) and any(ch in mode for ch in "wa+"):
                writes.append("open(mode=%r)" % mode)
        elif name in ("remove", "unlink", "rmtree", "move", "rename", "chmod", "write", "write_text",
                      "write_bytes", "makedirs", "mkstemp", "NamedTemporaryFile", "O_WRONLY"):
            writes.append(name)
    if writes:
        bad += 1
        print("  ✗ ⑧ 判据 8：本件源码里出现 %d 处写动作调用（必须 0）：%s" % (len(writes), "、".join(sorted(set(writes)))))
    else:
        print("  ✓ ⑧ 判据 8：本件源码 0 处写动作调用（`ast` 扫真调用：`open(w/a/+)` / `os.remove` / `shutil.move` 等）")
    print("  ── 自检：%d 格 · %d 格不过" % (total, bad))
    return EXIT_OK if bad == 0 else EXIT_NOCONCL


def resolve_plan_source(a):
    if a.plan:
        return a.plan, "显式（--plan）"
    if os.environ.get("ZERG_R28_PLAN"):
        return os.environ["ZERG_R28_PLAN"], "环境变量（ZERG_R28_PLAN）"
    st = os.environ.get("ZERG_STATE_DIR") or os.path.join(os.path.expanduser("~"), ".zerg", "state")
    return os.path.join(st, "r28-plan.txt"), "默认落点（状态目录）"


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。

    ⚠ 本门「判据 8」用 `ast` 静态扫本件源码、**禁任何写动作调用**（含 `sys.stderr.write`）⇒
       这里的提示一律走 `print(file=sys.stderr)`（`print` 不在写动作集合里），别改成 `.write(...)` ✗
    """
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
    ap.add_argument("--plan", default="")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        print("门㉜ 删前波纹门（`R28` 分档版）—— 判据：")
        print("  改 ⇒ 只记（不许红）｜ 符号级删 ⇒ 必拦（一律红）｜ 件级 ⇒ 可找回只报 / 找不回升必拦")
        print("  保险三条：未跟踪件 · 无归档副本 · 仓外件（任一命中 ⇒ 升必拦）")
        print("  ★ 拒**字面版**「删=必拦不分档」（被覆盖的那半句）· 断电：0 写盘 · 不碰写前置")
        return EXIT_OK
    if a.self_test:
        print("── 自检（合成夹具 · 9 格成对负控 + 判据 8 静态自证）──")
        rc = self_test()
        print("自检结论: %s" % ("全过" if rc == 0 else "不过（拒绝扫真目标）"))
        return rc

    src, how = resolve_plan_source(a)
    explicit = bool(a.plan) or bool(os.environ.get("ZERG_R28_PLAN"))
    print("门㉜ 删前波纹门（`R28` · 分档版）· 单的来源：%s（%s）" % (src, how))
    # ★ 缺口 `GAP-20260927-70`（同族 · 本门此前对该变量**静默取用**）：默认落点走 `ZERG_STATE_DIR`
    #   ⇒ 别会话把它留在环境里指向沙箱时，本门读的是**沙箱**状态目录下的单。照
    #   `check-trustroot-witness.py` 的同形警示：未显式给单而环境变量赢 ⇒ 逐字声明这是沙箱读数。
    #   （走 `print(file=sys.stderr)`：本件判据 8 用 ast 静态扫源码、**禁任何写动作调用**，`print` 不在集合里。）
    if not explicit and os.environ.get("ZERG_STATE_DIR"):
        print("  ★★ 单的落点是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"
              "**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--plan`）"
              % os.environ["ZERG_STATE_DIR"], file=sys.stderr)
    if src == "-":
        text = sys.stdin.read()
    else:
        try:
            with open(src, encoding="utf-8", errors="replace") as f:
                text = f.read()
        except OSError:
            if explicit:
                print("  ⛔ 指名的那份干跑单读不到（%s）⇒ **不给结论**（判据 7：缺件不许当绿）" % src)
                return EXIT_NOCONCL
            print("  空档：本趟**没有删面单**（%s 不在）⇒ 本门这一趟**没有对象可判**" % src)
            print("        ⚠ 照实点名：这是**空档**，不是绿 —— 判据 7 的字面（没单 ⇒ 2）在本步会恒 BLOCKED，"
                  "故取值取可运行（判词逐字写出）")
            print("  回执：判定档=无对象 · 条数=0")
            return EXIT_OK
    items = parse_plan(text)
    reds, notes, noc = judge(items)
    for n in notes:
        print("  · %s" % n)
    for r in reds:
        print("  ✗ %s" % r)
    for m in noc:
        print("  ⛔ %s" % m)
    print("  回执：%s" % verdict_face(reds, noc, items))
    if noc:
        print("⇒ **不给结论**（rc=2）：%d 条" % len(noc))
        return EXIT_NOCONCL
    if reds:
        print("⇒ 红（rc=1）：%d 条（必拦档：不可逆的删不许静默过）" % len(reds))
        return EXIT_RED
    print("⇒ 全绿（rc=0）：本趟删面全部落在分档表内（只记/只报档不许红）")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

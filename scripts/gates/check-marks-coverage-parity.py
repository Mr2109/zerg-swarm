#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-marks-coverage-parity.py —— 「标记表域 ⟷ 门覆盖面域」对拍 + 入库前两器排除预警（只读一致性门）

一句话：**断言「标记表里出现过的域」与「门覆盖面声明的域」是同一批**，并且在**入库前**就把
「表判 `public=no`、两器排除表却没收它」的件点名出来 —— 今天这两件事**没有任何门在管**。

为什么存在（两笔实测报出的两处无兜底，不是假想）

  ⑴ **域的真源有两份，口径一隐一显，无任何一致性判据** ✗
     · `scripts/公开标记.tsv` 的「域」是**隐式**的：表里没有域列，域 = 路径第二段
       （`scripts/<域>/…`）。表侧现读 **13** 个域（含分域前的工作目录）。
     · `scripts/gates/gate-coverage.config.json` 的 `scan.script_globs` 是**显式列**：
       每域一对 `"scripts/<域>/*.sh"` + `"scripts/<域>/*.py"`，另加两条顶层 glob ⇒ 现读 **10** 个域。
     · 两处**逐域对拍**：`check-gate-coverage.py` 全文不引 `公开标记.tsv`（0 命中 ✓），
       `check-public-marks.py` 的域面**由 `_derive_domains()` 派生**（`DOMAINS`，不再写死）——
       ⇒ 「门扫了这个域、表里一件都没登记」（新域只加 globs）与「表登记了这个域、门不扫它」
       这两向**谁都不会红**，全靠人记着。

  ⑵ **`public=no` 的件在 `git add` 那一瞬间才会让判据⑥ 红，而入库前没有任何门预警** ✗
     实证（背景记录）：`scripts/dev/isolate-ro.sh` 未跟踪时 `check-publish-face-sync.py` **判据⑥
     rc=0**（`git ls-files` 只看得见已跟踪件）⇒ **一入库即成 1 条红**（「表说 no、链上仍会发」）。
     今天这一批 26 条未跟踪的 `public=no` 件里已有一条（`scripts/dev/isolate-ro.sh`）**两器排除表
     一处都没有** —— 它入库那一刻 ⑥ 就红，而 ⑥ 不在提交闸的快速档里 ⇒ 红在提交之后才被看见。

判据（**只读** · 三档退码 0/1/2 · 与仓内其余门脚本同口径）

  R1 **域对拍**（域 = `scripts/<域>/` 的第二段；顶层 glob `scripts/*.<ext>` 记为域「顶层」）
     · 表侧域集 A = `公开标记.tsv` 里 `scripts/<域>/…` 行的 `<域>` 集；
     · 门侧域集 B = `gate-coverage.config.json` 的 `script_globs` 里 `scripts/<域>/*` 的 `<域>` 集。
     ★ **只数「实测有件」的 glob**：`scripts/*.sh` / `scripts/*.py` 这类顶层 glob 今天在盘上
       **一件都匹配不到** ⇒ 它不是「覆盖了一个域」，而是**空指 glob** —— 单列出来（见「空指 glob」格），
       **不计进 B**。否则本门会对着一个不存在的域报假红（口径同 `check-gate-coverage.py` 的空转判据）。
     · **R1-a 判红**：B ∖ A 非空 ⇒ 逐域点名（门扫这个域、表里一件都没登记 ⇒ 该域件对「标记面」不存在）；
     · **R1-b 判红**：(A ∖ B) 里**未登记豁免**的域 ⇒ 逐域点名（表登记了这个域、门不扫它 ⇒ 无对拍）；
     · **登记豁免**（不入退码 · 计数可见 · 逐条 `reason` + `date`）：`scripts/` 下**面外**的域
       （分域第二段之外的旧工作目录）本就由表逐件登记为 `public=no`，不属于「门覆盖面」——
       它们必须在「域豁免登记表」里逐域写明为什么（缺 `reason` 或缺 `date` ⇒ **rc=2**，
       口径同 `check-gate-coverage.py` 的白名单：写不清理由的豁免就是洗白）；
       **悬空豁免**（豁免了、但已不在 A ∖ B 里）⇒ 照报、不改退码（豁免面漂移看得见）。
       ★ 不为凑绿放宽：域豁免表**只收面外域**，面内域（见下「覆盖缺口」格）一律不许登记进去。

  R2 **入库前两器排除预警**（逐件核 tsv 判 `public=no` 的行；两侧 = 两器各自的排除面）
     · **侧甲** = `scripts/build/publish-public.sh` 的 `EXCLUDES`（B8 声明的**单一真源**；
       解析器**复用** `check-history-secrets.py` 的 `load_excludes` / `excluded`，绝不另抄一份）；
     · **侧乙** = `publish/mirror-public-lib.py` 的 `DROP_EXACT` / `DROP_PREFIX`（镜像器**自带**那张表）。
     · **有效判定**（与判据⑥ 同口径）：两器共用的那把闸是侧甲 ⇒ **缺侧甲 = 两器都发**。
     · **R2-a 判红**：**已跟踪** 且 缺侧甲 且 落在导出白名单候选集内 ⇒ 逐件点名
       （＝ `check-publish-face-sync.py` 判据⑥ 会红的那一批的**预演**：表行号 · 件 · 缺哪一侧）；
     · **R2-b 点名预警（不入退码）**：**未在 git 索引** 且 缺任一侧 ⇒ 逐件点名
       「`git add` 那一刻判据⑥ 会红」（这正是今天无兜底的那一格；今天无红 ⇒ 照 `check-public-marks.py`
       D 档「只报告」的先例，只点名、不进总 rc）；
     · **R2-c 点名预警（不入退码）**：侧甲 与 侧乙 不一致（只在一侧）⇒ 点名「两器清单漂移」
       （有效判定仍安全，故不入退码 —— 但两器清单必须逐条一致，漂移要看得见）。

覆盖缺口（**只报数 · 不入退码**，今天 = 1 个域）
   门侧域里**不在** `check-public-marks.py` 派生的域面（`DOMAINS`）内的域：该域的件由本门的 R1 看得见，
   而「漏登记」（该门 B2）**看不见它** ⇒ 该域件的「表里有没有它」今天无人管。
   ★ 本笔**只报**、不判红、**不改那份派生域面**（那是另一件判据的真源，改它 = 改判据 ⇒ 必须走拍板）。

单一真源（一律复用，绝不另抄一份解析器）
   · 标记表解析 —— `check-public-marks.py` 的 `load_table` / `TableError`（表不合规 ⇒ rc=2，A 档口径）
   · EXCLUDES 解析 + 白名单解析 —— `check-history-secrets.py` 的 `load_excludes` / `excluded` /
     `load_whitelist` / `published_path`
   · 镜像器自带丢弃表 —— `publish/mirror-public-lib.py` 的 `DROP_EXACT` / `DROP_PREFIX`
   · 「域」的口径 —— 本门逐字照 `check-public-marks.py` 的 B2（域 = `scripts/<域>/` 的第二段）

用法
    python3 scripts/gates/check-marks-coverage-parity.py                    # 判真目标（仓根）
    python3 scripts/gates/check-marks-coverage-parity.py --root <树>        # 判别的树（自检夹具同入口）
    python3 scripts/gates/check-marks-coverage-parity.py --max-examples 40
    python3 scripts/gates/check-marks-coverage-parity.py --self-test        # 只跑成对负控自检（仓外夹具）

退出码（三档 · **优先级 2 > 1 > 0**）
    0 = 全绿（R1-a/R1-b/R2-a 全空；R2-b/R2-c 只点名，不入退码）
    1 = **有红**（R1-a 缺域 / R1-b 未登记的多余域 / R2-a 已跟踪件缺侧甲）
    2 = **不给结论**（用法错 / 缺件 / **空转**：表零数据行 · 门侧域集为空 · `public=no` 零行 ·
        非 git 树 · 豁免条目缺 reason|date / 自检未过）—— 「判不了」时不给结论 ≠ 绿

纪律：**只读**（除 /tmp 夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · **不改任何已有文件** ·
      本门**无机器读面**（`--json` 一律拒，见文末 `Q-176`）· 挂法由父代理在提交闸上另拍（本笔**不挂** ✗）
"""
import argparse
import glob as globmod
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))                       # scripts/gates/ → 仓根

TABLE_REL = "scripts/公开标记.tsv"
CONFIG_REL = "scripts/gates/gate-coverage.config.json"
PUBLISH_SH_REL = "scripts/build/publish-public.sh"
MIRROR_LIB_REL = "publish/mirror-public-lib.py"
WHITELIST_REL = "publish/whitelist.txt"
MARKS_REL = "scripts/gates/check-public-marks.py"       # 表解析器（单一真源 · 复用）
HSEC_REL = "scripts/gates/check-history-secrets.py"     # EXCLUDES / 白名单解析器（单一真源 · 复用）

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

TOP = ""      # 顶层 glob（`scripts/*.sh`）的「域」标记：顶层（无第二段）

# ── 域豁免登记表（**例外表一档** · 逐条 reason + date；缺任一 ⇒ rc=2，不是红）────────────────────
# 口径（写死在这里，不另抄一份）：只收 **`scripts/` 下的面外域** —— 即不在
#   `check-public-marks.py` 的派生域面（`DOMAINS`）内、由表**逐件**登记为 `public=no` 的旧工作目录。
#   为什么它们可以豁免 R1-b：门覆盖面（config 的 script_globs）定义的是「**过门禁的脚本**」那一批，
#   面外域从来就不在门覆盖面里（`check-gate-coverage.py` 的硬排除同口径）⇒ 两边本就不同批是**设计**。
#   ★ 面内域（`DOMAINS` ＋ `dev`）**一律不许**登记进本表：它们必须与门侧域集逐域对齐（那正是 R1 的牙齿）。
EXEMPT_DOMAINS = (
    {"domain": "exportnames",
     "reason": "面外域·分域前的工作目录（Go module，非门覆盖面）：表逐件登记 public=no，"
               "gate-coverage.config.json 的 script_globs 从设计上不扫它",
     "date": "2026-09-26"},
    {"domain": "sandbox-probes",
     "reason": "面外域·分域前的实验目录（18 件探针）：表逐件登记 public=no，门覆盖面按设计不含它",
     "date": "2026-09-26"},
    {"domain": "plugins",
     "reason": "面外域·插件示例件（scripts/plugins/zerg-hello）：表逐件登记 public=no，"
               "不属「过门禁的脚本」那一批",
     "date": "2026-09-26"},
)


class Unsupported(Exception):
    """判不了（缺件 / 空转 / 用法错）⇒ rc=2（不给结论）。"""


def load_module(name, path):
    """复用仓内既有解析器（单一真源）；缺件/加载失败 ⇒ rc=2，绝不静默跳过。"""
    if not os.path.isfile(path):
        print("✗ 缺少要复用的资产：%s（本门不接受缺件）" % path, file=sys.stderr)
        raise SystemExit(EXIT_BLOCKED)
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        print("✗ 无法加载 %s" % path, file=sys.stderr)
        raise SystemExit(EXIT_BLOCKED)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


# ── 表侧：域集（域 = `scripts/<域>/` 的第二段 · 逐字照 check-public-marks.py 的 B2 口径）──────
def table_domains(rows):
    """⇒ (域→[件…], 顶层件数)。域只从 `scripts/<域>/…`（≥3 段）抽；`scripts/<件>`（2 段）= 顶层。"""
    doms, top_level = {}, []
    for _i, rel, _pub, _why in rows:
        p = rel.split("/")
        if p[0] != "scripts":
            continue
        if len(p) >= 3:
            doms.setdefault(p[1], []).append(rel)
        else:
            top_level.append(rel)
    return doms, top_level


# ── 门侧：域集（从 scan.script_globs 抽；**只数实测有件的 glob**）────────────────────────────
def glob_domains(root, globs):
    """⇒ (域集, 空指 glob 列表, 域→匹配件数)。

    ★ 为什么只数「实测有件」的：`scripts/*.sh` / `scripts/*.py` 今天在盘上 0 命中 ⇒
      把它当「覆盖了顶层这个域」会让本门对着一个不存在的域报假红（假红 = 判据没有区分度）。
      空指 glob 单列出来（空指 = 声明了覆盖面却一件都匹配不到，是真缺口，只是不属 R1 的域对拍）。
    """
    doms, empty, counts = set(), [], {}
    for g in globs:
        if not isinstance(g, str) or not g.strip():
            raise Unsupported("script_globs 里有非字符串/空条目：%r" % (g,))
        g = g.strip()
        if not g.startswith("scripts/"):
            raise Unsupported("script_globs 条目不在 scripts/ 面：%r（本门的面 = scripts/）" % g)
        p = g.split("/")
        dom = p[1] if len(p) >= 3 and "*" not in p[1] else TOP
        n = len(globmod.glob(os.path.join(root, g)))
        if n == 0:
            empty.append(g)
            continue
        doms.add(dom)
        counts[dom] = counts.get(dom, 0) + n
    return doms, empty, counts


def tracked_files(root):
    """git 索引里的件（`-c core.quotepath=false`：非 ASCII 名不许被转义成 `"scripts/\\345…"`）。"""
    r = subprocess.run(["git", "-C", root, "-c", "core.quotepath=false", "ls-files"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise Unsupported("git ls-files 失败：%s（rc=%d ⇒ 跟踪面读不到 = 判不了，缺件 ≠ 绿）"
                          % (root, r.returncode))
    return [ln for ln in r.stdout.splitlines() if ln]


def load_marks_rows(table):
    """表 ⇒ [(行号, 件, public, 理由)]；表不合规 ⇒ Unsupported（A 档口径：坏表的绿没有意义）。"""
    if not os.path.isfile(table):
        raise Unsupported("标记表不在：%s" % table)
    path = os.path.join(REPO, MARKS_REL)
    marks = load_module("parity_marks", path)
    try:
        return marks.load_table(table)
    except marks.TableError as e:
        raise Unsupported("标记表不合规：%s" % e)
    except AttributeError:
        raise Unsupported("%s 里没有 load_table/TableError（单一真源改名了 ⇒ 本门判不了）" % MARKS_REL)


def load_globs(cfg_path):
    if not os.path.isfile(cfg_path):
        raise Unsupported("config 不在：%s" % cfg_path)
    try:
        cfg = json.load(open(cfg_path, encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise Unsupported("config 读不了/不是合法 JSON：%s（%s）" % (cfg_path, e))
    globs = (cfg.get("scan") or {}).get("script_globs")
    if not isinstance(globs, list) or not globs:
        raise Unsupported("config 的 scan.script_globs 不是非空列表：%r ⇒ 门侧域集读不到 = 空转，拒绝给结论"
                          % (globs,))
    return globs


def check_exempt(entries):
    """域豁免登记表自身合规：每条必须有 domain + reason + date（缺任一 ⇒ rc=2，不是红）。"""
    bad, doms = [], {}
    for k, ent in enumerate(entries, 1):
        if not isinstance(ent, dict):
            bad.append("第 %d 条不是映射：%r" % (k, ent))
            continue
        d, why, when = ent.get("domain"), ent.get("reason"), ent.get("date")
        if not d:
            bad.append("第 %d 条缺 domain" % k)
            continue
        if not why:
            bad.append("第 %d 条（%s）缺 reason（写不清理由的豁免就是洗白）" % (k, d))
        if not when:
            bad.append("第 %d 条（%s）缺 date" % (k, d))
        if d in doms:
            bad.append("域重复登记：%s（第 %d 条与第 %d 条）" % (d, doms[d], k))
        doms[d] = k
    return bad, doms


def run(root=REPO, table=None, config=None, max_examples=40, out=None, exempt=None):
    """判一遍 ⇒ 退出码（0 全绿 · 1 有红 · 2 不给结论）。**只读**，不写仓。"""
    out = out or sys.stdout
    exempt = EXEMPT_DOMAINS if exempt is None else exempt
    table = table or os.path.join(root, TABLE_REL)
    config = config or os.path.join(root, CONFIG_REL)
    publish_sh = os.path.join(root, PUBLISH_SH_REL)
    mirror_lib = os.path.join(root, MIRROR_LIB_REL)
    whitelist = os.path.join(root, WHITELIST_REL)
    if not os.path.isdir(root):
        raise Unsupported("树根不存在：%s" % root)

    def dump(title, items, tail=""):
        out.write("\n── %s：%d 处%s\n" % (title, len(items), tail))
        for x in items[:max_examples]:
            out.write("   %s\n" % x)
        if len(items) > max_examples:
            out.write("   …另 %d 处\n" % (len(items) - max_examples))

    # ── 真源加载（一律复用；缺件/坏件 ⇒ rc=2）─────────────────────────────────────
    rows = load_marks_rows(table)
    globs = load_globs(config)
    exempt_bad, exempt_map = check_exempt(exempt)
    if exempt_bad:
        for b in exempt_bad:
            print("✗ 域豁免登记表不合规：%s" % b, file=sys.stderr)
        print("⇒ 不给结论（rc=2）：豁免写不清 = 判不了（不静默放绿）", file=sys.stderr)
        return EXIT_BLOCKED
    hsec = load_module("parity_hsec", os.path.join(REPO, HSEC_REL))
    for p in (publish_sh, mirror_lib, whitelist):
        if not os.path.isfile(p):
            print("✗ 缺件：%s ⇒ 不给结论（rc=2）" % p, file=sys.stderr)
            return EXIT_BLOCKED
    excludes = hsec.load_excludes(publish_sh)
    if not excludes:
        print("✗ 侧甲：从 %s 解析不出任何 EXCLUDES 条目 ⇒ 不给结论（rc=2）" % publish_sh, file=sys.stderr)
        print("   （「清单没了所以全绿」是最难发现的一类假绿）", file=sys.stderr)
        return EXIT_BLOCKED
    entries = hsec.load_whitelist(whitelist)
    if not entries:
        print("✗ 白名单解析为空：%s ⇒ 不给结论（rc=2）" % whitelist, file=sys.stderr)
        return EXIT_BLOCKED
    mirror = load_module("parity_mirror", mirror_lib)
    drop_exact = getattr(mirror, "DROP_EXACT", None)
    drop_prefix = getattr(mirror, "DROP_PREFIX", None)
    if drop_exact is None or drop_prefix is None:
        print("✗ 侧乙：%s 里没有 DROP_EXACT / DROP_PREFIX ⇒ 不给结论（rc=2）" % mirror_lib, file=sys.stderr)
        return EXIT_BLOCKED

    # ── 域对拍：R1 ────────────────────────────────────────────────────────────
    a_doms, top_files = table_domains(rows)
    b_doms, empty_globs, b_counts = glob_domains(root, globs)
    if not b_doms:
        print("✗ 门侧域集为空（script_globs %d 条一条都没匹配到件）⇒ 空转，拒绝给结论（rc=2）" % len(globs),
              file=sys.stderr)
        return EXIT_BLOCKED
    if not a_doms:
        print("✗ 表侧域集为空（表里没有一条 `scripts/<域>/…` 行）⇒ 空转，拒绝给结论（rc=2）", file=sys.stderr)
        return EXIT_BLOCKED

    r1_a = sorted(b_doms - set(a_doms))                         # 门扫的域、表里一件都没登记
    extra = sorted(set(a_doms) - b_doms)                        # 表登记的多余域
    r1_b = [d for d in extra if d not in exempt_map]            # 未登记的 ⇒ 红
    exempt_hit = [d for d in extra if d in exempt_map]          # 登记豁免 ⇒ 报数
    exempt_dangle = [d for d in sorted(exempt_map) if d not in extra]   # 悬空豁免 ⇒ 照报不改退码
    face_doms = set(getattr(load_module("parity_marks_face", os.path.join(REPO, MARKS_REL)), "DOMAINS", ()))
    gap = sorted(d for d in b_doms if face_doms and d not in face_doms)

    # ── 两器排除面：R2 ────────────────────────────────────────────────────────
    tracked = set(tracked_files(root))
    no_rows = [(i, rel, why) for i, rel, pub, why in rows if pub == "no"]
    if not no_rows:
        print("✗ 标记表里一条 `public=no` 都没有 ⇒ R2 无区分度（空转 = 假覆盖）⇒ 拒绝给结论（rc=2）",
              file=sys.stderr)
        return EXIT_BLOCKED
    r2_a, r2_b, r2_c = [], [], []
    for i, rel, why in no_rows:
        side_a = bool(hsec.excluded(rel, excludes))                  # 侧甲：旧器闸（两器共用）
        side_b = (rel in drop_exact) or any(rel.startswith(p) for p in drop_prefix)   # 侧乙：镜像器自带表
        if not side_a and not side_b:
            miss = "缺侧甲 + 侧乙（两器都会发它）"
        elif not side_a:
            miss = "缺侧甲（旧器闸没收 ⇒ 两器共用那把闸都放行）"
        elif not side_b:
            miss = "缺侧乙（镜像器自带表没登记）"
        else:
            miss = ""
        is_cand = bool(hsec.published_path(rel, entries))
        if rel in tracked and not side_a and is_cand:
            r2_a.append((i, rel, miss, why))
        elif rel not in tracked and miss:
            r2_b.append((i, rel, miss, is_cand))
        if miss and side_a != side_b:
            r2_c.append((i, rel, "只在一侧：" + ("侧甲有、侧乙无" if side_a else "侧乙有、侧甲无"), why))

    # ── 报告 ─────────────────────────────────────────────────────────────────
    out.write("标记表：%s（%d 行 · public=yes %d · public=no %d · 域 %d 个 · 顶层件 %d 件）\n"
              % (table, len(rows), len(rows) - len(no_rows), len(no_rows), len(a_doms), len(top_files)))
    out.write("门覆盖面：%s（script_globs %d 条 · 域 %d 个 · 空指 glob %d 条）\n"
              % (config, len(globs), len(b_doms), len(empty_globs)))
    out.write("\n══ R1 域对拍（域 = `scripts/<域>/` 的第二段）══\n")
    out.write("  表侧域（%d）：%s\n" % (len(a_doms), " · ".join(sorted(a_doms)) or "（空）"))
    out.write("  门侧域（%d）：%s\n" % (len(b_doms), " · ".join(sorted(d or "顶层" for d in b_doms))))
    out.write("  %s R1-a 门扫的域、表里一件都没登记：%d 个%s\n"
              % ("✗" if r1_a else "✅", len(r1_a), "　" + " · ".join(r1_a) if r1_a else ""))
    out.write("  %s R1-b 表登记的多余域（未登记豁免）：%d 个%s\n"
              % ("✗" if r1_b else "✅", len(r1_b), "　" + " · ".join(r1_b) if r1_b else ""))
    out.write("  · 登记豁免域（不入退码 · 逐条写在 EXEMPT_DOMAINS）：%d 个%s\n"
              % (len(exempt_map), "　" + " · ".join(sorted(exempt_map)) if exempt_map else "（无）"))
    if exempt_dangle:
        out.write("  · 悬空豁免（照报 · 不改退码）：%d 个　%s\n"
                  % (len(exempt_dangle), " · ".join(exempt_dangle)))
    if empty_globs:
        out.write("  · 空指 glob（声明了覆盖面却一件都匹配不到 ⇒ 不计进 B）：%d 条　%s\n"
                  % (len(empty_globs), " · ".join(empty_globs)))
    if gap:
        out.write("  · 覆盖缺口（**只报数 · 不入退码**）：门侧 %d 个域不在 `check-public-marks.py` 的派生域面（DOMAINS）"
                  "内 ⇒ 该域件的「漏登记」今天无人管：%s\n" % (len(gap), " · ".join(gap)))
    if r1_a:
        dump("R1-a 点名（门扫这个域、表里一件都没登记 ⇒ 补表 或 从 globs 撤掉该域 · 不许放宽本判据 ✗）",
             ["%s  ← `script_globs` 有 `scripts/%s/*.sh|*.py`（实测 %d 件），`公开标记.tsv` 里 0 行"
              % (d, d, b_counts.get(d, 0)) for d in r1_a])
    if r1_b:
        dump("R1-b 点名（表登记了这个域、门不扫它 ⇒ 补 `script_globs` 或按域豁免登记表写明为什么）",
             ["%s  ← 表里 %d 行 `scripts/%s/…`，`script_globs` 里 0 条" % (d, len(a_doms[d]), d)
              for d in r1_b])

    out.write("\n══ R2 两器排除面（tsv `public=no` %d 行 · 已跟踪 %d · 未在索引 %d）══\n"
              % (len(no_rows), sum(1 for _i, r, _w in no_rows if r in tracked),
                 sum(1 for _i, r, _w in no_rows if r not in tracked)))
    out.write("  侧甲 = `%s` 的 EXCLUDES（%d 条 · 两器共用那把闸）\n" % (PUBLISH_SH_REL, len(excludes)))
    out.write("  侧乙 = `%s` 的 DROP_EXACT %d 条 + DROP_PREFIX %d 条（镜像器自带表）\n"
              % (MIRROR_LIB_REL, len(drop_exact), len(drop_prefix)))
    out.write("  %s R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：%d 件\n" % ("✗" if r2_a else "✅", len(r2_a)))
    out.write("  · R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：%d 件\n" % len(r2_b))
    out.write("  · R2-c 两器清单漂移（只在一侧 · 不入退码）：%d 件\n" % len(r2_c))
    if r2_a:
        dump("R2-a 点名（补两器排除表 或 改表 · 不许放宽本判据凑绿 ✗）",
             ["%s（表第 %d 行）· %s" % (rel, i, miss) for i, rel, miss, _why in r2_a])
    if r2_b:
        dump("R2-b 入库前预警点名（这条件**一 `git add` 就会让判据⑥ 红**；入库前先补两器排除表）",
             ["%s（表第 %d 行）· %s%s ⇒ 入库那一刻判据⑥ 报「表说 no、链上却仍会发」"
              % (rel, i, miss, "" if is_cand else "　★ 且不在导出白名单候选集内（真入库也触不到 ⑥）")
              for i, rel, miss, is_cand in r2_b])
    if r2_c:
        dump("R2-c 点名（两器清单必须逐条一致；有效判定仍安全 ⇒ 只报）",
             ["%s（表第 %d 行）· %s" % (rel, i, miss) for i, rel, miss, _why in r2_c])

    bad = len(r1_a) + len(r1_b) + len(r2_a)
    out.write("\n合计：R1-a %d · R1-b %d · R2-a %d（以上判红）· R2-b 预警 %d · R2-c 漂移 %d（以上只报）\n"
              % (len(r1_a), len(r1_b), len(r2_a), len(r2_b), len(r2_c)))
    if bad:
        out.write("⇒ 违规 %d 处（rc=1）：① R1-a ⇒ 补表（附理由）或撤 glob；② R1-b ⇒ 补 glob 或走域豁免登记；\n"
                  "   ③ R2-a ⇒ 收进 `EXCLUDES` + `DROP_EXACT`，或**改表** —— **不许**放宽本判据来凑绿 ✗\n" % bad)
        return EXIT_FAIL
    out.write("⇒ 全绿（rc=0）：域两侧逐域对齐（豁免 %d 个已登记）· 已跟踪的 `no` 件两器都已排除 · "
              "未入库件预警 %d 条（只报）\n" % (len(exempt_map), len(r2_b)))
    return EXIT_OK


# ══════════════════════════════════════════════════════════════════
# 自检：四档（逐档负控 / 正控 / 真目标 / 退码 0-1-2 钳 / 防空绿档）· 仓外夹具 · 真命令行 + 真退码
# ══════════════════════════════════════════════════════════════════
SELF_PATH = os.path.abspath(__file__)
_TMPDIRS = []

TABLE_HEAD = "文件\tpublic\t理由\n"
# 自证用例条数（**写死**：少跑一条即断言失败 —— 照 check-public-marks.py 的口径）
WANT_CASES = 18
# 夹具侧甲：最小 publish-public.sh（只留 EXCLUDES 数组）
FIXTURE_PUBLISH_SH = """#!/usr/bin/env bash
# 夹具：最小旧器（只提供 EXCLUDES 数组；解析器是 check-history-secrets.py 的 load_excludes）
EXCLUDES=(
  "scripts/gates/excluded.py"
  "scripts/build/kept.py"
  "scripts/dev/covered.py"
)
"""
# 夹具侧乙：最小镜像器（只提供 DROP_EXACT / DROP_PREFIX）
FIXTURE_MIRROR = """# 夹具：最小镜像器（只提供本门读的两个名字）
DROP_PREFIX = ()
DROP_EXACT = {"scripts/gates/excluded.py", "scripts/build/kept.py"}
"""


def _mkroot(base, name):
    d = tempfile.mkdtemp(prefix=name + "-", dir=base)
    _TMPDIRS.append(d)
    return d


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _fixture(base, name, *, tsv_rows, globs, files, excludes=(), drop_exact=(),
             whitelist="scripts/\n", git=True, add=()):
    """造一棵最小树（表 + config + 两器排除表 + 白名单 + 盘上件）；`add` = 要 `git add` 的件。

    ★ 夹具全落**仓外**临时目录；`git init` 按需（R2 只看索引，不看提交）。
    """
    root = _mkroot(base, name)
    _write(os.path.join(root, TABLE_REL), TABLE_HEAD + "".join(tsv_rows))
    _write(os.path.join(root, CONFIG_REL),
           json.dumps({"schema": "gate-coverage/1", "baseline": 0,
                       "scan": {"script_globs": list(globs)},
                       "whitelist": {"build_dirs": [], "scripts": []}}, ensure_ascii=False, indent=2))
    ex = "".join('  "%s"\n' % e for e in excludes) or ""
    _write(os.path.join(root, PUBLISH_SH_REL), FIXTURE_PUBLISH_SH.replace(
        '  "scripts/gates/excluded.py"\n  "scripts/build/kept.py"\n  "scripts/dev/covered.py"\n', ex))
    _write(os.path.join(root, MIRROR_LIB_REL),
           FIXTURE_MIRROR.replace('{"scripts/gates/excluded.py", "scripts/build/kept.py"}',
                                  "{%s}" % ", ".join('"%s"' % d for d in drop_exact)))
    _write(os.path.join(root, WHITELIST_REL), whitelist)
    for rel in files:
        _write(os.path.join(root, rel), "#!/usr/bin/env python3\nprint('fixture')\n")
    if git:
        subprocess.run(["git", "init", "-q"], cwd=root, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL)
        for rel in add:
            subprocess.run(["git", "add", "-f", rel], cwd=root, stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL)
    return root


def _run(root):
    """跑**真命令行**（子进程 ⇒ 真退出码），rc + 输出。"""
    proc = subprocess.run([sys.executable, SELF_PATH, "--root", root],
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return proc.returncode, (proc.stdout.decode("utf-8", "replace")
                             + proc.stderr.decode("utf-8", "replace"))


# 基础夹具件（B = {build, gates} · 表侧 A = {build, gates, exportnames(豁免)}）
BASE_FILES = ("scripts/build/kept.py", "scripts/gates/excluded.py", "scripts/exportnames/plug.py")
BASE_GLOBS = ("scripts/*.sh", "scripts/*.py",
              "scripts/build/*.sh", "scripts/build/*.py",
              "scripts/gates/*.sh", "scripts/gates/*.py")
BASE_TSV = ("scripts/build/kept.py\tyes\tA 类：合成夹具（构建链）\n"
            "scripts/gates/excluded.py\tno\tB 类：合成夹具（门禁）\n"
            "scripts/exportnames/plug.py\tno\t面外域·合成夹具（豁免登记表里那一个）\n")


def self_test():
    """四档自检：逐档负控（rc=1）＋ 正控（rc=0）＋ 真目标（现读计数）＋ 退码 0-1-2 钳 ＋ 防空绿档。

    自带工作区隔离（只用临时目录 · 仓外夹具）· 负控格逐格贴真退出码与点名。
    """
    d = tempfile.mkdtemp(prefix="mcp-selftest-")
    _TMPDIRS.append(d)
    bad = 0
    total = 0

    def case(tag, desc, want_rc, want_text, root):
        nonlocal bad, total
        total += 1
        rc, text = _run(root)
        miss = [s for s in want_text if s not in text]
        ok = (rc == want_rc) and not miss
        bad += 0 if ok else 1
        print("  %s %s %s\n       rc=%d（期望 %d）· 缺字样 %s"
              % ("✓" if ok else "✗", tag, desc, rc, want_rc, miss or "无"))
        return rc, text

    print("── 自检（合成夹具 · 仓外临时目录 · 不碰真目标）──")
    # 夹具基线的两器排除面：**把表里判 `no` 的件全收齐** ⇒ 正控/成对格据此为绿
    BASE_EXCLUDES = ("scripts/gates/excluded.py", "scripts/exportnames/plug.py")
    DEV_ROW = "scripts/dev/lone.py\tno\tB 类：合成夹具（dev 域）\n"
    DEV_GLOBS = ("scripts/dev/*.sh", "scripts/dev/*.py")

    def fx(name, *, tsv=None, globs=None, files=None, excludes=BASE_EXCLUDES,
           drop_exact=BASE_EXCLUDES, add=None, git=True):
        files = BASE_FILES if files is None else files
        return _fixture(d, name, tsv_rows=[BASE_TSV if tsv is None else tsv],
                        globs=BASE_GLOBS if globs is None else globs, files=files,
                        excludes=excludes, drop_exact=drop_exact, git=git,
                        add=files if add is None else add)

    print("[正控] 域两侧对齐 + 面外域走登记豁免 + 已跟踪 no 件两侧都收 ⇒ rc=0")
    good = fx("good")
    case("正控①", "两侧域对齐（豁免域 exportnames 单侧出现）⇒ 绿", 0,
         ["R1-a 门扫的域、表里一件都没登记：0 个", "R1-b 表登记的多余域（未登记豁免）：0 个",
          "R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：0 件", "全绿（rc=0）"], good)

    print("[逐档负控] 每型一个用例 ⇒ 必须 rc=1 且**逐字点名**")
    # ── R1-a：门扫的域、表里一件都没登记（新域只加 globs 的真形态）───────────────
    case("R1-a", "门侧多一个域（dev）而表里 0 行 ⇒ 红且点名 `dev` + 两侧现值", 1,
         ["R1-a 门扫的域、表里一件都没登记：1 个",
          "dev  ← `script_globs` 有 `scripts/dev/*.sh|*.py`（实测 1 件），`公开标记.tsv` 里 0 行",
          "违规 1 处（rc=1）"],
         fx("r1a", globs=BASE_GLOBS + DEV_GLOBS, files=BASE_FILES + ("scripts/dev/lone.py",),
            excludes=BASE_EXCLUDES + ("scripts/dev/lone.py",)))
    case("成对①", "把 dev 补进表 + 收进两器 ⇒ 回绿（与 R1-a 成对 · 证明不是恒红）", 0,
         ["R1-a 门扫的域、表里一件都没登记：0 个", "全绿（rc=0）"],
         fx("r1a-fixed", tsv=BASE_TSV + DEV_ROW, globs=BASE_GLOBS + DEV_GLOBS,
            files=BASE_FILES + ("scripts/dev/lone.py",),
            excludes=BASE_EXCLUDES + ("scripts/dev/lone.py",)))
    # ── R1-b：表登记的多余域、未登记豁免 ⇒ 红 ────────────────────────────────
    case("R1-b", "表侧多一个域（zzz）且不在登记豁免表 ⇒ 红且点名 `zzz` + 表行数", 1,
         ["R1-b 表登记的多余域（未登记豁免）：1 个",
          "zzz  ← 表里 1 行 `scripts/zzz/…`，`script_globs` 里 0 条"],
         fx("r1b", tsv=BASE_TSV + "scripts/zzz/other.py\tno\tB 类：合成夹具（zzz 域）\n",
            files=BASE_FILES + ("scripts/zzz/other.py",),
            excludes=BASE_EXCLUDES + ("scripts/zzz/other.py",)))
    # ── R2-a：已跟踪 + 缺侧甲 ⇒ 红（判据⑥ 的预演）─────────────────────────────
    case("R2-a", "已跟踪的 no 件两器都缺（缺侧甲 + 侧乙）⇒ 红且点名件 + 缺哪一侧", 1,
         ["R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：1 件",
          "scripts/gates/excluded.py（表第 3 行）· 缺侧甲 + 侧乙（两器都会发它）"],
         fx("r2a", excludes=("scripts/exportnames/plug.py",),
            drop_exact=("scripts/exportnames/plug.py",)))
    case("成对②", "同一棵树把该件收进侧甲 ⇒ 回绿（与 R2-a 成对）", 0,
         ["R2-a 已跟踪且缺侧甲（= 判据⑥ 会红的那批的预演）：0 件", "全绿（rc=0）"],
         fx("r2a-fixed"))

    print("[防空绿档] 必须「构造就能复现」，且改正后必须退回绿（证明判据有牙齿、不是恒红/恒绿）")
    # ★ ⑵ 今天真形态：tsv 判 no + **未在 git 索引** + 两器都缺 ⇒ 必须出现预警行（rc=0 也不许静默）
    case("防空绿①", "未跟踪件 + 两器都缺 ⇒ 点名预警（入库那一刻判据⑥ 才红的那一格）", 0,
         ["R2-b 入库前预警点名", "scripts/dev/lone.py（表第 5 行）· 缺侧甲 + 侧乙（两器都会发它）",
          "⇒ 入库那一刻判据⑥ 报「表说 no、链上却仍会发」",
          "R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：1 件"],
         fx("r2b-untracked", tsv=BASE_TSV + DEV_ROW, globs=BASE_GLOBS + DEV_GLOBS,
            files=BASE_FILES + ("scripts/dev/lone.py",), add=BASE_FILES))   # lone.py **故意不 add**
    case("防空绿②", "同一棵树把该件收进两器 ⇒ 预警消失（证明预警不是恒亮）", 0,
         ["R2-b 未在索引且缺排除（**入库前预警** · 不入退码）：0 件"],
         fx("r2b-fixed", tsv=BASE_TSV + DEV_ROW, globs=BASE_GLOBS + DEV_GLOBS,
            files=BASE_FILES + ("scripts/dev/lone.py",), add=BASE_FILES,
            excludes=BASE_EXCLUDES + ("scripts/dev/lone.py",),
            drop_exact=BASE_EXCLUDES + ("scripts/dev/lone.py",)))
    case("防空绿③", "两侧清单漂移（侧甲有、侧乙无）⇒ 点名漂移（只报 · 不入退码）", 0,
         ["R2-c 两器清单漂移（只在一侧 · 不入退码）：1 件", "只在一侧：侧甲有、侧乙无",
          "全绿（rc=0）"], fx("r2c-drift", drop_exact=("scripts/exportnames/plug.py",)))

    print("[退码 0-1-2 钳] 缺件 / 空转 / 豁免条目不齐 / 非 git 树 ⇒ 一律 rc=2（判不了 ≠ 绿）")
    no_tbl = fx("no-table")
    os.remove(os.path.join(no_tbl, TABLE_REL))
    case("钳①", "标记表缺件 ⇒ rc=2", 2, ["不给结论"], no_tbl)
    no_cfg = fx("no-config")
    os.remove(os.path.join(no_cfg, CONFIG_REL))
    case("钳②", "config 缺件 ⇒ rc=2", 2, ["config 不在"], no_cfg)
    case("钳③", "侧甲解析为空 ⇒ rc=2（清单没了所以全绿 = 假绿）", 2, ["解析不出任何 EXCLUDES"],
         fx("empty-side", excludes=()))
    case("钳④", "script_globs 为空 ⇒ rc=2（空转）", 2, ["scan.script_globs 不是非空列表"],
         fx("bad-globs", globs=()))
    case("钳⑤", "非 git 树（跟踪面读不到）⇒ rc=2（缺件 ≠ 绿）", 2, ["git ls-files 失败"],
         fx("no-git", git=False))
    case("钳⑥", "表里一条 `public=no` 都没有 ⇒ rc=2（R2 无区分度）", 2,
         ["一条 `public=no` 都没有"],
         fx("no-no-row", tsv="scripts/build/kept.py\tyes\tA 类：合成夹具\n"))
    # 豁免条目缺 date / 缺 reason ⇒ rc=2（**进程内**调用：豁免表是模块级真源，换掉它判的是同一条判据）
    global EXEMPT_DOMAINS
    saved_ex = EXEMPT_DOMAINS
    import contextlib
    import io
    for label, ent in (("缺 date", {"domain": "exportnames", "reason": "夹具"}),
                       ("缺 reason", {"domain": "exportnames", "date": "2026-09-26"})):
        EXEMPT_DOMAINS = (ent,)
        buf, errbuf = io.StringIO(), io.StringIO()
        try:
            with contextlib.redirect_stderr(errbuf):
                rc_in = run(root=good, out=buf)
        except SystemExit as e:
            rc_in = e.code
        EXEMPT_DOMAINS = saved_ex
        got = errbuf.getvalue()
        total += 1
        ok = rc_in == EXIT_BLOCKED and "域豁免登记表不合规" in got and label in got
        bad += 0 if ok else 1
        print("  %s 钳⑦ · 豁免条目%s ⇒ rc=2 且点名（判不了 ≠ 绿）· rc=%d" % ("✓" if ok else "✗", label, rc_in))

    print("[真目标] 现读仓内（本件推得的仓根）⇒ 结构断言（rc ∈ {0,1} · 表侧域 > 0 · 门侧域 > 0）＋ 计数照实报")
    total += 1
    rc_real, text_real = _run(REPO)
    ok_real = rc_real in (EXIT_OK, EXIT_FAIL) and "R1-a 门扫的域、表里一件都没登记" in text_real
    bad += 0 if ok_real else 1
    line = [ln for ln in text_real.splitlines() if ln.startswith("合计：")]
    print("  %s 真目标 %s ⇒ rc=%d · %s" % ("✓" if ok_real else "✗", REPO, rc_real,
                                          line[0] if line else "（没有合计行）"))
    for ln in text_real.splitlines():
        if ln.strip().startswith(("✗ R1-", "✗ R2-", "· R2-b", "· R2-c", "· 覆盖缺口", "   ← ")):
            print("     %s" % ln.strip())
    if rc_real == EXIT_FAIL:
        print("     ★ 真目标现红：逐条见上 —— 本门**不改别人件来凑绿**（改判据/改表都要走拍板）")

    print("  自证用例：%d 条 · 通过 %d 条（断言用例条数 %d，少跑即失效）"
          % (total, total - bad, WANT_CASES))
    assert total == WANT_CASES, "自证用例数必须为 %d，实际 %d" % (WANT_CASES, total)
    print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
    return EXIT_OK if bad == 0 else EXIT_BLOCKED   # GAP-20261001-44：自检未过 ⇒ rc=2（与件头退出码表 L77-78 一致；原退回 EXIT_FAIL=1 与判词相矛盾）


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗ ──────────
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
          % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                            else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return EXIT_BLOCKED


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


def main():
    import atexit
    atexit.register(cleanup)
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True,
                                 description="标记表域 ⟷ 门覆盖面域 对拍 + 入库前两器排除预警（只读一致性门）")
    ap.add_argument("--root", default=REPO, help="树根（默认 = 仓根）")
    ap.add_argument("--table", default=None, help="标记表（默认 = <root>/%s）" % TABLE_REL)
    ap.add_argument("--config", default=None, help="门覆盖 config（默认 = <root>/%s）" % CONFIG_REL)
    ap.add_argument("--max-examples", type=int, default=40)
    # ★ `Q-176` 第六批：拼法统一 —— **准拼法 `--self-test`**，旧拼法 `--selftest` 作别名
    ap.add_argument("--self-test", "--selftest", dest="selftest", action="store_true",
                    help="只跑成对负控自检（合成夹具，不碰真目标）")
    args = ap.parse_args()
    if args.selftest:
        print("── check-marks-coverage-parity.py 自证（四档 · 含 must-fail 负控 · 仓外夹具）")
        return self_test()
    try:
        return run(root=os.path.abspath(args.root), table=args.table, config=args.config,
                   max_examples=args.max_examples)
    except Unsupported as e:
        print("✗ 不给结论（rc=2）：%s" % e, file=sys.stderr)
        return EXIT_BLOCKED


if __name__ == "__main__":
    sys.exit(main())

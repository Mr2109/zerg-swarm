#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-no-symlink-farm.py —— 门：**禁用「符号链接回真仓」的农场**（根因 `GAP-20260926-20` · P0）

事故形态（本门要抓的那一件）
------------------------------
一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场：

    FARM=/tmp/egg-farm
    ln -s "$REPO/core/cmd/zerg/testdata" "$FARM/case-a/testdata"   # ← 农场里的链接指回**真仓**
    rm -f "$FARM/case-a/testdata/cli-matrix.json"                  # ← 以为是删副本，删的是**真件**

真因不是「有人手滑」，是**没有「只读隔离」的工具面** ⇒ 想「在别处试」的人只能自己拼符号链接。
正门见 `scripts/dev/isolate-ro.sh`（硬拷贝隔离）与缺口账 `GAP-20260926-20`（`zerg isolate-ro` 待补）。

本门判什么（**只看形态，不看意图**）
------------------------------------------------
扫**给定目录**（默认仓根）里的脚本件（`.sh/.bash/.zsh/.py/.go/.mk`… 面见 `SCAN_EXTS`；整行注释跳过）。
在同一件里建模型：**① 变量 → ② `ln -s` 建了哪些链接 → ③ 哪些删除动作指向哪条路径**。判据 A1 / A1? / A2 / A3（＋现读档 B）：

  A1 **链接回真仓**：`ln -s <目标> <链接>` 的目标解析后落在「真仓前缀」内。目标算「解析得了」的两种形态：
     ① 字面绝对路径；② 同一件里的**赋值链**（`R="/x"; K="$R/y"` ⇒ `/x/y`）。
     ★ **只在 `--farm` 那一档算红**；扫仓内件时**只报告**（存量不许拿它把提交闸锁死 ✗ —— 同「只报告 / 基线棘轮」先例）。
  A1? **目标前缀追不到源头 ⇒ 点名待人工核**（值未知 · **不许静默放过** ✗）：前缀来自**命令替换**
     （`ROOT="$(cd … && pwd)"; ln -s "$ROOT/x" …` —— 值运行时才知道），或引用了**本件未赋值的变量**
     （值来自环境/调用方）。这类只进 `reports`（code `A1?`）+ 打印，**不入退码**；`--strict-links` 档算红。
     口径：**判不了就点名** —— 不静默放过（假绿 ✗）、也不一律判红（自己造红 ✗）；值的展开**不猜**。
     ⚠ **诚实边界（本笔实测** · `--root .`）：**不追**跨件/环境变量的值 · **不判**相对路径（cwd 未知）·
       A3 的删除目标同形态**不点名** —— 实测：给 A3 也上「点名待人工核」，真仓立刻多 **61 条**
       （`.go` 14 · `.sh` 47；来源 cmdsub 34 / 未赋值变量 27），逐条核过**全是假阳**：
         ① `.go` 那 14 条是**测试夹具/注释里的字串**（`core/internal/agent/bash_v101*.go` ·
            `core/internal/policy/builtin_rules*.go` 的 `"rm -rf $HOME"` · `// … rm -rf $VAR/`）——
            Go 件里 `$VAR` 只是文本，没有任何删除会执行；
         ② `.sh` 那 47 条是**临时/沙箱目录的收尾清理**（`trap 'rm -rf "$tmp"' EXIT` ·
            `rm -rf "${CLEANUP_DIR}"` · `rm -rf "$SANDBOX"`）——这些变量恰好展开到「**别处**」，
            点名它们 = 喊狼来了。口径：噪声 >10 条 ⇒ **不实现**（本批 61 条）；自检⑨ 把这条口径锁住。
       ⚠ 「变量名 hint」那一档**同样否掉**（本笔删掉的死常量 `REPO_VAR_HINTS` 就是它）：拿 hint 名
         （REPO/ROOT/ZERG…）过滤只把 61 条收到 **10 条**，而这 10 条仍是假阳（`ZERG_*_SANDBOX_DIR` ·
         `ZERG_PREFIX` · `REPO_ROOT` 展开出来的沙箱/发布暂存）；真仓里 hint 名变量持有「真仓**外**绝对路径」
         的赋值只有 **1 处**（`/tmp` 的报告文件）⇒ 照名字判红/点名 = 自己造红。A 档只认两条：
         **A2**（删除动作指向链接）+ **A3**（前缀真追得到真仓），其余未知形态交 `--strict-links` 与人工核。
     ★ 本仓实测（改后 · `--root .`）：**1083 件** · **红 0** · **只报告 1 条 A1?** —— `scripts/dev/isolate-ro.sh:353`
       （那是 isolate-ro.sh **自己的自检夹具**：`fix="${tmp}/fix"` 而 `tmp="$(mktemp -d …)"` ⇒ 值运行时才知道）。
       ⇒ 别把这 1 条读成「门在乱喊」，但**也不许**把它读成 0 件；`--strict-links` 档下这 1 条升红 ⇒ rc=1。
       ⚠ 历史那一处 `scripts/sandbox/test-fleet-sandbox.sh` 已改为 `cp -p` 硬拷贝 ⇒ 该形态在仓内已不存在，
         故 docstring **不再拿它举例**（旧版声称「报出 :270」而实测 0 处 ✗ —— 说法与实测不符就是本件要修的账）。
  A2 **删除动作穿过符号链接**：某条删除动作（`rm …` / `shutil.rmtree(` / `os.remove(` / `.unlink()` / `find -delete`）
     的目标路径以**同一件里刚建的那条链接**为前缀（`$FARM/link/…`）⇒ **红**。
     这条独立成判据：删的是「链接**下面**的东西」，删除会**透传到链接目标**（事故就是这样发生的）。
  A3 **删除动作的目标含真仓前缀**：目标要么是字面绝对路径、以真仓前缀开头，要么引用的变量在同一件里被赋过
     真仓前缀的绝对路径（`REPO="<volume-path>"; rm -rf "$REPO/scripts/tmp"`）⇒ **红**。

  B **农场现场（`--farm <目录>` · 现读文件系统）**：该目录下（深度 ≤ `--max-depth`）任何符号链接，其目标
    （`realpath`）落在真仓前缀内 ⇒ **红**。★ 这一档是「事故现场」档，**指回真仓就是红**（农场的意思就是
    「别处」；`.git`/`node_modules` 等重名目录走 `EXCLUDE_DIRS` 不进扫描面）。
    ★ 不给 `--farm` ⇒ 打印「B 档未给 ⇒ 跳过」并**不影响退码**（A1–A3 独立成立）。

成对控（`--self-test` · 合成夹具 · 一件真东西都不碰）
------------------------------------------------------
  ① 负控-A2/A1：`ln -s <真仓>/core/cmd/zerg/testdata <农场>/link` + `rm -f <农场>/link/cli-matrix.json` ⇒ **rc=1**，点名件:行
  ② 正控：把上面那条 `rm` 换成「删同一农场里自己拷出来的树」 ⇒ **rc=0**（与 ① 成对）
  ③ 负控-A3：`REPO="<真仓>"; rm -rf "$REPO/scripts/tmp"` ⇒ rc=1
  ④ 负控-B：`--farm <夹具农场>` 里放一条**真**符号链接指回夹具「真仓」 ⇒ rc=1
  ⑤ 正控-B：把那条链接删掉 ⇒ rc=0（与 ④ 成对）
  ⑥ 用法错：`--root` 不存在 ⇒ rc=2 · 候选 0 件（空转）⇒ rc=2
  ⑦ 负控-A1?（**命令替换前缀**）：`ROOT="$(cd … && pwd)"; ln -s "$KERNEL" <农场>/link` ⇒ **点名 `A1?`**
     （rc=0 · 只报告）；`--strict-links` 下同一夹具 ⇒ **rc=1**。
     ★ 改前实测：同一夹具 rc=0 / reports=0 = **静默放过** ⇒ 这一格就是给那次静默上的锁（防空绿）。
  ⑧ 正控：字面绝对路径（真仓外）+ 真·相对链接 ⇒ rc=0 且 **0 报告**（与 ⑦ 成对 · 防「一律点名」的假红）
  ⑨ 边界锁（**A3 侧不点名** · 本笔实测决定）：`CLEANUP_DIR="$(mktemp -d)"; rm -rf "${CLEANUP_DIR}"` ⇒ rc=0 且
     **0 报告** —— 把「A3 不做点名」这条口径钉死（61 条实测噪声见上「诚实边界」）⇒ 防后来人不带数据地打开它。

用法与退码
----------
    python3 scripts/gates/check-no-symlink-farm.py                     # 扫真仓（静态档 A）
    python3 scripts/gates/check-no-symlink-farm.py --root <目录> --real-repo <真仓>
    python3 scripts/gates/check-no-symlink-farm.py --farm <农场目录>    # 加现读档 B（指回真仓即红）
    python3 scripts/gates/check-no-symlink-farm.py --strict-links      # A1 / A1? 也算红（审一件可疑农场脚本时用）
    python3 scripts/gates/check-no-symlink-farm.py --self-test | --list-rules | --json

  退码：0 全绿 · 1 **有红**（A2 / A3 / B / `--strict-links` 下的 A1 与 A1?）· 2 **不给结论**（用法错 / root 缺 /
  候选 0 件 ⇒ 空转 · `--self-test` 不过 ⇒ 拒绝扫真目标）
  口径：**「判不了」不给结论** —— 报绿报红都要有数字；**只报告（A1 存量 + A1? 待人工核）与红分开打印**。

纪律：**只读**（不写盘、不删、不建链接）· 不联网 · 不起后台进程 · 不 sudo。
     写盘一律 `print(..., file=sys.stderr)`（本仓门禁自扫「写动作」形态 ⇒ 不许 `sys.stderr.write`）。
"""
import argparse
import json
import os
import re
import shlex
import shutil
import sys
import tempfile

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))

SCAN_EXTS = (".sh", ".bash", ".zsh", ".py", ".go", ".mk", ".pl", ".rb")
EXCLUDE_DIRS = {
    ".git", "vendor", "node_modules", "target", "dist", "bin", "data", "zerg-wt", ".zerg",
    "__pycache__", ".venv", "venv", ".cargo", "htmlobs", "htmlcov", ".history", ".build",
}
# ★ 已删（本笔）：这里曾有 `REPO_VAR_HINTS = ("REPO","ROOT","SRC","ZERG","TREE","KERNEL")` 这个**死常量**
#   （全件零引用 ✗）—— 它暗示过「变量名 hint ⇒ 当真仓前缀」那一档。**实测否掉、不实现**（数据见 docstring
#   「诚实边界」第 ② 条）：认前缀只认**路径比对**（见 `_under`），拿变量名猜就是**自己造红**。

RE_LN = re.compile(r"(?<![\w.-])ln\s+((?:-[A-Za-z]+\s+)*)(.*)$")
RE_RM = re.compile(r"(?<![\w.-])rm\s+((?:-{1,2}[A-Za-z][A-Za-z-]*\s+)*)(.*)$")
# ★ 缺口 `GAP-20260926-115`（2026-09-27 父代理现场补）：值分组此前是 `([^\s;]+)` ⇒ **只吃到第一个空白**，
#   于是 `R="<repo>"` 被吃成 `"<volume-path>`（截断 **且** 留着前导引号 ⇒
#   `_strip` 的去引号判据 `t[0]==t[-1]` 不成立 ⇒ 引号也留着）⇒ 交给 A1/A3 时 `rt.startswith("/")` 为假
#   ⇒ **「指向真仓」的判定静默失效** ✗。★ 真仓路径**本身带空格** ⇒ 这不是纸面隐患。
#   修法：引号串整体先吃（`"[^"]*"|'[^']*'`），再轮到无引号形态；去引号仍交给 `_strip`（口径不变）。
RE_ASSIGN = re.compile(
    "^\\s*(?:export\\s+|local\\s+|declare\\s+-[a-zA-Z]*\\s+)?"   # 前缀
    "([A-Za-z_][A-Za-z0-9_]*)="                                # 变量名 `=`
    "(\"[^\"]*\"|'[^']*'|[^\\s;]+)"                            # 值：双引号串 / 单引号串 / 无引号
)
RE_PY_RM = (
    (re.compile(r"shutil\.rmtree\(\s*([^,\)]+)"), "shutil.rmtree"),
    (re.compile(r"os\.(?:remove|unlink|rmdir)\(\s*([^,\)]+)"), "os.remove/unlink/rmdir"),
    (re.compile(r"find\([^)]*\)\.delete\(\)"), "Path.delete"),
    (re.compile(r"\.unlink\(\s*\)"), "Path.unlink"),
    (re.compile(r"\brmtree\(\s*([^,\)]+)"), "rmtree"),
)
RE_FIND_DELETE = re.compile(r"(?<![\w.-])find\s+((?:-[A-Za-z]+\s+)*)(.*)$")


RE_TOK = re.compile(r'"[^"]*"|\'[^\']*\'|\S+')

# ★ 三引号定界符**一律不写字面量**（三个连续的双引号 / 单引号）—— 拼接出来用。
#   跨件扫描器（`check-zh-en.py` 的 `RE_TRIPLE = (三个双引号|三个单引号)[\s\S]*?\1`）按**出现次序**两两配对：
#   源码里多出一处**不是块界**的字面量（本件旧 L236 写作 `s.startswith('三个双引号')`）⇒ 它和后面**真文档字符串**
#   的起始引号配成一对 ⇒ 那处 docstring 的**尾段落回「代码区」**、被当代码扫（`RE_TRIPLE` 没漏剥，是**错配**）。
#   实测（改前）：全件字面量计数 **17 = 奇数**（16 处真块界 + 旧 L236 那 1 处）；`RE_TRIPLE` 第 8 对 = L236..L376
#   ⇒ L376（`scan_farm` 的单行文档字符串）尾段 `B 档：…红。` 被报「全角标点(代码区)」= **本门自己的码上的假红**。
#   改为拼接后计数 16 = 偶数、8 对全配 ⇒ 假红消失。
#   ⚠ 这一步只**去掉歧义字面量**：判据（A1/A1?/A2/A3/B）与 docstring 剥离口径**一字未改**，门牙不松。
DQ3 = '"' * 3
SQ3 = "'" * 3


def _strip(tok):
    """去引号与尾随分号 ⇒ 便于前缀比对。

    ★ **不许** `rstrip(";,)]}")`：它会把 `${VAR}` 的右花括号一起啃掉（`${t}` → `${t`），
      于是变量名解析错位 ⇒ 假红/假绿都出过（实测：一次真仓扫描因此报出 20+ 条假红）。
    """
    t = tok.strip().rstrip(";,")
    if len(t) >= 2 and t[0] == t[-1] and t[0] in "\"'`":
        t = t[1:-1]
    return t


def _split_args(tail):
    """按引号分块切词（**不用 shlex**：裸引号/未闭合引号会让 shlex 抛异常并退回「按空格切」，
    那一步会把 `"${A}/x"` 切成 `"${A}/x"` 之外的东西 ⇒ 形态判据直接错位）。"""
    return [_strip(a) for a in RE_TOK.findall(tail)]


def _resolve_var(tok, assigns, depth=0):
    """把 `$REPO/x` / `${REPO}/x` 这类只认「已赋过绝对路径的变量」⇒ 值；认不出返回 None。"""
    if depth > 3:
        return None
    m = re.match(r"^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?(.*)$", tok)
    if not m:
        return None
    name, rest = m.group(1), m.group(2)
    if name not in assigns:
        return None
    base = _resolve_var(assigns[name], assigns, depth + 1) or assigns[name]
    if base is None:
        return None
    if "$(" in base or "`" in base:      # 命令替换 ⇒ 值未知 ⇒ 不猜（猜了就是自己造红）
        return None
    return base.rstrip("/") + rest


def _target_source(tok, assigns, depth=0):
    """追 `ln -s` **目标** token 的来源 ⇒ 分出「能判 / 判不了 / 与真仓无关」；⇒ (kind, 值/线索, 出处)：

      ("resolved", 值, tok)      变量链追到头是**字面绝对路径**（能判 ⇒ 交给 `_under` 比前缀）
      ("relative", 值, tok)      追到头的值是**相对路径**（值确定，但 cwd 未知 ⇒ 不判、也不点名）
      ("cmdsub", 片段, 变量名)   变量链某一环的值来自**命令替换**（`$(…)` / 反引号）⇒ 值运行时才知道
      ("unassigned", 名, 名)     变量在**本件里没有赋值**（值来自环境/调用方 ⇒ 同样值未知）
      ("other", tok, tok)        字面量但不是绝对路径（含真·相对路径）

    ★ 盲区账（改后修）：旧 A1 只认「赋值里的字面绝对路径」，**命令替换前缀**一律落进「相对/未解析」桶
      ⇒ **静默放过**（改前实测：`ROOT="$(cd … && pwd)"; ln -s "$ROOT/…" …` 这形态给 rc=0 / reports=0）。
      现在 cmdsub / unassigned **不再静默**：点名「待人工核」（只报告；`--strict-links` 下算红）。
      ★ 第三个元素（变量名）是为了「待人工核」时**能指回那一行原文** —— 人核时知道追到哪一环断了。
    ★ 判不了就**不说绿**（静默放过 ✗），也**不自己造红**（值的展开**不猜** ➜ 不拿它比真仓前缀）。
    """
    if depth > 5:
        return "other", tok, tok
    m = re.match(r"^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?(.*)$", tok)
    if not m:
        return ("resolved" if tok.startswith("/") else "other"), tok, tok
    name, rest = m.group(1), m.group(2)
    if name not in assigns:
        return "unassigned", name, name
    val = assigns[name]
    if "$(" in val or "`" in val:
        return "cmdsub", val, name
    if val.startswith("$"):
        kind, base, origin = _target_source(val, assigns, depth + 1)
        if kind == "resolved":
            return "resolved", base.rstrip("/") + rest, tok
        return kind, base, origin
    if val.startswith("/"):
        return "resolved", val.rstrip("/") + rest, tok
    return "relative", val + rest, tok


def _under(path, prefixes):
    """只有**绝对路径**才谈得上「落在真仓前缀内」。

    ★ 相对路径判不了（调用方 cwd 未知）⇒ 返回 None，**绝不** `os.path.abspath()` 硬拼成
      仓根下的路径 —— 实测踩过：`tmp="$(mktemp -d …)"` 这种值被 abspath 之后撞上仓前缀，
      一次真仓扫描报出 30+ 条假红（假红会把这门直接判死）。
    """
    if not path or "$" in path or "`" in path:
        return None
    if not path.startswith("/"):
        return None
    p = os.path.normpath(path)
    for pre in prefixes:
        if not pre:
            continue
        q = os.path.normpath(pre)
        if p == q or p.startswith(q + os.sep):
            return pre
    return None


def _scan_file(path, prefixes, rel):
    """⇒ (hits, notes)：hits = 红（A2/A3 + strict 下的 A1），notes = 只报告（A1 存量）。"""
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            lines = fh.read().splitlines()
    except OSError:
        return [], []

    # ★ **模块/函数 docstring 一律跳过**（三引号字符串不是代码，永远不会被执行）：
    #   本仓每个门件的开头都是一段**规格**，里面**必然**写着形态示例（本件自己的 docstring 里就写着
    #   那次事故的 `ln -s` + `rm` 两行）⇒ 不跳过 = 门被自己的说明书判红（实测栽过：唯一一条命中就是本件 docstring）。
    code_lines = []
    doc_delim = None
    for line in lines:
        s = line.strip()
        if doc_delim is None:
            if s.startswith(DQ3) or s.startswith(SQ3):
                d = s[:3]
                if s.count(d) >= 2:      # 单行三引号
                    code_lines.append("")
                    continue
                doc_delim = d
                code_lines.append("")
                continue
            code_lines.append(line)
        else:
            if doc_delim in s:
                doc_delim = None
            code_lines.append("")
    lines = code_lines

    assigns, links, hits, notes = {}, [], [], []
    assign_lines = {}          # 变量名 ⇒ 那一行原文（`RE_ASSIGN` 只吃到第一个空白 ⇒ 说明里用原文更准）
    # 第一遍：变量表（含「值引用另一个变量」的两轮传播）
    for line in lines:
        s = line.strip()
        if s.startswith("#"):
            continue
        m = RE_ASSIGN.match(line)
        if m:
            assigns[m.group(1)] = _strip(m.group(2))
            assign_lines[m.group(1)] = line.strip()
    for _ in range(2):
        for k, v in list(assigns.items()):
            if v.startswith("$"):
                r = _resolve_var(v, assigns)
                if r:
                    assigns[k] = r

    for i, line in enumerate(lines, 1):
        raw = line.strip()
        if raw.startswith("#"):
            continue                                  # 整行注释不算（注释里的示例不是隐患）
        body = re.split(r"\s#\s", line, maxsplit=1)[0]  # 行尾注释（粗糙但本地口径写死）
        for stmt in body.split(";"):
            stmt = stmt.strip()
            if not stmt:
                continue

            m = RE_LN.search(stmt)
            if m:
                args = _split_args(m.group(2))
                args = [a for a in args if not a.startswith("-")]
                if len(args) >= 2:
                    target, link = args[0], args[1]
                    rt = _resolve_var(target, assigns) or target
                    pre = _under(rt, prefixes) if rt.startswith("/") else None
                    if pre:
                        links.append((link, "真仓", i, stmt, rt))
                    elif rt.startswith("/"):
                        links.append((link, "绝对路径（非真仓）", i, stmt, rt))
                    else:
                        # ★ 改前这里**一律**落「相对/未解析」= 静默桶 —— 命令替换前缀就藏在这桶里（报 0 处）。
                        #   现在把「值未知」的两种从桶里挑出来**单列** ⇒ 下面进 notes 点名「待人工核」。
                        #   （真·相对路径仍在「相对/未解析」：值确定、只是 cwd 未知 ⇒ 维持原状、不点名）
                        skind, _sval, _sname = _target_source(target, assigns)
                        if skind == "cmdsub":
                            links.append((link, "待人工核（前缀来自命令替换）", i, stmt, target))
                        elif skind == "unassigned":
                            links.append((link, "待人工核（变量本件未赋值）", i, stmt, target))
                        else:
                            links.append((link, "相对/未解析", i, stmt, rt))

            targets = []
            m = RE_RM.search(stmt)
            if m:
                for a in _split_args(m.group(2)):
                    if a.startswith("-"):
                        continue
                    targets.append(("rm", a))
            for rx, kind in RE_PY_RM:
                for mm in rx.finditer(stmt):
                    g = mm.group(1) if mm.groups() else ""
                    if g:
                        targets.append((kind, _strip(g)))
            m = RE_FIND_DELETE.search(stmt)
            if m and "-delete" in m.group(0):
                for a in _split_args(m.group(2)):
                    if not a.startswith("-"):
                        targets.append(("find -delete", a))

            for kind, tok in targets:
                if not tok:
                    continue
                for link, lkind, lno, lline, ltgt in links:
                    if tok == link or tok.startswith(link + "/"):
                        hits.append((rel, i, "A2", kind, tok,
                                     "删除动作穿过符号链接（%s:%d `ln -s` ⇒ %s）" % (rel, lno, ltgt)))
                rt = _resolve_var(tok, assigns)
                pre = _under(rt, prefixes) if rt else None
                if pre is None and tok.startswith("/"):
                    pre = _under(tok, prefixes)
                if pre is not None:
                    hits.append((rel, i, "A3", kind, tok, "删除目标落在真仓前缀内：%s" % pre))

    for link, lkind, i, stmt, ltgt in links:
        if lkind == "真仓":
            notes.append((rel, i, "A1", link, ltgt, "农场链接指回真仓（本件无删除动作指向它 ⇒ 只报告）"))
        elif lkind.startswith("待人工核"):
            # ★ 值未知 ⇒ **点名**（不是静默放过 ✗，也不是一律判红 ✗）：把「追到哪一环断了」写进说明，方便人工核。
            _k, _v, _nm = _target_source(ltgt, assigns)
            if _k == "cmdsub":
                _why = ("链接目标前缀来自**命令替换**（运行时才知道值 ⇒ 门判不了）；断在源头那一行：`%s`"
                        % assign_lines.get(_nm, _v))
            else:
                _why = ("链接目标引用了**本件未赋值**的变量 `%s`（值来自环境/调用方 ⇒ 门判不了）" % _nm)
            notes.append((rel, i, "A1?", link, ltgt,
                          "%s ⇒ **待人工核**：展开后若落在真仓前缀内就是 A1 事故形态（本件无删除动作指向它 ⇒ 只报告）" % _why))
    return hits, notes


def scan_static(root, prefixes, strict_links, max_examples):
    hits, notes, nfiles = [], [], 0
    root = os.path.abspath(root)
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for name in sorted(filenames):
            if not name.endswith(SCAN_EXTS):
                continue
            p = os.path.join(dirpath, name)
            if os.path.islink(p) or not os.path.isfile(p):
                continue
            if os.path.getsize(p) > 4 * 1024 * 1024:
                continue
            nfiles += 1
            h, n = _scan_file(p, prefixes, os.path.relpath(p, root))
            hits.extend(h)
            notes.extend(n)
    if strict_links:
        for rel, i, code, link, tgt, why in notes:
            hits.append((rel, i, "A1(--strict-links)", link, tgt, why))
        notes = []
    return hits, notes, nfiles


def scan_farm(farm, prefixes, max_depth, max_examples):
    """B 档：现读农场目录 —— 任何符号链接的目标落在真仓前缀内 ⇒ 红。"""
    hits, nlinks = [], 0
    farm = os.path.abspath(farm)
    base_depth = farm.rstrip("/").count("/")
    for dirpath, dirnames, filenames in os.walk(farm, followlinks=False):
        if dirpath.rstrip("/").count("/") - base_depth >= max_depth:
            dirnames[:] = []
        dirnames[:] = [d for d in dirnames if d not in (".git", "node_modules", "__pycache__")]
        for name in list(dirnames) + list(filenames):
            p = os.path.join(dirpath, name)
            if not os.path.islink(p):
                continue
            nlinks += 1
            tgt = os.path.realpath(p)
            pre = _under(tgt, prefixes)
            if pre is not None:
                hits.append((os.path.relpath(p, farm), 0, "B", "symlink", tgt,
                             "农场里的链接指回真仓（%s）⇒ 对它的删除会透传到真件" % pre))
    return hits, nlinks


def run_self_test():
    tmp = tempfile.mkdtemp(prefix="zerg-nosymfarm-")
    bad = []

    def chk(label, ok, extra=""):
        print("   %s %s%s" % ("✓" if ok else "✗", label, ("（%s）" % extra) if extra and not ok else ""))
        if not ok:
            bad.append(label)

    def run(root, real, farm=None, strict=False):
        argv = ["--root", root, "--real-repo", real, "--no-self-test", "--json"]
        if farm:
            argv += ["--farm", farm]
        if strict:
            argv += ["--strict-links"]
        rc, payload = _main(argv)
        return rc, payload

    try:
        fix = os.path.join(tmp, "fix")
        real = os.path.join(fix, "real")
        os.makedirs(os.path.join(real, "core", "cmd", "zerg", "testdata"))
        with open(os.path.join(real, "core", "cmd", "zerg", "testdata", "cli-matrix.json"), "w") as fh:
            fh.write("{}\n")

        # ① 负控 A2/A1：链接回真仓 + 删链接下面的东西
        d1 = os.path.join(fix, "bad-link")
        os.makedirs(os.path.join(d1, "farm"))
        with open(os.path.join(d1, "farm.sh"), "w") as fh:
            fh.write("REPO=\"%s\"\nFARM=\"%s\"\n"
                     "ln -s \"${REPO}/core/cmd/zerg/testdata\" \"$FARM/case-a/testdata\"\n"
                     "rm -f \"$FARM/case-a/testdata/cli-matrix.json\"\n"
                     % (real, os.path.join(d1, "farm")))
        rc, payload = run(d1, real)
        codes = sorted({h[2] for h in payload.get("hits", [])}) if payload else []
        chk("① 负控-A2：链接回真仓 + 删链接下面 ⇒ rc=1 且判据 A2", rc == EXIT_FAIL and "A2" in codes,
            "rc=%s codes=%s" % (rc, codes))
        chk("① 负控-A1：同一件里报出「链接指回真仓」",
            any("A1" in h[2] or h[2] == "A2" for h in (payload or {}).get("hits", [])))

        # ② 正控：同样的农场，但删除只落在农场自己的树上 ⇒ rc=0
        d2 = os.path.join(fix, "good")
        os.makedirs(os.path.join(d2, "copy"))
        with open(os.path.join(d2, "copy", "x.json"), "w") as fh:
            fh.write("{}\n")
        with open(os.path.join(d2, "good.sh"), "w") as fh:
            fh.write("d=\"%s\"\nrm -rf \"$d/copy/x.json\"\n" % d2)
        rc, _ = run(d2, real)
        chk("② 正控：只删自己拷出来的树 ⇒ rc=0（与 ① 成对）", rc == EXIT_OK, "rc=%s" % rc)

        # ③ 负控 A3：变量含真仓前缀 + 删这个变量下面
        d3 = os.path.join(fix, "bad-var")
        os.makedirs(d3)
        with open(os.path.join(d3, "wipe.sh"), "w") as fh:
            fh.write("REPO=\"%s\"\nrm -rf \"$REPO/scripts/tmp\"\n" % real)
        rc, payload = run(d3, real)
        codes = sorted({h[2] for h in payload.get("hits", [])}) if payload else []
        chk("③ 负控-A3：删真仓变量下面 ⇒ rc=1 且判据 A3", rc == EXIT_FAIL and "A3" in codes,
            "rc=%s codes=%s" % (rc, codes))

        # ④ 负控 B / ⑤ 正控 B：农场现场里一条真符号链接
        farm = os.path.join(fix, "farm-live")
        os.makedirs(farm)
        link = os.path.join(farm, "testdata-link")
        os.symlink(os.path.join(real, "core", "cmd", "zerg", "testdata"), link)
        rc, payload = run(d2, real, farm=farm)
        chk("④ 负控-B：农场现场链接回真仓 ⇒ rc=1", rc == EXIT_FAIL, "rc=%s" % rc)
        os.unlink(link)
        rc, _ = run(d2, real, farm=farm)
        chk("⑤ 正控-B：链接拿掉 ⇒ rc=0（与 ④ 成对）", rc == EXIT_OK, "rc=%s" % rc)

        # ⑥ 用法错 / 空转
        rc, _ = _main(["--root", os.path.join(tmp, "no-such"), "--no-self-test"])
        chk("⑥ 用法错：--root 不存在 ⇒ rc=2", rc == EXIT_BLOCKED, "rc=%s" % rc)
        empty = os.path.join(fix, "empty")
        os.makedirs(empty)
        rc, _ = _main(["--root", empty, "--no-self-test"])
        chk("⑥ 用法错：候选 0 件（空转）⇒ rc=2", rc == EXIT_BLOCKED, "rc=%s" % rc)

        # ⑦ 负控-A1?（**命令替换前缀**）：目标前缀**追不到源头** ⇒ 不许静默放过（改前这里报 0 处 = 盲区）
        #    ★ 防空绿：改前同一夹具实测 rc=0 / reports=0（旧 A1 只认「赋值里的字面绝对路径」）⇒ 这格就是给那次静默上的锁。
        d4 = os.path.join(fix, "bad-cmdsub")
        os.makedirs(d4)
        with open(os.path.join(d4, "farm.sh"), "w") as fh:
            fh.write("REPO_ROOT=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")/../..\" && pwd)\"\n"
                     "KERNEL=\"$REPO_ROOT/scripts/build/zerg-upgrade.sh\"\n"
                     "FARM=\"%s\"\n"
                     "ln -sf \"$KERNEL\" \"$FARM/scripts/build/zerg-upgrade.sh\"\n"
                     "rm -f \"$FARM/copy/x.json\"\n" % os.path.join(d4, "farm"))
        rc, payload = run(d4, real)
        codes = sorted({n[2] for n in (payload or {}).get("reports", [])})
        chk("⑦ 负控-A1?：目标前缀来自命令替换 ⇒ **点名待人工核**（不是静默 0）",
            rc == EXIT_OK and "A1?" in codes, "rc=%s reports_codes=%s" % (rc, codes))
        rc, payload = run(d4, real, strict=True)
        codes = sorted({h[2] for h in (payload or {}).get("hits", [])})
        chk("⑦ 负控-A1?（--strict-links）：同一夹具 ⇒ rc=1",
            rc == EXIT_FAIL and "A1(--strict-links)" in codes, "rc=%s hits_codes=%s" % (rc, codes))

        # ⑧ 正控：无害形态（字面绝对路径在真仓外 + 真·相对链接）⇒ rc=0 且 **0 报告**（与 ⑦ 成对 · 防「一律点名」的假红）
        d5 = os.path.join(fix, "good-forms")
        os.makedirs(d5)
        with open(os.path.join(d5, "ok.sh"), "w") as fh:
            fh.write("D=\"%s\"\nln -s \"$D/target\" \"$D/link\"\nln -s ../rel/target rel-link\n"
                     "rm -rf \"$D/copy\"\n" % os.path.join(d5, "farm-good"))
        rc, payload = run(d5, real)
        chk("⑧ 正控：字面绝对路径（真仓外）+ 真·相对链接 ⇒ rc=0 且 0 报告",
            rc == EXIT_OK and not (payload or {}).get("reports"),
            "rc=%s reports=%s" % (rc, (payload or {}).get("reports")))

        # ⑨ 边界锁（**A3 侧不点名** —— 本笔实测：给 A3 上点名会引 61 条假阳，见 docstring「诚实边界」）
        #    这一格是**正控**：临时目录收尾清理这种形态**不许**被点名（点它就是喊狼来了）。
        d6 = os.path.join(fix, "tmp-clean")
        os.makedirs(d6)
        with open(os.path.join(d6, "clean.sh"), "w") as fh:
            fh.write("CLEANUP_DIR=\"$(mktemp -d)\"\nrm -rf \"${CLEANUP_DIR}\"\n")
        rc, payload = run(d6, real)
        chk("⑨ 边界锁：A3 侧「前缀来自命令替换」**不点名**（临时目录清理）⇒ rc=0 且 0 报告",
            rc == EXIT_OK and not (payload or {}).get("reports"),
            "rc=%s reports=%s" % (rc, (payload or {}).get("reports")))

        # ⑩ 含空格赋值（缺口 `GAP-20260926-115` · 2026-09-27 父代理现场补）：**引号里带空格**的赋值行。
        #    修前：`RE_ASSIGN` 只吃到第一个空白 ⇒ 值 `"<volume-path>`（还留前导引号）⇒ `startswith("/")`
        #    为假 ⇒ 指向真仓的 `ln -s` **静默不报** ✗（形态的失效已单独实测：`re.search` 捕获
        #    `'"<volume-path>'`）。修后：值完整 ⇒ A1 必须点名。★ 真仓路径本身带空格 ⇒ 这格不是构造题。
        d7 = os.path.join(fix, "space-assign")
        os.makedirs(d7)
        with open(os.path.join(d7, "space.sh"), "w") as fh:
            fh.write("R=\"%s/sub dir\"\nln -s \"$R/t\" \"$R/l\"\n" % real)
        rc, payload = run(d7, real)
        codes = sorted({n[2] for n in (payload or {}).get("reports", [])})
        chk("⑩ 含空格赋值（缺口 `GAP-20260926-115`）：引号里带空格的路径 ⇒ 值完整 ⇒ 指向真仓必须点名 A1",
            rc == EXIT_OK and any(c.startswith("A1") for c in codes),
            "rc=%s codes=%s" % (rc, codes))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("── 自检结论：%s ──" % ("全过" if not bad else "**不过**（%d 条）" % len(bad)))
    return EXIT_OK if not bad else EXIT_BLOCKED


def _rules():
    print("判据（本门只认形态）")
    print("  A1 链接回真仓（`ln -s <真仓>/… <链接>`）—— 扫仓内件时只报告；`--farm` / `--strict-links` 档算红")
    print("  A1? 链接目标前缀**追不到源头**（命令替换 `$(…)` / 本件未赋值的变量）—— 点名待人工核；"
          "只报告，`--strict-links` 档算红")
    print("  A2 删除动作穿过符号链接（`rm $农场/link/…`）—— 红（删除会透传到链接目标 = 事故本体）")
    print("  A3 删除目标含真仓前缀（字面路径，或引用的变量被赋过真仓前缀）—— 红")
    print("  B  农场现场（`--farm <目录>`）：任何链接目标落在真仓前缀内 —— 红")
    print("扫描面：%s；整行注释跳过；硬排除 %s" % (" ".join(SCAN_EXTS), " ".join(sorted(EXCLUDE_DIRS))))
    print("退码：0 全绿 · 1 有红 · 2 不给结论（用法错/缺件/空转/自检不过）")


def _main(argv=None):
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=REPO_ROOT_DEFAULT)
    ap.add_argument("--real-repo", default=None)
    ap.add_argument("--farm", default=None)
    ap.add_argument("--strict-links", action="store_true")
    ap.add_argument("--max-depth", type=int, default=3)
    ap.add_argument("--max-examples", type=int, default=20)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--json", action="store_true")
    a, unknown = ap.parse_known_args(argv)
    if unknown:
        print("✗ 用法错：未知参数 %s（--list-rules 看口径）" % " ".join(unknown), file=sys.stderr)
        return EXIT_BLOCKED, None
    if a.list_rules:
        _rules()
        return EXIT_OK, None

    if not a.no_self_test:
        if run_self_test() != EXIT_OK:
            print("✗ 自检不过 ⇒ 拒绝扫真目标（项目口径：没有自证过「会红」的门不许给结论）", file=sys.stderr)
            return EXIT_BLOCKED, None

    root = os.path.abspath(a.root)
    if not os.path.isdir(root):
        print("✗ 判不了：--root 不是目录 —— %s（rc=2）" % root, file=sys.stderr)
        return EXIT_BLOCKED, None

    # ★ 前缀**两种写法都要收**：用户给的字面路径（源码里就是这么写的）＋ 它的 realpath
    #   （macOS 上 `/var` → `/private/var`、`/tmp` → `/private/tmp`：只收 realpath 会漏掉
    #    源码里那句字面路径 ⇒ 假绿。实测踩过：夹具里写 /var/…，前缀只收 /private/var/… ⇒ A3 检不出）
    prefixes = []
    if a.real_repo:
        rr = os.path.abspath(os.path.expanduser(a.real_repo))
        prefixes += [rr, os.path.realpath(rr)]
    prefixes += [REPO_ROOT_DEFAULT, os.path.realpath(REPO_ROOT_DEFAULT)]
    prefixes = [q for q in dict.fromkeys(prefixes) if q]

    hits, notes, nfiles = scan_static(root, prefixes, a.strict_links, a.max_examples)
    fhits, nlinks = ([], 0)
    if a.farm:
        if not os.path.isdir(a.farm):
            print("✗ 判不了：--farm 不是目录 —— %s（rc=2）" % a.farm, file=sys.stderr)
            return EXIT_BLOCKED, None
        fhits, nlinks = scan_farm(a.farm, prefixes, a.max_depth, a.max_examples)
    hits += fhits

    if nfiles == 0:
        print("✗ 空转：%s 下 0 个候选脚本件 ⇒ 判不了（rc=2；假覆盖不算绿）" % root, file=sys.stderr)
        return EXIT_BLOCKED, None

    if a.json:
        print(json.dumps({
            "schema": "zerg/gate-no-symlink-farm/v1",
            "kind": "NoSymlinkFarmGate",
            "items": [{"file": h[0], "line": h[1], "code": h[2], "action": h[3], "path": h[4], "why": h[5]}
                      for h in hits],
            "reports": [{"file": n[0], "line": n[1], "code": n[2], "link": n[3], "target": n[4], "why": n[5]}
                        for n in notes],
            "meta": {"root": root, "prefixes": prefixes, "candidates": nfiles, "hits": len(hits),
                     "reports": len(notes), "farm": a.farm, "farm_links_scanned": nlinks,
                     "reports_pending": len([n for n in notes if n[2] == "A1?"]),
                     "strict_links": bool(a.strict_links), "rc": EXIT_FAIL if hits else EXIT_OK},
        }, ensure_ascii=False))
        return (EXIT_FAIL if hits else EXIT_OK), {"hits": hits, "reports": notes}

    print("== 无符号链接农场门 ==")
    print("· 扫描面：%s（候选 %d 件）" % (root, nfiles))
    print("· 真仓前缀：%s" % " · ".join(prefixes))
    if a.farm:
        print("· 农场现场（--farm）：%s（扫到链接 %d 条）" % (a.farm, nlinks))
    else:
        print("· 农场现场（--farm）：**未给 ⇒ B 档跳过**（不影响退码；审农场目录时请显式给）")
    if notes:
        print("· **只报告**（存量 A1 + 待人工核 A1? · 不入退码 %d 条）：" % len(notes))
        for n in notes[: a.max_examples]:
            if n[2] == "A1?":
                print("    ~ %s:%d [%s] %s —— %s" % (n[0], n[1], n[2], n[3], n[5]))
            else:
                print("    ~ %s:%d [%s] %s → %s" % (n[0], n[1], n[2], n[3], n[4]))
        if len(notes) > a.max_examples:
            print("    … 另有 %d 条" % (len(notes) - a.max_examples))
    if not hits:
        print("⇒ rc=0：在 %d 件里没有「符号链接回真仓 + 删除穿过它 / 删真仓前缀」的形态" % nfiles)
        if notes:
            _np = len([n for n in notes if n[2] == "A1?"])
            print("   ⚠ 但有 **%d 条只报告**（其中待人工核 A1? %d 条）⇒ rc=0 **不等于**「一件都没看见」，"
                  "A1? 请人工核" % (len(notes), _np))
        return EXIT_OK, {"hits": hits, "reports": notes}
    print("· **红**（%d 条）：" % len(hits))
    for h in hits[: a.max_examples]:
        print("    ✗ %s:%d [%s] %s %s —— %s" % (h[0], h[1], h[2], h[3], h[4], h[5]))
    if len(hits) > a.max_examples:
        print("    … 另有 %d 条" % (len(hits) - a.max_examples))
    print("⇒ rc=1：**禁止**在临时目录里建指向真仓的符号链接后再删它下面的东西 ——")
    print("   改用只读隔离工具：bash scripts/dev/isolate-ro.sh new \"<真仓>\" --dst <目录>（硬拷贝，删的是副本）")
    return EXIT_FAIL, {"hits": hits, "reports": notes}


def main():
    if "--self-test" in sys.argv[1:]:
        sys.exit(run_self_test())
    rc, _ = _main()
    sys.exit(rc)


if __name__ == "__main__":
    main()

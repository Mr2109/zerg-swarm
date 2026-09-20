#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-cmds.py — 门⑫ · 文档-命令一致性（M12 · §九 M12 + 开发文档门禁 D3 · 开工单 T-51）。

为什么要有它（§九 M12 逐字口径）
--------------------------------
命令面（`core/cmd/zerg` 的命令树）是**唯一入口**；文档（正式面 `docs/` 与开发文档面 `Zerg-内部文档/`）
里出现的 **`zerg …` 命令示例**如果拼不出命令树里的一条命令，那不是「文档写得糙」，
是**两套契约开始分叉** —— §一 1.2 的病灶同源（9 件 cmd 各写各的、退码四套并存）。
本门把「文档里的 zerg 示例必须能在命令清单里解析到」写成**可机检**的判据：
清单真源 = `bin/zerg --help` 的导出面？不 —— 真源是**命令树源码**（`core/cmd/zerg/main.go`
的 `path: []string{…}` 字面量；帮助导出物是它的下游，M16 的「导出物」口径同一条）。

M12 判到的深度（§九 M12：**判到 L2（对象 + 动作）起步**）
--------------------------------------------------------
  L1  第 1 段（族名/裸动词）：`zerg doctor` → `doctor`
  L2  前 2 段（对象 + 动作）：`zerg dev proposal` → `dev proposal`（**起步深度**，本门判到这里）
  L3+ 第 3 段只放 id/名（§3.1 深度上限）—— 不判（`zerg task show <id>` 的 `<id>` 不是命令名）。

两棵语料（§二 2.3 文档体系 + §6.4 D3 的接缝）
---------------------------------------------
  formal  = 正式面 `docs/**`（**手写命令示例在这里应当是阻断**；今天仓内 0 命中 ⇒ 一上来就能挂阻断档）
  devdocs = 开发文档面 `Zerg-内部文档/项目文档/v2.5.10/`（**先只报告档起步**：1250+ 篇历史稿，
            一次全红等于锁死提交闸 —— 照 D2 先例与《待办 D3》② 的节奏，**清到 0 再升 `tri`**）

两棵的档位**不许一样**：formal 阻断 + devdocs 只报告 —— 这个差本身就是 D3 的落地节奏
（《待办 D3》四步：①换根 ②先挂只报告 ③逐类收敛 ④升阻断）。

tri-report 语义（§6.3 S6 的「起步 tri-report + baseline 棘轮」）
--------------------------------------------------------------
  devdocs 侧的命中走 **REPORT**（只入清单、不进失败项、不影响退出码）；命中数受
  `scripts/gates/doc-cmds-baseline.json` 的 **baseline 棘轮**约束：只许下调、上调 = 红。
  formal 侧走 **tri**（rc=1 阻断）—— 基线 = 0（今天仓内 0 命中 ⇒ 起步即阻断，不放假绿）。

退码（§4.1 K3，不新立）：0 全过 · 1 有阻断红 · 2 **不给结论**（缺件/空转/判据不可判/自检不过）。

用法：
  python3 scripts/gates/check-doc-cmds.py                      # 两棵都判（默认档）
  python3 scripts/gates/check-doc-cmds.py --scope formal       # 只判正式面
  python3 scripts/gates/check-doc-cmds.py --scope devdocs      # 只判开发文档面（语料不在主仓 —— 走 ZERG_DEVDOCS_ROOT 或默认 ../Zerg-内部文档/项目文档/v2.5.10）
  python3 scripts/gates/check-doc-cmds.py --self-test          # 成对负控（好件/坏件/空转 6 组）
"""
import argparse
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_REPO = os.path.dirname(os.path.dirname(HERE))
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
BASELINE_REL = os.path.join("scripts", "gates", "doc-cmds-baseline.json")

RC_OK, RC_FAIL, RC_BLOCKED = 0, 1, 2

# 排除表：与仓内其它门的扫描口径同一张（grep --exclude-dir 的同款），不留静默的洞。
EXCL = {"target", "node_modules", "dist", "bin", "vendor", "data", ".git",
        ".venv", "venv", ".build", "__pycache__", ".history", ".obsidian"}

# 围栏/行内代码里的 `zerg …` 抽取：
#   ① 围栏块 ```…```（含 ~~~）里的整段
#   ② 行内反引号 `zerg …`
# 只认「zerg 后面跟命令词」的形态；`zerg` 单独出现（如「跑 zerg」）不抽。
ZERG_INLINE = re.compile(r'`zerg(\s+[^\s`][^`]*)?`')
FENCE = re.compile(r'^(```|~~~)')
CMD_WORD = re.compile(r'[A-Za-z][A-Za-z0-9_-]*')


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def parse_tree(root):
    """命令树 → {('dev','proposal'): True, …}（L1 与 L2 都登记：L2 命中先查，查不到退化查 L1）。"""
    src = read(os.path.join(root, MAIN_REL))
    try:
        start = src.index('commands = []*command{')
    except ValueError:
        return None
    end = src.index('\n\t}\n', start)
    seg = src[start:end]
    out = {}
    for m in re.finditer(r'path:\s*\[\]string\{([^}]*)\}', seg):
        parts = re.findall(r'"([^"]+)"', m.group(1))
        if not parts:
            continue
        out[tuple(parts)] = True
        out[tuple(parts[:1])] = True          # L1：裸动词/族名也登记
        out[tuple(parts[:2])] = True          # L2：对象 + 动作
    return out


def extract_zerg_segments(text):
    """一段 markdown → [(行号, ['dev','proposal','new',…])]（围栏内 + 行内都抽；只抽 zerg 开头）。"""
    hits, in_fence = [], False
    for lineno, line in enumerate(text.splitlines(), 1):
        stripped = line.strip()
        if FENCE.match(stripped):
            in_fence = not in_fence
            continue
        # 围栏块里的示例是文档里 `zerg …` 的主要形态：围栏正文行整行抽（去掉提示符前缀）；
        # 围栏外的普通行只抽行内反引号里的形态（防误伤「跑 zerg」这类叙述句）。
        if in_fence:
            for seg in re.split(r'(?<!\S)(?:\$|PS\s*[A-Za-z]>|>)\s*', stripped):
                m = re.match(r'zerg(\s+\S.*)?$', seg.strip())
                if m:
                    hits.append((lineno, CMD_WORD.findall(m.group(1) or "")))
        else:
            for m in ZERG_INLINE.finditer(line):
                hits.append((lineno, CMD_WORD.findall(m.group(1) or "")))
    return hits


def collect_md(root, rel_dir):
    """一棵语料树 → [(relpath, 绝对路径)]（排除表逐字对齐 EXCL）。"""
    base = os.path.join(root, rel_dir)
    if not os.path.isdir(base):
        return None
    out = []
    for dp, dn, fn in os.walk(base):
        dn[:] = [d for d in dn if d not in EXCL]
        for f in fn:
            if f.endswith(".md"):
                ap = os.path.join(dp, f)
                out.append((os.path.relpath(ap, root), ap))
    return out


def judge_md(files, tree):
    """→ (阻断命中数, 报告命中数, 明细行)。formal 的命中 = 阻断；devdocs 的命中 = 只报告。"""
    n_block, n_report, details = 0, 0, []
    for rel, ap in files:
        try:
            text = read(ap)
        except OSError as e:
            details.append("读不了 %s：%s" % (rel, e))
            continue
        for lineno, words in extract_zerg_segments(text):
            if not words:
                continue
            # L2 优先：前两段能解析 ⇒ 过；否则退化查 L1（裸动词）
            if tuple(words[:2]) in tree or tuple(words[:1]) in tree:
                continue
            n = n_block
            details.append("%s:%d → zerg %s（命令树里没有它；最像的：见 `zerg --help`）"
                           % (rel, lineno, " ".join(words[:2])))
            if n == 0:
                pass
    return n_block, n_report, details


def judge(files, tree, blocking):
    n = 0
    details = []
    for rel, ap in files:
        try:
            text = read(ap)
        except OSError as e:
            details.append("读不了 %s：%s" % (rel, e))
            continue
        for lineno, words in extract_zerg_segments(text):
            if not words:
                continue
            # L2 优先（§九 M12 起步深度：对象 + 动作）。L1 只救**单段**命令（裸动词如 `zerg doctor`）——
            # 两段里第一段族名对、第二段动作错 ⇒ 必须红（否则 `dev propozal` 这类坏动作被族名放行）。
            l2 = tuple(words[:2]) if len(words) >= 2 else None
            l1set = {t[0] for t in tree if len(t) == 1}
            if (l2 and l2 in tree) or (len(words) == 1 and words[0] in l1set):
                continue
            n += 1
            details.append("%s:%d → `zerg %s` 不在命令树（L2 判据）" % (rel, lineno, " ".join(words[:2])))
    if blocking:
        return n, 0, details
    return 0, n, details


def load_baseline(root):
    p = os.path.join(root, BASELINE_REL)
    try:
        with open(p, encoding="utf-8") as f:
            d = json.load(f)
        int(d["devdocs_report_hits"])
        return d
    except (OSError, KeyError, ValueError):
        return None


def check(root, scopes):
    if not os.path.isfile(os.path.join(root, MAIN_REL)):
        return RC_BLOCKED, ["⛔ 不给结论：命令树读不出来（%s 缺件/空转）—— 判据不可判 ⇒ 不给结论" % MAIN_REL]
    tree = parse_tree(root)
    if tree is None or not tree:
        return RC_BLOCKED, ["⛔ 不给结论：命令树读不出来（%s 缺件/空转）—— 判据不可判 ⇒ 不给结论" % MAIN_REL]
    baseline = load_baseline(root)
    if baseline is None:
        return RC_BLOCKED, ["⛔ 不给结论：baseline 缺件或解不动（%s）—— 棘轮没法判 ⇒ 不给结论" % BASELINE_REL]
    errs, lines = [], []
    devdocs_hits, formal_hits = 0, 0
    detail_all = []
    for scope in scopes:
        if scope == "formal":
            files = collect_md(root, "docs")
            if files is None:
                errs.append("formal：docs/ 不在（语料域整棵缺席）—— 不给结论档")
                continue
            if not files:
                errs.append("formal：扫到 0 篇 —— 拒绝静默空转（假绿）")
                continue
            n, _, details = judge(files, tree, blocking=True)
            formal_hits += n
            if n:
                errs.append("formal（正式面 · **阻断**）：%d 处手写 `zerg …` 示例拼不到命令树（§九 M12）" % n)
        else:
            docs_root = os.environ.get("ZERG_DEVDOCS_ROOT") or os.path.join(
                os.path.dirname(root), "Zerg-内部文档", "项目文档", "v2.5.10")
            files = collect_md(os.path.dirname(docs_root), os.path.basename(docs_root))
            if files is None:
                lines.append("devdocs：开发文档面不在（%s）—— 本机判不了这一棵（不红、给 REPORT 0）" % docs_root)
                continue
            if not files:
                errs.append("devdocs：扫到 0 篇 —— 拒绝静默空转（假绿）")
                continue
            _, r, details = judge(files, tree, blocking=False)
            devdocs_hits += r
            lines.append("devdocs（只报告档 · §6.3 S6 起步）：命中 %d 处（baseline %d）"
                         % (r, int(baseline["devdocs_report_hits"])))
        detail_all.extend(details[:20])
    # 棘轮（devdocs 侧）：只许下调
    if devdocs_hits > int(baseline["devdocs_report_hits"]):
        errs.append("T-51 棘轮：devdocs 命中 %d > baseline %d（只许**下调**）"
                    % (devdocs_hits, int(baseline["devdocs_report_hits"])))
    lines.append("命令树 %d 条（L1+L2）· formal 命中 %d（阻断档）· devdocs 命中 %d（只报告档）"
                 % (len(tree), formal_hits, devdocs_hits))
    lines.extend("  " + d for d in detail_all)
    for e in errs:
        lines.append("✗ " + e)
    return (RC_FAIL if errs else RC_OK), lines


def self_test():
    """成对负控：好件绿 / 坏件红 / 空转 2 —— 先证明这台机器能把红判出来。"""
    import tempfile
    bad = []
    tmp = tempfile.mkdtemp(prefix="doc-cmds-selftest-")
    os.makedirs(os.path.join(tmp, "core", "cmd", "zerg"), exist_ok=True)
    os.makedirs(os.path.join(tmp, "scripts", "gates"), exist_ok=True)
    tree_src = (
        "var commands []*command\n"
        "func init() {\n\tcommands = []*command{\n"
        '\t\t{path: []string{"version"}, run: cmdVersion},\n'
        '\t\t{path: []string{"dev", "proposal"}, run: cmdDevProposal},\n'
        "\t}\n}\n"
    )
    open(os.path.join(tmp, MAIN_REL), "w", encoding="utf-8").write(tree_src)
    json.dump({"devdocs_report_hits": 0, "note": "selftest"},
              open(os.path.join(tmp, BASELINE_REL), "w", encoding="utf-8"))

    def case(name, want, got):
        if want != got:
            bad.append("%s：want rc=%s got rc=%s" % (name, want, got))
        print("  %s ⇒ rc=%s" % (name, got))

    # ① 正控：好件（示例拼得到命令树）⇒ 0
    docs = os.path.join(tmp, "docs")
    os.makedirs(docs, exist_ok=True)
    open(os.path.join(docs, "good.md"), "w", encoding="utf-8").write(
        "跑这个：\n```sh\nzerg dev proposal list\n```\n行内 `zerg version` 也过。\n")
    rc, _ = check(tmp, ["formal"])
    case("① 正控（好件 ⇒ 0）", 0, rc)

    # ② 负控：坏件（拼不到）⇒ 1（formal 阻断）
    open(os.path.join(docs, "bad.md"), "w", encoding="utf-8").write(
        "```sh\nzerg dev propozal list\n```\n")
    rc, lines = check(tmp, ["formal"])
    case("② 负控（坏件 ⇒ 1）", 1, rc)
    if not any("propozal" in l for l in lines):
        bad.append("② 负控：判词里没有坏命令名")

    # ③ 空转：命令树读不出来 ⇒ 2（不给结论，不许当绿）
    os.rename(os.path.join(tmp, MAIN_REL), os.path.join(tmp, MAIN_REL) + ".bak")
    rc, _ = check(tmp, ["devdocs"])
    case("③ 空转（缺命令树 ⇒ 2）", 2, rc)
    os.rename(os.path.join(tmp, MAIN_REL) + ".bak", os.path.join(tmp, MAIN_REL))

    # ④ 负控：baseline 缺件 ⇒ 2
    os.rename(os.path.join(tmp, BASELINE_REL), os.path.join(tmp, BASELINE_REL) + ".bak")
    rc, _ = check(tmp, ["devdocs"])
    case("④ 负控（baseline 缺 ⇒ 2）", 2, rc)
    os.rename(os.path.join(tmp, BASELINE_REL) + ".bak", os.path.join(tmp, BASELINE_REL))

    # ⑤ devdocs 只报告档：坏件 ⇒ rc 仍 0（命中走 REPORT，不进退码）——
    #    但棘轮（baseline=0 · 命中 1）要把它拉红 ⇒ 判 1（棘轮有牙）
    devdocs = os.path.join(tmp, "devdocs")
    os.makedirs(devdocs, exist_ok=True)
    open(os.path.join(devdocs, "bad.md"), "w", encoding="utf-8").write(
        "```sh\nzerg dev propozal list\n```\n")
    os.environ["ZERG_DEVDOCS_ROOT"] = devdocs
    rc, lines = check(tmp, ["devdocs"])
    del os.environ["ZERG_DEVDOCS_ROOT"]
    case("⑤ devdocs 命中超 baseline ⇒ 棘轮红（1）", 1, rc)

    # ⑥ 棘轮上调放行（baseline=1 · 命中 1）⇒ 回绿（证明红红在棘轮那格，不是红在夹具）
    json.dump({"devdocs_report_hits": 1, "note": "selftest"},
              open(os.path.join(tmp, BASELINE_REL), "w", encoding="utf-8"))
    os.environ["ZERG_DEVDOCS_ROOT"] = devdocs
    rc, _ = check(tmp, ["devdocs"])
    del os.environ["ZERG_DEVDOCS_ROOT"]
    case("⑥ devdocs 命中 ≤ baseline ⇒ 0（棘轮回绿）", 0, rc)

    if bad:
        print("自检失败：")
        for b in bad:
            print("  ✗ " + b)
        return 2
    print("门⑫ 自检全过（6 组成对/空转负控）")
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True,
                                 description="门⑫ · 文档-命令一致性（M12 + D3 · T-51）")
    ap.add_argument("--scope", action="append", default=[], choices=["formal", "devdocs"],
                    help="语料域（可重复；缺省 = 两棵都判）")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        return self_test()
    root = os.path.abspath(DEFAULT_REPO)
    if not os.path.isfile(os.path.join(root, MAIN_REL)):
        print("⛔ 不给结论：找不到仓根（%s 不在）" % MAIN_REL)
        return RC_BLOCKED
    scopes = a.scope or ["formal", "devdocs"]
    rc, lines = check(root, scopes)
    print("门⑫ 文档-命令一致性（§九 M12 · §6.4 D3 · 开工单 T-51）· 仓根 %s" % root)
    for l in lines:
        print("  " + l)
    if rc == RC_OK:
        print("⇒ 全过（rc=0）")
    elif rc == RC_FAIL:
        print("⇒ 判红（rc=1）：逐条看上面的命中")
    else:
        print("⇒ 不给结论（rc=2）：缺件/空转 —— **不许当绿**")
    return rc


if __name__ == "__main__":
    sys.exit(main())

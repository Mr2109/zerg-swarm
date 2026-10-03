#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-bypass-idioms.py —— **元门**：绕闸形态登记册（缺口 `GAP-20260927-133` ⑤）

题眼（逐字引缺口 `GAP-20260927-133`）：
    「镜像器两处『**读不到 ⇒ 静默跳过**』= 本仓自己列的反面 #9『读不到被当成没坏』」
    ⇒ 本件封的是**形态**，不是单个脚本：**凡把「读不到 / 判不出」折成绿（rc=0）或整段跳过的写法，
      一律落到这张形态册上**，逐条必须带**登记五件套**。

判据（一句）
------------
**面内每一处绕闸形态，都必须对上登记册里的一条（内容锚 · 理由 · 出处 · 看着它的门）。**
对不上 ⇒ **rc=1 逐处点名**；登记了一条却一处都没对上 ⇒ **点名「登记了却没生效」**（看得见，不静默少判）。

登记五件套（每条登记的形状）
----------------------------
    {form, path, anchor, reason, source, gate}
    · form   —— 形态 id（本件 `FORMS` 表里的 id）
    · path   —— 仓内相对路径（形如 `publish/mirror-cn.sh`）
    · anchor —— **内容锚**（**不是行号**：行会挪，锚不会 —— 命中该行的稳定子串）
    · reason —— 为什么这一处可以留（不是豁免，是「已知 + 有票」）
    · source —— 出处（缺口号 / 提交号）
    · gate   —— **看着它的门**（接进哪道闸；本件自身 = 自看）

棘轮（只许减不许增）
--------------------
`REGISTRY_CEILING` = 登记册**条数上限**（写死常量）。`len(REGISTRY) > REGISTRY_CEILING` ⇒ **rc=2**
（「登记表只许减不许增」——修掉一处就删一条**并**把常量减一，机器面卡住「偷偷加登记把红洗白」）。
本件初始登记 **2 条**（`GAP-20260927-133` ①② 的两处），常量 = 2；候选件落地后应删这两条、常量改 0。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
    0 = 全绿（面内绕闸形态**逐处都有登记** · 且无「登记了却没生效」）
    1 = **有未登记的绕闸形态**（逐处点名 件:行 + 形态 id + 原文）—— 这一档就是本件要报的红
    2 = **不给结论**：扫描面**0 件**（空转）/ `FORMS` 空（判据没牙）/ 登记册超上限（棘轮被破）/
        `--self-test` 不过

用法
----
    python3 scripts/gates/check-bypass-idioms.py                       # 判真目标（publish/ + scripts/）
    python3 scripts/gates/check-bypass-idioms.py --root <目录>          # 判别的树（自检夹具入口 · 可重复）
    python3 scripts/gates/check-bypass-idioms.py --registry <file.json> # 追加登记册（自检正控用）
    python3 scripts/gates/check-bypass-idioms.py --json                 # 机器面
    python3 scripts/gates/check-bypass-idioms.py --list-forms           # 打 12 条形态 + 登记册 + 判据
    python3 scripts/gates/check-bypass-idioms.py --self-test            # 成对负控（合成夹具 · 不碰真目标）

★ 本件现读**未接闸**、**未登记** `scripts/公开标记.tsv`（本单硬约束只许改四件 ⇒ 那两处接线留给收口者，
  与同族 `scripts/gates/check-replace-rule-fixtures.py` 件头同规）。★ 本件自身**不进扫描面**（自排除）。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(HERE))
SELF_NAME = os.path.basename(__file__)
SELF_PATH = os.path.abspath(__file__)

# 通用排除表（与 §十七/§二十一 各处复跑命令同一口径）
EXCLUDE_DIRS = {"target", "node_modules", "dist", "bin", "vendor", "data", ".git",
                ".venv", "venv", ".build", "_history", "__pycache__"}
SCAN_SUFFIX = (".sh", ".py")

# ── 12 条正则形态（**唯一真源**）────────────────────────────────────────────
# 每条 = (id, regex, 一句话, 为什么是绕闸). regex 用 `re.search` 逐行判。
FORMS = [
    ("F01-DEVNULL", r"2\s*>\s*/dev/null",
     "吞掉 stderr", "读不到的原因被丢掉 ⇒ 只看得见空值、看不见失败"),
    ("F02-OR-TRUE", r"\|\|\s*true\b",
     "失败折成成功", "`|| true` 把 rc 抹平成 0 ⇒ 判据失败与通过同形"),
    ("F03-OR-COLON", r"\|\|\s*:(?:\s|$|;)",
     "no-op 兜底", "`|| :` 同上（成功码 0，无输出）"),
    ("F04-SKIP-EXIT0", r"exit\s+0\b[^\n]*(跳过|略过|skip|不做)|(跳过|略过|skip|不做)[^\n]*exit\s+0\b",
     "跳过即绿", "明写「跳过」却 `exit 0` ⇒ 未判折绿"),
    ("F05-MISSING-EXIT0", r"exit\s+0\b[^\n]*(无令牌|缺令牌|无凭据|未找到|读不到|不可达|不存在|没有)|(无令牌|缺令牌|无凭据|未找到|读不到|不可达|不存在)[^\n]*exit\s+0\b",
     "前置缺失即绿", "前置（令牌/远端/件）缺失 ⇒ `exit 0` ⇒ 未判折绿"),
    ("F06-NONEMPTY-GUARD", r"\[\s*-n\s+\"?\$[A-Za-z_][A-Za-z0-9_]*\"?\s*\]",
     "仅当非空才检", "变量取自外部（如远端基线）而空值直接跳过整段判据"),
    ("F07-LSREMOTE-SWALLOW", r"git\s+ls-remote[^\n]*(?:2\s*>\s*/dev/null|\|\|\s*true)",
     "远端基线读取吞错", "读远端吞 stderr / 折 rc ⇒ 读不到被当成没坏（GAP-133①）"),
    ("F08-CURL-FOLD", r"curl\b[^\n]*\|\|\s*(?:true|:|exit\s+0)",
     "网络失败折绿", "curl 失败被 `|| true/:` 折成成功"),
    ("F09-FORCE-FLAG", r"--force\b|--no-verify\b|--skip-[a-z][a-z-]*",
     "绕闸旗标", "强推/免检/跳步旗标须成文登记其依据"),
    ("F10-SKIP-ENV", r"\b[A-Z][A-Z0-9_]*(?:SKIP|BYPASS|FORCE)[A-Z0-9_]*\s*=\s*[1-9]",
     "环境旋钮绕闸", "`SKIP_*`/`BYPASS_*`/`FORCE_*` 环境位把门关掉"),
    ("F11-EMPTY-DEFAULT", r"\$\{[A-Za-z_][A-Za-z0-9_]*:-\}",
     "空值兜底", "`${X:-}` 把未设/空值静默折成空串"),
    ("F12-JUDGE-FOLD", r"(?:merge-base|is-ancestor|rev-parse|grep\s+-q|grep\s+-c|test\s+-|\{)[^\n]*\|\|\s*(?:true|:|exit\s+0)",
     "判据失败折绿", "祖先/解析/grep 判据失败被折成成功"),
]

# 预编译（写坏 ⇒ 建表即抛，调用方按「不给结论」处理）
COMPILED = []
for _fid, _rx, _short, _why in FORMS:
    COMPILED.append((_fid, re.compile(_rx), _short, _why))

# ── 登记册（**唯一真源** · 只许减不许增）──────────────────────────────────
REGISTRY_CEILING = 2

REGISTRY = [
    {"form": "F07-LSREMOTE-SWALLOW", "path": "publish/mirror-cn.sh",
     "anchor": "git ls-remote \"$REMOTE\" refs/heads/main 2>/dev/null | cut -f1 || true",
     "reason": "GAP-133 ① 待修：远端读不到被折成 rc=0、快进判据整段不跑",
     "source": "GAP-20260927-133", "gate": "本件（待接线）"},
    {"form": "F05-MISSING-EXIT0", "path": "publish/mirror-cn-init.sh",
     "anchor": "（无令牌 ⇒ 跳过 API 校验 ✓）",
     "reason": "GAP-133 ② 待修：无令牌 = 未判，却 `exit 0`",
     "source": "GAP-20260927-133", "gate": "本件（待接线）"},
]


def scan(roots):
    """扫面 ⇒ (occurrences, comments, files_scanned)。

    occurrence = (相对路径, 行号, 原文, [命中的形态 id…])。整行注释**不入判**（只计数）。
    本件自身按名**自排除**（正则字面量在 FORMS 表里，否则自红）。
    """
    occ, comments, files = [], 0, 0
    for root in roots:
        if not os.path.isdir(root):
            continue
        for dirpath, dirnames, filenames in os.walk(root):
            dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
            for fn in sorted(filenames):
                if not fn.endswith(SCAN_SUFFIX):
                    continue
                if fn == SELF_NAME:
                    continue
                full = os.path.join(dirpath, fn)
                # 统一用「仓根相对」；不在仓内（如自检临时树）⇒ 用 root 相对
                rel = os.path.relpath(full, REPO_ROOT) if full.startswith(REPO_ROOT + os.sep) else os.path.relpath(full, root)
                files += 1
                try:
                    text = open(full, encoding="utf-8", errors="replace").read()
                except OSError:
                    continue
                for i, line in enumerate(text.splitlines(), 1):
                    if line.lstrip().startswith("#"):
                        comments += 1
                        continue
                    hits = [fid for fid, rx, _s, _w in COMPILED if rx.search(line)]
                    if hits:
                        occ.append((rel, i, line.rstrip(), hits))
    return occ, comments, files


def judge(occ, registry):
    """⇒ (unregistered, stale)。

    unregistered = 命中形态却对不上登记册的 occurrence 列表；
    stale        = 登记了却一处都没对上的登记条目（看得见，不许静默）。
    对得上 = 登记条目 path 同、`anchor` 在本行里、且 entry.form 在本行命中的形态里。
    """
    used = set()
    unregistered = []
    for rel, lineno, line, hits in occ:
        matched = False
        for idx, e in enumerate(registry):
            if e.get("path") != rel:
                continue
            if e.get("anchor") and e["anchor"] in line and e.get("form") in hits:
                matched = True
                used.add(idx)
        if not matched:
            unregistered.append((rel, lineno, line, hits))
    stale = [(i, e) for i, e in enumerate(registry) if i not in used]
    return unregistered, stale


# ─────────────────────────── 自检（成对负控 · 合成夹具）───────────────────────────
def _st(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    bad = [0]
    print("── check-bypass-idioms.py --self-test（正控 0 · 负控 1 · 空转/用法 2）")
    # 判据表自身体检：12 条形态、且两两能编译
    if len(FORMS) == 12 and len(COMPILED) == 12:
        print("  ✓ 形态表：12 条正则形态（%s … %s）" % (FORMS[0][0], FORMS[-1][0]))
    else:
        bad[0] += 1
        print("  ✗ 形态表：期望 12 条，实得 %d" % len(FORMS))
    if len(REGISTRY) <= REGISTRY_CEILING:
        print("  ✓ 棘轮：登记册 %d 条 ≤ 上限 %d（只许减不许增）" % (len(REGISTRY), REGISTRY_CEILING))
    else:
        bad[0] += 1
        print("  ✗ 棘轮：登记册 %d 条 > 上限 %d —— 被破" % (len(REGISTRY), REGISTRY_CEILING))

    tmp = tempfile.mkdtemp(prefix="bypass-idioms-selftest-")
    try:
        bad_d = os.path.join(tmp, "bad")
        ok_d = os.path.join(tmp, "ok")
        empty_d = os.path.join(tmp, "empty")
        for d in (bad_d, ok_d, empty_d):
            os.makedirs(d)

        # 负控夹具：逐字同形 GAP-133 ①② 两处（合成远端名，不碰网络）
        bad_lines = [
            'BASE=$(git ls-remote "git@example.invalid:o/r.git" refs/heads/main 2>/dev/null | cut -f1 || true)',
            'T=$(read_token 2>/dev/null) || { echo "（无令牌 ⇒ 跳过校验 ✓）"; exit 0; }',
        ]
        with open(os.path.join(bad_d, "probe.sh"), "w", encoding="utf-8") as f:
            f.write("\n".join(bad_lines) + "\n")
        # 正控夹具：注释行不进判 + 一处已登记锚
        ok_lines = [
            "# 注释里出现 exit 0 与 2>/dev/null 都不入判",
            'BASE=$(git ls-remote "$R" refs/heads/main); LSRC=$?',
            'if [ "$LSRC" -ne 0 ]; then echo "读不到 ⇒ rc=2" >&2; exit 2; fi',
        ]
        with open(os.path.join(ok_d, "probe.sh"), "w", encoding="utf-8") as f:
            f.write("\n".join(ok_lines) + "\n")
        # 已登记锚正控：把负控那两行按锚登记 ⇒ 应转绿
        reg = [
            {"form": "F07-LSREMOTE-SWALLOW", "path": "probe.sh",
             "anchor": "2>/dev/null | cut -f1 || true", "reason": "自检", "source": "自检", "gate": "本件"},
            {"form": "F05-MISSING-EXIT0", "path": "probe.sh",
             "anchor": "（无令牌 ⇒ 跳过校验 ✓）", "reason": "自检", "source": "自检", "gate": "本件"},
        ]
        regf = os.path.join(tmp, "reg.json")
        with open(regf, "w", encoding="utf-8") as f:
            json.dump(reg, f, ensure_ascii=False)

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st(bad, "档① 负控：未登记绕闸形态（同形 GAP-133 ①②）⇒ rc=1", 1, *run("--root", bad_d))
        _st(bad, "档② 正控：等价但已收 rc 的写法（无绕闸形态）⇒ rc=0", 0, *run("--root", ok_d))
        _st(bad, "档③ 空转：面内 0 个 .sh/.py ⇒ rc=2（不许当绿）", 2, *run("--root", empty_d))
        _st(bad, "档④ 登记转绿：同一批绕闸处按内容锚登记 ⇒ rc=0（棘轮可减）", 0,
            *run("--root", bad_d, "--registry", regf))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 2


def load_registry(path):
    with open(path, encoding="utf-8") as f:
        extra = json.load(f)
    if not isinstance(extra, list):
        raise ValueError("登记册须是 JSON 数组")
    return extra


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", action="append", default=[],
                    help="判哪棵树（可重复；默认 publish/ + scripts/）")
    ap.add_argument("--registry", default="", help="追加登记册（JSON 数组 · 自检正控用）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-forms", action="store_true")
    a = ap.parse_args()

    if a.list_forms:
        print("⑯ 元门 · 绕闸形态登记册（缺口 GAP-20260927-133 ⑤）")
        print("判据：面内每一处绕闸形态都必须对上登记册一条（内容锚 · 理由 · 出处 · 看着它的门）；对不上 ⇒ rc=1")
        print("退码：0 全绿 · 1 有未登记绕闸形态 · 2 不给结论（扫面 0 件 / FORMS 空 / 登记超上限 / 自检不过）")
        print("12 条形态：")
        for fid, _rx, short, why in FORMS:
            print("  · %-20s %s —— %s" % (fid, short, why))
        print("登记册（%d 条 · 上限 %d）：" % (len(REGISTRY), REGISTRY_CEILING))
        for e in REGISTRY:
            print("  · [%s] %s  «%s»  ← %s · %s" % (e.get("form"), e.get("path"),
                                                     e.get("anchor"), e.get("source"), e.get("gate")))
        return 0

    if not FORMS:
        print("⛔ `FORMS` 为空（判据没牙）⇒ 不给结论（rc=2）", file=sys.stderr)
        return 2
    if len(REGISTRY) > REGISTRY_CEILING:
        print("⛔ 登记册 %d 条 > 上限 %d ⇒ 棘轮被破（只许减不许增）⇒ rc=2"
              % (len(REGISTRY), REGISTRY_CEILING), file=sys.stderr)
        return 2

    registry = list(REGISTRY)
    if a.registry:
        try:
            registry += load_registry(a.registry)
        except (OSError, ValueError, json.JSONDecodeError) as e:
            print("⛔ 登记册读不到/不可解析：%s ⇒ 不给结论（rc=2）" % e, file=sys.stderr)
            return 2

    roots = [os.path.abspath(r) for r in a.root] or [
        os.path.join(REPO_ROOT, "publish"), os.path.join(REPO_ROOT, "scripts")]
    occ, comments, files = scan(roots)

    if files == 0:
        print("⛔ 空转：扫描面 **0 个** `.sh`/`.py`（根写错 or 树不对）：%s" % " · ".join(roots),
              file=sys.stderr)
        print("⇒ 不给结论（rc=2）：**空转 ≠ 绿**", file=sys.stderr)
        return 2

    unreg, stale = judge(occ, registry)

    if a.json:
        print(json.dumps({
            "schema": "zerg/v1", "kind": "BypassIdiomScan", "roots": roots,
            "files_scanned": files, "occurrences": len(occ), "comments": comments,
            "forms": len(FORMS), "registry": len(REGISTRY), "registry_ceiling": REGISTRY_CEILING,
            "unregistered": len(unreg), "stale": len(stale),
            "unregistered_items": [{"file": p, "line": i, "forms": h} for (p, i, _l, h) in unreg],
            "stale_items": [{"form": e.get("form"), "path": e.get("path"), "anchor": e.get("anchor")}
                            for (_i, e) in stale],
        }, ensure_ascii=False))
        return 1 if unreg else 0

    print("── 元门 · 绕闸形态登记册（GAP-20260927-133 ⑤）──")
    print("   扫面：%s" % " · ".join(os.path.relpath(r, REPO_ROOT) if r.startswith(REPO_ROOT) else r for r in roots))
    print("   面内 .sh/.py **%d 个** · 绕闸形态命中 **%d 处** · 整行注释 %d 行（不入判）"
          % (files, len(occ), comments))
    print("   形态表 %d 条 · 登记册 %d 条（上限 %d）" % (len(FORMS), len(REGISTRY), REGISTRY_CEILING))
    for _i, e in stale:
        print("   ⚠ 登记了却没生效（本锚一处都没对上 ⇒ 应删此条并把上限减一）：[%s] %s «%s»"
              % (e.get("form"), e.get("path"), e.get("anchor")))
    if not unreg:
        print("⇒ 全绿（rc=0）：面内绕闸形态**逐处都有登记**")
        return 0
    print("   ✗ 未登记的绕闸形态 **%d 处**（逐处点名）：" % len(unreg))
    for rel, i, line, hits in unreg:
        print("      %s:%d  [%s]  %s" % (rel, i, ",".join(hits), line.strip()[:140]))
    print("⇒ 有未登记绕闸形态（rc=1）：**读不到/判不出不许折成绿** —— 要么收 rc 走 rc=2，"
          "要么按五件套登记（内容锚 · 理由 · 出处 · 看着它的门）")
    return 1


if __name__ == "__main__":
    sys.exit(main())

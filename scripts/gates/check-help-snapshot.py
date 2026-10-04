#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-help-snapshot.py —— **帮助成文快照 ↔ 运行期逐行对拍**（2026-09-24 · 波6 序70）

判据来源（**绑私有面** · 件头只写编号与读法）
------------------------------------------------
  · 组3 §一 序12（**`Q-012`**）逐字：**帮助成文快照 ↔ 运行期逐行对拍**。
  · 同面两条（**不另起命令** ✗）：`G-20`（`Q-115`）· `G-45`（`Q-074`）—— 本件只落「逐行对拍」那一半。
  · 判据（逐字两条）：
      ① `--json commands` 的 `items` **逐条含命令名**（旧状：`{"commands":"82"}` × N ⇒ 指不到是哪条 ✗）；
      ② **成文快照与运行期逐行对拍** ⇒ `rc=0`。

两半各是什么（写死）
--------------------
  ① `items_named`：现跑 `bin/zerg help export --json commands --dry-run`，取机器面那一行 JSON，
     `items` **每一条**都得有非空 `command`（行身份）；同时 `commands`（摘要格）**一个都不许丢**
     （§九 M6 字段只增不改）。
  ② `line_by_line`：拿**成文快照**（`导出-命令面帮助-<日期>.md` · 生成物）的 §一 表行 ⟷
     现跑 `bin/zerg help export --dry-run` 的**逐条清单**行 —— **逐行**比（长度必须相等、第 i 行必须同名）。
     ★ 快照是**产物**：比的是「**冻结的那一份**是否等于**今天现跑**」—— 换过制品/改过命令树
       ⇒ 快照没跟着重出，这一格当场红（这正是「成文面 ≠ 运行期」的病）。

退码
----
  0 = 两半都合（逐行一致 + 逐条有名字）
  1 = 判红：逐行不一致 / 逐条缺命令名 / 条数不等（判词点名到第几行）
  2 = **不给结论**：`bin/zerg` 不在盘 / 快照不在盘 / 快照零行 / 现跑读不回来

用法
----
    python3 scripts/gates/check-help-snapshot.py                     # 现跑 + 最新快照对拍
    python3 scripts/gates/check-help-snapshot.py --snapshot FILE     # 钉一份快照
    python3 scripts/gates/check-help-snapshot.py --docs-root DIR     # 换文档仓根（默认同级 Zerg-内部文档）
    python3 scripts/gates/check-help-snapshot.py --runtime-artifact DIR  # A 档：现跑取数改「读落盘制品」（不起 bin/zerg 子进程）
    python3 scripts/gates/check-help-snapshot.py --json              # 末尾追加机器面一行
    python3 scripts/gates/check-help-snapshot.py --list-rules        # 打判据 / 两半定义
    python3 scripts/gates/check-help-snapshot.py --self-test         # 只跑自检（合成夹具，不碰真目标）

`--runtime-artifact DIR`（缺口 `GAP-20261001-47` · A 档 · 拆档）
------------------------------------------------
判据**不得依赖运行期制品**：给本档时，两个取数口改读 `DIR` 里两份**落盘制品**（由**正门**先行
生成 · 不另起子进程 —— 本档**只读文件**）：
    · `DIR/help-export-dry-run.txt`        —— `zerg help export --dry-run` 的输出
    · `DIR/help-export-json-commands.txt`  —— `zerg help export --json commands --dry-run` 的输出
⇒ 判据结论**逐字可复现**（只由这两份件的内容决定，与 HEAD 处编译制品解耦）。
缺该档时行为**一字不动**（仍现跑 `bin/zerg` —— B 档口径）。
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile

VERSION = "1.0.0"

SNAP_GLOB = "导出-命令面帮助-"

# 现跑人面：`  zerg <命令> ｜ …`（两条清单都一样的前缀）
RUNTIME_RE = re.compile(r"^\s{2}zerg\s+(\S.*?)\s*(?:｜|$)")
# 快照 §一 表行：`| \`zerg <命令>\` | …`
SNAPSHOT_RE = re.compile(r"^\|\s*`zerg ([^`]+)`\s*\|")


def runtime_rows(binz: str):
    """现跑 `help export --dry-run` ⇒ (逐条清单行 | None, 理由)。"""
    if not os.path.isfile(binz):
        return None, "`bin/zerg` 不在盘（先用 `bash scripts/build/build-all.sh --only-cli` 出新制品）"
    p = subprocess.run([binz, "help", "export", "--dry-run"], capture_output=True, text=True,
                       stdin=subprocess.DEVNULL)
    if p.returncode != 0:
        return None, "现跑 `help export --dry-run` 退码 %d" % p.returncode
    rows = [m.group(1).strip() for m in (RUNTIME_RE.match(l) for l in p.stdout.splitlines()) if m]
    if not rows:
        return None, "现跑清单零行（制品不对 / 输出格式改了？）"
    return rows, ""


def runtime_json_rows(binz: str):
    """现跑 `help export --json commands --dry-run` ⇒ (items, 理由)。"""
    if not os.path.isfile(binz):
        return None, "`bin/zerg` 不在盘"
    p = subprocess.run([binz, "help", "export", "--json", "commands", "--dry-run"],
                       capture_output=True, text=True, stdin=subprocess.DEVNULL)
    if p.returncode != 0:
        return None, "现跑 `--json commands` 退码 %d" % p.returncode
    objs = [l for l in p.stdout.splitlines() if l.strip().startswith("{")]
    if not objs:
        return None, "现跑没有机器面那一行 JSON"
    try:
        return (json.loads(objs[-1]).get("items") or []), ""
    except ValueError as e:
        return None, "机器面 JSON 读不回来（%s）" % e


# ── A 档：只读**落盘制品**（`GAP-20261001-47` · 拆档）────────────────────────────────────────
# 与上面两个 `runtime_*` 的**唯一**不同：不吃子进程，只读一件文件 ⇒ 结论逐字可复现。
ARTIFACT_HUMAN = "help-export-dry-run.txt"
ARTIFACT_JSON = "help-export-json-commands.txt"


def runtime_rows_from_file(path: str):
    """读**落盘制品**（`help export --dry-run` 的输出先落一件）⇒ (逐条清单行 | None, 理由)。

    A 档取数口：判据**只吃文件**、不起子进程 ⇒ 结论与 HEAD 处编译制品解耦（`GAP-20261001-47`）。
    抽行口径与 `runtime_rows` **逐字同一正则**（`RUNTIME_RE`）—— 两档不许分家。
    """
    if not os.path.isfile(path):
        return None, "现跑制品不在盘：%s（先落一份 `help export --dry-run` 输出再判）" % path
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as e:
        return None, "现跑制品读不了（%s）" % e
    rows = [m.group(1).strip() for m in (RUNTIME_RE.match(l) for l in text.splitlines()) if m]
    if not rows:
        return None, "制品里一行 `zerg …` 清单都没有（制品不对 / 输出格式改了？）"
    return rows, ""


def runtime_json_rows_from_file(path: str):
    """读**落盘制品**的机器面那一行 JSON（`--json commands`）⇒ (items | None, 理由)。"""
    if not os.path.isfile(path):
        return None, "现跑制品不在盘：%s（先落一份 `help export --json commands --dry-run` 输出）" % path
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as e:
        return None, "现跑制品读不了（%s）" % e
    objs = [l for l in text.splitlines() if l.strip().startswith("{")]
    if not objs:
        return None, "制品里没有机器面那一行 JSON"
    try:
        return (json.loads(objs[-1]).get("items") or []), ""
    except ValueError as e:
        return None, "机器面 JSON 读不回来（%s）" % e


def snapshot_rows(path: str):
    """快照 §一 表行 ⇒ (命令名行, 理由)。"""
    if not os.path.isfile(path):
        return None, "快照不在盘：%s" % path
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as e:
        return None, "快照读不了（%s）" % e
    rows = [m.group(1).strip() for m in (SNAPSHOT_RE.match(l) for l in text.splitlines()) if m]
    if not rows:
        return None, "快照里一行 `zerg …` 表行都没有（§一 表被改坏了？）"
    return rows, ""


def newest_snapshot(docs_root: str):
    """文档仓里最新的 `导出-命令面帮助-*.md`（按 mtime）⇒ (路径, 理由)。"""
    hits = []
    for dp, dns, fns in os.walk(docs_root):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "__pycache__")]
        for fn in fns:
            if fn.startswith(SNAP_GLOB) and fn.endswith(".md"):
                full = os.path.join(dp, fn)
                try:
                    hits.append((os.path.getmtime(full), full))
                except OSError:
                    pass
    if not hits:
        return None, "文档仓里找不到快照（`%s*.md`）—— 先跑 `zerg help export`" % SNAP_GLOB
    hits.sort()
    return hits[-1][1], ""


def compare_lines(snap_rows, live_rows):
    """逐行比 ⇒ (相等吗, 判词表)。"""
    bad = []
    if len(snap_rows) != len(live_rows):
        bad.append("逐行对拍：行数不等 —— 快照 %d 行 / 现跑 %d 行" % (len(snap_rows), len(live_rows)))
    for i, (a, b) in enumerate(zip(snap_rows, live_rows)):
        if a != b:
            bad.append("逐行对拍：第 %d 行不同 —— 快照 `zerg %s` ⟷ 现跑 `zerg %s`" % (i + 1, a, b))
            if len(bad) >= 10:
                bad.append("（只报前 10 处差异）")
                break
    return (not bad), bad


def check_items_named(items):
    """① `items` 逐条含命令名 ⇒ 判词表。"""
    bad = []
    if not items:
        return ["`items` 是空的（零条 ⇒ 不许当绿）"]
    for i, it in enumerate(items):
        if not str(it.get("command", "")).strip():
            bad.append("第 %d 条 `items` 没有 `command`（逐条指不到是哪条）" % (i + 1))
            if len(bad) >= 10:
                bad.append("（只报前 10 条）")
                break
    if items and not all(str(it.get("commands", "")).strip() for it in items):
        bad.append("有 `items` 丢了摘要格 `commands`（字段只增不改 ⇒ 一个都不许丢）")
    return bad


def self_test(out) -> bool:
    total, failed = 0, 0

    def case(name, want, got):
        nonlocal total, failed
        total += 1
        ok = want == got
        if not ok:
            failed += 1
        out.write("  %s %s（期望 %s · 实得 %s）\n" % ("✓" if ok else "✗", name, want, got))

    tmp = tempfile.mkdtemp(prefix="hsnap-selftest-")
    good = os.path.join(tmp, "导出-命令面帮助-20260924.md")
    with open(good, "w", encoding="utf-8") as f:
        f.write("# 导出：命令面帮助（自动生成 · 勿手改）\n\n## 一、命令清单\n\n"
                "| 命令 | 说明 |\n|---|---|\n"
                "| `zerg agent ls` | 子端清单 |\n"
                "| `zerg gap ls` | 缺口账 |\n")
    rows, why = snapshot_rows(good)
    case("① 快照行抽得出来（两条）", ["agent ls", "gap ls"], rows)
    case("① 快照不在盘 ⇒ 判不了", True, "不在盘" in snapshot_rows(os.path.join(tmp, "nope.md"))[1])
    live = ["agent ls ｜ 茧壁:node ｜ 子端清单", "gap ls ｜ 茧壁:host ｜ 缺口账"]
    case("② 逐行：抽得出（前缀 `  zerg `）", ["agent ls", "gap ls"],
         [m.group(1).strip() for m in (RUNTIME_RE.match(l) for l in
                                       ["  zerg agent ls ｜ 茧壁:node ｜ 子端清单",
                                        "  zerg gap ls ｜ 茧壁:host ｜ 缺口账"]) if m])
    ok, bad = compare_lines(rows, live)
    case("② 逐行：不同 ⇒ 判红（快照说的是哪些行）", False, ok)
    case("② 逐行：判词点名到第 1 行", True, any("第 1 行" in b for b in bad))
    ok2, bad2 = compare_lines(rows, rows)
    case("② 逐行：同名同序 ⇒ 合", True, ok2)
    case("② 逐行：条数不等 ⇒ 判红", False, compare_lines(rows, rows + ["extra"])[0])
    case("③ items 逐条含命令名 ⇒ 合", [], check_items_named(
        [{"command": "zerg a", "commands": "9"}, {"command": "zerg b", "commands": "9"}]))
    case("③ items 缺命令名 ⇒ 判红", True, bool(check_items_named([{"commands": "9"}])))
    case("③ items 丢了摘要格 ⇒ 判红", True, bool(check_items_named([{"command": "zerg a"}])))
    case("③ items 零条 ⇒ 判红（不许当绿）", True, bool(check_items_named([])))
    # ── A 档（`GAP-20261001-47` · 拆档）：只读落盘制品、不起子进程 ──────────────────
    art = os.path.join(tmp, "artifact")
    os.makedirs(art)
    with open(os.path.join(art, ARTIFACT_HUMAN), "w", encoding="utf-8") as f:
        f.write("  zerg agent ls ｜ 茧壁:node ｜ 子端清单\n"
                "  zerg gap ls ｜ 茧壁:host ｜ 缺口账\n")
    with open(os.path.join(art, ARTIFACT_JSON), "w", encoding="utf-8") as f:
        f.write('{"schema":"zerg-gate/v1","items":[{"command":"zerg agent ls","commands":"9"}]}\n')
    frows, _ = runtime_rows_from_file(os.path.join(art, ARTIFACT_HUMAN))
    case("A 档 ①制品逐条清单读得出（与现跑同正则）", ["agent ls", "gap ls"], frows)
    fitems, _ = runtime_json_rows_from_file(os.path.join(art, ARTIFACT_JSON))
    case("A 档 ②制品机器面 items 读得出", ["zerg agent ls"], [(it.get("command") or "") for it in (fitems or [])])
    case("A 档 ③制品不在盘 ⇒ 判不了（不许当空清单）", True,
         "不在盘" in runtime_rows_from_file(os.path.join(art, "nope.txt"))[1])
    case("A 档 ④制品里没有清单行 ⇒ 判不了", True,
         runtime_rows_from_file(os.path.join(art, ARTIFACT_JSON))[1] != "")
    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


LIST_RULES = """判据（两半 · 波6 序70）
------------------------
  ① items_named：现跑 `bin/zerg help export --json commands --dry-run` ⇒ 机器面 `items`
     逐条含非空 `command`（行身份）· 摘要格 `commands` 一个都不丢（字段只增不改 §九 M6）
  ② line_by_line：成文快照（`导出-命令面帮助-<日期>.md` · 产物 · §一 表行）⟷ 现跑
     `bin/zerg help export --dry-run`（逐条清单行）—— 逐行比（条数相等 + 第 i 行同名）

不假装覆盖（照实登记）
----------------------
  · 只对拍**命令名**那一列（说明 / 字段面 / 端点 / 层级的逐字对拍不在本格）
  · 快照取「文档仓里 mtime 最新的那一份」；要钉版用 `--snapshot`
  · `--runtime-artifact DIR`（A 档 · 缺口 `GAP-20261001-47`）：两取数口改读**落盘制品**
    （`help-export-dry-run.txt` / `help-export-json-commands.txt`）⇒ 结论逐字可复现、与编译制品解耦；
    **谁生成制品**（正门）待拍 —— 本档只落「读」半边，**不新造第二条写路径** ✗
  · 本件**不挂门**（只单跑）—— 挂门那一半 = 另一条（不在本笔）

退码
----
  0 = 两半都合 · 1 = 判红（逐行不一致 / 逐条缺名 / 条数不等）· 2 = 不给结论（缺制品 / 缺快照 / 零行）
"""


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True, description="帮助成文快照 ↔ 运行期逐行对拍（波6 序70）")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--docs-root", default=None, help="文档仓根（默认 = 同级 Zerg-内部文档）")
    ap.add_argument("--snapshot", default=None, help="钉一份快照（默认 = 文档仓里 mtime 最新的那份）")
    ap.add_argument("--runtime-artifact", default=None,
                    help="A 档：现跑取数改「读落盘制品目录」（不起 bin/zerg 子进程 · 缺口 GAP-20261001-47）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        sys.stdout.write("── 自检（合成夹具 · 不碰真目标）──\n")
        return 0 if self_test(sys.stdout) else 2

    binz = os.path.join(a.repo_root, "bin", "zerg")
    docs_root = a.docs_root if a.docs_root is not None else os.path.join(os.path.dirname(a.repo_root), "Zerg-内部文档")
    sys.stdout.write("帮助成文快照 ↔ 运行期逐行对拍（波6 序70）· v%s\n" % VERSION)
    sys.stdout.write("  判据见 `--list-rules`：① items 逐条含命令名 · ② 快照 ⟷ 现跑 逐行\n")

    if a.runtime_artifact:
        # A 档（`GAP-20261001-47` · 拆档）：只读落盘制品 · **不起子进程** ⇒ 结论逐字可复现
        # （与 HEAD 处编译制品解耦）。缺该档 ⇒ 行为一字不动（B 档：现跑 bin/zerg）。
        sys.stdout.write("  取数档 = A 档（只读落盘制品 %s）—— 不起 bin/zerg 子进程\n" % a.runtime_artifact)
        items, jwhy = runtime_json_rows_from_file(os.path.join(a.runtime_artifact, ARTIFACT_JSON))
        live, lwhy = runtime_rows_from_file(os.path.join(a.runtime_artifact, ARTIFACT_HUMAN))
    else:
        items, jwhy = runtime_json_rows(binz)
        live, lwhy = runtime_rows(binz)
    if live is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % lwhy)
        return 2
    snap_path = a.snapshot
    if snap_path is None:
        snap_path, swhy = newest_snapshot(docs_root)
        if snap_path is None:
            sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % swhy)
            return 2
    snap, snwhy = snapshot_rows(snap_path)
    if snap is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % snwhy)
        return 2
    sys.stdout.write("  快照 = %s（%d 行）· 现跑 = %d 行\n" % (snap_path, len(snap), len(live)))

    bad = []
    item_bad = [] if items is None else check_items_named(items)
    if items is None:
        bad.append("① `--json commands` 现跑读不回来（%s）⇒ 这一半判不了" % jwhy)
    bad.extend(["① " + b for b in item_bad])
    ok, lbad = compare_lines(snap, live)
    bad.extend(["② " + b for b in lbad])
    if bad:
        sys.stdout.write("⇒ 判红（rc=1）：\n")
        for b in bad:
            sys.stdout.write("  ✗ %s\n" % b)
    else:
        sys.stdout.write("  ✓ ① items %d 条逐条含命令名（摘要格 `commands` 一条不丢）\n" % len(items))
        sys.stdout.write("  ✓ ② 逐行对拍 %d 行 ⇒ 一致\n" % len(snap))
        sys.stdout.write("⇒ 两半都合（rc=0）\n")
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-help-snapshot", "version": VERSION,
            "snapshot": snap_path, "snapshot_rows": len(snap), "live_rows": len(live),
            "items_total": len(items) if items is not None else None,
            "items_named": None if items is None else not item_bad,
            "line_by_line": ok, "bad": bad, "mode": "judge",
            "fetch_mode": "artifact" if a.runtime_artifact else "live",
        }, ensure_ascii=False) + "\n")
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())

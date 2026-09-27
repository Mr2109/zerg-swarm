#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-help-shape.py —— `help` 面的**形状判据**（2026-09-27 · 缺口 `GAP-20260927-401`）

为什么有这个件（缺口本身）
--------------------------
  `help --all` 与裸 `help` 的**形状**此前没有任何判据钉：
    · `scripts/gates/check-help-snapshot.py` 只咬 **`help export`（⟷ 文档快照）** 那一面；
    · `core/cmd/zerg/cli_help_all_test.go` 只判「条数等式 + 危险档逐条覆盖」（**数条数、不判段**）；
    · `cli_help_twostate_test.go` 判的是**两态 rc 语义**（本件禁碰）。
  ⇒ HEAD `a71aa538` 给 `help --all` 末尾追加「族名清单」时，**行数（264→272）/ 末行都无人会响** ——
    本件把这三样收进判据：**族名清单段消失 / 行数漂 / 末行被改 ⇒ 判红**。

判据（三条 · 全部「现算 + 对界」· 判据里没有一个字是自填的）
--------------------------------------------------------------
  S1 **族级段（闭集相等 · 现算两源）**
     源甲 = 现跑 `bin/zerg help --all` 末尾的族级段 token；源乙 = 命令树
     `core/cmd/zerg/testdata/cli-matrix.json` 的 `cases[].command` **首段**（去重·字典序）。
     判：段头在（形如 `族级面（<N> 族 · …）:`）· 段头括注的 `<N>` **等于**现算族数 ·
     两源**逐 token 相等**（双向差集必须都空）· 段外的裸 `help` **不许**出现族级段（只在 `--all` 档）。
     牙口：删掉整段 ⇒ token 集空 ⇒ 红；漏一族 / 多一族 ⇒ 红；族名改字 ⇒ 红。
  S2 **条数与行数（关系式 + 精确闭集常量）**
     关系式（三面现跑）：`rows(help --all) == rows(help) + rows(help dangerous)`（危险档那批行的净增）。
     关系式（段自洽）：族级段行数（自段头行算到 EOF）`== 1 + ceil(len(族)/8)`（`1` = 段头一行 ·
                     `8` = 源件布局常量 `main.go:3380` 那一行按 8 族一行）；另判**段头之前那一行必须是空行**。
     常量（**现跑取数 · 非自填**）：两个面的 `lines` / `bytes` 必须逐面相等 ——
                     取法 = `./bin/zerg help --all | wc -l -c` 与 `./bin/zerg help | wc -l -c`；
                     刷新 = 本件 `--emit-baseline`（现跑打印新值，照抄进 `BASELINE`，**别手写**）。
     牙口：`--all` 少/多一行 ⇒ 常量格红；整面漂 ⇒ 常量格红；`--all` 少一条命令行 ⇒ 关系式红。
  S3 **末行（现算重建 ⟷ 逐字）**
     末行必须是**由族名清单现算重建**的那一行（族名按字典序每 8 族一行，取最后一块，`"  " + " · ".join(块)`）⇒
     改末行任何一个字 / 掉一族 / 换分隔符都红。
     另钉裸 `help` 末行逐字（`BASELINE["help"]["last"]`）· 判据与 `--all` 末行**必须是两条不同的断言**
     （两态各自的末行都不许被改写）。

形态说明（为什么用「常量 + 关系式」而不是纯现算）
--------------------------------------------------
  · 族名 / 命令条数 / 段行数：**纯现算**（有第二个真源：命令树矩阵）。✓
  · 两个面的**总行数 / 总字节 / 裸 help 末行**：`help` 面只有一棵打印器，**没有第二个真源**能推出
    它的衬底（段头 / 隔行 / 版本行…）⇒ 纯关系式只能覆盖「相对变化」，覆盖不了「整面漂」——
    ⇒ 只能按本仓允许的第二形态：**精确闭集常量 + 成对负控**（`BASELINE` 三个数全部 **现跑取数**，
    刷新走 `--emit-baseline`，**禁手写**）。

退码
----
  0 = 三条都合 · 1 = 判红（段缺 / token 不等 / 关系式不成立 / 常量不等 / 末行不符）
  2 = **不给结论**（`bin/zerg` 不在盘 · 矩阵不在盘 / 读不回来 · 现跑退码非 0 · 输出零行）

用法
----
    python3 scripts/gates/check-help-shape.py
    python3 scripts/gates/check-help-shape.py --repo-root DIR --bin FILE --matrix FILE
    python3 scripts/gates/check-help-shape.py --emit-baseline   # 现跑打印常量新值（只读）
    python3 scripts/gates/check-help-shape.py --json
    python3 scripts/gates/check-help-shape.py --list-rules
    python3 scripts/gates/check-help-shape.py --self-test       # 合成夹具（正控 + 三负控）
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import subprocess
import sys

VERSION = "1.0.0"

MATRIX_REL = os.path.join("core", "cmd", "zerg", "testdata", "cli-matrix.json")

# 段头形如：`族级面（45 族 · 不必猜族名 · 逐族 `zerg help <族名>` 列该族全部动作）:`
SEC_HEADER_RE = re.compile(r"^族级面（(\d+) 族 ·.*）:$")
SEC_HEADER_PREFIX = "族级面（"
# 用法行前缀（与既有 Go 判据同一形态：两空格 + `zerg ` + 非空）
ROW_RE = re.compile(r"^\s{2}zerg\s+\S")

# 布局常量（源件 `core/cmd/zerg/main.go:3380` 一带）：族名清单段的**段头占 1 行**（段头之前必须另有 1 行空行）
# · 族名每行 8 族
SECTION_SCAFFOLD = 1
FAMILY_PER_LINE = 8

# ★ 精确闭集常量 —— **现跑取数**（禁手写）：
#   取法 · 1) lines/bytes：`./bin/zerg help --all | wc -l -c` · `./bin/zerg help | wc -l -c`
#          2) last：`./bin/zerg help | tail -n 1`（逐字）
#   刷新：本件 `--emit-baseline` 现跑打印，照抄。
BASELINE = {
    "help --all": {"lines": 272, "bytes": 67259},
    "help": {"lines": 216, "bytes": 47493,
             "last": "      非交互用 `--token-stdin`；详见 `zerg help credentials`）。"},
}


# ── 现跑 / 读文件（都不是判据，只负责取数）──
def run_face(binz: str, argv):
    if not os.path.isfile(binz):
        return None, "`%s` 不在盘" % binz
    p = subprocess.run([binz] + list(argv), capture_output=True, text=True, stdin=subprocess.DEVNULL)
    if p.returncode != 0:
        return None, "现跑 `%s` 退码 %d" % (" ".join(argv), p.returncode)
    if not p.stdout.strip():
        return None, "现跑 `%s` 零输出" % " ".join(argv)
    return p.stdout, ""


def matrix_commands(path: str):
    if not os.path.isfile(path):
        return None, "命令树矩阵不在盘：%s" % path
    try:
        with open(path, encoding="utf-8") as f:
            obj = json.load(f)
    except (OSError, ValueError) as e:
        return None, "命令树矩阵读不回来（%s）" % e
    cmds = sorted({str(c.get("command", "")).strip() for c in (obj.get("cases") or [])
                   if str(c.get("command", "")).strip()})
    if not cmds:
        return None, "命令树矩阵里一条 `command` 都没有（矩阵被改坏 / 换口径了？）"
    return cmds, ""


# ── 纯函数：形状抽取 ──
def rows_of(text: str):
    return [l for l in text.splitlines() if ROW_RE.match(l)]


def family_section(text: str):
    """⇒ (段头行号 | None, 段头括注的族数 | None, token 表, 段行数 | None)。"""
    lines = text.splitlines()
    idx = [i for i, l in enumerate(lines) if l.startswith(SEC_HEADER_PREFIX)]
    if not idx:
        return None, None, [], None
    i = idx[0]
    m = SEC_HEADER_RE.match(lines[i].strip())
    num = int(m.group(1)) if m else None
    body = lines[i + 1:]
    toks = [t.strip() for l in body for t in l.split("·") if t.strip()]
    return i, num, toks, len(lines) - i


def families_of(cmds):
    return sorted({c.split()[0] for c in cmds if c.split()})


def rebuilt_last_family_line(fams):
    """族名清单末行的**现算重建**（每 8 族一行，取最后一块）。"""
    chunk = fams[(len(fams) // FAMILY_PER_LINE) * FAMILY_PER_LINE:]
    return "  " + " · ".join(chunk)


def judge(all_text: str, bare_text: str, danger_text: str, cmds, baseline=None) -> list:
    """判据本体（**唯一一处** · 纯函数 · 不现跑、不读盘）。"""
    bl = BASELINE if baseline is None else baseline
    bad = []
    fams = families_of(cmds)

    # ---- S1 族级段（闭集相等）----
    idx, num, toks, sec_lines = family_section(all_text)
    if idx is None:
        bad.append("S1 族级段：`help --all` 末尾**没有**族级段（段头 `%s` 找不到）—— "
                   "族名清单消失（这正是本件要防的病）" % SEC_HEADER_PREFIX)
    else:
        if num != len(fams):
            bad.append("S1 族级段：段头括注 %r 族 ≠ 现算命令树族数 %d" % (num, len(fams)))
        miss = [f for f in fams if f not in toks]
        extra = [t for t in toks if t not in fams]
        if miss:
            bad.append("S1 族级段：漏了 %d 族（前 5：%s）—— 命令树里有、清单里没有"
                       % (len(miss), " · ".join(miss[:5])))
        if extra:
            bad.append("S1 族级段：多了 %d 项（前 5：%s）—— 清单里有、命令树里没有"
                       % (len(extra), " · ".join(extra[:5])))
        if not miss and not extra and toks != fams:
            bad.append("S1 族级段：token 集合相同但**次序不同**（族名清单要按字典序，"
                       "`help --all` 与命令树的序必须一致）")
        if bare_text and family_section(bare_text)[0] is not None:
            bad.append("S1 边界：裸 `help` 里出现了族级段 —— 族名清单**只许**在 `--all` 这一档末尾")
        prev = all_text.splitlines()[idx - 1] if idx > 0 else None
        if prev != "":
            bad.append("S1 族级段：段头之前那一行不是空行（%r）—— 段的形状漂了" % prev)

    # ---- S2 条数与行数（关系式 + 常量）----
    r_all, r_bare, r_dang = len(rows_of(all_text)), len(rows_of(bare_text)), len(rows_of(danger_text))
    if r_all != r_bare + r_dang:
        bad.append("S2 关系式：rows(`help --all`)=%d ≠ rows(`help`)=%d + rows(`help dangerous`)=%d（= %d）"
                   % (r_all, r_bare, r_dang, r_bare + r_dang))
    if idx is not None:
        want = SECTION_SCAFFOLD + math.ceil(len(fams) / FAMILY_PER_LINE)
        if sec_lines != want:
            bad.append("S2 关系式：族级段 %d 行 ≠ 2 + ceil(%d 族 / %d 族一行) = %d"
                       % (sec_lines, len(fams), FAMILY_PER_LINE, want))
    for face, text in (("help --all", all_text), ("help", bare_text)):
        want = bl.get(face, {})
        got_l, got_b = len(text.splitlines()), len(text.encode("utf-8"))
        if "lines" in want and got_l != want["lines"]:
            bad.append("S2 常量：`%s` 行数 %d ≠ 冻结读数 %d（形状漂了 —— 取法见件头 `BASELINE` 注释）"
                       % (face, got_l, want["lines"]))
        if "bytes" in want and got_b != want["bytes"]:
            bad.append("S2 常量：`%s` 字节 %d ≠ 冻结读数 %d" % (face, got_b, want["bytes"]))

    # ---- S3 末行 ----
    all_last = all_text.splitlines()[-1] if all_text.splitlines() else ""
    want_last = rebuilt_last_family_line(fams)
    if all_last != want_last:
        bad.append("S3 末行：`help --all` 末行 %r ≠ 由族名清单现算重建的 %r" % (all_last, want_last))
    bare_last = bare_text.splitlines()[-1] if bare_text.splitlines() else ""
    want_bare = bl.get("help", {}).get("last")
    if want_bare is not None and bare_last != want_bare:
        bad.append("S3 末行：裸 `help` 末行 %r ≠ 冻结读数 %r" % (bare_last, want_bare))
    return bad


# ── 夹具（合成 · 不碰真目标）──
FIX_CMDS = ["agent ls", "agent load", "gap ls", "gate run", "help", "model show",
            "repo commit", "task ls", "version", "watch"]


def fix_texts():
    """按真件形态现造三面文本（族名清单 = 字典序 · 每 8 族一行）。"""
    fams = families_of(FIX_CMDS)
    rows_all = ["  zerg " + c + " ｜ 茧壁:host ｜ 说明" for c in FIX_CMDS]
    rows_bare = rows_all[:5]
    rows_dang = rows_all[5:]
    all_text = ("zerg —— 形状夹具 · 深度 ≤ 3 层\n\n" + "\n".join(rows_all)
                + "\n\n族级面（%d 族 · 不必猜族名 · 逐族 `zerg help <族名>` 列该族全部动作）:\n" % len(fams)
                + "\n".join("  " + " · ".join(fams[i:i + FAMILY_PER_LINE])
                            for i in range(0, len(fams), FAMILY_PER_LINE)) + "\n")
    bare_text = ("zerg —— 形状夹具\n\n" + "\n".join(rows_bare)
                 + "\n" + BASELINE["help"]["last"] + "\n")
    dang_text = "\n".join(rows_dang) + "\n"
    return all_text, bare_text, dang_text


def self_test(out) -> bool:
    total, failed = 0, 0

    def case(name, want_red, bad):
        nonlocal total, failed
        total += 1
        got_red = bool(bad)
        ok = got_red == want_red
        if not ok:
            failed += 1
        out.write("  %s %s（期望%s · 实得%s）\n"
                  % ("✓" if ok else "✗", name, "红" if want_red else "绿", "红" if got_red else "绿"))
        for b in bad[:3]:
            out.write("      ↳ %s\n" % b)

    all_t, bare_t, dang_t = fix_texts()
    # 夹具基线（否则常量格必红 ⇒ 干扰判别）：只钉夹具自己的数
    bl = {"help --all": {"lines": len(all_t.splitlines()), "bytes": len(all_t.encode())},
          "help": {"lines": len(bare_t.splitlines()), "bytes": len(bare_t.encode()),
                   "last": BASELINE["help"]["last"]}}

    case("正控：原样 ⇒ 必绿", False, judge(all_t, bare_t, dang_t, FIX_CMDS, bl))

    lines = all_t.splitlines()
    idx = [i for i, l in enumerate(lines) if l.startswith(SEC_HEADER_PREFIX)][0]
    # 负控 ①：删掉族名清单段（段前空行 + 段头 + 族名行全删）
    case("负控①：删掉族级段 ⇒ 必红", True,
         judge("\n".join(lines[:idx - 1]) + "\n", bare_t, dang_t, FIX_CMDS, bl))
    # 负控 ②：行数偏移（掉一行族名行）
    case("负控②a：族级段掉一行 ⇒ 必红", True,
         judge("\n".join(lines[:idx + 2] + lines[idx + 3:]) + "\n", bare_t, dang_t, FIX_CMDS, bl))
    case("负控②b：整面多一行 ⇒ 必红", True,
         judge(all_t + "多余一行\n", bare_t, dang_t, FIX_CMDS, bl))
    # 负控 ③：末行改写
    case("负控③a：`--all` 末行改写 ⇒ 必红", True,
         judge("\n".join(lines[:-1] + ["  task · ui · update · version · watch-XX"]) + "\n",
               bare_t, dang_t, FIX_CMDS, bl))
    case("负控③b：裸 `help` 末行改写 ⇒ 必红", True,
         judge(all_t, "\n".join(bare_t.splitlines()[:-1] + ["凭据：改过的一句。"]) + "\n",
               dang_t, FIX_CMDS, bl))
    # 负控 ④：族名清单漏一族（命令树有、清单没有）
    case("负控④：清单漏一族 ⇒ 必红", True,
         judge(all_t.replace("watch", "watchx"), bare_t, dang_t, FIX_CMDS, bl))
    # 对界：命令树漂了（树里多一条命令）⇒ 必红
    case("对界：命令树多一族 ⇒ 必红", True,
         judge(all_t, bare_t, dang_t, FIX_CMDS + ["zzz ls"], bl))

    out.write("  自检 %d 格 · 失败 %d 格\n" % (total, failed))
    return failed == 0


LIST_RULES = """判据（三条 · 现算 + 对界 · 缺口 GAP-20260927-401）
-----------------------------------------------
  S1 族级段（闭集相等）：现跑 `help --all` 末尾段 token ⟷ 命令树（`cli-matrix.json` 的
     `cases[].command` 首段）· 段头在 · 括注族数 == 现算族数 · 双向差集空 · 裸 `help` 不带段
  S2 条数与行数：rows(--all) == rows(help) + rows(help dangerous) ·
     族级段行数 == 1 + ceil(族数/8)（段头之前必须是空行）；两个面的 lines/bytes == 冻结常量
     （现跑取数 · --emit-baseline 刷）
  S3 末行：`--all` 末行 == 由族名清单现算重建的那一行；裸 `help` 末行 == 冻结读数（逐字）

不假装覆盖（照实登记）
----------------------
  · 只判**形状**（段 / 条数 / 行数 / 末行）；段的**说明文字**逐字、危险档判词、层级列不在本格
  · 两态 rc 语义（`zerg <族> --help` 退 2 等）**不在本件**（那是 `cli_help_twostate_test.go` 的面，禁碰）
  · 建件时本件**未挂门**（挂门 + `scripts/公开标记.tsv` + `dryrun-semantics.json` 三处登记是随动口）
"""


def main() -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    repo_default = os.path.dirname(os.path.dirname(here))
    ap = argparse.ArgumentParser(add_help=True, description="`help` 面的形状判据（族级段 / 条数 / 末行）")
    ap.add_argument("--repo-root", default=repo_default)
    ap.add_argument("--bin", default=None)
    ap.add_argument("--matrix", default=None)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--emit-baseline", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if a.self_test:
        sys.stdout.write("── 自检（合成夹具 · 不碰真目标）──\n")
        return 0 if self_test(sys.stdout) else 2

    binz = a.bin or os.path.join(a.repo_root, "bin", "zerg")
    matrix = a.matrix or os.path.join(a.repo_root, MATRIX_REL)

    cmds, cwhy = matrix_commands(matrix)
    if cmds is None:
        sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % cwhy)
        return 2
    faces = {}
    for face, argv in (("help --all", ["help", "--all"]), ("help", ["help"]),
                       ("help dangerous", ["help", "dangerous"])):
        text, why = run_face(binz, argv)
        if text is None:
            sys.stdout.write("⛔ 不给结论（rc=2）：%s\n" % why)
            return 2
        faces[face] = text

    if a.emit_baseline:
        for face, text in faces.items():
            sys.stdout.write("%s\tlines=%d\tbytes=%d\tlast=%s\n"
                             % (face, len(text.splitlines()), len(text.encode("utf-8")),
                                text.splitlines()[-1] if text.splitlines() else ""))
        return 0

    sys.stdout.write("`help` 面形状判据 · v%s（族级段 / 条数 / 末行）\n" % VERSION)
    fams = families_of(cmds)
    idx, num, toks, sec_lines = family_section(faces["help --all"])
    sys.stdout.write("  命令树 %d 条 ⇒ 现算 %d 族 · `help --all` %d 行 / %d 字节 · 裸 `help` %d 行 · "
                     "族级段 %s 行（token %d）\n"
                     % (len(cmds), len(fams), len(faces["help --all"].splitlines()),
                        len(faces["help --all"].encode("utf-8")), len(faces["help"].splitlines()),
                        sec_lines if idx is not None else "缺", len(toks)))
    bad = judge(faces["help --all"], faces["help"], faces["help dangerous"], cmds)
    if bad:
        sys.stdout.write("⇒ 判红（rc=1）：\n")
        for b in bad:
            sys.stdout.write("  ✗ %s\n" % b)
    else:
        sys.stdout.write("  ✓ S1 族级段 %d 族 ⟷ 命令树逐 token 相等（裸 `help` 不带段）\n" % len(fams))
        sys.stdout.write("  ✓ S2 rows(--all)=%d == rows(help)=%d + rows(dangerous)=%d（危险档净增）；"
                         "两个面行/字节 == 冻结读数（段 %d 行 · 命令树 %d 条为族名另一源）\n"
                         % (len(rows_of(faces["help --all"])), len(rows_of(faces["help"])),
                            len(rows_of(faces["help dangerous"])),
                            sec_lines if sec_lines is not None else -1, len(cmds)))
        sys.stdout.write("  ✓ S3 末行 = 族名清单现算重建的那一行（裸 `help` 末行逐字符合）\n")
        sys.stdout.write("⇒ 三条都合（rc=0）\n")
    if a.json:
        sys.stdout.write(json.dumps({
            "schema": "zerg-gate/v1", "gate": "check-help-shape", "version": VERSION,
            "commands": len(cmds), "families": len(fams),
            "all_lines": len(faces["help --all"].splitlines()),
            "all_bytes": len(faces["help --all"].encode("utf-8")),
            "bare_lines": len(faces["help"].splitlines()),
            "family_section_lines": sec_lines, "family_tokens": len(toks),
            "bad": bad, "mode": "judge",
        }, ensure_ascii=False) + "\n")
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-fields-face-parity.py —— 命令级 `fields` ⟷ 面级 `--json` 键集 对拍门（缺口 `Q-178`）。

为什么要有它（缺口账 `Q-178` · 2026-09-26 本枚现读复现）
  `check-cli-contract.py:113` 逐字 `'fields': bool(FIELDS_RE.search(block))` —— 它把
  `fields: []string{…}` 抓到的**名单折成 bool**，只判「**有没有**字段表」、**不判对不对** ✗。
  ⇒ 「命令树里声明的字段面」⟷「该命令**真吐**的 `--json` 键集」**无一条对拍**：
  两处名不同（声明多一名 / 少一名 / 拼错），**没有任何一层会报** —— 门全绿而消费方按声明拿键必空。

判据（任一不满足 ⇒ rc=1；判不了 / 空转 ⇒ rc=2，**不给结论**）
  ① **声明侧**：`core/cmd/zerg/main.go` 命令树里每条 `path: []string{…} … fields: []string{…}`
     ⇒ 该命令的**声明字段集**（`FIELDS_RE` 同一形态，本件自己解析、不复用 bool）。
  ② **真身侧**：现跑 `<bin> <path…> --json zzz-nonexistent-zz`（**故意给一个不在表里的名**）
     ⇒ 字段面**执行前判** ⇒ rc=2 + stderr 逐字 `合法字段: a,b,c` = **真身闭集**。
     ★ 「不给字段」那一格**不是**本门的判据（那是 `K2` 甲/乙档 · 见 `Q-119`）—— 本门只对**名**。
  ③ **逐名对拍**：声明集 == 真身集 ⇒ 过；多一名 / 少一名 / 抄错一名 ⇒ 红并**逐名点名**。
  ④ **不给静默**：真身表取不到的命令（前置校验先于字段面 / 要活服务 / 重命令）**逐条点名 + 计数**，
     并受**棘轮**约束（`--min-covered` 默认 29）= 「今天能对拍的下限」，**只许上调、永不下调**。

用法
  python3 scripts/gates/check-fields-face-parity.py [仓库根]
  python3 scripts/gates/check-fields-face-parity.py --self-test   # 成对负控（合成声明/真身）
  python3 scripts/gates/check-fields-face-parity.py --json        # 机器读面

退码：0 全对拍且覆盖 ≥ 棘轮 · 1 有逐名差异 · 2 判不了（缺件 / 制品不在 / 覆盖不够 ⇒ 不许当绿）。
纪律：**只读**（不写仓）· 不联网 · 命令一律 `--json <坏名>` 走**用法错**这一格（不落任何写路径）。
"""
import argparse
import json
import os
import re
import subprocess
import sys

CMD_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
PATH_SEG_RE = re.compile(r'"([^"]+)"')
FIELDS_RE = re.compile(r'fields:\s*\[\]string\{([^}]*)\}')
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
LEGAL_FIELDS_PREFIX = "合法字段:"
BOGUS = "zzz-nonexistent-zz"
PER_CMD_TIMEOUT = 15
MIN_COVERED_DEFAULT = 29


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def declared_faces(root):
    """main.go 命令树 → [(path, [声明字段…])]（只列**有**字段表的命令）。"""
    try:
        src = read(os.path.join(root, MAIN_REL))
    except OSError:
        return None
    try:
        start = src.index("commands = []*command{")
        end = src.index("\n\t}\n", start)
    except ValueError:
        return None
    seg = src[start:end]
    out, ms = [], list(CMD_RE.finditer(seg))
    for i, m in enumerate(ms):
        parts = PATH_SEG_RE.findall(m.group(1))
        if not parts:
            continue
        nxt = ms[i + 1].start() if i + 1 < len(ms) else len(seg)
        fm = FIELDS_RE.search(seg[m.end():nxt])
        if fm:
            out.append((" ".join(parts), [s.strip() for s in PATH_SEG_RE.findall(fm.group(1))]))
    return out


def runtime_face(binpath, path, timeout=PER_CMD_TIMEOUT):
    """现跑一条命令的**真身**字段表 → (字段列表 | None, 原因)。

    只走 `--json <坏名>` 这一格（字段面**执行前判**）：rc=2 且 stderr 有「合法字段: …」⇒ 真身拿到。
    其它 rc / 无表 ⇒ 给原因（**逐条点名，不许静默**）。
    """
    argv = [binpath] + path.split() + ["--json", BOGUS]
    try:
        p = subprocess.run(argv, capture_output=True, text=True, stdin=subprocess.DEVNULL,
                           timeout=timeout)
    except subprocess.TimeoutExpired:
        return None, "超时（>{0}s ⇒ 不算对拍面）".format(timeout)
    except OSError as e:
        return None, "跑不起来：%s" % e
    for line in p.stderr.splitlines():
        if line.strip().startswith(LEGAL_FIELDS_PREFIX):
            return [s.strip() for s in line.split(":", 1)[1].split(",") if s.strip()], ""
    return None, "无字段表（rc=%d ⇒ 前置校验先于字段面 / 要活服务）" % p.returncode


def compare(decl, real):
    """→ (extra, missing)：声明多出来的 / 真身多出来的（**逐名**）。"""
    ds, rs = set(decl), set(real)
    return sorted(ds - rs), sorted(rs - ds)


def run(root, binpath, min_covered, timeout):
    decl = declared_faces(root)
    if decl is None:
        return 2, {"error": "读不到命令树（%s）⇒ 判不了" % MAIN_REL}
    if not os.path.exists(binpath):
        return 2, {"error": "缺制品 %s ⇒ 判不了（不许拿声明自己跟自己比）" % binpath}
    ok, mism, nope, covered = [], [], [], 0
    for path, fields in decl:
        real, why = runtime_face(binpath, path, timeout)
        if real is None:
            nope.append({"path": path, "reason": why})
            continue
        covered += 1
        extra, missing = compare(fields, real)
        if extra or missing:
            mism.append({"path": path, "declared_extra": extra, "runtime_extra": missing})
        else:
            ok.append(path)
    rc = 0
    if mism:
        rc = 1
    elif covered < min_covered:
        rc = 2  # 覆盖不够 = 空转（「配置没了所以全绿」那一类假绿）
    return rc, {"declared": len(decl), "covered": covered, "min_covered": min_covered,
                "matched": ok, "mismatched": mism, "not_comparable": nope}


def selftest():
    ok = True

    def chk(name, cond, why=""):
        nonlocal ok
        print(("   ✓ " if cond else "   ✗ ") + name + (("（%s）" % why) if (why and not cond) else ""))
        ok = ok and bool(cond)

    # 正控：逐名相等 ⇒ 无差
    chk("正控：声明 == 真身 ⇒ 无差", compare(["a", "b"], ["a", "b"]) == ([], []))
    # 负控①：声明多一名 ⇒ 必须点名
    e, m = compare(["a", "b", "c"], ["a", "b"])
    chk("负控①：声明多一名 ⇒ 点名 extra=['c']", e == ["c"] and m == [], str((e, m)))
    # 负控②：声明少一名 ⇒ 必须点名
    e, m = compare(["a"], ["a", "b"])
    chk("负控②：声明少一名 ⇒ 点名 runtime_extra=['b']", e == [] and m == ["b"], str((e, m)))
    # 负控③：抄错一名（同数不同名）⇒ 两边都点名（**同数不是对拍**）
    e, m = compare(["a", "z"], ["a", "b"])
    chk("负控③：同数不同名 ⇒ 两边都点名", e == ["z"] and m == ["b"], str((e, m)))
    # 负控④：顺序不同**不算**差（名集对拍，不比序）
    chk("负控④：仅顺序不同 ⇒ 无差（不比序）", compare(["b", "a"], ["a", "b"]) == ([], []))
    # 解析面：合成命令树 ⇒ 抓得到 path + 声明名单（不是 bool）
    import tempfile
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        with open(os.path.join(d, MAIN_REL), "w", encoding="utf-8") as f:
            f.write("\tcommands = []*command{\n"
                    "\t\t{path: []string{\"zz\", \"ls\"}, fields: []string{\"name\", \"verdict\"}},"
                    " run: x},\n\t}\n")
        got = declared_faces(d)
        chk("解析面：合成树 ⇒ 名单逐字（不是 bool）",
            got == [("zz ls", ["name", "verdict"])], str(got))
    # 负控⑤：制品不在 ⇒ rc=2（不许拿声明自己跟自己比）
    rc, _ = run("/nonexistent-zerg-root-zz", "/nonexistent-bin-zz", 1, 2)
    chk("负控⑤：读不到命令树 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    print("── " + ("全过" if ok else "有红") + " ──")
    return 0 if ok else 1


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("root", nargs="?", default=None)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--bin", default=None)
    ap.add_argument("--min-covered", type=int, default=MIN_COVERED_DEFAULT)
    ap.add_argument("--timeout", type=float, default=PER_CMD_TIMEOUT)
    a = ap.parse_args()
    if a.self_test:
        print("── check-fields-face-parity 自检（成对负控 · 合成夹具 · 不碰真目标）──")
        return selftest()
    root = os.path.abspath(a.root) if a.root else os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    binpath = a.bin or os.path.join(root, "bin", "zerg")
    rc, res = run(root, binpath, a.min_covered, a.timeout)
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "FieldsFaceParity", "items": [res],
                          "meta": {"count": 1, "source": "local（本机）"},
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    if "error" in res:
        print("✗ " + res["error"])
        return rc
    print("== 命令级 `fields` ⟷ 面级 `--json` 键集 对拍（缺口 `Q-178`）==")
    print("   声明侧（%s）：%d 条带字段表的命令" % (MAIN_REL, res["declared"]))
    print("   真身侧（现跑 `--json %s` ⇒ `合法字段:` 表）：可对拍 %d 条 · 不可对拍 %d 条"
          % (BOGUS, res["covered"], len(res["not_comparable"])))
    for x in res["not_comparable"]:
        print("   ○ 不给静默：%s —— %s" % (x["path"], x["reason"]))
    if res["mismatched"]:
        for x in res["mismatched"]:
            print("   ✗ %s：声明多 %s · 真身多 %s" % (x["path"], x["declared_extra"], x["runtime_extra"]))
    print("== 结论 ==")
    if res["mismatched"]:
        print("   ⇒ 红（rc=1）：声明集 ⟷ 真身集有逐名差异（%d 条）" % len(res["mismatched"]))
    elif res["covered"] < res["min_covered"]:
        print("   ⇒ 判不了（rc=2）：可对拍 %d < 棘轮 %d（**只许上调** —— 覆盖掉了就是空转）"
              % (res["covered"], res["min_covered"]))
    else:
        print("   ⇒ 绿（rc=0）：%d 条逐名相等 · 可对拍 %d ≥ 棘轮 %d"
              % (len(res["matched"]), res["covered"], res["min_covered"]))
    return rc


if __name__ == "__main__":
    sys.exit(main())

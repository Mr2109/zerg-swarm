#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""gate-check-health.py —— 判据转发壳 `zerg gate check` 的**体检件**（M1-07 · 承 `P22` 并入）

并入两档读数（✗ **不单立第二个顶层面** · 设计稿 9.2 前置④「平台底线并入体检件」）：

  ① **平台底线**：`bash` `3.2` ＋ `python` `3.9` —— 转发壳与它调起的判据件都跑在这两档上；
     低于底线 ⇒ 红（rc=1）；读数取不到 ⇒ **不给结论**（rc=2 · 不猜）。
  ② **库类不特殊化**（拍定第五条）：壳的名录**只按后缀收件**（`.py`⇒`python3` / `.sh`⇒`bash`），
     ✗ 不设**按名放行/排除名单**、✗ 不升格/不剔除库类件。判据面 = 现读壳件
     `core/cmd/zerg/family_gate_check.go`：解释器闭集恰为 `.py`/`.sh` ＋ 名录是**目录派生**
     （含 `filepath.WalkDir`）＋ 件内无写死的 `"scripts/…"` 名件路径（那是按名名单的形态）。

另附一格（只报告 · 不进 ① ② 的判红）：`scripts/gates/judge-registry.json` 若在盘，则其条目数须
等于现读名录件数（复用 `check-judge-registry.py` 的 `scan_roster` —— **单一真源，不另抄名录**）。

退码（三档 · fail-closed）：
  0 = 两档读数达底线 ＋ 库类不特殊化成立
  1 = 违例（平台低于底线 / 壳件出现按名名单或库类特殊化 / 注册表条数与现读名录不符）
  2 = **不给结论**：版本读数取不到 / 壳件读不到 / `--self-test` 不过

用法：
    python3 scripts/gates/gate-check-health.py              # 真目标体检
    python3 scripts/gates/gate-check-health.py --json       # 机器面（同一结构多渲染）
    python3 scripts/gates/gate-check-health.py --self-test  # 成对负控自检
"""
import argparse
import importlib.util
import json
import os
import re
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GO_SHELL = "core/cmd/zerg/family_gate_check.go"
REGISTRY = "scripts/gates/judge-registry.json"
MIN_TOOL = {"bash": (3, 2), "python": (3, 9)}
# 按名名单 / 库类特殊化的形态标记（壳件里出现任一 ⇒ 红）。✗ 不把注释里的「库类」二字当命中。
NAME_LIST_MARKERS = ("gateCheckAllow", "gateCheckDeny", "gateSkipNames", "gateExcludeNames",
                     "gateLibrary", "gateSpecialCase")


def parse_version(text):
    """取第一段数字点号版本 ⇒ (major, minor[,…])；取不到 ⇒ None。"""
    if not text:
        return None
    m = re.search(r"(\d+)\.(\d+)(?:\.(\d+))?", text)
    if not m:
        return None
    return tuple(int(g) for g in m.groups() if g is not None)


def probe_bash():
    try:
        out = subprocess.run(["bash", "--version"], capture_output=True, text=True, timeout=10)
        first = (out.stdout or "").splitlines()[0] if out.stdout else ""
        return parse_version(first)
    except Exception:
        return None


def probe_python():
    return tuple(sys.version_info[:3])


def check_go_shell(text):
    """壳件判据：返回问题清单（空 = 库类不特殊化成立）。"""
    problems = []
    m = re.search(r"gateCheckInterpreter\s*=\s*map\[string\]string\{(.*?)\}", text, re.S)
    if not m:
        problems.append("壳件里找不到 `gateCheckInterpreter` 解释器闭集 ⇒ 判不了")
    else:
        pairs = re.findall(r'"([^"]*)"\s*:\s*"([^"]*)"', m.group(1))
        keys = set(k for k, _ in pairs)
        if keys != {".py", ".sh"}:
            problems.append("解释器闭集不是 {.py, .sh}（现读 %s）⇒ 疑似对某类件特殊化" % sorted(keys))
    if "filepath.WalkDir" not in text:
        problems.append("名录不是**目录派生**（缺 `filepath.WalkDir`）⇒ 疑似按名名单")
    if '"scripts/' in text:
        problems.append("壳件里出现写死的 `\"scripts/…\"` 名件路径 ⇒ 疑似按名名单（把库类升格/剔除）")
    for mk in NAME_LIST_MARKERS:
        if mk in text:
            problems.append("壳件里出现按名名单/特殊化标记：%s" % mk)
    return problems


def load_registry_module():
    """复用 `check-judge-registry.py` 的 `scan_roster`（单一真源 · 不另抄名录）。"""
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-judge-registry.py")
    if not os.path.isfile(p):
        return None
    spec = importlib.util.spec_from_file_location("_judge_registry", p)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def _read(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        return f.read()


def run_health(root, go_shell_rel=GO_SHELL, registry_rel=REGISTRY):
    """⇒ (rc, report)。"""
    rep = {"root": root, "checks": {}, "problems": [], "notes": []}
    bash_v = probe_bash()
    py_v = probe_python()
    rep["checks"]["bash"] = ".".join(str(x) for x in bash_v) if bash_v else None
    rep["checks"]["python"] = ".".join(str(x) for x in py_v)

    if bash_v is None:
        rep["problems"].append("bash 版本读数取不到 ⇒ 不给结论")
    elif bash_v[:2] < MIN_TOOL["bash"]:
        rep["problems"].append("bash %s 低于底线 %s" % (".".join(map(str, bash_v)),
                                                         ".".join(map(str, MIN_TOOL["bash"]))))
    if py_v[:2] < MIN_TOOL["python"]:
        rep["problems"].append("python %s 低于底线 %s" % (".".join(map(str, py_v)),
                                                          ".".join(map(str, MIN_TOOL["python"]))))

    go_path = os.path.join(root, go_shell_rel)
    if not os.path.isfile(go_path):
        return 2, {**rep, "problems": rep["problems"] + ["壳件读不到：%s ⇒ 不给结论" % go_shell_rel]}
    rep["problems"] += check_go_shell(_read(go_path))

    # 只报告格：注册表条数 ⟷ 现读名录件数
    reg_path = os.path.join(root, registry_rel)
    mod = load_registry_module()
    if os.path.isfile(reg_path) and mod is not None:
        try:
            reg = json.loads(_read(reg_path))
            live = mod.scan_roster(root)
            live_n = sum(1 for v in live.values() if len(v) == 1)
            if int(reg.get("count", -1)) != live_n:
                rep["problems"].append("注册表条目数 %s ≠ 现读名录件数 %d" % (reg.get("count"), live_n))
            else:
                rep["notes"].append("注册表条数 == 现读名录件数（%d）" % live_n)
        except Exception as e:  # noqa: BLE001
            rep["notes"].append("注册表读数取不到（只报告 · 不判红）：%s" % e)
    else:
        rep["notes"].append("注册表不在盘 ⇒ 该格不适用（只报告）")

    rc = 1 if rep["problems"] else 0
    return rc, rep


def cmd_self_test():
    ok = True
    good = 'var gateCheckInterpreter = map[string]string{".py": "python3", ".sh": "bash"}\n' \
           'err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error { return nil })\n'
    bad1 = 'var gateCheckInterpreter = map[string]string{".py": "python3", ".sh": "bash", "": "sh"}\n' \
           'filepath.WalkDir\n'
    bad2 = 'var gateCheckInterpreter = map[string]string{".py": "python3"}\n"scripts/gates/foo"\n'
    if check_go_shell(good):
        print("✗ 正控：合规壳件被判红：%s" % check_go_shell(good))
        ok = False
    else:
        print("✓ 正控：合规壳件判绿")
    if not check_go_shell(bad1):
        print("✗ 负控①：解释器闭集被改宽未被抓")
        ok = False
    else:
        print("✓ 负控①：解释器闭集被改宽 ⇒ 现红")
    if not (check_go_shell(bad2)
            and any("名件路径" in x or "目录派生" in x or "闭集" in x for x in check_go_shell(bad2))):
        print("✗ 负控②：写死名件路径未被抓")
        ok = False
    else:
        print("✓ 负控②：写死名件路径 ⇒ 现红")
    if parse_version("GNU bash，版本 3.2.57(1)-release (x86_64)") != (3, 2, 57):
        print("✗ 负控③：bash 版本解析错")
        ok = False
    else:
        print("✓ 负控③：bash 版本解析对")
    if parse_version("no version here") is not None:
        print("✗ 负控④：取不到版本却给了值")
        ok = False
    else:
        print("✓ 负控④：取不到版本 ⇒ None")
    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="`zerg gate check` 转发壳体检件（M1-07 · 平台底线 ＋ 库类不特殊化）")
    ap.add_argument("--root", default=REPO)
    ap.add_argument("--json", action="store_true", help="机器面（只印 JSON）")
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return cmd_self_test()
    rc, rep = run_health(os.path.abspath(args.root))
    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2))
        return rc
    print("gate check 体检件 —— 平台底线 ＋ 库类不特殊化")
    print("  bash   : %s（底线 %s）" % (rep["checks"]["bash"], ".".join(map(str, MIN_TOOL["bash"]))))
    print("  python : %s（底线 %s）" % (rep["checks"]["python"], ".".join(map(str, MIN_TOOL["python"]))))
    for n in rep["notes"]:
        print("  · " + n)
    if rep["problems"]:
        print("✗ 违例 %d 条：" % len(rep["problems"]))
        for p in rep["problems"]:
            print("  · " + p)
    else:
        print("✓ 全绿（rc=0）：平台达底线 · 库类不特殊化成立")
    return rc


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-selfcount.py —— 「同件自算型」数字自证（缺口 GAP-20260927-102）

病象（现读坐实）：
    件头自指数字与现读不符，且其中若干正被当判据依据；旧账只能靠
    `grep -n '<N> 步' <件>` 手搓对账 ⇒ **无机器判据** ⇒ 下一笔照样腐烂。
    缺口要求的落件 = 一条**只治「同件自算型」**（步数 / 扫面件数 / 自检格数）
    的读数件，且**现算必须由该件自己的口产生**。

口径（写死 —— 逐条都是本件的判据）：
    · 唯一接口 = 注释里的**显式标注**：`{{selfcount:<kind> [key=val ...] = <N>}}`
      （kind ∈ steps / files / checks；`= <N>` 可就地写在标注内，或紧跟同一行 `}}` 之后）。
    · **无标注一律不判**（0 命中 ⇒ 本件不参与 rc，照实打印「不判」）。
    · **现算由本件自己的口产生**（✗ 不接受外部 `--expect` 喂数 ⇒ 防「人抄一遍数」）。
    · kind 语义：
        - `steps`  ：数本件的 `add_step [<scope>] ` 行。
                     不带 `scope` / `scope=all|*` ⇒ **分母 = 全集但排除合成自检步
                     `scope=self`**（★ 不排会把 140 算成 178 —— 合成自检步不是真门）；
                     带 `scope=X` ⇒ 只数 `add_step X `。
        - `files`  ：数**本件所在目录**里匹配 `glob=<g>` 的件数（`glob` 缺省 `*`）。
        - `checks` ：数本件的自检断言行（`assert_eq` / `_st_chk(` 开头）。
    · 声明数 `<N>` 就地取：标注内 `n=<N>` 或 `=<N>`，否则同一行 `}}` 之后的第一个整数。
    · **对不上 ⇒ rc=1 并逐条点名**（件:行 · kind · 声明=N · 现读=M）。
    · **缺语料 / 标注不全 / kind 未知 ⇒ rc=2 不给结论**（fail-closed，✗ 不许当绿）。
    · **基底常量一律不判**：本件不猜、不比历史时点读数（历史散文腐烂不归本件管）。
    · **自排除（防「自咬」）**：扫面时**跳过本件自身**（本件 docstring 的语法示例 + `--self-test`
      夹具里的 `{{selfcount:…}}` 示例串都不是真标注 —— 同 `check-cli-contract.py` 口径）；且 kind
      必须是**标识符**（`<kind>` / `…` / `...` 这类语法示例 / 散文提及**不算标注**，如接线注释里
      描述本门的那句）。⇒ 本件扫真目标得「无标注 ⇒ 不判」（rc=0 休眠态）。
    · **只读**：不写任何文件（无副作用 · 可任意复跑）。

用法：
    python3 scripts/gates/check-selfcount.py [--root <目录>] [--json] [--self-test]
    （`--root` 只为自检 / 负控 / 换根用；默认扫 `scripts/gates`。
      **负控**：把某件标注的 `= <N>` 改错 ⇒ 该件当场 rc=1 且被点名。）

★ 接线状态：**已挂门91**（tri-report 只报告档）· 2026-10-04 现读更正 ——
  原「仓外候件 / 未挂进任何门步」措辞已过期（接线早已落）。
"""
import argparse
import glob as globmod
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

# ── 本件自己的「口」（现算来源 · ✗ 不读外部声明）───────────────────────────────
STEP_LINE = re.compile(r"^\s*add_step\s+(\S+)\s")
CHECK_LINE = re.compile(r"^\s*(?:assert_eq|_st_chk)\b")
ANN = re.compile(r"\{\{\s*selfcount\s*:(?P<body>[^}]*)\}\}(?P<after>[^\n]*)")
# ★ 合法 kind 必须是**标识符**；`<kind>` / `…` / `...` 这类语法示例 / 散文提及 ⇒ **不是标注**（不判不计）。
KIND_ID = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")

REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
)
DEFAULT_ROOT = os.path.join(REPO_ROOT, "scripts", "gates")
KINDS = ("steps", "files", "checks")
EXTS = (".sh", ".py")

SELF_PATH = os.path.abspath(__file__)


def _as_int(s):
    s = (s or "").strip()
    return int(s) if re.fullmatch(r"\d+", s) else None


def parse_annotations(path, text):
    """回逐条标注：{file,line,kind,scope,glob,declared}（declared 可能 None）。"""
    out = []
    for i, ln in enumerate(text.splitlines(), 1):
        m = ANN.search(ln)
        if not m:
            continue
        toks = m.group("body").split()
        kind = toks[0] if toks else ""
        if not KIND_ID.match(kind):
            # ★ 非标识符 kind（`<kind>` / `…` / `...`）= **语法示例 / 散文提及**（如 precommit 里
            #   描述本门的那句），不是真标注 ⇒ 直接跳过（既不判、也不记 bad_ann）。
            continue
        kv = {}
        for t in toks[1:]:
            if "=" in t:
                k, v = t.split("=", 1)
                kv[k] = v
        declared = None
        if "n" in kv:
            declared = _as_int(kv["n"])
        elif "" in kv:  # 形如 `=3` ⇒ 拆成 k="" v="3"
            declared = _as_int(kv[""])
        if declared is None:
            mm = re.search(r"\d+", m.group("after"))
            if mm:
                declared = int(mm.group(0))
        out.append({"file": path, "line": i, "kind": kind,
                    "scope": kv.get("scope"), "glob": kv.get("glob"),
                    "declared": declared})
    return out


def live_count(a, cache):
    """由本件自己的口现算 kind 的真数。"""
    kind = a["kind"]
    if kind == "steps":
        text = cache[a["file"]]
        scope = a["scope"]
        n = 0
        for ln in text.splitlines():
            m = STEP_LINE.match(ln)
            if not m:
                continue
            s = m.group(1)
            if scope and scope not in ("all", "*"):
                if s == scope:
                    n += 1
            elif s != "self":  # 分母排除合成自检步
                n += 1
        return n
    if kind == "checks":
        text = cache[a["file"]]
        return sum(1 for ln in text.splitlines() if CHECK_LINE.match(ln))
    if kind == "files":
        d = os.path.dirname(a["file"])
        pat = a["glob"] or "*"
        return len(globmod.glob(os.path.join(d, pat)))
    return None


def scan_files(root):
    files = []
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d not in (".git", "node_modules", "__pycache__")]
        for fn in sorted(fns):
            if fn.endswith(EXTS):
                files.append(os.path.join(dp, fn))
    return sorted(files)


def run(root, as_json=False):
    if not os.path.isdir(root):
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % root, file=sys.stderr)
        return 2

    files = scan_files(root)
    if not files:  # ★ 空集守卫（GAP-20260927-108）：空域 ⇒ fail-closed（✗ 不许当「无标注 ⇒ 不判」绿）
        print("BLOCKED —— 空域：%s 下 0 个 .sh/.py（取不到输入）⇒ rc=2 不给结论" % root, file=sys.stderr)
        return 2
    anns, cache, blocked = [], {}, []
    for p in files:
        if os.path.abspath(p) == SELF_PATH:
            continue  # ★ 自排除：本件自身（docstring 语法示例 + 自检夹具示例串都落在此）——
            #  同 check-cli-contract.py `node_family_hits` 口径（`abspath(full) == me ⇒ skip`）。
        try:
            text = open(p, encoding="utf-8", errors="replace").read()
        except OSError as e:
            blocked.append({"file": p, "err": str(e)})
            continue
        cache[p] = text
        anns.extend(parse_annotations(p, text))

    checked, mism, bad_ann = [], [], []
    for a in anns:
        rel = os.path.relpath(a["file"], root)
        if a["kind"] not in KINDS:
            bad_ann.append((rel, a["line"], "kind 未知：%r（合法 %s）" % (a["kind"], "/".join(KINDS))))
            continue
        if a["declared"] is None:
            bad_ann.append((rel, a["line"], "标注缺声明数 N（须 `= <N>` 或行尾给一个整数）"))
            continue
        live = live_count(a, cache)
        rec = {"file": rel, "line": a["line"], "kind": a["kind"],
               "scope": a["scope"], "glob": a["glob"],
               "declared": a["declared"], "live": live, "ok": live == a["declared"]}
        checked.append(rec)
        if not rec["ok"]:
            mism.append(rec)

    rc = 2 if (blocked or bad_ann) else (1 if mism else 0)

    if as_json:
        print(json.dumps({"schema": "zerg/v1", "kind": "SelfCount", "root": root,
                          "annotations": len(anns), "checked": checked,
                          "mismatch": mism,
                          "blocked": blocked,
                          "bad_annotation": [{"file": f, "line": i, "err": e} for f, i, e in bad_ann]},
                         ensure_ascii=False))
        return rc

    print("扫描根：%s" % root)
    print("标注 %d 条 · 已判 %d 条 · 不符 %d 条 · 标注不全 %d 条"
          % (len(anns), len(checked), len(mism), len(bad_ann)))
    for rec in checked:
        print("  %s %s:%d · kind=%s%s · 声明=%d · 现读=%d"
              % ("✓" if rec["ok"] else "✗", rec["file"], rec["line"], rec["kind"],
                 (" scope=%s" % rec["scope"]) if rec["scope"] else "",
                 rec["declared"], rec["live"]))
    for f, i, e in bad_ann:
        print("  BLOCKED %s:%d · %s" % (f, i, e), file=sys.stderr)
    for b in blocked:
        print("  BLOCKED 读不到：%s（%s）" % (b["file"], b["err"]), file=sys.stderr)

    if rc == 2:
        print("⇒ 缺语料 / 标注不全 ⇒ rc=2（不给结论）")
    elif mism:
        print("⇒ 声明数 ⟷ 现读数**对不上**（逐条点名在上）⇒ rc=1")
    elif not anns:
        print("⇒ 无 `{{selfcount:…}}` 标注 ⇒ **不判**（rc=0）")
    else:
        print("⇒ 标注数逐条与现读相符 ⇒ rc=0")
    return rc


# ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────
#  纪律：合成件全在临时目录（tempfile.mkdtemp + finally 清理）⇒ 不读不写仓内件 ·
#        子进程**真跑本件**取真退出码（不猜）；stderr 一律 print(…, file=sys.stderr)。


def _st_chk(bad, label, want, rc, out):
    good = (rc == want)
    if not good:
        bad[0] += 1
    tail = "" if good else " ｜ " + ((out.strip().splitlines() or [""])[-1][:90])
    print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))
    return good


def self_test():
    """十档：steps/files/checks 三 kind 各成对负控 ＋ 无标注/自检步排除/缺语料。"""
    bad = [0]
    print("── check-selfcount.py --self-test 十档（三 kind 各正控 0 · 负控 1 · 缺语料 2）")
    tmp = tempfile.mkdtemp(prefix="selfcount-selftest-")
    try:
        def put(sub, name, body):
            d = os.path.join(tmp, sub)
            os.makedirs(d, exist_ok=True)
            with open(os.path.join(d, name), "w", encoding="utf-8") as f:
                f.write(body)
            return d

        # ① 正控：scope=gates 声明 = 现读
        d_ok = put("ok", "steps.sh",
                   "# {{selfcount:steps scope=gates}} = 2\n"
                   "add_step gates \"甲\"\nadd_step gates \"乙\"\n")
        # ② 负控：声明 ≠ 现读（同一份件，声明写成 3）
        d_bad = put("bad", "steps.sh",
                    "# {{selfcount:steps scope=gates}} = 3\n"
                    "add_step gates \"甲\"\nadd_step gates \"乙\"\n")
        # ③ 无标注：有 add_step 但无标注 ⇒ 不判
        d_noann = put("noann", "steps.sh",
                      "add_step gates \"甲\"\nadd_step gates \"乙\"\n")
        # ④ 自检步排除：全集声明 = 1（`self` 不计入分母）
        d_excl = put("excl", "steps.sh",
                     "# {{selfcount:steps}} = 1\n"
                     "add_step self \"自检-必过-1\"\nadd_step go \"core: build\"\n")
        # ⑤ 自检步排除负控：若误把 self 计入 ⇒ 现读会变 2，声明 1 ⇒ 红（这里声明 2 期望红）
        d_excl_bad = put("exclbad", "steps.sh",
                         "# {{selfcount:steps}} = 2\n"
                         "add_step self \"自检-必过-1\"\nadd_step go \"core: build\"\n")
        # ⑥ checks kind：正控（声明 2 = 现读 2）
        d_ck_ok = put("ckok", "checks.py",
                      "# {{selfcount:checks}} = 2\n"
                      "def t():\n    assert_eq(\"a\", 1, 1)\n    assert_eq(\"b\", 2, 2)\n")
        # ⑦ checks kind 负控：声明 3 ≠ 现读 2
        d_ck_bad = put("ckbad", "checks.py",
                       "# {{selfcount:checks}} = 3\n"
                       "def t():\n    assert_eq(\"a\", 1, 1)\n    assert_eq(\"b\", 2, 2)\n")
        # ⑧ files kind 正控：本目录 2 个 .py（本件 + 同伴）⇒ 声明 2 = 现读 2
        d_fl_ok = put("flok", "countme.py",
                      "# {{selfcount:files glob=*.py}} = 2\n")
        put("flok", "peer.py", "# 同伴件（供 files kind 计数）\n")
        # ⑨ files kind 负控：声明 5 ≠ 现读 2
        d_fl_bad = put("flbad", "countme.py",
                       "# {{selfcount:files glob=*.py}} = 5\n")
        put("flbad", "peer.py", "# 同伴件（供 files kind 计数）\n")

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH] + list(a),
                               capture_output=True, text=True, cwd=tmp)
            return p.returncode, (p.stdout or "") + (p.stderr or "")

        _st_chk(bad, "档① 正控：scope=gates 声明=现读 ⇒ rc=0", 0, *run("--root", d_ok))
        _st_chk(bad, "档② 负控：声明 3 ≠ 现读 2 ⇒ rc=1（点名）", 1, *run("--root", d_bad))
        _st_chk(bad, "档③ 无标注：一律不判 ⇒ rc=0", 0, *run("--root", d_noann))
        _st_chk(bad, "档④ 自检步排除：`self` 不计入分母（声明 1=现读 1）⇒ rc=0", 0, *run("--root", d_excl))
        _st_chk(bad, "档⑤ 自检步排除负控：声明 2 ≠ 现读 1（self 已排除）⇒ rc=1", 1, *run("--root", d_excl_bad))
        _st_chk(bad, "档⑥ checks 正控：声明 2=现读 2 ⇒ rc=0", 0, *run("--root", d_ck_ok))
        _st_chk(bad, "档⑦ checks 负控：声明 3 ≠ 现读 2 ⇒ rc=1", 1, *run("--root", d_ck_bad))
        _st_chk(bad, "档⑧ files 正控：声明 2=现读 2 ⇒ rc=0", 0, *run("--root", d_fl_ok))
        _st_chk(bad, "档⑨ files 负控：声明 5 ≠ 现读 2 ⇒ rc=1", 1, *run("--root", d_fl_bad))
        _st_chk(bad, "档⑩ 缺语料：`--root` 不存在 ⇒ rc=2（不给结论）", 2,
                *run("--root", os.path.join(tmp, "no-such-root")))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


def main():
    if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]:
        return self_test()
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    return run(a.root, a.json)


if __name__ == "__main__":
    sys.exit(main())

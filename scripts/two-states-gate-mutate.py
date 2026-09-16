#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""判据 7 门禁（`verify-two-states.py --wall`）的**变异验证**（批 2'.5；规程 §2.2：新增用例必须能失败）。

做法：把门禁自己改坏 ⇒ 断言它的自检（`--self-test --wall <茧壁>`）**必须红** ⇒ 还原 ⇒ 断言 sha
逐字节一致 ⇒ 断言复跑绿。任何一步不符合预期即整体 rc=1。

    python3 scripts/two-states-gate-mutate.py            # 三条变异都跑
    python3 scripts/two-states-gate-mutate.py --only G1  # 只跑一条（调试用）

退出码：`0` 全部按预期（红 ⇒ 还原 ⇒ 绿）· `1` 有一步不符合预期 · `2` 硬失败（脚本自身/环境问题）。

**还原写在 `finally` 里**：脚本被 Ctrl-C / 被杀也把门禁放回去 ——
「变异脚本把工作树改坏不还原」是这类脚本最贵的失败方式。

三条各自要红在一个**不同的**判据上（否则就是同一个洞被数了三次）：

| # | 变异 | 该红的判据 |
|---|---|---|
| G1 | 茧壁路线**静默回落**成原生执行（看起来一切正常） | 留痕断言：茧壁自己的痕不在 ⇒ 硬失败 |
| G2 | 路线**起不来就当「全被拦」**（起不来的沙箱看起来最安全） | ③-a 坏配方自检：必须硬失败，不许当「已封闭」 |
| G3 | `claim_egress` 不再判「封闭态仍可达」 | 护栏自检：断言 B 能红（合成输入那一格） |

判据还要求**红在判据上、不是崩在异常上**：变异体必须能编译，且输出里不许出现 Python traceback
（`SyntaxError` / 未捕获异常都算「不是判据红」⇒ 本条变异作废，硬失败）。
"""

import argparse
import hashlib
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TARGET = os.path.join("scripts", "sandbox-probes", "verify-two-states.py")
WALL_BIN = os.path.join(ROOT, "wall", "target", "debug", "zerg-wall")

EXIT_OK, EXIT_UNEXPECTED, EXIT_HARD = 0, 1, 2

# 每条：`anchor` 在目标文件里必须**恰好命中一次**（!= 1 就中止，不留半改的树）
MUTATIONS = [
    {
        "id": "G1",
        "why": "茧壁路线静默回落成原生黄金配方（探针照样输出、断言照样全过）",
        "anchor": '    return [route, "run", "--spec", write_wall_spec(platform, probe, env_extra)]\n',
        "repl": ('    # 变异 G1：忽略 route，静默改用原生黄金配方\n'
                 '    return list(prefix_for(platform, profile if profile is not None else\n'
                 '                           ("pF.sb" if platform == "darwin" else "L2"))) \\\n'
                 '        + [PY, os.path.join(PROBE_ROOT, probe)]\n'),
    },
    {
        "id": "G2",
        "why": "路线起不来时当成「全被拦」（起不来的沙箱看起来最安全）",
        "anchor": ('        raise ProbeError("rc=%d：%s\\n%s\\n命令：%s" % (p.returncode, probe,\n'
                   '                        (p.stderr or "").strip()[:400],\n'
                   '                        " ".join(display_path(a) for a in argv)))\n'),
        "repl": ('        return {"unix_bind": "FAIL:refused", "loopback": "FAIL:refused",\n'
                 '                "egress": "BLOCKED:refused", "target": "refused"}\n'),
    },
    {
        "id": "G3",
        "why": "断言 B 不再判「封闭态出网仍可达」（红的那一版被抹掉）",
        "anchor": "    if classify_reachable(i_v):\n",
        "repl": "    if False and classify_reachable(i_v):\n",
    },
]


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def mutate(path, anchor, repl):
    with open(path, "r", encoding="utf-8") as f:
        text = f.read()
    n = text.count(anchor)
    if n != 1:
        return "锚点命中 %d 次（应为 1）—— 未写盘" % n
    with open(path, "w", encoding="utf-8") as f:
        f.write(text.replace(anchor, repl))
    return ""


def run_gate():
    """跑门禁自检（含茧壁路线）；返回 (rc, 输出)。"""
    if not os.path.isfile(WALL_BIN):
        return EXIT_HARD, "茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`" % WALL_BIN
    p = subprocess.run([sys.executable, os.path.join(ROOT, TARGET), "--self-test", "--wall", WALL_BIN],
                       cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode, p.stdout.decode("utf-8", "replace")


def compiles(path):
    p = subprocess.run([sys.executable, "-c",
                        "import py_compile,sys; py_compile.compile(sys.argv[1], doraise=True)", path],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode == 0, p.stdout.decode("utf-8", "replace")


def main():
    ap = argparse.ArgumentParser(description="判据 7 门禁（--wall 路线）变异验证：改坏门禁，自检必须红")
    ap.add_argument("--only", default="", help="只跑某一条（G1 / G2 / G3）")
    args = ap.parse_args()

    path = os.path.join(ROOT, TARGET)
    if not os.path.exists(path):
        print("!! 找不到目标 %s —— 硬失败" % TARGET)
        return EXIT_HARD
    if not os.path.isfile(WALL_BIN):
        print("!! 茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`（本脚本不替人编）—— 硬失败"
              % os.path.relpath(WALL_BIN, ROOT))
        return EXIT_HARD

    print("=== 判据 7 门禁变异验证（目标 %s · 门禁 = --self-test --wall）===" % TARGET)
    rc, out = run_gate()
    print("基线：门禁 rc=%d（期望 0）" % rc)
    if rc != 0:
        print("!! 基线就不是绿的 ⇒ 硬失败（不许在红基线上做变异）：\n%s" % out[-1500:])
        return EXIT_HARD

    bad, tbl = [], []
    for m in MUTATIONS:
        if args.only and m["id"] != args.only:
            continue
        pre = sha(path)
        tmpdir = tempfile.mkdtemp(prefix="zerg-two-states-mutate-")
        bak = os.path.join(tmpdir, "verify-two-states.py")
        shutil.copy(path, bak)
        try:
            err = mutate(path, m["anchor"], m["repl"])
            if err:
                print("!! [%s] %s" % (m["id"], err))
                return EXIT_HARD
            if sha(path) == pre:
                print("!! [%s] 写盘后 sha 未变 —— 变异没生效" % m["id"])
                return EXIT_HARD
            ok, cerr = compiles(path)
            if not ok:
                print("!! [%s] 变异体编译不过 ⇒ 这条变异作废（红必须是判据红，不是语法错）：\n%s"
                      % (m["id"], cerr))
                return EXIT_HARD
            rc, out = run_gate()
            red_ok = rc != 0
            crashed = "Traceback (most recent call last)" in out
            print("%s [%s] %s ⇒ 门禁 rc=%d（期望非 0）%s"
                  % ("✓" if (red_ok and not crashed) else "✗", m["id"], m["why"], rc,
                     "· 崩溃（traceback）⇒ 不算判据红" if crashed else ""))
            if not red_ok:
                bad.append("%s 变异后门禁仍绿 ⇒ 这面镜子抓不住：%s" % (m["id"], m["why"]))
            elif crashed:
                bad.append("%s 变异后门禁崩了（traceback）⇒ 不是判据红，证据不足" % m["id"])
            else:
                hits = [l.strip() for l in out.splitlines()
                        if l.strip().startswith("✗") or "硬失败" in l or "[✗]" in l]
                for l in hits[:4]:
                    print("     红的原文：%s" % l[:200])
                if not hits:
                    print("     （门禁 rc=%d 但没打出 `✗`/硬失败字样 —— 复核输出尾部）\n%s"
                          % (rc, out[-800:]))
        finally:
            shutil.copy(bak, path)          # 先回拷
            os.utime(path, None)            # 再顶 mtime
            shutil.rmtree(tmpdir, ignore_errors=True)

        post = sha(path)
        restored = post == pre
        print("%s [%s] 还原后 sha %s" % ("✓" if restored else "✗", m["id"],
                                        "逐字节一致" if restored else "不一致！%s ≠ %s" % (post, pre)))
        if not restored:
            bad.append("%s 还原后 sha 不一致：%s ≠ %s（停手人工看）" % (m["id"], post, pre))
            return EXIT_HARD
        rc, out = run_gate()
        print("%s [%s] 复跑：rc=%d（期望 0）" % ("✓" if rc == 0 else "✗", m["id"], rc))
        if rc != 0:
            bad.append("%s 还原后复跑 rc=%d（期望 0）：%s" % (m["id"], rc, out[-800:]))
        tbl.append((m["id"], m["why"], pre))

    print("")
    if bad:
        for b in bad:
            print("!! " + b)
        return EXIT_UNEXPECTED
    for mid, why, pre in tbl:
        print("  %s %s · 变异前 sha=%s…" % (mid, why, pre[:12]))
    print("变异验证全绿：每条都「红 ⇒ 还原 sha 一致 ⇒ 复跑绿」")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

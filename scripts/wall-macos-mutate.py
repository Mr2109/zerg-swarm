#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""macOS 一档（Seatbelt 直调）的**变异验证**（批 2'.4；规程 §2.2：新增用例必须能失败）。

做法：把 `wall/src/platform/macos.rs` 逐条改坏 ⇒ 断言 `cargo test`（茧壁全套，含活体四态）**必须红**
⇒ 还原 ⇒ 断言 sha 逐字节一致 ⇒ 断言复跑绿。任何一步不符合预期即整体 rc=1。

    python3 scripts/wall-macos-mutate.py            # 四条变异都跑
    python3 scripts/wall-macos-mutate.py --only M2  # 只跑一条（调试用）

退出码：`0` 全部按预期（红 ⇒ 还原 ⇒ 绿）· `1` 有一步不符合预期 · `2` 硬失败（构建/脚本自身问题）。

**还原写在 `finally` 里**：脚本被 Ctrl-C / 被杀也会把源码放回去并校验 sha ——
「变异脚本把工作树改坏不还原」是这类脚本最贵的失败方式（迁移桥那轮已经踩过一次）。

四条各自要红在一个**不同的**判据上（否则就是同一个洞被数了四次）：
  M1 bind 加过滤     ⇒ `bind_rule_is_never_filtered` + 活体四态（unix socket 被挡）
  M2 可写面全局放开  ⇒ `write_face_is_limited_to_declared_paths` + 活体写空间外（OPEN）
  M3 吞掉施加失败    ⇒ `macos_bad_profile_is_refused`（坏策略必须报错，不许当成功）
  M4 自报面说谎      ⇒ `allowlist_is_honest_about_the_wide_read_grant`（全局只读不许藏起来）
"""

import argparse
import hashlib
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WALL_DIR = os.path.join(ROOT, "wall")

EXIT_OK, EXIT_UNEXPECTED, EXIT_HARD = 0, 1, 2

# 每条：目标文件里**恰好命中一次**的锚点 ⇒ 替换体（锚点命中次数 != 1 就中止，不留半改的树）
MUTATIONS = [
    {
        "id": "M1",
        "why": "bind 那一档加上过滤（§10.6 实测：会把 unix socket 挡掉）",
        "anchor": '    profile.push_str("(allow network-bind)\\n");\n',
        "repl": '    profile.push_str("(allow network-bind (local ip \\"localhost:*\\"))\\n");\n',
    },
    {
        "id": "M2",
        "why": "可写面放开成全局 `file-write*`（授权级封闭失效）",
        "anchor": '            "(allow file-write* (subpath {}))\\n",\n',
        "repl": '            "(allow file-write*)\\n(allow file-write* (subpath {}))\\n",\n',
    },
    {
        "id": "M3",
        "why": "`sandbox_init` 失败也当成功（吞掉失败 ⇒ 就成「宣称封闭、实际直跑」）",
        "anchor": "    if rc == 0 {\n        return Ok(());\n    }\n",
        "repl": "    if rc == 0 || rc != 0 {\n        return Ok(());\n    }\n",
    },
    {
        "id": "M4",
        "why": "自报面说谎：全局只读不列进放行清单",
        "anchor": '"file-read*:全局（**比基线大得多**：macOS 没有视图级封闭，只读面收不到声明路径）"\n',
        "repl": '"file-read*:声明路径（自报面）"\n',
    },
]

TARGET = os.path.join("wall", "src", "platform", "macos.rs")


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def cargo():
    p = shutil.which("cargo")
    if p:
        return p
    alt = os.path.join(os.path.expanduser("~"), ".cargo", "bin", "cargo")
    return alt if os.path.exists(alt) else ""


def run_tests():
    """跑茧壁全套（含活体四态）；返回 (rc, 失败用例名列表, 尾部输出)。"""
    c = cargo()
    if not c:
        return EXIT_HARD, [], "找不到 cargo（PATH 与 ~/.cargo/bin 都没有）"
    p = subprocess.run([c, "test", "--manifest-path", os.path.join(WALL_DIR, "Cargo.toml")],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    out = p.stdout.decode("utf-8", "replace")
    failed = [l.strip() for l in out.splitlines() if l.strip().endswith("... FAILED")]
    return p.returncode, failed, out[-1200:]


def mutate(path, anchor, repl):
    with open(path, "r", encoding="utf-8") as f:
        text = f.read()
    n = text.count(anchor)
    if n != 1:
        return "锚点命中 %d 次（应为 1）—— 未写盘" % n
    with open(path, "w", encoding="utf-8") as f:
        f.write(text.replace(anchor, repl))
    return ""


def main():
    ap = argparse.ArgumentParser(description="macOS 一档变异验证：改坏实现，茧壁用例必须红")
    ap.add_argument("--only", default="", help="只跑某一条（M1 / M2 / M3 / M4）")
    args = ap.parse_args()

    path = os.path.join(ROOT, TARGET)
    if not os.path.exists(path):
        print("!! 找不到目标 %s —— 硬失败" % path)
        return EXIT_HARD

    print("=== macOS 一档变异验证（目标 %s · 门禁 = cargo test 全套）===" % TARGET)
    rc, failed, log = run_tests()
    print("基线：cargo test rc=%d · 失败用例 %d 个" % (rc, len(failed)))
    if rc != 0:
        print("!! 基线就不是绿的 ⇒ 硬失败（不许在红基线上做变异）：\n%s" % log)
        return EXIT_HARD

    bad = []
    tbl = []
    for m in MUTATIONS:
        if args.only and m["id"] != args.only:
            continue
        pre = sha(path)
        tmpdir = tempfile.mkdtemp(prefix="zerg-macos-mutate-")
        bak = os.path.join(tmpdir, "macos.rs")
        shutil.copy(path, bak)
        try:
            err = mutate(path, m["anchor"], m["repl"])
            if err:
                print("!! [%s] %s" % (m["id"], err))
                return EXIT_HARD
            if sha(path) == pre:
                print("!! [%s] 写盘后 sha 未变 —— 变异没生效" % m["id"])
                return EXIT_HARD
            rc, failed, log = run_tests()
            red_ok = rc != 0
            print("%s [%s] %s ⇒ 门禁 rc=%d（期望非 0）· 红了 %d 个用例：%s" % (
                "✓" if red_ok else "✗", m["id"], m["why"], rc, len(failed),
                "、".join(failed) if failed else "(未列出用例名)"))
            if not red_ok:
                bad.append("%s 变异后门禁仍绿 ⇒ 这面镜子抓不住：%s" % (m["id"], m["why"]))
            elif not failed:
                bad.append("%s 变异后门禁红了但没列出失败用例名 ⇒ 证据不足" % m["id"])
        finally:
            shutil.copy(bak, path)          # 先回拷
            os.utime(path, None)            # 再顶 mtime（cargo 靠 mtime 判新旧）
            shutil.rmtree(tmpdir, ignore_errors=True)

        post = sha(path)
        restored = post == pre
        print("%s [%s] 还原后 sha %s" % ("✓" if restored else "✗", m["id"],
                                        "逐字节一致" if restored else "不一致！%s ≠ %s" % (post, pre)))
        if not restored:
            bad.append("%s 还原后 sha 不一致：%s ≠ %s" % (m["id"], post, pre))
            return EXIT_HARD
        rc, failed, log = run_tests()
        green_ok = rc == 0
        print("%s [%s] 复跑：rc=%d（期望 0）" % ("✓" if green_ok else "✗", m["id"], rc))
        if not green_ok:
            bad.append("%s 还原后复跑 rc=%d（期望 0）：%s" % (m["id"], rc, log))
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

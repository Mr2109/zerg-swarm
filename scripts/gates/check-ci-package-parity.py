#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-ci-package-parity.py —— `ci-public-test-surface.tsv` ↔ `publish/ci/ci.yml` **逐包点名一致性**

设计真源（逐字，不转述）
------------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-CI适配-20260923.md` 波A `A2`（= `设计-CI适配-v1.1.md` §十三 `M1`）：
  「**公开面测试面白/黑名单**：逐包写「为什么在公开树里无意义」」
  判据逐字：「① 名单内包 `go test` rc=0；② **名单外包不在 CI**（ci.yml **逐行核** ✓）」
  —— 「落点 = `publish/ci/ci.yml`（**已签 ✓**）+ 名单件」。

本件补的是**判据②**那半边的机检（判据① 是「在公开面快照上真跑」，那是另一件事 · 不在本件）：
名单件头部自己写死了两条现核命令（`awk` 取 `in-ci` 行 · `sed` 取 `ci.yml` 的包行 · `diff` 两边），
本件把「这两条现核」变成**每次提交都跑**的门 —— 因为**两套真源并存**（名单件的意图 vs `ci.yml` 的清单）
就会漂，而漂的后果是「名单说 no 的包被 CI 点了名」或「名单说 in-ci 的包 CI 根本不跑」两类**静默失配**。

判据（任一不满足 ⇒ rc=1；缺件/空转/自检不过 ⇒ rc=2）
------------------------------------------------------
  A **两向差集皆空**：`tsv` 里 `tier=in-ci` 的 `(module, pkg)` 集合 ↔ `ci.yml` 里 `go test` 续行
    逐包点名出来的 `(working-directory, pkg)` 集合 —— 两侧**都得为空**（只在一侧存在的包逐条点名）。
  B **名单外的包不许出现在 CI 点名表里**（`A2` 判据② 逐字「名单外包不在 CI」）：`tier` 为
    `out-ci` / `no-test` 的 `(module, pkg)` 若出现在 `ci.yml` 的点名表里 ⇒ 红（**逐条点名**）。
    ★ 判据 B 走**整包名全等**比（`internal/gateway` 与 `internal/gateway/adapter` 是两回事：
    前者 `out-ci`、后者 `in-ci`）—— 不许用前缀比（前缀比会把在名单里的子包连坐）。
  C **守卫（有区分度）**：`tsv` 里 `in-ci` 行 **0 条** / `ci.yml` 点名出 **0 个包** ⇒ **rc=2 不给结论**
    （「配置没了所以两边都空、所以全绿」是最难发现的那一类假绿 —— 同仓内其余门脚本的守卫口径）。
  D **闭集**：`tier` 不是 {`in-ci`,`out-ci`,`no-test`} 之一 ⇒ **rc=2**（名单件形态坏 ⇒ 判不了），
    不是静默跳过（静默跳过 = 把坏行当合法行 = 假覆盖）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿（两向差集皆空 · 名单外包一个都没进 CI · 两侧都非空）
  1 = **有差**（两侧差集逐条点名 / 名单外包出现在 CI 里）—— 这一档就是本件要报的红
  2 = **不给结论**：`tsv` 或 `ci.yml` 缺件 / `ci.yml` 的 `go test` 续行解不动 / 名单件有坏行
      （`tier` 闭集外）/ 任一侧解析出 0 条 / `--self-test` 不过 —— **缺件与空转一律不当绿**

用法
----
    python3 scripts/gates/check-ci-package-parity.py                    # 判真目标（仓根）
    python3 scripts/gates/check-ci-package-parity.py --root <树>         # 判别的树（自检夹具同入口）
    python3 scripts/gates/check-ci-package-parity.py --json surface,ci,only_surface,only_ci,out_ci_in_ci
    python3 scripts/gates/check-ci-package-parity.py --self-test         # 成对负控（合成夹具 · 不碰真目标）

★ 本件是**私有面件**（读 `scripts/gates/ci-public-test-surface.tsv` —— 名单件逐条写了私有面理由与账，
  不随公开面发布；`publish/ci/ci.yml` 是发布源件）⇒ 登记在 `scripts/公开标记.tsv` 的 `public=no` 面。
★ **不接管道取 rc**（本仓门禁的硬规矩：管道后 `$?` 是末端 rc）—— 本件自己不做任何 shell 调用。
"""
import argparse
import json
import os
import re
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-ci-package-parity.py v1.0.0"

TSV_REL = os.path.join("scripts", "gates", "ci-public-test-surface.tsv")
CI_REL = os.path.join("publish", "ci", "ci.yml")

TIER_CLOSED = ("in-ci", "out-ci", "no-test")
MODULES = ("core", "agent")

WD_RE = re.compile(r"^\s*working-directory:\s*(\S+)\s*$")
GOTEST_RE = re.compile(r"^\s*go test\s*\\\s*$")
PKG_RE = re.compile(r"^\s*\./([A-Za-z0-9_./\-]+)/\s*\\?\s*$")
END_RE = re.compile(r"^\s*-count=1\s*$")


class SurfaceError(Exception):
    """名单件 / CI 件判不了（缺件 · 坏行 · 解不动）⇒ 调用方给 rc=2（不给结论）。"""


# ── 解析（单一入口：自检夹具与真目标**共用同一套**）────────────────────────────────────

def load_surface(path):
    """名单件 → (rows, problems)。rows = [(module, pkg, tier), …]；`#` 注释与空行跳过。"""
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as exc:
        raise SurfaceError("名单件读不到（%s）：%s" % (path, exc))
    rows, problems = [], []
    for i, raw in enumerate(text.split("\n"), 1):
        line = raw.rstrip("\r")
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        f = line.split("\t")
        if len(f) < 3:
            problems.append("名单件第 %d 行：列数 %d < 3（要 `module<TAB>pkg<TAB>tier…`）" % (i, len(f)))
            continue
        module, pkg, tier = f[0].strip(), f[1].strip(), f[2].strip()
        if tier not in TIER_CLOSED:
            problems.append("名单件第 %d 行：`tier` = %r 不在闭集 %s 内（判不了 ⇒ 不给结论）"
                            % (i, tier, "/".join(TIER_CLOSED)))
            continue
        if module not in MODULES:
            problems.append("名单件第 %d 行：`module` = %r 不在 %s 内" % (i, module, "/".join(MODULES)))
            continue
        rows.append((module, pkg, tier))
    return rows, problems


def parse_ci_packages(text):
    """`ci.yml` → {working-directory: [pkg, …]}（只认 `go test \\` 那种**逐包点名**的续行块）。

    单行 `go test ./a/ ./b/ -run '…'`（如 compat job）**不**是本判据的对象 —— 它不是逐包点名。
    续行块里出现既不是包、也不是注释/空行的行 ⇒ 解不动（抛 SurfaceError，**不许**静默跳过）。
    """
    out, cur_wd, collecting = {}, None, False
    for raw in text.split("\n"):
        line = raw.rstrip("\r")
        if not collecting:
            m = WD_RE.match(line)
            if m:
                cur_wd = m.group(1)
                continue
            if GOTEST_RE.match(line):
                if cur_wd is None:
                    raise SurfaceError("`go test` 逐包块的上面没有 `working-directory:` ⇒ 判不了归属模块")
                collecting = True
                out.setdefault(cur_wd, [])
            continue
        if END_RE.match(line):
            collecting = False
            continue
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        m = PKG_RE.match(line)
        if m:
            out[cur_wd].append(m.group(1))
            continue
        raise SurfaceError("`go test` 续行里出现既不是包也不是注释的行：%r" % line)
    return out


def judge(root):
    """判一棵树 → (rc, out_lines, data)。data 供 `--json` 与自检断言用。"""
    tsv_path = os.path.join(root, TSV_REL)
    ci_path = os.path.join(root, CI_REL)
    out = []
    out.append("── CI 逐包点名一致性：名单件 ↔ `publish/ci/ci.yml`（波A `A2` 判据② 的机检）──")
    out.append("   树：%s" % root)
    try:
        rows, problems = load_surface(tsv_path)
    except SurfaceError as exc:
        out.append("   ⛔ %s ⇒ 不给结论（rc=2）" % exc)
        return 2, out, {"surface": None, "ci": None}
    if problems:
        for p in problems:
            out.append("   ⛔ %s" % p)
        out.append("   ⛔ 名单件形态坏 %d 处 ⇒ 不给结论（rc=2）：坏行**不许**当合法行静默跳过" % len(problems))
        return 2, out, {"surface": None, "ci": None}
    try:
        with open(ci_path, encoding="utf-8") as f:
            ci_text = f.read()
    except OSError as exc:
        out.append("   ⛔ CI 件读不到（%s）：%s ⇒ 不给结论（rc=2）" % (ci_path, exc))
        return 2, out, {"surface": None, "ci": None}
    try:
        ci = parse_ci_packages(ci_text)
    except SurfaceError as exc:
        out.append("   ⛔ %s ⇒ 不给结论（rc=2）" % exc)
        return 2, out, {"surface": None, "ci": None}

    in_ci = sorted({(m, p) for m, p, t in rows if t == "in-ci"})
    out_ci = sorted({(m, p) for m, p, t in rows if t != "in-ci"})
    named = sorted({(wd, p) for wd, ps in ci.items() for p in ps})
    out.append("   名单件（%s）：`in-ci` **%d** 个包 · 名单外（`out-ci`+`no-test`）**%d** 个包"
               % (TSV_REL, len(in_ci), len(out_ci)))
    out.append("   `ci.yml` 逐包点名：**%d** 个包（工作目录 %s）"
               % (len(named), " · ".join("%s×%d" % (k, len(v)) for k, v in sorted(ci.items()))))

    # C 守卫：任一侧 0 条 ⇒ 判据不可判（「配置没了所以全绿」是最难发现的假绿）
    if not in_ci:
        out.append("   ⛔ 守卫 C：名单件里 `in-ci` 行 **0 条** ⇒ 判据不可判 ⇒ 不给结论（rc=2）")
        return 2, out, {"surface": in_ci, "ci": named}
    if not named:
        out.append("   ⛔ 守卫 C：`ci.yml` 点名出 **0 个包** ⇒ 判据不可判 ⇒ 不给结论（rc=2）")
        return 2, out, {"surface": in_ci, "ci": named}

    only_surface = [tuple(x) for x in in_ci if tuple(x) not in set(named)]
    only_ci = [tuple(x) for x in named if tuple(x) not in set(in_ci)]
    bad_out = [tuple(x) for x in out_ci if tuple(x) in set(named)]

    rc = 0
    if only_surface:
        out.append("   ✗ 判据 A：名单件说 `in-ci`、`ci.yml` **没点名**的包 %d 个（名单里的包 CI 根本不跑）："
                   % len(only_surface))
        for m, p in only_surface:
            out.append("       ── %s  ./%s/" % (m, p))
        rc = 1
    if only_ci:
        out.append("   ✗ 判据 A：`ci.yml` 点名、名单件**没有** `in-ci` 的包 %d 个（CI 跑了名单外的包）："
                   % len(only_ci))
        for m, p in only_ci:
            out.append("       ── %s  ./%s/" % (m, p))
        rc = 1
    if bad_out:
        out.append("   ✗ 判据 B：名单**外**（`out-ci`/`no-test`）的包出现在 `ci.yml` 点名表里 %d 个"
                   "（`A2` 判据② 逐字：名单外包不在 CI）：" % len(bad_out))
        for m, p in bad_out:
            out.append("       ── %s  ./%s/" % (m, p))
        rc = 1

    if rc == 0:
        out.append("   ✅ 判据 A：两向差集皆空（`in-ci` %d ↔ 点名 %d，逐包一致）" % (len(in_ci), len(named)))
        out.append("   ✅ 判据 B：名单外 %d 个包一个都没进 CI 点名表" % len(out_ci))
        out.append("⇒ 全绿（rc=0）：名单件与 `ci.yml` 的逐包点名**逐字一致**")
    else:
        out.append("⇒ 有差（rc=1）：修名单件或修 `ci.yml`（**同批改** —— 处置只有这两条，"
                   "不许放宽本判据来凑绿）")
    data = {"surface": [list(x) for x in in_ci], "ci": [list(x) for x in named],
            "only_surface": [list(x) for x in only_surface], "only_ci": [list(x) for x in only_ci],
            "out_ci_in_ci": [list(x) for x in bad_out],
            "out_ci": [list(x) for x in out_ci]}
    return rc, out, data


# ── 自检（成对负控 · 合成夹具 · 先自证「会红」再扫真目标）──────────────────────────────

CI_TEMPLATE = """\
jobs:
  test:
    steps:
      - name: core 构建 + 测试
        working-directory: core
        run: |
          go build ./...
          go test \\
%s            -count=1
      - name: agent 构建 + 测试
        working-directory: agent
        run: |
          go build ./...
          go test \\
%s            -count=1
  compat:
    steps:
      - name: 兼容层单测
        working-directory: core
        run: go test ./internal/compat/ ./internal/api/ -run 'Compat' -count=1
"""


def _pkg_lines(pkgs):
    return "".join("            ./%s/ \\\n" % p for p in pkgs)


def make_fixture(base, surface_text, core_pkgs, agent_pkgs, drop_ci=False):
    os.makedirs(os.path.join(base, "scripts", "gates"), exist_ok=True)
    os.makedirs(os.path.join(base, "publish", "ci"), exist_ok=True)
    with open(os.path.join(base, TSV_REL), "w", encoding="utf-8") as f:
        f.write(surface_text)
    if not drop_ci:
        with open(os.path.join(base, CI_REL), "w", encoding="utf-8") as f:
            f.write(CI_TEMPLATE % (_pkg_lines(core_pkgs), _pkg_lines(agent_pkgs)))
    return base


GOOD_SURFACE = (
    "# 合成夹具（自检用 · 不碰真目标）\n"
    "core\tinternal/alpha\tin-ci\t纯源码判据\t—\n"
    "core\tinternal/beta\tin-ci\t纯源码判据\t—\n"
    "core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n"
    "agent\tinternal/delta\tin-ci\t纯源码判据\t—\n"
)


def self_test():
    checks = []

    def check(cond, desc):
        checks.append((bool(cond), desc))
        print("   %s %s" % ("✓" if cond else "✗", desc))

    t = tempfile.mkdtemp(prefix="ci-parity-selftest-")
    try:
        # ① 正控：两侧逐字一致 ⇒ 绿（含一个名单外包 · 一份单行 go test 不参与判）
        b = make_fixture(os.path.join(t, "ok"), GOOD_SURFACE,
                         ["internal/alpha", "internal/beta"], ["internal/delta"])
        rc, _, _ = judge(b)
        check(rc == 0, "① 正控 PC：名单件 `in-ci` ↔ `ci.yml` 点名**逐字一致** ⇒ rc=0（含 `out-ci` 行与单行 `go test`）")

        # ② 负控 A：名单件多一个包（CI 没点名）⇒ 必红
        b = make_fixture(os.path.join(t, "only-surface"),
                         GOOD_SURFACE + "core\tinternal/epsilon\tin-ci\t纯源码判据\t—\n",
                         ["internal/alpha", "internal/beta"], ["internal/delta"])
        rc, o, d = judge(b)
        check(rc == 1 and d["only_surface"] == [["core", "internal/epsilon"]],
              "② 负控 A：只在一侧（名单件）存在的包 ⇒ rc=1 且**逐条点名**（判据 A 有牙）")

        # ③ 负控 B：CI 多点名一个包（名单件没有）⇒ 必红
        b = make_fixture(os.path.join(t, "only-ci"), GOOD_SURFACE,
                         ["internal/alpha", "internal/beta", "internal/zeta"], ["internal/delta"])
        rc, o, d = judge(b)
        check(rc == 1 and d["only_ci"] == [["core", "internal/zeta"]],
              "③ 负控 B：只在一侧（`ci.yml`）存在的包 ⇒ rc=1 且逐条点名")

        # ④ 负控 C：名单外包出现在 CI 点名表里（`A2` 判据②）⇒ 必红
        b = make_fixture(os.path.join(t, "out-ci-named"), GOOD_SURFACE,
                         ["internal/alpha", "internal/beta", "cmd/gamma"], ["internal/delta"])
        rc, o, d = judge(b)
        check(rc == 1 and d["out_ci_in_ci"] == [["core", "cmd/gamma"]],
              "④ 负控 C：`out-ci` 的包被 CI 点了名 ⇒ rc=1（判据 B · 名单外包不在 CI）")

        # ⑤ 负控 D：模块归属错（TSV 说 agent、`ci.yml` 挂在 core）⇒ 也算差
        b = make_fixture(os.path.join(t, "module-mismatch"),
                         GOOD_SURFACE.replace("agent\tinternal/delta\tin-ci", "core\tinternal/delta\tin-ci"),
                         ["internal/alpha", "internal/beta"], ["internal/delta"])
        rc, o, d = judge(b)
        check(rc == 1, "⑤ 负控 D：同一包名但**模块归属**不同 ⇒ rc=1（比的是 (module, pkg) 二元组，不是裸包名）")

        # ⑥ 空转 VAC：名单件一条 `in-ci` 都没有 ⇒ 不给结论（不许当绿）
        b = make_fixture(os.path.join(t, "vacuous"),
                         "core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n",
                         ["internal/alpha"], [])
        rc, o, d = judge(b)
        check(rc == 2, "⑥ 空转 VAC1：名单件 `in-ci` 行 0 条 ⇒ rc=2（不给结论，不许当绿）")

        # ⑦ 空转 VAC2：`ci.yml` 点名 0 个包 ⇒ 不给结论
        b = make_fixture(os.path.join(t, "vacuous-ci"), GOOD_SURFACE, [], [])
        rc, o, d = judge(b)
        check(rc == 2, "⑦ 空转 VAC2：`ci.yml` 点名出 0 个包 ⇒ rc=2（不给结论，不许当绿）")

        # ⑧ 缺件：`ci.yml` 不在 ⇒ 不给结论
        b = make_fixture(os.path.join(t, "no-ci"), GOOD_SURFACE,
                         ["internal/alpha"], ["internal/delta"], drop_ci=True)
        rc, o, d = judge(b)
        check(rc == 2, "⑧ 缺件：`ci.yml` 不在 ⇒ rc=2（缺件 ≠ 绿）")

        # ⑨ 缺件：名单件不在 ⇒ 不给结论
        os.makedirs(os.path.join(t, "no-tsv", "publish", "ci"), exist_ok=True)
        with open(os.path.join(t, "no-tsv", CI_REL), "w", encoding="utf-8") as f:
            f.write(CI_TEMPLATE % (_pkg_lines(["internal/alpha"]), _pkg_lines(["internal/delta"])))
        rc, o, d = judge(os.path.join(t, "no-tsv"))
        check(rc == 2, "⑨ 缺件：名单件不在 ⇒ rc=2（缺件 ≠ 绿）")

        # ⑩ 闭集：`tier` 打错 ⇒ 不给结论（不是静默跳过）
        b = make_fixture(os.path.join(t, "bad-tier"),
                         GOOD_SURFACE + "core\tinternal/eta\tin_ci\t打错的下划线\t—\n",
                         ["internal/alpha", "internal/beta"], ["internal/delta"])
        rc, o, d = judge(b)
        check(rc == 2, "⑩ 闭集：`tier` 不在 {in-ci,out-ci,no-test} ⇒ rc=2（坏行**不许**静默跳过）")

        # ⑪ 区分度：同一份夹具，把 CI 侧一个包挪到 agent 侧之外 ⇒ 判据真的在比（不是恒绿/恒红）
        b1 = make_fixture(os.path.join(t, "disc-a"), GOOD_SURFACE,
                          ["internal/alpha", "internal/beta"], ["internal/delta"])
        b2 = make_fixture(os.path.join(t, "disc-b"), GOOD_SURFACE,
                          ["internal/alpha", "internal/beta"], ["internal/delta", "internal/theta"])
        r1, _, _ = judge(b1)
        r2, _, _ = judge(b2)
        check(r1 == 0 and r2 == 1, "⑪ 区分度：同夹具下「一致 ⇒ 0」「多一个包 ⇒ 1」（判据有牙）")

        # ⑫ 解析面：单行 `go test … -run`（compat job 那种）**不**进判据点名表
        with open(os.path.join(b1, CI_REL), encoding="utf-8") as f:
            ci = parse_ci_packages(f.read())
        check(ci.get("core") == ["internal/alpha", "internal/beta"],
              "⑫ 解析面：只认 `go test \\\\` 逐包续行块 —— 单行 `go test … -run '…'` 不混进来")
    finally:
        shutil.rmtree(t, ignore_errors=True)

    bad = [d for ok, d in checks if not ok]
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "不过：" + " · ".join(bad), len(checks), len(bad)))
    return 0 if not bad else 2


# ── 缺口 `Q-176`（规则③ · 与 `check-replace-rule-fixtures.py` 的成熟做法同口径）：`--json` 的
#    **字段名打错**不许静默吐 `null`（与「零命中」同形 ⇒ 探针打错一个字就拿到假包封、还不报错）
#    ⇒ **用法错 `rc=2` ＋ 印可选字段表**。**只把「不认的名」这一支从静默改成拒绝**，不动业务判据与退码。
JSON_FIELDS = ["surface", "ci", "only_surface", "only_ci", "out_ci_in_ci", "out_ci"]


def main():
    ap = argparse.ArgumentParser(description="CI 逐包点名一致性（名单件 ↔ publish/ci/ci.yml）")
    ap.add_argument("--root", default=REPO_ROOT, help="判哪棵树（默认仓根）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检直接判真目标（留给调用方显式选择）")
    ap.add_argument("--json", default="", help="机器面字段（逗号分隔 · 闭集：%s；**不认的名 ⇒ 用法错 rc=2**）"
                    % ",".join(JSON_FIELDS))
    args = ap.parse_args()

    if args.json:
        bad = [w.strip() for w in args.json.split(",") if w.strip() and w.strip() not in JSON_FIELDS]
        if bad:
            print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
            print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
            print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
            return 2

    print(VERSION)
    if args.self_test:
        return self_test()
    if not args.no_self_test:
        rc_st = self_test()
        if rc_st != 0:
            print("⛔ 自检不过 ⇒ 拒绝判真目标（门禁自己先能被证明「会红」）")
            return 2
    rc, lines, data = judge(os.path.abspath(args.root))
    for ln in lines:
        print(ln)
    if args.json:
        want = [w.strip() for w in args.json.split(",") if w.strip()]
        print(json.dumps({k: data.get(k) for k in want}, ensure_ascii=False, sort_keys=True))
    return rc


if __name__ == "__main__":
    sys.exit(main())

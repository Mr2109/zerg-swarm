#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-ci-green.py —— 公开 CI 归绿（**分档**：开发机只告警 / 公开制品阻断）

真源（**逐字**，不转述）
----------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:68`（`T10`）逐字：
  「**判据**：① 开发机档：`source_sha`/`dirty` **只告警**（rc 不拦）② 公开制品档：**必须阻断**
    （`source_sha != HEAD` 或 `dirty != false` ⇒ 红）③ 三条缺项照原样登记（① 人面看不见
    ② `warnings[]` 恒空 ③ attestation 未出）」
落点逐字（同件 `:69`）：「`.github/workflows/` + `scripts/gates/` · **依赖**：无
  （`R-25` attestation 按兵不动 ✓ 不引）」
三条缺项的**原样出处**（`调研-缺口-门禁证据-20260923.md:22`）逐字：
  「公开 CI 的形状**已经在了**（`publish/ci/ci.yml` 7 个 job · `release-agent.yml` 逐件 `.sha256` +
    `checksums.txt` + `manifest.json` · 「`uses:` 必须钉全长 commit SHA」），**缺三条**：
    ① 人面看不见（`G-10`）② `warnings[]` 恒空 ③ **attestation 今天没出**（源件逐字
    「将来若要出 attestation，也在本 job 内单独加」）」

本件判四组（**各自成行 · 不合并**）
----------------------------------
  `K1` **分档两态**：开发机档 ⇒ 只告警（rc 不拦）· 公开制品档 ⇒ 必须阻断。
       **单一口径**：现调 `scripts/gates/check-manifest-freshness-profile.py` 的两个 `--profile`
       （本件**不另写**一份新鲜度判据）。判的是「**档位行为**」：
         开发机档 rc 不许是 1（拦了就是错）· 公开制品档在有命中时 rc 必须是 1（不拦就是错）。
       两张样张：**样张 A = 合成清单（J1/J2 双命中）**——分档的机械证明；
                 **样张 B = 真清单**（`--manifest`，或 `dist/*/release/manifest.json` 里最新的一件）——照实读数。
  `K2` **三条缺项照原样登记**：三条**逐字**常量必须原样出现在输出里，并报「**仍缺 3 条**」
       —— 本件**不修**它们（照原样登记 ≠ 已修 ✗）。
  `K3` **`R-25` 按兵不动**：CI 件里 `id-token` / `attestations` 权限 **0 处**（今天「不引」✗）；
       出现即 **红**（引了不该引的）。
  `K4` **附加现读（不计入三档 · 照实单列）**：`uses:` 是否全部钉 40 位 hex · 顶层 `permissions:` 有无 `write`
       —— 这两条是 `R-22` 的现读面（**不是本件的判据**，红了也不改退码，只记账）。
  ★ **不声称**「必需检查集合 = 7 个 job」：`调研-缺口-门禁证据-20260923.md:209` 逐字记「**未核**
    （要 GitHub 侧 settings 才看得到）⇒ 标未验通」⇒ 本件只报**件内统计**的 job 名。

退码（三档）
------------
  0 = 分档行为对（dev 不拦 · public 拦）· 三条缺项照原样登记 · `R-25` 未引
  1 = **有命中**（档位行为错 / 三条缺项被写成已修 / `R-25` 权限出现）
  2 = **不给结论**：CI 件一件都读不到 / 分档真源件（`check-manifest-freshness-profile.py`）不在场 /
      样张跑不出来 / 自检不过

用法
----
    python3 scripts/gates/check-public-ci-green.py                       # 真目标（CI 件 + 分档样张）
    python3 scripts/gates/check-public-ci-green.py --manifest <件>        # 指定样张 B 的清单
    python3 scripts/gates/check-public-ci-green.py --list-rules
    python3 scripts/gates/check-public-ci-green.py --self-test           # 合成夹具 · 含 must-fail 真红
    python3 scripts/gates/check-public-ci-green.py --json rc,gaps,profile

**不接闸**（本批：波③ 只产件、不接线 ✗）：本件与同批的 `.github/workflows/public-green-profile.yml`
都**未**接进 `scripts/gates/precommit-gates.sh`、**未**进 `publish/ci/`、**未** push —— 挂闸/落点/登记
（`scripts/公开标记.tsv` · 两器）按单写入者纪律留给波④收口。
"""

import argparse
import glob
import json
import os
import re
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
PROFILE_GATE = os.path.join(HERE, "check-manifest-freshness-profile.py")

# ── K2：三条缺项（**原样** · 出处逐字 · 只登记不修）──────────────────────────────
GAPS_SOURCE = "Zerg-内部文档/项目文档/v2.5.11/调研-缺口-门禁证据-20260923.md:22"
THREE_GAPS = [
    {"id": "缺①", "text": "① 人面看不见（`G-10`）"},
    {"id": "缺②", "text": "② `warnings[]` 恒空"},
    {"id": "缺③", "text": "③ attestation 今天没出（源件逐字「将来若要出 attestation，也在本 job 内单独加」）"},
]

# ── K3：R-25 按兵不动 —— 今天**不引**的权限（出现即红）────────────────────────────
ATTEST_TOKENS = ["id-token", "attestations"]

# ── CI 件（现读面）：私有两件（发布时镜像成公开树 `.github/workflows/*`）+ 本批新增的 `.github/workflows/` ──
CI_FILES_ALWAYS = ["publish/ci/ci.yml", "publish/ci/release-agent.yml"]
CI_DIR_NEW = ".github/workflows"
USES_RE = re.compile(r"^-?\s*uses:\s*([^\s#]+)")
PINNED_RE = re.compile(r"@[0-9a-f]{40}$")


def read(path):
    try:
        return open(path, encoding="utf-8", errors="replace").read()
    except OSError:
        return None


def ci_files(extra=()):
    """返回 [(rel, text)] —— 私有面两件 + `.github/workflows/*.yml|*.yaml`（新增区）+ `--extra-ci` 指定的件。
    **缺件如实缺**（不假装在场）。"""
    out = [(rel, read(os.path.join(REPO_ROOT, rel))) for rel in CI_FILES_ALWAYS]
    d = os.path.join(REPO_ROOT, CI_DIR_NEW)
    for p in sorted(glob.glob(os.path.join(d, "*.yml")) + glob.glob(os.path.join(d, "*.yaml"))):
        out.append((os.path.relpath(p, REPO_ROOT), read(p)))
    for e in extra:
        p = e if os.path.isabs(e) else os.path.join(REPO_ROOT, e)
        out.append((e, read(p)))  # 绝对路径**原样**显示（不许拼成 ../../.. 那种难认的相对串）
    return out


def scan_ci(files):
    """K3/K4 的机械读数（纯函数 → 自检与真目标同一份）。"""
    r = {"attest_tokens": [], "uses_total": 0, "uses_float": [], "top_write": [], "top_read_declared": [],
         "jobs": {}}
    for rel, text in files:
        if text is None:
            continue
        for ln in text.splitlines():
            s = ln.strip()
            if s.startswith("#"):
                continue  # 抬头注释里也会出现 `uses:` 这个字样 —— 那是**规矩**，不是用法
            low = s.lower()
            for tok in ATTEST_TOKENS:
                # 只在**权限声明**语境里认（注释里提到「将来若要出 attestation」不算引用）
                if (low.startswith(tok + ":") or low.startswith("- " + tok) or ("permissions" in low and tok in low)):
                    r["attest_tokens"].append("%s: %s" % (rel, s[:120]))
            m = USES_RE.match(s)
            if m:
                r["uses_total"] += 1
                if not PINNED_RE.search(m.group(1)):
                    r["uses_float"].append("%s: %s" % (rel, s[:120]))
            if low.startswith("permissions:"):
                r["top_read_declared"].append("%s: %s" % (rel, s[:60]))
            if low.startswith("contents: write") and len(ln) - len(ln.lstrip()) <= 2:
                r["top_write"].append("%s: %s" % (rel, s[:60]))
        for m in re.finditer(r"^\s*name:\s*(.+)$", text, re.M):
            nm = m.group(1).strip()
            if nm.startswith(("ci.yml", "release-agent", "公开")):
                continue
            r["jobs"].setdefault(rel, []).append(nm)
    return r


def run_profile(profile, manifest, extra=()):
    """现调分档真源 → (rc, 末几行输出)。真源不在场 / 跑不起来 ⇒ (None, 说明)。"""
    if not os.path.isfile(PROFILE_GATE):
        return None, "分档真源件不在场：%s" % os.path.relpath(PROFILE_GATE, REPO_ROOT)
    argv = [sys.executable, PROFILE_GATE, "--profile", profile, "--json", "rc,hits,stale"]
    if manifest:
        argv += ["--manifest", manifest]
    argv += list(extra)
    try:
        p = subprocess.run(argv, capture_output=True, text=True, timeout=180,
                           cwd=REPO_ROOT, errors="replace")
    except Exception as e:
        return None, "分档真源跑不起来：%s" % e
    tail = [l for l in (p.stdout or "").strip().splitlines() if l.strip()]
    try:
        obj = json.loads(tail[-1]) if tail else {}
    except Exception:
        obj = {}
    return p.returncode, obj


# ── 自检（合成夹具 · 含 must-fail 真红）──────────────────────────────────────
FIX_CLEAN = """name: 合成干净件
on: {workflow_dispatch: {}}
permissions:
  contents: read
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262
"""
FIX_ATTEST = """name: 合成越线件（引了 attestation）
on: {workflow_dispatch: {}}
permissions:
  contents: read
  id-token: write
  attestations: write
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
"""
FIX_WRITE = """name: 合成顶层提权件
on: {workflow_dispatch: {}}
permissions:
  contents: write
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262
"""


def self_test():
    ok, bad, log = 0, [], []

    def check(name, got, want):
        nonlocal ok
        if got == want:
            ok += 1
            log.append("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            log.append("   ✗ %s ⇒ 期望 %s，实得 %s" % (name, want, got))

    clean = scan_ci([("合成/clean.yml", FIX_CLEAN)])
    check("正控 · 干净件 · attestation 权限 0 处", clean["attest_tokens"], [])
    check("正控 · 干净件 · uses 全钉 40 位 hex", (clean["uses_total"], clean["uses_float"]), (1, []))
    check("正控 · 干净件 · 顶层无 write", clean["top_write"], [])

    att = scan_ci([("合成/attest.yml", FIX_ATTEST)])
    check("负控(must-fail) · 引了 id-token/attestations ⇒ K3 命中（R-25 越线）",
          len(att["attest_tokens"]), 2)
    check("负控(must-fail) · 浮动 tag `@v4` ⇒ K4 附加命中", len(att["uses_float"]), 1)

    wr = scan_ci([("合成/write.yml", FIX_WRITE)])
    check("负控 · 顶层 contents: write ⇒ K4 附加命中", len(wr["top_write"]), 1)

    check("缺件 · 读不到的件不参与（None 不炸）", scan_ci([("合成/缺件.yml", None)])["uses_total"], 0)

    # K2：三条缺项**逐字**且**原样**（不是「已修」）
    keys = ["人面看不见", "`warnings[]` 恒空", "attestation 今天没出"]
    check("K2 · 三条缺项的条数", len(THREE_GAPS), 3)
    check("K2 · 三条逐字键都在", all(any(k in g["text"] for g in THREE_GAPS) for k in keys), True)
    check("K2 · 三条**没有**任何「已修/已引」字样",
          any(w in " ".join(g["text"] for g in THREE_GAPS) for w in ("已修", "已引", "已出")), False)

    # K1：分档两态（现调真源 · 合成清单）
    d = tempfile.mkdtemp(prefix="zerg-ci-green-")
    mf = os.path.join(d, "manifest.json")
    open(mf, "w", encoding="utf-8").write(json.dumps(
        {"commit": "zzzzzzzzzzzz", "source_sha": "0" * 40, "dirty": True,
         "generated_at": "2026-09-23T00:00:00Z"}))
    rc_dev, _ = run_profile("dev", mf, ["--release-head", "1" * 40])
    rc_pub, _ = run_profile("public", mf, ["--release-head", "1" * 40])
    check("K1 负控 · 双命中合成清单 · 开发机档 rc（只告警 ⇒ 不拦）", rc_dev, 0)
    check("K1 负控 · 双命中合成清单 · 公开制品档 rc（必须阻断）", rc_pub, 1)

    # 退码口径自证：dev 档命中不是 1（分档真的分了）
    check("K1 · 两档 rc 不同（同输入 · 分档有牙）", rc_dev != rc_pub, True)

    for ln in log:
        print(ln)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print("公开 CI 归绿 · 分档（T10）· 真源：任务清单-缺口收口-20260923.md:68-69 · 调研-缺口-门禁证据-20260923.md:22")
    print("K1 分档两态：开发机档只告警（rc 不拦）· 公开制品档必须阻断（source_sha != HEAD 或 dirty != false ⇒ 红）")
    print("   —— 单一口径：现调 %s 的两个 --profile" % os.path.relpath(PROFILE_GATE, REPO_ROOT))
    print("K2 三条缺项照原样登记（仍缺 3 条 · 本件不修）：")
    for g in THREE_GAPS:
        print("   %s %s" % (g["id"], g["text"]))
    print("   出处（逐字）：%s" % GAPS_SOURCE)
    print("K3 R-25 按兵不动：CI 件里 %s 权限 **0 处**（出现即红）" % " / ".join(ATTEST_TOKENS))
    print("K4 附加现读（不计入三档）：uses 钉 40 位 hex · 顶层 permissions 无 write")
    print("★ 不声称「必需检查集合 = 7 个 job」（调研:209 逐字记「未核 · 要 GitHub 侧 settings」）")
    print("CI 件面：%s + %s/*.yml|*.yaml" % (" · ".join(CI_FILES_ALWAYS), CI_DIR_NEW))
    print("退码：0 分档行为对+三条照原样+R-25 未引 · 1 有命中 · 2 不给结论（件读不到/自检不过）")
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--manifest", default="", help="样张 B 的真清单（缺省：dist/*/release/manifest.json 最新一件）")
    ap.add_argument("--extra-ci", action="append", default=[],
                    help="额外 CI 件（可重复）—— 用来**真跑负控**（合成 yml），不改仓内 CI 件 ✗")
    ap.add_argument("--json", default="")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        return list_rules()
    if a.self_test:
        print("── 公开 CI 归绿 · 分档（T10）· 自检（合成夹具 · 含 must-fail 真红）──")
        return self_test()

    files = ci_files(a.extra_ci)
    present = [(rel, t) for rel, t in files if t is not None]
    print("── 公开 CI 归绿 · 分档（T10）· 现读 ──")
    print("CI 件面（%d 件在场 / %d 件点名）：" % (len(present), len(files)))
    for rel, t in files:
        print("   %s %s" % ("✓" if t is not None else "✗（缺件）", rel))
    if not present:
        print("⇒ 一件 CI 件都读不到 ⇒ **不给结论** rc=2")
        return 2

    scan = scan_ci(present)
    hits = []

    # ── K1 分档两态 ──────────────────────────────────────────────────────────
    print("")
    print("── K1 分档两态（开发机只告警 / 公开制品阻断）──")
    print("   单一口径：现调 %s 的两个 --profile（本件不另写一份新鲜度判据）"
          % os.path.relpath(PROFILE_GATE, REPO_ROOT))
    dev_rc = dev_prof = pub_rc = pub_prof = None
    # 样张 A：合成清单（J1/J2 双命中）—— 分档的**机械证明**
    d = tempfile.mkdtemp(prefix="zerg-ci-green-")
    synth = os.path.join(d, "manifest.json")
    open(synth, "w", encoding="utf-8").write(json.dumps(
        {"commit": "zzzzzzzzzzzz", "source_sha": "0" * 40, "dirty": True,
         "generated_at": "2026-09-23T00:00:00Z"}))
    a_dev, _ = run_profile("dev", synth, ["--release-head", "1" * 40])
    a_pub, _ = run_profile("public", synth, ["--release-head", "1" * 40])
    print("   样张 A（合成清单 · J1/J2 双命中）：开发机档 rc=%s（期望 0 · 只告警）· 公开制品档 rc=%s（期望 1 · 阻断）"
          % (a_dev, a_pub))
    if a_dev != 0:
        hits.append("K1 开发机档**拦了**（rc=%s · 判据：开发机档只告警、rc 不拦）⇒ 红" % a_dev)
    if a_pub != 1:
        hits.append("K1 公开制品档**没拦**（rc=%s · 判据：`source_sha != HEAD` 或 `dirty != false` ⇒ 红）⇒ 红" % a_pub)

    # 样张 B：真清单（照实读数 · 命中不是错，是状态）
    real = a.manifest
    if not real:
        cands = sorted(glob.glob(os.path.join(REPO_ROOT, "dist", "*", "release", "manifest.json")))
        real = cands[-1] if cands else ""
    if real and os.path.isfile(real):
        dev_rc, dev_prof = run_profile("dev", real)
        pub_rc, pub_prof = run_profile("public", real)
        print("   样张 B（真清单 %s）：" % os.path.relpath(real, REPO_ROOT))
        print("     开发机档 rc=%s ⇒ %s" % (dev_rc, "只告警（不拦）✓" if dev_rc == 0 else "**拦了** ✗"))
        print("     公开制品档 rc=%s ⇒ %s" % (pub_rc, "阻断 ✓" if pub_rc == 1 else "未阻断（清单该条判据成立）"))
        if dev_rc == 1:
            hits.append("K1 真清单在开发机档被拦（rc=1）⇒ 红（判据：只告警）")
        if dev_rc is None:
            print("     （分档真源跑不出来 ⇒ 交给 rc=2 那一路）")
    else:
        print("   样张 B（真清单）：**没有可用的清单件** ⇒ 照实缺席（不当绿、也不当红）")

    # ── K2 三条缺项照原样登记 ────────────────────────────────────────────────
    print("")
    print("── K2 三条缺项照原样登记（**仍缺 3 条** · 本件不修 ✗）──")
    for g in THREE_GAPS:
        print("   %s %s" % (g["id"], g["text"]))
    print("   出处（逐字）：%s" % GAPS_SOURCE)
    print("   ⇒ 仍缺 %d 条（照原样登记 ≠ 已修）" % len(THREE_GAPS))
    if any(w in " ".join(g["text"] for g in THREE_GAPS) for w in ("已修", "已引", "已出")):
        hits.append("K2 三条缺项被写成「已修/已引/已出」⇒ 红（照原样登记，不许美化成已修）")

    # ── K3 R-25 按兵不动 ────────────────────────────────────────────────────
    print("")
    print("── K3 `R-25` attestation 按兵不动（今天**不引**）──")
    print("   权限标记 %s：%d 处" % (" / ".join(ATTEST_TOKENS), len(scan["attest_tokens"])))
    for x in scan["attest_tokens"]:
        print("     ✗ " + x)
    if scan["attest_tokens"]:
        hits.append("K3 CI 件里出现了 %s 权限（%d 处）⇒ 红：今天是「按兵不动 ✓ 不引」"
                    % ("/".join(ATTEST_TOKENS), len(scan["attest_tokens"])))
    else:
        print("   ✓ 0 处（与判据逐字一致：`R-25` attestation 按兵不动 ✓ 不引）")

    # ── K4 附加现读（不计入三档）────────────────────────────────────────────
    print("")
    print("── K4 附加现读（**不计入三档** · 只记账 ✗）──")
    print("   `uses:` 共 %d 行 · 未钉 40 位 hex：%d 行" % (scan["uses_total"], len(scan["uses_float"])))
    for x in scan["uses_float"][:5]:
        print("     · " + x)
    print("   顶层 `contents: write`：%d 处" % len(scan["top_write"]))
    for x in scan["top_write"][:5]:
        print("     · " + x)
    print("   job 名（**件内统计** · 不声称「必需检查集合」）：")
    for rel, names in scan["jobs"].items():
        print("     %s：%d 个 —— %s" % (rel, len(names), " · ".join(names[:8])))

    rc = 1 if hits else 0
    if hits and (dev_rc is None and pub_rc is None and not os.path.isfile(PROFILE_GATE)):
        rc = 2
    print("")
    if hits:
        print("── 命中 ──")
        for h in hits:
            print("   ✗ " + h)
    print("⇒ rc=%d" % rc)

    if a.json:
        want = [f.strip() for f in a.json.split(",") if f.strip()]
        obj = {"rc": rc, "gaps": [g["text"] for g in THREE_GAPS], "gaps_missing": len(THREE_GAPS),
               "profile": {"dev": dev_rc, "public": pub_rc},
               "synth": {"dev": a_dev, "public": a_pub},
               "attest_tokens": scan["attest_tokens"], "uses_total": scan["uses_total"],
               "uses_float": scan["uses_float"], "top_write": scan["top_write"],
               "ci_files_present": [rel for rel, _ in present], "hits": hits}
        print(json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main())

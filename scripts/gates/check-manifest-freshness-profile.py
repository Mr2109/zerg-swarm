#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-manifest-freshness-profile.py —— 清单新鲜度**分档**（R-23）

真源（**逐字**，不转述）
----------------------
`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:72`（`T11`）逐字：
  「`R-23` 开发机只告警 / 公开制品阻断」
`Zerg-内部文档/项目文档/v2.5.11/调研-缺口-门禁证据-20260923.md:136`（`R-23` 行）逐字：
  「【**条件性**】开发机维持「只告警」（照本仓现读 `check-manifest-freshness.py` 的三条 stale 判据）；
    **公开制品升成阻断**」·「公开制品：`manifest.source_sha == ` 发布提交的 HEAD **且** `dirty == false`；
    不等 ⇒ **红**（不是 stale 告警）」
`调研-缺口-门禁证据-20260923.md:167`（§三.5）逐字补了**为什么**：
  「**公开面**（镜像出来给人下载的那一份）必须把「`source_sha` == 发布提交」升成**红**，因为
    「**用 A 打包、声称是 B**」在公开面是**不可逆**的」

口径（三条硬规矩）
------------------
1. **判据只有两条**（`J1` / `J2`，逐条分列、**不合并成一句**）：
     `J1` `manifest.source_sha == 发布提交 HEAD`（`--release-head`，缺省 = 仓 HEAD）
     `J2` `manifest.dirty == false`
2. **分档只决定「拦不拦」，不决定「判不判」**：两档跑的是**同一套**判据（同一份代码、同一份输出形状）；
   开发机档把命中写成**告警**（rc 不拦）· 公开制品档把命中写成**阻断**（rc=1）。
3. **单一口径**：本件**不另写**一份新鲜度判据 —— 三条 stale 判据（`source_sha` / `dirty` / `generated_at`）
   一律**现调** `scripts/gates/check-manifest-freshness.py` 的 `diagnose()`（import 同一件，不是抄一份），
   照实报告、**不改它的语义**（它是明写的取舍：分发通道可能落后于分支 ⇒ 放宽窗口而非关掉检查）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 无阻断命中：**开发机档一律 0**（命中只告警、rc 不拦）；公开制品档 = `J1`/`J2` 两条全成立
  1 = **公开制品档有阻断命中**（`J1` 或 `J2` 任一不成立 —— 逐条点名，不合并）
  2 = **不给结论**：清单件读不到 / JSON 坏 / 取不到发布提交 HEAD ⇒ **缺件与空转一律不当绿**（也不当红）
      —— 另：`--profile` 不是 `dev|public` 之一、`--self-test` 不过，同样 rc=2

用法
----
    python3 scripts/gates/check-manifest-freshness-profile.py --profile dev    --manifest <manifest.json>
    python3 scripts/gates/check-manifest-freshness-profile.py --profile public --manifest <manifest.json> \
            --release-head <发布提交 sha>
    python3 scripts/gates/check-manifest-freshness-profile.py --list-rules
    python3 scripts/gates/check-manifest-freshness-profile.py --self-test     # 正/负成对用例 · 不碰真目标
    python3 scripts/gates/check-manifest-freshness-profile.py --json hits,warnings,profile

**不接闸**（本批：波③ 只产件、不接线 ✗）：本件**未**接进 `scripts/gates/precommit-gates.sh`；
挂闸与登记（`scripts/公开标记.tsv` · 两器）按单写入者纪律留给波④收口。
"""

import argparse
import importlib.util
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))

PROFILES = ("dev", "public")
PROFILE_ZH = {"dev": "开发机", "public": "公开制品"}


# ── 单一口径：现调 check-manifest-freshness.py（同目录同一件）────────────────────
def load_manifest_freshness():
    path = os.path.join(HERE, "check-manifest-freshness.py")
    if not os.path.isfile(path):
        return None
    spec = importlib.util.spec_from_file_location("zerg_check_manifest_freshness", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


MF = load_manifest_freshness()


# ── 两条判据（**逐条一个 id · 逐条一句人话 · 不许合并**）──────────────────────────
def judge(mf, release_head):
    """返回 [(id, 说明, 命中?, 原样读数)] —— 两条判据独立成行，合并成一句即失去一条。"""
    out = []
    src = (mf.get("source_sha") or "").strip() if isinstance(mf, dict) else ""
    dirty = mf.get("dirty") if isinstance(mf, dict) else None

    # J1：manifest.source_sha == 发布提交 HEAD
    if not src:
        out.append(("J1", "source_sha 等于发布提交 HEAD", True,
                    "manifest 里 **没有** source_sha（字段缺失 ⇒ 不成立 ⇒ 宁拒不可猜）"))
    elif not release_head:
        out.append(("J1", "source_sha 等于发布提交 HEAD", None,
                    "取不到发布提交 HEAD ⇒ 这一条**不给结论**"))
    elif src == release_head or release_head.startswith(src) or src.startswith(release_head):
        out.append(("J1", "source_sha 等于发布提交 HEAD", False,
                    "source_sha=%s == 发布提交 HEAD=%s" % (src[:12], release_head[:12])))
    else:
        out.append(("J1", "source_sha 等于发布提交 HEAD", True,
                    "source_sha=%s ≠ 发布提交 HEAD=%s（**用 A 打包、声称是 B**）"
                    % (src[:12], release_head[:12])))

    # J2：dirty == false
    if dirty is None:
        out.append(("J2", "dirty == false", True,
                    "manifest 里 **没有** dirty（字段缺失 ⇒ 不成立 ⇒ 宁拒不可猜）"))
    elif dirty is False:
        out.append(("J2", "dirty == false", False, "dirty=false（出清单时工作树干净）"))
    else:
        out.append(("J2", "dirty == false", True,
                    "dirty=%r（出清单时工作树**有未提交改动** ⇒ 这批资产无法从提交复现）" % (dirty,)))
    return out


def git_head(repo):
    if MF is None or not os.path.isdir(repo):
        return ""
    return MF._git(repo, ["rev-parse", "HEAD"])


def stale_warnings(mf, repo, max_age_hours):
    """三条 stale 判据 —— **现调**真源；真源不在 ⇒ 照实说「取不到」，不自己编一份。"""
    if MF is None:
        return None
    try:
        return MF.diagnose(mf, repo, max_age_hours)
    except Exception as e:  # 真源抛 ⇒ 照实报，不吞
        return ["（现调 check-manifest-freshness.py 的 diagnose() 抛异常：%s）" % e]


def evaluate(mf, repo, release_head, max_age_hours):
    hits = judge(mf, release_head)
    return {"hits": hits, "stale": stale_warnings(mf, repo, max_age_hours)}


def rc_of(profile, ev):
    """分档只决定「拦不拦」：dev ⇒ 命中也是 0（rc 不拦）· public ⇒ 命中即 1。"""
    hard = [h for h in ev["hits"] if h[2] is True]
    unknown = [h for h in ev["hits"] if h[2] is None]
    if profile == "public" and hard:
        return 1
    if unknown:
        return 2
    return 0


# ── 自检（正/负成对 · 合成夹具 · 不碰真目标）────────────────────────────────────
FRESH = {"commit": "abc1234567", "source_sha": "abc1234567890", "dirty": False,
         "generated_at": "2026-09-23T00:00:00Z"}


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

    head = FRESH["source_sha"]

    # ① 干净且新鲜 ⇒ 两档都无命中（rc=0）
    ev = evaluate(FRESH, "", head, 720)
    check("正控 · 干净清单 · J1/J2 全成立", [h[2] for h in ev["hits"]], [False, False])
    check("正控 · 干净清单 · dev 档 rc", rc_of("dev", ev), 0)
    check("正控 · 干净清单 · public 档 rc", rc_of("public", ev), 0)

    # ② 公开制品档：source_sha 不等 ⇒ **必须红**（不是 stale 告警）
    ev = evaluate({**FRESH, "source_sha": "ffffffffffff"}, "", head, 720)
    check("负控 · 公开制品档 · source_sha 不等 ⇒ rc", rc_of("public", ev), 1)
    check("负控 · 公开制品档 · 同一输入 · dev 档 rc（只告警 ⇒ 不拦）", rc_of("dev", ev), 0)
    check("负控 · 两条判据分列（J1 命中 · J2 仍成立）", [h[2] for h in ev["hits"]], [True, False])

    # ③ 公开制品档：dirty=true ⇒ 必须红；dev ⇒ 仍 0
    ev = evaluate({**FRESH, "dirty": True}, "", head, 720)
    check("负控 · 公开制品档 · dirty=true ⇒ rc", rc_of("public", ev), 1)
    check("负控 · 同一输入 · dev 档 rc", rc_of("dev", ev), 0)

    # ④ 两条同时命中 ⇒ 仍是**两条**（不许合并成一句）
    ev = evaluate({**FRESH, "source_sha": "ffffffffffff", "dirty": True}, "", head, 720)
    ids = [h[0] for h in ev["hits"] if h[2] is True]
    check("负控 · 两条同时命中 ⇒ 报出两条 id", ids, ["J1", "J2"])

    # ⑤ 字段缺失 ⇒ 公开制品档红（宁拒不可猜）· dev 档不拦
    ev = evaluate({"commit": "abc1234567"}, "", head, 720)
    check("负控 · 字段缺失 ⇒ 公开制品档 rc", rc_of("public", ev), 1)
    check("负控 · 字段缺失 ⇒ dev 档 rc", rc_of("dev", ev), 0)

    # ⑥ 取不到发布提交 HEAD ⇒ 那一条**不给结论**（rc=2，既不绿也不红）
    ev = evaluate(FRESH, "", "", 720)
    check("负控 · 取不到 HEAD ⇒ 不给结论（rc=2）", rc_of("dev", ev), 2)

    # ⑦ 三条 stale 判据**现调真源**（不是自己抄一份）：真源在 ⇒ 该报就报
    if MF is not None:
        w = stale_warnings({**FRESH, "generated_at": "2026-09-23T00:00:00Z"}, "", 1)
        check("单一口径 · 现调真源 diagnose() 可调用", isinstance(w, list), True)
        check("单一口径 · 真源在场（不是本件自己抄一份判据）", MF is not None, True)
    else:
        bad.append("真源 check-manifest-freshness.py 不在场")
        log.append("   ✗ 真源 check-manifest-freshness.py 不在场 ⇒ 单一口径不成立")

    for ln in log:
        print(ln)
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


def list_rules():
    print("清单新鲜度分档（R-23）· 真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:136/167")
    print("判据（两条 · 逐条分列 · 合并成一句即失去一条）：")
    print("  J1 manifest.source_sha == 发布提交 HEAD（--release-head；缺省 = 仓 HEAD）")
    print("  J2 manifest.dirty == false")
    print("分档（只决定「拦不拦」，不决定「判不判」）：")
    print("  dev    开发机  ⇒ 命中写成**告警**，rc 不拦（恒 0）")
    print("  public 公开制品 ⇒ 命中写成**阻断**，rc=1")
    print("三条 stale 判据：现调 scripts/gates/check-manifest-freshness.py 的 diagnose()（单一口径，不抄一份）")
    print("退码：0 无阻断 · 1 公开制品档有阻断命中 · 2 不给结论（缺件 / JSON 坏 / 取不到 HEAD / 自检不过）")
    return 0


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--profile", default="dev", choices=PROFILES)
    ap.add_argument("--manifest", default="")
    ap.add_argument("--release-head", default="", help="发布提交 sha（公开制品档应显式给；缺省 = 仓 HEAD）")
    ap.add_argument("--repo", default=REPO_ROOT)
    ap.add_argument("--max-age-hours", type=float, default=720.0)
    ap.add_argument("--json", default="", help="机器面字段（逗号分隔：hits,stale,rc,profile）")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    a = ap.parse_args()

    if a.list_rules:
        return list_rules()
    if a.self_test:
        print("── 清单新鲜度分档（R-23）· 自检（正/负成对 · 合成夹具）──")
        return self_test()

    # 不给结论的三类：件读不到 / JSON 坏 / 取不到 HEAD
    if not a.manifest or not os.path.isfile(a.manifest):
        print("缺件：清单件读不到（%s）⇒ **不给结论**（缺件不许静默放绿）" % (a.manifest or "（未给 --manifest）"))
        print("⇒ rc=2")
        return 2
    try:
        mf = json.load(open(a.manifest, encoding="utf-8"))
    except Exception as e:
        print("清单件 JSON 解析失败（%s）⇒ **不给结论**" % e)
        print("⇒ rc=2")
        return 2

    release_head = (a.release_head or "").strip()
    if not release_head:
        release_head = git_head(a.repo)
        head_src = "缺省取值 = 仓 HEAD（%s）" % (release_head[:12] if release_head else "取不到")
    else:
        head_src = "显式给出 --release-head"
    if not release_head:
        print("取不到发布提交 HEAD（--release-head 未给 · 仓 %s 也读不到）⇒ 清单文件：%s" % (a.repo, a.manifest))
        print("⇒ rc=2（**不给结论**：不假装绿，也不假装红）")
        return 2

    ev = evaluate(mf, a.repo, release_head, a.max_age_hours)
    rc = rc_of(a.profile, ev)

    print("── 清单新鲜度分档（R-23）· 档=%s（%s）──" % (a.profile, PROFILE_ZH[a.profile]))
    print("清单文件：%s" % a.manifest)
    print("发布提交 HEAD：%s（%s）" % (release_head[:12], head_src))
    print("判定：%s" % ("**阻断**（rc=1，命中即红）" if a.profile == "public" else "**只告警**（rc 不拦，命中不红）"))
    print("")
    print("── 两条判据（逐条分列 · 不合并）──")
    for jid, name, hit, note in ev["hits"]:
        tag = {True: ("阻断命中" if a.profile == "public" else "告警命中"),
               False: "成立", None: "不给结论"}[hit]
        print("   [%s] %s —— %s：%s" % (jid, tag, name, note))
    if any(h[2] is True for h in ev["hits"]):
        if a.profile == "public":
            print("   ⇒ 公开制品档：命中**不是 stale 告警**，是**红**（公开面「用 A 打包、声称是 B」不可逆）")
        else:
            print("   ⇒ 开发机档：命中**只告警**（分发通道可能落后于分支 ⇒ 放宽窗口而非关掉检查）· rc 不拦")
    print("")
    print("── 三条 stale 判据（现调 check-manifest-freshness.py 的 diagnose() · 单一口径 · 语义一字未改）──")
    if ev["stale"] is None:
        print("   （真源 check-manifest-freshness.py 不在场 ⇒ 照实报「取不到」，不自己编一份）")
    elif not ev["stale"]:
        print("   无（清单新鲜）")
    else:
        for w in ev["stale"]:
            print("   - " + w)
    print("")
    print("⇒ 档=%s · 判据 %d 条（命中 %d）· rc=%d"
          % (a.profile, len(ev["hits"]), len([h for h in ev["hits"] if h[2] is True]), rc))

    if a.json:
        want = [f.strip() for f in a.json.split(",") if f.strip()]
        obj = {
            "profile": a.profile,
            "manifest": a.manifest,
            "release_head": release_head,
            "rc": rc,
            "hits": [{"id": j, "name": n, "hit": h, "note": s} for j, n, h, s in ev["hits"]],
            "stale": ev["stale"],
        }
        print(json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False))
    return rc


if __name__ == "__main__":
    sys.exit(main())

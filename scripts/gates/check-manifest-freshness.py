#!/usr/bin/env python3
"""check-manifest-freshness.py —— 清单新鲜度检查（**只告警，不拒绝**）。

依据（2026-09-16 加固③）：Debian `apt.conf(5)` 对"可能滞后的镜像/分发"给的建议是
**放宽窗口（Min-ValidTime）而不是关掉检查** —— 同口径：资产/清单通道可能落后于分支，
所以这里只产出诊断与告警，**退出码恒为 0**（除非显式 --strict）。

判据（三条，任一命中即标 stale；逐条打印依据，不合并成一句）：
  1. manifest.source_sha 与当前分支头不一致 ⇒ 「清单出自另一棵树」
  2. manifest.dirty == true               ⇒ 「出清单时工作树有未提交改动」
  3. generated_at 距 now 超过 --max-age-hours（默认 720h=30 天）⇒ 「清单过旧」
另注：manifest.commit（声明的构建身份）与 source_sha（生成时的真实 HEAD）**不是同一件事** ——
两者不一致本身就是线索（用 A 打包却声称是 B），单独报一条。

分档口径（2026-09-24 加固 · v2.5.12 波14 序135 · 组4 §二.4 `W-53`（`研-禁:136` `R-23`/`R-24`））
--------------------------------------------------------------------------------------
本件**只管判**（三条 stale 判据 + 上面那条身份线索），**分档只决定「拦不拦」**：
  · **开发机档**（缺省）：命中只写告警，退出码恒 0（`--strict` 才拦）—— 依据仍是 Debian `apt.conf(5)`
    那一条：分发通道可能落后于分支 ⇒ **放宽窗口而非关掉检查**。
  · **公开制品档**：命中 ⇒ **阻断**（rc=1）。本件**不自己**再写一遍那份公开档判据 ——
    由 `scripts/gates/check-manifest-freshness-profile.py --profile public` **现调本件 `diagnose()`**
    判（单一口径：同一份代码、同一份输出形状，只是把「告警」写成「阻断」）。
    发布链上的把关位置 = `publish/ci/release-agent.yml` 的「公开制品档（阻断）」步（生成 manifest 之后、
    `gh release create` 之前）⇒ `source_sha` 不等于发布提交 ⇒ **不发布**（公开面「用 A 打包声称 B」不可逆）。
  · ★ **两条判据各自成句**：判据①（`source_sha` 与当前分支头）与判据②（声明的构建身份与真实来源）
    **必须各占一行**，**合并成一句即红** —— 由 `check-manifest-freshness-two-criteria.py`（门㉕）判
    （它同时核**输出行**与**源码级的两个分列 statement**）。

用法：
  python3 scripts/gates/check-manifest-freshness.py <manifest.json> [--repo <仓>] [--max-age-hours N] [--strict]
  python3 scripts/gates/check-manifest-freshness.py --self-test     # 正/负用例自测；不通过即拒绝服务
"""
import argparse
import datetime
import json
import os
import subprocess
import sys


def _git(repo: str, args: list) -> str:
    try:
        out = subprocess.run(["git", "-c", "core.quotepath=false"] + args, capture_output=True, text=True, cwd=repo)
        return out.stdout.strip() if out.returncode == 0 else ""
    except Exception:
        return ""


def diagnose(mf: dict, repo: str, max_age_hours: float, now=None) -> list:
    """返回告警列表（每条一个字符串）。**不做拒绝判定** —— 拒绝与否由调用方按 --strict 决定。"""
    warns = []
    now = now or datetime.datetime.now(datetime.timezone.utc)

    head = _git(repo, ["rev-parse", "HEAD"]) if repo else ""
    src = (mf.get("source_sha") or "").strip()
    if src and head and src != head:
        warns.append(f"清单出自另一棵树：source_sha={src[:12]} ≠ 当前分支头 {head[:12]}")

    if mf.get("dirty") is True:
        warns.append("清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现")

    commit_claim = (mf.get("commit") or "").strip()
    if commit_claim and src and not src.startswith(commit_claim) and commit_claim != src:
        warns.append(f"声明身份与真实来源不一致：commit={commit_claim[:12]} 而 source_sha={src[:12]}")

    gen = (mf.get("generated_at") or "").strip()
    if gen:
        try:
            t = datetime.datetime.strptime(gen, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)
            age_h = (now - t).total_seconds() / 3600.0
            if age_h > max_age_hours:
                warns.append(f"清单过旧：generated_at={gen} 距今 {age_h:.1f}h > 上限 {max_age_hours:g}h")
        except ValueError:
            warns.append(f"generated_at 无法解析（如实报，不猜）：{gen!r}")
    return warns


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main() -> int:
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("manifest", nargs="?")
    ap.add_argument("--repo", default=os.path.dirname(os.path.abspath(__file__)) + "/../..")
    ap.add_argument("--max-age-hours", type=float, default=720.0)
    ap.add_argument("--strict", action="store_true", help="有告警时退出码 1（默认只告警，恒 0）")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.self_test:
        now = datetime.datetime(2026, 9, 16, tzinfo=datetime.timezone.utc)
        fresh = {"commit": "abc123", "source_sha": "abc1234567890", "dirty": False,
                 "generated_at": "2026-09-15T00:00:00Z"}
        # 负用例①：干净且新鲜 ⇒ 零告警
        assert diagnose(fresh, "", 720, now) == [], "干净清单不应报警"
        # 正用例②：dirty ⇒ 必须报
        assert any("未提交改动" in w for w in diagnose({**fresh, "dirty": True}, "", 720, now)), "dirty 必须报"
        # 正用例③：过旧 ⇒ 必须报
        assert any("过旧" in w for w in diagnose(fresh, "", 1, now)), "过旧必须报"
        # 正用例④：声明身份与真实来源不一致 ⇒ 必须报
        bad = {**fresh, "commit": "zzzzzz", "source_sha": "yyyyyyyyyyyy"}
        assert any("不一致" in w for w in diagnose(bad, "", 720, now)), "身份不一致必须报"
        # 正用例⑤：generated_at 垃圾 ⇒ 如实报解析不了（不猜）
        assert any("无法解析" in w for w in diagnose({**fresh, "generated_at": "not-a-date"}, "", 720, now))
        # 正用例⑥：source_sha 与给定 HEAD 不符 ⇒ 必须报
        repo_head = _git(os.path.dirname(os.path.abspath(__file__)) + "/../..", ["rev-parse", "HEAD"])
        if repo_head:
            assert any("另一棵树" in w for w in diagnose(fresh, os.path.dirname(os.path.abspath(__file__)) + "/../..", 720, now))
        print("self-test 通过（6 条正/负用例）")
        return 0

    if not a.manifest or not os.path.isfile(a.manifest):
        print("用法：check-manifest-freshness.py <manifest.json> [--repo …] [--max-age-hours N] [--strict]")
        return 64
    mf = json.load(open(a.manifest, encoding="utf-8"))
    warns = diagnose(mf, a.repo, a.max_age_hours)
    if not warns:
        print("✓ 清单新鲜（无告警）")
        return 0
    print(f"⚠ 清单新鲜度告警 {len(warns)} 条（**只告警不拒绝**，依据：分发通道可能落后于分支，放宽窗口而非关掉检查）：")
    for w in warns:
        print("   - " + w)
    return 1 if a.strict else 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""make-manifest.py — 生成发布清单 manifest.json（2026-09-11，自动升级模块 P0-b/c）

**本地打包（scripts/build/pack-release.sh）与 CI（.github/workflows/release-agent.yml）共用这一份**，
避免"两个地方各写一份清单生成逻辑、慢慢长歪"——升级器读到的清单必须永远同形。

用法：
    python3 scripts/evals/make-manifest.py <制品目录> <版本> <commit> <build_time>

行为：
    · 扫描目录里的 zerg-<组件>-<os>-<arch>（跳过 .sha256 / manifest.json / checksums.txt）
    · 逐件算 sha256 与体积，写 <目录>/manifest.json
    · 断言：组件×平台矩阵必须与**该档的期望集**完全一致——多一件少一件都**拒绝出清单**
      （宁可让发布失败，也不能发出升级器读不懂的清单）
    · 两档（2026-09-21 修 G1）：
        严格档（默认 · CI/发布）  期望集 = EXPECTED（7 件 · 发布契约）
        本机档（ZERG_MANIFEST_LOCAL=1）期望集 = EXPECTED − CI_ONLY（6 件 · 差集**显式声明**，不是现推）
      详见文件下方 EXPECTED / CI_ONLY 处的口径注释。
"""
import datetime
import subprocess
import hashlib
import json
import os
import sys

# 制品矩阵的权威定义：改这里 = 改发布契约（同时要改 scripts/build/pack-release.sh 与 CI 的 matrix）
#
# ── 两档口径（2026-09-21 修 G1：本机路线两档都拒 ⇒ 发布件不成套）────────────────────
# 严格档（默认 · CI/发布用）：矩阵必须与 EXPECTED **逐件相合**（多一件少一件都拒出清单）。
# 本机档（环境变量 `ZERG_MANIFEST_LOCAL=1` · 由 `scripts/build/pack-release.sh` 自己导出）：
#   期望集**不是另写一份**，而是 `EXPECTED − CI_ONLY` —— 把两档的差异**显式声明成一个集合**。
#   修前用的是「现推」口径（`n for n in EXPECTED if "-darwin-" in n`）：本机路线照样会交叉编出
#   Go 的 linux 两件（pack-release.sh 无条件编），于是它们被现推口径判成「多余」—— 两档都拒 = 死结。
#   本机档**仍然逐件校验**：少一件、或多一件（含本机不该出现的 CI-only 件）一律拒出清单。
# 为什么差集只有一件：`zerg-wall-linux-amd64` 是 **CI-only** —— 茧壁是 Rust，本地不做交叉编
#   （设计稿 L4：Rust 交叉到 linux 要额外链接器/工具链，本版不做）；core/agent 两件是纯 Go 交叉编
#   （CGO_ENABLED=0），本机就出得来 ⇒ 本机档合法矩阵 = darwin 四件 + linux 两件 = 6 件。
import os as _os
LOCAL_MODE = _os.environ.get("ZERG_MANIFEST_LOCAL") == "1"

EXPECTED = {
    "zerg-core-darwin-arm64",
    "zerg-agent-darwin-arm64",
    "zerg-ui-darwin-arm64",
    "zerg-core-linux-amd64",
    "zerg-agent-linux-amd64",
    # 茧壁（Rust）：随卵分发 + 机器上装的那一份（§五之二/§五之三）。本机只出 darwin，
    # linux 件由 CI 的 ubuntu runner 构建（与 core/agent 的纯 Go 交叉编不同，Rust 本地不跨编）。
    "zerg-wall-darwin-arm64",
    "zerg-wall-linux-amd64",
}

# CI-only：本机路线出不了、且**不许出现**在产物里 —— 本机档从 EXPECTED 里减掉它。
# 改 EXPECTED / CI_ONLY 必须同批（脚本会自检 CI_ONLY ⊆ EXPECTED，漂了就拒出清单 rc=2）。
CI_ONLY = {"zerg-wall-linux-amd64"}


def _git(args: list) -> str:
    """在仓内跑一条只读 git 命令；任何失败都返回空串（如实缺席，不编造值）。"""
    try:
        out = subprocess.run(["git"] + args, capture_output=True, text=True, cwd=os.path.dirname(os.path.abspath(__file__)) + "/../..")
        return out.stdout.strip() if out.returncode == 0 else ""
    except Exception:
        return ""


def _cmd(argv: list) -> str:
    """跑一条只读命令取工具链指纹；失败/缺失返回空串（如实缺席，不编造）。"""
    try:
        out = subprocess.run(argv, capture_output=True, text=True)
        return out.stdout.strip() if out.returncode == 0 else ""
    except Exception:
        return ""


def main() -> int:
    if len(sys.argv) != 5:
        print(__doc__)
        return 64
    d, version, commit, build_time = sys.argv[1:5]
    if not os.path.isdir(d):
        print("❌ 不是目录: %s" % d)
        return 1

    found = [n for n in sorted(os.listdir(d))
             if n.startswith("zerg-") and not n.endswith(".sha256") and os.path.isfile(os.path.join(d, n))]

    # 两档口径的**自检**：CI_ONLY 必须是 EXPECTED 的子集（漂了 ⇒ 拒出清单，不猜哪边对）
    drift = sorted(CI_ONLY - set(EXPECTED))
    if drift:
        print("❌ CI_ONLY 里有一件不在 EXPECTED 里：%s ⇒ 两档口径漂了（不给结论）" % " · ".join(drift))
        return 2
    exp = set(EXPECTED)
    if LOCAL_MODE:
        # 本机档 = EXPECTED − CI_ONLY（差集显式声明，见 EXPECTED 处的口径注释）
        exp = exp - CI_ONLY
    missing, extra = sorted(exp - set(found)), sorted(set(found) - exp)
    if missing or extra:
        print("❌ 制品矩阵与契约不符——拒绝生成清单")
        for m in missing:
            print("  缺少: %s" % m)
        for e in extra:
            print("  多余: %s（契约里有吗？改脚本要同步改本文件的 EXPECTED）" % e)
        return 1

    arts = []
    for name in found:
        p = os.path.join(d, name)
        parts = name.split("-")            # zerg-<组件>-<os>-<arch>
        h = hashlib.sha256(open(p, "rb").read()).hexdigest()
        arts.append({
            "name": name,
            "component": parts[0] + "-" + parts[1],
            "os": parts[2],
            "arch": parts[3],
            "size": os.path.getsize(p),
            "sha256": h,
        })

    manifest = {
        "schema": 1,
        "version": version,
        "tag": "v" + version,
        "commit": commit,
        "build_time": build_time,
        "generated_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        # 新鲜度（2026-09-16 加固③，依据 Debian apt.conf(5)：对"可能滞后的分发"是**放宽窗口**而非关掉检查）：
        # source_sha = 产出这批资产时的本地提交；dirty = 当时工作树是否有未提交改动。
        # 消费侧看到 source_sha 与当前分支头不一致、或 generated_at 过旧 ⇒ **只告警不拒绝**（标 stale）。
        "source_sha": _git(["rev-parse", "HEAD"]),
        "source_sha_short": _git(["rev-parse", "--short", "HEAD"]),
        "dirty": bool(_git(["status", "--porcelain"])),
        # 构建自证（2026-09-16 加固①）：本机构建路线下"下件+校官方摘要"不存在 ⇒
        # 唯一判据是"同源同环境 ⇒ 逐字节相同"，故把**当时的环境与工具链**如实记下来。
        # 取不到就如实缺席（空串），不编造。
        "toolchain": {
            "go": _cmd(["go", "version"]),
            "rustc": _cmd(["rustc", "-V"]),
        },
        "host": _cmd(["uname", "-sm"]),
        "artifacts": arts,
    }
    out = os.path.join(d, "manifest.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)
    print("   📝 manifest.json: %d 件（版本 %s · 代码 %s · 档=%s）"
          % (len(arts), version, commit, "本机（EXPECTED−CI_ONLY）" if LOCAL_MODE else "严格（EXPECTED）"))
    return 0


if __name__ == "__main__":
    sys.exit(main())

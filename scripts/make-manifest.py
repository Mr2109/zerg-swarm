#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""make-manifest.py — 生成发布清单 manifest.json（2026-09-11，自动升级模块 P0-b/c）

**本地打包（scripts/pack-release.sh）与 CI（.github/workflows/release-agent.yml）共用这一份**，
避免"两个地方各写一份清单生成逻辑、慢慢长歪"——升级器读到的清单必须永远同形。

用法：
    python3 scripts/make-manifest.py <制品目录> <版本> <commit> <build_time>

行为：
    · 扫描目录里的 zerg-<组件>-<os>-<arch>（跳过 .sha256 / manifest.json / checksums.txt）
    · 逐件算 sha256 与体积，写 <目录>/manifest.json
    · 断言：组件×平台矩阵必须与 EXPECTED 完全一致——多一件少一件都**拒绝出清单**
      （宁可让发布失败，也不能发出升级器读不懂的清单）
"""
import datetime
import hashlib
import json
import os
import sys

# 制品矩阵的权威定义：改这里 = 改发布契约（同时要改 scripts/pack-release.sh 与 CI 的 matrix）
EXPECTED = {
    "zerg-core-darwin-arm64",
    "zerg-agent-darwin-arm64",
    "zerg-ui-darwin-arm64",
    "zerg-core-linux-amd64",
    "zerg-agent-linux-amd64",
}


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

    missing, extra = sorted(EXPECTED - set(found)), sorted(set(found) - EXPECTED)
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
        "artifacts": arts,
    }
    out = os.path.join(d, "manifest.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)
    print("   📝 manifest.json: %d 件（版本 %s · 代码 %s）" % (len(arts), version, commit))
    return 0


if __name__ == "__main__":
    sys.exit(main())

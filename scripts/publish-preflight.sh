#!/usr/bin/env bash
# 发布前置硬闸：对「镜像产物目录」跑五道检查；任一不过 ⇒ 非零退出（不得推送）
# 用法: bash scripts/publish-preflight.sh <产物目录>
set +e
D="${1:?用法: publish-preflight.sh <产物目录>}"
Z="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$PATH:/opt/homebrew/bin"
FAIL=0

echo "── 闸① 占位符截断残留（逐笔）──"
python3 "$Z/scripts/check-placeholder-residue.py" "$D" --all > /tmp/pf1.log 2>&1; RC1=$?
tail -3 /tmp/pf1.log | sed 's/^/   /'
[ "$RC1" != "0" ] && { echo "   ✗ 闸① 不过（rc=${RC1}）"; FAIL=1; }

echo "── 闸② 私有面 ──"
python3 "$Z/scripts/check-public-tree-private.py" "$D" > /tmp/pf2.log 2>&1; RC2=$?
tail -1 /tmp/pf2.log | sed 's/^/   /'
[ "$RC2" != "0" ] && { echo "   ✗ 闸② 不过（rc=${RC2}）"; FAIL=1; }

echo "── 闸③ 旧 token / 绝对路径（逐笔）──"
python3 - "$D" > /tmp/pf3.log 2>&1 <<'PY'
import subprocess, sys
D = sys.argv[1]
commits = subprocess.run(["git","-C",D,"rev-list","main"],capture_output=True,text=True).stdout.split()
bad = []
for c in commits:
    for pat in ["Mr2109","Mr2109","Mr2109","<volume-path>","~"]:
        if subprocess.run(["git","-C",D,"grep","-q","-I",pat,c,"--","."],capture_output=True).returncode == 0:
            bad.append((pat, c[:8]))
print("   提交数=%d 命中=%d（应 0）" % (len(commits), len(bad)))
for b in bad[:5]: print("   ✗", b)
sys.exit(1 if bad else 0)
PY
RC3=$?; tail -2 /tmp/pf3.log | sed 's/^/   /'
[ "$RC3" != "0" ] && { echo "   ✗ 闸③ 不过（rc=${RC3}）"; FAIL=1; }

echo "── 闸④ 危险路径与令牌形态（末树）──"
python3 "$Z/scripts/check-public-tree-hazards.py" "$D" HEAD > /tmp/pf4.log 2>&1; RC4=$?
tail -2 /tmp/pf4.log | sed 's/^/   /'
[ "$RC4" != "0" ] && { echo "   ✗ 闸④ 不过（rc=${RC4}）"; FAIL=1; }

echo "── 闸⑤ 产物可构建（core go build + UI 镜像形态测试）──"
(cd "$D/core" && GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=https://goproxy.cn,direct go build -buildvcs=false ./... > /tmp/pf5a.log 2>&1); RC5a=$?
echo "   core go build rc=$RC5a"
[ "$RC5a" != "0" ] && { head -3 /tmp/pf5a.log | sed 's/^/     /'; FAIL=1; }
(cd "$D/ui" && CARGO_TARGET_DIR=/tmp/pf-target cargo test -p zerg-ui --no-default-features > /tmp/pf5b.log 2>&1); RC5b=$?
echo "   ui 镜像形态 rc=$RC5b | $(grep -oE '[0-9]+ passed' /tmp/pf5b.log | awk '{s+=$1} END {print s}') passed"
[ "$RC5b" != "0" ] && FAIL=1

if [ "$FAIL" != "0" ]; then echo "❌ 发布前置未过 ⇒ 不得推送"; exit 1; fi
echo "✅ 五道闸全过 ⇒ 允许推送"

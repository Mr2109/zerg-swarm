#!/usr/bin/env bash
# 发布前置硬闸：对「镜像产物目录」跑八道检查；任一不过 ⇒ 非零退出（不得推送）
# 用法: bash scripts/publish-preflight.sh <产物目录>
# 退出码：0 全过 ⇒ 允许推送 · 1 有闸不过（真红）· 2 **不给结论**（有闸缺件/判不了 ⇒ 也不许推）
#   ★ 三档口径与提交闸一致（0/1/2）；2026-09-18 ③批次新增 闸②b 与 闸⑥ 时引入 `BLOCKED` 计数，
#     只为「缺件 ≠ 绿」这一件事服务（闸⑥ 的清单不在位时走它）。
set +e
D="${1:?用法: publish-preflight.sh <产物目录>}"
Z="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$PATH:/opt/homebrew/bin"
FAIL=0
BLOCKED=0

echo "── 闸⓪ 发布面同步（配置级·秒级，不依赖产物目录）──"
# 为什么放在最前面：它判的是**私有仓的配置**（白名单 × EXCLUDES × 私有面黑名单 × 镜像器丢弃口径），
# 成本 ~2 秒。2026-09-16 实测过一次漂移（三个发布机制脚本只进了镜像器的自带清单、没进 EXCLUDES ⇒
# 旧器真跑把私有面路径导出、被自己的闸②抓住而整批自中止）。这类问题在**推之前几秒**就能看见，
# 不必等跑到闸⑤（go build + UI 测试，几分钟）才发现。
python3 "$Z/scripts/check-publish-face-sync.py" > /tmp/pf0.log 2>&1; RC0=$?
tail -3 /tmp/pf0.log | sed 's/^/   /'
[ "$RC0" != "0" ] && { echo "   ✗ 闸⓪ 不过（rc=${RC0}）"; FAIL=1; }

echo "── 闸① 占位符截断残留（逐笔）──"
python3 "$Z/scripts/check-placeholder-residue.py" "$D" --all > /tmp/pf1.log 2>&1; RC1=$?
tail -3 /tmp/pf1.log | sed 's/^/   /'
[ "$RC1" != "0" ] && { echo "   ✗ 闸① 不过（rc=${RC1}）"; FAIL=1; }

echo "── 闸② 私有面 ──"
python3 "$Z/scripts/check-public-tree-private.py" "$D" > /tmp/pf2.log 2>&1; RC2=$?
tail -1 /tmp/pf2.log | sed 's/^/   /'
[ "$RC2" != "0" ] && { echo "   ✗ 闸② 不过（rc=${RC2}）"; FAIL=1; }

echo "── 闸②b 写死的私有绝对路径（私有源树 · 内容级）──"
# 为什么加在这里（2026-09-18 ③批次 · 门③ 断言 A 存量清偿第 ⑥只）：
#   `scripts/check-hardcoded-private-paths.py` 写好了、自证过，但**从没被任何闸按名执行** ——
#   它今天只被 scripts/publish-public.sh:130 的 EXCLUDES **清单项** 与 precommit-gates.sh 的**注释**提到
#   （门③ 的判据专门把这两类排除掉，就是它把这只门抓出来的原因）。
# 判据面 = **私有源树 `$Z`**（脚本自己的设计面：core/ agent/ ui/src/ 里不得写死 <volume-path>
#   ~；注释行与测试夹具豁免）—— 与 闸②（判**产物树** $D 的私有面）/ 闸③（判提交里的私有串）
#   互补：那两道看的是「已经进产物的」，这一道看的是「私有源码里写死的」（会经脱敏替换变成占位符 ⇒ 机群失效）。
# 现跑（2026-09-18 ③批次）：私有源树 rc=0 · 产物树 rc=0（两种面都绿，所以它今天不会让闸变红）。
python3 "$Z/scripts/check-hardcoded-private-paths.py" "$Z" > /tmp/pf2b.log 2>&1; RC2b=$?
tail -3 /tmp/pf2b.log | sed 's/^/   /'
[ "$RC2b" != "0" ] && { echo "   ✗ 闸②b 不过（rc=${RC2b}）"; FAIL=1; }

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

echo "── 闸⑥ 清单新鲜度（发布制品 manifest.json · 缺件 ⇒ 不给结论）──"
# 为什么加（2026-09-18 ③批次 · 门③ 断言 A 存量清偿第 ⑤只）：
#   `scripts/check-manifest-freshness.py`（2026-09-16 加固③）从造出来起**没被任何闸调过** ——
#   它判「清单与将要发布的那棵树同源吗 / 出清单时工作树脏不脏 / 清单是不是过旧」，
#   正是发布面最该在推之前看一眼的东西（Debian apt.conf(5) 口径：放宽窗口 ≠ 关掉检查）。
# 目标解析（**现读，不猜**；三处候选按序取第一个存在的）：
#   ① 产物目录自带 `$D/manifest.json`（将来镜像/制品树带清单时优先）
#   ② `scripts/pack-release.sh` 的权威布局 `dist/<版本>/release/manifest.json`
#   ③ 旧布局 `dist/<版本>/manifest.json`；<版本> 取自**正在推送的这棵树**（$D 的 version.go）
# 判据：`--strict`（有告警即 rc=1 ⇒ 本闸不过）——这是**阻断档**，不是只告警：
#   在发布闸里「清单过旧/身份不一致」必须拦住推送；`--repo "$Z"` 指本机私有仓（本机制作的清单出自它）。
# ★ 三档落点：0 ⇒ 过 · 1 ⇒ 不过（FAIL）· 64/异常 ⇒ **缺件 ⇒ BLOCKED（不给结论）**，也**不许当绿**。
VER="$(sed -n 's/^const Version = "\([^"]*\)".*/\1/p' "$D/core/internal/version/version.go" 2>/dev/null | head -1)"
MF=""
for c in "$D/manifest.json" "$Z/dist/$VER/release/manifest.json" "$Z/dist/$VER/manifest.json"; do
  if [ -f "$c" ]; then MF="$c"; break; fi
done
if [ -z "$MF" ]; then
  echo "   ⚠ 缺件：没有可判的 manifest.json（找过 \$D/manifest.json · dist/${VER}/release/manifest.json · dist/${VER}/manifest.json）"
  echo "   ⚠ 「没得判」不是「绿」⇒ 记 BLOCKED（不给结论 · 不计失败项 · 但也不许推送）"
  echo "   ⚠ 处置：先跑 scripts/pack-release.sh 出同版清单，或给本闸传制品目录（见 .md §待拍）"
  BLOCKED=1
else
  echo "   目标（版本 ${VER}）：$MF"
  python3 "$Z/scripts/check-manifest-freshness.py" "$MF" --repo "$Z" --strict > /tmp/pf6.log 2>&1; RC6=$?
  tail -3 /tmp/pf6.log | sed 's/^/   /'
  [ "$RC6" != "0" ] && { echo "   ✗ 闸⑥ 不过（rc=${RC6}）"; FAIL=1; }
fi

if [ "$FAIL" != "0" ]; then echo "❌ 发布前置未过 ⇒ 不得推送"; exit 1; fi
if [ "$BLOCKED" != "0" ]; then
  echo "⚠ 有闸「不给结论」（缺件/判不了 ⇒ BLOCKED）⇒ 也不得推送（rc=2）—— 补齐缺件后再跑"
  exit 2
fi
echo "✅ 八道闸全过 ⇒ 允许推送"

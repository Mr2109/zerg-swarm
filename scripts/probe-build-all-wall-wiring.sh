#!/usr/bin/env bash
# probe-build-all-wall-wiring.sh — 证「茧壁接进 build-all.sh」这条**接线**真的通（批 2'.6）
#
# 为什么需要它：本轮改的是 `scripts/build-all.sh` —— 而**跑真的 build-all.sh 会覆盖 `bin/` 里正在被托管的
# 生产制品**（主控 / 子端 / UI），那是换件，属本作业规程的禁区。于是换一条**等价但不碰生产**的取证路：
# 在 `/private/tmp` 里搭一棵**桩树**（假 `go`、真 `cargo`、真 `wall/` 源码、`--no-ui --no-sign`），
# 让**未改动的 build-all.sh 原文**在那棵树里跑完 ⇒ 这是它真实的执行路径，只是产物落在桩树的 `bin/`。
#
# 三次跑法。**判据 = 真实退出码 + 产物是否存在**；文字只作「哪一支跑的」的补充说明，不单独当判据
# （项目铁律：按真实退出码分支，别看输出字样）：
#   R1 正例       桩树 + cargo 在 PATH            ⇒ rc=0 · 桩树 `bin/zerg-wall` **在**
#   R2 守卫另一支 PATH 里没有 cargo               ⇒ rc=0 · `bin/zerg-wall` **不在** · 留痕里说了「无 cargo」
#                 （证守卫的另一支有出口 —— 不是死路，也不是安静跳过）
#   R3 区分度     桩树里放一份**变异副本**（调用行 → `:` 空操作）⇒ rc=0 但 `bin/zerg-wall` **不在**
#                 ⇒ 证 R1 的产物断言真的会因这一行消失而红（不是「什么都绿」）
#
# 用法：`bash scripts/probe-build-all-wall-wiring.sh`
# 退出码：0 三次都按期望 · 1 有断言红 · 2 硬失败（前置件/环境不对，**不静默跳过**）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_ALL="${REPO_ROOT}/scripts/build-all.sh"
[ -f "${BUILD_ALL}" ] || { echo "❌ 前置件不在: ${BUILD_ALL}" >&2; exit 2; }
[ -d "${REPO_ROOT}/wall/src" ] || { echo "❌ 前置件不在: wall/src" >&2; exit 2; }
command -v cargo >/dev/null 2>&1 || {
  echo "❌ 本机没有 cargo ⇒ R1 只会走「跳过」那条路，探针失去意义（硬失败，不静默降级）" >&2; exit 2; }

TMP="$(mktemp -d /private/tmp/zerg-buildall-wiring.XXXXXX)"
trap 'rm -rf "${TMP}"' EXIT

pass=0
fail=0
assert() { # <名称> <条件 0/1>
  if [ "$2" = "1" ]; then echo "  PASS $1"; pass=$((pass + 1));
  else echo "  ✗ 红 $1"; fail=$((fail + 1)); fi
}
want_rc() { [ "$1" = "$2" ] && echo 1 || echo 0; }
yn() { [ "$1" = "1" ] && echo 1 || echo 0; }

# ── 桩树骨架（**墙源码先建目录再拷**：`cp -R a b c dir/` 的 dir 必须先存在，否则整次拷贝落空）──
mk_tree() { # <目录名> → echo 该目录
  local d="${TMP}/$1"
  mkdir -p "${d}/scripts" "${d}/core/internal/version" "${d}/core/cmd" "${d}/agent" "${d}/bin" "${d}/wall"
  cp "${BUILD_ALL}" "${d}/scripts/build-all.sh"
  cp "${REPO_ROOT}/scripts/build-wall.sh" "${d}/scripts/build-wall.sh"
  cp -R "${REPO_ROOT}/wall/Cargo.toml" "${REPO_ROOT}/wall/Cargo.lock" "${REPO_ROOT}/wall/src" "${d}/wall/"
  [ -f "${d}/wall/Cargo.toml" ] || { echo "!! 桩树墙源码没拷成: ${d}/wall/Cargo.toml" >&2; return 2; }
  printf 'package version\n\nconst Version = "v0.0.0-stub"\n' > "${d}/core/internal/version/version.go"
  echo "${d}"
}

# 假 go：只为让构建链跑通 —— 把 `-o <路径>` 指到的文件造出来即 rc=0（本探针验的是「接线」，不是 Go 编译）
mkdir -p "${TMP}/stubbin"
cat > "${TMP}/stubbin/go" <<'STUB'
#!/usr/bin/env bash
out=""
prev=""
for a in "$@"; do
  if [ "${prev}" = "-o" ]; then out="${a}"; fi
  prev="${a}"
done
[ -n "${out}" ] || exit 0
mkdir -p "$(dirname "${out}")"
printf 'stub\n' > "${out}"
chmod +x "${out}"
exit 0
STUB
chmod +x "${TMP}/stubbin/go"

run_tree() { # <目录> <PATH> → stdout+stderr 合流；rc 由调用方用 $? 读
  local d="$1" path="$2"
  ( cd "${d}" && PATH="${path}" bash "${d}/scripts/build-all.sh" --no-ui --no-sign 2>&1 )
}

echo "=== R1 正例：桩树 + cargo 在 PATH（真跑未改动的 build-all.sh 原文）==="
D1=""
D1="$(mk_tree r1)" || { echo "!! 桩树建不起来 ⇒ 硬失败" >&2; exit 2; }
assert "R1 桩树里的 build-all.sh 与仓内原文逐字节相同（跑的是原文，不是我以为的原文）" \
  "$( [ "$(shasum -a 256 "${D1}/scripts/build-all.sh" | awk '{print $1}')" = "$(shasum -a 256 "${BUILD_ALL}" | awk '{print $1}')" ] && echo 1 || echo 0)"
rc=0
OUT1="$(run_tree "${D1}" "${TMP}/stubbin:${PATH}")" || rc=$?
assert "R1 rc=0（真实退出码 ${rc}）" "$(want_rc "${rc}" 0)"
assert "R1 桩树 bin/zerg-wall 存在且可执行（**主判据**）" "$(yn "$([ -x "${D1}/bin/zerg-wall" ] && echo 1 || echo 0)")"
assert "R1 桩树没有 dist/（制品矩阵未被动过）" "$(yn "$([ ! -e "${D1}/dist" ] && echo 1 || echo 0)")"
echo "  ⓘ 补充（不当判据）：输出里出现「→ 茧壁 zerg-wall」$(echo "${OUT1}" | grep -q '→ 茧壁 zerg-wall' && echo 是 || echo 否)"

echo
echo "=== R2 守卫另一支：PATH 里没有 cargo（不许静默、也不许死路）==="
D2=""
D2="$(mk_tree r2)" || { echo "!! 桩树建不起来 ⇒ 硬失败" >&2; exit 2; }
rc=0
OUT2="$(run_tree "${D2}" "${TMP}/stubbin:/usr/bin:/bin")" || rc=$?
assert "R2 rc=0（真实退出码 ${rc}）" "$(want_rc "${rc}" 0)"
assert "R2 没有产出 bin/zerg-wall（跳过就真的没编 —— **主判据**）" "$(yn "$([ ! -e "${D2}/bin/zerg-wall" ] && echo 1 || echo 0)")"
# 「跳过要留痕」这条性质**本身就是文字的** ⇒ 这里按文字断言（并说明它只判这一条性质）
assert "R2 留痕说了「本机无 cargo」（跳过不许安静）" \
  "$(yn "$(echo "${OUT2}" | grep -q '跳过茧壁 zerg-wall' && echo 1 || echo 0)")"

echo
echo "=== R3 区分度：桩树里放变异副本（调用行 → \`:\` 空操作）⇒ 产物必须不在 ==="
D3=""
D3="$(mk_tree r3)" || { echo "!! 桩树建不起来 ⇒ 硬失败" >&2; exit 2; }
python3 - "${D3}/scripts/build-all.sh" <<'PY' || { echo "!! 变异体生成失败（锚点命中数 != 1）" >&2; exit 2; }
import sys
p = sys.argv[1]
s = open(p, encoding="utf-8").read()
anchor = '  bash "$REPO_ROOT/scripts/build-wall.sh"\n'
if s.count(anchor) != 1:
    sys.exit(2)
open(p, "w", encoding="utf-8").write(s.replace(anchor, "  : # 变异：调用行被拿掉\n", 1))
PY
rc=0
OUT3="$(run_tree "${D3}" "${TMP}/stubbin:${PATH}")" || rc=$?
assert "R3 rc=0（变异体本身照样跑完 —— 所以**产物**才是唯一判据）" "$(want_rc "${rc}" 0)"
assert "R3 没有产出 bin/zerg-wall ⇒ R1 的产物断言有区分度" "$(yn "$([ ! -e "${D3}/bin/zerg-wall" ] && echo 1 || echo 0)")"
echo "  ⓘ 说明：R3 输出里**照样**有「→ 茧壁 zerg-wall」（那是调用行**之前**的 echo）——"
echo "     这条正说明「看输出字样判活」会把人骗过去：所以判据是产物与退出码。"

echo
echo "=== 汇总：PASS ${pass} / 红 ${fail} ==="
[ "${fail}" -eq 0 ] || { echo "!! 有断言红 —— 接线或守卫有问题（桩树已删）" >&2; exit 1; }
echo "✅ build-all.sh 里的茧壁接线与守卫两支都按期望（生产 bin/ 一字未动）"

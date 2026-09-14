#!/usr/bin/env bash
# test-publish-parity.sh — B8「两器产出一致性」可重复验收
#   （设计稿《源码式发布与升级》§十二 B8：镜像器可完全顶替压平发布器 —— 同一次发布两者产出**树内容一致**）
#
# 全部在 /tmp 里跑：对**同一份私有状态**（私有仓 HEAD）分别跑
#   旧器  scripts/publish-public.sh   → 压平单提交
#   镜像器 publish/mirror-public.sh   → 逐提交镜像
# 然后逐路径比 blob sha（内容寻址：sha 相同即逐字节相同）+ 文件模式 + 路径集合。
#
# 断言（每条都能失败）：
#   1 两器都跑绿（旧器现在带弃用警告但**不拒绝执行**，Q5 是「一个版本后删」）
#   2 树内容一致：路径集合相等 + 每个路径的 blob sha 相同 + 模式相同
#   3 差异集合 ⊆ publish/parity-contract.tsv（显式契约，逐条给理由）；契约外差异 ⇒ 红
#   4 过滤口径对账：两器读同一份白名单/EXCLUDES，未入公开面的路径双方对称
#   5 私有面门禁对**两份产出树**都 rc=0
#   6 私有称谓不泄露：镜像**全史**里 0 处私有拍板人称谓（B8 修掉的那个真泄露的回归门）
#   7 反例（造差异必失败）：多一个路径 ⇒ 红；改一个 blob 内容 ⇒ 红
#   8 反例（契约承重）：空契约下真差异必须被判「契约外」⇒ 红
#   9 反例（契约不许当许可证）：把契约写成 `*` ⇒ 比较器**拒绝**
#  10 私有仓只读：跑前跑后 HEAD 与 porcelain 行数不变
#  11 绝不推送：产出仓没有 remote、日志里没有 push
#
# 用法：  bash scripts/test-publish-parity.sh              # 完整跑（约 6 分钟，跑两器全史）
#         SKIP_TOOLS=1 bash scripts/test-publish-parity.sh # 复用上一次两器产出（只跑比对与反例）
#
# 注意：跑的时候**不要同时改私有仓**（镜像器会比对跑前跑后的 `git status --porcelain` 行数，
#       多一个未跟踪文件就会如实报「私有仓被改动了」并 exit 1 —— 这是它的保险，不是缺陷）。

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OLD_PUB="${REPO_ROOT}/scripts/publish-public.sh"
MIRROR="${REPO_ROOT}/publish/mirror-public.sh"
COMPARE="${REPO_ROOT}/publish/parity-compare.py"
CONTRACT="${REPO_ROOT}/publish/parity-contract.tsv"
PRIV_CHECK="${REPO_ROOT}/scripts/check-public-tree-private.py"
ALLOW="${REPO_ROOT}/publish/mirror-allow.txt"
BASE="${PARITY_BASE:-/tmp/zerg-parity-$(date +%Y%m%d-%H%M%S)}"
OLD="${BASE}/old"
MIR="${BASE}/mirror"
mkdir -p "${BASE}"
PASS=0; FAIL=0
ok()   { echo "  ✅ PASS  $*"; PASS=$((PASS+1)); }
bad()  { echo "  ❌ FAIL  $*"; FAIL=$((FAIL+1)); }
head1(){ printf '\n\033[1m== %s\033[0m\n' "$*"; }

echo "验收工作区: ${BASE}"
echo "私有仓:     ${REPO_ROOT}（只读）"
PRIV_HEAD0="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
PRIV_DIRTY0="$(git -C "${REPO_ROOT}" status --porcelain | wc -l | tr -d ' ')"
echo "私有仓 HEAD: ${PRIV_HEAD0:0:12} · porcelain: ${PRIV_DIRTY0}"

for f in "${OLD_PUB}" "${MIRROR}" "${COMPARE}" "${CONTRACT}"; do
  [ -f "$f" ] || { echo "✗ 缺文件：$f" >&2; exit 2; }
done

# ── 1. 跑两器（同一份私有状态）────────────────────────────────────────────────────────────
head1 "1 跑两器（旧=压平快照 / 新=逐提交镜像；都在 ${BASE} 下）"
if [ "${SKIP_TOOLS:-0}" = "1" ]; then
  echo "  … SKIP_TOOLS=1：复用既有产出"
else
  rm -rf "${OLD}" "${MIR}"
  bash "${OLD_PUB}" --out "${OLD}" > "${BASE}/old.log" 2>&1
  OLD_RC=$?
  bash "${MIRROR}" --out "${MIR}" --allow "${ALLOW}" > "${BASE}/mirror.log" 2>&1
  MIR_RC=$?
  echo "  旧器退出码 ${OLD_RC} · 镜像器退出码 ${MIR_RC}"
fi
[ "${OLD_RC:-0}" = "0" ] && ok "1a 旧压平发布器跑绿（含私有面/文档/i18n/版本/兼容清单五道门禁）" \
  || { bad "1a 旧器退出码 ${OLD_RC}（见 ${BASE}/old.log）"; tail -12 "${BASE}/old.log" | sed 's/^/      /'; }
[ "${MIR_RC:-0}" = "0" ] && ok "1b 逐提交镜像器跑绿（逐笔硬门禁 + 私有面门禁）" \
  || { bad "1b 镜像器退出码 ${MIR_RC}（见 ${BASE}/mirror.log）"; tail -12 "${BASE}/mirror.log" | sed 's/^/      /'; }

# 1c 弃用横幅（B8-A）：头部横幅 + 运行时警告都必须在，且**不拒绝执行**
if [ "${SKIP_TOOLS:-0}" = "1" ]; then
  echo "  （SKIP_TOOLS=1：跳过弃用警告检查）"
else
  grep -q '已弃用（DEPRECATED）' "${OLD_PUB}" && ok "1c 旧器头部有显著弃用横幅" || bad "1c 旧器头部缺弃用横幅"
  if grep -q '⚠️  已弃用：scripts/publish-public.sh' "${BASE}/old.log"; then
    ok "1c 运行时先打醒目警告再继续（日志首段可见，且仍然跑完 ⇒ 不拒绝执行）"
  else
    bad "1c 运行时没有弃用警告（Q5 要求『标弃用 + 一个版本后删』，须先警告再继续）"
  fi
  grep -qE '计划在\*\*下一个版本' "${OLD_PUB}" && ok "1c 横幅里写明删除时限（下一个版本 v2.6.0）" || bad "1c 横幅没写删除时限"
fi

# ── 2/3. 树内容一致性 + 契约对账 ─────────────────────────────────────────────────────────
head1 "2 树内容一致性（路径集合 + 每个路径的 blob sha + 模式）＋ 3 已知差异走契约"
python3 "${COMPARE}" --old "${OLD}" --new "${MIR}" --contract "${CONTRACT}" --repo "${REPO_ROOT}" \
  --json "${BASE}/parity.json" > "${BASE}/parity.log" 2>&1
PARITY_RC=$?
sed 's/^/  /' "${BASE}/parity.log"
echo "  比较器退出码: ${PARITY_RC}"
[ "${PARITY_RC}" = "0" ] && ok "2/3 两器树内容一致，差异集合 ⊆ 契约清单（契约外 0 处）" \
  || bad "2/3 存在契约外差异（退出码 ${PARITY_RC}）—— 两器产出不一致"

# 2b 契约里每条差异都必须被**实际命中并带理由**（避免契约里躺着没人读的例外 = 空许可）
if grep -q '契约内已知差异' "${BASE}/parity.log" && grep -q '\[\S*\] \.mirror-state' "${BASE}/parity.log" \
   && grep -q '理由：' "${BASE}/parity.log"; then
  ok "2b 契约条目被实际命中并逐条打印了理由（不是躺在文件里的空许可）"
else
  bad "2b 契约条目没被命中或没打印理由（契约是否与实际差异脱节？）"
fi

# ── 4. 过滤口径对账 ──────────────────────────────────────────────────────────────────────
head1 "4 过滤口径对账（白名单/排除项同源 + 未入公开面的路径双方对称）"
if grep -q '不对称 0' "${BASE}/parity.log"; then
  grep -E '白名单条目|排除项|未入公开面' "${BASE}/parity.log" | sed 's/^/  /'
  ok "4 两器未入公开面的路径**对称**（0 处不对称）—— 排除口径一致"
else
  bad "4 过滤口径不对称（见 parity.log）"
fi
# 4b 排除项确实来自旧器脚本（单一真源）
python3 - "${REPO_ROOT}" <<'PY'
import re, sys
p = sys.argv[1] + "/scripts/publish-public.sh"
t = open(p, encoding="utf-8").read()
m = re.search(r"EXCLUDES=\((.*?)\n\)", t, re.S)
ex = [l.split("#")[0].strip().strip('"').strip("'") for l in m.group(1).split("\n")
      if l.strip() and not l.strip().startswith("#")]
ex = [e for e in ex if e]
need = ["scripts/check-history-secrets.py", "scripts/check-hardcoded-private-paths.py",
        "scripts/publish-public.sh", "scripts/mirror-public-lib.py"]
missing = [n for n in need if n not in ex]
print("  排除项 %d 条；发布机制脚本登记齐全: %s" % (len(ex), "是" if not missing else "缺 %s" % missing))
sys.exit(1 if missing else 0)
PY
[ $? -eq 0 ] && ok "4b 发布机制自身已在 EXCLUDES 里统一登记（镜像器解析同一份 ⇒ 不会两处漂移）" \
  || bad "4b EXCLUDES 里缺发布机制脚本"

# ── 5. 私有面门禁（对两份产出树）──────────────────────────────────────────────────────────
head1 "5 私有面门禁（对旧器产出树 + 镜像产出树各跑一次既有检查器）"
for t in "${OLD}" "${MIR}"; do
  if python3 "${PRIV_CHECK}" "$t" > "${BASE}/priv-$(basename "$t").log" 2>&1; then
    ok "5 $(basename "$t") 私有面门禁 rc=0"
  else
    bad "5 $(basename "$t") 命中私有面"; sed 's/^/      /' "${BASE}/priv-$(basename "$t").log"
  fi
done

# ── 6. 私有称谓不泄露（全史回归门：B8 修掉的正是这条）────────────────────────────────────
head1 "6 私有称谓不泄露：镜像全史 0 处「私有拍板人称谓 / 助手署名」"
LEAK_HITS="$(git -C "${MIR}" grep -l -e 'Mr2109' -e 'Mr2109' -e 'Mr2109' \
             $(git -C "${MIR}" rev-list refs/heads/main) 2>/dev/null | wc -l | tr -d ' ')"
LEAK_COMMITS="$(git -C "${MIR}" rev-list refs/heads/main | wc -l | tr -d ' ')"
if [ "${LEAK_HITS}" = "0" ]; then
  ok "6 镜像 ${LEAK_COMMITS} 笔全史里 0 处私有称谓（替换规则不再空转）"
else
  bad "6 镜像全史里还有 ${LEAK_HITS} 处私有称谓命中（替换规则又空转了？）"
fi
# 6b 对照：旧**规则表**不许再出现「模式 = 替换」的自伤空操作
python3 - "${REPO_ROOT}" <<'PY'
import sys
p = sys.argv[1] + "/publish/replace-rules.tsv"
noop = []
for ln in open(p, encoding="utf-8"):
    ln = ln.rstrip("\n")
    if not ln.strip() or ln.lstrip().startswith("#"):
        continue
    f = ln.split("\t")
    if len(f) >= 2 and f[0] and f[0] == f[1]:
        noop.append(f[0])
print("  规则表自伤空操作（pattern == replacement）: %s" % (noop or "无"))
sys.exit(1 if noop else 0)
PY
[ $? -eq 0 ] && ok "6b 替换规则表无自伤空操作（pattern ≠ replacement）" \
  || bad "6b 规则表仍有自伤空操作"

# ── 7. 反例：造差异必失败 ────────────────────────────────────────────────────────────────
head1 "7 反例（造差异必失败）：多一个路径 ⇒ 红；改一个 blob ⇒ 红"
# 7a 多一个路径
rm -rf "${BASE}/neg-extra"
cp -R "${OLD}" "${BASE}/neg-extra"
echo 'injected' > "${BASE}/neg-extra/INJECTED-EXTRA.txt"
( cd "${BASE}/neg-extra" && \
  GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e \
  git add INJECTED-EXTRA.txt >/dev/null 2>&1 && \
  GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e \
  git commit -q -m "neg: inject extra path" >/dev/null 2>&1 )
python3 "${COMPARE}" --old "${BASE}/neg-extra" --new "${MIR}" --contract "${CONTRACT}" \
  > "${BASE}/neg-extra.log" 2>&1
RC=$?
grep -E '只有旧器有|契约外差异' "${BASE}/neg-extra.log" | head -3 | sed 's/^/      /'
[ "${RC}" != "0" ] && grep -q 'INJECTED-EXTRA.txt' "${BASE}/neg-extra.log" \
  && ok "7a 多一个路径 ⇒ 比较器报红（rc=${RC}）且点名 INJECTED-EXTRA.txt" \
  || bad "7a 造差异没被抓到（rc=${RC}）"
# 7b 改一个 blob 内容
rm -rf "${BASE}/neg-content"
cp -R "${OLD}" "${BASE}/neg-content"
printf '\n// injected by parity negative test\n' >> "${BASE}/neg-content/tools/ls.md"
( cd "${BASE}/neg-content" && \
  GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e \
  git add tools/ls.md >/dev/null 2>&1 && \
  GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e \
  git commit -q -m "neg: change one blob" >/dev/null 2>&1 )
python3 "${COMPARE}" --old "${BASE}/neg-content" --new "${MIR}" --contract "${CONTRACT}" \
  > "${BASE}/neg-content.log" 2>&1
RC=$?
grep -E '内容不同|契约外差异' "${BASE}/neg-content.log" | head -3 | sed 's/^/      /'
[ "${RC}" != "0" ] && grep -q 'tools/ls.md' "${BASE}/neg-content.log" \
  && ok "7b 改一个 blob 内容 ⇒ 比较器报红（rc=${RC}）且点名 tools/ls.md" \
  || bad "7b 内容差异没被抓到（rc=${RC}）"

# ── 8. 反例：契约必须承重（空契约 ⇒ 真差异必须被判契约外）────────────────────────────────
head1 "8 反例（契约承重）：空契约下真差异必须失败"
: > "${BASE}/empty-contract.tsv"
python3 "${COMPARE}" --old "${OLD}" --new "${MIR}" --contract "${BASE}/empty-contract.tsv" \
  > "${BASE}/neg-empty.log" 2>&1
RC=$?
grep -E '契约外差异|\.mirror-state' "${BASE}/neg-empty.log" | head -4 | sed 's/^/      /'
[ "${RC}" != "0" ] && grep -q '契约外差异' "${BASE}/neg-empty.log" \
  && ok "8 空契约 ⇒ 真差异被判契约外并失败（rc=${RC}）⇒ 契约清单是**承重**的，不是装饰" \
  || bad "8 空契约竟然也过（rc=${RC}）⇒ 契约过滤器没在起作用"

# ── 9. 反例：契约不许当许可证 ────────────────────────────────────────────────────────────
head1 "9 反例（契约护栏）：把契约写成 \`*\` ⇒ 比较器必须拒绝"
printf '*\tany\t放行一切\n' > "${BASE}/bad-contract.tsv"
python3 "${COMPARE}" --old "${OLD}" --new "${MIR}" --contract "${BASE}/bad-contract.tsv" \
  > "${BASE}/neg-star.log" 2>&1
RC=$?
grep -E '通配符|许可证' "${BASE}/neg-star.log" | head -2 | sed 's/^/      /'
[ "${RC}" != "0" ] && grep -q '许可证' "${BASE}/neg-star.log" \
  && ok "9 \`*\` 契约被拒绝（rc=${RC}）⇒ 契约不能把全部差异一次性豁免" \
  || bad "9 \`*\` 契约竟然被接受（rc=${RC}）"

# ── 10/11. 私有仓只读 + 绝不推送 ─────────────────────────────────────────────────────────
head1 "10 私有仓未被动过 · 11 绝不推送"
PRIV_HEAD1="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
PRIV_DIRTY1="$(git -C "${REPO_ROOT}" status --porcelain | wc -l | tr -d ' ')"
echo "  HEAD: ${PRIV_HEAD0:0:12} → ${PRIV_HEAD1:0:12} · porcelain: ${PRIV_DIRTY0} → ${PRIV_DIRTY1}"
[ "${PRIV_HEAD0}" = "${PRIV_HEAD1}" ] && ok "10 私有仓 HEAD 未变" || bad "10 私有仓 HEAD 被改！"
[ "${PRIV_DIRTY0}" = "${PRIV_DIRTY1}" ] && ok "10 porcelain 行数未变（两器都没碰私有工作树）" \
  || bad "10 私有仓工作树状态变了（${PRIV_DIRTY0} → ${PRIV_DIRTY1}）"
REMOTES="$(git -C "${OLD}" remote 2>/dev/null | wc -l | tr -d ' ')$(git -C "${MIR}" remote 2>/dev/null | wc -l | tr -d ' ')"
[ "${REMOTES}" = "00" ] && ok "11 两份产出仓都没有 remote（没有任何推送路径）" || bad "11 产出仓配了 remote（${REMOTES}）"
# 「推送成功」的两种工具真输出形如 `已推送:` / `已推送（非 force）:`；两器在 dry-run 下打印的是
# **推送命令提示**（含 `git push` 字样），那是文本不是动作 ⇒ 断言只认成功行，别把提示当动作。
if grep -qE '^已推送|已推送（非 force）' "${BASE}/old.log" "${BASE}/mirror.log" 2>/dev/null; then
  bad "11 日志里出现「已推送」成功行 —— 有真推送"
elif grep -q 'ZERG_MIRROR_ALLOW_PUSH=1' "${BASE}/mirror.log" 2>/dev/null; then
  bad "11 镜像器走了 --push 双重保险路径"
else
  ok "11 两器日志无「已推送」成功行、无双保险放行（只打印了推送命令提示，未执行）"
fi

head1 "小结"
echo "  通过 ${PASS} 项 · 失败 ${FAIL} 项 · 工作区 ${BASE}"
if [ "${FAIL}" = "0" ]; then
  echo "  ✅ 两器产出**树内容一致**（路径集合 + 逐路径 blob sha + 模式），差异集合 ⊆ ${CONTRACT##*/}"
  echo "     反例已验：多路径 / 改 blob / 空契约 / \`*\` 契约 四种造差异路径都能失败"
  exit 0
fi
echo "  ❌ 有失败项，逐条见上"
exit 1

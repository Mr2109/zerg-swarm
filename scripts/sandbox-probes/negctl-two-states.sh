#!/usr/bin/env bash
# negctl-two-states.sh —— verify-two-states.py 的**负例活体控制**（判据 7 的「红的版本」落到门禁自身）
#
# 两条负例都必须**硬失败 rc=2**（不许静默跳过）：
#   ① 探针缺失/路径漂移 —— 临时把 pF.sb 挪出仓，跑门禁 ⇒ 必须 rc=2；随即还原并比对 sha（逐字节一致）
#   ② 基线无区分度 —— 指定一个本机不可达的判别目标 ⇒ 必须 rc=2（防止「出网被拦」被探针本身坏了刷成假绿）
#
# 用法：bash scripts/sandbox-probes/negctl-two-states.sh
# 退出码：0 = 两条负例都按期望硬失败且现场已还原；1 = 有负例没按期望失败 / 还原失败（停手人工看）
set -u
cd "$(dirname "${BASH_SOURCE[0]}")/../.." || { echo "!! 进不了仓根 ⇒ 停手"; exit 1; }
P="scripts/sandbox-probes"
BAK="/tmp/pF.sb.bak.$$"
MOVED="/tmp/pF.sb.moved.$$"

echo "=== 负例① 探针缺失（pF.sb 临时挪走）==="
cp -p "${P}/pF.sb" "${BAK}" || { echo "!! 备份失败 ⇒ 停手"; exit 1; }
BEFORE=$(shasum -a 256 "${P}/pF.sb" | awk '{print $1}')
mv "${P}/pF.sb" "${MOVED}" || { echo "!! 挪走失败 ⇒ 停手"; exit 1; }
python3 "${P}/verify-two-states.py"; rc=$?
echo "负例① rc=$rc （期望 2）"
mv "${MOVED}" "${P}/pF.sb" || { echo "!! 还原失败 ⇒ 立即手工还原：mv ${MOVED} ${P}/pF.sb"; exit 1; }
AFTER=$(shasum -a 256 "${P}/pF.sb" | awk '{print $1}')
echo "还原 sha 比对：before=$BEFORE after=$AFTER"
[ "$BEFORE" = "$AFTER" ] || { echo "!! 还原后 sha 不一致 ⇒ 停手"; exit 1; }
[ "$rc" -eq 2 ] || { echo "!! 负例① 未按期望硬失败"; exit 1; }

echo ""
echo "=== 负例② 基线无区分度（指定一个不可达的判别目标）==="
TWO_STATE_TARGETS="<worker-ip>:9" python3 "${P}/verify-two-states.py"; rc=$?
echo "负例② rc=$rc （期望 2）"
[ "$rc" -eq 2 ] || { echo "!! 负例② 未按期望硬失败"; exit 1; }

echo ""
echo "=== 还原确认（该件应无改动）==="
git status --short "${P}"
echo "全部负例按期望硬失败 ✓"

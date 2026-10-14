#!/bin/bash
# zerg-dbbackup.sh —— 虫族库「**先备份、后归档**」的调度脚本（T4① · `~/Library/LaunchAgents/com.zerg.dbbackup.plist`
# 的 `ProgramArguments`：`/bin/bash <本件>` · `StartCalendarInterval` 03:30 本地 · WorkingDirectory = 仓根）。
#
# 唯一入口纪律：本脚本**不实现**备份 / 归档逻辑（✗ 第二份口径）—— 逐字转发到**命令面**
# `zerg core db backup`（= `core/internal/store/zdb` 的 `BackupDefault` 体例：
# checkpoint(TRUNCATE) → UTC 命名 → VACUUM INTO → 侧车八格 → GFS prune 日14/周8/月12 · 保底 ≥3）。
#
# ★ 归档那一半**本版未接线**（照实登记 · ✗ 不静默跳过 · ✗ 不假装做过）：
#   设计 §四.3 的归档动作 = 到期 DROP 下窗表 ＋ `PRAGMA incremental_vacuum`
#   （`zdb/archive.go` 的 `DropWindowTable` / `ReclaimSpace`）—— 二者**本版只落了库函数**，
#   命令面**没有出口**（`zerg core db` 的动作闭集 = check | backup | restore | prune）⇒
#   本脚本只做备份那一半，并在 stdout 明写「归档未接线」。归档出口落地后，在下面第二段补上
#   （**顺序铁律：备份在前、归档在后**）。
#
# 退码：0 备份成功 · 非 0 = 备份失败（launchd 会把 rc 记进作业的 last exit status ⇒ `launchctl print` 可查）。
#
# 落点（✗ 一律不写死私有绝对路径：仓根由本件位置现算）：
#   本件在 <仓根>/scripts/svc/ ⇒ 上溯两级 = 仓根 ⇒ 命令件 = <仓根>/bin/zerg。

set -u

SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
ZERG_BIN="${REPO_ROOT}/bin/zerg"

if [ ! -d "${REPO_ROOT}" ]; then
  echo "zerg-dbbackup: 仓根解析不到（${REPO_ROOT}）⇒ 停手（✗ 不猜别处）" >&2
  exit 2
fi
if [ ! -x "${ZERG_BIN}" ]; then
  echo "zerg-dbbackup: 命令件不在（${ZERG_BIN}）⇒ 停手（✗ 不猜别处；先 `make build`）" >&2
  exit 2
fi

cd "${REPO_ROOT}" || { echo "zerg-dbbackup: 进不了仓根（${REPO_ROOT}）" >&2; exit 2; }

echo "zerg-dbbackup: ① 备份开始（$(date -u +%Y-%m-%dT%H:%M:%SZ) · 库 = ${REPO_ROOT}/.runtime/zerg.db）"
"${ZERG_BIN}" core db backup --yes
backup_rc=$?
echo "zerg-dbbackup: ① 备份结束（rc=${backup_rc}）"

if [ "${backup_rc}" -ne 0 ]; then
  echo "zerg-dbbackup: 备份没过 ⇒ ② 归档段**不跑**（顺序铁律：先备份、后归档 —— 没备份成功就不许去动归档）" >&2
  exit "${backup_rc}"
fi

echo "zerg-dbbackup: ② 归档段**未接线**（命令面暂无归档出口 ⇒ 本版只做备份那一半 · ✗ 未做、也不算做过）"
exit 0

#!/bin/bash
# zerg-swap-ui.sh —— 桌面 UI（zerg-ui）换件的**受控流程 · 只报告档**（P13 扩闸 · 2026-09-21）。
#
# 形态来源：`scripts/build/zerg-swap-core.sh`（主控换件 · 2026-09-16 事故后固化）——
#   本件把它的**三道门禁 + 两道前置闸**逐条照搬成 UI 那一份，**一号一义、不另立形态**。
#   与子端那份（`zerg-swap-agent.sh`）的差别**只有三处目标物**（件 / 启动方式 / 自证判据），
#   判据形状逐条一致 —— 三份换件闸必须同形，否则「三闸形态」这句话就是三套。
#
# 三道门禁（与主控件逐条对应）：
#   ① 构建只走 `scripts/build/build-all.sh`（身份注入 + ad-hoc 重签都在里面）——**绝不手搓 cargo build**；
#   ② 换件前必须**验签**（`codesign --verify`，输出原样打印）——签名不过就不换；
#      （UI 尤其要命：macOS 上 UI 件签名失效会被**秒杀 EXIT137**，且面板上看不出原因）
#   ③ 起不来 ⇒ 回滚 + **回滚件同样验签** + 打印失败现场（**任何命令的输出都不许被吞**）。
# 两道前置闸：
#   闸 A **工作树里没有并行的半成品**（ui/ 与 wall/ 下有未提交改动 ⇒ 半成品进生产件）；
#   闸 B **启动链可用**：UI 不是 launchd 作业，走 A6 真源 `scripts/build/start-zerg-ui.sh`
#        （正门 `zerg ui start` 转发到它；它 os.execv 仓内 `bin/zerg-ui`）——
#        顶层 `start-zerg-ui.sh`=A17 已于 2026-10-08 退场并入 A6（剥代理已在 `ui/src/api.rs` 根治）。
#        ⇒ 判据 = 启动器在位 **且** 它 exec 的那个件真存在（启动器在位而件被搬走 = 同族悬空）。
#
# ★ 本轮档位 = **只报告**（P13 拍板原文：「先工装 · 先只报告」）：
#   · 本件**只读**：不写 `bin/`、不换件、不重启 UI、不碰启动器；
#   · **真换件档未开放** ⇒ 今天敲 `--swap` 一律 rc=2 且零副作用（要开先拍）。
#   · UI 的自证判据与子端不同：它**没有 HTTP 端点** ⇒ 判据落在「进程活着 + 进程当前跑的就是盘上那个件」
#     （按 inode + sha256 对拍），这是本件与子端那份唯一的实质性差异。
#
# 用法：bash scripts/build/zerg-swap-ui.sh            # 只报告（**默认动作**，只读）
#       bash scripts/build/zerg-swap-ui.sh --check    # 同上（显式写法）
#       bash scripts/build/zerg-swap-ui.sh --help     # 用法与退码表（原样打印，见 usage()）
#       bash scripts/build/zerg-swap-ui.sh --swap     # **未开放** ⇒ rc=2，不执行任何动作
#
# 退码（唯一说法 = usage()；与主控件 `zerg-swap-core.sh` **同号同义**）：
#   0 全部闸通过（只读判定）· 1 有闸判 FAIL · 2 用法错 / **不给结论**（有 BLOCKED）/ 真换件档未开放
#   3 **保留号**（回滚也失败 ⇒ 需人工）—— 只报告档跑不到它，占号是为与主控件逐号对齐
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

BIN_REL="${ZERG_SWAP_BIN:-bin/zerg-ui}"
LAUNCHER_REL="${ZERG_SWAP_LAUNCHER:-scripts/build/start-zerg-ui.sh}"
TS="$(date +%Y%m%dT%H%M%S)"
LOG="${ZERG_SWAP_LOG:-/tmp/zerg-swap-ui-$TS.log}"

N_PASS=0; N_FAIL=0; N_BLOCK=0
say() { echo "$@" | tee -a "$LOG"; }
hr()  { say "──────────────────────────────────────────────"; }

# ── 用法（--help 原文即下面这段；**退码与档位的唯一定义处**）──────────────────────
usage() {
  cat <<'EOF'
zerg-swap-ui.sh —— 桌面 UI（zerg-ui）换件的受控流程 · **只报告档**（本轮未开放真换件）

用法：
  bash scripts/build/zerg-swap-ui.sh            只报告：逐闸出判决（**只读**，零副作用）
  bash scripts/build/zerg-swap-ui.sh --check    同上（显式写法）
  bash scripts/build/zerg-swap-ui.sh --help     本页
  bash scripts/build/zerg-swap-ui.sh --swap     真换件 —— **本档未开放**，rc=2 且不执行任何动作

判的六闸（三闸形态 + 两前置闸；逐条对应 scripts/build/zerg-swap-core.sh）：
  闸 A 工作树半成品：`git status --porcelain ui/ wall/ core/ agent/` 的非未跟踪项计数必须 0
  闸 B 启动链可用：A6 真源 `scripts/build/start-zerg-ui.sh` 在位（正门 `zerg ui start`），**且**它 exec 的 `bin/zerg-ui` 真存在
  闸① 构建口径：件必须由 scripts/build/build-all.sh 产出；读 bin/build-info.json 判身份四格
  闸② 验签：目标件必须 `codesign --verify` 通过（UI 件验签失效 = 秒杀 EXIT137）
  闸③ 回滚位：回滚件（bin/_history/zerg-ui.prev）必须**在位且验签通过**
  闸④ 换件后自证判据可跑：UI **无 HTTP 端点** ⇒ 判据 = 进程活着 **且** 它当前跑的件与盘上件同 inode/sha256

退码（唯一说法）：
  0  全部闸 PASS（只读判定通过）
  1  有闸判 FAIL（真红）
  2  用法错 · 或**不给结论**（有闸 BLOCKED）· 或敲了 `--swap`（**真换件档未开放**，要开先拍）
  3  **保留号**（回滚也失败 ⇒ 需人工）；只报告档跑不到它，占号是为与主控件逐号对齐
EOF
}

ACTION="check"
case "${1:-}" in
  ""|--check) ACTION="check" ;;
  --swap)     ACTION="swap" ;;
  --help|-h)  usage; exit 0 ;;
  *) echo "✗ 未知参数: $1（用法见：bash scripts/build/zerg-swap-ui.sh --help）" >&2; exit 2 ;;
esac

if [ "$ACTION" = "swap" ]; then
  echo "✗ zerg-swap-ui.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）" >&2
  echo "  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh 的 UI 档 → 验签 → 重启 → 自证 → 回滚路径。" >&2
  exit 2
fi

say "== UI 换件闸（只报告 · 只读）$TS =="
say "日志：$LOG"
say "目标件：$BIN_REL · 启动链：$LAUNCHER_REL"

judge() {  # $1=闸名 $2=PASS|FAIL|BLOCKED $3=一句话依据
  case "$2" in
    PASS)    N_PASS=$((N_PASS+1)) ;;
    FAIL)    N_FAIL=$((N_FAIL+1)) ;;
    BLOCKED) N_BLOCK=$((N_BLOCK+1)) ;;
    *) echo "!! 内部错误：未知判决 $2" >&2; exit 2 ;;
  esac
  say "  [闸] $1 ⇒ $2 —— $3"
}

hr; say "[闸A] 工作树半成品（换件是从工作树编译的）"
DIRTY="$(git status --porcelain ui/ wall/ core/ agent/ 2>/dev/null | grep -v '^?? ' | wc -l | tr -d ' ')"
if [ "$DIRTY" = "0" ]; then
  judge "A 工作树半成品" PASS "未提交改动数 = 0"
else
  git -c core.quotepath=false status --porcelain ui/ wall/ core/ agent/ 2>/dev/null | grep -v '^?? ' | sed 's/^/       /' | tee -a "$LOG"
  judge "A 工作树半成品" FAIL "未提交改动数 = ${DIRTY}（换件会把半成品编进生产件）"
fi

hr; say "[闸B] 启动链可用（A6 真源：正门 zerg ui start → 启动器在位 ∧ 它 exec 的件在位）"
# A6 真源 = `scripts/build/start-zerg-ui.sh`（单一真源·顶层 `start-zerg-ui.sh`=A17 已于 2026-10-08 退场并入它）。
# exec 目标两种写法都认：A6 真源 `BIN=<仓根>/bin/…`（os.execv）；旧顶层（已退）`./bin/…`。
if [ ! -f "$LAUNCHER_REL" ]; then
  judge "B 启动链" FAIL "启动器不存在（${LAUNCHER_REL}）—— 换上去也起不来"
else
  EXEC_TARGET="$(grep -oE 'BIN="\$REPO_ROOT/[A-Za-z0-9_./-]+"' "$LAUNCHER_REL" 2>/dev/null | tail -1 | sed 's#^BIN="\$REPO_ROOT/##' | sed 's#"$##')"
  if [ -z "$EXEC_TARGET" ]; then
    EXEC_TARGET="$(grep -oE '\./[A-Za-z0-9_./-]+' "$LAUNCHER_REL" 2>/dev/null | tail -1 | sed 's#^\./##')"
  fi
  say "      启动器：${LAUNCHER_REL} · 它 exec 的件：${EXEC_TARGET:-（解析不到）}"
  if [ -z "$EXEC_TARGET" ]; then
    judge "B 启动链" BLOCKED "启动器里解析不出 exec 目标 —— 判不了，不许当绿"
  elif [ ! -e "$EXEC_TARGET" ]; then
    judge "B 启动链" FAIL "启动器 exec 的件盘上不存在（${EXEC_TARGET}）—— 悬空，同族于主控件 2026-09-20 那次"
  else
    judge "B 启动链" PASS "启动器（A6 真源 ${LAUNCHER_REL}）与它 exec 的件（${EXEC_TARGET}）都在位"
  fi
fi

hr; say "[闸①] 构建口径（件必须由 build-all.sh 产出 · 身份不许丢）"
BI="bin/build-info.json"
if [ ! -f "$BI" ]; then
  judge "① 构建口径" BLOCKED "缺 ${BI}（读不到构件身份 ⇒ 判不了，不许当绿）"
elif ! python3 - "$BI" <<'PY' >/dev/null 2>&1
import json, sys
d = json.load(open(sys.argv[1]))
assert d.get("version") and d.get("commit") and d.get("build_time") and d.get("builder_host")
PY
then
  judge "① 构建口径" BLOCKED "${BI} 解析不出四格身份（version/commit/build_time/builder_host）"
else
  IDENT="$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print("%s · %s · %s · %s"%(d["version"],d["commit"],d["build_time"],d["builder_host"]))' "$BI")"
  judge "① 构建口径" PASS "构件身份齐：${IDENT}（造件入口 = scripts/build/build-all.sh）"
fi

hr; say "[闸②] 验签（codesign --verify —— 输出原样打印，不吞）"
if [ ! -e "$BIN_REL" ]; then
  judge "② 验签" BLOCKED "件不存在（${BIN_REL}）—— 判不了"
else
  say "      $(codesign -dv --verbose=2 "$BIN_REL" 2>&1 | grep -E 'Identifier|Signature|Format' | tr '\n' ' ')"
  if codesign --verify --verbose=2 "$BIN_REL" 2>&1 | sed 's/^/      verify: /' | tee -a "$LOG"; then
    judge "② 验签" PASS "${BIN_REL} 验签通过"
  else
    judge "② 验签" FAIL "${BIN_REL} 验签不过（UI 件签名失效会被秒杀 EXIT137）"
  fi
fi

hr; say "[闸③] 回滚位（回滚件必须**在位且验签通过**）"
RB="bin/_history/$(basename "$BIN_REL").prev"
if [ ! -e "$RB" ]; then
  judge "③ 回滚位" BLOCKED "无回滚件（${RB}）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）"
elif codesign --verify --verbose=1 "$RB" >/dev/null 2>&1; then
  judge "③ 回滚位" PASS "回滚件在位且验签通过（${RB}）"
else
  judge "③ 回滚位" FAIL "回滚件在位但**验签不过**（${RB}）—— 2026-09-16 二次事故的形态"
fi

hr; say "[闸④] 换件后自证判据可跑（UI 无 HTTP 端点 ⇒ 进程活着 ∧ 跑的件 == 盘上件）"
if [ ! -e "$BIN_REL" ]; then
  judge "④ 自证判据" BLOCKED "件不存在 ⇒ 无从对拍——判不了"
else
  DISK_SHA="$(shasum -a 256 "$BIN_REL" | awk '{print $1}')"
  DISK_INO="$(stat -f %i "$BIN_REL")"
  PIDS="$(pgrep -f "$BIN_REL" 2>/dev/null | tr '\n' ' ' | sed 's/ $//')"
  say "      盘上件 inode=${DISK_INO} sha256=${DISK_SHA:0:16}…"
  if [ -z "$PIDS" ]; then
    judge "④ 自证判据" BLOCKED "UI 进程没在跑（pgrep 零命中）—— 判据本身活着，但此刻无从自证（**不是绿**）"
  else
    say "      在跑进程：$PIDS"
    OK=1
    for pid in $PIDS; do
      LOF="$(lsof -p "$pid" 2>/dev/null | awk -v p="$BIN_REL" 'index($NF,p)>0 {print $NF; exit}')"
      if [ "$LOF" != "$BIN_REL" ] && [ "$LOF" != "$REPO_ROOT/$BIN_REL" ]; then
        say "        pid $pid 映射的件解析不出（lsof 读到：${LOF:-（空）}）⇒ 本闸不给结论"
        OK=0
      fi
    done
    if [ "$OK" = "1" ]; then
      judge "④ 自证判据" PASS "进程在跑且映射到 ${BIN_REL}（判据：进程活着 ∧ 跑的就是盘上那个件）"
    else
      judge "④ 自证判据" BLOCKED "进程在跑但「它跑的是哪个件」解析不出 ⇒ 判不了，不许当绿"
    fi
  fi
fi

hr
say "状态计数：PASS $N_PASS · FAIL $N_FAIL · BLOCKED(不给结论) $N_BLOCK"
say "日志：$LOG"
if [ "$N_FAIL" -gt 0 ]; then
  say "⇒ 有闸判 FAIL（rc=1）—— 逐条见上"
  exit 1
fi
if [ "$N_BLOCK" -gt 0 ]; then
  say "⇒ 有闸 BLOCKED（rc=2 · **不给结论 ⇒ 不许当绿**，与提交闸同码同义）"
  exit 2
fi
say "⇒ 六闸全通过（rc=0 · 只读判定；本件**未换件、未重启、未改任何文件**）"
exit 0

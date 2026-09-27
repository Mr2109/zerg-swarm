#!/bin/bash
# zerg-swap-agent.sh —— 子端（zerg-agentd）换件的**受控流程 · 只报告档**（P13 扩闸 · 2026-09-21）。
#
# 形态来源：`scripts/build/zerg-swap-core.sh`（主控换件 · 2026-09-16 事故后固化，Mr2109 拍）——
#   本件把它的**三道门禁 + 两道前置闸**逐条照搬成子端那一份，**一号一义、不另立形态**。
#
# 三道门禁（与主控件逐条对应）：
#   ① 构建只走 `scripts/build/build-all.sh`（身份注入 + ad-hoc 重签都在里面）——**绝不手搓 go build**；
#   ② 换件前必须**验签**（取数档 `codesign -dvvv` ⇒ `Signature=` / `flags=` / `cdhash` 三格齐**且非 adhoc**；
#      输出原样打印）——`--verify` 不过是签名失效；**adhoc 档判红**（`--verify` rc=0 也不再算过）；
#   ③ 起不来 ⇒ 回滚 + **回滚件同样验签** + 打印失败现场（**任何命令的输出都不许被吞**）。
# 两道前置闸（换件前跑，不过 ⇒ 不换件且**未动生产**）：
#   闸 A **工作树里没有并行的半成品**：换件是**从工作树编译**的 ⇒ core/agent/ui/wall 下有未提交改动就把半成品编进生产件。
#   闸 B **服务定义可用**：盘上 launchd plist 引用的程序（`bin/zerg-agentd` 等）**必须真存在**。
#        为什么单列：脚本/产物搬家后 plist 里那句路径会变成**悬空**，而 launchd **缓存的是加载时的定义**
#        ——此时 `kickstart -k` 会先把在跑的子端 kill 掉、再用陈旧路径拉不起来，**回滚路径同样起不来**
#        （主控件 2026-09-20 真实发生过一次的同一形态；子端是同族风险）。
#
# ★ 本轮档位 = **只报告**（P13 拍板原文：「先工装 · 先只报告」，语义照 §11 的 `tri-report`）：
#   · 本件**只读**：不写 `bin/`、不换件、不重启、不碰 launchd、不动任何配置文件；
#   · **真换件档未开放** —— 要开先拍（见 usage() 的「未开放」段），今天敲它一律 **rc=2 且零副作用**。
#   · 为什么先只报告：跨架构签名缺口、`bin/_history/` 里**没有 `zerg-agentd.prev` 回滚位**这两条
#     今天就是红的（现跑见判决），直接升阻断等于把发布闸锁死 —— 先拿基线，归零再升档（G-3 既有节奏）。
#
# 用法：bash scripts/build/zerg-swap-agent.sh            # 只报告（**默认动作**，只读）
#       bash scripts/build/zerg-swap-agent.sh --check    # 同上（显式写法）
#       bash scripts/build/zerg-swap-agent.sh --help     # 用法与退码表（原样打印，见 usage()）
#       bash scripts/build/zerg-swap-agent.sh --swap     # **未开放** ⇒ rc=2，不执行任何动作
#
# 退码（唯一说法 = usage()；与主控件 `zerg-swap-core.sh` **同号同义**）：
#   0 全部闸通过（只读判定）· 1 有闸判 FAIL · 2 用法错 / **不给结论**（有 BLOCKED）/ 真换件档未开放
#   3 **保留号**（回滚也失败 ⇒ 需人工）—— 只报告档跑不到它，占号是为与主控件逐号对齐
#   ★ 没有 READY 式「回滚成功 = 1 + ROLLED_BACK」那一支：本档**不执行**回滚，那一支留给开启真换件档时补。
#
# ★ 纪律句（**不是判据** ✗ · 启动方式口径）：一切执行体一律 `bash <绝对路径>` 启动 ⇒ **不依赖 x 位**
#   —— 判据**不声称**「这枚执行体可直接执行」（真机上执行体可能没有 x 位，现读 edge-node 真身即 664）。
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

BIN_REL="${ZERG_SWAP_BIN:-bin/zerg-agentd}"
PLIST_LABEL="${ZERG_SWAP_LABEL:-com.zerg.agent-Mr2109}"
DOMAIN="gui/$UID"
PLIST="${ZERG_SWAP_PLIST:-$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist}"
TOKEN_FILE="${ZERG_SWAP_TOKEN_FILE:-$HOME/.zerg/token}"
# ── 令牌**不进 argv**（`Q-207` 拍板 · Mr2109 2026-09-26 · 缺口账 §附录四十五）──────────
# 值经 **stdin** 交给 curl（`-H @-`）⇒ 命令行上永不出现令牌的**字面值**、也不出现**变量展开**
# （目标机 `ps`/进程表里取不到实参令牌）；读法沿用旧口径：`$TOKEN_FILE`（默认 `~/.zerg/token`，600）。
# 缺件/读不到 ⇒ 退「不带该头」的形态（与旧行为等价：服务端回 401，判据照旧判红/判 BLOCKED）。
TOKEN_FILE="${TOKEN_FILE:-$HOME/.zerg/token}"
curl_tok() {  # curl_tok <curl 参数…>
  if [ -r "$TOKEN_FILE" ]; then
    printf 'X-Auth-Token: %s\n' "$(cat "$TOKEN_FILE" 2>/dev/null || true)" | curl -H @- "$@"
  else
    curl "$@"
  fi
}
READY_URL="${ZERG_SWAP_READY_URL:-http://127.0.0.1:8100/ready}"
STATUS_URL="${ZERG_SWAP_STATUS_URL:-http://127.0.0.1:8100/status}"
TS="$(date +%Y%m%dT%H%M%S)"
LOG="${ZERG_SWAP_LOG:-/tmp/zerg-swap-agent-$TS.log}"

N_PASS=0; N_FAIL=0; N_BLOCK=0
say() { echo "$@" | tee -a "$LOG"; }
hr()  { say "──────────────────────────────────────────────"; }

# ── 用法（--help 原文即下面这段；**退码与档位的唯一定义处**）──────────────────────
usage() {
  cat <<'EOF'
zerg-swap-agent.sh —— 子端（zerg-agentd）换件的受控流程 · **只报告档**（本轮未开放真换件）

用法：
  bash scripts/build/zerg-swap-agent.sh            只报告：逐闸出判决（**只读**，零副作用）
  bash scripts/build/zerg-swap-agent.sh --check    同上（显式写法）
  bash scripts/build/zerg-swap-agent.sh --help     本页
  bash scripts/build/zerg-swap-agent.sh --swap     真换件 —— **本档未开放**，rc=2 且不执行任何动作

判的六闸（三闸形态 + 两前置闸；逐条对应 scripts/build/zerg-swap-core.sh）：
  闸 A 工作树半成品：`git status --porcelain core/ agent/ ui/ wall/` 的非未跟踪项计数必须 0（换件是从工作树编译的）
  闸 B 服务定义可用：盘上 launchd plist 引用的程序必须真存在（脚本/产物搬家会让那句路径悬空）
  闸① 构建口径：件必须由 scripts/build/build-all.sh 产出（身份注入 + 重签）；读 bin/build-info.json 判
  闸② 验签：取数档 `codesign -dvvv` ⇒ `Signature=` / `flags=` / `cdhash` **三格齐且非 adhoc**（adhoc ⇒ **判红**；输出原样打印，不吞）
  闸③ 回滚位：回滚件（bin/_history/<件>.prev）必须**在位且验签通过** —— 「退得回去」得先证明
  闸④ 换件后自证判据可跑：探针打到子端自身 `/ready` + `/status`（只读 GET），证明判据活着

纪律句（**不是判据** · 启动方式口径 · ⒟甲）：
  一切执行体一律 `bash <绝对路径>` 启动 ⇒ **不依赖 x 位**（真机上件可能没有 x 位；判据不声称「可直接执行」）

退码（唯一说法）：
  0  全部闸 PASS（只读判定通过）
  1  有闸判 FAIL（真红：件缺/验签不过/工作树有半成品/plist 悬空…）
  2  用法错 · 或**不给结论**（有闸 BLOCKED：缺件、读不动、探针打不到、空转——**不许当绿**）
     · 或敲了 `--swap`（**真换件档未开放**：要开先拍，见下）
  3  **保留号**（回滚也失败 ⇒ 需人工）；只报告档跑不到它，占号是为与主控件逐号对齐

真换件档为什么未开放（要开先拍，不偷偷开）：
  ① 本件今天**红两处**：`bin/_history/` 里没有 `zerg-agentd.prev` 回滚位；跨架构件无签名（§11 B7/P4）；
  ② 换件要动正在跑的子端进程 ⇒ 属**不可逆档**，按仓内口径要 Mr2109 拍；
  ③ 拍之前，本件只做「证明它会不会红」这件事本身（先只报告 → 归零 → 升阻断）。
EOF
}

ACTION="check"
case "${1:-}" in
  ""|--check) ACTION="check" ;;
  --swap)     ACTION="swap" ;;
  --help|-h)  usage; exit 0 ;;
  *) echo "✗ 未知参数: $1（用法见：bash scripts/build/zerg-swap-agent.sh --help）" >&2; exit 2 ;;
esac

if [ "$ACTION" = "swap" ]; then
  echo "✗ zerg-swap-agent.sh：**真换件档未开放**（P13 拍板：先工装 · 先只报告）" >&2
  echo "  ⇒ 零副作用退出（rc=2）。要开先拍；拍后本档补：备份 → build-all.sh --only-agentd → 验签 → 重启 → 自证 → 回滚路径。" >&2
  exit 2
fi

say "== 子端换件闸（只报告 · 只读）$TS =="
say "日志：$LOG"
say "目标件：$BIN_REL · 作业：$PLIST_LABEL"

# ── 判档小工具（本件自用；四档与仓内口径一致：PASS / FAIL / BLOCKED）──────────────
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
DIRTY="$(git status --porcelain core/ agent/ ui/ wall/ 2>/dev/null | grep -v '^?? ' | wc -l | tr -d ' ')"
if [ "$DIRTY" = "0" ]; then
  judge "A 工作树半成品" PASS "未提交改动数 = 0"
else
  git -c core.quotepath=false status --porcelain core/ agent/ ui/ wall/ 2>/dev/null | grep -v '^?? ' | sed 's/^/       /' | tee -a "$LOG"
  judge "A 工作树半成品" FAIL "未提交改动数 = ${DIRTY}（换件会把半成品编进生产件）"
fi

hr; say "[闸B] 服务定义可用（盘上 plist 引用的程序必须真存在）"
say "      作业：${DOMAIN}/${PLIST_LABEL} · plist：${PLIST}"
if [ ! -f "$PLIST" ]; then
  judge "B 服务定义" BLOCKED "盘上 plist 不存在（${PLIST}）—— 判不了，不许当绿"
else
  MISSING="$(python3 - "$PLIST" <<'PY'
import os, plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
except Exception as e:
    print("(读不到 plist: %s)" % e); sys.exit(0)
for a in (d.get("ProgramArguments") or []):
    if a.startswith("/") and not os.path.exists(a):
        print(a)
PY
)"
  PROG_STR="$(python3 - "$PLIST" <<'PY'
import plistlib, sys
try:
    d = plistlib.load(open(sys.argv[1], "rb"))
    print(" ".join(d.get("ProgramArguments") or []))
except Exception as e:
    print("(读不到 plist: %s)" % e)
PY
)"
  say "      程序串：$PROG_STR"
  if [ -n "$MISSING" ]; then
    say "      plist 引用但盘上不存在的项："
    printf '        %s\n' $MISSING | tee -a "$LOG"
    judge "B 服务定义" FAIL "plist 引用的程序盘上不存在（launchd 用缓存定义 ⇒ kickstart 与回滚都会起不来）"
  else
    judge "B 服务定义" PASS "plist 引用的程序全部在位"
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

# ── 两枚换件器共用的**只读判据**（条 10 · `序 121` ／ 条 17 · `Q-207`）──────────────────
# `sign_three_cells`：换前验签的**取数档** —— `codesign -dvvv` 同档出 `Signature=` / `flags=` /
#   `cdhash` 三格（`-dv` / `-dvv` **均无** `CDHash` ⇒ 三格不在同一档就核不出一件是谁签的）。
#   退码：0 三格齐且**非 adhoc**（放行）· 1 **判红**（adhoc / 三格缺）· 2 **不给结论**（件不在 / codesign 起不来）。
#   为什么判红不再看 `codesign --verify`：adhoc 件 `--verify` **rc=0**（实测 `cp /bin/echo` + `codesign -s -`
#   ⇒ 通过）⇒ 只认 `--verify` 就等于「负控永不红」；adhoc 件换上会被秒杀 EXIT137。
sign_three_cells() {  # $1 = 件
  local bin="$1" out sig sigsize flags cdhash
  [ -e "$bin" ] || { echo "件不存在：${bin}"; return 2; }
  out="$(codesign -dvvv "$bin" 2>&1)" || { echo "codesign -dvvv 起不来：${bin}"; return 2; }
  sig="$(printf '%s\n' "$out" | sed -n 's/^Signature=\(.*\)$/\1/p' | head -1)"            # 只有 adhoc 是等号形
  sigsize="$(printf '%s\n' "$out" | sed -n 's/^Signature size=\(.*\)$/\1/p' | head -1)"   # 真签是 `Signature size=`
  flags="$(printf '%s\n' "$out" | grep -E '^CodeDirectory ' | sed -n 's/.*flags=\([^ ]*\).*/\1/p' | head -1)"
  cdhash="$(printf '%s\n' "$out" | grep -E '^(Candidate)?CDHash=' | head -1 | cut -d= -f2-)"
  echo "Signature=${sig:-真签（size=${sigsize:-缺格}）} · flags=${flags:-（缺格）} · cdhash=${cdhash:-（缺格）}"
  case "${sig}" in
    adhoc) echo "判红：Signature=adhoc —— 验签 rc=0 不算过（adhoc 件换上会被秒杀 EXIT137）"; return 1 ;;
  esac
  case "${flags}" in
    *adhoc*) echo "判红：flags=${flags}（ad-hoc 档）"; return 1 ;;
  esac
  if [ -z "${flags}" ] || [ -z "${cdhash}" ] || { [ -z "${sig}" ] && [ -z "${sigsize}" ]; }; then
    echo "判红：三格不齐（Signature / flags / cdhash）"; return 1
  fi
  return 0
}

# `history_drop_ok`：留档落点**一律** `<仓根>/bin/_history/…` —— 落在 `bin/` 根（产出面根）里
#   平铺一份 `*.bak-*` **判红**（本机现读仍有 14 枚**人手造的历史平铺件** ⇒ **不搬家** ✗，只管新落点）。
history_drop_ok() {  # $1 = 将要写的**留档落点**（仓内相对路径）；0 = 合规 · 1 = 判红
  case "$1" in
    bin/_history/*) return 0 ;;
  esac
  echo "留档落点 ${1} 不在 bin/_history/ 之下（产出面根里平铺 *.bak-* ⇒ 判红）"
  return 1
}

hr; say "[闸②] 验签（取数档 = codesign -dvvv ⇒ Signature= / flags= / cdhash 三格同档；输出原样打印，不吞）"
if [ ! -e "$BIN_REL" ]; then
  judge "② 验签" BLOCKED "件不存在（${BIN_REL}）—— 判不了"
else
  say "      $(codesign -dvvv "$BIN_REL" 2>&1 | grep -E '^Identifier=|^CodeDirectory |^CandidateCDHash |^CDHash=' | tr '\n' ' ')"
  CELLS="$(sign_three_cells "$BIN_REL")"; CELLS_RC=$?
  say "      三格：${CELLS}"
  if [ "$CELLS_RC" = "1" ]; then
    judge "② 验签" FAIL "${BIN_REL} **判红**（条 10 · 取数档 -dvvv）：${CELLS}"
  elif [ "$CELLS_RC" != "0" ]; then
    judge "② 验签" BLOCKED "${BIN_REL} 三格取不到 ⇒ 判不了（不许当绿）：${CELLS}"
  elif codesign --verify --verbose=2 "$BIN_REL" 2>&1 | sed 's/^/      verify: /' | tee -a "$LOG"; then
    judge "② 验签" PASS "${BIN_REL} 验签通过（Signature / flags / cdhash 三格齐 · 非 adhoc）"
  else
    judge "② 验签" FAIL "${BIN_REL} 验签不过（签名失效的件换上会被秒杀 EXIT137）"
  fi
fi

hr; say "[闸③] 回滚位（回滚件必须**在位且验签通过** —— 「退得回去」得先证明）"
RB="bin/_history/$(basename "$BIN_REL").prev"
if [ ! -e "$RB" ]; then
  judge "③ 回滚位" BLOCKED "无回滚件（${RB}）—— 换上去就退不回来 ⇒ 不给结论（**不许当绿**）"
elif codesign --verify --verbose=1 "$RB" >/dev/null 2>&1; then
  judge "③ 回滚位" PASS "回滚件在位且验签通过（${RB}）"
else
  judge "③ 回滚位" FAIL "回滚件在位但**验签不过**（${RB}）—— 这正是 2026-09-16 二次事故的形态"
fi

hr; say "[闸④] 换件后自证判据可跑（只读探针 —— 判据活着才谈得上自证）"
READY_CODE="$(curl_tok -s -o /tmp/zerg-swap-agent-ready.json -w '%{http_code}' -m 4 "$READY_URL" 2>/dev/null || echo 000)"
STATUS_CODE="$(curl_tok -s -o /tmp/zerg-swap-agent-status.json -w '%{http_code}' -m 4 "$STATUS_URL" 2>/dev/null || echo 000)"
say "      /ready=$READY_CODE · /status=$STATUS_CODE"
if [ "$READY_CODE" = "200" ] && [ "$STATUS_CODE" = "200" ]; then
  MACHINE="$(python3 -c 'import json;print(json.load(open("/tmp/zerg-swap-agent-status.json")).get("machine",""))' 2>/dev/null || echo "")"
  judge "④ 自证判据" PASS "探针 200 · 子端自报 machine=${MACHINE}"
elif [ "$READY_CODE" = "000" ] && [ "$STATUS_CODE" = "000" ]; then
  judge "④ 自证判据" BLOCKED "两个探针都打不到（服务没在跑？）—— 判不了，不许当绿"
else
  judge "④ 自证判据" FAIL "探针异常（/ready=$READY_CODE · /status=${STATUS_CODE}；401 = 令牌不对）"
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

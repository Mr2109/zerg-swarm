#!/usr/bin/env bash
# test-update-sandbox.sh —— B3/B4 验收套件（**全在 /tmp 沙箱里跑，绝不碰真机**）
#
# 覆盖：
#   1. `zerg update --check` 无副作用（bin/ 与 state/ 除缓存外不变）
#   2. 开发态不误报（本地领先 ⇒ 报「本地领先」而非「可更新」）
#   3. 自替换安全（G2）：update 不自己换自己，**新进程**完成换装（pid 变、自报 sha 变）
#   4. 失败不半成品（非法 ref / 断网）⇒ 明确报错、bin/ 与状态未被破坏
#   5. B4：升级后矩阵同 sha；坏件 ⇒ 自动回滚且回执记失败路径；--from-assets 仍可用（假资产）
#   6. 隔离证明：整轮跑完，真机 bin/ 与 ~/.zerg/state/ 不变
#
# 用法：bash scripts/test-update-sandbox.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX="${ZERG_SANDBOX_DIR:-/tmp/zerg-update-sandbox}"
PORT="${ZERG_SANDBOX_PORT:-18580}"
KERNEL="$REPO_ROOT/scripts/zerg-upgrade.sh"
TOOLS="$(mktemp -d /tmp/zerg-sandbox-tools-XXXXXX)"

PASS=0; FAIL=0
ok()   { printf '  ✅ %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  ❌ %s\n' "$1"; FAIL=$((FAIL+1)); }
note() { printf '     %s\n' "$1"; }
hdr()  { printf '\n=== %s ===\n' "$1"; }
ck()   { if [ "$2" = "$3" ]; then ok "$1（$2）"; else bad "$1：期望 [$3] 实得 [$2]"; fi; }

gitq() { env GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
         GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"; }

# ── 真机现状快照（整轮跑完必须不变）──────────────────────────────────────────
REAL_BIN_SNAP="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_SNAP="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"

# ── 沙箱骨架 ─────────────────────────────────────────────────────────────────
# 清掉上一轮残留的沙箱运行体（否则端口被旧进程占着，测到的不是本轮进程）
pkill -f "$SANDBOX/bin/zerg-core" 2>/dev/null || true
sleep 0.5
rm -rf "$SANDBOX"; mkdir -p "$SANDBOX"
PUB="$SANDBOX/public.git"          # 「公开仓」（裸）
WORK="$SANDBOX/pubwork"            # 往公开仓推提交的工作区
DEV="$SANDBOX/localdev"            # zerg update 的本地检出（= 落后一笔）
PREFIX="$SANDBOX/bin"
STATE="$SANDBOX/state"; mkdir -p "$STATE"
# 预置状态文件（对齐真机 ~/.zerg/state 的既有形态）：core 里 statepath.MigrateIfNeeded 是
# **包级初始化**，目标不存在就会把真机 /tmp 的遗留文件搬进来——预置目标即可让它按"幂等跳过"走。
mkdir -p "$STATE/tool_events"
: > "$STATE/tool_uses.json"
: > "$STATE/tool_errors.json"
RECEIPTS="$SANDBOX/receipts"
LOG="$SANDBOX/logs"; mkdir -p "$LOG"

# 玩具「公开仓」内容：只有 core 模块，主控是个会自报 code_sha 的最小 HTTP 服务
toy_write() { # $1=目标树
  local d="$1"
  mkdir -p "$d/core/internal/version" "$d/core/cmd/zerg-core" "$d/core/cmd/zerg-agent"
  cat > "$d/core/go.mod" <<'EOF'
module github.com/Mr2109/zerg-swarm/core

go 1.25
EOF
  cat > "$d/core/internal/version/version.go" <<'EOF'
package version

const Version = "0.0.1"

const Tag = "v" + Version

var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

func Line(component string) string {
	return component + " " + Version + " " + Commit + " " + BuildTime
}
EOF
  cat > "$d/core/cmd/zerg-core/main.go" <<'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println(version.Line("zerg-core"))
		return
	}
	port := os.Getenv("ZERG_SANDBOX_PORT")
	if port == "" {
		port = "18580"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name": "zerg-sandbox", "version": version.Tag, "code_sha": version.Commit,
		})
	})
	fmt.Printf("[sandbox-core] pid=%d %s listening :%s\n", os.Getpid(), version.Line("zerg-core"), port)
	_ = http.ListenAndServe("127.0.0.1:"+port, mux)
}
EOF
  cat > "$d/core/cmd/zerg-agent/main.go" <<'EOF'
package main

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

func main() { fmt.Println(version.Line("zerg-agent")) }
EOF
}

toy_build() { # $1=源码树 $2=输出目录 $3=注入的 commit sha
  local src="$1" out="$2" sha="$3"
  mkdir -p "$out"
  ( cd "$src/core" && GOFLAGS=-mod=mod GOSUMDB=off go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$sha \
                -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=2026-09-13T00:00:00Z" \
      -o "$out/zerg-core" ./cmd/zerg-core )
  ( cd "$src/core" && GOFLAGS=-mod=mod GOSUMDB=off go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$sha \
                -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=2026-09-13T00:00:00Z" \
      -o "$out/zerg-agent" ./cmd/zerg-agent )
  [ -d "$out" ] || mkdir -p "$out"
  for b in zerg-core zerg-agent; do codesign -s - --force "$out/$b" >/dev/null 2>&1 || true; done
}

hdr "0. 备料：玩具公开仓（两笔提交）+ 从它克隆的本地检出"
mkdir -p "$WORK"; toy_write "$WORK"
gitq -C "$WORK" init -q -b main
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m "c1: initial"
C1="$(gitq -C "$WORK" rev-parse HEAD)"
echo "k2" > "$WORK/core/README.md"
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m "c2: second"
C2="$(gitq -C "$WORK" rev-parse HEAD)"
gitq -C "$SANDBOX" init -q --bare "$PUB"
gitq -C "$WORK" remote add origin "$PUB"
gitq -C "$WORK" push -q origin main
gitq -C "$SANDBOX" clone -q "$PUB" "$DEV"
gitq -C "$DEV" reset -q --hard "$C1"          # 本地落后公开仓一笔
note "公开仓 tip = ${C2:0:12}（new）"
note "本地检出 HEAD = ${C1:0:12}（old，落后一笔）"

# 沙箱里跑的「新」主控二进制（带 update 子命令）
note "构建带 update 子命令的主控 → $TOOLS/zerg-core-new"
( cd "$REPO_ROOT/core" && go build -o "$TOOLS/zerg-core-new" ./cmd/zerg-core )
UPD="$TOOLS/zerg-core-new"

# 旧的沙箱运行体（sha = C1）+ 假启停脚本（**只动沙箱，绝不碰真机 launchd/UI**）
toy_build "$DEV" "$PREFIX" "$C1"
cat > "$SANDBOX/start-core.sh" <<EOF
#!/usr/bin/env bash
PF="$SANDBOX/core.pid"
if [ "\${1:-}" = "--stop" ]; then
  [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
  rm -f "\$PF"; exit 0
fi
[ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
sleep 0.2
ZERG_SANDBOX_PORT="$PORT" nohup "$PREFIX/zerg-core" >> "$LOG/core.log" 2>&1 &
echo \$! > "\$PF"
EOF
chmod +x "$SANDBOX/start-core.sh"
cat > "$SANDBOX/start-ui.sh" <<'EOF'
#!/usr/bin/env bash
exit 0   # 沙箱 --no-ui，不会走到这里
EOF
chmod +x "$SANDBOX/start-ui.sh"

# 公共环境（全部指向沙箱）
export ZERG_UPDATE_REPO="$DEV"
export ZERG_UPDATE_REMOTE="$PUB"
export ZERG_UPDATE_REF="main"
export ZERG_PREFIX="$PREFIX"
export ZERG_STATE_DIR="$STATE"
export ZERG_RECEIPTS_DIR="$RECEIPTS"
export ZERG_UPGRADE_SCRIPT="$KERNEL"
export ZERG_UPGRADE_REPO="$DEV"
export ZERG_API_BASE="http://127.0.0.1:$PORT"
export ZERG_START_CORE="$SANDBOX/start-core.sh"
export ZERG_START_UI="$SANDBOX/start-ui.sh"
export ZERG_UI_PATTERN="zerg-sandbox-ui-never"
export ZERG_FLEET_YAML="$SANDBOX/no-fleet.yaml"
# 隔离：把临时根也指到沙箱——否则 core 里 `var toolUsesFile = statepath.MigrateIfNeeded("/tmp/…")`
# 这类**包级初始化**会把真机 /tmp 的遗留文件搬进沙箱状态目录（污染 --check 无副作用的判定）。
export ZERG_TMP_DIR="$SANDBOX/tmp"; mkdir -p "$SANDBOX/tmp"

bash "$SANDBOX/start-core.sh"
# 等旧运行体起来（最多 20s）
OLD_SHA=""
for i in $(seq 1 20); do
  sleep 1
  OLD_SHA="$(curl -s -m 3 "http://127.0.0.1:$PORT/api/capabilities" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("code_sha",""))' 2>/dev/null || echo '')"
  [ -n "$OLD_SHA" ] && break
done
OLD_PID="$(cat "$SANDBOX/core.pid")"
note "旧运行体 pid=$OLD_PID 自报 code_sha=${OLD_SHA:0:12}"
ck "旧运行体自报 = C1" "${OLD_SHA:0:12}" "${C1:0:12}"

# ══════════════════════════════════════════════════════════════════════════════
hdr "① --check 无副作用（bin/ 与 state/ 除缓存外 sha/mtime 不变）"
B_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
B_STATE="$(find "$STATE" -type f ! -name 'update_check.json' | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
OUT="$("$UPD" update --check --no-cache 2>&1)"; RC=$?
echo "$OUT" | sed 's/^/     /'
A_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
A_STATE="$(find "$STATE" -type f ! -name 'update_check.json' | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
ck "--check 退出码 0" "$RC" "0"
[ "$B_BIN" = "$A_BIN" ] && ok "bin/ 逐文件 sha 未变" || bad "bin/ 被改动（副作用！）"
[ "$B_STATE" = "$A_STATE" ] && ok "state/（除缓存）未变" || bad "state/ 被改动（副作用！）"

hdr "①b 6 小时缓存 + 不确定不缓存"
"$UPD" update --check >/dev/null 2>&1
[ -f "$STATE/update_check.json" ] && ok "确定的结论写入缓存 update_check.json" || bad "缓存未写入"
note "缓存内容：$(python3 -c 'import json;d=json.load(open("'"$STATE"'/update_check.json"));print({k:d[k] for k in ("status","behind","local_sha","remote_sha")})')"
BEFORE="$(shasum -a 256 "$STATE/update_check.json" | awk '{print $1}')"
"$UPD" update --check >/dev/null 2>&1
AFTER="$(shasum -a 256 "$STATE/update_check.json" | awk '{print $1}')"
# 缓存命中则不重写文件（同内容也算）；用 mtime 更能说明"没重复 fetch"
note "缓存文件 mtime：$(stat -f '%Sm' "$STATE/update_check.json")"
# 不确定不缓存：断网检查后缓存不应被更新为新结论
rm -f "$STATE/update_check.json"
ZERG_UPDATE_REMOTE="https://nonexistent.invalid/x.git" "$UPD" update --check --no-cache >/dev/null 2>&1 || true
[ ! -f "$STATE/update_check.json" ] && ok "fetch 失败 ⇒ 不写缓存（下次立即重试）" || bad "不确定的结论被缓存了"

# ══════════════════════════════════════════════════════════════════════════════
hdr "② 开发态不误报（本地领先 ⇒ 「本地领先」而非「可更新」）"
DEVA="$SANDBOX/dev-ahead"
gitq -C "$SANDBOX" clone -q "$PUB" "$DEVA"
echo "dev" > "$DEVA/core/DEVONLY.md"
gitq -C "$DEVA" add -A && gitq -C "$DEVA" commit -q -m "local-only（开发态）"
OUT2="$(ZERG_UPDATE_REPO="$DEVA" "$UPD" update --check --no-cache 2>&1)"
echo "$OUT2" | sed 's/^/     /'
case "$OUT2" in
  *"本地领先"*) ok "报了「本地领先（开发态）」" ;;
  *可更新*|*有新版*) bad "开发态被误报成「有新版可更新」" ;;
  *) bad "未识别开发态：$OUT2" ;;
esac

# ══════════════════════════════════════════════════════════════════════════════
hdr "③ 自替换安全（G2）：update 不自己换自己，新进程完成换装"
UPD_PID_BEFORE="$OLD_PID"
T0="$(date +%s)"
OUT3="$("$UPD" update --no-ui 2>&1)"; RC3=$?
echo "$OUT3" | sed 's/^/     /'
ck "update 退出码 0（已交接）" "$RC3" "0"
KERNEL_PID="$(printf '%s' "$OUT3" | grep -o '给独立升级进程（pid [0-9]*' | grep -o '[0-9]*$' | head -1)"
# 等内核干完（最多 60s）
for i in $(seq 1 60); do
  NEW_PID="$(cat "$SANDBOX/core.pid" 2>/dev/null || echo '')"
  if [ -n "$NEW_PID" ] && [ "$NEW_PID" != "$UPD_PID_BEFORE" ]; then
    if curl -s -m 2 "http://127.0.0.1:$PORT/api/capabilities" >/dev/null 2>&1; then break; fi
  fi
  sleep 1
done
sleep 1
NEW_PID="$(cat "$SANDBOX/core.pid" 2>/dev/null || echo '')"
NEW_SHA="$(curl -s -m 3 "http://127.0.0.1:$PORT/api/capabilities" | python3 -c 'import json,sys;print(json.load(sys.stdin)["code_sha"])' 2>/dev/null || echo '')"
note "旧 pid=$UPD_PID_BEFORE → 新 pid=${NEW_PID}；内核 pid=${KERNEL_PID}"
[ -n "$NEW_PID" ] && [ "$NEW_PID" != "$UPD_PID_BEFORE" ] && ok "主控 pid 已变化（新进程）" || bad "pid 未变（可能自己换自己）"
ck "运行进程自报 sha = 目标 C2" "${NEW_SHA:0:12}" "${C2:0:12}"
[ "$NEW_PID" != "$KERNEL_PID" ] && ok "换装者是内核独立进程，不是 update 自己" || bad "内核 pid == 换装后主控 pid（异常）"

hdr "③b 交接回执（update-launch，记录交给哪个独立进程）"
LR="$(ls -1t "$RECEIPTS"/update-launch-*.json 2>/dev/null | head -1)"
if [ -n "$LR" ]; then
  ok "交接回执已落盘：$(basename "$LR")"
  python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print("     kernel=",d["kernel"],"pid=",d["kernel_pid"],"target=",d["target"]["commit"][:12],"artifacts=",[a["name"] for a in d["artifacts"]])' "$LR"
else
  bad "没有交接回执"
fi

# ══════════════════════════════════════════════════════════════════════════════
hdr "④ 失败不半成品（非法 ref / 断网）⇒ 明确报错、bin/ 与状态未破坏"
S_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
OUT4="$(ZERG_UPDATE_REF="no-such-ref-xyz" "$UPD" update --no-ui 2>&1)"; RC4=$?
echo "$OUT4" | sed 's/^/     /'
ck "非法 ref ⇒ 退出码 1" "$RC4" "1"
A_BIN4="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
[ "$S_BIN" = "$A_BIN4" ] && ok "bin/ 未被破坏（非法 ref）" || bad "bin/ 被非法 ref 破坏了"
OUT4b="$(ZERG_UPDATE_REMOTE="https://nonexistent.invalid/x.git" "$UPD" update --no-ui 2>&1)"; RC4b=$?
echo "$OUT4b" | sed 's/^/     /'
ck "断网/不存在域名 ⇒ 退出码 1" "$RC4b" "1"
A_BIN4b="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
[ "$S_BIN" = "$A_BIN4b" ] && ok "bin/ 未被破坏（断网）" || bad "bin/ 被断网搞坏了"
# 非 git 安装拒绝
NOTGIT="$SANDBOX/notgit"; mkdir -p "$NOTGIT"
OUT4c="$(ZERG_UPDATE_REPO="$NOTGIT" "$UPD" update --no-ui 2>&1)"; RC4c=$?
echo "$OUT4c" | sed 's/^/     /'
ck "非 git 安装 ⇒ 退出码 3（拒绝）" "$RC4c" "3"
# fleet 印记拒绝
mkdir -p "$NOTGIT/.git"; printf 'fleet\n' > "$NOTGIT/.install_method"
OUT4d="$(ZERG_UPDATE_REPO="$NOTGIT" "$UPD" update --no-ui 2>&1)"; RC4d=$?
echo "$OUT4d" | sed 's/^/     /'
ck "fleet 印记 ⇒ 退出码 3（拒绝）" "$RC4d" "3"

# ══════════════════════════════════════════════════════════════════════════════
hdr "⑤ B4：升级后矩阵同 sha / 坏件回滚 / --from-assets 假资产"
# 矩阵同 sha：运行进程 + 落盘主控 + 落盘子端 三者一致且 = 目标
DISK_CORE="$("$PREFIX/zerg-core" --version | awk '{print $3}')"
DISK_AGENT="$("$PREFIX/zerg-agent" --version | awk '{print $3}')"
note "运行进程=${NEW_SHA:0:12} · 落盘主控=${DISK_CORE:0:12} · 落盘子端=${DISK_AGENT:0:12} · 目标=${C2:0:12}"
if [ "${NEW_SHA:0:12}" = "${DISK_CORE:0:12}" ] && [ "${DISK_CORE:0:12}" = "${DISK_AGENT:0:12}" ] && [ "${DISK_AGENT:0:12}" = "${C2:0:12}" ]; then
  ok "矩阵三处同 sha == 目标"
else
  bad "矩阵混版"
fi
grep -q "运行进程）自报 code_sha" "$RECEIPTS"/kernel-*.log 2>/dev/null && ok "内核 verify 走的是「运行进程自报」" || note "（内核日志未见 verify 行，见下）"
tail -6 "$(ls -1t "$RECEIPTS"/kernel-*.log | head -1)" | sed 's/^/     /'

# ── 坏件 A：sha 与 manifest 不符 ⇒ 校验即拒绝（exit 4）、**未动任何文件**、无回滚 ──
BADSHA="$SANDBOX/bad-sha"; mkdir -p "$BADSHA"
cp -p "$PREFIX/zerg-core" "$BADSHA/zerg-core-darwin-arm64"
cp -p "$PREFIX/zerg-agent" "$BADSHA/zerg-agent-darwin-arm64"
python3 - "$BADSHA" <<'PY'
import json, os, sys
d = sys.argv[1]
json.dump({"schema": 1, "version": "0.0.9", "tag": "v0.0.9",
           "commit": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
           "build_time": "2026-09-13T00:00:00Z", "platform": "darwin-arm64",
           "artifacts": [{"name": "zerg-core-darwin-arm64", "sha256": "0" * 64},
                         {"name": "zerg-agent-darwin-arm64", "sha256": "0" * 64}]},
          open(os.path.join(d, "manifest.json"), "w"), indent=2)
PY
SB_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
N_RC_BEFORE="$(ls -1 "$RECEIPTS"/2*.json 2>/dev/null | wc -l | tr -d ' ')"
ZERG_UPGRADE_SOURCE="file://$BADSHA" bash "$KERNEL" --prefix "$PREFIX" --no-ui > "$LOG/bad-sha.log" 2>&1; BADRC=$?
sed 's/^/     /' "$LOG/bad-sha.log"
ck "坏件(sha 不符) ⇒ 内核退出码 4（校验不过、未动文件）" "$BADRC" "4"
SA_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
[ "$SB_BIN" = "$SA_BIN" ] && ok "bin/ 逐文件 sha 未变（拒绝于换装前）" || bad "bin/ 被坏件改动"
N_RC_AFTER="$(ls -1 "$RECEIPTS"/2*.json 2>/dev/null | wc -l | tr -d ' ')"
[ "$N_RC_BEFORE" = "$N_RC_AFTER" ] && ok "未产生回执（校验不过=什么都没动，无需回滚）" || bad "校验失败却产生了回执"

# ── 坏件 B：sha **正确**但起不来 ⇒ 换装后 verify 失败 ⇒ 自动回滚 + 回执记失败路径 ──
BADRUN="$SANDBOX/bad-run"; mkdir -p "$BADRUN"
# 用"会立刻退出"的子端冒充主控：签名有效、sha 与 manifest 一致，但永远起不来
cp -p "$PREFIX/zerg-agent" "$BADRUN/zerg-core-darwin-arm64"
cp -p "$PREFIX/zerg-agent" "$BADRUN/zerg-agent-darwin-arm64"
python3 - "$BADRUN" <<'PY'
import hashlib, json, os, sys
d = sys.argv[1]
arts = [{"name": n, "sha256": hashlib.sha256(open(os.path.join(d, n), "rb").read()).hexdigest()}
        for n in ("zerg-core-darwin-arm64", "zerg-agent-darwin-arm64")]
json.dump({"schema": 1, "version": "0.0.9", "tag": "v0.0.9",
           "commit": "bad0000bad0000bad0000bad0000bad0000bad0",
           "build_time": "2026-09-13T00:00:00Z", "platform": "darwin-arm64", "artifacts": arts},
          open(os.path.join(d, "manifest.json"), "w"), indent=2)
PY
ZERG_UPGRADE_SOURCE="file://$BADRUN" bash "$KERNEL" --prefix "$PREFIX" --no-ui > "$LOG/bad-run.log" 2>&1; BRC_RC=$?
sed 's/^/     /' "$LOG/bad-run.log"
ck "坏件(起不来) ⇒ 内核退出码 1（已回滚）" "$BRC_RC" "1"
BRC="$(ls -1t "$RECEIPTS"/2*.json | head -1)"
python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print("     receipt: result=%s failed_at=%s"%(d["result"],d["failed_at"]))' "$BRC"
python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));sys.exit(0 if d["result"]=="rollback" and d["failed_at"]=="core-start" else 1)' "$BRC" \
  && ok "回执记失败路径（result=rollback, failed_at=core-start）" || bad "回执未记失败路径"
sleep 1
R_SHA="$(curl -s -m 3 "http://127.0.0.1:$PORT/api/capabilities" | python3 -c 'import json,sys;print(json.load(sys.stdin)["code_sha"])' 2>/dev/null || echo '')"
ck "坏件后服务恢复且仍是原好版本（.prev 回滚）" "${R_SHA:0:12}" "${C2:0:12}"

# ── --from-assets 仍可用（假 gh + 假资产）──
ASSETS="$SANDBOX/assets"; mkdir -p "$ASSETS"
cp -p "$PREFIX/zerg-core" "$ASSETS/zerg-core-darwin-arm64"
cp -p "$PREFIX/zerg-agent" "$ASSETS/zerg-agent-darwin-arm64"
python3 - "$ASSETS" "$C2" <<'PY'
import hashlib, json, os, sys
d, commit = sys.argv[1], sys.argv[2]
arts = []
for n in ("zerg-core-darwin-arm64", "zerg-agent-darwin-arm64"):
    h = hashlib.sha256(open(os.path.join(d, n), "rb").read()).hexdigest()
    arts.append({"name": n, "sha256": h})
json.dump({"schema": 1, "version": "0.0.1", "tag": "v0.0.1", "commit": commit,
           "build_time": "2026-09-13T00:00:00Z", "artifacts": arts},
          open(os.path.join(d, "manifest.json"), "w"), indent=2)
PY
mkdir -p "$SANDBOX/fakebin"
cat > "$SANDBOX/fakebin/gh" <<EOF
#!/usr/bin/env bash
# 假 gh：把 --pattern 的资产从 $ASSETS 拷到 --dir
args=("\$@"); dir=""; pat=""
while [ \$# -gt 0 ]; do case "\$1" in --dir) dir="\$2"; shift 2;; --pattern) pat="\$2"; shift 2;; *) shift;; esac; done
[ -n "\$dir" ] && [ -n "\$pat" ] && cp -p "$ASSETS/\$pat" "\$dir/\$pat"
EOF
chmod +x "$SANDBOX/fakebin/gh"
FA_BIN="$(find "$PREFIX" -type f -exec shasum -a 256 {} \; | sort | shasum -a 256 | awk '{print $1}')"
PATH="$SANDBOX/fakebin:$PATH" ZERG_UPGRADE_SOURCE="" ZERG_ASSETS_REPO="fake/repo" \
  bash "$KERNEL" --from-assets --prefix "$PREFIX" --no-ui > "$LOG/assets.log" 2>&1; FARC=$?
sed 's/^/     /' "$LOG/assets.log"
ck "--from-assets（假资产）⇒ 内核退出码 0" "$FARC" "0"
grep -q "取件源=assets" "$LOG/assets.log" && ok "确实走了资产通道" || bad "未走资产通道"

# ══════════════════════════════════════════════════════════════════════════════
hdr "⑥ 隔离证明：整轮跑完，真机 bin/ 与 ~/.zerg/state/ 不变"
REAL_BIN_AFTER="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_AFTER="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
[ "$REAL_BIN_SNAP" = "$REAL_BIN_AFTER" ] && ok "真机 bin/ 未变" || bad "真机 bin/ 被改动（越界！）"
[ "$REAL_STATE_SNAP" = "$REAL_STATE_AFTER" ] && ok "真机 ~/.zerg/state/ 未变" || bad "真机 state/ 被改动（越界！）"
ps -p 31764 >/dev/null 2>&1 && ok "真机主控 pid 31764 仍在跑（未被误杀）" || note "（真机主控 pid 31764 不在了——若你重启过则正常）"

printf '\n──────── 结果：%d 过 / %d 败 ────────\n' "$PASS" "$FAIL"
[ "$FAIL" = "0" ]

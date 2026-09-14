#!/usr/bin/env bash
# test-setup-sandbox.sh —— B6 验收套件（**全在 /tmp 沙箱 + 假命令环境里跑，绝不接真机**）
#
# 被测对象：scripts/setup-zerg.sh（新机器一条命令 bootstrap 源码式虫族）
#   取码 → 工具链核验 → 按角色构建（走仓库自己的 build-all.sh）→ 原子安装（.prev）
#   → 注册/重启服务（launchd/systemd）→ 读运行进程自报身份对账；支持 --dry-run 与 node 角色
#
# 覆盖（每条都能失败）：
#   1. 静态契约：setup 里没有手写 go build；它调用 scripts/build-all.sh；真仓 build-all.sh
#      确有身份注入（-ldflags）+ ad-hoc 重签（codesign）；新脚本引号/变量写法过门禁
#   2. --dry-run 零副作用（不 clone、不动 bin/、不写 plist/unit、不调 launchctl、不写印记）
#   3. 组件集：controller=core,agent(+ui on darwin)；node=core,agentd（**不含 ui**）
#   4. controller 全链：clone 公开仓 → 构建 → 原子安装（.prev）→ 写 launchd plist（键齐）→
#      bootstrap/kickstart → 运行进程自报 code_sha == 安装件 == 树 HEAD → 退出码 0 → 印记=git
#   5. 幂等二跑：已存在 ⇒ 重写 plist 并 kickstart -k（reload）；换装留 .prev；不报错
#   6. 负例 · 身份对账：活进程自报 ≠ 安装件 ⇒ 退出码 5（换了盘没换活进程能被抓）
#   7. 负例 · 脏树：已有检出有未提交改动 ⇒ 退出码 6 且不静默改动
#   8. 负例 · 工具链不合：钉住 go1.99.0 ⇒ 退出码 7 + 安装指引，未构建未安装
#   9. 负例 · 平台：非 darwin 请求 ui ⇒ 退出码 4；controller 在 linux ⇒ 退出码 4；
#      node 无 systemd 且未显式注入 ⇒ 退出码 4
#  10. node 全链（**真仓真构建**）：真检出 clone → 真 build-all.sh → 装 core+agentd（无 ui/agent）
#      → 写 systemd unit → 假 systemctl daemon-reload + enable/start → zerg-agentd --version 对账
#  11. 负例 · node 服务没起来（假 systemctl no-op）⇒ 退出码 5
#  12. 隔离证明：真仓 bin/ 与 ~/.zerg/state/ 整轮不变
#
# 用法：bash scripts/test-setup-sandbox.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX="${ZERG_SETUP_SANDBOX_DIR:-/tmp/zerg-setup-sandbox}"
PORT="${ZERG_SETUP_LOCAL_PORT:-18590}"
STALE_PORT="${ZERG_SETUP_STALE_PORT:-18591}"
SETUP="$REPO_ROOT/scripts/setup-zerg.sh"
TOOLS="$(mktemp -d /tmp/zerg-setup-tools-XXXXXX)"

PASS=0; FAIL=0
ok()   { printf '  ✅ %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  ❌ %s\n' "$1"; FAIL=$((FAIL+1)); }
note() { printf '     %s\n' "$1"; }
hdr()  { printf '\n=== %s ===\n' "$1"; }
ck()   { if [ "$2" = "$3" ]; then ok "$1（$2）"; else bad "$1：期望 [$3] 实得 [$2]"; fi; }
has()  { if grep -qF -- "$2" "$1" 2>/dev/null; then ok "$3"; else bad "$3（未在 $1 找到 $2）"; fi; }
matchp() { # 短/长 sha 混用即算同一提交（互为前缀）
  [ -n "$1" ] || return 1
  case "$1" in "$2"*) return 0 ;; esac
  case "$2" in "$1"*) return 0 ;; esac
  return 1
}

gitq() { env GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
         GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"; }
sha12() { printf '%s' "$1" | cut -c1-12; }

# ── 真机现状快照（整轮跑完必须不变）──────────────────────────────────────────
REAL_BIN_SNAP="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_SNAP="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"

# ── 沙箱骨架 ─────────────────────────────────────────────────────────────────
pkill -f "$SANDBOX" 2>/dev/null || true
sleep 0.5
rm -rf "$SANDBOX"; mkdir -p "$SANDBOX/fakebin" "$SANDBOX/units" "$SANDBOX/LaunchAgents" "$SANDBOX/logs"
PUB="$SANDBOX/public.git"                 # 「公开镜像仓」（裸）
WORK="$SANDBOX/pubwork"                   # 往公开仓推提交的工作区
LAUNCH_LOG="$SANDBOX/launchctl.log"; : > "$LAUNCH_LOG"
SYSCTL_LOG="$SANDBOX/systemctl.log"; : > "$SYSCTL_LOG"
export ZERG_FAKE_LAUNCHCTL_LOG="$LAUNCH_LOG"

# ══════════════════════════════════════════════════════════════════════════════
hdr "0. 备料：玩具公开仓（含 badpin 分支）+ 假 launchctl/systemctl"
# 玩具树：core 模块（HTTP 自报身份的主控 + 子端 CLI）+ scripts/build-all.sh（玩具版）+ 守护包装
toy_write() { # $1=树 $2=端口 $3=core/go.mod 的 go 指令（默认 1.21）
  local d="$1" port="$2" pin="${3:-1.21}"
  mkdir -p "$d/core/internal/version" "$d/core/cmd/zerg-core" "$d/core/cmd/zerg-agent" \
           "$d/agent/internal/version" "$d/agent/cmd/zerg-agentd" "$d/scripts"
  cat > "$d/.gitignore" <<'EOF'
/bin/
/.install_method
EOF
  cat > "$d/core/go.mod" <<EOF
module github.com/Mr2109/zerg-swarm/core

go $pin
EOF
  cat > "$d/agent/go.mod" <<EOF
module github.com/Mr2109/zerg-swarm/agent

go $pin
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
  cat > "$d/agent/internal/version/version.go" <<'EOF'
package version

var (
	Version   = "unknown"
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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Println(version.Line("zerg-core"))
			return
		}
	}
	port := os.Getenv("ZERG_SANDBOX_PORT")
	if port == "" {
		port = "18590"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name": "zerg-sandbox-core", "version": version.Tag, "code_sha": version.Commit,
		})
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": []interface{}{}})
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
  cat > "$d/agent/cmd/zerg-agentd/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/Mr2109/zerg-swarm/agent/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println(version.Line("zerg-agentd"))
		return
	}
	fmt.Printf("[sandbox-agentd] pid=%d %s\n", os.Getpid(), version.Line("zerg-agentd"))
}
EOF
  # 玩具 build-all.sh：模仿真仓入口的纪律——身份注入 -ldflags + ad-hoc 重签
  # ZERG_TOY_FORCE_SHA 是**沙箱夹具的负例接缝**：强制注入另一个 commit（模拟「构建身份 ≠ 树 HEAD」）
  cat > "$d/scripts/build-all.sh" <<'EOF'
#!/usr/bin/env bash
# 玩具 build-all.sh（沙箱夹具）：走与真仓同形的构建纪律（身份注入 + 重签 + build-info.json）
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ "${1:-}" = "--no-ui" ] || true
VERSION="$(grep -m1 '^const Version = ' "$ROOT/core/internal/version/version.go" | sed 's/.*"\(.*\)".*/\1/')"
SHA="${ZERG_TOY_FORCE_SHA:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BT="2026-09-13T00:00:00Z"
mkdir -p "$ROOT/bin"
LDF="-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$SHA -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=$BT"
for pkg in zerg-core zerg-agent; do
  ( cd "$ROOT/core" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
      -ldflags "$LDF" -o "$ROOT/bin/$pkg" "./cmd/$pkg" )
done
( cd "$ROOT/agent" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=$VERSION -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=$SHA -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=$BT" \
    -o "$ROOT/bin/zerg-agentd" ./cmd/zerg-agentd )
for b in zerg-core zerg-agent zerg-agentd; do codesign -s - --force "$ROOT/bin/$b" >/dev/null 2>&1 || true; done
printf '{"version": "%s", "commit": "%s", "build_time": "%s"}\n' "$VERSION" "$SHA" "$BT" > "$ROOT/bin/build-info.json"
EOF
  # 守护包装（plist 调它；负责源 .env 与补 PATH——与真仓 scripts/zerg-core-daemon.sh 同职责）
  cat > "$d/scripts/zerg-core-daemon.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")/.." && pwd)"
cd "\$REPO_ROOT"
if [ -f .env ]; then set -a; . ./.env; set +a; fi
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export ZERG_SANDBOX_PORT="$port"
exec "\$REPO_ROOT/bin/zerg-core"
EOF
  chmod +x "$d/scripts/build-all.sh" "$d/scripts/zerg-core-daemon.sh"
  echo 'k' > "$d/README.md"
}

mkdir -p "$WORK"; toy_write "$WORK" "$PORT"
gitq -C "$WORK" init -q -b main
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m 'c1: toy public tree'
C1="$(gitq -C "$WORK" rev-parse HEAD)"
# badpin 分支：工具链钉到 go1.99.0（负例 ⑧ 用）
gitq -C "$WORK" checkout -q -b badpin
toy_write "$WORK" "$PORT" 1.99.0
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m 'badpin: pin go1.99.0'
gitq -C "$WORK" checkout -q main
gitq -C "$SANDBOX" init -q --bare "$PUB"
gitq -C "$WORK" remote add origin "$PUB"
gitq -C "$WORK" push -q origin main
gitq -C "$WORK" push -q origin badpin
note "玩具公开仓 main=$(sha12 "$C1") · badpin=$(gitq -C "$WORK" rev-parse badpin | cut -c1-12)"

# 假 launchctl：bootstrap/kickstart/print/bootout 用 pidfile + 真起进程（沙箱专用）
cat > "$SANDBOX/fakebin/launchd_launch.py" <<'PY'
import os, plistlib, subprocess, sys
plist, pfdir = sys.argv[1], sys.argv[2]
d = plistlib.load(open(plist, "rb"))
label = str(d.get("Label") or "unknown")
open(os.path.join(pfdir, label + ".plist"), "w").write(plist)
args = list(d.get("ProgramArguments") or [])
stale = os.environ.get("ZERG_FAKE_LAUNCH_STALE_CORE")
if stale:
    args = [stale]                      # 负例接缝：模拟「换了盘、没换活进程」
cwd = d.get("WorkingDirectory") or None
env = dict(os.environ)
env.update({str(k): str(v) for k, v in (d.get("EnvironmentVariables") or {}).items()})
log = os.environ.get("ZERG_FAKE_LAUNCH_LOG", "/dev/null")
with open(log, "a") as lf:
    p = subprocess.Popen(args, cwd=cwd, env=env, stdout=lf, stderr=lf,
                         stdin=subprocess.DEVNULL, start_new_session=True)
open(os.path.join(pfdir, label + ".pid"), "w").write(str(p.pid))
PY
cat > "$SANDBOX/fakebin/launchctl" <<EOF
#!/usr/bin/env bash
# 假 launchctl（沙箱专用，绝不碰真机 launchd）
echo "launchctl \$*" >> "$LAUNCH_LOG"
PF_DIR="$SANDBOX/launchd-pids"; mkdir -p "\$PF_DIR"
cmd="\${1:-}"
case "\$cmd" in
  print)
    lab="\${2##*/}"; PF="\$PF_DIR/\$lab.pid"
    if [ -f "\$PF" ] && kill -0 "\$(cat "\$PF")" 2>/dev/null; then echo "\$lab = { pid = \$(cat "\$PF") }"; exit 0; fi
    exit 1 ;;
  bootout)
    lab="\${2##*/}"; PF="\$PF_DIR/\$lab.pid"
    [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
    rm -f "\$PF"; exit 0 ;;
  bootstrap)
    exec python3 "$SANDBOX/fakebin/launchd_launch.py" "\$3" "\$PF_DIR" ;;
  kickstart)
    lab="\${2##*/}"; [ "\${2:-}" = "-k" ] && lab="\${3##*/}"
    PF="\$PF_DIR/\$lab.pid"
    [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
    rm -f "\$PF"
    plist="\$(cat "\$PF_DIR/\$lab.plist" 2>/dev/null || true)"
    [ -n "\$plist" ] || exit 0
    exec python3 "$SANDBOX/fakebin/launchd_launch.py" "\$plist" "\$PF_DIR" ;;
  load)
    plist="\${2:-}"; [ "\${2:-}" = "-w" ] && plist="\${3:-}"
    exec python3 "$SANDBOX/fakebin/launchd_launch.py" "\$plist" "\$PF_DIR" ;;
esac
exit 0
EOF
# 假 systemctl（沙箱专用）：daemon-reload / is-active / enable --now / restart 用 pidfile 实现
cat > "$SANDBOX/fakebin/systemctl" <<EOF
#!/usr/bin/env bash
echo "systemctl \$*" >> "$SYSCTL_LOG"
PF_DIR="$SANDBOX/systemd-pids"; mkdir -p "\$PF_DIR"
cmd="\${1:-}"; shift || true
unit=""
for a in "\$@"; do case "\$a" in --*) ;; *) unit="\$a" ;; esac; done
case "\$cmd" in
  daemon-reload) exit 0 ;;
  is-active)
    PF="\$PF_DIR/\$unit.pid"
    if [ -f "\$PF" ] && kill -0 "\$(cat "\$PF")" 2>/dev/null; then echo active; exit 0; fi
    echo inactive; exit 3 ;;
  stop)
    PF="\$PF_DIR/\$unit.pid"; [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true; rm -f "\$PF"; exit 0 ;;
  enable|start|restart)
    PF="\$PF_DIR/\$unit.pid"; [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true; rm -f "\$PF"
    [ "\${ZERG_FAKE_SYSTEMCTL_NOOP:-0}" = 1 ] && exit 0
    BIN="\${ZERG_FAKE_UNIT_BIN:-}"
    if [ -n "\$BIN" ]; then nohup "\$BIN" >> "$SANDBOX/logs/\$unit.log" 2>&1 & echo \$! > "\$PF"; fi
    exit 0 ;;
esac
exit 0
EOF
cat > "$SANDBOX/sleep-service.sh" <<'EOF'
#!/bin/sh
exec sleep 300
EOF
chmod +x "$SANDBOX/fakebin/launchctl" "$SANDBOX/fakebin/systemctl" "$SANDBOX/sleep-service.sh"
note "假命令就绪：launchctl / systemctl（都只写沙箱 pidfile 与日志）"

# ══════════════════════════════════════════════════════════════════════════════
hdr "① 静态契约：构建必须走仓库自己的入口，绝不手写 go build"
# setup 里不得出现被执行的 go build（注释/文案不算）
GOBUILD_HITS="$(python3 - "$SETUP" <<'PY'
import re, sys
bad = []
for i, line in enumerate(open(sys.argv[1], encoding="utf-8"), 1):
    if line.lstrip().startswith("#"):
        continue
    # 真·命令形态的 go build，且该行不含中文（中文只在注释/文案里）
    if re.search(r"(^|[;&|(]|\s)go\s+build(\s|$)", line) and not re.search(r"[\u4e00-\u9fff]", line):
        bad.append("%d: %s" % (i, line.rstrip()))
print("\n".join(bad))
PY
)"
[ -z "$GOBUILD_HITS" ] && ok "setup-zerg.sh 无手写 go build（构建走仓库自己的入口）" || { bad "setup 里出现了手写 go build"; printf '%s\n' "$GOBUILD_HITS" | sed 's/^/     /'; }
has "$SETUP" 'scripts/build-all.sh' "setup 调用 scripts/build-all.sh"
has "$SETUP" 'bash scripts/build-all.sh' "setup 用 bash 调仓库构建入口"
# 真仓 build-all.sh 是身份注入 + ad-hoc 重签的单一入口（setup 依赖它，须确有这两样）
has "$REPO_ROOT/scripts/build-all.sh" '-ldflags' "真仓 build-all.sh 有身份注入（-ldflags）"
has "$REPO_ROOT/scripts/build-all.sh" 'codesign -s - --force' "真仓 build-all.sh 有 ad-hoc 重签（codesign）"
has "$REPO_ROOT/scripts/build-all.sh" 'internal/version.Commit=' "真仓 build-all.sh 注入 Commit 身份"
python3 "$REPO_ROOT/scripts/check-shell-unicode-vars.py" --check "$SETUP" >/tmp/zerg-setup-uv.txt 2>&1 \
  && ok "setup-zerg.sh 过 \$VAR/非 ASCII 门禁（check-shell-unicode-vars.py）" \
  || { bad "setup-zerg.sh 未过变量写法门禁"; sed 's/^/     /' /tmp/zerg-setup-uv.txt; }
grep -q 'declare -A' "$SETUP" && bad 'setup 用了 declare -A（本机 bash 3.2 不支持）' || ok 'setup 未用 declare -A（bash 3.2 兼容）'

# ══════════════════════════════════════════════════════════════════════════════
hdr "② 组件集：controller=core,agent(+ui darwin)；node=core,agentd（不含 ui）"
DRY_CTL="$(bash "$SETUP" --dir "$SANDBOX/dry/tree" --dry-run 2>&1)"
case "$DRY_CTL" in *'（组件 core,agent,ui）'*) ok 'controller + darwin ⇒ 含 ui' ;; *) bad "controller 组件集不对：$DRY_CTL" ;; esac
DRY_NOUI="$(bash "$SETUP" --dir "$SANDBOX/dry/tree" --dry-run --no-ui 2>&1)"
case "$DRY_NOUI" in *'（组件 core,agent）'*) ok 'controller --no-ui ⇒ core,agent' ;; *) bad "--no-ui 组件集不对" ;; esac
case "$DRY_NOUI" in *',ui'*) bad "(--no-ui 仍含 ui)" ;; *) ok 'controller --no-ui 不含 ui' ;; esac
DRY_NODE="$(bash "$SETUP" --dir "$SANDBOX/dry/tree" --role node --no-service --dry-run 2>&1)"
case "$DRY_NODE" in *'（组件 core,agentd）'*) ok 'node ⇒ core,agentd（精确集合）' ;; *) bad "node 组件集不对：$DRY_NODE" ;; esac
case "$DRY_NODE" in *',ui'*) bad 'node 组件集含 ui（违反 UI 仅 Mac）' ;; *) ok 'node 组件集不含 ui' ;; esac
case "$DRY_CTL" in *'git clone --depth 1 --branch main'*) ok 'dry-run 计划里有 clone 命令' ;; *) bad 'dry-run 未展示 clone' ;; esac
# --dry-run 零副作用：不建树、不动 bin/、不写 plist、不调 launchctl、不写印记
[ ! -e "$SANDBOX/dry" ] && ok '--dry-run 未创建任何目录' || bad '--dry-run 竟然建了目录'
[ ! -e "$SANDBOX/LaunchAgents/com.zerg.core.plist" ] && ok '--dry-run 未写 plist' || bad '--dry-run 写了 plist'
[ ! -s "$LAUNCH_LOG" ] && ok '--dry-run 零 launchctl 调用' || { bad '--dry-run 调了 launchctl'; sed 's/^/     /' "$LAUNCH_LOG"; }

# ══════════════════════════════════════════════════════════════════════════════
hdr "③ controller 全链：clone → 构建 → 原子安装 → launchd plist → 身份对账"
export ZERG_SETUP_LAUNCHCTL="$SANDBOX/fakebin/launchctl"
export ZERG_SETUP_LAUNCHD_DIR="$SANDBOX/LaunchAgents"
export ZERG_API_BASE="http://127.0.0.1:$PORT"
export ZERG_SETUP_WAIT_S=15
export ZERG_SANDBOX_PORT="$PORT"
export ZERG_TOKEN_FILE="$SANDBOX/no-token"   # 沙箱不用真机令牌
CTL_TREE="$SANDBOX/ctl/tree"; CTL_PREFIX="$SANDBOX/ctl/prefix"
: > "$LAUNCH_LOG"
OUT1="$(bash "$SETUP" --dir "$CTL_TREE" --repo "$PUB" --ref main --role controller --no-ui --prefix "$CTL_PREFIX" 2>&1)"; RC1=$?
echo "$OUT1" | sed 's/^/     /'
ck 'controller bootstrap 退出码 0' "$RC1" '0'
TREE_SHA="$(gitq -C "$CTL_TREE" rev-parse HEAD)"
ck 'clone 到位且树 HEAD == 公开仓 main' "$(sha12 "$TREE_SHA")" "$(sha12 "$C1")"
[ -x "$CTL_PREFIX/zerg-core" ] && ok '安装件 zerg-core 在位且可执行' || bad 'zerg-core 未装'
[ -x "$CTL_PREFIX/zerg-agent" ] && ok '安装件 zerg-agent 在位' || bad 'zerg-agent 未装'
[ ! -e "$CTL_PREFIX/zerg-ui" ] && ok 'controller --no-ui：未装 ui' || bad '竟装了 ui'
DISK_SHA="$("$CTL_PREFIX/zerg-core" --version 2>/dev/null | awk '{print $3}')"
if matchp "$DISK_SHA" "$TREE_SHA"; then ok "安装件自报 == 树 HEAD（$(sha12 "$DISK_SHA")）"; else bad "安装件自报 [$DISK_SHA] ≠ 树 HEAD [$(sha12 "$TREE_SHA")]"; fi
[ -f "$CTL_TREE/.install_method" ] && ck '写了 .install_method = git' "$(cat "$CTL_TREE/.install_method")" 'git' || bad '未写 .install_method'
PLIST="$SANDBOX/LaunchAgents/com.zerg.core.plist"
if [ -f "$PLIST" ]; then
  ok 'launchd plist 已生成'
  has "$PLIST" '<key>KeepAlive</key><true/>' 'plist KeepAlive=true'
  has "$PLIST" '<key>RunAtLoad</key><true/>' 'plist RunAtLoad=true'
  has "$PLIST" '<key>ThrottleInterval</key><integer>10</integer>' 'plist ThrottleInterval=10'
  has "$PLIST" 'zerg-core-daemon.sh' 'plist 经仓库守护包装（源 .env + 补 PATH）'
  has "$PLIST" '<key>PATH</key>' 'plist 有 PATH 处理'
  has "$PLIST" "<string>${CTL_TREE}</string>" 'plist WorkingDirectory = 树路径'
  plutil -lint "$PLIST" >/dev/null 2>&1 && ok 'plist 语法有效（plutil -lint）' || bad 'plist 语法无效'
else
  bad '未生成 launchd plist'
fi
grep -q 'launchctl bootstrap' "$LAUNCH_LOG" && ok 'launchctl bootstrap 被调用（未托管 ⇒ 安装）' || bad '未调用 bootstrap'
LIVE_SHA="$(curl -s -m 3 "http://127.0.0.1:$PORT/api/capabilities" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get("code_sha",""))' 2>/dev/null || echo '')"
if matchp "$LIVE_SHA" "$TREE_SHA"; then ok "运行进程自报 code_sha == 树 HEAD（$(sha12 "$LIVE_SHA")）"; else bad "运行进程自报 [$LIVE_SHA] ≠ 树 HEAD [$(sha12 "$TREE_SHA")]"; fi
case "$OUT1" in *'运行进程自报 code_sha='*) ok 'verify 走的是「运行进程自报」' ;; *) bad 'verify 未见运行进程自报' ;; esac

hdr "④ 幂等二跑：重写 plist + kickstart -k（reload）；换装留 .prev"
: > "$LAUNCH_LOG"
OUT2="$(bash "$SETUP" --dir "$CTL_TREE" --repo "$PUB" --ref main --role controller --no-ui --prefix "$CTL_PREFIX" 2>&1)"; RC2=$?
echo "$OUT2" | sed 's/^/     /'
ck '二跑退出码 0（幂等）' "$RC2" '0'
[ -f "$CTL_PREFIX/zerg-core.prev" ] && ok '二跑留了 .prev（原子换装纪律）' || bad '二跑没留 .prev'
grep -q 'kickstart -k' "$LAUNCH_LOG" && ok '二跑走 kickstart -k（已托管 ⇒ 重载）' || bad '二跑未走 kickstart'
case "$OUT2" in *'已在 main@'*) ok '取码幂等：已在目标提交，未改动工作树' ;; *) bad '取码非幂等提示缺失' ;; esac
LIVE2="$(curl -s -m 3 "http://127.0.0.1:$PORT/api/capabilities" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get("code_sha",""))' 2>/dev/null || echo '')"
if matchp "$LIVE2" "$TREE_SHA"; then ok "二跑后运行进程自报仍 == 树 HEAD（$(sha12 "$LIVE2")）"; else bad "二跑后运行进程自报 [$LIVE2] ≠ 树 HEAD [$(sha12 "$TREE_SHA")]"; fi

hdr "④b 取码幂等：已存在且落后 ⇒ fetch + reset --hard 对齐（明确报，不静默）"
printf 'k3\n' >> "$WORK/README.md"
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m 'c2: advance public tip'
gitq -C "$WORK" push -q origin main
C2B="$(gitq -C "$WORK" rev-parse main)"
OUT2B="$(bash "$SETUP" --dir "$CTL_TREE" --repo "$PUB" --ref main --role controller --no-ui --no-service --prefix "$CTL_PREFIX" 2>&1)"; RC2B=$?
echo "$OUT2B" | sed 's/^/     /'
ck '落后树取码 ⇒ 退出码 0' "$RC2B" '0'
case "$OUT2B" in *'对齐到'*) ok '明确报「对齐到 <ref>@<sha>」（不静默）' ;; *) bad '未报对齐动作' ;; esac
ck '树 HEAD 已对齐公开仓新 tip' "$(gitq -C "$CTL_TREE" rev-parse HEAD)" "$C2B"

hdr "⑤ 负例 · 身份对账：活进程自报 ≠ 安装件 ⇒ 退出码 5"
# 造一件「注入别的 commit」的真玩具件当活进程（模拟换了盘没换活进程）
"$SANDBOX/fakebin/launchctl" bootout "gui/$(id -u)/com.zerg.core" >/dev/null 2>&1 || true
sleep 0.5
# 用玩具树源码，注入一个不同的 commit → 活进程自报 ≠ 安装件（安装件是 C1）
STALE="${TOOLS}/stale-core"
( cd "$CTL_TREE/core" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=deadbeefdeadbeefdeadbeefdeadbeefdeadbeef -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=2026-09-13T00:00:00Z" \
    -o "$STALE" ./cmd/zerg-core ) && ok '造了「旧身份」活进程件（注入 deadbeef…）' || bad '造旧件失败'
chmod +x "$STALE"
export ZERG_FAKE_LAUNCH_STALE_CORE="$STALE"; export ZERG_SANDBOX_PORT="$PORT"
OUT5="$(bash "$SETUP" --dir "$CTL_TREE" --repo "$PUB" --ref main --role controller --no-ui --prefix "$CTL_PREFIX" 2>&1)"; RC5=$?
echo "$OUT5" | tail -6 | sed 's/^/     /'
ck '活进程自报 ≠ 安装件 ⇒ 退出码 5' "$RC5" '5'
case "$OUT5" in *'身份对账失败'*) ok '报错点明「身份对账失败」' ;; *) bad '未点明身份对账失败' ;; esac
unset ZERG_FAKE_LAUNCH_STALE_CORE

hdr "⑥ 负例 · 脏树：有未提交改动 ⇒ 退出码 6，不静默改动"
echo 'dirty' > "$CTL_TREE/DIRTY.txt"
OUT6="$(bash "$SETUP" --dir "$CTL_TREE" --repo "$PUB" --ref main --role controller --no-ui --prefix "$CTL_PREFIX" 2>&1)"; RC6=$?
echo "$OUT6" | sed 's/^/     /'
ck '脏树 ⇒ 退出码 6' "$RC6" '6'
[ -f "$CTL_TREE/DIRTY.txt" ] && ok '脏树原样保留（未静默清理/覆盖）' || bad '脏改动被弄没了'
rm -f "$CTL_TREE/DIRTY.txt"
# 目标存在但不是 git 检出 ⇒ 拒绝覆盖
PLAINDIR="$SANDBOX/plain"; mkdir -p "$PLAINDIR"; echo 'x' > "$PLAINDIR/existing-file"
OUT6b="$(bash "$SETUP" --dir "$PLAINDIR" --role controller --no-service 2>&1)"; RC6b=$?
ck '目标存在且非 git 检出 ⇒ 退出码 6（拒绝覆盖）' "$RC6b" '6'
[ -f "$PLAINDIR/existing-file" ] && ok '非检出目录原样保留' || bad '非检出目录被覆盖'

hdr "⑦ 负例 · 工具链不合：钉住 go1.99.0 ⇒ 退出码 7 + 安装指引，未构建未安装"
NEG_TREE="$SANDBOX/neg/tree"; NEG_PREFIX="$SANDBOX/neg/prefix"
OUT7="$(bash "$SETUP" --dir "$NEG_TREE" --repo "$PUB" --ref badpin --role controller --no-ui --prefix "$NEG_PREFIX" 2>&1)"; RC7=$?
echo "$OUT7" | sed 's/^/     /'
ck '工具链不合 ⇒ 退出码 7' "$RC7" '7'
case "$OUT7" in *'go1.99.0'*) ok '点明了钉住值' ;; *) bad '未点明钉住值' ;; esac
case "$OUT7" in *'brew install go'*|*'go.dev/dl'*) ok '给了安装指引（且绝不自行升级）' ;; *) bad '缺少安装指引' ;; esac
[ ! -e "$NEG_PREFIX/zerg-core" ] && ok '拒绝时未安装任何文件' || bad '拒绝时却装了文件'
[ ! -e "$NEG_TREE/bin/zerg-core" ] && ok '拒绝时未构建任何产物' || bad '拒绝时却构建了'

hdr "⑧ 负例 · 平台：ui 非 darwin ⇒ 4；controller 在 linux ⇒ 4；node 无 systemd ⇒ 4"
OUT8a="$(ZERG_SETUP_OS=linux bash "$SETUP" --dir "$SANDBOX/p8/a" --role node --no-service --components ui --dry-run 2>&1)"; RC8a=$?
echo "$OUT8a" | sed 's/^/     /'
ck 'linux 请求 ui ⇒ 退出码 4' "$RC8a" '4'
case "$OUT8a" in *'UI 只出 Mac'*) ok '点明「UI 只出 Mac」' ;; *) bad '未点明 UI 平台限制' ;; esac
OUT8b="$(ZERG_SETUP_OS=linux bash "$SETUP" --dir "$SANDBOX/p8/b" --role controller --dry-run 2>&1)"; RC8b=$?
ck 'controller 在 linux ⇒ 退出码 4（launchd 仅 macOS）' "$RC8b" '4'
[ ! -e "$SANDBOX/p8/a" ] && ok '拒绝分支零副作用（未建目录）' || bad '拒绝分支建了目录'
# node 无 systemd 且未显式注入：默认 darwin 主机 ⇒ 拒绝
env -u ZERG_SETUP_SYSTEMCTL bash "$SETUP" --dir "$SANDBOX/p8/c" --role node >/tmp/zerg-setup-p8c.txt 2>&1; RC8c=$?
echo "     $(cat /tmp/zerg-setup-p8c.txt | head -2)" >/dev/null
ck 'node 无 systemd（且未显式注入）⇒ 退出码 4' "$RC8c" '4'
[ ! -e "$SANDBOX/p8/c" ] && ok 'node 前置拒绝零副作用（未 clone）' || bad 'node 前置拒绝仍建了目录'

# ══════════════════════════════════════════════════════════════════════════════
hdr "⑨ node 全链（**真仓真构建**）：clone 真仓 → 真 build-all.sh → core+agentd → systemd unit"
export ZERG_SETUP_SYSTEMCTL="$SANDBOX/fakebin/systemctl"
export ZERG_FAKE_UNIT_BIN="$SANDBOX/sleep-service.sh"
export ZERG_SETUP_UNIT_DIR="$SANDBOX/units"
NODE_TREE="$SANDBOX/node/tree"; NODE_PREFIX="$SANDBOX/node/bin"
: > "$SYSCTL_LOG"
OUT9="$(bash "$SETUP" --dir "$NODE_TREE" --repo "$REPO_ROOT" --ref main --role node \
        --prefix "$NODE_PREFIX" --controller "http://127.0.0.1:$PORT" --unit-dir "$SANDBOX/units" 2>&1)"; RC9=$?
echo "$OUT9" | sed 's/^/     /'
ck 'node bootstrap（真仓）退出码 0' "$RC9" '0'
REAL_HEAD="$(gitq -C "$NODE_TREE" rev-parse HEAD)"
[ -f "$NODE_TREE/bin/build-info.json" ] && ok '真仓 build-all.sh 确有产出 build-info.json（真构建）' || bad '真构建产物缺失'
BI_COMMIT="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["commit"])' "$NODE_TREE/bin/build-info.json" 2>/dev/null || echo '')"
ck 'build-info.json 的 commit == 真仓 HEAD（构建来自这棵树）' "$BI_COMMIT" "$(printf '%s' "$REAL_HEAD" | cut -c1-8)"
[ -x "$NODE_PREFIX/zerg-core" ] && ok 'node 装了 core（该机将来可自更新）' || bad 'node 未装 core'
[ -x "$NODE_PREFIX/zerg-agentd" ] && ok 'node 装了 agentd（真在跑的那个）' || bad 'node 未装 agentd'
[ ! -e "$NODE_PREFIX/zerg-ui" ] && ok 'node 未装 ui' || bad 'node 装了 ui'
[ ! -e "$NODE_PREFIX/zerg-agent" ] && ok 'node 未装 agent（组件集 = core,agentd）' || bad 'node 竟装了 agent'
NODE_SHA="$("$NODE_PREFIX/zerg-agentd" --version 2>/dev/null | awk '{print $3}')"
ck 'zerg-agentd 自报 == 真仓 HEAD' "$NODE_SHA" "$(printf '%s' "$REAL_HEAD" | cut -c1-8)"
UNIT="$SANDBOX/units/zerg-agentd.service"
if [ -f "$UNIT" ]; then
  ok 'systemd unit 已生成'
  has "$UNIT" "ExecStart=${NODE_PREFIX}/zerg-agentd" 'unit ExecStart = 安装前缀的 zerg-agentd'
  has "$UNIT" 'Restart=always' 'unit Restart=always'
  has "$UNIT" "EnvironmentFile=-${NODE_TREE}/.env" 'unit 源仓库 .env'
  has "$UNIT" 'WantedBy=multi-user.target' 'unit 开机自启'
else
  bad '未生成 systemd unit'
fi
grep -q 'systemctl daemon-reload' "$SYSCTL_LOG" && ok 'systemctl daemon-reload 被调用' || bad '未调用 daemon-reload'
grep -qE 'systemctl (enable|start|restart)' "$SYSCTL_LOG" && ok 'systemctl enable/start 被调用（服务已注册）' || bad '未启动服务'
case "$OUT9" in *'服务 zerg-agentd = active'*) ok '服务 is-active 被核对（node 口径）' ;; *) bad '未见服务活性核对' ;; esac
case "$OUT9" in *'check-gotoolchain.py'*|*'两模块工具链钉住一致'*) ok '真仓上跑了 check-gotoolchain.py（单一真源一致性）' ;; *) bad '未跑工具链一致性门禁' ;; esac

hdr "⑩ 负例 · node 服务没起来（假 systemctl no-op）⇒ 退出码 5"
rm -f "$SANDBOX/systemd-pids/zerg-agentd.pid"
OUT10="$(ZERG_FAKE_SYSTEMCTL_NOOP=1 bash "$SETUP" --dir "$NODE_TREE" --repo "$REPO_ROOT" --ref main --role node \
        --prefix "$NODE_PREFIX" --controller "http://127.0.0.1:$PORT" --unit-dir "$SANDBOX/units" 2>&1)"; RC10=$?
echo "$OUT10" | tail -5 | sed 's/^/     /'
ck '服务未 active ⇒ 退出码 5（不假装完成）' "$RC10" '5'
case "$OUT10" in *'未 active'*) ok '报错点明服务未 active' ;; *) bad '未点明服务未起' ;; esac

hdr "⑪ node 负例 · 安装件身份对账：zerg-agentd 自报 ≠ 树 HEAD ⇒ 退出码 5"
TOYNODE_TREE="$SANDBOX/toynode/tree"; TOYNODE_PREFIX="$SANDBOX/toynode/bin"
OUT11="$(ZERG_TOY_FORCE_SHA='deadbeefdeadbeefdeadbeefdeadbeefdeadbeef' bash "$SETUP" --dir "$TOYNODE_TREE" \
         --repo "$PUB" --ref main --role node --no-service --prefix "$TOYNODE_PREFIX" 2>&1)"; RC11=$?
echo "$OUT11" | tail -6 | sed 's/^/     /'
ck 'node 安装件自报 ≠ 树 HEAD ⇒ 退出码 5' "$RC11" '5'
case "$OUT11" in *'身份对账失败'*) ok '报错点明「身份对账失败」（node 口径）' ;; *) bad '未点明身份对账失败' ;; esac
# 反证：同一玩具 node 树、不强制身份 ⇒ 退出码 0（证明上面的红确由身份不一致引起）
OUT11b="$(bash "$SETUP" --dir "$TOYNODE_TREE" --repo "$PUB" --ref main --role node --no-service --prefix "$TOYNODE_PREFIX" 2>&1)"; RC11b=$?
ck '同一树不强制身份 ⇒ 退出码 0（反证红由身份不一致引起）' "$RC11b" '0'
[ -f "$TOYNODE_PREFIX/zerg-agentd.prev" ] && ok 'node 换装也留 .prev（原子换装纪律）' || bad 'node 换装未留 .prev'

# ══════════════════════════════════════════════════════════════════════════════
hdr "⑫ 隔离证明：真仓 bin/ 与 ~/.zerg/state/ 整轮不变；假命令只写沙箱"
"$SANDBOX/fakebin/launchctl" bootout "gui/$(id -u)/com.zerg.core" >/dev/null 2>&1 || true
REAL_BIN_AFTER="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_AFTER="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
[ "$REAL_BIN_SNAP" = "$REAL_BIN_AFTER" ] && ok '真仓 bin/ 未变' || bad '真仓 bin/ 被改动（越界！）'
[ "$REAL_STATE_SNAP" = "$REAL_STATE_AFTER" ] && ok '真仓 ~/.zerg/state/ 未变' || bad '真仓 state/ 被改动（越界！）'
UNKNOWN_UNITS="$(grep -vE 'daemon-reload|zerg-agentd' "$SYSCTL_LOG" || true)"
[ -z "$UNKNOWN_UNITS" ] && ok '假 systemctl 只碰沙箱 unit（daemon-reload / zerg-agentd）' || { bad "假 systemctl 触及了别的 unit"; printf '%s\n' "$UNKNOWN_UNITS" | sed 's/^/     /'; }
[ ! -e "$HOME/Library/LaunchAgents/com.zerg.core.plist.new" ] && ok '未在真机 LaunchAgents 留下半成品' || bad '真机 LaunchAgents 被写'

printf '\n──────── 结果：%d 过 / %d 败 ────────\n' "$PASS" "$FAIL"
[ "$FAIL" = "0" ]

#!/usr/bin/env bash
# test-fleet-sandbox.sh —— B5 验收套件（**全在 /tmp 沙箱 + 假二机环境里跑，绝不接真机**）
#
# 被测对象：scripts/zerg-upgrade.sh 的 --fleet（B5 改造：每台各自跑源码式更新）
#   本机   ：走本机的 `zerg update`（源码式）
#   远程机 ：ssh 上去让**它自己** fetch→本机构建→交**它本地的**六阶段内核换装
#   主控   ：只编排 + 回收回执 + 核对 /api/fleet/status 的 code_sha（三台一致且 = 目标）
#
# 覆盖（每条都能失败）：
#   1. `--fleet --plan` 无副作用（各机 bin/ 与 state/ 不变、无新回执、不执行任何更新）
#   2. 名册：ZERG_FLEET_NODES（env）与 gateway/fleet.yaml 的 update.nodes 两条来源同解
#   3. 每台各自源码式更新：各机有自己的内核日志与回执；落盘件自报 == 目标；
#      本机不再交叉编译/不再 scp 推预编译制品（ssh 调用里零 scp、零真 IP）
#   4. 版本矩阵收敛：内核退出码 0，三台 code_sha 一致且 = 目标（回执 matrix_check.converged）
#   5. 每台有回执：机群回执逐台记录 + 该机 receipts 里能回读到内核回执
#   6. 负例 A：某台 agentd 未真正换版（运行身份仍旧）⇒ 矩阵核对**必须**点名并判失败
#   7. 负例 B：某台没有检出/没有 CLI ⇒ pending-manual + 打印 bootstrap 命令 + 判失败
#   8. G6：工具链钉住（manifest 记 pin/actual/policy；低于钉住值 ⇒ 拒绝构建；
#      同 commit+同平台+同构建时间 ⇒ 同 sha256；三台回执里的 pin 与构建时间戳一致）
#   9. UI 仅 Mac：非 darwin 平台请求 ui 组件 ⇒ 拒绝；节点路径的组件集不含 ui；
#      真·linux/amd64 agentd（真 agent 模块交叉编译）能在 PLAT=linux-amd64 下换装并起服务（假 systemctl）
#  10. 隔离证明：真机 bin/ 与 ~/.zerg/state/ 不变；假 ssh 只连沙箱名册目标
#
# 用法：bash scripts/test-fleet-sandbox.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX="${ZERG_FLEET_SANDBOX_DIR:-/tmp/zerg-fleet-sandbox}"
LOCAL_PORT="${ZERG_FLEET_LOCAL_PORT:-18581}"
TOOLS="$(mktemp -d /tmp/zerg-fleet-tools-XXXXXX)"
KERNEL="$REPO_ROOT/scripts/zerg-upgrade.sh"

PASS=0; FAIL=0
ok()   { printf '  ✅ %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  ❌ %s\n' "$1"; FAIL=$((FAIL+1)); }
note() { printf '     %s\n' "$1"; }
hdr()  { printf '\n=== %s ===\n' "$1"; }
ck()   { if [ "$2" = "$3" ]; then ok "$1（$2）"; else bad "$1：期望 [$3] 实得 [$2]"; fi; }

gitq() { env GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
         GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"; }
sha12() { printf '%s' "$1" | cut -c1-12; }

# ── 真机现状快照（整轮跑完必须不变）──────────────────────────────────────────
REAL_BIN_SNAP="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_SNAP="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"

# ── 沙箱骨架 ─────────────────────────────────────────────────────────────────
pkill -f "$SANDBOX/local/bin/zerg-core" 2>/dev/null || true
pkill -f "$SANDBOX/x3/bin/zerg-agentd" 2>/dev/null || true
pkill -f "$SANDBOX/mini1/bin/zerg-agentd" 2>/dev/null || true
sleep 0.5
rm -rf "$SANDBOX"; mkdir -p "$SANDBOX/fakebin"
PUB="$SANDBOX/public.git"       # 「公开仓」（裸）
WORK="$SANDBOX/pubwork"         # 往公开仓推提交的工作区
SSHLOG="$SANDBOX/ssh-calls.log"; : > "$SSHLOG"

# ══ 0. 备料：玩具「公开仓」三件（core 模块 + agent 模块 + 内核脚本）+ 真 update 二进制 ══
toy_write() { # $1=目标树
  local d="$1"
  mkdir -p "$d/core/internal/version" "$d/core/cmd/zerg-core" "$d/core/cmd/zerg-agent" \
           "$d/agent/internal/version" "$d/agent/cmd/zerg-agentd" "$d/scripts"
  cat > "$d/core/go.mod" <<'EOF'
module github.com/Mr2109/zerg-swarm/core

go 1.21

toolchain go1.25.5
EOF
  cat > "$d/agent/go.mod" <<'EOF'
module github.com/Mr2109/zerg-swarm/agent

go 1.21

toolchain go1.25.5
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
	"os/exec"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

type hb struct {
	CodeVersion string `json:"code_version"`
	CodeSHA     string `json:"code_sha"`
	Healthy     bool   `json:"healthy"`
	At          int64  `json:"at"`
}

var (
	mu   sync.Mutex
	seen = map[string]hb{}
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Println(version.Line("zerg-core"))
			return
		case "update":
			// 沙箱 shim：玩具 core 里没有真的 update 实现 ⇒ 转交真二进制（B5 沙箱专用接缝）。
			// 这样"每台各自跑源码式更新"走的是**真的** Go selfupdate + **真的**内核脚本，
			// 只有"这台机器上跑什么"是假的。
			bin := os.Getenv("ZERG_TOY_UPDATE_BIN")
			if bin == "" {
				fmt.Fprintln(os.Stderr, "sandbox-shim: 未设 ZERG_TOY_UPDATE_BIN")
				os.Exit(1)
			}
			c := exec.Command(bin, append([]string{"update"}, os.Args[2:]...)...)
			c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
			if err := c.Run(); err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					os.Exit(ee.ExitCode())
				}
				os.Exit(1)
			}
			return
		}
	}
	port := os.Getenv("ZERG_SANDBOX_PORT")
	if port == "" {
		port = "18581"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"name": "zerg-sandbox-core", "version": version.Tag, "code_sha": version.Commit})
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"tasks": []interface{}{}})
	})
	mux.HandleFunc("/api/fleet/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Machine     string `json:"machine"`
			CodeVersion string `json:"code_version"`
			CodeSHA     string `json:"code_sha"`
			Healthy     bool   `json:"healthy"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Machine != "" {
			mu.Lock()
			seen[req.Machine] = hb{CodeVersion: req.CodeVersion, CodeSHA: req.CodeSHA, Healthy: req.Healthy, At: time.Now().Unix()}
			mu.Unlock()
		}
		writeJSON(w, map[string]interface{}{"ok": true})
	})
	mux.HandleFunc("/api/fleet/status", func(w http.ResponseWriter, r *http.Request) {
		machines := map[string]hb{"local": {CodeVersion: version.Version, CodeSHA: version.Commit, Healthy: true}}
		mu.Lock()
		for k, v := range seen {
			machines[k] = v
		}
		mu.Unlock()
		writeJSON(w, map[string]interface{}{"total_machines": len(machines), "machines": machines})
	})
	fmt.Printf("[sandbox-core] pid=%d %s listening :%s\n", os.Getpid(), version.Line("zerg-core"), port)
	_ = http.ListenAndServe("127.0.0.1:"+port, mux)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println(version.Line("zerg-agentd"))
		return
	}
	name := os.Getenv("ZERG_NODE_NAME")
	if name == "" {
		name = "unknown"
	}
	ctl := os.Getenv("ZERG_CONTROLLER")
	if pf := os.Getenv("ZERG_AGENTD_PIDFILE"); pf != "" {
		_ = os.WriteFile(pf, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
	}
	fmt.Printf("[sandbox-agentd] pid=%d machine=%s %s controller=%s\n", os.Getpid(), name, version.Line("zerg-agentd"), ctl)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	if ctl == "" {
		<-ch
		return
	}
	body, _ := json.Marshal(map[string]interface{}{
		"machine": name, "code_version": version.Version, "code_sha": version.Commit,
		"healthy": true, "backend_state": "idle",
	})
	go func() {
		for {
			if resp, err := http.Post(ctl+"/api/fleet/heartbeat", "application/json", bytes.NewReader(body)); err == nil {
				resp.Body.Close()
			}
			time.Sleep(time.Second)
		}
	}()
	<-ch
}
EOF
  # 内核脚本用**软链**指向工作树：保证测的是当前工作树的内核，而不是快照
  ln -sf "$KERNEL" "$d/scripts/zerg-upgrade.sh"
  echo "k1" > "$d/README.md"
}

toy_build() { # $1=源码树 $2=输出目录 $3=注入 commit
  local src="$1" out="$2" sha="$3"
  mkdir -p "$out"
  local bt="2026-09-13T00:00:00Z"
  ( cd "$src/core" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$sha \
                -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=$bt" \
      -o "$out/zerg-core" ./cmd/zerg-core ) || return 1
  ( cd "$src/core" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=$sha \
                -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=$bt" \
      -o "$out/zerg-agent" ./cmd/zerg-agent ) || return 1
  ( cd "$src/agent" && GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=0.0.1 \
                -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=$sha \
                -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=$bt" \
      -o "$out/zerg-agentd" ./cmd/zerg-agentd ) || return 1
  for b in zerg-core zerg-agent zerg-agentd; do codesign -s - --force "$out/$b" >/dev/null 2>&1 || true; done
}

hdr "0. 备料：玩具公开仓（两笔：c1 旧 / c2 目标）+ 三台假机 + 假 ssh"
mkdir -p "$WORK"; toy_write "$WORK"
gitq -C "$WORK" init -q -b main
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m "c1: initial"
C1="$(gitq -C "$WORK" rev-parse HEAD)"
echo "k2" > "$WORK/README.md"
gitq -C "$WORK" add -A && gitq -C "$WORK" commit -q -m "c2: target"
C2="$(gitq -C "$WORK" rev-parse HEAD)"
gitq -C "$SANDBOX" init -q --bare "$PUB"
gitq -C "$WORK" remote add origin "$PUB"
gitq -C "$WORK" push -q origin main
note "公开仓 tip = $(sha12 "$C2")（目标）· c1 = $(sha12 "$C1")（各机起点，落后一笔）"

note "构建带 update 子命令的真主控 → $TOOLS/zerg-core-new（沙箱 shim 的目标）"
( cd "$REPO_ROOT/core" && go build -o "$TOOLS/zerg-core-new" ./cmd/zerg-core ) || { bad "真主控构建失败（套件无法继续）"; exit 1; }

# 机器工厂：三台（本机 local / 远程 x3 / 远程 mini1）+ 一台裸机（裸 = 无检出无 CLI）
machine_make() { # $1=名 $2=端口 $3=角色(node|controller)
  local m="$1" port="$2" role="$3"
  mkdir -p "$SANDBOX/$m"/{bin,state,receipts,logs,tmp}
  mkdir -p "$SANDBOX/$m/state/tool_events"
  : > "$SANDBOX/$m/state/tool_uses.json"; : > "$SANDBOX/$m/state/tool_errors.json"
  gitq -C "$SANDBOX" clone -q "$PUB" "$SANDBOX/$m/repo"
  gitq -C "$SANDBOX/$m/repo" reset -q --hard "$C1"
  toy_build "$SANDBOX/$m/repo" "$SANDBOX/$m/bin" "$C1" || bad "$m 旧件构建失败"
  cat > "$SANDBOX/$m/start-ui.sh" <<'EOF'
#!/usr/bin/env bash
exit 0   # 本套件 --no-ui
EOF
  cat > "$SANDBOX/$m/start-core.sh" <<EOF
#!/usr/bin/env bash
NODE="$SANDBOX/$m"; PF="\$NODE/core.pid"
if [ "\${1:-}" = "--stop" ]; then
  [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
  rm -f "\$PF"; exit 0
fi
[ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
sleep 0.2
ZERG_SANDBOX_PORT=$port nohup "\$NODE/bin/zerg-core" >> "\$NODE/logs/core.log" 2>&1 &
echo \$! > "\$PF"
EOF
  cat > "$SANDBOX/$m/agentd-ctl.sh" <<EOF
#!/usr/bin/env bash
# 节点上的 agentd「systemd」替身：stop/restart/is-active 用 pidfile + 进程真起停。
NODE="$SANDBOX/$m"; PF="\$NODE/agentd.pid"
set -a; . "\$NODE/node.env"; set +a
case "\${1:-}" in
  stop)
    [ "\${ZERG_AGENTD_NOOP:-0}" = "1" ] && exit 0   # 负例：假装停成功（进程仍跑旧件）
    [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
    rm -f "\$PF"; exit 0 ;;
  is-active)
    [ -f "\$PF" ] && kill -0 "\$(cat "\$PF")" 2>/dev/null && exit 0 || exit 1 ;;
  restart|start)
    [ "\${ZERG_AGENTD_NOOP:-0}" = "1" ] && exit 0   # 负例：假装重启成功（运行身份仍旧）
    [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
    echo \$! > /dev/null
    ZERG_NODE_NAME=$m ZERG_CONTROLLER=http://127.0.0.1:$LOCAL_PORT ZERG_AGENTD_PIDFILE="\$PF" \\
      nohup "\$NODE/bin/zerg-agentd" >> "\$NODE/logs/agentd.log" 2>&1 &
    echo \$! > "\$PF"
    sleep 0.3; exit 0 ;;
esac
exit 0
EOF
  chmod +x "$SANDBOX/$m"/*.sh
  cat > "$SANDBOX/$m/node.env" <<EOF
ZERG_UPDATE_REPO=$SANDBOX/$m/repo
ZERG_UPDATE_REMOTE=$PUB
ZERG_UPDATE_REF=main
ZERG_PREFIX=$SANDBOX/$m/bin
ZERG_API_BASE=http://127.0.0.1:$port
ZERG_STATE_DIR=$SANDBOX/$m/state
ZERG_RECEIPTS_DIR=$SANDBOX/$m/receipts
ZERG_UPGRADE_SCRIPT=$SANDBOX/$m/repo/scripts/zerg-upgrade.sh
ZERG_UPGRADE_REPO=$SANDBOX/$m/repo
ZERG_TMP_DIR=$SANDBOX/$m/tmp
ZERG_TOY_UPDATE_BIN=$TOOLS/zerg-core-new
ZERG_SANDBOX_PORT=$port
ZERG_START_CORE=$SANDBOX/$m/start-core.sh
ZERG_START_UI=$SANDBOX/$m/start-ui.sh
ZERG_UI_PATTERN=zerg-ui-never-$m
ZERG_AGENTD_UNIT=$m-agent
ZERG_AGENTD_PIDFILE=$SANDBOX/$m/agentd.pid
ZERG_NODE_NAME=$m
ZERG_CONTROLLER=http://127.0.0.1:$LOCAL_PORT
PATH=$SANDBOX/fakebin:$PATH
EOF
  if [ "$role" = "node" ]; then
    printf 'ZERG_START_AGENTD=%s/%s/agentd-ctl.sh\n' "$SANDBOX" "$m" >> "$SANDBOX/$m/node.env"
  fi
}

machine_make local "$LOCAL_PORT" controller
machine_make x3 "18582" node
machine_make mini1 "18583" node
mkdir -p "$SANDBOX/bare"
# 裸机：有 SSH 入口、没有检出/没有 CLI（负例 B）
cat > "$SANDBOX/bare/node.env" <<EOF
ZERG_PREFIX=$SANDBOX/bare/bin
PATH=$SANDBOX/fakebin:$PATH
EOF

# 假 ssh：把「机器名」路由到沙箱目录里的 in-node.sh；**任何未知名册目标一律拒绝**（禁接真机）
cat > "$SANDBOX/fakebin/ssh" <<EOF
#!/usr/bin/env bash
args=()
while [ \$# -gt 0 ]; do
  case "\$1" in
    -o|-p|-i) shift 2 ;;
    -*) shift ;;
    *) args+=("\$1"); shift ;;
  esac
done
[ \${#args[@]} -ge 2 ] || { echo "fake-ssh: 参数不足" >&2; exit 64; }
tgt="\${args[0]}"; cmd="\${args[1]}"
echo "ssh \$tgt :: \$cmd" >> "$SSHLOG"
case "\$tgt" in
  x3|root@x3|admin@x3) node=x3 ;;
  mini1|root@mini1) node=mini1 ;;
  bare|root@bare) node=bare ;;
  *) echo "fake-ssh: 拒绝连接未知名册目标 '\$tgt'（沙箱禁接真机）" >&2; exit 99 ;;
esac
exec "$SANDBOX/\$node/in-node.sh" "\$cmd"
EOF
for m in local x3 mini1 bare; do
  cat > "$SANDBOX/$m/in-node.sh" <<EOF
#!/usr/bin/env bash
# 「登录到那台机器」：装载该机环境后执行一条命令
NODE="$SANDBOX/$m"
set -a; . "\$NODE/node.env"; set +a
cd "\$NODE" 2>/dev/null || true
exec /bin/sh -c "\$1"
EOF
  chmod +x "$SANDBOX/$m/in-node.sh"
done
chmod +x "$SANDBOX/fakebin/ssh"

# 假 systemctl（linux 路径演练；默认代码路径就叫 systemctl，故不注入 ZERG_START_AGENTD）
cat > "$SANDBOX/fakebin/systemctl" <<EOF
#!/usr/bin/env bash
# 假 systemctl：用 pidfile 实现 stop/restart/is-active（沙箱专用，绝不碰真机服务）
unit="\${2:-unknown}"
PF="$SANDBOX/systemd-\$unit.pid"
case "\${1:-}" in
  stop) [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true; rm -f "\$PF"; exit 0 ;;
  is-active) [ -f "\$PF" ] && kill -0 "\$(cat "\$PF")" 2>/dev/null && { echo active; exit 0; }; exit 3 ;;
  restart|start|daemon-reload|enable)
    if [ "\${1:-}" != "restart" ] && [ "\${1:-}" != "start" ]; then exit 0; fi
    [ -f "\$PF" ] && kill "\$(cat "\$PF")" 2>/dev/null || true
    nohup env ZERG_NODE_NAME=linuxnode ZERG_CONTROLLER=http://127.0.0.1:1 "\$ZERG_FAKE_UNIT_BIN" >> "$SANDBOX/logs-linux-agentd.log" 2>&1 &
    echo \$! > "\$PF"; sleep 0.3; exit 0 ;;
esac
exit 0
EOF
chmod +x "$SANDBOX/fakebin/systemctl"

# 本机（controller）的环境：直接装载它自己的 node.env（与真机"登录本机跑 --fleet"同形），
# 再叠加机群专属变量（名册 / 假 ssh / 等待上限）
set -a; . "$SANDBOX/local/node.env"; set +a
export ZERG_FLEET_YAML="$SANDBOX/fleet-test.yaml"
export ZERG_FLEET_SSH="$SANDBOX/fakebin/ssh"
export ZERG_FLEET_NODES="x3=x3,root=$SANDBOX/x3/repo,prefix=$SANDBOX/x3/bin,api=http://127.0.0.1:18582,receipts=$SANDBOX/x3/receipts mini1=mini1,root=$SANDBOX/mini1/repo,prefix=$SANDBOX/mini1/bin,api=http://127.0.0.1:18583,receipts=$SANDBOX/mini1/receipts"
export ZERG_FLEET_WAIT_S=45
export PATH="$SANDBOX/fakebin:$PATH"

# 假 fleet.yaml（名册的第二来源；suite 会让 --fleet 用它跑一次，验证两条来源同解）
cat > "$ZERG_FLEET_YAML" <<EOF
# 沙箱假 fleet.yaml（私有配置文件形态）
updates:
  check: true
update:
  nodes:
    - { name: x3, ssh: x3, root: $SANDBOX/x3/repo, prefix: $SANDBOX/x3/bin, api: "http://127.0.0.1:18582", receipts: "$SANDBOX/x3/receipts", components: "core,agentd" }
    - { name: mini1, ssh: mini1, root: $SANDBOX/mini1/repo, prefix: $SANDBOX/mini1/bin, api: "http://127.0.0.1:18583", receipts: "$SANDBOX/mini1/receipts", components: "core,agentd" }
fleet:
  x3: { host: 192.0.2.10, port: 8100, os: ubuntu }   # RFC 5737 文档专用段（夹具，非真机）
EOF

# 起「三台机器」的运行体：本机主控 + x3/mini1 的 agentd（迷你节点）
bash "$SANDBOX/local/start-core.sh" >/dev/null 2>&1
for m in x3 mini1; do bash "$SANDBOX/$m/agentd-ctl.sh" restart >/dev/null 2>&1; done
sleep 1
OLD_SHA=""
for i in $(seq 1 20); do
  OLD_SHA="$(curl -s -m 3 "http://127.0.0.1:$LOCAL_PORT/api/capabilities" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("code_sha",""))' 2>/dev/null || true)"
  [ -n "$OLD_SHA" ] && break
  sleep 1
done
ck "本机旧运行体自报 = c1" "$(sha12 "$OLD_SHA")" "$(sha12 "$C1")"
MATRIX0="$(curl -s -m 3 "http://127.0.0.1:$LOCAL_PORT/api/fleet/status" 2>/dev/null)"
printf '%s' "$MATRIX0" | python3 -c 'import json,sys
d=json.load(sys.stdin)["machines"]
for k,v in sorted(d.items()): print("     %s code_sha=%s" % (k, (v or {}).get("code_sha","")[:12]))'
note "起点：三台都在 c1（落后公开仓一笔）"

# ══════════════════════════════════════════════════════════════════════════════
hdr "① --fleet --plan 无副作用（各机 bin//state/ 不变、无新回执、不执行更新）"
snap_machine() { find "$1" -type f -exec shasum -a 256 {} \; 2>/dev/null | sort | shasum -a 256 | awk '{print $1}'; }
B_BIN_LOCAL="$(snap_machine "$SANDBOX/local/bin")"; B_BIN_X3="$(snap_machine "$SANDBOX/x3/bin")"
B_RC_X3="$(ls -1 "$SANDBOX/x3/receipts" 2>/dev/null | wc -l | tr -d ' ')"
OUT_PLAN="$(bash "$KERNEL" --fleet --plan 2>&1)"; RC_PLAN=$?
echo "$OUT_PLAN" | sed 's/^/     /'
ck "--fleet --plan 退出码 0" "$RC_PLAN" "0"
case "$OUT_PLAN" in *"每台各自源码式更新"*) ok "计划里明说「每台各自源码式更新」" ;; *) bad "计划说明缺失" ;; esac
case "$OUT_PLAN" in *"远程 → 本机"*) ok "顺序：远程先、本机最后" ;; *) bad "顺序未声明" ;; esac
a="$(snap_machine "$SANDBOX/local/bin")"; [ "$a" = "$B_BIN_LOCAL" ] && ok "本机 bin/ 未变" || bad "本机 bin/ 被 --plan 改动"
a="$(snap_machine "$SANDBOX/x3/bin")"; [ "$a" = "$B_BIN_X3" ] && ok "x3 bin/ 未变" || bad "x3 bin/ 被 --plan 改动"
a="$(ls -1 "$SANDBOX/x3/receipts" 2>/dev/null | wc -l | tr -d ' ')"; [ "$a" = "$B_RC_X3" ] && ok "x3 未产生回执（没执行更新）" || bad "--plan 产生了回执"
[ ! -s "$SSHLOG" ] && ok "假 ssh 零调用（--plan 不去连任何机器）" || { bad "--plan 竟然 ssh 了"; sed 's/^/     /' "$SSHLOG"; }

hdr "② 名册两条来源同解（ZERG_FLEET_NODES / gateway/fleet.yaml 的 update.nodes）"
OUT_YAML="$(ZERG_FLEET_NODES="" bash "$KERNEL" --fleet --plan 2>&1)"
echo "$OUT_YAML" | sed 's/^/     /'
case "$OUT_YAML" in *"x3（远程 · node x3）"*) ok "从 fleet.yaml 解出 x3" ;; *) bad "fleet.yaml 名册未解出 x3" ;; esac
case "$OUT_YAML" in *"mini1（远程 · node mini1）"*) ok "从 fleet.yaml 解出 mini1" ;; *) bad "fleet.yaml 名册未解出 mini1" ;; esac
OUT_ENV="$(ZERG_FLEET_YAML="$SANDBOX/no-such.yaml" bash "$KERNEL" --fleet --plan 2>&1)"
case "$OUT_ENV" in *"x3（远程 · node x3）"*) ok "env 名册优先（缺 yaml 也解出 x3）" ;; *) bad "env 名册未生效" ;; esac

hdr "③ --fleet 执行：每台各自跑源码式更新（本机 zerg update / 远程 ssh 自更新）"
: > "$SSHLOG"
OUT_APPLY="$(bash "$KERNEL" --fleet --no-ui 2>&1)"; RC_APPLY=$?
echo "$OUT_APPLY" | sed 's/^/     /'
ck "--fleet 退出码 0（三台收敛）" "$RC_APPLY" "0"
# 每台都有自己的内核日志 + 内核回执（证据：各机 receipts 目录里）
for m in local x3 mini1; do
  n_kernel="$(ls -1 "$SANDBOX/$m/receipts"/kernel-*.log 2>/dev/null | wc -l | tr -d ' ')"
  [ "$n_kernel" -ge 1 ] && ok "$m 有自己的内核日志（$n_kernel 份）" || bad "$m 没有内核日志"
  n_rc="$(ls -1 "$SANDBOX/$m/receipts"/2*.json 2>/dev/null | wc -l | tr -d ' ')"
  [ "$n_rc" -ge 1 ] && ok "$m 有内核回执（$n_rc 份）" || bad "$m 没有内核回执"
done
# 每台落盘件自报 == 目标
LOCAL_V="$("$SANDBOX/local/bin/zerg-core" --version 2>/dev/null | awk '{print $3}')"
X3_V="$("$SANDBOX/x3/bin/zerg-agentd" --version 2>/dev/null | awk '{print $3}')"
M1_V="$("$SANDBOX/mini1/bin/zerg-agentd" --version 2>/dev/null | awk '{print $3}')"
note "落盘自报：local=$(sha12 "$LOCAL_V") · x3=$(sha12 "$X3_V") · mini1=$(sha12 "$M1_V") · 目标=$(sha12 "$C2")"
ck "本机落盘 zerg-core == 目标" "$(sha12 "$LOCAL_V")" "$(sha12 "$C2")"
ck "x3 落盘 zerg-agentd == 目标" "$(sha12 "$X3_V")" "$(sha12 "$C2")"
ck "mini1 落盘 zerg-agentd == 目标" "$(sha12 "$M1_V")" "$(sha12 "$C2")"
# 主控不再推送预编译制品：ssh 里零 scp、零交叉编译、零真 IP
grep -q "scp" "$SSHLOG" && bad "ssh 调用里出现 scp（又回去推制品了！）" || ok "ssh 调用零 scp（不再推预编译制品）"
grep -qE "10\.0\.0\.2|192\.168\.110\.83" "$SSHLOG" && bad "ssh 调用出现真机 IP（禁接真机被破坏！）" || ok "ssh 调用零真机 IP"
grep -q "GOOS=linux" "$SSHLOG" && bad "远程命令里出现交叉编译（应由该机自编）" || ok "远程命令零交叉编译（该机自编）"
note "ssh 调用摘要："
awk '{print "       " $1, $2, substr($0, index($0,"::")+3, 60)}' "$SSHLOG" | head -8

hdr "④ 版本矩阵核对：三台 code_sha 一致且 = 目标"
MATRIX1="$(curl -s -m 3 "http://127.0.0.1:$LOCAL_PORT/api/fleet/status")"
printf '%s' "$MATRIX1" | python3 -c 'import json,sys
d=json.load(sys.stdin)["machines"]
for k,v in sorted(d.items()): print("     %s code_sha=%s" % (k, (v or {}).get("code_sha","")[:12]))'
MATRIX_OK="$(printf '%s' "$MATRIX1" | python3 -c 'import json,sys
t=sys.argv[1]; d=json.load(sys.stdin)["machines"]
want=["local","x3","mini1"]
shas={k:(d.get(k) or {}).get("code_sha","") for k in want}
bad=[k for k in want if not (shas[k] and (shas[k].startswith(t) or t.startswith(shas[k])))]
print("ok" if not bad else "bad:"+",".join(bad))' "$C2")"
ck "矩阵三台 code_sha 一致且 = 目标" "$MATRIX_OK" "ok"
FRC="$(ls -1t "$SANDBOX/local/receipts"/fleet-*.json 2>/dev/null | head -1)"
FRC_MAIN="$FRC"   # ③ 这一轮的回执（同一轮内才谈得上"三台同戳"）
[ -n "$FRC" ] && ok "机群回执已落盘：$(basename "$FRC")" || bad "没有机群回执"
python3 - "$FRC" "$C2" <<'PY'
import json, sys
d = json.load(open(sys.argv[1])); t = sys.argv[2]
print("     mode=%s converged=%s order=%s" % (d.get("mode"), d["matrix_check"]["converged"], d.get("order")))
nodes = d["nodes"]
print("     逐台：" + ", ".join("%s=%s(%s)" % (n["name"], n["code_sha_after"][:8] or "-", n["status"]) for n in nodes))
assert d.get("mode") == "per-machine-source-update", "mode 必须标「每台各自源码式更新」"
assert d["matrix_check"]["converged"] is True, "matrix_check.converged 应为 true"
assert all(n["converged"] for n in nodes), "每台都应 converged"
assert all(n["receipt_target"].startswith(t) or t.startswith(n["receipt_target"]) for n in nodes if n["receipt_target"]), "每台的交接回执 target 应等于目标"
PY
[ $? = 0 ] && ok "机群回执：每台 converged + 回执 target = 目标" || bad "机群回执内容不符"

hdr "⑤ 每台有回执（机群汇总 + 该机自己的一份）"
for m in local x3 mini1; do
  LR="$(ls -1t "$SANDBOX/$m/receipts"/update-launch-*.json 2>/dev/null | head -1)"
  if [ -n "$LR" ]; then
    got="$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(d["target"]["commit"])' "$LR")"
    role="$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(d.get("role"))' "$LR")"
    comps="$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(",".join(d.get("components") or []))' "$LR")"
    ck "$m 交接回执 target = 目标" "$(sha12 "$got")" "$(sha12 "$C2")"
    note "$m role=$role components=$comps"
  else
    bad "$m 没有交接回执"
  fi
done

hdr "⑥ 负例 A：某台 agentd 未真正换版 ⇒ 矩阵核对必须点名并判失败（混版能被抓）"
# 真造混版现场：先把 mini1 的**运行进程**换成 .prev（旧件，c1 身份），
# 再让它的 restart/stop 变 no-op ⇒ 换完盘后活进程仍是旧件——这正是"换了盘、没换活进程"。
[ -f "$SANDBOX/mini1/bin/zerg-agentd.prev" ] && (
  kill "$(cat "$SANDBOX/mini1/agentd.pid" 2>/dev/null)" 2>/dev/null || true
  sleep 0.4
  set -a; . "$SANDBOX/mini1/node.env"; set +a
  ZERG_NODE_NAME=mini1 ZERG_CONTROLLER="http://127.0.0.1:$LOCAL_PORT" ZERG_AGENTD_PIDFILE="$SANDBOX/mini1/agentd.pid" \
    nohup "$SANDBOX/mini1/bin/zerg-agentd.prev" >> "$SANDBOX/mini1/logs/agentd.log" 2>&1 &
  echo $! > "$SANDBOX/mini1/agentd.pid"
  sleep 1.2
  OLD_RUN="$(curl -s -m 3 "http://127.0.0.1:$LOCAL_PORT/api/fleet/status" | python3 -c 'import json,sys;print((json.load(sys.stdin)["machines"].get("mini1") or {}).get("code_sha",""))' 2>/dev/null || true)"
  note "混版前置：mini1 运行进程自报 $(sha12 "$OLD_RUN")（应为 c1）"
)
echo "ZERG_AGENTD_NOOP=1" >> "$SANDBOX/mini1/node.env"
: > "$SSHLOG"
OUT_MIX="$(bash "$KERNEL" --fleet --no-ui 2>&1)"; RC_MIX=$?
echo "$OUT_MIX" | tail -14 | sed 's/^/     /'
ck "混版（mini1 运行身份仍旧）⇒ 退出码 1" "$RC_MIX" "1"
case "$OUT_MIX" in *"❌ mini1"*) ok "点名了没收敛的那台（mini1）" ;; *) bad "未点名混版机器" ;; esac
# 复原：去掉 NOOP 并让它真的重启到新件
python3 - "$SANDBOX/mini1/node.env" <<'PY'
import sys
p = sys.argv[1]
open(p, "w", encoding="utf-8").write("".join(l for l in open(p, encoding="utf-8") if "ZERG_AGENTD_NOOP" not in l))
PY
bash "$SANDBOX/mini1/agentd-ctl.sh" restart >/dev/null 2>&1

hdr "⑦ 负例 B：某台没有检出/没有 CLI ⇒ pending-manual + 打印 bootstrap + 判失败"
OUT_BARE="$(ZERG_FLEET_NODES="x3=x3,root=$SANDBOX/x3/repo,prefix=$SANDBOX/x3/bin,receipts=$SANDBOX/x3/receipts bare=bare" bash "$KERNEL" --fleet --no-ui 2>&1)"; RC_BARE=$?
echo "$OUT_BARE" | tail -12 | sed 's/^/     /'
ck "有台不可自更新 ⇒ 退出码 1（不假装完成）" "$RC_BARE" "1"
case "$OUT_BARE" in *"pending-manual"*) ok "如实标 pending-manual" ;; *) bad "未标 pending-manual" ;; esac
case "$OUT_BARE" in *"git clone --depth 1"*) ok "打印了该机的 bootstrap 待执行命令" ;; *) bad "未打印 bootstrap 命令" ;; esac
FRC_BARE="$(ls -1t "$SANDBOX/local/receipts"/fleet-*.json | head -1)"
python3 -c 'import json,sys
d=json.load(open(sys.argv[1])); print("     nodes:", [(n["name"], n["status"]) for n in d["nodes"]])' "$FRC_BARE"

hdr "⑧ G6：Go 工具链钉住（manifest 留痕 / 低于钉住值即拒 / 同输入同 sha256 / 各机同戳）"
# pin 的取法：core/go.mod 的 toolchain（若显式）否则 go 指令——与 core/internal/selfupdate/toolchain.go 同口径
PIN_REAL="$(python3 - "$REPO_ROOT/core/go.mod" <<'PY'
import re, sys
txt = open(sys.argv[1], encoding="utf-8").read()
m = re.search(r"^toolchain\s+(go[\d.]+)", txt, re.M) or re.search(r"^go\s+([\d.]+)", txt, re.M)
v = m.group(1)
print(v if v.startswith("go") else "go" + v)
PY
)"
note "真仓 core/go.mod 的钉住值 = ${PIN_REAL}"
PINS="$(python3 - "$FRC_MAIN" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
for n in d["nodes"]:
    tc = (n.get("toolchain") or "-|-|-").split("|")
    tc += ["-"] * (3 - len(tc))
    print("|".join([n["name"], tc[0], tc[1], tc[2], n.get("build_time") or "-"]))
PY
)"
echo "$PINS" | sed 's/^/     /'
n_pin="$(echo "$PINS" | awk -F'|' '{print $2}' | sort -u | wc -l | tr -d ' ')"
n_bt="$(echo "$PINS" | awk -F'|' '{print $5}' | sort -u | wc -l | tr -d ' ')"
PIN_SEEN="$(echo "$PINS" | awk -F'|' '{print $2}' | sort -u | head -1)"
[ "$n_pin" = "1" ] && ok "三台留痕的工具链钉住值一致（pin=${PIN_SEEN}）" || bad "三台 pin 不一致"
[ "$PIN_SEEN" = "$PIN_REAL" ] && ok "机群钉住值 == 真仓 pin（${PIN_REAL}）" || bad "机群 pin($PIN_SEEN) ≠ 真仓 pin($PIN_REAL)"
[ "$n_bt" = "1" ] && ok "三台共享同一构建时间戳（G6 可逐字节比对）" || bad "三台构建时间戳不同"
POL="$(echo "$PINS" | awk -F'|' '{print $4}' | sort -u | tr '\n' ' ')"
[ "$POL" = "local " ] && ok "构建策略 = GOTOOLCHAIN=local（不在构建中途换工具链）" || bad "构建策略异常：[$POL]" 
# 低于钉住值 ⇒ 拒绝构建（能失败）
BIN_SNAP_X3="$(snap_machine "$SANDBOX/x3/bin")"
OUT_PIN="$(ZERG_GO_TOOLCHAIN_PIN=go99.0.0 ZERG_UPDATE_ROLE=node ZERG_UPDATE_REPO="$SANDBOX/x3/repo" \
  ZERG_PREFIX="$SANDBOX/x3/bin" ZERG_STATE_DIR="$SANDBOX/x3/state" ZERG_RECEIPTS_DIR="$SANDBOX/x3/receipts" \
  "$TOOLS/zerg-core-new" update --role node --no-ui --to main 2>&1)"; RC_PIN=$?
echo "$OUT_PIN" | head -4 | sed 's/^/     /'
ck "钉住值高于本机 ⇒ 退出码 1（拒绝构建）" "$RC_PIN" "1"
case "$OUT_PIN" in *"工具链"*) ok "明确报了工具链不满足" ;; *) bad "错误信息未提工具链" ;; esac
a="$(snap_machine "$SANDBOX/x3/bin")"; [ "$a" = "$BIN_SNAP_X3" ] && ok "拒绝时未动任何文件" || bad "拒绝时改了 bin/"
# 同 commit + 同平台 + 同构建时间 ⇒ 同 sha256（真模块，真断言）
cat > "$TOOLS/det.sh" <<EOF
cd "$REPO_ROOT/agent"
GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local go build -trimpath -buildvcs=false \\
  -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=0.0.1 \\
            -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=$C2 \\
            -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=2026-09-13T00:00:00Z" \\
  -o "\$1" ./cmd/zerg-agentd
EOF
bash "$TOOLS/det.sh" "$TOOLS/det1" && bash "$TOOLS/det.sh" "$TOOLS/det2"
D1="$(shasum -a 256 "$TOOLS/det1" | awk '{print $1}')"; D2="$(shasum -a 256 "$TOOLS/det2" | awk '{print $1}')"
[ -n "$D1" ] && [ "$D1" = "$D2" ] && ok "同 commit+同平台+同构建时间 ⇒ 同 sha256（${D1:0:12}…）" || bad "两次构建 sha256 不同（可复现性断言失败）"

hdr "⑨ UI 仅 Mac：非 darwin 拒绝 ui；节点组件集不含 ui；linux/amd64 节点路径真演练"
# 9a 非 darwin + ui ⇒ 拒绝
STAGE_LINUX="$SANDBOX/stage-linux"; mkdir -p "$STAGE_LINUX"
OUT_UI="$(ZERG_UPGRADE_PLAT=linux-amd64 ZERG_UPGRADE_SOURCE="file://$STAGE_LINUX" bash "$KERNEL" --components ui,core --no-service 2>&1)"; RC_UI=$?
echo "$OUT_UI" | sed 's/^/     /'
ck "linux 平台请求 ui ⇒ 退出码 4（拒绝，不动文件）" "$RC_UI" "4"
case "$OUT_UI" in *"UI 仅 Mac"*) ok "报错点明「UI 仅 Mac」" ;; *) bad "报错未点明原因" ;; esac
# 9b 真 agent 模块交叉编译 linux/amd64 → 内核在 PLAT=linux-amd64 + 假 systemctl 下换装并起服务
LINSHA="1111111111111111111111111111111111111111"
( cd "$REPO_ROOT/agent" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local \
    go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=0.0.1 \
              -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=$LINSHA \
              -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=2026-09-13T00:00:00Z" \
    -o "$STAGE_LINUX/zerg-agentd-linux-amd64" ./cmd/zerg-agentd ) && ok "真 agent 模块交叉编译 linux/amd64 成功" || bad "交叉编译失败"
python3 - "$STAGE_LINUX" "$LINSHA" <<'PY'
import hashlib, json, os, sys
d, sha = sys.argv[1], sys.argv[2]
n = "zerg-agentd-linux-amd64"
h = hashlib.sha256(open(os.path.join(d, n), "rb").read()).hexdigest()
json.dump({"schema": 1, "version": "0.0.1", "tag": "v0.0.1", "commit": sha,
           "build_time": "2026-09-13T00:00:00Z", "platform": "linux-amd64",
           "components": ["agentd"],
           "toolchain": {"pin": "go1.25.5", "actual": "local", "policy": "local"},
           "artifacts": [{"name": n, "sha256": h}]},
          open(os.path.join(d, "manifest.json"), "w"), indent=2)
PY
mkdir -p "$SANDBOX/linbin" "$SANDBOX/linbin-linux"
# 「服务进程」替身：linux 二进制在 macOS 上跑不起来（exec format error），
# 故默认 systemctl 路径用一个常驻 sleep 当服务进程——**演练的是内核的编排**（停→换→启→is-active），
# 而落盘的制品就是那份真 linux 制品（字节与清单逐字节相同，已单独断言）。
cat > "$SANDBOX/sleep-service.sh" <<'EOF'
#!/bin/sh
exec sleep 300
EOF
chmod +x "$SANDBOX/sleep-service.sh"
# 9b：linux/amd64 只换装（--no-service）⇒ 制品字节必须与清单**逐字节相同**（linux 不做 macOS ad-hoc 重签）
OUT_LIN="$(ZERG_UPGRADE_PLAT=linux-amd64 ZERG_UPGRADE_SOURCE="file://$STAGE_LINUX" ZERG_PREFIX="$SANDBOX/linbin-linux" \
  ZERG_UPGRADE_REPO="$SANDBOX/local/repo" ZERG_STATE_DIR="$SANDBOX/local/state" \
  ZERG_RECEIPTS_DIR="$SANDBOX/linreceipts" ZERG_API_BASE="http://127.0.0.1:$LOCAL_PORT" \
  bash "$KERNEL" --role node --components agentd --no-service 2>&1)"; RC_LIN=$?
echo "$OUT_LIN" | sed 's/^/     /'
ck "linux/amd64 节点路径（只换装）⇒ 退出码 0" "$RC_LIN" "0"
case "$OUT_LIN" in *"UI"*) bad "linux 路径竟然碰了 UI" ;; *) ok "linux 路径不碰 UI（UI 仅 Mac）" ;; esac
case "$OUT_LIN" in *"重签名"*) bad "linux 路径不该走 macOS 重签名" ;; *) ok "linux 路径不走 macOS 签名" ;; esac
STAGED_SHA="$(shasum -a 256 "$STAGE_LINUX/zerg-agentd-linux-amd64" | awk '{print $1}')"
INSTALLED_SHA="$(shasum -a 256 "$SANDBOX/linbin-linux/zerg-agentd" 2>/dev/null | awk '{print $1}')"
ck "落盘件与清单逐字节相同（linux 不做重签）" "$INSTALLED_SHA" "$STAGED_SHA"
# 9c：darwin 节点角色（--role node --components agentd）+ 假 systemctl ⇒ 真·起服务 + 真·verify（落盘自报）
STAGE_NODE="$SANDBOX/stage-node"; mkdir -p "$STAGE_NODE"
cp -p "$SANDBOX/x3/bin/zerg-agentd" "$STAGE_NODE/zerg-agentd-darwin-arm64"
python3 - "$STAGE_NODE" "$C2" <<'PY'
import hashlib, json, os, sys
d, sha = sys.argv[1], sys.argv[2]
n = "zerg-agentd-darwin-arm64"
h = hashlib.sha256(open(os.path.join(d, n), "rb").read()).hexdigest()
json.dump({"schema": 1, "version": "0.0.1", "tag": "v0.0.1", "commit": sha,
           "build_time": "2026-09-13T00:00:00Z", "platform": "darwin-arm64",
           "components": ["agentd"],
           "toolchain": {"pin": "go1.25.5", "actual": "local", "policy": "local"},
           "artifacts": [{"name": n, "sha256": h}]},
          open(os.path.join(d, "manifest.json"), "w"), indent=2)
PY
export ZERG_FAKE_UNIT_BIN="$SANDBOX/sleep-service.sh"
OUT_NODE="$(ZERG_UPGRADE_SOURCE="file://$STAGE_NODE" ZERG_PREFIX="$SANDBOX/linbin" \
  ZERG_AGENTD_UNIT=linuxnode-agent ZERG_UPGRADE_REPO="$SANDBOX/local/repo" ZERG_STATE_DIR="$SANDBOX/local/state" \
  ZERG_RECEIPTS_DIR="$SANDBOX/linreceipts" ZERG_API_BASE="http://127.0.0.1:$LOCAL_PORT" \
  bash "$KERNEL" --role node --components agentd 2>&1)"; RC_NODE=$?
echo "$OUT_NODE" | sed 's/^/     /'
ck "节点角色（agentd）完整六阶段 ⇒ 退出码 0" "$RC_NODE" "0"
case "$OUT_NODE" in *"agentd 已重启并 active"*) ok "经 systemctl（假）重启并 is-active" ;; *) bad "未走 systemctl 重启路径" ;; esac
[ -f "$SANDBOX/systemd-linuxnode-agent.pid" ] && ok "假 systemd 里该 unit 有活进程（服务已起）" || bad "服务未起（unit 无 pid）"
case "$OUT_NODE" in *"UI"*) bad "节点角色竟然碰了 UI" ;; *) ok "节点角色不碰 UI（UI 仅 Mac）" ;; esac
python3 -c 'import json,sys
d=json.load(open(sys.argv[1]))
print("     node receipt: role=%s components=%s verify=%s" % (d.get("role"), d.get("components"), d["verify"]["mode"]))
assert d.get("role")=="node" and d.get("components")==["agentd"], "节点回执应记 role/components"
assert "ui" not in d.get("components"), "节点组件不得含 ui"' "$SANDBOX/linreceipts/latest.json" \
  && ok "节点回执记 role=node / components=[agentd]（回执 schema v2）" || bad "节点回执内容不符"

hdr "⑩ 隔离证明：真机未被动；假 ssh 只连沙箱名册目标"
REAL_BIN_AFTER="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_AFTER="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
[ "$REAL_BIN_SNAP" = "$REAL_BIN_AFTER" ] && ok "真机 bin/ 未变" || bad "真机 bin/ 被改动（越界！）"
[ "$REAL_STATE_SNAP" = "$REAL_STATE_AFTER" ] && ok "真机 ~/.zerg/state/ 未变" || bad "真机 state/ 被改动（越界！）"
# 断言写法刻意不出现真机内网地址（公开树里那类字面量会被替换规则改写/被敏感扫描盯上）：
# 直接要求 ssh 目标**全部**落在沙箱名册内——比比对 IP 更强（名册外一律拒连，假 ssh rc=99）
UNKNOWN_TARGETS="$(awk '/^ssh /{print $2}' "$SSHLOG" | sort -u | grep -vE '^(x3|mini1|bare)$' || true)"
[ -z "$UNKNOWN_TARGETS" ] && ok "假 ssh 只连沙箱名册目标（零真机）" || bad "连了名册外目标：$UNKNOWN_TARGETS"
n_unknown="$(grep -c "拒绝连接未知名册目标" "$SSHLOG" 2>/dev/null || true)"
note "假 ssh 调用总数：$(grep -c '^ssh ' "$SSHLOG" 2>/dev/null || echo 0)"

printf '\n──────── 结果：%d 过 / %d 败 ────────\n' "$PASS" "$FAIL"
[ "$FAIL" = "0" ]

#!/usr/bin/env bash
# test-relay-sandbox.sh —— B5.1 主控镜像中继验收套件（**全在 /tmp 沙箱 + 假 ssh 里跑，绝不接真机**）
#
# 被测对象：
#   ① scripts/fleet-relay.sh —— 把本地镜像仓的 main 推到每台节点的裸仓（幂等 / 回读验证 / 失败点名 / --dry-run）
#   ② scripts/zerg-upgrade.sh --fleet 的**节点步** —— 从节点**本地裸仓**取源（显式注入 ZERG_UPDATE_REMOTE）
#
# 覆盖（每条都能失败）：
#   1. 首跑：两台节点的裸仓被建出、refs/heads/main == 源 sha、逐台回读一致
#   2. 幂等：再跑一次 ⇒ up-to-date，引用不变（零新对象），源仓只读
#   3. --dry-run：零副作用（不 ssh、不建仓、不推送）
#   4. 不可达节点（假 ssh 拒绝的未知名册目标）⇒ 非零退出 + **点名**，其余节点照样完成
#   5. 权限不对（目标落在只读目录）⇒ 非零退出 + 点名
#   6. --node <name>：只同步那台（别的机器一个字节都不动）
#   7. 名册第二来源（fleet.yaml 的 updates.nodes + relay= 键）与 env 名册同解
#   8. 守卫：--from 指向私有权威仓本身 / 不存在的目录 / 非法 --node ⇒ 明确拒绝（rc 2）
#   9. 编排：--fleet 的节点步命令行**确实带 ZERG_UPDATE_REMOTE=<该机裸仓>**（打印真实命令行）
#  10. 该机没有裸仓 ⇒ **不注入**取源 + 响亮告警（回落到它自己的 origin，不造假）
#  11. 「节点从裸仓取源」本地等价场景：真 `git fetch --depth 1 <裸仓路径> main` 拿得到目标 sha
#  12. 隔离：假 ssh 只连沙箱名册目标；真机 bin/ 与 ~/.zerg/state/ 不变
#
# 用法：bash scripts/test-relay-sandbox.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KERNEL="$REPO_ROOT/scripts/zerg-upgrade.sh"
RELAY="$REPO_ROOT/scripts/fleet-relay.sh"
SANDBOX="${ZERG_RELAY_SANDBOX_DIR:-/tmp/zerg-relay-sandbox}"
FAKEBIN="$SANDBOX/fakebin"
SSHLOG="$SANDBOX/ssh-calls.log"
MIRROR="$SANDBOX/mirror"          # 「镜像仓」：普通 git 仓，有 main（模拟 publish/mirror-public.sh 的产出）
RECEIPTS="$SANDBOX/receipts"
DEAD_API="http://127.0.0.1:18899" # 死端口：绝不碰真机主控（8580）

PASS=0; FAIL=0
ok()   { printf '  ✅ %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  ❌ %s\n' "$1"; FAIL=$((FAIL+1)); }
note() { printf '     %s\n' "$1"; }
hdr()  { printf '\n=== %s ===\n' "$1"; }
ck()   { if [ "$2" = "$3" ]; then ok "$1（$2）"; else bad "$1：期望 [$3] 实得 [$2]"; fi; }
sha12(){ printf '%s' "$1" | cut -c1-12; }
gitq() { env GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t \
         GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"; }
snap_tree() { find "$1" -type f -exec shasum -a 256 {} \; 2>/dev/null | sort | shasum -a 256 | awk '{print $1}'; }

# ── 真机现状快照（整轮跑完必须不变）──────────────────────────────────────────
REAL_BIN_SNAP="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_SNAP="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
REAL_RC_SNAP="$(ls -la "$HOME/.zerg/update_receipts" 2>/dev/null | shasum -a 256 | awk '{print $1}')"

# ── 沙箱骨架 ─────────────────────────────────────────────────────────────────
rm -rf "$SANDBOX"; mkdir -p "$FAKEBIN" "$SANDBOX/x3" "$SANDBOX/mini1" "$SANDBOX/mini0"
# 清理前先给只读目录松绑：上一轮可能留下 chmod -w 的夹具目录 ⇒ 直接 rm 会 Permission denied（2026-09-14 偶发实测）
[ -d "$SANDBOX" ] && chmod -R u+w "$SANDBOX" 2>/dev/null
: > "$SSHLOG"

# ══ 0. 备料：镜像仓（main 初值 c1）+ 节点骨架 + 假 ssh ══════════════════════════
hdr "0. 备料：镜像仓（main）+ 三台假节点（x3/mini1/mini0）+ 假 ssh"
mkdir -p "$MIRROR"
gitq -C "$MIRROR" init -q -b main
printf '{"last_mirrored_private_sha":"deadbeef"}\n' > "$MIRROR/.mirror-state"   # 镜像产物特征件
echo "v1" > "$MIRROR/README.md"
mkdir -p "$MIRROR/core/internal/version"
printf 'package version\n\nconst Version = "9.9.9"\n' > "$MIRROR/core/internal/version/version.go"
gitq -C "$MIRROR" add -A
gitq -C "$MIRROR" commit -q -m "chore: release v9.9.9"
C1="$(gitq -C "$MIRROR" rev-parse HEAD)"

# 「节点工作树」（浅克隆——模拟节点上的 git 检出）：在 main=c1 时克隆，后面才前进
gitq clone -q --depth 1 "file://$MIRROR" "$SANDBOX/x3/work" 2>/dev/null || gitq clone -q --depth 1 "$MIRROR" "$SANDBOX/x3/work"
note "节点工作树 $SANDBOX/x3/work 停在 $(sha12 "$C1")（浅= $(gitq -C "$SANDBOX/x3/work" rev-parse --is-shallow-repository)）"

echo "v2" > "$MIRROR/README.md"
gitq -C "$MIRROR" add -A; gitq -C "$MIRROR" commit -q -m "feat: c2"
echo "v3" > "$MIRROR/README.md"
gitq -C "$MIRROR" add -A; gitq -C "$MIRROR" commit -q -m "feat: c3（目标）"
C3="$(gitq -C "$MIRROR" rev-parse HEAD)"
MIRROR_HEAD0="$(gitq -C "$MIRROR" rev-parse HEAD)"
MIRROR_SNAP0="$(snap_tree "$MIRROR")"
note "镜像仓 main = $(sha12 "$C3")（$(gitq -C "$MIRROR" rev-list --count main) 笔）· c1=$(sha12 "$C1")"

# 节点骨架：名册 root 指向的检出 + bin/zerg-core（让 --fleet 的 preflight 能过）
mk_node() { # $1=机器名
  local m="$1"
  mkdir -p "$SANDBOX/$m/repo/.git" "$SANDBOX/$m/repo/scripts" "$SANDBOX/$m/bin" "$SANDBOX/$m/receipts" "$SANDBOX/$m/state"
  : > "$SANDBOX/$m/repo/scripts/zerg-upgrade.sh"
  cat > "$SANDBOX/$m/bin/zerg-core" <<EOF
#!/usr/bin/env bash
printf 'STUB[${m}] zerg-core 收到: %s\n' "\$*"
printf 'STUB[${m}] ZERG_UPDATE_REMOTE=%s\n' "\${ZERG_UPDATE_REMOTE:-<未设置>}"
exit 0
EOF
  chmod +x "$SANDBOX/$m/bin/zerg-core"
  cat > "$SANDBOX/$m/node.env" <<EOF
ZERG_UPDATE_REPO=$SANDBOX/$m/repo
ZERG_PREFIX=$SANDBOX/$m/bin
ZERG_STATE_DIR=$SANDBOX/$m/state
ZERG_RECEIPTS_DIR=$SANDBOX/$m/receipts
ZERG_API_BASE=$DEAD_API
PATH=$FAKEBIN:\$PATH
EOF
}
for m in x3 mini1 mini0; do mk_node "$m"; done

# 假 ssh：① git 传输协议（GIT_SSH_COMMAND 调它）② 普通远端命令；未知名册目标一律拒连（禁接真机）
cat > "$FAKEBIN/ssh" <<EOF
#!/usr/bin/env bash
[ "\${1:-}" = "-G" ] && exit 1          # git 的 ssh 变体探测：直接失败 ⇒ 退回默认变体
rest=()
while [ \$# -gt 0 ]; do
  case "\$1" in
    -o|-p|-i|-l|-F) shift 2 ;;
    -*) shift ;;
    *) rest+=("\$1"); shift ;;
  esac
done
[ \${#rest[@]} -ge 2 ] || { echo "fake-ssh: 参数不足" >&2; exit 64; }
tgt="\${rest[0]}"; cmd=""
for ((i=1; i<\${#rest[@]}; i++)); do cmd="\${cmd:+\$cmd }\${rest[\$i]}"; done
echo "ssh \$tgt :: \$cmd" >> "$SSHLOG"
[ -n "\${ZERG_FAKE_SSH_NOISE:-}" ] && { echo "MOTD（假 ssh 噪声）：欢迎登录 \$node —— 真机 ssh 常会打横幅/告警，判定不得被它污染" >&2; echo "回显（ssh -v 之类会回显整条命令）：\$cmd" >&2; }
case "\$tgt" in
  x3|root@x3) node=x3 ;;
  mini1|root@mini1) node=mini1 ;;
  mini0|root@mini0) node=mini0 ;;
  *) echo "fake-ssh: 拒绝连接未知名册目标 '\$tgt'（沙箱禁接真机）" >&2; exit 99 ;;
esac
set -a; . "$SANDBOX/\$node/node.env"; set +a
exec /bin/sh -c "\$cmd"
EOF
chmod +x "$FAKEBIN/ssh"
export ZERG_RELAY_SSH="$FAKEBIN/ssh"
export ZERG_FLEET_SSH="$FAKEBIN/ssh"
export ZERG_FLEET_YAML="$SANDBOX/no-such.yaml"
export ZERG_FLEET_WAIT_S=1
export PATH="$FAKEBIN:$PATH"

# 名册（env 形态）：x3 不给 relay（考默认推导：root 的父目录下 zerg-relay.git）；mini1 给显式 relay=
ROSTER2="x3=x3,root=$SANDBOX/x3/repo,prefix=$SANDBOX/x3/bin,receipts=$SANDBOX/x3/receipts mini1=mini1,root=$SANDBOX/mini1/repo,prefix=$SANDBOX/mini1/bin,relay=$SANDBOX/mini1/relay/zerg-relay.git,receipts=$SANDBOX/mini1/receipts"
ROSTER3="$ROSTER2 mini0=mini0,root=$SANDBOX/mini0/repo,prefix=$SANDBOX/mini0/bin,receipts=$SANDBOX/mini0/receipts"
X3_RELAY="$SANDBOX/x3/zerg-relay.git"                 # x3：默认推导（root 的父目录）
M1_RELAY="$SANDBOX/mini1/relay/zerg-relay.git"        # mini1：名册 relay= 显式
M0_RELAY="$SANDBOX/mini0/zerg-relay.git"              # mini0：默认推导（**故意不建**）
note "预期目标裸仓：x3=$X3_RELAY · mini1=$M1_RELAY"

# ══════════════════════════════════════════════════════════════════════════════
hdr "① 首跑：两台节点都建出裸仓 + refs/heads/main == 源 sha + 逐台回读"
[ ! -e "$X3_RELAY" ] && ok "前置：x3 裸仓尚不存在" || bad "前置：x3 裸仓已存在（沙箱没清干净）"
: > "$SSHLOG"
OUT1="$(ZERG_FLEET_NODES="$ROSTER2" bash "$RELAY" --from "$MIRROR" 2>&1)"; RC1=$?
echo "$OUT1" | sed 's/^/     /'
ck "中继首跑退出码 0" "$RC1" "0"
for pair in "x3:$X3_RELAY" "mini1:$M1_RELAY"; do
  n="${pair%%:*}"; p="${pair#*:}"
  if [ -d "$p/objects" ]; then ok "$n 裸仓已建出（$(basename "$(dirname "$p")")/$(basename "$p")）"; else bad "$n 裸仓没建出来：$p"; fi
  got="$(gitq -C "$p" rev-parse refs/heads/main 2>/dev/null || echo '')"
  ck "$n 裸仓 refs/heads/main == 源 sha" "$(sha12 "$got")" "$(sha12 "$C3")"
done
case "$OUT1" in *"回读一致"*) ok "输出里逐台打印了「回读一致」" ;; *) bad "输出没打回读结果" ;; esac
case "$OUT1" in *"git init --bare"*) ok "如实报告了「已 git init --bare」" ;; *) bad "没报告建仓动作" ;; esac
case "$OUT1" in *"中继完成：2 台回读一致"*) ok "收尾汇总「2 台回读一致」" ;; *) bad "收尾汇总不对" ;; esac
grep -q "git-receive-pack" "$SSHLOG" && ok "推送真的走了 git 传输协议（ssh 上跑 git-receive-pack）" || bad "ssh 日志里没有 git-receive-pack"

hdr "② 幂等：再跑一次 ⇒ up-to-date、引用不变、源仓只读"
REF_SNAP_X3="$(gitq -C "$X3_RELAY" rev-parse refs/heads/main)"
BARE_SNAP="$(snap_tree "$X3_RELAY")"
: > "$SSHLOG"
OUT2="$(ZERG_FLEET_NODES="$ROSTER2" bash "$RELAY" --from "$MIRROR" 2>&1)"; RC2=$?
echo "$OUT2" | sed 's/^/     /'
ck "二跑退出码 0" "$RC2" "0"
case "$OUT2" in *"up-to-date"*) ok "二跑识别为 up-to-date（未推送对象）" ;; *) bad "二跑没有识别为 up-to-date" ;; esac
ck "二跑后 x3 引用未变" "$(gitq -C "$X3_RELAY" rev-parse refs/heads/main)" "$REF_SNAP_X3"
ck "二跑后 x3 裸仓内容逐文件未变（零新对象）" "$(snap_tree "$X3_RELAY")" "$BARE_SNAP"
ck "源仓 HEAD 未被中继改动" "$(gitq -C "$MIRROR" rev-parse HEAD)" "$MIRROR_HEAD0"
ck "源仓工作树逐文件未变（中继只读它）" "$(snap_tree "$MIRROR")" "$MIRROR_SNAP0"
[ -z "$(gitq -C "$MIRROR" remote -v)" ] && ok "源仓没有被加上任何 remote（中继不往源仓里写）" || bad "源仓被改了 remote"

hdr "③ --dry-run：零副作用（不 ssh、不建仓、不推送）"
: > "$SSHLOG"
OUT3="$(ZERG_FLEET_NODES="$ROSTER3" bash "$RELAY" --from "$MIRROR" --dry-run 2>&1)"; RC3=$?
echo "$OUT3" | sed 's/^/     /'
ck "--dry-run 退出码 0" "$RC3" "0"
[ ! -s "$SSHLOG" ] && ok "假 ssh 零调用（dry-run 不连任何机器）" || { bad "dry-run 竟然 ssh 了"; sed 's/^/     /' "$SSHLOG"; }
[ ! -e "$M0_RELAY" ] && ok "mini0 裸仓仍不存在（dry-run 没建仓）" || bad "dry-run 建了仓（有副作用）"
case "$OUT3" in *"零副作用"*) ok "dry-run 自报「零副作用」" ;; *) bad "dry-run 没说明零副作用" ;; esac
case "$OUT3" in *"x3  x3:$X3_RELAY"*|*"$X3_RELAY"*) ok "计划里列出了 x3 的目标裸仓路径" ;; *) bad "计划里缺 x3 目标路径" ;; esac

hdr "④ 不可达节点 ⇒ 非零退出 + 点名（其余节点照样完成）"
: > "$SSHLOG"
ROSTER_GHOST="$ROSTER2 ghost=ghost,root=$SANDBOX/ghost/repo"
OUT4="$(ZERG_FLEET_NODES="$ROSTER_GHOST" bash "$RELAY" --from "$MIRROR" 2>&1)"; RC4=$?
echo "$OUT4" | tail -8 | sed 's/^/     /'
ck "有节点不可达 ⇒ 退出码 1" "$RC4" "1"
case "$OUT4" in *"失败点名：ghost"*) ok "收尾**点名**了不可达的 ghost" ;; *) bad "没有点名失败节点" ;; esac
case "$OUT4" in *"❌ ghost"*) ok "逐台结果里也标了 ❌ ghost" ;; *) bad "逐台结果没标 ghost" ;; esac
ck "同轮里 x3 的引用仍是目标（失败不牵连其它台）" "$(sha12 "$(gitq -C "$X3_RELAY" rev-parse refs/heads/main)")" "$(sha12 "$C3")"
grep -qE '^ssh ghost :: ' "$SSHLOG" && ok "ghost 的调用进了 ssh 日志（假 ssh 对名册外目标一律拒连 rc=99）" || bad "没看到对 ghost 的调用"
case "$OUT4" in *"ghost(不可达或权限不对"*) ok "逐台原因里点名了 ghost 并给出原因" ;; *) bad "没给逐台原因" ;; esac

hdr "⑤ 权限不对（目标落在只读目录）⇒ 非零退出 + 点名"
mkdir -p "$SANDBOX/ro"; chmod 500 "$SANDBOX/ro"
: > "$SSHLOG"
OUT5="$(ZERG_FLEET_NODES="$ROSTER2" bash "$RELAY" --from "$MIRROR" --to-path "$SANDBOX/ro/zerg-relay.git" 2>&1)"; RC5=$?
chmod 700 "$SANDBOX/ro"
echo "$OUT5" | tail -8 | sed 's/^/     /'
ck "写不进去 ⇒ 退出码 1" "$RC5" "1"
case "$OUT5" in *"失败点名：x3 mini1"*|*"失败点名：mini1 x3"*) ok "点名了两台（都失败）" ;; *) bad "没点名失败节点" ;; esac
case "$OUT5" in *"不可达或权限不对"*) ok "如实区分了「不可达/权限不对」" ;; *) bad "失败原因没说明" ;; esac

hdr "⑥ --node <name>：只同步那一台（别的机器一个字节都不动）"
: > "$SSHLOG"
OUT6="$(ZERG_FLEET_NODES="$ROSTER3" bash "$RELAY" --from "$MIRROR" --node x3 2>&1)"; RC6=$?
ck "--node x3 退出码 0" "$RC6" "0"
case "$OUT6" in *"只同步 x3"*) ok "自报范围「只同步 x3」" ;; *) bad "范围没声明" ;; esac
grep -q "^ssh mini0 " "$SSHLOG" && bad "--node x3 竟然连了 mini0" || ok "--node x3 没连 mini0（零 ssh）"
[ ! -e "$M0_RELAY" ] && ok "mini0 裸仓仍不存在" || bad "mini0 被顺带建仓了"

hdr "⑦ 名册第二来源：fleet.yaml 的 updates.nodes（含 relay= 键）与 env 名册同解"
cat > "$SANDBOX/fleet-test.yaml" <<EOF
# 沙箱假 fleet.yaml（私有配置文件形态）
updates:
  check: true
  nodes:
    - { name: x3, ssh: x3, root: $SANDBOX/x3/repo, prefix: $SANDBOX/x3/bin, receipts: "$SANDBOX/x3/receipts", components: "core,agentd", relay: "$SANDBOX/x3/yaml-relay.git" }
fleet:
  x3: { host: 192.0.2.10, port: 8100, os: ubuntu }   # RFC 5737 文档专用段（夹具，非真机）
EOF
: > "$SSHLOG"
OUT7="$(ZERG_FLEET_NODES="" ZERG_FLEET_YAML="$SANDBOX/fleet-test.yaml" bash "$RELAY" --from "$MIRROR" 2>&1)"; RC7=$?
echo "$OUT7" | sed 's/^/     /'
ck "yaml 名册中继退出码 0" "$RC7" "0"
ck "yaml 里的 relay= 生效（裸仓落在 yaml 指定的路径）" "$(sha12 "$(gitq -C "$SANDBOX/x3/yaml-relay.git" rev-parse refs/heads/main 2>/dev/null)")" "$(sha12 "$C3")"

hdr "⑧ 守卫：--from 指私有权威仓本身 / 不存在 / 非法 --node ⇒ 明确拒绝"
OUT8A="$(bash "$RELAY" --from "$REPO_ROOT" 2>&1)"; RC8A=$?
case "$OUT8A" in *"私有权威仓本身"*) ok "拒绝 --from=私有权威仓本身（rc=${RC8A}），并说明应当用镜像仓" ;; *) bad "没拦住 --from=私有仓：$OUT8A" ;; esac
[ "$RC8A" != "0" ] && ok "该拒绝是非零退出（rc=${RC8A}）" || bad "该拒绝竟然 rc=0"
OUT8A2="$(bash "$RELAY" --from "$REPO_ROOT/scripts" 2>&1)"; RC8A2=$?
case "$OUT8A2" in *"私有权威仓的工作树"*) ok "拒绝 --from=<私有仓子目录>（rc=${RC8A2}）——子目录会解析到同一工作树" ;; *) bad "没拦住私有仓子目录：$OUT8A2" ;; esac
OUT8B="$(bash "$RELAY" --from "$SANDBOX/does-not-exist" 2>&1)"; RC8B=$?
[ "$RC8B" = "2" ] && ok "不存在的 --from ⇒ rc=2（$(printf '%s' "$OUT8B" | head -1)）" || bad "不存在的 --from 退出码应为 2，实得 $RC8B"
OUT8C="$(ZERG_FLEET_NODES="$ROSTER2" bash "$RELAY" --from "$MIRROR" --node nosuch 2>&1)"; RC8C=$?
case "$OUT8C" in *"不在名册里"*) ok "--node nosuch ⇒ 点名拒绝（rc=${RC8C}）" ;; *) bad "非法 --node 没被拦住" ;; esac
mkdir -p "$SANDBOX/notgit"
OUT8D="$(bash "$RELAY" --from "$SANDBOX/notgit" 2>&1)"; RC8D=$?
[ "$RC8D" = "2" ] && ok "非 git 仓的 --from ⇒ rc=2" || bad "非 git 仓的 --from 退出码应为 2，实得 $RC8D"

hdr "⑧b ssh 带噪声（MOTD / 命令回显）时，判定不得被污染"
: > "$SSHLOG"
OUTN="$(ZERG_FAKE_SSH_NOISE=1 ZERG_FLEET_NODES="$ROSTER2" bash "$RELAY" --from "$MIRROR" 2>&1)"; RCN=$?
echo "$OUTN" | sed 's/^/     /'
ck "带噪声 ssh 下中继仍 rc=0" "$RCN" "0"
case "$OUTN" in *"裸仓已存在（沿用）"*) ok "「裸仓已存在」判定正确（没被回显里的 RELAY-CREATED 字面量骗到）" ;; *) bad "建仓判定被噪声/回显污染" ;; esac
case "$OUTN" in *"回读一致"*) ok "回读一致（sha 从噪声里正确提取）" ;; *) bad "回读被噪声污染（真机 ssh 横幅会让它误判）" ;; esac

hdr "⑨ 编排：--fleet 的节点步**确实带** ZERG_UPDATE_REMOTE=<该机裸仓>（打印真实命令行）"
: > "$SSHLOG"
OUT9="$(ZERG_FLEET_NODES="$ROSTER3" ZERG_API_BASE="$DEAD_API" ZERG_RECEIPTS_DIR="$RECEIPTS" \
  ZERG_FLEET_LOCAL_CMD="$SANDBOX/x3/bin/zerg-core" bash "$KERNEL" --fleet --no-ui --tag "$C3" 2>&1)"; RC9=$?
echo "$OUT9" | sed 's/^/     /'
note "── 假 ssh 记下的真实命令行（节点步）──"
grep -E "^ssh (x3|mini1|mini0) :: .*zerg-core' update" "$SSHLOG" | sed 's/^/       /' | head -6
grep -q "ZERG_UPDATE_REMOTE='$X3_RELAY'" "$SSHLOG" && ok "x3 节点步命令行里有 ZERG_UPDATE_REMOTE='$X3_RELAY'" || bad "x3 节点步没带该机裸仓路径"
grep -q "ZERG_UPDATE_REMOTE='$M1_RELAY'" "$SSHLOG" && ok "mini1 节点步命令行里有 ZERG_UPDATE_REMOTE='$M1_RELAY'（名册 relay= 生效）" || bad "mini1 节点步没带名册指定的裸仓路径"
grep -q "update --role node --components 'core,agentd'" "$SSHLOG" && ok "节点步仍是「该机自己 update」（口径未变）" || bad "节点步口径变了"
note "── 该轮 stdout 里节点取源那几行 ──"
printf '%s\n' "$OUT9" | grep -E "取源：该机本地裸仓|没有本地裸仓" | sed 's/^/       /'
FRC="$RECEIPTS/latest-fleet.json"
if [ -f "$FRC" ]; then
  ok "机群回执已落盘（未写进真机回执目录）"
  python3 - "$FRC" "$X3_RELAY" "$M1_RELAY" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
xl = {n["name"]: n.get("source_remote") for n in d["nodes"]}
print("       回执 source_remote：" + ", ".join("%s=%s" % (k, v) for k, v in xl.items()))
assert xl.get("x3") == sys.argv[2], "x3 的 source_remote 应记该机裸仓"
assert xl.get("mini1") == sys.argv[3], "mini1 的 source_remote 应记该机裸仓"
PY
  [ $? = 0 ] && ok "回执里逐台记下了取源（source_remote = 该机裸仓路径）" || bad "回执没记取源"
else
  bad "没有机群回执"
fi

hdr "⑩ 该机没有裸仓 ⇒ 不注入取源 + 响亮告警（不造假）"
: > "$SSHLOG"
OUT10="$(ZERG_FLEET_NODES="$ROSTER3" ZERG_API_BASE="$DEAD_API" ZERG_RECEIPTS_DIR="$RECEIPTS" \
  ZERG_FLEET_LOCAL_CMD="$SANDBOX/x3/bin/zerg-core" bash "$KERNEL" --fleet --no-ui --tag "$C3" 2>&1)"
M0_CMD="$(grep -E "^ssh mini0 :: .*zerg-core' update" "$SSHLOG" | head -1)"
note "mini0 真实命令行："
printf '%s\n' "$M0_CMD" | sed 's/^/       /'
[ -n "$M0_CMD" ] && ok "mini0 的节点步被执行了（能被看到，才谈得上核对）" || bad "mini0 节点步没执行"
case "$M0_CMD" in *ZERG_UPDATE_REMOTE*) bad "mini0 没有裸仓却仍然注入了取源" ;; *) ok "mini0 没有裸仓 ⇒ 命令行里**没有** ZERG_UPDATE_REMOTE" ;; esac
printf '%s\n' "$OUT10" | grep -q "该机没有本地裸仓" && ok "stdout 响亮告警「该机没有本地裸仓」" || bad "没有告警（会静默回落到该机 origin）"
printf '%s\n' "$OUT10" | grep -q "fleet-relay.sh --from" && ok "告警里给出下一步命令（fleet-relay.sh）" || bad "告警没给下一步命令"

hdr "⑪ 节点从本地裸仓取源：真 git fetch（本地等价场景）"
mkdir -p "$SANDBOX/x3/workcfg"
FETCH_ERR="$(cd "$SANDBOX/x3/work" && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null \
  git fetch --depth 1 "$X3_RELAY" main 2>&1)"; FRC_CODE=$?
note "git fetch --depth 1 $X3_RELAY main ⇒ rc=$FRC_CODE"
[ -n "$FETCH_ERR" ] && printf '%s\n' "$FETCH_ERR" | sed 's/^/       （git 输出）/' || true
FETCH_SHA="$(gitq -C "$SANDBOX/x3/work" rev-parse FETCH_HEAD 2>/dev/null || echo '')"
ck "本机（节点等价场景）真 fetch 拿到目标 sha" "$(sha12 "$FETCH_SHA")" "$(sha12 "$C3")"
note "节点工作树 fetch 后 shallow=$(gitq -C "$SANDBOX/x3/work" rev-parse --is-shallow-repository)"
gitq -C "$SANDBOX/x3/work" reset -q --hard FETCH_HEAD
ck "换装后工作树 HEAD == 目标" "$(sha12 "$(gitq -C "$SANDBOX/x3/work" rev-parse HEAD)")" "$(sha12 "$C3")"
ck "取到的树里带着镜像特征件 .mirror-state" "$(gitq -C "$SANDBOX/x3/work" ls-tree --name-only HEAD | grep -c '^\.mirror-state$' || true)" "1"
ck "取到的 README 内容 == 目标那笔（v3）" "$(cat "$SANDBOX/x3/work/README.md")" "v3"

hdr "⑫ 隔离证明：假 ssh 只连沙箱名册目标；真机未被动"
UNKNOWN="$(awk '/^ssh /{print $2}' "$SSHLOG" | sort -u | grep -vE '^(x3|mini1|mini0|ghost)$' || true)"
[ -z "$UNKNOWN" ] && ok "假 ssh 只连沙箱名册目标（零真机）" || bad "连了名册外目标：$UNKNOWN"
REAL_BIN_AFTER="$(cd "$REPO_ROOT" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')"
REAL_STATE_AFTER="$(find "$HOME/.zerg/state" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')"
[ "$REAL_BIN_SNAP" = "$REAL_BIN_AFTER" ] && ok "真机 bin/ 未变" || bad "真机 bin/ 被改动（越界！）"
[ "$REAL_STATE_SNAP" = "$REAL_STATE_AFTER" ] && ok "真机 ~/.zerg/state/ 未变" || bad "真机 state/ 被改动（越界！）"
[ ! -e "$HOME/.zerg/update_receipts/latest-fleet.json.new-by-suite" ] && ok "真机回执目录未被本套件写入（回执都在沙箱）" || bad "真机回执目录被动了"
REAL_RC_AFTER="$(ls -la "$HOME/.zerg/update_receipts" 2>/dev/null | shasum -a 256 | awk '{print $1}')"
[ "$REAL_RC_SNAP" = "$REAL_RC_AFTER" ] && ok "真机 ~/.zerg/update_receipts/ 目录未变" || bad "真机回执目录被改动（越界！）"

printf '\n──────── 结果：%d 过 / %d 败 ────────\n' "$PASS" "$FAIL"
[ "$FAIL" = "0" ]

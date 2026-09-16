#!/bin/bash
# calib-x3-model.sh —— X3（Linux/Radeon，有真 GTT）模型标定：3 轮取上界，产出实测档案（§8.4）。
#
# 与 calib-local-model.sh（macOS 版）的差别：
#   · GTT 是**真口径**，必须实测（读 amdgpu sysfs / rocm-smi），不许记 0；
#   · 端口默认 9699：**刻意避开**子端端口池 9400-9499 与沙箱池 9500-9509，免得和孵化抢端口。
#
# 用法（在 X3 上跑）：
#   bash calib-x3-model.sh <卵名> <gguf 绝对路径> [引擎] [ctx] [端口] [额外参数…]
#   # 例：bash calib-x3-model.sh example-35b /data/models/misc/example-35b-Q4_K_M.gguf \
#   #        /home/g01/llama-k2/build-k2/bin/llama-server 262144 9699
#   # 多模态例（带 mmproj）：
#   #   bash calib-x3-model.sh example-35b-v2 /data/models/ornith/Ornith-1.5-35B-Q4_K_M.gguf \
#   #        /home/g01/llama-k2/build-k2/bin/llama-server 262144 9699 --mmproj /data/models/ornith/mmproj-Ornith-1.5-35B-BF16.gguf
#
# 规矩：只读权重 · 跑完即停 · **有效轮数 < 3 ⇒ 不写档案**（宁可无档案，不许凑数）。
# 说明：额外参数在传给引擎的同时，会**原样写进档案的注释**里（配方可复现）。
set -u

NAME=${1:?用法: calib-x3-model.sh <卵名> <gguf 路径> [引擎] [ctx] [端口]}
FILE=${2:?缺少 gguf 路径}
ENG=${3:-/home/g01/llama-k2/build-k2/bin/llama-server}
CTX=${4:-262144}
PORT=${5:-9699}
EXTRA_DISPLAY="${6:-}"
shift $(( $# < 5 ? $# : 5 ))   # 余下参数原样传给引擎（如 --mmproj <路径>）
OUT="$HOME/.zerg/egg-profiles"
mkdir -p "$OUT"

[ -f "$FILE" ] || { echo "!! 权重不存在：$FILE"; exit 2; }
[ -x "$ENG" ] || { echo "!! 引擎不可执行：$ENG"; exit 2; }

gtt_used() {  # 返回 GTT 已用字节；优先 sysfs，退而求其次 rocm-smi
  local f
  for f in /sys/class/drm/card*/device/mem_info_gtt_used; do
    [ -r "$f" ] && { cat "$f"; return 0; }
  done
  rocm-smi --showmeminfo gtt 2>/dev/null | awk '/GTT Total Used/ {print $NF; exit}'
}

W_B=$(stat -c%s "$FILE")
WG=$(python3 -c "print('%.2f' % ($W_B/1073741824))")
G0=$(gtt_used); [ -n "${G0:-}" ] || { echo "!! 读不到 GTT 读数，不敢编数字 ⇒ 退出"; exit 3; }
echo "卵=$NAME 引擎=$ENG 权重=$W_B 字节（$WG GiB） ctx=$CTX port=$PORT"
echo "GTT 基线 = $G0 字节（$(python3 -c "print('%.1f'%($G0/1048576))") MiB）"

best_load=0; best_mem=0; best_tps=0; best_gtt=0; runs=0
for i in 1 2 3; do
  echo "== 第 $i 轮 =="
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 3
  T0=$(date +%s.%N)
  "$ENG" -m "$FILE" -c "$CTX" -ngl 999 --host 127.0.0.1 --port "$PORT" --no-warmup "$@" \
    > "/tmp/calib-$NAME-$i.log" 2>&1 &
  pid=$!
  ok=no
  for j in $(seq 1 150); do
    sleep 2
    c=$(curl -s -o /dev/null -w '%{http_code}' -m 2 "http://127.0.0.1:$PORT/health" 2>/dev/null)
    [ "$c" = "200" ] && { ok=yes; break; }
    kill -0 "$pid" 2>/dev/null || break
  done
  T1=$(date +%s.%N)
  if [ "$ok" != yes ]; then
    echo "  起不来：$(tail -2 "/tmp/calib-$NAME-$i.log" | tr '\n' ' ' | head -c 220)"
    pkill -f "llama-server.*--port $PORT" 2>/dev/null
    continue
  fi
  load=$(echo "$T1 - $T0" | bc)
  rss_kb=$(ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ')
  mem_gb=$(echo "scale=3; ${rss_kb:-0}/1048576" | bc)
  gtt_now=$(gtt_used)
  gtt_delta_b=$(( ${gtt_now:-0} - G0 ))
  gtt_delta_gb=$(python3 -c "print('%.3f' % ($gtt_delta_b/1073741824))")
  t2=$(date +%s.%N)
  n=$(curl -s -m 300 "http://127.0.0.1:$PORT/v1/chat/completions" -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"用一句话说明什么是本地推理。"}],"max_tokens":64,"temperature":0}' \
      | python3 -c "import json,sys
try: print(json.load(sys.stdin).get('usage',{}).get('completion_tokens',0))
except Exception: print(0)")
  t3=$(date +%s.%N)
  dt=$(echo "$t3 - $t2" | bc)
  tps=$(python3 -c "print('%.2f' % ($n/max($dt,0.001)))")
  echo "  装载 ${load}s · 峰值RSS ${mem_gb} GiB · GTT +${gtt_delta_gb} GiB · 生成 $n tok ⇒ $tps tok/s"
  runs=$((runs+1))
  best_load=$(python3 -c "print(max($best_load,$load))")
  best_mem=$(python3 -c "print(max($best_mem,$mem_gb))")
  best_tps=$(python3 -c "print(max($best_tps,$tps))")
  best_gtt=$(python3 -c "print(max($best_gtt,$gtt_delta_gb))")
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 5
done
pkill -f "llama-server.*--port $PORT" 2>/dev/null

echo "== 汇总 runs=$runs 装载=${best_load}s RSS=${best_mem}GiB GTT=${best_gtt}GiB 吞吐=${best_tps}tok/s =="
if [ "$runs" -ge 3 ]; then
  cat > "$OUT/$NAME.yaml" <<YML
# $NAME 实测档案（X3 标定；§8.4 六字段 + 溯源）
# 引擎：${ENG}；GTT 为实测差（基线 $G0 B）。
# 账种（丙4，Mr2109 2026-09-16 拍）：Linux/Radeon 侧这笔账是 **GTT**（macOS 侧对应的是 GPU wired）。
gpu_mem_kind: gtt
# 额外参数（原样传给引擎）：${EXTRA_DISPLAY:-（无）}
weight_size_gb: $WG
peak_gtt_gb: $best_gtt
peak_mem_gb: $best_mem
load_seconds: $best_load
throughput_tok_s: $best_tps
suggested_idle_unload_s: 300
schema_version: 1
measured_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)
machine: x3
calib_runs: $runs
YML
  echo "档案已写：$OUT/$NAME.yaml"
  cat "$OUT/$NAME.yaml"
else
  echo "!! 有效轮数 $runs < 3 ⇒ 按标定铁律不写档案（宁可无档案，不许凑数）"
fi

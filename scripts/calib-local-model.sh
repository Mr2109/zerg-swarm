#!/bin/bash
# calib-local-model.sh —— 本机（macOS）模型标定：3 轮取上界，产出实测档案（§8.4）。
#
# 为什么存在：批 5 破段要把 fleet.yaml 里每条 `host: local` 迁到 Mr2109 子端，而子端的闸门
# **只读实测档案**（§8.4 标定铁律）⇒ 每个要迁的模型都得先在本机标定一次。把一次性脚本做成工具，
# 免得每次重写、也免得各自长歪。
#
# 用法：
#   bash scripts/calib-local-model.sh <卵名> <gguf 绝对路径> [ctx] [引擎] [端口]
#   # 例：bash scripts/calib-local-model.sh example-35b ~/models/example-35b-Q4_K_M.gguf 262144
#
# 规矩（与项目铁律一致）：
#   · 无害：只读权重、高位端口、每轮跑完即停；
#   · **有效轮数 < 3 ⇒ 不写档案**（宁可无档案，不许凑数）；
#   · macOS 无 GTT ⇒ peak_gtt_gb 记 0 并在档案里注明口径（不编造）。
set -u

NAME=${1:?用法: calib-local-model.sh <卵名> <gguf 路径> [ctx] [引擎] [端口]}
FILE=${2:?缺少 gguf 路径}
CTX=${3:-131072}
ENG=${4:-/opt/homebrew/bin/llama-server}
PORT=${5:-9470}
OUT="$HOME/.zerg/egg-profiles"
mkdir -p "$OUT"

[ -f "$FILE" ] || { echo "!! 权重不存在：$FILE"; exit 2; }
[ -x "$ENG" ] || { echo "!! 引擎不可执行：$ENG"; exit 2; }

W_B=$(stat -f%z "$FILE")
WG=$(python3 -c "print('%.2f' % ($W_B/1073741824))")
echo "卵=$NAME 引擎=$ENG 权重=$W_B 字节（$WG GiB） ctx=$CTX port=$PORT"

best_load=0; best_mem=0; best_tps=0; runs=0
for i in 1 2 3; do
  echo "== 第 $i 轮 =="
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 2
  T0=$(date +%s.%N)
  "$ENG" -m "$FILE" -c "$CTX" -ngl 999 --host 127.0.0.1 --port "$PORT" --no-warmup \
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
  t2=$(date +%s.%N)
  n=$(curl -s -m 240 "http://127.0.0.1:$PORT/v1/chat/completions" \
      -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"用一句话说明什么是本地推理。"}],"max_tokens":64,"temperature":0}' \
      | python3 -c "import json,sys
try: print(json.load(sys.stdin).get('usage',{}).get('completion_tokens',0))
except Exception: print(0)")
  t3=$(date +%s.%N)
  dt=$(echo "$t3 - $t2" | bc)
  tps=$(python3 -c "print('%.2f' % ($n/max($dt,0.001)))")
  echo "  装载 ${load}s · 峰值RSS ${mem_gb} GiB · 生成 $n tok / ${dt}s ⇒ $tps tok/s"
  runs=$((runs+1))
  best_load=$(python3 -c "print(max($best_load,$load))")
  best_mem=$(python3 -c "print(max($best_mem,$mem_gb))")
  best_tps=$(python3 -c "print(max($best_tps,$tps))")
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 3
done
pkill -f "llama-server.*--port $PORT" 2>/dev/null

echo "== 汇总 runs=$runs 装载=${best_load}s RSS=${best_mem}GiB 吞吐=${best_tps}tok/s =="
if [ "$runs" -ge 3 ]; then
  cat > "$OUT/$NAME.yaml" <<YML
# $NAME 实测档案（本机 Mr2109 标定；§8.4 六字段 + 溯源）
# GTT 口径说明：macOS 无 GTT（Linux/Radeon 口径）⇒ 记 0 并在此注明，不编造。
weight_size_gb: $WG
peak_gtt_gb: 0
peak_mem_gb: $best_mem
load_seconds: $best_load
throughput_tok_s: $best_tps
suggested_idle_unload_s: 300
schema_version: 1
measured_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)
machine: Mr2109
calib_runs: $runs
YML
  echo "档案已写：$OUT/$NAME.yaml"
  cat "$OUT/$NAME.yaml"
else
  echo "!! 有效轮数 $runs < 3 ⇒ 按标定铁律不写档案（宁可无档案，不许凑数）"
fi

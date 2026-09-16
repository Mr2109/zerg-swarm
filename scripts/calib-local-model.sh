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
#   · macOS 无 GTT，但有 GPU wired 账 ⇒ peak_gtt_gb 写**实测 wired 峰值**（gpu_mem_kind: wired）。
set -u

NAME=${1:?用法: calib-local-model.sh <卵名> <gguf 路径> [ctx] [引擎] [端口]}
FILE=${2:?缺少 gguf 路径}
CTX=${3:-131072}
ENG=${4:-/opt/homebrew/bin/llama-server}
PORT=${5:-9470}
OUT="$HOME/.zerg/egg-profiles"
mkdir -p "$OUT"

# GPU wired 账（macOS/Apple Silicon 的"GTT 等价物"）：
#   统一内存里被 GPU 钉住、不可压缩不可换出的部分。读 IORegistry 的 IOAccelerator 计数
#   （实测不需要 sudo）。为什么用这笔账而不是记 0：闸门 §8.4 要求"预算只读实测档案"，
#   而 macOS 上"没有 GTT"≠"没有这笔账" —— 业界（llama.cpp/Ollama/MLX）也对着这笔账做放置决策。
gpu_wired_bytes() {
  ioreg -rd1 -c IOAccelerator 2>/dev/null | tr ',' '\n' \
    | grep -m1 '"In use system memory"' | grep -oE '[0-9]+' | head -1
}

[ -f "$FILE" ] || { echo "!! 权重不存在：$FILE"; exit 2; }
[ -x "$ENG" ] || { echo "!! 引擎不可执行：$ENG"; exit 2; }

W_B=$(stat -f%z "$FILE")
WG=$(python3 -c "print('%.2f' % ($W_B/1073741824))")
echo "卵=$NAME 引擎=$ENG 权重=$W_B 字节（$WG GiB） ctx=$CTX port=$PORT"

best_load=0; best_mem=0; best_tps=0; best_wired=0; runs=0
for i in 1 2 3; do
  echo "== 第 $i 轮 =="
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 2
  w0=$(gpu_wired_bytes); [ -n "${w0:-}" ] || w0=0   # 本轮基线（在起引擎之前）
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
  w1=$(gpu_wired_bytes); [ -n "${w1:-}" ] || w1=0
  t2=$(date +%s.%N)
  # 生成段：**必须真的出字**才算这轮有效（假绿洞修于 2026-09-16：此前只看 /health=200，
  # Qwen3.8-27B 三轮 HTTP 立刻失败、吞吐 0，却被写成了合法档案 ✗）。
  GEN_JSON="/tmp/calib-gen-$NAME-$i.json"
  GEN_CODE=$(curl -s -m 240 -o "$GEN_JSON" -w '%{http_code}' \
      "http://127.0.0.1:$PORT/v1/chat/completions" -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"用一句话说明什么是本地推理。"}],"max_tokens":64,"temperature":0}')
  n=$(python3 -c "import json
try: print(json.load(open('$GEN_JSON')).get('usage',{}).get('completion_tokens',0))
except Exception: print(0)")
  if [ "${GEN_CODE:-000}" != "200" ] || [ "${n:-0}" -le 0 ]; then
    echo "  生成未通过（HTTP=${GEN_CODE:-?} tokens=${n:-0}）⇒ **该轮无效、不计入**；正文前 200 字：$(head -c 200 "$GEN_JSON" 2>/dev/null)"
    pkill -f "llama-server.*--port $PORT" 2>/dev/null
    sleep 3
    continue
  fi
  t3=$(date +%s.%N)
  w2=$(gpu_wired_bytes); [ -n "${w2:-}" ] || w2=0
  wired_b=$(python3 -c "print(max(0, max(${w1:-0}, ${w2:-0}) - ${w0:-0}))")   # 差值 = 这枚卵自己钉住的量
  wired_gb=$(python3 -c "print('%.3f' % ($wired_b/1073741824))")
  dt=$(echo "$t3 - $t2" | bc)
  tps=$(python3 -c "print('%.2f' % ($n/max($dt,0.001)))")
  echo "  装载 ${load}s · 峰值RSS ${mem_gb} GiB · GPU wired ${wired_gb} GiB · 生成 $n tok / ${dt}s ⇒ $tps tok/s"
  runs=$((runs+1))
  best_load=$(python3 -c "print(max($best_load,$load))")
  best_mem=$(python3 -c "print(max($best_mem,$mem_gb))")
  best_tps=$(python3 -c "print(max($best_tps,$tps))")
  best_wired=$(python3 -c "print(max(${best_wired:-0},$wired_gb))")
  pkill -f "llama-server.*--port $PORT" 2>/dev/null
  sleep 3
done
pkill -f "llama-server.*--port $PORT" 2>/dev/null

echo "== 汇总 runs=$runs 装载=${best_load}s RSS=${best_mem}GiB GPU_wired=${best_wired}GiB 吞吐=${best_tps}tok/s =="
if [ "$runs" -ge 3 ]; then
  cat > "$OUT/$NAME.yaml" <<YML
# $NAME 实测档案（本机 Mr2109 标定；§8.4 六字段 + 溯源）
# GPU 账口径（丙4，Mr2109 2026-09-16 拍）：macOS 没有 Linux/Radeon 意义上的 GTT，
# 但**有对应的那笔账 = GPU wired memory**（统一内存里被 GPU 钉住、不可压缩不可换出）。
# 本档案的 peak_gtt_gb 装的是**实测的 GPU wired 峰值**（gpu_mem_kind: wired），不是 GTT；
# 预算口径：对着 Metal 的 recommendedMaxWorkingSetSize（≈75% 统一内存）与 sysctl
# iogpu.wired_limit_mb 取小。详见设计稿「平台账口径」一节。
gpu_mem_kind: wired
weight_size_gb: $WG
peak_gtt_gb: $best_wired
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

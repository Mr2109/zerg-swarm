#!/bin/bash
# v2.5.2 P2 定时接入——cron 触发提交任务进循环
# 用法: zerg-cron-submit.sh <任务描述> [type] [priority]
# 例:  zerg-cron-submit.sh "每日死代码扫描" code normal
TASK="${1:-定时任务}"
TYPE="${2:-code}"
PRIORITY="${3:-normal}"
PORT="${ZERG_API_PORT:-8083}"
curl -s -X POST "http://localhost:${PORT}/task" -H "Content-Type: application/json" \
  -d "{\"task\":\"${TASK}\",\"type\":\"${TYPE}\",\"priority\":\"${PRIORITY}\",\"source\":\"cron\"}"

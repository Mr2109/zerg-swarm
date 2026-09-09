#!/bin/bash
# start-zerg-core.sh — 主控启动器(2026-09-09 诊断 R4: 日志落盘——秒退/路由可取证)
# 用法: ./start-zerg-core.sh [前台]   —— 默认前台;日志 tee 到 /tmp/zerg-core.log
# 环境: ZERG_EXTRA_ALLOW_DIR 可选(冒号分隔白名单——测试样本目录用)
cd "$(dirname "$0")"
export ZERG_EXTRA_ALLOW_DIR="${ZERG_EXTRA_ALLOW_DIR:-}"
exec ./bin/zerg-core 2>&1 | tee /tmp/zerg-core.log

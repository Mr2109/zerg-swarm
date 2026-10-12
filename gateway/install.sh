#!/bin/bash
#
# install.sh — 本机三件套构建（protocol.md §10）
#  1. 模型管理器 App（LlamaControlCore + llama-control，含 FleetController/FleetAPI）
#  2. X3Monitor（:8580 群监控）
# 不执行任何部署/重启动作。
#
set -euo pipefail
cd "$(dirname "$0")"

MODEL_MANAGER_DIR="${MODEL_MANAGER_DIR:-}"   # 可选：模型管理器目录（按需用环境变量指定）
X3_MONITOR_DIR="${X3_MONITOR_DIR:-}"        # 可选：EDGE-NODE 监控目录

echo "==> [1/2] 构建模型管理器 App"
if [[ -d "$MODEL_MANAGER_DIR" ]]; then
    (cd "$MODEL_MANAGER_DIR" && swift build)
    echo "    构建完成（swift build）。可运行: open $MODEL_MANAGER_DIR/模型管理器.app"
else
    echo "    !! 目录不存在: $MODEL_MANAGER_DIR"
fi

echo "==> [2/2] 构建 X3Monitor"
if [[ -d "$X3_MONITOR_DIR" ]]; then
    (cd "$X3_MONITOR_DIR" && ./build.sh)
    echo "    构建完成: $X3_MONITOR_DIR/X3Monitor.app"
else
    echo "    !! 目录不存在: $X3_MONITOR_DIR"
fi

echo ""
echo "==> 全部完成。子端部署请使用: ./deploy_x3.sh / ./deploy_mini.sh"

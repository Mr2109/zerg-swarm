#!/usr/bin/env bash
# run_all.sh — 虫族 Agent 测试统一入口（2026-08-13）
# 用法: ./run_all.sh <模型> [版本标签]
# 例:   ./run_all.sh example-35b v2.5
# 作用: 跑全套测试 + 存档到 zerg-evals/results/agent/runs/<模型>-<版本>/

set -e
MODEL="${1:-example-35b}"
VERSION="${2:-$(date +%Y%m%d-%H%M)}"
BASE="<repo>"
EVALS="<volume-path>"
RUN_DIR="$EVALS/results/agent/runs/$MODEL-$VERSION"

echo "🐛 虫族 Agent 测试——模型: $MODEL | 版本: $VERSION"
echo "📁 存档: $RUN_DIR"
mkdir -p "$RUN_DIR"

# 层1: 框架单测（Go 单元测试——不依赖模型）
echo ""
echo "=== 层1: 框架单测 ==="
(cd "$BASE/core" && go test ./internal/agent/ 2>&1 | tee "$RUN_DIR/layer1-unit.txt")
echo "✅ 层1 完成"

# 层2: 基础任务（6 项）
echo ""
echo "=== 层2: 基础任务 ==="
python3 scripts/v25_example-35b-v2_tasks.py --model "$MODEL" --workdir "/tmp/zerg-agent-$MODEL-basic" 2>&1 | tee "$RUN_DIR/layer2-basic.txt" || echo "⚠️ 层2 部分失败（见输出）"
echo "✅ 层2 完成"

# 层3: 复合任务（4 项）
echo ""
echo "=== 层3: 复合任务 ==="
python3 scripts/v25_complex_tasks.py --model "$MODEL" --workdir "/tmp/zerg-agent-$MODEL-complex" 2>&1 | tee "$RUN_DIR/layer3-complex.txt" || echo "⚠️ 层3 部分失败（见输出）"
echo "✅ 层3 完成"

# 汇总
cat > "$RUN_DIR/summary.json" << EOF
{
  "model": "$MODEL",
  "version": "$VERSION",
  "date": "$(date '+%Y-%m-%d %H:%M')",
  "layers_run": ["layer1-unit", "layer2-basic", "layer3-complex"],
  "note": "完整结果见各 layer*.txt"
}
EOF

echo ""
echo "🎉 测试完成——存档: $RUN_DIR"
echo "查看: cat '$RUN_DIR/layer3-complex.txt'"

# 虫族 OCR 工具（看图）

> 截图自动识别——文字 + 坐标（位置关系）——开发调试必备

## 快速开始

```bash
# 命令模式（图片 OCR）
cd tools/ocr && ./venv/bin/python3 ocr_server.py --image 图片.png

# 截图 + OCR（全屏）
./venv/bin/python3 ocr_server.py --shot

# HTTP 常驻服务（虫族工具调）
./venv/bin/python3 ocr_server.py --server --port 8790
```

## API

```bash
curl -X POST http://127.0.0.1:8790/ocr \
  -H "Content-Type: application/json" \
  -d '{"image": "/path/to/img.png"}'
# → {"ok": true, "items": [{"text": "...", "box": [x1,y1,x2,y2], "score": 0.99}], "count": N, "elapsed_ms": 390}

# 或 base64 图片
curl -X POST http://127.0.0.1:8790/ocr -d '{"image_b64": "..."}'
```

## 架构（分层）

```
Layer 1 截图: 系统命令（screencapture/import/PowerShell）——零依赖
Layer 2 OCR 快层: RapidOCR（ONNX——MIT——毫秒级文字+坐标）✅ 已实现
Layer 3 布局层: PaddleOCR PP-StructureV3（PP-DocLayout——分类+阅读顺序）🔄 开发中
Layer 4 视觉层: qwen3.8（语义理解——OCR 不够时）✅ 已验证
```

## 位置关系

- box = [左上x, 左上y, 右下x, 右下y]——像素坐标
- 布局分析（Paddle）: 元素分类（文本/标题/表格）+ 阅读顺序——UI 结构

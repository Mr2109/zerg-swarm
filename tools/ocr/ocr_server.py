#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
虫族 OCR 服务（v0.1——RapidOCR 快层）
====================================
功能: 图片 → 文字识别 + 坐标（bounding box）——位置关系
用法:
  - 服务模式: python3 ocr_server.py --port 8790（常驻——虫族工具调）
  - 命令模式: python3 ocr_server.py --image /path/to/shot.png

API:
  POST /ocr  {"image": "/path/to/img.png"} 或 {"image_b64": "..."}
  → {"items": [{"text": "按钮", "box": [x1,y1,x2,y2], "score": 0.98}, ...]}
  → box = [左上x, 左上y, 右下x, 右下y]——位置关系（相对图片像素）

截图（内置——跨平台）:
  python3 ocr_server.py --shot → 截全屏 → /tmp/zerg-shot.png → OCR
"""
import argparse
import base64
import json
import subprocess
import sys
import time

# 截图命令（跨平台）
def take_screenshot(out_path: str) -> bool:
    import platform
    sys_name = platform.system()
    try:
        if sys_name == "Darwin":  # macOS
            subprocess.run(["screencapture", "-x", out_path], check=True)
        elif sys_name == "Linux":
            subprocess.run(["import", "-window", "root", out_path], check=True)
        elif sys_name == "Windows":
            import ctypes
            # PowerShell 截图（简化——全屏）
            ps = (
                "Add-Type -AssemblyName System.Windows.Forms;"
                "$b=[System.Windows.Forms.Screen]::PrimaryScreen.Bounds;"
                "$bmp=New-Object System.Drawing.Bitmap($b.Width,$b.Height);"
                "$g=[System.Drawing.Graphics]::FromImage($bmp);"
                "$g.CopyFromScreen($b.Location,[System.Drawing.Point]::Empty,$b.Size);"
                f"$bmp.Save('{out_path}')"
            )
            subprocess.run(["powershell", "-c", ps], check=True)
        else:
            print("不支持的平台截图")
            return False
        return True
    except Exception as e:
        print(f"截图失败: {e}", file=sys.stderr)
        return False


# RapidOCR 引擎（惰性初始化——常驻）
_engine = None

def get_engine():
    global _engine
    if _engine is None:
        from rapidocr_onnxruntime import RapidOCR
        _engine = RapidOCR()
    return _engine


# PaddleOCR 布局引擎（PP-DocLayout——惰性初始化）
_layout_engine = None

def get_layout_engine():
    global _layout_engine
    if _layout_engine is None:
        import warnings
        warnings.filterwarnings("ignore")
        from paddleocr import LayoutDetection
        # PP-DocLayout-M（平衡精度/速度——22.5MB）
        _layout_engine = LayoutDetection(model_name="PP-DocLayout-M")
    return _layout_engine


def layout_image(image_path: str) -> dict:
    """布局分析——元素分类 + 位置 + 阅读顺序（位置关系增强）"""
    engine = get_layout_engine()
    t0 = time.time()
    result = engine.predict(image_path, batch_size=1, layout_nms=True)
    elapsed = time.time() - t0
    items = []
    for res in result:
        res_dict = res.json if hasattr(res, "json") else res
        # 兼容不同返回格式
        boxes = res_dict.get("res", {}).get("boxes", []) if isinstance(res_dict, dict) else []
        if isinstance(res_dict, dict) and "boxes" in res_dict:
            boxes = res_dict["boxes"]
        for b in boxes:
            label = b.get("label", "unknown")
            score = b.get("score", 0.0)
            coord = b.get("coordinate", [])
            order = b.get("order", -1)
            if len(coord) >= 4:
                items.append({
                    "label": label,
                    "box": coord,  # [x1, y1, x2, y2]
                    "order": order,  # 阅读顺序
                    "score": score,
                })
    return {
        "items": items,
        "count": len(items),
        "elapsed_ms": round(elapsed * 1000),
        "engine": "paddle-layout",
    }


def ocr_image(image_path: str) -> dict:
    """OCR 识别——返回文字+坐标"""
    engine = get_engine()
    t0 = time.time()
    result, _ = engine(image_path)
    elapsed = time.time() - t0
    items = []
    if result:
        for line in result:
            # line: [box(4点), text, score]
            box = line[0]  # [[x1,y1],[x2,y2],[x3,y3],[x4,y4]]
            text = line[1]
            score = float(line[2]) if len(line) > 2 else 0.0
            # 归一化 bounding box（左上+右下）
            xs = [p[0] for p in box]
            ys = [p[1] for p in box]
            items.append({
                "text": text,
                "box": [min(xs), min(ys), max(xs), max(ys)],
                "score": score,
            })
    return {
        "items": items,
        "count": len(items),
        "elapsed_ms": round(elapsed * 1000),
        "engine": "rapidocr",
    }


def run_http_server(port: int):
    """HTTP 常驻服务（虫族工具调）"""
    from http.server import BaseHTTPRequestHandler, HTTPServer

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            if self.path in ("/ocr", "/layout"):
                length = int(self.headers.get("Content-Length", 0))
                body = json.loads(self.rfile.read(length))
                image_path = body.get("image", "")
                image_b64 = body.get("image_b64", "")
                if image_b64:
                    import io
                    from PIL import Image
                    tmp = "/tmp/zerg-ocr-input.png"
                    Image.open(io.BytesIO(base64.b64decode(image_b64))).save(tmp)
                    image_path = tmp
                try:
                    if self.path == "/layout":
                        result = layout_image(image_path)
                    else:
                        result = ocr_image(image_path)
                    resp = json.dumps({"ok": True, **result}).encode()
                except Exception as e:
                    resp = json.dumps({"ok": False, "error": str(e)}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(resp)))
                self.end_headers()
                self.wfile.write(resp)
            else:
                self.send_response(404)
                self.end_headers()

        def log_message(self, *args):
            pass  # 安静

    print(f"🐛 虫族 OCR 服务（RapidOCR + Paddle 布局）: http://127.0.0.1:{port}/ocr | /layout")
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()


def main():
    parser = argparse.ArgumentParser(description="虫族 OCR 服务")
    parser.add_argument("--port", type=int, default=8790, help="HTTP 端口（服务模式）")
    parser.add_argument("--image", type=str, default="", help="图片路径（命令模式）")
    parser.add_argument("--shot", action="store_true", help="截图后 OCR（命令模式）")
    parser.add_argument("--layout", action="store_true", help="布局分析（位置关系）")
    parser.add_argument("--server", action="store_true", help="HTTP 常驻服务")
    args = parser.parse_args()

    if args.server:
        run_http_server(args.port)
        return

    image_path = args.image
    if args.shot:
        image_path = "/tmp/zerg-shot.png"
        if not take_screenshot(image_path):
            sys.exit(1)
        print(f"📸 截图: {image_path}")

    if not image_path:
        print("用法: --server 或 --image <路径> 或 --shot")
        sys.exit(1)

    if args.layout:
        result = layout_image(image_path)
    else:
        result = ocr_image(image_path)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()

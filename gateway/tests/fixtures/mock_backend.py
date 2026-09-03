#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""mock_backend.py — 假 llama-server（测试用 mock 后端进程）。

由 agent.py 通过注册表 cmd 启动（`{port}` 占位符被替换），模拟真实后端的
/health、/v1/models、/v1/chat/completions（含 SSE 流式），并支持注入
启动延迟 / 推理延迟 / 健康检查失败等故障场景。
"""

import argparse
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class MockBackendHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    backend = None  # MockBackend 实例引用

    def log_message(self, fmt, *args):
        pass

    def _send(self, status, body_bytes, content_type="application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body_bytes)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(body_bytes)

    def do_GET(self):
        path = self.path.split("?")[0]
        if path == "/health":
            if time.time() - self.backend.started_at < self.backend.health_delay:
                self._send(503, b'{"status":"not ready"}')
                return
            self._send(200, b'{"status":"ok"}')
        elif path == "/v1/models":
            self._send(200, json.dumps(
                {"object": "list", "data": [{"id": "mock-model", "object": "model"}]}
            ).encode())
        else:
            self._send(404, b'{"error":"not found"}')

    def do_POST(self):
        path = self.path.split("?")[0]
        if path != "/v1/chat/completions":
            self._send(404, b'{"error":"not found"}')
            return
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            req = json.loads(raw.decode("utf-8"))
        except Exception:
            req = {}
        if self.backend.infer_delay > 0:
            time.sleep(self.backend.infer_delay)

        stream = bool(req.get("stream", False))
        if stream:
            payload = (
                'data: {"id":"mock-stream","object":"chat.completion.chunk",'
                '"choices":[{"delta":{"role":"assistant","content":"mock"},"index":0}]}\n\n'
                "data: [DONE]\n\n"
            ).encode()
            self._send(200, payload, content_type="text/event-stream")
        else:
            payload = json.dumps({
                "id": "mock-completion",
                "object": "chat.completion",
                "created": int(time.time()),
                "model": req.get("model", "mock"),
                "choices": [{
                    "index": 0,
                    "message": {"role": "assistant", "content": "mock reply"},
                    "finish_reason": "stop",
                }],
                "usage": {"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
            }).encode()
            self._send(200, payload)


class MockBackend:
    def __init__(self, port, health_delay=0.0, infer_delay=0.0):
        self.port = port
        self.health_delay = health_delay
        self.infer_delay = infer_delay
        self.started_at = time.time()

    def serve_forever(self):
        handler = lambda *a, **kw: MockBackendHandler(*a, **kw)  # noqa: E731
        httpd = ThreadingHTTPServer(("127.0.0.1", self.port), handler)
        MockBackendHandler.backend = self
        httpd.serve_forever(poll_interval=0.5)


def main():
    p = argparse.ArgumentParser(description="mock llama-server（测试用）")
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--health-delay", type=float, default=0.0,
                   help="健康检查就绪前延迟（秒）")
    p.add_argument("--infer-delay", type=float, default=0.0,
                   help="每次推理请求处理延迟（秒）")
    args = p.parse_args()
    backend = MockBackend(args.port, health_delay=args.health_delay,
                          infer_delay=args.infer_delay)
    try:
        backend.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    sys.exit(main())

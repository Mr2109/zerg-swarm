#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""mock_controller.py — 假主控（测试用）。

模拟主控 :8580 的 fleet 端点: 接收子端心跳 /api/fleet/heartbeat 与
日志批量上报 /api/fleet/logs，校验 X-Auth-Token，全部记录在内存中供测试断言。
既可被测试 import（线程启动），也可独立运行（--port --token）。
"""

import argparse
import json
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class MockControllerHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    controller = None  # MockController 实例引用

    def log_message(self, fmt, *args):
        pass

    def _send_json(self, status, obj):
        body = json.dumps(obj).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        path = self.path.split("?")[0]
        if path == "/health":
            self._send_json(200, {"status": "ok"})
        elif path == "/api/fleet/status":
            self._send_json(200, {"machines": self.controller.statuses()})
        else:
            self._send_json(404, {"error": "not found"})

    def do_POST(self):
        path = self.path.split("?")[0]
        token = self.headers.get("X-Auth-Token")
        if token != self.controller.token:
            self._send_json(401, {"error": "unauthorized"})
            return
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            body = json.loads(raw.decode("utf-8"))
        except Exception:
            self._send_json(400, {"error": "invalid json"})
            return

        if path == "/api/fleet/heartbeat":
            self.controller.record_heartbeat(body)
            self._send_json(200, {"ok": True})
        elif path == "/api/fleet/logs":
            self.controller.record_logs(body)
            self._send_json(200, {"ok": True})
        else:
            self._send_json(404, {"error": "not found"})


class MockController:
    """内存版主控: 记录心跳与日志批次，供测试断言。"""

    def __init__(self, token="test-token", port=18581):
        self.token = token
        self.port = port
        self.heartbeats = []
        self.log_batches = []
        self._lock = threading.Lock()
        self._httpd = None
        self._thread = None

    def start(self):
        handler = lambda *a, **kw: MockControllerHandler(*a, **kw)  # noqa: E731
        self._httpd = ThreadingHTTPServer(("127.0.0.1", self.port), handler)
        MockControllerHandler.controller = self
        self._thread = threading.Thread(target=self._httpd.serve_forever,
                                        name="mock-controller", daemon=True)
        self._thread.start()
        return self

    def stop(self):
        if self._httpd:
            self._httpd.shutdown()
            self._httpd.server_close()

    def record_heartbeat(self, body):
        with self._lock:
            self.heartbeats.append(body)

    def record_logs(self, body):
        with self._lock:
            self.log_batches.append(body)

    def statuses(self):
        with self._lock:
            return [h.get("machine") for h in self.heartbeats]

    # ---- 测试断言辅助 ----

    def last_heartbeat(self):
        with self._lock:
            return self.heartbeats[-1] if self.heartbeats else None

    def all_log_messages(self):
        msgs = []
        with self._lock:
            for batch in self.log_batches:
                for entry in batch.get("logs", []):
                    msgs.append(entry)
        return msgs


def main():
    p = argparse.ArgumentParser(description="mock 主控（测试用）")
    p.add_argument("--port", type=int, default=18581)
    p.add_argument("--token", default="test-token")
    args = p.parse_args()
    ctrl = MockController(token=args.token, port=args.port).start()
    print(f"mock controller listening on :{args.port} token={args.token}")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass
    finally:
        ctrl.stop()
    return 0


if __name__ == "__main__":
    sys.exit(main())

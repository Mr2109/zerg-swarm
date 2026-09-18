#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""test_agent.py — 子端 Agent 单元测试（mock 后端 + mock 主控，不真实加载模型）。

覆盖（对应 protocol.md §1/§2 与 DESIGN v2.3 验收）:
  - Token 认证（无/错/对 → 401/401/200）
  - /status 字段完整性
  - /load: 成功 / 幂等 / 未知模型 404 / 内存不足 507 / 队列满 429 / 熔断 503 / 双后端 409
  - /infer: 非流式透传 / SSE 流式透传 / 崩溃自愈自动重载 / 等待超时 504
  - /unload: 停止后端
  - /infer-file: 预留 404
  - 心跳上报（mock 主控接收）与日志批量上报
"""

import importlib.util
import json
import os
import queue
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, HERE)
sys.path.insert(0, ROOT)

# agent/ 目录无 __init__.py，会被当作命名空间包；用 importlib 直接加载单文件
_agent_path = os.path.join(ROOT, "agent", "agent.py")
_spec = importlib.util.spec_from_file_location("agent_mod", _agent_path)
agent_mod = importlib.util.module_from_spec(_spec)
assert _spec.loader is not None
_spec.loader.exec_module(agent_mod)

from mock_controller import MockController  # noqa: E402

TOKEN = "test-token-123"
AGENT_PORT = 18100
CTRL_PORT = 18581
REGISTRY = os.path.join(HERE, "fixtures", "agent_models_mock.yaml")


class AgentTestCase(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        cls.controller = MockController(token=TOKEN, port=CTRL_PORT).start()
        cls.agent = agent_mod.Agent(
            token=TOKEN,
            machine="mock-mini",
            controller=f"http://127.0.0.1:{CTRL_PORT}",
            registry_path=REGISTRY,
            backend_names=["mock_backend.py"],
            ds4_bin="ds4-server",
            log_dir=tempfile.mkdtemp(prefix="agent-test-log-"),
            log_max_bytes=1024 * 1024,
            queue_max=3,
            queue_timeout=3,
            heartbeat_interval=1,
            host="127.0.0.1",
            port=AGENT_PORT,
        )
        cls.thread = threading.Thread(target=cls.agent.start,
                                      name="agent-under-test", daemon=True)
        cls.thread.start()
        # 等待 HTTP 就绪
        deadline = time.time() + 15
        while time.time() < deadline:
            try:
                cls._http("GET", "/status", token=TOKEN)
                break
            except Exception:
                time.sleep(0.2)
        else:
            raise RuntimeError("agent 未能在 15s 内就绪")

    @classmethod
    def tearDownClass(cls):
        cls.agent.stop()
        cls.controller.stop()

    def tearDown(self):
        # 等待后台 infer worker 处理完遗留任务，避免异步状态污染下一个测试
        deadline = time.time() + 10
        while time.time() < deadline:
            if self.agent._infer_queue.empty() and self.agent.active_requests == 0:
                break
            time.sleep(0.2)
        # 清空队列残留
        while not self.agent._infer_queue.empty():
            try:
                self.agent._infer_queue.get_nowait()
            except queue.Empty:
                break
        # 卸载当前模型（同时重置熔断计数）
        self.agent.backends.stop()

    # ---- HTTP 辅助 ----

    @classmethod
    def _http(cls, method, path, body=None, token=TOKEN, timeout=15):
        url = f"http://127.0.0.1:{AGENT_PORT}{path}"
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if token is not None:
            req.add_header("X-Auth-Token", token)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                raw = resp.read().decode("utf-8")
                return resp.status, resp.headers.get("Content-Type", ""), raw
        except urllib.error.HTTPError as e:
            raw = e.read().decode("utf-8")
            return e.code, e.headers.get("Content-Type", ""), raw

    def _post(self, path, body, token=TOKEN, timeout=15):
        return self._http("POST", path, body=body, token=token, timeout=timeout)

    def _get(self, path, token=TOKEN):
        return self._http("GET", path, token=token)

    def _wait_backend_gone(self, pid, timeout=15):
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                os.kill(pid, 0)
            except OSError:
                return True
            time.sleep(0.2)
        return False

    # ---- Token 认证 ----

    def test_token_required(self):
        status, _, raw = self._get("/status", token=None)
        self.assertEqual(status, 401)
        self.assertIn("unauthorized", raw)
        status, _, _ = self._get("/status", token="wrong-token")
        self.assertEqual(status, 401)
        status, _, _ = self._get("/status", token=TOKEN)
        self.assertEqual(status, 200)

    # ---- /status ----

    def test_status_fields(self):
        status, _, raw = self._get("/status")
        self.assertEqual(status, 200)
        snap = json.loads(raw)
        for key in ("machine", "model", "backend", "port", "mem_available_gb",
                    "mem_total_gb", "load", "models", "uptime", "gpu_used_gb",
                    "backend_rss_gb", "active_requests", "healthy", "error"):
            self.assertIn(key, snap, f"缺失字段 {key}")
        self.assertEqual(snap["machine"], "mock-mini")
        self.assertIn("mock-small", snap["models"])

    # ---- /load ----

    def test_load_unknown_model_404(self):
        status, _, raw = self._post("/load", {"model": "no-such-model"})
        self.assertEqual(status, 404)
        self.assertIn("unknown model", raw)

    def test_load_missing_model_400(self):
        status, _, _ = self._post("/load", {})
        self.assertEqual(status, 400)

    def test_load_success(self):
        status, _, raw = self._post("/load", {"model": "mock-small"})
        self.assertEqual(status, 200)
        result = json.loads(raw)
        self.assertTrue(result["ok"])
        self.assertEqual(result["model"], "mock-small")
        self.assertIn("port", result)
        backend = self.agent.backends.current
        self.assertIsNotNone(backend)
        self.assertEqual(backend.model, "mock-small")
        self.assertTrue(backend.is_alive())
        # 幂等: 已加载同模型直接 ok
        status2, _, raw2 = self._post("/load", {"model": "mock-small"})
        self.assertEqual(status2, 200)
        self.assertTrue(json.loads(raw2)["ok"])

    def test_load_insufficient_memory_507(self):
        status, _, raw = self._post("/load", {"model": "mock-huge"})
        self.assertEqual(status, 507)
        body = json.loads(raw)
        self.assertIn("insufficient memory", body.get("error", ""))
        self.assertIn("available", body)
        self.assertIn("required", body)

    def test_load_circuit_breaker_503(self):
        # 前 2 次启动失败 → 500；第 3 次失败后熔断 → 503
        s1, _, _ = self._post("/load", {"model": "mock-crash"})
        s2, _, _ = self._post("/load", {"model": "mock-crash"})
        self.assertIn(s1, (500, 503))
        self.assertIn(s2, (500, 503))
        s3, _, raw3 = self._post("/load", {"model": "mock-crash"})
        self.assertEqual(s3, 503)
        self.assertIn("circuit open", raw3)
        # 熔断后仍拒绝
        s4, _, raw4 = self._post("/load", {"model": "mock-crash"})
        self.assertEqual(s4, 503)
        self.assertIn("circuit open", raw4)
        # 熔断状态反映在 /status
        _, _, status_raw = self._get("/status")
        self.assertFalse(json.loads(status_raw)["healthy"])
        # 卸载后重置熔断
        self._post("/unload", {})
        _, _, status_raw2 = self._get("/status")
        self.assertTrue(json.loads(status_raw2)["healthy"])

    def test_load_another_backend_running_409(self):
        # 手动起一个 mock 后端进程（agent 不管理它）→ 双后端检测 → 409
        proc = subprocess.Popen(
            [sys.executable, os.path.join(HERE, "fixtures", "mock_backend.py"),
             "--port", "18199"],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            time.sleep(1.0)  # 等进程起来
            status, _, raw = self._post("/load", {"model": "mock-small"})
            self.assertEqual(status, 409)
            self.assertIn("another backend running", raw)
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()

    def test_unload(self):
        self._post("/load", {"model": "mock-small"})
        backend = self.agent.backends.current
        self.assertIsNotNone(backend)
        pid = backend.pid
        status, _, raw = self._post("/unload", {})
        self.assertEqual(status, 200)
        self.assertTrue(json.loads(raw)["ok"])
        self.assertIsNone(self.agent.backends.current)
        self.assertTrue(self._wait_backend_gone(pid))
        _, _, status_raw = self._get("/status")
        snap = json.loads(status_raw)
        self.assertIsNone(snap["model"])
        self.assertTrue(snap["healthy"])

    # ---- /infer ----

    def test_infer_forward_non_stream(self):
        self._post("/load", {"model": "mock-small"})
        status, ctype, raw = self._post("/infer", {
            "model": "mock-small",
            "messages": [{"role": "user", "content": "hi"}],
        })
        self.assertEqual(status, 200)
        self.assertIn("application/json", ctype)
        self.assertIn("mock reply", raw)

    def test_infer_stream(self):
        self._post("/load", {"model": "mock-small"})
        status, ctype, raw = self._post("/infer", {
            "model": "mock-small",
            "messages": [{"role": "user", "content": "hi"}],
            "stream": True,
        })
        self.assertEqual(status, 200)
        self.assertIn("text/event-stream", ctype)
        self.assertIn("data: [DONE]", raw)

    def test_infer_auto_load_crash_recovery(self):
        """崩溃自愈: 加载后杀后端进程，下次 infer 自动重载并成功。"""
        self._post("/load", {"model": "mock-small"})
        old_pid = self.agent.backends.current.pid
        os.kill(old_pid, signal.SIGKILL)
        self.assertTrue(self._wait_backend_gone(old_pid))
        status, _, raw = self._post("/infer", {
            "model": "mock-small",
            "messages": [{"role": "user", "content": "again"}],
        })
        self.assertEqual(status, 200)
        self.assertIn("mock reply", raw)
        new_backend = self.agent.backends.current
        self.assertIsNotNone(new_backend)
        self.assertNotEqual(new_backend.pid, old_pid)
        self.assertTrue(new_backend.is_alive())

    def test_infer_queue_full_429(self):
        # 用满队列替换 agent 的推理队列（worker 仍在旧队列消费，不触碰新队列），
        # 保证 HTTP /infer 的 put_nowait 必定队列满 → 429
        dummy = {"body": {"model": "mock-small"}, "request_id": "fill",
                 "done": threading.Event(), "result": None}
        full_q = queue.Queue(maxsize=1)
        full_q.put_nowait(dummy)
        old_q = self.agent._infer_queue
        self.agent._infer_queue = full_q
        try:
            status, _, raw = self._post("/infer", {
                "model": "mock-small", "messages": []}, timeout=5)
            self.assertEqual(status, 429)
            self.assertIn("queue full", raw)
        finally:
            self.agent._infer_queue = old_q

    def test_infer_timeout_504(self):
        # mock-slow 后端 infer 延迟 8s > queue-timeout 3s → 504
        status, _, raw = self._post("/infer", {
            "model": "mock-slow", "messages": []}, timeout=10)
        self.assertEqual(status, 504)
        self.assertIn("inference timeout", raw)

    def test_infer_file_not_implemented(self):
        status, _, raw = self._post("/infer-file", {"model": "mock-small"})
        self.assertEqual(status, 404)

    # ---- 心跳与日志上报 ----

    def test_heartbeat_reported(self):
        deadline = time.time() + 15
        last = None
        while time.time() < deadline:
            last = self.controller.last_heartbeat()
            if last is not None:
                break
            time.sleep(0.5)
        self.assertIsNotNone(last, "未收到心跳")
        self.assertEqual(last["machine"], "mock-mini")
        self.assertIn("mem_available_gb", last)
        self.assertIn("models", last)

    def test_logs_reported(self):
        deadline = time.time() + 15
        msgs = []
        while time.time() < deadline:
            msgs = self.controller.all_log_messages()
            if msgs:
                break
            time.sleep(0.5)
        self.assertTrue(msgs, "未收到日志上报")
        entry = msgs[0]
        for key in ("ts", "level", "component", "message"):
            self.assertIn(key, entry)
        self.assertIn(entry["level"], ("DEBUG", "INFO", "WARN", "ERROR"))

    # ---- Token 错误拒绝 ----

    def test_heartbeat_bad_token_rejected(self):
        # 手动向 mock controller 发错误 token → 401（验证 controller 侧校验）
        req = urllib.request.Request(
            f"http://127.0.0.1:{CTRL_PORT}/api/fleet/heartbeat",
            data=b"{}", headers={"Content-Type": "application/json",
                                 "X-Auth-Token": "bad"},
            method="POST")
        try:
            urllib.request.urlopen(req, timeout=5)
            self.fail("错误 token 应 401")
        except urllib.error.HTTPError as e:
            self.assertEqual(e.code, 401)


if __name__ == "__main__":
    unittest.main(verbosity=2)

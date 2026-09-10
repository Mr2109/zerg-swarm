#!/usr/bin/env python3
"""虫族 Zerg MCP 服务（控制面）。

让 MCP 客户端（Hermes/Codex/龙虾）新会话自动发现并控制虫族集群。
- 数据面（模型推理）走 :8082 OpenAI 兼容，本服务不碰
- 控制面（fleet 状态/模型管理）薄封装主控 :8580 API

工具：
- fleet_status    : 所有机器/模型/健康/负载（只读）
- list_models     : 可用模型清单（只读）
- get_model_info  : 单模型详情（只读）
- fleet_topology  : 集群拓扑（只读）
- load_model      : 指定机器加载模型（写）
- unload_model    : 卸载模型（写）
- switch_model    : 切换模型（写）
- zerg_help       : 工具清单/用法

统一返回: {success, message, data}
"""

import json
import urllib.request
import urllib.error
import os

from mcp.server.fastmcp import FastMCP

# 主控地址（默认本机，可用环境变量覆盖）
def _zerg_token():
    """共享令牌（2026-09-11 A 批：库内零明文）——环境变量优先，其次 ~/.zerg/token"""
    import os as _os
    t = (_os.environ.get("ZERG_AUTH_TOKEN") or _os.environ.get("ZERG_API_TOKEN") or "").strip()
    if t:
        return t
    try:
        with open(_os.path.expanduser("~/.zerg/token"), encoding="utf-8") as f:
            return f.read().strip()
    except OSError:
        return ""


CONTROLLER = os.environ.get("ZERG_CONTROLLER", "http://127.0.0.1:8580")
TOKEN = os.environ.get("ZERG_TOKEN", _zerg_token())

mcp = FastMCP("zerg")


def _call(path: str, method: str = "GET", body: dict | None = None, timeout: int = 30) -> dict:
    """调用主控 API，返回统一 {success, message, data}。"""
    url = f"{CONTROLLER}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("X-Auth-Token", TOKEN)
    if body:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode()
            return {"success": True, "message": "ok", "data": json.loads(raw) if raw else {}}
    except urllib.error.HTTPError as e:
        msg = e.read().decode()[:300]
        return {"success": False, "message": f"主控返回 {e.code}: {msg}", "data": {}}
    except Exception as e:
        return {"success": False, "message": f"连接主控失败: {e}", "data": {}}


@mcp.tool()
def zerg_help() -> str:
    """虫族 MCP 工具清单与用法。"""
    return json.dumps({
        "success": True,
        "message": "虫族控制面工具：fleet_status / list_models / get_model_info / fleet_topology / load_model / unload_model / switch_model",
        "data": {
            "fleet_status": "查看所有机器/模型/健康状态（只读）",
            "list_models": "列出可用模型（只读）",
            "get_model_info": {"model": "模型名", "desc": "单模型详情（候选机器/文件/内存）"},
            "fleet_topology": "集群拓扑（本机/X3/mini 连接）",
            "load_model": {"machine": "x3|local|mini1", "model": "模型名", "desc": "指定机器加载模型"},
            "unload_model": {"machine": "x3|local|mini1", "model": "模型名", "desc": "卸载模型"},
            "switch_model": {"machine": "x3|local|mini1", "model": "模型名", "desc": "切换模型（加载新+卸载旧）"},
        },
    }, ensure_ascii=False)


@mcp.tool()
def fleet_status() -> str:
    """查看虫族集群所有机器的状态（健康/模型/负载/内存）。只读。"""
    r = _call("/api/fleet/status")
    if not r["success"]:
        return json.dumps(r, ensure_ascii=False)
    d = r["data"]
    machines = d.get("machines", {})
    summary = []
    for name, m in machines.items():
        summary.append({
            "machine": name,
            "healthy": m.get("healthy"),
            "state": m.get("backend_state"),
            "model": m.get("model"),
            "mem_available_gb": round(m.get("mem_available_gb", 0), 1),
            "active_requests": m.get("active_requests"),
        })
    r["data"] = {
        "healthy_count": d.get("healthy_count"),
        "available_models_count": len(d.get("available_models", [])),
        "machines": summary,
    }
    return json.dumps(r, ensure_ascii=False)


@mcp.tool()
def list_models() -> str:
    """列出虫族可用模型清单（含各模型候选机器）。只读。"""
    r = _call("/api/fleet/status")
    if not r["success"]:
        return json.dumps(r, ensure_ascii=False)
    d = r["data"]
    models = d.get("available_models", [])
    # 补充各模型候选机器（从 machines 的 models 字段推断）
    machine_models = {}
    for name, m in d.get("machines", {}).items():
        machine_models[name] = m.get("models", [])
    r["data"] = {
        "count": len(models),
        "models": models,
        "machine_models": machine_models,
    }
    return json.dumps(r, ensure_ascii=False)


@mcp.tool()
def get_model_info(model: str) -> str:
    """查看单个模型的详情（候选机器/状态）。只读。参数 model: 模型名。"""
    r = _call("/api/fleet/status")
    if not r["success"]:
        return json.dumps(r, ensure_ascii=False)
    d = r["data"]
    available = d.get("available_models", [])
    if model not in available:
        return json.dumps({"success": False, "message": f"模型 {model} 不在可用列表（共 {len(available)} 个）", "data": {"available": available}}, ensure_ascii=False)
    # 找哪台机器加载了/能跑
    info = {"model": model, "available": True}
    for name, m in d.get("machines", {}).items():
        if m.get("model") == model:
            info["loaded_on"] = name
            info["state"] = m.get("backend_state")
    if "loaded_on" not in info:
        info["loaded_on"] = None
        info["state"] = "not_loaded"
    return json.dumps({"success": True, "message": f"模型 {model} 详情", "data": info}, ensure_ascii=False)


@mcp.tool()
def fleet_topology() -> str:
    """查看虫族集群拓扑（机器地址/直连方式）。只读。"""
    r = _call("/api/fleet/status")
    if not r["success"]:
        return json.dumps(r, ensure_ascii=False)
    machines = r["data"].get("machines", {})
    topo = {}
    for name, m in machines.items():
        topo[name] = {
            "healthy": m.get("healthy"),
            "model": m.get("model"),
            "mem_available_gb": round(m.get("mem_available_gb", 0), 1),
        }
    # 补充说明：本机 en0=<controller-ip>(网线X3), en9=<worker-ip>(雷电mini1待配), X3=<worker-ip>
    return json.dumps({"success": True, "message": "集群拓扑（直连为主，WiFi 已放弃）", "data": {
        "machines": topo,
        "connectivity": {
            "local_en0": "<controller-ip> (网线直连 X3)",
            "x3_eno1": "<worker-ip> (网线直连本机)",
            "mini1": "<worker-ip> (雷电桥，待验证)",
        },
    }}, ensure_ascii=False)


@mcp.tool()
def load_model(machine: str, model: str) -> str:
    """在指定机器加载模型。参数 machine: x3|local|mini1；model: 模型名。"""
    if machine not in ("x3", "local", "mini1"):
        return json.dumps({"success": False, "message": f"machine 必须是 x3/local/mini1，收到 {machine}", "data": {}}, ensure_ascii=False)
    r = _call("/api/control/load", method="POST", body={"machine": machine, "model": model}, timeout=120)
    return json.dumps(r, ensure_ascii=False)


@mcp.tool()
def unload_model(machine: str, model: str) -> str:
    """卸载指定机器上的模型。参数 machine: x3|local|mini1；model: 模型名。"""
    if machine not in ("x3", "local", "mini1"):
        return json.dumps({"success": False, "message": f"machine 必须是 x3/local/mini1，收到 {machine}", "data": {}}, ensure_ascii=False)
    r = _call("/api/control/unload", method="POST", body={"machine": machine, "model": model}, timeout=60)
    return json.dumps(r, ensure_ascii=False)


@mcp.tool()
def switch_model(machine: str, model: str) -> str:
    """切换指定机器上的模型（加载新模型）。参数 machine: x3|local|mini1；model: 新模型名。"""
    if machine not in ("x3", "local", "mini1"):
        return json.dumps({"success": False, "message": f"machine 必须是 x3/local/mini1，收到 {machine}", "data": {}}, ensure_ascii=False)
    r = _call("/api/control/load", method="POST", body={"machine": machine, "model": model}, timeout=120)
    return json.dumps(r, ensure_ascii=False)


if __name__ == "__main__":
    mcp.run()

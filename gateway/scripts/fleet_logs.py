#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""fleet_logs.py — 主控侧日志查询工具（DESIGN v2.3 §3.5）

调用主控 :8580 日志查询 API（protocol.md §3）:
  logs query  --machine edge-node --component agent --since 10m
  logs trace  <request_id>
  logs errors --since 1h
  logs stats  --model GLM-5.2
仅使用 Python 3 标准库。
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

DEFAULT_CONTROLLER = "http://127.0.0.1:8580"
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


def _load_token(token_file: str = "") -> str:
    """令牌装载（缺口 GAP-20260926-85）：一律**不走 argv** —— 显式给 `--token-file` 则读该 600 档，
    否则按环境变量（`ZERG_AUTH_TOKEN`/`ZERG_API_TOKEN`）→ `~/.zerg/token` 的既有顺序。
    ★ 值只在**进程内存**里，绝不出现在命令行实参（进程表可见 ✗）。"""
    if token_file:
        try:
            with open(os.path.expanduser(token_file), encoding="utf-8") as f:
                return f.read().strip()
        except OSError as e:
            print(f"读令牌文件失败：{token_file}: {e}", file=sys.stderr)
            sys.exit(2)
    return _zerg_token()


def api_get(base: str, token: str, path: str, params: dict) -> dict:
    url = f"{base}{path}"
    if params:
        qs = "&".join(f"{k}={urllib.request.quote(str(v))}" for k, v in params.items() if v)
        url += f"?{qs}"
    req = urllib.request.Request(url, headers={"X-Auth-Token": token})
    with urllib.request.urlopen(req, timeout=10) as resp:
        return json.loads(resp.read().decode("utf-8"))


def cmd_query(args, base, token):
    rows = api_get(base, token, "/api/fleet/logs/query", {
        "machine": args.machine, "component": args.component,
        "since": args.since, "level": args.level,
    })
    _print_rows(rows)


def cmd_trace(args, base, token):
    rows = api_get(base, token, "/api/fleet/logs/trace", {"request_id": args.request_id})
    _print_rows(rows, chain=True)


def cmd_errors(args, base, token):
    rows = api_get(base, token, "/api/fleet/logs/errors", {"since": args.since})
    _print_rows(rows)


def cmd_stats(args, base, token):
    stats = api_get(base, token, "/api/fleet/logs/stats", {"model": args.model})
    print(f"模型 {args.model} 日志统计:")
    if not stats:
        print("  （无日志）")
    for level in ("DEBUG", "INFO", "WARN", "ERROR"):
        if level in stats:
            print(f"  {level}: {stats[level]}")


def _print_rows(rows, chain: bool = False):
    if not rows:
        print("（无匹配日志）")
        return
    for r in rows:
        rid = r.get("request_id") or "-"
        model = r.get("model") or "-"
        line = (f"{r.get('ts','')} [{r.get('level','')}] "
                f"[{r.get('machine','')}/{r.get('component','')}] "
                f"model={model} request_id={rid} {r.get('message','')}")
        if chain:
            line = f"  {line}"
        print(line)


def main(argv=None):
    p = argparse.ArgumentParser(description="Fleet 日志查询工具（调主控 :8580）")
    p.add_argument("--controller", default=DEFAULT_CONTROLLER, help=f"主控地址（默认 {DEFAULT_CONTROLLER}）")
    p.add_argument("--token-file", default="", help="共享 Token 的 600 档文件（令牌**不走 argv**；不给则读环境变量 / ~/.zerg/token）")

    sub = p.add_subparsers(dest="command", required=True)

    q = sub.add_parser("query", help="按条件查询日志")
    q.add_argument("--machine", help="机器名（edge-node/mini-node-1/...）")
    q.add_argument("--component", help="组件（agent/backend/health/heartbeat/fleet）")
    q.add_argument("--since", help="起始时间（10m/1h/30s 或 ISO8601）")
    q.add_argument("--level", help="级别（DEBUG/INFO/WARN/ERROR）")
    q.set_defaults(func=cmd_query)

    t = sub.add_parser("trace", help="按 request_id 拉全链路日志")
    t.add_argument("request_id", help="请求 ID（UUID）")
    t.set_defaults(func=cmd_trace)

    e = sub.add_parser("errors", help="只看错误")
    e.add_argument("--since", default="1h", help="起始时间（默认 1h）")
    e.set_defaults(func=cmd_errors)

    s = sub.add_parser("stats", help="某模型的加载/卸载/失败统计")
    s.add_argument("--model", required=True, help="模型名")
    s.set_defaults(func=cmd_stats)

    args = p.parse_args(argv)
    token = _load_token(args.token_file)
    try:
        args.func(args, args.controller.rstrip("/"), token)
    except urllib.error.HTTPError as e:
        print(f"API 错误 {e.code}: {e.read().decode('utf-8', errors='replace')}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"调用失败（主控 :8580 是否运行？）: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

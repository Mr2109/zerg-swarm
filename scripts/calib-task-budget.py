#!/usr/bin/env python3
"""预算标定（§8.4 标定铁律）：跑 N 次**小号无害任务**，量出"一次任务需要多少 步数/token/时间"。

靶子任务（零副作用）：读一个已知文件并贴出指定行 —— 不写任何东西 ✓
记录来源：① 观测面 chat_obs.jsonl（turn 的 tokens/耗时 + tool 的条数）② 会话 messages（正文长度）
产出：~/.zerg/egg-profiles/task-budget-calib.yaml（三次数 + 上界），闸门日后只读它。
"""
import json
import os
import time
import urllib.request

CORE = os.environ.get("ZERG_CORE", "http://127.0.0.1:8580")
TOK = open(os.path.expanduser("~/.zerg/token")).read().strip()
STATE = os.path.expanduser("~/.zerg/state/chat_obs.jsonl")
TASK = "请读 core/internal/version/version.go 这个文件，并把你看到的 version 常量那一行原文贴出来。只做这一步。"
MODEL = os.environ.get("ZERG_CALIB_MODEL", "Qwen3.8-27B")
RUNS = int(os.environ.get("ZERG_CALIB_RUNS", "3"))


def post(path, body, timeout=1200):
    req = urllib.request.Request(
        CORE + path, data=json.dumps(body).encode(),
        method="POST",
        headers={"Content-Type": "application/json", "X-Auth-Token": TOK},
    )
    return urllib.request.urlopen(req, timeout=timeout)


def get(path, timeout=60):
    req = urllib.request.Request(CORE + path, headers={"X-Auth-Token": TOK})
    body = urllib.request.urlopen(req, timeout=timeout).read().decode()
    try:
        return json.loads(body)
    except Exception:
        return None


def tail_obs(offset):
    """返回 offset 之后新增的观测记录（turn 的 tokens/耗时 + tool 计数）。"""
    if not os.path.exists(STATE):
        return [], offset
    out, seen = [], 0
    with open(STATE, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            seen += 1
            if seen > offset:
                try:
                    out.append(json.loads(line))
                except Exception:
                    pass
    return out, seen


def run_once(idx):
    d = post("/api/chat/sessions", {"title": f"[calib]task-budget-{idx}", "source": "calib"}).read().decode()
    sid = json.loads(d).get("session_id") or json.loads(d).get("id")
    post(f"/api/chat/sessions/{sid}/model", {"model": MODEL}).read()
    before, _ = tail_obs(0)
    off = len(before)
    t0 = time.time()
    with post(f"/api/chat/sessions/{sid}/send", {"content": TASK}) as resp:
        events = sum(1 for _ in resp)
    wall = time.time() - t0
    new, _ = tail_obs(off)
    turns = [r for r in new if r.get("kind") == "turn"]
    tools = [r for r in new if r.get("kind") == "tool"]
    steps = max((r.get("round") or 0) for r in turns) if turns else 0
    tokens_in = sum((r.get("turn") or {}).get("first_byte_ms", 0) for r in [])  # 占位：token 由 usage 提供
    # token 从会话消息接口拿（更可靠）
    msgs = get(f"/api/chat/sessions/{sid}/messages")
    # 修：接口可能返回 null 或 {"messages": null} ⇒ 一律兜成空列表（我上一跑的 run3 就栽在这 ✗）
    if isinstance(msgs, list):
        arr = msgs
    elif isinstance(msgs, dict):
        arr = msgs.get("messages") or []
    else:
        arr = []
    out_tok = sum(len((m.get("content") or "")) for m in arr if m.get("role") == "assistant") // 4
    return {
        "run": idx, "session": sid, "steps": steps, "tool_calls": len(tools),
        "wall_s": round(wall, 1), "events": events,
        "assistant_chars": sum(len((m.get("content") or "")) for m in arr if m.get("role") == "assistant"),
        "assistant_tokens_est": out_tok,
        "end_reasons": sorted({(r.get("end_reason") or "") for r in turns}),
        "verdicts": sorted({((r.get("turn") or {}).get("verdict") or "") for r in turns}),
        "queued_ms_max": max([(r.get("queued_ms") or 0) for r in new if r.get("kind") == "queued"] or [0]),
    }


def main():
    rows = []
    for i in range(1, RUNS + 1):
        print(f"[calib] 第 {i}/{RUNS} 次 …", flush=True)
        try:
            r = run_once(i)
            rows.append(r)
            print("   " + json.dumps(r, ensure_ascii=False), flush=True)
        except Exception as e:
            print(f"   ✗ 第 {i} 次失败：{e}", flush=True)
    if not rows:
        print("[calib] 无有效数据 —— 不写档案（宁缺勿造）")
        return
    ub = {
        "steps_max": max(r["steps"] for r in rows),
        "tool_calls_max": max(r["tool_calls"] for r in rows),
        "wall_s_max": max(r["wall_s"] for r in rows),
        "assistant_tokens_est_max": max(r["assistant_tokens_est"] for r in rows),
    }
    path = os.path.expanduser("~/.zerg/egg-profiles/task-budget-calib.yaml")
    with open(path, "w", encoding="utf-8") as f:
        f.write("# 任务级预算标定档案（§8.4：实测得出，闸门只读档案；无档案不许当已标定）\n")
        f.write(f"# 靶子任务：读 core/internal/version/version.go 并贴出 version 常量那一行（零副作用）\n")
        f.write(f"model: {MODEL}\n")
        f.write(f"calib_runs: {len(rows)}\n")
        f.write(f"measured_at: {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}\n")
        for k, v in ub.items():
            f.write(f"{k}: {v}\n")
        f.write("# 明细（三次逐条）\n")
        for r in rows:
            f.write(f"#   run={r['run']} session={r['session']} steps={r['steps']} tools={r['tool_calls']} "
                    f"wall={r['wall_s']}s verdicts={r['verdicts']} end={r['end_reasons']} queued_max={r['queued_ms_max']}\n")
    print(f"[calib] ✓ 已写档案：{path}")
    print(json.dumps(ub, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()

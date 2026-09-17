#!/usr/bin/env python3
"""A/B 实验（两个待实测课题）—— 用本机无害靶子，量出数字再下结论（§8.4 标定铁律）。

课题一：**打回重做（send_back）是帮助还是伤害**（Loop-Back Authority arXiv 2609.14767 与组织理论结论相左）
  A 组（对照）：无据主张 ⇒ 直接"降级/拆小步"（不打回）
  B 组（处理）：无据主张 ⇒ **打回重做**（要求带引用重交）
  判据：最终产出**可命中引用**与否 · 轮数 · 输出 token · 墙钟 · 是否被掐

课题二：**自愿应答 vs Lead 指派**（小池子 16 枚卵）
  甲档（指派）：任务由 Lead 直接指定给某枚卵
  乙档（自愿）：任务贴板，由成员按能力应答（本实验以"广播给 N 枚、先答者接单"近似）
  判据：完成率 · 单槽排队峰值 · 总墙钟 · 需人介入次数

纪律：靶子零副作用（只读文件并贴原文）· 每档 ≥3 次 · 结果落 JSON 与标定档案旁边 · 有错如实记不吞。
"""
import json
import os
import statistics
import time
import urllib.request

CORE = os.environ.get("ZERG_CORE", "http://127.0.0.1:8580")
TOK = open(os.path.expanduser("~/.zerg/token")).read().strip()
STATE = os.path.expanduser("~/.zerg/state/chat_obs.jsonl")
OUT = os.path.expanduser("~/.zerg/egg-profiles/ab-campaigns.json")
RUNS = int(os.environ.get("ZERG_AB_RUNS", "3"))

# 无害靶子（零副作用：只读、只贴一行）
TARGET = "core/internal/version/version.go"
TASK_A = f"请读 {TARGET}，把其中 version 常量那一行**原文**贴出来，并附上你读取它时那次工具调用的回执ID。只做这一步。"
TASK_B = f"请读 {TARGET}，把其中 version 常量那一行**原文**贴出来，并附上你读取它时那次工具调用的回执ID。只做这一步。"


def post(path, body, timeout=1200):
    req = urllib.request.Request(CORE + path, data=json.dumps(body).encode(), method="POST",
                                 headers={"Content-Type": "application/json", "X-Auth-Token": TOK})
    return urllib.request.urlopen(req, timeout=timeout)


def get(path, timeout=60):
    req = urllib.request.Request(CORE + path, headers={"X-Auth-Token": TOK})
    body = urllib.request.urlopen(req, timeout=timeout).read().decode()
    try:
        return json.loads(body)
    except Exception:
        return None


def obs_since(offset):
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


def count_obs():
    if not os.path.exists(STATE):
        return 0
    with open(STATE, encoding="utf-8") as f:
        return sum(1 for l in f if l.strip())


def messages(sid):
    m = get(f"/api/chat/sessions/{sid}/messages")
    if isinstance(m, list):
        return m
    if isinstance(m, dict):
        return m.get("messages") or []
    return []


def run_once(group, idx, task, model="Qwen3.8-27B"):
    d = post("/api/chat/sessions", {"title": f"[ab] {group}-{idx}", "source": "ab"}).read().decode()
    j = json.loads(d)
    sid = j.get("session_id") or j.get("id")
    post(f"/api/chat/sessions/{sid}/model", {"model": model}).read()
    off = count_obs()
    t0 = time.time()
    with post(f"/api/chat/sessions/{sid}/send", {"content": task}) as resp:
        events = sum(1 for _ in resp)
    wall = time.time() - t0
    new, _ = obs_since(off)
    turns = [r for r in new if r.get("kind") == "turn"]
    tools = [r for r in new if r.get("kind") == "tool"]
    queued = [r.get("queued_ms") or 0 for r in new if r.get("kind") == "queued"]
    msgs = messages(sid)
    assistant = [m for m in msgs if m.get("role") == "assistant"]
    body = "".join((m.get("content") or "") for m in assistant)
    # 判据一：最终产出是否含"可命中的原文引用"（靶子文件里 version 常量那行的特征串）
    hit = ("Version" in body) or ("version" in body and "2.5.9" in body)
    return {
        "group": group, "run": idx, "session": sid,
        "steps": max([r.get("round") or 0 for r in turns] or [0]),
        "tool_calls": len(tools),
        "wall_s": round(wall, 1),
        "events": events,
        "assistant_chars": len(body),
        "citation_hit": bool(hit),
        "end_reasons": sorted({(r.get("end_reason") or "") for r in turns}),
        "verdicts": sorted({((r.get("turn") or {}).get("verdict") or "") for r in turns}),
        "queued_max_ms": max(queued or [0]),
        "wrote_anything": any((t.get("tool") in ("write", "edit")) for t in tools),
    }


def summarize(rows):
    if not rows:
        return {}
    return {
        "n": len(rows),
        "citation_hit_rate": round(sum(1 for r in rows if r["citation_hit"]) / len(rows), 2),
        "wall_s_median": round(statistics.median([r["wall_s"] for r in rows]), 1),
        "tool_calls_median": statistics.median([r["tool_calls"] for r in rows]),
        "queued_max_ms": max(r["queued_max_ms"] for r in rows),
        "end_reasons": sorted({e for r in rows for e in r["end_reasons"]}),
    }


def main():
    results = {"run_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "runs_per_arm": RUNS, "arms": {}}
    plan = [("课题一A-不打回", "A_no_sendback", TASK_A), ("课题一B-打回重做", "B_sendback", TASK_B),
            ("课题二甲-指派", "C_assign", TASK_A), ("课题二乙-广播自愿", "D_broadcast", TASK_B)]
    for label, key, task in plan:
        rows = []
        for i in range(1, RUNS + 1):
            print(f"[ab] {label} 第 {i}/{RUNS} 次 …", flush=True)
            try:
                r = run_once(key, i, task)
                rows.append(r)
                print("   " + json.dumps(r, ensure_ascii=False), flush=True)
            except Exception as e:
                print(f"   ✗ 失败：{e}", flush=True)
        results["arms"][label] = {"rows": rows, "summary": summarize(rows)}
        print(f"   ⇒ 小结：{json.dumps(results['arms'][label]['summary'], ensure_ascii=False)}", flush=True)
    with open(OUT, "w", encoding="utf-8") as f:
        json.dump(results, f, ensure_ascii=False, indent=2)
    print(f"[ab] ✓ 已写：{OUT}")
    print("═══ 总览 ═══")
    for label, v in results["arms"].items():
        print(f"  {label}: {json.dumps(v['summary'], ensure_ascii=False)}")


if __name__ == "__main__":
    main()

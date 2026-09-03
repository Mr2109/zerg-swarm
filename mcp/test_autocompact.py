#!/usr/bin/env python3
"""测试：autoCompact 路径 LLMLingua-2 压缩效果（session 累计超阈值自动触发）。

方法：
1. 同一 session_id 连续发请求（每次 token 累计）
2. 埋针（关键值）在早期消息
3. 观察主控日志：LLMLingua-2 压缩是否触发
4. 压缩后会话继续，问针是否保留
"""
import json
import time
import urllib.request

GATEWAY = "http://127.0.0.1:8082"
TOKEN = "x3gw-shared-2026"
SESSION = "auto-compress-test-1"
MODEL = "example-35b"

def call_chat(messages, session_id=None, max_tokens=100):
    body = {"model": MODEL, "messages": messages, "max_tokens": max_tokens}
    if session_id:
        body["session_id"] = session_id
    req = urllib.request.Request(
        f"{GATEWAY}/v1/chat/completions",
        data=json.dumps(body).encode(),
        headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    try:
        d = json.loads(urllib.request.urlopen(req, timeout=300).read())
        m = d["choices"][0]["message"]
        return (m.get("content", "") or "") + " " + (m.get("reasoning_content", "") or "")
    except Exception as e:
        return f"ERROR: {e}"

def query_context():
    try:
        d = json.loads(urllib.request.urlopen(
            f"{GATEWAY}/v1/context/{SESSION}", timeout=30).read())
        return d
    except Exception as e:
        return {"error": str(e)}

# 1. 埋针：早期一条消息包含关键值
needle = "AX9-K7Q2-ZETA"
msgs = [{"role": "user", "content": f"记住关键值：{needle}，这是项目专属代号，很重要"}]
print(f"埋针: {needle}")

# 2. 连续发 6 轮，每轮带大文本（加速 token 累计）+ 保持针消息在历史里
big_pad = "今天天气不错，我们讨论了项目进展、预算分配、人员安排、技术选型、部署方案、性能优化、安全策略、测试计划、文档规范、代码评审等十个议题。" * 30  # ~1000字
for i in range(6):
    msgs.append({"role": "user", "content": big_pad})
    msgs.append({"role": "assistant", "content": f"第{i+1}轮：收到，已记录相关讨论内容，继续推进。"})
    r = call_chat(msgs, session_id=SESSION)
    ctx = query_context()
    print(f"轮{i+1}: 会话tokens={ctx.get('tokens', '?')} over_budget={ctx.get('over_budget')} 回答={r[:20]}")

# 3. 最终问针
final = msgs + [{"role": "user", "content": "之前让你记住的关键值是什么？直接回答"}]
r = call_chat(final, session_id=SESSION)
print(f"\n问针回答: {r[:100]}")
print(f"保针: {'✅' if needle in r else '❌'}")

# 4. 检查主控日志（压缩是否触发）
import subprocess
log = subprocess.run(["grep", "-E", "LLMLingua|压缩", "/tmp/zerg-core.log"],
                     capture_output=True, text=True).stdout
compress_lines = [l for l in log.split("\n") if "压缩" in l or "LLMLingua" in l]
print(f"\n压缩事件（日志）: {len(compress_lines)} 条")
for l in compress_lines[-3:]:
    print(" ", l[:90])

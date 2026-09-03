#!/usr/bin/env python3
"""演示快路径 TrimNoise（逐字精简）效果——纯规则，无 LLM。"""
import json

# ===== 模拟 TrimNoise 的 Go 规则（与 compactor.go 一致）=====
HEAD_LINES = 5   # 工具输出保留开头行数
TAIL_LINES = 3   # 工具输出保留结尾行数
MAX_LEN = 800    # 单个工具输出最大保留字符

FILLERS = ["好的", "收到", "明白了", "了解", "OK", "好的，记下来了", "已处理", "没问题"]

def mask_tool_output(content):
    """工具输出精简：只保留首尾 N 行，中间省略（一字不改保留的行）"""
    lines = content.split("\n")
    # 行数多 → 按行精简（首尾保留）
    if len(lines) > HEAD_LINES + TAIL_LINES + 1:
        head = "\n".join(lines[:HEAD_LINES])
        tail = "\n".join(lines[len(lines)-TAIL_LINES:])
        return head + f"\n...[省略 {len(lines)-HEAD_LINES-TAIL_LINES} 行]...\n" + tail, True
    # 行数少但单行超长 → 按字符截断中间
    if len(content) > MAX_LEN:
        head = content[:HEAD_LINES*40]
        tail = content[len(content)-TAIL_LINES*40:]
        return head + f"\n...[省略 {len(content)-MAX_LEN} 字符]...\n" + tail, True
    return content, False

def is_filler(content):
    return content.strip() in FILLERS or content.strip().endswith(("好的", "收到", "已处理"))

def trim_noise(messages):
    """与 Go TrimNoise 一致的规则"""
    out = []
    changed = False
    for m in messages:
        role = m.get("role", "")
        content = m.get("content", "")
        if role == "tool":
            new_c, c = mask_tool_output(content)
            if c:
                changed = True
            m2 = dict(m)
            m2["content"] = new_c
            out.append(m2)
        elif role == "assistant":
            if is_filler(content) and "tool_calls" not in m:
                changed = True
                continue  # 删除填充回复
            out.append(m)
        else:
            out.append(m)
    return out, changed

def show(title, messages):
    print(f"\n{'='*60}")
    print(f"📋 {title}")
    print(f"{'='*60}")
    trimmed, changed = trim_noise(messages)
    for i, m in enumerate(messages):
        role = m["role"]
        c = m["content"]
        shown = c if len(c) <= 200 else c[:100] + f"...(共{len(c)}字)"
        print(f"  [{i}] {role}: {shown}")
    print(f"\n  ⚡ 精简后 ({'✅ 有精简' if changed else '无变化'}):")
    for i, m in enumerate(trimmed):
        role = m["role"]
        c = m["content"]
        shown = c if len(c) <= 200 else c[:100] + f"...(共{len(c)}字)"
        print(f"  [{i}] {role}: {shown}")
    # 统计
    orig_chars = sum(len(m.get("content","")) for m in messages)
    new_chars = sum(len(m.get("content","")) for m in trimmed)
    pct = (orig_chars - new_chars) / orig_chars * 100 if orig_chars else 0
    print(f"\n  📊 字符: {orig_chars} → {new_chars} (省 {pct:.1f}%)")
    return pct

print("=" * 60)
print("演示：快路径 TrimNoise（逐字精简）")
print("纯规则 + 无 LLM + 毫秒级 + 幸存内容一字不改")
print("=" * 60)

# 演示 1：大工具输出（Codex 常见场景——ls/日志/测试输出）
show("演示 1：大工具输出（日志/命令输出）", [
    {"role": "user", "content": "运行测试看看结果"},
    {"role": "assistant", "content": "好的，运行 pytest", "tool_calls": [{"id": "t1", "type": "function", "function": {"name": "exec", "arguments": "{}"}}]},
    {"role": "tool", "tool_call_id": "t1", "content": "\n".join([
        "=== test run started ===",
        "test_auth.py::test_login PASSED",
        "test_auth.py::test_token PASSED",
        "test_auth.py::test_refresh FAILED",
        "  assert 401 == 200",
        "  at /app/tests/test_auth.py:47",
        "test_auth.py::test_logout PASSED",
        "test_user.py::test_create PASSED",
        "test_user.py::test_delete PASSED",
        "test_api.py::test_get PASSED",
        "test_api.py::test_post PASSED",
        "test_api.py::test_put PASSED",
        "test_api.py::test_delete PASSED",
        "test_api.py::test_404 PASSED",
        "test_api.py::test_500 PASSED",
        "test_db.py::test_connect PASSED",
        "test_db.py::test_query PASSED",
        "test_db.py::test_insert PASSED",
        "test_db.py::test_update PASSED",
        "test_db.py::test_migrate PASSED",
        "=== 1 failed, 19 passed in 12.4s ===",
    ])},
    {"role": "assistant", "content": "有一个测试失败：test_refresh 断言 401==200，在 test_auth.py:47"},
])

# 演示 2：填充回复删除
show("演示 2：assistant 填充回复（好的/收到）", [
    {"role": "user", "content": "帮我查一下 API 文档"},
    {"role": "assistant", "content": "好的"},
    {"role": "user", "content": "重点看认证部分"},
    {"role": "assistant", "content": "收到，我看一下认证部分"},
    {"role": "user", "content": "总结一下"},
    {"role": "assistant", "content": "认证部分要点：1. Bearer token 2. 过期刷新 3. 权限范围"},
])

# 演示 3：混合（短输出不精简 + 用户消息不动）
show("演示 3：短工具输出不精简 + 用户消息原文保留", [
    {"role": "user", "content": "查询用户 42 的信息"},
    {"role": "assistant", "content": "好的", "tool_calls": [{"id": "t2", "type": "function", "function": {"name": "db_query", "arguments": "{}"}}]},
    {"role": "tool", "tool_call_id": "t2", "content": "id=42, name=Alice, role=admin, email=alice@example.com"},
    {"role": "assistant", "content": "用户 42 是 Alice，管理员权限"},
])

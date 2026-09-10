#!/usr/bin/env python3
"""测试 1：针在草垛（Needle-in-Haystack）——压缩后关键信息保留。

方法：
1. 构造长对话（~1万字），在随机位置埋一个"针"（关键事实）
2. 调虫族 /v1/context/compact 压缩成摘要
3. 用 ornith 问摘要："针是什么？"
4. 答对 = 压缩保住信息；答错 = 压缩丢针
"""
import json
import random
import urllib.request

GATEWAY = "http://127.0.0.1:8082"
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


TOKEN = _zerg_token()

def call_chat(model, messages, max_tokens=300):
    body = json.dumps({"model": model, "messages": messages, "max_tokens": max_tokens}).encode()
    req = urllib.request.Request(f"{GATEWAY}/v1/chat/completions", data=body,
                                 headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    try:
        d = json.loads(urllib.request.urlopen(req, timeout=600).read())
        m = d["choices"][0]["message"]
        # content + reasoning 都拼上（思考模型答案可能在 reasoning）
        return (m.get("content", "") or "") + " " + (m.get("reasoning_content", "") or "")
    except Exception as e:
        return f"ERROR: {e}"

def call_compact(messages):
    body = json.dumps({"messages": messages}).encode()
    req = urllib.request.Request(f"{GATEWAY}/v1/context/compact", data=body,
                                 headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    d = json.loads(urllib.request.urlopen(req, timeout=600).read())
    return d.get("summary", "")

def build_long_dialogue(needle_text, filler_count=15):
    """构造长对话：filler 填充 + 一个 needle 埋在随机位置"""
    messages = []
    filler_templates = [
        "今天天气不错，讨论一下昨天的项目进展吧。",
        "我觉得这个方案可以再优化一下性能。",
        "关于接口设计，我们还有几个细节需要确认。",
        "测试通过了，不过边界条件还要补几个用例。",
        "文档更新了一版，你看看有什么遗漏。",
        "部署脚本有点问题，日志显示权限不对。",
        "这个需求优先级比较高，先安排人处理。",
        "数据表结构调整了，需要同步更新查询逻辑。",
        "版本号升到 2.0 了，发布前再检查一遍。",
        "反馈说新功能很好用，就是加载有点慢。",
        "安全审计发现两个低风险问题，下周修复。",
        "会议纪要我发到群里了，查收一下。",
        "代码评审通过了，可以合并到主分支。",
        "监控告警调低了阈值，误报减少了。",
        "下个迭代的重点是性能优化和稳定性。",
    ]
    # 随机埋针位置
    needle_pos = random.randint(3, len(filler_templates) - 2)
    for i in range(len(filler_templates)):
        if i == needle_pos:
            messages.append({"role": "user", "content": f"顺便说一下，{needle_text}"})
            messages.append({"role": "assistant", "content": "好的，记下来了。"})
        else:
            messages.append({"role": "user", "content": filler_templates[i]})
            messages.append({"role": "assistant", "content": f"收到，{filler_templates[i][:15]}...已处理。"})
    return messages, needle_text

def run_needle_test(needle, qa_question, expected_answer, rounds=3):
    """跑单次：埋针 → 压缩 → 问 → 判对错。rounds=连续压缩次数"""
    print(f"\n=== 针: {needle} ===")
    print(f"提问: {qa_question}")
    print(f"期望: {expected_answer}")

    messages, needle_text = build_long_dialogue(needle)
    print(f"原始对话: {len(messages)} 条消息")

    # 连续压缩 rounds 次（测"越压越精炼"信息衰减）
    summary = ""
    for r in range(rounds):
        source = [{"role": "user", "content": json.dumps(messages, ensure_ascii=False)}] if not summary else \
                 [{"role": "user", "content": f"之前的摘要: {summary}\n继续压缩成更精炼的摘要"}]
        summary = call_compact(source) or ""
        print(f"  压缩 {r+1} 次 → 摘要 {len(summary)} 字")

    # 问模型（直接回答，检查 content + reasoning 两个字段）
    ans = call_chat("Qwable-v1.Q5_K_M", [
        {"role": "system", "content": "根据以下摘要直接回答问题，不要思考过程，只输出答案。摘要:\n" + summary},
        {"role": "user", "content": qa_question},
    ], max_tokens=100)
    print(f"回答: {ans[:120]}")
    correct = expected_answer.lower() in ans.lower()
    print(f"结果: {'✅ 正确' if correct else '❌ 错误（可能思考中，看reasoning）'}")
    return correct

if __name__ == "__main__":
    print("=" * 60)
    print("测试 1：针在草垛（压缩保针）")
    print("=" * 60)

    # 测试 A：密码（事实记忆）
    r1 = run_needle_test(
        "保险箱的密码是 7329，放在书房书桌第二个抽屉里。",
        "保险箱密码是多少？",
        "7329",
        rounds=3  # 连续压缩 3 次
    )

    # 测试 B：人名/关系
    r2 = run_needle_test(
        "新来的项目经理叫王强，他之前是腾讯的架构师。",
        "新来的项目经理叫什么名字？",
        "王强",
        rounds=2
    )

    # 测试 C：数字/金额
    r3 = run_needle_test(
        "项目预算最终定为 500 万，分三期付款。",
        "项目预算是多少？",
        "500",
        rounds=1
    )

    print("\n" + "=" * 60)
    print(f"总结果: {sum([r1, r2, r3])}/3 通过")
    print("=" * 60)

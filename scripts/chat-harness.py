#!/usr/bin/env python3
# chat-harness.py — v2.5.10 对话/卵 测试骨架（最小可跑版）
#
# 设计：docs/01-设计/设计-v2.5.10-卵功能与对话机制-20260916.md
#       docs/项目文档/v2.5.10/靶子表与完成定义-20260916.md
#
# 三条纪律（写死在代码里）：
#   1) **驱动（"使用者"）= 本机模型**，直连本机子端 :8100/infer（不经路由 ⇒ 保证是"本机扮演人"）
#      被测 = 对话链路 :8580（X3/别处的模型经卵孵化后真跑）
#   2) **判据只认证据**：每轮后读 chat_obs.jsonl 的新增行，按 X1–X5 逐条断言（脚本层硬结论）
#   3) **不碰生产、不留脏**：只建自己的会话；本脚本不写用户数据、不改配置；中断即退
#
# 用法：
#   python3 scripts/chat-harness.py --profession P1 --max-turns 3 [--model <对话模型名>]
#   （--dry-run 只建会话与读观测，不发消息）
#
# ⚠ 骨架版：只带 P1 职业画像与 X 系列判据的空壳；扩 10 职业/长跑到完成/评审乙表见文件末尾 TODO。

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

CORE = os.environ.get("ZERG_CORE", "http://127.0.0.1:8580")      # 被测：对话链路（主控）
AGENT = os.environ.get("ZERG_LOCAL_AGENT", "http://127.0.0.1:8100")  # 驱动：本机模型（扮演使用者）
STATE_DIR = os.environ.get("ZERG_STATE_DIR", os.path.expanduser("~/.zerg/state"))
OBS = os.path.join(STATE_DIR, "chat_obs.jsonl")


def token():
    """读共享令牌（**只从磁盘读、绝不打印**）"""
    p = os.path.expanduser("~/.zerg/token")
    with open(p, "r", encoding="utf-8") as f:
        return f.read().strip()


def post(url, body, tok, stream=False, timeout=600):
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("X-Auth-Token", tok)
    return urllib.request.urlopen(req, timeout=timeout)


def obs_lines():
    """读观测文件（不存在返回空表）"""
    if not os.path.exists(OBS):
        return []
    with open(OBS, "r", encoding="utf-8") as f:
        return [l for l in f.read().splitlines() if l.strip()]


# ── 职业画像（骨架里只放 P1；其余见靶子表 §二）──
PROFESSIONS = {
    "P1": {
        "name": "程序员",
        "goal": "定位仓里一处小问题并说明改法（指到 file:line）",
        "system": ("你是一名程序员，正在和一个 AI 助手对话。你要用它解决真实的小活。"
                   "说话像真人：简短、具体、偶尔改口。不要写代码块给自己看，直接把话说完。"),
        "opening": "帮我看看 core/internal/chat/obs.go 这个文件是干什么的？",
    },
}


def local_user_reply(prof, history, tok):
    """让**本机模型**以该职业的口吻说出下一句（驱动 = 使用者）"""
    msgs = [{"role": "system", "content": prof["system"]}]
    for role, text in history[-8:]:
        msgs.append({"role": role, "content": text})
    body = {"model": os.environ.get("ZERG_DRIVER_MODEL", "gemma-4-26B"),
            "messages": msgs, "max_tokens": 200, "stream": False}
    with post(AGENT + "/infer", body, tok, timeout=300) as r:
        d = json.loads(r.read().decode())
    # llama-server 风格响应
    try:
        return d["choices"][0]["message"]["content"].strip()
    except Exception:
        return (d.get("content") or "").strip()


def send_turn(session_id, text, tok, timeout=900):
    """向对话链路发一轮（SSE）——返回 (事件数, 结束原因, 原始事件尾部)"""
    url = "%s/api/chat/sessions/%s/send" % (CORE, session_id)
    events, end, tail = 0, "no-end", []
    try:
        with post(url, {"content": text}, tok, timeout=timeout) as r:
            for raw in r:
                line = raw.decode("utf-8", "replace").strip()
                if not line.startswith("data:"):
                    continue
                events += 1
                if len(tail) < 200:
                    tail.append(line[:200])
                if any(k in line for k in ('"finish"', '"done"', '"error"')):
                    end = line[:200]
    except urllib.error.HTTPError as e:
        return events, "http_%d" % e.code, [str(e)]
    except Exception as e:  # 网络/超时
        return events, "exception:%s" % type(e).__name__, [str(e)[:200]]
    return events, end, tail


def check_invariants(new_obs):
    """X 系列判据（脚本层硬结论）——骨架版只做可直接判的几条"""
    bad = []
    for ln in new_obs:
        try:
            rec = json.loads(ln)
        except Exception:
            bad.append(("X-parse", "观测行不是合法 JSON: " + ln[:120]))
            continue
        if rec.get("kind") == "turn":
            er = rec.get("end_reason", "")
            if er != "finish":
                # X3：非正常收尾必须是三类分类码之一
                if er not in ("upstream_timeout", "client_aborted", "stream_truncated"):
                    bad.append(("X3", "非正常收尾却无分类：%s" % er))
                else:
                    bad.append(("X3-INFO", "本轮收尾=%s（分类码 ✓ 可定位）" % er))
            t = rec.get("turn") or {}
            if "max_gap_ms" not in t:
                bad.append(("X2", "turn 记录缺 max_gap_ms（卡死感无客观值）"))
        if rec.get("kind") == "compact":
            if rec.get("result") == "fail" and not rec.get("fail_reason"):
                bad.append(("X1", "压缩失败但没有 fail_reason"))
    return bad


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--profession", default="P1")
    ap.add_argument("--max-turns", type=int, default=3)
    ap.add_argument("--model", default=os.environ.get("ZERG_UNDER_TEST_MODEL", ""))
    ap.add_argument("--dry-run", action="store_true")
    a = ap.parse_args()
    prof = PROFESSIONS.get(a.profession) or sys.exit("未知职业：%s" % a.profession)

    tok = token()
    print("[harness] 职业=%s(%s) 目标=%s" % (a.profession, prof["name"], prof["goal"]))
    print("[harness] 被测=%s 驱动=%s 观测=%s" % (CORE, AGENT, OBS))

    # 1) 建会话（标题与来源都标成 harness，便于事后清理识别）
    with post(CORE + "/api/chat/sessions", {"title": "[harness]%s-%d" % (a.profession, int(time.time())),
                                            "source": "harness"}, tok, timeout=60) as r:
        _d = json.loads(r.read().decode())
        # 响应字段名实测为 id（会话对象亦然）；两者都收，避免再因字段名卡住
        sid = _d.get("session_id") or _d.get("id")
    print("[harness] 会话 = %s" % sid)

    # 2) 指定被测模型（可选）
    if a.model:
        with post("%s/api/chat/sessions/%s/model" % (CORE, sid), {"model": a.model}, tok, timeout=60) as r:
            print("[harness] 模型设为 %s（%s）" % (a.model, r.read().decode()[:80]))

    if a.dry_run:
        print("[harness] dry-run：只建会话，退出 ✓")
        return 0

    # 3) 逐轮：本机模型说 → 对话链路答 → 查观测
    history, findings = [], []
    before = len(obs_lines())
    for turn in range(1, a.max_turns + 1):
        user = prof["opening"] if turn == 1 else local_user_reply(prof, history, tok)
        if not user:
            findings.append(("driver", "本机模型没有产出用户发言（本轮跳过）"))
            break
        history.append(("user", user))
        print("\n[turn %d] 使用者> %s" % (turn, user[:160]))
        t0 = time.time()
        events, end, tail = send_turn(sid, user, tok)
        dt = time.time() - t0
        print("        助手> 事件 %d 个 · 用时 %.1fs · 收尾=%s" % (events, dt, end[:60]))
        # 助手回话（从消息表读最后一条 assistant）
        try:
            with urllib.request.urlopen(
                    urllib.request.Request("%s/api/chat/sessions/%s/messages" % (CORE, sid),
                                           headers={"X-Auth-Token": tok}), timeout=60) as r:
                msgs = json.loads(r.read().decode())
                arr = msgs if isinstance(msgs, list) else msgs.get("messages", [])
                last = [m for m in arr if m.get("role") == "assistant"]
                if last:
                    history.append(("assistant", last[-1].get("content", "")))
        except Exception as e:
            findings.append(("read-back", str(e)[:120]))

    # 4) 判据：只看新增观测行
    after = obs_lines()
    new = after[before:]
    print("\n[harness] 新增观测 %d 行" % len(new))
    for kind in ("turn", "compact", "tool"):
        n = sum(1 for l in new if ('"kind":"%s"' % kind) in l)
        print("           %-8s %d" % (kind, n))
    findings.extend(check_invariants(new))

    print("\n[harness] 发现 %d 条（甲表=硬结论；乙表=评审线索，未接）" % len(findings))
    for code, msg in findings:
        print("  - [%s] %s" % (code, msg))
    print("\n[harness] 会话 %s 保留待查（清理：DELETE /api/chat/sessions/%s）" % (sid, sid))
    return 0


# ── TODO（后续批次，按设计稿）──
#  · 10 职业画像（靶子表 §二）+ 每职业的真实任务与完成定义（§三 C1–C10）
#  · 长跑到"任务完成"为止（不设轮数上限；安全阀只在链路真死时停 ⇒ 记缺陷）
#  · X1 压缩：多次压缩后继续 / 失败不阻塞 / counter_reset 复位 / 摘要关键项不丢
#  · X4 卵侧：读 /eggs、GTT/内存、端口与在飞
#  · 乙表：本机模型按**同职业**评审（引原话）+ 归因到"系统/模型"两栏
#  · 报告：docs/issues/harness-<职业>-<日期>.md（含原始证据指针）
#  · runner：分批/断点续跑/净停
if __name__ == "__main__":
    sys.exit(main())

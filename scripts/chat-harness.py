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
        "system": ("你就是用户本人（程序员），正在用一个 AI 助手干你的活。\n"
                   "铁律：**你不是助手**。绝对不要写你的思考过程、计划、分析、步骤清单；"
                   "绝对不要用「The user wants…」「I need to…」这类旁白。\n"
                   "只输出**你要对助手说的那一句话**：像真人一样简短（一到两句）、具体、可以带口语和追问。\n"
                   "禁止输出任何英文分析，禁止输出编号列表。"),
        "opening": "帮我看看 core/internal/chat/obs.go 这个文件是干什么的？",
    },
}


def local_user_reply(prof, history, tok):
    """让**本机模型**以该职业的口吻说出下一句（驱动 = 使用者）"""
    msgs = [{"role": "system", "content": prof["system"]}]
    for role, text in history[-8:]:
        msgs.append({"role": role, "content": text})
    body = {"model": os.environ.get("ZERG_DRIVER_MODEL", "gemma-4-26B"),
            "messages": msgs, "max_tokens": 800, "stream": False,
            # 实测（2026-09-16）：本机 gemma 默认把额度花在思考上 ⇒ content='' 全是 reasoning。
            # 唯一有效办法是**引擎侧关思考**：enable_thinking=false ⇒ content 直接可用（0.5s）。
            # 提示词 /no_think、换模型（Nemotron/review）都无效。
            "chat_template_kwargs": {"enable_thinking": False}}
    with post(AGENT + "/infer", body, tok, timeout=300) as r:
        d = json.loads(r.read().decode())
    # llama-server 风格响应
    # 实测：content 可能为空而全文落在 reasoning_content（模型把额度用在了"想"上）⇒ 必须兜底，不当作"模型没说话"
    try:
        msg = d["choices"][0]["message"]
    except Exception:
        msg = {}
    txt = (msg.get("content") or "").strip()
    if not txt and False:  # 已废弃：从 reasoning 抽句会抽出分析文本（实测越兜越糟），改为引擎侧关思考
        r = (msg.get("reasoning_content") or "").strip()
        # 从思路里抽最后一句"像人说的话"（去掉编号/星号/英文分析行）
        cand = [l.strip(" *-\t") for l in r.split("\n") if l.strip(" *-\t")]
        cand = [l for l in cand if l and not l.startswith(("*", "#")) and not l[0].isascii()]
        txt = cand[-1] if cand else ""
        if txt:
            print("        （驱动：content 为空，已从 reasoning 抽句 ✓）")
    if not txt:
        # 重试一次：强制"只回一句话、不许分析"，并限制额度（不给它"想"的空间）
        strict = msgs[:-1] + [{"role": "user", "content": "只回**一句**你要对助手说的话（中文，一两句，口语）。禁止分析、禁止英文、禁止列表。"}]
        try:
            with post(AGENT + "/infer", {"model": os.environ.get("ZERG_DRIVER_MODEL", "gemma-4-26B"),
                                         "messages": strict, "max_tokens": 120, "stream": False}, tok, timeout=180) as r2:
                d2 = json.loads(r2.read().decode())
            txt = ((d2.get("choices", [{}])[0].get("message", {}) or {}).get("content") or "").strip()
            if txt:
                print("        （驱动：严格重试成功 ✓）")
        except Exception as e:
            print("        （驱动：严格重试失败 %s）" % str(e)[:80])
    if not txt:
        # 兜底：不中断会话 —— 用脚本续问（并如实标注"这条不是模型说的"）
        txt = "那你再具体一点说，我怎么验证它真的在工作？"
        print("        ⚠ [harness 缺口] 驱动两次都空 ⇒ 用脚本兜底续问（**如实标注：非模型生成**）")
    return txt


def send_turn(session_id, text, tok, timeout=900):
    """向对话链路发一轮（SSE）——返回 (事件数, 结束原因, 原始事件尾部)"""
    url = "%s/api/chat/sessions/%s/send" % (CORE, session_id)
    events, end, tail = 0, "no-end", []
    try:
        with post(url, {"content": text}, tok, timeout=timeout) as r:
            cur_ev = ""
            for raw in r:
                line = raw.decode("utf-8", "replace").rstrip()
                if line.startswith("event:"):          # 实测分帧：event: delta / compacting / done / error
                    cur_ev = line.split(":", 1)[1].strip()
                    if cur_ev in ("done", "finish", "error", "aborted"):
                        end = cur_ev
                    if len(tail) < 200:
                        tail.append(line[:200])
                    continue
                if not line.startswith("data:"):
                    continue
                events += 1
                payload = line[5:].strip()
                if cur_ev in ("done", "finish", "error", "aborted"):
                    end = "%s %s" % (cur_ev, payload[:120])
                if len(tail) < 200:
                    tail.append(line[:200])
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
        if end == "no-end" or end.startswith(("exception", "http_")):
            print("        ⚠ 收尾异常 ⇒ 打流尾 12 行（**留证据，不猜**）：")
            for _l in tail[-12:]:
                print("          尾| %s" % _l[:150])
        # 助手回话（从消息表读最后一条 assistant）
        try:
            with urllib.request.urlopen(
                    urllib.request.Request("%s/api/chat/sessions/%s/messages" % (CORE, sid),
                                           headers={"X-Auth-Token": tok}), timeout=60) as r:
                msgs = json.loads(r.read().decode())
                arr = msgs if isinstance(msgs, list) else msgs.get("messages", [])
                last = [m for m in arr if m.get("role") == "assistant"]
                if last:
                    _a = last[-1].get("content", "")
                    history.append(("assistant", _a))
                    print("        助手> %s" % _a.replace("\n", " ")[:200])
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

#!/usr/bin/env python3
"""多轮压缩保针衰减测试：连续压缩 N 轮，观察精确值从第几轮丢失。

对比：LLMLingua-2（删除式）vs SelectiveContext（自信息删除）vs gemma 摘要（重写式，走虫族网关）
"""
import json
import time
import urllib.request

MODEL_BASE = "<repo>/compress_models"
GATEWAY = "http://127.0.0.1:8082"
TOKEN = "x3gw-shared-2026"

NEEDLES = ["7329", "王强", "500", "三点", "Bearer", "auth.py:47"]

DIALOGUE = """
用户: 今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。
助手: 好的，记下来了。
用户: 新来的项目经理叫王强，他之前是腾讯的架构师。
助手: 收到。
用户: 项目预算最终定为 500 万，分三期付款。
助手: 明白。
用户: 明天下午三点开会，讨论 API 认证方案，重点看 Bearer token 过期刷新。
助手: 好的，我会准备。
用户: 代码里有个 bug 在 auth.py:47，需要修复。
助手: 记下了，auth.py:47。
"""

def check_needles(text):
    return {n: n in text for n in NEEDLES}

def load_llmlingua2():
    import torch
    from transformers import AutoModelForTokenClassification, AutoTokenizer
    tok = AutoTokenizer.from_pretrained(f"{MODEL_BASE}/llmlingua2")
    model = AutoModelForTokenClassification.from_pretrained(f"{MODEL_BASE}/llmlingua2")
    model.eval()
    return tok, model

def llmlingua2_compress(text, tok, model, max_tokens=500):
    import torch
    inputs = tok(text, return_tensors="pt")
    total = inputs["input_ids"].shape[1]
    all_keep = []
    for i in range(0, total, max_tokens):
        chunk = tok.decode(inputs["input_ids"][0][i:i+max_tokens], skip_special_tokens=True)
        ci = tok(chunk, return_tensors="pt")
        with torch.no_grad():
            logits = model(**ci).logits
        keep = logits.squeeze(0).argmax(-1).tolist()
        toks = tok.convert_ids_to_tokens(ci["input_ids"][0])
        all_keep.extend(t for t, k in zip(toks, keep) if k == 1)
    return tok.convert_tokens_to_string(all_keep)

def selective_compress(text, tok, model, reduce_ratio=0.5):
    import torch
    inputs = tok(text, return_tensors="pt")
    ids = inputs["input_ids"][0]
    with torch.no_grad():
        out = model(**inputs)
    logits = out.logits[0]
    probs = torch.softmax(logits, dim=-1)
    tok_probs = probs[torch.arange(len(ids)-1), ids[1:]]
    self_info = -torch.log(tok_probs.clamp(min=1e-9))
    n_keep = int(len(ids) * (1 - reduce_ratio))
    keep_idx = {0, len(ids)-1}
    top_idx = self_info.topk(max(n_keep - 2, 1)).indices + 1
    keep_idx.update(top_idx.tolist())
    kept_ids = [ids[i].item() for i in sorted(keep_idx)]
    return tok.decode(kept_ids, skip_special_tokens=True)

def gemma_summary(text):
    """走虫族网关 gemma 摘要（重写式对照）"""
    body = json.dumps({"messages": [{"role": "user", "content": f"将以下对话浓缩成简洁摘要，保留关键信息（数字/人名/路径必须原样保留）：\n{text}"}]}).encode()
    req = urllib.request.Request(f"{GATEWAY}/v1/context/compact", data=body,
                                 headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"})
    d = json.loads(urllib.request.urlopen(req, timeout=300).read())
    return d.get("summary", "")

def run_rounds(name, compress_fn, rounds=10):
    print(f"\n{'='*60}")
    print(f"📊 {name}（连续压缩 {rounds} 轮）")
    print(f"{'='*60}")
    text = DIALOGUE
    lost_round = {}
    for r in range(1, rounds+1):
        t0 = time.time()
        text = compress_fn(text)
        elapsed = time.time() - t0
        results = check_needles(text)
        # 记录首次丢失
        for n, found in results.items():
            if not found and n not in lost_round:
                lost_round[n] = r
        status = " | ".join(f"{n}:{'✅' if v else '❌'}" for n, v in results.items())
        print(f"  轮{r}: {elapsed:.1f}s | 长度{len(text)} | {status}")
        # 全部丢失就停
        if all(not v for v in results.values()):
            print(f"  ⚠️ 全部针丢失，提前停止")
            break
    print(f"\n  首次丢失轮次: {lost_round if lost_round else '全部存活'}")

if __name__ == "__main__":
    import torch  # noqa
    from transformers import AutoModelForCausalLM, AutoTokenizer

    # LLMLingua-2（删除式）
    tok, model = load_llmlingua2()
    run_rounds("LLMLingua-2（删除式）", lambda t: llmlingua2_compress(t, tok, model), rounds=8)

    # SelectiveContext（自信息删除）
    print("\n加载 GPT-2 ...")
    stok = AutoTokenizer.from_pretrained(f"{MODEL_BASE}/gpt2")
    smodel = AutoModelForCausalLM.from_pretrained(f"{MODEL_BASE}/gpt2")
    smodel.eval()
    run_rounds("SelectiveContext（自信息删除）", lambda t: selective_compress(t, stok, smodel), rounds=8)

    # gemma 摘要（重写式）
    run_rounds("gemma 摘要（重写式，虫族网关）", gemma_summary, rounds=4)

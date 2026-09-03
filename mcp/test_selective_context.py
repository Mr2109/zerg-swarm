#!/usr/bin/env python3
"""SelectiveContext 算法实现（绕过 spacy 依赖地狱）：
核心 = GPT-2 算每个 token 的 self-information（perplexity），删低信息 token。
参考 Selective_Context (liyucheng09) 论文：自信息低 = 可预测 = 冗余。
"""
import time
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer

def build_text(n_chars):
    base = "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。新来的项目经理叫王强，他之前是腾讯的架构师。项目预算最终定为 500 万，分三期付款。明天下午三点开会。"
    return base * max(1, n_chars // len(base))

def selective_compress(text, model, tok, reduce_ratio=0.5):
    """按 self-information 删除低信息 token"""
    inputs = tok(text, return_tensors="pt")
    ids = inputs["input_ids"][0]
    with torch.no_grad():
        out = model(**inputs)
    logits = out.logits[0]  # [seq, vocab]
    # token 的 self-information = -log(softmax 概率)
    probs = torch.softmax(logits, dim=-1)
    tok_probs = probs[torch.arange(len(ids)-1), ids[1:]]  # 每个 token 的概率（预测概率）
    self_info = -torch.log(tok_probs.clamp(min=1e-9))

    # 保留 top (1-ratio) 的信息 token
    n_keep = int(len(ids) * (1 - reduce_ratio))
    # 前 2 个 token（<s> 等）和最后一个 token 必须保留
    keep_idx = set()
    keep_idx.update([0, len(ids)-1])
    # 高自信息（低概率=信息量大）的保留
    top_idx = self_info.topk(max(n_keep - 2, 1)).indices + 1  # +1 偏移（对齐预测位置）
    keep_idx.update(top_idx.tolist())

    kept_ids = [ids[i].item() for i in sorted(keep_idx)]
    return tok.decode(kept_ids, skip_special_tokens=True), self_info

def main():
    print("=" * 60)
    print("SelectiveContext（GPT-2 自信息压缩）——绕过 spacy 的实现")
    print("=" * 60)

    t0 = time.time()
    tok = AutoTokenizer.from_pretrained("gpt2")
    model = AutoModelForCausalLM.from_pretrained("gpt2")
    model.eval()
    print(f"GPT-2 加载: {time.time()-t0:.2f}s")

    text = build_text(500)
    print(f"\n原始 ({len(text)}字): {text[:80]}...")

    for ratio in [0.3, 0.5, 0.7]:
        t0 = time.time()
        compressed, _ = selective_compress(text, model, tok, reduce_ratio=ratio)
        elapsed = time.time() - t0
        keep_pct = len(compressed) / len(text) * 100
        # 针检查
        needles = {"7329": "7329" in compressed, "王强": "王强" in compressed,
                   "500": "500" in compressed, "三点": "三点" in compressed}
        print(f"\n压缩率 {ratio*100:.0f}%: {len(text)} → {len(compressed)}字 (保留{keep_pct:.0f}%) 耗时 {elapsed:.2f}s")
        print(f"  压缩后: {compressed[:120]}...")
        print(f"  保针: {'/'.join('✅' if v else '❌' for v in needles.values())}")

if __name__ == "__main__":
    main()

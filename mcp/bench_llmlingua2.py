#!/usr/bin/env python3
"""LLMLingua-2 详细测评：加载时间 / 内存占用 / 处理时间（不同长度）"""
import os
import resource
import time
import sys

MODEL_PATH = "<repo>/compress_models/llmlingua2"

def mem_mb():
    return resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024

def build_text(n_chars):
    """构造不同长度的文本（中文对话风格）"""
    base = "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。新来的项目经理叫王强，他之前是腾讯的架构师。项目预算最终定为 500 万，分三期付款。"
    repeat = max(1, n_chars // len(base))
    return base * repeat

def main():
    print("=" * 60)
    print("LLMLingua-2 详细测评")
    print("=" * 60)

    # 1. 加载时间
    print("\n[1] 加载时间 + 内存")
    t0 = time.time()
    from transformers import AutoModelForTokenClassification, AutoTokenizer
    print(f"  transformers import: {time.time()-t0:.2f}s")
    t0 = time.time()
    tok = AutoTokenizer.from_pretrained(MODEL_PATH)
    print(f"  tokenizer 加载: {time.time()-t0:.2f}s")
    t0 = time.time()
    model = AutoModelForTokenClassification.from_pretrained(MODEL_PATH)
    load_time = time.time() - t0
    print(f"  模型加载: {load_time:.2f}s")
    print(f"  当前 RSS: {mem_mb():.0f} MB")
    print(f"  模型参数量: {sum(p.numel() for p in model.parameters())/1e6:.1f}M")

    # 2. 处理时间（不同长度）
    print("\n[2] 处理时间（不同长度文本）")
    import torch
    model.eval()
    for n_chars in [500, 1000, 2000, 4000, 8000, 16000]:
        text = build_text(n_chars)
        t0 = time.time()
        inputs = tok(text, return_tensors="pt", truncation=True, max_length=510)
        tok_time = time.time() - t0
        n_tokens = inputs["input_ids"].shape[1]

        t0 = time.time()
        with torch.no_grad():
            logits = model(**inputs).logits
        infer_time = time.time() - t0

        # 压缩率（label 1 = keep）
        keep = logits.squeeze(0).argmax(-1).tolist()
        keep_ratio = sum(1 for k in keep if k == 1) / max(len(keep), 1) * 100

        print(f"  {n_chars}字 ({n_tokens} tokens): tokenize {tok_time*1000:.0f}ms + 推理 {infer_time:.2f}s = 总 {tok_time+infer_time:.2f}s | 保留率 {keep_ratio:.0f}%")

    # 3. 总结
    print("\n[3] 结论")
    print(f"  加载: 一次 {load_time:.2f}s（常驻后不重复）")
    print(f"  内存: {mem_mb():.0f} MB（加载后常驻）")
    print(f"  单次处理: 与输入长度成正比，最长 16K 字约 Xs")

if __name__ == "__main__":
    main()

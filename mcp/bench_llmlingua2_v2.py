#!/usr/bin/env python3
"""LLMLingua-2 详细测评 v2：分块处理长文本 + 正确内存测量"""
import os
import time
import subprocess

MODEL_PATH = "<repo>/compress_models/llmlingua2"

def build_text(n_chars):
    base = "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。新来的项目经理叫王强，他之前是腾讯的架构师。项目预算最终定为 500 万，分三期付款。"
    return base * max(1, n_chars // len(base))

# 用子进程测内存（psutil 方式：RSS = 进程实际内存）
import resource

def main():
    print("=" * 60)
    print("LLMLingua-2 详细测评 v2（分块处理 + 正确内存）")
    print("=" * 60)

    import torch
    from transformers import AutoModelForTokenClassification, AutoTokenizer

    # 加载
    t0 = time.time()
    tok = AutoTokenizer.from_pretrained(MODEL_PATH)
    model = AutoModelForTokenClassification.from_pretrained(MODEL_PATH)
    load_time = time.time() - t0
    model.eval()

    # 内存（macOS ru_maxrss 单位是 bytes，转 MB）
    rss_bytes = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    rss_mb = rss_bytes / 1024 / 1024 if rss_bytes > 10**9 else rss_bytes / 1024
    print(f"加载: {load_time:.2f}s | 参数: {sum(p.numel() for p in model.parameters())/1e6:.0f}M | RSS: {rss_mb:.0f} MB")

    # 分块处理（max 510 token/块）
    def process_chunked(text, max_tokens=500):
        inputs = tok(text, return_tensors="pt")
        total_tokens = inputs["input_ids"].shape[1]
        # 分块
        all_keep = []
        for i in range(0, total_tokens, max_tokens):
            chunk = tok.decode(inputs["input_ids"][0][i:i+max_tokens], skip_special_tokens=True)
            ci = tok(chunk, return_tensors="pt")
            with torch.no_grad():
                logits = model(**ci).logits
            keep = logits.squeeze(0).argmax(-1).tolist()
            toks = tok.convert_ids_to_tokens(ci["input_ids"][0])
            all_keep.extend(t for t, k in zip(toks, keep) if k == 1)
        return tok.convert_tokens_to_string(all_keep)

    # 测不同长度（warmup 后）
    print("\n处理时间（分块）:")
    # warmup
    process_chunked(build_text(200), 500)

    for n_chars in [500, 2000, 5000, 10000, 20000]:
        text = build_text(n_chars)
        t0 = time.time()
        compressed = process_chunked(text)
        elapsed = time.time() - t0
        keep_pct = len(compressed) / len(text) * 100
        print(f"  {n_chars}字: {elapsed:.2f}s | 压缩后 {len(compressed)}字 (保留{keep_pct:.0f}%) | 针7329={'7329' in compressed}")

    # 单次请求的实际延迟组成
    print("\n结论:")
    print(f"  加载: 一次性 {load_time:.2f}s（常驻不重复）")
    print(f"  内存: {rss_mb:.0f} MB 常驻")
    print(f"  处理: 分块后与输入长度成正比（每 500 token 约 0.2s）")
    print(f"  单次请求总延迟 = 加载(首次) + 处理(分块×块数)")

if __name__ == "__main__":
    main()

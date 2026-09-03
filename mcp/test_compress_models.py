#!/usr/bin/env python3
"""测试：三种压缩模型效果对比——LLMLingua-2 / AutoCompressor / 规则精简。"""
import sys
import time

MODELS_DIR = "<repo>/compress_models"

NEEDLE_DIALOGUE = [
    {"role": "user", "content": "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。"},
    {"role": "assistant", "content": "好的，记下来了。"},
    {"role": "user", "content": "新来的项目经理叫王强，他之前是腾讯的架构师。"},
    {"role": "assistant", "content": "收到。"},
    {"role": "user", "content": "项目预算最终定为 500 万，分三期付款。"},
    {"role": "assistant", "content": "明白。"},
    {"role": "user", "content": "明天下午三点开会，讨论 API 认证方案，重点看 Bearer token 过期刷新。"},
    {"role": "assistant", "content": "好的，我会准备。"},
]

QUESTIONS = [
    ("保险箱密码是多少？", "7329"),
    ("新来的项目经理叫什么？", "王强"),
    ("项目预算是多少？", "500"),
    ("明天几点开会？", "三点"),
]


def text_of(msg):
    c = msg.get("content", "")
    return c if isinstance(c, str) else ""


def full_text(messages):
    return "\n".join(f"[{m.get('role')}] {text_of(m)}" for m in messages)


def check_needles(compressed_text):
    return [(q, expect, expect in compressed_text) for q, expect in QUESTIONS]


# ============ LLMLingua-2 ============
def test_llmlingua2():
    from transformers import AutoModelForTokenClassification, AutoTokenizer
    model_path = f"{MODELS_DIR}/llmlingua2"
    tok = AutoTokenizer.from_pretrained(model_path)
    model = AutoModelForTokenClassification.from_pretrained(model_path)

    original = full_text(NEEDLE_DIALOGUE)
    start = time.time()
    import torch
    inputs = tok(original, return_tensors="pt", truncation=True, max_length=510)
    with torch.no_grad():
        logits = model(**inputs).logits
    # label 1 = keep（LLMLingua-2 惯例：1 保留 0 删除）
    keep = logits.squeeze(0).argmax(-1).tolist()
    tokens = tok.convert_ids_to_tokens(inputs["input_ids"][0])
    kept = [t for t, k in zip(tokens, keep) if k == 1]
    compressed = tok.convert_tokens_to_string(kept)
    elapsed = time.time() - start

    return {
        "compressed": compressed,
        "orig_len": len(original),
        "comp_len": len(compressed),
        "ratio": f"{(1 - len(compressed)/max(len(original),1)) * 100:.1f}%",
        "elapsed": f"{elapsed:.2f}s",
        "needles": check_needles(compressed),
        "num_keep": sum(1 for k in keep if k == 1),
        "num_total": len(keep),
    }


# ============ AutoCompressor ============
def test_autocompressor():
    from transformers import AutoModelForCausalLM, AutoTokenizer
    model_path = f"{MODELS_DIR}/autocompressor"
    tok = AutoTokenizer.from_pretrained(model_path)
    model = AutoModelForCausalLM.from_pretrained(model_path)

    original = full_text(NEEDLE_DIALOGUE)
    start = time.time()
    import torch
    inputs = tok(original, return_tensors="pt", truncation=True, max_length=4096)
    with torch.no_grad():
        out = model(**inputs)
    elapsed = time.time() - start

    return {
        "note": "AutoCompressor 是向量级压缩（soft prompt），输出 logits 不可直接读文本",
        "orig_len": len(original),
        "logits_shape": list(out.logits.shape),
        "elapsed": f"{elapsed:.2f}s",
        "model_loaded": True,
    }


# ============ 规则精简（我们的快路径，对照） ============
def test_rule_trim():
    FILLERS = {"好的", "收到", "明白", "好的，记下来了", "好的，我会准备。"}
    trimmed = []
    changed = False
    for m in NEEDLE_DIALOGUE:
        content = text_of(m)
        if m.get("role") == "assistant" and content.strip() in FILLERS:
            changed = True
            continue
        trimmed.append(m)
    compressed = full_text(trimmed)
    return {
        "compressed": compressed,
        "orig_len": len(full_text(NEEDLE_DIALOGUE)),
        "comp_len": len(compressed),
        "ratio": f"{(1 - len(compressed)/max(len(full_text(NEEDLE_DIALOGUE)),1)) * 100:.1f}%",
        "elapsed": "0.00s（纯规则）",
        "needles": check_needles(compressed),
    }


def show(name, r):
    print(f"\n{'='*60}\n📊 {name}\n{'='*60}")
    if "error" in r:
        print(f"❌ {r['error']}")
        return
    if "note" in r:
        print(f"ℹ️ {r['note']}")
        print(f"   原始: {r['orig_len']} 字符 | logits: {r['logits_shape']} | 耗时: {r['elapsed']}")
        return
    print(f"   原始: {r['orig_len']} 字符 → 压缩: {r['comp_len']} 字符（{r['ratio']}）耗时 {r['elapsed']}")
    print(f"   压缩后: {r['compressed'][:150]}...")
    print(f"   针保留:")
    for q, expect, found in r["needles"]:
        print(f"     {'✅' if found else '❌'} {q}（期望 {expect}）")


if __name__ == "__main__":
    show("规则精简（我们的快路径，对照）", test_rule_trim())
    show("LLMLingua-2（token 分类压缩）", test_llmlingua2())
    show("AutoCompressor（摘要向量压缩）", test_autocompressor())

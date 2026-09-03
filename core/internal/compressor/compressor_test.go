package compressor

import (
	"strings"
	"testing"
)

func TestCompress(t *testing.T) {
	base := "<repo>/compress_models/llmlingua2-onnx"
	c := New(Config{
		ModelPath: base + "/model.onnx",
		TokPath:   base + "/tokenizer.json",
	})
	err := c.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	defer c.Destroy()

	text := "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。新来的项目经理叫王强，他之前是腾讯的架构师。项目预算最终定为 500 万，分三期付款。"
	compressed, origLen, compLen, err := c.Compress(text)
	if err != nil {
		t.Fatalf("Compress 失败: %v", err)
	}

	t.Logf("原始 %d 字: %s", origLen, text[:60])
	t.Logf("压缩 %d 字 (%.0f%%): %s", compLen, float64(compLen)/float64(origLen)*100, compressed[:min(len(compressed), 80)])

	// 保针检查
	needles := []string{"7329", "王强", "500"}
	for _, n := range needles {
		if !strings.Contains(compressed, n) {
			t.Errorf("针丢失: %s 不在压缩文本中", n)
		} else {
			t.Logf("✅ 针保留: %s", n)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

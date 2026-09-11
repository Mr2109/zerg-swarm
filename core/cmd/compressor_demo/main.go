package main

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/compressor"
)

func main() {
	base := filepath.Join(statepath.CompressModelsDir(), "llmlingua2-onnx")
	c := compressor.New(compressor.Config{
		ModelPath: base + "/model.onnx",
		TokPath:   base + "/tokenizer.json",
	})
	err := c.Load()
	if err != nil {
		fmt.Printf("load failed: %v\n", err)
		return
	}
	defer c.Destroy()

	text := "今天讨论项目进展，顺便说一下，保险箱的密码是 7329，放在书房书桌第二个抽屉里。新来的项目经理叫王强，他之前是腾讯的架构师。项目预算最终定为 500 万，分三期付款。"
	compressed, origLen, compLen, err := c.Compress(text)
	if err != nil {
		fmt.Printf("compression failed: %v\n", err)
		return
	}

	fmt.Printf("original %d chars: %s\n", origLen, text)
	fmt.Printf("compressed %d chars (%.0f%%): %s\n", compLen, float64(compLen)/float64(origLen)*100, compressed)
	for _, n := range []string{"7329", "王强", "500"} {
		ok := strings.Contains(compressed, n)
		fmt.Printf("needle %s: %v\n", n, ok)
	}
}

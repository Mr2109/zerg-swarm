//go:build darwin

package compressor

// 虫族 LLMLingua-2 压缩器（Go 一体化实现）
// ONNX 模型 + 纯 Go tokenizer（goSentencePiece 支持 HF tokenizer.json）
// 引擎无关：不依赖 Python/transformers/llama

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/tggo/goSentencePiece"
	"github.com/yalue/onnxruntime_go"
)

// Compressor LLMLingua-2 压缩器实例。
type Compressor struct {
	model     *onnxruntime_go.DynamicAdvancedSession
	tok       *sentencepiece.Tokenizer
	cfg       Config
	maxTokens int
	loaded    bool
}

// Config 压缩器配置。
type Config struct {
	ModelPath string // ONNX 模型路径（model.onnx）
	TokPath   string // tokenizer.json 路径
	MaxTokens int    // 分块上限（默认 500）
}

// New 创建压缩器（未加载，需 Load）。
func New(cfg Config) *Compressor {
	maxT := cfg.MaxTokens
	if maxT <= 0 {
		maxT = 500
	}
	return &Compressor{cfg: cfg, maxTokens: maxT}
}

// Load 加载 ONNX 模型 + tokenizer（用 New 时的配置）。
func (c *Compressor) Load() error {
	return c.LoadConfig(c.cfg)
}

// LoadConfig 加载 ONNX 模型 + tokenizer（显式配置）。
func (c *Compressor) LoadConfig(cfg Config) error {
	if c.loaded {
		return nil
	}
	// v2.5.1: macOS 下 onnxruntime_go 默认找 onnxruntime.so（Linux 命名）——需指定 .dylib
	if runtime.GOOS == "darwin" {
		candidates := []string{
			"/opt/homebrew/lib/libonnxruntime.dylib",
			"/usr/local/lib/libonnxruntime.dylib",
			"/usr/local/lib/libonnxruntime.1.dylib",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				onnxruntime_go.SetSharedLibraryPath(p)
				break
			}
		}
	}
	if err := onnxruntime_go.InitializeEnvironment(); err != nil {
		return fmt.Errorf("failed to initialize onnxruntime: %w", err)
	}

	model, err := onnxruntime_go.NewDynamicAdvancedSession(
		cfg.ModelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to load ONNX model: %w", err)
	}

	tok, err := sentencepiece.NewTokenizerFromJSON(cfg.TokPath)
	if err != nil {
		model.Destroy()
		return fmt.Errorf("failed to load tokenizer: %w", err)
	}

	c.model = model
	c.tok = tok
	c.cfg = cfg
	c.maxTokens = cfg.MaxTokens
	if c.maxTokens <= 0 {
		c.maxTokens = 500
	}
	c.loaded = true
	log.Printf("🧠 compressor loaded: %s", cfg.ModelPath)
	return nil
}

// Compress 压缩文本（分块处理，删除低信息 token）。
// 返回 (压缩后文本, 原始字符数, 压缩后字符数)。
func (c *Compressor) Compress(text string) (string, int, int, error) {
	if !c.loaded {
		return "", 0, 0, fmt.Errorf("compressor not loaded")
	}
	if strings.TrimSpace(text) == "" {
		return text, len(text), len(text), nil
	}

	ids, err := c.tok.Encode(text)
	if err != nil {
		return "", 0, 0, fmt.Errorf("encoding failed: %w", err)
	}

	keptIDs := make([]int, 0, len(ids))
	for start := 0; start < len(ids); start += c.maxTokens {
		end := start + c.maxTokens
		if end > len(ids) {
			end = len(ids)
		}
		kept, err := c.compressChunk(ids[start:end])
		if err != nil {
			return "", 0, 0, fmt.Errorf("chunk %d compression failed: %w", start/c.maxTokens, err)
		}
		keptIDs = append(keptIDs, kept...)
	}

	compressed, err := c.tok.Decode(keptIDs)
	if err != nil {
		return "", 0, 0, fmt.Errorf("decoding failed: %w", err)
	}
	compressed = strings.TrimSpace(compressed)
	return compressed, len(text), len(compressed), nil
}

// compressChunk 压缩单个 token 块（ONNX 推理）。返回保留的 token ID 列表。
func (c *Compressor) compressChunk(ids []int) ([]int, error) {
	n := len(ids)
	if n == 0 {
		return nil, nil
	}

	inputData := make([]int64, n)
	maskData := make([]int64, n)
	for i, id := range ids {
		inputData[i] = int64(id)
		maskData[i] = 1
	}

	inputTensor, err := onnxruntime_go.NewTensor[int64](
		onnxruntime_go.NewShape(int64(1), int64(n)), inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	maskTensor, err := onnxruntime_go.NewTensor[int64](
		onnxruntime_go.NewShape(int64(1), int64(n)), maskData)
	if err != nil {
		return nil, fmt.Errorf("failed to create mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	output, err := onnxruntime_go.NewEmptyTensor[float32](
		onnxruntime_go.NewShape(int64(1), int64(n), int64(2)))
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer output.Destroy()

	if err := c.model.Run(
		[]onnxruntime_go.Value{inputTensor, maskTensor},
		[]onnxruntime_go.Value{output},
	); err != nil {
		return nil, fmt.Errorf("ONNX inference failed: %w", err)
	}

	logits := output.GetData()
	kept := make([]int, 0, n)
	for i := 0; i < n; i++ {
		idx := i * 2
		if idx+1 >= len(logits) {
			break
		}
		if logits[idx+1] > logits[idx] {
			kept = append(kept, ids[i])
		}
	}
	return kept, nil
}

// Destroy 释放资源。
func (c *Compressor) Destroy() {
	if c.model != nil {
		c.model.Destroy()
		c.model = nil
	}
	c.loaded = false
}

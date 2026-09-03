//go:build linux

package compressor

// Linux no-op 版压缩器（v2.5.3 容器环境——不引 onnxruntime——跳过语义压缩）
// 容器内跑 zerg-agent 不需要压缩（压缩是主控侧优化——容器里直接原文本）
// 接口与 darwin 版一致（New/Load/LoadConfig/Compress/Destroy）

// Compressor Linux 版压缩器（no-op——不压缩）。
type Compressor struct {
	cfg Config
}

// Config 压缩器配置。
type Config struct {
	ModelPath string // ONNX 模型路径（Linux 版忽略）
	TokPath   string // tokenizer.json 路径（Linux 版忽略）
	MaxTokens int    // 分块上限
}

// New 创建压缩器（Linux 版 no-op）。
func New(cfg Config) *Compressor {
	return &Compressor{cfg: cfg}
}

// Load Linux 版无操作（不需要模型）。
func (c *Compressor) Load() error { return nil }

// LoadConfig Linux 版无操作。
func (c *Compressor) LoadConfig(cfg Config) error {
	c.cfg = cfg
	return nil
}

// Compress Linux 版原样返回（不压缩——无 onnx 依赖）。
func (c *Compressor) Compress(text string) (string, int, int, error) {
	return text, len(text), len(text), nil
}

// Destroy Linux 版无操作。
func (c *Compressor) Destroy() {}

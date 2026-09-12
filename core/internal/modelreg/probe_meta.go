package modelreg

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

// ── probe.meta.gguf.v1：直接读本地 .gguf 文件的元数据 ──────────────────────
//
// 为什么要自己解析 GGUF 头：本机可能没有可跑的引擎（开工方案 §八 风险2），
// 此时"读 GGUF 元数据"是 probe 唯一还能给出的证据——它用于生成
// context_window 与 EngineRecipe.ChatTemplate，且不加载权重、不联网。

// GGUF 值类型（GGUF 规范 v2/v3）。
const (
	ggufTypeUint8   = 0
	ggufTypeInt8    = 1
	ggufTypeUint16  = 2
	ggufTypeInt16   = 3
	ggufTypeUint32  = 4
	ggufTypeInt32   = 5
	ggufTypeFloat32 = 6
	ggufTypeBool    = 7
	ggufTypeString  = 8
	ggufTypeArray   = 9
	ggufTypeUint64  = 10
	ggufTypeInt64   = 11
	ggufTypeFloat64 = 12
)

// ggufReadBudget 给元数据区扫描设上限，避免畸形/超大文件把探测拖死。
// 正常 GGUF 的 KV 区（含 token 表）远小于此；即便超了也只是少读几个键，不影响判定。
const ggufReadBudget = 256 << 20

var errGGUFBudget = errors.New("gguf 元数据读取超出预算")

// GGUFMeta 是从 .gguf 文件头读到的元数据（probe.meta.gguf.v1 的产物）。
type GGUFMeta struct {
	Version       int
	Architecture  string
	Name          string
	ContextWindow int
	BlockCount    int
	EmbedLength   int
	ChatTemplate  string
	LicenseSPDX   string
	LicenseName   string
	LicenseLink   string
	FileType      int
	KeyCount      int
	BytesRead     int64
	Truncated     bool

	// ── 批 3 补读（《设计-资源管理器》§八 Q4 / §3.2）：KV cache 估算要的真值 ──
	// 口径：读不到一律 0（缺席）——绝不填经验值冒充实测（估算侧会按架构族回退并标 estimated=true）。
	AttentionHeadKv int // {arch}.attention.head_count_kv —— KV 头数（n_kv_heads）
	AttentionHeadN  int // {arch}.attention.head_count    —— 注意力头数（n_head）
	KeyLength       int // {arch}.attention.key_length    —— 头维度（新式 GGUF 显式给出）
	ValueLength     int // {arch}.attention.value_length
	RopeDimCount    int // {arch}.rope.dimension_count    —— 旋转维度（头维度缺失时的代理）
}

type ggufScanner struct {
	br    *bufio.Reader
	limit int64
	used  int64
}

func newGGUFScanner(r io.Reader) *ggufScanner {
	return &ggufScanner{br: bufio.NewReaderSize(r, 1<<16), limit: ggufReadBudget}
}

func (s *ggufScanner) readFull(b []byte) error {
	if s.used+int64(len(b)) > s.limit {
		return errGGUFBudget
	}
	if _, err := io.ReadFull(s.br, b); err != nil {
		return err
	}
	s.used += int64(len(b))
	return nil
}

func (s *ggufScanner) discard(n int64) error {
	if n < 0 {
		return fmt.Errorf("gguf：负长度 %d", n)
	}
	if s.used+n > s.limit {
		return errGGUFBudget
	}
	m, err := io.CopyN(io.Discard, s.br, n)
	s.used += m
	return err
}

func (s *ggufScanner) u32() (uint32, error) {
	var b [4]byte
	if err := s.readFull(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

func (s *ggufScanner) u64() (uint64, error) {
	var b [8]byte
	if err := s.readFull(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func (s *ggufScanner) str() (string, error) {
	n, err := s.u64()
	if err != nil {
		return "", err
	}
	if n > uint64(s.limit) {
		return "", errGGUFBudget
	}
	b := make([]byte, n)
	if err := s.readFull(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func ggufFixedSize(t uint32) int64 {
	switch t {
	case ggufTypeUint8, ggufTypeInt8, ggufTypeBool:
		return 1
	case ggufTypeUint16, ggufTypeInt16:
		return 2
	case ggufTypeUint32, ggufTypeInt32, ggufTypeFloat32:
		return 4
	case ggufTypeUint64, ggufTypeInt64, ggufTypeFloat64:
		return 8
	}
	return 0
}

// skipValue 消费掉一个值但不解析（token 表这类巨大数组走这里，用 Discard 不占内存）。
func (s *ggufScanner) skipValue(vt uint32) error {
	if sz := ggufFixedSize(vt); sz > 0 {
		return s.discard(sz)
	}
	switch vt {
	case ggufTypeString:
		n, err := s.u64()
		if err != nil {
			return err
		}
		return s.discard(int64(n))
	case ggufTypeArray:
		et, err := s.u32()
		if err != nil {
			return err
		}
		cnt, err := s.u64()
		if err != nil {
			return err
		}
		if sz := ggufFixedSize(et); sz > 0 {
			if cnt > 1<<40 {
				return fmt.Errorf("gguf：数组长度异常 %d", cnt)
			}
			return s.discard(int64(cnt) * sz)
		}
		for i := uint64(0); i < cnt; i++ {
			if err := s.skipValue(et); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("gguf：未知值类型 %d", vt)
}

// readInt 读整数（浮点截断）；类型不匹配时返回 ok=false 且**不消费**，由调用方跳过。
func (s *ggufScanner) readInt(vt uint32) (int64, bool, error) {
	switch vt {
	case ggufTypeUint8:
		var b [1]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		return int64(b[0]), true, nil
	case ggufTypeInt8:
		var b [1]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		return int64(int8(b[0])), true, nil
	case ggufTypeUint16, ggufTypeInt16:
		var b [2]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		v := binary.LittleEndian.Uint16(b[:])
		if vt == ggufTypeInt16 {
			return int64(int16(v)), true, nil
		}
		return int64(v), true, nil
	case ggufTypeUint32, ggufTypeInt32:
		var b [4]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		v := binary.LittleEndian.Uint32(b[:])
		if vt == ggufTypeInt32 {
			return int64(int32(v)), true, nil
		}
		return int64(v), true, nil
	case ggufTypeUint64, ggufTypeInt64:
		var b [8]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		v := binary.LittleEndian.Uint64(b[:])
		return int64(v), true, nil
	case ggufTypeFloat32:
		var b [4]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		return int64(math.Float32frombits(binary.LittleEndian.Uint32(b[:]))), true, nil
	case ggufTypeFloat64:
		var b [8]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		return int64(math.Float64frombits(binary.LittleEndian.Uint64(b[:]))), true, nil
	case ggufTypeBool:
		var b [1]byte
		if err := s.readFull(b[:]); err != nil {
			return 0, false, err
		}
		return int64(b[0]), true, nil
	}
	return 0, false, nil
}

func (s *ggufScanner) wantString(vt uint32, dst *string) error {
	if vt != ggufTypeString {
		return s.skipValue(vt)
	}
	v, err := s.str()
	if err != nil {
		return err
	}
	*dst = v
	return nil
}

func (s *ggufScanner) wantInt(vt uint32, dst *int) error {
	v, ok, err := s.readInt(vt)
	if err != nil {
		return err
	}
	if !ok {
		return s.skipValue(vt)
	}
	*dst = int(v)
	return nil
}

// assign 按 GGUF 键名分派；不关心的键一律跳过。
func (s *ggufScanner) assign(m *GGUFMeta, key string, vt uint32) error {
	switch {
	case key == "general.architecture":
		return s.wantString(vt, &m.Architecture)
	case key == "general.name":
		return s.wantString(vt, &m.Name)
	case key == "general.license":
		return s.wantString(vt, &m.LicenseSPDX)
	case key == "general.license.name":
		return s.wantString(vt, &m.LicenseName)
	case key == "general.license.link":
		return s.wantString(vt, &m.LicenseLink)
	case key == "tokenizer.chat_template":
		return s.wantString(vt, &m.ChatTemplate)
	case key == "general.file_type":
		return s.wantInt(vt, &m.FileType)
	case strings.HasSuffix(key, ".context_length"):
		return s.wantInt(vt, &m.ContextWindow)
	case strings.HasSuffix(key, ".block_count"):
		return s.wantInt(vt, &m.BlockCount)
	case strings.HasSuffix(key, ".embedding_length"):
		return s.wantInt(vt, &m.EmbedLength)
	case strings.HasSuffix(key, ".attention.head_count_kv"):
		return s.wantInt(vt, &m.AttentionHeadKv)
	case strings.HasSuffix(key, ".attention.head_count"):
		return s.wantInt(vt, &m.AttentionHeadN)
	case strings.HasSuffix(key, ".attention.key_length"), strings.HasSuffix(key, ".key_length"):
		return s.wantInt(vt, &m.KeyLength)
	case strings.HasSuffix(key, ".attention.value_length"), strings.HasSuffix(key, ".value_length"):
		return s.wantInt(vt, &m.ValueLength)
	case strings.HasSuffix(key, ".rope.dimension_count"):
		return s.wantInt(vt, &m.RopeDimCount)
	default:
		return s.skipValue(vt)
	}
}

func parseGGUF(s *ggufScanner) (*GGUFMeta, error) {
	var magic [4]byte
	if err := s.readFull(magic[:]); err != nil {
		return nil, fmt.Errorf("读取 magic 失败：%w", err)
	}
	if string(magic[:]) != "GGUF" {
		return nil, fmt.Errorf("不是 GGUF（magic=%q）", string(magic[:]))
	}
	ver, err := s.u32()
	if err != nil {
		return nil, fmt.Errorf("读取版本失败：%w", err)
	}
	if ver < 2 || ver > 3 {
		return nil, fmt.Errorf("不支持的 GGUF 版本 %d", ver)
	}
	if _, err := s.u64(); err != nil { // tensor_count：本探测器用不上
		return nil, fmt.Errorf("读取 tensor_count 失败：%w", err)
	}
	kvCount, err := s.u64()
	if err != nil {
		return nil, fmt.Errorf("读取 kv_count 失败：%w", err)
	}
	if kvCount > 1<<20 {
		return nil, fmt.Errorf("kv_count 异常：%d", kvCount)
	}
	meta := &GGUFMeta{Version: int(ver), KeyCount: int(kvCount)}
	for i := uint64(0); i < kvCount; i++ {
		key, err := s.str()
		if err != nil {
			meta.Truncated = true
			return meta, nil
		}
		vt, err := s.u32()
		if err != nil {
			meta.Truncated = true
			return meta, nil
		}
		if err := s.assign(meta, key, vt); err != nil {
			if errors.Is(err, errGGUFBudget) {
				meta.Truncated = true
				return meta, nil
			}
			return meta, fmt.Errorf("解析键 %q 失败：%w", key, err)
		}
	}
	meta.BytesRead = s.used
	return meta, nil
}

// ProbeMetaGGUFFile 读一个 .gguf 的元数据，并给出探测留痕（probe.meta.gguf.v1）。
func ProbeMetaGGUFFile(path string) (*GGUFMeta, Trace, error) {
	tr := Trace{Probe: EvidenceMetaGGUF}
	start := time.Now()
	f, err := os.Open(path)
	if err != nil {
		tr.ElapsedMS = time.Since(start).Milliseconds()
		tr.FailureClass = FailNoMeta
		tr.Summary = truncate(err.Error(), 200)
		return nil, tr, err
	}
	defer f.Close()

	meta, perr := parseGGUF(newGGUFScanner(f))
	tr.ElapsedMS = time.Since(start).Milliseconds()
	if meta != nil {
		dim, derived := meta.HeadDim()
		tr.Summary = fmt.Sprintf("arch=%s context_length=%d layers=%d kv_heads=%d head_dim=%d(head_dim_derived=%v) chat_template=%v license=%q keys=%d bytes=%d truncated=%v",
			meta.Architecture, meta.ContextWindow, meta.BlockCount, meta.KVHeads(), dim, derived,
			meta.ChatTemplate != "", meta.LicenseSPDX, meta.KeyCount, meta.BytesRead, meta.Truncated)
	}
	if perr != nil {
		tr.FailureClass = FailNoMeta
		tr.Summary = truncate(perr.Error(), 200)
		return nil, tr, perr
	}
	tr.OK = true
	return meta, tr, nil
}

// Package eggpkg —— 卵的单文件封装（*.egg）在**子端侧**的只读解析与展开规划。
//
// 格式 v1 与 `scripts/x3/zerg-egg.py` **同源**（固定 32 字节前缀 + 索引 JSON + 按 align 对齐的数据段）：
//
//	[0:4) magic "ZEGG" | [4:8) version(u32 LE) | [8:16) align(u64 LE) | [16:24) index_len(u64 LE) | [24:32) 保留
//
// 不变量（违背即拒绝，照 §11.2）：align 是 2 的幂且 ≥4096；段偏移对齐到 align；段不出文件末尾；
// path 为相对路径且不含 ".."；读端**只信文件里记的 align**，绝不按运行平台硬编码。
//
// 本包是**纯读取与规划**：真正的孵化（把段落进空间）由孵化器按本包给出的计划执行（§9.6 第 3 步）。
package eggpkg

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path"
	"strings"
)

const (
	Magic         = "ZEGG"
	FormatVersion = 1
	HeaderPrefix  = 32
	MinAlign      = 4096
	MaxIndexLen   = 1 << 22 // 索引上限（4 MiB，防畸形文件把内存吃光）
)

// Entry 一段数据（一个文件的完整内容）。
type Entry struct {
	Path   string `json:"path"`
	Off    int64  `json:"off"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// Index 卵的索引（与 Python 侧的 build_index 同形）。
type Index struct {
	Magic   string `json:"magic"`
	Version int    `json:"version"`
	Align   int64  `json:"align"`
	Tool    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"tool"`
	CreatedAt string  `json:"created_at"`
	Entries   []Entry `json:"entries"`
}

// Plan 一段的展开动作（孵化器照此把段落到空间内，并复核 sha）。
type Plan struct {
	Egg      string
	Align    int64
	Targets  []Target
	TotalOut int64
}

// Target 单段落点。
type Target struct {
	Path   string // 空间内相对路径
	Off    int64  // 段在卵里的偏移
	Size   int64
	SHA256 string
	Mode   uint32
}

// ReadIndex 读取并校验索引（前缀 + JSON + 不变量）。
func ReadIndex(egg string) (*Index, error) {
	f, err := os.Open(egg)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hdr := make([]byte, HeaderPrefix)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, fmt.Errorf("卵太短，读不到 %d 字节前缀：%w", HeaderPrefix, err)
	}
	if string(hdr[0:4]) != Magic {
		return nil, fmt.Errorf("magic 不符（不是 %s 文件）：%q", Magic, string(hdr[0:4]))
	}
	ver := binary.LittleEndian.Uint32(hdr[4:8])
	if ver != FormatVersion {
		return nil, fmt.Errorf("格式版本不支持：%d", ver)
	}
	align := int64(binary.LittleEndian.Uint64(hdr[8:16]))
	n := int64(binary.LittleEndian.Uint64(hdr[16:24]))
	if align < MinAlign || bits.OnesCount64(uint64(align)) != 1 {
		return nil, fmt.Errorf("align 不合理（须为 2 的幂且 ≥%d）：%d", MinAlign, align)
	}
	if n <= 0 || n > MaxIndexLen || n > align-HeaderPrefix {
		return nil, fmt.Errorf("索引长度不合理：%d", n)
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	idx := &Index{}
	dec := json.NewDecoder(io.LimitReader(f, n))
	if err := dec.Decode(idx); err != nil {
		return nil, fmt.Errorf("索引 JSON 解析失败：%w", err)
	}
	if idx.Align != align {
		return nil, fmt.Errorf("索引里的 align(%d) 与前缀(%d) 不一致", idx.Align, align)
	}
	if err := idx.validate(st.Size()); err != nil {
		return nil, err
	}
	return idx, nil
}

func (ix *Index) validate(fileSize int64) error {
	if len(ix.Entries) == 0 {
		return fmt.Errorf("索引里没有段")
	}
	seen := map[string]bool{}
	for i, e := range ix.Entries {
		if e.Path == "" || strings.HasPrefix(e.Path, "/") || e.Path != path.Clean(e.Path) {
			return fmt.Errorf("段 %d 的 path 不规范：%q", i, e.Path)
		}
		if strings.HasPrefix(e.Path, "..") || strings.Contains(e.Path, "../") {
			return fmt.Errorf("段 %d 的 path 越界（含 ..）：%q", i, e.Path)
		}
		if seen[e.Path] {
			return fmt.Errorf("段 %d 的 path 重复：%q", i, e.Path)
		}
		seen[e.Path] = true
		if e.Size < 0 || e.Off < 0 {
			return fmt.Errorf("段 %d 的 off/size 为负", i)
		}
		if e.Off%ix.Align != 0 {
			return fmt.Errorf("段 %q 偏移未对齐（%d %% %d != 0）", e.Path, e.Off, ix.Align)
		}
		if e.Off+e.Size > fileSize {
			return fmt.Errorf("段 %q 越出文件末尾（%d+%d > %d）", e.Path, e.Off, e.Size, fileSize)
		}
		if len(e.SHA256) != 64 {
			return fmt.Errorf("段 %q 的 sha256 长度不对：%q", e.Path, e.SHA256)
		}
	}
	return nil
}

// BuildPlan 依据索引产出展开计划（纯函数：只读索引，不碰文件内容、不写盘）。
// 孵化器按 Targets 把每段落到空间内，并逐段复核 sha（§六 判据 3）。
func BuildPlan(egg string, ix *Index) (*Plan, error) {
	if ix == nil {
		return nil, fmt.Errorf("索引为 nil")
	}
	p := &Plan{Egg: egg, Align: ix.Align}
	for _, e := range ix.Entries {
		p.Targets = append(p.Targets, Target{Path: e.Path, Off: e.Off, Size: e.Size, SHA256: e.SHA256, Mode: e.Mode})
		p.TotalOut += e.Size
	}
	return p, nil
}

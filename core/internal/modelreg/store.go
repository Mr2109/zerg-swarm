package modelreg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ── 批 3：接线——让记录真正进「目录」──────────────────────────────────────
//
// 《开工方案-模型探测与校验.md》§七 批 3 的原文定义：
//
//	「接线：`probe --out` 落到 `~/.zerg/models/manifests/` + `list` 子命令
//	 —— 让记录真正进"目录"」
//
// 位置与门禁由《标准-模型接入与目录贡献.md》定：
//   - §三：记录位置 = ~/.zerg/models/manifests/<model_id>/<version>.json
//   - §十二.1（已拍板）：目录记录不合标准 = **直接拒**，不做"先合进来再修"
//
// 本批只碰 manifests/**：不搬权重、不建 blobs/refs（开工方案 §二 家目录约定）。
// 不碰运行中的主控与 UI，纯本地文件操作（开工方案 §八 回退）。
//
// 两条写盘纪律（本批硬要求）：
//   - **原子**：先写同目录临时文件，再 rename 到位（同分区 rename 是原子的）
//   - **幂等**：内容与目录里已有的完全相同时不重写，重复跑结果一致

// ModelsDirEnv 是模型目录根的环境变量（不设时用 ~/.zerg/models）。
// 存在的理由：测试要写到 t.TempDir()、多库/离线分发要换位置，都不该改代码。
const ModelsDirEnv = "ZERG_MODELS_DIR"

// DefaultModelsDir 返回模型目录根（默认 ~/.zerg/models）。
func DefaultModelsDir() string {
	if v := strings.TrimSpace(os.Getenv(ModelsDirEnv)); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".zerg", "models")
	}
	return filepath.Join(home, ".zerg", "models")
}

// Store 是模型目录的读写层。
//
// Root = 模型目录根（默认 ~/.zerg/models），记录落在 <Root>/manifests/ 下。
// ManifestsRoot 非空时直接用它当 manifests 目录（给 `probe --out <目录>` 用：
// 用户显式指一个目录，就是那个目录当作记录目录，不再套一层 manifests/）。
type Store struct {
	Root          string
	ManifestsRoot string
}

// NewStore 用一个模型目录根构造 Store（空 = 默认根）。
func NewStore(root string) *Store {
	if strings.TrimSpace(root) == "" {
		root = DefaultModelsDir()
	}
	return &Store{Root: root}
}

// NewStoreAtManifests 直接用给定的 manifests 目录构造 Store。
func NewStoreAtManifests(dir string) *Store { return &Store{ManifestsRoot: dir} }

// ManifestsDir 是记录目录（标准 §三）。
func (s *Store) ManifestsDir() string {
	if s.ManifestsRoot != "" {
		return s.ManifestsRoot
	}
	return filepath.Join(s.Root, "manifests")
}

// VersionOf 由 digest 推出记录版本号。
//
// 为什么版本 = 摘要前缀：标准 §二 的底线是"摘要即身份"，于是同一组建材无论
// 文件名/目录/重复探测多少次，都必然落到同一个 <version>.json —— 这是"入目录"
// 天然幂等的根据（不引日期、不引序号、不引计数器）。
func VersionOf(rec *Record) string {
	d := strings.TrimPrefix(strings.TrimSpace(rec.Digest), "sha256:")
	if len(d) > 12 {
		d = d[:12]
	}
	if d == "" {
		d = "unknown"
	}
	return "sha256-" + d
}

// RecordPath 是某条记录在目录里的位置（标准 §三 布局）。
func (s *Store) RecordPath(id, version string) string {
	return filepath.Join(s.ManifestsDir(), id, version+".json")
}

// PathFor 给出某条记录的落盘路径。
func (s *Store) PathFor(rec *Record) string { return s.RecordPath(rec.ID, VersionOf(rec)) }

// AdmissionError 表示记录被门禁拒绝，**什么都没写**。
// 标准 §十二.1：verify 不过直接拒。findings 原样带上，包含 error 级结论。
type AdmissionError struct {
	Path     string
	Findings []Finding
}

func (e *AdmissionError) Error() string {
	return fmt.Sprintf("记录不合标准，拒绝入目录（error %d 条）：%s", CountErrors(e.Findings), e.Path)
}

// ConflictError 表示目标位置已有**内容不同**的记录。
//
// 为什么不覆盖：目录里的记录承载人工补的许可证留痕（accepted_by/accepted_at，
// 法律留痕）。同摘要同路径被静默改写 = 可能把人工留痕抹掉，而"删/改Mr2109的东西"
// 必须由人点头。因此这里保守拒绝，把决定权交回人工（刷新语义属后续批）。
type ConflictError struct {
	Path string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("目录里已有同摘要但内容不同的记录，拒绝覆盖：%s", e.Path)
}

// PutResult 是一次入目录的结论。
// Changed=false 表示内容与目录里已有的完全一致（幂等重跑，文件未被重写）。
type PutResult struct {
	Path     string
	Version  string
	Changed  bool
	Findings []Finding
}

// Put 把一条记录写进目录：先过 verify 门禁，再原子落盘。
//
//	门禁不通过 → *AdmissionError，什么都不写（不许用占位值把 error 骗成绿）
//	已有同内容 → Changed=false，不重写（幂等）
//	已有不同内容 → *ConflictError，不覆盖（保护人工留痕）
func (s *Store) Put(rec *Record, strict bool) (PutResult, error) {
	res := PutResult{Version: VersionOf(rec), Path: s.PathFor(rec)}
	res.Findings = Verify(rec, strict)
	if n := CountErrors(res.Findings); n > 0 {
		return res, &AdmissionError{Path: res.Path, Findings: res.Findings}
	}
	if strict {
		for _, f := range res.Findings {
			if f.Level == "warn" {
				return res, &AdmissionError{Path: res.Path, Findings: res.Findings}
			}
		}
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return res, fmt.Errorf("序列化记录失败：%w", err)
	}
	data = append(data, '\n')

	if old, rerr := os.ReadFile(res.Path); rerr == nil {
		if bytes.Equal(old, data) {
			return res, nil // 幂等：内容一致，不动文件
		}
		return res, &ConflictError{Path: res.Path}
	}
	if _, werr := WriteFileAtomic(res.Path, data); werr != nil {
		return res, werr
	}
	res.Changed = true
	return res, nil
}

// WriteFileAtomic 原子写盘：先写同目录临时文件（同一分区），fsync 后 rename 到位。
// 内容与已有文件完全一致时不重写，返回 changed=false（幂等）。
// 失败路径不留临时文件（defer remove；rename 成功后那个 remove 是 no-op）。
//
// 写入权限（待修补 #3 阶段 1）：目录 0o700、文件 0o600 —— **仅属主可读写/进入**。
// 为什么收紧：manifests 是本地身份库，将来要承载签名记录（阶段 2）；把「谁能把文件
// 放进来 / 读出去」先压到最小（同机其他用户与组都进不来），是零成本的第一道防线。
// 边界（如实说）：这只挡本机其他用户，挡不住属主自己、root，也不提供任何**防伪**
// ——记录的真伪只能靠签名与官方来源交叉核对（阶段 2），权限不是签名。
// 目录 ModeDir / 文件 ModeFile 是本批唯一权限口径，改动前先看标准
// 《写入权限与防伪（阶段 1 / 阶段 2 边界）》。
func WriteFileAtomic(path string, data []byte) (bool, error) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return false, nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, ModeDir); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmpName, ModeFile); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, err
	}
	return true, nil
}

// StoredRecord 是 list 的一行：目录里一条记录的可见信息 + 它的校验结论。
type StoredRecord struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Path         string   `json:"path"`
	Digest       string   `json:"digest,omitempty"`
	Name         string   `json:"name,omitempty"`
	Commercial   string   `json:"commercial,omitempty"`
	State        string   `json:"state,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Files        int      `json:"files"`
	Errors       int      `json:"errors"`
	Warns        int      `json:"warns"`
	// DefaultEligible 编码标准 §五 的红线：commercial 非 yes 的记录
	// 不得作为任何默认项进入目录、路由池或预设。此处只做标注，供后续路由批次取用。
	DefaultEligible bool   `json:"default_eligible"`
	Err             string `json:"error,omitempty"`
}

// List 扫记录目录，按 (id, version) 排序返回。
//
// 只读：目录不存在 = 空目录，不报错也**不创建**（创建只发生在写入路径）。
// 单个文件读不动/不合 JSON → 该行如实记 Err，不中断整个列表（反例优先：
// 目录里混进坏文件时 list 必须还能用）。
func (s *Store) List() ([]StoredRecord, error) {
	entries, err := os.ReadDir(s.ManifestsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []StoredRecord
	for _, idEnt := range entries {
		if !idEnt.IsDir() || strings.HasPrefix(idEnt.Name(), ".") {
			continue
		}
		id := idEnt.Name()
		idDir := filepath.Join(s.ManifestsDir(), id)
		files, err := os.ReadDir(idDir)
		if err != nil {
			out = append(out, StoredRecord{ID: id, Path: idDir, Err: err.Error()})
			continue
		}
		for _, f := range files {
			name := f.Name()
			if f.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
				continue
			}
			// 待修补 #24 / 能力快照：记录旁的兄弟文件（<version>.trace.json 探测留痕、
			// <version>.capabilities.json 能力快照）不是记录，不进目录语义——list 必须
			// 跳过它们，否则会当成一条坏记录报 error。
			if strings.HasSuffix(name, ".trace.json") || strings.HasSuffix(name, ".capabilities.json") {
				continue
			}
			p := filepath.Join(idDir, name)
			row := StoredRecord{ID: id, Version: strings.TrimSuffix(name, ".json"), Path: p}
			rec, err := Load(p)
			if err != nil {
				row.Err = err.Error()
				out = append(out, row)
				continue
			}
			row.Digest = rec.Digest
			row.Name = rec.Name
			row.Commercial = rec.License.Commercial
			row.State = rec.State
			row.Files = len(rec.Files)
			for _, c := range rec.Capabilities {
				if c.Value {
					row.Capabilities = append(row.Capabilities, c.Name)
				}
			}
			findings := Verify(rec, false)
			row.Errors = CountErrors(findings)
			row.Warns = len(findings) - row.Errors
			row.DefaultEligible = rec.License.Commercial == "yes"
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Version < out[j].Version
	})
	return out, nil
}

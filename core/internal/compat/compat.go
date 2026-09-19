// compat.go — B7 / G10：跨版本状态文件的**兼容清单**（单一真源 + 进程启动时注册）。
//
// 背景（设计稿 §十 G10）：源码式升级（B0–B6）把客户端变成「拉源码自己编」，于是同一台机器上
// 会先后出现**写状态文件的旧版二进制**与**读它/写它的新版二进制**。没有 schema 版本号时，
// 一次升级就可能把用户状态读坏或写丢（「升级即丢配置」）。
//
// 本文件解决三件事：
//  1. **单一真源**：compat.json（go:embed）声明【文件 → 当前 schema → 最低可读版本 → 迁移函数 → 读写锚点】；
//  2. **清单自检**：MustManifest() 在首次访问时解析 + 自检，清单坏了直接 panic（编译期级错误，绝不静默降级）；
//  3. **双向一致性**：本文件只负责「清单 → 代码」这一半（迁移函数必须已在 migrations 里注册、锚点字段非空）；
//     另一半「代码 → 清单」（新状态文件必须登记）由门禁 scripts/gates/check-compat-manifest.py 扫源码完成。
//
// 与 statepath 的关系：statepath 是**路径**的单一真源，本包是**跨版本可读性**的单一真源；
// 路径解析统一走 statepath.Dir（清单里 dirs.state.resolver 如实标注）。
package compat

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

//go:embed compat.json
var manifestJSON []byte

// Entry —— 清单里的一条「跨版本状态文件」。
type Entry struct {
	Name          string `json:"name"`           // 逻辑名（compat.Read/Write 用它查找）
	File          string `json:"file"`           // 状态目录下的相对路径（kind=glob 时含 *）
	Dir           string `json:"dir"`            // state | receipts（见清单 dirs）
	Owner         string `json:"owner"`          // go | ui —— 谁写这个文件
	Kind          string `json:"kind"`           // file | glob（glob=一组同类文件）
	Envelope      string `json:"envelope"`       // inband（信封内带 schema 字段）| sidecar（旁路文件带版本）
	SchemaField   string `json:"schema_field"`   // inband 时的字段名（有的文件原生叫 version）
	CurrentSchema int    `json:"current_schema"` // 本进程认知的当前 schema
	MinReadable   int    `json:"min_readable"`   // 本进程仍能读懂的最低 schema
	MigrateFunc   string `json:"migrate_func"`   // 迁移函数名（Go 侧须有 func migrate<Name>）
	StampsSchema  string `json:"stamps_schema"`  // layer | native | sidecar | none —— 谁负责盖版本号
	AnchorFile    string `json:"anchor_file"`    // 读写实现所在文件（门禁用）
	Anchor        string `json:"anchor"`         // 该文件里必须存在的符号/字面量（门禁用）
	Note          string `json:"note,omitempty"`
}

// Excluded —— 明确**不**进 schema 化清单的状态文件（必须给理由，门禁强制非空）。
type Excluded struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// DirSpec —— 一组状态目录的解析方式（环境变量覆盖 + 默认值）。
type DirSpec struct {
	Resolver string `json:"resolver"`
	Env      string `json:"env"`
	Default  string `json:"default"`
}

// Manifest —— compat.json 的内存形态。
type Manifest struct {
	Schema   int                `json:"schema"`
	Note     string             `json:"note"`
	Dirs     map[string]DirSpec `json:"dirs"`
	Entries  []Entry            `json:"entries"`
	Excluded []Excluded         `json:"excluded"`

	byName map[string]Entry
}

var (
	manifestOnce sync.Once
	manifestVal  *Manifest
	manifestErr  error
)

// MustManifest —— 解析并自检清单；任何问题都 panic。
//
// 为什么是 panic 而不是返回 error：清单是**编译进二进制**的资产（go:embed），它不自洽就是
// 构建/部署错误，必须在进程启动的第一秒炸出来，而不是等到某个状态文件读坏用户的配置才发现。
func MustManifest() *Manifest {
	manifestOnce.Do(func() {
		m := &Manifest{}
		if err := json.Unmarshal(manifestJSON, m); err != nil {
			manifestErr = fmt.Errorf("compat: 兼容清单解析失败: %w", err)
			panic(manifestErr)
		}
		m.byName = make(map[string]Entry, len(m.Entries))
		for _, e := range m.Entries {
			m.byName[e.Name] = e
		}
		if probs := m.Check(); len(probs) > 0 {
			manifestErr = fmt.Errorf("compat: 兼容清单自检失败: %s", strings.Join(probs, "; "))
			panic(manifestErr)
		}
		manifestVal = m
	})
	if manifestVal == nil {
		panic(manifestErr)
	}
	return manifestVal
}

// Entries —— 全部条目（拷贝，调用方可安全改动）。
func Entries() []Entry { return append([]Entry(nil), MustManifest().Entries...) }

// ManifestJSONForDump —— 内嵌清单的原文（CLI `manifest` 与 CI 门禁的对照面）。
func ManifestJSONForDump() []byte { return append([]byte(nil), manifestJSON...) }

// Lookup —— 按逻辑名查条目。
func Lookup(name string) (Entry, bool) {
	e, ok := MustManifest().byName[name]
	return e, ok
}

// Check —— 清单 vs 代码的自检（返回问题列表；空=通过）。CLI 与单测都走它。
//
// 只覆盖「清单 → 代码」这一半（迁移函数已注册、字段合法、锚点非空）；
// 「代码 → 清单」由 scripts/gates/check-compat-manifest.py 扫源码覆盖。
func (m *Manifest) Check() []string {
	var probs []string
	if m.Schema < 1 {
		probs = append(probs, fmt.Sprintf("清单自身的 schema=%d 非法（须 ≥1）", m.Schema))
	}
	for key, d := range m.Dirs {
		if strings.TrimSpace(d.Resolver) == "" || strings.TrimSpace(d.Env) == "" || strings.TrimSpace(d.Default) == "" {
			probs = append(probs, fmt.Sprintf("dirs.%s 的 resolver/env/default 不得为空", key))
		}
	}
	seenName := map[string]bool{}
	seenFile := map[string]bool{}
	for i, e := range m.Entries {
		where := fmt.Sprintf("entries[%d](%s)", i, e.Name)
		if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.File) == "" {
			probs = append(probs, where+": name/file 不得为空")
			continue
		}
		if seenName[e.Name] {
			probs = append(probs, where+": name 重复")
		}
		seenName[e.Name] = true
		key := e.Dir + "/" + e.File
		if seenFile[key] {
			probs = append(probs, where+": file 重复（"+key+"）")
		}
		seenFile[key] = true

		if e.Owner != "go" && e.Owner != "ui" {
			probs = append(probs, where+": owner 必须是 go|ui，实际 "+e.Owner)
		}
		if e.Kind != "file" && e.Kind != "glob" {
			probs = append(probs, where+": kind 必须是 file|glob，实际 "+e.Kind)
		}
		if e.Envelope != "inband" && e.Envelope != "sidecar" {
			probs = append(probs, where+": envelope 必须是 inband|sidecar，实际 "+e.Envelope)
		}
		switch e.StampsSchema {
		case "layer", "native", "sidecar", "none":
		default:
			probs = append(probs, where+": stamps_schema 非法（"+e.StampsSchema+"）")
		}
		if e.CurrentSchema < 1 {
			probs = append(probs, fmt.Sprintf("%s: current_schema=%d 必须 ≥1（schema 化的意义就是有版本号）", where, e.CurrentSchema))
		}
		if e.MinReadable < 0 {
			probs = append(probs, fmt.Sprintf("%s: min_readable=%d 不得为负", where, e.MinReadable))
		}
		// 承重不变量：当前 schema 必须 ≥ 最低可读版本（否则声明自相矛盾）
		if e.CurrentSchema < e.MinReadable {
			probs = append(probs, fmt.Sprintf("%s: current_schema=%d < min_readable=%d —— 声明自相矛盾", where, e.CurrentSchema, e.MinReadable))
		}
		if _, ok := migrations[e.MigrateFunc]; !ok {
			probs = append(probs, fmt.Sprintf("%s: migrate_func=%q 未在 migrations 注册（Go 侧缺 func migrate%s）", where, e.MigrateFunc, e.MigrateFunc))
		}
		if e.Envelope == "inband" && strings.TrimSpace(e.SchemaField) == "" {
			probs = append(probs, where+": inband 信封必须给 schema_field")
		}
		if strings.TrimSpace(e.AnchorFile) == "" || strings.TrimSpace(e.Anchor) == "" {
			probs = append(probs, where+": anchor_file/anchor 不得为空（门禁靠它证明「真有读写实现」）")
		}
	}
	for i, x := range m.Excluded {
		if strings.TrimSpace(x.File) == "" {
			probs = append(probs, fmt.Sprintf("excluded[%d]: file 不得为空", i))
		}
		if strings.TrimSpace(x.Reason) == "" {
			probs = append(probs, fmt.Sprintf("excluded[%d](%s): 必须给出不登记的理由（不许无理由排除）", i, x.File))
		}
		if seenFile["state/"+x.File] || seenFile["receipts/"+x.File] {
			probs = append(probs, fmt.Sprintf("excluded[%d](%s): 同时出现在 entries —— 二者互斥", i, x.File))
		}
	}
	sort.Strings(probs)
	return probs
}

// ───────────────────────── 路径解析 ─────────────────────────

// Dirs —— 状态目录集合（生产走 DefaultDirs；测试必须注入 t.TempDir()，绝不碰真机状态）。
type Dirs struct {
	State    string
	Receipts string
}

// DefaultDirs —— 生产路径：state=statepath.Dir（ZERG_STATE_DIR 覆盖），receipts=~/.zerg/update_receipts。
func DefaultDirs() Dirs {
	return Dirs{State: statepath.Dir(), Receipts: ReceiptsDir()}
}

// ReceiptsDir —— 升级回执目录（ZERG_RECEIPTS_DIR 覆盖 → ~/.zerg/update_receipts）。
// 与 core/internal/selfupdate 的 Options.Receipts 同源（同一环境变量、同一默认值）。
func ReceiptsDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_RECEIPTS_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "zerg-update_receipts")
	}
	return filepath.Join(home, ".zerg", "update_receipts")
}

func (d Dirs) base(dir string) string {
	switch dir {
	case "receipts":
		return d.Receipts
	default:
		return d.State
	}
}

// Path —— 条目的单文件路径（kind=glob 时含通配符）。
func (e Entry) Path(d Dirs) string { return filepath.Join(d.base(e.Dir), e.File) }

// Paths —— 条目涉及的文件列表（glob 展开；不存在的文件不返回）。
func (e Entry) Paths(d Dirs) ([]string, error) {
	if e.Kind != "glob" {
		return []string{e.Path(d)}, nil
	}
	ms, err := filepath.Glob(e.Path(d))
	if err != nil {
		return nil, fmt.Errorf("compat: 展开 %s 失败: %w", e.Path(d), err)
	}
	sort.Strings(ms)
	return ms, nil
}

// SidecarPath —— sidecar 信封的版本文件路径：`<file>.schema.json`。
// 为什么单独一个文件：见清单里 ui_layout / ui_modules / ui_external_modules / tool_uses 的 note
// ——那几个载荷的形状是「每个键都是数据」，在信封里塞 schema 会被读侧当成数据本身，
// 个别形状（map<string,bool>）甚至会让整份解析失败、把用户状态整个吃掉。
func (e Entry) SidecarPath(file string) string { return file + ".schema.json" }

// Inband —— 信封内携带 schema 字段。
func (e Entry) Inband() bool { return e.Envelope == "inband" }

// Sidecar —— 版本号走旁路文件。
func (e Entry) Sidecar() bool { return e.Envelope == "sidecar" }

// SchemaKey —— 信封内 schema 字段名（默认 "schema"）。
func (e Entry) SchemaKey() string {
	if strings.TrimSpace(e.SchemaField) != "" {
		return e.SchemaField
	}
	return "schema"
}

// migrate.go — 带 schema 版本号的读写与**一次性迁移**层（B7 / G10 的执行体）。
//
// 四条纪律（对应设计稿 §十 G10 与 skill 里的状态文件纪律）：
//  1. **向后兼容**：读到旧版（无 schema 或低于 current）⇒ 走一次性迁移：先备份 `<file>.bak-<ts>`，
//     再原地写回带 current 的文件，最后**回读校验**（schema 到位 + 旧顶层键一个不少），
//     校验不过就回滚成原文件并报错。迁移幂等：二跑看到 schema==current 直接短路，不写盘、不再备份。
//  2. **向前兼容（不猜）**：读到**高于**本进程认知的 schema ⇒ 既不迁移也不回写，返回 OutcomeFuture；
//     调用方按「只读降级」用已知字段解析，并提示用户升级本机二进制。
//     为什么不猜：高版本可能改了字段语义（同一键换了含义），照旧解析会把新语义读成旧含义——
//     那比读不到更危险（会基于误解写坏状态）。所以宁可只读已知字段 + 显式告警。
//  3. **不静默丢状态**：载荷非对象、JSON 损坏、备份失败、回读校验失败——一律走 Err* 返回 + 日志留痕，
//     且**原文件保持不动**（失败不许半写）。
//  4. **不碰真机**：所有路径来自注入的 Dirs；生产走 DefaultDirs()，测试必须注入 t.TempDir()。
//     本层自己不做任何 ~/.zerg 兜底——目录从哪来由调用方决定，测试就不可能误写真实状态。
package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Outcome —— 一次读/迁移的结论（可判定，不是「成功了/失败了」两态）。
type Outcome string

const (
	OutcomeMissing  Outcome = "missing"  // 文件不存在（首次运行）——非错误
	OutcomeCurrent  Outcome = "current"  // schema == 本进程认知，可直接用（幂等短路的落点）
	OutcomeMigrated Outcome = "migrated" // 旧版/无 schema → 已迁移 + 回读校验通过
	OutcomeFuture   Outcome = "future"   // 高于本机认知 → 只读降级，绝不迁移/回写
	OutcomeCorrupt  Outcome = "corrupt"  // 非法 JSON / 信封形状不符 / 迁移或校验失败
)

// 哨兵错误：调用方（以及 zerg-compat 的退出码）靠 errors.Is 分流，不靠字符串匹配。
var (
	ErrSchemaTooNew = errors.New("状态文件 schema 高于本机二进制认知——不猜测，请升级本机二进制")
	ErrCorrupt      = errors.New("状态文件不是合法 JSON")
	ErrNotAnObject  = errors.New("状态文件信封不是 JSON 对象（该形状请用 sidecar 信封）")
	ErrVerifyFailed = errors.New("迁移后回读校验失败")
)

// Config —— 注入点（目录 / 日志 / 时钟）。零值不可用，请用 DefaultConfig() 或显式构造。
type Config struct {
	Dirs Dirs
	Logf func(format string, args ...any) // nil ⇒ log.Printf
	Now  func() time.Time                 // nil ⇒ time.Now
}

// DefaultConfig —— 生产配置：真机状态目录 + 标准日志 + 真实时钟。
func DefaultConfig() Config {
	return Config{
		Dirs: DefaultDirs(),
		Logf: log.Printf,
		Now:  time.Now,
	}
}

func (c Config) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// FileStatus —— 不落盘的现状快照（CLI `check` 与排障用）。
type FileStatus struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Outcome     Outcome `json:"outcome"`
	OnDisk      int     `json:"on_disk_schema"` // -1 = 文件里没有版本号
	Expected    int     `json:"current_schema"`
	MinReadable int     `json:"min_readable"`
	Envelope    string  `json:"envelope"`
	Owner       string  `json:"owner"`
	Err         string  `json:"error,omitempty"`
}

// checkDirs —— 目录必须由调用方注入。空目录会被 filepath.Join 解析成**当前工作目录**，
// 那正好是本项目最忌讳的一类事故（测试/工具误写真机或误写仓库）。所以宁可直接报错。
func (c Config) checkDirs(e Entry) error {
	if strings.TrimSpace(c.Dirs.base(e.Dir)) == "" {
		return fmt.Errorf("compat: %s 的状态目录为空——请显式注入（生产用 DefaultConfig()，测试用 t.TempDir()）", e.Name)
	}
	return nil
}

// Read —— 读一个跨版本状态文件（需要迁移就顺带做一次性迁移）。
//
// 返回的 data 是「本进程可读」的字节；Outcome=Future 时 data 是**未改动的原文**（只读降级）。
// kind=glob（一组同类文件）不提供单文件读——请用 MigrateAll / Status。
func Read(name string, cfg Config) ([]byte, Outcome, error) {
	e, ok := Lookup(name)
	if !ok {
		return nil, "", fmt.Errorf("compat: 未登记的条目 %q（跨版本状态文件必须先写进 compat.json）", name)
	}
	if e.Kind != "file" {
		return nil, "", fmt.Errorf("compat: %s 是 %s 条目，不支持单文件读——用 MigrateAll/Status", name, e.Kind)
	}
	if err := cfg.checkDirs(e); err != nil {
		return nil, "", err
	}
	return readPath(e, e.Path(cfg.Dirs), cfg)
}

// Write —— 以当前 schema 原子写回（inband 时注入 schema 字段；sidecar 时另写版本旁路文件）。
//
// 拒绝条件：磁盘上的文件已经带着**更高**的 schema ⇒ ErrSchemaTooNew，绝不拿旧认知盖掉新版状态。
func Write(name string, payload any, cfg Config) (string, error) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("compat: 序列化 %s 失败: %w", name, err)
	}
	return WriteRaw(name, b, cfg)
}

// WriteRaw —— 与 Write 相同，但载荷已经是编码好的 JSON。
func WriteRaw(name string, payload []byte, cfg Config) (string, error) {
	e, ok := Lookup(name)
	if !ok {
		return "", fmt.Errorf("compat: 未登记的条目 %q", name)
	}
	if e.Kind != "file" {
		return "", fmt.Errorf("compat: %s 是 %s 条目，不支持单文件写", name, e.Kind)
	}
	if err := cfg.checkDirs(e); err != nil {
		return "", err
	}
	path := e.Path(cfg.Dirs)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("compat: 建目录 %s 失败: %w", filepath.Dir(path), err)
	}
	if e.Inband() {
		if onDisk, has, err := onDiskSchema(e, path); err == nil && has && onDisk > e.CurrentSchema {
			return "", fmt.Errorf("%w（%s 的 %s=%d > 本机 %d）", ErrSchemaTooNew, path, e.SchemaKey(), onDisk, e.CurrentSchema)
		}
		obj, err := decodeObject(payload)
		if err != nil {
			return "", err
		}
		if v, has := schemaOf(obj, e.SchemaKey()); !has || v != e.CurrentSchema {
			obj[e.SchemaKey()] = json.RawMessage(strconv.Itoa(e.CurrentSchema))
		}
		out, err := marshalOrdered(e.SchemaKey(), obj)
		if err != nil {
			return "", err
		}
		if err := writeFileAtomic(path, out); err != nil {
			return "", err
		}
		return path, nil
	}
	// sidecar：载荷原样落盘 + 版本号进旁路文件
	if _, err := validateJSON(payload); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, payload); err != nil {
		return "", err
	}
	if err := writeSidecar(e, path, cfg); err != nil {
		return "", err
	}
	return path, nil
}

// Report —— MigrateAll 的逐文件结论。
type Report struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Outcome Outcome `json:"outcome"`
	Err     string  `json:"error,omitempty"`
}

// MigrateAll —— 对清单里**每个**条目（含 glob 展开）执行一次性迁移；幂等，可反复跑。
// 返回逐文件结论；任何一条 Corrupt 都不影响其它条（一个坏文件不该拖死全部状态）。
func MigrateAll(cfg Config) []Report {
	var out []Report
	for _, e := range Entries() {
		if err := cfg.checkDirs(e); err != nil {
			out = append(out, Report{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeCorrupt, Err: err.Error()})
			continue
		}
		paths, err := e.Paths(cfg.Dirs)
		if err != nil {
			out = append(out, Report{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeCorrupt, Err: err.Error()})
			continue
		}
		if len(paths) == 0 {
			out = append(out, Report{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeMissing})
			continue
		}
		for _, p := range paths {
			_, outcome, rerr := readPath(e, p, cfg)
			rep := Report{Name: e.Name, Path: p, Outcome: outcome}
			if rerr != nil {
				rep.Err = rerr.Error()
			}
			out = append(out, rep)
		}
	}
	return out
}

// StatusAll —— 只读现状（不迁移、不备份、不写盘）——CLI `check` 用。
func StatusAll(cfg Config) []FileStatus {
	var out []FileStatus
	for _, e := range Entries() {
		if err := cfg.checkDirs(e); err != nil {
			out = append(out, FileStatus{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeCorrupt, Err: err.Error(),
				Expected: e.CurrentSchema, MinReadable: e.MinReadable, Envelope: e.Envelope, Owner: e.Owner, OnDisk: -1})
			continue
		}
		paths, err := e.Paths(cfg.Dirs)
		if err != nil {
			out = append(out, FileStatus{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeCorrupt, Err: err.Error(),
				Expected: e.CurrentSchema, MinReadable: e.MinReadable, Envelope: e.Envelope, Owner: e.Owner, OnDisk: -1})
			continue
		}
		if len(paths) == 0 {
			out = append(out, FileStatus{Name: e.Name, Path: e.Path(cfg.Dirs), Outcome: OutcomeMissing, OnDisk: -1,
				Expected: e.CurrentSchema, MinReadable: e.MinReadable, Envelope: e.Envelope, Owner: e.Owner})
			continue
		}
		for _, p := range paths {
			st := FileStatus{Name: e.Name, Path: p, OnDisk: -1,
				Expected: e.CurrentSchema, MinReadable: e.MinReadable, Envelope: e.Envelope, Owner: e.Owner}
			raw, err := os.ReadFile(p)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				st.Outcome = OutcomeMissing
			case err != nil:
				st.Outcome, st.Err = OutcomeCorrupt, err.Error()
			default:
				outcome, ver, ierr := inspect(e, p, raw)
				st.Outcome, st.OnDisk = outcome, ver
				if ierr != nil {
					st.Err = ierr.Error()
				}
			}
			out = append(out, st)
		}
	}
	return out
}

// ───────────────────────── 内部：读 + 判定 ─────────────────────────

func readPath(e Entry, path string, cfg Config) ([]byte, Outcome, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, OutcomeMissing, nil
		}
		cfg.logf("⚠️ [compat] 读状态文件失败 %s: %v", path, err)
		return nil, OutcomeCorrupt, fmt.Errorf("compat: 读 %s 失败: %w", path, err)
	}
	return reconcile(e, path, raw, cfg)
}

// inspect —— 纯判定（不落盘、不日志）：文件内容对应的 schema 与结论。
func inspect(e Entry, path string, raw []byte) (Outcome, int, error) {
	if e.Inband() {
		obj, err := decodeObject(raw)
		if err != nil {
			return OutcomeCorrupt, -1, err
		}
		ver, has := schemaOf(obj, e.SchemaKey())
		if !has {
			return OutcomeMigrated, -1, nil // 旧版：需要迁移
		}
		switch {
		case ver > e.CurrentSchema:
			return OutcomeFuture, ver, nil
		case ver == e.CurrentSchema:
			return OutcomeCurrent, ver, nil
		default:
			return OutcomeMigrated, ver, nil
		}
	}
	if _, err := validateJSON(raw); err != nil {
		return OutcomeCorrupt, -1, err
	}
	sc, err := readSidecar(e, path)
	if err != nil {
		return OutcomeCorrupt, -1, err
	}
	switch {
	case sc > e.CurrentSchema:
		return OutcomeFuture, sc, nil
	case sc == e.CurrentSchema:
		return OutcomeCurrent, sc, nil
	default:
		return OutcomeMigrated, sc, nil
	}
}

// reconcile —— 判定 + 需要时执行一次性迁移。
func reconcile(e Entry, path string, raw []byte, cfg Config) ([]byte, Outcome, error) {
	outcome, ver, err := inspect(e, path, raw)
	switch outcome {
	case OutcomeCorrupt:
		cfg.logf("❌ [compat] 状态文件不可解析 %s: %v（保持原文件不动）", path, err)
		return raw, OutcomeCorrupt, err
	case OutcomeCurrent:
		return raw, OutcomeCurrent, nil
	case OutcomeFuture:
		cfg.logf("⚠️ [compat] %s 的 schema=%d 高于本机认知 %d —— 不猜测、不迁移、不回写；本次只读降级，请升级本机二进制",
			path, ver, e.CurrentSchema)
		return raw, OutcomeFuture, nil
	}
	return migrateOne(e, path, raw, cfg)
}

// migrateOne —— 备份 → 迁移 → 写回 → 回读校验（失败回滚）。
func migrateOne(e Entry, path string, raw []byte, cfg Config) ([]byte, Outcome, error) {
	// ① 备份（同名备份已存在则不覆盖——迁移可重入而不吃掉更早的备份）
	bak := fmt.Sprintf("%s.bak-%s", path, cfg.now().UTC().Format("20060102T150405"))
	if _, err := os.Stat(bak); errors.Is(err, fs.ErrNotExist) {
		if werr := writeFileAtomicNew(bak, raw); werr != nil {
			cfg.logf("❌ [compat] 迁移前备份失败 %s: %v —— 不迁移，保持原文件", bak, werr)
			return raw, OutcomeCorrupt, fmt.Errorf("compat: 备份 %s 失败: %w", path, werr)
		}
	}

	// ② 迁移（纯变换）
	fn, ok := migrations[e.MigrateFunc]
	if !ok {
		return raw, OutcomeCorrupt, fmt.Errorf("compat: 迁移函数 %q 未注册", e.MigrateFunc)
	}
	out, err := fn(e, raw)
	if err != nil {
		cfg.logf("❌ [compat] 迁移 %s 失败: %v —— 保持原文件", path, err)
		return raw, OutcomeCorrupt, err
	}

	// ③ 写回
	if err := writeFileAtomic(path, out); err != nil {
		cfg.logf("❌ [compat] 迁移写回失败 %s: %v —— 尝试回滚", path, err)
		if rerr := writeFileAtomic(path, raw); rerr != nil {
			cfg.logf("❌ [compat] 回滚也失败 %s: %v（备份仍在 %s，请人工恢复）", path, rerr, bak)
		}
		return raw, OutcomeCorrupt, err
	}

	// ④ 回读校验（inband 才比对顶层键；sidecar 校验版本文件已生效）
	if e.Inband() {
		if verr := verifyInband(e, path, raw); verr != nil {
			if rerr := writeFileAtomic(path, raw); rerr != nil {
				cfg.logf("❌ [compat] 校验失败且回滚失败 %s: %v（备份 %s）", path, rerr, bak)
			} else {
				cfg.logf("❌ [compat] 迁移后回读校验失败 %s: %v —— 已回滚为原文件", path, verr)
			}
			return raw, OutcomeCorrupt, verr
		}
	} else if err := writeSidecar(e, path, cfg); err != nil {
		cfg.logf("❌ [compat] 写旁路版本文件失败 %s: %v", path, err)
		return raw, OutcomeCorrupt, err
	}

	cfg.logf("✅ [compat] 状态文件已迁移到 schema %d: %s（原文件备份 %s）", e.CurrentSchema, path, bak)
	return out, OutcomeMigrated, nil
}

// verifyInband —— 回读校验：schema 必须到位，且**旧顶层键一个都不能少**。
//
// 这条是「不静默丢用户状态」的机械保证：迁移函数写错、Marshal 丢字段、并发写坏——
// 都会在这里被抓住并回滚，而不是等到用户发现配置没了。
func verifyInband(e Entry, path string, before []byte) error {
	after, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: 回读失败 %v", ErrVerifyFailed, err)
	}
	ao, err := decodeObject(after)
	if err != nil {
		return fmt.Errorf("%w: 迁移后不是合法对象: %v", ErrVerifyFailed, err)
	}
	ver, ok := schemaOf(ao, e.SchemaKey())
	if !ok || ver != e.CurrentSchema {
		return fmt.Errorf("%w: 迁移后 %s=%d（期望 %d）", ErrVerifyFailed, e.SchemaKey(), ver, e.CurrentSchema)
	}
	bo, err := decodeObject(before)
	if err != nil {
		return fmt.Errorf("%w: 迁移前不是合法对象: %v", ErrVerifyFailed, err)
	}
	var lost []string
	for k := range bo {
		if _, still := ao[k]; !still {
			lost = append(lost, k)
		}
	}
	if len(lost) > 0 {
		sort.Strings(lost)
		return fmt.Errorf("%w: 顶层键丢失 %q", ErrVerifyFailed, lost)
	}
	return nil
}

// ───────────────────────── 内部：JSON 与文件小工具 ─────────────────────────

func decodeObject(raw []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if obj == nil {
		return nil, ErrNotAnObject
	}
	return obj, nil
}

func validateJSON(raw []byte) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, ErrCorrupt
	}
	return json.RawMessage(raw), nil
}

// schemaOf —— 取顶层 schema 字段的整数值；字段不存在/非整数 ⇒ has=false。
func schemaOf(obj map[string]json.RawMessage, field string) (int, bool) {
	rv, ok := obj[field]
	if !ok {
		return -1, false
	}
	var n int
	if err := json.Unmarshal(rv, &n); err != nil {
		return -1, false
	}
	return n, true
}

func onDiskSchema(e Entry, path string) (int, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return -1, false, err
	}
	obj, err := decodeObject(raw)
	if err != nil {
		return -1, false, err
	}
	v, has := schemaOf(obj, e.SchemaKey())
	return v, has, nil
}

// marshalOrdered —— 稳定序列化：schema 字段**置首**，其余键按字典序。
//
// 为什么固定顺序：同一份状态两次序列化必须逐字节相同（内容寻址与幂等校验都依赖它）；
// Go 的 map 序列化本身已排序，这里只额外把版本字段提到最前，方便人读与 diff。
func marshalOrdered(schemaKey string, obj map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		if k != schemaKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{\n")
	writeKV := func(k string, i int) error {
		v, err := json.Marshal(obj[k])
		if err != nil {
			return err
		}
		b.WriteString("  ")
		kb, err := json.Marshal(k)
		if err != nil {
			return err
		}
		b.Write(kb)
		b.WriteString(": ")
		b.Write(v)
		if i >= 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
		return nil
	}
	if _, has := obj[schemaKey]; has {
		idx := len(keys)
		if idx == 0 {
			idx = -1 // 只有 schema 一个键时不能留尾逗号
		}
		if err := writeKV(schemaKey, idx); err != nil {
			return nil, err
		}
	}
	for i, k := range keys {
		last := i == len(keys)-1
		idx := i
		if last {
			idx = -1
		}
		if err := writeKV(k, idx); err != nil {
			return nil, err
		}
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// writeFileAtomicNew —— 只写「本来不存在」的文件（备份专用：绝不覆盖已有备份）。
func writeFileAtomicNew(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	// O_EXCL 语义：rename 会覆盖，所以先用 O_CREATE|O_EXCL 占位，再 rename 覆盖自己的占位。
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		_ = os.Remove(tmp)
		if errors.Is(err, fs.ErrExist) {
			return nil // 备份已存在 ⇒ 保留更早那份（幂等、可重入）
		}
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}

type sidecarDoc struct {
	Schema    int    `json:"schema"`
	UpdatedAt string `json:"updated_at"`
	ManagedBy string `json:"managed_by"`
}

func readSidecar(e Entry, path string) (int, error) {
	b, err := os.ReadFile(e.SidecarPath(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return -1, nil
		}
		return -1, err
	}
	var d sidecarDoc
	if err := json.Unmarshal(b, &d); err != nil {
		return -1, fmt.Errorf("compat: 旁路版本文件损坏 %s: %w", e.SidecarPath(path), err)
	}
	return d.Schema, nil
}

func writeSidecar(e Entry, path string, cfg Config) error {
	doc := sidecarDoc{
		Schema:    e.CurrentSchema,
		UpdatedAt: cfg.now().UTC().Format(time.RFC3339),
		ManagedBy: "zerg-compat",
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(e.SidecarPath(path), append(b, '\n'))
}

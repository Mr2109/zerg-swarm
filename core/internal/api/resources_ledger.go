package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// resources_ledger.go —— 资源管理器观测面：每机账本 + 逐模型"跑得动吗"估算
// （《设计-资源管理器》批 4；规格 §三.4 观测面 / §四 形状 / §八 Q8）。
//
// 路由（注册在 cmd/zerg-core/main.go，与既有 /api/* 同一套 AuthMiddleware 鉴权）：
//
//	GET /api/resources/ledger → {"machines":[...],"count":N,"models_dir":"...","registered_models":M}
//
// 每台机器一台账本（内存/显存/驻留明细/未托管项）+ 逐登记模型在该机上的估算。
//
// 【严格只读——本接口硬约束，改动时不要破坏】
//   - 只做 os.ReadDir / os.Open（读登记库 JSON、读 .gguf 元数据）；**绝不**调用
//     MkdirAll / WriteFile / Rename / Remove（与 /api/models/registry 同一纪律）。
//   - 拿不到的字段**整键不出现**（omitempty/指针），绝不造值、绝不 500：
//     · 该机器没有心跳快照 → 该机器整条不出现；
//     · 没有驻留明细 → resident 整键不出现；没有未托管项 → unmanaged 整键不出现；
//     · 显存拿不到 → vram_known=false 且 vram_total/used/free **整键不出现**（不出现假值）；
//     · 内存拿不到 → mem_known=false 且 mem_total/available **整键不出现**；
//     · 登记库读不动 / 坏记录 → 估算项跳过（registered_models 整键不出现），不改整体状态码。
//
// 【诚实原则（§3.2）】估算输入全部来自现有数据：登记库 files[].size（权重真值）+ context_window
// （上下文上限）+ 批 3 的 probe_meta GGUF 真值（n_layers / n_kv_heads / head_dim）。
//   - KV dtype 与引擎开销是**引擎运行参数**，GGUF 不记录 → 由 Handler 字段
//     （KvCacheBytesPerElem / EngineOverheadGb）提供；未提供则回退常量并标 estimated=true
//     （宁可标"估的"，不许冒充实测——§八 Q4）。
//   - 关键输入缺失（权重字节/上下文/层数）→ 估算 fail-closed 判 no_fit，basis 写明原因。
//   - 登记库里没有的模型不在此出现（"缺就缺"）。

// ResourceMachineView 是一台机器的账本对外视图（§3.4 观测面）。
//
// 取值纪律：resident / unmanaged / vram_* / mem_* 用 omitempty 或指针——
// 拿不到就整键不出现；mem_known / vram_known 恒出现，因为"知不知道"本身就是信息。
type ResourceMachineView struct {
	Machine      string   `json:"machine"`
	MemKnown     bool     `json:"mem_known"`
	MemTotalGb   *float64 `json:"mem_total_gb,omitempty"`
	MemAvailGb   *float64 `json:"mem_available_gb,omitempty"`
	VramKnown    bool     `json:"vram_known"`
	VramTotalGb  *float64 `json:"vram_total_gb,omitempty"`
	VramUsedGb   *float64 `json:"vram_used_gb,omitempty"`
	VramFreeGb   *float64 `json:"vram_free_gb,omitempty"`
	GpuPct       *float64 `json:"gpu_pct,omitempty"`
	BackendState string   `json:"backend_state,omitempty"`

	// 驻留明细（state / last_used_ago_s / req_count / managed / pinned / pin_ttl_s /
	// weights_bytes / ctx_window）。pin_ttl_s 即 pin 的**剩余 TTL 秒**（= §八 Q5 的 pin 剩余时间）。
	Resident []resources.ResidentEntry `json:"resident,omitempty"`
	// 未托管但占着资源的进程/端口（覆盖实测 E2；§八 Q6：只标注，不接管、不杀）。
	Unmanaged []resources.UnmanagedProcess `json:"unmanaged,omitempty"`
	// 逐登记模型在本机上的"跑得动吗"估算（§3.2；含 verdict / estimated / basis）。
	Fit []resources.FitEstimate `json:"fit,omitempty"`
}

// fitInput 是一个登记模型的估算输入（与机器无关；读取成本只付一次，各机器复用）。
type fitInput struct {
	Name         string
	Digest       string
	WeightsBytes int64
	Ctx          int
	ArchFamily   string
	Meta         *modelreg.GGUFMeta // 读到本地 GGUF 元数据则非 nil；拿不到为 nil
}

// localModelFile 是从 fleet 配置解析出的"某权重文件名在本地对应的路径 + 架构族"。
type localModelFile struct {
	Path string
	Arch string
}

// ResourceLedgerHandler — GET /api/resources/ledger（需鉴权、严格只读）
func (h *Handlers) ResourceLedgerHandler(w http.ResponseWriter, r *http.Request) {
	inputs, regErr := h.registeredFitInputs()
	machines := h.buildResourceMachines(inputs)
	resp := map[string]interface{}{
		"machines":   machines,
		"count":      len(machines),
		"models_dir": h.modelsRoot(),
	}
	// 登记库读不动 → registered_models 整键不出现（缺就缺），但账本本体照常返回（不 500）
	if regErr == "" {
		resp["registered_models"] = len(inputs)
	}
	writeJSON(w, http.StatusOK, resp)
}

// buildResourceMachines 把 store 里的每台机器快照翻成对外账本视图（机器名排序，响应可复现）。
func (h *Handlers) buildResourceMachines(inputs []fitInput) []ResourceMachineView {
	out := []ResourceMachineView{}
	if h.Store == nil {
		return out
	}
	snaps := h.Store.GetAllSnapshots()
	names := make([]string, 0, len(snaps))
	for n := range snaps {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		snap := snaps[n]
		if snap == nil {
			continue
		}
		out = append(out, h.machineView(snap, inputs))
	}
	return out
}

// machineView 单台机器 → 对外视图。
func (h *Handlers) machineView(snap *store.FleetSnapshot, inputs []fitInput) ResourceMachineView {
	mv := ResourceMachineView{Machine: snap.Machine}

	// 内存：总量 > 0 视为已知；未知则两个值键都不出现（不出现假值）
	if snap.MemTotalGb > 0 {
		mv.MemKnown = true
		total, avail := snap.MemTotalGb, snap.MemAvailableGb
		mv.MemTotalGb = &total
		mv.MemAvailGb = &avail
	}
	// 显存：只看子端自报的 vram_known（拿不到 = false，值键整键不出现），绝不用 RSS 冒充
	if snap.VramKnown {
		mv.VramKnown = true
		t, u, f := snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb
		mv.VramTotalGb, mv.VramUsedGb, mv.VramFreeGb = &t, &u, &f
	}
	if snap.GpuPct > 0 {
		gp := snap.GpuPct
		mv.GpuPct = &gp
	}
	mv.BackendState = strings.TrimSpace(snap.BackendState)
	if len(snap.Resident) > 0 {
		mv.Resident = snap.Resident
	}
	if len(snap.Unmanaged) > 0 {
		mv.Unmanaged = snap.Unmanaged
	}
	mv.Fit = h.fitEstimatesFor(snap, inputs)
	return mv
}

// fitEstimatesFor 对一台机器算逐登记模型的"跑得动吗"（输入来自登记库 + GGUF 真值）。
func (h *Handlers) fitEstimatesFor(snap *store.FleetSnapshot, inputs []fitInput) []resources.FitEstimate {
	if len(inputs) == 0 {
		return nil // 无登记模型 → fit 整键不出现
	}
	ledger := resources.MachineLedger{
		Machine:    snap.Machine,
		MemTotalGb: snap.MemTotalGb,
		MemAvailGb: snap.MemAvailableGb,
		Resident:   snap.Resident,
	}
	// 显存只在子端真拿到了（vram_known=true）时才进比较式；否则按统一内存口径只判内存
	if snap.VramKnown {
		ledger.VramTotalGb = snap.VramTotalGb
		ledger.VramUsedGb = snap.VramUsedGb
		ledger.VramFreeGb = snap.VramFreeGb
	}
	// 引擎开销：Handler 提供则用真值，否则留 0 → EstimateFit 回退常量并标 estimated
	if h.EngineOverheadGb > 0 {
		ledger.EngineOverheadGb = h.EngineOverheadGb
	}
	out := make([]resources.FitEstimate, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, modelreg.EstimateFitFromMeta(
			in.Meta, in.Name, in.Ctx, in.WeightsBytes, h.KvCacheBytesPerElem, in.ArchFamily, ledger))
	}
	return out
}

// registeredFitInputs 扫登记库，把每条记录翻成估算输入。
//
// 返回值 (inputs, errMsg)：errMsg 非空 = 登记库整体读不动（此时 inputs 为空、registered_models 不出现）。
// 单条坏记录 / 单条读不动 → 只跳过这一条（反例优先：一条坏记录不能让整个账本挂掉）。
func (h *Handlers) registeredFitInputs() ([]fitInput, string) {
	st := modelreg.NewStore(h.modelsRoot())
	rows, err := st.List()
	if err != nil {
		return nil, err.Error()
	}
	idx := h.localModelIndex()
	out := make([]fitInput, 0, len(rows))
	for _, row := range rows {
		if row.Err != "" {
			continue // 坏记录：跳过，不 500
		}
		rec, lerr := modelreg.Load(row.Path)
		if lerr != nil {
			continue
		}
		in := fitInput{
			Name:   firstNonEmpty(rec.Name, rec.ID, row.ID),
			Digest: rec.Digest,
			Ctx:    rec.ContextWindow,
		}
		var weights int64
		for _, f := range rec.Files {
			weights += f.Size
		}
		in.WeightsBytes = weights
		// 定位本机可读的权重文件（拿不到就 Meta=nil，估算按回退/fail-closed 处理）
		if p, arch := h.resolveWeightFile(rec, idx); p != "" {
			in.ArchFamily = arch
			// probe.meta.gguf.v1（批 3）真值：层数 / n_kv_heads / head_dim
			if meta, _, merr := modelreg.ProbeMetaGGUFFile(p); merr == nil {
				in.Meta = meta
				if meta.Architecture != "" {
					in.ArchFamily = meta.Architecture
				}
			}
		}
		out = append(out, in)
	}
	return out, ""
}

// localModelIndex 从 fleet 配置建"权重文件名(basename) → 本地路径 + 架构族"索引，
// 用于把登记库记录里的建材定位到本机可读的 .gguf。文件不存在也保留路径，
// 由 resolveWeightFile 再做存在性判断（本机没有的（如 X3 权重）自然缺席）。
func (h *Handlers) localModelIndex() map[string]localModelFile {
	idx := map[string]localModelFile{}
	if h.Config == nil {
		return idx
	}
	for _, cands := range h.Config.Models {
		for _, c := range cands {
			if strings.TrimSpace(c.File) == "" {
				continue
			}
			base := filepath.Base(c.File)
			if _, dup := idx[base]; dup {
				continue
			}
			arch := c.Architecture
			if arch == "" {
				arch = c.Arch
			}
			idx[base] = localModelFile{Path: c.File, Arch: arch}
		}
	}
	return idx
}

// resolveWeightFile 为一条登记记录找本机可读的 .gguf 路径（拿不到返回 ""，不编造）。
func (h *Handlers) resolveWeightFile(rec *modelreg.Record, idx map[string]localModelFile) (string, string) {
	for _, f := range rec.Files {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) {
			if fileExists(name) {
				return name, ""
			}
			continue
		}
		if e, ok := idx[filepath.Base(name)]; ok && fileExists(e.Path) {
			return e.Path, e.Arch
		}
	}
	return "", ""
}

// fileExists 报告路径是否为已存在的普通文件（只读探测，不创建任何东西）。
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// firstNonEmpty 返回第一个非空（已去空白）字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// 模型目录（modelreg）只读快照 HTTP 接口。
//
// 路由（注册在 cmd/zerg-core/main.go，与既有 /api/* 同一套 AuthMiddleware 鉴权）：
//
//	GET /api/models/registry → {"root":"...","manifests_dir":"...","count":N,"bad_records":M,"records":[...]}
//
// 动机：`zerg-model list --json` 只能在本机命令行跑；UI 要渲染"模型库"页（有哪些模型、
// 许可证商用性、能力标签、建材、校验结论）时没有入口。这个接口把 Store.List 的语义
// 通过 HTTP 只读暴露出来。
//
// 【严格只读——这是本接口的硬约束，改动时不要破坏】
//   - 只做 os.ReadDir / os.ReadFile（经 modelreg.Store.List + modelreg.Load +
//     modelreg.LoadCapabilitySnapshot），**绝不**调用 MkdirAll / CreateTemp / WriteFile / rename / os.Remove。
//   - 根目录不存在或为空 = 正常状态，返回 200 + count=0（不 404、不 500、不建目录）。
//     UI 需要能渲染空态；"没有模型"不是错误。
//   - 坏记录（JSON 解析失败等）只计入该条的 errors，不改整个请求的状态码。
//   - 能力快照兄弟文件（<version>.capabilities.json）读不到/是坏 JSON = 该条能力降级为只用
//     记录正文的断言，同样不 500、不报错到整体请求失败（与坏记录策略一致）。
//
// 枚举复用 modelreg.Store.List 的语义（跳过 .trace.json 兄弟文件、跳过点开头文件、
// 按 (id, version) 排序、坏文件如实记 Err 而不中断），再对可解析的记录用 modelreg.Load
// 取完整字段（建材明细、上下文窗口、引擎配方），并用 modelreg.LoadCapabilitySnapshot
// 读实测能力（待修补 #26：记录正文不再装 capabilities，能力与出处来自快照兄弟文件）。

// ModelRegistryCapability 是一条能力断言的对外视图（标准 §四：必须带来源）。
//
// Engines（待修补 #39，数据由 #11 加入快照）是这条断言被**证过成立**的引擎（规范化标签，
// 如 llama.cpp / vllm）。语义：同一能力在不同引擎上可真假不同，硬门槛按目标引擎取证据；
// 缺引擎维度 = 不可判定（不等于可用）。本字段回答 UI 的"这条能力是在哪个引擎上测出来的"，
// 是证据链的一环。
//
// 出现条件：快照里该能力条**确实带了非空 engines**（原音照抄，只丢空白/空串）。
// 缺字段 / 空数组 → 本键**整键不出现**（omitempty），与既有 snapshot_* 字段同一口径——
// 缺 = 未知，绝不填占位值（如 "unknown"/"n/a"），也绝不编造一个默认引擎。
type ModelRegistryCapability struct {
	Name     string   `json:"name"`
	Value    bool     `json:"value"`
	Source   string   `json:"source"`
	Evidence string   `json:"evidence,omitempty"`
	Engines  []string `json:"engines,omitempty"`
}

// ModelRegistryUnverifiable 是一条"无法判定"记录的对外视图（待修补 #28）。
//
// 语义：本轮探测因预算耗尽/超时**没探出结论**——既不是"支持"，也不是"确定不支持"。
// 与 capabilities[].value=false（端点明确表态没有）是两件事，故放独立数组，绝不混进 capabilities。
// 三个字段照抄快照原文（name/reason/evidence 一字不改），不推断、不补默认、不改写原因分类。
type ModelRegistryUnverifiable struct {
	Name     string `json:"name"`
	Reason   string `json:"reason"`             // 原因分类：budget_exhausted / timeout（照抄）
	Evidence string `json:"evidence,omitempty"` // 探测器名+版本+实际预算+原始响应摘要（照抄；与快照同形）
}

// ModelRegistryFile 是一份建材的对外视图（标准 §三：一个模型是一组建材）。
type ModelRegistryFile struct {
	Role   string `json:"role"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// ModelRegistryLicense 是一条记录的许可证块对外视图（标准 §五：读权重，不读仓库徽章）。
//
// 与顶层兼容字段 commercial 的关系：顶层 commercial 是既有 UI 已依赖的「商用性」单值
// （Store.List 的裁剪行只带这一个字段），保留不动；本块是详情层要的完整许可信息
// ——SPDX 表达式、许可名/链接、是否门控、来源 URL、以及受限许可的人工留痕。
//
// 取值纪律：一律照抄 modelreg.License 的真实值；缺值就缺（optional 字段 omitempty，
// 整块无内容时连 license 都不出现），绝不填 unset/pending/tbd 这类占位串。
type ModelRegistryLicense struct {
	SPDX       string `json:"spdx"`
	Name       string `json:"name,omitempty"`
	Link       string `json:"link,omitempty"`
	Commercial string `json:"commercial"`
	Gated      bool   `json:"gated"`
	SourceURL  string `json:"source_url,omitempty"`
	AcceptedBy string `json:"accepted_by,omitempty"`
	AcceptedAt string `json:"accepted_at,omitempty"`
	// Evidence 是许可证结论的**来源锚**（待修补 #39，数据由 #12 加入记录）：
	// probe.license.v1 写"读的是哪个键/哪个文件"，例如
	// `probe.license.v1 (gguf_key: general.license="apache-2.0")`；读不到时写
	// `probe.license.v1 (no_license_source: tried …)`。它回答 UI 的"这个许可证结论是从哪读来的"，
	// 是证据链的一环。缺（记录没有该字段 / 是空串）→ 本键**整键不出现**（omitempty），
	// 绝不填占位值——与 license 块内其余可选字段同一口径。
	Evidence string `json:"evidence,omitempty"`
}

// ModelRegistryRecord 是目录里一条记录的对外视图。
// Err 非空 = 该条读不动/不是合法 JSON：errors 计 1，其余字段留零值，不伪装成"读到了"。
type ModelRegistryRecord struct {
	ID              string                    `json:"id"`
	Version         string                    `json:"version"`
	Path            string                    `json:"path"`
	Digest          string                    `json:"digest,omitempty"`
	Name            string                    `json:"name,omitempty"`
	Commercial      string                    `json:"commercial,omitempty"`
	License         *ModelRegistryLicense     `json:"license,omitempty"`
	Capabilities    []ModelRegistryCapability `json:"capabilities"`
	Files           []ModelRegistryFile       `json:"files"`
	ContextWindow   int                       `json:"context_window"`
	EngineRecipes   []string                  `json:"engine_recipes"`
	Errors          int                       `json:"errors"`
	Warns           int                       `json:"warns"`
	DefaultEligible bool                      `json:"default_eligible"`
	Error           string                    `json:"error,omitempty"`

	// FleetID 是本记录到**路由表**（`/api/fleet/models`）的 id 规范化结果（T-41 · `R2-P1`）。
	// 快照 id 形如 `example-35b-v2-1-5-35b-q4-k-m`，路由表 id 形如 `example-35b-v2` —— 两套命名
	// 今天接不上（§二十一 第 19 条）。本字段给出**精确**映射（真源 = `contract.ModelIDMap()`，
	// 只认显式条目）。**候选多于一**（同一份权重在路由表里挂了多个名字，如 gemma-4-26B 与
	// example-26b-review）⇒ 逗号连接、按真源顺序。**未知 id ⇒ 整键不出现**（omitempty），
	// 绝不猜、绝不模糊匹配兜（设计稿 `R2-P1` 逐字）。
	FleetID string `json:"fleet_id,omitempty"`

	// ── 能力快照的出处（待修补 #26）────────────────────────────────────────
	// 能力现在分层：记录正文（标准 §四，人工/声明）为底，实测能力放在记录旁的
	// <version>.capabilities.json 快照兄弟文件里。这三个字段回答"上面这些能力是在哪探出来的"，
	// 让 UI 一眼能分辨能力真假：来源=哪个端点 / 何时探的 / 这次探了没有。
	//
	// 出现条件：快照**读到了**且至少提供了一条实测能力（len(capabilities) > 0）——出现即表示
	// 上面展示的能力里有可溯源到该端点的实测项。快照不存在 / 坏 JSON / 空快照 → 这三个字段
	// **整键不出现**（omitempty），绝不造值。
	//
	// snapshot_online_probed 用指针：false 是真实取值（这次没做在线探测），必须能如实显示，
	// 不能与"没有快照"混为一谈（缺省=没有快照，false=有快照但没在线探）。
	SnapshotEndpoint     string `json:"snapshot_endpoint,omitempty"`
	SnapshotGeneratedAt  string `json:"snapshot_generated_at,omitempty"`
	SnapshotOnlineProbed *bool  `json:"snapshot_online_probed,omitempty"`

	// ── 不可判定能力（待修补 #28）──────────────────────────────────────────
	// Unverifiable 照抄能力快照的 unverifiable[]（预算耗尽/超时 → 本轮**没探出结论**），
	// 与 capabilities[].value=false（端点明确表态"没有"）严格区分：缺 = 未知，绝不 = 没有。
	//
	// 出现条件：快照**读到了**且 unverifiable 非空 → 原样带出（name/reason/evidence 一字不改）。
	// 快照不存在 / 坏 JSON / 无该字段 / 空数组 → 本键**整键不出现**（omitempty），绝不造值、不 500。
	//
	// 与上面三个 snapshot_* 出处字段解耦：本数组**不改变**它们的既有出现条件（仍只看
	// len(capabilities) > 0）。取舍见 Handler 内注释——出处描述的是"能力在哪探出来的"，
	// 而只有 unverifiable（无实测能力）时没有可溯源的实测项可标注，故出处不出现。
	Unverifiable []ModelRegistryUnverifiable `json:"unverifiable,omitempty"`
}

// licenseBlockEmpty 判定一条记录的许可证块是否整块为空（零值比较，不引入占位串）。
// 全空 = 目录里这条记录没写任何许可细节 → 对外响应里连 license 字段都不出现（缺值就缺）。
// 注：evidence 也计入"非空"——它承载的是"这个结论从哪读来的"，是真实证据而非装饰；
// 只要它有值，块就不算空（否则会把唯一的一环证据链丢掉）。
func licenseBlockEmpty(l modelreg.License) bool {
	return l.SPDX == "" && l.Name == "" && l.Link == "" && l.Commercial == "" &&
		!l.Gated && l.SourceURL == "" && l.AcceptedBy == "" && l.AcceptedAt == "" &&
		l.Evidence == ""
}

// toRegistryCapability 把一条断言（记录正文或快照）翻成对外视图。
// source 与 evidence 一并带出（标准 §四：每句断言可追溯，source=probed 必须带探测证据）。
// engines（待修补 #39）同样照抄：只丢空白/空串、保持原顺序；全空 → nil（omitempty 因而整键不出现）。
func toRegistryCapability(c modelreg.Capability) ModelRegistryCapability {
	return ModelRegistryCapability{
		Name:     c.Name,
		Value:    c.Value,
		Source:   c.Source,
		Evidence: c.Evidence,
		Engines:  trimRegistryEngines(c.Engines),
	}
}

// trimRegistryEngines 收敛对外的引擎列表：去空白、丢空串、保持出现顺序、去重。
// 空输入 / 全是空串 → nil（omitempty → 整键不出现），绝不造空数组、绝不填占位值。
// 注意：**不**做引擎名规范化（那是写入侧 CanonicalEngine 的职责，快照里已是规范化标签）——
// 只读接口照抄即可，避免读侧擅自改写证据。
func trimRegistryEngines(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, e := range in {
		e = strings.TrimSpace(e)
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// toRegistryUnverifiable 把快照里一条"无法判定"记录翻成对外视图（待修补 #28）。
// 三个字段原文照抄（name/reason/evidence），不推断、不补默认、不改写原因分类。
func toRegistryUnverifiable(u modelreg.Unverifiable) ModelRegistryUnverifiable {
	return ModelRegistryUnverifiable{Name: u.Name, Reason: u.Reason, Evidence: u.Evidence}
}

// mapRegistryUnverifiable 按快照原顺序把整条 unverifiable 列表翻成对外视图。
// 空输入 → 返回 nil（omitempty 因而整键不出现，与"缺就缺"一致，绝不造空数组）。
func mapRegistryUnverifiable(us []modelreg.Unverifiable) []ModelRegistryUnverifiable {
	if len(us) == 0 {
		return nil
	}
	out := make([]ModelRegistryUnverifiable, 0, len(us))
	for _, u := range us {
		out = append(out, toRegistryUnverifiable(u))
	}
	return out
}

// mergeRegistryCapabilities 把记录正文的断言（recCaps）与能力快照的断言（snapCaps）合成对外视图。
//
// 规则（待修补 #26 原文）：「能力取自快照（当记录正文无该能力时）；两者都有时以快照为准」——
// 即**按 name 合并**：
//   - 同名：用快照值（快照是实测 source=probed，比正文的声明/人工可信）；
//   - 只在正文有：原样保留（不缺不丢——正文里人工标注的能力不该被快照抹掉）；
//   - 只在快照有：补进来。
//
// 顺序确定（响应可复现）：先按正文顺序输出（同名的换成快照值），再按快照顺序追加正文没有的名字。
// 两侧都空 → 返回非 nil 空切片（保持 capabilities 字段恒为 JSON []，与既有响应形状一致）。
func mergeRegistryCapabilities(recCaps, snapCaps []modelreg.Capability) []ModelRegistryCapability {
	snapByName := make(map[string]modelreg.Capability, len(snapCaps))
	for _, c := range snapCaps {
		if _, ok := snapByName[c.Name]; !ok {
			snapByName[c.Name] = c
		}
	}
	out := make([]ModelRegistryCapability, 0, len(recCaps)+len(snapCaps))
	seen := make(map[string]bool, len(recCaps)+len(snapCaps))
	for _, c := range recCaps {
		if seen[c.Name] {
			continue
		}
		if sc, ok := snapByName[c.Name]; ok {
			out = append(out, toRegistryCapability(sc)) // 同名以快照为准
		} else {
			out = append(out, toRegistryCapability(c))
		}
		seen[c.Name] = true
	}
	for _, c := range snapCaps {
		if seen[c.Name] {
			continue
		}
		out = append(out, toRegistryCapability(c))
		seen[c.Name] = true
	}
	return out
}

// modelsRoot 解析模型目录根：handlers 上显式指定的优先（测试注入），否则用标准解析
// （ZERG_MODELS_DIR 优先，缺省 ~/.zerg/models）。两者都只返回路径字符串，不建目录。
func (h *Handlers) modelsRoot() string {
	if r := strings.TrimSpace(h.ModelsDir); r != "" {
		return r
	}
	return modelreg.DefaultModelsDir()
}

// ModelRegistryHandler — GET /api/models/registry（只读模型目录快照）
func (h *Handlers) ModelRegistryHandler(w http.ResponseWriter, r *http.Request) {
	st := modelreg.NewStore(h.modelsRoot())
	// List 只读：目录不存在 = 空列表（自身已对 IsNotExist 做兜底，不创建目录）
	rows, err := st.List()
	if err != nil {
		// 真·读取失败（权限等）才 500——"目录不存在"不会走到这里
		writeErrorCode(w, http.StatusInternalServerError, "MODELS_DIR_READ_FAILED", "读取模型目录失败: "+err.Error())
		return
	}
	records := make([]ModelRegistryRecord, 0, len(rows))
	bad := 0
	// id 规范化真源（T-41 · `R2-P1`）：快照 id ⇒ 路由表 id。解不动 = 不解（缺就缺），
	// 绝不因它让整个只读接口挂掉；但不猜——没有真源就没有 fleet_id 这个键。
	idmap, _ := contract.ModelIDMap()
	for _, row := range rows {
		item := ModelRegistryRecord{
			ID:              row.ID,
			Version:         row.Version,
			Path:            row.Path,
			Digest:          row.Digest,
			Name:            row.Name,
			Commercial:      row.Commercial,
			Errors:          row.Errors,
			Warns:           row.Warns,
			DefaultEligible: row.DefaultEligible,
			Capabilities:    []ModelRegistryCapability{},
			Files:           []ModelRegistryFile{},
			EngineRecipes:   []string{},
		}
		if idmap != nil {
			if fleets, ok := idmap.FleetIDForRegistry(row.ID); ok {
				item.FleetID = strings.Join(fleets, ",")
			}
		}
		// 坏记录：List 已如实记 Err——计入 errors 并在 records 里保留这一条，
		// 整个请求仍是 200（反例优先：目录里混进坏文件不能让整个模型库页挂掉）。
		if row.Err != "" {
			bad++
			item.Errors = 1
			item.Error = row.Err
			records = append(records, item)
			continue
		}
		// 可解析的记录——再读一次取完整字段（只读，路径来自 List，不自己拼目录）
		rec, lerr := modelreg.Load(row.Path)
		if lerr != nil {
			bad++
			item.Errors = 1
			item.Error = lerr.Error()
			records = append(records, item)
			continue
		}
		// 能力分层（待修补 #26）：记录正文（人工/声明）为底，实测能力在记录旁的
		// <version>.capabilities.json 快照兄弟文件里。合并规则：按 name，同名以快照为准
		// （它是实测，比声明可信），快照没有、正文有的名字原样保留（不缺不丢），
		// 快照有、正文没有的补进来。
		//
		// 严格只读 + 容忍坏文件：LoadCapabilitySnapshot 只读该兄弟文件；读不到（不存在）
		// 或不是合法 JSON 一律降级——该条只用正文能力，不 500、不报错到整体请求失败，
		// 与坏记录策略一致。快照里没有的能力**绝不凭空造**（缺就缺）。
		var snapCaps []modelreg.Capability
		var snapUnverifiable []modelreg.Unverifiable
		if snap, serr := modelreg.LoadCapabilitySnapshot(modelreg.CapabilitySnapshotPath(row.Path)); serr == nil {
			snapCaps = snap.Capabilities
			// 快照确有一条实测能力可用时才暴露出处：出现即表示上面有可溯源到该端点的实测项。
			// 三个字段照抄快照真实值（缺就缺，omitempty），绝不造值。
			if len(snapCaps) > 0 {
				item.SnapshotEndpoint = snap.Endpoint
				item.SnapshotGeneratedAt = snap.GeneratedAt
				online := snap.OnlineProbed
				item.SnapshotOnlineProbed = &online
			}
			// 不可判定能力（待修补 #28）：照抄快照的 unverifiable[]，与 capabilities 解耦。
			snapUnverifiable = snap.Unverifiable
		}
		item.Capabilities = mergeRegistryCapabilities(rec.Capabilities, snapCaps)
		// unverifiable 只在快照真给了内容时出现（空/缺/坏快照 → 整键不出现，见 omitempty）。
		// 取舍：不改动 snapshot_* 的既有出现条件（仍只看 len(capabilities)>0）——那三个字段
		// 标注的是"上面这些能力在哪探出来的"，而纯 unverifiable 场景没有实测能力可溯源；
		// 若日后 UI 需要"这段不可判定结论出自哪个端点"，再让出处随 unverifiable 出现即可。
		item.Unverifiable = mapRegistryUnverifiable(snapUnverifiable)
		for _, fl := range rec.Files {
			item.Files = append(item.Files, ModelRegistryFile{
				Role: fl.Role, Name: fl.Name, SHA256: fl.SHA256, Size: fl.Size,
			})
		}
		item.ContextWindow = rec.ContextWindow
		// 许可证块：Store.List 的裁剪行只带 Commercial，完整许可信息必须来自 Load 回来的
		// rec.License（真实值照抄，不做任何默认/推断）。整块无内容 → 不出现 license 字段。
		if lic := rec.License; !licenseBlockEmpty(lic) {
			item.License = &ModelRegistryLicense{
				SPDX:       lic.SPDX,
				Name:       lic.Name,
				Link:       lic.Link,
				Commercial: lic.Commercial,
				Gated:      lic.Gated,
				SourceURL:  lic.SourceURL,
				AcceptedBy: lic.AcceptedBy,
				AcceptedAt: lic.AcceptedAt,
				Evidence:   lic.Evidence,
			}
		}
		for name := range rec.EngineRecipes {
			item.EngineRecipes = append(item.EngineRecipes, name)
		}
		sort.Strings(item.EngineRecipes) // map 遍历无序——排序保证响应可复现
		records = append(records, item)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"root":          st.Root,
		"manifests_dir": st.ManifestsDir(),
		"count":         len(records),
		"bad_records":   bad,
		"records":       records,
	})
}

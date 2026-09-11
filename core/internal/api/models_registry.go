package api

import (
	"net/http"
	"sort"
	"strings"

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
//   - 只做 os.ReadDir / os.ReadFile（经 modelreg.Store.List + modelreg.Load），
//     **绝不**调用 MkdirAll / CreateTemp / WriteFile / rename / os.Remove。
//   - 根目录不存在或为空 = 正常状态，返回 200 + count=0（不 404、不 500、不建目录）。
//     UI 需要能渲染空态；"没有模型"不是错误。
//   - 坏记录（JSON 解析失败等）只计入该条的 errors，不改整个请求的状态码。
//
// 枚举复用 modelreg.Store.List 的语义（跳过 .trace.json 兄弟文件、跳过点开头文件、
// 按 (id, version) 排序、坏文件如实记 Err 而不中断），再对可解析的记录用 modelreg.Load
// 取完整字段（能力断言的 value/source、建材明细、上下文窗口、引擎配方）。

// ModelRegistryCapability 是一条能力断言的对外视图（标准 §四：必须带来源）。
type ModelRegistryCapability struct {
	Name     string `json:"name"`
	Value    bool   `json:"value"`
	Source   string `json:"source"`
	Evidence string `json:"evidence,omitempty"`
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
}

// licenseBlockEmpty 判定一条记录的许可证块是否整块为空（零值比较，不引入占位串）。
// 全空 = 目录里这条记录没写任何许可细节 → 对外响应里连 license 字段都不出现（缺值就缺）。
func licenseBlockEmpty(l modelreg.License) bool {
	return l.SPDX == "" && l.Name == "" && l.Link == "" && l.Commercial == "" &&
		!l.Gated && l.SourceURL == "" && l.AcceptedBy == "" && l.AcceptedAt == ""
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
		for _, c := range rec.Capabilities {
			item.Capabilities = append(item.Capabilities, ModelRegistryCapability{
				Name: c.Name, Value: c.Value, Source: c.Source,
				Evidence: c.Evidence,
			})
		}
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

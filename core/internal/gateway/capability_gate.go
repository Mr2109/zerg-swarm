package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// ── 能力硬门槛：按目标引擎判定（待修补 #11）────────────────────────────
//
// 背景（评审 example-35b-v2 观点 1）：清单/快照声明 `vision: true`（是在 llama.cpp 上探得的），
// 但路由把图发给 vLLM（其配方未挂 mmproj）→ 端点报错 → 用户只看到"这个模型不行"。
// 这条**静默失败**直接违反四条底线之一「绝不静默降级」。
//
// 根因：能力断言缺引擎维度，硬门槛只能按「模型 + 能力」判 → 被引擎层打穿。
//
// 本文件的规则（宁可保守，绝不误放）：
//   - 放行的唯一条件：**目标引擎**确实被证过支持该必需能力
//     （该能力条 engines[] 含目标引擎）——判定在 modelreg.EvaluateCapabilityForEngine；
//   - 目标引擎无该能力证据 / 能力条无引擎维度（旧快照）/ 快照缺失 / 目标引擎未知
//     → 一律 unverifiable + fail-closed（不放行）+ 留痕（哪个引擎缺哪条能力证据）；
//   - 绝不读记录正文的能力、绝不把全局 capability 放行到未验证引擎。
//
// 触发点（必要性升档，硬门槛）：请求体里出现结构化图像部件 → 必需 vision。
// 质量升档 + 75% 门槛是备选设计（本期不实现），此处不做。

// CapabilitySnapshotSource 提供某模型当前的能力快照（只读）。
//
// 生产实现读模型登记库（`<version>.capabilities.json` 兄弟文件）；测试注入假实现，
// 绝不写 ~/.zerg（用 t.TempDir）。找不到记录返回 (nil, nil)——"没有快照"是正常态，
// 由门槛按"不可判定"处理，不是错误。
type CapabilitySnapshotSource interface {
	CapabilitySnapshot(model string) (*modelreg.CapabilitySnapshotArtifact, error)
}

// fileCapabilitySource 是生产实现：从模型目录根按 id 找到记录，再读其能力快照兄弟文件。
type fileCapabilitySource struct{ root string }

// newFileCapabilitySource 构造只读文件来源（root 为空时由调用方先解析默认根）。
func newFileCapabilitySource(root string) *fileCapabilitySource {
	return &fileCapabilitySource{root: root}
}

// CapabilitySnapshot 只读地取某模型的能力快照。
//   - 模型目录里没有该 id 的记录 → (nil, nil)（没有快照 = 不能判定）；
//   - 记录坏 / 快照坏 / 读失败 → (nil, err)（由门槛按 fail-closed + 留痕处理）。
//
// 同 id 多版本时取 List() 的第一条（List 已按 (id, version) 排序，确定）。
func (f *fileCapabilitySource) CapabilitySnapshot(model string) (*modelreg.CapabilitySnapshotArtifact, error) {
	st := modelreg.NewStore(f.root)
	rows, err := st.List()
	if err != nil {
		return nil, fmt.Errorf("list model registry: %w", err)
	}
	for _, row := range rows {
		if row.ID != model {
			continue
		}
		if row.Err != "" {
			return nil, fmt.Errorf("record %s/%s unreadable: %s", row.ID, row.Version, row.Err)
		}
		snap, serr := modelreg.LoadCapabilitySnapshot(modelreg.CapabilitySnapshotPath(row.Path))
		if serr != nil {
			return nil, fmt.Errorf("load capability snapshot for %s: %w", model, serr)
		}
		return snap, nil
	}
	return nil, nil
}

// CapabilityGateError 是硬门槛拒绝（fail-closed）时返回的错误。
//
// 它明确写出「哪个引擎缺哪条能力证据」，供上层原样转给用户——这是"明说缺什么能力"，
// 不是静默降级。
type CapabilityGateError struct {
	Model      string
	Engine     string
	Capability string
	Reason     string
	Trace      string
}

func (e *CapabilityGateError) Error() string {
	return fmt.Sprintf("capability gate rejected: model %q on engine %q lacks proven capability %q (%s) — %s",
		e.Model, e.Engine, e.Capability, e.Reason, e.Trace)
}

// gateRoute 按 host 解析目标引擎（fleet backend）并判必需能力。
// 不合格返回 *CapabilityGateError（fail-closed，写明哪个引擎缺哪条能力证据）；
// 合格或 required 为空返回 nil。
func (g *Gateway) gateRoute(model, host string, required []string) error {
	if len(required) == 0 {
		return nil
	}
	engine := g.engineForModelHost(model, host)
	d := g.gateRequiredCapabilities(model, engine, required)
	if d.Allowed {
		return nil
	}
	return &CapabilityGateError{
		Model:      model,
		Engine:     d.Engine,
		Capability: d.Name,
		Reason:     d.Reason,
		Trace:      d.Trace,
	}
}

// modelsRoot 解析模型目录根：Gateway 上显式指定的优先（测试注入），否则用标准解析
// （ZERG_MODELS_DIR 优先，缺省 ~/.zerg/models）。只返回路径，不建目录。
func (g *Gateway) modelsRoot() string {
	if r := strings.TrimSpace(g.modelsRootPath); r != "" {
		return r
	}
	return modelreg.DefaultModelsDir()
}

// capabilitySnapshotFor 取目标模型的能力快照（只读）。未注入来源时用文件来源。
func (g *Gateway) capabilitySnapshotFor(model string) (*modelreg.CapabilitySnapshotArtifact, error) {
	src := g.capSource
	if src == nil {
		src = newFileCapabilitySource(g.modelsRoot())
	}
	return src.CapabilitySnapshot(model)
}

// gateRequiredCapabilities 按**目标引擎**逐条判定必需能力。
//
// 只要有一条不能判定 / 确定不支持 → Allowed=false（fail-closed），
// Reason/Trace 写清「哪个引擎缺哪条能力证据」。required 为空时不判定（直接放行）。
func (g *Gateway) gateRequiredCapabilities(model, engine string, required []string) modelreg.CapabilityDecision {
	if len(required) == 0 {
		return modelreg.CapabilityDecision{
			Allowed: true, Engine: modelreg.CanonicalEngine(engine), Reason: modelreg.CapReasonAllowed,
			Trace: "capability gate: no required capability",
		}
	}
	snap, err := g.capabilitySnapshotFor(model)
	if err != nil {
		d := modelreg.CapabilityDecision{
			Allowed: false, Engine: modelreg.CanonicalEngine(engine), Reason: modelreg.CapReasonNoSnapshot,
			Trace: fmt.Sprintf("capability gate: reading capability snapshot for model %q failed: %v — fail-closed", model, err),
		}
		log.Printf("🚧 capability gate BLOCK model=%s engine=%s reason=%s | %s", model, d.Engine, d.Reason, d.Trace)
		return d
	}
	for _, capName := range required {
		d := modelreg.EvaluateCapabilityForEngine(snap, engine, capName)
		if !d.Allowed {
			log.Printf("🚧 capability gate BLOCK model=%s engine=%s capability=%s reason=%s | %s",
				model, d.Engine, capName, d.Reason, d.Trace)
			return d
		}
	}
	return modelreg.CapabilityDecision{
		Allowed: true, Name: strings.Join(required, ","), Engine: modelreg.CanonicalEngine(engine),
		Reason: modelreg.CapReasonAllowed,
		Trace:  fmt.Sprintf("capability gate: engine %q proven for required capabilities %v", modelreg.CanonicalEngine(engine), required),
	}
}

// engineForModelHost 找 (model, host) 候选中该 host 的引擎（fleet backend，已规范化）。
// 找不到返回 ""（缺 = 未知，由门槛按"目标引擎未知"fail-closed）。
func (g *Gateway) engineForModelHost(model, host string) string {
	if g.config == nil {
		return ""
	}
	for _, c := range g.config.Models[model] {
		if c.Host == host {
			return modelreg.CanonicalEngine(c.Backend)
		}
	}
	return ""
}

// RequiredCapabilitiesFromRequest 从请求体推断**硬门槛**必需能力（必要性升档）。
//
// 目前只认结构化信号：消息 content 里出现图像部件
// （OpenAI chat: `{"type":"image_url",...}`；Responses: `{"type":"input_image",...}`；
// 或 image_url 的值以 `data:image` 开头）→ 返回 ["vision"]。请求里没有图像 → 无必需能力。
//
// 只做"不升就做不了"的判定；不碰质量升档（备选设计，本期不实现）。解析不了 → 无必需能力
// （由既有参数校验去拒绝坏 body，本函数不越权）。
func RequiredCapabilitiesFromRequest(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil
	}
	if hasImagePart(root) {
		return []string{"vision"}
	}
	return nil
}

// hasImagePart 递归扫描请求体，判断是否存在结构化图像部件。
// 只认明确的图像标记，不做"字符串里含 image 就算"的模糊匹配（会误升级）。
func hasImagePart(v any) bool {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if hasImagePart(e) {
				return true
			}
		}
	case map[string]any:
		if typ, ok := t["type"].(string); ok {
			switch strings.ToLower(typ) {
			case "image_url", "input_image", "image":
				return true
			}
		}
		if iu, ok := t["image_url"]; ok && iu != nil {
			switch val := iu.(type) {
			case string:
				if val != "" {
					return true
				}
			case map[string]any:
				if u, ok := val["url"].(string); ok && u != "" {
					return true
				}
			default:
				return true
			}
		}
		for _, e := range t {
			if hasImagePart(e) {
				return true
			}
		}
	}
	return false
}

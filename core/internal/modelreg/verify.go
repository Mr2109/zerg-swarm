package modelreg

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Finding 是一条校验结论。Level 为 "error"（违标）或 "warn"（建议）。
type Finding struct {
	Level  string `json:"level"`
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

// Verify 按《标准-模型接入与目录贡献》§三–§六 校一条记录。
// strict=true 时 warn 也算不通过（标准 §十三.1：verify 不过直接拒）。
func Verify(r *Record, strict bool) []Finding {
	var f []Finding
	errf := func(field, detail string) { f = append(f, Finding{"error", field, detail}) }
	warnf := func(field, detail string) { f = append(f, Finding{"warn", field, detail}) }

	// §三 必填六项
	if r.Schema == "" {
		errf("schema", "必填：schema（应为 "+SchemaV1+"）")
	} else if r.Schema != SchemaV1 {
		errf("schema", "未知 schema："+r.Schema+"（本标准只认 "+SchemaV1+"）")
	}
	if strings.TrimSpace(r.ID) == "" {
		errf("id", "必填：id")
	} else if strings.ToLower(r.ID) != r.ID || strings.ContainsAny(r.ID, " \t") {
		errf("id", "id 必须为小写且不含空格："+r.ID)
	}
	if strings.TrimSpace(r.Digest) == "" {
		errf("digest", "必填：digest（模型身份 = 内容摘要）")
	} else if !strings.HasPrefix(r.Digest, "sha256:") {
		errf("digest", "digest 必须是 sha256: 前缀："+r.Digest)
	}
	if len(r.Files) == 0 {
		errf("files", "必填：files[]（一个模型是一组建材，不是单个文件）")
	}
	for i, fl := range r.Files {
		if fl.Role == "" {
			errf(fmt.Sprintf("files[%d].role", i), "role 必填（如 weights / mmproj / tokenizer）")
		}
		if fl.SHA256 == "" {
			errf(fmt.Sprintf("files[%d].sha256", i), "sha256 必填："+fl.Name)
		}
	}
	if strings.TrimSpace(r.License.SPDX) == "" {
		errf("license.spdx", "必填：license.spdx（优先 SPDX 表达式；非标准写 other + license_name + license_link）")
	}
	if r.License.SPDX == "other" && (r.License.Name == "" || r.License.Link == "") {
		errf("license", "spdx=other 时必须同时给 license_name 与 license_link（标准 §五）")
	}
	if r.License.Commercial == "" {
		errf("license.commercial", "必填：yes / no / revenue_gated / unknown（绝不默认 yes）")
	} else if !CommercialStates[r.License.Commercial] {
		errf("license.commercial", "取值越界："+r.License.Commercial)
	}
	// §五 留痕：非 yes 必须写接受人/时间
	if r.License.Commercial != "yes" && (r.License.AcceptedBy == "" || r.License.AcceptedAt == "") {
		errf("license", "非商用("+r.License.Commercial+")必须留痕：accepted_by / accepted_at 均必填")
	}

	// §四 能力标签与证据规则
	for i, c := range r.Capabilities {
		where := fmt.Sprintf("capabilities[%d]", i)
		if !CapabilityNames[c.Name] {
			errf(where+".name", "取值表外的能力标签："+c.Name)
		}
		if !EvidenceSources[c.Source] {
			errf(where+".source", "来源必须是 probed / declared / manual，当前："+c.Source)
		}
		if c.Source == "probed" && strings.TrimSpace(c.Evidence) == "" {
			errf(where+".evidence", "source=probed 必须带 evidence（探测器名+版本，可复现）")
		}
		if c.Source == "declared" && strings.TrimSpace(c.Evidence) == "" {
			warnf(where+".evidence", "source=declared 建议带来源锚（模型卡小节 / GGUF 键名）")
		}
	}

	// §六 引擎配方
	for name, rec := range r.EngineRecipes {
		where := "engine_recipes." + name
		if rec.ChatTemplate != "" && !ValidChatTemplate(rec.ChatTemplate) {
			errf(where+".chat_template", "只能 from_gguf / from_tokenizer / inline:<模板>，当前："+rec.ChatTemplate)
		}
		for k := range rec.ExtraEnv {
			if !strings.HasPrefix(k, "ZERG_") {
				errf(where+".extra_env."+k, "私有开关必须加 ZERG_ 前缀（引擎原生参数放 args）")
			}
		}
	}

	// §二 底线 4：不许编造——来源可追溯（离线原则：不查 URL 可达性，只要求有）
	if r.SourceURL == "" && r.License.SourceURL == "" {
		warnf("source_url", "无任何来源链接（离线校验不查可达性，但标准 §二要求可追溯）")
		if !strings.Contains(r.Notes, "unverified_source") {
			warnf("notes", "建议注明 unverified_source")
		}
	}
	return f
}

// Load 读一条记录。**未知字段必须被忽略而不报错**（标准 §十 向后兼容）。
func Load(path string) (*Record, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil { // encoding/json 默认忽略未知字段
		return nil, fmt.Errorf("记录不是合法 JSON：%w", err)
	}
	return &r, nil
}

// CountErrors 统计 error 级结论数。
func CountErrors(fs []Finding) int {
	n := 0
	for _, f := range fs {
		if f.Level == "error" {
			n++
		}
	}
	return n
}

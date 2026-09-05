package subtask

// plan_cache.go — 计划缓存（设计 3.8/APC——同类任务第二次更快）
// 一期: 文件制 JSONL 存 /tmp/zerg-plan-templates/——语义指纹=描述关键词集合(Jaccard)
// 入库门槛: 任务最终 done（G9 防污染——失败任务的坏计划不入库）

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const planTemplateDir = "/tmp/zerg-plan-templates"

// PlanTemplate — 计划模板（入库单元）
type PlanTemplate struct {
	TaskDesc  string   `json:"task_desc"`
	Keywords  []string `json:"keywords"`
	PlanJSON  string   `json:"plan_json"`
	CreatedAt int64    `json:"created_at"`
}

// SavePlanTemplate — 入库（G9: 仅 done 任务调用）
func SavePlanTemplate(taskDesc string, p *Plan) error {
	if err := os.MkdirAll(planTemplateDir, 0o755); err != nil {
		return err
	}
	t := PlanTemplate{
		TaskDesc: taskDesc,
		Keywords: extractKeywords(taskDesc),
		PlanJSON: p.ToJSON(),
	}
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	// 文件名=关键词 hash（同签名覆盖——同类任务保留最新最佳计划）
	name := strings.Join(t.Keywords, "-")
	if len(name) > 80 {
		name = name[:80]
	}
	return os.WriteFile(filepath.Join(planTemplateDir, name+".json"), b, 0o644)
}

// FindPlanTemplate — 检索（Jaccard 相似度——≥0.35 命中）
func FindPlanTemplate(taskDesc string) (*Plan, float64) {
	entries, err := os.ReadDir(planTemplateDir)
	if err != nil {
		return nil, 0
	}
	kw := extractKeywords(taskDesc)
	best := 0.0
	var bestPlan *Plan
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(planTemplateDir, e.Name()))
		if err != nil {
			continue
		}
		var t PlanTemplate
		if json.Unmarshal(b, &t) != nil {
			continue
		}
		sim := jaccard(kw, t.Keywords)
		if sim > best {
			var p Plan
			if json.Unmarshal([]byte(t.PlanJSON), &p) == nil && len(p.Steps) > 0 {
				best = sim
				bestPlan = &p
			}
		}
	}
	if best >= 0.35 {
		return bestPlan, best
	}
	return nil, 0
}

// extractKeywords — 关键词抽取（去停用词的 2+ 字符中文词/英文词——简化 trigram）
func extractKeywords(desc string) []string {
	stop := map[string]bool{"的": true, "了": true, "在": true, "是": true, "和": true, "与": true,
		"然后": true, "最后": true, "接着": true, "并且": true, "work": true, "txt": true,
		"工作区": true, "创建": true, "内容": true, "写入": true, "文件": true, "任务": true, "报告": false}
	// 分词: 连续中文段按 2-gram + 英文词
	var kws []string
	seen := map[string]bool{}
	words := strings.FieldsFunc(desc, func(r rune) bool {
		return !(r >= 0x4e00 && r <= 0x9fff) && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && r != '.' && r != '_'
	})
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if len(w) < 2 || stop[w] {
			continue
		}
		// 中文段 2-gram
		runes := []rune(w)
		if len(runes) >= 2 && runes[0] >= 0x4e00 {
			for i := 0; i+2 <= len(runes); i++ {
				g := string(runes[i : i+2])
				if !stop[g] && !seen[g] {
					seen[g] = true
					kws = append(kws, g)
				}
			}
		} else if !seen[w] {
			seen[w] = true
			kws = append(kws, w)
		}
	}
	sort.Strings(kws)
	return kws
}

// jaccard — 集合相似度
func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setB := map[string]bool{}
	for _, x := range b {
		setB[x] = true
	}
	inter := 0
	for _, x := range a {
		if setB[x] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

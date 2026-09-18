package subtask

// plan_cache.go — 计划缓存（设计 3.8/APC——同类任务第二次更快）
// 一期: 文件制 JSONL 存统一状态目录（原写死 /tmp/zerg-plan-templates/）——语义指纹=描述关键词集合(Jaccard)
// 入库门槛: 任务最终 done（G9 防污染——失败任务的坏计划不入库）
//
// 2026-09-18 修（硬编码治理批 B-1，口径同 api/tasks_persist.go 与 agent/idle_persist.go；
// 目录类同 agent/bash_v101.go）:
//
//	原写死 `const planTemplateDir = "/tmp/zerg-plan-templates"` —— macOS 重启 /tmp 即清 +
//	tmp_cleaner 3 天未访问即删 ⇒ 攒下的同类任务快路经验丢；多实例还共用同一目录互相覆盖。
//	改为 statepath 统一状态目录派生（ZERG_STATE_DIR → ~/.zerg/state/zerg-plan-templates），
//	新目录首次写入自动创建（含父目录）。
//	迁移兼容（首次）: 目录类口径 **只读保留不搬** —— 旧目录存在 ⇒ 仍作可读入口（Find 仍扫得到旧模板）；
//	**写只写新目录**；旧目录**不删、不改**。显式覆盖（测试隔离）⇒ 不复旧目录（防测试读真机 /tmp）。

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

const (
	// planTemplateDirName 计划模板目录在统一状态目录下的目录名。
	planTemplateDirName = "zerg-plan-templates"
	// planTemplateLegacyDefaultDir 旧硬编码落点字面量（一期起写死 const planTemplateDir）。
	// 只作**迁移兼容的只读来源**：旧目录仍在读取入口里（旧模板还能被命中）；
	// 不删、不改、**永不再写入**。
	planTemplateLegacyDefaultDir = "/tmp/zerg-plan-templates"
)

// legacyPlanTemplateDir 旧计划模板目录（包级变量 = 上面的字面量；用例/测试进程隔离可切换）。
var legacyPlanTemplateDir = planTemplateLegacyDefaultDir

// planTemplateDirOverride 显式覆盖（测试隔离）：非空 ⇒ 写/读都用它，且不复旧目录。
var planTemplateDirOverride = ""

// planTemplateLegacyNoticeOnce 旧目录只读兼容提示（进程内一次——Find 每次拆解都跑，防日志刷屏）。
var planTemplateLegacyNoticeOnce sync.Once

// planTemplateWriteDir 写路径：永远是统一状态目录（ZERG_STATE_DIR → ~/.zerg/state/<planTemplateDirName>）。
// 永不写旧 /tmp 目录——多实例各写各的（不再互相覆盖）。
func planTemplateWriteDir() string {
	if p := strings.TrimSpace(planTemplateDirOverride); p != "" {
		return p
	}
	return statepath.File(planTemplateDirName)
}

// planTemplateReadDirs 读取入口：新目录优先；旧目录存在 ⇒ 追加为**只读**入口（目录类口径——只读保留不搬）。
//
// 为什么目录类保留旧目录（与文件类「新在则旧完全不看」的差异，理由写在这）:
//
//	文件类的迁移兼容是「读旧内容一次」（内容会被夹除，故新在则必须不看旧）；
//	目录类没有「读内容」这一步——这里是**检索入口**，不夹除任何内容。
//	去掉旧目录 = 用过旧版本攒下的计划模板从此命中不了（真回归，等于经验白攒）；
//	保留它只是多扫一个只读目录，写路径永远只走新目录（永不写旧目录）。
//	显式覆盖（测试隔离）⇒ 不追加旧目录（覆盖即「已指定唯一来源」，防测试读真机 /tmp）。
func planTemplateReadDirs() []string {
	writeDir := planTemplateWriteDir()
	dirs := []string{writeDir}
	if strings.TrimSpace(planTemplateDirOverride) != "" {
		return dirs // 显式覆盖 ⇒ 不复旧
	}
	legacy := strings.TrimSpace(legacyPlanTemplateDir)
	if legacy == "" || legacy == writeDir {
		return dirs
	}
	if st, err := os.Stat(legacy); err != nil || !st.IsDir() {
		return dirs // 无旧目录——首次运行
	}
	planTemplateLegacyNoticeOnce.Do(func() {
		log.Printf("📜 计划模板目录迁移兼容: 旧 %s 保留只读入口（写入只落 %s；旧目录不删不改）\n", legacy, writeDir)
	})
	return append(dirs, legacy)
}

// PlanTemplate — 计划模板（入库单元）
type PlanTemplate struct {
	TaskDesc  string   `json:"task_desc"`
	Keywords  []string `json:"keywords"`
	PlanJSON  string   `json:"plan_json"`
	CreatedAt int64    `json:"created_at"`
}

// SavePlanTemplate — 入库（G9: 仅 done 任务调用）
// 写只写新路径（统一状态目录派生）——旧 /tmp 目录永不写（2026-09-18 批 B-1）。
func SavePlanTemplate(taskDesc string, p *Plan) error {
	dir := planTemplateWriteDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	return os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644)
}

// FindPlanTemplate — 检索（Jaccard 相似度——≥0.35 命中）
// 读入口按 planTemplateReadDirs 顺序扫（新目录先 ⇒ 同分时新路径优先）；旧目录只读、不删不改。
// 目录缺失/不可读 ⇒ 跳过（两都无 = 空启动不报错，首次运行返回 nil, 0）。
func FindPlanTemplate(taskDesc string) (*Plan, float64) {
	kw := extractKeywords(taskDesc)
	best := 0.0
	var bestPlan *Plan
	for _, dir := range planTemplateReadDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // 该入口不存在/不可读——空启动不报错
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var t PlanTemplate
			if json.Unmarshal(b, &t) != nil {
				continue
			}
			sim := jaccard(kw, t.Keywords)
			if sim > best { // 严格大于 + 新目录先扫 ⇒ 同分保留新路径的那份
				var p Plan
				if json.Unmarshal([]byte(t.PlanJSON), &p) == nil && len(p.Steps) > 0 {
					best = sim
					bestPlan = &p
				}
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
		// S11b 修: 过滤含点号/路径片段的 token(如 "calc2.go"/"tmp/zerg-tasks")——避免文件名以"."开头变隐藏文件
		if len(w) < 2 || stop[w] || strings.ContainsAny(w, "./\\") {
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

// Package subtask — 子任务结晶模式（2026-09-05 设计-子任务结晶模式-20260905）
// 任务拆解（强模型一轮）→ 阶段执行（loopcore×弱模型）→ 结晶（结构化摘要）→ 下一阶段
// 资产=结晶链+产物；上下文=耗材
package subtask

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Contract — 任务验收契约（从 api/contract.go 移入——subtask 成为契约的家）
// api 包保留别名转发（兼容既有调用点）
type Contract struct {
	MustWriteFiles  []string `json:"must_write_files,omitempty"`
	MustPassCmds    []string `json:"must_pass_cmds,omitempty"`
	MustDiffPaths   []string `json:"must_diff_paths,omitempty"`
	MinChangedFiles int      `json:"min_changed_files,omitempty"`
}

// IsEmpty — 空契约（未声明任何验收项）
func (c *Contract) IsEmpty() bool {
	return len(c.MustWriteFiles) == 0 && len(c.MustPassCmds) == 0 &&
		len(c.MustDiffPaths) == 0 && c.MinChangedFiles == 0
}

// Step — 子任务步骤（拆解轮输出单元）
type Step struct {
	ID       string    `json:"id"`                 // A/B/C...N
	Goal     string    `json:"goal"`               // 做什么（一句话）
	Produces []string  `json:"produces"`           // 产出文件清单（相对 workdir）
	Depends  []string  `json:"depends,omitempty"`  // 依赖的前置步骤 id
	Critical bool      `json:"critical,omitempty"` // 关键步骤（失败=任务失败）
	Contract *Contract `json:"contract,omitempty"` // 本步验收契约
}

// Plan — 拆解轮输出（完整计划）
type Plan struct {
	Steps     []Step `json:"steps"`
	Rationale string `json:"rationale,omitempty"` // 拆解思路（审计用）
}

// Validate — 拆解校验（程序持尺——设计 3.1/G1/G8）
// originalTask: 原任务描述（范围锚定用）
// 返回: 错误清单（空=通过——可直接喂回拆解模型做重拆反馈——G3/P-V-E 同构）
func (p *Plan) Validate(originalTask string) []string {
	var errs []string
	// ① 步骤数 2-8（太少=没拆——太多=结晶链膨胀）
	if len(p.Steps) < 2 {
		errs = append(errs, fmt.Sprintf("步骤数 %d < 2——任务未被有效拆解", len(p.Steps)))
	}
	if len(p.Steps) > 8 {
		errs = append(errs, fmt.Sprintf("步骤数 %d > 8——结晶链将膨胀（请合并子任务）", len(p.Steps)))
	}
	// ② id 唯一性 + A 起始
	ids := map[string]bool{}
	for i, s := range p.Steps {
		if s.ID == "" {
			errs = append(errs, fmt.Sprintf("步骤 %d 无 id", i+1))
			continue
		}
		if ids[s.ID] {
			errs = append(errs, fmt.Sprintf("步骤 id 重复: %s", s.ID))
		}
		ids[s.ID] = true
	}
	// ③ 依赖可判定性：B 依赖的 id 必须存在且在其之前（无环——拓扑序）
	for _, s := range p.Steps {
		for _, dep := range s.Depends {
			if !ids[dep] {
				errs = append(errs, fmt.Sprintf("步骤 %s 依赖不存在的 id: %s", s.ID, dep))
			}
		}
	}
	if err := checkNoCycle(p.Steps); err != "" {
		errs = append(errs, err)
	}
	// ④ 契约完整性：每步 contract 至少一项非空（G1 姊妹——无契约=无法机器验收）
	for i := range p.Steps {
		s := &p.Steps[i]
		if s.Contract == nil || s.Contract.IsEmpty() {
			// 产物清单可作为隐式契约（有 produces 亦可）
			if len(s.Produces) == 0 {
				errs = append(errs, fmt.Sprintf("步骤 %s 无验收契约且无产物声明——不可机器验收", s.ID))
			}
		}
	}
	// ⑤ 范围锚定（G1——防拆解幻觉）：每步 goal 关键词必须能在原任务溯源
	// 简化实现: goal 中的核心词（去虚词后长度≥2 的连续段）应在原任务或产出声明中出现
	taskNorm := normalizeForMatch(originalTask)
	for _, s := range p.Steps {
		if !goalAnchored(s.Goal, taskNorm, s.Produces) {
			errs = append(errs, fmt.Sprintf("步骤 %s 的目标「%s」无法在原任务中溯源——疑似拆解幻觉", s.ID, truncateRunes(s.Goal, 40)))
		}
	}
	// ⑥ 危险命令扫描（G8——注入面）：must_pass_cmds 过黑名单
	for _, s := range p.Steps {
		if s.Contract == nil {
			continue
		}
		for _, cmd := range s.Contract.MustPassCmds {
			if hit := dangerousCmd(cmd); hit != "" {
				errs = append(errs, fmt.Sprintf("步骤 %s 的验收命令含危险操作（%s）: %s", s.ID, hit, truncateRunes(cmd, 60)))
			}
		}
	}
	return errs
}

// checkNoCycle — 步骤依赖环检测（DFS）
func checkNoCycle(steps []Step) string {
	byID := map[string]Step{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(id string) string
	dfs = func(id string) string {
		if visiting[id] {
			return "步骤依赖成环: " + id
		}
		if visited[id] {
			return ""
		}
		visiting[id] = true
		if s, ok := byID[id]; ok {
			for _, dep := range s.Depends {
				if msg := dfs(dep); msg != "" {
					return msg
				}
			}
		}
		visiting[id] = false
		visited[id] = true
		return ""
	}
	for _, s := range steps {
		if msg := dfs(s.ID); msg != "" {
			return msg
		}
	}
	return ""
}

// goalAnchored — 范围锚定：goal 的实词能在原任务或产出声明中找到
// 宽松实现（防误杀）: goal 去掉虚词/标点后任一 ≥2 字连续段命中即可
func goalAnchored(goal, taskNorm string, produces []string) bool {
	g := normalizeForMatch(goal)
	// 提取 goal 中的 2-gram 实词段
	seg := []rune(g)
	producesNorm := normalizeForMatch(strings.Join(produces, " "))
	for i := 0; i+2 <= len(seg); i++ {
		frag := string(seg[i : i+2])
		if isStopword2(frag) {
			continue
		}
		if strings.Contains(taskNorm, frag) || strings.Contains(producesNorm, frag) {
			return true
		}
	}
	return false
}

// normalizeForMatch — 归一化（去空白/标点——保留中英文数字）
func normalizeForMatch(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x4e00 && r <= 0x9fff: // 中文
			b.WriteRune(r)
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// isStopword2 — 高频虚词二连（避免"的""了"组合误命中）
func isStopword2(frag string) bool {
	stop := []string{"的一", "一个", "这个", "然后", "并且", "或者", "以及", "进行", "可以", "需要", "最后", "然后"}
	for _, s := range stop {
		if frag == s {
			return true
		}
	}
	// 单字符重复（如"个个"）
	if len(frag) >= 2 && frag[0] == frag[1] {
		// 中文叠字可能实义（如"看看"）——宽放行
		return false
	}
	return false
}

// dangerousCmd — 危险命令黑名单（G8——与 chat.ChatGate 同理念：只拦绝对不可逆+系统级）
func dangerousCmd(cmd string) string {
	m := strings.ToLower(cmd)
	rules := []struct {
		pattern string
		name    string
	}{
		{"rm -rf /", "递归删除根目录"},
		{"rm -fr /", "递归删除根目录"},
		{"mkfs", "格式化"},
		{"shutdown", "关机"},
		{"reboot", "重启"},
		{"poweroff", "关机"},
		{"dd if=/dev/zero", "零填充"},
		{"drop database", "删库"},
		{"git push --force", "强推覆盖远端"},
		{"> /dev/sda", "直写磁盘"},
	}
	for _, r := range rules {
		if strings.Contains(m, r.pattern) {
			return r.name
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// SortedStepIDs — 拓扑排序后的步骤 id（DAG 依赖序——R4 预留并行）
func (p *Plan) SortedStepIDs() []string {
	byID := map[string]Step{}
	inDeg := map[string]int{}
	for _, s := range p.Steps {
		byID[s.ID] = s
		if _, ok := inDeg[s.ID]; !ok {
			inDeg[s.ID] = 0
		}
		for range s.Depends {
			inDeg[s.ID]++
		}
	}
	var order []string
	done := map[string]bool{}
	for len(order) < len(p.Steps) {
		progressed := false
		var ids []string
		for id := range inDeg {
			ids = append(ids, id)
		}
		sort.Strings(ids) // 确定性
		for _, id := range ids {
			if done[id] || inDeg[id] > 0 {
				continue
			}
			order = append(order, id)
			done[id] = true
			progressed = true
			for _, s := range p.Steps {
				if s.ID == id {
					continue
				}
				for _, dep := range s.Depends {
					if dep == id {
						inDeg[s.ID]--
					}
				}
			}
			break
		}
		if !progressed {
			break // 环（Validate 已拦——防御）
		}
	}
	return order
}

// ToJSON — 计划落盘格式（plan.json）
func (p *Plan) ToJSON() string {
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b)
}

// ParsePlanJSON — 从拆解模型输出解析 Plan（剥代码块——容错）
func ParsePlanJSON(modelOutput string) (*Plan, error) {
	s := strings.TrimSpace(modelOutput)
	// 剥 markdown 代码块
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	var p Plan
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, fmt.Errorf("拆解输出非合法 JSON: %w", err)
	}
	return &p, nil
}

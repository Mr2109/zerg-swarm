package subtask

// crystal.go — 结晶类型与程序核验（2026-09-05 设计 3.2 + 缺漏 G2/G5/G11）

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CrystalExitKind — 阶段退出形态（对齐 loopcore.ExitKind + 契约核验合成）
type CrystalExitKind string

const (
	ExitDone    CrystalExitKind = "done"    // 契约全过
	ExitPartial CrystalExitKind = "partial" // 契约部分过/异常退出但产物在（G4/G5——进度保全）
	ExitBlocked CrystalExitKind = "blocked" // 契约没过且无产物（阶段失败）
)

// Crystal — 阶段结晶（结构化——设计 3.2 schema + G2 过程教训字段）
type Crystal struct {
	StepID    string          `json:"step_id"`
	Goal      string          `json:"goal"`                // goal 原文（程序填——非模型）
	ExitKind  CrystalExitKind `json:"exit_kind"`           // done/partial/blocked
	Results   []string        `json:"results"`             // 契约核验结果（程序填）
	Artifacts []string        `json:"artifacts"`           // 实际落盘产物（程序 glob——防编造）
	KeyData   []string        `json:"key_data"`            // 关键数据（模型提取——精确值：路径/命令/数字）
	Lessons   []string        `json:"lessons,omitempty"`   // 过程教训（G2——坑与绕法——1-2 条）
	Influence string          `json:"influence,omitempty"` // 对后续影响（1-3 句）
	Model     string          `json:"model"`               // 执行模型
	Tokens    int64           `json:"tokens"`              // 本阶段执行 token（G6 分相计量）
	DurationS int             `json:"duration_s"`          // 本阶段墙钟秒
	Timestamp float64         `json:"timestamp"`
}

// Validate — 程序核验结晶（G2 防空防编——设计 3.2 程序职责）
// workdir: 实际工作区（产物比对）
func (c *Crystal) Validate(workdir string) []string {
	var errs []string
	if len(c.KeyData) == 0 {
		errs = append(errs, "结晶缺关键数据（路径/命令/数字至少 1 条）")
	}
	// 产物清单与磁盘比对（防编造——只检查声明的 must 文件是否真实存在）
	for _, art := range c.Artifacts {
		p := art
		if !filepath.IsAbs(p) {
			p = filepath.Join(workdir, art)
		}
		if _, err := os.Stat(p); err != nil {
			errs = append(errs, fmt.Sprintf("结晶产物未落盘: %s", art))
		}
	}
	return errs
}

// SummaryLine — 单行摘要（注入下阶段结晶链用——紧凑）
func (c *Crystal) SummaryLine() string {
	status := "✅"
	switch c.ExitKind {
	case ExitPartial:
		status = "⚠️"
	case ExitBlocked:
		status = "❌"
	}
	s := fmt.Sprintf("%s [%s] %s", status, c.StepID, truncateRunes(c.Goal, 60))
	if len(c.Artifacts) > 0 {
		s += " | 产物: " + truncateRunes(joinN(c.Artifacts, ",", 3), 80)
	}
	if c.Influence != "" {
		s += " | " + truncateRunes(c.Influence, 60)
	}
	return s
}

// Fallback — 程序兜底结晶（G5/G11——模型结晶失败时零模型调用保链不断）
// 只用程序数据: goal 原文+契约核验结果+文件清单
func Fallback(step Step, exitKind CrystalExitKind, contractResults []string, workdir string, model string, tokens int64, durS int) *Crystal {
	c := &Crystal{
		StepID:    step.ID,
		Goal:      step.Goal,
		ExitKind:  exitKind,
		Results:   contractResults,
		Model:     model,
		Tokens:    tokens,
		DurationS: durS,
		Timestamp: float64(time.Now().Unix()),
	}
	// 产物=契约 must_write_files 与 must 产物声明中实际存在的
	for _, f := range step.Produces {
		if _, err := os.Stat(filepath.Join(workdir, f)); err == nil {
			c.Artifacts = append(c.Artifacts, f)
		}
	}
	if step.Contract != nil {
		for _, f := range step.Contract.MustWriteFiles {
			p := filepath.Join(workdir, f)
			if _, err := os.Stat(p); err == nil {
				found := false
				for _, a := range c.Artifacts {
					if a == f {
						found = true
					}
				}
				if !found {
					c.Artifacts = append(c.Artifacts, f)
				}
			}
		}
	}
	// G11: 兜底结晶自带 KeyData（产物路径=程序数据——零模型调用仍满足 Validate ≥1 条）
	for _, a := range c.Artifacts {
		c.KeyData = append(c.KeyData, "产物: "+a)
	}
	if len(c.KeyData) == 0 {
		c.KeyData = append(c.KeyData, "无产物落盘（阶段失败——详见契约核验结果）")
	}
	c.Influence = "（程序兜底结晶——模型结晶失败；详见任务目录 crystallization.jsonl 与 transcript）"
	return c
}

// joinN — 前 n 个元素逗号连接
func joinN(items []string, sep string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	out := ""
	for i, it := range items {
		if i > 0 {
			out += sep
		}
		out += it
	}
	return out
}

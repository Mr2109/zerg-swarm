package compress

import (
	"fmt"
	"strings"
)

// Guard —— 必留闸：压缩前后逐条比对，任一不满足即整次作废。
//
// 七类必留（设计稿 v5 §4.1）由适配器打 Pinned 标；本闸只负责「打了标就必须逐字节还在」，
// 并额外保证 **Anchor（文件路径/命令/标识符）永不丢**。
type Guard struct {
	// StrictAnchors = true 时，输入里出现过的锚点必须在输出里仍能找到（默认 true）。
	StrictAnchors bool
}

func NewGuard() Guard { return Guard{StrictAnchors: true} }

// Check 返回 nil 表示通过。
func (g Guard) Check(before, after []Segment) error {
	// ① 必留逐条：同 Source 下 hash 必须一字不差
	bySource := map[string]map[string]int{}
	for _, s := range after {
		if bySource[s.Source] == nil {
			bySource[s.Source] = map[string]int{}
		}
		bySource[s.Source][Hash(s.Text)]++
	}
	for _, s := range before {
		if !s.Pinned {
			continue
		}
		if bySource[s.Source][Hash(s.Text)] == 0 {
			return fmt.Errorf("必留片段丢了或已被改写（source=%q kind=%s hash=%s）", s.Source, s.Kind, Hash(s.Text))
		}
	}

	if !g.StrictAnchors {
		return nil
	}
	// ② 锚点：输入出现过的必须在输出里找得到（原文或 Anchor 字段均可）
	hay := make([]string, 0, len(after))
	for _, s := range after {
		hay = append(hay, s.Text)
		hay = append(hay, s.Anchors...)
	}
	blob := strings.Join(hay, "\n")
	for _, s := range before {
		for _, a := range s.Anchors {
			if a == "" {
				continue
			}
			if !strings.Contains(blob, a) {
				return fmt.Errorf("锚点丢失（%q，来自 source=%q）—— 路径/命令/标识符永不丢", a, s.Source)
			}
		}
	}
	return nil
}

// guardPinned 是 Apply 内部用的便捷包装。
func guardPinned(before, after []Segment) error { return NewGuard().Check(before, after) }

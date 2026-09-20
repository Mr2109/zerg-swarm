// selfupdate_split.go —— 「`update` / `upgrade` **不分指两事**」的机检面
// （§7.1 `P12` · §3.3 `N2` · §十七 `SD12` · 开工单 T-52 判据②③）。
//
// 判据②：`zerg update` 与 `zerg core update` 语义**不重** —— 前者**源码式自更新**（走 `F-3` 例外清单）、
//
//	后者**主控自身换件**（`P12`）；两者**共用同一支内核脚本**（`scripts/build/zerg-upgrade.sh` 六阶段）。
//
// 判据③：全仓**不再**用 `update` / `upgrade` 分指两事 —— 命令面里不许出现第三个近义动词承接其中任一件事。
//
// ★ 判定口是**函数**（吃一份命令形状列表），不是「直接读全局 commands 然后恒绿」——
//
//	那样它没法做负控，也就证明不了自己会红。
package main

import (
	"fmt"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// updateCmdShape —— 判定口要吃的最小形状。
type updateCmdShape struct {
	Path         string
	Summary      string
	Usage        string
	DangerSource string
}

// updateWordSplitViolations —— 判据②③的**唯一判定口**：返回逐条违规（空 = 全过）。
func updateWordSplitViolations(spec *contract.SelfUpdateSpec, cmds []updateCmdShape) []string {
	out := []string{}
	// ① 两条命令各自的「该说什么」必须逐字对上（语义不重 = 各自说得清自己那件事）。
	for _, key := range []string{"zerg update", "zerg core update"} {
		raw, ok := spec.SemanticsSplit[key]
		if !ok {
			out = append(out, fmt.Sprintf("真源里没有登记 %q 的语义", key))
			continue
		}
		entry, _ := raw.(map[string]any)
		want := strOf(entry["command"])
		needSummary := strOf(entry["summary_must_contain"])
		needSource := strOf(entry["danger_source_must_contain"])
		hit := findShape(cmds, want)
		if hit == nil {
			out = append(out, fmt.Sprintf("%q：命令树里找不到命令 %q", key, want))
			continue
		}
		if needSummary != "" && !strings.Contains(hit.Summary, needSummary) {
			out = append(out, fmt.Sprintf("%s 的概要里没有逐字 %q（现在：%s）", want, needSummary, hit.Summary))
		}
		if needSource != "" && !strings.Contains(hit.DangerSource, needSource) {
			out = append(out, fmt.Sprintf("%s 的语义来源里没有逐字 %q（现在：%s）", want, needSource, hit.DangerSource))
		}
	}
	// ② 除这两条外，**任何**命令都不许再出现近义动词（第三个动词承接同一件事 = 病）。
	allowed := map[string]bool{}
	for _, key := range []string{"zerg update", "zerg core update"} {
		if entry, ok := spec.SemanticsSplit[key].(map[string]any); ok {
			allowed[strOf(entry["command"])] = true
		}
	}
	for _, c := range cmds {
		if allowed[c.Path] {
			continue
		}
		text := c.Path + " · " + c.Summary + " · " + c.Usage
		for _, w := range spec.NearSynonymForbidden {
			if strings.Contains(text, w) {
				out = append(out, fmt.Sprintf("%s 里出现近义动词 %q（§3.3 N2：不许用近义动词分指两事）", c.Path, w))
				break
			}
		}
	}
	return out
}

func findShape(cmds []updateCmdShape, path string) *updateCmdShape {
	for i := range cmds {
		if cmds[i].Path == path {
			return &cmds[i]
		}
	}
	return nil
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

// updateSplitViolations —— 扫**真命令树**（同一份真源，不另抄）。
func updateSplitViolations(spec *contract.SelfUpdateSpec) []string {
	cs := []updateCmdShape{}
	for _, c := range commands {
		src := ""
		if c.danger != nil {
			src = c.danger.Source
		}
		cs = append(cs, updateCmdShape{Path: strings.Join(c.path, " "), Summary: c.summary, Usage: c.usage, DangerSource: src})
	}
	return updateWordSplitViolations(spec, cs)
}

// updateSplitViolationsWith —— 在**真命令树**上叠加若干条覆盖（同路径的替换、新路径的追加）。
// 负控要的正是这个：只把「那一格」改坏，其余整棵树保持原样 —— 否则负控自己会造出
// 「命令树里找不到命令」这类**无关**命中，判词就不干净了（首次落地时就是这么被自己绊倒的）。
func updateSplitViolationsWith(spec *contract.SelfUpdateSpec, overrides []updateCmdShape) []string {
	cs := []updateCmdShape{}
	for _, c := range commands {
		src := ""
		if c.danger != nil {
			src = c.danger.Source
		}
		cs = append(cs, updateCmdShape{Path: strings.Join(c.path, " "), Summary: c.summary, Usage: c.usage, DangerSource: src})
	}
	for _, o := range overrides {
		replaced := false
		for i := range cs {
			if cs[i].Path == o.Path {
				cs[i] = o
				replaced = true
				break
			}
		}
		if !replaced {
			cs = append(cs, o)
		}
	}
	return updateWordSplitViolations(spec, cs)
}

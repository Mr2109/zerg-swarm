package chat

// chat_tool_docsearch.go — doc_search 项目文档检索工具（v2.5.8——2026-09-03 t5）
// 设计: Zerg-内部文档/项目文档/v2.5.8/设计-v2.5.8-文档体系-模型读取成本-20260903.md（2026-09-19 二次分家）
// 核心: 关键词 → docs(项目文档+常青) 命中文件+片段（限长）——替代全文读——片段入上下文
// 行为: 版本目录全扫（当前版命中优先排前）——每文件最多 2 命中行——总输出限长

import (
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const docSearchMaxFiles = 6    // 最多返回命中文件数
const docSearchMaxTotal = 3500 // 总输出字符上限

// docSearch — doc_search 执行器
// args: query(必填——关键词——空格分词 AND 匹配), scope(可选——限定子串如 v2.5.9/常青——默认全 docs)
func docSearch(args map[string]any, workDir string) (string, error) {
	q, _ := args["query"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("doc_search 参数错误: query 必填——如 doc_search query=\"插件体系\"")
	}
	scope, _ := args["scope"].(string)
	terms := strings.Fields(strings.ToLower(q))

	// 检索根：项目文档全部版本（**仓内优先、缺则仓外**——2026-09-19「开发文档分家」）+ 常青
	// ★ 缺件不许静默：两处取源根都不在盘上时**直接报缺件**（不是「无命中」）——
	// 老实现把仓内 `docs/项目文档` 写死成包级常量，分家后 Walk 一个不存在的目录 ⇒ 静默返回「无命中」。
	evergreen := filepath.Join(ZergRepoRoot, "docs", "常青")
	var roots []string
	if base := zergDocsBase(); base != "" {
		roots = append(roots, base)
	}
	if st, err := os.Stat(evergreen); err == nil && st.IsDir() {
		roots = append(roots, evergreen)
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("doc_search 取源根缺失，判据不可判（不给「无命中」）：%s；常青目录 %s 也不在盘上",
			statepath.DocsBaseMissingNote(), evergreen)
	}
	cur := latestVersionDir() // 当前版目录（命中优先）

	type hit struct {
		path   string // 相对 docs 根的展示路径（含版本）
		verKey int    // 版本排序键（当前版 0——其他按字典）
		lines  []string
	}
	hitMap := map[string]*hit{}
	var order []string

	walkFn := func(fp string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(fp, ".md") {
			return nil
		}
		if scope != "" && !strings.Contains(fp, scope) {
			return nil
		}
		data, err := os.ReadFile(fp)
		if err != nil {
			return nil
		}
		content := string(data)
		lc := strings.ToLower(content)
		all := true
		for _, t := range terms {
			if !strings.Contains(lc, t) {
				all = false
				break
			}
		}
		if !all {
			return nil
		}
		// 收集最多 2 个命中行（含上下文 ±0——只取命中行截断）
		lines := strings.Split(content, "\n")
		matches := 0
		snippets := []string{}
		for i, ln := range lines {
			if matches >= 2 {
				break
			}
			if strings.Contains(strings.ToLower(ln), terms[0]) {
				ln = strings.TrimSpace(ln)
				if len(ln) > 150 {
					ln = string([]rune(ln)[:150]) + "…"
				}
				if ln != "" {
					snippets = append(snippets, "L"+itoa(i+1)+": "+ln)
					matches++
				}
			}
		}
		if len(snippets) == 0 { // 命中在文件但无命中行（罕见）——给首行
			ln := strings.TrimSpace(lines[0])
			if len(ln) > 100 {
				ln = string([]rune(ln)[:100])
			}
			snippets = append(snippets, ln)
		}
		rel := docSearchDisplayPath(fp)
		h := &hit{path: rel, verKey: 1, lines: snippets}
		curBase := filepath.Base(cur)
		if cur != "" && strings.HasPrefix(rel, "项目文档"+string(filepath.Separator)+curBase) {
			h.verKey = 0 // 当前版最优先
		} else if strings.HasPrefix(rel, "项目文档") {
			h.verKey = 2 // 历史版最后
		}
		hitMap[rel] = h
		order = append(order, rel)
		return nil
	}
	for _, r := range roots {
		filepath.Walk(r, walkFn)
	}
	if len(order) == 0 {
		note := ""
		if zergDocsBase() == "" {
			note = "\n⚠ " + statepath.DocsBaseMissingNote() // 缺件不静默：说清是「根没找到」而非「确实没写」
		}
		return "doc_search: 无命中（query=\"" + q + "\"）——换关键词或 scope 限定（如 scope=v2.5.9）" + note, nil
	}
	// 排序：当前版(0) → 常青/其他(1) → 历史版(2)，同键按路径
	sort.Slice(order, func(i, j int) bool {
		a, b := hitMap[order[i]], hitMap[order[j]]
		if a.verKey != b.verKey {
			return a.verKey < b.verKey
		}
		return a.path < b.path
	})
	// 输出（限长）
	var b strings.Builder
	b.WriteString("doc_search 命中 " + itoa(len(order)) + " 文件（当前版优先）：\n\n")
	n := 0
	for _, rel := range order {
		if n >= docSearchMaxFiles {
			break
		}
		h := hitMap[rel]
		flag := ""
		if h.verKey == 0 {
			flag = " ★当前版"
		} else if h.verKey == 2 {
			flag = " ⏳历史"
		}
		b.WriteString("◆ " + h.path + flag + "\n")
		for _, s := range h.lines {
			b.WriteString("  " + s + "\n")
		}
		b.WriteString("\n")
		n++
		if utf8.RuneCountInString(b.String()) > docSearchMaxTotal {
			break
		}
	}
	out := b.String()
	if utf8.RuneCountInString(out) > docSearchMaxTotal {
		rs := []rune(out)
		out = string(rs[:docSearchMaxTotal]) + "\n…（结果过长已截断——缩小 scope 或换关键词）"
	}
	return out, nil
}

// docSearchDisplayPath — 展示路径：**一律相对「项目文档」的父目录** ⇒ 分家前后同一形态
// （`项目文档/vX.Y.Z/x.md`），排序键（当前版 0 / 历史版 2）与「当前版」前缀判定照旧成立——
// 否则分家后取源根在仓外，相对 <仓库>/docs 会算成 `../../Zerg-内部文档/…`，前缀判定全失效。
// 不在取源根下的（仓内常青等）按原口径相对 <仓库>/docs 算。
func docSearchDisplayPath(fp string) string {
	clean := filepath.Clean(fp)
	for _, base := range statepath.DocsBaseCandidates() {
		b := filepath.Clean(base)
		if strings.HasPrefix(clean, b+string(filepath.Separator)) {
			if rel, err := filepath.Rel(filepath.Dir(b), clean); err == nil {
				return rel
			}
		}
	}
	rel, _ := filepath.Rel(filepath.Join(ZergRepoRoot, "docs"), clean)
	return rel
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

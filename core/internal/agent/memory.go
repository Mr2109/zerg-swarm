package agent

// memory.go — 虫族 v2.5 Agent 私有记忆（Files 即索引——热冷分离 + BM25 式搜索）
// 设计：docs/设计-v2.5-替代Codex-v2.md 十六记忆
// 实现：2026-08-13（Codex 连续 high demand 失败——我直接写）

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MemoryStore — agent 私有记忆（一事实一文件——Files 即索引）
type MemoryStore struct {
	Dir     string // 记忆目录（.zerg/memory/{agentId}/）
	HotFile string // 热记忆（MEMORY.md——上限 4KB）
}

const hotLimit = 4096 // 热记忆上限（4KB——对齐调研）

// NewMemoryStore — 创建记忆存储（建目录）
func NewMemoryStore(agentID, baseDir string) *MemoryStore {
	dir := filepath.Join(baseDir, "memory", agentID)
	for _, sub := range []string{"facts", "incidents", "lessons"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	return &MemoryStore{
		Dir:     dir,
		HotFile: filepath.Join(dir, "MEMORY.md"),
	}
}

// memID — 生成记忆文件 id（时间戳——唯一）
func memID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// WriteMem — 写一条记忆（一事实一文件——YAML frontmatter）
func (m *MemoryStore) WriteMem(memType, content, tags string) (string, error) {
	subdir := map[string]string{
		"fact": "facts", "incident": "incidents", "lesson": "lessons",
	}[memType]
	if subdir == "" {
		subdir = "facts"
	}
	id := memID()
	path := filepath.Join(m.Dir, subdir, id+".md")
	body := fmt.Sprintf("---\ntype: %s\ntags: %s\nlearned: %s\n---\n\n%s\n",
		memType, tags, time.Now().Format("2006-01-02"), content)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

// WriteFact — 写事实（规则）
func (m *MemoryStore) WriteFact(content, tags string) (string, error) {
	return m.WriteMem("fact", content, tags)
}

// ReadHot — 读热记忆（MEMORY.md——预算内）
func (m *MemoryStore) ReadHot() string {
	data, err := os.ReadFile(m.HotFile)
	if err != nil {
		return ""
	}
	return string(data)
}

// AddToHot — 追加热记忆（超 4KB 截断）
func (m *MemoryStore) AddToHot(content string) error {
	existing := m.ReadHot()
	newContent := existing
	if newContent != "" {
		newContent += "\n"
	}
	newContent += content
	if len(newContent) > hotLimit {
		// 保留尾部（最近——截断超限部分）
		newContent = newContent[len(newContent)-hotLimit:]
	}
	return os.WriteFile(m.HotFile, []byte(newContent), 0o644)
}

// Search — BM25 式搜索（词频 + IDF——不依赖外部库）
// 简化实现：查询词在记忆中出现的文档加权排序（词频 + 长度归一）
func (m *MemoryStore) Search(query string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	type score struct {
		path  string
		score float64
	}
	var results []score

	// 遍历所有记忆文件
	for _, sub := range []string{"facts", "incidents", "lessons"} {
		dir := filepath.Join(m.Dir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			text := strings.ToLower(string(data))
			s := 0.0
			for _, t := range terms {
				count := strings.Count(text, t)
				if count > 0 {
					// 词频 + 长度归一（BM25 简化）
					s += float64(count) / float64(len(text))/100 + 1
				}
			}
			if s > 0 {
				results = append(results, score{path, s})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]string, 0, len(results))
	for _, r := range results {
		data, _ := os.ReadFile(r.path)
		out = append(out, fmt.Sprintf("【%s】\n%s", filepath.Base(filepath.Dir(r.path)), string(data)))
	}
	return out
}

// ListFacts — 列出事实文件
func (m *MemoryStore) ListFacts() []string {
	dir := filepath.Join(m.Dir, "facts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	return out
}

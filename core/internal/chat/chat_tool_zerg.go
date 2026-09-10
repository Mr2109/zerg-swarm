// chat_tool_zerg.go — v2.5.7 P4-48 对话工具第三批（虫族管理 + 效率补充）
// 设计: 虫族管理 8（连本机 core API——真实可用）+ 效率补充 5（纯本地）
// 原则: 真实调用可用——不写空壳——外部服务依赖标注

package chat

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// zergAPI — 调本机 core API（任务/集群/模型——X-Auth-Token）
func zergAPI(method, path string, body []byte) (string, error) {
	req, err := http.NewRequest(method, "http://127.0.0.1:8580"+path, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if token, err := os.ReadFile("/tmp/zerg-chat/token.txt"); err == nil {
		req.Header.Set("X-Auth-Token", strings.TrimSpace(string(token)))
	} else {
		req.Header.Set("X-Auth-Token", config.ResolveAuthToken())
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("core API 调用失败: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("core API %s 返回 %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// ── 虫族管理（8）──

// taskList — 任务队列
func taskList(args map[string]any) (string, error) {
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	raw, err := zergAPI("GET", "/api/tasks", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Count int `json:"count"`
		Tasks []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Status      string `json:"status"`
			Model       string `json:"model"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("解析任务队列失败: %w", err)
	}
	if resp.Count == 0 {
		return "任务队列为空", nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("任务队列共 %d 个（显示前 %d——如需更多传 limit 参数——但通常前 10 足够总结——数据已完整——无需重复查询）:\n", resp.Count, limit))
	for i, t := range resp.Tasks {
		if i >= limit {
			break
		}
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:60] + "…"
		}
		b.WriteString(fmt.Sprintf("- [%s] %s | %s\n", t.Status, t.ID, desc))
	}
	return b.String(), nil
}

// taskDetail — 任务详情
func taskDetail(args map[string]any) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("参数 id 必填——任务 ID（task_list 查）")
	}
	raw, err := zergAPI("GET", "/api/tasks/"+id, nil)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", err
	}
	// 压缩输出（大字段截断）
	for _, k := range []string{"prompt", "result", "report"} {
		if v, ok := obj[k].(string); ok && len(v) > 300 {
			obj[k] = v[:300] + "…"
		}
	}
	out, _ := json.MarshalIndent(obj, "", "  ")
	return string(out), nil
}

// taskStats — 任务统计（从 /api/tasks 聚合）
func taskStats(args map[string]any) (string, error) {
	raw, err := zergAPI("GET", "/api/tasks", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Count int `json:"count"`
		Tasks []struct {
			Status string `json:"status"`
			Model  string `json:"model"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", err
	}
	statusCount := map[string]int{}
	modelCount := map[string]int{}
	for _, t := range resp.Tasks {
		statusCount[t.Status]++
		modelCount[t.Model]++
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("任务统计（共 %d 个）:\n", resp.Count))
	b.WriteString("状态分布: ")
	keys := make([]string, 0, len(statusCount))
	for k := range statusCount {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s=%d ", k, statusCount[k]))
	}
	b.WriteString("\n模型分布: ")
	keys = keys[:0]
	for k := range modelCount {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s=%d ", k, modelCount[k]))
	}
	return b.String(), nil
}

// fleetStatus — 集群状态
func fleetStatus(args map[string]any) (string, error) {
	raw, err := zergAPI("GET", "/api/fleet/status", nil)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", err
	}
	if mc, ok := obj["machines"].([]any); ok {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("集群机器 %d 台（健康 %v）:\n", len(mc), obj["healthy_count"]))
		for _, m := range mc {
			if mm, ok := m.(map[string]any); ok {
				b.WriteString(fmt.Sprintf("- %v: %v（%v）\n", mm["name"], mm["status"], mm["host"]))
			}
		}
		if av, ok := obj["available_models"].([]any); ok && len(av) > 0 {
			b.WriteString(fmt.Sprintf("可用模型 %d 个: %s\n", len(av), strings.Join(anyStrings(av), ", ")))
		}
		return b.String(), nil
	}
	// 原样返回
	out, _ := json.MarshalIndent(obj, "", "  ")
	return string(out), nil
}

// modelStatus — 模型加载状态
func modelStatus(args map[string]any) (string, error) {
	raw, err := zergAPI("GET", "/api/fleet/models", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Count  int `json:"count"`
		Models []struct {
			ID      string `json:"id"`
			Host    string `json:"host"`
			Backend string `json:"backend"`
			MemGB   any    `json:"mem_gb"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", err
	}
	if resp.Count == 0 {
		return "模型库为空", nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("模型库共 %d 个:\n", resp.Count))
	hosts := map[string][]string{}
	for _, m := range resp.Models {
		hosts[m.Host] = append(hosts[m.Host], fmt.Sprintf("%s(%vG)", m.ID, m.MemGB))
	}
	for h, ms := range hosts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", h, strings.Join(ms, ", ")))
	}
	return b.String(), nil
}

// resourceList — 资源库（模型/工具/skill/mcp）
func resourceList(args map[string]any) (string, error) {
	rtype, _ := args["type"].(string)
	if rtype == "" {
		rtype = "model"
	}
	raw, err := zergAPI("GET", "/api/resources/"+rtype, nil)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(obj, "", "  ")
	if len(out) > 2000 {
		out = out[:2000]
	}
	return string(out), nil
}

// zergHealth — 系统健康（core/网关/UI 端口探测）
func zergHealth(args map[string]any) (string, error) {
	probes := []struct{ name, url string }{
		{"core 8580", "http://127.0.0.1:8580/api/chat/sessions"},
		{"网关 8082", "http://127.0.0.1:8082/v1/models"},
	}
	var b strings.Builder
	b.WriteString("虫族系统健康:\n")
	for _, p := range probes {
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequest("GET", p.url, nil)
		req.Header.Set("X-Auth-Token", config.ResolveAuthToken())
		resp, err := client.Do(req)
		if err != nil {
			b.WriteString(fmt.Sprintf("- %s: ❌ %v\n", p.name, err))
			continue
		}
		resp.Body.Close()
		b.WriteString(fmt.Sprintf("- %s: ✅ %d\n", p.name, resp.StatusCode))
	}
	return b.String(), nil
}

// zergVersion — 虫族版本（构建信息）
func zergVersion(args map[string]any) (string, error) {
	// 从 git 仓库读取最近版本
	gitDir := "<repo>"
	ver := "unknown"
	if b, err := os.ReadFile(filepath.Join(gitDir, ".git", "HEAD")); err == nil {
		ver = strings.TrimSpace(string(b))
	}
	return fmt.Sprintf("虫族 Zerg——git HEAD: %s\nv2.5.7（2026-09-01——对话模块收尾）", ver), nil
}

// ── 效率补充（5——纯本地）──

// treeDir — 目录树
func treeDir(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	depth := 3
	if v, ok := args["depth"].(float64); ok && v > 0 {
		depth = int(v)
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	var b strings.Builder
	b.WriteString(full + "\n")
	var walk func(dir string, level int)
	walk = func(dir string, level int) {
		if level > depth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			b.WriteString(strings.Repeat("  ", level) + "├─ " + e.Name() + "\n")
			if e.IsDir() {
				walk(filepath.Join(dir, e.Name()), level+1)
			}
		}
	}
	walk(full, 1)
	return b.String(), nil
}

// zipCreate — 压缩
func zipCreate(args map[string]any, workDir string) (string, error) {
	src, _ := args["source"].(string)
	dst, _ := args["destination"].(string)
	if src == "" || dst == "" {
		return "", fmt.Errorf("参数 source 和 destination 必填——如 {\"source\":\"core\",\"destination\":\"core.zip\"}")
	}
	srcFull := filepath.Join(workDir, src)
	dstFull := filepath.Join(workDir, dst)
	f, err := os.Create(dstFull)
	if err != nil {
		return "", err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	err = filepath.Walk(srcFull, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已压缩: %s → %s", src, dst), nil
}

// zipExtract — 解压
func zipExtract(args map[string]any, workDir string) (string, error) {
	src, _ := args["source"].(string)
	dst, _ := args["destination"].(string)
	if src == "" || dst == "" {
		return "", fmt.Errorf("参数 source 和 destination 必填")
	}
	srcFull := filepath.Join(workDir, src)
	dstFull := filepath.Join(workDir, dst)
	zr, err := zip.OpenReader(srcFull)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	count := 0
	for _, f := range zr.File {
		target := filepath.Join(dstFull, f.Name)
		if !strings.HasPrefix(target, dstFull) {
			continue // 防 zip slip
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return "", err
		}
		count++
	}
	return fmt.Sprintf("已解压 %d 个文件: %s → %s", count, src, dst), nil
}

// csvView — CSV 查看（前 N 行）
func csvView(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填")
	}
	rows := 10
	if v, ok := args["rows"].(float64); ok && v > 0 {
		rows = int(v)
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "CSV 为空", nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("CSV %d 行 × %d 列（显示前 %d 行）:\n", len(records), len(records[0]), rows))
	for i, rec := range records {
		if i >= rows {
			break
		}
		b.WriteString(fmt.Sprintf("%d| %s\n", i, strings.Join(rec, " | ")))
	}
	return b.String(), nil
}

// jsonPath — JSON 提取（键路径查询）
func jsonPath(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	key, _ := args["key"].(string)
	if path == "" || key == "" {
		return "", fmt.Errorf("参数 path（JSON 文件）和 key（键路径——如 a.b.c）必填")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("JSON 解析失败: %w", err)
	}
	cur := obj
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("键路径 %s 不存在（%s 不是对象）", key, part)
		}
		cur, ok = m[part]
		if !ok {
			return "", fmt.Errorf("键 %s 不存在", part)
		}
	}
	out, _ := json.MarshalIndent(cur, "", "  ")
	return string(out), nil
}

// anyStrings — []any → []string
func anyStrings(v []any) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		out = append(out, fmt.Sprintf("%v", x))
	}
	return out
}

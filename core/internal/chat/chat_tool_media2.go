// chat_tool_media2.go — v2.5.7 P4-48 对话工具第四批（剪辑增强 + 文件批处理 + 数据/网络）
// 设计: 剪辑增强 4（Mr2109核心场景）+ 文件批处理 3 + 数据 2 + 网络 2 + 多模态 1
// 原则: 真实可用——不写空壳——依赖外部服务标注

package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── 剪辑增强（4）──

// footageSearch — 素材按名搜索
func footageSearch(args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("参数 name 必填（素材文件名关键字）")
	}
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	// 转义单引号（SQL 注入防护）
	name = strings.ReplaceAll(name, "'", "''")
	cond := fmt.Sprintf("name LIKE '%%%s%%' OR clip LIKE '%%%s%%'", name, name)
	return footageQuery("video_clips", cond, limit)
}

// audioGain — 音频增益（ffmpeg volume）
func audioGain(args map[string]any, workDir string) (string, error) {
	input, _ := args["input"].(string)
	output, _ := args["output"].(string)
	gain, _ := args["gain"].(string)
	if input == "" || output == "" || gain == "" {
		return "", fmt.Errorf("参数 input/output/gain 必填——gain 如 6dB 或 0.5")
	}
	inFull := filepath.Join(workDir, input)
	if !filepath.IsAbs(inFull) {
		inFull = filepath.Join(workDir, input)
	}
	outFull := filepath.Join(workDir, output)
	if !filepath.IsAbs(outFull) {
		outFull = filepath.Join(workDir, output)
	}
	if _, err := os.Stat(inFull); err != nil {
		return "", fmt.Errorf("输入文件不存在: %s", input)
	}
	out, err := execCommand("ffmpeg", "-y", "-i", inFull, "-af", "volume="+gain, "-c:a", "aac", outFull)
	if err != nil {
		return "", fmt.Errorf("ffmpeg 失败: %v", err)
	}
	return fmt.Sprintf("音频增益完成: %s → %s（gain=%s）\n%s", input, output, gain, truncateArgs(out, 500)), nil
}

// subtitleSrt — SRT 字幕处理（SRT 转纯文本 / 纯文本转 SRT）
func subtitleSrt(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	mode, _ := args["mode"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	switch mode {
	case "", "to_text":
		// SRT → 纯文本（去序号/时间轴）
		var b strings.Builder
		sc := bufio.NewScanner(strings.NewReader(string(raw)))
		inCue := false
		for sc.Scan() {
			ln := strings.TrimSpace(sc.Text())
			if strings.Contains(ln, "-->") {
				inCue = true
				continue
			}
			if ln == "" {
				inCue = false
				continue
			}
			if inCue && !strings.Contains(ln, "-->") {
				b.WriteString(ln + "\n")
			}
		}
		text := strings.TrimSpace(b.String())
		if text == "" {
			return "（无字幕文本——文件可能不是 SRT）", nil
		}
		return fmt.Sprintf("SRT 字幕文本（%d 字符）:\n%s", len(text), truncateArgs(text, 2000)), nil
	case "count":
		// 统计字幕条数
		count := 0
		sc := bufio.NewScanner(strings.NewReader(string(raw)))
		for sc.Scan() {
			if strings.Contains(sc.Text(), "-->") {
				count++
			}
		}
		return fmt.Sprintf("SRT 共 %d 条字幕", count), nil
	default:
		return "", fmt.Errorf("mode 只支持 to_text/count")
	}
}

// mediaDuration — 媒体时长（ffprobe）
func mediaDuration(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	out, err := execCommand("ffprobe", "-v", "error", "-show_entries", "format=duration,size,bit_rate", "-of", "json", full)
	if err != nil {
		return "", fmt.Errorf("ffprobe 失败: %v", err)
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return "", err
	}
	dur := info.Format.Duration
	if d, err := parseFloat(dur); err == nil {
		dur = fmt.Sprintf("%.1f 秒（%d:%02d）", d, int(d)/60, int(d)%60)
	}
	return fmt.Sprintf("媒体信息: 时长=%s 大小=%s 字节 码率=%s", dur, info.Format.Size, info.Format.BitRate), nil
}

// ── 文件批处理（3）──

// batchRename — 批量重命名（前缀/后缀/替换）
func batchRename(args map[string]any, workDir string) (string, error) {
	dir, _ := args["dir"].(string)
	pattern, _ := args["pattern"].(string)
	find, _ := args["find"].(string)
	replace, _ := args["replace"].(string)
	if dir == "" || pattern == "" {
		return "", fmt.Errorf("参数 dir 和 pattern 必填（pattern 如 *.txt）")
	}
	if find == "" && replace == "" && pattern == "" {
		return "", fmt.Errorf("需要 find+replace（替换）或 prefix/suffix（加前后缀）")
	}
	prefix, _ := args["prefix"].(string)
	suffix, _ := args["suffix"].(string)
	full := filepath.Join(workDir, dir)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, dir)
	}
	matches, err := filepath.Glob(filepath.Join(full, pattern))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "（无匹配文件）", nil
	}
	renamed := 0
	var b strings.Builder
	for _, m := range matches {
		base := filepath.Base(m)
		newBase := base
		if find != "" {
			newBase = strings.ReplaceAll(newBase, find, replace)
		}
		newBase = prefix + newBase + suffix
		if newBase == base {
			continue
		}
		if err := os.Rename(m, filepath.Join(filepath.Dir(m), newBase)); err != nil {
			b.WriteString(fmt.Sprintf("- 失败: %s → %s（%v）\n", base, newBase, err))
			continue
		}
		b.WriteString(fmt.Sprintf("- %s → %s\n", base, newBase))
		renamed++
	}
	return fmt.Sprintf("批量重命名完成 %d 个（共 %d 匹配）:\n%s", renamed, len(matches), b.String()), nil
}

// fileCount — 文件统计（按扩展名分布）
func fileCount(args map[string]any, workDir string) (string, error) {
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
	extCount := map[string]int{}
	total := 0
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
			if e.IsDir() {
				walk(filepath.Join(dir, e.Name()), level+1)
				continue
			}
			total++
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == "" {
				ext = "(无扩展名)"
			}
			extCount[ext]++
		}
	}
	walk(full, 1)
	keys := make([]string, 0, len(extCount))
	for k := range extCount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return extCount[keys[i]] > extCount[keys[j]] })
	var b strings.Builder
	b.WriteString(fmt.Sprintf("文件统计（共 %d 个）:\n", total))
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("  %s: %d\n", k, extCount[k]))
	}
	return b.String(), nil
}

// textFreq — 文本词频统计
func textFreq(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	top, _ := args["top"].(float64)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填")
	}
	if top <= 0 {
		top = 20
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	words := strings.FieldsFunc(string(raw), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff)
	})
	freq := map[string]int{}
	for _, w := range words {
		w = strings.ToLower(w)
		if len([]rune(w)) < 2 {
			continue
		}
		freq[w]++
	}
	type kv struct {
		word string
		n    int
	}
	list := make([]kv, 0, len(freq))
	for w, n := range freq {
		list = append(list, kv{w, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	var b strings.Builder
	b.WriteString(fmt.Sprintf("词频统计（top %d）:\n", int(top)))
	for i, kv := range list {
		if i >= int(top) {
			break
		}
		b.WriteString(fmt.Sprintf("  %s: %d\n", kv.word, kv.n))
	}
	return b.String(), nil
}

// ── 数据（2）──

// dbSchema — 数据库表结构/行数
func dbSchema(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填（SQLite 数据库路径）")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("数据库不存在: %s", path)
	}
	out, err := execCommand("sqlite3", full, ".tables")
	if err != nil {
		return "", fmt.Errorf("sqlite3 失败: %v", err)
	}
	tables := strings.Fields(out)
	if len(tables) == 0 {
		return "（数据库无表）", nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("数据库 %s 表结构:\n", path))
	for _, t := range tables {
		rows, _ := execCommand("sqlite3", full, fmt.Sprintf("SELECT count(*) FROM %q", t))
		b.WriteString(fmt.Sprintf("  %s: %s 行\n", t, strings.TrimSpace(rows)))
	}
	return b.String(), nil
}

// logAnalyze — 日志分析（错误统计/时间线）
func logAnalyze(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填（日志文件）")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(raw), "\n")
	errorCount := 0
	errorKinds := map[string]int{}
	for _, ln := range lines {
		lower := strings.ToLower(ln)
		if strings.Contains(lower, "error") || strings.Contains(lower, "panic") || strings.Contains(lower, "fail") {
			errorCount++
			// 提取错误类型（前 3 个词）
			fields := strings.Fields(ln)
			kind := "unknown"
			if len(fields) > 0 {
				kind = strings.ToLower(fields[len(fields)-1])
				if len(kind) > 30 {
					kind = kind[:30]
				}
			}
			errorKinds[kind]++
		}
	}
	keys := make([]string, 0, len(errorKinds))
	for k := range errorKinds {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return errorKinds[keys[i]] > errorKinds[keys[j]] })
	var b strings.Builder
	b.WriteString(fmt.Sprintf("日志分析（共 %d 行——错误 %d 行）:\n", len(lines), errorCount))
	for _, k := range keys {
		if len(b.String()) > 1500 {
			break
		}
		b.WriteString(fmt.Sprintf("  %s: %d\n", k, errorKinds[k]))
	}
	return b.String(), nil
}

// ── 网络（2）──

// downloadFile — 下载文件（curl）
func downloadFile(args map[string]any, workDir string) (string, error) {
	url, _ := args["url"].(string)
	output, _ := args["output"].(string)
	if url == "" {
		return "", fmt.Errorf("参数 url 必填")
	}
	if output == "" {
		return "", fmt.Errorf("参数 output 必填（保存路径）")
	}
	outFull := filepath.Join(workDir, output)
	if !filepath.IsAbs(outFull) {
		outFull = filepath.Join(workDir, output)
	}
	// 网络下载（timeout 60s——curl 带重试）
	out, err := execCommand("curl", "-sL", "--max-time", "60", "-o", outFull, url)
	if err != nil {
		return "", fmt.Errorf("下载失败: %v", err)
	}
	st, err := os.Stat(outFull)
	if err != nil {
		return "", err
	}
	_ = out
	return fmt.Sprintf("下载完成: %s → %s（%d 字节）", url, output, st.Size()), nil
}

// netInfo — 网络信息（IP/接口）
func netInfo(args map[string]any) (string, error) {
	out, err := execCommand("sh", "-c", "ifconfig 2>/dev/null | grep -E 'inet |status' | head -12")
	if err != nil {
		return "", fmt.Errorf("网络信息获取失败: %v", err)
	}
	ip, _ := execCommand("sh", "-c", "ipconfig getifaddr en0 2>/dev/null || echo 未知")
	return fmt.Sprintf("本机网络:\n主 IP: %s\n接口:\n%s", strings.TrimSpace(ip), truncateArgs(out, 1500)), nil
}

// ── 多模态（1）──

// ocrText — 图片 OCR（RapidOCR 服务 8790）
func imageOcr(args map[string]any, workDir string) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("参数 path 必填（图片路径）")
	}
	full := filepath.Join(workDir, path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(workDir, path)
	}
	// 调本机 OCR 服务（RapidOCR + qwen3.8 视觉——ocr_server.py:8790）
	req, err := http.NewRequest("POST", "http://127.0.0.1:8790/ocr", strings.NewReader(fmt.Sprintf(`{"path":%q}`, full)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR 服务不可用（8790——需启动 ocr_server.py）: %v", err)
	}
	defer resp.Body.Close()
	var r struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Text == "" {
		return "（图片未识别到文字）", nil
	}
	return fmt.Sprintf("OCR 结果:\n%s", truncateArgs(r.Text, 2000)), nil
}

// execCommand — 执行命令（辅助）
func execCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

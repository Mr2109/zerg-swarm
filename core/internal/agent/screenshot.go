package agent

// screenshot.go — 虫族看图工具（v2.5.5）
// 截图（跨平台系统命令）→ OCR 服务（RapidOCR——文字+坐标）→ 返回结构化描述
// 可选 vision=true → qwen3.8 视觉理解（语义描述——较慢）

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ScreenshotAndOCR 截图 + 识别（虫族看图工具）
// path: 图片路径（空=截全屏）
// vision: 是否用视觉模型语义理解（可选——较慢）
func ScreenshotAndOCR(path string, vision bool) (string, error) {
	// 1. 确定图片路径（没给就截图）
	imagePath := path
	if imagePath == "" {
		imagePath = filepath.Join(statepath.RuntimeLogDir(), "zerg-shot.png")
		if err := takeScreenshot(imagePath); err != nil {
			return "", fmt.Errorf("截图失败: %w", err)
		}
	}

	// 2. OCR 识别（RapidOCR 服务——文字+坐标）
	ocrResult, err := callOCRService(imagePath, false)
	if err != nil {
		return "", fmt.Errorf("OCR 失败: %w", err)
	}

	// 3. 可选视觉理解（qwen3.8——语义）
	if vision {
		visionResult, err := callOCRService(imagePath, true)
		if err == nil {
			return visionResult, nil // 视觉描述更全面
		}
		// 视觉失败——退回 OCR
	}

	return ocrResult, nil
}

// takeScreenshot 跨平台截图（macOS/Linux/Windows）
func takeScreenshot(outPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("screencapture", "-x", outPath).Run()
	case "linux":
		return exec.Command("import", "-window", "root", outPath).Run()
	case "windows":
		ps := "Add-Type -AssemblyName System.Windows.Forms;" +
			"$b=[System.Windows.Forms.Screen]::PrimaryScreen.Bounds;" +
			"$bmp=New-Object System.Drawing.Bitmap($b.Width,$b.Height);" +
			"$g=[System.Drawing.Graphics]::FromImage($bmp);" +
			"$g.CopyFromScreen($b.Location,[System.Drawing.Point]::Empty,$b.Size);" +
			fmt.Sprintf("$bmp.Save('%s')", outPath)
		return exec.Command("powershell", "-c", ps).Run()
	default:
		return fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}
}

// callOCRService 调 OCR 服务（8790——/ocr 或视觉）
func callOCRService(imagePath string, vision bool) (string, error) {
	// OCR 服务地址（可用环境变量覆盖）
	ocrBase := "http://127.0.0.1:8790"
	// vision 用主控网关（qwen3.8——已验证）
	if vision {
		return callVisionModel(imagePath)
	}

	payload := map[string]any{"image": imagePath}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(ocrBase+"/ocr", "application/json", bytes.NewReader(body))
	if err != nil {
		// OCR 服务没起——提示（不自建——服务由 tools/ocr 启动）
		return "", fmt.Errorf("OCR 服务未运行（tools/ocr/ocr_server.py --server）: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result struct {
		OK    bool `json:"ok"`
		Items []struct {
			Text  string    `json:"text"`
			Box   []float64 `json:"box"`
			Score float64   `json:"score"`
		} `json:"items"`
		Count     int    `json:"count"`
		ElapsedMS int    `json:"elapsed_ms"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("OCR 响应解析失败: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("OCR 错误: %s", result.Error)
	}

	// 格式化输出（文字+坐标——位置关系）
	var sb bytes.Buffer
	fmt.Fprintf(&sb, "📸 截图识别（%d 项——%dms）:\n", result.Count, result.ElapsedMS)
	for _, item := range result.Items {
		box := ""
		if len(item.Box) == 4 {
			box = fmt.Sprintf(" [%.0f,%.0f-%.0f,%.0f]", item.Box[0], item.Box[1], item.Box[2], item.Box[3])
		}
		fmt.Fprintf(&sb, "- %s%s (%.2f)\n", item.Text, box, item.Score)
	}
	return sb.String(), nil
}

// callVisionModel 视觉模型理解（qwen3.8——语义描述）
func callVisionModel(imagePath string) (string, error) {
	// 读图片 → base64 → 主控网关 /v1/responses（qwen3.8）
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	payload := map[string]any{
		"model": "Qwen3.8-27B",
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_image", "image_url": "data:image/png;base64," + imgB64},
					{"type": "input_text", "text": "用中文描述这张图的所有内容（标题/文字/布局/颜色）。不要推理过程。"},
				},
			},
		},
		"max_output_tokens": 1000,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 180 * time.Second}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8082/v1/responses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.ResolveAuthToken())
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("视觉模型调用失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("视觉响应解析失败: %w", err)
	}
	var texts []string
	for _, o := range result.Output {
		if o.Type == "message" {
			for _, c := range o.Content {
				if c.Type == "output_text" {
					texts = append(texts, c.Text)
				}
			}
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("视觉模型无输出")
	}
	return "🧠 视觉理解: " + strings.Join(texts, "\n"), nil
}

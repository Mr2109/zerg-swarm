package agent

// verifier.go — v2.5.2 验收器
// 职责：读取 issue 文件，按任务类型执行验收逻辑，返回通过/不通过+原因

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Verify — 验收函数
// 读 issue 文件（任务描述+类型）→ 按类型验收 → 返回 (通过, 原因)
//
// 验收规则：
//   - code：检查 workDir 下是否有测试文件(.go test)或构建产物
//   - report：检查报告中引用的文件是否存在且非空
//   - research：检查报告文件是否存在
//   - fix：检查修复描述是否存在
//
// 通用：从 issue 内容中提取验收目标（如 "写报告 /tmp/x.md"），检查文件存在
func Verify(issuePath, workDir string) (bool, string) {
	data, err := os.ReadFile(issuePath)
	if err != nil {
		return false, "无法读取 issue 文件: " + err.Error()
	}

	content := string(data)

	// 1. 提取类型
	taskType := extractType(content)
	if taskType == "" {
		return false, "无法从 issue 中提取任务类型"
	}

	// 2. 按类型验收
	switch taskType {
	case TaskTypeCode:
		return verifyCode(content, workDir)
	case TaskTypeReport:
		return verifyReport(content, workDir)
	case TaskTypeResearch:
		return verifyResearch(content, workDir)
	case TaskTypeFix:
		return verifyFix(content, workDir)
	default:
		return false, "未知任务类型: " + taskType
	}
}

// extractType — 从 issue 内容中提取任务类型
func extractType(content string) string {
	re := regexp.MustCompile(`- \*\*类型\*\*: (\w+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// verifyCode — code 类型验收：检查测试文件或构建产物
func verifyCode(content, workDir string) (bool, string) {
	// 检查是否有测试文件
	testFiles, err := filepath.Glob(filepath.Join(workDir, "*_test.go"))
	if err == nil && len(testFiles) > 0 {
		return true, "通过：发现 " + strings.Join(testFiles, ", ") + " 测试文件"
	}

	// 检查是否有构建产物（.a 文件或可执行文件）
	buildProducts, err := filepath.Glob(filepath.Join(workDir, "*.a"))
	if err == nil && len(buildProducts) > 0 {
		return true, "通过：发现构建产物 " + strings.Join(buildProducts, ", ")
	}

	// 检查是否有二进制文件
	bins, err := filepath.Glob(filepath.Join(workDir, "*"))
	if err == nil {
		for _, f := range bins {
			info, statErr := os.Stat(f)
			if statErr == nil && !info.IsDir() {
				ext := filepath.Ext(f)
				if ext == "" && info.Size() > 1024 {
					// 无扩展名且大小>1KB，可能是二进制
					return true, "通过：发现可能的构建产物 " + f
				}
			}
		}
	}

	return false, "未通过：未找到测试文件或构建产物（workDir: " + workDir + "）"
}

// verifyReport — report/research 类型验收：检查报告中引用的文件是否存在且非空
func verifyReport(content, workDir string) (bool, string) {
	// 提取文件中提到的路径（Markdown 链接、行内路径等）
	matched := extractFilePaths(content)

	if len(matched) == 0 {
		// 没有明确文件引用，检查通用验收目标
		return verifyGeneralTarget(content, workDir)
	}

	for _, path := range matched {
		// 如果路径是相对路径，尝试在 workDir 下查找
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}

		info, err := os.Stat(path)
		if err != nil {
			continue // 文件不存在，跳过
		}

		if info.IsDir() {
			return true, "通过：报告目录存在 " + path
		}

		if info.Size() > 0 {
			return true, "通过：报告文件存在且非空 " + path
		}
	}

	return false, "未通过：报告中引用的文件不存在或为空"
}

// verifyResearch — research 类型验收：检查报告文件是否存在
func verifyResearch(content, workDir string) (bool, string) {
	// 复用 report 逻辑（research 和 report 验收标准一致）
	return verifyReport(content, workDir)
}

// verifyFix — fix 类型验收：检查修复描述是否存在
func verifyFix(content, workDir string) (bool, string) {
	// 检查是否有修复描述（"## 修复"、"Fix:"、"## 结论" 等）
	indicators := []string{
		"## 修复",
		"## 结论",
		"## 修复记录",
		"Fix:",
		"fix:",
		"## 改动",
		"## 变更",
		"## 修改",
	}

	for _, indicator := range indicators {
		if strings.Contains(content, indicator) {
			return true, "通过：发现修复描述（" + indicator + "）"
		}
	}

	// 检查是否有代码改动描述（宽松——但避免"任务描述含修复二字"误判——需段落级）
	if strings.Contains(content, "## 修复") || strings.Contains(content, "## 结论") || strings.Contains(content, "修复:") {
		return true, "通过：发现修复结论"
	}

	return false, "未通过：未找到修复结论（需 ## 修复/## 结论/修复: 段落）"
}

// verifyGeneralTarget — 通用验收目标提取与检查
// 从任务描述中提取类似 "写报告 /tmp/x.md" 的目标，检查文件是否存在
func verifyGeneralTarget(content, workDir string) (bool, string) {
	// 提取 Markdown 链接中的路径 [text](path)
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	links := linkRe.FindAllStringSubmatch(content, -1)

	for _, link := range links {
		path := link[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return true, "通过：验收目标文件存在 " + path
		}
	}

	// 提取行内路径（/tmp/xxx.md 或 .md 文件）
	pathRe := regexp.MustCompile(`(?:写|创建|生成|输出|保存)[^\S\n]*(\/[^\s\)]+)`)
	paths := pathRe.FindAllStringSubmatch(content, -1)

	for _, p := range paths {
		path := p[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return true, "通过：验收目标文件存在 " + path
		}
	}

	// 检查 issue 中是否有具体的产出要求（如 "输出报告到 xxx"）
	outputRe := regexp.MustCompile(`(?:输出|写到|生成到|保存到)[^\S\n]*(\/[^\s\)]+)`)
	outputs := outputRe.FindAllStringSubmatch(content, -1)

	for _, o := range outputs {
		path := o[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return true, "通过：验收产出文件存在 " + path
		}
	}

	return false, "未通过：未找到明确的验收目标或验收目标文件不存在"
}

// extractFilePaths — 从内容中提取所有可能的文件路径
func extractFilePaths(content string) []string {
	var paths []string

	// Markdown 链接：[text](path)
	linkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for _, m := range linkRe.FindAllStringSubmatch(content, -1) {
		paths = append(paths, m[1])
	}

	// 行内代码路径：`/path/to/file`
	codePathRe := regexp.MustCompile("`(/[^`]+)`")
	for _, m := range codePathRe.FindAllStringSubmatch(content, -1) {
		paths = append(paths, m[1])
	}

	// 绝对路径：以 / 开头，包含 . 的路径（如 /tmp/report.md——RE2 不支持 lookbehind——用简单匹配）
	absPathRe := regexp.MustCompile(`(/[a-zA-Z0-9_./\-]+\.[a-zA-Z]{2,})`)
	for _, m := range absPathRe.FindAllStringSubmatch(content, -1) {
		if !containsPath(paths, m[1]) {
			paths = append(paths, m[1])
		}
	}

	return paths
}

// containsPath — 检查路径切片中是否已包含某路径
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

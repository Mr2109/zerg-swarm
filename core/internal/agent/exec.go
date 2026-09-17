package agent

// exec.go — 虫族 v2.5 Agent 工具执行（M1b：7 个工具的执行逻辑）
// 对齐：Claude Code 工具执行 + 安全校验
// 设计：每个工具执行前过 gate 检查，执行后返回 ToolCallResult

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// 执行上下文

// ExecContext — 工具执行上下文
type ExecContext struct {
	WorkDir   string        // 工作区根目录
	Timeout   time.Duration // 命令超时（默认 30s）
	OutputMax int           // 输出最大字符数（默认 2000）
	AgentName string        // agent 名称（用于 gate 日志）
	// v2.5.5 P2: 父 agent 引用（spawn_agent 子 agent 委托——2026-08-21 Mr2109）
	Parent *Agent
	// S10: 白名单目录（任务目录——结晶模式报告写入——validatePath 例外）
	ExtraAllowDirs []string
	// ── T1.2 观测（实现见 tool_decision.go）──
	// Session: 会话 id（**只观测**；空 ⇒ 观测面无 trace/span 骨架——不编造 id）
	// obs:     本次调用的判定作用域（工具名/参数摘要/是否已记拒绝）
	Session string
	obs     toolObsScope
}

// NewExecContext — 创建执行上下文
func NewExecContext(workDir string) *ExecContext {
	ec := &ExecContext{
		WorkDir:   workDir,
		Timeout:   120 * time.Second, // v2.5：30s→120s（评测/长命令——Codex bash 对齐）
		OutputMax: 16000,             // v2.5：2000→16000（读长文件——Codex 对齐——模型需完整代码）
	}
	// S11d: 任务目录白名单统一注入（ZERG_TASK_DIR——所有 CA 通用）
	// 场景: ①结晶任务报告写入 ②复查任务读被复查任务的报告——都需要跨 workdir 访问任务目录
	if td := os.Getenv("ZERG_TASK_DIR"); td != "" {
		ec.ExtraAllowDirs = append(ec.ExtraAllowDirs, td)
	}
	// S11d: 附加白名单（冒号分隔——复查任务需要访问被复查任务目录）
	if extra := os.Getenv("ZERG_EXTRA_ALLOW_DIR"); extra != "" {
		for _, d := range strings.Split(extra, ":") {
			if d != "" {
				ec.ExtraAllowDirs = append(ec.ExtraAllowDirs, d)
			}
		}
	}
	// bash v1.0.1: 溢出落盘目录白名单（超长输出 spill——模型 read 可续读）
	ec.ExtraAllowDirs = append(ec.ExtraAllowDirs, BashOverflowDir)
	return ec
}

// 安全校验

// validatePath — 校验路径必须在 WorkDir 内（S10: 白名单目录 ExtraAllowDirs 例外——任务目录报告写入）
func (ec *ExecContext) validatePath(relPath string) (string, error) {
	// 清理路径（去除 .. 等）
	cleaned := filepath.Clean(relPath)

	// 绝对路径直接用；相对路径拼 WorkDir
	var absPath string
	if filepath.IsAbs(cleaned) {
		absPath = cleaned
	} else {
		absPath = filepath.Join(ec.WorkDir, cleaned)
	}

	// 确保在 WorkDir 内
	absWorkDir, err := filepath.Abs(ec.WorkDir)
	if err != nil {
		return "", fmt.Errorf("获取工作区绝对路径失败: %w", err)
	}

	// 路径必须在 WorkDir 内（防止逃逸）
	if !strings.HasPrefix(absPath, absWorkDir+string(os.PathSeparator)) && absPath != absWorkDir {
		// S10 白名单: 任务目录（ZERG_TASK_DIR——报告写入场景——结晶模式报告路径冲突治本）
		for _, dir := range ec.ExtraAllowDirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			if strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) || absPath == absDir {
				return absPath, nil
			}
		}
		// T1.2 观测：路径出白名单 = 一个**拒绝分支** ⇒ 记一条 deny（只写观测，判定/错误文本一字不改）
		ec.obsDeny(toolobs.ReasonPathOutside)
		// FFP 式可行动拒绝(2026-09-08——报告路径沙盒冲突治本): 拒绝=教学——
		// 报出合法落点(ZERG_TASK_DIR 已在白名单——报告应写任务目录而非越界自创路径)
		allowed := ""
		if td := os.Getenv("ZERG_TASK_DIR"); td != "" {
			allowed = fmt.Sprintf("。报告/产物请写到任务目录(已在白名单): %s/internal-task-report.md", td)
		}
		return "", fmt.Errorf("路径 %q 不在工作区 %q 内，拒绝访问%s", relPath, ec.WorkDir, allowed)
	}

	return absPath, nil
}

// 工具执行

// dangerousBashPatterns — v2.5.4.9 危险命令黑名单（拦真危险——其余放行——避免白名单误杀）
// 白名单太严（CA 常用命令 go run/curl/python3 会被拦）——黑名单式平衡：
// 只拦"不可逆/系统级"操作——正常开发命令全放行
var dangerousBashPatterns = []struct {
	pattern string
	reason  string
	endOnly bool // true: 模式必须出现在命令结尾（避免误匹配前缀）
}{
	// rm -rf 后跟根/主目录（endOnly——避免误匹配 /tmp 等）
	{"rm -rf /", "删除根目录", true},
	{"rm -rf ~", "删除主目录", true},
	{"sudo rm -rf /", "sudo 删除根目录", true},
	{"sudo rm -rf ~", "sudo 删除主目录", true},
	{"mkfs", "格式化磁盘", false},
	{"dd if=", "磁盘级写入", false},
	{":(){:|:&};:", "fork 炸弹", false},
	{"shutdown", "关机", false},
	{"reboot", "重启", false},
	{"init 0", "关机", false},
	{"chmod -R 777 /", "根目录权限", false},
	// 管道到 sh/bash——下载执行未知代码（危险）；curl -o 下载保存不拦
	{"| sh", "管道执行脚本（未知来源）", false},
	{"| bash", "管道执行脚本（未知来源）", false},
}

// checkDangerousCommand — 检查命令是否含危险模式（防误删/系统级破坏）
// 拦截时给"引导建议"（不是纯拒绝——教模型换安全方式）
// endOnly 模式: 必须出现在命令结尾或后跟空格（如 "rm -rf /" 精确根目录——不误匹配 /tmp）
func checkDangerousCommand(command string) error {
	for _, d := range dangerousBashPatterns {
		if !strings.Contains(command, d.pattern) {
			continue
		}
		if d.endOnly {
			// 模式后必须是空格或结尾（"rm -rf /tmp" 不匹配——因为 / 后还有 tmp）
			idx := strings.Index(command, d.pattern)
			rest := command[idx+len(d.pattern):]
			if rest != "" && !strings.HasPrefix(rest, " ") {
				continue
			}
		}
		return fmt.Errorf("命令含危险操作(%s)——拒绝: %s。建议: 换安全方式（如 rm 具体文件、trash 恢复、备份后操作）", d.reason, d.pattern)
	}
	return nil
}

// executeBash v1.0.0 已升级为 executeBashV101（bash_v101.go——2026-09-06 AI 专用执行契约）
// v1.0.0 问题: 裸文本返回无 exit_code/截断一刀切/无 cwd/timeout_s 参数
// → v1.0.1: 三段式返回+系统断言+防呆拦截+溢出落盘+错误分类引导

// executeRead — 读取文件内容(v1.0.2:类型感知分层路由——多文件类型可读)
// 设计: docs/01-设计/设计-read工具v1.0.2-多类型读取升级-20260908.md
// 路由: 文本(iconv 编码修正+行号)/PDF(pdftotext)/docx·doc·rtf·html(textutil)/epub·odt(pandoc)/
//
//	xlsx(内置 zip+XML 单 sheet TSV)/图像·音频·视频(委托专用工具)/二进制(仅提示)
func (ec *ExecContext) executeRead(ctx context.Context, path string, args map[string]any, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("read", path, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("文件读取被 gate 拦截: %s — %s", path, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("文件不存在或无法访问: %w", err)
	}

	// 参数(分页沿用;v1.0.2 新: num 行号默认开 / format raw 强制原文)
	offset := 1
	if o, ok := args["offset"].(float64); ok && o > 0 {
		offset = int(o)
	}
	limit := 500
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	num := true
	if n, ok := args["num"].(bool); ok {
		num = n
	}
	forceRaw := false
	if fm, ok := args["format"].(string); ok && fm == "raw" {
		forceRaw = true
	}

	rel, _ := filepath.Rel(ec.WorkDir, absPath)
	wrap := func(ftype string, text, note string, total int) string {
		lines := strings.Split(text, "\n")
		if offset > total {
			offset = 1
		}
		end := offset + limit - 1
		if end > total {
			end = total
		}
		var sb strings.Builder
		for i := offset - 1; i < end; i++ {
			ln := lines[i]
			if num {
				ln = fmt.Sprintf("%4d │ %s", i+1, ln)
			}
			if len([]rune(ln)) > 2000 { // 超长行截断(业界规格)
				ln = string([]rune(ln)[:2000]) + " [截断]"
			}
			sb.WriteString(ln)
			sb.WriteString("\n")
		}
		head := fmt.Sprintf("<file path=%q type=%q lines=%d>\n", rel, ftype, total)
		body := sb.String()
		tail := "</file>"
		if note != "" {
			note = "\n" + note
		}
		pageInfo := ""
		if total > limit || offset > 1 {
			pageInfo = fmt.Sprintf("\n[分页] 共 %d 行——显示 %d-%d 行。续读用 read offset=%d limit=%d。", total, offset, end, end+1, limit)
		}
		return head + body + tail + note + pageInfo
	}

	// 头部探测(前 512B)
	head := make([]byte, 512)
	fh, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	n, _ := io.ReadFull(fh, head)
	head = head[:n]
	fh.Close()

	kind := detectReadKind(head, absPath, info.Size())
	if !forceRaw {
		switch kind {
		case "binary":
			ec.obsDeny(toolobs.ReasonUnsupported) // T1.2 观测：本工具不执行该目标（二进制）= 拒绝分支
			return "", fmt.Errorf("文件是二进制(%.0f KB)——不输出原文(防乱码/幻觉)。需要解析可委托专用工具或人工。", float64(info.Size())/1024)
		case "img":
			ec.obsDeny(toolobs.ReasonUnsupported) // T1.2 观测：本工具不执行该目标（图像）= 拒绝分支
			return "", fmt.Errorf("文件是图像——read 不读像素。用 image_desc(描述)或 image_ocr(取文字)工具。")
		case "audio":
			ec.obsDeny(toolobs.ReasonUnsupported) // T1.2 观测：本工具不执行该目标（音频）= 拒绝分支
			return "", fmt.Errorf("文件是音频——read 不支持。用 audio 系工具(如 asr_transcribe 转文字)。")
		case "video":
			ec.obsDeny(toolobs.ReasonUnsupported) // T1.2 观测：本工具不执行该目标（视频）= 拒绝分支
			return "", fmt.Errorf("文件是视频——read 不支持。用 media_info/ffprobe 系工具看元数据。")
		case "pdf", "docx", "doc", "rtf", "html", "epub", "odt", "xlsx":
			if info.Size() > 100*1024*1024 {
				ec.obsDeny(toolobs.ReasonContentTooLarge) // T1.2 观测：超抽取上限 = 拒绝分支
				return "", fmt.Errorf("文档过大 (%.0f MB)——超过 100MB 抽取上限", float64(info.Size())/1024/1024)
			}
			text, note, err := extractDocument(ctx, kind, absPath)
			if err != nil {
				return "", err
			}
			if kind == "pdf" && strings.TrimSpace(text) == "" {
				return "", fmt.Errorf("PDF 无文本层(扫描件/纯图)——用 image_ocr 逐页取字,或先转图片。")
			}
			total := len(strings.Split(text, "\n"))
			return wrap(kind, text, note, total), nil
		}
	}

	// 纯文本路径(500KB 上限沿用)
	if info.Size() > 500*1024 {
		return "", fmt.Errorf("文件过大 (%d bytes)，请用 read offset/limit 分段读取", info.Size())
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	// 编码修正(v1.0.2: UTF-16/GB18030 → UTF-8;iconv 零依赖)
	// format=raw: 原样字节输出——不做编码修正(真正的 raw 语义)
	if !forceRaw {
		if kind == "utf16le" || kind == "utf16be" {
			if d, err := iconvBytes(data, kind); err == nil {
				data = d
			} else {
				return "", fmt.Errorf("UTF-16 转码失败: %v", err)
			}
		} else if !utf8.Valid(data) {
			if d, err := iconvBytes(data, "gb18030"); err == nil {
				data = d
			} else {
				return "", fmt.Errorf("文件非 UTF-8 且 GB18030/UTF-16 转码失败——可能为二进制或特殊编码(可 format=raw 强读)")
			}
		}
	}
	text := string(data)
	total := len(strings.Split(text, "\n"))
	note := ""
	if forceRaw && kind != "text" && kind != "" {
		note = fmt.Sprintf("(format=raw 强制按原文读取——文件类型 %s)", kind)
	}
	return wrap("text", text, note, total), nil
}

// readFileKindName — executeRead 里 kind 的展示名(文档抽取注记)
func docExtractNote(kind string) string {
	return fmt.Sprintf("(已从 %s 抽取文本——原格式/图表/排版可能丢失)", strings.ToUpper(kind))
}

// detectReadKind — 魔数优先+后缀兜底的类型探测
func detectReadKind(head []byte, absPath string, size int64) string {
	// 编码 BOM
	if len(head) >= 2 && head[0] == 0xFF && head[1] == 0xFE {
		return "utf16le"
	}
	if len(head) >= 2 && head[0] == 0xFE && head[1] == 0xFF {
		return "utf16be"
	}
	// PDF
	if len(head) >= 5 && string(head[:5]) == "%PDF-" {
		return "pdf"
	}
	// zip 容器(docx/xlsx/pptx——查 Content_Types)
	if len(head) >= 4 && head[0] == 'P' && head[1] == 'K' && (head[2] == 3 || head[2] == 5) && head[3] == 4 {
		switch kindFromZip(absPath) {
		case "xlsx":
			return "xlsx"
		case "docx":
			return "docx"
		case "pptx":
			return "pptx"
		}
		return "binary"
	}
	// 图像/媒体魔数(RIFF→WAVE=audio、AVI=video;ftyp=视频/图片容器按品牌粗分)
	if len(head) >= 4 && string(head[:4]) == "RIFF" {
		if len(head) >= 12 {
			if string(head[8:12]) == "WAVE" {
				return "audio"
			}
			if string(head[8:12]) == "AVI " {
				return "video"
			}
		}
		return "binary"
	}
	if len(head) >= 8 && bytes.Equal(head[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) ||
		len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF ||
		len(head) >= 4 && string(head[:4]) == "GIF8" {
		return "img"
	}
	// 音频
	if len(head) >= 3 && string(head[:3]) == "ID3" ||
		len(head) >= 4 && string(head[:4]) == "fLaC" ||
		len(head) >= 4 && string(head[:4]) == "OggS" {
		return "audio"
	}
	// 视频/ISO 容器(ftyp)
	if len(head) >= 12 && bytes.Contains(head[:12], []byte("ftyp")) {
		return "video"
	}
	// 后缀兜底(文档类)
	ext := strings.ToLower(filepath.Ext(absPath))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "docx"
	case ".rtf":
		return "rtf"
	case ".html", ".htm":
		return "html"
	case ".epub":
		return "epub"
	case ".odt":
		return "odt"
	case ".xlsx":
		return "xlsx"
	case ".pptx":
		return "pptx"
	}
	// 文本判定:可打印占比 + 无 null(业界 null-byte 探测)
	if bytes.IndexByte(head, 0) >= 0 {
		return "binary"
	}
	printable := 0
	for _, b := range head {
		if b == '\n' || b == '\r' || b == '	' || (b >= 0x20 && b < 0x7F) || b >= 0x80 {
			printable++
		}
	}
	if len(head) > 0 && float64(printable)/float64(len(head)) < 0.9 {
		return "binary"
	}
	return "text"
}

// kindFromZip — 查 zip 内 [Content_Types].xml 判断 office 类型
func kindFromZip(path string) string {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return ""
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "[Content_Types].xml" {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 64*1024))
			rc.Close()
			s := string(b)
			switch {
			case strings.Contains(s, "wordprocessingml"):
				return "docx"
			case strings.Contains(s, "spreadsheetml"):
				return "xlsx"
			case strings.Contains(s, "presentationml"):
				return "pptx"
			}
		}
	}
	return ""
}

// extractDocument — 按类型调系统 CLI 抽取文本(零新依赖)
func extractDocument(ctx context.Context, kind, absPath string) (string, string, error) {
	run := func(sec int, name string, args ...string) (string, error) {
		cctx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, name, args...)
		var out, errb bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%s 抽取失败: %v(%s)", name, err, strings.TrimSpace(errb.String())[:min(200, len(strings.TrimSpace(errb.String())))])
		}
		return out.String(), nil
	}
	var text string
	var err error
	switch kind {
	case "pdf":
		text, err = run(60, "pdftotext", "-layout", "-enc", "UTF-8", absPath, "-")
	case "docx", "doc", "rtf", "html":
		text, err = run(30, "textutil", "-convert", "txt", "-stdout", absPath)
	case "epub", "odt":
		text, err = run(30, "pandoc", absPath, "-t", "plain")
	case "xlsx":
		text, err = xlsxToTSV(absPath)
	default:
		// 该 default 在调用方（executeRead 的外层 switch）已过滤未知类型，实际不可达；
		// T1.2 观测口径见文件头：不虚构原因码，故此处不记判定事件。
		err = fmt.Errorf("不支持的抽取类型: %s", kind)
	}
	if err != nil {
		return "", "", err
	}
	return text, docExtractNote(kind), nil
}

// xlsxToTSV — 内置轻抽取: sharedStrings + 首 sheet → TSV(单 sheet;更多用 db/csv 系)
func xlsxToTSV(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("xlsx 打开失败: %w", err)
	}
	defer zr.Close()
	// sharedStrings
	var shared []string
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, _ := f.Open()
			shared = parseSharedStrings(rc)
			rc.Close()
			break
		}
	}
	// 首 sheet(按名字典序取第一个 sheet1/…,简化取 xl/worksheets/sheet1.xml,缺则任意第一个)
	var sheetFile *zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
			sheetFile = f
			if strings.HasSuffix(f.Name, "sheet1.xml") {
				break
			}
		}
	}
	if sheetFile == nil {
		return "", fmt.Errorf("xlsx 无 worksheet")
	}
	rc, err := sheetFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	return parseSheetTSV(rc, shared)
}

type siItem struct {
	Text string `xml:",chardata"`
	Runs []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func parseSharedStrings(r io.Reader) []string {
	var out []string
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "si" {
			var si struct {
				T  string `xml:"t"`
				Ts []struct {
					Text string `xml:",chardata"`
				} `xml:"r>t"`
			}
			_ = dec.DecodeElement(&si, &se)
			s := si.T
			for _, x := range si.Ts {
				s += x.Text
			}
			out = append(out, s)
		}
	}
	return out
}

func parseSheetTSV(r io.Reader, shared []string) (string, error) {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "row" {
			var row struct {
				Cells []struct {
					Type string `xml:"t,attr"`
					V    string `xml:"v"`
					Is   struct {
						T string `xml:"t"`
					} `xml:"is"`
				} `xml:"c"`
			}
			_ = dec.DecodeElement(&row, &se)
			cells := make([]string, 0, len(row.Cells))
			for _, c := range row.Cells {
				v := c.V
				if c.Type == "s" {
					idx := 0
					fmt.Sscanf(v, "%d", &idx)
					if idx >= 0 && idx < len(shared) {
						v = shared[idx]
					}
				} else if c.Type == "inlineStr" || c.Type == "str" {
					if c.Is.T != "" {
						v = c.Is.T
					}
				}
				cells = append(cells, strings.ReplaceAll(strings.ReplaceAll(v, "\n", " "), "	", " "))
			}
			sb.WriteString(strings.Join(cells, "	"))
			sb.WriteString("\n")
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("xlsx 首 sheet 无数据(可能为空或保护)")
	}
	return sb.String(), nil
}

// iconvBytes — 经系统 iconv 转码到 UTF-8(零依赖)
func iconvBytes(data []byte, from string) ([]byte, error) {
	cmd := exec.Command("iconv", "-f", from, "-t", "UTF-8")
	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// writeOpts — write v1.0.1 参数(设计-write工具v1.0.1-类型感知写入升级-20260909)
type writeOpts struct {
	BOM     bool   // 前加 UTF-8 BOM(Windows/Excel 中文)
	LineEnd string // lf/crlf(默认 lf)
	Format  string // auto(默认,防呆+自检)/ raw(裸写逃生门)
}

// nonTextWriteKinds — 禁文本直写类型(与 read 探测集合对齐——Mr2109拍板)
var nonTextWriteKinds = map[string]bool{
	"pdf": true, "docx": true, "doc": true, "xlsx": true, "pptx": true,
	"epub": true, "odt": true, "binary": true, "img": true, "audio": true, "video": true,
}

// writeTargetKind — 写目标类型(存在→魔数优先;不存在→后缀兜底)
func writeTargetKind(absPath string) string {
	if st, err := os.Stat(absPath); err == nil && st.Mode().IsRegular() {
		if f, err := os.Open(absPath); err == nil {
			h := make([]byte, 512)
			n, _ := io.ReadFull(f, h)
			f.Close()
			return detectReadKind(h[:n], absPath, st.Size())
		}
	}
	return detectReadKind(nil, absPath, 0)
}

// writeTargetBlock — 类型防呆(write/edit 共用):非文本目标 → 可行动拒绝;空串=放行
func writeTargetBlock(absPath string) string {
	kind := writeTargetKind(absPath)
	if !nonTextWriteKinds[kind] {
		return ""
	}
	return fmt.Sprintf("目标为 %s 文件——纯文本写入/替换会损坏其结构(2026-09-09 防呆)。可选:① 用该类型专用工具/脚本生成或修改 ② 先删除目标,再用 write 建文本(.txt/.md/.json 等) ③ write 确需裸写加 format=raw(自担损坏)", strings.ToUpper(kind))
}

// validateWritten — 结构化写后自检(json/xml 硬校验;yaml 有解析器则校验)
// 返回 (注记, 错误串)——错误串非空=校验失败(调用方须回滚删除)
func validateWritten(absPath string) (string, string) {
	ext := strings.ToLower(filepath.Ext(absPath))
	switch ext {
	case ".json":
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", "读取校验失败: " + err.Error()
		}
		if !json.Valid(data) {
			return "", "JSON 语法错误(写后自检失败)"
		}
		return "", ""
	case ".xml":
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", "读取校验失败: " + err.Error()
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			if _, err := dec.Token(); err != nil {
				if err == io.EOF {
					return "", ""
				}
				return "", "XML 语法错误: " + err.Error()
			}
		}
	case ".yaml", ".yml":
		note, err := validateYAML(absPath)
		return note, err
	}
	return "", ""
}

// validateYAML — python3 yaml 校验(无解析器→跳过注记;不引 Go 依赖——拍板 2)
func validateYAML(absPath string) (string, string) {
	cmd := exec.Command("python3", "-c", "import sys,yaml;yaml.safe_load(sys.stdin.read())")
	in, err := os.Open(absPath)
	if err != nil {
		return "", ""
	}
	defer in.Close()
	cmd.Stdin = in
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), "No module named") {
			return "(yaml 校验跳过:系统无 yaml 解析器——建议 write 后 read 复核)", ""
		}
		return "", "YAML 语法错误: " + strings.TrimSpace(errb.String())[:min(200, len(strings.TrimSpace(errb.String())))]
	}
	return "", ""
}

// executeWrite — 原子写文件(v1.0.1:类型防呆+回读校验+结构化自检+编码/行尾)
// 设计: docs/01-设计/设计-write工具v1.0.1-类型感知写入升级-20260909.md
func (ec *ExecContext) executeWrite(ctx context.Context, path string, content string, o writeOpts, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("write", path, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("文件写入被 gate 拦截: %s — %s", path, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}

	// 目录目标拒绝
	if st, err := os.Stat(absPath); err == nil && st.IsDir() {
		ec.obsDeny(toolobs.ReasonPathIsDir) // T1.2 观测：write 目标为目录 = 拒绝分支
		return "", fmt.Errorf("目标是目录——write 需指向文件路径")
	}

	// 内容规整: 行尾统一 + BOM(v1.0.1)
	le := o.LineEnd
	if le == "" {
		le = "lf"
	}
	s := strings.ReplaceAll(content, "\r\n", "\n")
	if le == "crlf" {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	} else {
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	if o.BOM {
		s = "\xEF\xBB\xBF" + s
	}
	// 大小上限(10MB——L0 预检)
	if len(s) > 10*1024*1024 {
		ec.obsDeny(toolobs.ReasonContentTooLarge) // T1.2 观测：内容超上限 = 拒绝分支
		return "", fmt.Errorf("内容过大 (%d KB > 10MB 上限)——拒绝写入(防 OOM/误写巨型内容)", len(s)/1024)
	}

	// 类型防呆(2.1——format=raw 逃生门跳过)
	if o.Format != "raw" {
		if blk := writeTargetBlock(absPath); blk != "" {
			ec.obsDeny(toolobs.ReasonNonTextTarget) // T1.2 观测：类型防呆（非文本目标）= 拒绝分支
			return "", fmt.Errorf("%s", blk)
		}
	}

	// 创建父目录
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建父目录失败: %w", err)
	}

	// 事务原子写(Resilient Write L1 四阶段): 临时写→回读校验→原子 rename
	tmpFile := absPath + ".tmp"
	cleanup := func() {
		if rmErr := os.Remove(tmpFile); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "⚠️ 临时文件清理失败 %s: %v\n", tmpFile, rmErr)
		}
	}
	if err := os.WriteFile(tmpFile, []byte(s), 0o644); err != nil {
		return "", fmt.Errorf("写临时文件失败: %w", err)
	}
	back, err := os.ReadFile(tmpFile)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("回读校验失败: %w", err)
	}
	if !bytes.Equal(back, []byte(s)) {
		cleanup()
		return "", fmt.Errorf("写后回读校验不符(%d vs %d 字节)——已撤销(磁盘/截断问题)", len(back), len(s))
	}
	if err := os.Rename(tmpFile, absPath); err != nil {
		cleanup()
		return "", fmt.Errorf("重命名文件失败: %w", err)
	}

	// 结构化写后自检(2.2——坏则回滚删除,不留坏文件)
	if note, verr := validateWritten(absPath); verr != "" {
		os.Remove(absPath)
		return "", fmt.Errorf("写后自检失败——已回滚删除目标:%s", verr)
	} else if note != "" {
		return fmt.Sprintf("已写入 %d 字节到 %s\n%s", len(s), path, note), nil
	}

	return fmt.Sprintf("已写入 %d 字节到 %s", len(s), path), nil
}

// executeEdit — 替换文件内容
// 精确匹配 search 字符串并替换
func (ec *ExecContext) executeEdit(ctx context.Context, path string, search string, replace string, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("edit", fmt.Sprintf("search=%q, replace=%q", search, replace), ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("文件编辑被 gate 拦截: %s — %s", path, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}

	// v1.0.1 类型防呆(与 write 共用——search/replace 落文档类同样毁文件——拍板 1)
	if blk := writeTargetBlock(absPath); blk != "" {
		ec.obsDeny(toolobs.ReasonNonTextTarget) // T1.2 观测：类型防呆（非文本目标）= 拒绝分支
		return "", fmt.Errorf("%s", blk)
	}

	// 读取原文件
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)

	// 查找并替换（第一处匹配）
	idx := strings.Index(content, search)
	if idx == -1 {
		return "", fmt.Errorf("在 %s 中未找到匹配: %q", path, search)
	}

	newContent := content[:idx] + replace + content[idx+len(search):]

	// 原子写入(edit 沿用 write v1.0.1 原子写/回读/自检——但类型防呆已在上方拦非文本目标)
	return ec.executeWrite(ctx, path, newContent, writeOpts{}, nil)
}

// executeApplyPatch — 精确补丁应用（Codex apply_patch 借鉴——2026-08-21 Mr2109 P0）
// 用系统 patch 命令严格应用——位置不对/上下文不匹配→失败报错（不默默改错）
func (ec *ExecContext) executeApplyPatch(ctx context.Context, path string, patch string, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("apply_patch", fmt.Sprintf("path=%s", path), ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("补丁应用被 gate 拦截: %s — %s", path, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}

	// 写补丁到临时文件
	patchFile, err := os.CreateTemp("", "zerg-patch-*.diff")
	if err != nil {
		return "", fmt.Errorf("创建补丁临时文件失败: %w", err)
	}
	defer os.Remove(patchFile.Name())
	if _, err := patchFile.WriteString(patch + "\n"); err != nil {
		return "", fmt.Errorf("写补丁文件失败: %w", err)
	}
	patchFile.Close()

	// 严格应用（patch 命令——--fuzz=0 精确匹配——失败报错）
	cmd := exec.CommandContext(ctx, "patch", "--fuzz=0", "--silent", absPath, patchFile.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 应用失败——返回错误（不默默改错——模型重新读再写）
		return "", fmt.Errorf("补丁应用失败（位置/上下文不匹配——重新读文件再写）: %v\n%s", err, tailStr(string(out), 300))
	}
	return "✅ 补丁应用成功（精确匹配）\n" + tailStr(string(out), 200), nil
}

// executeSpawnAgent — 子 agent 委托（agent 间通信——2026-08-21 Mr2109 P2）
// 派子 agent 执行独立子任务——返回精简摘要
// machine 参数: 跨机协作预留（当前同机 SpawnSubagent——多机调度后续）
func (ec *ExecContext) executeSpawnAgent(ctx context.Context, prompt, subType, machine string, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("spawn_agent", fmt.Sprintf("prompt=%s", truncate(prompt, 50)), ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("子 agent 派发被 gate 拦截: %s", decision.Message)
		}
	}

	// 子 agent 类型
	st := TypeGeneral
	switch subType {
	case "explore":
		st = TypeExplore
	case "plan":
		st = TypePlan
	}

	// 派子 agent（同机——独立上下文——精简摘要）
	if ec.Parent == nil {
		ec.obsDeny(toolobs.ReasonUnsupported) // T1.2 观测：能力缺失（无父上下文）⇒ 不执行 = 拒绝分支
		return "", fmt.Errorf("父 agent 未注入（spawn_agent 需要父上下文——当前环境不支持）")
	}
	res, err := SpawnSubagent(ec.Parent, TaskInput{
		Description: truncate(prompt, 80),
		Prompt:      prompt,
		Type:        st,
	})
	if err != nil {
		return "", fmt.Errorf("子 agent 执行失败: %w", err)
	}

	// 返回摘要（含机器信息——双机协作可追溯）
	m := machine
	if m == "" {
		m = "默认"
	}
	return fmt.Sprintf("✅ 子 agent 完成（机器: %s——类型: %s）\n摘要: %s\n工具: %s\n终止: %s",
		m, st, res.Summary, res.ToolUse, res.Terminates), nil
}

// executeTodo — 任务清单管理（Claude Code todo 借鉴——2026-08-21 Mr2109）
// 持久化到工作区 .zerg/todo.json——跨轮次保留
func (ec *ExecContext) executeTodo(ctx context.Context, action string, args map[string]any, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("todo", action, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("todo 被 gate 拦截: %s", decision.Message)
		}
	}

	// todo 文件路径（工作区 .zerg/todo.json）
	todoFile := filepath.Join(ec.WorkDir, ".zerg", "todo.json")

	// 读取现有清单
	type todoItem struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	items := []todoItem{}
	if data, err := os.ReadFile(todoFile); err == nil {
		_ = json.Unmarshal(data, &items)
	}

	switch action {
	case "create":
		// 从 args 解析 items
		rawItems, ok := args["items"].([]any)
		if !ok || len(rawItems) == 0 {
			return "", fmt.Errorf("create 需要 items 数组")
		}
		items = []todoItem{}
		for _, raw := range rawItems {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			content, _ := m["content"].(string)
			status, _ := m["status"].(string)
			if status == "" {
				status = "pending"
			}
			items = append(items, todoItem{Content: content, Status: status})
		}
	case "update":
		// 更新某项状态
		itemID := -1
		if id, ok := args["item_id"].(float64); ok {
			itemID = int(id)
		}
		status, _ := args["status"].(string)
		if itemID < 0 || itemID >= len(items) {
			return "", fmt.Errorf("item_id %d 超出范围（共 %d 项）", itemID, len(items))
		}
		if status == "" {
			return "", fmt.Errorf("update 需要 status")
		}
		items[itemID].Status = status
	case "list", "":
		// 列出当前清单
	default:
		return "", fmt.Errorf("未知 action: %s（create/update/list）", action)
	}

	// 持久化
	if err := os.MkdirAll(filepath.Dir(todoFile), 0755); err != nil {
		return "", fmt.Errorf("创建 todo 目录失败: %w", err)
	}
	data, _ := json.MarshalIndent(items, "", "  ")
	if err := os.WriteFile(todoFile, data, 0644); err != nil {
		return "", fmt.Errorf("写入 todo 失败: %w", err)
	}

	// 返回清单
	var b strings.Builder
	b.WriteString("📋 任务清单（%d 项）:\n")
	for i, it := range items {
		mark := "⬜"
		switch it.Status {
		case "completed":
			mark = "✅"
		case "in_progress":
			mark = "🔄"
		case "cancelled":
			mark = "🚫"
		}
		b.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, mark, it.Content))
	}
	return fmt.Sprintf(b.String(), len(items)), nil
}

// tailStr 输出末尾 N 字符
func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// executeGlob — 文件模式匹配
// 支持 * 和 ** 通配符
func (ec *ExecContext) executeGlob(ctx context.Context, pattern string, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("glob", pattern, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("glob 被 gate 拦截: %s — %s", pattern, decision.Message)
		}
	}

	// 匹配文件（v2.5 修复：filepath.Glob 不支持 ** 递归——手写）
	budget := newWalkBudget()
	matches, err := globRecursive(ec.WorkDir, pattern, budget)
	if err != nil {
		return "", fmt.Errorf("glob 匹配失败: %w", err)
	}

	// 只返回文件（不含目录）——P4-40 排除隐藏目录（.git/.zerg/.codegraph 等——防 193 行噪音淹没模型）
	var files []string
	for _, m := range matches {
		hidden := false
		for _, seg := range strings.Split(m, string(filepath.Separator)) {
			if strings.HasPrefix(seg, ".") {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		abs := filepath.Join(ec.WorkDir, m) // v2.5 修复：m 是相对路径——须拼 WorkDir
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// 转为相对路径
			rel, _ := filepath.Rel(ec.WorkDir, abs)
			files = append(files, rel)
		}
	}

	if len(files) == 0 {
		return "无匹配文件", nil
	}

	return strings.Join(files, "\n") + budget.Note(), nil
}

// executeGrep — 搜索文件内容
// 返回匹配行 + 行号
func (ec *ExecContext) executeGrep(ctx context.Context, path string, pattern string, args map[string]any, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("grep", fmt.Sprintf("pattern=%q", pattern), ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("grep 被 gate 拦截: %s — %s", pattern, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}

	// 编译正则
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("正则表达式无效: %w", err)
	}

	// 读取文件（目录→递归搜索所有文本文件——v2.5 修复）
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("读取失败: %w", err)
	}
	if info.IsDir() {
		// 目录递归搜索
		var matches []string
		budget := newWalkBudget()
		filepath.Walk(absPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				if strings.HasPrefix(fi.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !budget.Allowed() { // 丙：预算到顶 ⇒ 早停（少走，而不是事后掐断）
				return filepath.SkipAll
			}
			// 跳过隐藏文件
			if strings.HasPrefix(fi.Name(), ".") {
				return nil
			}
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if re.MatchString(line) {
					rel, _ := filepath.Rel(absPath, p)
					matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if len(matches) == 0 {
			return "（目录中无匹配）", nil
		}
		// v2.5.1: 分页（offset 跳过——模型控制看后续匹配——治本非截断）
		offset := 0
		if o, ok := args["offset"].(float64); ok && o > 0 {
			offset = int(o)
		}
		const maxGrepMatches = 200
		total := len(matches)
		if offset >= total {
			return "（已到最后——无更多匹配）", nil
		}
		end := offset + maxGrepMatches
		if end > total {
			end = total
		}
		shown := matches[offset:end]
		result := strings.Join(shown, "\n")
		if total > end {
			result += fmt.Sprintf("\n…（共匹配 %d 条——显示 %d-%d。看后续用 grep offset=%d）", total, offset+1, end, end)
		}
		return result, nil
	}

	// 单文件读取
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 逐行搜索
	lines := strings.Split(string(data), "\n")
	var results []string
	for i, line := range lines {
		if re.MatchString(line) {
			results = append(results, fmt.Sprintf("%d: %s", i+1, line))
		}
	}

	if len(results) == 0 {
		return "无匹配", nil
	}

	return strings.Join(results, "\n"), nil
}

// lsOptions — ls v1.0.2 参数(设计-ls工具v1.0.2-目录概览升级-20260908)
type lsOptions struct {
	Limit   int    // 最大列出行(0=不截断)
	DirOnly bool   // 只列目录
	Hidden  bool   // 显示点文件/目录(默认隐藏——治 .zerg 噪音)
	Pattern string // 名称 fnmatch 过滤(与 glob 分工: 概览内收窄)
	SortBy  string // name/time/size(目录恒在前)
}

// executeLs — 列出目录内容(ls v1.0.2——权限格式修复/摘要行/截断引导/排序/参数)
func (ec *ExecContext) executeLs(ctx context.Context, path string, o lsOptions, gate ToolGater) (string, error) {
	// Gate 检查
	if gate != nil {
		decision, err := gate.Check("ls", path, ec.AgentName)
		if err != nil {
			return "", fmt.Errorf("gate 检查失败: %w", err)
		}
		if decision.Action == "block" {
			return "", fmt.Errorf("ls 被 gate 拦截: %s — %s", path, decision.Message)
		}
	}

	// 路径校验
	absPath, err := ec.validatePath(path)
	if err != nil {
		return "", err
	}

	// 读取目录
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %w", err)
	}

	// 过滤+收集(点项默认隐藏;dir_only;pattern)
	type row struct {
		name  string
		dirs  bool
		size  int64
		mod   time.Time
		perms string
		extra string // 符号链接 -> 目标
	}
	var rows []row
	for _, entry := range entries {
		name := entry.Name()
		if !o.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if o.Pattern != "" {
			if ok, _ := filepath.Match(o.Pattern, name); !ok {
				continue
			}
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mode := info.Mode()
		isDir := mode.IsDir()
		if o.DirOnly && !isDir {
			continue
		}
		r := row{name: name, dirs: isDir, size: info.Size(), mod: info.ModTime()}
		full := filepath.Join(absPath, name)
		if mode&os.ModeSymlink != 0 {
			// 符号链接: 类型 l + 目标权限(os.Stat 跟随——GNU ls 一致——拍板 4)
			if tgt, err := os.Readlink(full); err == nil {
				r.extra = " -> " + tgt
			}
			permMode := mode
			if st, err := os.Stat(full); err == nil {
				permMode = st.Mode()
			} else {
				permMode = 0 // 断链——全 '-' 权限
			}
			r.perms = "l" + lsPermsBody(permMode)
		} else {
			r.perms = formatPermissions(mode)
		}
		rows = append(rows, r)
	}

	// 排序: 目录恒在前(拍板内固定);组内按 sort_by
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].dirs != rows[j].dirs {
			return rows[i].dirs // 目录在前
		}
		switch o.SortBy {
		case "time":
			if !rows[i].mod.Equal(rows[j].mod) {
				return rows[i].mod.After(rows[j].mod)
			}
		case "size":
			if rows[i].size != rows[j].size {
				return rows[i].size > rows[j].size
			}
		}
		return rows[i].name < rows[j].name
	})

	// 摘要行
	dirCount, fileCount := 0, 0
	for _, r := range rows {
		if r.dirs {
			dirCount++
		} else {
			fileCount++
		}
	}
	total := len(rows)
	showTime := o.SortBy == "time" // 拍板 3: sort_by=time 显示时间列(本地 AI 判新旧——Mr2109)
	lines := []string{fmt.Sprintf("== %s (共 %d 项:%d 目录 / %d 文件)==", absPath, total, dirCount, fileCount)}
	if total == 0 {
		switch {
		case o.DirOnly:
			lines = append(lines, "(无目录)")
		case o.Pattern != "":
			lines = append(lines, "(无匹配项)")
		default:
			lines = append(lines, "(空目录)")
		}
		return strings.Join(lines, "\n"), nil
	}

	limit := o.Limit
	if limit <= 0 {
		limit = total
	}
	for i, r := range rows {
		if i >= limit {
			lines = append(lines, fmt.Sprintf("… [已省略 %d 项——共 %d 项——用 pattern 或 glob 缩小范围]", total-limit, total))
			break
		}
		name := r.name
		if r.dirs {
			name += "/"
		}
		name += r.extra
		sizeCol := fmt.Sprintf("%8d", r.size)
		if r.dirs {
			sizeCol = "       -"
		}
		line := fmt.Sprintf("%s %s %s", r.perms, sizeCol, name)
		if showTime {
			line += "  " + r.mod.Format("01-02 15:04")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// formatPermissions — 格式化文件权限(标准 10 字符——按用户类分组 rwx/rwx/rwx)
// v1.0.2 修复: 旧版按位型分组(rrr/www/xxx→"drrrw--xxx" 畸形——2026-09-08 日志实锤)
func formatPermissions(mode os.FileMode) string {
	if mode.IsDir() {
		return "d" + lsPermsBody(mode)
	}
	if mode&os.ModeSymlink != 0 {
		return "l" + lsPermsBody(mode)
	}
	return "-" + lsPermsBody(mode)
}

// lsPermsBody — 9 位权限体(owner rwx + group rwx + other rwx)
func lsPermsBody(mode os.FileMode) string {
	p := make([]byte, 9)
	// owner
	p[0], p[1], p[2] = bitRwx(mode, 0o400, 0o200, 0o100)
	// group
	p[3], p[4], p[5] = bitRwx(mode, 0o040, 0o020, 0o010)
	// other
	p[6], p[7], p[8] = bitRwx(mode, 0o004, 0o002, 0o001)
	return string(p)
}

// bitRwx — 单类 rwx 三元组(某类读/写/执行位)
func bitRwx(mode os.FileMode, r, w, x os.FileMode) (byte, byte, byte) {
	rb, wb, xb := byte('-'), byte('-'), byte('-')
	if mode&r != 0 {
		rb = 'r'
	}
	if mode&w != 0 {
		wb = 'w'
	}
	if mode&x != 0 {
		xb = 'x'
	}
	return rb, wb, xb
}

// 工具路由

// toolHelp — 工具帮助（help:true → 读 tools/<名>.md——P4-49 工具履历——按需——零成本复用）
// fallback: 工具定义描述（无履历文件不报错——给 schema 描述）
func toolHelp(name string) ToolCallResult {
	// 1. 读履历文件 tools/<name>.md（含说明/示例/变更记录——进化日记）
	mdPath := filepath.Join(statepath.WorkspaceRoot(), "tools", name+".md")
	if b, err := os.ReadFile(mdPath); err == nil && len(b) > 0 {
		return ToolCallResult{Content: fmt.Sprintf("【工具 %s 帮助】\n%s", name, string(b))}
	}
	// 2. fallback: 工具定义描述（schema——参数说明）
	for _, t := range AllTools() {
		if t.Function.Name == name {
			return ToolCallResult{Content: fmt.Sprintf("【工具 %s 帮助（无独立文档——工具定义）】\n%s", name, t.Function.Description)}
		}
	}
	return ToolCallResult{Content: fmt.Sprintf("未知工具: %s（先 tool_search 搜索）", name)}
}

// ExecuteTool — 根据工具名路由到对应执行函数
// 执行前过 gate 检查，返回 ToolCallResult
func (ec *ExecContext) ExecuteTool(ctx context.Context, toolName string, args map[string]any, gate ToolGater) ToolCallResult {
	// T1.2 观测：开启本次调用的判定作用域（**先于**名字规范化——未知工具名的拒绝也要带上模型发来的原名）
	ec.obsBeginCall(toolName, args)
	// 工具名规范化（2026-09-17 实测：模型发 `Read`（大写）⇒ 区分大小写匹配 ⇒ 报未知工具 ⇒ 白耗一轮）
	// 原则：行为上宽容（大小写不敏感 + 去空白），契约上严格（执行一律用注册表真名）。歧义 ⇒ 拒绝，不猜。
	if canon, denyReason, rerr := resolveToolNameCoded(toolName); rerr == nil {
		if canon != toolName {
			fmt.Fprintf(os.Stderr, "⚠️ 工具名纠正：%q → %q\n", toolName, canon)
		}
		toolName = canon
		ec.obsSetTool(canon) // 观测与执行同真名（日志/观测/计数不分裂）
	} else {
		// T1.2 观测：工具名不合法（未知/歧义）= 拒绝分支 ⇒ 记一条 deny
		ec.obsDeny(denyReason)
		return ToolCallResult{Error: rerr.Error()}
	}
	// T1.2 观测：gate 包装（纯透传）——10 处工具内的 gate 拒绝由这一处集中落观测（判定不变）
	if gate != nil {
		gate = &obsGater{inner: gate, ec: ec}
	}
	// P4-49 统一工具计数（CA 调用计入——成功执行才计）
	// 2026-09-06: 计数+事件流双写(事件=未来账本源——含耗时)
	start := time.Now()
	res := ec.executeToolInner(ctx, toolName, args, gate)
	ec.obsEndCall() // T1.2 收尾：本次无任何拒绝 ⇒ 记一条 allow（有拒绝则已在分支里记过，一次调用只有一条判定）
	if res.Error == "" && toolName != "" && toolName[0] != '_' {
		RecordToolUse(toolName)
		appendToolEvent(ToolEvent{Ts: time.Now().Unix(), Node: nodeName, Tool: toolName, DurMs: time.Since(start).Milliseconds()})
	}
	return res
}

func (ec *ExecContext) executeToolInner(ctx context.Context, toolName string, args map[string]any, gate ToolGater) ToolCallResult {
	// P4-50 工具帮助体系: help:true → 读 tools/<名>.md（提示词只说"不确定先 help"——按需看详细用法）
	if h, ok := args["help"].(bool); ok && h {
		return toolHelp(toolName)
	}
	// 多语言 D2（2026-09-11）：执行接口约定断言——schema 里声明的约定（enum / x-zerg-format）先校验，
	// 违约定（典型：中文值塞进英文枚举 = MLCL 主因）→ 回 FFP 教学文本，不执行。
	// 只认 schema 声明的约定，未声明参数一律放行（中文路径/中文查询词合法——防误伤）。
	if def, ok := ToolDefOf(toolName); ok {
		if vs := ffp.CheckContract(toolName, def.Function.Parameters, args); len(vs) > 0 {
			// T1.2 观测：参数不合约（enum/x-zerg-format）= 拒绝分支 ⇒ 记一条 deny
			ec.obsDeny(toolobs.ReasonParamContract)
			return ToolCallResult{Error: ffp.BuildParamContract(vs[0])}
		}
	}
	switch toolName {
	case "bash":
		command, ok := args["command"].(string)
		if !ok || strings.TrimSpace(command) == "" {
			// FFP 2026-09-08: 格式错误≠执行失败——回结构化教学文本(分类+原文回显+最小示例)
			// 实测样本: 空 command 旧反馈"命令参数为空"零信息→模型原样重发死循环
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			kind := "参数缺失: bash.command"
			sent := "<空/缺失>"
			if !ok {
				kind = "类型错误: bash.command(string)"
				sent = ffp.EchoSafe(args["command"])
			}
			return ToolCallResult{Error: ffp.Build(kind, sent,
				"字段 command:string 非空(trim 后)——为必填字段",
				`{"command": "ls -la"}`,
				"命令字段不能为空——先想好要执行什么,再发完整 JSON 工具调用;命令本身勿用引号包裹 JSON")}
		}
		// v1.0.1: cwd/timeout_s 可选参数（无状态 shell 补偿+超时覆盖）
		cwd, _ := args["cwd"].(string)
		timeoutS := 0
		if tv, ok := args["timeout_s"]; ok {
			if n, ok2 := jsonNumber(tv); ok2 {
				timeoutS = n
			}
		}
		output, err := ec.executeBashV101(ctx, command, cwd, timeoutS, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		// P4-50 端口探测拦截升级（不是模型不信工具——是 bash 走得太顺——系统让模型走正路）
		// lsof LISTEN/ss -tln 全量扫描 → 强制 port_services（中文标注——lsof 原样输出需人工解读+截断）
		// lsof -i :port 单端口 → 强制 port_check（参数 port——兼容数字/字符串）
		lowerCmd := strings.ToLower(command)
		isFullScan := (strings.Contains(lowerCmd, "lsof") && strings.Contains(lowerCmd, "listen")) || strings.Contains(lowerCmd, "ss -tln")
		isPortQuery := strings.Contains(lowerCmd, "lsof") && strings.Contains(lowerCmd, "-i :")
		if isFullScan {
			return ToolCallResult{Content: "【bash lsof/ss 不执行——有专用工具】本机服务/端口清单用 port_services（tool_search 搜 \"端口 服务\" 或 \"系统\" 发现——或系统提示已列）——返回中文标注清单（lsof 原样输出截断且需人工解读——port_services 直接给答案）。请改用 port_services。"}
		}
		if isPortQuery {
			return ToolCallResult{Content: "【bash lsof 不执行——有专用工具】查单端口占用用 port_check（参数 port 传端口号——如 {\"port\":8104}——tool_search 搜 \"系统\" 发现——或系统提示已列）。请改用 port_check。"}
		}
		return ToolCallResult{Content: output}

	case "read":
		path, ok := args["path"].(string)
		if !ok || path == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "路径参数为空"}
		}
		content, err := ec.executeRead(ctx, path, args, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: content}

	case "write":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		if path == "" || content == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "路径或内容参数为空"}
		}
		// write v1.0.1: bom/line_end/format
		wo := writeOpts{}
		if v, ok := args["bom"].(bool); ok && v {
			wo.BOM = true
		}
		if v, ok := args["line_end"].(string); ok && v != "" {
			wo.LineEnd = v
		}
		if v, ok := args["format"].(string); ok && v != "" {
			wo.Format = v
		}
		result, err := ec.executeWrite(ctx, path, content, wo, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "edit":
		path, _ := args["path"].(string)
		search, _ := args["search"].(string)
		replace, _ := args["replace"].(string)
		if path == "" || search == "" || replace == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "路径/搜索/替换参数为空"}
		}
		result, err := ec.executeEdit(ctx, path, search, replace, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "apply_patch":
		// v2.5.5 P0: apply_patch 精确补丁（Codex 借鉴——2026-08-21 Mr2109）
		path, _ := args["path"].(string)
		patch, _ := args["patch"].(string)
		if path == "" || patch == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "路径/补丁参数为空"}
		}
		result, err := ec.executeApplyPatch(ctx, path, patch, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "spawn_agent":
		// v2.5.5 P2: agent 间通信（子 agent 委托——2026-08-21 Mr2109）
		prompt, _ := args["prompt"].(string)
		subType, _ := args["type"].(string)
		machine, _ := args["machine"].(string)
		if prompt == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "prompt 参数为空"}
		}
		result, err := ec.executeSpawnAgent(ctx, prompt, subType, machine, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "todo":
		// v2.5.5 P2: todo 任务清单（Claude Code 借鉴——2026-08-21 Mr2109）
		action, _ := args["action"].(string)
		if action == "" {
			action = "list"
		}
		result, err := ec.executeTodo(ctx, action, args, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "glob":
		pattern, ok := args["pattern"].(string)
		if !ok || pattern == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "模式参数为空"}
		}
		result, err := ec.executeGlob(ctx, pattern, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "grep":
		path, _ := args["path"].(string)
		pattern, _ := args["pattern"].(string)
		if path == "" || pattern == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "路径或模式参数为空"}
		}
		result, err := ec.executeGrep(ctx, path, pattern, args, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "ls":
		path, _ := args["path"].(string)
		// ls v1.0.2: 参数(limit/dir_only/hidden/pattern/sort_by——默认最省 token)
		o := lsOptions{Limit: 60, SortBy: "name"}
		if v, ok := args["limit"].(float64); ok && v >= 0 {
			o.Limit = int(v)
		}
		if v, ok := args["dir_only"].(bool); ok && v {
			o.DirOnly = true
		}
		if v, ok := args["hidden"].(bool); ok && v {
			o.Hidden = true
		}
		if v, ok := args["pattern"].(string); ok {
			o.Pattern = v
		}
		if v, ok := args["sort_by"].(string); ok && v != "" {
			o.SortBy = v
		}
		result, err := ec.executeLs(ctx, path, o, gate)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "web_search":
		query, _ := args["query"].(string)
		if query == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "查询参数为空"}
		}
		// v1.0.1: 参数解析（lang/time_range/domains/fetch_top/rewrite）
		sp := SearchParams{Query: query, Limit: 5}
		if l, ok := args["limit"].(float64); ok && l > 0 {
			sp.Limit = int(l)
		}
		if v, ok := args["lang"].(string); ok && v != "" {
			sp.Lang = v
		}
		if v, ok := args["time_range"].(string); ok && v != "" {
			sp.TimeRange = v
		}
		if v, ok := args["domains"].([]any); ok {
			for _, d := range v {
				if ds, ok := d.(string); ok && ds != "" {
					sp.Domains = append(sp.Domains, ds)
				}
			}
		}
		if v, ok := args["fetch_top"].(float64); ok && v > 0 {
			sp.FetchTop = int(v)
		}
		if v, ok := args["rewrite"].(bool); ok && v {
			sp.Rewrite = true
		}
		result, err := WebSearchV2(sp)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "web_fetch":
		url, _ := args["url"].(string)
		if url == "" {
			ec.obsDeny(toolobs.ReasonParamMissing) // T1.2 观测：参数缺失/空（参数不合法）= 拒绝分支
			return ToolCallResult{Error: "URL 参数为空"}
		}
		result, err := WebFetch(url)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	case "screenshot":
		// v2.5.5: 虫族看图工具——截图+OCR 识别（开发调试）
		path, _ := args["path"].(string)
		vision, _ := args["vision"].(bool)
		result, err := ScreenshotAndOCR(path, vision)
		if err != nil {
			return ToolCallResult{Error: err.Error()}
		}
		return ToolCallResult{Content: result}

	default:
		// P4-49 扩展工具路由（对话层注册——CA/对话统一工具库——Mr2109 2026-09-02）
		if et, ok := LookupExtraTool(toolName); ok {
			content, err := et.Fn(args, ec.WorkDir)
			if err != nil {
				return ToolCallResult{Error: err.Error()}
			}
			return ToolCallResult{Content: content}
		}
		// 教学式错误（2026-09-17 实测：模型把示例里的占位词「工具名」照抄成工具名 ⇒ 调用必败 ⇒ 反复空转）
		// 原则：错误反馈＝教学（可行动、给正确示例、列出可用工具）——不让人/模型猜。
		if isPlaceholderToolName(toolName) {
			ec.obsDeny(toolobs.ReasonPlaceholderTool) // T1.2 观测：占位词当工具名 = 拒绝分支
			return ToolCallResult{Error: fmt.Sprintf(`你把示例里的占位符当成工具名了：「%s」不是工具。name 必须换成真实工具名；正确形态示例：{"name": "read", "arguments": {"path": "core/internal/agent/exec.go"}}`, toolName)}
		}
		ec.obsDeny(toolobs.ReasonUnknownTool) // T1.2 观测：未知工具（默认分支）= 拒绝分支
		return ToolCallResult{Error: fmt.Sprintf(`未知工具: %s。当前可用：%s。正确形态示例：{"name": "read", "arguments": {"path": "core/internal/agent/exec.go"}}`, toolName, availableToolNames())}

		// isPlaceholderToolName — 教学示例中的占位词（被照抄即成假工具名）

		// availableToolNames — 当前注册的工具名（供教学式错误列出；取前 12 个够用）
	}
}

// v2.5.4.9 清理: 删除 var _ = io.EOF 侧链（io import 同步删——未实际使用）

// globRecursive — 支持 ** 递归的 glob 匹配（v2.5——filepath.Glob 不支持 **）
// 实现：拆分段——** 递归遍历目录，其余段用 filepath.Match
func globRecursive(root, pattern string, budget *walkBudget) ([]string, error) {
	// 统一分隔符
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	root = filepath.Clean(root)

	// 处理 root 前缀（模式可能含 WorkDir——去掉）
	if strings.HasPrefix(pattern, root) {
		pattern = strings.TrimPrefix(pattern, root)
		pattern = strings.TrimPrefix(pattern, "/")
	}

	segs := strings.Split(pattern, "/")
	var results []string

	var walk func(dir string, idx int) error
	walk = func(dir string, idx int) error {
		if idx >= len(segs) {
			return nil
		}
		seg := segs[idx]

		if seg == "**" {
			// ** 匹配零或多层——递归子目录
			if idx == len(segs)-1 {
				// ** 在末尾——匹配所有文件
				filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if !budget.Allowed() { // 丙：预算到顶 ⇒ 早停（大卷上少走）
						return filepath.SkipAll
					}
					if !fi.IsDir() {
						rel, _ := filepath.Rel(root, p)
						results = append(results, rel)
					}
					return nil
				})
				return nil
			}
			// ** 在中间——递归匹配后续段
			filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !budget.Allowed() { // 丙：预算到顶 ⇒ 早停
					return filepath.SkipAll
				}
				if fi.IsDir() {
					// 递归尝试后续段
					walk(p, idx+1)
				}
				return nil
			})
			return nil
		}

		// 普通段——匹配当前目录下的条目
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			matched, err := filepath.Match(seg, e.Name())
			if err != nil {
				continue
			}
			if !matched {
				continue
			}
			child := filepath.Join(dir, e.Name())
			if idx == len(segs)-1 {
				// 最后一段——文件
				if !e.IsDir() {
					rel, _ := filepath.Rel(root, child)
					results = append(results, rel)
				}
			} else if e.IsDir() {
				walk(child, idx+1)
			}
		}
		return nil
	}

	// 空模式或 "."——返回根目录文件
	if pattern == "" || pattern == "." {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if !e.IsDir() {
				rel, _ := filepath.Rel(root, filepath.Join(root, e.Name()))
				results = append(results, rel)
			}
		}
		return results, nil
	}

	walk(root, 0)
	return results, nil
}

// ── 以下两个 helper 由 2026-09-17 修复「模型照抄占位词」时加入（顶格：函数不能嵌套在函数体内）──
func isPlaceholderToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "工具名", "参数名", "参数值", "tool_name", "tool", "name", "函数名", "工具":
		return true
	}
	return false
}

func availableToolNames() string {
	var names []string
	for _, t := range AllTools() {
		names = append(names, t.Function.Name)
		if len(names) >= 12 {
			break
		}
	}
	return strings.Join(names, " · ")
}

// ── 工具名规范化（2026-09-17 Mr2109 拍：② 程序层大小写锁定为主，① 约定/教学为辅）────────────────

// resolveToolName — 把模型给的工具名解析成注册表里的**真名**。
//
// 规则（Poka-yoke：行为宽容、契约严格）：
//
//	· 去首尾空白 + 大小写不敏感（Read/READ/ read  都指向 read）
//	· 歧义零容忍：若注册表中存在只差大小写的多个名字 ⇒ 拒绝执行并返回教学式错误（不猜）
//	· 返回的永远是注册表真名 ⇒ 日志/观测/权限/计数/履历不分裂
func resolveToolName(raw string) (string, error) {
	name, _, err := resolveToolNameCoded(raw)
	return name, err
}

// resolveToolNameCoded — 同 resolveToolName，另返回**拒绝原因码**（T1.2 观测用）。
// 判定逻辑只有这一份（resolveToolName 转调本函数）——观测绝不分叉出第二套"名字合法性"判断。
func resolveToolNameCoded(raw string) (string, string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	var hits []string
	for _, t := range AllTools() {
		if strings.ToLower(t.Function.Name) == name {
			hits = append(hits, t.Function.Name)
		}
	}
	switch len(hits) {
	case 0:
		return raw, toolobs.ReasonUnknownTool, fmt.Errorf("未知工具: %s。当前可用：%s。正确形态示例：{\"name\": \"read\", \"arguments\": {\"path\": \"core/internal/agent/exec.go\"}}", raw, availableToolNames())
	case 1:
		return hits[0], "", nil
	default:
		return raw, toolobs.ReasonAmbiguousTool, fmt.Errorf("工具名有歧义：%s 同时存在。请用完整真名（本系统工具名一律小写）", strings.Join(hits, " / "))
	}
}

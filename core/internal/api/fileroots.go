package api

// fileroots.go — 文件/目录浏览器 阶段 1 后端（《设计-文件浏览器集装箱-20260913》§4.2 / §4.4）
//
// 三件事：
//  ① GET  /api/fileroots             —— 五根白名单 + 可配置项（类型清单 / 界面显示上限）
//  ② GET  /api/docs?root=&path=      —— 既有文档读接口参数化到白名单根（缺省路径行为逐字不变，见 handlers.go）
//  ③ POST /api/fileroots/open|reveal —— 把目录/文件「交给系统」打开，白名单校验 + 审计留痕
//
// 为什么不放开任意路径：根集合是**显式声明**（不做目录发现），路径只接受相对形式，
// 解析后还要复核物理位置仍在根内——否则一个符号链接就能把白名单变成后门（设计 §三 非目标）。

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── ① 根集合 ────────────────────────────────────────────────────────────────

// FileRoot 是白名单根的一项（JSON 字段名与设计 §4.2 逐字一致）。
type FileRoot struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Default  bool   `json:"default"`
	Writable bool   `json:"writable"`
}

// weightsDirEnv 是「模型权重」根的环境变量（不设时用 <HOME>/models）。
const weightsDirEnv = "ZERG_WEIGHTS_DIR"

// weightsDir 返回模型权重根：ZERG_WEIGHTS_DIR 优先，缺省 <HOME>/models。
// 为什么单列一个根：GGUF 权重与同目录的 mmproj、LICENSE 只有在这一层看得见
// （登记库里只有 JSON）——设计 §九 Q1 Mr2109 2026-09-13 拍板加到五项。
func weightsDir() string {
	if d := strings.TrimSpace(os.Getenv(weightsDirEnv)); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "models"
	}
	return filepath.Join(home, "models")
}

// docsRootPath 解析虫族文档根（<仓库根>/docs）。
// 与 docs_ops.go 的 docsRoot 同源（同一个 statepath 解析器，且 docsRoot 正是它 init 时的取值）；
// 这里特意在**调用时**解析，使测试能用 ZERG_WORKSPACE 指向固定目录快照做逐字节断言。
func docsRootPath() string { return filepath.Join(statepath.WorkspaceRoot(), "docs") }

// fileRoots 返回五项白名单根（顺序即接口契约顺序）。
// 路径全部由既有解析器给出（statepath / modelreg / HOME），不写死绝对路径：
// 换机器、换安装位置、测试注入都只改环境变量，不改代码（设计 §八 R2）。
// 路径不存在也照样返回——列表接口对不存在的根返回空列表，由 UI 显示「不存在」。
func fileRoots() []FileRoot {
	ws := statepath.WorkspaceRoot()
	return []FileRoot{
		{ID: "docs", Label: "虫族文档", Path: docsRootPath(), Default: true, Writable: true},
		{ID: "repo", Label: "虫族仓库", Path: ws},
		{ID: "models", Label: "模型登记库", Path: modelreg.DefaultModelsDir()},
		{ID: "tasks", Label: "任务目录", Path: statepath.TaskRoot()},
		{ID: "weights", Label: "模型权重", Path: weightsDir()},
	}
}

// findFileRoot 按 id 查白名单根。
// 未知 id 一律 false——**绝不回退到某个默认根**，否则「打错一个字」就变成越权浏览别处。
func findFileRoot(id string) (FileRoot, bool) {
	for _, r := range fileRoots() {
		if r.ID == id {
			return r, true
		}
	}
	return FileRoot{}, false
}

// ── ② 可配置项（设计 §4.4「可调开关」：放开类型 = 改配置、不改代码）──────────

const (
	fileBrowserTextExtsEnv   = "ZERG_FILEBROWSER_TEXT_EXTS"       // 类型清单（逗号分隔）
	fileBrowserAllowAllEnv   = "ZERG_FILEBROWSER_ALLOW_ALL_TYPES" // =1 放开为任意类型
	fileBrowserDisplayMaxEnv = "ZERG_FILEBROWSER_DISPLAY_MAX"     // 界面内渲染上限（字节）
)

// defaultTextExts 是默认文本类型清单（设计 §4.4 所列，逐字）。
var defaultTextExts = []string{".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".go", ".rs", ".sh", ".py", ".log", ".csv", ".ini", ".conf"}

// defaultDisplayMax 是界面内**渲染**上限（1 MiB）。
// 注意它只管显示：读取侧不设上限（Mr2109 2026-09-13 拍板，设计 §九 Q3）——
// 后端不许留「读不到」的能力缺口，截断只发生在界面里。
const defaultDisplayMax int64 = 1048576

// FileBrowserConfig 是接口的 config 段（前端据此决定能读什么、显示多少、放不放行任意类型）。
type FileBrowserConfig struct {
	DisplayMax    int64    `json:"display_max"`
	AllowAllTypes bool     `json:"allow_all_types"`
	TextExts      []string `json:"text_exts"`
}

// fileBrowserTextExts 解析类型清单：环境变量优先（逗号分隔，自动补前导点、转小写），否则用默认。
func fileBrowserTextExts() []string {
	if v := strings.TrimSpace(os.Getenv(fileBrowserTextExtsEnv)); v != "" {
		out := []string{}
		for _, p := range strings.Split(v, ",") {
			p = strings.ToLower(strings.TrimSpace(p))
			if p == "" {
				continue
			}
			if !strings.HasPrefix(p, ".") {
				p = "." + p
			}
			out = append(out, p)
		}
		if len(out) > 0 {
			return out
		}
	}
	return append([]string(nil), defaultTextExts...)
}

// fileBrowserAllowAll 读放开开关（默认关——避免误点执行任意可执行文件）。
func fileBrowserAllowAll() bool {
	return strings.TrimSpace(os.Getenv(fileBrowserAllowAllEnv)) == "1"
}

// fileBrowserDisplayMax 读界面显示上限；非法或非正值回落默认（1 MiB）。
func fileBrowserDisplayMax() int64 {
	if v := strings.TrimSpace(os.Getenv(fileBrowserDisplayMaxEnv)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultDisplayMax
}

// fileBrowserConfig 组装 config 段。
func fileBrowserConfig() FileBrowserConfig {
	return FileBrowserConfig{
		DisplayMax:    fileBrowserDisplayMax(),
		AllowAllTypes: fileBrowserAllowAll(),
		TextExts:      fileBrowserTextExts(),
	}
}

// fileBrowserExtAllowed 判定路径扩展名是否在允许清单内（放开开关打开时一律放行）。
// 无扩展名文件默认不允许（Mr2109 2026-09-13：无扩展名只给「在访达中显示」——
// 因此读取与打开两个动作在默认配置下都拒，放开开关一并放开）。
func fileBrowserExtAllowed(p string) bool {
	if fileBrowserAllowAll() {
		return true
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext == "" {
		return false
	}
	for _, e := range fileBrowserTextExts() {
		if e == ext {
			return true
		}
	}
	return false
}

// ── ③ 路径安全 ──────────────────────────────────────────────────────────────

// safePathIn 把「根 id + 相对路径」解析成根内绝对路径。
// 成功返回 (abs, "")；失败返回 ("", 错误码)：
//
//	INVALID_ROOT —— 根不在白名单；
//	INVALID_PATH —— 空串 / .. 段 / 绝对路径 / 含 NUL / 物理路径逃出根（含符号链接）。
//
// 为什么解析后还要 EvalSymlinks 复核：只做字符串前缀比对挡不住符号链接——
// 根内放一个指向 /etc 的链接，前缀照样是根，但读出去的已经是根外（设计 §4.4 路径行）。
func safePathIn(rootID, rel string) (string, string) {
	root, ok := findFileRoot(rootID)
	if !ok {
		return "", "INVALID_ROOT"
	}
	if rel == "" || strings.Contains(rel, "\x00") || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", "INVALID_PATH"
	}
	// 逐段拒 ".."：按子串拒会把 "..x.md" 这类合法文件名误杀，按段判才精确
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return "", "INVALID_PATH"
		}
	}
	rootClean := filepath.Clean(root.Path)
	cand := filepath.Join(rootClean, filepath.FromSlash(rel))
	// 复核一：字符串层面仍在根内（Join 会 Clean，异常拼接在这里现形）
	if cand != rootClean && !strings.HasPrefix(cand, rootClean+string(os.PathSeparator)) {
		return "", "INVALID_PATH"
	}
	// 复核二：物理层面仍在根内（符号链接逃逸）
	real, ok := resolvePhysical(cand)
	if !ok {
		return "", "INVALID_PATH"
	}
	rootReal, ok := resolvePhysical(rootClean)
	if !ok {
		return "", "INVALID_PATH"
	}
	if !withinDir(real, rootReal) {
		return "", "INVALID_PATH"
	}
	return cand, ""
}

// resolvePhysical 取路径的物理位置：整条不存在时逐级上溯到存在的祖先求真实位置，再拼回剩余段。
// 为什么要上溯：读一个「还不存在」的路径是常态（前端会先探路），不能因为 EvalSymlinks
// 报错就当非法；但也不能因此跳过逃逸复核，所以对存在的祖先求真实位置，其余段原样拼回。
func resolvePhysical(p string) (string, bool) {
	p = filepath.Clean(p)
	var missing []string
	cur := p
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		missing = append(missing, filepath.Base(cur))
		cur = parent
	}
}

// withinDir 判断 p 是否落在 dir 内（或就是 dir 自己）。
func withinDir(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(os.PathSeparator))
}

// listFileRoot 列根内条目（相对该根的相对路径）。
// docs 根保持改造前的语义（只收 *.md，排除 issues/thunderbolt 噪音）；
// 其余根收全部文件——权重根的意义就是看见 .gguf/mmproj/LICENSE，按 .md 过滤会把它们全滤掉。
// 根不存在时 Walk 首个回调即带 err，返回空列表（设计 §4.4：不报错，交给 UI 显示「不存在」）。
func listFileRoot(root FileRoot) (files, dirs []string) {
	files, dirs = []string{}, []string{}
	_ = filepath.Walk(root.Path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(root.Path, p)
		if rerr != nil {
			return nil
		}
		isDocs := root.ID == "docs"
		if info.IsDir() {
			if rel == "." {
				return nil // 根本身不进列表
			}
			if isDocs && (strings.Contains(rel, "/issues") || strings.Contains(rel, "/thunderbolt") || rel == "issues" || rel == "thunderbolt") {
				return nil
			}
			dirs = append(dirs, rel)
			return nil
		}
		if isDocs {
			if strings.HasSuffix(p, ".md") && !strings.Contains(p, "/issues/") && !strings.Contains(p, "/thunderbolt/") {
				files = append(files, rel)
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files, dirs
}

// ── ④ 审计（设计 §4.4 审计行；Mr2109 2026-09-13 拍板「按意见」）──────────────
//
// 位置：~/.zerg/logs/ui-actions.log。为什么不用 /tmp：审计的用途是「事后能查」，
// 而 /tmp 是可丢位置（与 RuntimeLogDir 的运行日志分家）。
// 字段：时间(ISO8601)/动作/根/相对路径/结果 —— **不记文件内容**（免得把凭证类文本抄进日志）。
// 保留：单文件满 1 MiB 轮转留 5 份 + 30 天清理（open/reveal 是低频动作，这个量足够）。

const (
	defaultAuditMaxBytes int64 = 1 << 20 // 1 MiB
	defaultAuditKeep           = 5
	defaultAuditAgeDays        = 30
	uiActionsLogName           = "ui-actions.log"
)

// uiActionEntry 是审计行（一行一个 JSON 对象：路径里带空格/中文时也能被可靠解析）。
type uiActionEntry struct {
	Time   string `json:"time"`
	Action string `json:"action"`
	Root   string `json:"root"`
	Path   string `json:"path"`
	Result string `json:"result"`
}

// fileBrowserAuditParams 是审计写入的可注入参数（零值 = 生产默认）。
// 存在理由：轮转与 30 天清理必须能被「造小上限 + 伪时钟」验证（设计 §七 第 6 条），
// 而生产不需要这些旋钮——所以只在 Handlers 上注入，不引环境变量。
type fileBrowserAuditParams struct {
	Dir      string           // 覆盖目录（空 = ~/.zerg/logs）
	MaxBytes int64            // 单文件上限（0 = 1 MiB）
	Keep     int              // 轮转保留份数（0 = 5）
	AgeDays  int              // 轮转文件保留天数（0 = 30）
	Now      func() time.Time // 伪时钟（nil = time.Now）
}

// resolved 用生产默认补齐零值。
func (p fileBrowserAuditParams) resolved() fileBrowserAuditParams {
	if p.Dir == "" {
		p.Dir = uiActionsLogDir()
	}
	if p.MaxBytes <= 0 {
		p.MaxBytes = defaultAuditMaxBytes
	}
	if p.Keep <= 0 {
		p.Keep = defaultAuditKeep
	}
	if p.AgeDays <= 0 {
		p.AgeDays = defaultAuditAgeDays
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	return p
}

// uiActionsLogDir 是审计日志目录：~/.zerg/logs（statepath 持久状态根的兄弟目录）。
// 用状态根推出来而不是另立一份环境变量：状态根已是唯一真源（ZERG_STATE_DIR 一处改、两处跟）。
func uiActionsLogDir() string {
	return filepath.Join(filepath.Dir(statepath.Dir()), "logs")
}

// fileBrowserAuditParamsFromHandlers 取注入参数（nil = 全默认）。
func (h *Handlers) fileBrowserAuditParamsFromHandlers() fileBrowserAuditParams {
	if h.FileBrowserAudit == nil {
		return fileBrowserAuditParams{}.resolved()
	}
	return h.FileBrowserAudit.resolved()
}

// auditUIAction 为一次 open/reveal 落一行审计。
// 审计失败只告警、不改变动作结果——审计是留痕，不该反过来让「打开文件」失败。
// 成功与失败都写（失败动作同样留痕，结果列错误码）——设计 §七 第 6 条。
func (h *Handlers) auditUIAction(action, rootID, rel, result string) {
	p := h.fileBrowserAuditParamsFromHandlers()
	line, err := json.Marshal(uiActionEntry{
		Time:   p.Now().Format(time.RFC3339),
		Action: action,
		Root:   rootID,
		Path:   rel,
		Result: result,
	})
	if err != nil {
		log.Printf("⚠️ ui-actions audit marshal failed: %v", err)
		return
	}
	if err := appendAuditLine(p, string(line)+"\n"); err != nil {
		log.Printf("⚠️ ui-actions audit write failed: %v", err)
	}
}

// appendAuditLine 追加一行：写前按大小轮转，写后清过期轮转文件。
func appendAuditLine(p fileBrowserAuditParams, line string) error {
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(p.Dir, uiActionsLogName)
	if fi, err := os.Stat(path); err == nil && fi.Size()+int64(len(line)) > p.MaxBytes {
		rotateAuditLog(path, p.Keep)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(line)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	cleanupRotatedAuditLogs(p)
	return nil
}

// rotateAuditLog 把 ui-actions.log 滚成 .1，旧 .N 依次后移，末位丢弃（共保留 keep 份）。
func rotateAuditLog(path string, keep int) {
	_ = os.Remove(fmt.Sprintf("%s.%d", path, keep))
	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	_ = os.Rename(path, path+".1")
}

// cleanupRotatedAuditLogs 删除超过保留期的轮转文件（每次写入顺带清）。
func cleanupRotatedAuditLogs(p fileBrowserAuditParams) {
	matches, err := filepath.Glob(filepath.Join(p.Dir, uiActionsLogName+".*"))
	if err != nil {
		return
	}
	cutoff := p.Now().Add(-time.Duration(p.AgeDays) * 24 * time.Hour)
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.Remove(m)
		}
	}
}

// ── ⑤ 接口 ──────────────────────────────────────────────────────────────────

// FileRootsHandler GET /api/fileroots —— 五根白名单 + 可配置项。
// 严格只读：不探目录、不建目录、不读文件（不存在的根也如实返回）。
func (h *Handlers) FileRootsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"roots":  fileRoots(),
		"config": fileBrowserConfig(),
	})
}

// FileOpenHandler POST /api/fileroots/open {"root","path","mode"} —— 用默认应用打开
func (h *Handlers) FileOpenHandler(w http.ResponseWriter, r *http.Request) {
	h.fileBrowserAction(w, r, "open")
}

// FileRevealHandler POST /api/fileroots/reveal {"root","path"} —— 在访达中显示
func (h *Handlers) FileRevealHandler(w http.ResponseWriter, r *http.Request) {
	h.fileBrowserAction(w, r, "reveal")
}

// fileBrowserAction 是 open/reveal 的共同流程：解析 → 校验 → 交给系统 → 审计。
// 每个出口都审计（含失败），结果列 "ok" 或错误码。
func (h *Handlers) fileBrowserAction(w http.ResponseWriter, r *http.Request, action string) {
	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
		Mode string `json:"mode"` // file|dir：契约字段；后端以实际 stat 为准（不信前端声明）
	}
	if err := readJSONBody(r, &req); err != nil {
		h.auditUIAction(action, "", "", "INVALID_BODY")
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体解析失败: "+err.Error())
		return
	}
	abs, code := resolveFileActionTarget(action, req.Root, req.Path)
	if code != "" {
		h.auditUIAction(action, req.Root, req.Path, code)
		writeErrorCode(w, errorStatus(code), code, errorMessage(code, req.Root, req.Path))
		return
	}
	if err := h.runFileBrowserAction(action, abs); err != nil {
		h.auditUIAction(action, req.Root, req.Path, "OPEN_FAILED")
		writeErrorCode(w, http.StatusInternalServerError, "OPEN_FAILED", "打开失败: "+err.Error())
		return
	}
	h.auditUIAction(action, req.Root, req.Path, "ok")
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "abs": abs})
}

// resolveFileActionTarget 校验「根 + 相对路径」并给出可交给系统的绝对路径。
// 类型规则（设计 §4.4）：
//   - 目录一律放行；
//   - **open（用默认应用打开）**：文件须在 text_exts 内（ALLOW_ALL_TYPES=1 时放开为任意类型）——
//     它会让系统按关联程序执行/解析该文件，所以与「读入 UI」共用同一份清单（设计 §九 Q5）；
//   - **reveal（在访达中显示）**：对**任意类型**放行（清单外类型只允许 reveal，设计 §4.4 原文）——
//     它只是打开文件管理器并高亮，不执行也不解析文件，风险与"浏览目录"同级。
//
// 注意：类型闸门只影响「交给系统」这一步；路径本身仍必须过 safePathIn（防穿越）。
// 相对路径为空表示「根本身」（打开/显示这个根），这是白名单内的合法目标。
func resolveFileActionTarget(action, rootID, rel string) (string, string) {
	root, ok := findFileRoot(rootID)
	if !ok {
		return "", "INVALID_ROOT"
	}
	abs := filepath.Clean(root.Path)
	if rel != "" {
		a, code := safePathIn(rootID, rel)
		if code != "" {
			return "", code
		}
		abs = a
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", "NOT_FOUND"
	}
	if fi.IsDir() {
		return abs, ""
	}
	if action == "open" && !fileBrowserExtAllowed(abs) {
		return "", "NOT_ALLOWED"
	}
	return abs, ""
}

// runFileBrowserAction 真正把路径交给系统。
// macOS：open / open -R；Linux 与其他：xdg-open（reveal 退化为打开父目录，因为 xdg-open 没有 -R）。
// 子进程必须回收：用 cmd.Run()（内部 Start+Wait），Start 后不 Wait 会留僵尸。
func (h *Handlers) runFileBrowserAction(action, abs string) error {
	if h.FileBrowserRun != nil {
		return h.FileBrowserRun(action, abs) // 测试注入桩：避免测试真开 Finder 窗口
	}
	var name string
	var args []string
	switch {
	case runtime.GOOS == "darwin" && action == "reveal":
		name, args = "open", []string{"-R", abs}
	case runtime.GOOS == "darwin":
		name, args = "open", []string{abs}
	case action == "reveal":
		name, args = "xdg-open", []string{filepath.Dir(abs)}
	default:
		name, args = "xdg-open", []string{abs}
	}
	return exec.Command(name, args...).Run()
}

// docsParameterized 是 GET /api/docs 的参数化分支（带 root / path 查询参数时走这里）。
// 契约（设计 §4.2 ②）：
//
//	?root=<id>            → {"files":[...],"dirs":[...]}（相对该根）
//	?root=<id>&path=<rel> → {"path":rel,"content":str,"size":int,"root":id}（内容不截断、无大小上限）
func (h *Handlers) docsParameterized(w http.ResponseWriter, r *http.Request) {
	rootID := strings.TrimSpace(r.URL.Query().Get("root"))
	if rootID == "" {
		rootID = "docs" // 只给 path 不给 root ⇒ 等价于 docs 根
	}
	root, ok := findFileRoot(rootID)
	if !ok {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_ROOT", "未知根: "+rootID)
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" {
		files, dirs := listFileRoot(root)
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": files, "dirs": dirs})
		return
	}
	abs, code := safePathIn(rootID, rel)
	if code != "" {
		writeErrorCode(w, errorStatus(code), code, errorMessage(code, rootID, rel))
		return
	}
	fi, err := os.Stat(abs)
	if err != nil {
		writeErrorCode(w, http.StatusNotFound, "NOT_FOUND", "文件不存在: "+rel)
		return
	}
	if fi.IsDir() {
		writeErrorCode(w, http.StatusBadRequest, "NOT_ALLOWED", "目录不能按文件读取: "+rel)
		return
	}
	if !fileBrowserExtAllowed(abs) {
		writeErrorCode(w, http.StatusBadRequest, "NOT_ALLOWED", errorMessage("NOT_ALLOWED", rootID, rel))
		return
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "READ_FAILED", "读取失败: "+err.Error())
		return
	}
	// size 用实际读到的字节数：与磁盘一致（不截断、不设上限——设计 §九 Q3）
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    rel,
		"content": string(content),
		"size":    len(content),
		"root":    rootID,
	})
}

// errorStatus 把错误码映射到 HTTP 状态码（契约 §六：INVALID_* 400 / NOT_FOUND 404 / 执行失败 500）。
func errorStatus(code string) int {
	switch code {
	case "NOT_FOUND":
		return http.StatusNotFound
	case "READ_FAILED", "OPEN_FAILED":
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// errorMessage 生成错误码对应的人话（UI 侧按 code 本地化，未收录时原样回显这里）。
func errorMessage(code, rootID, rel string) string {
	switch code {
	case "INVALID_ROOT":
		return "未知根: " + rootID
	case "INVALID_PATH":
		return "非法路径: " + rel
	case "NOT_FOUND":
		return "路径不存在: " + rel
	case "NOT_ALLOWED":
		return "该类型不允许（仅目录与文本类型；ZERG_FILEBROWSER_ALLOW_ALL_TYPES=1 可放开）"
	default:
		return code
	}
}

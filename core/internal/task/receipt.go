// Package task —— 任务族的**契约件**（回执 14 组字段 ＋ 「留痕 ≠ 当前态 / 现读」两列并列）。
//
// receipt.go —— 子代理运行回执的**字段真源**。
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md
//
//	· §九.2 · §九.3「★ 细节补稿（回执必带 14 组字段 · 直接落 `task show --json`）」
//	· §二.8 DQ5（「回执 → 对话」的呈现契约：两列并列）
//	· §二.10（两条文案硬规矩 ＋ 教训二「运行留痕 ≠ 当前态」）
//
// 缺口真源：`GAP-20261014-61`（看门狗两把时钟 ⇒ ⑧ 时刻五件里的 `last_progress_at` / `last_heartbeat_at`）
//
// ★ 两条硬规矩（§二.10 逐字取向，本文件把它们落成**输出口径**）：
//
//	① **回执那一侧必带「为最近一次运行留痕 ≠ 当前态」**；
//	② **现读那一侧必带「现读」**。
//
//	✗ 不用回执值当当前态，✗ 也不用它掩盖现读值 —— 所以这里出的是**两列并列**，不是一列。
//
// ★ 教训二（今晚实证 · §二.10）：**运行留痕 ≠ 当前态** ——
// 回执印 `内核=rollback`，而实况在跑新件（`code_sha=ca565e927061` ＝ 目标）；
// 那个 `rollback` 是**最近一次内核运行的留痕**，✗ 不是该机当前安装态。
// ⇒ 对话/回执**只出过去时**，当前态**一律现读**。
//
// ★ 字段名尽量靠 **OTel GenAI 语义约定**（将来接 OpenTelemetry 不用改名）。
package task

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ── 两列并列的两条文案（**硬规矩**，逐字，✗ 不许改写成近义句）──────────────────────

const (
	// RetainedHeader 回执那一列的表头（§二.10 ①）。
	RetainedHeader = "为最近一次运行留痕 ≠ 当前态"
	// LiveHeader 现读那一列的表头（§二.10 ②）。
	LiveHeader = "现读"
)

// FailureReason 终态失败原因闭集（§九.3 ④）。
const (
	FailStalled       = "stalled"
	FailTimeout       = "timeout"
	FailMaxAttempts   = "max_attempts_exceeded"
	FailWorkerLost    = "worker_lost"
	FailSchemaInvalid = "schema_invalid"
	FailCancelled     = "cancelled"
	FailSteered       = "missed_steer"
	FailProviderError = "provider_error"
	FailContentFilter = "content_filter"
	FailUnknown       = "unknown"
)

// failureReasons 闭集（**判定式正向白名单**：✗ 不写 `if not v` 那种反向判据）。
var failureReasons = map[string]bool{
	FailStalled: true, FailTimeout: true, FailMaxAttempts: true, FailWorkerLost: true,
	FailSchemaInvalid: true, FailCancelled: true, FailSteered: true, FailProviderError: true,
	FailContentFilter: true, FailUnknown: true,
}

// finishReasonClosed ⑤ finish_reasons[] 的闭集（§九.3 ⑤）。
var finishReasonClosed = map[string]bool{
	"stop": true, "length": true, "error": true, "content_filter": true, "tool_calls": true,
}

// ── 出口信号（⑪ exit_code ＋ signal 必须**可辨 SIGTERM / SIGKILL**）─────────────────

const (
	SignalNone  = ""        // 无信号（正常退出码路径）
	SignalTERM  = "SIGTERM" // 优雅终止请求
	SignalKILL  = "SIGKILL" // 硬断（收尾机会已被剥夺）
	SignalOther = "OTHER"   // 其它信号（原样保留在 Receipt.Signal 里）
)

// SignalKind 把 `Receipt.Signal` 归到**可辨档**。
//
// 接受两种写法（真机两种都出现过）：信号名（`SIGTERM`）与信号号（`15` / `-15`）。
func SignalKind(signal string) string {
	s := strings.ToUpper(strings.TrimSpace(signal))
	s = strings.TrimPrefix(s, "-")
	switch s {
	case "", "0", "NONE", "EXIT":
		return SignalNone
	case "SIGTERM", "TERM", "15":
		return SignalTERM
	case "SIGKILL", "KILL", "9":
		return SignalKILL
	}
	if n, err := strconv.Atoi(s); err == nil {
		switch syscall.Signal(n) {
		case syscall.SIGTERM:
			return SignalTERM
		case syscall.SIGKILL:
			return SignalKILL
		}
	}
	return SignalOther
}

// ExitOutcome 把 `(exit_code, signal)` 读成**一句可辨的出口**。
//
// 为什么要分开读：**退出码与信号是两条不同的真相** —— 143（128+15）与“收到 SIGTERM”同义，
// 137（128+9）与“收到 SIGKILL”同义；而 0 只说明"它自己退的"，不说明它干完了。
// 把两种混成一句 ⇒ 复盘时分不清"我们优雅请它退"还是"我们硬断了它"。
func (r Receipt) ExitOutcome() string {
	switch SignalKind(r.Signal) {
	case SignalTERM:
		return fmt.Sprintf("信号终止：SIGTERM（优雅终止请求；exit_code=%d）", r.ExitCode)
	case SignalKILL:
		return fmt.Sprintf("信号终止：SIGKILL（**硬断**，收尾机会已被剥夺；exit_code=%d）", r.ExitCode)
	case SignalOther:
		return fmt.Sprintf("信号终止：%s（既非 SIGTERM 亦非 SIGKILL；exit_code=%d）", strings.TrimSpace(r.Signal), r.ExitCode)
	}
	if r.ExitCode == 0 {
		return "正常退出（exit_code=0）"
	}
	return fmt.Sprintf("非零退出码（exit_code=%d；无信号 ⇒ 它自己退的）", r.ExitCode)
}

// ── 回执本体：14 组字段 ────────────────────────────────────────────────────────────

// Receipt 一次子代理运行的**回执**（14 组字段 · 组号与设计 §九.3 逐条对齐）。
type Receipt struct {
	// ① 身份
	TaskID  string `json:"task_id"`
	RunID   string `json:"run_id"`
	Attempt int    `json:"attempt"`
	// ② 归属（★ caller **不得空** —— 「✗ 不能出现谁派的查不到」）
	Caller      string `json:"caller"`
	SubmittedAt string `json:"submitted_at"`
	// ③ 模型（★ `model` 与 `resp_model` **两个都给**：前者是我们请的、后者是回读到的实际模型）
	Model     string `json:"model"`
	RespModel string `json:"resp_model"`
	// ④ 终态（闭集）
	State         string `json:"state"`
	ExitReason    string `json:"exit_reason"`
	FailureReason string `json:"failure_reason"`
	// ⑤ finish_reasons[]（闭集：stop / length / error / content_filter）
	FinishReasons []string `json:"finish_reasons"`
	// ⑥ 用量四件（`input_tokens` 应含缓存 token）
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	// ⑦ 计数
	APICalls       int `json:"api_calls"`
	Turns          int `json:"turns"`
	AttemptsMade   int `json:"attempts_made"`
	StalledCounter int `json:"stalled_counter"`
	// ⑧ 时刻五件（含**两把锁**：liveness / progress · 看门狗判卡只看后者）
	CreatedAt       string `json:"created_at"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	LastProgressAt  string `json:"last_progress_at"`
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	// ⑨ 时长切分
	DurationMillis int64 `json:"duration_ms"`
	QueueMillis    int64 `json:"queue_ms"`
	PrefillMillis  int64 `json:"prefill_ms"`
	TTFTMillis     int64 `json:"ttft_ms"`
	// ⑩ 内容
	ResultHash  string `json:"result_hash"`
	SchemaValid bool   `json:"schema_valid"`
	Truncated   bool   `json:"truncated"`
	// ⑪ 出口（exit_code ＋ signal 可辨 SIGTERM / SIGKILL）
	ExitCode int    `json:"exit_code"`
	Signal   string `json:"signal"`
	// ⑫ 指针（★ **现场只留指针**：工作树 / diff 摘要 / 日志路径 —— ✗ 不把过程搬进回执）
	Worktree string `json:"worktree"`
	DiffStat string `json:"diff_stat"`
	LogPath  string `json:"log_path"`
	// ⑬ 审计锚（链要能对：`prev_hash` → `hash`）
	AuditSeq  int64  `json:"audit_seq"`
	AuditHash string `json:"audit_hash"`
	PrevHash  string `json:"prev_hash"`
	// ⑭ 幂等
	IdempotencyKey string `json:"idempotency_key"`
	RequestHash    string `json:"request_hash"`
}

// receiptGroups 14 组的**字段真源**（组号 → 字段名）。★ 它就是"14 组齐不齐"的判据真身。
var receiptGroups = []struct {
	No     int
	Name   string
	Fields []string
}{
	{1, "身份", []string{"task_id", "run_id", "attempt"}},
	{2, "归属", []string{"caller", "submitted_at"}},
	{3, "模型（两个都给）", []string{"model", "resp_model"}},
	{4, "终态（闭集）", []string{"state", "exit_reason", "failure_reason"}},
	{5, "finish_reasons", []string{"finish_reasons"}},
	{6, "用量四件", []string{"input_tokens", "output_tokens", "reasoning_output_tokens", "cache_read_input_tokens"}},
	{7, "计数", []string{"api_calls", "turns", "attempts_made", "stalled_counter"}},
	{8, "时刻五件（含两把锁）", []string{"created_at", "started_at", "finished_at", "last_progress_at", "last_heartbeat_at"}},
	{9, "时长切分", []string{"duration_ms", "queue_ms", "prefill_ms", "ttft_ms"}},
	{10, "内容", []string{"result_hash", "schema_valid", "truncated"}},
	{11, "出口（exit_code ＋ signal 可辨）", []string{"exit_code", "signal"}},
	{12, "指针（现场只留指针）", []string{"worktree", "diff_stat", "log_path"}},
	{13, "审计锚", []string{"audit_seq", "audit_hash", "prev_hash"}},
	{14, "幂等", []string{"idempotency_key", "request_hash"}},
}

// ReceiptGroupCount 14（判据读数用：组数不许悄悄少一组）。
const ReceiptGroupCount = 14

// signalLooksValid 判定 signal 是不是**能归档的形态**：信号名（`SIGKILL` / `KILL`）或信号号（`9` / `-15`）。
// ✗ 自由文本一律不收 —— 形态不收就会把"是不是 SIGKILL"这个问题变成读不出答案的。
func signalLooksValid(s string) bool {
	t := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "-")
	if t == "" {
		return false
	}
	if _, err := strconv.Atoi(t); err == nil {
		return true
	}
	if !strings.HasPrefix(t, "SIG") {
		return false
	}
	for _, r := range t[3:] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return len(t) > 3
}

// ReceiptFieldNames 把 14 组摊平成一串字段名（`--json <字段>` 面的真源；命令行字段表照它抄）。
func ReceiptFieldNames() []string {
	out := make([]string, 0, 40)
	for _, g := range receiptGroups {
		out = append(out, g.Fields...)
	}
	return out
}

// ReceiptGroupNames 返回 (组号, 组名) 逐条（人面/文档用）。
func ReceiptGroupNames() []string {
	out := make([]string, 0, ReceiptGroupCount)
	for _, g := range receiptGroups {
		out = append(out, fmt.Sprintf("%d · %s", g.No, g.Name))
	}
	return out
}

// ── 判据（可机检 · 正向白名单写法）────────────────────────────────────────────────

// Validate 逐条核 14 组里的**必填 / 成对 / 闭集**，返回缺失或不合规项（每条一个字段名）。
//
// 判的正是不许空、不许漏的那几条：
//
//	② `caller` **不得空**（「✗ 不能出现谁派的查不到」· §十二 Q9）
//	③ `model` 与 `resp_model` **两个都给**（一个是我们请的、一个是回读到的实际模型）
//	④ `failure_reason` 若给就必须在**闭集**里；⑤ `finish_reasons` 逐条在闭集里
//	⑧ 两把锁都在（`last_progress_at` / `last_heartbeat_at`）—— 看门狗判卡只看后者那吧
//	   （两把都出才分得清"算得慢"与"真卡死"）
//	⑪ 出口**可辨**：有 exit_code 或无信号正常退出；信号必须能归到可辨档
//	⑫ 现场**只留指针**：三件都在，且 `diff_stat` 是**摘要**（✗ 不许把整篇 diff 塞进回执）
//	⑬ 审计锚三件齐（`audit_seq` / `audit_hash` / `prev_hash`）
//	⑭ 幂等两件齐（`idempotency_key` / `request_hash`）
func (r Receipt) Validate() []string {
	var bad []string
	add := func(f string) { bad = append(bad, f) }

	// ① 身份
	if strings.TrimSpace(r.TaskID) == "" {
		add("task_id")
	}
	if strings.TrimSpace(r.RunID) == "" {
		add("run_id")
	}
	// ② 归属：★ caller 不得空
	if strings.TrimSpace(r.Caller) == "" {
		add("caller")
	}
	if strings.TrimSpace(r.SubmittedAt) == "" {
		add("submitted_at")
	}
	// ③ 模型：两个都给
	if strings.TrimSpace(r.Model) == "" {
		add("model")
	}
	if strings.TrimSpace(r.RespModel) == "" {
		add("resp_model")
	}
	// ④ 终态闭集
	if strings.TrimSpace(r.State) == "" {
		add("state")
	}
	if r.FailureReason != "" && !failureReasons[r.FailureReason] {
		add("failure_reason")
	}
	// ⑤ finish_reasons 闭集
	for _, fr := range r.FinishReasons {
		if !finishReasonClosed[strings.ToLower(strings.TrimSpace(fr))] {
			add("finish_reasons")
			break
		}
	}
	// ⑧ 两把锁（liveness / progress）都要在
	if strings.TrimSpace(r.LastProgressAt) == "" {
		add("last_progress_at")
	}
	if strings.TrimSpace(r.LastHeartbeatAt) == "" {
		add("last_heartbeat_at")
	}
	if r.CreatedAt == "" {
		add("created_at")
	}
	// ⑪ 出口可辨：信号必须是**能归档的形态**（信号名 `SIG*` 或信号号）——
	// ✗ 不许把一段自由文本塞进 signal（那样"是不是 SIGKILL"就判不出来了）。
	if s := strings.TrimSpace(r.Signal); s != "" && !signalLooksValid(s) {
		add("signal")
	}
	// ⑫ 现场只留指针
	if strings.TrimSpace(r.Worktree) == "" {
		add("worktree")
	}
	if strings.TrimSpace(r.DiffStat) == "" {
		add("diff_stat")
	}
	if strings.TrimSpace(r.LogPath) == "" {
		add("log_path")
	}
	// ★ 指针的判据：`diff_stat` 只能是**一行摘要**（把整篇 diff 塞回执 = 违反"现场只留指针"）
	if strings.ContainsAny(r.DiffStat, "\n\r") {
		add("diff_stat")
	}
	// ⑬ 审计锚
	if r.AuditSeq <= 0 {
		add("audit_seq")
	}
	if strings.TrimSpace(r.AuditHash) == "" {
		add("audit_hash")
	}
	if strings.TrimSpace(r.PrevHash) == "" {
		add("prev_hash")
	}
	// ⑭ 幂等
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		add("idempotency_key")
	}
	if strings.TrimSpace(r.RequestHash) == "" {
		add("request_hash")
	}
	return bad
}

// ToRow 摊成字符串行（`--json <字段>` 面与两列渲染共用同一份取值口径 ⇒ 天然不漂）。
func (r Receipt) ToRow() map[string]string {
	return map[string]string{
		"task_id": r.TaskID, "run_id": r.RunID, "attempt": strconv.Itoa(r.Attempt),
		"caller": r.Caller, "submitted_at": r.SubmittedAt,
		"model": r.Model, "resp_model": r.RespModel,
		"state": r.State, "exit_reason": r.ExitReason, "failure_reason": r.FailureReason,
		"finish_reasons":          strings.Join(r.FinishReasons, ","),
		"input_tokens":            strconv.FormatInt(r.InputTokens, 10),
		"output_tokens":           strconv.FormatInt(r.OutputTokens, 10),
		"reasoning_output_tokens": strconv.FormatInt(r.ReasoningOutputTokens, 10),
		"cache_read_input_tokens": strconv.FormatInt(r.CacheReadInputTokens, 10),
		"api_calls":               strconv.Itoa(r.APICalls),
		"turns":                   strconv.Itoa(r.Turns),
		"attempts_made":           strconv.Itoa(r.AttemptsMade),
		"stalled_counter":         strconv.Itoa(r.StalledCounter),
		"created_at":              r.CreatedAt, "started_at": r.StartedAt, "finished_at": r.FinishedAt,
		"last_progress_at": r.LastProgressAt, "last_heartbeat_at": r.LastHeartbeatAt,
		"duration_ms":  strconv.FormatInt(r.DurationMillis, 10),
		"queue_ms":     strconv.FormatInt(r.QueueMillis, 10),
		"prefill_ms":   strconv.FormatInt(r.PrefillMillis, 10),
		"ttft_ms":      strconv.FormatInt(r.TTFTMillis, 10),
		"result_hash":  r.ResultHash,
		"schema_valid": strconv.FormatBool(r.SchemaValid),
		"truncated":    strconv.FormatBool(r.Truncated),
		"exit_code":    strconv.Itoa(r.ExitCode),
		"signal":       SignalKind(r.Signal),
		"worktree":     r.Worktree, "diff_stat": r.DiffStat, "log_path": r.LogPath,
		"audit_seq":  strconv.FormatInt(r.AuditSeq, 10),
		"audit_hash": r.AuditHash, "prev_hash": r.PrevHash,
		"idempotency_key": r.IdempotencyKey, "request_hash": r.RequestHash,
	}
}

// ── 两列并列：「为最近一次运行留痕 ≠ 当前态」⟷「现读」──────────────────────────────

// Wire 「回执 → 对话/人面」的**呈现契约**（§二.8 DQ5）：
// **两列并列**，回执侧标「为最近一次运行留痕 ≠ 当前态」、现读侧标「现读」。
//
// ★ ✗ 不许只出一列（§二.8 DQ5：「两个都出」）：
// 只出回执 ⇒ 把过去时当现状（今晚实证的那个错：`内核=rollback` 而实况在跑新件）；
// 只出现读 ⇒ 丢掉"这一次到底交了什么"的留痕（审计就断了）。
type Wire struct {
	// Retained 回执侧：**为最近一次运行留痕 ≠ 当前态**。
	Retained Receipt
	// Live 现读侧：**现读**（一律现取，✗ 不拿回执值兜）。
	Live Receipt
}

// Render 两列并列写到 out（默认按 14 组的字段顺序逐行出；`fields` 非空 ⇒ 只出这几格）。
func (w Wire) Render(out io.Writer, fields []string) {
	fs := fields
	if len(fs) == 0 {
		fs = ReceiptFieldNames()
	}
	ret, live := w.Retained.ToRow(), w.Live.ToRow()
	width := 28
	for _, f := range fs {
		if n := len([]rune(f)); n > width {
			width = n
		}
	}
	fmt.Fprintf(out, "%-*s | %s | %s\n", width, "字段", RetainedHeader, LiveHeader)
	fmt.Fprintf(out, "%s-+-%s-+-%s\n", strings.Repeat("-", width),
		strings.Repeat("-", len(RetainedHeader)), strings.Repeat("-", len(LiveHeader)))
	for _, f := range fs {
		rv, lv := orDash(ret[f]), orDash(live[f])
		mark := ""
		if rv != lv {
			mark = " ⟵ 不一致" // 这一列才是"该看现读"的信号
		}
		fmt.Fprintf(out, "%-*s | %-*s | %s%s\n", width, f, len(RetainedHeader), rv, lv, mark)
	}
}

// DiffFields 逐字对比两列，返回**不一致**的字段名（升序）。
//
// ★ 它是「留痕 ≠ 当前态」这条教训的**取证件**：非空 ⇒ 回执值与现读值真的不同 ⇒
// 一律以现读为准，且**不许**用回执值掩盖现读值（§二.10 逐字）。
func (w Wire) DiffFields(fields []string) []string {
	fs := fields
	if len(fs) == 0 {
		fs = ReceiptFieldNames()
	}
	ret, live := w.Retained.ToRow(), w.Live.ToRow()
	var diff []string
	for _, f := range fs {
		if orDash(ret[f]) != orDash(live[f]) {
			diff = append(diff, f)
		}
	}
	sort.Strings(diff)
	return diff
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// NowStamp 统一的时刻写法（RFC3339 秒级；回执/现读两侧同一口径，便于逐字比）。
func NowStamp() string { return time.Now().Format(time.RFC3339) }

// Package replay —— 工具执行的**录播层**（T4.1；设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 E 组）。
//
// ── 为什么必须在「工具层」录播（E2 ★★★）──
//
// 网络侧拦截（HTTP RoundTripper / MITM）只能覆盖**走 HTTP 的东西**。本机文件写、子进程、
// IPC、DB 这些副作用它一条也拦不住 ⇒ 只拦网络的录播会让「回放」悄悄真写盘、真起进程，
// 而回放的**全部价值**恰恰是"不真发还能重跑一遍"。所以录制/回放必须落在**工具调用的分发口**
// （Dispatcher）：所有工具执行统一走它，HTTP 拦截降级为**双保险**。（本包只做分发口本体，
// 把 agent 的工具执行路径整体接到本包上 = 后续批；见包尾"未接项"。）
//
// ── 本包提供什么（每条都对应设计稿的一条漏点）──
//
//	Record / Trace       录播事实与 JSONL 载体：追加写 + 顺序读 + **消费游标**（E6：进程重启续读，
//	                     不从头重放）+ trace 损坏/截断**明确报错**（绝不静默跳过）
//	Normalizer / Call    **参数规范化与请求指纹**（E3）：键排序 / 数字统一 / 路径绝对化 / 去噪字段
//	                     ⇒ ArgsCanonical + ArgsDigest + Fingerprint。**默认不使用原始字节哈希**——
//	                     一个 ts 字段就能把命中率打到 0
//	Player / MissError   **严格语义**（E1）：未命中 ⇒ 报错，错误里带 **fingerprint 与最近邻候选**；
//	                     **禁止静默放行**，也**禁止"顺手新录一条"**
//	EffectKey/Ledger     **效果级幂等键与账本**（E4）：effect_key = sha256(工具名 + ArgsCanonical +
//	                     效果范围)；同一 key 第二次出现 ⇒ 返回"已发生"，不重放；一次调用多效果时
//	                     按 (call_id, effect_index) **逐条**判重；账本可序列化
//	Precondition         **前置状态校验**（E5）：PreconditionHash / ObservedAfterHash 不符 ⇒
//	                     报"状态不匹配"，而不是默默回放
//	Dispatcher           录制 / 回放 / 真发 三态的唯一分发口
//
// ── 三条写死的纪律（本包的"为什么这么做"都收敛到这里）──
//
//  1. **未命中就是错**。VCR / go-vcr / WireMock 三家在匹配失败上的共同选择都是"报错"；
//     静默放行会让一次"回放"退化成"真跑"，而报告上仍然写着"回放通过"——这是最坏的一种假绿。
//  2. **回放路径只读**。回放期不写真发、不写 trace、不写效果账本（账本只**读**来报告"这个效果
//     此前已发生过"）。只读这一点由用例③用真文件 + 真计数钉住。
//  3. **不猜**。"这次是不是回放"、"这个参数有没有副作用"、"哪个字段算噪声"——一律由调用方**显式声明**；
//     本包不按名字/形态/时间戳去推断，猜错的方向都是**危险的那一侧**。
//
// ── 冻结与边界（如实）──
//
//	· 时间：只用来量 DurationMS（且可注入 Dispatcher.Now）；时间**不参与**匹配、不参与指纹。
//	· 绝不读取环境变量 / 网络 / 时钟来做决定（唯一例外：BaseDir 为空时用 os.Getwd() 解析相对路径）。
//	· 零内部依赖：只 import 标准库，不 import chat / gateway / agent（叶包纪律，见 T5.1 同款）。
//	· 未接项（后续批）：① 把 agent 的工具执行路径整体切到本包的 Dispatcher；② 落盘层（压缩/轮转）；
//	  ③ schema_version + tool_fingerprint + 引擎指纹的失效机制（T4.6）；④ 保真度三指标与二分定位（T4.4）。
package replay

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SchemaVersion —— trace 行格式版本（E11 的版本字段；工具/引擎指纹归 T4.6）。
const SchemaVersion = 1

// 哨兵错误：调用方用 errors.Is 分流（不要靠字符串匹配）。
var (
	// ErrReplayMiss —— 回放未命中录播（严格语义）。
	ErrReplayMiss = errors.New("replay: 未命中录播")
	// ErrStateMismatch —— 前置状态与录制时不符（E5）。
	ErrStateMismatch = errors.New("replay: 状态不匹配")
	// ErrTraceCorrupt —— trace 行损坏/截断/版本不认识（**不得静默跳过**）。
	ErrTraceCorrupt = errors.New("replay: trace 损坏")
	// ErrBadCursor —— 消费游标不可用（trace 被重录/覆盖）。
	ErrBadCursor = errors.New("replay: 消费游标不可用")
	// ErrNormalize —— 参数无法规范化（含不支持的类型：宁可报错也不静默丢字段）。
	ErrNormalize = errors.New("replay: 参数无法规范化")
)

// Digest —— 字节的 sha256 指纹（带 "sha256:" 前缀，便于人读与日志比对）。
func Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Fingerprint —— 一次工具调用的**请求指纹**（E3）：工具名 ‖ 0x1f ‖ 规范化参数。
//
// 分隔符用 0x1f（JSON 文本里不会出现的控制字符；JSON 规范要求控制字符必须转义），
// 故 "工具名+参数" 的拼接不存在歧义。
func Fingerprint(tool string, canonical []byte) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0x1f})
	h.Write(canonical)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// ── 录播事实 ────────────────────────────────────────────────────────────────

// EffectRange —— 一次调用里的一条**效果**（E4：一次调用多效果 ⇒ (call_id, effect_index) 逐条判重）。
type EffectRange struct {
	Index int    `json:"index"` // 0 起：本调用内的效果序号
	Scope string `json:"scope"` // 效果范围（如 "fs:/tmp/x.txt"、"proc:bash:git commit"）
	Key   string `json:"key"`   // effect_key（EffectKey 算出）
}

// Record —— 一次工具调用的录播事实 = trace 里的一行。
//
//	· ResultBody 是**原样字节**：JSON 里以 base64 承载（encoding/json 对 []byte 的默认行为）
//	  ⇒ 逐字节往返一致，且不会有非法 UTF-8 把整行 JSON 写坏的问题。
//	· 异常路径（工具报错/超时/部分成功）**照样入 trace**（E10）：ErrText 与 ResultBody 都留。
//	  "只录成功"的 cassette 是常见坑——录制时失败、回放时静默成功，等于换了事实。
//	· 两个 digest 是**可复算**的：读侧拿 ArgsCanonical / ResultBody 重算即可校验（见 validateRecord）。
//	· 指纹（ArgsDigest/Fingerprint）与正文之外的字段（DurationMS 等）**不参与匹配**。
type Record struct {
	V                 int             `json:"v"` // SchemaVersion（0 = 老行，按 1 读）
	Seq               int             `json:"seq"`
	CallID            string          `json:"call_id,omitempty"` // 同一次模型调用产生的多条 Record 共用一个 id
	Tool              string          `json:"tool"`
	ArgsCanonical     json.RawMessage `json:"args_canonical"`
	ArgsDigest        string          `json:"args_digest"`
	ResultBody        []byte          `json:"result_body,omitempty"`
	ResultDigest      string          `json:"result_digest"`
	ErrText           string          `json:"err_text,omitempty"`
	DurationMS        int64           `json:"duration_ms"`
	PreconditionHash  string          `json:"precondition_hash,omitempty"`   // E5：执行**前**的状态指纹
	ObservedAfterHash string          `json:"observed_after_hash,omitempty"` // E5：执行**后**的状态指纹
	Effects           []EffectRange   `json:"effects,omitempty"`
}

// ToolName —— 该录播事实的工具名。
func (r Record) ToolName() string { return r.Tool }

// Fingerprint —— 该录播事实的请求指纹（读侧用 ArgsDigest 复算，两处必须一致）。
func (r Record) Fingerprint() string { return Fingerprint(r.Tool, r.ArgsCanonical) }

// ── trace 载体：JSONL 追加写 + 顺序读 + 消费游标 ─────────────────────────────

// Trace —— 追加写的 JSONL 载体（E6：JSONL + 后续加压缩，优于单 YAML）。
//
// 写语义：一行一条、单次 Write、**写完即认为已录**。崩溃语义如实说：最后一行可能只写了一半
// ⇒ 读侧会**报错**（而不是跳过），因为"半行"与"没有这一条"是两件事；这属于 F6 的
// at-least-once 边界（跨崩溃），不是 exactly-once。
type Trace struct {
	mu   sync.Mutex
	path string
	f    *os.File
	n    int // 已写入条数
}

// Create —— 新建（截断）一条 trace。录制开始前调用一次。
func Create(path string) (*Trace, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("replay: 建目录 %s：%w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("replay: 新建 trace %s：%w", path, err)
	}
	return &Trace{path: path, f: f}, nil
}

// OpenAppend —— 打开既有 trace 追加。
//
// 打开时**严格通读一遍**（既拿到下一个 Seq，也顺手校验既有内容）：内容坏了就报错，
// 而不是把新行续在坏文件后面 —— 后者会让"坏掉的那一段"永久留在读数里。
func OpenAppend(path string) (*Trace, error) {
	r, err := OpenReader(path, 0)
	if err != nil {
		return nil, err
	}
	n, prev := 0, 0
	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.Close()
			return nil, err
		}
		n, prev = n+1, rec.Seq
	}
	if prev != n {
		r.Close()
		return nil, fmt.Errorf("replay: %s：末条 seq=%d 与条数 %d 不一致（拒绝续写）", path, prev, n)
	}
	if err := r.Close(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("replay: 打开 trace %s：%w", path, err)
	}
	return &Trace{path: path, f: f, n: n}, nil
}

// Path —— trace 文件路径。
func (t *Trace) Path() string { return t.path }

// Len —— 已写入条数。
func (t *Trace) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n
}

// NextSeq —— 下一条将被写入的序号（1 起）。
func (t *Trace) NextSeq() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n + 1
}

// Append —— 追加一条事实，Seq 由 trace 赋值（1 起、单调 +1）⇒ 同一 trace 内序号唯一。
func (t *Trace) Append(rec Record) (Record, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return Record{}, errors.New("replay: trace 已关闭")
	}
	if strings.TrimSpace(rec.Tool) == "" {
		// 没有工具名的录播事实永远匹配不上任何调用 ⇒ 写进去只是垃圾（且会被读侧判为损坏）
		return Record{}, fmt.Errorf("%w: 工具名为空，拒绝写入", ErrNormalize)
	}
	if len(rec.ArgsCanonical) == 0 {
		return Record{}, fmt.Errorf("%w: args_canonical 为空，拒绝写入", ErrNormalize)
	}
	if rec.ArgsDigest == "" {
		rec.ArgsDigest = Digest(rec.ArgsCanonical)
	}
	if rec.ResultDigest == "" {
		rec.ResultDigest = Digest(rec.ResultBody)
	}
	rec.V = SchemaVersion
	rec.Seq = t.n + 1
	line, err := json.Marshal(rec)
	if err != nil {
		return Record{}, fmt.Errorf("replay: 序列化录播事实：%w", err)
	}
	if _, err := t.f.Write(append(line, '\n')); err != nil {
		return Record{}, fmt.Errorf("replay: 写 trace %s：%w", t.path, err)
	}
	t.n++
	return rec, nil
}

// Sync —— 把已写内容刷到盘（耐久级别由调用方决定；见 F5/F6 的边界）。
func (t *Trace) Sync() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return nil
	}
	return t.f.Sync()
}

// Close —— 关闭。
func (t *Trace) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return nil
	}
	err := t.f.Close()
	t.f = nil
	return err
}

// ── 顺序读 + 消费游标 ───────────────────────────────────────────────────────

// Cursor —— **消费游标**（E6）：已消费条数 + 已消费前缀的链式指纹。
//
// 为什么要前缀指纹：子端重启后续读，若期间 trace 被重录/覆盖，同一个"N"指向的就是**另一批事实**
// ⇒ 此时必须报错（ErrBadCursor），而不是拿新事实当旧进度接着放。
type Cursor struct {
	TracePath    string `json:"trace_path"`
	Consumed     int    `json:"consumed"`
	PrefixDigest string `json:"prefix_digest,omitempty"`
}

// Reader —— trace 的顺序读（严格：任何异常都报错，绝不静默跳过）。
type Reader struct {
	path      string
	f         *os.File
	br        *bufio.Reader
	line      int    // 已读行数（含被 skip 掉的行）
	consumed  int    // 已**返回给调用方**的条数
	prevSeq   int    // 上一条的 Seq（校验单调 +1）
	chain     string // 已消费前缀的链式指纹
	tolerateT bool   // 显式声明的"容忍末行截断"（默认 false —— 见 readRecord 的注释）
}

// OpenReader —— 打开 trace，从第 from 条之后继续读（from = 已消费条数，0 起）。
//
// **跳过的前缀照样逐行校验**（解析 + digest 复算 + seq 单调）：跳过的段里塞了坏行，
// 恰恰是"从头重放才会发现"的那类问题 ⇒ 不能因为"我不读它"就放过。
func OpenReader(path string, from int) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("replay: 打开 trace %s：%w", path, err)
	}
	r := &Reader{path: path, f: f, br: bufio.NewReaderSize(f, 1<<20)}
	if from < 0 {
		r.Close()
		return nil, fmt.Errorf("%w: 游标为负（%d）", ErrBadCursor, from)
	}
	for i := 0; i < from; i++ {
		if _, err := r.advance(); err != nil {
			r.Close()
			if err == io.EOF {
				return nil, fmt.Errorf("%w: 游标 %d 超出 trace 末尾（%s）", ErrBadCursor, from, path)
			}
			return nil, err
		}
	}
	r.consumed = from
	return r, nil
}

// OpenReaderWithCursor —— 带游标续读：并校验**已消费前缀的指纹**与游标记录的一致。
func OpenReaderWithCursor(path string, c Cursor) (*Reader, error) {
	r, err := OpenReader(path, c.Consumed)
	if err != nil {
		return nil, err
	}
	if c.PrefixDigest != "" && r.chain != c.PrefixDigest {
		r.Close()
		return nil, fmt.Errorf("%w: 前缀指纹不符（游标 %s，实际 %s）——trace 已被重录/覆盖，拒绝错位续读",
			ErrBadCursor, c.PrefixDigest, r.chain)
	}
	return r, nil
}

// TolerateTruncatedTail —— **显式**声明"容忍末行缺换行"（只对**最后一行**生效，且仍要求它是合法 JSON
// 且 seq 正确）。默认关闭：崩溃留下的半行会让整条 trace 读不出来，这是刻意的 —— 半行=可疑，
// 可疑就必须让人看见，而不是替人决定"这半行不算数"。
func (r *Reader) TolerateTruncatedTail(v bool) { r.tolerateT = v }

// Next —— 下一条；读到末尾返回 io.EOF；损坏返回 ErrTraceCorrupt。
func (r *Reader) Next() (Record, error) {
	rec, err := r.advance()
	if err != nil {
		return Record{}, err
	}
	r.consumed++
	return rec, nil
}

// Consumed —— 已消费条数（= 可持久化的游标位）。
func (r *Reader) Consumed() int { return r.consumed }

// Cursor —— 当前游标（含已消费前缀的链式指纹）。
func (r *Reader) Cursor() Cursor {
	return Cursor{TracePath: r.path, Consumed: r.consumed, PrefixDigest: r.chain}
}

// Close —— 关闭。
func (r *Reader) Close() error {
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

// advance —— 读一行并校验（不推进消费条数）。
func (r *Reader) advance() (Record, error) {
	lineNo := r.line + 1
	b, err := r.br.ReadBytes('\n')
	truncated := false
	switch {
	case err == io.EOF:
		if len(b) == 0 {
			return Record{}, io.EOF
		}
		truncated = true // 末行没有换行符
	case err != nil:
		return Record{}, fmt.Errorf("replay: 读 trace %s 第 %d 行：%w", r.path, lineNo, err)
	}
	if truncated && !r.tolerateT {
		return Record{}, r.corrupt(lineNo, "末行没有换行符（写入被中断或文件被截断）；若确要接受，请显式 TolerateTruncatedTail(true)", nil)
	}
	raw := bytes.TrimSuffix(b, []byte("\n"))
	if len(bytes.TrimSpace(raw)) == 0 {
		return Record{}, r.corrupt(lineNo, "空行（不得静默跳过）", nil)
	}
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Record{}, r.corrupt(lineNo, "不是合法 JSON", err)
	}
	if err := validateRecord(rec, r.prevSeq, lineNo); err != nil {
		return Record{}, err
	}
	r.line, r.prevSeq = lineNo, rec.Seq
	sum := sha256.Sum256(append([]byte(r.chain), raw...))
	r.chain = "sha256:" + hex.EncodeToString(sum[:])
	return rec, nil
}

func (r *Reader) corrupt(line int, why string, cause error) error {
	e := fmt.Errorf("%w: %s 第 %d 行：%s", ErrTraceCorrupt, r.path, line, why)
	if cause != nil {
		return fmt.Errorf("%w（%v）", e, cause)
	}
	return e
}

// validateRecord —— 读侧的**可复算**校验：digest 必须对得上正文，seq 必须 +1，版本必须认识。
func validateRecord(rec Record, prevSeq, line int) error {
	if rec.V != 0 && rec.V != SchemaVersion {
		return fmt.Errorf("%w: 第 %d 行 v=%d 高于本包已知 v=%d（拒绝猜读）", ErrTraceCorrupt, line, rec.V, SchemaVersion)
	}
	if rec.Seq != prevSeq+1 {
		return fmt.Errorf("%w: 第 %d 行 seq=%d，应为 %d（序号断层 = 中间少了事实，不得静默跳过）",
			ErrTraceCorrupt, line, rec.Seq, prevSeq+1)
	}
	if strings.TrimSpace(rec.Tool) == "" {
		return fmt.Errorf("%w: 第 %d 行工具名为空", ErrTraceCorrupt, line)
	}
	if len(rec.ArgsCanonical) == 0 {
		return fmt.Errorf("%w: 第 %d 行 args_canonical 为空", ErrTraceCorrupt, line)
	}
	if !json.Valid(rec.ArgsCanonical) {
		return fmt.Errorf("%w: 第 %d 行 args_canonical 不是合法 JSON", ErrTraceCorrupt, line)
	}
	if got := Digest(rec.ArgsCanonical); got != rec.ArgsDigest {
		return fmt.Errorf("%w: 第 %d 行 args_digest 复算不符（行内 %s，实际 %s）", ErrTraceCorrupt, line, rec.ArgsDigest, got)
	}
	switch {
	case rec.ResultDigest == "" && len(rec.ResultBody) > 0:
		return fmt.Errorf("%w: 第 %d 行有正文却没有 result_digest", ErrTraceCorrupt, line)
	case rec.ResultDigest != "" && rec.ResultDigest != Digest(rec.ResultBody):
		return fmt.Errorf("%w: 第 %d 行 result_digest 复算不符（行内 %s，实际 %s）",
			ErrTraceCorrupt, line, rec.ResultDigest, Digest(rec.ResultBody))
	}
	for i, ef := range rec.Effects {
		if ef.Key == "" || ef.Scope == "" {
			return fmt.Errorf("%w: 第 %d 行第 %d 条效果缺少 key/scope", ErrTraceCorrupt, line, i)
		}
		if ef.Index != i {
			return fmt.Errorf("%w: 第 %d 行第 %d 条效果的 effect_index=%d（应为 %d）", ErrTraceCorrupt, line, i, ef.Index, i)
		}
	}
	return nil
}

// LoadCursor —— 读游标文件。第二个返回值表示"文件是否存在"（不存在 ≠ 错误：从未读过）。
func LoadCursor(path string) (Cursor, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Cursor{}, false, nil
	}
	if err != nil {
		return Cursor{}, false, fmt.Errorf("replay: 读游标 %s：%w", path, err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, false, fmt.Errorf("%w: 游标文件 %s 损坏：%v", ErrBadCursor, path, err)
	}
	if c.Consumed < 0 {
		return Cursor{}, false, fmt.Errorf("%w: 游标为负（%d）", ErrBadCursor, c.Consumed)
	}
	return c, true, nil
}

// SaveCursor —— 原子写游标（临时文件 + rename）：半写的游标比没有游标更危险。
func SaveCursor(path string, c Cursor) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("replay: 序列化游标：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("replay: 建目录 %s：%w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("replay: 写游标 %s：%w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replay: 落游标 %s：%w", path, err)
	}
	return nil
}

// match.go —— 参数规范化 + 请求指纹 + 严格匹配（E1/E3）。
//
// ── 为什么不能直接 hash 原始字节（E3 ★★★）──
//
// 同一个逻辑调用在不同时刻的 JSON 几乎从不逐字节相同：键序可能变、数字可能写成 1 / 1.0、
// 路径可能写成 ./a/../a.txt、还会带上 request_id / ts 这类每次都不一样的噪声字段。
// 直接 hash 原始字节 ⇒ 命中率崩到 0，然后大家就会去"放宽匹配"，最后连"匹配"都没了。
// 正确的做法是**先规范化再指纹**：MatchOn 字段集 + 键排序 + 数字统一 + 路径绝对化 + 去噪字段。
//
// ── 严格语义（E1 ★★★）──
//
// 未命中 ⇒ 报 *MissError：带 fingerprint + **最近邻候选**（同工具、按字段相同比例排序、
// 指出差在哪几个字段）。既**不静默放行**（那是换事实），也不"顺手新录一条"（那是换事实且更难查）。
package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DefaultNoiseFields —— 默认**去噪**字段（点路径）。
//
// 只放"确定与语义无关"的：时间戳、请求/追踪 id、随机串、纯耗时读数。
// **不确定的一律不默认丢** —— 丢错一个字段的后果是"回放期命中了一条**不是这次**的录播"，
// 比"未命中"危险得多（未命中会立刻报错，错命中会顺着错的结果往下跑）。
// 想彻底关掉去噪：显式给 `Drop: []string{}`（非 nil 空切片 ≠ nil）。
var DefaultNoiseFields = []string{
	"ts", "timestamp", "time_ms", "created_at", "request_id", "req_id", "nonce",
	"call_id", "trace_id", "span_id", "duration_ms", "latency_ms",
}

// DefaultPathFields —— 默认**需要绝对化**的字段（点路径；按**路径末段**匹配，故 `opts.path` 也算）。
//
// 路径绝对化能吃掉两类假未命中：相对/绝对写法不同（`a.txt` vs `/w/a.txt`）、以及
// `./a/../a.txt` 这类没 Clean 的写法。
var DefaultPathFields = []string{
	"path", "file", "filepath", "file_path", "cwd", "dir", "directory", "workdir",
	"dest", "destination", "target", "root", "root_dir", "out_path", "src_path",
}

// Normalizer —— 参数规范化器（纯函数：同输入必然同输出，无时间/无随机/无网络）。
//
// MatchOn 与 Drop 都是**点路径**（`a.b.c`），支持"子树"语义：
// 写 `opts` 表示整棵 opts 子树；Drop 一条 `debug` 表示丢掉整棵 debug 子树。
// 参数键含 `.` 会让路径有歧义 —— 仓内工具参数名都是简单标识符，不为这个做转义（写死口径）。
type Normalizer struct {
	// MatchOn —— 参与匹配与指纹的字段集（点路径）。**空 = 除 Drop 外的全部字段**。
	MatchOn []string
	// Drop —— 去噪字段（点路径）。**nil = 用 DefaultNoiseFields**；显式 `[]string{}` = 不去噪。
	Drop []string
	// PathFields —— 需要绝对化的字段（点路径，按末段匹配）。**nil = 用 DefaultPathFields**。
	PathFields []string
	// BaseDir —— 相对路径的基准目录。空 ⇒ 用 os.Getwd()。
	//
	// 注意（如实）：录制与回放的 BaseDir 必须指向同一个目录，否则**同一文件**会被规范化成
	// 两个不同字符串 ⇒ 未命中。这是刻意的：绝对化换来的是"同一个文件必须表现成同一个字符串"，
	// 代价就是基准必须一致。错误信息里会指出差在哪个字段。
	BaseDir string
	// NumberPrecision —— 数字统一精度。0（默认）= 用**最短往返表示**统一（1 / 1.0 / 1.00 ⇒ "1"）；
	// >0 = 额外截断到该小数位（用于浮点噪声很大的场景）。
	NumberPrecision int
}

// Call —— 一次调用的规范化结果（匹配、指纹、效果键都基于它）。
type Call struct {
	Tool        string            `json:"tool"`
	Canonical   []byte            `json:"-"` // 规范化后的 JSON 对象（键排序、数字统一、路径绝对化、噪声已剔除）
	ArgsDigest  string            `json:"-"`
	Fingerprint string            `json:"-"`
	Fields      map[string]string `json:"-"` // 点路径 → 规范化后的**字面文本**（最近邻差异定位用）
}

func (n Normalizer) dropFields() []string {
	if n.Drop == nil {
		return DefaultNoiseFields
	}
	return n.Drop
}

func (n Normalizer) pathFields() []string {
	if n.PathFields == nil {
		return DefaultPathFields
	}
	return n.PathFields
}

// Normalize —— 规范化参数并算指纹。
func (n Normalizer) Normalize(tool string, args map[string]any) (Call, error) {
	if strings.TrimSpace(tool) == "" {
		return Call{}, fmt.Errorf("%w: 工具名为空", ErrNormalize)
	}
	base := strings.TrimSpace(n.BaseDir)
	if base == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Call{}, fmt.Errorf("%w: 取不到工作目录来解析相对路径：%v", ErrNormalize, err)
		}
		base = wd
	}
	enc := &encoder{
		norm:      n,
		base:      base,
		matchOn:   n.MatchOn,
		drop:      n.dropFields(),
		pathField: n.pathFields(),
		fields:    map[string]string{},
	}
	if err := enc.encodeMap(args, ""); err != nil {
		return Call{}, err
	}
	canon := enc.buf.Bytes()
	return Call{
		Tool:        tool,
		Canonical:   canon,
		ArgsDigest:  Digest(canon),
		Fingerprint: Fingerprint(tool, canon),
		Fields:      enc.fields,
	}, nil
}

// ── 规范化编码器（确定性：键排序 + 数字统一 + 路径绝对化 + 去噪）─────────────

type encoder struct {
	norm      Normalizer
	base      string
	matchOn   []string
	drop      []string
	pathField []string
	buf       bytes.Buffer
	fields    map[string]string
}

// inSet —— 点路径是否落在给定路径集里（精确或子树）。空集返回 false。
func inSet(set []string, path string) bool {
	for _, s := range set {
		if s == "" {
			continue
		}
		if path == s || strings.HasPrefix(path, s+".") || strings.HasPrefix(path, s+"[") {
			return true
		}
	}
	return false
}

// lastSegment —— 路径末段（去掉数组下标），用于 PathFields 的末段匹配。
func lastSegment(path string) string {
	p := path
	if i := strings.LastIndex(p, "["); i >= 0 {
		p = p[:i]
	}
	if i := strings.LastIndex(p, "."); i >= 0 {
		p = p[i+1:]
	}
	return p
}

func (e *encoder) keep(path string) bool {
	if inSet(e.drop, path) {
		return false
	}
	if len(e.matchOn) == 0 {
		return true
	}
	return inSet(e.matchOn, path)
}

func (e *encoder) encodeMap(m map[string]any, prefix string) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys) // 键排序：字节序（不是 map 迭代序）
	e.buf.WriteByte('{')
	first := true
	for _, k := range keys {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if !e.keep(path) {
			continue
		}
		if !first {
			e.buf.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		e.buf.Write(kb)
		e.buf.WriteByte(':')
		if err := e.encodeValue(m[k], path); err != nil {
			return err
		}
	}
	e.buf.WriteByte('}')
	return nil
}

func (e *encoder) encodeSlice(s []any, path string) error {
	e.buf.WriteByte('[')
	for i, v := range s {
		if i > 0 {
			e.buf.WriteByte(',')
		}
		if err := e.encodeValue(v, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	e.buf.WriteByte(']')
	return nil
}

func (e *encoder) encodeValue(v any, path string) error {
	start := e.buf.Len()
	switch t := v.(type) {
	case nil:
		e.buf.WriteString("null")
	case bool:
		if t {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
	case string:
		s := t
		if inSet(e.pathField, lastSegment(path)) || contains(e.pathField, path) {
			s = e.absolutize(s)
		}
		b, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("%w: 字段 %s 的字符串无法编码：%v", ErrNormalize, path, err)
		}
		e.buf.Write(b)
	case json.Number:
		s, err := normalizeNumber(string(t), e.norm.NumberPrecision)
		if err != nil {
			return fmt.Errorf("%w: 字段 %s：%v", ErrNormalize, path, err)
		}
		e.buf.WriteString(s)
	case float64:
		f, err := normalizeNumber(strconv.FormatFloat(t, 'g', -1, 64), e.norm.NumberPrecision)
		if err != nil {
			return fmt.Errorf("%w: 字段 %s：%v", ErrNormalize, path, err)
		}
		e.buf.WriteString(f)
	case float32:
		f, err := normalizeNumber(strconv.FormatFloat(float64(t), 'g', -1, 32), e.norm.NumberPrecision)
		if err != nil {
			return fmt.Errorf("%w: 字段 %s：%v", ErrNormalize, path, err)
		}
		e.buf.WriteString(f)
	case int:
		e.buf.WriteString(strconv.FormatInt(int64(t), 10))
	case int64:
		e.buf.WriteString(strconv.FormatInt(t, 10))
	case int32:
		e.buf.WriteString(strconv.FormatInt(int64(t), 10))
	case uint64:
		e.buf.WriteString(strconv.FormatUint(t, 10))
	case map[string]any:
		if err := e.encodeMap(t, path); err != nil {
			return err
		}
	case []any:
		if err := e.encodeSlice(t, path); err != nil {
			return err
		}
	case []string:
		anySlice := make([]any, len(t))
		for i, s := range t {
			anySlice[i] = s
		}
		if err := e.encodeSlice(anySlice, path); err != nil {
			return err
		}
	default:
		// 不支持的类型**报错**而不是静默丢字段：丢字段 = 悄悄放松匹配条件。
		return fmt.Errorf("%w: 字段 %s 的类型 %T 不可规范化（请转成 JSON 原生类型）", ErrNormalize, path, v)
	}
	e.fields[path] = e.buf.String()[start:e.buf.Len()]
	return nil
}

func contains(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

// absolutize —— 路径绝对化：`~` 不展开（不读环境变量）、不解析符号链接（不做 IO ⇒ 纯函数）、
// 只做 Clean + 相对转绝对。空串原样（"没给路径"与"给了个相对路径"是两件事）。
func (e *encoder) absolutize(s string) string {
	if s == "" {
		return s
	}
	if !filepath.IsAbs(s) {
		return filepath.Clean(filepath.Join(e.base, s))
	}
	return filepath.Clean(s)
}

// normalizeNumber —— 数字统一：整数值一律整数写法（1 / 1.0 / 1.00 ⇒ "1"），
// 其余用最短往返表示（0.5 / 5e-1 ⇒ "0.5"）；prec > 0 时额外截断到该小数位。
// NaN / ±Inf 报错（不是合法 JSON，且"偷偷写 null"会把两个不同的事实变成同一个指纹）。
func normalizeNumber(s string, prec int) (string, error) {
	if s == "" {
		return "", errors.New("空数字")
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return strconv.FormatInt(i, 10), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("不是合法数字（%q）", s)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", fmt.Errorf("非法数字（%q）", s)
	}
	if prec > 0 {
		scale := math.Pow(10, float64(prec))
		r := math.Round(f*scale) / scale
		f = r
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	}
	return strconv.FormatFloat(f, 'f', -1, 64), nil
}

// ── 回放器：严格匹配 + 最近邻 ──────────────────────────────────────────────

// Neighbor —— 一个"最近邻候选"（同工具的既有录播）。
type Neighbor struct {
	Seq         int      `json:"seq"`
	Fingerprint string   `json:"fingerprint"`
	Score       float64  `json:"score"`   // 相同字段占比（0..1）
	Equal       []string `json:"equal"`   // 相同的字段
	Differs     []string `json:"differs"` // 两侧都有但值不同
	Missing     []string `json:"missing"` // 本次有、候选没有
	Extra       []string `json:"extra"`   // 候选有、本次没有
}

// MissError —— 未命中（E1）：错误里必须带 fingerprint 与最近邻候选。
type MissError struct {
	Tool          string
	Fingerprint   string
	ArgsCanonical []byte
	Nearest       []Neighbor
	Scanned       int // 同工具候选总数（0 = trace 里连这个工具都没有）
}

func (e *MissError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v：工具 %s，fingerprint %s", ErrReplayMiss, e.Tool, e.Fingerprint)
	if e.Scanned == 0 {
		fmt.Fprintf(&b, "；trace 内没有任何工具 %q 的录制（严格语义：不静默放行，也不新录一条）", e.Tool)
	} else {
		fmt.Fprintf(&b, "；该工具共 %d 条录制，均不匹配。最近邻：", e.Scanned)
		for i, nb := range e.Nearest {
			fmt.Fprintf(&b, " [#%d seq=%d %s 相同 %.0f%%", i+1, nb.Seq, nb.Fingerprint, nb.Score*100)
			if len(nb.Differs) > 0 {
				fmt.Fprintf(&b, " 差异字段=%s", strings.Join(nb.Differs, ","))
			}
			if len(nb.Missing) > 0 {
				fmt.Fprintf(&b, " 本次多出=%s", strings.Join(nb.Missing, ","))
			}
			if len(nb.Extra) > 0 {
				fmt.Fprintf(&b, " 录播多出=%s", strings.Join(nb.Extra, ","))
			}
			b.WriteString("]")
		}
	}
	b.WriteString("；本次参数（规范化后）：")
	b.Write(e.ArgsCanonical)
	return b.String()
}

// Is —— 支持 errors.Is(err, ErrReplayMiss)。
func (e *MissError) Is(target error) bool { return target == ErrReplayMiss }

// StateMismatchError —— 前置状态不符（E5）：报"状态不匹配"，而不是默默回放。
type StateMismatchError struct {
	Tool        string
	Seq         int    // 命中的录播序号
	Want        string // 录制时记录的前置状态指纹
	Got         string // 本次调用方给出的当前状态指纹
	Fingerprint string
	Reason      string
}

func (e *StateMismatchError) Error() string {
	return fmt.Sprintf("%v：工具 %s 命中录播 seq=%d（fingerprint %s），但前置状态不符 —— %s（录制时 %q，本次 %q）；拒绝在此状态上回放",
		ErrStateMismatch, e.Tool, e.Seq, e.Fingerprint, e.Reason, e.Want, e.Got)
}

// Is —— 支持 errors.Is(err, ErrStateMismatch)。
func (e *StateMismatchError) Is(target error) bool { return target == ErrStateMismatch }

type playerEntry struct {
	rec    Record
	fields map[string]string
}

// Player —— 回放器：**只读**一组录播事实，按 fingerprint 严格匹配。
//
// 构造后不再碰文件系统（Dispatcher 在回放期不读也不写 trace），故"回放期只读"是结构上成立的，
// 不靠自觉。
type Player struct {
	norm      Normalizer
	entries   []playerEntry
	byTool    map[string][]int
	byFP      map[string][]int
	maxNearby int
}

// NewPlayer —— 用既有录播事实建回放器。
func NewPlayer(recs []Record, norm Normalizer) *Player {
	p := &Player{
		norm:      norm,
		byTool:    map[string][]int{},
		byFP:      map[string][]int{},
		maxNearby: 3,
	}
	for _, rec := range recs {
		fields, err := flattenCanonical(rec.ArgsCanonical, norm.NumberPrecision)
		if err != nil {
			fields = map[string]string{"<解析失败>": err.Error()}
		}
		i := len(p.entries)
		p.entries = append(p.entries, playerEntry{rec: rec, fields: fields})
		p.byTool[rec.Tool] = append(p.byTool[rec.Tool], i)
		fp := rec.Fingerprint()
		p.byFP[fp] = append(p.byFP[fp], i)
	}
	return p
}

// MaxNeighbors —— 错误里最多列几个最近邻（默认 3）。
func (p *Player) MaxNeighbors(n int) {
	if n > 0 {
		p.maxNearby = n
	}
}

// Len —— 录播条数。
func (p *Player) Len() int { return len(p.entries) }

// Records —— 录播事实（副本，调用方改不动内部状态）。
func (p *Player) Records() []Record {
	out := make([]Record, 0, len(p.entries))
	for _, e := range p.entries {
		out = append(out, e.rec)
	}
	return out
}

// PlayerFromTrace —— 按消费游标从 trace 文件载入回放器，返回**新游标**（E6：重启续读）。
//
// from = 之前已消费的条数；只有 `from` 之后的录播才会进回放器 ⇒ 重启后不会从头重放。
// 新游标的前缀指纹覆盖**整个**已读前缀（含本次载入的部分），可直接落盘。
func PlayerFromTrace(path string, norm Normalizer, from int) (*Player, Cursor, error) {
	r, err := OpenReader(path, from)
	if err != nil {
		return nil, Cursor{}, err
	}
	defer r.Close()
	var recs []Record
	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, Cursor{}, err
		}
		recs = append(recs, rec)
	}
	return NewPlayer(recs, norm), r.Cursor(), nil
}

// Lookup —— 严格命中：返回**唯一**指纹相同的录播事实。未命中 ⇒ *MissError。
//
// 同一指纹有多条录制（同一调用在 trace 里出现过两次）时返回**第一条**，是刻意的：
// 录播的语义是"当时发生过什么"，而"第一次发生"才是原始事实；后续同指纹的行属于重复执行
// （其去重应由 EffectLedger 负责，不该在匹配层偷偷替调用方选一条）。
func (p *Player) Lookup(call Call) (Record, error) {
	if idxs := p.byFP[call.Fingerprint]; len(idxs) > 0 {
		return p.entries[idxs[0]].rec, nil
	}
	return Record{}, p.miss(call)
}

// LookupChecked —— 严格命中 + **前置状态校验**（E5）。三个方向都不许静默：
//
//	· 录播记了前置、本次给了不同的 ⇒ 状态不匹配
//	· 录播记了前置、本次**没给** ⇒ 状态不匹配（"无法校验"不等于"校验通过"，fail-closed）
//	· 录播**没记**前置、本次给了 ⇒ 状态不匹配（录制未记 = 这条 trace 无法支撑这次校验）
func (p *Player) LookupChecked(call Call, currentStateHash string) (Record, error) {
	rec, err := p.Lookup(call)
	if err != nil {
		return Record{}, err
	}
	switch {
	case rec.PreconditionHash == "" && currentStateHash == "":
		return rec, nil
	case rec.PreconditionHash == "":
		return Record{}, &StateMismatchError{
			Tool: call.Tool, Seq: rec.Seq, Want: "", Got: currentStateHash,
			Fingerprint: call.Fingerprint, Reason: "录制时未记前置状态（无法校验）",
		}
	case currentStateHash == "":
		return Record{}, &StateMismatchError{
			Tool: call.Tool, Seq: rec.Seq, Want: rec.PreconditionHash, Got: "",
			Fingerprint: call.Fingerprint, Reason: "调用方未提供当前状态指纹",
		}
	case rec.PreconditionHash != currentStateHash:
		return Record{}, &StateMismatchError{
			Tool: call.Tool, Seq: rec.Seq, Want: rec.PreconditionHash, Got: currentStateHash,
			Fingerprint: call.Fingerprint, Reason: "前置状态指纹不符",
		}
	}
	return rec, nil
}

// miss —— 造 *MissError（含最近邻）。
func (p *Player) miss(call Call) error {
	cand := p.byTool[call.Tool]
	neighbors := make([]Neighbor, 0, len(cand))
	for _, i := range cand {
		neighbors = append(neighbors, compareFields(p.entries[i].rec, p.entries[i].fields, call.Fields))
	}
	sort.SliceStable(neighbors, func(a, b int) bool {
		if neighbors[a].Score != neighbors[b].Score {
			return neighbors[a].Score > neighbors[b].Score
		}
		return neighbors[a].Seq < neighbors[b].Seq
	})
	if len(neighbors) > p.maxNearby {
		neighbors = neighbors[:p.maxNearby]
	}
	return &MissError{
		Tool:          call.Tool,
		Fingerprint:   call.Fingerprint,
		ArgsCanonical: call.Canonical,
		Nearest:       neighbors,
		Scanned:       len(cand),
	}
}

// compareFields —— 字段级差异（最近邻的打分与"差在哪"）。
// 分母用**并集**：只多不少都算差异 ⇒ 分数不会因为"两边字段数差很多"而虚高。
func compareFields(rec Record, want, got map[string]string) Neighbor {
	nb := Neighbor{Seq: rec.Seq, Fingerprint: rec.Fingerprint()}
	keys := map[string]bool{}
	for k := range want {
		keys[k] = true
	}
	for k := range got {
		keys[k] = true
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	equal := 0
	for _, k := range ordered {
		w, wOK := want[k]
		g, gOK := got[k]
		switch {
		case wOK && gOK && w == g:
			equal++
			nb.Equal = append(nb.Equal, k)
		case wOK && gOK:
			nb.Differs = append(nb.Differs, k)
		case gOK:
			nb.Missing = append(nb.Missing, k) // 录播多出（本次没给）
		default:
			nb.Extra = append(nb.Extra, k) // 本次多出
		}
	}
	if len(ordered) > 0 {
		nb.Score = float64(equal) / float64(len(ordered))
	}
	return nb
}

// flattenCanonical —— 把已规范化的 JSON 还原成"点路径 → 字面文本"（差异定位的输入）。
// 数字经 json.Number 保字面（不经过 float64，避免又一次精度损失）。
func flattenCanonical(b []byte, prec int) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if err := flattenInto(out, v, "", prec); err != nil {
		return nil, err
	}
	return out, nil
}

func flattenInto(out map[string]string, v any, path string, prec int) error {
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if err := flattenInto(out, sub, p, prec); err != nil {
				return err
			}
		}
	case []any:
		for i, sub := range t {
			if err := flattenInto(out, sub, fmt.Sprintf("%s[%d]", path, i), prec); err != nil {
				return err
			}
		}
	case json.Number:
		s, err := normalizeNumber(string(t), prec)
		if err != nil {
			return err
		}
		out[path] = s
	case nil:
		out[path] = "null"
	case bool:
		out[path] = strconv.FormatBool(t)
	case string:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		out[path] = string(b)
	default:
		return fmt.Errorf("%w: 无法归一的字面类型 %T（字段 %s）", ErrNormalize, v, path)
	}
	return nil
}

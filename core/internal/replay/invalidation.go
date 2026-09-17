// invalidation.go —— trace 的**版本与失效判定**（T4.6；设计稿 §〇 E11 ★★ + G2 ★★★）。
//
// ── 病根 ──
//
// trace 是"当时那套东西"的录像。工具的参数 schema 改了、引擎换了（llama.cpp 的 system_fingerprint
// 变了）、trace 行格式改了 —— 旧录像对新运行**已经不是一个事实来源**。此时若还拿它回放，得到的
// "通过"就是拿旧世界的事实给新世界背书；而这类假绿极难发现：回放会命中、会给出结果，只是结果是
// 另一个世界的。E11 的处置一句话：**不匹配即作废重录**。
//
// ── 口径（写死）──
//
//  1. **三枚指纹逐字段硬比**：schema_version（行格式版本，必须精确等于本包的 SchemaVersion）/
//     tool_fingerprint（工具表）/ engine_fingerprint（引擎，值级复用 infergeom 给的那枚）。
//     任一不等 ⇒ *InvalidationError（errors.Is(err, ErrTraceInvalidated)），错误里**同时给出两侧的
//     指纹值**：只报"不匹配"等于让人自己去猜哪边变了。
//  2. **fail-closed**：任一侧**没记**（空）而另一侧记了 ⇒ 作废（"没记"不等于"没变"）；两侧都没记
//     ⇒ 视为未记录（该字段不参与判定）。旧 trace 没有头部声明 ⇒ 一律作废 —— 这正是 E11 要的
//     "旧录像重录"；确实要放行老 trace 必须**显式**声明（InvalidationOptions.AllowHeaderless）。
//  3. 头部是 **sidecar 文件**（`<trace>.header.json`），不塞进 trace 行：trace 的读侧是**严格**的
//     （seq 必须 +1、行必须合法、digest 必须复算得上），往行格式里加字段等于给 T4.1 已验收的严格读
//     开一个口子；sidecar 还能在**打开 trace 之前**就把作废判掉。
//  4. **引擎指纹不 import infergeom**（叶包纪律：零内部依赖）：这里做的是**值级复用** ——
//     调用方把 infergeom.Parse 出来的 Geometry.SystemFingerprint（实测形如 b10470-34af94cd9）抄进
//     来即可；本包不反向依赖任何内部包。
//  5. 头部**不含时间、不含条数**：它是身份声明，不是事件。带时间只会让头部字节随运行变化
//     （而头部本身也应当可比），并在审计表里凭空多出一个未注入的取时点。
package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ErrTraceInvalidated —— trace 作废（专项错误码；errors.Is(err, ErrTraceInvalidated)）。
var ErrTraceInvalidated = errors.New("replay: trace 已作废")

// headerSuffix —— 头部 sidecar 的后缀（`<trace>.jsonl.header.json`）。
const headerSuffix = ".header.json"

// FieldDiff —— 一处身份差异（两侧的**值**都留着：报告要能直接说清哪边变了）。
type FieldDiff struct {
	Field string `json:"field"`
	Want  string `json:"want"` // 录制侧（trace 头部声明）
	Got   string `json:"got"`  // 本次运行
}

// Fingerprints —— 一次运行的"身份"。
//
// EngineDigest / ModelDigest 是 G2 要求必记的两项（引擎二进制 sha、模型 sha256）：
// 它们同样参与判定（口径与三枚指纹一致：都空 = 未记录；一侧空 = 作废；都非空 = 必须相等）。
type Fingerprints struct {
	SchemaVersion     int    `json:"schema_version"`
	ToolFingerprint   string `json:"tool_fingerprint,omitempty"`
	EngineFingerprint string `json:"engine_fingerprint,omitempty"`
	EngineDigest      string `json:"engine_digest,omitempty"`
	ModelDigest       string `json:"model_digest,omitempty"`
}

// String —— 一行身份（错误信息与报告里用；空值写 "-"）。
func (f Fingerprints) String() string {
	return fmt.Sprintf("{schema=%d tool=%s engine=%s engine_sha=%s model_sha=%s}",
		f.SchemaVersion, orDash(short(f.ToolFingerprint)), orDash(f.EngineFingerprint),
		orDash(short(f.EngineDigest)), orDash(short(f.ModelDigest)))
}

// Check —— 逐字段硬比（h = 录制侧声明，cur = 本次运行身份）。
func (f Fingerprints) Check(cur Fingerprints) error {
	return checkFingerprints(f, cur)
}

// InvalidationError —— trace 作废（专项错误码）。
type InvalidationError struct {
	TracePath string
	Reason    string
	Diffs     []FieldDiff
	Want      Fingerprints // 录制侧
	Got       Fingerprints // 本次运行
}

// Error —— 人读判词：**两侧指纹都在里面**（E11 要求"附两边指纹"）。
func (e *InvalidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v：%s", ErrTraceInvalidated, e.Reason)
	if e.TracePath != "" {
		fmt.Fprintf(&b, "（trace %s）", e.TracePath)
	}
	for _, d := range e.Diffs {
		fmt.Fprintf(&b, "；%s：录制侧 %s ≠ 本次 %s", d.Field, orDash(d.Want), orDash(d.Got))
	}
	fmt.Fprintf(&b, "；两侧指纹 —— 录制侧 %s；本次 %s", e.Want.String(), e.Got.String())
	return b.String()
}

// Is —— 支持 errors.Is(err, ErrTraceInvalidated)。
func (e *InvalidationError) Is(target error) bool { return target == ErrTraceInvalidated }

// checkFingerprints —— 失效判定的唯一实现（口径见包注释 1/2）。
func checkFingerprints(hdr, cur Fingerprints) error {
	var diffs []FieldDiff
	var reasons []string
	add := func(field, want, got, why string) {
		diffs = append(diffs, FieldDiff{Field: field, Want: want, Got: got})
		reasons = append(reasons, why)
	}

	// ① 行格式版本：必须精确等于本包的 SchemaVersion。0 = 没声明 ⇒ 作废（"老录像"的入口）。
	if hdr.SchemaVersion != SchemaVersion {
		add("schema_version", strconv.Itoa(hdr.SchemaVersion), strconv.Itoa(SchemaVersion),
			fmt.Sprintf("头部声明的行格式版本是 %d，本包只认 %d（旧录像的行格式可能已被改过 ⇒ 作废重录）",
				hdr.SchemaVersion, SchemaVersion))
	}
	if cur.SchemaVersion != SchemaVersion {
		add("schema_version(本次)", strconv.Itoa(SchemaVersion), strconv.Itoa(cur.SchemaVersion),
			fmt.Sprintf("本次运行声明的行格式版本是 %d（本包为 %d）：调用方没声明对 ⇒ fail-closed 作废",
				cur.SchemaVersion, SchemaVersion))
	}

	// ② 四枚身份指纹：都空 = 未记录（不参与）；一侧空 = 作废；都非空 = 必须相等。
	pairs := []struct {
		field string
		want  string
		got   string
		note  string
	}{
		{"tool_fingerprint", hdr.ToolFingerprint, cur.ToolFingerprint, "工具表（工具名 + 参数 schema）变了 ⇒ 同一份参数在两个世界里不是同一件事"},
		{"engine_fingerprint", hdr.EngineFingerprint, cur.EngineFingerprint, "引擎身份变了（如 system_fingerprint 从 b10470-34af94cd9 变成别的）⇒ 采样与几何都可能不同"},
		{"engine_digest", hdr.EngineDigest, cur.EngineDigest, "引擎二进制换了 ⇒ 逐位可复现的前提没了（G2）"},
		{"model_digest", hdr.ModelDigest, cur.ModelDigest, "模型权重换了 ⇒ 同一份录播对应的不再是同一个模型（G2：按 sha256 记，不按路径）"},
	}
	for _, p := range pairs {
		switch {
		case p.want == "" && p.got == "":
			continue // 两侧都没记 ⇒ 视为未记录（不参与判定）
		case p.want == p.got:
			continue
		case p.want == "":
			add(p.field, "", p.got, fmt.Sprintf("录制侧未记录 %s，本次却有值 ⇒ fail-closed 作废（没记不等于没变）：%s", p.field, p.note))
		case p.got == "":
			add(p.field, p.want, "", fmt.Sprintf("本次未声明 %s（录制侧是它）⇒ fail-closed 作废：%s", p.field, p.note))
		default:
			add(p.field, p.want, p.got, fmt.Sprintf("%s 不符：%s", p.field, p.note))
		}
	}

	if len(diffs) == 0 {
		return nil
	}
	return &InvalidationError{
		Reason: reasons[0],
		Diffs:  diffs,
		Want:   hdr,
		Got:    cur,
	}
}

// ── 头部（sidecar）──────────────────────────────────────────────────────────

// Header —— trace 的身份声明（写在 `<trace>.header.json`）。
//
// Trace 字段是自校验用的一环：头部与 trace 配错（复制粘贴、手工改路径）时，只比指纹是发现不了的。
type Header struct {
	V int `json:"v"` // 头部格式版本（= SchemaVersion；独立字段便于以后单独升级头部）
	Fingerprints
	Trace string `json:"trace,omitempty"` // trace 文件名（配错时能立刻指出）
	Note  string `json:"note,omitempty"`  // 人读备注（**不参与**判定）
}

// HeaderPath —— 头部 sidecar 的路径（`<trace>.header.json`）。
func HeaderPath(tracePath string) string { return tracePath + headerSuffix }

// WriteHeader —— 原子写头部（临时文件 + rename）：半写的头部比没有头部更危险 ——
// 它会让一次"看起来检查过了"的回放跑在错身份上。
func WriteHeader(tracePath string, h Header) error {
	if h.V == 0 {
		h.V = SchemaVersion
	}
	if h.V != SchemaVersion {
		return fmt.Errorf("%w: 头部格式版本 %d 本包不认识（只认 %d）", ErrTraceInvalidated, h.V, SchemaVersion)
	}
	if h.Trace == "" {
		h.Trace = filepath.Base(tracePath)
	}
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: 序列化头部：%v", ErrTraceInvalidated, err)
	}
	path := HeaderPath(tracePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%w: 建目录 %s：%v", ErrTraceInvalidated, filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("%w: 写头部 %s：%v", ErrTraceInvalidated, tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("%w: 落头部 %s：%v", ErrTraceInvalidated, path, err)
	}
	return nil
}

// ReadHeader —— 读头部。第三个返回值 = "是否存在"（不存在 ≠ 错误：由调用方按 fail-closed 处置）。
func ReadHeader(tracePath string) (Header, bool, error) {
	var h Header
	b, err := os.ReadFile(HeaderPath(tracePath))
	if errors.Is(err, os.ErrNotExist) {
		return Header{}, false, nil
	}
	if err != nil {
		return Header{}, false, fmt.Errorf("%w: 读头部 %s：%v", ErrTraceInvalidated, HeaderPath(tracePath), err)
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return Header{}, false, fmt.Errorf("%w: 头部 %s 不是合法 JSON：%v", ErrTraceInvalidated, HeaderPath(tracePath), err)
	}
	if h.V != SchemaVersion {
		return Header{}, true, &InvalidationError{
			TracePath: tracePath,
			Reason:    fmt.Sprintf("头部自身的格式版本 v=%d 本包不认识（只认 %d）⇒ 作废", h.V, SchemaVersion),
			Want:      h.Fingerprints,
		}
	}
	return h, true, nil
}

// InvalidationOptions —— 作废判定的显式开关。
type InvalidationOptions struct {
	// AllowHeaderless —— 显式放行"没有头部声明"的 trace（默认 false = 作废）。
	// 为什么默认作废：没有头部就无从判定版本与指纹，而 E11 的处置是"不匹配即作废重录"；
	// 静默放行等于把整个机制关掉而没人知道。
	AllowHeaderless bool
}

// CheckTrace —— 回放**之前**的作废判定（E11 的入口）：读头部 + 逐字段比 + 配错检查。
func CheckTrace(tracePath string, cur Fingerprints, opt InvalidationOptions) (Header, error) {
	h, ok, err := ReadHeader(tracePath)
	if err != nil {
		return Header{}, err
	}
	if !ok {
		if opt.AllowHeaderless {
			return Header{}, nil
		}
		return Header{}, &InvalidationError{
			TracePath: tracePath,
			Reason: "无头部声明（" + HeaderPath(tracePath) + " 不存在）⇒ 无法判定行格式版本与工具/引擎指纹 ⇒ 作废重录；" +
				"确实要放行老 trace 请**显式**设置 InvalidationOptions.AllowHeaderless",
			Got: cur,
		}
	}
	if h.Trace != "" && h.Trace != filepath.Base(tracePath) {
		return Header{}, &InvalidationError{
			TracePath: tracePath,
			Reason:    fmt.Sprintf("头部指向的是另一个 trace（头部声明 %q，实际 %q）⇒ 头部与 trace 配错了 ⇒ 作废", h.Trace, filepath.Base(tracePath)),
			Want:      h.Fingerprints,
			Got:       cur,
		}
	}
	if err := checkFingerprints(h.Fingerprints, cur); err != nil {
		var ie *InvalidationError
		if errors.As(err, &ie) {
			ie.TracePath = tracePath
		}
		return h, err
	}
	return h, nil
}

// NewRecorderWithHeader —— 录制前先落**头部声明**：这样录出来的 trace 自带身份，别人回放时才有东西可比。
//
// 拒绝"什么都没说"的头部（schema_version 必须等于本包版本、tool_fingerprint 必须非空）：
// 声明一个空身份比不声明更坏 —— 后者会被 fail-closed 拦下，前者会被当成"检查过了"。
// 引擎那两项在纯工具场景可以留空（两侧都空 = 未记录），有引擎的场景必须填（G2）。
func NewRecorderWithHeader(tracePath string, norm Normalizer, live ToolFunc, fp Fingerprints) (*Dispatcher, error) {
	if fp.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%w: 录制头部必须声明 schema_version=%d（实得 %d）",
			ErrTraceInvalidated, SchemaVersion, fp.SchemaVersion)
	}
	if strings.TrimSpace(fp.ToolFingerprint) == "" {
		return nil, fmt.Errorf("%w: 录制头部必须给 tool_fingerprint（空身份比不声明更坏：它会被当成"+
			"「检查过了」而放行）", ErrTraceInvalidated)
	}
	d, err := NewRecorder(tracePath, norm, live)
	if err != nil {
		return nil, err
	}
	if err := WriteHeader(tracePath, Header{V: SchemaVersion, Fingerprints: fp}); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// NewReplayerChecked —— 回放**之前**先做作废判定：指纹不符 ⇒ 专项错误码，且**不返回 Player**
// （"作废"必须是"没有回放器可用"，而不是"给了回放器但打印了一条警告"）。
func NewReplayerChecked(tracePath string, norm Normalizer, from int, cur Fingerprints, opt InvalidationOptions) (*Dispatcher, Cursor, error) {
	if _, err := CheckTrace(tracePath, cur, opt); err != nil {
		return nil, Cursor{}, err
	}
	return NewReplayer(tracePath, norm, from)
}

// ToolTableFingerprint —— **工具表指纹**：工具名 + 参数 schema 的规范化摘要。
//
// 输入 tools = 工具名 → 该工具的 schema 文本（JSON 即可）。本包**不解释** schema 的内容 ——
// 谁定义工具谁负责给摘要口径；这里只保证"同样的工具表必然同样指纹、键序不影响结果"（显式排序）。
func ToolTableFingerprint(tools map[string]string) string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names) // 显式排序：map 迭代序不进结果（审计表 map 类会盯这一处）
	h := sha256.New()
	fmt.Fprintf(h, "tools\x1f%d\x1f", len(names))
	for _, name := range names {
		fmt.Fprintf(h, "%s\x1f%s\x1e", name, tools[name])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

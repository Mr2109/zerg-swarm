// normalize_policy.go —— T4.2 的**口径声明**与**配置自检**（E3）。
//
// ── 为什么"规范化口径"本身要能读出来、能比对 ─────────────────────────────────
//
// 指纹是"规范化后的字节"的哈希 ⇒ **口径变了，指纹就整体变了**。而口径是十几个字段集
// （MatchOn / Drop / PathFields / CollapseWhitespace / NullEqualsAbsent / NumberPrecision / BaseDir）
// 的组合，光看代码很难说"这次运行用的是哪一份口径"。后果有两种，都很贵：
//
//	· 录制时用一套口径、回放时用另一套 ⇒ 大面积未命中，且看着像"trace 坏了"；
//	· 不比对口径直接比指纹 ⇒ 同一条逻辑调用在不同口径下算出不同指纹，去重与幂等跟着一起错。
//
// 所以：Policy() 把**实际生效**的口径（nil 已展开成默认表）摊平成一个可序列化结构，
// PolicyID() 是它的 sha256。口径一模一样 ⇒ ID 一模一样（含集合次序无关）；口径变了 ⇒ ID 变。
// 跨机比对用 PolicyShapeID()（不含 BaseDir，因为工作目录天然因机而异）。
//
// ── 配置自检：三类"一定是写错了"的配置当场报错 ───────────────────────────────
//
//	① MatchOn 里写了噪声字段（ts / trace_id / 重试计数 / 耗时）⇒ 命中率归零（E3 的病根）；
//	② MatchOn 与 Drop 打架（同一字段既匹配又丢弃）⇒ 二义，Drop 会赢，但不是人写代码时的本意；
//	③ AllowNoiseInMatchOn 里声明了豁免，但 MatchOn 里根本没有这个字段
//	   ⇒ 豁免表在腐化（跟审计侧的"过期豁免"一条道理）。
//
// 这三条都由 Normalize **自动**调用 Validate 兜住 —— 口径配错就在第一次规范化当场炸，
// 而不是等回放期"到处未命中"再回头猜。三段式报错都带字段名与处置办法。
package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNoiseFieldInMatchOn —— 匹配字段集里含噪声字段（哨兵错误，调用方用 errors.Is 分流）。
var ErrNoiseFieldInMatchOn = errors.New("replay: 匹配字段集里含噪声字段")

// DefaultNormalizer —— 默认口径的规范化器（显式给基准目录）。
//
// 默认口径（写死，有用例钉住）：MatchOn 空 = 除噪声字段外的全部参数；Drop/PathFields/
// CollapseWhitespace 取各自的默认表；NullEqualsAbsent 空 = 显式 null 与缺省**不等价**；
// NumberPrecision 0 = 最短往返表示统一（1 / 1.0 / 1.00 ⇒ "1"）。
//
// BaseDir 请**显式给**：路径字段一旦走绝对化，指纹就与基准目录绑定，录制与回放必须同基准
// （见 Normalizer.BaseDir 的注释）。
func DefaultNormalizer(baseDir string) Normalizer {
	return Normalizer{BaseDir: baseDir}
}

// Validate —— 口径自检（Normalize 会先调它；也可单独调用做"启动即校验"）。
//
// 只拦"确定是写错了"的三类（见文件头），不做任何推断、不看环境、不读时钟。
func (n Normalizer) Validate() error {
	for _, f := range n.MatchOn {
		name := strings.TrimSpace(f)
		if name == "" {
			return fmt.Errorf("%w: MatchOn 里有空字段名（空名字匹配不到任何字段，只会让人以为收窄了）", ErrNormalize)
		}
		seg := lastSegment(name)
		exempt := contains(n.AllowNoiseInMatchOn, name) || contains(n.AllowNoiseInMatchOn, seg)
		if isNoiseName(seg) && !exempt {
			return fmt.Errorf("%w: 匹配集里含 %q（噪声字段每次调用都不同 ⇒ 命中率归零）。"+
				"要从匹配口径里去掉它就别写进 MatchOn；确有语义请显式写进 AllowNoiseInMatchOn 留下痕迹",
				ErrNoiseFieldInMatchOn, name)
		}
		if inSet(n.dropFields(), name) {
			return fmt.Errorf("%w: 字段 %q 同时出现在 MatchOn 与 Drop（二义：Drop 会赢，但这份配置不是写它的人的本意）。"+
				"要匹配就从 Drop 里去掉，要丢弃就别写进 MatchOn", ErrNormalize, name)
		}
	}
	// 过期豁免：声明了却不在 MatchOn 里（MatchOn 为空时"全部字段"口径下，豁免没有作用点）
	for _, a := range n.AllowNoiseInMatchOn {
		if strings.TrimSpace(a) == "" {
			continue
		}
		if !contains(n.MatchOn, a) && !inSet(n.MatchOn, a) {
			return fmt.Errorf("%w: AllowNoiseInMatchOn 里的 %q 不在 MatchOn 中（过期豁免：豁免表在腐化，"+
				"要么它是多余的一行，要么真正要匹配的字段名写错了）", ErrNormalize, a)
		}
	}
	return nil
}

// isNoiseName —— 末段名是否在噪声字段黑名单里。
func isNoiseName(seg string) bool {
	if seg == "" {
		return false
	}
	return contains(NoiseFieldNames, seg)
}

// Policy —— **实际生效**的规范化口径（nil 已展开成默认表；集合语义：次序无关、无重复）。
type Policy struct {
	MatchOn             []string `json:"match_on"`            // 空 = 除 Drop 外的全部参数
	Drop                []string `json:"drop"`                // 去噪字段
	PathFields          []string `json:"path_fields"`         // 绝对化字段
	CollapseWhitespace  []string `json:"collapse_whitespace"` // 折叠空白字段
	NullEqualsAbsent    []string `json:"null_equals_absent"`  // 显式 null ≡ 缺省的字段（空 = 不等价）
	AllowNoiseInMatchOn []string `json:"allow_noise_in_match_on,omitempty"`
	NumberPrecision     int      `json:"number_precision"`
	BaseDir             string   `json:"base_dir,omitempty"` // 空 = 用进程工作目录（运行时兜底，见 Normalize）
}

// Policy —— 摊平当前生效口径。
func (n Normalizer) Policy() Policy {
	return Policy{
		MatchOn:             normSet(n.MatchOn),
		Drop:                normSet(n.dropFields()),
		PathFields:          normSet(n.pathFields()),
		CollapseWhitespace:  normSet(n.wsFields()),
		NullEqualsAbsent:    normSet(n.NullEqualsAbsent),
		AllowNoiseInMatchOn: normSet(n.AllowNoiseInMatchOn),
		NumberPrecision:     n.NumberPrecision,
		BaseDir:             strings.TrimSpace(n.BaseDir),
	}
}

// PolicyID —— 口径指纹（含 BaseDir）：`sha256:` + 十六进制。
//
// 用途：记录/比对"这份 trace 的指纹是按哪套口径算出来的"。口径一变，ID 必变；口径完全相同
// （含"用默认表"与"显式列出同一份默认表"这两种写法）ID 相同。
func (n Normalizer) PolicyID() string { return policyDigest(n.Policy(), true) }

// PolicyShapeID —— 口径指纹（**不含** BaseDir）：跨机 / 跨工作目录比对用。
//
// 注意（如实）：BaseDir 为空时，指纹仍由**进程工作目录**参与（路径字段的绝对化），
// 因此 ShapeID 相同**不等于**指纹可以互相匹配 —— 要跨机复现请显式给 BaseDir，
// 并让录制与回放指向同一份基准（否则会在错误信息里逐字段指出差在哪个 path）。
func (n Normalizer) PolicyShapeID() string { return policyDigest(n.Policy(), false) }

func policyDigest(p Policy, withBase bool) string {
	if !withBase {
		p.BaseDir = ""
	}
	b, err := json.Marshal(p)
	if err != nil {
		// 结构体里的字段都是字符串切片与 int，Marshal 不会失败；真失败了也**不静默**：
		// 返回一个"无法计算"的显式形态，调用方一比就知道有问题。
		return "sha256:<无法序列化：" + strings.ReplaceAll(err.Error(), "\n", " ") + ">"
	}
	return Digest(b)
}

// normSet —— 集合归一：拷贝（不碰调用方的切片）+ 去重 + 字节序排序（Locale 无关）。
func normSet(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

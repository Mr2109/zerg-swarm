// level.go —— 封闭等级（设计稿 §4.2/§4.3）。
//
// 两条不许含糊的口径：
//
//	① 等级与"放行清单"是**两个字段**（某引擎必须开空间内 IPC 或必须允许 JIT ⇒ 它仍可判
//	   enclosed.kernel，但档案必须明写额外放行了什么）—— 照着 PraisonAI 那类
//	   "宣称等级与实际强制不一致"的事故写的（§10.3 第 2 条）。
//	② "读不到证据"是**独立等级 unverified**：既不当成拒绝，也不许当成 enclosed.*
//	   （AR4SI：证据不足不得被当作肯定断言；GKE：沙箱内自报不可信 ⇒ 证据必须来自空间之外）。
package enclosure

import "time"

// Level 封闭等级。
type Level string

const (
	// LevelKernel 视图级封闭：东西"看不见"（独立 mount namespace + pivot_root 生效）。
	LevelKernel Level = "enclosed.kernel"
	// LevelOS 授权级封闭：东西看得见但打不开（策略在进程上实际生效）。
	LevelOS Level = "enclosed.os"
	// LevelUnverified 有策略声明、但拿不到证据（独立一级，不等于拒绝）。
	LevelUnverified Level = "unverified"
	// LevelNone 实读无任何封闭证据。
	LevelNone Level = "none"
)

// Evidence 外部实读证据（**必须取自空间之外**；空间内自报一律不可信）。
type Evidence struct {
	Read          bool // 是否读到证据（读不到不是"没有封闭"，而是"不知道"）
	MountIsolated bool // 视图级：独立 mount namespace 且挂载集实读与声明相符
	PolicyActive  bool // 授权级：策略确实作用在该进程上
}

// Verdict 等级声明。expected 与 observed **两个字段都必须出现**（判据 8：不许合并成一个布尔）。
type Verdict struct {
	Expected  Level     `json:"expected" yaml:"expected"`
	Observed  Level     `json:"observed" yaml:"observed"`
	Allowlist []string  `json:"allowlist,omitempty" yaml:"allowlist,omitempty"`
	CheckedAt time.Time `json:"checked_at" yaml:"checked_at"`
	Note      string    `json:"note,omitempty" yaml:"note,omitempty"`
}

// Judge 依据外部实读证据判定实测等级。
//
// 判定顺序（先严后宽，且**永不在无证据时给 enclosed.***）：
//   - 读不到证据          ⇒ unverified（独立一级）
//   - 视图级成立          ⇒ enclosed.kernel
//   - 仅授权级成立        ⇒ enclosed.os
//   - 读到但无任何封闭证据 ⇒ none
func Judge(expected Level, ev Evidence, allowlist []string, at time.Time) Verdict {
	v := Verdict{Expected: expected, Allowlist: allowlist, CheckedAt: at}
	switch {
	case !ev.Read:
		v.Observed = LevelUnverified
		v.Note = "读不到封闭证据：判为独立等级 unverified（既不作拒绝，也不作 enclosed.*）"
	case ev.MountIsolated:
		v.Observed = LevelKernel
	case ev.PolicyActive:
		v.Observed = LevelOS
	default:
		v.Observed = LevelNone
	}
	if v.Observed != v.Expected {
		v.Note = "期望等级与实测等级不一致（两字段各自保留，不许合并）"
	}
	return v
}

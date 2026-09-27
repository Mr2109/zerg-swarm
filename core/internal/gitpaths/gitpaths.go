// Package gitpaths —— 件名统一出口（适配层）。
//
// ★ 本包是仓内**唯一**取件名的适配层（C-1）：凡需要「版本库里的件名」的地方，
// 只许经本包的 List 拿，不许裸调 git 的取件名子命令（裸调由门
// scripts/gates/check-git-path-outlet.py 拦，C-6）。
//
// 条文对照（《设计-件名统一出口与调用点判据-v0.1.md》§4.2 · 任务清单 A1）：
//
//	C-1 唯一出口 ...... 本包即那「一个」出口；导出面只有一个查询函数 List。
//	C-2 出口一律 -z .... 七类面的 argv 全部带 -z（见 Face.args），按 \0 切分
//	                    （splitNUL）。`-c` 只存在于 QuoteRespectConfig 这一应急分支，
//	                    绝不作默认档。
//	C-3 -c 位置铁律 .... -c 只由 buildArgv 的全局选项区产出（恒在子命令之前），
//	                    再由 assertGlobalOptionsFirst 复查：`-c` 落在子命令之后 ⇒ error。
//	C-4 拼法唯一 ....... 只用官方拼写 core.quotePath（常量 KQuotePath），禁第二拼法。
//	C-5 出口后置校验 ... 返回前逐件名跑 checkVerbatim：首字符 `"` / 末字符 `"` /
//	                    含 `\` + 三位八进制 ⇒ EscapedNameError（**硬失败，不清洗**）。
//	C-6 调用点判据 ..... 由门承担；本件是它的白名单例外件（改本件须同批改例外注释）。
//	C-7 比对判据 ....... 件名比对只许在出口返回值（Entry）上进行，见 Entry.Equal。
//	C-8 形态照抄 ....... QuoteMode 是显式枚举（照 git quote.c 的 Octal / RespectConfig /
//	                    Verbatim），全包**无任何布尔开关**。
//	C-9 面覆盖 ......... FaceTracked / FaceTree / FaceDiff / FaceDiffCached / FaceStatus /
//	                    FaceGrep / FaceOthers 七类齐（FaceOthers = 未跟踪面，见下；
//	                    FaceDiffCached = 索引 vs HEAD，见该面注释）。
//	C-10 字节真值 ...... Entry.raw 以 []byte 承载，String() 只在末端派生；
//	                    execGit 全程不经过 string 转换。
//	C-11 pathspec 判据 ... 「pathspec 不存在」是**可判**的：WithErrorUnmatch 让 ls-files
//	                    两面的 argv 补 `--error-unmatch`，出口把 git 的 rc=1 翻成
//	                    ErrPathspecNotFound（判据 = errors.As(*exec.ExitError) + ExitCode()，
//	                    **不解析错误文本**）。谓词形态：IsPathspecMissing。
//	C-12 status 补面 .... FaceStatus 与 FaceTracked / FaceOthers / FaceDiffCached 同形吃
//	                    WithPrefix（pathspec 恒在 `--` 之后），并吃 WithUntrackedFiles(模式)
//	                    （`--untracked-files=<mode>`：**取枚举值的正交修饰**，不是无值开关
//	                    —— 见 C-8；恒拼在 pathspecSuffix **之前**）。★ XY 状态字母**不**由本面
//	                    出口：本包出口一律是「件名」（C-1/C-5/C-7/C-10），XY 属**另一类面**
//	                    （记录面），未立设计稿前不做 —— 禁顺手把字母塞进件名面。
//
// 机制真值（本机 Mr2109 / git 2.50.1 探针仓实测，见回执「A 段」）：
//   - `-z` 是**子命令选项**（`git ls-files -z`），不是全局选项：`git -z ls-files`
//     被 git 判为未知开关 ⇒ 因此 argv 的全局选项区只放 `-C` 与（应急时的）`-c`。
//   - `-z` 是**全免疫**（官方 "completely verbatim"）：带引号名 / 含 `\` 名 / 纯中文名
//     三类全真名；`-c core.quotepath=false` 只管非 ASCII 一类（含引号、反斜杠的
//     永远转义）⇒ `-c` 只能当应急分支（C-2）。
//   - `ls-files --others --exclude-standard`（FaceOthers 未跟踪面）属**会转义**面：
//     git 2.50.1 探针仓实测，不带 `-z` 时中文名实出 `"sub/\346\234\252…"`（首尾引号 +
//     `\` + 三位八进制），带 `-z` 时出真名逐字节 ⇒ 该面的 `-z` 是硬要求（C-2）。
package gitpaths

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// KQuotePath —— C-4：官方拼写，唯一保留的拼法（`core.quotepath` 不得再新增）。
const KQuotePath = "core.quotepath"

// kQuotePathOff —— 应急分支使用的值（C-2：`-c` 不是默认出口）。
const kQuotePathOff = "false"

// kErrorUnmatch —— 拼法照 C-4 的精神：git 的官方拼写只此一处（ls-files 的子命令选项）。
const kErrorUnmatch = "--error-unmatch"

// kUntrackedFiles —— C-4 同精神：git 的**长**拼写只此一处（`git status --untracked-files=<mode>`）。
// ★ 官方短形 `-u<mode>` 是**同义别名** ⇒ 本包不产出（禁第二拼法）。
const kUntrackedFiles = "--untracked-files"

// QuoteMode —— C-8：显式枚举，照 git 自己 quote.c 的三种形态。
// ★ 禁布尔开关（`verbatim bool` 这类形态一律不出现）。
type QuoteMode int

const (
	// QuoteVerbatim —— `-z` 出口（唯一默认档）：官方语义 "do not quote filenames"。
	QuoteVerbatim QuoteMode = iota
	// QuoteRespectConfig —— 应急分支：只在某面确实上不了 `-z` 时用，
	// 落 `-c core.quotepath=false`。★ 官方明说它**关不掉引号本身**（只管非 ASCII）
	// ⇒ 用完仍走 C-5 自检（自检是最后一道闸，不因换档而关）。
	QuoteRespectConfig
	// QuoteOctal —— git 的默认转义档（首尾引号 + `\` + 八进制）。虫族**不产出**该档：
	// 显式请求即报错，而不是静默降级（C-8 枚举三值齐，但只用 Verbatim）。
	QuoteOctal
)

// String —— 只用于错误信息，不参与任何判定。
func (m QuoteMode) String() string {
	switch m {
	case QuoteVerbatim:
		return "Verbatim(-z)"
	case QuoteRespectConfig:
		return "RespectConfig(-c core.quotepath=false)"
	case QuoteOctal:
		return "Octal(转义形态·虫族不产出)"
	}
	return fmt.Sprintf("QuoteMode(%d)", int(m))
}

// Face —— C-9：取件名的七类面，显式枚举（一次一面，不是一个布尔开关）。
type Face int

const (
	// FaceTracked —— `git ls-files -z`：索引里的已跟踪件（含暂存改动）。
	// ★ C-11：与 FaceOthers 同吃 WithErrorUnmatch（补 `--error-unmatch`）。
	FaceTracked Face = iota
	// FaceTree —— `git ls-tree -r -z --name-only <rev>`：某次提交的树（默认 HEAD）。
	FaceTree
	// FaceDiff —— `git diff --name-only -z [<rev>]`：默认 = 工作树 vs 索引（U-2 口径）。
	FaceDiff
	// FaceStatus —— `git status --porcelain -z`：默认 = 工作树 vs HEAD。
	// ★ C-12（A1 补面五）：本面与 FaceTracked / FaceOthers / FaceDiffCached 同吃
	//   WithPrefix（pathspec 恒在 `--` 之后），并吃 WithUntrackedFiles(模式)
	//   （`--untracked-files=<mode>`：取枚举值的正交修饰，非无值开关，恒在 pathspec 之前）。
	// ★ 本面**不吃** WithErrorUnmatch（git status 没有该选项 ⇒ 硬失败）：故「不命中的
	//   pathspec」在本面仍是 git 的**静默 rc=0 / 0 件**（探针仓实测）—— C-11 判据本面给不了，
	//   这是本面既定的窄口径，不拿别的面的选项冒充。
	// ★ 本面出口仍是**逐行件名**（XY 按协议剥掉，见 stripFacePrefix）；XY 字母**不**从本面
	//   引出（那属另一类「记录面」，须另立设计稿）—— 禁把协议字母混进件名面。
	FaceStatus
	// FaceGrep —— `git grep -z -l -e <模式> [<rev>]`：命中的件名。
	FaceGrep
	// FaceOthers —— `git ls-files --others --exclude-standard -z`：**未跟踪**件
	// （工作树里有、索引里没有，且未被 .gitignore 等排除源吃掉）。
	// ★ 与 FaceTracked 是**互补面**、不是同面的档位：索引面取不到未跟踪件，
	// 未跟踪面取不到索引件 ⇒ 两者不许互替（互替 = 静默错，同 C-5 拒清洗一条道理）。
	// ★ 本面**会转义**（不带 -z 时非 ASCII 件名照 git 默认档出 `"…\346\234…"`），
	// 故 `-z` 是本面的**硬要求**而非风格（探针仓实测见包内注释 / 回执「D 段」）。
	// ★ C-11：本面与 FaceTracked 同吃 WithErrorUnmatch —— 探针仓实测：件在 ⇒ rc=0，
	//   件不在 ⇒ rc=1（与索引面同形：`--others` 不改变 `--error-unmatch` 的判据）。
	FaceOthers
	// FaceDiffCached —— `git diff --cached --name-only -z`：**索引 vs HEAD**
	// （= 已 `git add` 进索引、尚未提交的那一档）。
	// ★ 与 FaceDiff 是**不同的比较对**、不是同面的档位：FaceDiff = 工作树 vs 索引。
	// ★ 本面**没有 rev 槽**：`--cached` 是**子命令选项**、不是 rev（仓内 4 处真实调用点的
	//   argv 逐字为 `<git> -c core.quotepath=false diff --cached --name-only`，无 rev）；
	//   而 `git diff --cached <rev>` 的另一义是「索引 vs <rev>」⇒ 塞 rev 即静默混义
	//   ⇒ 给了 WithRev 一律硬失败（见 args；与 C-5「宁可硬失败，不清洗」同一道理）。
	// ★ 与 FaceTracked / FaceOthers 同吃 WithPrefix（pathspec 放 `--` 之后）。
	// ★ 与 FaceDiff 的 argv 只差一个 `--cached` ⇒ String() 带上它才分得清报错来路
	//   （与 FaceOthers / FaceTracked 的关系同形）。
	// ★ 置尾追加 ⇒ 既有面值不变（零回归）。
	FaceDiffCached
)

// String —— 只用于错误信息。
func (f Face) String() string {
	switch f {
	case FaceTracked:
		return "ls-files"
	case FaceTree:
		return "ls-tree"
	case FaceDiff:
		return "diff"
	case FaceStatus:
		return "status"
	case FaceGrep:
		return "grep"
	case FaceOthers:
		// 与 FaceTracked 同子命令（ls-files）⇒ 串里带上 `--others` 才分得清两面的报错来路。
		return "ls-files --others"
	case FaceDiffCached:
		// 与 FaceDiff 同子命令（diff）⇒ 串里带上 `--cached` 才分得清两面的报错来路。
		return "diff --cached"
	}
	return fmt.Sprintf("Face(%d)", int(f))
}

// Entry —— 一件名。C-10：内部以**字节**承载，字符串/平台类型只在末端派生。
type Entry struct {
	raw []byte
}

// Bytes —— 返回**副本**：出口后的真值不许被下游就地改写。
func (e Entry) Bytes() []byte {
	out := make([]byte, len(e.raw))
	copy(out, e.raw)
	return out
}

// String —— 末端派生（唯一把字节变成平台字符串的地方）。
func (e Entry) String() string { return string(e.raw) }

// Equal —— C-7：比对只许发生在出口返回值上（逐字节，不走平台字符串归一）。
func (e Entry) Equal(o Entry) bool { return bytes.Equal(e.raw, o.raw) }

// IsZero —— 空件名（不变量：List 只会返回非空件名）。
func (e Entry) IsZero() bool { return len(e.raw) == 0 }

// Option —— 显式选项（禁布尔开关：每个可变量都有自己的具名构造子）。
type Option func(*config) error

// pathspecStrict —— C-8 同精神：内部**显式枚举**（全包无布尔字段）。
// 零值 = 默认档 pathspecLenient ⇒ 不调 WithErrorUnmatch 时 argv 与旧行为逐字相同。
type pathspecStrict int

const (
	// pathspecLenient —— 默认档：pathspec 不命中 ⇒ git 静默 rc=0 / 0 件（旧行为）。
	pathspecLenient pathspecStrict = iota
	// pathspecMustMatch —— 严档：argv 补 `--error-unmatch` ⇒ 不命中 git rc=1，
	// 出口翻成 ErrPathspecNotFound（**只 ls-files 两面吃**，见 acceptsErrorUnmatch）。
	pathspecMustMatch
)

// UntrackedFilesMode —— C-12/C-8：`--untracked-files=<mode>` 是**取枚举值的正交修饰**
// （与 rev / pathspec / pattern / 编码同族），**不是**无值布尔开关 ⇒ 可以做选项。
// ★ 官方三档（git-status.adoc：no / normal / all），三档都收；零值 = 默认档 = **不传**
// ⇒ 不调 WithUntrackedFiles 时 argv 与旧行为**逐字相同**（零回归）。
// ★ 裸 `--untracked-files`（无值形态，等价 all）**不产出**：那是 C-8 明禁的开关形态
// （探针仓实测：裸形态 = all 的 5 段，但它的语义靠“缺省值”承担 ⇒ 不给它出口）。
type UntrackedFilesMode int

const (
	// UntrackedFilesDefault —— 零值：**不传**该旋钮（git 自己的默认 = normal）。
	UntrackedFilesDefault UntrackedFilesMode = iota
	// UntrackedFilesNormal —— `--untracked-files=normal`：未跟踪**目录**折叠成一条
	// （探针仓实测：与不传该旋钮的输出逐字节相同 ⇒ 显式化默认值不改行为）。
	UntrackedFilesNormal
	// UntrackedFilesAll —— `--untracked-files=all`：未跟踪目录**展开**成逐件
	// （探针仓实测：4 段 66 B ⇒ 5 段 100 B，`未跟踪目录/` 变成 `未跟踪目录/一.txt` + `二.txt`）。
	UntrackedFilesAll
	// UntrackedFilesNo —— `--untracked-files=no`：**不列**未跟踪件
	// （探针仓实测：4 段 66 B ⇒ 2 段 27 B，只剩已跟踪的改动）。
	UntrackedFilesNo
)

// String —— 只用于错误信息，不参与任何判定。
func (m UntrackedFilesMode) String() string {
	switch m {
	case UntrackedFilesDefault:
		return "default(不传)"
	case UntrackedFilesNormal:
		return "normal"
	case UntrackedFilesAll:
		return "all"
	case UntrackedFilesNo:
		return "no"
	}
	return fmt.Sprintf("UntrackedFilesMode(%d)", int(m))
}

// argValue —— 官方的档值串（只此一处拼写）。零值（默认档）返回 false ⇒ 不进 argv。
func (m UntrackedFilesMode) argValue() (string, bool) {
	switch m {
	case UntrackedFilesNormal:
		return "normal", true
	case UntrackedFilesAll:
		return "all", true
	case UntrackedFilesNo:
		return "no", true
	}
	return "", false
}

type config struct {
	rev      string         // FaceTree / FaceGrep 的树或版本
	prefixes []string       // FaceTracked / FaceOthers 的 pathspec 前缀（0..N 段，按调用序）
	pattern  string         // FaceGrep 的模式
	mode     QuoteMode      // C-8：模式走枚举
	strict   pathspecStrict // C-11：pathspec 匹配契约（枚举，非布尔）
	// untracked —— C-12：status 面的未跟踪件档（枚举，非布尔；零值 = 不传该旋钮）。
	untracked UntrackedFilesMode
}

// WithRev —— 指定树/版本（FaceTree 默认 HEAD）。
func WithRev(rev string) Option {
	return func(c *config) error {
		if strings.TrimSpace(rev) == "" {
			return errors.New("gitpaths: WithRev 给了空 rev")
		}
		c.rev = rev
		return nil
	}
}

// WithPrefix —— FaceTracked / FaceOthers / FaceDiffCached 的路径前缀（作为 pathspec 放在 `--` 之后，免选项歧义）。
//
// ★ A1 补面甲：**可给多段** pathspec。首参 + `more...` 按调用序原样落到 `--` 之后，
// `--` 只出现一次（git 自己把 `--` 之后的段一律当 pathspec、不再当选项）。
// 三态兼容性：① 零段（不调用本选项）⇒ argv 里不出现 `--`（与旧行为逐字相同）；
// ② 一段 ⇒ `-- <p>`（与旧单参行为逐字相同，向后兼容，旧调用点无需改）；
// ③ N 段 ⇒ `-- <p1> … <pN>`。
// 任一段为空串 ⇒ 硬失败（**不静默跳过**：空 pathspec 会被 git 当整仓 ⇒ 静默放大面）。
func WithPrefix(prefix string, more ...string) Option {
	return func(c *config) error {
		all := make([]string, 0, 1+len(more))
		all = append(all, prefix)
		all = append(all, more...)
		for i, p := range all {
			if p == "" {
				return fmt.Errorf("gitpaths: WithPrefix 第 %d 段给了空前缀（N 段 pathspec 不许夹空串）", i+1)
			}
		}
		c.prefixes = append([]string(nil), all...)
		return nil
	}
}

// WithPattern —— FaceGrep 必填的模式。
func WithPattern(pattern string) Option {
	return func(c *config) error {
		if pattern == "" {
			return errors.New("gitpaths: WithPattern 给了空模式")
		}
		c.pattern = pattern
		return nil
	}
}

// WithErrorUnmatch —— C-11：给 argv 补 `--error-unmatch`，把「pathspec 不存在」从
// **静默 rc=0 / 0 件**变成**可判的硬失败**（ErrPathspecNotFound / IsPathspecMissing）。
//
// 位次铁律：子命令选项区**末尾**、`--` **之前**（= 紧贴 `--` 的子命令一侧）。
// ★ 落到 `--` 之后会被 git 当 pathspec（探针仓实测 `ls-files -z -- --error-unmatch`
// ⇒ rc=0、0 字节：静默失效）⇒ errorUnmatchTail 恒在 pathspecSuffix 之前拼。
//
// 吃它的面：**只有 ls-files 系列两面**（FaceTracked / FaceOthers）—— 探针仓实测：
// 存在件 ⇒ rc=0（不误伤），不存在件 ⇒ rc=1；**无前缀**时也合法（rc=0、照常列全件）。
// 其余五面（ls-tree / diff / diff --cached / status / grep）的 git 子命令**没有**这个
// 选项 ⇒ 给了一律**硬失败**（禁静默丢：静默丢弃会让调用点以为已开严档而其实没有）。
func WithErrorUnmatch() Option {
	return func(c *config) error {
		c.strict = pathspecMustMatch
		return nil
	}
}

// WithUntrackedFiles —— C-12：给 status 面的 argv 补 `--untracked-files=<mode>`。
//
// ★ 定性（C-8）：它是**取枚举值的正交修饰**（官方拼写 `--untracked-files[=<mode>]`，mode ∈
// no / normal / all）—— 与 rev / pathspec / pattern / 编码同族 ⇒ **可以做选项**；
// 而**无值形态**（裸 `--untracked-files`，等价 all）属 C-8 禁的「无值布尔开关」⇒ 本包不产出
// （探针仓实测裸形态 = 5 段 100 B，与 `=all` 逐字节相同；靠“缺省值”承担语义的旋钮不给出口）。
//
// 位次铁律：子命令选项区**末尾**、`--` 与 pathspec **之前**。
// ★ 落到 `--` 之后会被 git 当 pathspec ⇒ **静默失效**（探针仓实测 `status --porcelain -z --
// --untracked-files=all` ⇒ rc=0、**0 字节**；`… -z -- <前缀> --untracked-files=no` ⇒ rc=0、
// 仍照旧列出未跟踪件）⇒ untrackedFilesTail 恒在 pathspecSuffix 之前拼。
//
// 吃它的面：**只有 status 面**（FaceStatus）—— 其余六面的 git 子命令**没有**这个旋钮
// ⇒ 给了一律**硬失败**（禁静默丢：静默丢弃会让调用点以为已调档而其实还是默认档）。
//
// 三态：① 不传（零值 UntrackedFilesDefault）⇒ argv 里**不出现**该旋钮（与旧行为逐字相同）；
// ② UntrackedFilesAll ⇒ `--untracked-files=all`；③ UntrackedFilesNo ⇒ `--untracked-files=no`。
func WithUntrackedFiles(m UntrackedFilesMode) Option {
	return func(c *config) error {
		switch m {
		case UntrackedFilesDefault, UntrackedFilesNormal, UntrackedFilesAll, UntrackedFilesNo:
			c.untracked = m
			return nil
		}
		return fmt.Errorf("gitpaths: 未知 UntrackedFilesMode %d（C-8：只认 default/normal/all/no）", int(m))
	}
}

// WithQuoteMode —— C-2/C-8：默认 QuoteVerbatim；只有确实上不了 `-z` 的面才允许
// QuoteRespectConfig（应急）；QuoteOctal 直接拒。
func WithQuoteMode(m QuoteMode) Option {
	return func(c *config) error {
		switch m {
		case QuoteVerbatim, QuoteRespectConfig:
			c.mode = m
			return nil
		case QuoteOctal:
			return fmt.Errorf("gitpaths: 拒绝 QuoteOctal（C-8 虫族只用 Verbatim；转义形态不得流出出口）")
		}
		return fmt.Errorf("gitpaths: 未知 QuoteMode %d", int(m))
	}
}

// gitRunner —— 跑 git 的唯一形状：吃 argv、吐**字节**（C-10：不经过 string）。
type gitRunner func(argv []string) ([]byte, error)

// gitRunnerImpl 是唯一真跑 git 的入口，也是**测试注入点**（生产恒为 execGit）：
// 让判据格能在不跑 git 的前提下对切分与 C-5 自检发牙齿（见 gitpaths_test.go）。
var gitRunnerImpl gitRunner = execGit

// List —— 本包**唯一**的导出查询函数（C-1）。返回的一律是已过 C-5 自检的真名。
//
// 面（C-9）：FaceTracked / FaceTree / FaceDiff / FaceDiffCached / FaceStatus / FaceGrep / FaceOthers。
// 顺序即 git 自己的输出顺序（本包不排序、不去重：排序属调用点语义，出口只保真）。
func List(root string, face Face, opt ...Option) ([]Entry, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("gitpaths: root 不能为空（C-1 出口必须指到具体仓）")
	}
	cfg := config{mode: QuoteVerbatim}
	for _, o := range opt {
		if o == nil {
			continue
		}
		if err := o(&cfg); err != nil {
			return nil, err
		}
	}
	argv, err := buildArgv(root, face, &cfg)
	if err != nil {
		return nil, err
	}
	// C-3：`-c` 的位置是判据，不是风格 —— 放错会静默走差路（diff 退 128、
	// ls-files 把 -c 当 --cached 退 0 但 0 行），所以出 argv 后立刻复查。
	if err := assertGlobalOptionsFirst(argv); err != nil {
		return nil, err
	}
	out, err := gitRunnerImpl(argv)
	if err != nil {
		// C-11：只有调用点**显式开了严档**时，「pathspec 不存在」才作可判错误上报。
		// 未开严档时 git 对不存在的 pathspec 是静默 rc=0 / 0 件，此时把任何失败都
		// 当「不存在」纯属凭空发明判据 ⇒ 这里以 cfg.strict 为闸。
		if cfg.strict == pathspecMustMatch {
			if code, ok := exitCodeOf(err); ok && code == kExitPathspecUnmatched {
				return nil, fmt.Errorf(
					"gitpaths: %s 面 %w（git rc=%d ⇒ 该 pathspec 没命中任何件；不带 `--error-unmatch` 时是静默 rc=0 / 0 件）: %w",
					face, ErrPathspecNotFound, code, err)
			}
		}
		return nil, fmt.Errorf("gitpaths: %s 面取件名失败: %w", face, err)
	}
	raws, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(raws))
	for i, r := range raws {
		name := face.stripFacePrefix(r) // 面的协议前缀（status 的 XY）先按协议解，再自检
		if err := checkVerbatim(name); err != nil {
			return nil, fmt.Errorf("gitpaths: 第 %d 件 %w", i, err)
		}
		entries = append(entries, Entry{raw: name})
	}
	return entries, nil
}

// kExitPathspecUnmatched —— ls-files 在 `--error-unmatch` 下「pathspec 没命中」的退出码。
// ★ 官方口径（git-ls-files.adoc「--error-unmatch」）：任一 <file> 不在索引里 ⇒ 当错误（退 1）。
// ★ 探针仓实测（git 2.50.1）：存在件 ⇒ rc=0；不存在件 ⇒ rc=1。
// ★ **只认 1**：git 的其它失败形态不走这个码（未知开关 129 / 非仓 128）
// ⇒ 拿它们当「pathspec 不存在」是伸手过界，本件不给这种判据（宁窄不宽）。
const kExitPathspecUnmatched = 1

// exitCodeOf —— C-11 的**唯一**判据来源：errors.As 到 *exec.ExitError 再取 ExitCode()。
// ★ **不解析 stderr 文本**：文本是给人看的，git 改一句文案就会把判据掀掉。
// ★ 零签名改动：execGit 的 error 早已用 `%w` 包过 *exec.ExitError ⇒ 这里 errors.As 直接穿透。
func exitCodeOf(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), true
	}
	return 0, false
}

// ErrPathspecNotFound —— C-11：git 判「pathspec 没命中任何件」的**可判**标记。
// 判法：errors.Is(err, ErrPathspecNotFound)（或直接用 IsPathspecMissing）。
// ★ 只有调用点开了 WithErrorUnmatch 的面才产它 —— 未开严档时的「静默 rc=0 / 0 件」
// 是旧接口的既定行为，不是错误，不该被翻成它（否则消费侧会莫名拿到「不存在」）。
//
// ★ 原始 error 仍留在链上：errors.As(err, &ee) 取 *exec.ExitError 照旧可用（不吞细节）。
var ErrPathspecNotFound = errors.New("gitpaths: pathspec 不存在（--error-unmatch 判据：git rc=1）")

// IsPathspecMissing —— C-11 的**谓词**形态（窄函数，**不是第二个查询主入口**：
// 它不碰件名、不跑 git、不返回 []Entry，只回答「这个 err 是不是出口报的 pathspec 不存在」）。
// ★ 判定**唯一发生在 List 里**（errors.As + ExitCode()==1）；本谓词不做二次判定 ——
// 若在这里再嗅 *exec.ExitError，消费侧就能对「没开严档的面」凭空得到该判据。
func IsPathspecMissing(err error) bool { return errors.Is(err, ErrPathspecNotFound) }

// buildArgv —— 全局选项区（`-C <root>` + 应急时的 `-c`）+ 子命令 argv。
// C-2/C-3：`-c` 若出现，恒在这一段、恒在子命令之前。`-z` 属子命令选项（见包注释）。
func buildArgv(root string, face Face, cfg *config) ([]string, error) {
	head := []string{"-C", root}
	if cfg.mode == QuoteRespectConfig {
		head = append(head, "-c", KQuotePath+"="+kQuotePathOff) // C-4：只有官方拼写
	}
	sub, err := face.args(cfg)
	if err != nil {
		return nil, err
	}
	return append(head, sub...), nil
}

// pathspecSuffix —— 把 config 里的 0..N 段前缀拼成 `-- <p1> … <pN>`。
// ★ 零段 ⇒ nil（argv 里**不出现** `--`，与旧行为逐字相同）；
//
//	N 段（N≥1）⇒ `--` 恰出现一次、其后按调用序跟 N 段。
//
// 用途：FaceTracked / FaceOthers 两面同吃（两面都是 pathspec 收件面）。
func pathspecSuffix(cfg *config) []string {
	if len(cfg.prefixes) == 0 {
		return nil
	}
	out := make([]string, 0, len(cfg.prefixes)+1)
	out = append(out, "--")
	out = append(out, cfg.prefixes...)
	return out
}

// errorUnmatchTail —— C-11：`--error-unmatch` 的插入位（子命令选项区**末尾**）。
// ★ 零代价默认：不调 WithErrorUnmatch ⇒ 返回 nil ⇒ argv 与旧行为**逐字相同**。
// ★ 位次铁律：恒在 `pathspecSuffix` **之前**拼（`--` 之后的段 git 一律当 pathspec）。
func errorUnmatchTail(cfg *config) []string {
	if cfg.strict != pathspecMustMatch {
		return nil
	}
	return []string{kErrorUnmatch}
}

// acceptsErrorUnmatch —— C-11：哪些面吃 `--error-unmatch`：**只有 ls-files 系列两面**。
// ★ 判据是「git 该子命令有没有这个选项」，不是「我们喜不喜欢」：
// ls-files 有（--error-unmatch：任一 <file> 不在索引里 ⇒ 退 1）；
// ls-tree / diff / status / grep 没有 ⇒ 硬失败（入口闸在 args 顶部，禁静默丢）。
func (f Face) acceptsErrorUnmatch() bool {
	return f == FaceTracked || f == FaceOthers
}

// untrackedFilesTail —— C-12：`--untracked-files=<mode>` 的插入位（子命令选项区**末尾**）。
// ★ 零代价默认：不调 WithUntrackedFiles（或给零值档）⇒ 返回 nil ⇒ argv 与旧行为**逐字相同**。
// ★ 位次铁律：恒在 `pathspecSuffix` **之前**拼（`--` 之后的段 git 一律当 pathspec ⇒ 旋钮静默失效）。
func untrackedFilesTail(cfg *config) []string {
	v, ok := cfg.untracked.argValue()
	if !ok {
		return nil
	}
	return []string{kUntrackedFiles + "=" + v}
}

// acceptsUntrackedFiles —— C-12：哪些面吃 `--untracked-files=<mode>`：**只有 status 面**。
// ★ 判据同 acceptsErrorUnmatch（「git 该子命令有没有这个选项」，不是「我们喜不喜欢」）：
// 官方 git-status.adoc 有 no/normal/all；ls-files / ls-tree / diff / grep **没有**
// ⇒ 硬失败（入口闸在 args 顶部，禁静默丢）。FaceDiffCached（`diff --cached`）同样没有。
func (f Face) acceptsUntrackedFiles() bool {
	return f == FaceStatus
}

// args —— C-2：每一面的 argv 逐条写死在这里，七类面**全部**带 `-z`。
func (f Face) args(cfg *config) ([]string, error) {
	// C-11 入口闸：严档只有 ls-files 两面吃；其余面（含未知面值）**硬失败**，
	// 不许把选项静默丢掉（静默丢 = 调用点以为开了严档，实际仍是静默 rc=0 的面）。
	if cfg.strict == pathspecMustMatch && !f.acceptsErrorUnmatch() {
		return nil, fmt.Errorf("gitpaths: %s 面不吃 `--error-unmatch`（该选项只属 ls-files 系列：FaceTracked / FaceOthers）⇒ 硬失败，不静默丢", f)
	}
	// C-12 入口闸：未跟踪件档只有 status 面吃；其余面（含未知面值）**硬失败**，
	// 不许把旋钮静默丢掉（静默丢 = 调用点以为已调档，实际仍是默认档）。
	if cfg.untracked != UntrackedFilesDefault && !f.acceptsUntrackedFiles() {
		return nil, fmt.Errorf("gitpaths: %s 面不吃 `--untracked-files=<mode>`（该旋钮只属 status 面：FaceStatus）⇒ 硬失败，不静默丢", f)
	}
	switch f {
	case FaceTracked:
		// `ls-files -z`；pathspec（0..N 段）放在 `--` 之后（见 pathspecSuffix）。
		a := []string{"ls-files", "-z"}
		// C-11：严档时补 `--error-unmatch` —— 位次在 `--` 之前（见 errorUnmatchTail）。
		a = append(a, errorUnmatchTail(cfg)...)
		a = append(a, pathspecSuffix(cfg)...)
		return a, nil
	case FaceOthers:
		// 未跟踪面：`ls-files --others --exclude-standard -z`。
		// ★ `-z` 必须在**子命令之后**（与 FaceTracked 同形；`-z` 不是全局选项，
		//   `git -z ls-files` 被 git 判未知开关 —— 见包注释「机制真值」）。
		// ★ 本面**会转义**：不带 `-z` 时非 ASCII 件名照 git 默认档出
		//   `"…\346\234…"`（探针仓实测）⇒ 该面必须带 `-z`，否则 C-5 自检必命中。
		// ★ `--exclude-standard` 是**面的定义**的一部分（照 .gitignore / 全局排除源），
		//   不是可选项：少了它，「未跟踪」会退化成「连被忽略的垃圾也算件名」。
		a := []string{"ls-files", "--others", "--exclude-standard", "-z"}
		// C-11：本面也吃 `--error-unmatch`（探针仓实测：件在 ⇒ rc=0，件不在 ⇒ rc=1）；
		// 位次同上：子命令选项区末尾、`--` 之前。
		a = append(a, errorUnmatchTail(cfg)...)
		// 与 FaceTracked 同吃前缀（0..N 段）：作 pathspec 放在 `--` 之后（免选项歧义）。
		a = append(a, pathspecSuffix(cfg)...)
		return a, nil
	case FaceTree:
		// §2.4 订正：`ls-tree` **有** `-z`（官方 ls-tree.adoc：output verbatim）。
		rev := cfg.rev
		if rev == "" {
			rev = "HEAD"
		}
		return []string{"ls-tree", "-r", "-z", "--name-only", rev}, nil
	case FaceDiff:
		a := []string{"diff", "--name-only", "-z"}
		if cfg.rev != "" {
			a = append(a, cfg.rev)
		}
		return a, nil
	case FaceDiffCached:
		// A1 补面三：`diff --cached --name-only -z` = **索引 vs HEAD**（已 add 未提交的那一档）。
		// ★ `--cached` 是**子命令选项**、不是 rev：仓内 4 处真实调用点的 argv 逐字为
		//   `git -c core.quotepath=false diff --cached --name-only`（无 rev、`--cached` 紧贴 `diff`）
		//   ⇒ 本面**不开 rev 槽**（`git diff --cached <rev>` 的另一义 = 索引 vs <rev>，属另一比较对）
		//   ⇒ 静默吃 rev = 静默混义，故一律硬失败（同 C-5：宁可硬失败，不猜）。
		// ★ `-z` 恒在子命令之后（C-2）。
		if cfg.rev != "" {
			return nil, errors.New("gitpaths: FaceDiffCached 不吃 WithRev（本面 = 索引 vs HEAD；`git diff --cached <rev>` 是「索引 vs <rev>」，属另一比较对）")
		}
		a := []string{"diff", "--cached", "--name-only", "-z"}
		// 与 FaceTracked / FaceOthers 同吃前缀（0..N 段）：作 pathspec 放 `--` 之后（免选项歧义）。
		a = append(a, pathspecSuffix(cfg)...)
		return a, nil
	case FaceStatus:
		// A1 补面五·甲：与 FaceTracked / FaceOthers / FaceDiffCached 同形地吃 pathspec
		// （0..N 段，放 `--` 之后 —— 免选项歧义）。★ 探针仓实测：`status --porcelain -z
		// -- <前缀>` rc=0，且只出命中件（中文名件 1 段 18 B / 目录前缀 1 段 20 B /
		// N 段 2 段 38 B，基线 4 段 66 B）⇒ 位次正确。
		// A1 补面五·乙：`--untracked-files=<mode>` 恒在 pathspecSuffix **之前**
		// （见 untrackedFilesTail；放 `--` 之后 ⇒ 探针实测静默失效）。
		a := []string{"status", "--porcelain", "-z"}
		a = append(a, untrackedFilesTail(cfg)...)
		a = append(a, pathspecSuffix(cfg)...)
		return a, nil
	case FaceGrep:
		if cfg.pattern == "" {
			return nil, errors.New("gitpaths: grep 面必须给 WithPattern（C-9 五类面之一）")
		}
		a := []string{"grep", "-z", "-l", "-e", cfg.pattern}
		if cfg.rev != "" {
			a = append(a, cfg.rev)
		}
		return a, nil
	}
	return nil, fmt.Errorf("gitpaths: 未知面 %v（C-9 只认六类）", f)
}

// assertGlobalOptionsFirst —— C-3 的机检：任何 `-c` 落在子命令之后 ⇒ error。
// `--` 之后的段是 pathspec（git 自己不当选项），故遇到 `--` 即停。
func assertGlobalOptionsFirst(argv []string) error {
	i := 0
	for i < len(argv) {
		switch argv[i] {
		case "-C", "-c":
			i += 2
			continue
		}
		if strings.HasPrefix(argv[i], "-") {
			i++
			continue
		}
		break
	}
	if i >= len(argv) {
		return errors.New("gitpaths: argv 里没有子命令")
	}
	sub := argv[i]
	for j := i + 1; j < len(argv); j++ {
		if argv[j] == "--" {
			break
		}
		if argv[j] == "-c" {
			return fmt.Errorf("gitpaths: C-3 违规 —— `-c` 落在子命令 %q 之后（放错位置会静默走差路）", sub)
		}
	}
	return nil
}

// splitNUL —— C-2：`-z` 出口一律按 `\0` 切分。
// git 用 `\0` 收尾 ⇒ 末尾必多一个空段（去掉）；**中间**出现空段 ⇒ 硬失败
// （空段只可能来自自己解析错，静默丢弃会让件名静默少一件）。
func splitNUL(out []byte) ([][]byte, error) {
	if len(out) == 0 {
		return nil, nil
	}
	parts := bytes.Split(out, []byte{0})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	raws := make([][]byte, 0, len(parts))
	for i, p := range parts {
		if len(p) == 0 {
			return nil, fmt.Errorf("gitpaths: 第 %d 段为空（`\\0` 连缀）⇒ 拒收", i)
		}
		raws = append(raws, p)
	}
	return raws, nil
}

// stripFacePrefix —— 面的**协议**前缀（只 status 面有）。
//
// `status --porcelain -z` 的每一段不是裸件名，而是 `XY <path>`（前 3 字节 = 两枚状态码
// + 一个空格）；重命名/复制在 `-z` 下多出一段 **rename 源件**（该段没有状态码前缀）。
// ⇒ 有协议前缀的段剥 3 字节，无前缀的段原样收（它是源件，同样是件名）。
//
// ★ 这与 C-5 的「禁止静默清洗」**不冲突**：C-5 禁的是清洗 git 的**转义形态**；
// 状态码是本面的输出协议（git-status 官方在 `-z` 下逐字节定义），必须按协议解。
// ★ 只剥**确认是** `XY ` 形态的段；形态不符则原样送去 C-5 自检（宁可硬失败，不猜）。
func (f Face) stripFacePrefix(raw []byte) []byte {
	if f != FaceStatus || len(raw) < 3 || raw[2] != ' ' {
		return raw
	}
	if !isStatusCodeByte(raw[0]) || !isStatusCodeByte(raw[1]) {
		return raw
	}
	return raw[3:]
}

// isStatusCodeByte —— porcelain v1 的 XY 取值集合。
func isStatusCodeByte(b byte) bool {
	return b == ' ' || b == 'M' || b == 'T' || b == 'A' || b == 'D' ||
		b == 'R' || b == 'C' || b == 'U' || b == 'X' || b == '?' || b == '!'
}

// EscapedNameError —— C-5 自检命中（**硬失败**）。原始件名一并带上：判因不靠猜。
type EscapedNameError struct {
	Reason string
	Raw    []byte
}

func (e *EscapedNameError) Error() string {
	return fmt.Sprintf("gitpaths: C-5 自检命中（%s）⇒ 转义形态不得流出出口：%s",
		e.Reason, string(e.Raw))
}

// checkVerbatim —— C-5：逐件名自检，命中即 error（**禁止静默清洗**：
// 去引号/反转义会产出「看着正常但错」的件名，比报错坏得多）。
func checkVerbatim(raw []byte) error {
	if len(raw) == 0 {
		return &EscapedNameError{Reason: "空件名", Raw: raw}
	}
	if raw[0] == '"' {
		return &EscapedNameError{Reason: `首字符为 "`, Raw: raw}
	}
	if raw[len(raw)-1] == '"' {
		return &EscapedNameError{Reason: `末字符为 "`, Raw: raw}
	}
	if hasOctalEscape(raw) {
		return &EscapedNameError{Reason: `含 \ + 三位八进制`, Raw: raw}
	}
	return nil
}

// hasOctalEscape —— `\` 后跟三位八进制（照 git quote.c 的转义形态）。
// ★ 注意口径：含**裸反斜杠**的合法件名（如 `b\s.txt`）不算命中 ——
// 命中面只认 `\` + 恰好三位八进制。
func hasOctalEscape(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if i+3 < len(raw) && isOctalByte(raw[i+1]) && isOctalByte(raw[i+2]) && isOctalByte(raw[i+3]) {
			return true
		}
	}
	return false
}

func isOctalByte(b byte) bool { return b >= '0' && b <= '7' }

// execGit —— 唯一真跑 git 的地方（C-10：只取字节面，全程不经过 string）。
// stderr 收进错误信息：失败不许无痕。
func execGit(argv []string) ([]byte, error) {
	cmd := exec.Command("git", argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w（stderr: %s）", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

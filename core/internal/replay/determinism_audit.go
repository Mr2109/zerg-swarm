// determinism_audit.go —— **非确定性源清单化审计**（T4.5；设计稿 §〇 E8 ★★ + E9 ★★）。
//
// ── 为什么要"清单化 + 可机检"（E8）──
//
// "我们的回放是确定的"这句话最常见的死法不是被人反驳，而是**没人能反驳**：没有清单，就没人知道
// 该检查哪几类源；没有机检，清单第一次改动就过期。E8 的处置原文是"逐项声明已冻结/未冻结，未覆盖
// 必须标红"。所以本文件做两件事：
//
//	① **十类源的声明表**（时间 / 随机 / 网络 / DNS / 文件系统 / 环境变量 / goroutine 调度 /
//	   map 迭代 / CPU 数 / locale）：每类给 状态（已冻结 / 未冻结 / 不适用）+ 冻结机制（可注入点）
//	   + 理由 + 禁模式 + **声明的命中数**；
//	② **扫描器**：对给定源码做词法扫描（去注释、保留字符串）+ AST 补充（禁 import 路径、
//	   range-over-map），命中"未注入的禁模式"即报红；报告里带 文件:行:模式:所属函数:该类状态。
//
// ── 六条写死的纪律 ──
//
//  1. **三态不许留空**：每一类都必须给状态；"未冻结"本身不违法，但会在报告里**恒定标红**（必须处置）。
//  2. **计数漂移即红**（AuditItem.ExpectHits）：声明的命中数与实际不符 ⇒ 红，且错误信息告诉人怎么处置。
//     这是词法/AST 检查"可能漏报"的补偿控制：漏报的是**新增**点，而新增点会让计数变。
//  3. **豁免要自证**：允许点（如 dispatch.go 里可注入 Now 的兜底分支、match.go 里 BaseDir 的兜底）
//     走显式豁免规则；扫描器会检查每条豁免**仍然命中至少一处** —— 不命中的豁免算"过期豁免"，
//     同样报红（否则豁免表会慢慢变成一张把所有命中都吞掉的免死牌）。
//     ⚠ 前提：豁免表与声明数是按**整包**登记的 ⇒ 只扫一个子集（测试里的植入源）时，
//     "其它文件的豁免没命中"与"计数变了"都会如实报红。要结论就扫整包。
//  4. **零输入即报错**：没有声明表、或没有源文件 ⇒ 报错退出，绝不"扫了个空"还打印通过。
//  5. **自引用陷阱**（本文件的实战教训）：禁模式是写在**本文件**里的字符串字面量；若模式的字面形态
//     与自己匹配（例如把 `time.Now(` 原样写成模式，或散文里写了 `LC_ALL=`），审计会永远红在一个假点上。
//     所以本文件里：模式的字面量一律写成**转义形态**（`time\.Now\(` —— 它匹配不了自己），
//     散文里不写 `<词>.` / `<词>(` / 裸的 LC_ALL 这类形态。与 scripts/check-placeholder-residue.py
//     里"本文件自身不得出现完整占位符形态"是同一条纪律。
//  6. **必要性不豁免、只计数**（AuditItem.CountOnly）：有些调用在本层是**必需且合法**的（读文件、建目录）。
//     把它们一律判红会逼出一堆"因为不得不做"的豁免 —— 而豁免表一旦开始为必要性开口子，就不再是证据。
//     所以它们进**计数**（漂移即红），不进禁模式：新增一处 IO 面照样必须过"改声明"这一关。
//
// ── 机检的能力边界（如实）──
//
//	· 词法层：抓"直接调用/直接引用"；抓不到间接调用（把取时函数存进字段再调用）。
//	· 禁 import：AST 判定，按**子树**匹配（禁 golang.org/x/text 即连 x/text/language 一起禁）。
//	· range-over-map：AST + 作用域，只对"能用语法证明是 map"的名字生效（跨包调用返回的 map 证明不了）。
//	· 以上三种漏报都由 ExpectHits 计数漂移兜住。
package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrDeterminismAudit —— 确定性审计不通过（专项错误码：errors.Is(err, ErrDeterminismAudit)）。
var ErrDeterminismAudit = errors.New("replay: 确定性审计不通过")

// SourceStatus —— 三态（写死，没有第四态）。
type SourceStatus string

const (
	// StatusFrozen —— 已冻结：有可注入点或纪律，且有对应的机检。
	StatusFrozen SourceStatus = "已冻结"
	// StatusUnfrozen —— 未冻结：**必须标红**（处置完才能宣称回放确定）。
	StatusUnfrozen SourceStatus = "未冻结"
	// StatusNA —— 不适用：本层根本不做这件事（同样要写清为什么，并保留机检防止以后有人加进来）。
	StatusNA SourceStatus = "不适用"
)

// AllowRule —— 显式豁免。三条硬要求：① 必须指定文件（不许全仓放行）；② 必须写理由；
// ③ 必须**仍然命中**（不命中 ⇒ 过期豁免 ⇒ 报红）。
//
// Pattern 与 Func 是两种定位方式（二选一）：
//   - Pattern：词法命中（与禁模式同形态即可，比较时按"去转义"归一）；
//   - Func：AST 命中（range-over-map 用"所属函数名"定位，比行号稳）。
type AllowRule struct {
	File    string `json:"file"`
	Pattern string `json:"pattern,omitempty"`
	Func    string `json:"func,omitempty"`
	Reason  string `json:"reason"`
}

// AuditItem —— 一类非确定性源的声明（E8："逐项声明"，不许留空）。
type AuditItem struct {
	ID         string       // ascii 短名（脚本 / JSON 用）
	Name       string       // 中文名
	Status     SourceStatus // 三态之一
	Mechanism  string       // 冻结手段（可注入点 / 纪律）——"不适用"也要写为什么
	Reason     string       // 为什么是这个状态
	Forbidden  []string     // 禁模式（regexp，扫**代码区**：去注释、保留字符串）——命中即红（除非有豁免）
	CountOnly  []string     // 只计数的模式（见文件头第 6 条）：合法但必须过声明这一关
	Imports    []string     // 禁 import 路径（AST，按子树匹配）——命中即红（无豁免）
	Allow      []AllowRule  // 显式豁免（必须仍命中，否则算过期）
	ExpectHits int          // 声明的命中数（-1 = 不检查）；实际 != 声明 ⇒ 计数漂移 ⇒ 红
	AST        string       // 该类是否有额外的 AST 检查（目前仅 map 迭代有）
}

// astMapRange —— AST 检查的种类名（range-over-map）。
const astMapRange = "map-range"

// mapRangePattern —— AST 命中的 Pattern 标签（脚本与报告里靠它区分词法命中与 AST 命中）。
const mapRangePattern = "for … := range <map>（AST）"

// AuditTable —— **十类非确定性源的声明表**（顺序照抄 E8 原文）。
//
// ExpectHits 怎么来的：**先跑一遍审计数出来，再写死**（不是先写数再改代码 —— 那是把声明凑成事实）。
// 它的含义是"这十类目前共命中 N 处，全部可在表里对上账"；以后任何新增使用点都会让 N 变 ⇒ 红。
var AuditTable = []AuditItem{
	{
		ID:     "time",
		Name:   "时间",
		Status: StatusFrozen,
		Mechanism: "Dispatcher.Now 注入（唯一取时点，可注入冻结时钟）；DurationMS 在回放期用录播原值；" +
			"匹配与指纹**不含**时间字段（DefaultNoiseFields 去噪 ts/timestamp/created_at/duration_ms 等）",
		Reason: "包内唯一的取时点在可注入的兜底分支里，且回放分支（ModeReplay）不经过它；" +
			"时间不参与匹配、不参与指纹 ⇒ 同输入必然同输出",
		Forbidden: []string{
			`time\.Now\(`, `time\.Since\(`, `time\.Until\(`, `time\.After\(`,
			`time\.NewTimer\(`, `time\.Tick\(`, `time\.NewTicker\(`, `time\.Sleep\(`,
		},
		Allow: []AllowRule{{
			File:    "dispatch.go",
			Pattern: `time\.Now\(`,
			Reason:  "可注入 Now 的兜底分支（d.Now != nil 时一律用注入值）：回放分支不经过它，且时间不参与匹配/指纹",
		}},
		ExpectHits: 1,
	},
	{
		ID:     "random",
		Name:   "随机",
		Status: StatusFrozen,
		Mechanism: "确定性 ID = DeterministicID(traceID, seq)（sha256 前 128 位，纯函数）；" +
			"回放路径禁 uuid / crypto/rand（E9 原文写死）",
		Reason: "包内零随机源：唯一「造新标识」的口是纯函数 DeterministicID —— " +
			"随机 ID 会让指标②（输出等价性）当场失效，且失败形态是「差几个十六进制字符」，极难归因",
		Forbidden:  []string{`\brand\.`, `NewUUID\(`, `RandString\(`, `randomBytes\(`},
		Imports:    []string{"crypto/rand", "math/rand", "math/rand/v2", "github.com/google/uuid"},
		ExpectHits: 0,
	},
	{
		ID:        "net",
		Name:      "网络",
		Status:    StatusFrozen,
		Mechanism: "本包不发任何网络请求；HTTP 拦截（RoundTripper / MITM）是**调用方**侧的双保险（E2），不在本包",
		Reason:    "零网络依赖：录播层只跟 trace 文件 + 调用方给的 ToolFunc 打交道 ⇒ 网络抖动进不了回放",
		Forbidden: []string{
			`net\.Dial`, `net\.Listen`, `http\.Get\(`, `http\.Post\(`, `http\.Do\(`,
			`http\.NewRequest`, `grpc\.Dial`, `websocket\.Dial`,
		},
		Imports:    []string{"net/http", "net/rpc", "google.golang.org/grpc", "github.com/gorilla/websocket"},
		ExpectHits: 0,
	},
	{
		ID:         "dns",
		Name:       "DNS",
		Status:     StatusNA,
		Mechanism:  "不适用：本层不做任何名字解析（没有网络、没有上游地址）",
		Reason:     "录播层只认文件与显式参数；仍保留机检（禁名字解析调用与自定义解析器）以防以后有人把解析塞进来",
		Forbidden:  []string{`net\.Lookup`, `net\.Resolver`, `net\.DefaultResolver`},
		ExpectHits: 0,
	},
	{
		ID:     "fs",
		Name:   "文件系统",
		Status: StatusFrozen,
		Mechanism: "路径一律来自调用方**显式参数**（TracePath / BaseDir / 产物目录）；" +
			"禁「从环境推断路径」（临时目录 / 切换工作目录 / HOME 与缓存目录 / 可执行文件位置 / 环境变量展开）；" +
			"产物摘要遇符号链接即报错（不跟随）、目录缺失即报错（不与「两边都缺」混同）",
		Reason: "文件 IO 消不掉（trace 就是文件），冻结的是**输入口径**：同一份显式输入 ⇒ 同一份字节；" +
			"不跟随链接、不按环境变量选路径 ⇒ 同一台机器上两次运行看到的文件集合一致",
		Forbidden: []string{
			`os\.TempDir`, `os\.MkdirTemp`, `os\.CreateTemp`, `os\.Chdir`, `os\.UserHomeDir`,
			`os\.UserCacheDir`, `os\.UserConfigDir`, `os\.Executable`, `os\.ExpandEnv`, `os\.Setenv`,
			`os\.Getwd\(`, `os\.Symlink\(`, `os\.Link\(`, `os\.Hostname\(`,
			`filepath\.EvalSymlinks`, `filepath\.Glob\(`, `filepath\.Abs\(`,
		},
		// 下面这一栏是"必需且合法"的那部分（见文件头第 6 条）：只计数、不判红。
		CountOnly: []string{
			`os\.ReadFile\(`, `os\.WriteFile\(`, `os\.OpenFile\(`, `os\.Open\(`, `os\.Create\(`,
			`os\.MkdirAll\(`, `os\.Mkdir\(`, `os\.Remove\(`, `os\.RemoveAll\(`, `os\.Rename\(`,
			`os\.Stat\(`, `os\.Lstat\(`, `os\.ReadDir\(`, `os\.Chmod\(`, `os\.Truncate\(`,
			`filepath\.WalkDir\(`, `filepath\.Walk\(`,
		},
		Allow: []AllowRule{{
			File:    "match.go",
			Pattern: `os\.Getwd\(`,
			Reason:  "BaseDir 为空时的兜底：这是**显式声明的降级**，且录制与回放必须同基准（不符会在错误里逐字段指出）",
		}},
		ExpectHits: 24, // 实测：1 处豁免（match.go 的 BaseDir 兜底）+ 23 处必要性调用
		//          （读/写/建目录/改名/stat；其中 4 处是 T4.3 的**效果账本本地文件档**：
		//           读账本 / 建账本目录 / 写临时件 / 改名为正式账本 —— 原子落盘本身就要这几步）
	},
	{
		ID:     "env",
		Name:   "环境变量",
		Status: StatusFrozen,
		Mechanism: "生产代码零环境读取：配置一律由调用方显式传参（trace.go 的纪律注释里已写死" +
			"「绝不读取环境变量做决定」）",
		Reason: "零容忍类：任一 Getenv / LookupEnv / Environ / ExpandEnv 都是红（**无豁免规则**）；" +
			"_test.go 不在默认扫描范围（夹具允许用环境变量指定落盘目录，见 AuditOptions.IncludeTests）",
		Forbidden:  []string{`os\.Getenv`, `os\.LookupEnv`, `os\.Environ`, `os\.ExpandEnv`, `syscall\.Getenv`},
		ExpectHits: 0,
	},
	{
		ID:     "goroutine",
		Name:   "goroutine 调度",
		Status: StatusFrozen,
		Mechanism: "包内**不派生任何 goroutine**（无 go 语句、无 Gosched、无 Sleep）；" +
			"并发只体现为 sync.Mutex 保护共享结构（服务并发调用方），不引入任何顺序依赖",
		Reason:     "没有派生就没有调度差异；互斥锁只保证「不会同时改」，不改变输出（凡输出都走稳定序）",
		Forbidden:  []string{`\bgo\s+[a-zA-Z_(]`, `runtime\.Gosched`, `sync\.WaitGroup`, `\bselect\s*\{`},
		ExpectHits: 0,
	},
	{
		ID:     "map",
		Name:   "map 迭代",
		Status: StatusFrozen,
		Mechanism: "AST 检查：凡 range 的对象是 map 必须逐处登记（AllowRule 带**所属函数**名与理由）；" +
			"所有对外输出/落盘/判重/比较一律先**排序**（sort.Strings / sort.Slice 稳定序）",
		Reason: "Go 的 map 迭代序随机 ⇒ 任何「按迭代序输出/判重」都是不确定源；" +
			"这是唯一不能靠词法判断的类，故用 AST 精确到作用域与函数，并用计数漂移守漏报",
		AST: astMapRange,
		Allow: []AllowRule{
			{File: "match.go", Func: "encoder.encodeMap", Reason: "键先收集再 sort.Strings（字节序）后才写进规范化结果 ⇒ 迭代序不影响输出"},
			{File: "match.go", Func: "compareFields", Reason: "先按并集收集键并 sort.Strings，再逐键比较 ⇒ 迭代序不影响差异报告"},
			{File: "match.go", Func: "flattenInto", Reason: "写入 map（该 map 只做键查询，不参与任何顺序敏感输出）"},
			{File: "effects.go", Func: "EffectLedger.Snapshot", Reason: "收集后 sort.Slice 稳定序 ⇒ 序列化与断言都与迭代序无关"},
			{File: "fidelity.go", Func: "mapKeys", Reason: "收集后 sort.Strings 返回稳定序键表 ⇒ 产物差异报告的次序固定"},
			{File: "determinism_audit.go", Func: "sortedKeys", Reason: "收集后 sort.Strings（扫描顺序也必须确定，否则报告次序会抖）"},
			{File: "determinism_audit.go", Func: "Auditor.AuditSources", Reason: "只做搬运（筛掉 _test.go 后写进另一个 map），次序不参与任何输出；真正的扫描次序走 sortedKeys"},
			{File: "invalidation.go", Func: "ToolTableFingerprint", Reason: "收集工具名后 sort.Strings 再摘要 ⇒ 工具表指纹与键序无关（这正是它要守的东西）"},
		},
		ExpectHits: 10,
	},
	{
		ID:         "cpu",
		Name:       "CPU 数",
		Status:     StatusFrozen,
		Mechanism:  "不读 NumCPU / 不设 GOMAXPROCS / 不按核数分片；无 worker 池 ⇒ 并发度不改变任何输出",
		Reason:     "零 CPU 探测：IO 与解析全单序列；调用方若并发调用，互斥锁只保证安全、不改变结果",
		Forbidden:  []string{`runtime\.NumCPU`, `runtime\.GOMAXPROCS`, `runtime\.NumGoroutine`},
		ExpectHits: 0,
	},
	{
		ID:     "locale",
		Name:   "locale",
		Status: StatusFrozen,
		Mechanism: "排序一律**字节序**（sort.Strings；脚本侧同一口径即 C locale）；" +
			"JSON 编解码按 UTF-8 字节；不引入 x/text 归一化",
		Reason:     "Go 标准库的字符串排序/大小写不依赖 locale；包内零 locale 读取（也不读语言/地区类环境变量）",
		Forbidden:  []string{`\bLANG\b`, `\bLC_ALL\b`, `\bLC_CTYPE\b`, `collate\.`, `cases\.`, `language\.`},
		Imports:    []string{"golang.org/x/text"},
		ExpectHits: 0,
	},
}

// DeterministicID —— 回放路径上的**确定性 ID**（E9 原文口径：确定性 ID = sha256(traceID ‖ seq)）。
//
// 为什么必须有它：uuid / crypto/rand 会让"同一份输入 + 同一份 trace"产出两套不同的字节 ⇒
// 指标②（输出等价性）当场失效，而失败会以"文件里差几个十六进制字符"的形态出现，极难归因。
// 本函数是包内唯一的"造新标识"口；**禁 uuid / 禁 crypto/rand** 由本文件的审计表守住。
// 取 128 位（32 个十六进制字符）：做标识足够，且短到可以进文件名。
func DeterministicID(traceID string, seq int) string {
	h := sha256.New()
	h.Write([]byte(traceID))
	h.Write([]byte{0x1f})
	h.Write([]byte(strconv.Itoa(seq)))
	return "zid:" + hex.EncodeToString(h.Sum(nil))[:32]
}

// ── 报告 ────────────────────────────────────────────────────────────────────

// Finding —— 一处命中。
type Finding struct {
	File    string     `json:"file"`
	Line    int        `json:"line"`
	Class   string     `json:"class"`
	Name    string     `json:"name"`
	Status  string     `json:"status"`
	Pattern string     `json:"pattern"`
	Text    string     `json:"text"`
	Allowed *AllowRule `json:"allowed,omitempty"`
	Red     bool       `json:"red"`
	Why     string     `json:"why,omitempty"`
}

// AuditItemRow —— 报告里的**一行**（十类各一行 —— 这就是"每项结论写成表"的表）。
type AuditItemRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Mechanism  string `json:"mechanism"`
	Reason     string `json:"reason"`
	Hits       int    `json:"hits"`        // 命中数（禁模式 + 计数 + AST，含已豁免）
	Allowed    int    `json:"allowed"`     // 其中已豁免
	Counted    int    `json:"counted"`     // 其中"只计数"（必要性调用）
	ExpectHits int    `json:"expect_hits"` // 声明的命中数（-1 = 不检查）
	Drift      bool   `json:"drift"`
	Red        bool   `json:"red"`
}

// AuditReport —— 审计结论（可序列化；脚本按字段判，不靠输出字样）。
type AuditReport struct {
	Dir         string         `json:"dir,omitempty"`
	Scanned     []string       `json:"scanned"`
	Items       []AuditItemRow `json:"items"`
	Findings    []Finding      `json:"findings,omitempty"`     // 未豁免的命中（红）
	Allowed     []Finding      `json:"allowed,omitempty"`      // 已豁免的命中（记账，不隐藏）
	Counted     []Finding      `json:"counted,omitempty"`      // 只计数的必要性调用（记账）
	Stale       []AllowRule    `json:"stale_allow,omitempty"`  // 过期豁免（红）
	ParseErrors []string       `json:"parse_errors,omitempty"` // 解析失败的文件（红：AST 检查会静默漏检）
	RedCount    int            `json:"red_count"`
}

// Verdict —— 审计判词：nil = PASS（专项错误码 ErrDeterminismAudit）。
func (r *AuditReport) Verdict() error {
	if r == nil {
		return &AuditError{Reasons: []string{"报告缺失（nil）：没有证据"}}
	}
	var why []string
	for _, row := range r.Items {
		if row.Status == string(StatusUnfrozen) {
			why = append(why, fmt.Sprintf("%s：声明「未冻结」⇒ 标红（未冻结项必须处置后才能宣称回放确定）", row.Name))
		}
		if row.Drift {
			why = append(why, fmt.Sprintf("%s：计数漂移（声明 %d 处，实际 %d 处）—— 新增使用点要改代码或补豁免；"+
				"若确实是必要性调用（见 CountOnly），才更新声明数", row.Name, row.ExpectHits, row.Hits))
		}
	}
	for _, f := range r.Findings {
		why = append(why, fmt.Sprintf("%s:%d [%s·%s] 命中 %s：%s（%s）",
			f.File, f.Line, f.Name, f.Status, f.Pattern, f.Text, orDash(f.Why)))
	}
	for _, s := range r.Stale {
		why = append(why, fmt.Sprintf("过期豁免（声明了却一次都没命中，豁免表在腐化）：%s 函数 %q 模式 %q —— %s",
			s.File, s.Func, s.Pattern, s.Reason))
	}
	for _, p := range r.ParseErrors {
		why = append(why, fmt.Sprintf("源码无法解析 ⇒ AST 检查静默漏检：%s", p))
	}
	if len(why) == 0 {
		return nil
	}
	return &AuditError{Reasons: why}
}

// Table —— 十类结论表（markdown；"把每项的结论写成表"的可执行版）。
func (r *AuditReport) Table() string {
	var b strings.Builder
	b.WriteString("| # | 类别 | 状态 | 命中(豁免/计数) | 声明 | 判词 | 冻结机制 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for i, row := range r.Items {
		state := "✓"
		if row.Red {
			state = "✗ 红"
		}
		fmt.Fprintf(&b, "| %d | %s（%s） | %s | %d(%d/%d) | %d | %s | %s |\n",
			i+1, row.Name, row.ID, row.Status, row.Hits, row.Allowed, row.Counted, row.ExpectHits, state,
			truncate(row.Mechanism, 90))
	}
	return b.String()
}

// String —— 人读摘要（表 + 红项明细）。
func (r *AuditReport) String() string {
	if r == nil {
		return "审计报告缺失"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "确定性审计：扫 %d 个文件，十类逐项声明，红项 %d 处\n", len(r.Scanned), r.RedCount)
	b.WriteString(r.Table())
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "  ✗ %s:%d [%s] %s\n", f.File, f.Line, f.Name, f.Text)
	}
	for _, s := range r.Stale {
		fmt.Fprintf(&b, "  ✗ 过期豁免 %s/%s %s\n", s.File, s.Func, s.Pattern)
	}
	return b.String()
}

// AuditError —— 审计不通过（专项错误码）。
type AuditError struct{ Reasons []string }

func (e *AuditError) Error() string {
	return fmt.Sprintf("%v：%s", ErrDeterminismAudit, strings.Join(e.Reasons, "；"))
}

// Is —— 支持 errors.Is(err, ErrDeterminismAudit)。
func (e *AuditError) Is(target error) bool { return target == ErrDeterminismAudit }

// ── 审计器 ──────────────────────────────────────────────────────────────────

// AuditOptions —— 扫描口径。
type AuditOptions struct {
	// IncludeTests —— 是否连 _test.go 一起扫。默认 **false**：测试允许造夹具（取时、读环境、range map），
	// 生产路径才是要守的东西；打开它是更严的用法（每日巡检可用）。
	IncludeTests bool
	// MaxLineLen —— 命中行截断（默认 160）。
	MaxLineLen int
	// DirLabel —— 报告里的目录标签（仅人读）。
	DirLabel string
}

func (o AuditOptions) maxLine() int {
	if o.MaxLineLen <= 0 {
		return 160
	}
	return o.MaxLineLen
}

// Auditor —— 持已编译模式的审计器。
type Auditor struct {
	items   []AuditItem
	pats    [][]*regexp.Regexp // 禁模式
	patsCnt [][]*regexp.Regexp // 只计数模式
}

// NewAuditor —— 编译声明表。模式非法 ⇒ 报错（**不静默跳过一条模式**：静默跳过的模式等于没有门禁）。
func NewAuditor(items []AuditItem) (*Auditor, error) {
	a := &Auditor{items: append([]AuditItem(nil), items...)}
	for _, it := range items {
		var ps, cs []*regexp.Regexp
		for _, p := range it.Forbidden {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("%w: 类别 %s 的禁模式 %q 非法：%v", ErrDeterminismAudit, it.ID, p, err)
			}
			ps = append(ps, re)
		}
		for _, p := range it.CountOnly {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("%w: 类别 %s 的计数模式 %q 非法：%v", ErrDeterminismAudit, it.ID, p, err)
			}
			cs = append(cs, re)
		}
		a.pats = append(a.pats, ps)
		a.patsCnt = append(a.patsCnt, cs)
	}
	return a, nil
}

// DefaultAuditor —— 用 AuditTable（十类逐项声明）。
func DefaultAuditor() (*Auditor, error) { return NewAuditor(AuditTable) }

// AuditDir —— 扫一个目录下的 .go 文件（不递归：本包是叶包，一层的文件就是全部）。
func (a *Auditor) AuditDir(dir string, opt AuditOptions) (*AuditReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: 读目录 %s：%v", ErrDeterminismAudit, dir, err)
	}
	srcs := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%w: 读 %s：%v", ErrDeterminismAudit, e.Name(), err)
		}
		srcs[e.Name()] = string(b)
	}
	if opt.DirLabel == "" {
		opt.DirLabel = dir
	}
	rep, err := a.AuditSources(srcs, opt)
	if err != nil {
		return nil, err
	}
	rep.Dir = opt.DirLabel
	return rep, nil
}

// AuditSources —— 对一组内存源码扫描（名字当相对路径）。测试用它植入变体，不必落盘。
func (a *Auditor) AuditSources(sources map[string]string, opt AuditOptions) (*AuditReport, error) {
	if a == nil || len(a.items) == 0 {
		return nil, fmt.Errorf("%w: 审计器无声明表（拒绝空跑出结论）", ErrDeterminismAudit)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%w: 零个源文件（拒绝空跑出结论）", ErrDeterminismAudit)
	}
	// _test.go 的过滤放在这里（唯一的入口收口）：两个入口（AuditDir / AuditSources）口径必须一致 ——
	// 否则"目录里扫不到、内存里扫得到"会变成一个查不出来的假绿。
	if !opt.IncludeTests {
		filtered := make(map[string]string, len(sources))
		for name, src := range sources {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			filtered[name] = src
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("%w: 源文件全是 _test.go 且未开 IncludeTests（要么显式打开，要么这个目录不该扫）", ErrDeterminismAudit)
		}
		sources = filtered
	}
	parsed, parseErrs := parseSources(sources)
	slots := collectMapScopes(parsed)
	rep := &AuditReport{Dir: opt.DirLabel, ParseErrors: parseErrs}

	hits := map[string]int{}
	allowed := map[string]int{}
	counted := map[string]int{}
	used := map[AllowRule]int{} // 豁免规则 → 命中次数（0 = 过期）
	names := sortedKeys(sources)

	for _, name := range names {
		rep.Scanned = append(rep.Scanned, name)
		for i, raw := range strings.Split(sources[name], "\n") {
			code := codeRegion(raw)
			if strings.TrimSpace(code) == "" {
				continue
			}
			text := truncate(strings.TrimSpace(code), opt.maxLine())
			for ii := range a.items {
				it := a.items[ii]
				if pi := firstMatch(a.pats[ii], code); pi >= 0 {
					pat := it.Forbidden[pi]
					hits[it.ID]++
					f := Finding{File: name, Line: i + 1, Class: it.ID, Name: it.Name, Status: string(it.Status), Pattern: pat, Text: text}
					if rule, ok := matchAllowByPattern(it.Allow, name, pat); ok {
						f.Allowed = &rule
						used[rule]++
						allowed[it.ID]++
						rep.Allowed = append(rep.Allowed, f)
						continue
					}
					f.Red = true
					f.Why = redWhy(it)
					rep.Findings = append(rep.Findings, f)
					continue
				}
				if pi := firstMatch(a.patsCnt[ii], code); pi >= 0 {
					hits[it.ID]++
					counted[it.ID]++
					rep.Counted = append(rep.Counted, Finding{
						File: name, Line: i + 1, Class: it.ID, Name: it.Name, Status: string(it.Status),
						Pattern: it.CountOnly[pi], Text: text,
					})
				}
			}
		}
	}

	// AST 补充（词法抓不到的形态）：
	for _, p := range parsed {
		for _, imp := range importsOf(p) {
			for ii := range a.items {
				it := a.items[ii]
				if !importIsBanned(it.Imports, imp) {
					continue
				}
				hits[it.ID]++
				rep.Findings = append(rep.Findings, Finding{
					File: p.name, Line: importLine(p, imp), Class: it.ID, Name: it.Name, Status: string(it.Status),
					Pattern: "import " + imp, Text: "import " + imp, Red: true,
					Why: "禁 import（按子树禁用：禁整包即连同其子包一起禁；无豁免规则，只能改代码）",
				})
			}
		}
		if it, ok := a.itemByAST(astMapRange); ok {
			for _, rh := range rangesOverMap(p, slots) {
				hits[it.ID]++
				f := Finding{
					File: p.name, Line: rh.line, Class: it.ID, Name: it.Name, Status: string(it.Status),
					Pattern: mapRangePattern, Text: "range 的对象是 map（所属函数 " + orDash(rh.fn) + "）",
				}
				if rule, ok := matchAllowByFunc(it.Allow, p.name, rh.fn); ok {
					f.Allowed = &rule
					used[rule]++
					allowed[it.ID]++
					rep.Allowed = append(rep.Allowed, f)
					continue
				}
				f.Red = true
				f.Why = redWhy(it)
				rep.Findings = append(rep.Findings, f)
			}
		}
	}

	sortFindings(rep.Findings)
	sortFindings(rep.Allowed)
	sortFindings(rep.Counted)

	for ii := range a.items {
		it := a.items[ii]
		row := AuditItemRow{
			ID: it.ID, Name: it.Name, Status: string(it.Status), Mechanism: it.Mechanism, Reason: it.Reason,
			Hits: hits[it.ID], Allowed: allowed[it.ID], Counted: counted[it.ID], ExpectHits: it.ExpectHits,
		}
		if it.ExpectHits >= 0 && row.Hits != it.ExpectHits {
			row.Drift = true
			row.Red = true
		}
		if it.Status == StatusUnfrozen {
			row.Red = true
		}
		for _, f := range rep.Findings {
			if f.Class == it.ID {
				row.Red = true
			}
		}
		rep.Items = append(rep.Items, row)
	}

	for ii := range a.items {
		for _, rule := range a.items[ii].Allow {
			if used[rule] == 0 {
				rep.Stale = append(rep.Stale, rule)
			}
		}
	}

	reds := len(rep.Findings) + len(rep.Stale) + len(rep.ParseErrors)
	for _, row := range rep.Items {
		if row.Drift {
			reds++
		}
		if row.Status == string(StatusUnfrozen) {
			reds++
		}
	}
	rep.RedCount = reds
	return rep, nil
}

func firstMatch(pats []*regexp.Regexp, code string) int {
	for i := range pats {
		if pats[i].MatchString(code) {
			return i
		}
	}
	return -1
}

func (a *Auditor) itemByAST(kind string) (AuditItem, bool) {
	for _, it := range a.items {
		if it.AST == kind {
			return it, true
		}
	}
	return AuditItem{}, false
}

func redWhy(it AuditItem) string {
	if it.Status == StatusUnfrozen {
		return "该类声明为「未冻结」，命中即标红"
	}
	return "命中该类禁模式且无豁免：若这是合理的注入点/显式降级，请补 AllowRule（带理由）；否则改代码"
}

// importIsBanned —— 禁 import 按**子树**判定（禁 golang.org/x/text 即连 x/text/language 一起禁）。
func importIsBanned(bans []string, imp string) bool {
	for _, b := range bans {
		if b == "" {
			continue
		}
		if imp == b || strings.HasPrefix(imp, b+"/") {
			return true
		}
	}
	return false
}

// matchAllowByPattern —— 词法命中的豁免匹配。
//
// 比较用**去转义**后的形态：模式与豁免写的是同一个词，只是转义写法可能不同（`time\.Now\(` 与 `time.Now(`）。
// 反过来**不做**正则匹配 —— 豁免表只该精确指向一条禁模式；宽匹配会让豁免表变成免死牌。
func matchAllowByPattern(rules []AllowRule, file, pattern string) (AllowRule, bool) {
	want := deEscape(pattern)
	for _, r := range rules {
		if r.Func != "" || r.Pattern == "" {
			continue
		}
		if r.File != "" && r.File != file {
			continue
		}
		if deEscape(r.Pattern) == want {
			return r, true
		}
	}
	return AllowRule{}, false
}

func deEscape(s string) string { return strings.ReplaceAll(s, `\`, "") }

// matchAllowByFunc —— AST 命中的豁免匹配（文件 + 所属函数名）。
func matchAllowByFunc(rules []AllowRule, file, fn string) (AllowRule, bool) {
	if fn == "" {
		return AllowRule{}, false
	}
	for _, r := range rules {
		if r.Func == "" {
			continue
		}
		if r.File != "" && r.File != file {
			continue
		}
		if r.Func == fn {
			return r, true
		}
	}
	return AllowRule{}, false
}

// codeRegion —— 取一行的"代码区"：去掉行注释。
//
// **字符串字面量保留**（不屏蔽）：import 路径与模式字面量都在字符串里，屏蔽它们会让"整包禁"
// （crypto/rand、uuid、x/text）漏报 —— 而漏报正是门禁最坏的失效方向。代价有两条，都如实写在这里：
//  1. 字符串里提到禁模式会算命中（罕见；改措辞或补豁免即可，不会静默）；
//  2. 跨行块注释按"到行尾为止"近似（本包与仓内 Go 代码都不用跨行块注释）。
func codeRegion(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == '\\' && quote != '`' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				return line[:i]
			}
		}
	}
	return line
}

// ── AST 部分：禁 import + range-over-map ────────────────────────────────────

type parsedSource struct {
	name string
	fset *token.FileSet
	file *ast.File
}

func parseSources(srcs map[string]string) ([]parsedSource, []string) {
	var out []parsedSource
	var bad []string
	for _, name := range sortedKeys(srcs) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, srcs[name], parser.ParseComments)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s：%v", name, err))
			continue
		}
		out = append(out, parsedSource{name: name, fset: fset, file: f})
	}
	return out, bad
}

func importsOf(p parsedSource) []string {
	var out []string
	for _, imp := range p.file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		out = append(out, path)
	}
	return out
}

func importLine(p parsedSource, path string) int {
	for _, imp := range p.file.Imports {
		if got, err := strconv.Unquote(imp.Path.Value); err == nil && got == path {
			return p.fset.Position(imp.Pos()).Line
		}
	}
	return 0
}

// mapSlot —— "这个名字在**某个作用域**里是 map"的事实（global = 按名字全局判定）。
type mapSlot struct {
	name   string
	start  token.Pos
	end    token.Pos
	global bool
}

// span —— 源码区间（作用域定位用）。
type span struct {
	start, end token.Pos
	name       string
}

// collectMapScopes —— 收集"名字在某作用域里是 map"的事实。
//
// 为什么必须带作用域（实测踩过的坑）：仓内短名会被复用 ——
// match.go 里 `keys` 在一处是 []string、在另一处是 map[string]bool；同一个类型 switch 里
// map 分支与 slice 分支共用一个绑定名。只按名字判定会把 []string 的 range 误报成 map range。
// 所以：字段名按**全局**名字判定（跨方法用得到），局部量/参数按**最小包围作用域**判定。
//
// 边界如实：靠**语法**判定，不加载类型信息（叶包零依赖纪律）；跨包调用返回的 map 证明不了 ⇒ 会漏报。
// 补偿控制是 ExpectHits 的计数漂移（新增点会让计数变）。
func collectMapScopes(files []parsedSource) []mapSlot {
	var out []mapSlot
	for _, p := range files {
		scopes := scopeSpans(p)
		// struct 字段的位置集合：字段名按**名字全局**判定（方法体里 range 的是 `x.field`，
		// 那个位置在类型声明之外，用作用域包含判定必然漏检）；参数/返回值则按所属函数作用域判定。
		structFields := map[token.Pos]bool{}
		ast.Inspect(p.file, func(n ast.Node) bool {
			if st, ok := n.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					structFields[f.Pos()] = true
				}
			}
			return true
		})
		inner := func(pos token.Pos) span {
			best := span{}
			bestSize := token.Pos(1 << 30)
			for _, s := range scopes {
				if s.start <= pos && pos <= s.end {
					if size := s.end - s.start; size < bestSize {
						best, bestSize = s, size
					}
				}
			}
			return best
		}
		ast.Inspect(p.file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.TypeSpec:
				if _, ok := t.Type.(*ast.MapType); ok {
					out = append(out, mapSlot{name: t.Name.Name, global: true})
				}
			case *ast.Field: // struct 字段（按名字全局判定）/ 参数与返回值（作用域=所属函数）
				if exprIsMap(t.Type) {
					if structFields[t.Pos()] {
						for _, id := range t.Names {
							out = append(out, mapSlot{name: id.Name, global: true})
						}
						break
					}
					sp := inner(t.Pos())
					for _, id := range t.Names {
						out = append(out, mapSlot{name: id.Name, start: sp.start, end: sp.end})
					}
				}
			case *ast.ValueSpec: // var / const
				if exprIsMap(t.Type) || anyExprIsMap(t.Values) {
					sp := inner(t.Pos())
					for _, id := range t.Names {
						out = append(out, mapSlot{name: id.Name, start: sp.start, end: sp.end})
					}
				}
			case *ast.AssignStmt: // x := make(map…) / x := map[…]{} …
				if t.Tok != token.DEFINE {
					break
				}
				sp := inner(t.Pos())
				for i, r := range t.Rhs {
					if !exprIsMap(r) || i >= len(t.Lhs) {
						continue
					}
					if id, ok := t.Lhs[i].(*ast.Ident); ok {
						out = append(out, mapSlot{name: id.Name, start: sp.start, end: sp.end})
					}
				}
			case *ast.TypeSwitchStmt: // switch m := v.(type) { case map[string]any: … }
				name := typeSwitchBound(t.Assign)
				if name == "" {
					break
				}
				for _, st := range t.Body.List {
					if cc, ok := st.(*ast.CaseClause); ok && anyExprIsMap(cc.List) {
						out = append(out, mapSlot{name: name, start: cc.Pos(), end: cc.End()})
					}
				}
			}
			return true
		})
	}
	return out
}

// scopeSpans —— 候选作用域区间（函数 / 块 / case 分支）。
func scopeSpans(p parsedSource) []span {
	var out []span
	ast.Inspect(p.file, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncDecl:
			out = append(out, span{t.Pos(), t.End(), declName(t)})
		case *ast.FuncLit:
			out = append(out, span{t.Pos(), t.End(), fmt.Sprintf("<闭包@%d>", p.fset.Position(t.Pos()).Line)})
		case *ast.BlockStmt:
			out = append(out, span{t.Pos(), t.End(), ""})
		case *ast.CaseClause:
			out = append(out, span{t.Pos(), t.End(), ""})
		}
		return true
	})
	return out
}

func typeSwitchBound(as ast.Stmt) string {
	a, ok := as.(*ast.AssignStmt)
	if !ok || len(a.Lhs) != 1 {
		return ""
	}
	id, _ := a.Lhs[0].(*ast.Ident)
	if id == nil {
		return ""
	}
	return id.Name
}

// exprIsMap —— 该表达式是不是"map 形态"（map 类型 / map 复合字面量 / make(map…) / 括号包裹）。
func exprIsMap(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.MapType:
		return true
	case *ast.CompositeLit:
		return exprIsMap(t.Type)
	case *ast.ParenExpr:
		return exprIsMap(t.X)
	case *ast.CallExpr:
		if id, ok := t.Fun.(*ast.Ident); ok && id.Name == "make" && len(t.Args) > 0 {
			return exprIsMap(t.Args[0])
		}
	}
	return false
}

func anyExprIsMap(es []ast.Expr) bool {
	for _, e := range es {
		if exprIsMap(e) {
			return true
		}
	}
	return false
}

// isMapRangeExpr —— range 的被遍历对象是不是"已知在该位置是 map"的标识符/字段。
func isMapRangeExpr(x ast.Expr, slots []mapSlot, pos token.Pos) bool {
	name := ""
	switch t := x.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.SelectorExpr:
		name = t.Sel.Name
	case *ast.ParenExpr:
		return isMapRangeExpr(t.X, slots, pos)
	case *ast.StarExpr:
		return isMapRangeExpr(t.X, slots, pos)
	}
	if name == "" {
		return false
	}
	for _, s := range slots {
		if s.name != name {
			continue
		}
		if s.global || (s.start <= pos && pos <= s.end) {
			return true
		}
	}
	return false
}

// rangeHit —— 一处 range-over-map。
type rangeHit struct {
	line int
	fn   string
}

// rangesOverMap —— 单文件里所有 range-over-map（含所属函数名）。
//
// 归属用"包含该 range 的最小函数区间"判定 —— 自己管理栈式的写法在 ast.Inspect 下没有退出钩子，
// 会串函数（把后面的 range 记到前面的函数名下），所以这里用区间包含。
func rangesOverMap(p parsedSource, slots []mapSlot) []rangeHit {
	var spans []span
	var raws []token.Pos
	ast.Inspect(p.file, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncDecl:
			spans = append(spans, span{t.Pos(), t.End(), declName(t)})
		case *ast.FuncLit:
			spans = append(spans, span{t.Pos(), t.End(), fmt.Sprintf("<闭包@%d>", p.fset.Position(t.Pos()).Line)})
		case *ast.RangeStmt:
			if isMapRangeExpr(t.X, slots, t.Pos()) {
				raws = append(raws, t.Pos())
			}
		}
		return true
	})
	out := make([]rangeHit, 0, len(raws))
	for _, pos := range raws {
		best := span{}
		bestSize := token.Pos(1 << 30)
		for _, s := range spans {
			if s.start <= pos && pos <= s.end {
				if size := s.end - s.start; size < bestSize {
					best, bestSize = s, size
				}
			}
		}
		out = append(out, rangeHit{line: p.fset.Position(pos).Line, fn: best.name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].line < out[j].line })
	return out
}

func declName(t *ast.FuncDecl) string {
	if t.Recv != nil && len(t.Recv.List) > 0 {
		if recv := typeNameOf(t.Recv.List[0].Type); recv != "" {
			return recv + "." + t.Name.Name
		}
	}
	return t.Name.Name
}

func typeNameOf(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return typeNameOf(t.X)
	case *ast.IndexExpr:
		return typeNameOf(t.X)
	}
	return ""
}

// ── 小工具 ──────────────────────────────────────────────────────────────────

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Class < fs[j].Class
	})
}

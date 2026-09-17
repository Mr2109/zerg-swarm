// sendback_reason.go — B 项④-A：**打回原因码**（责任前缀 `PRE_`/`POST_`/`INV_` + R 编号 + 一句可行动建议）。
//
// 要治的缺口（设计稿 v2.1 §4.7「原因码加责任前缀」／§4.10 并轨定论「失败归因」）：
//
//	打回如果只说「不行」，归因就只能是**主观**的 —— 三个人看同一份产出会给出三种说法（模型不行 / 提示没写好 /
//	环境不对），事后无法聚合、无法判定「到底该改哪一层」。故打回**必须带原因码**，且原因码里**自带责任归属**：
//
//		PRE_*  = **切片者 / 输入可见集**问题（片本身错：判据不全、落点不存在、上下文不足…）
//		POST_* = **执行者**问题（片没错，执行没做到：主张无据、越片动作、未按判据交付…）
//		INV_*  = **环境 / 流程**问题（两边都没错：工具挂、依赖不可得、合成门基础设施失败…）
//
//	形如 `PRE_R3`：前缀告诉人「该改哪一层」，R 编号告诉机器「哪条判据没过」（R 编号取自设计稿 v2.1 §3.4
//	的机器判据 **R0–R13** 闭集 —— 本文件按闭集校验，越界即报错，见 JudgeCodeMin/JudgeCodeMax）。
//
// ── 与设计稿逐条对齐（只引用，不改它们）──
//
//	§4.7：「格式类打回必须附**最小修复提示**，否则会把判据缺陷记成模型缺陷 ✗」
//	      ⇒ 本文件的 `Advice` 就是这条最小修复提示，`Validate` **要求它非空**（只报「不行」不算有效打回）。
//	§4.7：「追问/阻滞 ⇒ 信息不足，退回上游补规格」  ⇒ 同前缀族的第 4 态见 fourth_state.go（同包）。
//	§3.4：机器判据 R0–R13（闭集）—— R 编号的**唯一真源**；本文件**不自造编号**。
//	§2.6：`INV_MERGE_*`（合成门红时凶手片的归因）用 `INV_` 前缀 —— 与三分归因同规；**它不带 R 编号**，
//	      故本文件的解析器**认不出**它（登记为设计缺口，见回报的「冲突/未实现项」；不自行放宽 ✗）。
//
// ── 三条硬口径（写死在代码里，由用例逐条钉住）──
//
//	① **未知前缀 ⇒ 报错**：绝不默认成某一类（默认 = 把责任替人定了，且事后无从发现）。
//	② **缺 R 编号 ⇒ 报错**：`PRE_` / `PRE` / `PRE_R` 一律拒（没有编号 = 归因落到前缀就停了，机器无法聚合）。
//	③ **零值 / 认不出的东西一律 `Validate` 不过**：认不出按「最严」——**不可发回**，不是「最宽」。
//
// ── 两层校验的分工（别把「解析通过」读成「可以发回」）──
//
//	ParseSendBackCode  —— 判**码本身**：前缀 + R 编号（建议可有可无，因为解析侧要能读日志里只有码的行）
//	Validate          —— 判**能不能发回**：码合法 **且** 建议非空（§4.7「格式类打回必须附最小修复提示」）
//	⇒ 发回出口（loopcore.SendBack）只认过了 Validate 的码；解析通过但没建议的码**发不出去**。
//
// ── 本批边界（与 T5.1/T5.3/T5.4/T5.7/T5.8 同一纪律）──
//
//	· 叶子包：只 import 标准库（errors / fmt / strconv / strings / unicode）；零内部依赖。
//	· **纯函数**：不读时间、不读 IO、不落盘、不改任何既有类型的行为（pending.go / decide.go 一行不动）。
//	· 本批**不实现**：原因码的落账/观测出口、复议（同原因码连续 2 次打回 ⇒ 先复审判据）的计数器、
//	  状态机（设计稿 设计-任务模块骨架 v1.0 §1.1/§1.2）。这些属后续批次，登记不落。
package policy

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ── 专项错误（可判：调用方按 errors.Is 分派，不解析错误原文）──────────────────

var (
	// ErrUnknownResponsibility — 责任前缀认不出（**绝不默认成某一类**）。
	ErrUnknownResponsibility = errors.New("打回原因码的责任前缀认不出")
	// ErrMissingJudgeNumber — 缺 R 编号（判据编号是机器可聚合的那一半，缺了归因就停在人话层面）。
	ErrMissingJudgeNumber = errors.New("打回原因码缺 R 编号")
	// ErrJudgeNumberOutOfRange — R 编号越界（设计稿 §3.4 的闭集是 R0–R13；越界 = 编号认不出，不猜）。
	ErrJudgeNumberOutOfRange = errors.New("打回原因码的 R 编号越界")
	// ErrMissingAdvice — 缺「最小修复提示」（只报「不行」不算有效打回 —— §4.7）。
	ErrMissingAdvice = errors.New("打回原因码缺最小修复提示（可行动建议）")
)

// ── 责任前缀（三分归因：它让「该改哪一层」从主观变成可判定）──────────────────

// Responsibility — 打回责任的**三个前缀**（三类，无第四类）。字符串值就是码里的前缀本身。
type Responsibility string

const (
	// RespPreSlice — `PRE_`：**切片者 / 输入可见集**问题（片本身错）。
	RespPreSlice Responsibility = "PRE"
	// RespPostExec — `POST_`：**执行者**问题（片没错，执行没做到）。
	RespPostExec Responsibility = "POST"
	// RespInvEnv — `INV_`：**环境 / 流程**问题（两边都没错；含 §2.6 的 `INV_MERGE_*` 一族）。
	RespInvEnv Responsibility = "INV"
)

// Valid — 是否三类之一（认不出一律不猜，见 ParseResponsibility / Validate）。
func (r Responsibility) Valid() bool {
	switch r {
	case RespPreSlice, RespPostExec, RespInvEnv:
		return true
	}
	return false
}

// Label — 中文责任标签（给人看：打回指令与观测面都要能一眼读懂「该改哪一层」）。
// 认不出的前缀 ⇒ 回显「认不出」而**不冒充**某一类（与 fail-closed 同向）。
func (r Responsibility) Label() string {
	switch r {
	case RespPreSlice:
		return "切片者/输入可见集责任"
	case RespPostExec:
		return "执行者责任"
	case RespInvEnv:
		return "环境/流程责任"
	}
	return "责任前缀认不出（不得据此归因）"
}

// ParseResponsibility — 解析前缀文本（大小写不敏感、去首尾空白、容忍尾随 `_`，如 `pre_` ⇒ PRE）。
// **认不出一律报错**：绝不默认成某一类（默认 = 替人定了责任，且事后无从发现）。
func ParseResponsibility(raw string) (Responsibility, error) {
	tok := strings.TrimSpace(raw)
	tok = strings.TrimSuffix(tok, "_") // 容忍写成 `PRE_` 的形态（有些日志这么拼）
	r := Responsibility(strings.ToUpper(strings.TrimSpace(tok)))
	if !r.Valid() {
		return "", fmt.Errorf("%w：「%s」（只认 PRE_ / POST_ / INV_ 三种；**不得默认成某一类**）",
			ErrUnknownResponsibility, strings.TrimSpace(raw))
	}
	return r, nil
}

// ── R 编号的闭集（设计稿 v2.1 §3.4）────────────────────────────────────────

// 机器判据编号的**闭集**：设计稿 v2.1 §3.4「机器判据 R0–R13」。
// 越界 ⇒ 报错（编号认不出时不猜；若设计稿将来扩编号，**只改这两行**）。
const (
	JudgeCodeMin = 0
	JudgeCodeMax = 13
)

// judgeRangeText — 闭集的可读形态（错误信息里给「合法的值域」——错误必须可行动）。
func judgeRangeText() string {
	return fmt.Sprintf("R%d–R%d（设计稿 v2.1 §3.4 的机器判据闭集）", JudgeCodeMin, JudgeCodeMax)
}

// ── 原因码 ──────────────────────────────────────────────────────────────────

// SendBackCode — 一个打回原因码：`<前缀>_R<编号>` + 一句**可行动**建议。
//
// 形如 `PRE_R3`：`Resp=PRE`（切片者责任）、`R=3`（判据 R3「criteria 三件套齐全」没过）、
// `Advice="补齐命令+期望输出+当前红/绿三件套"`（最小修复提示）。
//
// 序列化形态（观测/审计/HTTP 面复用）：`String()` = 码本身（`PRE_R3`，低基数、可聚合）；
// `Format()` = 码 + 建议（给人看的完整形态）。
type SendBackCode struct {
	// Resp — 责任前缀（三类；零值 = 未给 ⇒ Validate 不过 ⇒ 发不出去）。
	Resp Responsibility `json:"resp"`
	// R — 判据编号（§3.4 闭集 R0–R13）。
	// **`R = 0` 是合法值**（判据 R0「正面句式」确实存在）⇒「有没有编号」这件事只由**解析**判
	//（`PRE_` 一律报错），结构侧用负数表示「缺编号」（见 Validate）。
	R int `json:"r"`
	// Advice — **最小修复提示**（§4.7：格式类打回必须附它，否则会把判据缺陷记成模型缺陷 ✗）。
	// 它必须指向**动作**（补什么字段 / 改什么句式 / 重跑什么命令），不是复述症状。
	Advice string `json:"advice"`
}

// String — 码的规范形态：`PRE_R3`（**不含**建议；低基数、可聚合、可进日志与观测）。
// 前缀/编号非法时也照实拼出来（便于在错误信息里回显），**不伪装成合法码**。
func (c SendBackCode) String() string {
	return fmt.Sprintf("%s_R%d", string(c.Resp), c.R)
}

// Format — 完整形态：`PRE_R3 建议：<advice>`（给人看的；发回指令里嵌的是它）。
// **必须先过 Validate**：认不出 / 缺编号 / 缺建议一律报错 —— 格式化的出口也不许静默放行。
func (c SendBackCode) Format() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	return c.String() + " 建议：" + strings.TrimSpace(c.Advice), nil
}

// Validate — 「这个码能不能发回」的唯一判据（**发回出口只认过它的码**）：
//
//	· 前缀必须三类之一 —— 认不出 ⇒ ErrUnknownResponsibility（**不默认成某一类**）
//	· R 编号必须给出且在 §3.4 闭集内 —— 缺/越界 ⇒ ErrMissingJudgeNumber / ErrJudgeNumberOutOfRange
//	· 建议必须非空 —— 缺 ⇒ ErrMissingAdvice（只报「不行」不算有效打回）
func (c SendBackCode) Validate() error {
	if !c.Resp.Valid() {
		return fmt.Errorf("%w：「%s」（只认 PRE_ / POST_ / INV_；**不得默认成某一类**⇒ 本码不可发回）",
			ErrUnknownResponsibility, string(c.Resp))
	}
	if c.R < JudgeCodeMin {
		return fmt.Errorf("%w：%s_R?（形态 `<前缀>_R<编号>`，例如 PRE_R3；编号值域 %s ⇒ 本码不可发回）",
			ErrMissingJudgeNumber, string(c.Resp), judgeRangeText())
	}
	if c.R > JudgeCodeMax {
		return fmt.Errorf("%w：%s（编号值域 %s ⇒ 认不出的编号不猜、不截断）",
			ErrJudgeNumberOutOfRange, c.String(), judgeRangeText())
	}
	if strings.TrimSpace(c.Advice) == "" {
		return fmt.Errorf("%w：码 %s 只说了「哪条没过」，没说「怎么改」"+
			"（§4.7「格式类打回必须附最小修复提示，否则会把判据缺陷记成模型缺陷 ✗」⇒ 本码不可发回）",
			ErrMissingAdvice, c.String())
	}
	return nil
}

// NewSendBackCode — 构造并**当场校验**（构造侧唯一的正规入口：不可能构造出一个过不了 Validate 的码）。
// 建议里只有空白 ⇒ 报错（空建议与被省略的建议是同一件事）。
func NewSendBackCode(resp Responsibility, judge int, advice string) (SendBackCode, error) {
	c := SendBackCode{Resp: resp, R: judge, Advice: strings.TrimSpace(advice)}
	if err := c.Validate(); err != nil {
		return SendBackCode{}, err
	}
	return c, nil
}

// ParseSendBackCode — 解析原因码文本。**纯函数**。
//
// 语法（写死在这里；大小写不敏感、首尾空白容忍）：
//
//	原因码 = <前缀>_R<编号> [ <分隔> <可行动建议> ]
//	前缀   = PRE | POST | INV
//	分隔   = 空白 / 全角空格 / `：` / `:`（建议前可再写一次「建议」二字，见 Format）
//
// 例：`PRE_R3` · `pre_r0` · `INV_R13 重启工具链后重跑合成门` · `POST_R10 建议：贴出回执原文片段`
//
// 三条 fail-closed（**全部有反例用例钉住**）：
//
//	① 前缀认不出（`XYZ_R3` / 只有数字 `R3`）⇒ ErrUnknownResponsibility（**不默认成某一类**）
//	② 缺 R 编号（`PRE_` / `PRE` / `PRE_R` / `PRE_RX`）⇒ ErrMissingJudgeNumber（**不默认成某个编号**）
//	③ 编号越界（`PRE_R14`）⇒ ErrJudgeNumberOutOfRange
//
// **解析通过 ≠ 可以发回**：建议可缺（日志里常常只有码），要发回必须先过 Validate。
func ParseSendBackCode(raw string) (SendBackCode, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return SendBackCode{}, fmt.Errorf("%w：原因是**空串**（打回必须带原因码，不能空着）", ErrUnknownResponsibility)
	}
	codeTok, rest := splitCodeToken(text)
	resp, judge, err := parseCodeToken(codeTok)
	if err != nil {
		return SendBackCode{}, err
	}
	return SendBackCode{Resp: resp, R: judge, Advice: parseAdvice(rest)}, nil
}

// splitCodeToken — 把「码 + 建议」切两半：码 = 首个分隔符之前的部分。
// 分隔符 = 空白/制表/全角空格/`：`/`:`（**第一个**分隔符之后全是建议原文，不再二次切分）。
func splitCodeToken(text string) (code, rest string) {
	for i, r := range text {
		if r == '：' || r == ':' || unicode.IsSpace(r) {
			return strings.TrimSpace(text[:i]), text[i:]
		}
	}
	return text, ""
}

// parseAdvice — 从分隔符之后取出建议原文：剥掉分隔符与可选的「建议」二字，其余**原样**保留。
func parseAdvice(rest string) string {
	s := strings.TrimLeftFunc(rest, func(r rune) bool { return r == '：' || r == ':' || unicode.IsSpace(r) })
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "建议") { // 与 Format() 的形态对称（`PRE_R3 建议：…` ⇒ 解析回来还是同一句建议）
		s = strings.TrimLeftFunc(strings.TrimPrefix(s, "建议"), func(r rune) bool {
			return r == '：' || r == ':' || unicode.IsSpace(r)
		})
	}
	return strings.TrimSpace(s)
}

// parseCodeToken — 解析 `<前缀>_R<编号>`。三条 fail-closed 全在这里（唯一实现点）。
func parseCodeToken(tok string) (Responsibility, int, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", 0, fmt.Errorf("%w：只有空串（形态 `<前缀>_R<编号>`，例如 PRE_R3）", ErrMissingJudgeNumber)
	}
	// ② 缺 R 编号：整段就是前缀（`PRE` / `PRE_`）⇒ 报错，**不默认成某个编号**
	if resp, err := ParseResponsibility(tok); err == nil {
		return "", 0, fmt.Errorf("%w：「%s」只写了前缀（形态 `<前缀>_R<编号>`，例如 %s_R3；"+
			"编号是机器可聚合的那一半，缺了就不算有效打回）", ErrMissingJudgeNumber, tok, string(resp))
	}
	// 切前缀。注意：**先用 ParseResponsibility 判「整段是不是只有前缀」**（上面那一步），
	// 故这里只在真的带了下划线时才继续 —— `XYZ_R3` 会走到下面报「前缀认不出」。
	head, tail, ok := strings.Cut(tok, "_")
	if !ok {
		return "", 0, fmt.Errorf("%w：「%s」没有下划线分隔（形态 `<前缀>_R<编号>`，例如 PRE_R3；"+
			"只认 PRE_ / POST_ / INV_ 三种前缀，**不得默认成某一类**）", ErrUnknownResponsibility, tok)
	}
	resp, err := ParseResponsibility(head)
	if err != nil {
		return "", 0, err
	}
	// ① 编号部分必须是非空数字且紧跟在 R 后面（`RX3` / `R` / 空 / 别的字母 ⇒ 认不出，不猜）
	digits, ok := strings.CutPrefix(strings.ToUpper(strings.TrimSpace(tail)), "R")
	if !ok {
		return "", 0, fmt.Errorf("%w：「%s」的编号部分「%s」不以 R 开头（形态 `<前缀>_R<编号>`，"+
			"例如 %s_R3；编号值域 %s）", ErrMissingJudgeNumber, tok, strings.TrimSpace(tail), string(resp), judgeRangeText())
	}
	digits = strings.TrimSpace(digits)
	if digits == "" {
		return "", 0, fmt.Errorf("%w：「%s」有 R 却没有编号（形态 `<前缀>_R<编号>`，例如 %s_R3；编号值域 %s）",
			ErrMissingJudgeNumber, tok, string(resp), judgeRangeText())
	}
	n, convErr := strconv.Atoi(digits)
	if convErr != nil {
		return "", 0, fmt.Errorf("%w：「%s」的编号「%s」不是十进制数字（形态 `<前缀>_R<编号>`；值域 %s）",
			ErrMissingJudgeNumber, tok, digits, judgeRangeText())
	}
	if n < JudgeCodeMin || n > JudgeCodeMax {
		return "", 0, fmt.Errorf("%w：「%s」的编号 %d 不在值域 %s 内（认不出的编号不猜、不截断）",
			ErrJudgeNumberOutOfRange, tok, n, judgeRangeText())
	}
	return resp, n, nil
}

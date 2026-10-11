// family_gap_reread.go —— `gap reread <GAP id>…`（只读面 · 判据复跑 · 2026-10-05 · 缺口账 `GAP-20260927-248`）。
//
// 病（缺口账 `GAP-20260927-248` 逐字）：`solved_evidence` 是**一次性文本**（"现有 75 条非空"）
// 而非**可重放结果** ⇒ 「已解」条目**是否已回归**机器判不出（只能靠 `gap verify` 现跑，而它自己
// 又不设防 —— 见 `GAP-20260927-247`）。同族实测（`GAP-20260927-248` 的注 · 第六批元面复核）：
// 抽 29 条已解账真跑复核，17 条仍逐字复现、12 条读数已失真（行号漂 7 · 计数/形状漂 5）。
//
// 本面的口径（**只报告 · 不对拍不进退出码** —— 免得存量陈旧把它变成恒红）：
//
//	① **真源** = 该条账行的 `solved_evidence`（不新增真源键）。
//	② **带键标读数** = `gapTwoStateParse` 认得的三格形态
//	   `cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`（销案两态对拍 `改前{…} · 改后{…}` 里的
//	   两段 · 或老形态里直接嵌的一段）—— 判据面**复用同族那一枚**，不另造第二套解析。
//	③ **对拍** = 重跑它的 `cmd`，与记下的 `rc` / `reading` 逐条比：
//	   rc 变（记 rc ≠ 现跑 rc）⇒ `reading_stale`；rc 一致再看 `reading` 里有 `件:行` 引用时
//	   **只判行内容**（那个件里还在不在）✗ 不判绝对行号。
//	④ **安全铁律** = 取值命令**不得是写命令**：重跑前先把 `cmd` 解析到命令树，落在**危险档**（写面）
//	   / 含 shell 元字符 / 解析不出 / 是本命令自己（自递归）⇒ **一律不执行**，记 `not_rereadable`
//	   （判不准就不跑 —— ✗ 绝不为「对一拍」真跑一条写面）。
//	⑤ **解析不出的条目 ⇒ 跳过并计数**（`skipped`）—— ✗ 禁报假 stale。
//	⑥ ✗ **不做**自由散文读数的机检（同路实测自由文本假阳 ≈79% ⇒ 不可行）：散文那一格只回 rc 一致，
//	   不拿它去判 stale。
//
// 退码（一律引现有表 · 本族不取新号）：0 读真源成功并给出读数（**含**有 `reading_stale`）·
// 2 用法错（缺目标 / `--all` 与 id 同给 / `--limit` 非非负整数 / `--json` 不给字段 / 账内没有该 id）·
// 8 真源读不到。
//
// ★ `--include-open`（2026-10-05 · 本面扩 · 缺口账 `GAP-20261005-60` 剩面）：把**未闭**（`state == 仍缺`）条目的 `verify_cmd`
//
//	纳入**同一套读数**（复用 `gapRereadWhy` 那一套只读闸 —— ✗ 不新造第五套）——「这条缺口今天还在不在」
//	不必再靠人眼估。★ 安全面**一字不放宽**：危险档 / 含 shell 元字符 / 占位串 / 解析不出 / 自递归 ⇒ **一律拒跑**（归 `not_rereadable`）。
//	★ 未闭条**无对拍基线**（没有记下的 rc/reading 可对）⇒ 跑通即记 `ok`（detail 给现跑 rc + 读数行），
//	✗ 不拿它判 `reading_stale`。★ 表尾**守恒自校**：`未闭总数 == 已跑 + 拒跑 + 截断`（逐字印三数）。
//	★ 默认档（不给本旗标）**行为逐字不变**；退码同现口径：**只报告** —— `not_rereadable` / 守恒不成立均**不进退出码**。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// gapReread 四条读数判定（本面闭集）+ 哑化档。
const (
	gapRereadOK      = "ok"             // rc 一致（散文那一格未机检）
	gapRereadStale   = "reading_stale"  // rc 变 或 件:行 的行内容在件里已找不到
	gapRereadNoRun   = "not_rereadable" // 写面 / 含 shell 元字符 / 解析不出 / 自递归 ⇒ 不执行
	gapRereadSkip    = "skipped"        // 解析不出带键标读数 ⇒ 跳过并计数
	gapRereadUnknown = "无法判"            // 对拍关（负控）⇒ 不给结论
	// gapRereadFixedNotClosed —— **已修未销**：未闭条（`state == 仍缺`）的 `verify_cmd` 现跑 rc==0
	// ⇒ 症状**已不复现**而 state 仍 未闭（缺口 `GAP-20260929-12` want：单列「已修未销」）。
	gapRereadFixedNotClosed = "已修未销"
	// gapRereadUnitRotted / gapRereadUnitMissing —— **落点腐检**两档（只读标注 · ✗ 不改账 · ✗ 不进退出码）：
	//   · 落点行号已腐 —— `件:行` 指的件在盘，但该行现读**不是内容行**（空/注释）或**越界**
	//     （行号会腐烂 —— 与同族 `gapRereadCiteProbe` 同一条「只判件 · 不判绝对行号」口径的**行级补面**）；
	//   · 落点件缺失 —— `unit`（**非**兜底档）现读不在盘（件被删/改名 —— 现读实证 M2 退场批的 `.py`/`.sh`）。
	// ★ 兜底 `unit`（`无件(命令面)` / 拟件 / 空）**不进**「件缺失」（现读 721 条是这类 ⇒ 误判＝造红）。
	gapRereadUnitRotted  = "落点行号已腐"
	gapRereadUnitMissing = "落点件缺失"
)

// gapRereadCompareOn —— 对拍开关（默认开）。负控把它置 false ⇒ 每条可解析读数回「无法判」，
// 用来证明「能分出 ok / reading_stale」靠的就是这一处对拍（哑化 ⇒ 退化成「无法判」）。
var gapRereadCompareOn = true

// gapRereadBetweenReads —— **仅供机检**的一次性钩子：在「T0 读入账」与「退出前重读」之间被调用一次，
// 用来在确定性夹具里模拟「两次读数之间真源被**外部**改过」（默认 nil ⇒ 什么都不做）。
// 生产路径恒为 nil ⇒ 行为面一字节不受影响。
var gapRereadBetweenReads func()

// gapRereadSHA16 —— 一段真源字节的账本身份（`sha256` 前 16）—— 与 `gapLsLedgerSHA16` **同一套取法**。
// 用于给「我实际分析的那份账」（`led.Raw` · T0）取快照身份，✗ 不是退出前再现读一次。
func gapRereadSHA16(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// gapRereadShowSHA —— 人面展示：退出前重读**读不到**（空串）时给可读占位，✗ 不印空。
func gapRereadShowSHA(s string) string {
	if s == "" {
		return "（重读不到）"
	}
	return s
}

// gapRereadFields —— `--json` 可取字段（每行 = 一条目标账）。★ `unit_rot` = 落点腐检档（既存 + 本枚）。
var gapRereadFields = []string{"id", "fp", "state", "readings", "ok", "reading_stale", "not_rereadable", "skipped", "stale", "unit_rot"}

// gapRereadCiteRe —— 「件:行」引用的形态（件 = 带扩展名的相对路径 · 行 = 十进制）。
var gapRereadCiteRe = regexp.MustCompile(`([A-Za-z0-9_][A-Za-z0-9_./-]*\.[A-Za-z][A-Za-z0-9]*):(\d+)`)

// gapRereadIdentRe —— 够长的标识符（≥6 字符 —— 太短的不当「可机检的锚」，免得假 stale）。
var gapRereadIdentRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{5,}`)

// gapRereadRow —— 一条目标的读数结果（人面 / 机器面同一份数据）。
type gapRereadRow struct {
	ID       string
	FP       string
	State    string
	Readings int
	OK       int
	Stale    int
	NoRun    int
	Skip     int
	Unknown  int
	// FixedNotClosed —— 本条是否「已修未销」（未闭条 verify_cmd 现跑 rc==0）。仅当本条被「点名 /
	// `--include-open`」档判过时才置位；未判 ⇒ 保持 false（`--json` 的 `stale` 格按「在册缺值 ⇒ 整键略去」）。
	FixedNotClosed bool
	// UnitRot / UnitRotNote —— **落点腐检**（只读）：`unit` 与正文 `件:行` 现读的腐档（空 = 无异常）。
	// 值 = `gapRereadUnitRotted` / `gapRereadUnitMissing` 之一；Note = 一行说明（人面印 · `--json` 不入）。
	UnitRot     string
	UnitRotNote string
	Lines       []string
}

// gapRereadOpenRow —— 一条**未闭**条（`--include-open`）的判活读数（人面 / 机器面同一份数据）。
// ★ 本面**不产 `reading_stale`** —— 未闭条没有对拍基线，只有 `ok`（跑通）/ `not_rereadable`（拒跑）两读；
// 跑通里再按现跑 rc 分：rc≠0 ⇒ 症状**仍可复现**；rc==0 ⇒ **已修未销**（`FixedNotClosed`）。
type gapRereadOpenRow struct {
	ID     string
	State  string
	Cmd    string
	Status string // ok / not_rereadable
	RC     string
	Detail string
	// FixedNotClosed —— rc==0 ⇒ 症状已不复现（**已修未销** · 单列）。与 Status 正交（Status 仍记 ok）。
	FixedNotClosed bool
}

// gapRereadReadings —— 从一条 `solved_evidence` 里取出**带键标读数**的候选段（逐条给判定口）。
//
// 两种形态（照拄 `gapTwoStateLanded` 的落法）：
//
//	① 大括号块：`销案两态对拍 · 改前{…} · 改后{…} · 散文=<evidence>` ⇒ 取两个 `{…}` 块；
//	② 老形态：整段正文里直接一段 `cmd=… · rc=… · reading=…` ⇒ 整段交给判定口（解析不出自然落 skipped）。
//
// ✗ 不在这里判 stale（判在 `gapRereadJudge`）：本函数只负责「哪儿有候选」。
func gapRereadReadings(ev string) []string {
	ev = strings.TrimSpace(ev)
	if ev == "" {
		return nil
	}
	out := []string{}
	if strings.Contains(ev, "{") {
		i := 0
		for {
			a := strings.Index(ev[i:], "{")
			if a < 0 {
				break
			}
			a += i
			b := strings.Index(ev[a:], "}")
			if b < 0 {
				break
			}
			b += a
			inner := ev[a+1 : b]
			if strings.Contains(inner, "cmd=") {
				out = append(out, inner)
			}
			i = b + 1
		}
	}
	if len(out) > 0 {
		return out
	}
	if strings.Contains(ev, "cmd=") {
		return []string{ev}
	}
	return nil
}

// gapRereadWhy —— **只读闸**：把一条取值命令解析到命令树，判「能不能安全重跑」。
// 非空 = 不执行（调用方记 `not_rereadable`）；空串 = 可跑。★ 比 `gapRegressRefuse` 更严：
// 危险档一律拒（连带 `--dry-run` 的也拒 —— 本面只跑**非危险档**）。
func gapRereadWhy(cmdStr string) string {
	if m := gapVerifyOneMetaHit(cmdStr); m != "" {
		return fmt.Sprintf("含 shell 元字符 %q（只跑那一格命令本身 · 不拼 shell 串）", m)
	}
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return "空命令"
	}
	if filepath.Base(fields[0]) == progName {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return "只有 `zerg` 前缀、没有命令名"
	}
	cmd, rest := resolve(fields)
	if cmd == nil {
		return fmt.Sprintf("命令树里没有 %q（判不了是不是写面 ⇒ 不执行）", strings.Join(fields, " "))
	}
	if name := strings.Join(cmd.path, " "); name == "gap reread" {
		return "判据是本命令自己（**自递归** ⇒ 不执行）"
	}
	// ★ 本批（批A · P1 丁③）：`gate check <只读判据名>` 命中中央分类表的**只读档** ⇒ 判据本体零副作用
	//   ⇒ 放行。★ 本短路**必须早于**下方 danger 与 passthrough 两道拒 —— 因 `gate check` 节点
	//   自带 `passthrough:true`（`main.go:939`），只改 danger 那条仍会被 passthrough 拒掉。
	//   ✗ 不放宽其余档：透传 / 拒执 / 非开放三档一字不松（判不准仍跳过 ⇒ `not_rereadable`）。
	if gateCheckReadJudgeEscape(cmd.path, rest) {
		return ""
	}
	if cmd.danger != nil {
		return fmt.Sprintf("落在**危险档** %s（写面 ⇒ 不执行；本面只跑非危险档）", cmd.danger.Level)
	}
	// ★ 本闸在**真账现跑**时补上的（实测事故）：`zerg gate run --step … --only-step` 落在
	//   **原样透传型**（`passthrough` —— 参数逐字交给 `scripts/gates/*.sh`），它**不是**危险档
	//   ⇒ 单看 danger 会漏放 ⇒ 真跑一次把整条 `precommit-gates.sh` 门跑起来了（含 shard go-test）。
	//   同族只读闸（`gapRegressRefuse`）也漏这一档 —— 本面**更严**：透传型 / 拒执型 / 非开放型
	//   一律**不执行**（判不准就跳过 ⇒ 记 `not_rereadable`）。
	if cmd.passthrough {
		return "**原样透传型**（gate 族：参数逐字交给被包的脚本 ⇒ 会真跑脚本 / 门跑等重活 ⇒ 不执行）"
	}
	if cmd.refuses || !openedForRun(cmd) {
		return "**今天真跑拒执 / 非开放档**（⇒ 不给结论 · 不执行）"
	}
	return ""
}

// gapRereadCiteProbe —— 「件:行」类读数的**可机检那一部分**：只判**行内容**（那个标识符在件里还在不在），
// ✗ **不判绝对行号**（行号会漂 —— `GAP-20260927-248` 的注逐字：行号漂 7）。
// 只对**仓内已跟踪件**、且能在行号后取到一个**够长标识符**（≥6 字符）时才判；取不到 ⇒ 不判
// （返回 false ⇒ 上层记 rc 一致 —— ✗ 绝不因取不到而报假 stale）。
func gapRereadCiteProbe(reading string, tracked map[string]bool, root string) (bool, string) {
	if root == "" || len(tracked) == 0 || reading == "" {
		return false, ""
	}
	for _, m := range gapRereadCiteRe.FindAllStringSubmatchIndex(reading, -1) {
		file := reading[m[2]:m[3]]
		if !tracked[file] {
			continue
		}
		line := reading[m[4]:m[5]]
		win := reading[m[1]:]
		if len(win) > 80 {
			win = win[:80]
		}
		tok := gapRereadIdentRe.FindString(win)
		if tok == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		if !strings.Contains(string(body), tok) {
			return true, fmt.Sprintf("`%s` 在件 %s 里**已找不到**（只判行内容 · 不判行号 :%s）", tok, file, line)
		}
	}
	return false, ""
}

// gapRereadUnitFallback —— `unit` 兜底档（命令面无件 / 手写拟件 / 空）⇒ ✗ **不判**「落点件缺失」。
func gapRereadUnitFallback(u string) bool {
	return u == "" || strings.HasPrefix(u, "无件") || strings.HasPrefix(u, "拟(")
}

// gapRereadLineNotContent —— 一行现读**不是内容行**（空 / 注释）⇒ 「行号已腐」的信号。
// 注释首 = `//` `#` `/*` `<!--` `*/`（跨 Go / py / sh / yaml / md 的常见注释形态）。
func gapRereadLineNotContent(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	for _, p := range []string{"//", "#", "/*", "<!--", "*/"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// gapRereadCitIndex —— 「件名 → 现读入库路径」的解析面（**主锚 = 件**）：先认**全等**路径，
// 再认**唯一**裸件名（同名多件 ⇒ 该名解析面作废 `""` —— 宁可漏报不误报）。✗ 不改现盘。
func gapRereadCitIndex(tracked map[string]bool) map[string]string {
	base := map[string]string{}
	for f := range tracked {
		b := filepath.Base(f)
		if _, dup := base[b]; dup {
			base[b] = "" // 同名多件 ⇒ 解析面作废（不判）
			continue
		}
		base[b] = f
	}
	return base
}

// gapRereadUnitRot —— **落点腐检**（只读标注 · 以「件」为主锚 · 勘定单推荐 (iii)）：判一条账的落点
// （`unit` 字段）与正文里的 `件:行` 引用。返回 ("", "") = 无异常；否则 (档名, 一行说明)。两档：
//
//	① 落点件缺失 —— `unit`（**非**兜底档）现读**不在盘**（对齐门101/门88 的「落点件在盘」口径 —— 用
//	   盘上存在性，✗ 不另造第二套真源）；现读实证 = M2 退场批里被删的 `scripts/gates/*.py` 等；
//	② 落点行号已腐 —— 件在盘，但 `件:行` 指的**那一行现读不是内容行**（空/注释）或**越界**
//	   （只判「件在不在 · 该行是不是内容」· ✗ 不判绝对行号等于原值 —— 与 `gapRereadCiteProbe` 同口径的**行级**补面）。
//
// ✗ 兜底 `unit`（`无件(命令面)` / 拟件 / 空）**不判**件缺失（现读 721 条是这类 · 误判＝造红）。
// ✗ 本函数**零写盘**（只 `os.Stat` / `os.ReadFile` 现读件）。
func gapRereadUnitRot(r gapRecord, tracked map[string]bool, base map[string]string, root string) (string, string) {
	u := strings.TrimSpace(r.Unit)
	if u != "" && !gapRereadUnitFallback(u) {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(u))); err != nil {
			return gapRereadUnitMissing, fmt.Sprintf("`unit` = %s 现读不在盘（件被删/改名）", u)
		}
	}
	for _, txt := range []string{r.Symptom, r.Handmade, r.Evidence, strings.Join(r.Notes, " ⏎ ")} {
		for _, m := range gapRereadCiteRe.FindAllStringSubmatch(txt, -1) {
			file := m[1]
			resolved := ""
			if tracked[file] {
				resolved = file
			} else {
				resolved = base[filepath.Base(file)] // 空串 = 不判（同名多件 / 不在清单）
			}
			if resolved == "" {
				continue
			}
			n, nerr := strconv.Atoi(m[2])
			if nerr != nil {
				continue
			}
			raw, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(resolved)))
			if rerr != nil {
				continue
			}
			lines := strings.Split(string(raw), "\n")
			if n < 1 || n > len(lines) {
				return gapRereadUnitRotted, fmt.Sprintf("`%s:%s` 越界（件 %s 现读 %d 行 —— 件在盘 · 行号已腐）", file, m[2], resolved, len(lines))
			}
			if gapRereadLineNotContent(lines[n-1]) {
				return gapRereadUnitRotted, fmt.Sprintf("`%s:%s` 现读不是内容行（空/注释 ⇒ 件在盘 · 行号已腐）", file, m[2])
			}
		}
	}
	return "", ""
}

// gapRereadJudge —— 一条带键标读数的判决（**本面唯一判处**）：
// 先判「能不能安全重跑」（只读闸），能跑才重跑并与 `rc` 对拍；rc 一致再看 `件:行` 的行内容。
func gapRereadJudge(cmd, rc, reading string, tracked map[string]bool, root string) (string, string, string) {
	if why := gapRereadWhy(cmd); why != "" {
		return gapRereadNoRun, "", why
	}
	rcRun, _, jwhy := gapVerifyOneRun(cmd)
	if jwhy != "" {
		return gapRereadNoRun, "", jwhy
	}
	got := strconv.Itoa(rcRun)
	if got != strings.TrimSpace(rc) {
		return gapRereadStale, got, fmt.Sprintf("rc 变：记 rc=%s · 现跑 rc=%s", rc, got)
	}
	if stale, note := gapRereadCiteProbe(reading, tracked, root); stale {
		return gapRereadStale, got, note
	}
	return gapRereadOK, got, "rc 一致"
}

// gapRereadOne —— 跑一条目标的全部带键标读数，汇总成一枚 `gapRereadRow`（只读）。
func gapRereadOne(r gapRecord, tracked map[string]bool, root string) gapRereadRow {
	row := gapRereadRow{ID: r.ID, FP: r.FP, State: r.State}
	cands := gapRereadReadings(r.Evidence)
	row.Readings = len(cands)
	for _, c := range cands {
		cmd, rc, reading, why, ok := gapTwoStateParse(c)
		if !ok {
			row.Skip++
			row.Lines = append(row.Lines, "[skipped] 解析不出带键标读数："+why)
			continue
		}
		if !gapRereadCompareOn {
			row.Unknown++
			row.Lines = append(row.Lines, fmt.Sprintf("[无法判] cmd=%s · rc=%s（对拍关 ⇒ 不给结论）", cmd, rc))
			continue
		}
		status, gotRC, detail := gapRereadJudge(cmd, rc, reading, tracked, root)
		switch status {
		case gapRereadOK:
			row.OK++
			row.Lines = append(row.Lines, fmt.Sprintf("[ok] %s · 记 rc=%s · 现跑 rc=%s · %s", cmd, rc, gotRC, detail))
		case gapRereadStale:
			row.Stale++
			row.Lines = append(row.Lines, fmt.Sprintf("[%s] %s · %s", gapRereadStale, cmd, detail))
		default:
			row.NoRun++
			row.Lines = append(row.Lines, fmt.Sprintf("[%s] %s · %s", gapRereadNoRun, cmd, detail))
		}
	}
	return row
}

// gapRereadOpenRefuse —— 未闭条 `verify_cmd` 的**只读闸**（复用同一套安全面 + 补一条「占位串」）。
// 非空 = 不执行（归 `not_rereadable`）；空串 = 可跑。★ 与已解条**同一条** `gapRereadWhy`（危险档 /
// shell 元字符 / 解析不出 / 自递归 ⇒ 拒），本处**只多**一条「占位串」（占位 = 真判据还没建 ⇒ 拒跑）。
func gapRereadOpenRefuse(cmd string) string {
	if strings.Contains(cmd, "占位") {
		return "**占位串**（占位 = 真判据还没建 ⇒ 不执行）"
	}
	return gapRereadWhy(cmd)
}

// gapRereadOpenOne —— 跑一条未闭条（`state == 仍缺`）的 `verify_cmd`（只读 · 判活）。
// ★ 未闭条**无对拍基线** ⇒ 跑通即记 `ok`（detail 给现跑 rc + 读数行）；✗ 不判 `reading_stale`。
func gapRereadOpenOne(r gapRecord) (status, rc, detail string, fixedNotClosed bool) {
	cmd := strings.TrimSpace(r.VerifyCmd)
	if why := gapRereadOpenRefuse(cmd); why != "" {
		return gapRereadNoRun, "", why, false
	}
	rcRun, out, jwhy := gapVerifyOneRun(cmd)
	if jwhy != "" {
		return gapRereadNoRun, "", jwhy, false
	}
	reading := gapVerifyOneReading(rcRun, out)
	if rcRun == 0 {
		// ★ 本枚（缺口 `GAP-20260929-12`）：rc==0 ⇒ 症状**已不复现** ⇒ 「已修未销」（单列）。
		return gapRereadOK, strconv.Itoa(rcRun), reading + "（症状已不复现 ⇒ " + gapRereadFixedNotClosed + " · 无对拍基线）", true
	}
	return gapRereadOK, strconv.Itoa(rcRun), reading + "（症状仍可复现 · 无对拍基线）", false
}

func cmdGapReread(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapRereadFields, ","))
		return exitUsage
	}
	ids := gapIDsOf(inv)
	if inv.all && len(ids) > 0 {
		inv.setErr("usage", "target_arity", "--all 与点名 id 不许同给")
		fmt.Fprintf(stderr, "%s: `--all` 与点名 id **不许同给**（要一条一条跑就逐个给 id）\n", progName)
		return exitUsage
	}
	if !inv.all && len(ids) == 0 {
		inv.setErr("usage", "target_required", "缺目标")
		fmt.Fprintf(stderr, "%s: 要给目标：`zerg gap reread <GAP id>…` 或 `--all`\n", progName)
		// ★ 运行期用法行（与本命令命令树 `usage:` 同形 —— 含 `--include-open`）：门73 面⑥ 按
		//   「件内 `用法：zerg …` 运行期用法行 ⟷ 命令树同命令用法串」双向对拍（缺口 `GAP-20260928-183`）。
		fmt.Fprintf(stderr, "用法：zerg gap reread [<GAP id>…] [--all] [--include-open] [--limit <N>] [--json <字段>]\n")
		fmt.Fprintf(stderr, "  例   : `zerg gap reread GAP-20260927-248`\n")
		fmt.Fprintf(stderr, "  说明 : 只复跑 `solved_evidence` 里的带键标读数（只读 · 一字节不写）· reading_stale **不进退出码**\n")
		return exitUsage
	}
	limit := 0
	if v := strings.TrimSpace(inv.flagVal("--limit")); v != "" {
		n, lerr := strconv.Atoi(v)
		if lerr != nil || n < 0 {
			inv.setErr("usage", "bad_limit", "--limit 要给非负整数")
			fmt.Fprintf(stderr, "%s: `--limit %s` 不是非负整数（本族口径 = 用法错 2）\n", progName, v)
			return exitUsage
		}
		limit = n
	}
	includeOpen := inv.hasFlag("--include-open")

	// ② 读真源（读不到 ⇒ 8 · 不许当绿）· 两种因机器可辨（`Q-138`）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ③ 目标集（点名 id ⇒ 账内没有 = 2 · `--all` ⇒ 全账）
	tgts := []int{}
	if inv.all {
		for i := range led.Recs {
			tgts = append(tgts, i)
		}
	} else {
		for _, id := range ids {
			i := gapFindIdx(led, id)
			if i < 0 {
				inv.setErr("usage", "target_not_found", "账内没有 "+id)
				fmt.Fprintf(stderr, "%s: 账内没有 %s（账里 %d 条）—— 「名给错」是**用法错** ⇒ 退码 2\n",
					progName, id, len(led.Recs))
				fmt.Fprintf(stderr, "  看账 : `zerg gap ls`（本命令**只读** · 一字节不写）\n")
				return exitUsage
			}
			tgts = append(tgts, i)
		}
	}

	// ④ 件清单（现读一次 · 给 `件:行` 行内容判定用）
	root := repoRoot()
	tracked := map[string]bool{}
	for _, f := range gapTrackedFiles() {
		tracked[f] = true
	}
	// ★ 落点腐检的**裸件名解析面**（件在盘而正文只写裸件名 —— 现读实证 `-20260927-301` 写 `family_code.go:54`）。
	citBase := gapRereadCitIndex(tracked)

	// ⑤ 逐条跑（`--limit` 卡的是**读数条数**，防在真账上一次跑太多）
	rows := []gapRereadRow{}
	processed := 0
	capped := false
	var nRead, nOK, nStale, nNoRun, nSkip, nUnk int
	var nUnitMissing, nUnitRotted int
	for _, idx := range tgts {
		if limit > 0 && processed >= limit {
			capped = true
			break
		}
		row := gapRereadOne(led.Recs[idx], tracked, root)
		// ★ 落点腐检（只读标注 · 与 `--limit` 无关 —— 每目标条都判）：件缺失 / 行号已腐。
		row.UnitRot, row.UnitRotNote = gapRereadUnitRot(led.Recs[idx], tracked, citBase, root)
		if row.UnitRot == gapRereadUnitMissing {
			nUnitMissing++
		}
		if row.UnitRot == gapRereadUnitRotted {
			nUnitRotted++
		}
		processed += row.Readings
		nRead += row.Readings
		nOK += row.OK
		nStale += row.Stale
		nNoRun += row.NoRun
		nSkip += row.Skip
		nUnk += row.Unknown
		rows = append(rows, row)
	}
	// ⑤b **未闭判活面**（`--include-open` / 点名档）：把**未闭**（`state == 仍缺`）条目的 `verify_cmd`
	//   纳入**同一套读数** ——「这条缺口今天还在不在」不必再靠人眼估。★ 安全面一字不放宽
	//   （危险档 / 元字符 / 占位 / 解析不出 / 自递归 ⇒ 一律拒跑记 `not_rereadable`）。
	//   ★ 本枚（缺口 `GAP-20260929-12`）：**点名档（非 `--all`）默认也判** —— want「输入一组账条
	//     ⇒ 逐条现读跑『症状是否仍可复现』并把『已修未销』的条目单列」；`--all`（全账）为控成本仍只
	//     走 `solved_evidence`（要未闭判活给它 `--include-open`）。rc==0 ⇒ **已修未销**（症状已不
	//     复现而 `state` 仍 未闭）⇒ 单列；rc≠0 ⇒ 症状仍可复现。
	judgeOpen := includeOpen || !inv.all
	openRows := []gapRereadOpenRow{}
	openTotal, openRan, openRefused, openTrunc, openStale := 0, 0, 0, 0, 0
	openConserved := true
	if judgeOpen {
		openIdx := []int{}
		for _, idx := range tgts {
			if led.Recs[idx].State == gapStOpen {
				openIdx = append(openIdx, idx)
			}
		}
		openTotal = len(openIdx)
		processedOpen := 0
		for _, idx := range openIdx {
			if limit > 0 && processedOpen >= limit {
				openTrunc = openTotal - processedOpen
				break
			}
			rec := led.Recs[idx]
			st, rcRun, detail, fnc := gapRereadOpenOne(rec)
			if st == gapRereadOK {
				openRan++
				if fnc {
					openStale++
				}
			} else {
				openRefused++
			}
			openRows = append(openRows, gapRereadOpenRow{
				ID: rec.ID, State: rec.State, Cmd: strings.TrimSpace(rec.VerifyCmd),
				Status: st, RC: rcRun, Detail: detail, FixedNotClosed: fnc})
			processedOpen++
		}
		openConserved = openTotal == openRan+openRefused+openTrunc
	}
	// ★ 「已修未销」落回目标行（`--json` 的 `stale` 格）：只有判过的条才置位（未判 ⇒ 缺键）。
	staleSet := map[string]bool{}
	for _, r := range openRows {
		if r.FixedNotClosed {
			staleSet[r.ID] = true
		}
	}
	for i := range rows {
		if staleSet[rows[i].ID] {
			rows[i].FixedNotClosed = true
		}
	}
	// ★ **外写自发现**（缺口 `GAP-20261005-44`）：上面一切读数都基于 T0 读入的 `led`（`led.Recs`），
	//   而旧版把 `ledger_sha16` 在**同一刻**（T1）现读一次 ⇒ 「我分析的账」与「印出去的 sha」**不互校**，
	//   两次读数之间的外部写入**不可见**（只能靠隔离复跑才判得出 = 偶然而非机制）。
	//   治：① T0 快照 = `led.Raw` 的 sha16 + 行数（固定值）；② 退出前**再读一次**真源；③ 与快照比，
	//   不符 ⇒ 机读格 `external_write_detected=true` + 两个 sha16（人面同步印）。✗ 不动四档语义、
	//   ✗ 不进退出码（与 reading_stale 同口径：只报告）。
	snapSHA16 := gapRereadSHA16(led.Raw)
	snapLines := len(led.Lines)
	if gapRereadBetweenReads != nil {
		gapRereadBetweenReads() // 机检钩子：模拟外部写入（生产恒 nil）
	}
	exitSHA16 := gapLsLedgerSHA16(led.Path)
	extWrite := exitSHA16 != snapSHA16

	inv.changed = boolPtr(false)

	// ⑥ 机器面信封（只走 `metaAdd*` 既有口子 ⇒ 顶层六键不动）
	inv.metaAddJSON("total", strconv.Itoa(nRead))
	inv.metaAddJSON("ok", strconv.Itoa(nOK))
	inv.metaAddJSON("reading_stale", strconv.Itoa(nStale))
	inv.metaAddJSON("not_rereadable", strconv.Itoa(nNoRun))
	inv.metaAddJSON("skipped", strconv.Itoa(nSkip))
	inv.metaAddStr("query_ts", gapNow())
	// ★ `ledger_sha16` 现在记的是**我分析的那份账**（T0 快照 · `led.Raw`）的身份 —— 与下面 `snapshot_*`
	//   同源；保留此键是为与同族五格**同形**（`gap ls` 一族）。真源到底动没动由 `external_write_detected` 报。
	inv.metaAddStr("ledger_sha16", snapSHA16)
	inv.metaAddStr("snapshot_sha16", snapSHA16)
	inv.metaAddJSON("snapshot_lines", strconv.Itoa(snapLines))
	inv.metaAddStr("readback_sha16", exitSHA16)
	inv.metaAddJSON("external_write_detected", strconv.FormatBool(extWrite))
	// ★ 落点腐检两档计数（只读标注 · 不进退出码 · 现读现算）。
	inv.metaAddJSON("unit_missing", strconv.Itoa(nUnitMissing))
	inv.metaAddJSON("unit_rotted", strconv.Itoa(nUnitRotted))
	// ★ 未闭判活面的**守恒三数 + 逐条行**走同族信封的 `meta`（无新增字段白名单 ✗）。
	if judgeOpen {
		inv.metaAddJSON("open_total", strconv.Itoa(openTotal))
		inv.metaAddJSON("open_ran", strconv.Itoa(openRan))
		inv.metaAddJSON("open_refused", strconv.Itoa(openRefused))
		inv.metaAddJSON("open_truncated", strconv.Itoa(openTrunc))
		inv.metaAddJSON("open_conserved", strconv.FormatBool(openConserved))
		inv.metaAddJSON("open_stale", strconv.Itoa(openStale)) // ★ 已修未销条数（缺口 GAP-20260929-12）
		openLines := make([]string, 0, len(openRows))
		for _, r := range openRows {
			openLines = append(openLines, fmt.Sprintf("%s [%s] %s · %s", r.ID, r.Status, r.Cmd, r.Detail))
		}
		inv.metaAddStrings("open_lines", openLines)
	}

	if inv.jsonGiven {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
		js := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			m := map[string]string{
				"id": row.ID, "fp": row.FP, "state": row.State,
				"readings": strconv.Itoa(row.Readings), "ok": strconv.Itoa(row.OK),
				"reading_stale": strconv.Itoa(row.Stale), "not_rereadable": strconv.Itoa(row.NoRun),
				"skipped": strconv.Itoa(row.Skip),
			}
			if judgeOpen {
				// ★ 已修未销（缺口 GAP-20260929-12）：判过的条才给这一格（未判 ⇒ 整键略去）。
				m["stale"] = boolWord(row.FixedNotClosed)
			}
			// ★ 落点腐检档（既存常给 `ok`）：`ok` / `落点件缺失` / `落点行号已腐`。
			if row.UnitRot == "" {
				m["unit_rot"] = "ok"
			} else {
				m["unit_rot"] = row.UnitRot
			}
			js = append(js, m)
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, js)
	}

	// ⑦ 人面（表尾逐字声称：**只报告 · reading_stale 不进退出码**）
	fmt.Fprintf(stdout, "缺口判据复跑（只读面 · 重跑 solved_evidence 里的带键标读数 · 只报告不改账）\n")
	fmt.Fprintf(stdout, "  真源     : %s（快照 %d 行 · snapshot_sha16 = %s）\n",
		led.Path, snapLines, snapSHA16)
	fmt.Fprintf(stdout, "  外写自校 : external_write_detected=%t（snapshot_sha16=%s · readback_sha16=%s）\n",
		extWrite, snapSHA16, gapRereadShowSHA(exitSHA16))
	if extWrite {
		fmt.Fprintf(stdout, "             ⚠ 两次读数之间真源被**外部**改过（上面读数一律基于快照那一份）\n")
	}
	fmt.Fprintf(stdout, "  目标     : %d 条（%s）\n", len(rows), targetLabelOf(inv.all, ids))
	fmt.Fprintf(stdout, "  读数     : 共 %d 条（判定面 %d）\n", nRead, nRead-nSkip)
	fmt.Fprintf(stdout, "  结果     : ok %d · reading_stale %d · not_rereadable %d · skipped %d\n",
		nOK, nStale, nNoRun, nSkip)
	fmt.Fprintf(stdout, "  落点腐检 : 落点件缺失 %d · 落点行号已腐 %d（只读标注 · 不进退出码 · 以「件」为主锚 · ✗ 不判绝对行号）\n",
		nUnitMissing, nUnitRotted)
	if !gapRereadCompareOn {
		fmt.Fprintf(stdout, "  ⚠ 对拍 **关**（负控档）⇒ 全部可解析读数记「%s」（不给结论）\n", gapRereadUnknown)
	}
	if capped {
		fmt.Fprintf(stdout, "  ⚠ `--limit %d` 已卡（其后读数**未跑** · 这不是全量）\n", limit)
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "  条 %s（state=%s · 读数 %d：ok %d · stale %d · not_rereadable %d · skipped %d）\n",
			row.ID, row.State, row.Readings, row.OK, row.Stale, row.NoRun, row.Skip)
		for _, l := range row.Lines {
			fmt.Fprintf(stdout, "    %s\n", l)
		}
		if row.UnitRot != "" {
			fmt.Fprintf(stdout, "    ★ %s：%s\n", row.UnitRot, row.UnitRotNote)
		}
	}
	if judgeOpen {
		fmt.Fprintf(stdout, "  未闭判活（点名/--include-open · 逐条跑未闭账的 verify_cmd · 只读报告档）\n")
		for _, r := range openRows {
			fmt.Fprintf(stdout, "    条 %s（state=%s · verify_cmd=%s）: [%s] %s\n",
				r.ID, r.State, r.Cmd, r.Status, r.Detail)
		}
		verdict := "守恒成立"
		if !openConserved {
			verdict = "✗ 不守恒（报出来 · 不给结论）"
		}
		fmt.Fprintf(stdout, "  未闭守恒: 未闭总数 %d == 已跑 %d + 拒跑 %d + 截断 %d ⇒ %s\n",
			openTotal, openRan, openRefused, openTrunc, verdict)
		fmt.Fprintf(stdout, "             ★ 未闭条**无对拍基线** ⇒ 跑通即记 `%s`（✗ 不判 reading_stale）· 安全面一字不放宽\n", gapRereadOK)
		// ★ 已修未销单列（缺口 `GAP-20260929-12`）：症状已不复现而 state 仍 未闭 ⇒ 待销案。
		if openStale > 0 {
			fmt.Fprintf(stdout, "  已修未销（单列 · 症状已不复现而 state 仍 %s —— 待销案）：%d 条\n", gapStOpen, openStale)
			for _, r := range openRows {
				if r.FixedNotClosed {
					fmt.Fprintf(stdout, "    %s（state=%s · verify_cmd=%s）: [%s] %s\n",
						r.ID, r.State, r.Cmd, r.Status, r.Detail)
				}
			}
		}
	}
	fmt.Fprintf(stdout, "  覆盖边界: 只按「条」现读跑账内那一格判据**本身**（✗ 不拼 shell 串 · ✗ 不覆盖非确定性/"+
		"环境依赖场景）；点名档逐条判未闭条、`--all` 只走 `solved_evidence`（未闭条判活走 `--include-open`）\n")
	fmt.Fprintf(stdout, "  表尾声称: **只报告** —— reading_stale / not_rereadable / 已修未销 **均不进退出码**"+
		"（免得存量陈旧读数把它变成恒红）· 本面一字节都不写真源、不写审计\n")
	return exitOK
}

// targetLabelOf —— 人面那一行「目标」的口径说明。
func targetLabelOf(all bool, ids []string) string {
	if all {
		return "--all（全账）"
	}
	if len(ids) == 1 {
		return "点名 " + ids[0]
	}
	return fmt.Sprintf("点名 %d 条：%s", len(ids), strings.Join(ids, " "))
}

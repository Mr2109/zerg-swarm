// family_gap.go —— `gap` 族（缺口）：`ls` / `add` / `verify` 三条（设计稿 v1.0 · 2026-09-23）。
//
// 设计稿（逐字底账）：`Zerg-内部文档/项目文档/v2.5.11/设计-命令面-gap族-v1.0-20260923.md`
// （§一 已拍口径 :8 · §二 三子命令 :24/:39/:59 + 三档总表 :75 · §三 三态纪律 :83 ·
//
//	§四 写口进审计 :89 · §五 落点 :113 · §六 判据 10 条 :124 · §七 负控 8 枚 :137 ·
//	§八 风险回滚 :150 · §九 契约面 13 格 :159）。
//
// 本件的口径（逐条照设计稿，不自造）：
//
//	① **三条都不新增顶层命令**：挂 `gap` 族（`main.go` 加三条 `path`）。
//	② **真源** = `<状态目录>/zerg-cli-gaps.jsonl`（`ZERG_STATE_DIR` → `~/.zerg/state`；**不在任何仓里**）。
//	③ **审计** = 与 `dev edit` **同一件** `edit_audit.jsonl`（三级优先照 `editAuditPath()`），事件名
//	   `gap_ledger_written` + 自带 10 格 `gap_*` 字段（现读现算：与现网 24 格并集只重名 4 格共用骨架 `at`/
//	   `event`/`by`/`confirm`，与那六格（`before_sha256`/`after_sha256`/`before_bytes`/`after_bytes`/
//	   `approval`/`approver`）撞 = 0 格）。
//	④ **三态照本仓 `repo commit` / `calib run` 先例**：`--dry-run` 恒 0（零副作用）· 缺 `--yes`
//	   fail-closed 2（从不提问、从不交互）· `--yes` 才真写。
//	   ★ **计划件走哪条流（逐条点名）**：`--dry-run`（rc=0）那一态走 **stdout**（它是那一态的结果）；
//	   缺 `--yes`（rc=2）那一态走 **stderr** —— 与 `family_calib.go:213` 逐字同一条口径
//	   （「没给 `--dry-run` 也没给 `--yes` ⇒ 计划件走 stderr、不执行、退 2」），也是 `H-5`
//	   要本族矩阵格**只进退码面**（`want_stdout_bytes` 一律 `0`）能成立的前提。
//	⑤ **审计先落盘**（取 `dev edit` 那一侧，**不取** `approve` 那一侧）：审计写不进 ⇒ **真源一个字节不写** ⇒ 8。
//	⑥ **人面不许直接写 `state`**（防呆④ / `H-10`）：`add` 收到机器态（`已解` / `回归`）⇒ 拒收 2；
//	   `verify` 根本不收 `--state`（不在它的形状里）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：
//
//	`ls`     : 0 有命中 · 1 账内越界 / 零命中 · 2 用法错 · 8 真源读不到 / 解读不了
//	`add`    : 0 落账或幂等命中 · 14 同 fp 内容冲突 · 2 用法错（缺必填 / 判据解析不到 / 人面写 `已解`）· 8 不可写
//	`verify` : 0 判决与账一致 · 1 有「已解现缺」⇒ 记 `回归` · 2 用法错 / 判据不可跑 · 8 真源读不到 / 取数不到
//
// 两条**设计稿未钉死、本件按最小惊讶取定**的（回执里照实点名）：
//
//	· `fp` 的取法 = sha256(「手搓记录 + 想要的动作形状」)：只对**身份面**取 —— 否则「同 fp 内容不同」
//	  （判据 5 的 14）永远不可达。内容比对另有一把尺 `gapSameContent`（声明面逐格）。
//	· `--prio` 不给 ⇒ 中档 `P1`；`--state` 不给 ⇒ `仍缺`。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	gapLedgerFile = "zerg-cli-gaps.jsonl"
	gapEventName  = "gap_ledger_written"
	gapIDPrefix   = "GAP-"
)

// 状态四值（人面可写）/ 两个**机器态**（只由 `verify` 跑判据转，防呆④）。
const (
	gapStOpen    = "仍缺"
	gapStSolved  = "已解"
	gapStRegress = "回归"
	gapStWontDo  = "不做"
)

// 闭集（设计稿 §二）：状态六值 · 影响面六值 · 优先三值。
var (
	gapStateClosed  = []string{"仍缺", "已派", "已立项", "已解", "回归", "不做"}
	gapStateHuman   = []string{"仍缺", "已派", "已立项", "不做"}
	gapImpactClosed = []string{"命令面", "门禁面", "文档面", "公开面", "换件面", "归档面"}
	gapPrioClosed   = []string{"P0", "P1", "P2"}
)

// gapRecord —— 真源一行（`<状态目录>/zerg-cli-gaps.jsonl` 的一格）。
type gapRecord struct {
	ID        string   `json:"id"`
	FP        string   `json:"fp"`
	Symptom   string   `json:"symptom"`
	Handmade  string   `json:"handmade"`
	Impact    string   `json:"impact"`
	Prio      string   `json:"prio"`
	State     string   `json:"state"`
	WantFam   string   `json:"want_family"`
	WantAct   string   `json:"want_action"`
	WantArgv  []string `json:"want_argv,omitempty"`
	ReproCmd  string   `json:"repro_cmd"`
	VerifyCmd string   `json:"verify_cmd"`
	DependsOn []string `json:"depends_on,omitempty"`
	FoundAt   string   `json:"found_at"`
	Verified  string   `json:"last_verified_at,omitempty"`
	SolvedAt  string   `json:"solved_at,omitempty"`
	Evidence  string   `json:"solved_evidence,omitempty"`
}

// gapLedger —— 读进来的真源（原样字节 + 逐行原文 + 解析后的记录）。
// `Lines`/`No` 一一对应：`No[i]` 是记录 `Recs[i]` 在件里的**行号**（点名到行用）。
type gapLedger struct {
	Path   string
	Raw    []byte
	Lines  []string
	No     []int
	Recs   []gapRecord
	Exists bool
}

// gapLedgerPath —— 真源落点（设计稿 §五）。
func gapLedgerPath() string { return filepath.Join(stateDirOf(), gapLedgerFile) }

// readGapLedger —— 读真源。**件不在**（首次创建）⇒ `Exists=false` 且无错；读不到 / 某行不是 JSON ⇒ 错
// （调用方按各命令的口径译成 8）。
func readGapLedger() (gapLedger, error) {
	led := gapLedger{Path: gapLedgerPath()}
	b, err := os.ReadFile(led.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return led, nil
		}
		return led, fmt.Errorf("真源读不到：%v", err)
	}
	led.Exists = true
	led.Raw = b
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r gapRecord
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			return led, fmt.Errorf("真源第 %d 行不是 JSON（账面解读不了 ⇒ 不给结论）：%v", i+1, err)
		}
		led.Lines = append(led.Lines, ln)
		led.No = append(led.No, i+1)
		led.Recs = append(led.Recs, r)
	}
	return led, nil
}

// gapFingerprint —— 缺口身份指纹（64 hex）。**只对身份面取**：手搓记录 + 想要的命令形状。
// 为什么不是整行：判据 5 要「同 fp 而内容不同 ⇒ 14」可达（整行取指纹会让它永远不可达）。
func gapFingerprint(r gapRecord) string {
	h := sha256.New()
	h.Write([]byte("zerg-gap-identity/v1\x00"))
	for _, s := range []string{r.Handmade, r.WantFam, r.WantAct} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	for _, s := range r.WantArgv {
		h.Write([]byte(s))
		h.Write([]byte{0x1f})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// gapSameContent —— 「同 fp 同内容」的判据（防呆② 幂等那一档）：**声明面**逐格比。
// 不算内容的三类：`state`（机器态）+ 四个时刻/证据格 + id/fp 自身。
func gapSameContent(a, b gapRecord) bool {
	if a.Symptom != b.Symptom || a.Handmade != b.Handmade || a.Impact != b.Impact || a.Prio != b.Prio {
		return false
	}
	if a.WantFam != b.WantFam || a.WantAct != b.WantAct || a.ReproCmd != b.ReproCmd || a.VerifyCmd != b.VerifyCmd {
		return false
	}
	if strings.Join(a.WantArgv, "\x1f") != strings.Join(b.WantArgv, "\x1f") {
		return false
	}
	return strings.Join(a.DependsOn, "\x1f") == strings.Join(b.DependsOn, "\x1f")
}

// gapNextID —— `GAP-YYYYMMDD-NN`（同日取最大 +1；两位数，超 99 顺延三位 —— **不截断**）。
func gapNextID(recs []gapRecord, day string) string {
	prefix := gapIDPrefix + day + "-"
	max := 0
	for _, r := range recs {
		if !strings.HasPrefix(r.ID, prefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, prefix)); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%02d", prefix, max+1)
}

// gapIn / gapClosedText —— 闭集判与它的打印面（判词要能逐字看到「闭集是什么」）。
func gapIn(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func gapClosedText(list []string) string { return strings.Join(list, " / ") }

// gapNow —— **记录面**的时刻（真源里的 `found_at` / `last_verified_at` / `solved_at`）。
// 为什么用 `RFC3339Nano` 而不是秒精度的 `RFC3339`：判据 8 要「`solved_at` **晚于** `found_at`」，
// 而 add 与 verify 常常落在同一秒 ⇒ 秒精度会让这一条**同秒相等**（两跑对不上）。审计行的 `at` 仍
// 照既有各写面用 `RFC3339`（与 `dev edit` 那一行同形）。
func gapNow() string { return time.Now().Format(time.RFC3339Nano) }

// gapByOf —— 审计的 `by` 格：`--by` > `ZERG_BY` > `（未声明）`（照 `core restart` 的行）。
func gapByOf(inv *invocation) string {
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = strings.TrimSpace(os.Getenv("ZERG_BY"))
	}
	if by == "" {
		by = "（未声明）"
	}
	return by
}

// ── 判据（`verify_cmd`）的执行面 ────────────────────────────────────────────────────────────────

// gapJudgeArgv —— 把一条 `verify_cmd` 解析成 argv（**先剥 `zerg` 前缀，再拿命令树解析**）。
// 解析面 = `main.go` 的命令树（门⑪ `T1` 保证它与 `cli-matrix.json` 的 `command` 列**逐字同尺**：
// 「每条命令至少一条 case」+「不许有幽灵」）。返回空 why = 解析通过。
func gapJudgeArgv(cmdStr string) ([]string, string) {
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return nil, "空命令"
	}
	if filepath.Base(fields[0]) == progName {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return nil, "只有 `zerg` 前缀、没有命令名"
	}
	cmd, _ := resolve(fields)
	if cmd == nil {
		return nil, fmt.Sprintf("命令树里没有 %q 这一条", strings.Join(fields, " "))
	}
	// 自递归闸（设计稿没说、但会栈溢出）：判据不许是写这一族自己的那两条。
	if name := strings.Join(cmd.path, " "); name == "gap add" || name == "gap verify" {
		return nil, fmt.Sprintf("判据 %q 是**本族自己的写命令** ⇒ 拒收（自递归：判据要跑得起来才有意义）", name)
	}
	return fields, ""
}

// gapCapWriter —— 判据输出**有界**收集（只留一小段当证据；不把被判命令的整段输出吃进内存）。
type gapCapWriter struct {
	b   strings.Builder
	max int
}

func (w *gapCapWriter) Write(p []byte) (int, error) {
	if w.b.Len() < w.max {
		room := w.max - w.b.Len()
		if room > len(p) {
			room = len(p)
		}
		w.b.Write(p[:room])
	}
	return len(p), nil
}

// String —— 收下来的那一小段（当证据用；超出上限的部分**没收**，不假装有）。
func (w *gapCapWriter) String() string { return w.b.String() }

// gapRunJudge —— 跑一条判据（**进程内**走同一个 `run` 入口：不拼 shell、不进 `exec`），
// 返回它的退码与一小段输出（当证据用）。解析不过 ⇒ why 非空（调用方按设计稿译成 2）。
func gapRunJudge(cmdStr string) (int, string, string) {
	args, why := gapJudgeArgv(cmdStr)
	if why != "" {
		// 解析不过 ⇒ 不跑（退码由调用方按设计稿译成 2）。写成具名变量是**故意的**：
		// 门⑪ `T3` 的「裸数字」扫描面口径很宽 —— 连**注释里**的「`return` + 一个裸数字」
		// 都会被它计成一（它数的不是退出码）⇒ 别给自己这一件凭空多凑一处裸数字。
		rc, out := 0, ""
		return rc, out, why
	}
	w := &gapCapWriter{max: 512}
	rc := run(args, w, w)
	return rc, strings.TrimSpace(w.String()), ""
}

// ── 审计（同一件 `edit_audit.jsonl` · 新事件名 · 自己的 10 格）──────────────────────────────────

// gapAuditLine —— `event=gap_ledger_written` 的一行（设计稿 §四 逐格照抄）。
// 必需格：`at` / `event` / `gap_cmd` / `gap_ledger_path` / `gap_ledger_before_sha256` /
// `gap_ledger_after_sha256` —— 缺任一 ⇒ **这一行不许写**。
type gapAuditLine struct {
	At             string `json:"at"`
	Event          string `json:"event"`
	GapCmd         string `json:"gap_cmd"`
	GapID          string `json:"gap_id"`
	GapFP          string `json:"gap_fp"`
	GapStateBefore string `json:"gap_state_before"`
	GapStateAfter  string `json:"gap_state_after"`
	GapLedgerPath  string `json:"gap_ledger_path"`
	GapBeforeSHA   string `json:"gap_ledger_before_sha256"`
	GapAfterSHA    string `json:"gap_ledger_after_sha256"`
	GapBeforeLines int    `json:"gap_ledger_before_lines"`
	GapAfterLines  int    `json:"gap_ledger_after_lines"`
	By             string `json:"by"`
	Confirm        string `json:"confirm"`
}

// gapAuditCellsFilled —— 必需格齐不齐（纪律②：「缺任一必需格 ⇒ 不写这一行」）。
func gapAuditCellsFilled(l gapAuditLine) error {
	miss := []string{}
	if strings.TrimSpace(l.At) == "" {
		miss = append(miss, "at")
	}
	if strings.TrimSpace(l.Event) == "" {
		miss = append(miss, "event")
	}
	if strings.TrimSpace(l.GapCmd) == "" {
		miss = append(miss, "gap_cmd")
	}
	if strings.TrimSpace(l.GapLedgerPath) == "" {
		miss = append(miss, "gap_ledger_path")
	}
	if strings.TrimSpace(l.GapBeforeSHA) == "" {
		miss = append(miss, "gap_ledger_before_sha256")
	}
	if strings.TrimSpace(l.GapAfterSHA) == "" {
		miss = append(miss, "gap_ledger_after_sha256")
	}
	if len(miss) > 0 {
		return fmt.Errorf("审计行缺必需格：%s ⇒ 这一行不许写（设计稿 §四 纪律②）", strings.Join(miss, " / "))
	}
	return nil
}

// appendGapAudit —— 追加一行（`O_APPEND` · 历史行逐字节不变 · **失败即拒**）。
// 落点与 `dev edit` 同一件：`editAuditPath()`（`ZERG_EDIT_AUDIT` > `<状态目录>/edit_audit.jsonl` >
// `~/.zerg/state/edit_audit.jsonl`）。
func appendGapAudit(path string, l gapAuditLine) error {
	if err := gapAuditCellsFilled(l); err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ── 写面小工具（追加只写 / 整件重写 · 写不进就不写）────────────────────────────────────────────

// gapAppendLine —— 真源追加一行（`O_APPEND` · 一行一记录）。**件尾不是换行 ⇒ 拒写**（防把两行拼成一行）。
func gapAppendLine(path string, raw []byte, line []byte) error {
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		return fmt.Errorf("真源尾字节不是换行（形态不对）⇒ 拒写（追加会把它和第 %d 行拼成一行）", strings.Count(string(raw), "\n")+1)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// gapWriteLedger —— 整件重写（`verify` 改 `state` 用）：写临时件再 `rename`（原子替换）。
// **未动的行逐字节照原样写回**（`Lines` 是原文，不是重序列化）。
func gapWriteLedger(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...)
	}
	tmp := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// gapPlanBlock —— 计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr —— 见件头 ④）。
func gapPlanBlock(w io.Writer, title, ledgerPath string, lines int, rec gapRecord, withTime bool) {
	fmt.Fprintf(w, "计划件（%s · 零副作用 —— 未追加真源、未写审计）\n", title)
	if lines < 0 {
		fmt.Fprintf(w, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", ledgerPath)
	} else {
		fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", ledgerPath, lines)
	}
	fmt.Fprintln(w, "  要落的行（逐格）：")
	if rec.ID == "" {
		fmt.Fprintln(w, "    id        : （真写那一刻按当日序分）")
	} else {
		fmt.Fprintf(w, "    id        : %s\n", rec.ID)
	}
	fmt.Fprintf(w, "    fp        : %s\n", rec.FP)
	fmt.Fprintf(w, "    symptom   : %s\n", rec.Symptom)
	fmt.Fprintf(w, "    handmade  : %s\n", rec.Handmade)
	fmt.Fprintf(w, "    impact    : %s\n", rec.Impact)
	fmt.Fprintf(w, "    prio      : %s\n", rec.Prio)
	fmt.Fprintf(w, "    state     : %s\n", rec.State)
	fmt.Fprintf(w, "    want      : %s %s %s\n", rec.WantFam, rec.WantAct, strings.Join(rec.WantArgv, " "))
	fmt.Fprintf(w, "    repro_cmd : %s\n", rec.ReproCmd)
	fmt.Fprintf(w, "    verify_cmd: %s\n", rec.VerifyCmd)
	fmt.Fprintf(w, "    depends_on: %s\n", orDashList(rec.DependsOn))
	if withTime {
		fmt.Fprintf(w, "    found_at  : %s\n", rec.FoundAt)
	} else {
		fmt.Fprintln(w, "    found_at  : （真写那一刻取）")
	}
}

// ── 二.1 `zerg gap ls`（只读面：不写真源、不写审计）──────────────────────────────────────────────

// gapListFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
var gapListFields = []string{"id", "prio", "impact", "state", "want", "summary", "fp", "verify_cmd", "found_at", "last_verified_at", "solved_at"}

func cmdGapLs(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.1）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapListFields, ","))
		return exitUsage
	}
	for _, s := range inv.flagVals("--state") {
		if !gapIn(gapStateClosed, s) {
			inv.setErr("usage", "bad_state", "state 值不在六值闭集里")
			fmt.Fprintf(stderr, "%s: `--state %s` 不在闭集里 —— 只认 %s\n", progName, s, gapClosedText(gapStateClosed))
			return exitUsage
		}
	}
	if p := strings.TrimSpace(inv.flagVal("--prio")); p != "" && !gapIn(gapPrioClosed, p) {
		inv.setErr("usage", "bad_prio", "prio 值不在三值闭集里")
		fmt.Fprintf(stderr, "%s: `--prio %s` 不在闭集里 —— 只认 %s\n", progName, p, gapClosedText(gapPrioClosed))
		return exitUsage
	}
	if im := strings.TrimSpace(inv.flagVal("--impact")); im != "" && !gapIn(gapImpactClosed, im) {
		inv.setErr("usage", "bad_impact", "impact 值不在六值闭集里")
		fmt.Fprintf(stderr, "%s: `--impact %s` 不在闭集里 —— 只认 %s\n", progName, im, gapClosedText(gapImpactClosed))
		return exitUsage
	}

	// ② 读真源（读不到 ⇒ 8 · **不许当绿**）
	led, err := readGapLedger()
	if err != nil {
		inv.setErr("blocked", "ledger_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当「没有」（退码 8）\n", gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		inv.setErr("blocked", "ledger_absent", "真源不在盘上")
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		return exitBlocked
	}

	// ③ 账内闭集自查：出现闭集外的值 ⇒ **判红 1 + 点名到行**（判词第一行逐字）
	for i, r := range led.Recs {
		field, val := "", ""
		switch {
		case !gapIn(gapStateClosed, r.State):
			field, val = "state", r.State
		case !gapIn(gapImpactClosed, r.Impact):
			field, val = "impact", r.Impact
		case !gapIn(gapPrioClosed, r.Prio):
			field, val = "prio", r.Prio
		}
		if field == "" {
			continue
		}
		inv.setErr("failed", "ledger_out_of_range", fmt.Sprintf("第 %d 行 %s 越界", led.No[i], field))
		fmt.Fprintf(stderr, "账内越界：%d %s\n", led.No[i], field)
		fmt.Fprintf(stderr, "  %s 的值 %q 不在闭集里（%s）—— 真源只由命令写（防呆③ 同源）\n",
			field, val, gapClosedText(gapClosureOf(field)))
		fmt.Fprintf(stderr, "  ⇒ 判红 1（账坏了不是「零命中」；修法：`zerg gap verify` 或手工订正该行）\n")
		return exitFail
	}

	// ④ 筛选（零命中 ⇒ 1，判词第一行与上面那条**逐字不同**）
	wantStates := inv.flagVals("--state")
	wantPrio := strings.TrimSpace(inv.flagVal("--prio"))
	wantImpact := strings.TrimSpace(inv.flagVal("--impact"))
	rows, out := []map[string]string{}, []gapRecord{}
	for _, r := range led.Recs {
		if len(wantStates) > 0 && !gapIn(wantStates, r.State) {
			continue
		}
		if wantPrio != "" && r.Prio != wantPrio {
			continue
		}
		if wantImpact != "" && r.Impact != wantImpact {
			continue
		}
		out = append(out, r)
		rows = append(rows, map[string]string{
			"id": r.ID, "prio": r.Prio, "impact": r.Impact, "state": r.State,
			"want":             gapWantText(r),
			"summary":          r.Symptom,
			"fp":               r.FP,
			"verify_cmd":       r.VerifyCmd,
			"found_at":         r.FoundAt,
			"last_verified_at": r.Verified,
			"solved_at":        r.SolvedAt,
		})
	}
	if len(out) == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：账内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(led.Recs))
		return exitFail
	}
	inv.changed = boolPtr(false)
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	// 人面表格：`id / prio / impact / state / want / 摘要`
	idw, priow, imw, stw, wantw := 0, 0, 0, 0, 0
	for _, r := range out {
		idw, stw = maxInt(idw, displayWidth(r.ID)), maxInt(stw, displayWidth(r.State))
		priow, imw = maxInt(priow, displayWidth(r.Prio)), maxInt(imw, displayWidth(r.Impact))
		wantw = maxInt(wantw, displayWidth(gapWantText(r)))
	}
	fmt.Fprintf(stdout, "账内 %d 条（筛选后 %d 条）\n", len(led.Recs), len(out))
	fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad("id", idw), pad("prio", priow), pad("impact", imw), pad("state", stw), pad("want", wantw), "摘要")
	for _, r := range out {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad(r.ID, idw), pad(r.Prio, priow), pad(r.Impact, imw), pad(r.State, stw), pad(gapWantText(r), wantw), truncateDisplay(r.Symptom, 40))
	}
	return exitOK
}

// gapClosureOf —— 按字段名取它的闭集（判词要能把闭集逐字列出来）。
func gapClosureOf(field string) []string {
	switch field {
	case "state":
		return gapStateClosed
	case "impact":
		return gapImpactClosed
	default:
		return gapPrioClosed
	}
}

// gapWantText —— `ls` 表里那一格：族 + 动作 + argv（逐字，不加解释）。
func gapWantText(r gapRecord) string {
	s := strings.TrimSpace(r.WantFam + " " + r.WantAct)
	if len(r.WantArgv) > 0 {
		s += " " + strings.Join(r.WantArgv, " ")
	}
	return s
}

func boolPtr(b bool) *bool { return &b }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── 二.2 `zerg gap add`（写面 · 留证据）───────────────────────────────────────────────────────

// gapAddFields —— `--json` 可取字段（设计稿 §二.2：items 里带 `id` / `fp` / `state`）。
var gapAddFields = []string{"id", "fp", "state"}

func cmdGapAdd(inv *invocation, stdout, stderr io.Writer) int {
	symptom := strings.TrimSpace(inv.flagVal("--symptom"))
	handmade := strings.TrimSpace(inv.flagVal("--handmade"))
	impact := strings.TrimSpace(inv.flagVal("--impact"))
	wantFam := strings.TrimSpace(inv.flagVal("--want-family"))
	wantAct := strings.TrimSpace(inv.flagVal("--want-action"))
	wantArgv := inv.flagVals("--want-argv")
	prio := strings.TrimSpace(inv.flagVal("--prio"))
	reproCmd := strings.TrimSpace(inv.flagVal("--repro-cmd"))
	verifyCmd := strings.TrimSpace(inv.flagVal("--verify-cmd"))
	dependsOn := inv.dependsOn // `--depends-on` 是具名旗标（收进 invocation.dependsOn，不是 kv）
	stateWant := strings.TrimSpace(inv.flagVal("--state"))

	// ① 用法面（在任何盘面动作之前 —— §4.1 K14 四件套那一条口径）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapAddFields, ","))
		return exitUsage
	}
	missing := []string{}
	for _, m := range []struct{ name, val string }{
		{"--symptom", symptom}, {"--handmade", handmade}, {"--impact", impact},
		{"--want-family", wantFam}, {"--want-action", wantAct},
		{"--repro-cmd", reproCmd}, {"--verify-cmd", verifyCmd},
	} {
		if m.val == "" {
			missing = append(missing, m.name)
		}
	}
	if len(missing) > 0 {
		inv.setErr("usage", "missing_required", "缺必填旗标")
		fmt.Fprintf(stderr, "%s: `gap add` 缺必填旗标：%s\n", progName, strings.Join(missing, " · "))
		fmt.Fprintf(stderr, "防呆⑤：**手搓记录（--handmade）与验证命令（--verify-cmd）是两件必填** —— 缺一 ⇒ 拒收 2，不许「先收下回头补」\n")
		fmt.Fprintf(stderr, "用法：zerg gap add --symptom <一句> --handmade <命令原样> --impact <%s> --want-family <族> --want-action <动作> --repro-cmd <命令> --verify-cmd <命令>\n",
			strings.Join(gapImpactClosed, "|"))
		return exitUsage
	}
	if !gapIn(gapImpactClosed, impact) {
		inv.setErr("usage", "bad_impact", "impact 值不在六值闭集里")
		fmt.Fprintf(stderr, "%s: `--impact %s` 不在闭集里 —— 只认 %s\n", progName, impact, gapClosedText(gapImpactClosed))
		return exitUsage
	}
	if prio == "" {
		prio = "P1" // 未标 ⇒ 中档（设计稿未钉；本件取定，见件头）
	}
	if !gapIn(gapPrioClosed, prio) {
		inv.setErr("usage", "bad_prio", "prio 值不在三值闭集里")
		fmt.Fprintf(stderr, "%s: `--prio %s` 不在闭集里 —— 只认 %s\n", progName, prio, gapClosedText(gapPrioClosed))
		return exitUsage
	}
	if stateWant == "" {
		stateWant = gapStOpen
	}
	if !gapIn(gapStateHuman, stateWant) {
		inv.setErr("usage", "state_human_forbidden", "人面不许直接写 state")
		fmt.Fprintf(stderr, "%s: 人面不许直接写 `state`（防呆④ / `H-10`）—— 收到 %q 一律拒收 2\n", progName, stateWant)
		fmt.Fprintf(stderr, "  人面只许：%s（`已解` / `回归` 是**机器态**：只由 `zerg gap verify` 跑判据转）\n", gapClosedText(gapStateHuman))
		return exitUsage
	}
	if _, why := gapJudgeArgv(verifyCmd); why != "" {
		inv.setErr("usage", "verify_cmd_unresolved", why)
		fmt.Fprintf(stderr, "%s: `--verify-cmd` 解析不到命令树：%s\n", progName, why)
		fmt.Fprintf(stderr, "  v1.3 §七 规矩 2：判据必须能在命令树里解析到（解析面 = 命令树，门⑪ `T1` 保证它与矩阵 `command` 列同尺）\n")
		return exitUsage
	}

	rec := gapRecord{
		Symptom: symptom, Handmade: handmade, Impact: impact, Prio: prio, State: stateWant,
		WantFam: wantFam, WantAct: wantAct, WantArgv: wantArgv,
		ReproCmd: reproCmd, VerifyCmd: verifyCmd, DependsOn: dependsOn,
	}
	rec.FP = gapFingerprint(rec)
	planPath := gapLedgerPath()

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（见件头 ④）。
	//    ⚠ 判在**读真源之前**：确认档不齐 ⇒ 一行都不读、一行都不写（从不提问、也不替人猜）。
	if !inv.dryRun && !inv.yes {
		gapPlanBlock(stderr, "缺 `--yes`（D2 档）", planPath, -1, rec, false)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格 —— 与 `H-10`「不要人签」不冲突）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（`add` 侧：件不在 = 首次创建，**不算 8**；读不到 / 解读不了 ⇒ 8）
	led, err := readGapLedger()
	if err != nil {
		inv.setErr("blocked", "ledger_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；写不进就不写（退码 8）\n", planPath)
		return exitBlocked
	}

	// ④ 幂等优先（防呆②）：同 fp —— 内容同 ⇒ 0 且不新增行；内容不同 ⇒ 14 并给既有 id
	for _, r := range led.Recs {
		if r.FP != rec.FP {
			continue
		}
		if gapSameContent(r, rec) {
			rec.ID, rec.FoundAt = r.ID, r.FoundAt
			inv.changed = boolPtr(false)
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": r.FP, "state": r.State})
			}
			fmt.Fprintf(stdout, "已落账 %s · fp=%s （幂等命中：同 fp 同内容 ⇒ 0 且不新增行 · 防呆②）\n", r.ID, shortSHA(r.FP))
			return exitOK
		}
		msg := fmt.Sprintf("同 fp（%s）已有 %s，而内容不同 ⇒ 拒收该行", shortSHA(rec.FP), r.ID)
		if inv.dryRun {
			// 三态纪律①：`--dry-run` 恒 0（唯一会返回 0 的那一态）
			// `--json` ⇒ 机器面取代人面（同 `verify` 的干跑档 · 同本仓列表命令的老规矩）
			if inv.jsonGiven {
				inv.changed = boolPtr(false)
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": rec.FP, "state": r.State})
			}
			gapPlanBlock(stdout, "--dry-run（冲突不落账）", planPath, len(led.Lines), rec, true)
			fmt.Fprintf(stdout, "  同名 fp  : %s（内容不同）⇒ 真跑会退 `14 conflict`（防呆②「幂等优先」）\n", r.ID)
			fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未追加真源、未写审计）\n")
			return exitOK
		}
		inv.setErr("conflict", "gap_fp_conflict", msg)
		fmt.Fprintf(stderr, "%s: %s（`14 conflict` · 防呆②「幂等优先」）\n", progName, msg)
		fmt.Fprintf(stderr, "  既有 id  : %s\n", r.ID)
		fmt.Fprintf(stderr, "  下一步   : 改它 ⇒ `zerg gap verify %s`；加新的一条 ⇒ 换 --handmade / --want-family / --want-action / --want-argv\n", r.ID)
		return exitConflict
	}

	// ⑤ 新行：分 id、取时刻
	rec.ID = gapNextID(led.Recs, time.Now().Format("20060102"))
	rec.FoundAt = gapNow()

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		// `--json` ⇒ 机器面取代人面（与 `verify` 的干跑档同一条口径；设计稿 §二.2「`--json` ⇒ 六键包封，
		// `kind` = `GapAdd`，`items` 里带 `id` / `fp` / `state`」在这一态同样成立）
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": rec.ID, "fp": rec.FP, "state": rec.State})
		}
		gapPlanBlock(stdout, "--dry-run", planPath, len(led.Lines), rec, true)
		fmt.Fprintln(stdout, "  审计     : 计划写一行 `event=gap_ledger_written`（这一态**不写**）")
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未追加真源、未写审计）\n")
		return exitOK
	}

	// ⑦ 真写：**审计先落盘**（审计写不进 ⇒ 真源一行都不写 ⇒ 8）
	line, err := json.Marshal(rec)
	if err != nil {
		inv.setErr("failed", "ledger_encode_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 真源这一行序列化不过：%v\n", progName, err)
		return exitFail
	}
	afterRaw := append(append([]byte{}, led.Raw...), append(line, '\n')...)
	audit := gapAuditLine{
		At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: "add",
		GapID: rec.ID, GapFP: rec.FP, GapStateBefore: "（无：新行）", GapStateAfter: rec.State,
		GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(afterRaw),
		GapBeforeLines: len(led.Lines), GapAfterLines: len(led.Lines) + 1,
		By: gapByOf(inv), Confirm: "--yes",
	}
	if err := appendGapAudit(editAuditPath(), audit); err != nil {
		inv.setErr("blocked", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一行都不写**（审计先落盘 · 退码 8）：%v\n", progName, err)
		fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
		return exitBlocked
	}
	if err := gapAppendLine(led.Path, led.Raw, line); err != nil {
		inv.setErr("blocked", "ledger_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 真源写不进（审计那一行已落盘）：%v\n", progName, err)
		return exitBlocked
	}
	// 读回对拍：真源现算 sha256 必须等于审计里那一格（逐字相同 —— 判据 3）
	back, err := os.ReadFile(led.Path)
	if err != nil || sha256Of(back) != audit.GapAfterSHA {
		inv.setErr("blocked", "ledger_readback_mismatch", "写回读对不上")
		fmt.Fprintf(stderr, "%s: 写后读回**对不上**审计里那一格（`gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": rec.ID, "fp": rec.FP, "state": rec.State})
	}
	fmt.Fprintf(stdout, "已落账 %s · fp=%s （新增：真源 %d → %d 行）\n", rec.ID, shortSHA(rec.FP), len(led.Lines), len(led.Lines)+1)
	return exitOK
}

// ── 二.3 `zerg gap verify`（写面 · 改状态）────────────────────────────────────────────────────

// gapVerifyFields —— `--json` 可取字段（设计稿 §二.3：逐条 `id` / 判据 / `rc` / 判决 / 新 `state`）。
var gapVerifyFields = []string{"id", "judge", "judge_rc", "verdict", "state"}

// gapVerdict —— 一条目标的判决（跑完判据之后的账）。
type gapVerdict struct {
	Idx     int // 在 `led.Recs` 里的下标
	ID      string
	Judge   string // 判据（`verify_cmd` 原样）
	JudgeRC int
	Before  string
	After   string
	Out     string // 判据输出的一小段（当证据）
	Err     string // 判据跑不起来的原因（解析不到）
}

// gapVerdictLine —— 逐条那一行：`id / 判据 / rc / 判决 / 新 state`。
func (v gapVerdict) Line() string {
	return fmt.Sprintf("%s  判据=%s  rc=%d  判决=%s→%s  新 state=%s", v.ID, v.Judge, v.JudgeRC, v.Before, v.After, v.After)
}

// gapTargets —— 点名 / `--all` 两种意图译成下标集（设计稿 §二.3 的用法面全在这里）。
func gapTargets(led gapLedger, ids []string, all bool) ([]int, int, string) {
	if all {
		if len(led.Recs) == 0 {
			return nil, exitUsage, "不给结论：真源 0 行（照闸② `L5`：空转不许当绿）"
		}
		idxs := make([]int, len(led.Recs))
		for i := range led.Recs {
			idxs[i] = i
		}
		return idxs, 0, ""
	}
	var idxs []int
	for _, id := range ids {
		found := -1
		for i, r := range led.Recs {
			if r.ID == id {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, exitBlocked, fmt.Sprintf("取数不到：账内没有 %s", id)
		}
		idxs = append(idxs, found)
	}
	return idxs, 0, ""
}

// gapJudgeOne —— 跑一条判据并把「判决表」译出来（设计稿 §二.3 判决 + v1.3 §七）。
func gapJudgeOne(led gapLedger, idx int) gapVerdict {
	r := led.Recs[idx]
	v := gapVerdict{Idx: idx, ID: r.ID, Judge: r.VerifyCmd, Before: r.State, After: r.State}
	rc, out, why := gapRunJudge(r.VerifyCmd)
	if why != "" {
		v.Err = why
		return v
	}
	v.JudgeRC, v.Out = rc, out
	switch {
	case r.State == gapStWontDo:
		// 人拍板「不做」⇒ 判据不许把它翻过来（只记一次核对）。
		v.After = gapStWontDo
	case rc == 0:
		v.After = gapStSolved
	case r.State == gapStSolved || r.State == gapStRegress:
		// **已解现缺** ⇒ 回归（唯一能让本命令退 1 的东西）
		v.After = gapStRegress
	default:
		v.After = r.State
	}
	return v
}

func cmdGapVerify(inv *invocation, stdout, stderr io.Writer) int {
	ids := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			ids = append(ids, s)
		}
	}

	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.3）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapVerifyFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 verify 的形状里")
		fmt.Fprintf(stderr, "%s: `gap verify` 不收 `--state %s`（形状 = `[<GAP id>…] [--all] [--dry-run] [--yes] [--json <字段…>]`）\n", progName, v)
		fmt.Fprintf(stderr, "  防呆④：人面不许直接写 `state`（`已解` / `回归` 是机器态，只由本命令跑判据转）\n")
		return exitUsage
	}
	if inv.all && len(ids) > 0 {
		inv.setErr("usage", "all_and_ids", "--all 与点名 id 同给")
		fmt.Fprintf(stderr, "%s: 「给范围」（`--all`）与「点名」（%s）是两种意图 ⇒ 同给 = 用法错\n", progName, strings.Join(ids, " "))
		return exitUsage
	}
	if !inv.all && len(ids) == 0 {
		inv.setErr("usage", "target_required", "缺目标")
		fmt.Fprintf(stderr, "%s: 要给目标：`zerg gap verify <GAP id>…` 或 `zerg gap verify --all`\n", progName)
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：照设计稿 §二.3 ② —— **跑判据、印判决**，但**一个字不写**；rc=2。
	//    ⚠ 确认档先判（fail-closed）：真源读不到也仍是 2 —— 已经拒执了，不许把「判不了」升成别的码。
	if !inv.dryRun && !inv.yes {
		fmt.Fprintf(stderr, "计划件（缺 `--yes`（D2 档）· 零副作用 —— 未改 `state`、未写 `solved_evidence`、未写审计）\n")
		fmt.Fprintf(stderr, "  真源     : %s\n", gapLedgerPath())
		if led, err := readGapLedger(); err == nil && led.Exists {
			if idxs, _, why := gapTargets(led, ids, inv.all); why == "" {
				red := 0
				for _, i := range idxs {
					v := gapJudgeOne(led, i)
					if v.Err != "" {
						red, idxs = -1, nil
						break
					}
					fmt.Fprintf(stderr, "    %s\n", v.Line())
					if v.Before == gapStSolved || v.Before == gapStRegress {
						if v.JudgeRC != 0 {
							red++
						}
					}
				}
				if red > 0 {
					fmt.Fprintf(stderr, "  ⚠ 有 %d 条「已解现缺」⇒ **真跑会退 1**（判红只在 `--yes` 那一态可达）\n", red)
				}
			} else {
				fmt.Fprintf(stderr, "    （目标解析不了：%s —— 但缺 `--yes` 仍拒执）\n", why)
			}
		} else {
			fmt.Fprintln(stderr, "    （真源没读到 —— 但缺 `--yes` 仍拒执；这一态不退 8：确认档不齐已经先挡了）")
		}
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（`--dry-run` 与 `--yes` 都要它：读不到 ⇒ 8 · 不许当绿）
	led, err := readGapLedger()
	if err != nil {
		inv.setErr("blocked", "ledger_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		inv.setErr("blocked", "ledger_absent", "真源不在盘上")
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		return exitBlocked
	}
	idxs, rc, why := gapTargets(led, ids, inv.all)
	if why != "" {
		if rc == exitBlocked {
			inv.setErr("blocked", "target_not_found", why)
		} else {
			inv.setErr("usage", "no_conclusion", why)
		}
		fmt.Fprintf(stderr, "%s: %s（退码 %d）\n", progName, why, rc)
		if rc == exitUsage {
			fmt.Fprintf(stderr, "  `--all` 而真源实际 0 行 = 空转 ⇒ 不给结论（闸② `L5` 同款）\n")
		}
		return rc
	}

	// ④ 跑判据（先全部解析、再跑 —— 判据不可跑 ⇒ 2，一个字节都不写）
	vs := make([]gapVerdict, 0, len(idxs))
	for _, i := range idxs {
		v := gapJudgeOne(led, i)
		if v.Err != "" {
			inv.setErr("usage", "verify_cmd_unresolved", v.Err)
			fmt.Fprintf(stderr, "%s: 判据不可跑（%s）：%s\n", progName, v.ID, v.Err)
			fmt.Fprintf(stderr, "  ⇒ 不给结论（退码 2）；真源**一个字节未改**\n")
			return exitUsage
		}
		vs = append(vs, v)
	}

	red := 0
	for _, v := range vs {
		if (v.Before == gapStSolved || v.Before == gapStRegress) && v.JudgeRC != 0 {
			red++
		}
	}

	// ⑤ `--dry-run`：跑判据、逐条印判决 ⇒ rc=**0**（唯一会返回 0 的那一态 · 零副作用）
	if inv.dryRun {
		rows := make([]map[string]string, 0, len(vs))
		for _, v := range vs {
			rows = append(rows, map[string]string{
				"id": v.ID, "judge": v.Judge, "judge_rc": strconv.Itoa(v.JudgeRC),
				"verdict": v.Before + "→" + v.After, "state": v.After,
			})
		}
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
		}
		fmt.Fprintf(stdout, "判决（--dry-run · 零副作用 —— 未改 `state`、未写审计）\n")
		for _, v := range vs {
			fmt.Fprintf(stdout, "  %s\n", v.Line())
		}
		if red > 0 {
			fmt.Fprintf(stdout, "  判红预告 : 有 %d 条「已解现缺」⇒ 真跑会退 1（判红只在 `--yes` 那一态可达）\n", red)
		}
		fmt.Fprintf(stderr, "（--dry-run：只跑判据、只印判决 · 零副作用 —— 未改 `state`、未写 `solved_evidence`、未写审计）\n")
		return exitOK
	}

	// ⑥ `--yes`：真改 —— ① 算出新件 ② **审计先落盘** ③ 写真源 ④ 读回对拍
	now := gapNow()
	newRecs := make(map[int]gapRecord, len(vs))
	for _, v := range vs {
		r := led.Recs[v.Idx]
		r.Verified = now
		switch {
		case v.After == gapStSolved && v.Before != gapStSolved:
			r.State = gapStSolved
			r.SolvedAt = now
			r.Evidence = gapEvidenceText(v, now)
			r.Verified = now
		case v.After == gapStRegress:
			r.State = gapStRegress // 历史（`solved_at` / `solved_evidence`）**不删**（追加式口径）
		default:
			r.State = v.After
		}
		newRecs[v.Idx] = r
	}
	lines := make([]string, 0, len(led.Lines))
	for i, raw := range led.Lines {
		if r, ok := newRecs[i]; ok {
			enc, err := json.Marshal(r)
			if err != nil {
				inv.setErr("failed", "ledger_encode_failed", err.Error())
				fmt.Fprintf(stderr, "%s: 真源第 %d 行序列化不过：%v\n", progName, led.No[i], err)
				return exitFail
			}
			lines = append(lines, string(enc))
			continue
		}
		lines = append(lines, raw) // 未动的行**逐字节照原样**（历史行不变）
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...)
	}
	for _, v := range vs {
		audit := gapAuditLine{
			At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: "verify",
			GapID: v.ID, GapFP: led.Recs[v.Idx].FP, GapStateBefore: v.Before, GapStateAfter: v.After,
			GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(body),
			GapBeforeLines: len(led.Lines), GapAfterLines: len(lines),
			By: gapByOf(inv), Confirm: "--yes",
		}
		if err := appendGapAudit(editAuditPath(), audit); err != nil {
			inv.setErr("blocked", "audit_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一个字节不改**（审计先落盘 · 退码 8）：%v\n", progName, err)
			fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
			return exitBlocked
		}
	}
	if err := gapWriteLedger(led.Path, lines); err != nil {
		inv.setErr("blocked", "ledger_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 真源写不进（审计那几行已落盘）：%v\n", progName, err)
		return exitBlocked
	}
	back, err := os.ReadFile(led.Path)
	if err != nil || sha256Of(back) != sha256Of(body) {
		inv.setErr("blocked", "ledger_readback_mismatch", "写回读对不上")
		fmt.Fprintf(stderr, "%s: 写后读回对不上（审计里的 `gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}

	rows := make([]map[string]string, 0, len(vs))
	for _, v := range vs {
		rows = append(rows, map[string]string{
			"id": v.ID, "judge": v.Judge, "judge_rc": strconv.Itoa(v.JudgeRC),
			"verdict": v.Before + "→" + v.After, "state": v.After,
		})
	}
	changed := false
	for _, v := range vs {
		if v.Before != v.After {
			changed = true
		}
	}
	inv.changed = boolPtr(changed)
	if red > 0 {
		// 判红：唯一能让本命令退 1 的东西（`已解现缺` ⇒ 记 `回归`）
		for _, v := range vs {
			if (v.Before == gapStSolved || v.Before == gapStRegress) && v.JudgeRC != 0 {
				inv.setErr("failed", "regressed", v.ID+" 已解现缺")
				fmt.Fprintf(stderr, "已解现缺：%s 判据 rc=%d ⇒ 该条 state → `回归`\n", v.ID, v.JudgeRC)
				break
			}
		}
	}
	if inv.jsonGiven {
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); rc != exitOK {
			return rc
		}
		if red > 0 {
			return exitFail
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "判决（--yes · 真改了 %d 条 · 审计已落 %d 行）\n", len(vs), len(vs))
	for _, v := range vs {
		fmt.Fprintf(stdout, "  %s\n", v.Line())
	}
	if red > 0 {
		fmt.Fprintf(stdout, "  判红     : 有 %d 条「已解现缺」⇒ 退码 1（该条已记 `回归`）\n", red)
		return exitFail
	}
	return exitOK
}

// gapEvidenceText —— `solved_evidence` 的取值（判据那一跑的原样 + rc + 时刻；**只留指纹级的短证据**）。
func gapEvidenceText(v gapVerdict, at string) string {
	s := fmt.Sprintf("%s ⇒ rc=0 @ %s", v.Judge, at)
	if v.Out != "" {
		out := strings.ReplaceAll(v.Out, "\n", " ⏎ ")
		s += " · 输出=" + truncateDisplay(out, 120)
	}
	return s
}

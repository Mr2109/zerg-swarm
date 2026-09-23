// family_receipt.go —— 交接回执（§20.3 `H3`「回执带 `trace_id`」）与「上次做到哪」的入口
// （§十二 `P-119` 落点 · `P-120` 定案 ①：挂 A 族 `zerg context ls`）。
//
// 判据（开工单 T-61 逐字）：
//
//	① 每轮收尾写**一页回执**：本轮 id · 做了什么 · 证据（命令 / commit）· **下一步** · 阻碍，
//	   且**带 `trace_id`**；**收尾不写回执 ⇒ 下一轮的入口必须报「无回执」并退码 `2`**。
//	⑤ 「上次做到哪」的入口名**定案** = `zerg context ls --resume`（`P-120` 定案 ①，**不新立族**）。
//
// 三条纪律：
//
//	· 回执是**轮级**续做件，不是版本级史书（`P-119` 的取舍逐字）⇒ 一轮一页、只加不改；
//	· `trace_id` **不与** `idempotency_key` 混用（§18.4 接缝第 9 行逐字）；
//	· `--resume` 是**读**面：没有回执就明说「无回执」并退 `2` —— **不假装「上一轮什么都没做」**。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// receiptFields —— `--json` 面的全部字段（K1：机器面先定）。
var receiptFields = []string{"round_id", "trace_id", "what", "next", "blockers",
	"evidence", "commits", "created_at", "path"}

// receiptRecord —— 一页回执（字段名一律取真源里的词，不自造近义词）。
type receiptRecord struct {
	RoundID   string   `json:"round_id"`
	TraceID   string   `json:"trace_id"`
	What      string   `json:"what"`
	Next      string   `json:"next"`
	Blockers  []string `json:"blockers"`
	Evidence  []string `json:"evidence"`
	Commits   []string `json:"commits"`
	CreatedAt string   `json:"created_at"`
	Path      string   `json:"path,omitempty"`
}

func receiptDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_RECEIPT_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		return filepath.Join(d, "receipts")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "state", "receipts")
}

// cmdDevReceipt —— `zerg dev receipt <动作>`（动作 = new / ls / show）。
func cmdDevReceipt(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.args) > 0 {
		action = inv.args[0]
	}
	switch action {
	case "new":
		return cmdReceiptNew(inv, stdout, stderr)
	case "ls":
		return cmdReceiptLs(inv, stdout, stderr)
	case "show":
		return cmdReceiptShow(inv, stdout, stderr)
	case "":
		inv.setErr("usage", "missing_action", "缺动作")
		fmt.Fprintf(stderr, "%s: `dev receipt` 要给动作：new | ls | show\n", progName)
		fmt.Fprintf(stderr, "用法：zerg dev receipt new --what <做了什么> [--next <下一步>] [--evidence <证据>]… "+
			"[--blockers <阻碍>]… [--commit <sha>]… [--trace <trace_id>]\n")
		fmt.Fprintf(stderr, "       zerg dev receipt ls | show <轮次 id>\n")
		return exitUsage
	default:
		inv.setErr("usage", "unknown_action", "未知动作")
		fmt.Fprintf(stderr, "%s: 未知 `dev receipt` 动作 %q（可用：new · ls · show）\n", progName, action)
		return exitUsage
	}
}

func cmdReceiptNew(inv *invocation, stdout, stderr io.Writer) int {
	spec, err := contract.Receipt()
	if err != nil {
		inv.setErr("failed", "receipt_spec_broken", err.Error())
		fmt.Fprintf(stderr, "%s: 回执真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitUsage
	}
	rec := receiptRecord{
		What:     strings.TrimSpace(inv.flagVal("--what")),
		Next:     strings.TrimSpace(inv.flagVal("--next")),
		TraceID:  strings.TrimSpace(inv.flagVal("--trace")),
		Evidence: trimAll(inv.flagVals("--evidence")),
		Blockers: trimAll(inv.flagVals("--blockers")),
		Commits:  trimAll(inv.flagVals("--commit")),
	}
	// 必填逐条点名（§九 M7：错误要给下一步）
	missing := []string{}
	if rec.What == "" {
		missing = append(missing, "--what")
	}
	if rec.Next == "" {
		missing = append(missing, "--next")
	}
	if len(missing) > 0 {
		inv.setErr("usage", "missing_flag", "缺必需旗标："+strings.Join(missing, " · "))
		fmt.Fprintf(stderr, "%s: 缺必需旗标：%s（回执五格：做了什么 · 证据 · 下一步 · 阻碍 ⇒ 缺一格就不算一页回执）\n",
			progName, strings.Join(missing, " · "))
		return exitUsage
	}
	// `trace_id`：生成侧的真源就在这里（`OM4` 的「谁生成」那一格）。
	if rec.TraceID == "" {
		rec.TraceID = newTraceID()
	} else if !strings.HasPrefix(rec.TraceID, "TR-") {
		inv.setErr("usage", "bad_trace_id", "trace_id 形态不对（要 `TR-` 前缀）")
		fmt.Fprintf(stderr, "%s: `trace_id` 形态不对（给的是 %q）—— 真源口径：`TR-` + 12 位十六进制（%s）\n",
			progName, rec.TraceID, spec.TraceIDRule)
		return exitUsage
	}
	rec.RoundID = "R-" + time.Now().Format("20060102T150405")
	rec.CreatedAt = time.Now().Format(time.RFC3339)

	dir := receiptDir()
	if dir == "" {
		inv.setErr("failed", "no_receipt_dir", "回执落点解析不出来")
		fmt.Fprintf(stderr, "%s: 回执落点解析不出来 ⇒ 不给结论\n", progName)
		return exitFail
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		inv.setErr("failed", "receipt_dir_create_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回执目录建不出来：%v\n", progName, err)
		return exitFail
	}
	rec.Path = filepath.Join(dir, rec.RoundID+".json")
	b, _ := json.MarshalIndent(rec, "", " ")
	if err := os.WriteFile(rec.Path, append(b, '\n'), 0o644); err != nil {
		inv.setErr("failed", "receipt_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回执写不进去：%v\n", progName, err)
		return exitFail
	}
	row := receiptRow(rec)
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, receiptFields, row)
	}
	fmt.Fprintf(stdout, "%s\n", rec.RoundID)
	fmt.Fprintf(stdout, "trace_id: %s\n", rec.TraceID)
	fmt.Fprintf(stdout, "回执落点: %s\n", rec.Path)
	fmt.Fprintf(stdout, "下一步的读法：zerg context ls --resume\n")
	return exitOK
}

func cmdReceiptLs(inv *invocation, stdout, stderr io.Writer) int {
	recs, err := loadReceipts()
	if err != nil {
		inv.setErr("failed", "receipt_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回执目录读不出来：%v\n", progName, err)
		return exitFail
	}
	rows := []map[string]string{}
	for _, r := range recs {
		rows = append(rows, receiptRow(r))
	}
	return listCmd(inv, stdout, stderr, []string{"round_id", "trace_id", "what", "next"}, rows)
}

func cmdReceiptShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 {
		inv.setErr("usage", "missing_round", "缺轮次 id")
		fmt.Fprintf(stderr, "%s: 用法：zerg dev receipt show <轮次 id>\n", progName)
		return exitUsage
	}
	want := strings.TrimSpace(inv.args[1])
	recs, err := loadReceipts()
	if err != nil {
		inv.setErr("failed", "receipt_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回执目录读不出来：%v\n", progName, err)
		return exitFail
	}
	for _, r := range recs {
		if r.RoundID == want {
			row := receiptRow(r)
			if inv.jsonGiven {
				if rc := requireFields(inv, stderr); rc != exitOK {
					return rc
				}
				return selectJSON(stdout, stderr, inv, inv.path, receiptFields, row)
			}
			fmt.Fprintf(stdout, "轮次      : %s\n", r.RoundID)
			fmt.Fprintf(stdout, "trace_id  : %s\n", r.TraceID)
			fmt.Fprintf(stdout, "做了什么  : %s\n", r.What)
			fmt.Fprintf(stdout, "证据      : %s\n", orDash(strings.Join(r.Evidence, " · ")))
			fmt.Fprintf(stdout, "下一步    : %s\n", r.Next)
			fmt.Fprintf(stdout, "阻碍      : %s\n", orDash(strings.Join(r.Blockers, " · ")))
			return exitOK
		}
	}
	inv.setErr("failed", "receipt_not_found", "没有这一轮的回执："+want)
	fmt.Fprintf(stderr, "%s: 没有轮次 %q 的回执\n", progName, want)
	return exitFail
}

// cmdContextResume —— `zerg context ls --resume`：**「上次做到哪」的唯一入口**（`P-120` 定案 ①）。
//
// 判据①的后半：**收尾不写回执 ⇒ 下一轮的入口必须报「无回执」并退码 `2`** ——
// 这里**不猜、不退回「什么都没发生」**，明说没有回执。
func cmdContextResume(inv *invocation, stdout, stderr io.Writer) int {
	spec, err := contract.Receipt()
	if err != nil {
		inv.setErr("failed", "receipt_spec_broken", err.Error())
		fmt.Fprintf(stderr, "%s: 回执真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitUsage
	}
	recs, rerr := loadReceipts()
	if rerr != nil {
		inv.setErr("failed", "receipt_read_failed", rerr.Error())
		fmt.Fprintf(stderr, "%s: 回执目录读不出来：%v ⇒ 不给结论\n", progName, rerr)
		return exitFail
	}
	if len(recs) == 0 {
		inv.setErr("usage", "no_receipt", "没有回执")
		fmt.Fprintf(stderr, "%s: **无回执**。\n", progName)
		fmt.Fprintf(stderr, "上一轮收尾没有写回执 ⇒ 这一轮**不给结论**（退码 2）—— 不许把「没写」当成「什么都没做」。\n")
		fmt.Fprintf(stderr, "写一页：zerg dev receipt new --what <做了什么> --next <下一步> [--evidence <证据>]…\n")
		fmt.Fprintf(stderr, "入口名定案（§十二 P-120 ①）：%s\n", spec.ResumeEntry)
		fmt.Fprintf(stderr, "error.kind=usage · detail=no_receipt · retryable=false\n")
		return exitUsage
	}
	last := recs[len(recs)-1] // loadReceipts 已按轮次 id 升序
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, receiptFields, receiptRow(last))
	}
	fmt.Fprintf(stdout, "上次做到哪（%s · 共 %d 轮有回执）\n", spec.ResumeEntry, len(recs))
	fmt.Fprintf(stdout, "  轮次      : %s\n", last.RoundID)
	fmt.Fprintf(stdout, "  trace_id  : %s\n", last.TraceID)
	fmt.Fprintf(stdout, "  做了什么  : %s\n", last.What)
	fmt.Fprintf(stdout, "  证据      : %s\n", orDash(strings.Join(last.Evidence, " · ")))
	fmt.Fprintf(stdout, "  下一步    : %s\n", last.Next)
	fmt.Fprintf(stdout, "  阻碍      : %s\n", orDash(strings.Join(last.Blockers, " · ")))
	return exitOK
}

// loadReceipts 读全部回执（按轮次 id 升序 ⇒ 「最新」= 最后一条）。
func loadReceipts() ([]receiptRecord, error) {
	dir := receiptDir()
	if dir == "" {
		return nil, fmt.Errorf("回执落点解析不出来")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 目录不存在 = 还没有任何回执（不是错误）
		}
		return nil, err
	}
	out := []receiptRecord{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var r receiptRecord
		if err := json.Unmarshal(b, &r); err != nil {
			continue
		}
		r.Path = p
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoundID < out[j].RoundID })
	return out, nil
}

func receiptRow(r receiptRecord) map[string]string {
	return map[string]string{
		"round_id": r.RoundID, "trace_id": r.TraceID, "what": r.What, "next": r.Next,
		"blockers": strings.Join(r.Blockers, ","), "evidence": strings.Join(r.Evidence, " · "),
		"commits": strings.Join(r.Commits, ","), "created_at": r.CreatedAt, "path": r.Path,
	}
}

// newTraceID 造一个 `trace_id`（`TR-` + 12 位十六进制）—— 生成侧的真源。
func newTraceID() string {
	now := time.Now()
	sum := int64(now.UnixNano())
	const hexdig = "0123456789abcdef"
	b := make([]byte, 12)
	for i := 0; i < 12; i++ {
		b[i] = hexdig[(sum>>uint(4*(11-i)))&0xf]
	}
	return "TR-" + string(b)
}

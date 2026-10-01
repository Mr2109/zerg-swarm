package agent

// edit_ledger.go — GAP-20260927-80：agent 写工具改 tracked 件落「件级账」。
//
// 病灶：`write`/`edit`/`apply_patch` 三个写工具走 gate.Check 后即改盘，**不留任何件级账**
// —— `dev edit` 一族却把每次写落进 `<state>/edit_audit.jsonl`（一行一事件 · 同形键集）。
// 本件把那条件级账接到写工具上：**同一条账、同形键集**
// （`at/event/proposal/file/mode/before_sha256/after_sha256/before_bytes/after_bytes/by/confirm/audit_path`），
// 令「谁在何时把哪一件改成什么」不只在 `dev edit` 面可回读。
//
// 口径（沿用 GAP-20260927-80 的循证结论）：件级账**只做定点溯源**，不接成判据
// （拿它当判据实测假红 ~100% —— 许可写与真漏都长成「没有行」）。落账失败**不影响写**
// （写已成功，不许因记账失败而反过来判写失败）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// agentEditAuditLine — 与 `core/cmd/zerg/family_dev_edit.go` 的 `editAuditLine`
// **同形键集**（同一条账的读法一致：按 file+at 查 by/mode/before_sha256/after_sha256）。
// 写工具没有 `dev edit` 的提案/确认/批准档 ⇒ 那几格留空（proposal/confirm）。
type agentEditAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"`
	Proposal     string `json:"proposal"`
	File         string `json:"file"`
	Mode         string `json:"mode"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	BeforeBytes  int64  `json:"before_bytes"`
	AfterBytes   int    `json:"after_bytes"`
	By           string `json:"by"`
	Confirm      string `json:"confirm"`
	AuditPath    string `json:"audit_path"`
}

// agentEditAuditPath — 与 dev edit 同一落点（`ZERG_EDIT_AUDIT` > `<state>/edit_audit.jsonl`）。
func agentEditAuditPath() string {
	if p := os.Getenv("ZERG_EDIT_AUDIT"); p != "" {
		return p
	}
	return filepath.Join(statepath.Dir(), "edit_audit.jsonl")
}

// shortSHA16 — 16 位 sha256 前缀（与 dev edit 的 `shortSHA` 同口径）。
func shortSHA16(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// recordAgentEditLedger — 写工具成功改盘后追一行件级账。
// ★ 只记事件、不设判据；失败静默（写已成功）。
func recordAgentEditLedger(tool, file string, before, after []byte, by string) {
	path := agentEditAuditPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	line := agentEditAuditLine{
		At:           time.Now().Format(time.RFC3339),
		Event:        "edit",
		File:         file,
		Mode:         tool,
		BeforeSHA256: shortSHA16(before),
		AfterSHA256:  shortSHA16(after),
		BeforeBytes:  int64(len(before)),
		AfterBytes:   len(after),
		By:           by,
		Confirm:      tool,
		AuditPath:    path,
	}
	body, err := json.Marshal(line)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(body, '\n'))
}

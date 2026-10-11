// cli_approve_rotate_test.go —— `zerg approve rotate`（撤销/覆盖 · GAP-20261001-76）的面判据。
//
// 落点：`package main` —— 够得着包内**退位核心**（`retireTicket` / `recordApprovalRetireAudit`）；
// 命令面走桥 `RunForTest`（与 `main()` 同一个 `run`）。★ 夹具全在 `t.TempDir()` 的**合成状态目录**里，
// **真状态目录一个字节不碰**（硬线⑦：✗ 不真动生产批准件目录）。
//
// 正控：
//
//	① 干跑那一态可判：`--dry-run` rc=0 · 计划面（覆盖/轮换 + 退位位 + 审计事件名）逐字在 · **零副作用**；
//	② 退位核心可做：`retireTicket` 把件 **mv** 进 `retired/`（**原路径没了 · 退位件字节 == 原件字节**）；
//	③ 审计核心可做：`recordApprovalRetireAudit` 落一行 `approve_retired`（带退位件 sha256）。
//
// 负控（真红）：
//
//	④ 非法/缺格 ⇒ **按既有档拒并点名**（缺工具名/缺 --by/缺 --confirm/缺 --yes/值不逐字相同/干跑真写同给）；
//	⑤ 取不到/无终端 ⇒ **非 0 不给结论**（无 TTY 真跑 ⇒ rc=2 点名「终端」· stdout 0 字节；
//	  退位审计取不到退位件 ⇒ 报错**不写行**）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zrRun —— 跑一条口令（进程内 · 与 `main()` 同一个 `run`）。
func zrRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

func zrWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func zrRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读夹具 %s 失败：%v", path, err)
	}
	return b
}

func zrExists(p string) bool { _, err := os.Stat(p); return err == nil }

func zrSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func zrCount(t *testing.T, dir string) int {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(ents)
}

// TestApproveRotateDryRunPlan —— 正控①：干跑那一态可判 · 计划面逐字在 · 零副作用。
func TestApproveRotateDryRunPlan(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	ticket := filepath.Join(state, "approvals", "dev_edit.json")
	zrWrite(t, ticket, "{\"tool\":\"dev_edit\"}\n")
	before := zrSHA(zrRead(t, ticket))

	rc, out, errb := zrRun("approve", "rotate", "dev_edit", "--by", "Mr2109", "--scope", "core/", "--dry-run")
	if rc != 0 {
		t.Fatalf("正控① 破：干跑 rc=%d（要 0）\nstdout=\n%s\nstderr=\n%s", rc, out, errb)
	}
	for _, want := range []string{"覆盖/轮换", "退位", "core/", "approve_retired"} {
		if !strings.Contains(out, want) {
			t.Errorf("正控① 破：计划面缺 %q\nstdout=\n%s", want, out)
		}
	}
	if !strings.Contains(errb, "--dry-run") {
		t.Errorf("正控① 破：stderr 没点名 --dry-run\nstderr=\n%s", errb)
	}
	// 零副作用：旧件字节不变 · 没有退位件 · 没有审计件。
	if got := zrSHA(zrRead(t, ticket)); got != before {
		t.Errorf("正控① 破：干跑动了旧件（%s ≠ %s）", got, before)
	}
	if n := zrCount(t, filepath.Join(state, "approvals", "retired")); n != 0 {
		t.Errorf("正控① 破：干跑落了退位件 %d 个", n)
	}
	if zrExists(filepath.Join(state, "edit_audit.jsonl")) {
		t.Errorf("正控① 破：干跑落了审计件")
	}
}

// TestApproveRotateRetireCore —— 正控②③：退位（mv 不删 · 字节不丢）+ 审计落一行。
func TestApproveRotateRetireCore(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	ticket := filepath.Join(state, "approvals", "dev_edit.json")
	body := "{\"tool\":\"dev_edit\",\"scope\":\"*\"}\n"
	zrWrite(t, ticket, body)

	rp, err := retireTicket(ticket, "dev_edit")
	if err != nil {
		t.Fatalf("正控② 破：退位失败：%v", err)
	}
	if !strings.HasPrefix(rp, filepath.Join(state, "approvals", "retired")+string(os.PathSeparator)) {
		t.Errorf("正控② 破：退位件不在 retired/ 下：%s", rp)
	}
	if zrExists(ticket) {
		t.Errorf("正控② 破：退位后**原路径仍在**（不是 mv —— 应已不在）：%s", ticket)
	}
	got := zrSHA(zrRead(t, rp))
	if got != zrSHA([]byte(body)) {
		t.Errorf("正控② 破：退位件字节与原件不符（%s ≠ %s）—— 退位必须**字节不丢**", got, zrSHA([]byte(body)))
	}
	// 正控③：审计核心落一行 approve_retired（带退位件 sha256）。
	if _, err := recordApprovalRetireAudit("dev_edit", "Mr2109", rp, "revoke", ""); err != nil {
		t.Fatalf("正控③ 破：退位审计没落成：%v", err)
	}
	raw := strings.TrimSpace(string(zrRead(t, filepath.Join(state, "edit_audit.jsonl"))))
	var line struct {
		Event         string `json:"event"`
		Tool          string `json:"tool"`
		RetiredPath   string `json:"retired_path"`
		RetiredSHA256 string `json:"retired_sha256"`
		Mode          string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(raw), &line); err != nil {
		t.Fatalf("正控③ 破：审计行解不动：%v\n%s", err, raw)
	}
	if line.Event != "approve_retired" || line.Tool != "dev_edit" || line.Mode != "revoke" {
		t.Errorf("正控③ 破：审计行字段不符 = %+v", line)
	}
	if line.RetiredPath != rp || line.RetiredSHA256 != got {
		t.Errorf("正控③ 破：审计行的退位落点/sha256 不符（%s / %s）", line.RetiredPath, line.RetiredSHA256)
	}
}

// TestApproveRotateRejections —— 负控④：非法/缺格 ⇒ 按既有档拒并点名（执行前判 · stdout 0 字节 · 不落件）。
func TestApproveRotateRejections(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	cases := []struct {
		name    string
		argv    []string
		wantSub string
	}{
		{"缺工具名", []string{"approve", "rotate", "--by", "Mr2109"}, "工具名"},
		{"缺 --by", []string{"approve", "rotate", "dev_edit"}, "--by"},
		{"缺 --confirm", []string{"approve", "rotate", "dev_edit", "--by", "Mr2109"}, "--confirm"},
		{"缺 --yes", []string{"approve", "rotate", "dev_edit", "--by", "Mr2109", "--confirm=dev_edit"}, "--yes"},
		{"--confirm 值不匹配", []string{"approve", "rotate", "dev_edit", "--by", "Mr2109", "--confirm=别的", "--yes"}, "不逐字相同"},
		{"干跑与真写同给", []string{"approve", "rotate", "dev_edit", "--by", "Mr2109", "--dry-run", "--yes"}, "不许同给"},
	}
	for _, c := range cases {
		rc, out, errb := zrRun(c.argv...)
		if rc != 2 {
			t.Errorf("负控④ 破：%s rc=%d（要 2 —— 用法错 · 执行前判）\nstderr=\n%s", c.name, rc, errb)
		}
		if out != "" {
			t.Errorf("负控④ 破：%s 往 stdout 写了 %q（拒执那一态要 0 字节）", c.name, out)
		}
		if !strings.Contains(errb, c.wantSub) {
			t.Errorf("负控④ 破：%s 的拒因没点名 %q：\n%s", c.name, c.wantSub, errb)
		}
	}
	// 执行前判 ⇒ 一个件都不许落（退位件 / 审计件都没有）。
	if n := zrCount(t, filepath.Join(state, "approvals", "retired")); n != 0 {
		t.Errorf("负控④ 破：拒执那一态落了退位件 %d 个", n)
	}
	if zrExists(filepath.Join(state, "edit_audit.jsonl")) {
		t.Errorf("负控④ 破：拒执那一态落了审计件")
	}
}

// TestApproveRotateNoTTYNoConclusion —— 负控⑤：无 TTY 真跑 ⇒ 非 0 不给结论（点名人签面）。
func TestApproveRotateNoTTYNoConclusion(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	rc, out, errb := zrRun("approve", "rotate", "dev_edit", "--by", "Mr2109", "--confirm=dev_edit", "--yes")
	if rc != 2 {
		t.Errorf("负控⑤ 破：无 TTY 真跑 rc=%d（要 2 —— 人签面拦下）\nstderr=\n%s", rc, errb)
	}
	if out != "" {
		t.Errorf("负控⑤ 破：无 TTY 往 stdout 写了 %q（要 0 字节）", out)
	}
	if !strings.Contains(errb, "终端") {
		t.Errorf("负控⑤ 破：拒因没点名「终端」：\n%s", errb)
	}
	if zrExists(filepath.Join(state, "edit_audit.jsonl")) {
		t.Errorf("负控⑤ 破：无 TTY 那一路落了审计件")
	}
}

// TestApproveRotateRetireAuditMissingCellRefuses —— 负控⑤：退位件取不到 ⇒ 审计行**不写**（非 0 口径）。
func TestApproveRotateRetireAuditMissingCellRefuses(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	if _, err := recordApprovalRetireAudit("dev_edit", "Mr2109", filepath.Join(state, "nope.json"), "revoke", ""); err == nil {
		t.Errorf("负控⑤ 破：退位件取不到却没报错（缺格 ⇒ 这一行不写）")
	}
	if zrExists(filepath.Join(state, "edit_audit.jsonl")) {
		t.Errorf("负控⑤ 破：缺格却落了审计行")
	}
}

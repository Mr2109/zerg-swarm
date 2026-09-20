// chat_tool_gate_test.go —— 「对话层两个危险调用点接电」的**成对判据**（2026-09-21）。
//
// 一条判据不够：判「接上了」要三面同时成立 ——
//
//	① 成对负控：**没有**人签批准件 ⇒ 拒（且**文件还在** —— 「拒」必须是真的没执行）；
//	② 逃生门正控：人签批准件在册 ⇒ **照旧可用**（不许为了关门把工具弄成不能用的）；
//	③ 表里 allow 的正控：判定是 allow 时，工具**逐字照旧**（能力没被删）。
//
// 另加两条「废票」负控：人名空 / scope 不覆盖本次参数 ⇒ **不算批准**（读不到一律不算批准）。
package chat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// writeApproval 落一枚人签批准件到当前 ZERG_STATE_DIR。
func writeApproval(t *testing.T, tool, body string) {
	t.Helper()
	p := approvalPath(tool)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ① 成对负控：无批准件 ⇒ 拒 + 文件还在（「无检查直接执行」这条老形态必须不再复现）。
func TestDangerousToolsRefusedWithoutApproval(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := ExecuteChatTool("delete_file", map[string]any{"path": victim}, dir)
	if res.Error == "" || res.Content != "" {
		t.Fatalf("无批准件时必须拒（Error 非空 · Content 空），得到 Content=%q Error=%q", res.Content, res.Error)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("被拒的调用**不该动文件**：%v", err)
	}
	t.Logf("拒因原样：%s", res.Error)

	out := filepath.Join(dir, "dl.txt")
	res2 := ExecuteChatTool("download", map[string]any{"url": "file:///etc/hosts", "output": "dl.txt"}, dir)
	if res2.Error == "" {
		t.Fatalf("download 无批准件时必须拒，得到 %q", res2.Content)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("被拒的 download **不该落盘**")
	}
	t.Logf("download 拒因原样：%s", res2.Error)
}

// ② 逃生门正控：人签批准件在册 ⇒ 两件都照旧可用。
func TestDangerousToolsUsableWithHumanApproval(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	writeApproval(t, "delete_file", `{"approver":"Mr2109","approved_at":"2026-09-21T00:00:00Z","scope":"*","note":"判据正控"}`)
	writeApproval(t, "download", `{"approver":"Mr2109","approved_at":"2026-09-21T00:00:00Z","scope":"*","note":"判据正控"}`)

	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := ExecuteChatTool("delete_file", map[string]any{"path": victim}, dir)
	if res.Error != "" {
		t.Fatalf("批准件在册时不该被拒：%q", res.Error)
	}
	if _, err := os.Stat(victim); err == nil {
		t.Fatal("批准后应当真删掉")
	}
	t.Logf("批准后 delete_file：%s", res.Content)

	if err := os.WriteFile(victim, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	res2 := ExecuteChatTool("download", map[string]any{"url": "file:///etc/hosts", "output": "dl.txt"}, dir)
	if res2.Error != "" {
		t.Fatalf("批准件在册时 download 不该被拒：%q", res2.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "dl.txt")); err != nil {
		t.Fatalf("批准后应当真落盘：%v", err)
	}
	t.Logf("批准后 download：%s", res2.Content)
}

// ③ 表里 allow 的正控：判定 = allow 时工具逐字照旧（**不硬禁 · 不删能力**）。
func TestAllowedToolStillWorks(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	g, err := control.NewGateFromYAML([]byte("version: 1\nrules:\n  default: block\n  tools:\n    - name: \"delete_file\"\n      action: allow\n"))
	if err != nil {
		t.Fatal(err)
	}
	chatGateMu.Lock()
	chatGateInst, chatGateLoaded = control.NewAgentGate(g), true
	chatGateMu.Unlock()
	t.Cleanup(func() {
		chatGateMu.Lock()
		chatGateInst, chatGateLoaded = nil, false
		chatGateMu.Unlock()
	})
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := ExecuteChatTool("delete_file", map[string]any{"path": victim}, dir)
	if res.Error != "" {
		t.Fatalf("表里 allow ⇒ 必须照旧可用：%q", res.Error)
	}
	if _, err := os.Stat(victim); err == nil {
		t.Fatal("allow ⇒ 应当真删掉")
	}
}

// 负控：废票（人名空）与 scope 不覆盖本次参数 —— 都**不算批准**。
func TestApprovalTicketNegativeControls(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	args := `path="/tmp/x"`
	// ① 件不存在
	if ok, _ := approvalGranted("delete_file", args); ok {
		t.Fatal("件不存在不算批准")
	}
	// ② 人名空 = 废票
	writeApproval(t, "delete_file", `{"approver":"","approved_at":"2026-09-21T00:00:00Z","scope":"*"}`)
	if ok, why := approvalGranted("delete_file", args); ok {
		t.Fatalf("人名空必须是废票，得到 ok=true（%s）", why)
	}
	// ③ scope 不覆盖本次参数
	writeApproval(t, "delete_file", `{"approver":"Mr2109","scope":"/tmp/y"}`)
	if ok, why := approvalGranted("delete_file", args); ok {
		t.Fatalf("scope 不覆盖时不算批准，得到 ok=true（%s）", why)
	}
	// ④ scope 恰好覆盖 ⇒ 算批准（同一份件的正控，证明判据不是恒假）
	writeApproval(t, "delete_file", `{"approver":"Mr2109","scope":"/tmp/x"}`)
	if ok, why := approvalGranted("delete_file", args); !ok {
		t.Fatalf("scope 覆盖时应当算批准：%s", why)
	}
	// ⑤ 解析不了的件 ⇒ 不算批准
	writeApproval(t, "delete_file", `{ 这不是 json`)
	if ok, _ := approvalGranted("delete_file", args); ok {
		t.Fatal("解析不了的件不算批准")
	}
}

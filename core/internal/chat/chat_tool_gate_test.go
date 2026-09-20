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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// ★ D3b 第四步（2026-09-21）：_人签批准件_ 从此要带**签名** —— 夹具也必须**真签**（否则「正控」
// 测的是一枚手写件，而手写件早就该被拦）。这两枚 helper 模拟 `zerg approve keygen` / `approve new`
// 的产物：`operatorForTest` 落公钥件，`writeSignedApproval` 用私钥签一枚真件。
var testOperatorPriv ed25519.PrivateKey

// 同一状态目录只建一把密钥（第二次调用要**复用** —— 否则后一枚公钥会覆盖前一枚，
// 先前签的件当场变成「换了密钥就是换了签的人」；本件的第一个实现就踩了这个坑，实测抓到的）。
var testOperatorByDir = map[string]ed25519.PrivateKey{}

func operatorForTest(t *testing.T) {
	t.Helper()
	if dir := filepath.Dir(approvalPath("x")); true {
		if priv, ok := testOperatorByDir[dir]; ok {
			testOperatorPriv = priv
			return
		}
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	testOperatorPriv = priv
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	pf, _ := json.Marshal(map[string]any{
		"alg": "ed25519", "pub": pubB64, "key_id": control.KeyID(pubB64),
	})
	dst := filepath.Join(filepath.Dir(approvalPath("x")), "operator.pub")
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, pf, 0o644); err != nil {
		t.Fatal(err)
	}
	testOperatorByDir[filepath.Dir(dst)] = priv
}

// writeSignedApproval 落一枚**真签**的批准件（人签链路的正控夹具）。
func writeSignedApproval(t *testing.T, tool, approver, at, note, scope string) {
	t.Helper()
	operatorForTest(t)
	payload := control.ApprovalPayload(tool, scope, approver, at, note)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(testOperatorPriv, payload))
	body, _ := json.Marshal(map[string]any{
		"tool": tool, "approver": approver, "approved_at": at, "scope": scope, "note": note,
		"sig_alg": "ed25519", "key_id": control.KeyID(base64.StdEncoding.EncodeToString(testOperatorPriv.Public().(ed25519.PublicKey))),
		"sig": sig,
	})
	writeApproval(t, tool, string(body))
}

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
	writeSignedApproval(t, "delete_file", "Mr2109", "2026-09-21T00:00:00Z", "判据正控", "*")
	writeSignedApproval(t, "download", "Mr2109", "2026-09-21T00:00:00Z", "判据正控", "*")

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
	// ④ scope 恰好覆盖 + **真签名** ⇒ 算批准（同一份件的正控，证明判据不是恒假）
	writeSignedApproval(t, "delete_file", "Mr2109", "2026-09-21T00:00:00Z", "scope 正控", "/tmp/x")
	if ok, why := approvalGranted("delete_file", args); !ok {
		t.Fatalf("scope 覆盖时应当算批准：%s", why)
	}
	// ⑤ 解析不了的件 ⇒ 不算批准
	writeApproval(t, "delete_file", `{ 这不是 json`)
	if ok, _ := approvalGranted("delete_file", args); ok {
		t.Fatal("解析不了的件不算批准")
	}
}

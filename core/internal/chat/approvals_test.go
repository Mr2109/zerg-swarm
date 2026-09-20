// approvals_test.go —— 人批通道**消费者侧**的成对负控（D3b 第四步 · 最要紧的一对）。
//
// 这一对正是 D3b 要的两条判据：
//
//	① **无批准件 ⇒ 需审批即不执行**（老口径，必须保持）；
//	② **有批准件 ⇒ 放行**（逃生门在册 · 能力没被删）；
//
// 外加两条 D3b 新立的（否则①就等于没立）：
//
//	③ **手写件不算批准**（无签名 / 签名坏 / 公钥读不到 ⇒ 一律不算）—— 判「人签」靠**签名**，不靠字段齐；
//	④ **签名件的格被改过 ⇒ 不算批准**（签名覆盖五格：tool/scope/approver/approved_at/note）。
//
// 全部落 `t.TempDir()`（`ZERG_STATE_DIR`）⇒ 不碰真状态目录、不碰生产。
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

// writeTicket 手写一枚批准件（测试夹具：模拟「模型写得出的那件事」）。
func writeTicket(t *testing.T, dir string, tk map[string]any) {
	t.Helper()
	b, _ := json.Marshal(tk)
	if err := os.WriteFile(filepath.Join(dir, "approvals", "terminal.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalChannelPairs(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	appr := filepath.Join(state, "approvals")
	if err := os.MkdirAll(appr, 0o700); err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	// ① 无件 ⇒ 不算批准（「无审批即不执行」那一格）
	if ok, _ := approvalGranted("terminal", "cmd=ls"); ok {
		t.Error("① 没有批准件却判了放行（逃生门成了默认开口）")
	}

	// ③-a 手写件（字段齐、**无签名**）⇒ 不算批准 —— 这一格是 D3b 的核心：字段模型也写得出来
	writeTicket(t, state, map[string]any{
		"tool": "terminal", "approver": "张三", "approved_at": "2026-09-21T00:00:00+08:00",
		"scope": "*", "note": "我自己写的",
	})
	if ok, msg := approvalGranted("terminal", "cmd=ls"); ok {
		t.Error("③ 手写件（无签名）被当成「人签」放行了 —— 自签自批")
	} else if msg == "" {
		t.Error("③ 拒因是空的（拒了也要说清为什么）")
	}

	// ② 真签名件 ⇒ 放行（用**在册公钥**对应的私钥签）
	at, note := "2026-09-21T00:00:00+08:00", "测试夹具：放行一次"
	payload := control.ApprovalPayload("terminal", "*", "张三", at, note)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
	pubJSON, _ := json.Marshal(map[string]any{
		"alg": "ed25519", "pub": pubB64, "key_id": control.KeyID(pubB64),
	})
	if err := os.WriteFile(filepath.Join(appr, "operator.pub"), pubJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTicket(t, state, map[string]any{
		"tool": "terminal", "approver": "张三", "approved_at": at,
		"scope": "*", "note": note, "sig_alg": "ed25519",
		"key_id": control.KeyID(pubB64), "sig": sig,
	})
	if ok, msg := approvalGranted("terminal", "cmd=ls"); !ok {
		t.Errorf("② 签名验过的件却没放行（逃生门失效）：%s", msg)
	}

	// ④ 签名件的格被改过 ⇒ 不算批准
	writeTicket(t, state, map[string]any{
		"tool": "terminal", "approver": "张三", "approved_at": at,
		"scope": "*", "note": "改过的理由", "sig_alg": "ed25519",
		"key_id": control.KeyID(pubB64), "sig": sig,
	})
	if ok, _ := approvalGranted("terminal", "cmd=ls"); ok {
		t.Error("④ 格被改过却仍放行（签名没覆盖内容）")
	}

	// ⑤ 公钥件被删 ⇒ 不算批准（fail-closed：读不到不当过人签）
	if err := os.Remove(filepath.Join(appr, "operator.pub")); err != nil {
		t.Fatal(err)
	}
	if ok, _ := approvalGranted("terminal", "cmd=ls"); ok {
		t.Error("⑤ 公钥读不到却仍放行（读不到必须按没批准处理）")
	}
}

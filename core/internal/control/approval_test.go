// approval_test.go —— 人签批准件（D3b 第四步）的**成对负控**：待签形态唯一 + 验签只认签名。
//
// 判据（逐条给正控与负控）：
//
//	① 待签形态**唯一**：同一组五格算出的字节逐字相同（两处各自算一次 ⇒ 必须一致）；
//	② 签名能验过（正控）—— 拿在册公钥验真签出的件；
//	③ 改任一格 ⇒ 验不过（负控：件的内容被改过）；
//	④ 换成别的公钥 ⇒ 验不过（负控：手写件 / 自造密钥那条路）；
//	⑤ 空签名 / 坏 base64 / 公钥长度不对 ⇒ **一律不算批准**（fail-closed，不吞、不猜）；
//	⑥ 换行不许出现在格里（`SanitizeApprovalField`）—— 否则两格能拼成一格（签名歧义）。
package control

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestApprovalPayloadIsSingleForm(t *testing.T) {
	a := ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:00+08:00", "放行一次")
	b := ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:00+08:00", "放行一次")
	if string(a) != string(b) {
		t.Errorf("同一组格算出两串不同字节：%q vs %q", a, b)
	}
	if strings.Count(string(a), "\n") != 5 {
		t.Errorf("形态应是「版本行 + 五格」（5 个换行分隔 6 段），实测 %d 个换行：%q",
			strings.Count(string(a), "\n"), a)
	}
	// ⑥ 换行在格内被清掉 ⇒ 拼不成另一组格
	if strings.Contains(string(ApprovalPayload("a\nb", "*", "x", "t", "n")), "zerg-approval/v1\na\nb\n") {
		t.Error("格内的换行没被清掉（会造成「两格拼成一格」的签名歧义）")
	}
	if got := SanitizeApprovalField("a\nb\rc "); got != "a b c" {
		t.Errorf("SanitizeApprovalField 实测 %q（要把 \\n / \\r 换成空格再 TrimSpace）", got)
	}
}

func TestVerifyApprovalSignaturePairs(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	payload := ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:00+08:00", "放行一次")
	sigB64 := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))

	// ② 正控
	if err := VerifyApprovalSignature(pubB64, sigB64, payload); err != nil {
		t.Errorf("正控失败（真签的件验不过）：%v", err)
	}
	// ③ 负控：内容改一格
	if err := VerifyApprovalSignature(pubB64, sigB64,
		ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:01+08:00", "放行一次")); err == nil {
		t.Error("负控失败（改了签的时间还验得过）")
	}
	// ④ 负控：别的公钥
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyApprovalSignature(base64.StdEncoding.EncodeToString(otherPub), sigB64, payload); err == nil {
		t.Error("负控失败（拿别的公钥也验得过 —— 那手写件就能过关了）")
	}
	// ⑤ 负控：空签名 / 坏 base64 / 公钥长度不对
	if err := VerifyApprovalSignature(pubB64, "", payload); err == nil {
		t.Error("负控失败（空签名也验得过）")
	}
	if err := VerifyApprovalSignature(pubB64, "!!not-base64!!", payload); err == nil {
		t.Error("负控失败（坏 base64 也验得过）")
	}
	if err := VerifyApprovalSignature(base64.StdEncoding.EncodeToString([]byte("short")), sigB64, payload); err == nil {
		t.Error("负控失败（公钥长度不对也验得过）")
	}
	// KeyID 稳定且与公钥一一对应
	if KeyID(pubB64) == "" || KeyID(pubB64) != KeyID(pubB64) {
		t.Error("KeyID 应稳定非空")
	}
	if KeyID(pubB64) == KeyID(base64.StdEncoding.EncodeToString(otherPub)) {
		t.Error("两把公钥的 KeyID 撞了")
	}
}

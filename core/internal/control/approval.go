// approval.go —— 人签批准件的**唯一真源**（待签字节形态 + 验签）。
//
// 为什么放在控制层（`core/internal/control`）：批准件是**逃生门**的载体（`require_approval` 无审批即不执行），
// 而「谁签的」这件事必须由**控制层**判 —— 命令面（`zerg approve`）负责**签**，对话层/网关/CA 负责**验**。
// 两处必须算**同一串字节**（待签形态只有一处定义），否则同一枚件在两边会有两种结论。
//
// 不可伪造性从哪来（这是本件存在的理由）：件里带一枚 **Ed25519 签名**，而私钥**只在人的口令下**才解得开
// （`zerg approve keygen` 由人在终端上敲口令生成 · 私钥以口令派生的密钥加密落盘）。
// ⇒ **模型写得出这枚件的字节，但写不出这枚签名**；而消费者**只认签名**（无签名 / 签名坏 ⇒ 不算批准）。
package control

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// ApprovalPayload —— 待签字节（**唯一形态**：签的一方与验的一方都调它）。
//
// 形态：一句版本行 + 五格（`\n` 分隔）——格与格之间不许有歧义（`\n` 不出现在格内：
// 各格在入库前已被 `TrimSpace`，且 `\n` 在格内会被 `SanitizeApprovalField` 换成空格）。
func ApprovalPayload(tool, scope, approver, approvedAt, note string) []byte {
	return []byte("zerg-approval/v1\n" +
		SanitizeApprovalField(tool) + "\n" +
		SanitizeApprovalField(scope) + "\n" +
		SanitizeApprovalField(approver) + "\n" +
		SanitizeApprovalField(approvedAt) + "\n" +
		SanitizeApprovalField(note))
}

// SanitizeApprovalField 把一格里的换行/回车换成空格（防「两格拼成一格」的歧义签名）。
func SanitizeApprovalField(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// KeyID —— 公钥的短身份（`sha256(pub)[:16]` 的十六进制）——件里记它，验签时先对身份再验签。
func KeyID(pubB64 string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubB64))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

// VerifyApprovalSignature —— 验一枚批准件的签名（**唯一判定口**）。
//
// 返回 nil = 验过；否则返回**为什么不算批准**（调用方把它写进拒因，不吞）。
// ★ 读不到 / 解不开 / 长度不对 / 公钥坏 —— 一律**不算批准**（fail-closed，绝不当放行）。
func VerifyApprovalSignature(pubB64, sigB64 string, payload []byte) error {
	pubRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubB64))
	if err != nil {
		return fmt.Errorf("操作员公钥解不开（base64）：%v", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return fmt.Errorf("操作员公钥长度不对（%d 字节 ⇒ 要 %d）", len(pubRaw), ed25519.PublicKeySize)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil {
		return fmt.Errorf("批准件签名解不开（base64）：%v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("批准件签名长度不对（%d 字节 ⇒ 要 %d）", len(sig), ed25519.SignatureSize)
	}
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), payload, sig) {
		return fmt.Errorf("**签名验不过** —— 这枚件不是拿在册私钥签的（手写的件在这里被拦下）")
	}
	return nil
}

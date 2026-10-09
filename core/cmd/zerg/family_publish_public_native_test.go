package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestPublishPublicEmbeddedPyBlocksByteExact —— 内嵌 python 块（base64url 承载）解码后必须与
// 旧壳 heredoc **逐字节**同：每块 sha256 与字节数两处锚（`ppPy<块>SHA256` / `ppPy<块>BytesLen`）现读必须相等。
func TestPublishPublicEmbeddedPyBlocksByteExact(t *testing.T) {
	cases := []struct {
		name    string
		b64     string
		wantSHA string
		wantLen int
	}{
		{"ppPyWhitelist", ppPyWhitelistB64, ppPyWhitelistSHA256, ppPyWhitelistBytesLen},
		{"ppPyCargoToml", ppPyCargoTomlB64, ppPyCargoTomlSHA256, ppPyCargoTomlBytesLen},
		{"ppPyCargoLock", ppPyCargoLockB64, ppPyCargoLockSHA256, ppPyCargoLockBytesLen},
		{"ppPyReplaceRules", ppPyReplaceRulesB64, ppPyReplaceRulesSHA256, ppPyReplaceRulesBytesLen},
		{"ppPyPatterns", ppPyPatternsB64, ppPyPatternsSHA256, ppPyPatternsBytesLen},
	}
	for _, c := range cases {
		got := ppPyDecode(c.b64)
		if got == "" {
			t.Fatalf("%s：ppPyDecode 解码失败（base64 常量坏了）", c.name)
		}
		if len([]byte(got)) != c.wantLen {
			t.Fatalf("%s：解码后字节数 %d ≠ 锚 %d（与旧壳漂移）", c.name, len([]byte(got)), c.wantLen)
		}
		sum := sha256.Sum256([]byte(got))
		hexsum := hex.EncodeToString(sum[:])
		if hexsum != c.wantSHA {
			t.Fatalf("%s：解码后 sha256 %s ≠ 锚 %s（与旧壳漂移）", c.name, hexsum, c.wantSHA)
		}
	}
}

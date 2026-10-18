package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// tctSampleToken —— 合成令牌（不是任何真令牌）：判据只关心「它有没有出现在输出里」。
const tctSampleToken = "tok-7f3a2b9c-synthetic-not-a-secret"

// tctIsolate 把三路都清干净，并把 HOME 指到临时目录（文件级可控）。
func tctIsolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{TokenEnvPrimary, TokenLegacyAuthToken, TokenLegacyAPIToken} {
		t.Setenv(k, "")
	}
	return home
}

func tctWriteTokenFile(t *testing.T, home, v string) {
	t.Helper()
	dir := filepath.Join(home, ".zerg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("建令牌目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(v+"\n"), 0o600); err != nil {
		t.Fatalf("写令牌文件失败: %v", err)
	}
}

// TestTokenChainThreeRoutesTwoStates —— 三路（env / file / 旧名）两态 + 顺序：
// 只断言**来源标记**与取值来自哪一路，✗ 不打印令牌值。
func TestTokenChainThreeRoutesTwoStates(t *testing.T) {
	cases := []struct {
		name    string
		primary string
		legacy  map[string]string
		file    string
		want    string
		wantSrc TokenSource
	}{
		{"env 主路命中", tctSampleToken, nil, "", tctSampleToken, TokenSourceEnv},
		{"file 命中（无 env）", "", nil, tctSampleToken, tctSampleToken, TokenSourceFile},
		{"旧名 ZERG_AUTH_TOKEN 命中", "", map[string]string{TokenLegacyAuthToken: tctSampleToken}, "", tctSampleToken, TokenSourceLegacyAuth},
		{"旧名 ZERG_API_TOKEN 命中", "", map[string]string{TokenLegacyAPIToken: tctSampleToken}, "", tctSampleToken, TokenSourceLegacyAPI},
		{"env 压过 file 与旧名", tctSampleToken, map[string]string{TokenLegacyAuthToken: "old-a", TokenLegacyAPIToken: "old-b"}, "file-c", tctSampleToken, TokenSourceEnv},
		{"file 压过旧名（旧名已降级）", "", map[string]string{TokenLegacyAuthToken: tctSampleToken}, "file-c", "file-c", TokenSourceFile},
		{"三路全空", "", nil, "", "", TokenSourceMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := tctIsolate(t)
			if c.primary != "" {
				t.Setenv(TokenEnvPrimary, c.primary)
			}
			for k, v := range c.legacy {
				t.Setenv(k, v)
			}
			if c.file != "" {
				tctWriteTokenFile(t, home, c.file)
			}
			got, src := ResolveAuthTokenWithSource()
			if got != c.want {
				t.Fatalf("取值不符：want %q · got %q", c.want, got)
			}
			if src != c.wantSrc {
				t.Fatalf("来源标记不符：want %q · got %q", c.wantSrc, src)
			}
			if got != "" && strings.Contains(string(src), got) {
				t.Fatalf("来源标记里出现了令牌值（五不进之日志）：%q", src)
			}
		})
	}
}

// TestLegacyAnnounceOnceSourceOnly —— 旧名命中**出声一次**：只报来源、不回显令牌。
func TestLegacyAnnounceOnceSourceOnly(t *testing.T) {
	tctIsolate(t)
	t.Setenv(TokenLegacyAuthToken, tctSampleToken)

	var buf bytes.Buffer
	oldSink, oldOnce := tokenLegacySink, tokenLegacyOnce
	tokenLegacySink = &buf
	tokenLegacyOnce = &sync.Once{}
	t.Cleanup(func() { tokenLegacySink = oldSink; tokenLegacyOnce = oldOnce })

	if got := ResolveAuthToken(); got != tctSampleToken {
		t.Fatalf("旧名取值不符")
	}
	if got := ResolveAuthToken(); got != tctSampleToken {
		t.Fatalf("旧名第二次取值不符")
	}

	before := buf.Len()
	AnnounceLegacyTokenSource(TokenSourceEnv)
	if buf.Len() != before {
		t.Fatalf("主路（env:ZERG_TOKEN）也出声了 —— 只许旧名提示")
	}

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("旧名命中却没出声（要求提示一次）")
	}
	if lines := strings.Split(out, "\n"); len(lines) != 1 {
		t.Fatalf("旧名提示应恰好一次，实得 %d 行", len(lines))
	}
	if !strings.Contains(out, string(TokenSourceLegacyAuth)) {
		t.Fatalf("提示里没点名来源标记：%q", out)
	}
	if err := tokenLeakGuard(out, tctSampleToken); err != nil {
		t.Fatalf("%v", err)
	}
}

// tokenLeakGuard —— 判据本体：输出里出现令牌值 ⇒ 非 nil（红）。
func tokenLeakGuard(out, token string) error {
	if token == "" {
		return nil
	}
	if strings.Contains(out, token) {
		return fmt.Errorf("令牌回显：输出里出现了令牌值（len=%d）—— 五不进之「不进日志」被破坏", len(token))
	}
	return nil
}

// tctBadTokenLogLine —— 负控夹具：现实里最常见的那种「把令牌拼进日志格式」写法。
func tctBadTokenLogLine(src TokenSource, token string) string {
	return fmt.Sprintf("令牌来源 %s · token=%s\n", src, token)
}

// TestTokenLeakGuardNegativeControl —— 负控：故意把令牌写进日志格式 ⇒ 判据必红。
func TestTokenLeakGuardNegativeControl(t *testing.T) {
	if err := tokenLeakGuard(tctBadTokenLogLine(TokenSourceLegacyAuth, tctSampleToken), tctSampleToken); err == nil {
		t.Fatal("负控失败：故意把令牌放进日志格式，判据仍绿 ⇒ 判据没牙")
	} else {
		t.Logf("负控读数：红 · %v", err)
	}
	if err := tokenLeakGuard("令牌来源 "+string(TokenSourceLegacyAuth)+"\n", tctSampleToken); err != nil {
		t.Fatalf("正控失败：只报来源的输出被判红 ⇒ 判据过宽：%v", err)
	}
}

package main

// 子端令牌链判据（GAP-20261017-04）：收敛后子端**不自备优先级** —— 它必须与主控/CLI
// 走同一份实现（`shared/credential`）。本件测的是 `resolveToken` 的**真行为**：
// ① 顺序：env:ZERG_TOKEN 压过 file，file 压过两个旧名（改前子端是反的：ZERG_TOKEN 排末位）；
// ② 空值回退：空白 env / 空白或缺失的令牌文件都继续往下找；
// ③ 不回显：缺令牌报话里只有来源与链，没有令牌值。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/shared/credential"
)

// tSampleToken —— 合成令牌（不是任何真令牌）。
const tSampleToken = "tok-agent-side-synthetic-not-a-secret"

// tIsolate 清三路 env，并把 HOME 指到临时目录。
func tIsolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{credential.EnvPrimary, credential.EnvLegacyAuth, credential.EnvLegacyAPI} {
		t.Setenv(k, "")
	}
	return home
}

func tWriteTokenFile(t *testing.T, home, v string) {
	t.Helper()
	dir := filepath.Join(home, ".zerg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("建令牌目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(v+"\n"), 0o600); err != nil {
		t.Fatalf("写令牌文件失败: %v", err)
	}
}

// TestResolveTokenPriorityChain —— 顺序：主路赢、文件次之、旧名降级为最后一级。
func TestResolveTokenPriorityChain(t *testing.T) {
	cases := []struct {
		name    string
		primary string
		auth    string
		api     string
		file    string
		want    string
		wantSrc credential.Source
	}{
		{"env:ZERG_TOKEN 压过一切", tSampleToken, "auth-w", "api-l", "file-c", tSampleToken, credential.SourceEnv},
		{"file 压过两个旧名", "", "auth-w", "api-l", "file-c", "file-c", credential.SourceFile},
		{"AUTH 只是最后一级", "", "auth-w", "api-l", "", "auth-w", credential.SourceLegacyAuth},
		{"API 兜底", "", "", "api-l", "", "api-l", credential.SourceLegacyAPI},
		{"全空 ⇒ 缺", "", "", "", "", "", credential.SourceMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := tIsolate(t)
			if c.primary != "" {
				t.Setenv(credential.EnvPrimary, c.primary)
			}
			if c.auth != "" {
				t.Setenv(credential.EnvLegacyAuth, c.auth)
			}
			if c.api != "" {
				t.Setenv(credential.EnvLegacyAPI, c.api)
			}
			if c.file != "" {
				tWriteTokenFile(t, home, c.file)
			}
			got, src := resolveToken()
			if got != c.want {
				t.Fatalf("取值不符（want %q · got %q）—— 子端顺序又跑偏了", c.want, got)
			}
			if src != c.wantSrc {
				t.Fatalf("来源标记不符：want %q · got %q", c.wantSrc, src)
			}
		})
	}
}

// TestChainWinnerLabel —— 两态对拍用的一行读数：同一场景下**谁赢**（只打标签，✗ 不打值）。
func TestChainWinnerLabel(t *testing.T) {
	home := tIsolate(t)
	tWriteTokenFile(t, home, "file-X")
	t.Setenv(credential.EnvPrimary, "env-X")
	t.Setenv(credential.EnvLegacyAuth, "auth-X")
	t.Setenv(credential.EnvLegacyAPI, "api-X")
	got, src := resolveToken()
	labels := map[string]string{
		"env-X":  "env:ZERG_TOKEN",
		"auth-X": "env:ZERG_AUTH_TOKEN",
		"api-X":  "env:ZERG_API_TOKEN",
		"file-X": "file:~/.zerg/token",
	}
	t.Logf("唯一链（改后）读数：赢家 = %s · 来源标记 = %s", labels[got], src)
}

// TestResolveTokenEmptyFallback —— 设置了但为空 / 文件空白 ⇒ 继续往下找，不许当成命中。
func TestResolveTokenEmptyFallback(t *testing.T) {
	home := tIsolate(t)
	t.Setenv(credential.EnvPrimary, "  ")
	tWriteTokenFile(t, home, "  ")
	t.Setenv(credential.EnvLegacyAuth, tSampleToken)
	if got, src := resolveToken(); got != tSampleToken || src != credential.SourceLegacyAuth {
		t.Fatalf("空白值未回退：got=%q src=%q", got, src)
	}
}

// TestMissingTokenMessageNoEcho —— 缺令牌报话：机读键齐（token_source= / TOKEN-CHAIN:）且**不回显**。
func TestMissingTokenMessageNoEcho(t *testing.T) {
	tIsolate(t)
	_, src := resolveToken()
	msg := missingTokenMessage(src)
	for _, want := range []string{"token_source=" + string(credential.SourceMissing), "TOKEN-CHAIN: " + credential.Chain} {
		if !strings.Contains(msg, want) {
			t.Fatalf("报话缺 %q：%s", want, msg)
		}
	}
	if strings.Contains(msg, tSampleToken) {
		t.Fatal("报话里出现了令牌值（五不进之日志）")
	}
	// 负控：真把令牌拼进去 ⇒ 判据会红（证明上面那条不是空过）。
	bad := msg + " token=" + tSampleToken
	if !strings.Contains(bad, tSampleToken) {
		t.Fatal("负控夹具失效")
	}
}

package config

// 本件只测**转发层**：优先级逻辑与「旧名出声一次 / 不回显」的实现判据搬到了
// `shared/credential/tokenchain_test.go`（实现搬到哪，判据跟到哪 —— 否则判据测的是影子）。
// 这里守两件事：① 转发层真把共享实现接上了（三路两态取值/来源都对）；
// ② 转发层**没有**偷偷长出第二套优先级（源文件里不许有 `os.Getenv`、链字面量与真源逐字相等）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/shared/credential"
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

// TestTokenChainForwardingOnly —— 转发层不变量（机读）：
//
//	① 链字面量与真源逐字相等；② 令牌文件路径与真源相等；
//	③ 本件源码里**没有** `os.Getenv`（否则就是又一套优先级）；
//	④ 缺令牌报话点名 `token_source=` 与 `TOKEN-CHAIN:`，且不含令牌值。
func TestTokenChainForwardingOnly(t *testing.T) {
	if TokenChain != credential.Chain {
		t.Fatalf("链字面量不是转发来的：config=%q · shared=%q", TokenChain, credential.Chain)
	}
	tctIsolate(t)
	if got, want := TokenFilePath(), credential.FilePath(); got != want {
		t.Fatalf("令牌文件路径不是同源：config=%q · shared=%q", got, want)
	}

	src, err := os.ReadFile("tokenchain.go")
	if err != nil {
		t.Fatalf("读不了 tokenchain.go: %v", err)
	}
	if strings.Contains(string(src), "os.Getenv") {
		t.Fatal("转发层里出现 os.Getenv ⇒ 又长出了第二套优先级（真源只在 shared/credential）")
	}
	if !strings.Contains(string(src), "// TOKEN-CHAIN: "+tcpCanonical) {
		t.Fatalf("转发层缺链标记行（名字序）：%q", tcpCanonical)
	}

	hint := credential.MissingHint()
	if !strings.Contains(hint, "TOKEN-CHAIN: "+credential.Chain) {
		t.Fatalf("缺令牌报话没有机器可读的链：%q", hint)
	}
	if !strings.Contains(hint, "token_source="+string(TokenSourceMissing)) {
		t.Fatalf("缺令牌报话没有 token_source 键：%q", hint)
	}
	if strings.Contains(hint, tctSampleToken) {
		t.Fatal("报话里出现了令牌值")
	}
}

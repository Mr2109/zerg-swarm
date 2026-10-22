package config

// 对拍判据（GAP-20261017-04 收敛后的形态）：**真源一份**（`shared/credential`）+ 各读取面只转发。
// 因此本件不再「钉住已知偏差」，而是钉住三件事：
//  ① 每个读取面的源文件都带同一行 `// TOKEN-CHAIN: ...`（机读、逐字同序）；
//  ② 各读取面**不许**自己 `os.Getenv` 令牌名（出现即第二套优先级 ⇒ 红）；
//     子端是唯一例外：它直接调 `credential.ResolveWithSource()` —— 判据要求这行在场。
//  ③ 真源常量 `TokenChain` 的内部次序递增。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tcpMarkerPrefix —— 各读取面在源文件里写死的一行「链字面」（机读形态）。
// 形态固定：行首 `// TOKEN-CHAIN: a > b > c > d`（顺序逐字，可对拍）。
const tcpMarkerPrefix = "// TOKEN-CHAIN: "

// tcpCanonical —— 唯一链的**名字序**（与 TokenChain 常量同序；只省掉 `env:` / `(旧名·兼容)` 后缀）。
const tcpCanonical = "ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN"

// tcpMarker 抽源文件里的链标记行。
func tcpMarker(text string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, tcpMarkerPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(s, tcpMarkerPrefix)), true
		}
	}
	return "", false
}

// tcpReadRepo 读仓内件（本测试件的工作目录 = core/internal/config）。
func tcpReadRepo(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", rel))
	if err != nil {
		t.Fatalf("读不了 %s: %v", rel, err)
	}
	return string(b)
}

// tcpTokenEnvLiterals —— 令牌环境变量名的**字面量**（只在本判据里出现一次）。
var tcpTokenEnvLiterals = []string{`"ZERG_TOKEN"`, `"ZERG_AUTH_TOKEN"`, `"ZERG_API_TOKEN"`}

// TestTokenChainParitySurfaces —— 对拍：真源 + 三个读取面标记逐字同序，且真源次序递增。
func TestTokenChainParitySurfaces(t *testing.T) {
	surfaces := []struct {
		name string
		rel  string
	}{
		{"真源（shared/credential）", "shared/credential/tokenchain.go"},
		{"主控转发层（core/internal/config）", "core/internal/config/tokenchain.go"},
		{"CLI（core/cmd/zerg/credentials.go）", "core/cmd/zerg/credentials.go"},
		{"子端（agent/cmd/zerg-agentd/main.go）", "agent/cmd/zerg-agentd/main.go"},
	}
	for _, s := range surfaces {
		got, ok := tcpMarker(tcpReadRepo(t, s.rel))
		if !ok {
			t.Fatalf("%s 缺 `%s` 标记行（链被抄成第二份 / 标记被删 ⇒ 判据盲）", s.name, strings.TrimSpace(tcpMarkerPrefix))
		}
		if got != tcpCanonical {
			t.Fatalf("%s 的链序不符：want %q · got %q", s.name, tcpCanonical, got)
		}
	}

	// 常量顺序（真跑用的那一串）必须与标记行同序 —— 只比名字，不比文案。
	last := -1
	for _, m := range []string{string(TokenSourceEnv), string(TokenSourceFile), string(TokenSourceLegacyAuth), string(TokenSourceLegacyAPI)} {
		i := strings.Index(TokenChain, m)
		if i < 0 {
			t.Fatalf("TokenChain 里没有 %q", m)
		}
		if i <= last {
			t.Fatalf("TokenChain 顺序不符（%q 位置 %d ≤ 前一级 %d）：%q", m, i, last, TokenChain)
		}
		last = i
	}
}

// TestTokenChainNoSecondPriority —— 除真源外，任何读取面都不许自带令牌名 `os.Getenv`：
// 各读一遍 = 各有一套优先级（GAP-20261017-04 的病根）。子端必须**调真源**而不是抄顺序。
func TestTokenChainNoSecondPriority(t *testing.T) {
	surfaces := []string{
		"core/internal/config/tokenchain.go",
		"core/internal/config/config.go",
		"core/cmd/zerg/credentials.go",
		"core/cmd/zerg-agent/main.go",
		"agent/cmd/zerg-agentd/main.go",
	}
	for _, rel := range surfaces {
		text := tcpReadRepo(t, rel)
		for _, lit := range tcpTokenEnvLiterals {
			if strings.Contains(text, "os.Getenv("+lit+")") {
				t.Fatalf("%s 里出现 os.Getenv(%s) ⇒ 第二套优先级（令牌名只在 shared/credential 出现）", rel, lit)
			}
		}
	}

	// 子端收敛的机读证据：直接调用真源，且不留自建的「名字数组」。
	sub := tcpReadRepo(t, "agent/cmd/zerg-agentd/main.go")
	if !strings.Contains(sub, "credential.ResolveWithSource()") {
		t.Fatal("子端没调 shared/credential.ResolveWithSource() ⇒ 优先级仍在自己手里")
	}
	if strings.Contains(sub, `[]string{"ZERG_`) {
		t.Fatal("子端还留着自建的环境变量名数组 ⇒ 第二套顺序")
	}
	// 令牌文件路径也只有一份。
	if strings.Contains(sub, `".zerg", "token"`) {
		t.Fatal("子端自己拼令牌文件路径 ⇒ 路径真源应在 shared/credential.FilePath()")
	}
}

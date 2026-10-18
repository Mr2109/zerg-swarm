package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tcpMarkerPrefix —— 三处读取面各自在源文件里写死的一行「链字面」（机读形态）。
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

// tcpReadCore 读 core 内的件（本测试件的工作目录 = core/internal/config）。
func tcpReadCore(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("读不了 %s: %v", rel, err)
	}
	return string(b)
}

// TestTokenChainParityCoreSurfaces —— 对拍（甲）：CLI 与主控**逐字同序**，且与 TokenChain 常量同序。
// 两处都走 `config.ResolveAuthTokenWithSource`，标记行是给「有人把链抄成第二份」留的牙。
func TestTokenChainParityCoreSurfaces(t *testing.T) {
	surfaces := []struct {
		name string
		rel  string
	}{
		{"主控（core/internal/config）", "internal/config/tokenchain.go"},
		{"CLI（core/cmd/zerg/credentials.go）", "cmd/zerg/credentials.go"},
	}
	for _, s := range surfaces {
		got, ok := tcpMarker(tcpReadCore(t, s.rel))
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

// tcpAgentKnownDeviation —— 子端**现读**顺序（本单不许动 `agent/**` ⇒ 见 GAP-20261017-04 的「子端那半」）。
// 钉住它：改成唯一链 ⇒ 本判据自动放行（并提示已收敛）；变成第三种顺序 ⇒ 当场红。
const tcpAgentKnownDeviation = "ZERG_AUTH_TOKEN > ZERG_API_TOKEN > ZERG_TOKEN > ~/.zerg/token"

var (
	tcpAgentFuncRE = regexp.MustCompile(`(?s)func resolveToken\(\) string \{.*?\[\]string\{([^}]*)\}`)
	tcpAgentNameRE = regexp.MustCompile(`"([A-Z0-9_]+)"`)
)

// tcpAgentOrder 从子端源文件抽「环境变量名序 + 令牌文件」，得到可对拍的顺序串。
func tcpAgentOrder(text string) (string, bool) {
	m := tcpAgentFuncRE.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	var names []string
	for _, q := range tcpAgentNameRE.FindAllStringSubmatch(m[1], -1) {
		names = append(names, q[1])
	}
	if len(names) == 0 || !strings.Contains(text, `".zerg", "token"`) {
		return "", false
	}
	return strings.Join(names, " > ") + " > ~/.zerg/token", true
}

// TestTokenChainParitySubend —— 对拍（乙）：子端在另一 module（`agent/go.mod`）⇒ 不能 import 本件，
// 只能按源文件对拍。已知偏差钉死在此，✗ 不静默放过第三种形态。
func TestTokenChainParitySubend(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "agent", "cmd", "zerg-agentd", "main.go"))
	if err != nil {
		t.Fatalf("读不了子端 main.go: %v", err)
	}
	got, ok := tcpAgentOrder(string(b))
	if !ok {
		t.Fatal("子端 resolveToken 形态变了：抽不出顺序（判据盲 ⇒ 必须人工核）")
	}
	switch got {
	case tcpCanonical:
		t.Logf("子端已收敛到唯一链：%s", got)
	case tcpAgentKnownDeviation:
		t.Logf("子端仍为已知偏差（GAP-20261017-04 · 需另开一单）：%s", got)
	default:
		t.Fatalf("子端令牌顺序出现第三形态：%s（唯一链 %q · 已知偏差 %q）", got, tcpCanonical, tcpAgentKnownDeviation)
	}
}

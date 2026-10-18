package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// tcmSampleToken —— 合成令牌（不是真令牌）：判据只关心「它有没有出现在输出里」。
const tcmSampleToken = "tok-master-4c1d-synthetic-only"

// tcmIsolate 三路环境变量清空 + HOME 指到临时目录（文件级可控）。
func tcmIsolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{config.TokenEnvPrimary, config.TokenLegacyAuthToken, config.TokenLegacyAPIToken} {
		t.Setenv(k, "")
	}
	return home
}

// tcmWriteFleet 落一份最小 fleet.yaml（只含 auth.token）。
func tcmWriteFleet(t *testing.T, yamlToken string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fleet.yaml")
	if err := os.WriteFile(p, []byte("auth:\n  token: "+yamlToken+"\n"), 0o600); err != nil {
		t.Fatalf("写 fleet.yaml 失败: %v", err)
	}
	return p
}

func tcmWriteTokenFile(t *testing.T, home, v string) {
	t.Helper()
	dir := filepath.Join(home, ".zerg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("建令牌目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(v+"\n"), 0o600); err != nil {
		t.Fatalf("写令牌文件失败: %v", err)
	}
}

// TestMasterTokenChainThreeRoutes —— 主控（LoadFleetConfig）三路两态：取值与**来源标记**必须对，
// 且来源标记里不得出现令牌值（只显来源，不显值）。
func TestMasterTokenChainThreeRoutes(t *testing.T) {
	cases := []struct {
		name    string
		primary string
		legacy  map[string]string
		file    string
		yaml    string
		wantTok string
		wantSrc config.TokenSource
	}{
		{"env:ZERG_TOKEN 主路（压过 yaml / 旧名）", tcmSampleToken, map[string]string{config.TokenLegacyAuthToken: "old-name"}, "", "yaml-value", tcmSampleToken, config.TokenSourceEnv},
		{"file 命中（无 env）", "", nil, tcmSampleToken, "yaml-value", tcmSampleToken, config.TokenSourceFile},
		{"旧名 ZERG_AUTH_TOKEN（降级档）", "", map[string]string{config.TokenLegacyAuthToken: tcmSampleToken}, "", "yaml-value", tcmSampleToken, config.TokenSourceLegacyAuth},
		{"旧名 ZERG_API_TOKEN（降级档）", "", map[string]string{config.TokenLegacyAPIToken: tcmSampleToken}, "", "yaml-value", tcmSampleToken, config.TokenSourceLegacyAPI},
		{"yaml 兜底", "", nil, "", "yaml-value", "yaml-value", config.TokenSourceConfig},
		{"三路全空 · yaml 也空", "", nil, "", "", "", config.TokenSourceMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := tcmIsolate(t)
			if c.primary != "" {
				t.Setenv(config.TokenEnvPrimary, c.primary)
			}
			for k, v := range c.legacy {
				t.Setenv(k, v)
			}
			if c.file != "" {
				tcmWriteTokenFile(t, home, c.file)
			}
			cfg, err := config.LoadFleetConfig(tcmWriteFleet(t, c.yaml))
			if err != nil {
				t.Fatalf("加载失败: %v", err)
			}
			if cfg.Auth.Token != c.wantTok {
				t.Fatalf("取值不符：want %q · got %q", c.wantTok, cfg.Auth.Token)
			}
			if cfg.Auth.Source != string(c.wantSrc) {
				t.Fatalf("来源标记不符：want %q · got %q", c.wantSrc, cfg.Auth.Source)
			}
			if cfg.Auth.Token != "" && strings.Contains(cfg.Auth.Source, cfg.Auth.Token) {
				t.Fatalf("来源标记里出现了令牌值（五不进之日志）：%q", cfg.Auth.Source)
			}
		})
	}
}

// TestMasterAuthUsesConvergedToken —— 行为面：主控认证真用收敛后的那一枚（ZERG_TOKEN 优先，
// 旧名同时在场也不改判），证明「认 ZERG_TOKEN」不是只写在注释里。
func TestMasterAuthUsesConvergedToken(t *testing.T) {
	tcmIsolate(t)
	t.Setenv(config.TokenEnvPrimary, tcmSampleToken)
	t.Setenv(config.TokenLegacyAuthToken, "old-name-value")

	cfg, err := config.LoadFleetConfig(tcmWriteFleet(t, "yaml-value"))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Auth.Source != string(config.TokenSourceEnv) {
		t.Fatalf("来源标记不符：got %q", cfg.Auth.Source)
	}

	h := AuthMiddleware(cfg.Auth.Token)
	status := func(tok string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		if tok != "" {
			req.Header.Set("X-Auth-Token", tok)
		}
		h(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })).ServeHTTP(rec, req)
		return rec.Code
	}
	if got := status(tcmSampleToken); got != http.StatusOK {
		t.Fatalf("主控没认 env:ZERG_TOKEN（want 200 · got %d）", got)
	}
	if got := status("old-name-value"); got != http.StatusForbidden {
		t.Fatalf("旧名同时在场却仍被主控认下（want 403 · got %d）—— 旧名必须降级", got)
	}
}

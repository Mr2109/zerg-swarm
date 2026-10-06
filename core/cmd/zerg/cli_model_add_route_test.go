package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// zz_gap42_route_verify_test.go —— GAP-20260925-42 的两态机检（**只走命令面 + HTTP**，不引用本笔
// 新增的 Go 符号 ⇒ 改前/改后源码都能编译，改前必红、改后必绿）。
//
// 造一个临时「仓根」（`ZERG_REPO` 指它），名册件 = `<tmp>/gateway/fleet.yaml`（即主控现读件的
// 同形路径）；再起一台**假主控**（`ZERG_PORT` 指它）当 `POST /api/config/reload` 与
// `GET /api/fleet/models` 的出口。
//
// 判据面：
//  正控 —— 假主控的路由表**认**这一条 ⇒ `model add` rc=0，stderr 含「现读路由表已认」。
//  负控 —— 假主控 reload 报 ok 而路由表**不认** ⇒ rc≠0 且名册件**逐字节回滚**（改前会 rc=0 且名册留着新条）。

const zzFleet = `models:
  m-old:
    - { host: fake, backend: llama-server, file: "/models/old.gguf", mem_gb: 6 }

fleet:
  fake:
    host: 127.0.0.1
    port: 59999
`

func zzPortOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("解析假主控 URL 失败：%v", err)
	}
	if _, err := strconv.Atoi(u.Port()); err != nil {
		t.Fatalf("假主控端口不认：%q", u.Port())
	}
	return u.Port()
}

// zzFakeCore —— 假主控：`POST /api/config/reload` 恒 ok；`GET /api/fleet/models` 回 inject 的体。
func zzFakeCore(t *testing.T, fleetModelsJSON string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config/reload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","models":2,"added":1}`))
	})
	mux.HandleFunc("/api/fleet/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fleetModelsJSON))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return zzPortOf(t, s.URL)
}

func zzTempRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatalf("建临时仓根失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(zzFleet), 0o644); err != nil {
		t.Fatalf("写临时名册失败：%v", err)
	}
	return root
}

func zzSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", p, err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func zzAddArgs(root string) []string {
	// 用 `--path <临时仓根>/gateway/fleet.yaml`：与 `coreRosterPath()`（按 `ZERG_REPO` 推仓根）**同形同值**
	// ⇒ 联动触发。夹具件在 `t.TempDir()` 里，绝不碰真名册。
	return []string{"model", "add", "--path", filepath.Join(root, "gateway", "fleet.yaml"),
		"--model", "m-new", "--host", "fake", "--file", "/models/new.gguf",
		"--confirm", "m-new", "--yes"}
}

func TestGAP42_ModelAddVerifiesRoute(t *testing.T) {
	// ── 正控：路由表**认** ⇒ rc=0 且点名已认 ──
	root := zzTempRepo(t)
	port := zzFakeCore(t, `{"count":2,"models":[{"id":"m-old","host":"fake","file":"/models/old.gguf"},{"id":"m-new","host":"fake","file":"/models/new.gguf"}]}`)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_PORT", port)
	rc, out, errb := runCapture(zzAddArgs(root)...)
	if rc != 0 {
		t.Fatalf("正控：rc=%d（要 0）· out=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "现读路由表已认") {
		t.Fatalf("改前特征：stderr **没有**「现读路由表已认」（改后必须有 —— 写后即 reload + 现读校验）：stderr=%s", errb)
	}
	if !strings.Contains(out, "m-new") {
		t.Errorf("正控：行式面里没有 m-new：out=%s", out)
	}
	if !strings.Contains(zzRead(t, filepath.Join(root, "gateway", "fleet.yaml")), "/models/new.gguf") {
		t.Errorf("正控：名册件里没写进新条")
	}
}

func TestGAP42_ModelAddRollsBackWhenRouteDisagrees(t *testing.T) {
	// ── 负控：reload 报 ok 而路由表**不认** ⇒ rc≠0 + 名册件逐字节回滚 ──
	root := zzTempRepo(t)
	roster := filepath.Join(root, "gateway", "fleet.yaml")
	before := zzSHA(t, roster)
	// 假主控的路由表只有旧条（不认 m-new）。
	port := zzFakeCore(t, `{"count":1,"models":[{"id":"m-old","host":"fake","file":"/models/old.gguf"}]}`)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_PORT", port)
	rc, _, errb := runCapture(zzAddArgs(root)...)
	if rc == 0 {
		t.Fatalf("改前特征：reload 报 ok 而路由表不认，`model add` 却 rc=0（改后必须 rc≠0）：stderr=%s", errb)
	}
	if !strings.Contains(errb, "回滚") {
		t.Errorf("负控：判词里没有「回滚」：stderr=%s", errb)
	}
	if after := zzSHA(t, roster); after != before {
		t.Errorf("负控：名册件**没有**逐字节回滚（before=%s after=%s）", before[:16], after[:16])
	}
}

func zzRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", p, err)
	}
	return string(b)
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFleetLoadableSelfTest 跑本门自检（与 py `--self-test` 同五格）。
func TestFleetLoadableSelfTest(t *testing.T) {
	if rc := cflSelfTest(); rc != 0 {
		t.Fatalf("自检不过 rc=%d", rc)
	}
}

// TestFleetLoadableRealFleet 对真仓 gateway/fleet.yaml 跑一遍（两态用 `--fleet` 指候选件）。
func TestFleetLoadableRealFleet(t *testing.T) {
	p := os.Getenv("CFL_FLEET")
	if p == "" {
		p = filepath.Join("..", "..", "..", "gateway", "fleet.yaml")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("找不到 fleet.yaml（%s）：%v", p, err)
	}
	rc, findings := cflCheck(p)
	t.Logf("rc=%d findings=%d", rc, len(findings))
	for _, f := range findings {
		t.Logf("  [%s] %s", f.face, f.msg)
	}
}

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

// TestEggEntries_PidPassthrough —— pid 必须透出装配器与 JSON 载荷（`zerg agent engines` 的 pid 列真源，
// 缺口 `GAP-20260927-04`）。装配器丢了这一格 ⇒ 主控端 pid 恒缺席、`agent engines` 只剩端口可看。
func TestEggEntries_PidPassthrough(t *testing.T) {
	obs := []backend.EggObservation{{EggID: "e1", Model: "e1", State: "ready", Port: 9400, PID: 4242}}
	got := eggEntries(obs, nil, nil)
	if len(got) != 1 {
		t.Fatalf("应装配出 1 条，实得 %d", len(got))
	}
	if got[0].PID != 4242 {
		t.Fatalf("pid 必须透出装配器，实得 %+v", got[0])
	}
	b, err := json.Marshal(got[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"pid":4242`) {
		t.Fatalf("载荷应带 pid=4242，实得 %s", b)
	}
}

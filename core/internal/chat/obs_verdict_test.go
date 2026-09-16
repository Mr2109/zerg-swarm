package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 丙+丁 落盘守卫：verdict / gate_sec / waited_ms / queued_ms 必须真写进记录，且「卡」与「慢」落成不同结论。
// 同包测试：直接置内部计时字段（不依赖真实等待，避免用 sleep 把测试变慢/变脆）。
func TestObsVerdictLandsInRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	// ① 首字节超闸（121s > 120s）⇒ 卡
	tm := NewObsTimer("sess-V1", 1, "Qwen3.8-27B")
	tm.SetGate(120)
	tm.start = time.Now().Add(-121 * time.Second)
	tm.firstByte = tm.start.Add(121 * time.Second)
	tm.chunks = 1
	tm.Finish("upstream_timeout")

	b, err := os.ReadFile(filepath.Join(dir, "chat_obs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(strings.TrimSpace(string(b)), "\n")[0]
	var rec struct {
		EndReason string `json:"end_reason"`
		Turn      struct {
			Verdict  string `json:"verdict"`
			GateSec  int    `json:"gate_sec"`
			WaitedMS int64  `json:"waited_ms"`
			QueuedMS int64  `json:"queued_ms"`
		} `json:"turn"`
	}
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("记录不是合法 JSON：%v\n%s", err, line)
	}
	if rec.Turn.Verdict != string(VerdictStalled) {
		t.Errorf("首字节超闸应判「卡」，实际 verdict=%q（记录：%s）", rec.Turn.Verdict, line)
	}
	if rec.Turn.GateSec != 120 || rec.Turn.WaitedMS < 120000 {
		t.Errorf("闸/等待未落盘：gate=%d waited=%d", rec.Turn.GateSec, rec.Turn.WaitedMS)
	}
	if rec.Turn.QueuedMS != 0 {
		t.Errorf("未测到排队时不得编造：queued=%d", rec.Turn.QueuedMS)
	}

	// ② 首字节很快、中途停 10s（< 60s 阈）⇒ 慢（不该杀）；且排队 2s 必须落盘
	tm2 := NewObsTimer("sess-V2", 1, "Qwen3.8-27B")
	tm2.SetGate(600)
	tm2.SetQueued(2 * time.Second)
	tm2.start = time.Now().Add(-14 * time.Second)
	tm2.firstByte = tm2.start.Add(4 * time.Second)
	tm2.stall = 10 * time.Second
	tm2.chunks = 5
	tm2.Finish("finish")

	b2, _ := os.ReadFile(filepath.Join(dir, "chat_obs.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(b2)), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"verdict":"慢"`) {
		t.Errorf("「慢」应落盘且与「卡」不同，实际记录：%s", last)
	}
	if !strings.Contains(last, `"queued_ms":2000`) {
		t.Errorf("排队时长应落盘：%s", last)
	}
}

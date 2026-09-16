package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 乙-2：子端自报的排队时长必须能落盘；缺失时**不得编造**（不写记录）。
func TestObsQueuedWritesRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	ObsQueued("<worker-ip>", "Qwen3.8-27B", 1234)
	b, err := os.ReadFile(filepath.Join(dir, "chat_obs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	for _, want := range []string{`"kind":"queued"`, `"host":"<worker-ip>"`, `"queued_ms":1234`, "Qwen3.8-27B"} {
		if !strings.Contains(line, want) {
			t.Errorf("记录缺少 %s：%s", want, line)
		}
	}
}

// 负数（异常值） ⇒ 不写（不编造）
func TestObsQueuedRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	ObsQueued("h", "m", -1)
	if _, err := os.Stat(filepath.Join(dir, "chat_obs.jsonl")); err == nil {
		t.Error("负值不应落盘")
	}
}

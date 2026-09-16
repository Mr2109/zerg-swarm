package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 丁-1：回落必须落盘（谁→谁、原因原文），且 best-effort（目录不可写不得 panic）。
func TestObsFailoverWritesRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	ObsFailover("x3", "Mr2109", `Post "http://<worker-ip>:8100/infer": net/http: timeout awaiting response headers`, "Qwen3.8-27B")

	b, err := os.ReadFile(filepath.Join(dir, "chat_obs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	for _, want := range []string{`"kind":"failover"`, `"failover_from":"x3"`, `"failover_to":"Mr2109"`, "timeout awaiting response headers"} {
		if !strings.Contains(line, want) {
			t.Errorf("记录缺少 %s：%s", want, line)
		}
	}
}

func TestObsFailoverBestEffort(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", "/dev/null/nope") // 不可写
	ObsFailover("x3", "Mr2109", "x", "m")          // 不得 panic / 不得外抛
}

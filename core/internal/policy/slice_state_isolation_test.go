// slice_state_isolation_test.go — 本包测试的状态目录隔离（B 项⑤ 起必需）。
//
// 背景：B 项⑤ 起 `RaiseClarification` / `SettleClarification` 会落片观测事件
// （`statepath.File("slice-mount-events.jsonl")`）。本包既有用例不隔离 ZERG_STATE_DIR，
// 若不隔离，跑一次 `go test ./internal/policy/` 就会往用户真实状态目录追加观测行 ——
// 违反本仓明写的纪律（见 internal/chat/obs_behavior_test.go:15）。
//
// 包级 TestMain 钉临时目录（**不改任何既有用例**；用例自己 t.Setenv 时以后者为准）。
package policy

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "zerg-policy-test-state")
	if err != nil {
		panic("建临时状态目录失败: " + err.Error())
	}
	_ = os.Setenv("ZERG_STATE_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// slice_state_isolation_test.go — 本包测试的状态目录隔离（B 项⑤ 起必需）。
//
// 背景：B 项⑤ 起 `SendBack` 每次成功打回都会落一行片观测事件
// （`statepath.File("slice-mount-events.jsonl")`）。本包既有用例**不隔离** ZERG_STATE_DIR
// （它们原先不触发任何状态目录写入），若不隔离，跑一次 `go test ./internal/loopcore/`
// 就会往用户真实状态目录里追加观测行 —— 违反本仓明写的纪律
// （见 internal/chat/obs_behavior_test.go:15「观测一律落隔离目录…绝不写用户真实状态」）。
//
// 做法：包级 TestMain 把 ZERG_STATE_DIR 钉到临时目录（**不改任何既有用例**：
// 逐条 t.Setenv 会动到 11 个既有函数，风险更大）。用例自己再 t.Setenv 时以后者为准。
package loopcore

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "zerg-loopcore-test-state")
	if err != nil {
		panic("建临时状态目录失败: " + err.Error())
	}
	_ = os.Setenv("ZERG_STATE_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

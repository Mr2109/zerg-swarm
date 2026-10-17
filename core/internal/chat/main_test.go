package chat

import (
	"os"
	"os/exec"
	"testing"
)

// main_test.go — 2026-10-17：本包测试会经两条链把事实写进**真机状态目录** `~/.zerg/state/`：
//
//	① 观测账 `chat_obs.jsonl`：agent/tool_decision.go:77 obsEndCall / :56 obsDeny → toolobs.Emit
//	   → 本包 obs.go:813 init() 注册的 sink → obs.go:800 ObsToolDecision → obs.go:435 obsWrite
//	   → obs.go:444 statepath.Dir() ⇒ 追加进真账（chat_tool_register_test.go:37/:49 只隔离了
//	   ExecContext.WorkDir、没设 ZERG_STATE_DIR；实测 129 条 tool_decision 里 120 条是 go test 产物）。
//	② 工具账 `tool_uses.json` ＋ 事件账 `tool_events/zerg-tool-events-<日>.jsonl`：
//	   chat_tool_extra.go:44 agent.RecordToolUse / :45 agent.AppendToolEvent（chat_tool_register_test.go:58 也走这条）。
//
// 后两本的路径在 `internal/agent` 包 **init 期**就定死（tool_uses.go:25 · tool_events.go:24 都是包级变量）
// ⇒ 进程内、测试体级改 env 已经太晚（所以 chat 里那些 t.Setenv 用例照样污染这两本）；
// 唯有用**重执行自身**的兜底，才能让 ZERG_STATE_DIR 在包 init 之前就摆好。
// 形制照同仓先例 internal/agent/tool_uses_test.go:117-139 func TestMain：
// 先尝试进程内改环境，再用带临时状态目录重执行自身兜底，跑完 RemoveAll。
func TestMain(m *testing.M) {
	if os.Getenv("ZERG_TEST_STATE_ISOLATED") == "1" {
		os.Exit(m.Run())
	}
	if os.Getenv("ZERG_STATE_DIR") == "" {
		d, err := os.MkdirTemp("", "zerg-chat-test-state-")
		if err == nil {
			// 兜底：带环境重执行自身，确保包初始化期就能看到隔离目录
			cmd := exec.Command(os.Args[0], os.Args[1:]...)
			cmd.Env = append(os.Environ(), "ZERG_STATE_DIR="+d, "ZERG_TEST_STATE_ISOLATED=1")
			cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
			err := cmd.Run()
			_ = os.RemoveAll(d)
			if cmd.ProcessState != nil {
				os.Exit(cmd.ProcessState.ExitCode())
			}
			if err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

// engine_argv_test.go —— GAP-20260926-01（engine argv）的回归牙。
//
// 缺口语境：生效引擎的命令行原先**只在 stderr.log 的 spawn 行**里，命令面机读不到。
// 本牙钉死「真 argv 记到卵上 + 观测面如实透出」两半，并给出**负控**：
// 未记 argv 的卵 ⇒ 观测面如实缺席（nil），**不编**一个空数组（nil ⟷ 空数组是两回事）。
package backend

import (
	"reflect"
	"testing"
)

// TestEngineArgv_RetainedAndExposed 正控：buildEngineArgv 的产出经 engineArgvOf 记到卵上后，
// 观测面（EggObservations，/eggs 的真源）**逐字**透出 —— 可执行在前、参数在后，一项不改。
func TestEngineArgv_RetainedAndExposed(t *testing.T) {
	entry := newNPTestEntry("/w/run-argv.sh -m /w/m.gguf -c 131072")
	cmdPath, execArgs := buildEngineArgv("gemma-4-26B", entry, 9400)
	want := engineArgvOf(cmdPath, execArgs)

	// 正控①：engineArgvOf = 可执行 + 参数（逐字）。
	if want[0] != "/w/run-argv.sh" {
		t.Fatalf("argv 首项应是卵清单定的可执行（/w/run-argv.sh），实得 %q", want[0])
	}
	if len(want) != len(execArgs)+1 {
		t.Fatalf("argv 长度应为 execArgs+1（%d），实得 %d：%v", len(execArgs)+1, len(want), want)
	}

	m := &Manager{procs: map[string]*subproc{}}
	m.procs["gemma-4-26B"] = &subproc{
		model: "gemma-4-26B", entry: entry, state: StateReady, port: 9400, engineArgv: want,
	}
	obs := m.EggObservations()
	if len(obs) != 1 {
		t.Fatalf("应 1 枚观测，实得 %d", len(obs))
	}
	got := obs[0].Argv
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("观测面 argv 应逐字等于真 argv：\n want=%#v\n got =%#v", want, got)
	}
	// 逐字点名：卵清单写的 `-c 131072` 必须仍在（真 argv 的核心诉求：看得到 -c 一类）。
	if !argvHasFlagValue(got, "-c", "131072") {
		t.Fatalf("真 argv 里应含 `-c 131072`（卵清单值），实得 %v", got)
	}
}

// TestEngineArgv_AbsentStaysNil 负控：本端从未为此卵记下 argv ⇒ 观测面**如实缺席**（nil），
// ✗ 不许把它当成「有、是空的」。
func TestEngineArgv_AbsentStaysNil(t *testing.T) {
	m := &Manager{procs: map[string]*subproc{}}
	m.procs["ghost"] = &subproc{model: "ghost", state: StateReady, port: 9400}
	obs := m.EggObservations()
	if len(obs) != 1 {
		t.Fatalf("应 1 枚观测，实得 %d", len(obs))
	}
	if obs[0].Argv != nil {
		t.Fatalf("未记 argv 应缺席（nil），实得 %#v", obs[0].Argv)
	}
}

// argvHasFlagValue 判 argv 里是否存在「旗标 + 紧邻的那个值」这一对（本牙自带，不借别件）。
func argvHasFlagValue(argv []string, flag, val string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == val {
			return true
		}
	}
	return false
}

// TestEngineCmd_DeclaredPassthrough 正控：卵声明的 `cmd:`（注册表逐字）经观测面（EggObservations）
// **透出** —— 这是远程机取数面的子端出口（GAP-20261006-02）：
// 「四方不一致命令面」（GAP-20260926-04）的**远程 side** 经 `/eggs` 读的正是这一格。
// 与 argv（实际执行）不同：cmd 是名册声明、可含 {file}/{port} 占位符。
func TestEngineCmd_DeclaredPassthrough(t *testing.T) {
	want := `["/w/run-cmd.sh", "-m", "{file}", "-c", "65536"]`
	entry := newNPTestEntry(want)
	m := &Manager{procs: map[string]*subproc{}}
	m.procs["gemma-4-26B"] = &subproc{model: "gemma-4-26B", entry: entry, state: StateReady, port: 9400}
	obs := m.EggObservations()
	if len(obs) != 1 {
		t.Fatalf("应 1 枚观测，实得 %d", len(obs))
	}
	if obs[0].Cmd != want {
		t.Fatalf("观测面 cmd 应逐字等于卵声明（%q），实得 %q", want, obs[0].Cmd)
	}
}

// TestEngineCmd_AbsentStaysEmpty 负控：本端从未声明 cmd:（走默认命令）⇒ 观测面**如实缺席**（空串），
// ✗ 不许把它当成「有、是空的字符串」而编出内容。
func TestEngineCmd_AbsentStaysEmpty(t *testing.T) {
	m := &Manager{procs: map[string]*subproc{}}
	m.procs["ghost"] = &subproc{model: "ghost", state: StateReady, port: 9400}
	obs := m.EggObservations()
	if len(obs) != 1 {
		t.Fatalf("应 1 枚观测，实得 %d", len(obs))
	}
	if obs[0].Cmd != "" {
		t.Fatalf("未声明 cmd: 应缺席（空串），实得 %#v", obs[0].Cmd)
	}
}

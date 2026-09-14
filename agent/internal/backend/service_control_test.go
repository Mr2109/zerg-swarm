// service_control_test.go —— P3b 的回归测试（纯函数 + 一条真机负例 N7）。
//
// 设计依据：设计-子端服务切换与基线服务声明-20260914.md §9.2（空闲检定）、§9.3（借还）、
// §11 M2（进程树孤儿）与验收 N7（停在有子孙的进程上，断言无孤儿残留）。
package backend

import (
	"os/exec"
	"reflect"
	"testing"
	"time"
)

// ── 进程树解析（纯函数）────────────────────────────────────────────────────

func TestParsePsTree_AndDescendants(t *testing.T) {
	// 真机 screen 形态：1129426(screen) → 1129428(bash) → 1129430(llama-server)
	out := "1129426 1129400\n1129428 1129426\n1129430 1129428\n999 1\n"
	tree := parsePsTree(out)
	if got := descendantsOf(tree, 1129426); !reflect.DeepEqual(got, []int{1129426, 1129428, 1129430}) {
		t.Fatalf("应取到整条链，实得 %v", got)
	}
	if got := descendantsOf(tree, 1129430); !reflect.DeepEqual(got, []int{1129430}) {
		t.Fatalf("叶子应只有自身，实得 %v", got)
	}
	if got := descendantsOf(tree, 12345); len(got) != 1 || got[0] != 12345 {
		t.Fatalf("未知 pid 应返回自身（保守），实得 %v", got)
	}
}

func TestParsePsTree_IgnoresJunk(t *testing.T) {
	tree := parsePsTree("garbage\nabc def\n123\n  \n456 789\n")
	if got := descendantsOf(tree, 456); len(got) != 1 || got[0] != 456 {
		t.Fatalf("垃圾行应被忽略，实得 %v", got)
	}
}

func TestParsePsPGID(t *testing.T) {
	m := parsePsPGID("1129426 1129426\n1129428 1129426\nbad line\n")
	if len(m) != 2 || m[1129426] != 1129426 || m[1129428] != 1129426 {
		t.Fatalf("进程组解析错误：%+v", m)
	}
}

// ── 空闲检定（§9.2；真机 payload 直接拿来当夹具）────────────────────────────

func TestParseSlotBusy(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			"真机 9000：四槽全闲（今天实测原文）",
			`[{"id":0,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":1,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":2,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":3,"n_ctx":262144,"speculative":false,"is_processing":false}]`,
			false,
		},
		{
			"真机 9001：单槽闲（今天实测原文）",
			`[{"id":0,"n_ctx":32768,"speculative":false,"is_processing":false}]`,
			false,
		},
		{"有槽在处理 ⇒ 忙", `[{"id":0,"is_processing":false},{"id":1,"is_processing":true}]`, true},
		{"带空格的 JSON 也算忙", `[{"id":0,"is_processing": true}]`, true},
		{"空响应 ⇒ 按忙（保守）", "", true},
		{"无该字段 ⇒ 按忙（保守）", `[{"id":0,"n_ctx":32768}]`, true},
		{"非法 JSON ⇒ 按忙（保守）", "not json", true},
	}
	for _, c := range cases {
		got, detail := parseSlotBusy([]byte(c.body))
		if got != c.want {
			t.Errorf("%s: busy=%v（%s），期望 %v", c.name, got, detail, c.want)
		}
		if detail == "" {
			t.Errorf("%s: 必须给出判定依据（detail）", c.name)
		}
	}
}

// ── 归还时的 env 还原（脱敏占位不注入）──────────────────────────────────────

func TestEnvFromFiltered_SkipsRedacted(t *testing.T) {
	got := envFromFiltered(map[string]string{
		"LD_LIBRARY_PATH": "/home/g01/ds4-pr670/rocm-libs",
		"PATH":            "/usr/bin:/bin",
		"ZERG_TOKEN":      "[REDACTED]",
	})
	if len(got) != 2 {
		t.Fatalf("脱敏占位不应注入，实得 %v", got)
	}
	if got[0] != "LD_LIBRARY_PATH=/home/g01/ds4-pr670/rocm-libs" || got[1] != "PATH=/usr/bin:/bin" {
		t.Fatalf("应按键名排序且原样还原，实得 %v", got)
	}
	if got := envFromFiltered(nil); got != nil {
		t.Fatalf("空映射应返回 nil，实得 %v", got)
	}
}

// ── 真机负例 N7：停在有子孙的进程上，断言无孤儿残留 ─────────────────────────

func TestStopProcessTree_NoOrphans_N7(t *testing.T) {
	if testing.Short() {
		t.Skip("真机进程用例，-short 下跳过")
	}
	// 起一棵三层树：sh → 两个 sleep 子进程
	cmd := exec.Command("/bin/sh", "-c", "sleep 120 & sleep 120 & wait")
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动测试进程（环境限制）：%v", err)
	}
	defer func() { _ = cmd.Wait() }()
	time.Sleep(700 * time.Millisecond) // 等子进程起来

	tree := processTree(cmd.Process.Pid)
	if len(tree) < 3 {
		t.Fatalf("测试进程树应至少 3 个（sh + 两个 sleep），实得 %v", tree)
	}

	killed, err := stopProcessTree(cmd.Process.Pid, 3*time.Second)
	if err != nil {
		t.Fatalf("stopProcessTree 失败：%v（killed=%v）", err, killed)
	}
	if left := aliveOf(tree); len(left) != 0 {
		t.Fatalf("N7 被破：停服后仍有孤儿残留 %v", left)
	}
}

func TestStopProcessTree_RejectsIllegalPID(t *testing.T) {
	if _, err := stopProcessTree(0, time.Second); err == nil {
		t.Fatal("pid=0 应被拒绝")
	}
	if _, err := stopProcessTree(1, time.Second); err == nil {
		t.Fatal("pid=1（init）应被拒绝")
	}
}

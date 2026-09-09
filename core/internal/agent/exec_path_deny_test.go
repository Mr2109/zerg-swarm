package agent

import (
	"os"
	"strings"
	"testing"
)

// TestValidatePathActionableDeny — FFP 式可行动拒绝(2026-09-08 报告路径沙盒冲突治本):
// 越界路径被拒时,若 ZERG_TASK_DIR 已设,消息必须报出合法报告落点——模型不再原地瞎重试
func TestValidatePathActionableDeny(t *testing.T) {
	os.Setenv("ZERG_TASK_DIR", "/tmp/zerg-tasks/task-demo")
	defer os.Unsetenv("ZERG_TASK_DIR")
	ec := &ExecContext{WorkDir: "/tmp/zerg-tasks/task-demo/work"}
	ec.ExtraAllowDirs = []string{os.Getenv("ZERG_TASK_DIR")} // 镜像 NewExecContext:S11d env 注入
	_, err := ec.validatePath("/tmp/zerg-tasks/REPORT/internal-task-report.md")
	if err == nil {
		t.Fatal("越界路径应被拒绝")
	}
	msg := err.Error()
	for _, want := range []string{
		"不在工作区", "拒绝访问",
		"报告/产物请写到任务目录(已在白名单)",
		"/tmp/zerg-tasks/task-demo/internal-task-report.md",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("拒绝消息缺 %q——got: %s", want, msg)
		}
	}
	// 白名单内路径仍应放行(任务目录自身)
	ok, err2 := ec.validatePath("/tmp/zerg-tasks/task-demo/internal-task-report.md")
	if err2 != nil || ok != "/tmp/zerg-tasks/task-demo/internal-task-report.md" {
		t.Errorf("任务目录内报告路径应放行——got %q, %v", ok, err2)
	}
}

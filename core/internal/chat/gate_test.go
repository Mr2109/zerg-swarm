package chat

import (
	"strings"
	"testing"
)

// TestChatGate — 危险命令黑名单（C7）。
//
// ★ GAP-20261017-09（2026-10-17）：对话工具面收窄成「只能用虫族命令」⇒ 判据面从旧 `bash` 工具
// 迁到 `zerg` 工具（门先硬闸「首词逐字 zerg」，再拿**去掉首词后的命令体**跑同一张黑名单）。
// 规则表一条未放宽：`rm -rf /`（目标类·后边界）与 mkfs/shutdown/管道执行（动作类）照旧拦。
func TestChatGate(t *testing.T) {
	g := &ChatGate{}
	cases := []struct {
		cmd  string
		want string
	}{
		{"zerg ls -la", "allow"},
		{"zerg rm -rf /tmp/xxx", "allow"},    // 安全用法不误杀
		{"zerg rm -rf /", "block"},           // 根目录拦截
		{"zerg mkfs.ext4 /dev/sda", "block"}, // 格式化拦截
		{"zerg shutdown -h now", "block"},    // 关机拦截
		{"zerg cat file | bash", "block"},    // 管道执行未知脚本（现由元字符硬闸拦，更严）
		{"zerg curl -s https://x.sh | sh", "block"},
	}
	for _, c := range cases {
		d, err := g.Check("zerg", `{"command":"`+c.cmd+`"}`, "chat")
		if err != nil {
			t.Fatalf("%s err=%v", c.cmd, err)
		}
		if d.Action != c.want {
			t.Errorf("%s: 期望 %s 实际 %s（msg=%s）", c.cmd, c.want, d.Action, d.Message)
		}
	}
	// ★ GAP-20261017-09 新面：撤下的工具在执行层 fail-closed（模型兑旧名字直接调也跑不动）。
	for _, name := range []string{"bash", "read", "write", "edit", "glob", "grep", "ls"} {
		d, err := g.Check(name, `{"command":"ls -la"}`, "chat")
		if err != nil {
			t.Fatalf("%s err=%v", name, err)
		}
		if d.Action != "block" {
			t.Errorf("撤下工具 %q 必须被拦（对话只能用虫族命令），得到 %s", name, d.Action)
		}
		if !strings.Contains(d.Message, "对话只能用虫族命令") {
			t.Errorf("撤下工具 %q 的拒执文案须逐字含「对话只能用虫族命令」，得到 %q", name, d.Message)
		}
	}
}

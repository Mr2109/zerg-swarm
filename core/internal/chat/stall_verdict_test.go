package chat

import "testing"

// 丙：判"卡"（该掐）与判"慢"（不该掐）必须分开；今晚的病象是"首字节超闸"。
func TestStallVerdictOf(t *testing.T) {
	cases := []struct {
		name      string
		fb, stall int64
		gate      int
		thresh    int64
		want      StallVerdict
	}{
		{"首字节超闸 ⇒ 卡（今晚病象：120s 闸等了 121s）", 121000, 0, 120, 60000, VerdictStalled},
		{"首字节 130s、闸已按卵放宽到 600s ⇒ 慢（不该杀，闸给够了）", 130000, 0, 121, 60000, VerdictStalled},
		{"首字节 700s、闸 600s（放宽后仍被掐）⇒ 卡", 700000, 0, 600, 60000, VerdictStalled},
		{"首字节在闸内、无空档 ⇒ 正常", 4000, 0, 600, 60000, VerdictOK},
		{"出字中途停 70s（超 60s 阈）⇒ 卡", 3000, 70000, 600, 60000, VerdictStalled},
		{"出字中途停 10s（未超阈）⇒ 慢（不该杀）", 3000, 10000, 600, 60000, VerdictSlow},
		{"无闸（gate=0）且无空档 ⇒ 正常", 999999, 0, 0, 60000, VerdictOK},
	}
	for _, c := range cases {
		if got := StallVerdictOf(c.fb, c.stall, c.gate, c.thresh); got != c.want {
			t.Errorf("%s：得到 %s，期望 %s", c.name, got, c.want)
		}
	}
}

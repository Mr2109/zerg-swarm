package loopcore

import (
	"strings"
	"testing"
)

// 实验开关守卫：默认关（行为不变）；开则产出"打回重做"指令。两组跑**同一份二进制** ⇒ 差异只来自开关。
func TestSendBackSwitch(t *testing.T) {
	t.Setenv("ZERG_SENDBACK", "")
	if SendBackEnabled() {
		t.Error("默认必须为关（不得改变线上行为）")
	}
	t.Setenv("ZERG_SENDBACK", "1")
	if !SendBackEnabled() {
		t.Error("=1 时应为开")
	}
	for _, v := range []string{"0", "true", "yes", " 2 "} {
		t.Setenv("ZERG_SENDBACK", v)
		if SendBackEnabled() {
			t.Errorf("只有 '1' 才算开，%q 不应为开", v)
		}
	}
}

func TestSendBackNoteActionable(t *testing.T) {
	got := SendBackNote("仍有未完成的工作")
	if !strings.Contains(got, "【打回重做】") {
		t.Errorf("必须带可识别的标记：%s", got)
	}
	if !strings.Contains(got, "回执") {
		t.Errorf("必须写明重做要求（工具回执）：%s", got)
	}
	if SendBackNote("") == "" {
		t.Error("空原因也必须有说明")
	}
}

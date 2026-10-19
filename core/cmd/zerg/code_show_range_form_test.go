// code_show_range_form_test.go —— `件:起:止` 双冒号误形的判据件。
//
// 病（实测）：本地卵多发一次 `zerg code show x.go:695:705`，LastIndex 把 `:695` 留在件名里
// ⇒ 报「件不在」这个**错归因**，它于是在同一个死循环里连烧三轮预算。
//
// 治法：认出「两个数字段」的双冒号形态 ⇒ 退码 2 ＋ 给出可照抄的正确命令（`件:起-止`）。
// 两态：① 双冒号 ⇒ rc=2、报话含「件:起-止」与纠正后的命令；② 正确形态不受影响（仍由后面那支处理）。
package main

import (
	"strings"
	"testing"
)

func TestCodeShowDoubleColonRangeNamed(t *testing.T) {
	var errb strings.Builder
	inv := &invocation{}
	_, _, rc := codeShowResolveOne(inv, &errb, "", "core/cmd/zerg/family_code.go:695:705", 3, false)

	if rc != exitUsage {
		t.Fatalf("双冒号区间应退 %d，实得 %d（stderr=%q）", exitUsage, rc, errb.String())
	}
	msg := errb.String()
	if !strings.Contains(msg, "件:起-止") {
		t.Fatalf("报话未点名正确形态 `件:起-止`：%q", msg)
	}
	if !strings.Contains(msg, "core/cmd/zerg/family_code.go:695-705") {
		t.Fatalf("报话未给出可照抄的正确命令：%q", msg)
	}
	if strings.Contains(msg, "件不在") {
		t.Fatalf("仍在报「件不在」这个错归因：%q", msg)
	}
}

func TestCodeShowSingleLineUnaffected(t *testing.T) {
	var errb strings.Builder
	inv := &invocation{}
	// 单行档不该被新那条拦下（它会往下走，因件不在而返回其它 rc，但不得是双冒号那条报话）
	_, _, _ = codeShowResolveOne(inv, &errb, "", "core/cmd/zerg/family_code.go:695", 3, false)
	if strings.Contains(errb.String(), "件:起-止") {
		t.Fatalf("单行档被误判成区间误形：%q", errb.String())
	}
}

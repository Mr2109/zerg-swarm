// check_write_inventory_tmp_test.go —— GAP-20261018-19（续）的判据件。
//
// 病（实测）：探针把 TMPDIR 指到沙箱的 tmp/，被测命令往 TMPDIR 拷源码跑（gate self-test
// 一次 17 件 tmp/cek*/core/cmd/zerg/*.go），旧判据把它们算成「默认档写盘」⇒ 同一命令
// 同一台机时绿时红（一次 0 破承诺、下一次 1 条）。
//
// 口径：沙箱 tmp/ 下的写是临时碎料，不算状态写；其余路径（home/ state/ work/ …）照旧全算。
package main

import "testing"

func TestWVIDropSandboxTmp(t *testing.T) {
	in := []string{
		"tmp/cek1361299739/core/cmd/zerg/errors.go",
		"tmp/wvi-probe-x/state/zerg.jsonl",
		"state/zerg-cli-gaps.jsonl",
		"home/.config/x",
		"work/out.txt",
	}
	got := wviDropSandboxTmp(in)
	want := []string{"state/zerg-cli-gaps.jsonl", "home/.config/x", "work/out.txt"}
	if len(got) != len(want) {
		t.Fatalf("保留 %d 条，期望 %d 条：%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 = %q，期望 %q", i, got[i], want[i])
		}
	}
	// 负控：全是 tmp/ ⇒ 必须清空（别把空输入也当「有写」）
	if len(wviDropSandboxTmp([]string{"tmp/a", "tmp/b"})) != 0 {
		t.Fatal("tmp/ 全命中时应为零")
	}
}

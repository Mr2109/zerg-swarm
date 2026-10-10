// check_swap_agent_discipline_test.go —— check_swap_agent_discipline.go（缺口 `GAP-20260926-193` · 甲案）的
// 成对正负控测试。前缀 `TestSwapAgentDiscipline`（定向跑：`go test -run SwapAgentDiscipline`）。
//
// 两态自证（本单验收）：
//
//	正控：落点（Go 半）现读正向句在 ∧ 反转不在 ⇒ 绿（rc=0）。
//	负控①：摘掉正向句 ⇒ 红（rc=1）。
//	负控②：两半皆缺（空仓根）⇒ rc=2（不给结论 · ✗ 不静默绿）。
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// csadGoodSentence —— 现读真句形态（正向在 ∧ 无反转）。
const csadGoodSentence = "  一切执行体一律 `bash <绝对路径>` 启动 ⇒ **不依赖 x 位**（真机上件可能没有 x 位；判据不声称「可直接执行」）\n"

// csadWrite —— 造合成仓根里的一件。
func csadWrite(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写件失败：%v", err)
	}
}

// TestSwapAgentDisciplineJudgeText —— 内容级判定：正控 + 两负控（摘正句 / 现反转句）。
func TestSwapAgentDisciplineJudgeText(t *testing.T) {
	if p := csadJudgeText(csadGoodSentence); len(p) != 0 {
		t.Fatalf("正控失败：正面样本被判红：%v", p)
	}
	p := csadJudgeText(strings.ReplaceAll(csadGoodSentence, "不依赖 x 位", "依赖可执行位"))
	if len(p) == 0 || !csadAnyHas(p, "缺正向纪律句") {
		t.Fatalf("负控失败：摘掉正向句未判红 / 未点名：%v", p)
	}
	p2 := csadJudgeText(csadGoodSentence + "  ⇒ **依赖 x 位**\n")
	if len(p2) == 0 || !csadAnyHas(p2, "出现反转句") {
		t.Fatalf("负控失败：现反转句未判红 / 未点名：%v", p2)
	}
}

// TestSwapAgentDisciplineReadSubject —— 落点解析：退役态读 Go 半 / 旧路读件本体 / 两半皆缺 unreadable。
func TestSwapAgentDisciplineReadSubject(t *testing.T) {
	r1 := t.TempDir()
	csadWrite(t, r1, csadGoHalfRel, csadGoodSentence)
	rel, text, st := csadReadSubject(r1)
	if st != "ok" || rel != csadGoHalfRel || text != csadGoodSentence {
		t.Fatalf("退役态应读 Go 半：rel=%q st=%q", rel, st)
	}
	r2 := t.TempDir()
	csadWrite(t, r2, csadShellRel, csadGoodSentence)
	rel2, _, st2 := csadReadSubject(r2)
	if st2 != "ok" || rel2 != csadShellRel {
		t.Fatalf("件在盘应读件本体：rel=%q st=%q", rel2, st2)
	}
	if _, _, st3 := csadReadSubject(t.TempDir()); st3 != "unreadable" {
		t.Fatalf("两半皆缺应 unreadable：得 %q", st3)
	}
}

// TestSwapAgentDisciplineGateMainStates —— 入口退码三态：绿 / 红 / 两半皆缺（rc=2）＋用法错。
func TestSwapAgentDisciplineGateMainStates(t *testing.T) {
	rOk := t.TempDir()
	csadWrite(t, rOk, csadGoHalfRel, csadGoodSentence)
	if rc := checkSwapAgentDisciplineGateMain(io.Discard, io.Discard, []string{"--root", rOk}); rc != 0 {
		t.Fatalf("正控应 rc=0，得 %d", rc)
	}
	rR := t.TempDir()
	csadWrite(t, rR, csadGoHalfRel, strings.ReplaceAll(csadGoodSentence, "不依赖 x 位", "依赖可执行位"))
	if rc := checkSwapAgentDisciplineGateMain(io.Discard, io.Discard, []string{"--root", rR}); rc != 1 {
		t.Fatalf("摘掉正句应 rc=1，得 %d", rc)
	}
	if rc := checkSwapAgentDisciplineGateMain(io.Discard, io.Discard, []string{"--root", t.TempDir()}); rc != 2 {
		t.Fatalf("两半皆缺应 rc=2（不给结论），得 %d", rc)
	}
	if rc := checkSwapAgentDisciplineGateMain(io.Discard, io.Discard, []string{"--zzz"}); rc != 2 {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
}

// TestSwapAgentDisciplineSelfTest —— 自检面（正控 + 三负控）在真仓上全过。
func TestSwapAgentDisciplineSelfTest(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	t.Setenv("ZERG_REPO_ROOT", root)
	var out bytes.Buffer
	if rc := checkSwapAgentDisciplineGateMain(&out, io.Discard, []string{"--self-test"}); rc != 0 {
		t.Fatalf("自检应 rc=0，得 %d\n%s", rc, out.String())
	}
}

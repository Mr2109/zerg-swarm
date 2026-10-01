// check_<干跑令牌>_test.go —— `check_` + 干跑令牌 + `_semantics.go` 的**同批成对负控**测试。
//
// 纪律（M2 样板）：**✗ 禁恒绿** —— 每个用例都要能单独失败（成对负控：改坏夹具 ⇒ 判红；
// 复原 ⇒ 回绿）。自指防线同正件：本测试件也落 `core/cmd/zerg/`（在门⑩ 扫描面内），
// 故正文里含令牌的串**一律拼串**，✗ 无连续字面落地。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeA（正控夹具）：一件登记表 + 两条登记件，现跑即命中且全登记 ⇒ rc=0。
func drsTreeA(t *testing.T, extra string) string {
	t.Helper()
	root := t.TempDir()
	drsMk(root, drsGoodReg(), []string{"core/a.go", "scripts/b.sh"})
	if extra != "" {
		p := filepath.Join(root, extra)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("# "+drsFlagToken+" 夹具\n"+extra+"\n"), 0o644)
	}
	return root
}

func drsRunCapture(argv ...string) (int, string, string) {
	var o, e bytes.Buffer
	rc := checkDryrunSemanticsGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

func TestCheckDryrunSemanticsGatePort(t *testing.T) {
	// ① 正控：全绿 ⇒ rc=0（转反）。
	green := drsTreeA(t, "")
	if rc, out, _ := drsRunCapture("--root", green); rc != 0 || !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("① 正控全绿：rc=%d（要 0）· out=%q", rc, out)
	}

	// ② 负控（成对·前半）：同一夹具加一件**未登记**的语义站点 ⇒ rc=1 并点名 R1。
	bad := drsTreeA(t, "agent/c.go")
	rc2, out2, _ := drsRunCapture("--root", bad)
	if rc2 != 1 {
		t.Fatalf("② 负控未登记件：rc=%d（要 1）", rc2)
	}
	if !strings.Contains(out2, "R1 未登记件（= 又一套语义）：agent/c.go") {
		t.Fatalf("② 负控未点名 R1 未登记件：out=%q", out2)
	}

	// ③ 正控（成对·后半）：把那件拿掉 ⇒ 回绿（改坏 ⇒ 判红 / 复原 ⇒ 回绿）。
	if rc, out, _ := drsRunCapture("--root", green); rc != 0 || !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("③ 成对复原：rc=%d（要 0）· out=%q", rc, out)
	}

	// ④ 空转（扫描 0 件）⇒ rc=2（空转 ≠ 绿）。
	empty := t.TempDir()
	drsMk(empty, drsGoodReg(), []string{})
	if rc, out, _ := drsRunCapture("--root", empty); rc != 2 || !strings.Contains(out, "R0 空转") {
		t.Fatalf("④ 空转：rc=%d（要 2）· out=%q", rc, out)
	}

	// ⑤ 取不到输入：登记表缺件 ⇒ rc=2（不拿「读不到」当「全绿」）。
	none := t.TempDir()
	if rc, out, _ := drsRunCapture("--root", none); rc != 2 || !strings.Contains(out, "R0 登记表不存在") {
		t.Fatalf("⑤ 登记表缺件：rc=%d（要 2）· out=%q", rc, out)
	}

	// ⑥ 魔法串面：`--json` 而本门无机器读面 ⇒ rc=2 且说明落 **stderr**。
	if rc, out, errt := drsRunCapture("--json"); rc != 2 || out != "" || !strings.Contains(errt, "本门无机器读面") {
		t.Fatalf("⑥ --json 拒收：rc=%d（要 2）· stdout=%q · stderr=%q", rc, out, errt)
	}

	// ⑦ 自检面：三码齐（1 正控 + 6 负控 + 4 分道）⇒ rc=0。
	if rc, out, _ := drsRunCapture("--self-test"); rc != 0 || !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("⑦ --self-test：rc=%d（要 0）· out=%q", rc, out)
	}
}

// check_idspace_map_test.go —— `check_idspace_map.go` 的**成对负控**单测（M2 · 路B续7 · q–z 区间）。
//
// 口径：**成对负控**（✗ 禁恒绿）—— 同一套判据，正控（好夹具）⇒ rc=0；负控（同线重号 / 跨线同名
// 未登记 / 形态越域 / 空转 / 缺件）⇒ 各自 rc=1 / rc=2 并**逐条点名**；真话负控（把负控那处补回）
// ⇒ 回绿 rc=0。**只是**去掉判据本身会 FAIL（哑化验证见交付读数 · 仓外副本）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func idmRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkIdspaceMapGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func idmWrite(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "登记件.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// idmTbl —— 与 py `_fixture` 同形的合成表。
func idmTbl(mapRows, crossRows []string) string {
	body := []string{"# 夹具", "", "## 二 · 映射表", "", idmHdrMap, "|---|---|---|---|---|---|"}
	body = append(body, mapRows...)
	body = append(body, "", "> 口径（夹具）", "", "## 三 · 同号两义 / 跨线同名登记", "",
		idmHdrCross, "|---|---|---|---|")
	body = append(body, crossRows...)
	return strings.Join(body, "\n") + "\n"
}

func TestCheckIdspaceMapGatePort(t *testing.T) {
	// ① 正控：好表 ⇒ rc=0
	good := idmWrite(t, idmTbl([]string{
		"| `Q-073` | `Q` | 件A | `Q-073` | 1 | x |",
		"| `N-06` | `N` | 件A | 无 Q 号（原记号保留 · 转实） | " + idmDupOK + " | y |",
	}, []string{}))
	if rc, out, _ := idmRun("--doc", good); rc != 0 || !strings.Contains(out, "⇒ rc=0") {
		t.Fatalf("正控失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ② 负控：同线重号（未登记形态）⇒ rc=1 且点名 K1 + 行号
	dup := idmWrite(t, idmTbl([]string{
		"| `Q-073` | `Q` | 件A | `Q-073` | 1 | x |",
		"| `Q-073` | `Q` | 件B | `Q-074` | 1 | y |",
	}, []string{}))
	if rc, out, _ := idmRun("--doc", dup); rc != 1 || !strings.Contains(out, "✗ K1 :8") {
		t.Fatalf("负控①（同线重号）失败：期望 rc=1 且点名 K1:8，实得 rc=%d out=%q", rc, out)
	}

	// ③ 真话负控（成对回绿）：把第二行写成登记形态 ⇒ rc=0
	regreen := idmWrite(t, idmTbl([]string{
		"| `Q-073` | `Q` | 件A | `Q-073` | 1 | x |",
		"| `Q-073` | `Q` | 件B | `Q-074` | " + idmDupOK + " | y |",
	}, []string{}))
	if rc, _, _ := idmRun("--doc", regreen); rc != 0 {
		t.Fatalf("成对回绿失败：期望 rc=0，实得 rc=%d", rc)
	}

	// ④ 负控：跨线同名未登记 ⇒ rc=1 且 K5 点名记号
	cross := idmWrite(t, idmTbl([]string{
		"| `G-10` | `G①` | 件A | 无 Q 号（C-02 线） | 1 | x |",
		"| `G-10` | `G②` | 件B | `Q-110` | 1 | y |",
	}, []string{}))
	if rc, out, _ := idmRun("--doc", cross); rc != 1 ||
		!strings.Contains(out, "K5 :0") || !strings.Contains(out, "G-10") {
		t.Fatalf("负控②（跨线同名）失败：期望 rc=1 且 K5 点名 G-10，实得 rc=%d out=%q", rc, out)
	}
	// ⑤ 真话负控：登记进 §三 ⇒ rc=0
	cross2 := idmWrite(t, idmTbl([]string{
		"| `G-10` | `G①` | 件A | 无 Q 号（C-02 线） | 1 | x |",
		"| `G-10` | `G②` | 件B | `Q-110` | 1 | y |",
	}, []string{"| `G-10` | 两件（`G①`／`G②`） | **两义** ✗ | x |"}))
	if rc, _, _ := idmRun("--doc", cross2); rc != 0 {
		t.Fatalf("真话负控（登记进 §三）失败：期望 rc=0，实得 rc=%d", rc)
	}

	// ⑥ 负控：形态越域 ⇒ rc=1 且 K2/K3/K4 各自点名
	form := idmWrite(t, idmTbl([]string{
		"| `q73` | `Q` | 件 | `Q-073` | 1 | x |",
		"| `Q-073` | `Q` | 件 | `Q73` | 1 | x |",
		"| `Q-074` | `Q` | 件 | `Q-074` | 2 | x |",
	}, []string{}))
	if rc, out, _ := idmRun("--doc", form); rc != 1 ||
		!strings.Contains(out, "✗ K2 :7") || !strings.Contains(out, "✗ K3 :8") || !strings.Contains(out, "✗ K4 :9") {
		t.Fatalf("负控③（形态越域）失败：期望 rc=1 且 K2:7/K3:8/K4:9，实得 rc=%d out=%q", rc, out)
	}

	// ⑦ 空转 / 缺件 ⇒ rc=2（不给结论 · 不当绿）
	if rc, out, _ := idmRun("--doc", filepath.Join(t.TempDir(), "不在盘.md")); rc != 2 ||
		!strings.Contains(out, "不给结论") {
		t.Fatalf("负控④（缺件）失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}
	noHdr := idmWrite(t, "# 坏件\n\n没有那两行表头\n")
	if rc, _, _ := idmRun("--doc", noHdr); rc != 2 {
		t.Fatalf("负控⑤（表头缺）失败：期望 rc=2，实得 rc=%d", rc)
	}
	empty := idmWrite(t, idmHdrMap+"\n|---|---|---|---|---|---|\n\n"+idmHdrCross+"\n")
	if rc, out, _ := idmRun("--doc", empty); rc != 2 || !strings.Contains(out, "空转") {
		t.Fatalf("负控⑥（空表）失败：期望 rc=2 且含「空转」，实得 rc=%d out=%q", rc, out)
	}

	// ⑧ --self-test（合成夹具 · 不碰真目标）⇒ rc=0
	if rc, out, _ := idmRun("--self-test"); rc != 0 || !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：期望 rc=0 全过，实得 rc=%d out=%q", rc, out)
	}

	// ⑨ --list-rules / -h ⇒ rc=0
	if rc, out, _ := idmRun("--list-rules"); rc != 0 ||
		!strings.Contains(out, "K5") || !strings.Contains(out, "K6") {
		t.Fatalf("--list-rules 失败：期望 rc=0 含 K5/K6，实得 rc=%d out=%q", rc, out)
	}
	if rc, out, _ := idmRun("-h"); rc != 0 || !strings.Contains(out, "usage: check-idspace-map.py") {
		t.Fatalf("-h 失败：期望 rc=0 含 usage，实得 rc=%d", rc)
	}

	// ⑩ --json 合法集 / 表外字段（Q-176）：两态在机器面上分得开
	if rc, out, _ := idmRun("--doc", good, "--json", "verdict,rows"); rc != 0 ||
		!strings.Contains(out, `"verdict": "ok"`) {
		t.Fatalf("--json 合法集失败：期望 rc=0 且 verdict=ok，实得 rc=%d out=%q", rc, out)
	}
	if rc, _, errb := idmRun("--json", "verdict,nope"); rc != 2 || !strings.Contains(errb, "不认的字段名") {
		t.Fatalf("--json 表外字段失败：期望 rc=2 且 stderr 点名，实得 rc=%d err=%q", rc, errb)
	}

	// ⑪ 真仓现读（默认真目标 · cwd 上溯根）⇒ rc=0（外部树在场才算；缺席 ⇒ 本格跳过，
	//    因为那条面属声明 (a) 的「脚本位置 ≠ cwd 上溯根」差异面）。
	if fi, err := os.Stat(idmDefaultDoc()); err == nil && fi.Mode().IsRegular() {
		if rc, out, _ := idmRun(); rc != 0 || !strings.Contains(out, "映射表 ") {
			t.Fatalf("真仓现读失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
		}
	}
}

// check_write_face_gate_test.go —— 判据件 `scripts/gates/check-write-face-gate.py` 真搬件
// （`check_write_face_gate.go` · 入口 `checkWriteFaceGateGateMain`）的**同批成对负控**测试。
//
// 覆盖（✗ 禁恒绿）：帮助面 + 自检 PASS + 自检 tempdir 自回收（A 类牙） + 各退码档（0/1/2） +
// 三条判定各自的**哑化必红**牙：
//
//	① 「落点旗标 --out ⇒ 属无闸、缺白名单」档（去掉 declaresOut 分支 ⇒ FAIL）
//	② 「无闸声明不真」档（去掉 AUTH_FLAG 检 ⇒ FAIL）
//	③ 「usage 常量引用查不到 ⇒ 判不了（✗ 不假判不存在）」档（去掉该支 ⇒ FAIL）
//	④ 「缺扫描面 / 空 catalog / 缺登记表」三档 BLOCKED（rc=2）
//
// 断言失败一律带 `t.Errorf/Fatalf` ⇒ 报文自动带「文件:行」。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// cwfgTestMk —— 造假的 `core/cmd/zerg` 扫描面（main.go + 写面件），回假仓根。
func cwfgTestMk(t *testing.T, mainSrc string, writers map[string]string) string {
	t.Helper()
	root := t.TempDir()
	d := cwaPyJoin(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwaPyJoin(d, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	for n, body := range writers {
		if err := os.WriteFile(cwaPyJoin(d, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// cwfgTestWriteReg —— 写 `authority-map.json`（授权类闭集 + 写面登记）。
func cwfgTestWriteReg(t *testing.T, root string, data any) {
	t.Helper()
	d := cwaPyJoin(root, "core", "internal", "contract")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwaPyJoin(d, "authority-map.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// cwfgTestRun —— 调入口，回 (rc, stdout, stderr)。
func cwfgTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkWriteFaceGateGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// 写面件体（★ 拼接写点原语，避免任何自扫面命中；测试件本就在扫描面外）。
func cwfgTestW(name, call string) string {
	return "func " + name + "(inv *invocation, o, e io.Writer) int {\n\t" +
		"os.WriteFile" + "(\"" + call + "\", b, 0o644)\n\treturn 0\n}\n"
}

func TestCheckWriteFaceGateGateMainHelp(t *testing.T) {
	rc, out, errb := cwfgTestRun(t, "--help")
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	if out != cwfgHelpText {
		t.Errorf("帮助正文不逐字：\n%q", out)
	}
	if errb != "" {
		t.Errorf("stderr 应空：%q", errb)
	}
}

func TestCheckWriteFaceGateGateMainSelfTest(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	rc, out, errb := cwfgTestRun(t, "--self-test")
	if rc != 0 {
		t.Fatalf("rc=%d want 0（out=%s）", rc, out)
	}
	if !strings.Contains(out, "自检 PASS（10 格）") {
		t.Errorf("自检非 PASS：\n%s", out)
	}
	if !strings.Contains(errb, "BLOCKED —— 缺 main.go / 扫描面") {
		t.Errorf("格7/9/10 的 BLOCKED 未上 stderr：\n%s", errb)
	}
	// ★ A 类牙：私有 TMPDIR 下自检结束后应 0 残留（defer os.RemoveAll 生效）。
	des, err := os.ReadDir(os.Getenv("TMPDIR"))
	if err != nil {
		t.Fatal(err)
	}
	if len(des) != 0 {
		t.Errorf("tempdir 泄漏 %d 件（自回收失效）", len(des))
	}
}

// ★ 哑化必红①：去掉「落点旗标 --out ⇒ 属无闸、缺白名单」分支 ⇒ 本用例 FAIL。
func TestCheckWriteFaceGateGateMainOutFlagUnregistered(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"w\"}, usage: \"zerg w [--out <件>]\", run: cmdW},\n"+
			"}\n",
		map[string]string{"family_w.go": cwfgTestW("cmdW", "x")})
	cwfgTestWriteReg(t, root, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}}, "写面登记": []any{}})
	rc, out, _ := cwfgTestRun(t, "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(out, "带落点旗标 --out 但无授权旗标") {
		t.Errorf("落点旗标档未点名（declaresOut 分支失效？）：\n%s", out)
	}
}

// ★ 哑化必红②：去掉「无闸声明不真」检 ⇒ 本用例 FAIL。
func TestCheckWriteFaceGateGateMainNoGateUntrue(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"g\"}, usage: \"zerg g [--yes]\", danger: &dangerSpec{...}, run: cmdG},\n"+
			"}\n",
		map[string]string{"family_g.go": cwfgTestW("cmdG", "y")})
	cwfgTestWriteReg(t, root, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}},
		"写面登记":  []any{map[string]any{"命令": "g", "授权类": "无闸", "理由": "无闸 —— 洗白", "出处": "t"}}})
	rc, out, _ := cwfgTestRun(t, "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(out, "无闸声明不真") {
		t.Errorf("无闸不真未点名（授权旗标检失效？）：\n%s", out)
	}
}

// ★ 哑化必红③：去掉「usage 常量引用查不到 ⇒ 判不了」支 ⇒ 本用例 FAIL。
func TestCheckWriteFaceGateGateMainGhostConstUndecided(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"ghost\"}, usage: ghostUsageLine, run: cmdGhost},\n"+
			"}\n",
		map[string]string{"family_ghost.go": cwfgTestW("cmdGhost", "g")})
	cwfgTestWriteReg(t, root, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}}, "写面登记": []any{}})
	rc, out, _ := cwfgTestRun(t, "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(out, "判不了") {
		t.Errorf("常量 usage 的「判不了」档缺失：\n%s", out)
	}
	if strings.Contains(out, "命令面不存在") {
		t.Errorf("误判为「命令面不存在」（缺陷①回归）：\n%s", out)
	}
}

func TestCheckWriteFaceGateGateMainGreen(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"w\"}, usage: \"zerg w [--out <件>]\", run: cmdW},\n"+
			"}\n",
		map[string]string{"family_w.go": cwfgTestW("cmdW", "x")})
	cwfgTestWriteReg(t, root, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}},
		"写面登记":  []any{map[string]any{"命令": "w", "授权类": "无闸", "理由": "无闸 —— 落点旗标", "出处": "t"}}})
	rc, out, _ := cwfgTestRun(t, "--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0（out=%s）", rc, out)
	}
}

// ★ 空格：无 authority-map.json ⇒ 「无登记」（非 BLOCKED）⇒ 写面仍判未声明（空≠缺）。
func TestCheckWriteFaceGateGateMainNoMapStillHits(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"w\"}, usage: \"zerg w [--out <件>]\", run: cmdW},\n"+
			"}\n",
		map[string]string{"family_w.go": cwfgTestW("cmdW", "x")})
	rc, out, _ := cwfgTestRun(t, "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(out, "带落点旗标 --out 但无授权旗标") {
		t.Errorf("无登记时写面未判未声明：\n%s", out)
	}
}

// ★ 空域三档：缺扫描面 / 空 catalog / 缺登记表 全 ⇒ rc=2 BLOCKED（fail-closed）。
func TestCheckWriteFaceGateGateMainBlocked(t *testing.T) {
	// 缺扫描面
	rc, out, errb := cwfgTestRun(t, "--root", t.TempDir())
	if rc != 2 {
		t.Fatalf("缺扫描面 rc=%d want 2", rc)
	}
	if out != "" {
		t.Errorf("stdout 应空：%q", out)
	}
	if !strings.Contains(errb, "BLOCKED —— 缺 main.go / 扫描面") {
		t.Errorf("stderr=%q", errb)
	}
	// 空 catalog
	root2 := cwfgTestMk(t, "package main\n\nvar cat = []*command{}\n", nil)
	cwfgTestWriteReg(t, root2, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}}, "写面登记": []any{}})
	rc2, _, errb2 := cwfgTestRun(t, "--root", root2)
	if rc2 != 2 {
		t.Fatalf("空 catalog rc=%d want 2", rc2)
	}
	if !strings.Contains(errb2, "catalog() 解析出 0 条") {
		t.Errorf("stderr=%q", errb2)
	}
	// 缺「写面登记」表
	root3 := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"w\"}, usage: \"zerg w [--out <件>]\", run: cmdW},\n"+
			"}\n",
		map[string]string{"family_w.go": cwfgTestW("cmdW", "x")})
	cwfgTestWriteReg(t, root3, map[string]any{"授权类闭集": []any{map[string]any{"值": "无闸"}}})
	rc3, _, errb3 := cwfgTestRun(t, "--root", root3)
	if rc3 != 2 {
		t.Fatalf("缺登记表 rc=%d want 2", rc3)
	}
	if !strings.Contains(errb3, "缺「写面登记」表") {
		t.Errorf("stderr=%q", errb3)
	}
}

// ★ 机器面：--json 键表齐全 + rc 内嵌一致。
func TestCheckWriteFaceGateGateMainJSONShape(t *testing.T) {
	root := cwfgTestMk(t,
		"package main\nvar cat = []*command{\n"+
			" {path: []string{\"w\"}, usage: \"zerg w [--out <件>]\", run: cmdW},\n"+
			"}\n",
		map[string]string{"family_w.go": cwfgTestW("cmdW", "x")})
	cwfgTestWriteReg(t, root, map[string]any{
		"授权类闭集": []any{map[string]any{"值": "无闸"}}, "写面登记": []any{}})
	rc, out, _ := cwfgTestRun(t, "--root", root, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("JSON 不可解析：%v（%q）", err, out)
	}
	for _, k := range []string{"rc", "registered", "registered_by_class", "unregistered", "fake_entries", "undecided"} {
		if _, ok := got[k]; !ok {
			t.Errorf("JSON 缺键 %q：%s", k, out)
		}
	}
	if got["registered"].([]any) == nil {
		t.Errorf("registered 应为数组非 null：%s", out)
	}
}

// ★ argparse 面：未知旗标 + 唯一前缀缩写。
func TestCheckWriteFaceGateGateMainArgs(t *testing.T) {
	rc, out, errb := cwfgTestRun(t, "--nope")
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if out != "" {
		t.Errorf("stdout 应空：%q", out)
	}
	if !strings.Contains(errb, "unrecognized arguments: --nope") {
		t.Errorf("stderr=%q", errb)
	}
	// 唯一前缀缩写 --r <根>（指到缺扫描面目录 ⇒ BLOCKED rc=2，证缩写被解成 --root）
	rc2, _, errb2 := cwfgTestRun(t, "--r", t.TempDir())
	if rc2 != 2 || !strings.Contains(errb2, "BLOCKED —— 缺 main.go / 扫描面") {
		t.Errorf("缩写 --r 未解成 --root（rc=%d err=%q）", rc2, errb2)
	}
	// 选项样的下一词不算值
	rc3, _, errb3 := cwfgTestRun(t, "--root", "--json")
	if rc3 != 2 || !strings.Contains(errb3, "argument --root: expected one argument") {
		t.Errorf("--root 吃选项样值（rc=%d err=%q）", rc3, errb3)
	}
}

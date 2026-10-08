// cli_write_face_gate_test.go —— 判据件 `check-write-face-gate`（门100 · Go 真搬
// `check_write_face_gate.go` · 入口 `checkWriteFaceGateGateMain`）的**成对负控**测试。
//
// 治什么（GAP-20260927-90）：门100 判「命令面写面『声明↔实现』两面判」—— 静态抽
// `core/cmd/zerg/*.go`（非测试）的**实现面**（含写调用的顶层函数）与 `main.go` catalog 的
// **声明面**（usage / danger），三条判定：
//
//	① 旗标必须声明：会写盘的命令面，usage 必须声明授权旗标（--yes/--confirm）或进 danger；
//	② 无闸必须显式白名单：无闸写面只能在 `authority-map.json`「写面登记」里逐条登记（理由须逐字含「无闸」）；
//	③ 强闸必须进危险清单：登记为 D2/D3 的命令面，catalog() 必须有 danger 档。
//
// 本件与姊妹件 `check_write_face_gate_test.go` 互补：本件每条都写成 **阳控必中 · 阴控必不中**
// 的**成对负控**（同一夹具只换一处，读数必须对调），末加一格**空转**（默认档零副作用）。
// ★ 前缀取 `TestCWFG`（本件全部 Test 名的真公共前缀；✗ 不按文件名猜、✗ 不截断）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 夹具（★ 写点原语一律拼接写 ⇒ 本件不落任何「被别的门当命中形态」的连续字面）──────────────

// cwfgCliCatalog —— 造 main.go 的 catalog 源（单命令 w · 可选 danger 档）。
func cwfgCliCatalog(usage string, withDanger bool) string {
	danger := ""
	if withDanger {
		danger = "danger: &dangerSpec{...}, "
	}
	return "package main\nvar cat = []*command{\n" +
		" {path: []string{\"w\"}, usage: \"" + usage + "\", " + danger + "run: cmdW},\n}\n"
}

// cwfgCliWriter —— 写面件体：cmdW 含写调用（★ 写点原语拼接写）。
func cwfgCliWriter() string {
	return "func cmdW(inv *invocation, o, e io.Writer) int {\n\t" +
		"os.WriteFile" + "(\"x\", b, 0o644)\n\treturn 0\n}\n"
}

// cwfgCliMk —— 造 `core/cmd/zerg` 假扫描面：main.go + family_w.go 写面件；回假仓根。
func cwfgCliMk(t *testing.T, mainSrc, writerBody string) string {
	t.Helper()
	root := t.TempDir()
	d := cwaPyJoin(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwaPyJoin(d, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwaPyJoin(d, "family_w.go"), []byte(writerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// cwfgCliRegEntry —— 造一条写面登记项。
func cwfgCliRegEntry(cmd, cls, reason string) map[string]any {
	return map[string]any{"命令": cmd, "授权类": cls, "理由": reason, "出处": "负控夹具"}
}

// cwfgCliReg —— 写 `authority-map.json`（授权类闭集三值 + 写面登记；nil ⇒ 空表）。
func cwfgCliReg(t *testing.T, root string, entries []any) {
	t.Helper()
	d := cwaPyJoin(root, "core", "internal", "contract")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if entries == nil {
		entries = []any{}
	}
	data := map[string]any{
		"授权类闭集": []any{map[string]any{"值": "D2"}, map[string]any{"值": "D3"}, map[string]any{"值": "无闸"}},
		"写面登记":  entries,
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwaPyJoin(d, "authority-map.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// cwfgCliRun —— 调入口（走整条管道：扫面 → catalog → 登记面验证 → 两面判）。
func cwfgCliRun(t *testing.T, root string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	rc := checkWriteFaceGateGateMain(&so, &se, []string{"--root", root})
	return rc, so.String(), se.String()
}

// cwfgCliTree —— 夹具树「相对路径 → 内容」快照（判默认档是否零副作用）。
func cwfgCliTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// cwfgCliSameTree —— 两快照逐件逐字相同？
func cwfgCliSameTree(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// ── 成对负控① 旗标必须声明：会写盘而 usage 未声明授权旗标且无 danger ⇒ 逐条点名 ──────────────
func TestCWFGFlagMustBeDeclared(t *testing.T) {
	// 阳控：usage 无授权旗标、命令面无 danger ⇒ rc=1 且点名「未声明授权旗标」。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w --dry-run", false), cwfgCliWriter())
	cwfgCliReg(t, root, nil)
	rc, out, errs := cwfgCliRun(t, root)
	if rc != 1 || errs != "" {
		t.Fatalf("阳控 rc=%d err=%q want rc=1", rc, errs)
	}
	if !strings.Contains(out, "写面但 usage 未声明授权旗标") {
		t.Fatalf("阳控未点名「未声明授权旗标」：\n%s", out)
	}
	// 阴控：同一夹具只把 usage 换成声明 --yes ⇒ rc=0 且未声明计数=0。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes]", false), cwfgCliWriter())
	cwfgCliReg(t, root2, nil)
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 {
		t.Fatalf("阴控 rc=%d want rc=0（--yes 已声明授权旗标）：\n%s", rc2, out2)
	}
	if !strings.Contains(out2, "未声明/未白名单 = 0 条") {
		t.Fatalf("阴控仍有未声明命中：\n%s", out2)
	}
}

// ── 成对负控② 落点旗标 --out 不算闸：单独 --out ⇒ 仍判「无闸」缺白名单 ──────────────────
func TestCWFGOutFlagIsNotAGate(t *testing.T) {
	// 阳控：usage 只有落点旗标 --out ⇒ rc=1 且点名「带落点旗标 --out」。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root, nil)
	rc, out, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	if !strings.Contains(out, "带落点旗标 --out 但无授权旗标") {
		t.Fatalf("阳控未点名落点旗标档：\n%s", out)
	}
	// 阴控：同一夹具加上授权旗标 --yes ⇒ rc=0（证明 --out 之差真来自「授权旗标才是闸」）。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes --out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root2, nil)
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 {
		t.Fatalf("阴控 rc=%d want rc=0：\n%s", rc2, out2)
	}
}

// ── 成对负控③ danger 命令面仍须声明授权旗标：有 danger、usage 无授权旗标 ⇒ 命中 ─────────────
func TestCWFGDangerStillNeedsFlag(t *testing.T) {
	// 阳控：命令面有 danger 档，usage 无授权旗标 ⇒ rc=1 且点名「已登记 danger 但 usage 未声明授权旗标」。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w --dry-run", true), cwfgCliWriter())
	cwfgCliReg(t, root, nil)
	rc, out, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	if !strings.Contains(out, "已登记 danger 但 usage 未声明授权旗标") {
		t.Fatalf("阳控未点名 danger 缺旗标：\n%s", out)
	}
	// 阴控：同一夹具 usage 补上 --yes ⇒ rc=0。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes]", true), cwfgCliWriter())
	cwfgCliReg(t, root2, nil)
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 {
		t.Fatalf("阴控 rc=%d want rc=0：\n%s", rc2, out2)
	}
}

// ── 成对负控④ 无闸必须显式白名单：登记为「无闸」（理由含「无闸」）⇒ 免判；不登记 ⇒ 命中 ─────────
func TestCWFGNoGateNeedsExplicitWhitelist(t *testing.T) {
	// 阳控：无闸写面（只有 --out）未登记 ⇒ rc=1 命中。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root, nil)
	rc, _, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	// 阴控：同一夹具逐条登记为「无闸」⇒ rc=0 且进已登记面。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root2, []any{cwfgCliRegEntry("w", "无闸", "无闸 —— 仅落点旗标，无授权旗标")})
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 {
		t.Fatalf("阴控 rc=%d want rc=0（已显式登记无闸）：\n%s", rc2, out2)
	}
	if !strings.Contains(out2, "  · [无闸] w") {
		t.Fatalf("阴控未体现已登记写面：\n%s", out2)
	}
}

// ── 成对负控⑤ 无闸声明不真：usage 已声明授权旗标却洗成「无闸」⇒ 假条目 ─────────────────────
func TestCWFGNoGateUntrue(t *testing.T) {
	// 阳控：usage 含 --yes，登记却写「无闸」⇒ rc=1 且点名「无闸声明不真」。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes]", false), cwfgCliWriter())
	cwfgCliReg(t, root, []any{cwfgCliRegEntry("w", "无闸", "无闸 —— 洗白试试")})
	rc, out, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	if !strings.Contains(out, "无闸声明不真") {
		t.Fatalf("阳控未点名「无闸声明不真」：\n%s", out)
	}
	// 阴控：usage 只声明落点旗标 --out（真无闸）⇒ 同一条登记不判假 ⇒ rc=0。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root2, []any{cwfgCliRegEntry("w", "无闸", "无闸 —— 仅落点旗标")})
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 || strings.Contains(out2, "无闸声明不真") {
		t.Fatalf("阴控 rc=%d 不应判「无闸不真」：\n%s", rc2, out2)
	}
}

// ── 成对负控⑥ 强闸必须进危险清单：登记 D2/D3 而 catalog 无 danger 档 ⇒ 假条目 ─────────────────
func TestCWFGStrongGateNeedsDanger(t *testing.T) {
	// 阳控：登记 D2、catalog 无 danger ⇒ rc=1 且点名「D2 声明不真：catalog() 无 danger 档」。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes]", false), cwfgCliWriter())
	cwfgCliReg(t, root, []any{cwfgCliRegEntry("w", "D2", "强闸（危险档）")})
	rc, out, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	if !strings.Contains(out, "D2 声明不真：catalog() 无 danger 档") {
		t.Fatalf("阳控未点名 D2 缺 danger：\n%s", out)
	}
	// 阴控：同一夹具给命令面补 danger 档 ⇒ D2 登记成立 ⇒ rc=0。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--yes]", true), cwfgCliWriter())
	cwfgCliReg(t, root2, []any{cwfgCliRegEntry("w", "D2", "强闸（危险档）")})
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 || strings.Contains(out2, "声明不真") {
		t.Fatalf("阴控 rc=%d 不应判声明不真：\n%s", rc2, out2)
	}
}

// ── 成对负控⑦ 登记命令必须真实存在：瞎写命令 ⇒ 假条目（且不能替真写面免判）───────────────────
func TestCWFGRegistryCommandMustExist(t *testing.T) {
	// 阳控：只登记一条瞎写命令 ⇒ rc=1 且点名「命令面不存在」，真写面 w 仍命中。
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root, []any{cwfgCliRegEntry("ghost no-cmd", "无闸", "无闸 —— 瞎写")})
	rc, out, _ := cwfgCliRun(t, root)
	if rc != 1 {
		t.Fatalf("阳控 rc=%d want rc=1", rc)
	}
	if !strings.Contains(out, "命令面不存在") {
		t.Fatalf("阳控未点名「命令面不存在」：\n%s", out)
	}
	if !strings.Contains(out, "带落点旗标 --out 但无授权旗标") {
		t.Fatalf("阳控：瞎写条目不应替真写面 w 免判：\n%s", out)
	}
	// 阴控：同一夹具把登记改成真实的 w ⇒ rc=0。
	root2 := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root2, []any{cwfgCliRegEntry("w", "无闸", "无闸 —— 真实命令")})
	rc2, out2, _ := cwfgCliRun(t, root2)
	if rc2 != 0 || strings.Contains(out2, "命令面不存在") {
		t.Fatalf("阴控 rc=%d：\n%s", rc2, out2)
	}
}

// ── 空转（零副作用）：默认档对夹具树逐字节不动 + 两次跑三面读数逐字相同 ──────────────────────
func TestCWFGNoOpZeroSideEffect(t *testing.T) {
	root := cwfgCliMk(t, cwfgCliCatalog("zerg w [--out <件>]", false), cwfgCliWriter())
	cwfgCliReg(t, root, []any{cwfgCliRegEntry("w", "无闸", "无闸 —— 仅落点旗标")})
	before := cwfgCliTree(t, root)

	rc1, out1, err1 := cwfgCliRun(t, root)
	after := cwfgCliTree(t, root)
	if !cwfgCliSameTree(before, after) {
		t.Fatalf("默认档改动了夹具树（非空转）：\n before=%v\n after=%v", before, after)
	}

	// 确定性：同一夹具再跑一次，rc 与三面输出逐字相同。
	rc2, out2, err2 := cwfgCliRun(t, root)
	if rc1 != rc2 || out1 != out2 || err1 != err2 {
		t.Fatalf("两次跑读数不一致（rc %d/%d）", rc1, rc2)
	}
	if rc1 != 0 || out1 == "" {
		t.Fatalf("空转档读数异常：rc=%d out=%q", rc1, out1)
	}
}

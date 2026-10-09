// cli_dev_edit_approval_test.go —— `zerg dev edit` 的**人签批准件**闸（主体③-a · 2026-09-21）。
//
// 本件判据四条（**全部在合成仓根 + 合成状态目录里跑**，不碰真仓的件）：
//
//	① 真写**没有批准件** ⇒ 退 2，且**件一个字节都没动**（fail-closed 的那一格）；
//	② **手写件**（字段齐、没签名）⇒ 退 2（「无签名（不算批准）」逐字给出来）；
//	③ 真签名件但**范围不含**本次改件 ⇒ 退 2（范围只认 `*` / 逐字同路径 / `目录/` 前缀）；
//	④ 真签名件 + 范围对上 ⇒ 退 0、件按 `--from` 写成、审计里能回读到**批准人与批准件路径**。
//
// 判据的四枚钥匙都由本件自己造（Ed25519 + `control.ApprovalPayload`/`KeyID`/`VerifyApprovalSignature`
// 是**消费者同一条路**）：私钥不留盘、公钥落 `operator.pub` —— 与 `approve keygen` 的产物同格式。
//
// 判据⑤（2026-09-22 本枚 · **真事故级**）：**写入前语法自检** —— `.py` 改完解析不了 ⇒ 拒写
// （退 2 · 件一个字节不动 · 连审计都不落）；同一件改合法 ⇒ 退 0 且写上盘（成对 = 有区分度）。
package main_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// hostnameOf 本机名（`dev edit` 的 `--confirm` 值必须与它逐字相同 —— 与 `planHost()` 同源）。
func hostnameOf(t *testing.T) string {
	t.Helper()
	h, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// synthRepoForEdit 建一个合成仓根（判据件 + 一件待改的件），返回 (仓根, 目标件相对路径)。
func synthRepoForEdit(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	target := filepath.Join("core", "cmd", "zerg", "target.go")
	mustWrite(t, filepath.Join(root, target), "package main\n\n// 原件\n")
	return root, target
}

// 合成提案件（声明它要改的那一件）——`dev edit` 的作用域真源。
func writeEditProposal(t *testing.T, dir, id, target string) {
	t.Helper()
	rec := map[string]any{
		"id": id, "title": "合成提案（人签批准件的判据夹具）", "target": "D3b",
		"goal": "g", "evidence": []string{"e"}, "rollback_ref": "r",
		"criterion": "zerg gate run --fast", "files": []string{target},
		"by": "夹具", "subject": "", "subject_kind": "human", "state": "未决",
		"created_at": "2026-09-21T00:00:00+08:00",
	}
	b, _ := json.MarshalIndent(rec, "", " ")
	mustWrite(t, filepath.Join(dir, id+".json"), string(b)+"\n")
}

// writeOperatorKey 造一枚操作员密钥对（私钥只在本函数里活着）；返回签发函数。
func writeOperatorKey(t *testing.T, state string) func(tool, scope, approver, at, note string) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	pf := map[string]string{"alg": "ed25519", "pub": pubB64, "key_id": control.KeyID(pubB64), "at": "2026-09-21T00:00:00+08:00"}
	b, _ := json.MarshalIndent(pf, "", " ")
	mustWrite(t, filepath.Join(state, "approvals", "operator.pub"), string(b)+"\n")
	return func(tool, scope, approver, at, note string) []byte {
		tk := map[string]string{
			"tool": tool, "approver": approver, "approved_at": at, "scope": scope, "note": note,
			"sig_alg": "ed25519", "key_id": control.KeyID(pubB64),
			"sig": base64.StdEncoding.EncodeToString(
				ed25519.Sign(priv, control.ApprovalPayload(tool, scope, approver, at, note))),
		}
		out, _ := json.MarshalIndent(tk, "", " ")
		return append(out, '\n')
	}
}

// runDevEdit 进程内跑一条 `dev edit`（env 全部隔离到合成根/合成状态目录）。
func runDevEdit(t *testing.T, repo, prop, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_PROPOSAL_DIR", prop)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(append([]string{"dev", "edit"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

func TestDevEdit_RealWriteNeedsHumanApproval(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9001", target)
	sign := writeOperatorKey(t, state) // 钥匙在，但**没签任何件**
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 被改后的内容\n")

	// ① 无件 ⇒ 拒
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 {
		t.Fatalf("真写没有批准件 ⇒ 退 2（fail-closed），得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "真写要一枚人签批准件") || !strings.Contains(errb, "approve new --tool dev_edit") {
		t.Errorf("要给拒因 + 下一步（怎么签那枚件）：%q", errb)
	}
	got, _ := os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "// 原件") {
		t.Errorf("被拒的那一格里件**一个字节都不许动**：%q", got)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("被拒 ⇒ 连审计都不该落（没写就不记账）")
	}

	// ② 手写件（字段齐、无签名）⇒ 拒
	hand := map[string]string{"tool": "dev_edit", "approver": "手写", "approved_at": "2026-09-21T00:00:00+08:00",
		"scope": "*", "note": "手写的"}
	hb, _ := json.MarshalIndent(hand, "", " ")
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"), string(hb)+"\n")
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 || !strings.Contains(errb, "无签名（不算批准）") {
		t.Errorf("手写件 ⇒ 拒且逐字给出「无签名（不算批准）」：rc=%d stderr=%q", rc, errb)
	}

	// ③ 真签名件但范围不含本次改件 ⇒ 拒
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "core/别的目录/", "张三", "2026-09-21T07:00:00+08:00", "批的是别处"))+"\n")
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 || !strings.Contains(errb, "不含本次改的件") {
		t.Errorf("范围不符 ⇒ 拒并点名范围：rc=%d stderr=%q", rc, errb)
	}

	// ④ 真签名件 + 范围对上 ⇒ 真写；审计里能回读到批准人与批准件路径
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-09-21T07:00:00+08:00", "放行这一件"))+"\n")
	rc, out, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9001", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("人签齐了 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	got, _ = os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "被改后的内容") {
		t.Errorf("人签齐了就要真写：%q", got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("审计件不在：%v", err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ab))), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v · %q", err, ab)
	}
	if line["approver"] != "张三" || !strings.HasSuffix(line["approval"].(string), "approvals/dev_edit.json") {
		t.Errorf("审计要能回读「谁让它改的」：%+v", line)
	}
}

// synthRepoForEditPy —— 同上，但目标件是 **.py**（语法自检那一格的夹具）。
func synthRepoForEditPy(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	target := filepath.Join("core", "tools", "target.py")
	mustWrite(t, filepath.Join(root, target), "def f():\n    return 1\n")
	return root, target
}

// TestDevEdit_SyntaxGateRefusesUnparsablePy —— 判据⑤：写盘前语法自检（`.py` ⇒ `ast.parse`）。
//
// 病征（真事故）：一次 `--replace` 漏闭括号，命令面**照写** —— 写上盘的是一件当场 SyntaxError 的件
// （改前只有「回读 sha256 对拍」，那对得上恰恰证明**写坏了也照过**）。本用例逐格钉住三态：
//
//	① 确认档齐 + 人签批准件齐、内容漏闭括号 ⇒ 退 2，目标件一个字节不动、连审计都不落；
//	② 同一件同一提案、内容合法 ⇒ 退 0 且真写上盘（成对：证明 ① 不是「一律拒」）；
//	③ 干跑 ⇒ 退 0 且计划面**先打出同一份判决**（「真写会被拒」），零副作用。
func TestDevEdit_SyntaxGateRefusesUnparsablePy(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("哨兵失守：本机没有 python3（✗ 不留 Skip · fail-closed）：%v", err)
	}
	repo, target := synthRepoForEditPy(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9003", target)
	sign := writeOperatorKey(t, state)
	bad := filepath.Join(t.TempDir(), "bad.py")
	mustWrite(t, bad, "def f(:\n    return 1\n") // 漏闭括号 —— 真事故那一枚
	good := filepath.Join(t.TempDir(), "good.py")
	mustWrite(t, good, "def f():\n    return 2\n")

	// ① 语法不过 ⇒ 拒写
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-09-22T00:00:00+08:00", "放行这一件"))+"\n")
	rc, _, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9003", "--file", target,
		"--from", bad, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 {
		t.Fatalf("语法不过 ⇒ 退 2（fail-closed），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "语法自检不过") {
		t.Errorf("拒因要点名「语法自检不过」：%q", errb)
	}
	if !strings.Contains(errb, "SyntaxError") {
		t.Errorf("拒因要带上检查器原文（`SyntaxError`）：%q", errb)
	}
	got, _ := os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "return 1") || strings.Contains(string(got), "def f(:") {
		t.Errorf("被拒的那一格里件**一个字节都不许动**：%q", got)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("被拒 ⇒ 连审计都不该落（没写就不记账）")
	}

	// ② 内容合法 ⇒ 真写上盘（成对）
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9003", "--file", target,
		"--from", good, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("内容合法 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	got, _ = os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "return 2") {
		t.Errorf("合法内容要真写上盘：%q", got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	if !strings.Contains(string(ab), "\"syntax\":\"过\"") {
		t.Errorf("审计要能回读语法判决：%q", ab)
	}

	// ③ 干跑：计划面先给判决（零副作用）
	rc, out, _ := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9003", "--file", target,
		"--from", bad, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ 退 0（零副作用），得到 %d", rc)
	}
	if !strings.Contains(out, "语法自检") || !strings.Contains(out, "真写会被拒") {
		t.Errorf("计划面要打出语法判决并点明「真写会被拒」：%q", out)
	}
}

// 干跑**不需要**批准件（零副作用那一档），但计划面要照实打出批准件的判决。
func TestDevEdit_DryRunNeedsNoApprovalButShowsIt(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9002", target)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 干跑不该落地\n")

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9002", "--file", target,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ 退 0（不需要批准件），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "批准件") || !strings.Contains(out, "无件（真写要一枚人签批准件）") {
		t.Errorf("计划面要打出批准件的判决（先看后签）：%q", out)
	}
	got, _ := os.ReadFile(filepath.Join(repo, target))
	if !strings.Contains(string(got), "// 原件") {
		t.Errorf("干跑零副作用：%q", got)
	}
}

// synthRepoWithSibling —— 建「主仓 + 同级文仓」合成树：base/repo（主仓）+ base/Zerg-内部文档（同级文仓）。
// 返回 (主仓根, 同级仓根, 同级仓内相对路径)。**不碰真 Zerg-内部文档**（隔离纪律同 devdocs_test.go）。
func synthRepoWithSibling(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	sib := filepath.Join(base, "Zerg-内部文档")
	mustWrite(t, filepath.Join(repo, "core", "internal", "version", "version.go"), "package version\n")
	rel := filepath.Join("项目文档", "合成清单.md")
	mustWrite(t, filepath.Join(sib, rel), "改前\n")
	return repo, sib, rel
}

// TestDevEdit_SiblingRepoFlagGate —— `GAP-20261001-07` 甲案：`--allow-sibling-repo <名>`。
//
// 成对负控（✗ 禁恒绿 —— 每一条都有相反的那一格）：
//
//	① 不带旗标 + `--file ../Zerg-内部文档/…` ⇒ rc=2 · **前闸逐字**（`bad_file_path` 那一行）；
//	② 不带旗标 + `--file Zerg-内部文档/…`（无 `..`）⇒ **静默改道**：件落在仓内假路径
//	   〈仓根〉/Zerg-内部文档/… 而**真同级仓那一件一个字节不动**（病灶直证）；
//	③ 带旗标 + 闭集名 + 干跑 ⇒ rc=0 且零副作用；
//	④ 旗标值不在闭集（路径形态 / 大小写别名 / 陌生名 / 空名）⇒ 一律 rc=2 · 逐字；
//	⑤ 闭集名但同级仓根不在盘 ⇒ rc=8（不猜）；
//	⑥ 带旗标真写（人签齐）⇒ rc=0 · 件落在**真同级仓**里 + 审计行带 `sibling_repo` / `sibling_root`。
func TestDevEdit_SiblingRepoFlagGate(t *testing.T) {
	repo, sib, rel := synthRepoWithSibling(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9101", rel)                             // 声明「同级仓内相对路径」
	writeEditProposal(t, prop, "DEV-9102", filepath.Join("Zerg-内部文档", rel)) // 声明「无 `..` 的改道路径」
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "*", "张三", "2026-10-01T00:00:00+08:00", "放行同级文仓"))+"\n")
	from := filepath.Join(t.TempDir(), "new.md")
	mustWrite(t, from, "改后\n")

	// ① 默认态（不带旗标）+ `..` ⇒ rc=2 · 前闸逐字不变
	rc, _, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9101", "--file",
		filepath.Join("..", "Zerg-内部文档", rel), "--from", from, "--dry-run")
	if rc != 2 || !strings.Contains(errb, "里有 `..` —— 不许越出仓根") {
		t.Fatalf("默认态 `..` ⇒ rc=2 · 逐字前闸：rc=%d stderr=%q", rc, errb)
	}

	// ② 默认态 + 无 `..` 的同级仓相对路径 ⇒ **静默改道**（主闸也放行）
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9102", "--file",
		filepath.Join("Zerg-内部文档", rel), "--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("默认态无 `..` ⇒ 主闸放行（改道），rc=%d stderr=%q", rc, errb)
	}
	if _, err := os.Stat(filepath.Join(repo, "Zerg-内部文档", rel)); err != nil {
		t.Errorf("改道的牙口：件应落在**仓内假路径** %s：%v", filepath.Join(repo, "Zerg-内部文档", rel), err)
	}
	if got, _ := os.ReadFile(filepath.Join(sib, rel)); string(got) != "改前\n" {
		t.Errorf("默认态**到不了**真同级仓 ⇒ 同级仓件一个字节不许动：%q", got)
	}

	// ③ 带旗标 + 闭集名 + 干跑 ⇒ rc=0（零副作用）
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9101", "--file", rel,
		"--from", from, "--dry-run", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 0 {
		t.Fatalf("带旗标干跑 ⇒ rc=0：rc=%d stderr=%q", rc, errb)
	}
	if got, _ := os.ReadFile(filepath.Join(sib, rel)); string(got) != "改前\n" {
		t.Errorf("干跑零副作用：%q", got)
	}

	// ④ 旗标值不在闭集 ⇒ 一律 rc=2 · 逐字
	for _, bad := range []string{filepath.Join("..", "Zerg-内部文档"), "/etc", "ZergDocs", "foo", ""} {
		rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9101", "--file", rel,
			"--from", from, "--dry-run", "--allow-sibling-repo", bad)
		if rc != 2 || !strings.Contains(errb, "只收闭集里的名") {
			t.Errorf("旗标值 %q 不在闭集 ⇒ rc=2 · 逐字：rc=%d stderr=%q", bad, rc, errb)
		}
	}

	// ⑤ 闭集名但同级仓根不在盘 ⇒ rc=8（不给结论）
	repo2 := t.TempDir()
	mustWrite(t, filepath.Join(repo2, "core", "internal", "version", "version.go"), "package version\n")
	rc, _, errb = runDevEdit(t, repo2, prop, state, "--proposal", "DEV-9101", "--file", rel,
		"--from", from, "--dry-run", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 8 || !strings.Contains(errb, "不在盘上") {
		t.Errorf("同级仓根不在盘 ⇒ rc=8 · 逐字：rc=%d stderr=%q", rc, errb)
	}

	// ⑥ 带旗标真写 ⇒ rc=0 · 件落在**真同级仓**里 + 审计行带两格
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9101", "--file", rel,
		"--from", from, "--confirm="+hostnameOf(t), "--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 0 {
		t.Fatalf("带旗标真写 ⇒ rc=0：rc=%d stderr=%q", rc, errb)
	}
	if got, _ := os.ReadFile(filepath.Join(sib, rel)); string(got) != "改后\n" {
		t.Errorf("真写要落在同级仓里：%q", got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	all := strings.Split(strings.TrimSpace(string(ab)), "\n")
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(all[len(all)-1])), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if line["sibling_repo"] != "Zerg-内部文档" || line["sibling_root"] != sib {
		t.Errorf("审计要回读「同级仓名 + 它的根」：%+v（想要 repo=Zerg-内部文档 root=%s）", line, sib)
	}
}

// TestDevEdit_DryRunRefusesOutsideApprovalScope —— 判据⑥（`GAP-20260927-441` · 2026-10-04 本枚）：
// **干跑不许对「批准面外」假绿** —— 批准件在盘、真签名、工具名对，但 `scope` 不含本次要改的件时：
//
//	① 干跑**退 2**（把「不算批准」从括号提示升级为**可判定的退码** · 与「钩子必拦」同档 · 不新立码）；
//	② 计划面（stdout）与 stderr 都**明写**「本件不在批准面」；
//	③ 件一个字节不动、审计不落（零副作用一字不动 —— 变的只是判决不再是放行）。
//
// 成对负控（✗ 禁恒绿）：同一件把 `scope` 改成覆盖本件 ⇒ 干跑仍退 0（有区分度）。
// ★「无件」那一档**不在此列**（「先看后签」的正路）—— 由 `TestDevEdit_DryRunNeedsNoApprovalButShowsIt` 钉住。
func TestDevEdit_DryRunRefusesOutsideApprovalScope(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9004", target)
	sign := writeOperatorKey(t, state)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 批准面外：干跑不许假绿\n")

	// 批准件：真签名、工具名 dev_edit、但 scope 只批「另一个目录」⇒ 不含 target（批准面外那一档）。
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "core/别的目录/", "张三", "2026-10-04T00:00:00+08:00", "批的是别处"))+"\n")
	before, _ := os.ReadFile(filepath.Join(repo, target))

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9004", "--file", target,
		"--from", from, "--dry-run")
	if rc != 2 {
		t.Fatalf("批准面外 ⇒ 干跑退 2（可判定的退码），得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "不在批准面") || !strings.Contains(errb, "不在批准面") {
		t.Errorf("计划面与 stderr 都要明写「本件不在批准面」：stdout=%q stderr=%q", out, errb)
	}
	if after, _ := os.ReadFile(filepath.Join(repo, target)); string(after) != string(before) {
		t.Errorf("干跑零副作用：件一个字节不许动：%q", after)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("干跑不许落审计（没写就不记账）")
	}

	// 成对负控：scope 覆盖本件 ⇒ 干跑退 0（证明 ① 不是「一律拒」）。
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-04T00:00:00+08:00", "放行这一件"))+"\n")
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9004", "--file", target,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("批准面覆盖本件 ⇒ 干跑退 0，得到 %d · stderr=%s", rc, errb)
	}
}

// TestDevEdit_ApprovalFaceSelfCheckGivesHowto —— ★ GAP-20260928-105（本枚）：批准面的**只读自查口**。
// 病灶（缺口逐字）：没有任何 CLI 面说明批准件从哪来 / 怎么续期 / scope 闭集怎么扩
// ⇒ 换机器 / 新会话撞退 2 时使用者**无从自查**。本判据钉两处（**同一块 · 两处同源**）：
//
//	① 干跑计划面（只读 · 零副作用）出「从哪来 / 续期扩scope / scope闭集 / 自查」四行；
//	② 真写拒执面（无件那一档）也出同一块 + 逐字拒因。
func TestDevEdit_ApprovalFaceSelfCheckGivesHowto(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9005", target)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 自查面\n")

	// ① 干跑（只读）：无件也在计划面出自己的批准面说明块。
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9005", "--file", target,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	for _, want := range []string{"批准面·从哪来", "续期/扩scope", "scope闭集", "批准面·自查", "approve new --tool dev_edit"} {
		if !strings.Contains(out, want) {
			t.Errorf("干跑计划面缺批准面说明 %q：%q", want, out)
		}
	}

	// ② 真写拒执（无批准件）：同一块也出（两处同源）。
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9005", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 {
		t.Fatalf("无批准件真写 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "真写要一枚人签批准件") || !strings.Contains(errb, "批准面·从哪来") {
		t.Errorf("拒执面要给拒因 + 批准面说明块：%q", errb)
	}
}

// TestDevEdit_ReplaceOnAlreadyLandedDisambiguates —— ★ GAP-20261009-10：
//
//	① 已改盘件（改前文本已不在盘上）跑 `--replace` ⇒ rc=2 且 stderr **消歧**
//	   （明写「已落盘 / 回放不了 / 替补路线：还原前态后用 --from」），替掉误导性的「改成更长的上下文」；
//	② 替补路线（还原**真前态** + `--from <改后整件>`）⇒ rc=0 且审计 before≠after（真落地 · 留真指纹）。
func TestDevEdit_ReplaceOnAlreadyLandedDisambiguates(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9009", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", "*", "张三", "2026-10-09T00:00:00+08:00", "放行本件"))+"\n")

	// 造「已改盘件」：真前态 = synth 初值，现盘 = 手改后的内容。
	baseline := "package main\n\n// 原件\n"
	after := "package main\n\n// 改后\n"
	mustWrite(t, filepath.Join(repo, target), after)
	pb, _ := json.Marshal([]map[string]string{{"old": "// 原件\n", "new": "// 改后\n"}})
	patch := filepath.Join(t.TempDir(), "p.json")
	mustWrite(t, patch, string(pb))

	// ① 对已改盘件 --replace ⇒ rc=2 + 消歧
	rc, _, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9009", "--file", target,
		"--replace", patch, "--dry-run")
	if rc != 2 {
		t.Fatalf("已改盘件 --replace ⇒ rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "回放不了") || !strings.Contains(errb, "替补路线") {
		t.Errorf("stderr 要消歧（回放不了 / 替补路线）：%q", errb)
	}
	if strings.Contains(errb, "改成更长的上下文") {
		t.Errorf("已落盘那一档不许再用误导性的「改成更长的上下文」：%q", errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != after {
		t.Errorf("拒执 ⇒ 件一个字节不许动：%q", got)
	}

	// ② 替补路线：还原真前态 + --from <改后整件> ⇒ rc=0 且审计 before≠after
	mustWrite(t, filepath.Join(repo, target), baseline)
	from := filepath.Join(t.TempDir(), "after.go")
	mustWrite(t, from, after)
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9009", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("替补路线（还原前态 + --from）⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); string(got) != after {
		t.Errorf("替补路线要把件写成改后整件：%q", got)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	all := strings.Split(strings.TrimSpace(string(ab)), "\n")
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(all[len(all)-1])), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if line["before_sha256"] == line["after_sha256"] {
		t.Errorf("审计要留**真** before→after（真落地）：before=%v after=%v", line["before_sha256"], line["after_sha256"])
	}
}

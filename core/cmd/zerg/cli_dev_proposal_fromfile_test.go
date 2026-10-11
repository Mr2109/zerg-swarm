// cli_dev_proposal_fromfile_test.go —— `dev proposal new --from-file <件>`（单片档可机读声明件）的机检。
// 缺口 GAP-20260928-65：`dev proposal new` 的自由文本字段（--evidence/--goal/--rollback/--title）对反引号 /
// `$(` / `${` 无防呆 —— bash 会先做**命令替换**、把片段整个吃掉或替换成空（实害 = 提案内容静默残废而工具不报错）。
// 本件钉住缺口 want 的**权威判据**（`zerg dev proposal new --from-file <件> --dry-run`）要的那条出口：
// 文本从**盘上**读（`os.ReadFile`）⇒ **不经 shell** ⇒ 从根上避开「自由文本在进程内无法分辨真危险/纯文本」的死结。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）。
// 全部合成盘面：`ZERG_PROPOSAL_DIR` 指 `t.TempDir()` ⇒ 真 `~/.zerg/proposals` **一个字节不碰**。
//
// 钉住的格（逐条可单独失败）：
//
//	① 正控 · 干跑出计划件：一枚合法 JSON 对象件 ⇒ `--dry-run` rc=0 · stdout 出「目标/标题」· 目录零写。
//	② 正控 · 真写落一件且逐字等于件：`--yes` ⇒ rc=0 · 落 DEV-0001 · 盘上件含件里的 title/target/evidence 逐字。
//	③ 负控 · BAD 样本三发点名（✗ 别静默）：JSON 非法 / 件不存在 / 件是目录 ⇒ 各 rc=2 且 stderr 点名。
//	④ 负控 · 形状：件是数组 / `null` / 含认不得的键 ⇒ 各自 rc=2 且点名。
//	⑤ 负控 · 互斥：与单片旗标同给 / 与 `--batch` 同给 ⇒ 各自 rc=2 点名。
//	⑥ 负控 · 审批语义未放宽：缺 `--yes` ⇒ rc=2 点名 · 零写（与单片/批量同一条 fail-closed 门）。
//	⑦ 负控 · 收口不静默吞：别族命令（`version`）usage 未列它 ⇒ rc=2「未知旗标」。
//	⑧ 正控 · 机器面：`--json id,title` 出一枚可机读项。
package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ffEnv 合成提案目录 + 一枚「现读可得」的退点件（真状态目录一个字节不碰）。
func ffEnv(t *testing.T) (dir, rollback string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	rb := filepath.Join(t.TempDir(), "family_dev.go.before")
	if err := os.WriteFile(rb, []byte("（退点夹具 · 现读可得）\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, rb
}

func ffRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ffObj 一枚形状合法的单片档对象（键 = 单片档旗标名去 `--`）。
func ffObj(rb, title string) string {
	b, _ := json.Marshal(map[string]any{
		"title":     title,
		"target":    "D3b",
		"goal":      "目标一句话",
		"evidence":  []string{"描述性出处（无前缀自由文本 ⇒ 不走在册闸）"},
		"rollback":  rb,
		"criterion": "zerg code find cmdDevProposalNewFromFile",
		"file":      []string{"core/cmd/zerg/family_dev.go"},
	})
	return string(b)
}

func ffFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "one.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func ffIDs(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	ids := []string{}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids
}

// ---- ① 正控 · 干跑出计划件 + 零写 ----
func TestFromFile_DryRunPlanZeroWrite(t *testing.T) {
	dir, rb := ffEnv(t)
	f := ffFile(t, ffObj(rb, "单片档干跑"))
	rc, out, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--dry-run")
	if rc != 0 {
		t.Fatalf("① 破：--dry-run rc=%d（要 0）\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	for _, want := range []string{"单片档干跑", "目标   : D3b", "标题"} {
		if !strings.Contains(out, want) {
			t.Errorf("① 破：--dry-run stdout 未见 %q\n%s", want, out)
		}
	}
	if ids := ffIDs(t, dir); len(ids) != 0 {
		t.Errorf("① 破：--dry-run 竟落 %d 件（%v）· 应零写", len(ids), ids)
	}
}

// ---- ② 正控 · 真写落一件且逐字等于件 ----
func TestFromFile_WritesOneMatchingSpec(t *testing.T) {
	dir, rb := ffEnv(t)
	f := ffFile(t, ffObj(rb, "单片档真写"))
	rc, out, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--yes")
	if rc != 0 {
		t.Fatalf("② 破：--yes rc=%d（要 0）\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "DEV-0001") {
		t.Errorf("② 破：真写 stdout 未见提案号\n%s", out)
	}
	if ids := ffIDs(t, dir); len(ids) != 1 {
		t.Fatalf("② 破：应落 1 件，实落 %d 件（%v）", len(ids), ids)
	}
	back, err := os.ReadFile(filepath.Join(dir, "DEV-0001.json"))
	if err != nil {
		t.Fatalf("② 破：读不回落件：%v", err)
	}
	for _, want := range []string{"单片档真写", "D3b", "描述性出处（无前缀自由文本"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("② 破：盘上件缺 %q（件里给的字段没原样落）\n%s", want, back)
		}
	}
}

// ---- ③ 负控 · BAD 样本三发点名（✗ 别静默）----
func TestFromFile_BadSamplesNamed(t *testing.T) {
	ffEnv(t)
	dirPath := t.TempDir() // 「件是目录」
	cases := []struct {
		name string
		path string
		want string
	}{
		{"JSON 非法", ffFile(t, "{ this is not json "), "解不动"},
		{"件不存在", filepath.Join(t.TempDir(), "nope.json"), "读不到"},
		{"件是目录", dirPath, "读不到"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, _, errb := ffRun(t, "dev", "proposal", "new", "--from-file", c.path, "--dry-run")
			if rc != 2 {
				t.Errorf("③ 破：%s rc=%d（要 2 = 用法错 · 快失败）\nstderr=%s", c.name, rc, errb)
			}
			if !strings.Contains(errb, c.want) {
				t.Errorf("③ 破：%s 的判词没点名 %q\nstderr=%s", c.name, c.want, errb)
			}
		})
	}
}

// ---- ④ 负控 · 形状（数组 / null / 认不得的键）----
func TestFromFile_Shapes(t *testing.T) {
	_, rb := ffEnv(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"件是数组", "[" + ffObj(rb, "甲") + "]", "解不动"},
		{"件是 null", "null", "不是一枚 JSON 对象"},
		{"认不得的键", `{"nope":"x"}`, "认不得的键"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := ffFile(t, c.body)
			rc, _, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--dry-run")
			if rc != 2 {
				t.Errorf("④ 破：%s rc=%d（要 2）\nstderr=%s", c.name, rc, errb)
			}
			if !strings.Contains(errb, c.want) {
				t.Errorf("④ 破：%s 的判词没点名 %q\nstderr=%s", c.name, c.want, errb)
			}
		})
	}
}

// ---- ⑤ 负控 · 互斥（单片旗标 / --batch）----
func TestFromFile_Conflicts(t *testing.T) {
	_, rb := ffEnv(t)
	f := ffFile(t, ffObj(rb, "互斥"))
	// 与单片旗标同给
	rc, _, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--title", "混着给", "--dry-run")
	if rc != 2 || !strings.Contains(errb, "互斥") {
		t.Errorf("⑤ 破：from-file × 单片旗标 rc=%d（要 2 且点名「互斥」）\nstderr=%s", rc, errb)
	}
	// 与 --batch 同给
	sp := ffFile(t, "[]")
	rc2, _, errb2 := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--batch", sp, "--dry-run")
	if rc2 != 2 || !strings.Contains(errb2, "并存") {
		t.Errorf("⑤ 破：from-file × --batch rc=%d（要 2 且点名「并存」）\nstderr=%s", rc2, errb2)
	}
}

// ---- ⑥ 负控 · 审批语义未放宽（缺 --yes ⇒ 2 · 零写）----
func TestFromFile_ApprovalNotLoosened(t *testing.T) {
	dir, rb := ffEnv(t)
	f := ffFile(t, ffObj(rb, "缺 yes"))
	rc, _, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f)
	if rc != 2 || !strings.Contains(errb, "--yes") {
		t.Errorf("⑥ 破：缺 --yes rc=%d（要 2 且点名 --yes）\nstderr=%s", rc, errb)
	}
	if ids := ffIDs(t, dir); len(ids) != 0 {
		t.Errorf("⑥ 破：缺 --yes 竟落 %d 件", len(ids))
	}
}

// ---- ⑦ 负控 · 收口不静默吞：别族命令 usage 未列它 ⇒ rc=2「未知旗标」----
func TestFromFile_ForeignCommandRejected(t *testing.T) {
	rc, _, errb := ffRun(t, "version", "--from-file", "/tmp/x")
	if rc != 2 {
		t.Errorf("⑦ 破：`version --from-file` rc=%d（要 2 = 未知旗标 · 收口生效）\nstderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "未知旗标") {
		t.Errorf("⑦ 破：判词没点名「未知旗标」\nstderr=%s", errb)
	}
}

// ---- ⑧ 正控 · 机器面 --json ----
func TestFromFile_JSONFace(t *testing.T) {
	_, rb := ffEnv(t)
	f := ffFile(t, ffObj(rb, "机器面"))
	rc, out, errb := ffRun(t, "dev", "proposal", "new", "--from-file", f, "--yes", "--json", "id,title")
	if rc != 0 {
		t.Fatalf("⑧ 破：--json rc=%d（要 0）\nstderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("⑧ 破：--json 不是合法 JSON：%v\n%s", err, out)
	}
	if len(env.Items) != 1 {
		t.Errorf("⑧ 破：--json items=%d（要 1）", len(env.Items))
	}
}

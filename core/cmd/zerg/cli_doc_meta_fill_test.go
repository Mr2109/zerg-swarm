// cli_doc_meta_fill_test.go —— `zerg doc meta fill` 的**真二进制**判据（缺口-命令面 §八 · D3③-a 的等价命令）。
//
// 五格（全部走**合成文档树**，不碰真 `Zerg-内部文档`）：
//
//	① `--dry-run` **零副作用**（件字节不变、审计件不出现），且计划面逐件给出日期与来源；
//	② `--yes` 真写出来的**字节**与 D3③-a 那 62 篇的形态**逐字相同**（H1 + 两行插入）；
//	③ 幂等：写过之后再跑 ⇒ 待回填 0 件；
//	④ 没有 H1 / 日期两个来源都取不到 ⇒ 那一件**不给结论**（跳过 · 退码 8 · 一个字节没写）；
//	⑤ 缺 `--yes` ⇒ 退 2 且零副作用（D2 档 fail-closed）；`--json` 不给字段 ⇒ 退 1、stdout 0 字节。
package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execWithEnv 起真二进制（在 execCase 之上多给几枚环境变量 —— 本命令要 `ZERG_DEVDOCS_ROOT`/`ZERG_STATE_DIR`）。
func execWithEnv(t *testing.T, bin, repo string, env map[string]string, argv ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, argv...)
	cmd.Env = append(os.Environ(), "ZERG_REPO="+repo)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = strings.NewReader("")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	rc := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起不了 bin/zerg：%v", err)
		}
	}
	return rc, out.String(), errb.String()
}

// docRoot 建一棵合成文档树：`a-20260102.md`（缺标注）+ `b-20260103.md`（已有标注）。
// 返回 (根, 两份原件字节)。
func docRoot(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	orig := map[string]string{
		"a-20260102.md": "# 甲件\n\n> 用途：合成夹具的第一件。\n第二行正文。\n",
		"b-20260103.md": "# 乙件\n\n> 2026-01-03 · **本稿不开源**（开发文档侧：与主仓库并列、不进公开面与站点导出）。\n\n> 已有标注 ⇒ 幂等跳过。\n",
	}
	for n, body := range orig {
		mustWrite(t, filepath.Join(root, n), body)
	}
	return root, orig
}

func docRows(t *testing.T, out string) []map[string]string {
	t.Helper()
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	return env.Items
}

// ① 干跑零副作用 + ② --yes 写出来的字节与 D3③-a 的形态逐字相同 + ③ 幂等。
func TestDocMetaFill_DryRunThenWrite(t *testing.T) {
	bin := zergBinary(t)
	repo := syntheticRepo(t, "exit 0\n")
	root, orig := docRoot(t)
	state := t.TempDir()
	env := map[string]string{"ZERG_STATE_DIR": state}

	rc, out, errb := execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--dry-run",
		"--json", "file,date,date_source,action,reason,before_sha256,after_sha256")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	rows := docRows(t, out)
	if len(rows) != 1 || rows[0]["file"] != "a-20260102.md" {
		t.Fatalf("待回填的只有缺标注那一件：%+v", rows)
	}
	if rows[0]["date"] != "2026-01-02" || !strings.Contains(rows[0]["date_source"], "文件名") {
		t.Errorf("日期要取文件名里的 `-YYYYMMDD` 并写明来源：%+v", rows[0])
	}
	if rows[0]["action"] != "planned" {
		t.Errorf("干跑那一档的 action 必须是 planned：%+v", rows[0])
	}
	if rows[0]["before_sha256"] == "" || rows[0]["after_sha256"] == "" {
		t.Errorf("计划面要给前后 sha（改前一份 + 改后一份，供人复核）：%+v", rows[0])
	}
	// ① 零副作用：件字节逐字不变、审计件**没被建出来**
	got, _ := os.ReadFile(filepath.Join(root, "a-20260102.md"))
	if string(got) != orig["a-20260102.md"] {
		t.Errorf("干跑改了件？！\n得到：%q\n期望：%q", got, orig["a-20260102.md"])
	}
	if _, err := os.Stat(filepath.Join(state, "doc_meta_audit.jsonl")); err == nil {
		t.Errorf("干跑不许落审计（零副作用）")
	}

	// ② 真写
	rc, out, errb = execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--yes",
		"--json", "file,action,after_sha256,audit_path")
	if rc != 0 {
		t.Fatalf("--yes 真写 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	rows = docRows(t, out)
	if len(rows) != 1 || rows[0]["action"] != "written" {
		t.Fatalf("真写那一档的 action 必须是 written：%+v", rows)
	}
	got, _ = os.ReadFile(filepath.Join(root, "a-20260102.md"))
	want := "# 甲件\n\n> 2026-01-02 · **本稿不开源**（开发文档侧：与主仓并列、不进公开面与站点导出）。\n\n> 用途：合成夹具的第一件。\n第二行正文。\n"
	if string(got) != want {
		t.Errorf("写出来的字节必须与 D3③-a 的形态逐字相同：\n得到：%q\n期望：%q", got, want)
	}
	// 审计一行一事件
	ab, err := os.ReadFile(filepath.Join(state, "doc_meta_audit.jsonl"))
	if err != nil {
		t.Fatalf("审计件不在：%v", err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ab))), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v · %q", err, ab)
	}
	if line["file"] != "a-20260102.md" || line["before_sha256"] == line["after_sha256"] {
		t.Errorf("审计要记谁/哪件/前后 sha：%+v", line)
	}

	// ③ 幂等：再跑一次干跑 ⇒ 待回填 0 件
	rc, out, _ = execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--dry-run", "--json", "file")
	if rc != 0 || len(docRows(t, out)) != 0 {
		t.Errorf("幂等：已有标注的必须跳过（待回填 0 件），rc=%d out=%s", rc, out)
	}
}

// ④ 没有 H1 / 日期两来源都取不到 ⇒ 那一件不给结论（退码 8 · 一个字节没写）。
func TestDocMetaFill_UndecidableIsBlocked(t *testing.T) {
	bin := zergBinary(t)
	repo := syntheticRepo(t, "exit 0\n")
	root := t.TempDir()
	noH1 := "这是没有 H1 的件（插入点机械上判不了）。\n"
	mustWrite(t, filepath.Join(root, "noH1-20260104.md"), noH1)
	state := t.TempDir()
	env := map[string]string{"ZERG_STATE_DIR": state}
	rc, out, errb := execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--yes",
		"--json", "file,action,reason")
	if rc != 8 {
		t.Fatalf("不给结论 ⇒ 退 8（**不许当绿**），得到 %d · stderr=%s", rc, errb)
	}
	rows := docRows(t, out)
	if len(rows) != 1 || rows[0]["action"] != "skipped" || !strings.Contains(rows[0]["reason"], "H1") {
		t.Errorf("没有 H1 的件要标成 skipped 并写明原因：%+v", rows)
	}
	got, _ := os.ReadFile(filepath.Join(root, "noH1-20260104.md"))
	if string(got) != noH1 {
		t.Errorf("不给结论的件**一个字节都不许改**：%q", got)
	}
	if !strings.Contains(errb, "不给结论") {
		t.Errorf("人面要明说「不给结论」：%q", errb)
	}
}

// ⑤ 缺 `--yes` ⇒ 退 2 且零副作用（D2 档 fail-closed）；`--json` 不给字段 ⇒ 退 1、stdout 0 字节。
func TestDocMetaFill_YesAndJSONDiscipline(t *testing.T) {
	bin := zergBinary(t)
	repo := syntheticRepo(t, "exit 0\n")
	root, orig := docRoot(t)
	state := t.TempDir()
	env := map[string]string{"ZERG_STATE_DIR": state}

	rc, _, errb := execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root)
	if rc != 2 {
		t.Errorf("缺 --yes（也没给 --dry-run）⇒ 退 2（D2 fail-closed），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "缺 `--yes`") {
		t.Errorf("要明说缺哪一枚旗标：%q", errb)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a-20260102.md"))
	if string(got) != orig["a-20260102.md"] {
		t.Errorf("缺 --yes 时一个字节都不许写：%q", got)
	}

	// ★ 口径照实（与 `dev edit` 同款 · 本命令是**危险档**）：K2 退 1，且危险档的 stdout 走
	//   **错误包封**（`dev build --json` 现跑 = 285 字节 / rc=2 同一形状）——「0 字节」那条只适用于
	//   非危险档的结果面（`run()` 里 danger == nil 那一支）。字段清单在 stderr。
	rc, out, errb := execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--dry-run", "--json")
	if rc != 1 {
		t.Errorf("--json 不给字段 ⇒ 退 1（K2），得到 rc=%d", rc)
	}
	if !strings.Contains(errb, "可选字段") {
		t.Errorf("K2 要在 stderr 列可用字段：%q", errb)
	}
	if !strings.Contains(out, "\"schema\"") || !strings.Contains(out, "\"error\"") {
		t.Errorf("危险档的 K2 失败面 = 错误包封（六键 + error 块）：%q", out)
	}
	rc, _, _ = execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", root, "--dry-run", "--json", "nope")
	if rc != 2 {
		t.Errorf("未知字段 ⇒ 退 2（K1），得到 %d", rc)
	}
	rc, _, errb = execWithEnv(t, bin, repo, env, "doc", "meta", "fill", "--scope", filepath.Join(root, "no-such-dir"))
	if rc != 2 {
		t.Errorf("落点不是目录 ⇒ 退 2（用法面先判），得到 %d · stderr=%s", rc, errb)
	}
}

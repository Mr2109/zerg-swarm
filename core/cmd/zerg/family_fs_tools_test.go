// family_fs_tools_test.go —— `GAP-20261018-07`：`fs read` / `fs write` 的**兄弟仓正门**
// `--allow-sibling-repo <仓名>`（默认仍 fail-closed）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest`（跑**当前源码**的 `run` —— 连旗标解析面一起过，
// 所以「旗标真被命令面收下」这件事也在判据里，不是只测一个内部函数）。夹具全在 `t.TempDir()` 里：
//
//	<tmp>/Zerg/          —— 合成主仓根（`ZERG_REPO`）
//	<tmp>/Zerg-内部文档/     —— 合成兄弟仓根（= 仓根的兄弟目录 · 闭集里唯一的那个名字）
//
// 成对负控（✗ 禁恒绿 —— 每一格都有相反的那一格）：
//
//	① **默认态逐字不变**：不给旗标读 `../Zerg-内部文档/…` ⇒ rc=2 + 旧文案「件出仓了 … ✗ 只碰本仓工作树」；
//	② 给旗标读真件（三种写法：兄弟仓相对 / `../Zerg-内部文档/…` / 绝对路径）⇒ rc=0 且**内容真读到**；
//	③ 越界：旗标 + `../../etc/passwd` / 仓外绝对路径 ⇒ rc=2（✗ 从兄弟仓再绕出去）；
//	④ 非白名单：旗标值给路径形态 / 别家名 / 空名 ⇒ rc=2 且具名「非白名单兄弟仓」；
//	⑤ 兄弟仓根不在盘 ⇒ rc=8（不给结论）；
//	⑥ 写面：给旗标写临时件 ⇒ rc=0 · **读回字节一致** · 审计行带 `sibling_repo`/`sibling_root` ·
//	   stderr 明写「写了兄弟仓 X 的件 Y」；不给旗标写同一落点 ⇒ rc=2 且那一件**一个字节不动**；
//	⑦ 兄弟仓写面仍受原约束：父目录不存在 ⇒ rc=2（✗ 不自建）、目标件是符号链接 ⇒ rc=2。
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

// fsRunTest —— 在合成仓根/状态目录上跑一条 `zerg fs …`（与 `runDevEdit` 同形）。
func fsRunTest(t *testing.T, repo, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(append([]string{"fs"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// fsuoSynthRepoWithSibling —— 合成「主仓 + 兄弟仓 Zerg-内部文档」两棵树；返回 (仓根, 兄弟仓根, 兄弟仓内相对路径)。
func fsuoSynthRepoWithSibling(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "Zerg")
	sib := filepath.Join(base, "Zerg-内部文档")
	mustWrite(t, filepath.Join(repo, "core", "internal", "version", "version.go"), "package version\n")
	rel := filepath.Join("项目文档", "v2.5.15", "变更-v2.5.15.md")
	mustWrite(t, filepath.Join(sib, rel), "第一行：文档面\n第二行：原文\n")
	return repo, sib, rel
}

// TestFSRead_SiblingRepoGate —— 判据 ①–⑤（读面）。
func TestFSRead_SiblingRepoGate(t *testing.T) {
	repo, sib, rel := fsuoSynthRepoWithSibling(t)
	state := t.TempDir()

	// ① 默认态（不给旗标）⇒ rc=2 · **旧文案逐字**（fail-closed 一格）
	rc, out, errb := fsRunTest(t, repo, state, "read", filepath.Join("..", "Zerg-内部文档", rel))
	if rc != 2 {
		t.Fatalf("默认态读兄弟仓件 ⇒ rc=2（fail-closed）：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "件出仓了") || !strings.Contains(errb, "✗ 只碰本仓工作树") {
		t.Errorf("默认态文案必须与改前逐字同（`件出仓了 … ✗ 只碰本仓工作树`）：%q", errb)
	}

	// ② 给旗标读真件 —— 三种写法都要能读（内容真读到，不是「rc=0 但空」）
	for _, p := range []string{
		rel,                                   // 相对**兄弟仓根**（主用法 · 与 dev edit --file 同口径）
		filepath.Join("..", "Zerg-内部文档", rel), // 相对**主仓根**的旧写法
		filepath.Join(sib, rel),               // 绝对路径（含空格也能用 —— argv 是一枚实参）
	} {
		rc, out, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", p)
		if rc != 0 {
			t.Fatalf("给旗标读 %q ⇒ rc=0：rc=%d stdout=%q stderr=%q", p, rc, out, errb)
		}
		if !strings.Contains(out, "第一行：文档面") || !strings.Contains(out, "第二行：原文") {
			t.Errorf("给旗标读 %q ⇒ 内容要真读到：stdout=%q", p, out)
		}
	}

	// ②b 机器面 `path` = **兄弟仓内相对**（与 `dev edit` 的 `File` 同口径 ⇒ 回读时原样喂回）
	rc, out, errb = fsRunTest(t, repo, state, "read", rel, "--allow-sibling-repo", "Zerg-内部文档", "--json", "path")
	if rc != 0 || !strings.Contains(out, "项目文档/v2.5.15/变更-v2.5.15.md") {
		t.Errorf("机器面 path 要出兄弟仓内相对路径：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}

	// ③ 越界防绕：给了旗标也不许从兄弟仓再 `..` 出去
	for _, p := range []string{
		filepath.Join("..", "..", "etc", "passwd"),                                     // 相对兄弟仓根 ⇒ 两层 `..`
		filepath.Join("..", "..", "Zerg", "core", "internal", "version", "version.go"), // 绕回主仓也不行（只开兄弟树）
		"/etc/passwd", // 绝对路径出界
	} {
		rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", p)
		if rc != 2 || !strings.Contains(errb, "越出") {
			t.Errorf("越界路径 %q ⇒ rc=2 · 逐字「越出」：rc=%d stderr=%q", p, rc, errb)
		}
	}

	// ④ 非白名单兄弟仓 ⇒ rc=2 且**具名**拒（路径形态 / 大小写别名 / 陌生名 / 空名）
	for _, bad := range []string{filepath.Join("..", "Zerg-内部文档"), "../whatever", "ZergDocs", "foo", ""} {
		rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", bad, rel)
		if rc != 2 || !strings.Contains(errb, "非白名单兄弟仓") {
			t.Errorf("旗标值 %q ⇒ rc=2 · 具名「非白名单兄弟仓」：rc=%d stderr=%q", bad, rc, errb)
		}
	}

	// ⑤ 闭集名但兄弟仓根不在盘 ⇒ rc=8（不给结论 · 不猜）
	repo2 := t.TempDir()
	mustWrite(t, filepath.Join(repo2, "core", "internal", "version", "version.go"), "package version\n")
	rc, _, errb = fsRunTest(t, repo2, t.TempDir(), "read", "--allow-sibling-repo", "Zerg-内部文档", rel)
	if rc != 8 || !strings.Contains(errb, "不在盘上") {
		t.Errorf("兄弟仓根不在盘 ⇒ rc=8 · 逐字「不在盘上」：rc=%d stderr=%q", rc, errb)
	}

	// ⑥ 件不存在 ⇒ rc=2（给旗标也一样 · ✗ 不冒充空件）
	rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", "没有这件.md")
	if rc != 2 || !strings.Contains(errb, "件不存在") {
		t.Errorf("件不存在 ⇒ rc=2 · 逐字「件不存在」：rc=%d stderr=%q", rc, errb)
	}
}

// TestFSWrite_SiblingRepoGate —— 判据 ⑥–⑦（写面）。
func TestFSWrite_SiblingRepoGate(t *testing.T) {
	repo, sib, rel := fsuoSynthRepoWithSibling(t)
	state := t.TempDir()
	tmpDir := filepath.Join("临时-兄弟仓写面", "20261018")
	if err := os.MkdirAll(filepath.Join(sib, tmpDir), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmpDir, "探针件.md")
	body := "兄弟仓写面探针\n第二行\n"

	// ① 不给旗标 ⇒ rc=2（**默认仍 fail-closed**）· 件一个字节都不许出现
	rc, out, errb := fsRunTest(t, repo, state, "write", filepath.Join("..", "Zerg-内部文档", target),
		"--content", body, "--yes")
	if rc != 2 || !strings.Contains(errb, "件出仓了") {
		t.Fatalf("不给旗标写兄弟仓 ⇒ rc=2 · 旧文案：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	if _, err := os.Stat(filepath.Join(sib, target)); err == nil {
		t.Fatalf("被拒的那一格：兄弟仓件**不许**被建出来")
	}

	// ② 给旗标 + 兄弟仓相对路径 ⇒ rc=0 · 读回字节一致 · stderr 留痕 · 审计两格
	rc, out, errb = fsRunTest(t, repo, state, "write", target, "--content", body, "--yes",
		"--allow-sibling-repo", "Zerg-内部文档")
	if rc != 0 {
		t.Fatalf("给旗标写兄弟仓 ⇒ rc=0：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	got, rerr := os.ReadFile(filepath.Join(sib, target))
	if rerr != nil || string(got) != body {
		t.Fatalf("写后读回要逐字节一致：err=%v got=%q want=%q", rerr, got, body)
	}
	if _, err := os.Stat(filepath.Join(repo, target)); err == nil {
		t.Errorf("写了兄弟仓 ⇒ ✗ 不许在主仓同名路径留下副本")
	}
	if !strings.Contains(errb, "写了兄弟仓 Zerg-内部文档 的件") {
		t.Errorf("写兄弟仓必须留痕（stderr 一行）：%q", errb)
	}
	ab, aerr := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if aerr != nil {
		t.Fatalf("真写要落审计：%v", aerr)
	}
	lines := strings.Split(strings.TrimSpace(string(ab)), "\n")
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if row["sibling_repo"] != "Zerg-内部文档" || row["sibling_root"] != sib {
		t.Errorf("审计行要带「兄弟仓名 + 它的根」：%+v（want repo=Zerg-内部文档 root=%s）", row, sib)
	}
	if row["fs_path"] != filepath.ToSlash(target) {
		t.Errorf("审计 fs_path 要出兄弟仓内相对路径：%v", row["fs_path"])
	}

	// ③ 兄弟仓写面仍受原约束：父目录不存在 ⇒ rc=2（✗ 不自建）
	rc, _, errb = fsRunTest(t, repo, state, "write", filepath.Join("没有这层", "件.md"),
		"--content", body, "--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "父目录不存在") {
		t.Errorf("兄弟仓父目录不存在 ⇒ rc=2（✗ 不自建）：rc=%d stderr=%q", rc, errb)
	}
	if _, err := os.Stat(filepath.Join(sib, "没有这层")); err == nil {
		t.Errorf("✗ 不许自建目录")
	}

	// ④ 兄弟仓写面仍受原约束：目标件是符号链接 ⇒ rc=2
	link := filepath.Join(tmpDir, "链接件.md")
	if err := os.Symlink(filepath.Join(sib, rel), filepath.Join(sib, link)); err != nil {
		t.Fatalf("夹具建符号链接失败：%v", err)
	}
	rc, _, errb = fsRunTest(t, repo, state, "write", link, "--content", body, "--yes",
		"--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "符号链接") {
		t.Errorf("兄弟仓目标件是符号链接 ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	if b, _ := os.ReadFile(filepath.Join(sib, rel)); string(b) != "第一行：文档面\n第二行：原文\n" {
		t.Errorf("符号链接那一格：被指件一个字节都不许动：%q", b)
	}

	// ⑤ 写面越界 / 非白名单同样拒（写面不是「读面松、写面跟着松」）
	rc, _, errb = fsRunTest(t, repo, state, "write", filepath.Join("..", "..", "etc", "zerg-probe"),
		"--content", body, "--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "越出") {
		t.Errorf("写面越界 ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	rc, _, errb = fsRunTest(t, repo, state, "write", target, "--content", body, "--yes",
		"--allow-sibling-repo", "../whatever")
	if rc != 2 || !strings.Contains(errb, "非白名单兄弟仓") {
		t.Errorf("写面非白名单 ⇒ rc=2 · 具名：rc=%d stderr=%q", rc, errb)
	}
}

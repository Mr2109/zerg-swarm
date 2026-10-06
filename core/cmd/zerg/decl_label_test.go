package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dlRun 跑入口，返回 (rc, stdout, stderr)。
func dlRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := declLabelGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// dlWriteDecl 造声明件（内容逐字写；夹具落 t.TempDir() · 仓外）。
func dlWriteDecl(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "服务声明.tsv")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	return p
}

// dlSample 一份三行声明件（表头 + 两条 launchd + 一条 ghost），逐栏用中性占位。
const dlSample = "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\tlayer\n" +
	"launchd\tcom.zerg.core\tdecl1\tprog1\tbin/zerg-core\tmain\t-\thost\n" +
	"launchd\tcom.zerg.ui\tdecl2\tprog2\tbin/zerg-ui\tui\t-\thost\n" +
	"ghost\tcocoon-docs\t-\tprog3\tbin/cocoon-docs-service\tdocs\treap\t-\thost\n"

func TestDeclLabelExactOne(t *testing.T) {
	decl := dlWriteDecl(t, dlSample)
	rc, out, errb := dlRun("--decl", decl, "com.zerg.core")
	if rc != 0 || out != "com.zerg.core\n" || errb != "" {
		t.Fatalf("恰好一条：rc=%d out=%q err=%q（要 0 / name / 空）", rc, out, errb)
	}
}

func TestDeclLabelProcMatch(t *testing.T) {
	decl := dlWriteDecl(t, dlSample)
	rc, out, errb := dlRun("--decl", decl, "bin/zerg-ui")
	if rc != 0 || out != "com.zerg.ui\n" || errb != "" {
		t.Fatalf("进程特征命中：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelKindFilter(t *testing.T) {
	// ghost 行的 name/proc 都在件里，但 kind != launchd ⇒ 必须不计（否则 rc 会从 1 变 0）。
	decl := dlWriteDecl(t, dlSample)
	rc, out, errb := dlRun("--decl", decl, "cocoon-docs")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 0 条") {
		t.Fatalf("ghost 行须被 kind 过滤掉：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelZeroHit(t *testing.T) {
	decl := dlWriteDecl(t, dlSample)
	rc, out, errb := dlRun("--decl", decl, "no-such-name")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 0 条（要恰好 1 条）——查询 'no-such-name'") {
		t.Fatalf("零命中(非 --all)：rc=%d out=%q err=%q", rc, out, errb)
	}
	rc, out, errb = dlRun("--decl", decl, "--all", "no-such-name")
	if rc != 1 || out != "" || !strings.Contains(errb, "零命中：no-such-name") {
		t.Fatalf("零命中(--all)：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelMultiHit(t *testing.T) {
	// 一条 launchd 行 name 与 proc 同时命中查询？不 —— 造两行 name 相同：
	multi := "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\tlayer\n" +
		"launchd\tsvcA\td1\tp1\tfeatA\t-\t-\thost\n" +
		"launchd\tsvcA\td2\tp2\tfeatB\t-\t-\thost\n"
	decl := dlWriteDecl(t, multi)
	rc, out, errb := dlRun("--decl", decl, "svcA")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 2 条（要恰好 1 条）") {
		t.Fatalf("多条(非 --all)须 rc=1：rc=%d out=%q err=%q", rc, out, errb)
	}
	rc, out, errb = dlRun("--decl", decl, "--all", "svcA")
	if rc != 0 || out != "svcA\nsvcA\n" || errb != "" {
		t.Fatalf("多条(--all)须逐行按文件序：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
	}{
		{"缺查询", []string{"--all"}},
		{"未知旗标", []string{"-x", "q"}},
		{"裸横线", []string{"-", "q"}},
		{"decl缺参", []string{"--decl"}},
	} {
		rc, out, errb := dlRun(tc.argv...)
		if rc != 2 || out != "" || errb != dlUsage {
			t.Fatalf("%s：rc=%d out=%q err=%q（要 2 / 空 / usage）", tc.name, rc, out, errb)
		}
	}
}

func TestDeclLabelHelp(t *testing.T) {
	rc, out, errb := dlRun("-h")
	if rc != 0 || out != "" || errb != dlUsage {
		t.Fatalf("-h：rc=%d out=%q err=%q（要 0 / 空 / usage）", rc, out, errb)
	}
	rc, out, errb = dlRun("--help")
	if rc != 0 || out != "" || errb != dlUsage {
		t.Fatalf("--help：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelMissingDecl(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.tsv")
	rc, out, errb := dlRun("--decl", missing, "q")
	if rc != 2 || out != "" || !strings.Contains(errb, "声明件不存在："+missing) {
		t.Fatalf("缺声明件：rc=%d out=%q err=%q", rc, out, errb)
	}
	// 目录也不是常规件（bash `-f` 语义）。
	rc, out, errb = dlRun("--decl", t.TempDir(), "q")
	if rc != 2 || out != "" || !strings.Contains(errb, "声明件不存在：") {
		t.Fatalf("声明件是目录：rc=%d out=%q err=%q", rc, out, errb)
	}
}

func TestDeclLabelTerminator(t *testing.T) {
	// `--` 之后的位置参数不进查询 ⇒ 查询空 ⇒ 用法错 rc=2。
	rc, out, errb := dlRun("--", "com.zerg.core")
	if rc != 2 || out != "" || errb != dlUsage {
		t.Fatalf("`--` 后参数被忽略：rc=%d out=%q err=%q", rc, out, errb)
	}
	// `--` 之后即便写了 --decl 也不生效 ⇒ 缺声明件默认根/或空查询。
	decl := dlWriteDecl(t, dlSample)
	rc, out, errb = dlRun("--decl", decl, "cc", "--", "--all")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 0 条") {
		t.Fatalf("`--` 后 --decl/位置参数均忽略：rc=%d out=%q err=%q", rc, out, errb)
	}
}

// TestDeclLabelIFSCollapse —— 负控：钉住 bash `IFS=$'\t'` 的「连续制表符折叠 + 首尾剥」。
// 哑化（改回 strings.Split(line,"\t")）⇒ 列 4 落到空串 ⇒ 本用例必红。
func TestDeclLabelIFSCollapse(t *testing.T) {
	// 第 5 列前多插一个制表符：折叠后 proc 仍在列 4；不折叠则列 4 = ""。
	content := "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\tlayer\n" +
		"launchd\tcom.zerg.core\tdecl1\tprog1\t\tbin/zerg-core\tmain\t-\thost\n" +
		"\tlaunchd\tcom.zerg.ui\tdecl2\tprog2\tbin/zerg-ui\tui\t-\thost\n"
	decl := dlWriteDecl(t, content)
	if rc, out, _ := dlRun("--decl", decl, "bin/zerg-core"); rc != 0 || out != "com.zerg.core\n" {
		t.Fatalf("连续制表符折叠失效：rc=%d out=%q（列 4 应为 bin/zerg-core）", rc, out)
	}
	if rc, out, _ := dlRun("--decl", decl, "com.zerg.ui"); rc != 0 || out != "com.zerg.ui\n" {
		t.Fatalf("行首制表符剥除失效：rc=%d out=%q（kind 应仍为 launchd）", rc, out)
	}
}

// TestDeclLabelNoTrailingNewlineDropped —— 负控：钉住「末行无换行 ⇒ 丢弃」（bash read 于 EOF 返非零）。
// 哑化（按 strings.Split(…,"\n") 全处理）⇒ 末行被计入 ⇒ 本用例必红。
func TestDeclLabelNoTrailingNewlineDropped(t *testing.T) {
	// 末行无尾换行 ⇒ 该行丢弃 ⇒ 命中 0（rc=1）。
	content := "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\tlayer\n" +
		"launchd\tcom.zerg.core\tdecl1\tprog1\tbin/zerg-core\tmain\t-\thost\n" +
		"launchd\tcom.zerg.ui\tdecl2\tprog2\tbin/zerg-ui\tui\t-\thost" // ← 无尾换行
	decl := dlWriteDecl(t, content)
	rc, out, errb := dlRun("--decl", decl, "bin/zerg-ui")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 0 条") {
		t.Fatalf("末行无换行须被丢弃：rc=%d out=%q err=%q", rc, out, errb)
	}
	// 对照：给末行补上尾换行 ⇒ 立刻命中（证「丢弃」是唯一因，不是别处错）。
	decl2 := dlWriteDecl(t, content+"\n")
	if rc, out, _ := dlRun("--decl", decl2, "bin/zerg-ui"); rc != 0 || out != "com.zerg.ui\n" {
		t.Fatalf("补尾换行后应命中：rc=%d out=%q", rc, out)
	}
}

func TestDeclLabelEmptyDecl(t *testing.T) {
	decl := dlWriteDecl(t, "")
	rc, out, errb := dlRun("--decl", decl, "q")
	if rc != 1 || out != "" || !strings.Contains(errb, "命中 0 条") {
		t.Fatalf("空声明件：rc=%d out=%q err=%q", rc, out, errb)
	}
}

// TestDeclLabelDefaultRoot —— 缺省声明件 = <仓根>/deploy/服务声明.tsv。
// 用 ZERG_REPO 钉住仓根（repoRoot 的首选项），验证缺省解析与控制流（不经 --decl）。
func TestDeclLabelDefaultRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", "服务声明.tsv"), []byte(dlSample), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_SVCDECL_DECL", "")
	rc, out, errb := dlRun("com.zerg.ui")
	if rc != 0 || out != "com.zerg.ui\n" || errb != "" {
		t.Fatalf("缺省根解析：rc=%d out=%q err=%q", rc, out, errb)
	}
}

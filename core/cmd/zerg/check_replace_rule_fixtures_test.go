// check_replace_rule_fixtures_test.go —— Go 真搬的**成对负控**定向单测（M2 · 替换规则夹具族）。
//
// ✗ 禁恒绿：本件同时钉住「必然判绿 rc=0」「必然出红明细 rc=1（逐条点名）」「不给结论 rc=2（多档）」
// 与 py 真口子的**现状牙**（B 类读失败静默：读不了的件被 `scanned` 计入却不产命中行），
// 并钉住 `--json` 的**空值三形态**（`[]` 在场 / 键不在场）。
//
// ★ 本件自身落 `core/` 且以 `_test.go` 结尾 ⇒ **在本门扫描面内**（自指②）⇒ 正文**零私有面字面量**
//
//	（家目录 / 用户名 / 内网地址 / 私有项目名 / 仓名一律不写；夹具正文一律走**合成串**：
//	`/srv/demo` · `userdemo` · `198.51.100.7`）—— 否则真仓本门读数被自伤污染。
//
// 用例名：TestCheckReplaceRuleFixturesGatePort（定向：go test -run TestCheckReplaceRuleFixturesGatePort ./cmd/zerg）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// crrfTestPy —— 仓外 py 副本（由夹具目录里现读件拷贝而来）。
const crrfTestPy = "check-replace-rule-fixtures.py"

// crrfW —— 造件（自动建父目录）。
func crrfW(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// crrfFx —— 造一棵仓外夹具根（含 `scripts/gates/` 哨兵，供缺省根上溯命中）。
func crrfFx(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "fx")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	crrfW(t, filepath.Join(root, "scripts", "gates", crrfTestPy), "# sentinel\n")
	return root
}

// crrfRun —— 跑入口，收 (rc, stdout, stderr)。
func crrfRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkReplaceRuleFixturesGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckReplaceRuleFixturesGatePort(t *testing.T) {
	// ── 正控（必然判绿）：规则命中**非夹具**件 ⇒ rc=0 ────────────────────────────
	t.Run("正控_命中非夹具_rc0", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "a", "impl.go"), "home := \"/srv/demo\"\n")
		crrfW(t, filepath.Join(root, "core", "a", "clean_test.go"), "x := 1\n")
		rc, out, errs := crrfRun(t, "--root", root)
		if rc != 0 || errs != "" {
			t.Fatalf("正控失败：应 rc=0/stderr 空，得 rc=%d err=%q out=%s", rc, errs, out)
		}
		if !strings.Contains(out, "全绿（rc=0）") {
			t.Fatalf("正控失败：正文缺「全绿」out=%s", out)
		}
	})

	// ── 负控①（有红明细）：规则命中 `*_test.go` ⇒ rc=1 且逐条点名（规则/件/行号/片段）────
	t.Run("负控_命中夹具_rc1_逐条点名", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "a", "fixture_test.go"), "home := \"/srv/demo/x.key\"\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 1 {
			t.Fatalf("负控失败：应 rc=1，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "core/a/fixture_test.go:1:") {
			t.Fatalf("负控失败：未逐条点名件+行号 out=%s", out)
		}
		if !strings.Contains(out, "命中：**1 条**规则") {
			t.Fatalf("负控失败：未报命中条数 out=%s", out)
		}
	})

	// ── 负控②：规则表缺件 / 白名单缺件 / 空转 —— 一律「不给结论 rc=2」────────────
	t.Run("退2_规则表缺件", func(t *testing.T) {
		root := crrfFx(t)
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "规则表缺件") {
			t.Fatalf("应 rc=2 规则表缺件，得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("退2_白名单缺件", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "白名单缺件") {
			t.Fatalf("应 rc=2 白名单缺件，得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("退2_空规则_空转", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "# only comment\n\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "一条真规则都没有") {
			t.Fatalf("应 rc=2 空转，得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("退2_空转_无夹具", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "一件 `*_test.go` 都没扫到") {
			t.Fatalf("应 rc=2 空转≠绿，得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("退2_坏规则表形态", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "no_tab_rule\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "不是 `pattern<TAB>replacement...` 形态") {
			t.Fatalf("应 rc=2 不可解析，得 rc=%d out=%s", rc, out)
		}
	})

	// ── 夹具隔离：不隔离 ⇒ 必红 · 隔离 ⇒ 必绿（且逐件登记）────────────────────────
	t.Run("隔离_登记后必绿", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"),
			"regex:/srv/demo\t<home>\t合成\n!夹具\tcore/a/*_test.go\t隔离\n")
		crrfW(t, filepath.Join(root, "core", "a", "fixture_test.go"), "home := \"/srv/demo/x\"\n")
		crrfW(t, filepath.Join(root, "core", "b", "other_test.go"), "ok := 1\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 0 {
			t.Fatalf("隔离后应 rc=0，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "core/a/fixture_test.go") || !strings.Contains(out, "隔离条目") {
			t.Fatalf("隔离应逐件登记可视化，out=%s", out)
		}
	})
	t.Run("隔离吃光_rc2", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"),
			"regex:/srv/demo\t<home>\t合成\n!夹具\t*_test.go\t吃光\n")
		crrfW(t, filepath.Join(root, "core", "a", "x_test.go"), "ok := 1\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 2 || !strings.Contains(out, "吃光") {
			t.Fatalf("隔离吃光应 rc=2，得 rc=%d out=%s", rc, out)
		}
	})

	// ── `--json` 空值三形态：`[]` 在场 / 键不在场（★ 不是 `null`、也不是 omitempty 吞键）──
	t.Run("json_空列表在场", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "a", "clean_test.go"), "x := 1\n")
		rc, out, _ := crrfRun(t, "--root", root, "--json", "hits,files,per_rule")
		if rc != 0 {
			t.Fatalf("应 rc=0，得 rc=%d", rc)
		}
		if !strings.Contains(out, "\"hits\": 0") || !strings.Contains(out, "\"files\": []") ||
			!strings.Contains(out, "\"per_rule\": []") {
			t.Fatalf("空值三形态失败：应 `[]` 在场且非 null，out=%s", out)
		}
		if strings.Contains(out, "null") {
			t.Fatalf("实证失败：不得出现 null，out=%s", out)
		}
	})
	t.Run("json_键不在场", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "a", "clean_test.go"), "x := 1\n")
		_, out, _ := crrfRun(t, "--root", root, "--json", "rules")
		if !strings.Contains(out, "{\"rules\": 1}") {
			t.Fatalf("应只出 rules 一键，out=%s", out)
		}
		if strings.Contains(out, "\"files\"") {
			t.Fatalf("未请求的键不得在场，out=%s", out)
		}
	})
	t.Run("json_字段名打错_rc2", func(t *testing.T) {
		rc, out, _ := crrfRun(t, "--json", "a,b")
		if rc != 2 || !strings.Contains(out, "不认的字段名") {
			t.Fatalf("应 rc=2 字段名打错，得 rc=%d out=%s", rc, out)
		}
	})
	t.Run("json_新三格_去重和", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"),
			"regex:userdemo\t<u>\t合成\nregex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "x", "a_test.go"), "s := \"userdemo\"\nh := \"/srv/demo\"\n")
		crrfW(t, filepath.Join(root, "core", "x", "b_test.go"), "u := \"userdemo\"\n")
		_, out, _ := crrfRun(t, "--root", root, "--json", "files,files_dedup,files_sum")
		if !strings.Contains(out, "\"files_sum\": 3") || !strings.Contains(out, "\"files_dedup\": [\"core/x/a_test.go\", \"core/x/b_test.go\"]") {
			t.Fatalf("去重集 2 ≠ 未去重和 3 应分得开，out=%s", out)
		}
	})

	// ── py 真口子的**现状牙**（B 类读失败静默）：读不了的 `*_test.go` 仍被计入件面，但不产命中行 ──
	t.Run("口子_读不了的件静默", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无视权限位 ⇒ 本用例不适用")
		}
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		bad := filepath.Join(root, "core", "unread", "bad_test.go")
		crrfW(t, bad, "home := \"/srv/demo/secret\"\n")
		if err := os.Chmod(bad, 0o000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(bad, 0o644)
		crrfW(t, filepath.Join(root, "core", "a", "hit_test.go"), "home := \"/srv/demo/x\"\n")
		rc, out, _ := crrfRun(t, "--root", root)
		if rc != 1 {
			t.Fatalf("应 rc=1（另一件命中），得 rc=%d out=%s", rc, out)
		}
		if strings.Contains(out, "core/unread/bad_test.go") {
			t.Fatalf("读不了的件本应静默跳过（py 现状），却出现在正文：out=%s", out)
		}
		if !strings.Contains(out, "core/a/hit_test.go:1:") {
			t.Fatalf("可读件的命中仍须点名 out=%s", out)
		}
	})

	// ── `--self-test` / `--list-rules`：判据自检 + 档位面 ──────────────────────
	t.Run("自检_rc0", func(t *testing.T) {
		rc, out, _ := crrfRun(t, "--self-test")
		if rc != 0 || !strings.Contains(out, "自检结论: 全过") {
			t.Fatalf("自检应 rc=0 全过，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "断言 14 条 · 失败 0 条") {
			t.Fatalf("自检断言计数不符 out=%s", out)
		}
	})
	t.Run("档位_rc0", func(t *testing.T) {
		rc, out, _ := crrfRun(t, "--list-rules")
		if rc != 0 || !strings.Contains(out, "退码：0 全绿 · 1 有命中 · 2 不给结论") {
			t.Fatalf("档位面应 rc=0 且逐字含退码行，得 rc=%d out=%s", rc, out)
		}
	})

	// ── argparse 面：未知旗标（usage+error 落 stderr）· 缺值 · 非法 int ──────────
	t.Run("未知旗标_rc2", func(t *testing.T) {
		rc, out, errs := crrfRun(t, "--nope")
		if rc != 2 || out != "" || !strings.Contains(errs, "unrecognized arguments: --nope") {
			t.Fatalf("未知旗标应 rc=2/stderr 报话，得 rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.HasPrefix(errs, "usage: ") {
			t.Fatalf("stderr 应带 usage，得 %q", errs)
		}
	})
	t.Run("缺值_rc2", func(t *testing.T) {
		rc, _, errs := crrfRun(t, "--root")
		if rc != 2 || !strings.Contains(errs, "expected one argument") {
			t.Fatalf("缺值应 rc=2，得 rc=%d err=%q", rc, errs)
		}
	})
	t.Run("非法int_rc2", func(t *testing.T) {
		rc, _, errs := crrfRun(t, "--max-examples", "x")
		if rc != 2 || !strings.Contains(errs, "invalid int value") {
			t.Fatalf("非法 int 应 rc=2，得 rc=%d err=%q", rc, errs)
		}
	})

	// ── 缺省根（见件头 (a)）：不传 --root ⇒ 从 cwd 上溯找 `scripts/gates/` ──────
	t.Run("缺省根上溯", func(t *testing.T) {
		root := crrfFx(t)
		crrfW(t, filepath.Join(root, "publish", "whitelist.txt"), "core/\n")
		crrfW(t, filepath.Join(root, "publish", "replace-rules.tsv"), "regex:/srv/demo\t<home>\t合成\n")
		crrfW(t, filepath.Join(root, "core", "a", "clean_test.go"), "x := 1\n")
		cwd, _ := os.Getwd()
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)
		rc, out, _ := crrfRun(t)
		// ★ Go 无 `__file__` 语境 ⇒ 缺省根起于 cwd 上溯（`os.Getwd()` 解析真路径，
		// 与 py `os.getcwd()` 同）⇒ 期望值取 `EvalSymlinks`（见件头 (a)：符号链接路径两态不同）。
		want, _ := filepath.EvalSymlinks(root)
		if rc != 0 || !strings.Contains(out, "树："+want) {
			t.Fatalf("缺省根应上溯到夹具根，得 rc=%d out=%s", rc, out)
		}
	})
}

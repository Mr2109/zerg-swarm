// check_affected_map_test.go —— `check_affected_map.go` 的**成对负控**单测
// （M2 · 承接 GAP-20261001-13 · 入口 `checkAffectedMapGateMain`）。
//
// 口径：✗ 禁恒绿 —— 每格都断言**判据面**（rc ＋ 关键正文行），不是只断言「跑起来了」。
// 成对：正控（声明齐 ⇒ rc=0）与负控（悬空声明 / 空表 `[]` / `null` / 缺 `files` 键 / 缺件 ⇒ rc=1）成对，
// 空值三形态（`[]` · `null` · 缺键）**逐形态**各一格。
// 有牙实证（仓外副本哑化 ⇒ 本件用例 FAIL）见交付回执：把 `afmCheckMap` 的
// `if len(class2) > 0 { rc = 1 }` 掐成 `rc = 0` ⇒ 「负控:悬空声明」当场 FAIL（带行号）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// afmFix —— 造一棵**仓外夹具树**（自带 scripts/gates 三件 + 一件脚本候选）。
func afmFix(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	sc := "scripts/gates/" + "check-gate-coverage.py"
	step := "DEFAULT_SCOPES=(\"go\")\n" +
		"add_step go \"core: build\" rc \"${REPO_ROOT}/core\" \"go build ./...\"\n" +
		"add_step docs \"门① 覆盖面\" tri \"${REPO_ROOT}\" \"python3 " + sc + "\"\n"
	if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte(step), 0o644); err != nil {
		t.Fatal(err)
	}
	cov := `{"scan":{"script_globs":["scripts/gates/*.py"]},` +
		`"exclude":{"path_components":[".git"],"prefixes":[]},` +
		`"whitelist":{"scripts":[{"path":"` + sc + `"}]}}`
	if err := os.WriteFile(filepath.Join(g, "gate-coverage.config.json"), []byte(cov), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "newcmd-paths-baseline.json"), []byte(`{"count":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 声明表里那条 literal 必须在盘上（否则正控自己会变成「悬空声明 ⇒ rc=1」）。
	if err := os.WriteFile(filepath.Join(root, sc), []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func afmRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkAffectedMapGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// afmWriteMap —— 往夹具落一张声明表，返回其路径。
func afmWriteMap(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, "map-crafted.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckAffectedMapGatePort(t *testing.T) {
	root := afmFix(t)
	diskOK := `[{"pattern":"core/**","kind":"prefix"},` +
		`{"pattern":"scripts/gates/check-gate-coverage.py","kind":"literal"}]`
	// ★ 步面格（③）夹具：`faces` 命名步必须 ⟷ 夹具步骤表（afmFix 的 2 步）。
	facesOK := `{"core":["core: build"],"repo":["门① 覆盖面"]}`

	t.Run("正控:emit 落件 rc=0 且出 schema", func(t *testing.T) {
		outMap := filepath.Join(root, "emitted.json")
		rc, so, se := afmRun(t, "--emit", "--root", root, "--out", outMap)
		if rc != 0 {
			t.Fatalf("emit rc=%d 期望 0（stderr=%q）", rc, se)
		}
		if !strings.Contains(so, "已生成声明表：") {
			t.Fatalf("emit 未打印生成行：%q", so)
		}
		b, err := os.ReadFile(outMap)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"schema"`) {
			t.Fatalf("emit 落件缺 schema 键")
		}
	})

	t.Run("正控:声明齐 rc=0 且 both 计数在场", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"schema":"affected-map/1","files":`+diskOK+`,"faces":`+facesOK+`}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（%q）", rc, so)
		}
		if !strings.Contains(so, "声明条目 2（both=2）") {
			t.Fatalf("both 计数不对：%q", so)
		}
	})

	t.Run("负控:悬空声明 rc=1 且点名叫出", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"files":[{"pattern":"core/**","kind":"prefix"},`+
			`{"pattern":"scripts/gates/ghost.py","kind":"literal"}],"faces":`+facesOK+`}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "表里有件盘上无（悬空声明 ⇒ 红）：1 条") {
			t.Fatalf("未点名悬空声明：%q", so)
		}
	})

	t.Run("负控:空表 [] rc=1", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"schema":"affected-map/1","files":[]}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "无鉴别力") {
			t.Fatalf("未走「空表无鉴别力」档：%q", so)
		}
	})

	t.Run("负控:files 为 null rc=1", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"schema":"affected-map/1","files":null}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "无鉴别力") {
			t.Fatalf("null 未与 [] 同档：%q", so)
		}
	})

	t.Run("负控:缺 files 键 rc=1", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"schema":"affected-map/1"}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "缺 `files`") {
			t.Fatalf("缺键未走坏件档：%q", so)
		}
	})

	t.Run("负控:声明表缺件 rc=1", func(t *testing.T) {
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", filepath.Join(root, "no-such.json"))
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "声明表不在") {
			t.Fatalf("缺件未点名：%q", so)
		}
	})

	t.Run("负控:未声明件不是红 rc=0", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"files":[{"pattern":"core/**","kind":"prefix"}],"faces":`+facesOK+`}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（未声明是常态）", rc)
		}
		if !strings.Contains(so, "盘上有件但表里没声") {
			t.Fatalf("未列未声明面：%q", so)
		}
	})

	t.Run("负控:无参 rc=2 用法档", func(t *testing.T) {
		rc, so, se := afmRun(t)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if so != "" || !strings.Contains(se, "用法：check-affected-map.py") {
			t.Fatalf("用法档错面：stdout=%q stderr=%q", so, se)
		}
	})

	t.Run("负控:未知旗标 rc=2", func(t *testing.T) {
		rc, _, se := afmRun(t, "--nope")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(se, "unrecognized arguments: --nope") {
			t.Fatalf("未报未知旗标：%q", se)
		}
	})

	t.Run("正控:步面齐 rc=0 且 ③ 绿", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"files":`+diskOK+`,"faces":`+facesOK+`}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（%q）", rc, so)
		}
		if !strings.Contains(so, "③ 步面") || !strings.Contains(so, "：绿 · ") {
			t.Fatalf("③ 步面未判绿：%q", so)
		}
	})

	t.Run("负控:步面漂移 D≠N rc=3（≠悬空声明的 1）", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"files":`+diskOK+`,"faces":{"core":["core: build"]}}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 3 {
			t.Fatalf("rc=%d 期望 3（步面漂移）", rc)
		}
		if !strings.Contains(so, "静默漏选") {
			t.Fatalf("未点名步面漂移：%q", so)
		}
	})

	t.Run("负控:步面集合不等 rc=3", func(t *testing.T) {
		mp := afmWriteMap(t, root, `{"files":`+diskOK+`,"faces":{"core":["core: build"],"repo":["zzz"]}}`)
		rc, so, _ := afmRun(t, "--check", "--root", root, "--map", mp)
		if rc != 3 {
			t.Fatalf("rc=%d 期望 3（集合不等）", rc)
		}
		if !strings.Contains(so, "命名集合 ≠ 表集合") {
			t.Fatalf("未点名集合不等：%q", so)
		}
	})

	t.Run("--self-test rc=0 十控全过", func(t *testing.T) {
		rc, so, _ := afmRun(t, "--self-test")
		if rc != 0 {
			t.Fatalf("self-test rc=%d 期望 0：%q", rc, so)
		}
		if !strings.Contains(so, "自检通过：10 控") {
			t.Fatalf("自检文案不对：%q", so)
		}
	})

	t.Run("-h rc=0 帮助正文在场", func(t *testing.T) {
		rc, so, _ := afmRun(t, "-h")
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if !strings.Contains(so, "usage: check-affected-map.py") || !strings.Contains(so, "--self-test") {
			t.Fatalf("帮助正文不全：%q", so)
		}
	})
}

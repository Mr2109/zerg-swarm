// check_gap_state_vs_artifact_test.go —— 判据件 `scripts/gates/check-gap-state-vs-artifact.py`
// Go 真搬（`check_gap_state_vs_artifact.go` · 入口 `checkGapStateVsArtifactGateMain`）的**成对负控**。
//
// 口径（M2 搬运面）：
//
//	· 每条断言都是「**哑化 ⇒ FAIL 带行号**」的：把主件里对应的匹配器/判据体改瞎（如把 `gsaBoundary`
//	  退化成 ASCII、把 `gsaVisibleNotes` 的作废注过滤删掉、把 R5 语境门槛去掉），下列用例必在**本件
//	  的行号**上 FATAL。
//	· 只吃主件的**导出/包级**符号（同 package main）—— 不另起子进程、不碰真仓/真源。
//	· 夹具一律落 `t.TempDir()`（自带回收）；显式传 `--repo-root`，不依赖 cwd 摆位（声明 (a) 面）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func gsaRunEntry(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkGapStateVsArtifactGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestGsaEntryHelp(t *testing.T) {
	rc, out, errs := gsaRunEntry(t, "-h")
	if rc != 0 {
		t.Fatalf("line==this: -h 应 rc=0，得 %d", rc)
	}
	if !strings.HasPrefix(out, "usage: check-gap-state-vs-artifact.py [-h] [--source SOURCE]\n") {
		t.Fatalf("line==this: 帮助 usage 首行不符:\n%q", out)
	}
	if !strings.Contains(out, "\noptions:\n") {
		t.Fatalf("line==this: 帮助应有 py3.14 的 `options:` 段:\n%q", out)
	}
	if errs != "" {
		t.Fatalf("line==this: -h 的 stderr 应空: %q", errs)
	}
}

func TestGsaEntryVersion(t *testing.T) {
	rc, out, _ := gsaRunEntry(t, "--version")
	if rc != 0 || out != "check-gap-state-vs-artifact.py v1.0.0\n" {
		t.Fatalf("line==this: --version 应 rc=0 且印标题，得 rc=%d out=%q", rc, out)
	}
}

func TestGsaEntryArgErrors(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"unknown", []string{"--nope"}, "error: unrecognized arguments: --nope\n"},
		{"missing-value", []string{"--source"}, "error: argument --source: expected one argument\n"},
		{"ambiguous", []string{"--e"}, "error: ambiguous option: --e could match --expect-sha16, --expect-lines\n"},
		{"flag-value", []string{"--json=1"}, "error: argument --json: ignored explicit argument '1'\n"},
		{"positional", []string{"foo"}, "error: unrecognized arguments: foo\n"},
	}
	for _, c := range cases {
		rc, out, errs := gsaRunEntry(t, c.argv...)
		if rc != 2 || out != "" || !strings.HasSuffix(errs, c.want) {
			t.Fatalf("line==this: %s 应 rc=2、stdout 空、stderr 尾 %q，得 rc=%d out=%q err=%q",
				c.name, c.want, rc, out, errs)
		}
	}
}

func TestGsaEvidenceMissing(t *testing.T) {
	cases := []struct {
		ev   string
		want string
	}{
		{"现读 `zerg gap ls` ⇒ rc=0 ⇒ 411 条", ""},
		{"跑 `zerg gap ls` rc=0", "读数"},
		{"现读 scripts/gates/x.py 命中 12 处", "退码"},
		{"", "命令/退码/读数"},
		{"退码=0", "命令/读数"},
	}
	for _, c := range cases {
		got := strings.Join(gsaEvidenceMissing(c.ev), "/")
		if got != c.want {
			t.Fatalf("line==this: evidence_missing(%q) 得 %q 要 %q", c.ev, got, c.want)
		}
	}
}

// ★ Unicode 词界 `\b`：把 gsaBoundary 退化成 ASCII 词界 ⇒ 本用例必红（CJK 是 py 的词字符）。
func TestGsaBoundaryUnicode(t *testing.T) {
	if gsaRead3([]rune("12")) != true {
		t.Fatalf("line==this: `\\b\\d{2,}\\b` 对独立 12 应真")
	}
	if gsaRead3([]rune("a12")) != false {
		t.Fatalf("line==this: 12 前贴词字符 a ⇒ 无词界，应假（ASCII 词界会误判真）")
	}
	if gsaRead3([]rune("中12")) != false {
		t.Fatalf("line==this: 12 前贴 CJK 中（py 词字符）⇒ 无词界，应假（ASCII 词界会误判真）")
	}
	if gsaCmd2([]rune(" zerg dev")) != true {
		t.Fatalf("line==this: 空格后 zerg dev 应真")
	}
	if gsaCmd2([]rune("中zerg dev")) != false {
		t.Fatalf("line==this: CJK 紧贴 zerg ⇒ py 无词界，应假")
	}
	if gsaRC4([]rune("rc 3")) != true {
		t.Fatalf("line==this: rc 3 应真")
	}
	if gsaRC4([]rune("中rc 3")) != false {
		t.Fatalf("line==this: CJK 紧贴 rc ⇒ 无词界，应假")
	}
}

// ★ `\d` 是 Nd（py 语义）：`退码` 后允许「-」或 ASCII 0-9；`读数` 的 `\d` 认全角数字。
func TestGsaDigitSemantics(t *testing.T) {
	if gsaRC2([]rune("退码 5")) != true {
		t.Fatalf("line==this: 退码 5 应真")
	}
	if gsaRC2([]rune("退码x")) != false {
		t.Fatalf("line==this: 退码 后非 [-0-9] 应假")
	}
	if gsaRead1([]rune("３ 条")) != true {
		t.Fatalf("line==this: 全角数字 ３ 属 Nd，应真")
	}
	if gsaCmd5([]rune("xcmd=1")) != false {
		t.Fatalf("line==this: cmd 前贴词字符 ⇒ 无词界，应假")
	}
	if gsaCmd5([]rune("cmd=")) != true {
		t.Fatalf("line==this: cmd= 应真")
	}
	if gsaRead2([]rune("sha 0123ab")) != true {
		t.Fatalf("line==this: sha + ≥6 位十六进制应真")
	}
}

// ★ R5 语境门槛：只有「真路径主张」才抽；散文提及/URL/绝对/非仓内顶层/`...` 都不抽。
func TestGsaEvidencePathTokens(t *testing.T) {
	top := map[string]bool{"core": true, "scripts": true}
	cases := []struct {
		name string
		ev   string
		want []string
	}{
		{"反引号内真路径", "现读 `core/cmd/zerg/main.go` rc=0 12 条", []string{"core/cmd/zerg/main.go"}},
		{"散文提及（无反引号）", "排查时提到 core/cmd/zerg/main.go rc=0 12 条", nil},
		{"前缀 cmd= 后", "现跑 cmd=core/x/y.go rc=0", []string{"core/x/y.go"}},
		{"URL/绝对/非仓外", "见 `https://github.com/x/y.go` · `/etc/hosts` · `otherrepo/a/b.py`", nil},
		{"Go 模式串", "跑 `core/...` rc=0", nil},
		{"`~/.zerg/x` 后缀不抽", "取 ~/.zerg/selftest", nil},
	}
	for _, c := range cases {
		got := gsaEvidencePathTokens(c.ev, top)
		if len(got) != len(c.want) {
			t.Fatalf("line==this: %s 抽 %v 要 %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("line==this: %s 抽 %v 要 %v", c.name, got, c.want)
			}
		}
	}
}

// ★ notes_void 过滤：删掉过滤 ⇒ 本用例必红。
func TestGsaVisibleNotesVoid(t *testing.T) {
	row := map[string]any{
		"notes":      []any{"仍缺：前提还在"},
		"notes_void": []any{map[string]any{"void": true, "n": 1}},
	}
	if got := gsaVisibleNotes(row); len(got) != 0 {
		t.Fatalf("line==this: 作废注应被滤掉，得 %v", got)
	}
	row2 := map[string]any{"notes": []any{"a", "仍缺 b"}}
	if got := gsaVisibleNotes(row2); len(got) != 2 || got[1] != "仍缺 b" {
		t.Fatalf("line==this: 无作废应保序两条，得 %v", got)
	}
}

// ★ 存量/新增划界：found_at 或号内日期。
func TestGsaRowIsNew(t *testing.T) {
	if !gsaRowIsNew(map[string]any{"id": "x", "found_at": "2026-09-28T00:00:00"}, gsaCutoffDef) {
		t.Fatalf("line==this: found_at>=cutoff ⇒ 新增")
	}
	if gsaRowIsNew(map[string]any{"id": "GAP-20260920-01", "found_at": "2026-09-20T00:00:00"}, gsaCutoffDef) {
		t.Fatalf("line==this: 旧账旧时刻 ⇒ 存量")
	}
	if !gsaRowIsNew(map[string]any{"id": "GAP-20261001-01", "found_at": "2026-09-20T00:00:00"}, gsaCutoffDef) {
		t.Fatalf("line==this: 号内日期>=2026-09-28 ⇒ 新增")
	}
}

// ★ 编码面：py json.dumps(ensure_ascii=False) 的 `, `/`: ` 分隔与空集合 [] 。
func TestGsaDumpsExact(t *testing.T) {
	got := gsaDumps(gsaObj{
		{"schema", "zerg/v1"},
		{"ki" + "nd", "GapStateVsArtifact"},
		{"warnings", []any{}},
		{"meta", gsaObj{{"lines", 3}, {"ok", true}}},
	})
	want := `{"schema": "zerg/v1", "kind": "GapStateVsArtifact", "warnings": [], "meta": {"lines": 3, "ok": true}}`
	if got != want {
		t.Fatalf("line==this: gsaDumps 得 %q 要 %q", got, want)
	}
}

// ★ 自检：主件自检全过（任何匹配器/判据体被哑化 ⇒ 自检自身即红）。
func TestGsaSelfTestPasses(t *testing.T) {
	var buf bytes.Buffer
	if rc := gsaSelfTest(&buf); rc != 0 {
		t.Fatalf("line==this: 自检应 rc=0，得 %d:\n%s", rc, buf.String())
	}
	if !strings.Contains(buf.String(), "自检结论: 全过") {
		t.Fatalf("line==this: 自检应印「全过」:\n%s", buf.String())
	}
}

// ★ 端到端（显式 --source/--repo-root，不依赖摆位）：R1 新增红 · R3 红 · R5 新增红 · R2 只报告。
func TestGsaEntryFixtureRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/core/cmd/zerg", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/core/cmd/zerg/main.go", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rows := strings.Join([]string{
		`{"id": "GAP-20260928-02", "state": "已解", "solved_evidence": "缺证据", "found_at": "2026-09-28T00:00:00"}`,
		`{"id": "GAP-20260928-04", "state": "已解", "solved_evidence": "` + "`zerg x`" + ` rc=0 5 条", "found_at": "2026-09-28", "notes": ["仍缺：前提还在"]}`,
		`{"id": "GAP-20260928-05", "state": "仍缺", "unit": "core/cmd/zerg/main.go"}`,
		`{"id": "GAP-20260928-06", "state": "已解", "solved_evidence": "现读 ` + "`core/cmd/zerg/ghost.go`" + ` rc=0 12 条", "found_at": "2026-09-28T00:00:00"}`,
	}, "\n") + "\n"
	src := root + "/rows.jsonl"
	if err := os.WriteFile(src, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := gsaRunEntry(t, "--source", src, "--repo-root", root)
	if rc != 1 {
		t.Fatalf("line==this: 夹具应 rc=1，得 %d:\n%s", rc, out)
	}
	for _, want := range []string{
		"✗ [R1] GAP-20260928-02【已解】证据缺 命令/退码/读数（新增 ⇒ 判红）",
		"✗ [R3] GAP-20260928-04【已解】可见 notes 末条含「仍缺」⇒ 同条自相矛盾",
		"✗ [R5] GAP-20260928-06 销案证据点名仓内路径 1 条不在盘：core/cmd/zerg/ghost.go（新增 ⇒ 判红）",
		"R2  : 只报告 1 条",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("line==this: 人读输出缺 %q:\n%s", want, out)
		}
	}
	// 机读档：JSON 逐字节（同一夹具）。
	rc2, out2, _ := gsaRunEntry(t, "--source", src, "--repo-root", root, "--json")
	if rc2 != 1 {
		t.Fatalf("line==this: --json 应 rc=1，得 %d", rc2)
	}
	if !strings.HasPrefix(out2, `{"schema": "zerg/v1", "kind": "GapStateVsArtifact", "tool": "check-gap-state-vs-artifact.py v1.0.0", "items": [`) {
		t.Fatalf("line==this: --json 前缀不符:\n%s", out2)
	}
	if !strings.Contains(out2, `"problems": [], "reports": `) {
		t.Fatalf("line==this: --json 缺 problems/reports 段:\n%s", out2)
	}
}

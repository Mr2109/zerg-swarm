package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cafbEnv —— 「假仓 + 假状态目录」夹具（与 py 自检同形 · 全真目录 · ✗ 无符号链接）。
type cafbEnv struct {
	repo, state, gate string
}

func cafbFixGo(withMark, badAudit, noOwn bool) string {
	fields := ""
	for _, t := range cafbTicketEight {
		fields += "\t" + t + " string `json:\"" + t + "\"`\n"
	}
	sig := "\tsig string `json:\"sig\"`\n"
	mark := "\t" + cafbMarkField + " bool `json:\"" + cafbMarkField + "\"`\n"
	if withMark {
		fields = strings.ReplaceAll(fields, sig, sig+mark)
	}
	auditLast := mark
	if badAudit {
		auditLast = "\tapproval string `json:\"approval\"`\n"
	}
	src := "package main\n\n" +
		"type approvalTicketFile struct {\n" + fields + "}\n\n" +
		"type approvalAuditLine struct {\n" +
		"\tat string `json:\"at\"`\n" +
		"\tevent string `json:\"event\"`\n" +
		"\ttool string `json:\"tool\"`\n" +
		"\tapproval_path string `json:\"approval_path\"`\n" +
		auditLast + "}\n"
	if !noOwn {
		src += "var approveAuditOwnFields = []string{\"at\", \"event\", \"tool\", \"approval_path\", \"" + cafbMarkField + "\"}\n"
	}
	return src
}

func cafbRow(mark string) string {
	if mark == "" {
		return `{"event": "` + cafbEvent + `"}`
	}
	return `{"event": "` + cafbEvent + `", "` + cafbMarkField + `": ` + mark + `}`
}

func cafbSetup(t *testing.T, goBody string, rows []string, ticket, audit, stateIsFile bool) cafbEnv {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(filepath.Join(repo, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if stateIsFile {
		if err := os.WriteFile(state, []byte("not-a-dir\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(filepath.Join(state, "approvals"), 0o755); err != nil {
			t.Fatal(err)
		}
		if audit {
			body := ""
			for _, r := range rows {
				body += r + "\n"
			}
			if err := os.WriteFile(filepath.Join(state, "edit_audit.jsonl"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if ticket {
			if err := os.WriteFile(filepath.Join(state, "approvals", "dev_edit.json"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "core", "cmd", "zerg", "family_approve.go"), []byte(goBody), 0o644); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(root, "check-approve-backfill.py")
	if err := os.WriteFile(gate, []byte("#!/usr/bin/env python3\n"+cafbConstName+" = \""+strings.Repeat("0", 64)+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cafbEnv{repo: repo, state: state, gate: gate}
}

func cafbRun(argv ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := checkApproveBackfillGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// TestCheckApproveBackfillPyJoin —— ★ 三类语义差定向单测（空段 / 后段绝对 / 不归一 `..`）。
func TestCheckApproveBackfillPyJoin(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a", ""}, "a/"},            // 空段：py 补分隔符（filepath.Join 得 "a"）
		{[]string{"", "a"}, "a"},             // py join('','a')=="a"
		{[]string{"a", "", "b"}, "a/b"},      // 中间空段
		{[]string{"a", "/b"}, "/b"},          // 后段绝对丢前段（filepath.Join 得 "a/b"）
		{[]string{"a", "..", "b"}, "a/../b"}, // 不归一 `..`（filepath.Join 得 "b"）
		{[]string{"core", "cmd", "zerg", "family_approve.go"}, "core/cmd/zerg/family_approve.go"},
		{[]string{"approvals", "dev_edit.json"}, "approvals/dev_edit.json"},
		{[]string{"", ""}, ""},
	}
	for _, c := range cases {
		if got := cafbPyJoin(c.in...); got != c.want {
			t.Errorf("pyJoin(%q) = %q · 要 %q", c.in, got, c.want)
		}
	}
}

// TestCheckApproveBackfillPyStrip —— ★ py `str.strip()` 是 Unicode 语义（含 `\x1c-\x1f`）≠ `strings.TrimSpace`。
func TestCheckApproveBackfillPyStrip(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  a\n", "a"},
		{"\x1ca\x1f", "a"}, // ✗ strings.TrimSpace 不动它；py 会剥掉
		{"\u00a0a\u3000", "a"},
		{"", ""},
		{"\x1c", ""},
		{"a\x1db", "a\x1db"}, // 中间不动
	}
	for _, c := range cases {
		if got := cafbPyStrip(c.in); got != c.want {
			t.Errorf("pyStrip(%q) = %q · 要 %q", c.in, got, c.want)
		}
	}
}

// TestCheckApproveBackfillFindConstant —— 常量行正则（`(?m)^…$` · 无 `\d`/`\s` 展开面）+ 行号。
func TestCheckApproveBackfillFindConstant(t *testing.T) {
	src := "#!/usr/bin/env python3\n# \u6ce8\u91ca\n" + cafbConstName + " = \"" + strings.Repeat("a", 64) + "\"\nprint(1)\n"
	lit, line, start, end, ok := cafbFindConstant(src)
	if !ok || lit != strings.Repeat("a", 64) || line != 3 || end-start != 64 {
		t.Fatalf("findConstant 读数不对：ok=%v lit=%q line=%d span=%d", ok, lit, line, end-start)
	}
	// 负控：`$` 是行尾锚 —— 常量行尾巴多了字符 ⇒ 解不出来（✗ 不许放宽成子串命中）
	if _, _, _, _, ok2 := cafbFindConstant(cafbConstName + " = \"" + strings.Repeat("a", 64) + "\" extra\n"); ok2 {
		t.Fatal("负控失败：常量行尾部多字还命中（`$` 锚被放宽）")
	}
	// 负控：大写十六进制不在 `[0-9a-f]` 内 ⇒ 解不出来
	if _, _, _, _, ok3 := cafbFindConstant(cafbConstName + " = \"" + strings.Repeat("A", 64) + "\"\n"); ok3 {
		t.Fatal("负控失败：大写 hex 还命中")
	}
}

// TestCheckApproveBackfillGatePort —— 入口正控 + 成对负控（带牙：哑化任一判据 ⇒ 必 FAIL ＋行号）。
func TestCheckApproveBackfillGatePort(t *testing.T) {
	type tc struct {
		name     string
		body     string
		rows     []string
		noTicket bool
		noAudit  bool
		stateF   bool
		extra    []string
		wantRC   int
		word     string
		errW     bool
	}
	cases := []tc{
		{"负控 B1 明标进件字段面", cafbFixGo(true, false, false), []string{cafbRow("false")}, false, false, false, nil, 1, "B1 破", false},
		{"负控 B4 台帐一条补签", cafbFixGo(false, false, false), []string{cafbRow("false"), cafbRow("true")}, false, false, false, nil, 1, "B4 破", false},
		{"负控 B4 点名条数 1 条", cafbFixGo(false, false, false), []string{cafbRow("false"), cafbRow("true")}, false, false, false, nil, 1, "1 条", false},
		{"负控 B2 蹭 before_sha256", cafbFixGo(false, true, false), []string{cafbRow("false")}, false, false, false, nil, 1, "B2 破", false},
		{"正控 干净夹具唯一红=B5", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, nil, 1, "B5 破", false},
		{"正控 干净夹具植入面零命中", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, nil, 1, "⇒ 门禁红（rc=1）：1 条", false},
		{"负控 审计件缺 ⇒ rc=2 未取数", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, true, false, nil, 2, "未取数", false},
		{"负控 在册件缺 ⇒ rc=2", cafbFixGo(false, false, false), []string{cafbRow("false")}, true, false, false, nil, 2, "在册那枚批准件不在", false},
		{"负控 状态目录是件 ⇒ rc=2", cafbFixGo(false, false, false), nil, false, false, true, nil, 2, "状态目录不在", false},
		{"负控 own_fields 不在 ⇒ rc=2", cafbFixGo(false, false, true), []string{cafbRow("false")}, false, false, false, nil, 2, "approveAuditOwnFields", false},
		{"正控 空台帐 ⇒ 0 行 + 口径行", cafbFixGo(false, false, false), []string{}, false, false, false, nil, 1, "现跑 0 行", false},
		{"正控 台帐 null 行 ⇒ 不炸 · 计行不计数", cafbFixGo(false, false, false), []string{"null", cafbRow("false")}, false, false, false, nil, 1, "现跑 2 行", false},
		{"负控 --json 无机器读面", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, []string{"--json"}, 2, "无机器读面", true},
		{"负控 根不在 ⇒ rc=2", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, []string{"--root", "no-such-root"}, 2, "不出结论", false},
		{"正控 --self-test 只跑自检", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, []string{"--self-test"}, 0, "自检：16 格过", false},
		{"负控 互斥 ⇒ rc=2", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, []string{"--emit-constant", "--refresh-constant"}, 2, "互斥", true},
		{"负控 --yes 单飞 ⇒ rc=2", cafbFixGo(false, false, false), []string{cafbRow("false")}, false, false, false, []string{"--yes"}, 2, "只跟", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := cafbSetup(t, c.body, c.rows, !c.noTicket, !c.noAudit, c.stateF)
			base := []string{"--root", e.repo, "--state", e.state, "--gate", e.gate}
			hasST := false
			for _, x := range c.extra {
				if x == "--self-test" {
					hasST = true
				}
			}
			if !hasST {
				base = append(base, "--no-self-test")
			}
			argv := append(append([]string{}, base...), c.extra...)
			rc, out, errb := cafbRun(argv...)
			if rc != c.wantRC {
				t.Fatalf("rc=%d 要 %d\n out=%s\n err=%s", rc, c.wantRC, out, errb)
			}
			hay := out
			if c.errW {
				hay = errb
			}
			if !strings.Contains(hay, c.word) {
				t.Fatalf("缺关键字 %q\n out=%s\n err=%s", c.word, out, errb)
			}
		})
	}
}

// TestCheckApproveBackfillEmitAndRefresh —— 常量通道：emit 只读 / 干跑零写入 / 真写等长最小替换 / 仓内拒写。
func TestCheckApproveBackfillEmitAndRefresh(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "check-approve-backfill.py")
	oldSHA := strings.Repeat("0", 64)
	newSHA := strings.Repeat("1", 64)
	src := "#!/usr/bin/env python3\n# \u6ce8\u91ca\n" + cafbConstName + " = \"" + oldSHA + "\"\nprint(\"tail\")\n"
	if err := os.WriteFile(gate, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(gate)

	var sb strings.Builder
	if rc := cafbEmitConstant(&sb, gate, &newSHA); rc != 1 {
		t.Fatalf("emit rc=%d 要 1", rc)
	}
	after, _ := os.ReadFile(gate)
	if string(after) != string(before) {
		t.Fatal("负控失败：emit 档**动了件**（应只读）")
	}
	if !strings.Contains(sb.String(), newSHA) || !strings.Contains(sb.String(), oldSHA) {
		t.Fatal("emit 档没同时印出现值与应有值")
	}

	dry := filepath.Join(dir, "bak-dry")
	sb.Reset()
	if rc := cafbRefreshConstant(&sb, gate, &newSHA, false, dry, ""); rc != 0 {
		t.Fatalf("干跑 rc=%d 要 0", rc)
	}
	if after2, _ := os.ReadFile(gate); string(after2) != string(before) {
		t.Fatal("负控失败：干跑档写了件（应零写入）")
	}
	if _, err := os.Stat(dry); err == nil {
		t.Fatal("负控失败：干跑档建了备份目录")
	}

	bdir := filepath.Join(dir, "bak")
	repoFx := filepath.Join(dir, "repo")
	sb.Reset()
	if rc := cafbRefreshConstant(&sb, gate, &newSHA, true, bdir, repoFx); rc != 0 {
		t.Fatalf("真写 rc=%d 要 0", rc)
	}
	idx := strings.Index(src, oldSHA)
	expect := []byte(src[:idx] + newSHA + src[idx+len(oldSHA):])
	got, _ := os.ReadFile(gate)
	if string(got) != string(expect) {
		t.Fatal("负控失败：替换不是**等长最小替换**")
	}
	ents, _ := os.ReadDir(bdir)
	if len(ents) != 1 {
		t.Fatalf("备份数=%d 要 1", len(ents))
	}
	bak, _ := os.ReadFile(filepath.Join(bdir, ents[0].Name()))
	if string(bak) != string(before) {
		t.Fatal("负控失败：备份不是改前整件")
	}

	sb.Reset()
	if rc := cafbRefreshConstant(&sb, gate, &oldSHA, true, filepath.Join(repoFx, "bak-in-repo"), repoFx); rc != 2 {
		t.Fatalf("仓内备份 rc=%d 要 2（拒写）", rc)
	}
	if !strings.Contains(sb.String(), "仓内") {
		t.Fatal("仓内拒写没逐字点名")
	}
}

// TestCheckApproveBackfillRetired —— 退役态（G10 · 2026-10-07）：真源件（原 py）不在盘 ⇒ 回落到
// **内化进本件**的常量 `cafbDevEditSHA256`（✗ 不 fail-closed）；件仍在盘但常量行解不出仍 fail-closed。
// ★ 哑化负控：抽掉 `cafbScan` 的退役态分支（`if !ok && !cafbExists(gatePath)`）⇒ ② 格 rc=2 ⇒ 本用例必红。
func TestCheckApproveBackfillRetired(t *testing.T) {
	e := cafbSetup(t, cafbFixGo(false, false, false), []string{cafbRow("false")}, true, true, false)
	base := []string{"--root", e.repo, "--state", e.state, "--no-self-test"}
	run := func(gate string) (int, string) {
		rc, out, _ := cafbRun(append(append([]string{}, base...), "--gate", gate)...)
		return rc, out
	}
	// ① 真源件在盘（常量 = 全 0 ≠ 在册件 sha）⇒ 现读登记值 ⇒ B5 破 ⇒ rc=1
	if rc, out := run(e.gate); rc != 1 || !strings.Contains(out, "B5 破") {
		t.Fatalf("在盘态失败：rc=%d out=%q", rc, out)
	}
	// ② 真源件不在盘（退役态）⇒ 回落内化常量 ⇒ 仍到判据面（B5 破）· rc=1（✗ 不得 rc=2）
	if rc, out := run(filepath.Join(e.repo, "no-such-gate-xyz.py")); rc != 1 || !strings.Contains(out, "B5 破") {
		t.Fatalf("退役态失败（应回落内化常量 · ✗ 不 fail-closed）：rc=%d out=%q", rc, out)
	}
	// ③ 件在盘但常量行解不出（坏件）⇒ 仍 fail-closed rc=2（退役态分支不吃这条）
	if rc, out := run(filepath.Join(e.repo, "core", "cmd", "zerg", "family_approve.go")); rc != 2 || !strings.Contains(out, "不出结论") {
		t.Fatalf("坏件态失败（应 fail-closed rc=2）：rc=%d out=%q", rc, out)
	}
}

// TestCheckApproveBackfillSelfTest —— 自检 16 格必须全过（哑化判据面 ⇒ 本格红）。
func TestCheckApproveBackfillSelfTest(t *testing.T) {
	rc, out, _ := cafbRun("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test rc=%d 要 0\n%s", rc, out)
	}
	if !strings.Contains(out, "自检：16 格过 · 0 格不过") {
		t.Fatalf("自检格数不对：\n%s", out)
	}
}

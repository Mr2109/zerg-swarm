// check_manifest_freshness_two_criteria_test.go —— Go 真搬的**成对负控**定向单测（M2 批）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckManifestFreshnessTwoCriteriaGatePort（定向：go test -run TestCheckManifestFreshnessTwoCriteriaGatePort）。
//
// 夹具策略：本件真跑 `git` ⇒ 用例在 `t.TempDir()`（**仓外**）里 `git init` + `commit` 造一棵真仓，
// 并按 py 的 `HERE` 形态把**真源与同族 py 两份原件**拷进 `<夹具>/scripts/gates/`（每夹具自放），
// 再用 `--root <夹具>` 指过去。✗ 不碰真仓、✗ 不写任何真实状态目录。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cmfTestRepoRoot —— 由 cwd 上溯找同族 py 的位置（与入口 `--root` 缺省推导同一枚标记）。
func cmfTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cur := d
	for i := 0; i < 32; i++ {
		if cmfFileExists(filepath.Join(cur, cmfSelfRelPath)) {
			return cur
		}
		p := filepath.Dir(cur)
		if p == cur {
			break
		}
		cur = p
	}
	t.Fatalf("找不到同族 py 所在仓根（由 %s 上溯）", d)
	return ""
}

// cmfTestFixture —— 在 t.TempDir() 下按 py 的 HERE 形态造一棵夹具树；withTruth=false ⇒ 不放真源件。
func cmfTestFixture(t *testing.T, withTruth bool) string {
	t.Helper()
	src := cmfTestRepoRoot(t)
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile := func(name string) {
		b, err := os.ReadFile(filepath.Join(src, "scripts", "gates", name))
		if err != nil {
			t.Fatalf("缺同族 py 原件 %s：%v", name, err)
		}
		if err := os.WriteFile(filepath.Join(g, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copyFile("check-manifest-freshness-two-criteria.py")
	if withTruth {
		copyFile("check-manifest-freshness.py")
	}
	return root
}

// cmfTestGitInit —— 只在**仓外夹具**里 git init/commit（✗ 不碰真仓）。
func cmfTestGitInit(t *testing.T, root string) {
	t.Helper()
	cmds := [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			t.Fatalf("夹具 git %v 失败：%v", args, err)
		}
	}
}

func cmfTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkManifestFreshnessTwoCriteriaGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckManifestFreshnessTwoCriteriaGatePort(t *testing.T) {
	root := cmfTestFixture(t, true)
	cmfTestGitInit(t, root)
	rootNoTruth := cmfTestFixture(t, false)
	cmfTestGitInit(t, rootNoTruth)
	rootNoGit := cmfTestFixture(t, true) // 有件无 .git ⇒ 「取不到」档

	// ── 正控（必然判绿）：真仓夹具 + 真源在场 ⇒ 两条判据分列 ⇒ rc=0 ─────────
	rcGreen, outGreen, errGreen := cmfTestRun(t, "--root", root)
	if rcGreen != 0 {
		t.Fatalf("正控失败：应 rc=0，得 rc=%d\nout=%q\nerr=%q", rcGreen, outGreen, errGreen)
	}
	if !strings.Contains(outGreen, "⇒ 两条判据分列=green · 源级分列=✓ · 真源自检=0 · rc=0") {
		t.Fatalf("正控失败：末行不符 ⇒ out=%q", outGreen)
	}
	if !strings.Contains(outGreen, "── 真源报出的告警行（原样 · 逐行）──") {
		t.Fatalf("正控失败：缺告警行小节 ⇒ out=%q", outGreen)
	}

	// ── 机器面：--json 子集（顺序 = 给定顺序 · py json.dumps 分隔符逐字）──────
	rcJ, outJ, _ := cmfTestRun(t, "--root", root, "--json", "hits,rc,verdict")
	if rcJ != 0 || !strings.Contains(outJ, `{"hits": [], "rc": 0, "verdict": "green"}`) {
		t.Fatalf("--json 子集失败：rc=%d out=%q", rcJ, outJ)
	}
	rcJA, outJA, _ := cmfTestRun(t, "--root", root, "--json", "rc,criteria,source_lines")
	if rcJA != 0 || !strings.Contains(outJA, `{"rc": 0, "criteria": {"source_sha_ne_head": true, "commit_ne_source_sha": true}, "source_lines": {"a": [9, 99, 193], "b": [111, 116, 185]}}`) {
		t.Fatalf("--json 全集失败：rc=%d out=%q", rcJA, outJA)
	}

	// ── Q-176：表外字段 ⇒ rc=2 ＋ stderr 印可选字段表（≠ 零命中两态必须分得开）─
	rcQ, outQ, errQ := cmfTestRun(t, "--root", root, "--json", "nope,rc")
	if rcQ != 2 || outQ != "" || !strings.Contains(errQ, "⛔ `--json` 不认的字段名：`nope`") ||
		!strings.Contains(errQ, "可选字段表：`rc` · `verdict`") {
		t.Fatalf("Q-176 失败：rc=%d out=%q err=%q", rcQ, outQ, errQ)
	}

	// ── --self-test 成对负控自身必须全过（防恒绿/恒红）─────────────────────
	rcST, outST, _ := cmfTestRun(t, "--root", root, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过（断言 10 条 · 失败 0 条）") {
		t.Fatalf("--self-test 失败：rc=%d out=%q", rcST, outST)
	}
	// 成对（✗ 禁恒绿）：真仓夹具 ⇒ 绿；下面「取不到」夹具 ⇒ 红 —— 两码必须不同。
	if rcGreen == 0 && rcST != 0 {
		t.Fatalf("成对断言失败：正控 rc=%d 与自检 rc=%d", rcGreen, rcST)
	}

	// ── 取不到那棵树的 git 面（有件无 .git）⇒ py 抛 Unsupported：stdout 空 · rc=1 ─
	rcNoGit, outNoGit, errNoGit := cmfTestRun(t, "--root", rootNoGit)
	if rcNoGit != 1 || outNoGit != "" {
		t.Fatalf("非 git 仓档失败：应 rc=1 且 stdout 空，得 rc=%d out=%q", rcNoGit, outNoGit)
	}
	if !strings.Contains(errNoGit, "不给结论") {
		t.Fatalf("非 git 仓档失败：stderr 未走声明文案 ⇒ %q", errNoGit)
	}
	// 成对：绿档 rc=0 ⟷ 取不到档 rc=1 必须不同码。
	if rcGreen == rcNoGit {
		t.Fatalf("成对断言失败：正控 rc=%d 与取不到 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcNoGit)
	}

	// ── 真源件不在场 ⇒ **不给结论**（rc=2 ＋ 缺件行；✗ 不许当绿）─────────────
	rcMiss, outMiss, _ := cmfTestRun(t, "--root", rootNoTruth)
	if rcMiss != 2 || !strings.Contains(outMiss, "缺件：真源") || !strings.Contains(outMiss, "⇒ rc=2") {
		t.Fatalf("真源缺件档失败：应 rc=2，得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── --list-rules（人读面）⇒ rc=0 ＋ 单一口径行 ─────────────────────────
	rcLR, outLR, _ := cmfTestRun(t, "--root", root, "--list-rules")
	if rcLR != 0 || !strings.Contains(outLR, "单一口径：现调 scripts/gates/check-manifest-freshness.py 的 diagnose()") {
		t.Fatalf("--list-rules 失败：rc=%d out=%q", rcLR, outLR)
	}

	// ── 判据函数的直接负控（有牙：合并成一句 ⇒ hit 且点名 C2；只报一条 ⇒ hit）──
	merged := []string{"清单出自另一棵树且声明身份与真实来源不一致：source_sha=0000"}
	if v, h, _ := cmfJudgeLines(merged); v != "hit" || !cmfAnyContains(h, "C2 合并成一句") {
		t.Fatalf("judge 负控失败：合并句应 hit+C2，得 v=%q h=%v", v, h)
	}
	if v, _, _ := cmfJudgeLines([]string{"清单出自另一棵树：source_sha=0000"}); v != "hit" {
		t.Fatalf("judge 负控失败：只报判据① 应 hit，得 %q", v)
	}
	if v, _, _ := cmfJudgeLines(nil); v != "vacuous" {
		t.Fatalf("judge 负控失败：零告警行应 vacuous，得 %q", v)
	}
	// 区分度（恒绿/恒红当场抓）：同一函数对「分列」绿、对「合并」红。
	two := []string{"清单出自另一棵树：source_sha=0000", "声明身份与真实来源不一致：commit=z 而 source_sha=0"}
	vGood, _, _ := cmfJudgeLines(two)
	vBad, _, _ := cmfJudgeLines(merged)
	if vGood != "green" || vBad != "hit" {
		t.Fatalf("区分度失败：分列=%q 合并=%q（应 green/hit）", vGood, vBad)
	}
}

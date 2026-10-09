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

// cmfTestRepoRoot —— 由 cwd 上溯找同族标记所在仓根（与入口缺省推导同一枚标记）。
// ★ 退役稳态（B轨 · 2026-10-07）：认「自指 py **或** `cmfRootMarkerRel`」⇒ 自指 py 退役后仍能定位真仓。
func cmfTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cur := d
	for i := 0; i < 32; i++ {
		if cmfFileExists(filepath.Join(cur, cmfSelfRelPath)) || cmfFileExists(filepath.Join(cur, cmfRootMarkerRel)) {
			return cur
		}
		p := filepath.Dir(cur)
		if p == cur {
			break
		}
		cur = p
	}
	t.Fatalf("找不到同族标记所在仓根（由 %s 上溯）", d)
	return ""
}

// cmfTestFixture —— 在 t.TempDir() 下按 py 的 HERE 形态造一棵夹具树；withTruth=false ⇒ 不放真源件。
// ★ 可退役件（自指 py）在原件缺时不 fail（退役态夹具）；`cmfRootMarkerRel` 同拷（根上溯形态同真仓）。
func cmfTestFixture(t *testing.T, withTruth bool) string {
	t.Helper()
	src := cmfTestRepoRoot(t)
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	copyIfPresent := func(name string) {
		b, err := os.ReadFile(filepath.Join(src, "scripts", "gates", name))
		if err != nil {
			return // 退役件缺席 ⇒ 合法（✗ 不 fail-closed）
		}
		if err := os.WriteFile(filepath.Join(g, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copyIfPresent("check-manifest-freshness-two-criteria.py")
	copyIfPresent("precommit-gates.sh")
	if withTruth {
		// ★ 内化金标哨兵（DEV-2143 同口径 · 甲类同族三处之一）：真源 `check-manifest-freshness.py`
		// 已退场（2026-10-07 M2 退役批）⇒ 旧哨兵「真源 py 缺席 ∧ Go 半在盘 ⇒ t.Skipf」恒真（整测零断言）。
		// 改锚其 **Go 半** `core/cmd/zerg/check_manifest_freshness.go`（本门已在进程内重建其 diagnose()）；
		// 找不到 = 真仓结构被破 ⇒ **fail-closed**（✗ 不留 Skip）。夹具落 Go 半 ⇒ 入口 `cmfTruthRetired`
		// 判「已退役」⇒ truthPresent=true · C3 改读 Go 半源码 ⇒ 判据照常出结论。
		b, err := os.ReadFile(filepath.Join(src, "core", "cmd", "zerg", "check_manifest_freshness.go"))
		if err != nil {
			t.Fatalf("真源 Go 半不在盘 ⇒ fail-closed：%v（✗ 不留 Skip）", err)
		}
		gd := filepath.Join(root, "core", "cmd", "zerg")
		if err := os.MkdirAll(gd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gd, "check_manifest_freshness.go"), b, 0o644); err != nil {
			t.Fatal(err)
		}
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
	if rcJA != 0 || !strings.Contains(outJA, `{"rc": 0, "criteria": {"source_sha_ne_head": true, "commit_ne_source_sha": true}, "source_lines": {"a": [185, 345], "b": [199, 204, 332]}}`) {
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

// TestCheckManifestFreshnessTwoCriteriaRetired —— 退役态（B轨 · 2026-10-07）：本件同族 py 退役
// （不在盘）⇒ 判据照常出结论（✗ 不得 fail-closed / rc=2）。
//
//	① 根上溯：自指 py 缺席时靠退役稳态标记 `cmfRootMarkerRel` 仍能上溯到仓根（✗ 退回 cwd）；
//	② 两态逐字：同夹具「py 在盘」⟷「py 移开」⇒ rc/stdout/stderr 三面逐字同，且非 rc=2。
//
// ★ 哑化负控：抽掉 `cmfResolveRootFrom` 的 `|| cmfFileExists(…cmfRootMarkerRel)` 一支 ⇒ ① 断言红（带行号）。
func TestCheckManifestFreshnessTwoCriteriaRetired(t *testing.T) {
	// ① 根上溯（退役态：仓里只有退役稳态标记，✗ 无自指 py）。
	fx := t.TempDir()
	g := filepath.Join(fx, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(fx, "core", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := cmfResolveRootFrom(sub); got != fx {
		t.Fatalf("退役态根上溯失败：应 %s，得 %s ⇒ 自指 py 退役后上溯落空、退回 cwd ⇒ 真源找不到 ⇒ rc=2", fx, got)
	}

	// ② py 在盘 ⟷ py 移开：**同一夹具、同一 git 仓**（⇒ 分支头哈希不变）三面逐字同。
	fx2 := cmfTestFixture(t, true)
	cmfTestGitInit(t, fx2)
	rcOn, outOn, errOn := cmfTestRun(t, "--root", fx2)
	// 强制退役态：把自指 py 从夹具移开（若已随真仓拷入；✗ 不动真仓）。
	_ = os.Remove(filepath.Join(fx2, "scripts", "gates", "check-manifest-freshness-two-criteria.py"))
	rcRe, outRe, errRe := cmfTestRun(t, "--root", fx2)
	if rcRe != rcOn || outRe != outOn || errRe != errOn {
		t.Fatalf("退役态读数与在盘态不一致：\n-- 在盘 rc=%d --\n%s\nstderr=%q\n-- 退役 rc=%d --\n%s\nstderr=%q",
			rcOn, outOn, errOn, rcRe, outRe, errRe)
	}
	if rcRe == 2 {
		t.Fatalf("退役态 fail-closed（rc=2）—— 件退役不得让判据判不出")
	}
}

// TestCheckManifestFreshnessTwoCriteriaTruthRetired —— **真源**退役态（B轨 · 2026-10-07 · 类①）：
// 真源 `check-manifest-freshness.py` 退役（py 缺席 · 其 Go 半在盘）⇒ 判据照常出结论（✗ 不得 rc=2）。
//
//	① 退役（py 缺席 · Go 半在盘）⇒ rc=0，且 C3 改读真源 **Go 半源码**（人读面「真源：core/cmd/zerg/check_manifest_freshness.go（在场）」）；
//	② 成对负控 · **真消失**（两半皆缺）⇒ rc=2（fail-closed 一格不变）；
//	③ 成对正控 · py 在盘 ⇒ rc=0（逐字沿用原语义）。
//
// ★ 哑化负控（带行号）：抽掉入口 `truthPresent := cmfFileExists(truthPath) || cmfTruthRetired(repoRoot)`
// 的 `|| cmfTruthRetired(repoRoot)` 一支（`check_manifest_freshness_two_criteria.go:188`）⇒ ① 断言当场红（rc=2）。
func TestCheckManifestFreshnessTwoCriteriaTruthRetired(t *testing.T) {
	src := cmfTestRepoRoot(t)

	build := func(withTruthPy, withGoHalf bool) string {
		root := t.TempDir()
		g := filepath.Join(root, "scripts", "gates")
		if err := os.MkdirAll(g, 0o755); err != nil {
			t.Fatal(err)
		}
		// 根上溯标记（退役稳态）——本用例一律显式 `--root`，但保持与真仓同形。
		if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if withGoHalf {
			gd := filepath.Join(root, "core", "cmd", "zerg")
			if err := os.MkdirAll(gd, 0o755); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(src, "core", "cmd", "zerg", "check_manifest_freshness.go"))
			if err != nil {
				t.Fatalf("缺真源 Go 半原件：%v", err)
			}
			if err := os.WriteFile(filepath.Join(gd, "check_manifest_freshness.go"), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if withTruthPy {
			b, err := os.ReadFile(filepath.Join(src, "scripts", "gates", "check-manifest-freshness.py"))
			if err != nil {
				t.Fatalf("缺真源 py 原件：%v", err)
			}
			if err := os.WriteFile(filepath.Join(g, "check-manifest-freshness.py"), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmfTestGitInit(t, root)
		return root
	}

	// ① 退役（py 缺席 · Go 半在盘）⇒ rc=0 且 C3 读 Go 半源码。
	fxRetired := build(false, true)
	rcR, outR, errR := cmfTestRun(t, "--root", fxRetired)
	if rcR != 0 {
		t.Fatalf("① 真源退役态应 rc=0（判据照常出结论），得 rc=%d\nout=%s\nerr=%s", rcR, outR, errR)
	}
	if !strings.Contains(outR, "真源：core/cmd/zerg/check_manifest_freshness.go（在场）") {
		t.Fatalf("① 退役态 C3 未改读真源 Go 半 ⇒ out=%s", outR)
	}
	if strings.Contains(outR, "缺件：真源") {
		t.Fatalf("① 退役态仍落 fail-closed 缺件分支 ⇒ out=%s", outR)
	}
	// 退役态 `--self-test` 亦须过（C3/C4 走 Go 半与进程内自检）。
	if rcST, outST, _ := cmfTestRun(t, "--root", fxRetired, "--self-test"); rcST != 0 {
		t.Fatalf("① 退役态 --self-test 应 rc=0，得 rc=%d out=%s", rcST, outST)
	}

	// ② 成对负控 · 真消失（两半皆缺）⇒ rc=2（fail-closed 一格不变）。
	fxGone := build(false, false)
	rcG, outG, _ := cmfTestRun(t, "--root", fxGone)
	if rcG != 2 || !strings.Contains(outG, "缺件：真源") {
		t.Fatalf("② 真消失应 fail-closed rc=2，得 rc=%d out=%s", rcG, outG)
	}

	// ③ 成对正控 · py 在盘 ⇒ rc=0 且仍是 py 源（原语义逐字沿用）。
	//   ★ 真源 py 退役容忍（2026-10-07 公开面退 10 件批 · 同族 `mfpTestFixture` 口径）：真源 py 退役
	//   （不在盘）∧ 其 Go 半在盘 ⇒ 本片无「py 在盘」正控对象，如实跳过本片（①② 已覆退役态/真消失态）；
	//   两半皆缺（真消失）已由 ② 的成对负控兜住。
	truthPy := filepath.Join(src, "scripts", "gates", "check-manifest-freshness.py")
	if !cmfFileExists(truthPy) {
		if !cmfFileExists(filepath.Join(src, "core", "cmd", "zerg", "check_manifest_freshness.go")) {
			t.Fatalf("③ 真源 py 与 Go 半皆不在盘（真消失）⇒ fail-closed，不应走到本片")
		}
		t.Logf("③ 跳过：真源 py %s 已退场（Go 半在盘）⇒ 无「py 在盘」正控对象", truthPy)
		return
	}
	fxPy := build(true, true)
	rcP, outP, _ := cmfTestRun(t, "--root", fxPy)
	if rcP != 0 || !strings.Contains(outP, "check-manifest-freshness.py（在场）") {
		t.Fatalf("③ py 在盘应 rc=0 且读 py 源，得 rc=%d out=%s", rcP, outP)
	}
}

// ── 内化金标（DEV-2143 同口径）：旧 py 对拍退场 ⇒ 逐格金标收 Go 常量 · 两态自证（✗ 不 Skip）─────
//
// 为什么这么测（与 `check_manifest_freshness_test.go` 的 `TestCheckManifestFreshnessGoldenInternalized`
// / `...GoldenTwoStates` 同族）：本件金标原经「真源 py 在盘」这一前置才成立（夹具拷 `check-manifest-freshness.py`）；
// 该 py 已退场（2026-10-07 M2 退役批）⇒ C3 源级分列改读真源 **Go 半**（`cmfTruthRetired` 内化）。
// 按 DEV-2143 先例把金标真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 由「Go 半现役行为」现算再冻）：逐块「输入（合成清单 + argv `--root`）
//	  ⇒ (rc, stdout 关键块)」由 `cmfTestRun` 现跑捕获（一次性现算、只读夹具、✗ 不落仓），冻成下表；
//	  **本机时点类量**比对前归一化（✗ 不冻结）：夹具根（t.TempDir() 随机路径）⇒ `<FX>`。
//	  其中 C3 两行 `[185, 345]` / `[199, 204, 332]` = 真源 Go 半里两条判据标记所在的**现役源码行**。
//
// 两态（✗ 不许 Skip）：A 态 = 夹具真源 **Go 半**在盘（退役 py 不在盘）；B 态 = 合成「旧态」夹具
// （退役真源 py 以**两行打标桩**放回 · 两条判据分列于不同源码行）⇒ 两态**判据结论同**（rc 同 · 判决尾行逐字同）。
//
// ★ 哑化负控（"牙" · 见交付读数）：抽掉 `check_manifest_freshness_two_criteria.go:188` 的
//   `|| cmfTruthRetired(repoRoot)` 一支 ⇒ `TestCheckManifestFreshnessTwoCriteriaTruthRetired` ① 断言当场红（rc=2 · 带行号）。

// cmfGoldenNorm —— 归一本机时点量（夹具绝对路径）⇒ 金标跨机可复现。
func cmfGoldenNorm(s, root string) string {
	if root != "" {
		s = strings.ReplaceAll(s, root, "<FX>")
	}
	return s
}

// cmfRunGolden —— 金标自证：现建夹具（真源 Go 半）现跑本入口 · 归一 ⇒ 返回 (rc, 归一化 stdout)。
// stub != "" ⇒ 合成「旧态」：把退役真源 py 以打标桩放回（造「py 在盘」那一态）。
func cmfRunGolden(t *testing.T, stub string) (int, string) {
	t.Helper()
	root := cmfTestFixture(t, true)
	if stub != "" {
		if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "check-manifest-freshness.py"), []byte(stub), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmfTestGitInit(t, root)
	rc, out, _ := cmfTestRun(t, "--root", root)
	return rc, cmfGoldenNorm(out, root)
}

// TestCheckManifestFreshnessTwoCriteriaGoldenInternalized —— 内化金标自证（A 态：真仓 · 退役 py 不在盘）。
// ✗ 不 Skip、逐块真断言（✗ 不放宽为「只判非空」）。
func TestCheckManifestFreshnessTwoCriteriaGoldenInternalized(t *testing.T) {
	rc, out := cmfRunGolden(t, "")
	if rc != 0 {
		t.Fatalf("金标 rc=%d（冻值 0）\n%s", rc, out)
	}
	// 金标块（现算自证 · ✗ 手写）：真源内化面 / C1 结论 / C3 源级分列行号 / 判据尾行。
	want := []string{
		"真源：core/cmd/zerg/check_manifest_freshness.go（在场）",
		"   结论：green —— 两条判据分列（①1 行 · ②1 行 · 合并 0 行）",
		"   判据①「另一棵树」源码行：[185, 345]",
		"   判据②「声明身份与真实来源不一致」源码行：[199, 204, 332]",
		"   ✓ 两条在**不同源码行**（源头就是两条 statement）",
		"⇒ 两条判据分列=green · 源级分列=✓ · 真源自检=0 · rc=0",
	}
	for _, w := range want {
		if !strings.Contains(out, w+"\n") {
			t.Errorf("金标行缺失：%q\n--- 实得 ---\n%s", w, out)
		}
	}
}

// TestCheckManifestFreshnessTwoCriteriaGoldenTwoStates —— 两态判据结论同：A 态 = 夹具真源 Go 半（退役 py 不在盘）；
// B 态 = 合成「旧态」夹具（退役真源 py 以两行打标桩放回）⇒ rc 同 · 判决尾行逐字同（✗ 不 Skip）。
func TestCheckManifestFreshnessTwoCriteriaGoldenTwoStates(t *testing.T) {
	rcA, outA := cmfRunGolden(t, "")
	// 打标桩：两条判据各占一行（源级分列成立）⇒ B 态判决与 A 态同。
	stub := "# 判据① 另一棵树\n# 判据② 声明身份与真实来源不一致\n"
	rcB, outB := cmfRunGolden(t, stub)
	if rcA != rcB {
		t.Fatalf("两态 rc 不同：A=%d B=%d\n--- A ---\n%s\n--- B ---\n%s", rcA, rcB, outA, outB)
	}
	tail := "⇒ 两条判据分列=green · 源级分列=✓ · 真源自检=0 · rc=0\n"
	if !strings.Contains(outA, tail) {
		t.Fatalf("A 态判决尾行不符：\n%s", outA)
	}
	if !strings.Contains(outB, tail) {
		t.Fatalf("B 态判决尾行不符（py 在盘不得改判据结论）：\n%s", outB)
	}
}

// check_manifest_freshness_profile_test.go —— Go 真搬的**成对负控**定向单测（M2 批 · 甲案 C 类）。
//
// ✗ 禁恒绿：本件同时钉「必然判绿」「必然判红」「不给结论（rc=2）」三态，
// 任一态被写死（例如把 rc 恒置 0 或恒置 1）都必有断言失败。
// 用例名：TestCheckManifestFreshnessProfileGatePort（定向：go test -run TestCheckManifestFreshnessProfileGatePort）。
//
// 夹具策略：需要真仓的用例只在本测试的 t.TempDir()（**仓外**）里 `git init` + `commit`；
// ✗ 不碰真仓、✗ 不 git add/commit 到真仓、✗ 不写任何真实状态目录。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mfpTestInitWD —— 包初始化时的 cwd（= 被测包目录 · 早于任何 `t.Chdir`）。
// ★ 本件用例会 `t.Chdir(夹具根)` 让入口自根推导落在夹具上 ⇒ 仓根上溯**不得**随 Chdir 漂
// （否则第二次 `mfpTestFixture` 从夹具上溯、找不到同族标记 ⇒ 假红）。故上溯起点钉在包初始化 cwd。
var mfpTestInitWD, _ = os.Getwd()

// mfpTestRepoRoot —— 由**包初始化 cwd** 上溯找同族 py 原件所在仓根（与入口缺省推导同一枚标记）。
// ★ 退役稳态（B轨 · 2026-10-07）：认「自指 py **或** `cmfRootMarkerRel`（退役稳态标记）」⇒
// 自指 py 退役后仍能定位真仓根（与 `cmfTestRepoRoot` 同口径 · 复用同一枚标记，✗ 不新造第二套）。
func mfpTestRepoRoot(t *testing.T) string {
	t.Helper()
	d := mfpTestInitWD
	if d == "" {
		var err error
		if d, err = os.Getwd(); err != nil {
			t.Fatal(err)
		}
	}
	cur := d
	for i := 0; i < 32; i++ {
		if mfpFileExists(filepath.Join(cur, mfpSelfRel)) || mfpFileExists(filepath.Join(cur, cmfRootMarkerRel)) {
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

// mfpTestFixture —— 在 t.TempDir() 下落一份真仓（仓外 git init），并返回 (repoRoot, manifestDir)。
// withNeighbor=false ⇒ 把邻件真源从夹具的 scripts/gates/ 里挪走（用来钉「真源不在场」那一支）。
func mfpTestFixture(t *testing.T, withNeighbor bool) (string, string) {
	t.Helper()
	src := mfpTestRepoRoot(t)
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	// 退役稳态标记 `precommit-gates.sh`（仓根上溯用）＋ 保证夹具非空（夹具 `git commit` 需有件）。
	if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 自指 py（本件被判件 · `check-manifest-freshness-profile.py`）退役：Go 半在进程内自证 ⇒
	// 夹具不拷它即可（入口根推导由 cwd 回落夹具根 · `mfpResolveRoot`）。
	if b, err := os.ReadFile(filepath.Join(src, "scripts", "gates", "check-manifest-freshness-profile.py")); err == nil {
		if err := os.WriteFile(filepath.Join(g, "check-manifest-freshness-profile.py"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withNeighbor {
		// ★ 内化金标哨兵（DEV-2143 同口径 · 甲类同族三处之一）：邻件真源 `check-manifest-freshness.py`
		// 已退场（2026-10-07 M2 退役批）⇒ 旧哨兵「真源 py 缺席 ∧ Go 半在盘 ⇒ t.Skipf」恒真（整测零断言）。
		// 改锚其 **Go 半** `core/cmd/zerg/check_manifest_freshness.go`（本门已在进程内重建其 diagnose()）；
		// 找不到 = 真仓结构被破 ⇒ **fail-closed**（✗ 不留 Skip）。夹具落 Go 半 ⇒ 入口 `mfpNeighborRetired`
		// 判「已退役」⇒ neiPresent=true ⇒ 判据照常出结论。
		b, err := os.ReadFile(filepath.Join(src, "core", "cmd", "zerg", "check_manifest_freshness.go"))
		if err != nil {
			t.Fatalf("邻件真源 Go 半不在盘 ⇒ fail-closed：%v（✗ 不留 Skip）", err)
		}
		gd := filepath.Join(root, "core", "cmd", "zerg")
		if err := os.MkdirAll(gd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gd, "check_manifest_freshness.go"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"commit", "-q", "-m", "seed"},
	} {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("夹具 git %v 失败：%v\n%s", args, err, out)
		}
	}
	mfDir := filepath.Join(root, "mf")
	if err := os.MkdirAll(mfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, mfDir
}

func mfpTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mfpTestRun —— 真调入口，返回 (rc, stdout, stderr)。
func mfpTestRun(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkManifestFreshnessProfileGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func TestCheckManifestFreshnessProfileGatePort(t *testing.T) {
	repo, mfDir := mfpTestFixture(t, true)
	t.Chdir(repo) // 让入口的自根推导落在夹具上（同 py 的 HERE 面）

	// 夹具清单：source_sha 与夹具 HEAD 对齐 ⇒ 两条判据都「成立」。
	head := mfpGitTolerant(repo, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatal("夹具 HEAD 取不到")
	}
	fresh := filepath.Join(mfDir, "fresh.json")
	mfpTestWrite(t, fresh, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":false,"generated_at":"2026-09-23T00:00:00Z"}`)
	mismatch := filepath.Join(mfDir, "mismatch.json")
	mfpTestWrite(t, mismatch, `{"commit":"`+head+`","source_sha":"ffffffffffff","dirty":false}`)
	dirty := filepath.Join(mfDir, "dirty.json")
	mfpTestWrite(t, dirty, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":true}`)
	bad := filepath.Join(mfDir, "bad.json")
	mfpTestWrite(t, bad, "{not json")

	// ① 必然判绿：dev 档 + 新鲜清单（两条判据皆「成立」）⇒ rc=0。
	rc, out, _ := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("① dev+新鲜 应 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "成立") || !strings.Contains(out, "rc=0") {
		t.Fatalf("① 判定行不对：\n%s", out)
	}

	// ② 必然判红：public 档 + source_sha 不符 ⇒ rc=1（且必须点明「不是 stale 告警，是红」）。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", mismatch, "--repo", "", "--release-head", head})
	if rc != 1 {
		t.Fatalf("② public+不符 应 rc=1，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "**阻断**") || !strings.Contains(out, "用 A 打包、声称是 B") {
		t.Fatalf("② 阻断判定行不对：\n%s", out)
	}

	// ③ 同一条命中在 dev 档只告警 ⇒ rc=0（档位真的改退码，不是写死）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", mismatch, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("③ dev+不符 应 rc=0（只告警），实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "**只告警**") {
		t.Fatalf("③ 告警判定行不对：\n%s", out)
	}

	// ④ public + dirty=true ⇒ 命中 J2 ⇒ rc=1（第二条判据也真的参与退码）。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", dirty, "--repo", "", "--release-head", head})
	if rc != 1 {
		t.Fatalf("④ public+dirty 应 rc=1，实得 rc=%d\n%s", rc, out)
	}

	// ⑤ 不给结论：清单件读不到 ⇒ rc=2（且必须自陈「不给结论」）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", filepath.Join(mfDir, "nope.json")})
	if rc != 2 {
		t.Fatalf("⑤ 缺件 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "缺件") || !strings.Contains(out, "不许静默放绿") {
		t.Fatalf("⑤ 缺件文案不对：\n%s", out)
	}

	// ⑥ 不给结论：JSON 坏 ⇒ rc=2。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", bad})
	if rc != 2 {
		t.Fatalf("⑥ JSON 坏 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "JSON 解析失败") {
		t.Fatalf("⑥ JSON 坏文案不对：\n%s", out)
	}

	// ⑦ 不给结论：给的仓与 --release-head 都取不到 ⇒ rc=2（「都取不到」才退 2）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", ""})
	if rc != 2 {
		t.Fatalf("⑦ 取不到 HEAD 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "取不到发布提交 HEAD") {
		t.Fatalf("⑦ 取不到文案不对：\n%s", out)
	}

	// ⑧ 只给 --release-head（仓取不到）⇒ 不算「都取不到」⇒ 照走判据 rc=0。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("⑧ 显式 release-head 应 rc=0，实得 rc=%d\n%s", rc, out)
	}

	// ⑨ Q-176：--json 里有不认的字段名 ⇒ rc=2 ＋ 印可选字段表（stdout 为空）。
	rc, out, se := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--json", "hits,warnings"})
	if rc != 2 {
		t.Fatalf("⑨ 不认字段 应 rc=2，实得 rc=%d\nstdout=%s", rc, out)
	}
	if out != "" || !strings.Contains(se, "不认的字段名") || !strings.Contains(se, "可选字段表") {
		t.Fatalf("⑨ Q-176 面不对：stdout=%q stderr=%q", out, se)
	}

	// ⑩ 合法字段照旧走命中面：--json profile,rc ⇒ 出 JSON 行且 rc 照判。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", fresh, "--repo", "", "--release-head", head, "--json", "profile,rc"})
	if rc != 0 {
		t.Fatalf("⑩ public+新鲜 应 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, `{"profile": "public", "rc": 0}`) {
		t.Fatalf("⑩ JSON 行不对：\n%s", out)
	}

	// ⑪ --now 注入旋钮确定化：同一 --now 两次逐字节相同（且与不给 --now 同构）。
	old := filepath.Join(mfDir, "old.json")
	mfpTestWrite(t, old, `{"commit":"x","source_sha":"x","dirty":false,"generated_at":"2020-01-01T00:00:00Z"}`)
	a := []string{"--profile", "dev", "--manifest", old, "--repo", "", "--release-head", "x", "--max-age-hours", "1", "--now", "2026-10-01T00:00:00Z"}
	rcA, outA, _ := mfpTestRun(a)
	_, outB, _ := mfpTestRun(a)
	if outA != outB {
		t.Fatalf("⑪ --now 未确定化：两次输出不同\n%s\n---\n%s", outA, outB)
	}
	if rcA != 0 || !strings.Contains(outA, "> 上限 1h") || !strings.Contains(outA, "清单过旧") {
		t.Fatalf("⑪ --now 注入后的过龄判词不对（rc=%d）：\n%s", rcA, outA)
	}

	// ⑫ 真源不在场 ⇒ stale 面照实报「取不到」，不自己编一份（rc 仍由两条判据定）。
	repo2, mfDir2 := mfpTestFixture(t, false)
	t.Chdir(repo2)
	head2 := mfpGitTolerant(repo2, []string{"rev-parse", "HEAD"})
	fresh2 := filepath.Join(mfDir2, "fresh.json")
	mfpTestWrite(t, fresh2, `{"commit":"`+head2+`","source_sha":"`+head2+`","dirty":false}`)
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh2, "--repo", "", "--release-head", head2})
	if rc != 0 {
		t.Fatalf("⑫ 真源不在场应仍判绿 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "取不到") {
		t.Fatalf("⑫ 真源不在场文案不对：\n%s", out)
	}

	// ⑬ --list-rules 走静态表，rc=0 且不含判定面（防把静态面写成判据面）。
	rc, out, _ = mfpTestRun([]string{"--list-rules"})
	if rc != 0 || !strings.Contains(out, "R-23") {
		t.Fatalf("⑬ --list-rules 不对（rc=%d）：\n%s", rc, out)
	}
}

// mfpRetiredFixture —— 造一枚「邻件已退役」的仓外夹具：本件 py 在盘（自根推导用）＋ 邻件 Go 半在盘，
// 邻件 py **缺席**。☆ 真仓不受影响（全在 t.TempDir()）。
func mfpRetiredFixture(t *testing.T, withGoHalf bool) string {
	t.Helper()
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "check-manifest-freshness-profile.py"), []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withGoHalf {
		gh := filepath.Join(root, "core", "cmd", "zerg")
		if err := os.MkdirAll(gh, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gh, "check_manifest_freshness.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "seed"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("夹具 git %v 失败：%v\n%s", args, err, out)
		}
	}
	return root
}

// TestCheckManifestFreshnessProfileRetired —— 退役容忍（B轨 · 2026-10-07 · 类①）：邻件
// `check-manifest-freshness.py` 退役（py 缺席 · 其 Go 半在盘）⇒ 本件视同「单一口径可得」
// ⇒ 判据照常出结论（✗ 不得 rc=2 fail-closed）；真消失（无 Go 半）⇒ 仍 fail-closed（⑫ 格不破）。
//
// ★ 哑化负控：抽掉 `mfpNeighborRetired`（恒 false）⇒ 本用例退役两格必 FAIL 带行号。
func TestCheckManifestFreshnessProfileRetired(t *testing.T) {
	// ① 退役态：邻件 py 缺席 + Go 半在盘 ⇒ 缺省档（不给 --release-head，靠仓 HEAD）照常出结论。
	root := mfpRetiredFixture(t, true)
	t.Chdir(root)
	head := mfpGitTolerant(root, []string{"rev-parse", "HEAD"})
	fresh := filepath.Join(root, "fresh.json")
	mfpTestWrite(t, fresh, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":false,"generated_at":"2026-09-23T00:00:00Z"}`)
	rc, out, _ := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh})
	if rc == 2 {
		t.Fatalf("退役态 fail-closed（rc=2）—— 邻件退役不得让判据判不出：\n%s", out)
	}
	if strings.Contains(out, "真源不在场") {
		t.Fatalf("退役态仍报「真源不在场」（未走退役容忍分支）：\n%s", out)
	}
	// ①′ 自检也不得因邻件退役而不过（门 all.sh 会逐门跑自检档）。
	rcST, outST, _ := mfpTestRun([]string{"--self-test"})
	if rcST != 0 {
		t.Fatalf("退役态 --self-test 应 rc=0（真源由 Go 半承载），实得 rc=%d：\n%s", rcST, outST)
	}

	// ② 真消失（邻件 py 缺席 **且** 无 Go 半）⇒ 未退役 ⇒ 旧 fail-closed 语义必须保留（仅 --release-head 缺时退 2）。
	root2 := mfpRetiredFixture(t, false)
	t.Chdir(root2)
	head2 := mfpGitTolerant(root2, []string{"rev-parse", "HEAD"})
	fresh2 := filepath.Join(root2, "fresh.json")
	mfpTestWrite(t, fresh2, `{"commit":"`+head2+`","source_sha":"`+head2+`","dirty":false}`)
	rc2, out2, _ := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh2})
	if !strings.Contains(out2, "取不到") {
		t.Fatalf("真消失（无 Go 半）应仍报「取不到」（⑫ 格语义被误改）：\n%s", out2)
	}
	if rc2 != 2 {
		t.Fatalf("真消失（无 Go 半）应仍 fail-closed rc=2（取不到发布提交 HEAD），实得 rc=%d", rc2)
	}
}

// ── 内化金标（DEV-2143 同口径）：旧 py 对拍退场 ⇒ 逐格金标收 Go 常量 · 两态自证（✗ 不 Skip）─────
//
// 为什么这么测（与 `check_manifest_freshness_test.go` 的 `TestCheckManifestFreshnessGoldenInternalized`
// / `...GoldenTwoStates` 同族）：本件旧「py 真身 ⟷ Go 真搬」对拍源 `scripts/gates/check-manifest-freshness-profile.py`
// 已退场（2026-10-07 M2 退役批）⇒ 对拍源消失、断言恒 Skip（假绿、零保护）。按 DEV-2143 先例把金标真源
// **收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 由「Go 半现役行为」现算再冻）：逐格「输入（profile + manifest + argv）
//	  ⇒ (rc, stdout 关键行)」由 `mfpTestRun` 现跑捕获（一次性现算、只读夹具、✗ 不落仓），冻成 `want`；
//	  **本机时点类量**比对前归一化（✗ 不冻结）：发布头前 12 位 `mfpCut(head,12)` ⇒ `<HEAD12>`。
//
// 两态（✗ 不许 Skip）：A 态 = 夹具邻件**Go 半**在盘（真仓现态 · 退役 py 不在盘）；B 态 = 合成「旧态」
// 夹具（把退役邻件 py `scripts/gates/check-manifest-freshness.py` 以**打标桩**放回）⇒ 本门**不读邻件 py
// 正文**（`mfpDiagnose` 已在进程内重建）⇒ 两态对同一套金标**逐字同命中**（证 py 在/不在盘都不改判据结论）。
//
// ★ 哑化负控（"牙" · 见交付读数）：抽掉 `mfpNeighborRetired`（恒 false）⇒ `TestCheckManifestFreshnessProfileRetired`
//   退役两格必 FAIL 且带行号（✗ 不真改仓内源）。

// mfpGoldenNorm —— 归一本机时点类量（夹具根 / 发布头前 12 位）⇒ 金标跨机跨提交可复现。
func mfpGoldenNorm(s, head, root string) string {
	if root != "" {
		s = strings.ReplaceAll(s, root, "<FX>")
	}
	if h12 := mfpCut(head, 12); h12 != "" {
		s = strings.ReplaceAll(s, h12, "<HEAD12>")
	}
	return s
}

// mfpRunGolden —— 金标自证：dev+fresh 现跑本入口 · 归一（现算自证 · ✗ 手写）⇒ 返回 (rc, 归一化 stdout)。
// withStubNeighbor=true ⇒ 合成「旧态」：把退役邻件 py 以**打标桩**放回（本门不读其正文）。
func mfpRunGolden(t *testing.T, withStubNeighbor bool) (int, string) {
	t.Helper()
	repo, mfDir := mfpTestFixture(t, true)
	t.Chdir(repo)
	if withStubNeighbor {
		if err := os.WriteFile(filepath.Join(repo, "scripts", "gates", "check-manifest-freshness.py"),
			[]byte("# stub：造「旧态」——退役邻件 py 在位（本门不读其正文）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	head := mfpGitTolerant(repo, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatal("夹具 HEAD 取不到")
	}
	fresh := filepath.Join(mfDir, "fresh.json")
	mfpTestWrite(t, fresh, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":false,"generated_at":"2026-09-23T00:00:00Z"}`)
	rc, out, _ := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", head})
	return rc, mfpGoldenNorm(out, head, repo)
}

// TestCheckManifestFreshnessProfileGoldenInternalized —— 内化金标自证（A 态：真仓 · 退役 py 不在盘）。
// 旧 py 对拍（恒 Skip、零保护）已删；本用例✗ 不 Skip、逐行真断言（✗ 不放宽为「只判非空」）。
func TestCheckManifestFreshnessProfileGoldenInternalized(t *testing.T) {
	rc, out := mfpRunGolden(t, false)
	if rc != 0 {
		t.Fatalf("金标 rc=%d（冻值 0）\n%s", rc, out)
	}
	want := []string{
		"── 清单新鲜度分档（R-23）· 档=dev（开发机）──",
		"发布提交 HEAD：<HEAD12>（显式给出 --release-head）",
		"判定：**只告警**（rc 不拦，命中不红）",
		"   [J1] 成立 —— source_sha 等于发布提交 HEAD：source_sha=<HEAD12> == 发布提交 HEAD=<HEAD12>",
		"   [J2] 成立 —— dirty == false：dirty=false（出清单时工作树干净）",
		"── 三条 stale 判据（现调 check-manifest-freshness.py 的 diagnose() · 单一口径 · 语义一字未改）──",
		"   无（清单新鲜）",
		"⇒ 档=dev · 判据 2 条（命中 0）· rc=0",
	}
	for _, w := range want {
		if !strings.Contains(out, w+"\n") {
			t.Errorf("金标行缺失：%q\n--- 实得 ---\n%s", w, out)
		}
	}
}

// TestCheckManifestFreshnessProfileGoldenTwoStates —— 两态逐字同：A 态 = 夹具邻件 Go 半在盘（真仓现态 ·
// 退役 py 不在盘）；B 态 = 合成「旧态」夹具（退役邻件 py 以打标桩放回）⇒ 两态对同一套金标逐字同命中
// （✗ 不 Skip · ✗ 不放宽为「只判非空」）。
func TestCheckManifestFreshnessProfileGoldenTwoStates(t *testing.T) {
	rcA, outA := mfpRunGolden(t, false)

	// B 态：另起一枚合成「旧态」夹具（退役邻件 py 以打标桩放回）。本门不读其正文 ⇒ 输出必须逐字同。
	rcB, outB := mfpRunGolden(t, true)

	if rcA != rcB {
		t.Fatalf("两态 rc 不同：A=%d B=%d", rcA, rcB)
	}
	if outA != outB {
		t.Fatalf("两态逐字不同（py 在/不在盘不得改判据结论）：\n--- A ---\n%s\n--- B ---\n%s", outA, outB)
	}
}

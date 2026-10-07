// check_manifest_freshness_test.go —— Go 真搬的**成对负控**定向单测 + 定靶对拍（M2 批 · 甲案 C 类第 10 枚）。
//
// ✗ 禁恒绿：本件同时钉「必然判绿」「必然判红（--strict）」「不给结论（rc=2）」「用法错（rc=64）」四态，
// 任一态被写死都必有断言失败。
//
// 用例名：TestCheckManifestFreshnessGatePort（定向：go test -run TestCheckManifestFreshnessGatePort）·
// TestCheckManifestFreshnessDifferential（对拍：同夹具喂 py 真身与 Go 真搬，逐格比 rc/stdout）。
//
// 夹具策略：manifest 一律落在本测试的 t.TempDir()（**仓外**）；需要真 git 面的用例只用**只读**地
// `git rev-parse HEAD` 取仓根头；✗ 不碰真仓、✗ 不 git add/commit、✗ 不写任何真实状态目录。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mfTestRepoRoot —— 由 cwd 上溯找仓根（与入口缺省推导同一枚标记）。
// ★ 退役稳态（B轨 · 2026-10-07）：认「自指 py **或** mfRootMarkerRel」⇒ 自指 py 退役后仍能定位真仓。
func mfTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if root := mfRepoFrom(d); root != "" {
		return root
	}
	t.Fatalf("找不到同族标记所在仓根（由 %s 上溯）", d)
	return ""
}

// mfTestRun —— 真调入口，返回 (rc, stdout, stderr)。
func mfTestRun(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := mfMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// mfTestWriteManifest —— 落一份 manifest 夹具，返回其绝对路径。
func mfTestWriteManifest(t *testing.T, m map[string]any) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// ── 定向单测：四态成对负控（不依赖 py 解释器）──────────────────────────────────

func TestCheckManifestFreshnessGatePort(t *testing.T) {
	root := mfTestRepoRoot(t)
	head := mfGit(root, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatalf("拿不到仓根 HEAD（夹具前置不成立）")
	}
	fresh := map[string]any{
		"commit":       head,
		"source_sha":   head,
		"dirty":        false,
		"generated_at": "2026-09-15T00:00:00Z",
	}

	// ① 绿态：干净 + 陈旧日期但放宽窗口 ⇒ 零告警，rc=0
	p := mfTestWriteManifest(t, fresh)
	if rc, so, se := mfTestRun([]string{p, "--repo", root, "--max-age-hours", "999999"}); rc != 0 || so != "✓ 清单新鲜（无告警）\n" || se != "" {
		t.Fatalf("绿态失真：rc=%d so=%q se=%q", rc, so, se)
	}

	// ② 红态（--strict）：dirty=true ⇒ rc=1，且只告警不拒绝的缺省态仍 rc=0
	dirty := map[string]any{"commit": head, "source_sha": head, "dirty": true, "generated_at": "2026-09-15T00:00:00Z"}
	pd := mfTestWriteManifest(t, dirty)
	rc0, so0, _ := mfTestRun([]string{pd, "--repo", root, "--max-age-hours", "999999"})
	if rc0 != 0 || !strings.Contains(so0, "未提交改动") {
		t.Fatalf("缺省档应「只告警不拒绝」rc=0：rc=%d so=%q", rc0, so0)
	}
	rc1, so1, _ := mfTestRun([]string{pd, "--repo", root, "--max-age-hours", "999999", "--strict"})
	if rc1 != 1 || !strings.Contains(so1, "未提交改动") {
		t.Fatalf("--strict 档应升 rc=1：rc=%d so=%q", rc1, so1)
	}

	// ③ 不给结论（rc=2）：仓根取不到 git 面（不存在的仓根 ⇒ 不许静默当绿）
	rc2, _, se2 := mfTestRun([]string{p, "--repo", "__no_such_repo__"})
	if rc2 != 2 || !strings.Contains(se2, mfCodeTrackedUnknown) {
		t.Fatalf("取不到 git 面应 rc=2 + %s：rc=%d se=%q", mfCodeTrackedUnknown, rc2, se2)
	}

	// ④ 用法错（rc=64）：不给 manifest / 不存在的 manifest
	if rc, so, _ := mfTestRun([]string{"--repo", root}); rc != 64 || !strings.Contains(so, "用法：") {
		t.Fatalf("缺 manifest 应 rc=64：rc=%d so=%q", rc, so)
	}
	if rc, _, _ := mfTestRun([]string{filepath.Join(t.TempDir(), "nope.json")}); rc != 64 {
		t.Fatalf("manifest 不存在应 rc=64：rc=%d", rc)
	}

	// ⑤ 成对打印 `mfPair`：两枚不同串共享前 12 位 ⇒ 给全串（截断无鉴别力）
	if a, b := mfPair("AAAAAAAAAAAA1", "AAAAAAAAAAAA2"); a != "AAAAAAAAAAAA1" || b != "AAAAAAAAAAAA2" {
		t.Fatalf("mfPair 共享前 12 位应给全串：a=%q b=%q", a, b)
	}
	if a, b := mfPair("AAAAAAAAAAAA1", "BBBBBBBBBBBB2"); a != "AAAAAAAAAAAA" || b != "BBBBBBBBBBBB" {
		t.Fatalf("mfPair 前 12 位有鉴别力应截断：a=%q b=%q", a, b)
	}

	// ⑥ --json：本门无机器读面 ⇒ rc=2（机器面缺 ≠ 零命中）
	if rc, _, se := mfTestRun([]string{"--json"}); rc != 2 || !strings.Contains(se, "无机器读面") {
		t.Fatalf("--json 应 rc=2 且明说无机器读面：rc=%d se=%q", rc, se)
	}

	// ⑦ 判据①语义：source_sha ≠ 当前分支头 ⇒ 必报「另一棵树」
	if w, err := mfDiagnose(map[string]any{"source_sha": "ffffffffffffffff"}, root, 999999, time.Now().UTC()); err != nil || !mfAnyContains(w, "另一棵树") {
		t.Fatalf("source_sha 不等于分支头应报：w=%v err=%v", w, err)
	}

	// ⑧ 空清单 `{}`（`GAP-20260927-108`）：**没输入可判** ⇒ ✗ 不许报「新鲜」；rc 契约不变（缺省 0 · --strict 拦 1）
	pe := mfTestWriteManifest(t, map[string]any{})
	if rc, so, _ := mfTestRun([]string{pe, "--repo", root, "--max-age-hours", "999999"}); rc != 0 || !strings.Contains(so, "没输入可判") || strings.Contains(so, "清单新鲜") {
		t.Fatalf("空清单缺省档应 rc=0 + 「没输入可判」且 ✗ 不报「新鲜」：rc=%d so=%q", rc, so)
	}
	if rc, so, _ := mfTestRun([]string{pe, "--repo", root, "--max-age-hours", "999999", "--strict"}); rc != 1 || !strings.Contains(so, "没输入可判") {
		t.Fatalf("空清单 --strict 档应拦（rc=1）：rc=%d so=%q", rc, so)
	}
	// ⑧′ 成对：**非空**但零告警的真清单 ⇒ 照旧报「✓ 清单新鲜」（证 ⑧ 不是把「新鲜」整体删掉）
	if rc, so, _ := mfTestRun([]string{p, "--repo", root, "--max-age-hours", "999999"}); rc != 0 || so != "✓ 清单新鲜（无告警）\n" {
		t.Fatalf("真清单应照旧报「✓ 清单新鲜（无告警）」（与 ⑧ 成对）：rc=%d so=%q", rc, so)
	}
}

// ── 定靶对拍：同一夹具喂 py 真身与 Go 真搬，逐格比 rc / stdout（--json 那格另比 stderr）──────

type mfDiffCase struct {
	name          string
	mf            map[string]any // nil ⇒ 不落 manifest 夹具
	args          func(mfPath, root string) []string
	compareStderr bool
}

func TestCheckManifestFreshnessDifferential(t *testing.T) {
	root := mfTestRepoRoot(t)
	py := filepath.Join(root, "scripts", "gates", "check-manifest-freshness.py")
	if _, err := os.Stat(py); err != nil {
		t.Skipf("py 真身不在场 ⇒ 跳过对拍：%v", err)
	}
	py3, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 不在 PATH ⇒ 跳过对拍：%v", err)
	}
	head := mfGit(root, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatalf("拿不到仓根 HEAD")
	}
	const bigAge = "999999"
	base := func() map[string]any {
		return map[string]any{"commit": head, "source_sha": head, "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}
	}
	with := func(k string, v any) map[string]any { return mfWith(base(), k, v) }
	repoArgs := func(mfPath, _ string) []string {
		return []string{mfPath, "--repo", root, "--max-age-hours", bigAge}
	}
	cases := []mfDiffCase{
		{name: "绿_干净新鲜", mf: base(), args: repoArgs},
		{name: "告警_dirty", mf: with("dirty", true), args: repoArgs},
		{name: "红_dirty_strict", mf: with("dirty", true), args: func(mfPath, root string) []string {
			return append(repoArgs(mfPath, root), "--strict")
		}},
		{name: "另一棵树_source_sha", mf: with("source_sha", "ffffffffffffffffffffffffffffffffffffffff"), args: repoArgs},
		{name: "前缀弱匹配", mf: map[string]any{"commit": "AAAAAAAAAAAA", "source_sha": "AAAAAAAAAAAABBBB", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs},
		{name: "不一致_非前缀", mf: map[string]any{"commit": "zzzzzz", "source_sha": "yyyyyyyyyyyy", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs},
		{name: "成对_全串回退", mf: map[string]any{"commit": "AAAAAAAAAAAA1", "source_sha": "AAAAAAAAAAAA2", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs},
		{name: "无法解析_generated_at", mf: with("generated_at", "not-a-date"), args: repoArgs},
		{name: "过旧_generated_at", mf: with("generated_at", "2000-01-01T00:00:00Z"), args: func(mfPath, root string) []string {
			return []string{mfPath, "--repo", root, "--max-age-hours", "720"}
		}},
		{name: "缺_generated_at", mf: map[string]any{"commit": head, "source_sha": head, "dirty": false}, args: repoArgs},
		{name: "空清单_没输入可判", mf: map[string]any{}, args: repoArgs},
		{name: "空清单_strict_拦", mf: map[string]any{}, args: func(mfPath, root string) []string {
			return append(repoArgs(mfPath, root), "--strict")
		}},
		{name: "用法错_无参", mf: nil, args: func(mfPath, root string) []string { return []string{"--repo", root} }},
		{name: "用法错_件不存在", mf: nil, args: func(mfPath, root string) []string { return []string{mfPath, "--repo", root} }},
		{name: "无机器读面_json", mf: nil, args: func(mfPath, root string) []string { return []string{"--json"} }, compareStderr: true},
		{name: "自检_self_test", mf: nil, args: func(mfPath, root string) []string { return []string{"--self-test"} }},
	}

	cells, mismatch := 0, 0
	for _, c := range cases {
		mfPath := "/nonexistent/manifest.json"
		if c.mf != nil {
			mfPath = mfTestWriteManifest(t, c.mf)
		}
		args := c.args(mfPath, root)

		cmd := exec.Command(py3, append([]string{py}, args...)...)
		cmd.Dir = root
		var po, pe bytes.Buffer
		cmd.Stdout, cmd.Stderr = &po, &pe
		perr := cmd.Run()
		pyRC := 0
		if ee, ok := perr.(*exec.ExitError); ok {
			pyRC = ee.ExitCode()
		} else if perr != nil {
			t.Fatalf("[%s] 跑 py 失败：%v", c.name, perr)
		}

		goRC, goOut, goErr := mfTestRun(args)

		cells++
		if pyRC != goRC {
			mismatch++
			t.Errorf("[%s] rc 失配：py=%d go=%d", c.name, pyRC, goRC)
		}
		if po.String() != goOut {
			mismatch++
			t.Errorf("[%s] stdout 失配：\n  py=%q\n  go=%q", c.name, po.String(), goOut)
		}
		if c.compareStderr && pe.String() != goErr {
			mismatch++
			t.Errorf("[%s] stderr 失配：\n  py=%q\n  go=%q", c.name, pe.String(), goErr)
		}
	}
	t.Logf("对拍格数=%d 失配数=%d", cells, mismatch)
}

// TestCheckManifestFreshnessRetired —— 退役态（B轨 · 2026-10-07）：本件自指 py 退役（不在盘）
// ⇒ 仓根上溯照常出结论（✗ 不得退回 cwd ⇒ 缺省档 rc=2 fail-closed）。
//
//	① 根上溯：自指 py 缺席时靠退役稳态标记 mfRootMarkerRel 仍能上溯到仓根（✗ 退回 cwd）；
//	② 两态逐字：同夹具「py 在盘」⟷「py 移开（退役）」⇒ mfRepoFrom 逐字同、且非空。
//
// ★ 哑化负控：抽掉 mfRepoFrom 的 mfRootMarkerRel 一支 ⇒ ① 断言红（带行号）。
func TestCheckManifestFreshnessRetired(t *testing.T) {
	// ① 退役态：仓里只有退役稳态标记（✗ 无自指 py）。
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
	if got := mfRepoFrom(sub); got != fx {
		t.Fatalf("退役态根上溯失败：应 %s，得 %s ⇒ 自指 py 退役后上溯落空、退回 cwd ⇒ 判据 rc=2", fx, got)
	}

	// ② 同夹具「py 在盘」⟷「py 移开」：mfRepoFrom 读数逐字同（✗ 不因退役而变）。
	fx2 := t.TempDir()
	g2 := filepath.Join(fx2, "scripts", "gates")
	if err := os.MkdirAll(g2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g2, "precommit-gates.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	py := filepath.Join(g2, "check-manifest-freshness.py")
	if err := os.WriteFile(py, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	onRep := mfRepoFrom(g2)
	_ = os.Remove(py) // 强制退役态（✗ 不动真仓）
	reRep := mfRepoFrom(g2)
	if onRep != reRep || reRep != fx2 {
		t.Fatalf("退役态与在盘态根定位不一致：在盘=%q 退役=%q（应同为 %q）⇒ py 退役改变了根定位结论", onRep, reRep, fx2)
	}
}

// check_manifest_freshness_test.go —— Go 真搬的**成对负控**定向单测 + 定靶对拍（M2 批 · 甲案 C 类第 10 枚）。
//
// ✗ 禁恒绿：本件同时钉「必然判绿」「必然判红（--strict）」「不给结论（rc=2）」「用法错（rc=64）」四态，
// 任一态被写死都必有断言失败。
//
// 用例名：TestCheckManifestFreshnessGatePort（定向：go test -run TestCheckManifestFreshnessGatePort）·
// TestCheckManifestFreshnessGoldenInternalized（内化金标：旧 py 对拍退场 ⇒ 逐格金标收成 Go 常量 · ✗ 不 Skip）·
// TestCheckManifestFreshnessGoldenTwoStates（两态逐字同：真仓无 py ⟷ 合成旧态有 py）。
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
	"regexp"
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

// ── 内化金标：旧 py 对拍的逐格金标收成 Go 常量 · 现算自证 ──────────────────────────
//
// 为什么这么测（M2 口径 · 与 `archive_mount_test.go` 的 `TestArchiveMountTwoStatesGoldenInternalized`
// 同族）：本件原「py 真身 ⟷ Go 真搬」逐字对拍（旧名 `TestCheckManifestFreshnessDifferential`）的
// 对拍源 `scripts/gates/check-manifest-freshness.py` 已于 2026-10-07 退场（`35d270c0` M2 退役批）
// ⇒ 对拍源消失、断言恒 Skip（假绿、零保护）。按 DEV-2115 先例把金标真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「输入（manifest + argv）⇒ (rc, stdout, stderr)」由 `mfMain` 现跑捕获（`TestMFCaptureGolden`
//	  一次性现算、只读真仓、✗ 不落仓），冻成下表的 `wantRC/wantOut/wantErr` 常量；两处**本机时点量**
//	  在比对前归一化（✗ 不冻结 —— 因其本非常量）：
//	    ① 分支头前 12 位 `mfCut(head,12)` ⇒ `<HEAD12>`（随提交漂）；
//	    ② 过旧档 `距今 <N>h` 里的墙钟小时数 ⇒ `<AGE>`（随真实时钟漂；该格另有数字段在场的结构断言）。
//
// 两态（✗ 不许 Skip）：A 态 = 真仓（退役 py 不在盘）现跑；B 态 = 合成「旧态」仓（`git init` ＋
// 把退役 py 放回 `scripts/gates/`）现跑 ⇒ 两态**对同一套金标逐字同命中**，证 py 在/不在盘都不改
// 判据结论（内化彻底）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `check_manifest_freshness.go` 的 dirty 一臂哑化
// ⇒ 本用例**必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
type mfGoldenCase struct {
	name    string
	mf      map[string]any // nil ⇒ 不落 manifest 夹具
	args    func(mfPath, root string) []string
	wantRC  int
	wantOut string
	wantErr string
}

// 告警头（1 条 / 2 条）、用法行、空清单文案 —— 逐字取自 Go 真源 `mfMain`。
const (
	mfGoldenHdr1  = "⚠ 清单新鲜度告警 1 条（**只告警不拒绝**，依据：分发通道可能落后于分支，放宽窗口而非关掉检查）：\n"
	mfGoldenHdr2  = "⚠ 清单新鲜度告警 2 条（**只告警不拒绝**，依据：分发通道可能落后于分支，放宽窗口而非关掉检查）：\n"
	mfGoldenUsage = "用法：check-manifest-freshness.py <manifest.json> [--repo …] [--max-age-hours N] [--strict]\n"
	mfGoldenEmpty = "⚠ 空清单：没输入可判 —— 本清单未携带任何判据字段（source_sha / dirty / commit / generated_at 全缺）⇒ 不报「新鲜」\n"
)

// mfGoldenCases —— 金标真源（16 格）。head = 被跑那棵仓的分支头（真仓 / 合成旧态仓各自给）。
func mfGoldenCases(head string) []mfGoldenCase {
	base := func() map[string]any {
		return map[string]any{"commit": head, "source_sha": head, "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}
	}
	with := func(k string, v any) map[string]any { return mfWith(base(), k, v) }
	repoArgs := func(mfPath, root string) []string {
		return []string{mfPath, "--repo", root, "--max-age-hours", "999999"}
	}
	return []mfGoldenCase{
		{name: "绿_干净新鲜", mf: base(), args: repoArgs, wantRC: 0, wantOut: "✓ 清单新鲜（无告警）\n"},
		{name: "告警_dirty", mf: with("dirty", true), args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr1 + "   - 清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现\n"},
		{name: "红_dirty_strict", mf: with("dirty", true), args: func(mfPath, root string) []string {
			return append(repoArgs(mfPath, root), "--strict")
		}, wantRC: 1,
			wantOut: mfGoldenHdr1 + "   - 清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现\n"},
		{name: "另一棵树_source_sha", mf: with("source_sha", "ffffffffffffffffffffffffffffffffffffffff"), args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr2 +
				"   - 清单出自另一棵树：source_sha=ffffffffffff ≠ 当前分支头 <HEAD12>\n" +
				"   - 声明身份与真实来源不一致：commit=<HEAD12> 而 source_sha=ffffffffffff\n"},
		{name: "前缀弱匹配", mf: map[string]any{"commit": "AAAAAAAAAAAA", "source_sha": "AAAAAAAAAAAABBBB", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr2 +
				"   - 清单出自另一棵树：source_sha=AAAAAAAAAAAA ≠ 当前分支头 <HEAD12>\n" +
				"   - 声明身份与真实来源不一致：commit=AAAAAAAAAAAA 而 source_sha=AAAAAAAAAAAABBBB —— 只是**前缀弱匹配**（前 12 位同），不是相等（明文判据是逐字全串 ==；短串无鉴别力 ⇒ 不当「一致」判）\n"},
		{name: "不一致_非前缀", mf: map[string]any{"commit": "zzzzzz", "source_sha": "yyyyyyyyyyyy", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr2 +
				"   - 清单出自另一棵树：source_sha=yyyyyyyyyyyy ≠ 当前分支头 <HEAD12>\n" +
				"   - 声明身份与真实来源不一致：commit=zzzzzz 而 source_sha=yyyyyyyyyyyy\n"},
		{name: "成对_全串回退", mf: map[string]any{"commit": "AAAAAAAAAAAA1", "source_sha": "AAAAAAAAAAAA2", "dirty": false, "generated_at": "2026-09-15T00:00:00Z"}, args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr2 +
				"   - 清单出自另一棵树：source_sha=AAAAAAAAAAAA ≠ 当前分支头 <HEAD12>\n" +
				"   - 声明身份与真实来源不一致：commit=AAAAAAAAAAAA1 而 source_sha=AAAAAAAAAAAA2\n"},
		{name: "无法解析_generated_at", mf: with("generated_at", "not-a-date"), args: repoArgs, wantRC: 0,
			wantOut: mfGoldenHdr1 + "   - generated_at 无法解析（如实报，不猜）：'not-a-date'\n"},
		{name: "过旧_generated_at", mf: with("generated_at", "2000-01-01T00:00:00Z"), args: func(mfPath, root string) []string {
			return []string{mfPath, "--repo", root, "--max-age-hours", "720"}
		}, wantRC: 0,
			wantOut: mfGoldenHdr1 + "   - 清单过旧：generated_at=2000-01-01T00:00:00Z 距今 <AGE>h > 上限 720h\n"},
		{name: "缺_generated_at", mf: map[string]any{"commit": head, "source_sha": head, "dirty": false}, args: repoArgs, wantRC: 0,
			wantOut: "✓ 清单新鲜（无告警）\n"},
		{name: "空清单_没输入可判", mf: map[string]any{}, args: repoArgs, wantRC: 0, wantOut: mfGoldenEmpty},
		{name: "空清单_strict_拦", mf: map[string]any{}, args: func(mfPath, root string) []string {
			return append(repoArgs(mfPath, root), "--strict")
		}, wantRC: 1, wantOut: mfGoldenEmpty},
		{name: "用法错_无参", mf: nil, args: func(mfPath, root string) []string { return []string{"--repo", root} }, wantRC: 64, wantOut: mfGoldenUsage},
		{name: "用法错_件不存在", mf: nil, args: func(mfPath, root string) []string { return []string{mfPath, "--repo", root} }, wantRC: 64, wantOut: mfGoldenUsage},
		{name: "无机器读面_json", mf: nil, args: func(mfPath, root string) []string { return []string{"--json"} }, wantRC: 2, wantOut: "",
			wantErr: "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）\n   字段表：无（本门只出人读面）\n⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n"},
		{name: "自检_self_test", mf: nil, args: func(mfPath, root string) []string { return []string{"--self-test"} }, wantRC: 0, wantOut: "self-test 通过（6 条正/负用例）\n"},
	}
}

// mfAgeRe —— 归一「过旧」档的墙钟小时数（本机时点量 ⇒ 不冻值）。
var mfAgeRe = regexp.MustCompile(`距今 [0-9.]+h`)

// mfNormGolden —— 归一两处本机时点量（分支头前 12 位 / 过旧档墙钟小时数）⇒ 金标跨机跨提交可复现。
func mfNormGolden(s, head string) string {
	if h12 := mfCut(head, 12); h12 != "" {
		s = strings.ReplaceAll(s, h12, "<HEAD12>")
	}
	return mfAgeRe.ReplaceAllString(s, "距今 <AGE>h")
}

// mfRunGolden —— 逐格跑金标并断言，返回失配格数（0 ⇒ 全过）。tag 标状态便于排错。
func mfRunGolden(t *testing.T, tag, root, head string) int {
	t.Helper()
	mism := 0
	for _, c := range mfGoldenCases(head) {
		mfPath := "/nonexistent/manifest.json"
		if c.mf != nil {
			mfPath = mfTestWriteManifest(t, c.mf)
		}
		rc, out, errb := mfTestRun(c.args(mfPath, root))
		gotOut, gotErr := mfNormGolden(out, head), mfNormGolden(errb, head)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// TestCheckManifestFreshnessGoldenInternalized —— 内化金标自证（A 态：真仓 · 退役 py 不在盘）。
// 旧 py 对拍（恒 Skip、零保护）已删；本用例✗ 不 Skip、逐格给真断言（✗ 不放宽为「只判非空」）。
func TestCheckManifestFreshnessGoldenInternalized(t *testing.T) {
	root := mfTestRepoRoot(t)
	head := mfGit(root, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatalf("拿不到仓根 HEAD（夹具前置不成立）")
	}
	// A 态前置：py 真身确已退场（不在盘）—— 若它回流，本用例的前提就不再是「内化后无 py」。
	py := filepath.Join(root, "scripts", "gates", "check-manifest-freshness.py")
	if _, err := os.Stat(py); err == nil {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s", py)
	}
	if n := mfRunGolden(t, "A", root, head); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestCheckManifestFreshnessGoldenTwoStates —— 两态逐字同：A 态 = 真仓（退役 py 不在盘）；
// B 态 = 合成「旧态」仓（`git init` ＋ 把退役 py 放回 `scripts/gates/`）⇒ 两态**对同一套金标
// 逐字同命中**（证 py 在/不在盘都不改判据结论 · ✗ 不因退役 Skip、✗ 不放宽断言）。
func TestCheckManifestFreshnessGoldenTwoStates(t *testing.T) {
	realRoot := mfTestRepoRoot(t)
	realHead := mfGit(realRoot, []string{"rev-parse", "HEAD"})
	if realHead == "" {
		t.Fatalf("拿不到真仓 HEAD（夹具前置不成立）")
	}
	if n := mfRunGolden(t, "A真仓无py", realRoot, realHead); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}

	oldRoot := mfMakeOldStateRepo(t)
	oldPy := filepath.Join(oldRoot, "scripts", "gates", "check-manifest-freshness.py")
	if _, err := os.Stat(oldPy); err != nil {
		t.Fatalf("B 态前提破：合成旧态仓里没有退役 py：%v", err)
	}
	oldHead := mfGit(oldRoot, []string{"rev-parse", "HEAD"})
	if oldHead == "" {
		t.Fatalf("拿不到合成旧态仓 HEAD")
	}
	if n := mfRunGolden(t, "B合成旧态有py", oldRoot, oldHead); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}

// mfMakeOldStateRepo —— 造一棵「旧态」合成仓（仓外 · `git init` 出一枚真 HEAD）：把退役 py
// 放回 `scripts/gates/`（py 在位的那一态），另放退役稳态标记保持与真仓同形。✗ 不碰真仓。
func mfMakeOldStateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "precommit-gates.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "check-manifest-freshness.py"), []byte("# stub：造「旧态」——退役 py 在位\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "seed"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("合成旧态仓 git %v 失败：%v\n%s", args, err, out)
		}
	}
	return root
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

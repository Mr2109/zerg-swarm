// export_frontmatter_test.go —— 导出件 frontmatter 体检的两态（缺口 `GAP-20261018-10` 的**判据半**）。
//
// 病（本单要修的那一半）：导出器自 DEV-3069 起会写 frontmatter 六件套，但**目录里既有的历史导出件**
// 缺 frontmatter 时，导出器一声不响 ⇒ 那几件永远过不了 `check-doc-meta`，而 M1 默认 warn 不阻断
// ⇒ 没人会知道（`v2.5.15/` 里两件、`现行/` 里一件就是这么漂着的）。
//
// 判据（本件的全部断言都落在**进程退码 + 报话**上，✗ 不测内部函数返回）：
//
//	① 写档（真导出）：历史件缺块 / 缺键 / 值不符 ⇒ **退 1**，且**逐件逐键**点名；
//	② 补齐后同一命令 ⇒ **退 0**（转绿，且留一行「体检真跑过」的证据）；
//	③ 只读档（`--dry-run`）：**只报不判** —— 退码仍是 0（`core/internal/contract/dryrun-semantics.json`
//	   的 `help export` 条目逐字写着「`--dry-run` ⇒ 退码 0」，本单不改在册契约），但报话照样逐件点名；
//	④ 非导出件（目录里别的 .md 缺 frontmatter）**不在本体检的判面**（✗ 不越界判别人的文档）；
//	⑤ 前 400 行内没有闭合 `---` 的块按「没块」算（认不出的写法不拿好话糊过去）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// efmLegacyName —— 夹具里的「历史导出件」件名（照真件命名法，但换一个绝不可能与本趟撞日的日子）。
const efmLegacyName = "导出-命令面帮助-20200101.md"

// efmRun —— 跑一条命令（与 `main()` 调的是同一个 `run`）。
func efmRun(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := RunForTest(argv, &so, &se)
	return rc, so.String(), se.String()
}

// efmWithIdentity —— 写档要先过「身份两格不同时 unknown」那道门（`go test` 编出的制品两格都是
// `unknown` ⇒ 会先撞 exitBlocked，走不到判面）。这里临时注入真值，跑完复原。
func efmWithIdentity(t *testing.T) {
	t.Helper()
	oldC, oldB := version.Commit, version.BuildTime
	version.Commit, version.BuildTime = "0123456789abcdef0123456789abcdef01234567", "2026-10-19T00:00:00Z"
	t.Cleanup(func() { version.Commit, version.BuildTime = oldC, oldB })
}

// efmGoodFM —— 一件**合规**导出件的头部（六件套逐字照生成器产出的形状）。
func efmGoodFM(updated string) string {
	return "---\n" +
		"title: 导出：命令面帮助（自动生成 · 勿手改）\n" +
		"type: reference\n" +
		"status: historical\n" +
		"source_of_truth: core/cmd/zerg/ 命令树（zerg help export 现算）\n" +
		"owner: Mr2109\n" +
		"updated_at: " + updated + "\n" +
		"---\n\n# 导出：命令面帮助（自动生成 · 勿手改）\n"
}

// efmNoFM —— 一件**没有 frontmatter 块**的导出件（真件的形态：标题顶格）。
const efmNoFM = "# 导出：命令面帮助（自动生成 · 勿手改）\n\n> 生成命令：`zerg help export`\n"

// efmWrite —— 往夹具目录落一件（件名/内容由调用方定）。
func efmWrite(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("夹具落盘失败：%v", err)
	}
	return p
}

// efmTodaysExport —— 本趟导出物的件名（导出器按当天日期命名）。
func efmTodaysExport(dir string) string {
	return filepath.Join(dir, fmt.Sprintf("导出-命令面帮助-%s.md", time.Now().Format("20060102")))
}

// ── ① 写档 · 历史件缺 frontmatter ⇒ 退 1 且逐键点名 ─────────────────────────

func TestExportFrontmatterRedNamesMissingKeys(t *testing.T) {
	efmWithIdentity(t)
	dir := t.TempDir()
	efmWrite(t, dir, efmLegacyName, efmNoFM)

	rc, so, se := efmRun("help", "export", "--out", dir, "--yes")
	if rc != exitFail {
		t.Fatalf("历史件缺 frontmatter ⇒ 应退 %d（可判红），实得 %d\nstdout=%q\nstderr=%q", exitFail, rc, so, se)
	}
	if !strings.Contains(se, efmLegacyName) {
		t.Fatalf("报话必须**点名**那件历史件（%s），stderr=%q", efmLegacyName, se)
	}
	for _, k := range exportArtifactKeys {
		if !strings.Contains(se, k) {
			t.Fatalf("缺的键 %q 没被点名 —— 报话只说了「不合规」等于没说，stderr=%q", k, se)
		}
	}
	// 红的是**历史件**，本趟导出物照写（红 ≠ 本趟写失败）。
	if _, err := os.Stat(efmTodaysExport(dir)); err != nil {
		t.Fatalf("本趟导出物应照写（红的是历史件）：%v", err)
	}
	// 本趟件的头部六件套必须齐 —— 这是「生成器自己产的东西能过自己的判据」那一格。
	b, err := os.ReadFile(efmTodaysExport(dir))
	if err != nil {
		t.Fatalf("读不回本趟导出物：%v", err)
	}
	if !strings.HasPrefix(string(b), "---\n") {
		head := string(b)
		if len(head) > 80 {
			head = head[:80]
		}
		t.Fatalf("本趟导出物开头不是 frontmatter 块：%q", head)
	}
	for _, k := range exportArtifactKeys {
		if !strings.Contains(string(b), k+":") {
			t.Fatalf("本趟导出物缺六件套的键 %q", k)
		}
	}
}

// ── ② 补齐后 ⇒ 退 0（转绿，且留证据行） ────────────────────────────────────

func TestExportFrontmatterGreenAfterFill(t *testing.T) {
	efmWithIdentity(t)
	dir := t.TempDir()
	efmWrite(t, dir, efmLegacyName, efmGoodFM("2020-01-01"))

	rc, so, se := efmRun("help", "export", "--out", dir, "--yes")
	if rc != exitOK {
		t.Fatalf("六件套齐 ⇒ 应退 %d，实得 %d\nstdout=%q\nstderr=%q", exitOK, rc, so, se)
	}
	if !strings.Contains(se, "导出件体检 绿") {
		t.Fatalf("绿档要留一行证据（体检真跑过），stderr=%q", se)
	}
	if strings.Contains(se, "导出件体检 **红**") {
		t.Fatalf("补齐之后不许还报红，stderr=%q", se)
	}
}

// ── ③ 只读档 · 只报不判（退码 0 是在册契约） ─────────────────────────────────

func TestExportFrontmatterReadOnlyReportsOnly(t *testing.T) {
	dir := t.TempDir()
	efmWrite(t, dir, efmLegacyName, efmNoFM)

	rc, _, se := efmRun("help", "export", "--out", dir, "--dry-run")
	if rc != exitOK {
		t.Fatalf("`--dry-run` 的退码 0 是在册契约（core/internal/contract/dryrun-semantics.json）⇒ 应 0，实得 %d\nstderr=%q", rc, se)
	}
	if !strings.Contains(se, efmLegacyName) || !strings.Contains(se, "只报不判") {
		t.Fatalf("只读档也要逐件点名、并自报「只报不判」，stderr=%q", se)
	}
	// 零副作用：一个字节都不写（本趟件不得出现在盘上）。
	if _, err := os.Stat(efmTodaysExport(dir)); err == nil {
		t.Fatalf("`--dry-run` 一个字节都不许写，却落了 %s", efmTodaysExport(dir))
	}
}

// ── ④ 值不符也点名（不是只判「有没有键」） ───────────────────────────────────

func TestExportFrontmatterWrongValueNamed(t *testing.T) {
	efmWithIdentity(t)
	dir := t.TempDir()
	bad := strings.Replace(efmGoodFM("2020-01-01"), "status: historical", "status: current", 1)
	efmWrite(t, dir, efmLegacyName, bad)

	rc, _, se := efmRun("help", "export", "--out", dir, "--yes")
	if rc != exitFail {
		t.Fatalf("status 值不符 ⇒ 应退 %d，实得 %d\nstderr=%q", exitFail, rc, se)
	}
	if !strings.Contains(se, "status=current") || !strings.Contains(se, "应 historical") {
		t.Fatalf("值不符要按「现值 → 应值」点名，stderr=%q", se)
	}
}

// ── ⑤ 不越界：目录里别的 .md（缺 frontmatter）不在本体检的判面 ────────────────

func TestExportFrontmatterIgnoresOtherDocs(t *testing.T) {
	efmWithIdentity(t)
	dir := t.TempDir()
	efmWrite(t, dir, "使用指南.md", "# 使用指南\n\n（别的文档，不属于导出件家族）\n")

	rc, _, se := efmRun("help", "export", "--out", dir, "--yes")
	if rc != exitOK {
		t.Fatalf("导出件家族之外的文档不归本体检判（它们各有各的门）⇒ 应退 %d，实得 %d\nstderr=%q", exitOK, rc, se)
	}
	if strings.Contains(se, "使用指南.md") {
		t.Fatalf("✗ 报话点名了非导出件，stderr=%q", se)
	}
}

// ── ⑥ 块没闭合 = 没块（认不出的写法不拿好话糊过去） ──────────────────────────

func TestExportFrontmatterUnclosedBlockIsMissing(t *testing.T) {
	efmWithIdentity(t)
	dir := t.TempDir()
	unclosed := "---\ntitle: 半截\nstatus: historical\n\n# 导出：命令面帮助（自动生成 · 勿手改）\n"
	p := efmWrite(t, dir, efmLegacyName, unclosed)

	g, err := exportInspectArtifact(p)
	if err != nil {
		t.Fatalf("体检读不回夹具：%v", err)
	}
	if !g.bad() {
		t.Fatalf("没有闭合 `---` 的块必须当「没块」判红，实得 %+v", g)
	}
	for _, k := range exportArtifactKeys {
		found := false
		for _, m := range g.Missing {
			if m == k {
				found = true
			}
		}
		if !found {
			t.Fatalf("未闭合块 ⇒ 六件套应全部算缺，缺 %q 没在 Missing 里：%+v", k, g)
		}
	}
}

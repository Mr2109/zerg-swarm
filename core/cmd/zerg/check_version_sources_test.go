// check_version_sources_test.go —— Go 真搬件的**成对负控**定向单测（M2 第四枚 A 类纯静态件）。
//
// ✗ 禁恒绿：同时断言「必然判绿」「必然判红」「不给结论 rc=2」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckVersionSourcesGatePort（定向：go test -run TestCheckVersionSourcesGatePort）。
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const vstFixGo = "package version\n\n// Version 当前发布版（不带前缀）。\nconst Version = \"%s\"\n\nconst Tag = \"v\" + Version\n"
const vstFixUI = "[package]\nname = \"zerg-ui\"\nversion = \"%s\"\nedition = \"2021\"\n\n[dependencies]\nserde = { version = \"1\", features = [\"derive\"] }\n"
const vstFixWall = "[package]\nname = \"zerg-wall\"\nversion = \"%s\"   # 锚到版本真源\nedition = \"2021\"\n"
const vstFixVersion = "# 虫族 Zerg 版本号\n# 本文件已非真源：真源见 core/internal/version/version.go 与 ui/Cargo.toml\n"

// vstFixture 造一棵仓外夹具树（含四源 + 活文档 + 冻结节 + 排除域）。
func vstFixture(t *testing.T, goV, uiV, wallV, versionText string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"core/internal/version", "ui", "wall", "docs/zh", "docs/项目文档/v9.9.8", "Zerg-内部文档/01-设计"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	wr := func(rel, txt string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wr("core/internal/version/version.go", sprintf(vstFixGo, goV))
	wr("ui/Cargo.toml", sprintf(vstFixUI, uiV))
	wr("wall/Cargo.toml", sprintf(vstFixWall, wallV))
	wr("VERSION", versionText)
	wr("docs/zh/活文档.md", "# 活文档\n当前版本 9.9.8 已发布。\n见 `docs/项目文档/v9.9.8/变更-v9.9.8.md`。\n")
	wr("docs/项目文档/v9.9.8/变更-v9.9.8.md", "旧账：9.9.8 于昨日发布。\n")
	wr("Zerg-内部文档/01-设计/旧文.md", "旧设计（已分家到工作树之外）：9.9.8。\n")
	return root
}

func sprintf(f string, a ...interface{}) string { return fmt.Sprintf(f, a...) }

func vstRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkVersionSourcesGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckVersionSourcesGatePort(t *testing.T) {
	// ── 正控（必然判绿）：四处同版 9.9.9 + VERSION 纯指针 ─────────────────
	green := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	rcGreen, outGreen, _ := vstRun(t, green)
	if rcGreen != 0 || !strings.Contains(outGreen, "结果: OK（全绿）  rc=0") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：version.go 改一个数 ⇒ rc=1 ────────────────────
	r := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	if err := os.WriteFile(filepath.Join(r, "core/internal/version/version.go"),
		[]byte(sprintf(vstFixGo, "9.9.8")), 0o644); err != nil {
		t.Fatal(err)
	}
	rcGo, outGo, _ := vstRun(t, r)
	if rcGo != 1 || !strings.Contains(outGo, "≠ 基准") {
		t.Fatalf("负控-version.go 失败：得 rc=%d out=%q", rcGo, outGo)
	}
	if rcGreen == rcGo {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcGo)
	}

	// ── 负控：ui / wall 各自改一个数 ⇒ rc=1 ─────────────────────────────
	ru := vstFixture(t, "9.9.9", "9.9.8", "9.9.9", vstFixVersion)
	rcUI, outUI, _ := vstRun(t, ru)
	if rcUI != 1 || !strings.Contains(outUI, "ui/Cargo.toml:3") {
		t.Fatalf("负控-ui 失败：得 rc=%d out=%q", rcUI, outUI)
	}
	rw := vstFixture(t, "9.9.9", "9.9.9", "9.9.8", vstFixVersion)
	rcWall, _, _ := vstRun(t, rw)
	if rcWall != 1 {
		t.Fatalf("负控-wall 失败：得 rc=%d", rcWall)
	}

	// ── 负控：VERSION 里出现数值行 ⇒ rc=1 ───────────────────────────────
	rn := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion+"9.9.9\n")
	rcNum, outNum, _ := vstRun(t, rn)
	if rcNum != 1 || !strings.Contains(outNum, "出现版本数值") {
		t.Fatalf("负控-VERSION 数值失败：得 rc=%d out=%q", rcNum, outNum)
	}

	// ── 负控：VERSION 指针不再点名真源 ⇒ rc=1 ──────────────────────────
	rp := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", "# 版本号见某处\n")
	rcPtr, outPtr, _ := vstRun(t, rp)
	if rcPtr != 1 || !strings.Contains(outPtr, "没点名真源") {
		t.Fatalf("负控-指针失败：得 rc=%d out=%q", rcPtr, outPtr)
	}

	// ── 不给结论（≠ 绿）：缺件 ⇒ rc=2 ──────────────────────────────────
	rm := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	if err := os.Remove(filepath.Join(rm, "core/internal/version/version.go")); err != nil {
		t.Fatal(err)
	}
	rcMiss, outMiss, _ := vstRun(t, rm)
	if rcMiss != 2 || !strings.Contains(outMiss, "缺件") {
		t.Fatalf("负控-缺件 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 不给结论：VERSION 空文件 ⇒ rc=2（空转不许当绿） ─────────────────
	re := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", "")
	rcEmpty, outEmpty, _ := vstRun(t, re)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "空文件") {
		t.Fatalf("负控-空 VERSION 失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 不给结论：活文档语料 0 个文件被扫到 ⇒ rc=2 ──────────────────────
	rc2 := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	// ★ R6 收窄后：删整个 docs/ 会连带把「语料域根 docs」判成不在盘 ⇒ 改删唯一非排他语料件。
	if err := os.Remove(filepath.Join(rc2, "docs", "zh", "活文档.md")); err != nil {
		t.Fatal(err)
	}
	rcCorpus, outCorpus, _ := vstRun(t, rc2)
	if rcCorpus != 2 || !strings.Contains(outCorpus, "空转") {
		t.Fatalf("负控-空转 失败：得 rc=%d out=%q", rcCorpus, outCorpus)
	}

	// ── R6 运行时默认根（目录）点名覆盖：正控（好件报告含 R6 段）＋ 负控（默认根不在盘 ⇒ 红）──
	if !strings.Contains(outGreen, "-- 运行时默认根（R6 · 点名覆盖：4 条）--") {
		t.Fatalf("R6 正控失败：好件报告里没有运行时默认根段：out=%q", outGreen)
	}
	// 负控：抹掉一个运行时默认根（仓根标记目录 `ui`）⇒ R6 必红（不在盘且未登记迁出）。
	// ★ 不选 `docs/项目文档`：它非盘会让 `docs_snapshot_root` 先走 rc=2 早退（R6 够不着）。
	r6 := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	if err := os.RemoveAll(filepath.Join(r6, "ui")); err != nil {
		t.Fatal(err)
	}
	rcR6, outR6, _ := vstRun(t, r6)
	if rcR6 != 1 || !strings.Contains(outR6, "未登记迁出") || !strings.Contains(outR6, "R6") {
		t.Fatalf("R6 负控失败：默认根不在盘且未登记迁出应 rc=1 含「未登记迁出」，得 rc=%d out=%q", rcR6, outR6)
	}

	// ── 成对：同一夹具 + --strict-docs（默认只报告 rc0 ↔ 加严 rc1）───────
	rst := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
	rcLoose, outLoose, _ := vstRun(t, rst)
	rcStrict, outStrict, _ := vstRun(t, "--strict-docs", rst)
	if rcLoose != 0 || !strings.Contains(outLoose, "只报告") {
		t.Fatalf("成对-默认档 失败：得 rc=%d out=%q", rcLoose, outLoose)
	}
	if rcStrict != 1 || !strings.Contains(outStrict, "--strict-docs") {
		t.Fatalf("成对-加严档 失败：得 rc=%d out=%q", rcStrict, outStrict)
	}
	if rcLoose == rcStrict {
		t.Fatalf("成对断言失败：只报告 rc=%d 与加严 rc=%d 同码 ⇒ 无鉴别力", rcLoose, rcStrict)
	}

	// ── 机器面：--json 键在场且含基准 ──────────────────────────────────
	_, outJSON, _ := vstRun(t, "--json", green)
	if !strings.Contains(outJSON, "\"canonical\": \"9.9.9\"") || !strings.Contains(outJSON, "\"tool_version\": \"v1.0.0\"") {
		t.Fatalf("--json 失败：out=%q", outJSON)
	}
	// 空集合形态：problems/blockers 必须是 [] 而非 null
	if !strings.Contains(outJSON, "\"problems\": []") || !strings.Contains(outJSON, "\"blockers\": []") {
		t.Fatalf("--json 空集合形态 失败：out=%q", outJSON)
	}

	// ── 未知旗标 ⇒ rc=2（不给结论 ≠ 绿） ────────────────────────────────
	rcFlag, _, errFlag := vstRun(t, "--no-such-flag", green)
	if rcFlag != 2 || !strings.Contains(errFlag, "用法错") {
		t.Fatalf("未知旗标 失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── --self-test 自身必须全过（防恒绿/恒红） ─────────────────────────
	rcST, outST, _ := vstRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "结果: 通过（20/20 条断言通过）  rc=0") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}

	// ── 真口子钉牙（B 类：读失败静默少判）：读不了的 version.go ⇒ rc=2 ──
	//   py 对「存在但读不到」的源件走 blocker（不让它静默绿）；本用例把该现状钉死。
	if os.Geteuid() != 0 {
		ru2 := vstFixture(t, "9.9.9", "9.9.9", "9.9.9", vstFixVersion)
		p := filepath.Join(ru2, "core/internal/version/version.go")
		if err := os.Chmod(p, 0o000); err == nil {
			rcUnreadable, outUnreadable, _ := vstRun(t, ru2)
			if rcUnreadable != 2 || !strings.Contains(outUnreadable, "不可解析") {
				t.Fatalf("真口子-读不了的件 失败：得 rc=%d out=%q", rcUnreadable, outUnreadable)
			}
			_ = os.Chmod(p, 0o644)
		}
	}
}

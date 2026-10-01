// check_help_snapshot_test.go —— 同干判据件 Go 真搬的**成对负控测试件**。
//
// 为什么有这件（两件事都要钉住）：
//
//	① **负控有牙**：正控（同形夹具 ⇒ rc=0）与负控（漂移 / 零行 / 缺键 / 制品不在盘 ⇒ 各自 rc）
//	   成对跑；断言打在**逐字 stdout** 与**退码**上（哑化任一处构造 ⇒ 断言当场 FAIL，带行号）。
//	② **自检自回收之证（delta=0）**：同干 py 原件是 `hsnap-selftest-*` 临时目录的**已知泄漏源**
//	   （全件 0 处回收 ⇒ /private/var/tmp 下成百上千枚残留）。本件的 `--self-test` 自带
//	   `defer os.RemoveAll` ⇒ 跑测前后同名前缀计数**相等**（delta=0）；测试件显式钉住这一格。
//
// 测试件用**合成 stub 制品**（`<repoRoot>/bin/zerg` 是一个小壳脚本）驱动两半，不依赖真制品路径
// ⇒ 可移植；真制品面的两环境对拍在落件报告里另出（不在本测试件里硬编任何跑测机路径）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// hsWriteStub —— 在 repoRoot/bin/zerg 落一枚 stub 制品（壳脚本）。
func hsWriteStub(t *testing.T, repoRoot, body string) {
	t.Helper()
	binDir := filepath.Join(repoRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("建 bin/ 失败：%v", err)
	}
	p := filepath.Join(binDir, "zerg")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("写 stub 失败：%v", err)
	}
}

// hsStubBody —— 两半各出什么：人面逐行 + 机器面 JSON 行。
func hsStubBody(jsonLine, humanLines string) string {
	return "#!/bin/sh\n" +
		"json=0\n" +
		"for a in \"$@\"; do [ \"$a\" = \"--json\" ] && json=1; done\n" +
		"if [ \"$json\" = \"1\" ]; then\n" +
		"  printf '%s\\n' '" + jsonLine + "'\n" +
		"else\n" +
		"  printf '%s\\n' '" + humanLines + "'\n" +
		"fi\n"
}

// hsWriteSnap —— 落一份快照（§一 表形态）。
func hsWriteSnap(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写快照失败：%v", err)
	}
	return p
}

const hsSnapHeader = "# 导出：命令面帮助（自动生成）\n\n## 一、命令清单\n\n| 命令 | 说明 |\n|---|---|\n"

func hsSnapRows(names ...string) string {
	var b strings.Builder
	b.WriteString(hsSnapHeader)
	for _, n := range names {
		b.WriteString("| `zerg " + n + "` | 夹具 |\n")
	}
	return b.String()
}

func hsRun(argv ...string) (string, string, int) {
	var out, errb bytes.Buffer
	rc := checkHelpSnapshotGateMain(&out, &errb, argv)
	return out.String(), errb.String(), rc
}

const hsSelfTestGolden = "── 自检（合成夹具 · 不碰真目标）──\n" +
	"  ✓ ① 快照行抽得出来（两条）（期望 ['agent ls', 'gap ls'] · 实得 ['agent ls', 'gap ls']）\n" +
	"  ✓ ① 快照不在盘 ⇒ 判不了（期望 True · 实得 True）\n" +
	"  ✓ ② 逐行：抽得出（前缀 `  zerg `）（期望 ['agent ls', 'gap ls'] · 实得 ['agent ls', 'gap ls']）\n" +
	"  ✓ ② 逐行：不同 ⇒ 判红（快照说的是哪些行）（期望 False · 实得 False）\n" +
	"  ✓ ② 逐行：判词点名到第 1 行（期望 True · 实得 True）\n" +
	"  ✓ ② 逐行：同名同序 ⇒ 合（期望 True · 实得 True）\n" +
	"  ✓ ② 逐行：条数不等 ⇒ 判红（期望 False · 实得 False）\n" +
	"  ✓ ③ items 逐条含命令名 ⇒ 合（期望 [] · 实得 []）\n" +
	"  ✓ ③ items 缺命令名 ⇒ 判红（期望 True · 实得 True）\n" +
	"  ✓ ③ items 丢了摘要格 ⇒ 判红（期望 True · 实得 True）\n" +
	"  ✓ ③ items 零条 ⇒ 判红（不许当绿）（期望 True · 实得 True）\n" +
	"  自检 11 格 · 失败 0 格\n"

// ── 正控 / 分道 ①：全合 ⇒ rc=0（逐字 stdout）────────────────────────────────
func TestHelpSnapshot_Green(t *testing.T) {
	root := t.TempDir()
	hsWriteStub(t, root, hsStubBody(
		`{"schema":"zerg/v1","items":[{"command":"zerg agent ls","commands":"9"}]}`,
		"  zerg agent ls ｜ 茧壁:node ｜ 子端清单"))
	snap := hsWriteSnap(t, t.TempDir(), "导出-命令面帮助-20260924.md", hsSnapRows("agent ls"))
	out, errS, rc := hsRun("--repo-root", root, "--snapshot", snap)
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0（stdout=%q stderr=%q）", rc, out, errS)
	}
	want := "帮助成文快照 ↔ 运行期逐行对拍（波6 序70）· v1.0.0\n" +
		"  判据见 `--list-rules`：① items 逐条含命令名 · ② 快照 ⟷ 现跑 逐行\n" +
		"  快照 = " + snap + "（1 行）· 现跑 = 1 行\n" +
		"  ✓ ① items 1 条逐条含命令名（摘要格 `commands` 一条不丢）\n" +
		"  ✓ ② 逐行对拍 1 行 ⇒ 一致\n" +
		"⇒ 两半都合（rc=0）\n"
	if out != want {
		t.Fatalf("stdout 不逐字：\n实得 %q\n期望 %q", out, want)
	}
	if errS != "" {
		t.Fatalf("stderr 应为空，实得 %q", errS)
	}
}

// ── 负控 ②：快照漂移（条数不等）⇒ rc=1 且判词点名 ────────────────────────────
func TestHelpSnapshot_DriftRed(t *testing.T) {
	root := t.TempDir()
	hsWriteStub(t, root, hsStubBody(
		`{"items":[{"command":"zerg a","commands":"9"},{"command":"zerg b","commands":"9"}]}`,
		"  zerg a ｜ x\n  zerg b ｜ y"))
	snap := hsWriteSnap(t, t.TempDir(), "导出-命令面帮助-20260924.md", hsSnapRows("a"))
	_, _, rc := hsRun("--repo-root", root, "--snapshot", snap)
	if rc != 1 {
		t.Fatalf("rc=%d 期望 1（漂移须判红）", rc)
	}
	out, _, _ := hsRun("--repo-root", root, "--snapshot", snap)
	if !strings.Contains(out, "② 逐行对拍：行数不等 —— 快照 1 行 / 现跑 2 行") {
		t.Fatalf("缺条数不等判词：%q", out)
	}
	if !strings.Contains(out, "⇒ 判红（rc=1）：") {
		t.Fatalf("缺判红抬头：%q", out)
	}
}

// ── 负控 ③：items 显式 null ⇒ 与缺键同形（py `or []` ⇒ 零条 ⇒ 不许当绿）──────
func TestHelpSnapshot_ItemsNull(t *testing.T) {
	for _, tc := range []struct{ name, jsonLine string }{
		{"显式 null", `{"items":null}`},
		{"缺键", `{"schema":"zerg/v1"}`},
		{"空数组", `{"items":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			hsWriteStub(t, root, hsStubBody(tc.jsonLine, "  zerg a ｜ x"))
			snap := hsWriteSnap(t, t.TempDir(), "导出-命令面帮助-20260924.md", hsSnapRows("a"))
			out, _, rc := hsRun("--repo-root", root, "--snapshot", snap)
			if rc != 1 {
				t.Fatalf("rc=%d 期望 1", rc)
			}
			if !strings.Contains(out, "① `items` 是空的（零条 ⇒ 不许当绿）") {
				t.Fatalf("缺零条判词：%q", out)
			}
		})
	}
}

// ── 负控 ④：items 缺 command 名 ⇒ rc=1 ───────────────────────────────────────
func TestHelpSnapshot_ItemsMissingCommand(t *testing.T) {
	root := t.TempDir()
	hsWriteStub(t, root, hsStubBody(
		`{"items":[{"commands":"9"}]}`, "  zerg a ｜ x"))
	snap := hsWriteSnap(t, t.TempDir(), "导出-命令面帮助-20260924.md", hsSnapRows("a"))
	out, _, rc := hsRun("--repo-root", root, "--snapshot", snap)
	if rc != 1 {
		t.Fatalf("rc=%d 期望 1", rc)
	}
	if !strings.Contains(out, "① 第 1 条 `items` 没有 `command`（逐条指不到是哪条）") {
		t.Fatalf("缺逐条点名判词：%q", out)
	}
}

// ── 负控 ⑤：制品不在盘 / 制品是目录 / 制品退非零 ⇒ rc=2 ─────────────────────
func TestHelpSnapshot_ArtifactBad(t *testing.T) {
	t.Run("不在盘", func(t *testing.T) {
		root := t.TempDir()
		out, _, rc := hsRun("--repo-root", root, "--snapshot", "/nope/x.md")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(out, "⛔ 不给结论（rc=2）：`bin/zerg` 不在盘") {
			t.Fatalf("缺制品不在盘判词：%q", out)
		}
	})
	t.Run("制品是目录", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "bin", "zerg"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, _, rc := hsRun("--repo-root", root, "--snapshot", "/nope/x.md")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2（目录非 isfile）", rc)
		}
	})
	t.Run("制品退非零", func(t *testing.T) {
		root := t.TempDir()
		hsWriteStub(t, root, "#!/bin/sh\nexit 3\n")
		out, _, rc := hsRun("--repo-root", root, "--snapshot", "/nope/x.md")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(out, "现跑 `help export --dry-run` 退码 3") {
			t.Fatalf("缺退码判词：%q", out)
		}
	})
}

// ── 负控 ⑥：快照面（不在盘 / 零行）⇒ rc=2 ───────────────────────────────────
func TestHelpSnapshot_SnapshotBad(t *testing.T) {
	root := t.TempDir()
	hsWriteStub(t, root, hsStubBody(`{"items":[{"command":"zerg a","commands":"9"}]}`, "  zerg a ｜ x"))
	t.Run("不在盘", func(t *testing.T) {
		out, _, rc := hsRun("--repo-root", root, "--snapshot", filepath.Join(root, "nope.md"))
		if rc != 2 || !strings.Contains(out, "快照不在盘：") {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
	t.Run("零行", func(t *testing.T) {
		snap := hsWriteSnap(t, t.TempDir(), "导出-命令面帮助-20260924.md", "# 没有表行\n")
		out, _, rc := hsRun("--repo-root", root, "--snapshot", snap)
		if rc != 2 || !strings.Contains(out, "表行都没有") {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
}

// ── 正控 ⑦：--list-rules / --self-test 逐字 ─────────────────────────────────
func TestHelpSnapshot_ListRulesAndSelfTest(t *testing.T) {
	t.Run("list-rules", func(t *testing.T) {
		out, _, rc := hsRun("--list-rules")
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if !strings.HasPrefix(out, "判据（两半 · 波6 序70）\n") {
			t.Fatalf("抬头不对：%q", out)
		}
		if !strings.Contains(out, "0 = 两半都合 · 1 = 判红") {
			t.Fatalf("缺退码段：%q", out)
		}
	})
	t.Run("self-test", func(t *testing.T) {
		out, _, rc := hsRun("--self-test")
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if out != hsSelfTestGolden {
			t.Fatalf("自检 stdout 不逐字：\n实得 %q\n期望 %q", out, hsSelfTestGolden)
		}
	})
}

// ── 负控 ⑧：未知旗标 ⇒ rc=2（用法错 ≠ 零命中）───────────────────────────────
func TestHelpSnapshot_UnknownFlag(t *testing.T) {
	_, errS, rc := hsRun("--no-such-flag")
	if rc != 2 {
		t.Fatalf("rc=%d 期望 2", rc)
	}
	if !strings.Contains(errS, "未知参数 --no-such-flag") {
		t.Fatalf("缺用法错判词：%q", errS)
	}
}

// ── 自检自回收之证：跑 `--self-test` 前后同名前缀计数相等（delta=0）───────────
func hsSeltestTmpCount() int {
	d := os.TempDir()
	ents, err := os.ReadDir(d)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "hsnap-selftest-") {
			n++
		}
	}
	return n
}

func TestHelpSnapshot_SelfCleanDeltaZero(t *testing.T) {
	before := hsSeltestTmpCount()
	if before < 0 {
		t.Skipf("读不了 TMPDIR=%s", os.TempDir())
	}
	var out, errb bytes.Buffer
	rc := checkHelpSnapshotGateMain(&out, &errb, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检 rc=%d 期望 0", rc)
	}
	after := hsSeltestTmpCount()
	if before != after {
		t.Fatalf("自检未自回收：同名前缀计数 delta=%d（前 %d / 后 %d）—— py 原件即漏源，本件须 delta=0",
			after-before, before, after)
	}
}

// TestHelpSnapshotHarness —— 两环境对拍的仓外驱动器入口：argv/输出/退码经环境变量与文件交换。
//
//	★ 默认**跳过**（只在 HSNAP_HARNESS=1 时真跑）⇒ 不影响常规 `go test`。
func TestHelpSnapshotHarness(t *testing.T) {
	if os.Getenv("HSNAP_HARNESS") != "1" {
		t.Skip("仓外驱动器未开门（HSNAP_HARNESS != 1）")
	}
	var argv []string
	if raw := os.Getenv("HSNAP_ARGV"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &argv); err != nil {
			t.Fatalf("HSNAP_ARGV 不是 JSON 数组：%v", err)
		}
	}
	var out, errb bytes.Buffer
	rc := checkHelpSnapshotGateMain(&out, &errb, argv)
	_ = os.WriteFile(os.Getenv("HSNAP_OUT"), out.Bytes(), 0o644)
	_ = os.WriteFile(os.Getenv("HSNAP_ERR"), errb.Bytes(), 0o644)
	_ = os.WriteFile(os.Getenv("HSNAP_RC"), []byte(strconv.Itoa(rc)), 0o644)
}

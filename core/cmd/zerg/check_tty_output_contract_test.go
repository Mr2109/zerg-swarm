// check_tty_output_contract_test.go —— Go 真搬样板的**成对负控**定向单测（M2 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」「空 vs 缺」「夹具自回收」五态 ——
// 任一态被写死都必失败。用例名：TestCheckTtyOutputContractGatePort
// （定向：go test -run TestCheckTtyOutputContractGatePort）。
//
// ★ 自指：本件被 `check-error-kinds` 的 R3 扫面**排除**（它跳过 `_test.go`）⇒ 无 R3 约束；
//
//	但仍不写连续引号原串形态（本门零 setErr 调用）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// ctocTestRun —— 进程内跑入口（与 py `main()` 同形）。
func ctocTestRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkTtyOutputContractGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ctocCountTempDirs —— 数 TMPDIR 里本件前缀的夹具目录（`zerg-tty-`）⇒ A 类自回收的牙。
func ctocCountTempDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "zerg-tty-") {
			n++
		}
	}
	return n
}

// ctocLastLine —— 取 stdout 最后一个非空行（JSON 档落在此）。
func ctocLastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestCheckTtyOutputContractGatePort(t *testing.T) {
	// ── 正控 ①：帮助面 golden（`GAP-20261001-61`：py `-h` stdout 的 sha256 钉死）──────────
	t.Run("help_golden_sha256", func(t *testing.T) {
		rc, out, errb := ctocTestRun("-h")
		if rc != 0 {
			t.Fatalf("help: rc=%d（期望 0）", rc)
		}
		if errb != "" {
			t.Fatalf("help: stderr 应为空，实得 %q", errb)
		}
		sum := sha256.Sum256([]byte(out))
		if got := hex.EncodeToString(sum[:]); got != ctocHelpSha256 {
			t.Fatalf("help golden 失配：实得 %s（期望 %s · %d 字节 / 现读 %d 字节）",
				got, ctocHelpSha256, len(ctocHelpSha256)/2, len(out))
		}
		if len(out) != 398 {
			t.Fatalf("help 字节数：实得 %d（期望 398）", len(out))
		}
		if !strings.HasPrefix(out, "usage: check-tty-output-contract.py [-h] [--target-cmd TARGET_CMD]\n") ||
			!strings.Contains(out, "optional arguments:") {
			t.Fatalf("help 正文形态失配：%q", out[:ctocMin(len(out), 80)])
		}
	})

	// ── 内化自证 ①b（判据件退役前置 ② · GAP-20261007-16）：金标真源 = Go 半 `ctocHelp`（✗ 不读 py）──
	//    py 退役后本格仍须绿；抽掉 `ctocHelpNativeSha256` 的内化（改成与 ctocHelp 无关的值）本格即红。
	t.Run("help_golden_internalized", func(t *testing.T) {
		if got := ctocHelpNativeSha256(); got != ctocHelpSha256 {
			t.Fatalf("Go 真源自证失配：sha256(ctocHelp)=%s ≠ 冻结 %s（金标真源已离开 Go 半）",
				got, ctocHelpSha256)
		}
	})

	// ── 正控 ②：`--list-rules` 逐字 ────────────────────────────────────────────────
	t.Run("list_rules", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--list-rules")
		if rc != 0 || errb != "" {
			t.Fatalf("list-rules: rc=%d stderr=%q", rc, errb)
		}
		for _, want := range []string{
			"归一白名单（只两条 · 都是 PTY 层）：N1 `script` 前缀 · N2 ONLCR（\\r\\n→\\n）",
			"一次跑的目标数上限：1（给多了 ⇒ rc=2，拒绝把成本 ×2）",
			"退码：0 逐字相同 · 1 有命中 · 2 不给结论（跑不出/空转/假 TTY/目标>1/自检不过）",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("list-rules 缺行：%q\n实得：%q", want, out)
			}
		}
	})

	// ── 正控 ③：两态同形合成目标 ⇒ rc=0（机器面逐字同）──────────────────────────────
	t.Run("same_target_rc0", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--target-cmd", `printf '{"a":1}\n'`)
		if rc != 0 || errb != "" {
			t.Fatalf("same: rc=%d stderr=%q", rc, errb)
		}
		if !strings.Contains(out, "✓ 逐字相同（TTY 差异不在机器面）") {
			t.Fatalf("same: 未见逐字相同行：%q", out)
		}
	})

	// ── 负控(must-fail) ④：TTY 加 ANSI ⇒ 必须真红 rc=1 ＋ 命中点名 ＋ `%r` 上下文 ────────
	t.Run("tty_color_mustfail_rc1", func(t *testing.T) {
		tc := `if [ -t 1 ]; then printf '\033[32m{"a":1}\033[0m\n'; else printf '{"a":1}\n'; fi`
		rc, out, _ := ctocTestRun("--target-cmd", tc)
		if rc != 1 {
			t.Fatalf("tty_color: rc=%d（期望 1）out=%q", rc, out)
		}
		if !strings.Contains(out, "归一后仍不同 ⇒ 命中") ||
			!strings.Contains(out, "**归一后仍不同**（首个差异在偏移 0") {
			t.Fatalf("tty_color: 未点名命中：%q", out)
		}
		if !strings.Contains(out, `   ✗    TTY   ：'\x1b[32m{"a":1}\x1b[0m\n'`) {
			t.Fatalf("tty_color: TTY 侧 repr 上下文失配：%q", out)
		}
	})

	// ── 负控 ⑤：`%r` 上下文逐字（纯 ASCII 差异）─────────────────────────────────────
	t.Run("first_diff_repr_ascii", func(t *testing.T) {
		_, out, _ := ctocTestRun("--target-cmd", `if [ -t 1 ]; then printf 'X\n'; else printf 'Y\n'; fi`)
		if !strings.Contains(out, `   ✗    非 TTY：'Y\n'`) || !strings.Contains(out, `   ✗    TTY   ：'X\n'`) {
			t.Fatalf("first_diff repr 失配：%q", out)
		}
	})

	// ── 不给结论 ⑥：零字节两边（空转 ≠ 绿）⇒ rc=2 ──────────────────────────────────
	t.Run("silent_rc2", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--target-cmd", "true")
		if rc != 2 || errb != "" {
			t.Fatalf("silent: rc=%d stderr=%q", rc, errb)
		}
		if !strings.Contains(out, "空转") {
			t.Fatalf("silent: 未见空转文案：%q", out)
		}
		// ★ C 类真口子 fail-closed 的牙：py 在这一格 KeyError 崩（rc=1 + 回溯）⇒ 本件 rc=2，
		//   且 stdout 停在命中行（不出现「⇒ rc=」/「附加判据」）—— 删掉 fail-closed 分支即红。
		if strings.Contains(out, "⇒ rc=") || strings.Contains(out, "附加判据") {
			t.Fatalf("silent: fail-closed 失守（stdout 越过了 py 崩溃点）：%q", out)
		}
		if !strings.HasSuffix(out, "⇒ 空转\n") {
			t.Fatalf("silent: stdout 尾部应停在空转命中行：%q", out)
		}
	})

	// ── 不给结论 ⑦：假 TTY（探针读数被改坏）⇒ rc=2（判据单元的牙）─────────────────────
	t.Run("fake_tty_rc2", func(t *testing.T) {
		run := ctocRunPair(`printf '{"a":1}\n'`, ctocTimeoutSecs)
		if run.hasNote {
			t.Skipf("本机取不到真两态（note=%q）⇒ 该格不适用", run.note)
		}
		run.isattyT = "0"
		rc, hits, info := ctocJudge(run)
		if rc != 2 || info == nil || info.present {
			t.Fatalf("fake_tty: rc=%d info=%v（期望 2 / info 非空但 present=false —— 空 dict 档）", rc, info)
		}
		if len(hits) != 1 || !strings.Contains(hits[0], "探针证不出真两态") {
			t.Fatalf("fake_tty: 文案失配：%v", hits)
		}
	})

	// ── 负控 ⑧：一次给 2 条目标 ⇒ rc=2（「跑两遍的步数 ≤1」）──────────────────────────
	t.Run("two_targets_rc2", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--target-cmd", "true", "--target-cmd", "true")
		if rc != 2 || errb != "" {
			t.Fatalf("two_targets: rc=%d stderr=%q", rc, errb)
		}
		if !strings.Contains(out, "一次给了 **2** 个目标 ⇒ 会把「跑两遍」的步数从 1 顶到 2") {
			t.Fatalf("two_targets: 文案失配：%q", out)
		}
	})

	// ── 机器读面 ⑨：`--json` 子集（键序 + py `\", \"` / `\": \"` 分隔符）────────────────
	t.Run("json_subset", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--json", "rc,equal_norm", "--target-cmd", `printf '{"a":1}\n'`)
		if rc != 0 || errb != "" {
			t.Fatalf("json_subset: rc=%d stderr=%q", rc, errb)
		}
		if got := ctocLastLine(out); got != `{"rc": 0, "equal_norm": true}` {
			t.Fatalf("json_subset 逐字失配：%q", got)
		}
	})

	// ── 机器读面 ⑩：`--json` 全表（键在场且空 = `[]` / `\"\"`）─────────────────────────
	t.Run("json_full_empty_hits", func(t *testing.T) {
		rc, out, _ := ctocTestRun("--json", "rc,target,cmd,step_runs,max_targets,equal_raw,equal_norm,normalize_used,rc_n,rc_t,hits,note",
			"--target-cmd", `printf '{"a":1}\n'`)
		if rc != 0 {
			t.Fatalf("json_full: rc=%d", rc)
		}
		got := ctocLastLine(out)
		for _, want := range []string{`"hits": []`, `"note": ""`, `"rc": 0`, `"step_runs": 1`, `"max_targets": 1`} {
			if !strings.Contains(got, want) {
				t.Fatalf("json_full 缺 %s：%q", want, got)
			}
		}
	})

	// ── 负控 ⑪：`--json` 字段名打错 ⇒ rc=2 ＋ 印可选字段表（不静默、不炸堆栈）───────────
	t.Run("json_bad_field_rc2", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--json", "rc,nope")
		if rc != 2 || out != "" {
			t.Fatalf("json_bad: rc=%d out=%q", rc, out)
		}
		for _, want := range []string{
			"⛔ `--json` 不认的字段名：`nope`",
			"   可选字段表：`rc` · `target` · `cmd` · `step_runs` · `max_targets` · `equal_raw` · `equal_norm` · `normalize_used` · `rc_n` · `rc_t` · `hits` · `note`",
			"⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开",
		} {
			if !strings.Contains(errb, want) {
				t.Fatalf("json_bad 缺 %q：%q", want, errb)
			}
		}
	})

	// ── 负控 ⑫：未知旗标 ⇒ rc=2 ＋ usage 两行 + argparse 报错（逐字）──────────────────
	t.Run("unknown_flag_rc2", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--wat")
		if rc != 2 || out != "" {
			t.Fatalf("unknown: rc=%d out=%q", rc, out)
		}
		want := ctocUsageHead + "check-tty-output-contract.py: error: unrecognized arguments: --wat\n"
		if errb != want {
			t.Fatalf("unknown stderr 逐字失配：\n实得 %q\n期望 %q", errb, want)
		}
	})

	// ── 负控 ⑬：`--json` 缺值 ⇒ argparse 报错（先于 pre-scan 命中）─────────────────────
	t.Run("json_missing_value_rc2", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--json")
		if rc != 2 || out != "" {
			t.Fatalf("json_missing: rc=%d out=%q", rc, out)
		}
		if !strings.Contains(errb, "check-tty-output-contract.py: error: argument --json: expected one argument") {
			t.Fatalf("json_missing stderr 失配：%q", errb)
		}
	})

	// ── 自检 ⑭：`--self-test` 全过（9 断言 · 含 must-fail 真红）───────────────────────
	t.Run("self_test_all_pass", func(t *testing.T) {
		rc, out, errb := ctocTestRun("--self-test")
		if errb != "" {
			t.Fatalf("self-test stderr 应为空：%q", errb)
		}
		if rc != 0 {
			t.Fatalf("self-test: rc=%d（脚本相位不足或判据回归）out=%q", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过（断言 9 条 · 失败 0 条）") {
			t.Fatalf("self-test 结论行失配：%q", out)
		}
	})

	// ── 三态分离 ⑮：`null`（取不到）⟷ `[]`/`\"\"`（空集合）⟷ 值 —— 逐条不兼表 ────────────
	t.Run("note_json_null_not_false", func(t *testing.T) {
		run := ctocRun{note: "一侧没落 transcript（缺件）", hasNote: true}
		js := ctocJSONFor([]string{}, 2, ctocDefaultTargets[0], run, []string{}, nil)
		for _, want := range []string{`"equal_raw": null`, `"equal_norm": null`, `"normalize_used": null`,
			`"rc_n": null`, `"rc_t": null`, `"hits": []`, `"note": "一侧没落 transcript（缺件）"`} {
			if !strings.Contains(js, want) {
				t.Fatalf("note-JSON 缺 %s：%s", want, js)
			}
		}
		if strings.Contains(js, `"equal_raw": false`) {
			t.Fatalf("note-JSON：null 被误写成 false（取不到 ≠ 空集合）：%s", js)
		}
	})

	// ── 空 vs 缺 ⑯：judge 对「探针文件取不到」⇒ rc=2（不给结论），不当作 isatty=0 ──────────
	t.Run("missing_probe_is_not_zero", func(t *testing.T) {
		run := ctocRunPair(`printf '{"a":1}\n'`, ctocTimeoutSecs)
		if run.hasNote {
			t.Skipf("本机取不到真两态（note=%q）⇒ 该格不适用", run.note)
		}
		run.isattyN = "取不到"
		rc, _, info := ctocJudge(run)
		if rc != 2 || info == nil || info.present {
			t.Fatalf("missing_probe: rc=%d info=%v（期望 2 · present=false —— 「取不到」不得当 0）", rc, info)
		}
	})

	// ── A 类自回收 ⑰：跑一发前后 TMPDIR 同前缀目录增量 = 0（删 defer os.RemoveAll 即红）────
	t.Run("tempdir_delta_zero", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
		before := ctocCountTempDirs(t)
		rc, _, _ := ctocTestRun("--target-cmd", `printf '{"a":1}\n'`)
		if rc != 0 {
			t.Fatalf("tempdir 子用例的靶档应先绿：rc=%d", rc)
		}
		if got := ctocCountTempDirs(t); got != before {
			t.Fatalf("A 类自回收失败：zerg-tty- 目录增量 %d（期望 0）", got-before)
		}
	})
}

func ctocMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

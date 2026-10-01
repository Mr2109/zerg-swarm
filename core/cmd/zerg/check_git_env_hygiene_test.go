// check_git_env_hygiene_test.go —— Go 真搬样板的**成对负控**定向单测（M2 路B续6 · 4.2「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿（官方全清 / env 全清）」「必然判红（--strict 清面不全）」「不给结论」三态 ——
// 任一态被写死都必失败。
// 用例名：TestCheckGitEnvHygieneGatePort（定向：go test ./cmd/zerg -run TestCheckGitEnvHygieneGatePort）。
//
// 负控有牙的证法（见本单 SUMMARY ⑤）：把判据分支在**仓外副本**上哑化（例：把 `clears_all` 的
// `!have[f] → false` 写成 `continue`）⇒ 正控/负控那条断言当场 FAIL —— 落仓件一字未动。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gehFullVars —— 与 py 现跑 `git rev-parse --local-env-vars` 同形的正源名单（自检/夹具用 · 逐字）。
var gehFullVars = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
	"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

// gehFixture —— 在 t.TempDir()（仓外）造一棵含 `scripts/gates/<name>` 的夹具树。
func gehFixture(t *testing.T, shBody string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "site.sh"), []byte(shBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// gehFullEnvLine —— `env -u <全部正源变量> git …` 形态（必然达标）。
func gehFullEnvLine() string {
	return "cd ${root} && env " + strings.Join(prefixAll("-u ", gehFullVars), " ") + " git status --porcelain\n"
}

func prefixAll(p string, xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = p + x
	}
	return out
}

func gehRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := gitEnvHygieneGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckGitEnvHygieneGatePort(t *testing.T) {
	// ── 正控 A（必然判绿）：官方形态 `unset $(git rev-parse --local-env-vars)` ⇒ rc=0 ──
	official := "# foreign\nforeign_desc=$(unset $(git rev-parse --local-env-vars); git -C ../f describe)\n"
	rcOff, outOff, _ := gehRun(t, "--repo", gehFixture(t, official))
	if rcOff != 0 || !strings.Contains(outOff, "[官方形态(unset 全清)] 清面 15/15") ||
		!strings.Contains(outOff, "⇒ 全绿（rc=0）") {
		t.Fatalf("正控-官方形态 失败：应 rc=0，得 rc=%d out=%q", rcOff, outOff)
	}

	// ── 正控 B（必然判绿）：env 形态**全清** 15 个 ⇒ rc=0 ──────────────────
	rcFull, outFull, _ := gehRun(t, "--repo", gehFixture(t, gehFullEnvLine()))
	if rcFull != 0 || !strings.Contains(outFull, "[env -u] 清面 15/15") {
		t.Fatalf("正控-env全清 失败：应 rc=0 且 15/15，得 rc=%d out=%q", rcFull, outFull)
	}

	// ── 负控①（必然判红）：env 形态清面不全（只清 3 个）⇒ **--strict** rc=1 ──
	gap := "cd ${root} && env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git diff --name-only\n"
	rcGapStrict, outGapStrict, _ := gehRun(t, "--repo", gehFixture(t, gap), "--strict")
	if rcGapStrict != 1 || !strings.Contains(outGapStrict, "清面 3/15") ||
		!strings.Contains(outGapStrict, "⇒ 判红（rc=1 · `--strict`）") {
		t.Fatalf("负控①-strict 失败：应 rc=1 且点名 3/15，得 rc=%d out=%q", rcGapStrict, outGapStrict)
	}
	// 报告档（同夹具 · 无 --strict）⇒ rc=0 但仍点名未达标（「只告警不拦」≠「没判到」）
	rcGapReport, outGapReport, _ := gehRun(t, "--repo", gehFixture(t, gap))
	if rcGapReport != 0 || !strings.Contains(outGapReport, "⇒ 判到了（rc=0 · **报告档**）") {
		t.Fatalf("负控①-报告档 失败：应 rc=0 且报「判到了」，得 rc=%d out=%q", rcGapReport, outGapReport)
	}
	// 成对（✗ 禁恒绿/恒红）：同一夹具 报告档 rc 与 strict rc 必须不同码
	if rcGapReport == rcGapStrict {
		t.Fatalf("成对断言失败：报告档 rc=%d 与 strict rc=%d 同码 ⇒ 无鉴别力", rcGapReport, rcGapStrict)
	}

	// ── 负控②：空转（站点 0 个）⇒ rc=2（空转 ≠ 绿）─────────────────────
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "scripts", "gates", "readme.md"), []byte("# none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcEmpty, outEmpty, _ := gehRun(t, "--repo", empty)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "站点 0 个") {
		t.Fatalf("负控②-空转 失败：应 rc=2，得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 取不到输入：--repo 不存在 ⇒ rc=2（`跑不动 git`）────────────────────
	rcNo, outNo, _ := gehRun(t, "--repo", filepath.Join(t.TempDir(), "nope"))
	if rcNo != 2 || !strings.Contains(outNo, "跑不动 git") {
		t.Fatalf("取不到输入 失败：应 rc=2，得 rc=%d out=%q", rcNo, outNo)
	}

	// ── 未知旗标（argparse 拒收）⇒ rc=2 · stderr 逐字面 ──────────────────
	rcFlag, outFlag, errFlag := gehRun(t, "--repo", gehFixture(t, official), "--no-such-flag")
	if rcFlag != 2 || outFlag != "" || !strings.Contains(errFlag, "error: unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标 失败：得 rc=%d out=%q stderr=%q", rcFlag, outFlag, errFlag)
	}

	// ── Q-176：`--json` 坏字段名 ⇒ rc=2 · stderr 印可选字段表 ────────────
	rcBadJ, _, errBadJ := gehRun(t, "--repo", gehFixture(t, gap), "--json", "zzz-bad")
	if rcBadJ != 2 || !strings.Contains(errBadJ, "`--json` 不认的字段名：`zzz-bad`") ||
		!strings.Contains(errBadJ, "可选字段表") {
		t.Fatalf("--json 坏字段 失败：得 rc=%d stderr=%q", rcBadJ, errBadJ)
	}

	// ── `--json` 机器面：恒 dump 全 9 键（含 mode）⇒ rc=0 ────────────────
	rcJ, outJ, _ := gehRun(t, "--repo", gehFixture(t, gap), "--json", "full_env_vars,sites,gaps")
	if rcJ != 0 || !strings.Contains(outJ, `{"schema": "zerg-gate/v1", "gate": "check-git-env-hygiene"`) ||
		!strings.Contains(outJ, `"mode": "report"`) || !strings.Contains(outJ, `"gaps": 1`) {
		t.Fatalf("--json 失败：得 rc=%d out=%q", rcJ, outJ)
	}

	// ── `-h`：帮助正文 ⇒ rc=0 ──────────────────────────────────────────
	rcH, outH, _ := gehRun(t, "-h")
	if rcH != 0 || !strings.HasPrefix(outH, gehUsageLine+"\n") || !strings.Contains(outH, "--list-rules") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── `--self-test`：13 格断言自身必须全过（防恒绿/恒红）──────────────
	rcST, outST, _ := gehRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检 13 格 · 失败 0 格") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}

	// ── `--list-rules` ⇒ rc=0 ──────────────────────────────────────────
	rcLR, outLR, _ := gehRun(t, "--list-rules")
	if rcLR != 0 || !strings.Contains(outLR, "清面 ⊇ 全量") {
		t.Fatalf("--list-rules 失败：得 rc=%d out=%q", rcLR, outLR)
	}

	// ── 成对总断言（✗ 禁恒绿）：绿 0 / 红 1 / 不给结论 2 三码都实证到 ──────
	if !(rcOff == 0 && rcGapStrict == 1 && rcEmpty == 2) {
		t.Fatalf("三态成对失败：绿=%d 红=%d 判不了=%d", rcOff, rcGapStrict, rcEmpty)
	}
}

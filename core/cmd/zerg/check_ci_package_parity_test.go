// check_ci_package_parity_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · §4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红（判据 A / 判据 B）」「不给结论（空转 / 缺件 / 用法错）」
// 三态 —— 任一态被写死（恒绿或恒红）都必失败。
// 用例名：TestCheckCiPackageParityGatePort（定向：go test -run TestCheckCiPackageParityGatePort）。
// 子用例（9 条 · `-v` 可逐条现读）：
//
//	正控-PC-两侧一致-rc0 · 负控A-只在名单件-rc1 · 负控B-名单外包进CI-rc1 · 空转VAC1-tsv零in-ci-rc2 ·
//	取不到输入-ci缺件-rc2 · 机器面-json合法子集 · 机器面-json表外字段-rc2 · 自检-self-test-rc0 · 用法错-未知旗标-rc2
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cippTestSurface —— 与 py `GOOD_SURFACE` 同形的合成名单（三个 in-ci + 一个 out-ci）。
const cippTestSurface = "# 单测夹具（不碰真目标）\n" +
	"core\tinternal/alpha\tin-ci\t纯源码判据\t—\n" +
	"core\tinternal/beta\tin-ci\t纯源码判据\t—\n" +
	"core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n" +
	"agent\tinternal/delta\tin-ci\t纯源码判据\t—\n"

// cippTestWrite —— 写夹具件（自动建父目录）。
func cippTestWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cippTestCITemplate —— 与 py `CI_TEMPLATE` 同形（逐包续行块 + 一份单行 `go test -run` 干扰项）。
const cippTestCITemplate = `jobs:
  test:
    steps:
      - name: core
        working-directory: core
        run: |
          go test \
%s            -count=1
      - name: agent
        working-directory: agent
        run: |
          go test \
%s            -count=1
  compat:
    steps:
      - name: compat
        working-directory: core
        run: go test ./internal/compat/ -run 'Compat' -count=1
`

// cippTestRepo —— 造一棵含 tsv + ci.yml 的合成仓根（仓外 · t.TempDir()）。
func cippTestRepo(t *testing.T, surface string, corePkgs, agentPkgs []string) string {
	t.Helper()
	root := t.TempDir()
	cippTestWrite(t, root, cippTSVRel, surface)
	body := strings.Replace(cippTestCITemplate, "%s", cippPkgLines(corePkgs), 1)
	body = strings.Replace(body, "%s", cippPkgLines(agentPkgs), 1)
	cippTestWrite(t, root, cippCIRel, body)
	return root
}

func cippTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := ciPackageParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckCiPackageParityGatePort(t *testing.T) {
	goodCore := []string{"internal/alpha", "internal/beta"}
	goodAgent := []string{"internal/delta"}

	// 夹具（全部仓外 · t.TempDir()）：好仓 / 只在名单件 / 名单外包进 CI / 空转（tsv 零 in-ci）/ 无 CI 件
	fxGood := cippTestRepo(t, cippTestSurface, goodCore, goodAgent)
	fxOnlySurface := cippTestRepo(t, cippTestSurface+"core\tinternal/epsilon\tin-ci\t纯源码判据\t—\n", goodCore, goodAgent)
	fxOutInCI := cippTestRepo(t, cippTestSurface, []string{"internal/alpha", "internal/beta", "cmd/gamma"}, goodAgent)
	fxVacuous := cippTestRepo(t, "core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n", goodCore, goodAgent)
	fxNoCI := t.TempDir()
	cippTestWrite(t, fxNoCI, cippTSVRel, cippTestSurface)

	t.Run("正控-PC-两侧一致-rc0", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxGood, "--no-self-test")
		if rc != 0 {
			t.Fatalf("正控-PC 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "⇒ 全绿（rc=0）：名单件与 `ci.yml` 的逐包点名**逐字一致**") {
			t.Fatalf("正控-PC 失败：绿判词不见 out=%q", out)
		}
		if !strings.Contains(out, "`in-ci` **3** 个包 · 名单外（`out-ci`+`no-test`）**1** 个包") {
			t.Fatalf("正控-PC 失败：计数行不对 out=%q", out)
		}
	})

	t.Run("负控A-只在名单件-rc1", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxOnlySurface, "--no-self-test")
		if rc != 1 {
			t.Fatalf("负控-A 失败：得 rc=%d（期望 1）out=%q", rc, out)
		}
		if !strings.Contains(out, "       ── core  ./internal/epsilon/") {
			t.Fatalf("负控-A 失败：没逐条点名 epsilon out=%q", out)
		}
	})

	t.Run("负控B-名单外包进CI-rc1", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxOutInCI, "--no-self-test")
		if rc != 1 {
			t.Fatalf("负控-B 失败：得 rc=%d（期望 1）out=%q", rc, out)
		}
		if !strings.Contains(out, "名单**外**（`out-ci`/`no-test`）的包出现在 `ci.yml` 点名表里 1 个") {
			t.Fatalf("负控-B 失败：判据 B 判词不见 out=%q", out)
		}
	})

	t.Run("空转VAC1-tsv零in-ci-rc2", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxVacuous, "--no-self-test")
		if rc != 2 {
			t.Fatalf("空转-VAC 失败：得 rc=%d（期望 2）out=%q", rc, out)
		}
		if !strings.Contains(out, "守卫 C：名单件里 `in-ci` 行 **0 条** ⇒ 判据不可判 ⇒ 不给结论（rc=2）") {
			t.Fatalf("空转-VAC 失败：守卫 C 判词不见 out=%q", out)
		}
	})

	t.Run("取不到输入-ci缺件-rc2", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxNoCI, "--no-self-test")
		if rc != 2 {
			t.Fatalf("缺件失败：得 rc=%d（期望 2）out=%q", rc, out)
		}
		if !strings.Contains(out, "CI 件读不到（") || !strings.Contains(out, "⇒ 不给结论（rc=2）") {
			t.Fatalf("缺件失败：判词不见 out=%q", out)
		}
	})

	t.Run("机器面-json合法子集", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--root", fxGood, "--no-self-test", "--json", "only_surface,surface,out_ci")
		if rc != 0 {
			t.Fatalf("--json 失败：得 rc=%d out=%q", rc, out)
		}
		want := `{"only_surface": [], "out_ci": [["core", "cmd/gamma"]], ` +
			`"surface": [["agent", "internal/delta"], ["core", "internal/alpha"], ["core", "internal/beta"]]}`
		if !strings.Contains(out, want) {
			t.Fatalf("--json 失败：机器面形态不对（want 含 %s）out=%q", want, out)
		}
	})

	t.Run("机器面-json表外字段-rc2", func(t *testing.T) {
		rc, _, errOut := cippTestRun(t, "--json", "zzz")
		if rc != 2 {
			t.Fatalf("--json 表外失败：得 rc=%d（期望 2）", rc)
		}
		if !strings.Contains(errOut, "⛔ `--json` 不认的字段名：`zzz`") {
			t.Fatalf("--json 表外失败：stderr 判词不见 err=%q", errOut)
		}
	})

	t.Run("自检-self-test-rc0", func(t *testing.T) {
		rc, out, _ := cippTestRun(t, "--self-test")
		if rc != 0 {
			t.Fatalf("--self-test 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过（断言 12 条 · 失败 0 条）") {
			t.Fatalf("--self-test 失败：结论行不对 out=%q", out)
		}
	})

	t.Run("用法错-未知旗标-rc2", func(t *testing.T) {
		rc, _, errOut := cippTestRun(t, "--no-such-flag")
		if rc != 2 {
			t.Fatalf("未知旗标失败：得 rc=%d（期望 2）", rc)
		}
		if !strings.Contains(errOut, "用法错（rc=2）") {
			t.Fatalf("未知旗标失败：stderr 不见 err=%q", errOut)
		}
	})
}

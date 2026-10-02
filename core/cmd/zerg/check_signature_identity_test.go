// check_signature_identity_test.go —— `check_signature_identity.go` 的**定向单测 · 成对负控**（M2 · 承接 GAP-20261001-13）。
//
// 口径（与 py 原件两态对拍之外的「有牙」证据）：本用例**逐格判** rc 与**逐字判** stdout ——
// 任一条规则被哑化（G1 不判 / G2 不判 / G3 不判 / 缺件当绿 / 未签当无结论 / 空段 join 被 Clean /
// normpath 双前导斜杠被折 / JSON 键序乱 / 未转义反斜杠 / 自检临时目录不回收）⇒ **当场 FAIL**。
// 负控有牙的实证 = 把落在**仓外副本**上的同一件哑化一条判据后重跑本用例 ⇒ FAIL（见交付表的逐字读数 · 带行号）。
//
// ✗ 本件不接线、不注册命令、不删 py 原件（M2 边界）· 只跑 macOS 本机 `codesign`（非 darwin 跳过）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// csidTestCopy —— 把 /bin/echo 复制到 dst（字节级 · 自检夹具同法）。
func csidTestCopy(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", src, err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatalf("写 %s 失败：%v", dst, err)
	}
}

// csidTestRun —— 进程内直调入口（同 argv · 与 py 打印文案逐字对齐）。
func csidTestRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkSignatureIdentityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// csidTestFixture —— 造苹果签名件 / ad-hoc 件 / 未签件 / 目录件四态于 t.TempDir()（仓外）。
func csidTestFixture(t *testing.T) (dir, apple, adhoc, unsigned, adir string) {
	t.Helper()
	dir = t.TempDir()
	apple = filepath.Join(dir, "apple")
	csidTestCopy(t, "/bin/echo", apple)

	adhoc = filepath.Join(dir, "adhoc")
	csidTestCopy(t, "/bin/echo", adhoc)
	if err := exec.Command("codesign", "-s", "-", "--force", adhoc).Run(); err != nil {
		t.Fatalf("造 ad-hoc 夹具失败：%v", err)
	}

	unsigned = filepath.Join(dir, "unsigned")
	csidTestCopy(t, "/bin/echo", unsigned)
	_ = exec.Command("codesign", "--remove-signature", unsigned).Run()

	adir = filepath.Join(dir, "adir")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("造目录夹具失败：%v", err)
	}
	return
}

func csidRequireMac(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("codesign"); err != nil {
		t.Skip("无 `codesign`（非 macOS）⇒ 签名档面跳过")
	}
	if _, err := os.Stat("/bin/echo"); err != nil {
		t.Skip("无 /bin/echo 夹具源")
	}
}

func TestCheckSignatureIdentityGatePort(t *testing.T) {
	csidRequireMac(t)

	// ① 正控：苹果原样签名件 ⇒ rc=0 且 stdout 逐字到「全过」行。
	t.Run("正控-苹果签名件绿", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, errb := csidTestRun("--root", dir, "--no-self-test", "--piece", "apple")
		if rc != 0 {
			t.Fatalf("正控 失败：rc=%d out=%q err=%q", rc, out, errb)
		}
		if !strings.Contains(out, "  ✓ apple ") ||
			!strings.Contains(out, "Identifier=com.apple.echo · flags=0x0(none) · Signature=\n") ||
			!strings.Contains(out, "       三格全过\n") ||
			!strings.Contains(out, "⇒ 全过（rc=0）：被测件都不是 ad-hoc 件（`Identifier` ≠ `a.out` · `flags` 不含 `adhoc`）\n") {
			t.Fatalf("正控 stdout 不符：%q", out)
		}
		if errb != "" {
			t.Fatalf("正控 stderr 应空：%q", errb)
		}
	})

	// ② 负控：ad-hoc 件 ⇒ rc=1 且逐字点名 G2（哑化 G2 判据必 FAIL）。
	t.Run("负控-adhoc件红并点名G2", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", "adhoc")
		if rc != 1 {
			t.Fatalf("负控-adhoc 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "✗ adhoc ") || !strings.Contains(out, "G2：`flags=0x2(adhoc)` 含 `adhoc`") {
			t.Fatalf("负控-adhoc 未逐字点名 G2：%q", out)
		}
		if !strings.Contains(out, "⇒ 判红（rc=1）：上面带 ✗ 的件不是稳定身份签名") {
			t.Fatalf("负控-adhoc 结论行不符：%q", out)
		}
	})

	// ③ 负控：未签件 ⇒ rc=1（不许与「缺件」混成同一个码）。
	t.Run("负控-未签件红", func(t *testing.T) {
		dir, _, _, unsigned, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", unsigned)
		if rc != 1 {
			t.Fatalf("负控-未签 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "该件**没有签名**（`code object is not signed at all`）") {
			t.Fatalf("负控-未签 未点名：%q", out)
		}
	})

	// ④ 成对区分度：同一件只改「签 / 不签」一个变量 ⇒ 一绿一红。
	t.Run("成对-同件只改签不签", func(t *testing.T) {
		dir, apple, _, unsigned, _ := csidTestFixture(t)
		rcA, _, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", apple)
		rcU, _, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", unsigned)
		if rcA != 0 || rcU != 1 {
			t.Fatalf("成对区分度失败：苹果件 rc=%d（要 0）· 未签件 rc=%d（要 1）", rcA, rcU)
		}
	})

	// ⑤ 缺件 ⇒ rc=2（哑化「缺件不给结论」必 FAIL）。
	t.Run("缺件-不给结论", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", filepath.Join(dir, "不存在这件"))
		if rc != 2 {
			t.Fatalf("缺件 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "被测件不在（缺件 ⇒ 不给结论）") ||
			!strings.Contains(out, "⇒ 不给结论（rc=2）：上面带 ⛔ 的件读不到（缺件 / 工具不在）") {
			t.Fatalf("缺件 未逐字点名：%q", out)
		}
	})

	// ⑥ 目录件（isfile=True 不成立）⇒ rc=2。
	t.Run("目录件-不给结论", func(t *testing.T) {
		dir, _, _, _, adir := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--no-self-test", "--piece", adir)
		if rc != 2 {
			t.Fatalf("目录件 失败：rc=%d out=%q", rc, out)
		}
	})

	// ⑦ `--list-rules` 逐字（哑化 RULES 表必 FAIL）。
	t.Run("list-rules逐字", func(t *testing.T) {
		rc, out, errb := csidTestRun("--list-rules")
		if rc != 0 || errb != "" {
			t.Fatalf("list-rules 失败：rc=%d err=%q", rc, errb)
		}
		want := "判据（三格 · 逐件独立判）：\n" +
			"  G1 `Identifier` 不得为 `a.out`（Go linker ad-hoc 形态）\n" +
			"  G2 `flags` 不得含 `adhoc`（`0x2(adhoc)` / `0x20002(adhoc,linker-signed)`）\n" +
			"  G3 `Signature` 不得为 `adhoc`（该行不出现时这一格不判）\n" +
			"被测件（默认）：bin/zerg · bin/zerg-core\n" +
			"退码：0 全绿 · 1 有红（三格之一不过 / 未签）· 2 不给结论（缺件 / 无 codesign / 自检不过）\n" +
			"自检：① ad-hoc 件必红 ② 苹果签名件必绿 ③ 未签件必红 ④ 缺件必 2\n"
		if out != want {
			t.Fatalf("list-rules stdout 不符：\n得 %q\n要 %q", out, want)
		}
	})

	// ⑧ `--self-test` 成对负控 7 断言 ⇒ rc=0（哑化任一夹具断言必 FAIL）。
	t.Run("self-test-七断言", func(t *testing.T) {
		rc, out, _ := csidTestRun("--self-test")
		if rc != 0 {
			t.Fatalf("自检 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "  ✓ 夹具① ad-hoc 件 ⇒ **必判红**（负控半边）\n") ||
			!strings.Contains(out, "  ✓ 夹具② 原样苹果签名件 ⇒ **必判绿**（正控半边）\n") ||
			!strings.Contains(out, "自检结论: 全过（断言 7 条 · 失败 0 条）\n") {
			t.Fatalf("自检 stdout 不符：%q", out)
		}
	})

	// ⑨ `--json`：键序逐字 + 「三格齐全时 signature 是**存在的空串**（不是省略）」。
	t.Run("json-键序与空串", func(t *testing.T) {
		dir, apple, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--json", "--no-self-test", "--piece", apple)
		if rc != 0 {
			t.Fatalf("json 失败：rc=%d out=%q", rc, out)
		}
		want := `{
  "root": "` + dir + `",
  "rows": [
    {
      "piece": "` + apple + `",
      "path": "` + apple + `",
      "identifier": "com.apple.echo",
      "flags": "0x0(none)",
      "signature": "",
      "cdhash": "` + "dfddf159b0891153c1f3b7cb305ccbf5b90a82db" + `",
      "reason": "三格全过",
      "verdict": "ok"
    }
  ],
  "ok": 1,
  "red": 0,
  "blocked": 0
}
⇒ 全过（rc=0）：被测件都不是 ad-hoc 件（` + "`Identifier` ≠ `a.out` · `flags` 不含 `adhoc`" + `）
`
		if out != want {
			t.Fatalf("json stdout 不符：\n得 %q\n要 %q", out, want)
		}
	})

	// ⑩ Join 空段：`--piece ""` ⇒ join(root,"") == root+"/"（哑化 csidPyJoin 空段必 FAIL）。
	t.Run("join-空段", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--json", "--no-self-test", "--piece", "")
		if rc != 2 {
			t.Fatalf("join-空段 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, `"path": "`+dir+`/",`) {
			t.Fatalf("join-空段 未保尾斜杠（root+空段 应得 root+/）：%q", out)
		}
	})

	// ⑪ Join 吃点段：`--piece a/../b` 必须**原样串拼**（✗ 不 Clean ⇒ 哑化必 FAIL）。
	t.Run("join-吃点段不归并", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--json", "--no-self-test", "--piece", "bin/../bin/x")
		if rc != 2 {
			t.Fatalf("join-吃点段 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, `"path": "`+dir+`/bin/../bin/x",`) {
			t.Fatalf("join-吃点段 被归并（应原样）：%q", out)
		}
	})

	// ⑫ normpath 双前导斜杠：`--root //` 必须保留 `//`（✗ filepath.Clean 会折 ⇒ 哑化必 FAIL）。
	t.Run("normpath-双前导斜杠保留", func(t *testing.T) {
		rc, out, _ := csidTestRun("--root", "//", "--json", "--no-self-test")
		if rc != 2 {
			t.Fatalf("normpath 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, `"root": "//"`) || !strings.Contains(out, `"path": "//bin/zerg"`) {
			t.Fatalf("normpath 双前导斜杠被折：%q", out)
		}
		rc2, out2, _ := csidTestRun("--root", "///", "--json", "--no-self-test")
		if rc2 != 2 || !strings.Contains(out2, `"root": "/"`) {
			t.Fatalf("normpath 三斜杠应折成一个：rc=%d %q", rc2, out2)
		}
	})

	// ⑬ JSON 转义安全：含反斜杠的 `--piece` 串必须转义（✗ 未转义 ⇒ 整单被平台拒收）。
	t.Run("json-反斜杠转义", func(t *testing.T) {
		dir, _, _, _, _ := csidTestFixture(t)
		rc, out, _ := csidTestRun("--root", dir, "--json", "--no-self-test", "--piece", `a\b`)
		if rc != 2 {
			t.Fatalf("转义 失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, `"piece": "a\\b"`) || !strings.Contains(out, `"path": "`+dir+`/a\\b"`) {
			t.Fatalf("反斜杠未转义安全：%q", out)
		}
	})

	// ⑭ 自回收：跑两趟 `--self-test` 后，TMPDIR 里 zerg-sigident- 前缀目录数必须回到 0（delta=0）。
	t.Run("自回收-临时目录delta0", func(t *testing.T) {
		root := t.TempDir() // ★ 专属根：自检目录随 TMPDIR 落在这里，前后计数只数自己
		// 旧写法数共享 TMPDIR → 别的进程同时建/删 `zerg-sigident-` 就翻 delta ⇒ 假红（GAP-20261003-01、-race 档 4 片并跑时更易碰）。
		t.Setenv("TMPDIR", root)
		count := func() int {
			es, _ := os.ReadDir(root)
			n := 0
			for _, e := range es {
				if strings.HasPrefix(e.Name(), "zerg-sigident-") {
					n++
				}
			}
			return n
		}
		before := count()
		for i := 0; i < 2; i++ {
			if rc, out, _ := csidTestRun("--self-test"); rc != 0 {
				t.Fatalf("自回收 前置失败：rc=%d out=%q", rc, out)
			}
		}
		after := count()
		if after-before != 0 {
			t.Fatalf("自检临时目录未回收：before=%d after=%d delta=%d", before, after, after-before)
		}
	})
}

package main

// check_exit_codes_test.go —— 门⑨ Go 真搬的**成对负控**测试件（M2 · 刻意无调用点）。
//
// 成对结构（✗ 不是恒绿）：
//   正控 = 好夹具 ⇒ rc=0 · 负控 = 逐条造 violation ⇒ rc=1/2（各一条错判即红）。
//   哑化取证（回执）：把**仓外副本**里对应判据掐掉 ⇒ 本件当场 FAIL 并给行号。
//
// ★ 自指（标准声明 (b)）：本件同样落 core/ ⇒ 也在 R6 面内。同上，被扫形态一律**拆串**。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cecTWrite(t *testing.T, dir, rel, text string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func cecTWriteBytes(t *testing.T, dir, rel string, b []byte) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func cecTRun(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{root}, args...)
	rc := checkExitCodesGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// TestCheckExitCodesGatePort 成对负控（正控 1 格 + 负控 6 格 + 空转 1 格 + 读不了件 1 格）。
func TestCheckExitCodesGatePort(t *testing.T) {
	e8 := "ex" + "it 8" // 被扫形态拆串（自指形态③）
	good := func(t *testing.T) string {
		d := t.TempDir()
		cecFixture(d, cecOver{})
		return d
	}

	t.Run("正控_好夹具_rc0", func(t *testing.T) {
		rc, out, _ := cecTRun(t, good(t))
		if rc != 0 {
			t.Fatalf("rc=%d 应为 0\n%s", rc, out)
		}
		if !strings.Contains(out, "全绿（rc=0）") {
			t.Fatalf("判词缺全绿档：%s", out)
		}
	})

	t.Run("负控_R1_主表数字重复_rc1", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{table: cecTPtr(strings.Replace(cecGoodTable,
			"{2, \"usage\", \"用法错 / **不给结论**\", \"改用法后可重试\"},",
			"{0, \"dup\", \"重复的 0\", \"—\"},", 1))})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 {
			t.Fatalf("rc=%d 应为 1\n%s", rc, out)
		}
		if !strings.Contains(out, "FAIL R1") {
			t.Fatalf("判词缺 R1：%s", out)
		}
	})

	t.Run("负控_R2_门禁行不逐字一致_rc1", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{gateFileLine: cecTPtr("退出码：0 全绿 · 1 有失败项 · 2 不给结论")})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 || !strings.Contains(out, "FAIL R2") {
			t.Fatalf("rc=%d 应 1 且含 FAIL R2\n%s", rc, out)
		}
	})

	t.Run("负控_R3_声明面缺标记行_rc1", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{decl: cecTPtr(strings.Replace(cecGoodDecl,
			"# "+cecMarker+"（契约 §三）—— 本件是组件自有码空间\n", "", 1))})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 || !strings.Contains(out, "FAIL R3") {
			t.Fatalf("rc=%d 应 1 且含 FAIL R3\n%s", rc, out)
		}
	})

	t.Run("负控_R4_回滚成功又写成2_rc1", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{swap: cecTPtr(strings.Replace(cecGoodSwap,
			"# 「回滚成功」= 1 + error.kind=ROLLED_BACK（不是 0，也不是 2）\n",
			"# 回滚成功 = 2（老说法）\n", 1))})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 || !strings.Contains(out, "FAIL R4") {
			t.Fatalf("rc=%d 应 1 且含 FAIL R4\n%s", rc, out)
		}
	})

	t.Run("负控_R5_用未登记退码_rc1", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{swap: cecTPtr(cecGoodSwap + "exit 7\n")})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 || !strings.Contains(out, "FAIL R5") {
			t.Fatalf("rc=%d 应 1 且含 FAIL R5\n%s", rc, out)
		}
	})

	t.Run("负控_R7_同号未签字_rc1", func(t *testing.T) {
		d := t.TempDir()
		r := cecGoodRoster()
		r["signed_differences"] = []any{map[string]any{
			"site": "scripts/build/zerg-swap-core.sh", "code": 8, "cli_code": 8, "reason": "夹具"}}
		cecFixture(d, cecOver{table: cecTPtr(cecGoodTable8), roster: r,
			decl: cecTPtr(cecGoodDecl + "// 值无效：空串参数 ⇒ " + cecEC8 + "\n")})
		rc, out, _ := cecTRun(t, d)
		if rc != 1 || !strings.Contains(out, "FAIL R7") {
			t.Fatalf("rc=%d 应 1 且含 FAIL R7\n%s", rc, out)
		}
	})

	t.Run("空转_登记表缺件_rc2", func(t *testing.T) {
		d := t.TempDir()
		cecFixture(d, cecOver{})
		if err := os.Remove(filepath.Join(d, cecSitesRel)); err != nil {
			t.Fatal(err)
		}
		rc, out, _ := cecTRun(t, d)
		if rc != 2 || !strings.Contains(out, "不给结论（rc=2）") {
			t.Fatalf("rc=%d 应 2（✗ 不静默绿）\n%s", rc, out)
		}
	})

	t.Run("取不到根_rc2", func(t *testing.T) {
		empty := t.TempDir()
		rc, out, _ := cecTRun(t, empty)
		if rc != 2 || !strings.Contains(out, "找不到 core/cmd/zerg") {
			t.Fatalf("rc=%d 应 2\n%s", rc, out)
		}
	})

	// ★「读不了的件」成对：同一个被扫形态 —— 合法 UTF-8 的件 ⇒ 计入；非法 UTF-8 的件 ⇒ **两态同形（同静默）**。
	t.Run("读不了的件_与可读件成对", func(t *testing.T) {
		readable := "# 值无效：参数空串 ⇒ " + e8 + "\n" + e8 + "\n"
		d := t.TempDir()
		cecTWrite(t, d, "scripts/build/readable.sh", readable)
		cecTWriteBytes(t, d, "scripts/build/unreadable.sh", []byte("# \xff\xfe \x80\nexit 8\n"))
		_, hits, _ := cecScanValueInvalid8(d)
		nRead, nUnread := 0, 0
		for _, h := range hits {
			if strings.HasSuffix(h.Site, "readable.sh") {
				nRead++
			}
			if strings.HasSuffix(h.Site, "unreadable.sh") {
				nUnread++
			}
		}
		if nRead != 2 {
			t.Fatalf("可读件命中 %d 行 · 应 2（判据没牙）", nRead)
		}
		if nUnread != 0 {
			t.Fatalf("读不了的件命中 %d 行 · 应 0（py `except UnicodeDecodeError: continue` 逐字对齐：同静默跳过）", nUnread)
		}
	})

	// ★「读失败不再静默」回归牙：扫面里一份读不动的 `.sh` ⇒ 登记进 cecReadFails
	// ⇒ 入口非空即 stderr BLOCKED 头 ＋ 逐件点名 ＋ rc=2（B 类静默面已闭）。
	t.Run("读失败件_登记并拦停_rc2", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 无视 0o000 权限位：本格无鉴别力")
		}
		d := t.TempDir()
		cecFixture(d, cecOver{})
		p := filepath.Join(d, "scripts", "build", "teeth-unreadable.sh")
		cecTWrite(t, d, "scripts/build/teeth-unreadable.sh", "#!/usr/bin/env bash\n")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(p, 0o644) }()
		cecReadFails = nil
		rc, _, errb := cecTRun(t, d)
		if rc != 2 {
			t.Fatalf("读不动的件应拦停 rc=2（✗ 不许静默跳过），得 rc=%d stderr=%q", rc, errb)
		}
		if !strings.Contains(errb, "⛔ BLOCKED：判据件读失败") {
			t.Fatalf("stderr 缺 BLOCKED 头：%q", errb)
		}
		if !strings.Contains(errb, p) {
			t.Fatalf("stderr 未逐件点名读不动的件 %s：%q", p, errb)
		}
		if len(cecReadFails) == 0 {
			t.Fatal("cecReadFails 空：读失败仍被静默吞掉")
		}
	})

	t.Run("自检_自跑全过_rc0", func(t *testing.T) {
		rc, out, _ := cecTRun(t, ".")
		_ = rc
		var b bytes.Buffer
		rc2 := cecSelfTestTo(&b)
		if rc2 != 0 || !strings.Contains(b.String(), "自检结论: 全过") {
			t.Fatalf("自检 rc=%d\n%s", rc2, b.String())
		}
		_ = out
	})

	t.Run("--json_无机器读面_rc2_且stderr有正文", func(t *testing.T) {
		rc, out, errb := cecTRun(t, ".")
		_ = rc
		_ = out
		_ = errb
		rc2, o2, e2 := cecTRun(t, ".", "--json")
		if rc2 != 2 {
			t.Fatalf("--json rc=%d 应 2", rc2)
		}
		if o2 != "" {
			t.Fatalf("--json 不许有 stdout：%q", o2)
		}
		if !strings.Contains(e2, "本门无机器读面") {
			t.Fatalf("--json stderr 缺判词：%q", e2)
		}
	})
}

func cecTPtr(s string) *string { return &s }

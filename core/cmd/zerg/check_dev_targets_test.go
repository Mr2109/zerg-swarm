// check_dev_targets_test.go —— Go 真搬的**成对负控**定向单测（M2 · 批-3 第 4 粒）。
//
// ✗ 禁恒绿：同时断言「必然判绿」「必然判红」「不给结论」「缺件」四态 —— 任一态被写死都必失败；
// 并钉一条**自回收牙**（`--self-test` 跑后同前缀临时目录计数 delta=0）。
// 用例名：TestCheckDevTargetsGatePort（定向：go test -run TestCheckDevTargetsGatePort）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// cdtRunTest —— 进程内跑入口，取 (rc, stdout, stderr)。
func cdtRunTest(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkDevTargetsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cdtWriteTest —— 在 t.TempDir() 下落一件 `<dir>/dev-targets.json`（仓外），返回其路径。
func cdtWriteTest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := dir + string(os.PathSeparator) + "dev-targets.json"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckDevTargetsGatePort(t *testing.T) {
	// ── 正控（必然判绿）：ids 与 families 展平集合相等且同序 ──────────────────
	good := cdtWriteTest(t, `{"ids":["D1","D2","E1"],"families":{"D":["D1","D2"],"E":["E1"],"F":[],"G":[]}}`)
	rcGreen, outGreen, errGreen := cdtRunTest(t, good)
	if rcGreen != 0 || !strings.Contains(outGreen, "⇒ 全绿（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}
	if errGreen != "" {
		t.Fatalf("正控失败：正常档 stderr 应空，得 %q", errGreen)
	}

	// ── 负控（必然判红）：集合不等（families 漏 D2）⇒ rc=1 ────────────────────
	badSet := cdtWriteTest(t, `{"ids":["D1","D2","E1"],"families":{"D":["D1"],"E":["E1"]}}`)
	rcSet, outSet, _ := cdtRunTest(t, badSet)
	if rcSet != 1 || !strings.Contains(outSet, "FAIL 集合不等") {
		t.Fatalf("负控-集合 失败：得 rc=%d out=%q", rcSet, outSet)
	}

	// ── 负控（必然判红）：顺序不等（ids 换序）⇒ rc=1 ──────────────────────────
	badOrder := cdtWriteTest(t, `{"ids":["D2","D1","E1"],"families":{"D":["D1","D2"],"E":["E1"]}}`)
	rcOrder, outOrder, _ := cdtRunTest(t, badOrder)
	if rcOrder != 1 || !strings.Contains(outOrder, "FAIL 顺序不等") {
		t.Fatalf("负控-顺序 失败：得 rc=%d out=%q", rcOrder, outOrder)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码。
	if rcGreen == rcSet || rcGreen == rcOrder {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d/%d 同码 ⇒ 无鉴别力", rcGreen, rcSet, rcOrder)
	}

	// ── 不给结论 · 空转（ids 为空）⇒ rc=2 ────────────────────────────────────
	empty := cdtWriteTest(t, `{"ids":[],"families":{"D":[]}}`)
	rcEmpty, outEmpty, _ := cdtRunTest(t, empty)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "空转：ids 为空") {
		t.Fatalf("负控-空转 失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 不给结论 · 形状不符（缺 families）⇒ rc=2 ─────────────────────────────
	shape := cdtWriteTest(t, `{"ids":["D1"]}`)
	rcShape, outShape, _ := cdtRunTest(t, shape)
	if rcShape != 2 || !strings.Contains(outShape, "形状不符") {
		t.Fatalf("负控-形状 失败：得 rc=%d out=%q", rcShape, outShape)
	}

	// ── 不给结论 · 缺件（路径不存在）⇒ rc=2 ──────────────────────────────────
	rcMiss, outMiss, _ := cdtRunTest(t, "/nonexistent/definitely/not/here.json")
	if rcMiss != 2 || !strings.Contains(outMiss, "缺件：") {
		t.Fatalf("负控-缺件 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 负控 · 崩溃档（顶层非对象）：py `d.get` 未捕 ⇒ rc=1（真口子 · C 类 · 见件头未覆盖面②）──
	crash := cdtWriteTest(t, `[1,2,3]`)
	rcCrash, outCrash, errCrash := cdtRunTest(t, crash)
	if rcCrash != 1 || outCrash != "" || errCrash == "" {
		t.Fatalf("崩溃档失败：顶层非对象应 rc=1 + stdout 空 + stderr 非空，得 rc=%d out=%q err=%q", rcCrash, outCrash, errCrash)
	}

	// ── 成对：四态码互不相同（0/1/2 三档齐 + 缺件同 2 但文案不同）────────────
	if rcEmpty == rcGreen || rcShape == rcGreen {
		t.Fatalf("成对断言失败：不给结论档 rc=%d/%d 与正控 rc=%d 同码", rcEmpty, rcShape, rcGreen)
	}

	// ── `--self-test`（四合成档 + 缺件档 · 全过 ⇒ rc=0）─────────────────────
	rcST, outST, _ := cdtRunTest(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("自检失败：应 rc=0 且「自检结论: 全过」，得 rc=%d out=%q", rcST, outST)
	}

	// ── `-h` 面：py 不过滤单破折号项 ⇒ 被当**路径** ⇒ 缺件 rc=2 ───────────────
	rcH, outH, _ := cdtRunTest(t, "-h")
	if rcH != 2 || !strings.Contains(outH, "缺件：-h 不存在") {
		t.Fatalf("`-h` 面失败：应被当路径 ⇒ rc=2「缺件：-h 不存在」，得 rc=%d out=%q", rcH, outH)
	}
	// ── `--json` 面：py 静默丢弃 `--` 前缀项 ⇒ 回落缺省路径（此处 cwd 无该件 ⇒ rc=2 缺省路径）──
	rcJ, outJ, _ := cdtRunTest(t, "--json")
	if rcJ != 2 || !strings.Contains(outJ, cdtDefaultPath) {
		t.Fatalf("`--json` 面失败：应静默回落缺省路径 ⇒ rc=2，得 rc=%d out=%q", rcJ, outJ)
	}
}

// TestCheckDevTargetsSelfRecycle —— A 类自回收牙：`--self-test` 在同前缀临时目录上 delta=0。
// 哑化「自回收」（去掉 defer os.RemoveAll）⇒ 本用例必红（带行号）。
func TestCheckDevTargetsSelfRecycle(t *testing.T) {
	prev := os.Getenv("TMPDIR")
	if prev == "" {
		prev = os.TempDir()
	}
	priv := t.TempDir()
	t.Setenv("TMPDIR", priv)

	count := func() int {
		ents, err := os.ReadDir(priv)
		if err != nil {
			return -1
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "devt-selftest-") {
				n++
			}
		}
		return n
	}
	before := count()
	rc, _, _ := cdtRunTest(t, "--self-test")
	after := count()
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 rc=%d", rc)
	}
	if before != 0 || after != 0 {
		t.Fatalf("自回收牙失败：临时目录同前缀计数 before=%d after=%d（应 0/0）", before, after)
	}
}

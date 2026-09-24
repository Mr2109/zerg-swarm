// cli_wallclock_test.go —— 命令面**墙钟上界**的成对判据（序138 · 组4 §二.4 `W-57`（`研-禁:216` §六 栗④）·
// 上级裁定 **B**：超时一律退 `11`）。
//
// 判据栏逐字：「命令面有墙钟上界旗标；超时 ⇒ **`11`** 且状态**先复原**（负控：超时后盘上有残留 ⇒ 判红）」。
//
// 本件把三半都钉住（白盒 · `package main` —— 判据要能**直接驱动**那一格，不是转引别的层的表现）：
//
//	① **旗标接消费者**（`TestWallclockParse` / `TestWallclockUsageErrorBeforeAnyAction`）：
//	   时长解析逐格（含非法 ⇒ 用法错）· 端到端 `run()` 上「非 法时长 ⇒ 先于任何动作退 `2`」
//	   （stdout **0 字节** ⇒ 那一条真没跑）。
//	② **先复原再报**（`TestWallclockDeadlineRestoresPreImage` / `…RemovesResidue` / `…StopsOwnChild`）：
//	   到点 ⇒ 退 `11` **且**盘上零残留（两态：写前在盘 ⇒ 写回前像；写前不在盘 ⇒ 删残留）·
//	   本趟亲手起的子进程被停。
//	③ **负控**（`TestWallclockResidueNegativeControl`）：把「复原」那一步换成**空档** ⇒
//	   残留**留下** ⇒ 正控当场红 —— 这一条证明①②的断言**有牙**（不是恒绿的摆设）。
//
// ★ 反假绿：每一条正控都同时断言「残留**曾经真的出现过**」（`wroteResidue`）——
//
//	否则「超时太快、根本没写」也会让「盘上零残留」看起来通过。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// blockForever 是一枚**永不关闭**的通道：让被测命令「挂住」而不写第二个字节（到点后不再污染盘）。
func blockForever() chan struct{} { return make(chan struct{}) }

// ── ① 旗标接消费者 · 时长解析 ─────────────────────────────────────────────────

func TestWallclockParse(t *testing.T) {
	ok := []struct {
		in   string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"2m", 2 * time.Minute},
		{"1h", time.Hour},
		{"500ms", 500 * time.Millisecond},
		{"1.5s", 1500 * time.Millisecond},
		{"90", 90 * time.Second}, // 裸数字 ⇒ 秒
		{" 30s ", 30 * time.Second},
	}
	for _, c := range ok {
		got, err := parseWallclock(c.in)
		if err != nil {
			t.Errorf("parseWallclock(%q) 不该报错：%v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseWallclock(%q) = %v，要 %v", c.in, got, c.want)
		}
	}
	bad := []string{"", "   ", "abc", "0s", "0", "-5s", "-1", "30x"}
	for _, in := range bad {
		if got, err := parseWallclock(in); err == nil {
			t.Errorf("parseWallclock(%q) 该报用法错（拿到 %v）", in, got)
		}
	}
}

// ── ② 先复原再报 · 正控两态 ───────────────────────────────────────────────────

func TestWallclockDeadlineRestoresPreImage(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "件.txt")
	if err := os.WriteFile(target, []byte("前像"), 0o644); err != nil {
		t.Fatalf("造前像失败：%v", err)
	}
	wallclockJournalReset()
	if err := wallclockRecordPreImage(target); err != nil {
		t.Fatalf("登记前像失败：%v", err)
	}

	var wrote atomic.Bool
	blocked := blockForever()
	var stderr bytes.Buffer
	rc := wallclockGuard(150*time.Millisecond, func() int {
		_ = os.WriteFile(target, []byte("残留（超时前写下去的那一份）"), 0o644)
		wrote.Store(true)
		<-blocked
		return exitOK
	}, &stderr)

	if rc != exitTimeout {
		t.Fatalf("到点该退 %d（超时），拿到 %d", exitTimeout, rc)
	}
	if !wrote.Load() {
		t.Fatalf("用例无效：被测命令没来得及写残留 ⇒ 「零残留」是假绿（本件不许这种绿）")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("复原后读不到件：%v", err)
	}
	if string(got) != "前像" {
		t.Errorf("**盘上有残留**：复原后是 %q，要 %q", string(got), "前像")
	}
	if !strings.Contains(stderr.String(), "先复原再报") {
		t.Errorf("到点那一格的措辞该点明「先复原再报」，实到：%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "11") {
		t.Errorf("到点那一格该报出退码 `11`，实到：%s", stderr.String())
	}
}

func TestWallclockDeadlineRemovesResidue(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "新建件.txt") // 写前**不在盘**
	wallclockJournalReset()
	if err := wallclockRecordPreImage(target); err != nil {
		t.Fatalf("登记前像失败：%v", err)
	}

	var wrote atomic.Bool
	blocked := blockForever()
	var stderr bytes.Buffer
	rc := wallclockGuard(150*time.Millisecond, func() int {
		_ = os.WriteFile(target, []byte("新建件（写前不在盘 ⇒ 到点必须删掉）"), 0o644)
		wrote.Store(true)
		<-blocked
		return exitOK
	}, &stderr)

	if rc != exitTimeout {
		t.Fatalf("到点该退 %d，拿到 %d", exitTimeout, rc)
	}
	if !wrote.Load() {
		t.Fatalf("用例无效：被测命令没来得及写残留")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("**盘上有残留**：写前不在盘的新建件到点后仍应不存在（err=%v）", err)
	}
}

// ── ② 先复原再报 · 本趟亲手起的子进程被停 ────────────────────────────────────

func TestWallclockDeadlineStopsOwnChild(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Skipf("起不了 sleep 子进程（本机形态不同）：%v", err)
	}
	runningChild = child
	defer func() { runningChild = nil }()

	blocked := blockForever()
	var stderr bytes.Buffer
	rc := wallclockGuard(150*time.Millisecond, func() int { <-blocked; return exitOK }, &stderr)
	if rc != exitTimeout {
		t.Fatalf("到点该退 %d，拿到 %d", exitTimeout, rc)
	}
	// 收尸（生产路径里本进程直接退出；用例里必须 Wait，否则看到的是僵尸）。
	_ = child.Wait()
	ps := child.ProcessState
	if ps == nil {
		t.Fatalf("到点该把本趟亲手起的子进程停掉：没收尸（ProcessState=nil）")
	}
	// ★ 照实：被**杀掉**的进程 `Exited()` 是 false（它是「被信号终止」不是「正常退出」）
	//   ⇒ 判据认的是 `Signaled()`，不是「进程不在了」这种含糊话。
	if ws, ok := ps.Sys().(syscall.WaitStatus); !ok || !ws.Signaled() {
		t.Fatalf("到点该把本趟亲手起的子进程**杀掉**（信号召回），实到 %v", ps)
	}
	if err := child.Process.Signal(syscall.Signal(0)); err == nil {
		t.Errorf("子进程还在（信号 0 打得到）⇒ 「先复原」那一半没生效")
	}
}

// ── ③ 负控：复原那一步换成空档 ⇒ 残留留下 ⇒ 上面那条正控当场红 ────────────────

func TestWallclockResidueNegativeControl(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "件.txt")
	if err := os.WriteFile(target, []byte("前像"), 0o644); err != nil {
		t.Fatalf("造前像失败：%v", err)
	}
	wallclockJournalReset()
	if err := wallclockRecordPreImage(target); err != nil {
		t.Fatalf("登记前像失败：%v", err)
	}

	// ★ 负控手法：把「复原」**整个**换成空档（`wallclockRestoreFn` 就是这个调用点）。
	//   这是**突变**——正控的断言若没牙（比如它只看 rc、不看盘），这里就该照旧「通过」；
	//   它现在必须**留下残留**，正控那条读盘断言当场红 ⇒ 证明那条断言盯着的是真东西。
	saved := wallclockRestoreFn
	wallclockRestoreFn = func() []string { return nil }
	defer func() { wallclockRestoreFn = saved }()

	var wrote atomic.Bool
	blocked := blockForever()
	var stderr bytes.Buffer
	rc := wallclockGuard(150*time.Millisecond, func() int {
		_ = os.WriteFile(target, []byte("残留"), 0o644)
		wrote.Store(true)
		<-blocked
		return exitOK
	}, &stderr)

	if rc != exitTimeout {
		t.Fatalf("负控里退码仍该是 %d（换掉的是复原、不是到点那一档），拿到 %d", exitTimeout, rc)
	}
	if !wrote.Load() {
		t.Fatalf("用例无效：没来得及写残留")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("负控里读不到件：%v", err)
	}
	if string(got) == "前像" {
		t.Fatalf("负控**没生效**：把复原换成空档之后盘上仍是前像 ⇒ 说明复原那一步没走这条缝（正控的牙无从谈起）")
	}
	if string(got) != "残留" {
		t.Fatalf("负控期望盘上留下 %q，实到 %q", "残留", string(got))
	}
	// 复原之后（这一次是空档）登记本由 guard 之外的 `wallclockRestoreAll` 收口；用例自己收干净。
	wallclockJournalReset()
}

// ── 未给旗标 ⇒ 一个字都不加 ──────────────────────────────────────────────────

func TestWallclockAbsentFlagIsNoOp(t *testing.T) {
	wallclockJournalReset()
	var stderr bytes.Buffer
	rc := wallclockGuard(time.Hour, func() int { return exitFail }, &stderr)
	if rc != exitFail {
		t.Fatalf("不给旗标那一档该原样转出命令的码（%d），拿到 %d", exitFail, rc)
	}
	if stderr.Len() != 0 {
		t.Errorf("没到点就不该多写一个字，实到：%s", stderr.String())
	}
}

// ── ① 端到端：非法时长 ⇒ 先于任何动作退 2（stdout 0 字节）────────────────────

func TestWallclockUsageErrorBeforeAnyAction(t *testing.T) {
	for _, bad := range []string{"bogus", "0s", "abc"} {
		var out, errb bytes.Buffer
		rc := run([]string{"version", "--timeout", bad}, &out, &errb)
		if rc != exitUsage {
			t.Errorf("`--timeout %s` 该退 %d（用法错），拿到 %d", bad, exitUsage, rc)
		}
		if out.Len() != 0 {
			t.Errorf("`--timeout %s` ⇒ **先于任何动作**：stdout 该 0 字节，实到 %d", bad, out.Len())
		}
		if !strings.Contains(errb.String(), wallclockFlag) {
			t.Errorf("`--timeout %s` 的 stderr 该点名那枚旗标，实到：%s", bad, errb.String())
		}
	}

	// 照实一处：`--timeout -1s`（值以 `-` 起头）走的是**解析器**那条路（`-1s` 被当未知旗标 ⇒
	// 同一个 `2`、同样**先于任何动作**）；判据只钉「码 + 零动作」，不钉它由哪一条分支报出。
	var negOut, negErr bytes.Buffer
	if rc := run([]string{"version", "--timeout", "-1s"}, &negOut, &negErr); rc != exitUsage || negOut.Len() != 0 {
		t.Errorf("`--timeout -1s` 该退 %d 且 stdout 0 字节，实到 rc=%d stdout=%d", exitUsage, rc, negOut.Len())
	}

	// 机器面：非法时长 ⇒ `--json` 包封里 kind=usage（与 rc 同源）。
	var out, errb bytes.Buffer
	rc := run([]string{"version", "--json", "version", "--timeout", "bogus"}, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("带 `--json` 的非法时长该退 %d，拿到 %d", exitUsage, rc)
	}
	if !strings.Contains(out.String(), `"usage"`) {
		t.Errorf("机器面该给出 kind=usage，实到：%s", out.String())
	}

	// 合法时长 ⇒ 不拦（命令照跑）。
	out.Reset()
	errb.Reset()
	if rc := run([]string{"version", "--timeout", "5s"}, &out, &errb); rc != exitOK {
		t.Fatalf("合法时长该放行（退 0），拿到 %d（stderr=%s）", rc, errb.String())
	}
	if !strings.Contains(out.String(), "zerg") {
		t.Errorf("合法时长下 `version` 该照出身分行，实到：%s", out.String())
	}
}

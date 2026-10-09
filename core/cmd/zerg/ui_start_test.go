// ui_start_test.go —— A6 正门（`zerg ui start` · `cmdUIStart`）**真搬**后的**两态三面**回归牙。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdUIStart`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给合成仓根，断言：
//
//	① **真搬不 exec 壳**：仓根里放一枚**毒壳**（打印 POISON / 退 42）⇒ 正门 stderr 若出现 POISON
//	   即证明实现仍在 exec 脚本（壳转账），本用例当红；
//	② **两态三面逐字同**：`scripts/build/start-zerg-ui.sh` 在盘 ⟷ 移出仓（退役态），正门
//	   rc/stdout/stderr 三面逐字同，且退役态**不得**退 8/2（旧壳转账正是靠 `os.Stat(脚本)` 退 8）；
//	③ **内化金标（三档 × 三面）**：真件已退场 ⇒ 逐档金标收成 Go 常量（见下 `uisGoldenScens`），照
//	   `atomic_write_test.go` / `pack_release_test.go` / `eval_run_all_test.go`。旧「与真脚本直跑逐字同」
//	   （恒 Skip）的对照面**已内化**为「Go 半现役行为现算再冻」。
//
// 哑化负控（"牙"）：把 `cmdUIStart` 改回「exec 脚本」或「一律 return 0 / 吞 stdout」⇒ ①②当场 FAIL
// 并给出本件行号（负控只在**仓外副本 / `-overlay`** 做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**包装入口栅格**（`uiStartGateMain` · inproc 同签名）：与 `cmdUIStart` 三面逐字同（只转发、零新增语义）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// uisSkeletonRepo —— 建最小合成仓根（带仓根判据件），返回其路径（已解符号链接 ⇒ 与脚本侧 `pwd` 同形）。
func uisSkeletonRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// uisInstallShell —— 把 body 当 `scripts/build/start-zerg-ui.sh` 落进合成仓根（0755）。
func uisInstallShell(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// uisRunCmd —— 进程内直调 `cmdUIStart`（给合成仓根）⇒ rc / stdout / stderr。
func uisRunCmd(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"ui", "start"},
		orig: append([]string{"ui", "start"}, tail...),
	}
	rc := cmdUIStart(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// uisRunGate —— 进程内直调包装入口 `uiStartGateMain` ⇒ rc / stdout / stderr。
func uisRunGate(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	rc := uiStartGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// ① 真搬不 exec 壳 ＋ 退役态两态三面逐字同（★ 退役态不得退 8/2）。
func TestUiStartNotExecutableRetiredStateByteIdentical(t *testing.T) {
	root := uisSkeletonRepo(t)
	// 放一枚**毒壳**：实现若仍 exec 脚本 ⇒ stderr 出现 POISON、rc=42 ⇒ 本用例当红（= 仍是壳转账）。
	uisInstallShell(t, root, "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n")

	wantErr := "❌ UI 二进制不可执行: " + filepath.Join(root, uiStartBinRel) +
		"（先 cargo build --release -p zerg-ui → 拷到 bin/ → codesign -s - --force）\n"

	rc1, out1, err1 := uisRunCmd(t, root)
	if strings.Contains(err1, "POISON") {
		t.Fatalf("实现仍在 exec 脚本（stderr 出现 POISON ⇒ 仍是壳转账）：%q", err1)
	}
	if rc1 != exitFail || out1 != "" || err1 != wantErr {
		t.Fatalf("态①（壳在盘）期望 rc=%d · stdout 空 · stderr=%q；实得 rc=%d out=%q err=%q",
			exitFail, wantErr, rc1, out1, err1)
	}

	// 退役态：把壳移出仓 ⇒ 三面必须逐字同。
	if err := os.Remove(filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")); err != nil {
		t.Fatal(err)
	}
	rc2, out2, err2 := uisRunCmd(t, root)
	if rc1 != rc2 || out1 != out2 || err1 != err2 {
		t.Fatalf("两态三面不逐字同：\n态① rc=%d out=%q err=%q\n态② rc=%d out=%q err=%q",
			rc1, out1, err1, rc2, out2, err2)
	}
	if rc2 == exitBlocked || rc2 == exitUsage {
		t.Fatalf("退役态**不得**退 8/2（旧壳转账靠 os.Stat(脚本) 退 8 的病）：rc=%d", rc2)
	}
}

// uisPrepAlreadyRunning —— 造「已经在运行」：真二进制（`/bin/sleep` 的**符号链接** —— 拷贝二进制在 macOS
// 上会被签名策略杀掉），argv[0] = `<root>/bin/zerg-ui` ⇒ 逐 PID 点名命中（同族 `.bak` 不会命中）。
// ★ `/bin/sleep` 不在 ⇒ fail-closed 硬错（✗ 不 Skip —— 本档金标必须真跑，缺前置即判不了）。
func uisPrepAlreadyRunning(t *testing.T, root string) func() {
	t.Helper()
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Fatalf("前置失守：/bin/sleep 不在（already-running 档无替身可造）⇒ fail-closed（✗ 不 Skip）")
	}
	bin := filepath.Join(root, uiStartBinRel)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/bin/sleep", bin); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("起 sleep 夹具失败（fail-closed · ✗ 不 Skip）：%v", err)
	}
	time.Sleep(300 * time.Millisecond) // 等它进进程表
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// uisPrepLaunchFailure —— 造「起得来又立刻退」：一枚立刻退的脚本；其 ps 名（`/usr/bin/env bash …`）≠ `$BIN`
// ⇒ 回读必落空 ⇒ 走失败支（睡 4 秒 + 日志尾部）。
func uisPrepLaunchFailure(t *testing.T, root string) func() {
	t.Helper()
	bin := filepath.Join(root, uiStartBinRel)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env bash\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return func() {}
}

// ── 内化金标：旧 sh 对拍（恒 Skip · 零保护）的逐格金标收成 Go 常量（现算现冻 · ✗ 不臆造）──────
//
// ★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「合成仓根 + 场景 prep ⇒
//   (rc, stdout, stderr)」由 `cmdUIStart` 现跑捕获（一次性现算、✗ 不落仓），冻成下表
//   `uisGoldenScens`；**本机时点/位置类量**在比对前归一化（✗ 不冻结 —— 因其本非常量）：
//     ① 夹具仓根（`t.TempDir()` 物理路径 + macOS `/var`⟷`/private/var` 符号链接）⇒ `<FX>`；
//     ② 已在运行档的 PID（真起 `/bin/sleep` 符号链接 ⇒ 逐 PID 点名命中）⇒ `<PID>`。
//
// 两态（✗ 不许 Skip）：A 态 = 合成仓根无壳（= 真仓现态）；B 态 = 把退役壳路径
//   `scripts/build/start-zerg-ui.sh` 的 stub 放回 ⇒ 两态对同一套金标逐字同命中（Go 真身 ✗ exec 脚本）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 ui_start.go 的一条出口文案/退码臂哑化 ⇒ 本用例
//   **必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。

// uisGoHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的壳 —— 一退役即失锚）。
const uisGoHalfRel = "core/cmd/zerg/ui_start.go"

// uisShellRel —— 已退场壳真身的仓根相对路径；「两态」就钉在它「在 / 不在盘」。
const uisShellRel = "scripts/build/start-zerg-ui.sh"

// uisSentinel —— 哨兵：上溯找 **Go 半**（本件真源 · 编译进测试二进制）。找不到 ⇒ fail-closed
// （✗ 不留 Skip）：旧版对上溯不到 sh 就 `t.Skipf`（恒 Skip · 零保护）—— 本改把它换成硬前置。
func uisSentinel(t *testing.T) string {
	t.Helper()
	want := filepath.FromSlash(uisGoHalfRel)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败：%v", err)
	}
	for i := 0; i < 8; i++ {
		if st, e := os.Stat(filepath.Join(dir, want)); e == nil && st.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）", uisGoHalfRel)
	return ""
}

// uisIsFile —— 判在盘且是普通件（哨兵腿用）。
func uisIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// uisRePid —— 归一本机 PID（已在运行档）；只命中文案里的 `PID <digits>`。
var uisRePid = regexp.MustCompile(`PID [0-9]+`)

// uisNormGolden —— 归一本机时点/位置类量（夹具仓根 / PID）⇒ 金标跨机可复现。
func uisNormGolden(s, fx string) string {
	if fx != "" {
		s = strings.ReplaceAll(s, fx, "<FX>")
	}
	return uisRePid.ReplaceAllString(s, "PID <PID>")
}

type uisGoldenCase struct {
	name    string
	prep    func(t *testing.T, root string) func()
	wantRC  int
	wantOut string // 现算自证（由 cmdUIStart 现跑捕获）· ✗ 手写猜
	wantErr string
}

// uisGoldenScens —— 金标真源（3 档 · 与旧对拍逐档同场景）。want* 由 `cmdUIStart` 现跑捕获
// （见段头金标计算式）。
func uisGoldenScens() []uisGoldenCase {
	return []uisGoldenCase{
		{name: "not-executable", prep: func(t *testing.T, root string) func() { return func() {} },
			wantRC: exitFail, wantOut: "",
			wantErr: "❌ UI 二进制不可执行: <FX>/bin/zerg-ui（先 cargo build --release -p zerg-ui → 拷到 bin/ → codesign -s - --force）\n"},
		{name: "already-running", prep: uisPrepAlreadyRunning,
			wantRC: exitOK, wantOut: "ℹ️ UI 已在运行（PID <PID>）——不重复启动\n", wantErr: ""},
		{name: "launch-failure", prep: uisPrepLaunchFailure,
			wantRC: exitFail, wantOut: "", wantErr: "❌ UI 启动失败——日志尾部:\n"},
	}
}

// uisRunGolden —— 逐档现建合成仓根、现跑 Go 半（cmdUIStart）、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往合成仓根落「旧态」壳 stub（两态用）。
func uisRunGolden(t *testing.T, tag string, plant func(root string)) int {
	t.Helper()
	mism := 0
	for _, c := range uisGoldenScens() {
		root := uisSkeletonRepo(t)
		t.Setenv("ZERG_UI_LOG", filepath.Join(root, "ui.log"))
		if plant != nil {
			plant(root)
		}
		cleanup := c.prep(t, root)
		rc, out, errb := uisRunCmd(t, root)
		cleanup()

		gotOut := uisNormGolden(out, root)
		gotErr := uisNormGolden(errb, root)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// plantUisShell —— B 态「旧态」：把退役壳路径 `scripts/build/start-zerg-ui.sh` 的合成 stub 落回合成仓根。
func plantUisShell(root string) {
	p := filepath.Join(root, filepath.FromSlash(uisShellRel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("#!/bin/sh\n# [退役占位] scripts/build/start-zerg-ui.sh —— 真件已退场（Go 半承载）；stub 仅保夹具形状。\n"), 0o755)
}

// TestUiStartGoldenInternalized —— A 态（真仓 · 退役壳不在盘）金标自证
// （✗ 不 Skip · 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestUiStartGoldenInternalized(t *testing.T) {
	repo := uisSentinel(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	if uisIsFile(filepath.Join(repo, filepath.FromSlash(uisShellRel))) {
		t.Fatalf("A 态前提破：退役壳竟在盘 %s（本件前提 = 内化后无壳）", uisShellRel)
	}
	if n := uisRunGolden(t, "A夹具无壳", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestUiStartGoldenTwoStates —— 两态逐字同：A 态 = 合成仓根无壳（= 真仓现态）；B 态 = 把退役壳路径
// `scripts/build/start-zerg-ui.sh` 的 stub 放回 ⇒ 两态对同一套金标逐字同命中（✗ 不 Skip）。
func TestUiStartGoldenTwoStates(t *testing.T) {
	if n := uisRunGolden(t, "A夹具无壳", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	if n := uisRunGolden(t, "B旧态有壳", plantUisShell); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（壳在盘不得改判据结论）", n)
	}
}

// ④ 包装入口栅格：`uiStartGateMain` 与 `cmdUIStart` 三面逐字同（只转发 · 零新增语义）。
func TestUiStartGateMainForwardsCmdUIStart(t *testing.T) {
	root := uisSkeletonRepo(t)
	rcC, outC, errC := uisRunCmd(t, root)
	rcW, outW, errW := uisRunGate(t, root, "--not-a-flag", "x")
	if rcC != rcW || outC != outW || errC != errW {
		t.Fatalf("包装入口与 cmdUIStart 三面不同：\ncmd  rc=%d out=%q err=%q\ngate rc=%d out=%q err=%q",
			rcC, outC, errC, rcW, outW, errW)
	}
}

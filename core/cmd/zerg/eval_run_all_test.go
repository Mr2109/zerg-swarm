// eval_run_all_test.go —— 件 `core/cmd/zerg/eval_run_all.go`（`cmdEvalRunAll` · 原
// `scripts/build/run_all.sh` 真搬 · DEV-2034）的回归牙：**金标内化**（旧 sh 对拍退场 ⇒ 逐格金标
// 收成 Go 常量 · 两态 · ✗ 不 Skip）＋ 缺省版本标签 / 空串位置参数语义。
//
// 为什么这么测（M2 口径 · 与 `atomic_write_test.go` 的 TestAwTwoStatesGoldenInternalized /
// `check_verify_cmd_real_test.go` 的 TestCheckVerifyCmdRealGoldenInternalized /
// `check_manifest_freshness_test.go` 的 TestCheckManifestFreshnessGoldenInternalized / 更早
// `archive_mount_test.go` 的 TestArchiveMountRunsWithoutScript 同族）：本件原「sh 真身 ⟷ Go 真搬」
// 逐字对拍（旧名 `TestEvalRunAllTwoStateThreeFaceParity`）的对拍源 `scripts/build/run_all.sh`
// 已退场（DEV-2034）⇒ 对拍源消失、断言恒 Skip（`f.realScript` 永假 · 假绿 · 零保护）。按先例把
// 金标真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「输入（仓外合成夹具 + 位置参数 <模型> <版本标签>）⇒ (rc, stdout, stderr, 四件落档)」
//	  由 `cmdEvalRunAll` 在**仓外夹具**上现跑捕获（一次性现算、只读夹具、✗ 不落仓），冻成下表
//	  `erallGolden*` 常量；**本机时点类量**在比对前归一化 / 钉死（✗ 不冻结机器时点量）：
//	    ① 夹具根 `t.TempDir()`（随机路径 + macOS /var↔/private/var）⇒ `<TMP>`（`erallNormGolden`）；
//	    ② `summary.json` 的 `date` 走注入缝 `evalRunAllClock` ⇒ 对拍时把钟**钉死**（`erallPinClock`
//	      到 2026-10-09 14:00）⇒ 其确定值可冻（真不定的只有①，已在比对前归一）。
//
// 两态（✗ 不许 Skip）：A 态 = 夹具**无** sh（= 真仓现态）；B 态 = 合成「旧态」夹具（把退役 sh 路径
// `scripts/build/run_all.sh` 的 stub 放回）⇒ 两态**对同一套金标逐字同命中**，证 sh 在/不在盘都不改
// Go 半读数（Go 真身 ✗ 不 exec 脚本）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `eval_run_all.go` 的层文案 / 落档 / 汇总任一臂哑化
// ⇒ 本用例**必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
//
// 两态夹具一律落**仓外**（t.TempDir() ⇒ $TMPDIR · ✗ 不落仓内：门76 会把盘上任何新件算进分母）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// evalRunAllGoHalfRel —— 本件 Go 真身（哨兵锚：判「能力可用」而非「退役 sh 在盘」）。
const evalRunAllGoHalfRel = "core/cmd/zerg/eval_run_all.go"

// erallStubScript —— 退役 sh 的合成 stub（B 态「旧态」占位 · 只证 sh 在/不在盘不改 Go 半读数 ·
// ✗ 不作对拍对象）。
const erallStubScript = "#!/usr/bin/env bash\n" +
	"# [退役占位] scripts/build/run_all.sh —— 真件已退场（Go 半承载）；本 stub 仅保夹具形状。\n" +
	"set -e\nexit 0\n"

// erallSentinel —— 哨兵：上溯找 **Go 半**（本件真源）。✗ 不改锚退役 sh（退场即失锚 ⇒ 恒 Skip）。
// 找不到 ⇒ **fail-closed**（✗ 不留 Skip）：Go 半是编译进本测试二进制的真源，上溯见不到它 = 真仓
// 结构被破 ⇒ 硬错。
func erallSentinel(t *testing.T) string {
	t.Helper()
	want := filepath.FromSlash(evalRunAllGoHalfRel)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败: %v", err)
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
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）", evalRunAllGoHalfRel)
	return ""
}

// erallFixture —— 仓外夹具（仓根 / 存档落点 / 两个位置参数）。
type erallFixture struct {
	tmp   string // t.TempDir()（= 夹具父目录；`<tmp>/zerg-evals` 是 EVALS 缺省落点）
	root  string // 夹具仓根 = <tmp>/fx
	evals string // = <tmp>/zerg-evals（原件 `dirname/../../../zerg-evals` 的推导结果）
	model string
	ver   string
}

func (f *erallFixture) runDir() string {
	return f.evals + "/results/agent/runs/" + f.model + "-" + f.ver
}

func erallWrite(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// erallStubPy —— 层2/3 的替身载荷：**确定输出**（无计时、无随机），回显它收到的 argv。
// stdout 显式 flush 后再写 stderr ⇒ 合并流里的行序在两态都稳定（同一支解释器跑同一支载荷）。
func erallStubPy(tag string) string {
	return "#!/usr/bin/env python3\n" +
		"import sys\n" +
		"sys.stdout.write('stub " + tag + " 载荷 argv: ' + ' '.join(sys.argv[1:]) + '\\n')\n" +
		"sys.stdout.write('stub " + tag + " 载荷 第二行\\n')\n" +
		"sys.stdout.flush()\n" +
		"sys.stderr.write('stub " + tag + " 载荷 stderr 行\\n')\n" +
		"sys.stderr.flush()\n" +
		"sys.exit(0)\n"
}

// erallBuildFixture 造夹具：stub 载荷 ＋ 最小 Go module ＋ 同级 zerg-evals。
//
// ★ 不再读退役 sh（Go 半 ✗ 不 exec 它）—— B 态要「旧态」时另调 `erallAddStubScript`。
func erallBuildFixture(t *testing.T, model, ver string) *erallFixture {
	t.Helper()
	tmp := t.TempDir()
	f := &erallFixture{tmp: tmp, root: filepath.Join(tmp, "fx"), evals: filepath.Join(tmp, "zerg-evals"),
		model: model, ver: ver}

	// ② 层2/3 的替身载荷（真载荷需活模型 ⇒ 见件头「不可跑层」）。
	erallWrite(t, filepath.Join(f.root, evalRunAllLayer2Target), erallStubPy("层2"), 0o644)
	erallWrite(t, filepath.Join(f.root, evalRunAllLayer3Target), erallStubPy("层3"), 0o644)

	// ③ 最小 Go module：`core/internal/agent` 无测试件 ⇒ `go test` 输出**确定**（无计时）。
	erallWrite(t, filepath.Join(f.root, "core", "go.mod"), "module example.com/erallfx\n\ngo 1.21\n", 0o644)
	erallWrite(t, filepath.Join(f.root, "core", "internal", "agent", "agent.go"), "package agent\n", 0o644)
	// 仓根判据件（`isRepoRoot` 要它）—— 夹具根因此能被 `repoRoot()` 认出。
	erallWrite(t, filepath.Join(f.root, "core", "internal", "version", "version.go"), "package version\n", 0o644)

	// ④ EVALS 缺省落点必须**先在盘**（原件那档是 `cd … && pwd` ⇒ 不在即 `set -e` 中断）。
	if err := os.MkdirAll(f.evals, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// erallAddStubScript —— B 态「旧态」：把退役 sh 路径 `scripts/build/run_all.sh` 的合成 stub 放回。
func erallAddStubScript(t *testing.T, f *erallFixture) {
	t.Helper()
	erallWrite(t, filepath.Join(f.root, filepath.FromSlash(evalRunAllSrcRel)), erallStubScript, 0o755)
}

// runGo —— **走 Go 半**：仓根走 `ZERG_REPO`（= 夹具根），cwd 也切到夹具根（层2/3 相对路径面）。
func (f *erallFixture) runGo(t *testing.T) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", f.root)
	t.Chdir(f.root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"eval", "run-all"},
		orig: []string{"eval", "run-all", f.model, f.ver},
	}
	rc := cmdEvalRunAll(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// runGoNoArgs —— 只给模型（版本标签落缺省 ⇒ 走 `evalRunAllClock()`）。
func (f *erallFixture) runGoNoArgs(t *testing.T) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", f.root)
	t.Chdir(f.root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"eval", "run-all"},
		orig: []string{"eval", "run-all", f.model},
	}
	rc := cmdEvalRunAll(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

func (f *erallFixture) readArtifacts(t *testing.T) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, n := range []string{evalRunAllLayer1File, evalRunAllLayer2File, evalRunAllLayer3File, evalRunAllSummaryRel} {
		b, err := os.ReadFile(f.runDir() + "/" + n)
		if err != nil {
			t.Fatalf("落档缺：%s（%v）—— 逐层落档是三面之外必须比的一面", n, err)
		}
		m[n] = string(b)
	}
	return m
}

// erallPinClock —— 把 `evalRunAllClock` 钉到给定时刻（测试期有效 · 用完自动还原）。
func erallPinClock(t *testing.T, at time.Time) {
	t.Helper()
	orig := evalRunAllClock
	t.Cleanup(func() { evalRunAllClock = orig })
	evalRunAllClock = func() time.Time { return at }
}

// ── 内化金标：旧 sh 对拍的逐格金标收成 Go 常量 · 现算自证 ──────────────────────────

// erallGolden* —— 金标（rc / stdout / stderr / 四件落档）。逐格由 Go 半在仓外夹具上现跑捕获
// （见件头金标计算式 · ✗ 手写猜）。`<TMP>` = 夹具根占位（比对前由 `erallNormGolden` 归一）。
const (
	erallGoldenRC  = 0
	erallGoldenErr = ""
)

const erallGoldenOut = "🐛 虫族 Agent 测试——模型: example-35b | 版本: v2.5\n" +
	"📁 存档: <TMP>/zerg-evals/results/agent/runs/example-35b-v2.5\n" +
	"\n" +
	"=== 层1: 框架单测 ===\n" +
	"?   \texample.com/erallfx/internal/agent\t[no test files]\n" +
	"✅ 层1 完成\n" +
	"\n" +
	"=== 层2: 基础任务 ===\n" +
	"stub 层2 载荷 argv: --model example-35b --workdir /tmp/zerg-agent-example-35b-basic\n" +
	"stub 层2 载荷 第二行\n" +
	"stub 层2 载荷 stderr 行\n" +
	"✅ 层2 完成\n" +
	"\n" +
	"=== 层3: 复合任务 ===\n" +
	"stub 层3 载荷 argv: --model example-35b --workdir /tmp/zerg-agent-example-35b-complex\n" +
	"stub 层3 载荷 第二行\n" +
	"stub 层3 载荷 stderr 行\n" +
	"✅ 层3 完成\n" +
	"\n" +
	"🎉 测试完成——存档: <TMP>/zerg-evals/results/agent/runs/example-35b-v2.5\n" +
	"查看: cat '<TMP>/zerg-evals/results/agent/runs/example-35b-v2.5/layer3-complex.txt'\n"

const erallGoldenLayer1 = "?   \texample.com/erallfx/internal/agent\t[no test files]\n"

const erallGoldenLayer2 = "stub 层2 载荷 argv: --model example-35b --workdir /tmp/zerg-agent-example-35b-basic\n" +
	"stub 层2 载荷 第二行\n" +
	"stub 层2 载荷 stderr 行\n"

const erallGoldenLayer3 = "stub 层3 载荷 argv: --model example-35b --workdir /tmp/zerg-agent-example-35b-complex\n" +
	"stub 层3 载荷 第二行\n" +
	"stub 层3 载荷 stderr 行\n"

const erallGoldenSummary = "{\n" +
	"  \"model\": \"example-35b\",\n" +
	"  \"version\": \"v2.5\",\n" +
	"  \"date\": \"2026-10-09 14:00\",\n" +
	"  \"layers_run\": [\"layer1-unit\", \"layer2-basic\", \"layer3-complex\"],\n" +
	"  \"note\": \"完整结果见各 layer*.txt\"\n" +
	"}\n"

// erallNormGolden —— 归一本机时点类量（夹具根 t.TempDir()）⇒ 金标跨机可复现。
func erallNormGolden(s, fx string) string {
	if fx != "" {
		s = strings.ReplaceAll(s, fx, "<TMP>")
	}
	return s
}

// erallRunGolden —— 逐格现跑 Go 半（夹具在 $TMPDIR · 仓外），归一后与冻值逐字比；返回失配格数。
// withScript=true ⇒ 夹具再加退役 sh 的合成 stub（B 态「旧态」）；两态对同一套金标。
func erallRunGolden(t *testing.T, tag string, withScript bool) int {
	t.Helper()
	f := erallBuildFixture(t, "example-35b", "v2.5")
	if withScript {
		erallAddStubScript(t, f)
	}
	erallPinClock(t, time.Date(2026, 10, 9, 14, 0, 0, 0, time.Local))
	rc, out, errb := f.runGo(t)
	arts := f.readArtifacts(t)

	mism := 0
	if rc != erallGoldenRC {
		mism++
		t.Errorf("[%s] rc=%d（冻值 %d）", tag, rc, erallGoldenRC)
	}
	if got := erallNormGolden(out, f.tmp); got != erallGoldenOut {
		mism++
		t.Errorf("[%s] stdout 漂移\n实得=%q\n冻值=%q", tag, got, erallGoldenOut)
	}
	if got := erallNormGolden(errb, f.tmp); got != erallGoldenErr {
		mism++
		t.Errorf("[%s] stderr 漂移\n实得=%q\n冻值=%q", tag, got, erallGoldenErr)
	}
	for _, c := range []struct{ name, want string }{
		{evalRunAllLayer1File, erallGoldenLayer1},
		{evalRunAllLayer2File, erallGoldenLayer2},
		{evalRunAllLayer3File, erallGoldenLayer3},
		{evalRunAllSummaryRel, erallGoldenSummary},
	} {
		if got := erallNormGolden(arts[c.name], f.tmp); got != c.want {
			mism++
			t.Errorf("[%s] 落档 %s 漂移\n实得=%q\n冻值=%q", tag, c.name, got, c.want)
		}
	}
	return mism
}

// TestEvalRunAllGoldenInternalized —— A 态（真仓 · 退役 sh 不在盘）金标自证
// （✗ 不 Skip · 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestEvalRunAllGoldenInternalized(t *testing.T) {
	root := erallSentinel(t)
	sh := filepath.Join(root, filepath.FromSlash(evalRunAllSrcRel))
	if _, err := os.Stat(sh); err == nil {
		t.Fatalf("A 态前提破：退役 sh 竟在盘 %s（本件前提 = 内化后无 sh）", sh)
	}
	if n := erallRunGolden(t, "A真仓无sh", false); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestEvalRunAllGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 sh（= 真仓现态）；B 态 = 合成「旧态」
// 夹具（`scripts/build/run_all.sh` 在位）⇒ 两态对同一套金标逐字同命中（✗ 不 Skip）。
func TestEvalRunAllGoldenTwoStates(t *testing.T) {
	if n := erallRunGolden(t, "A夹具无sh", false); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	if n := erallRunGolden(t, "B旧态有sh", true); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（sh 在盘不得改判据结论）", n)
	}
}

// ── 缺省版本标签 = `date +%Y%m%d-%H%M`（走注入缝 · 生产路 = time.Now）────────────────

func TestEvalRunAllDefaultVersionFromClock(t *testing.T) {
	f := erallBuildFixture(t, "example-35b", "ignored")
	at := time.Date(2026, 10, 8, 22, 41, 7, 0, time.Local)
	erallPinClock(t, at)

	rc, out, errb := f.runGoNoArgs(t)
	if rc != exitOK {
		t.Fatalf("缺省版本档应 rc=0（得 %d）· stderr=%q", rc, errb)
	}
	wantDir := f.evals + "/results/agent/runs/" + f.model + "-20261008-2241"
	if !strings.Contains(out, "📁 存档: "+wantDir) {
		t.Errorf("缺省版本标签不合 `date +%%Y%%m%%d-%%H%%M`：想要 %q · stdout=%q", wantDir, out)
	}
	if _, err := os.Stat(wantDir + "/" + evalRunAllSummaryRel); err != nil {
		t.Errorf("缺省版本档没落档：%v", err)
	}
}

// ── 空串位置参数 = `:-` 语义（空按未给 ⇒ 落缺省）──────────────────────────────

func TestEvalRunAllEmptyModelFallsBackToDefault(t *testing.T) {
	f := erallBuildFixture(t, "", "v2.5")
	erallPinClock(t, time.Date(2026, 10, 8, 22, 41, 0, 0, time.Local))

	rc, out, errb := f.runGo(t)
	if rc != exitOK {
		t.Fatalf("空模型档应 rc=0（得 %d）· stderr=%q", rc, errb)
	}
	wantDir := f.evals + "/results/agent/runs/" + evalRunAllDefaultModel + "-v2.5"
	if !strings.Contains(out, "📁 存档: "+wantDir) {
		t.Errorf("`${1:-…}` 语义：空串应按未给 ⇒ 落缺省模型 · 想要 %q · stdout=%q", wantDir, out)
	}
}

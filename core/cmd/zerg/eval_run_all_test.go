// eval_run_all_test.go —— 件 `scripts/build/run_all.sh` **真搬**（`cmdEvalRunAll`）的
// **两态 · 三面 · 逐层对拍** ＋ **哑化负控**（成对 · 有牙）。
//
// 真源 = **Go 半**（`core/cmd/zerg/eval_run_all.go` · `cmdEvalRunAll`）；壳件 `scripts/build/run_all.sh`
// （原 49 行）**已退场**（DEV-2034）—— 它在盘时曾是**对拍 oracle**，其三层里有**重活**（层1 `go test` ·
// 层2/3 `python3 <载荷>`）⇒ 对拍必须**逐字节比「脚本直跑」**，不能只比自家两块 buffer。做法（照 M2 对拍台）：
//
//	· **仓外夹具**（`t.TempDir()` ⇒ 落 `TMPDIR` · ✗ 不进仓树 —— 门76 把盘上任何新件算进分母）：
//	  夹具里放 ① 真件的**逐字副本** `scripts/build/run_all.sh`（它就是**对拍 oracle**）
//	  ② 层2/3 的 **stub 载荷**（可控输出 + 回显 argv ⇒ 顺带钉死「已归档载荷的调用 argv」）
//	  ③ 一棵最小 Go module（`core/internal/agent` 无测试件 ⇒ `go test` 输出确定：
//	  `?   <mod>/internal/agent\t[no test files]` · rc=0 · 无计时 ⇒ **可逐字节比**）。
//
//	· **两态**：① 脚本在盘 —— 脚本直跑 ⟷ Go 半，逐层逐字；② 脚本**移出仓** —— Go 半必须与 ① 逐字同
//	  （★★ 这一档是本单的**核心判据**：✗ 不得因脚本不在而退 2/8 —— 那说明还在 exec 脚本）。
//
//	· **三面 + 落档**：rc · stdout · stderr，外加四件落档（`layer1-unit.txt` / `layer2-basic.txt` /
//	  `layer3-complex.txt` / `summary.json`）逐字节比 —— 只比 stdout 会漏「回显对、落档不同」的档。
//
//	· **★ 不可跑层（已写明）**：层2/3 的**真载荷**（`scripts/evals/_archive/v25_*.py`）内部
//	  `go run ./cmd/zerg-agent -model <模型>` 打**生产链路 · 需活模型** ⇒ **本件不跑真载荷**。
//	  本件对拍的是**编排面**（argv / 工作目录 / `2>&1|tee` 合并 / 落档 / 退码），用 stub 顶上载荷位；
//	  **✗ 不拿 stub 的绿冒充「真载荷已验」**（真载荷读数需活模型 ⇒ 留给人手动跑）。
//
//	· **时刻面**：原件两处 `date`（缺省版本号 / `summary.json` 的 `date`）走注入缝 `evalRunAllClock` ——
//	  对拍时把 Go 的钟**钉到「脚本那一跑」读到的同一分钟**（照 M2 口径：自己会写的时刻用旋钮钉，✗ 不靠掩码）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// evalRunAllGoHalfRel —— 本件 Go 真身（退役态判据：判「能力可用」而非「件在盘」）。
const evalRunAllGoHalfRel = "core/cmd/zerg/eval_run_all.go"

// erallSyntheticScript —— 退役态占位 stub（真件离盘后保夹具形状 · ✗ 不作对拍 oracle）。
const erallSyntheticScript = "#!/usr/bin/env bash\n" +
	"# [退役占位] scripts/build/run_all.sh —— 真件已退场（Go 半承载）；本 stub 仅保夹具形状。\n" +
	"set -e\nexit 0\n"

// erallGoHalfInPlace —— 真件 rel 的 Go 半边是否在盘（退役态判据：判「能力可用」）。
func erallGoHalfInPlace(root string) bool {
	fi, err := os.Stat(filepath.Join(root, evalRunAllGoHalfRel))
	return err == nil && !fi.IsDir()
}

// erallRepoRoot 现读**真仓根**（给「夹具取真脚本逐字副本」用）。
// ★ ✗ 不用 `ZERG_REPO` —— 那条旋钮在本轮会被设成**夹具根**。
func erallRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if r := findRoot(wd); r != "" {
		return r
	}
	t.Fatalf("现读不到真仓根（cwd=%s）—— 对拍要拿真脚本的逐字副本", wd)
	return ""
}

// erallFixture —— 仓外夹具（仓根 / 存档落点 / 两个位置参数）。
type erallFixture struct {
	tmp   string // t.TempDir()（= 夹具父目录；`<tmp>/zerg-evals` 是 EVALS 缺省落点）
	root  string // 夹具仓根 = <tmp>/fx
	evals string // = <tmp>/zerg-evals（原件 `dirname/../../../zerg-evals` 的推导结果）
	model string
	ver   string
	// realScript：真件 `scripts/build/run_all.sh` 是否在盘（true ⇒ 夹具里的副本是**逐字 oracle**；
	// false ⇒ 退役态，夹具落**合成 stub** 占位，依赖真件的对拍格自行体面跳过）。
	realScript bool
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

// erallBuildFixture 造夹具：真脚本逐字副本（oracle）＋ stub 载荷 ＋ 最小 Go module ＋ 同级 zerg-evals。
func erallBuildFixture(t *testing.T, model, ver string) *erallFixture {
	t.Helper()
	realRoot := erallRepoRoot(t)
	realScript, readErr := os.ReadFile(filepath.Join(realRoot, evalRunAllSrcRel))
	realPresent := readErr == nil
	if !realPresent {
		// ★ 三段式回退腿（先例 `atomic_write_test.go` TestAwTwoStateParity ·
		//   `pack_release_test.go` prHalvesGuard）：sh 原件缺席而其 **Go 半**在盘
		//   ⇒ 退役态 —— 对拍 oracle 不可得 ⇒ 落夹具自带的**合成 stub** 占位（只保夹具形状，
		//   ✗ 不作对拍对象），依赖真件的对拍格按 `f.realScript` 体面跳过；
		//   两半皆缺（真消失）⇒ fail-closed（✗ 缺件 ≠ 绿）。
		if !erallGoHalfInPlace(realRoot) {
			t.Fatalf("读真脚本失败（%s）：%v（且其 Go 半 %s 亦不在盘 ⇒ fail-closed）",
				evalRunAllSrcRel, readErr, evalRunAllGoHalfRel)
		}
		realScript = []byte(erallSyntheticScript)
	}

	tmp := t.TempDir()
	f := &erallFixture{tmp: tmp, root: filepath.Join(tmp, "fx"), evals: filepath.Join(tmp, "zerg-evals"),
		model: model, ver: ver, realScript: realPresent}

	// ① 真脚本逐字副本 = oracle（★ 它**一定是现役真身的逐字拷贝**，✗ 不是手写的近似版）；
	//    退役态 ⇒ 合成 stub 占位（对拍格已在调用侧跳过）。
	erallWrite(t, filepath.Join(f.root, evalRunAllSrcRel), string(realScript), 0o755)

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

// runScript —— **脚本直跑**（oracle 侧）：cwd = 夹具根（层2/3 的相对路径只在 cwd=仓根时解析得到）。
func (f *erallFixture) runScript(t *testing.T) (int, string, string) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(f.root, evalRunAllSrcRel), f.model, f.ver)
	cmd.Dir = f.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return erallRc(err), out.String(), errb.String()
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

func erallRc(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
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

func erallAssertSame(t *testing.T, what, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("%s **不逐字**：\n  脚本侧 = %q\n  Go 侧  = %q", what, want, got)
	}
}

// erallSummaryDate 从 summary.json 正文里取 `date` 字段值（`+%Y-%m-%d %H:%M`）。
func erallSummaryDate(t *testing.T, summary string) time.Time {
	t.Helper()
	const k = "\"date\": \""
	i := strings.Index(summary, k)
	if i < 0 {
		t.Fatalf("summary.json 里没有 date 字段：%q", summary)
	}
	rest := summary[i+len(k):]
	j := strings.Index(rest, "\"")
	if j < 0 {
		t.Fatalf("summary.json 的 date 字段没闭合：%q", summary)
	}
	tm, err := time.ParseInLocation("2006-01-02 15:04", rest[:j], time.Local)
	if err != nil {
		t.Fatalf("date 字段值不合 `+%%Y-%%m-%%d %%H:%%M`：%q（%v）", rest[:j], err)
	}
	return tm
}

// erallPinClock —— 把 `evalRunAllClock` 钉到给定时刻（测试期有效 · 用完自动还原）。
func erallPinClock(t *testing.T, at time.Time) {
	t.Helper()
	orig := evalRunAllClock
	t.Cleanup(func() { evalRunAllClock = orig })
	evalRunAllClock = func() time.Time { return at }
}

// ── 主判据：两态 · 三面 · 逐层 ────────────────────────────────────────────────

func TestEvalRunAllTwoStateThreeFaceParity(t *testing.T) {
	f := erallBuildFixture(t, "example-35b", "v2.5")
	if !f.realScript {
		// ★ 退役态：真件缺席 ⇒ 对拍 oracle 不可得 ⇒ 体面跳过（✗ 不判红 · ✗ 不把断言放宽成永真）；
		//   Go 半自身的断言由 TestEvalRunAllDefaultVersionFromClock /
		//   TestEvalRunAllEmptyModelFallsBackToDefault **照常覆盖**；Go 半亦缺席由夹具 fail-closed。
		t.Skipf("退役态：sh 原件 %s 已退场（其 Go 半 %s 在盘）⇒ 两态对拍格无对象，容忍跳过",
			evalRunAllSrcRel, evalRunAllGoHalfRel)
	}

	// 态①：脚本在盘 —— 先拿脚本直跑的读数（oracle）。
	rcScript, outScript, errScript := f.runScript(t)
	artsScript := f.readArtifacts(t)
	if rcScript != exitOK {
		t.Fatalf("夹具里脚本直跑应 rc=0（得 %d）· stdout=%q stderr=%q", rcScript, outScript, errScript)
	}
	// 逐层读数（三面：rc/stdout/stderr · 以及落档）——先证明夹具真跑了三层。
	for _, want := range []string{"=== 层1: 框架单测 ===", "=== 层2: 基础任务 ===", "=== 层3: 复合任务 ===",
		"✅ 层1 完成", "✅ 层2 完成", "✅ 层3 完成", "🎉 测试完成——存档: "} {
		if !strings.Contains(outScript, want) {
			t.Fatalf("脚本直跑 stdout 缺层序文案 %q（夹具没跑全三层？）· stdout=%q", want, outScript)
		}
	}
	// 层1 真跑了 `go test`（无测试件包 ⇒ 确定行）。
	if !strings.Contains(artsScript[evalRunAllLayer1File], "[no test files]") {
		t.Errorf("层1 落档没含 `go test` 的确定输出：%q", artsScript[evalRunAllLayer1File])
	}
	// 层2/3 的载荷调用 argv（已归档载荷的调用面在这里被钉死：模型 + workdir 两个模板值）。
	if !strings.Contains(artsScript[evalRunAllLayer2File],
		"--model "+f.model+" --workdir /tmp/zerg-agent-"+f.model+"-basic") {
		t.Errorf("层2 载荷 argv 不符原件：%q", artsScript[evalRunAllLayer2File])
	}
	if !strings.Contains(artsScript[evalRunAllLayer3File],
		"--model "+f.model+" --workdir /tmp/zerg-agent-"+f.model+"-complex") {
		t.Errorf("层3 载荷 argv 不符原件：%q", artsScript[evalRunAllLayer3File])
	}

	// 把 Go 的钟钉到**脚本那一跑**读到的同一分钟（两处 `date` 的唯一非确定面）。
	erallPinClock(t, erallSummaryDate(t, artsScript[evalRunAllSummaryRel]))

	// 态①：Go 半 ⟷ 脚本直跑。
	rcGo, outGo, errGo := f.runGo(t)
	artsGo := f.readArtifacts(t)
	if rcGo != rcScript {
		t.Errorf("态①（脚本在盘）rc 不逐字：脚本=%d Go=%d", rcScript, rcGo)
	}
	erallAssertSame(t, "态① stdout", outScript, outGo)
	erallAssertSame(t, "态① stderr", errScript, errGo)
	for _, n := range []string{evalRunAllLayer1File, evalRunAllLayer2File, evalRunAllLayer3File, evalRunAllSummaryRel} {
		erallAssertSame(t, "态① 落档 "+n, artsScript[n], artsGo[n])
	}

	// 态②：脚本**移出仓** ⇒ Go 半必须与态① 逐字同（★ 这是「✗ 不 exec 脚本」的机检面）。
	if err := os.Remove(filepath.Join(f.root, evalRunAllSrcRel)); err != nil {
		t.Fatalf("把脚本移出仓失败：%v", err)
	}
	rcGo2, outGo2, errGo2 := f.runGo(t)
	artsGo2 := f.readArtifacts(t)
	if rcGo2 == exitUsage {
		t.Fatalf("态②（脚本移出仓）rc=2 —— 说明还在依赖脚本（本件必须 ✗ 不 exec 脚本）")
	}
	if rcGo2 != rcGo {
		t.Errorf("态② rc 与态① 不逐字：态①=%d 态②=%d", rcGo, rcGo2)
	}
	erallAssertSame(t, "态② stdout", outGo, outGo2)
	erallAssertSame(t, "态② stderr", errGo, errGo2)
	for _, n := range []string{evalRunAllLayer1File, evalRunAllLayer2File, evalRunAllLayer3File, evalRunAllSummaryRel} {
		erallAssertSame(t, "态② 落档 "+n, artsGo[n], artsGo2[n])
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

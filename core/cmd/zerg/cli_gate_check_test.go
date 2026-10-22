// cli_gate_check_test.go —— M1 批B：`zerg gate check` 判据转发壳的**进程内真用例**。
//
// 为什么这层（缺口 `GAP-20261001-10`）：清清单 M1-01/M1-03 引的 `go test -run TestGateCheck`
// **全仓 0 命中** ⇒ 那几条验收命令一直**空跑**（假绿）。本件把 TestGateCheck 补在盘上：
//
//	① 未知判据名 ⇒ 退 2（保守拒 · 给可用名）；
//	② 子件失败**逐字透传** ⇒ 壳退子件的码（此例 7），且台账落一行 `raw_rc=7`（六字段齐）；
//	③ `ZERG_GATE_FORWARD=off` ⇒ 退 8（`detail=judge_broken`）· 子件**没跑**（成对负控：`on` ⇒ 子件真跑）；
//	④ 壳级 `--dry-run --json` 全表档 ⇒ 退 0 · `count` == 名录件数 · **零副作用**（成对负控：真跑有副作用）；
//	⑤ 件级锁竞争 ⇒ 退 2（**不给结论** · 不 exec ⇒ 不落台账）。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gctTestPyExe —— 起判据件子进程用的解释器**钉死**（缺口 `GAP-20261001-61`：✗ 不用 PATH 上的裸
//
//	`python3`；同 `cicgTestPython` / `ccatTestPyExe` / `tsmPython` 一律 `/usr/bin/python3`）。
const gctTestPyExe = "/usr/bin/python3"

// gctWrite 往合成仓根写一件（自动建目录）。
func gctWrite(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// gctInv 造一条 `zerg gate check …` 的 invocation。`orig` = `os.Args[1:]`（**不含 argv0**）⇒
// `orig[2:]` 恰是壳身后的逐字参数（首枚 = 判据名）。
func gctInv(args ...string) *invocation {
	return &invocation{orig: append([]string{"gate", "check"}, args...)}
}

// gctRun 进程内跑转发壳一次 ⇒ (rc, stdout, stderr)。
func gctRun(inv *invocation, root string) (int, string, string) {
	var out, errb strings.Builder
	rc := gateCheck(inv, &out, &errb, root)
	return rc, out.String(), errb.String()
}

// ① 名字不在册 ⇒ **保守拒**（退 2 · 给可用名 · ✗ 不模糊匹配）。
func TestGateCheckUnknownNameIsUsageError(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/known-good.py", "print('ok')\n")
	rc, _, errb := gctRun(gctInv("no-such-judge"), root)
	if rc != 2 {
		t.Fatalf("未知判据名 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "不在册") || !strings.Contains(errb, "known-good") {
		t.Errorf("要明说「不在册」并**给可用名列表**：%q", errb)
	}
	if !strings.Contains(errb, "judge_resolved=false") {
		t.Errorf("壳自身错 ⇒ judge_resolved=false：%q", errb)
	}
}

// ② 子件退非 0 ⇒ 壳**原样透传**该码（不是「非 0 一律 1」），并落一行台账（六字段齐 · `raw_rc` = 子件码）。
func TestGateCheckChildExitPassthroughAndLedger(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/boom.py", "import sys\nprint('boom-out')\nsys.exit(7)\n")
	rc, out, errb := gctRun(gctInv("boom"), root)
	if rc != 7 {
		t.Fatalf("子件退 7 ⇒ 壳退 7（逐字透传），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "boom-out") {
		t.Errorf("子件 stdout 要逐字对接：%q", out)
	}
	led := filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")
	b, err := os.ReadFile(led)
	if err != nil {
		t.Fatalf("失败支要落一行台账（append-only）：%v", err)
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		t.Fatal("台账行是空的")
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatalf("台账行不是合法 JSON：%v · %q", err, line)
	}
	for _, k := range []string{"raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"} {
		if _, ok := row[k]; !ok {
			t.Errorf("台账行缺六字段之一的 `%s`：%q", k, line)
		}
	}
	if got, ok := row["raw_rc"].(float64); !ok || got != 7 {
		t.Errorf("`raw_rc` 要装子件原样退码 7：%q", line)
	}
	if row["detail"] == nil || row["detail"] == "" {
		t.Errorf("`detail` 不许空：%q", line)
	}
}

// ③ 转发开关：`off` ⇒ **不 exec** ⇒ 退 8（`detail=judge_broken`）；成对负控 `on` ⇒ 子件真跑。
func TestGateCheckForwardOffIsBlocked(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "RAN")
	gctWrite(t, root, "scripts/writer.py",
		"import os\nopen(os.environ['GCT_MARKER'],'w').write('x')\n")
	t.Setenv("GCT_MARKER", marker)

	// 负控（off）：子件不许跑。
	t.Setenv("ZERG_GATE_FORWARD", "off")
	rc, _, errb := gctRun(gctInv("writer"), root)
	if rc != 8 {
		t.Fatalf("ZERG_GATE_FORWARD=off ⇒ 退 8（不给结论 · judge_broken），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "detail=judge_broken") {
		t.Errorf("要逐字带 `detail=judge_broken`：%q", errb)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("off ⇒ **不 exec**，子件不该跑（marker 却在：%v）", err)
	}

	// 正控（on）：非 `off` ⇒ 转发开，子件真跑。
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc2, _, errb2 := gctRun(gctInv("writer"), root)
	if rc2 != 0 {
		t.Fatalf("ZERG_GATE_FORWARD=on ⇒ 照转发（子件退 0 ⇒ 壳退 0），得到 %d · stderr=%s", rc2, errb2)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("on ⇒ 子件应当真跑并产生副作用：%v", err)
	}
}

// ④ 壳级 `--dry-run --json` 全表档：退 0 · `count` == 名录件数 · **零副作用**（成对负控：真跑有副作用）。
func TestGateCheckDryRunFullTableZeroSideEffect(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "RAN")
	gctWrite(t, root, "scripts/one.py",
		"import os\nopen(os.environ['GCT_MARKER'],'w').write('x')\n")
	gctWrite(t, root, "scripts/two.sh", "#!/usr/bin/env bash\nexit 0\n")
	t.Setenv("GCT_MARKER", marker)
	t.Setenv("ZERG_GATE_FORWARD", "on")

	rc, out, errb := gctRun(gctInv("--dry-run", "--json"), root)
	if rc != 0 {
		t.Fatalf("全表干跑 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Count    int  `json:"count"`
		Resolved int  `json:"resolved"`
		DryRun   bool `json:"dry_run"`
		Entries  []struct {
			Name          string `json:"name"`
			JudgeResolved bool   `json:"judge_resolved"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	// ★ 缺口 GAP-20261012-15：`count` = **名册 N = B ∪ A** 的件数（清书面与名解析面同口径）——
	//   B = 夹具里那 2 件（目录派生）；A = inproc 键表（真搬 Go 体在册 · 与夹具根无关）⇒
	//   期望值改由**现读两张表**现算（✗ 不再写死 2：表一长旧写法必假红，且验不出 B∪A 并集）。
	want := map[string]bool{"one": true, "two": true}
	for k := range gateCheckGoEntry {
		want[k] = true
	}
	for k := range gateCheckClassEntry {
		want[k] = true
	}
	wantN := len(want)
	if env.Count != wantN || env.Resolved != wantN || len(env.Entries) != wantN {
		t.Fatalf("count/resolved/entries 要 == 名册件数 %d（名册 N = B ∪ A）：%+v", wantN, env)
	}
	if !env.DryRun {
		t.Errorf("--dry-run ⇒ dry_run 标记为真：%+v", env)
	}
	for _, e := range env.Entries {
		if !e.JudgeResolved {
			t.Errorf("逐件 judge_resolved 应为真：%+v", e)
		}
	}
	// 零副作用：干跑不许 exec（marker 不在）。
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("干跑**零副作用**，子件不该跑（marker 却在：%v）", err)
	}
	// 成对负控：真跑会产生副作用（否则本用例是恒绿）。
	if rc2, _, errb2 := gctRun(gctInv("one"), root); rc2 != 0 {
		t.Fatalf("负控：真跑 one 应退 0，得到 %d · stderr=%s", rc2, errb2)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("负控失败：真跑应当产生副作用（marker 缺）⇒ 本用例可能恒绿：%v", err)
	}
}

// ⑤ 件级锁竞争 ⇒ 退 2（**不给结论** · 不 exec ⇒ 不落台账）。调**现成的** `judge-lock.py` 造真竞争。
func TestGateCheckLockContentionIsBlocked(t *testing.T) {
	repoRel := filepath.Join("..", "..", "..")
	// 哨兵改锚 Go 半 judge_lock.go（✗ 不锚已退场 py scripts/gates/judge-lock.py）；找不到 ⇒ fail-closed（✗ 不留 Skip）。
	if _, err := os.Stat(filepath.Join(repoRel, "core", "cmd", "zerg", "judge_lock.go")); err != nil {
		t.Fatalf("哨兵失守：Go 半 core/cmd/zerg/judge_lock.go 不在 ⇒ fail-closed（✗ 不留 Skip）：%v", err)
	}
	realTool := filepath.Join(repoRel, "scripts", "gates", "judge-lock.py")
	if _, err := os.Stat(realTool); err != nil {
		// ── 退役态（A态 · 锁件 judge-lock.py 已退场）：`gateCheckAcquire` 退役支**不过问取锁**
		//   （family_gate_check.go:387-395）⇒ 即便同名锁被真持也**不阻断**：退 exitOK · 不 exec 取号件 ·
		//   不落取锁台账。此即现役语义（与 TestGateCheckAcquireLockToolRetiredSilent 同源）。
		//   ★ 哑化负控（"牙" · 带行号）：把 gateCheckAcquire 退役支改成 `return exitBlocked,false`
		//     ⇒ 本用例 FAIL（退码≠exitOK）。
		root := t.TempDir()
		marker := filepath.Join(root, "RAN")
		gctWrite(t, root, "scripts/lockprobe.py",
			"import os\nopen(os.environ['GCT_MARKER'],'w').write('x')\n")
		t.Setenv("GCT_MARKER", marker)
		t.Setenv("ZERG_GATE_LOCK_DIR", t.TempDir())
		rc, _, errb := gctRun(gctInv("lockprobe"), root)
		if rc != exitOK {
			t.Fatalf("退役态锁件不在盘 ⇒ gate-check 不过问取锁 ⇒ 不阻断（退 exitOK=%d），得到 %d · stderr=%s", exitOK, rc, errb)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Errorf("退役态不阻断 ⇒ 应继续 exec 子件（marker 应在）：%v", err)
		}
		return
	}
	root := t.TempDir()
	tool := gctWrite(t, root, "scripts/gates/judge-lock.py", "")
	src, err := os.ReadFile(realTool)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, src, 0o644); err != nil {
		t.Fatal(err)
	}
	gctWrite(t, root, "scripts/lockprobe.py", "print('lockprobe')\n")
	gctWrite(t, root, "scripts/other.py", "print('other')\n")

	lockDir := t.TempDir()
	t.Setenv("ZERG_GATE_LOCK_DIR", lockDir)

	// 起**真第二个进程**持同名锁（内部 `--_selftest-hold` 真持锁数秒）。
	// 解释器钉死（缺口 `GAP-20261001-61`）：✗ 不用 PATH 上的裸 python3；不在盘则跳过（前置不成立）。
	if _, err := os.Stat(gctTestPyExe); err != nil {
		t.Fatalf("哨兵失守：%s 不在盘（✗ 不留 Skip · fail-closed · GAP-20261001-61）：%v", gctTestPyExe, err)
	}
	holder := exec.Command(gctTestPyExe, tool, "--name", "lockprobe", "--slots", "2", "--wait", "0",
		"--_selftest-hold", "8")
	holder.Env = os.Environ()
	pipe, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	holder.Stderr = os.Stderr
	if err := holder.Start(); err != nil {
		t.Fatalf("起持锁子进程：%v", err)
	}
	defer func() { _ = holder.Process.Kill(); _, _ = holder.Process.Wait() }()

	held := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == "HELD" {
				held <- true
				return
			}
		}
		held <- false
	}()
	select {
	case ok := <-held:
		if !ok {
			t.Fatal("持锁子进程没报 HELD（前置不成立）")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("等 HELD 超时（前置不成立）")
	}

	rc, _, errb := gctRun(gctInv("lockprobe"), root)
	if rc != 2 {
		t.Fatalf("同名件级锁被占 ⇒ 退 2（不给结论），得到 %d · stderr=%s", rc, errb)
	}
	// 不 exec ⇒ 不落台账（锁竞争在 exec 之前拦下）。
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("锁竞争不 exec ⇒ 不该落台账：%v", err)
	}

	// 成对负控：**不同名**不互阻 ⇒ 照跑（退 0）。
	rc2, _, errb2 := gctRun(gctInv("other"), root)
	if rc2 != 0 {
		t.Fatalf("不同名不互阻 ⇒ 照跑退 0，得到 %d · stderr=%s", rc2, errb2)
	}
}

// ⑥ 进程内 Go 表完整性：项数 == 162，且**每项 key 都能被名解析找到**（名字 ⇒ 件 不断裂）。
// ★ 放宽（2026-10-07 退役批）：旧不变量「表键必在名录」⇒「表键 ⇒ 名录 或 inproc 键 二者其一」。
// ★ 棘轮口径：本数 == 表内真搬入口数（现读 113）。门111 check-fleet-loadable 的 Go 真搬件（check_fleet_loadable.go）入 gate_inproc_table.go 后 107→108 ⇒ 期望随动（✗ 非凑绿）。★ 2026-10-05 接线 check-manifest-freshness（mfMain）⇒ 108→109。★ 2026-10-05 同批接线 check-contract-version-diff（cvdMain）· check-wallclock-ratchet（checkWallclockRatchetMain）⇒ 109→111。★ 2026-10-06 接线 gate-check-health（gateCheckHealthMain）· gate-report（gateReportMain）⇒ 111→113。★ 2026-10-07 接线批（本单）：14 件（buildSelfclean/checkDevTargets/checkDryrunYesConflictCoverage/checkEnvKnobs/checkGapClosureDrift/checkGatePointOutput/checkGateScriptRegistered/checkMainrepoMd/checkMkdirTempCleanup/checkRecordNotEvidence/stepSwitch/stepcountDrift/writeAuthz/m2GoPyIsomorphismScan）⇒ 113→127（check-registry-key-unique 未接：自检档 stderr 非法 JSON 报错正文 py⟷Go 不逐字）。★ 2026-10-07 接线批-第一批（本单）：9 件（check-repo-symlink-farm/check-selftest-knobs/check-outrepo-dead-refs/decl-label/evalCompare/evalStabilityStats/genGlossaryExports/checkRegistryKeyUnique/checkBypassIdioms）⇒ 127→136（writeout-probe 未接：两态不逐字 —— Go 缺 PROBE_DIR 时 rc=2 fail-closed ⟷ py 原件对环境变量取键抛 KeyError ⇒ 回溯 rc=1）。★ 本单同批落 决-10：防波门 B 侧真身路径改查判据注册表 entries[].path★ 2026-10-07 接线批-第二批（本单）：15 件（checkDelegationSelfreport/checkEmptyDomainGuards/checkExecTwoCondition/checkGapStateVsArtifact/checkI18nDrift/checkLooseExitRegistry/checkModelSizeAnchor/checkPublicMarksTrackedParity/checkSelfcount/csf/checkToolVersionSync/checkVerifyCmdReal/checkWriteFaceGate/genCliReference/zergEgg）⇒ 136→151（gen-gap-ledger 未接：py 侧 stderr 逐发带 SyntaxWarning（反斜杠 d 非法转义）⟷ Go 无 ⇒ stderr 面不逐字；writer-lock 未接：转发壳按 D2 写工具档拒执 rc=2 ⟷ py 真身 rc=0）。★ 本批勘误：后接10 的 inplace-window 无 inproc 入口（inplace_window.go 无 GateMain）⇒ 不接；check-i18n·doc-index-gen·gen-tools-md·i18n-audit 真身不在判据注册表（ui/scripts·tools）⇒ 不接为判据；kb-docs-sync 无注册表真身 ⇒ 不接。★ 2026-10-07 接线批-第三批（本单）：接线 inplace-window（ipwGateMain）⇒ 151→152。gen-gap-ledger 未接：py 侧 stderr 逐发带 SyntaxWarning（反斜杠 d 非法转义 · 门环境 python3 ≥3.12）⟷ Go 无 ⇒ stderr 面不逐字；writer-lock 未接：壳按件头 write D2 声明判写工具档拒执（缺 --yes ⇒ rc=2）⟷ py 真身 rc=0 ⇒ rc/stdout 面不逐字。★ 两条待防波门补掩码/侧路后另批再接（在册 GAP-20261007-11）。★ 2026-10-07 接线批-第四批（本单·收尾）：接线 gen-gap-ledger（genGapLedgerGateMain）· writer-lock（writerLockGateMain）⇒ 152→154（防波门 DEV-1816 已落：SyntaxWarning 面掩码 ⇒ gen-gap-ledger 的 stderr 面归一；写类键 side_a 补壳级 --yes ⇒ writer-lock 写工具档 fail-closed 消解）。
// ★ 2026-10-07 接线批-第四批（本单 · 语言冻结门接入）：接线 check-lang-freeze（checkLangFreezeGateMain）⇒ 154→155。
// ★ 2026-10-07 接线批-第五批（本单）：接线 check-partial-run-acceptance（checkPartialRunAcceptanceGateMain）⇒ 155→156。
// ★ 2026-10-07 接线批-第六批（本单）：接线 check-py-go-pairing-drift（pgpdGateMain · 防波门本体）⇒ 156→157。★ 自指面：本门「自身两态对拍」= 判自己（见表A/未覆盖面）—— 本批只接线，不主张自身 py⟷Go 逐字。
// ★ 2026-10-08 接线批-第七批（本单）：接线 repo-diffstat-check（repoDiffstatGateMain · T20 报数三格薄壳的进程内适配入口）⇒ 157→158。★ 薄壳件 `repo-diffstat-check.py` 只解析 `--file`（缺省读 stdin）并把判定整件委托给正门 `zerg repo diffstat` ⇒ Go 入口同构**进程内**调 `cmdRepoDiffstat`（✗ 不起子进程）。两态对拍（`--self-test` 档）stdout/stderr/退码逐字同（防波门 `--only repo-diffstat-check` 差 0）。
// ★ 2026-10-08 接线批（M2真搬簇·本单）：接线 writeout-probe（writeoutProbeGateMain）⇒ 158→159。★ rc/stdout 面逐字同（rc 1⟷1 崩溃档 · env 齐时 JSON 逐字同）；stderr 面 = 已声明的门侧掩码缺口（回溯帧 vs 具名文案）。
// ★ 2026-10-08 接线批（本单 · check-retire 族）：接线 check-retire-consistency（checkRetireConsistencyGateMain）· check-retire-roster（retireRosterGateMain）⇒ 159→161。
// ★ 2026-10-08 接线批（本单 · GAP-20260928-59）：接线 check-genflag-surface（checkGenflagSurfaceGateMain）⇒ 161→162。
// ★ 2026-10-08 接线批（M2真搬簇·本单）：真搬并接线 check-face-parity（checkFaceParityGateMain · 判据件 check-face-parity.py 的 Go 半 · 承接 GAP-20261001-13）⇒ 162→163。
// ★ 2026-10-08 接线批（DEV-1960 · GAP-20260927-129 ④）：接线 check-doc-consumers（checkDocConsumersGateMain · 公告 consumers 只读判据）⇒ 163→164。
// ★ 2026-10-08 建键批（本单）：入表 10 枚已搬 Go 半（atomic-write · check-i18n · deploy-launch · doc-index-gen · doc-site-build · fleet-logs · gen-tools-md · i18n-audit · kb-docs-sync · ui-start）⇒ 164→174。
// ★ 2026-10-08 建键批（本单 · pack-release · DEV-2005）：接线 pack-release（packReleaseMain · scripts/build/pack-release.sh 的 Go 真身）⇒ 175→176。
// ★ 2026-10-08 接线批（GAP-20260927-347 · check-danger-guard-parity）：接线 check-danger-guard-parity（dangerGuardParityMain · 危险档↔执行门对账闸的 Go 真身 · 无 py 半）⇒ 176→177。
// ★ 2026-10-09（隐私面口径 B+A · 本单）：入表 check-public-marks-deep（checkPublicMarksDeepGateMain · 「真实出树面 ⨯ 私有面模式表」只报告档 · Go 半无 py 半 · 接既有 cptp* 出树面件族）⇒ 177→178。
// ★ 2026-10-09（AlltoGo-② 第二波「入表」· 本单）：建键 check-cli-ownership（checkCliOwnershipGateMain · CLI 归属门 · Go 半无 py 半 · 判存量闭集 py/sh 是否接 inproc 标准路由 · tri-report 只报告）⇒ 178→179。
// ★ 2026-10-09（缺口 GAP-20260927-183 · 扫 shell 执行位只报告档门 · 本单）：建键 check-shell-true-scan
//
//	（checkShellTrueScanGateMain · Go 半无 py 半 · 扫仓内「shell 键赋 True」执行位三分类计数 · tri-report 只报告）⇒ 179→180。
//
// ★ 2026-10-10（GAP-20260927-47 甲(a) · 本单）：建键 check-devdocs-root-single-source
//
//	（checkDevdocsRootSingleSourceGateMain · Go 半无 py 半 · 判据件默认根 ⟷ 版号真源只报告判据 · tri-report 只报告）⇒ 182→183。
//
// ★ 2026-10-10（GAP-20260927-220 want① · 本单）：建键 check-lineref-census
//
//	（checkLinerefCensusGateMain · Go 半无 py 半 · 件行引用语义普查「锚点符号 ⟷ 被引行」只报告判据 · tri-report 只报告）⇒ 183→184。
//
// ★ 2026-10-10（GAP-20260927-191 残面 · 本单）：建键 check-structured-count-assert
//
//	（checkStructuredCountAssertGateMain · Go 半无 py 半 · 「抽结构再数数」判据点四属性普查只报告 · tri-report）⇒ 184→185。
func TestGateCheckGoEntryTableComplete(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	byName, dupes, err := gateCheckRoster(root)
	if err != nil {
		t.Fatalf("哨兵失守：现读名录读不到（✗ 不留 Skip · fail-closed）：%v", err)
	}
	if len(dupes) != 0 {
		t.Fatalf("真实名录有同名多件 ⇒ 前置不成立：%v", dupes)
	}
	// ★ 2026-10-13（承接项 ①-6 · 本单）：发布验收三条升格真门，入表 +3
	//
	//	（check-publish-accept-lsremote / -fsck / -rollback · 见 core/cmd/zerg/check_publish_accept.go）⇒ 185→188。
	// ★ 2026-10-14（缺口 GAP-20261014-62 · 本单）：能力闸快照来源面判据件入表 +1
	//
	//	（check-capability-bridge · 见 core/cmd/zerg/check_capability_bridge.go · 网关侧路由表 id → 快照 id
	//	必经桥映射）⇒ 188→189。
	// ★ 2026-10-14（缺口 GAP-20261014-67 · 本单）：两侧同名模型 mem_gb 对拍门判据件入表 +1
	//
	//	（check-mem-gb-parity · 见 core/cmd/zerg/check_mem_gb_parity.go · 主控 fleet.yaml ⟷ 本机子端注册表
	//	逐同名模型比 mem_gb · 两侧都标口径才放行）⇒ 189→190。
	// ★ 2026-10-15（2.5.15 清账批 E4 · 本单）：两件 `.sh` 判据 Go 真搬 ＋ 入 inproc 表 +2
	//
	//	（check-agentd-unit-default / check-upgrade-local-source · 见 core/cmd/zerg/check_agentd_unit_default.go
	//	与 core/cmd/zerg/check_upgrade_local_source.go）⇒ 190→192。
	//	★ 同批撤掉那两件在 check-cli-ownership 里的「未接 inproc 标准路由」缺口 ⇒ 未接合计 3→1
	//	（余 family_publish_snapshot_native.go —— 它要动 main.go，另路）。
	// ★ 2026-10-15（2.5.15 清账 F51 · 本单）：镜像引擎承载件**生成器**入 inproc 表 +1
	//
	//	（gen-mirror-public-pyblocks · 见 core/cmd/zerg/mirror_public_pyblocks_gen.go · 入口 mpPyblocksGenGateMain ·
	//	在盘无同名 py 件 ⇒ 名解析走 inproc 兜底 · 先例同 gen-cli-reference 一族）⇒ 192→193。
	// ★ 2026-10-18（缺口 GAP-20260927-228 · 本单）：契约描述 ↔ 真源对拍门入 inproc 表 +1
	//
	//	（check-contract-describes-truth · 见 core/cmd/zerg/check_contract_describes_truth.go ·
	//	registry.json 每条 truth_shape 点名的键集 ⊇ 真源实读键集 · 1=漏列 8=读不到）⇒ 193→194。
	// ★ 2026-10-18（缺口 GAP-20260927-35 · 本单）：命令面写盘点对拍判据入 inproc 表 +1
	//
	//	（check-write-inventory · 见 core/cmd/zerg/check_write_inventory.go · 真源
	//	core/internal/contract/write-inventory.json 的「默认档是否写盘 ＋ 只读旗标名」与实践
	//	沙箱对拍；0=一致 1=破承诺 8=判不了 · 在盘无同名 py 件 ⇒ 名解析走 inproc 兜底）⇒ 194→195。
	// ★ 2026-10-22（P3 · 门133 桶内未闭归零 · 本单）：判据入 inproc 表 +1
	//
	//	（check-gap-bucket-zero · 见 core/cmd/zerg/check_gap_bucket_zero.go · 读
	//	~/.zerg/state/zerg-cli-gaps.jsonl ＋ 清单件 scripts/gates/bucket-zero-close.json；
	//	0=清单桶全归零 1=有桶未归零/白名单过期或空转 2=取数面拿不到 · 在盘无同名 py 件
	//	⇒ 名解析走 inproc 兜底）⇒ 195→196。
	if got := len(gateCheckGoEntry); got != 196 {
		t.Fatalf("进程内 Go 表项数要 == 196，得到 %d", got)
	}
	// ★ 放宽（2026-10-07 退役批）：旧不变量「表键必在名录」⇒「表键 ⇒ 名录 **或** inproc 键 二者其一」。
	//   退役件（py 已 `git rm`）只余 inproc 那一路（family_gate_check.go 的 inproc 兜底 ⇒ rc=0）；
	//   py 在盘那一路逐字同旧（byName 命中 ⇒ 沿用真件路径）。二者皆无 ⇒ 仍 fail-closed（不许出新孤儿）。
	missing := []string{}
	inprocOnly := []string{} // 有键无件（py 已退役）：名解析改由 inproc 键那一路担保。
	for name := range gateCheckGoEntry {
		_, inRoster := byName[name]
		_, inProc := gateCheckGoEntry[name]
		switch {
		case inRoster:
			// py 件在盘：名解析走名录真件路径（旧路逐字同）。
		case inProc:
			// 有键无件（退役态）：名解析走 inproc 兜底（退役批新路）。
			inprocOnly = append(inprocOnly, name)
		default:
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("表里 %d 项既不在名录、又非 inproc 兜底（名字 ⇒ 件 断裂）：%q", len(missing), missing)
	}
	if len(inprocOnly) != 0 {
		t.Logf("退役态（有键无件 · 名解析走 inproc 兜底）：%q", inprocOnly)
	}
}

// ⑦ 进程内 Go 表 key 形态：key == 名录件的**基名去扩展名**（与名录扫描面同形 · 现读交叉 · ✗ 不抄字面量）。
// ★ 放宽（2026-10-07 退役批）：名录命中 ⇒ 仍逐字同旧（key == 基名去扩展名）；
//
//	名录未命中 ⇒ 放宽为「须是 inproc 键」（退役态 · 名解析走 inproc 兜底）· ✗ 不静默（二者皆无即判红）。
func TestGateCheckGoEntryKeysMirrorRoster(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	byName, _, err := gateCheckRoster(root)
	if err != nil {
		t.Fatalf("哨兵失守：现读名录读不到（✗ 不留 Skip · fail-closed）：%v", err)
	}
	for name := range gateCheckGoEntry {
		full, ok := byName[name]
		if !ok {
			if _, inproc := gateCheckGoEntry[name]; inproc {
				continue // 退役态（有键无件）：名解析由 inproc 兜底担保。
			}
			t.Errorf("表 key %q 既不在名录、又非 inproc 兜底", name)
			continue
		}
		base := filepath.Base(full)
		got := strings.TrimSuffix(base, filepath.Ext(base))
		if got != name {
			t.Errorf("表 key %q 与名录件基名去扩展名 %q 不同形（件 %s）", name, got, full)
		}
	}
}

// ⑧ 开关**在进程内分派之前**：off ⇒ 已在 Go 表里的判据名也退 8，且**不落台账**
// （进程内分派点的台账 append 无条件 ⇒ 无行即证明没走到分派）。
func TestGateCheckForwardOffPrecedesInproc(t *testing.T) {
	name := ""
	for k := range gateCheckGoEntry {
		name = k
		break
	}
	if name == "" {
		t.Fatal("进程内 Go 表为空 ⇒ 前置不成立")
	}
	// 成对前提：该名**确实**在表里 —— 不设开关时它就会走进程内分派。
	if _, hit := gateCheckGoEntry[name]; !hit {
		t.Fatalf("前置不成立：%q 不在 Go 表里", name)
	}
	root := t.TempDir()
	gctWrite(t, root, "scripts/"+name+".py", "print('unused')\n")
	led := filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")

	t.Setenv("ZERG_GATE_FORWARD", "off")
	rc, _, errb := gctRun(gctInv(name), root)
	if rc != exitBlocked {
		t.Fatalf("ZERG_GATE_FORWARD=off ⇒ 退 %d（不给结论），得到 %d · stderr=%s", exitBlocked, rc, errb)
	}
	if !strings.Contains(errb, "转发已关") || !strings.Contains(errb, "detail=judge_broken") {
		t.Errorf("要明说「转发已关」并带 detail=judge_broken：%q", errb)
	}
	if _, err := os.Stat(led); !os.IsNotExist(err) {
		t.Errorf("off ⇒ 在进程内分派（其 append 无条件）**之前**就返回 ⇒ 不该落台账：%v", err)
	}
}

// ⑨ gateCheckRunInproc：cwd **无残留**；两枚 env 也跟着恢复（含**原本不存在**的档 ⇒ 调后回到「不存在」）。
func TestGateCheckRunInprocRestoresCwdAndEnv(t *testing.T) {
	root := t.TempDir()
	rootReal := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		rootReal = r
	}
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// 档一：两枚 env **原本不存在**。
	oldRoot, hadRoot := os.LookupEnv("ZERG_REPO_ROOT")
	oldRepo, hadRepo := os.LookupEnv("ZERG_REPO")
	_ = os.Unsetenv("ZERG_REPO_ROOT")
	_ = os.Unsetenv("ZERG_REPO")
	defer func() {
		if hadRoot {
			_ = os.Setenv("ZERG_REPO_ROOT", oldRoot)
		} else {
			_ = os.Unsetenv("ZERG_REPO_ROOT")
		}
		if hadRepo {
			_ = os.Setenv("ZERG_REPO", oldRepo)
		} else {
			_ = os.Unsetenv("ZERG_REPO")
		}
	}()

	var inWd, inRoot, inRepo string
	fn := func(stdout, stderr io.Writer, argv []string) int {
		inWd, _ = os.Getwd()
		inRoot, _ = os.LookupEnv("ZERG_REPO_ROOT")
		inRepo, _ = os.LookupEnv("ZERG_REPO")
		return 0
	}

	if rc := gateCheckRunInproc(fn, root, io.Discard, io.Discard, nil); rc != 0 {
		t.Fatalf("哑件退 0 应原样转出，得到 %d", rc)
	}
	if inWd != rootReal {
		t.Errorf("域内 cwd 应切到 root（%s），得到 %s", rootReal, inWd)
	}
	if inRoot != root || inRepo != root {
		t.Errorf("域内 ZERG_REPO_ROOT/ZERG_REPO 都应设为 root（%s）：%q / %q", root, inRoot, inRepo)
	}
	if after, _ := os.Getwd(); after != before {
		t.Errorf("cwd 有残留：调前 %s · 调后 %s", before, after)
	}
	if v, ok := os.LookupEnv("ZERG_REPO_ROOT"); ok {
		t.Errorf("原本不存在 ⇒ 调后应回到「不存在」，却 = %q", v)
	}
	if v, ok := os.LookupEnv("ZERG_REPO"); ok {
		t.Errorf("原本不存在 ⇒ 调后应回到「不存在」，却 = %q", v)
	}

	// 档二：两枚 env **原本有值** ⇒ 调后回到原值。
	_ = os.Setenv("ZERG_REPO_ROOT", "/sentinel/root")
	_ = os.Setenv("ZERG_REPO", "/sentinel/repo")
	if rc := gateCheckRunInproc(fn, root, io.Discard, io.Discard, nil); rc != 0 {
		t.Fatalf("哑件退 0 应原样转出（档二），得到 %d", rc)
	}
	if v, _ := os.LookupEnv("ZERG_REPO_ROOT"); v != "/sentinel/root" {
		t.Errorf("原本有值 ⇒ 调后应回原值 /sentinel/root，却 = %q", v)
	}
	if v, _ := os.LookupEnv("ZERG_REPO"); v != "/sentinel/repo" {
		t.Errorf("原本有值 ⇒ 调后应回原值 /sentinel/repo，却 = %q", v)
	}
	if after, _ := os.Getwd(); after != before {
		t.Errorf("cwd 有残留（档二）：调前 %s · 调后 %s", before, after)
	}
}

// ⑩ 未命中 ⇒ 原路：不在 Go 表里的判据名**不走进程内**（真起解释器 exec 件）；连名录都不在 ⇒ 走「不在册」退 2。
func TestGateCheckNameMissFallsThroughToExec(t *testing.T) {
	const name = "gct-not-in-go-table" // 自造名：现读断言它不在进程内 Go 表里。
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟在 Go 表里", name)
	}
	root := t.TempDir()
	gctWrite(t, root, "scripts/"+name+".py", "print('gct-exec-path')\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	rc, out, errb := gctRun(gctInv(name), root)
	if rc != 0 {
		t.Fatalf("不在 Go 表 ⇒ 走 exec 原路（合成 py 退 0 ⇒ 壳退 0），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "gct-exec-path") {
		t.Errorf("未命中表 ⇒ 应 exec 真件（透传其 stdout），却 %q", out)
	}
	b, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl"))
	if err != nil {
		t.Fatalf("exec 支应落台账：%v", err)
	}
	if !strings.Contains(string(b), "\"detail\":\"ok\"") {
		t.Errorf("exec 成功支台账该落 detail=ok：%q", string(b))
	}

	// 反面对照：名字**连名录都不在** ⇒ 走「不在册」那条错路（退 2 · 不 exec）。
	rc2, _, errb2 := gctRun(gctInv("gct-nowhere-at-all"), root)
	if rc2 != exitUsage {
		t.Fatalf("不在册 ⇒ 退 %d（保守拒），得到 %d · stderr=%s", exitUsage, rc2, errb2)
	}
	if !strings.Contains(errb2, "不在册") {
		t.Errorf("要逐字说「不在册」：%q", errb2)
	}
}

// ⑪ ★ GAP-20261005-20 · **重编类护栏**（属性判）：落在构建/发布入口目录 或 非注释行写 repo bin/ 的件
//
//	⇒ 与写工具同档，缺 `--yes` 拒执（退 2 · **不 exec**）；给 `--yes` ⇒ 照跑（成对负控，证护栏是唯一拦点）。
func TestGateCheckRebuildClassGuard(t *testing.T) {
	root := t.TempDir()
	// 重编类夹具：落在构建/发布入口目录，且**非注释行**含「写 repo bin/」写位形态（在 `if false` 里 ⇒ 不真跑）。
	gctWrite(t, root, "scripts/build/build-all.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-rebuild-fixture\n"+
			"if false; then\n"+
			"  go build -o \"${REPO_ROOT}/bin/gct-artifact\" ./cmd/x\n"+
			"fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 成对负控（护栏的另一半）：给 `--yes` ⇒ 重编类件**照跑**（证它不是「永远拒执」）。
	invYes := gctInv("build-all", "--yes")
	invYes.yes = true // 进程内测试里 `--yes` 由全局解析置位（✗ 不靠 orig 反解）
	rcYes, outYes, errbYes := gctRun(invYes, root)
	if rcYes != 0 || !strings.Contains(outYes, "gct-rebuild-fixture") {
		t.Fatalf("给 --yes ⇒ 重编类件照跑（成对负控）：rc=%d out=%q stderr=%q", rcYes, outYes, errbYes)
	}

	// 改后语义（有护栏）：缺 `--yes` ⇒ 拒执（退 2）· **不 exec**（夹具的 echo 不出现）。
	rc, out, errb := gctRun(gctInv("build-all"), root)
	if rc != exitUsage {
		t.Fatalf("重编类缺 --yes ⇒ 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-rebuild-fixture") {
		t.Errorf("缺 --yes ⇒ **不 exec**，夹具不该跑：out=%q", out)
	}
	if !strings.Contains(errb, "重编类") || !strings.Contains(errb, "--yes") {
		t.Errorf("要明说「重编类」并指 --yes：%q", errb)
	}
}

// ⑫ ★ 属性判**不是靠名**：件既不在构建目录、名字也不含 build/all —— 仅凭「非注释行写 repo bin/」就该判危险档。
func TestGateCheckRebuildClassByContentNotName(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/whatever.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-content-fixture\n"+
			"if false; then printf x > bin/zerg-core; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("whatever"), root)
	if rc != exitUsage {
		t.Fatalf("非注释行写 bin/ ⇒ 判重编类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-content-fixture") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑬ ★ 不回归：**只读判据**（读 bin/、注释里提构建目录）⇒ 不受影响，照跑退 0（✗ 不把日常只读判据全变危险档）。
func TestGateCheckReadOnlyJudgeUnaffected(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/readonly-judge.py",
		"import sys\n"+
			"# 只是**读**：下面这两处都不是写位形态\n"+
			"#   open('bin/zerg','r')  · grep scripts/build/build-all.sh\n"+
			"print('readonly-ok')\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("readonly-judge"), root) // ✗ 不给 --yes
	if rc != 0 {
		t.Fatalf("只读判据 ⇒ 照跑退 0（不受护栏影响），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "readonly-ok") {
		t.Errorf("只读判据 stdout 应透传：%q", out)
	}
}

// ⑭ ★ 目录角色属性：落在构建/发布入口目录下的件（即便正文没有写位形态）⇒ 仍判重编类。
func TestGateCheckRebuildDirRole(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/build/plain-helper.sh", "#!/usr/bin/env bash\necho plain\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, _, errb := gctRun(gctInv("plain-helper"), root)
	if rc != exitUsage {
		t.Fatalf("构建目录角色 ⇒ 判重编类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
}

// ⑮ ★ GAP-20261005-20 另一半 · **整套门类护栏**（属性判）：会**真跑整套门/步**的件（遍历名册逐个真跑）
//
//	⇒ 与写工具同档，缺 `--yes` 拒执（退 2 · **不 exec**）；给 `--yes` ⇒ 照跑（成对负控，证护栏是唯一拦点）。
func TestGateCheckSuiteClassGuard(t *testing.T) {
	root := t.TempDir()
	// 整套门夹具：现读名册目录（`${GATEDIR}`）并**命令位**起解释器跑一个变量执行体（在 `if false` 里 ⇒ 不真跑）。
	gctWrite(t, root, "scripts/gates/suite-runner.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-suite-fixture\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then\n"+
			"  bash \"${GATEDIR}/some-gate.py\"\n"+
			"fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 成对负控：给 `--yes` ⇒ 整套门类件**照跑**（证它不是「永远拒执」）。
	invYes := gctInv("suite-runner", "--yes")
	invYes.yes = true
	rcYes, outYes, errbYes := gctRun(invYes, root)
	if rcYes != 0 || !strings.Contains(outYes, "gct-suite-fixture") {
		t.Fatalf("给 --yes ⇒ 整套门类件照跑（成对负控）：rc=%d out=%q stderr=%q", rcYes, outYes, errbYes)
	}

	// 改后语义（有护栏）：缺 `--yes` ⇒ 拒执（退 2）· **不 exec**（夹具的 echo 不出现）。
	rc, out, errb := gctRun(gctInv("suite-runner"), root)
	if rc != exitUsage {
		t.Fatalf("整套门类缺 --yes ⇒ 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-suite-fixture") {
		t.Errorf("缺 --yes ⇒ **不 exec**，夹具不该跑：out=%q", out)
	}
	if !strings.Contains(errb, "整套门") || !strings.Contains(errb, "--yes") {
		t.Errorf("要明说「整套门类」并指 --yes：%q", errb)
	}
}

// ⑯ ★ 属性判**不是靠名**：件名字既不含 all 也不含 real-gates —— 仅凭「现读名册 + 命令位跑变量执行体」就该判危险档。
func TestGateCheckSuiteClassByContentNotName(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/zzz-whatever.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-suite-content\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then exec \"${GATEDIR}/g.py\"; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("zzz-whatever"), root)
	if rc != exitUsage {
		t.Fatalf("名册执行属性 ⇒ 判整套门类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-suite-content") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑰ ★ 步命令执行属性（shell 起 `-c`）：整套门**执行器**本体 ⇒ 同档拒执。
func TestGateCheckSuiteStepCmdGuard(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/exec-body.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-stepcmd-fixture\n"+
			"if false; then bash -c \"${cmd}\"; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("exec-body"), root)
	if rc != exitUsage {
		t.Fatalf("步命令执行属性 ⇒ 判整套门类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-stepcmd-fixture") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑱ ★ 不回归：「只读读步骤表」的判据（把步骤表当文本读 · 带只读档起名册件）⇒ 不受影响，照跑退 0。
func TestGateCheckStepTableReadersUnaffected(t *testing.T) {
	root := t.TempDir()
	// 读步骤表（合成步表 + open 解析）—— 从不起命令位解释器 ⇒ 两条属性都不命中。
	gctWrite(t, root, "scripts/gates/reader-judge.py",
		"# 只读：把步表当文本读（不 exec）\n"+
			"txt = open('scripts/gates/step-table.sh').read()\n"+
			"print('reader-ok')\n")
	gctWrite(t, root, "scripts/gates/step-table.sh",
		"#!/usr/bin/env bash\n# add_step go \"x\" tri \"${REPO_ROOT}\" \"echo x\"\n")
	// 名册件**带只读档**（`--self-test`）⇒ 同行有只读档 ⇒ 不判真跑。
	gctWrite(t, root, "scripts/gates/selfonly.sh",
		"#!/usr/bin/env bash\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then bash \"${GATEDIR}/g.py\" --self-test; fi\n"+
			"echo selfonly-ok\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	for _, name := range []string{"reader-judge", "selfonly"} {
		rc, _, errb := gctRun(gctInv(name), root)
		if rc != 0 {
			t.Fatalf("只读判据 %q ⇒ 照跑退 0（不受护栏影响），得到 %d · stderr=%s", name, rc, errb)
		}
	}
}

// ⑲ ★ 退役最后一道前置：**键 ∈ inproc 表 ⇒ 视为在册**（✗ 不再要求 py 件在盘）。
//
//	名解析真源从「现读 scripts/ 递归列举」并入「现读名录 ∪ inproc 表键」—— 某键的 py 件退役
//	（已删）后，判据体仍在 Go 侧（gateCheckGoEntry）跑得动 ⇒ 照跑。
//	两态（**仓外合成**「退役态」夹具 · 只动合成面，真仓不删任何 py 件）：
//	  ①「有键无件」：inproc 表里的键、盘上无 py 件 ⇒ 名解析通过、走 inproc（rc/输出 = Go 体真读数）；
//	     成对对照：py 件在盘时**逐字同**（inproc 优先 ⇒ 行为不变）；
//	  ②「既不在 inproc、又无 py 件」⇒ 仍退 2「不在册」（成对负控 · 必留一格）。
func TestGateCheckInprocKeyResolvesWithoutPyFile(t *testing.T) {
	const name = "gct-retired-probe" // 合成键：现读断言它不在真表里（避免污染真表）。
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在 Go 表里", name)
	}
	// 注入一枚合成 inproc 入口（跑完删回 —— 只动本进程内存 · ✗ 不碰 gate_inproc_table.go）。
	gateCheckGoEntry[name] = func(stdout, stderr io.Writer, argv []string) int {
		_, _ = io.WriteString(stdout, "gct-inproc-ran\n")
		return 0
	}
	defer delete(gateCheckGoEntry, name)

	root := t.TempDir()
	py := gctWrite(t, root, "scripts/gates/"+name+".py", "print('py-should-not-run')\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 态①-a（成对对照）：py 件在盘 ⇒ 行为不变（inproc 优先 ⇒ 走 Go 体 · py 不跑）。
	rc, out, errb := gctRun(gctInv(name), root)
	if rc != 0 || !strings.Contains(out, "gct-inproc-ran") || strings.Contains(out, "py-should-not-run") {
		t.Fatalf("py 件在盘 ⇒ 走 inproc、行为不变：rc=%d out=%q stderr=%q", rc, out, errb)
	}

	// 态①-b（退役态）：**把 py 件移开** ⇒ 键仍在 inproc 表 ⇒ **仍解析、仍走 inproc**（本单新行为）。
	if err := os.Remove(py); err != nil {
		t.Fatal(err)
	}
	rc, out, errb = gctRun(gctInv(name), root)
	if rc != 0 {
		t.Fatalf("有键无件（退役态）⇒ 名解析应通过并走 inproc 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "gct-inproc-ran") {
		t.Errorf("退役态应走 inproc 真体（stdout 带标记），却 %q", out)
	}
	if strings.Contains(errb, "不在册") {
		t.Errorf("退役态**不该**判「不在册」（那正是本单要除的病）：%q", errb)
	}

	// 态②（成对负控）：既不在 inproc、又无 py 件 ⇒ 仍退 2「不在册」（必留一格）。
	if _, hit := gateCheckGoEntry["gct-truly-absent"]; hit {
		t.Fatal("前置不成立：gct-truly-absent 竟在 Go 表里")
	}
	rc2, _, errb2 := gctRun(gctInv("gct-truly-absent"), root)
	if rc2 != exitUsage {
		t.Fatalf("既不在 inproc、又无 py 件 ⇒ 退 %d（保守拒），得到 %d · stderr=%s", exitUsage, rc2, errb2)
	}
	if !strings.Contains(errb2, "不在册") {
		t.Errorf("负控要逐字说「不在册」：%q", errb2)
	}
}

// TestGateCheckAcquireLockToolRetiredSilent —— G16（2026-10-07 退役批）：锁件 `judge-lock.py`
// 退役（不在盘）而它的 Go 半（inproc 键 `judge-lock`）在册 ⇒ **退役态不过问**：静默跳过取锁
// （✗ 不告警 / ✗ 不改退码）—— 否则每跑一门都往 stderr 吐一行警告，弄脏「两态三面逐字同」的面。
// ★ 哑化负控：抽掉 `if _, inproc := gateCheckGoEntry[gateCheckLockToolKey]; inproc` 这层
// （即无论有无 Go 半都走「如实报」那支，或反之恒静默）⇒ 下面「负控」格 FAIL（带行号）。
func TestGateCheckAcquireLockToolRetiredSilent(t *testing.T) {
	root := t.TempDir()
	// ✗ 不放 scripts/gates/judge-lock.py —— 即退役态。
	var errb bytes.Buffer
	rc, proceed := gateCheckAcquire(root, "any-gate", &errb)
	if rc != exitOK || !proceed {
		t.Fatalf("退役态该静默放行：(rc=%d, proceed=%v)", rc, proceed)
	}
	if errb.Len() != 0 {
		t.Fatalf("退役态该**静默**（✗ 不告警）：stderr=%q", errb.String())
	}

	// 负控：Go 半也移开（现读时刻删掉 inproc 键）⇒ 保留原「如实报」档（✗ 不静默降级）。
	saved, had := gateCheckGoEntry[gateCheckLockToolKey]
	delete(gateCheckGoEntry, gateCheckLockToolKey)
	defer func() {
		if had {
			gateCheckGoEntry[gateCheckLockToolKey] = saved
		}
	}()
	var errb2 bytes.Buffer
	rc2, p2 := gateCheckAcquire(root, "any-gate", &errb2)
	if rc2 != exitOK || !p2 {
		t.Fatalf("负控（无 Go 半）仍应放行：(rc=%d, %v)", rc2, p2)
	}
	if !strings.Contains(errb2.String(), "件级锁件不在盘") {
		t.Fatalf("负控（无 Go 半）该保留「如实报」档：stderr=%q", errb2.String())
	}
}

// ⒏ ★ 本批（2026-10-08 拍板⑤）· **受信运行器「闸内免盾」机制**（单旋钮 `ZERG_TRUSTED_RUNNER=1`）。
//
//	只免「`gate check` 危险档（写工具 / 重编类 / 整套门类）缺 `--yes` ⇒ 拒执」这**一道**盾；
//	✗ 不放宽 `--yes` 本身；缺旋钮 / 值不对 ⇒ 与今天**逐字同**（裸跑仍 rc=2 · 零写）。
//	ⓑ 留痕：免盾时 stderr 出审计行（受信身份 · 哪一步 · 免的哪一档）＋台账行带 `waiver` 字段（六字段形状不变）。
func TestGateCheckTrustedRunnerWaiver(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "RAN")
	// 危险档夹具：分类**走中央登记**（GAP-20261008-11 后 ✗ 不再靠件头）⇒ 注入一格 gateCheckClassEntry
	// （跑完删回 · 只动本进程内存 · ✗ 不碰 gate_inproc_table.go）。真跑时落 marker（代表「真写」）。
	gctWrite(t, root, "scripts/gates/trusted-fixture.sh",
		"#!/usr/bin/env bash\n"+
			"printf x > \"$GCT_MARKER\"\n"+
			"echo gct-trusted-fixture\n")
	gateCheckClassEntry["trusted-fixture"] = gateCheckClassSpec{"write", dangerD2}
	defer delete(gateCheckClassEntry, "trusted-fixture")
	t.Setenv("GCT_MARKER", marker)
	t.Setenv("ZERG_GATE_FORWARD", "on")
	t.Setenv("ZERG_TRUSTED_RUNNER", "")
	led := filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")
	_ = os.Remove(marker)

	// (a) 缺旋钮 + 缺 `--yes` ⇒ 危险档拒执（退 2 · **不 exec** · 零写 —— 与今天逐字同）。
	rc, out, errb := gctRun(gctInv("trusted-fixture"), root)
	if rc != exitUsage {
		t.Fatalf("缺旋钮缺 --yes ⇒ 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-trusted-fixture") {
		t.Errorf("缺 --yes ⇒ **不 exec**：out=%q", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("缺旋钮 ⇒ 零写（marker 不该在）：%v", err)
	}

	// (b) 旋钮值**不对**（"true" · 非闭集）⇒ 视为没设 ⇒ 与 (a) 同（退 2 · 不 exec · 零写）。
	t.Setenv("ZERG_TRUSTED_RUNNER", "true")
	rc2, out2, _ := gctRun(gctInv("trusted-fixture"), root)
	if rc2 != exitUsage || strings.Contains(out2, "gct-trusted-fixture") {
		t.Fatalf("旋钮值不对 ⇒ 视为没设（退 %d · 不 exec），得到 rc=%d out=%q", exitUsage, rc2, out2)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("旋钮值不对 ⇒ 零写：%v", err)
	}

	// (c) 受信运行器（=1）+ 缺 `--yes` ⇒ **免这一道盾**：真跑（退 0）+ 真写 + ⓑ 留痕。
	t.Setenv("ZERG_TRUSTED_RUNNER", "1")
	rc3, out3, errb3 := gctRun(gctInv("trusted-fixture"), root)
	if rc3 != 0 || !strings.Contains(out3, "gct-trusted-fixture") {
		t.Fatalf("受信运行器 ⇒ 危险档真跑（退 0 · 子件 stdout 透传）：rc=%d out=%q stderr=%q", rc3, out3, errb3)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("受信运行器 ⇒ 真跑（marker 应在）：%v", err)
	}
	if !strings.Contains(errb3, "免盾") || !strings.Contains(errb3, "ZERG_TRUSTED_RUNNER=1") ||
		!strings.Contains(errb3, "gate check trusted-fixture") {
		t.Errorf("ⓑ 留痕：要明说「免盾」· 身份（ZERG_TRUSTED_RUNNER=1）· 哪一步（gate check trusted-fixture）：%q", errb3)
	}
	// 台账行带 waiver 字段（可查证的审计面）。
	b, err := os.ReadFile(led)
	if err != nil {
		t.Fatalf("免盾那一步应落台账：%v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	var row map[string]any
	if err := json.Unmarshal([]byte(last), &row); err != nil {
		t.Fatalf("台账行不是合法 JSON：%v · %q", err, last)
	}
	if row["waiver"] != "trusted-runner:ZERG_TRUSTED_RUNNER=1" {
		t.Errorf("台账 waiver 字段应记「谁免的盾」：%q", last)
	}
	for _, k := range []string{"raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"} {
		if _, ok := row[k]; !ok {
			t.Errorf("台账行仍须齐六字段：缺 %s · %q", k, last)
		}
	}

	// (d) `--dry-run` ⇒ 退 0 · 出计划件 · 零写（与旋钮无关）。
	_ = os.Remove(marker)
	invDry := gctInv("trusted-fixture", "--dry-run")
	invDry.dryRun = true
	rc4, out4, _ := gctRun(invDry, root)
	if rc4 != 0 || !strings.Contains(out4, "计划件") {
		t.Fatalf("--dry-run ⇒ 退 0 出计划件，得到 rc=%d out=%q", rc4, out4)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("--dry-run ⇒ 零写：%v", err)
	}

	// (e) `--dry-run --yes` ⇒ 自相矛盾 ⇒ 退 2（与旋钮无关）。
	invDY := gctInv("trusted-fixture", "--dry-run", "--yes")
	invDY.dryRun = true
	invDY.yes = true
	rc5, _, errb5 := gctRun(invDY, root)
	if rc5 != exitUsage {
		t.Fatalf("--dry-run --yes ⇒ 退 %d（自相矛盾），得到 %d · stderr=%s", exitUsage, rc5, errb5)
	}

	// (f) 旋钮**不放宽 `--yes` 本身**：无旋钮 + `--yes` ⇒ 照旧真跑（退 0 · 真写）。
	t.Setenv("ZERG_TRUSTED_RUNNER", "")
	_ = os.Remove(marker)
	invYes := gctInv("trusted-fixture", "--yes")
	invYes.yes = true
	rc6, out6, errb6 := gctRun(invYes, root)
	if rc6 != 0 || !strings.Contains(out6, "gct-trusted-fixture") {
		t.Fatalf("无旋钮 + --yes ⇒ 照旧真跑：rc=%d out=%q stderr=%q", rc6, out6, errb6)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("无旋钮 + --yes ⇒ 真写：%v", err)
	}
}

// ㉑ ★ GAP-20261008-11 · **退件流程那一格**：写工具件**在盘 ⟷ 离盘（退役）**，
//
//	`zerg gate check <判据名> --dry-run` 的 **rc 与 stdout 逐字相同**（✗ 静默失明）。
//
//	破口（件头方案）：件一离盘 ⇒ 件头随之消失 ⇒ 分类读空 ⇒ 写工具闸消失 ⇒ `--dry-run` 泄漏给子件
//	（退 rc=2、stdout 空）—— 与在盘的 rc=0/计划件**不同形**。本格断言：分类改中央登记后，
//	退件前后**逐字同**。成对负控：件离盘后**裸跑**仍退 2（缺 --yes 的盾不随件消失）。
//
//	与 TestGateCheckInprocKeyResolvesWithoutPyFile 的配套：那格证「键在 inproc ⇒ 名可解析」；
//	本格证「**在册名的档位**不随件离盘而变」（分类真源 = gateCheckClassEntry，与件在不在盘无关）。
func TestGateCheckDryRunStableAcrossRetire(t *testing.T) {
	const name = "gct-class-probe" // 合成名（✗ 避免污染真表）
	if _, hit := gateCheckClassEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在中央分类表里", name)
	}
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在 Go 表里", name)
	}
	// 中央登记：判据名 ⇒ 写工具 D2；inproc 键（退役态名解析的兜底，与 gateCheckClassOf 分属两件事）。
	gateCheckClassEntry[name] = gateCheckClassSpec{"write", dangerD2}
	gateCheckGoEntry[name] = func(stdout, stderr io.Writer, argv []string) int { return 0 }
	defer func() { delete(gateCheckClassEntry, name); delete(gateCheckGoEntry, name) }()

	root := t.TempDir()
	sh := gctWrite(t, root, "scripts/gates/"+name+".sh", "#!/usr/bin/env bash\necho gct-class-probe\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	dry := func() (int, string, string) {
		inv := gctInv(name, "--dry-run")
		inv.dryRun = true
		return gctRun(inv, root)
	}

	// 态①（件在盘）：写工具档 ⇒ --dry-run 出计划件、退 0。
	rcA, outA, errA := dry()
	if rcA != exitOK || !strings.Contains(outA, "计划件") {
		t.Fatalf("件在盘：--dry-run 应退 %d 出计划件，得到 rc=%d out=%q stderr=%q", exitOK, rcA, outA, errA)
	}

	// 态②（件离盘 · 退役）：分类走中央表 ⇒ rc 与 stdout **逐字同**（件头方案下此格会红）。
	if err := os.Remove(sh); err != nil {
		t.Fatal(err)
	}
	rcB, outB, errB := dry()
	if rcB != rcA || outB != outA {
		t.Fatalf("件离盘后 --dry-run 与在盘**逐字同**（✗ 静默失明）：\n  在盘 rc=%d out=%q\n  离盘 rc=%d out=%q stderr=%q",
			rcA, outA, rcB, outB, errB)
	}

	// 成对负控：件离盘后**裸跑**（缺 --yes）仍退 2 —— 写工具闸不随件消失。
	rcC, _, errC := gctRun(gctInv(name), root)
	if rcC != exitUsage || !strings.Contains(errC, "--yes") {
		t.Fatalf("件离盘后裸跑仍须退 %d（缺 --yes ⇒ 拒执），得到 rc=%d stderr=%q", exitUsage, rcC, errC)
	}
}

// ㉒ ★ A 方案（2026-10-08）· **重编/写档类退役不失效**（件缺席而盾仍在）＝ T1。
//
//	合成名登记为中央表 `write`（判据名 ⇒ 危险档 · 与件在不在盘无关）⇒ 件在盘 ⟷ 件离盘两态：
//	`--dry-run` 均退 0 且**计划件 stdout 逐字同**；裸跑两态均退 2（缺 `--yes` 的盾不随件消失）。
//	破口（不登中央表 · 仅靠目录判/行为判）：件一离盘 ⇒ 甲/乙读空 ⇒ 盾静默消失 ⇒ 在盘态无计划件、
//	子件照跑，与离盘态不同形 ⇒ 本格红。件落 `scripts/gates/`（非构建目录）⇒ kind 两态均「写工具」。
//	负控：删本用例里那行 `gateCheckClassEntry[name] = …` ⇒ 态①即红（证本格有牙）。
func TestGateCheckRebuildClassShieldStableAcrossRetire(t *testing.T) {
	const name = "gct-rebuild-probe" // 合成名（✗ 避免污染真表）
	if _, hit := gateCheckClassEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在中央分类表里", name)
	}
	gateCheckClassEntry[name] = gateCheckClassSpec{"write", dangerD2}
	defer delete(gateCheckClassEntry, name)

	root := t.TempDir()
	sh := gctWrite(t, root, "scripts/gates/"+name+".sh", "#!/usr/bin/env bash\necho gct-rebuild-probe\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	dry := func() (int, string, string) {
		inv := gctInv(name, "--dry-run")
		inv.dryRun = true
		return gctRun(inv, root)
	}

	// 态①（件在盘）：登记 write ⇒ --dry-run 出计划件、退 0。
	rcA, outA, errA := dry()
	if rcA != exitOK || !strings.Contains(outA, "计划件") {
		t.Fatalf("件在盘：--dry-run 应退 %d 出计划件，得到 rc=%d out=%q stderr=%q", exitOK, rcA, outA, errA)
	}
	// 态①裸跑（缺 --yes）：登记 write ⇒ 拒执退 2 · **不 exec**。
	rcA2, outA2, errA2 := gctRun(gctInv(name), root)
	if rcA2 != exitUsage || strings.Contains(outA2, "gct-rebuild-probe") || !strings.Contains(errA2, "--yes") {
		t.Fatalf("件在盘裸跑：缺 --yes ⇒ 退 %d · 不 exec，得到 rc=%d out=%q stderr=%q", exitUsage, rcA2, outA2, errA2)
	}

	// 态②（件离盘 · 退役）：分类走中央表 ⇒ --dry-run rc 与 stdout **逐字同**（不登表则此格红）。
	if err := os.Remove(sh); err != nil {
		t.Fatal(err)
	}
	rcB, outB, errB := dry()
	if rcB != rcA || outB != outA {
		t.Fatalf("件离盘后 --dry-run 与在盘**逐字同**（件缺席而盾仍在）：\n  在盘 rc=%d out=%q\n  离盘 rc=%d out=%q stderr=%q",
			rcA, outA, rcB, outB, errB)
	}
	// 态②裸跑：缺 --yes 的盾**不随件消失** ⇒ 仍退 2。
	rcB2, outB2, errB2 := gctRun(gctInv(name), root)
	if rcB2 != exitUsage || strings.Contains(outB2, "gct-rebuild-probe") || !strings.Contains(errB2, "--yes") {
		t.Fatalf("件离盘后裸跑仍须退 %d（缺 --yes ⇒ 拒执 · 不 exec），得到 rc=%d out=%q stderr=%q", exitUsage, rcB2, outB2, errB2)
	}
}

// ㉓ ★ A 方案 · **登 `read` 能压过目录判**＝ T2（今日 OR 下做不到的那一格）。
//
//	合成名登记为中央表 `read` 且件落 `scripts/build/`（否则必命中甲目录判）⇒ **无盾**：裸跑照 exec
//	子件（退 0 · 子件 stdout 透传）。成对负控：同件、同盘，把该登记删掉 ⇒ 退回「未登走甲兜底」⇒
//	目录判命中 ⇒ 有盾（退 2 · 不 exec）—— 证 `read` 登记正是「压过目录判」的那一步。
func TestGateCheckRegisteredReadOverridesDirClass(t *testing.T) {
	const name = "gct-read-override-probe" // 合成名
	if _, hit := gateCheckClassEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在中央分类表里", name)
	}
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在 Go 表里", name)
	}
	root := t.TempDir()
	gctWrite(t, root, "scripts/build/"+name+".sh", "#!/usr/bin/env bash\necho gct-read-override\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 态①（登 read）：压过目录判 ⇒ 无盾 ⇒ 裸跑照 exec（退 0 · stdout 透传）。
	gateCheckClassEntry[name] = gateCheckClassSpec{"read", ""}
	rc, out, errb := gctRun(gctInv(name), root)
	delete(gateCheckClassEntry, name)
	if rc != 0 || !strings.Contains(out, "gct-read-override") {
		t.Fatalf("登 read 应压过目录判（无盾 · 照 exec 退 0），得到 rc=%d out=%q stderr=%q", rc, out, errb)
	}

	// 态②（成对负控 · 删该登记）：未登 ⇒ 走甲兜底 ⇒ 目录判命中 ⇒ 有盾（退 2 · 不 exec）。
	if _, hit := gateCheckClassEntry[name]; hit {
		t.Fatalf("态②前置：登记应已删")
	}
	rc2, out2, errb2 := gctRun(gctInv(name), root)
	if rc2 != exitUsage || strings.Contains(out2, "gct-read-override") || !strings.Contains(errb2, "--yes") {
		t.Fatalf("未登 ⇒ 目录判兜底应给盾（退 %d · 不 exec），得到 rc=%d out=%q stderr=%q", exitUsage, rc2, out2, errb2)
	}
}

// ㉔ ★ A 方案 · **未登名兜底逐字同旧**＝ T3：件落 `scripts/build/`（甲目录判）而中央表未登 ⇒
//
//	在盘有盾（退 2 · 不 exec）；件离盘 ⇒ 名解析兜底两条（inproc 键/中央表键）皆不中 ⇒ 退 2「不在册」
//	（与今日同 · 防 A 误碰兜底路）。与 T2 成对：T2 证「登 read 压过目录判」，本格证「未登仍走甲」。
func TestGateCheckUnregisteredBuildDirStillShields(t *testing.T) {
	const name = "gct-unreg-dir-probe"
	if _, hit := gateCheckClassEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在中央分类表里", name)
	}
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟已在 Go 表里", name)
	}
	root := t.TempDir()
	sh := gctWrite(t, root, "scripts/build/"+name+".sh", "#!/usr/bin/env bash\necho gct-unreg-dir\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 态①（件在盘）：未登 ⇒ 甲目录判兜底 ⇒ 有盾（退 2 · 不 exec）。
	rc, out, errb := gctRun(gctInv(name), root)
	if rc != exitUsage || strings.Contains(out, "gct-unreg-dir") {
		t.Fatalf("未登件在盘：目录判兜底应给盾（退 %d · 不 exec），得到 rc=%d out=%q stderr=%q", exitUsage, rc, out, errb)
	}
	if !strings.Contains(errb, "--yes") || !strings.Contains(errb, "重编类") {
		t.Fatalf("未登件在盘：要明说「重编类」并指 --yes：%q", errb)
	}

	// 态②（件离盘）：既不在 inproc、又未登中央表 ⇒ 名解析不中 ⇒ 退 2「不在册」（与今日同）。
	if err := os.Remove(sh); err != nil {
		t.Fatal(err)
	}
	rc2, _, errb2 := gctRun(gctInv(name), root)
	if rc2 != exitUsage || !strings.Contains(errb2, "不在册") {
		t.Fatalf("未登件离盘：仍应退 %d「不在册」（兜底两路皆不中 · 与今日同），得到 rc=%d stderr=%q", exitUsage, rc2, errb2)
	}
}

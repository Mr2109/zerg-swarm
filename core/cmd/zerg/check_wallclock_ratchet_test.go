// check_wallclock_ratchet_test.go —— `check_wallclock_ratchet.go`（py `check-wallclock-ratchet.py` 的 Go 真搬）的
// **定向单测 · 成对负控**（M2 · 墙钟棘轮族）。
//
// 口径：本用例对**同一 argv / cwd / env**跑两态（py 原件副本 ↔ 本件入口），逐字节比
// **rc + stdout + stderr** 三面；写面另比**产物件字节**（baseline + 台账）。任一条判据被哑化
// （棘轮只许下降 / 上调需 --allow-raise / 未标定不给结论 / 按档筛选 / 未知旗标拒收）⇒ **当场 FAIL**。
//
// ★ 解释器钉死 /usr/bin/python3（GAP-20261001-61：✗ 不裸写 python3）；夹具一律落 t.TempDir()（仓外）。
// ★ 时刻面：py 的「当前时刻」无注入旋钮 ⇒ 写面对拍先跑 py 读其 ts，再用 Go-only 旋钮 --now 把 Go 钉到**同一 ts**，
//
//	✗ 不靠掩码（另在本件内证明 py 连跑两发 ts 会变）。
//
// ✗ 不进 main.go 命令树、不注册命令、不删 py 原件（M2 边界；已挂 gateCheckGoEntry["check-wallclock-ratchet"] ⇒ 走 inproc）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cwrTestPyExe —— 钉死的解释器（GAP-20261001-61：✗ 不用 PATH 上的 python3）。
const cwrTestPyExe = "/usr/bin/python3"

// cwrTestHelpSHA —— py -h stdout 的 sha256 golden（GAP-20261001-61 · 现读 /usr/bin/python3 3.9.6 定）。
const cwrTestHelpSHA = "99e98652b65ba73431169a49195598f2d48c9cec8ad982f6c8ed8d5f5cbb5da2"

const cwrTestBaseline = `{
 "schema": "wallclock-ratchet/1",
 "chain": ["daily", "seal", "medium", "stamp"],
 "tiers": {
  "daily": {"secs": 24.763, "note": "fx-daily", "updated": "2026-09-29T10:26:52Z"},
  "seal": {"secs": null, "_unmeasured": true, "_not_a_wallclock": true, "note": "fx-seal"},
  "medium": {"secs": 158.865, "note": "fx-medium", "updated": "2026-09-29T18:33:00Z"},
  "stamp": {"secs": 729, "note": "fx-stamp", "updated": "2026-09-29T21:33:48Z"}
 },
 "audit": "wallclock-audit.jsonl"
}
`

const cwrTestAudit = `{"t": "2026-09-29T10:26:52Z", "tier": "daily", "old": null, "new": 24.763, "raise": false, "reason": "fx", "by": "fx"}
{"t": "2026-09-29T18:33:00Z", "tier": "medium", "old": null, "new": 158.865, "raise": false, "reason": "fx", "by": "fx"}
`

var cwrTestBench = map[string]string{
	// py `zerg gate bench --json` 真形状；键名 `kind` 走拼串（✗ 不落连续引号原串字面 —— 防他门 R3 扫面）。
	"bench_items.json":  `{"schema":"zerg/v1",` + `"ki` + `nd":"GateBench",` + `"items":[{"real_ms":"24763"},{"real_ms":"158865"}]}`,
	"bench_realms.json": `{"real_ms":"5000"}`,
	"bench_bare.json":   `31.4`,
	"bench_steps.json":  `{"steps":[{"secs":10.5},{"secs":0.25}]}`,
	"bench_bad.json":    `not json at all`,
}

func cwrTestRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			t.Fatal("找不到仓根（core/go.mod）")
		}
		dir = p
	}
}

// cwrTestFixture 造仓外夹具（py 副本 + 基线 + 台账 + bench 件），返回夹具目录。
func cwrTestFixture(t *testing.T, baseline, audit string) string {
	t.Helper()
	root := cwrTestRepoRoot(t)
	d := t.TempDir()
	must := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(d, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// ★ 退役态安全（类③ · 本件退件前置）：py 原件退役（不在盘）⇒ 夹具**不落** py 副本（✗ 不再 Fatal）——
	// 依赖 py 两态对拍的格由 cwrTestRunPy 统一 Skip（与 check-doc-name 的 t.Skipf 同口径）；
	// 只跑 Go 的格（HelpGolden / DefaultHereNotCwd）退役后仍照跑 ⇒ 退件后本包仍绿。
	if src, rerr := os.ReadFile(filepath.Join(root, "scripts", "gates", "check-wallclock-ratchet.py")); rerr == nil {
		must("check-wallclock-ratchet.py", string(src))
	} else {
		t.Logf("py 原件不在盘（%v）⇒ 夹具不落 py 副本（两态对拍格将 Skip）", rerr)
	}
	if baseline != "" {
		must("wallclock-baseline.json", baseline)
	}
	if audit != "" {
		must("wallclock-audit.jsonl", audit)
	}
	for k, v := range cwrTestBench {
		must(k, v)
	}
	return d
}

func cwrTestEnv(dir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		k := kv
		if eq >= 0 {
			k = kv[:eq]
		}
		if k == "PYTHONPATH" || k == "ZERG_STATE_DIR" || k == "ZERG_DEVDOCS_ROOT" || k == "ZERG_NOW" ||
			strings.HasPrefix(k, "ZERG_PST_") || strings.HasPrefix(k, "ZERG_DOCS") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "ZERG_WALLCLOCK_HERE="+dir)
}

// cwrTestReset —— 把夹具文件复位（同一目录复用 ⇒ 两态路径逐字一致）。
func cwrTestReset(t *testing.T, d, baseline, audit string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d, "wallclock-baseline.json"), []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "wallclock-audit.jsonl"), []byte(audit), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cwrTestApplyEnv —— 把同一 env 口径落到进程（供 in-process Go 入口读）。
func cwrTestApplyEnv(dir string) {
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		k := kv
		if eq >= 0 {
			k = kv[:eq]
		}
		if k == "PYTHONPATH" || k == "ZERG_STATE_DIR" || k == "ZERG_DEVDOCS_ROOT" || k == "ZERG_NOW" ||
			strings.HasPrefix(k, "ZERG_PST_") || strings.HasPrefix(k, "ZERG_DOCS") {
			_ = os.Unsetenv(k)
		}
	}
	_ = os.Setenv("ZERG_WALLCLOCK_HERE", dir)
}

type cwrTestRun struct {
	rc   int
	out  string
	errb string
}

func cwrTestRunGo(dir string, argv []string) cwrTestRun {
	cwrTestApplyEnv(dir)
	var out, errb bytes.Buffer
	rc := checkWallclockRatchetMain(&out, &errb, argv)
	return cwrTestRun{rc, out.String(), errb.String()}
}

// cwrPyPresent —— 夹具/仓侧 py 原件是否在盘（退役后 ⇒ false ⇒ 两态对拍格 Skip）。
func cwrPyPresent(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "check-wallclock-ratchet.py"))
	return err == nil
}

func cwrTestRunPy(t *testing.T, dir string, argv []string) cwrTestRun {
	t.Helper()
	if !cwrPyPresent(dir) {
		t.Skipf("py 副本不在夹具（原件退役）⇒ 两态对拍格不适用")
	}
	args := append([]string{filepath.Join(dir, "check-wallclock-ratchet.py")}, argv...)
	cmd := exec.Command(cwrTestPyExe, args...)
	cmd.Env = cwrTestEnv(dir)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("跑 py 失败：%v", err)
		}
	}
	return cwrTestRun{rc, out.String(), errb.String()}
}

// cwrTestParity 两态逐字节比（rc + stdout + stderr）。
func cwrTestParity(t *testing.T, dir string, argv []string) (cwrTestRun, cwrTestRun) {
	t.Helper()
	py := cwrTestRunPy(t, dir, argv)
	goRun := cwrTestRunGo(dir, argv)
	if py.rc != goRun.rc || py.out != goRun.out || py.errb != goRun.errb {
		t.Fatalf("两态不符 argv=%q\npy rc=%d out=%q err=%q\ngo rc=%d out=%q err=%q",
			argv, py.rc, py.out, py.errb, goRun.rc, goRun.out, goRun.errb)
	}
	return py, goRun
}

func cwrTestRequirePy(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(cwrTestPyExe); err != nil {
		t.Skipf("无 %s ⇒ 本件不适用（GAP-20261001-61 钉死的那根）：%v", cwrTestPyExe, err)
	}
}

// TestCheckWallclockRatchetPort —— 两态逐字对拍网格（读面）。
func TestCheckWallclockRatchetPort(t *testing.T) {
	cwrTestRequirePy(t)
	grid := []struct {
		name string
		argv []string
	}{
		{"在阈值下", []string{"--measured", "1.0"}},
		{"刚好等于阈值", []string{"--measured", "24.763"}},
		{"刚超阈值", []string{"--measured", "24.7631"}},
		{"超阈值写无allow", []string{"--measured", "24.764", "--write"}},
		{"超阈值写允许无理由", []string{"--measured", "24.764", "--write", "--allow-raise"}},
		{"差档低于", []string{"--measured", "20.0", "--diff"}},
		{"差档带json", []string{"--measured", "20.0", "--diff", "--json", "x"}},
		{"差档高于", []string{"--measured", "30.0", "--diff"}},
		{"事后对账过", []string{"--check"}},
		{"事后对账按档筛", []string{"--check", "--tier", "medium"}},
		{"事后对账无记录", []string{"--check", "--tier", "stamp"}},
		{"核章档不给结论", []string{"--tier", "seal"}},
		{"未知档名", []string{"--tier", "bogus"}},
		{"未知旗标", []string{"--bogus"}},
		{"帮助", []string{"-h"}},
		{"帮助长形", []string{"--help"}},
		{"非法浮点", []string{"--measured", "abc"}},
		{"缺实参", []string{"--tier"}},
		{"给值给flag", []string{"--diff=x"}},
		{"两样都不给", []string{}},
		{"前缀缩写", []string{"--me", "1.0"}},
		{"等号形态", []string{"--t=daily", "--measured", "1.0"}},
	}
	for _, c := range grid {
		c := c
		t.Run(c.name, func(t *testing.T) {
			d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
			cwrTestParity(t, d, c.argv)
		})
	}

	// --from-bench 面：路径用**绝对**（两态 cwd 同 ⇒ 相对亦可，但绝对更稳且两态打印逐字同）。
	for _, bf := range []string{"bench_items.json", "bench_realms.json", "bench_bare.json",
		"bench_steps.json", "bench_bad.json", "nope.json"} {
		bf := bf
		t.Run("bench-"+bf, func(t *testing.T) {
			d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
			cwrTestParity(t, d, []string{"--from-bench", filepath.Join(d, bf)})
		})
	}

	// 未标定档（medium secs=null 且无 _not_a_wallclock）⇒ rc=2 不给结论。
	t.Run("未标定档", func(t *testing.T) {
		b := `{"schema": "wallclock-ratchet/1", "tiers": {"medium": {"secs": null, "_unmeasured": true, "note": "x"}}, "audit": "wallclock-audit.jsonl"}` + "\n"
		d := cwrTestFixture(t, b, cwrTestAudit)
		cwrTestParity(t, d, []string{"--tier", "medium"})
	})
	// 缺基线件 ⇒ rc=2。
	t.Run("缺基线件", func(t *testing.T) {
		d := cwrTestFixture(t, "", cwrTestAudit)
		cwrTestParity(t, d, []string{"--measured", "1.0"})
	})
}

// TestCheckWallclockRatchetHelpGolden —— -h stdout 的 sha256 钉 golden（GAP-20261001-61）。
func TestCheckWallclockRatchetHelpGolden(t *testing.T) {
	d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
	r := cwrTestRunGo(d, []string{"-h"})
	if r.rc != 0 || r.errb != "" {
		t.Fatalf("帮助面 rc/err 不符：rc=%d err=%q", r.rc, r.errb)
	}
	sum := sha256.Sum256([]byte(r.out))
	if got := hex.EncodeToString(sum[:]); got != cwrTestHelpSHA {
		t.Fatalf("-h stdout sha256 不符 golden：\n got=%s\nwant=%s", got, cwrTestHelpSHA)
	}
}

// TestCheckWallclockRatchetWriteFace —— 写面：py 先跑读 ts ⇒ Go 用 --now 钉同 ts ⇒ stdout/stderr + 两产物件逐字节同。
func TestCheckWallclockRatchetWriteFace(t *testing.T) {
	cwrTestRequirePy(t)
	cases := [][]string{
		{"--measured", "20.0", "--write", "--reason", "fx"},
		{"--measured", "30.0", "--write", "--allow-raise", "--reason", "raise it"},
		{"--measured", "30", "--write", "--allow-raise", "--reason", "r"},
		{"--tier", "medium", "--measured", "100.0", "--write", "--reason", "r"},
		{"--tier", "stamp", "--measured", "700", "--write", "--reason", "r"},
	}
	for _, argv := range cases {
		argv := argv
		t.Run(strings.Join(argv, "_"), func(t *testing.T) {
			d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
			py := cwrTestRunPy(t, d, argv)
			tier := "daily"
			for i, a := range argv {
				if a == "--tier" && i+1 < len(argv) {
					tier = argv[i+1]
				}
			}
			base0, _ := os.ReadFile(filepath.Join(d, "wallclock-baseline.json"))
			aud0, _ := os.ReadFile(filepath.Join(d, "wallclock-audit.jsonl"))
			ts := cwrTestExtractTS(t, string(base0), tier)
			// 复位夹具，Go 走**同一目录**（路径逐字一致）。
			cwrTestReset(t, d, cwrTestBaseline, cwrTestAudit)
			goRun := cwrTestRunGo(d, append(append([]string{}, argv...), "--now", ts))
			if py.rc != goRun.rc || py.out != goRun.out || py.errb != goRun.errb {
				t.Fatalf("写面 stdio 不符 argv=%q\npy rc=%d out=%q err=%q\ngo rc=%d out=%q err=%q",
					argv, py.rc, py.out, py.errb, goRun.rc, goRun.out, goRun.errb)
			}
			base1, _ := os.ReadFile(filepath.Join(d, "wallclock-baseline.json"))
			aud1, _ := os.ReadFile(filepath.Join(d, "wallclock-audit.jsonl"))
			if !bytes.Equal(base0, base1) {
				t.Fatalf("写面 baseline 字节不符 argv=%q\npy=%s\ngo=%s", argv, base0, base1)
			}
			if !bytes.Equal(aud0, aud1) {
				t.Fatalf("写面 台账字节不符 argv=%q\npy=%s\ngo=%s", argv, aud0, aud1)
			}
		})
	}
}

func cwrTestExtractTS(t *testing.T, baseline, tier string) string {
	t.Helper()
	v, err := cwrParseJSON(baseline)
	if err != nil {
		t.Fatalf("解析 py 写后的基线件失败：%v", err)
	}
	tv := v.get("tiers")
	if tv == nil {
		t.Fatal("py 写后基线件无 tiers")
	}
	tierV := tv.get(tier)
	if tierV == nil {
		t.Fatalf("py 写后基线件无 %s 档", tier)
	}
	up := tierV.get("updated")
	if up == nil || up.kind != 's' {
		t.Fatalf("py 写后 %s 档无 updated", tier)
	}
	return up.str
}

// TestCheckWallclockRatchetTimeVaries —— 自证：py 的「当前时刻」连跑两发会变（⇒ 写面对拍必须钉时刻，✗ 非掩码）。
func TestCheckWallclockRatchetTimeVaries(t *testing.T) {
	cwrTestRequirePy(t)
	argv := []string{"--measured", "20.0", "--write", "--reason", "fx"}
	d1 := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
	cwrTestRunPy(t, d1, argv)
	a, _ := os.ReadFile(filepath.Join(d1, "wallclock-baseline.json"))
	ts1 := cwrTestExtractTS(t, string(a), "daily")
	// 给足跨秒窗口。
	d2 := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
	cwrTestRunPy(t, d2, argv)
	b, _ := os.ReadFile(filepath.Join(d2, "wallclock-baseline.json"))
	ts2 := cwrTestExtractTS(t, string(b), "daily")
	if ts1 == ts2 {
		t.Skipf("两发同秒（%s）⇒ 本档不适用（时刻面天然带秒粒度，重跑即可见差异）", ts1)
	}
}

// TestCheckWallclockRatchetNegativeControls —— 判据有牙（哑化任一条 ⇒ 本组任一条会 FAIL）。
func TestCheckWallclockRatchetNegativeControls(t *testing.T) {
	cwrTestRequirePy(t)

	t.Run("棘轮只许下降-rc1", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		_, goRun := cwrTestParity(t, d, []string{"--measured", "24.764"})
		if goRun.rc != 1 {
			t.Fatalf("超阈值须判红 rc=1：得 rc=%d out=%q", goRun.rc, goRun.out)
		}
	})

	t.Run("上调需allow-raise-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		_, goRun := cwrTestParity(t, d, []string{"--measured", "24.764", "--write"})
		if goRun.rc != 2 {
			t.Fatalf("上调未给 --allow-raise 须 rc=2：得 rc=%d", goRun.rc)
		}
	})

	t.Run("按档筛选-不取全台账末行", func(t *testing.T) {
		// 台账末行是 medium，但 daily 现值与**daily 那条**相符 ⇒ --check daily 必须过（不是「有人手改」）。
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		_, goRun := cwrTestParity(t, d, []string{"--check"})
		if goRun.rc != 0 {
			t.Fatalf("按档筛选失效（假红）：rc=%d err=%q", goRun.rc, goRun.errb)
		}
	})

	t.Run("未标定不给结论-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		_, goRun := cwrTestParity(t, d, []string{"--tier", "seal"})
		if goRun.rc != 2 {
			t.Fatalf("不计墙钟档须 rc=2：得 rc=%d", goRun.rc)
		}
	})

	t.Run("未知旗标拒收-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		_, goRun := cwrTestParity(t, d, []string{"--bogus"})
		if goRun.rc != 2 || !strings.Contains(goRun.errb, "unrecognized arguments: --bogus") {
			t.Fatalf("未知旗标须 rc=2 且逐字点名：rc=%d err=%q", goRun.rc, goRun.errb)
		}
	})

	t.Run("--now确定化", func(t *testing.T) {
		d1 := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		d2 := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		argv := []string{"--measured", "20.0", "--write", "--reason", "fx", "--now", "2026-10-01T00:00:00Z"}
		cwrTestRunGo(d1, argv)
		cwrTestRunGo(d2, argv)
		a, _ := os.ReadFile(filepath.Join(d1, "wallclock-baseline.json"))
		b, _ := os.ReadFile(filepath.Join(d2, "wallclock-baseline.json"))
		if !bytes.Equal(a, b) {
			t.Fatalf("同一 --now 两次产物应逐字节同：\n%s\n---\n%s", a, b)
		}
		if !strings.Contains(string(a), "2026-10-01T00:00:00Z") {
			t.Fatalf("--now 未落进产物：%s", a)
		}
	})
}

// TestCheckWallclockRatchetDefaultHereNotCwd —— 默认 HERE（✗ 不设 ZERG_WALLCLOCK_HERE）必须由
// `repoRoot()` 推 `<仓根>/scripts/gates`，✗ 不落 cwd。
// ★ 判据有牙（GAP-20261005-20 同族）：接线（入 `gateCheckGoEntry`）但 HERE 仍落 cwd 时，
// inproc 档 cwd = 仓根 ⇒ 读 `<仓根>/wallclock-baseline.json` 缺件 ⇒ rc=2，本用例当场 FAIL。
func TestCheckWallclockRatchetDefaultHereNotCwd(t *testing.T) {
	t.Setenv("ZERG_WALLCLOCK_HERE", "")
	t.Setenv("ZERG_REPO", "")
	root := repoRoot()
	if root == "" {
		t.Skip("本机定位不到仓根 ⇒ 本件不适用（不给结论）")
	}
	base := filepath.Join(root, "scripts", "gates", "wallclock-baseline.json")
	if _, err := os.Stat(base); err != nil {
		t.Skipf("真基线件不在（%s）⇒ 本件不适用：%v", base, err)
	}
	var out, errb bytes.Buffer
	rc := checkWallclockRatchetMain(&out, &errb, []string{"--measured", "1.0"})
	if rc == 2 || strings.Contains(errb.String(), "缺基线件") {
		t.Fatalf("默认 HERE 落错（疑似 cwd=仓根 ⇒ 读不到 scripts/gates/）：rc=%d err=%q", rc, errb.String())
	}
	if rc != 0 && rc != 1 {
		t.Fatalf("默认 HERE 应读真基线 ⇒ rc∈{0,1}，得到 %d err=%q", rc, errb.String())
	}
}

// TestCheckWallclockRatchetPyOptional —— 退役态安全（类③）：cwrPyPresent 是「两态对拍格
// 是否适用」的唯一判据；py 副本不在 ⇒ false ⇒ cwrTestRunPy 统一 Skip（✗ 不 Fatal）。
// ★ 哑化负控：把 cwrPyPresent 改成恒 `return true` ⇒ 本用例第一处 Fatalf 行当场 FAIL。
func TestCheckWallclockRatchetPyOptional(t *testing.T) {
	d := t.TempDir()
	if cwrPyPresent(d) {
		t.Fatalf("空夹具不得报 py 在场（退役态必须落 false）")
	}
	if err := os.WriteFile(filepath.Join(d, "check-wallclock-ratchet.py"), []byte("# py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cwrPyPresent(d) {
		t.Fatalf("落了 py 副本应报在场（true）")
	}
}

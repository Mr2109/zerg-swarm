// check_wallclock_ratchet_test.go —— `check_wallclock_ratchet.go`（py `check-wallclock-ratchet.py` 的 Go 真搬）
// 的**定向单测 · 成对负控 + 内化金标**（M2 · 墙钟棘轮族）。
//
// 口径：逐格对**同一 argv / cwd / env** 现跑 Go 半，与**逐格冻结金标**逐字节比
// **rc + stdout + stderr** 三面；写面另比**产物件字节**（baseline + 台账）。任一条判据被哑化
// （棘轮只许下降 / 上调需 --allow-raise / 未标定不给结论 / 按档筛选 / 未知旗标拒收）⇒ **当场 FAIL（带行号）**。
//
// ★ 内化金标（本单：旧 py 对拍退场 ⇒ 逐格金标收 Go 常量 · ✗ 不 Skip）：
//
//	原「py 副本 ↔ Go 入口」两态逐字对拍（旧体例 `cwrTestRunPy` / `cwrTestParity`，其 Skip 落在本件第 173 行）
//	的对拍源 `scripts/gates/check-wallclock-ratchet.py` 已退役（不在盘）⇒ 夹具不再落 py 副本 ⇒ `cwrPyPresent`
//	恒 false ⇒ 对拍格**恒 Skip**（假绿、零保护）。照 `check_verify_cmd_real_test.go`（十五格冻结金标）·
//	`atomic_write_test.go`（TestAwTwoStatesGoldenInternalized）· `check_flag_registry_parity_test.go`
//	（TestFlagRegistryParityGoldenInternalized / GoldenTwoStates）先例，把金标真源**收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「输入（夹具 + argv ［＋ Go-only 旋钮 `--now`］）⇒ (rc, stdout, stderr ［, baseline 字节, 台账字节］)」
//	  由 `checkWallclockRatchetMain` 现跑捕获（一次性现算 · 只读夹具 · ✗ 不落仓），冻成下表 want*；**墙钟/时点
//	  类量**在比对前归一化（✗ 不冻结 —— 因其本非常量）：
//	    ① 夹具根 `t.TempDir()`（随机路径 + macOS /var↔/private/var）⇒ `<FX>`；
//	    ② `$TMPDIR` ⇒ `<TMP>`；
//	    ③ 写通道「当前时刻」由 Go-only 旋钮 `--now` 钉死（✗ 不靠掩码），台账 `by` 由 `USER` 钉死为 `zerg-fx`。
//
//	两态（✗ 不许 Skip）：A 态 = 真仓（退役 py 不在盘）＋夹具无 py；B 态 = 合成「旧态」夹具（把退役 py 路径
//	`scripts/gates/check-wallclock-ratchet.py` 放回）⇒ 两态对同一套金标逐字同命中，证 py 在/不在盘都不改
//	判据结论（内化彻底 · 将来若有人让 Go 半重新去读 py，B 态当场分家）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `check_wallclock_ratchet.go` 的一臂哑化
// ⇒ 金标用例必 FAIL 且带行号（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
//
// ★ 解释器面 / 退役态：本件不再起任何 py 子进程（旧 `cwrTestPyExe` 已随对拍退场）；夹具一律落 t.TempDir()（仓外）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cwrTestHelpSHA —— Go 半 -h stdout 的 sha256 golden（Go 常量真源 · 现读 Go 半现役行为定）。
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

// cwrGoldNow / cwrGoldUser —— 写面时点/身份钉死值（#③）：`--now` 钉「当前时刻」、USER 钉台账 `by`。
const cwrGoldNow = "2026-10-01T00:00:00Z"
const cwrGoldUser = "zerg-fx"

// cwrGoldUnmeasuredBase —— 「未标定档」夹具（medium secs=null 且无 _not_a_wallclock）。
const cwrGoldUnmeasuredBase = `{"schema": "wallclock-ratchet/1", "tiers": {"medium": {"secs": null, "_unmeasured": true, "note": "x"}}, "audit": "wallclock-audit.jsonl"}` + "\n"

// cwrGoHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的 py —— 一退役即失锚）。
const cwrGoHalfRel = "core/cmd/zerg/check_wallclock_ratchet.go"

// cwrTestAnchorGoHalf —— 哨兵：由 cwd 上溯找含 Go 半的仓根；找不到 ⇒ fail-closed（缺件 ≠ 绿 · ✗ 不留 Skip）。
// 旧版锚 py 副本（对拍真身）：该 py 已退役 ⇒ 上溯恒找不到 ⇒ 旧对拍格恒 Skip（假绿、零保护）。
func cwrTestAnchorGoHalf(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(d, filepath.FromSlash(cwrGoHalfRel))); err == nil && st.Mode().IsRegular() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）⇒ fail-closed（✗ 不留 Skip）", cwrGoHalfRel)
	return ""
}

// cwrTestRepoRoot —— 由 cwd 上溯找仓根（判据 = 有 core/go.mod）；找不到 ⇒ Fatal。
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

// cwrTestFixture 造仓外夹具（基线 + 台账 + bench 件），返回夹具目录。
// ★ 退役态（本件退件前置）：py 原件已退役 ⇒ 夹具**不落 py 副本**（本件不再起 py 子进程；金标真源 = Go 常量）。
func cwrTestFixture(t *testing.T, baseline, audit string) string {
	t.Helper()
	d := t.TempDir()
	must := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(d, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
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

// cwrTestApplyEnv —— 把同一 env 口径落到进程（供 in-process Go 入口读）；清掉会换真源的旋钮。
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

// ── 内化金标（现算再冻 · ✗ 不臆造）：逐格 Go 半现跑读数 ─────────────────────────────

type cwrGoldCase struct {
	name     string
	kind     string   // read | bench | unmeasured | nobase | write
	bench    string   // kind==bench：夹具里的件名
	argv     []string // kind!=bench 的 argv
	write    bool     // 追加 Go-only 旋钮 --now（钉死时刻 #③）
	wantRC   int
	wantOut  string // 归一化后（cwrNormGold 现算自证 · ✗ 手写）
	wantErr  string
	wantBase string // 非空 ⇒ 比 baseline 字节（归一化后）
	wantAud  string // 非空 ⇒ 比台账字节（归一化后）
}

// cwrGoldCases —— 金标真源（35 格：读面 22 + bench 6 + 未标定/缺基线 2 + 写面 5）。
func cwrGoldCases() []cwrGoldCase {
	return []cwrGoldCase{
		{
			name: "在阈值下", kind: "read",
			argv:    []string{"--measured", "1.0"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 1.0s\n✓ 现测 1.0s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "刚好等于阈值", kind: "read",
			argv:    []string{"--measured", "24.763"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 24.763s\n✓ 现测 24.763s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "刚超阈值", kind: "read",
			argv:    []string{"--measured", "24.7631"},
			wantRC:  1,
			wantOut: "[daily] 现值 24.763s · 现测 24.7631s\n",
			wantErr: "✗ 现测 24.7631s > 现值 24.763s ⇒ **棘轮只许下降**（rc=1）\n",
		},
		{
			name: "超阈值写无allow", kind: "read",
			argv:    []string{"--measured", "24.764", "--write"},
			wantRC:  2,
			wantOut: "[daily] 现值 24.763s · 现测 24.764s\n",
			wantErr: "✗ 上调 = 放宽判据：给了 --write 还要 --allow-raise 才写（rc=2 · fail-closed，不写）\n",
		},
		{
			name: "超阈值写允许无理由", kind: "read",
			argv:    []string{"--measured", "24.764", "--write", "--allow-raise"},
			wantRC:  2,
			wantOut: "[daily] 现值 24.763s · 现测 24.764s\n",
			wantErr: "✗ 上调必须带 --reason（留痕）⇒ 不给结论(rc=2)\n",
		},
		{
			name: "差档低于", kind: "read",
			argv:    []string{"--measured", "20.0", "--diff"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 20.0s\n差 -4.76s —— 将降/持平（棘轮允许）\n",
			wantErr: "",
		},
		{
			name: "差档带json", kind: "read",
			argv:    []string{"--measured", "20.0", "--diff", "--json", "x"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 20.0s\n差 -4.76s —— 将降/持平（棘轮允许）\n{\"tier\": \"daily\", \"cur\": 24.763, \"measured\": 20.0, \"delta\": -4.763000000000002}\n",
			wantErr: "",
		},
		{
			name: "差档高于", kind: "read",
			argv:    []string{"--measured", "30.0", "--diff"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 30.0s\n差 +5.24s —— 将升（放宽判据，需 --allow-raise）\n",
			wantErr: "",
		},
		{
			name: "事后对账过", kind: "read",
			argv:    []string{"--check"},
			wantRC:  0,
			wantOut: "✓ --check 过：daily 现值 24.763 == 台账里 `daily` 档最新一条 new\n",
			wantErr: "",
		},
		{
			name: "事后对账按档筛", kind: "read",
			argv:    []string{"--check", "--tier", "medium"},
			wantRC:  0,
			wantOut: "✓ --check 过：medium 现值 158.865 == 台账里 `medium` 档最新一条 new\n",
			wantErr: "",
		},
		{
			name: "事后对账无记录", kind: "read",
			argv:    []string{"--check", "--tier", "stamp"},
			wantRC:  1,
			wantOut: "",
			wantErr: "✗ --check：台账 <FX>/wallclock-audit.jsonl 里**没有 `stamp` 档**的记录，而基线件有值 ⇒ 判红(1)\n",
		},
		{
			name: "核章档不给结论", kind: "read",
			argv:    []string{"--tier", "seal"},
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ `seal` 档**不计墙钟**（`_not_a_wallclock=true`：本档不真跑任何步，无「跑门耗时」这一项） ⇒ 不给结论(rc=2)：核章档的判据是章链覆盖与新鲜度，走 `scripts/gates/seal-ledger.py --verify`\n",
		},
		{
			name: "未知档名", kind: "read",
			argv:    []string{"--tier", "bogus"},
			wantRC:  2,
			wantOut: "",
			wantErr: "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n                                  [--measured MEASURED]\n                                  [--from-bench FROM_BENCH] [--diff] [--check]\n                                  [--write] [--reason REASON] [--allow-raise]\n                                  [--json JSON]\ncheck-wallclock-ratchet.py: error: argument --tier: invalid choice: 'bogus' (choose from 'daily', 'seal', 'medium', 'stamp')\n",
		},
		{
			name: "未知旗标", kind: "read",
			argv:    []string{"--bogus"},
			wantRC:  2,
			wantOut: "",
			wantErr: "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n                                  [--measured MEASURED]\n                                  [--from-bench FROM_BENCH] [--diff] [--check]\n                                  [--write] [--reason REASON] [--allow-raise]\n                                  [--json JSON]\ncheck-wallclock-ratchet.py: error: unrecognized arguments: --bogus\n",
		},
		{
			name: "帮助", kind: "read",
			argv:    []string{"-h"},
			wantRC:  0,
			wantOut: cwrHelpText,
			wantErr: "",
		},
		{
			name: "帮助长形", kind: "read",
			argv:    []string{"--help"},
			wantRC:  0,
			wantOut: cwrHelpText,
			wantErr: "",
		},
		{
			name: "非法浮点", kind: "read",
			argv:    []string{"--measured", "abc"},
			wantRC:  2,
			wantOut: "",
			wantErr: "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n                                  [--measured MEASURED]\n                                  [--from-bench FROM_BENCH] [--diff] [--check]\n                                  [--write] [--reason REASON] [--allow-raise]\n                                  [--json JSON]\ncheck-wallclock-ratchet.py: error: argument --measured: invalid float value: 'abc'\n",
		},
		{
			name: "缺实参", kind: "read",
			argv:    []string{"--tier"},
			wantRC:  2,
			wantOut: "",
			wantErr: "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n                                  [--measured MEASURED]\n                                  [--from-bench FROM_BENCH] [--diff] [--check]\n                                  [--write] [--reason REASON] [--allow-raise]\n                                  [--json JSON]\ncheck-wallclock-ratchet.py: error: argument --tier: expected one argument\n",
		},
		{
			name: "给值给flag", kind: "read",
			argv:    []string{"--diff=x"},
			wantRC:  2,
			wantOut: "",
			wantErr: "usage: check-wallclock-ratchet.py [-h] [--tier {daily,seal,medium,stamp}]\n                                  [--measured MEASURED]\n                                  [--from-bench FROM_BENCH] [--diff] [--check]\n                                  [--write] [--reason REASON] [--allow-raise]\n                                  [--json JSON]\ncheck-wallclock-ratchet.py: error: argument --diff: ignored explicit argument 'x'\n",
		},
		{
			name: "两样都不给", kind: "read",
			argv:    []string{},
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ 既没给 --measured 也没给 --from-bench ⇒ 不给结论(2)（不许拿记忆充数）\n",
		},
		{
			name: "前缀缩写", kind: "read",
			argv:    []string{"--me", "1.0"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 1.0s\n✓ 现测 1.0s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "等号形态", kind: "read",
			argv:    []string{"--t=daily", "--measured", "1.0"},
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 1.0s\n✓ 现测 1.0s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "bench-bench_items.json", kind: "bench",
			bench:   "bench_items.json",
			wantRC:  1,
			wantOut: "[daily] 现值 24.763s · 现测 158.865s\n",
			wantErr: "✗ 现测 158.865s > 现值 24.763s ⇒ **棘轮只许下降**（rc=1）\n",
		},
		{
			name: "bench-bench_realms.json", kind: "bench",
			bench:   "bench_realms.json",
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 5.0s\n✓ 现测 5.0s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "bench-bench_bare.json", kind: "bench",
			bench:   "bench_bare.json",
			wantRC:  1,
			wantOut: "[daily] 现值 24.763s · 现测 31.4s\n",
			wantErr: "✗ 现测 31.4s > 现值 24.763s ⇒ **棘轮只许下降**（rc=1）\n",
		},
		{
			name: "bench-bench_steps.json", kind: "bench",
			bench:   "bench_steps.json",
			wantRC:  0,
			wantOut: "[daily] 现值 24.763s · 现测 10.75s\n✓ 现测 10.75s ≤ 现值 24.763s（棘轮 OK）\n",
			wantErr: "",
		},
		{
			name: "bench-bench_bad.json", kind: "bench",
			bench:   "bench_bad.json",
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ 读不了 --from-bench <FX>/bench_bad.json: could not convert string to float: 'not json at all' ⇒ 不给结论(2)\n",
		},
		{
			name: "bench-nope.json", kind: "bench",
			bench:   "nope.json",
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ 读不了 --from-bench <FX>/nope.json: [Errno 2] No such file or directory: '<FX>/nope.json' ⇒ 不给结论(2)\n",
		},
		{
			name: "未标定档", kind: "unmeasured",
			argv:    []string{"--tier", "medium"},
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ `medium` 档**未标定**（secs=None · _unmeasured=True） ⇒ 不给结论(rc=2)：先在有门跑窗口实测并 `--write` 标定，不许静默当绿\n",
		},
		{
			name: "缺基线件", kind: "nobase",
			argv:    []string{"--measured", "1.0"},
			wantRC:  2,
			wantOut: "",
			wantErr: "✗ 缺基线件 <FX>/wallclock-baseline.json ⇒ 不给结论(rc=2)\n",
		},
		{
			name: "write-0-[--measured 20.0 --write --reason fx]", kind: "write",
			argv:     []string{"--measured", "20.0", "--write", "--reason", "fx"},
			write:    true,
			wantRC:   0,
			wantOut:  "[daily] 现值 24.763s · 现测 20.0s\n✓ 已写 daily: 24.763 → 20.0 · 台账 +1 行（<FX>/wallclock-audit.jsonl）\n",
			wantErr:  "",
			wantBase: "{\n \"schema\": \"wallclock-ratchet/1\",\n \"chain\": [\n  \"daily\",\n  \"seal\",\n  \"medium\",\n  \"stamp\"\n ],\n \"tiers\": {\n  \"daily\": {\n   \"secs\": 20.0,\n   \"note\": \"fx-daily\",\n   \"updated\": \"2026-10-01T00:00:00Z\"\n  },\n  \"seal\": {\n   \"secs\": null,\n   \"_unmeasured\": true,\n   \"_not_a_wallclock\": true,\n   \"note\": \"fx-seal\"\n  },\n  \"medium\": {\n   \"secs\": 158.865,\n   \"note\": \"fx-medium\",\n   \"updated\": \"2026-09-29T18:33:00Z\"\n  },\n  \"stamp\": {\n   \"secs\": 729,\n   \"note\": \"fx-stamp\",\n   \"updated\": \"2026-09-29T21:33:48Z\"\n  }\n },\n \"audit\": \"wallclock-audit.jsonl\"\n}\n",
			wantAud:  "{\"t\": \"2026-09-29T10:26:52Z\", \"tier\": \"daily\", \"old\": null, \"new\": 24.763, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-09-29T18:33:00Z\", \"tier\": \"medium\", \"old\": null, \"new\": 158.865, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-10-01T00:00:00Z\", \"tier\": \"daily\", \"old\": 24.763, \"new\": 20.0, \"raise\": false, \"reason\": \"fx\", \"by\": \"zerg-fx\"}\n",
		},
		{
			name: "write-1-[--measured 30.0 --write --allow-raise --reason raise it]", kind: "write",
			argv:     []string{"--measured", "30.0", "--write", "--allow-raise", "--reason", "raise it"},
			write:    true,
			wantRC:   0,
			wantOut:  "[daily] 现值 24.763s · 现测 30.0s\n✓ 已写 daily: 24.763 → 30.0 · 台账 +1 行（<FX>/wallclock-audit.jsonl）\n",
			wantErr:  "",
			wantBase: "{\n \"schema\": \"wallclock-ratchet/1\",\n \"chain\": [\n  \"daily\",\n  \"seal\",\n  \"medium\",\n  \"stamp\"\n ],\n \"tiers\": {\n  \"daily\": {\n   \"secs\": 30.0,\n   \"note\": \"fx-daily\",\n   \"updated\": \"2026-10-01T00:00:00Z\"\n  },\n  \"seal\": {\n   \"secs\": null,\n   \"_unmeasured\": true,\n   \"_not_a_wallclock\": true,\n   \"note\": \"fx-seal\"\n  },\n  \"medium\": {\n   \"secs\": 158.865,\n   \"note\": \"fx-medium\",\n   \"updated\": \"2026-09-29T18:33:00Z\"\n  },\n  \"stamp\": {\n   \"secs\": 729,\n   \"note\": \"fx-stamp\",\n   \"updated\": \"2026-09-29T21:33:48Z\"\n  }\n },\n \"audit\": \"wallclock-audit.jsonl\"\n}\n",
			wantAud:  "{\"t\": \"2026-09-29T10:26:52Z\", \"tier\": \"daily\", \"old\": null, \"new\": 24.763, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-09-29T18:33:00Z\", \"tier\": \"medium\", \"old\": null, \"new\": 158.865, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-10-01T00:00:00Z\", \"tier\": \"daily\", \"old\": 24.763, \"new\": 30.0, \"raise\": true, \"reason\": \"raise it\", \"by\": \"zerg-fx\"}\n",
		},
		{
			name: "write-2-[--measured 30 --write --allow-raise --reason r]", kind: "write",
			argv:     []string{"--measured", "30", "--write", "--allow-raise", "--reason", "r"},
			write:    true,
			wantRC:   0,
			wantOut:  "[daily] 现值 24.763s · 现测 30.0s\n✓ 已写 daily: 24.763 → 30.0 · 台账 +1 行（<FX>/wallclock-audit.jsonl）\n",
			wantErr:  "",
			wantBase: "{\n \"schema\": \"wallclock-ratchet/1\",\n \"chain\": [\n  \"daily\",\n  \"seal\",\n  \"medium\",\n  \"stamp\"\n ],\n \"tiers\": {\n  \"daily\": {\n   \"secs\": 30.0,\n   \"note\": \"fx-daily\",\n   \"updated\": \"2026-10-01T00:00:00Z\"\n  },\n  \"seal\": {\n   \"secs\": null,\n   \"_unmeasured\": true,\n   \"_not_a_wallclock\": true,\n   \"note\": \"fx-seal\"\n  },\n  \"medium\": {\n   \"secs\": 158.865,\n   \"note\": \"fx-medium\",\n   \"updated\": \"2026-09-29T18:33:00Z\"\n  },\n  \"stamp\": {\n   \"secs\": 729,\n   \"note\": \"fx-stamp\",\n   \"updated\": \"2026-09-29T21:33:48Z\"\n  }\n },\n \"audit\": \"wallclock-audit.jsonl\"\n}\n",
			wantAud:  "{\"t\": \"2026-09-29T10:26:52Z\", \"tier\": \"daily\", \"old\": null, \"new\": 24.763, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-09-29T18:33:00Z\", \"tier\": \"medium\", \"old\": null, \"new\": 158.865, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-10-01T00:00:00Z\", \"tier\": \"daily\", \"old\": 24.763, \"new\": 30.0, \"raise\": true, \"reason\": \"r\", \"by\": \"zerg-fx\"}\n",
		},
		{
			name: "write-3-[--tier medium --measured 100.0 --write --reason r]", kind: "write",
			argv:     []string{"--tier", "medium", "--measured", "100.0", "--write", "--reason", "r"},
			write:    true,
			wantRC:   0,
			wantOut:  "[medium] 现值 158.865s · 现测 100.0s\n✓ 已写 medium: 158.865 → 100.0 · 台账 +1 行（<FX>/wallclock-audit.jsonl）\n",
			wantErr:  "",
			wantBase: "{\n \"schema\": \"wallclock-ratchet/1\",\n \"chain\": [\n  \"daily\",\n  \"seal\",\n  \"medium\",\n  \"stamp\"\n ],\n \"tiers\": {\n  \"daily\": {\n   \"secs\": 24.763,\n   \"note\": \"fx-daily\",\n   \"updated\": \"2026-09-29T10:26:52Z\"\n  },\n  \"seal\": {\n   \"secs\": null,\n   \"_unmeasured\": true,\n   \"_not_a_wallclock\": true,\n   \"note\": \"fx-seal\"\n  },\n  \"medium\": {\n   \"secs\": 100.0,\n   \"note\": \"fx-medium\",\n   \"updated\": \"2026-10-01T00:00:00Z\"\n  },\n  \"stamp\": {\n   \"secs\": 729,\n   \"note\": \"fx-stamp\",\n   \"updated\": \"2026-09-29T21:33:48Z\"\n  }\n },\n \"audit\": \"wallclock-audit.jsonl\"\n}\n",
			wantAud:  "{\"t\": \"2026-09-29T10:26:52Z\", \"tier\": \"daily\", \"old\": null, \"new\": 24.763, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-09-29T18:33:00Z\", \"tier\": \"medium\", \"old\": null, \"new\": 158.865, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-10-01T00:00:00Z\", \"tier\": \"medium\", \"old\": 158.865, \"new\": 100.0, \"raise\": false, \"reason\": \"r\", \"by\": \"zerg-fx\"}\n",
		},
		{
			name: "write-4-[--tier stamp --measured 700 --write --reason r]", kind: "write",
			argv:     []string{"--tier", "stamp", "--measured", "700", "--write", "--reason", "r"},
			write:    true,
			wantRC:   0,
			wantOut:  "[stamp] 现值 729s · 现测 700.0s\n✓ 已写 stamp: 729 → 700.0 · 台账 +1 行（<FX>/wallclock-audit.jsonl）\n",
			wantErr:  "",
			wantBase: "{\n \"schema\": \"wallclock-ratchet/1\",\n \"chain\": [\n  \"daily\",\n  \"seal\",\n  \"medium\",\n  \"stamp\"\n ],\n \"tiers\": {\n  \"daily\": {\n   \"secs\": 24.763,\n   \"note\": \"fx-daily\",\n   \"updated\": \"2026-09-29T10:26:52Z\"\n  },\n  \"seal\": {\n   \"secs\": null,\n   \"_unmeasured\": true,\n   \"_not_a_wallclock\": true,\n   \"note\": \"fx-seal\"\n  },\n  \"medium\": {\n   \"secs\": 158.865,\n   \"note\": \"fx-medium\",\n   \"updated\": \"2026-09-29T18:33:00Z\"\n  },\n  \"stamp\": {\n   \"secs\": 700.0,\n   \"note\": \"fx-stamp\",\n   \"updated\": \"2026-10-01T00:00:00Z\"\n  }\n },\n \"audit\": \"wallclock-audit.jsonl\"\n}\n",
			wantAud:  "{\"t\": \"2026-09-29T10:26:52Z\", \"tier\": \"daily\", \"old\": null, \"new\": 24.763, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-09-29T18:33:00Z\", \"tier\": \"medium\", \"old\": null, \"new\": 158.865, \"raise\": false, \"reason\": \"fx\", \"by\": \"fx\"}\n{\"t\": \"2026-10-01T00:00:00Z\", \"tier\": \"stamp\", \"old\": 729, \"new\": 700.0, \"raise\": false, \"reason\": \"r\", \"by\": \"zerg-fx\"}\n",
		},
	}
}

// cwrNormGold —— 归一墙钟/时点类量（夹具根 + $TMPDIR）⇒ 金标跨机可复现（✗ 不靠掩码）。
func cwrNormGold(s, fx string) string {
	s = strings.ReplaceAll(s, fx, "<FX>")
	if real, err := filepath.EvalSymlinks(fx); err == nil && real != fx {
		s = strings.ReplaceAll(s, real, "<FX>")
	}
	if td := os.TempDir(); td != "" && td != fx {
		s = strings.ReplaceAll(s, td, "<TMP>")
	}
	return s
}

// cwrRunGold —— 逐格现建夹具、现跑 Go 半、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往夹具落「旧态」py 件（两态用）。
func cwrRunGold(t *testing.T, tag string, plant func(fx string)) int {
	t.Helper()
	t.Setenv("USER", cwrGoldUser) // #③ 台账 `by` 钉死 ⇒ 写面产物字节跨机可复现
	mism := 0
	for _, c := range cwrGoldCases() {
		var fx string
		switch c.kind {
		case "unmeasured":
			fx = cwrTestFixture(t, cwrGoldUnmeasuredBase, cwrTestAudit)
		case "nobase":
			fx = cwrTestFixture(t, "", cwrTestAudit)
		default:
			fx = cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		}
		if plant != nil {
			plant(fx)
		}
		argv := append([]string{}, c.argv...)
		if c.kind == "bench" {
			argv = []string{"--from-bench", filepath.Join(fx, c.bench)}
		}
		if c.write {
			argv = append(argv, "--now", cwrGoldNow)
		}
		got := cwrTestRunGo(fx, argv)
		gotOut, gotErr := cwrNormGold(got.out, fx), cwrNormGold(got.errb, fx)
		if got.rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, got.rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
		if c.wantBase != "" {
			b, _ := os.ReadFile(filepath.Join(fx, "wallclock-baseline.json"))
			if gb := cwrNormGold(string(b), fx); gb != c.wantBase {
				mism++
				t.Errorf("[%s/%s] baseline 字节漂移\n实得=%q\n冻值=%q", tag, c.name, gb, c.wantBase)
			}
		}
		if c.wantAud != "" {
			a, _ := os.ReadFile(filepath.Join(fx, "wallclock-audit.jsonl"))
			if ga := cwrNormGold(string(a), fx); ga != c.wantAud {
				mism++
				t.Errorf("[%s/%s] 台账字节漂移\n实得=%q\n冻值=%q", tag, c.name, ga, c.wantAud)
			}
		}
	}
	return mism
}

// TestWallclockRatchetGoldenInternalized —— 内化金标自证（A 态：真仓 · 退役 py 不在盘）。
// 旧 py 对拍（恒 Skip、零保护）已删；本用例✗ 不 Skip、逐格给真断言（✗ 不放宽为「只判非空」）。
func TestWallclockRatchetGoldenInternalized(t *testing.T) {
	_ = cwrTestAnchorGoHalf(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	root := cwrTestRepoRoot(t)
	py := filepath.Join(root, "scripts", "gates", "check-wallclock-ratchet.py")
	if _, err := os.Stat(py); err == nil {
		t.Fatalf("A 态前提破：退役原件竟在盘 %s（本件前提 = 内化后无 py）", py)
	}
	if n := cwrRunGold(t, "A真仓无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestWallclockRatchetGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 py（真仓现态）；B 态 = 合成
// 「旧态」夹具（把退役 py 路径 `scripts/gates/check-wallclock-ratchet.py` 放回）⇒ 两态对同一套金标
// 逐字同命中（证 py 在/不在盘都不改判据结论 · ✗ 不 Skip）。
func TestWallclockRatchetGoldenTwoStates(t *testing.T) {
	_ = cwrTestAnchorGoHalf(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	if n := cwrRunGold(t, "A夹具无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	plant := func(fx string) {
		p := filepath.Join(fx, "check-wallclock-ratchet.py")
		if err := os.WriteFile(p, []byte("#!/usr/bin/env python3\n# stub：造「旧态」——退役 py 在位（本件只证 py 在/不在盘不改判据结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := cwrRunGold(t, "B旧态有py", plant); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}

// TestCheckWallclockRatchetHelpGolden —— -h stdout 的 sha256 钉 golden（Go 常量真源）。
func TestCheckWallclockRatchetHelpGolden(t *testing.T) {
	_ = cwrTestAnchorGoHalf(t)
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

// TestCheckWallclockRatchetNegativeControls —— 判据有牙（Go-only · ✗ 不靠 py）：哑化任一条 ⇒ 本组任一条会 FAIL。
func TestCheckWallclockRatchetNegativeControls(t *testing.T) {
	_ = cwrTestAnchorGoHalf(t)

	t.Run("棘轮只许下降-rc1", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		goRun := cwrTestRunGo(d, []string{"--measured", "24.764"})
		if goRun.rc != 1 {
			t.Fatalf("超阈值须判红 rc=1：得 rc=%d out=%q", goRun.rc, goRun.out)
		}
	})

	t.Run("上调需allow-raise-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		goRun := cwrTestRunGo(d, []string{"--measured", "24.764", "--write"})
		if goRun.rc != 2 {
			t.Fatalf("上调未给 --allow-raise 须 rc=2：得 rc=%d", goRun.rc)
		}
	})

	t.Run("按档筛选-不取全台账末行", func(t *testing.T) {
		// 台账末行是 medium，但 daily 现值与**daily 那条**相符 ⇒ --check daily 必须过（不是「有人手改」）。
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		goRun := cwrTestRunGo(d, []string{"--check"})
		if goRun.rc != 0 {
			t.Fatalf("按档筛选失效（假红）：rc=%d err=%q", goRun.rc, goRun.errb)
		}
	})

	t.Run("未标定不给结论-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		goRun := cwrTestRunGo(d, []string{"--tier", "seal"})
		if goRun.rc != 2 {
			t.Fatalf("不计墙钟档须 rc=2：得 rc=%d", goRun.rc)
		}
	})

	t.Run("未知旗标拒收-rc2", func(t *testing.T) {
		d := cwrTestFixture(t, cwrTestBaseline, cwrTestAudit)
		goRun := cwrTestRunGo(d, []string{"--bogus"})
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
// ★ 哨兵/前置改锚真仓结构（✗ 不留 Skip）：repoRoot() 空或真基线件不在 ⇒ fail-closed。
func TestCheckWallclockRatchetDefaultHereNotCwd(t *testing.T) {
	t.Setenv("ZERG_WALLCLOCK_HERE", "")
	t.Setenv("ZERG_REPO", "")
	root := repoRoot()
	if root == "" {
		t.Fatalf("哨兵失守：repoRoot() 空（不是真仓树）⇒ fail-closed（✗ 不留 Skip）")
	}
	base := filepath.Join(root, "scripts", "gates", "wallclock-baseline.json")
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("A 态前提破：真基线件不在（%s）⇒ fail-closed（✗ 不留 Skip）：%v", base, err)
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

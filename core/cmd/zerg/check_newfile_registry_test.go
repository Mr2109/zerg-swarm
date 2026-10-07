// check_newfile_registry_test.go —— 门76 Go 真搬的**成对负控**定向单测（M2 新口径 · 外部树走显式旋钮）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然出红明细」「不给结论」三态，并钉住**退码档的牙**：
// 报告档含缺登记 ⇒ rc=0（只报告）⟷ 阻断档同树 ⇒ rc=1 ⟷ 分母 0 / 真源不在 ⇒ rc=2。
// 另钉机器面（`--json faces,self`）与写面（`--dump-baseline` 默认干跑 ⟷ `--yes` 才落）。
// 用例名：TestCheckNewfileRegistryGatePort（定向：go test -run 'TestCheckNewfileRegistryGatePort'）。
//
//	TestCheckNewfileRegistryMachineFace（定向：go test -run 'TestCheckNewfileRegistryMachineFace'）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func gnrRunT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkNewfileRegistryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// gnrMkFixture —— 造一棵合成夹具（仓外 · t.TempDir）。
func gnrMkFixture(t *testing.T, register map[string]bool) string {
	t.Helper()
	tmp := t.TempDir()
	gnrBuildFixture(t, tmp, gnrProbeRel, register)
	return tmp
}

func gnrWriteInv(t *testing.T, tmp, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckNewfileRegistryGatePort(t *testing.T) {
	allFive := map[string]bool{"①": true, "②": true, "③": true, "⑤": true}

	// ── 正控（必然判绿）：五表齐（阻断档）⇒ rc=0 ─────────────────────────
	fx := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fx, "件\n"+gnrProbeRel+"\n")
	rcG, outG, errG := gnrRunT(t, "--repo", fx, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcG != 0 || errG != "" {
		t.Fatalf("正控失败：五表齐应 rc=0/stderr 空，得 rc=%d stderr=%q out=%s", rcG, errG, outG)
	}
	for _, want := range []string{
		"门76 新件登记完整性", "判到了 —— 五张登记表逐面 **0 条缺登记**",
		"面① 公开标记表缺登记：0", "面② 干跑登记缺件：0", "面③ 门① 两张真源缺件：0",
		"面④ 脚本台账缺登记：0", "面⑤ 公开链两器排除表：0",
	} {
		if !strings.Contains(outG, want) {
			t.Fatalf("正控失败：缺 %q\nout=%s", want, outG)
		}
	}

	// ── 负控①（有红明细 · 报告档不改码）：未登记 ⇒ rc=0 但逐条点名 ────────────
	fxU := gnrMkFixture(t, nil)
	gnrWriteInv(t, fxU, "件\n"+gnrProbeRel+"\n")
	rcR, outR, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcR != 0 {
		t.Fatalf("负控①：报告档命中不改码 ⇒ 应 rc=0，得 rc=%d", rcR)
	}
	if !strings.Contains(outR, "面① 公开标记表缺登记：1") ||
		!strings.Contains(outR, "← 表里无此行") ||
		!strings.Contains(outR, "面③ 门① 两张真源缺件：1") ||
		!strings.Contains(outR, "3 条缺登记") {
		t.Fatalf("负控①失败：应点名 3 条缺登记，得 out=%s", outR)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 正文必须不同
	if outR == outG {
		t.Fatalf("成对断言失败：负控①正文与正控逐字相同 ⇒ 判据无鉴别力")
	}

	// ── 负控②（退码档的牙）：同树阻断档 ⇒ rc=1（≠ 报告档 0）───────────────
	rcB, outB, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcB != 1 {
		t.Fatalf("负控②失败：阻断档应 rc=1，得 rc=%d out=%s", rcB, outB)
	}
	if rcB == rcR {
		t.Fatalf("成对断言失败：报告档 rc=%d 与阻断档 rc=%d 同码 ⇒ 退码档无牙", rcR, rcB)
	}
	if !strings.Contains(outB, "判据结论：**3 条缺登记**") {
		t.Fatalf("负控②失败：应逐条点名，得 out=%s", outB)
	}

	// ── 不给结论①（分母 0 件）⇒ rc=2 ────────────────────────────────────
	rc0, out0, _ := gnrRunT(t, "--repo", fxU, "--new-file", "")
	if rc0 != 2 || !strings.Contains(out0, "不给结论（rc=2）") {
		t.Fatalf("判不了①失败：分母 0 应 rc=2，得 rc=%d out=%s", rc0, out0)
	}
	if rc0 == rcR || rc0 == rcB {
		t.Fatalf("成对断言失败：判不了 rc=2 与报告/阻断档同码")
	}

	// ── 不给结论②（外部树显式旋钮取不到）⇒ rc=2 ─────────────────────────
	rcL, outL, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "路径不存在.tsv")
	if rcL != 2 || !strings.Contains(outL, "面④ 脚本台账") {
		t.Fatalf("判不了②失败：仓外台账取不到应 rc=2 并照实打印，得 rc=%d out=%s", rcL, outL)
	}
	// 成对（新口径的牙）：单棵缺席照判在场那棵 ⇒ 台账在场（inv.tsv）⇒ rc=0 ≠ 都取不到 2
	rcOne, _, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcOne != 0 {
		t.Fatalf("退2边界失败：台账在场 ⇒ 应 rc=0，得 rc=%d", rcOne)
	}
	if rcOne == rcL {
		t.Fatalf("成对断言失败：台账在场 rc=%d 与取不到 rc=%d 同码 ⇒ 边界无牙", rcOne, rcL)
	}

	// ── 不给结论③（基线不在盘）⇒ rc=2 ───────────────────────────────────
	rcBase, outBase, _ := gnrRunT(t, "--repo", fxU, "--baseline", "no-such-baseline.tsv")
	if rcBase != 2 || !strings.Contains(outBase, "冻结基线不在盘") {
		t.Fatalf("判不了③失败：基线不在盘应 rc=2，得 rc=%d out=%s", rcBase, outBase)
	}

	// ── 弱挂载档不判红 ──────────────────────────────────────────────────
	fxW := gnrMkFixture(t, map[string]bool{"①": true, "②": true, "⑤": true})
	f, err := os.OpenFile(filepath.Join(fxW, gnrStepsRel), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("add_step \"build\" tri \"构建\" bash scripts/build/new-probe.sh\n")
	f.Close()
	gnrWriteInv(t, fxW, "件\n"+gnrProbeRel+"\n")
	rcW, outW, _ := gnrRunT(t, "--repo", fxW, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcW != 0 || !strings.Contains(outW, "弱挂载 1 件") {
		t.Fatalf("弱挂载失败：应 rc=0 且计弱挂载 1 件，得 rc=%d out=%s", rcW, outW)
	}

	// ── 写面（新口径）：--dump-baseline **默认干跑不写** ⟷ 带 --yes 才真落 ─────────
	// （旧断言「不带 --yes 就打印『基线已写：』」已被 py 新版废弃 ⇒ 断言改为判「干跑未写 + 零字节」。）
	dump := filepath.Join(t.TempDir(), "base.tsv")
	rcD, outD, _ := gnrRunT(t, "--repo", fx, "--dump-baseline", dump)
	if rcD != 0 || !strings.Contains(outD, "冻结基线生成档（**干跑** · 未写件）") ||
		strings.Contains(outD, "基线已写：") {
		t.Fatalf("--dump-baseline 干跑失败：应 rc=0 且打印计划、✗ 不打印『基线已写』，得 rc=%d out=%s", rcD, outD)
	}
	if _, err := os.Stat(dump); !os.IsNotExist(err) {
		t.Fatalf("--dump-baseline 干跑失败：**一个字节都不该写**（%s 竟在盘 · err=%v）", dump, err)
	}
	// 成对（写面的牙）：加 --yes 才真落 + 逐字保留「基线已写：%s（%d 件）」
	rcDY, outDY, _ := gnrRunT(t, "--repo", fx, "--dump-baseline", dump, "--yes")
	if rcDY != 0 || !strings.Contains(outDY, "基线已写：") || !strings.Contains(outDY, "（7 件）") {
		t.Fatalf("--dump-baseline --yes 失败：应 rc=0 且逐字打印『基线已写：…（7 件）』，得 rc=%d out=%s", rcDY, outDY)
	}
	if _, err := os.Stat(dump); err != nil {
		t.Fatalf("--dump-baseline --yes 失败：应真落盘，得 %v", err)
	}
	if rcD == rcDY && outD == outDY {
		t.Fatalf("成对断言失败：干跑与真落同文 ⇒ 写面无牙")
	}
	// ★ 甲/乙 成对（新口径）：打当前树（面非空 ∖ 在盘基线 = 0 件）⇒ **甲态 rc=0**（正常态）；
	//   基线不在盘 ⇒ **乙态 rc=2**（零执行 / 真源拿不到 ✗ 不放宽）。
	rcAfter, outAfter, _ := gnrRunT(t, "--repo", fx, "--inventory", "inv.tsv", "--baseline", dump)
	if rcAfter != 0 || !strings.Contains(outAfter, "全树扫过、确定性无新件") {
		t.Fatalf("甲态失守：生成口打当前树后应分母 0 ⇒ rc=0（正常态），得 rc=%d out=%s", rcAfter, outAfter)
	}
	rcMiss, outMiss, _ := gnrRunT(t, "--repo", fx, "--inventory", "inv.tsv", "--baseline", filepath.Join(t.TempDir(), "no-such-base.tsv"))
	if rcMiss != 2 || !strings.Contains(outMiss, "冻结基线不在盘") {
		t.Fatalf("乙态失守：基线不在盘应 rc=2，得 rc=%d out=%s", rcMiss, outMiss)
	}

	// ── 读不了的件：公开标记表 chmod 000 ⇒ 面① 判不了 ⇒ rc=2（fail-closed）────
	fxX := gnrMkFixture(t, allFive)
	os.Chmod(filepath.Join(fxX, gnrMarksRel), 0)
	rcX, outX, _ := gnrRunT(t, "--repo", fxX, "--new-file", gnrProbeRel, "--baseline", "no-base.tsv")
	if rcX != 2 || !strings.Contains(outX, "面①公开标记表 判不了") {
		t.Fatalf("读不了的件失败：应 rc=2 且点名面① 判不了，得 rc=%d out=%s", rcX, outX)
	}

	// ── 清单 / 自检 / 帮助 / 用法错 ─────────────────────────────────────
	rcST, outST, _ := gnrRunT(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过（断言 48 条 · 失败 0）") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%s", rcST, outST)
	}
	rcH, outH, _ := gnrRunT(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "usage: check-newfile-registry.py [-h] [--repo REPO]") {
		t.Fatalf("-h 失败：得 rc=%d out=%s", rcH, outH)
	}
	// 帮助面须含新两旗标（逐字）
	if !strings.Contains(outH, "[--json SECTIONS] [--yes]") ||
		!strings.Contains(outH, "机器面 JSON（逗号分隔断面：faces") ||
		!strings.Contains(outH, "写面确认：只有 `--dump-baseline` 会写件") {
		t.Fatalf("-h 失败：帮助面未见新旗标 --json/--yes，得 out=%s", outH)
	}
	rcZ, outZ, errZ := gnrRunT(t, "--zzz")
	if rcZ != 2 || outZ != "" || !strings.Contains(errZ, "unrecognized arguments: --zzz") {
		t.Fatalf("未知旗标失败：得 rc=%d stdout=%q stderr=%q", rcZ, outZ, errZ)
	}
	rcM, _, errM := gnrRunT(t, "--repo")
	if rcM != 2 || !strings.Contains(errM, "argument --repo: expected one argument") {
		t.Fatalf("缺值失败：得 rc=%d stderr=%q", rcM, errM)
	}
}

// TestCheckNewfileRegistryMachineFace —— 机器面（--json）与写面用法错的定向单测。
func TestCheckNewfileRegistryMachineFace(t *testing.T) {
	allFive := map[string]bool{"①": true, "②": true, "③": true, "⑤": true}
	fx := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fx, "件\n"+gnrProbeRel+"\n")
	base := []string{"--repo", fx, "--new-file", gnrProbeRel, "--inventory", "inv.tsv"}

	// ── faces,self：rc=0 · 人面向零输出 · JSON 形状齐 ────────────────────
	rc, out, errS := gnrRunT(t, append(append([]string{}, base...), "--json", "faces,self")...)
	if rc != 0 || errS != "" {
		t.Fatalf("--json faces,self 失败：应 rc=0/stderr 空，得 rc=%d stderr=%q", rc, errS)
	}
	if strings.Contains(out, "（件级 × 六张登记表 · **只报告**）") {
		t.Fatalf("--json 失败：人面向 stdout 未让位，得 out=%s", out)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("--json 失败：输出不是合法 JSON：%v\nout=%s", err, out)
	}
	for _, k := range []string{"schema", "tool", "gate", "repo", "selector", "strict", "rc",
		"expected_faces", "reported_faces", "face_states", "face_records", "counts",
		"host_required_faces", "host_required_count", "missing_faces", "missing_count",
		"fully_exercised", "all_green", "verdict", "faces", "self"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("--json 失败：缺顶层键 %q", k)
		}
	}
	if ef, _ := obj["expected_faces"].([]interface{}); len(ef) != 10 {
		t.Fatalf("--json 失败：expected_faces 应 10 面，得 %v", obj["expected_faces"])
	}
	fs, _ := obj["face_states"].(map[string]interface{})
	if len(fs) != 10 {
		t.Fatalf("--json 失败：face_states 应 10 键，得 %v", fs)
	}
	cnt, _ := obj["counts"].(map[string]interface{})
	for _, s := range []string{"PASS", "FAIL", "BLOCKED", "REPORT", "UNKNOWN", "HOST-REQUIRED"} {
		if _, ok := cnt[s]; !ok {
			t.Fatalf("--json 失败：counts 缺六态键 %q（得 %v）", s, cnt)
		}
	}
	// ★ 面⑧ host-bound：单列 HOST-REQUIRED · 不计 pass/fail · 暴露计数
	fr, _ := obj["face_records"].(map[string]interface{})
	if fr == nil || len(fr) != 10 {
		t.Fatalf("--json 失败：face_records 应 10 面，得 %v", fr)
	}
	r8, _ := fr["⑧家目录批准件"].(map[string]interface{})
	if r8 == nil || r8["state"] != "HOST-REQUIRED" || r8["reason_code"] != "SKIPPED-HOST-BOUND" || r8["decided"] != false {
		t.Fatalf("--json 失败：面⑧ 记录应 HOST-REQUIRED/SKIPPED-HOST-BOUND/decided=false，得 %v", r8)
	}
	if hf, _ := obj["host_required_faces"].([]interface{}); len(hf) != 1 || hf[0] != "⑧家目录批准件" {
		t.Fatalf("--json 失败：host_required_faces 应 [⑧家目录批准件]，得 %v", obj["host_required_faces"])
	}
	if obj["host_required_count"] != float64(1) {
		t.Fatalf("--json 失败：host_required_count 应 1，得 %v", obj["host_required_count"])
	}
	if mf, _ := obj["missing_faces"].([]interface{}); len(mf) != 0 {
		t.Fatalf("--json 失败：missing_faces 应空，得 %v", obj["missing_faces"])
	}
	if obj["missing_count"] != float64(0) {
		t.Fatalf("--json 失败：missing_count 应 0，得 %v", obj["missing_count"])
	}
	// ★ fully_exercised = (missing==0 且 host_bound==0)；有 host-bound 面 ⇒ false
	if obj["fully_exercised"] != false {
		t.Fatalf("--json 失败：有 host-bound 面 ⇒ fully_exercised 应 false，得 %v", obj["fully_exercised"])
	}
	if obj["all_green"] != true {
		t.Fatalf("--json 失败：全绿夹具应 all_green=true，得 %v", obj["all_green"])
	}
	// ★ 键序（输出行序的决定者 = 插入序，非 map 迭代）：钉顶层与 face_states 的插入序。
	// 用 `\"k\":` 形态（带冒号）以区分「键」与 selector/数组里的同名字符串值。
	order := []string{`"expected_faces":`, `"reported_faces":`, `"face_states":`, `"face_records":`,
		`"counts":`, `"host_required_faces":`, `"host_required_count":`, `"missing_faces":`,
		`"missing_count":`, `"fully_exercised":`, `"all_green":`, `"verdict":`, `"faces":`, `"self":`}
	last := -1
	for _, k := range order {
		i := strings.Index(out, k)
		if i < 0 || i < last {
			t.Fatalf("--json 失败：键序错（%s 位置 %d < 前 %d）\nout=%s", k, i, last, out)
		}
		last = i
	}
	fsOrder := []string{`"①公开标记表":`, `"②干跑登记":`, `"③门①两张真源":`, `"④脚本台账":`,
		`"⑤两器排除表":`, `"⑥失败文案面":`, `"⑧家目录批准件":`, `"⑨.sh门面清册":`, `"口径差栏":`, `"自指":`}
	last = -1
	for _, k := range fsOrder {
		i := strings.Index(out, k)
		if i < 0 || i < last {
			t.Fatalf("--json 失败：face_states 键序错（%s）\nout=%s", k, out)
		}
		last = i
	}

	// ── 断面单选：faces 无 self · self 无 faces ─────────────────────────
	_, outF, _ := gnrRunT(t, append(append([]string{}, base...), "--json", "faces")...)
	if !strings.Contains(outF, `"faces"`) || strings.Contains(outF, `"self"`) {
		t.Fatalf("--json faces 失败：应含 faces、✗ 不含 self，得 out=%s", outF)
	}
	_, outS, _ := gnrRunT(t, append(append([]string{}, base...), "--json", "self")...)
	if !strings.Contains(outS, `"self"`) || strings.Contains(outS, `"faces":`) {
		t.Fatalf("--json self 失败：应含 self、✗ 不含 faces 详情，得 out=%s", outS)
	}
	// 成对（断面真的有牙）：faces,self 比只 faces 多出 self 段 ⇒ 两文不同
	if outF == outS {
		t.Fatalf("成对断言失败：faces 与 self 断面同文 ⇒ 断面无牙")
	}

	// ── 负控：红件（未登记）机器面 ⇒ rc=0（报告）· fail 面计 FAIL ─────────
	fxU := gnrMkFixture(t, nil)
	gnrWriteInv(t, fxU, "件\n"+gnrProbeRel+"\n")
	rcU, outU, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--json", "faces,self")
	if rcU != 0 {
		t.Fatalf("机器面负控：报告档应 rc=0，得 rc=%d", rcU)
	}
	var oU map[string]interface{}
	if err := json.Unmarshal([]byte(outU), &oU); err != nil {
		t.Fatalf("机器面负控：JSON 解析失败 %v", err)
	}
	if oU["all_green"] != false {
		t.Fatalf("机器面负控：缺登记应 all_green=false，得 %v", oU["all_green"])
	}
	if cU, _ := oU["counts"].(map[string]interface{}); cU["FAIL"].(float64) == 0 {
		t.Fatalf("机器面负控：应至少一面 FAIL，得 counts=%v", cU)
	}
	// 成对（退码档的牙）：同件阻断档机器面 ⇒ rc=1
	rcUS, _, _ := gnrRunT(t, "--repo", fxU, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--json", "faces,self", "--strict")
	if rcUS != 1 {
		t.Fatalf("机器面负控：阻断档应 rc=1，得 rc=%d", rcUS)
	}

	// ── 用法错（rc=2 · 与 py 逐字同）：断面名不认 / 为空 / --json 与 dump / --yes 单独 ─
	rcBog, outBog, errBog := gnrRunT(t, append(append([]string{}, base...), "--json", "bogus")...)
	if rcBog != 2 || outBog != "" || !strings.Contains(errBog, "只认断面 faces,self；不认：bogus") {
		t.Fatalf("--json bogus 失败：得 rc=%d out=%q err=%q", rcBog, outBog, errBog)
	}
	rcEmp, _, errEmp := gnrRunT(t, append(append([]string{}, base...), "--json", "")...)
	if rcEmp != 2 || !strings.Contains(errEmp, "需要断面（faces,self 任一）") {
		t.Fatalf("--json 空失败：得 rc=%d err=%q", rcEmp, errEmp)
	}
	rcNoVal, _, errNoVal := gnrRunT(t, append(append([]string{}, base...), "--json")...)
	if rcNoVal != 2 || !strings.Contains(errNoVal, "argument --json: expected one argument") {
		t.Fatalf("--json 缺值失败：得 rc=%d err=%q", rcNoVal, errNoVal)
	}
	rcJD, _, errJD := gnrRunT(t, append(append([]string{}, base...), "--json", "faces", "--dump-baseline", filepath.Join(t.TempDir(), "b.tsv"))...)
	if rcJD != 2 || !strings.Contains(errJD, "只跟报告档连用（不与 `--dump-baseline` 同用）") {
		t.Fatalf("--json 与 dump 同用失败：得 rc=%d err=%q", rcJD, errJD)
	}
	rcY, _, errY := gnrRunT(t, "--yes")
	if rcY != 2 || !strings.Contains(errY, "只跟 `--dump-baseline`/`--sync` 连用") {
		t.Fatalf("--yes 单独失败：得 rc=%d err=%q", rcY, errY)
	}
	// 成对：合法组合不被误杀
	rcOK, _, errOK := gnrRunT(t, append(append([]string{}, base...), "--json", "faces,self")...)
	if rcOK != 0 || errOK != "" {
		t.Fatalf("成对失败：合法 --json 组合被误杀 rc=%d err=%q", rcOK, errOK)
	}
}

// gnrMkGitFixture —— 造一棵**真 git 夹具**（含被跟踪件 scripts/a.sh 并提交）· 仓外 t.TempDir。
func gnrMkGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "a.sh"), []byte("#!/usr/bin/env bash\necho a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	for _, a := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, a...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v 失败：%v\n%s", a, err, out)
		}
	}
	return root
}

// TestCheckNewfileRegistryCheckMissing —— ③ 查缺档（三合并式 · 隔离工作树）
// ＋ ④ 缺面合成 UNKNOWN 退码 2（直接单测聚合器 · 注入一枚不存在的面）。
// ✗ 禁恒绿：每一格都配了成对负控（干净 ⟷ 红 ⟷ 判不了 三态各成对）。
func TestCheckNewfileRegistryCheckMissing(t *testing.T) {
	root := gnrMkGitFixture(t)

	// ── ③① 干净工作树 ⇒ rc=0（三合并式逐条 0 件）─────────────────────────
	rc0, out0, err0 := gnrRunT(t, "--check-missing", "--repo", root)
	if rc0 != 0 || err0 != "" {
		t.Fatalf("查缺档干净档失败：应 rc=0/stderr 空，得 rc=%d err=%q out=%s", rc0, err0, out0)
	}
	for _, want := range []string{"门76 查缺档（三合并式 · 只读 · 隔离工作树）",
		"三合并式：① tracked 改动 0 · ② untracked 0 · ③ ignored 0", "三合并式逐条 0 件", "工作树已清 是"} {
		if !strings.Contains(out0, want) {
			t.Fatalf("查缺档干净档缺 %q\nout=%s", want, out0)
		}
	}

	// ── ③② 未跟踪新件（不 git add）⇒ 红 rc=1（git diff 看不见它 ⇒ ② 那条腿补洞）──
	if err := os.WriteFile(filepath.Join(root, "scripts", "new-probe.sh"), []byte("#!/usr/bin/env bash\necho new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcU, outU, _ := gnrRunT(t, "--check-missing", "--repo", root)
	if rcU != 1 || !strings.Contains(outU, "② untracked 1") ||
		!strings.Contains(outU, "✗ untracked scripts/new-probe.sh") || !strings.Contains(outU, "⇒ **红**") {
		t.Fatalf("查缺档未跟踪档失败：应 rc=1 且点名 untracked，得 rc=%d out=%s", rcU, outU)
	}
	if outU == out0 {
		t.Fatalf("成对断言失败：干净档与红档正文逐字相同 ⇒ 判据无鉴别力")
	}
	if drc, dout, _ := gnrGit(root, "diff", "--name-only", "--exit-code", "HEAD", "--", "scripts"); drc != 0 || len(dout) != 0 {
		t.Fatalf("① git diff 单用竟看见 untracked（洞没被 ② 补）：rc=%d out=%v", drc, dout)
	}
	os.Remove(filepath.Join(root, "scripts", "new-probe.sh"))

	// ── ③③ .gitignore 命中 ⇒ 红（ignored 捕获）· 绝不 rc=0 ────────────────
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("scripts/ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "ignored", "x.sh"), []byte("#!/usr/bin/env bash\necho x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcI, outI, _ := gnrRunT(t, "--check-missing", "--repo", root)
	if rcI != 1 || !strings.Contains(outI, "③ ignored 1") {
		t.Fatalf("查缺档 ignored 档失败：应 rc=1 且 ignored 1，得 rc=%d out=%s", rcI, outI)
	}
	os.RemoveAll(filepath.Join(root, "scripts", "ignored"))
	os.Remove(filepath.Join(root, ".gitignore"))

	// ── ③④ 零执行探针（空件集）⇒ rc=2 · 非绿 ────────────────────────────
	rcE, outE, _ := gnrRunT(t, "--check-missing", "--repo", root, "--paths", "scripts-nonexistent")
	if rcE != 2 || !strings.Contains(outE, "零执行探针") {
		t.Fatalf("查缺档空件集档失败：应 rc=2，得 rc=%d out=%s", rcE, outE)
	}
	if rcE == rc0 {
		t.Fatalf("成对断言失败：空件集 rc=2 与干净 rc=0 同码 ⇒ no_data 无牙")
	}

	// ── ③⑤ 非 git 工作树 ⇒ 不适用 rc=0 ─────────────────────────────────
	rcN, outN, _ := gnrRunT(t, "--check-missing", "--repo", t.TempDir())
	if rcN != 0 || !strings.Contains(outN, "三合并式**不适用**（非 git 工作树）") {
		t.Fatalf("查缺档非 git 档失败：应 rc=0 且不适用，得 rc=%d out=%s", rcN, outN)
	}

	// ── ③⑥ 真源不可读（.git 破损）⇒ rc=2 ───────────────────────────────
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: /nonexistent-gnr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcX, outX, _ := gnrRunT(t, "--check-missing", "--repo", broken)
	if rcX != 2 || !strings.Contains(outX, "真源不可读") {
		t.Fatalf("查缺档真源不可读档失败：应 rc=2，得 rc=%d out=%s", rcX, outX)
	}

	// ── ③⑦ 隔离 + 生成前清空输出目录（--outdir · 报告件字节与 JSON 键）──────
	outd := t.TempDir()
	if err := os.WriteFile(filepath.Join(outd, "旧件残留.txt"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcO, outO, _ := gnrRunT(t, "--check-missing", "--repo", root, "--outdir", outd)
	if rcO != 0 {
		t.Fatalf("查缺档 --outdir 失败：应 rc=0，得 rc=%d out=%s", rcO, outO)
	}
	if _, err := os.Stat(filepath.Join(outd, "旧件残留.txt")); !os.IsNotExist(err) {
		t.Fatalf("生成前未清空输出目录：旧件残留.txt 仍在（err=%v）", err)
	}
	for _, f := range []string{"查缺档.json", "查缺档.tsv"} {
		if _, err := os.Stat(filepath.Join(outd, f)); err != nil {
			t.Fatalf("查缺档 --outdir 未生成 %s：%v", f, err)
		}
	}
	// 写盘件 JSON 是 sort_keys=True ⇒ 顶层键按码位升序（appl… 在 file… 前）
	if raw, err := os.ReadFile(filepath.Join(outd, "查缺档.json")); err == nil {
		s := string(raw)
		if strings.Index(s, `"applicable":`) > strings.Index(s, `"file_set_size":`) {
			t.Fatalf("写盘件未按 sort_keys=True 排序：applicable 应在 file_set_size 之前\n%s", s)
		}
	}
	// worktree 退出必清：git worktree list 仅 1 条（主树）
	if lrc, lout, _ := gnrGit(root, "worktree", "list"); lrc != 0 || len(lout) != 1 {
		t.Fatalf("worktree 未清干净：rc=%d list=%v", lrc, lout)
	}

	// ── ③⑧ --json chk（机器面）＋ 用法错（逐字）─────────────────────────
	rcJ, outJ, errJ := gnrRunT(t, "--check-missing", "--repo", root, "--json", "chk")
	if rcJ != 0 || errJ != "" {
		t.Fatalf("查缺档 --json chk 失败：应 rc=0/stderr 空，得 rc=%d err=%q", rcJ, errJ)
	}
	var cobj map[string]interface{}
	if err := json.Unmarshal([]byte(outJ), &cobj); err != nil {
		t.Fatalf("查缺档 --json chk 不是合法 JSON：%v\nout=%s", err, outJ)
	}
	for _, k := range []string{"schema", "tool", "gate", "repo", "paths", "rc", "applicable", "red",
		"no_data", "unreadable", "reason", "tracked_modified", "untracked", "ignored",
		"file_set_size", "head_tracked_count", "worktree", "worktree_cleaned", "outdir"} {
		if _, ok := cobj[k]; !ok {
			t.Fatalf("查缺档 --json 缺顶层键 %q", k)
		}
	}
	if cobj["applicable"] != true || cobj["red"] != false || cobj["no_data"] != false || cobj["unreadable"] != false {
		t.Fatalf("查缺档 --json 干净档四标志应 true/false/false/false，得 %v/%v/%v/%v",
			cobj["applicable"], cobj["red"], cobj["no_data"], cobj["unreadable"])
	}
	// 用法错：断面只认 chk
	rcJB, _, errJB := gnrRunT(t, "--check-missing", "--repo", root, "--json", "bogus")
	if rcJB != 2 || !strings.Contains(errJB, "只认断面 chk；不认：bogus") {
		t.Fatalf("查缺档 --json bogus 失败：得 rc=%d err=%q", rcJB, errJB)
	}
	// 用法错：--outdir 只跟 --check-missing
	rcOD, _, errOD := gnrRunT(t, "--repo", root, "--outdir", t.TempDir())
	if rcOD != 2 || !strings.Contains(errOD, "`--outdir` 只跟 `--check-missing` 连用") {
		t.Fatalf("--outdir 单独失败：得 rc=%d err=%q", rcOD, errOD)
	}
	// 用法错：--check-missing 不与 --dump-baseline 同用
	rcCD, _, errCD := gnrRunT(t, "--check-missing", "--repo", root, "--dump-baseline", filepath.Join(t.TempDir(), "b.tsv"))
	if rcCD != 2 || !strings.Contains(errCD, "`--check-missing` 不与 `--dump-baseline` 同用") {
		t.Fatalf("--check-missing 与 dump 同用失败：得 rc=%d err=%q", rcCD, errCD)
	}
	// 缺值：--paths / --outdir
	if _, _, errP := gnrRunT(t, "--check-missing", "--paths"); !strings.Contains(errP, "argument --paths: expected one argument") {
		t.Fatalf("--paths 缺值失败：err=%q", errP)
	}
	if _, _, errO := gnrRunT(t, "--check-missing", "--outdir"); !strings.Contains(errO, "argument --outdir: expected one argument") {
		t.Fatalf("--outdir 缺值失败：err=%q", errO)
	}

	// ── ④ 缺面合成 UNKNOWN · 退码钉 2（直接单测聚合器 · 注入一枚不存在的面）──
	fx := gnrMkFixture(t, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
	gnrWriteInv(t, fx, "件\n"+gnrProbeRel+"\n")
	resA := gnrAudit(fx, []string{gnrProbeRel}, true, "inv.tsv", true, "")
	recs := gnrFaceRecords(resA, nil, nil)
	r8 := gnrRecValue(recs, "⑧家目录批准件")
	if s := gnrJRecField(r8, "state"); s != "HOST-REQUIRED" {
		t.Fatalf("面⑧ state 应 HOST-REQUIRED，得 %q", s)
	}
	if s := gnrJRecField(r8, "reason_code"); s != "SKIPPED-HOST-BOUND" {
		t.Fatalf("面⑧ reason_code 应 SKIPPED-HOST-BOUND，得 %q", s)
	}
	if gnrJRecBool(r8, "decided") {
		t.Fatalf("面⑧ decided 应 false（既不当绿也不当红）")
	}
	// 注入缺面 ⇒ MISSING_FACE 合成 · 退码钉 2
	bogusFace := "⑨合成缺面"
	facesB := append(gnrExpectedFaces(), bogusFace)
	recsB := gnrFaceRecords(resA, nil, facesB)
	rb := gnrRecValue(recsB, bogusFace)
	if s := gnrJRecField(rb, "state"); s != "UNKNOWN" {
		t.Fatalf("缺面 state 应 UNKNOWN，得 %q", s)
	}
	if s := gnrJRecField(rb, "reason_code"); s != "MISSING_FACE" {
		t.Fatalf("缺面 reason_code 应 MISSING_FACE，得 %q", s)
	}
	if gnrJRecBool(rb, "decided") {
		t.Fatalf("缺面 decided 应 false")
	}
	if mf := gnrMissingFaces(resA, facesB); len(mf) != 1 || mf[0] != bogusFace {
		t.Fatalf("missing_faces 应 [%s]，得 %v", bogusFace, mf)
	}
	if rcM, _, _ := gnrVerdict(resA, true, facesB); rcM != 2 {
		t.Fatalf("缺面注入 ⇒ 退码应钉 2，得 rc=%d", rcM)
	}
	if rcOK, _, _ := gnrVerdict(resA, true, nil); rcOK != 0 {
		t.Fatalf("正控·无缺面 ⇒ 不应因聚合器变 rc，得 rc=%d", rcOK)
	}
}

// TestCheckNewfileRegistryShGatesFace —— ⑨ `.sh` 门面清册（盘上 `scripts/gates/*.sh` ⇄ `real-gates.sh` 的 `SH_GATES` 双向差集）。
// ✗ 禁恒绿：正控（相等 ⇒ PASS）⟷ 负控甲（盘上多一件 ⇒ FAIL·缺登记）⟷ 负控乙（清册点名盘上无 ⇒ FAIL·悬空）
// ⟷ 真源缺（real-gates.sh 不在盘 ⇒ BLOCKED·rc=2 不给结论）四态成对。
func TestCheckNewfileRegistryShGatesFace(t *testing.T) {
	allFive := map[string]bool{"①": true, "②": true, "③": true, "⑤": true}

	// 正控：夹具桩（SH_GATES 逐只 == 盘上 scripts/gates/*.sh）⇒ 面⑨ PASS · strict rc=0 · 机器面 PASS
	fxP := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fxP, "件\n"+gnrProbeRel+"\n")
	rcP, outP, _ := gnrRunT(t, "--repo", fxP, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcP != 0 || !strings.Contains(outP, "面⑨ .sh 门面清册") || !strings.Contains(outP, "：0") {
		t.Fatalf("⑨ 正控失败：应 rc=0 且面⑨ 计 0，得 rc=%d out=%s", rcP, outP)
	}
	_, outJP, _ := gnrRunT(t, "--repo", fxP, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--json", "faces")
	var oP map[string]interface{}
	if err := json.Unmarshal([]byte(outJP), &oP); err != nil {
		t.Fatalf("⑨ 正控机器面非 JSON：%v", err)
	}
	if fs, _ := oP["face_states"].(map[string]interface{}); fs["⑨.sh门面清册"] != "PASS" {
		t.Fatalf("⑨ 正控 face_states 面⑨ 应 PASS，得 %v", fs["⑨.sh门面清册"])
	}

	// 负控甲：盘上多一件（未进 SH_GATES）⇒ FAIL（缺登记）· 报告档 rc=0 ⟷ 阻断档 rc=1（成对）
	fxE := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fxE, "件\n"+gnrProbeRel+"\n")
	if err := os.WriteFile(filepath.Join(fxE, gnrShGatesDir, "ghost-on-disk.sh"), []byte("#!/usr/bin/env bash\necho g\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcEr, outEr, _ := gnrRunT(t, "--repo", fxE, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcEr != 0 || !strings.Contains(outEr, "ghost-on-disk.sh") || !strings.Contains(outEr, "盘上有但没进") {
		t.Fatalf("⑨ 负控甲（报告档）失败：应 rc=0 且点名 ghost-on-disk.sh，得 rc=%d out=%s", rcEr, outEr)
	}
	rcEs, _, _ := gnrRunT(t, "--repo", fxE, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcEs != 1 {
		t.Fatalf("⑨ 负控甲（阻断档）失败：应 rc=1，得 rc=%d", rcEs)
	}
	if rcEr == rcEs {
		t.Fatalf("⑨ 成对断言失败：报告档 rc=%d 与阻断档 rc=%d 同码 ⇒ 退码档无牙", rcEr, rcEs)
	}

	// 负控乙：清册点名一件而盘上无 ⇒ FAIL（悬空登记）· strict rc=1 · 与正控正文不同（有牙）
	fxM := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fxM, "件\n"+gnrProbeRel+"\n")
	if err := os.WriteFile(filepath.Join(fxM, gnrShGatesRel), []byte("SH_GATES=(precommit-gates.sh real-gates.sh ghost-in-roster.sh)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcMs, outMs, _ := gnrRunT(t, "--repo", fxM, "--new-file", gnrProbeRel, "--inventory", "inv.tsv", "--strict")
	if rcMs != 1 || !strings.Contains(outMs, "ghost-in-roster.sh") || !strings.Contains(outMs, "悬空登记") {
		t.Fatalf("⑨ 负控乙失败：应 rc=1 且点名悬空，得 rc=%d out=%s", rcMs, outMs)
	}
	if outMs == outP {
		t.Fatalf("⑨ 成对断言失败：负控乙正文与正控逐字相同 ⇒ 判据无鉴别力")
	}

	// BLOCKED：真源 real-gates.sh 不在盘 ⇒ 面⑨ 判不了 · rc=2（不给结论 · fail-closed）
	fxB := gnrMkFixture(t, allFive)
	gnrWriteInv(t, fxB, "件\n"+gnrProbeRel+"\n")
	if err := os.Remove(filepath.Join(fxB, gnrShGatesRel)); err != nil {
		t.Fatal(err)
	}
	rcB, outB, _ := gnrRunT(t, "--repo", fxB, "--new-file", gnrProbeRel, "--inventory", "inv.tsv")
	if rcB != 2 || !strings.Contains(outB, "面⑨.sh门面清册 判不了") {
		t.Fatalf("⑨ 真源缺失败：应 rc=2 且面⑨ 判不了，得 rc=%d out=%s", rcB, outB)
	}
}

// TestGnrRefParity —— 与 py 新版**逐字节对拍**的挂钩（仓外驱动注入环境变量；不设则跳过）。
// 环境：GNR_MANIFEST=清单 JSON（[{name, argv[], out, err, rc}]）。
func TestGnrRefParity(t *testing.T) {
	man := os.Getenv("GNR_MANIFEST")
	if man == "" {
		t.Skip("GNR_MANIFEST 未设 ⇒ 跳过对拍（由仓外驱动注入）")
	}
	raw, err := os.ReadFile(man)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string   `json:"name"`
		Argv []string `json:"argv"`
		Out  string   `json:"out"`
		Err  string   `json:"err"`
		RC   string   `json:"rc"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("清单解析失败：%v", err)
	}
	if len(cases) == 0 {
		t.Fatal("清单为空 ⇒ 假绿")
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		rc := checkNewfileRegistryGateMain(&out, &errb, c.Argv)
		if want, e := os.ReadFile(c.Out); e != nil {
			t.Errorf("[%s] 读期望 stdout 失败：%v", c.Name, e)
		} else if !bytes.Equal(out.Bytes(), want) {
			t.Errorf("[%s] stdout 与 py 不一致：\n--- go ---\n%s\n--- py ---\n%s", c.Name, out.String(), string(want))
		}
		if c.Err != "" {
			if want, e := os.ReadFile(c.Err); e != nil {
				t.Errorf("[%s] 读期望 stderr 失败：%v", c.Name, e)
			} else if !bytes.Equal(errb.Bytes(), want) {
				t.Errorf("[%s] stderr 与 py 不一致：\n--- go ---\n%s\n--- py ---\n%s", c.Name, errb.String(), string(want))
			}
		}
		if c.RC != "" && strconv.Itoa(rc) != c.RC {
			t.Errorf("[%s] 退码与 py 不一致：go=%d py=%s", c.Name, rc, c.RC)
		}
	}
}

// TestGnrDryStepCliFormFallback —— 2026-10-07 检查点4：步骤表直调 py 改走 CLI 形
// `zerg gate check <键>` ⇒ 门76 面② 的干跑判据解析器须按**键名**落到已搬 Go 的进程内判据件
// （✗ 不因行里没有 `.py` 落点而「判不了」= fail-closed）。★ 成对负控：键名不认 + 行里无 `.py` ⇒ fail-closed。
// ★ 哑化负控：抽掉 `gnrLoadDryJudge` 里的 CLI 分支 ⇒ 本用例第①格 FAIL（带行号）。
func TestGnrDryStepCliFormFallback(t *testing.T) {
	key := "check-" + "dry" + "run" + "-semantics" // 拼串构造（✗ 不落连续令牌字面 · 门⑩ 自扫面）
	tok := "dry" + "run"
	mk := func(root, rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	build := func(cliKey string) string {
		root := t.TempDir()
		mk(root, "core/probe.go", "// "+tok+" site\npackage x\n")
		mk(root, "scripts/gates/precommit-gates.sh",
			"#!/usr/bin/env bash\n"+
				"add_step \"gates\" tri \"门⑩ 干跑：一套语义\" \"${REPO_ROOT}\" \"zerg gate check "+cliKey+"\"\n")
		mk(root, "core/internal/contract/"+tok+"-semantics.json",
			"{\"件\": [{\"file\": \"core/probe.go\"}, {\"file\": \"scripts/gates/precommit-gates.sh\"}]}")
		return root
	}
	// ① CLI 形键名认（行里无 `.py`）⇒ 落到进程内判据，取到命中集与登记集。
	d := gnrLoadDryJudge(build(key))
	if d.problem != "" {
		t.Fatalf("CLI 形干跑步该按键名落到进程内判据（✗ 判不了）：%q", d.problem)
	}
	if !d.hitsOK || !d.regOK {
		t.Fatalf("CLI 形该取到命中集与登记集：hitsOK=%v regOK=%v", d.hitsOK, d.regOK)
	}
	// ② 负控：键名不认 + 行里无 `.py` ⇒ fail-closed（不给结论 · ✗ 静默绿）。
	d2 := gnrLoadDryJudge(build("check-unknown-xyz"))
	if d2.problem == "" {
		t.Fatal("键名不认 + 无 .py ⇒ 该 fail-closed（✗ 静默绿）")
	}
}

// ═════════════════════════════════════════════════════════════════════════════
// ③ 根定位（与 py `__file__` 同语义 · 与 cwd 无关）＋ ④ help 跨语言逐字对拍机械拦阻
//    —— 对应主件件头 (a) 与未覆盖面①②。✗ 禁恒绿：以下每一格都配了成对负控。
//
// 退路声明（④ · 取舍照实写）：主路 = **运行时逐字对拍**（现跑 `/usr/bin/python3 <py 件> -h`，
// 本仓测试环境恒有该解释器 —— 多个既有测件已依赖它）。若某机无 python3 ⇒ 退路 = **钉住 py 正文
// sha256**（`gnrHelpPySha256` ⇔ `gnrHelpGoSha256` 互等断言）：仍能抳住「Go 副本漂了」，
// 但「py 正文漂了而钉子没跟着刷」需靠**重生成正门**刷（`GNR_REGEN_HELP_SHA=1 go test …`）才可见。
// 该退路必红条件：Go 帮助面字节/sha 一变、或两常量被改成不等 ⇒ 当场红。
// ═════════════════════════════════════════════════════════════════════════════

// gnrTSha256 —— 帮助面指纹（本测件内唯一）。
func gnrTSha256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// gnrTRootMarkerFixture —— 造一棵「像仓根」的夹具（只放本件认根用的两处标记）。
func gnrTRootMarkerFixture(t *testing.T) string {
	t.Helper()
	fx := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fx, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx, gnrStepsRel), []byte("# 夹具步表（只作仓根标记）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return fx
}

// gnrRepoRootForTest —— 与本件源码同栖的仓根（独立于 gnrDefaultRoot 的推导 · 供期望值用）。
func gnrRepoRootForTest(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 不可用")
	}
	d := filepath.Dir(f)
	for i := 0; i < 12; i++ {
		if gnrIsRootDir(d) {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	t.Fatalf("从 %s 上溯找不到仓根标记", f)
	return ""
}

// TestGnrRootMappingParity —— ③「两边文件住不同深度」的映射现读对拍：
//
//	py  REPO = dirname^3(<root>/scripts/gates/<件>.py) == <root>
//	Go  gnrRootFromExe(<root>/bin/<名>)                == <root>（上溯两级）
//
// ★ 成对负控：无仓根标记的临时目录**不得**被认成仓根（测试二进制住临时目录时的防误判）。
func TestGnrRootMappingParity(t *testing.T) {
	fx := gnrTRootMarkerFixture(t)

	pyRoot := filepath.Dir(filepath.Dir(filepath.Dir(
		filepath.Join(fx, "scripts", "gates", "check-newfile-registry.py"))))
	if pyRoot != fx {
		t.Fatalf("py 映射不符：dirname^3 = %s 期望 %s", pyRoot, fx)
	}
	goRoot, ok := gnrRootFromExe(filepath.Join(fx, "bin", "zerg"))
	if !ok || goRoot != fx {
		t.Fatalf("go 映射不符：gnrRootFromExe(%s) = (%q,%v) 期望 (%s,true)",
			filepath.Join(fx, "bin", "zerg"), goRoot, ok, fx)
	}
	if goRoot != pyRoot {
		t.Fatalf("两侧根定位语义不同：go=%s py=%s", goRoot, pyRoot)
	}

	// 负控（防误判）：临时目录（无标记）⇒ ok=false
	if r, ok := gnrRootFromExe(filepath.Join(t.TempDir(), "bin", "zerg.test")); ok {
		t.Fatalf("负控失败：无仓根标记的临时目录被认成仓根 %s", r)
	}
	// 正控对照：同一「空目录」补上标记后转 ok ⇒ 判的是「标记」而非「路径形态」（负控有牙）
	bare := t.TempDir()
	if r, ok := gnrRootFromExe(filepath.Join(bare, "bin", "zerg")); ok {
		t.Fatalf("正控前：空目录竟被判成仓根 %s", r)
	}
	if err := os.MkdirAll(filepath.Join(bare, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bare, gnrStepsRel), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r, ok := gnrRootFromExe(filepath.Join(bare, "bin", "zerg")); !ok || r != bare {
		t.Fatalf("正控失败：补上标记后应判为仓根 %s，得 (%q,%v)", bare, r, ok)
	}
}

// TestGnrCwdProbe —— 子进程探针：由 TestGnrDefaultRootIndependentOfCwd 以 `-test.run` 起，
// cwd 由父进程设成不同目录 ⇒ 即「在非仓根 cwd 下跑 Go 入口」。直接把入口三面写到真实
// stdout/stderr 并以入口退码退出（✗ 不经 testing 缓冲 ⇒ 子进程读数 = 入口读数 · 不受 -test.v 噪声干扰）。
func TestGnrCwdProbe(t *testing.T) {
	if os.Getenv("GNR_CWD_PROBE") != "1" {
		t.Skip("仅由 TestGnrDefaultRootIndependentOfCwd 以子进程方式起（GNR_CWD_PROBE=1）")
	}
	rc := checkNewfileRegistryGateMain(os.Stdout, os.Stderr,
		[]string{"--dump-baseline", os.Getenv("GNR_CWD_DUMP")})
	os.Exit(rc)
}

// TestGnrDefaultRootIndependentOfCwd —— ③ 的验收负控：同一 Go 入口在「非仓根 cwd」与「仓根 cwd」下，
// stdout / stderr / rc 三面**逐字相同**（同一条命令在任何目录同结果）；py 侧同格对照 + Go↔py 逐字。
// 三档 cwd：临时目录（非仓根）· 仓根 · 仓根子目录 core/。
func TestGnrDefaultRootIndependentOfCwd(t *testing.T) {
	wantRoot := gnrRepoRootForTest(t)
	if got := gnrDefaultRoot(); got != wantRoot {
		t.Fatalf("默认根 = %q，期望仓根 %q", got, wantRoot)
	}
	if !gnrIsRootDir(gnrDefaultRoot()) {
		t.Fatalf("默认根 %q 非仓根", gnrDefaultRoot())
	}
	// ★ 退役稳态（B轨 · 2026-10-07）：只硬要求 Go 半在盘；同族 py（gnrSelfRel）是退役候选 ⇒
	//   在盘时顺带核一次、✗ 不在盘不 fail（退役态合法 · 件退役后本件照常出结论）。
	for _, rel := range []string{"core/cmd/zerg/check_newfile_registry.go"} {
		if _, err := os.Stat(filepath.Join(wantRoot, rel)); err != nil {
			t.Fatalf("仓根 %s 下缺 %s：%v", wantRoot, rel, err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(t.TempDir(), "probe-base.tsv")
	runGo := func(dir string) (int, string, string) {
		t.Helper()
		cmd := exec.Command(exe, "-test.run", "^TestGnrCwdProbe$")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GNR_CWD_PROBE=1", "GNR_CWD_DUMP="+dump)
		var ob, eb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &ob, &eb
		err := cmd.Run()
		rc := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				rc = ee.ExitCode()
			} else {
				t.Fatalf("探针起不来 @%s：%v", dir, err)
			}
		}
		return rc, ob.String(), eb.String()
	}
	nonRoot := t.TempDir()
	rcA, outA, errA := runGo(nonRoot)
	rcB, outB, errB := runGo(wantRoot)
	rcC, outC, errC := runGo(filepath.Join(wantRoot, "core"))
	if outA != outB || errA != errB || rcA != rcB {
		t.Fatalf("非仓根 cwd 与仓根 cwd 结果不一致：\n[cwd=%s] rc=%d\n--stdout--\n%s\n--stderr--\n%s\n[cwd=%s] rc=%d\n--stdout--\n%s\n--stderr--\n%s",
			nonRoot, rcA, outA, errA, wantRoot, rcB, outB, errB)
	}
	if outA != outC || errA != errC || rcA != rcC {
		t.Fatalf("core/ cwd 与临时 cwd 结果不一致：rcA=%d rcC=%d", rcA, rcC)
	}
	// 该档真跑在仓根上（非「扫面不在盘」的 rc=2）⇒ 证上面三面同的是**真读数**
	if rcA != 0 || !strings.Contains(outA, "冻结基线生成档") {
		t.Fatalf("探针未在仓根上真跑：rc=%d stdout=%s stderr=%s", rcA, outA, errA)
	}

	// ── py 侧同格对照：py 默认根 = `__file__` 推导 ⇒ 异 cwd 同结果（并顺带与 Go 逐字比）──
	pyExe := gnrTHelpPython()
	if pyExe == "" {
		t.Log("无 /usr/bin/python3 ⇒ py 侧同格对照跳过（Go 侧三档已证 · 见测件头退路声明）")
		return
	}
	pyFile := filepath.Join(wantRoot, gnrSelfRel)
	if _, err := os.Stat(pyFile); err != nil {
		// ★ 退役态（B轨 · 2026-10-07）：同族 py 退役（不在盘）⇒ py 侧同格对照不适用；Go 侧三档已证 ⇒
		//   ✗ 不 fail（件退役后本件照常出结论，✗ 不得 fail-closed）。
		t.Logf("同族 py 退役（不在盘）⇒ py 侧同格对照跳过：%s", pyFile)
		return
	}
	runPy := func(dir string) (int, string, string) {
		t.Helper()
		cmd := exec.Command(pyExe, pyFile, "--dump-baseline", dump)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "COLUMNS=80")
		var ob, eb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &ob, &eb
		err := cmd.Run()
		rc := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				rc = ee.ExitCode()
			} else {
				t.Fatalf("py 起不来 @%s：%v", dir, err)
			}
		}
		return rc, ob.String(), eb.String()
	}
	prcA, poutA, perrA := runPy(nonRoot)
	prcB, poutB, perrB := runPy(wantRoot)
	if poutA != poutB || perrA != perrB || prcA != prcB {
		t.Fatalf("py 异 cwd 结果不一致：rcA=%d rcB=%d outA=%q outB=%q", prcA, prcB, poutA, poutB)
	}
	// Go ↔ py 逐字（同一 cwd=仓根 · 默认根两侧同为仓根）
	if outB != poutB || errB != perrB || rcB != prcB {
		t.Fatalf("Go↔py 默认根读数不同：\n-- go rc=%d --\n%s\nstderr=%q\n-- py rc=%d --\n%s\nstderr=%q",
			rcB, outB, errB, prcB, poutB, perrB)
	}
}

// gnrTHelpPython —— 钉死的 py 解释器（GAP-20261001-61：✗ 不用 PATH 上的 python3）；
// 无 /usr/bin/python3 ⇒ 返回空串（走钉 sha256 的退路）。
func gnrTHelpPython() string {
	p := "/usr/bin/python3"
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// gnrTRunPyHelp —— 现跑 py 件某档（COLUMNS=80 钉住折行），返回 rc/stdout/stderr 三面。
func gnrTRunPyHelp(t *testing.T, pyExe, pyFile string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(pyExe, append([]string{pyFile}, args...)...)
	cmd.Env = append(os.Environ(), "COLUMNS=80")
	var ob, eb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &ob, &eb
	err := cmd.Run()
	rc := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("py 起不来：%v", err)
		}
	}
	return rc, ob.String(), eb.String()
}

// gnrHelpGoSha256 = sha256(gnrHelp) —— Go 现行帮助面正文（COLUMNS=80 · 见 gnrHelpBytes）。
const gnrHelpGoSha256 = "e6b04dba6dc821bf1b88d5a12826761319daee59ba57b5860267c9b2f45aed49"

// gnrHelpPySha256 = 同一正文 —— py 原件 `COLUMNS=80 /usr/bin/python3 <py 件> -h` 的 stdout 全串。
// ★ 机械拦阻的核心：两者不等即红（py 正文一改 / Go 副本一改，都当场点名）。
// 重生成正门（现读打印两值 · 不落任何件）：
//
//	GNR_REGEN_HELP_SHA=1 go test ./cmd/zerg -run '^TestGnrHelpParityWithPy$' -v
const gnrHelpPySha256 = "e6b04dba6dc821bf1b88d5a12826761319daee59ba57b5860267c9b2f45aed49"

// gnrHelpBytes —— 帮助面正文字节数（2397 · 与 COLUMNS=80 的 py 现跑一致）。
const gnrHelpBytes = 2397

// TestGnrHelpParityWithPy —— ④ 机械拦阻：Go 的 help 输出 == py 的 --help 输出（stdout+stderr+rc 三面）。
//
//	① Go 侧三面 + 字节数 + sha256（钉 gnrHelpGoSha256）；
//	② 两个 sha256 常量必须互等（py 面漂了而 Go 没跟 ⇒ 必红）；
//	③ py 在场：现跑 `-h` / `--help` / `--zzz`，与 Go 逐字节对拍；
//	④ py 缺席：退路 = 只校验①（并在日志声明）。
func TestGnrHelpParityWithPy(t *testing.T) {
	// ── ① Go 侧三面（-h 与 --help 同文）──────────────────────────────
	goOut := map[string]string{}
	for _, flag := range []string{"-h", "--help"} {
		rc, out, errs := gnrRunT(t, flag)
		if rc != 0 || errs != "" {
			t.Fatalf("Go %s：rc=%d stderr=%q 期望 rc=0/空", flag, rc, errs)
		}
		if len(out) != gnrHelpBytes {
			t.Fatalf("Go %s 字节数 = %d 期望 %d ⇒ Go 帮助面已漂", flag, len(out), gnrHelpBytes)
		}
		if got := gnrTSha256(out); got != gnrHelpGoSha256 {
			t.Fatalf("Go %s 帮助面 sha256 = %s 期望 %s ⇒ Go 侧副本已漂（改 gnrHelp 后须同批刷新本常量）", flag, got, gnrHelpGoSha256)
		}
		goOut[flag] = out
	}
	if goOut["-h"] != goOut["--help"] {
		t.Fatalf("-h 与 --help 输出不同（argparse 两侧应同文）")
	}
	// ── ② 两常量互等（py 正文漂了却没跟 Go ⇒ 必红）─────────────────────
	if gnrHelpGoSha256 != gnrHelpPySha256 {
		t.Fatalf("Go 帮助面 sha256 ≠ 钉住的 py 帮助面 sha256 ⇒ py 正文已改而 Go 副本未同批跟：go=%s py=%s",
			gnrHelpGoSha256, gnrHelpPySha256)
	}

	// ── 重生成正门（现读打印 · 不落件）──────────────────────────────────
	if os.Getenv("GNR_REGEN_HELP_SHA") == "1" {
		pyExe := gnrTHelpPython()
		if pyExe == "" {
			t.Fatalf("无 /usr/bin/python3 ⇒ 无 py 侧现读值可回填")
		}
		pyFile := filepath.Join(gnrRepoRootForTest(t), gnrSelfRel)
		_, out, _ := gnrTRunPyHelp(t, pyExe, pyFile, "-h")
		fmt.Printf("GNR_HELP_REGEN go_sha256=%s py_sha256=%s bytes=%d\n", gnrHelpGoSha256, gnrTSha256(out), len(out))
		return
	}

	// ── ③ py 在场：现跑三面逐字对拍 ─────────────────────────────────────
	pyExe := gnrTHelpPython()
	if pyExe == "" {
		t.Log("无 /usr/bin/python3 ⇒ 只走钉 sha256 的退路（py 正文漂移需靠重生成正门刷 gnrHelpPySha256 才可见）")
		return
	}
	pyFile := filepath.Join(gnrRepoRootForTest(t), gnrSelfRel)
	gnrHelpParityOrRetired(t, pyExe, pyFile)
}

// TestGnrHelpRetiredNoPy —— 退役态（B轨 · 2026-10-07）：同族 py 原件**不在盘** ⇒ 帮助面只走 Go 侧 +
// 钉 sha256（✗ 不得 fail-closed / t.Fatalf）。本件（Go 半）不读该 py ⇒ 退役不改 Go 侧结论。
// ★ 哑化负控：把 `gnrHelpParityOrRetired` 退役分支改成 `t.Fatalf("py 件不在盘")` ⇒ 本用例红（带行号）。
func TestGnrHelpRetiredNoPy(t *testing.T) {
	gnrHelpParityOrRetired(t, gnrTHelpPython(), filepath.Join(t.TempDir(), "no-such-check-newfile-registry.py"))
}

// gnrHelpParityOrRetired —— pyExe 空 ⇒ 只走钉 sha256；py 原件**不在盘（退役）** ⇒ 跳过 py 侧逐字对拍
// （Go 侧 sha256/字节数已由调用方钉过）；在盘 ⇒ 三面逐字对拍。
func gnrHelpParityOrRetired(t *testing.T, pyExe, pyFile string) {
	t.Helper()
	if pyExe == "" {
		t.Log("无 /usr/bin/python3 ⇒ 只走钉 sha256 的退路（py 正文漂移需靠重生成正门刷 gnrHelpPySha256 才可见）")
		return
	}
	if _, err := os.Stat(pyFile); err != nil {
		// ★ 退役态：同族 py 退役（不在盘）⇒ py 侧逐字对拍不适用；Go 侧 sha256/字节数已由调用方钉住 ⇒ ✗ 不 fail。
		t.Logf("同族 py 退役（不在盘）⇒ py 侧帮助面逐字对拍跳过：%s", pyFile)
		return
	}
	for _, flag := range []string{"-h", "--help"} {
		prc, pout, perr := gnrTRunPyHelp(t, pyExe, pyFile, flag)
		if prc != 0 || perr != "" {
			t.Fatalf("py %s：rc=%d stderr=%q 期望 rc=0/空", flag, prc, perr)
		}
		if got := gnrTSha256(pout); got != gnrHelpPySha256 {
			t.Fatalf("py %s 帮助面 sha256 = %s ≠ 钉住的 %s ⇒ py 正文已改：把 gnrHelp 改成现读正文并同批刷新本常量", flag, got, gnrHelpPySha256)
		}
		rc, gout, gerr := gnrRunT(t, flag)
		if pout != gout || perr != gerr || prc != rc {
			t.Fatalf("help 逐字对拍失败（%s）：\n-- go rc=%d --\n%s\nstderr=%q\n-- py rc=%d --\n%s\nstderr=%q",
				flag, rc, gout, gerr, prc, pout, perr)
		}
	}
	// 帮助面家族第三格（未知旗标 usage 正文 · 逐字保留）：stderr + rc
	prc, pout, perr := gnrTRunPyHelp(t, pyExe, pyFile, "--zzz")
	rc, gout, gerr := gnrRunT(t, "--zzz")
	if pout != gout || perr != gerr || prc != rc {
		t.Fatalf("未知旗标 usage 对拍失败：\n-- go rc=%d out=%q err=%q\n-- py rc=%d out=%q err=%q",
			rc, gout, gerr, prc, pout, perr)
	}
}

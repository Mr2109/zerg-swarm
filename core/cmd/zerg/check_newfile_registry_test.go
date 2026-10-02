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
	"encoding/json"
	"os"
	"path/filepath"
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
	if rcDY != 0 || !strings.Contains(outDY, "基线已写：") || !strings.Contains(outDY, "（6 件）") {
		t.Fatalf("--dump-baseline --yes 失败：应 rc=0 且逐字打印『基线已写：…（6 件）』，得 rc=%d out=%s", rcDY, outDY)
	}
	if _, err := os.Stat(dump); err != nil {
		t.Fatalf("--dump-baseline --yes 失败：应真落盘，得 %v", err)
	}
	if rcD == rcDY && outD == outDY {
		t.Fatalf("成对断言失败：干跑与真落同文 ⇒ 写面无牙")
	}
	// py 口子①（--dump-baseline 真落口 · 逐字保留）⇒ 打当前树后分母恒 0
	rcAfter, outAfter, _ := gnrRunT(t, "--repo", fx, "--baseline", dump)
	if rcAfter != 2 || !strings.Contains(outAfter, "分母（现读新件 ∖ 冻结基线）：0 件") {
		t.Fatalf("py 口子①失守：生成口打当前树后应分母 0 ⇒ rc=2，得 rc=%d out=%s", rcAfter, outAfter)
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
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过（断言 8 条 · 失败 0）") {
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
		"expected_faces", "reported_faces", "face_states", "counts", "all_green", "verdict", "faces", "self"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("--json 失败：缺顶层键 %q", k)
		}
	}
	if ef, _ := obj["expected_faces"].([]interface{}); len(ef) != 8 {
		t.Fatalf("--json 失败：expected_faces 应 8 面，得 %v", obj["expected_faces"])
	}
	fs, _ := obj["face_states"].(map[string]interface{})
	if len(fs) != 8 {
		t.Fatalf("--json 失败：face_states 应 8 键，得 %v", fs)
	}
	cnt, _ := obj["counts"].(map[string]interface{})
	for _, s := range []string{"PASS", "FAIL", "BLOCKED", "REPORT", "UNKNOWN"} {
		if _, ok := cnt[s]; !ok {
			t.Fatalf("--json 失败：counts 缺五态键 %q（得 %v）", s, cnt)
		}
	}
	if obj["all_green"] != true {
		t.Fatalf("--json 失败：全绿夹具应 all_green=true，得 %v", obj["all_green"])
	}
	// ★ 键序（输出行序的决定者 = 插入序，非 map 迭代）：钉顶层与 face_states 的插入序。
	// 用 `"k":` 形态（带冒号）以区分「键」与 selector/数组里的同名字符串值。
	order := []string{`"expected_faces":`, `"reported_faces":`, `"face_states":`, `"counts":`, `"all_green":`, `"verdict":`, `"faces":`, `"self":`}
	last := -1
	for _, k := range order {
		i := strings.Index(out, k)
		if i < 0 || i < last {
			t.Fatalf("--json 失败：键序错（%s 位置 %d < 前 %d）\nout=%s", k, i, last, out)
		}
		last = i
	}
	fsOrder := []string{`"①公开标记表":`, `"②干跑登记":`, `"③门①两张真源":`, `"④脚本台账":`, `"⑤两器排除表":`, `"⑥失败文案面":`, `"口径差栏":`, `"自指":`}
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
	if rcY != 2 || !strings.Contains(errY, "只跟 `--dump-baseline` 连用") {
		t.Fatalf("--yes 单独失败：得 rc=%d err=%q", rcY, errY)
	}
	// 成对：合法组合不被误杀
	rcOK, _, errOK := gnrRunT(t, append(append([]string{}, base...), "--json", "faces,self")...)
	if rcOK != 0 || errOK != "" {
		t.Fatalf("成对失败：合法 --json 组合被误杀 rc=%d err=%q", rcOK, errOK)
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

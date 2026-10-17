package main

// zerg_egg_version_test.go —— 卵6（版本与兼容）的**机检件**：13 档表完整性（含 D6a/D6b 拆分）· 二分类表 ·
// 四态亲测（真跑 → 真读数）· 负控（哑化 min_readable 那条 ⇒ 态③ 必红）· `--self-test` 链尾那一行。
//
// 口径三条（照任务单「卵6」＋ 设计稿 §11.2 / §16.5 / §十九 #4/#29/#30/#31）：
//  ① 四态**走真命令面**（`zergEggGateMain`）取 rc/stdout/stderr，✗ 不只调内部函数 —— 内部档位另由
//     `zegParseIndex` 的类型化拒（`*zegRefusal`）逐态点名。
//  ② 负控是**两态对拍**：同一份 `min_readable` 超本端输入，改前形态（`zegCompatGateLegacy`）⇒ 放行，
//     有牙档 ⇒ 拒；两结论**必须不同**（哑化牙 ⇒ 本件红，见 `-overlay` 负控）。
//  ③ 本件✗ 不做 `len(<结构>) != N` 一类计数断言（照门128 的四属性口径避开）。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegTMakeProbeEgg —— 造一枚干净的探针卵（件名**不带内容 sha** ⇒ 每态都真解析索引，✗ 不走内容寻址缓存）。
func zegTMakeProbeEgg(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 2048)
	for i := range payload {
		payload[i] = byte(i * 5)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.safetensors"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"arch":"probe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	egg := filepath.Join(dir, zegVerProbeEgg)
	if _, err := zegPack(src, egg, 4096, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	return egg
}

// zegTCloneEggWithIdx —— 复制一枚卵并把索引体按 `edit` 改过（段偏移一位不动）。
func zegTCloneEggWithIdx(t *testing.T, src, dst string, edit func(map[string]json.RawMessage) error, prefixVer int) string {
	t.Helper()
	d, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, d, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zegVerRebuildEgg(dst, func(idx []byte) ([]byte, error) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(idx, &m); err != nil {
			return nil, err
		}
		if err := edit(m); err != nil {
			return nil, err
		}
		return json.Marshal(m)
	}, prefixVer); err != nil {
		t.Fatal(err)
	}
	return dst
}

// ---- ① 13 档表（含 D6a/D6b 拆分）· 逐档「谁报 · 报什么 · 退码 · 依据」------------------------------

func TestZegVersionTierTable(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range zegVersionTiers {
		seen[row.Tier] = true
	}
	for _, want := range zegTierRequiredIDs {
		if !seen[want] {
			t.Fatalf("13 档表缺档位 %s（表内档位：%v）", want, zegTierIDs())
		}
	}
	allowed := map[string]bool{}
	for _, id := range zegTierRequiredIDs {
		allowed[id] = true
	}
	for _, row := range zegVersionTiers {
		if !allowed[row.Tier] {
			t.Fatalf("表里出现表外档位 %q（闭集 = D1–D13 · D6 拆两行 ⇒ 14 行）", row.Tier)
		}
		for _, cell := range []string{row.Trigger, row.Who, row.Msg, row.Basis, row.Now} {
			if strings.TrimSpace(cell) == "" {
				t.Fatalf("档 %s 有空格（触发 / 谁报 / 报话 / 依据 / 现读 五格必填）", row.Tier)
			}
		}
	}
	// ★ 退码逐档（用户 2026-10-15 拍定：D7/D8 归 1；D6b/D12 归 0；D10 归 8；D11 归 10；其余归 2）。
	want := map[string]int{
		"D1": exitUsage, "D2": exitUsage, "D3": exitUsage, "D4": exitUsage, "D5": exitUsage,
		"D6a": exitUsage, "D6b": exitOK, "D7": exitFail, "D8": exitFail, "D9": exitUsage,
		"D10": exitBlocked, "D11": exitResource, "D12": exitOK, "D13": exitUsage,
	}
	for tier, code := range want {
		row, ok := zegTierOf(tier)
		if !ok {
			t.Fatalf("取不到档 %s", tier)
		}
		if row.Code != code {
			t.Fatalf("档 %s 退码 = %d ≠ 表定/已拍 %d", tier, row.Code, code)
		}
	}
	// ★★ 逐字核用户拍定的那两格（✗ 不看上面那个映射表 —— 直接读真源行）
	if row, _ := zegTierOf("D7"); row.Code != exitFail {
		t.Fatalf("D7 退码 = %d ≠ 拍定 1", row.Code)
	}
	if row, _ := zegTierOf("D8"); row.Code != exitFail {
		t.Fatalf("D8 退码 = %d ≠ 拍定 1", row.Code)
	}
	t.Logf("13 档表：档位 %v · 各档退码 = %v", zegTierIDs(), func() []string {
		out := make([]string, 0, len(zegVersionTiers))
		for _, r := range zegVersionTiers {
			out = append(out, r.Tier+"="+itoaForLog(r.Code))
		}
		return out
	}())
}

// itoaForLog —— 小工具（✗ 不引 strconv：日志面上的整数格式化够用）。
func itoaForLog(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ---- ② 不可变面 / 可加面二分类表 ------------------------------------------------------------

func TestZegVersionSurfaceClassification(t *testing.T) {
	incompat := map[string]bool{}
	known := map[string]bool{}
	for _, s := range zegIndexKeySpecs {
		known[s.Key] = true
		if s.Surface == zegSurfaceIncompat {
			incompat["top."+s.Key] = true
		}
	}
	for _, s := range zegEntryKeySpecs {
		known[s.Key] = true
		if s.Surface == zegSurfaceIncompat {
			incompat["entry."+s.Key] = true
		}
	}
	// §11.2 不变量 7 + 设计稿 §16.5 R1 的不可变面清单（magic · 主号 · 对齐 · 段表逐字段偏移 · path 禁 ..）
	for _, k := range []string{
		"top.magic", "top.version", "top.align", "top.entries",
		"entry.path", "entry.off", "entry.size", "entry.sha256", "entry.mode",
	} {
		if !incompat[k] {
			t.Fatalf("不可变面缺键 %s（§11.2 不变量 7 · §十九 #29 · 照 erofs feature_incompat）", k)
		}
	}
	// 可加面样例**必须**是读端「未知键」（✗ 不得进已知键表 —— 进了就会被当不可变面判）
	for _, k := range zegAdditiveKeyExamples {
		if known[k] {
			t.Fatalf("可加面样例 %s 竟在已知键表里（那会把它按不可变面判）", k)
		}
	}
	// 每一行的 kind 必须落在四类闭集里 ＋ 依据非空
	kinds := map[string]bool{"string": true, "number": true, "array": true, "object": true}
	for _, s := range append(append([]zegKeySpec{}, zegIndexKeySpecs...), zegEntryKeySpecs...) {
		if !kinds[s.Kind] {
			t.Fatalf("键 %s 的 kind %q 不在闭集（string/number/array/object）", s.Key, s.Kind)
		}
		if strings.TrimSpace(s.Why) == "" {
			t.Fatalf("键 %s 没写依据（逐条可读是这张表的意义）", s.Key)
		}
		if s.Surface != zegSurfaceIncompat && s.Surface != zegSurfaceCompat {
			t.Fatalf("键 %s 的面归属 %q 不在闭集（incompat/compat）", s.Key, s.Surface)
		}
	}
}

// ---- ③ 机检物导出（表 ↦ JSON）--------------------------------------------------------------

func TestZegVersionCompatTableJSON(t *testing.T) {
	raw, err := zegCompatTableJSON()
	if err != nil {
		t.Fatalf("导表失败：%v", err)
	}
	var doc struct {
		Schema          string `json:"schema"`
		MainVersion     int    `json:"main_version"`
		MinReadable     int    `json:"min_readable"`
		SigBlockVersion int    `json:"sig_block_main_version"`
		Tiers           []struct {
			Tier string `json:"tier"`
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Who  string `json:"who"`
		} `json:"tiers"`
		AdditiveExamples []string `json:"additive_key_examples"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("导出的 JSON 不可解析：%v", err)
	}
	if doc.Schema != "zerg-egg-compat/1" {
		t.Fatalf("schema 不符：%q", doc.Schema)
	}
	if doc.MainVersion != zegFormatVer || doc.MinReadable != zegMinReadableVersion {
		t.Fatalf("导出的主号/最低可读与常量不符：%d / %d", doc.MainVersion, doc.MinReadable)
	}
	if doc.SigBlockVersion != zegFormatVersionSigBlock {
		t.Fatalf("导出的签名块主号 = %d ≠ %d", doc.SigBlockVersion, zegFormatVersionSigBlock)
	}
	codes := map[string]int{}
	for _, row := range doc.Tiers {
		codes[row.Tier] = row.Code
		if strings.TrimSpace(row.Msg) == "" || strings.TrimSpace(row.Who) == "" {
			t.Fatalf("导出档 %s 缺报话或谁报", row.Tier)
		}
	}
	for _, id := range zegTierRequiredIDs {
		if _, ok := codes[id]; !ok {
			t.Fatalf("导出 JSON 缺档位 %s", id)
		}
	}
	if codes["D7"] != exitFail || codes["D8"] != exitFail {
		t.Fatalf("导出 JSON 里 D7/D8 = %d/%d ≠ 拍定 1", codes["D7"], codes["D8"])
	}
	seenAdditive := false
	for _, k := range doc.AdditiveExamples {
		seenAdditive = seenAdditive || strings.TrimSpace(k) != ""
	}
	if !seenAdditive {
		t.Fatalf("导出 JSON 里可加面样例为空（二分类的另一半没写成机检物）")
	}
	tierN := len(doc.Tiers)
	addN := len(doc.AdditiveExamples)
	t.Logf("机检物 JSON：schema=%s 主号=%d 最低可读=%d 签名块主号=%d 档位=%d 可加面样例=%d",
		doc.Schema, doc.MainVersion, doc.MinReadable, doc.SigBlockVersion, tierN, addN)
}

// ---- ④ 四态亲测（真跑 · 真读数 · 走真命令面）--------------------------------------------------

func TestZegVersionFourStates(t *testing.T) {
	dir := t.TempDir()
	base := zegTMakeProbeEgg(t, dir)

	// ---- 态①：同主号**可加面**未知段 ⇒ 旧读端**照旧跑**（✗ 不 error）----
	addEgg := zegTCloneEggWithIdx(t, base, filepath.Join(dir, "additive.egg"), func(m map[string]json.RawMessage) error {
		m["min_readable"] = json.RawMessage("1")     // 可加面键（值＝本端主号 ⇒ ✗ 不该拒）
		m["wall_min_version"] = json.RawMessage("1") // 可加面键（§16.5 V2）
		var ents []map[string]json.RawMessage
		if err := json.Unmarshal(m["entries"], &ents); err != nil {
			return err
		}
		ents[0]["mediaType"] = json.RawMessage(`"application/vnd.zerg.weights.v9+unknown"`) // 新增非必需段类型
		eb, err := json.Marshal(ents)
		if err != nil {
			return err
		}
		m["entries"] = eb
		return nil
	}, 0)
	var o1, e1 strings.Builder
	rc1 := zergEggGateMain(&o1, &e1, []string{"verify", addEgg})
	audit, aerr := zegCompatGateIndexBody(zegTIdxBody(t, addEgg))
	if aerr != nil {
		t.Fatalf("态① 面审计必须不报错（照 OCI「MUST NOT error」）：%v", aerr)
	}
	if rc1 != exitOK || strings.TrimSpace(e1.String()) != "" {
		t.Fatalf("态① 可加面档 rc=%d stderr=%q（✗ 应为照旧跑 rc=0 且不出声）", rc1, e1.String())
	}
	if !strings.Contains(o1.String(), "✓ 校验通过") {
		t.Fatalf("态① 没走到校验通过那一行：%q", o1.String())
	}
	t.Logf("态① rc=%d · stderr=%q · stdout 末行=%q · 可加面登记面=顶层 %v ＋ 段 %v",
		rc1, e1.String(), lastLine(o1.String()), audit.AdditiveTopKeys, audit.AdditiveEntryKeys)

	// ---- 态②：同主号**不可变面改语义** ⇒ **必拒** ----
	semEgg := zegTCloneEggWithIdx(t, base, filepath.Join(dir, "samekey.egg"), func(m map[string]json.RawMessage) error {
		m["align"] = json.RawMessage(`"4096"`) // 同一键换语义：number → string
		return nil
	}, 0)
	var o2, e2 strings.Builder
	rc2 := zergEggGateMain(&o2, &e2, []string{"info", semEgg})
	_, semErr := zegParseIndex(semEgg)
	tier2 := tierOfErr(semErr)
	if rc2 == exitOK || semErr == nil {
		t.Fatalf("态② 同键换语义竟被放行（rc=%d err=%v）", rc2, semErr)
	}
	if tier2 != zegTierSurface {
		t.Fatalf("态② 档位 = %q ≠ %q（✗ 不可变面不符必须点名档位）", tier2, zegTierSurface)
	}
	// ★ 卵族命令面（`zegCmdInfo`）把读端错误打在 **stdout**（人面读数走 stdout，机器面另有包封）
	msg2 := strings.TrimSpace(o2.String() + e2.String())
	if !strings.Contains(msg2, "同一键换了语义") {
		t.Fatalf("态② 报话没点名「同一键换了语义」：%q", msg2)
	}
	t.Logf("态② rc=%d · 档位=%s · 人面=%q", rc2, tier2, msg2)

	// ---- 态③：`min_readable` 超本端（不可变面不符）⇒ **必拒且点名** ----
	minEgg := zegTCloneEggWithIdx(t, base, filepath.Join(dir, "minread.egg"), func(m map[string]json.RawMessage) error {
		m["min_readable"] = json.RawMessage(itoaForLog(zegFormatVersionSigBlock)) // 卵要「更新世代的读端」
		return nil
	}, 0)
	var o3, e3 strings.Builder
	rc3 := zergEggGateMain(&o3, &e3, []string{"info", minEgg})
	_, minErr := zegParseIndex(minEgg)
	tier3 := tierOfErr(minErr)
	if rc3 == exitOK || minErr == nil {
		t.Fatalf("态③ min_readable 超本端竟被放行（rc=%d err=%v）", rc3, minErr)
	}
	msg3 := strings.TrimSpace(o3.String() + e3.String())
	if tier3 != "D3" || !strings.Contains(msg3, "min_readable") {
		t.Fatalf("态③ 档位=%q 报话=%q（须点名 D3 ＋ min_readable）", tier3, msg3)
	}
	t.Logf("态③ rc=%d · 档位=%s · 人面=%q", rc3, tier3, msg3)

	// ---- 态④：主号 3（签名块世代）⇒ 旧读端**必拒** ----
	sigEgg := zegTCloneEggWithIdx(t, base, filepath.Join(dir, "sigblock.egg"), func(m map[string]json.RawMessage) error {
		return nil
	}, zegFormatVersionSigBlock)
	var o4, e4 strings.Builder
	rc4 := zergEggGateMain(&o4, &e4, []string{"info", sigEgg})
	_, sigErr := zegParseIndex(sigEgg)
	tier4 := tierOfErr(sigErr)
	if rc4 == exitOK || sigErr == nil {
		t.Fatalf("态④ 主号 %d（签名块世代）竟被旧读端放行（rc=%d err=%v）✗ 不是忽略", zegFormatVersionSigBlock, rc4, sigErr)
	}
	msg4 := strings.TrimSpace(o4.String() + e4.String())
	if tier4 != "D2" || !strings.Contains(msg4, "签名块 v2") {
		t.Fatalf("态④ 档位=%q 报话=%q（须点名 D2 ＋ 签名块 v2）", tier4, msg4)
	}
	t.Logf("态④ rc=%d · 档位=%s · 人面=%q", rc4, tier4, msg4)
}

// zegTIdxBody —— 抠出索引体（真读数用）。
func zegTIdxBody(t *testing.T, egg string) []byte {
	t.Helper()
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	return d[zegHeaderPrefix : zegHeaderPrefix+zegIdxLenOf(d)]
}

// tierOfErr —— 从错误里取档位号（非 `*zegRefusal` ⇒ "<非拒>"）。
func tierOfErr(err error) string {
	if err == nil {
		return "<放行>"
	}
	if ref, ok := err.(*zegRefusal); ok {
		return ref.Tier
	}
	return "<非拒>"
}

// lastLine —— 取最后一行（日志面用）。
func lastLine(s string) string {
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return parts[len(parts)-1]
}

// ---- ⑤ 负控：哑化 `min_readable` 那条 ⇒ 态③ 必红（两态对拍）---------------------------------

func TestZegVersionMinReadableNegativeControl(t *testing.T) {
	dir := t.TempDir()
	base := zegTMakeProbeEgg(t, dir)
	minEgg := zegTCloneEggWithIdx(t, base, filepath.Join(dir, "minread.egg"), func(m map[string]json.RawMessage) error {
		m["min_readable"] = json.RawMessage(itoaForLog(zegFormatVersionSigBlock))
		return nil
	}, 0)

	// 有牙档：拒 D3（本端主号 1 < 卵声明 2）
	if err := zegCompatGateMinReadable(zegFormatVersionSigBlock, zegFormatVer); err == nil {
		t.Fatalf("有牙档竟放行 min_readable=%d > 本端 %d", zegFormatVersionSigBlock, zegFormatVer)
	} else if ref, ok := err.(*zegRefusal); !ok || ref.Tier != "D3" {
		t.Fatalf("有牙档档位不符：%v", err)
	}
	// ★ 改前形态：同一输入（主号 1）**必须放行** —— 逐字等于改前那条「有声明无牙」
	if err := zegCompatGateLegacy(zegFormatVer); err != nil {
		t.Fatalf("改前形态在同一输入上应放行（＝片5 F1 的永真无牙），实得 %v", err)
	}
	// ★ 改前形态对**主号不符**的报话必须逐字复刻（改前那一条的原文）
	if got := zegCompatGateLegacy(zegFormatVersionSigBlock); got == nil {
		t.Fatalf("改前形态对主号 %d 应拒", zegFormatVersionSigBlock)
	} else if want := fmt.Sprintf("格式版本不支持：%d", zegFormatVersionSigBlock); got.Error() != want {
		t.Fatalf("改前形态报话 = %q ≠ 改前原文 %q", got.Error(), want)
	}
	// 两态对拍：牙 ⇒ 拒；改前形态 ⇒ 放行 —— 两个结论必须不同（否则这枚牙是永真的）
	_, errTooth := zegParseIndex(minEgg)
	if errTooth == nil {
		t.Fatalf("态③ 经真读端竟放行（牙没接上 zegParseIndex？）")
	}
	t.Logf("负控两态：有牙 ⇒ 拒[%s] · 改前形态 ⇒ 放行（＝永真无牙）· 两结论不同 ⇒ 哑化该条本件必红",
		tierOfErr(errTooth))
}

// ---- ⑥ `--self-test` 链尾那一行（真入口）-----------------------------------------------------

func TestZegVersionSelfTestChain(t *testing.T) {
	var out, errb strings.Builder
	rc := zergEggGateMain(&out, &errb, []string{"--self-test"})
	if rc != exitOK {
		t.Fatalf("`--self-test` rc=%d stderr=%q", rc, errb.String())
	}
	// ★ 旧档那一行（py 真搬面 · 10 项）是钉住的契约读数 —— 本单 ✗ 不动它
	if !strings.Contains(out.String(), "self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）") {
		t.Fatalf("旧档自检行不在了（✗ 本单不许动它）：%q", out.String())
	}
	// ★ 卵3① 那行也在
	if !strings.Contains(out.String(), "self-test（卵3① 并行＋缓存）通过") {
		t.Fatalf("卵3① 自检行不在了：%q", out.String())
	}
	// ★ 本单那一行
	if !strings.Contains(out.String(), "self-test（卵6 版本与兼容）通过（4 态 ＋ 1 负控）") {
		t.Fatalf("卵6 自检行缺失：%q", out.String())
	}
	for _, frag := range []string{"① 可加面未知段 ⇒ 照旧跑", "② 同键换语义",
		fmt.Sprintf("③ min_readable=%d > 本端 %d ⇒ 拒[D3]", zegFormatVersionSigBlock, zegFormatVer),
		fmt.Sprintf("④ 主号 %d 签名块 ⇒ 拒[D2]", zegFormatVersionSigBlock), "负控"} {
		if !strings.Contains(out.String(), frag) {
			t.Fatalf("卵6 自检行缺片段 %q：%q", frag, out.String())
		}
	}
	t.Logf("`--self-test` 读数：rc=%d · 卵6 那行=%q", rc, lastLine(out.String()))
}

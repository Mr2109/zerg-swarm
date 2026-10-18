// zerg_egg_tier_gap24_test.go —— `GAP-20261015-24` 的判据件（D1 / D4 / D13 三档 · **两态**）。
//
// 缺口：卵族读端 13 档报话/退码与设计建议表系统性差，其中 **D1 / D4 / D13 三档「两差并存」**
// （报话文案与退码都不对）。本件只判这三档 —— 其余 10 档 ✗ 不判（本单不修，见账）。
//
// 设计真源（设计稿 §十六 16.5 · 13 档表 928–943 行；★ 只按节引，✗ 按行号锚定）：
//
//	D1  拒孵：不是卵（magic 期望 ZEGG，实读十六进制）                                          退码 2
//	D4  拒孵：align 不合法（须 2 的幂且 ≥4096）／段偏移未对齐                                   退码 2
//	D13 拒孵：索引 JSON 不可解析；文件保持不动                                                  退码 2
//
// ★ 上面的设计原文里 D1 的「实读十六进制」是**占位**（同表 D7 的「期望 a，实读 b」亦然）⇒ 本仓落成
// `实读 %s` 模板，`%s` 填**实读 4B 的小写十六进制**。判据按「模板 ＋ 填值」逐字对，✗ 不按占位字面。
//
// 两态口径（逐档各三格）：
//
//	① **改前**：旧报话逐字（`legacy`）＋ 旧退码 1 —— 本件判它**必须已消失**（旧串一旦回到输出即红；
//	   这是「变异即红」的反向钉子：把生产件改回旧形态，本件立刻红）；
//	② **改后报话**：命令面第一行必须**逐字等于** `zerg-egg.py: ` ＋ 设计报话（模板填值后的整串）；
//	③ **改后退码**：`zergEggGateMain` 的 rc 必须等于 13 档表该行 `Code`（= 2），✗ 不得再是 1。
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegTierProbe —— 一档的一枚夹具卵 ＋ 两态读数。
type zegTierProbe struct {
	Tier string
	Name string
	// 夹具：v1 固定前缀 ＋ 索引体（`prefixMagic` 缺省 ZEGG）。
	PrefixMagic string
	Align       uint64
	Body        []byte
	RawBytes    []byte // 非空 ⇒ 直接落这些字节（用来造「前缀 < 32B」）
	// 设计面（逐字取自设计稿 §十六 16.5 · 13 档表）：报话模板 ＋ 填值。
	DesignMsg  string
	DesignArgs []any
	// 改前态：旧报话片段（必须已消失）＋ 旧退码。
	LegacyMsg  string
	LegacyCode int
}

// zegTierProbes —— 三档 6 个夹具（D1 两处触发 · D4 两处触发 · D13 一处 ＋ 索引体 magic 一处）。
func zegTierProbes() []zegTierProbe {
	v1Body := func(align string, magic string) []byte {
		return []byte(fmt.Sprintf(`{"magic":%q,"version":1,"align":%s,"entries":[]}`, magic, align))
	}
	return []zegTierProbe{
		{ // D1-①：前 4B ≠ ZEGG（前缀档）
			Tier: "D1", Name: "magic-前缀档",
			PrefixMagic: "XXXX", Align: 4096, Body: v1Body("4096", "XXXX"),
			DesignMsg: "拒孵：不是卵（magic 期望 ZEGG，实读 %s）", DesignArgs: []any{hex.EncodeToString([]byte("XXXX"))},
			LegacyMsg: "magic 不符（不是 ZEGG 文件）", LegacyCode: exitFail,
		},
		{ // D1-②：前缀 < 32B（同一档 · 改前是另一句「文件太短」）
			Tier: "D1", Name: "前缀短于32B",
			RawBytes:  append([]byte(zegMagic), 1, 0, 0, 0),
			DesignMsg: "拒孵：不是卵（magic 期望 ZEGG，实读 %s）", DesignArgs: []any{hex.EncodeToString([]byte(zegMagic))},
			LegacyMsg: "文件太短，不足固定前缀", LegacyCode: exitFail,
		},
		{ // D1-③：前缀 magic 对、索引体 magic 换掉（索引体档）
			Tier: "D1", Name: "magic-索引体档",
			PrefixMagic: "ZEGG", Align: 4096, Body: v1Body("4096", "XEGG"),
			DesignMsg: "拒孵：不是卵（magic 期望 ZEGG，实读 %s）", DesignArgs: []any{hex.EncodeToString([]byte("XEGG"))},
			LegacyMsg: "magic 不符（不是 ZEGG 文件）", LegacyCode: exitFail,
		},
		{ // D4-①：前缀里的 align 非 2 的幂
			Tier: "D4", Name: "align-前缀档",
			PrefixMagic: "ZEGG", Align: 3000, Body: v1Body("4096", "ZEGG"),
			DesignMsg: "拒孵：align 不合法（须 2 的幂且 ≥4096）／段偏移未对齐",
			LegacyMsg: "前缀里的 align 不合理", LegacyCode: exitFail,
		},
		{ // D4-②：索引体里的 align 非法（前缀合法）
			Tier: "D4", Name: "align-索引体档",
			PrefixMagic: "ZEGG", Align: 4096, Body: v1Body("3000", "ZEGG"),
			DesignMsg: "拒孵：align 不合法（须 2 的幂且 ≥4096）／段偏移未对齐",
			LegacyMsg: "align 字段不合理", LegacyCode: exitFail,
		},
		{ // D13：索引 JSON 不可解析
			Tier: "D13", Name: "索引JSON不可解析",
			PrefixMagic: "ZEGG", Align: 4096, Body: []byte("{not json"),
			DesignMsg: "拒孵：索引 JSON 不可解析；文件保持不动",
			LegacyMsg: "拒孵：索引 JSON 不可解析（", LegacyCode: exitFail,
		},
	}
}

// zegTierWriteProbe —— 落一枚夹具卵（v1 固定前缀 32B ＋ 变长索引体）。
func zegTierWriteProbe(t *testing.T, dir string, p zegTierProbe) string {
	t.Helper()
	path := filepath.Join(dir, p.Tier+"-"+p.Name+".egg")
	if p.RawBytes != nil {
		if err := os.WriteFile(path, p.RawBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	magic := p.PrefixMagic
	if magic == "" {
		magic = zegMagic
	}
	hdr := make([]byte, zegHeaderPrefix)
	copy(hdr[0:4], magic)
	binary.LittleEndian.PutUint32(hdr[4:8], zegFormatVerV1)
	binary.LittleEndian.PutUint64(hdr[8:16], p.Align)
	binary.LittleEndian.PutUint64(hdr[16:24], uint64(len(p.Body)))
	if err := os.WriteFile(path, append(hdr, p.Body...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestZergEggTierGap24D1D4D13TwoStates —— 三档 × 两态（改前/改后）逐字判据。
//
// ★ 判据有牙的两条：① 报话整行**相等**（不是 Contains —— 少一个字就红）；② 退码**相等**到 13 档表 `Code`；
// 并且 ③ 旧串必须**不在**输出里（把生产件改回旧形态 ⇒ 这里当场红）。
func TestZergEggTierGap24D1D4D13TwoStates(t *testing.T) {
	dir := t.TempDir()
	for _, p := range zegTierProbes() {
		p := p
		t.Run(p.Tier+"/"+p.Name, func(t *testing.T) {
			path := zegTierWriteProbe(t, dir, p)

			// 13 档表这一行必须与设计面同源（改表未同步 ⇒ 红）。
			row, ok := zegTierOf(p.Tier)
			if !ok {
				t.Fatalf("13 档表里没有 %s", p.Tier)
			}
			if row.Msg != p.DesignMsg {
				t.Fatalf("表内报话 %q ≠ 设计报话 %q（✗ 表与设计脱钩）", row.Msg, p.DesignMsg)
			}
			if row.Code != exitUsage {
				t.Fatalf("%s 的表内退码 = %d ≠ 设计 2", p.Tier, row.Code)
			}
			want := fmt.Sprintf(p.DesignMsg, p.DesignArgs...)

			// ---- 改后态：info 面 ----
			var o, e bytes.Buffer
			rc := zergEggGateMain(&o, &e, []string{"info", path})
			out := o.String() + e.String()
			if rc != row.Code {
				t.Fatalf("%s 改后退码 = %d ≠ 设计表 %d（改前 = %d）\n输出：%s", p.Tier, rc, row.Code, p.LegacyCode, out)
			}
			if rc == p.LegacyCode {
				t.Fatalf("%s 退码仍是改前的 %d（✗ 退码没对齐设计表）", p.Tier, p.LegacyCode)
			}
			first := strings.SplitN(strings.TrimRight(out, "\n"), "\n", 2)[0]
			wantLine := zegProg + ": " + want
			if first != wantLine {
				t.Fatalf("%s 改后报话不逐字：\n 得到 %q\n 期望 %q\n整段：%s", p.Tier, first, wantLine, out)
			}
			if strings.Contains(out, p.LegacyMsg) {
				t.Fatalf("%s 旧报话 %q 仍在输出里（改前态回来了 ⇒ 判红）\n整段：%s", p.Tier, p.LegacyMsg, out)
			}

			// ---- 改后态：verify 面（同一读端 · 同一档位/退码 —— ✗ 两个子命令各报一套）----
			var o2, e2 bytes.Buffer
			rc2 := zergEggGateMain(&o2, &e2, []string{"verify", path})
			out2 := o2.String() + e2.String()
			if rc2 != row.Code {
				t.Fatalf("%s verify 面退码 = %d ≠ 设计表 %d\n输出：%s", p.Tier, rc2, row.Code, out2)
			}
			if !strings.Contains(out2, want) {
				t.Fatalf("%s verify 面报话不含设计串 %q\n整段：%s", p.Tier, want, out2)
			}
		})
	}
}

// TestZergEggTierGap24ScopeGuard —— 本单的**面界**判据：只有已接码的档走表码，其余档仍走既有 1 通道。
// ★ 它防两件事：① 「顺手把其余 10 档也改了」（越界）；② 把 `Coded` 的零值当成码 0（= 假绿）放行。
func TestZergEggTierGap24ScopeGuard(t *testing.T) {
	// 未接码的拒（D2/D3/D9 等）⇒ 仍旧 1。
	if rc := zegEggErrExit(&zegRefusal{Tier: "D2", Msg: "x"}); rc != exitFail {
		t.Fatalf("未接码的档竟退 %d（要 %d —— 本单 ✗ 不扩面）", rc, exitFail)
	}
	// 普通错误 ⇒ 仍旧 1。
	if rc := zegEggErrExit(fmt.Errorf("boom")); rc != exitFail {
		t.Fatalf("普通错误竟退 %d（要 %d）", rc, exitFail)
	}
	// 三档必须带码，且码 == 表内 Code。
	for _, tier := range []string{"D1", "D4", "D13"} {
		row, _ := zegTierOf(tier)
		if rc := zegEggErrExit(zegTierRefuse(tier, "probe")); rc != row.Code {
			t.Fatalf("%s 带码拒退 %d ≠ 表内 %d", tier, rc, row.Code)
		}
	}
}

// TestZergEggTierGap24DetailKept —— 细节读数不许被「逐字化报话」吃掉：报话行之后必须另起一行给细节。
func TestZergEggTierGap24DetailKept(t *testing.T) {
	var o, e bytes.Buffer
	rc := zergEggGateMain(&o, &e, []string{"info", filepath.Join(t.TempDir(), "missing.egg")})
	if rc == exitOK {
		t.Fatalf("不存在的件竟退 0")
	}
	if !strings.Contains(e.String(), "no such file") && !strings.Contains(o.String(), "no such file") {
		t.Fatalf("真错因（no such file）没出现在读数里：stdout=%q stderr=%q", o.String(), e.String())
	}
}

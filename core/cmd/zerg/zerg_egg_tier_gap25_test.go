// zerg_egg_tier_gap25_test.go —— `GAP-20261015-25` 的判据件（**D9 / D2 / D7 三档 · 两态**）。
//
// 缺口（账内 `GAP-20261015-25`）：卵族 13 档里 **D9（段 path 非法 · 码 1≠2）**、**D2（前向拒 · 报话＋码
// 两差）**、**D7（段 sha256 不符 · 报话差）** 三档与设计表不一致。本件只判这三档 —— 其余档 ✗ 不判（见账）。
//
// 设计真源（设计稿 §十六 16.5 · 13 档表；★ 只按节引，✗ 按行号锚定）：
//
//	D9  拒孵：段 path 非法（须规范化相对路径，禁绝对/盘符/空段/..）                     退码 2
//	D2  拒孵：卵格式 version 高于本机认知 —— 不按新格式猜着跑；请升级孵化器（或用卵内茧壁兜底）  退码 2
//	D7  拒孵：段内容 sha256 与索引不符（期望 a，实读 b）                                退码 1
//
// ★ D7 的 `a`/`b` 与 D1 的「实读十六进制」同属**占位词**（`GAP-20261015-24` 对拍表 §五-④）⇒ 本仓落成
// `%s` 模板，两个 `%s` 分别填**期望 sha256**（索引里的值）与**实读 sha256**（现算值）。判据按「模板 ＋ 填值」
// 逐字对，✗ 不按占位字面。
//
// 两态口径（逐档）：
//
//	① **改前**：旧报话 ＋ 旧退码 —— 本件用**同一把尺**（`zegGap25Check`）判它必须红（两态对拍：尺若对
//	   改前读数也判绿，则这条判据无牙 ⇒ 本件当场红）；
//	② **改后报话**：命令面第一行**逐字等于** `zerg-egg.py: ` ＋ 设计报话（✗ 用 Contains —— 少一个字就红）；
//	③ **改后退码**：＝ 13 档表该行 `Code`（D9/D2 = 2；D7 = 1）。
//
// ★ 设计面常量**独立写在本件**（✗ 从生产表反读）：生产表被改成「另一种说法」时，`TestZergEggTierGap25Table`
// 当场红 —— 判据不许拿被测对象自己判自己。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- 设计面（逐字 · 独立于生产表）-------------------------------------------------------------

const (
	zegGap25DesignD9 = "拒孵：段 path 非法（须规范化相对路径，禁绝对/盘符/空段/..）"
	zegGap25DesignD2 = "拒孵：卵格式 version 高于本机认知 —— 不按新格式猜着跑；请升级孵化器（或用卵内茧壁兜底）"
	// ★ `%s`/`%s` ＝ 期望 / 实读 sha256（占位词 a/b 的落法，见件头）。
	zegGap25DesignD7 = "拒孵：段内容 sha256 与索引不符（期望 %s，实读 %s）"
)

// ---- 夹具（v1 布局：32 B 固定前缀 ＋ 变长索引体 ＋ 可选段数据区）----------------------------------

// zegGap25Egg —— 一枚夹具卵的三格（前缀主号 / 索引体主号 ／ 索引体与段数据）。
type zegGap25Egg struct {
	PrefixVer int
	BodyVer   int
	Body      []byte
	SegData   []byte
}

// zegGap25Body —— 组索引体 JSON（只带不可变面四键 ＋ 可选 entries）。
func zegGap25Body(ver int, entries string) []byte {
	if entries == "" {
		entries = "[]"
	}
	return []byte(fmt.Sprintf(`{"magic":"ZEGG","version":%d,"align":%d,"entries":%s}`, ver, zegMinAlign, entries))
}

// zegGap25Entry —— 组一条段表项（off 恒落在第一个对齐边界上）。
func zegGap25Entry(path, sha string, size int64) string {
	return fmt.Sprintf(`{"path":%q,"off":%d,"size":%d,"sha256":%q,"mode":420}`, path, zegMinAlign, size, sha)
}

// zegGap25Write —— 落一枚夹具卵（头块整块 = align 字节 ⇒ 段偏移一位不动）。
func zegGap25Write(t *testing.T, dir, name string, e zegGap25Egg) string {
	t.Helper()
	align := int64(zegMinAlign)
	if int64(zegHeaderPrefix)+int64(len(e.Body)) > align {
		t.Fatalf("夹具坏：索引体 %d B 放不进 %d B 头块", len(e.Body), align)
	}
	hdr := make([]byte, align)
	copy(hdr[0:4], zegMagic)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(e.PrefixVer))
	binary.LittleEndian.PutUint64(hdr[8:16], uint64(align))
	binary.LittleEndian.PutUint64(hdr[16:24], uint64(len(e.Body)))
	copy(hdr[zegHeaderPrefix:], e.Body)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, append(hdr, e.SegData...), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// zegGap25SHA —— 十六进制 sha256（夹具与期望值共用）。
func zegGap25SHA(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// ---- 唯一一把尺：报话逐字 ＋ 退码对齐（两态共用 ⇒ 尺若无牙本件自证）------------------------------

// zegGap25Check —— `first`（命令面第一行 / problems 行）必须**逐字等于** `wantLine`，`rc` 必须等于 `wantRC`。
// 返回 nil 仅当两条同时成立。★ 两态对拍靠它：真读数 ⇒ nil；改前读数 ⇒ 非 nil（否则判据无牙）。
func zegGap25Check(first string, rc int, wantLine string, wantRC int) error {
	if first != wantLine {
		return fmt.Errorf("报话不逐字：\n 得到 %q\n 期望 %q", first, wantLine)
	}
	if rc != wantRC {
		return fmt.Errorf("退码 = %d ≠ 设计表 %d", rc, wantRC)
	}
	return nil
}

// zegGap25FirstLine —— 命令面输出第一行。
func zegGap25FirstLine(out string) string {
	return strings.SplitN(strings.TrimRight(out, "\n"), "\n", 2)[0]
}

// zegGap25ProblemLine —— verify 面 problems 段里以 `   - ` 开头的那一行（去掉前缀；找不到 ⇒ 空串）。
func zegGap25ProblemLine(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "   - ") {
			return strings.TrimPrefix(ln, "   - ")
		}
	}
	return ""
}

// ---- 三档两态 --------------------------------------------------------------------------------

// TestZergEggTierGap25D9D2D7 —— 三档 × 两态（改前必须被同一把尺判红；改后必须逐字命中设计面）。
func TestZergEggTierGap25D9D2D7(t *testing.T) {
	dir := t.TempDir()
	zero16 := make([]byte, 16)
	emptySHA := zegGap25SHA(nil) // 空载荷 sha256（16B 零段的 sha 另算）

	// ============ D9 · 段 path 非法（`..` 穿越 / 绝对路径两处触发）============
	d9Probes := []struct {
		Name    string
		Path    string
		Rule    string
		WantSeg string
	}{
		{"点段穿越", "../x", "path_dotdot", ""},
		{"绝对路径", "/etc/passwd", "path_absolute", ""},
	}
	for _, p := range d9Probes {
		p := p
		t.Run("D9/"+p.Name, func(t *testing.T) {
			egg := zegGap25Write(t, dir, "d9-"+p.Rule+".egg", zegGap25Egg{
				PrefixVer: zegFormatVerV1, BodyVer: zegFormatVerV1,
				Body:    zegGap25Body(zegFormatVerV1, "["+zegGap25Entry(p.Path, emptySHA, 16)+"]"),
				SegData: zero16,
			})
			var o, e bytes.Buffer
			rc := zergEggGateMain(&o, &e, []string{"info", egg})
			out := o.String() + e.String()
			first := zegGap25FirstLine(out)
			wantLine := zegProg + ": " + zegGap25DesignD9

			if err := zegGap25Check(first, rc, wantLine, exitUsage); err != nil {
				t.Fatalf("D9（%s）改后不达标：%v\n整段输出：%s", p.Rule, err, out)
			}
			// 规则名不许丢（只是从报话移进细节行）。
			if !strings.Contains(out, p.Rule) {
				t.Fatalf("D9 细节行没点名规则 %s（✗ 规则不可判）：%s", p.Rule, out)
			}
			if strings.Contains(out, "拒：段 path") {
				t.Fatalf("D9 旧报话「拒：段 path」仍在输出里（改前形态回潮）：%s", out)
			}
			// ★ 两态：改前读数（旧报话 ＋ 旧退码 1）必须被**同一把尺**判红。
			legacy := fmt.Sprintf("%s: 拒：段 path %q（须为规范化 NFC 相对路径） —— 命中规则 %s（…）\n",
				zegProg, p.Path, p.Rule)
			if err := zegGap25Check(zegGap25FirstLine(legacy), exitFail, wantLine, exitUsage); err == nil {
				t.Fatalf("D9 判据对改前读数竟判绿（无牙）")
			}
		})
	}

	// D9 的 verify 面同形（同一读端 · 同一档位/退码 —— ✗ 两个子命令各报一套）。
	t.Run("D9/verify面同形", func(t *testing.T) {
		egg := zegGap25Write(t, dir, "d9-verify.egg", zegGap25Egg{
			PrefixVer: zegFormatVerV1, BodyVer: zegFormatVerV1,
			Body:    zegGap25Body(zegFormatVerV1, "["+zegGap25Entry("../y", emptySHA, 16)+"]"),
			SegData: zero16,
		})
		var o, e bytes.Buffer
		rc := zergEggGateMain(&o, &e, []string{"verify", egg})
		out := o.String() + e.String()
		if err := zegGap25Check(zegGap25FirstLine(out), rc, zegProg+": "+zegGap25DesignD9, exitUsage); err != nil {
			t.Fatalf("D9 verify 面不达标：%v\n整段输出：%s", err, out)
		}
	})

	// ============ D2 · 前向拒（主号 3 = 签名块世代）============
	t.Run("D2/主号3签名块世代", func(t *testing.T) {
		egg := zegGap25Write(t, dir, "d2-v3.egg", zegGap25Egg{
			PrefixVer: zegFormatVersionSigBlock, BodyVer: zegFormatVersionSigBlock,
			Body: zegGap25Body(zegFormatVersionSigBlock, ""),
		})
		var o, e bytes.Buffer
		rc := zergEggGateMain(&o, &e, []string{"info", egg})
		out := o.String() + e.String()
		first := zegGap25FirstLine(out)
		if err := zegGap25Check(first, rc, zegProg+": "+zegGap25DesignD2, exitUsage); err != nil {
			t.Fatalf("D2 改后不达标：%v\n整段输出：%s", err, out)
		}
		// 读数不丢：版本、主号、签名块 v2 世代、判在前缀 —— 全在细节行。
		for _, frag := range []string{"签名块 v2", fmt.Sprintf("version=%d", zegFormatVersionSigBlock), "[前缀]"} {
			if !strings.Contains(out, frag) {
				t.Fatalf("D2 细节行缺读数 %q（✗ 对表 ≠ 丢诊断）：%s", frag, out)
			}
		}
		if strings.Contains(first, "前向拒（★") {
			t.Fatalf("D2 报话仍是改前那串（前向拒（★ …））：%q", first)
		}
		// ★ 两态：改前读数（长串报话 ＋ 退码 1）必须被判红。
		legacy := fmt.Sprintf("%s: 拒孵：卵格式 version=%d 高于本机认知 %d —— 前向拒（★ 签名块 v2 世代 ＝ 主号 ≥ %d（主号 2 是段表化）⇒ 旧读端**必拒** …）[前缀]\n",
			zegProg, zegFormatVersionSigBlock, zegFormatVer, zegFormatVersionSigBlock)
		if err := zegGap25Check(zegGap25FirstLine(legacy), exitFail, zegProg+": "+zegGap25DesignD2, exitUsage); err == nil {
			t.Fatalf("D2 判据对改前读数竟判绿（无牙）")
		}
	})

	// ============ D7 · 段内容 sha256 与索引不符 ============
	t.Run("D7/段内容sha256不符", func(t *testing.T) {
		wantData := zero16
		gotData := append([]byte{}, zero16...)
		gotData[0] ^= 0xff // 段内改一字节（✗ 改 pad：末段后的补零不参与判）
		egg := zegGap25Write(t, dir, "d7-corrupt.egg", zegGap25Egg{
			PrefixVer: zegFormatVerV1, BodyVer: zegFormatVerV1,
			Body:    zegGap25Body(zegFormatVerV1, "["+zegGap25Entry("model/w.bin", zegGap25SHA(wantData), int64(len(wantData)))+"]"),
			SegData: gotData,
		})
		var o, e bytes.Buffer
		rc := zergEggGateMain(&o, &e, []string{"verify", egg})
		out := o.String() + e.String()
		wantLine := "model/w.bin: " + fmt.Sprintf(zegGap25DesignD7, zegGap25SHA(wantData), zegGap25SHA(gotData))
		line := zegGap25ProblemLine(out)
		if err := zegGap25Check(line, rc, wantLine, exitFail); err != nil {
			t.Fatalf("D7 改后不达标：%v\n整段输出：%s", err, out)
		}
		if strings.Contains(out, "sha256 不符") {
			t.Fatalf("D7 旧报话「sha256 不符」仍在输出里（改前形态回潮）：%s", out)
		}
		// ★ 两态：改前读数（`<path> sha256 不符` ＋ 退码 1 —— 码本来就对，报话不对）必须被判红。
		legacy := "   - model/w.bin sha256 不符\n"
		if err := zegGap25Check(zegGap25ProblemLine(legacy), exitFail, wantLine, exitFail); err == nil {
			t.Fatalf("D7 判据对改前读数竟判绿（无牙）")
		}
	})
}

// TestZergEggTierGap25Table —— 生产表的这三行必须**逐字**等于设计面（✗ 表与设计脱钩），且码位不变。
func TestZergEggTierGap25Table(t *testing.T) {
	want := map[string]struct {
		Msg  string
		Code int
	}{
		"D9": {zegGap25DesignD9, exitUsage},
		"D2": {zegGap25DesignD2, exitUsage},
		"D7": {zegGap25DesignD7, exitFail},
	}
	for tier, w := range want {
		row, ok := zegTierOf(tier)
		if !ok {
			t.Fatalf("13 档表里没有 %s", tier)
		}
		if row.Msg != w.Msg {
			t.Fatalf("%s 表内报话 %q ≠ 设计面 %q（✗ 表与设计脱钩）", tier, row.Msg, w.Msg)
		}
		if row.Code != w.Code {
			t.Fatalf("%s 表内退码 = %d ≠ 设计 %d", tier, row.Code, w.Code)
		}
	}
	// D7 的报话必须是**两枚**占位（期望 ＋ 实读），✗ 不许悄悄退回一句无读数的话。
	if n := strings.Count(zegGap25DesignD7, "%s"); n != 2 {
		t.Fatalf("D7 设计模板占位数 = %d ≠ 2", n)
	}
}

// TestZergEggTierGap25ScopeGuard —— 面界判据：本单只接 D9/D2 的**码**，其余档一律仍旧 1。
//
// ★ 它防三件事：① 「顺手把其余档也接了码」（越界）；② 把非 path 语义的规则借 D9 的号蹭到 2；
// ③ 把 `Coded` 的零值当成码 0（＝假绿）放行。
func TestZergEggTierGap25ScopeGuard(t *testing.T) {
	// ① 未接码的档（D3/D5/D8/D10/D11/D12）＋ 普通错误 ⇒ 仍旧 1。
	for _, tier := range []string{"D3", "D5", "D8", "D10", "D11", "D12"} {
		if rc := zegEggErrExit(&zegRefusal{Tier: tier, Msg: "probe"}); rc != exitFail {
			t.Fatalf("未接码的档 %s 竟退 %d（要 %d —— 本单 ✗ 不扩面）", tier, rc, exitFail)
		}
	}
	if rc := zegEggErrExit(fmt.Errorf("boom")); rc != exitFail {
		t.Fatalf("普通错误竟退 %d（要 %d）", rc, exitFail)
	}
	// ② 借挂 D9 号的**非 path 规则**（mode / 符号链接 / 重建落点）不许蹭到 2：仍走旧形态。
	if rc := zegEggErrExit(zegRuleRefuse("D9", "fs_mode_unsafe", "x", "probe")); rc != exitFail {
		t.Fatalf("D9 号下的非 path 规则竟退 %d（要 %d —— 本单只对齐 path 面）", rc, exitFail)
	}
	// ③ 已接码的档必须带码且与表内同值；D1/D4/D13（上一单）不许被本单改回去。
	for _, tier := range []string{"D1", "D2", "D4", "D9", "D13"} {
		row, _ := zegTierOf(tier)
		if rc := zegEggErrExit(zegTierRefuse(tier, "probe")); rc != row.Code {
			t.Fatalf("%s 带码拒退 %d ≠ 表内 %d", tier, rc, row.Code)
		}
	}
}

// sendback_reason_test.go — B 项④-A 实证：**打回原因码**（责任前缀 + R 编号 + 可行动建议）。
//
// 用例纪律（与 T5.1/T5.3/T5.7/T5.8 同一套）：每条都用例既断「该拒的拒了」，也断「该过的过了」，
// 且**报错要判到专项错误**（`errors.Is`）—— 只断「有错」会漏掉「错成了别的意思」。
//
//	① 三种责任前缀各一 ⇒ 前缀 / R 编号 / 建议都解析正确（含往返一致）
//	② 未知前缀 ⇒ 报错（**不得默认成某一类**）
//	③ 缺 R 编号 ⇒ 报错（`PRE_` / `PRE` / `PRE_R` / `PRE_RX` 四种形态）
//	④ 编号越界 ⇒ 报错（§3.4 的闭集 R0–R13）
//	⑤ 建议缺失 ⇒ **不能发回**（§4.7「格式类打回必须附最小修复提示」），但解析仍能读日志
//	⑥ 零值 / 手搓未归一的枚举 ⇒ 一律 Validate 不过（认不出不猜，不做静默归一化）
package policy

import (
	"errors"
	"strings"
	"testing"
)

// ── ① 三种责任前缀各一：前缀 / R 编号 / 建议都要正确 ──────────────────────────

func TestSendBackCode_ParseThreePrefixesWithJudgeNumber(t *testing.T) {
	cases := []struct {
		raw    string
		resp   Responsibility
		judge  int
		advice string
	}{
		// PRE_ = 切片者/输入可见集问题；R3 = 判据「criteria 三件套齐全」（设计稿 v2.1 §3.4）
		{"PRE_R3 补齐 criteria 三件套：命令 + 期望输出 + 当前红/绿", RespPreSlice, 3, "补齐 criteria 三件套：命令 + 期望输出 + 当前红/绿"},
		// POST_ = 执行者问题；R0 = 判据「正面句式」（这里作为"编号可为 0 且 0 不是缺失"的钉子）
		{"post_r0", RespPostExec, 0, ""},
		// INV_ = 环境/流程问题；R13 = 编号上界（§3.4 最后一条）
		{"INV_R13 建议：重启工具链后重跑合成门", RespInvEnv, 13, "重启工具链后重跑合成门"},
	}
	for _, c := range cases {
		got, err := ParseSendBackCode(c.raw)
		if err != nil {
			t.Fatalf("ParseSendBackCode(%q) 不应失败：%v", c.raw, err)
		}
		if got.Resp != c.resp {
			t.Errorf("%q 的责任前缀应为 %s，得到 %q（**不得默认成某一类**）", c.raw, c.resp, got.Resp)
		}
		if got.R != c.judge {
			t.Errorf("%q 的 R 编号应为 %d，得到 %d", c.raw, c.judge, got.R)
		}
		if got.Advice != c.advice {
			t.Errorf("%q 的建议应为 %q，得到 %q", c.raw, c.advice, got.Advice)
		}
		// 规范形态：前缀大写 + `_R` + 十进制编号（前导 0 / 小写都在这里归一）
		want := string(c.resp) + "_R" + itoa(c.judge)
		if got.String() != want {
			t.Errorf("%q 的规范形态应为 %q，得到 %q", c.raw, want, got.String())
		}
		// 大小写与前导零不另立一个码：`PRE_R03` 与 `PRE_R3` 是同一个码
		if alt, err := ParseSendBackCode(strings.ToLower(string(c.resp)) + "_R0" + itoa(c.judge)); err == nil {
			if alt.String() != want {
				t.Errorf("前导零形态应归一到 %q，得到 %q", want, alt.String())
			}
		}
	}
}

// ── ② 未知前缀 ⇒ 报错（不得默认成某一类）─────────────────────────────────────

func TestSendBackCode_UnknownPrefixRejected(t *testing.T) {
	// 每条都是「像原因码但不是」：认不出 ⇒ 报错，绝不落到某个默认责任上
	for _, raw := range []string{
		"XYZ_R3 随便补一下",  // 前缀不在闭集里
		"R3 随便补一下",      // 只有编号，没有前缀
		"PREX_R3 随便补一下", // 前缀拼错（不是 PRE）
		"根因_R3 随便补一下",   // 中文/其它形态
		"PRE-R3 随便补一下",  // 分隔符写错（不是下划线）
	} {
		got, err := ParseSendBackCode(raw)
		if !errors.Is(err, ErrUnknownResponsibility) {
			t.Errorf("ParseSendBackCode(%q) 应报 ErrUnknownResponsibility，得到 %v", raw, err)
		}
		if got != (SendBackCode{}) {
			t.Errorf("认不出时必须返回**零值**码（调用方拿不到可误当原因码的东西），得到 %+v", got)
		}
	}
	// 反例的对侧：三种合法前缀都必须过（防「把一切都拒了」的假绿）
	for _, raw := range []string{"PRE_R1 x", "POST_R1 x", "INV_R1 x"} {
		if _, err := ParseSendBackCode(raw); err != nil {
			t.Errorf("合法前缀 %q 不应被拒：%v", raw, err)
		}
	}
}

// ── ③ 缺 R 编号 ⇒ 报错（四种形态全覆盖）─────────────────────────────────────

func TestSendBackCode_MissingJudgeNumberRejected(t *testing.T) {
	for _, raw := range []string{
		"PRE_",        // 只有前缀 + 下划线
		"PRE",         // 只有前缀
		"POST_ ",      // 前缀 + 下划线 + 空白
		"PRE_R",       // 有 R 没编号
		"PRE_R 补一下",   // 有 R 没编号，后面直接是建议
		"PRE_RX 补一下",  // 编号不是数字
		"PRE_R3A 补一下", // 编号后面拖字符
	} {
		_, err := ParseSendBackCode(raw)
		if !errors.Is(err, ErrMissingJudgeNumber) {
			t.Errorf("ParseSendBackCode(%q) 应报 ErrMissingJudgeNumber（**不得默认成某个编号**），得到 %v", raw, err)
		}
	}
}

// ── ④ 编号越界 ⇒ 报错（§3.4 闭集 R0–R13）─────────────────────────────────────

func TestSendBackCode_JudgeNumberOutOfRangeRejected(t *testing.T) {
	for _, raw := range []string{"PRE_R14 x", "INV_R99 x", "POST_R1000 x", "INV_R-1 补一下"} {
		if _, err := ParseSendBackCode(raw); !errors.Is(err, ErrJudgeNumberOutOfRange) {
			t.Errorf("ParseSendBackCode(%q) 应报 ErrJudgeNumberOutOfRange，得到 %v", raw, err)
		}
	}
	// 边界必须**包含**（闭集两端都合法）：少一个就是过严的假红
	for _, raw := range []string{"PRE_R0 x", "PRE_R13 x"} {
		if _, err := ParseSendBackCode(raw); err != nil {
			t.Errorf("闭集端点 %q 应合法：%v", raw, err)
		}
	}
	// 结构侧越界同样要拒（不只解析侧）
	if err := (SendBackCode{Resp: RespPreSlice, R: 14, Advice: "x"}).Validate(); !errors.Is(err, ErrJudgeNumberOutOfRange) {
		t.Errorf("结构侧越界编号应报错，得到 %v", err)
	}
}

// ── ⑤ 建议缺失 ⇒ 不能发回（但解析仍读得出码）─────────────────────────────────

func TestSendBackCode_AdviceRequiredToSendBack(t *testing.T) {
	code, err := ParseSendBackCode("PRE_R5")
	if err != nil {
		t.Fatalf("只带码的行必须能解析（日志里常常只有码）：%v", err)
	}
	// 「解析通过」不等于「可以发回」：§4.7 要求格式类打回附最小修复提示
	if err := code.Validate(); !errors.Is(err, ErrMissingAdvice) {
		t.Errorf("没有建议的码必须 Validate 不过（ErrMissingAdvice），得到 %v", err)
	}
	if _, err := code.Format(); err == nil {
		t.Error("格式化出口也必须拦住缺建议的码（不给「看起来完整」的形态）")
	}
	if _, err := NewSendBackCode(RespPreSlice, 5, "   "); !errors.Is(err, ErrMissingAdvice) {
		t.Errorf("只有空白字符的建议 = 没给建议，应报 ErrMissingAdvice，得到 %v", err)
	}
	// 反例的对侧：补齐建议后同一个码必须能发回，且形态含码与建议
	ok, err := NewSendBackCode(RespPreSlice, 5, "把未决项写成 NEEDS-CLARIFICATION 并给出落点片段")
	if err != nil {
		t.Fatalf("补齐建议后不应失败：%v", err)
	}
	formatted, err := ok.Format()
	if err != nil {
		t.Fatalf("Format 不应失败：%v", err)
	}
	if !strings.Contains(formatted, "PRE_R5") || !strings.Contains(formatted, "NEEDS-CLARIFICATION") {
		t.Errorf("完整形态必须同时含码与建议，得到 %q", formatted)
	}
}

// ── ⑥ 往返一致 + 零值/未归一枚举一律不过 ────────────────────────────────────

func TestSendBackCode_FormatRoundTripAndZeroValue(t *testing.T) {
	orig, err := NewSendBackCode(RespInvEnv, 6, "给每条 gate_cmds 补 expect")
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}
	formatted, err := orig.Format()
	if err != nil {
		t.Fatalf("Format 失败：%v", err)
	}
	back, err := ParseSendBackCode(formatted)
	if err != nil {
		t.Fatalf("Format 的产物必须能解析回来（往返一致）：%v", err)
	}
	if back != orig {
		t.Errorf("往返不一致：原 %+v，回 %+v", orig, back)
	}

	// 零值：缺前缀 + 缺建议 ⇒ 两样都不过（**绝不默认成某一类**）
	if err := (SendBackCode{}).Validate(); err == nil {
		t.Error("零值码必须 Validate 不过")
	}
	// 手搓结构体：Enum 值必须已归一（小写 `post` 认不出 ⇒ 报错，**不做静默归一化** ——
	// 「行为宽容、契约严格」落在**入口**（Parse/New 负责归一），不是落在 Validate 上）
	if err := (SendBackCode{Resp: "post", R: 3, Advice: "x"}).Validate(); !errors.Is(err, ErrUnknownResponsibility) {
		t.Errorf("未归一的枚举值必须被拒，得到 %v", err)
	}
	// 责任标签：三类各有中文说法，认不出时**不冒充**某一类
	for resp, wantSub := range map[Responsibility]string{
		RespPreSlice: "切片者", RespPostExec: "执行者", RespInvEnv: "环境",
	} {
		if !strings.Contains(resp.Label(), wantSub) {
			t.Errorf("%s 的责任标签应含 %q，得到 %q", resp, wantSub, resp.Label())
		}
	}
	if strings.Contains(Responsibility("XYZ").Label(), "切片者") {
		t.Error("认不出的前缀不得冒充任何一类责任")
	}
}

// itoa — 用例内部用的小工具（不引 strconv：让用例只依赖被测语义）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

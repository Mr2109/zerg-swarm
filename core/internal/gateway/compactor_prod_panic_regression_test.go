package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// ============================================================
// compactor.go maskToolOutput 生产 panic 回归（[:0] slice bounds out of range）
//
// 现场（/tmp/zerg-core.log，33 条，形态一致）：
//   2026/09/17 02:15:38 http: panic serving 127.0.0.1:53566: runtime error: slice bounds out of range [:0] with length 1683
//     gateway.maskToolOutput({…, 0x693})  compactor.go:81 +0x2fc
//     gateway.TrimNoise                   compactor.go:43  +0x114
//     gateway.trimRequestMessages         compactor.go:206 +0x21c
//     (*Gateway).handleRequest            gateway.go:763   （HEAD 上该调用点已漂到 831）
//
// 真因：字符截断分支的守卫是 `len(content) > toolOutputMaxLen(800)`，而切口是常量
//   content[:toolOutputHeadLines*40] = content[:2400]  ⇒  长度落在 (800, 2400) 的单行工具输出必然越界。
// 本文件把这条带子带住：修复前 4 组断言里的前 3 组必红（panic），修复后全绿。
// ============================================================

// prodPanicLensAll —— 日志里 33 条 panic 的行长（含重复，按日志时间序逐条）。
// 生成方式（机械，无手抄）：grep -o "with length [0-9]*" /tmp/zerg-core.log | awk '{print $3}'
var prodPanicLensAll = []int{
	1683, 1017, 1683, 2168, 2345, 1687, 2216, 2171,
	1553, 1017, 1017, 1711, 1513, 2263, 1896, 1579,
	1395, 1510, 1237, 1237, 1237, 885, 1205, 956,
	1632, 1650, 1491, 1918, 1210, 1700, 1297, 1733,
	1768,
}

// ztLines 造 n 行（每行 "line i\n"）⇒ strings.Split 后是 n+1 个元素。
func ztLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("line ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	return b.String()
}

func ztDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ① 复现用例：单行、1768 字节（日志原文 `[:0] with length 1768`）。
// 修复前：maskToolOutput 在 compactor.go:81 越界 panic（本用例 FAIL）。
// 修复后：1768 ≤ 首尾窗口 2400+1600 ⇒ 原样返回（不裁、不许改一个字节）。
func TestProdPanicReproMaskToolOutput1768(t *testing.T) {
	if len(prodPanicLensAll) != 33 {
		t.Fatalf("现场长度表被改动：应 33 条，实际 %d 条", len(prodPanicLensAll))
	}
	in := strings.Repeat("x", 1768) // 单行、无 \n
	out := maskToolOutput(in)       // 修复前：panic: slice bounds out of range [:0] with length 1768
	if out != in {
		t.Fatalf("1768 字节单行输出落在首尾窗口内 ⇒ 必须逐字节原样返回；实际 %d 字节", len(out))
	}
}

// ② 生产长度带整体过一遍 TrimNoise（compactor.go:43 那条帧）：
// 33 条日志长度逐条喂进去，不得 panic、不得静默删内容。
func TestProdPanicBandViaTrimNoise(t *testing.T) {
	for _, n := range prodPanicLensAll {
		payload := strings.Repeat("x", n) // 单行工具输出，与现场同形
		msgs := []map[string]interface{}{
			{"role": "system", "content": "sys"},
			{"role": "user", "content": "用户输入"},
			{"role": "tool", "tool_call_id": "call_1", "content": payload},
		}
		out, changed := TrimNoise(msgs) // 修复前：n<2400 的第一条（1683）就在这里 panic
		if changed {
			t.Fatalf("len=%d：输出落在首尾窗口（2400+1600）内 ⇒ 不该发生精简", n)
		}
		got, _ := out[2]["content"].(string)
		if got != payload {
			t.Fatalf("len=%d：工具输出被改动（%d → %d 字节）", n, len(payload), len(got))
		}
	}
}

// ③ 走真实网关入口 trimRequestMessages（compactor.go:206 那条帧；HEAD 的调用点是 gateway.go:831）。
// body = 一条 1768 字节单行工具输出 + 一条 assistant 填充回复（后者触发真正回写 body 的路径）。
func TestProdBandViaTrimRequestMessages(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"ascii-1768", strings.Repeat("x", 1768)},
		{"cjk-1768", "x" + strings.Repeat("汉", 589)}, // 1 + 1767 = 1768 字节
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.payload) != 1768 {
				t.Fatalf("用例载荷长度不对：%d", len(c.payload))
			}
			body, err := json.Marshal(map[string]interface{}{
				"model": "probe",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "sys"},
					map[string]interface{}{"role": "user", "content": "用户原始输入必须原样保留"},
					map[string]interface{}{"role": "assistant", "content": "收到"},
					map[string]interface{}{"role": "tool", "tool_call_id": "call_1", "content": c.payload},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			changed := trimRequestMessages(&body) // 修复前：在这里 panic
			if !changed {
				t.Fatal("填充回复应被删除 ⇒ 应报告发生了精简")
			}
			var obj map[string]interface{}
			if err := json.Unmarshal(body, &obj); err != nil {
				t.Fatal(err)
			}
			msgs := obj["messages"].([]interface{})
			if len(msgs) != 3 {
				t.Fatalf("填充回复未被删除：messages=%d 条", len(msgs))
			}
			last := msgs[2].(map[string]interface{})
			if got, _ := last["content"].(string); got != c.payload {
				t.Fatalf("工具输出被改动：%d → %d 字节", len(c.payload), len(got))
			}
			if got, _ := msgs[1].(map[string]interface{})["content"].(string); got != "用户原始输入必须原样保留" {
				t.Fatalf("用户消息被改动：%q", got)
			}
		})
	}
}

// ④ 正常输入字节稳定性（golden digest）：
// 这些档位在**修复前**就有定义（不 panic），修复不许动它们的输出一个字节。
// golden 值取自修复前 revision 6f198dcb（c9210bf3^）的实测输出，见交付报告。
// 注意：**故意不在**此表里的两档（修复前后行为本就不同，属预期变更）：
//
//	· len ∈ [2400, 4000]：修复前不 panic 但 content[:2400]+content[len-1600:] 有重叠 ⇒ 内容被复制、
//	  省略数写的是 len-800（与实际删掉的量不符）；修复后改为「窗口装得下就不裁」。
//	· len > 4000：省略数语义从 len-toolOutputMaxLen 改为真删掉的字节数。
func TestMaskToolOutputNormalInputsGoldenDigests(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // sha256(修复前该档的输出)
	}{
		{"empty", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"short-100", strings.Repeat("x", 100), "09ecb6ebc8bcefc733f6f2ec44f791abeed6a99edf0cc31519637898aebd52d8"},
		{"single-800", strings.Repeat("x", 800), "20885ae2150992180a94350ef60c6d8a1599899bcd2752d3392f8ad591ddd8b1"},
		{"lines-100(边界:不裁)", ztLines(100), "b4c395cc55a76980dcc23b596801da4dce057b3b21dc632998cb7b0fc6c23b01"},
		{"lines-101(边界:裁)", ztLines(101), "be66603e5e3426dcb610931af9ad000d47bf2866b3c1b4fe4b6a86e3d20f0373"},
		{"lines-75", ztLines(75), "530fad180108f7b62b895938fcde2be9fc205d01f118f32f46e8a6ef6b41b250"},
		{"lines-400", ztLines(400), "1fbf717325342b98ccad95f19aaa9760092f10fe4d431f50329bc81da5529a21"},
		{"cjk-short", strings.Repeat("汉", 100), "62e6491b121005bef2e1114a79c7193cedb2a4766003a98d4e7d91dcad5cb33f"},
	}
	for _, c := range cases {
		got := ztDigest(maskToolOutput(c.in))
		if c.want == "" {
			t.Logf("HARVEST %-22s len(in)=%-5d digest=%s", c.name, len(c.in), got)
			continue
		}
		if got != c.want {
			t.Errorf("%s：输出与修复前不一致（sha256 前=%s 现=%s，len(in)=%d）", c.name, c.want[:12], got[:12], len(c.in))
		}
	}
}

// cli_usage_nextstep_test.go —— 序83「错误文案不给下一步」的**逐条抽查**判据（波10 · 组3 §一 序52 · 缺口 `Q-070`）。
//
// 判据（任务单 §一 序83「判据」栏逐字）：**族内每条用法错文案含「下一步」句（负控逐条抽查）**。
//
// 本条钉两件：
//
//	① **正控（逐条）**：`help export` 族的用法错（`Q-070` 那条「落点不是目录」＋ `--json` 缺字段那条）
//	   逐条现跑 ⇒ 退 `2` · stdout **0 字节** · stderr 里**必有一句「下一步」**；
//	② **负控**：把同一批 stderr 里的「下一步」那几行**抹掉** ⇒ 本判据的检查函数**必须判「没有」**
//	   （证明这条判据**会红**，不是恒绿的空转 —— 本仓那条「空转不许给绿」）。
//
// 「下一步」句的形态（词表**只有一处**真源 = `nextStepMarkers`）：
//
//	· 显式 `下一步：…`（本波补的三处用的就是这一形态）；
//	· 或指向用法面的 `See '<命令> --help'。`（本仓既有的、可照抄的指路句 —— 同样算「下一步」）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为，与
// `cli_version_jsongap_test.go` / `cli_help_twostate_test.go` 同一条路；全程只读，不起子进程、不碰状态目录。
package main_test

import (
	"bytes"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// nextStepMarkers —— 「下一步」那一件的标记词表（判据值的**唯一**真源）。
// 负控就是拿 `stripNextStep` 把这些标记从 stderr 里摘干净，看检查函数还认不认。
var nextStepMarkers = []string{"下一步：", "See '"}

// hasNextStep —— 「这条 stderr 里有没有一句『下一步』」判据本体。
func hasNextStep(stderr string) bool {
	for _, m := range nextStepMarkers {
		if strings.Contains(stderr, m) {
			return true
		}
	}
	return false
}

// stripNextStep —— 把带「下一步」标记的行逐行摘掉（负控夹具：一份「没有下一步」的 stderr）。
func stripNextStep(stderr string) string {
	var keep []string
	for _, ln := range strings.Split(stderr, "\n") {
		hit := false
		for _, m := range nextStepMarkers {
			if strings.Contains(ln, m) {
				hit = true
				break
			}
		}
		if !hit {
			keep = append(keep, ln)
		}
	}
	return strings.Join(keep, "\n")
}

// TestCLIUsageErrorsCarryNextStep —— ① 正控：`help export` 族用法错逐条现跑，每条 stderr 必含「下一步」句。
func TestCLIUsageErrorsCarryNextStep(t *testing.T) {
	// 三条 argv **都不挂「干跑」那一枚旗标**：本条不是干跑语义站点 —— ① 落点不是目录、
	// ② 版本目录不在盘上，都在**任何写动作之前**返回；③ 是 `version` 的纯读路径。
	// ⇒ 它既不落进「干跑语义站点台账」（那枚判据扫的是**真有干跑语义的件**）的扫描面，也不起写动作。
	cases := []struct {
		name string
		argv []string
	}{
		{"Q-070 落点不是目录", []string{"help", "export", "--out", "/nonexistent-zz/x"}},
		{"解析不到落点", []string{"help", "export", "--docs-ver", "9.9.9"}},
		{"--json 缺字段（requireFields）", []string{"version", "--json"}},
	}
	for _, c := range cases {
		var so, se bytes.Buffer
		rc := zerg.RunForTest(c.argv, &so, &se)
		if rc != 2 {
			t.Errorf("%s：`zerg %s` ⇒ 用法错应退 2，得到 %d（判据不可判）", c.name, strings.Join(c.argv, " "), rc)
			continue
		}
		if so.Len() != 0 {
			t.Errorf("%s：用法错 stdout 必须 0 字节（提示面走 stderr），得到 %q", c.name, so.String())
		}
		if !hasNextStep(se.String()) {
			t.Errorf("%s：stderr 缺「下一步」句（§4.1 K14 四件套）：%q", c.name, se.String())
		}
	}
}

// TestCLIUsageNextStep_NegativeControl —— ② 负控：抹掉「下一步」那几行 ⇒ 检查函数必须判「没有」。
// 没有这一格，上面那条正控可能只是「词表太宽」造成的恒绿空转。
func TestCLIUsageNextStep_NegativeControl(t *testing.T) {
	var so, se bytes.Buffer
	_ = zerg.RunForTest([]string{"help", "export", "--out", "/nonexistent-zz/x"}, &so, &se)
	msg := se.String()
	if strings.TrimSpace(msg) == "" {
		t.Fatal("拿不到 stderr（判据不可判）")
	}
	// 正控半边：原样 ⇒ 认得出「下一步」。
	if !hasNextStep(msg) {
		t.Fatalf("负控前置不成立：原始 stderr 里就找不到「下一步」句：%q", msg)
	}
	// 负控半边：摘干净 ⇒ 必须判「没有」。
	if hasNextStep(stripNextStep(msg)) {
		t.Errorf("负控失败：摘掉「下一步」那几行后仍判「有」⇒ 判据认的不是那一件（恒绿空转）：%q",
			stripNextStep(msg))
	}
}

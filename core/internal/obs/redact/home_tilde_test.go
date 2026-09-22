package redact

import (
	"strings"
	"testing"
)

// home_tilde_test.go —— 家目录的**波浪号形态**（`~/…`）判据（设计-CI适配-v1.1 §八 待拍 3 的 (a) 半）。
//
// 缺口的形状（真缺口，不是夹具问题）：用户面上 `~/…` **就是**家目录 —— 日志、报错、shell 回显里
// 写出来的常常是 `~/…`，而这条路径快路的**必要条件门**过去只认 `/Users/`、`/home/` 两个绝对字面量
// ⇒ `~/…` 整段跳过脱敏（公开面实测：`LEAK: 秘密 "~/projects/zerg/vault.key" 以投影 "~/…" 残留`）。
//
// 三条判据：
//
//	① 正控：`~/projects/…` 必须被换成 `${HOME}/projects/…`（**分隔符留在原位** —— 少了它
//	   尾部路径会被粘成 `${HOME}projects/…`，定位信息就废了）；
//	② 必要条件门的**健全性**：reHomeDir 命中 ⇒ hasHomePath 必真（门松一格就是「静默丢覆盖」，
//	   见 redact.go 里那条注释的口径）；
//	③ 负控：**没有分隔符**的 `~`（`elapsed ~5ms`）不该被当成家目录 —— 门若写成「见到 ~ 就算」，
//	   普通文本会被改写（假阳性比漏遮更早把门禁关掉）。
func TestHomeTildeFormIsRedacted(t *testing.T) {
	in := "open ~/projects/zerg/vault.key: permission denied"
	out := RedactValue(in)
	if strings.Contains(out, "~/") {
		t.Fatalf("`~/` 形态整段跳过脱敏（真缺口）：%q → %q", in, out)
	}
	if want := PlaceholderPath + "/projects/zerg/vault.key"; !strings.Contains(out, want) {
		t.Errorf("`~/` 应换成 %q（分隔符留在原位，尾部照旧保留），实得 %q", want, out)
	}
}

func TestHomePathGateIsNecessaryForBothForms(t *testing.T) {
	// 两支形态都要在：绝对（`/Users` · `/home`）与波浪号（`~/`）。
	for _, in := range []string{
		"/Users/someone/x", "/home/someone/x", "~/x", "~/", "a ~/b/c",
	} {
		if !reHomeDir.MatchString(in) {
			t.Fatalf("夹具失效：reHomeDir 本该命中 %q（换夹具必须同批改本条）", in)
		}
		if !hasHomePath(in) {
			t.Errorf("必要条件门破了：reHomeDir 命中 %q，hasHomePath 却是假 ⇒ 该形态会静默跳过脱敏", in)
		}
	}
}

func TestBareTildeIsNotAHomePath(t *testing.T) {
	in := "elapsed ~5ms (approx)"
	if hasHomePath(in) {
		t.Fatalf("门太松：没有分隔符的 `~` 不该算家目录（%q）—— 那会把普通文本当路径改写", in)
	}
	if out := RedactValue(in); out != in {
		t.Fatalf("假阳性：无分隔符的 `~` 被改写了 %q → %q", in, out)
	}
}

// self_origin_test.go —— 源提交（`Origin`）取源的回归牙（缺口 `GAP-20261021-08`）。
//
// 判据（缺一即错）：
//
//	① 取源顺序：`-X version.Origin` > 嵌入段 `origin=` > `ZERG_ORIGIN` > **空串**；
//	② 空串 ≠ `unknown` 字面量：机器面（心跳 `origin_sha` / `/status`）带**空串**（= 判不了），
//	   人面（`Identity`）印 `unknown` —— 两套口径各有其位，✗ 不许互相顶；
//	③ `Identity` 只在 `Line` 的四格后**追加**两格 ⇒ 现有按 `$1..$4` 解析的脚本零改动；
//	④ Origin 是**每次现读**（进程级注入可在运行中被测试改动断言），件事实仍缓存。
package version

import (
	"strings"
	"testing"
)

// withOrigin 把 `-X` 那格临时置为 v（`t.Cleanup` 还原），供各用例构造前置。
func withOrigin(t *testing.T, v string) {
	t.Helper()
	old := Origin
	Origin = v
	t.Cleanup(func() { Origin = old })
}

func TestResolveOrigin_Precedence(t *testing.T) {
	cases := []struct {
		name     string
		ldflags  string
		embedded string
		env      string
		wantVal  string
		wantSrc  string
	}{
		{"三档齐全 ⇒ -X 优先", "aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc", "aaaaaaaaaaaa", SourceLdflags},
		{"无 -X ⇒ 嵌入 tags", "", "bbbbbbbbbbbb", "cccccccccccc", "bbbbbbbbbbbb", SourceEmbeddedTags},
		{"只有环境变量", "", "", "cccccccccccc", "cccccccccccc", SourceEnv},
		{"三档全无 ⇒ 空串 + unknown", "", "", "", "", SourceUnknown},
		{"空白不算注入", "   ", "  ", "  ", "", SourceUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withOrigin(t, c.ldflags)
			t.Setenv(OriginEnv, c.env)
			gotVal, gotSrc := resolveOrigin(c.embedded)
			if gotVal != c.wantVal || gotSrc != c.wantSrc {
				t.Fatalf("resolveOrigin = (%q, %q)，want (%q, %q)", gotVal, gotSrc, c.wantVal, c.wantSrc)
			}
		})
	}
}

func TestReadSelf_CarriesOrigin(t *testing.T) {
	withOrigin(t, "")
	t.Setenv(OriginEnv, "3b04f6abcd12")
	resetSelfForTest()
	s := ReadSelf()
	if s.Origin != "3b04f6abcd12" || s.OriginSource != SourceEnv {
		t.Fatalf("ReadSelf 应带上环境变量那一档，实得 Origin=%q Source=%q", s.Origin, s.OriginSource)
	}
	// 进程级注入**每次现读**：改成 -X 值后同一进程里立刻看得见（件事实仍缓存）。
	Origin = "aaaaaaaaaaaa"
	if got := ReadSelf().Origin; got != "aaaaaaaaaaaa" {
		t.Fatalf("Origin 应现读注入变量，实得 %q（缓存化的写法会让运维面读到旧值）", got)
	}
	if got := ReadSelf().OriginSource; got != SourceLdflags {
		t.Fatalf("来路应随之变为 %q，实得 %q", SourceLdflags, got)
	}
}

func TestReadSelf_OriginAbsentIsEmptyNotUnknown(t *testing.T) {
	withOrigin(t, "")
	t.Setenv(OriginEnv, "")
	resetSelfForTest()
	s := ReadSelf()
	if s.Origin != "" {
		t.Fatalf("未注入时 Origin 必须是**空串**（主控按空串判 unknown），实得 %q", s.Origin)
	}
	if s.OriginSource != SourceUnknown {
		t.Fatalf("来路应如实为 %q，实得 %q", SourceUnknown, s.OriginSource)
	}
	if got := s.OriginText(); got != SelfUnknown {
		t.Fatalf("人面词应为 %q，实得 %q", SelfUnknown, got)
	}
}

func TestIdentity_AppendsAfterLine(t *testing.T) {
	t.Setenv(OriginEnv, "")
	withOrigin(t, "aaaaaaaaaaaa")
	resetSelfForTest()
	s := ReadSelf()
	line := s.Line("zerg-agentd")
	if n := len(strings.Fields(line)); n != 4 {
		t.Fatalf("Line 必须仍是四格（旧脚本按 $1..$4 切），实得 %d 格：%q", n, line)
	}
	want := line + " origin=aaaaaaaaaaaa (" + SourceLdflags + ")"
	if got := s.Identity("zerg-agentd"); got != want {
		t.Fatalf("Identity 应是 Line 之后**追加**两格：\n got %q\nwant %q", got, want)
	}
	// 缺席时人面印 unknown、机器面仍空串。
	Origin = ""
	if got := ReadSelf().Identity("zerg-agentd"); got != line+" origin=unknown ("+SourceUnknown+")" {
		t.Fatalf("缺席时 Identity 应印 unknown：%q", got)
	}
}

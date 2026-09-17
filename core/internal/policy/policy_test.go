// policy_test.go — T5.1 叶包实证：优先级（deny > ask > allow）、地板优先、默认 ask、
// fail-closed（解析失败 / 判不了）**逐条钉死**。
//
// 为什么叶包要有自己的用例（而不是只在接线后的链路上断）：判定是**纯函数级**的事实
// —— 同样输入必然同样输出（无时间、无随机、无 IO），所以优先级与 fail-closed 的每一条都能在这里
// 用最小输入直接钉住；接线后的用例只负责证明"sink 里拿到的是同一个决定"。
//
// 纪律：本包不碰文件系统、不 import 任何业务包；每条用例自带正例与反例
// （只断"该 deny 的 deny"会漏掉"把一切都 deny"这种假绿，故每条都要有对侧断言）。
package policy

import (
	"strings"
	"testing"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

func mustLoad(t *testing.T, rules, floor string) *Engine {
	t.Helper()
	e, err := Load(Config{Rules: rules, Floor: floor})
	if err != nil {
		t.Fatalf("Load 不应失败：%v", err)
	}
	if e == nil {
		t.Fatal("Load 返回了 nil 引擎却没有错误（fail-closed 要求：要么可用，要么报错）")
	}
	return e
}

// check — 一次判定的三段断言：决定 + 原因码 + 命中规则（wantRule == "" 表示**必须没有**命中规则）。
func check(t *testing.T, got Verdict, want Effect, wantReason Reason, wantRule string) {
	t.Helper()
	if got.Decision != want {
		t.Errorf("decision = %q，期望 %q（reason=%s note=%s）", got.Decision, want, got.Reason, got.Note)
	}
	if got.Reason != wantReason {
		t.Errorf("reason = %q，期望 %q（note=%s）", got.Reason, wantReason, got.Note)
	}
	if wantRule == "" {
		if got.MatchedRule != nil {
			t.Errorf("不应有命中规则，却有 %q", got.MatchedRule.Raw)
		}
		return
	}
	if got.MatchedRule == nil {
		t.Fatalf("应有命中规则 %q，实际没有（reason=%s note=%s）", wantRule, got.Reason, got.Note)
	}
	if got.MatchedRule.Raw != wantRule {
		t.Errorf("命中规则 = %q，期望 %q", got.MatchedRule.Raw, wantRule)
	}
}

func cmd(s string) map[string]any     { return map[string]any{"command": s} }
func pathArg(s string) map[string]any { return map[string]any{"path": s} }
func urlArg(s string) map[string]any  { return map[string]any{"url": s} }

// deepArgs — 造"嵌套过深"的参数（最深处的字符串**存在**，只是超出下探上限 ⇒ 判不了）。
func deepArgs(levels int) map[string]any {
	inner := map[string]any{"path": "npm run build"}
	for i := 0; i < levels; i++ {
		inner = map[string]any{"level": inner}
	}
	return map[string]any{"command": inner}
}

// ── (a) deny 压 allow（同工具同时匹配）──────────────────────────────────────

func TestA_DenyBeatsAllow(t *testing.T) {
	const allowRule = "allow bash(npm run *)"
	const denyRule = "deny bash(npm run deploy)"
	args := cmd("npm run deploy")

	t.Run("控制：只有 allow 时该 allow 真的命中", func(t *testing.T) {
		// 没有这一步，"deny 压 allow"可能只是"allow 本来就没命中"的假象。
		eng := mustLoad(t, allowRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectAllow, ReasonRuleAllow, allowRule)
	})

	t.Run("deny 压 allow", func(t *testing.T) {
		eng := mustLoad(t, allowRule+"\n"+denyRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectDeny, ReasonRuleDeny, denyRule)
	})

	t.Run("与书写顺序无关", func(t *testing.T) {
		eng := mustLoad(t, denyRule+"\n"+allowRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectDeny, ReasonRuleDeny, denyRule)
	})

	t.Run("ask 压 allow", func(t *testing.T) {
		eng := mustLoad(t, allowRule+"\nask bash(npm run dep*)", "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectAsk, ReasonRuleAsk, "ask bash(npm run dep*)")
	})

	t.Run("deny 压 ask", func(t *testing.T) {
		eng := mustLoad(t, "ask bash(npm run *)\n"+denyRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectDeny, ReasonRuleDeny, denyRule)
	})

	t.Run("同效果多命中：取先出现的那条作命中规则", func(t *testing.T) {
		eng := mustLoad(t, "deny bash(npm run deploy)\ndeny bash(npm run *)", "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectDeny, ReasonRuleDeny, "deny bash(npm run deploy)")
	})

	t.Run("无匹配式按工具名整体压过带匹配式的 allow", func(t *testing.T) {
		eng := mustLoad(t, "ask bash\n"+allowRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectAsk, ReasonRuleAsk, "ask bash")
	})

	t.Run("deny 不影响别的工具（防'一律 deny'的假绿）", func(t *testing.T) {
		eng := mustLoad(t, allowRule+"\n"+denyRule, "")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("npm run build")}), EffectAllow, ReasonRuleAllow, allowRule)
	})
}

// ── (b) 地板压过档位 ────────────────────────────────────────────────────────

func TestB_FloorBeatsMode(t *testing.T) {
	t.Run("地板 deny 压全放档", func(t *testing.T) {
		rmArgs := cmd("rm -rf /tmp/x")
		// 控制：同样的档位与参数，**无地板** ⇒ allow —— 证明这个档位真的是"全放"。
		wide := mustLoad(t, "", "")
		check(t, wide.Decide(Request{Tool: "bash", Args: rmArgs, Mode: ModeBypass}), EffectAllow, ReasonBypassAllow, "")
		// 地板命中 ⇒ 仍 deny（组织/系统下发的约束不因会话档位而松动）。
		eng := mustLoad(t, "", "deny bash(rm -rf *)")
		check(t, eng.Decide(Request{Tool: "bash", Args: rmArgs, Mode: ModeBypass}), EffectDeny, ReasonFloorDeny, "deny bash(rm -rf *)")
	})

	t.Run("地板 ask 压全放档", func(t *testing.T) {
		eng := mustLoad(t, "", "ask bash(curl *)")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("curl x"), Mode: ModeBypass}), EffectAsk, ReasonFloorAsk, "ask bash(curl *)")
	})

	t.Run("地板 ask 压普通 allow", func(t *testing.T) {
		eng := mustLoad(t, "allow bash(git push *)", "ask bash(git push *)")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("git push origin main")}), EffectAsk, ReasonFloorAsk, "ask bash(git push *)")
	})

	t.Run("地板 deny 在无人值守档仍是地板原因码", func(t *testing.T) {
		eng := mustLoad(t, "", "deny bash(rm -rf *)")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("rm -rf /tmp/x"), Mode: ModeDontAsk}), EffectDeny, ReasonFloorDeny, "deny bash(rm -rf *)")
	})

	t.Run("地板里的 allow 不授予任何权限", func(t *testing.T) {
		// 地板只加约束、不放松：写在地板里的 allow 等于没写 ⇒ 回到默认 ask。
		eng := mustLoad(t, "", "allow bash(curl *)")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("curl x")}), EffectAsk, ReasonDefaultAsk, "")
		if fl := eng.Floor(); len(fl) != 1 || !fl[0].Floor {
			t.Fatalf("地板规则应带 Floor 标记：%+v", fl)
		}
	})
}

// ── (c) 未知工具 / 无匹配 ⇒ 默认 ask ────────────────────────────────────────

func TestC_NoMatchDefaultsAsk(t *testing.T) {
	eng := mustLoad(t, "allow bash(npm run *)", "deny read(./.env)")

	t.Run("未知工具", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "definitely_not_a_tool", Args: cmd("npm run x")}),
			EffectAsk, ReasonDefaultAsk, "")
	})
	t.Run("已知工具但无规则匹配", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("ls -la")}), EffectAsk, ReasonDefaultAsk, "")
	})
	t.Run("空参数字典也算无匹配", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "write", Args: nil}), EffectAsk, ReasonDefaultAsk, "")
	})
	t.Run("工具名为空 ⇒ 判不了", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "", Args: cmd("npm run x")}), EffectAsk, ReasonInvalidRequest, "")
		check(t, eng.Decide(Request{Tool: "   ", Args: cmd("npm run x")}), EffectAsk, ReasonInvalidRequest, "")
	})
	t.Run("默认档的默认值是 ask，不是 allow", func(t *testing.T) {
		// 同一份规则、同一个工具：只把档位从 default 换成 bypass，结论才从 ask 变 allow
		// ⇒ 证明 ask 是**默认档**的默认，不是别的东西顺手给的。
		req := Request{Tool: "bash", Args: cmd("ls -la")}
		check(t, eng.Decide(req), EffectAsk, ReasonDefaultAsk, "")
		req.Mode = ModeBypass
		check(t, eng.Decide(req), EffectAllow, ReasonBypassAllow, "")
	})
	t.Run("无人值守档：无命中 ⇒ deny", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("ls -la"), Mode: ModeDontAsk}), EffectDeny, ReasonDontAskDeny, "")
	})
	t.Run("无人值守档：ask 规则 ⇒ deny（凡会弹窗一律拒）", func(t *testing.T) {
		engAsk := mustLoad(t, "ask bash(git push *)", "")
		check(t, engAsk.Decide(Request{Tool: "bash", Args: cmd("git push origin main"), Mode: ModeDontAsk}),
			EffectDeny, ReasonDontAskDeny, "ask bash(git push *)")
		// 对侧：allow 规则在无人值守档下照旧 allow（不是"什么都拒"）
		engAllow := mustLoad(t, "allow bash(npm run *)", "")
		check(t, engAllow.Decide(Request{Tool: "bash", Args: cmd("npm run build"), Mode: ModeDontAsk}),
			EffectAllow, ReasonRuleAllow, "allow bash(npm run *)")
	})
	t.Run("未知档位 ⇒ 判不了", func(t *testing.T) {
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("npm run x"), Mode: Mode("yolo")}),
			EffectAsk, ReasonInvalidMode, "")
	})
}

// ── (d) 参数不可解析 ⇒ ask（fail-closed）────────────────────────────────────

func TestD_ArgUnresolvedAsks(t *testing.T) {
	const rule = "allow bash(npm run *)"
	eng := mustLoad(t, rule, "")

	cases := []struct {
		name string
		args map[string]any
	}{
		{"参数为 nil", map[string]any{"command": nil}},
		{"参数是数字", map[string]any{"command": 123}},
		{"参数是布尔", map[string]any{"command": true}},
		{"参数是切片但里面没有字符串", map[string]any{"command": []any{123, true}}},
		{"参数键不存在", map[string]any{"other": "npm run build"}},
		{"Args 整个为 nil", nil},
		{"嵌套过深（字符串在深处，超出下探上限）", deepArgs(12)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check(t, eng.Decide(Request{Tool: "bash", Args: c.args}), EffectAsk, ReasonArgsUnresolved, "")
		})
	}

	t.Run("反例：浅层嵌套里的字符串取得到 ⇒ 规则照常命中", func(t *testing.T) {
		// 没有这一条，"判不了"可能只是"凡容器都判不了"的假象。
		args := map[string]any{"command": map[string]any{"text": "npm run build"}}
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectAllow, ReasonRuleAllow, rule)
	})

	t.Run("反例：浅层取到的字符串不匹配 ⇒ 回到默认 ask（不是 args_unresolved）", func(t *testing.T) {
		args := map[string]any{"command": map[string]any{"text": "npm install"}}
		check(t, eng.Decide(Request{Tool: "bash", Args: args}), EffectAsk, ReasonDefaultAsk, "")
	})

	t.Run("判不了不得被 deny 之外的任何东西覆盖", func(t *testing.T) {
		// 另一条规则命中了 allow，但同一个工具名下还有一条判不了的规则 ⇒ 不放行。
		eng2 := mustLoad(t, rule+"\nallow bash(broken * form)", "")
		check(t, eng2.Decide(Request{Tool: "bash", Args: cmd("npm run build")}), EffectAsk, ReasonSpecifierUnrecognized, "")
	})
}

// ── (e) 解析失败 ⇒ 整套拒绝启用 ─────────────────────────────────────────────

func TestE_ParseFailureRejectsWholeSet(t *testing.T) {
	bad := []struct{ name, rules string }{
		{"未知效果", "grant bash(npm run *)"},
		{"只有效果没有工具名", "deny"},
		{"效果后多余空白", "deny   "},
		{"括号未闭合", "deny bash(rm -rf *"},
		{"行尾多余字符", "deny bash(rm -rf *)x"},
		{"匹配式为空", "deny bash()"},
		{"匹配式嵌套括号", "deny bash(rm (x))"},
		{"工具名带空白", "deny ba sh(npm)"},
		{"工具名通配", "deny *(npm)"},
		{"坏行夹在好行之间", "deny bash(rm -rf *)\nallow\nallow read"},
		{"整行注释后跟坏行", "# 说明\nask\n"},
	}
	for _, c := range bad {
		t.Run("普通规则-"+c.name, func(t *testing.T) {
			eng, err := Load(Config{Rules: c.rules})
			if err == nil {
				t.Fatalf("坏规则集必须返回错误，却加载成功（引擎=%v）", eng != nil)
			}
			if eng != nil {
				t.Fatalf("解析失败必须返回**空引擎**，不得回退成「无规则」的可用引擎（rules=%d）", len(eng.Rules()))
			}
		})
	}

	t.Run("地板解析失败同样整体拒绝", func(t *testing.T) {
		eng, err := Load(Config{Rules: "allow read", Floor: "deny bash(broken"})
		if err == nil || eng != nil {
			t.Fatalf("地板坏 ⇒ 整套拒绝启用：eng=%v err=%v", eng, err)
		}
		if !strings.Contains(err.Error(), "地板") {
			t.Errorf("错误信息应指明是地板规则集出了问题：%v", err)
		}
	})

	t.Run("错误必须带行号（可定位）", func(t *testing.T) {
		_, err := Load(Config{Rules: "deny bash(rm -rf *)\nallow\nallow read"})
		if err == nil {
			t.Fatal("应报错")
		}
		if !strings.Contains(err.Error(), "第 2 行") {
			t.Errorf("错误信息应带行号「第 2 行」：%v", err)
		}
	})

	t.Run("控制：合法文本必须能加载（防'什么都报错'的假绿）", func(t *testing.T) {
		eng := mustLoad(t, "# 注释\nallow read\nask write(src/**)\ndeny bash(rm -rf *)\n", "deny webfetch(domain:evil.com)")
		if n := len(eng.Rules()); n != 3 {
			t.Fatalf("应解析出 3 条普通规则，实际 %d", n)
		}
		if n := len(eng.Floor()); n != 1 {
			t.Fatalf("应解析出 1 条地板规则，实际 %d", n)
		}
	})

	t.Run("nil 引擎照旧 fail-closed：ask，且不 panic", func(t *testing.T) {
		var eng *Engine
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("npm run build")}), EffectAsk, ReasonEngineDisabled, "")
		v := eng.Decide(Request{Tool: "bash", Mode: ModeBypass})
		if v.Decision != EffectAsk {
			t.Fatalf("未启用的引擎在全放档下也必须是 ask（fail-closed），实际 %s", v.Decision)
		}
	})
}

// ── (f) 三类匹配式：各一条命中 + 一条不命中 ─────────────────────────────────

func TestF_SpecifierKinds_HitAndMiss(t *testing.T) {
	cases := []struct {
		name       string
		rules      string
		tool       string
		args       map[string]any
		mode       Mode
		want       Effect
		wantReason Reason
		wantRule   string
	}{
		// ① 前缀通配
		{"前缀-命中", "allow bash(npm run *)", "bash", cmd("npm run build"), "", EffectAllow, ReasonRuleAllow, "allow bash(npm run *)"},
		{"前缀-不命中", "allow bash(npm run *)", "bash", cmd("npm install"), "", EffectAsk, ReasonDefaultAsk, ""},
		{"前缀-纯通配命中任意命令", "ask bash(*)", "bash", cmd("anything at all"), "", EffectAsk, ReasonRuleAsk, "ask bash(*)"},
		{"前缀-无通配则逐字相等命中", "deny bash(git push)", "bash", cmd("git push"), "", EffectDeny, ReasonRuleDeny, "deny bash(git push)"},
		{"前缀-无通配则逐字相等不命中", "deny bash(git push)", "bash", cmd("git push --force"), "", EffectAsk, ReasonDefaultAsk, ""},

		// ② 路径
		{"路径前缀-命中直接子文件", "allow write(src/**)", "write", pathArg("src/a.go"), "", EffectAllow, ReasonRuleAllow, "allow write(src/**)"},
		{"路径前缀-命中深层（键名 file_path）", "allow write(src/**)", "write", map[string]any{"file_path": "src/x/y/z.go"}, "", EffectAllow, ReasonRuleAllow, "allow write(src/**)"},
		{"路径前缀-不命中同前缀异目录", "allow write(src/**)", "write", pathArg("srcz/a.go"), "", EffectAsk, ReasonDefaultAsk, ""},
		{"路径前缀-不命中别处", "allow write(src/**)", "write", pathArg("/tmp/a.go"), "", EffectAsk, ReasonDefaultAsk, ""},
		{"路径逐字-归一生效（./.env ≡ .env）", "deny read(./.env)", "read", pathArg("./.env"), "", EffectDeny, ReasonRuleDeny, "deny read(./.env)"},
		{"路径逐字-不命中", "deny read(./.env)", "read", pathArg("./.env.local"), "", EffectAsk, ReasonDefaultAsk, ""},

		// ③ 域名
		{"域名-精确命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("https://example.com/a?b=1"), "", EffectAllow, ReasonRuleAllow, "allow webfetch(domain:example.com)"},
		{"域名-子域命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("api.example.com/v1"), "", EffectAllow, ReasonRuleAllow, "allow webfetch(domain:example.com)"},
		{"域名-带端口仍命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("http://example.com:8443/x"), "", EffectAllow, ReasonRuleAllow, "allow webfetch(domain:example.com)"},
		{"域名-异域名不命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("https://evil.com/example.com"), "", EffectAsk, ReasonDefaultAsk, ""},
		{"域名-无点号边界不命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("https://notexample.com"), "", EffectAsk, ReasonDefaultAsk, ""},
		{"域名-前缀相同但域名不同不命中", "allow webfetch(domain:example.com)", "webfetch", urlArg("https://example.com.evil.com"), "", EffectAsk, ReasonDefaultAsk, ""},

		// ④ 认不出的匹配式 ⇒ 判不了 ⇒ ask（**即使档位是全放**）
		{"认不出-中缀通配", "allow bash(npm * run)", "bash", cmd("npm run build"), "", EffectAsk, ReasonSpecifierUnrecognized, ""},
		{"认不出-后缀通配", "allow read(*.env)", "read", pathArg(".env"), "", EffectAsk, ReasonSpecifierUnrecognized, ""},
		{"认不出-路径中缀", "allow write(src/**/*.go)", "write", pathArg("src/a.go"), "", EffectAsk, ReasonSpecifierUnrecognized, ""},
		{"认不出-域名残缺", "allow webfetch(domain:)", "webfetch", urlArg("https://example.com"), "", EffectAsk, ReasonSpecifierUnrecognized, ""},
		{"认不出-全放档下也不放行", "allow read(*.env)", "read", pathArg(".env"), ModeBypass, EffectAsk, ReasonSpecifierUnrecognized, ""},
		{"认不出-全放档下别的工具照旧全放", "allow read(*.env)", "write", pathArg("a.go"), ModeBypass, EffectAllow, ReasonBypassAllow, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eng := mustLoad(t, c.rules, "")
			check(t, eng.Decide(Request{Tool: c.tool, Args: c.args, Mode: c.mode}), c.want, c.wantReason, c.wantRule)
		})
	}
}

// TestClassifySpec — 形态判定的单元表（形态由**语法**决定，不看调用侧参数 ⇒ 解析期即可定型）。
func TestClassifySpec(t *testing.T) {
	cases := []struct {
		spec string
		want SpecKind
	}{
		{"", SpecNone},
		{"npm run *", SpecPrefix},
		{"*", SpecPrefix},
		{"git push", SpecPrefix},
		{"rm -rf *", SpecPrefix},
		{"src/**", SpecPath},
		{"**", SpecPath},
		{"./.env", SpecPath},
		{"/etc/passwd", SpecPath},
		{"~/secrets/**", SpecPath},
		{"domain:example.com", SpecDomain},
		{"domain:sub.example.com", SpecDomain},
		{"domain:", SpecUnknown},
		{"domain:example.com:8080", SpecUnknown},
		{"domain:*.example.com", SpecUnknown},
		{"npm * run", SpecUnknown},
		{"*.env", SpecUnknown},
		{"src/**/*.go", SpecUnknown},
		{"**/x", SpecUnknown},
		{"src/*", SpecUnknown},
		{"*.go?", SpecUnknown},
	}
	for _, c := range cases {
		if got := classifySpec(c.spec); got != c.want {
			t.Errorf("classifySpec(%q) = %q，期望 %q", c.spec, got, c.want)
		}
	}
}

// TestToolNameCaseInsensitive — 工具名归一化（去空白 + 小写），与仓内工具名匹配纪律一致。
func TestToolNameCaseInsensitive(t *testing.T) {
	eng := mustLoad(t, "ALLOW  BaSh(npm run *)\n", "")
	if got := eng.Rules()[0].Tool; got != "bash" {
		t.Fatalf("规则里的工具名应归一化为小写：%q", got)
	}
	if got := eng.Rules()[0].Effect; got != EffectAllow {
		t.Fatalf("效果大小写不敏感：%q", got)
	}
	if got := eng.Rules()[0].Specifier; got != "npm run *" {
		t.Fatalf("匹配式应去首尾空白：%q", got)
	}
	check(t, eng.Decide(Request{Tool: "BASH", Args: cmd("npm run build")}), EffectAllow, ReasonRuleAllow, "ALLOW  BaSh(npm run *)")
	check(t, eng.Decide(Request{Tool: " bash ", Args: cmd("npm run build")}), EffectAllow, ReasonRuleAllow, "ALLOW  BaSh(npm run *)")
}

// TestUnknownSpecifierDoesNotPoisonOtherTools — "判不了"按**工具名**记账：
// read 的坏规则不许把 write 的合法 allow 一起压成 ask（否则一处写错 = 全系统变问人）。
func TestUnknownSpecifierDoesNotPoisonOtherTools(t *testing.T) {
	eng := mustLoad(t, "allow read(*.env)\nallow write(src/**)", "")
	check(t, eng.Decide(Request{Tool: "write", Args: pathArg("src/a.go")}), EffectAllow, ReasonRuleAllow, "allow write(src/**)")
	check(t, eng.Decide(Request{Tool: "read", Args: pathArg(".env")}), EffectAsk, ReasonSpecifierUnrecognized, "")
}

// TestParseMode — 档位解析：空 = 保守档；未知一律报错（不猜）。
func TestParseMode(t *testing.T) {
	if m, err := ParseMode("  "); err != nil || m != ModeDefault {
		t.Fatalf("空串应是保守档：%q %v", m, err)
	}
	if m, err := ParseMode("BYPASS"); err != nil || m != ModeBypass {
		t.Fatalf("档位大小写不敏感：%q %v", m, err)
	}
	if _, err := ParseMode("yolo"); err == nil {
		t.Fatal("未知档位必须报错（不猜档位）")
	}
}

// TestStrictnessOrder — 优先级序只有一处定义（Effect.strictness），避免第二套"谁压谁"。
func TestStrictnessOrder(t *testing.T) {
	if !(EffectDeny.strictness() > EffectAsk.strictness() && EffectAsk.strictness() > EffectAllow.strictness()) {
		t.Fatalf("严格度序必须是 deny > ask > allow：%d %d %d",
			EffectDeny.strictness(), EffectAsk.strictness(), EffectAllow.strictness())
	}
	if Effect("grant").strictness() > EffectAllow.strictness() {
		t.Fatal("非法效果不得比 allow 更宽")
	}
	if !EffectAllow.Valid() || Effect("grant").Valid() {
		t.Fatal("效果合法性判定有误")
	}
}

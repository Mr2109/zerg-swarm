// posture_test.go — T5.4 实证：无人值守姿态两档（dontAsk / yolo）+ 常态档（normal）。
//
// 四条用例逐条钉死：
//
//	⑦ dontAsk：**所有"会弹窗的"判定结果一律 deny**（含地板 ask 与普通 ask），且 allow/deny 一字不动；
//	⑧ yolo   ：工具表**不含**问人类工具（按能力标记 / 保留名摘），普通工具原序原样**不许多摘一个**；
//	⑨ 反例   ：normal 姿态**惰性** —— 不摘任何工具、不把 ask 变 deny、不把 ask 变 allow；
//	⑩ 继承与重启：沿扇出继承（`InheritedByFanout` 才继承）+ 姿态可序列化并在重启后仍是一个事实；
//	  另外钉住：认不出的姿态取值**按最严**（不许静默降级成常态）。
//
// 纪律：与 T5.1/T5.2/T5.3 同一套 —— 每条都要有对侧断言（只断"摘了问人工具"会漏掉"顺手把别的也摘了"）。
package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// postureToolset — 一个"像真的"工具表：普通工具 + 两个问人类工具（一个靠能力标记、一个靠保留名）。
func postureToolset() []Tool {
	return []Tool{
		{Name: "bash"},
		{Name: "read"},
		{Name: "respond", Caps: nil}, // 靠**保留名**被认出（设计稿 F9）
		{Name: "write"},
		{Name: "ask_user", Caps: []ToolCap{ToolCapAsksHuman}}, // 靠**能力标记**被认出
		{Name: "spawn_agent"},
	}
}

func toolNames(ts []Tool) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

func containsName(ts []Tool, name string) bool {
	for _, t := range ts {
		if t.Name == name {
			return true
		}
	}
	return false
}

// postureEngine — 一个能同时产出"地板 ask / 普通 ask / allow / deny"四类判定的引擎（用于 dontAsk 用例）。
func postureEngine(t *testing.T) *Engine {
	t.Helper()
	return mustLoad(t,
		// 普通规则：allow（read 工作区内）+ ask（write .env）+ deny（bash rm -rf ~）+ allow（bash git status）
		"allow read(./src/**)\n"+
			"ask write(./.env)\n"+
			"deny bash(rm -rf ~)\n"+
			"allow bash(git status)\n",
		// 地板：ask（装包）
		"ask bash(npm install*)",
	)
}

// ── ⑦ dontAsk：凡会弹窗的一律拒 ─────────────────────────────────────────────

func TestPosture_DontAskTurnsEveryAskIntoDeny(t *testing.T) {
	e := postureEngine(t)
	dontAsk := Posture{Mode: PostureDontAsk, InheritedByFanout: true}

	cases := []struct {
		name string
		req  Request
		// wantAsk 为真时断言：裸引擎判出 ask，加了 dontAsk 之后变 deny（"会弹窗的"都被拒）
		wantAsk bool
	}{
		{"普通 ask 命中（write .env）", Request{Tool: "write", Args: map[string]any{"path": "./.env"}}, true},
		{"地板 ask 命中（npm install）", Request{Tool: "bash", Args: map[string]any{"command": "npm install lodash"}}, true},
		{"无规则命中 ⇒ 默认档 ask", Request{Tool: "bash", Args: map[string]any{"command": "ls -la"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bare := e.Decide(tc.req)
			if tc.wantAsk && bare.Decision != EffectAsk {
				t.Fatalf("前置不成立：裸引擎应判 ask，实际 %q（reason=%s）", bare.Decision, bare.Reason)
			}
			got := ApplyPostureVerdict(bare, dontAsk)
			if got.Decision != EffectDeny {
				t.Errorf("dontAsk 下「会弹窗的」判定 = %q，期望 deny（%s）", got.Decision, got.Note)
			}
			if got.Reason != ReasonDontAskDeny {
				t.Errorf("原因码 = %q，期望 %q（同一件事实只允许一个原因码）", got.Reason, ReasonDontAskDeny)
			}
			if got.MatchedRule == nil && bare.MatchedRule != nil {
				t.Error("dontAsk 把命中规则抹掉了 ⇒ 审计无法回答「本来是哪条要问人」")
			}
			if !strings.Contains(got.Note, "dontAsk") {
				t.Errorf("说明里应写明是姿态改写的：%q", got.Note)
			}
		})
	}

	// 对侧（同样重要）：**不是 ask 的一律一字不动** —— 不许把 allow 变 deny（那是"全拒"，不是 dontAsk）。
	for _, req := range []Request{
		{Tool: "read", Args: map[string]any{"path": "./src/a.go"}},    // allow
		{Tool: "bash", Args: map[string]any{"command": "git status"}}, // allow
		{Tool: "bash", Args: map[string]any{"command": "rm -rf ~"}},   // deny（地板之上的普通 deny）
	} {
		bare := e.Decide(req)
		got := ApplyPostureVerdict(bare, dontAsk)
		if got.Decision != bare.Decision || got.Reason != bare.Reason || got.MatchedRule != bare.MatchedRule {
			t.Errorf("dontAsk 改写了非 ask 的判定（%s ⇒ %s / reason %s ⇒ %s）⇒ 这不是 dontAsk，是全拒",
				bare.Decision, got.Decision, bare.Reason, got.Reason)
		}
	}
	// 反例：非法判定值不该被本层"顺手修正"（本层只认 ask 这一种改写）。
	weird := Verdict{Decision: Effect("maybe"), Reason: "x"}
	if got := ApplyPostureVerdict(weird, dontAsk); got != weird {
		t.Errorf("非法的 decision 被改写了：%+v", got)
	}
}

// ── ⑧ yolo：工具表里没有问人工具，普通工具不变 ───────────────────────────────

func TestPosture_YoloRemovesAskToolsOnly(t *testing.T) {
	orig := postureToolset()
	yolo := Posture{Mode: PostureYolo, InheritedByFanout: true}

	got := ApplyPosture(orig, yolo)

	// 正例一：问人类工具（保留名 respond / 能力标记 ask_user）都不在返回的工具表里。
	if containsName(got.Tools, "respond") || containsName(got.Tools, "ask_user") {
		t.Errorf("yolo 下工具表仍含问人工具 ⇒ 模型还会白跑一轮：%v", toolNames(got.Tools))
	}
	// 正例二：**普通工具一个不少、且顺序不变**（只摘问人，不重排、不顺手清场）。
	want := []string{"bash", "read", "write", "spawn_agent"}
	if strings.Join(toolNames(got.Tools), ",") != strings.Join(want, ",") {
		t.Errorf("普通工具表应原序原样保留 %v，实际 %v", want, toolNames(got.Tools))
	}
	// 正例三：摘了什么必须可枚举，且说明里写明"这是有意的"。
	if strings.Join(got.Removed, ",") != "respond,ask_user" {
		t.Errorf("摘除清单 = %v，期望 [respond ask_user]", got.Removed)
	}
	if !strings.Contains(got.Note, "有意") {
		t.Errorf("说明里必须写明这是有意的（用户看到工具变少要能一眼看到原因）：%q", got.Note)
	}
	// 反例：**入参不许被就地修改** —— "摘掉"只发生在返回的视图上，全局注册表不能被污染。
	if len(orig) != 6 || !containsName(orig, "respond") || !containsName(orig, "ask_user") {
		t.Errorf("yolo 修改了传入的工具表本身（一次 yolo 会永久污染调用方）：%v", toolNames(orig))
	}
	// 反例：不许误摘 —— 名字只是"像"（前缀/拼写相近）或没标记的工具必须留着。
	near := []Tool{
		{Name: "respond_log"}, // 前缀相近，不是问人工具
		{Name: "askqestion"},  // 拼错的名字不该命中
		{Name: "screenshot"},  // 无标记
		{Name: "Reject"},      // 大小写不敏感 ⇒ 保留名，应被摘
		{Name: "ask_user_v2"}, // 前缀相近，不是保留名
	}
	nearGot := ApplyPosture(near, yolo)
	if strings.Join(toolNames(nearGot.Tools), ",") != "respond_log,askqestion,screenshot,ask_user_v2" {
		t.Errorf("名字匹配过宽（模糊/前缀匹配）⇒ 摘掉了不该摘的：%v", toolNames(nearGot.Tools))
	}
	if strings.Join(nearGot.Removed, ",") != "Reject" {
		t.Errorf("保留名匹配应大小写不敏感且**逐字**：摘除清单 = %v，期望 [Reject]", nearGot.Removed)
	}
	// 反例：表里本来就没有问人工具时，摘 0 个（不许"空表也报摘过东西"）。
	empty := ApplyPosture([]Tool{{Name: "bash"}, {Name: "read"}}, yolo)
	if len(empty.Removed) != 0 || len(empty.Tools) != 2 {
		t.Errorf("没有问人工具时不应摘任何东西：removed=%v tools=%v", empty.Removed, toolNames(empty.Tools))
	}
	// dontAsk 只管判定，**不动工具表**（否则两档语义会糊在一起）。
	dontAskSet := ApplyPosture(orig, Posture{Mode: PostureDontAsk})
	if len(dontAskSet.Removed) != 0 || strings.Join(toolNames(dontAskSet.Tools), ",") != strings.Join(toolNames(orig), ",") {
		t.Errorf("dontAsk 改动了工具表（摘工具是 yolo 的语义）：%v", dontAskSet.Removed)
	}
}

// ── ⑨ 反例：normal 姿态是惰性的 ─────────────────────────────────────────────

func TestPosture_NormalIsInert(t *testing.T) {
	e := postureEngine(t)
	normal := Posture{Mode: PostureNormal} // 常态：不声明继承（子代理也不继承）

	// (a) 不摘任何工具（含问人工具 —— 常态下问人工具本来就该在，模型需要它）。
	orig := postureToolset()
	got := ApplyPosture(orig, normal)
	if len(got.Removed) != 0 {
		t.Errorf("normal 姿态摘了工具：%v ⇒ 装一层姿态不许悄悄削掉能力", got.Removed)
	}
	if strings.Join(toolNames(got.Tools), ",") != strings.Join(toolNames(orig), ",") {
		t.Errorf("normal 姿态改动/重排了工具表：%v", toolNames(got.Tools))
	}
	// (b) 不把 ask 变 deny，也不把 ask 变 allow、不改 deny、不改 allow。
	for _, req := range []Request{
		{Tool: "write", Args: map[string]any{"path": "./.env"}},     // ask
		{Tool: "bash", Args: map[string]any{"command": "ls -la"}},   // 默认 ask
		{Tool: "read", Args: map[string]any{"path": "./src/a.go"}},  // allow
		{Tool: "bash", Args: map[string]any{"command": "rm -rf ~"}}, // deny
	} {
		bare := e.Decide(req)
		if after := ApplyPostureVerdict(bare, normal); after != bare {
			t.Errorf("normal 姿态改写了判定（%s ⇒ %s）：装姿态不许有副作用", bare.Decision, after.Decision)
		}
		// 同样钉住 yolo 不改写判定（摘工具≠改判定；残留 ask 的处理属 T5.8，本批不猜）。
		if after := ApplyPostureVerdict(bare, Posture{Mode: PostureYolo}); after != bare {
			t.Errorf("yolo 改写了判定（%s ⇒ %s）：yolo 只摘问人工具；把 ask 改写成 allow 会越过地板",
				bare.Decision, after.Decision)
		}
	}
	// (c) 空姿态（Mode == ""）**不是** normal：判不了 ⇒ 按最严（会弹窗的一律拒）。
	unknown := Posture{}
	ask := e.Decide(Request{Tool: "bash", Args: map[string]any{"command": "ls -la"}})
	if ask.Decision != EffectAsk {
		t.Fatal("前置：这条应判 ask")
	}
	if got := ApplyPostureVerdict(ask, unknown); got.Decision != EffectDeny {
		t.Errorf("未设/认不出的姿态取值应**按最严**（ask ⇒ deny），实际 %q ⇒ 静默降级成常态 = 无人值守变全放", got.Decision)
	}
	// 但**不许**顺手摘工具表（摘工具是显式意图才做的事，不该由一次拼写错误顺带做掉）。
	unknownSet := ApplyPosture(orig, unknown)
	if len(unknownSet.Removed) != 0 || len(unknownSet.Tools) != len(orig) {
		t.Errorf("认不出的姿态取值改动了工具表：removed=%v", unknownSet.Removed)
	}
}

// ── ⑩ 跨扇出继承 + 跨重启存活 ───────────────────────────────────────────────

func TestPosture_InheritAlongFanoutAndRestart(t *testing.T) {
	// (a) 声明继承 ⇒ 子代理拿到同一个姿态（无人值守姿态盖住整棵扇出树）。
	yoloInherit := Posture{Mode: PostureYolo, InheritedByFanout: true}
	child := yoloInherit.Inherit()
	if child.Mode != PostureYolo || !child.InheritedByFanout {
		t.Errorf("声明继承的姿态没有传给子代理：%+v", child)
	}
	// 再往下一层仍继承（扇出可以有多级）。
	if grand := child.Inherit(); grand.Mode != PostureYolo {
		t.Errorf("孙代理丢了姿态 ⇒ 只继承了一层：%+v", grand)
	}
	// (b) 反例：**没声明**继承 ⇒ 子代理回到常态（一次开关不许静默放大到整棵子树）。
	child2 := Posture{Mode: PostureYolo, InheritedByFanout: false}.Inherit()
	if child2.Mode != PostureNormal {
		t.Errorf("未声明继承却把姿态传给了子代理（默认继承 = 用户在视线外被放大）：%+v", child2)
	}
	// 反例第二面：父姿态**非法**时，子代理拿"最严"而不是"常态"（非法值不因扇出变宽）。
	child3 := Posture{Mode: "yolooo", InheritedByFanout: true}.Inherit()
	if child3.Mode != PostureDontAsk {
		t.Errorf("父姿态非法时子代理应拿最严（dontAsk），实际 %+v", child3)
	}

	// (c) 跨重启：序列化 ⇒ 反序列化后仍是同一个事实（无人值守不会因为重启而变成有人值守）。
	for _, p := range []Posture{yoloInherit, {Mode: PostureDontAsk, InheritedByFanout: true}, {Mode: PostureNormal}} {
		data, err := MarshalPosture(p)
		if err != nil {
			t.Fatalf("姿态序列化失败：%v", err)
		}
		back, err := UnmarshalPosture(data)
		if err != nil {
			t.Fatalf("姿态反序列化失败：%v", err)
		}
		if back != p {
			t.Errorf("重启继承失败：写出 %+v，读回 %+v", p, back)
		}
	}
	// 反例：坏 JSON / 不认识的档位 ⇒ **报错**，绝不静默载成常态。
	if _, err := UnmarshalPosture([]byte("{不是 JSON")); err == nil {
		t.Error("坏 JSON 没有报错 ⇒ 会静默降级（无人值守悄悄变成常态）")
	}
	if _, err := UnmarshalPosture([]byte(`{"mode":"yolooo","inherited_by_fanout":true}`)); err == nil {
		t.Error("不认识的姿态档位没有报错 ⇒ 按未知档位解释（等于猜）")
	}
	// 解析侧：空串 = 常态；拼错的档位报错（配置期不猜）。
	if m, err := ParsePostureMode(""); err != nil || m != PostureNormal {
		t.Errorf("空串应解析成常态：%q %v", m, err)
	}
	if m, err := ParsePostureMode("  DONTASK "); err != nil || m != PostureDontAsk {
		t.Errorf("姿态文本应大小写不敏感、去空白：%q %v", m, err)
	}
	for _, bad := range []string{"yolooo", "none", "autopilot"} {
		if _, err := ParsePostureMode(bad); err == nil {
			t.Errorf("认不出的姿态「%s」应报错（配置期拒绝，不许猜）", bad)
		}
	}
	// 保留名回显是可枚举的、且是副本（UI 要能列出"哪些名字会被摘掉"）。
	names := AsksHumanReservedNames()
	if len(names) == 0 {
		t.Fatal("保留名列表为空 ⇒ yolo 的名字兜底形同不存在")
	}
	names[0] = "被改了"
	if AsksHumanReservedNames()[0] == "被改了" {
		t.Error("AsksHumanReservedNames 返回的是内部切片 ⇒ 调用方改返回值会改掉摘除判据")
	}
	// 姿态本身可 JSON 化（接线批要把它塞进会话状态/子代理派发参数）。
	if _, err := json.Marshal(Posture{Mode: PostureDontAsk, InheritedByFanout: true}); err != nil {
		t.Errorf("姿态应可 JSON 序列化：%v", err)
	}
}

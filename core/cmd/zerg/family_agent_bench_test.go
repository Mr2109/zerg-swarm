// family_agent_bench_test.go —— `zerg agent bench` 的**三态判据**与「按名字/端口认准引擎」的内部判据
// （缺口 `GAP-20260925-49` · 2026-09-26）。
//
// 为什么要有它：旧判据把两件**不同**的事混成一格 ——
//
//	(a) **思考草稿**（正文空 + 思考满 + `finish_reason=length`）：思考模型要烧 ~570 token 才吐正文，
//	    预算 96 时必然只有草稿 ⇒ 这是「**推理预算不够**」，不是「乱码」；
//	(b) **真乱码**（非中文/非英文的糊字符占比超阈值）。
//
// 旧实现两种都回 `readable=false` + 退 1 ⇒ **两种都是错判** ✗。本件把三态**逐态钉死**：
//
//	① 可读正文 / ② 只有思考（推理预算不够，未到正文）/ ③ 真乱码 —— 三态各自成立、**不许塌成一格**；
//	  ③ 的样本取自**缺口账里那次真跑**（`GAP-20260925-49` 手工复现的原文，逐字照抄 ——
//	  不许自己编一段「像乱码」的东西来凑绿）。
//	④ 引擎认准：**两枚在跑 + 不点名 ⇒ 退 2**（旧实现只取第一个 `port>0` 的卵 = 「抓碰巧第一个」✗）；
//	  点名 ⇒ 逐字认那一条（**不是**「先抓第一个再比名字」）。
package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// gap49Garbled —— 缺口账 `GAP-20260925-49` 那次真跑的**原文**（乱码实据，逐字照抄）。
const gap49Garbled = "క로es lilt-e-xin-de-yi-ju-hua-sei-ming-shu-shu-chiller-zi-mu-shi-shen-m…"

// gap49Thinking —— 同一次现场那一档的**思考草稿**（正文空、思考满、finish_reason=length）。
const gap49Thinking = "用户要求「用一句话说明虫族是什么。」需要用中文一句话回答。需要判断「虫族」可能指：游戏/科幻中的虫族（如"

// TestAgentBenchClassify_ThreeStatesAreDistinct —— 三态逐态成立，且**两两不塌成一格**。
func TestAgentBenchClassify_ThreeStatesAreDistinct(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		reasoning    string
		finish       string
		wantVerdict  string
		wantReadable bool
		wantHasHint  bool
	}{
		{
			name:    "只有思考·推理预算不够（第二态：不是乱码）",
			content: "", reasoning: gap49Thinking, finish: "length",
			wantVerdict: "reasoning_only", wantReadable: false,
		},
		{
			name:    "真乱码（第三态：缺口账原文）",
			content: gap49Garbled, finish: "stop",
			wantVerdict: "garbled", wantReadable: false,
		},
		{
			name:        "可读正文（第一态）",
			content:     "虫族是科幻作品（如《星际争霸》）中由虫母统一意志控制、以无限增殖和吞噬有机生命为特征的异星生物集合体。",
			finish:      "stop",
			wantVerdict: "readable", wantReadable: true,
		},
		{
			name:        "空输出（正文与思考都没有 —— 空 ≠ 乱码）",
			finish:      "stop",
			wantVerdict: "empty", wantReadable: false,
		},
		{
			name:        "全英文的干净回答：正文照样按可读过，只给口径提示",
			content:     "The zerg are a fictional alien species dominated by a single hive mind.",
			finish:      "stop",
			wantVerdict: "readable", wantReadable: true, wantHasHint: true,
		},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verdict, readable, reasons := agentBenchClassify(c.content, c.reasoning, c.finish)
			if verdict != c.wantVerdict {
				t.Fatalf("verdict = %q（要 %q）· reasons=%v", verdict, c.wantVerdict, reasons)
			}
			if readable != c.wantReadable {
				t.Fatalf("readable = %t（要 %t）· reasons=%v", readable, c.wantReadable, reasons)
			}
			if c.wantHasHint && len(reasons) == 0 {
				t.Fatalf("要有一条口径提示（全英文回答那一格），却一条 warnings 都没有")
			}
			if !c.wantHasHint && verdict != "readable" && len(reasons) == 0 {
				t.Fatalf("verdict=%q 却没给理由（三态都要能说清为什么）", verdict)
			}
			// 三态**必须两两不同**（塌成一格 = 本批要修的病）。
			seen[verdict] = true
		})
	}
	for _, want := range []string{"readable", "reasoning_only", "garbled"} {
		if !seen[want] {
			t.Fatalf("三态里 %q 这一格没被任何一个用例走到（判据塌成一格了）", want)
		}
	}
}

// TestAgentBenchSplitThink —— 思考被内联进 `content`（`reasoning_format=none` 之类）时要能劈开：
// 不劈就会把「思考已出、正文未到」看成「正文里全是乱码」（`GAP-20260925-49` 的误判现场）。
func TestAgentBenchSplitThink(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantBody  string
		wantThink string
	}{
		{
			name:      "内联思考 + 正文",
			content:   "<think>\n用户问的是虫族……\n</think>\n\n虫族是科幻作品中的异星生物集合体。",
			wantBody:  "虫族是科幻作品中的异星生物集合体。",
			wantThink: "用户问的是虫族……",
		},
		{
			name:      "只开了没闭（预算烧在思考中途）",
			content:   "<think>\n用户要求一句话说明",
			wantBody:  "",
			wantThink: "用户要求一句话说明",
		},
		{
			name:     "没有内联思考 ⇒ 原样",
			content:  "虫族是科幻作品中的异星生物集合体。",
			wantBody: "虫族是科幻作品中的异星生物集合体。",
		},
		{
			name:     "形状认不出来的 `<think` ⇒ 原样，不猜",
			content:  "这里有个 <think 但没有尖括号结尾",
			wantBody: "这里有个 <think 但没有尖括号结尾",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, think := agentBenchSplitThink(c.content)
			if body != c.wantBody || think != c.wantThink {
				t.Fatalf("劈开后 = (%q, %q)，要 (%q, %q)", body, think, c.wantBody, c.wantThink)
			}
		})
	}
}

// agentStubChildForTest —— 一台**合成子端**（只回 `/eggs` 与 `/status` 两条只读面）。
func agentStubChildForTest(t *testing.T, eggsBody, statusBody string) agentAddr {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/eggs":
			_, _ = io.WriteString(w, eggsBody)
		case "/status":
			_, _ = io.WriteString(w, statusBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("合成子端地址解析失败：%v", err)
	}
	_, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("合成子端端口解析失败：%v", err)
	}
	port, _ := strconv.Atoi(portStr)
	return agentAddr{Host: u.Hostname(), Port: port}
}

// 两枚卵都在跑（`managed=true` 且 `port>0`）—— 「抓碰巧第一个」的现场就是这一份回据。
const twoEggsBody = `{"eggs":[
 {"egg_id":"gemma-4-26B","model":"gemma-4-26B","state":"idle_armed","port":9401,"managed":true},
 {"egg_id":"example-35b","model":"example-35b","state":"idle_armed","port":9400,"managed":true}
]}`

const statusTwoModels = `{"machine":"Mr2109","port":0,"model":"","models":["gemma-4-26B","example-35b","Qwen3.8-27B"]}`

// TestAgentEnginePick_NoFirstComeFirstServed —— ④ **两枚在跑 + 不点名 ⇒ 退 2**（不许挑碰巧第一个）；
// 点名 ⇒ **逐字认那一条**（哪怕它不是回据里的第一条）。
func TestAgentEnginePick_NoFirstComeFirstServed(t *testing.T) {
	child := agentStubChildForTest(t, twoEggsBody, statusTwoModels)
	inv := &invocation{}

	eggs, rc := agentRunningEggs(inv, "Mr2109", child, io.Discard)
	if rc != exitOK {
		t.Fatalf("读子端回据要退 0，得到 %d", rc)
	}
	if len(eggs) != 2 {
		t.Fatalf("回据里两枚在跑，只认出 %d 枚（旧写法「取第一个就返回」的病根）", len(eggs))
	}

	inv2 := &invocation{}
	if _, rc := agentEnginePick(inv2, "Mr2109", child, "", io.Discard); rc != exitUsage {
		t.Fatalf("两枚在跑、不点名 ⇒ 要退 2（要你指名道姓），得到 %d", rc)
	}

	inv3 := &invocation{}
	got, rc := agentEnginePick(inv3, "Mr2109", child, "example-35b", io.Discard)
	if rc != exitOK {
		t.Fatalf("点名在跑的那一枚 ⇒ 要退 0，得到 %d", rc)
	}
	if got.Port != 9400 || got.Model != "example-35b" {
		t.Fatalf("点名 example-35b ⇒ 认到的却是 %+v（回据里它排在第二条 —— 不许抓第一条）", got)
	}

	inv4 := &invocation{}
	if _, rc := agentEnginePick(inv4, "Mr2109", child, "Qwen3.8-27B", io.Discard); rc != exitBlocked {
		t.Fatalf("注册表里有、但没在跑 ⇒ 要退 8（不给结论），得到 %d", rc)
	}

	inv5 := &invocation{}
	if _, rc := agentEnginePick(inv5, "Mr2109", child, "no-such-model-zz", io.Discard); rc != exitUsage {
		t.Fatalf("注册表里没有 ⇒ 要退 2（不是错、是没有），得到 %d", rc)
	}
}

// TestAgentBenchVerdictNames_MatchVerdicts —— 人读名与机器面值**同一处定义**（三态的名字不许分家）。
func TestAgentBenchVerdictNames_MatchVerdicts(t *testing.T) {
	for _, v := range []string{"readable", "reasoning_only", "garbled", "empty"} {
		if got := agentBenchVerdictName(v); got == v || got == "" {
			t.Fatalf("verdict %q 的人读名没定义（得到 %q）", v, got)
		}
	}
}

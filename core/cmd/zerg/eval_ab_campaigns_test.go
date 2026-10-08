// eval_ab_campaigns_test.go —— 件 eval_ab_campaigns.go 的成对负控（✗ 禁恒绿 · 前缀 TestEabc）。
//
//	正控：以 httptest 合成主控（本机随机端口 · **不碰生产 8580**）走完整条主控链路，断言 stdout 形状 +
//	     落档件结构（含 turn/tool/queued 聚合、citation_hit、end_reasons 排序）。
//	负控：把每条关键语义各钉一枚「反例」—— 日后谁把实现退化（argv 被读 / py 空白 · 行界被 Go 版顶替 /
//	     py float · round 换 Go 写法 / JSON 空集合变 null / 观测偏移语义改掉），对应用例立刻红。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eabcStub —— 合成主控：逐次请求记 method+path 与 X-Auth-Token；`/send` 时向状态件追加观测行。
type eabcStub struct {
	srv   *httptest.Server
	hits  []string
	toks  []string
	sess  int
	state string
}

func eabcNewStub(t *testing.T, state string) *eabcStub {
	t.Helper()
	st := &eabcStub{state: state}
	st.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st.hits = append(st.hits, r.Method+" "+r.URL.Path)
		st.toks = append(st.toks, r.Header.Get("X-Auth-Token"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/sessions":
			st.sess++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "{\"session_id\":\"S%02d\"}", st.sess)
		case strings.HasSuffix(r.URL.Path, "/model"):
			fmt.Fprint(w, "{}")
		case strings.HasSuffix(r.URL.Path, "/send"):
			f, err := os.OpenFile(st.state, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				f.WriteString("{\"kind\":\"turn\",\"round\":1,\"end_reason\":\"stop\",\"turn\":{\"verdict\":\"ok\"}}\n")
				f.WriteString("{\"kind\":\"turn\",\"round\":2,\"end_reason\":\"max_steps\",\"turn\":{\"verdict\":\"downgrade\"}}\n")
				f.WriteString("{\"kind\":\"tool\",\"tool\":\"read\"}\n")
				f.WriteString("{\"kind\":\"queued\",\"queued_ms\":12}\n")
				f.WriteString("{\"kind\":\"queued\",\"queued_ms\":30}\n")
				f.Close()
			}
			fmt.Fprint(w, "line1\nline2\nline3\n")
		case strings.HasSuffix(r.URL.Path, "/messages"):
			fmt.Fprint(w, `[{"role":"assistant","content":"Version = 2.5.9"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(st.srv.Close)
	return st
}

// eabcFixture —— 建仓外 HOME 夹：token + 状态件 + 落档目录齐全（py 的 import 期与收尾写档都要它们）。
func eabcFixture(t *testing.T) (home, state string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".zerg", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".zerg", "egg-profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zerg", "token"), []byte("tok-abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state = filepath.Join(home, ".zerg", "state", "chat_obs.jsonl")
	if err := os.WriteFile(state, []byte("{\"kind\":\"queued\",\"queued_ms\":5}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, state
}

func TestEabcFullRunAgainstStub(t *testing.T) {
	home, state := eabcFixture(t)
	st := eabcNewStub(t, state)
	t.Setenv("ZERG_CORE", st.srv.URL) // ★ 指合成主控 ⇒ 绝不碰生产 8580
	t.Setenv("ZERG_AB_RUNS", "3")

	var out, errb bytes.Buffer
	rc := evalABCampaignsGateMain(&out, &errb, nil)
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	so := out.String()
	for _, label := range []string{"课题一A-不打回", "课题一B-打回重做", "课题二甲-指派", "课题二乙-广播自愿"} {
		if n := strings.Count(so, "[ab] "+label+" 第 "); n != 3 {
			t.Fatalf("进度行 %q 出现 %d 次（要 3）：\n%s", label, n, so)
		}
	}
	if n := strings.Count(so, "   ⇒ 小结："); n != 4 {
		t.Fatalf("「⇒ 小结」应 4 行，得 %d", n)
	}
	if !strings.Contains(so, "[ab] ✓ 已写：") || !strings.Contains(so, "═══ 总览 ═══\n") {
		t.Fatalf("收尾行缺失：\n%s", so)
	}
	for _, k := range []string{"\"citation_hit\": true", "\"steps\": 2", "\"tool_calls\": 1", "\"queued_max_ms\": 30",
		"\"end_reasons\": [\"max_steps\", \"stop\"]", "\"verdicts\": [\"downgrade\", \"ok\"]", "\"wrote_anything\": false",
		"\"assistant_chars\": 15"} {
		if !strings.Contains(so, k) {
			t.Fatalf("stdout 缺 %s：\n%s", k, so)
		}
	}

	// 落档件：4 档 × (3 rows + summary)，与 py `json.dump(indent=2)` 同构。
	raw, err := os.ReadFile(filepath.Join(home, ".zerg", "egg-profiles", "ab-campaigns.json"))
	if err != nil {
		t.Fatalf("落档件读不到：%v", err)
	}
	if strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("落档件不应带尾换行（py json.dump 不加）")
	}
	var doc struct {
		RunsPerArm int `json:"runs_per_arm"`
		Arms       map[string]struct {
			Rows    []map[string]interface{} `json:"rows"`
			Summary map[string]interface{}   `json:"summary"`
		} `json:"arms"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("落档件不是合法 JSON：%v", err)
	}
	if doc.RunsPerArm != 3 || len(doc.Arms) != 4 {
		t.Fatalf("落档结构不对：runs=%d arms=%d", doc.RunsPerArm, len(doc.Arms))
	}
	for _, arm := range doc.Arms {
		if len(arm.Rows) != 3 {
			t.Fatalf("某档 rows=%d（要 3）", len(arm.Rows))
		}
		if arm.Summary["n"] != float64(3) {
			t.Fatalf("summary.n=%v", arm.Summary["n"])
		}
		if arm.Summary["citation_hit_rate"] != float64(1) {
			t.Fatalf("citation_hit_rate=%v（要 1）", arm.Summary["citation_hit_rate"])
		}
		if arm.Summary["tool_calls_median"] != float64(1) {
			t.Fatalf("tool_calls_median=%v", arm.Summary["tool_calls_median"])
		}
		if arm.Summary["queued_max_ms"] != float64(30) {
			t.Fatalf("queued_max_ms=%v", arm.Summary["queued_max_ms"])
		}
	}

	// 请求面：4 档 × 3 次 ×（sessions、model、send、messages）= 48 条，且**全部落在合成主控**。
	if len(st.hits) != 48 {
		t.Fatalf("请求数 %d（要 48）：%v", len(st.hits), st.hits)
	}
	created := 0
	for _, h := range st.hits {
		if h == "POST /api/chat/sessions" {
			created++
		}
	}
	if created != 12 {
		t.Fatalf("建会话请求数=%d（要 12）：%v", created, st.hits)
	}
	for _, tk := range st.toks {
		if tk != "tok-abc" {
			t.Fatalf("认证头未逐字带上（得 %q）", tk)
		}
	}
}

// 负控 ①：token 缺 ⇒ import 期异常 ⇒ rc=1（✗ 不许静默绿）。
func TestEabcTokenMissingFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // .zerg/token 不存在
	var out, errb bytes.Buffer
	rc := evalABCampaignsGateMain(&out, &errb, nil)
	if rc != 1 {
		t.Fatalf("rc=%d（要 1）", rc)
	}
	if !strings.Contains(errb.String(), "token") {
		t.Fatalf("stderr 未点名 token：%s", errb.String())
	}
}

// 负控 ②：ZERG_AB_RUNS 非整数 ⇒ py int() 抛 ⇒ rc=1。
func TestEabcBadRunsFailsClosed(t *testing.T) {
	_, _ = eabcFixture(t)
	t.Setenv("ZERG_AB_RUNS", "abc")
	var out, errb bytes.Buffer
	rc := evalABCampaignsGateMain(&out, &errb, nil)
	if rc != 1 {
		t.Fatalf("rc=%d（要 1）", rc)
	}
}

// 负控 ③：argv 一字不读 —— 无参 / --help / 未知参 三档 stdout 形状逐字同（py main() 不接参数）。
func TestEabcArgvIgnored(t *testing.T) {
	run := func(argv []string) string {
		_, state := eabcFixture(t)
		st := eabcNewStub(t, state)
		t.Setenv("ZERG_CORE", st.srv.URL)
		t.Setenv("ZERG_AB_RUNS", "1")
		var out, errb bytes.Buffer
		if rc := evalABCampaignsGateMain(&out, &errb, argv); rc != 0 {
			t.Fatalf("argv=%v rc=%d", argv, rc)
		}
		// 抹掉仅有的两处非确定面（真远墙钟 + 落档打印里的 HOME），只比「形状」。
		s := out.String()
		s = strings.ReplaceAll(s, filepath.Join(os.Getenv("HOME"), ".zerg"), "<HOME>/.zerg")
		return eabcNormalizeWall(s)
	}
	if a, b, c := run(nil), run([]string{"--help"}), run([]string{"--nope", "x"}); a != b || b != c {
		t.Fatalf("argv 三档不该分叉：\nnil=%s\n--help=%s\n未知=%s", a, b, c)
	}
}

// eabcNormalizeWall —— 把 wall_s 数值抹平（唯一非确定的数值面 · 时间本身不可逐字）。
func eabcNormalizeWall(s string) string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if i := strings.Index(ln, "\"wall_s\": "); i >= 0 {
			j := strings.Index(ln[i:], ",")
			if j > 0 {
				ln = ln[:i] + "\"wall_s\": X" + ln[i+j:]
			}
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// ---- py 原语同构的成对正/负控 ---- //

func TestEabcFileLinesMatchesPyIter(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a\n", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"a\n\n", []string{"a", ""}},
	}
	for _, c := range cases {
		got := eabcFileLines(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("eabcFileLines(%q)=%#v 期望 %#v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("eabcFileLines(%q)[%d]=%q 期望 %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestEabcPyStripUnicodeBlank(t *testing.T) {
	// 只有 NBSP / \x1c-1f 的行在 py 里是「空行」⇒ 不计分母。退化成 Go TrimSpace ⇒ NBSP 不剥 ⇒ 红。
	dir := t.TempDir()
	p := filepath.Join(dir, "obs.jsonl")
	os.WriteFile(p, []byte("\n  \n\u00a0\n\x1c\x1d\n{}\n"), 0o644)
	n, cerr := eabcCountObs(p)
	if cerr != nil || n != 1 {
		t.Fatalf("非空行数=%d err=%v（要 1 —— NBSP/\\x1c 行须判空）", n, cerr)
	}
}

func TestEabcPyCountLines(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{{"", 0}, {"a", 1}, {"a\n", 1}, {"a\nb", 2}, {"a\nb\n", 2}, {"a\n\n", 2}} {
		if got := eabcPyCountLines([]byte(c.in)); got != c.want {
			t.Fatalf("eabcPyCountLines(%q)=%d 期望 %d", c.in, got, c.want)
		}
	}
}

func TestEabcPyFloatAndRound(t *testing.T) {
	if got := eabcPyFloat(1.0); got != "1.0" {
		t.Fatalf("eabcPyFloat(1.0)=%q（py repr 带 .0）", got)
	}
	if got := eabcPyFloat(0.5); got != "0.5" {
		t.Fatalf("eabcPyFloat(0.5)=%q", got)
	}
	if got := eabcPyRound(1.0/3.0, 2); eabcPyFloat(got) != "0.33" {
		t.Fatalf("round(1/3,2)=%q", eabcPyFloat(got))
	}
	if got := eabcPyRound(2.0/3.0, 2); eabcPyFloat(got) != "0.67" {
		t.Fatalf("round(2/3,2)=%q", eabcPyFloat(got))
	}
}

func TestEabcJSONStringEscaping(t *testing.T) {
	if got := eabcJSONString("a\"b\\c\n\t\u00e9"); got != "\"a\\\"b\\\\c\\n\\t\u00e9\"" {
		t.Fatalf("eabcJSONString=%q", got)
	}
}

// 负控 ④：空集合必须是 []（✗ 不许变 null）；空小结必须是 {}（✗ 不许变 null）。
func TestEabcEmptyCollectionsAreNotNil(t *testing.T) {
	var b strings.Builder
	eabcEmitJSON(eabcNewObj().set("end_reasons", []interface{}{}), "", false, &b)
	if b.String() != `{"end_reasons": []}` {
		t.Fatalf("空集合渲染=%q（要 []）", b.String())
	}
	b.Reset()
	eabcEmitJSON(eabcSummarize(nil), "", false, &b)
	if b.String() != "{}" {
		t.Fatalf("空 rows 小结=%q（要 {}）", b.String())
	}
}

// 负控 ⑤：median 的 int/float 两态（奇数取中位 int、偶数取均值 float —— py statistics.median 口径）。
func TestEabcMedianIntFloat(t *testing.T) {
	if v := eabcMedianInts([]int64{5, 1, 3}); v != int64(3) {
		t.Fatalf("奇数 median=%v（要 int 3）", v)
	}
	if v := eabcMedianInts([]int64{1, 2, 3, 4}); v != 2.5 {
		t.Fatalf("偶数 median=%v（要 float 2.5）", v)
	}
}

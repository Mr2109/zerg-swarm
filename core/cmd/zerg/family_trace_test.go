// family_trace_test.go —— `zerg trace`（CA 全量追踪 · 批 B1 归入命令面）的判据。
//
// 落点：`package main_test` + `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品）。
// 全部合成盘面（`t.TempDir()` 当日志目录）：不碰真 `~/.zerg/state`、不起 `zerg-trace` 二进制。
//
// 本件钉住的六格：
//
//	① 命令树里**真有**这条命令（path `trace` · 旗标逐字：`--live` / `--json` / `--step` /
//	   `--tokens` / `--errors` · 字段表 = `summary,steps,metrics`）；
//	② 默认视图（时间线 + 总览）：事件数 / 轮数 / token 累计 / `❄冷启动` 标记 / 机器指标联动；
//	③ `--step N` 明细 · `--tokens` 逐轮 · `--errors` 清单（三视图各一格）；
//	④ `--json`：裸形 = 源件那份 `{summary,steps,metrics}` 三段文档；给字段 = 只出点名段；
//	   点错字段 ⇒ rc=2 + `合法字段:` 表（门73/字段面对拍取数面）；
//	⑤ 用法面：缺 `<logdir>` ⇒ 2 · `--step` 不是正整数 ⇒ 2 · 未知旗标 ⇒ 2 · 日志目录读不到 ⇒ 1；
//	⑥ **只读**：跑前/跑后日志目录里两件的 sha256 逐字不变。
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// traceRun 跑一次 `trace`（合成盘面）。
func traceRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// traceFixture 造一个日志目录：3 条事件（2 轮）＋ 1 条机器采样。
//
//	step1: model_call（120 tok · 2s · 冷启动）＋ tool_call（bash · 3s）
//	step2: model_call（80 tok · 4s）＋ tool_error（error 非空）
//
// 合计：events=4 · steps=2 · tokens=200 · model time=6s · tool time=3s
func traceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	events := []string{
		`{"Seq":1,"Type":"model_call","Action":"round1","Step":1,"Duration":"2s","Timestamp":"2026-10-20T10:00:00Z","Args":{"tokens":120,"cold_start":true}}`,
		`{"Seq":2,"Type":"tool_call","Action":"run","ToolName":"bash","Step":1,"Duration":"3s","Timestamp":"2026-10-20T10:00:03Z"}`,
		`{"Seq":3,"Type":"model_call","Action":"round2","Step":2,"Duration":"4s","Timestamp":"2026-10-20T10:00:10Z","Args":{"tokens":80}}`,
		`{"Seq":4,"Type":"tool_error","Action":"boom","ToolName":"bash","Step":2,"Error":"exit 1","Timestamp":"2026-10-20T10:00:20Z"}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(strings.Join(events, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	metrics := `{"ts":"2026-10-20T10:00:00Z","step":1,"local_cpu":42,"x3_gpu":7,"disk_read_mbps":1.5,"disk_write_mbps":2.5}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sysmetrics.jsonl"), []byte(metrics), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ① 命令树登记面：path / 旗标逐字 / 字段表 —— 三条一起钉（旗标漏一枚即红）。
func TestTraceCommandRegistered(t *testing.T) {
	c, ok := zerg.CommandInfoOfForTest("trace")
	if !ok {
		t.Fatalf("命令树里没有 `zerg trace`（批 B1 的核心那一条）")
	}
	usage := c.Usage
	for _, f := range []string{"--live", "--json", "--step", "--tokens", "--errors"} {
		if !strings.Contains(usage, f) {
			t.Errorf("用法串漏了旗标 %s（源件五枚必须逐字保留）：%q", f, usage)
		}
	}
	if !strings.Contains(usage, "<logdir>") {
		t.Errorf("用法串没写必填位置参数 `<logdir>`：%q", usage)
	}
	got := strings.Join(c.Fields, ",")
	if got != "summary,steps,metrics" {
		t.Errorf("字段表 = %q（要 `summary,steps,metrics` —— 按 JSON 输出的实际三段顶层键）", got)
	}
}

// ② 默认视图：总览 + 时间线（冷启动标记 / 机器指标联动）。
func TestTraceTimelineView(t *testing.T) {
	dir := traceFixture(t)
	rc, out, errb := traceRun(t, "trace", dir)
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	for _, want := range []string{
		"═══ CA full trace ═══",
		"events: 4 | steps: 2 | tokens: 200",
		"model time: 6s | tool time: 3s",
		"step1 ",
		"❄冷启动",
		"bash[tool_call]",
		"CPU42% GPU7% disk1.5/2.5MB/s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("时间线缺 %q\n—— 实得：\n%s", want, out)
		}
	}
}

// ③ 三视图：`--step N` 明细 / `--tokens` 逐轮 / `--errors` 清单。
func TestTraceStepTokensErrorsViews(t *testing.T) {
	dir := traceFixture(t)

	rc, out, _ := traceRun(t, "trace", dir, "--step", "2")
	if rc != 0 {
		t.Fatalf("`--step 2` rc=%d（要 0）", rc)
	}
	if !strings.Contains(out, "═══ step 2 detail") || !strings.Contains(out, "error: exit 1") {
		t.Errorf("`--step 2` 明细不对：\n%s", out)
	}
	if strings.Contains(out, "round1") {
		t.Errorf("`--step 2` 里混进了第 1 轮的事件：\n%s", out)
	}

	rc, out, _ = traceRun(t, "trace", dir, "--step", "9")
	if rc != 0 || !strings.Contains(out, "step 9 does not exist") {
		t.Errorf("不存在的轮次要照实说（rc=0 + `step 9 does not exist`）：rc=%d out=%q", rc, out)
	}

	rc, out, _ = traceRun(t, "trace", dir, "--tokens")
	if rc != 0 {
		t.Fatalf("`--tokens` rc=%d（要 0）", rc)
	}
	if !strings.Contains(out, "═══ token analysis (per step) ═══") ||
		!strings.Contains(out, "1      120        2s         60.0         120") {
		t.Errorf("`--tokens` 读数不对（逐轮 tokens / 速度 / 累计）：\n%s", out)
	}

	rc, out, _ = traceRun(t, "trace", dir, "--errors")
	if rc != 0 {
		t.Fatalf("`--errors` rc=%d（要 0）", rc)
	}
	if !strings.Contains(out, "═══ error list ═══") || !strings.Contains(out, "step2 ") || !strings.Contains(out, "error: exit 1") {
		t.Errorf("`--errors` 清单不对：\n%s", out)
	}
}

// ④ `--json` 两面：裸形（三段文档）＋ 点名字段（只出点名段）＋ 点错字段（rc=2 + 合法字段表）。
func TestTraceJSONFaces(t *testing.T) {
	dir := traceFixture(t)

	rc, out, errb := traceRun(t, "trace", dir, "--json")
	if rc != 0 {
		t.Fatalf("裸 `--json` rc=%d（要 0）· stderr=%s", rc, errb)
	}
	var doc struct {
		Summary struct {
			TotalEvents int
			TotalSteps  int
			TotalTokens int
		}
		Steps   []map[string]any `json:"steps"`
		Metrics []map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("裸 `--json` 不是合法 JSON：%v\n%s", err, out)
	}
	if doc.Summary.TotalEvents != 4 || doc.Summary.TotalSteps != 2 || doc.Summary.TotalTokens != 200 {
		t.Errorf("裸 `--json` 的 summary 不对（要 4/2/200）：%+v", doc.Summary)
	}
	if len(doc.Steps) != 2 || len(doc.Metrics) != 1 {
		t.Errorf("裸 `--json` 的三段不齐：steps=%d metrics=%d（要 2/1）", len(doc.Steps), len(doc.Metrics))
	}

	rc, out, _ = traceRun(t, "trace", dir, "--json", "summary")
	if rc != 0 {
		t.Fatalf("`--json summary` rc=%d（要 0）", rc)
	}
	var only map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &only); err != nil {
		t.Fatalf("`--json summary` 不是合法 JSON：%v\n%s", err, out)
	}
	if len(only) != 1 {
		t.Errorf("`--json summary` 该只出一段，实得 %d 段：%s", len(only), out)
	}
	if _, ok := only["summary"]; !ok {
		t.Errorf("`--json summary` 里没有 summary：%s", out)
	}

	rc, out, errb = traceRun(t, "trace", dir, "--json", "summary,bogus")
	if rc != 2 {
		t.Errorf("点错字段 rc=%d（要 2 · 用法错）", rc)
	}
	if out != "" {
		t.Errorf("点错字段 stdout 要零字节（用法类错误不吐包封 · K2 甲档）：%q", out)
	}
	if !strings.Contains(errb, "合法字段: summary,steps,metrics") {
		t.Errorf("点错字段没列合法字段表（门73/字段面对拍靠这一行取数）：%q", errb)
	}
}

// ⑤ 用法面四格 ＋ 只读面一格。
func TestTraceUsageAndReadOnly(t *testing.T) {
	rc, out, errb := traceRun(t, "trace")
	if rc != 2 || out != "" {
		t.Errorf("缺 `<logdir>` ⇒ rc=2 且 stdout 0 字节（实得 rc=%d out=%q）", rc, out)
	}
	if !strings.Contains(errb, "zerg trace <logdir>") {
		t.Errorf("缺参的 stderr 要给可照抄的用法串：%q", errb)
	}

	dir := traceFixture(t)
	for _, bad := range []string{"0", "abc"} {
		rc, _, errb = traceRun(t, "trace", dir, "--step", bad)
		if rc != 2 {
			t.Errorf("`--step %s` rc=%d（要 2）", bad, rc)
		}
		if !strings.Contains(errb, "正整数") {
			t.Errorf("`--step %s` 的 stderr 没说清「要正整数」：%q", bad, errb)
		}
	}
	// `-3` 走解析面的「值旗标看着像旗标」那一档（值按空收 ⇒ 键在、值空）⇒ 仍 rc=2，
	// 但报话是「未知旗标 -3」而不是本命令的正整数判词（源件同档：负号开头本来就不是值）。
	if rc, _, _ = traceRun(t, "trace", dir, "--step", "-3"); rc != 2 {
		t.Errorf("`--step -3` rc=%d（要 2）", rc)
	}

	// 未知旗标（解析面兜底）：`--nope` 不属于本命令 ⇒ 2。
	if rc, _, _ = traceRun(t, "trace", dir, "--nope"); rc != 2 {
		t.Errorf("未知旗标 rc=%d（要 2）", rc)
	}

	// 日志目录读不到 ⇒ 1（源件同档：跑到了、没成功）。
	missing := filepath.Join(t.TempDir(), "nope")
	if rc, _, errb = traceRun(t, "trace", missing); rc != 1 {
		t.Errorf("`events.jsonl` 打不开 rc=%d（要 1）· stderr=%q", rc, errb)
	}

	// 只读：跑前/跑后两件 sha256 逐字不变。
	before := traceDirHash(t, dir)
	if rc, _, _ = traceRun(t, "trace", dir, "--tokens"); rc != 0 {
		t.Fatalf("只读面正控跑失败：rc=%d", rc)
	}
	if after := traceDirHash(t, dir); after != before {
		t.Errorf("`zerg trace` 动了日志目录（只读档）：%s ≠ %s", before, after)
	}
}

// traceDirHash —— 目录里两件的 sha256 拼接（只读面的对拍口径）。
func traceDirHash(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	for _, n := range []string{"events.jsonl", "sysmetrics.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		b.WriteString(hex.EncodeToString(sum[:]))
		b.WriteString(" ")
	}
	return b.String()
}

// family_gap_reread_test.go —— `gap reread`（只读面 · 判据复跑）的**机检**（缺口账 `GAP-20260927-248`）。
//
// 口径（与同族一致）：`package main` 内部测试包 —— 本命令**尚未接进命令树**（`main.go` 那张表由
// 父代理串行落）⇒ 直调实现 `cmdGapReread`（不起子进程、不碰真 `~/.zerg/state`）。
//
// 钉住的格（逐格对设计口径）：
//
//	① 两态（仓外夹具账）：真读数条 ⇒ `ok`；已漂条（rc 变）⇒ `reading_stale`（**能分出两类**）
//	② 解析不出的条目 ⇒ `skipped`（✗ 不报假 stale）
//	③ 写面 / 解析不出的取值命令 ⇒ `not_rereadable`（**不执行**）
//	④ 哑化负控：对拍关 ⇒ 回「无法判」（证明「分出 ok / stale」靠的就是那一处对拍）
//	⑤ 只读：全跑前后真源 `sha256` 逐字不变
//	⑥ 退码：reading_stale **不进退出码**（恒 0）；缺目标 / 账外 id / `--all` 与 id 同给 ⇒ 2
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// gapRereadInvocation —— 造一个 `gap reread` 的 invocation（本命令尚未接进命令树 ⇒ 直调实现）。
func gapRereadInvocation(argv []string) *invocation {
	inv := &invocation{path: []string{"gap", "reread"}, kv: map[string][]string{}}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--json":
			inv.jsonGiven = true
			if i+1 < len(argv) {
				i++
				inv.fields = splitFields(argv[i])
			}
		case a == "--all":
			inv.all = true
		case a == "--include-open":
			inv.kvSet("--include-open", "true")
		case a == "--limit":
			if i+1 < len(argv) {
				i++
				inv.kvSet("--limit", argv[i])
			}
		default:
			inv.args = append(inv.args, a)
		}
	}
	return inv
}

func gapRereadRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := cmdGapReread(gapRereadInvocation(argv), &out, &errb)
	return rc, out.String(), errb.String()
}

// gapRereadFixtureRec —— 造一条合法账行（闭集内的 state/impact/prio）。
func gapRereadFixtureRec(id, state, evidence string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（只读对拍）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": state,
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls",
		"found_at": "2026-10-01T00:00:00+08:00", "solved_at": "2026-10-01T01:00:00+08:00",
		"solved_evidence": evidence,
	}
}

func gapRereadWriteLedger(t *testing.T, dir string, recs []map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("夹具行序列化失败：%v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
	return p
}

func gapRereadFileSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读夹具真源失败：%v", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestGapRereadTwoStateAndNegativeControl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	// 仓外夹具账：两条真读数（rc 一致）+ 一条已漂（rc 变）+ 一条解析不出 + 一条取值命令不可跑。
	// ★ 夹具账用 `zerg gap ls --state 不做`（本账零 不做 ⇒ 真跑 rc=1）与 `--state 已解`
	//   （本账四条全 已解 ⇒ 真跑 rc=0）当两枚确定性的「取值命令」。
	recs := []map[string]any{
		gapRereadFixtureRec("GAP-20261001-01", "已解",
			"销案两态对拍 · 改前{cmd=zerg gap ls --state 不做 · rc=1 · reading=改前零 不做} · "+
				"改后{cmd=zerg gap ls --state 已解 · rc=0 · reading=改后该号列于已解} · 散文=夹具"),
		gapRereadFixtureRec("GAP-20261001-02", "已解",
			"销案两态对拍 · 改前{cmd=zerg gap ls --state 不做 · rc=1 · reading=改前零 不做} · "+
				"改后{cmd=zerg gap ls --state 不做 · rc=0 · reading=（已漂：原记零命中 · 现读一条）} · 散文=夹具"),
		gapRereadFixtureRec("GAP-20261001-03", "已解",
			"这段是散文：cmd=zerg gap ls · rc=0（故意缺 reading 那一格 ⇒ 解析不出）"),
		gapRereadFixtureRec("GAP-20261001-04", "已解",
			"改前{cmd=zerg nosuchcommand --x · rc=0 · reading=不存在的命令} · 散文=夹具"),
	}
	ledger := gapRereadWriteLedger(t, dir, recs)
	sha0 := gapRereadFileSHA(t, ledger)

	// ① 真读数条 ⇒ ok（**不出现** reading_stale）
	rc, out, errb := gapRereadRun(t, "GAP-20261001-01")
	if rc != 0 {
		t.Fatalf("真读数条要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "[ok]") {
		t.Errorf("真读数条要出现 `[ok]`：%q", out)
	}
	if strings.Contains(out, "["+gapRereadStale+"]") {
		t.Errorf("真读数条**不许**出现 reading_stale：%q", out)
	}

	// ① 已漂条 ⇒ reading_stale（rc 变）
	rc, out, _ = gapRereadRun(t, "GAP-20261001-02")
	if rc != 0 {
		t.Errorf("reading_stale **不进退出码**（要 rc=0），得到 %d", rc)
	}
	if !strings.Contains(out, "["+gapRereadStale+"]") {
		t.Errorf("已漂条要出现 reading_stale：%q", out)
	}

	// ② 解析不出 ⇒ skipped（✗ 不报假 stale）
	rc, out, _ = gapRereadRun(t, "GAP-20261001-03")
	if rc != 0 || !strings.Contains(out, "[skipped]") {
		t.Errorf("解析不出的条目要记 skipped 且 rc=0：rc=%d out=%q", rc, out)
	}
	if strings.Contains(out, "["+gapRereadStale+"]") {
		t.Errorf("解析不出的条目**不许**报假 stale：%q", out)
	}

	// ③ 取值命令不可跑 ⇒ not_rereadable（不执行）
	rc, out, _ = gapRereadRun(t, "GAP-20261001-04")
	if rc != 0 || !strings.Contains(out, "["+gapRereadNoRun+"]") {
		t.Errorf("不可跑的取值命令要记 not_rereadable 且 rc=0：rc=%d out=%q", rc, out)
	}

	// 全账聚合：读数 6 · ok 3 · reading_stale 1 · not_rereadable 1 · skipped 1
	rc, out, errb = gapRereadRun(t, "--all")
	if rc != 0 {
		t.Fatalf("--all 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	for _, want := range []string{"ok 3", "reading_stale 1", "not_rereadable 1", "skipped 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("--all 汇总缺 %q：%q", want, out)
		}
	}

	// ④ 哑化负控：对拍关 ⇒ 回「无法判」（不再分 ok / stale）
	gapRereadCompareOn = false
	rc, out, _ = gapRereadRun(t, "GAP-20261001-02")
	gapRereadCompareOn = true
	if rc != 0 {
		t.Errorf("哑化档也要 rc=0，得到 %d", rc)
	}
	if strings.Contains(out, "["+gapRereadStale+"]") {
		t.Errorf("对拍关**不许**再分出 reading_stale：%q", out)
	}
	if !strings.Contains(out, "["+gapRereadUnknown+"]") {
		t.Errorf("对拍关要回「%s」：%q", gapRereadUnknown, out)
	}

	// ⑤ 只读：全跑前后真源 sha256 逐字不变（本面一字节都不写真源）
	if got := gapRereadFileSHA(t, ledger); got != sha0 {
		t.Errorf("本面必须**只读**（真源 sha 变了）：%s → %s", sha0, got)
	}

	// ⑥ 用法面
	if rc, _, _ := gapRereadRun(t); rc != 2 {
		t.Errorf("缺目标 ⇒ 2，得到 %d", rc)
	}
	if rc, _, _ := gapRereadRun(t, "--all", "GAP-20261001-01"); rc != 2 {
		t.Errorf("`--all` 与 id 同给 ⇒ 2，得到 %d", rc)
	}
	if rc, _, _ := gapRereadRun(t, "GAP-不存在"); rc != 2 {
		t.Errorf("账外 id ⇒ 2，得到 %d", rc)
	}
}

// TestGapRereadRealLedgerProportion —— **真账占比读数**（只报告 · 不改账）。
// 默认**跳过**（本仓判据件要 hermetic ⇒ 不连真 `~/.zerg/state`）；要跑就显式 `ZERG_REREAD_REAL=1`。
// `ZERG_REREAD_LIMIT` 卡读数条数（缺省 0 = 全量）。
func TestGapRereadRealLedgerProportion(t *testing.T) {
	if os.Getenv("ZERG_REREAD_REAL") == "" {
		t.Skip("设 ZERG_REREAD_REAL=1 才跑真账占比（默认跳过 —— 不连真状态目录）")
	}
	argv := []string{"--all"}
	if v := strings.TrimSpace(os.Getenv("ZERG_REREAD_LIMIT")); v != "" {
		argv = append(argv, "--limit", v)
	}
	rc, out, errb := gapRereadRun(t, argv...)
	if rc != 0 {
		t.Fatalf("真账占比要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	t.Logf("真账占比读数（本跑现读）：\n%s", out)
}

// TestGapRereadExternalWriteSelfDiscovery —— **外写自发现**（缺口 `GAP-20261005-44`）两态。
//
//	① 无外部写入：快照 sha16 == 退出前重读 sha16 ⇒ `external_write_detected=false`
//	② 有外部写入（机检钩子在两次读数之间改账）：两 sha 不等 ⇒ `external_write_detected=true`（人面/机器面同印）
//
// 负控（哑化）：把 `extWrite := exitSHA16 != snapSHA16` 改成 `false` ⇒ 态②的漂变**不可见**
// （本牙靠 `go test -overlay` 的哑化两态验证：不带 overlay 应绿、带哑化 overlay 应红）。
func TestGapRereadExternalWriteSelfDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gapRereadFixtureRec("GAP-20261005-44", "已解",
			"销案两态对拍 · 改前{cmd=zerg gap ls --state 不做 · rc=1 · reading=零 不做} · 散文=夹具"),
	}
	ledger := gapRereadWriteLedger(t, dir, recs)

	// ① 无外部写入 ⇒ external_write_detected=false（两 sha16 相等且都印出）
	gapRereadBetweenReads = nil
	rc, out, errb := gapRereadRun(t, "--all")
	if rc != 0 {
		t.Fatalf("无外写态要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "external_write_detected=false") {
		t.Errorf("无外部写入要报 external_write_detected=false：%q", out)
	}
	if s, b := gapRereadSHAsInHuman(out); s == "" || b == "" || s != b {
		t.Errorf("无外部写入要两 sha16 相等且都印出：snapshot=%q readback=%q", s, b)
	}
	// 机器面（`--json` 的 meta 格）同印
	rc, jout, jerr := gapRereadRun(t, "--all", "--json", "id")
	if rc != 0 {
		t.Fatalf("--json 要 rc=0，得到 %d · stderr=%q", rc, jerr)
	}
	if !strings.Contains(jout, "\"external_write_detected\":false") {
		t.Errorf("--json 的 meta 要带 external_write_detected=false：%q", jout)
	}

	// ② 有外部写入 ⇒ external_write_detected=true（两 sha16 不等）
	shaBefore := gapRereadFileSHA(t, ledger)
	gapRereadBetweenReads = func() {
		extra := gapRereadFixtureRec("GAP-20261005-44-EXT", "仍缺", "外部进程在两次读数之间追加的一条")
		line, merr := json.Marshal(extra)
		if merr != nil {
			t.Fatalf("夹具行序列化失败：%v", merr)
		}
		f, ferr := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0o644)
		if ferr != nil {
			t.Fatalf("模拟外部写入失败：%v", ferr)
		}
		_, _ = f.Write(append(line, '\n'))
		_ = f.Close()
	}
	rc, out, errb = gapRereadRun(t, "--all")
	gapRereadBetweenReads = nil
	if rc != 0 {
		t.Fatalf("有外写态也要 rc=0（**不进退出码**），得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "external_write_detected=true") {
		t.Errorf("有外部写入要报 external_write_detected=true：%q", out)
	}
	s, b := gapRereadSHAsInHuman(out)
	if s == "" || b == "" {
		t.Fatalf("人面要同时印出两个 sha16：%q", out)
	}
	if s == b {
		t.Errorf("有外部写入时两 sha16 **必须不等**：snapshot=%q readback=%q", s, b)
	}
	if gapRereadFileSHA(t, ledger) == shaBefore {
		t.Errorf("机检钩子未被调用（真源未变）—— 态②没成立")
	}
}

// gapRereadSHAsInHuman —— 从人面「外写自校」那一行取 (snapshot_sha16, readback_sha16)。
func gapRereadSHAsInHuman(out string) (string, string) {
	snap, back := "", ""
	for _, ln := range strings.Split(out, "\n") {
		if !strings.Contains(ln, "外写自校") {
			continue
		}
		snap = gapRereadHexIn(ln, "snapshot_sha16=")
		back = gapRereadHexIn(ln, "readback_sha16=")
	}
	return snap, back
}

// gapRereadHexIn —— 取 `key` 后紧跟的 16 位（找不到 / 太短 ⇒ 空串）。
func gapRereadHexIn(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	s := line[i+len(key):]
	if len(s) < 16 {
		return ""
	}
	return s[:16]
}

func gapRereadOpenFixtureRec(id, verify string) map[string]any {
	r := gapRereadFixtureRec(id, "仍缺", "")
	r["verify_cmd"] = verify
	return r
}

// gapRereadParseConserved —— 从人面「未闭守恒」那一行取 (未闭总数, 已跑, 拒跑, 截断)。
func gapRereadParseConserved(t *testing.T, out string) (tot, ran, ref, tr int) {
	t.Helper()
	re := regexp.MustCompile(`未闭总数 (\d+) == 已跑 (\d+) \+ 拒跑 (\d+) \+ 截断 (\d+)`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("人面缺「未闭守恒」行：%q", out)
	}
	for i, p := range []*int{&tot, &ran, &ref, &tr} {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			t.Fatalf("守恒数解析失败：%v", err)
		}
		*p = n
	}
	return
}

// TestGapRereadIncludeOpen —— `--include-open`（未闭条 verify_cmd 判活面 · 缺口 `GAP-20261005-60` 剩面）。
//
//	① 默认档（不给旗标）**不出现**未闭判活面（默认行为一字不变）
//	② `--include-open` 把未闭条 verify_cmd 纳入同一套读数：可跑 ⇒ ok；元字符 / 占位 / 自递归 / 空 / 解析不出 ⇒ not_rereadable（**不执行**）
//	③ 表尾**守恒自校**：未闭总数 == 已跑 + 拒跑 + 截断（可跑条目数少 ⇒ 截断 > 0）
//	④ 只报告：rc 恒 0（not_rereadable / 截断 均不进退出码）
func TestGapRereadIncludeOpen(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	recs := []map[string]any{
		gapRereadFixtureRec("GAP-20261005-01", "已解",
			"销案两态对拍 · 改后{cmd=zerg gap ls --state 不做 · rc=1 · reading=零 不做} · 散文=夹具"),
		gapRereadOpenFixtureRec("GAP-20261005-11", "zerg gap ls --state 不做"), // 可跑 ⇒ ok
		gapRereadOpenFixtureRec("GAP-20261005-12", "zerg gap ls | wc -l"),    // 管道 ⇒ 拒跑
		gapRereadOpenFixtureRec("GAP-20261005-13", "zerg gap ls; echo x"),    // 分号 ⇒ 拒跑
		gapRereadOpenFixtureRec("GAP-20261005-14", "zerg gap ls 占位"),         // 占位串 ⇒ 拒跑
		gapRereadOpenFixtureRec("GAP-20261005-15", "zerg gap reread --all"),  // 自递归 ⇒ 拒跑
		gapRereadOpenFixtureRec("GAP-20261005-16", "zerg nosuchcmd --x"),     // 解析不出 ⇒ 拒跑
		gapRereadOpenFixtureRec("GAP-20261005-17", ""),                       // 空 ⇒ 拒跑
	}
	ledger := gapRereadWriteLedger(t, dir, recs)
	sha0 := gapRereadFileSHA(t, ledger)

	// ① 默认档（不给旗标）：不出现未闭判活面
	rc, out, errb := gapRereadRun(t, "--all")
	if rc != 0 {
		t.Fatalf("默认档要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	for _, bad := range []string{"未闭判活", "未闭守恒", "open_total"} {
		if strings.Contains(out, bad) {
			t.Errorf("默认档**不许**出现未闭判活面（%q）：%q", bad, out)
		}
	}

	// ② 全量 --include-open：7 条未闭条（1 可跑 + 6 拒跑）
	rc, out, errb = gapRereadRun(t, "--all", "--include-open")
	if rc != 0 {
		t.Fatalf("--include-open 要 rc=0（只报告），得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "未闭判活") {
		t.Fatalf("要给未闭判活面：%q", out)
	}
	if !strings.Contains(out, "[ok]") {
		t.Errorf("可跑的未闭条要记 ok：%q", out)
	}
	for _, want := range []string{"元字符", "占位", "自递归", "命令树里没有", "空命令"} {
		if !strings.Contains(out, want) {
			t.Errorf("未闭判活的拒跑理由里要出现 %q：%q", want, out)
		}
	}
	if n := strings.Count(out, "["+gapRereadNoRun+"]"); n < 6 {
		t.Errorf("未闭条里 6 条拒跑要各落 [%s]（实得 %d）：%q", gapRereadNoRun, n, out)
	}

	// ③ 守恒：未闭总数 == 已跑 + 拒跑 + 截断（全量 ⇒ 截断 0）
	tot, ran, ref, tr := gapRereadParseConserved(t, out)
	if tot != ran+ref+tr {
		t.Errorf("守恒式不成立：%d != %d + %d + %d", tot, ran, ref, tr)
	} else if tot != 7 || ran != 1 || ref != 6 || tr != 0 {
		t.Errorf("未闭读数应为 7=1+6+0，实得 %d=%d+%d+%d", tot, ran, ref, tr)
	}

	// ③ 截断：--limit 2 ⇒ 截断 > 0，且守恒仍成立
	rc, out, errb = gapRereadRun(t, "--all", "--include-open", "--limit", "2")
	if rc != 0 {
		t.Fatalf("--include-open --limit 2 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	tot, ran, ref, tr = gapRereadParseConserved(t, out)
	if tot != ran+ref+tr {
		t.Errorf("截断档守恒式不成立：%d != %d + %d + %d", tot, ran, ref, tr)
	} else if tr <= 0 {
		t.Errorf("--limit 2 下截断应 > 0（未闭 7 > 2），实得截断=%d", tr)
	}

	// ④ 只读：本面一字节都不写真源
	if got := gapRereadFileSHA(t, ledger); got != sha0 {
		t.Errorf("本面必须只读（真源 sha 变了）：%s → %s", sha0, got)
	}
	// ⑤ 缺目标仍走既有用法错（不受本旗标影响）
	if rc, _, _ := gapRereadRun(t, "--include-open"); rc != 2 {
		t.Errorf("缺目标 + --include-open ⇒ 2，得到 %d", rc)
	}
}

// TestGapRereadRuntimeUsageLine —— 运行期用法行列 `--include-open`（门73 面⑥ 对拍面 · 缺口 `GAP-20260928-183`）。
// 缺目标 ⇒ 退 2，并在 stderr 打出与本命令命令树 `usage:` 同形的运行期用法行（含 `--include-open` · `--limit` · `--json`）。
func TestGapRereadRuntimeUsageLine(t *testing.T) {
	rc, _, errb := gapRereadRun(t)
	if rc != 2 {
		t.Fatalf("缺目标 ⇒ 2，得到 %d", rc)
	}
	if !strings.Contains(errb, "用法：zerg gap reread") {
		t.Errorf("运行期用法行要以 `用法：zerg gap reread` 起（门73 面⑥ 取数面）：%q", errb)
	}
	for _, f := range []string{"--all", "--include-open", "--limit", "--json"} {
		if !strings.Contains(errb, f) {
			t.Errorf("运行期用法行要列 %q：%q", f, errb)
		}
	}
}

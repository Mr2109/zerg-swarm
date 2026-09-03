// compat_test.go — B7 / G10 验收单测。
//
// 纪律：**全部**用例只写 t.TempDir()（绝不碰真机 ~/.zerg/state）；每个用例注入自己的
// Config{Dirs:…, Logf:capture, Now:fixed}，因此迁移的时间戳/备份名都是确定的。
package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ───────────────────────── 夹具 ─────────────────────────

type logCapture struct{ lines []string }

func (l *logCapture) logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}
func (l *logCapture) joined() string { return strings.Join(l.lines, "\n") }

func fixture(t *testing.T) (Config, *logCapture, string) {
	t.Helper()
	state := t.TempDir()
	receipts := t.TempDir()
	cap := &logCapture{}
	cfg := Config{
		Dirs: Dirs{State: state, Receipts: receipts},
		Logf: cap.logf,
		Now:  func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) },
	}
	return cfg, cap, state
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", path, err)
	}
	return string(b)
}

func globCount(t *testing.T, pattern string) int {
	t.Helper()
	ms, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s 失败: %v", pattern, err)
	}
	return len(ms)
}

func entryT(t *testing.T, name string) Entry {
	t.Helper()
	e, ok := Lookup(name)
	if !ok {
		t.Fatalf("清单里没有条目 %q", name)
	}
	return e
}

// ───────────────────────── ① 清单自检 ─────────────────────────

// 清单必须自洽：current ≥ min ≥ 0、迁移函数已注册、锚点非空、excluded 都有理由。
func TestManifestSelfCheck(t *testing.T) {
	probs := MustManifest().Check()
	if len(probs) > 0 {
		t.Fatalf("清单自检未通过:\n  %s", strings.Join(probs, "\n  "))
	}
}

// 「当前 schema ≥ 清单声明的最低可读版本」这条承重不变量，逐条钉死。
func TestCurrentSchemaCoversMinReadable(t *testing.T) {
	for _, e := range Entries() {
		if e.CurrentSchema < e.MinReadable {
			t.Errorf("%s: current_schema=%d < min_readable=%d", e.Name, e.CurrentSchema, e.MinReadable)
		}
		if e.CurrentSchema < 1 {
			t.Errorf("%s: current_schema=%d 必须 ≥1", e.Name, e.CurrentSchema)
		}
	}
}

// 任务书点名的六个文件必须在清单里（漏一个就等于「升级即丢配置」没被覆盖）。
func TestRequiredFilesRegistered(t *testing.T) {
	want := map[string]bool{
		"internal_engine": false, "update_check": false, "ui_state": false,
		"ui_prefs": false, "ui_layout": false, "update_receipts": false,
	}
	for _, e := range Entries() {
		if _, ok := want[e.Name]; ok {
			want[e.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("清单缺少任务书点名的条目 %q", name)
		}
	}
}

// 未登记的条目名必须被拒绝（防止绕过清单直接拼路径）。
func TestUnregisteredEntryRefused(t *testing.T) {
	cfg, _, _ := fixture(t)
	if _, _, err := Read("no_such_state_file", cfg); err == nil {
		t.Fatal("未登记条目应报错")
	}
}

// ───────────────────────── ② 向后兼容：旧版 ⇒ 一次性迁移 ─────────────────────────

// 旧版（无 schema）⇒ 迁移成功；载荷一个字段不丢；备份存在；二次读幂等（不再动盘、不再备份）。
func TestLegacyMigratesOnceAndIsIdempotent(t *testing.T) {
	cfg, cap, state := fixture(t)
	path := filepath.Join(state, "internal_engine.json")
	legacy := `{"stopped":true,"since":"2026-01-02T03:04:05Z"}`
	mustWrite(t, path, legacy)

	data, out, err := Read("internal_engine", cfg)
	if err != nil {
		t.Fatalf("迁移旧版应成功，实际 err=%v", err)
	}
	if out != OutcomeMigrated {
		t.Fatalf("结论应为 %s，实际 %s", OutcomeMigrated, out)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("迁移后不是合法 JSON: %v", err)
	}
	if got["schema"] != float64(1) {
		t.Errorf("迁移后应带 schema=1，实际 %v", got["schema"])
	}
	if got["stopped"] != true || got["since"] != "2026-01-02T03:04:05Z" {
		t.Errorf("载荷字段必须一个不少，实际 %v", got)
	}

	bak := path + ".bak-20260913T120000"
	if readFileT(t, bak) != legacy {
		t.Errorf("备份必须是迁移前原文；实际 %q", readFileT(t, bak))
	}
	if n := globCount(t, path+".bak-*"); n != 1 {
		t.Errorf("应恰好 1 份备份，实际 %d", n)
	}

	// 二跑：幂等 —— 结论 current、磁盘逐字节不变、不再新增备份
	first := readFileT(t, path)
	data2, out2, err := Read("internal_engine", cfg)
	if err != nil || out2 != OutcomeCurrent {
		t.Fatalf("二跑应幂等（current），实际 out=%s err=%v", out2, err)
	}
	if string(data2) != first {
		t.Errorf("二跑返回的字节应与磁盘一致")
	}
	if readFileT(t, path) != first {
		t.Errorf("二跑不得改动文件")
	}
	if n := globCount(t, path+".bak-*"); n != 1 {
		t.Errorf("二跑不得新增备份，实际 %d 份", n)
	}
	if !strings.Contains(cap.joined(), "状态文件已迁移到 schema 1") {
		t.Errorf("迁移必须留痕（日志），实际:\n%s", cap.joined())
	}
}

// 已是当前 schema ⇒ 绝不覆盖：不改一个字节、不产生备份、不写日志。
func TestCurrentSchemaIsNeverRewritten(t *testing.T) {
	cfg, cap, state := fixture(t)
	path := filepath.Join(state, "internal_engine.json")
	cur := "{\"schema\": 1, \"stopped\": false, \"since\": \"2026-05-05T00:00:00Z\"}\n"
	mustWrite(t, path, cur)

	_, out, err := Read("internal_engine", cfg)
	if err != nil || out != OutcomeCurrent {
		t.Fatalf("应为 current，实际 out=%s err=%v", out, err)
	}
	if readFileT(t, path) != cur {
		t.Errorf("已是当前 schema 的文件不得被改写")
	}
	if n := globCount(t, path+".bak-*"); n != 0 {
		t.Errorf("不得产生备份，实际 %d 份", n)
	}
	if strings.Contains(cap.joined(), "已迁移") {
		t.Errorf("无迁移不该打迁移日志：%s", cap.joined())
	}
}

// 已有的同名备份绝不覆盖（迁移可重入，更早那份备份是最珍贵的）。
func TestExistingBackupIsNeverOverwritten(t *testing.T) {
	cfg, _, state := fixture(t)
	path := filepath.Join(state, "internal_engine.json")
	mustWrite(t, path, `{"stopped":true}`)
	bak := path + ".bak-20260913T120000"
	mustWrite(t, bak, `{"earlier":"backup"}`) // 更早的一份备份

	if _, out, err := Read("internal_engine", cfg); err != nil || out != OutcomeMigrated {
		t.Fatalf("迁移应成功，实际 out=%s err=%v", out, err)
	}
	if readFileT(t, bak) != `{"earlier":"backup"}` {
		t.Errorf("已有备份被覆盖了：%q", readFileT(t, bak))
	}
	if n := globCount(t, path+".bak-*"); n != 1 {
		t.Errorf("备份数应仍为 1，实际 %d", n)
	}
}

// ───────────────────────── ③ 向前兼容：高版本 ⇒ 不猜 ─────────────────────────

// 高版本 schema ⇒ 只读降级：不迁移、不回写、不备份，且明确告警；写路径拒绝。
func TestFutureSchemaIsReadOnlyAndWriteRefused(t *testing.T) {
	cfg, cap, state := fixture(t)
	path := filepath.Join(state, "internal_engine.json")
	future := `{"schema": 99, "stopped": true, "future_only_field": {"x": 1}}`
	mustWrite(t, path, future)

	data, out, err := Read("internal_engine", cfg)
	if err != nil {
		t.Fatalf("高版本应「只读降级」而非报错，实际 err=%v", err)
	}
	if out != OutcomeFuture {
		t.Fatalf("结论应为 %s，实际 %s", OutcomeFuture, out)
	}
	if string(data) != future {
		t.Errorf("高版本文件必须原样返回（不猜），实际 %q", string(data))
	}
	if readFileT(t, path) != future {
		t.Errorf("高版本文件不得被改写")
	}
	if n := globCount(t, path+".bak-*"); n != 0 {
		t.Errorf("高版本不该触发备份，实际 %d 份", n)
	}
	if !strings.Contains(cap.joined(), "高于本机认知") || !strings.Contains(cap.joined(), "升级本机二进制") {
		t.Errorf("必须明确告警并提示升级本机二进制，实际:\n%s", cap.joined())
	}

	// 写路径：拒绝覆盖更高版本的状态
	_, werr := WriteRaw("internal_engine", []byte(`{"stopped":false}`), cfg)
	if !errors.Is(werr, ErrSchemaTooNew) {
		t.Fatalf("写高版本状态应被拒绝（ErrSchemaTooNew），实际 %v", werr)
	}
	if readFileT(t, path) != future {
		t.Errorf("被拒绝的写不得改动文件")
	}
}

// ───────────────────────── ④ sidecar 信封：载荷逐字节不动 ─────────────────────────

// ui_layout 是 HashMap<String,f32>：信封内塞 schema 会被读侧当成一条比例 ⇒ 必须走旁路。
func TestSidecarKeepsPayloadByteIdentical(t *testing.T) {
	cfg, _, state := fixture(t)
	path := filepath.Join(state, "ui", "ui_layout.json")
	legacy := `{"split_docs1":0.17503357}`
	mustWrite(t, path, legacy)

	_, out, err := Read("ui_layout", cfg)
	if err != nil || out != OutcomeMigrated {
		t.Fatalf("旧版应迁移，实际 out=%s err=%v", out, err)
	}
	if readFileT(t, path) != legacy {
		t.Errorf("sidecar 载荷必须逐字节不动，实际 %q", readFileT(t, path))
	}
	sc := readFileT(t, path+".schema.json")
	var doc sidecarDoc
	if err := json.Unmarshal([]byte(sc), &doc); err != nil {
		t.Fatalf("旁路版本文件不是合法 JSON: %v", err)
	}
	if doc.Schema != 1 {
		t.Errorf("旁路版本号应为 1，实际 %d", doc.Schema)
	}

	// 写路径：载荷里绝不能出现 schema 键（那会被当成一条比例为 1.0 的条目）
	raw := `{"split_docs1":0.9,"split_docs2":0.5}`
	if _, err := WriteRaw("ui_layout", []byte(raw), cfg); err != nil {
		t.Fatalf("写 sidecar 条目失败: %v", err)
	}
	body := readFileT(t, path)
	if strings.Contains(body, "schema") {
		t.Errorf("sidecar 载荷不得含 schema 键，实际 %q", body)
	}
	var layout map[string]float32
	if err := json.Unmarshal([]byte(body), &layout); err != nil {
		t.Fatalf("载荷应能被 HashMap<String,f32> 解析（这正是不能塞 schema 的原因）：%v", err)
	}
	if len(layout) != 2 {
		t.Errorf("载荷键数应为 2，实际 %d（%v）", len(layout), layout)
	}
	if _, out, err := Read("ui_layout", cfg); err != nil || out != OutcomeCurrent {
		t.Fatalf("写完应读到 current，实际 out=%s err=%v", out, err)
	}
}

// ui_modules 是 map<string,bool>：信封内塞整数会让**整份**解析失败 ⇒ 旁路是必须的，不是洁癖。
func TestSidecarIsRequiredForBoolMapShape(t *testing.T) {
	cfg, _, state := fixture(t)
	path := filepath.Join(state, "ui", "modules.json")
	mustWrite(t, path, `{"git":false,"upgrade":false}`)

	if _, out, err := Read("ui_modules", cfg); err != nil || out != OutcomeMigrated {
		t.Fatalf("旧版应迁移，实际 out=%s err=%v", out, err)
	}
	body := readFileT(t, path)
	var disabled map[string]bool
	if err := json.Unmarshal([]byte(body), &disabled); err != nil {
		t.Fatalf("迁移后必须仍能被 map<string,bool> 解析（否则用户「卸下」状态全丢）：%v", err)
	}
	if len(disabled) != 2 || disabled["git"] || disabled["upgrade"] {
		t.Errorf("载荷应原样保留，实际 %v", disabled)
	}
	// 反证：把 schema 塞进信封会让整份解析失败 —— 记录这条事实，防止后人「统一成 inband」。
	broken := `{"schema":1,"git":false}`
	if err := json.Unmarshal([]byte(broken), &disabled); err == nil {
		t.Fatal("反证失败：map<string,bool> 竟然接受了整数 schema —— 该结论变了，请复核清单")
	}
}

// ───────────────────────── ⑤ 不静默：坏文件留痕且原文不动 ─────────────────────────

func TestCorruptFileKeepsOriginalAndTracesLog(t *testing.T) {
	cfg, cap, state := fixture(t)
	path := filepath.Join(state, "internal_engine.json")
	broken := `{"stopped": tru`
	mustWrite(t, path, broken)

	data, out, err := Read("internal_engine", cfg)
	if out != OutcomeCorrupt || !errors.Is(err, ErrCorrupt) {
		t.Fatalf("坏文件应为 corrupt+ErrCorrupt，实际 out=%s err=%v", out, err)
	}
	if string(data) != broken {
		t.Errorf("坏文件必须原样返回，实际 %q", string(data))
	}
	if readFileT(t, path) != broken {
		t.Errorf("坏文件不得被改写")
	}
	if !strings.Contains(cap.joined(), "不可解析") {
		t.Errorf("坏文件必须留痕，实际:\n%s", cap.joined())
	}
}

// 回读校验必须能抓住「迁移把顶层键弄丢了」这类错误（承重守卫的变异验证）。
func TestVerifyGuardCatchesLostTopLevelKey(t *testing.T) {
	e := entryT(t, "internal_engine")
	dir := t.TempDir()
	path := filepath.Join(dir, "internal_engine.json")
	mustWrite(t, path, `{"schema":1,"a":1}`) // 迁移后：只剩 a
	before := []byte(`{"a":1,"b":2,"c":3}`)  // 迁移前：a/b/c 三个键

	err := verifyInband(e, path, before)
	if err == nil {
		t.Fatal("顶层键丢失时必须报 ErrVerifyFailed")
	}
	if !errors.Is(err, ErrVerifyFailed) {
		t.Fatalf("错误应包装 ErrVerifyFailed，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "\"b\"") || !strings.Contains(err.Error(), "\"c\"") {
		t.Errorf("错误信息应点出丢失的键，实际 %v", err)
	}
}

// ───────────────────────── ⑥ 写路径：版本字段置首且可回读 ─────────────────────────

func TestWriteStampsSchemaAndRoundTrips(t *testing.T) {
	cfg, _, state := fixture(t)
	path, err := Write("internal_engine", map[string]any{"stopped": true, "since": "2026-07-07T00:00:00Z"}, cfg)
	if err != nil {
		t.Fatalf("写失败: %v", err)
	}
	if path != filepath.Join(state, "internal_engine.json") {
		t.Errorf("写路径不符: %s", path)
	}
	body := readFileT(t, path)
	// 版本字段必须置首（人读 + diff 稳定）：它出现的位置早于任何载荷键
	si := strings.Index(body, `"schema": 1`)
	since := strings.Index(body, `"since"`)
	stopped := strings.Index(body, `"stopped"`)
	if si < 0 || si > since || si > stopped {
		t.Errorf("版本字段应置首且格式确定（schema@%d since@%d stopped@%d），实际:\n%s", si, since, stopped, body)
	}
	if strings.Count(body, `"schema"`) != 1 {
		t.Errorf("schema 键应恰好出现一次，实际:\n%s", body)
	}
	data, out, err := Read("internal_engine", cfg)
	if err != nil || out != OutcomeCurrent {
		t.Fatalf("写完应 current，实际 out=%s err=%v", out, err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if got["stopped"] != true || got["since"] != "2026-07-07T00:00:00Z" {
		t.Errorf("载荷往返必须无损，实际 %v", got)
	}
	// 确定性：两次写同一载荷 ⇒ 逐字节相同（幂等/内容寻址依赖它）
	_, _ = Write("internal_engine", map[string]any{"stopped": true, "since": "2026-07-07T00:00:00Z"}, cfg)
	if readFileT(t, path) != body {
		t.Errorf("同一载荷两次写必须逐字节相同")
	}
}

// ───────────────────────── ⑦ glob 集合（升级回执）逐文件迁移 ─────────────────────────

func TestGlobCollectionMigratesEachFile(t *testing.T) {
	cfg, _, _ := fixture(t)
	receipts := cfg.Dirs.Receipts
	legacy := `{"result":"ok","from":{"core":"2.5.8"}}`
	cur := "{\"schema\": 1, \"result\": \"ok\"}\n"
	mustWrite(t, filepath.Join(receipts, "20260101T000000Z.json"), legacy)
	mustWrite(t, filepath.Join(receipts, "latest.json"), cur)

	reports := MigrateAll(cfg)
	byPath := map[string]Report{}
	for _, r := range reports {
		byPath[r.Path] = r
	}
	old := filepath.Join(receipts, "20260101T000000Z.json")
	latest := filepath.Join(receipts, "latest.json")
	if r, ok := byPath[old]; !ok || r.Outcome != OutcomeMigrated {
		t.Fatalf("旧回执应迁移，实际 %+v", byPath[old])
	}
	if r, ok := byPath[latest]; !ok || r.Outcome != OutcomeCurrent {
		t.Fatalf("已是当前 schema 的回执应保持 current，实际 %+v", byPath[latest])
	}
	if readFileT(t, old) == legacy {
		t.Errorf("旧回执应被迁移（带 schema）")
	}
	if readFileT(t, latest) != cur {
		t.Errorf("current 回执不得被改写")
	}
	if n := globCount(t, old+".bak-*"); n != 1 {
		t.Errorf("旧回执应有一份备份，实际 %d", n)
	}
	// 幂等：再跑一遍，结论全部 current/missing，且旧回执字节不再变
	first := readFileT(t, old)
	for _, r := range MigrateAll(cfg) {
		if r.Outcome == OutcomeCorrupt {
			t.Errorf("二跑不应出现 corrupt：%+v", r)
		}
	}
	if readFileT(t, old) != first {
		t.Errorf("二跑不得改动已迁移的回执")
	}
}

// ───────────────────────── ⑧ 目录注入纪律 ─────────────────────────

// 空目录必须报错——空串经 filepath.Join 会落到当前工作目录（测试/工具误写仓库的经典形态）。
func TestEmptyDirsRefused(t *testing.T) {
	if _, _, err := Read("internal_engine", Config{Logf: func(string, ...any) {}}); err == nil {
		t.Fatal("未注入状态目录时必须报错")
	}
	if _, err := WriteRaw("internal_engine", []byte(`{"a":1}`), Config{Logf: func(string, ...any) {}}); err == nil {
		t.Fatal("未注入状态目录时写也必须报错")
	}
}

// 生产解析器必须认环境变量（测试与机群靠它换目录，而不是靠改代码）。
func TestDefaultDirsHonoursEnv(t *testing.T) {
	s := t.TempDir()
	r := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", s)
	t.Setenv("ZERG_RECEIPTS_DIR", r)
	d := DefaultDirs()
	if d.State != s {
		t.Errorf("state 目录应随 ZERG_STATE_DIR，实际 %s", d.State)
	}
	if d.Receipts != r {
		t.Errorf("receipts 目录应随 ZERG_RECEIPTS_DIR，实际 %s", d.Receipts)
	}
}

// 路径解析：ui/* 落在 state 下的子目录；glob 条目按通配展开且不吞掉备份文件。
func TestPathResolution(t *testing.T) {
	d := Dirs{State: "/s", Receipts: "/r"}
	if p := entryT(t, "ui_state").Path(d); p != filepath.Join("/s", "ui", "ui_state.json") {
		t.Errorf("ui_state 路径不符: %s", p)
	}
	if p := entryT(t, "update_receipts").Path(d); p != filepath.Join("/r", "*.json") {
		t.Errorf("回执 glob 路径不符: %s", p)
	}
	if p := entryT(t, "internal_engine").SidecarPath("/s/internal_engine.json"); p != "/s/internal_engine.json.schema.json" {
		t.Errorf("旁路版本文件路径不符: %s", p)
	}
}

// 清单里每条 inband 条目的 schema 字段名必须和「读写实现真正用的键名」一致——
// prefix_cache 用的是 version 而不是 schema，这条防止清单写成想当然的 schema。
func TestSchemaFieldMatchesNativeKey(t *testing.T) {
	e := entryT(t, "prefix_cache")
	if e.SchemaKey() != "version" {
		t.Errorf("prefix_cache 的原生版本键是 version，清单写成 %q 会让读侧对不上", e.SchemaKey())
	}
}

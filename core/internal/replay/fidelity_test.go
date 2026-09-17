// fidelity_test.go —— T4.4（保真度三指标 + 二分定位）与 T4.6（trace 版本与失效）的实证：
// 逐条钉死，**每条自带反例**（只断"该报错的报错了"会漏掉"把所有调用都判成失败"这种假绿）。
//
// 用例清单（编号 ↔ 设计稿 E7/E11 的处置）：
//
//	① TestFidelityCountAndVerdict          F 计算正确 / 零尝试 ⇒ FAIL / 计数矛盾 ⇒ FAIL
//	② TestFidelityMissedIsFailureNotWarning missed>0 ⇒ FAIL（不是警告）；missed=0 ⇒ PASS（对侧）
//	③ TestFidelityArtifactEquivalence      产物相等才 PASS；差一字节 ⇒ 指名道姓；0==0 ⇒ FAIL
//	④ TestFidelityDeterminismReplayOfReplay replay(replay(x)) == replay(x)；非幂等 ⇒ 露馅
//	⑤ TestFidelityBisectEarliestDivergence 二分定位最早分歧 seq + 最小可复现前缀；前提破裂 ⇒ 报错
//	⑥ TestTraceInvalidationOnFingerprintChange 指纹变 ⇒ 作废（专项错误码 + 两边指纹）
//	⑦ TestTraceFingerprintUnchangedReplays 指纹相同 ⇒ 正常回放（反例：防误报）
//	⑧ TestTraceHeaderlessIsInvalidated     缺头部 ⇒ 作废；显式放行是唯一出口
//	⑨ TestToolTableFingerprintStable       工具表指纹与 map 序无关、内容变则指纹变
//	⑩ TestDeterministicID                  确定性 ID = sha256(traceID‖seq)（E9），非随机
//	⑪ TestFidelityFixtureForGate           **固定夹具**：脚本 scripts/replay-fidelity.sh 的输入源
package replay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ── 夹具小工具 ──────────────────────────────────────────────────────────────

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录 %s：%v", dir, err)
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录 %s：%v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("写 %s：%v", p, err)
		}
	}
}

func mustDigestDir(t *testing.T, dir string) *ArtifactSet {
	t.Helper()
	set, err := DigestDir(dir, ArtifactOptions{})
	if err != nil {
		t.Fatalf("DigestDir(%s)：%v", dir, err)
	}
	return set
}

// fixtureRoot —— 门禁输出根：有 `ZERG_REPLAY_FIDELITY_DIR` 就用它（脚本读这里的报告），
// 否则用 t.TempDir()。**两种情况下用例都真跑** —— "没设环境变量就跳过"是另一种假绿。
func fixtureRoot(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("ZERG_REPLAY_FIDELITY_DIR"); d != "" {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建门禁输出根 %s：%v", d, err)
		}
		return d
	}
	return t.TempDir()
}

func writeReport(t *testing.T, root, name string, v any) {
	t.Helper()
	b, err := jsonOf(v)
	if err != nil {
		t.Fatalf("序列化 %s：%v", name, err)
	}
	if err := os.WriteFile(filepath.Join(root, name), b, 0o644); err != nil {
		t.Fatalf("写 %s：%v", name, err)
	}
}

// ── ① F 计算正确（含零尝试与计数矛盾）────────────────────────────────────────

func TestFidelityCountAndVerdict(t *testing.T) {
	c := NewFidelityCounter()
	// 3 命中 / 4 尝试 ⇒ F=0.75，missed=1
	c.Record("read_file", "sha256:aa", nil)
	c.Record("write_file", "sha256:bb", nil)
	c.Record("bash", "sha256:cc", nil)
	c.Record("read_file", "sha256:dd", errors.New("未命中录播"))
	rep := c.Report()
	if rep.Attempted != 4 || rep.Hit != 3 || rep.Missed != 1 {
		t.Fatalf("计数应为 4/3/1，实得 %d/%d/%d", rep.Attempted, rep.Hit, rep.Missed)
	}
	if rep.F != 0.75 {
		t.Errorf("F 应为 0.75，实得 %v", rep.F)
	}
	if len(rep.Misses) != 1 || rep.Misses[0].Index != 4 || rep.Misses[0].Tool != "read_file" {
		t.Errorf("未命中明细应含第 4 次尝试的 read_file，实得 %+v", rep.Misses)
	}
	if rep.Verdict() == nil {
		t.Errorf("missed>0 时 Verdict 必须非 nil")
	}

	// 零尝试：F **无定义**（不许写成 1.0）
	empty := NewFidelityCounter().Report()
	if !empty.FUndefined || empty.F != 0 {
		t.Errorf("零尝试应 FUndefined=true 且 F=0，实得 undefined=%v f=%v", empty.FUndefined, empty.F)
	}
	if empty.Verdict() == nil {
		t.Errorf("零尝试必须 FAIL（一次什么都没尝试的\"回放\"等于没有证据）")
	}
	want := "零次尝试"
	if !strings.Contains(empty.Verdict().Error(), want) {
		t.Errorf("零尝试的判词应指出「%s」：%v", want, empty.Verdict())
	}

	// 对侧：全命中 ⇒ PASS
	ok := NewFidelityCounter()
	ok.Record("read_file", "sha256:aa", nil)
	ok.Record("read_file", "sha256:aa", nil)
	if err := ok.Report().Verdict(); err != nil {
		t.Errorf("全命中必须 PASS，实得 %v", err)
	}

	// 计数矛盾（报告被拼过/计数器被改过）⇒ FAIL
	bad := FidelityReport{Attempted: 4, Hit: 3, Missed: 0}
	if bad.Verdict() == nil {
		t.Errorf("hit+missed != attempted 必须 FAIL")
	}

	// 并发安全：并发记账后计数不得丢
	cc := NewFidelityCounter()
	var wg []chan struct{}
	for i := 0; i < 8; i++ {
		ch := make(chan struct{})
		wg = append(wg, ch)
		go func(ch chan struct{}) {
			defer close(ch)
			for j := 0; j < 50; j++ {
				cc.Record("t", "fp", nil)
			}
		}(ch)
	}
	for _, ch := range wg {
		<-ch
	}
	if got := cc.Attempted(); got != 400 {
		t.Errorf("并发记账应得 400，实得 %d", got)
	}
}

// ── ② missed>0 即 FAIL（不软化）+ RecordCall 的来源铁律 ─────────────────────

func TestFidelityMissedIsFailureNotWarning(t *testing.T) {
	c := NewFidelityCounter()
	c.Record("read_file", "sha256:aa", nil)
	c.Record("read_file", "sha256:zz", &MissError{Tool: "read_file", Fingerprint: "sha256:zz", Scanned: 1})
	rep := c.Report()
	err := rep.Verdict()
	if err == nil {
		t.Fatalf("missed=1 必须 FAIL")
	}
	if !errors.Is(err, ErrFidelityFailed) {
		t.Errorf("判词必须能被 errors.Is(err, ErrFidelityFailed) 认出，实得 %v", err)
	}
	var fe *FidelityError
	if !errors.As(err, &fe) {
		t.Fatalf("判词应是 *FidelityError，实得 %T", err)
	}
	if !strings.Contains(err.Error(), "missed=1") {
		t.Errorf("判词应点名 missed=1：%s", err.Error())
	}

	// RecordCall 的来源铁律：回放模式下 nil 错误但来源不是 recorded ⇒ 按未命中计（接线错了最不该放过）
	c2 := NewFidelityCounter()
	c2.RecordCall("read_file", "sha256:aa", Result{Source: SourceRecorded}, nil)
	c2.RecordCall("read_file", "sha256:aa", Result{Source: SourceLive}, nil)
	if c2.Missed() != 1 || c2.Hit() != 1 {
		t.Errorf("来源非 recorded 的 nil 错误应记未命中：命中 %d 未命中 %d", c2.Hit(), c2.Missed())
	}
	c3 := NewFidelityCounter()
	c3.RecordCall("read_file", "sha256:aa", Result{Source: SourceRecorded}, nil)
	if c3.Missed() != 0 {
		t.Errorf("recorded 来源不应记未命中：未命中 %d", c3.Missed())
	}
}

// ── ③ 产物等价性 ────────────────────────────────────────────────────────────

func TestFidelityArtifactEquivalence(t *testing.T) {
	root := t.TempDir()
	rec, rep := filepath.Join(root, "recorded"), filepath.Join(root, "replayed")
	writeFiles(t, rec, map[string]string{"a.txt": "A", "sub/b.txt": "B"})
	writeFiles(t, rep, map[string]string{"a.txt": "A", "sub/b.txt": "B"})

	a, b := mustDigestDir(t, rec), mustDigestDir(t, rep)
	eq := CompareArtifacts(a, b)
	if !eq.Equal || len(eq.Diffs) != 0 {
		t.Fatalf("同内容产物应判相等：%s（差异 %+v）", eq, eq.Diffs)
	}
	if err := eq.Verdict(); err != nil {
		t.Errorf("相等时应 PASS，实得 %v", err)
	}
	// 摘要与遍历顺序无关：同一份产物换一个绝对路径根目录，摘要必须一致（相对路径口径）
	rec2 := filepath.Join(root, "another-root", "recorded")
	writeFiles(t, rec2, map[string]string{"a.txt": "A", "sub/b.txt": "B"})
	if got := mustDigestDir(t, rec2).Digest; got != a.Digest {
		t.Errorf("摘要只认相对路径：换根目录后应同摘要，%s vs %s", got, a.Digest)
	}

	// 差一个字节 ⇒ 不等，且**指名道姓**（同长度改动 ⇒ 判"内容不同"；长度变了会判"大小不同"）
	if err := os.WriteFile(filepath.Join(rep, "sub", "b.txt"), []byte("C"), 0o644); err != nil {
		t.Fatalf("改字节：%v", err)
	}
	b2 := mustDigestDir(t, rep)
	eq2 := CompareArtifacts(a, b2)
	if eq2.Equal {
		t.Fatalf("改过一字节必须判不等")
	}
	if d, ok := eq2.Earliest(); !ok || d.Rel != "sub/b.txt" {
		t.Errorf("差异应点名 sub/b.txt，实得 %+v", eq2.Diffs)
	}
	if err := eq2.Verdict(); err == nil || !errors.Is(err, ErrFidelityFailed) {
		t.Errorf("不等必须 FAIL：%v", err)
	}

	// 多一个文件 / 少一个文件都要被抓
	if err := os.WriteFile(filepath.Join(rep, "extra.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	eq3 := CompareArtifacts(a, mustDigestDir(t, rep))
	kinds := map[string]bool{}
	for _, d := range eq3.Diffs {
		kinds[d.Kind] = true
	}
	if !kinds["内容不同"] || !kinds["仅本次侧有"] {
		t.Errorf("应同时报出「内容不同」与「仅本次侧有」，实得 %+v", eq3.Diffs)
	}

	// 0 == 0 不是等价性证据
	emptyA, emptyB := t.TempDir(), t.TempDir()
	eq4 := CompareArtifacts(mustDigestDir(t, emptyA), mustDigestDir(t, emptyB))
	if err := eq4.Verdict(); err == nil {
		t.Errorf("两侧都空必须 FAIL（0 == 0 是最危险的假绿）")
	}

	// 目录缺失 ⇒ 报错（不许把"两边都缺"当成"两边相等"）
	if _, err := DigestDir(filepath.Join(root, "nope"), ArtifactOptions{}); err == nil || !errors.Is(err, ErrArtifacts) {
		t.Errorf("缺目录必须报 ErrArtifacts，实得 %v", err)
	}
	// 符号链接 ⇒ 报错（摘要不能指向目录外）
	if err := os.Symlink(filepath.Join(rec, "a.txt"), filepath.Join(rec, "link.txt")); err != nil {
		t.Fatalf("建符号链接：%v", err)
	}
	if _, err := DigestDir(rec, ArtifactOptions{}); err == nil || !errors.Is(err, ErrArtifacts) {
		t.Errorf("符号链接必须报 ErrArtifacts，实得 %v", err)
	}
	// 显式跳过 ⇒ 如实报告（不静默）
	skipped, err := DigestDir(rec, ArtifactOptions{Skip: []string{"link.txt"}})
	if err != nil {
		t.Fatalf("显式跳过后应能算：%v", err)
	}
	if len(skipped.Skipped) != 1 || skipped.Skipped[0] != "link.txt" {
		t.Errorf("被跳过的文件应如实列出，实得 %+v", skipped.Skipped)
	}
}

// ── ④ replay(replay(x)) == replay(x) ────────────────────────────────────────

func TestFidelityDeterminismReplayOfReplay(t *testing.T) {
	root := t.TempDir()
	setup := func(fix string) error {
		return os.WriteFile(filepath.Join(fix, "in.txt"), []byte("in\n"), 0o644)
	}
	// 正常回放：读夹具、写产物（幂等 ⇒ 三趟同摘要）
	okRun := func(fix, out string) error {
		b, err := os.ReadFile(filepath.Join(fix, "in.txt"))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, "answer.txt"), b, 0o644)
	}
	rep, err := CheckDeterminism(setup, okRun, filepath.Join(root, "ok"), ArtifactOptions{})
	if err != nil {
		t.Fatalf("CheckDeterminism：%v", err)
	}
	if !rep.Stable || !rep.LayerStable || !rep.FixtureUntouched {
		t.Errorf("幂等回放应三趟同摘要且夹具只读：%s", rep)
	}
	if err := rep.Verdict(); err != nil {
		t.Errorf("应 PASS，实得 %v", err)
	}

	// 反例：追加写 —— 独立两趟一样（Stable 真），"回放的回放"那趟露馅（LayerStable 假）
	appendRun := func(fix, out string) error {
		f, err := os.OpenFile(filepath.Join(out, "answer.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("step\n")
		return err
	}
	bad, err := CheckDeterminism(setup, appendRun, filepath.Join(root, "bad"), ArtifactOptions{})
	if err != nil {
		t.Fatalf("CheckDeterminism（反例）：%v", err)
	}
	if !bad.Stable {
		t.Errorf("两次独立起点都只追加一次 ⇒ 摘要相同（这一维本就抓不到非幂等）：%s", bad)
	}
	if bad.LayerStable {
		t.Errorf("叠加那趟必须不同 ⇒ LayerStable 应为 false：%s", bad)
	}
	if err := bad.Verdict(); err == nil || !errors.Is(err, ErrFidelityFailed) {
		t.Errorf("非幂等必须 FAIL，实得 %v", err)
	}

	// 反例：回放往**夹具**里写 ⇒ 夹具只读这条守卫要红
	dirtyRun := func(fix, out string) error {
		if err := os.WriteFile(filepath.Join(fix, "side.log"), []byte("x"), 0o644); err != nil {
			return err
		}
		return okRun(fix, out)
	}
	dirty, err := CheckDeterminism(setup, dirtyRun, filepath.Join(root, "dirty"), ArtifactOptions{})
	if err != nil {
		t.Fatalf("CheckDeterminism（脏写）：%v", err)
	}
	if dirty.FixtureUntouched {
		t.Errorf("回放写了夹具目录 ⇒ FixtureUntouched 应为 false")
	}
	if err := dirty.Verdict(); err == nil {
		t.Errorf("写夹具必须 FAIL")
	}

	// 空跑（setup/run 为 nil）⇒ 报错，不给结论
	if _, err := CheckDeterminism(nil, nil, root, ArtifactOptions{}); err == nil || !errors.Is(err, ErrDeterminism) {
		t.Errorf("nil setup/run 必须报 ErrDeterminism，实得 %v", err)
	}
}

// ── ⑤ 二分定位最早分歧 + 最小可复现前缀 ─────────────────────────────────────

func TestFidelityBisectEarliestDivergence(t *testing.T) {
	root := t.TempDir()
	tracePath := filepath.Join(root, "trace.jsonl")

	// 造两条 trace：a（变异体）与 b（基准），只在第 41 条的结果上不同。
	const n = 64
	base := make([]Record, 0, n)
	mut := make([]Record, 0, n)
	for i := 1; i <= n; i++ {
		body := []byte(fmt.Sprintf("第 %d 条的结果\n", i))
		canon := []byte(fmt.Sprintf(`{"i":%d}`, i))
		base = append(base, Record{Tool: "step", ArgsCanonical: canon, ResultBody: body})
		m := Record{Tool: "step", ArgsCanonical: canon, ResultBody: body}
		if i == 41 {
			m.ResultBody = []byte("第 41 条的结果（被改过）\n")
		}
		mut = append(mut, m)
	}

	probes := 0
	run := func(recs []Record) (*ArtifactSet, error) {
		probes++
		tr, err := Create(tracePath)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			if _, err := tr.Append(r); err != nil {
				tr.Close()
				return nil, err
			}
		}
		if err := tr.Close(); err != nil {
			return nil, err
		}
		disp, _, err := NewReplayer(tracePath, Normalizer{BaseDir: root}, 0)
		if err != nil {
			return nil, err
		}
		out := filepath.Join(root, fmt.Sprintf("out-%03d", probes))
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		var sb strings.Builder
		for i, r := range recs {
			args := map[string]any{"i": i + 1} // 与录制时的参数逐字一致（不能用 r.Seq：手写的 Record 没有 Seq）
			res, err := disp.Execute(context.Background(), r.Tool, args)
			if err != nil {
				return nil, err
			}
			sb.Write(res.Body)
		}
		if err := os.WriteFile(filepath.Join(out, "answer.txt"), []byte(sb.String()), 0o644); err != nil {
			return nil, err
		}
		return DigestDir(out, ArtifactOptions{})
	}

	// 静态逐条比对：也应给出 41（并且不需要跑任何回放）
	st := DiffRecords(mut, base)
	if st.EarliestSeq != 41 || st.PrefixLen != 41 {
		t.Errorf("静态比对应给出最早分歧 seq=41 / 前缀 41，实得 %+v", st)
	}
	if st.Source != "静态逐条比对" {
		t.Errorf("静态比对的来源标记应正确，实得 %q", st.Source)
	}

	// 行为二分
	d, err := BisectEarliestDivergence(mut, base, run)
	if err != nil {
		t.Fatalf("BisectEarliestDivergence：%v", err)
	}
	if d.EarliestSeq != 41 || d.PrefixLen != 41 {
		t.Errorf("二分应给出最早分歧 seq=41（前缀 41 条），实得 %+v", d)
	}
	if len(d.Prefix) != 41 || string(d.Prefix[40].ResultBody) != string(mut[40].ResultBody) {
		t.Errorf("最小可复现前缀应是 a[:41] 本体，实得 %d 条", len(d.Prefix))
	}
	if len(d.Hybrid) != n || len(d.Hybrid[0].ResultBody) != len(base[0].ResultBody) {
		t.Errorf("混合 trace 长度应为 %d，实得 %d", n, len(d.Hybrid))
	}
	if d.Probes >= 20 {
		t.Errorf("二分应远快于线性（线性需 64 次），实得 %d 次探测", d.Probes)
	}
	if d.Probes <= 3 {
		t.Errorf("探测次数应>3（p=0 复验 + 全量 + 二分），实得 %d —— 太少说明没真跑", d.Probes)
	}
	if len(d.Artifacts) == 0 {
		t.Errorf("分歧点应附产物侧差异（否则只说了\"哪条\"，没说\"变了什么\"）")
	}

	// 反例：两条 trace 完全一样 ⇒ 无分歧（EarliestSeq = 0，不是硬报一个数字）
	same, err := BisectEarliestDivergence(base, base, run)
	if err != nil {
		t.Fatalf("相同 trace 应能给出结论：%v", err)
	}
	if same.EarliestSeq != 0 {
		t.Errorf("相同 trace 应报无分歧，实得 %+v", same)
	}
	if st0 := DiffRecords(base, base); st0.EarliestSeq != 0 {
		t.Errorf("静态比对相同 trace 应无分歧，实得 %+v", st0)
	}

	// 反例：长度不同 ⇒ 静态比对指出"最短侧的下一条"
	short := append([]Record(nil), base[:10]...)
	if got := DiffRecords(base, short); got.EarliestSeq != 11 {
		t.Errorf("条数不同应指出第 11 条，实得 %+v", got)
	}

	// 前提破裂之一：run 自己不确定（同一份输入两次不同产物）⇒ 报错，不猜
	nondet := 0
	flaky := func(recs []Record) (*ArtifactSet, error) {
		if nondet++; nondet%2 == 0 {
			recs = append([]Record(nil), recs...)
		}
		return &ArtifactSet{Digest: Digest([]byte(fmt.Sprintf("%d", nondet)))}, nil
	}
	if _, err := BisectEarliestDivergence(mut, base, flaky); err == nil || !errors.Is(err, ErrBisectNonMonotone) {
		t.Errorf("回放不确定时必须报 ErrBisectNonMonotone（不猜），实得 %v", err)
	}
	// 前提破裂之二：会**说谎**的探针（第 3 次探测报"分歧"、其余都说"一致"）⇒ 复验必须抓到并报错，
	// 而不是把 64 当结论（真实场景：探测被别人并发写的状态影响 ⇒ 探测结果不可复现）
	lying := 0
	if _, err := BisectEarliestDivergence(mut, base, func(recs []Record) (*ArtifactSet, error) {
		lying++
		if lying == 3 {
			return &ArtifactSet{Digest: Digest([]byte("说谎"))}, nil
		}
		return &ArtifactSet{Digest: Digest([]byte("一致"))}, nil
	}); err == nil || !errors.Is(err, ErrBisectNonMonotone) {
		t.Errorf("说谎的探针必须被复验抓到并报 ErrBisectNonMonotone（不猜数字），实得 %v", err)
	}
	// 前提破裂之三：run 在**同一输入**上都给不出同一答案（回放不确定）⇒ 报错
	flaky2 := 0
	if _, err := BisectEarliestDivergence(mut, base, func(recs []Record) (*ArtifactSet, error) {
		flaky2++
		return &ArtifactSet{Digest: Digest([]byte(fmt.Sprintf("v%d", flaky2%3)))}, nil
	}); err == nil || !errors.Is(err, ErrBisectNonMonotone) {
		t.Errorf("回放不确定（同输入两套产物）必须报错，实得 %v", err)
	}
	// run 为 nil ⇒ 报错
	if _, err := BisectEarliestDivergence(mut, base, nil); err == nil || !errors.Is(err, ErrBisectNonMonotone) {
		t.Errorf("run 为 nil 必须报错，实得 %v", err)
	}
}

// ── ⑥ 指纹变 ⇒ 作废（T4.6）──────────────────────────────────────────────────

func TestTraceInvalidationOnFingerprintChange(t *testing.T) {
	root := t.TempDir()
	tracePath := filepath.Join(root, "t.jsonl")
	fp := Fingerprints{
		SchemaVersion:     SchemaVersion,
		ToolFingerprint:   ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`}),
		EngineFingerprint: "b10470-34af94cd9", // 实测形态（G2）
		EngineDigest:      Digest([]byte("engine-bin-v1")),
		ModelDigest:       Digest([]byte("model-weights-v1")),
	}
	// 录一条真 trace + 头部
	rec, err := NewRecorderWithHeader(tracePath, Normalizer{BaseDir: root}, func(_ context.Context, _ Call, _ map[string]any) Result {
		return Result{Body: []byte("ok")}
	}, fp)
	if err != nil {
		t.Fatalf("NewRecorderWithHeader：%v", err)
	}
	if _, err := rec.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt"}); err != nil {
		t.Fatalf("录制：%v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}
	if _, err := os.Stat(HeaderPath(tracePath)); err != nil {
		t.Fatalf("录制应落头部 sidecar：%v", err)
	}

	// 指纹相同 ⇒ 不作废（对侧，防误报）
	if _, err := CheckTrace(tracePath, fp, InvalidationOptions{}); err != nil {
		t.Fatalf("指纹相同不应作废：%v", err)
	}

	cases := []struct {
		name string
		mut  func(f Fingerprints) Fingerprints
		key  string
	}{
		{"工具表指纹变", func(f Fingerprints) Fingerprints {
			f.ToolFingerprint = ToolTableFingerprint(map[string]string{"read_file": `{"path":"string","n":1}`})
			return f
		}, "tool_fingerprint"},
		{"引擎指纹变", func(f Fingerprints) Fingerprints { f.EngineFingerprint = "b10471-00000000"; return f }, "engine_fingerprint"},
		{"引擎二进制变", func(f Fingerprints) Fingerprints { f.EngineDigest = Digest([]byte("engine-bin-v2")); return f }, "engine_digest"},
		{"模型权重变", func(f Fingerprints) Fingerprints { f.ModelDigest = Digest([]byte("model-weights-v2")); return f }, "model_digest"},
		{"行格式版本变", func(f Fingerprints) Fingerprints { f.SchemaVersion = SchemaVersion + 1; return f }, "schema_version"},
		{"版本没声明", func(f Fingerprints) Fingerprints { f.SchemaVersion = 0; return f }, "schema_version"},
		{"引擎指纹没声明", func(f Fingerprints) Fingerprints { f.EngineFingerprint = ""; return f }, "engine_fingerprint"},
	}
	for _, tc := range cases {
		cur := tc.mut(fp)
		err := cur.Check(fp) // 方向反过来也无所谓：两侧都留着
		if err == nil {
			t.Errorf("%s：必须作废", tc.name)
			continue
		}
		if !errors.Is(err, ErrTraceInvalidated) {
			t.Errorf("%s：作废必须是专项错误码（errors.Is(err, ErrTraceInvalidated)），实得 %v", tc.name, err)
		}
		var ie *InvalidationError
		if !errors.As(err, &ie) {
			t.Fatalf("%s：应是 *InvalidationError，实得 %T", tc.name, err)
		}
		msg := err.Error()
		for _, want := range []string{tc.key, fp.String(), cur.String()} {
			if !strings.Contains(msg, want) {
				t.Errorf("%s：判词必须同时给出两边指纹（缺 %q）：%s", tc.name, want, msg)
			}
		}
	}

	// 回放侧：指纹变 ⇒ **不给回放器**（作废 = 没有回放器可用，而不是打印一条警告继续跑）
	bad := fp
	bad.EngineFingerprint = "b10471-00000000"
	disp, _, err := NewReplayerChecked(tracePath, Normalizer{BaseDir: root}, 0, bad, InvalidationOptions{})
	if err == nil {
		t.Fatalf("指纹变时必须拒绝建回放器")
	}
	if disp != nil {
		t.Errorf("作废时必须不返回 Dispatcher，实得 %+v", disp)
	}
	if !errors.Is(err, ErrTraceInvalidated) {
		t.Errorf("专项错误码应是 ErrTraceInvalidated：%v", err)
	}

	// 头部与 trace 配错（复制粘贴）⇒ 也要拦
	wrong := fp
	if err := WriteHeader(filepath.Join(root, "other.jsonl"), Header{Fingerprints: wrong}); err != nil {
		t.Fatalf("写另一个头部：%v", err)
	}
	hdr, ok, err := ReadHeader(filepath.Join(root, "other.jsonl"))
	if err != nil || !ok {
		t.Fatalf("读头部：%v（存在=%v）", err, ok)
	}
	if hdr.Trace != "other.jsonl" {
		t.Errorf("头部应记 trace 文件名，实得 %q", hdr.Trace)
	}
	// 头部指向另一个 trace（配错）⇒ 作废
	if _, err := CheckTrace(tracePath, fp, InvalidationOptions{}); err != nil {
		t.Errorf("同指纹仍应通过：%v", err)
	}
	mis := filepath.Join(root, "mis.jsonl")
	if err := WriteHeader(mis, Header{Fingerprints: fp, Trace: "t.jsonl"}); err != nil {
		t.Fatalf("写错配头部：%v", err)
	}
	if _, err := CheckTrace(mis, fp, InvalidationOptions{}); err == nil || !strings.Contains(err.Error(), "配错") {
		t.Errorf("头部与 trace 配错必须作废，实得 %v", err)
	}
	// 空身份头部拒绝落盘（声明空身份比不声明更坏）
	if err := NewRecorderWithHeaderOk(t, root, Fingerprints{SchemaVersion: SchemaVersion}); err == nil {
		t.Errorf("空 tool_fingerprint 的录制必须被拒")
	}

	// 落盘取证（脚本 scripts/replay-fidelity.sh 打印与复核这一节）
	c0 := CheckTrace2Err(tracePath, fp, InvalidationOptions{})
	ev := invalidationEvidence{Trace: filepath.Base(tracePath), Header: fp, Cases: []invalidationCase{
		{Name: "指纹相同（对侧：不得误报）", Invalidated: c0 != nil, Error: errString(c0)},
	}}
	if ev.Cases[0].Invalidated {
		t.Fatalf("对侧取证写反了（指纹相同被记成作废）：%s", ev.Cases[0].Error)
	}
	for _, tc := range cases {
		cur := tc.mut(fp)
		err := CheckTrace2Err(tracePath, cur, InvalidationOptions{})
		ev.Cases = append(ev.Cases, invalidationCase{
			Name: tc.name, Field: tc.key, Invalidated: err != nil,
			Error: errString(err), HasBothSides: err != nil && strings.Contains(err.Error(), fp.String()) && strings.Contains(err.Error(), cur.String()),
		})
	}
	ev.AllInvalidated = true
	for _, c := range ev.Cases[1:] {
		if !c.Invalidated || !c.HasBothSides {
			ev.AllInvalidated = false
		}
	}
	writeReport(t, fixtureRoot(t), "invalidation.json", ev)
}

// invalidationCase / invalidationEvidence —— T4.6 的取证件（脚本读它打印作废判定那一节）。
type invalidationCase struct {
	Name         string `json:"name"`
	Field        string `json:"field,omitempty"`
	Invalidated  bool   `json:"invalidated"`
	HasBothSides bool   `json:"has_both_sides"`
	Error        string `json:"error,omitempty"`
}

type invalidationEvidence struct {
	Trace          string             `json:"trace"`
	Header         Fingerprints       `json:"header"`
	AllInvalidated bool               `json:"all_invalidated"`
	Cases          []invalidationCase `json:"cases"`
}

// CheckTrace2Err —— 作废判定的取证取用（只要判词原文）。
func CheckTrace2Err(p string, cur Fingerprints, opt InvalidationOptions) error {
	_, err := CheckTrace(p, cur, opt)
	return err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// NewRecorderWithHeaderOk —— 试建一个"空身份"的录制器，期望报错（用例里的期望值检查用）。
func NewRecorderWithHeaderOk(t *testing.T, root string, fp Fingerprints) error {
	t.Helper()
	d, err := NewRecorderWithHeader(filepath.Join(root, "empty-id.jsonl"), Normalizer{BaseDir: root},
		func(context.Context, Call, map[string]any) Result { return Result{} }, fp)
	if err == nil && d != nil {
		d.Close()
	}
	return err
}

// ── ⑦ 指纹相同 ⇒ 正常回放（反例）────────────────────────────────────────────

func TestTraceFingerprintUnchangedReplays(t *testing.T) {
	root := t.TempDir()
	tracePath := filepath.Join(root, "t.jsonl")
	fp := Fingerprints{
		SchemaVersion:     SchemaVersion,
		ToolFingerprint:   ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`}),
		EngineFingerprint: "b10470-34af94cd9",
	}
	rec, err := NewRecorderWithHeader(tracePath, Normalizer{BaseDir: root}, func(context.Context, Call, map[string]any) Result {
		return Result{Body: []byte("正文 v1")}
	}, fp)
	if err != nil {
		t.Fatalf("NewRecorderWithHeader：%v", err)
	}
	if _, err := rec.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt"}); err != nil {
		t.Fatalf("录制：%v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}

	disp, cur, err := NewReplayerChecked(tracePath, Normalizer{BaseDir: root}, 0, fp, InvalidationOptions{})
	if err != nil {
		t.Fatalf("指纹相同必须能建回放器：%v", err)
	}
	if cur.Consumed != 1 {
		t.Errorf("游标应为 1，实得 %d", cur.Consumed)
	}
	res, err := disp.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatalf("回放应命中：%v", err)
	}
	if !bytes.Equal(res.Body, []byte("正文 v1")) || res.Source != SourceRecorded {
		t.Errorf("回放正文/来源不对：%q %s", res.Body, res.Source)
	}
	// 引擎指纹两侧都空（纯工具场景）也应放行 —— "未记录"口径，而不是 fail-closed 红
	fpNoEngine := fp
	fpNoEngine.EngineFingerprint = ""
	rec2 := fingerprintOnlyTrace(t, root, "no-engine.jsonl", fpNoEngine)
	if _, _, err := NewReplayerChecked(rec2, Normalizer{BaseDir: root}, 0, fpNoEngine, InvalidationOptions{}); err != nil {
		t.Errorf("两侧都没记引擎指纹应视为未记录（放行），实得 %v", err)
	}
}

func fingerprintOnlyTrace(t *testing.T, root, name string, fp Fingerprints) string {
	t.Helper()
	p := filepath.Join(root, name)
	rec, err := NewRecorderWithHeader(p, Normalizer{BaseDir: root}, func(context.Context, Call, map[string]any) Result {
		return Result{Body: []byte("x")}
	}, fp)
	if err != nil {
		t.Fatalf("建 trace %s：%v", name, err)
	}
	if _, err := rec.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt"}); err != nil {
		t.Fatalf("录制 %s：%v", name, err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close %s：%v", name, err)
	}
	return p
}

// ── ⑧ 缺头部 ⇒ 作废（fail-closed），显式放行是唯一出口 ──────────────────────

func TestTraceHeaderlessIsInvalidated(t *testing.T) {
	root := t.TempDir()
	tracePath := filepath.Join(root, "old.jsonl")
	tr, err := Create(tracePath)
	if err != nil {
		t.Fatalf("Create：%v", err)
	}
	if _, err := tr.Append(Record{Tool: "read_file", ArgsCanonical: []byte(`{"path":"/x"}`), ResultBody: []byte("旧")}); err != nil {
		t.Fatalf("Append：%v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}
	cur := Fingerprints{SchemaVersion: SchemaVersion, ToolFingerprint: "sha256:deadbeef"}

	_, err = CheckTrace(tracePath, cur, InvalidationOptions{})
	if err == nil || !errors.Is(err, ErrTraceInvalidated) {
		t.Fatalf("无头部必须作废（fail-closed），实得 %v", err)
	}
	if !strings.Contains(err.Error(), "AllowHeaderless") {
		t.Errorf("判词应告诉人唯一出口（AllowHeaderless）：%s", err.Error())
	}
	if _, _, err := NewReplayerChecked(tracePath, Normalizer{BaseDir: root}, 0, cur, InvalidationOptions{}); err == nil {
		t.Errorf("缺头部时回放也必须被拒")
	}
	// 唯一出口：显式放行
	if _, err := CheckTrace(tracePath, cur, InvalidationOptions{AllowHeaderless: true}); err != nil {
		t.Errorf("显式 AllowHeaderless 应放行，实得 %v", err)
	}
	disp, _, err := NewReplayerChecked(tracePath, Normalizer{BaseDir: root}, 0, cur, InvalidationOptions{AllowHeaderless: true})
	if err != nil {
		t.Fatalf("显式放行后应能建回放器：%v", err)
	}
	if _, err := disp.Execute(context.Background(), "read_file", map[string]any{"path": "/x"}); err != nil {
		t.Errorf("放行后应能命中：%v", err)
	}
	// 头部的头部版本不认识 ⇒ 也作废
	if err := os.WriteFile(HeaderPath(tracePath), []byte(`{"v":99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckTrace(tracePath, cur, InvalidationOptions{AllowHeaderless: true}); err == nil {
		t.Errorf("头部版本不认识必须作废")
	}
}

// ── ⑨ 工具表指纹 ────────────────────────────────────────────────────────────

func TestToolTableFingerprintStable(t *testing.T) {
	a := ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`, "bash": `{"cmd":"string"}`})
	b := ToolTableFingerprint(map[string]string{"bash": `{"cmd":"string"}`, "read_file": `{"path":"string"}`})
	if a != b {
		t.Errorf("工具表指纹不得依赖 map 迭代序：%s vs %s", a, b)
	}
	// 再复算多次（map 迭代序随机 ⇒ 任何一次不同都要露馅）
	for i := 0; i < 20; i++ {
		c := ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`, "bash": `{"cmd":"string"}`})
		if c != a {
			t.Fatalf("第 %d 次复算不一致：%s vs %s", i, c, a)
		}
	}
	c := ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`, "bash": `{"cmd":"integer"}`})
	if c == a {
		t.Errorf("参数 schema 变了，指纹必须变")
	}
	d := ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`, "bash": `{"cmd":"string"}`, "ls": `{}`})
	if d == a {
		t.Errorf("多一个工具，指纹必须变")
	}
	if ToolTableFingerprint(nil) != ToolTableFingerprint(map[string]string{}) {
		t.Errorf("nil 与空表的口径要一致（都空 ⇒ 同值）；这一条只是防止以后有人把 nil 当特例")
	}
	if !strings.HasPrefix(a, "sha256:") {
		t.Errorf("指纹形态应是 sha256:<hex>，实得 %s", a)
	}
}

// ── ⑩ DeterministicID（E9）─────────────────────────────────────────────────

func TestDeterministicID(t *testing.T) {
	id := DeterministicID("trace-1", 7)
	// **独立**复算：sha256(traceID ‖ 0x1f ‖ seq) 前 128 位
	indep := "zid:" + hexPrefix(Digest([]byte("trace-1\x1f7")), 32)
	if id != indep {
		t.Errorf("确定性 ID 口径应等于 sha256(traceID‖0x1f‖seq) 前 128 位：%s vs %s", id, indep)
	}
	if DeterministicID("trace-1", 7) != id {
		t.Errorf("同一 (traceID, seq) 必须同 ID")
	}
	if DeterministicID("trace-1", 8) == id {
		t.Errorf("不同 seq 必须不同 ID")
	}
	if DeterministicID("trace-2", 7) == id {
		t.Errorf("不同 traceID 必须不同 ID")
	}
	if len(strings.TrimPrefix(id, "zid:")) != 32 {
		t.Errorf("形态应是 zid:<32 hex>，实得 %s", id)
	}
}

// hexPrefix —— 取 Digest 十六进制部分的前 n 个字符（独立复算用）。
func hexPrefix(digest string, n int) string {
	s := strings.TrimPrefix(digest, "sha256:")
	if len(s) < n {
		return s
	}
	return s[:n]
}

// ── ⑪ 门禁夹具（脚本 scripts/replay-fidelity.sh 的输入源）──────────────────

// 夹具的**固定内容**：改这里 ⇒ 三条指标的 sha 全变，属有意变更（脚本会打印新值）。
const (
	fixInBody  = "输入夹具 v1\n"
	fixOutBody = "产物 A\n"
	fixLogBody = "日志 1\n"
)

// gateFixture —— 脚本读的那份报告（字段名就是脚本的判据键）。
type gateFixture struct {
	Dir            string             `json:"dir"`
	Fidelity       FidelityReport     `json:"fidelity"`
	Equivalence    EquivalenceReport  `json:"equivalence"`
	Determinism    *DeterminismReport `json:"determinism"`
	Bisect         Divergence         `json:"bisect"`
	RecordedDigest string             `json:"recorded_digest"`
	ReplayADigest  string             `json:"replay_a_digest"`
	ReplayBDigest  string             `json:"replay_b_digest"`
	WorldUntouched bool               `json:"world_untouched"`
	LiveCalls      int                `json:"live_calls"`
	Verdict        string             `json:"verdict"`
}

// TestFidelityFixtureForGate —— 跑**固定夹具**并把报告写到 ZERG_REPLAY_FIDELITY_DIR：
//
//	<root>/world/          输入（in.txt）与真发写入的文件（out.txt / log.txt）
//	<root>/trace/          录制得到的 trace（+ 头部 sidecar）与变异体 trace
//	<root>/recorded/       录制运行（ModeRecord）的产物
//	<root>/replay-a/       回放运行（ModeReplay #1）的产物
//	<root>/replay-b/       回放运行（ModeReplay #2）的产物（③ 用）
//	<root>/fidelity.json   三条指标的结论 + 三份产物摘要（脚本按字段判）
//	<root>/fidelity.txt    人读三行
//
// 产物口径写死：产物 = **工具结果派生出来的东西**（answer.txt / steps.tsv / effects.tsv），
// 不含 source / duration 这类"这次怎么跑的"读数 —— 把环境读数写进产物，等价性就永远不成立。
func TestFidelityFixtureForGate(t *testing.T) {
	root := fixtureRoot(t)
	world := filepath.Join(root, "world")
	traceDir := filepath.Join(root, "trace")
	recorded, replayA, replayB := filepath.Join(root, "recorded"), filepath.Join(root, "replay-a"), filepath.Join(root, "replay-b")

	// 铺夹具（每次运行都从**同一个初态**开始；回放前还会再铺一次以证明回放没写世界）
	setupWorld := func() error {
		for _, f := range []string{"out.txt", "log.txt"} {
			if err := os.Remove(filepath.Join(world, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err := os.MkdirAll(traceDir, 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(world, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(world, "in.txt"), []byte(fixInBody), 0o644)
	}
	if err := setupWorld(); err != nil {
		t.Fatalf("铺夹具：%v", err)
	}
	pristineWorld := mustDigestDir(t, world)

	fp := Fingerprints{
		SchemaVersion:     SchemaVersion,
		ToolFingerprint:   ToolTableFingerprint(map[string]string{"read_file": `{"path":"string"}`, "write_file": `{"path":"string","body":"string"}`, "hash_file": `{"path":"string"}`}),
		EngineFingerprint: "b10470-34af94cd9", // 实测形态（infergeom.Geometry.SystemFingerprint 的值级复用）
		EngineDigest:      Digest([]byte("gate-engine-bin")),
		ModelDigest:       Digest([]byte("gate-model-weights")),
	}
	norm := Normalizer{BaseDir: world}
	tracePath := filepath.Join(traceDir, "recorded.jsonl")

	// ① 录制运行（真发真写盘）
	var liveCalls int32
	recF, err := runGateFixture(ModeRecord, tracePath, world, recorded, norm, fp, &liveCalls, InvalidationOptions{})
	if err != nil {
		t.Fatalf("录制运行：%v", err)
	}
	if liveCalls == 0 {
		t.Fatalf("录制运行必须真发")
	}
	if recF.Attempted != 6 || recF.Missed != 0 {
		t.Errorf("录制运行应 6 次真发且无未命中：%s", recF)
	}
	if _, err := os.Stat(filepath.Join(world, "out.txt")); err != nil {
		t.Fatalf("录制运行应真的写出 out.txt：%v", err)
	}
	recordedSet := mustDigestDir(t, recorded)
	if recordedSet.Digest == "" || recordedSet.Len() == 0 {
		t.Fatalf("录制产物集不应为空")
	}

	// ②/③ 回放运行两趟（世界先恢复原状 ⇒ 任何写入都能被抓到）
	if err := setupWorld(); err != nil {
		t.Fatalf("回放前铺夹具：%v", err)
	}
	var replayCalls int32
	repF, err := runGateFixture(ModeReplay, tracePath, world, replayA, norm, fp, &replayCalls, InvalidationOptions{})
	if err != nil {
		t.Fatalf("回放运行 A：%v", err)
	}
	repF2, err := runGateFixture(ModeReplay, tracePath, world, replayB, norm, fp, &replayCalls, InvalidationOptions{})
	if err != nil {
		t.Fatalf("回放运行 B：%v", err)
	}
	replayASet, replayBSet := mustDigestDir(t, replayA), mustDigestDir(t, replayB)
	worldAfter := mustDigestDir(t, world)

	// 第 4 趟：把回放**叠在 replay-a 自己的产物上**再跑一次（replay(replay(x))）
	repF3, err := runGateFixture(ModeReplay, tracePath, world, replayA, norm, fp, &replayCalls, InvalidationOptions{})
	if err != nil {
		t.Fatalf("叠加回放运行：%v", err)
	}
	replayASet2 := mustDigestDir(t, replayA)

	fid := repF
	eq := CompareArtifacts(recordedSet, replayASet)
	det := NewDeterminismReport(replayASet, replayBSet, replayASet2, pristineWorld, worldAfter)
	det.Runs = 3

	// ④ 二分：变异体（把第 4 条录播的结果改掉）与基准比 —— 最小前缀应落在 4
	mutPath := filepath.Join(traceDir, "mutant.jsonl")
	baseRecs, err := LoadRecords(tracePath)
	if err != nil {
		t.Fatalf("读基准 trace：%v", err)
	}
	if len(baseRecs) != 6 {
		t.Fatalf("夹具应有 6 条录播，实得 %d", len(baseRecs))
	}
	mutRecs := make([]Record, len(baseRecs))
	copy(mutRecs, baseRecs)
	mutRecs[3].ResultBody = []byte("改写过的第 4 条结果\n")
	mutRecs[3].ResultDigest = ""
	if err := writeTraceFile(mutPath, mutRecs); err != nil {
		t.Fatalf("写变异体 trace：%v", err)
	}
	mutRecs, err = LoadRecords(mutPath)
	if err != nil {
		t.Fatalf("读变异体 trace：%v", err)
	}
	probeN := 0
	bisectRun := func(recs []Record) (*ArtifactSet, error) {
		probeN++
		p := filepath.Join(root, "bisect-trace", fmt.Sprintf("hybrid-%03d.jsonl", probeN))
		if err := writeTraceFile(p, recs); err != nil {
			return nil, err
		}
		out := filepath.Join(root, "bisect-out", fmt.Sprintf("run-%03d", probeN))
		var calls int32
		// AllowHeaderless：混合 trace 是**派生诊断载体**（不是录制产物 ⇒ 没有身份声明）；
		// 这里只问"产物变没变"，故显式放行 —— 显式放行本身也写进用例注释，不是悄悄关掉判定。
		if _, err := runGateFixture(ModeReplay, p, world, out, norm, fp, &calls, InvalidationOptions{AllowHeaderless: true}); err != nil {
			return nil, err
		}
		if calls != 0 {
			return nil, fmt.Errorf("二分探测期出现了真发（%d 次）", calls)
		}
		return DigestDir(out, ArtifactOptions{})
	}
	div, err := BisectEarliestDivergence(mutRecs, baseRecs, bisectRun)
	if err != nil {
		t.Fatalf("BisectEarliestDivergence（夹具）：%v", err)
	}

	// 内联断言（夹具自己也要过；否则脚本读到的是"一份漂亮的假报告"）
	if err := fid.Verdict(); err != nil || fid.F != 1.0 || fid.Missed != 0 {
		t.Errorf("① F 应 =1.0 且 PASS：%s / %v", fid, err)
	}
	if err := eq.Verdict(); err != nil || !eq.Equal {
		t.Errorf("② 产物应相等：%s / %v", eq, err)
	}
	if err := det.Verdict(); err != nil || !det.Stable || !det.LayerStable || !det.FixtureUntouched {
		t.Errorf("③ 确定性应全真：%s / %v", det, err)
	}
	if div.EarliestSeq != 4 || div.PrefixLen != 4 {
		t.Errorf("④ 二分应给出最早分歧 seq=4（前缀 4 条），实得 %+v", div)
	}
	if replayCalls != 0 {
		t.Errorf("回放期真发次数必须为 0，实得 %d", replayCalls)
	}
	if worldAfter.Digest != pristineWorld.Digest {
		t.Errorf("回放期不得改动世界目录：%s vs %s", worldAfter.Digest, pristineWorld.Digest)
	}
	if repF2.Missed != 0 || repF3.Attempted != 6 {
		t.Errorf("第 2/3 趟回放也应是 6 次尝试全命中：%s / %s", repF2, repF3)
	}

	// 写报告（脚本读这些文件；值都是可复核的摘要与计数）
	gf := gateFixture{
		Dir: root, Fidelity: fid, Equivalence: eq, Determinism: det, Bisect: div,
		RecordedDigest: recordedSet.Digest, ReplayADigest: replayASet2.Digest, ReplayBDigest: replayBSet.Digest,
		WorldUntouched: worldAfter.Digest == pristineWorld.Digest, LiveCalls: int(replayCalls),
		Verdict: "PASS",
	}
	if all := Verdict(fid, eq, det); all != nil {
		gf.Verdict = "FAIL: " + all.Error()
	}
	writeReport(t, root, "fidelity.json", gf)
	txt := fmt.Sprintf("① %s\n② %s\n③ %s\n④ %s\n产物摘要 recorded=%s replay-a=%s replay-b=%s\n世界目录未变=%v 回放期真发=%d\n",
		fid, eq, det, div, recordedSet.Digest, replayASet2.Digest, replayBSet.Digest, gf.WorldUntouched, replayCalls)
	if err := os.WriteFile(filepath.Join(root, "fidelity.txt"), []byte(txt), 0o644); err != nil {
		t.Fatalf("写 fidelity.txt：%v", err)
	}
	if gf.Verdict != "PASS" {
		t.Fatalf("夹具自身判词应为 PASS：%s", gf.Verdict)
	}
}

// runGateFixture —— 用本包的**真链路**跑一遍固定夹具并产出"产物"。
//
// 产物 = 工具结果派生出的三份文件（answer.txt / steps.tsv / effects.tsv）：
//   - recorded 与 replayed 的**工具副作用**当然不同（回放不真发），但产物必须逐字节相同 ——
//     这正是 E7 ② 要说的事。
func runGateFixture(mode Mode, tracePath, world, outDir string, norm Normalizer, fp Fingerprints, liveCalls *int32, opt InvalidationOptions) (FidelityReport, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return FidelityReport{}, err
	}
	tools := map[string]ToolFunc{
		"read_file": func(_ context.Context, _ Call, args map[string]any) Result {
			atomic.AddInt32(liveCalls, 1)
			p, _ := args["path"].(string)
			b, err := os.ReadFile(p)
			if err != nil {
				return Result{Err: err.Error()}
			}
			return Result{Body: b}
		},
		"write_file": func(_ context.Context, _ Call, args map[string]any) Result {
			atomic.AddInt32(liveCalls, 1)
			p, _ := args["path"].(string)
			body, _ := args["body"].(string)
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				return Result{Err: err.Error()}
			}
			return Result{Body: []byte("ok\n")}
		},
		"hash_file": func(_ context.Context, _ Call, args map[string]any) Result {
			atomic.AddInt32(liveCalls, 1)
			p, _ := args["path"].(string)
			b, err := os.ReadFile(p)
			if err != nil {
				return Result{Err: err.Error()}
			}
			return Result{Body: []byte(Digest(b) + "\n")}
		},
	}
	effectScope := func(call Call, args map[string]any) []string {
		if call.Tool != "write_file" {
			return nil
		}
		p, _ := args["path"].(string)
		return []string{"fs:" + p}
	}

	var disp *Dispatcher
	var err error
	switch mode {
	case ModeRecord:
		disp, err = NewRecorderWithHeader(tracePath, norm, func(ctx context.Context, call Call, args map[string]any) Result {
			tf, ok := tools[call.Tool]
			if !ok {
				return Result{Err: "未知工具 " + call.Tool}
			}
			return tf(ctx, call, args)
		}, fp)
		if err != nil {
			return FidelityReport{}, err
		}
		disp.Effects = NewEffectLedger()
		disp.EffectScope = effectScope
	case ModeReplay:
		disp, _, err = NewReplayerChecked(tracePath, norm, 0, fp, opt)
		if err != nil {
			return FidelityReport{}, err
		}
		// Live **装上**：回放分支结构性不经过它（真发计数为 0 就是这条的实证）
		disp.Live = func(ctx context.Context, call Call, args map[string]any) Result {
			return tools[call.Tool](ctx, call, args)
		}
		disp.EffectScope = effectScope
	default:
		return FidelityReport{}, fmt.Errorf("夹具不支持模式 %s", mode)
	}
	defer disp.Close()

	calls := gateCalls(world)
	counter := NewFidelityCounter()
	var answer strings.Builder
	var steps, effects strings.Builder
	for i, c := range calls {
		var res Result
		var err error
		if mode == ModeRecord {
			res, err = disp.Execute(context.Background(), c.tool, c.args)
			counter.Record(c.tool, "", err)
		} else {
			res, err = disp.Execute(context.Background(), c.tool, c.args)
			counter.RecordCall(c.tool, "", res, err)
		}
		if err != nil {
			return FidelityReport{}, fmt.Errorf("第 %d 次调用（%s）失败：%w", i+1, c.tool, err)
		}
		if res.Source == SourceDeduped {
			return FidelityReport{}, fmt.Errorf("第 %d 次调用被去重（夹具不应出现）", i+1)
		}
		fmt.Fprintf(&answer, "#%d %s\n%s\n", i+1, c.tool, res.Body)
		fmt.Fprintf(&steps, "%d	%s	%s	%s	%s\n", i+1, c.tool, short(callDigest(norm, c)), short(Digest(res.Body)), res.Err)
		for j, ef := range res.Effects {
			fmt.Fprintf(&effects, "%d\t%d\t%s\t%s\n", i+1, j, ef.Scope, ef.Key)
		}
	}
	for _, f := range []struct {
		name string
		body string
	}{{"answer.txt", answer.String()}, {"steps.tsv", steps.String()}, {"effects.tsv", effects.String()}} {
		if err := os.WriteFile(filepath.Join(outDir, f.name), []byte(f.body), 0o644); err != nil {
			return FidelityReport{}, err
		}
	}
	return counter.Report(), nil
}

// gateCall —— 夹具里的一次调用（顺序写死；第 4 次的**结果**被变异体改掉）。
type gateCall struct {
	tool string
	args map[string]any
}

// gateCalls —— 固定调用序列（6 次）。
func gateCalls(world string) []gateCall {
	in := filepath.Join(world, "in.txt")
	out := filepath.Join(world, "out.txt")
	log := filepath.Join(world, "log.txt")
	return []gateCall{
		{"read_file", map[string]any{"path": in}},
		{"write_file", map[string]any{"path": out, "body": fixOutBody}},
		{"read_file", map[string]any{"path": out}},
		{"write_file", map[string]any{"path": log, "body": fixLogBody}},
		{"hash_file", map[string]any{"path": out}},
		{"read_file", map[string]any{"path": log}},
	}
}

func callDigest(norm Normalizer, c gateCall) string {
	call, err := norm.Normalize(c.tool, c.args)
	if err != nil {
		return "sha256:归一化失败"
	}
	return call.ArgsDigest
}

// writeTraceFile —— 把一组录播事实写成一条新 trace（Seq 由载体赋值）。
func writeTraceFile(path string, recs []Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tr, err := Create(path)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if _, err := tr.Append(r); err != nil {
			tr.Close()
			return err
		}
	}
	return tr.Close()
}

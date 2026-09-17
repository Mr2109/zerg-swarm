// replay_test.go —— T4.1 录播层实证：录制/回放/严格未命中/规范化与指纹/效果级幂等/
// 消费游标/前置状态校验/trace 损坏，逐条钉死。
//
// 为什么叶包要有自己的用例：录播层的每条性质都是**纯函数级或纯 IO 级**的事实
// （规范化是纯函数、匹配是纯查表、幂等是纯计数、游标是纯位移），可以在这里用最小输入直接钉住；
// 接线之后（把 agent 的工具执行路径切到 Dispatcher，属后续批）的用例只负责证明
// "链路里拿到的是同一个决定"。
//
// 纪律：**每条用例自带反例**——只断"该报错的报错了"会漏掉"把所有调用都判成未命中"这种假绿；
// 只断"第二次没执行"会漏掉"干脆一次都不执行"。所以每条都补一条对侧断言。
//
// 用例①里用到的"真写文件"是真 IO（不是 mock）："回放不真发"这件事，只有拿真文件 + 真 sha256
// 才能证明 —— mock 掉文件系统等于把要验证的东西替换掉了。
package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// fakeTool —— 一个**会真产生副作用**的假工具：每次"真发"都把第几次写进落盘文件。
// 录制态必须看到它写；回放态必须看到它**一点没写**。
type fakeTool struct {
	calls  int
	path   string // 非空 ⇒ 每次真发覆写这个文件（真副作用）
	body   []byte
	errTxt string
}

func (f *fakeTool) hook() ToolFunc {
	return func(ctx context.Context, call Call, args map[string]any) Result {
		f.calls++
		if f.path != "" {
			if err := os.WriteFile(f.path, []byte(fmt.Sprintf("真发第 %d 次\n", f.calls)), 0o644); err != nil {
				return Result{Err: err.Error()}
			}
		}
		return Result{Body: f.body, Err: f.errTxt}
	}
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s：%v", path, err)
	}
	return Digest(b)
}

// indepEffectKey —— 在用例里**独立**复算 effect_key（不用被测代码），钉住键的组成口径。
func indepEffectKey(tool string, canonical []byte, scope string) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0x1f})
	h.Write([]byte(canonical))
	h.Write([]byte{0x1f})
	h.Write([]byte(scope))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// indepFingerprint —— 在用例里**独立**复算请求指纹（钉住"工具名 ‖ 0x1f ‖ 规范化参数"的口径）。
func indepFingerprint(tool string, canonical []byte) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0x1f})
	h.Write([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func mustNormalize(t *testing.T, n Normalizer, tool string, args map[string]any) Call {
	t.Helper()
	c, err := n.Normalize(tool, args)
	if err != nil {
		t.Fatalf("Normalize(%s) 不应失败：%v", tool, err)
	}
	return c
}

var ctx = context.Background()

// ── ① 录制 → 回放命中（噪声字段变了照样命中），并证明"没用原始字节哈希" ──────

func TestReplayHit_NoiseIgnoredAndCanonicalNotRawBytes(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")
	tool := &fakeTool{body: []byte("文件内容 v1")}

	rec, err := NewRecorder(tracePath, norm, tool.hook())
	if err != nil {
		t.Fatalf("NewRecorder：%v", err)
	}
	recordArgs := map[string]any{
		"path": "sub/a.txt", "ts": 111, "request_id": "req-1", "limit": 1.0,
	}
	got, err := rec.Execute(ctx, "read", recordArgs)
	if err != nil {
		t.Fatalf("录制期执行：%v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("录制期应真发 1 次，实得 %d", tool.calls)
	}
	if got.Source != SourceLive || got.Seq != 1 {
		t.Fatalf("录制结果应 source=live seq=1，实得 source=%s seq=%d", got.Source, got.Seq)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}

	// 回放：噪声字段全变、路径写成没 Clean 的相对形式、数字由 1.0 写成 1
	disp, cur, err := NewReplayer(tracePath, norm, 0)
	if err != nil {
		t.Fatalf("NewReplayer：%v", err)
	}
	if cur.Consumed != 1 {
		t.Fatalf("载入 1 条录播后游标应为 1，实得 %d（%+v）", cur.Consumed, cur)
	}
	replayArgs := map[string]any{
		"path": "./sub/../sub/a.txt", "ts": 999, "request_id": "req-2", "limit": 1,
	}
	hit, err := disp.Execute(ctx, "read", replayArgs)
	if err != nil {
		t.Fatalf("噪声字段变化后回放应命中（规范化就是为这件事存在的）：%v", err)
	}
	if !bytes.Equal(hit.Body, []byte("文件内容 v1")) {
		t.Errorf("回放正文应与录制正文逐字节相同，实得 %q", hit.Body)
	}
	if hit.Source != SourceRecorded || hit.Seq != 1 {
		t.Errorf("回放应 source=recorded seq=1，实得 source=%s seq=%d", hit.Source, hit.Seq)
	}
	if tool.calls != 1 {
		t.Errorf("回放期真发次数应保持 1，实得 %d", tool.calls)
	}

	// 规范化前后：同一个逻辑调用必须落到同一个指纹
	ca := mustNormalize(t, norm, "read", recordArgs)
	cb := mustNormalize(t, norm, "read", replayArgs)
	if ca.ArgsDigest != cb.ArgsDigest {
		t.Errorf("规范化后 ArgsDigest 应相同：%s vs %s", ca.ArgsDigest, cb.ArgsDigest)
	}
	if !bytes.Equal(ca.Canonical, cb.Canonical) {
		t.Errorf("规范化后 ArgsCanonical 应逐字节相同：%s vs %s", ca.Canonical, cb.Canonical)
	}
	// **关键**：指纹不是"原始字节哈希"——原始 JSON 里带噪声、路径写法不同，
	// 如果拿原始字节算哈希，这条回放必然未命中（E3 的病根）。
	raw, err := json.Marshal(replayArgs)
	if err != nil {
		t.Fatalf("marshal：%v", err)
	}
	if cb.ArgsDigest == Digest(raw) {
		t.Errorf("ArgsDigest 等于原始字节哈希（%s）⇒ 规范化没生效，噪声字段会把命中率打到 0", Digest(raw))
	}
	if cb.Fingerprint != indepFingerprint("read", cb.Canonical) {
		t.Errorf("fingerprint 口径不符：%s vs 独立复算 %s", cb.Fingerprint, indepFingerprint("read", cb.Canonical))
	}

	// 反例 1：语义字段变了 ⇒ 必须未命中（证明匹配不是"什么都能中"）
	if _, err := disp.Execute(ctx, "read", map[string]any{
		"path": "other.txt", "ts": 999, "request_id": "req-2", "limit": 1,
	}); !errors.Is(err, ErrReplayMiss) {
		t.Errorf("语义字段（path）变化应未命中（ErrReplayMiss），实得 %v", err)
	}

	// 反例 2（异常路径也入 trace，E10）：录制时失败的调用，回放期照样命中并原样返回失败
	failing := filepath.Join(dir, "trace_fail.jsonl")
	bad := &fakeTool{errTxt: "boom: 子进程退出码 3"}
	rec2, err := NewRecorder(failing, norm, bad.hook())
	if err != nil {
		t.Fatalf("NewRecorder：%v", err)
	}
	if _, err := rec2.Execute(ctx, "bash", map[string]any{"command": "false"}); err != nil {
		t.Fatalf("录制失败路径：%v", err)
	}
	rec2.Close()
	disp2, _, err := NewReplayer(failing, norm, 0)
	if err != nil {
		t.Fatalf("NewReplayer：%v", err)
	}
	back, err := disp2.Execute(ctx, "bash", map[string]any{"command": "false"})
	if err != nil {
		t.Fatalf("失败路径的录播也应命中（只录成功的 cassette 是常见坑）：%v", err)
	}
	if back.Err != "boom: 子进程退出码 3" {
		t.Errorf("失败原文应原样回放，实得 %q", back.Err)
	}
	if bad.calls != 1 {
		t.Errorf("回放期不应再真发，实得 %d 次", bad.calls)
	}

	// 子用例：规范化口径逐条钉住
	t.Run("规一化口径", func(t *testing.T) {
		n := Normalizer{BaseDir: dir}
		a := mustNormalize(t, n, "t", map[string]any{"b": 1, "a": "x", "ts": 1})
		b := mustNormalize(t, n, "t", map[string]any{"a": "x", "b": 1.0, "ts": 2})
		if string(a.Canonical) != `{"a":"x","b":1}` {
			t.Errorf("规范化应为「键排序 + 数字统一 + 去噪」：实得 %s", a.Canonical)
		}
		if a.ArgsDigest != b.ArgsDigest {
			t.Errorf("键序/数字写法/噪声字段不同 ⇒ 指纹应相同：%s vs %s", a.ArgsDigest, b.ArgsDigest)
		}
		// 路径绝对化（Clean + Join BaseDir）
		p1 := mustNormalize(t, n, "t", map[string]any{"path": "a/../a.txt"})
		p2 := mustNormalize(t, n, "t", map[string]any{"path": "a.txt"})
		if p1.ArgsDigest != p2.ArgsDigest {
			t.Errorf("路径绝对化后应同指纹：%s vs %s", p1.ArgsDigest, p2.ArgsDigest)
		}
		if want := `{"path":"` + filepath.Join(dir, "a.txt") + `"}`; string(p1.Canonical) != want {
			t.Errorf("路径应绝对化：实得 %s，期望 %s", p1.Canonical, want)
		}
		// MatchOn 收窄 ⇒ 未列字段不进指纹
		narrow := Normalizer{BaseDir: dir, MatchOn: []string{"path"}}
		c := mustNormalize(t, narrow, "t", map[string]any{"path": "a.txt", "content": "很长"})
		if string(c.Canonical) != `{"path":"`+filepath.Join(dir, "a.txt")+`"}` {
			t.Errorf("MatchOn 收窄后应只剩 path：实得 %s", c.Canonical)
		}
		// Drop 显式关掉 ⇒ 噪声字段回到指纹里（"nil = 默认去噪 / 空切片 = 不去噪"是两件事）
		noisy := Normalizer{BaseDir: dir, Drop: []string{}}
		c1 := mustNormalize(t, noisy, "t", map[string]any{"path": "a.txt", "ts": 1})
		c2 := mustNormalize(t, noisy, "t", map[string]any{"path": "a.txt", "ts": 2})
		if c1.ArgsDigest == c2.ArgsDigest {
			t.Error("显式关掉去噪后 ts 应进指纹 ⇒ 两个 ts 不同的调用指纹应不同")
		}
		// 不支持的类型报错（不静默丢字段）
		if _, err := n.Normalize("t", map[string]any{"when": time.Now()}); !errors.Is(err, ErrNormalize) {
			t.Errorf("不支持的类型应报 ErrNormalize，实得 %v", err)
		}
	})
}

// ── ② 未命中 ⇒ 报错且带 fingerprint + 最近邻候选（绝不静默放行）───────────

func TestReplayMiss_ErrorCarriesFingerprintAndNearestNeighbors(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")

	tr, err := Create(tracePath)
	if err != nil {
		t.Fatalf("Create：%v", err)
	}
	seeded := []map[string]any{
		{"pattern": "foo", "path": "a.txt"},
		{"pattern": "bar", "path": "b.txt"},
	}
	for i, a := range seeded {
		c := mustNormalize(t, norm, "grep", a)
		if _, err := tr.Append(Record{
			Tool: "grep", ArgsCanonical: c.Canonical, ArgsDigest: c.ArgsDigest,
			ResultBody: []byte(fmt.Sprintf("录制正文 %d", i)),
		}); err != nil {
			t.Fatalf("Append：%v", err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}

	disp, _, err := NewReplayer(tracePath, norm, 0)
	if err != nil {
		t.Fatalf("NewReplayer：%v", err)
	}

	missArgs := map[string]any{"pattern": "foo", "path": "c.txt"}
	_, err = disp.Execute(ctx, "grep", missArgs)
	if err == nil {
		t.Fatal("未命中必须报错——静默放行会让一次\"回放\"退化成\"真跑\"，而报告上仍写着回放通过")
	}
	if !errors.Is(err, ErrReplayMiss) {
		t.Fatalf("应为 ErrReplayMiss，实得 %v", err)
	}
	var me *MissError
	if !errors.As(err, &me) {
		t.Fatalf("应为 *MissError，实得 %T", err)
	}
	// 指纹：独立复算，钉住口径
	call := mustNormalize(t, norm, "grep", missArgs)
	if want := indepFingerprint("grep", call.Canonical); me.Fingerprint != want {
		t.Errorf("错误里的 fingerprint 应等于独立复算值：%s vs %s", me.Fingerprint, want)
	}
	if !strings.HasPrefix(me.Fingerprint, "sha256:") || len(me.Fingerprint) != len("sha256:")+64 {
		t.Errorf("fingerprint 形态不对：%q", me.Fingerprint)
	}
	// 最近邻：同工具两条候选，pattern 相同、path 不同 ⇒ 得分 0.5，且指出差异字段
	if me.Scanned != 2 {
		t.Errorf("同工具候选数应为 2，实得 %d", me.Scanned)
	}
	if len(me.Nearest) == 0 {
		t.Fatal("未命中必须给出最近邻候选（没有候选就没法判\"差在哪\"）")
	}
	nb := me.Nearest[0]
	if nb.Seq != 1 {
		t.Errorf("最近邻应是与本次最像的 seq=1（pattern 相同 ⇒ 0.5 > 0），实得 seq=%d", nb.Seq)
	}
	if nb.Score != 0.5 {
		t.Errorf("相同字段占比应为 0.5（pattern 同、path 异），实得 %v", nb.Score)
	}
	if !strings.Contains(strings.Join(nb.Differs, ","), "path") {
		t.Errorf("差异字段应包含 path，实得 %v", nb.Differs)
	}
	// 错误文本必须具备可查性：fingerprint + 最近邻 + 差异字段
	msg := err.Error()
	for _, want := range []string{
		"未命中录播", me.Fingerprint, "#1", "seq=1", nb.Fingerprint, "差异字段=path", "相同 50%",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误文本缺 %q：\n%s", want, msg)
		}
	}
	// 反例：trace 里连这个工具都没有 ⇒ 仍然报错（不是"没有候选就放行"）
	_, err = disp.Execute(ctx, "bash", map[string]any{"command": "ls"})
	if !errors.Is(err, ErrReplayMiss) {
		t.Fatalf("trace 内无该工具时也必须报错，实得 %v", err)
	}
	var me2 *MissError
	if !errors.As(err, &me2) {
		t.Fatalf("应为 *MissError，实得 %T", err)
	}
	if me2.Scanned != 0 || len(me2.Nearest) != 0 {
		t.Errorf("该工具无录制时 Scanned=0 且无候选，实得 Scanned=%d 候选=%d", me2.Scanned, len(me2.Nearest))
	}
	if !strings.Contains(me2.Error(), `没有任何工具 "bash" 的录制`) {
		t.Errorf("错误应点明\"trace 内没有这个工具的录制\"：%s", me2.Error())
	}
	// 反例：命中项必须真的返回正文（证明上面不是"全都未命中"的假绿）
	hitRes, err := disp.Execute(ctx, "grep", seeded[0])
	if err != nil {
		t.Fatalf("录播里存在的调用应命中：%v", err)
	}
	if string(hitRes.Body) != "录制正文 0" {
		t.Errorf("命中正文应为\"录制正文 0\"，实得 %q", hitRes.Body)
	}
}

// ── ③ 回放不真发（真文件 + 真 sha256 + 真计数）─────────────────────────────

func TestReplayNeverExecutesLive(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")
	sideEffect := filepath.Join(dir, "副作用.txt")

	tool := &fakeTool{path: sideEffect, body: []byte("done")}
	ledger := NewEffectLedger()
	scopeOf := func(call Call, args map[string]any) []string {
		return []string{"fs:" + string(call.Canonical)}
	}
	rec := &Dispatcher{
		Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, tracePath), Live: tool.hook(),
		Effects: ledger, EffectScope: scopeOf,
	}
	args := map[string]any{"command": "touch 副作用.txt"}
	if _, err := rec.Execute(ctx, "bash", args); err != nil {
		t.Fatalf("录制期执行：%v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("录制期应真发 1 次，实得 %d", tool.calls)
	}
	rec.Close()
	if _, err := os.Stat(sideEffect); err != nil {
		t.Fatalf("录制期应真的写了副作用文件：%v", err)
	}
	fileBefore := fileHash(t, sideEffect)
	traceBefore := fileHash(t, tracePath)
	ledgerBefore := ledger.Len()

	// 回放：**把同一个会真写文件的 hook 装上**（装了也不许被调用）
	disp := &Dispatcher{
		Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, tracePath, norm, 0),
		Live: tool.hook(), Effects: ledger, EffectScope: scopeOf,
	}
	res, err := disp.Execute(ctx, "bash", args)
	if err != nil {
		t.Fatalf("回放应命中：%v", err)
	}
	if res.Source != SourceRecorded || string(res.Body) != "done" {
		t.Errorf("回放结果应来自录播，实得 source=%s body=%q", res.Source, res.Body)
	}
	if tool.calls != 1 {
		t.Errorf("回放期**真发**次数应保持 1（回放绝不真发），实得 %d", tool.calls)
	}
	if got := fileHash(t, sideEffect); got != fileBefore {
		t.Errorf("回放期副作用文件被改了：%s → %s", fileBefore, got)
	}
	if got := fileHash(t, tracePath); got != traceBefore {
		t.Errorf("回放期 trace 被改了（回放只读）：%s → %s", traceBefore, got)
	}
	if ledger.Len() != ledgerBefore {
		t.Errorf("回放期效果账本被写了（回放只读）：%d → %d", ledgerBefore, ledger.Len())
	}
	// 回放期账本**只读报告**：录播里的效果已被（此前）执行过，这里应报告出来而不是认领
	if len(res.Deduped) == 0 {
		t.Error("回放期应报告\"该效果此前已发生\"（只读账本），实得空")
	}
	// 反例（对侧）：真发模式下同一个 hook 必须真的执行——否则"没被调用"可能只是因为 hook 本身没接上
	live := &Dispatcher{Mode: ModeLive, Norm: norm, Live: tool.hook()}
	if _, err := live.Execute(ctx, "bash", args); err != nil {
		t.Fatalf("live 模式：%v", err)
	}
	if tool.calls != 2 {
		t.Errorf("live 模式必须真发（对照组），实得 %d", tool.calls)
	}
	if got := fileHash(t, sideEffect); got == fileBefore {
		t.Error("live 模式必须真的改文件（对照组）")
	}
	// 反例：未知模式不得静默按真发跑
	bogus := &Dispatcher{Mode: Mode("replay "), Norm: norm, Live: tool.hook()}
	if _, err := bogus.Execute(ctx, "bash", args); err == nil {
		t.Error("未知模式必须报错，不得静默放行")
	}
	// 反例：回放模式缺 Player ⇒ 报错（不得退化成真发）
	noPlayer := &Dispatcher{Mode: ModeReplay, Norm: norm, Live: tool.hook()}
	if _, err := noPlayer.Execute(ctx, "bash", args); !errors.Is(err, ErrReplayMiss) {
		t.Errorf("回放模式缺 Player 应报 ErrReplayMiss，实得 %v", err)
	}
	if tool.calls != 2 {
		t.Errorf("缺 Player 的回放不得调用真发，实得 %d 次", tool.calls)
	}
}

// ── ④ 效果级幂等：同一 EffectKey 第二次出现 ⇒ 已发生、不执行 ────────────────

func TestEffectLedger_SameEffectExecutedOnce(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")
	tool := &fakeTool{body: []byte("ok")}
	ledger := NewEffectLedger()
	scopeOf := func(call Call, args map[string]any) []string { return []string{"fs:" + string(call.Canonical)} }

	d := &Dispatcher{
		Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, tracePath), Live: tool.hook(),
		Effects: ledger, EffectScope: scopeOf,
	}
	args := map[string]any{"command": "echo ok > out.txt"}
	first, err := d.Execute(ctx, "bash", args)
	if err != nil {
		t.Fatalf("第一次执行：%v", err)
	}
	if first.Source != SourceLive || tool.calls != 1 {
		t.Fatalf("第一次应真发（source=live, calls=1），实得 source=%s calls=%d", first.Source, tool.calls)
	}
	call := mustNormalize(t, norm, "bash", args)
	wantKey := indepEffectKey("bash", call.Canonical, "fs:"+string(call.Canonical))
	if len(first.Effects) != 1 || first.Effects[0].Key != wantKey {
		t.Fatalf("效果键口径不符：实得 %+v，独立复算 %s", first.Effects, wantKey)
	}

	// 第二次同样的调用 ⇒ 返回"已发生"，**不再执行**
	second, err := d.Execute(ctx, "bash", args)
	if err != nil {
		t.Fatalf("第二次执行不该报错（去重是恢复路径的正常分支）：%v", err)
	}
	if second.Source != SourceDeduped {
		t.Errorf("第二次应 source=deduped，实得 %s", second.Source)
	}
	if tool.calls != 1 {
		t.Errorf("同一效果第二次出现不得重放：真发次数应保持 1，实得 %d", tool.calls)
	}
	if len(second.Deduped) != 1 || second.Deduped[0].Key != wantKey {
		t.Errorf("应报告已发生的效果（含 keyscope），实得 %+v", second.Deduped)
	}
	if ledger.Len() != 1 {
		t.Errorf("账本应只有 1 条，实得 %d", ledger.Len())
	}
	// 反例：换一个语义不同的调用（不同效果范围）⇒ 必须真发（不是"一刀切全拦"）
	other, err := d.Execute(ctx, "bash", map[string]any{"command": "echo other > other.txt"})
	if err != nil {
		t.Fatalf("不同调用：%v", err)
	}
	if other.Source != SourceLive || tool.calls != 2 {
		t.Errorf("不同效果范围必须真发，实得 source=%s calls=%d", other.Source, tool.calls)
	}
	// 反例：EffectScope 未声明（nil）⇒ 本包不判重也不假设无副作用
	noScope := &Dispatcher{
		Mode: ModeLive, Norm: norm, Live: tool.hook(), Effects: ledger,
	}
	for i := 0; i < 2; i++ {
		if _, err := noScope.Execute(ctx, "bash", args); err != nil {
			t.Fatalf("未声明效果范围：%v", err)
		}
	}
	if tool.calls != 4 {
		t.Errorf("未声明效果范围时两次调用都应真发（calls=2→4），实得 %d", tool.calls)
	}
	d.Close()

	// 可序列化：账本落盘后再判重，结论必须一致
	blob, err := ledger.MarshalLedger()
	if err != nil {
		t.Fatalf("MarshalLedger：%v", err)
	}
	blob2, err := ledger.MarshalLedger()
	if err != nil {
		t.Fatalf("MarshalLedger：%v", err)
	}
	if !bytes.Equal(blob, blob2) {
		t.Error("账本序列化必须是稳定序（同样内容 ⇒ 同样字节）")
	}
	loaded, err := LoadEffectLedger(blob)
	if err != nil {
		t.Fatalf("LoadEffectLedger：%v", err)
	}
	if claim, entry := loaded.Claim(EffectEntry{Key: wantKey, Tool: "bash", Scope: "fs:x"}); claim != EffectAlreadyApplied {
		t.Errorf("落盘再读后同一 key 仍应判为已发生，实得 %s", claim)
	} else if entry.Scope != "fs:"+string(call.Canonical) {
		t.Errorf("已发生条目应返回**首次**那条（不被后到者覆盖），实得 %+v", entry)
	}
	// 反例：坏账本必须报错（账本坏掉 = 幂等判断全错）
	if _, err := LoadEffectLedger([]byte(`{"v":99,"effects":[]}`)); err == nil {
		t.Error("版本不认识的账本应报错")
	}
	if _, err := LoadEffectLedger([]byte(`{"v":1,"effects":[{"key":"k"},{"key":"k"}]}`)); err == nil {
		t.Error("重复 key 的账本应报错")
	}
	if _, err := LoadEffectLedger([]byte(`{`)); err == nil {
		t.Error("非法 JSON 的账本应报错")
	}
}

// ── ⑤ 一次调用多效果：逐条判重（(call_id, effect_index)）────────────────────

func TestEffectLedger_MultiEffectPerCall(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")
	tool := &fakeTool{body: []byte("ok")}
	ledger := NewEffectLedger()
	// 一次调用三个效果（如 mkdir + 写文件 + chown）
	scopes := []string{"fs:mkdir wd", "fs:write wd/a.txt", "proc:chown wd"}
	d := &Dispatcher{
		Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, tracePath), Live: tool.hook(),
		Effects: ledger, EffectScope: func(call Call, args map[string]any) []string { return scopes },
	}
	args := map[string]any{"call_id": "c1", "command": "mkdir wd && echo a > wd/a.txt && chown me wd"}
	if _, err := d.Execute(ctx, "bash", args); err != nil {
		t.Fatalf("第一次执行：%v", err)
	}
	if tool.calls != 1 || ledger.Len() != 3 {
		t.Fatalf("一次调用三个效果：应真发 1 次、账本 3 条，实得 calls=%d 账本=%d", tool.calls, ledger.Len())
	}
	call := mustNormalize(t, norm, "bash", args)
	for i, s := range scopes {
		wantKey := indepEffectKey("bash", call.Canonical, s)
		e, ok := ledger.AppliedIndex("c1", i)
		if !ok {
			t.Fatalf("应能按 (call_id, effect_index) 查到第 %d 条效果", i)
		}
		if e.Key != wantKey || e.Scope != s {
			t.Errorf("第 %d 条效果不符：%+v，期望 key=%s scope=%s", i, e, wantKey, s)
		}
		if e.Index != i {
			t.Errorf("第 %d 条效果的 effect_index 应为 %d，实得 %d", i, i, e.Index)
		}
	}
	// 三个效果的 key 必须互不相同（否则"逐条"就退化成"一条"）
	seen := map[string]bool{}
	for _, e := range ledger.Snapshot() {
		if seen[e.Key] {
			t.Errorf("效果键重复：%s", e.Key)
		}
		seen[e.Key] = true
	}
	// 整调用重来 ⇒ 整调用不执行（先认领后执行：任一已发生即不真发）
	again, err := d.Execute(ctx, "bash", args)
	if err != nil {
		t.Fatalf("重来：%v", err)
	}
	if again.Source != SourceDeduped || tool.calls != 1 {
		t.Errorf("整调用重来应判为已发生且不真发，实得 source=%s calls=%d", again.Source, tool.calls)
	}
	if ledger.Len() != 3 {
		t.Errorf("判为已发生时不得再认领其余效果，账本应仍为 3 条，实得 %d", ledger.Len())
	}

	// **逐条**判重：模拟"上次恢复只做到第 0 个效果就中断"
	freshTool := &fakeTool{body: []byte("ok")}
	freshLedger := NewEffectLedger()
	fresh := &Dispatcher{
		Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, filepath.Join(dir, "t2.jsonl")),
		Live: freshTool.hook(), Effects: freshLedger,
		EffectScope: func(call Call, args map[string]any) []string { return scopes },
	}
	key0 := indepEffectKey("bash", call.Canonical, scopes[0])
	if claim, _ := freshLedger.Claim(EffectEntry{Key: key0, Tool: "bash", Scope: scopes[0], CallID: "c1", Index: 0}); claim != EffectNew {
		t.Fatalf("手工认领第 0 个效果应为 new，实得 %s", claim)
	}
	res, err := fresh.Execute(ctx, "bash", args)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if res.Source != SourceDeduped {
		t.Errorf("只要有一个效果已发生，整调用就不许执行（实得 source=%s）", res.Source)
	}
	if freshTool.calls != 0 {
		t.Errorf("整调用不许真发，实得 %d 次", freshTool.calls)
	}
	if len(res.Deduped) != 1 || res.Deduped[0].Index != 0 || res.Deduped[0].Scope != scopes[0] {
		t.Errorf("应精确指出是哪一条效果已发生（第 0 条），实得 %+v", res.Deduped)
	}
	if freshLedger.Len() != 1 {
		t.Errorf("未执行的效果不得被认领，账本应仍为 1 条，实得 %d", freshLedger.Len())
	}
	// 去重域跨调用：同样效果换个 call_id 仍是同一效果 ⇒ 仍判已发生
	otherCall := map[string]any{"call_id": "c2", "command": args["command"]}
	if _, err := fresh.Execute(ctx, "bash", otherCall); err != nil {
		t.Fatalf("%v", err)
	}
	if freshTool.calls != 0 {
		t.Errorf("效果键与 call_id 无关（跨调用同一效果也去重），不许真发，实得 %d 次", freshTool.calls)
	}
}

// ── ⑥ 消费游标：重启后续读，不从头重放 ─────────────────────────────────────

func TestCursorResume_AfterRestart(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	tracePath := filepath.Join(dir, "trace.jsonl")
	cursorPath := filepath.Join(dir, "cursor.json")
	tr := mustCreate(t, tracePath)
	argsList := []map[string]any{
		{"path": "a.txt"}, {"path": "b.txt"}, {"path": "c.txt"},
	}
	for i, a := range argsList {
		c := mustNormalize(t, norm, "read", a)
		if _, err := tr.Append(Record{
			Tool: "read", ArgsCanonical: c.Canonical, ArgsDigest: c.ArgsDigest,
			ResultBody: []byte(fmt.Sprintf("正文 %d", i)),
		}); err != nil {
			t.Fatalf("Append：%v", err)
		}
	}
	tr.Close()

	// 第一次运行：只消费前 2 条，然后落游标（模拟进程退出）
	r, err := OpenReader(tracePath, 0)
	if err != nil {
		t.Fatalf("OpenReader：%v", err)
	}
	for i := 0; i < 2; i++ {
		rec, err := r.Next()
		if err != nil {
			t.Fatalf("第 %d 条：%v", i+1, err)
		}
		if rec.Seq != i+1 {
			t.Fatalf("顺序读应给出 seq=%d，实得 %d", i+1, rec.Seq)
		}
	}
	if r.Consumed() != 2 {
		t.Fatalf("消费条数应为 2，实得 %d", r.Consumed())
	}
	cur := r.Cursor()
	if cur.Consumed != 2 || cur.PrefixDigest == "" {
		t.Fatalf("游标应带消费条数与前缀指纹，实得 %+v", cur)
	}
	r.Close()
	if err := SaveCursor(cursorPath, cur); err != nil {
		t.Fatalf("SaveCursor：%v", err)
	}

	// —— 重启 ——
	loaded, found, err := LoadCursor(cursorPath)
	if err != nil || !found {
		t.Fatalf("LoadCursor：found=%v err=%v", found, err)
	}
	if loaded.Consumed != 2 {
		t.Fatalf("重启后游标应为 2，实得 %d", loaded.Consumed)
	}
	r2, err := OpenReaderWithCursor(tracePath, loaded)
	if err != nil {
		t.Fatalf("OpenReaderWithCursor：%v", err)
	}
	rec, err := r2.Next()
	if err != nil {
		t.Fatalf("续读：%v", err)
	}
	if rec.Seq != 3 {
		t.Fatalf("重启后续读的第一条应是 seq=3（**不从第 1 条重放**），实得 seq=%d", rec.Seq)
	}
	if _, err := r2.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("第 3 条之后应到末尾，实得 %v", err)
	}
	if r2.Consumed() != 3 {
		t.Errorf("续读到底后游标应为 3，实得 %d", r2.Consumed())
	}
	final := r2.Cursor()
	r2.Close()
	if err := SaveCursor(cursorPath, final); err != nil {
		t.Fatalf("SaveCursor：%v", err)
	}
	if again, _, _ := LoadCursor(cursorPath); again.Consumed != 3 {
		t.Errorf("游标应推进到 3，实得 %d", again.Consumed)
	}

	// 回放器也按游标续读：已在游标之前的事实**不进**回放器 ⇒ 不会从头重放
	p, newCur, err := PlayerFromTrace(tracePath, norm, 2)
	if err != nil {
		t.Fatalf("PlayerFromTrace：%v", err)
	}
	if p.Len() != 1 {
		t.Fatalf("从游标 2 续读应只载入 1 条，实得 %d", p.Len())
	}
	if newCur.Consumed != 3 {
		t.Errorf("PlayerFromTrace 应返回新游标 3，实得 %d", newCur.Consumed)
	}
	if _, err := p.Lookup(mustNormalize(t, norm, "read", argsList[2])); err != nil {
		t.Errorf("游标之后的事实应可命中：%v", err)
	}
	if _, err := p.Lookup(mustNormalize(t, norm, "read", argsList[0])); !errors.Is(err, ErrReplayMiss) {
		t.Errorf("游标**之前**的事实不该被重放（应未命中），实得 %v", err)
	}
	// 反例：trace 被重录（前缀变了）⇒ 游标必须报错，不得错位续读
	if _, err := OpenReaderWithCursor(tracePath, Cursor{
		TracePath: tracePath, Consumed: 2, PrefixDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}); !errors.Is(err, ErrBadCursor) {
		t.Errorf("前缀指纹不符应报 ErrBadCursor，实得 %v", err)
	}
	// 反例：游标超出 trace 末尾 ⇒ 报错
	if _, err := OpenReader(tracePath, 9); !errors.Is(err, ErrBadCursor) {
		t.Errorf("游标越界应报 ErrBadCursor，实得 %v", err)
	}
	// 正例：游标 0 时前缀指纹为空，不带指纹的旧游标也应能续读（Consumed 仍有约束）
	if _, err := OpenReaderWithCursor(tracePath, Cursor{Consumed: 1}); err != nil {
		t.Errorf("没有前缀指纹的游标应能续读（只用消费条数）：%v", err)
	}
}

// ── ⑦ 前置状态不符 ⇒ 报"状态不匹配"而非默默回放 ───────────────────────────

func TestPreconditionMismatch_ReportsStateMismatch(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}

	// 场景 A：录制时记了前置状态
	tracePath := filepath.Join(dir, "a.jsonl")
	state := "state-v1"
	tool := &fakeTool{body: []byte("ok")}
	d := &Dispatcher{
		Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, tracePath), Live: tool.hook(),
		StateHash: func() string { return state },
	}
	args := map[string]any{"path": "a.txt", "content": "x"}
	if _, err := d.Execute(ctx, "write", args); err != nil {
		t.Fatalf("录制：%v", err)
	}
	d.Close()
	recs, err := readAll(tracePath)
	if err != nil {
		t.Fatalf("readAll：%v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("应有 1 条录播，实得 %d", len(recs))
	}
	if recs[0].PreconditionHash != "state-v1" || recs[0].ObservedAfterHash != "state-v1" {
		t.Errorf("E5 要求前置与事后状态都落盘：实得 pre=%q after=%q",
			recs[0].PreconditionHash, recs[0].ObservedAfterHash)
	}

	// 正例：状态一致 ⇒ 回放命中
	state = "state-v1"
	ok := &Dispatcher{
		Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, tracePath, norm, 0),
		VerifyPrecondition: true, StateHash: func() string { return state },
	}
	if res, err := ok.Execute(ctx, "write", args); err != nil || res.Source != SourceRecorded {
		t.Fatalf("状态一致时应回放命中：res=%+v err=%v", res, err)
	}
	// 反例：状态不符 ⇒ 必须报"状态不匹配"，不得默默回放
	state = "state-v2"
	bad := &Dispatcher{
		Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, tracePath, norm, 0),
		VerifyPrecondition: true, StateHash: func() string { return state },
	}
	_, err = bad.Execute(ctx, "write", args)
	if err == nil {
		t.Fatal("前置状态不符必须报错——否则回放会悄悄跑在错误的状态上")
	}
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("应为 ErrStateMismatch，实得 %v", err)
	}
	var sm *StateMismatchError
	if !errors.As(err, &sm) {
		t.Fatalf("应为 *StateMismatchError，实得 %T", err)
	}
	if sm.Want != "state-v1" || sm.Got != "state-v2" || sm.Seq != 1 {
		t.Errorf("错误里应带两侧状态与命中序号：%+v", sm)
	}
	for _, want := range []string{"状态不匹配", "state-v1", "state-v2", "seq=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误文本缺 %q：%s", want, err.Error())
		}
	}
	// 反例：调用方拿不出当前状态 ⇒ fail-closed（"无法校验"不等于"校验通过"）
	noState := &Dispatcher{
		Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, tracePath, norm, 0),
		VerifyPrecondition: true,
	}
	if _, err := noState.Execute(ctx, "write", args); !errors.Is(err, ErrStateMismatch) {
		t.Errorf("取不到当前状态应报不匹配（fail-closed），实得 %v", err)
	}
	// 反例：录制时**没记**前置 ⇒ 这条 trace 支撑不了这次校验，也要报错
	traceB := filepath.Join(dir, "b.jsonl")
	dB := &Dispatcher{Mode: ModeRecord, Norm: norm, Trace: mustCreate(t, traceB), Live: tool.hook()}
	if _, err := dB.Execute(ctx, "write", args); err != nil {
		t.Fatalf("录制（不记前置）：%v", err)
	}
	dB.Close()
	verifyB := &Dispatcher{
		Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, traceB, norm, 0),
		VerifyPrecondition: true, StateHash: func() string { return "state-v1" },
	}
	_, err = verifyB.Execute(ctx, "write", args)
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("录制未记前置时应报不匹配，实得 %v", err)
	}
	if !strings.Contains(err.Error(), "未记前置状态") {
		t.Errorf("错误应点明\"录制时未记前置状态\"：%s", err.Error())
	}
	// 反例：**不**开启校验时，老 trace 必须照常可回放（不能把没开校验的路径也搞红）
	noVerify := &Dispatcher{Mode: ModeReplay, Norm: norm, Player: mustPlayer(t, traceB, norm, 0)}
	if _, err := noVerify.Execute(ctx, "write", args); err != nil {
		t.Errorf("未开启前置校验时应照常回放：%v", err)
	}
}

// ── ⑧ trace 损坏/截断 ⇒ 明确报错（不得静默跳过）───────────────────────────

func TestTraceCorrupt_ErrorsLoudlyNeverSkips(t *testing.T) {
	dir := t.TempDir()
	norm := Normalizer{BaseDir: dir}
	good := filepath.Join(dir, "good.jsonl")
	tr := mustCreate(t, good)
	for i := 0; i < 3; i++ {
		c := mustNormalize(t, norm, "read", map[string]any{"path": fmt.Sprintf("%c.txt", 'a'+i)})
		if _, err := tr.Append(Record{
			Tool: "read", ArgsCanonical: c.Canonical, ArgsDigest: c.ArgsDigest,
			ResultBody: []byte(fmt.Sprintf("正文 %d", i)),
		}); err != nil {
			t.Fatalf("Append：%v", err)
		}
	}
	tr.Close()
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatalf("读：%v", err)
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if len(lines) != 4 { // 3 条 + 末尾空串
		t.Fatalf("应有 3 行，实得 %d", len(lines)-1)
	}
	body := func(i int) string { return lines[i] }

	// 每个变体：坏行之前的记录照常给，坏行处**报错**，坏行之后的一条都不给
	cases := []struct {
		name      string
		content   string
		wantLine  int
		wantInMsg string
	}{
		{
			name:      "末行被截断（无换行）",
			content:   body(0) + body(1) + body(2)[:len(body(2))-8],
			wantLine:  3,
			wantInMsg: "末行没有换行符",
		},
		{
			name:      "中间行是垃圾",
			content:   body(0) + "这不是 JSON\n" + body(1) + body(2),
			wantLine:  2,
			wantInMsg: "不是合法 JSON",
		},
		{
			name:      "中间行是空行",
			content:   body(0) + "\n" + body(1) + body(2),
			wantLine:  2,
			wantInMsg: "空行",
		},
		{
			name:      "seq 断层",
			content:   body(0) + strings.Replace(body(1), `"seq":2`, `"seq":7`, 1) + body(2),
			wantLine:  2,
			wantInMsg: "序号断层",
		},
		{
			name:      "版本不认识",
			content:   strings.Replace(body(0), `"v":1`, `"v":99`, 1) + body(1) + body(2),
			wantLine:  1,
			wantInMsg: "高于本包已知",
		},
		{
			name:      "正文指纹被篡改",
			content:   strings.Replace(body(0), `"result_digest":"sha256:`, `"result_digest":"sha256:00`, 1) + body(1) + body(2),
			wantLine:  1,
			wantInMsg: "result_digest 复算不符",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, "bad.jsonl")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("写：%v", err)
			}
			r, err := OpenReader(p, 0)
			if err != nil {
				t.Fatalf("OpenReader：%v", err)
			}
			defer r.Close()
			got := 0
			var readErr error
			for {
				_, err := r.Next()
				if err == nil {
					got++
					if got > len(lines) {
						t.Fatalf("读到超过文件条数（%d）的记录，说明校验形同虚设", got)
					}
					continue
				}
				readErr = err
				break
			}
			if readErr == nil || errors.Is(readErr, io.EOF) {
				t.Fatalf("损坏的 trace 必须报错，实得 %v（静默跳过是绝不允许的）", readErr)
			}
			if !errors.Is(readErr, ErrTraceCorrupt) {
				t.Fatalf("应为 ErrTraceCorrupt，实得 %v", readErr)
			}
			if want := tc.wantLine - 1; got != want {
				t.Errorf("坏行之前的记录数应为 %d（坏行之后一条都不给），实得 %d", want, got)
			}
			if !strings.Contains(readErr.Error(), fmt.Sprintf("第 %d 行", tc.wantLine)) {
				t.Errorf("错误应指出行号（第 %d 行）：%s", tc.wantLine, readErr.Error())
			}
			if !strings.Contains(readErr.Error(), tc.wantInMsg) {
				t.Errorf("错误应含 %q：%s", tc.wantInMsg, readErr.Error())
			}
			// 回放器载入必须整体失败（不给"半个回放器"）
			if _, _, err := PlayerFromTrace(p, norm, 0); err == nil {
				t.Error("损坏的 trace 不得载入回放器（半个回放器会让一半调用被当成未命中）")
			}
			// 续写也必须拒绝（不许把新行续在坏文件后面）
			if _, err := OpenAppend(p); err == nil {
				t.Error("损坏的 trace 不得续写")
			}
		})
	}

	t.Run("显式声明才容忍末行截断", func(t *testing.T) {
		// 末行是**合法 JSON** 但没有换行（写入被中断）⇒ 默认报错，显式声明后才接受
		p := filepath.Join(dir, "tail2.jsonl")
		if err := os.WriteFile(p, []byte(body(0)+body(1)+strings.TrimSuffix(body(2), "\n")), 0o644); err != nil {
			t.Fatalf("写：%v", err)
		}
		r, err := OpenReader(p, 0)
		if err != nil {
			t.Fatalf("OpenReader：%v", err)
		}
		seen := 0
		var got error
		for {
			_, err := r.Next()
			if err != nil {
				got = err
				break
			}
			seen++
		}
		r.Close()
		if !errors.Is(got, ErrTraceCorrupt) {
			t.Errorf("默认必须对末行截断报错（不得静默跳过），实得 %v", got)
		}
		if !strings.Contains(got.Error(), "末行没有换行符") {
			t.Errorf("错误应点明末行缺换行：%s", got.Error())
		}
		if seen != 2 {
			t.Errorf("报错前应给出 2 条，实得 %d", seen)
		}

		// 显式容忍 ⇒ 3 条全读到
		r2, err := OpenReader(p, 0)
		if err != nil {
			t.Fatalf("OpenReader：%v", err)
		}
		r2.TolerateTruncatedTail(true)
		n := 0
		for {
			_, err := r2.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("显式容忍后末行应可读：%v", err)
			}
			n++
		}
		r2.Close()
		if n != 3 {
			t.Errorf("显式容忍后应读到 3 条，实得 %d", n)
		}
	})

	t.Run("容忍模式不是跳过坏行的后门", func(t *testing.T) {
		q := filepath.Join(dir, "mid.jsonl")
		if err := os.WriteFile(q, []byte(body(0)+"垃圾\n"+body(1)), 0o644); err != nil {
			t.Fatalf("写：%v", err)
		}
		r2, err := OpenReader(q, 0)
		if err != nil {
			t.Fatalf("OpenReader：%v", err)
		}
		r2.TolerateTruncatedTail(true)
		defer r2.Close()
		seen := 0
		var got error
		for {
			_, err := r2.Next()
			if err != nil {
				got = err
				break
			}
			seen++
		}
		if !errors.Is(got, ErrTraceCorrupt) {
			t.Errorf("中间行垃圾即使在容忍模式下也必须报错，实得 %v", got)
		}
		if seen != 1 {
			t.Errorf("容忍模式下也只应给出坏行之前的 1 条，实得 %d", seen)
		}
	})

	t.Run("末尾空行也是损坏", func(t *testing.T) {
		p := filepath.Join(dir, "trailing.jsonl")
		if err := os.WriteFile(p, []byte(body(0)+body(1)+body(2)+"\n"), 0o644); err != nil {
			t.Fatalf("写：%v", err)
		}
		r, err := OpenReader(p, 0)
		if err != nil {
			t.Fatalf("OpenReader：%v", err)
		}
		defer r.Close()
		n := 0
		var got error
		for {
			_, err := r.Next()
			if err != nil {
				got = err
				break
			}
			n++
		}
		if !errors.Is(got, ErrTraceCorrupt) {
			t.Errorf("多出来的空行应报错（多一行空行 = 事实数对不上），实得 %v", got)
		}
		if n != 3 {
			t.Errorf("空行前的 3 条应给出，实得 %d", n)
		}
	})
}

// ── 小助手 ──────────────────────────────────────────────────────────────────

func mustCreate(t *testing.T, path string) *Trace {
	t.Helper()
	tr, err := Create(path)
	if err != nil {
		t.Fatalf("Create(%s)：%v", path, err)
	}
	return tr
}

func mustPlayer(t *testing.T, path string, norm Normalizer, from int) *Player {
	t.Helper()
	p, _, err := PlayerFromTrace(path, norm, from)
	if err != nil {
		t.Fatalf("PlayerFromTrace(%s)：%v", path, err)
	}
	return p
}

func readAll(path string) ([]Record, error) {
	r, err := OpenReader(path, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []Record
	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
}

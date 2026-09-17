// normalize_policy_test.go —— T4.2 的实证：**请求指纹与规范化口径**（E3）。
//
//	① TestT42_NoiseOnlyChangeStillHits        只改噪声字段 ⇒ 仍命中；改语义字段 ⇒ 必未命中（含 MatchOn 可配置）
//	② TestT42_CanonicalRulesPinned            口径逐条钉死：键排序 / 数字统一 / 精度截断 / 路径绝对化 /
//	                                          空白折叠 / 缺省-vs-显式 null（写明选择：默认**不等价**）
//	③ TestT42_MatchOnConfigurableAndNoiseRejected  MatchOn 可配置 + 噪声字段进匹配集 ⇒ 当场报错（反例）
//
// 纪律：每条都配**对侧**断言 —— "什么都命中"（口径太松）与"什么都不命中"（口径崩了）都是假绿。
package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildStateHash —— 用例里的**状态指纹**：把若干文件的内容按固定次序摘要（不读环境、不看时间）。
func buildStateHash(t *testing.T, paths ...string) string {
	t.Helper()
	var buf bytes.Buffer
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("读 %s：%v", p, err)
		}
		buf.WriteString(p)
		buf.WriteByte(0)
		buf.Write(b)
		buf.WriteByte(0x1f)
	}
	return Digest(buf.Bytes())
}

// ── ① 只改噪声字段仍命中 / 改语义字段必未命中 ──────────────────────────────

func TestT42_NoiseOnlyChangeStillHits(t *testing.T) {
	dir := t.TempDir()
	norm := DefaultNormalizer(dir)
	tracePath := filepath.Join(dir, "t42.jsonl")
	tool := &fakeTool{body: []byte("ok: 只改噪声仍命中")}

	rec, err := NewRecorder(tracePath, norm, tool.hook())
	if err != nil {
		t.Fatalf("NewRecorder：%v", err)
	}
	recordArgs := map[string]any{
		"path": "out/a.txt", "content": "v1",
		"ts": 1, "trace_id": "tr-1", "retry_count": 0, "attempt": 1, "duration_ms": 12,
	}
	if _, err := rec.Execute(ctx, "write", recordArgs); err != nil {
		t.Fatalf("录制：%v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("录制期应真发 1 次，实得 %d", tool.calls)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close：%v", err)
	}

	// 回放：**所有噪声字段都变了**（时间戳 / trace_id / 重试计数 / 耗时），路径写法也不同
	replayArgs := map[string]any{
		"path": "./out/../out/a.txt", "content": "v1",
		"ts": 999, "trace_id": "tr-2", "retry_count": 7, "attempt": 3, "duration_ms": 4096,
	}
	disp, _, err := NewReplayer(tracePath, norm, 0)
	if err != nil {
		t.Fatalf("NewReplayer：%v", err)
	}
	hit, err := disp.Execute(ctx, "write", replayArgs)
	if err != nil {
		t.Fatalf("只改噪声字段应仍命中（这是 E3 存在的全部理由）：%v", err)
	}
	if !bytes.Equal(hit.Body, tool.body) || hit.Source != SourceRecorded {
		t.Errorf("应逐字节回放录制正文且 source=recorded：body=%q source=%s", hit.Body, hit.Source)
	}
	if tool.calls != 1 {
		t.Errorf("回放期不得真发，实得 %d 次", tool.calls)
	}

	// 独立复算：噪声字段确实没进指纹，且指纹口径 = sha256(工具名 ‖ 0x1f ‖ 规范化参数)
	ca := mustNormalize(t, norm, "write", recordArgs)
	cb := mustNormalize(t, norm, "write", replayArgs)
	if !bytes.Equal(ca.Canonical, cb.Canonical) {
		t.Errorf("噪声字段不同 ⇒ 规范化字节应逐字节相同：%s vs %s", ca.Canonical, cb.Canonical)
	}
	if ca.Fingerprint != indepFingerprint("write", ca.Canonical) {
		t.Errorf("指纹口径不符：%s vs 独立复算 %s", ca.Fingerprint, indepFingerprint("write", ca.Canonical))
	}
	for _, noise := range []string{"ts", "trace_id", "retry_count", "attempt", "duration_ms"} {
		if strings.Contains(string(ca.Canonical), `"`+noise+`"`) {
			t.Errorf("噪声字段 %s 不该出现在规范化结果里：%s", noise, ca.Canonical)
		}
	}

	// 反例 1：语义字段 content 变了 ⇒ 必未命中，且错误指向差在哪个字段
	_, err = disp.Execute(ctx, "write", map[string]any{
		"path": "./out/../out/a.txt", "content": "v2",
		"ts": 999, "trace_id": "tr-2", "retry_count": 7, "attempt": 3, "duration_ms": 4096,
	})
	if !errors.Is(err, ErrReplayMiss) {
		t.Fatalf("语义字段变化必须未命中（否则匹配形同没有）：%v", err)
	}
	var me *MissError
	if !errors.As(err, &me) {
		t.Fatalf("未命中应是 *MissError，实得 %T", err)
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("错误里应指出差在 content：%s", err.Error())
	}
	if tool.calls != 1 {
		t.Errorf("未命中不得退化成真发，实得 %d 次", tool.calls)
	}
	// 反例 2：语义字段 path 变了 ⇒ 必未命中
	if _, err := disp.Execute(ctx, "write", map[string]any{"path": "out/b.txt", "content": "v1"}); !errors.Is(err, ErrReplayMiss) {
		t.Errorf("path 变化必须未命中，实得 %v", err)
	}

	// MatchOn **可配置**：收窄到只剩 path ⇒ 单独一对"录制 + 回放"证明 content 不再参与匹配
	traceNarrow := filepath.Join(dir, "t42-narrow.jsonl")
	narrow := Normalizer{BaseDir: dir, MatchOn: []string{"path"}}
	toolN := &fakeTool{body: []byte("ok: 收窄口径")}
	recN, err := NewRecorder(traceNarrow, narrow, toolN.hook())
	if err != nil {
		t.Fatalf("NewRecorder（收窄）：%v", err)
	}
	if _, err := recN.Execute(ctx, "write", recordArgs); err != nil {
		t.Fatalf("录制（收窄）：%v", err)
	}
	recN.Close()
	dispN, _, err := NewReplayer(traceNarrow, narrow, 0)
	if err != nil {
		t.Fatalf("NewReplayer（收窄）：%v", err)
	}
	if _, err := dispN.Execute(ctx, "write", map[string]any{
		"path": "out/a.txt", "content": "完全不同的内容", "ts": 1,
	}); err != nil {
		t.Errorf("MatchOn 收窄到 path 后，content 变化不该导致未命中：%v", err)
	}
	// 反例（对侧）：收窄后 path 仍在匹配集里 —— 改 path 必须未命中，否则是"什么都命中"的假绿
	if _, err := dispN.Execute(ctx, "write", map[string]any{"path": "out/c.txt", "content": "v1"}); !errors.Is(err, ErrReplayMiss) {
		t.Errorf("收窄口径下 path 变化必须未命中，实得 %v", err)
	}
	if norm.PolicyID() == narrow.PolicyID() {
		t.Error("两份口径（默认 / 收窄）不同 ⇒ PolicyID 必须不同")
	}
}

// ── ② 规范化口径逐条钉死（含反例）──────────────────────────────────────────

func TestT42_CanonicalRulesPinned(t *testing.T) {
	dir := t.TempDir()
	norm := DefaultNormalizer(dir)

	t.Run("键排序（两层都按字节序，不依赖 map 迭代序）", func(t *testing.T) {
		a := mustNormalize(t, norm, "t", map[string]any{"b": 2, "a": map[string]any{"y": 2, "x": 1}})
		b := mustNormalize(t, norm, "t", map[string]any{"a": map[string]any{"x": 1, "y": 2}, "b": 2})
		if want := `{"a":{"x":1,"y":2},"b":2}`; string(a.Canonical) != want {
			t.Errorf("键排序口径不符：实得 %s，期望 %s", a.Canonical, want)
		}
		if a.ArgsDigest != b.ArgsDigest {
			t.Errorf("同一逻辑调用（键序不同）应同指纹：%s vs %s", a.ArgsDigest, b.ArgsDigest)
		}
	})

	t.Run("数字统一精度（1 ≡ 1.0 ≡ 1.00 ≡ 1e0）", func(t *testing.T) {
		forms := []any{1, 1.0, json.Number("1.00"), json.Number("1e0"), float32(1)}
		var first Call
		for i, v := range forms {
			c := mustNormalize(t, norm, "t", map[string]any{"n": v})
			if want := `{"n":1}`; string(c.Canonical) != want {
				t.Errorf("第 %d 种写法（%#v）应统一成 %s，实得 %s", i+1, v, want, c.Canonical)
			}
			if i == 0 {
				first = c
				continue
			}
			if c.ArgsDigest != first.ArgsDigest {
				t.Errorf("数字写法不同但同值 ⇒ 应同指纹：%s vs %s", c.ArgsDigest, first.ArgsDigest)
			}
		}
		// 反例：值不同 ⇒ 必不同指纹（防止"数字统一"退化成"数字全丢"）
		if mustNormalize(t, norm, "t", map[string]any{"n": 1}).ArgsDigest ==
			mustNormalize(t, norm, "t", map[string]any{"n": 1.5}).ArgsDigest {
			t.Error("1 与 1.5 必须不同指纹")
		}
		// 反例：NaN / ±Inf 不是合法 JSON ⇒ 报 ErrNormalize（不许偷偷写成 null 把两个事实合一）
		for _, bad := range []any{math.NaN(), math.Inf(1), math.Inf(-1)} {
			if _, err := norm.Normalize("t", map[string]any{"n": bad}); !errors.Is(err, ErrNormalize) {
				t.Errorf("非法数字 %v 应报 ErrNormalize，实得 %v", bad, err)
			}
		}
	})

	t.Run("NumberPrecision 截断（浮点噪声场景）", func(t *testing.T) {
		p3 := Normalizer{BaseDir: dir, NumberPrecision: 3}
		x := mustNormalize(t, p3, "t", map[string]any{"score": 0.12340001})
		y := mustNormalize(t, p3, "t", map[string]any{"score": 0.12344999})
		if x.ArgsDigest != y.ArgsDigest {
			t.Errorf("截断到 3 位后应同指纹：%s vs %s", x.ArgsDigest, y.ArgsDigest)
		}
		// 反例：默认精度（0）下两者必须不同 —— 否则"精度"这个旋钮是假的
		if mustNormalize(t, norm, "t", map[string]any{"score": 0.12340001}).ArgsDigest ==
			mustNormalize(t, norm, "t", map[string]any{"score": 0.12344999}).ArgsDigest {
			t.Error("默认精度下两个不同浮点应不同指纹")
		}
		// 反例：截断不能把"离得远"的数字也合成一个
		z := mustNormalize(t, p3, "t", map[string]any{"score": 0.9})
		if x.ArgsDigest == z.ArgsDigest {
			t.Error("0.1234 与 0.9 截断后仍必须不同指纹")
		}
	})

	t.Run("路径绝对化（Clean + 相对转绝对；换基准 ⇒ 换指纹）", func(t *testing.T) {
		p1 := mustNormalize(t, norm, "t", map[string]any{"path": "a/../a.txt"})
		p2 := mustNormalize(t, norm, "t", map[string]any{"path": "a.txt"})
		if p1.ArgsDigest != p2.ArgsDigest {
			t.Errorf("路径写法不同但同一文件 ⇒ 应同指纹：%s vs %s", p1.ArgsDigest, p2.ArgsDigest)
		}
		if want := `{"path":"` + filepath.Join(dir, "a.txt") + `"}`; string(p1.Canonical) != want {
			t.Errorf("路径应绝对化：实得 %s，期望 %s", p1.Canonical, want)
		}
		// 对侧：换基准目录 ⇒ 同一相对路径落到不同文件 ⇒ 指纹必须变（这正是"录制与回放必须同基准"）
		other := Normalizer{BaseDir: filepath.Join(dir, "elsewhere")}
		if mustNormalize(t, other, "t", map[string]any{"path": "a.txt"}).ArgsDigest == p1.ArgsDigest {
			t.Error("不同基准目录下的同一相对路径不该同指纹（基准是口径的一部分）")
		}
	})

	t.Run("空白折叠（命令行类折叠；内容类默认不折叠）", func(t *testing.T) {
		ws1 := mustNormalize(t, norm, "t", map[string]any{"command": "  echo   a	>  out/b.txt \n"})
		ws2 := mustNormalize(t, norm, "t", map[string]any{"command": "echo a > out/b.txt"})
		// 期望文本用 encoding/json 的转义口径（`>` 会写成 \u003e）—— 字符串转义也是口径的一部分，
		// 故用同一套转义构造期望值，而不是手写裸文本。
		wantText, err := json.Marshal("echo a > out/b.txt")
		if err != nil {
			t.Fatalf("marshal：%v", err)
		}
		if want := `{"command":` + string(wantText) + `}`; string(ws1.Canonical) != want {
			t.Errorf("命令行空白应折叠成单空格并去首尾：实得 %s，期望 %s", ws1.Canonical, want)
		}
		if ws1.ArgsDigest != ws2.ArgsDigest {
			t.Errorf("折叠后应同指纹：%s vs %s", ws1.ArgsDigest, ws2.ArgsDigest)
		}
		// 反例：内容类字段默认**不折叠**（空白是内容的一部分）
		c1 := mustNormalize(t, norm, "t", map[string]any{"content": "a  b"})
		c2 := mustNormalize(t, norm, "t", map[string]any{"content": "a b"})
		if c1.ArgsDigest == c2.ArgsDigest {
			t.Error("content 默认不该折叠空白（折叠会造出「两个不同内容同指纹」的错命中）")
		}
		// 显式声明后折叠 ⇒ 同指纹（旋钮真的可用）
		cw := Normalizer{BaseDir: dir, CollapseWhitespace: []string{"content"}}
		if mustNormalize(t, cw, "t", map[string]any{"content": "a  b"}).ArgsDigest !=
			mustNormalize(t, cw, "t", map[string]any{"content": "a b"}).ArgsDigest {
			t.Error("显式声明 content 折叠后应同指纹")
		}
		// 反例：显式 []string{} ⇒ 连默认表的 command 也不折叠
		nows := Normalizer{BaseDir: dir, CollapseWhitespace: []string{}}
		if mustNormalize(t, nows, "t", map[string]any{"command": "a  b"}).ArgsDigest ==
			mustNormalize(t, nows, "t", map[string]any{"command": "a b"}).ArgsDigest {
			t.Error("CollapseWhitespace 显式给空切片 ⇒ 一律不折叠（非 nil 空切片 ≠ nil）")
		}
	})

	t.Run("缺省 vs 显式 null：默认不等价，显式声明才等价", func(t *testing.T) {
		absent := mustNormalize(t, norm, "t", map[string]any{"path": "a.txt"})
		nulled := mustNormalize(t, norm, "t", map[string]any{"path": "a.txt", "limit": nil})
		if absent.ArgsDigest == nulled.ArgsDigest {
			t.Error("默认口径下「缺省」与「显式 null」必须不同指纹（合并两者 = 放松匹配，猜错方向是错命中）")
		}
		if !strings.Contains(string(nulled.Canonical), `"limit":null`) {
			t.Errorf("显式 null 应原样记成 null：%s", nulled.Canonical)
		}
		// 显式声明该字段可空等价 ⇒ 两种写法同指纹
		eq := Normalizer{BaseDir: dir, NullEqualsAbsent: []string{"limit"}}
		if !bytes.Equal(mustNormalize(t, eq, "t", map[string]any{"path": "a.txt", "limit": nil}).Canonical, absent.Canonical) {
			t.Error("显式声明 NullEqualsAbsent 后，null 与缺省应落到同一份规范化字节")
		}
		// 通配 "*" ⇒ 全部字段都按"可空等价"处理
		all := Normalizer{BaseDir: dir, NullEqualsAbsent: []string{"*"}}
		if !bytes.Equal(mustNormalize(t, all, "t", map[string]any{"path": "a.txt", "limit": nil, "more": nil}).Canonical,
			absent.Canonical) {
			t.Error("NullEqualsAbsent=[\"*\"] 应把全部显式 null 都当成缺省")
		}
		// 口径不同 ⇒ PolicyID 不同（口径是能被读出来、被比对的）
		if norm.PolicyID() == eq.PolicyID() {
			t.Error("NullEqualsAbsent 不同 ⇒ PolicyID 必须不同")
		}
	})

	t.Run("端到端：录制时缺省、回放时写成 null ⇒ 默认未命中；显式声明等价后命中", func(t *testing.T) {
		tracePath := filepath.Join(dir, "null-equiv.jsonl")
		tool := &fakeTool{body: []byte("ok: null 口径")}
		rec, err := NewRecorder(tracePath, norm, tool.hook())
		if err != nil {
			t.Fatalf("NewRecorder：%v", err)
		}
		if _, err := rec.Execute(ctx, "read", map[string]any{"path": "a.txt"}); err != nil {
			t.Fatalf("录制：%v", err)
		}
		rec.Close()
		disp, _, err := NewReplayer(tracePath, norm, 0)
		if err != nil {
			t.Fatalf("NewReplayer：%v", err)
		}
		// 反例：把"没给"写成"给了 null" ⇒ 默认口径下必须未命中（宁可未命中，不可错命中）
		if _, err := disp.Execute(ctx, "read", map[string]any{"path": "a.txt", "limit": nil}); !errors.Is(err, ErrReplayMiss) {
			t.Errorf("默认口径下显式 null 应未命中，实得 %v", err)
		}
		// 对侧：显式声明 limit 可空等价 ⇒ 同一次调用命中（旋钮真的能解决这类假未命中）
		eq := Normalizer{BaseDir: dir, NullEqualsAbsent: []string{"limit"}}
		dispEq, _, err := NewReplayer(tracePath, eq, 0)
		if err != nil {
			t.Fatalf("NewReplayer（等价口径）：%v", err)
		}
		res, err := dispEq.Execute(ctx, "read", map[string]any{"path": "a.txt", "limit": nil})
		if err != nil {
			t.Fatalf("声明等价后应命中：%v", err)
		}
		if res.Source != SourceRecorded || tool.calls != 1 {
			t.Errorf("应回放命中且不真发：source=%s calls=%d", res.Source, tool.calls)
		}
	})
}

// ── ③ MatchOn 可配置 + 噪声字段不许进匹配集（反例）─────────────────────────

func TestT42_MatchOnConfigurableAndNoiseRejected(t *testing.T) {
	dir := t.TempDir()

	// 默认口径：MatchOn 空（= 除 Drop 外全部）；四类噪声字段都在去噪表里
	def := DefaultNormalizer(dir)
	p := def.Policy()
	if len(p.MatchOn) != 0 {
		t.Errorf("默认 MatchOn 应为空（= 除噪声字段外的全部参数），实得 %v —— %v", p.MatchOn, p)
	}
	for _, noise := range []string{"ts", "timestamp", "trace_id", "span_id", "retry_count", "attempt", "duration_ms"} {
		if !contains(p.Drop, noise) {
			t.Errorf("默认去噪表应含噪声字段 %s（否则它会把命中率打到 0）：%v", noise, p.Drop)
		}
	}

	// 可配置：收窄口径与默认口径不是同一份；**集合语义**（次序无关）
	narrow := Normalizer{BaseDir: dir, MatchOn: []string{"path", "limit"}}
	reordered := Normalizer{BaseDir: dir, MatchOn: []string{"limit", "path", "path"}} // 次序不同 + 有重复
	if def.PolicyID() == narrow.PolicyID() {
		t.Error("收窄口径与默认口径的 PolicyID 必须不同")
	}
	if narrow.PolicyID() != reordered.PolicyID() {
		t.Errorf("同一集合（次序/重复不同）应同一 PolicyID：%s vs %s", narrow.PolicyID(), reordered.PolicyID())
	}
	// BaseDir 只进 PolicyID（形状 ID 不含它）—— 跨机比对要用 ShapeID
	here := Normalizer{BaseDir: dir}
	other := Normalizer{BaseDir: "/tmp"}
	if here.PolicyID() == other.PolicyID() {
		t.Error("BaseDir 是口径的一部分 ⇒ PolicyID 必须变")
	}
	if here.PolicyShapeID() != other.PolicyShapeID() {
		t.Error("PolicyShapeID 不含 BaseDir ⇒ 换基准不该变形状 ID")
	}
	// 口径一致（同一份配置两次构造）⇒ ID 逐字节一致（可比对才有意义）
	if def.PolicyID() != DefaultNormalizer(dir).PolicyID() {
		t.Error("同一份口径两次构造应得同一个 PolicyID")
	}

	// 反例：把噪声字段配进 MatchOn ⇒ **当场**报错（不是等回放期大面积未命中）
	for _, field := range []string{"ts", "trace_id", "retry_count", "attempt", "duration_ms", "opts.retry_count"} {
		n := Normalizer{BaseDir: dir, MatchOn: []string{"path", field}}
		if err := n.Validate(); !errors.Is(err, ErrNoiseFieldInMatchOn) {
			t.Errorf("Validate 应拦下 MatchOn 里的 %q，实得 %v", field, err)
		}
		if _, err := n.Normalize("t", map[string]any{"path": "a.txt", "ts": 1}); !errors.Is(err, ErrNoiseFieldInMatchOn) {
			t.Errorf("Normalize 应拦下 MatchOn 里的 %q（fail-fast），实得 %v", field, err)
		}
		if !strings.Contains(n.Validate().Error(), field) {
			t.Errorf("错误里应点名是哪个字段：%v", n.Validate())
		}
	}

	// 逃生舱：显式豁免 ⇒ 放行，且该字段**真的参与匹配**。
	// 两道声明都要显式：① 黑名单豁免（AllowNoiseInMatchOn）② 从默认去噪表里把它去掉（Drop）——
	// 豁免只作用于"匹配集黑名单"，去噪是另一道闸；两道都写下来才叫"这个字段确实有语义"。
	exDrop := make([]string, 0, len(DefaultNoiseFields))
	for _, f := range DefaultNoiseFields {
		if f != "ts" {
			exDrop = append(exDrop, f)
		}
	}
	ex := Normalizer{BaseDir: dir, MatchOn: []string{"path", "ts"}, AllowNoiseInMatchOn: []string{"ts"}, Drop: exDrop}
	if err := ex.Validate(); err != nil {
		t.Fatalf("显式豁免后不该报错：%v", err)
	}
	e1 := mustNormalize(t, ex, "t", map[string]any{"path": "a.txt", "ts": 1})
	e2 := mustNormalize(t, ex, "t", map[string]any{"path": "a.txt", "ts": 2})
	if e1.ArgsDigest == e2.ArgsDigest {
		t.Error("显式豁免的字段必须真的进指纹（否则豁免是假的）")
	}
	// 反例：只写豁免、没把它从 Drop 里去掉 ⇒ Drop 会赢（豁免形同没有）⇒ 报错点明要去掉
	half := Normalizer{BaseDir: dir, MatchOn: []string{"path", "ts"}, AllowNoiseInMatchOn: []string{"ts"}}
	if err := half.Validate(); err == nil || !strings.Contains(err.Error(), "Drop") {
		t.Errorf("豁免了却仍被 Drop 覆盖应报错并点明要去掉：%v", err)
	}

	// 反例：过期豁免（声明了却不在 MatchOn 里）⇒ 报错（豁免表不许腐化）
	stale := Normalizer{BaseDir: dir, MatchOn: []string{"path"}, AllowNoiseInMatchOn: []string{"ts"}}
	if err := stale.Validate(); err == nil || !strings.Contains(err.Error(), "过期豁免") {
		t.Errorf("过期豁免应报错并说清原因，实得 %v", err)
	}
	// 反例：同一字段既 MatchOn 又 Drop ⇒ 二义，报错
	conflict := Normalizer{BaseDir: dir, MatchOn: []string{"path"}, Drop: []string{"path"}}
	if err := conflict.Validate(); err == nil {
		t.Error("MatchOn 与 Drop 打架应报错（Drop 会赢，但不是写它的人的本意）")
	}
	// 反例：空字段名 ⇒ 报错（空名字匹配不到任何字段，只会让人以为收窄了）
	if err := (Normalizer{BaseDir: dir, MatchOn: []string{"  "}}).Validate(); err == nil {
		t.Error("MatchOn 里的空字段名应报错")
	}
	// 反例：口径自检在 Normalize 之前（配错口径的第一条调用就炸，不会静默用错口径匹配）
	if _, err := conflict.Normalize("t", map[string]any{"path": "a.txt"}); err == nil {
		t.Error("Normalize 必须先跑口径自检")
	}
}

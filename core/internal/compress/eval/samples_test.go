package eval

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 样本完整性：任务单 §3.4 的机器判据，逐条落成断言。
// 这是「尺子自身可信」的第一道闸——样本不合格，后面的数字全是幻觉。
func TestSamples_Complete(t *testing.T) {
	all := Samples()

	if len(all) != 10 {
		t.Fatalf("样本数 = %d，本笔要求 10（observation 类）", len(all))
	}

	var (
		zh, en, controls int
		needleTotal      int
		winTotal         int
		recTotal         int
		anchorsTotal     int
	)
	seen := map[string]bool{}
	// 规9：秘密扫描（合成值才允许；命中即构造失败）
	secrets := []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`1[3-9]\d{9}`),
	}

	for _, s := range all {
		if seen[s.ID] {
			t.Errorf("%s：id 重复", s.ID)
		}
		seen[s.ID] = true

		if s.Class != "observation" {
			t.Errorf("%s：class = %q，本笔只造 observation", s.ID, s.Class)
		}
		switch s.Lang {
		case "zh":
			zh++
		case "en":
			en++
		default:
			t.Errorf("%s：lang = %q", s.ID, s.Lang)
		}
		if s.Control {
			controls++
		}

		// 规3：长度断言用 EstimateTokens 实测
		got := s.Tokens()
		if got < s.LenAssert.Min || got > s.LenAssert.Max {
			t.Errorf("%s：%d token 越界 [%d, %d]", s.ID, got, s.LenAssert.Min, s.LenAssert.Max)
		}
		if s.Control && (got < 1800 || got > 2400) {
			t.Errorf("%s：对照样本 %d token，规2 要求 1800–2400", s.ID, got)
		}
		if !s.Control && (got < 4400 || got > 6400) {
			t.Errorf("%s：非对照样本 %d token，规2 要求 4400–6400", s.ID, got)
		}

		// 规4：每样本 5 条 = 2 window + 3 recoverable
		if len(s.Needles) != 5 {
			t.Errorf("%s：needle %d 条，要求 5", s.ID, len(s.Needles))
		}
		if w := s.CountNeedles(WhereWindow); w != 2 {
			t.Errorf("%s：window needle %d 条，要求 2", s.ID, w)
		}
		if r := s.CountNeedles(WhereRecoverable); r != 3 {
			t.Errorf("%s：recoverable needle %d 条，要求 3", s.ID, r)
		}

		// 规5：needle 必须是原文逐字子串（在**原文**里验，不用压缩机验）
		for _, nd := range s.Needles {
			needleTotal++
			if nd.Where == WhereWindow {
				winTotal++
			} else {
				recTotal++
			}
			if nd.Segment < 0 || nd.Segment >= len(s.Segments) {
				t.Errorf("%s/%s：segment 下标 %d 越界", s.ID, nd.ID, nd.Segment)
				continue
			}
			if !strings.Contains(s.Segments[nd.Segment].Text, nd.Text) {
				t.Errorf("%s/%s：needle 不在声明段的原文里：%q", s.ID, nd.ID, nd.Text)
			}
			if nd.Type == "" {
				t.Errorf("%s/%s：type 未填", s.ID, nd.ID)
			}
			// 类型配比（规4 注：每样本至少盖 3 类）
		}
		types := map[string]bool{}
		for _, nd := range s.Needles {
			types[nd.Type] = true
		}
		if len(types) < 3 {
			t.Errorf("%s：needle 类型只盖了 %d 类，要求 ≥3", s.ID, len(types))
		}

		// 规8：可卸载段必须给 raw_ptr，且名字 = sha256(原文)[:16]
		ptrSegs := 0
		for i, sg := range s.Segments {
			if sg.RawPtr == "" {
				continue
			}
			ptrSegs++
			want := "unload/" + textHash(sg.Text) + ".txt"
			if sg.RawPtr != want {
				t.Errorf("%s/seg%d：raw_ptr = %q，应为 %q（内容寻址）", s.ID, i, sg.RawPtr, want)
			}
			if !strings.HasPrefix(sg.RawPtr, "unload/") || !strings.HasSuffix(sg.RawPtr, ".txt") {
				t.Errorf("%s/seg%d：raw_ptr 形态不对：%q", s.ID, i, sg.RawPtr)
			}
		}
		if ptrSegs == 0 {
			t.Errorf("%s：没有任何可回查段（raw_ptr）", s.ID)
		}

		// 规7：observation 只钉错误/失败证据段 —— 钉了的段必须带「失败/panic/error」痕迹
		for i, sg := range s.Segments {
			if !sg.Pinned {
				continue
			}
			low := strings.ToLower(sg.Text)
			if !strings.Contains(low, "fail") && !strings.Contains(low, "panic") && !strings.Contains(low, "exit status") {
				t.Errorf("%s/seg%d：钉了但没有失败证据痕迹", s.ID, i)
			}
		}

		// anchors：每样本 ≥2 条，且每条在原文里出现（guard 的复算面）
		n := 0
		for _, sg := range s.Segments {
			for _, a := range sg.Anchors {
				n++
				anchorsTotal++
				found := false
				for _, other := range s.Segments {
					if strings.Contains(other.Text, a) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s：anchor %q 不在任何段原文里", s.ID, a)
				}
			}
		}
		if n < 2 {
			t.Errorf("%s：anchor 只有 %d 条，要求 ≥2", s.ID, n)
		}

		// 规9
		blob := ""
		for _, sg := range s.Segments {
			blob += sg.Text
		}
		for _, re := range secrets {
			if m := re.FindString(blob); m != "" {
				t.Errorf("%s：命中疑似真秘密 %q（规9 要求明显合成值）", s.ID, m)
			}
		}

		// 探针题面必须带判分口径（本笔不算 DSR，但题面不许是空的）
		if s.Probe.Q == "" || s.Probe.ExpectRegex == "" || s.Probe.Grader != "regex" {
			t.Errorf("%s：probe 不完整", s.ID)
		}
		if _, err := regexp.Compile(s.Probe.ExpectRegex); err != nil {
			t.Errorf("%s：probe.expect_regex 编译不过：%v", s.ID, err)
		}
	}

	if zh != 5 || en != 5 {
		t.Errorf("中英各半：zh=%d en=%d", zh, en)
	}
	if controls != 2 {
		t.Errorf("对照样本 = %d，要求 2（R05/R10）", controls)
	}
	if needleTotal != 50 || winTotal != 20 || recTotal != 30 {
		t.Errorf("needle 合计 = %d（W %d / R %d），要求 50（20/30）", needleTotal, winTotal, recTotal)
	}
	if anchorsTotal < 20 {
		t.Errorf("anchor 合计 = %d，要求 ≥ 10×2", anchorsTotal)
	}
	if !seen["R05"] || !seen["R10"] {
		t.Errorf("对照样本必须是 R05 / R10（规2）")
	}

	// 长度分布打印成人能看的一行（跑批回报要用）
	var lens []string
	for _, s := range all {
		lens = append(lens, fmt.Sprintf("%s=%d", s.ID, s.Tokens()))
	}
	t.Logf("样本 %d 条 · token 分布：%s", len(all), strings.Join(lens, " "))
}

// 确定性：同一二进制两次生成逐字节相同（任务单 §3.4 第 8 条）。
// 生成器里没有 map 迭代、没有 rand、没有时间——这条红了说明有人加了不确定性。
func TestSamples_Deterministic(t *testing.T) {
	a, err := json.Marshal(Samples())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Samples())
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("两次生成不一致：len %d vs %d", len(a), len(b))
	}
	t.Logf("样本字节数 %d · sha256 %s", len(a), sha256Hex(a)[:16])
}

// 尺子不许「靠 Pinned 全塞」蒙混：至少一条非对照样本的 window needle 必须落在**未钉**的段里，
// 否则 PerOutputPct / headroom 两道闸根本没被检验到。
func TestSamples_WindowNeedlesNotAllPinned(t *testing.T) {
	all := Samples()
	unpinned := 0
	for _, s := range all {
		if s.Control {
			continue
		}
		for _, nd := range s.Needles {
			if nd.Where == WhereWindow && !s.Segments[nd.Segment].Pinned {
				unpinned++
			}
		}
	}
	if unpinned < 4 {
		t.Fatalf("非对照样本里落在未钉段的 window needle 只有 %d 条，闸没被真正检验", unpinned)
	}
	t.Logf("未钉段承载的 window needle：%d 条", unpinned)
}

// 编译期确认样本用的 Kind 只在 observation（本笔范围）。
func TestSamples_KindRange(t *testing.T) {
	for _, s := range Samples() {
		for i, sg := range s.Segments {
			if sg.Kind != compress.KindObservation {
				t.Errorf("%s/seg%d：kind=%q，本笔只造 observation", s.ID, i, sg.Kind)
			}
		}
	}
}

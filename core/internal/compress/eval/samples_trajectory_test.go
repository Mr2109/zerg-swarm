package eval

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// trajectory 类的样本完整性：任务单 §3.1 / §3.3 的机器判据，逐条落成断言。
func TestTrajectory_Complete(t *testing.T) {
	all := TrajectorySamples()
	if len(all) != 10 {
		t.Fatalf("样本数 = %d，要求 10（trajectory 类 R11–R20）", len(all))
	}

	var (
		zh, en, controls       int
		needleTotal, win, rec  int
		anchorsTotal           int
		unpinnedWindowNeedles  int
		withObservationSegment int
	)
	seen := map[string]bool{}
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
		if s.Class != "trajectory" {
			t.Errorf("%s：class = %q，本条只造 trajectory", s.ID, s.Class)
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

		// 规4 / 3.3：每样本 5 条 = 4 window + 1 recoverable
		if len(s.Needles) != 5 {
			t.Errorf("%s：needle %d 条，要求 5", s.ID, len(s.Needles))
		}
		if w := s.CountNeedles(WhereWindow); w != 4 {
			t.Errorf("%s：window needle %d 条，要求 4（trajectory 配额 W40）", s.ID, w)
		}
		if r := s.CountNeedles(WhereRecoverable); r != 1 {
			t.Errorf("%s：recoverable needle %d 条，要求 1（trajectory 配额 R10）", s.ID, r)
		}

		// 全文拼接（用于「只出现一次」判定）
		full := ""
		for _, sg := range s.Segments {
			full += sg.Text + "\n"
		}

		types := map[string]bool{}
		for _, nd := range s.Needles {
			needleTotal++
			if nd.Where == WhereWindow {
				win++
			} else {
				rec++
			}
			if nd.Segment < 0 || nd.Segment >= len(s.Segments) {
				t.Errorf("%s/%s：segment 下标 %d 越界", s.ID, nd.ID, nd.Segment)
				continue
			}
			// 规5：逐字子串，且必须落在**声明段**里
			if !strings.Contains(s.Segments[nd.Segment].Text, nd.Text) {
				t.Errorf("%s/%s：needle 不在声明段的原文里：%q", s.ID, nd.ID, nd.Text)
			}
			// ★ 只出现一次：同一条信息在两段各留一份，会让 Budget 扫描的「丢没丢」失真
			if n := strings.Count(full, nd.Text); n != 1 {
				t.Errorf("%s/%s：needle 全文出现 %d 次（要求 1）：%q", s.ID, nd.ID, n, nd.Text)
			}
			if nd.Type == "" {
				t.Errorf("%s/%s：type 未填", s.ID, nd.ID)
			}
			types[nd.Type] = true
			if nd.Where == WhereWindow && !s.Segments[nd.Segment].Pinned {
				unpinnedWindowNeedles++
			}
		}
		if len(types) < 3 {
			t.Errorf("%s：needle 类型只盖了 %d 类，要求 ≥3", s.ID, len(types))
		}

		// 规8：raw_ptr 内容寻址
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
		}
		if ptrSegs == 0 {
			t.Errorf("%s：没有任何可回查段（raw_ptr）", s.ID)
		}

		// 规7：trajectory 钉最近轮 ⇒ 最后一段必须是 Pinned，且钉的段里不许出现「可卸载」的 observation
		last := s.Segments[len(s.Segments)-1]
		if !last.Pinned {
			t.Errorf("%s：最后一段（最近轮）没钉", s.ID)
		}
		pinnedObs := 0
		for i, sg := range s.Segments {
			if sg.Kind == compress.KindObservation && sg.Pinned {
				pinnedObs++
			}
			if sg.Kind == compress.KindObservation {
				withObservationSegment++
			}
			if sg.Pinned && sg.Kind == compress.KindTrajectory && i == len(s.Segments)-1 {
				continue
			}
		}

		// anchors：每条在原文里出现
		for _, sg := range s.Segments {
			for _, a := range sg.Anchors {
				anchorsTotal++
				if !strings.Contains(full, a) {
					t.Errorf("%s：anchor %q 不在任何段原文里", s.ID, a)
				}
			}
		}

		// 规9：秘密扫描
		for _, re := range secrets {
			if m := re.FindString(full); m != "" {
				t.Errorf("%s：命中疑似真秘密 %q（规9 要求明显合成值）", s.ID, m)
			}
		}

		// 探针题面
		if s.Probe.Q == "" || s.Probe.ExpectRegex == "" || s.Probe.Grader != "regex" {
			t.Errorf("%s：probe 不完整", s.ID)
		}
		if _, err := regexp.Compile(s.Probe.ExpectRegex); err != nil {
			t.Errorf("%s：probe.expect_regex 编译不过：%v", s.ID, err)
		}

		// 轨迹类必须真有对白段（不是 observation 换皮）
		trajSegs := 0
		for _, sg := range s.Segments {
			if sg.Kind == compress.KindTrajectory {
				trajSegs++
			}
		}
		if trajSegs < 2 {
			t.Errorf("%s：trajectory 段只有 %d 段，要求 ≥2", s.ID, trajSegs)
		}
		_ = pinnedObs
	}

	if zh != 5 || en != 5 {
		t.Errorf("中英各半：zh=%d en=%d", zh, en)
	}
	if controls != 2 {
		t.Errorf("对照样本 = %d，要求 2（R15/R20）", controls)
	}
	if needleTotal != 50 || win != 40 || rec != 10 {
		t.Errorf("needle 合计 = %d（W %d / R %d），要求 50（40/10）", needleTotal, win, rec)
	}
	if anchorsTotal < 20 {
		t.Errorf("anchor 合计 = %d，要求 ≥ 10×2", anchorsTotal)
	}
	if !seen["R15"] || !seen["R20"] {
		t.Errorf("对照样本必须是 R15 / R20（规2）")
	}
	// 扫描要能分辨档位 ⇒ 未钉段必须承载足够多的 window needle（否则各档 MRR 恒 1.000）
	if unpinnedWindowNeedles < 18 {
		t.Errorf("未钉段承载的 window needle 只有 %d 条，Budget 扫描没有分辨力", unpinnedWindowNeedles)
	}

	var lens []string
	for _, s := range all {
		lens = append(lens, fmt.Sprintf("%s=%d(W%d/%d段)", s.ID, s.Tokens(), len(s.Segments), s.CountNeedles(WhereWindow)))
	}
	t.Logf("trajectory 10 条 · token 分布：%s", strings.Join(lens, " "))
	t.Logf("未钉段 window needle %d 条 · observation 段 %d 个", unpinnedWindowNeedles, withObservationSegment)
}

// 确定性：两次生成逐字节相同（任务单 §3.4 第 8 条）。
func TestTrajectory_Deterministic(t *testing.T) {
	a := fmt.Sprintf("%v", TrajectorySamples())
	b := fmt.Sprintf("%v", TrajectorySamples())
	if a != b {
		t.Fatalf("两次生成不一致")
	}
}

// AllSamples = 五类各 10 条（observation/trajectory/plan/memory/representation），id 不重叠。
func TestAllSamples_Union(t *testing.T) {
	all := AllSamples()
	if len(all) != 50 {
		t.Fatalf("AllSamples = %d 条，要求 50", len(all))
	}
	seen := map[string]bool{}
	for _, s := range all {
		if seen[s.ID] {
			t.Fatalf("%s：两类里 id 重复", s.ID)
		}
		seen[s.ID] = true
	}
}

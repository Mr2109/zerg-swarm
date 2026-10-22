package eval

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 本文件是**全量 50 条的构造闸**（任务单 §3.1 / §3.3 / §3.4 的机器判据）。
//
// 为什么要单开一份而不是只靠各类文件里的 10 条自检：配额是**跨类**的
// （250 = 180 W + 70 R，且每类 W/R 配额不同），任何一条样本单独看都自洽、
// 合起来却可能对不上 —— 只有把 50 条放在一张表上数，才能把「凑数」挡住。

// TestAllSamples_Quota —— 规1/规2/规4：类 × 语 配额、对照条数与长度、needle 配额。
func TestAllSamples_Quota(t *testing.T) {
	all := AllSamples()
	if len(all) != 50 {
		t.Fatalf("全量样本 = %d 条，要求 50", len(all))
	}

	// 每类 W/R 配额（任务单 3.3）
	wantNeedles := map[string][2]int{
		"observation":    {2, 3}, // 每样本 2 window + 3 recoverable
		"trajectory":     {4, 1},
		"plan":           {5, 0},
		"memory":         {5, 0},
		"representation": {2, 3},
	}
	wantControls := map[string]string{
		"observation": "R05/R10", "trajectory": "R15/R20", "plan": "R25/R30",
		"memory": "R35/R40", "representation": "R45/R50",
	}

	var totalWin, totalRec int
	perClass := map[string]int{}
	perClassLang := map[string]int{}
	seen := map[string]bool{}

	for _, s := range all {
		if seen[s.ID] {
			t.Errorf("%s：id 重复", s.ID)
		}
		seen[s.ID] = true
		perClass[s.Class]++
		perClassLang[s.Class+"/"+s.Lang]++

		// 规3：长度断言（对照 1800–2400 / 非对照 4400–6400）
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

		// 规4：每样本 5 条 needle，W/R 按类
		q := wantNeedles[s.Class]
		if len(s.Needles) != 5 {
			t.Errorf("%s：needle %d 条，要求 5", s.ID, len(s.Needles))
		}
		if w := s.CountNeedles(WhereWindow); w != q[0] {
			t.Errorf("%s：window needle %d 条，%s 类要求 %d", s.ID, w, s.Class, q[0])
		}
		if r := s.CountNeedles(WhereRecoverable); r != q[1] {
			t.Errorf("%s：recoverable needle %d 条，%s 类要求 %d", s.ID, r, s.Class, q[1])
		}
		totalWin += s.CountNeedles(WhereWindow)
		totalRec += s.CountNeedles(WhereRecoverable)
	}
	// 规1：每类 10 条、中英各半
	for _, class := range ClassNames {
		if perClass[class] != 10 {
			t.Errorf("%s 类 %d 条，要求 10", class, perClass[class])
		}
		if perClassLang[class+"/zh"] != 5 || perClassLang[class+"/en"] != 5 {
			t.Errorf("%s 类中英各半：zh=%d en=%d，要求 5/5",
				class, perClassLang[class+"/zh"], perClassLang[class+"/en"])
		}
	}
	// 规2：对照恰 10 条，且落在指定编号
	var controls []string
	for _, s := range all {
		if s.Control {
			controls = append(controls, s.ID)
		}
	}
	sort.Strings(controls)
	wantCtrl := []string{"R05", "R10", "R15", "R20", "R25", "R30", "R35", "R40", "R45", "R50"}
	if strings.Join(controls, ",") != strings.Join(wantCtrl, ",") {
		t.Errorf("对照样本 = %v，要求 %v", controls, wantCtrl)
	}
	_ = wantControls

	// 规4：全批 250 = 180 W + 70 R
	if totalWin+totalRec != 250 || totalWin != 180 || totalRec != 70 {
		t.Errorf("needle 合计 = %d（W %d / R %d），要求 250（180/70）", totalWin+totalRec, totalWin, totalRec)
	}
}

// TestAllSamples_NeedleVerbatim —— 规5/规6：needle 是原文逐字子串。
//
// 「全文只出现一次」只对本次新增的三类（plan / memory / representation）断言，
// 理由：同一条信息在两段各留一份，Budget 扫描里「这一档丢没丢」就分不清是闸没碰到
// 第一份还是第二份，临界档的读数会失真。observation 类早前的样本刻意重复了路径 /
// 计数（那批的判读靠 raw_ptr 与 RVS，不靠「唯一性」），trajectory 类另有自己的唯一性
// 断言（TestTrajectory_Complete）；这里不追溯历史，只锁新面。
func TestAllSamples_NeedleVerbatim(t *testing.T) {
	uniqueClasses := map[string]bool{"plan": true, "memory": true, "representation": true}
	for _, s := range AllSamples() {
		full := ""
		for _, sg := range s.Segments {
			full += sg.Text + "\n"
		}
		types := map[string]bool{}
		for _, nd := range s.Needles {
			if nd.Segment < 0 || nd.Segment >= len(s.Segments) {
				t.Errorf("%s/%s：segment 下标 %d 越界", s.ID, nd.ID, nd.Segment)
				continue
			}
			if !strings.Contains(s.Segments[nd.Segment].Text, nd.Text) {
				t.Errorf("%s/%s：needle 不在声明段的原文里：%q", s.ID, nd.ID, nd.Text)
			}
			if uniqueClasses[s.Class] {
				if n := strings.Count(full, nd.Text); n != 1 {
					t.Errorf("%s/%s：needle 全文出现 %d 次（要求 1）：%q", s.ID, nd.ID, n, nd.Text)
				}
			}
			if nd.Type == "" {
				t.Errorf("%s/%s：type 未填", s.ID, nd.ID)
			}
			types[nd.Type] = true
			if nd.Where != WhereWindow && nd.Where != WhereRecoverable {
				t.Errorf("%s/%s：where = %q", s.ID, nd.ID, nd.Where)
			}
		}
		if len(types) < 3 {
			t.Errorf("%s：needle 类型只盖了 %d 类，要求 ≥3（任务单 3.3 注）", s.ID, len(types))
		}
	}
}

// TestAllSamples_Structure —— 规7/规8/规9：钉法按类、raw_ptr 内容寻址、anchors 真出现、无真秘密。
func TestAllSamples_Structure(t *testing.T) {
	secrets := []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`1[3-9]\d{9}`),
	}
	pinnedByClass := map[string]int{}
	unpinnedWindowByClass := map[string]int{}

	for _, s := range AllSamples() {
		var full strings.Builder
		ptrSegs, pinnedSegs := 0, 0
		for i, sg := range s.Segments {
			full.WriteString(sg.Text)
			full.WriteString("\n")
			if sg.Pinned {
				pinnedSegs++
			}
			if sg.RawPtr == "" {
				continue
			}
			ptrSegs++
			want := "unload/" + textHash(sg.Text) + ".txt"
			if sg.RawPtr != want {
				t.Errorf("%s/seg%d：raw_ptr = %q，应为 %q（内容寻址）", s.ID, i, sg.RawPtr, want)
			}
		}
		pinnedByClass[s.Class] += pinnedSegs
		if ptrSegs == 0 {
			t.Errorf("%s：没有任何可回查段（raw_ptr）", s.ID)
		}

		// anchors：每样本 ≥2 条，且每条真在原文里（否则 Guard 的复算面是虚的）
		nAnchors := 0
		for _, sg := range s.Segments {
			for _, a := range sg.Anchors {
				nAnchors++
				if !strings.Contains(full.String(), a) {
					t.Errorf("%s：anchor %q 不在任何段原文里", s.ID, a)
				}
			}
		}
		if nAnchors < 2 {
			t.Errorf("%s：anchor 只有 %d 条，要求 ≥2", s.ID, nAnchors)
		}

		// 规7：plan/memory 的 window needle 必须落在钉住段（该类的「必留」语义）
		if s.Class == "plan" || s.Class == "memory" {
			for _, nd := range s.Needles {
				if nd.Where == WhereWindow && !s.Segments[nd.Segment].Pinned {
					t.Errorf("%s/%s：%s 类的 window needle 落在未钉段（规7 要求计划主体/决策账 Pinned）",
						s.ID, nd.ID, s.Class)
				}
			}
		}
		// 规7：representation 不钉
		if s.Class == "representation" && pinnedSegs != 0 {
			t.Errorf("%s：representation 类钉了 %d 段（规7 要求不钉，走 dedup/fold）", s.ID, pinnedSegs)
		}
		for _, nd := range s.Needles {
			if nd.Where == WhereWindow && !s.Segments[nd.Segment].Pinned {
				unpinnedWindowByClass[s.Class]++
			}
		}

		// 规9：秘密扫描
		for _, re := range secrets {
			if m := re.FindString(full.String()); m != "" {
				t.Errorf("%s：命中疑似真秘密 %q", s.ID, m)
			}
		}

		// 类型与类一致：kind 必须是 compress.Kind 的合法值之一
		for i, sg := range s.Segments {
			switch sg.Kind {
			case compress.KindObservation, compress.KindTrajectory, compress.KindPlan,
				compress.KindMemory, compress.KindRepresentation:
			default:
				t.Errorf("%s/seg%d：kind = %q 不是合法类", s.ID, i, sg.Kind)
			}
		}

		// 探针题面（本笔不判分，但不许空）
		if s.Probe.Q == "" || s.Probe.ExpectRegex == "" || s.Probe.Grader != "regex" {
			t.Errorf("%s：probe 不完整", s.ID)
		}
		if _, err := regexp.Compile(s.Probe.ExpectRegex); err != nil {
			t.Errorf("%s：probe.expect_regex 编译不过：%v", s.ID, err)
		}
	}

	if pinnedByClass["representation"] != 0 {
		t.Errorf("representation 类共钉了 %d 段，要求 0", pinnedByClass["representation"])
	}
	if pinnedByClass["plan"] == 0 || pinnedByClass["memory"] == 0 {
		t.Errorf("plan/memory 必须至少钉住一段：plan=%d memory=%d", pinnedByClass["plan"], pinnedByClass["memory"])
	}
	// 扫描要有分辨力：plan/memory 的 window needle 靠钉住段免疫，其余三类必须有落在
	// 未钉段里的 window needle，否则低预算档永远看不到丢。
	for _, class := range []string{"observation", "trajectory", "representation"} {
		if unpinnedWindowByClass[class] < 10 {
			t.Errorf("%s：未钉段承载的 window needle 只有 %d 条，Budget 扫描没有分辨力",
				class, unpinnedWindowByClass[class])
		}
	}
}

// TestTwoClassSamples_FrozenCohort —— 冻结 cohort 不许跟着样本面一起长。
//
// AllSamples() 是「当前全量」（50 条）；TwoClassSamples() 是「前两笔那 20 条」（R01–R20，
// 顺序不变）。compress 包那条「修前基线逐字节一致」的回归断言用的就是这个冻结集合 ——
// 一旦有人把新样本塞进去，历史读数就不再可比，「一致」二字立刻变成假的。
func TestTwoClassSamples_FrozenCohort(t *testing.T) {
	ss := TwoClassSamples()
	if len(ss) != 20 {
		t.Fatalf("冻结 cohort = %d 条，要求 20", len(ss))
	}
	for i, s := range ss {
		want := fmt.Sprintf("R%02d", i+1)
		if s.ID != want {
			t.Fatalf("第 %d 条 = %s，按 R01–R20 顺序应为 %s", i+1, s.ID, want)
		}
		if s.Class != "observation" && s.Class != "trajectory" {
			t.Fatalf("%s：class=%s，冻结 cohort 只含 observation/trajectory", s.ID, s.Class)
		}
	}
	if len(AllSamples()) != 50 {
		t.Fatalf("全量样本 %d 条，要求 50", len(AllSamples()))
	}
}

// TestAllSamples_Deterministic —— 50 条两次生成逐字节相同（任务单 §3.4 第 8 条）。
func TestAllSamples_Deterministic(t *testing.T) {
	a := fmt.Sprintf("%v", AllSamples())
	b := fmt.Sprintf("%v", AllSamples())
	if a != b {
		t.Fatalf("两次生成不一致")
	}
	t.Logf("全量 50 条 · 字节 %d", len(a))
}

// TestAllSamples_Distribution —— 把分布打成一行（回报与报告都要用这张表）。
func TestAllSamples_Distribution(t *testing.T) {
	type row struct {
		n, zh, en, ctrl, minT, maxT, w, r int
	}
	agg := map[string]*row{}
	for _, s := range AllSamples() {
		r := agg[s.Class]
		if r == nil {
			r = &row{minT: 1 << 30}
			agg[s.Class] = r
		}
		r.n++
		if s.Lang == "zh" {
			r.zh++
		} else {
			r.en++
		}
		if s.Control {
			r.ctrl++
		}
		if got := s.Tokens(); got < r.minT {
			r.minT = got
		}
		if got := s.Tokens(); got > r.maxT {
			r.maxT = got
		}
		r.w += s.CountNeedles(WhereWindow)
		r.r += s.CountNeedles(WhereRecoverable)
	}
	for _, class := range ClassNames {
		r := agg[class]
		t.Logf("%-14s n=%2d zh=%d en=%d 对照=%d token[%d,%d] needle W%02d/R%02d",
			class, r.n, r.zh, r.en, r.ctrl, r.minT, r.maxT, r.w, r.r)
	}
}

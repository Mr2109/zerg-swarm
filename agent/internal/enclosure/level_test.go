package enclosure

import (
	"strings"
	"testing"
	"time"
)

// 等级取值校验：四级合法；陌生串一律不合法（不许当"某种更严的等级"放过去）。
func TestLevelValid(t *testing.T) {
	for _, l := range AllLevels {
		if !l.Valid() {
			t.Fatalf("%q 应为合法等级", l)
		}
	}
	for _, bad := range []Level{"", "trusted", "sandboxed", "enclosed", "ENCLOSED.KERNEL", "kernel", "none "} {
		if bad.Valid() {
			t.Fatalf("%q 不该被认成合法等级（陌生值必须拒，判 fail-closed）", bad)
		}
	}
}

// 合法值清单与四级同源（错误信息不许各处手抄 —— 手抄就会与枚举漂移）。
func TestLevelsString(t *testing.T) {
	s := LevelsString()
	for _, want := range []string{"enclosed.kernel", "enclosed.os", "unverified", "none"} {
		if !strings.Contains(s, want) {
			t.Fatalf("清单应含 %q，实得 %q", want, s)
		}
	}
	if got := strings.Count(s, "|"); got != len(AllLevels)-1 {
		t.Fatalf("清单分隔符 %d 个，应与等级数 %d 一致：%q", got, len(AllLevels), s)
	}
}

// 表驱动：**无证据时永不给 enclosed.***（读不到 = unverified，独立一级）。
func TestJudge_NeverEnclosedWithoutEvidence(t *testing.T) {
	now := time.Now()
	cases := []Evidence{
		{Read: false},
		{Read: false, MountIsolated: true, PolicyActive: true}, // 读不到但自称有 ⇒ 仍不许给 enclosed.*
	}
	for i, ev := range cases {
		v := Judge(LevelKernel, ev, nil, now)
		if v.Observed == LevelKernel || v.Observed == LevelOS {
			t.Fatalf("用例 %d：无证据却给了 enclosed.*（%s）", i, v.Observed)
		}
		if v.Observed != LevelUnverified {
			t.Fatalf("用例 %d：应判 unverified，得到 %s", i, v.Observed)
		}
	}
}

func TestJudge_Levels(t *testing.T) {
	now := time.Now()
	if v := Judge(LevelKernel, Evidence{Read: true, MountIsolated: true}, nil, now); v.Observed != LevelKernel {
		t.Fatalf("视图级应判 kernel，得到 %s", v.Observed)
	}
	if v := Judge(LevelOS, Evidence{Read: true, PolicyActive: true}, nil, now); v.Observed != LevelOS {
		t.Fatalf("仅授权级应判 os，得到 %s", v.Observed)
	}
	if v := Judge(LevelNone, Evidence{Read: true}, nil, now); v.Observed != LevelNone {
		t.Fatalf("读到但无封闭应判 none，得到 %s", v.Observed)
	}
}

// 判据 8：expected 与 observed **两个字段都出现**，不一致时两字段各自保留。
func TestJudge_ExpectedObservedBothPresent(t *testing.T) {
	v := Judge(LevelKernel, Evidence{Read: true, PolicyActive: true}, []string{"ipc:in-space"}, time.Now())
	if v.Expected != LevelKernel {
		t.Fatalf("expected 丢失：%s", v.Expected)
	}
	if v.Observed != LevelOS {
		t.Fatalf("observed 丢失：%s", v.Observed)
	}
	if v.Expected == v.Observed {
		t.Fatal("本用例本就该不一致；若相等说明判定错了")
	}
	if v.Note == "" {
		t.Fatal("不一致时必须有 Note 说明（不许静默合并）")
	}
	if len(v.Allowlist) != 1 {
		t.Fatalf("放行清单必须随等级一起出现：%v", v.Allowlist)
	}
}

// 未申报（expected 空串，例：v1 旧档案没有 expected 这一项）：
// expected 如实留空、不与实测比较、也**不许回落成实测值**（那等于把两个字段合并着造假）。
func TestJudge_ExpectedUndeclared(t *testing.T) {
	now := time.Now()
	v := Judge("", Evidence{Read: true, MountIsolated: true}, nil, now)
	if v.Expected != "" {
		t.Fatalf("未申报时期望必须如实留空，实得 %q（回落成实测值 = 把两字段合并）", v.Expected)
	}
	if v.Observed != LevelKernel {
		t.Fatalf("实测有视图级证据应判 kernel，实得 %s", v.Observed)
	}
	if !strings.Contains(v.Note, "未申报") {
		t.Fatalf("未申报时必须留痕说明（不许写成「期望与实测不一致」），实得 %q", v.Note)
	}
	// 无证据时同样不许给 enclosed.*（未申报不影响这条铁律）
	if u := Judge("", Evidence{}, nil, now); u.Observed != LevelUnverified {
		t.Fatalf("未申报 + 读不到证据 ⇒ unverified，实得 %s", u.Observed)
	}
}

package main

// family_doctor_daemons_test.go —— `zerg doctor daemons` 的判据（缺口 `GAP-20260926-224`）。
//
// 四条牙：
//   〔甲〕解析：只认 `com.zerg.*` 前缀；`-` 的 pid 与制表符列都要认；非本前缀不看
//   〔乙〕判定：崩溃循环（退出码非 0 + KeepAlive=true）⇒ FAIL；非常驻 ⇒ REPORT；在跑且退出码 0 ⇒ PASS
//   〔丙〕plist：`<true/>` 与 `<dict>` 都算常驻；没有那一格 ⇒ false
//   〔丁〕退码：有 FAIL ⇒ 1；有 BLOCKED ⇒ 8（没结论优先于判红）

import (
	"strings"
	"testing"
)

func TestDaParseList(t *testing.T) {
	out := `PID	Status	Label
77600	0	com.zerg.core
79716	255	com.zerg.tunnel.mini-node-1
-	0	com.zerg.watchdog
123	0	com.apple.something
-	78	com.zerg.ui
`
	rows := daParseList(out)
	if len(rows) != 4 {
		t.Fatalf("只该认 4 条 com.zerg.*（实得 %d 条：%+v）", len(rows), rows)
	}
	want := []struct{ label, pid, exit string }{
		{"com.zerg.core", "77600", "0"},
		{"com.zerg.tunnel.mini-node-1", "79716", "255"},
		{"com.zerg.watchdog", "-", "0"},
		{"com.zerg.ui", "-", "78"},
	}
	for i, w := range want {
		if rows[i].Label != w.label || rows[i].PID != w.pid || rows[i].LastExit != w.exit {
			t.Errorf("第 %d 条解析错：实得 %+v，期望 %+v", i, rows[i], w)
		}
	}
}

func TestDaDecide(t *testing.T) {
	cases := []struct {
		name string
		in   daRow
		want string
	}{
		{"在跑且退出码 0 ⇒ PASS", daRow{PID: "77600", LastExit: "0", KeepAlive: true}, "PASS"},
		{"在跑但退出码非 0 且常驻 ⇒ FAIL（反复重启）", daRow{PID: "79716", LastExit: "255", KeepAlive: true}, "FAIL"},
		{"在跑但退出码非 0 且非常驻 ⇒ REPORT", daRow{PID: "123", LastExit: "2", KeepAlive: false}, "REPORT"},
		{"未运行 + 退出码非 0 + 常驻 ⇒ FAIL（拉起即死）", daRow{PID: "-", LastExit: "78", KeepAlive: true}, "FAIL"},
		{"未运行 + 退出码非 0 + 非常驻 ⇒ REPORT", daRow{PID: "-", LastExit: "78", KeepAlive: false}, "REPORT"},
		{"未运行且无退出码 ⇒ REPORT", daRow{PID: "-", LastExit: "0", KeepAlive: true}, "REPORT"},
	}
	for _, c := range cases {
		r := c.in
		daDecide(&r)
		if r.Verdict != c.want {
			t.Errorf("%s：判定 = %q，期望 %q（note=%q）", c.name, r.Verdict, c.want, r.Note)
		}
	}
}

func TestDaPlistKeepAlive(t *testing.T) {
	cases := map[string]bool{
		`<key>KeepAlive</key><true/>`:                                        true,
		`<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>`: true,
		`<key>KeepAlive</key><false/>`:                                       false,
		`<key>RunAtLoad</key><true/>`:                                        false,
		``:                                                                   false,
	}
	for in, want := range cases {
		if got := daPlistKeepAlive(in); got != want {
			t.Errorf("daPlistKeepAlive(%q) = %v，期望 %v", in, got, want)
		}
	}
}

func TestDaRC(t *testing.T) {
	if rc := daRC([]daRow{{Verdict: "PASS"}, {Verdict: "REPORT"}}); rc != 0 {
		t.Errorf("全 PASS/REPORT ⇒ rc=0，实得 %d", rc)
	}
	if rc := daRC([]daRow{{Verdict: "PASS"}, {Verdict: "FAIL"}}); rc != 1 {
		t.Errorf("有 FAIL ⇒ rc=1，实得 %d", rc)
	}
	if rc := daRC([]daRow{{Verdict: "FAIL"}, {Verdict: "BLOCKED"}}); rc != 8 {
		t.Errorf("有 BLOCKED ⇒ rc=8（没结论优先），实得 %d", rc)
	}
}

// TestDaNoteNamesTheLoop —— 判红的 note 必须说清「为什么红」（✗ 只给一个 FAIL 字）。
func TestDaNoteNamesTheLoop(t *testing.T) {
	r := daRow{PID: "79716", LastExit: "255", KeepAlive: true}
	daDecide(&r)
	for _, want := range []string{"255", "KeepAlive"} {
		if !strings.Contains(r.Note, want) {
			t.Errorf("note 要点名 %q（实得 %q）", want, r.Note)
		}
	}
}

// ── 稳跑档（2026-10-23 真机现读的假红修正）─────────────────────────────────

func TestDaParseEtime(t *testing.T) {
	cases := map[string]int{
		"00:03":      3,
		"02:04:10":   7450,
		"1-00:32:12": 86400 + 32*60 + 12,
		"":           0,
		"胡说":         0,
	}
	for in, want := range cases {
		if got := daParseEtime(in); got != want {
			t.Errorf("daParseEtime(%q) = %d，期望 %d", in, got, want)
		}
	}
}

// TestDaDecideStableUptimeIsNotACrashLoop —— 现读实证：com.zerg.ui 的 pid 已连跑 1 天，
// 而 launchctl 第二列还是上一代的 78。若照旧一律判 FAIL，就是自己造假红。
func TestDaDecideStableUptimeIsNotACrashLoop(t *testing.T) {
	r := daRow{PID: "1626", LastExit: "78", KeepAlive: true, UptimeSec: 86400 + 1920}
	daDecide(&r)
	if r.Verdict != "REPORT" {
		t.Fatalf("已稳跑（>5min）时不该判崩溃循环，实得 %q（note=%q）", r.Verdict, r.Note)
	}
	if !strings.Contains(r.Note, "上一代") {
		t.Errorf("note 要说清这个退出码是上一代的（实得 %q）", r.Note)
	}

	// 刚被拉起（< 5min）⇒ 仍判 FAIL（真循环）
	r2 := daRow{PID: "30262", LastExit: "255", KeepAlive: true, UptimeSec: 3}
	daDecide(&r2)
	if r2.Verdict != "FAIL" {
		t.Fatalf("刚被拉起（3s）且退出码 255 ⇒ 必须判 FAIL，实得 %q", r2.Verdict)
	}
}

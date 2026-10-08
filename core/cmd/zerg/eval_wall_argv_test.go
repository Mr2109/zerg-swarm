// eval_wall_argv_test.go —— 件 core/cmd/zerg/eval_wall_argv.go 的成对负控（✗ 禁恒绿）。
//
// 本件只测**判据层与参数层**（纯函数）＋一条「自检有牙」的负控 —— 入口的真跑面（真起
// go test / 茧壁二进制）由对拍台在仓外做两态对拍，不在单测里重跑（那是分钟级、且要吃茧壁制品）。
package main

import "testing"

func TestEwaExecutableVerdict(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		ok   bool
	}{
		{"bwrap 当头 ⇒ 绿", []string{"bwrap", "--unshare-pid", "--die-with-parent"}, true},
		{"选项当头 ⇒ 红", []string{"--ro-bind", "/usr", "/usr"}, false},
		{"以 '-' 开头 ⇒ 红", []string{"-bwrap", "--x"}, false},
		{"空 argv ⇒ 红", []string{}, false},
	}
	for _, c := range cases {
		got := ewaExecutableVerdict(c.argv) == ""
		if got != c.ok {
			t.Fatalf("%s：判决 %v，期望 %v", c.name, got, c.ok)
		}
	}
}

func TestEwaArgvVerdict(t *testing.T) {
	goArgv := []string{"--ro-bind", "/usr", "/usr", "--unshare-pid"}
	same := []string{"bwrap", "--ro-bind", "/usr", "/usr", "--unshare-pid"}
	if v := ewaArgvVerdict(goArgv, same); v != "" {
		t.Fatalf("逐条一致应无判决，实得 %q", v)
	}
	short := []string{"bwrap", "--ro-bind", "/usr", "/usr"}
	if v := ewaArgvVerdict(goArgv, short); v == "" {
		t.Fatal("少一项应判红，实得空（判据没牙）")
	}
	drift := []string{"bwrap", "--ro-bind", "/usr", "/usr", "--proc", "/proc"}
	if v := ewaArgvVerdict(goArgv, drift); v == "" {
		t.Fatal("末项漂移应判红，实得空（判据没牙）")
	}
}

func TestEwaCompareOne(t *testing.T) {
	okRes := ewaGoRes{Argv: []string{"--unshare-pid"}}
	ok, detail := ewaCompareOne("ok-01.json", okRes, 0, `{"argv":["bwrap","--unshare-pid"]}`, "")
	if !ok {
		t.Fatalf("好配方两侧一致应绿，实得红：%s", detail)
	}
	if ok, _ := ewaCompareOne("ok-01.json", okRes, 2, "", "boom"); ok {
		t.Fatal("茧壁 rc≠0 应判红（判据没牙）")
	}
	rejRes := ewaGoRes{Error: "缺 egg_id"}
	if ok, _ := ewaCompareOne("reject-01.json", rejRes, 2, "", ""); !ok {
		t.Fatal("坏配方两侧都拒应绿")
	}
	if ok, _ := ewaCompareOne("reject-01.json", ewaGoRes{}, 2, "", ""); ok {
		t.Fatal("Go 侧放行坏配方应判红（判据没牙）")
	}
	if ok, _ := ewaCompareOne("weird-01.json", okRes, 0, "", ""); ok {
		t.Fatal("认不得的文件名前缀应判红")
	}
}

func TestEwaParseArgs(t *testing.T) {
	a := ewaParseArgs([]string{"--help"}, "W", "S")
	if !a.helpRequested {
		t.Fatal("--help 应置 helpRequested")
	}
	a = ewaParseArgs([]string{"-h"}, "W", "S")
	if !a.helpRequested {
		t.Fatal("-h 应置 helpRequested")
	}
	a = ewaParseArgs([]string{"--zzz"}, "W", "S")
	if len(a.unrecognized) != 1 || a.unrecognized[0] != "--zzz" {
		t.Fatalf("未知旗标应进 unrecognized，实得 %v", a.unrecognized)
	}
	a = ewaParseArgs([]string{"--s"}, "W", "S")
	if a.parseErr == "" {
		t.Fatal("--s 有歧义（--specs/--self-test）应报 parseErr")
	}
	a = ewaParseArgs([]string{"--wall", "/x", "--no-build", "--specs=/y"}, "W", "S")
	if a.wall != "/x" || !a.noBuild || a.specs != "/y" {
		t.Fatalf("取值/开关档解析不对：%+v", a)
	}
	a = ewaParseArgs([]string{"--wall"}, "W", "S")
	if a.parseErr == "" {
		t.Fatal("--wall 缺值应报 parseErr")
	}
	a = ewaParseArgs([]string{}, "W", "S")
	if a.wall != "W" || a.specs != "S" {
		t.Fatalf("缺省值应回落到默认：%+v", a)
	}
}

func TestEwaWallPayload(t *testing.T) {
	argv, exc, ok := ewaWallPayload(`{"argv":["bwrap","--unshare-pid"]}`)
	if !ok || len(argv) != 2 || argv[0] != "bwrap" {
		t.Fatalf("正常对象应取到 argv，实得 ok=%v argv=%v exc=%q", ok, argv, exc)
	}
	argv, _, ok = ewaWallPayload(`{"other":1}`)
	if !ok || len(argv) != 0 {
		t.Fatalf("缺 argv 键应给空表且 ok，实得 ok=%v argv=%v", ok, argv)
	}
	if _, _, ok := ewaWallPayload("not json"); ok {
		t.Fatal("非法 JSON 应 ok=false（判据没牙）")
	}
}

// TestEwaSelfTestTeeth —— 负控：自检必须能红（✗ 禁恒绿）。
func TestEwaSelfTestTeeth(t *testing.T) {
	quiet := func(string) {}
	good := []ewaSelfTestRow{{name: "ok-01.json",
		goArgv: []string{"--a", "--b"}, wallArgv: []string{"bwrap", "--a", "--b"}}}
	if !ewaSelfTest(good, quiet) {
		t.Fatal("原样一致时自检应绿")
	}
	drift := []ewaSelfTestRow{{name: "ok-01.json",
		goArgv: []string{"--a", "--b"}, wallArgv: []string{"bwrap", "--a", "--c"}}}
	if ewaSelfTest(drift, quiet) {
		t.Fatal("真实产物漂移时自检必须红（判据没牙）")
	}
	empty := []ewaSelfTestRow{{name: "ok-01.json",
		goArgv: []string{"--a"}, wallArgv: []string{}}}
	if ewaSelfTest(empty, quiet) {
		t.Fatal("空 argv 时自检必须红（py 宽容切片口径）")
	}
}

// check_two_pass_identity_test.go —— `core/cmd/zerg/check_two_pass_identity.go`（M2 真搬件）的成对负控。
//
// 目的：给 `checkTwoPassIdentityGateMain` 一道**带牙**的回归 —— 哑化任一条判据
// （① 抖动登记表剥多/剥少 · ② 文本抖动行 · ③ 比较器 · ④ 四档退码 · ⑤ 写旗标 · ⑥ 自检），
// 立刻有用例红并点名。
//
// 纪律：只读真仓（子进程只跑本用例自建临时目录里的合成判据件）；不写真仓一个字节；临时目录 `t.TempDir` 自动回收。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func ctpiRunMain(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkTwoPassIdentityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ① 归一：已登记抖动字段（meta.at）**被剥**；未登记字段（pid）**不剥** —— 两态可辨（有牙）。
func TestCtpiNormalizeDropsOnlyRegistered(t *testing.T) {
	lineReg := `{"items":[{"path":"a.md","rc":0,"baseline_from":"head-blob:a.md"}],"meta":{"at":"2026-10-01T18:00:00+0800"}}`
	lineUnreg := `{"items":[{"path":"a.md","rc":0,"baseline_from":"head-blob:a.md"}],"meta":{"pid":12345}}`
	c1, d1, ok1 := ctpiCanonJSONLine(lineReg)
	if !ok1 || len(d1) != 1 {
		t.Fatalf("已登记字段 meta.at 应被剥 1 项，实得 ok=%v dropped=%v", ok1, d1)
	}
	if strings.Contains(c1, "\"at\"") {
		t.Fatalf("归一后不该再含 meta.at：%s", c1)
	}
	c2, d2, ok2 := ctpiCanonJSONLine(lineUnreg)
	if !ok2 || len(d2) != 0 {
		t.Fatalf("未登记字段 pid **不许**被剥（剥了就假绿），实得 ok=%v dropped=%v", ok2, d2)
	}
	if !strings.Contains(c2, "\"pid\"") {
		t.Fatalf("未登记字段 pid 应原样留在归一结果里：%s", c2)
	}
}

// ② 文本档抖动行「时刻: ISO」**被剥**；别的行原样保留。
func TestCtpiNormalizeTextVolatileLine(t *testing.T) {
	in := []byte("行甲\n时刻: 2026-10-01T18:00:00+0800\n行乙\n")
	_, norm, dropped := ctpiNormalize(in)
	if len(dropped) != 1 {
		t.Fatalf("应剥 1 行文本抖动，实得 %v", dropped)
	}
	if strings.Contains(string(norm), "时刻:") {
		t.Fatalf("抖动行应被剥掉：%q", string(norm))
	}
	if !strings.Contains(string(norm), "行甲") || !strings.Contains(string(norm), "行乙") {
		t.Fatalf("非抖动行应原样保留：%q", string(norm))
	}
}

// ③ 比较器有牙：不同字节必报不同并报名首差行；相同字节报同（哑化 ⇒ 本用例 FAIL 带行号）。
func TestCtpiFirstDiffLineHasTeeth(t *testing.T) {
	n, x, y, ok := ctpiFirstDiffLine([]byte("a\nb\n"), []byte("a\nc\n"))
	if !ok || n != 2 || x != "b" || y != "c" {
		t.Fatalf("首差行应报第 2 行 b⟷c，实得 n=%d x=%q y=%q ok=%v", n, x, y, ok)
	}
	if _, _, _, ok2 := ctpiFirstDiffLine([]byte("x\n"), []byte("x\n")); ok2 {
		t.Fatal("逐字节相同的两跑被报成有差异 ⇒ 比较器假绿")
	}
}

// ④ 四档退码：稳定 ⇒ 0 · 未登记抖动(pid) ⇒ 1 · 无机器读面 ⇒ 2 · 可变件 ⇒ 8。
func TestCtpiJudgeBranches(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("无 python3 ⇒ 本格不给结论（不假装绿）")
	}
	d := t.TempDir()
	mk := func(name, body string) string {
		p := filepath.Join(d, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	stable := mk("stable.py", "import json;print(json.dumps({'items':[{'path':'a.md','rc':0,'baseline_from':'head-blob:a.md'}]},sort_keys=True))")
	unreg := mk("unreg.py", "import json,os;print(json.dumps({'items':[{'path':'a.md','rc':0,'baseline_from':'head-blob:a.md'}],'meta':{'pid':os.getpid()}},sort_keys=True))")
	prose := mk("prose.py", "print('这不是 JSON')")
	dirty := mk("dirty.py", "import json;print(json.dumps({'items':[{'path':'w.txt','rc':8,'class':'工作树','note':'无可比基准'}]},sort_keys=True))")
	root := ctpiRepoRoot()
	var buf bytes.Buffer
	if rc, _ := ctpiJudge(&buf, root, "python3 "+stable, true); rc != ctpiRCOK {
		t.Fatalf("稳定判据应 rc=0，实得 %d", rc)
	}
	if rc, _ := ctpiJudge(&buf, root, "python3 "+unreg, true); rc != ctpiRCRed {
		t.Fatalf("未登记抖动(pid) 应 rc=1，实得 %d", rc)
	}
	if rc, _ := ctpiJudge(&buf, root, "python3 "+prose, true); rc != ctpiRCNoConc {
		t.Fatalf("无机器读面应 rc=2，实得 %d", rc)
	}
	if rc, _ := ctpiJudge(&buf, root, "python3 "+dirty, true); rc != ctpiRCNoBase {
		t.Fatalf("可变件应 rc=8，实得 %d", rc)
	}
}

// ⑤ 写旗标 ⇒ rc=2 且逐字点名（先于 argparse）。哑化「写旗标检查」⇒ 本用例 FAIL。
func TestCtpiWriteFlagRejected(t *testing.T) {
	rc, out, _ := ctpiRunMain(t, "--record")
	if rc != ctpiRCNoConc {
		t.Fatalf("写旗标应 rc=2，实得 %d", rc)
	}
	if !strings.Contains(out, "写旗标 ['--record']") {
		t.Fatalf("应逐字点名写旗标，实得 %q", out)
	}
}

// ⑥ 自检全过（成对正/负夹具）—— 哑化任一判据 ⇒ 本用例 FAIL。
func TestCtpiSelfTestAllPass(t *testing.T) {
	rc, out, _ := ctpiRunMain(t, "--self-test")
	if rc != ctpiRCOK {
		t.Fatalf("自检应 rc=0，实得 %d；输出:\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("自检应全过，实得:\n%s", out)
	}
}

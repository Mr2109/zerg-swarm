// check_public_marks_test.go —— 判据件 `scripts/gates/check-public-marks.py` 的 Go 真搬 **成对负控**测试件。
//
// 覆盖：
//
//	· cpmPyJoin 三语义差（中间 `..` 原样留 · 后段绝对丢前段 · 空段保留尾 `/`）—— 定向单测
//	· cpmDecodeB64 严格解码（4k+1 非法 · 填充补不上 · 非 utf-8 ⇒ 不解）
//	· cpmScanPrivate 真口子：读不了的件 ⇒ 报「读不了文件：[Errno …]」（B 类留牙）
//	· selftest 36 条全绿（正控）
//	· must-fail 有牙：哑化编码变形格 ⇒ 自检必红（见交付③的仓外副本哑化实证）
package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func b64StdEnc(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func TestCpmPyJoin三语义差(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"中间 `..` 原样留", cpmPyJoin("a", "../b"), "a/../b"},
		{"后段绝对丢前段", cpmPyJoin("repo", "/abs"), "/abs"},
		{"空段保留尾斜杠", cpmPyJoin("a", ""), "a/"},
		{"空段中间", cpmPyJoin("a", "", "b"), "a/b"},
		{"空首段", cpmPyJoin("", "a"), "a"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s：cpmPyJoin = %q，期望 %q", c.name, c.got, c.want)
		}
	}
	if cpmPyJoin("a", "b") != "a/b" {
		t.Errorf("常规拼接错")
	}
}

func TestCpmPyRelpath(t *testing.T) {
	if cpmPyRelpath("/r/scripts/build/x.sh", "/r") != "scripts/build/x.sh" {
		t.Errorf("relpath 基本态错：%q", cpmPyRelpath("/r/scripts/build/x.sh", "/r"))
	}
}

func TestCpmDecodeB64严格(t *testing.T) {
	if s, ok := cpmDecodeB64("L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=", false); !ok || s != "/Users/"+`Mr2109`+"/.config/zerg/token" {
		t.Errorf("标准 base64 解错：ok=%v s=%q", ok, s)
	}
	if _, ok := cpmDecodeB64("abcde", false); ok { // len%4==1 ⇒ 非法
		t.Errorf("4k+1 必须不解")
	}
	if _, ok := cpmDecodeB64(b64StdEnc([]byte("abcd")), false); !ok { // 合法且 utf-8 ⇒ 能解
		t.Errorf("合法 base64 应能解出")
	}
	if _, ok := cpmDecodeB64("ZZZZ", false); ok { // 解出 0x65 0x96 0x59 非 utf-8 ⇒ 不解（同 py）
		t.Errorf("非 utf-8 的 base64 必须不解")
	}
	raw := []byte{0xff, 0xfe}
	if _, ok := cpmDecodeB64(b64StdEnc(raw), false); ok {
		t.Errorf("非 utf-8 必须不解")
	}
}

func TestCpmEncoded仅命中形态表(t *testing.T) {
	// 无害 base64（普通英文）⇒ 解码成功但不命中形态表 ⇒ 不报
	ok := b64StdEnc([]byte("hello zerg swarm public fixture ok"))
	if dec, good := cpmDecodeB64(ok, false); !good {
		t.Fatalf("无害 base64 应能解")
	} else if why, _ := cpmPlainMatch(dec); why != "" {
		t.Errorf("无害 base64 不该命中形态表，实际命中 %q", why)
	}
}

// 真口子（B 类）留牙：读不了的件既不静默跳过、也不崩，而是逐字报「读不了文件」。
func TestCpmScanPrivate读不了文件留牙(t *testing.T) {
	f, err := os.CreateTemp("", "cpm-noread-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name)
	if err := os.Chmod(name, 0o000); err != nil {
		t.Skip("无法 chmod（环境不支持）")
	}
	hits := cpmScanPrivate(name)
	if len(hits) == 0 || !strings.Contains(hits[0].why, "读不了文件") ||
		!strings.Contains(hits[0].why, "Permission denied") {
		t.Fatalf("读不了的件必须报「读不了文件：[Errno 13] Permission denied」（B 类留牙），实际 %#v", hits)
	}
}

// 正控：36 条自检必须全绿。
func TestCpmSelftest全绿(t *testing.T) {
	var b strings.Builder
	if rc := cpmSelftest(&b); rc != 0 {
		t.Fatalf("自检应 rc=0，实际 rc=%d\n%s", rc, b.String())
	}
	if !strings.Contains(b.String(), "自证用例：36 条 · 通过 36 条") {
		t.Fatalf("自检条数断言缺失：\n%s", b.String())
	}
}

// 负控有牙：私有关键模式被摘掉后，②（内网 IP）那格必须失守 ⇒ rc≠0。
func TestCpmPrivatePattern哑化必红(t *testing.T) {
	saved := cpmPrivatePatterns
	cpmPrivatePatterns = nil
	defer func() { cpmPrivatePatterns = saved }()
	var b strings.Builder
	if rc := cpmSelftest(&b); rc == 0 {
		t.Fatalf("摘掉私有模式表后自检必须红（负控有牙），实际 rc=0")
	}
}

// 机读面（缺口 GAP-20260926-50）：`--json` 出 payload ＋ 未知字段 ⇒ rc=2（哑化任一 ⇒ 本用例必红）。
func TestCpm机读面正负控(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("scripts/build/good.sh", "#!/bin/sh\necho ok\n")
	mk(cpmTableRel, "文件\tpublic\t理由\nscripts/build/good.sh\tyes\tA 类：合成夹具\n")
	run := func(argv ...string) (int, string, string) {
		var ob, eb strings.Builder
		rc := checkPublicMarksGateMain(&ob, &eb, argv)
		return rc, ob.String(), eb.String()
	}
	rc, out, _ := run("--root", root, "--json")
	if rc != 0 || !strings.Contains(out, "\"mode\": \"judge\"") || !strings.Contains(out, "\"missing\": [") {
		t.Fatalf("--json 正控：期望 rc=0 ＋ payload（mode=judge · missing 在场），实得 rc=%d out=%q", rc, out)
	}
	rc, out, _ = run("--root", root, "--json", "root")
	if rc != 0 || !strings.Contains(out, "\"root\":") || strings.Contains(out, "\"missing\":") {
		t.Fatalf("--json root：期望只出 root 键，实得 rc=%d out=%q", rc, out)
	}
	rc, _, errs := run("--root", root, "--json", "nope")
	if rc != 2 || !strings.Contains(errs, "不认的字段名") {
		t.Fatalf("--json nope：期望 rc=2 ＋「不认的字段名」，实得 rc=%d stderr=%q", rc, errs)
	}
}

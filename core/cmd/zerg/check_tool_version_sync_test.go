// check_tool_version_sync_test.go —— 判据件 `scripts/gates/check-tool-version-sync.sh` 的 Go 真搬 **成对负控**测试件。
//
// 覆盖：ctvsPyJoin / ctvsSplitext / ctvsSplitLines 三语义差 · 入口 argv 不含程序名（inproc 口径）·
// ctvsCheck 成对正负控（抬头一致⇒0 / 不符⇒1 / 台账值缺失⇒2 / 缺文档⇒1 / 无台账⇒2）·
// 「空转 ≠ 取不到」两态分开 · 盘上多件不改退码 · 自检全绿 · **哑化有牙**（替换抬头正则 ⇒ 正控必翻红）。
package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

// ctvsTgFixture —— 建仓外合成夹具（<d>/tools/versions.json + <d>/tools/*.md）。versionsJSON == "" ⇒ 不建台账。
func ctvsTgFixture(t *testing.T, versionsJSON string, mds map[string]string) string {
	t.Helper()
	d := t.TempDir()
	td := ctvsPyJoin(d, "tools")
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	if versionsJSON != "" {
		if err := os.WriteFile(ctvsPyJoin(td, "versions.json"), []byte(versionsJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range mds {
		if err := os.WriteFile(ctvsPyJoin(td, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// ctvsTgRun —— 注入夹具根（免 os.Chdir 级联），跑 Go 入口，回 (rc, stdout+stderr)。
func ctvsTgRun(root string, argv []string) (int, string) {
	old := ctvsRoot
	ctvsRoot = func() (string, error) { return root, nil }
	defer func() { ctvsRoot = old }()
	var out, errb bytes.Buffer
	rc := checkToolVersionSyncGateMain(&out, &errb, argv)
	return rc, out.String() + errb.String()
}

func TestCtvsPyJoin三语义差(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"中间 `..` 原样留", ctvsPyJoin("a", "../b"), "a/../b"},
		{"后段绝对丢前段", ctvsPyJoin("repo", "/abs"), "/abs"},
		{"空段保留尾斜杠", ctvsPyJoin("a", ""), "a/"},
		{"空段中间", ctvsPyJoin("a", "", "b"), "a/b"},
		{"空首段", ctvsPyJoin("", "a"), "a"},
		{"常规拼接", ctvsPyJoin("a", "b"), "a/b"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s：ctvsPyJoin = %q，期望 %q", c.name, c.got, c.want)
		}
	}
}

func TestCtvsSplitext(t *testing.T) {
	cases := []struct{ in, base, ext string }{
		{"alpha.md", "alpha", ".md"},
		{"a.b.md", "a.b", ".md"},
		{".md", ".md", ""},
		{"noext", "noext", ""},
	}
	for _, c := range cases {
		b, e := ctvsSplitext(c.in)
		if b != c.base || e != c.ext {
			t.Errorf("ctvsSplitext(%q) = (%q,%q)，期望 (%q,%q)", c.in, b, e, c.base, c.ext)
		}
	}
}

func TestCtvsSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"\n", []string{""}},
		{"a\u2028b", []string{"a", "b"}},
		{"a\u000bb", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := ctvsSplitLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("ctvsSplitLines(%q) 长度 %d，期望 %d (%q)", c.in, len(got), len(c.want), c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ctvsSplitLines(%q)[%d] = %q，期望 %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestCtvsCheck成对正负控(t *testing.T) {
	mdOK := "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.3\n"
	// 正控：抬头一致 ⇒ rc=0 且带 ✓ 行
	fx := ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, map[string]string{"alpha.md": mdOK})
	if rc, out := ctvsTgRun(fx, nil); rc != 0 || !strings.Contains(out, ctvsOKLine) {
		t.Errorf("正控应 rc=0 且带 ✓ 行，得 rc=%d out=%q", rc, out)
	}
	// 负控：抬头不符 ⇒ rc=1
	fx = ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, map[string]string{"alpha.md": "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.4\n"})
	if rc, _ := ctvsTgRun(fx, nil); rc != 1 {
		t.Errorf("抬头不符应 rc=1，得 %d", rc)
	}
	// 负控：台账值缺失 ⇒ rc=2（旧件此处静默跳过 ⇒ 假绿）
	fx = ctvsTgFixture(t, `{"tools":{"alpha":{}}}`, map[string]string{"alpha.md": mdOK})
	if rc, _ := ctvsTgRun(fx, nil); rc != 2 {
		t.Errorf("台账值缺失应 rc=2，得 %d", rc)
	}
	// 负控：缺文档 ⇒ rc=1
	fx = ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, nil)
	if rc, _ := ctvsTgRun(fx, nil); rc != 1 {
		t.Errorf("缺文档应 rc=1，得 %d", rc)
	}
	// 取不到输入：无 versions.json ⇒ rc=2（fail-closed）
	fx = ctvsTgFixture(t, "", nil)
	if rc, out := ctvsTgRun(fx, nil); rc != 2 || !strings.Contains(out, ctvsVMiss) {
		t.Errorf("无台账应 rc=2 且带 FATAL 文案，得 rc=%d out=%q", rc, out)
	}
	// ★ 空转 ≠ 取不到：空台账（无 tools/*.md）⇒ rc=0（py 原样 · 不当作取不到）
	fx = ctvsTgFixture(t, `{"tools":{}}`, nil)
	if rc, _ := ctvsTgRun(fx, nil); rc != 0 {
		t.Errorf("空台账应 rc=0（空转非取不到），得 %d", rc)
	}
	// 报告档：盘上有而台账无 ⇒ 出声但退码不受影响（仍 0）
	fx = ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, map[string]string{
		"alpha.md": mdOK,
		"beta.md":  "# beta\n" + ctvsVerWord + ctvsColon + "v0.0.1\n",
	})
	if rc, out := ctvsTgRun(fx, nil); rc != 0 || !strings.Contains(out, ctvsUnlHdr) {
		t.Errorf("盘上多件应 rc=0 且出声，得 rc=%d out=%q", rc, out)
	}
}

func TestCtvs入口argv不含程序名(t *testing.T) {
	fx := ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, map[string]string{
		"alpha.md": "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.4\n",
	})
	// --quiet 必须落在 argv[0] 位就生效（inproc 口径：argv 不含程序名）
	rc, out := ctvsTgRun(fx, []string{"--quiet"})
	if rc != 1 {
		t.Fatalf("带 --quiet 应 rc=1，得 %d", rc)
	}
	if strings.Contains(out, ctvsBadHdr) {
		t.Errorf("--quiet 未生效：仍打印明细块头 —— argv 第 0 位被当程序名吃掉？")
	}
	// 未知旗标 ⇒ rc=2 且点名（证明扫描到位、非静默放行）
	rc, out = ctvsTgRun(fx, []string{"--zzz"})
	if rc != 2 || !strings.Contains(out, ctvsUnknownMsg+"--zzz") {
		t.Errorf("未知旗标应 rc=2 且点名，得 rc=%d out=%q", rc, out)
	}
}

func TestCtvs自检全绿(t *testing.T) {
	var buf bytes.Buffer
	rc := ctvsSelftest(&buf)
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 %d\n%s", rc, buf.String())
	}
	if !strings.Contains(buf.String(), ctvsSTHdr) {
		t.Errorf("自检缺前言：\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "自检结论: 全过") {
		t.Errorf("自检未见『全过』：\n%s", buf.String())
	}
}

// TestCtvs哑化有牙_抬头正则 —— 负控有牙实证：把抬头正则换成永不匹配 ⇒ 正控夹具必由 0 翻 1。
func TestCtvs哑化有牙_抬头正则(t *testing.T) {
	fx := ctvsTgFixture(t, `{"tools":{"alpha":"v1.2.3"}}`, map[string]string{
		"alpha.md": "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.3\n",
	})
	if rc, _ := ctvsTgRun(fx, nil); rc != 0 {
		t.Fatalf("基线应 rc=0，得 %d（夹具或实现已坏）", rc)
	}
	old := ctvsHeadRe
	ctvsHeadRe = regexp.MustCompile(`\A\znever-match\z`)
	defer func() { ctvsHeadRe = old }()
	if rc, _ := ctvsTgRun(fx, nil); rc != 1 {
		t.Errorf("哑化抬头正则后应 rc=1（全判缺抬头），得 %d —— 本条即「有牙」证据", rc)
	}
}

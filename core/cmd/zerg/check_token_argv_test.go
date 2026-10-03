// check_token_argv_test.go —— `check_token_argv.go`（`check-token-argv.py` 的 Go 真搬）的**成对负控**测试。
//
// 纪律：
//
//	· **成对**：正控（干净件 ⇒ rc=0）＋ 逐型负控（五型逐型 rc=1）＋ 缺件/空转/豁免钳（rc=2）—— ✗ 禁恒绿。
//	· **夹具全在 `t.TempDir()`**（仓外）· **自回收**（`t.TempDir` 由测试框架回收；自检档另验 delta=0）。
//	· **自指处置**：本测试件也是 `.go`、落 `core/cmd/zerg/` ⇒ 落在被判件自己的 read 面（登记档）内 ⇒
//	  凡危险字面（令牌选项名 / 令牌头名 / `<string>` …）**一律拼串**，✗ 不写真引号连续形态（见实现件标准声明 (b)）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ctaTestRun —— 进程内复用入口（同 argv · 取三面）。
func ctaTestRun(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkTokenArgvGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func ctaTestWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("造目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
}

// TestCtaJoinAbsoluteSegment —— 标准④：`ctaPyJoin` 逐字对齐 `os.path.join`（三处语义差坐实）。
func TestCtaJoinAbsoluteSegment(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"后段绝对 ⇒ 丢弃前面所有段", []string{"repo", "/abs"}, "/abs"},
		{"中间 `..` 不被归一", []string{"a/../b", "c"}, "a/../b/c"},
		{"空段跳过（不产双斜杠）", []string{"a", "", "b"}, "a/b"},
		{"前段带尾斜杠", []string{"a/", "b"}, "a/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ctaPyJoin(c.in...); got != c.want {
				t.Fatalf("ctaPyJoin(%v)=%q want %q（标准④：Join ⟷ os.path.join 不得混用）", c.in, got, c.want)
			}
		})
	}
}

// TestCheckTokenArgvGate_NegCases —— 逐型负控（五型逐型 rc=1 且档名相符）。
func TestCheckTokenArgvGate_NegCases(t *testing.T) {
	for _, c := range ctaNegCases() {
		c := c
		t.Run("负控_"+c.tag+"_"+c.name, func(t *testing.T) {
			sub := filepath.Join(t.TempDir(), "fx")
			ctaTestWrite(t, sub, c.fn, c.body)
			rc, out, _ := ctaTestRun("--root", sub)
			if rc != 1 {
				t.Fatalf("负控未判红：rc=%d want 1\n%s", rc, out)
			}
			if !strings.Contains(out, "["+c.tag+"]") {
				t.Fatalf("命中未点名档 %s\n%s", c.tag, out)
			}
		})
	}
}

// TestCheckTokenArgvGate_PosCases —— 正控（干净件 ⇒ rc=0 且判红 0 处）。
func TestCheckTokenArgvGate_PosCases(t *testing.T) {
	for _, c := range ctaPosCases() {
		c := c
		t.Run("正控_"+c.name, func(t *testing.T) {
			sub := filepath.Join(t.TempDir(), "fx")
			ctaTestWrite(t, sub, c.fn, c.body)
			rc, out, _ := ctaTestRun("--root", sub)
			if rc != 0 {
				t.Fatalf("正控未归零：rc=%d want 0\n%s", rc, out)
			}
			if strings.Contains(out, "  ✗ ") {
				t.Fatalf("正控出现判红行\n%s", out)
			}
		})
	}
}

// TestCheckTokenArgvGate_EmptyMissingExempt —— 空转 / 缺件 / 豁免钳 ⇒ 一律 rc=2（不许当绿）· 有牙。
func TestCheckTokenArgvGate_EmptyMissingExempt(t *testing.T) {
	t.Run("空转_零脚本单元件_rc2", func(t *testing.T) {
		sub := filepath.Join(t.TempDir(), "fx")
		ctaTestWrite(t, sub, "a.md", "# 只有登记面件\n")
		if rc, _, _ := ctaTestRun("--root", sub); rc != 2 {
			t.Fatalf("空转未退 2：rc=%d（零脚本/单元件不许当绿）", rc)
		}
	})
	t.Run("缺件_扫描根不存在_rc2", func(t *testing.T) {
		if rc, _, _ := ctaTestRun("--root", filepath.Join(t.TempDir(), "nope")); rc != 2 {
			t.Fatalf("缺件未退 2：rc=%d", rc)
		}
	})
	t.Run("豁免钳_表长2_rc2", func(t *testing.T) {
		sub := filepath.Join(t.TempDir(), "fx")
		ctaTestWrite(t, sub, "x.sh", "#!/bin/sh\ntrue\n")
		ctaExemptQs = append(ctaExemptQs, ctaExempt{"x/y.sh", "sh"})
		defer func() { ctaExemptQs = ctaExemptQs[:len(ctaExemptQs)-1] }()
		if rc, _, _ := ctaTestRun("--root", sub); rc != 2 {
			t.Fatalf("豁免表长>1 未自废：rc=%d want 2", rc)
		}
	})
}

// TestCheckTokenArgvGate_UnreadableRed —— ⑨ B 类真口子**已治**：读不了的判红面件
// ⇒ **登记 + 判红（不给结论 · rc=2）+ 逐件点名**（读不到不许当健康 ·
// 体例照同包已治件 `check_unsubstituted_placeholders.go:390-405`：件读不了 ⇒ 点名 ⇒ 按红记）。
func TestCheckTokenArgvGate_UnreadableRed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视 chmod 000 ⇒ 本用例不适用")
	}
	sub := filepath.Join(t.TempDir(), "fx")
	ctaTestWrite(t, sub, "ok.sh", "#!/bin/sh\ncurl -m 4 \"$U\"\n")
	locked := filepath.Join(sub, "locked.sh")
	ctaTestWrite(t, sub, "locked.sh", "#!/bin/sh\ncurl -m 4 \"$U\"\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o644) }()
	rc, out, _ := ctaTestRun("--root", sub)
	if rc != 2 {
		t.Fatalf("读不了的判红面件应**不给结论**（rc=2 · 读不到不许当健康）：rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "判红面件 2 个") {
		t.Fatalf("读不了的件仍应计入判红面件数（分母守恒）\n%s", out)
	}
	if !strings.Contains(out, "locked.sh") {
		t.Fatalf("读不了的件**必须被点名**（不许静默跳过）\n%s", out)
	}
	if !strings.Contains(out, "UNREADABLE") {
		t.Fatalf("读不了的件应按「件读不了」档点名\n%s", out)
	}
}

// TestCheckTokenArgvGate_JSONShapes —— `--json` 面形态：空集合**显式 `[]`**（非 null / 非缺键）。
func TestCheckTokenArgvGate_JSONShapes(t *testing.T) {
	t.Run("空集合_judged_sites为[]", func(t *testing.T) {
		sub := filepath.Join(t.TempDir(), "fx")
		ctaTestWrite(t, sub, "ok.sh", "#!/bin/sh\ncurl -H @- \"$U\"\n")
		rc, out, _ := ctaTestRun("--root", sub, "--json")
		if rc != 0 {
			t.Fatalf("rc=%d want 0", rc)
		}
		if !strings.Contains(out, `"judged": 0`) || !strings.Contains(out, `"judged_sites": []`) {
			t.Fatalf("空集合必须显式 `[]`（✗ 不得 null / ✗ 不得没了键）\n%s", out)
		}
	})
	t.Run("有命中_judged_sites逐处", func(t *testing.T) {
		sub := filepath.Join(t.TempDir(), "fx")
		ctaTestWrite(t, sub, "h.sh", "#!/bin/sh\ncurl -s -H \"X-Auth-"+"Token: *** \" \"$U\"\n")
		rc, out, _ := ctaTestRun("--root", sub, "--json")
		if rc != 1 {
			t.Fatalf("rc=%d want 1", rc)
		}
		if !strings.Contains(out, `"shape": "HDR-ARGV"`) {
			t.Fatalf("命中未落进 judged_sites\n%s", out)
		}
	})
}

// TestCheckTokenArgvGate_SelfTest —— 自检全过（rc=0）＋ 夹具自回收（TMPDIR 前后 delta=0）。
func TestCheckTokenArgvGate_SelfTest(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	before := ctaCountSelfTestDirs(t)
	rc, out, _ := ctaTestRun("--self-test")
	after := ctaCountSelfTestDirs(t)
	if rc != 0 {
		t.Fatalf("自检未全过：rc=%d\n%s", rc, out)
	}
	if after != before {
		t.Fatalf("自检夹具未回收：TMPDIR 前 %d / 后 %d（delta≠0）", before, after)
	}
}

// ctaCountSelfTestDirs —— 数 TMPDIR 里本件自检前缀的残留目录（自回收之证）。
func ctaCountSelfTestDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("读 TMPDIR 失败：%v", err)
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "q207-selftest-") {
			n++
		}
	}
	return n
}

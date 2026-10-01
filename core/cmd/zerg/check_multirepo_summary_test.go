// check_multirepo_summary_test.go —— `core/cmd/zerg/check_multirepo_summary.go`（M2 真搬件）的成对负控。
//
// 目的：给 `checkMultirepoSummaryGateMain` 一道**带牙**的回归 —— 哑化任一条判据（写词预扫 / 只读子命令白名单 /
// 只读自证差异检测 / 缺件 fail-closed / JSON 形状）⇒ 必有一格 FAIL。全部合成夹具走 `t.TempDir()`（仓外，不碰真目标）。
package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// mrsTEnv —— 进程内取 (rc, stdout, stderr)。
func mrsTEnv(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkMultirepoSummaryGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// mrsTFix —— 造两仓夹具（主仓脏 1／未跟踪 1 · 文档仓干净），回 (主仓, 文档仓)。
func mrsTFix(t *testing.T) (string, string) {
	t.Helper()
	d := t.TempDir()
	mainRepo := mrsMkRepo(d, "主仓", []string{"a.txt"}, true)
	docs := mrsMkRepo(d, "Zerg-内部文档", []string{"x.md"}, false)
	return mainRepo, docs
}

// mrsTWantCases —— 子用例条数（写死：少跑一格即断言失败）。
const mrsTWantCases = 12

func TestCheckMultirepoSummaryGatePort(t *testing.T) {
	cases := 0
	sub := func(name string, fn func(t *testing.T)) {
		cases++
		t.Run(name, fn)
	}

	sub("正控 两仓汇总 rc=0", func(t *testing.T) {
		m, d := mrsTFix(t)
		rc, out, _ := mrsTEnv("--root", m, "--docs-root", d, "--source", "git")
		if rc != 0 || !strings.Contains(out, "脏件数") || !strings.Contains(out, "主仓") || !strings.Contains(out, "文档仓") {
			t.Fatalf("正控失败：rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "两仓合计：未提交 2 处（脏 1 · 未跟踪 1）") {
			t.Fatalf("计数档失败：out=%q", out)
		}
	})

	sub("负控 写旗标一律拒 rc=2（真拒：HEAD/index 不变）", func(t *testing.T) {
		m, d := mrsTFix(t)
		gd, _ := mrsGitDir(m)
		idx0, _ := mrsFileSha(mrsPyJoin(gd, "index"))
		h0, _ := mrsGitRo(m, []string{"rev-parse", "HEAD"}, 60*time.Second)
		for _, flag := range []string{"--commit", "add", "--push=origin", "commit"} {
			rc, out, _ := mrsTEnv("--root", m, "--docs-root", d, flag)
			if rc != 2 || !strings.Contains(out, "拒") {
				t.Fatalf("写旗标 %q 未拒：rc=%d out=%q", flag, rc, out)
			}
		}
		idx1, _ := mrsFileSha(mrsPyJoin(gd, "index"))
		h1, _ := mrsGitRo(m, []string{"rev-parse", "HEAD"}, 60*time.Second)
		if idx0 != idx1 || h0 != h1 {
			t.Fatalf("拒写后夹具被改动：index %s->%s head %s->%s", idx0, idx1, h0, h1)
		}
	})

	sub("负控 内部牙 白名单外子命令抛 Refused", func(t *testing.T) {
		m, _ := mrsTFix(t)
		if _, err := mrsGitRo(m, []string{"commit", "-m", "x"}, 60*time.Second); err == nil {
			t.Fatal("git_ro 未拒 commit")
		} else {
			var rf mrsRefused
			if !errors.As(err, &rf) {
				t.Fatalf("错的拒态类型：%T %v", err, err)
			}
		}
	})

	sub("正控 只读自证窄口径 差异 0 ⇒ rc=0", func(t *testing.T) {
		m, d := mrsTFix(t)
		rc, out, _ := mrsTEnv("--root", m, "--docs-root", d, "--source", "git", "--prove-readonly")
		if rc != 0 || !strings.Contains(out, "零副作用") {
			t.Fatalf("窄口径失败：rc=%d out=%q", rc, out)
		}
	})

	sub("正控 只读自证宽口径 tree 差异 0 ⇒ rc=0", func(t *testing.T) {
		m, d := mrsTFix(t)
		rc, out, _ := mrsTEnv("--root", m, "--docs-root", d, "--source", "git", "--prove-readonly=tree")
		if rc != 0 || !strings.Contains(out, "零副作用") {
			t.Fatalf("宽口径失败：rc=%d out=%q", rc, out)
		}
	})

	sub("负控 只读自证 窗口内被改 ⇒ 检测器抓到（差异非 0）", func(t *testing.T) {
		_, d := mrsTFix(t)
		before := mrsRepoFingerprint(d, true)
		fh, err := os.OpenFile(mrsPyJoin(d, "x.md"), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fh.WriteString("改\n")
		fh.Close()
		after := mrsRepoFingerprint(d, true)
		if got := mrsDiffFingerprint(before, after); len(got) == 0 {
			t.Fatal("检测器漏报窗口内改动")
		}
	})

	sub("正控 同件前后各取一次 ⇒ 差异 0 件", func(t *testing.T) {
		_, d := mrsTFix(t)
		a := mrsRepoFingerprint(d, true)
		b := mrsRepoFingerprint(d, true)
		if got := mrsDiffFingerprint(a, b); len(got) != 0 {
			t.Fatalf("乱报：%v", got)
		}
	})

	sub("负控 --json 形状 zerg/v1 items 2", func(t *testing.T) {
		m, d := mrsTFix(t)
		rc, out, _ := mrsTEnv("--root", m, "--docs-root", d, "--source", "git", "--json")
		var jd map[string]any
		if err := json.Unmarshal([]byte(out), &jd); err != nil {
			t.Fatalf("非 JSON：%v out=%q", err, out)
		}
		if rc != 0 {
			t.Fatalf("rc=%d", rc)
		}
		m2 := jd["meta"].(map[string]any)
		if jd["schema"] != "zerg/v1" || int(m2["count"].(float64)) != 2 || int(m2["dirty"].(float64)) != 1 {
			t.Fatalf("形状不符：%s", out)
		}
		if !strings.Contains(out, "\"proof\": null") {
			t.Fatalf("proof 该为 null：%s", out)
		}
	})

	sub("负控 缺件 非仓 ⇒ rc=2 不给结论", func(t *testing.T) {
		_, d := mrsTFix(t)
		rc, out, _ := mrsTEnv("--root", mrsPyJoin(t.TempDir(), "没有这个仓"), "--docs-root", d, "--source", "git")
		if rc != 2 || !strings.Contains(out, "不是 git 仓") {
			t.Fatalf("缺件未 fail-closed：rc=%d out=%q", rc, out)
		}
	})

	sub("负控 未知旗标 ⇒ rc=2 usage 进 stderr", func(t *testing.T) {
		rc, out, se := mrsTEnv("--bogus")
		if rc != 2 || !strings.Contains(se, "unrecognized arguments: --bogus") || out != "" {
			t.Fatalf("未知旗标档失败：rc=%d out=%q err=%q", rc, out, se)
		}
	})

	sub("正控 --list-rules 打闭集", func(t *testing.T) {
		rc, out, _ := mrsTEnv("--list-rules")
		if rc != 0 || !strings.Contains(out, "写词闭集") || !strings.Contains(out, "只读 git 子命令白名单") {
			t.Fatalf("--list-rules 失败：rc=%d out=%q", rc, out)
		}
	})

	sub("正控 -h usage 首行", func(t *testing.T) {
		rc, out, _ := mrsTEnv("-h")
		if rc != 0 || !strings.HasPrefix(out, "usage: check-multirepo-summary.py") {
			t.Fatalf("-h 失败：rc=%d out=%q", rc, out[:min(40, len(out))])
		}
	})

	if cases != mrsTWantCases {
		t.Fatalf("子用例条数 %d ≠ 期望 %d", cases, mrsTWantCases)
	}
}

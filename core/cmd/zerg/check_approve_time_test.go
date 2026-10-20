// check_approve_time_test.go —— `check_approve_time.go` 的**成对**测试件（✗ 禁恒绿）。
//
// 覆盖面（逐条对应 py 的判据）：
//
//	✓ 自检六格（①正控 ②R1负控 ③R2负控 ④更早世代+跨机正控 ⑤件缺 ⑥提交表缺）经入口真跑
//	✓ 运行档 A/B/C 三处读数 + 逐行排序 + 机器判不了 + 退码三档
//	✓ **时刻边界**（AtEqMtime 下界闭 / AtEqApproved 上界开 / ApprovedMinus1 落窗）★ 有牙
//	✓ `--json` 前置拒收（rc=2 · 三行 stderr · 先于 argparse）
//	✓ `-h/--help` 逐字（Python 3.9 argparse · `optional arguments:`）
//	✓ 未知旗标 / 缺值 / 歧义 / `--json=` 形态
//	✓ `apvtParseISO` 语义（3.9 口径：`Z` 已前置替换 · 无时区⇒false · `+0800`⇒false · `.1`⇒false）
//	✓ `apvtPyJoin` 三语义差（✗ 不是 filepath.Join）
//
// ★ 负控有牙：见 TestCheckApproveTimeTimeBoundaries 的 b1 —— 若把 R1 下界从「闭」改成「开」
// （`apvtJudge` 里把 `!mtime.After(when)` 改成 `when.After(mtime)`），该子用例立刻红并带行号。
// ★ 夹具全落 `t.TempDir()`（自动回收）· 不碰真状态目录 · 不联网。
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const apvtTApproved = "2026-09-21T20:53:35+08:00"

// apvtTRepo 造一枚定日期的 git 仓（提交时刻钉死 ⇒ `%cI` 可复现）。
func apvtTRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不在 PATH")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = repo
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for i, date := range []string{"2026-09-21T20:00:00+08:00", "2026-09-22T10:00:00+08:00"} {
		name := fmt.Sprintf("f%d.txt", i)
		if err := os.WriteFile(filepath.Join(repo, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", name)
		c := exec.Command("git", "commit", "-q", "-m", fmt.Sprintf("c%d", i))
		c.Dir = repo
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("commit: %v\n%s", err, out)
		}
	}
	return repo
}

// apvtTState 造状态目录：件 mtime = 签发时刻 + shift 秒；审计行显式给 `at`。
func apvtTState(t *testing.T, shiftSeconds int, ats []string) string {
	t.Helper()
	st := t.TempDir()
	if err := os.MkdirAll(filepath.Join(st, "approvals"), 0o755); err != nil {
		t.Fatal(err)
	}
	tp := filepath.Join(st, "approvals", apvtDefaultTicket+".json")
	ticket := "{\n \"tool\": \"dev_edit\",\n \"approver\": \"Mr2109\",\n \"%s\": \"%s\",\n \"scope\": \"*\",\n \"note\": \"D5 夹具\",\n \"sig_alg\": \"ed25519\",\n \"key_id\": \"0000000000000000\",\n \"sig\": \"\"\n}\n"
	if err := os.WriteFile(tp, []byte(fmt.Sprintf(ticket, apvtKeyApprovedAt, apvtTApproved)), 0o644); err != nil {
		t.Fatal(err)
	}
	base, ok := apvtParseISO(apvtTApproved)
	if !ok {
		t.Fatal("夹具签发时刻解不出")
	}
	mt := base.Add(time.Duration(shiftSeconds) * time.Second)
	if err := os.Chtimes(tp, mt, mt); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, at := range ats {
		b.WriteString(fmt.Sprintf("{%q: %q, \"file\": \"x\", %q: %q}\n",
			apvtKeyAt, at, apvtKeyApproval, tp))
	}
	if err := os.WriteFile(filepath.Join(st, "edit_audit.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return st
}

// apvtHasToken ★ 按**词边界**判某个 token 出不出现。
// 不用 `strings.Contains`：门跑的私有临时目录名可能带 `R1…`（如 shard-affected-iso-core-R1ujRh），
// 裸子串匹配会把**路径里的 R1** 误判成**世代标签 R1** ⇒ 假红。判据语义不变（仍不许出现该标签），
// 只是不再被无关的字串碰瓷。
func apvtHasToken(s, tok string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], tok)
		if j < 0 {
			return false
		}
		p := i + j
		before := byte(' ')
		if p > 0 {
			before = s[p-1]
		}
		after := byte(' ')
		if p+len(tok) < len(s) {
			after = s[p+len(tok)]
		}
		if !apvtWordByte(before) && !apvtWordByte(after) {
			return true
		}
		i = p + 1
	}
}

func apvtWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func apvtTRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkApproveTimeGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckApproveTimeHelp(t *testing.T) {
	rc, out, errb := apvtTRun("-h")
	if rc != 0 || errb != "" {
		t.Fatalf("`-h`：rc=%d stderr=%q", rc, errb)
	}
	for _, want := range []string{"usage: check-approve-time.py [-h]", "optional arguments:", "--ticket TICKET", "--no-self-test"} {
		if !strings.Contains(out, want) {
			t.Errorf("帮助正文缺 %q", want)
		}
	}
	if rcLong, outLong, _ := apvtTRun("--help"); rcLong != 0 || outLong != out {
		t.Errorf("`--help` 与 `-h` 应逐字同：rc=%d", rcLong)
	}
}

func TestCheckApproveTimeJSONRefused(t *testing.T) {
	rc, out, errb := apvtTRun("--json")
	if rc != 2 || out != "" {
		t.Fatalf("`--json`：rc=%d stdout=%q（应 rc=2 · stdout 空）", rc, out)
	}
	if !strings.Contains(errb, "缺口 Q-176") || !strings.Contains(errb, "字段表：无（本门只出人读面）") {
		t.Errorf("拒收正文不合：%q", errb)
	}
	// ★ 先于 argparse：即便同给 `-h` 也走拒收（py 同序）。
	if rc2, _, _ := apvtTRun("-h", "--json=1"); rc2 != 2 {
		t.Errorf("`--json=1` 应先于 argparse 拒收：rc=%d", rc2)
	}
}

func TestCheckApproveTimeUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"未知旗标", []string{"--bogus"}, "unrecognized arguments: --bogus"},
		{"缺值", []string{"--root"}, "argument --root: expected one argument"},
		{"歧义", []string{"--s", "x"}, "ambiguous option: --s could match --state, --self-test"},
		{"显式值", []string{"--self-test=1"}, "argument --self-test: ignored explicit argument '1'"},
	}
	for _, c := range cases {
		rc, out, errb := apvtTRun(c.argv...)
		if rc != 2 || out != "" {
			t.Errorf("%s：rc=%d stdout=%q（应 rc=2 · stdout 空）", c.name, rc, out)
		}
		if !strings.HasPrefix(errb, apvtUsage) || !strings.Contains(errb, c.want) {
			t.Errorf("%s：stderr=%q 应含 %q", c.name, errb, c.want)
		}
	}
}

func TestCheckApproveTimeMissingInputs(t *testing.T) {
	repo := apvtTRepo(t)
	st := t.TempDir() // 无 approvals/ ⇒ 件缺
	rc, out, _ := apvtTRun("--no-self-test", "--root", repo, "--state", st)
	if rc != 2 || !strings.Contains(out, "未取数") || !strings.Contains(out, "件不在") {
		t.Errorf("件缺应 rc=2 且 stdout 含「未取数/件不在」：rc=%d out=%q", rc, out)
	}
	// 提交表缺
	st2 := apvtTState(t, 0, nil)
	rc2, out2, _ := apvtTRun("--no-self-test", "--root", t.TempDir(), "--state", st2)
	if rc2 != 2 || !strings.Contains(out2, "没有 .git") {
		t.Errorf("仓根无 .git 应 rc=2 且 stdout 含「没有 .git」：rc=%d out=%q", rc2, out2)
	}
}

// TestCheckApproveTimeTimeBoundaries ★ 时刻面**有牙**的边界档（夹具层注入 mtime / at 造出）。
func TestCheckApproveTimeTimeBoundaries(t *testing.T) {
	repo := apvtTRepo(t)
	cases := []struct {
		name      string
		shift     int
		at        string
		wantRC    int
		wantIn    string
		wantNotIn string
	}{
		// mtime = 20:43:35（shift=-600）：窗口 = [20:43:35, 20:53:35)
		{"下界闭_AtEqMtime", -600, "2026-09-21T20:43:35+08:00", 1, "R1", ""},
		{"上界开_AtEqApproved", -600, apvtTApproved, 0, "本世代", "R1"},
		{"窗口内_ApprovedMinus1", -600, "2026-09-21T20:53:34+08:00", 1, "R1", ""},
		{"下界前1秒_更早世代", -600, "2026-09-21T20:43:34+08:00", 0, "更早世代", "R1"},
		{"无时区_R2", 0, "2026-09-21T20:54:00", 1, "R2", ""},
		{"mtime等于签发_窗口空", 0, "2026-09-21T20:53:35+08:00", 0, "本世代", "R1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := apvtTState(t, c.shift, []string{c.at})
			rc, out, _ := apvtTRun("--no-self-test", "--root", repo, "--state", st)
			if rc != c.wantRC {
				t.Fatalf("rc=%d 应 %d\nstdout:\n%s", rc, c.wantRC, out)
			}
			if !strings.Contains(out, c.wantIn) {
				t.Errorf("stdout 应含 %q\n%s", c.wantIn, out)
			}
			if c.wantNotIn != "" && apvtHasToken(out, c.wantNotIn) {
				t.Errorf("stdout 不应含 %q\n%s", c.wantNotIn, out)
			}
		})
	}
}

// TestApvtHasToken ★ 判据件自身要有牙：路径里的 R1 不许碰瓷，独立标签 R1 必须上榜。
func TestApvtHasToken(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"/var/folders/x/shard-affected-iso-core-R1ujRh/tmp/a", false},
		{"── 世代 R1 · 越界", true},
		{"R1", true},
		{"R10", false},
		{"_R1_", false},
	}
	for _, c := range cases {
		if got := apvtHasToken(c.s, "R1"); got != c.want {
			t.Errorf("apvtHasToken(%q) = %v，应 %v", c.s, got, c.want)
		}
	}
}

func TestCheckApproveTimeEmptyAuditAndRows(t *testing.T) {
	repo := apvtTRepo(t)
	st := apvtTState(t, 0, nil)
	rc, out, _ := apvtTRun("--no-self-test", "--root", repo, "--state", st)
	if rc != 0 {
		t.Fatalf("空审计应 rc=0：rc=%d\n%s", rc, out)
	}
	// ★ nil-不得兼表「取不到」：空集合照判在场（键到本件 0 行 · 全量 0 行），✗ 不退 2。
	if !strings.Contains(out, "键到本件 0 行") || !strings.Contains(out, "全量 0 行") {
		t.Errorf("空集合应照判在场：\n%s", out)
	}
	// >6 行裁切：占位行首列是 `…`（✗ 不是 `-1`）
	st2 := apvtTState(t, 0, []string{
		"2026-09-21T21:00:00+08:00", "2026-09-21T21:01:00+08:00", "2026-09-21T21:02:00+08:00",
		"2026-09-21T21:03:00+08:00", "2026-09-21T21:04:00+08:00", "2026-09-21T21:05:00+08:00",
		"2026-09-21T21:06:00+08:00",
	})
	_, out2, _ := apvtTRun("--no-self-test", "--root", repo, "--state", st2)
	if !strings.Contains(out2, "   行 … · at … · 世代 …") {
		t.Errorf(">6 行的占位行首列应为 `…`：\n%s", out2)
	}
	if strings.Contains(out2, "   行 -1 ·") {
		t.Errorf("占位行首列不得为 -1：\n%s", out2)
	}
}

func TestCheckApproveTimeSelfTestGreen(t *testing.T) {
	repo := apvtTRepo(t)
	st := apvtTState(t, 0, []string{"2026-09-21T21:00:00+08:00", "2026-09-21T12:00:00+08:00"})
	rc, out, errb := apvtTRun("--self-test", "--root", repo, "--state", st)
	if rc != 0 || errb != "" {
		t.Fatalf("自检应 rc=0：rc=%d stderr=%q\n%s", rc, errb, out)
	}
	if !strings.Contains(out, "自检：6 格过 · 0 格不过") {
		t.Errorf("自检格数不合：\n%s", out)
	}
}

func TestCheckApproveTimeParseISOSemantics(t *testing.T) {
	ok := []string{"2026-09-21T20:53:35+08:00", "2026-09-21T20:53:35Z", "2026-09-21X20:53:35+08:00", "2026-09-21T20:53:35+08:00:00"}
	bad := []string{"2026-09-21T20:53:35", "2026-09-21T20:53:35+0800", "2026-09-21T20:53:35+08", "2026-09-21T20:53:35.1+08:00", "20260921T205335", " 2026-09-21T20:53:35", "2026-09-21"}
	for _, s := range ok {
		if _, k := apvtParseISO(s); !k {
			t.Errorf("应认（带时区）：%q", s)
		}
	}
	for _, s := range bad {
		if _, k := apvtParseISO(s); k {
			t.Errorf("应拒（无时区/坏形）：%q", s)
		}
	}
}

func TestCheckApproveTimePyJoinSemantics(t *testing.T) {
	if got := apvtPyJoin("a", "../b"); got != "a/../b" {
		t.Errorf("`..` 段应原样留：%q", got)
	}
	if got := apvtPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("后段绝对应重置：%q", got)
	}
	if got := apvtPyJoin("a", ""); got != "a/" {
		t.Errorf("空尾段应留分隔符：%q", got)
	}
	if got := apvtPyNormpath("//a/b"); got != "//a/b" {
		t.Errorf("两个前导斜杠应保留：%q", got)
	}
}

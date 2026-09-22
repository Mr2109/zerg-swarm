// cli_core_proc_test.go —— `core ps`（`Q-056`）+ `core daemon ls --declared` 两列（`Q-057`）的判据机检（波① `T2`）。
//
// 判据（`任务清单-缺口收口-20260923.md` §`T2` 原样四条 · 逐条落成断言）：
//
//	① `zerg core ps` rc=0 且**三格齐**（pid / 起时 / 命令行）
//	② `declared` 列**非全否**（拿脚本的仓根相对路径去比声明件的「程序」列 —— 今天的病根就在这一处）
//	③ **同源**：幽灵条数 == `zerg doctor` 的「回收候选（幽灵服务）」条数（逐字同值）
//	④ **只读**：跑前后状态目录逐件不变（本件用 `t.TempDir()` 沙箱 + 逐件 `sha256` 对拍）
//
// 夹具口径与门⑧ `check-service-declaration.py` **同名同义**：`ZERG_SVCDECL_DECL`（声明件）、
// `ZERG_SVCDECL_PS`（ps 输出落文件 · 门⑧ 形状 = `pid ppid etime command`）、`ZERG_REPO`（仓根）。
// **不碰真机状态**：主控地址一律指到一个没人听的端口（`ZERG_PORT=1`）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	procFixtureDecl = "kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n" +
		"launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n" +
		"ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧自带服务\t进 reap 候选（**不是**自动候选）· 停它=不可逆 ⇒ 待拍\n"

	// 门⑧ 形状（`ps -eo pid,ppid,etime,command`）：第二格是 ppid、第三格是 etime。
	procFixturePS = "  111     1 01:02:03 /opt/x/bin/zerg-core\n" +
		"  222     1 04:05:06 /opt/x/bin/cocoon-docs-service -port 8610\n" +
		"  333     1 07:08:09 /usr/sbin/cupsd\n"
)

// procFixture 造夹具：仓根（scripts/svc/*.sh + deploy/服务声明.tsv）+ 声明件 + ps 落文件。
func procFixture(t *testing.T) (repo, decl, ps string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "scripts", "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"zerg-core-daemon.sh", "watchdog.sh"} {
		if err := os.WriteFile(filepath.Join(repo, "scripts", "svc", s), []byte("#!/bin/bash\ntrue\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(repo, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	decl = filepath.Join(repo, "deploy", "服务声明.tsv")
	if err := os.WriteFile(decl, []byte(procFixtureDecl), 0o644); err != nil {
		t.Fatal(err)
	}
	ps = filepath.Join(repo, "ps.txt")
	if err := os.WriteFile(ps, []byte(procFixturePS), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_SVCDECL_DECL", decl)
	t.Setenv("ZERG_SVCDECL_PS", ps)
	t.Setenv("ZERG_PORT", "1") // 没人听：本族不该碰主控
	return repo, decl, ps
}

// 判据 ① `core ps`：rc=0 且三格齐（夹具里两件自研件：launchd 声明的 + ghost 的）。
func TestCorePsThreeColumns(t *testing.T) {
	procFixture(t)
	rc, out, errb := runCapture("core", "ps")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 { // 表头 + 2 条
		t.Fatalf("行数 %d（要 3 = 表头 + 2 条）· 输出：\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "pid\tstart\tcommand") {
		t.Fatalf("表头不是 pid/start/command：%q", lines[0])
	}
	for _, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) < 3 || f[0] == "" || f[1] == "" || f[2] == "" {
			t.Fatalf("三格不齐：%q（拆开 %v）", l, f)
		}
		if !regexp.MustCompile(`^\d+$`).MatchString(f[0]) {
			t.Errorf("pid 格不是数字：%q", f[0])
		}
	}
	if !strings.Contains(out, "bin/zerg-core") || !strings.Contains(out, "cocoon-docs-service") {
		t.Fatalf("声明件点名的两件没都列出：\n%s", out)
	}
}

// 判据 ② `declared` 列非全否（修好匹配口径：拿脚本仓根相对路径比「程序」列）。
func TestDaemonLsDeclaredNotAllNo(t *testing.T) {
	procFixture(t)
	rc, out, errb := runCapture("core", "daemon", "ls")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	yes, no := 0, 0
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n")[1:] {
		f := strings.Split(l, "\t")
		if len(f) < 3 {
			continue
		}
		switch f[2] {
		case "是":
			yes++
		case "否":
			no++
		}
	}
	if yes == 0 {
		t.Fatalf("`declared` 列**全否**（要非全否 —— 这就是 `Q-057` 的病根）：\n%s", out)
	}
	if no == 0 {
		t.Errorf("两条脚本里另一条该是「否」，却全「是」：\n%s", out)
	}
	if !strings.Contains(out, "zerg-core-daemon.sh\t是") {
		t.Errorf("点名的那条脚本没落在「是」上：\n%s", out)
	}
}

// 判据 ③ **同源**：`--declared` 的幽灵条数 == `doctor` 的「回收候选（幽灵服务）」条数（逐字同值）。
func TestDaemonLsGhostCountSameSourceAsDoctor(t *testing.T) {
	procFixture(t)
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)

	rc, out, errb := runCapture("core", "daemon", "ls", "--declared", "--json", "name,script,declared,note")
	if rc != 0 {
		t.Fatalf("daemon ls --declared rc=%d（要 0）· stderr=%s", rc, errb)
	}
	// 侧①：幽灵段条数（stderr 那一行 + 行面里 `否（幽灵）` 的条数，两处必须一致）
	m := regexp.MustCompile(`幽灵条数 (\d+)`).FindStringSubmatch(errb)
	if m == nil {
		t.Fatalf("stderr 里没有「幽灵条数 N」：%s", errb)
	}
	side1 := strings.Count(out, "否（幽灵）")
	if side1 == 0 {
		t.Fatalf("机读面里没有幽灵行：%s", out)
	}
	// 侧②：`doctor` 的同一格（逐字同值）
	drc, dout, derr := runCapture("doctor", "--json", "name,verdict,detail,advice")
	if drc != 0 {
		t.Fatalf("doctor rc=%d（要 0）· stderr=%s", drc, derr)
	}
	dm := regexp.MustCompile(`回收候选（幽灵服务）[^}]*?(\d+) 条`).FindStringSubmatch(dout)
	if dm == nil {
		t.Fatalf("doctor 机读面里找不到「回收候选（幽灵服务）…N 条」：%s", dout)
	}
	if m[1] != dm[1] || side1 != atoi(dm[1]) {
		t.Fatalf("**不同源**：`daemon ls --declared` 幽灵条数=%s（行面 %d 条）· doctor=%s 条", m[1], side1, dm[1])
	}
	if !strings.Contains(dout, "pid=222") {
		t.Errorf("doctor 的幽灵条没点名夹具里的 pid：%s", dout)
	}
}

// 判据 ④ **只读**：跑前跑后状态目录（+ 仓根夹具）逐件 `sha256` 不变。
func TestCorePsReadOnlyOnStateDir(t *testing.T) {
	repo, _, _ := procFixture(t)
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	// 造一点"状态"：夹具状态目录里放两件（跑命令不该动它们）
	if err := os.MkdirAll(filepath.Join(state, "approvals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "approvals", "a.json"), []byte(`{"k":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "models.yaml"), []byte("m: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := func() map[string]string {
		out := map[string]string{}
		for _, root := range []string{state, repo} {
			_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				b, _ := os.ReadFile(p)
				sum := sha256.Sum256(b)
				out[p] = hex.EncodeToString(sum[:])
				return nil
			})
		}
		return out
	}
	before := snap()
	for _, argv := range [][]string{
		{"core", "ps"},
		{"core", "ps", "--json", "pid,start,command"},
		{"core", "daemon", "ls"},
		{"core", "daemon", "ls", "--declared"},
	} {
		if rc, _, errb := runCapture(argv...); rc != 0 {
			t.Fatalf("%v rc=%d（要 0）· stderr=%s", argv, rc, errb)
		}
	}
	after := snap()
	if len(before) != len(after) {
		t.Fatalf("件数变了：%d → %d（只读破功）", len(before), len(after))
	}
	for p, h := range before {
		if after[p] != h {
			t.Fatalf("%s 被改了：%s → %s（只读破功）", p, h, after[p])
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

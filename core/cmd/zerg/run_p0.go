// run_p0.go —— 判据件 `run_p0`：虫族对话 **P0 静态契约套件** 的 Go 真搬（inproc 标准路由）。
//
// 底层原件 = `scripts/chat-eval/run_p0.py`（121 行 · 11 条静态契约 ＋ 1 条副作用面 ＋ 内嵌元测试）。
// 本件把它的**判定体**收进进程内（✗ 不再经 python 解释器），逐字保住三样东西：
//
//	① 用例表：11 条 (id · 说明 · argv · 期望 rc · 期望 stdout 字节数) ＋ p0-12 副作用面一条；
//	② 隔离口径：**一例一临时树**（HOME / ZERG_STATE_DIR / TMPDIR 三个旋钮全指向它 · 并剔掉
//	   ZERG_REPO / ZERG_GATE_LOCK_DIR / GIT_INDEX_FILE）⇒ 跑它**不污染真账**（真 ~/.zerg/state 零写）；
//	③ **自带元测试**：主用例跑完后把 p0-01 的期望故意改错（改判 rc==0），必须能红 —— 否则说明
//	   套件是恒绿的（无牙）⇒ 判失败。这条是套件的**牙**，不是可选项。
//
// 为什么这条属于 inproc 面（判据形态，不是动作形态）：
//
//	· 判定读数是「rc ＋ stdout 字节数」两个标量，输入确定、只读仓内制品（`bin/zerg`）；
//	· 写面只在**仓外** `os.MkdirTemp`（defer 清）⇒ 与「判据件一律零仓内写」同规；
//	· 它判的是**命令面契约**（用法错必须 rc=2 且零 stdout、零副作用），正是 `gate check` 名录
//	  里那一类「判据」的形状 ⇒ 无外部评测器、无凭据、无网络。
//
// ★ 与 py 半的关系（**诚实边界** · ✗ 不假装已退休）：
//
//	提交门的 门132 步仍逐字 `python3 scripts/chat-eval/run_p0.py`（步表在 `precommit-gates.sh`，
//	本批禁改）⇒ 现读是「**两半并存**」：inproc 路由（本件）＋ 门步（py 半）。本件因此**不声明**
//	py 半已退场，也不改它的语义；两半是否分叉由同仓防波门 `check-py-go-pairing-drift` 看着 ——
//	它对「键在 inproc 表 ∧ py 在盘」的键逐对跑 `gate check <键> --self-test` ⟷ `python3 <py>
//	--self-test` 并比 rc/stdout/stderr ⇒ 本件的 `--self-test` 输出必须与 py **逐字节同**
//	（这就是下面「对拍面」一条的由来）。要真退场，先把门132 步表那行改指 inproc（另路提交）。
//
// 对拍面（**逐字**）：
//
//	· `--self-test`：py 只跑元测试并打印 `元测试：负控已红（N 条）⇒ 套件有牙 ✓`（N = 把 p0-01
//	  期望改错后**仍然红**的条数：制品全合规时 N 恒 1；制品本来就坏 ⇒ N 更大；N=0 ⇒ 套件无牙，
//	  打印 `✗ 元测试没红 ⇒ 套件是恒绿的（无牙）` 并判失败）。本件逐字照抄（含全角括号/✓）。
//	· 默认档逐条行：`PASS p0-01 chat ask --json 缺值  rc=2 (want 2 ) stdout=0    (want 0    )`
//	  （格式串同 py 的 `%s %-4s %-28s rc=%-2s(want %-2s) stdout=%-5s(want %-5s)`）；
//	  副作用面行：`PASS p0-12 用法错零副作用（不许建会话/登记别名）（rc=2 · 新增落盘=[]）`；
//	  收尾行：`P0 合计 11 条 · 失败 0 条（副作用面另 1 条 · 失败 0 条）`。
//
// 已声明的三处**非判定面**差异（都只在「跑不起来 / 给错旗标」档 ⇒ 不动任何一条判据）：
//
//	(a) 缺 `bin/zerg`：py 抛 `FileNotFoundError`（traceback · rc=1）；本件 fail-closed 退 **2**
//	    （「不给结论」≠「判红」—— 制品不在盘时 12 条全 fails 是把环境的错记成契约的错）。
//	(b) `-h` / `--help`：本件有用法串（py 无 · 它把 `--help` 当普通 argv 照跑）。
//	(c) 未知旗标 ⇒ **2**（py 静默忽略）。家族同形（与他判据件一致）· 且 drift 门的 probe argv
//	    恒为 `--self-test` ⇒ 这三处不影响对拍面。
//
// 退码：0 全绿 ／ 1 判定红（有用例失败 · 或元测试没红=无牙）／ 2 不给结论（仓根取不到 ·
// 制品 `bin/zerg` 不在盘/不可执行 · 用法错）。
//
// 覆盖面的**入表纪律**（本件落地时现读到的两条硬形状，✗ 别违）：
//
//	· 本键在 `gate_inproc_table.go` 里**只占一行、不带行尾注释** —— 防波门
//	  `check-py-go-pairing-drift` 是按**行形状**现读那张表的（tab ＋ 键 ＋ 冒号 ＋ 符号 ＋ 逗号收行），行尾一带字
//	  就看不见这枚键 ⇒ 那一对**静默不参与两态对拍**（＝两半分叉无人看）。
//	· 也不要在表中间插注释行：那会把 gofmt 的对齐分组切断，尾部 10 行被重新对齐（纯噪声 diff）。
//
// 自指检查：本件是 `.go` ⇒ 落不进 `check-cli-ownership` 自己的候选闭集（只收 `.py`/`.sh`）；
// 本件 read-only（默认档不写任何仓内件）· 正文零私有绝对路径字面 ⇒ 不进私有面判据的扫描面。
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// rp0JudgeKey —— 判据键名（＝件名去扩展名 · 与名录/inproc 同口径）。
	rp0JudgeKey = "run_p0"
	// rp0Prog —— 件名（报话前缀）。
	rp0Prog = "run_p0"
	// rp0BinRel —— 被判制品的仓内相对路径（py `ZERG = os.path.join(ROOT, "bin", "zerg")`）。
	rp0BinRel = "bin/zerg"
	// rp0CaseTimeout —— 单例时限（py `timeout=30`）。
	rp0CaseTimeout = 30 * time.Second
	// rp0WantNoBytes —— 期望 stdout 字节数 = 「不比」（py 的 `None`）。
	rp0WantNoBytes = -1
)

// rp0UsageText —— 用法串（家族同型）。
const rp0UsageText = `usage: run_p0 [--check | --self-test] [--root <根>] [-h|--help]

  （默认/--check）跑 P0 静态契约套件（11 条 ＋ 副作用面 1 条 ＋ 内嵌元测试）
  --self-test     只跑元测试（负控必须红 ⇒ 证套件有牙）
  --root <根>     仓根（缺省：ZERG_REPO_ROOT → ZERG_REPO → 可执行/当前目录上溯）
`

// rp0Case —— 一条用例（py CASES 的一行：(id, 说明, argv, want_rc, want_stdout_bytes)）。
type rp0Case struct {
	ID     string
	Desc   string
	Argv   []string
	WantRC int
	// WantBytes —— 期望 stdout 字节数；rp0WantNoBytes（-1）= py 的 `None`（不比）。
	WantBytes int
}

// rp0Cases —— 11 条静态契约（逐字照 py 的 CASES 表 · 顺序亦照）。
//
// ★ 口径：全部是**用法错**面 —— 期望 rc=2 且 **stdout 零字节**（错必须落 stderr，✗ 不许吐半截
// 机器面）；p0-09/10 两条 `--help` 反过来必须成功（rc=0）且不比字节数。
var rp0Cases = []rp0Case{
	{"p0-01", "chat ask --json 缺值", []string{"chat", "ask", "--json"}, 2, 0},
	{"p0-02", "chat send 三态缺失", []string{"chat", "send", "hi"}, 2, 0},
	{"p0-03", "别名空串", []string{"chat", "ask", "--as", "", "hi"}, 2, 0},
	{"p0-04", "--timeout 非法时长", []string{"chat", "ask", "--as", "x", "--timeout", "nope", "hi"}, 2, 0},
	{"p0-05", "prompt-file 不存在", []string{"chat", "ask", "--as", "x", "--prompt-file", "/no/such/f"}, 2, 0},
	{"p0-06", "chat alias ls --json 给错字段", []string{"chat", "alias", "ls", "--json", "nope"}, 2, 0},
	{"p0-07", "未知子命令", []string{"chat", "frobnicate"}, 2, 0},
	{"p0-08", "已知旗标给到别的命令", []string{"chat", "send", "--as", "x", "hi"}, 2, 0},
	{"p0-09", "chat --help 应成功", []string{"chat", "--help"}, 0, rp0WantNoBytes},
	{"p0-10", "chat ask --help 应成功", []string{"chat", "ask", "--help"}, 0, rp0WantNoBytes},
	{"p0-11", "--new 与 --last 冲突", []string{"chat", "send", "--new", "--last", "hi"}, 2, 0},
}

// rp0CleanCase —— 副作用面那一条（py CLEAN_CASES 的唯一项）：用法错必须**零副作用**
// （别名不许登记、会话不许建）。判据 = 临时 `ZERG_STATE_DIR` 里 `chat-*` 件集合前后**不变**
// （`tool_events/` 是观测层每次调用都建的框架级目录 ⇒ 不计 · 与 py 同口径）。
var rp0CleanCase = rp0Case{
	ID:        "p0-12",
	Desc:      "用法错零副作用（不许建会话/登记别名）",
	Argv:      []string{"chat", "ask", "--as", "p0probe", "--json", "nope", "hi"},
	WantRC:    2,
	WantBytes: rp0WantNoBytes,
}

// rp0DropEnv —— 隔离时必须从继承环境里**剔掉**的旋钮（py 逐字剔这三枚：让制品不许把
// 调用方的仓根/门锁/索引偷偷带进来），外加被重写的三枚（同键若留旧值，子进程 getenv
// 拿到的会是**先出现**的那一份 ⇒ 隔离当场失效）。
var rp0DropEnv = []string{
	"ZERG_REPO", "ZERG_GATE_LOCK_DIR", "GIT_INDEX_FILE",
	"HOME", "ZERG_STATE_DIR", "TMPDIR",
}

// rp0EnvFor —— 造一例的隔离环境（py `env_for(tmp)`）：HOME / ZERG_STATE_DIR / TMPDIR 三个
// 子目录建好并指向 `dir`，被剔的旋钮一个不带。
//
// ★ 返回**完整**的 env 列表（os.Environ 为底）⇒ 调用方直接喂 exec.Cmd.Env；同键只留一份。
func rp0EnvFor(dir string) []string {
	home := filepath.Join(dir, "home")
	state := filepath.Join(dir, "state")
	tmp := filepath.Join(dir, "tmp")
	for _, d := range []string{home, state, tmp} {
		_ = os.MkdirAll(d, 0o755)
	}
	drop := make(map[string]bool, len(rp0DropEnv))
	for _, k := range rp0DropEnv {
		drop[k] = true
	}
	out := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if drop[k] {
			continue
		}
		out = append(out, kv)
	}
	return append(out,
		"HOME="+home,
		"ZERG_STATE_DIR="+state,
		"TMPDIR="+tmp,
	)
}

// rp0RunOne —— 跑一例：返回 (rc, stdout 字节数)。stderr 丢弃（py `capture_output=True` 后不看它）。
//
// 起不来 / 超时 ⇒ rc = -1（必判失败 · 与 py 的 TimeoutExpired/异常同向：都是「这一例没判成」）。
// 被信号杀掉 ⇒ 取 ExitCode()（负值 · 同样必判失败）。
func rp0RunOne(bin string, argv []string, env []string) (int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), rp0CaseTimeout)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, argv...)
	cmd.Env = env
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), buf.Len()
		}
		return -1, buf.Len()
	}
	return 0, buf.Len()
}

// rp0Verdict —— 一例的判定（**纯函数** · 负控可喂合成读数）。
//
// mutateFirst=true 且 id=="p0-01" ⇒ **期望故意改错**：改判「rc 必须 0」。正常档下 p0-01 是
// 用法错（rc=2）⇒ 这一格必红 ⇒ 证明判据体真在比 rc（而不是恒绿）。
func rp0Verdict(c rp0Case, rc, nbytes int, mutateFirst bool) bool {
	ok := rc == c.WantRC && (c.WantBytes == rp0WantNoBytes || nbytes == c.WantBytes)
	if mutateFirst && c.ID == "p0-01" {
		ok = rc == 0
	}
	return ok
}

// rp0PASSFAIL —— 一行前缀（py 的 `"PASS" if ok else "FAIL"`）。
func rp0PASSFAIL(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// rp0RunCases —— 跑 11 条（py `run_cases`）。
//
// mutateFirst=true ⇒ **不打印**（py 同档：元测试只数失败条数）· 且 p0-01 期望改错。
func rp0RunCases(out io.Writer, bin string, mutateFirst bool) int {
	fails := 0
	for _, c := range rp0Cases {
		tmp, err := os.MkdirTemp("", "chateval-")
		if err != nil {
			fails++
			continue
		}
		rc, nbytes := rp0RunOne(bin, c.Argv, rp0EnvFor(tmp))
		_ = os.RemoveAll(tmp)
		ok := rp0Verdict(c, rc, nbytes, mutateFirst)
		if !ok {
			fails++
		}
		if mutateFirst {
			continue
		}
		want := "-"
		if c.WantBytes != rp0WantNoBytes {
			want = strconv.Itoa(c.WantBytes)
		}
		fmt.Fprintf(out, "%s %-4s %-28s rc=%-2d(want %-2d) stdout=%-5d(want %-5s)\n",
			rp0PASSFAIL(ok), c.ID, c.Desc, rc, c.WantRC, nbytes, want)
	}
	return fails
}

// rp0ChatFiles —— 现读目录里 `chat-` 前缀的件（py `chat_files`：只比 chat 专属件）。
func rp0ChatFiles(dir string) map[string]bool {
	out := map[string]bool{}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "chat-") {
			out[e.Name()] = true
		}
	}
	return out
}

// rp0PyList —— 照 Python 的 list repr 打一串（`[]` / `['a', 'b']`）—— 逐字对齐 py 那行输出。
func rp0PyList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	q := make([]string, 0, len(items))
	for _, s := range items {
		q = append(q, "'"+strings.ReplaceAll(s, "'", "\\'")+"'")
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// rp0RunClean —— 跑副作用面那一条（py `run_clean_cases`）：返回失败条数（0/1）。
func rp0RunClean(out io.Writer, bin string) int {
	tmp, err := os.MkdirTemp("", "chateval-x-")
	if err != nil {
		return 1
	}
	env := rp0EnvFor(tmp)
	state := filepath.Join(tmp, "state")
	before := rp0ChatFiles(state)
	rc, _ := rp0RunOne(bin, rp0CleanCase.Argv, env)
	after := rp0ChatFiles(state)
	_ = os.RemoveAll(tmp)

	var added []string
	for n := range after {
		if !before[n] {
			added = append(added, n)
		}
	}
	sort.Strings(added)
	clean := rc == rp0CleanCase.WantRC && len(before) == len(after) && len(added) == 0
	fmt.Fprintf(out, "%s %-4s %s（rc=%d · 新增落盘=%s）\n",
		rp0PASSFAIL(clean), rp0CleanCase.ID, rp0CleanCase.Desc, rc, rp0PyList(added))
	if clean {
		return 0
	}
	return 1
}

// rp0MutatedFails —— 元测试：把 p0-01 期望改错后跑一遍，返回「红了的条数」（py 同档：必须 ≥1）。
func rp0MutatedFails(bin string) int {
	return rp0RunCases(io.Discard, bin, true)
}

// rp0Main —— 判定体（root 已解析 · selfTestOnly = py 的 `--self-test`）。
//
// 退码：0 全绿 ／ 1 判定红（含「元测试没红 ⇒ 无牙」）／ 2 不给结论（制品不在盘）。
func rp0Main(stdout, stderr io.Writer, root string, selfTestOnly bool) int {
	bin := filepath.Join(root, filepath.FromSlash(rp0BinRel))
	if !rp0Executable(bin) {
		fmt.Fprintf(stderr, "%s: 被判制品不在盘或不可执行（%s）⇒ 不给结论（rc=2 · ✗ 不把环境错记成契约错）\n",
			rp0Prog, bin)
		return 2
	}
	if selfTestOnly {
		fails := rp0MutatedFails(bin)
		if fails < 1 {
			fmt.Fprintln(stdout, "✗ 元测试没红 ⇒ 套件是恒绿的（无牙）")
			return 1
		}
		fmt.Fprintf(stdout, "元测试：负控已红（%d 条）⇒ 套件有牙 ✓\n", fails)
		return 0
	}
	fails := rp0RunCases(stdout, bin, false)
	cfails := rp0RunClean(stdout, bin)
	fmt.Fprintf(stdout, "\nP0 合计 %d 条 · 失败 %d 条（副作用面另 1 条 · 失败 %d 条）\n",
		len(rp0Cases), fails, cfails)
	// ★ 内嵌元测试（py 同档：主用例跑完自证一次）—— 没红 ⇒ 套件无牙 ⇒ 判失败。
	if rp0MutatedFails(bin) < 1 {
		fmt.Fprintln(stdout, "✗ 内嵌元测试没红 ⇒ 套件无牙 ⇒ 判失败")
		return 1
	}
	if fails == 0 && cfails == 0 {
		return 0
	}
	return 1
}

// rp0Executable —— 制品可用判定：存在 ∧ 是普通件 ∧ 任一执行位（与 `bin/zerg` 的用法同规）。
func rp0Executable(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode().Perm()&0o111 != 0
}

// rp0ResolveRoot —— 仓根：`--root` 优先；否则 ZERG_REPO_ROOT（转发壳 inproc 前必设）
// → ZERG_REPO → `repoRoot()`（可执行件/当前目录上溯 · 锚同全仓）。取不到 ⇒ ""（调用方退 2）。
func rp0ResolveRoot(rootArg string, rootSet bool) string {
	if rootSet {
		return strings.TrimSpace(rootArg)
	}
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO_ROOT")); v != "" {
		return v
	}
	return strings.TrimSpace(repoRoot())
}

// runP0GateMain —— 入口（argv 不含程序名 · 与全仓判据件同形）。
func runP0GateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg := ""
	rootSet := false
	doCheck, selfTest := false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, rp0UsageText)
			return 0
		case a == "--self-test":
			selfTest = true
		case a == "--check":
			doCheck = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", rp0Prog)
				fmt.Fprint(stderr, rp0UsageText)
				return 2
			}
			i++
			rootArg, rootSet = argv[i], true
		case strings.HasPrefix(a, "--root="):
			rootArg, rootSet = strings.TrimPrefix(a, "--root="), true
			if strings.TrimSpace(rootArg) == "" {
				fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", rp0Prog)
				return 2
			}
		default:
			fmt.Fprintf(stderr, "%s: 未知旗标 %s ⇒ 不给结论（rc=2）\n", rp0Prog, a)
			fmt.Fprint(stderr, rp0UsageText)
			return 2
		}
	}
	if doCheck && selfTest {
		fmt.Fprintf(stderr, "%s: `--check` / `--self-test` 互斥 ⇒ 不给结论（rc=2）\n", rp0Prog)
		return 2
	}
	root := rp0ResolveRoot(rootArg, rootSet)
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且上溯取不到）⇒ 不给结论（rc=2）\n", rp0Prog)
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return rp0Main(stdout, stderr, root, selfTest)
}

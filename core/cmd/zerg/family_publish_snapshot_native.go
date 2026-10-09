// family_publish_snapshot_native.go —— `zerg publish snapshot-self-test`（拟名 · **待命令树登记**）的 **Go 真身**
// （发布链 Go 半优先 · 本批第四件 · 上三件 = 预检 `family_publish_preflight_native.go` ·
// 压平器 `family_publish_public_native.go` · 两器对拍 `family_publish_verify_native.go`）。
//
// 源件：`scripts/build/publish-snapshot-self-test.py`（205 行 · 把公开 CI 的同款步骤在本地公开快照上先跑一遍 ·
// **无 CLI 入口**）。本笔把它的**编排**搬进 CLI 进程内，逐段与旧 py 逐字对齐：
//
//	① 解析 argv（位置参数 = 公开快照目录；`--job` 可重复；`--json`）
//	② 判据树形态机检（快照里有没有 `.git`）
//	③ 真源 = `publish/ci/ci.yml` + `publish/ci/release-agent.yml`（本件**不另写一套判据** —— 一处真源）
//	④ 逐作业 × 逐步骤：`${{ }}` ⇒ SKIP；无仓 + 需要仓的 git 子命令 ⇒ SKIP；工作目录缺 ⇒ FAIL(2)；
//	   其余 `bash -e -c <body>` 真跑（cwd = 快照 / 工作目录），rc 与合并输出逐字取回
//	⑤ 人面 / `--json` 两态渲染（含逐步骤耗时降序）+ 出口三档（0/1/2）
//
// ★ 子进程边界逐字保留（与旧 py 同一条）：每步 = py 的 subprocess.run（参数 bash / -e / -c / body，
// cwd=… env=…，stdout 收 PIPE、stderr 并 STDOUT）⟷ Go 侧 exec.Command("bash","-e","-c",body)
// （stdout 与 stderr 合并进**同一**缓冲）⇒ 命令面与合并口径**同源**、✗ 不在 Go 侧另造第二口径。
//
// ★ 两态对拍口径：`publishSnapshotSelfTestNative(<root>, …)`（本真身）⟷ py 原件
// （放在仓外夹具的 `scripts/build/publish-snapshot-self-test.py` · 由 `__file__` 上溯三级取仓根）
// 在**同一棵仓外夹具**上 **rc/stdout/stderr 三面逐字同**（掩时间 / 临时路径 / 随机 id）。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· 默认根定位差异：py 从 `__file__` 上溯三级取仓根；Go 无 `__file__` 语境，本入口**显式收 root 入参**
//	  ⇒「脚本位置 ≠ 入参 root」时两态指向不同树（对拍一律把 py 放在夹具的 `scripts/build/` 下、
//	  Go 侧传同一夹具为 root ⇒ 两侧同锚；仓内调用方按同族口径传 `repoRoot()`）。
//	· argparse 的 `-h/--help` 与未知旗标 usage 正文：文本不同源（本件只保证退码 0 / 2）；
//	· `--job` / `--json` 的 argparse 边界（如 `--job` 缺值、多余位置参数）文案不同源；
//	· 子进程被信号杀时 py 的 returncode 为**负**、本件按 Go 惯例返 -1（对拍面里两步都非信号死）；
//	· 非 UTF-8 输出 decode：py `errors="replace"` 逐**子串**替换 ⟷ Go `strings.ToValidUTF8` 逐**连串**替换，
//	  相邻非法序列的替换个数可能不同（对拍夹具均为 ASCII）。
//	· 逐步骤耗时（`secs` / `wall_secs` / 降序表）属**纯观测**面（旧 py 逐步骤计时仅为观测），对拍时归一。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// pssWorkflows —— 判据真源（逐字照 py `WORKFLOWS`）。本件**不另写一套判据**。
var pssWorkflows = []string{"publish/ci/ci.yml", "publish/ci/release-agent.yml"}

// pssReGitRepoSubcmd —— 复刻 py 的 `GIT_REPO_SUBCMD`（「这一步要不要一个 git 仓」的机检真源 ①）。
// `git config --global` **不在**此列 —— 它仓外也成立（见 py 件头 ★）。
var pssReGitRepoSubcmd = regexp.MustCompile(
	`(?:^|[\s;&|(])git\s+(?:-C\s+\S+\s+)?` +
		`(rev-parse|rev-list|status|log|describe|diff|show|ls-files|cat-file|for-each-ref|` +
		`fetch|push|commit|tag|branch|checkout|worktree|remote|stash|merge-base)\b`)

// pssManifestFreshness —— 复刻 py `MANIFEST_FRESHNESS`（真源 ② · 该族脚本内部 `_git_strict` 调 `git rev-parse`）。
const pssManifestFreshness = "check-manifest-freshness"

// pssRCNotFound —— shell 惯例退码 127（子进程起不来）；127 属 internal 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
const pssRCNotFound = 127

// pssWorkflow 行式解析（✗ 不引第三方库）的固定正则（逐字照 py `parse_workflow`）。
var (
	pssReJobsOnly = regexp.MustCompile(`^jobs:\s*$`)
	pssReJobName  = regexp.MustCompile(`^  ([A-Za-z0-9_\-]+):\s*$`)
	pssReStepName = regexp.MustCompile(`^      - name:\s*(.+?)\s*$`)
	pssReWorkDir  = regexp.MustCompile(`^\s+working-directory:\s*(.+?)\s*$`)
	pssReRun      = regexp.MustCompile(`^(\s+)run:\s*\|?\s*(.*)$`)
)

// pssStep / pssJob —— 解析产物（✗ 用切片保序，不用 map —— 复刻 py dict 的插入序语义）。
type pssStep struct {
	name string
	cwd  string
	run  []string
}

type pssJob struct {
	name  string
	steps []*pssStep
}

// pssNeedsGitRepo —— 复刻 py `needs_git_repo`：纯文本机检，不跑任何东西。
func pssNeedsGitRepo(body string) bool {
	if strings.Contains(body, pssManifestFreshness) {
		return true
	}
	for _, ln := range strings.Split(body, "\n") {
		if pssReGitRepoSubcmd.MatchString(ln) {
			return true
		}
	}
	return false
}

// pssParseWorkflow —— 复刻 py `parse_workflow`（行式解析：作业 → 步骤 name / working-directory / run 块）。
// 保序：作业按**首次出现序**、步骤按出现序（py dict setdefault 同义）。
func pssParseWorkflow(path string) []pssJob {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	jobs := []pssJob{}
	idxOf := map[string]int{}
	curJobIdx, curStepIdx := -1, -1
	inJobs := false
	i := 0
	for i < len(lines) {
		rawLine := lines[i]
		if pssReJobsOnly.MatchString(rawLine) {
			inJobs = true
			i++
			continue
		}
		if inJobs {
			if m := pssReJobName.FindStringSubmatch(rawLine); m != nil {
				name := m[1]
				if k, ok := idxOf[name]; ok {
					curJobIdx = k
				} else {
					jobs = append(jobs, pssJob{name: name})
					curJobIdx = len(jobs) - 1
					idxOf[name] = curJobIdx
				}
				curStepIdx = -1
				i++
				continue
			}
			if m := pssReStepName.FindStringSubmatch(rawLine); m != nil && curJobIdx >= 0 {
				nm := strings.Trim(m[1], "'\"")
				jobs[curJobIdx].steps = append(jobs[curJobIdx].steps, &pssStep{name: nm, run: []string{}})
				curStepIdx = len(jobs[curJobIdx].steps) - 1
				i++
				continue
			}
			if m := pssReWorkDir.FindStringSubmatch(rawLine); m != nil && curStepIdx >= 0 {
				jobs[curJobIdx].steps[curStepIdx].cwd = strings.Trim(m[1], "'\"")
				i++
				continue
			}
			if m := pssReRun.FindStringSubmatch(rawLine); m != nil && curStepIdx >= 0 {
				indent := len(m[1])
				first := m[2]
				if strings.TrimSpace(first) != "" && strings.TrimSpace(first) != "|" {
					jobs[curJobIdx].steps[curStepIdx].run =
						append(jobs[curJobIdx].steps[curStepIdx].run, first)
					i++
					continue
				}
				i++
				for i < len(lines) {
					nxt := lines[i]
					if strings.TrimSpace(nxt) == "" {
						jobs[curJobIdx].steps[curStepIdx].run =
							append(jobs[curJobIdx].steps[curStepIdx].run, "")
						i++
						continue
					}
					if pssLeadWS(nxt) <= indent {
						break
					}
					jobs[curJobIdx].steps[curStepIdx].run =
						append(jobs[curJobIdx].steps[curStepIdx].run, strings.TrimSpace(nxt))
					i++
				}
				continue
			}
		}
		i++
	}
	// 复刻 py 收尾 `{k: v for k, v in jobs.items() if v}`：丢掉无步骤的作业。
	out := []pssJob{}
	for _, jb := range jobs {
		if len(jb.steps) > 0 {
			out = append(out, jb)
		}
	}
	return out
}

// pssLeadWS —— 前导空白字节数（复刻 py `len(nxt) - len(nxt.lstrip())`；行进块只有空格制表）。
func pssLeadWS(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}

// pssRec —— 一条步骤记录（pass / fail / skip 三态共形；out 仅 fail 有、why 仅 skip 有）。
type pssRec struct {
	step string
	rc   int
	secs float64
	out  string
	why  string
}

// pssIsDir —— 存在且是目录（照 py `os.path.isdir`）。
func pssIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// pssIsFile —— 存在且不是目录（照 py `os.path.isfile`）。
func pssIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// pssAbs —— 复刻 py `os.path.abspath`（相对 → 拼 cwd；规范化；✗ 不解符号链接）。
func pssAbs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return filepath.Clean(p)
}

// pssSetEnv —— 在 env 切片里覆盖/追加一个键（照 py `env["PATH"] = …`）。
func pssSetEnv(env []string, k, v string) []string {
	prefix := k + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + v
			return env
		}
	}
	return append(env, prefix+v)
}

// pssDecodeRepl —— 复刻 py `bytes.decode("utf-8","replace")` 的常见面（合法 UTF-8 逐字节同；
// 非法连串替换为 U+FFFD —— 与 py 逐子串替换在相邻多错序列上可能不同，已列入未覆盖面）。
func pssDecodeRepl(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// pssExitCode —— 子进程退码（信号死按 Go 惯例 -1；py 的 returncode 为负，已列入未覆盖面）。
func pssExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return pssRCNotFound
}

// pssClip —— 复刻 py `ln[:200]`（**按码点**截断，✗ 不按字节）。
func pssClip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// pssTailJoined —— 复刻 py `"\n".join(out.strip().split("\n")[-25:])`。
func pssTailJoined(out string) string {
	trimmed := strings.TrimSpace(out)
	parts := strings.Split(trimmed, "\n")
	if len(parts) > 25 {
		parts = parts[len(parts)-25:]
	}
	return strings.Join(parts, "\n")
}

// pssPyFloat —— 复刻 py `json.dumps` 里 float 的文本（`float.__repr__` 的最短往返形态）。
// Go 'g'/最短与 py repr 在常见区间逐字同；整值补 `.0`（py `0.0` ≠ Go `0`）。
func pssPyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// pssJSONString —— 复刻 py `json.dumps(ensure_ascii=False)` 的字符串转义（控制符走短转义 / \u00xx，非 ASCII 原样）。
func pssJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// pssJSONList —— 复刻 py `json.dumps(..., indent=2)` 的记录数组（三态键序逐字：pass=[step,rc,secs] ·
// fail=[step,rc,secs,out] · skip=[step,secs,why]）。
func pssJSONList(b *strings.Builder, recs []pssRec, mode int) {
	if len(recs) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, r := range recs {
		b.WriteString("    {\n")
		b.WriteString("      \"step\": " + pssJSONString(r.step))
		if mode == 2 {
			b.WriteString(",\n      \"secs\": " + pssPyFloat(r.secs))
			b.WriteString(",\n      \"why\": " + pssJSONString(r.why) + "\n")
		} else {
			b.WriteString(",\n      \"rc\": " + strconv.Itoa(r.rc))
			b.WriteString(",\n      \"secs\": " + pssPyFloat(r.secs))
			if mode == 1 {
				b.WriteString(",\n      \"out\": " + pssJSONString(r.out))
			}
			b.WriteString("\n")
		}
		b.WriteString("    }")
		if i != len(recs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ]")
}

// pssSkipWhyNoGit —— 复刻 py 无仓步骤的 SKIP 理由（逐字拼接，✗ 不写成一串易读长句）。
const pssSkipWhyNoGit = "判据树无 `.git`（快照树）· 本步机检含需要仓的 git 子命令" +
	"或 `check-manifest-freshness*` ⇒ 真 CI 里由前面的 `actions/checkout` " +
	"铺出仓、本地再现缺那层 ⇒ 如实列 SKIP（不当跑过 · 也不当红）"

// pssSkipWhyCtx —— 复刻 py 含 GitHub 上下文表达式的 SKIP 理由。
const pssSkipWhyCtx = "含 GitHub 上下文表达式（${{ }}）⇒ 本地不猜"

// publishSnapshotSelfTestNative —— `zerg publish snapshot-self-test <快照目录> [--job 作业] [--json]` 的
// 进程内真身（旧 py 逐段转写 · 退码与 stdout/stderr 三面照旧）。root = 判据真源所在仓根
// （仓内调用方传 `repoRoot()`；对拍夹具传夹具根）。
func publishSnapshotSelfTestNative(root string, stdout, stderr io.Writer, argv []string) int {
	snapArg := ""
	jobsFilter := []string{}
	jsonOut := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--job":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "publish-snapshot-self-test.py: error: argument --job: expected one argument")
				return exitUsage
			}
			jobsFilter = append(jobsFilter, argv[i+1])
			i++
		case strings.HasPrefix(a, "--job="):
			jobsFilter = append(jobsFilter, a[len("--job="):])
		case a == "-h" || a == "--help":
			// argparse `-h` 正文属未覆盖面（✗ 不声称逐字对齐）；只保证退码 0。
			fmt.Fprintln(stdout, "usage: publish-snapshot-self-test.py [-h] [--job JOB] [--json] snapshot")
			return exitOK
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "publish-snapshot-self-test.py: error: unrecognized arguments: %s\n", a)
			return exitUsage
		default:
			if snapArg != "" {
				fmt.Fprintf(stderr, "publish-snapshot-self-test.py: error: unrecognized arguments: %s\n", a)
				return exitUsage
			}
			snapArg = a
		}
	}
	if snapArg == "" {
		fmt.Fprintln(stderr, "publish-snapshot-self-test.py: error: the following arguments are required: snapshot")
		return exitUsage
	}

	snap := pssAbs(snapArg)
	if !pssIsDir(snap) {
		fmt.Fprintf(stderr, "判不了：快照目录不存在 %s ⇒ 退码 2\n", snap)
		return exitUsage
	}
	repo := root
	if repo == "" {
		repo = repoRoot()
	}
	missing := []string{}
	for _, w := range pssWorkflows {
		if !pssIsFile(filepath.Join(repo, filepath.FromSlash(w))) {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "判不了：工作流真源缺件 %s ⇒ 退码 2\n", strings.Join(missing, "、"))
		return exitUsage
	}

	env := append([]string{}, os.Environ()...)
	env = pssSetEnv(env, "PATH", os.Getenv("PATH")+":/opt/homebrew/bin:/usr/local/go/bin")

	snapHasGit := pssIsDir(filepath.Join(snap, ".git"))

	results := []pssRec{}
	fails := []pssRec{}
	skips := []pssRec{}
	tAll := time.Now()
	for _, wf := range pssWorkflows {
		jobs := pssParseWorkflow(filepath.Join(repo, filepath.FromSlash(wf)))
		for _, jb := range jobs {
			if len(jobsFilter) > 0 {
				hit := false
				for _, jf := range jobsFilter {
					if jf == jb.name {
						hit = true
						break
					}
				}
				if !hit {
					continue
				}
			}
			for idx, st := range jb.steps {
				body := strings.TrimSpace(strings.Join(st.run, "\n"))
				tag := fmt.Sprintf("%s/%s#%d %s", filepath.Base(wf), jb.name, idx+1, st.name)
				if body == "" {
					continue
				}
				if strings.Contains(body, "${") {
					skips = append(skips, pssRec{step: tag, secs: 0, why: pssSkipWhyCtx})
					continue
				}
				if !snapHasGit && pssNeedsGitRepo(body) {
					skips = append(skips, pssRec{step: tag, secs: 0, why: pssSkipWhyNoGit})
					continue
				}
				workdir := snap
				if st.cwd != "" {
					workdir = filepath.Join(snap, filepath.FromSlash(st.cwd))
				}
				if !pssIsDir(workdir) {
					fails = append(fails, pssRec{step: tag, rc: 2, secs: 0,
						out: fmt.Sprintf("工作目录不在场：%s", workdir)})
					continue
				}
				t0 := time.Now()
				cmd := armChildGroup(exec.Command("bash", "-e", "-c", body))
				runningChild = cmd
				cmd.Dir = workdir
				cmd.Env = env
				var buf bytes.Buffer
				cmd.Stdout = &buf
				cmd.Stderr = &buf
				err := cmd.Run()
				runningChild = nil
				dt := time.Since(t0).Seconds()
				rc := pssExitCode(err)
				out := pssDecodeRepl(buf.Bytes())
				rec := pssRec{step: tag, rc: rc, secs: dt}
				if rc == 0 {
					results = append(results, rec)
				} else {
					rec.out = pssTailJoined(out)
					fails = append(fails, rec)
				}
			}
		}
	}
	wall := time.Since(tAll).Seconds()

	if jsonOut {
		var b strings.Builder
		b.WriteString("{\n")
		b.WriteString("  \"pass\": ")
		pssJSONList(&b, results, 0)
		b.WriteString(",\n  \"fail\": ")
		pssJSONList(&b, fails, 1)
		b.WriteString(",\n  \"skip\": ")
		pssJSONList(&b, skips, 2)
		b.WriteString(",\n  \"snapshot_has_git\": ")
		if snapHasGit {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(",\n  \"wall_secs\": ")
		b.WriteString(pssPyFloat(wall))
		b.WriteString("\n}")
		fmt.Fprintln(stdout, b.String())
	} else {
		fmt.Fprintf(stdout, "── 公开快照自测（判据真源 = publish/ci/*.yml）· 快照：%s ──\n", snap)
		for _, r := range results {
			fmt.Fprintf(stdout, "  ✓ %s\n", r.step)
		}
		for _, s := range skips {
			fmt.Fprintf(stdout, "  – SKIP %s（%s）\n", s.step, s.why)
		}
		for _, f := range fails {
			fmt.Fprintf(stdout, "  ✗ %s（rc=%d）\n", f.step, f.rc)
			for _, ln := range strings.Split(f.out, "\n") {
				if strings.TrimSpace(ln) != "" {
					fmt.Fprintf(stdout, "      | %s\n", pssClip(ln, 200))
				}
			}
		}
		type row struct {
			mark string
			rec  pssRec
		}
		rows := []row{}
		for _, r := range results {
			rows = append(rows, row{"✓ ", r})
		}
		for _, f := range fails {
			rows = append(rows, row{"✗ ", f})
		}
		for _, s := range skips {
			rows = append(rows, row{"SKIP", s})
		}
		total := 0.0
		for _, rr := range rows {
			total += rr.rec.secs
		}
		fmt.Fprintf(stdout, "── 逐步骤耗时（降序 · 计入 %d 条 · 合计 %.1fs · 全程 %.1fs）──\n",
			len(rows), total, wall)
		// py `sorted(..., reverse=True)` 是**稳定**排序 ⇒ 用 SliceStable（相等键保插入序）。
		sort.SliceStable(rows, func(a, b int) bool { return rows[a].rec.secs > rows[b].rec.secs })
		for _, rr := range rows {
			fmt.Fprintf(stdout, "  %7.1fs  %-4s %s\n", rr.rec.secs, rr.mark, rr.rec.step)
		}
		fmt.Fprintf(stdout, "⇒ 通过 %d · 失败 %d · SKIP %d\n", len(results), len(fails), len(skips))
	}
	if len(fails) > 0 {
		fmt.Fprintln(stderr, "⇒ 有步骤失败 ⇒ 退码 1（**不许推**：推到远端只会红同样的几步）")
		return exitFail
	}
	fmt.Fprintln(stdout, "⇒ 全过（公开 CI 同款步骤本地已绿）⇒ 退码 0")
	return exitOK
}

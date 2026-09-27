// family_gate_run_step.go —— `zerg gate run --step <步名>` 的**机器面**（缺口-命令面 §三 `C3` ·
// §十一「P0 之外但今天也撞到的」头条 · 2026-09-21）。
//
// 为什么要有它：D3 当时的实测是「定位一件事只能整档跑」—— `--scope devdocs` 4 步、默认档 60 步，
// 改一道门要等整档；单门自检只能手搓 `python3 scripts/gates/check-error-kinds.py --self-test`（17 行）。
//
// 本件只补三格，执行面一个字都没搬进来：
//
//	① **步名逐字校验**（执行前判）：真源 = 脚本自己的 `add_step` 行（`parseAddSteps`，与 `gate explain`
//	   同一份解析）。未知名 ⇒ 退 2 并给「最像的合法输入」（K14 第三件 · 与 `agent` 族**同一方言**）；
//	   重名（歧义）⇒ 退 2。它只在**跑之前**判，不改任何脚本退码。
//	② `--json <字段>`：给这一步的判决出一份**契约形状**的包封。
//	③ `--step <步名> --verify-live`：单步**只读复核**的正门 —— 把那只旗标**逐字透传给那一步的
//	   命令串**（不是自举件；后者不认识它 ⇒ 现读 rc=2），退码原样转出（见件末 `gateRunStepLiveVerify`）。
//
// 为什么不把执行也搬进来（`gate.go` 的「只转发、不翻译」）：步骤表住在脚本里，**唯一真源**在那边——
// 命令面自己再跑一遍就等于另写一份步骤表（`G1-a` 明令禁止）。所以执行永远是
// `bash scripts/gates/precommit-gates.sh --step <步名> [--self-test] [--outdir <目录>]`，
// 命令面只是**转出它的退码**（连异常码也不改写）与**把它的报告照原样给人看**。
//
// 为什么 `--json` 必须由命令面出（而不是退给脚本自己拼）：包封形状（六键 · `items` 恒数组 · `kind`
// 单数 CamelCase）是 §九 M6 的**命令面契约**，让每个被包的脚本各写一份 JSON 就是把契约抄成 N 份。
// 判决本身仍是脚本的（命令面逐格读 `results.tsv`，不自己判 PASS/FAIL）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gateRunStepFields —— `gate run --step … --json` 的全部字段（K1：机器面先定）。
var gateRunStepFields = []string{"step", "scope", "mode", "verdict", "rc", "secs", "log", "outdir"}

// gateRunStepJSON —— 单步档的机器面（人面仍由脚本原样给出，走 stderr）。
//
// 退码口径：**脚本的码原样转出**（0 全绿 / 1 有失败项 / 2 不给结论）；命令面自己只在
// 「执行前判」与「K2 的 `--json` 不给字段」两处退码：前者 2、后者**取自退码表**（`usage` · 归一后同为 2）。
func gateRunStepJSON(inv *invocation, stdout, stderr io.Writer, root, script string, tail []string) int {
	// K2（§4.1）：给了 `--json` 但不给字段 ⇒ 退码由 `requireFields` **取自退码表**回（`K2` 归一后 = 用法错 2），
	// stdout 0 字节（与 `requireFields` 同一口径）。
	if len(inv.fields) == 0 {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	name, outdir, hasOutdir := "", "", false
	// `--json <字段>` 是**命令面**的旗标：透传给被包的脚本会被它当未知参数拒掉（rc=2）——
	// 这里逐枚剔掉它（含它的值），脚本收到的是它本来就认识的旗标。
	scriptTail := make([]string, 0, len(tail))
	for i := 0; i < len(tail); i++ {
		a := tail[i]
		if a == "--json" {
			i++ // 连同它的值一起丢掉
			continue
		}
		if strings.HasPrefix(a, "--json=") {
			continue
		}
		scriptTail = append(scriptTail, a)
	}
	for i := 0; i < len(scriptTail); i++ {
		switch scriptTail[i] {
		case "--step":
			if i+1 < len(scriptTail) {
				name = scriptTail[i+1]
			}
		case "--outdir":
			if i+1 < len(scriptTail) {
				outdir, hasOutdir = scriptTail[i+1], true
			}
		}
	}
	if strings.TrimSpace(name) == "" {
		inv.setErr("usage", "missing_step_name", "`--json` 只在单步档上有面")
		fmt.Fprintf(stderr, "%s: `gate run --json` 只在**单步档**（`--step <步名>`）上有面 —— 全档的判决面是脚本自己的报告\n", progName)
		fmt.Fprintf(stderr, "例：%s gate run --step '门⑤ 命令面 error.kind 闭集 + 命名纪律（阻断）' --json %s\n",
			progName, strings.Join(gateRunStepFields[:3], ","))
		return exitUsage
	}

	// ① 步名逐字校验（**执行前判**）：真源 = 脚本自己的 add_step 行，不另抄一份表。
	src, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	decls := parseAddSteps(string(src))
	hits := []gateStepDecl{}
	for _, d := range decls {
		if d.Name == name {
			hits = append(hits, d)
		}
	}
	if len(hits) == 0 {
		inv.setErr("usage", "step_absent", "步名不存在")
		fmt.Fprintf(stderr, "%s: 步骤名 %q **逐字**找不到（现读脚本里的 add_step 行 %d 条）⇒ 退码 2\n", progName, name, len(decls))
		if s := nearestSteps(decls, name, 1); len(s) > 0 {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s[0])
		}
		for _, s := range nearestSteps(decls, name, 3) {
			fmt.Fprintf(stderr, "  候选：%s\n", s)
		}
		return exitUsage
	}
	if len(hits) > 1 {
		inv.setErr("usage", "step_ambiguous", "步名歧义")
		fmt.Fprintf(stderr, "%s: 步骤名 %q 在脚本里出现 %d 次 ⇒ **歧义**（判据要一个答案 · 退码 2）：\n", progName, name, len(hits))
		for _, d := range hits {
			fmt.Fprintf(stderr, "  %s:%d  scope=%s mode=%s\n", gateScriptRel, d.Line, d.Scope, d.Mode)
		}
		return exitUsage
	}
	d := hits[0]

	// ② 日志目录：`--outdir` 给了就用它，没给就命令面指定一个 —— 因为要按它**读回** results.tsv。
	//   读不回来时**不许编数**：字段值一律写「（读不到）」并在 stderr 说明。
	if !hasOutdir {
		outdir = filepath.Join(os.TempDir(), "zerg-gate-step-"+time.Now().Format("20060102-150405"))
		if err := os.MkdirAll(outdir, 0o755); err != nil {
			inv.setErr("failed", "outdir_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 建不了日志目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, outdir, err)
			return exitBlocked
		}
		tail = append(tail, "--outdir", outdir)
		scriptTail = append(scriptTail, "--outdir", outdir)
	}

	// ③ 真跑那一步（执行面 = 脚本自己；命令面只转出退码）。
	cmd := exec.Command("bash", append([]string{script}, scriptTail...)...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	var human bytes.Buffer
	cmd.Stdout = &human // 人面（脚本的报告）先收着，稍后原样转到 stderr —— stdout 只留机器面
	cmd.Stderr = stderr
	runningChild = cmd
	err = cmd.Run()
	runningChild = nil
	rc := exitOK
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode() // ★ 只转发、不翻译：脚本退多少就返多少
	} else if err != nil {
		fmt.Fprintf(stderr, "%s: 跑不动门禁脚本：%v\n", progName, err)
		return exitFail
	}
	_, _ = io.Copy(stderr, &human)

	// ④ 判决从脚本自己的结果表读回（不自己判）。
	row := map[string]string{
		"step": d.Name, "scope": d.Scope, "mode": d.Mode, "outdir": outdir,
		"verdict": "（读不到：脚本没落结果表）", "rc": "（读不到）", "secs": "（读不到）", "log": "（读不到）",
	}
	if body, rerr := os.ReadFile(filepath.Join(outdir, "results.tsv")); rerr == nil {
		for _, ln := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
			f := strings.Split(ln, "\t")
			if len(f) < 5 {
				continue
			}
			row["verdict"], row["rc"], row["secs"], row["log"] = f[0], f[2], f[3], f[4]
			break
		}
	} else {
		fmt.Fprintf(stderr, "%s: 结果表读不到（%s）：%v ⇒ 这一格的判决**不给结论**（不许当绿）\n",
			progName, filepath.Join(outdir, "results.tsv"), rerr)
	}
	if rc := listCmd(inv, stdout, stderr, gateRunStepFields, []map[string]string{row}); rc != exitOK {
		return rc
	}
	return rc
}

// ── `zerg gate run --step <步名> --verify-live`：单步**只读复核**的可手敲正门 ──
//
// 病（现读实测 2026-09-27）：
//
//	· `--verify-live` 是**步骤脚本自己的**旗标（门⑪ 那件 `check-cli-contract.py` 的 §六 `R-6` 逐格回放）；
//	· `gate run` 的透传面把它交给的是**自举件** `scripts/gates/precommit-gates.sh` —— 那件不认识它
//	  ⇒ `✗ 未知参数: --verify-live`（rc=2）；
//	· 直跑 `python3 scripts/gates/check-cli-contract.py --verify-live` 是 CLI 守卫拦下的手搓形态
//	  ⇒ 这一格**只读复核没有可手敲的正门**（人只能绕 `subprocess`）。
//
// 口径（与 `gate.go` 开头那条「只转发、不翻译」同规，零新表）：
//
//	· 步名 ⇒ 命令串 的真源仍是脚本自己的 `add_step` 行（`parseAddSteps` —— 与 `gate explain`/`gate find`
//	  同一份解析），**不另抄一份步骤表**；
//	· ∵ `--verify-live` 挂在**那一步的命令串**上、不挂自举件 ⇒ 命令面把它**逐字透传给那一步的命令串**
//	  （不是透传给 `precommit-gates.sh` —— 那正是现读 rc=2 的病根）；
//	· 退码**原样转出**（这一步的命令退多少 = 命令面退多少）；命令面只在「执行前判」退 2
//	  （步名不存在/歧义、命令串不是纯 argv、缺步名）；
//	· 步名先把**逐字**匹配（与 `--json` 档同规）；逐字不中再试**唯一前缀**（手抄名字易掉「（阻断）」这类
//	  档位后缀，而 `gate ls` 的名字列是带后缀的全名）；多义 ⇒ 退 2 逐个列，不猜。
const gateLiveVerifyFlag = "--verify-live"

// gateRunWantsLiveVerify —— 这一发 tail 里有没有只读复核旗标。
func gateRunWantsLiveVerify(tail []string) bool {
	for _, a := range tail {
		if a == gateLiveVerifyFlag {
			return true
		}
	}
	return false
}

// gateStepNameFrom —— tail 里 `--step` 的值（没有 ⇒ 空串）。只读、不改任何既有函数。
func gateStepNameFrom(tail []string) string {
	for i := 0; i < len(tail); i++ {
		if tail[i] == "--step" && i+1 < len(tail) {
			return tail[i+1]
		}
	}
	return ""
}

// resolveGateStep —— 步名 ⇒ 声明：**逐字**优先，不中再试**唯一前缀**（多义 ⇒ 退 2）。
// 退码：命中 ⇒ exitOK；缺名/不存在 ⇒ exitUsage（含「最像的合法输入」）；读不到声明表由调用方先判。
func resolveGateStep(decls []gateStepDecl, name string, stderr io.Writer) (gateStepDecl, int) {
	if strings.TrimSpace(name) == "" {
		fmt.Fprintf(stderr, "%s: `gate run --step <步名> %s` 要给步名（先 `zerg gate ls` 看步名）\n",
			progName, gateLiveVerifyFlag)
		return gateStepDecl{}, exitUsage
	}
	hits := []gateStepDecl{}
	for _, d := range decls {
		if d.Name == name {
			hits = append(hits, d)
		}
	}
	if len(hits) == 0 {
		// 唯一前缀档：逐字不中才试；命中恰一条 ⇒ 用（并在 stderr 明说用了哪条）。
		pre := []gateStepDecl{}
		for _, d := range decls {
			if strings.HasPrefix(d.Name, name) {
				pre = append(pre, d)
			}
		}
		switch len(pre) {
		case 1:
			fmt.Fprintf(stderr, "%s: 步名 %q 逐字不中，但是**唯一前缀** ⇒ 按 %q 走\n", progName, name, pre[0].Name)
			return pre[0], exitOK
		case 0:
			fmt.Fprintf(stderr, "%s: 步骤名 %q **逐字**与前缀都找不到（现读脚本里的 add_step 行 %d 条）⇒ 退码 2\n",
				progName, name, len(decls))
			if s := nearestSteps(decls, name, 1); len(s) > 0 {
				fmt.Fprintf(stderr, "最像的合法输入: %s\n", s[0])
			}
			for _, s := range nearestSteps(decls, name, 3) {
				fmt.Fprintf(stderr, "  候选：%s\n", s)
			}
			return gateStepDecl{}, exitUsage
		default:
			fmt.Fprintf(stderr, "%s: 前缀 %q 命中 %d 条 ⇒ **有歧义**（判据要一个答案 · 退码 2）：\n",
				progName, name, len(pre))
			for _, d := range pre {
				fmt.Fprintf(stderr, "  %s:%d  %s\n", gateScriptRel, d.Line, d.Name)
			}
			return gateStepDecl{}, exitUsage
		}
	}
	if len(hits) > 1 {
		fmt.Fprintf(stderr, "%s: 步骤名 %q 在脚本里出现 %d 次 ⇒ **歧义**（判据要一个答案 · 退码 2）：\n",
			progName, name, len(hits))
		for _, d := range hits {
			fmt.Fprintf(stderr, "  %s:%d  scope=%s mode=%s\n", gateScriptRel, d.Line, d.Scope, d.Mode)
		}
		return gateStepDecl{}, exitUsage
	}
	return hits[0], exitOK
}

// gateRunStepLiveVerify —— 把只读复核旗标**逐字透传给那一步的命令串**并原样转出它的退码。
// 只在 `--step <步名>` 与旗标同给时由 `gate.go` 的 `run` 支调进（**既有形态一个字节不改**：
// 缺 `--step` 的那一发仍原样走自举件，退码照旧）。
func gateRunStepLiveVerify(inv *invocation, stdout, stderr io.Writer, root, script string, tail []string) int {
	src, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	d, rc := resolveGateStep(parseAddSteps(string(src)), gateStepNameFrom(tail), stderr)
	if rc != exitOK {
		inv.setErr("usage", "step_absent", "步名缺失/不存在/歧义")
		return rc
	}
	cmdStr := strings.TrimSpace(d.Cmd)
	if cmdStr == "" {
		inv.setErr("usage", "step_cmd_absent", "这一步的命令串是空的")
		fmt.Fprintf(stderr, "%s: 这一步（%s）的命令串在脚本里是空的（%s:%d）⇒ 没东西可复核 ⇒ 退码 2\n",
			progName, d.Name, gateScriptRel, d.Line)
		return exitUsage
	}
	// 「不是纯 argv」就不猜：带 shell 语法（管道/重定向/替换/引号）的命令串**逐字**拼不出安全 argv
	// ⇒ 明说不给结论（宁退 2 也不跑一个悄悄变形的命令）。
	if strings.ContainsAny(cmdStr, "|&;<>()$`\\\"'") {
		inv.setErr("usage", "step_cmd_not_argv", "这一步的命令串含 shell 语法，逐字透传不了")
		fmt.Fprintf(stderr, "%s: 这一步（%s）的命令串不是纯 argv（含 shell 语法）：%s（%s:%d）\n",
			progName, d.Name, cmdStr, gateScriptRel, d.Line)
		fmt.Fprintf(stderr, "⇒ `%s` 没法逐字透传 ⇒ 退码 2（不猜、不翻译）\n", gateLiveVerifyFlag)
		return exitUsage
	}
	argv := append(strings.Fields(cmdStr), gateLiveVerifyFlag)
	fmt.Fprintf(stderr, "%s: 只读复核 —— 把 `%s` 逐字透传给这一步（%s）的命令串：%s\n",
		progName, gateLiveVerifyFlag, d.Name, strings.Join(argv, " "))
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runningChild = cmd
	defer func() { runningChild = nil }()
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode() // ★ 只转发、不翻译
	} else if err != nil {
		fmt.Fprintf(stderr, "%s: 跑不动这一步的命令串（%v）\n", progName, err)
		return exitFail
	}
	return exitOK
}

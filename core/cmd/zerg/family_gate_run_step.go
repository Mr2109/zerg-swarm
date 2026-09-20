// family_gate_run_step.go —— `zerg gate run --step <步名>` 的**机器面**（缺口-命令面 §三 `C3` ·
// §十一「P0 之外但今天也撞到的」头条 · 2026-09-21）。
//
// 为什么要有它：D3 当时的实测是「定位一件事只能整档跑」—— `--scope devdocs` 4 步、默认档 60 步，
// 改一道门要等整档；单门自检只能手搓 `python3 scripts/gates/check-error-kinds.py --self-test`（17 行）。
//
// 本件**只补两格**，执行面一个字都没搬进来：
//
//	① **步名逐字校验**（执行前判）：真源 = 脚本自己的 `add_step` 行（`parseAddSteps`，与 `gate explain`
//	   同一份解析）。未知名 ⇒ 退 2 并给「最像的合法输入」（K14 第三件 · 与 `agent` 族**同一方言**）；
//	   重名（歧义）⇒ 退 2。它只在**跑之前**判，不改任何脚本退码。
//	② `--json <字段>`：给这一步的判决出一份**契约形状**的包封。
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
// 「执行前判」与「K2 的 `--json` 不给字段」两处退 2 / 1。
func gateRunStepJSON(inv *invocation, stdout, stderr io.Writer, root, script string, tail []string) int {
	// K2（§4.1）：给了 `--json` 但不给字段 ⇒ 退 1 且 stdout 0 字节（与 `requireFields` 同一口径）。
	if len(inv.fields) == 0 {
		if !requireFields(inv, stderr) {
			return exitFail
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

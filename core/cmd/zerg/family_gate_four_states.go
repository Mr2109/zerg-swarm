// family_gate_four_states.go —— `zerg gate four-states --current <命令串> --patched <命令串>`（T11）。
//
// 病灶（逐字）：四态亲测（现役件与改件各带／不带 `--yes` 各一次 · 四格逐格比对）**没有正门** ——
// 今天只能手搓四条命令再肉眼比。
//
// 四格 = {现役件, 改件} × {不带 `--yes`（只读面）, 带 `--yes`（写面）}：
//
//	A 现役·只读   B 现役·写面   C 改件·只读   D 改件·写面
//
// 判据（三条，缺一条即不成立）：
//
//	① 四格读数齐全 —— 缺任一格（跑不动/命令不可执行）⇒ **判不了** ⇒ 8，不许当绿
//	② 只读面 A 与 C **必须逐字节相同**（改件不得污染 dry-run 面）
//	③ 写面 B 与 D **不许逐字节相同**（恒同 ⇒ 改件未生效，可疑）
//
// ★ 退码：0 = 四格成立 · 1 = 有失败项 · 2 = 用法错 · 8 = 判不了
//
// ★ 命令串按**空白切词**，**不做 shell 展开**（无引号语义、无管道/重定向/变量替换）——
// 这是本命令的边界：它只跑「一条 argv」，不跑 shell。
// ★ 本命令**自己不起任何门禁**：跑什么由两条命令串**逐字**定；不读不写 `scripts/gates/` 下任何件。
package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// gateFourStatesFields —— `--json` 的字段表（每格一行三列：格名 / 退码 / stdout 字节数）。
var gateFourStatesFields = []string{"cell", "rc", "bytes"}

// gateFourStatesTimeout —— 单格上限（跑不动 ⇒ 判不了，不无限等）。
const gateFourStatesTimeout = 60 * time.Second

func gateFourStatesFieldLegal(f string) bool {
	for _, ok := range gateFourStatesFields {
		if f == ok {
			return true
		}
	}
	return false
}

// gateFourStatesCell —— 一格的读数（退码 / stdout 逐字节 / 跑不动的成因）。
type gateFourStatesCell struct {
	RC    int
	Out   []byte
	Err   string
	Empty bool
}

// gateFourStatesRun —— 跑一格：argv = 命令串切词 + 追加旗标。cwd 缺省 = 进程当前目录。
func gateFourStatesRun(line string, extra []string) gateFourStatesCell {
	argv := append(strings.Fields(line), extra...)
	if len(argv) == 0 {
		return gateFourStatesCell{Err: "命令串切词后是空的", Empty: true}
	}
	ctx, cancel := context.WithTimeout(context.Background(), gateFourStatesTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	out, err := cmd.Output()
	c := gateFourStatesCell{Out: out}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			c.RC = ee.ExitCode()
			c.Out = append(out, ee.Stderr...)
			return c
		}
		c.Err = err.Error()
		c.Empty = true
		return c
	}
	return c
}

// cmdGateFourStates —— `gate four-states`（只读于仓 · 不改任何脚本退码）。
func cmdGateFourStates(inv *invocation, stdout, stderr io.Writer) int {
	cur := strings.TrimSpace(inv.flagVal("--current"))
	pat := strings.TrimSpace(inv.flagVal("--patched"))
	yesFlag := strings.TrimSpace(inv.flagVal("--yes-flag"))
	if yesFlag == "" {
		yesFlag = "--yes"
	}
	if cur == "" || pat == "" {
		inv.setErr("usage", "missing_sides", "缺 --current / --patched")
		fmt.Fprintf(stderr, "%s: `gate four-states` 要给两条命令串：`--current <现役件命令串>` 与 `--patched <改件命令串>`\n", progName)
		fmt.Fprintf(stderr, "口径：四格 = {现役, 改件} × {不带 %s, 带 %s}\n", yesFlag, yesFlag)
		return exitUsage
	}
	if cur == pat {
		inv.setErr("usage", "same_sides", "两条命令串逐字相同")
		fmt.Fprintf(stderr, "%s: `--current` 与 `--patched` **逐字相同** ⇒ 写面必然恒同（假红）—— 用法错 2\n", progName)
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate four-states` 不收位置参数（四格由两条命令串 + `--yes-flag` 定）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateFourStatesFields
	}
	for _, f := range fields {
		if !gateFourStatesFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}

	yes := []string{yesFlag}
	cells := []struct {
		name string
		c    gateFourStatesCell
	}{
		{"A 现役·只读", gateFourStatesRun(cur, nil)},
		{"B 现役·写面", gateFourStatesRun(cur, yes)},
		{"C 改件·只读", gateFourStatesRun(pat, nil)},
		{"D 改件·写面", gateFourStatesRun(pat, yes)},
	}
	byName := map[string]gateFourStatesCell{}
	for _, c := range cells {
		byName[c.name] = c.c
	}

	// ① 四格齐
	for _, c := range cells {
		if c.c.Empty {
			inv.setErr("blocked", "four_states_cell_unrunnable", c.name)
			fmt.Fprintf(stderr, "%s: **判不了** —— 格 %s 跑不动：%s ⇒ 不给结论（退码 8）\n", progName, c.name, c.c.Err)
			fmt.Fprintf(stderr, "口径：缺格既不是「成立」也不是「不成立」—— 不许当绿\n")
			return exitBlocked
		}
		if c.c.RC == 126 || c.c.RC == 127 {
			inv.setErr("blocked", "four_states_cell_not_executable", c.name)
			fmt.Fprintf(stderr, "%s: **判不了** —— 格 %s 的命令**不可执行**（rc=%d）⇒ 不给结论（退码 8）\n", progName, c.name, c.c.RC)
			return exitBlocked
		}
	}

	sameRead := string(byName["A 现役·只读"].Out) == string(byName["C 改件·只读"].Out)
	sameWrite := string(byName["B 现役·写面"].Out) == string(byName["D 改件·写面"].Out)
	fail := []string{}
	if !sameRead {
		fail = append(fail, "② 只读面 A≠C：改件污染了 dry-run 面")
	}
	if sameWrite {
		fail = append(fail, "③ 写面 B≡D：改件在写面无差异（恒同，可疑）")
	}

	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			inv.fields = fields
		}
		inv.metaAddStr("current", cur)
		inv.metaAddStr("patched", pat)
		inv.metaAddStr("yes_flag", yesFlag)
		inv.metaAddJSON("same_readonly", fmt.Sprintf("%t", sameRead))
		inv.metaAddJSON("same_write", fmt.Sprintf("%t", sameWrite))
		items := make([]map[string]string, 0, len(cells))
		for _, c := range cells {
			items = append(items, map[string]string{
				"cell": c.name, "rc": fmt.Sprintf("%d", c.c.RC), "bytes": fmt.Sprintf("%d", len(c.c.Out)),
			})
		}
		rc := selectJSONList(stdout, stderr, inv, inv.path, fields, items)
		if rc != exitOK {
			return rc
		}
		if len(fail) > 0 {
			return exitFail
		}
		return exitOK
	}

	fmt.Fprintf(stdout, "现役件：%s\n改件　：%s\n确认旗标：%s\n── 四格读数（退码 · stdout 字节数）──\n", cur, pat, yesFlag)
	for _, c := range cells {
		fmt.Fprintf(stdout, "  格 %-12s rc=%-3d bytes=%d\n", c.name, c.c.RC, len(c.c.Out))
	}
	fmt.Fprintf(stdout, "只读面 A==C ：%s\n", map[bool]string{true: "是", false: "否"}[sameRead])
	fmt.Fprintf(stdout, "写面   B==D ：%s\n", map[bool]string{true: "是", false: "否"}[sameWrite])
	if len(fail) > 0 {
		for _, f := range fail {
			fmt.Fprintf(stdout, "  FAIL %s\n", f)
		}
		fmt.Fprintf(stdout, "结论：%d 项失败（退 1）\n", len(fail))
		return exitFail
	}
	fmt.Fprintf(stdout, "结论：四格成立（退 0）—— 只读面逐字节相同、写面确有差异\n")
	return exitOK
}

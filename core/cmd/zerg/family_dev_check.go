// family_dev_check.go —— `zerg dev check --syntax`：**类型面只读正门**（GAP-20260927-377 · P0）。
//
// 病灶（缺口逐字）：gofmt 绿 ≠ 能编译 —— `routeWriteGate` 返回 `(int, bool)`，单值 `return rc`
// 的补丁 `gofmt -e/-l` 全绿却**过不了编译**；而施工卵的禁跑清单里含 `go build|vet` ⇒ 类型面
// **没有任何编译器背书**（本批施工卵自报：用 DEV-0240 修成 `return rc, false`，全靠自己读代码发现）。
//
// 本命令的落法：给施工类派单一条**只判定、不产物**的正门 —— 真跑一次编译 / 类型检查，
// 制品一律丢到 `os.DevNull`（`-o /dev/null`：不落任何制品、不碰 `bin/`、不改工作树），
// 把编译器 / `go vet` 的判决**原样转出**（rc + stderr 首行）。
//
// 形态（逐字）：
//
//	zerg dev check --syntax [--file <仓内相对 .go 件>]… | [--pkg <包>]… [--vet] [--json <字段>]
//
// 目标缺 ⇒ 2（不给结论 · 空转不许放绿）；仓根 / `core/` 模块解析不到 ⇒ 8（不猜）；
// `--vet` ⇒ 跑 `go vet`（含类型面），默认 ⇒ 跑 `go build -o /dev/null`（要**真编**那一条）。
//
// rc：0 类型面过 · 2 编译 / 类型不过 · 8 前置缺（读不到 ⇒ 不给结论）。
// 本命令**只读**（零写盘、零副作用 · 不进危险档清单）。
//
// ★ 边界（照实说）：本件由 lane R3 落；**命令树登记在 main.go（R1 独占）** ⇒ 登记前本命令
// **不可达**（行为面待重编）。同批随动（门78 新命令随动总账 · 阻断）见随件说明：
// `devFamilyClosure`（dev_proposal_test.go）· `cli-matrix.json` must-fail（R2/R4/R5）·
// `cli-contract-baseline.json` 计数 · `公开标记.tsv` + 两器排除 · 门69 `check-help-shape` BASELINE ·
// 公开面命令 counts。**不落齐这些面 ⇒ 门78 判红**（不得单独落 main.go 那一处）。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// devCheckFields —— `--json` 面上可取的全部字段（K1：机器面先定）。
var devCheckFields = []string{"pkg", "mode", "rc", "ok", "say"}

// devCheckTimeout —— 单包编译 / vet 的墙钟上限（秒）。超时 ⇒ 当「不给结论」那一档（8）。
const devCheckTimeout = 300 * time.Second

// cmdDevCheck —— `zerg dev check --syntax`：类型面只读正门（零制品 · 编译判决原样转出）。
func cmdDevCheck(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法规矩：多余位置参数 ⇒ 2（在任何盘面动作之前判 · 同族体例）。
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: dev check 不收位置参数（多给了 %q）—— 目标一律走 `--file` / `--pkg`\n", progName, inv.args[0])
		return exitUsage
	}
	// ② `--syntax` 是**本命令的语义旗标**（表明这是类型面只读检查那一档）—— 缺 ⇒ 2。
	if !inv.hasFlag("--syntax") {
		inv.setErr("usage", "missing_syntax_flag", "缺 --syntax")
		fmt.Fprintf(stderr, "%s: `dev check` 要给 `--syntax`（类型面只读正门那一档）—— 形态：%s dev check --syntax [--file <仓内相对 .go 件>]… | [--pkg <包>]… [--vet]\n", progName, progName)
		return exitUsage
	}
	files := trimAll(inv.flagVals("--file"))
	pkgs := trimAll(inv.flagVals("--pkg"))
	if len(files) == 0 && len(pkgs) == 0 {
		inv.setErr("usage", "missing_target", "缺目标（--file / --pkg）")
		fmt.Fprintf(stderr, "%s: `dev check --syntax` 至少要一个 `--file <件>` 或 `--pkg <包>` —— 空目标 ⇒ **不给结论**（不静默放绿）\n", progName)
		return exitUsage
	}
	// ③ 件名形状（同 `dev edit` 的作用域口径：仓内相对路径 · 不许绝对 / `..`）。
	if why := declaredFilesWhy(files); why != "" {
		inv.setErr("usage", "bad_file_path", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		return exitUsage
	}
	// ④ 仓根 / core 模块（读不到 ⇒ 8 · 不猜）。
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	coreDir := filepath.Join(root, "core")
	if st, err := os.Stat(filepath.Join(coreDir, "go.mod")); err != nil || st.IsDir() {
		inv.setErr("blocked", "core_module_absent", "core/ 模块不在盘上")
		fmt.Fprintf(stderr, "%s: core/go.mod 不在盘上（%s）⇒ 不给结论（退码 8 · 不猜）\n", progName, coreDir)
		return exitBlocked
	}
	// ⑤ 目标 → 包清单：`--file` 收**仓内相对**路径（要先剥掉 `core/` 模块前缀 —— `go` 的包路
	//    是相对 `core/` 模块根，不是相对仓根）；`--pkg` 收**模块相对**包路（如 `./cmd/zerg`）。
	//    去重保序；不在 `core/` 模块下的件 ⇒ 2（类型面只对 core 模块里的 Go 源）。
	var list []string
	seen := map[string]bool{}
	addPkg := func(p string) {
		p = filepath.ToSlash(p)
		p = strings.TrimPrefix(p, "./")
		p = strings.Trim(p, "/")
		if p == "" {
			p = "."
		}
		pkg := "./" + p
		if !seen[pkg] {
			seen[pkg] = true
			list = append(list, pkg)
		}
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			inv.setErr("usage", "not_go_file", "只收 .go 件")
			fmt.Fprintf(stderr, "%s: `--file %s` 不是 `.go` 件 —— 类型面只对 Go 源（别的面另有它的门）\n", progName, f)
			return exitUsage
		}
		dir := filepath.ToSlash(filepath.Dir(f))
		switch {
		case dir == "core":
			dir = "."
		case strings.HasPrefix(dir, "core/"):
			dir = strings.TrimPrefix(dir, "core/")
		default:
			inv.setErr("usage", "file_outside_core_module", "件不在 core 模块下")
			fmt.Fprintf(stderr, "%s: `--file %s` 不在 `core/` 模块下（本命令编的是 core 模块的 Go 包 —— 别的模块另有它的门）⇒ 退码 2\n", progName, f)
			return exitUsage
		}
		addPkg(dir)
	}
	for _, p := range pkgs {
		addPkg(p)
	}
	// ⑥ 编译 / vet 模式（默认 go build -o /dev/null —— 要「真编」那一条）。
	mode, argv0 := "build", []string{"build", "-mod=mod", "-o", os.DevNull}
	if inv.hasFlag("--vet") {
		mode, argv0 = "vet", []string{"vet", "-mod=mod"}
	}

	rows := []map[string]string{}
	failed := 0
	for _, pkg := range list {
		rc, say := devCheckRun(coreDir, argv0, pkg)
		ok := rc == 0
		if !ok {
			failed++
		}
		rows = append(rows, map[string]string{
			"pkg": pkg, "mode": mode,
			"rc": fmt.Sprintf("%d", rc), "ok": fmt.Sprintf("%t", ok), "say": say,
		})
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	for _, r := range rows {
		mark := "过"
		if r["rc"] != "0" {
			mark = "不过"
		}
		fmt.Fprintf(stdout, "  %-6s %-28s rc=%s → %s\n", r["mode"], r["pkg"], r["rc"], mark)
		if r["say"] != "" {
			fmt.Fprintf(stdout, "         ↳ %s\n", r["say"])
		}
	}
	if failed > 0 {
		inv.setErr("usage", "type_face_failed", fmt.Sprintf("%d/%d 个包类型面不过", failed, len(list)))
		fmt.Fprintf(stderr, "%s: 类型面**不过** —— %d/%d 个包没过（编译 / vet 判决见上 · 原样转出，不润色）⇒ 退码 2\n", progName, failed, len(list))
		return exitUsage
	}
	fmt.Fprintf(stderr, "%s: 类型面**过** —— %d 个包（mode=%s · 制品丢 /dev/null · 零制品 · GAP-20260927-377）\n", progName, len(list), mode)
	return exitOK
}

// devCheckRun 在 core 模块目录里跑一次 `go <argv0…> <pkg>`，返回 (rc, stderr 首行)。
// 超时 / 起不来 ⇒ rc=8（**不给结论** · 不冒充「不过」也不冒充「过」）；否则 rc = go 自己的退码。
func devCheckRun(coreDir string, argv0 []string, pkg string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), devCheckTimeout)
	defer cancel()
	args := append(append([]string{}, argv0...), pkg)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = coreDir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	var eb strings.Builder
	cmd.Stderr = &eb
	err := cmd.Run()
	say := firstNonEmptyLine(eb.String())
	if ctx.Err() == context.DeadlineExceeded {
		return exitBlocked, fmt.Sprintf("超时（%s）—— 不给结论", devCheckTimeout)
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), say
		}
		// 起不来 go（PATH 里没有）⇒ 不给结论（8），不冒充判决。
		return exitBlocked, "起不了 `go`：" + err.Error() + " —— 不给结论"
	}
	return 0, say
}

// firstNonEmptyLine 取一段文本里第一行非空（错误文案要给下一步且不许灌满屏 · stderr 可能很长）。
func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}

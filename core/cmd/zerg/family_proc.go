// family_proc.go —— `core ps`（`Q-056`）+ 「现值 vs 声明」两列的同源读数（`Q-057`）—— 波① `T2`。
//
// 两条缺口（`任务清单-缺口收口-20260923.md` §`T2` 逐字）：
//
//	① `zerg core ps` 给人读三格（`pid / 起时 / 命令行`）—— 今天**没有**这条命令（现跑「未知命令」）。
//	② `core daemon ls --declared` 的 `declared` 列**不再是全「否」**（今天五条脚本全「否」：匹配口径
//	   拿「脚本名」去比声明件的**第 2 列（服务名）**，而声明面把脚本写在**第 4 列（程序）**里）。
//	③ **同源**：幽灵条数 == `zerg doctor` 的「回收候选（幽灵服务）」条数（逐字同值）。
//
// 同源怎么保证（不是「抄一遍」）：**唯一一处**现值读数 = 下面的 `ghostProcesses`，
// `doctor` 的 ⑧ 与 `core daemon ls --declared` **都调它**；声明侧真源 = `deploy/服务声明.tsv`
// （该件表头逐字写着「本件是『现值 vs 声明』差集判定的**声明侧唯一真源**」）。
//
// 夹具口径与门⑧ `check-service-declaration.py` **同名同义**（便于两个面在同一份夹具上对拍）：
//
//	`ZERG_SVCDECL_DECL` 声明树路径（默认 `<仓根>/deploy/服务声明.tsv`）
//	`ZERG_SVCDECL_PS`   ps 输出落文件（默认真跑 `ps -eo pid=,lstart=,command=`）
//
// 只读：本族只跑 `ps`（不杀、不停、不改任何进程），也不写状态目录（判据：跑前后 `~/.zerg/` 逐件不变）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// procRow —— 现值面一条进程（`pid / 起时 / 命令行` 三格 + 它属于哪一条声明）。
type procRow struct {
	Pid     string
	Start   string
	Command string
	Kind    string // launchd / ghost
	Name    string // 声明件的服务名（幽灵行就是它自己的 name）
	Hint    string // 命中它的那一枚进程特征（可复核）
	Owner   string // 归属列（照声明件）
}

// svcDeclRow —— 声明面一行（`deploy/服务声明.tsv` 的七列里用到的四列）。
type svcDeclRow struct {
	Kind        string
	Name        string
	DeclFile    string
	Program     string
	ProcFeature string
	Owner       string
	Disposition string
}

// svcDeclPath 解析声明树路径（夹具 env → `--path` → 仓根默认件）。
func svcDeclPath(inv *invocation) (string, string) {
	if p := strings.TrimSpace(os.Getenv("ZERG_SVCDECL_DECL")); p != "" {
		return p, ""
	}
	if inv != nil {
		if p := strings.TrimSpace(inv.flagVal("--path")); p != "" {
			return p, ""
		}
		if r := strings.TrimSpace(inv.flagVal("--root")); r != "" {
			return filepath.Join(r, "deploy", "服务声明.tsv"), ""
		}
	}
	root := repoRoot()
	if root == "" {
		return "", "解析不到仓根（给 `--root <仓根>` / `--path <声明件>` 或设 `ZERG_REPO`）"
	}
	return filepath.Join(root, "deploy", "服务声明.tsv"), ""
}

// loadSvcDecl 读声明树 ⇒ 逐行（跳过空行/注释/表头）。读不到 ⇒ (nil, 原因)。
func loadSvcDecl(path string) ([]svcDeclRow, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("声明件读不到（%s）：%v", path, err)
	}
	rows := []svcDeclRow{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 || strings.TrimSpace(f[0]) == "kind" {
			continue
		}
		rows = append(rows, svcDeclRow{
			Kind:        strings.TrimSpace(f[0]),
			Name:        strings.TrimSpace(f[1]),
			DeclFile:    strings.TrimSpace(f[2]),
			Program:     strings.TrimSpace(f[3]),
			ProcFeature: strings.TrimSpace(f[4]),
			Owner:       strings.TrimSpace(f[5]),
			Disposition: strings.TrimSpace(f[6]),
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Sprintf("声明件里一条声明行都没有（%s）", path)
	}
	return rows, ""
}

// psLines 取现值面的 `ps` 行（夹具 env → 真跑 `ps`）。**只读**：跑一次 `ps`，不碰任何进程。
func psLines() ([]string, string) {
	if p := strings.TrimSpace(os.Getenv("ZERG_SVCDECL_PS")); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Sprintf("ps 夹具读不到（%s）：%v", p, err)
		}
		return splitNonEmpty(string(b)), ""
	}
	out, err := exec.Command("ps", "-eo", "pid=,lstart=,command=").Output()
	if err != nil {
		return nil, fmt.Sprintf("`ps` 跑不起来：%v", err)
	}
	return splitNonEmpty(string(out)), ""
}

func splitNonEmpty(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// parsePSLine 摊一行 ps 成三格。两种形状都认（**不猜**：按第二格是不是纯数字分档）：
//
//	活体（`ps -eo pid=,lstart=,command=`）：`pid 起时（5 格：周 月 日 时 年）命令行`
//	夹具（门⑧ 同名同义 `ps -eo pid,ppid,etime,command`）：`pid ppid 起时(etime) 命令行`
func parsePSLine(line string) (pid, start, command string, ok bool) {
	f := strings.Fields(line)
	if len(f) < 3 {
		return "", "", "", false
	}
	pid = f[0]
	if !isDigits(f[1]) { // 活体形状：起时是 5 格
		if len(f) < 7 {
			return "", "", "", false
		}
		return pid, strings.Join(f[1:6], " "), strings.Join(f[6:], " "), true
	}
	// 夹具形状：第二格是 ppid，第三格是 etime（起时用**已运行时长**表示，与门⑧ 同一口径）
	if len(f) < 4 {
		return "", "", "", false
	}
	return pid, f[2], strings.Join(f[3:], " "), true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// procTable 现读一次现值面（摊成三格），并按**声明件点名的进程特征**打标。
// 返回 (rows, 原因)：原因非空 ⇒ 不给结论。
func procTable(decl []svcDeclRow) ([]procRow, string) {
	lines, perr := psLines()
	if perr != "" {
		return nil, perr
	}
	rows := []procRow{}
	seen := map[string]bool{}
	for _, r := range decl {
		if r.ProcFeature == "" {
			continue
		}
		for _, line := range lines {
			if !strings.Contains(line, r.ProcFeature) {
				continue
			}
			pid, start, cmd, ok := parsePSLine(line)
			if !ok || seen[pid] {
				continue
			}
			seen[pid] = true
			rows = append(rows, procRow{Pid: pid, Start: start, Command: cmd,
				Kind: r.Kind, Name: r.Name, Hint: r.ProcFeature, Owner: r.Owner})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Pid < rows[j].Pid })
	return rows, ""
}

// ghostProcesses —— 「**现值有 · 声明无**」的幽灵服务条（`kind=ghost` 的声明行 × 现值命中）。
//
// ★ **唯一一处**幽灵读数：`zerg doctor` 的 ⑧「回收候选（幽灵服务）」与
// `zerg core daemon ls --declared` 的幽灵段**都调它** ⇒ 两处条数**同源同值**（`T2` 判据③）。
// 口径：`kind=ghost` 的声明行的**进程特征**出现在 `ps` 里即命中（照 §九 M9：只报 + 干跑单列「不属管辖」）。
func ghostProcesses() ([]procRow, string) {
	path, perr := svcDeclPath(nil)
	if perr != "" {
		return nil, perr
	}
	decl, derr := loadSvcDecl(path)
	if derr != "" {
		return nil, derr
	}
	ghosts := []svcDeclRow{}
	for _, r := range decl {
		if r.Kind == "ghost" {
			ghosts = append(ghosts, r)
		}
	}
	if len(ghosts) == 0 {
		// 声明面没有 ghost 行 ⇒ 退回**内置特征**（`doctor` 一直在用的那一枚），
		// 这样「声明件读不到/没登记幽灵」也不会让 doctor 的 ⑧ 变成假绿。
		ghosts = []svcDeclRow{{Kind: "ghost", Name: "cocoon-docs-service", ProcFeature: "cocoon-docs-service"}}
	}
	rows, perr := procTable(ghosts)
	if perr != "" {
		return nil, perr
	}
	return rows, ""
}

// ghostGhint 把幽灵条按 `doctor` 的既有形状摊平（`pid=… 命令行`），确保两处**逐字同值**。
func ghostBrief(rows []procRow) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, "pid="+r.Pid+" "+r.Command)
	}
	return out
}

// ---- `zerg core ps`（Q-056）----------------------------------------------------------------

// cmdCorePs —— `zerg core ps`：现值面给人读 **三格**（`pid / 起时 / 命令行`），只读。
//
// 行面 = 声明件点名的进程特征命中的现值进程（`launchd` 声明的 + `ghost` 幽灵的**都列**，
// 用 `kind` 分档）—— 这样「看得见幽灵」不必先跑 `doctor`。
func cmdCorePs(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	path, perr := svcDeclPath(inv)
	if perr != "" {
		inv.setErr("blocked", "decl_absent", perr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）—— 不硬造一份声明面\n", progName, perr)
		return exitBlocked
	}
	decl, derr := loadSvcDecl(path)
	if derr != "" {
		inv.setErr("blocked", "decl_unreadable", derr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, derr)
		return exitBlocked
	}
	rows, perr2 := procTable(decl)
	if perr2 != "" {
		inv.setErr("blocked", "ps_unreadable", perr2)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, perr2)
		return exitBlocked
	}
	out := []map[string]string{}
	for _, r := range rows {
		out = append(out, map[string]string{
			"pid": r.Pid, "start": r.Start, "command": r.Command,
			"kind": r.Kind, "name": r.Name, "owner": r.Owner,
		})
	}
	if len(out) == 0 {
		// 空表**不是**「算完了」：一件都没看到 ⇒ 报出来但把口径写明（不当绿也不当红）
		fmt.Fprintf(stderr, "%s: 现值面一件自研件都没扫到（声明件 %s 点名的进程特征一个都没命中）—— 照实报空表\n", progName, path)
	}
	fmt.Fprintf(stderr, "%s: 声明件 %s · 现值 %d 条（三格 = pid / 起时 / 命令行 · 只读：没杀、没停、没改任何进程）\n", progName, path, len(out))
	return listCmd(inv, stdout, stderr, []string{"pid", "start", "command", "kind", "name", "owner"}, out)
}

// declaredForScript —— 脚本 ↔ 声明行的**修好的口径**（`Q-057` 的病根就在这一处）：
// 用「**程序**列里带该脚本的仓根相对路径」匹配（今天：`com.zerg.core` 的程序列 =
// `/bin/bash scripts/svc/zerg-core-daemon.sh`），而不是拿脚本名去比**服务名**列。
func declaredForScript(decl []svcDeclRow, scriptRel string) (svcDeclRow, bool) {
	for _, r := range decl {
		if r.Kind == "launchd" && strings.Contains(r.Program, scriptRel) {
			return r, true
		}
	}
	return svcDeclRow{}, false
}

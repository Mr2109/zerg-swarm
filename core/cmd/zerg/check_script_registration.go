// check_script_registration.go —— 门135 **脚本登记正门**（缺口 `GAP-20261020-03`）。
//
// 病（缺口正文逐字）：「登记面发散：同一个『新增一件』的事实要在五处分别声明（公开标记.tsv、
// Zerg-内部文档 脚本台账、门⑩ 干跑语义表 dryrun-semantics.json、门步骤表、契约矩阵/基线），
// 五处互不联动也无一处能推导出其余四处……缺一条『新件登记一次、其余面自动生成/校验』的正门。」
//
// 已有的三条腿（本件**不复制**它们的口径，只补缺的那条）：
//
//	上半 `registry_fanout_test.go` —— 判据件（基线棘轮）：现读缺的进基线（只许缩减），
//	  新增一件没在两面表态 ⇒ go test 红并点名。⚠ 它是**测试档**（基线自举那趟直接 Skip），
//	  且只判「相对基线有没有新增」⇒ 存量缺口永不红。
//	下半 `family_registry_sync.go` —— 补面（`zerg registry sync`）：把能由「件本身」推出来的
//	  那一面补齐（`scripts/公开标记.tsv` 缺行 ⇒ 尾部追加一行，第 3 列逐字标「自动补登 ·
//	  待人工确认用途」）。⚠ 它**只写不判**（缺面也算成功，rc=0）。
//	门76 `check_newfile_registry.go` —— 十面总闸，面① 正是「公开标记表缺登记」。
//	  ⚠ 现读挂的是 `tri-report`（**只报告 · 不接闸**），且整门带 20 条存量缺口 ⇒
//	  它**不会**因为「新加一件没登记」把提交拦住。
//
// ⇒ 本件 = 缺的那条腿：**把「scripts/ 下的每一件在登记表有一行」做成阻断档的门**，
// 让「新增一件却没登记」在提交闸上**被拦**，并且**点名治法**（机器写入口就是 `zerg registry sync`）。
//
// ★ 单一真源（✗ 不新造第二份口径 · ✗ 不新造第二张表）：
//
//	· 脚本面 = **同包** `gnrScanLedgerFace(root)`（`check_newfile_registry.go` · 门76 面① 的同一个扫描器：
//	  13 个排除目录 · 符号链接不下降 · 无后缀件须有执行位且首行 `#!`）⇒ 本件**没有**自己的目录遍历规则。
//	· 登记表 = **同包** `gsrRegistered(root)`（`check_gate_script_registered.go` · py 原件的逐字搬法：
//	  取 `公开标记.tsv` 第一列 · 跳过空行与 `#` 注释行 · universal newlines）。
//	· 登记行本身 = `pathMarksTable` 常量（`paths_canonical.go` · 全包唯一路径字面）。
//
// 判什么（一条断言 · 两态）：
//
//	A 未登记（阻断）  脚本面里每一件都必须在登记表第一列出现一次；缺一件即 rc=1 并逐条点名。
//	  治法 = `zerg registry sync`（机器补行 · 与门76 面④ 同一写入口；本门**只读不写**）。
//	B 死行（旁栏）    登记表里有行、盘上却无此件 ⇒ 打印点名，**不进退码**。
//	  为什么不拦：本仓有「件退场、行后补」的既有节奏（退场另走正门），
//	  且并行开发时别人的未跟踪件随时落盘/撤走 ⇒ 拿它当阻断会红在他人的半成品上。
//	  这一面归「登记面卫生」，✗ 不冒充「新增件被拦」。
//
// 退码（三档齐）：
//
//	0 判到了且未登记 0 件 · 1 判到了且有未登记件（逐条点名）· 2 不给结论（仓根取不到 /
//	  `scripts/` 不在盘 / 脚本面为 0 件 / 登记表读不到 —— 均 fail-closed，✗ 不许当绿）。
//
// 声明 (a) 默认根定位：本件**不是 py 搬运**（无原件可对拍），默认根走同包 `repoRoot()`
// （env `ZERG_REPO` → 可执行件上溯 → cwd 上溯，锚 `core/internal/version/version.go`）。
// 沙箱/测试一律显式 `--root`（或 env `ZERG_REPO`），✗ 不靠 cwd 猜。
//
// 声明 (b) 自指面：
//
//	① 本件落 `core/cmd/zerg/` ⇒ **不在**自己的扫描面内（扫描面只遍历 `<root>/scripts/**`）。
//	② 本件正文不写任何被扫件的**可执行内容**，只读路径集合 ⇒ 无「把自己扫红」的标记字面。
//	③ 本件零绝对私有路径字面（家目录一律 `repoRoot()` 现算）⇒ 不进 `check-hardcoded-private-paths` 面。
//	④ 本件不写盘、不起子进程、不碰网络、零计时 ⇒ 默认档**零副作用**（`check-write-inventory` 面内）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// csrMarksRel —— 登记表相对路径（唯一真源：同包常量 · ✗ 不写字面）。
const csrMarksRel = pathMarksTable

// csrSyncHint —— 机器写入口（治法逐字给命令 · 与人读文档同一串）。
const csrSyncHint = "zerg registry sync"

// csrResult —— 一次判读的全部读数（纯计算 + 只读盘面）。
type csrResult struct {
	Root      string // 仓根（绝对或调用方给的原样）
	RootFrom  string // 仓根取法（给人读）
	MarksPath string
	Scanned   int      // 脚本面件数
	MarkRows  int      // 登记表行数（含非脚本行：表是六道门共用）
	Unreg     []string // 未登记（排序）
	Dead      []string // 死行（排序 · 旁栏）
	NoScan    bool     // scripts/ 不在盘或脚本面 0 件
	MarksGone bool     // 登记表读不到
	ErrDetail string
}

// csrJudge —— 纯判读：脚本面（复用门76 扫描器）⟷ 登记表（复用门③ 解析器）。
func csrJudge(root, from string) csrResult {
	res := csrResult{Root: root, RootFrom: from, MarksPath: filepath.Join(root, csrMarksRel)}
	scripts := gnrScanLedgerFace(root)
	if len(scripts) == 0 {
		res.NoScan = true
		if !gnrIsDir(filepath.Join(root, "scripts")) {
			res.ErrDetail = "scripts/ 不在盘"
		} else {
			res.ErrDetail = "scripts/ 下 0 件脚本（空域 ≠ 全登记）"
		}
		return res
	}
	marks, st := gsrRegistered(root)
	if st != 0 || marks == nil {
		res.MarksGone = true
		res.ErrDetail = "登记表读不到"
		return res
	}
	res.Scanned = len(scripts)
	res.MarkRows = len(marks)
	for rel := range scripts {
		if !marks[rel] {
			res.Unreg = append(res.Unreg, rel)
		}
	}
	sort.Strings(res.Unreg)
	// 死行只判「scripts/ 下的脚本后缀行」—— 登记表同时登记 .md/.json 等伴生件，
	// 那些件的存废不归本门（它们的门各自判）。
	for rel := range marks {
		if !strings.HasPrefix(rel, "scripts/") {
			continue
		}
		if !strings.HasSuffix(rel, ".sh") && !strings.HasSuffix(rel, ".py") {
			continue
		}
		if !gnrIsFile(filepath.Join(root, rel)) {
			res.Dead = append(res.Dead, rel)
		}
	}
	sort.Strings(res.Dead)
	return res
}

// csrUsageText —— 用法（未知旗标 → rc=2）。
const csrUsageText = "用法：zerg gate check check-script-registration [--root <仓根>]\n" +
	"  默认根 = env ZERG_REPO → 可执行件上溯 → cwd 上溯（锚 core/internal/version/version.go）\n" +
	"  判据：scripts/ 下每一件（.sh/.py/无后缀 shebang）须在 scripts/公开标记.tsv 第一列有一行\n"

// csrRun —— 打印 + 退码。
func csrRun(stdout, stderr io.Writer, root, from string) int {
	if root == "" {
		fmt.Fprintln(stderr, "⛔ 不给结论（rc=2）：仓根取不到（env ZERG_REPO / 可执行件上溯 / cwd 上溯 全失败）⇒ 请显式 `--root <仓根>`")
		return 2
	}
	res := csrJudge(root, from)
	if res.NoScan {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：%s（仓根 %s）⇒ 判不了「登记齐不齐」，✗ 不当作全登记\n", res.ErrDetail, root)
		return 2
	}
	if res.MarksGone {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：登记表读不到：%s ⇒ 判不了\n", res.MarksPath)
		return 2
	}
	fmt.Fprintln(stdout, "── 脚本登记正门（scripts/** ⇕ "+csrMarksRel+"）（GAP-20261020-03）──")
	fmt.Fprintf(stdout, "  仓根 %s（取法=%s）\n", res.Root, res.RootFrom)
	fmt.Fprintf(stdout, "  脚本面 %d 件 · 登记表 %d 行 · **未登记 %d 件**\n", res.Scanned, res.MarkRows, len(res.Unreg))
	for _, rel := range res.Unreg {
		fmt.Fprintf(stdout, "  ✗ 未登记：%s\n", rel)
	}
	if len(res.Unreg) > 0 {
		fmt.Fprintf(stdout, "     ⇒ 治法：`%s`（机器补一行 · 第 3 列逐字标「自动补登 · 待人工确认用途」）\n", csrSyncHint)
		fmt.Fprintf(stdout, "              要写清用途就手写一行：`<相对路径>\\t<yes|no>\\t<理由>`\n")
	}
	fmt.Fprintf(stdout, "  旁栏 · 登记行指向盘上不存在的件：%d（不进退码 · 登记面卫生另算）\n", len(res.Dead))
	for _, rel := range res.Dead {
		fmt.Fprintf(stdout, "     · %s\n", rel)
	}
	if len(res.Unreg) > 0 {
		fmt.Fprintf(stdout, "  判据结论：**未登记 %d 件** ⇒ rc=1（阻断：新增脚本必须先在登记表有一行）\n", len(res.Unreg))
		return 1
	}
	fmt.Fprintf(stdout, "  判据结论：未登记 0 件（脚本面 %d 件全部有登记行）⇒ rc=0\n", res.Scanned)
	return 0
}

// checkScriptRegistrationGateMain —— 入口（同族旗标面：`--root` · `-h/--help`）。
func checkScriptRegistrationGateMain(stdout, stderr io.Writer, argv []string) int {
	root, from := "", ""
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, csrUsageText)
			return 0
		case a == "--root":
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				fmt.Fprintln(stderr, "用法错：--root 缺值")
				fmt.Fprint(stderr, csrUsageText)
				return 2
			}
			i++
			root, from = argv[i], "--root"
		case strings.HasPrefix(a, "--root="):
			root, from = strings.TrimPrefix(a, "--root="), "--root"
			if root == "" {
				fmt.Fprintln(stderr, "用法错：--root 缺值")
				fmt.Fprint(stderr, csrUsageText)
				return 2
			}
		default:
			fmt.Fprintf(stderr, "用法错：不认的旗标/位置参数 %q\n", a)
			fmt.Fprint(stderr, csrUsageText)
			return 2
		}
	}
	if root == "" {
		root = repoRoot()
		switch {
		case root == "":
			from = "取不到"
		case os.Getenv("ZERG_REPO") == root:
			from = "env:ZERG_REPO"
		default:
			from = "可执行件/cwd 上溯"
		}
	}
	return csrRun(stdout, stderr, root, from)
}

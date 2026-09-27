// family_lsface.go —— `zerg ls-face`：件名统一出口（A1 = `core/internal/gitpaths`）的**命令面**。
//
// 出处：《设计-件名统一出口与调用点判据-v0.1.md》§4.1（P-7）· §六 P-10 · 任务清单 A2
// （缺口账 `GAP-20260927-216` 的 ②「统一出口」的命令面那一格）。
//
// 形态（照设计稿 §4.1 逐字）：
//
//	zerg ls-face [--tracked | --others | --staged | --diff] [--prefix <路径>] [--utf8 | --json <字段>]
//
// 三条铁律（本件的**全部**内容）：
//
//	① 取件名**只**经 A1 出口（`gitpaths.List`）—— 本件**零 git argv**：不 exec git、
//	   不拼 argv、不解析 git 输出。本命令就是那个出口的**命令面**（C-1 的调用点）。
//	② 四个面旗标**互斥且缺省不猜**：给 0 个 ⇒ 用法错 2；给 ≥2 个 ⇒ 用法错 2。
//	   （设计稿 §4.1 表里 `--tracked` 写「默认」—— 本件**不**把「没说」读成「说了 tracked」，
//	   理由与 §九 M8 三档互斥同：两种不同的真值不许并成同一个形状。）
//	③ `--json` 必带**字节面字段**（P-10 · 照 git2-rs `path_bytes` 形状）：
//	   `path_bytes` = 件名的**全量小写十六进制**（`hex.EncodeToString(Entry.Bytes())`）——
//	   无损（逐字节可还原）、ASCII 安全（不必转义）、可人眼逐字节复核；
//	   `path` 只是它的**派生**（`Entry.String()`，C-10「字符串只在末端派生」）。
//
// A1 导出面 ⟷ 本命令四个面的对拍（★ 2026-09-27 现读 `core/internal/gitpaths/gitpaths.go`
// 全部导出符号后逐条盖章；A1 的导出面**只有** `List`（+ `Entry` 三法 + 六值 `Face` +
// 三值 `QuoteMode` + 四个 `Option` 构造子 + `KQuotePath` + `EscapedNameError`））：
//
//	--tracked → gitpaths.FaceTracked                argv = `git -C <仓> ls-files -z [-- <前缀>]` ✓
//	--diff    → gitpaths.FaceDiff                   argv = `git -C <仓> diff --name-only -z`
//	            （U-2 口径：工作树 vs 索引）✓
//	--staged  → gitpaths.FaceDiffCached
//	            argv = `git -C <仓> diff --cached --name-only -z [-- <前缀>]`；git 官方语义 = 索引 vs HEAD
//	            （正 U-2 的 `--staged` 口径）。★ argv **仍全由 A1 拼** —— 本件一个 git 词都没写 ✓
//	            ★ 前缀现读：本面与 FaceTracked / FaceOthers 同吃 `--prefix`（A1 该面 args() 里带
//	            `pathspecSuffix(cfg)`，pathspec 落 `--` 之后）⇒ 映射表 `prefixOK` 已改 `true`
//	            ★ 2026-09-27 现读：A1 已补出 `FaceDiffCached` ⇒ 本面**不再借 rev 槽**。旧形态
//	            `FaceDiff + WithRev("--cached")` 是「语义对、形态不干净」：A1 的 FaceDiff 把 rev 拼在
//	            `--name-only -z` **之后**，而那一槽的 git 官方语义是「工作树 vs <rev>」；A1 的新面
//	            `FaceDiffCached` **有意硬拒 WithRev**（`git diff --cached <rev>` 的另一义 = 索引 vs
//	            <rev>）⇒ 留着那枚 rev 槽（填 `--cached`）会被新档当场硬失败（退码 8）
//	--others  → gitpaths.FaceOthers                 argv = `git -C <仓> ls-files --others
//	            --exclude-standard -z [-- <前缀>]` ✓（★ A1 于 2026-09-27 补出 `FaceOthers`
//	            后**已接上**本面，同吃 `--prefix`；先前 A1 表达不了该面、本件硬失败退 8
//	            的那条退路**已撤**，四面现为同一张映射表 + 同一套 `opt...` 的全同形）
//	            —— `FaceOthers` 与 `FaceTracked` 是**互补面**、不是同面的档位：索引面取不到
//	            未跟踪件、未跟踪面取不到索引件 ⇒ 两面不许互替（互替 = 静默错 —— 与 C-5
//	            拒绝清洗同一条道理）。`-z` 在**子命令之后**，且是该面的**硬要求**：本面
//	            **会转义**，不带 `-z` 时非 ASCII 件名走 git 默认档，C-5 自检必命中 ✓
//
// 退码（真源 `zerg help exit-codes` / `exitcodes.go`）：0 出表 · 2 用法错
// （0 个或 ≥2 个面旗标 · `--utf8` 与 `--json` 同给 · `--prefix` 给了空值或给了
// 不吃前缀的面）· 8 不给结论（仓根解析不到 · 出口报错 · C-5 自检命中）。
//
// ★ 未做项（本单不碰，留给 A3 · 命令面同批五件）：`core/cmd/zerg/testdata/cli-matrix.json` ·
// `scripts/gates/cli-contract-baseline.json` · `docs/*/参考-命令行.md` ·
// `publish/docs/CLI.*.md` · 门⑩ `dryrun-semantics.json` 一律**未改**（新增命令未入矩阵
// ⇒ 契约/门禁本轮**必红**，属预期，不是缺陷）。
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/gitpaths"
)

// lsFaceUsage —— 本条命令的**完整用法串**（唯一真源：登记条目与各条用法错路径都读它）。
const lsFaceUsage = "zerg ls-face [--tracked | --others | --staged | --diff] [--prefix <路径>] [--utf8 | --json <字段>]"

// lsFaceFields —— `--json <字段>` 的字段表（唯一真源：`main.go` 的登记条目读它）。
// `path_bytes` = P-10 要的字节面（全量小写十六进制）；`path` 是它的 UTF-8 派生；`bytes` 是件名字节数。
var lsFaceFields = []string{"path", "path_bytes", "bytes"}

// lsFacePlan —— 命令面的一面 → A1 出口的**一次调用**（形参只有 A1 的 Face 与 A1 的 Option）。
type lsFacePlan struct {
	flag     string        // 面旗标（逐字）
	face     gitpaths.Face // 传给 gitpaths.List 的面
	rev      string        // 非空 ⇒ 传 gitpaths.WithRev(rev)
	prefixOK bool          // A1 的该面吃不吃 WithPrefix（现读：FaceTracked / FaceOthers / FaceDiffCached 三面都吃）
}

// lsFacePlans —— 四个面旗标到 A1 出口的**唯一**映射表（顺序 = 帮助面与错误面的枚举序）。
// ★ 四面**同形**：一律只填 A1 的面值（+ 必要时 `rev`）与 `prefixOK`，没有旁路、没有缺口位。
var lsFacePlans = []lsFacePlan{
	{flag: "--tracked", face: gitpaths.FaceTracked, prefixOK: true},
	{flag: "--others", face: gitpaths.FaceOthers, prefixOK: true},
	{flag: "--staged", face: gitpaths.FaceDiffCached, prefixOK: true},
	{flag: "--diff", face: gitpaths.FaceDiff},
}

// lsFacePlanOf —— 按键取映射（构造上只会被在册的四个旗标调用）。
func lsFacePlanOf(flag string) (lsFacePlan, bool) {
	for _, p := range lsFacePlans {
		if p.flag == flag {
			return p, true
		}
	}
	return lsFacePlan{}, false
}

// lsFaceFlagNames —— 四个面旗标的名字（错误面与帮助面同一真源）。
func lsFaceFlagNames() []string {
	out := make([]string, 0, len(lsFacePlans))
	for _, p := range lsFacePlans {
		out = append(out, p.flag)
	}
	return out
}

// cmdLsFace —— `zerg ls-face` 的实现：判用法 → 调 A1 出口 → 两态渲染（真 UTF-8 行面 / 带字节面的 JSON）。
func cmdLsFace(inv *invocation, stdout, stderr io.Writer) int {
	// ── ① 四个面旗标**互斥且缺省不猜** ────────────────────────────────────────────────
	var chosen []string
	for _, p := range lsFacePlans {
		if inv.hasFlag(p.flag) {
			chosen = append(chosen, p.flag)
		}
	}
	if len(chosen) != 1 {
		names := strings.Join(lsFaceFlagNames(), " · ")
		if len(chosen) == 0 {
			inv.setErr("usage", "face_required", "四个面旗标一个都没给")
			fmt.Fprintf(stderr, "%s: `ls-face` 必须**恰给一个**面旗标（缺省不猜：没说 ≠ 说了 --tracked）\n", progName)
		} else {
			inv.setErr("usage", "face_conflict", "面旗标互斥")
			fmt.Fprintf(stderr, "%s: 面旗标**互斥**（给了 %d 个：%s）\n", progName, len(chosen), strings.Join(chosen, " "))
		}
		fmt.Fprintf(stderr, "可选：%s\n", names)
		fmt.Fprintf(stderr, "用法：%s\n", lsFaceUsage)
		return exitUsage
	}

	// ── ② `--utf8 | --json` 显式互斥（设计稿签名里的竖线；`--utf8` 本就是默认面）────────
	//  `--json` 不给字段的 K2 档由 `listCmd` → `requireFields` 一处决定（退码取自退码表）。
	if inv.hasFlag("--utf8") && inv.jsonGiven {
		inv.setErr("usage", "utf8_json_conflict", "--utf8 与 --json 互斥")
		fmt.Fprintf(stderr, "%s: `--utf8`（人面）与 `--json`（机器面）**互斥** —— 二选一\n", progName)
		fmt.Fprintf(stderr, "用法：%s\n", lsFaceUsage)
		return exitUsage
	}

	flag := chosen[0]
	plan, ok := lsFacePlanOf(flag)
	if !ok {
		inv.setErr("usage", "unknown_face", "面旗标不在映射表里")
		fmt.Fprintf(stderr, "%s: 认不得的面旗标 %q\n", progName, flag)
		return exitUsage
	}

	// ── ③ 仓根（出口必须指到具体仓 —— C-1）─────────────────────────────────────────
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 取不了件名（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}

	// ── ④ `--prefix`：A1 里**只有** FaceTracked / FaceOthers / FaceDiffCached 三面吃前缀（各面 args() 里
	//  都有 `pathspecSuffix(cfg)`）⇒ 别面（现读：只有 `--diff`）给了就**明着拒**，不静默丢 ──
	opt := []gitpaths.Option{}
	if inv.hasFlag("--prefix") {
		if !plan.prefixOK {
			inv.setErr("usage", "prefix_unsupported", plan.face.String()+" 面不吃 --prefix")
			fmt.Fprintf(stderr, "%s: `%s` 面不吃 `--prefix`（A1 的该面 argv 里没有 pathspec 槽）⇒ 退码 2，不静默忽略\n", progName, flag)
			fmt.Fprintf(stderr, "用法：%s\n", lsFaceUsage)
			return exitUsage
		}
		prefix := strings.TrimSpace(inv.flagVal("--prefix"))
		if prefix == "" {
			inv.setErr("usage", "empty_prefix", "--prefix 给了但不给值")
			fmt.Fprintf(stderr, "%s: `--prefix` 给了却没给值（空前缀不猜成「整仓」）⇒ 退码 2\n", progName)
			return exitUsage
		}
		opt = append(opt, gitpaths.WithPrefix(prefix))
	}
	if plan.rev != "" {
		opt = append(opt, gitpaths.WithRev(plan.rev))
	}

	// ── ⑤ 出口（A1）—— 本件的**唯一**外部调用 ──────────────────────────────────────
	entries, err := gitpaths.List(root, plan.face, opt...)
	if err != nil {
		// C-5 自检命中（P-5：**硬失败，不清洗**）与其它失败分开报 —— 判因不靠猜。
		var esc *gitpaths.EscapedNameError
		if errors.As(err, &esc) {
			inv.setErr("failed", "escaped_name", "C-5 自检命中：转义形态不得流出出口")
			fmt.Fprintf(stderr, "%s: ★ C-5 自检命中（硬失败，**不清洗**）：%v\n", progName, err)
			fmt.Fprintf(stderr, "这不是本命令的错，是 A1 出口按 P-5 定的规矩：件名带首尾引号 / `\\`+三位八进制一律拒收\n")
			return exitBlocked
		}
		inv.setErr("blocked", "face_read_failed", "出口取件名失败")
		fmt.Fprintf(stderr, "%s: 出口取件名失败：%v（不给结论 · 退码 8）\n", progName, err)
		return exitBlocked
	}

	// ── ⑥ 两态渲染（行面只出件名；JSON 带字节面）─────────────────────────────────────
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		b := e.Bytes() // 副本（出口的规矩）
		rows = append(rows, map[string]string{
			"path":       e.String(), // 派生：唯一把字节变成平台字符串的地方
			"path_bytes": hex.EncodeToString(b),
			"bytes":      strconv.Itoa(len(b)),
		})
	}
	fmt.Fprintf(stderr, "%s: 面 %s(%s) · 仓 %s · %d 件（顺序 = git 自己的输出顺序；出口不排序不去重）\n",
		progName, flag, plan.face, root, len(rows))
	// 行面 = 设计稿 §4.1 的 `--utf8`（真 UTF-8 文本行，人可读）；机器面由 listCmd 的 `--json` 支负责。
	return listCmd(inv, stdout, stderr, []string{"path"}, rows)
}

// eval_run_all.go —— 件 `scripts/build/run_all.sh`（49 行 · Agent 评测批跑总入口 · 现属面＝构建面/评测批跑）
// 的 Go **真搬**（原落法 (b) 壳转账 ⇒ 本单升为 (a) 真搬）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A7；
// 待办表「壳转账 16」卡点逐字：**「批跑逻辑未搬；且层2/3 真调 `_archive/v25_*.py`（14 件·已归档）」**。
//
// 落法：把原件的**实际行为**（参数解析 / 缺省值 / 层序 / mkdir / `2>&1|tee` 合并与落档 / 输出文案 /
// `set -e` 退码语义）搬进 Go，**✗ 不再 exec 脚本**（不再 `bash scripts/build/run_all.sh`）⇒
// 脚本移出仓后本件仍逐字同、✗ 不退 2/8。
//
// 逐条声明（原件 49 行的每一段都在这三档里落位 —— ✗ 无一段「假装已搬」）：
//
//	① **真搬（Go 承）**：`MODEL`/`VERSION` 取值与缺省（`${1:-example-35b}` /
//	   `${2:-$(date +%Y%m%d-%H%M)}`）· `BASE`（仓根）· `EVALS`（`${ZERG_EVALS_DIR:-<仓根>/../zerg-evals}`）·
//	   `RUN_DIR` 拼法（`$EVALS/results/agent/runs/$MODEL-$VERSION` · 逐字拼接不归一）· 三段层头/层尾
//	   `echo` 文案 · `mkdir -p` · 层1 的 `(cd <仓根>/core && go test ./internal/agent/)` ·
//	   三档 `tee <RUN_DIR>/layerN-*.txt`（fd2→fd1 合并后**既落档又回显**）· `summary.json` heredoc 正文 ·
//	   尾两行 `echo` · `set -e` 退码语义。
//
//	② **保留薄壳（进程调用 · 逐条声明为「不可等价 Go 化」）**：三层的**重活**本身是外部进程 ——
//	   层1 `go test`、层2/3 `python3 <载荷> --model … --workdir …`。本件**直接起这三个子进程**
//	   （✗ 不经脚本），argv / 工作目录 / 合并语义与原件逐字同。
//
//	③ **如实声明为待处理（✗ 不假装已搬）**：层2/3 的**载荷** `scripts/evals/_archive/v25_example-35b-v2_tasks.py`
//	   与 `…/v25_complex_tasks.py` 是 **已归档** 的 `v25_*.py`（14 件之一）且**需活模型**
//	   （载荷内部 `go run ./cmd/zerg-agent -model <模型>` 打生产链路 · 见 `family_eval.go` 的「在跑实现」注）
//	   ⇒ **不可等价 Go 化**（Go 化＝重写整套评测载荷，超出「搬运」范围）。本件**保留其进程调用**
//	   （行为一字不改）并在件头点名：**这 2 层 = 「薄壳 ＋ 待处理」，不是已搬**；其**真载荷读数对拍**
//	   需活模型 ⇒ **未跑**（见测试件「不可跑层」声明）。
//
// 出厂声明 (a) 默认根定位差异：原件用 `$BASH_SOURCE` 上溯定位 `BASE`（`dirname/../..`）与
// `EVALS`（`dirname/../../../zerg-evals`）；本件无 `__file__` 语境 ⇒ `BASE` 走 `repoRoot()`
// （锚 `core/internal/version/version.go`），`EVALS` 走「同名旋钮 `ZERG_EVALS_DIR` 优先 →
// 回落 `<仓根>/../zerg-evals`（与原件同一条推导）」。**脚本位置 ≠ 仓根** 时两侧可能不同根
// （原件按**脚本所在**推根、本件按**仓根判据件**推根）；同一仓内二者一致。
// 取不到仓根 ⇒ **fail-closed rc=8**（不给结论 · 不猜一个仓根）。
//
// 出厂声明 (b) 自指检查：本件落在 `core/cmd/zerg/*.go` 全件扫描面内 ⇒ 逐条消解：
//   - 零私有绝对路径字面量（只有仓根相对串）；
//   - 零命令树层级字面量（本件**不进命令树** · ✗ 不在 `main.go` 命令表登记 —— 登记＝另一路独占）；
//   - 件内 kind 字面量只在登记函数首参位且取自理有闭集（`blocked`），件头一律描述式。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// evalRunAllSrcRel —— 被真搬的原件（仓根相对路径 · **只作来源说明/审计口径，✗ 不 exec 它**）。
const evalRunAllSrcRel = "scripts/build/run_all.sh"

// evalRunAllDefaultModel —— 原件 `MODEL="${1:-example-35b}"` 的缺省（逐字）。
const evalRunAllDefaultModel = "example-35b"

// 三层各自的**重活**（外部进程 · 件头 ② 档）。层1 的靶是**包**（`./internal/agent/`，跑在 `<仓根>/core`）；
// 层2/3 的靶是**已归档载荷**（③ 档 · 相对路径由 cwd 解析 —— 与原件同一条相对语义）。
const (
	evalRunAllLayer1Target = "./internal/agent/"
	evalRunAllLayer2Target = "scripts/evals/_archive/v25_example-35b-v2_tasks.py"
	evalRunAllLayer3Target = "scripts/evals/_archive/v25_complex_tasks.py"
)

// 四件落档名（原件逐字）。
const (
	evalRunAllLayer1File = "layer1-unit.txt"
	evalRunAllLayer2File = "layer2-basic.txt"
	evalRunAllLayer3File = "layer3-complex.txt"
	evalRunAllSummaryRel = "summary.json"
)

// evalRunAllClock —— 「现在」的唯一取点（默认 `time.Now` · 生产路一字不动）。
//
// 为什么留这条缝：原件两处 `date`（缺省版本号 `+%Y%m%d-%H%M` · `summary.json` 的 `date` 字段
// `+%Y-%m-%d %H:%M`）都取墙上时钟 ⇒ 逐字对拍要把两侧钉到**同一次脚本跑读到的那个时刻**
// （照 M2 口径：py 自己会写的时刻用注入旋钮钉，✗ 不靠掩码）。
var evalRunAllClock = time.Now

// cmdEvalRunAll —— `zerg eval run-all [<模型> [版本标签]]`（A7 的 Go 正门 · 真搬）。
//
// 退码口径（**逐档照原件**）：
//
//	0  三层跑完并落档（★ 原件 `set -e` 但**未**设 `pipefail` ⇒ 层1/2/3 的
//	   `cmd 2>&1 | tee` 这颗管道的退码是 **tee 的**（恒 0）⇒ 子进程失败**不会**中断、层2/3 的
//	   `|| echo "⚠️ …"` 也**只在 tee 本身失败时**才出 ⇒ 本件逐条照搬）。
//	1  原件 `set -e` 真会中断的两档：`mkdir -p` 失败、层1 子壳的 `cd <仓根>/core` 失败
//	   （★★ 管道退码是 tee 的 ⇒ 层1 `go test` 红**不**中断 —— 别把它读成中断）。
//	8  取不到仓根 / 缺 `<仓根>/../zerg-evals`（`EVALS=""` 后 `set -e` 中断）⇒ 不给结论。
func cmdEvalRunAll(inv *invocation, stdout, stderr io.Writer) int {
	// 位置参数：`zerg eval run-all` 之后的**一切**（`inv.orig` 去掉族/动作两段）。
	// 原件只吃 `$1`（模型）与 `$2`（版本标签），**第 3 个起静默丢弃** ⇒ 照搬。
	tail := []string{}
	if inv != nil && len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	// `${1:-默认}`：**空串按未给**处理（`:-` 语义）⇒ `tail[0] == ""` 时落缺省。
	model := evalRunAllDefaultModel
	if len(tail) >= 1 && tail[0] != "" {
		model = tail[0]
	}
	version := evalRunAllClock().Format("20060102-1504")
	if len(tail) >= 2 && tail[1] != "" {
		version = tail[1]
	}

	// BASE = 仓根（原件 `dirname($BASH_SOURCE)/../..`）。
	base := repoRoot()
	if base == "" {
		if inv != nil {
			inv.setErr("blocked", "eval_run_all_repo_root_absent", "解析不到仓根")
		}
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 判不了评测批跑（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}

	// EVALS = ${ZERG_EVALS_DIR:-<仓根>/../zerg-evals}（原件 `dirname/../../../zerg-evals` 的推导结果）。
	// ★ 原件这一档是 `$(cd … && pwd)` ⇒ 目录**不在**时命令替换为空、赋值非零 ⇒ `set -e` 中断（退非零）。
	evals := os.Getenv("ZERG_EVALS_DIR")
	if evals == "" {
		evals = filepath.Dir(base) + "/zerg-evals"
		if fi, err := os.Stat(evals); err != nil || !fi.IsDir() {
			fmt.Fprintf(stderr, "%s: 判不了评测批跑 —— 缺 zerg-evals（%s）\n", progName, evals)
			return exitFail
		}
	}

	// RUN_DIR = `$EVALS/results/agent/runs/$MODEL-$VERSION`（**逐字拼接** · ✗ 不走 Clean）。
	runDir := evals + "/results/agent/runs/" + model + "-" + version

	fmt.Fprintf(stdout, "🐛 虫族 Agent 测试——模型: %s | 版本: %s\n", model, version)
	fmt.Fprintf(stdout, "📁 存档: %s\n", runDir)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: mkdir -p 存档目录失败：%v\n", progName, err)
		return exitFail
	}

	// ── 层1: 框架单测（Go 单元测试——不依赖模型）────────────────────────────────
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "=== 层1: 框架单测 ===\n")
	coreDir := base + "/core"
	if fi, err := os.Stat(coreDir); err != nil || !fi.IsDir() {
		// 原件这一档：子壳 `cd` 失败 ⇒ 非零 ⇒ `set -e` 中断（✗ 不退缺省 0）。
		fmt.Fprintf(stderr, "%s: 判不了评测批跑 —— 层1 的 <仓根>/core 不可用（%s）\n", progName, coreDir)
		return exitFail
	}
	out := evalRunAllRun(exec.Command("go", "test", evalRunAllLayer1Target), coreDir, stderr)
	if err := evalRunAllTee(stdout, runDir+"/"+evalRunAllLayer1File, out); err != nil {
		fmt.Fprintf(stderr, "%s: 判不了评测批跑 —— 层1 落档失败：%v\n", progName, err)
		return exitFail
	}
	fmt.Fprint(stdout, "✅ 层1 完成\n")

	// ── 层2: 基础任务（6 项）───────────────────────────────────────────────────
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "=== 层2: 基础任务 ===\n")
	out = evalRunAllRun(exec.Command("python3", evalRunAllLayer2Target,
		"--model", model, "--workdir", "/tmp/zerg-agent-"+model+"-basic"), "", stderr)
	if err := evalRunAllTee(stdout, runDir+"/"+evalRunAllLayer2File, out); err != nil {
		fmt.Fprint(stdout, "⚠️ 层2 部分失败（见输出）\n")
	}
	fmt.Fprint(stdout, "✅ 层2 完成\n")

	// ── 层3: 复合任务（4 项）───────────────────────────────────────────────────
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "=== 层3: 复合任务 ===\n")
	out = evalRunAllRun(exec.Command("python3", evalRunAllLayer3Target,
		"--model", model, "--workdir", "/tmp/zerg-agent-"+model+"-complex"), "", stderr)
	if err := evalRunAllTee(stdout, runDir+"/"+evalRunAllLayer3File, out); err != nil {
		fmt.Fprint(stdout, "⚠️ 层3 部分失败（见输出）\n")
	}
	fmt.Fprint(stdout, "✅ 层3 完成\n")

	// ── 汇总（原件 heredoc 正文 · 逐字）────────────────────────────────────────
	summary := "{\n" +
		"  \"model\": \"" + model + "\",\n" +
		"  \"version\": \"" + version + "\",\n" +
		"  \"date\": \"" + evalRunAllClock().Format("2006-01-02 15:04") + "\",\n" +
		"  \"layers_run\": [\"layer1-unit\", \"layer2-basic\", \"layer3-complex\"],\n" +
		"  \"note\": \"完整结果见各 layer*.txt\"\n" +
		"}\n"
	if err := os.WriteFile(runDir+"/"+evalRunAllSummaryRel, []byte(summary), 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: 判不了评测批跑 —— 写 summary.json 失败：%v\n", progName, err)
		return exitFail
	}

	fmt.Fprint(stdout, "\n")
	fmt.Fprintf(stdout, "🎉 测试完成——存档: %s\n", runDir)
	fmt.Fprintf(stdout, "查看: cat '%s/%s'\n", runDir, evalRunAllLayer3File)
	return exitOK
}

// evalRunAllRun 起一个子进程，把 **fd1 + fd2 合并** 收进一块 buf。
//
// 合并语义照原件 `cmd … 2>&1 | tee <件>`：`os/exec` 在 `Stdout == Stderr`（同一个 Writer）时
// 只开一条管道、**保写入序** ⇒ 与 shell 的 `2>&1|tee` 逐字节同。
// `dir == ""` ⇒ 继承当前进程 cwd（照原件层2/3 的**相对路径**语义：那只在 cwd = 仓根时解析得到）；
// 层1 传 `<仓根>/core`（照原件子壳的 `cd`）。
//
// ★ 返回值**只有输出**、**不含退码** —— 原件这颗管道未设 `pipefail` ⇒ 退码是 `tee` 的，
// 子进程退多少**都不**进入控制流（层2/3 的 `|| echo` 兜底因此实不可达）。
// 子进程**起不来**（缺 `go`/`python3`）⇒ 原件退非零但同样不中断；本件把该错**打到真 stderr**
// （✗ 不混进 tee 流 —— 原件那行由 bash 自己写、文案不可逐字复刻），控制流照旧继续。
func evalRunAllRun(cmd *exec.Cmd, dir string, stderr io.Writer) []byte {
	var buf bytes.Buffer
	cmd.Dir = dir
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintf(stderr, "%s: 子进程起不来（%s）：%v\n", progName, cmd.Path, err)
		}
	}
	return buf.Bytes()
}

// evalRunAllTee 照 `tee <件>`：把合并后的字节**既落档又回显**（先落档后回显 —— tee 的写序）。
// 返回 tee 自己的错（那个「管道退码」）：非 nil ⇒ 调用方照原件输出 `⚠️ 层N 部分失败（见输出）`。
func evalRunAllTee(stdout io.Writer, path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	_, err := stdout.Write(data)
	return err
}

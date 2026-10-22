// check_compress_ruler.go —— 门136 **压缩尺子硬闸**（P2 收尾 · 把尺子接成阻断门）。
//
// 病（P2 现状，逐字）：`core/internal/compress/eval/` 已经会跑批、会出 M2/M3/M5 读数，但**只在
// `go test` 里跑**（`eval/*_test.go`）。⇒ 引擎一退化（把必留信息挤出窗口 / 锚点丢掉），提交链上
// **没有一处会拦**：读数照样被写进报告，人看见了也只是「一行难看的数」。尺子量得出坏，量不出
// 「不许坏」。
//
// 本件 = 缺的那条腿：把「尺子的一次满格读数」做成**阻断档的门**。判据三条（任务单 §4 M5 的硬闸，
// 不是本件自定的阈值）：
//
//	A `MRR_win == 1.0000`  窗口 needle 一条都不许被挤出窗口。
//	B `MRR_rec == 1.0000`  真丢的 needle 一条都不许有（窗口 ∪ 盘上取回，见 RunBatch 的 M5 口径）。
//	C `AnchorLost == 0`    原文锚点一条都不许丢（绝对数，不是率）。
//
// 判据面（**唯一** · ✗ 不新造第二份样本表）：`eval.AllSamples()` —— 五类全量 50 条（observation 10
// + trajectory 10 + plan 10 + memory 10 + representation 10）；预算 `8192`、引擎 `free`（任务单 §5.1
// 的 free 列）。跑批入口也只有一处：`eval.RunBatch`（它内部只走 `compress.Apply`）。
//
// 为什么**不**用 `TwoClassSamples()`：那 20 条是**冻结 cohort**，专供
// `pruner_free_eval_regress_test.go` 的「修前基线逐字节一致」断言（样本面扩到 50 条后不许重算）。
// 本门判的是**当下引擎对全部五类**是否满格 ⇒ 取全量；拿 20 条当门面会让后三类（plan/memory/
// representation）整类**无人看守**。
//
// 退码（三档齐 · 与同族判据一致）：
//
//	0 判到了且三条全过 · 1 判到了但有一条没过（**逐条点名**丢的 needle / 丢锚点的样本）·
//	2 不给结论（跑批起不来 / cohort 缩水或分母为 0 / 用法错 —— 均 fail-closed，✗ 不许当绿）。
//
// 声明 (a) 默认根定位：本件**不是 py 搬运**（无原件可对拍）。样本面**在二进制里**（`eval.AllSamples()`
// 是代码生成的），跑批只往**仓外临时目录**写 `unload/` 与 `samples.json` ⇒ 本件**不读仓内任何文件**、
// 不需要 `--root`。这不只是省事：它意味着本门在**任何**工作目录下判的都是同一件事（✗ 不会因为
// 「根取错了」而静默判成绿）。
//
// 声明 (b) 自指面：
//
//	① 本件不写任何仓内件：跑批落点 = `os.MkdirTemp`（defer 清掉）；`--work` 只给人显式指定落点。
//	② 本件正文零私有绝对路径字面（家目录/仓外卷一律不写）⇒ 不进 `check-hardcoded-private-paths` 面。
//	③ 本件不写 error.kind / 层级常量 ⇒ `check-error-kinds` / 茧壁层级门命中 0。
//	④ 本件退码只用 0/1/2（与 `check_script_registration.go` 同形，具名注释在返回值上）⇒ 无表外裸数字。
//	⑤ 本件不扫 `scripts/**` ⇒ 不在门135（脚本登记正门）的扫描面内；本件自己的新件登记走
//	   `scripts/公开标记.tsv`（由门78 面④ / 门76 面① 现读点名）。
//
// `--budget` 这一格（**唯一**的非判据旋钮，写清用途）：判据预算恒为 8192；`--budget N` 只给
// **负控**用 —— 把窗口压小（例如 2048）后本门**必须红并点名**，这就是「门有牙」的现场证明
// （`--self-test` 内部跑的就是这一对）。✗ 不许拿它当「放宽判据」用：判据三条与预算无关，
// 变的是引擎被压到哪一档。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/compress/eval"
)

// czrBudget —— 判据预算（任务单 §5.1 free 列）。✗ 不随 `--budget` 改这一格。
const czrBudget = 8192

// czrEngine —— 本门只认 free 引擎（summary/external 未接模型 ⇒ 不编读数）。
const czrEngine = "free"

// czrMinN —— cohort 下限：`eval.AllSamples()` 现读 50 条。少于这个数 ⇒ 样本面**缩水**，
// 分母跟着小 ⇒ 宁可退 2 也不许「在 3 条样本上满格」冒充全绿。
const czrMinN = 50

// czrVerdict —— 一次判读的读数（纯计算：喂 Report，不碰盘）。
type czrVerdict struct {
	Budget   int
	N        int
	Controls int
	WinDen   int
	RecDen   int
	AnchorN  int
	Win      float64
	Rec      float64
	Anchor   int

	WinLost    []string // 被挤出窗口的 needle（"R03#N1" 形）
	RecLost    []string // 窗口与盘上都取不回的 needle（真丢）
	AnchorLost []string // 有锚点丢的样本（"R07（2/5 锚点丢）"）

	WinOK    bool
	RecOK    bool
	AnchorOK bool
}

// czrPass —— 三条全过才算过（判据唯一出口）。
func (v czrVerdict) Pass() bool { return v.WinOK && v.RecOK && v.AnchorOK }

// czrJudge —— 纯判读：从跑批产物里取三条硬闸 + 逐条点名面。
//
// ★ 点名面取 `RunRow.WindowLostIDs` / `RecLostIDs`（RunBatch 逐条记录的原字段），**不**在这里
// 重算命中 —— 判据与点名必须是同一次判定，✗ 不许「门说红、点名说没事」。
func czrJudge(rep *eval.Report) czrVerdict {
	v := czrVerdict{
		Budget: rep.Budget, N: rep.N, Controls: rep.Controls,
		WinDen: rep.M5.MRRWinDenom, RecDen: rep.M5.MRRRecDenom,
		AnchorN: rep.M5.AnchorTotal, Anchor: rep.M5.AnchorLost,
		Win: rep.M5.MRRWin, Rec: rep.M5.MRRRec,
	}
	for _, row := range rep.Rows {
		for _, id := range row.WindowLostIDs {
			v.WinLost = append(v.WinLost, row.SampleID+"#"+id)
		}
		for _, id := range row.RecLostIDs {
			v.RecLost = append(v.RecLost, row.SampleID+"#"+id)
		}
		if row.AnchorsLost > 0 {
			v.AnchorLost = append(v.AnchorLost, fmt.Sprintf("%s（%d/%d 锚点丢）", row.SampleID, row.AnchorsLost, row.AnchorsTotal))
		}
	}
	sort.Strings(v.WinLost)
	sort.Strings(v.RecLost)
	sort.Strings(v.AnchorLost)
	// 分母为 0 ⇒ 「满格」是空话（0/0）⇒ 判不了，不许算过。
	v.WinOK = v.WinDen > 0 && v.Win == 1
	v.RecOK = v.RecDen > 0 && v.Rec == 1
	v.AnchorOK = v.AnchorN > 0 && v.Anchor == 0
	return v
}

// czrRunBatch —— 跑一遍（唯一入口：`eval.RunBatch` ⇒ `compress.Apply`）。
func czrRunBatch(budget int, work string) (*eval.Report, error) {
	return eval.RunBatch(context.Background(), eval.AllSamples(), eval.Config{
		Budget:  budget,
		Engine:  czrEngine,
		WorkDir: work,
		RunID:   fmt.Sprintf("gate136-%s-b%d", czrEngine, budget),
	})
}

// czrUsageText —— 用法（未知旗标 → rc=2）。
const czrUsageText = "用法：zerg gate check check-compress-ruler [--budget <N>] [--work <落点>] [--self-test]\n" +
	"  判据：eval.AllSamples()（50 条 · 五类）经 eval.RunBatch（Engine=free）跑一遍，" +
	"要求 MRR_win == 1.0000 且 MRR_rec == 1.0000 且 AnchorLost == 0\n" +
	"  退码：0 全过 · 1 有丢（逐条点名 needle / 样本）· 2 不给结论（跑批起不来 / 样本面缩水 / 用法错）\n" +
	"  --budget N   **只给负控用**：判据预算恒为 8192；压到 2048 后本门必须红并点名（门有牙的现场证明）\n" +
	"  --work DIR   unload/ 与 samples.json 的落点（缺省 = 仓外临时目录，跑完即清）\n"

// czrPrint —— 人读面（读数 + 逐条点名 + 结论），返回是否全过。
func czrPrint(stdout io.Writer, v czrVerdict) bool {
	fmt.Fprintln(stdout, "── 门136 压缩尺子硬闸（eval.AllSamples × free 引擎）（P2 收尾）──")
	fmt.Fprintf(stdout, "  样本 %d 条（对照 %d 条）· Budget %d · 引擎 %s\n", v.N, v.Controls, v.Budget, czrEngine)
	fmt.Fprintf(stdout, "  M5 MRR_win    = %.4f（%d/%d）判据 = 1.0000 ⇒ %s\n",
		v.Win, v.WinDen-v.lenWindowLost(), v.WinDen, czrMark(v.WinOK))
	fmt.Fprintf(stdout, "  M5 MRR_rec    = %.4f（%d/%d）判据 = 1.0000 ⇒ %s\n",
		v.Rec, v.RecDen-len(v.RecLost), v.RecDen, czrMark(v.RecOK))
	fmt.Fprintf(stdout, "  M5 AnchorLost = %d/%d 判据 = 0 ⇒ %s\n", v.Anchor, v.AnchorN, czrMark(v.AnchorOK))

	if len(v.WinLost) > 0 {
		fmt.Fprintf(stdout, "  ✗ 被挤出窗口的 needle %d 条（MRR_win 口径）：\n", len(v.WinLost))
		for _, id := range v.WinLost {
			fmt.Fprintf(stdout, "     · %s\n", id)
		}
	}
	if len(v.RecLost) > 0 {
		fmt.Fprintf(stdout, "  ✗ 窗口与盘上都取不回的 needle %d 条（MRR_rec 口径 ⇒ 真丢）：\n", len(v.RecLost))
		for _, id := range v.RecLost {
			fmt.Fprintf(stdout, "     · %s\n", id)
		}
	}
	if len(v.AnchorLost) > 0 {
		fmt.Fprintf(stdout, "  ✗ 丢锚点的样本 %d 条（AnchorLost 口径）：\n", len(v.AnchorLost))
		for _, s := range v.AnchorLost {
			fmt.Fprintf(stdout, "     · %s\n", s)
		}
	}
	if v.Pass() {
		fmt.Fprintf(stdout, "  判据结论：MRR_win/MRR_rec/AnchorLost 三条全过 ⇒ rc=0\n")
		return true
	}
	fmt.Fprintf(stdout, "  判据结论：**%d 条 needle 丢出窗口 · %d 条真丢 · %d 条样本丢锚点** ⇒ rc=1"+
		"（阻断：压缩引擎不许把必留信息挤出窗口 / 弄丢锚点）\n",
		len(v.WinLost), len(v.RecLost), len(v.AnchorLost))
	return false
}

// lenWindowLost —— 窗口丢的 needle 条数（打印用；口径 = 点名表长度，✗ 不再算一遍率）。
func (v czrVerdict) lenWindowLost() int { return len(v.WinLost) }

func czrMark(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// czrRun —— 打印 + 退码（三档）。
func czrRun(stdout, stderr io.Writer, budget int, work string) int {
	cleanup := func() {}
	if work == "" {
		d, err := os.MkdirTemp("", "zerg-gate136-")
		if err != nil {
			fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：仓外临时目录建不出来：%v\n", err)
			return 2
		}
		work, cleanup = d, func() { _ = os.RemoveAll(d) }
	}
	defer cleanup()
	if err := os.MkdirAll(filepath.Join(work, "unload"), 0o755); err != nil {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：落点不可写（%s）：%v\n", work, err)
		return 2
	}

	rep, err := czrRunBatch(budget, work)
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：跑批起不来：%v（✗ 不当作全过）\n", err)
		return 2
	}
	if rep.N < czrMinN || rep.M5.MRRWinDenom == 0 || rep.M5.MRRRecDenom == 0 || rep.M5.AnchorTotal == 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：样本面缩水或分母为 0（n=%d · win 分母 %d · rec 分母 %d · 锚点 %d；"+
			"下限 n>=%d）⇒ 这条读数判不了「满格」\n",
			rep.N, rep.M5.MRRWinDenom, rep.M5.MRRRecDenom, rep.M5.AnchorTotal, czrMinN)
		return 2
	}
	if czrPrint(stdout, czrJudge(rep)) {
		return 0
	}
	return 1
}

// czrSelfTest —— 本门自己的两态（成对负控）：判据档必须绿 · 压窗档必须红且点名。
//
// 为什么这门非要有自检：它的判据是一条**率**。率最容易出的一种坏是「恒绿」——分母变 0、
// 样本面被悄悄换掉、或者干脆没跑（脏目录跑不起来却当成功）。自检把两态钉在**同一次调用**里：
// 8192 绿 ∧ 2048 红 ∧ 红时点名非空 ⇒ 这条判据确实在跟着引擎状态动。
func czrSelfTest(stdout, stderr io.Writer) int {
	base, err := os.MkdirTemp("", "zerg-gate136-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 自检不给结论（rc=2）：临时目录建不出来：%v\n", err)
		return 2
	}
	defer os.RemoveAll(base)

	bad := 0
	check := func(name string, got, want any) {
		gs, ws := fmt.Sprint(got), fmt.Sprint(want)
		if gs == ws {
			fmt.Fprintf(stdout, "   ✓ %s（=%s）\n", name, gs)
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s（实际=%s 期望=%s）\n", name, gs, ws)
		bad++
	}

	// ① 判据档：8192 ⇒ 三条全过。
	hi, err := czrRunBatch(czrBudget, filepath.Join(base, "hi"))
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 自检不给结论（rc=2）：8192 档跑批起不来：%v\n", err)
		return 2
	}
	vh := czrJudge(hi)
	check("① 判据档 8192：MRR_win == 1.0000", vh.WinOK, true)
	check("① 判据档 8192：MRR_rec == 1.0000", vh.RecOK, true)
	check("① 判据档 8192：AnchorLost == 0", vh.AnchorOK, true)
	check("① 判据档 8192：三条全过 ⇒ 本门绿", vh.Pass(), true)

	// ② 负控档：2048 ⇒ 必须红，且**点名表非空**（红而点不出名 = 判据与点名两套口径，不许过）。
	lo, err := czrRunBatch(2048, filepath.Join(base, "lo"))
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 自检不给结论（rc=2）：2048 档跑批起不来：%v\n", err)
		return 2
	}
	vl := czrJudge(lo)
	check("② 负控档 2048：本门必须红（压窗后必留信息被挤出窗口）", vl.Pass(), false)
	check("② 负控档 2048：MRR_win < 1.0000", vl.WinOK, false)
	check("② 负控档 2048：点名表非空（丢的 needle 逐条可指）", len(vl.WinLost) > 0, true)
	check("② 负控档 2048：三条判据读数逐条打印（人读面能对回样本）", strings.Contains(czrRender(vl), "MRR_win"), true)

	if bad > 0 {
		fmt.Fprintf(stderr, "⛔ 自检不过（%d 条失败）⇒ 本门拒绝给结论\n", bad)
		return 2
	}
	fmt.Fprintln(stdout, "  自检结论：全过（两态成对：8192 绿 · 2048 红并点名）⇒ 本门有牙")
	return 0
}

// czrRender —— 把一次判读渲染成人读面（自检复用人读面的同一段渲染，✗ 不另写一份）。
func czrRender(v czrVerdict) string {
	var b strings.Builder
	czrPrint(&b, v)
	return b.String()
}

// checkCompressRulerGateMain —— 入口（同族旗标面：`--budget` · `--work` · `--self-test` · `-h/--help`）。
func checkCompressRulerGateMain(stdout, stderr io.Writer, argv []string) int {
	budget := czrBudget
	work := ""
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, czrUsageText)
			return 0
		case a == "--self-test":
			selfTest = true
		case a == "--budget":
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				fmt.Fprintln(stderr, "用法错：--budget 缺值")
				fmt.Fprint(stderr, czrUsageText)
				return 2
			}
			i++
			n, ok := czrParseBudget(argv[i])
			if !ok {
				fmt.Fprintf(stderr, "用法错：--budget 值不认（%q ⇒ 要正整数 token 预算）\n", argv[i])
				fmt.Fprint(stderr, czrUsageText)
				return 2
			}
			budget = n
		case strings.HasPrefix(a, "--budget="):
			n, ok := czrParseBudget(strings.TrimPrefix(a, "--budget="))
			if !ok {
				fmt.Fprintf(stderr, "用法错：--budget 值不认（%q ⇒ 要正整数 token 预算）\n", a)
				fmt.Fprint(stderr, czrUsageText)
				return 2
			}
			budget = n
		case a == "--work":
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				fmt.Fprintln(stderr, "用法错：--work 缺值")
				fmt.Fprint(stderr, czrUsageText)
				return 2
			}
			i++
			work = argv[i]
		case strings.HasPrefix(a, "--work="):
			work = strings.TrimPrefix(a, "--work=")
			if work == "" {
				fmt.Fprintln(stderr, "用法错：--work 缺值")
				fmt.Fprint(stderr, czrUsageText)
				return 2
			}
		default:
			fmt.Fprintf(stderr, "用法错：不认的旗标/位置参数 %q\n", a)
			fmt.Fprint(stderr, czrUsageText)
			return 2
		}
	}
	if selfTest {
		return czrSelfTest(stdout, stderr)
	}
	return czrRun(stdout, stderr, budget, work)
}

// czrParseBudget —— 正整数解析（✗ 不用 strconv.Atoi 吞掉 "+5"/"-3" 这类形态）。
func czrParseBudget(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1<<20 {
			return 0, false
		}
	}
	if n <= 0 {
		return 0, false
	}
	return n, true
}

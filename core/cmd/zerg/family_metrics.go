// family_metrics.go —— `zerg metrics`（**度量与排序面** · 一条只读命令）·
// 组1 序12（源件 `承接自-v2.5.11/承接-度量与排序面-20260921.md:40-42`）。
//
// 病灶（源件逐字）：影响面只回答「改这一处会牵动谁」—— 它管的是「**别改坏**」；
// 而「**该改哪**、往哪改、改完真的更好吗」它一个字都答不了。源件 §一 第 ① 环逐字：
//
//	「该改哪 —— 哪里慢 / 重复 / 腐化 / 易错：`jscpd` 读数 · 覆盖率 · 门禁四档读数 ·
//	 标定档案 ⇒ ⚠ **有料，但没成一条命令**」
//
// 本命令 = 把**四类读数归一成一个可比的排序**（读数 → 归一 → 排序 · **全程只读**）。
// 源件 §三 的形态建议逐字：「一条**只读**的「排序面」命令，把上面几类读数归一成一个可比的排序
// （哪里最值得改）」；本命令照此落，不加第二维（与 `impact` **配对用**、不合并成一条）。
//
// 形态（只读 · 零写）：
//
//	zerg metrics [--input <读数档>] [--json <字段>]
//
// 读数档（JSON · 默认落点 = `<状态目录>/metrics-readings.json` —— `statepath.File` 一处解析，
// 不硬编码私有路径）：
//
//	{
//	  "generated_at": "2026-09-24T04:00:00Z",            // 可省（只给人读）
//	  "blocks": {
//	    "dup":   {"dir": "le", "unit": "克隆处",   "source_cmd": "npx jscpd@4 core --format go --min-tokens 50",
//	              "readings": [{"target": "core/cmd/zerg", "value": 119, "budget": 40}]},
//	    "dead":  {"dir": "le", "unit": "未用符号", "source_cmd": "staticcheck -checks=U1000 ./...",
//	              "readings": [{"target": "core/cmd/zerg", "value": 65,  "budget": 20}]},
//	    "cover": {"dir": "ge", "unit": "%",        "source_cmd": "go test -cover ./...",
//	              "readings": [{"target": "core/cmd/zerg", "value": 12,  "budget": 60}]},
//	    "gates": {"dir": "le", "unit": "步",       "source_cmd": "zerg gate results --last",
//	              "readings": [{"target": "docs", "value": 4, "budget": 1}]}
//	  }
//	}
//
// 四类是**闭集**（`dup` / `dead` / `cover` / `gates`）—— 某类缺就照实记 0 条，**不猜、不补**。
// ★ 本命令**不产出**读数档：`jscpd` / `staticcheck` / 覆盖率 / 门禁这四条链各自是它们的事；
// 本命令只负责「读数 → 可比排序」这一段（**不替谁造数**，读不到就照实说读不到）。
//
// 归一（逐条可核 · 一个数的来源写在档里，不在本文件里）：
//
//	dir = "le"（越小越好）⇒ score = value / budget
//	dir = "ge"（越大越好）⇒ score = budget / value
//	档（grade）：score ≥ 2 ⇒ D（最该改）· 1 ≤ score < 2 ⇒ C · 0.5 ≤ score < 1 ⇒ B · < 0.5 ⇒ A
//	  ★ 这三条阈值是**本面自定的一条尺**（写在这里，不借「业界惯例」的名义 —— 块E §八③ 的
//	    阈值引法纪律句：引阈值必须写全依据链；这里给的就是依据链本身：一条读数与它自己的预算之比）。
//	★ `budget ≤ 0`、或 `dir` 不是 `le`/`ge` ⇒ 该条**不可比** ⇒ **不给分、不进排序**（照实点名，
//	  不许猜一个分把它塞进排序）。
//
// 排序：可比条目按 `score` 降序；并列按类（`dup` < `dead` < `cover` < `gates`）再按 `target` 升序。
// 不可比条目排在其后、`rank` 记 `-`（**不静默丢弃** —— 机器面看得见「有几条没进排序」）。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0  出了排序（可比 ≥ 1 条 —— 哪怕全是 A 档，也是「有结论」）
//	2  用法错 · **空输入**（读数档不在盘 / 空件 / 零读数）· 或**全部不可比**（给不出可比排序）⇒ 不给结论
//	8  读数档在、但**读不通**（JSON 坏 / `blocks` 不是对象 ⇒ 判不了 ⇒ 不许当绿）
//
// ★ 只读：`os.Stat` / `os.ReadFile` 两件，**不写不删不改**任何件（跑完仓内件 sha 不变 ——
//
//	负控在 `cli_metrics_test.go` 的 `TestMetricsIsReadOnly` 里）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// metricsInputDefaultName —— 读数档的默认件名（落 `<状态目录>`；与命令面其余状态件同一口径）。
const metricsInputDefaultName = "metrics-readings.json"

// metricsFields —— `--json` 的字段表（一条读数一行）。
// `source` = 该类读数在档里声明的 `source_cmd`（读数**出自哪条命令** —— 不写死在本文件里）。
var metricsFields = []string{"rank", "grade", "category", "target", "value", "budget", "unit", "score", "source"}

// metricsCategoryOrder —— 四类的**闭集**与并列时的决胜序（`dup` < `dead` < `cover` < `gates`）。
var metricsCategoryOrder = map[string]int{"dup": 0, "dead": 1, "cover": 2, "gates": 3}

// metricsCategoryLabel —— 四类的中文标签（只为人面；机器面一律用闭集键）。
var metricsCategoryLabel = map[string]string{"dup": "重复", "dead": "未用符号", "cover": "覆盖率", "gates": "门禁"}

// metricsCategoryList —— 闭集的**稳定遍历序**（与决胜序同一份，不另写一遍）。
var metricsCategoryList = []string{"dup", "dead", "cover", "gates"}

type metricsReading struct {
	Target string  `json:"target"`
	Value  float64 `json:"value"`
	Budget float64 `json:"budget"`
}

type metricsBlock struct {
	Dir       string           `json:"dir"`
	Unit      string           `json:"unit"`
	SourceCmd string           `json:"source_cmd"`
	Readings  []metricsReading `json:"readings"`
}

type metricsDoc struct {
	GeneratedAt string                  `json:"generated_at"`
	Blocks      map[string]metricsBlock `json:"blocks"`
}

// metricsRow —— 排序里的一条（可比与不可比**同一个结构**，靠 `comparable` 分开）。
type metricsRow struct {
	Category   string
	Target     string
	Unit       string
	Source     string
	Value      float64
	Budget     float64
	Score      float64
	Comparable bool
}

// metricsGrade 档位：score ≥ 2 ⇒ D · ≥ 1 ⇒ C · ≥ 0.5 ⇒ B · 其余 ⇒ A（本面自定的一条尺 · 见件头）。
func metricsGrade(score float64) string {
	switch {
	case score >= 2:
		return "D"
	case score >= 1:
		return "C"
	case score >= 0.5:
		return "B"
	default:
		return "A"
	}
}

// metricsNum 数字的**原样**写法（只为显示；不做单位换算、不四舍五入）。
func metricsNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func cmdMetrics(inv *invocation, stdout, stderr io.Writer) int {
	path := strings.TrimSpace(inv.flagVal("--input"))
	src := "声明面（`--input`）"
	if path == "" {
		path = statepath.File(metricsInputDefaultName)
		src = "默认落点（`<状态目录>`）"
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// **空输入 ⇒ 不给结论**（本条的负控判据逐字）：读数档不在盘 ⇒ 一个分都不给。
			inv.setErr("usage", "empty_input", "读数档不在盘 ⇒ 空输入")
			fmt.Fprintf(stderr, "%s: 空输入 —— 读数档不在盘：%s（%s）\n", progName, path, src)
			fmt.Fprintf(stderr, "下一步：让四条链之一把读数落成这个形状（`jscpd` 读数 · `staticcheck -checks=U1000 ./...` · 覆盖率 · `zerg gate results --last`），或 `--input <读数档>` 指一个\n")
			fmt.Fprintf(stderr, "★ 空输入 ⇒ **不给结论**（退码 2）：本命令**不替你造读数**、不猜一个分\n")
			return exitUsage
		}
		inv.setErr("blocked", "input_unreadable", "读数档读不到")
		fmt.Fprintf(stderr, "%s: 读数档读不到（%v）⇒ 判不了、不许当绿（退码 8）\n", progName, err)
		return exitBlocked
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		inv.setErr("usage", "empty_input", "读数档是空件 ⇒ 空输入")
		fmt.Fprintf(stderr, "%s: 空输入 —— 读数档是空件：%s（%s）⇒ 不给结论（退码 2）\n", progName, path, src)
		return exitUsage
	}

	var doc metricsDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		inv.setErr("blocked", "input_unreadable", "读数档 JSON 解析不过")
		fmt.Fprintf(stderr, "%s: 读数档读不通（JSON 解析不过：%v）⇒ 判不了、不许当绿（退码 8）\n", progName, err)
		return exitBlocked
	}
	if doc.Blocks == nil {
		inv.setErr("blocked", "input_unreadable", "读数档没有 blocks 对象")
		fmt.Fprintf(stderr, "%s: 读数档读不通（没有 `blocks` 对象）⇒ 判不了、不许当绿（退码 8）\n", progName)
		return exitBlocked
	}

	total := 0
	perCat := map[string][2]int{} // [读数条数, 可比条数]
	var comparable, uncomparable []metricsRow
	for _, cat := range metricsCategoryList {
		blk := doc.Blocks[cat]
		for _, r := range blk.Readings {
			total++
			row := metricsRow{
				Category: cat, Target: r.Target, Unit: blk.Unit, Source: blk.SourceCmd,
				Value: r.Value, Budget: r.Budget,
			}
			switch {
			case blk.Dir == "le" && r.Budget > 0:
				row.Score, row.Comparable = r.Value/r.Budget, true
			case blk.Dir == "ge" && r.Value > 0 && r.Budget > 0:
				row.Score, row.Comparable = r.Budget/r.Value, true
			}
			c := perCat[cat]
			c[0]++
			if row.Comparable {
				c[1]++
				comparable = append(comparable, row)
			} else {
				uncomparable = append(uncomparable, row)
			}
			perCat[cat] = c
		}
		// 闭集里出现「档里没有的类」⇒ 点名（照实，不算错、也不补 0 条之外的任何数）。
		if _, ok := doc.Blocks[cat]; !ok {
			fmt.Fprintf(stderr, "%s: 读数档里没有 `%s` 这一类（照实记 0 条 —— 本命令不替它补数）\n", progName, cat)
		}
	}
	// 闭集外的类名 ⇒ 点名（不静默吞；也不把它们塞进排序 —— 闭集就是闭集）。
	for k := range doc.Blocks {
		if _, ok := metricsCategoryOrder[k]; !ok {
			fmt.Fprintf(stderr, "%s: 读数档里有闭集外的类 %q（四类是闭集：%s）—— 照实不把它算进排序\n",
				progName, k, strings.Join(metricsCategoryList, " / "))
		}
	}

	fmt.Fprintf(stderr, "%s: 读数档 %s（%s）· 现读 %d 条读数（可比 %d · 不可比 %d）\n",
		progName, path, src, total, len(comparable), len(uncomparable))
	for _, cat := range metricsCategoryList {
		c := perCat[cat]
		fmt.Fprintf(stderr, "  %s(%s) %d/%d", metricsCategoryLabel[cat], cat, c[1], c[0])
	}
	fmt.Fprintln(stderr)

	if total == 0 {
		inv.setErr("usage", "empty_readings", "读数档零条读数 ⇒ 空输入")
		fmt.Fprintf(stderr, "%s: 空输入 —— 读数档里零条读数（四类合计 0）⇒ 给不出排序、不给结论（退码 2）\n", progName)
		return exitUsage
	}
	if len(comparable) == 0 {
		inv.setErr("usage", "no_comparable", "全部不可比 ⇒ 给不出可比排序")
		fmt.Fprintf(stderr, "%s: **全部不可比** —— %d 条读数没有一条能算出分（`budget` ≤ 0 或 `dir` 不是 le/ge）⇒ 不给结论（退码 2）\n",
			progName, total)
		for _, r := range uncomparable {
			fmt.Fprintf(stderr, "  不可比：%s/%s value=%s budget=%s unit=%s\n",
				r.Category, r.Target, metricsNum(r.Value), metricsNum(r.Budget), r.Unit)
		}
		return exitUsage
	}

	sort.SliceStable(comparable, func(i, j int) bool {
		if comparable[i].Score != comparable[j].Score {
			return comparable[i].Score > comparable[j].Score
		}
		if metricsCategoryOrder[comparable[i].Category] != metricsCategoryOrder[comparable[j].Category] {
			return metricsCategoryOrder[comparable[i].Category] < metricsCategoryOrder[comparable[j].Category]
		}
		return comparable[i].Target < comparable[j].Target
	})
	sort.SliceStable(uncomparable, func(i, j int) bool {
		if metricsCategoryOrder[uncomparable[i].Category] != metricsCategoryOrder[uncomparable[j].Category] {
			return metricsCategoryOrder[uncomparable[i].Category] < metricsCategoryOrder[uncomparable[j].Category]
		}
		return uncomparable[i].Target < uncomparable[j].Target
	})

	rows := make([]map[string]string, 0, len(comparable)+len(uncomparable))
	put := func(rank string, r metricsRow) {
		score := "-"
		grade := "-"
		if r.Comparable {
			score = fmt.Sprintf("%.2f", r.Score)
			grade = metricsGrade(r.Score)
		}
		rows = append(rows, map[string]string{
			"rank":     rank,
			"grade":    grade,
			"category": r.Category,
			"target":   r.Target,
			"value":    metricsNum(r.Value),
			"budget":   metricsNum(r.Budget),
			"unit":     r.Unit,
			"score":    score,
			"source":   r.Source,
			"reading":  metricsNum(r.Value) + "/" + metricsNum(r.Budget) + " " + r.Unit,
		})
	}
	for i, r := range comparable {
		put(strconv.Itoa(i+1), r)
	}
	for _, r := range uncomparable {
		put("-", r)
	}
	for _, r := range uncomparable {
		fmt.Fprintf(stderr, "  不可比（不进排序）：%s/%s value=%s budget=%s\n",
			r.Category, r.Target, metricsNum(r.Value), metricsNum(r.Budget))
	}
	fmt.Fprintf(stderr, "%s: 档位 —— D 最该改（score ≥ 2）· C（≥ 1）· B（≥ 0.5）· A（其余）；排序 = score 降序\n", progName)

	return listCmd(inv, stdout, stderr,
		[]string{"rank", "grade", "category", "target", "reading", "score"}, rows)
}

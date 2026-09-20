// family_reap.go —— 回收与残留的**干跑单**（§十 · §十五.2/§十五.3 · `RC1`–`RC14` ·
// §十二 `P-104`–`P-113` · 开工单 T-54）。
//
// 本件只落**干跑**那一半（`RC1` 的默认档；真回收属写面 ⇒ 未开放，真跑一律拒执 ⇒ 2），
// 但落得**能红**：
//
//	① 干跑单带 **`plan_id`**（`RC12`/`P-112`：候选集按「路径 + 体积 + mtime」排序后的 sha256 前 12 位）
//	   —— 它是「干跑不是打印稿」的唯一保证；逐件带**证据**（判据①）。
//	② **`RC11` 红线**（入库件永不进候选）：候选里出现一件 `git ls-files` 命中的件 ⇒ **整单拒**
//	   （退码 2，detail=tracked_in_candidates）—— **不是**「跳过那一件」（判据②）。
//	③ **回滚件到期前一律档 ③**（`P-105`）：`bin/_history/` 前缀下的件恒落档 ③（判据③）。
//	④ 三档回收权 × 五类差集**逐条有判词**：判词真源 = `reap.json` 的 `differences[].judge`，
//	   本件只**引用**、不重写（判据④）。
//	⑤ `RC14`：「跳过」必须配一行**计数与体积**（只跳过不计数 ⇒ 报 REPORT）。
//
// ★ 本版边界（照实说）：三档里**档 ①**（五道闸全绿：声明认领 / 引用计数 / 四证物 / 白名单 /
//
//	写审计）要**声明树 + 审计写入面**才成立 ⇒ 本版只落**分档与判词**，不落「真动手」那一步。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// reapItem —— 干跑单里的一件（逐件带证据）。
type reapItem struct {
	Rel      string
	Bytes    int64
	ModTime  time.Time
	Tier     string // "2" / "3" / "skip"
	Class    string // 清册类 id + 名（`#1 换件残片` 一类）
	Diff     string // 差集 id（`D-*`；本版文件类落 `D-A`，读不到落 `D-E`）
	Judge    string
	Evidence map[string]string
}

// reapPlan —— 一份干跑单。
type reapPlan struct {
	PlanID     string
	Items      []reapItem
	Tracked    []string // `RC11` 命中（非空 ⇒ 整单拒）
	Skipped    []reapItem
	AgeDays    int
	Root       string
	TotalBytes int64
}

// classOf —— **分档的唯一判定口**：按**清册类**定档（§十五.3），不是按「差集」定档 ——
// 两个轴（声明↔现值的差集 / 这件东西属于哪一类）不许混成一个数。命中多类 ⇒ 取**更严**的那一档
// （档 ③ > 档 ② > 档 ①），因为从严的错是「少收一件」，从宽的错是「收错一件」。
func classOf(rel string, spec *contract.ReapSpec, readable bool) (tier, class, judge string) {
	if !readable {
		return "skip", "（读不到）", "`D-E` 读不到 ⇒ **不是「没坏」**（`SKIP` + 退码 8 · `RC9`）"
	}
	tier, class, judge = "2", "（清册外）", ""
	for _, c := range spec.ObjectClasses {
		hit := false
		for _, g := range c.Globs {
			if ok, _ := filepath.Match(filepath.FromSlash(g), filepath.FromSlash(rel)); ok {
				hit = true
				break
			}
			// glob 带目录（`ui/target/debug`）时 Match 对整串也成立；再加前缀判一层。
			if strings.HasPrefix(rel, strings.TrimSuffix(filepath.ToSlash(g), "/")+"/") {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if judge == "" || c.Tier == "3" {
			tier, class, judge = c.Tier, c.ID+" "+c.Class, c.Judge
		}
	}
	for _, p := range spec.Tier3Prefixes { // 硬前缀（安全网 / 冻结归档 / 入库前缀）一律档 ③
		if strings.HasPrefix(rel, p) && tier != "3" {
			tier = "3"
			if judge == "" {
				judge = spec.LegendNote
			}
		}
	}
	if judge == "" {
		// 清册外（`RC3`：清册外**默认拒绝**）⇒ 档 ③，判词点明「默认拒绝」。
		tier, class, judge = "3", "（清册外）", "`RC3` 对象清册是**闭集**、清册外默认拒绝 ⇒ 档 ③ 永不自动"
	}
	return tier, class, judge
}

// diffOf —— 差集归类（本版只有「现值有 · 声明无」与「读不到」两类 —— 不臆造第六个值 · `RC4`）。
func diffOf(tier string) string {
	if tier == "skip" {
		return "D-E"
	}
	return "D-A"
}

// buildReapPlan —— 造干跑单（纯函数：给根 + 真源 + 龄阈值 + 「算不算入库件」的判定口）。
//
// `isTracked(rel)` 由调用方注入（生产侧走 `git ls-files`；测试侧喂合成集合）——
// 这样 `RC11` 那条红线**能负控**（喂一件 tracked 进来必须整单拒）。
func buildReapPlan(root string, spec *contract.ReapSpec, minAgeDays int, isTracked func(rel string) bool, now time.Time) reapPlan {
	p := reapPlan{Root: root, AgeDays: minAgeDays}
	seen := map[string]bool{}
	globs := []string{}
	for _, c := range spec.ObjectClasses {
		globs = append(globs, c.Globs...)
	}
	for _, g := range globs {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(g)))
		sort.Strings(matches)
		for _, m := range matches {
			rel, err := filepath.Rel(root, m)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if seen[rel] {
				continue
			}
			seen[rel] = true
			it := reapItem{Rel: rel, Diff: "", Evidence: map[string]string{}}
			st, statErr := os.Stat(m)
			readable := statErr == nil
			if readable {
				it.Bytes = st.Size()
				it.ModTime = st.ModTime()
			}
			it.Tier, it.Class, it.Judge = classOf(rel, spec, readable)
			it.Diff = diffOf(it.Tier)
			tracked := isTracked(rel)
			it.Evidence["tracked"] = boolAbbr(tracked)
			it.Evidence["age_days"] = ageDays(now, it.ModTime, readable)
			it.Evidence["bytes"] = strconv.FormatInt(it.Bytes, 10)
			it.Evidence["tier"] = it.Tier
			if tracked {
				// `RC11`：**不是**「跳过那一件」—— 跳过去说明判据自己看走了眼 ⇒ 整单拒。
				p.Tracked = append(p.Tracked, rel)
				continue
			}
			// 龄阈值（`RC6`：显式参数，不许魔数）：没到期的**不进候选**，但按 `RC14` 单列计数与体积。
			if readable && ageInDays(now, it.ModTime) < minAgeDays {
				p.Skipped = append(p.Skipped, it)
				continue
			}
			if it.Tier == "skip" {
				p.Skipped = append(p.Skipped, it)
				continue
			}
			p.Items = append(p.Items, it)
			p.TotalBytes += it.Bytes
		}
	}
	sort.Slice(p.Items, func(i, j int) bool { return p.Items[i].Rel < p.Items[j].Rel })
	sort.Slice(p.Skipped, func(i, j int) bool { return p.Skipped[i].Rel < p.Skipped[j].Rel })
	p.PlanID = reapPlanIDOf(p)
	return p
}

// reapPlanIDOf —— `RC12`/`P-112`：候选集指纹（路径 + 体积 + mtime 排序后 sha256 前 12 位）。
// ★ 名字与 `family_fusion.go` 的 `planIDOf`（意图件的那种）**故意不同** —— 两件事不是一件事。
func reapPlanIDOf(p reapPlan) string {
	lines := make([]string, 0, len(p.Items)+len(p.Skipped))
	for _, it := range p.Items {
		lines = append(lines, fmt.Sprintf("c|%s|%d|%d", it.Rel, it.Bytes, it.ModTime.Unix()))
	}
	for _, it := range p.Skipped {
		lines = append(lines, fmt.Sprintf("s|%s|%d|%d", it.Rel, it.Bytes, it.ModTime.Unix()))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "PLAN-" + hex.EncodeToString(sum[:])[:12]
}

func ageInDays(now, t time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(now.Sub(t).Hours() / 24)
}

func ageDays(now, t time.Time, readable bool) string {
	if !readable {
		return "?"
	}
	return strconv.Itoa(ageInDays(now, t))
}

func boolAbbr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// gitTracked —— `git ls-files --error-unmatch <rel>` 命中即「入库件」（`RC11` 红线的那一类）。
// 不在 git 仓 / 取不到 ⇒ 报 false（**宁可少收**：这一格由每件证据里的 `tracked=` 写明）。
func gitTracked(root, rel string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", rel)
	cmd.Dir = root
	return cmd.Run() == nil
}

// gitTrackedInRepo —— 生产侧的「算不算入库件」判定口：`git ls-files --error-unmatch <rel>`。
// 命中 ⇒ tracked（`RC11`）。**只在仓内可用**；取不到（不是 git 仓）⇒ 一律按「未入库」报
// （**这是保守的反面**：宁可少收，也不许把入库件当候选 —— 但这一格必须能被看见，
// 故由 `evidence["tracked"]` 逐件写明）。
func gitTrackedInRepo(root string) func(rel string) bool {
	return func(rel string) bool {
		return gitTracked(root, rel)
	}
}

// cmdAgentReap —— `zerg agent reap`（本版只有干跑一档；真跑沿用同一套防呆形状 ⇒ 拒执 2）。
func cmdAgentReap(inv *invocation, stdout, stderr io.Writer) int {
	if !inv.dryRun {
		// 真跑：**同一套防呆形状**（缺确认 / 确认值不匹配 / 本版未开放都在这条路上）。
		return cmdGuarded(inv, stdout, stderr)
	}
	spec, err := contract.Reap()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 回收真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked
	}
	// `RC6`：年龄阈值**显式**、不许魔数 ⇒ 不给就拒（默认值就是一个魔数）。
	raw := strings.TrimSpace(inv.flagVal("--min-age-days"))
	if raw == "" {
		inv.setErr("usage", "age_threshold_required", "缺 --min-age-days")
		fmt.Fprintf(stderr, "%s: 缺 `--min-age-days <n>` —— **年龄阈值是显式参数、不许写魔数**（`RC6`）\n", progName)
		fmt.Fprintf(stderr, "%s\n", spec.AgeThresholdRule)
		return exitUsage
	}
	minAge, err := strconv.Atoi(raw)
	if err != nil || minAge < 0 {
		inv.setErr("usage", "bad_age_threshold", "年龄阈值不是非负整数")
		fmt.Fprintf(stderr, "%s: `--min-age-days` 只认非负整数（给了 %q）\n", progName, raw)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "no_repo_root", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（读不到不当没有 · `RC9`）\n", progName)
		return exitBlocked
	}
	plan := buildReapPlan(root, spec, minAge, gitTrackedInRepo(root), time.Now())

	// ── `RC11` 红线：整单拒（不是「跳过那一件」）───────────────────────────────
	if len(plan.Tracked) > 0 {
		inv.setErr("usage", "tracked_in_candidates", "候选里出现入库件")
		fmt.Fprintf(stderr, "%s: 候选里出现 **%d 件入库件（git-tracked）** ⇒ **整单拒**（退码 2）\n",
			progName, len(plan.Tracked))
		for _, t := range plan.Tracked {
			fmt.Fprintf(stderr, "  · %s\n", t)
		}
		fmt.Fprintf(stderr, "红线（`RC11`）：%s\n", spec.RedLine.Rule)
		fmt.Fprintf(stderr, "口径：%s\n", spec.RedLine.Enforcement)
		return exitUsage
	}

	// ── 出干跑单（`RC1` 逐项一行 + 合计；`RC14` 跳过也配计数与体积）────────────
	tierBytes := map[string]int64{}
	tierCount := map[string]int{}
	for _, it := range plan.Items {
		tierBytes[it.Tier] += it.Bytes
		tierCount[it.Tier]++
	}
	var skipBytes int64
	for _, it := range plan.Skipped {
		skipBytes += it.Bytes
	}
	fmt.Fprintf(stdout, "干跑单 %s（`plan_id` = 候选集指纹 · RC12/P-112）\n", plan.PlanID)
	fmt.Fprintf(stdout, "  仓根     : %s\n", root)
	fmt.Fprintf(stdout, "  龄阈值   : %d 天（显式参数 · RC6：不许魔数）\n", minAge)
	fmt.Fprintf(stdout, "  档 ② 点名可动 : %d 件 · %s（要 `--only <id> --yes`，本版未开放）\n", tierCount["2"], humanBytes(tierBytes["2"]))
	fmt.Fprintf(stdout, "  档 ③ 永不自动 : %d 件 · %s（回滚件到期前 · 冻结归档 · 清册外）\n", tierCount["3"], humanBytes(tierBytes["3"]))
	fmt.Fprintf(stdout, "  跳过         : %d 件 · %s（`RC14`：跳过也配计数与体积）\n", len(plan.Skipped), humanBytes(skipBytes))
	fmt.Fprintf(stdout, "  合计         : %d 件 · %s\n", len(plan.Items), humanBytes(plan.TotalBytes))
	for _, d := range spec.Differences {
		fmt.Fprintf(stdout, "  差集 %-4s %-8s %-16s ⇒ %s（档 %s）\n", d.ID, d.Name, d.What, d.Judge, d.Tier)
	}
	for _, it := range plan.Items {
		keys := []string{}
		for k := range it.Evidence {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ev := []string{}
		for _, k := range keys {
			ev = append(ev, k+"="+it.Evidence[k])
		}
		fmt.Fprintf(stdout, "    档%s %-34s %-52s %10s  %s\n          判词：%s\n",
			it.Tier, it.Class, it.Rel, humanBytes(it.Bytes), strings.Join(ev, " "), it.Judge)
	}
	fmt.Fprintf(stderr, "★ 本单是**干跑**：一个字节都没动、没删任何件（`RC1` 默认档）。\n")
	fmt.Fprintf(stderr, "★ 本版只剩**分档与判词**：档 ①（五道闸全绿）要声明树 + 审计写入面 ⇒ %s\n", spec.Boundary)
	if len(plan.Skipped) > 0 {
		// `RC14`：只跳过不计数 ⇒ 报 REPORT（本件**计数了**，故这里是说明而不是告警）。
		return exitBlocked
	}
	return exitOK
}

// humanBytes —— 人面用的体积（只做展示，判据一律用字节数）。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	f := float64(n)
	i := -1
	for f >= unit && i < len(units)-1 {
		f /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

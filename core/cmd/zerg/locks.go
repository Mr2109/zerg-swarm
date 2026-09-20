// locks.go —— 并发与锁（§九 M5 · 调研-M5 §三 条文 1–12）。
//
// 五条已定条文（本文件是它们的自描述面）：
//
//	`C1` **三级粒度**：机（一台机同时只跑一个写动作）· 卡（显存/槽位）· 卵（一枚卵一个生命周期）。
//	`C2` **真源在持锁者那一端**（子端），命令面**不许自建本地锁** —— 薄壳没有第二份真相。
//	`C3` **租约化**：锁随持有者生命周期**自动失效**（不永久锁、不做人工解锁）。
//	`C4` 判据顺序「**先锁 → 再判闸门 → 再收卵**」；槽位**可版本化**（`slot` 块带版本号 / epoch）。
//	`C5` **三类时钟各自命名、数值不臆造**（`lock_wait_s` / `queue_wait_s` / `idle_unload_s`）。
//
// 冲突语义（§十二 `P-013` ② · `P-019`）：**冲突输家拿 `14`，不是 `507`**；默认冲突**即返码**，
// 只有显式 `--wait` 才排队（排队与不排队是两档，不许含糊）。
package main

import (
	"fmt"
	"strings"
)

// 三级锁粒度（C1 · 措辞照 调研-M5 行 163–174）。
var lockGrains = []string{
	"机（machine）：一台机同一时刻只跑一个写动作（装载/卸载/换件）",
	"卡（slot）  ：显存/槽位是最稀缺的一种 —— 单槽机是串行的",
	"卵（egg）   ：一枚卵一个生命周期（孵化 → 在孵 → 回收）",
}

// 三类时钟（C5）：**只定名**；取值必须来自标定，本文件一个数字都不写。
var lockClocks = []struct {
	Name    string
	Meaning string
}{
	{"lock_wait_s", "等锁的时间（显式 `--wait` 才产生）"},
	{"queue_wait_s", "排队的时间（进队列之后、拿到槽位之前）"},
	{"idle_unload_s", "空闲多久可以被收卵（**标定值**，不是拍脑袋）"},
}

// helpLocks —— `zerg help locks`（§九 M5 的自描述面）。
func helpLocks() string {
	var b strings.Builder
	b.WriteString("并发与锁（§九 M5 · 调研-M5 §三）\n\n")
	b.WriteString("三级粒度（`C1`）：\n")
	for _, g := range lockGrains {
		b.WriteString("  · " + g + "\n")
	}
	b.WriteString("\n五条硬纪律：\n")
	b.WriteString("  · `C2` **真源在持锁者那一端（子端）** —— 命令面**不许自建本地锁**（薄壳没有第二份真相）。\n")
	b.WriteString("  · `C3` **租约化**：锁随持有者生命周期**自动失效**；没有「人工解锁」这个动作。\n")
	b.WriteString("  · `C4` 判据顺序 **先锁 → 再判闸门 → 再收卵**；`slot` 块带**版本号 / epoch**\n")
	b.WriteString("        （§十二 `P-022` 定案：加了它客户端才**机器可判**「我看到的槽位是不是变了」）。\n")
	b.WriteString("  · `C5` **三类时钟各自命名、数值不臆造**（见下）——「显式参数、不许魔数」（§九 M19 `RC6` 同款）。\n")
	b.WriteString("  · 冲突与资源不足**必须分开退码**：被占走 `14`、资源不足走 `10`（不许都塞进 `507`）。\n\n")
	b.WriteString("三类时钟（**只有名字是本契约的**；取值一律来自标定）：\n")
	w := 0
	for _, c := range lockClocks {
		if len(c.Name) > w {
			w = len(c.Name)
		}
	}
	for _, c := range lockClocks {
		fmt.Fprintf(&b, "  %s  %s\n", pad(c.Name, w), c.Meaning)
	}
	b.WriteString("\n★ 数值口径（防误读）：本契约里**一个时长数字都没有** —— 取值必须**标定后**填\n")
	b.WriteString("（`scripts/calib/` 三条脚本）；标定没做之前，契约里出现任何时长都算**魔数**（不许进契约）。\n\n")
	b.WriteString("冲突档（§十二 `P-013` ② · `P-019`）：\n")
	b.WriteString("  · 输家退 **`14`**（`kind=conflict`）—— **不许**再用 `507` 表达「被占」。\n")
	b.WriteString("  · 默认**冲突即返码**；只有显式 `--wait` 才**排队**（排队与否是两档，不许含糊）。\n")
	b.WriteString("  · 合并只对「同键同终态」成立，且合并**必须可区分**于「我亲手装的」（复用即明说复用）。\n\n")
	b.WriteString("隔离（§十二 `P-124` · 调研-S2）：**一枚卵一棵工作树 —— 强制**（外部任务也进同一形状）；\n")
	b.WriteString("「同一分支不可两处检出」当**免费保证**用，不当约束去背。\n")
	return b.String()
}

// slotBlockLines —— 计划件里的 `slot` 块（`C4` / `P-022`：带版本号与 epoch）。
func slotBlockLines(machine string) []string {
	return []string{
		"  slot     : 目标 " + orDash(machine) + "（三级粒度：机 / 卡 / 卵 —— 真源在**子端**）",
		"  slot.version  : 0（**本版未取** —— 写面未开，取不到就该留空/报不给结论，不许编一个）",
		"  slot.epoch    : 0（同上；`C4` 要的是「客户端能机器判定槽位变了没有」）",
		"  冲突档   : 输家退 `14`（`kind=conflict`）—— **不是** `507`；显式 `--wait` 才排队",
	}
}

// lockLine 给判词用的一行（在计划件与错误面里回显同一句，避免两处写法漂）。
func lockLine() string {
	return "锁：真源在子端 · 租约化自动失效 · 三级粒度（机/卡/卵）· 冲突退 14（不是 507）"
}

var _ = strings.TrimSpace

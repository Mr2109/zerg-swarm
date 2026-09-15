// egg_gc.go —— 启动 GC：**任何启动先把遗留卵清干净**（设计 §6.5，Mr2109 已拍）。
//
// 为什么必须做（2026-09-15 第一枚卵真机实测 · 报告 §12 缺陷 10 / §9 的 E1）：
// 卵落 llm.slice 独立单元、刻意不被子端的 KillMode 带走（归属设计，E1 实证「KillMode 独立性成立」）
// ⇒ 子端一重启，**卵还活着**（引擎仍在服务、GTT 仍被占着），而新实例的账本是空的
// （/eggs=[]、/status=idle）⇒ 孤儿卵占着 GTT 且没人认领：既不会被空窗收走，也不会被任何请求复用。
// 设计 §6.5 的处置是唯一自洽的一种：**启动就把遗留卵清干净**（卵不活过子端重启）。
//
// 范围铁律（写死，越界即事故）：
//   - **只清本子端名下的卵单元**：单元名必须匹配 `zerg-<name>.service`，且必须出现在
//     `systemctl --user list-units --slice=llm.slice` 的清单里——孵化器是唯一往 llm.slice 里
//     写 zerg-* 单元的人（hatch.BuildSystemdRunArgv 硬编码 --slice=llm.slice）；
//   - **绝不碰用户手工服务**：X3 的 :9000（裸进程）与 :9001（screen 会话）根本不是 systemd 单元，
//     更不在 llm.slice 里；清单里出现的任何非 `zerg-*.service` 名字一律跳过（下面的解析与过滤
//     两层都做，见 parseEggUnitNames / gcLeftoverEggs）；
//   - 清不掉必须留痕（§6.9：命令返回成功 ≠ 真清干净了）——停完再复核一次 ActiveState，还在跑就
//     进 stuck 名单并打日志，绝不静默当成功。
package backend

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
)

// eggUnitNameRe 孵化器造得出的卵单元名（hatch.UnitName 的产物：`zerg-` + 收敛后的小写名 + `.service`）。
// 过滤用**白名单形态**而不是黑名单：任何不像卵单元的名字（含用户自己的 x3-agent.service /
// k2.service 之类）一律不动作。
var eggUnitNameRe = regexp.MustCompile(`^zerg-[a-z0-9][a-z0-9-]*\.service$`)

// parseEggUnitNames 从 `systemctl --user list-units --all --no-legend --plain 'zerg-*'`
// 的输出里取出单元名（纯函数，便于单测）。
//
// ⚠ 为什么**不用** `--slice=<切片>` 过滤（2026-09-15 X3 真机实测）：systemd 259 的
// `list-units` 不认识 `--slice=`（`systemctl: 未识别的选项 "--slice=llm.slice"`）⇒ 那条命令在
// 真机上直接失败、启动 GC 会一次都不生效。改为：先按名字列 `zerg-*`，再对每个候选**逐个核
// 它的归属**（Slice / ControlGroup 是不是 llm.slice，见 gcLeftoverEggs）——这比过滤参数更强：
// 「谁是我管的」由单元自己的归属证明，而不是由一条命令行选项保证。
//
// 输出形态（LOAD ACTIVE SUB DESCRIPTION 四列，无表头）：
//
//	zerg-qwen38-27b-egg.service loaded active running 卵 qwen38-27b-egg
//	● zerg-dead-egg.service        loaded failed failed   卵 dead-egg
//
// 取第一列，去掉 systemd 的 `●` 标记，再做白名单过滤（只认 `zerg-*.service`）。
func parseEggUnitNames(out string) []string {
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// systemd 对 failed 单元行首加标记（`● zerg-x.service …`）——先剥掉，
		// 否则第一列会变成 "●"、真单元名被当成第二列丢掉（failed 的遗留卵会漏清）。
		line = strings.TrimSpace(strings.TrimPrefix(line, "●"))
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		if !eggUnitNameRe.MatchString(name) {
			continue // 不是本子端名下的卵单元 ⇒ 绝不动作
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// benignStopErr 停单元时的良性错误（单元本来就不存在 / 没加载）——与收卵同一口径的**幂等成功**
// （hatch 包 collectOutcome 同一判据，此处不引它以保持「收卵在 hatch 层、清场在 backend 层」的分工）。
func benignStopErr(msg string) bool {
	low := strings.ToLower(msg)
	for _, b := range []string{"not found", "not loaded", "no such unit", "could not be found"} {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

// gcLeftoverEggs 清一道 llm.slice 里的遗留卵：逐个核归属 → stop → reset-failed → 复核。
//
// 归属判据（**逐个单元核**，不是靠一条命令行过滤参数）：
//   - 名字必须匹配 `zerg-*.service`（孵化器造得出的形态）；**且**
//   - 该单元自己的 `Slice`（或 cgroup 链）必须是 llm.slice —— 读不到归属就**不动它**
//     （fail-closed：宁可留一枚孤儿卵要人接手，也不误停一枚不是本子端孵出来的单元）。
//
// 返回 (清掉的, 清不掉的, err)：
//   - err 非 nil ⇒ 连清单都读不到（非 Linux / 没有 systemd 用户实例）：**什么都没做**，
//     调用方如实留痕（不许静默）；
//   - stuck 非空 ⇒ 停不掉 / 复核仍在跑 / 归属读不到（必须在日志里留痕，不许当清干净了）。
func gcLeftoverEggs(run unitCmdRunnerFunc) (stopped []string, stuck []string, err error) {
	if run == nil {
		run = unitCmdRunner
	}
	// 注意：**不**用 `--slice=`（systemd 259 的 list-units 不认识它，真机上会让整条命令失败）。
	out, listErr := run(10*time.Second, "systemctl", "--user", "list-units",
		"--all", "--no-legend", "--plain", "zerg-*")
	names := parseEggUnitNames(out)
	if listErr != nil && len(names) == 0 {
		return nil, nil, fmt.Errorf("列 zerg-* 卵单元失败：%v：%s", listErr, strings.TrimSpace(out))
	}
	for _, unit := range names {
		// ① 逐个核归属：不是我管的切片 ⇒ 跳过（绝不动作），并留痕说明为什么跳。
		st, probeErr := probeUnitStateWith(run, unit)
		if probeErr != nil {
			stuck = append(stuck, fmt.Sprintf("%s（读不到单元归属，不敢动：%v）", unit, probeErr))
			continue
		}
		if !st.inSlice(hatch.SliceName) {
			log.Printf("[backend] 启动 GC 跳过 %s：不归 %s 管辖（Slice=%q ControlGroup=%q）——"+
				"只管本子端孵出来的卵，不碰别的单元", unit, hatch.SliceName, st.Slice, st.ControlGroup)
			continue
		}
		// ② 停：幂等（本来就不存在 = 成功）。
		if stopOut, stopErr := run(20*time.Second, "systemctl", "--user", "stop", unit); stopErr != nil && !benignStopErr(stopOut+" "+stopErr.Error()) {
			stuck = append(stuck, fmt.Sprintf("%s（stop 失败：%v：%s）", unit, stopErr, strings.TrimSpace(stopOut)))
			continue
		}
		// ③ reset-failed：失败态的单元不 reset 会一直挂在 systemd 里（占名字、看着像还在）。
		if rsOut, rsErr := run(10*time.Second, "systemctl", "--user", "reset-failed", unit); rsErr != nil && !benignStopErr(rsOut+" "+rsErr.Error()) {
			log.Printf("[backend] ⚠ 启动 GC：reset-failed %s 未成功：%v：%s", unit, rsErr, strings.TrimSpace(rsOut))
		}
		// ④ 复核（§6.9：命令成功 ≠ 清干净了）——只依据实读的单元状态，不依据命令退出码。
		after, vErr := probeUnitStateWith(run, unit)
		switch {
		case vErr != nil:
			// 停命令成功但复核读不到：**不许**当「已确认清干净」，如实留痕（仍计入已停清单）。
			log.Printf("[backend] ⚠ 启动 GC：%s 的 stop 返回成功，但复核读不到单元状态（无法确认已清干净）：%v",
				unit, vErr)
		default:
			if dead, why := after.dead(); !dead {
				stuck = append(stuck, fmt.Sprintf("%s（停完仍在跑：%s）", unit, why))
				continue
			}
		}
		stopped = append(stopped, unit)
	}
	return stopped, stuck, nil
}

// startupEggGC 启动 GC 的执行体（注入点：单测替换它，不碰真 systemd）。
var startupEggGC = func() ([]string, []string, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, nil, fmt.Errorf("本机没有 systemctl（非 Linux / 无 systemd 用户实例）：%v", err)
	}
	return gcLeftoverEggs(nil)
}

// gcLeftoverEggsAtStartup 子端启动时的遗留卵清理（§6.5「任何启动先把遗留卵清干净」）。
//
// 只在**孵化开关开**的子端上做：开关关 ⇒ 本子端不孵任何卵（卵之外无引擎），没有「本子端名下的卵」
// 要认领；此时整段跳过，启动路径与孵化器落地前逐字一致（不新增动作、不打日志）。
// 开关开而清不掉 ⇒ 必须留痕（孤儿卵占着 GTT，这是要人接手的状态，不许静默）。
func gcLeftoverEggsAtStartup() {
	if !hatchEnabled() {
		return
	}
	stopped, stuck, err := startupEggGC()
	if err != nil {
		log.Printf("[backend] ⚠ 启动 GC 未执行（遗留卵可能仍占着 GTT，需人工确认）: %v", err)
		return
	}
	if len(stopped) == 0 && len(stuck) == 0 {
		log.Printf("[backend] 启动 GC：%s 里没有遗留卵单元", hatch.SliceName)
		return
	}
	if len(stopped) > 0 {
		log.Printf("[backend] 启动 GC：已清掉遗留卵 %d 枚: %v", len(stopped), stopped)
	}
	if len(stuck) > 0 {
		log.Printf("[backend] ⚠ 启动 GC：清不掉 %d 枚（需人工接手）: %v", len(stuck), stuck)
	}
}

// guardlog.go — 守卫拦截事件落点（control 层 · 唯一真源）。
//
// 出处（不新造语义）：
//
//	设计稿 `Zerg-内部文档/项目文档/v2.5.13/设计-缺口账与自进化-v2.0-20260928.md` §11.3
//	  「**三类入口**」之一「**守卫拦**」（候选池 = 独立文件 · 不进主账）
//	+ 缺口 `GAP-20260928-174`（「守卫拦截落点件与写入层」已拍：**落 `core/internal/control`**；
//	  回收面走 `zerg gap idea sweep --source guard` —— **拦截点不内联写候选池**）。
//
// 口径三条（缺一即错）：
//
//	① **落点 = 控制层**：谁做拦截判定谁记录 —— 即 `Gate.Check`（守卫的唯一判定点）在判定为
//	   **拦截**（`block` / `require_approval`）时追加一条事件。放行（`allow`）**不记**：
//	   记的是「被拦」这件事，不是每次工具调用。CLI 侧（`gap idea sweep`）只**读**本件。
//	② **事件件独立 · 不进主账**：`<状态目录>/zerg-cli-gap-guard-events.jsonl`（与候选池
//	   `zerg-cli-gap-candidates.jsonl` 同前缀）—— 与真源 `zerg-cli-gaps.jsonl` **完全分离**，
//	   本面任何路径都不写真源。
//	③ **best-effort 落盘**：写事件**绝不改变**控制层的判定与任何退码（写不进去只丢一条事件，
//	   拦/放行的语义一个字节不动）—— 照同族先例「门判红 ⇒ 候选池」钩子的 best-effort 纪律
//	   （`scripts/gates/precommit-gates.sh` 的 `gap_intake_red`：恒返回 0、绝不改任何步的 rc）。
//
// 去重口径：池内去重靠 `fp`（同 `source` 同 `fp` ⇒ 池内最多一条 · 设计稿 §3.A3）——
// 本件的 `fp` 与候选池那一格**同一取法**（`sha256("guard\x00<raw>")[:12]`），
// 让「同一条守卫拦重复出现」在回收时天然幂等（回收面 = `family_gap.go:cmdGapIdeaSweep`）。
package control

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// GuardEventFile —— 守卫拦截事件件名（与候选池同前缀 · 独立文件 · 不进主账）。
const GuardEventFile = "zerg-cli-gap-guard-events.jsonl"

// guardSymptomPrefix —— 守卫腿候选症状的固定前缀（回收面与采集面**同一形态**的唯一真源）。
const guardSymptomPrefix = "守卫拦截：工具 "

// GuardEvent —— 一条守卫拦截事件（一行 JSON）。
//
// 字段口径：`ts` 拦截时刻（RFC3339Nano）· `tool` 被拦的工具名 · `agent` 发起方（可空）·
// `action` 拦截档（`block` / `require_approval` —— `allow` 永不落）· `rule` 命中的规则名
// （落到默认档那一支为空）· `message` 控制层的判定说明（人面用）· `fp` 拦截指纹
// （＝同一条拦截在候选池里的 `fp`，见下）。
type GuardEvent struct {
	Ts      string `json:"ts"`
	Tool    string `json:"tool"`
	Agent   string `json:"agent,omitempty"`
	Action  string `json:"action"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message,omitempty"`
	FP      string `json:"fp"`
}

// GuardEventPath —— 事件件落点（`<状态目录>/…`）：与 CLI 侧 `stateDirOf()` 同口径
// （`ZERG_STATE_DIR` → `~/.zerg/state`）。
func GuardEventPath() string { return filepath.Join(statepath.Dir(), GuardEventFile) }

// GuardSymptom —— 拦截事件的**症状文本**（回收面造候选时的 `raw` · 唯一形态）。
//
// 为什么单拎成函数（不各写一份）：`raw` 是 `fp` 的输入，而 `fp` 是幂等的判据 ——
// 采集面（本件）与回收面（`gap idea sweep`）**必须**算出同一个 `raw`，否则同一条拦截
// 会被当两条（或反向：两条不同的拦截合成一条）。★ 文本里**不含时刻**（含时刻 ⇒ 每次
// 重放都新 fp ⇒ 去重失效）。
func GuardSymptom(tool, action, rule string) string {
	if rule != "" {
		return fmt.Sprintf("%s%s 命中规则 %s（action=%s）", guardSymptomPrefix, tool, rule, action)
	}
	return fmt.Sprintf("%s%s 未命中规则——默认 %s", guardSymptomPrefix, tool, action)
}

// GuardFP —— 拦截指纹：候选池里「守卫腿」那条候选的 `fp`（与 `gap idea add` 同取法：
// `sha256(source\\x00raw)[:12]` · `source` 恒 `guard`）。只用于池内去重。
func GuardFP(raw string) string {
	sum := sha256.Sum256([]byte("guard\x00" + raw))
	return hex.EncodeToString(sum[:])[:12]
}

// GuardEventOf —— 由一次控制层判定造一条事件：只有**拦截**（`block` / `require_approval`）
// 才造，放行 ⇒ `ok=false`（口径①）。
func GuardEventOf(tool, agentName string, d GateDecision, now time.Time) (GuardEvent, bool) {
	switch d.Action {
	case ActionBlock, ActionRequireApproval:
	default:
		return GuardEvent{}, false
	}
	return GuardEvent{
		Ts:      now.Format(time.RFC3339Nano),
		Tool:    tool,
		Agent:   agentName,
		Action:  string(d.Action),
		Rule:    d.Rule,
		Message: d.Message,
		FP:      GuardFP(GuardSymptom(tool, string(d.Action), d.Rule)),
	}, true
}

// AppendGuardEvent —— 追加一条事件（一行 JSON · 建目录 · **追加只写**：不动既有行）。
func AppendGuardEvent(path string, ev GuardEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(string(b) + "\n")
	return err
}

// ReadGuardEvents —— 读事件件：**件不在**（从没有过拦截）⇒ 空 + 无错；
// 读不到 / 某行不是 JSON ⇒ 错（调用方译 8 —— 「读不出来」不是「没有」）。
func ReadGuardEvents(path string) ([]GuardEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	out := []GuardEvent{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev GuardEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("守卫事件件第 %d 行不是 JSON：%v", n, err)
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// EnableGuardIntake —— 武装守卫入池：把拦截事件追加到 path（"" ⇒ 关）。幂等。
//
// 生产态由 `NewGateFromFile`（从件装载）调用；`NewGateFromYAML`（合成夹具/单测）不调
// ⇒ 测试不会往真状态目录写事件（见 `gate.go` 的同款注释）。
func (g *Gate) EnableGuardIntake(path string) {
	if g == nil {
		return
	}
	g.guardSink = path
}

// noteGuardInterception —— 判定为拦截时记一条事件（口径③：**best-effort** ——
// 写不进去既不报错、也不改判定与退码；放行不记）。
func (g *Gate) noteGuardInterception(toolName, agentName string, d GateDecision) {
	if g == nil || g.guardSink == "" {
		return
	}
	ev, ok := GuardEventOf(toolName, agentName, d, time.Now())
	if !ok {
		return
	}
	_ = AppendGuardEvent(g.guardSink, ev)
}

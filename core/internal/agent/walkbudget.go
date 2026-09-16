// walkbudget.go — 遍历预算（丙：治大卷上 glob/grep 无节制遍历，实测一次 glob 87 秒）。
//
// 为什么不是"加个 timeout 就完事"：超时只会**掐断**，模型仍拿不到结果、还会重试 ⇒ 更糟。
// 正确做法是**少走**：给遍历一个**条目上限 + 时间上限**，到顶就**停并如实说明**（可行动 ✓）。
// 参照 grep 1.0.1 设计稿同源口径（范围/上限/早停）。
package agent

import (
	"sync/atomic"
	"time"
)

const (
	walkMaxEntries = 200000           // 单次遍历条数上限（防大卷失控）
	walkMaxDur     = 20 * time.Second // 单次遍历时间上限（到顶即停并说明）
)

// walkBudget — 遍历预算：调用方在 Walk 回调里先问 Allowed()，到顶返回 false ⇒ 用 filepath.SkipAll 收尾。
type walkBudget struct {
	entries atomic.Int64
	start   time.Time
	stopped atomic.Bool
	reason  atomic.Value // string
}

func newWalkBudget() *walkBudget {
	return &walkBudget{start: time.Now()}
}

// Allowed — 还能继续走吗？（超条数/超时间 ⇒ false，并记下原因）
func (b *walkBudget) Allowed() bool {
	if b.stopped.Load() {
		return false
	}
	if b.entries.Add(1) > walkMaxEntries {
		b.reason.Store("条目数超过上限")
		b.stopped.Store(true)
		return false
	}
	if time.Since(b.start) > walkMaxDur {
		b.reason.Store("耗时超过上限")
		b.stopped.Store(true)
		return false
	}
	return true
}

// Stopped / Reason / Entries — 供结果尾注说明（如实告诉模型"没走完、为什么" ✓ 可行动）
func (b *walkBudget) Stopped() bool  { return b.stopped.Load() }
func (b *walkBudget) Entries() int64 { return b.entries.Load() }
func (b *walkBudget) Reason() string {
	if v, ok := b.reason.Load().(string); ok {
		return v
	}
	return ""
}

// budgetNote — 到顶时追加的可行动说明（不改结果内容，只在末尾补一句）。
func (b *walkBudget) Note() string {
	if !b.Stopped() {
		return ""
	}
	return "\n…[遍历提前结束：" + b.Reason() + "（已查条目 " + itoa(b.Entries()) +
		"）——请缩小范围（指定更深的 path、或更精确的 pattern）后重试]…\n"
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

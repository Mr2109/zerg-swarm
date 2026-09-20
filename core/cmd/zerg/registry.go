// registry.go —— `zerg help registry`：契约登记表的**命令面视图**（§九 M16 · §十二 `P-084`/`P-085`）。
//
// 为什么把登记表挂到命令面上：`P-085` 要的不是「一个 json 文件」，是**一处能指认的真源**。
// 真源若只有文件、没有任何消费者，它与「写死在文档里」没有区别（§二十一 第 10 条的病灶同族：
// 规则写了、没人接电）。本文件就是那个接电点 —— 它读 `core/internal/contract` 的嵌入清单。
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// helpRegistry —— 渲染登记表（读的是嵌入的 registry.json，**不**另抄一份）。
func helpRegistry() string {
	var b strings.Builder
	b.WriteString("契约的单一真源与变更流程（§九 M16 · §十二 `P-084`–`P-092` · 登记表 = `core/internal/contract/registry.json`）\n\n")
	r, err := contract.Load()
	if err != nil {
		fmt.Fprintf(&b, "⛔ 登记表读不出来：%v\n", err)
		return b.String()
	}
	b.WriteString("真源口径（`P-084`）：**契约真源 = 代码内 schema**；文档与 `/api/capabilities`、`zerg help export` 都是**导出物**。\n")
	if v, ok := r.Authority["rule"].(string); ok {
		b.WriteString("  · " + v + "\n")
	}
	fmt.Fprintf(&b, "\n登记条目（%d 条 · 逐条**指向**真源，不复制内容）：\n", len(r.Entries))
	w := 0
	for _, e := range r.Entries {
		if len(e.Truth) > w {
			w = len(e.Truth)
		}
	}
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "  %-4s %-26s %s  [档 %s]\n", e.ID, e.Name, pad(e.Truth, w), e.ChangeClass)
		fmt.Fprintf(&b, "       版本字段 %s · 门禁 %s\n", e.VersionField, e.Gate)
	}
	b.WriteString("\n变更流程（`V0`–`V7` · **批者 ≠ 作者**）：\n")
	for i, s := range r.ChangeFlow.Steps {
		fmt.Fprintf(&b, "  V%d %s\n", i, s)
	}
	fmt.Fprintf(&b, "  载体：%s\n", r.ChangeFlow.Carrier)
	fmt.Fprintf(&b, "  批者 ≠ 作者：%t\n", r.ChangeFlow.AuthorNotApprover)
	b.WriteString("\n破坏性变更（**留期**成文）：\n  " + r.ChangeFlow.BreakingChangeDeprecate + "\n")
	b.WriteString("\n判据（本件可复跑）：`zerg help registry` 能出表 = 登记表**真的被读**（引用面非 0）；\n")
	b.WriteString("条目数 = 6 处先例 + 本件命令面契约（`S-a`–`S-g`）。\n")
	return b.String()
}

// cmdRegistryCheck 不在命令树里（本版不新立命令名）；`zerg help registry` 就是它的面。
var _ = io.Discard

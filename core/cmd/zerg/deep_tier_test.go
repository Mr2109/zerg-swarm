// deep_tier_test.go —— `core/cmd/zerg` 的**贵档闸**（缺口 `Q-109`：整包耗时贴着 `go test` 的预算线）。
//
// 现读实测（2026-09-23 · `go test ./cmd/zerg -count=1 -json` 本机现跑 → 按顶层用例聚合
// `Elapsed`）：整包 **427.2s**（262 条有耗时的顶层用例合计）；其中影响面族 `TestImpact*`
// **56 条 = 399.5s（93.5%）**，其余 43 个族加在一起只有 **27.6s**。⇒ 这个包的时间**几乎全是
// 影响面族的「现跑」那批**，不是「包大」也不是「机器慢」。
//
// 方案（**照本仓现成先例**，不另造一套开关）：
//
//	· 形态 = **环境变量开关 `ZERG_DEEP=1`** —— 与既有的 `ZERG_LIVE=1`
//	  （`core/internal/chat/infer_live_test.go`：「加 env 开关（X3 熔断/离线时不红 CI——手动
//	  `ZERG_LIVE=1` 跑）」）、`ZERG_REFRESH_HINTS=1`（`unresolved_entries_test.go`：「只在
//	  `ZERG_REFRESH_HINTS=1` 时跑（默认 skip，免得每次 `go test` 都刷一屏）」）同一条写法：
//	  默认档 `t.Skip`、设了才跑；分组则沿用本包既有的 `go test ./cmd/zerg -run TestImpact…` 口径。
//	· 同一手法在**命令面**上早有先例：`zerg impact --deep`（`core/cmd/zerg/main.go`：`--deep` 那
//	  一格逐字写着「要真跑就显式 `--deep` —— 跑一次约 40s，**不给日常路径与门禁套件加这份账**」）；
//	  `scripts/gates/cli-contract-baseline.json` 的 `landed` 里记的就是这条：「**两批贵面收进显式
//	  `--deep`**（跑一次约 40s ⇒ 不给日常路径与门禁套件加这份账）」。本件把同一条判词搬到 Go 测试面。
//
// 闸线（**现读实测的自然断点**，不设魔法数）：**单个用例 ≥ 1.0s** 的 **24 条**收进贵档 ——
// 本机现跑这 24 条 **397.9s = 整包的 93.2%**；而**静态自检那批全部 ≤ 0.4s**（下一名 0.4s，
// 2.5 倍以上的空档）⇒ 这条闸不是拍的：两边的分母（24 条 vs 全包）在日志里都可复算。
//
// 保底保留（默认档**照跑、一条都不删**）：其余 **206 条**用例 + 影响面族里 **32 条 ≤0.4s 的
// 静态自检**（判据件只读源码/契约件：阈值不写死、不落盘、键表与实现对齐……，一次几毫秒）。
// 为什么这么切：
//
//	· 贵 = 「对真仓跑真命令 / 真跑门禁 / 现跑标定 / 起子进程」—— 那是**本机口径**的账，
//	  换个负载就变（本包实测 427s / 536s / 637s 波动）⇒ 把它挂在每次 `go test ./...` 上，
//	  等于**把判据交给机器的忙闲**；本包已因此吃过一次 panic（`panic: test timed out after 10m0s`）。
//	· 免费的那 32 条判的是**源码与契约件本身**，毫秒级、与负载无关 ⇒ 留在默认档**零成本**，
//	  摘出去只会白丢判据（贵档也不换回任何东西）。
//	· 判据**一个都没改**：一个断言、一个阈值、一枚 `want_rc` / `want_stdout_bytes` 都没动 ——
//	  收进贵档变的只是**什么时候跑**，**判什么**一字未动；用例总数也不变
//	  （`go test -list '.*'` 前后同数，`t.Skip` 的用例照在册）。
//
// 两档怎么跑：
//
//	go test ./cmd/zerg -count=1                                 # 默认档（日常路径与门禁套件走这条）
//	ZERG_DEEP=1 go test ./cmd/zerg -count=1                     # 贵档（加回影响面族那 24 条现跑判据）
//	ZERG_DEEP=1 go test ./cmd/zerg -run 'TestImpact' -count=1   # 只要影响面族那一批
//
// ★ 谁把判据件里的 `requireDeep(t)` 摘掉 ⇒ 那一条立刻回到默认档（本文件与门禁步骤表
// `scripts/gates/precommit-gates.sh` 的 `go` 段注释里都写明了这批账为什么不该挂日常路径）。
package main_test

import (
	"os"
	"strings"
	"testing"
)

// requireDeep —— 贵档闸：`ZERG_DEEP=1` 才跑，否则 `t.Skip`。
// 写成**显式 skip** 而不是裸 `return`：静默返回会被日志读成「跑了且绿」（自欺），
// 而 skip 会照实打出「本跑没跑这一条」。
func requireDeep(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("ZERG_DEEP")) == "" {
		t.Skip("贵档（对真仓现跑 / 起子进程 / 真跑门禁 / 现跑标定）：ZERG_DEEP=1 才跑 —— " +
			"默认档不给日常路径与门禁套件加这份账（缺口 Q-109 · 见本件头注释）")
	}
}

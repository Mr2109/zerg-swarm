// Package version — 子端（agent 模块）自己的代码身份。
//
// 为什么要单独一份：`agent/` 是**独立 Go 模块**，Go 的 internal 包规则 + 模块边界
// 明文禁止它导入 `core/internal/version`（实测报 use of internal package … not allowed）。
// 因此这里镜像核心侧的做法——**真相源仍是同一个 git 提交**，只是注入点各在自己在的模块：
//
//	go build -ldflags "-X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=<sha> \
//	                   -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=<UTC>"
//
// 未注入时保持 "unknown"——宁可自报未知，也不假装知道（自动升级 L3 的版本矩阵据此判"混版"）。
package version

// Version / Commit / BuildTime 必须是 var（-ldflags -X 只能写字符串变量）。
var (
	Version   = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
	// Origin —— 构建期注入的**源提交**（同源判定的可比键 · GAP-20261013-09 · 候选 c ·
	// 注入面由 GAP-20261021-08 补齐）：本机二进制所编那棵树的 HEAD 若带 `GitOrigin-RevId` trailer
	// （镜像树逐笔都带）则取 trailer 值（= 私有图源提交）；没有 trailer（私有权威树）则取 HEAD 本身。
	// ✗ 未注入时保持**空串 = 判不了**（主控把本机那一行判成 `unknown`）—— ✗ 绝不回落成 `Commit`：
	// 节点 `Commit` 是**镜像 tip**、与主控的私有提交不同名，回落即把两套命名空间混成一句假判决。
	//
	// ★ 形状由构建器**归一**：一律 12 位十六进制前缀（`scripts/build/cmd/zerg-build-all` 的
	// `gitOrigin`）。理由：trailer 里是 40 位全 sha、`rev-parse --short HEAD` 是 7~8 位，
	// 两侧不归一 ⇒ 逐字比必然 `mixed`（假红）。✗ 不带 `+dirty`：脏是**内容**不同（由 Digest 答），
	// 源提交仍是那一笔。
	// ★ 运行期取源顺序（含手搓构建的兜底）见 `resolveOrigin`：`-X` > 嵌入 tags `origin=` > 环境变量。
	Origin = ""
)

// OriginEnv —— Origin 的**运行期兜底**变量（手搓 `go build` 的机器自报源提交用）。
// 优先级最低（与 `-ldflags -X` 打架时以构建期注入为准），见 `resolveOrigin`。
const OriginEnv = "ZERG_ORIGIN"

// Line 机器可读单行身份（与核心侧同形）：zerg-daemon 2.5.9 abc1234 2026-09-11T07:00:00Z
func Line(component string) string {
	return component + " " + Version + " " + Commit + " " + BuildTime
}

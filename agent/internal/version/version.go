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
)

// Line 机器可读单行身份（与核心侧同形）：zerg-daemon 2.5.9 abc1234 2026-09-11T07:00:00Z
func Line(component string) string {
	return component + " " + Version + " " + Commit + " " + BuildTime
}

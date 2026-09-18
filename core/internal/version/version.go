// Package version —— 版本号单一来源（Go 侧）。
//
// 收版时改这里的 Version + ui/Cargo.toml 的 version（两处必须一致）；
// 一致性由 scripts/check_version.py 断言，已接入 CI 与发布导出流水线。
//
// 背景（APP-A23 收口 2026-09-11）：版本号曾硬编码在 5 处（UI 标题/底栏、模块箱 ×10、
// 启动横幅、capabilities、openapi），互相漂移（模块箱长期是 0.1.0）。
// 现在：UI 侧编译期取自 Cargo.toml（env!("CARGO_PKG_VERSION")），Go 侧只有本文件。
package version

// Version 当前发布版（不带前缀，如 "2.5.9"）——与 ui/Cargo.toml 的 version 一致。
const Version = "2.5.10"

// Tag 展示用带前缀形式（横幅、能力清单、openapi），如 "v2.5.9"。
const Tag = "v" + Version

// Commit / BuildTime —— 构建时由 -ldflags -X 注入的代码身份（自动升级模块的验证依据）。
// 必须是 var（-X 只能写字符串变量，不能写 const）；未注入时为 "unknown"。
// 注入命令见 scripts/build-all.sh：
//
//	-ldflags "-X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=<sha40/短> \
//	          -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=<UTC ISO8601>"
var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Line —— 机器可读的单行身份（升级器/安装器比对用，纯 ASCII 空格分隔）：
//
//	zerg-core 2.5.9 e322802c 2026-09-11T05:28:05Z
func Line(component string) string {
	return component + " " + Version + " " + Commit + " " + BuildTime
}

// Describe —— 人类可读的完整身份，如 "v2.5.9+2ddeba9e（2026-09-11T05:33:09Z）"。
// 用于启动横幅、upgrade --plan 的盘点输出、版本矩阵。
func Describe() string {
	out := Tag + "+" + Commit
	if BuildTime != "unknown" && BuildTime != "" {
		out += "（" + BuildTime + "）"
	}
	return out
}

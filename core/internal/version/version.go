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
const Version = "2.5.9"

// Tag 展示用带前缀形式（横幅、能力清单、openapi），如 "v2.5.9"。
const Tag = "v" + Version

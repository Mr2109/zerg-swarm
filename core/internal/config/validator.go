package config

import (
	"os"
	"strings"
)

// ValidationLevel 校验严重级别
type ValidationLevel int

const (
	FatalLevel ValidationLevel = iota // 致命错误，配置不可用
	WarnLevel                         // 警告，配置可能有问题但可用
	InfoLevel                         // 提示信息
)

// String 返回校验级别的中文标签
func (l ValidationLevel) String() string {
	switch l {
	case FatalLevel:
		return "Fatal"
	case WarnLevel:
		return "Warn"
	case InfoLevel:
		return "Info"
	default:
		return "Unknown"
	}
}

// ValidationError 一条校验结果
type ValidationError struct {
	Field   string
	Level   ValidationLevel
	Message string
}

// ValidationResult 校验结果汇总
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

// HasFatal 检查是否存在 Fatal 级别错误
func (r ValidationResult) HasFatal() bool {
	return len(r.Errors) > 0
}

// KnownFamilies 已知模型家族列表
var KnownFamilies = []string{
	"llama", "gpt", "claude", "gemini", "mistral", "qwen", "deepseek",
	"ornith", "cogvlm", "internlm", "yi", "baichuan", "chatglm",
	"falcon", "command-r", "dbrx", "mixtral", "codellama", "wizardlm",
}

// MainlineUnsupportedArchitectures 主线 llama.cpp **不支持**的架构清单。
//
// 判据来源（不是推测）：这类架构必须由**非主线引擎实现**（专用 fork 二进制 + 包装脚本）
// 承载，卵清单里必须有 cmd:，否则会静默退回主线引擎 —— 后果是起不来
// （主线报 unknown model architecture）或误链，**而且都不报错**。
//   - k2-horizon：agent/internal/modeladapter/k2horizon.go 文件头（llama.cpp IFM fork 专用，
//     主线未支持 issue#28361）；
//   - 设计-子端沙箱化-20260914 §1.2（引擎实现/变体是卵的必需字段）、§4.7（卵清单必须带上它，
//     集群级统一）、附录 C·C1（fleet.yaml 全篇 0 处 cmd: ⇒ 第二台设备孵 K2 会静默退回主线）。
var MainlineUnsupportedArchitectures = []string{"k2-horizon"}

// eggArch 卵的架构标识（architecture 优先，回落 arch）。
func eggArch(c ModelCandidate) string {
	if s := strings.TrimSpace(c.Architecture); s != "" {
		return s
	}
	return strings.TrimSpace(c.Arch)
}

// NeedsEngineImpl 这枚卵是否**必须显式声明引擎实现/变体**（即 cmd: 必须非空）。
//
// 两种触发（任一）：
//  1. 架构在 MainlineUnsupportedArchitectures 里（主线引擎根本起不来）；
//  2. 条目显式打了 engine_impl_required: true（新引擎进清单前的通用开关，不必改本函数）。
//
// 不适用的情形：backend 已是非 llama 家族（如 ds4-server）——那时「引擎实现」由 backend 字段
// 承载（manager 侧按 backend 直接取 ds4 可执行文件），不重复要求 cmd:。
func NeedsEngineImpl(c ModelCandidate) bool {
	if c.Backend != "" && c.Backend != "llama-server" {
		return false
	}
	if c.EngineImplRequired {
		return true
	}
	arch := strings.ToLower(eggArch(c))
	for _, a := range MainlineUnsupportedArchitectures {
		if arch == a {
			return true
		}
	}
	return false
}

// isKnownHost 检查主机是否在 fleet 已知列表中
func isKnownHost(host string) bool {
	// 3c（2026-09-16）：本机角色（localback）退役 ⇒ 本机 = 名为 Mr2109 的普通子端（与 x3 同形）。
	// ⚠ 这份名单是**硬编码**的：以后加机器别忘了同步（或改为从 fleet 动态取，承接项）。
	known := []string{"Mr2109", "x3", "mini1", "mini2", "mini3"}
	for _, h := range known {
		if h == host {
			return true
		}
	}
	return false
}

// isKnownFamily 检查家族是否在已知列表中
func isKnownFamily(family string) bool {
	family = strings.ToLower(family)
	for _, known := range KnownFamilies {
		if known == family {
			return true
		}
	}
	return false
}

// Validate 校验一个 ModelCandidate，返回 V001-V015 全部结果。
// name 由调用方从 map key 传入（fleet.yaml 的 models 是 map——name 不在 candidate 内）
func Validate(name string, candidate ModelCandidate) ValidationResult {
	var errs []ValidationError
	var warnings []ValidationError

	// ===== V001: name 必填 =====
	if name == "" {
		errs = append(errs, ValidationError{
			Field:   "name",
			Level:   FatalLevel,
			Message: "模型名称不能为空",
		})
	}

	// ===== V002: family 必填（存量兼容：可从 name 推断——取 name 首段，Warn 不 Fatal）=====
	// M0 过渡期决策（2026-08-13）：存量 fleet.yaml 无 family 字段——硬拒会全炸
	// 新模型严格必填；存量推断（family = name 首段）+ Warn
	if candidate.Family == "" {
		if name != "" {
			inferred := name
			for _, sep := range []string{"-", "_", "."} {
				if idx := strings.Index(name, sep); idx > 0 {
					inferred = name[:idx]
					break
				}
			}
			warnings = append(warnings, ValidationError{
				Field:   "family",
				Level:   WarnLevel,
				Message: "模型家族为空——已从名称推断: " + inferred,
			})
		} else {
			errs = append(errs, ValidationError{
				Field:   "family",
				Level:   FatalLevel,
				Message: "模型家族不能为空",
			})
		}
	}

	// ===== V003: host 必填 =====
	if candidate.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "host",
			Level:   FatalLevel,
			Message: "主机不能为空",
		})
	}

	// ===== V004: backend 必填 =====
	if candidate.Backend == "" {
		errs = append(errs, ValidationError{
			Field:   "backend",
			Level:   FatalLevel,
			Message: "后端类型不能为空",
		})
	}

	// ===== V005: file 必填 =====
	if candidate.File == "" {
		errs = append(errs, ValidationError{
			Field:   "file",
			Level:   FatalLevel,
			Message: "文件路径不能为空",
		})
	}

	// ===== V006: file 本地存在性（仅本机子端时检查；3c：host 名 local → Mr2109）=====
	// 口径不变：只有"跑在主控这台机器上"的候选才能由主控直接 stat；远程机由子端自己核。
	if candidate.File != "" && candidate.Host == "Mr2109" {
		if _, err := os.Stat(candidate.File); os.IsNotExist(err) {
			errs = append(errs, ValidationError{
				Field:   "file",
				Level:   FatalLevel,
				Message: "文件不存在：" + candidate.File,
			})
		}
	}

	// ===== V007: mem_gb > 0 =====
	if candidate.MemGb <= 0 {
		errs = append(errs, ValidationError{
			Field:   "mem_gb",
			Level:   FatalLevel,
			Message: "内存预算必须为正数",
		})
	}

	// ===== V008: ctx_window > 0（存量兼容：默认 4096——Warn 不 Fatal）=====
	if candidate.CtxWindow <= 0 {
		warnings = append(warnings, ValidationError{
			Field:   "ctx_window",
			Level:   WarnLevel,
			Message: "上下文窗口未设置——默认 4096（存量兼容）",
		})
	}

	// ===== V009: modality 必填（存量兼容：默认 text——Warn 不 Fatal）=====
	if candidate.Modality == "" {
		warnings = append(warnings, ValidationError{
			Field:   "modality",
			Level:   WarnLevel,
			Message: "模态为空——默认 text（存量兼容）",
		})
	}

	// ===== V010: tool_support 不能为空（nil=未知——存量兼容：默认 false——Warn 不 Fatal）=====
	if candidate.ToolSupport == nil {
		warnings = append(warnings, ValidationError{
			Field:   "tool_support",
			Level:   WarnLevel,
			Message: "工具支持标记为空——默认 false（存量兼容）",
		})
	}

	// ===== V016: 卵清单必须承载「引擎实现/变体」（P1；设计 §1.2 / §4.7 / 附录 C·C1）=====
	// 判据：需要非主线引擎实现的卵（NeedsEngineImpl）**缺 cmd:** ⇒ Fatal（拒孵）。
	// 理由：缺失时 manager 会落到 detectLlamaServerPath() 的**主线** llama-server ⇒
	// 起不来（主线报 unknown model architecture: k2-horizon）或误链，**而且不报错**
	// ——「静默退回主线引擎」是本项要堵的那一个坑，故判 Fatal 而不是 Warn。
	if NeedsEngineImpl(candidate) && len(candidate.Cmd) == 0 {
		errs = append(errs, ValidationError{
			Field: "cmd",
			Level: FatalLevel,
			Message: "引擎实现/变体缺失：架构 " + eggArch(candidate) +
				" 主线引擎不支持，必须声明 cmd:（专用 fork 二进制 + 包装脚本，含 env 处理）" +
				"——不许静默退回主线 llama-server",
		})
	}

	// ===== 以下为 Warn/Info 级别 =====

	// V011: host 不在 fleet 已知主机中
	if candidate.Host != "" && !isKnownHost(candidate.Host) {
		warnings = append(warnings, ValidationError{
			Field:   "host",
			Level:   WarnLevel,
			Message: "主机 " + candidate.Host + " 不在 fleet.yaml 已知主机列表中",
		})
	}

	// V012: mem_gb > 128 但无 SSD
	if candidate.MemGb > 128 && !candidate.SSD {
		warnings = append(warnings, ValidationError{
			Field:   "mem_gb",
			Level:   WarnLevel,
			Message: "内存预算 > 128GB 但未配置 SSD 缓存",
		})
	}

	// V013: description 为空
	if candidate.Description == "" {
		warnings = append(warnings, ValidationError{
			Field:   "description",
			Level:   InfoLevel,
			Message: "模型描述为空",
		})
	}

	// V014: 未知家族
	if candidate.Family != "" && !isKnownFamily(candidate.Family) {
		warnings = append(warnings, ValidationError{
			Field:   "family",
			Level:   WarnLevel,
			Message: "未知模型家族：" + candidate.Family,
		})
	}

	// V015: arch 标识为空
	if candidate.Arch == "" {
		warnings = append(warnings, ValidationError{
			Field:   "arch",
			Level:   InfoLevel,
			Message: "架构标识为空",
		})
	}

	return ValidationResult{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warnings,
	}
}

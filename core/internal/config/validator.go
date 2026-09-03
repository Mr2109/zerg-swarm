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

// isKnownHost 检查主机是否在 fleet 已知列表中
func isKnownHost(host string) bool {
	known := []string{"local", "x3", "mini1", "mini2", "mini3"}
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

	// ===== V006: file 本地存在性（仅 host=local 时检查）=====
	if candidate.File != "" && candidate.Host == "local" {
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

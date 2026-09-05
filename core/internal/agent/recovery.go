package agent

// recovery.go - 虫族 v2.5 Agent 错误恢复机制
// 设计：
//   - RetryPolicy + Retry：有界重试 + 指数退避
//   - NoProgressDetector：检测连续无进展（连续 N 轮结果相同）
//   - 错误分类：IsTransient（网络/超时/5xx） vs IsFatal（语法/权限/不存在）
//   - 对齐设计文档 14.3：retry 有界 + 退避 + 升级，区分瞬时/不可解/无进展

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// 默认重试策略配置
const (
	DefaultMaxRetries  = 3
	DefaultBaseBackoff = 2 * time.Second
)

// RetryPolicy - 重试策略（有界 + 指数退避）
// 零值不是合法策略——MaxRetries 默认为 0，需在 Retry 前设置
type RetryPolicy struct {
	MaxRetries int
	Backoff    time.Duration
	Retryable  func(err error) bool
}

// DefaultPolicy - 返回默认重试策略
func DefaultPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: DefaultMaxRetries,
		Backoff:    DefaultBaseBackoff,
		Retryable:  IsTransient,
	}
}

// Retry - 带指数退避的有界重试
//
// fn: 要执行的函数，接收 error 参数（nil 表示成功）
// policy: 重试策略
//
// 执行流程：
//
//  1. 调用 fn()
//  2. 失败且可重试 → 退避 sleep → 重试
//  3. <=MaxRetries 次后成功 → 返回 nil
//  4. 不可重试 / 次数耗尽 → 返回最后一次错误
func Retry(fn func() error, policy RetryPolicy) error {
	if policy.MaxRetries <= 0 {
		policy.MaxRetries = DefaultMaxRetries
	}
	if policy.Backoff <= 0 {
		policy.Backoff = DefaultBaseBackoff
	}
	if policy.Retryable == nil {
		policy.Retryable = IsTransient
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt >= policy.MaxRetries || !policy.Retryable(err) {
				return err
			}
			// 指数退避
			backoff := time.Duration(float64(policy.Backoff) * math.Pow(2, float64(attempt)))
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			time.Sleep(backoff)
		} else {
			return nil
		}
	}
	return lastErr
}

// --- NoProgressDetector ---

// NoProgressDetector - 连续 N 轮结果相同则判定无进展
// 线程安全，可在并发循环中使用
type NoProgressDetector struct {
	mu        sync.Mutex
	Threshold int
	lastHash  string
	count     int
}

// NewNoProgressDetector - 创建无进展检测器（默认阈值 3）
func NewNoProgressDetector() *NoProgressDetector {
	return &NoProgressDetector{Threshold: 3}
}

// Check - 检查无进展
// content: 本轮结果内容（会被哈希）
//
// 返回 (isNoProgress, count)：
//
//	isNoProgress: true 表示连续 N 次结果相同，判定为无进展
//	count: 当前连续相同次数
//
// 线程安全，可在并发循环中使用
func (npd *NoProgressDetector) Check(content string) (isNoProgress bool, count int) {
	npd.mu.Lock()
	defer npd.mu.Unlock()

	hash := hashContent(content)

	if npd.lastHash == hash {
		npd.count++
	} else {
		npd.count = 1
		npd.lastHash = hash
	}

	threshold := npd.Threshold
	if threshold <= 0 {
		threshold = 3
	}

	return npd.count >= threshold, npd.count
}

// Reset - 重置检测器状态
func (npd *NoProgressDetector) Reset() {
	npd.mu.Lock()
	defer npd.mu.Unlock()
	npd.lastHash = ""
	npd.count = 0
}

// Count - 获取当前连续相同次数（线程安全）
func (npd *NoProgressDetector) Count() int {
	npd.mu.Lock()
	defer npd.mu.Unlock()
	return npd.count
}

// hashContent - 对内容做 SHA-256 哈希，用于无进展检测
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// --- 错误分类 ---

// IsTransient - 判断错误是否为瞬时错误（可重试）
//
// 瞬时错误：网络断开、超时、5xx 服务端错误等——稍后可能自动恢复
// 致命错误：语法错误、权限不足、资源不存在等——重试无意义
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	// 常见网络/超时关键词
	transientKeywords := []string{
		"timeout",
		"timed out",
		"connection refused",
		"connection reset",
		"connection closed",
		"network is unreachable",
		"no such host",
		"i/o timeout",
		"read tcp",
		"write tcp",
		"EOF",
		"broken pipe",
		"too many open files",
		"rate limit",
		"temporary failure",
	}
	for _, kw := range transientKeywords {
		if strings.Contains(strings.ToLower(msg), kw) {
			return true
		}
	}

	// 检查 HTTP 状态码
	for _, code := range []int{500, 502, 503, 504} {
		if strings.Contains(msg, fmt.Sprintf("%d", code)) {
			return true
		}
	}

	// 检查 errors.Is 匹配（Go 1.13+ error wrapping）
	var netErr interface{ Temporary() bool }
	if errors.As(err, &netErr) {
		return true
	}

	return false
}

// IsFatal - 判断错误是否为致命错误（不可重试）
//
// 致命错误：语法错误、权限不足、文件不存在、格式错误等——重试无意义
// 与 IsTransient 互补：一个 error 不可能同时是 transient 和 fatal
func IsFatal(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	// 常见致命错误关键词
	fatalKeywords := []string{
		"permission denied",
		"access denied",
		"not found",
		"no such file",
		"no such directory",
		"parse error",
		"syntax error",
		"invalid argument",
		"invalid format",
		"bad request",
		"unauthorized",
		"forbidden",
		"already exists",
		"conflict",
	}
	for _, kw := range fatalKeywords {
		if strings.Contains(strings.ToLower(msg), kw) {
			return true
		}
	}

	return false
}

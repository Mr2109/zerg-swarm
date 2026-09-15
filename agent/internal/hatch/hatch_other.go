//go:build !linux

// hatch_other.go —— 非 Linux 平台的孵化器桩：**明确拒绝**，不假装能孵。
//
// 为什么不做"本地模拟"：孵化依赖 bwrap + systemd 用户实例 + AMD 设备节点，这些在 macOS 上
// 不存在；假装成功会让上层把"没孵"当"孵好了"——正是 §6.9 要防的那类静默失效。
// macOS 上可测的是**声明校验与命令行构造**（纯函数，见 hatch.go / hatch_check.go）。
package hatch

import (
	"context"
	"errors"
	"time"
)

// ErrUnsupportedPlatform 非 Linux：真孵化不支持（命令构造仍可用，做 dry-run/单测）。
var ErrUnsupportedPlatform = errors.New("孵化仅支持 Linux（bwrap + systemd 用户实例 + 设备节点）；本平台只能用声明校验与命令行构造做 dry-run")

// Hatcher 非 Linux 版（保留同名类型，让上层代码跨平台编译）。
type Hatcher struct {
	StopTimeout time.Duration
}

func (h Hatcher) Hatch(_ context.Context, spec Spec) (string, error) {
	if err := spec.Validate(); err != nil {
		return UnitName(spec.EggID), err // 声明错误照常暴露（这部分跨平台一致）
	}
	return UnitName(spec.EggID), ErrUnsupportedPlatform
}

func (h Hatcher) Collect(_ context.Context, _ string) error { return ErrUnsupportedPlatform }

func (h Hatcher) Active(_ context.Context, _ string) (bool, error) {
	return false, ErrUnsupportedPlatform
}

func (h Hatcher) UnitCgroup(_ context.Context, _ string) (string, error) {
	return "", ErrUnsupportedPlatform
}

func (h Hatcher) MainPID(_ context.Context, _ string) (int, error) {
	return 0, ErrUnsupportedPlatform
}

func (h Hatcher) VerifyEnclosure(_ int) (EnclosureReport, error) {
	return EnclosureReport{}, ErrUnsupportedPlatform
}

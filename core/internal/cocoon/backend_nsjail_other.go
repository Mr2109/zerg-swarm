//go:build !linux

// backend_nsjail_other.go — 非 linux 平台上的 nsjail 后端：**有类型、没能力**。
//
// 为什么不是「整个类型不存在」：调用方（CLI / 编排层）要能不判平台就把后端名映射成
// 一个 Backend；类型不存在会让每个调用点各写一遍 `if runtime.GOOS == "linux"`，
// 而且总有一处会忘。这里让五个动作**统一**返回 ErrBackendUnsupported —— 失败得早、
// 失败得明（不是「跑了个空茧」那种假成功）。
//
// ★ 也不许「降级成假后端」：FakeBackend 会把结果照预设吐出来，一个真茧被静默替换成
// 假后端，恰恰是「谎报能力」的那类缺陷。
package cocoon

import (
	"context"
	"fmt"
	"runtime"
)

// nsjailUnsupported — 统一的「本平台没有 nsjail 后端」错误（带平台名与动作名，便于审计）。
func nsjailUnsupported(op string) error {
	return fmt.Errorf("%w: nsjail 后端只在 %s 上可用（本平台 %s，%s 不可用）",
		ErrBackendUnsupported, PlatformLinux, runtime.GOOS, op)
}

// Provision — 本平台不支持。
func (b *NsjailBackend) Provision(context.Context, *Spec) (Instance, error) {
	return Instance{}, nsjailUnsupported("Provision")
}

// Exec — 本平台不支持。
func (b *NsjailBackend) Exec(context.Context, Instance, []string, ExecOpts) (ExecResult, error) {
	return ExecResult{}, nsjailUnsupported("Exec")
}

// Mount — 本平台不支持。
func (b *NsjailBackend) Mount(context.Context, Instance, string, string, bool) error {
	return nsjailUnsupported("Mount")
}

// Expose — 本平台不支持（linux 上另有「不映射端口」的专门说明）。
func (b *NsjailBackend) Expose(context.Context, Instance, int, string) (int, error) {
	return 0, nsjailUnsupported("Expose")
}

// Destroy — 本平台不支持：这里什么都没 provision 过，返回 nil 等于假装「销毁成功」。
func (b *NsjailBackend) Destroy(context.Context, Instance) error {
	return nsjailUnsupported("Destroy")
}

// Capabilities — 本平台没有这份能力：返回错误而不是一张「全 false 的表」
// （全 false 的表会被误读成「后端在，只是弱」；事实上是后端根本不在）。
func (b *NsjailBackend) Capabilities() (Capabilities, error) {
	return Capabilities{}, nsjailUnsupported("Capabilities")
}

// InstancePIDs — 本平台不起进程 ⇒ 没有 pid 可报（空列表就是实话，不是错误）。
func (b *NsjailBackend) InstancePIDs(Instance) []int { return nil }

// ReapPersisted — 本平台什么都 provision 过：不许返回 nil（那会被读成「收干净了」）。
func (b *NsjailBackend) ReapPersisted(context.Context, Instance, []int) error {
	return nsjailUnsupported("ReapPersisted")
}

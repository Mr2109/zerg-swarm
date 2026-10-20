package cocoon

// backend_caps_test.go — 能力表的**端口面**（Expose / ExposeMechanism）与两个新可选面的诚实性。
//
// 三格：
//
//	① 表完整：三平台都得过 Validate（Expose=false 时 ExposeMechanism 必须写清 ——
//	   「不映射」与「忘了写」不能被读成同一件事）。
//	② linux 行：Expose=false（nsjail 不做 DNAT），且理由里点明「同宿主网络栈」。
//	③ 非 linux 上 ReapPersisted 必须 ErrBackendUnsupported（不许 nil —— 那会被读成「收干净了」），
//	   InstancePIDs 空列表（没有进程就是没有，不是错误）。
//	④ ExecOpts.LogPath 只收宿主绝对路径（相对路径会在服务起来之后才炸，且炸在茧里看不到）。

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestCapabilitiesExposeFace(t *testing.T) {
	for _, p := range Platforms() {
		c, ok := PlatformCapabilities(p)
		if !ok {
			t.Fatalf("平台 %q 没有能力声明", p)
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("平台 %q 的能力声明过不了 Validate：%v", p, err)
		}
		// Expose=false 时必须写清「为什么不映射」；true 时必须写清「怎么映射」。
		if c.ExposeMechanism == "" {
			t.Fatalf("平台 %q 的 ExposeMechanism 为空（%t）—— 空值等于「没说」", p, c.Expose)
		}
	}

	// linux/nsjail：不映射端口（茧与调用方同网络栈）——这条是判据，不是描述。
	lin := LinuxCapabilities()
	if lin.Expose {
		t.Fatalf("linux 行 Expose 应为 false（nsjail 不做 DNAT），得到 %+v", lin)
	}
	if !strings.Contains(lin.ExposeMechanism, "同宿主网络栈") {
		t.Fatalf("linux 的 ExposeMechanism 没写清「同宿主网络栈」这个理由：%q", lin.ExposeMechanism)
	}

	// 负控：Expose=false 且机制为空 ⇒ Validate 必须拒（否则「没说」会被当成「没问题」）。
	bad := LinuxCapabilities()
	bad.ExposeMechanism = ""
	if err := bad.Validate(); err == nil {
		t.Fatal("Expose=false + 空机制 应当被 Validate 拒")
	}
}

func TestNsjailNonLinuxReapAndPIDs(t *testing.T) {
	if runtime.GOOS == PlatformLinux {
		t.Skip("linux 上这两个是真实现（见 TestNsjailLinuxBookkeeping）")
	}
	be := NewNsjailBackend()
	ctx := context.Background()

	// 没有进程可报就是空 —— 这是事实，不是失败。
	if pids := be.InstancePIDs(Instance{ID: "x", Backend: "nsjail"}); len(pids) != 0 {
		t.Fatalf("非 linux 上不该报出 pid，得到 %v", pids)
	}
	// 但「按句柄收尾」必须拒：返回 nil 会被上层读成「已经收干净了」。
	err := be.ReapPersisted(ctx, Instance{ID: "x", Backend: "nsjail"}, []int{1})
	if !errors.Is(err, ErrBackendUnsupported) {
		t.Fatalf("非 linux 上 ReapPersisted 应 ErrBackendUnsupported，得到 %v", err)
	}
}

func TestExecOptsLogPathValidation(t *testing.T) {
	if err := (ExecOpts{LogPath: "logs/api.log"}).Validate(); err == nil {
		t.Fatal("相对 LogPath 应当被拒（它会相对茧内 cwd 解析，落点不可控）")
	}
	if err := (ExecOpts{LogPath: "/tmp/a.log"}).Validate(); err != nil {
		t.Fatalf("绝对 LogPath 应当通过：%v", err)
	}
	if err := (ExecOpts{}).Validate(); err != nil {
		t.Fatalf("空 LogPath 是合法缺省：%v", err)
	}
}

//go:build linux

// egg8_gpu_empty_root_linux_test.go —— GAP-20261018-14：**空根 bwrap 里看得见 GPU**。
//
// 第 7 步证到的是「空根里跑得起真引擎」（rc=0 三档）＋「五件 .so 的 (dev,inode) 等于展开目录」；
// 它没打满的那一格是 HIP 设备探测：逐字 `ggml_cuda_init: failed to initialize ROCm: no ROCm-capable
// device is detected`。本件把那一格补上，并把根因钉成回归：
//
//	真因（edge-node 真跑 2026-10-18）＝ **库层**，不是设备/权限/`/sys`：
//	  设备节点可 open(O_RDWR)、`/sys/class/kfd/kfd/topology/nodes` 可读、整棵 `/dev` 绑进来也不改善；
//	  而 HIP 用 `dlopen("libamd_comgr.so.3")` 加载 COMGR —— 它**不在任何 DT_NEEDED 面**，
//	  打蛋端的闭包自检因此看不见它 ⇒ 蛋里没有 COMGR ⇒ rocclr 建不出 Device：
//	    comgrctx.cpp: Failed to load COMGR library.
//	    rocdevice.cpp: Code object manager initialization failed for HSA device gfx1151
//	    rocdevice.cpp: Error creating new instance of Device
//	    hipGetDeviceCount: Returned hipErrorNoDevice
//
// 判据（缺一不可）：
//
//	① 孵化前自检过（`hatch.CheckEggGPUClosure`）：蛋声明要 GPU 就必须带 dlopen 层的 ROCm 运行时件；
//	② 生产链路（ExpandEgg → hatchSpecFor → hatch.BuildBwrapArgv）在空根里跑 `--list-devices`，rc=0；
//	③ stdout 里出现 **`ROCm0`**（★ 这一行就是「空间里真认到 GPU」的唯一硬读数；
//	   ✗ 不许拿「日志里没有那句报错」当证据 —— 成功路径根本不打印它）。
//
// 跑法（edge-node；蛋里带 COMGR 闭包的那一枚）：
//
//	ZERG_EGG8_EGG=/tmp/egg7verify/real8/out/real8.egg \
//	ZERG_EGG8_ROOT=/tmp/egg7verify/real8/expanded4 \
//	ZERG_EGG8_WORK=/tmp/egg7verify/real8/work \
//	go test -v -run TestEgg8GPUVisibleInEmptyRoot ./internal/backend/
package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/eggpkg"
	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
)

// TestEgg8GPUVisibleInEmptyRoot —— 空根里 `--list-devices` 必须列出 `ROCm0`。
//
// 三档跳过（都写成人话，免得把「没跑」误读成「过了」）：没给蛋 / 本机没 bwrap / 宿主没有 `/dev/kfd`
// （没有 ROCm 设备的机器上这一格本来就判不了）。
func TestEgg8GPUVisibleInEmptyRoot(t *testing.T) {
	egg := strings.TrimSpace(os.Getenv("ZERG_EGG8_EGG"))
	root := strings.TrimSpace(os.Getenv("ZERG_EGG8_ROOT"))
	if egg == "" || root == "" {
		t.Skip("未给 ZERG_EGG8_EGG / ZERG_EGG8_ROOT ⇒ 跳过（本用例要在带 bwrap 与 ROCm 设备的 Linux 上真空根跑）")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("本机没有 bwrap ⇒ 跳过：空根只能在真 bwrap 上造")
	}
	if _, err := os.Stat("/dev/kfd"); err != nil {
		t.Skip("宿主没有 /dev/kfd ⇒ 本机没有 ROCm 设备，这一格判不了")
	}
	work := strings.TrimSpace(os.Getenv("ZERG_EGG8_WORK"))
	if work == "" {
		work = filepath.Join(os.TempDir(), "egg8work")
	}
	t.Setenv(EnvWorkDirRoot, work) // ✗ 不往家目录写

	// ── ① 展开（幂等：已展开的落点直接复用，一个字节都不写）──────────────────────
	res, err := eggpkg.ExpandEgg(egg, root)
	if err != nil {
		t.Fatalf("ExpandEgg(%s → %s)：%v", egg, root, err)
	}
	t.Logf("EGG8|expand|segments=%d|files=%d|symlinks=%d|bytes=%d|skipped=%v",
		res.Segments, res.Files, res.Symlinks, res.Bytes, res.Skipped)

	// ── ② 生产映射：卵声明 → 孵化声明（✗ 不手搓 spec）──────────────────────────────
	spec, err := egg7Spec(t, egg, root)
	if err != nil {
		t.Fatalf("hatchSpecFor（生产映射）：%v", err)
	}
	// ②b 孵化前自检：蛋声明要 GPU ⇒ 缺 COMGR 这类 dlopen 层件必须**在这里**被点名拒掉
	//    （运行时那句 `no ROCm-capable device is detected` 既不点名件、也不指向库层）。
	if err := hatch.CheckEggGPUClosure(spec); err != nil {
		t.Fatalf("孵化前 GPU 运行时自检没过（蛋里缺 dlopen 层的 ROCm 运行时件？）：%v", err)
	}

	// ── ③ 空根里跑 `--list-devices` ──────────────────────────────────────────────
	s := spec
	s.EngineArgs = []string{"--list-devices"}
	argv, err := hatch.BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	if egg7HasUsrBind(argv) {
		t.Fatalf("蛋模式 argv 里仍有 `--ro-bind /usr /usr`（那就不是空根了）：%v", argv)
	}
	out, errOut, rc := egg7Run(argv)
	t.Logf("EGG8|argv|elements=%d|has_usr_bind=false|engine=%s", len(argv), spec.EnginePathInSpace)
	t.Logf("EGG8|run|list-devices|rc=%d|stdout=%q|stderr=%q", rc, egg7Clip(out, 800), egg7Clip(errOut, 1200))
	if rc != 0 {
		t.Fatalf("空根里 `--list-devices` rc=%d（要 0）：stdout=%q stderr=%q", rc, egg7Clip(out, 800), egg7Clip(errOut, 1200))
	}
	if !strings.Contains(out, "ROCm0") {
		t.Fatalf("空根里看不到 GPU：stdout 里没有 `ROCm0`（stdout=%q stderr=%q）——"+
			"若 stderr 有 `no ROCm-capable device is detected`，先核蛋里有没有 libamd_comgr.so.3（dlopen 面）",
			egg7Clip(out, 800), egg7Clip(errOut, 1200))
	}
	// 逐行把设备面打出来（★ 这是本单的验收读数，✗ 不折成一句「看到了」）。
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			t.Logf("EGG8|device_out|%s", strings.TrimSpace(ln))
		}
	}
	// 负控：命中成功路径时**不该**再出现那句 no-device 报错（两者同时出现 = 语义自相矛盾，
	// 会把「第二块卡起不来」误读成「整机没卡」）。
	if strings.Contains(errOut, "no ROCm-capable device is detected") {
		t.Fatalf("stdout 有 ROCm0 却又报 no ROCm-capable device（自相矛盾）：stderr=%q", egg7Clip(errOut, 1200))
	}
}

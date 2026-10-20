//go:build linux

// backend_nsjail_rootfs_it_linux_test.go — 真 rootfs 的**真机**验收（四格）。
//
// 只在「真 linux + root + 有 nsjail + 指了一个镜像根目录」时跑；缺任何一样都 Skip 并说清缺什么
// （缺前置就 Skip 是本仓的惯例：假绿比不跑更糟，所以 Skip 的理由必须能被读到）。
//
// 环境变量：
//
//	ZERG_COCOON_IT_ROOTFS  必需。已解包的最小 rootfs 目录（debian:bookworm 解包 / busybox 静态都行）
//	ZERG_COCOON_IT_BASE    可选。茧目录父目录（缺省 t.TempDir()）。★ 该盘必须能当 overlay 的
//	                       **上层**：tmpfs/ext4/xfs 可以；容器里的 overlayfs（/tmp 常是它）不行。
//
// 四格断言（对应任务书 5①②③④）：
//
//	① 茧内 /bin/sh 真跑起来（stdout 里有 from-image）；
//	② 茧内看不到宿主仓与家目录（拿**绝对路径**在茧内探，存在就是漏）；
//	③ 写 /tmp 成功、写 /usr 失败（系统面只读）；
//	④ overlay 的 upper 里出现茧内新写的文件（可写层真的落在我们指定的地方）。
//
// 跑法（本机 macOS 上照 M1 的办法）见仓外脚本；容器要 `--privileged --cgroupns=host`
// 且把镜像根与仓分别 bind 进去。
package cocoon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNsjailRootfsIntegration(t *testing.T) {
	rootfs := strings.TrimSpace(os.Getenv("ZERG_COCOON_IT_ROOTFS"))
	if rootfs == "" {
		t.Skip("未设 ZERG_COCOON_IT_ROOTFS（真 rootfs 目录）：本格只在 linux 真机/特权容器里跑")
	}
	if os.Geteuid() != 0 {
		t.Skip("需要 root：overlay mount + chroot 都要它")
	}
	if _, err := exec.LookPath("nsjail"); err != nil {
		t.Skipf("找不到 nsjail（%v）", err)
	}
	if fi, err := os.Stat(rootfs); err != nil || !fi.IsDir() {
		t.Skipf("ZERG_COCOON_IT_ROOTFS=%s 不是一个可读目录（err=%v）", rootfs, err)
	}

	ctx := context.Background()
	be := NewNsjailBackend()
	be.BaseDir = os.Getenv("ZERG_COCOON_IT_BASE")
	if be.BaseDir == "" {
		be.BaseDir = t.TempDir()
	}
	be.HostNetwork = true // 命令面的取法（茧内端口即宿主端口），与本格的四条断言无关

	caps, err := be.Capabilities()
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if !caps.Overlay {
		t.Fatalf("linux 上 Overlay 应为 true，得到 %+v", caps)
	}

	// 契约面：只给 rootfs（[茧].overlay_upper 留空 ⇒ 缺省 <茧目录>/upper）。
	// ★ 资源位留零：ResourceSpec 的 cgroup 面在 M1 已被单独验过，本格只验 rootfs 那四条。
	sp := &Spec{
		File:   SpecFileName,
		Cocoon: CocoonSpec{Name: "rootfs-it", Image: "debian:bookworm", Rootfs: rootfs},
	}
	plan, err := nsjailRootfsPlanFor(sp, be.Chroot, "")
	if err == nil {
		// 茧目录还没分配（Provision 才给）⇒ 这一步**应当**报错（证明「算不出落点就红」）。
		t.Fatalf("没有茧目录时方案应当报错，却成功：%+v", plan)
	}

	inst, err := be.Provision(ctx, sp)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	defer func() {
		if derr := be.Destroy(ctx, inst); derr != nil {
			t.Errorf("Destroy: %v", derr)
		}
	}()

	plan, err = nsjailRootfsPlanFor(sp, be.Chroot, inst.Root)
	if err != nil {
		t.Fatalf("方案算不出来：%v", err)
	}

	repoDir, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	// ① + ②：一条命令里跑两件事（少一次 exec 就少一次「两格其实不是同一环境」的风险）。
	// ★ 探针路径要挑「宿主有、镜像里一定没有」的：本机（容器里）/root 恰好两边都有 ——
	//   拿它当标记会假报泄漏。所以家目录只在它不是 /root 时才算标记（macOS 开发机是
	//   /Users/<名>，那才是要验的东西），另加一枚自造的 sentinel 保证至少有一条硬标记。
	markers := []string{repoDir}
	sentinel := filepath.Join(be.BaseDir, "host-sentinel")
	if err := os.WriteFile(sentinel, []byte("x"), 0o600); err != nil {
		t.Fatalf("造宿主 sentinel 失败：%v", err)
	}
	markers = append(markers, sentinel)
	if home != "" && home != "/" && home != "/root" {
		markers = append(markers, home)
	}
	script := `echo from-image; ls /; ` +
		`for p in "$@"; do if [ -e "$p" ]; then echo "LEAK:$p"; fi; done`
	probeArgv := append([]string{"/bin/sh", "-c", script, "it"}, markers...)
	res, err := be.Exec(ctx, inst, probeArgv, ExecOpts{Timeout: 120 * time.Second})
	if err != nil {
		t.Fatalf("Exec（① 起茧内 /bin/sh）: %v", err)
	}
	if res.RC != 0 {
		t.Fatalf("① 茧内命令退出码 = %d（want 0），stderr=%s", res.RC, res.Stderr)
	}
	out := string(res.Stdout)
	if !strings.Contains(out, "from-image") {
		t.Fatalf("① 没看到茧内输出 from-image：stdout=%q stderr=%q", out, res.Stderr)
	}
	if !strings.Contains(out, "usr") || !strings.Contains(out, "etc") {
		// 镜像根没挂上的典型症状：ls / 是空的（只有我们补的 proc/dev/tmp）。
		t.Fatalf("① `ls /` 不像一个 debian 根（应有 usr/etc 等）：%q", out)
	}
	// 读数留在 -v 里：这是真机验收，读数是唯一能复核的东西（别只写「通过」）。
	t.Logf("① 茧内执行：rc=%d ms=%d stdout=%q stderr=%q", res.RC, res.MS, out, res.Stderr)
	if strings.Contains(out, "LEAK:") {
		t.Fatalf("② 茧内竟能看见宿主路径（换根没生效）：%q", out)
	}
	// 反向对照：把路径换成茧内**确实存在**的那个，探针必须报 LEAK —— 否则「没 LEAK」可能只是
	// 探针本身不工作（一条永远为真的断言不算判据）。
	self, err := be.Exec(ctx, inst, []string{"/bin/sh", "-c", script, "it", "/usr"},
		ExecOpts{Timeout: 120 * time.Second})
	if err != nil || self.RC != 0 {
		t.Fatalf("② 负控 exec 失败：err=%v rc=%d stderr=%s", err, self.RC, self.Stderr)
	}
	if !strings.Contains(string(self.Stdout), "LEAK:/usr") {
		t.Fatalf("② 负控没咬到（探针不会报已存在的路径 ⇒ 上面那条「无 LEAK」不可信）：%q", self.Stdout)
	}

	// ③ + ④：/tmp 可写、/usr 只读、写在别处的新文件要落进 upper。
	probe := `echo tmp-ok > /tmp/probe-from-it && echo TMP_OK || echo TMP_FAIL; ` +
		`echo usr-x > /usr/probe-from-it && echo USR_SUCCEEDED || echo USR_DENIED; ` +
		`echo up-x > /root/probe-from-it && echo UPPER_WRITE_OK || echo UPPER_WRITE_FAIL`
	res2, err := be.Exec(ctx, inst, []string{"/bin/sh", "-c", probe},
		ExecOpts{Timeout: 120 * time.Second})
	if err != nil {
		t.Fatalf("Exec（③④ 写面）: %v", err)
	}
	out2 := string(res2.Stdout)
	if res2.RC != 0 {
		t.Fatalf("③④ 茧内命令退出码 = %d，stdout=%q stderr=%q", res2.RC, out2, res2.Stderr)
	}
	t.Logf("③④ 写面：rc=%d stdout=%q stderr=%q · upper=%s tmp=%s",
		res2.RC, out2, res2.Stderr, plan.Upper, plan.TmpDir)
	if !strings.Contains(out2, "TMP_OK") {
		t.Fatalf("③ 写 /tmp 应当成功：%q（stderr=%q）", out2, res2.Stderr)
	}
	// 壳外也要看见（/tmp 不是 tmpfs：写了什么必须能在宿主上查到）。
	if _, err := os.Stat(filepath.Join(plan.TmpDir, "probe-from-it")); err != nil {
		t.Fatalf("③ 茧内写在 /tmp 的文件没落到宿主 %s：%v", plan.TmpDir, err)
	}
	if strings.Contains(out2, "USR_SUCCEEDED") || !strings.Contains(out2, "USR_DENIED") {
		t.Fatalf("③ 写 /usr 必须失败（系统面只读）：%q", out2)
	}
	if _, err := os.Stat(filepath.Join(plan.Upper, "usr", "probe-from-it")); err == nil {
		t.Fatalf("③ /usr 的写入竟被 copy-up 进了 upper %s", filepath.Join(plan.Upper, "usr"))
	}
	if !strings.Contains(out2, "UPPER_WRITE_OK") {
		t.Fatalf("④ 往可写面（/root）写应当成功：%q（stderr=%q）", out2, res2.Stderr)
	}
	if _, err := os.Stat(filepath.Join(plan.Upper, "root", "probe-from-it")); err != nil {
		t.Fatalf("④ overlay upper 里没出现新文件 %s：%v", filepath.Join(plan.Upper, "root", "probe-from-it"), err)
	}

	// 收尾面：Destroy 之后合并视图必须已经卸掉（否则删目录只会剩个挂着的空壳，
	// 下一次同名目录会被它遮住）。Destroy 由 defer 跑，这里手动跑一遍拿读数（幂等）。
	if err := be.Destroy(ctx, inst); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if nsjailIsMountpoint(plan.Merged) {
		t.Fatalf("Destroy 之后 %s 仍是挂载点（overlay 没卸）", plan.Merged)
	}
}

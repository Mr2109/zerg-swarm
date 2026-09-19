// egg_profile_cache_test.go —— 2026-09-19 ①：卵档案被缓存/陈旧 ⇒ 闸门按旧 peak 放行。
//
// 现象（真机）：盘上档案已更新为 `peak_gtt_gb: 74.567 / peak_mem_gb: 74.882`，而子端日志仍打印
// `双闸门通过: deepseek-v4-flash（… 档案 peak_gtt=14.5 GB peak_mem=15.2 GB）`（旧值）⇒ 按旧值放行；
// `/infer/reload` 只重载注册表、**不重载档案**。
//
// 本文件钉住（档案目录用 ZERG_EGG_PROFILE_DIR 指到 t.TempDir()，**绝不碰真实 home**）：
//
//	① 孵化闸门**每次现读盘上档案**：同一台 Manager，先按 16 GB 放行 → 盘上改成 74.567 GB
//	   （不重启、不 reload）→ 闸门必须读到**新值**；再改成非法值 ⇒ 必须拒孵（fail-closed）；
//	② 静态预检（hatchPrecheckLocked）同样走现读（源码层钉住调用点）；
//	③ RefreshEggProfiles 把缓存按盘上档案**重建**（这就是 /infer/reload 调的那一步）；
//	④ 观测面的 has_profile 读缓存、缓存没有才现读一次；档案被删后刷新 ⇒ 如实 false。
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// profileDirForTest 把档案目录指到临时目录（**绝不写真实 ~/.zerg**）并回它。
func profileDirForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	if got := monitor.EggProfileDir(); got != dir {
		t.Fatalf("档案目录没被环境变量接管（用例会写到真实 home）：%q", got)
	}
	return dir
}

// ① 现读：盘上档案变了，闸门（同进程、同 Manager）必须立刻读到新值。
func TestProfileGate_ReadsProfileFromDiskEachTime(t *testing.T) {
	dir := profileDirForTest(t)
	entry := hatchTestEntry("egg-a")
	m := newEvictTestManager(1, map[string]*subproc{})

	// 1) 初值 16 GB ⇒ 放行，且闸门交出的就是 16
	writeHatchProfile(t, dir, "egg-a", 16, 16)
	m.mu.Lock()
	prof, reject := m.profileGateLocked("egg-a", entry)
	m.mu.Unlock()
	if reject != nil {
		t.Fatalf("档案可用时不该拒孵：%v", reject)
	}
	if prof.PeakGttGb != 16 {
		t.Fatalf("闸门应读到 16 GB，实得 %v", prof.PeakGttGb)
	}

	// 2) 盘上重标定为 74.567 / 74.882（**不 reload、不重启**）⇒ 闸门必须读到新值
	writeHatchProfile(t, dir, "egg-a", 74.567, 74.882)
	m.mu.Lock()
	prof2, reject2 := m.profileGateLocked("egg-a", entry)
	m.mu.Unlock()
	if reject2 != nil {
		t.Fatalf("重标定后的档案仍可用，不该拒孵：%v", reject2)
	}
	if prof2.PeakGttGb != 74.567 || prof2.PeakMemGb != 74.882 {
		t.Fatalf("闸门必须**现读盘上**档案（真机缺陷就是这里拿了旧值 14.5/15.2）：实得 peak_gtt=%v peak_mem=%v",
			prof2.PeakGttGb, prof2.PeakMemGb)
	}

	// 3) 盘上档案变非法（peak_gtt_gb=0）⇒ 必须拒孵（不许拿上一份好档案继续放行）
	writeHatchProfile(t, dir, "egg-a", 0, 74.882)
	m.mu.Lock()
	_, reject3 := m.profileGateLocked("egg-a", entry)
	m.mu.Unlock()
	if reject3 == nil {
		t.Fatal("盘上档案已非法 ⇒ 必须拒孵（fail-closed：不许拿缓存里的旧副本当凭据）")
	}
	if code, _ := reject3["code"].(string); code != "no measured profile" {
		t.Fatalf("拒孵理由应是「无有效实测档案」，实得 %v", reject3)
	}
}

// ② 静态预检（收卵前的那道）也必须现读 —— 源码层钉住调用点，防「只改了一处」。
func TestHatchPrecheck_UsesFreshProfileRead(t *testing.T) {
	src, err := readFileForAssertion("manager.go")
	if err != nil {
		t.Fatalf("读 manager.go 失败: %v", err)
	}
	code := codeOnly(src)
	if !strings.Contains(code, "m.eggProfileFreshLocked(strings.TrimSpace(entry.EggName()))") {
		t.Fatal("hatchPrecheckLocked 的档案读取必须走 eggProfileFreshLocked（现读）；" +
			"改回 monitor.LoadEggProfile/读缓存都会让「重标定后第一次 /load 按新值判」这条失效")
	}
	if strings.Contains(code, "prof, err := monitor.LoadEggProfile(profilePath)") {
		t.Fatal("hatchPrecheckLocked 仍在直接 monitor.LoadEggProfile：① 要求所有孵化判据走唯一读入口")
	}
}

// ③ RefreshEggProfiles（/infer/reload 调它）：缓存按盘上档案重建。
func TestRefreshEggProfiles_RebuildsCacheFromDisk(t *testing.T) {
	dir := profileDirForTest(t)
	m := startLockTestManager(t, "egg-a", "egg-b")

	// 预热缓存（驻留前也可能有人读过：这里直接走现读入口）
	writeHatchProfile(t, dir, "egg-a", 16, 16)
	writeHatchProfile(t, dir, "egg-b", 17, 17)
	for _, id := range []string{"egg-a", "egg-b"} {
		if _, err := m.eggProfileFresh(id); err != nil {
			t.Fatalf("预热档案失败(%s)：%v", id, err)
		}
	}
	if p, ok := m.CachedEggProfile("egg-a"); !ok || p.PeakGttGb != 16 {
		t.Fatalf("预热后缓存应是 16，实得 %+v ok=%v", p.PeakGttGb, ok)
	}

	// 盘上重标定 + 删掉 egg-b 的档案 ⇒ 刷新后：a 变新值、b 进问题清单（如实，不当成功）
	writeHatchProfile(t, dir, "egg-a", 74.567, 74.882)
	if err := os.Remove(filepath.Join(dir, "egg-b.yaml")); err != nil {
		t.Fatal(err)
	}
	ok, problems := m.RefreshEggProfiles()

	if ok != 1 {
		t.Fatalf("应刷新成功 1 枚（egg-a），实得 %d（问题清单 %v）", ok, problems)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "egg-b") {
		t.Fatalf("读不到的卵必须如实进问题清单：%v", problems)
	}
	if p, ok := m.CachedEggProfile("egg-a"); !ok || p.PeakGttGb != 74.567 {
		t.Fatalf("刷新后缓存必须是盘上的新值 74.567，实得 %v ok=%v", p.PeakGttGb, ok)
	}
	if _, ok := m.CachedEggProfile("egg-b"); ok {
		t.Fatal("档案已被删的卵不该留在缓存里（下次现读会把它读回来或如实报错）")
	}
}

// ④ 观测面 has_profile：读缓存、缓存没有才现读一次；档案被删 + 刷新 ⇒ 如实 false。
func TestEggProfileAvailable_CacheThenFreshRead(t *testing.T) {
	dir := profileDirForTest(t)
	m := startLockTestManager(t, "egg-a")

	if m.EggProfileAvailable("egg-a") {
		t.Fatal("档案还没写就不许说 has_profile=true（§8.4：不编造）")
	}
	writeHatchProfile(t, dir, "egg-a", 16, 16)
	if !m.EggProfileAvailable("egg-a") {
		t.Fatal("档案已落盘 ⇒ has_profile 应为 true（缓存没有时现读一次）")
	}
	// 删档 + 刷新缓存（等价于重标定后 reload）⇒ 缓存里没有了、现读也读不到 ⇒ false
	if err := os.Remove(filepath.Join(dir, "egg-a.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, problems := m.RefreshEggProfiles(); len(problems) != 1 {
		t.Fatalf("删档后刷新应报一个问题（如实），实得 %v", problems)
	}
	if m.EggProfileAvailable("egg-a") {
		t.Fatal("档案已删 ⇒ has_profile 必须 false（不许拿旧缓存继续宣称有档案）")
	}
}

// reconcile_test.go — 五格判据（+ 一格护栏）：
//
//	① TestReconcileClean            干净收敛：全一致 ⇒ 零变更、零待人
//	② TestReconcileOrphanDir        有目录无 meta ⇒ 只记待人，真目录/里面的文件一个不动
//	③ TestReconcileGhostMeta        有 meta 无目录 ⇒ 标 gone（墓碑留盘），第二遍不再重写
//	④ TestReconcileRunningDrift     phase=running 但无进程/无端口 ⇒ 降 failed（三小格）
//	⑤ TestReconcileIdempotent       连跑两次：第二遍零变更、meta 字节逐份不变
//	⑥ TestReconcileBrokenAndMissing 护栏：meta 损坏只报不动盘；茧根不存在 ⇒ ErrNotFound；tmp 残渣不计入四类
package cocoon

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ── 夹具（mustSave / mustLoad 复用 meta_test.go） ──────────────────────

// mkCocoon — 造一个茧目录：<root>/<id>/meta.json，phase 走 SetPhase 前进写入。
func mkCocoon(t *testing.T, root, id string, phase Phase, pid int, ports map[int]int) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建茧目录 %s 失败: %v", dir, err)
	}
	m := New(id, "egg-test", "A2/测试")
	m.PID = pid
	m.Ports = ports
	if err := m.SetPhase(phase); err != nil { // created → phase：只许前进
		t.Fatalf("SetPhase(%s) 失败: %v", phase, err)
	}
	mustSave(t, MetaPath(dir), m)
	return dir
}

// mkFlat — 造一份平铺 meta（<root>/<id>.meta.json）：目录不在、meta 还在的幽灵面。
func mkFlat(t *testing.T, root, id string, phase Phase) string {
	t.Helper()
	m := New(id, "egg-test", "A2/测试")
	if err := m.SetPhase(phase); err != nil {
		t.Fatalf("SetPhase(%s) 失败: %v", phase, err)
	}
	path := FlatMetaPath(root, id)
	mustSave(t, path, m)
	return path
}

// deadPID — 起一个立刻退出的子进程，拿到一个**确实不在**的 pid。
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("起探针进程失败: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("等探针进程失败: %v", err)
	}
	if pidAlive(pid) {
		t.Skipf("pid %d 已被复用，这一格判不了", pid)
	}
	return pid
}

// snapshotMetas — 扫 root 下所有 meta 文件的字节（幂等判据：字节不变 = 没重写过）。
func snapshotMetas(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatalf("快照 %s 失败: %v", root, err)
	}
	return out
}

// ── ① 干净收敛 ────────────────────────────────────────────────────────

func TestReconcileClean(t *testing.T) {
	root := t.TempDir()
	mkCocoon(t, root, "c-run", PhaseRunning, os.Getpid(), map[int]int{8080: 38080})
	mkCocoon(t, root, "c-created", PhaseCreated, 0, nil)
	mkCocoon(t, root, "c-del", PhaseDeleting, 0, nil)

	rep, err := Reconcile(root)
	if err != nil {
		t.Fatalf("Reconcile 失败: %v", err)
	}
	if got := len(rep.Consistent); got != 3 {
		t.Fatalf("一致条数 = %d，want 3\n%s", got, rep.String())
	}
	if n := len(rep.OrphanDirs) + len(rep.GhostMetas) + len(rep.PhaseDrift) + len(rep.Broken); n != 0 {
		t.Fatalf("不该有异常条目，got %d\n%s", n, rep.String())
	}
	if rep.Changed() != 0 {
		t.Fatalf("干净面不该写盘，Changed = %d", rep.Changed())
	}
	if rep.PendingHuman() != 0 {
		t.Fatalf("干净面不该待人，PendingHuman = %d", rep.PendingHuman())
	}
	out := rep.String()
	for _, want := range []string{"一致 3", "孤儿目录 0", "幽灵 meta 0", "phase 不符 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("报告缺计数 %q:\n%s", want, out)
		}
	}
	for _, id := range []string{"c-run", "c-created", "c-del"} {
		if !strings.Contains(out, id) {
			t.Fatalf("报告缺逐条 id %q:\n%s", id, out)
		}
	}
}

// ── ② 孤儿目录 ────────────────────────────────────────────────────────

func TestReconcileOrphanDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "c-orphan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	keep := filepath.Join(dir, "work-data.bin")
	if err := os.WriteFile(keep, []byte("用户的活不能丢"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	rep, err := Reconcile(root)
	if err != nil {
		t.Fatalf("Reconcile 失败: %v", err)
	}
	if len(rep.OrphanDirs) != 1 {
		t.Fatalf("孤儿条数 = %d，want 1\n%s", len(rep.OrphanDirs), rep.String())
	}
	f := rep.OrphanDirs[0]
	if f.ID != "c-orphan" || f.Path != dir {
		t.Fatalf("孤儿条目不对: %+v", f)
	}
	if !f.PendingHuman || f.Changed {
		t.Fatalf("孤儿必须「只记待人、零写盘」: %+v", f)
	}
	if rep.Changed() != 0 {
		t.Fatalf("孤儿不该引发写盘，Changed = %d", rep.Changed())
	}

	// 真目录、里面用户的文件、以及"不凭空造 meta"三件事都要成立。
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("真目录被动了: %v", err)
	}
	if b, err := os.ReadFile(keep); err != nil || len(b) == 0 {
		t.Fatalf("目录里的文件被动了: %v", err)
	}
	if _, err := os.Stat(MetaPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("不许凭空给孤儿目录造 meta.json（err=%v）", err)
	}
	if !strings.Contains(rep.String(), "c-orphan") {
		t.Fatalf("报告缺孤儿 id:\n%s", rep.String())
	}
}

// ── ③ 幽灵 meta ───────────────────────────────────────────────────────

func TestReconcileGhostMeta(t *testing.T) {
	root := t.TempDir()
	path := mkFlat(t, root, "c-ghost", PhaseDeleting)

	rep, err := Reconcile(root)
	if err != nil {
		t.Fatalf("Reconcile 失败: %v", err)
	}
	if len(rep.GhostMetas) != 1 {
		t.Fatalf("幽灵条数 = %d，want 1\n%s", len(rep.GhostMetas), rep.String())
	}
	f := rep.GhostMetas[0]
	if f.ID != "c-ghost" || !f.Changed || f.Next != PhaseGone || f.Phase != PhaseDeleting {
		t.Fatalf("幽灵处置不对: %+v", f)
	}
	if got := mustLoad(t, path).Phase; got != PhaseGone {
		t.Fatalf("盘上 phase = %s，want gone", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("墓碑 meta 不该被删（删除权在 A5）: %v", err)
	}
	if !strings.Contains(rep.String(), "c-ghost") || !strings.Contains(rep.String(), "gone") {
		t.Fatalf("报告缺幽灵原因:\n%s", rep.String())
	}

	// 第二遍：已 gone ⇒ 只报待人、零写盘（否则每跑一次都重写同一份 JSON）。
	before, _ := os.ReadFile(path)
	rep2, err := Reconcile(root)
	if err != nil {
		t.Fatalf("第二遍 Reconcile 失败: %v", err)
	}
	if rep2.Changed() != 0 {
		t.Fatalf("第二遍仍写盘，Changed = %d\n%s", rep2.Changed(), rep2.String())
	}
	if len(rep2.GhostMetas) != 1 || !rep2.GhostMetas[0].PendingHuman {
		t.Fatalf("已 gone 的墓碑应「报待人」: %+v", rep2.GhostMetas)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatalf("第二遍重写了墓碑（字节变了）")
	}
}

// ── ④ phase 与实况不符 ────────────────────────────────────────────────

func TestReconcileRunningDrift(t *testing.T) {
	dead := deadPID(t)
	cases := []struct {
		name  string
		pid   int
		ports map[int]int
		want  string // 原因里必须出现的关键词
	}{
		{"没有 pid", 0, map[int]int{8080: 38080}, "pid=0"},
		{"进程不在", dead, map[int]int{8080: 38080}, "已不在"},
		{"没有端口", os.Getpid(), nil, "没有端口"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := mkCocoon(t, root, "c-drift", PhaseRunning, tc.pid, tc.ports)

			rep, err := Reconcile(root)
			if err != nil {
				t.Fatalf("Reconcile 失败: %v", err)
			}
			if len(rep.PhaseDrift) != 1 || len(rep.Consistent) != 0 {
				t.Fatalf("应恰好一条 drift，got drift=%d consistent=%d\n%s",
					len(rep.PhaseDrift), len(rep.Consistent), rep.String())
			}
			f := rep.PhaseDrift[0]
			if !f.Changed || f.Next != PhaseFailed || f.Phase != PhaseRunning {
				t.Fatalf("drift 处置不对: %+v", f)
			}
			if !strings.Contains(f.Reason, tc.want) {
				t.Fatalf("原因没点明病灶（want 含 %q）: %s", tc.want, f.Reason)
			}
			if !strings.Contains(f.Reason, "failed") {
				t.Fatalf("原因必须写明落到 failed: %s", f.Reason)
			}
			if got := mustLoad(t, MetaPath(dir)).Phase; got != PhaseFailed {
				t.Fatalf("盘上 phase = %s，want failed", got)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Fatalf("降级不该动目录: %v", err)
			}

			// 第二遍收敛：failed 不是 running，再跑不会继续降级（也不会自动重试）。
			rep2, err := Reconcile(root)
			if err != nil {
				t.Fatalf("第二遍 Reconcile 失败: %v", err)
			}
			if rep2.Changed() != 0 || len(rep2.PhaseDrift) != 0 {
				t.Fatalf("降级后应收敛，got Changed=%d drift=%d", rep2.Changed(), len(rep2.PhaseDrift))
			}
			// 收敛后归「一致」：failed 是终态，不是待处置的 drift。
			if len(rep2.Consistent) != 1 {
				t.Fatalf("failed 应收敛为一致条目，got %d\n%s", len(rep2.Consistent), rep2.String())
			}
			// 且盘上 epoch 未被动过：对账**不重试**（重试只走 ResetForRetry）。
			if got := mustLoad(t, MetaPath(dir)).Epoch; got != 0 {
				t.Fatalf("对账不该改 epoch，got %d", got)
			}
		})
	}
}

// ── ⑤ 幂等：连跑两次，第二遍零变更 ────────────────────────────────────

func TestReconcileIdempotent(t *testing.T) {
	root := t.TempDir()
	mkCocoon(t, root, "c-ok", PhaseRunning, os.Getpid(), map[int]int{8080: 38080}) // 一致
	if err := os.MkdirAll(filepath.Join(root, "c-orphan"), 0o755); err != nil {    // 孤儿
		t.Fatalf("建孤儿目录失败: %v", err)
	}
	mkFlat(t, root, "c-ghost", PhaseDeleting)                               // 幽灵
	mkCocoon(t, root, "c-drift", PhaseRunning, 0, map[int]int{8080: 38080}) // 无进程

	rep1, err := Reconcile(root)
	if err != nil {
		t.Fatalf("第一遍 Reconcile 失败: %v", err)
	}
	if rep1.Changed() == 0 {
		t.Fatalf("第一遍应处置幽灵+drift（Changed>0），实际 0\n%s", rep1.String())
	}
	if len(rep1.Consistent) != 1 || len(rep1.OrphanDirs) != 1 ||
		len(rep1.GhostMetas) != 1 || len(rep1.PhaseDrift) != 1 {
		t.Fatalf("第一遍四类计数不对:\n%s", rep1.String())
	}
	snap1 := snapshotMetas(t, root)
	t.Logf("第一遍报告：\n%s", rep1.String())

	rep2, err := Reconcile(root)
	if err != nil {
		t.Fatalf("第二遍 Reconcile 失败: %v", err)
	}
	if rep2.Changed() != 0 {
		t.Fatalf("第二遍必须零变更，Changed = %d\n%s", rep2.Changed(), rep2.String())
	}
	snap2 := snapshotMetas(t, root)
	if !reflect.DeepEqual(snap1, snap2) {
		for k, v := range snap2 {
			if !bytes.Equal(snap1[k], v) {
				t.Fatalf("第二遍重写了 %s:\n before=%s\n after=%s", k, snap1[k], v)
			}
		}
		t.Fatalf("第二遍的 meta 集合变了: before=%d 份 after=%d 份", len(snap1), len(snap2))
	}
	// 收敛后的归类：drift 那条已落 failed ⇒ 第二遍归"一致"（c-ok + c-drift = 2）；
	// 幽灵仍在，但已是 settled（报待人、零写盘）；孤儿两遍都待人。
	if len(rep2.PhaseDrift) != 0 {
		t.Fatalf("drift 应已收敛，got %d\n%s", len(rep2.PhaseDrift), rep2.String())
	}
	if len(rep2.Consistent) != 2 {
		t.Fatalf("收敛后一致条数 = %d，want 2（c-ok + c-drift 的 failed）\n%s",
			len(rep2.Consistent), rep2.String())
	}
	if len(rep2.OrphanDirs) != 1 || !rep2.OrphanDirs[0].PendingHuman {
		t.Fatalf("孤儿目录两遍都该待人: %+v", rep2.OrphanDirs)
	}
}

// ── ⑥ 护栏：meta 损坏 / 茧根不存在 / tmp 残渣 ─────────────────────────

func TestReconcileBrokenAndMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "c-bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	badPath := MetaPath(dir)
	bad := []byte("{ 这不是 JSON")
	if err := os.WriteFile(badPath, bad, 0o644); err != nil {
		t.Fatalf("写坏 meta 失败: %v", err)
	}
	residue := filepath.Join(root, ".meta-123456.tmp")
	if err := os.WriteFile(residue, []byte("{}"), 0o644); err != nil {
		t.Fatalf("写 tmp 残渣失败: %v", err)
	}

	rep, err := Reconcile(root)
	if err != nil {
		t.Fatalf("有条目损坏不该整体失败: %v", err)
	}
	if len(rep.Broken) != 1 || rep.Broken[0].ID != "c-bad" || !rep.Broken[0].PendingHuman {
		t.Fatalf("损坏 meta 归类不对: %+v", rep.Broken)
	}
	if rep.Changed() != 0 {
		t.Fatalf("损坏 meta 不该被写，Changed = %d", rep.Changed())
	}
	if got, _ := os.ReadFile(badPath); !bytes.Equal(got, bad) {
		t.Fatalf("损坏 meta 被改写了: %q", got)
	}
	if len(rep.Residue) != 1 || rep.Residue[0] != residue {
		t.Fatalf("tmp 残渣记录不对: %v", rep.Residue)
	}
	if _, err := os.Stat(residue); err != nil {
		t.Fatalf("tmp 残渣不该被删（删权在 A5）: %v", err)
	}

	if _, err := Reconcile(filepath.Join(root, "不存在")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("茧根不存在应返回 ErrNotFound，got %v", err)
	}
}

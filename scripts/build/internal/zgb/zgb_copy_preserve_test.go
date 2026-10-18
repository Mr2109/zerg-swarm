package zgb

// 判据：CopyPreserve 的**临时件名必须逐调用唯一**，且 dst 不合法时要**点名目标路径**。
//
// 来历（GAP-20261018-26）：`make build` 曾必红在 UI 一步，逐字
// `rename <…>/zerg-wall.tmp-zergbuild <…>/zerg-wall: no such file or directory` ＋
// `exit status 2`（zerg-build 侧）/ rc=1（build-all 侧）。真因不是路径拼错，而是
// `tmp := dst + ".tmp-zergbuild"` 这一条**固定临时名**：同一 dst 的写者（两趟 build 并行、
// 或同批多个拷贝点）共用同一个临时件 ⇒ 甲 rename 走后，乙的 chmod/rename 当场 ENOENT。
// 本条判据把这个机理钉住：单一临时名 ⇒ 并发格必红（实测改前 16 路里 14–15 路失败）。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// copyPreserveTestSrc 造一枚"像二进制"的源件（含 0 字节与非 ASCII，避免文本捷径掩盖差异）。
func copyPreserveTestSrc(t *testing.T, dir string, n int) string {
	t.Helper()
	p := filepath.Join(dir, "zerg-wall")
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	if err := os.WriteFile(p, b, 0o755); err != nil {
		t.Fatalf("造源件失败：%v", err)
	}
	return p
}

func copyPreserveTestLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	var left []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.Contains(d.Name(), ".tmp-zergbuild") {
			left = append(left, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("列目录失败：%v", err)
	}
	return left
}

// 正控：字节逐同 · 可执行位保留 · 收尾不留临时件。
func TestCopyPreserveKeepsBytesAndMode(t *testing.T) {
	dir := t.TempDir()
	src := copyPreserveTestSrc(t, dir, 128*1024)
	dst := filepath.Join(dir, "bin", "zerg-wall")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("建 dst 目录失败：%v", err)
	}
	if err := CopyPreserve(src, dst); err != nil {
		t.Fatalf("正控：拷贝失败 %v", err)
	}
	want, _ := os.ReadFile(src)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读 dst 失败：%v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("正控：内容不同（%d vs %d 字节）", len(got), len(want))
	}
	fi, err := os.Stat(dst)
	if err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("正控：可执行位没保留（mode=%v err=%v）", fi.Mode(), err)
	}
	if left := copyPreserveTestLeftovers(t, dir); len(left) > 0 {
		t.Fatalf("正控：收尾残留临时件 %v", left)
	}
}

// ★ 本缺口的牙：同一 dst 上 16 路并发写者，必须**各路都成功**且终件逐字节同。
// 负控（改前态 `tmp := dst + ".tmp-zergbuild"`）：多路互相 rename/chmod 掉对方的临时件 ⇒
// 逐字 `rename …: no such file or directory`，本格红。
func TestCopyPreserveConcurrentSameDst(t *testing.T) {
	dir := t.TempDir()
	src := copyPreserveTestSrc(t, dir, 256*1024)
	want, _ := os.ReadFile(src)
	dst := filepath.Join(dir, "bin", "zerg-wall")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("建 dst 目录失败：%v", err)
	}
	const writers, rounds = 16, 5
	for r := 0; r < rounds; r++ {
		var wg sync.WaitGroup
		errs := make([]error, writers)
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = CopyPreserve(src, dst)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("并发格：第 %d 轮第 %d 路失败（临时件名被同 dst 的别的写者踩掉？）：%v", r, i, err)
			}
		}
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("并发格：第 %d 轮读 dst 失败：%v", r, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("并发格：第 %d 轮终件内容不同（%d vs %d 字节）", r, len(got), len(want))
		}
	}
	if left := copyPreserveTestLeftovers(t, dir); len(left) > 0 {
		t.Fatalf("并发格：收尾残留临时件 %v", left)
	}
}

// 负控：dst 多一段不存在的前缀（`rg/bin/zerg-wall`）⇒ 必须**红**且报错里**点名该 dst**，
// 不产出半成品、不留临时件。改前改后都要红 —— 本条钉的是"报错看得懂"，不是"能拷"。
func TestCopyPreserveBadDstPrefixIsNamed(t *testing.T) {
	dir := t.TempDir()
	src := copyPreserveTestSrc(t, dir, 4096)
	bad := filepath.Join(dir, "rg", "bin", "zerg-wall") // rg/ 父目录不存在
	err := CopyPreserve(src, bad)
	if err == nil {
		t.Fatalf("负控：dst 父目录不存在却报成功（%s）", bad)
	}
	if !strings.Contains(err.Error(), bad) {
		t.Fatalf("负控：报错没点名目标 dst（%s）：%v", bad, err)
	}
	if _, statErr := os.Stat(bad); statErr == nil {
		t.Fatalf("负控：不该产出 %s", bad)
	}
	if left := copyPreserveTestLeftovers(t, dir); len(left) > 0 {
		t.Fatalf("负控：残留临时件 %v", left)
	}
}

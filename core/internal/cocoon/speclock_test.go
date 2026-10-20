// speclock_test.go — 契约哈希锁（判据 9 的库半 · `speclock.go`）的判据件。
//
// 四格（每格都带负控 —— 哑化对应判据即红）：
//
//	① 算得对：`LockSpec` 与 `crypto/sha256` 独立算的结果逐字相同 · 空字节必拒 · 一位之差必不同。
//	② 判得对：一致 ⇒ nil；改一行 ⇒ `ErrSpecDrift`；读不到 ⇒ `ErrSpecUnreadable`；
//	   没锁 ⇒ `ErrSpecNoLock`；0 字节 / 锁值形状非法 ⇒ 按漂移拒（fail-closed）。
//	③ **错误里只出前 8 位**：期望与实得并列印，但完整 64 位一个都不许出现在错误串里。
//	④ **不写盘**：比对（含三种失败档）前后，整棵树的件集合、内容哈希、mtime 逐件相同。
package cocoon

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const specLockBody = `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["uvicorn", "app:api"]
`

// slWrite — 落一份契约，返回路径。
func slWrite(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, SpecFileName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写契约失败: %v", err)
	}
	return p
}

// ── ① 算得对 ────────────────────────────────────────────────────────────

func TestSpecLockMatchesSHA256(t *testing.T) {
	got, err := LockSpec([]byte(specLockBody))
	if err != nil {
		t.Fatalf("LockSpec 失败: %v", err)
	}
	sum := sha256.Sum256([]byte(specLockBody))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("LockSpec 与 sha256 不符：得 %q 要 %q", got, want)
	}
	if len(got) != 64 || strings.ToLower(got) != got {
		t.Fatalf("锁值应是 64 位小写 hex，得 %q（%d 位）", got, len(got))
	}
	// 同字节 ⇒ 同值；一位之差 ⇒ 必不同（否则「漂移」判据是哑的）。
	again, _ := LockSpec([]byte(specLockBody))
	if again != got {
		t.Fatalf("同一份字节两次算出的锁值不同：%q / %q", got, again)
	}
	other, _ := LockSpec([]byte(strings.Replace(specLockBody, `"shop"`, `"shop2"`, 1)))
	if other == got {
		t.Fatal("改了一个字节，锁值却没变（漂移判据会全哑）")
	}
	// 空字节 ⇒ 必拒（不给 e3b0c442… 那种「看起来合法」的锁）。
	if _, err := LockSpec(nil); !errors.Is(err, ErrSpecEmpty) {
		t.Fatalf("空字节应回 ErrSpecEmpty，得 %v", err)
	}
	if _, err := LockSpec([]byte{}); !errors.Is(err, ErrSpecEmpty) {
		t.Fatalf("0 长度切片应回 ErrSpecEmpty，得 %v", err)
	}
}

// ── ② 判得对 ────────────────────────────────────────────────────────────

func TestCheckSpecVerdicts(t *testing.T) {
	dir := t.TempDir()
	p := slWrite(t, dir, specLockBody)
	lock, err := LockSpec([]byte(specLockBody))
	if err != nil {
		t.Fatal(err)
	}

	// 一致 ⇒ nil。
	if err := CheckSpec(p, lock); err != nil {
		t.Fatalf("一致时应回 nil，得 %v", err)
	}
	// 大小写不同的锁值仍是同一个锁（规整后比）。
	if err := CheckSpec(p, strings.ToUpper(lock)); err != nil {
		t.Fatalf("大写锁值应照样一致，得 %v", err)
	}

	// 改一行 ⇒ 漂移。
	modified := strings.Replace(specLockBody, `"shop"`, `"shop2"`, 1)
	slWrite(t, dir, modified)
	err = CheckSpec(p, lock)
	if !errors.Is(err, ErrSpecDrift) {
		t.Fatalf("改一行后应回 ErrSpecDrift，得 %v", err)
	}
	if errors.Is(err, ErrSpecUnreadable) {
		t.Fatalf("漂移被误判成读不到: %v", err)
	}

	// 注释/空白也算字节（②口径：不按结构比）。
	slWrite(t, dir, "# 改了一行注释\n"+specLockBody)
	if err := CheckSpec(p, lock); !errors.Is(err, ErrSpecDrift) {
		t.Fatalf("只改注释也应算漂移（本轮按原始字节锁），得 %v", err)
	}
	slWrite(t, dir, specLockBody)

	// 没锁 ⇒ ErrSpecNoLock（不许当成一致）。
	if err := CheckSpec(p, ""); !errors.Is(err, ErrSpecNoLock) {
		t.Fatalf("空锁定值应回 ErrSpecNoLock，得 %v", err)
	}
	if err := CheckSpec(p, "   "); !errors.Is(err, ErrSpecNoLock) {
		t.Fatalf("空白锁定值应回 ErrSpecNoLock，得 %v", err)
	}

	// 锁值形状非法 ⇒ 按漂移拒（锁坏了不比对 ≈ 没锁，fail-closed）。
	for _, bad := range []string{"deadbeef", strings.Repeat("z", 64), lock[:63], lock + "0"} {
		if err := CheckSpec(p, bad); !errors.Is(err, ErrSpecDrift) {
			t.Fatalf("非法锁值 %q 应回 ErrSpecDrift，得 %v", bad, err)
		}
	}

	// 读不到 ⇒ ErrSpecUnreadable（与漂移分开：一处去看文件，一处去看路径）。
	if err := CheckSpec(filepath.Join(dir, "nope", SpecFileName), lock); !errors.Is(err, ErrSpecUnreadable) {
		t.Fatalf("缺文件应回 ErrSpecUnreadable，得 %v", err)
	}
	// 0 字节契约 ⇒ 漂移（不是「空 == e3b0c442…」）。
	slWrite(t, dir, "")
	if err := CheckSpec(p, lock); !errors.Is(err, ErrSpecDrift) {
		t.Fatalf("0 字节契约应回 ErrSpecDrift，得 %v", err)
	}
	empty, _ := LockSpec([]byte(" "))
	if err := CheckSpec(p, empty); err == nil {
		t.Fatal("0 字节契约竟判成一致（空文件当成了锁）")
	}
}

// ── ③ 错误面只出前 8 位 ─────────────────────────────────────────────────

func TestCheckSpecErrorShowsOnlyShortPrefix(t *testing.T) {
	dir := t.TempDir()
	p := slWrite(t, dir, specLockBody)
	lock, _ := LockSpec([]byte(specLockBody))
	slWrite(t, dir, strings.Replace(specLockBody, `"shop"`, `"shop2"`, 1))

	st := ProbeSpec(p, lock)
	if !errors.Is(st.Err, ErrSpecDrift) {
		t.Fatalf("应判漂移，得 %v", st.Err)
	}
	msg := st.Err.Error()
	// 期望与实得的前 8 位都在。
	if !strings.Contains(msg, SpecShort(lock)) {
		t.Fatalf("错误串没印期望值的前 8 位（%s）：%s", SpecShort(lock), msg)
	}
	if st.Current == "" || !strings.Contains(msg, SpecShort(st.Current)) {
		t.Fatalf("错误串没印实得值的前 8 位（%s · current=%q）：%s",
			SpecShort(st.Current), st.Current, msg)
	}
	// ★ 完整 64 位一个都不许出现（指纹不进日志/stderr）。
	for _, full := range []string{lock, st.Current} {
		if strings.Contains(msg, full) {
			t.Fatalf("错误串里出现了完整哈希 %q：%s", full, msg)
		}
	}
	if len(msg) > 512 {
		t.Fatalf("错误串过长了（%d 字节），多半是把正文/全文带进去了：%s", len(msg), msg)
	}
	// SpecShort 的边界：短串原样回、长串截 8 位。
	if got := SpecShort("ab"); got != "ab" {
		t.Fatalf("短哈希应原样回，得 %q", got)
	}
	if got := SpecShort("  ABCDEF0123  "); got != "abcdef01" {
		t.Fatalf("SpecShort 应去空白+转小写+截 8 位，得 %q", got)
	}
}

// ── ④ 不写盘 ────────────────────────────────────────────────────────────

func TestCheckSpecWritesNothing(t *testing.T) {
	dir := t.TempDir()
	p := slWrite(t, dir, specLockBody)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, _ := LockSpec([]byte(specLockBody))

	snap := func() map[string]string {
		out := map[string]string{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读目录失败: %v", err)
		}
		var walk func(base string)
		walk = func(base string) {
			es, err := os.ReadDir(base)
			if err != nil {
				t.Fatalf("读 %s 失败: %v", base, err)
			}
			for _, e := range es {
				full := filepath.Join(base, e.Name())
				fi, err := os.Stat(full)
				if err != nil {
					t.Fatalf("stat %s 失败: %v", full, err)
				}
				if fi.IsDir() {
					out[e.Name()+"/"] = "dir " + fi.ModTime().UTC().Format(time.RFC3339Nano)
					walk(full)
					continue
				}
				b, err := os.ReadFile(full)
				if err != nil {
					t.Fatalf("读 %s 失败: %v", full, err)
				}
				sum := sha256.Sum256(b)
				out[e.Name()] = hex.EncodeToString(sum[:]) + " " + fi.ModTime().UTC().Format(time.RFC3339Nano)
			}
		}
		walk(dir)
		_ = entries
		return out
	}

	before := snap()
	// 走四种档：一致 · 漂移 · 没锁 · 读不到。
	_ = CheckSpec(p, lock)
	slWrite(t, dir, strings.Replace(specLockBody, `"shop"`, `"shop2"`, 1))
	before = snap()
	_ = CheckSpec(p, lock)
	_ = CheckSpec(p, "")
	_ = CheckSpec(filepath.Join(dir, "nope"), lock)
	_ = ProbeSpec(p, lock)
	after := snap()

	if len(before) != len(after) {
		t.Fatalf("件数变了（比对写了盘）：before=%v after=%v", before, after)
	}
	for k, v := range after {
		if before[k] != v {
			t.Errorf("比对改动了 %q（before=%q after=%q）", k, before[k], v)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			t.Errorf("比对比对后少了 %q", k)
		}
	}
	// 不许凭空多出锁件。
	if _, err := os.Stat(p + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("比对建出了锁件（%s）：%v", p+".lock", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".speclock")); !os.IsNotExist(err) {
		t.Fatalf("比对建出了第二个锁落点（.speclock）：%v", err)
	}
}

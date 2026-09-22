// cli_archive_test.go —— `archive` 族的判据机检（缺口 `G-07` · 波① `T1`）。
//
// 判据（`任务清单-缺口收口-20260923.md` §`T1` 原样三条 · 逐条落成断言）：
//
//	① `archive hash <目录>…` 一进程吃 N 件 ⇒ **输出行数 = 件数**、每行 `sha256␣␣路径`、
//	   与 `shasum -a 256` **逐字相同**（本件用**独立实现**（标准库 `crypto/sha256` 逐件算）
//	   对拍 ≥100 件；与 `shasum` 二进制的对拍走**进程外**那一层（真机现跑，回执里贴原样输出））。
//	② 出三件套（RFC 8493 BagIt）⇒ **清单里逐件 digest 与件本体重算相等**；且**负控**：
//	   改一个字节 / 抹掉清单里一条 / 盘上多出未登记件 ⇒ 判红（rc=1）。
//	③ `--dry-run` 先行：干跑 rc=0 且**零副作用**（落点目录不建 · 载荷件 `sha256` 不变）。
//
// 成对负控：每一条正控旁边都有一条必红的负控（件数不对 / 清单少一条 / 字节改了 / 不是袋）。
package main_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// archiveFixture 造一批合成件：n 件 + 一条子目录件；返回 (载荷目录, 逐件相对路径)。
func archiveFixture(t *testing.T, n int) (string, []string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "载荷")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("建夹具失败：%v", err)
	}
	rels := []string{}
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("件-%03d.bin", i)
		body := []byte(strings.Repeat(fmt.Sprintf("合成-%d;", i), i%17+1))
		if err := os.WriteFile(filepath.Join(dir, rel), body, 0o644); err != nil {
			t.Fatalf("写夹具失败：%v", err)
		}
		rels = append(rels, rel)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "子件.txt"), []byte("子目录里的一件\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	sort.Strings(rels)
	return dir, rels
}

// sha256OfFile 独立实现（判据①的对拍基准）。
func sha256OfFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", p, err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// TestArchiveHashBatchAgainstIndependentImpl —— 判据①：一进程吃 N 件 ⇒ 行数 = 件数、
// 逐行 `sha256␣␣路径`（**无表头**）、digest 与独立实现逐字相同。N ≥ 100。
func TestArchiveHashBatchAgainstIndependentImpl(t *testing.T) {
	const n = 137
	dir, rels := archiveFixture(t, n)
	want := n // `archive hash <目录>` = **顶层逐件**（非递归）⇒ 与 `shasum -a 256 <目录>/*` 同口径
	rc, out, errb := runCapture("archive", "hash", dir)
	if rc != 0 {
		t.Fatalf("archive hash rc=%d（要 0）· stderr=%s", rc, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != want {
		t.Fatalf("输出行数 %d ≠ 件数 %d（判据①：行数 = 件数）", len(lines), want)
	}
	if strings.Contains(out, "\t") {
		t.Errorf("输出里出现了制表符（判据①要的是 `sha256␣␣路径` 裸行，与 `shasum -a 256` 逐字同）：%q", out)
	}
	got := map[string]string{}
	for _, l := range lines {
		parts := strings.SplitN(l, "  ", 2)
		if len(parts) != 2 || len(parts[0]) != 64 {
			t.Fatalf("行不是 `sha256␣␣路径` 形状：%q", l)
		}
		got[parts[1]] = parts[0]
	}
	wantPaths := append([]string{}, rels...)
	sort.Strings(wantPaths)
	for _, rel := range wantPaths {
		p := filepath.Join(dir, rel)
		if got[p] != sha256OfFile(t, p) {
			t.Errorf("%s 的 digest 与独立实现不同：命令面 %q ≠ 标准库 %q", p, got[p], sha256OfFile(t, p))
		}
	}
	t.Logf("一进程吃 %d 件 · 行数 %d · 与独立实现逐件相同 ✓", want, len(lines))
}

// TestArchiveManifestDryRunZeroSideEffect —— 判据③：干跑 rc=0 · 落点不建 · 载荷 sha256 不变。
func TestArchiveManifestDryRunZeroSideEffect(t *testing.T) {
	dir, rels := archiveFixture(t, 5)
	before := map[string]string{}
	for _, rel := range rels {
		before[rel] = sha256OfFile(t, filepath.Join(dir, rel))
	}
	bag := filepath.Join(t.TempDir(), "袋")
	rc, out, errb := runCapture("archive", "manifest", dir, "--out", bag, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "计划件") || !strings.Contains(errb, "零副作用") {
		t.Errorf("干跑的人面/机面话不全：stdout=%s stderr=%s", out, errb)
	}
	if !strings.Contains(out, "data/") || !strings.Contains(out, rels[0]) {
		t.Errorf("计划件里没有**逐件清单**（判据③：干跑先出清单）：%s", out)
	}
	if _, err := os.Stat(bag); !os.IsNotExist(err) {
		t.Errorf("干跑竟建了落点 %s（零副作用不成立）", bag)
	}
	for rel, sum := range before {
		if got := sha256OfFile(t, filepath.Join(dir, rel)); got != sum {
			t.Errorf("干跑改了载荷 %s 的 sha256：%s → %s（零副作用不成立）", rel, sum, got)
		}
	}
}

// TestArchiveManifestAndVerify —— 判据②：三件套形状 + 逐件 digest 与件本体重算相等（正控），
// 以及三条**成对负控**（改一字节 / 抹清单一条 / 多出未登记件）必须判红。
func TestArchiveManifestAndVerify(t *testing.T) {
	dir, rels := archiveFixture(t, 9)
	bag := filepath.Join(t.TempDir(), "袋")

	// 负控第 0 步：缺 --yes ⇒ 2（fail-closed），且**不建落点**。
	rc, _, _ := runCapture("archive", "manifest", dir, "--out", bag)
	if rc != 2 {
		t.Fatalf("缺 --yes 的 rc=%d（要 2 · 写面 fail-closed）", rc)
	}
	if _, err := os.Stat(bag); !os.IsNotExist(err) {
		t.Fatalf("缺 --yes 时竟建了落点 %s", bag)
	}

	rc, out, errb := runCapture("archive", "manifest", dir, "--out", bag, "--yes")
	if rc != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "bagit.txt") || !strings.Contains(out, "tagmanifest-sha256.txt") {
		t.Errorf("三件套的话不全：%s", out)
	}
	for _, p := range []string{"bagit.txt", "manifest-sha256.txt", "tagmanifest-sha256.txt", "data"} {
		if _, err := os.Stat(filepath.Join(bag, p)); err != nil {
			t.Fatalf("三件套缺 %s：%v", p, err)
		}
	}
	// 正控：每一条清单行的 digest == 件本体重算。
	manBytes, err := os.ReadFile(filepath.Join(bag, "manifest-sha256.txt"))
	if err != nil {
		t.Fatalf("读清单失败：%v", err)
	}
	manLines := strings.Split(strings.TrimRight(string(manBytes), "\n"), "\n")
	if len(manLines) != len(rels)+1 {
		t.Fatalf("清单行数 %d ≠ 载荷件数 %d", len(manLines), len(rels)+1)
	}
	for _, l := range manLines {
		parts := strings.SplitN(l, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("清单行不是 `digest␣␣路径` 形状：%q", l)
		}
		want := sha256OfFile(t, filepath.Join(bag, filepath.FromSlash(parts[1])))
		if parts[0] != want {
			t.Errorf("清单 %s 的 digest 与件本体重算不同：%s ≠ %s", parts[1], parts[0], want)
		}
	}
	// 正控：verify ⇒ 0。
	if rc, _, errb := runCapture("archive", "verify", bag); rc != 0 {
		t.Fatalf("verify rc=%d（要 0）· stderr=%s", rc, errb)
	}

	// 负控①：载荷改一字节 ⇒ 判红（rc=1）。
	target := filepath.Join(bag, "data", rels[0])
	orig, _ := os.ReadFile(target)
	if err := os.WriteFile(target, append(orig, 'X'), 0o644); err != nil {
		t.Fatalf("改件失败：%v", err)
	}
	if rc, _, errb := runCapture("archive", "verify", bag); rc != 1 {
		t.Errorf("载荷改一字节后 verify rc=%d（要 1）· stderr=%s", rc, errb)
	}
	if err := os.WriteFile(target, orig, 0o644); err != nil {
		t.Fatalf("还原失败：%v", err)
	}

	// 负控②：抹掉清单里任意一条 ⇒ 判红（判据② 的反例探针逐字要求）。
	keep := append([]string{}, manLines...)
	removed := keep[0]
	kept := []string{}
	for _, l := range keep[1:] {
		kept = append(kept, l)
	}
	if err := os.WriteFile(filepath.Join(bag, "manifest-sha256.txt"),
		[]byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("改清单失败：%v", err)
	}
	if rc, _, errb := runCapture("archive", "verify", bag); rc != 1 {
		t.Errorf("抹掉清单一条（%s）后 verify rc=%d（要 1）· stderr=%s", removed, rc, errb)
	}
	if err := os.WriteFile(filepath.Join(bag, "manifest-sha256.txt"),
		[]byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("还原清单失败：%v", err)
	}

	// 负控③：盘上多出未登记件 ⇒ 判红。
	intruder := filepath.Join(bag, "data", "未登记-zz.bin")
	if err := os.WriteFile(intruder, []byte("intruder"), 0o644); err != nil {
		t.Fatalf("放闯入件失败：%v", err)
	}
	if rc, _, errb := runCapture("archive", "verify", bag); rc != 1 {
		t.Errorf("多出未登记件后 verify rc=%d（要 1）· stderr=%s", rc, errb)
	}
	if err := os.Remove(intruder); err != nil {
		t.Fatalf("清理闯入件失败：%v", err)
	}
	// 回正控：修回来 ⇒ 0（证明上面那几条红不是恒红装置）。
	if rc, _, errb := runCapture("archive", "verify", bag); rc != 0 {
		t.Errorf("三条负控复原后 verify rc=%d（要 0）· stderr=%s", rc, errb)
	}
}

// TestArchiveVerifyNotABagGivesUp —— 「判不了」不许当「过了」：不是袋 ⇒ 退码 8。
func TestArchiveVerifyNotABagGivesUp(t *testing.T) {
	dir := t.TempDir()
	if rc, _, errb := runCapture("archive", "verify", dir); rc != 8 {
		t.Errorf("对不是袋的目录 verify rc=%d（要 8 —— 不给结论）· stderr=%s", rc, errb)
	}
	if rc, _, _ := runCapture("archive", "hash", filepath.Join(dir, "没有这件-zz")); rc != 2 {
		t.Errorf("路径不存在时 hash 的 rc=%d（要 2 —— 用法错，不静默跳过）", rc)
	}
}

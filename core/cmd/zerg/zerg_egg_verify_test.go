package main

// zerg_egg_verify_test.go —— 卵3①（**逐段并行 sha256 ＋ 内容寻址校验缓存**）的成对负控测试件。
//
// 五组牙（逐组都带「哑化即红」的口径）：
//
//	① 并行 ⟷ 单线程 **逐字节一致**：同一枚卵的 problems 序列（含未对齐/越界/截断/不符四档）必须逐字同；
//	   并把两态各落一件到 `$ZEGV_DIFF_DIR`（仓外 `cmp` 对拍用）。
//	② **四态**（走真命令入口 `verify` 分支）：冷跑（未命中）· 二次跑（印 `(缓存)`）· 改数据段一字节
//	   （必红 ＋ 缓存被击穿）· 改前 32 B ＋ 复原 mtime（必红 —— 且**必须**复原 mtime，否则本格退化成普通未命中）。
//	③ 缓存**失败/损坏回退**：条目损坏 / 键不符 / 目录不可写 ⇒ **一律未命中 ＋ 照旧全量**，✗ 不得因缓存误报绿。
//	④ ★ **负控锚**：态④ 单列一例 —— 哑化「前 32 B 复核」那一支（`-overlay` 换一份变体件）⇒ **本用例必红**。
//	⑤ 并发度公式 `W = clamp(ceil(D/h), 1, NumCPU)` ＋ 探针 ＋ 物理面（`zegvReadPhys`）与金标前缀格式一致。
//
// ★ 本件**不写** `TestMain`（包内他件已占该符号 ⇒ go vet 会报 multiple definitions）。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// zegvTFixture —— 造一枚**内容寻址名**（§13.2 的 `<sha>.egg`）的卵；返回卵路径。
// 夹具内容里带上 `name` ⇒ 不同的夹具名必得不同的 sha（✗ 不会互相覆盖）。
func zegvTFixture(t *testing.T, dir, name string, shards, shardKiB int) string {
	t.Helper()
	src := filepath.Join(dir, name+"-src")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < shards; i++ {
		buf := make([]byte, shardKiB*1024)
		for j := range buf {
			buf[j] = byte(j + i*13)
		}
		p := filepath.Join(src, "model", fmt.Sprintf("shard-%02d.safetensors", i))
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := fmt.Sprintf(`{"arch":"fixture","tag":%q}`, name)
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, name+".tmp.egg")
	if _, err := zegPack(src, tmp, 4096, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(d)
	final := filepath.Join(dir, hex.EncodeToString(sum[:])+".egg")
	if err := os.Rename(tmp, final); err != nil {
		t.Fatal(err)
	}
	return final
}

// zegvTRun —— 走**真命令入口**（`zergEggGateMain` 的 `verify` 分支），拿 rc/stdout/stderr。
func zegvTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var o, e strings.Builder
	rc := zergEggGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

// zegvTeggCopy —— 复制一枚卵（改件测试不污染原夹具）。
func zegvTeggCopy(t *testing.T, src, dst string) {
	t.Helper()
	d, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, d, 0o644); err != nil {
		t.Fatal(err)
	}
}

// zegvTsub —— 索引体里的**定长**同长替换（✗ 不许改长度：index_len 在 32 B 前缀里钉着）。
func zegvTsub(t *testing.T, d []byte, old, new string) []byte {
	t.Helper()
	if len(old) != len(new) {
		t.Fatalf("同长替换为前提：%q ⟷ %q", old, new)
	}
	n := strings.Count(string(d), old)
	if n != 1 {
		t.Fatalf("锚 %q 命中 %d 次（要恰好 1 次）", old, n)
	}
	return []byte(strings.Replace(string(d), old, new, 1))
}

// ---- ① 并行 ⟷ 单线程 逐字节一致 --------------------------------------------------------------

func TestZergEggVerifyParallelMatchesSingle(t *testing.T) {
	dir := t.TempDir()
	egg := zegvTFixture(t, dir, "par", 6, 24)
	raw, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	// 五档：干净 · 改一段 · 改三段（看**索引序**汇总）· 截断 · 索引里 off 未对齐 · 索引里 size 越界
	mutated := map[string][]byte{}
	d := append([]byte(nil), raw...)
	d[len(d)-1] ^= 0xff
	mutated["改一段"] = d
	d = append([]byte(nil), raw...)
	for _, off := range []int{20000, 60000, 100000} {
		d[off] ^= 0xa5
	}
	mutated["改三段"] = d
	mutated["截断"] = append([]byte(nil), raw[:len(raw)/2]...)
	// 索引体里的 `"off":<n>` / `"size":<n>` 同长改写（parse 照过 ⇒ 走「偏移未对齐」/「越出文件末尾」两支）
	base, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	// ★ 锚一律带 path（✗ 光用 `"off":N`：`8192` 是 `81920` 的前缀 ⇒ 会假命中 2 次）
	e1 := base.Entries[1]
	// ★ 越界那格取**末段**（size 换成同长的全 9 ⇒ off+size 必越出文件末尾 ⇒ 走 bounds 支）
	e0 := base.Entries[len(base.Entries)-1]
	off1 := strconv.FormatInt(e1.Off, 10)
	offBad := strconv.FormatInt(e1.Off-1, 10)
	size0 := strconv.FormatInt(e0.Size, 10)
	sizeBad := strings.Repeat("9", len(size0))
	d = append([]byte(nil), raw...)
	mutated["off未对齐"] = zegvTsub(t, d, `"path":"`+e1.Path+`","off":`+off1+`,`, `"path":"`+e1.Path+`","off":`+offBad+`,`)
	d = append([]byte(nil), raw...)
	mutated["size越界"] = zegvTsub(t, d, `"path":"`+e0.Path+`","off":`+strconv.FormatInt(e0.Off, 10)+`,"size":`+size0+`,`, `"path":"`+e0.Path+`","off":`+strconv.FormatInt(e0.Off, 10)+`,"size":`+sizeBad+`,`)

	names := []string{"干净", "改一段", "改三段", "截断", "off未对齐", "size越界"}
	var single, parallel strings.Builder
	for _, name := range names {
		p := filepath.Join(dir, "case-"+name+".egg")
		body := raw
		if name != "干净" {
			body = mutated[name]
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
		probsSingle, _, err := zegVerify(p, "") // 旧档：单线程全量（行为的真源）
		if err != nil {
			t.Fatalf("%s 单线程档报错：%v", name, err)
		}
		idx, err := zegParseIndex(p)
		if err != nil {
			t.Fatalf("%s 索引解析失败：%v", name, err)
		}
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		fi, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []int{1, 4, runtime.NumCPU()} {
			probsPar := zegvVerifyParallel(f, idx.Entries, idx.Align, fi.Size(), w)
			a := strings.Join(probsSingle, "\n")
			b := strings.Join(probsPar, "\n")
			if a != b {
				f.Close()
				t.Fatalf("%s（W=%d）并行档与单线程档不一致：\n单线程=%q\n并行  =%q", name, w, a, b)
			}
		}
		f.Close()
		single.WriteString(name + "\t" + strings.Join(probsSingle, " | ") + "\n")
		parallel.WriteString(name + "\t" + strings.Join(probsSingle, " | ") + "\n")
	}
	// 两态落件（仓外 `cmp` 逐字节对拍；✗ 本件不断言文件内容）
	if dd := os.Getenv("ZEGV_DIFF_DIR"); dd != "" {
		if err := os.MkdirAll(dd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dd, "single-thread.txt"), []byte(single.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dd, "parallel.txt"), []byte(parallel.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// ---- ② 四态（真命令入口）---------------------------------------------------------------------

func TestZergEggVerifyCacheFourStates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	egg := zegvTFixture(t, dir, "four", 4, 32)

	// ① 冷跑（无缓存）
	rc, out, errs := zegvTRun(t, "verify", egg, "--workers", "8")
	if rc != exitOK {
		t.Fatalf("冷跑 rc=%d out=%q err=%q", rc, out, errs)
	}
	if strings.Contains(out, zegvCacheMark) {
		t.Fatalf("首跑就印了缓存标记：%q", out)
	}
	if !strings.Contains(out, "未命中") {
		t.Fatalf("首跑没报未命中：%q", out)
	}
	probs, _, err := zegVerify(egg, "") // 同一枚的**单线程档**读数（冷跑结论的真源）
	if err != nil {
		t.Fatal(err)
	}
	n := len(probs)
	if n != 0 {
		t.Fatalf("冷跑：单线程档给出 problems=%v", probs)
	}
	// ② 二次跑 ⇒ 命中（逐字印 (缓存)）
	rc2, out2, errs2 := zegvTRun(t, "verify", egg, "--workers", "8")
	if rc2 != exitOK {
		t.Fatalf("二次跑 rc=%d out=%q err=%q", rc2, out2, errs2)
	}
	if !strings.Contains(out2, zegvCacheMark) {
		t.Fatalf("二次跑未逐字印 %q：%q", zegvCacheMark, out2)
	}
	// ③ 改数据段一字节（自然写 ⇒ mtime 走）⇒ 必红 ＋ 缓存被击穿
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	base, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	// ★ 改**段内**一字节（首段 off+3）：✗ 别改 `d[len(d)-1]` —— 末段后可能有补零 pad（改 pad 判绿是格式的既有口径）
	hit := base.Entries[0].Off + 3
	if hit+1 > int64(len(d)) {
		t.Fatalf("夹具卵太短：%d", len(d))
	}
	d[hit] ^= 0xff
	if err := os.WriteFile(egg, d, 0o644); err != nil {
		t.Fatal(err)
	}
	rc3, out3, _ := zegvTRun(t, "verify", egg, "--workers", "8")
	if rc3 == exitOK {
		t.Fatalf("改了一字节却判绿：%q", out3)
	}
	// ★ D7（GAP-20261015-25）：旧串 `sha256 不符` → 设计报话 `段内容 sha256 与索引不符（期望 …，实读 …）`。
	if !strings.Contains(out3, "sha256 与索引不符") {
		t.Fatalf("改字节没报 D7 设计报话（sha256 与索引不符）：%q", out3)
	}
	if !strings.Contains(out3, "未命中") {
		t.Fatalf("缓存未被击穿（没报未命中）：%q", out3)
	}
	if strings.Contains(out3, zegvCacheMark) {
		t.Fatalf("改了字节仍报命中：%q", out3)
	}
	// ④ 前 32 B 被改（＋复原 mtime）⇒ 必红（magic 那一位）
	rc4, out4, _ := zegvTPrefixTamperState(t, dir, "four-p3", 3)
	if rc4 == exitOK {
		t.Fatalf("前 32 B 被改却判绿：%q", out4)
	}
	if !strings.Contains(out4, "前 32 B") {
		t.Fatalf("没报前 32 B 复核失败：%q", out4)
	}
	// ④b 保留 8 字节（解析器**看不见**）⇒ 解析照过、数据段照好 ⇒ 判红**只能**来自复核束这条 fail-closed
	rc4b, out4b, _ := zegvTPrefixTamperState(t, dir, "four-p30", 30)
	if rc4b == exitOK {
		t.Fatalf("保留 8 字节被改却判绿（fail-closed 那条没咬住）：%q", out4b)
	}
}

// zegvTPrefixTamperState —— 态④ 的可复用流程：新造一枚 → 冷跑落条目 → 改前 32 B 第 off 位 → **复原 mtime** →
// 再跑一次。返回后一次读数。★ 复原 mtime 是本格的前提（不复原就退化成普通未命中，牙就没了）。
func zegvTPrefixTamperState(t *testing.T, dir, name string, off int) (int, string, string) {
	t.Helper()
	egg := zegvTFixture(t, dir, name, 3, 32)
	if rc, out, errb := zegvTRun(t, "verify", egg, "--workers", "4"); rc != exitOK {
		t.Fatalf("冷跑不绿：rc=%d out=%q err=%q", rc, out, errb)
	}
	fi, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	d[off] ^= 0x5a
	if err := os.WriteFile(egg, d, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(egg, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	if fi2.ModTime().UnixNano() != fi.ModTime().UnixNano() {
		t.Fatalf("mtime 未复原 ⇒ 本格退化成普通未命中（前提不成立）：%d ≠ %d", fi2.ModTime().UnixNano(), fi.ModTime().UnixNano())
	}
	return zegvTRun(t, "verify", egg, "--workers", "4")
}

// ---- ③ 缓存失败 / 损坏回退（✗ 不得因缓存误报绿）-----------------------------------------------

func TestZergEggVerifyCacheCorruptFallback(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	t.Setenv(zegvCacheEnv, cacheDir)
	egg := zegvTFixture(t, dir, "cf", 3, 24)
	key, keyKnown := zegvKeyFromBase(filepath.Base(egg))
	if !keyKnown {
		t.Fatal("夹具名不是内容寻址形")
	}
	// 首跑 ⇒ 条目在
	if rc, out, errb := zegvTRun(t, "verify", egg, "--workers", "4"); rc != exitOK {
		t.Fatalf("首跑 rc=%d out=%q err=%q", rc, out, errb)
	}
	cp := zegvCachePath(cacheDir, key)
	if _, err := os.Stat(cp); err != nil {
		t.Fatalf("首跑没落条目：%v", err)
	}
	// (a) 条目损坏 ⇒ 未命中且结论仍由全量给（干净卵 ⇒ 绿）
	if err := os.WriteFile(cp, []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := zegvTRun(t, "verify", egg, "--workers", "4")
	if rc != exitOK {
		t.Fatalf("条目坏了把干净卵判红：rc=%d out=%q", rc, out)
	}
	if strings.Contains(out, zegvCacheMark) {
		t.Fatalf("条目坏了却报命中：%q", out)
	}
	if !strings.Contains(out, "条目损坏") {
		t.Fatalf("没报条目损坏：%q", out)
	}
	// (b) 条目**键不符**（拿别键的绿顶到本键位）⇒ 不得命中
	other := &zegvCacheEntry{Ver: zegvCacheVer, Key: strings.Repeat("0", 64), ToolVersion: zegToolVersion, Segments: 1, Align: 4096}
	if err := zegvWriteCache(cp, other); err != nil {
		t.Fatal(err)
	}
	rcb, outb, _ := zegvTRun(t, "verify", egg, "--workers", "4")
	if rcb != exitOK {
		t.Fatalf("键不符把干净卵判红：rc=%d out=%q", rcb, outb)
	}
	if strings.Contains(outb, zegvCacheMark) {
		t.Fatalf("键不符却报命中：%q", outb)
	}
	if !strings.Contains(outb, "条目键") {
		t.Fatalf("没报条目键不符：%q", outb)
	}
	// (c) 缓存目录不可写 ⇒ 本次不落条目、**判结论不变**（干净卵仍绿）
	roDir := filepath.Join(dir, "ro-cache")
	if err := os.MkdirAll(roDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o700) })
	t.Setenv(zegvCacheEnv, roDir)
	rcc, outc, _ := zegvTRun(t, "verify", egg, "--workers", "4")
	if rcc != exitOK {
		t.Fatalf("缓存目录不可写把干净卵判红：rc=%d out=%q", rcc, outc)
	}
	if !strings.Contains(outc, "未落条目") {
		t.Fatalf("缓存目录不可写却没出声：%q", outc)
	}
}

// ---- ④ ★ 负控锚：哑化「前 32 B 复核」⇒ 本用例必红 --------------------------------------------

func TestZergEggVerifyPrefixRecheckIsLoadBearing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	rc, out, errb := zegvTPrefixTamperState(t, dir, "anchor", 3)
	if rc == exitOK {
		t.Fatalf("负控锚：前 32 B 被改（mtime 已复原）却判绿 —— 前 32 B 复核这一支没咬住 rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "前 32 B") {
		t.Fatalf("负控锚：红了但没点名「前 32 B」：%q", out)
	}
}

// ---- ⑤ 并发度公式 ＋ 探针 ＋ 物理面与金标前缀格式一致 -----------------------------------------

func TestZergEggVerifyWorkersDerivation(t *testing.T) {
	ncpu := runtime.NumCPU()
	clamp := func(w int) int {
		if w < 1 {
			return 1
		}
		if w > ncpu {
			return ncpu
		}
		return w
	}
	gib := int64(1) << 30
	cases := []struct {
		name   string
		data   int64
		h      float64
		forced int
		want   int
	}{
		{"零字节活", 0, 1.0, 0, 1},
		{"0.5 GiB（h 上界 0.53 · 探针更快）", gib / 2, 2.98, 0, clamp(1)},
		{"1 GiB", gib, 2.98, 0, clamp(2)},
		{"2 GiB", 2 * gib, 2.98, 0, clamp(4)},
		{"22 GiB / 0.53（设计稿档）", 22 * gib, 2.98, 0, clamp(42)},
		{"探针比口径更慢（0.2 GB/s）⇒ 取探针", gib, 0.2, 0, clamp(5)},
		{"探针拿不到读数 ⇒ 兜底 0.53", 4 * gib, 0, 0, clamp(8)},
		{"--workers 99 ⇒ clamp 到 NumCPU", 1, 1.0, 99, ncpu},
		{"1 字节 · 自动档 ⇒ 1", 1, 1.0, 0, clamp(1)},
		{"--workers 3 ⇒ 3", 1, 1.0, 3, clamp(3)},
	}
	for _, c := range cases {
		got := zegvWorkersFor(c.data, c.h, c.forced)
		if got != c.want {
			t.Fatalf("%s：W=%d want=%d", c.name, got, c.want)
		}
	}
	g := zegvProbeHashGBps()
	if g <= 0 {
		t.Fatalf("探针读数非正：%v", g)
	}
	t.Logf("本机 NumCPU=%d · 单流 sha256 探针 %.3f GB/s", ncpu, g)
}

// TestZergEggVerifyPhysMatchesGolden —— 钉住 `zegvReadPhys` 与金标前缀格式的关系：
// 索引长度必须等于金标索引 JSON 的**字节数**（`index_len` 在 32 B 前缀的 16..24 位）；
// 前缀摘要必须等于金标产物前 32 B 的 sha256（✗ 换偏移 / 换端序 ⇒ 本格必红）。
func TestZergEggVerifyPhysMatchesGolden(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	zegTBuildGoldenTree(t, src)
	egg := filepath.Join(dir, "g.egg")
	if _, err := zegPack(src, egg, 16384, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(egg)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	p, err := zegvReadPhys(f, fi)
	if err != nil {
		t.Fatal(err)
	}
	if p.IndexLen != int64(len(zegGoldenIndexJSON)) {
		t.Fatalf("index_len 偏移/端序与金标不符：got=%d want=%d", p.IndexLen, len(zegGoldenIndexJSON))
	}
	want := sha256.Sum256([]byte(zegTIndexBytes(t, egg)))
	if p.IndexSHA != hex.EncodeToString(want[:]) {
		t.Fatalf("索引摘要与金标不符：got=%s", p.IndexSHA)
	}
	raw, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := sha256.Sum256(raw[:zegHeaderPrefix])
	if p.PrefixSHA != hex.EncodeToString(wantPrefix[:]) {
		t.Fatalf("前缀摘要与金标不符：got=%s", p.PrefixSHA)
	}
	if p.StSize != int64(len(raw)) || p.StSize == 0 {
		t.Fatalf("st_size 与盘上不符：got=%d want=%d", p.StSize, len(raw))
	}
	// 并发度口径随 D 单调（同一台机上）：段数据越大，W 不减
	small := zegvWorkersFor(1<<20, 1.0, 0)
	big := zegvWorkersFor(4<<30, 1.0, 0)
	if big < small {
		t.Fatalf("并发度不随 D 单调：small=%d big=%d", small, big)
	}
}

// TestZergEggSelfTestCLIBothHalves —— `--self-test` 的**两档都真跑**（旧档 py 真搬面 10 项 ＋ 卵3① 并行＋缓存面 7 项）。
// ★ 这条是那枚「负例① 改成 pad 字节 ⇒ 自检假红」的回归牙：只跑旧档 / 只印一行 ⇒ 本格必红。
func TestZergEggSelfTestCLIBothHalves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	rc, out, errb := zegvTRun(t, "--self-test")
	if rc != exitOK {
		t.Fatalf("--self-test rc=%d\n stdout=%q\n stderr=%q", rc, out, errb)
	}
	if !strings.Contains(out, "self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）") {
		t.Fatalf("旧档（py 真搬面）那行不在：%q", out)
	}
	if !strings.Contains(out, "self-test（卵3① 并行＋缓存）通过（7 项：3 正例 + 4 负例）") {
		t.Fatalf("卵3① 档那行不在：%q", out)
	}
}

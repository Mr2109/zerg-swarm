package main

// zerg_egg_test.go —— `zerg_egg.go`（`scripts/node-a/zerg-egg.py` 的 Go 真搬）的**成对负控测试件**。
//
// 四组牙：
//  ① 格式契约（golden）：固定树 + 固定 `created_at` ⇒ pack 产物的 **sha256 与索引 JSON 逐字节**必须命中常量
//     （任何字段序 / 编码 / 对齐漂移 ⇒ 红）。
//  ② 校验面：干净袋不报；改一字节 / 截断 ⇒ 必报。
//  ③ ★ A 类止血牙：`zegSelfTest` 必须在**自建的临时目录**里跑并**回收干净**（`defer os.RemoveAll`）——
//     把那一行删掉 ⇒ 本用例必红（见回执「哑化负控」）。py 原件在这一档**每次都漏一个目录**（已现读坐实）。
//  ④ 命令行面：未知旗标 / 非法子命令 / 无子命令的 rc 与 py 逐字对齐（退 2 / 2 / 64）。
//
// ★ 本件**不写** `TestMain`（包内他件已占该符号 ⇒ go vet 会报 multiple definitions）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegGoldenEggSHA —— 固定 fixture（`a.txt`="hello" · `sub/b.bin`=0x00..0xFF · 均 0644 ·
// `--now 2000-01-01T00:00:00Z` · align 16384）下 Go 侧 pack 产物的 sha256。
//
// ★ **2026-10-15（卵1① 段语义对齐 OCI `mediaType`）起不再与 py 原件逐字节同** —— `entries[]` 逐段**多**
// 一个 `mediaType` 键（`§9.11②` 的**纯声明** · `§11.2` 不变量 7 的**可加面** ⇒ 旧读端**忽略且不报错**；
// py 真身 `scripts/node-a/zerg-egg.py` 已登记退场、盘上无此件）。本值 = **改后现跑**照抄（✗ 不手编）。
const zegGoldenEggSHA = "9c843c0ffc423beb52d5c408b66f154a1238079fc2c133bbab86a373a460d672"

// zegGoldenIndexJSON —— 同一 fixture 的索引 JSON **逐字节**（字段序即契约：
// `path/off/size/sha256/mode/mediaType` —— ★ `mediaType` 由 卵1① 追加在**末位**，✗ 不改前五格序）。
const zegGoldenIndexJSON = `{"magic":"ZEGG","version":1,"align":16384,"tool":{"name":"zerg-egg","version":"1.0.0"},"created_at":"2000-01-01T00:00:00Z","entries":[{"path":"a.txt","off":16384,"size":5,"sha256":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824","mode":420,"mediaType":"application/vnd.oci.image.config.v1+json"},{"path":"sub/b.bin","off":32768,"size":256,"sha256":"40aff2e9d2d8922e47afd4648e6967497158785fbd1da870e7110266bf944880","mode":420,"mediaType":"application/vnd.oci.image.config.v1+json"}]}`

// zegTBuildGoldenTree —— 造固定树（内容 + 权限都钉死 ⇒ 产物可逐字节复现）。
func zegTBuildGoldenTree(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 256)
	for i := 0; i < 256; i++ {
		b[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.bin"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	// umask 兜底：显式钉回 0644（索引里的 mode 逐字取决于它）。
	if err := os.Chmod(filepath.Join(root, "a.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "sub", "b.bin"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// zegTIndexBytes —— 从一枚卵里抠出索引 JSON 字节。
func zegTIndexBytes(t *testing.T, egg string) []byte {
	t.Helper()
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	n := int64(d[16]) | int64(d[17])<<8 | int64(d[18])<<16 | int64(d[19])<<24 |
		int64(d[20])<<32 | int64(d[21])<<40 | int64(d[22])<<48 | int64(d[23])<<56
	if zegHeaderPrefix+int(n) > len(d) {
		t.Fatalf("索引长度越界：n=%d len=%d", n, len(d))
	}
	return d[zegHeaderPrefix : zegHeaderPrefix+int(n)]
}

// ---- ① 格式契约 -----------------------------------------------------------------------------

func TestZergEggGoldenFormat(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	zegTBuildGoldenTree(t, src)
	egg := filepath.Join(dir, "g.egg")
	if _, err := zegPack(src, egg, 16384, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(d)
	if got := hex.EncodeToString(sum[:]); got != zegGoldenEggSHA {
		t.Fatalf("golden sha256 漂移：got=%s want=%s（格式契约被改）", got, zegGoldenEggSHA)
	}
	if got := string(zegTIndexBytes(t, egg)); got != zegGoldenIndexJSON {
		t.Fatalf("索引 JSON 逐字节不符：\n got=%s\nwant=%s", got, zegGoldenIndexJSON)
	}
	if string(d[0:4]) != zegMagic {
		t.Fatalf("前缀 magic 不符：%q", d[0:4])
	}
}

// ---- ② 校验面 -------------------------------------------------------------------------------

func TestZergEggVerifyDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	zegTBuildGoldenTree(t, src)
	egg := filepath.Join(dir, "g.egg")
	if _, err := zegPack(src, egg, 16384, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if probs, _, err := zegVerify(egg, ""); err != nil || len(probs) != 0 {
		t.Fatalf("干净袋不该有问题：probs=%v err=%v", probs, err)
	}
	// 改数据区一字节（落在 a.txt 的 5 字节载荷里，off=16384）
	d, _ := os.ReadFile(egg)
	if len(d) < 16384+20 {
		t.Fatal("卵太小")
	}
	d[16384+2] ^= 0xff
	bad := filepath.Join(dir, "bad.egg")
	if err := os.WriteFile(bad, d, 0o644); err != nil {
		t.Fatal(err)
	}
	probs, _, err := zegVerify(bad, "")
	if err != nil {
		t.Fatal(err)
	}
	hit := false
	for _, p := range probs {
		if strings.Contains(p, "sha256") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("改字节未被检出：probs=%v", probs)
	}
	// 截断
	trunc := filepath.Join(dir, "trunc.egg")
	if err := os.WriteFile(trunc, d[:16384+1], 0o644); err != nil {
		t.Fatal(err)
	}
	if probs, _, err := zegVerify(trunc, ""); err != nil || len(probs) == 0 {
		t.Fatalf("截断未被检出：probs=%v err=%v", probs, err)
	}
}

// ---- ⑤ v2 布局（段表化 ＋ 引擎段在尾）：闭环 · 只写尾部 · v1 后向兼容 -------------------------
//
// 这三组是设计稿 Q3 的**机检物**：① v2 打完能读回（info 面 version=2 / gen / 引擎段在尾）＋
// `--rebuild-into` 三级链逐字同 ＋ 1 MiB 假 GGUF sha 前后一致；② 升引擎只写尾部（数据区 sha 不变 ·
// 写区间起点越过非引擎段末尾 · inode 不变）＋ 负控「改权重段必拒」；③ 盘上 v1 卵仍 rc=0（后向兼容）。

// zegTV2BuildTree —— v2 夹具树（9 段：权重 1 MiB 假 GGUF ＋ 引擎 4 段含三级相对链 ＋ 配方/契约 4 段）。
func zegTV2BuildTree(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{"model", "engine", "bin", "sysroot/lib"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := make([]byte, 1<<20)
	copy(w, []byte("GGUF"))
	for i := 4; i < len(w); i++ {
		w[i] = byte(i % 251)
	}
	if err := os.WriteFile(filepath.Join(root, "model", "demo.gguf"), w, 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"config.json":           `{"arch":"v2-test"}`,
		"contract/policy.json":  `{"policy":1}`,
		"bin/run.sh":            "#!/bin/sh\nexec ./engine/zerg-engine\n",
		"engine/zerg-engine":    "#!/bin/sh\necho engine-v1\n",
		"engine/libfoo.so.1.12": "ELF-foo-1.12",
		"sysroot/lib/libc.so.6": "stub-libc",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(root, "bin", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "engine", "zerg-engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libfoo.so.1.12", filepath.Join(root, "engine", "libfoo.so.1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libfoo.so.1", filepath.Join(root, "engine", "libfoo.so")); err != nil {
		t.Fatal(err)
	}
}

// zegTV2Run 走**真命令面**（`zergEggGateMain`）取 rc/stdout/stderr。
func zegTV2Run(argv ...string) (int, string, string) {
	var o, e strings.Builder
	rc := zergEggGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

// TestZergEggV2ClosedLoop —— ③ v2 闭环五验：info 面 / 重建 / 三级链 / 权重 sha / 段序。
func TestZergEggV2ClosedLoop(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tree")
	zegTV2BuildTree(t, src)
	egg := filepath.Join(dir, "v2.egg")

	rc, out, errb := zegTV2Run("pack", src, egg, "--align", "4096")
	if rc != exitOK {
		t.Fatalf("pack rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "v2 布局") {
		t.Fatalf("pack 没印 v2 布局行：%q", out)
	}
	// 段数：夹具固定 9 段（自印 N ＋ 下界断言 ⇒ 抽空/少段都当场红）
	const wantSegs = 9
	idx, err := zegParseIndex(egg)
	if err != nil {
		t.Fatalf("v2 卵读不回：%v", err)
	}
	if len(idx.Entries) < wantSegs {
		t.Fatalf("段数 %d 低于下限 %d（夹具打空 ⇒ fail-closed）", len(idx.Entries), wantSegs)
	}
	if len(idx.Entries) != wantSegs {
		t.Fatalf("段数 %d ≠ 夹具固定 %d", len(idx.Entries), wantSegs)
	}
	if idx.Layout != zegV2LayoutVer || idx.Version != zegV2LayoutVer || idx.Gen != 1 {
		t.Fatalf("布局/主号/gen = %d/%d/%d（要 2/2/1）", idx.Layout, idx.Version, idx.Gen)
	}
	// ① info 面：version=2 · gen · 引擎段在尾（`cocoon info` 走的就是这条壳）
	rc, out, errb = zegTV2Run("info", egg)
	if rc != exitOK {
		t.Fatalf("info rc=%d err=%q", rc, errb)
	}
	for _, frag := range []string{"version=2", "布局=v2", "gen=1", "引擎段在尾=是"} {
		if !strings.Contains(out, frag) {
			t.Fatalf("info 面缺 %q：%q", frag, out)
		}
	}
	// ② 引擎段必须全在尾部（判据与 info 同源）
	if !zegV2EngineAtTail(idx.Entries) {
		t.Fatalf("引擎段不在尾：%v", zegV2SegOrder(idx.Entries))
	}
	// ③ `--rebuild-into` ＋ 三级链逐字同
	reb := filepath.Join(dir, "rebuilt")
	rc, _, errb = zegTV2Run("verify", egg, "--rebuild-into", reb)
	if rc != exitOK {
		t.Fatalf("verify --rebuild-into rc=%d err=%q", rc, errb)
	}
	for _, c := range []struct{ path, want string }{
		{"engine/libfoo.so", "libfoo.so.1"},
		{"engine/libfoo.so.1", "libfoo.so.1.12"},
	} {
		full := filepath.Join(reb, filepath.FromSlash(c.path))
		li, lerr := os.Lstat(full)
		if lerr != nil || li.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("重建后 %s 不是符号链接（err=%v）", c.path, lerr)
		}
		if got, rerr := os.Readlink(full); rerr != nil || got != c.want {
			t.Fatalf("重建后 %s 的链名 %q ≠ 原件 %q（err=%v）", c.path, got, c.want, rerr)
		}
	}
	if fi, lerr := os.Lstat(filepath.Join(reb, "bin", "run.sh")); lerr != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("重建后 bin/run.sh 权限不是 0755：err=%v mode=%v", lerr, fi.Mode())
	}
	// ④ 1 MiB 假 GGUF 的 sha256 前后一致
	a, _ := os.ReadFile(filepath.Join(src, "model", "demo.gguf"))
	b, _ := os.ReadFile(filepath.Join(reb, "model", "demo.gguf"))
	if sha256.Sum256(a) != sha256.Sum256(b) {
		t.Fatalf("1 MiB 假 GGUF 重建后 sha256 不一致")
	}
	// ⑤ 内容寻址缓存对 v2 也成立（键 = `<卵内容 sha>.egg`；v2 的复核束取「8 KiB 整头 ＋ 尾表字节」）
	eggBytes, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(eggBytes)
	cached := filepath.Join(dir, "cached")
	if err := os.MkdirAll(cached, 0o755); err != nil {
		t.Fatal(err)
	}
	caddr := filepath.Join(cached, hex.EncodeToString(sum[:])+".egg")
	if err := os.WriteFile(caddr, eggBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(zegvCacheEnv, filepath.Join(cached, "cache"))
	if rc, out, errb := zegTV2Run("verify", caddr); rc != exitOK {
		t.Fatalf("v2 内容寻址卵首次 verify rc=%d err=%q", rc, errb)
	} else if !strings.Contains(out, "未命中") {
		t.Fatalf("首次应未命中缓存：%q", out)
	}
	if rc, out, errb := zegTV2Run("verify", caddr); rc != exitOK {
		t.Fatalf("v2 内容寻址卵二次 verify rc=%d err=%q", rc, errb)
	} else if !strings.Contains(out, zegvCacheMark) {
		t.Fatalf("v2 卵二次 verify 未命中缓存（复核束对 v2 没接上）：%q", out)
	}
}

// TestZergEggV2UpgradeOnlyTouchesTail —— ④ 升引擎只写尾部（含负控：改权重段必拒）。
func TestZergEggV2UpgradeOnlyTouchesTail(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tree")
	zegTV2BuildTree(t, src)
	egg := filepath.Join(dir, "up.egg")
	if rc, _, errb := zegTV2Run("pack", src, egg, "--align", "4096"); rc != exitOK {
		t.Fatalf("pack rc=%d err=%q", rc, errb)
	}
	before, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	fi0, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	idx0, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	// 只改一个**只属引擎段**的小文件
	eng := filepath.Join(src, "engine", "zerg-engine")
	if err := os.WriteFile(eng, []byte("#!/bin/sh\necho engine-v2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := zegTV2Run("pack", src, egg, "--update")
	if rc != exitOK {
		t.Fatalf("--update rc=%d err=%q", rc, errb)
	}
	for _, frag := range []string{"升引擎完成", "gen 1 → 2", "起点 > 末尾 = true", "sha256 前后一致 = true"} {
		if !strings.Contains(out, frag) {
			t.Fatalf("升引擎读数缺 %q：%q", frag, out)
		}
	}
	fi1, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	// ① inode 不变（原地写、没换件）＋ 体积只增不减
	if fi0.Size() <= 0 || fi1.Size() < fi0.Size() {
		t.Fatalf("体积异常：%d → %d", fi0.Size(), fi1.Size())
	}
	if !os.SameFile(fi0, fi1) {
		t.Fatalf("inode 变了 ⇒ 不是原地追加")
	}
	// ② 数据区逐字节同（[8 KiB, 旧 file_len)：权重/配方/契约段一个字节未动）
	after, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	if !(idx0.FileLen > zegV2HeaderBytes && int64(len(before)) >= idx0.FileLen && int64(len(after)) >= idx0.FileLen) {
		t.Fatalf("区间越界（旧 file_len=%d before=%d after=%d）", idx0.FileLen, len(before), len(after))
	}
	if sha256.Sum256(before[zegV2HeaderBytes:idx0.FileLen]) != sha256.Sum256(after[zegV2HeaderBytes:idx0.FileLen]) {
		t.Fatalf("数据区 sha256 变了 ⇒ 前面被重写（升引擎必须只写尾部）")
	}
	// ③ 权重段那一坨字节直接对拍
	weightOff, weightSize := int64(-1), int64(0)
	for _, e := range idx0.Entries {
		if e.Path == "model/demo.gguf" {
			weightOff, weightSize = e.Off, e.Size
		}
	}
	if weightOff < 0 || weightSize != 1<<20 {
		t.Fatalf("夹具里找不到 1 MiB 权重段（off=%d size=%d）", weightOff, weightSize)
	}
	if sha256.Sum256(before[weightOff:weightOff+weightSize]) != sha256.Sum256(after[weightOff:weightOff+weightSize]) {
		t.Fatalf("权重段字节变了（✗ 升引擎只许追加引擎段）")
	}
	// ④ gen 单调 ＋ 升级后仍自洽（新引擎内容读得出）
	idx1, err := zegParseIndex(egg)
	if err != nil {
		t.Fatalf("升级后读不回：%v", err)
	}
	if idx1.Gen != idx0.Gen+1 {
		t.Fatalf("gen %d → %d（要 +1）", idx0.Gen, idx1.Gen)
	}
	if probs, _, verr := zegVerify(egg, ""); verr != nil || len(probs) != 0 {
		t.Fatalf("升级后 verify 报问题：err=%v probs=%v", verr, probs)
	}
	// ⑤ 负控：改权重段 ⇒ 升级必拒，且件零字节变动
	snap := after
	if err := os.WriteFile(filepath.Join(src, "model", "demo.gguf"), []byte("GGUF-changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb = zegTV2Run("pack", src, egg, "--update")
	if rc == exitOK {
		t.Fatalf("改了权重段竟被原地升级（✗ 必须拒）")
	}
	if !strings.Contains(errb, "非引擎段") {
		t.Fatalf("拒了但没点名「非引擎段」：%q", errb)
	}
	now, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(snap) != sha256.Sum256(now) {
		t.Fatalf("拒执却动了件（%d → %d B）", len(snap), len(now))
	}
}

// TestZergEggV1BackwardCompat —— ② 盘上 v1 卵仍 rc=0（读端必顶 v1 · 写端才默认 v2）。
func TestZergEggV1BackwardCompat(t *testing.T) {
	// 仓内既有 v1 夹具（`agent/internal/eggpkg/testdata/demo.egg` · 2 段 · 未标 mediaType 的老形态）
	demo := filepath.Join("..", "..", "..", "agent", "internal", "eggpkg", "testdata", "demo.egg")
	if _, err := os.Stat(demo); err != nil {
		t.Skipf("仓内 v1 夹具不在（%v）⇒ 本机无法取这一格读数", err)
	}
	rc, out, errb := zegTV2Run("verify", demo)
	if rc != exitOK {
		t.Fatalf("v1 卵 verify rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "校验通过") {
		t.Fatalf("v1 卵 verify 没报通过：%q", out)
	}
	rc, out, errb = zegTV2Run("info", demo)
	if rc != exitOK {
		t.Fatalf("v1 卵 info rc=%d err=%q", rc, errb)
	}
	if !strings.Contains(out, "version=1") {
		t.Fatalf("v1 卵 info 没印 version=1：%q", out)
	}
	if strings.Contains(out, "布局=v2") {
		t.Fatalf("v1 卵被印成 v2（✗ 两种布局混了）：%q", out)
	}
}

// ---- ③ ★ A 类止血牙（哑化 `defer os.RemoveAll(t)` ⇒ 本用例必红）--------------------------------

func TestZergEggSelfTestRecyclesTemp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	before := zegTCountEntries(t, dir)
	var out, errb strings.Builder
	if rc := zegSelfTest(&out, &errb); rc != exitOK {
		t.Fatalf("自检 rc=%d stderr=%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）") {
		t.Fatalf("自检成功行不符：%q", out.String())
	}
	after := zegTCountEntries(t, dir)
	if after != before {
		t.Fatalf("自检遗留临时目录：before=%d after=%d（A 类泄漏未止血）", before, after)
	}
}

func zegTCountEntries(t *testing.T, dir string) int {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(des)
}

// ---- ④ 命令行面（rc 与 py 逐字对齐）---------------------------------------------------------

func TestZergEggArgvFaces(t *testing.T) {
	// 无子命令 ⇒ 打帮助 + 退 64
	var o, e strings.Builder
	if rc := zergEggGateMain(&o, &e, nil); rc != zegExitToolUsage {
		t.Fatalf("无子命令 rc=%d want=%d", rc, zegExitToolUsage)
	}
	if o.String() != zegHelpText {
		t.Fatalf("无子命令 stdout 非帮助正文")
	}
	// 未知旗标 ⇒ 退 2，stderr 带 argparse 式错误串
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"--bogus"}); rc != exitUsage {
		t.Fatalf("未知旗标 rc=%d want=2", rc)
	}
	if !strings.Contains(e.String(), "error: unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标 stderr 不符：%q", e.String())
	}
	// 非法子命令 ⇒ 退 2
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"zzz"}); rc != exitUsage {
		t.Fatalf("非法子命令 rc=%d want=2", rc)
	}
	if !strings.Contains(e.String(), "invalid choice: 'zzz'") {
		t.Fatalf("非法子命令 stderr 不符：%q", e.String())
	}
	// pack 缺实参 ⇒ 退 64
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"pack"}); rc != zegExitToolUsage {
		t.Fatalf("pack 缺参 rc=%d want=64", rc)
	}
	if !strings.Contains(e.String(), "用法：pack") {
		t.Fatalf("pack 缺参 stderr 不符：%q", e.String())
	}
}

// ---- ⑤ ★ 卵1①② 四态亲测（真命令面 ＋ 真造件）＋ 两枚负控 -----------------------------------------

// TestZergEggMediaPathFourStates —— 卵1①② 的机检件：四态**走真命令面**（`zergEggGateMain`）取
// rc/stdout，✗ 不只调内部函数；负控 = 同一份输入走**不审计**的旧形态必须放行（＝本牙非永真）。
func TestZergEggMediaPathFourStates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte("{\"arch\":\"four-states\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.safetensors"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	egg := filepath.Join(dir, "four.egg")
	if _, err := zegPack(src, egg, 4096, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	clone := func(name string, edit func(ents []map[string]json.RawMessage) error) string {
		t.Helper()
		pp := filepath.Join(dir, name)
		d, err := os.ReadFile(egg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pp, d, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := zegVerRebuildEgg(pp, func(idx []byte) ([]byte, error) {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(idx, &m); err != nil {
				return nil, err
			}
			var ents []map[string]json.RawMessage
			if err := json.Unmarshal(m["entries"], &ents); err != nil {
				return nil, err
			}
			if err := edit(ents); err != nil {
				return nil, err
			}
			eb, err := json.Marshal(ents)
			if err != nil {
				return nil, err
			}
			m["entries"] = eb
			return json.Marshal(m)
		}, 0); err != nil {
			t.Fatal(err)
		}
		return pp
	}

	// 态①：正常卵 ⇒ `info` 逐段印 `mediaType=`（取值落 OCI 闭集内）＋ 写端 ⟷ 读端逐条一致
	var o1, e1 strings.Builder
	if rc := zergEggGateMain(&o1, &e1, []string{"info", egg}); rc != exitOK {
		t.Fatalf("态① info rc=%d stderr=%q", rc, e1.String())
	}
	out1 := o1.String()
	if !strings.Contains(out1, "mediaType="+zegMediaConfig) {
		t.Fatalf("态① info 没有 config 类的 mediaType：%q", out1)
	}
	if !strings.Contains(out1, "mediaType="+zegMediaLayer) {
		t.Fatalf("态① info 没有 layer 类的 mediaType：%q", out1)
	}
	idx, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Entries {
		if e.MediaType != zegMediaTypeOfPath(e.Path) {
			t.Fatalf("态① 段 %q 的 mediaType=%q ≠ 表推出 %q", e.Path, e.MediaType, zegMediaTypeOfPath(e.Path))
		}
	}
	t.Logf("态① info 读数：%s", strings.ReplaceAll(strings.TrimSpace(out1), "\n", " | "))

	// 态②：`mediaType` 缺失 / 闭集外 ⇒ 照 OCI **放行 ＋ 登记**（✗ 不 error）
	oldEgg := clone("old.egg", func(ents []map[string]json.RawMessage) error {
		for i := range ents {
			delete(ents[i], "mediaType")
		}
		return nil
	})
	var o2, e2 strings.Builder
	if rc := zergEggGateMain(&o2, &e2, []string{"verify", oldEgg}); rc != exitOK {
		t.Fatalf("态② 未标档 verify rc=%d stderr=%q", rc, e2.String())
	}
	if !strings.Contains(o2.String(), "未标 mediaType") {
		t.Fatalf("态② 未标档未逐字印「未标 mediaType」：%q", o2.String())
	}
	t.Logf("态② 未标读数：%q", strings.TrimSpace(o2.String()))
	unkEgg := clone("unk.egg", func(ents []map[string]json.RawMessage) error {
		ents[0]["mediaType"] = json.RawMessage("\"application/vnd.zerg.weights.v9+unknown\"")
		return nil
	})
	var o3, e3 strings.Builder
	if rc := zergEggGateMain(&o3, &e3, []string{"verify", unkEgg}); rc != exitOK {
		t.Fatalf("态② 闭集外 verify rc=%d stderr=%q", rc, e3.String())
	}
	if !strings.Contains(o3.String(), "未知 mediaType") {
		t.Fatalf("态② 闭集外未逐字印「未知 mediaType」：%q", o3.String())
	}
	t.Logf("态② 闭集外读数：%q", strings.TrimSpace(o3.String()))

	// 态③：恶意路径逐类 ⇒ 必拒且**点名规则**
	cases := []struct{ name, bad, rule string }{
		{"..", "../x", "path_dotdot"},
		{"绝对", "/etc/passwd", "path_absolute"},
		{"盘符", "C:/x", "path_windows_drive"},
		{"空段", "a//b", "path_not_normalized"},
		{"NUL", "a\x00b", "path_nul"},
		{"尾随点", "a.", "path_trailing_dot_space"},
		{"保留名", "con.txt", "path_windows_reserved"},
		{"反斜杠", "a\\b", "path_backslash"},
		{"非NFC", "cafe\u0301.json", "path_not_nfc"},
	}
	for i, c := range cases {
		badPath := c.bad
		name, rule := c.name, c.rule
		badEgg := clone("bad-"+itoaForLog(i)+".egg", func(ents []map[string]json.RawMessage) error {
			qb, err := json.Marshal(badPath)
			if err != nil {
				return err
			}
			ents[0]["path"] = json.RawMessage(qb)
			return nil
		})
		_, perr := zegParseIndex(badEgg)
		if perr == nil {
			t.Fatalf("态③ %s 恶意路径竟放行", name)
		}
		ref, ok := perr.(*zegRefusal)
		// ★ D9（GAP-20261015-25）：规则名移进 `Detail`（报话逐字 ＝ 13 档表 D9 行）——「点名规则」口径不变。
		if !ok || ref.Tier != "D9" || !strings.Contains(zegRefusalDetail(perr), rule) {
			t.Fatalf("态③ %s 未点名 D9/%s：%v", name, rule, perr)
		}
		t.Logf("态③ %s ⇒ rc≠0 · %v", name, perr)
	}

	// 态④：同键换语义（闭集内却与该段 `path` 推出的类不符）⇒ 必拒且点名档位
	semEgg := clone("sem.egg", func(ents []map[string]json.RawMessage) error {
		for i := range ents {
			var p string
			if json.Unmarshal(ents[i]["path"], &p) == nil && p == "config.json" {
				qb, jerr := json.Marshal(zegMediaLayer)
				if jerr != nil {
					return jerr
				}
				ents[i]["mediaType"] = json.RawMessage(qb)
			}
		}
		return nil
	})
	_, serr := zegParseIndex(semEgg)
	if serr == nil {
		t.Fatalf("态④ 同键换语义竟放行")
	}
	if ref, ok := serr.(*zegRefusal); !ok || ref.Tier != zegTierSurface {
		t.Fatalf("态④ 未点名 %s：%v", zegTierSurface, serr)
	}
	t.Logf("态④ ⇒ rc≠0 · %v", serr)

	// 负控 ①：同一份「未标」输入走**不审计**的旧形态 ⇒ 必须放行（＝本牙非永真）
	d, err := os.ReadFile(oldEgg)
	if err != nil {
		t.Fatal(err)
	}
	n := zegIdxLenOf(d)
	if n <= 0 {
		t.Fatal("负控 ① 取不到索引体（夹具坏）")
	}
	body := d[zegHeaderPrefix : zegHeaderPrefix+n]
	if _, aerr := zegCompatGateIndexBody(body); aerr != nil {
		t.Fatalf("负控 ① 旧形态在未标档上竟报错：%v", aerr)
	}
	t.Logf("负控 ① 旧形态（不审计）在同一份未标输入上 ⇒ 放行（牙非永真）")

	// 负控 ②：符号链接「只许相对链」四道具名拒（`GAP-20261018-05`）—— 四条各自点名、互不串名
	symAll := []string{"fs_symlink_absolute", "fs_symlink_escape", "fs_symlink_cycle", "fs_symlink_depth"}
	for _, c := range []struct {
		name string
		rule string
		mk   func(d string) error
	}{
		{"absolute", "fs_symlink_absolute", func(d string) error {
			return os.Symlink("/etc/hosts", filepath.Join(d, "link.bin"))
		}},
		{"escape", "fs_symlink_escape", func(d string) error {
			return os.Symlink("../outside.bin", filepath.Join(d, "link.bin"))
		}},
		{"cycle", "fs_symlink_cycle", func(d string) error {
			if err := os.Symlink("b.bin", filepath.Join(d, "a.bin")); err != nil {
				return err
			}
			return os.Symlink("a.bin", filepath.Join(d, "b.bin"))
		}},
		{"depth", "fs_symlink_depth", func(d string) error {
			for i := 0; i < 10; i++ {
				tgt := fmt.Sprintf("l%02d.bin", i+1)
				if i == 9 {
					tgt = "real.bin"
				}
				if err := os.Symlink(tgt, filepath.Join(d, fmt.Sprintf("l%02d.bin", i))); err != nil {
					return err
				}
			}
			return nil
		}},
	} {
		sd := filepath.Join(dir, "lsrc-"+c.name)
		if err := os.MkdirAll(sd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sd, "real.bin"), []byte("z"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := c.mk(sd); err != nil {
			t.Fatal(err)
		}
		_, lerr := zegPack(sd, filepath.Join(dir, "l-"+c.name+".egg"), 4096, "")
		if lerr == nil {
			t.Fatalf("负控 ②/%s 符号链接源件竟被打包", c.name)
		}
		if !strings.Contains(lerr.Error(), c.rule) {
			t.Fatalf("负控 ②/%s 拒了但没点名 %s：%v", c.name, c.rule, lerr)
		}
		for _, other := range symAll {
			if other != c.rule && strings.Contains(lerr.Error(), other) {
				t.Fatalf("负控 ②/%s 串名（本该 %s，报话里还有 %s）：%v", c.name, c.rule, other, lerr)
			}
		}
		t.Logf("负控 ②/%s ⇒ 拒[%s]", c.name, c.rule)
	}

	// 链尾：`--self-test` 四行（旧档 / 卵3① / 卵6 / 卵1）—— ★ 前三行必须**逐字未漂**
	var so, se strings.Builder
	if rc := zergEggGateMain(&so, &se, []string{"--self-test"}); rc != exitOK {
		t.Fatalf("--self-test rc=%d stderr=%q", rc, se.String())
	}
	for _, frag := range []string{
		"self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）",
		"self-test（卵3① 并行＋缓存）通过（7 项：3 正例 + 4 负例）",
		"self-test（卵6 版本与兼容）通过（4 态 ＋ 1 负控）",
		"self-test（卵1 格式与工具面）通过（4 态 ＋ 4 负控 ＋ 1 正控）",
		// ★ 第五行 = v2 布局（段表化 ＋ 引擎段在尾）—— ✗ 与前四行混行（前四行契约读数一字不动）
		"self-test（卵 v2 布局）通过（6 态 ＋ 2 负控）",
		// ★ 第六行 = 引擎闭包 · sysroot（`--engine-dir` ＋ 缺件即拒）—— ✗ 与前五行混行
		"self-test（引擎闭包 · sysroot）通过（5 态 ＋ 3 负控）",
	} {
		if !strings.Contains(so.String(), frag) {
			t.Fatalf("--self-test 缺行 %q：%q", frag, so.String())
		}
	}
	lines := strings.Split(strings.TrimRight(so.String(), "\n"), "\n")
	lineN := len(lines)
	// 自印 N ＋ 下界（抽空/少一行即 fail-closed）：四档历史面 ＋ v2 档 ＋ 闭包档 = **6 行**。
	const wantLines = 6
	if lineN < wantLines {
		t.Fatalf("--self-test 行数 = %d 低于下限 %d（抽空 ⇒ fail-closed）", lineN, wantLines)
	}
	if lineN != wantLines {
		t.Fatalf("--self-test 行数 = %d ≠ %d（六档必须分开印）", lineN, wantLines)
	}
	t.Logf("--self-test 末行（闭包那行）= %q", lines[lineN-1])
}

// ---- ⑥ ★ 引擎闭包 · sysroot（卵沙箱第 3 步）四态亲测 + 两枚负控 --------------------------------

// TestZergEggEngineClosureSysroot —— `--engine-dir`（把引擎目录连同其 .so 收进蛋）＋ 闭包自检。
//
// 四态**全走真命令面**（`zergEggGateMain`：pack / info）—— ✗ 不只调内部函数：
//
//	① 完整闭包（含 soname 链）⇒ pack rc=0 · stdout 印「全命中 ✓」· `info` 里 .so 段落在「引擎段」；
//	② 抽掉一个**被 NEEDED** 的 .so ⇒ pack 失败且逐字点名「缺 `lib…` · 被 … 需要」；
//	③ 抽掉 loader（`ld-linux*`）⇒ pack 失败且点名 ld-linux*；
//	④ `--engine-dir` 给到非 pack 子命令 ⇒ 退 2（✗ 静默忽略）。
//	负控① 同一份缺件树走**不审计**的 v1 `zegPack` ⇒ 成功（＝②③ 的红来自闭包自检，不是路径/mode 规则）；
//	负控② 构建树的 0775 mode ⇒ 归一后仍能打包（且不是靠放宽安全子集：蛋内段恒 0644/0755）。
func TestZergEggEngineClosureSysroot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	eng := filepath.Join(dir, "build", "bin")
	for _, d := range []string{src, eng} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string, b []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(p, b, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil { // 防 umask 改写
			t.Fatal(err)
		}
	}
	// 源目录：权重/配方（无动态件）—— 打蛋的「正常内容」。
	write(filepath.Join(src, "recipe.json"), []byte("{\"n_gpu_layers\": 99}\n"), 0o644)
	write(filepath.Join(src, "weights.gguf"), []byte("GGUF-fake\n"), 0o644)
	// 引擎目录：exe NEEDED 里含 soname `libggml.so.0`（真件只有 libggml.so.0.1 ＋ 一条相对链）
	write(filepath.Join(eng, "llama-server"), zegSynthELF([]string{
		"libggml.so.0", "libstdc++.so.6", zegClosureLibc, zegClosureLibm,
	}), 0o775) // ★ 构建树的 0775：负控②
	write(filepath.Join(eng, "libggml.so.0.1"), zegSynthELF([]string{zegClosureLibc}), 0o775)
	if err := os.Symlink("libggml.so.0.1", filepath.Join(eng, "libggml.so.0")); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(eng, "libstdc++.so.6"), []byte("synth-stdcxx"), 0o755)
	// 恒必带三件（真闭包里它们来自 /lib/x86_64-linux-gnu，这里由构建脚本显式交进来）
	write(filepath.Join(eng, zegClosureLibc), []byte("synth-libc"), 0o755)
	write(filepath.Join(eng, zegClosureLibm), []byte("synth-libm"), 0o755)
	write(filepath.Join(eng, "ld-linux-x86-64.so.2"), []byte("synth-loader"), 0o755)

	run := func(argv ...string) (int, string, string) {
		var so, se strings.Builder
		rc := zergEggGateMain(&so, &se, argv)
		return rc, so.String(), se.String()
	}

	// ---- 态① 完整闭包 ⇒ rc=0，且 info 里 .so 落在「引擎段」 ----
	egg := filepath.Join(dir, "ok.egg")
	rc, out, errS := run("pack", src, egg, "--engine-dir", eng)
	if rc != exitOK {
		t.Fatalf("态① pack rc=%d out=%q err=%q", rc, out, errS)
	}
	for _, frag := range []string{"引擎闭包自检", "全命中 ✓", "引擎目录 1 个已收进蛋"} {
		if !strings.Contains(out, frag) {
			t.Fatalf("态① pack 输出缺 %q：%q", frag, out)
		}
	}
	rc, out, errS = run("info", egg)
	if rc != exitOK {
		t.Fatalf("态① info rc=%d out=%q err=%q", rc, out, errS)
	}
	for _, frag := range []string{"engine/bin/libggml.so.0 ", "engine/bin/ld-linux-x86-64.so.2", "seg=引擎段"} {
		if !strings.Contains(out, frag) {
			t.Fatalf("态① info 缺 %q（.so 段没落进引擎段？）：%q", frag, out)
		}
	}
	// soname 靠**链名**命中：链段必须真的进段表（size=0 ＋ linkTarget）
	if !strings.Contains(out, "linkTarget=libggml.so.0.1") {
		t.Fatalf("态① soname 链段没进段表：%q", out)
	}

	// ---- 负控② 0775 的构建树件 ⇒ 蛋内段恒 0644/0755（归一而非放宽安全子集；`zegModeSafeSet` 一字未改）----
	idx, perr := zegParseIndex(egg)
	if perr != nil {
		t.Fatalf("负控② 读不回索引：%v", perr)
	}
	seen775 := 0
	for _, e := range idx.Entries {
		if !zegModeSafeSet[e.Mode] {
			t.Fatalf("负控② 蛋内段 %q 的 mode=%#o 不在安全子集（归一没生效 / 子集被放宽）", e.Path, e.Mode)
		}
		if e.Path == "engine/bin/libggml.so.0.1" || e.Path == "engine/bin/llama-server" {
			seen775++
			if e.Mode != 0o755 {
				t.Fatalf("负控② 来源 0775 的可执行段 %q 归一成 %#o（要 0755）", e.Path, e.Mode)
			}
		}
	}
	if seen775 != 2 {
		t.Fatalf("负控② 该有 2 个来自 0775 的可执行段，实得 %d", seen775)
	}

	// ---- 态② 抽掉被 NEEDED 的 libggml.so.0（链）⇒ 名点名拒 ----
	bad := filepath.Join(dir, "bad.egg")
	if err := os.Remove(filepath.Join(eng, "libggml.so.0")); err != nil {
		t.Fatal(err)
	}
	rc, out, errS = run("pack", src, bad, "--engine-dir", eng)
	if rc == exitOK {
		t.Fatalf("态② 缺一个 NEEDED 的 soname 竟打成了蛋：%q", out)
	}
	if !strings.Contains(errS, "缺 `libggml.so.0`") || !strings.Contains(errS, "llama-server") {
		t.Fatalf("态② 报话没点名缺件与发起方：%q", errS)
	}
	if _, serr := os.Stat(bad); serr == nil {
		t.Fatalf("态② 失败档竟留下了半截蛋 %s", bad)
	}
	// 负控① 同一份缺件树走**不审计**的 v1 打包 ⇒ 必须成功（＝红来自本检）
	if _, verr := zegPack(src, filepath.Join(dir, "neg1.egg"), 4096, "2000-01-01T00:00:00Z"); verr != nil {
		t.Fatalf("负控① 不审计的旧档在**源目录**上就失败了（那本检的红分不清来源）：%v", verr)
	}

	// ---- 态③ 抽掉 loader ⇒ 点名 ld-linux* ----
	if err := os.Remove(filepath.Join(eng, "ld-linux-x86-64.so.2")); err != nil {
		t.Fatal(err)
	}
	rc, _, errS = run("pack", src, filepath.Join(dir, "noloader.egg"), "--engine-dir", eng)
	if rc == exitOK {
		t.Fatalf("态③ 缺 loader 竟打成了蛋")
	}
	if !strings.Contains(errS, "缺 `"+zegClosureLoaderPrefix+"*`") {
		t.Fatalf("态③ 报话没点名 loader：%q", errS)
	}

	// ---- 态④ `--engine-dir` 给到非 pack 子命令 ⇒ 退 2（✗ 静默忽略）----
	rc, _, errS = run("info", egg, "--engine-dir", eng)
	if rc != exitUsage {
		t.Fatalf("态④ --engine-dir 配 info 竟 rc=%d（要 2）", rc)
	}
	if !strings.Contains(errS, "只对 `pack` 有效") {
		t.Fatalf("态④ 报话没点名原因：%q", errS)
	}
}

// ---- ⑦ ★ GAP-20261018-14：声明要 GPU 的蛋缺 dlopen 层 ROCm 运行时件（COMGR）⇒ 打蛋即具名拒 ----

// TestZergEggEngineClosureGPU 走**真命令面**（`zergEggGateMain` 的 pack）钉住这条牙：
//
//	① 引擎里有 HIP 运行时（`libamdhip64.so.7`）、蛋里没有 `libamd_comgr.so.3` ⇒ pack 必须失败，
//	   且**逐字点名** `libamd_comgr.so.3`（DT_NEEDED 面在这一态里全命中 —— 只有 dlopen 面判据能判红）；
//	② 把 COMGR（连同它 NEEDED 的 `libLLVM.so.21.1`）补进引擎目录 ⇒ pack 成功（证判据不是「见 GPU 恒红」）；
//	③ 纯 CPU 蛋（无 HIP 运行时、无 rocblas 目录）⇒ 不要求 COMGR（不误伤）。
//
// 病灶真身（edge-node 真跑 2026-10-18）：设备节点与 /sys 全好、闭包自检「全命中 ✓」，而空根里逐字
// `no ROCm-capable device is detected` —— 缺的就是 dt-needed 面看不见的 COMGR。
func TestZergEggEngineClosureGPU(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	gpu := filepath.Join(dir, "hip", "bin")
	cpu := filepath.Join(dir, "cpu", "bin")
	for _, d := range []string{src, gpu, cpu} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string, b []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(p, b, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(src, "recipe.json"), []byte("{\"n_gpu_layers\": 99}\n"), 0o644)
	write(filepath.Join(src, "weights.gguf"), []byte("GGUF-fake\n"), 0o644)

	// GPU 引擎：exe NEEDED 里有 HIP 运行时；HIP 自己 NEEDED 里**没有** COMGR（真件就是 dlopen 拿它）。
	write(filepath.Join(gpu, "llama-server"), zegSynthELF([]string{
		zegClosureHIPPrefix + ".7", zegClosureLibc, zegClosureLibm,
	}), 0o755)
	write(filepath.Join(gpu, zegClosureHIPPrefix+".7"), zegSynthELF([]string{zegClosureLibc}), 0o755)
	write(filepath.Join(gpu, zegClosureLibc), []byte("synth-libc"), 0o755)
	write(filepath.Join(gpu, zegClosureLibm), []byte("synth-libm"), 0o755)
	write(filepath.Join(gpu, "ld-linux-x86-64.so.2"), []byte("synth-loader"), 0o755)

	// 纯 CPU 引擎：同名 exe 但不碰 HIP（负控③）。
	write(filepath.Join(cpu, "llama-server"), zegSynthELF([]string{zegClosureLibc, zegClosureLibm}), 0o755)
	write(filepath.Join(cpu, zegClosureLibc), []byte("synth-libc"), 0o755)
	write(filepath.Join(cpu, zegClosureLibm), []byte("synth-libm"), 0o755)
	write(filepath.Join(cpu, "ld-linux-x86-64.so.2"), []byte("synth-loader"), 0o755)

	run := func(argv ...string) (int, string, string) {
		var so, se strings.Builder
		rc := zergEggGateMain(&so, &se, argv)
		return rc, so.String(), se.String()
	}

	// ① 缺 COMGR ⇒ 红且逐字点名（并说清 dlopen 面这条判据）
	eggBad := filepath.Join(dir, "gpu-bad.egg")
	rc, out, errS := run("pack", src, eggBad, "--engine-dir", gpu)
	if rc == exitOK {
		t.Fatalf("① 声明要 GPU 却缺 COMGR 竟打成了蛋：%q", out)
	}
	if !strings.Contains(errS, "缺 `"+zegClosureComgrName+"`") {
		t.Fatalf("① 报话没点名 %s：%q", zegClosureComgrName, errS)
	}
	if !strings.Contains(errS, "dlopen") {
		t.Fatalf("① 报话没点明「dlopen 面看不见它」（否则下一个人还会去查 DT_NEEDED）：%q", errS)
	}
	if _, serr := os.Stat(eggBad); serr == nil {
		t.Fatalf("① 失败档竟留下了半截蛋 %s", eggBad)
	}

	// ② 补上 COMGR ＋ libLLVM ⇒ 绿
	write(filepath.Join(gpu, zegClosureComgrName), zegSynthELF([]string{"libLLVM.so.21.1", zegClosureLibc}), 0o755)
	write(filepath.Join(gpu, "libLLVM.so.21.1"), zegSynthELF([]string{zegClosureLibc}), 0o755)
	rc, out, errS = run("pack", src, filepath.Join(dir, "gpu-ok.egg"), "--engine-dir", gpu)
	if rc != exitOK {
		t.Fatalf("② 补上 COMGR 后仍打不成蛋 rc=%d out=%q err=%q", rc, out, errS)
	}
	if !strings.Contains(out, "全命中 ✓") {
		t.Fatalf("② 过档读数缺「全命中 ✓」：%q", out)
	}

	// ③ 纯 CPU 蛋 ⇒ 不要求 COMGR（同一条判据不误伤）
	rc, out, errS = run("pack", src, filepath.Join(dir, "cpu.egg"), "--engine-dir", cpu)
	if rc != exitOK {
		t.Fatalf("③ 纯 CPU 蛋被要求 COMGR（误伤）rc=%d out=%q err=%q", rc, out, errS)
	}
}

// TestZergEggWeightsDeclGate —— GAP-20261018-16 的**打蛋/校验两面**自检。
//
// 口径（与 backend 侧 `eggWeightSource` 同源）：从蛋孵化时空间内 `/models/<基名>` 的挂载源
// = 蛋内 `models/<基名>` ⇒ 蛋**离开打包机之前**就该是自包含的。声明只能来自源树自带的
// `recipe.json`（打包端不认识名册）：
//
//	{"weights": ["<基名>", …], "require_models": true}
//
// 五态：① 声明齐且件齐 ⇒ 绿；② 声明了却缺件 ⇒ 具名拒（点名 models/<基名>）；③ require_models 而整层缺 ⇒ 具名拒；
// ④ 无这两个键（真机 real10 的自由文本 recipe）⇒ 逐字不适用（零回归）；⑤ 校验面：索引里声明了却缺件 ⇒ 具名拒。
func TestZergEggWeightsDeclGate(t *testing.T) {
	mkTree := func(t *testing.T, recipe string, withModels bool) string {
		t.Helper()
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, "recipe.json"), []byte(recipe), 0o644); err != nil {
			t.Fatal(err)
		}
		if withModels {
			if err := os.MkdirAll(filepath.Join(src, "models"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(src, "models", "w1.gguf"), []byte("GGUF-w1"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return src
	}
	run := func(argv ...string) (int, string, string) {
		var so, se strings.Builder
		rc := zergEggGateMain(&so, &se, argv)
		return rc, so.String(), se.String()
	}

	t.Run("① 声明齐且件齐 ⇒ 绿", func(t *testing.T) {
		src := mkTree(t, `{"weights":["w1.gguf"]}`, true)
		rc, out, errS := run("pack", src, filepath.Join(t.TempDir(), "a.egg"))
		if rc != exitOK {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errS)
		}
	})

	t.Run("② 声明了却缺件 ⇒ 具名拒", func(t *testing.T) {
		src := mkTree(t, `{"weights":["w1.gguf","w2.gguf"]}`, true)
		egg := filepath.Join(t.TempDir(), "b.egg")
		rc, out, errS := run("pack", src, egg)
		if rc == exitOK {
			t.Fatalf("缺件必须拒打包（out=%q）", out)
		}
		if !strings.Contains(errS, "打蛋面") || !strings.Contains(errS, "models/w2.gguf") {
			t.Fatalf("拒的理由应点名打蛋面与缺的 models/w2.gguf，实得：%q / %q", out, errS)
		}
		if _, serr := os.Stat(egg); serr == nil {
			t.Fatalf("拒打包不许留下半截蛋：%s", egg)
		}
	})

	t.Run("③ require_models 而整层缺 ⇒ 具名拒", func(t *testing.T) {
		src := mkTree(t, `{"require_models":true}`, false)
		rc, _, errS := run("pack", src, filepath.Join(t.TempDir(), "c.egg"))
		if rc == exitOK {
			t.Fatal("声明了 require_models 却没有 models/ 层必须拒")
		}
		if !strings.Contains(errS, "没有 models/ 这一层") {
			t.Fatalf("拒的理由应点名 models/ 整层缺失，实得 %q", errS)
		}
	})

	t.Run("④ 无声明键（真机形态）⇒ 不适用、行为不变", func(t *testing.T) {
		src := mkTree(t, `{"kind":"egg10-real","note":"weights=… 4 shards"}`, false)
		rc, out, errS := run("pack", src, filepath.Join(t.TempDir(), "d.egg"))
		if rc != exitOK {
			t.Fatalf("自由文本 recipe 不该被新判据拦住：rc=%d out=%q err=%q", rc, out, errS)
		}
	})

	t.Run("⑤ 校验面：索引声明了却缺件 ⇒ 具名拒", func(t *testing.T) {
		dir := t.TempDir()
		egg := filepath.Join(dir, "fake.egg")
		rec := []byte(`{"weights":["w9.gguf"]}`)
		if err := os.WriteFile(egg, rec, 0o644); err != nil {
			t.Fatal(err)
		}
		ix := &zegIndex{Entries: []zegEntryJSON{
			{Path: "recipe.json", Off: 0, Size: int64(len(rec))},
			{Path: "models/w1.gguf", Off: 0, Size: 0},
		}}
		err := zegWeightsDeclGateEgg(egg, ix)
		if err == nil {
			t.Fatal("校验面必须拒：声明了 w9.gguf 而索引里没有 models/w9.gguf")
		}
		if !strings.Contains(err.Error(), "校验面") || !strings.Contains(err.Error(), "models/w9.gguf") {
			t.Fatalf("拒的理由应点名校验面与缺件，实得：%v", err)
		}
		// 壳索引（缓存命中档无 entries）⇒ 不给结论也不判红
		if err := zegWeightsDeclGateEgg(egg, &zegIndex{}); err != nil {
			t.Fatalf("壳索引不该判红：%v", err)
		}
	})
}

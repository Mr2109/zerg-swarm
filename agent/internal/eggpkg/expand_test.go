// expand_test.go —— 蛋展开器的**真夹具**验收（第 6 步 ①–③ ＋ 与 core 写端的跨实现对拍）。
//
// 夹具形状与 core 侧 `zegTV2BuildTree` 同源（v2 夹具蛋：9 段 ＝ 1 MiB 假 GGUF ＋ 引擎 4 段含
// **两级链 / 三级名** ＋ 配方 / 契约 / 系统库 3 段），因为「读写两端必须同源」这条只能靠**同一形状**
// 在两个实现上跑才证得了：
//
//	① `TestExpandEgg_RoundTrip`      —— 逐件 sha256 / 0755 / `readlink` 两跳逐字 / 字节总数；
//	② `TestExpandEgg_Idempotent`     —— 第二次调用**一个字节都不写**（靠 `.complete`）；
//	③ `TestExpandEgg_Refuse*`        —— 三负控：改一段字节 / 目标目录有内容 / 段表塞 `../..`；
//	④ `TestExpandEgg_RealPackerParity` —— 用**真写端**（`bin/zerg cocoon pack`）打蛋再展开，
//	   并把我的头/表读数与 `cocoon info` 的读数逐格对拍（跨实现同源）。缺 `bin/zerg` ⇒ 如实跳过。
package eggpkg

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---- 测试侧的最小写端（只为造夹具；★ 与 core 写端同源由 ④ 那枚对拍用例担保）------------

type eggTestSeg struct {
	Path   string
	Data   []byte
	Mode   uint32
	Link   string
	Medium string
}

// eggTestPack 按 v2 布局写一枚蛋：8 KiB 双槽头 ＋ 数据段 ＋ 64 KiB 对齐的尾段表。
func eggTestPack(t *testing.T, eggPath string, segs []eggTestSeg, align int64) {
	t.Helper()
	if align == 0 {
		align = 4096
	}
	out, err := os.Create(eggPath)
	if err != nil {
		t.Fatalf("建蛋 %s：%v", eggPath, err)
	}
	defer out.Close()
	if _, err := out.WriteAt(make([]byte, eggHeaderBytes), 0); err != nil {
		t.Fatalf("头占位：%v", err)
	}
	cur := eggAlignUp(eggHeaderBytes, align)
	entries := make([]EggSegment, 0, len(segs))
	var uuid [16]byte
	copy(uuid[:], []byte("egg6-fixture-001"))
	for _, s := range segs {
		cur = eggAlignUp(cur, align)
		e := EggSegment{Path: s.Path, Off: cur, Size: int64(len(s.Data)), Mode: s.Mode, MediaType: s.Medium}
		if s.Link != "" {
			e.Size = 0
			e.SHA256 = eggEmptySHA
			e.LinkTarget = s.Link
		} else {
			sum := sha256.Sum256(s.Data)
			e.SHA256 = hex.EncodeToString(sum[:])
			if _, err := out.WriteAt(s.Data, cur); err != nil {
				t.Fatalf("写段 %s：%v", s.Path, err)
			}
		}
		entries = append(entries, e)
		cur += e.Size
	}
	fileLen := cur
	tableOff := eggAlignUp(fileLen, eggTableAlign)
	table := map[string]interface{}{
		"magic":      eggMagicV2,
		"version":    eggLayoutV2,
		"align":      align,
		"tool":       map[string]string{"name": "eggpkg-test", "version": "1.0.0"},
		"created_at": "2026-10-18T00:00:00Z",
		"entries":    entries,
	}
	body, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("段表序列化：%v", err)
	}
	if _, err := out.WriteAt(body, tableOff); err != nil {
		t.Fatalf("写段表：%v", err)
	}
	tsha := sha256.Sum256(body)
	// 槽 0 = 基线 gen0，槽 1 = 现行 gen1（与 core 写端同序：读端取 gen 大者）。
	for slot, gen := range []int64{0, 1} {
		b := make([]byte, eggSlotBytes)
		copy(b[eggOffMagic:], eggMagicV2)
		binary.LittleEndian.PutUint32(b[eggOffLayout:], uint32(eggLayoutV2))
		binary.LittleEndian.PutUint64(b[eggOffGen:], uint64(gen))
		binary.LittleEndian.PutUint64(b[eggOffFileLen:], uint64(fileLen))
		binary.LittleEndian.PutUint64(b[eggOffTableOff:], uint64(tableOff))
		binary.LittleEndian.PutUint64(b[eggOffTableLen:], uint64(len(body)))
		binary.LittleEndian.PutUint64(b[eggOffAlign:], uint64(align))
		copy(b[eggOffUUID:], uuid[:])
		copy(b[eggOffTableSHA:], tsha[:])
		binary.LittleEndian.PutUint32(b[eggOffCRC:], crc32.Checksum(b[:eggCRCSpan], crc32.MakeTable(crc32.Castagnoli)))
		if _, err := out.WriteAt(b, int64(slot*eggSlotBytes)); err != nil {
			t.Fatalf("写头槽 %d：%v", slot, err)
		}
	}
	if err := out.Sync(); err != nil {
		t.Fatalf("蛋 fsync：%v", err)
	}
}

// eggTestFixture —— core `zegTV2BuildTree` 的同形夹具（9 段 · 1 MiB 假 GGUF · 0755 脚本 · 两级链）。
func eggTestFixture(t *testing.T) []eggTestSeg {
	t.Helper()
	gguf := make([]byte, 1<<20)
	copy(gguf, []byte("GGUF"))
	for i := 4; i < len(gguf); i++ {
		gguf[i] = byte(i % 251)
	}
	return []eggTestSeg{
		{Path: "config.json", Data: []byte(`{"arch":"v2-test"}`), Mode: 0o644},
		{Path: "contract/policy.json", Data: []byte(`{"policy":1}`), Mode: 0o644},
		{Path: "bin/run.sh", Data: []byte("#!/bin/sh\nexec ./engine/zerg-engine\n"), Mode: 0o755},
		{Path: "engine/zerg-engine", Data: []byte("#!/bin/sh\necho engine-v1\n"), Mode: 0o755},
		{Path: "engine/libfoo.so.1.12", Data: []byte("ELF-foo-1.12"), Mode: 0o644},
		{Path: "engine/libfoo.so.1", Mode: 0o755, Link: "libfoo.so.1.12"},
		{Path: "engine/libfoo.so", Mode: 0o755, Link: "libfoo.so.1"},
		{Path: "model/demo.gguf", Data: gguf, Mode: 0o644},
		{Path: "sysroot/lib/libc.so.6", Data: []byte("stub-libc"), Mode: 0o644},
	}
}

func eggTestWrite(t *testing.T, segs []eggTestSeg) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture.egg")
	eggTestPack(t, p, segs, 4096)
	return p
}

// TestExpandEgg_RoundTrip —— ① 夹具蛋真展开：逐件 sha / 0755 / 两跳链 / 字节总数。
func TestExpandEgg_RoundTrip(t *testing.T) {
	segs := eggTestFixture(t)
	egg := eggTestWrite(t, segs)
	dst := filepath.Join(t.TempDir(), "expanded")

	ix, err := ReadEggIndex(egg)
	if err != nil {
		t.Fatalf("ReadEggIndex：%v", err)
	}
	if len(ix.Entries) != len(segs) {
		t.Fatalf("段数 %d ≠ 夹具 %d", len(ix.Entries), len(segs))
	}
	res, err := ExpandEgg(egg, dst)
	if err != nil {
		t.Fatalf("ExpandEgg：%v", err)
	}
	if res.Skipped || res.Files+res.Symlinks != len(segs) || res.Symlinks != 2 {
		t.Fatalf("展开读数不对：%+v", res)
	}
	var wantBytes int64
	for _, s := range segs {
		wantBytes += int64(len(s.Data))
	}
	if res.Bytes != wantBytes {
		t.Fatalf("落盘字节总数 %d ≠ 段表 size 之和 %d", res.Bytes, wantBytes)
	}
	if ix.DataBytes != wantBytes {
		t.Fatalf("段表 DataBytes=%d ≠ 夹具 %d", ix.DataBytes, wantBytes)
	}
	// 逐件 sha256（✗ 不抽样：9 段一件不落）
	var onDisk int64
	for _, e := range ix.Entries {
		full := filepath.Join(dst, filepath.FromSlash(e.Path))
		if e.IsSymlink() {
			li, lerr := os.Lstat(full)
			if lerr != nil || li.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("链段 %s 没落成符号链接（err=%v）", e.Path, lerr)
			}
			got, rerr := os.Readlink(full)
			if rerr != nil || got != e.LinkTarget {
				t.Fatalf("链段 %s 的链名 %q ≠ 段表 %q（err=%v）", e.Path, got, e.LinkTarget, rerr)
			}
			continue
		}
		b, rerr := os.ReadFile(full)
		if rerr != nil {
			t.Fatalf("读落盘件 %s：%v", e.Path, rerr)
		}
		sum := sha256.Sum256(b)
		got := hex.EncodeToString(sum[:])
		if got != e.SHA256 {
			t.Fatalf("段 %s 落盘 sha256=%s ≠ 段表 %s", e.Path, got[:16], e.SHA256[:16])
		}
		onDisk += int64(len(b))
		li, lerr := os.Lstat(full)
		if lerr != nil {
			t.Fatalf("lstat %s：%v", e.Path, lerr)
		}
		if uint32(li.Mode().Perm()) != e.Mode&0o777 {
			t.Fatalf("段 %s 的 mode=%#o ≠ 段表 %#o", e.Path, li.Mode().Perm(), e.Mode&0o777)
		}
		t.Logf("✓ %-32s size=%-8d sha256=%s mode=%#o", e.Path, e.Size, got[:16], li.Mode().Perm())
	}
	if onDisk != wantBytes {
		t.Fatalf("盘上字节 %d ≠ 段表 size 之和 %d", onDisk, wantBytes)
	}
	// 两跳链：readlink 逐字（a → b → 实件）
	for _, c := range []struct{ rel, want string }{
		{"engine/libfoo.so", "libfoo.so.1"},
		{"engine/libfoo.so.1", "libfoo.so.1.12"},
	} {
		got, rerr := os.Readlink(filepath.Join(dst, filepath.FromSlash(c.rel)))
		if rerr != nil || got != c.want {
			t.Fatalf("readlink %s = %q（要 %q，err=%v）", c.rel, got, c.want, rerr)
		}
	}
	// 0755 保留（脚本 ＋ 引擎）
	for _, rel := range []string{"bin/run.sh", "engine/zerg-engine"} {
		li, lerr := os.Lstat(filepath.Join(dst, filepath.FromSlash(rel)))
		if lerr != nil || li.Mode().Perm() != 0o755 {
			t.Fatalf("%s 的 mode 不是 0755：%v / %v", rel, li.Mode(), lerr)
		}
	}
	// `.complete`：整件 sha ＋ 段数
	var rc eggReceipt
	b, rerr := os.ReadFile(filepath.Join(dst, eggMarkerComplete))
	if rerr != nil {
		t.Fatalf("读 `.complete`：%v", rerr)
	}
	if jerr := json.Unmarshal(b, &rc); jerr != nil {
		t.Fatalf("`.complete` 不是 JSON：%v", jerr)
	}
	sum, _ := eggSha256File(egg)
	if rc.EggSHA256 != sum || rc.Segments != len(segs) || rc.Bytes != wantBytes || rc.Gen != 1 {
		t.Fatalf("`.complete` 读数不对：%+v（整件 sha=%s）", rc, sum[:16])
	}
	t.Logf("① 展开：段 %d（文件 %d / 链 %d）· 字节 %d · 整件 sha=%s · 段表 sha=%s",
		res.Segments, res.Files, res.Symlinks, res.Bytes, rc.EggSHA256[:16], res.TableSHA256[:16])
}

// TestExpandEgg_Idempotent —— ② 再调一次：不重写、不报错（靠 `.complete`）。
func TestExpandEgg_Idempotent(t *testing.T) {
	egg := eggTestWrite(t, eggTestFixture(t))
	dst := filepath.Join(t.TempDir(), "expanded")
	first, err := ExpandEgg(egg, dst)
	if err != nil {
		t.Fatalf("第一次展开：%v", err)
	}
	type stamp struct {
		mod  int64
		sha  string
		size int64
	}
	snap := func() map[string]stamp {
		m := map[string]stamp{}
		err := filepath.Walk(dst, func(p string, fi os.FileInfo, werr error) error {
			if werr != nil || fi.IsDir() {
				return werr
			}
			rel, _ := filepath.Rel(dst, p)
			st := stamp{mod: fi.ModTime().UnixNano(), size: fi.Size()}
			if fi.Mode()&os.ModeSymlink == 0 {
				sum, _ := eggSha256File(p)
				st.sha = sum
			}
			m[filepath.ToSlash(rel)] = st
			return nil
		})
		if err != nil {
			t.Fatalf("快照 %s：%v", dst, err)
		}
		return m
	}
	before := snap()
	rcBefore, _ := os.ReadFile(filepath.Join(dst, eggMarkerComplete))

	second, err := ExpandEgg(egg, dst)
	if err != nil {
		t.Fatalf("第二次展开应幂等跳过，实得错误：%v", err)
	}
	if !second.Skipped {
		t.Fatalf("第二次没有走跳过档：%+v", second)
	}
	if second.EggSHA256 != first.EggSHA256 || second.Segments != first.Segments {
		t.Fatalf("跳过档读数与首趟不一致：%+v vs %+v", second, first)
	}
	after := snap()
	if len(after) != len(before) {
		t.Fatalf("跳过档改了件数 %d → %d", len(before), len(after))
	}
	for k, v := range before {
		w := after[k]
		if v != w {
			t.Fatalf("跳过档重写了 %s（mtime/sha/size %+v → %+v）", k, v, w)
		}
	}
	rcAfter, _ := os.ReadFile(filepath.Join(dst, eggMarkerComplete))
	if string(rcBefore) != string(rcAfter) {
		t.Fatalf("跳过档改了 `.complete`")
	}
	t.Logf("② 幂等：第二次 Skipped=true · %d 件 mtime/sha 逐件未变 · `.complete` 未重写", len(before))
}

// TestExpandEgg_SingleWriterLock —— `.expanding` 在 ⇒ 拒（✗ 两路同写）；只剩 `.failed` ⇒ 清掉重来。
func TestExpandEgg_SingleWriterLock(t *testing.T) {
	egg := eggTestWrite(t, eggTestFixture(t))
	dst := filepath.Join(t.TempDir(), "expanded")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dst, eggMarkerExpanding)
	if err := os.WriteFile(lock, []byte("pid=999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExpandEgg(egg, dst); err == nil || !strings.Contains(err.Error(), "另一趟展开正在跑") {
		t.Fatalf("有锁竟没拒（err=%v）", err)
	}
	if b, rerr := os.ReadFile(lock); rerr != nil || !strings.Contains(string(b), "pid=") {
		t.Fatalf("拒的时候把别人的锁清了：%q / %v", string(b), rerr)
	}
	// 恢复路：确认无人跑 ⇒ 人工删锁 ⇒ 再展开成功。
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	// 只剩 `.failed`（上一趟失败痕迹）⇒ 清掉重来（✗ 不许它把目录永久锁死）。
	if err := os.WriteFile(filepath.Join(dst, eggMarkerFailed), []byte(`{"reason":"上一趟"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExpandEgg(egg, dst); err != nil {
		t.Fatalf("只有 .failed 时应可重来：%v", err)
	}
	if _, serr := os.Stat(filepath.Join(dst, eggMarkerFailed)); !os.IsNotExist(serr) {
		t.Fatalf("成功后 `.failed` 还在")
	}
	t.Logf("单写者锁：锁在 ⇒ 拒且不替人清锁；删锁 ＋ 只余 .failed ⇒ 重来成功")
}

// TestExpandEgg_RefuseTamperedSegment —— ③a 改一段字节 ⇒ 拒且具名点名该件，✗ 不留半截目录。
func TestExpandEgg_RefuseTamperedSegment(t *testing.T) {
	segs := eggTestFixture(t)
	egg := eggTestWrite(t, segs)
	ix, err := ReadEggIndex(egg)
	if err != nil {
		t.Fatalf("ReadEggIndex：%v", err)
	}
	// 改「model/demo.gguf」段中间一个字节（★ 只改载荷，不改段表 ⇒ 只有 sha 复核能抓到）。
	var victim EggSegment
	for _, e := range ix.Entries {
		if e.Path == "model/demo.gguf" {
			victim = e
		}
	}
	if victim.Path == "" {
		t.Fatalf("夹具里找不到 model/demo.gguf 段")
	}
	f, err := os.OpenFile(egg, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("打开蛋：%v", err)
	}
	one := make([]byte, 1)
	if _, err := f.ReadAt(one, victim.Off+victim.Size/2); err != nil {
		t.Fatalf("读目标字节：%v", err)
	}
	one[0] ^= 0xff
	if _, err := f.WriteAt(one, victim.Off+victim.Size/2); err != nil {
		t.Fatalf("改目标字节：%v", err)
	}
	f.Close()

	dst := filepath.Join(t.TempDir(), "expanded")
	_, err = ExpandEgg(egg, dst)
	if err == nil {
		t.Fatalf("改过一段字节竟展开成功")
	}
	msg := err.Error()
	if !strings.Contains(msg, victim.Path) || !strings.Contains(msg, "sha256") {
		t.Fatalf("报话没点名件与判据：%v", msg)
	}
	if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
		t.Fatalf("失败后留下了半截目录 %s（serr=%v）", dst, serr)
	}
	t.Logf("③a 拒：%v", msg)
}

// TestExpandEgg_RefuseNonEmptyDst —— ③b 目标目录已存在且有文件 ⇒ 拒，且原文件不动。
func TestExpandEgg_RefuseNonEmptyDst(t *testing.T) {
	egg := eggTestWrite(t, eggTestFixture(t))
	dst := filepath.Join(t.TempDir(), "occupied")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dst, "keep.txt")
	if err := os.WriteFile(keep, []byte("别人的东西\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExpandEgg(egg, dst)
	if err == nil || !strings.Contains(err.Error(), "有内容") {
		t.Fatalf("目标目录有内容竟没拒（err=%v）", err)
	}
	b, rerr := os.ReadFile(keep)
	if rerr != nil || string(b) != "别人的东西\n" {
		t.Fatalf("拒绝时动了盘上的件：%q / %v", string(b), rerr)
	}
	des, _ := os.ReadDir(dst)
	if len(des) != 1 {
		t.Fatalf("拒绝时往目标目录里撒了东西：%d 项", len(des))
	}
	t.Logf("③b 拒：%v", err)
}

// TestExpandEgg_RefuseTraversal —— ③c 段表塞 `../../etc/x` ⇒ 拒且报穿越（绝对路径 / 出根链同理）。
func TestExpandEgg_RefuseTraversal(t *testing.T) {
	cases := []struct {
		name string
		seg  eggTestSeg
		want string
	}{
		{"点段穿越", eggTestSeg{Path: "../../etc/x", Data: []byte("pwn"), Mode: 0o644}, "path_dotdot"},
		{"绝对路径", eggTestSeg{Path: "/etc/x", Data: []byte("pwn"), Mode: 0o644}, "path_absolute"},
		{"链越界", eggTestSeg{Path: "a/link", Mode: 0o755, Link: "../../../etc/passwd"}, "fs_symlink_escape"},
		{"链绝对", eggTestSeg{Path: "a/link2", Mode: 0o755, Link: "/etc/passwd"}, "fs_symlink_absolute"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			segs := []eggTestSeg{
				{Path: "ok.txt", Data: []byte("fine"), Mode: 0o644},
				c.seg,
			}
			egg := eggTestWrite(t, segs)
			dst := filepath.Join(t.TempDir(), "expanded")
			_, err := ExpandEgg(egg, dst)
			if err == nil {
				t.Fatalf("%s 竟展开成功", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%s 的报话没点名规则 %s：%v", c.name, c.want, err)
			}
			if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
				t.Fatalf("%s 失败后留下了目录 %s", c.name, dst)
			}
			t.Logf("③c 拒（%s）：%v", c.name, err)
		})
	}
}

// TestExpandEgg_RefuseOtherGeneration —— 同一落点上换了另一枚蛋 ⇒ 拒（✗ 拿旧展开冒充新蛋）。
func TestExpandEgg_RefuseOtherGeneration(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "expanded")
	if _, err := ExpandEgg(eggTestWrite(t, eggTestFixture(t)), dst); err != nil {
		t.Fatalf("首趟展开：%v", err)
	}
	// 旧展开的「指纹」：`.complete` ＋ 一件落盘件的 sha —— 拒绝时**必须原样留着**。
	completeBefore, cerr := os.ReadFile(filepath.Join(dst, eggMarkerComplete))
	if cerr != nil {
		t.Fatalf("读首趟的 `.complete`：%v", cerr)
	}
	ggufBefore, gerr := eggSha256File(filepath.Join(dst, "model", "demo.gguf"))
	if gerr != nil {
		t.Fatalf("读首趟的权重段：%v", gerr)
	}
	other := eggTestFixture(t)
	other[0].Data = []byte(`{"arch":"另一个蛋"}`)
	_, err := ExpandEgg(eggTestWrite(t, other), dst)
	if err == nil || !strings.Contains(err.Error(), "另一代") {
		t.Fatalf("换蛋没拒（err=%v）", err)
	}
	completeAfter, _ := os.ReadFile(filepath.Join(dst, eggMarkerComplete))
	ggufAfter, aerr := eggSha256File(filepath.Join(dst, "model", "demo.gguf"))
	if aerr != nil {
		t.Fatalf("拒绝时把旧展开的权重段删了：%v", aerr)
	}
	if string(completeBefore) != string(completeAfter) || ggufBefore != ggufAfter {
		t.Fatalf("拒绝另一代时动了**旧展开**（.complete 或段内容变了）—— 旧展开必须原样留着")
	}
	if _, serr := os.Stat(filepath.Join(dst, eggMarkerFailed)); !os.IsNotExist(serr) {
		t.Fatalf("拒绝另一代时还往旧展开目录里写了 `.failed`")
	}
	t.Logf("换蛋拒：%v", err)
}

// ---- ④ 与 core 写端的跨实现对拍（缺制品 ⇒ 如实跳过）------------------------------------

func eggTestRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("算仓根：%v", err)
	}
	if _, serr := os.Stat(filepath.Join(root, "core", "cmd", "zerg")); serr != nil {
		t.Skipf("不在仓内跑（%s 下没有 core/cmd/zerg）⇒ 跳过真写端对拍", root)
	}
	return root
}

func eggTestZerg(t *testing.T, root string) string {
	t.Helper()
	p := filepath.Join(root, "bin", "zerg")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		t.Skipf("缺 %s（制品未重编）⇒ 跳过真写端对拍", p)
	}
	return p
}

// TestExpandEgg_RealPackerParity —— 真写端打蛋 → 本件展开 → 与 `cocoon info` 的读数逐格对拍。
func TestExpandEgg_RealPackerParity(t *testing.T) {
	root := eggTestRepoRoot(t)
	binZerg := eggTestZerg(t, root)
	work := t.TempDir()
	src := filepath.Join(work, "tree")
	for _, d := range []string{"model", "engine", "bin", "contract", "sysroot/lib"} {
		if err := os.MkdirAll(filepath.Join(src, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gguf := make([]byte, 1<<20)
	copy(gguf, []byte("GGUF"))
	for i := 4; i < len(gguf); i++ {
		gguf[i] = byte(i % 251)
	}
	must := func(p string, b []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(p, b, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(src, "model", "demo.gguf"), gguf, 0o644)
	must(filepath.Join(src, "config.json"), []byte(`{"arch":"v2-test"}`), 0o644)
	must(filepath.Join(src, "contract", "policy.json"), []byte(`{"policy":1}`), 0o644)
	must(filepath.Join(src, "bin", "run.sh"), []byte("#!/bin/sh\nexec ./engine/zerg-engine\n"), 0o755)
	must(filepath.Join(src, "engine", "zerg-engine"), []byte("#!/bin/sh\necho engine-v1\n"), 0o755)
	must(filepath.Join(src, "engine", "libfoo.so.1.12"), []byte("ELF-foo-1.12"), 0o644)
	must(filepath.Join(src, "sysroot", "lib", "libc.so.6"), []byte("stub-libc"), 0o644)
	if err := os.Symlink("libfoo.so.1.12", filepath.Join(src, "engine", "libfoo.so.1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libfoo.so.1", filepath.Join(src, "engine", "libfoo.so")); err != nil {
		t.Fatal(err)
	}
	egg := filepath.Join(work, "real.egg")
	cmd := exec.Command(binZerg, "cocoon", "pack", src, egg, "--align", "4096")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("真写端打蛋失败：%v\n%s", err, out)
	}
	info := exec.Command(binZerg, "cocoon", "info", egg)
	info.Env = os.Environ()
	infoOut, err := info.CombinedOutput()
	if err != nil {
		t.Fatalf("cocoon info 失败：%v\n%s", err, infoOut)
	}
	ix, err := ReadEggIndex(egg)
	if err != nil {
		t.Fatalf("本件读真写端的蛋失败：%v\n%s", err, infoOut)
	}
	// 跨实现对拍：段数 / 布局主号 / gen / uuid / 段表 sha 前 16 位 —— 逐格必须同时出现在 info 面里。
	want := []string{
		fmt.Sprintf("段数=%d", len(ix.Entries)),
		"version=2",
		fmt.Sprintf("gen=%d", ix.Gen),
		"uuid=" + ix.UUID,
		"table_sha256=" + ix.TableSHA[:16],
	}
	for _, w := range want {
		if !strings.Contains(string(infoOut), w) {
			t.Fatalf("`cocoon info` 里没有 %q（两实现读数不一致）：\n%s", w, infoOut)
		}
	}
	// `cocoon verify` 与本件展开必须同时给绿。
	ver := exec.Command(binZerg, "cocoon", "verify", egg)
	ver.Env = os.Environ()
	if out, verr := ver.CombinedOutput(); verr != nil {
		t.Fatalf("`cocoon verify` 失败：%v\n%s", verr, out)
	}
	dst := filepath.Join(work, "expanded")
	res, err := ExpandEgg(egg, dst)
	if err != nil {
		t.Fatalf("展开真写端的蛋失败：%v", err)
	}
	for _, e := range ix.Entries {
		full := filepath.Join(dst, filepath.FromSlash(e.Path))
		if e.IsSymlink() {
			if got, rerr := os.Readlink(full); rerr != nil || got != e.LinkTarget {
				t.Fatalf("链段 %s：%q / %v（要 %q）", e.Path, got, rerr, e.LinkTarget)
			}
			continue
		}
		sum, serr := eggSha256File(full)
		if serr != nil || sum != e.SHA256 {
			t.Fatalf("段 %s sha256 不符：%v / %s（要 %s）", e.Path, serr, sum, e.SHA256)
		}
	}
	t.Logf("④ 真写端对拍：段 %d · 字节 %d · gen=%d · 段表 %s · 与 `cocoon info`/`verify` 逐格一致",
		res.Segments, res.Bytes, ix.Gen, ix.TableSHA[:16])
}

// TestExpandEgg_SymlinkDepthLimit —— 链层数 > 8 ⇒ 拒（与写端同一张规则表）。
func TestExpandEgg_SymlinkDepthLimit(t *testing.T) {
	var segs []eggTestSeg
	segs = append(segs, eggTestSeg{Path: "chain/l00", Data: []byte("tail"), Mode: 0o644})
	// l01 → l00 · l02 → l01 … l09 → l08 ⇒ 从 l09 起链层数 10 > 8。
	for i := 1; i <= 9; i++ {
		segs = append(segs, eggTestSeg{
			Path: fmt.Sprintf("chain/l%02d", i), Mode: 0o755,
			Link: fmt.Sprintf("l%02d", i-1),
		})
	}
	egg := eggTestWrite(t, segs)
	_, err := ExpandEgg(egg, filepath.Join(t.TempDir(), "expanded"))
	if err == nil || !strings.Contains(err.Error(), "fs_symlink_depth") {
		t.Fatalf("超深链没拒（err=%v）", err)
	}
	t.Logf("超深链拒：%v", err)
}

// TestReadEggIndex_RejectsBadHeader —— 头/表的每一道闸各一枚负控（✗ 只测「能读」）。
func TestReadEggIndex_RejectsBadHeader(t *testing.T) {
	good, err := os.ReadFile(eggTestWrite(t, eggTestFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	write := func(b []byte) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "bad.egg")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// ★ 双槽布局：坏槽**不参与择优** ⇒ 只坏槽 0 时读端仍会取槽 1（那是设计，不是漏判）。
	// 因此每条「头坏了」的负控都要**两槽一起坏**，否则测的是「槽 1 还能用」。
	bothSlots := func(b []byte, f func(slot []byte)) {
		for i := 0; i < 2; i++ {
			f(b[i*eggSlotBytes : (i+1)*eggSlotBytes])
		}
	}
	recrc := func(slot []byte) {
		binary.LittleEndian.PutUint32(slot[eggOffCRC:], crc32.Checksum(slot[:eggCRCSpan], crc32.MakeTable(crc32.Castagnoli)))
	}
	cases := []struct {
		name string
		mut  func(b []byte)
	}{
		{"坏 magic（两槽）", func(b []byte) { bothSlots(b, func(s []byte) { copy(s[0:4], "XXXX") }) }},
		{"坏布局主号（两槽）", func(b []byte) {
			bothSlots(b, func(s []byte) { binary.LittleEndian.PutUint32(s[eggOffLayout:], 9) })
		}},
		{"坏 crc（两槽）", func(b []byte) { bothSlots(b, func(s []byte) { s[eggOffCRC] ^= 0xff }) }},
		{"坏 align（两槽 · crc 自洽）", func(b []byte) {
			bothSlots(b, func(s []byte) {
				binary.LittleEndian.PutUint64(s[eggOffAlign:], 4097)
				recrc(s)
			})
		}},
		{"段表落点未对齐（两槽 · crc 自洽）", func(b []byte) {
			bothSlots(b, func(s []byte) {
				binary.LittleEndian.PutUint64(s[eggOffTableOff:], 8192+1)
				recrc(s)
			})
		}},
	}
	for _, c := range cases {
		b := append([]byte{}, good...)
		c.mut(b)
		if _, err := ReadEggIndex(write(b)); err == nil {
			t.Fatalf("%s 竟读成功", c.name)
		} else {
			t.Logf("✓ 拒（%s）：%v", c.name, err)
		}
	}
	// 段表字节被改 ⇒ table_sha256 不符
	b := append([]byte{}, good...)
	ix, err := ReadEggIndex(eggTestWrite(t, eggTestFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	b[ix.TableOff] ^= 0xff
	if _, err := ReadEggIndex(write(b)); err == nil || !strings.Contains(err.Error(), "段表字节的 sha256") {
		t.Fatalf("改段表字节没拒（err=%v）", err)
	} else {
		t.Logf("✓ 拒（改段表字节）：%v", err)
	}
	// 截断（只留头）
	if _, err := ReadEggIndex(write(good[:eggHeaderBytes])); err == nil {
		t.Fatalf("截断的蛋竟读成功")
	}
}

// TestExpandEgg_FilePathAudit —— 展开后的路径集合与段表逐条相等（✗ 多出件 / 少件都不许）。
func TestExpandEgg_FilePathAudit(t *testing.T) {
	segs := eggTestFixture(t)
	egg := eggTestWrite(t, segs)
	dst := filepath.Join(t.TempDir(), "expanded")
	ix, err := ReadEggIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExpandEgg(egg, dst); err != nil {
		t.Fatal(err)
	}
	var got []string
	err = filepath.Walk(dst, func(p string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dst, p)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".") {
			return nil
		}
		got = append(got, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range ix.Entries {
		want = append(want, e.Path)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("落盘路径集合与段表不符：\n盘上=%v\n段表=%v", got, want)
	}
	// 逐字节复核：整件 sha 之外的又一道（防止「路径对、内容错」的静默档）。
	if n, err := countLines(dst); err != nil || n == 0 {
		t.Fatalf("计数失败：%d / %v", n, err)
	}
}

// countLines —— 包内小工具（数落盘件数；给上面的两态用例一个不依赖 Walk 的复核口）。
func countLines(root string) (int, error) {
	n := 0
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

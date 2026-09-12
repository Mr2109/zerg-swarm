package modelreg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ── 待修补 #3（高）+ #17（中）阶段 1 验收用例（反例优先）──────────────────────
//
// 覆盖：①正确记录+正确路径 → 过；②内容被改而路径未变 → error；③路径被改而内容未变 → error；
// ④写入权限模式（目录/文件）；⑤旧记录向后兼容；另加 #17 的「记录被放进别的 id 目录」反例
// 与「兄弟文件不被误判」反例。所有写盘一律落 t.TempDir()，不碰真实家目录。

// marshalRecord 把记录序列化成与 Store.Put 一致的字节（用于模拟“直接改盘”）。
func marshalRecord(t *testing.T, rec *Record) []byte {
	t.Helper()
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	return append(b, '\n')
}

// ① 正确记录 + 正确路径 → 身份自洽，零 findings。
func TestPlacementCorrectRecordPasses(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("good-model", "a")
	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("入目录失败：%v", err)
	}
	if f := VerifyPlacement(res.Path, rec); len(f) != 0 {
		t.Fatalf("正确记录+正确路径不该有任何 findings，实际：%+v", f)
	}
	got, err := CheckPlacement(res.Path, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("CheckPlacement(文件) 应返回 1 条：got=%+v err=%v", got, err)
	}
	if got[0].Failed(false) {
		t.Fatalf("正确记录不该判不通过：%+v", got[0])
	}
}

// ② 内容被改而路径未变（digest 改了，文件名还是旧 digest）→ error。
//
//	模拟攻击者直接改盘上的正文（绕过 Store.Put）。
func TestPlacementContentEditedPathUnchanged(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("tamper-model", "a")
	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("入目录失败：%v", err)
	}
	// 直接改盘：换成另一条 digest（路径名不变，仍是旧 digest 的名字）
	tampered := goodRecord("tamper-model", "b")
	tampered.Notes = "内容被改，路径未变"
	if err := os.WriteFile(res.Path, marshalRecord(t, tampered), 0o600); err != nil {
		t.Fatalf("写篡改内容失败：%v", err)
	}
	got, err := CheckPlacement(res.Path, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("应返回 1 条：got=%+v err=%v", got, err)
	}
	if got[0].PlacementErrors() == 0 {
		t.Fatalf("内容改而路径未变必须报 error，实际 findings=%+v", got[0].Findings)
	}
	if !got[0].Failed(false) {
		t.Fatal("该记录应判不通过")
	}
}

// ③ 路径被改而内容未变（把文件改名成不匹配 digest 的名字）→ error。
func TestPlacementPathChangedContentUnchanged(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("moved-model", "c")
	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("入目录失败：%v", err)
	}
	b, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	// 改名成另一个（也与 digest 不符的）名字，内容一字不动
	wrong := filepath.Join(filepath.Dir(res.Path), "sha256-deadbeef0000.json")
	if err := os.Remove(res.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrong, b, 0o600); err != nil {
		t.Fatal(err)
	}
	// 用内存记录（与磁盘内容相同）直接校验
	f := VerifyPlacement(wrong, rec)
	if CountErrors(f) == 0 {
		t.Fatalf("路径被改而内容未变必须报 error，实际 findings=%+v", f)
	}
	got, _ := CheckPlacement(wrong, false)
	if len(got) != 1 || !got[0].Failed(false) {
		t.Fatalf("该记录应判不通过：%+v", got)
	}
}

// ③补：记录被放进别的 model_id 目录（内容与文件名都对，只有目录名错）→ error。
//
//	这是 #17「同名 model_id 可被替换」的一条真实入口：store.List 以目录名当 id。
func TestPlacementRecordInWrongIDDir(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("real-id", "d")
	// 故意放到 wrong-id 目录下（文件名与 digest 是自洽的）
	wrongDir := filepath.Join(st.ManifestsDir(), "wrong-id")
	if err := os.MkdirAll(wrongDir, ModeDir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(wrongDir, VersionOf(rec)+".json")
	if err := os.WriteFile(p, marshalRecord(t, rec), 0o600); err != nil {
		t.Fatal(err)
	}
	f := VerifyPlacement(p, rec)
	// 文件名自洽 → 只应有 1 条（目录名/id 不符）
	if CountErrors(f) != 1 || f[0].Field != "id" {
		t.Fatalf("应恰好报 1 条 id 不自洽，实际：%+v", f)
	}
}

// ④ 写入权限：新建目录 0700、新建文件 0600（断言 os.Stat 的模式位）。
//
//	断言钉**字面量**（0o700/0o600），不钉 ModeDir/ModeFile 常量——否则改常量两边一起变，
//	测试恒绿（变异验证抓到过一次：把常量改回 0o755 时本用例之前的写法没变红）。
func TestWritePermissionsTightened(t *testing.T) {
	const wantDir, wantFile = os.FileMode(0o700), os.FileMode(0o600)
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("perm-model", "e")
	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("入目录失败：%v", err)
	}
	for _, c := range []struct {
		p    string
		want os.FileMode
		dir  bool
	}{
		{st.ManifestsDir(), wantDir, true},
		{filepath.Join(st.ManifestsDir(), "perm-model"), wantDir, true},
		{res.Path, wantFile, false},
	} {
		fi, err := os.Stat(c.p)
		if err != nil {
			t.Fatalf("stat %s: %v", c.p, err)
		}
		if fi.IsDir() != c.dir {
			t.Fatalf("%s 的目录/文件类型不符：%v", c.p, fi.IsDir())
		}
		if got := fi.Mode().Perm(); got != c.want {
			t.Fatalf("%s 权限应为 %#o，实际 %#o", c.p, uint32(c.want), uint32(got))
		}
	}
	// 另十字校验：代码里落实权限用的常量必须正是这条政策（防“测试对了但接线用了别的常量”）。
	if ModeDir != wantDir || ModeFile != wantFile {
		t.Fatalf("ModeDir/ModeFile 必须等于政策 0700/0600，实际 ModeDir=%#o ModeFile=%#o", uint32(ModeDir), uint32(ModeFile))
	}
}

// ④补：WriteFileAtomic 直接建目录/文件时也应 0700/0600（同样钉字面量）。
func TestWriteFileAtomicPermissions(t *testing.T) {
	const wantDir, wantFile = os.FileMode(0o700), os.FileMode(0o600)
	p := filepath.Join(t.TempDir(), "sub", "rec.json") // 目标目录还不存在
	if _, err := WriteFileAtomic(p, []byte("A\n")); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	di, err := os.Stat(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != wantDir {
		t.Fatalf("新建目录应为 %#o，实际 %#o", uint32(wantDir), uint32(di.Mode().Perm()))
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != wantFile {
		t.Fatalf("新建文件应为 %#o，实际 %#o", uint32(wantFile), uint32(fi.Mode().Perm()))
	}
}

// ⑤ 旧记录向后兼容：凡经 Store.Put 落盘的（含多个 id/版本）走目录校验一律通过，
//
//	不因新校验误报；且兄弟文件（trace/capabilities）不被当成记录。
func TestPlacementBackwardCompatibleOldRecords(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	recs := []*Record{
		goodRecord("old-a", "1"),
		goodRecord("old-a", "2"), // 同 id 两个版本
		goodRecord("old-b", "3"),
	}
	for _, r := range recs {
		res, err := st.Put(r, false)
		if err != nil {
			t.Fatalf("入目录失败：%v", err)
		}
		// 落盘路径必须正好是规范路径（这是向后兼容的根据：VersionOf 一直只由 digest 推出）
		if want := st.RecordPath(r.ID, VersionOf(r)); res.Path != want {
			t.Fatalf("落盘路径非规范：want=%s got=%s", want, res.Path)
		}
	}
	// 造兄弟文件：它们不是记录，目录校验必须跳过
	p := st.PathFor(recs[0])
	if err := os.WriteFile(TraceSiblingPath(p), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CapabilitySnapshotPath(p), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := CheckPlacement(st.ManifestsDir(), false)
	if err != nil {
		t.Fatalf("目录校验不该报错：%v", err)
	}
	if len(got) != 3 {
		t.Fatalf("应只剩 3 条记录（跳过 2 个兄弟文件），实际 %d 条：%+v", len(got), got)
	}
	for _, r := range got {
		if r.Failed(false) {
			t.Fatalf("经 Store.Put 落盘的旧记录不该判不通过（向后兼容）：%+v", r)
		}
	}
}

// ⑥ --strict 口径一致：格式 warn 在非严格下不算不通过，严格下算（与 `verify --strict` 同义）。
func TestPlacementStrictTreatsWarnsAsFailure(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("warn-model", "a")
	rec.SourceURL = ""
	rec.License.SourceURL = "" // → 两条 warn，零 error
	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("非严格下零 error 的记录应能入目录：%v", err)
	}
	got, _ := CheckPlacement(res.Path, false)
	if len(got) != 1 || got[0].Failed(false) {
		t.Fatalf("非严格：位置自洽 + 零 error 不该判不通过：%+v", got)
	}
	gotS, _ := CheckPlacement(res.Path, true)
	if len(gotS) != 1 || !gotS[0].Failed(true) {
		t.Fatalf("严格：warn 也应算不通过（与 verify --strict 同口径）：%+v", gotS)
	}
	if gotS[0].PlacementErrors() != 0 {
		t.Fatalf("本用例前提是位置自洽（零 placement error），实际：%+v", gotS[0].Findings)
	}
}

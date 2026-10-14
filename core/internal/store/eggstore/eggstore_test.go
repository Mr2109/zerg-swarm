package eggstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ───────────────────────── 夹具 ─────────────────────────

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("打开卵库失败：%v", err)
	}
	return s
}

// landAndRegister 走真落地（LandFile）再登记 ⇒ 名册里的行与文件面一致。
func landAndRegister(t *testing.T, s *Store, name string, payload []byte) (Egg, LandResult) {
	t.Helper()
	dst := filepath.Join(s.Root, name+".egg")
	res, err := LandFile(dst, strings.NewReader(string(payload)), LandOptions{ChunkSize: 1 << 16})
	if err != nil {
		t.Fatalf("落地 %s 失败：%v", name, err)
	}
	e := Egg{Name: name, SHA256: res.SHA256, Path: dst, Bytes: res.Bytes,
		State: StateVerified, Complete: true, Blocks: res.Blocks}
	if err := s.Register(e); err != nil {
		t.Fatalf("登记 %s 失败：%v", name, err)
	}
	got, _ := s.Roster.Get(name)
	return got, res
}

// ───────────────────────── 四态亲测（真跑 · 贴读数）─────────────────────────

// 态① 落地中途失败 ⇒ 半成品不入名册 ＋ 可重试。
func TestEggstoreState01LandingMidwayFailure(t *testing.T) {
	s := newTestStore(t)
	dst := filepath.Join(s.Root, "half.egg")
	tmp := TmpNameFor(dst)

	src := &failingReader{total: 300000, failAt: 131072}
	_, err := LandFile(dst, src, LandOptions{ChunkSize: 1 << 16})
	if err == nil {
		t.Fatalf("态①：源中途失败 ⇒ 落地必须出错")
	}
	t.Logf("态① 落地中途失败：err=%v", err)

	if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
		t.Fatalf("态①：终名 %s 不该产生（err=%v）", dst, serr)
	}
	if _, serr := os.Stat(tmp); !os.IsNotExist(serr) {
		t.Fatalf("态①：半成品临时名 %s 应被清掉（err=%v）", tmp, serr)
	}
	if n := s.Roster.Count(); n != 0 {
		t.Fatalf("态①：名册必须零新增，实 %d 条", n)
	}
	// 负控：就算有人硬把半成品塞进名册，也必须被拒。
	if rerr := s.Register(Egg{Name: "half", SHA256: "aa", State: StateLanded, Complete: false}); rerr == nil {
		t.Fatalf("态①负控：半成品本该被名册拒")
	} else {
		t.Logf("态① 负控（硬塞半成品入册）⇒ 被拒：%v", rerr)
	}
	if n := s.Roster.Count(); n != 0 {
		t.Fatalf("态①：被拒之后名册仍须为空，实 %d 条", n)
	}

	// 可重试：同一终名再来一次应成功，并如实入册。
	res, err := LandFile(dst, strings.NewReader("retry-ok-payload"), LandOptions{})
	if err != nil {
		t.Fatalf("态①：重试应成功，实 %v", err)
	}
	if err := s.Register(Egg{Name: "half", SHA256: res.SHA256, Path: dst, Bytes: res.Bytes,
		State: StateVerified, Complete: true, Blocks: res.Blocks}); err != nil {
		t.Fatalf("态①：重试后登记应成功，实 %v", err)
	}
	t.Logf("态① 读数：名册 0 → %d 条 · 终名 %s · 块 %d · sha16=%s · 父目录 fsync=%v",
		s.Roster.Count(), filepath.Base(dst), res.Blocks, res.SHA256[:16], res.DirSynced)
}

// 态② 引用计数未归零 ⇒ 不删；归零 ⇒ 删且报数用 df 核。
func TestEggstoreState02RefcountGatesDeletion(t *testing.T) {
	s := newTestStore(t)
	// 32 MiB 卵：够大，df 差值可读（✗ 不靠 du）。
	payload := make([]byte, 32<<20)
	for i := range payload {
		payload[i] = byte(i * 31)
	}
	egg, _ := landAndRegister(t, s, "big", payload)
	if _, _, err := s.SetState("big", StateReclaimable, "test", "空窗到期，进入可回收"); err != nil {
		t.Fatalf("态②：置 reclaimable 失败：%v", err)
	}
	// 展开物 / 克隆副本：**同一份引用计数**（这里用 RefClone 代表克隆副本）。
	if err := s.Refs.Acquire("big", "clone-1", RefClone); err != nil {
		t.Fatalf("态②：加引用失败：%v", err)
	}
	if err := s.Refs.Acquire("big", "exp-1", RefExpansion); err != nil {
		t.Fatalf("态②：加引用失败：%v", err)
	}

	rep, err := s.GC(false)
	if err != nil {
		t.Fatalf("态②：GC 失败：%v", err)
	}
	if len(rep.Deleted) != 0 {
		t.Fatalf("态②：引用计数=2 时**不许删**，却删了 %v", rep.Deleted)
	}
	if _, serr := os.Stat(egg.Path); serr != nil {
		t.Fatalf("态②：卵件不该被动，实 err=%v", serr)
	}
	t.Logf("态② 未归零：count=%d · 钉住理由=%q · 已删=0 · 卵件仍在盘",
		s.Refs.Count("big"), rep.Pinned[0].Why)

	// 解引用（逐个解；展开物与克隆副本都算）。
	for _, ref := range []string{"clone-1", "exp-1"} {
		if ok, rerr := s.Refs.Release("big", ref); rerr != nil || !ok {
			t.Fatalf("态②：解引用 %s 失败（ok=%v err=%v）", ref, ok, rerr)
		}
	}
	if !s.Refs.Zero("big") {
		t.Fatalf("态②：解引用后引用计数应归零")
	}
	before, err := StatfsAvailable(s.Root)
	if err != nil {
		t.Fatalf("态②：取 df 失败：%v", err)
	}
	rep2, err := s.GC(false)
	if err != nil {
		t.Fatalf("态②：GC 失败：%v", err)
	}
	if len(rep2.Deleted) != 1 {
		t.Fatalf("态②：归零后应真删 1 枚，实 %v（钉住 %+v）", rep2.Deleted, rep2.Pinned)
	}
	if _, serr := os.Stat(egg.Path); !os.IsNotExist(serr) {
		t.Fatalf("态②：卵件应已真删（err=%v）", serr)
	}
	tomb, ok := s.Roster.Get("big")
	if !ok || tomb.State != StateDeleted || tomb.Refs != 0 {
		t.Fatalf("态②：名册应留墓碑（deleted/refs=0），实 %+v ok=%v", tomb, ok)
	}
	t.Logf("态② 归零后：已删=%v · df←%d · df→%d · 已释放(df 地面真值)=%d B · 卵名义字节(✗ 不作已释放)=%d B · 采样器自证 before=%d",
		rep2.Deleted, rep2.DFBefore, rep2.DFAfter, rep2.FreedBytesDF, rep2.NominalBytesNotFreed, before)
	if rep2.FreedSource != FreedSourceDF {
		t.Fatalf("态②：报数来源必须是 df，实 %q", rep2.FreedSource)
	}
	if rep2.NominalNote == "" || !strings.Contains(rep2.NominalNote, "严禁") {
		t.Fatalf("态②：名义字节那格必须带「✗ 严禁当已释放上报」的注")
	}
	t.Logf("态② 报数口径：source=%q · 名义字节注=%q", rep2.FreedSource, rep2.NominalNote)
}

// 态③ 盘满触发水线 ⇒ 拒新孵化（✗ 不是静默继续）＋ 给可回收卵清单。
func TestEggstoreState03WaterlineRefusesHatch(t *testing.T) {
	s := newTestStore(t)
	payload := make([]byte, 1<<20)
	_, _ = landAndRegister(t, s, "ready", payload)
	// 另造一枚可回收卵，用来验证「拒孵时给可回收清单」。
	_, _ = landAndRegister(t, s, "old", payload)
	if _, _, err := s.SetState("old", StateReclaimable, "test", "空窗到期"); err != nil {
		t.Fatalf("态③：置 reclaimable 失败：%v", err)
	}
	img, err := ReadVolume(s.Root)
	if err != nil {
		t.Fatalf("态③：取卷事实失败：%v", err)
	}
	node, err := ReadVolume("/")
	if err != nil {
		t.Fatalf("态③：取根卷事实失败：%v", err)
	}
	t.Logf("态③ 真盘读数 imagefs：%s", img.Line())
	t.Logf("态③ 真盘读数 nodefs ：%s", node.Line())

	// (a) 默认水线下的真实裁决。
	if _, err := s.HatchGate("ready", s.Root, "/"); err != nil {
		t.Logf("态③ 默认水线下 hatch 判定（本机现况）：err=%v", err)
	} else {
		t.Logf("态③ 默认水线下 hatch 判定（本机现况）：放行 rc=0")
	}

	// (b) 把水线的「高水线」压到 1% ⇒ 本机真实盘面必然过线 ⇒ 必须拒孵 + 给清单。
	w := s.WaterlineNow()
	w.HighUsedPercent = 1
	w.LowUsedPercent = 0
	s.Waterline = &w
	p, err := s.HatchGate("ready", s.Root, "/")
	if err == nil {
		t.Fatalf("态③：过水线时必须拒孵（✗ 不许静默继续），实放行")
	}
	if p.Verdict != VerdictRefuse {
		t.Fatalf("态③：判决应为 %s，实 %s", VerdictRefuse, p.Verdict)
	}
	var re *RefuseError
	if !errors.As(err, &re) {
		t.Fatalf("态③：应是 RefuseError（出声），实 %T", err)
	}
	if !strings.Contains(re.Reason, "可回收卵清单") || !strings.Contains(re.Reason, "old") {
		t.Fatalf("态③：拒孵理由必须带可回收卵清单并点名 old，实 %q", re.Reason)
	}
	t.Logf("态③ 判决=%s · 理由=%v", p.Verdict, p.Reasons)
	t.Logf("态③ 拒孵出声：%s", re.Error())
	t.Logf("态③ 可回收清单=%v", p.Reclaimable)
}

// 态④ 完工标记缺失 ⇒ 视为不可用（✗ 不当成可用）。
func TestEggstoreState04DoneMarkerIsState(t *testing.T) {
	s := newTestStore(t)
	exp := filepath.Join(s.Root, "expanded", "big")
	if err := os.MkdirAll(exp, 0o755); err != nil {
		t.Fatal(err)
	}
	// 目录明明在盘上、里面甚至还有内容。
	if err := os.WriteFile(filepath.Join(exp, "weights.bin"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, why := DirUsable(exp)
	if ok {
		t.Fatalf("态④：缺完工标记必须判不可用（✗ 不许拿「目录存在」当可用）")
	}
	t.Logf("态④ 缺标记：DirUsable=false · why=%s · %s", why, DirNote(exp))

	// 真落地一整套：写文件 + 完工标记 ⇒ 才可用。
	dst := filepath.Join(exp, "part0")
	if _, err := LandFile(dst, strings.NewReader("0123456789"), LandOptions{ChunkSize: 4}); err != nil {
		t.Fatal(err)
	}
	if ok, why := DirUsable(exp); ok {
		t.Fatalf("态④：只落了数据、还没写完工标记 ⇒ 仍须判不可用（why=%s）", why)
	}
	if err := WriteDoneMarker(exp, DoneMarker{SHA256: "aa", Blocks: 3, Bytes: 10}); err != nil {
		t.Fatal(err)
	}
	ok, why = DirUsable(exp)
	if !ok {
		t.Fatalf("态④：完工标记齐备后应判可用，实不可用 why=%s", why)
	}
	m, found, err := ReadDoneMarker(exp)
	if err != nil || !found || m.Blocks != 3 {
		t.Fatalf("态④：完工标记读回不对：%+v found=%v err=%v", m, found, err)
	}
	t.Logf("态④ 有标记：DirUsable=true · 标记=%+v · %s", m, DirNote(exp))

	// 负控：空标记必须被拒（✗ 不许写空标记充完工）。
	if werr := WriteDoneMarker(filepath.Join(s.Root, "emptymark"), DoneMarker{}); werr == nil {
		t.Fatalf("态④负控：空完工标记本该被拒")
	} else {
		t.Logf("态④ 负控（空标记）⇒ 被拒：%v", werr)
	}
}

// 负控：跨卷临时名 ⇒ 拒（rename 会静默退化成 copy）。用**真第二卷**，没有就跳过。
func TestEggstoreNegativeControlCrossVolume(t *testing.T) {
	other, ok := realSecondVolume()
	if !ok {
		t.Skip("本机只有一个卷 ⇒ 这一格不给结论（✗ 不用假卷凑）")
	}
	s := newTestStore(t)
	dstDir := filepath.Join(other, "eggstore-xvol-test")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Skipf("第二卷不可写（%v）⇒ 这一格不给结论", err)
	}
	defer func() { _ = os.RemoveAll(dstDir) }()
	dst := filepath.Join(dstDir, "x.egg")
	tmp := filepath.Join(os.TempDir(), "eggstore-xvol.part")
	same, err := SameVolume(tmp, dst)
	if err != nil {
		t.Fatalf("同卷判定失败：%v", err)
	}
	if same {
		t.Skip("本机两卷被判定成同卷 ⇒ 这一格不给结论")
	}
	_, lerr := LandFile(dst, strings.NewReader("hello"), LandOptions{TmpPath: tmp})
	if !errors.Is(lerr, ErrCrossVolume) {
		t.Fatalf("跨卷临时名必须被 ErrCrossVolume 拒，实 err=%v", lerr)
	}
	if _, serr := os.Stat(dst); !os.IsNotExist(serr) {
		t.Fatalf("被拒后终名不该存在（err=%v）", serr)
	}
	if _, serr := os.Stat(tmp); !os.IsNotExist(serr) {
		t.Fatalf("被拒后不该留临时件（err=%v）", serr)
	}
	t.Logf("负控 跨卷：tmp=%s（卷 A）⟷ dst=%s（卷 B）⇒ err=%v", tmp, dst, lerr)
	_ = s
}

// 报数口径的**真盘**复核：df 与「名义字节」在同一时刻各是多少（✗ 不用 du 当判据）。
func TestEggstoreReportUsesDFNotNominal(t *testing.T) {
	s := newTestStore(t)
	payload := make([]byte, 24<<20)
	egg, _ := landAndRegister(t, s, "rpt", payload)
	if _, _, err := s.SetState("rpt", StateReclaimable, "test", "空窗"); err != nil {
		t.Fatal(err)
	}
	// 造一个「共享物理块」的副本：硬链（APFS/reflink 的极端形态 —— 完全同一批块）。
	link := filepath.Join(s.Root, "rpt.clone")
	if err := os.Link(egg.Path, link); err != nil {
		t.Skipf("本卷不支持硬链（%v）⇒ 这一格不给结论", err)
	}
	if err := s.Refs.Acquire("rpt", "clone-1", RefClone); err != nil {
		t.Fatal(err)
	}
	repPinned, err := s.GC(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repPinned.Deleted) != 0 {
		t.Fatalf("有克隆副本时不许删，实 %v", repPinned.Deleted)
	}
	// 解引用 + 撤掉那枚共享块副本 ⇒ 归零。
	if _, err := s.Refs.Release("rpt", "clone-1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	rep, err := s.GC(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("报数口径：nominal=%d B（=%.1f MiB）· df before=%d · df after=%d · FreedBytesDF=%d · source=%q",
		rep.NominalBytesNotFreed, float64(rep.NominalBytesNotFreed)/(1<<20),
		rep.DFBefore, rep.DFAfter, rep.FreedBytesDF, rep.FreedSource)
	if rep.FreedSource != FreedSourceDF {
		t.Fatalf("来源必须是 df，实 %q", rep.FreedSource)
	}
	if rep.DFAfter-rep.DFBefore != uint64(rep.FreedBytesDF) && rep.DFAfter >= rep.DFBefore {
		t.Fatalf("df 差值必须与报出的已释放一致（after=%d before=%d freed=%d）",
			rep.DFAfter, rep.DFBefore, rep.FreedBytesDF)
	}
	if len(rep.Deleted) != 1 {
		t.Fatalf("归零后应删 1 枚，实 %v", rep.Deleted)
	}
}

// ───────────────────────── 自检口 ─────────────────────────

func TestEggstoreSelfTest(t *testing.T) {
	passed, failed, skipped, lines := SelfTest()
	for _, l := range lines {
		t.Log(l)
	}
	verdict := "全过"
	if failed > 0 {
		verdict = "有失败"
	}
	t.Logf("自检结论: %s（断言 %d 格 · 失败 %d 格 · 不适用 %d 格）", verdict, passed+failed+skipped, failed, skipped)
	if failed != 0 {
		t.Fatalf("自检有 %d 格失败（详见上面 FAIL 行）", failed)
	}
	if passed == 0 {
		t.Fatalf("自检一格都没跑 ⇒ 恒绿（✗ 不给结论）")
	}
}

// ───────────────────────── 夹具 ─────────────────────────

// failingReader 读满 failAt 字节之后报错 —— 造「落地中途失败」。
type failingReader struct {
	total, failAt, pos int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.pos >= r.failAt {
		return 0, fmt.Errorf("注入的读失败（第 %d 字节）", r.failAt)
	}
	n := len(p)
	if r.pos+n > r.failAt {
		n = r.failAt - r.pos
	}
	if r.pos+n > r.total {
		n = r.total - r.pos
	}
	for i := 0; i < n; i++ {
		p[i] = byte(r.pos + i)
	}
	r.pos += n
	return n, nil
}

var _ io.Reader = (*failingReader)(nil)

// realSecondVolume 真第二卷（与根卷设备号不同）。
func realSecondVolume() (string, bool) {
	rootDev, err := VolumeKeyOf("/")
	if err != nil {
		return "", false
	}
	if _, err := os.Stat("/Volumes"); err != nil {
		return "", false
	}
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return "", false
	}
	for _, e := range ents {
		p := filepath.Join("/Volumes", e.Name())
		dev, derr := VolumeKeyOf(p)
		if derr != nil || dev == rootDev {
			continue
		}
		if st, serr := os.Stat(p); serr == nil && st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// TestEggstoreSHAStable 抖一下 LandFile 的 sha 口径（内容相同 ⇒ sha 相同）。
func TestEggstoreSHAStable(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "s.egg")
	r1, err := LandFile(dst, strings.NewReader("stable"), LandOptions{ChunkSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	dst2 := filepath.Join(filepath.Dir(dst), "s2.egg")
	r2, err := LandFile(dst2, strings.NewReader("stable"), LandOptions{ChunkSize: 8})
	if err != nil {
		t.Fatal(err)
	}
	if r1.SHA256 != r2.SHA256 {
		t.Fatalf("同内容不同块大小 ⇒ sha 必须相同：%s ⟷ %s", r1.SHA256, r2.SHA256)
	}
	want := sha256.Sum256([]byte("stable"))
	if r1.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("sha 与标准库不符：%s", r1.SHA256)
	}
	t.Logf("sha 稳定口径：%s（块 3 与 1 两态同值）", r1.SHA256[:16])
}
